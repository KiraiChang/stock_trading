package market

import (
	"context"
	"time"
)

// MarketDataSource 為 Fetcher 依賴的歷史資料來源能力（目前由 FinMind 提供）。
// 抽成介面是為了讓 Fugle 之類的即時資料來源可以並行掛載，而不需要 Fetcher
// 綁死具體型別。
type MarketDataSource interface {
	FetchDailyCandles(ctx context.Context, symbol string, start, end time.Time) ([]Candle, error)
	FetchMinuteCandles(ctx context.Context, symbol string, date time.Time) ([]Candle, error)
}

// QuoteSource 為 Tier 1 廣度掃描用的輪詢式行情來源，受 REST 呼叫頻率配額限制
// （Fugle 免費方案為 60 次/分鐘）。
type QuoteSource interface {
	// FetchIntradayCandles 拉取當日盤中 K 線（Fugle: GET /intraday/candles/{symbol}）
	FetchIntradayCandles(ctx context.Context, symbol string) ([]Candle, error)
	// RateLimit 回傳每分鐘可呼叫次數，用於換算 round-robin 掃描批次
	RateLimit() int
}

// BatchQuoteSource 為支援「單次請求批次多檔」的 Tier 1 輪詢式行情來源（Yahoo）。
// 相較於 QuoteSource 一檔一請求，批次可大幅降低全 watchlist／全市場掃描的請求數。
type BatchQuoteSource interface {
	// FetchIntradayCandlesBatch 一次拉取多檔當日 1 分K，symbols 為系統 symbol（如 "2330"），
	// 回傳以系統 symbol 為 key 的 map；無資料的 symbol 不會出現在 map 中。
	FetchIntradayCandlesBatch(ctx context.Context, symbols []string) (map[string][]Candle, error)
	// RateLimit 回傳每分鐘可呼叫次數（一次批次請求計為一次）
	RateLimit() int
	// BatchSize 回傳單次請求建議帶入的最大 symbol 數
	BatchSize() int
}

// StreamingSource 為 Tier 2 熱點用的即時推送來源，受同時訂閱檔數限制
// （Fugle 免費方案為 5 檔、1 條 WebSocket 連線）。
type StreamingSource interface {
	// Subscribe 訂閱單一股票的即時分K推送，收到新的（或更新中的）K棒時呼叫 onCandle
	Subscribe(ctx context.Context, symbol string, onCandle func(Candle)) error
	Unsubscribe(ctx context.Context, symbol string) error
	// MaxSubscriptions 回傳同時可訂閱的檔數上限
	MaxSubscriptions() int
	Close() error
}
