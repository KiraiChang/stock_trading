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

	storeCandles := toStoreCandles(candles)
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
		if err := f.candles.BulkInsert(context.Background(), toStoreCandles([]Candle{c})); err != nil {
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

	storeCandles := toStoreCandles(candles)
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

	storeCandles := toStoreCandles(candles)
	if err := f.candles.BulkInsert(ctx, storeCandles); err != nil {
		return err
	}

	f.log.Info("fetched minute candles", zap.String("symbol", symbol), zap.Int("count", len(storeCandles)))
	return nil
}

// BackfillHistory 補齊歷史日K，days 為往前幾天，回傳失敗的股票數量
func (f *Fetcher) BackfillHistory(ctx context.Context, symbols []string, days int) int {
	end := timeutil.TodayTaipei()
	start := end.AddDate(0, 0, -days)

	failed := 0
	for _, symbol := range symbols {
		candles, err := f.client.FetchDailyCandles(ctx, symbol, start, end)
		if err != nil {
			f.log.Warn("backfill failed", zap.String("symbol", symbol), zap.Error(err))
			failed++
			continue
		}
		storeCandles := toStoreCandles(candles)
		if err := f.candles.BulkInsert(ctx, storeCandles); err != nil {
			f.log.Warn("backfill insert failed", zap.String("symbol", symbol), zap.Error(err))
			failed++
			continue
		}
		f.log.Info("backfill done", zap.String("symbol", symbol), zap.Int("count", len(storeCandles)))
	}
	return failed
}

func toStoreCandles(cs []Candle) []store.Candle {
	result := make([]store.Candle, len(cs))
	for i, c := range cs {
		result[i] = store.Candle{
			Symbol:    c.Symbol,
			Timeframe: c.Timeframe,
			Open:      c.Open,
			High:      c.High,
			Low:       c.Low,
			Close:     c.Close,
			Volume:    c.Volume,
			Amount:    c.Amount,
			Timestamp: c.Timestamp,
		}
	}
	return result
}
