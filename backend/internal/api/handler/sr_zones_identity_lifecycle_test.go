package handler

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/analysis"
	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/database"
	"github.com/trading/backend/internal/store"
)

// synthetic 血緣 integration test（T-048 階段 C，F3 的覆蓋缺口）。
//
// **為什麼需要它**：2026-08-19 的 as-of 階梯裡 45 個 zone 身分**全部 ACTIVE、
// 沒有任何終止**，所以 D4（parent 終止時事件收攤）與 F4 的修法一次都沒被真的執行過。
// 純函數測試證明得了「輸出的欄位對」，證明不了「兩個 repo 的交易、外鍵、upsert 的
// COALESCE / CASE 保護在真的有資料時成立」。
//
// **不經 HTTP**：SRZoneHandler.client 是具體型別 *analysis.Client，為這件事把它
// 抽成介面是跨模組重構，與本輪範圍不成比例。這裡直接組 analysis.ZoneIdentityMatchResult
// 餵進兩個 build 純函數 ＋ 兩個 repo.Apply，逐階推進。HTTP 那一跳仍只由 as-of 階梯覆蓋。

func newIdentityStackForTest(t *testing.T) (*sqlx.DB, store.ZoneIdentityRepo, store.EventIdentityRepo) {
	t.Helper()
	tmp, err := os.CreateTemp("", "identity-lifecycle-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	t.Cleanup(func() { os.Remove(tmp.Name()) })

	db, err := store.NewDB(config.DatabaseConfig{Driver: "sqlite", DSN: tmp.Name()})
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.RunMigrations(context.Background(), db, "sqlite", zap.NewNop()); err != nil {
		t.Fatalf("migrations failed: %v", err)
	}
	return db, store.NewZoneIdentityRepo(db), store.NewEventIdentityRepo(db)
}

func seqUID(prefix string) func() string {
	n := 0
	return func() string {
		n++
		return fmt.Sprintf("%s-%d", prefix, n)
	}
}

func keyedZone(low, high float64, role, key string) store.SRZone {
	return store.SRZone{
		PriceLow: low, PriceHigh: high,
		Method: "recent_pivot", Role: role, ZoneKey: key,
	}
}

// stage 是一階分析：釋出一批 zone、拿到一份 matcher 結果、算出一批事件狀態。
type stage struct {
	name   string
	zones  []store.SRZone
	match  *analysis.ZoneIdentityMatchResult
	states []store.MarketEventState
}

// runStage 複製 persistZoneIdentity → persistEventIdentity 的順序與資料流，
// 只是把 matcher 的 HTTP 呼叫換成 stage 給定的結果。
func runStage(
	t *testing.T, ctx context.Context,
	zoneRepo store.ZoneIdentityRepo, eventRepo store.EventIdentityRepo,
	st stage, now time.Time, analysisID uint64, incUID func() string,
) eventIdentityStats {
	t.Helper()

	live, err := zoneRepo.ListLive(ctx, "0050", "1d", zoneIdentityMaxAbsences)
	if err != nil {
		t.Fatalf("%s: list live: %v", st.name, err)
	}
	// **在 Apply 之前讀**，與 persistZoneIdentity 相同：讀到的是先前各階留下的 key。
	aliasRefs, err := zoneRepo.ListKeyAliases(ctx, "0050", "1d", zoneIdentityMaxAbsences)
	if err != nil {
		t.Fatalf("%s: list aliases: %v", st.name, err)
	}

	zw := buildZoneIdentityWrite("0050", "1d", analysisID, now, st.zones, live, st.match, incUID)
	if err := zoneRepo.Apply(ctx, zw); err != nil {
		t.Fatalf("%s: zone apply: %v", st.name, err)
	}

	aliasByKey, aliasAmbiguous := aliasUIDByZoneKey(aliasRefs, st.match.ExpiredPrevious)
	outcome := &zoneIdentityOutcome{
		UIDByZoneKey:      zoneUIDByZoneKey(st.zones, st.match.ZoneUIDs),
		AliasUIDByZoneKey: aliasByKey,
		AliasAmbiguous:    aliasAmbiguous,
		EndedZoneUIDs:     uidSet(st.match.TerminatedPrevious),
	}

	latest, err := eventRepo.ListLatestChains(ctx, "0050", "1d")
	if err != nil {
		t.Fatalf("%s: list chains: %v", st.name, err)
	}
	ew, stats := buildEventIdentityWrite("0050", "1d", analysisID, now,
		st.states, latest, outcome, seqUID("E-"+st.name))
	if len(ew.Instances) > 0 || len(ew.Transitions) > 0 {
		if err := eventRepo.Apply(ctx, ew); err != nil {
			t.Fatalf("%s: event apply: %v", st.name, err)
		}
	}
	return stats
}

// assertIdentityInvariants 是階梯驗收門檻的 SQL 版本，每一階跑一次。
// 這幾條都是「錯了資料表面上仍然正常」的那種，只有明寫成查詢才看得見。
func assertIdentityInvariants(t *testing.T, db *sqlx.DB, stage string) {
	t.Helper()

	type check struct {
		name string
		sql  string
	}
	checks := []check{{
		// 門檻④：F4。ended_at 有值就代表鏈終結了，state / active 必須跟上。
		name: "ended_at 有值但 state / active 沒跟上",
		sql: `SELECT COUNT(*) FROM event_instances
		      WHERE ended_at IS NOT NULL
		        AND (state NOT IN ('RESOLVED','EXPIRED') OR active = 1)`,
	}, {
		// 門檻②：F2。誕生是唯一 from_state 留白的轉換，而誕生即終態代表
		// 「這次看到」被當成了「這次發生」。
		name: "鏈誕生即終態",
		sql: `SELECT COUNT(*) FROM event_transitions
		      WHERE from_state IS NULL AND to_state IN ('RESOLVED','EXPIRED')`,
	}, {
		// 門檻⑦：匯流點 continue-or-create。兩條活鏈時 ListLatestChains 只回
		// MAX(seq)，舊鏈從此撈不出來——F1 的靜默凍結，換了個成因。
		name: "同一個 (zone_scope_key, family) 有兩條未終結的鏈",
		sql: `SELECT COUNT(*) FROM (
		          SELECT symbol, timeframe, zone_scope_key, event_family
		          FROM event_instances WHERE ended_at IS NULL
		          GROUP BY symbol, timeframe, zone_scope_key, event_family
		          HAVING COUNT(*) > 1
		      ) t`,
	}, {
		// 門檻⑤：事件掛在一個已經不存在的身分上，而且還沒被收掉。
		name: "未終結的鏈指向非 ACTIVE 的身分",
		sql: `SELECT COUNT(*) FROM event_instances e
		      JOIN zone_instances z ON z.zone_uid = e.zone_uid
		      WHERE e.ended_at IS NULL AND z.state <> 'ACTIVE'`,
	}, {
		// D4 的另一面：身分終止了，它身上的鏈就不能還活著。
		name: "身分已終止但鏈還活著",
		sql: `SELECT COUNT(*) FROM event_instances e
		      JOIN zone_instances z ON z.zone_uid = e.zone_uid
		      WHERE z.ended_at IS NOT NULL AND e.ended_at IS NULL`,
	}}

	for _, c := range checks {
		var n int
		if err := db.Get(&n, c.sql); err != nil {
			t.Fatalf("%s: %s: %v", stage, c.name, err)
		}
		if n != 0 {
			t.Errorf("%s: 不變式違反——%s（%d 列）", stage, c.name, n)
		}
	}
}

func TestZoneAndEventIdentityAcrossSplitMergeReshape(t *testing.T) {
	db, zoneRepo, eventRepo := newIdentityStackForTest(t)
	ctx := context.Background()
	incUID := seqUID("INC")
	day := func(n int) time.Time {
		return time.Date(2026, 8, 10+n, 7, 0, 0, 0, time.UTC)
	}

	const (
		keyA = "SUPPORT:104.0000:105.0000"
		keyB = "SUPPORT:99.0000:100.0000"
		// A 分裂出來的兩個 child。
		keyA1 = "SUPPORT:104.0000:104.4000"
		keyA2 = "SUPPORT:104.6000:105.0000"
		// A1 / A2 合併回一個。
		keyM = "SUPPORT:104.0000:105.0000"
		// 血緣無法解析的重整。
		keyR = "SUPPORT:103.2000:104.1000"
	)

	// ── 第一階：兩個新身分，各帶一條事件鏈 ──
	stats := runStage(t, ctx, zoneRepo, eventRepo, stage{
		name:  "s1",
		zones: []store.SRZone{keyedZone(104, 105, "SUPPORT", keyA), keyedZone(99, 100, "SUPPORT", keyB)},
		match: &analysis.ZoneIdentityMatchResult{
			ZoneUIDs:             []string{"Z-A", "Z-B"},
			NextObservedAbsences: map[string]int{},
		},
		states: []store.MarketEventState{
			eventState(keyA, "SUPPORT_BREAKDOWN", "SUPPORT_BREAKDOWN", "CONFIRMED"),
			eventState(keyB, "SUPPORT_RECLAIM", "INTRADAY_RECLAIM", "CONFIRMED"),
		},
	}, day(1), 1, incUID)
	if stats.MatchedByCurrent != 2 {
		t.Fatalf("s1: 兩筆都該由本次 map 命中，got %+v", stats)
	}
	assertIdentityInvariants(t, db, "s1")

	// ── 第二階：Z-A 分裂成 Z-A1 / Z-A2 ──
	// D4：parent 身上的鏈要收掉、**不傳給 child**；child 從 seq=1 重新開始。
	stats = runStage(t, ctx, zoneRepo, eventRepo, stage{
		name: "s2-split",
		zones: []store.SRZone{
			keyedZone(104, 104.4, "SUPPORT", keyA1),
			keyedZone(104.6, 105, "SUPPORT", keyA2),
			keyedZone(99, 100, "SUPPORT", keyB),
		},
		match: &analysis.ZoneIdentityMatchResult{
			ZoneUIDs: []string{"Z-A1", "Z-A2", "Z-B"},
			Relations: []analysis.ZoneIdentityRelation{
				{ParentZoneUID: "Z-A", ChildZoneUID: "Z-A1", Relation: "SPLIT"},
				{ParentZoneUID: "Z-A", ChildZoneUID: "Z-A2", Relation: "SPLIT"},
			},
			TerminatedPrevious:   []string{"Z-A"},
			NextObservedAbsences: map[string]int{},
		},
		states: []store.MarketEventState{
			eventState(keyA1, "SUPPORT_BREAKDOWN", "SUPPORT_BREAKDOWN", "CONFIRMED"),
			// keyB 的鏈這一階沒有新偵測，只是被 carry forward。
			carriedState(keyB, "SUPPORT_RECLAIM", "INTRADAY_RECLAIM", "ACTIVE"),
		},
	}, day(2), 2, incUID)
	assertIdentityInvariants(t, db, "s2-split")

	var parentClosed struct {
		State     string         `db:"state"`
		Active    bool           `db:"active"`
		EndReason sql.NullString `db:"end_reason"`
		EndedAt   sql.NullTime   `db:"ended_at"`
	}
	if err := db.Get(&parentClosed,
		`SELECT state, active, end_reason, ended_at FROM event_instances WHERE zone_uid = 'Z-A'`,
	); err != nil {
		t.Fatalf("parent 的鏈應該還在表裡（收掉，不是刪掉）：%v", err)
	}
	if !parentClosed.EndedAt.Valid || parentClosed.EndReason.String != "ZONE_IDENTITY_ENDED" {
		t.Errorf("parent 的鏈要因身分終止而收攤，got %+v", parentClosed)
	}
	if parentClosed.State != "EXPIRED" || parentClosed.Active {
		t.Errorf("F4：instance 的終態要與 transition 對齊，got state=%q active=%v",
			parentClosed.State, parentClosed.Active)
	}

	// carried ＋ 有活鏈 → 延續，不是新開。護欄只擋「找不到活鏈」的 carried。
	if len(stats.CarriedNoop) != 0 {
		t.Errorf("s2: keyB 有活鏈，不該被 carried 護欄擋下，got %+v", stats.CarriedNoop)
	}

	// ── 第三階：Z-A1 / Z-A2 合併回一個身分 ──
	stats = runStage(t, ctx, zoneRepo, eventRepo, stage{
		name: "s3-merge",
		zones: []store.SRZone{
			keyedZone(104, 105, "SUPPORT", keyM),
			keyedZone(99, 100, "SUPPORT", keyB),
		},
		match: &analysis.ZoneIdentityMatchResult{
			ZoneUIDs: []string{"Z-M", "Z-B"},
			Relations: []analysis.ZoneIdentityRelation{
				{ParentZoneUID: "Z-A1", ChildZoneUID: "Z-M", Relation: "MERGE"},
				{ParentZoneUID: "Z-A2", ChildZoneUID: "Z-M", Relation: "MERGE"},
			},
			TerminatedPrevious:   []string{"Z-A1", "Z-A2"},
			NextObservedAbsences: map[string]int{},
		},
		states: []store.MarketEventState{
			eventState(keyM, "SUPPORT_BREAKDOWN", "SUPPORT_BREAKDOWN", "ACTIVE"),
			eventState(keyB, "SUPPORT_RECLAIM", "INTRADAY_RECLAIM", "RESOLVED"),
		},
	}, day(3), 3, incUID)
	assertIdentityInvariants(t, db, "s3-merge")

	var mergedParents int
	if err := db.Get(&mergedParents,
		`SELECT COUNT(*) FROM event_instances
		 WHERE zone_uid IN ('Z-A1','Z-A2') AND end_reason = 'ZONE_IDENTITY_ENDED'`); err != nil {
		t.Fatal(err)
	}
	if mergedParents != 1 {
		t.Errorf("MERGE 的 parent 身上有鏈的那一個要被收掉，got %d", mergedParents)
	}

	// ── 第四階：Z-M 重整成一個血緣無法解析的新身分 ──
	// 事件帶的 key 是上一階的 keyM，而這次分析只認得 keyR——修法前這裡會關聯失敗。
	stats = runStage(t, ctx, zoneRepo, eventRepo, stage{
		name:  "s4-reshape",
		zones: []store.SRZone{keyedZone(103.2, 104.1, "SUPPORT", keyR)},
		match: &analysis.ZoneIdentityMatchResult{
			ZoneUIDs:             []string{"Z-R"},
			TerminatedPrevious:   []string{"Z-M"},
			ExpiredPrevious:      []string{},
			NextObservedAbsences: map[string]int{"Z-B": 1},
		},
		states: []store.MarketEventState{
			// 事件仍帶著 Z-M 的 key：鏈以 last_zone_key 命中，但身分本次終止，
			// 所以要交給 D4 而不是延續。
			carriedState(keyM, "SUPPORT_BREAKDOWN", "SUPPORT_BREAKDOWN", "ACTIVE"),
			eventState(keyR, "SUPPORT_BREAKDOWN", "SUPPORT_BREAKDOWN", "CONFIRMED"),
		},
	}, day(4), 4, incUID)
	assertIdentityInvariants(t, db, "s4-reshape")

	if len(stats.ZoneEndedSkipped) != 1 {
		t.Errorf("s4: 掛在終止身分上的事件要交給 D4，got %+v", stats.ZoneEndedSkipped)
	}
	var reshapedClosed int
	if err := db.Get(&reshapedClosed,
		`SELECT COUNT(*) FROM event_instances
		 WHERE zone_uid = 'Z-M' AND state = 'EXPIRED' AND active = 0
		   AND end_reason = 'ZONE_IDENTITY_ENDED'`); err != nil {
		t.Fatal(err)
	}
	if reshapedClosed != 1 {
		t.Errorf("RESHAPE 的 parent 鏈要收乾淨，got %d", reshapedClosed)
	}

	// 每一次身分終止都要留痕，且 from_state 不可留白（留白等同誕生）。
	var closingTransitions int
	if err := db.Get(&closingTransitions,
		`SELECT COUNT(*) FROM event_transitions
		 WHERE reason_codes LIKE '%ZONE_IDENTITY_ENDED%'
		   AND from_state IS NOT NULL AND to_state = 'EXPIRED'`); err != nil {
		t.Fatal(err)
	}
	if closingTransitions != 3 {
		t.Errorf("SPLIT / MERGE / RESHAPE 各該留下一筆收攤轉換，got %d", closingTransitions)
	}
}

func TestEventIdentityKeepsChainAliveWhileZoneKeyDrifts(t *testing.T) {
	// **F1 的端到端回歸。** 同一個身分連續三階，但每一階的 zone_key 都被 ATR 推走
	// （第三階連 role 都進了 AT_ZONE），而事件身上帶的仍是**第一階**那個 key
	// （Python 把狀態 carry forward）。
	//
	// 修法前：本次 map 對不上 → 事件被跳過 → 鏈停在第一階、last_seen_at 不再更新，
	// 資料表面上完全正常。這正是 live 上 0c2bbbe4 的 SUPPORT_RECLAIM 發生的事。
	//
	// 修法後有兩道都接得住它：第一把鑰匙（last_zone_key）先命中，所以連 key 都不必
	// 解析；alias history 是給「鏈還不存在」的新關聯用的第二道，由單元測試覆蓋。
	db, zoneRepo, eventRepo := newIdentityStackForTest(t)
	ctx := context.Background()
	incUID := seqUID("INC")
	day := func(n int) time.Time { return time.Date(2026, 8, 10+n, 7, 0, 0, 0, time.UTC) }

	const firstKey = "SUPPORT:104.0000:105.0000"
	drifting := []string{firstKey, "SUPPORT:104.0500:105.0400", "AT_ZONE:104.0900:105.0700"}
	roles := []string{"SUPPORT", "SUPPORT", "AT_ZONE"}

	for i, key := range drifting {
		st := stage{
			name:  fmt.Sprintf("s%d", i+1),
			zones: []store.SRZone{keyedZone(104, 105, roles[i], key)},
			match: &analysis.ZoneIdentityMatchResult{
				ZoneUIDs:             []string{"Z-1"},
				NextObservedAbsences: map[string]int{},
			},
			// 事件永遠帶著第一階的 key。
			states: []store.MarketEventState{
				carriedState(firstKey, "SUPPORT_RECLAIM", "INTRADAY_RECLAIM", "CONFIRMED"),
			},
		}
		if i == 0 {
			st.states = []store.MarketEventState{
				eventState(firstKey, "SUPPORT_RECLAIM", "INTRADAY_RECLAIM", "CONFIRMED"),
			}
		}
		stats := runStage(t, ctx, zoneRepo, eventRepo, st, day(i+1), uint64(i+1), incUID)
		if len(stats.UnmatchedKeys) != 0 {
			t.Fatalf("%s: 不該有關聯失敗，got %v", st.name, stats.UnmatchedKeys)
		}
		assertIdentityInvariants(t, db, st.name)
	}

	var chain struct {
		EventUID    string         `db:"event_uid"`
		Seq         int            `db:"seq"`
		LastSeenAt  time.Time      `db:"last_seen_at"`
		LastZoneKey sql.NullString `db:"last_zone_key"`
	}
	if err := db.Get(&chain,
		`SELECT event_uid, seq, last_seen_at, last_zone_key FROM event_instances`); err != nil {
		t.Fatalf("應該只有一條鏈：%v", err)
	}
	if chain.Seq != 1 {
		t.Errorf("三階都是同一條鏈，seq 不該前進，got %d", chain.Seq)
	}
	if !chain.LastSeenAt.Equal(day(3)) {
		t.Errorf("鏈不該凍結，last_seen_at 要走到最後一階，got %v", chain.LastSeenAt)
	}
	if chain.LastZoneKey.String != firstKey {
		t.Errorf("last_zone_key 記的是事件帶的 key，got %+v", chain.LastZoneKey)
	}
}
