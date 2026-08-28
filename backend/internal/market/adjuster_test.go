package market

import (
	"context"
	"errors"
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

// TestBackfillRecomputesAdjFactor 驗：回補插入比事件更早的 K 棒之後，
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
		t.Errorf("回補後 2025-06-10 的係數 = %v, want 0.25——回補沒有觸發重算", got)
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

// ── SyncPerSymbolEvents 的逾時止血與失敗計數（2026-08-24）──

// stubDividendSource 依 symbol 決定成功或失敗，並記錄實際被問到的 symbol 順序。
// 記錄「被問到誰」是關鍵：逾時止血要驗的是**沒有發出請求**，不是「請求失敗了」。
type stubDividendSource struct {
	asked   []string
	failFor map[string]bool
	actions map[string][]store.CorporateAction
	onCall  func(symbol string)
}

func (s *stubDividendSource) FetchDividends(_ context.Context, symbol string) ([]store.CorporateAction, error) {
	s.asked = append(s.asked, symbol)
	if s.onCall != nil {
		s.onCall(symbol)
	}
	if s.failFor[symbol] {
		return nil, errors.New("fetch dividends boom")
	}
	return s.actions[symbol], nil
}

// failingUpsertRepo 包一層真的 repo，只讓 Upsert 失敗，用來製造
// 「同一檔在 fetch 與 upsert 都失敗」這種會被重複計數的情境。
type failingUpsertRepo struct {
	store.CorporateActionRepo
}

func (r *failingUpsertRepo) Upsert(context.Context, []store.CorporateAction) error {
	return errors.New("upsert boom")
}

func dividendAction(symbol string) store.CorporateAction {
	return store.CorporateAction{
		Symbol: symbol, EventDate: taipeiDay(2025, 7, 15),
		ActionType:  store.CorporateActionDividend,
		BeforePrice: 100, AfterPrice: 97, Factor: 0.97, VolumeFactor: 1, Source: "test",
	}
}

// TestSyncPerSymbolEventsSkipsAllWhenContextAlreadyDone 驗 ctx 已逾時的情況下
// **一個請求都不該送出**。修改前的迴圈不看 ctx，會把剩下的檔全跑完，
// 每檔兩次注定失敗的呼叫外加兩行 warn log。
func TestSyncPerSymbolEventsSkipsAllWhenContextAlreadyDone(t *testing.T) {
	adj, _, _ := newAdjusterTestDB(t)
	div := &stubDividendSource{}
	adj.SetDividendSource(div)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	processed, failed, err := adj.SyncPerSymbolEvents(ctx, []string{"0050", "2330", "8088"})
	if err == nil {
		t.Fatal("ctx 已取消時應回傳錯誤，讓上層記成 partial")
	}
	if len(div.asked) != 0 {
		t.Errorf("ctx 已取消卻仍送出請求：%v", div.asked)
	}
	if processed != 0 || failed != 0 {
		t.Errorf("processed=%d failed=%d，期望都是 0", processed, failed)
	}
}

// TestSyncPerSymbolEventsStopsMidwayOnCancel 驗跑到一半被取消時立刻停，
// 且 processed 停在實際跑完的檔數——上層要靠 planned-processed 換算未處理數。
func TestSyncPerSymbolEventsStopsMidwayOnCancel(t *testing.T) {
	adj, _, _ := newAdjusterTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	div := &stubDividendSource{onCall: func(symbol string) {
		if symbol == "2330" {
			cancel() // 第二檔跑完後逾時
		}
	}}
	adj.SetDividendSource(div)
	t.Cleanup(cancel)

	symbols := []string{"0050", "2330", "8088", "9999"}
	processed, _, err := adj.SyncPerSymbolEvents(ctx, symbols)
	if err == nil {
		t.Fatal("中途取消應回傳錯誤")
	}
	if processed != 2 {
		t.Errorf("processed = %d, 期望 2（0050 與 2330）", processed)
	}
	if len(div.asked) != 2 {
		t.Errorf("取消後仍繼續送出請求：%v", div.asked)
	}
}

// TestSyncPerSymbolEventsCountsFailedSymbols 驗失敗檔數會往上傳。
// 修改前的簽章只回 (total, nil)，808 檔失敗在 job_runs 裡完全看不出來。
func TestSyncPerSymbolEventsCountsFailedSymbols(t *testing.T) {
	adj, _, _ := newAdjusterTestDB(t)
	adj.SetDividendSource(&stubDividendSource{
		failFor: map[string]bool{"8088": true},
		actions: map[string][]store.CorporateAction{"0050": {dividendAction("0050")}},
	})

	processed, failed, err := adj.SyncPerSymbolEvents(context.Background(), []string{"0050", "2330", "8088"})
	if err != nil {
		t.Fatalf("沒有逾時不該回錯誤: %v", err)
	}
	if processed != 3 {
		t.Errorf("processed = %d, 期望 3", processed)
	}
	if failed != 1 {
		t.Errorf("failed = %d, 期望 1", failed)
	}
}

// TestSyncPerSymbolEventsCountsSymbolFailureOnce 驗同一檔在 fetch 與 upsert 都失敗時
// 只計一次。修改前兩處各 failed++（adjuster.go:253 與 :259），會讓 symbols_failed
// 超過實際標的數，進而讓 finishRun 的 failed >= total 誤判成 failed。
func TestSyncPerSymbolEventsCountsSymbolFailureOnce(t *testing.T) {
	adj, _, _ := newAdjusterTestDB(t)
	adj.actions = &failingUpsertRepo{CorporateActionRepo: adj.actions}
	// 除權息 fetch 失敗，但減資有拿到事件 → 走到必定失敗的 Upsert。
	adj.SetDividendSource(&stubDividendSource{failFor: map[string]bool{"0050": true}})
	adj.SetCapitalReductionSource(&stubReductionSource{
		actions: map[string][]store.CorporateAction{"0050": {dividendAction("0050")}},
	})

	processed, failed, err := adj.SyncPerSymbolEvents(context.Background(), []string{"0050"})
	if err != nil {
		t.Fatalf("個別標的失敗不該讓整輪回錯誤: %v", err)
	}
	if processed != 1 {
		t.Errorf("processed = %d, 期望 1", processed)
	}
	if failed != 1 {
		t.Errorf("failed = %d, 期望 1——同一檔失敗兩次仍只能算一檔", failed)
	}
}

// failingApplyRepo 包一層真的 repo，只讓 ApplyAdjFactors 失敗——用來製造
// 「事件寫進去了，但還原係數重算不起來」這種資料不一致的情境。
type failingApplyRepo struct {
	store.CandleRepo
}

func (r *failingApplyRepo) ApplyAdjFactors(context.Context, string, []store.AdjFactorRange) error {
	return errors.New("apply adj factors boom")
}

// TestSyncPerSymbolEventsCountsRecomputeFailure 驗**重算失敗也算這檔失敗**。
//
// 修改前（2026-08-24 review 之前）重算失敗只寫一行 log：事件已經 upsert 進去、
// 但 K 棒的 adj_factor 沒跟上，那檔的價格從此不一致，而 processed 照加、failed 不動，
// 其他標的正常時整輪還會記成 success——正是這批修改要消滅的「狀態不誠實」。
func TestSyncPerSymbolEventsCountsRecomputeFailure(t *testing.T) {
	adj, _, _ := newAdjusterTestDB(t)
	adj.candles = &failingApplyRepo{CandleRepo: adj.candles}
	// 0050 有事件 → upsert 成功 → 走到必定失敗的重算；2330 沒事件，不該被牽連。
	adj.SetDividendSource(&stubDividendSource{
		actions: map[string][]store.CorporateAction{"0050": {dividendAction("0050")}},
	})

	processed, failed, err := adj.SyncPerSymbolEvents(context.Background(), []string{"0050", "2330"})
	if err != nil {
		t.Fatalf("個別標的失敗不該讓整輪回錯誤: %v", err)
	}
	if processed != 2 {
		t.Errorf("processed = %d, 期望 2", processed)
	}
	if failed != 1 {
		t.Errorf("failed = %d, 期望 1——重算失敗的 0050 必須算進失敗檔數", failed)
	}
}

// TestSyncPerSymbolEventsReportsDeadlineHitOnLastSymbol 驗 deadline 落在**最後一檔**
// 的請求裡時，逾時原因仍會往上傳。
//
// 迴圈只在每輪開頭檢查 ctx，最後一檔跑完就不會再進下一輪，ctxErr 會留在 nil；
// 此時 processed == planned、skipped == 0，scheduler 端算不出任何未處理數，
// job_runs 就會是「partial 但 error 欄空白」，與 api-reference.md 的契約不符。
func TestSyncPerSymbolEventsReportsDeadlineHitOnLastSymbol(t *testing.T) {
	adj, _, _ := newAdjusterTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	// 只有最後一檔在請求「當中」撞到 deadline：取消後 fetch 才回錯誤。
	div := &stubDividendSource{
		failFor: map[string]bool{"2330": true},
		onCall: func(symbol string) {
			if symbol == "2330" {
				cancel()
			}
		},
	}
	adj.SetDividendSource(div)

	symbols := []string{"0050", "2330"}
	processed, failed, err := adj.SyncPerSymbolEvents(ctx, symbols)
	if processed != len(symbols) {
		t.Fatalf("processed = %d, 期望 %d（最後一檔有跑到，不是沒輪到）", processed, len(symbols))
	}
	if failed != 1 {
		t.Errorf("failed = %d, 期望 1", failed)
	}
	// 這是本條的重點：上層要靠它填 job_runs.error，否則 partial 會不帶任何原因。
	if err == nil {
		t.Fatal("deadline 落在最後一檔時仍要回傳 ctx 錯誤，否則 job_runs 會是 partial 但 error 空白")
	}
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, 期望 ctx 錯誤", err)
	}
}

// TestSyncPerSymbolEventsCleanRunReportsNoDeadline 是上一條的對照組：
// 每檔都成功、只是預算在最後一檔之後才到期，那輪是真的做完了，
// **不該**憑空長出一個逾時錯誤（會變成零失敗卻帶著 error 訊息）。
func TestSyncPerSymbolEventsCleanRunReportsNoDeadline(t *testing.T) {
	adj, _, _ := newAdjusterTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	div := &stubDividendSource{onCall: func(symbol string) {
		if symbol == "2330" { // 最後一檔，成功之後預算才到期
			cancel()
		}
	}}
	adj.SetDividendSource(div)

	processed, failed, err := adj.SyncPerSymbolEvents(ctx, []string{"0050", "2330"})
	if processed != 2 || failed != 0 {
		t.Fatalf("processed=%d failed=%d, 期望 2/0", processed, failed)
	}
	if err != nil {
		t.Errorf("err = %v, 期望 nil——每檔都跑完了，逾時只是發生在收尾之後", err)
	}
}

// TestSyncPerSymbolEventsDoesNotBlameCtxForEarlierFailure 是「歸因」的守門測試：
// 先前有檔因為**一般** API 錯誤失敗，最後一檔跑完之後預算才到期——那輪的失敗與預算無關，
// 不該回傳 ctx 錯誤。
//
// 舊寫法用整輪 `failed > 0` 推斷逾時，這個組合會被誤標成 context deadline exceeded，
// job_runs.error 因此指向錯誤的方向：讀的人會去調 timeout_sec / shard_count，
// 真正該看的是資料源的錯誤。上一條的對照組只涵蓋「全程零失敗」，補不到這裡。
func TestSyncPerSymbolEventsDoesNotBlameCtxForEarlierFailure(t *testing.T) {
	adj, _, _ := newAdjusterTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	div := &stubDividendSource{
		// 0050 是一般失敗（ctx 還活著），2330 成功之後預算才到期。
		failFor: map[string]bool{"0050": true},
		onCall: func(symbol string) {
			if symbol == "2330" {
				cancel()
			}
		},
	}
	adj.SetDividendSource(div)

	processed, failed, err := adj.SyncPerSymbolEvents(ctx, []string{"0050", "2330"})
	if processed != 2 {
		t.Fatalf("processed = %d, 期望 2（兩檔都跑到了）", processed)
	}
	if failed != 1 {
		t.Errorf("failed = %d, 期望 1——0050 的一般失敗仍要計進失敗檔數", failed)
	}
	// 本條重點：failed > 0 不等於逾時。
	if err != nil {
		t.Errorf("err = %v, 期望 nil——0050 是一般失敗，預算是在最後一檔跑完之後才到期", err)
	}
}

// TestSyncPerSymbolEventsDoesNotBlameCtxForEarlierStageOfSameSymbol 驗**同一檔內**的歸因：
// dividends 因一般 API 錯誤失敗（ctx 還活著），同檔的 reductions 成功之後預算才到期，
// 那檔的失敗原因仍然是 dividends，不是預算。
//
// 上一條對照組只涵蓋「前一檔失敗、最後一檔成功」的跨檔組合。若 deadlineHit 是等整檔
// 四個階段跑完才採樣（而不是在各失敗分支當場採樣），這個同檔組合會被誤歸因成逾時
// （2026-08-24 review 的二次修正）。
//
// 這裡 reductions 回空集合，所以不會進 Upsert／Recompute——那兩階段若在已到期的 ctx 下
// 執行並失敗，「這檔撞到預算」就是真的，不屬於誤判。
func TestSyncPerSymbolEventsDoesNotBlameCtxForEarlierStageOfSameSymbol(t *testing.T) {
	adj, _, _ := newAdjusterTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// 階段 1：dividends 一般失敗，此時 ctx 正常。
	adj.SetDividendSource(&stubDividendSource{failFor: map[string]bool{"2330": true}})
	// 階段 2：reductions 成功（回空集合），回傳之後預算才到期。
	adj.SetCapitalReductionSource(&stubReductionSource{
		onCall: func(symbol string) {
			if symbol == "2330" {
				cancel()
			}
		},
	})

	processed, failed, err := adj.SyncPerSymbolEvents(ctx, []string{"2330"})
	if processed != 1 {
		t.Fatalf("processed = %d, 期望 1", processed)
	}
	if failed != 1 {
		t.Errorf("failed = %d, 期望 1——dividends 失敗仍要計進失敗檔數", failed)
	}
	// 本條重點：失敗發生在 ctx 到期**之前**，不該歸因給預算。
	if err != nil {
		t.Errorf("err = %v, 期望 nil——dividends 是一般失敗，預算是在該階段成功之後才到期", err)
	}
}

// TestSyncPerSymbolEventsSkipsRemainingStagesAfterDeadline 驗**階段之間**也不再送出
// 注定失敗的請求：dividends 因 ctx 到期失敗後，同一檔的 reductions 一次都不該被呼叫。
//
// 修改前迴圈只在每輪開頭檢查 ctx，所以 deadline 落在 dividends 時，同檔的 reductions
// 仍會被送出一次（原本只做到標的粒度，沒做到階段粒度）。那次呼叫即使立刻失敗
// 也會燒掉一個節流槽——rateLimiter.wait 先推進 next 才判 ctx，而 limiter 全 repo 共用。
//
// 同時驗「skip 要連著記失敗」：跳過剩餘階段不等於這檔沒問題，failed 仍須為 1。
func TestSyncPerSymbolEventsSkipsRemainingStagesAfterDeadline(t *testing.T) {
	adj, _, _ := newAdjusterTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// dividends 在 ctx 到期之後才回傳錯誤——模擬 deadline 落在這個階段裡。
	adj.SetDividendSource(&stubDividendSource{
		failFor: map[string]bool{"2330": true},
		onCall: func(symbol string) {
			if symbol == "2330" {
				cancel()
			}
		},
	})
	red := &stubReductionSource{}
	adj.SetCapitalReductionSource(red)

	processed, failed, err := adj.SyncPerSymbolEvents(ctx, []string{"2330"})
	if processed != 1 {
		t.Fatalf("processed = %d, 期望 1（這檔有跑到，只是被逾時砍斷）", processed)
	}
	// 本條重點：逾時後同一檔剩下的階段不該再送出請求。
	if len(red.asked) != 0 {
		t.Errorf("逾時後仍呼叫了減資查詢：%v", red.asked)
	}
	if failed != 1 {
		t.Errorf("failed = %d, 期望 1——跳過剩餘階段不代表這檔沒問題", failed)
	}
	if err == nil {
		t.Fatal("deadline 落在 dividends 階段時要回傳 ctx 錯誤")
	}
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, 期望 ctx 錯誤", err)
	}
}

// cancelOnUpsertRepo 讓 Upsert **成功**回傳的同時預算到期，用來製造
// 「事件寫進去了，但重算已經沒有預算」這個階段組合，並記錄 ListBySymbol 的呼叫數。
//
// 觀測點選 ListBySymbol 而不是 ApplyAdjFactors：ctx 已死時 RecomputeSymbol 在
// ListBySymbol（adjuster.go:142）就會失敗，根本走不到 ApplyAdjFactors，
// 兩條路徑都是 0 次、分不出「有沒有跳過重算」。
type cancelOnUpsertRepo struct {
	store.CorporateActionRepo
	cancel    context.CancelFunc
	listCalls int
}

func (r *cancelOnUpsertRepo) Upsert(ctx context.Context, actions []store.CorporateAction) error {
	if err := r.CorporateActionRepo.Upsert(ctx, actions); err != nil {
		return err
	}
	r.cancel() // 寫入成功之後預算才到期
	return nil
}

func (r *cancelOnUpsertRepo) ListBySymbol(ctx context.Context, symbol string) ([]store.CorporateAction, error) {
	r.listCalls++
	return r.CorporateActionRepo.ListBySymbol(ctx, symbol)
}

// TestSyncPerSymbolEventsSkipsRecomputeAfterDeadline 驗階段守衛也涵蓋 Upsert 與重算之間：
// Upsert 成功回傳時預算已到期，就不該再送出注定失敗的 RecomputeSymbol。
//
// 兩條路徑的結果本來就相同（該檔計為失敗、adj_factor 留在舊值、下一輪冪等重跑自癒），
// 差別只有那一次注定失敗的本地查詢——所以這條的斷言重點是「有沒有送出」。
func TestSyncPerSymbolEventsSkipsRecomputeAfterDeadline(t *testing.T) {
	adj, _, _ := newAdjusterTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	repo := &cancelOnUpsertRepo{CorporateActionRepo: adj.actions, cancel: cancel}
	adj.actions = repo
	adj.SetDividendSource(&stubDividendSource{
		actions: map[string][]store.CorporateAction{"0050": {dividendAction("0050")}},
	})

	processed, failed, err := adj.SyncPerSymbolEvents(ctx, []string{"0050"})
	if processed != 1 {
		t.Fatalf("processed = %d, 期望 1", processed)
	}
	// 本條重點：預算到期後不再送出注定失敗的重算查詢。
	if repo.listCalls != 0 {
		t.Errorf("逾時後仍呼叫了 RecomputeSymbol：ListBySymbol 被叫了 %d 次", repo.listCalls)
	}
	if failed != 1 {
		t.Errorf("failed = %d, 期望 1——事件寫了但係數沒跟上，這檔要算失敗", failed)
	}
	if err == nil {
		t.Fatal("預算在 Upsert 之後到期時要回傳 ctx 錯誤")
	}
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, 期望 ctx 錯誤", err)
	}
}

// stubReductionSource 是減資來源的測試替身。
type stubReductionSource struct {
	asked   []string
	actions map[string][]store.CorporateAction
	onCall  func(symbol string)
}

func (s *stubReductionSource) FetchCapitalReductions(_ context.Context, symbol string) ([]store.CorporateAction, error) {
	s.asked = append(s.asked, symbol)
	if s.onCall != nil {
		s.onCall(symbol)
	}
	return s.actions[symbol], nil
}
