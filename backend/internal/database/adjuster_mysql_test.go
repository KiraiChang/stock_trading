package database_test

import (
	"context"
	"math"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	_ "github.com/go-sql-driver/mysql"

	"github.com/trading/backend/internal/api/handler"
	"github.com/trading/backend/internal/database"
	"github.com/trading/backend/internal/store"
)

// mysql 的欄位寬度回歸驗證，對應 adjuster_postgres_test.go 的
// TestPostgresMigrationsRealValuesFitAllColumns。
//
// **為什麼 mysql 也要一份**：063／064／065 是同一個 session 內連續三次
// 「欄位寬度裝不下程式碼常數」事故，三支 migration 的 mysql 版與 postgres 版是同樣的欄寬限制，
// 但事後補的回歸測試只有 postgres 有。少了這一份，mysql 的下一次欄寬事故只能等人工發現——
// 而 dev／live 都跑 postgres，mysql 沒有任何日常執行路徑會撞到它。
//
// 放在 package database_test（外部測試套件）的理由同 postgres 版：要 import store 與 handler，
// 而 market 會 import database，同套件內會形成循環。
//
// 以 MYSQL_MIGRATION_DSN gate 住，用 scripts/test-mysql-migrations.sh 執行。
// **測試名稱必須以 TestMySQLMigrations 開頭**，否則不符合該腳本的 -test.run 過濾條件
// （postgres 側 2026-08-11 踩過一次：測試加了卻沒被執行，輸出看起來完全正常）。

// TestMySQLMigrationsRealValuesFitAllColumns 拿**程式碼裡真正的常數清單**寫入，而不是寫死字串，
// 所以日後新增更長的值會直接在這裡失敗。
//
// 逐欄驗證（只換 action_type、其餘欄位塞安全的短字串）抓不到下一個欄位——065 就是這樣漏掉的。
// 因此這裡走真實的 repo Upsert，讓每一組 (action_type, source) 的所有欄位同時吃到正式值。
func TestMySQLMigrationsRealValuesFitAllColumns(t *testing.T) {
	dsn := os.Getenv("MYSQL_MIGRATION_DSN")
	if dsn == "" {
		t.Skip("未設 MYSQL_MIGRATION_DSN，跳過（用 scripts/test-mysql-migrations.sh 執行）")
	}
	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		t.Fatalf("連不上 MySQL: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()

	if err := database.RunMigrations(ctx, db, "mysql", zap.NewNop()); err != nil {
		t.Fatalf("migration 失敗: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM corporate_actions`)
		_, _ = db.Exec(`DELETE FROM job_runs`)
		_, _ = db.Exec(`DELETE FROM evaluation_universe`)
		_, _ = db.Exec(`DELETE FROM indicator_snapshots`)
		_, _ = db.Exec(`DELETE FROM signals`)
	})

	// 刻意用 UTC 而不是 TaipeiTZ：go-sql-driver 寫入前會把 time.Time 轉成連線的 loc
	// （DSN 沒帶 loc 即 UTC），所以台北午夜存進 DATE 會變成前一天——pgx 則是直接取
	// 值本身時區的日曆日。這裡的日期只是佔位、不參與斷言，用 UTC 才不會讓人以為
	// 存進去的就是 2020-01-01。該時區不對稱本身見 docs/issue.md I-054。
	eventDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	// corporate_actions：走真實的 repo（driver 是 mysql，走 ON DUPLICATE KEY UPDATE 那一支），
	// 每一組 (type, source) 都試。
	repo := store.NewCorporateActionRepo(db)
	i := 0
	for _, at := range store.AllCorporateActionTypes() {
		for _, src := range store.AllCorporateActionSources() {
			i++
			err := repo.Upsert(ctx, []store.CorporateAction{{
				Symbol: "T" + strconv.Itoa(i), EventDate: eventDate,
				ActionType: at, BeforePrice: 100, AfterPrice: 50,
				Factor: 0.5, VolumeFactor: 0.5, Source: src,
			}})
			if err != nil {
				t.Errorf("action_type=%q(%d 字元) source=%q(%d 字元) 寫不進 corporate_actions: %v",
					at, len([]rune(at)), src, len(src), err)
			}
		}
	}

	// job_runs：同樣拿程式碼裡真正註冊的名稱清單。
	for _, name := range handler.KnownSchedulerJobs() {
		if _, err := db.Exec(
			`INSERT INTO job_runs (job_name, status, started_at) VALUES (?, 'running', NOW())`,
			name); err != nil {
			t.Errorf("job_name %q（%d 字元）寫不進 job_runs: %v", name, len(name), err)
		}
	}

	// evaluation_universe：與 postgres 版對稱。這是**目前唯一**會對真實 MySQL 執行
	// 本 repo 寫入路徑的地方（issue.md I-054 第 1 項：其餘 CRUD 仍只跑 sqlite）。
	uniRepo := store.NewEvaluationUniverseRepo(db)
	for j, role := range store.AllUniverseRoles() {
		entry := store.EvaluationUniverseEntry{
			Symbol:          "U" + strconv.Itoa(j),
			BucketHint:      "NORMAL_VOLATILITY",
			BucketEdgeLow:   0.046089927430152715,
			BucketEdgeHigh:  0.06278197721225691,
			UniverseVersion: "v2",
			UniverseRole:    role,
			SelectedAt:      time.Now(),
			Source:          "T-040_STEP3",
		}
		if err := uniRepo.Upsert(ctx, []store.EvaluationUniverseEntry{entry}); err != nil {
			t.Errorf("universe_role=%q(%d 字元) 寫不進 evaluation_universe: %v",
				role, len(role), err)
		}
	}

	// indicator_snapshots / signals 的數值邊界。
	// 欄位型別與選型依據見 docs/database-schema.md。
	// 設計理由見 postgres 版的同名測試；這裡只留 mysql 的差異。
	//
	// ⚠️ 這兩條也讓 I-054「mysql 只有兩條真實 repo 寫入路徑被執行過」的清單
	// 變成四條——改動這裡時要同步更新那一筆，否則會低估 mysql 的覆蓋。
	indRepo := store.NewIndicatorRepo(db)
	sigRepo := store.NewSignalRepo(db)
	for i, volRatio := range []float64{4204257454, float64(math.MaxInt64)} {
		snap := &store.IndicatorSnapshot{
			Symbol:    "B" + strconv.Itoa(i),
			Timeframe: "1m",
			Timestamp: time.Now(),
			RSI14:     100,
			VolRatio:  volRatio,
		}
		if err := indRepo.Upsert(ctx, snap); err != nil {
			t.Errorf("rsi14=100 vol_ratio=%v 寫不進 indicator_snapshots: %v", volRatio, err)
			continue
		}
		got, err := indRepo.GetLatest(ctx, snap.Symbol, "1m")
		if err != nil {
			t.Errorf("讀不回 indicator_snapshots(%s): %v", snap.Symbol, err)
		} else if got.RSI14 != 100 || got.VolRatio != volRatio {
			t.Errorf("indicator_snapshots round-trip 失真：rsi14 %v→%v、vol_ratio %v→%v",
				100.0, got.RSI14, volRatio, got.VolRatio)
		}

		sig := &store.Signal{
			Symbol: "B" + strconv.Itoa(i), SignalType: "VOLUME_SPIKE", Direction: "BUY",
			Price: 100, Volume: 1, VolRatio: volRatio, Trend: "Bullish",
			Note: "numeric boundary", Timestamp: time.Now(), Strength: 1.0,
		}
		if err := sigRepo.Insert(ctx, sig); err != nil {
			t.Errorf("vol_ratio=%v 寫不進 signals: %v", volRatio, err)
		}
	}

	// mysql 的 information_schema 欄位名與 postgres 相同，但 table_schema 要限定成
	// 當前 database，否則同一台 server 上的其他 schema 會一起被撈到。
	for _, want := range []struct {
		table, column    string
		precision, scale int
	}{
		{"indicator_snapshots", "rsi14", 7, 4},
		{"indicator_snapshots", "vol_ratio", 23, 4},
		{"signals", "vol_ratio", 23, 4},
	} {
		var gotPrecision, gotScale int
		if err := db.QueryRow(
			`SELECT numeric_precision, numeric_scale FROM information_schema.columns
			 WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`,
			want.table, want.column).Scan(&gotPrecision, &gotScale); err != nil {
			t.Errorf("查不到 %s.%s 的精度: %v", want.table, want.column, err)
			continue
		}
		if gotPrecision != want.precision || gotScale != want.scale {
			t.Errorf("%s.%s 精度應為 (%d,%d)，實際 (%d,%d)",
				want.table, want.column, want.precision, want.scale, gotPrecision, gotScale)
		}
	}
}
