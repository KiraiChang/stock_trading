package market

import (
	"context"
	"errors"
	"time"
)

// ErrBrokerDataUnsupported 代表目前 provider 沒有分點資料來源（例如 FinMind
// 沒有這個 dataset），呼叫端（chip.Syncer）應把 broker_score fallback 為中性，
//而不是視為抓取失敗（見 docs/chip-analysis-design.md 第2節 fallback 策略）。
var ErrBrokerDataUnsupported = errors.New("chip: broker/branch trading data not supported by this provider")

// InstitutionalTrade 為三大法人買賣超原始資料，單位為股。
type InstitutionalTrade struct {
	Symbol                string
	Date                  time.Time
	ForeignNetBuy         int64
	InvestmentTrustNetBuy int64
	DealerNetBuy          int64
	TotalNetBuy           int64
}

// MarginTrade 為融資融券原始資料，單位為股（provider 若回傳「張」需在轉換
// 時換算，見 finmind_chip.go 的說明）。UsageRate 為 0~1 的比例，provider
// 未提供額度上限時為 nil。
type MarginTrade struct {
	Symbol          string
	Date            time.Time
	MarginBalance   int64
	MarginChange    int64
	ShortBalance    int64
	ShortChange     int64
	MarginUsageRate *float64
	ShortUsageRate  *float64
}

// BrokerTrade 為券商分點買賣超原始資料，單位為股。
type BrokerTrade struct {
	Symbol     string
	Date       time.Time
	BrokerName string
	BranchName string
	BuyVolume  int64
	SellVolume int64
	NetBuy     int64
}

// ChipDataSource 為籌碼資料來源能力。FetchInstitutionalTrades/FetchMarginTrades
// 用 range 版本（比照 MarketDataSource.FetchDailyCandles），因為 FinMind 這兩個
// dataset 本身支援區間查詢——若逐日呼叫，500 個交易日回補在預設 rate limit
// 下會需要極長時間，不可行。FetchBrokerTrades 維持單日版：分點是「當日排行」
// 語意，且目前沒有支援區間查詢的資料源。
type ChipDataSource interface {
	FetchInstitutionalTrades(ctx context.Context, symbol string, start, end time.Time) ([]InstitutionalTrade, error)
	FetchMarginTrades(ctx context.Context, symbol string, start, end time.Time) ([]MarginTrade, error)
	FetchBrokerTrades(ctx context.Context, symbol string, date time.Time) ([]BrokerTrade, error)
}
