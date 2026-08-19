package store

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/database"
)

// **只跑 sqlite**，與其他 repo 測試相同的既有限制（issue.md I-054 第 1 項）：
// repo 層 CRUD 從未對真實 MySQL 驗證過。mysql 的 DDL 由
// scripts/test-mysql-migrations.sh 涵蓋，CRUD 沒有——本 repo 又多一個未驗證案例。
func newZoneIdentityRepoForTest(t *testing.T) (ZoneIdentityRepo, context.Context) {
	t.Helper()
	tmp, err := os.CreateTemp("", "zone-identity-test-*.db")
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
	return NewZoneIdentityRepo(db), context.Background()
}

var zoneSeenAt = time.Date(2026, 8, 18, 7, 0, 0, 0, time.UTC)

// 真實形狀：0050 recent_pivot [104.73,105.37]，就是 live 那筆邊界完全沒動卻翻轉的 zone。
func zoneInstance(uid string, low, high float64, seen time.Time) ZoneInstance {
	return ZoneInstance{
		ZoneUID:     uid,
		Symbol:      "0050",
		Timeframe:   "1d",
		Method:      "recent_pivot",
		State:       "ACTIVE",
		PriceLow:    low,
		PriceHigh:   high,
		FirstSeenAt: seen,
		LastSeenAt:  seen,
	}
}

func zoneIncarnation(uid, zoneUID, role string, seq int) ZoneRoleIncarnation {
	return ZoneRoleIncarnation{
		IncarnationUID: uid,
		ZoneUID:        zoneUID,
		Seq:            seq,
		Role:           role,
		State:          "ACTIVE",
		StartedAt:      zoneSeenAt,
	}
}

func TestZoneIdentityApplyWritesAllFourTables(t *testing.T) {
	repo, ctx := newZoneIdentityRepoForTest(t)

	err := repo.Apply(ctx, ZoneIdentityWrite{
		Instances: []ZoneInstance{
			zoneInstance("Z-P", 104.73, 105.37, zoneSeenAt),
			zoneInstance("Z-C1", 104.70, 105.20, zoneSeenAt),
		},
		Incarnations: []ZoneRoleIncarnation{zoneIncarnation("I-1", "Z-P", "SUPPORT", 1)},
		Transitions: []ZoneTransition{{
			ZoneUID:        "Z-P",
			IncarnationUID: sql.NullString{String: "I-1", Valid: true},
			TransitionKind: "ROLE_FLIPPED",
			FromRole:       sql.NullString{String: "SUPPORT", Valid: true},
			ToRole:         sql.NullString{String: "RESISTANCE", Valid: true},
			ReasonCodes:    RawJSON(`["ROLE_FLIP_OBSERVED"]`),
			OccurredAt:     zoneSeenAt,
		}},
		Relations: []ZoneRelation{{
			ParentZoneUID: "Z-P",
			ChildZoneUID:  "Z-C1",
			Relation:      "SPLIT",
			OccurredAt:    zoneSeenAt,
		}},
	})
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	live, err := repo.ListLive(ctx, "0050", "1d", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(live) != 2 {
		t.Fatalf("want 2 live zones, got %d", len(live))
	}
}

func TestZoneIdentityListLiveJoinsOpenIncarnationRole(t *testing.T) {
	repo, ctx := newZoneIdentityRepoForTest(t)

	// 一世已結束（翻轉），下一世是 RESISTANCE。ListLive 要回**還沒結束**的那一筆，
	// 否則 matcher 會拿到過期的角色而漏判下一次翻轉。
	if err := repo.Apply(ctx, ZoneIdentityWrite{
		Instances: []ZoneInstance{zoneInstance("Z-P", 104.73, 105.37, zoneSeenAt)},
		Incarnations: []ZoneRoleIncarnation{
			{
				IncarnationUID: "I-1", ZoneUID: "Z-P", Seq: 1, Role: "SUPPORT",
				State: "INVALIDATED", StartedAt: zoneSeenAt,
				EndedAt:   sql.NullTime{Time: zoneSeenAt, Valid: true},
				EndReason: sql.NullString{String: "ROLE_FLIPPED", Valid: true},
			},
			zoneIncarnation("I-2", "Z-P", "RESISTANCE", 2),
		},
	}); err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	live, _ := repo.ListLive(ctx, "0050", "1d", 3)
	if len(live) != 1 {
		t.Fatalf("want 1 row, got %d", len(live))
	}
	if live[0].IncarnationRole.String != "RESISTANCE" {
		t.Errorf("應回當前這一世的角色 RESISTANCE，得到 %q", live[0].IncarnationRole.String)
	}
	if live[0].IncarnationUID.String != "I-2" {
		t.Errorf("應回未結束的那一世 I-2，得到 %q", live[0].IncarnationUID.String)
	}
}

func TestZoneIdentityListLiveKeepsLongAbsentIdentitiesSoTheyCanBeCollected(t *testing.T) {
	repo, ctx := newZoneIdentityRepoForTest(t)

	old := zoneSeenAt.Add(-180 * 24 * time.Hour)
	if err := repo.Apply(ctx, ZoneIdentityWrite{
		Instances: []ZoneInstance{
			zoneInstance("Z-OLD", 104.73, 105.37, old),
			zoneInstance("Z-NEW", 200.00, 201.00, zoneSeenAt),
		},
	}); err != nil {
		t.Fatal(err)
	}

	// **刻意沒有時間下界。** 加了的話，超過下界的身分會被 SQL 擋在 matcher 之前，
	// 於是永遠不會被判失格、永遠不會收攤，就這樣以 ACTIVE 留在表裡——
	// 與次數軸用 `<` 會造成的死碼是同一個洞，只是換一個軸。
	// 時間軸的判定屬於 matcher（它才有交易日曆）。
	live, _ := repo.ListLive(ctx, "0050", "1d", 3)
	if len(live) != 2 {
		t.Fatalf("消失很久的身分仍要撈得出來才收得掉，得到 %+v", live)
	}
}

func TestZoneIdentityListLiveExcludesEndedIdentities(t *testing.T) {
	repo, ctx := newZoneIdentityRepoForTest(t)

	ended := zoneInstance("Z-SPLIT", 104.73, 105.37, zoneSeenAt)
	ended.State = "SPLIT"
	ended.EndedAt = sql.NullTime{Time: zoneSeenAt, Valid: true}
	if err := repo.Apply(ctx, ZoneIdentityWrite{Instances: []ZoneInstance{ended}}); err != nil {
		t.Fatal(err)
	}

	live, _ := repo.ListLive(ctx, "0050", "1d", 3)
	if len(live) != 0 {
		t.Fatalf("身分已終止不該回傳，得到 %+v", live)
	}
}

func TestZoneIdentityApplyIsIdempotentForSameAnalysis(t *testing.T) {
	repo, ctx := newZoneIdentityRepoForTest(t)

	w := ZoneIdentityWrite{
		Instances: []ZoneInstance{
			zoneInstance("Z-P", 104.73, 105.37, zoneSeenAt),
			zoneInstance("Z-C", 104.70, 105.20, zoneSeenAt),
		},
		Incarnations: []ZoneRoleIncarnation{zoneIncarnation("I-1", "Z-P", "SUPPORT", 1)},
		Relations: []ZoneRelation{{
			ParentZoneUID: "Z-P", ChildZoneUID: "Z-C",
			Relation: "SPLIT", OccurredAt: zoneSeenAt,
		}},
	}

	// 排程重跑同一次分析必須安全。血緣邊的主鍵是 (parent, child, occurred_at)，
	// 沒有 DO NOTHING 的話第二次會直接撞主鍵。
	if err := repo.Apply(ctx, w); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	if err := repo.Apply(ctx, w); err != nil {
		t.Fatalf("重跑同一次分析不該失敗: %v", err)
	}
}

func TestZoneIdentityApplyUpdatesLastSeenButKeepsFirstSeen(t *testing.T) {
	repo, ctx := newZoneIdentityRepoForTest(t)

	first := zoneSeenAt.Add(-10 * 24 * time.Hour)
	if err := repo.Apply(ctx, ZoneIdentityWrite{
		Instances: []ZoneInstance{zoneInstance("Z-P", 104.73, 105.37, first)},
	}); err != nil {
		t.Fatal(err)
	}

	later := zoneInstance("Z-P", 104.80, 105.40, zoneSeenAt)
	later.FirstSeenAt = zoneSeenAt // 呼叫端算錯也不該覆寫
	if err := repo.Apply(ctx, ZoneIdentityWrite{Instances: []ZoneInstance{later}}); err != nil {
		t.Fatal(err)
	}

	live, _ := repo.ListLive(ctx, "0050", "1d", 3)
	if len(live) != 1 {
		t.Fatalf("want 1 row, got %d", len(live))
	}
	// first_seen_at 是「這個價位第一次出現」的事實，重寫會讓身分壽命失真。
	if !live[0].FirstSeenAt.Equal(first) {
		t.Errorf("first_seen_at 不該被覆寫，want %v got %v", first, live[0].FirstSeenAt)
	}
	if !live[0].LastSeenAt.Equal(zoneSeenAt) {
		t.Errorf("last_seen_at 應更新，want %v got %v", zoneSeenAt, live[0].LastSeenAt)
	}
	if live[0].PriceLow != 104.80 {
		t.Errorf("邊界應更新為最近觀測值，得到 %v", live[0].PriceLow)
	}
}

func TestZoneIdentityRejectsSelfLoopRelation(t *testing.T) {
	repo, ctx := newZoneIdentityRepoForTest(t)

	err := repo.Apply(ctx, ZoneIdentityWrite{
		Instances: []ZoneInstance{zoneInstance("Z-P", 104.73, 105.37, zoneSeenAt)},
		Relations: []ZoneRelation{{
			ParentZoneUID: "Z-P", ChildZoneUID: "Z-P",
			Relation: "CONTINUE", OccurredAt: zoneSeenAt,
		}},
	})

	// 自環會讓沿 parent 遞迴回溯祖先的查詢無法終止。schema 有 CHECK，
	// 但在 repo 層先擋是為了讓錯誤訊息指得出是哪一筆。
	if err == nil {
		t.Fatal("自環的血緣邊必須被拒絕")
	}
}

func TestZoneIdentityApplyIsAtomic(t *testing.T) {
	repo, ctx := newZoneIdentityRepoForTest(t)

	// relation 指向不存在的身分 → 外鍵失敗。instances 那半也不該留下來，
	// 否則血緣圖會出現無父的孤兒，而那與「新生」在資料上無法區分。
	err := repo.Apply(ctx, ZoneIdentityWrite{
		Instances: []ZoneInstance{zoneInstance("Z-P", 104.73, 105.37, zoneSeenAt)},
		Relations: []ZoneRelation{{
			ParentZoneUID: "Z-P", ChildZoneUID: "Z-MISSING",
			Relation: "SPLIT", OccurredAt: zoneSeenAt,
		}},
	})
	if err == nil {
		t.Fatal("外鍵不存在時應該失敗")
	}

	live, _ := repo.ListLive(ctx, "0050", "1d", 3)
	if len(live) != 0 {
		t.Fatalf("交易失敗後不該留下任何身分，得到 %+v", live)
	}
}

func TestZoneIdentityListLiveExcludesZonesPastTheAbsenceLimit(t *testing.T) {
	repo, ctx := newZoneIdentityRepoForTest(t)

	past := zoneInstance("Z-GONE", 104.73, 105.37, zoneSeenAt)
	past.ObservedAbsences = 4 // 已收攤過（收攤時 +1 越過上限）
	ok := zoneInstance("Z-OK", 200.00, 201.00, zoneSeenAt)
	ok.ObservedAbsences = 2
	if err := repo.Apply(ctx, ZoneIdentityWrite{
		Instances: []ZoneInstance{past, ok},
	}); err != nil {
		t.Fatal(err)
	}

	// 次數軸的粗篩在 SQL（不需要交易日曆）；剛好等於上限的那一筆仍要放進來一次，
	// 見 TestZoneIdentityListLiveIncludesZoneAtAbsenceLimitSoItCanExpire。
	live, _ := repo.ListLive(ctx, "0050", "1d", 3)
	if len(live) != 1 || live[0].ZoneUID != "Z-OK" {
		t.Fatalf("已越過上限的身分不該再進候選集合，得到 %+v", live)
	}
	if live[0].ObservedAbsences != 2 {
		t.Errorf("observed_absences 應原樣讀回，得到 %d", live[0].ObservedAbsences)
	}
}

// ── review 修正的回歸測試 ──

func TestZoneIdentityListLiveIncludesZoneAtAbsenceLimitSoItCanExpire(t *testing.T) {
	repo, ctx := newZoneIdentityRepoForTest(t)

	atLimit := zoneInstance("Z-LIMIT", 104.73, 105.37, zoneSeenAt)
	atLimit.ObservedAbsences = 3
	past := zoneInstance("Z-PAST", 200.00, 201.00, zoneSeenAt)
	past.ObservedAbsences = 4
	if err := repo.Apply(ctx, ZoneIdentityWrite{
		Instances: []ZoneInstance{atLimit, past},
	}); err != nil {
		t.Fatal(err)
	}

	// **剛好累到上限的必須還撈得出來一次**，否則它進不了 matcher、不會出現在
	// expired_previous，就沒有任何東西會把它收成 EXPIRED——整條收攤流程變成死碼。
	// 收攤時次數 +1，之後才真正消失。
	live, _ := repo.ListLive(ctx, "0050", "1d", 3)
	if len(live) != 1 || live[0].ZoneUID != "Z-LIMIT" {
		t.Fatalf("上限那筆要進得來、超過的要擋掉，得到 %+v", live)
	}
}

func TestZoneIdentityListLivePicksLatestOpenIncarnation(t *testing.T) {
	repo, ctx := newZoneIdentityRepoForTest(t)

	// schema 沒有「每個身分最多一筆 ended_at IS NULL」的約束（mysql 給不起對等寫法），
	// 所以查詢不能假設它。出現兩筆未結束時 LEFT JOIN 會放大列數，
	// matcher 的 previous 就會有同一身分兩次，1→1 的 CONTINUE 被判成 2→1 的 MERGE。
	if err := repo.Apply(ctx, ZoneIdentityWrite{
		Instances: []ZoneInstance{zoneInstance("Z-P", 104.73, 105.37, zoneSeenAt)},
		Incarnations: []ZoneRoleIncarnation{
			zoneIncarnation("I-1", "Z-P", "SUPPORT", 1),
			zoneIncarnation("I-2", "Z-P", "RESISTANCE", 2),
		},
	}); err != nil {
		t.Fatal(err)
	}

	live, _ := repo.ListLive(ctx, "0050", "1d", 3)
	if len(live) != 1 {
		t.Fatalf("同一身分不該被放大成多列，得到 %d 列", len(live))
	}
	if live[0].IncarnationUID.String != "I-2" {
		t.Errorf("應取 seq 最大的未結束一世 I-2，得到 %q", live[0].IncarnationUID.String)
	}
}

func TestZoneIdentityUpsertKeepsLastSeenMonotonic(t *testing.T) {
	repo, ctx := newZoneIdentityRepoForTest(t)

	if err := repo.Apply(ctx, ZoneIdentityWrite{
		Instances: []ZoneInstance{zoneInstance("Z-P", 104.73, 105.37, zoneSeenAt)},
	}); err != nil {
		t.Fatal(err)
	}

	// 「這次沒看到、只想把 absences +1」的寫入若順手填了較早或較晚的時間，
	// 都不該讓 last_seen_at 倒退——倒退等於謊報「它更久沒出現」，
	// 前進則等於宣告「它剛被看到」而讓時間軸閘門永遠不觸發。
	older := zoneInstance("Z-P", 104.73, 105.37, zoneSeenAt.Add(-72*time.Hour))
	older.ObservedAbsences = 1
	if err := repo.Apply(ctx, ZoneIdentityWrite{Instances: []ZoneInstance{older}}); err != nil {
		t.Fatal(err)
	}

	live, _ := repo.ListLive(ctx, "0050", "1d", 3)
	if !live[0].LastSeenAt.Equal(zoneSeenAt) {
		t.Errorf("last_seen_at 不該倒退，want %v got %v", zoneSeenAt, live[0].LastSeenAt)
	}
	if live[0].ObservedAbsences != 1 {
		t.Errorf("observed_absences 仍須更新，得到 %d", live[0].ObservedAbsences)
	}
}

func TestZoneIdentityUpsertDoesNotResurrectEndedIdentity(t *testing.T) {
	repo, ctx := newZoneIdentityRepoForTest(t)

	ended := zoneInstance("Z-P", 104.73, 105.37, zoneSeenAt)
	ended.State = "SPLIT"
	ended.EndedAt = sql.NullTime{Time: zoneSeenAt, Valid: true}
	if err := repo.Apply(ctx, ZoneIdentityWrite{Instances: []ZoneInstance{ended}}); err != nil {
		t.Fatal(err)
	}

	// 忘了帶 EndedAt 的重複 upsert 不該把已終止的身分復活。
	revive := zoneInstance("Z-P", 104.73, 105.37, zoneSeenAt)
	if err := repo.Apply(ctx, ZoneIdentityWrite{Instances: []ZoneInstance{revive}}); err != nil {
		t.Fatal(err)
	}

	var endedAt sql.NullTime
	row := repo.(*zoneIdentityRepo).db.QueryRowContext(ctx,
		"SELECT ended_at FROM zone_instances WHERE zone_uid = 'Z-P'")
	if err := row.Scan(&endedAt); err != nil {
		t.Fatal(err)
	}
	if !endedAt.Valid {
		t.Error("ended_at 被清掉了——已終止的身分不該被 upsert 復活")
	}
}

func TestZoneIdentityRejectsDuplicateInstancesInOneBatch(t *testing.T) {
	repo, ctx := newZoneIdentityRepoForTest(t)

	// 同批重複的 zone_uid 只有 postgres 會炸，sqlite/mysql 吞得下去。
	// 測試只跑 sqlite，所以這道 Go 層的檢查是唯一測得到的防線。
	err := repo.Apply(ctx, ZoneIdentityWrite{
		Instances: []ZoneInstance{
			zoneInstance("Z-DUP", 104.73, 105.37, zoneSeenAt),
			zoneInstance("Z-DUP", 104.80, 105.40, zoneSeenAt),
		},
	})
	if err == nil {
		t.Fatal("同批重複 zone_uid 必須被拒絕，否則只在 postgres 上失敗")
	}
}

func TestZoneIdentityNormalisesEmptyReasonCodes(t *testing.T) {
	repo, ctx := newZoneIdentityRepoForTest(t)

	// RawJSON 零值是 ''，會被寫進 NOT NULL DEFAULT '[]' 的欄位（DEFAULT 不生效，
	// 因為欄位有被列在 INSERT 裡）。本批只寫不讀，'' 會安靜累積到階段 C 才炸。
	if err := repo.Apply(ctx, ZoneIdentityWrite{
		Instances: []ZoneInstance{zoneInstance("Z-P", 104.73, 105.37, zoneSeenAt)},
		Transitions: []ZoneTransition{{
			ZoneUID:        "Z-P",
			TransitionKind: "STATE_CHANGE",
			OccurredAt:     zoneSeenAt,
		}},
	}); err != nil {
		t.Fatal(err)
	}

	var raw string
	row := repo.(*zoneIdentityRepo).db.QueryRowContext(ctx,
		"SELECT reason_codes FROM zone_transitions WHERE zone_uid = 'Z-P'")
	if err := row.Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if raw != "[]" {
		t.Errorf("空的 reason_codes 應正規化為 []，得到 %q", raw)
	}
}

func TestZoneIdentityListTradingDaysReturnsDistinctDatesNewestFirst(t *testing.T) {
	repo, ctx := newZoneIdentityRepoForTest(t)
	db := repo.(*zoneIdentityRepo).db

	// 同一天兩筆（不同 symbol）只算一個交易日；週末沒有 K 棒所以自然不在清單裡。
	for _, row := range []struct {
		symbol string
		ts     string
	}{
		{"0050", "2026-08-14"}, {"2330", "2026-08-14"},
		{"0050", "2026-08-17"}, {"0050", "2026-08-18"},
	} {
		if _, err := db.ExecContext(ctx,
			`INSERT INTO candles (symbol, timeframe, open, high, low, close, volume, amount, ts)
			 VALUES (?, '1d', 1, 1, 1, 1, 1, 1, ?)`, row.symbol, row.ts); err != nil {
			t.Fatal(err)
		}
	}

	days, err := repo.ListTradingDays(ctx, "1d", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(days) != 3 {
		t.Fatalf("同一天多檔只算一個交易日，want 3 got %d：%v", len(days), days)
	}
	// 由新到舊——與 Python 端 fetch_market_trading_days 同一個順序約定。
	// matcher 端的 TradingCalendar.from_iterable 會負責排成升冪。
	if days[0] != "2026-08-18" {
		t.Errorf("應由新到舊，得到 %v", days)
	}
}
