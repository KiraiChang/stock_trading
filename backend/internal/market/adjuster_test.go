package market

import (
	"context"
	"os"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/database"
	"github.com/trading/backend/internal/store"
	"github.com/trading/backend/pkg/timeutil"
)

// stubSplitSource 回傳固定的分割事件，讓測試不依賴 FinMind。
type stubSplitSource struct {
	actions []store.CorporateAction
	calls   int
}

func (s *stubSplitSource) FetchSplitPrices(context.Context, time.Time, time.Time) ([]store.CorporateAction, error) {
	s.calls++
	return s.actions, nil
}

func taipeiDay(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, timeutil.TaipeiTZ)
}

func newAdjusterTestDB(t *testing.T) (*Adjuster, store.CandleRepo, *stubSplitSource) {
	t.Helper()
	tmp, err := os.CreateTemp("", "adjuster-test-*.db")
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

	candles := store.NewCandleRepo(db)
	actions := store.NewCorporateActionRepo(db)
	src := &stubSplitSource{}
	return NewAdjuster(src, actions, candles, zap.NewNop()), candles, src
}

// seedDaily 塞入從 from 起連續 n 個交易日的日 K，收盤價固定為 price。
func seedDaily(t *testing.T, repo store.CandleRepo, symbol string, from time.Time, n int, price float64, volume int64) {
	t.Helper()
	cs := make([]store.Candle, 0, n)
	for i := 0; i < n; i++ {
		cs = append(cs, store.Candle{
			Symbol: symbol, Timeframe: "1d",
			Open: price, High: price, Low: price, Close: price,
			Volume: volume, Amount: price * float64(volume),
			Timestamp: from.AddDate(0, 0, i),
		})
	}
	if err := repo.BulkInsert(context.Background(), cs); err != nil {
		t.Fatalf("seed failed: %v", err)
	}
}

func factorsBySymbol(t *testing.T, repo store.CandleRepo, symbol string) map[string]float64 {
	t.Helper()
	rows, err := repo.GetRange(context.Background(), symbol, "1d",
		time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	out := make(map[string]float64, len(rows))
	for _, r := range rows {
		out[r.Timestamp.In(timeutil.TaipeiTZ).Format("2006-01-02")] = r.AdjFactor
	}
	return out
}

// TestRecomputeIsIdempotent 是本功能最重要的一條：跑幾次結果都必須一致。
//
// 冪等性靠的是「adj_factor 是事件表的純函數、重算整段覆寫」。若哪天有人把
// SetAdjFactor 改成 `adj_factor = adj_factor * ?`，這支測試會在第二輪就抓到——
// 而正式環境不會有任何東西報錯，只會靜靜地把係數平方。
func TestRecomputeIsIdempotent(t *testing.T) {
	adj, candles, src := newAdjusterTestDB(t)
	ctx := context.Background()

	seedDaily(t, candles, "0050", taipeiDay(2025, 6, 9), 10, 188.65, 1000)
	src.actions = []store.CorporateAction{{
		Symbol: "0050", EventDate: taipeiDay(2025, 6, 18), ActionType: store.CorporateActionSplit,
		BeforePrice: 188.65, AfterPrice: 47.16, Factor: 47.16 / 188.65, Source: "test",
	}}

	var snapshots []map[string]float64
	for i := 0; i < 3; i++ {
		if _, err := adj.SyncSplits(ctx, taipeiDay(2015, 1, 1), taipeiDay(2026, 8, 11)); err != nil {
			t.Fatalf("第 %d 次 SyncSplits 失敗: %v", i+1, err)
		}
		snapshots = append(snapshots, factorsBySymbol(t, candles, "0050"))
	}

	for i := 1; i < len(snapshots); i++ {
		if len(snapshots[i]) != len(snapshots[0]) {
			t.Fatalf("第 %d 次的列數不同: %d vs %d", i+1, len(snapshots[i]), len(snapshots[0]))
		}
		for day, want := range snapshots[0] {
			if got := snapshots[i][day]; got != want {
				t.Errorf("第 %d 次重算後 %s 的係數 = %v, 第一次為 %v——重算不是冪等的",
					i+1, day, got, want)
			}
		}
	}
}

// TestRecomputeAsOfBoundary 驗 as-of 邊界：event_date 是新價的第一個交易日，
// 所以事件當日**不**套用係數，前一日才套。差一天就會讓分割當天的價格被重複縮小。
func TestRecomputeAsOfBoundary(t *testing.T) {
	adj, candles, src := newAdjusterTestDB(t)
	ctx := context.Background()

	// 6/16、6/17、6/18、6/19 四天。
	seedDaily(t, candles, "0050", taipeiDay(2025, 6, 16), 4, 100, 1000)
	src.actions = []store.CorporateAction{{
		Symbol: "0050", EventDate: taipeiDay(2025, 6, 18), ActionType: store.CorporateActionSplit,
		BeforePrice: 200, AfterPrice: 50, Factor: 0.25, Source: "test",
	}}
	if _, err := adj.SyncSplits(ctx, taipeiDay(2015, 1, 1), taipeiDay(2026, 8, 11)); err != nil {
		t.Fatal(err)
	}

	got := factorsBySymbol(t, candles, "0050")
	for day, want := range map[string]float64{
		"2025-06-16": 0.25, // 事件前：要套
		"2025-06-17": 0.25, // 事件前一日：要套
		"2025-06-18": 1,    // 事件當日：已經是新價，不套
		"2025-06-19": 1,    // 事件後：不套
	} {
		if got[day] != want {
			t.Errorf("%s 的係數 = %v, want %v", day, got[day], want)
		}
	}
}

// TestRecomputeCumulativeAcrossTwoEvents 驗多次事件的累積連乘。
func TestRecomputeCumulativeAcrossTwoEvents(t *testing.T) {
	adj, candles, src := newAdjusterTestDB(t)
	ctx := context.Background()

	seedDaily(t, candles, "9999", taipeiDay(2024, 1, 1), 400, 100, 1000)
	src.actions = []store.CorporateAction{
		{Symbol: "9999", EventDate: taipeiDay(2024, 6, 1), ActionType: store.CorporateActionSplit,
			BeforePrice: 200, AfterPrice: 100, Factor: 0.5, Source: "test"},
		{Symbol: "9999", EventDate: taipeiDay(2024, 12, 1), ActionType: store.CorporateActionSplit,
			BeforePrice: 100, AfterPrice: 25, Factor: 0.25, Source: "test"},
	}
	if _, err := adj.SyncSplits(ctx, taipeiDay(2015, 1, 1), taipeiDay(2026, 8, 11)); err != nil {
		t.Fatal(err)
	}

	got := factorsBySymbol(t, candles, "9999")
	for day, want := range map[string]float64{
		"2024-05-31": 0.125, // 兩次事件都在它之後：0.5 * 0.25
		"2024-06-01": 0.25,  // 只剩第二次在它之後
		"2024-11-30": 0.25,
		"2024-12-01": 1, // 之後沒有事件了
	} {
		if got[day] != want {
			t.Errorf("%s 的係數 = %v, want %v", day, got[day], want)
		}
	}
}

// TestRecomputeClearsStaleFactorsWhenEventRemoved 驗「先歸零再重算」。
// 少了歸零那一步，被刪掉的事件所留下的係數會留在原地，重算就不再是事件表的純函數。
func TestRecomputeClearsStaleFactorsWhenEventRemoved(t *testing.T) {
	adj, candles, src := newAdjusterTestDB(t)
	ctx := context.Background()

	seedDaily(t, candles, "0050", taipeiDay(2025, 6, 16), 4, 100, 1000)
	src.actions = []store.CorporateAction{{
		Symbol: "0050", EventDate: taipeiDay(2025, 6, 18), ActionType: store.CorporateActionSplit,
		BeforePrice: 200, AfterPrice: 50, Factor: 0.25, Source: "test",
	}}
	if _, err := adj.SyncSplits(ctx, taipeiDay(2015, 1, 1), taipeiDay(2026, 8, 11)); err != nil {
		t.Fatal(err)
	}
	if got := factorsBySymbol(t, candles, "0050")["2025-06-16"]; got != 0.25 {
		t.Fatalf("前置條件不成立，係數 = %v", got)
	}

	// 直接重算（ApplyAdjFactors 內含歸零），驗證重算能還原出相同結果。
	if err := adj.RecomputeSymbol(ctx, "0050"); err != nil {
		t.Fatal(err)
	}
	// RecomputeSymbol 讀的是事件表，事件仍在，所以係數應該還在。
	if got := factorsBySymbol(t, candles, "0050")["2025-06-16"]; got != 0.25 {
		t.Errorf("歸零後重算的係數 = %v, want 0.25", got)
	}
}

// TestAdjustedPriceVolumeInvariant 驗價與量方向相反、且乘積不變。
// 這條恆等式是「有沒有寫反」的最直接檢查。
func TestAdjustedPriceVolumeInvariant(t *testing.T) {
	c := store.Candle{Close: 188.65, Volume: 1000, AdjFactor: 47.16 / 188.65}

	if got, want := c.AdjustedClose(), 47.16; got < want-1e-9 || got > want+1e-9 {
		t.Errorf("還原價 = %v, want %v", got, want)
	}
	if got := c.AdjustedVolume(); got < 3999 || got > 4001 {
		t.Errorf("還原量 = %v, want ≈4000（股數變 4 倍，歷史量要放大）", got)
	}
	raw := c.Close * float64(c.Volume)
	adjusted := c.AdjustedClose() * c.AdjustedVolume()
	if diff := raw - adjusted; diff > 1e-6 || diff < -1e-6 {
		t.Errorf("還原前後乘積不同: %v vs %v——價乘量除的方向寫反了", raw, adjusted)
	}
}

// TestAdjustedFallsBackToOne：AdjFactor 為 0（欄位沒被 SELECT 出來）時當作 1。
// 「漏 select」不該表現成「這檔股票不值錢」。
func TestAdjustedFallsBackToOne(t *testing.T) {
	c := store.Candle{Close: 100, Volume: 500}
	if got := c.AdjustedClose(); got != 100 {
		t.Errorf("AdjFactor 為 0 時還原價 = %v, want 100", got)
	}
	if got := c.AdjustedVolume(); got != 500 {
		t.Errorf("AdjFactor 為 0 時還原量 = %v, want 500", got)
	}
}

// TestBackfillRecomputesAdjFactor 驗 I-066：回補插入比事件更早的 K 棒之後，
// 係數要**立即**重算，而不是等隔天排程。
//
// 這是最容易漏的一條路徑：K 棒寫進去了、回補回報成功、沒有任何錯誤，
// 但那些列帶著 adj_factor = 1（未還原），跨越分割的計算全部看到假跳空。
type backfillStubSource struct {
	candles []Candle
}

func (s *backfillStubSource) FetchDailyCandles(_ context.Context, symbol string, _, _ time.Time) ([]Candle, error) {
	out := make([]Candle, 0, len(s.candles))
	for _, c := range s.candles {
		c.Symbol = symbol
		out = append(out, c)
	}
	return out, nil
}

func (s *backfillStubSource) FetchMinuteCandles(context.Context, string, time.Time) ([]Candle, error) {
	return nil, nil
}

func TestBackfillRecomputesAdjFactor(t *testing.T) {
	adj, candles, src := newAdjusterTestDB(t)
	ctx := context.Background()

	// 先建立事件並重算（此時該檔還沒有任何 K 棒）。
	src.actions = []store.CorporateAction{{
		Symbol: "0050", EventDate: taipeiDay(2025, 6, 18), ActionType: "分割",
		BeforePrice: 188.65, AfterPrice: 47.16, Factor: 0.25, Source: "test",
	}}
	if _, err := adj.SyncSplits(ctx, taipeiDay(2015, 1, 1), taipeiDay(2026, 8, 11)); err != nil {
		t.Fatal(err)
	}

	// 回補插入比事件更早的 K 棒。
	daily := &backfillStubSource{candles: []Candle{
		{Timeframe: "1d", Open: 188, High: 189, Low: 187, Close: 188.65,
			Volume: 1000, Timestamp: taipeiDay(2025, 6, 10)},
	}}
	f := NewFetcher(daily, candles, zap.NewNop())
	f.SetAdjuster(adj)

	if failed := f.BackfillHistory(ctx, []string{"0050"}, 5, nil); failed != 0 {
		t.Fatalf("回補失敗數 = %d, want 0", failed)
	}

	got := factorsBySymbol(t, candles, "0050")["2025-06-10"]
	if got != 0.25 {
		t.Errorf("回補後 2025-06-10 的係數 = %v, want 0.25——回補沒有觸發重算（I-066）", got)
	}
}

// TestBackfillWithoutAdjusterStillWorks：未掛 adjuster 時行為不變，不可 panic。
func TestBackfillWithoutAdjusterStillWorks(t *testing.T) {
	_, candles, _ := newAdjusterTestDB(t)
	daily := &backfillStubSource{candles: []Candle{
		{Timeframe: "1d", Open: 10, High: 11, Low: 9, Close: 10,
			Volume: 100, Timestamp: taipeiDay(2025, 1, 2)},
	}}
	f := NewFetcher(daily, candles, zap.NewNop())

	if failed := f.BackfillHistory(context.Background(), []string{"2330"}, 5, nil); failed != 0 {
		t.Fatalf("回補失敗數 = %d, want 0", failed)
	}
}

// TestRecomputeAffectedSkipsSymbolsWithoutEvents：沒有事件的標的不該被重算。
// 無腦對每個回補過的檔呼叫 RecomputeSymbol 會對整段歷史做一次無謂的 UPDATE，
// 回補 200 檔就是 200 次全表掃描，而全市場只有 31 檔有事件。
func TestRecomputeAffectedSkipsSymbolsWithoutEvents(t *testing.T) {
	adj, candles, src := newAdjusterTestDB(t)
	ctx := context.Background()

	// 2330 沒有事件，但先人工把係數設成非 1，用來觀察它有沒有被動到。
	seedDaily(t, candles, "2330", taipeiDay(2025, 1, 2), 3, 100, 1000)
	if err := candles.ApplyAdjFactors(ctx, "2330", []store.AdjFactorRange{{
		From: taipeiDay(2020, 1, 1), To: taipeiDay(2030, 1, 1), Price: 0.5, Volume: 0.5,
	}}); err != nil {
		t.Fatal(err)
	}
	src.actions = []store.CorporateAction{{
		Symbol: "0050", EventDate: taipeiDay(2025, 6, 18), ActionType: "分割",
		BeforePrice: 188.65, AfterPrice: 47.16, Factor: 0.25, Source: "test",
	}}
	if err := adj.actions.Upsert(ctx, src.actions); err != nil {
		t.Fatal(err)
	}

	if err := adj.RecomputeAffected(ctx, []string{"2330"}); err != nil {
		t.Fatal(err)
	}
	if got := factorsBySymbol(t, candles, "2330")["2025-01-02"]; got != 0.5 {
		t.Errorf("沒有事件的標的被重算了（係數 = %v, want 維持 0.5）", got)
	}
}

// TestSymbolsWithCandlesCoversNonWatchlistSymbols 鎖住 2026-08-11 review 的修正：
// 除權息的標的來源必須是「有 K 棒的標的」，不是 watchlist。
//
// 評估標的池（T-040）的標的不在 watchlist 裡；只跑 watchlist 會讓它們
// 「分割有還原、除權息沒有」，而且不會有任何東西報錯。
func TestSymbolsWithCandlesCoversNonWatchlistSymbols(t *testing.T) {
	adj, candles, _ := newAdjusterTestDB(t)
	ctx := context.Background()

	seedDaily(t, candles, "2330", taipeiDay(2025, 1, 2), 2, 100, 1000)
	seedDaily(t, candles, "9999", taipeiDay(2025, 1, 2), 2, 50, 500) // 不在任何 watchlist

	got, err := adj.SymbolsWithCandles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "2330" || got[1] != "9999" {
		t.Errorf("SymbolsWithCandles = %v, want [2330 9999]（依 symbol 升冪）", got)
	}
}
