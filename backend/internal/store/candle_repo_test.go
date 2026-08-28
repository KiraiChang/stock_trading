package store

import (
	"context"
	"os"
	"reflect"
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

// ── I-091：缺漏偵測要的「實際日期集合」 ──────────────────────────────────────

// 本體：回傳的是**台北日期**，而且中段的洞看得出來。
//
// 只看最新日期或本輪筆數都抓不到視窗中段的缺漏，而那正是 T-062 的跳過最佳化之後的
// 主要盲點——某檔今天有 K 棒就會被整檔跳過，五天前那個洞永遠不會被重新抓取。
func TestCandleDatesInRangeReturnsTaipeiDatesAndExposesMiddleGap(t *testing.T) {
	repo, ctx := newCandleRepoForTest(t)
	from := time.Date(2026, 8, 24, 0, 0, 0, 0, timeutil.TaipeiTZ)
	to := from.AddDate(0, 0, 5)

	// 8/24、8/26、8/27 有，8/25 是中段的洞。
	seedCandle(t, repo, "2330", "1d", from.Add(13*time.Hour+30*time.Minute))
	seedCandle(t, repo, "2330", "1d", from.AddDate(0, 0, 2).Add(13*time.Hour+30*time.Minute))
	// **台北 00:30**：用 UTC 日界會被算成前一天，日期就會標錯。
	seedCandle(t, repo, "2330", "1d", from.AddDate(0, 0, 3).Add(30*time.Minute))

	got, err := repo.CandleDatesInRange(ctx, []string{"2330"}, "1d", from, to)
	if err != nil {
		t.Fatalf("CandleDatesInRange failed: %v", err)
	}
	want := []string{"2026-08-24", "2026-08-26", "2026-08-27"}
	if !reflect.DeepEqual(got["2330"], want) {
		t.Errorf("回傳 %v, 期望 %v（升冪、台北日期、8/25 應缺）", got["2330"], want)
	}
}

// 區間是半開的 `[from, to)`，兩端都要驗——多算或少算一天都會直接造成誤報或漏報。
func TestCandleDatesInRangeUsesHalfOpenInterval(t *testing.T) {
	repo, ctx := newCandleRepoForTest(t)
	from := time.Date(2026, 8, 24, 0, 0, 0, 0, timeutil.TaipeiTZ)
	to := from.AddDate(0, 0, 2)

	seedCandle(t, repo, "2330", "1d", from.Add(-time.Second)) // 區間前一刻，排除
	seedCandle(t, repo, "2330", "1d", from)                   // 起點，含
	seedCandle(t, repo, "2330", "1d", to.Add(-time.Second))   // 終點前一刻，含
	seedCandle(t, repo, "2330", "1d", to)                     // 終點，排除

	got, err := repo.CandleDatesInRange(ctx, []string{"2330"}, "1d", from, to)
	if err != nil {
		t.Fatalf("CandleDatesInRange failed: %v", err)
	}
	want := []string{"2026-08-24", "2026-08-25"}
	if !reflect.DeepEqual(got["2330"], want) {
		t.Errorf("回傳 %v, 期望 %v（半開區間 [from, to)）", got["2330"], want)
	}
}

// 多檔一次查回，且**完全沒有 K 棒的標的不會有 key**。
//
// 呼叫端據此區分「這檔整段都缺」與「這檔有幾天缺」，兩者的處置不同。
// timeframe 也要有效：同一檔的 1m K 棒不該混進日 K 的日期集合。
func TestCandleDatesInRangeGroupsBySymbolAndFiltersTimeframe(t *testing.T) {
	repo, ctx := newCandleRepoForTest(t)
	from := time.Date(2026, 8, 24, 0, 0, 0, 0, timeutil.TaipeiTZ)
	to := from.AddDate(0, 0, 3)

	seedCandle(t, repo, "2330", "1d", from.Add(13*time.Hour))
	seedCandle(t, repo, "2454", "1d", from.AddDate(0, 0, 1).Add(13*time.Hour))
	// 1m 的 K 棒不該出現在 1d 的日期集合裡。
	seedCandle(t, repo, "6182", "1m", from.Add(13*time.Hour))
	// 沒被問到的標的不該出現。
	seedCandle(t, repo, "9999", "1d", from.Add(13*time.Hour))

	got, err := repo.CandleDatesInRange(ctx, []string{"2330", "2454", "6182"}, "1d", from, to)
	if err != nil {
		t.Fatalf("CandleDatesInRange failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("應只有 2 檔有日期，得到 %#v", got)
	}
	if !reflect.DeepEqual(got["2330"], []string{"2026-08-24"}) {
		t.Errorf("2330 = %v", got["2330"])
	}
	if !reflect.DeepEqual(got["2454"], []string{"2026-08-25"}) {
		t.Errorf("2454 = %v", got["2454"])
	}
	// **整段都缺的標的沒有 key**，不是空 slice——呼叫端要看得出差別。
	if _, ok := got["6182"]; ok {
		t.Error("只有 1m K 棒的標的不該出現在 1d 的結果裡")
	}
	if _, ok := got["9999"]; ok {
		t.Error("沒被問到的標的不該出現")
	}
}

// 空清單回空 map 而不是 nil，也不送查詢——呼叫端會直接對它取值。
func TestCandleDatesInRangeWithNoSymbols(t *testing.T) {
	repo, ctx := newCandleRepoForTest(t)
	day := time.Date(2026, 8, 25, 0, 0, 0, 0, timeutil.TaipeiTZ)

	got, err := repo.CandleDatesInRange(ctx, nil, "1d", day, day.AddDate(0, 0, 1))
	if err != nil {
		t.Fatalf("CandleDatesInRange failed: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("空清單應回空 map，得到 %#v", got)
	}
}
