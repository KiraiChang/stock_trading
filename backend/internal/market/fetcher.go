package market

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/store"
	"github.com/trading/backend/pkg/timeutil"
)

type Fetcher struct {
	client  MarketDataSource
	candles store.CandleRepo
	log     *zap.Logger

	// quoteSource / streamSource 為選填的 Fugle 並行資料源（Tier 1 / Tier 2），
	// 未設定（SetFugle 未呼叫）時相關方法會回傳錯誤，呼叫端應維持走 FinMind。
	quoteSource  QuoteSource
	streamSource StreamingSource

	// batchSource 為選填的批次盤中源（Yahoo），支援單次請求多檔；與 Fugle 並存，
	// 未設定（SetIntradaySource 未呼叫）時 FetchAndStoreIntradayBatch 回傳錯誤。
	batchSource BatchQuoteSource
}

func NewFetcher(client MarketDataSource, candles store.CandleRepo, log *zap.Logger) *Fetcher {
	return &Fetcher{
		client:  client,
		candles: candles,
		log:     log,
	}
}

// SetFugle 掛載 Fugle 的 Tier 1（REST 廣度掃描）／Tier 2（WebSocket 熱點）資料源。
// 兩者可分別為 nil（例如尚未開通即時方案時只掛 Tier 1）。
func (f *Fetcher) SetFugle(quote QuoteSource, stream StreamingSource) {
	f.quoteSource = quote
	f.streamSource = stream
}

// SetIntradaySource 掛載通用的批次盤中源（目前為 Yahoo）。與 SetFugle 並存，
// 兩者可擇一或並用；未設定時 FetchAndStoreIntradayBatch 回傳錯誤。
func (f *Fetcher) SetIntradaySource(src BatchQuoteSource) {
	f.batchSource = src
}

// HasIntradaySource 回報是否已掛載批次盤中源，供 scheduler 決定盤中是否走批次路徑。
func (f *Fetcher) HasIntradaySource() bool {
	return f.batchSource != nil
}

// IntradayBatchSize 回傳批次盤中源建議的單次 symbol 數；未掛載時回傳 0。
func (f *Fetcher) IntradayBatchSize() int {
	if f.batchSource == nil {
		return 0
	}
	return f.batchSource.BatchSize()
}

// FetchAndStoreIntradayBatch 用批次盤中源一次拉取多檔當日 1 分K 並寫入 candles，
// 回傳成功寫入的 symbol 數。個別 symbol 寫入失敗只記 log、不中斷其他 symbol。
func (f *Fetcher) FetchAndStoreIntradayBatch(ctx context.Context, symbols []string) (int, error) {
	if f.batchSource == nil {
		return 0, fmt.Errorf("intraday batch source not configured")
	}
	bySymbol, err := f.batchSource.FetchIntradayCandlesBatch(ctx, symbols)
	if err != nil {
		return 0, err
	}

	stored := 0
	for sym, candles := range bySymbol {
		if len(candles) == 0 {
			continue
		}
		if err := f.candles.BulkInsert(ctx, f.toStoreCandles(candles)); err != nil {
			f.log.Warn("intraday batch store failed", zap.String("symbol", sym), zap.Error(err))
			continue
		}
		stored++
	}
	f.log.Info("fetched intraday batch candles", zap.Int("requested", len(symbols)), zap.Int("stored", stored))
	return stored, nil
}

// FetchAndStoreFugleIntraday 為 Tier 1：用 Fugle REST 拉取當日 1 分K 並寫入 candles，
// 取代（或在驗證階段並行於）FinMind 的 FetchAndStoreMinute。
func (f *Fetcher) FetchAndStoreFugleIntraday(ctx context.Context, symbol string) error {
	if f.quoteSource == nil {
		return fmt.Errorf("fugle quote source not configured")
	}
	candles, err := f.quoteSource.FetchIntradayCandles(ctx, symbol)
	if err != nil {
		return err
	}

	storeCandles := f.toStoreCandles(candles)
	if err := f.candles.BulkInsert(ctx, storeCandles); err != nil {
		return err
	}
	f.log.Info("fetched fugle intraday candles", zap.String("symbol", symbol), zap.Int("count", len(storeCandles)))
	return nil
}

// SubscribeRealtime 為 Tier 2：透過 Fugle WebSocket 訂閱單一股票的即時分K，
// 收到推送時直接寫入 candles。
func (f *Fetcher) SubscribeRealtime(ctx context.Context, symbol string) error {
	if f.streamSource == nil {
		return fmt.Errorf("fugle stream source not configured")
	}
	return f.streamSource.Subscribe(ctx, symbol, func(c Candle) {
		if err := f.candles.BulkInsert(context.Background(), f.toStoreCandles([]Candle{c})); err != nil {
			f.log.Warn("fugle realtime candle store failed", zap.String("symbol", symbol), zap.Error(err))
		}
	})
}

// UnsubscribeRealtime 取消 Tier 2 對某檔股票的即時訂閱（例如熱點名額被釋放）。
func (f *Fetcher) UnsubscribeRealtime(ctx context.Context, symbol string) error {
	if f.streamSource == nil {
		return nil
	}
	return f.streamSource.Unsubscribe(ctx, symbol)
}

// FugleMaxSubscriptions 回傳 Tier 2 同時可訂閱的檔數上限；Fugle 未設定時回傳 0。
func (f *Fetcher) FugleMaxSubscriptions() int {
	if f.streamSource == nil {
		return 0
	}
	return f.streamSource.MaxSubscriptions()
}

func (f *Fetcher) FetchAndStoreDaily(ctx context.Context, symbol string, date time.Time) error {
	candles, err := f.client.FetchDailyCandles(ctx, symbol, date, date)
	if err != nil {
		return err
	}

	storeCandles := f.toStoreCandles(candles)
	if err := f.candles.BulkInsert(ctx, storeCandles); err != nil {
		return err
	}

	f.log.Info("fetched daily candles", zap.String("symbol", symbol), zap.Int("count", len(storeCandles)))
	return nil
}

func (f *Fetcher) FetchAndStoreMinute(ctx context.Context, symbol string, date time.Time) error {
	candles, err := f.client.FetchMinuteCandles(ctx, symbol, date)
	if err != nil {
		return err
	}

	storeCandles := f.toStoreCandles(candles)
	if err := f.candles.BulkInsert(ctx, storeCandles); err != nil {
		return err
	}

	f.log.Info("fetched minute candles", zap.String("symbol", symbol), zap.Int("count", len(storeCandles)))
	return nil
}

// BackfillHistory 補齊歷史日K，days 為往前幾天，回傳失敗的股票數量。
// onSymbol 可為 nil；不為 nil 時**每一檔都會呼叫一次**（成功時 err 為 nil），
// 供呼叫端即時更新進度。比照 chip.Syncer.SyncRange 的回呼形狀——沒有這個回呼就
// 只能等整批跑完才知道結果，20 檔在 rate limit 下要 4 分鐘。
func (f *Fetcher) BackfillHistory(ctx context.Context, symbols []string, days int, onSymbol func(symbol string, err error)) int {
	end := timeutil.TodayTaipei()
	start := end.AddDate(0, 0, -days)

	report := func(symbol string, err error) {
		if onSymbol != nil {
			onSymbol(symbol, err)
		}
	}

	failed := 0
	for _, symbol := range symbols {
		candles, err := f.client.FetchDailyCandles(ctx, symbol, start, end)
		if err != nil {
			f.log.Warn("backfill failed", zap.String("symbol", symbol), zap.Error(err))
			failed++
			report(symbol, err)
			continue
		}
		storeCandles := f.toStoreCandles(candles)
		if err := f.candles.BulkInsert(ctx, storeCandles); err != nil {
			f.log.Warn("backfill insert failed", zap.String("symbol", symbol), zap.Error(err))
			failed++
			report(symbol, err)
			continue
		}
		f.log.Info("backfill done", zap.String("symbol", symbol), zap.Int("count", len(storeCandles)))
		report(symbol, nil)
	}
	return failed
}

// toStoreCandles 轉成 store 層的 Candle，並擋掉價格非正的 K 棒。
//
// 為什麼要擋（2026-08-10，見 docs/database-schema.md 的 candles 章節）：live DB 出現過 4 根 OHLCV 全為 0 的
// 日 K（2454 2016-05-13、3630 2024-12-18、2317 2025-07-30、1101 2025-08-13）。那幾天
// 其他二十多檔都正常交易且有量，所以不是整輪抓取失敗，而是單檔單日的異常；個股停牌
// （上游以 0 表示無成交）與上游 glitch 都有可能，但兩種情況的正解相同：
// **無成交的日子應該是「沒有那筆資料」，不是一根價格為 0 的 K 棒**。
//
// 留一根零價 K 棒的代價遠大於少一天：MA / RSI / ATR、zone 建構、breakout 偵測全都會被
// 那一根污染，而且不會有任何東西報錯——當初是靠 SR Zone 的 MAE 剛好等於 −100% 才浮出來的。
//
// 只驗價格不驗 volume：成交量為 0 在盤中分K 是正常的（該分鐘沒有成交），
// 價格為 0 則在任何情況下都不成立。
func (f *Fetcher) toStoreCandles(cs []Candle) []store.Candle {
	result := make([]store.Candle, 0, len(cs))
	for _, c := range cs {
		if c.Open <= 0 || c.High <= 0 || c.Low <= 0 || c.Close <= 0 {
			f.log.Warn("skip candle with non-positive price",
				zap.String("symbol", c.Symbol),
				zap.String("timeframe", c.Timeframe),
				zap.Time("ts", c.Timestamp),
				zap.Float64("open", c.Open),
				zap.Float64("high", c.High),
				zap.Float64("low", c.Low),
				zap.Float64("close", c.Close),
			)
			continue
		}
		result = append(result, store.Candle{
			Symbol:    c.Symbol,
			Timeframe: c.Timeframe,
			Open:      c.Open,
			High:      c.High,
			Low:       c.Low,
			Close:     c.Close,
			Volume:    c.Volume,
			Amount:    c.Amount,
			Timestamp: c.Timestamp,
		})
	}
	return result
}
