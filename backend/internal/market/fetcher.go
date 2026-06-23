package market

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/store"
	"github.com/trading/backend/pkg/timeutil"
)

type Fetcher struct {
	client  *FinMindClient
	candles store.CandleRepo
	log     *zap.Logger
}

func NewFetcher(client *FinMindClient, candles store.CandleRepo, log *zap.Logger) *Fetcher {
	return &Fetcher{
		client:  client,
		candles: candles,
		log:     log,
	}
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

// BackfillHistory 補齊歷史日K，days 為往前幾天
func (f *Fetcher) BackfillHistory(ctx context.Context, symbols []string, days int) error {
	end := timeutil.TodayTaipei()
	start := end.AddDate(0, 0, -days)

	for _, symbol := range symbols {
		candles, err := f.client.FetchDailyCandles(ctx, symbol, start, end)
		if err != nil {
			f.log.Warn("backfill failed", zap.String("symbol", symbol), zap.Error(err))
			continue
		}
		storeCandles := toStoreCandles(candles)
		if err := f.candles.BulkInsert(ctx, storeCandles); err != nil {
			f.log.Warn("backfill insert failed", zap.String("symbol", symbol), zap.Error(err))
			continue
		}
		f.log.Info("backfill done", zap.String("symbol", symbol), zap.Int("count", len(storeCandles)))

		// 簡單的 rate limit：每次請求後等待 200ms
		time.Sleep(200 * time.Millisecond)
	}
	return nil
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
