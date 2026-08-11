package database_test

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/trading/backend/internal/api/handler"
	"github.com/trading/backend/internal/database"
	"github.com/trading/backend/internal/market"
	"github.com/trading/backend/internal/store"
	"github.com/trading/backend/pkg/timeutil"
)

// 還原係數的 **postgres** 路徑驗證（T-042）。
//
// **為什麼需要它**：`ApplyAdjFactors` 的既有測試全走 sqlite，但 live 與 dev 跑的是 postgres，
// 而這段 SQL 有三個只有在 postgres 上才會顯現的地方：
//
//   - 下界用 `time.Time{}`（西元 1 年）當「涵蓋全部歷史」，pgx 對零值時間的處理與 sqlite 不同。
//   - `adj_factor` / `vol_factor` 是 `DECIMAL(18,10)`，sqlite 是 `REAL`——捨入行為不一樣。
//   - 歸零與覆寫在同一個交易內完成。
//
// 放在 package database_test（外部測試套件）是因為要 import market 與 store，
// 而 market 會 import database——同套件內會形成循環。
//
// 以 POSTGRES_MIGRATION_DSN gate 住，用 scripts/test-postgres-migrations.sh 執行。
// 測試名稱必須以 TestPostgresMigrations 開頭，否則不符合該腳本的 -test.run 過濾條件
// （2026-08-11 踩過一次：測試加了卻沒被執行，輸出看起來完全正常）。

type stubSplits struct{ actions []store.CorporateAction }

func (s *stubSplits) FetchSplitPrices(context.Context, time.Time, time.Time) ([]store.CorporateAction, error) {
	return s.actions, nil
}

func pgDay(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, timeutil.TaipeiTZ)
}

func TestPostgresMigrationsAdjusterAppliesFactors(t *testing.T) {
	dsn := os.Getenv("POSTGRES_MIGRATION_DSN")
	if dsn == "" {
		t.Skip("未設 POSTGRES_MIGRATION_DSN，跳過（用 scripts/test-postgres-migrations.sh 執行）")
	}
	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Fatalf("連不上 postgres: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()

	if err := database.RunMigrations(ctx, db, "postgres", zap.NewNop()); err != nil {
		t.Fatalf("migration 失敗: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM candles`)
		_, _ = db.Exec(`DELETE FROM corporate_actions`)
	})

	candles := store.NewCandleRepo(db)
	actions := store.NewCorporateActionRepo(db)

	// 真實數字：0050 在 2025-06-18 的 1:4 分割，前一交易日收 188.65。
	seed := []store.Candle{
		{Symbol: "0050", Timeframe: "1d", Open: 183.4, High: 184.05, Low: 183.05, Close: 183.7,
			Volume: 14115012, Amount: 1, Timestamp: pgDay(2025, 6, 9)},
		{Symbol: "0050", Timeframe: "1d", Open: 184.9, High: 188.9, Low: 184.9, Close: 188.65,
			Volume: 31483080, Amount: 1, Timestamp: pgDay(2025, 6, 10)},
		{Symbol: "0050", Timeframe: "1d", Open: 47.5, High: 47.72, Low: 47.14, Close: 47.57,
			Volume: 252639825, Amount: 1, Timestamp: pgDay(2025, 6, 18)},
	}
	if err := candles.BulkInsert(ctx, seed); err != nil {
		t.Fatalf("seed 失敗: %v", err)
	}

	src := &stubSplits{actions: []store.CorporateAction{{
		Symbol: "0050", EventDate: pgDay(2025, 6, 18), ActionType: "分割",
		BeforePrice: 188.65, AfterPrice: 47.16,
		Factor: 47.16 / 188.65, VolumeFactor: 47.16 / 188.65, Source: "test",
	}}}
	adj := market.NewAdjuster(src, actions, candles, zap.NewNop())

	// 跑三次，驗冪等性在 postgres 上同樣成立（DECIMAL 的捨入不會逐輪漂移）。
	var snapshots []map[string][2]float64
	for i := 0; i < 3; i++ {
		if _, err := adj.SyncSplits(ctx, pgDay(2015, 1, 1), pgDay(2026, 8, 11)); err != nil {
			t.Fatalf("第 %d 次 SyncSplits 失敗: %v", i+1, err)
		}
		rows, err := candles.GetRange(ctx, "0050", "1d", pgDay(2000, 1, 1), pgDay(2030, 1, 1))
		if err != nil {
			t.Fatalf("讀取失敗: %v", err)
		}
		snap := make(map[string][2]float64, len(rows))
		for _, r := range rows {
			snap[r.Timestamp.In(timeutil.TaipeiTZ).Format("2006-01-02")] =
				[2]float64{r.AdjFactor, r.VolFactor}
		}
		snapshots = append(snapshots, snap)
	}

	for i := 1; i < len(snapshots); i++ {
		for day, want := range snapshots[0] {
			if got := snapshots[i][day]; got != want {
				t.Errorf("第 %d 次重算後 %s 的係數 = %v, 第一次為 %v——postgres 上不是冪等的",
					i+1, day, got, want)
			}
		}
	}

	final := snapshots[len(snapshots)-1]
	// 零值時間下界必須真的涵蓋到最早的 K 棒——pgx 對 time.Time{} 的處理是這裡的風險點。
	for _, day := range []string{"2025-06-09", "2025-06-10"} {
		f := final[day]
		if diff := f[0] - 0.24998675; diff > 1e-6 || diff < -1e-6 {
			t.Errorf("%s 的 adj_factor = %v, want ≈0.2499868（零值時間下界沒涵蓋到這根？）", day, f[0])
		}
		if f[1] != f[0] {
			t.Errorf("%s 的 vol_factor = %v, 應與 adj_factor 相同（純股數事件）", day, f[1])
		}
	}
	if f := final["2025-06-18"]; f[0] != 1 || f[1] != 1 {
		t.Errorf("事件當日的係數 = %v, want [1 1]——as-of 邊界在 postgres 上算錯了", f)
	}

	// 還原價要對得上官方的分割後參考價。
	rows, err := candles.GetRange(ctx, "0050", "1d", pgDay(2025, 6, 10), pgDay(2025, 6, 10))
	if err != nil || len(rows) != 1 {
		t.Fatalf("讀 2025-06-10 失敗: %v (%d 列)", err, len(rows))
	}
	if got := rows[0].AdjustedClose(); got < 47.15 || got > 47.17 {
		t.Errorf("2025-06-10 還原價 = %v, want ≈47.16", got)
	}
	if got := rows[0].AdjustedVolume(); got < 125_900_000 || got > 126_000_000 {
		t.Errorf("2025-06-10 還原量 = %v, want ≈1.259 億（31,483,080 × 4）", got)
	}
}

// TestPostgresMigrationsRealValuesFitAllColumns 把「欄位寬度裝不下程式碼常數」這件事
// 一次擋掉——**同一個 session 內已經撞上三次**：
//
//	job_runs.job_name             VARCHAR(20) ← corporate_action_sync（21）
//	corporate_actions.action_type VARCHAR(16) ← CAPITAL_REDUCTION（17）
//	corporate_actions.source      VARCHAR(32) ← TaiwanStock…ReferencePrice（41）
//
// **前一版的測試沒抓到第三次**，因為它逐欄驗證：只把 action_type 換成各種常數，
// 其餘欄位（包含 source）寫死成安全的短字串。逐欄測試抓不到下一個欄位。
//
// 所以這一版改成：用**真實的 repo Upsert**寫入每一組實際會出現的
// (action_type, source) 組合，讓所有欄位同時吃到正式值。
func TestPostgresMigrationsRealValuesFitAllColumns(t *testing.T) {
	dsn := os.Getenv("POSTGRES_MIGRATION_DSN")
	if dsn == "" {
		t.Skip("未設 POSTGRES_MIGRATION_DSN，跳過")
	}
	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Fatalf("連不上 postgres: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()

	if err := database.RunMigrations(ctx, db, "postgres", zap.NewNop()); err != nil {
		t.Fatalf("migration 失敗: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM corporate_actions`)
		_, _ = db.Exec(`DELETE FROM job_runs`)
	})

	// corporate_actions：走真實的 repo，每一組 (type, source) 都試。
	repo := store.NewCorporateActionRepo(db)
	i := 0
	for _, at := range store.AllCorporateActionTypes() {
		for _, src := range store.AllCorporateActionSources() {
			i++
			err := repo.Upsert(ctx, []store.CorporateAction{{
				Symbol: "T" + strconv.Itoa(i), EventDate: pgDay(2020, 1, 1),
				ActionType: at, BeforePrice: 100, AfterPrice: 50,
				Factor: 0.5, VolumeFactor: 0.5, Source: src,
			}})
			if err != nil {
				t.Errorf("action_type=%q(%d 字元) source=%q(%d 字元) 寫不進 corporate_actions: %v",
					at, len(at), src, len(src), err)
			}
		}
	}

	// job_runs：同樣拿程式碼裡真正註冊的名稱清單。
	for _, name := range handler.KnownSchedulerJobs() {
		if _, err := db.Exec(
			`INSERT INTO job_runs (job_name, status, started_at) VALUES ($1, 'running', NOW())`,
			name); err != nil {
			t.Errorf("job_name %q（%d 字元）寫不進 job_runs: %v", name, len(name), err)
		}
	}
}
