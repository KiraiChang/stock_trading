package store

import (
	"context"
	"os"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/database"
	"github.com/trading/backend/pkg/timeutil"
)

func TestCandleBulkInsertBatchesAndUpserts(t *testing.T) {
	tmp, err := os.CreateTemp("", "candle-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	defer os.Remove(tmp.Name())

	db, err := NewDB(config.DatabaseConfig{Driver: "sqlite", DSN: tmp.Name()})
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer db.Close()
	if err := database.RunMigrations(context.Background(), db, "sqlite", zap.NewNop()); err != nil {
		t.Fatalf("migrations failed: %v", err)
	}

	repo := NewCandleRepo(db)
	ctx := context.Background()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := make([]Candle, 0, candleBulkInsertBatchSize+25)
	for i := 0; i < candleBulkInsertBatchSize+25; i++ {
		candles = append(candles, Candle{
			Symbol:    "2330",
			Timeframe: "1d",
			Open:      float64(100 + i),
			High:      float64(101 + i),
			Low:       float64(99 + i),
			Close:     float64(100 + i),
			Volume:    int64(1000 + i),
			Amount:    float64(100000 + i),
			Timestamp: base.AddDate(0, 0, i),
		})
	}

	if err := repo.BulkInsert(ctx, candles); err != nil {
		t.Fatalf("bulk insert failed: %v", err)
	}

	update := candles[10]
	update.Open = 900
	update.High = 910
	update.Low = 890
	update.Close = 905
	update.Volume = 9999
	update.Amount = 8888
	if err := repo.BulkInsert(ctx, []Candle{update}); err != nil {
		t.Fatalf("bulk upsert failed: %v", err)
	}

	rows, err := repo.GetRange(ctx, "2330", "1d", base, base.AddDate(0, 0, len(candles)))
	if err != nil {
		t.Fatalf("get range failed: %v", err)
	}
	if len(rows) != len(candles) {
		t.Fatalf("expected %d candles, got %d", len(candles), len(rows))
	}
	got := rows[10]
	if got.Open != update.Open || got.Close != update.Close || got.Volume != update.Volume || got.Amount != update.Amount {
		t.Fatalf("expected row 10 to be updated, got open=%v close=%v volume=%v amount=%v", got.Open, got.Close, got.Volume, got.Amount)
	}
}

// newCandleRepoForTest 與 job_run_repo_test 同一個限制：**只跑 sqlite**
// （mysql 的 CRUD 未驗，見 issue.md I-054）。
func newCandleRepoForTest(t *testing.T) (CandleRepo, context.Context) {
	t.Helper()
	tmp, err := os.CreateTemp("", "candle-day-test-*.db")
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
	return NewCandleRepo(db), context.Background()
}

func seedCandle(t *testing.T, repo CandleRepo, symbol, timeframe string, ts time.Time) {
	t.Helper()
	if err := repo.BulkInsert(context.Background(), []Candle{{
		Symbol: symbol, Timeframe: timeframe,
		Open: 10, High: 11, Low: 9, Close: 10,
		Volume: 100, Amount: 1000, Timestamp: ts,
	}}); err != nil {
		t.Fatalf("seed failed: %v", err)
	}
}

// **這條的重點是台北時區的日界**（見 docs/architecture.md 的日 K 維護段）。
// `ts` 是 timestamptz，若拿 UTC 日界去比，台灣時間當天 08:00 以前的 K 棒會被算成前一天，
// 於是排程要嘛每天跳過全部、要嘛每天都不跳過——前者會靜默漏抓。
func TestSymbolsWithCandleOnUsesTaipeiDayBoundary(t *testing.T) {
	repo, ctx := newCandleRepoForTest(t)
	day := time.Date(2026, 8, 25, 0, 0, 0, 0, timeutil.TaipeiTZ)

	// 台北 00:30——落在當天，但用 UTC 日界會被算成前一天（UTC 前一日 16:30）。
	seedCandle(t, repo, "1101", "1d", day.Add(30*time.Minute))
	// 台北 23:30——同樣在當天，用 UTC 日界會被算成當天（UTC 15:30），不具鑑別度但要在。
	seedCandle(t, repo, "1102", "1d", day.Add(23*time.Hour+30*time.Minute))
	// 前一天的最後一刻與隔天的第一刻，兩邊都必須被排除。
	seedCandle(t, repo, "1103", "1d", day.Add(-time.Second))
	seedCandle(t, repo, "1104", "1d", day.AddDate(0, 0, 1))

	got, err := repo.SymbolsWithCandleOn(ctx, []string{"1101", "1102", "1103", "1104"}, "1d", day)
	if err != nil {
		t.Fatalf("SymbolsWithCandleOn failed: %v", err)
	}
	want := []string{"1101", "1102"}
	if len(got) != len(want) {
		t.Fatalf("回傳 %v, 期望 %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("回傳 %v, 期望 %v（且需升冪）", got, want)
		}
	}
}

// **一定要帶 timeframe**：同一檔的 1m K 棒不該讓日 K 的排程誤判「今天抓過了」。
func TestSymbolsWithCandleOnFiltersByTimeframe(t *testing.T) {
	repo, ctx := newCandleRepoForTest(t)
	day := time.Date(2026, 8, 25, 0, 0, 0, 0, timeutil.TaipeiTZ)

	seedCandle(t, repo, "2330", "1m", day.Add(9*time.Hour))

	got, err := repo.SymbolsWithCandleOn(ctx, []string{"2330"}, "1d", day)
	if err != nil {
		t.Fatalf("SymbolsWithCandleOn failed: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("只有 1m K 棒時不該回傳任何標的，得到 %v", got)
	}
}

// 沒有任何當日 K 棒時回空集合而不是錯誤——排程靠這個狀態決定「整池都要跑」。
func TestSymbolsWithCandleOnReturnsEmptyWhenNothingSynced(t *testing.T) {
	repo, ctx := newCandleRepoForTest(t)
	day := time.Date(2026, 8, 25, 0, 0, 0, 0, timeutil.TaipeiTZ)

	got, err := repo.SymbolsWithCandleOn(ctx, []string{"1101"}, "1d", day)
	if err != nil {
		t.Fatalf("空表不該回錯誤，得到 %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("空表應回 0 筆，得到 %v", got)
	}
}

// **查詢一定要限制在傳入的 symbols 內**（2026-08-25 review）：不約束 symbol 的話
// 複合索引 (symbol, timeframe, ts) 的首欄用不上，PostgreSQL 16 沒有 skip scan，
// 會退化成整張 candles 的 seq scan（live 實測 368ms vs 1.96ms）。
// 這條同時驗語意：池外的標的即使今天有 K 棒也不該出現在結果裡。
func TestSymbolsWithCandleOnOnlyConsidersRequestedSymbols(t *testing.T) {
	repo, ctx := newCandleRepoForTest(t)
	day := time.Date(2026, 8, 25, 0, 0, 0, 0, timeutil.TaipeiTZ)

	seedCandle(t, repo, "1101", "1d", day.Add(9*time.Hour))
	seedCandle(t, repo, "9999", "1d", day.Add(9*time.Hour)) // 不在詢問清單內

	got, err := repo.SymbolsWithCandleOn(ctx, []string{"1101"}, "1d", day)
	if err != nil {
		t.Fatalf("SymbolsWithCandleOn failed: %v", err)
	}
	if len(got) != 1 || got[0] != "1101" {
		t.Fatalf("回傳 %v, 期望只有 1101（9999 不在詢問清單內）", got)
	}
}

// symbols 為空時不送查詢、回空集合。呼叫端拿空池時會走到這條。
func TestSymbolsWithCandleOnWithNoSymbols(t *testing.T) {
	repo, ctx := newCandleRepoForTest(t)
	day := time.Date(2026, 8, 25, 0, 0, 0, 0, timeutil.TaipeiTZ)

	seedCandle(t, repo, "1101", "1d", day.Add(9*time.Hour))

	got, err := repo.SymbolsWithCandleOn(ctx, nil, "1d", day)
	if err != nil {
		t.Fatalf("空清單不該回錯誤，得到 %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("空清單應回 0 筆，得到 %v", got)
	}
}
