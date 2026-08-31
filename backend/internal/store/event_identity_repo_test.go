package store

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/database"
)

// **只跑 sqlite**，與其他 repo 測試相同的既有限制（issue.md I-054 第 1 項）。
func newEventIdentityRepoForTest(t *testing.T) (EventIdentityRepo, context.Context) {
	t.Helper()
	_, repo, ctx := newEventIdentityStackForTest(t)
	return repo, ctx
}

// newEventIdentityStackForTest 多回一個 *sqlx.DB，給需要自己種
// stock_sr_zone_analyses（identity_since 的 K 棒軸來源）的測試用。
func newEventIdentityStackForTest(t *testing.T) (*sqlx.DB, EventIdentityRepo, context.Context) {
	t.Helper()
	tmp, err := os.CreateTemp("", "event-identity-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	t.Cleanup(func() { os.Remove(tmp.Name()) })

	db, err := NewDB(config.DatabaseConfig{Driver: "sqlite", DSN: tmp.Name()})
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.RunMigrations(context.Background(), db, "sqlite", zap.NewNop()); err != nil {
		t.Fatalf("migrations failed: %v", err)
	}
	// event_instances.zone_uid 有外鍵指向 zone_instances，先種一個真實的身分進去。
	zoneRepo := NewZoneIdentityRepo(db)
	if err := zoneRepo.Apply(context.Background(), ZoneIdentityWrite{
		Instances: []ZoneInstance{
			zoneInstance("Z-1", 104.73, 105.37, eventSeenAt),
			zoneInstance("Z-2", 99.10, 100.20, eventSeenAt),
		},
	}); err != nil {
		t.Fatalf("seed zone identities failed: %v", err)
	}
	return db, NewEventIdentityRepo(db), context.Background()
}

var eventSeenAt = time.Date(2026, 8, 18, 7, 0, 0, 0, time.UTC)

func eventInstance(uid, zoneUID, family, state string, seq int, seen time.Time) EventInstance {
	inst := EventInstance{
		EventUID:        uid,
		Symbol:          "0050",
		Timeframe:       "1d",
		ZoneScopeKey:    zoneUID,
		EventScope:      "ZONE",
		EventFamily:     family,
		Seq:             seq,
		RootEventType:   "SUPPORT_BREAKDOWN",
		LatestEventType: "SUPPORT_BREAKDOWN",
		State:           state,
		Active:          true,
		Direction:       "BEARISH",
		FirstSeenAt:     seen,
		LastSeenAt:      seen,
		// 預設決策可見：既有四個事件型別都是，只有階段 D 那兩個 family 不是。
		DecisionVisible: true,
	}
	if zoneUID == SymbolScopeKey {
		inst.EventScope = "SYMBOL"
	} else {
		inst.ZoneUID = sql.NullString{String: zoneUID, Valid: true}
	}
	return inst
}

func TestEventIdentityApplyWritesBothTables(t *testing.T) {
	repo, ctx := newEventIdentityRepoForTest(t)

	err := repo.Apply(ctx, EventIdentityWrite{
		Instances: []EventInstance{eventInstance("E-1", "Z-1", "SUPPORT_BREAKDOWN", "CONFIRMED", 1, eventSeenAt)},
		Transitions: []EventTransition{{
			EventUID:   "E-1",
			ToState:    "CONFIRMED",
			OccurredAt: eventSeenAt,
		}},
	})
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	got, err := repo.ListLatestChains(ctx, "0050", "1d")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(got) != 1 || got[0].EventUID != "E-1" || got[0].State != "CONFIRMED" {
		t.Fatalf("unexpected chains: %+v", got)
	}
	if !got[0].ZoneUID.Valid || got[0].ZoneUID.String != "Z-1" {
		t.Errorf("ZONE scope 的鏈要指得到身分，got %+v", got[0].ZoneUID)
	}
}

func TestEventIdentityApplyIsAtomic(t *testing.T) {
	// 只寫了 instances 卻沒寫 transitions 的話，鏈的「現在是什麼」有紀錄、
	// 「怎麼走到這裡」沒有——而那正是本階段要消滅的情況。
	repo, ctx := newEventIdentityRepoForTest(t)

	err := repo.Apply(ctx, EventIdentityWrite{
		Instances: []EventInstance{eventInstance("E-1", "Z-1", "SUPPORT_BREAKDOWN", "CONFIRMED", 1, eventSeenAt)},
		Transitions: []EventTransition{{
			EventUID:   "E-NOPE", // 外鍵指向不存在的鏈 → 整筆必須 rollback
			ToState:    "CONFIRMED",
			OccurredAt: eventSeenAt,
		}},
	})
	if err == nil {
		t.Fatal("外鍵違反必須讓整筆失敗")
	}
	got, _ := repo.ListLatestChains(ctx, "0050", "1d")
	if len(got) != 0 {
		t.Errorf("transitions 失敗時 instances 不能留下，got %+v", got)
	}
}

func TestEventIdentityRejectsDuplicateUIDInOneBatch(t *testing.T) {
	// 同一批重複 event_uid **只有 postgres 會炸**，sqlite/mysql 吞得下去。
	// 測試只跑 sqlite，所以在 Go 這層擋掉，三個 engine 行為才一致。
	repo, ctx := newEventIdentityRepoForTest(t)

	err := repo.Apply(ctx, EventIdentityWrite{Instances: []EventInstance{
		eventInstance("E-1", "Z-1", "SUPPORT_BREAKDOWN", "CONFIRMED", 1, eventSeenAt),
		eventInstance("E-1", "Z-2", "SUPPORT_RECLAIM", "CONFIRMED", 1, eventSeenAt),
	}})
	if err == nil {
		t.Fatal("同一批重複 event_uid 必須被擋下")
	}
}

func TestEventIdentityListLatestChainsIncludesEndedChains(t *testing.T) {
	// **這是 seq 能正確遞增的前提。** 只撈未終結的鏈，新鏈會一直算出 seq=1
	// 而撞 uq_event_instance_seq，且失敗是靜默的（寫入失敗只記 log）。
	repo, ctx := newEventIdentityRepoForTest(t)

	ended := eventInstance("E-1", "Z-1", "SUPPORT_BREAKDOWN", "RESOLVED", 1, eventSeenAt)
	ended.EndedAt = sql.NullTime{Time: eventSeenAt, Valid: true}
	ended.EndReason = sql.NullString{String: "RESOLVED", Valid: true}
	if err := repo.Apply(ctx, EventIdentityWrite{Instances: []EventInstance{ended}}); err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	got, err := repo.ListLatestChains(ctx, "0050", "1d")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(got) != 1 || !got[0].EndedAt.Valid || got[0].Seq != 1 {
		t.Fatalf("已終結的鏈必須撈得到（供 seq 遞增用），got %+v", got)
	}
}

func TestEventIdentityListLatestChainsReturnsHighestSeqPerKey(t *testing.T) {
	repo, ctx := newEventIdentityRepoForTest(t)

	first := eventInstance("E-1", "Z-1", "SUPPORT_BREAKDOWN", "RESOLVED", 1, eventSeenAt)
	first.EndedAt = sql.NullTime{Time: eventSeenAt, Valid: true}
	second := eventInstance("E-2", "Z-1", "SUPPORT_BREAKDOWN", "ACTIVE", 2, eventSeenAt)
	if err := repo.Apply(ctx, EventIdentityWrite{
		Instances: []EventInstance{first, second},
	}); err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	got, err := repo.ListLatestChains(ctx, "0050", "1d")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(got) != 1 || got[0].EventUID != "E-2" || got[0].Seq != 2 {
		t.Fatalf("每個 key 只回最新一條鏈，got %+v", got)
	}
}

func TestEventIdentityUpsertKeepsRootAndFirstSeen(t *testing.T) {
	// root 被 latest 蓋掉的話，欄位名叫 root 卻永遠等於 latest，鏈的起點無法還原
	// （T-045 在 market_event_states 上踩過一次）。first_seen 同理。
	repo, ctx := newEventIdentityRepoForTest(t)

	if err := repo.Apply(ctx, EventIdentityWrite{
		Instances: []EventInstance{eventInstance("E-1", "Z-1", "SUPPORT_BREAKDOWN", "CONFIRMED", 1, eventSeenAt)},
	}); err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	later := eventSeenAt.Add(48 * time.Hour)
	update := eventInstance("E-1", "Z-1", "SUPPORT_BREAKDOWN", "ACTIVE", 1, later)
	update.RootEventType = "INTRADAY_RECLAIM" // 想蓋掉 root
	update.FirstSeenAt = later
	update.LatestEventType = "INTRADAY_RECLAIM"
	if err := repo.Apply(ctx, EventIdentityWrite{Instances: []EventInstance{update}}); err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	got, _ := repo.ListLatestChains(ctx, "0050", "1d")
	if len(got) != 1 {
		t.Fatalf("unexpected chains: %+v", got)
	}
	if got[0].RootEventType != "SUPPORT_BREAKDOWN" {
		t.Errorf("root 不可被覆寫，got %q", got[0].RootEventType)
	}
	if !got[0].FirstSeenAt.Equal(eventSeenAt) {
		t.Errorf("first_seen_at 不可被覆寫，got %v", got[0].FirstSeenAt)
	}
	if got[0].LatestEventType != "INTRADAY_RECLAIM" || got[0].State != "ACTIVE" {
		t.Errorf("latest/state 應該要更新，got %+v", got[0])
	}
	if !got[0].LastSeenAt.Equal(later) {
		t.Errorf("last_seen_at 應該前進，got %v", got[0].LastSeenAt)
	}
}

func TestEventIdentityUpsertDoesNotResurrectEndedChain(t *testing.T) {
	repo, ctx := newEventIdentityRepoForTest(t)

	ended := eventInstance("E-1", "Z-1", "SUPPORT_BREAKDOWN", "RESOLVED", 1, eventSeenAt)
	ended.EndedAt = sql.NullTime{Time: eventSeenAt, Valid: true}
	ended.EndReason = sql.NullString{String: "RESOLVED", Valid: true}
	if err := repo.Apply(ctx, EventIdentityWrite{Instances: []EventInstance{ended}}); err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	// 忘了帶 EndedAt 的重複 upsert 不能讓已終結的鏈復活。
	revive := eventInstance("E-1", "Z-1", "SUPPORT_BREAKDOWN", "ACTIVE", 1, eventSeenAt.Add(time.Hour))
	if err := repo.Apply(ctx, EventIdentityWrite{Instances: []EventInstance{revive}}); err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	got, _ := repo.ListLatestChains(ctx, "0050", "1d")
	if len(got) != 1 || !got[0].EndedAt.Valid {
		t.Errorf("已終結的鏈不可復活，got %+v", got)
	}
}

func TestEventIdentitySymbolScopeChainHasNullZoneUID(t *testing.T) {
	// live 的 86 筆 market_event_states 裡有 12 筆是 SYMBOL scope，
	// 它們不屬於任何 zone。zone_uid 為 NULL，唯一性靠 zone_scope_key。
	repo, ctx := newEventIdentityRepoForTest(t)

	if err := repo.Apply(ctx, EventIdentityWrite{
		Instances: []EventInstance{eventInstance("E-S", SymbolScopeKey, "VOLUME_CONTEXT", "ACTIVE", 1, eventSeenAt)},
	}); err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	got, _ := repo.ListLatestChains(ctx, "0050", "1d")
	if len(got) != 1 || got[0].ZoneUID.Valid {
		t.Fatalf("SYMBOL scope 的 zone_uid 必須是 NULL，got %+v", got)
	}
	if got[0].ZoneScopeKey != SymbolScopeKey {
		t.Errorf("zone_scope_key 要有值才擋得住重複，got %q", got[0].ZoneScopeKey)
	}
}

func TestEventIdentityReasonCodesDefaultsToEmptyArray(t *testing.T) {
	// RawJSON 是純 string，零值會把 '' 寫進 NOT NULL DEFAULT '[]' 的欄位，
	// 等有人 json.Unmarshal 才炸。
	repo, ctx := newEventIdentityRepoForTest(t)

	if err := repo.Apply(ctx, EventIdentityWrite{
		Instances: []EventInstance{eventInstance("E-1", "Z-1", "SUPPORT_BREAKDOWN", "CONFIRMED", 1, eventSeenAt)},
		Transitions: []EventTransition{{
			EventUID:   "E-1",
			ToState:    "CONFIRMED",
			OccurredAt: eventSeenAt,
		}},
	}); err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	// 寫得進去就代表沒有把 '' 塞進去（欄位是 NOT NULL，'' 也過得了，
	// 所以這裡直接讀回來確認內容）。
	var raw string
	if err := eventIdentityScanReasonCodes(t, repo, &raw); err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if raw != "[]" {
		t.Errorf("reason_codes 零值要補成 []，got %q", raw)
	}
}

func eventIdentityScanReasonCodes(t *testing.T, repo EventIdentityRepo, out *string) error {
	t.Helper()
	r, ok := repo.(*eventIdentityRepo)
	if !ok {
		t.Fatal("unexpected repo type")
	}
	return r.db.Get(out, "SELECT reason_codes FROM event_transitions LIMIT 1")
}

func TestEventIdentityUpsertKeepsTerminalStateOnEndedChain(t *testing.T) {
	// **F4 在 DB 層的最後一道**（T-048 階段 C 修法）。ended_at / end_reason 原本就有
	// COALESCE 保護，但 state / active 是無條件覆寫——已終結的鏈被寫入非終態時，
	// ended_at 保住了、state 卻退回 ACTIVE，寫出來仍是「ended_at 有值卻 active=true」
	// 的自相矛盾資料，只是從另一個方向產生。
	repo, ctx := newEventIdentityRepoForTest(t)

	ended := eventInstance("E-1", "Z-1", "SUPPORT_BREAKDOWN", "EXPIRED", 1, eventSeenAt)
	ended.Active = false
	ended.EndedAt = sql.NullTime{Time: eventSeenAt, Valid: true}
	ended.EndReason = sql.NullString{String: "EXPIRED", Valid: true}
	if err := repo.Apply(ctx, EventIdentityWrite{Instances: []EventInstance{ended}}); err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	// 之後某次分析（不論成因）又對同一個 event_uid 寫入 ACTIVE。
	revived := eventInstance("E-1", "Z-1", "SUPPORT_BREAKDOWN", "ACTIVE", 1, eventSeenAt.Add(24*time.Hour))
	if err := repo.Apply(ctx, EventIdentityWrite{Instances: []EventInstance{revived}}); err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	got, err := repo.ListLatestChains(ctx, "0050", "1d")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("unexpected chains: %+v", got)
	}
	if got[0].State != "EXPIRED" || got[0].Active {
		t.Errorf("已終結的鏈不可退回非終態，got state=%q active=%v", got[0].State, got[0].Active)
	}
	if !got[0].EndedAt.Valid {
		t.Errorf("ended_at 也不該被清掉，got %+v", got[0].EndedAt)
	}
}

func TestEventIdentityUpsertRefreshesLastZoneKey(t *testing.T) {
	// last_zone_key 是鏈延續的第一把鑰匙。停在誕生那天的值，往後每天都會 miss，
	// 而 miss 的後果是整條鏈靜默凍結（F1）。
	repo, ctx := newEventIdentityRepoForTest(t)

	first := eventInstance("E-1", "Z-1", "SUPPORT_BREAKDOWN", "CONFIRMED", 1, eventSeenAt)
	first.LastZoneKey = sql.NullString{String: "SUPPORT:104.7300:105.3700", Valid: true}
	if err := repo.Apply(ctx, EventIdentityWrite{Instances: []EventInstance{first}}); err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	next := eventInstance("E-1", "Z-1", "SUPPORT_BREAKDOWN", "ACTIVE", 1, eventSeenAt.Add(24*time.Hour))
	next.LastZoneKey = sql.NullString{String: "SUPPORT:104.7412:105.3688", Valid: true}
	if err := repo.Apply(ctx, EventIdentityWrite{Instances: []EventInstance{next}}); err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	got, err := repo.ListLatestChains(ctx, "0050", "1d")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].LastZoneKey.String != "SUPPORT:104.7412:105.3688" {
		t.Fatalf("last_zone_key 要更新成最近一次觀測到的 key，got %+v", got)
	}
}

// decision_visible 要能寫進去也讀得回來，而且**重新觀測不會把它翻回預設值**。
// 這個欄位錯了資料看起來完全正常：鏈還在、狀態也對，只是決策可見性反了，
// 而顯示端會照它把「只寫不讀的事實紀錄」畫成會影響進場的事件。
func TestEventIdentityUpsertKeepsDecisionVisibleFalse(t *testing.T) {
	repo, ctx := newEventIdentityRepoForTest(t)

	first := eventInstance("E-1", "Z-1", "RESISTANCE_BREAKOUT", "CANDIDATE", 1, eventSeenAt)
	first.DecisionVisible = false
	if err := repo.Apply(ctx, EventIdentityWrite{Instances: []EventInstance{first}}); err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	next := eventInstance("E-1", "Z-1", "RESISTANCE_BREAKOUT", "CONFIRMED", 1, eventSeenAt.Add(24*time.Hour))
	next.DecisionVisible = false
	if err := repo.Apply(ctx, EventIdentityWrite{Instances: []EventInstance{next}}); err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	got, err := repo.ListLatestChains(ctx, "0050", "1d")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("要恰好一條鏈，got %+v", got)
	}
	if got[0].DecisionVisible {
		t.Errorf("重新觀測不該把 decision_visible 翻回預設值，got %+v", got[0])
	}
	if got[0].State != "CONFIRMED" {
		t.Errorf("其餘欄位照常更新，got %+v", got[0])
	}
}

// 對照組：既有事件（沒有這個旗標的那四個型別）一律是決策可見的。
func TestEventIdentityDecisionVisibleDefaultsToTrue(t *testing.T) {
	repo, ctx := newEventIdentityRepoForTest(t)

	inst := eventInstance("E-1", "Z-1", "SUPPORT_BREAKDOWN", "CONFIRMED", 1, eventSeenAt)
	if err := repo.Apply(ctx, EventIdentityWrite{Instances: []EventInstance{inst}}); err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	got, err := repo.ListChains(ctx, "0050", "1d", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !got[0].DecisionVisible {
		t.Fatalf("既有事件要是決策可見的，got %+v", got)
	}
}

// ── GetIdentitySince（原記於 todo.md T-051 R5，已收斂）────────────────────

// eventCandleDay 是 K 棒軸上的日期，與 eventSeenAt（as_of wall clock）刻意差好幾天：
// 任何把兩條軸搞混的實作都會在斷言上炸開。
func eventCandleDay(d int) time.Time {
	return time.Date(2026, 7, d, 0, 0, 0, 0, time.UTC)
}

// seedAnalysis 種一列 stock_sr_zone_analyses；identity_since 的時間軸就取自它的 analyzed_at。
func seedAnalysis(t *testing.T, db *sqlx.DB, id int64, at time.Time) {
	t.Helper()
	_, err := db.Exec(db.Rebind(`
		INSERT INTO stock_sr_zone_analyses
			(id, symbol, timeframe, analyzed_at, current_price, global_trend, global_volatility, model_version)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`), id, "0050", "1d", at, 100.0, 0.0, 0.0, "test")
	if err != nil {
		t.Fatalf("seed analysis %d failed: %v", id, err)
	}
}

func TestGetIdentitySinceUsesAnalyzedAt(t *testing.T) {
	db, repo, ctx := newEventIdentityStackForTest(t)
	seedAnalysis(t, db, 1, eventCandleDay(3))
	seedAnalysis(t, db, 2, eventCandleDay(9))

	if err := repo.Apply(ctx, EventIdentityWrite{
		Instances: []EventInstance{eventInstance("E-1", "Z-1", "SUPPORT_BREAKDOWN", "CONFIRMED", 1, eventSeenAt)},
		Transitions: []EventTransition{
			{EventUID: "E-1", AnalysisID: sql.NullInt64{Int64: 2, Valid: true}, FromState: sql.NullString{String: "CANDIDATE", Valid: true}, ToState: "CONFIRMED", OccurredAt: eventSeenAt},
			{EventUID: "E-1", AnalysisID: sql.NullInt64{Int64: 1, Valid: true}, ToState: "CANDIDATE", OccurredAt: eventSeenAt},
		},
	}); err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	got, err := repo.GetIdentitySince(ctx, "0050", "1d")
	if err != nil {
		t.Fatalf("get identity since failed: %v", err)
	}
	if !got.Valid || !got.Time.UTC().Equal(eventCandleDay(3)) {
		t.Errorf("identity_since = %v, want 最早那次分析的 K 棒時間 %v", got, eventCandleDay(3))
	}
}

// 鏈由排程收尾時 analysis_id 是 NULL，那一步只能用 occurred_at 的 wall clock。
func TestGetIdentitySinceFallsBackToOccurredAt(t *testing.T) {
	db, repo, ctx := newEventIdentityStackForTest(t)
	seedAnalysis(t, db, 1, eventCandleDay(9))
	// **要比有分析的那一步更早**，否則最早的一步仍是 analyzed_at，這條測試就沒有
	// 測到 fallback（第一版就是這樣寫錯的）。
	noAnalysisAt := eventCandleDay(9).Add(-72 * time.Hour)

	if err := repo.Apply(ctx, EventIdentityWrite{
		Instances: []EventInstance{eventInstance("E-1", "Z-1", "SUPPORT_BREAKDOWN", "CONFIRMED", 1, eventSeenAt)},
		Transitions: []EventTransition{
			{EventUID: "E-1", ToState: "CANDIDATE", OccurredAt: noAnalysisAt},
			{EventUID: "E-1", AnalysisID: sql.NullInt64{Int64: 1, Valid: true}, FromState: sql.NullString{String: "CANDIDATE", Valid: true}, ToState: "CONFIRMED", OccurredAt: eventSeenAt},
		},
	}); err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	got, err := repo.GetIdentitySince(ctx, "0050", "1d")
	if err != nil {
		t.Fatalf("get identity since failed: %v", err)
	}
	if !got.Valid || !got.Time.UTC().Equal(noAnalysisAt) {
		t.Errorf("identity_since = %v, want 退回 occurred_at %v", got, noAnalysisAt)
	}
}

// 只有 instances 沒有 transitions 是寫入端的異常，但鏈仍要被算進 identity_since——
// 靜靜消失會讓「身分層從哪天開始有紀錄」答錯。
func TestGetIdentitySinceFallsBackToFirstSeen(t *testing.T) {
	repo, ctx := newEventIdentityRepoForTest(t)
	if err := repo.Apply(ctx, EventIdentityWrite{
		Instances: []EventInstance{eventInstance("E-1", "Z-1", "SUPPORT_BREAKDOWN", "CONFIRMED", 1, eventSeenAt)},
	}); err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	got, err := repo.GetIdentitySince(ctx, "0050", "1d")
	if err != nil {
		t.Fatalf("get identity since failed: %v", err)
	}
	if !got.Valid || !got.Time.UTC().Equal(eventSeenAt) {
		t.Errorf("identity_since = %v, want 退回 first_seen_at %v", got, eventSeenAt)
	}
}

func TestGetIdentitySinceReturnsNullWithoutChains(t *testing.T) {
	repo, ctx := newEventIdentityRepoForTest(t)
	got, err := repo.GetIdentitySince(ctx, "0050", "1d")
	if err != nil {
		t.Fatalf("get identity since failed: %v", err)
	}
	if got.Valid {
		t.Errorf("這檔沒有任何鏈時應為 NULL，得到 %v", got.Time)
	}
}

// **R5 的回歸測試**：一條早已終結、會被 ListChains 的視窗濾掉的舊鏈，仍要被算進
// identity_since。這正是原本「由回傳的 chains 推導」會答錯的情況——畫面會宣告
// 「更早的分析沒有事件鏈」，而那條鏈其實存在，只是這次沒查到。
func TestGetIdentitySinceIgnoresListChainsWindow(t *testing.T) {
	db, repo, ctx := newEventIdentityStackForTest(t)
	seedAnalysis(t, db, 1, eventCandleDay(2))
	seedAnalysis(t, db, 2, eventCandleDay(28))

	old := eventInstance("E-OLD", "Z-1", "SUPPORT_BREAKDOWN", "EXPIRED", 1, eventSeenAt)
	old.Active = false
	old.EndedAt = sql.NullTime{Time: eventSeenAt, Valid: true}
	old.EndReason = sql.NullString{String: "EXPIRED", Valid: true}

	if err := repo.Apply(ctx, EventIdentityWrite{
		Instances: []EventInstance{old, eventInstance("E-NEW", "Z-2", "SUPPORT_BREAKDOWN", "CONFIRMED", 1, eventSeenAt)},
		Transitions: []EventTransition{
			{EventUID: "E-OLD", AnalysisID: sql.NullInt64{Int64: 1, Valid: true}, ToState: "CONFIRMED", OccurredAt: eventSeenAt},
			{EventUID: "E-NEW", AnalysisID: sql.NullInt64{Int64: 2, Valid: true}, ToState: "CONFIRMED", OccurredAt: eventSeenAt},
		},
	}); err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	// 前提：視窗確實把舊鏈濾掉了，否則這條測試什麼都沒證明。
	chains, err := repo.ListChains(ctx, "0050", "1d", eventCandleDay(20))
	if err != nil {
		t.Fatalf("list chains failed: %v", err)
	}
	if len(chains) != 1 || chains[0].EventUID != "E-NEW" {
		t.Fatalf("視窗應只留下 E-NEW，得到 %d 條", len(chains))
	}

	got, err := repo.GetIdentitySince(ctx, "0050", "1d")
	if err != nil {
		t.Fatalf("get identity since failed: %v", err)
	}
	if !got.Valid || !got.Time.UTC().Equal(eventCandleDay(2)) {
		t.Errorf("identity_since = %v, want 視窗外舊鏈的 %v", got, eventCandleDay(2))
	}
}
