package market

import "time"

// FinMind 日K 原始欄位
type RawDailyCandle struct {
	Date         string  `json:"date"`
	Open         float64 `json:"open"`
	High         float64 `json:"max"`
	Low          float64 `json:"min"`
	Close        float64 `json:"close"`
	Volume       int64   `json:"Trading_Volume"`
	Amount       float64 `json:"Trading_money"`
	Spread       float64 `json:"spread"`
	TurnoverRate float64 `json:"turnover_rate"`
}

// FinMind 分K 原始欄位
type RawMinuteCandle struct {
	Date   string  `json:"date"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume int64   `json:"volume"`
	Amount float64 `json:"amount"`
}

type FinMindResponse struct {
	Msg    string `json:"msg"`
	Status int    `json:"status"`
	Data   []map[string]interface{} `json:"data"`
}

// 轉換後的標準 K 棒
type Candle struct {
	Symbol    string
	Timeframe string
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    int64
	Amount    float64
	Timestamp time.Time
}
