// Package chip 實作籌碼分析的分數計算邏輯（見 docs/chip-analysis-design.md）。
// 除 Syncer（sync.go）以外皆為純函式，方便單元測試與未來被訊號引擎/回測引用。
package chip

import (
	"time"

	"github.com/trading/backend/internal/store"
)

// Signal 代表籌碼面的整體方向判斷。
type Signal string

const (
	Bullish Signal = "BULLISH"
	Bearish Signal = "BEARISH"
	Neutral Signal = "NEUTRAL"
	Risk    Signal = "RISK"
)

// Score 是單一交易日的完整籌碼分析結果，對應 store.ChipScore 的計算來源。
type Score struct {
	InstitutionalScore float64
	MarginScore        float64
	BrokerScore        float64
	ConcentrationScore float64
	TotalScore         float64
	Signal             Signal
	Reasons            []string
}

// ChipScoreInput 彙整計算單一交易日籌碼分數所需的全部輸入，皆由呼叫端
// （chip.Syncer）從 store 查詢後組裝，Calculate 本身不做任何 IO。
type ChipScoreInput struct {
	Symbol string
	Date   time.Time

	// InstitutionalHistory 依 trade_date 升冪排序，最後一筆須為 Date 當日資料，
	// 建議帶 20 筆以上供連續買賣超天數與 5日累積買賣超計算。
	InstitutionalHistory []store.InstitutionalTrade

	// MarginHistory 依 trade_date 升冪排序，最後一筆須為 Date 當日資料。
	MarginHistory []store.MarginTrade

	// BrokerTrades 為當日全部分點列（依 net_buy DESC 排序），可能為空
	// （fallback broker_score=0/中性，不阻塞其他分數計算）。
	BrokerTrades []store.BrokerTrade

	// DailyVolume 為當日總成交量（股），來自 candles。
	DailyVolume int64

	// AvgVolume20 為近 20 日均量（股），來自 candles，用於法人買賣超相對
	// 成交量規模的趨勢分數計算。
	AvgVolume20 float64

	// PriceChangePercent 為當日收盤 vs 前一日收盤的漲跌幅（%），用於融資
	// 融券規則判斷「融券增加且價格突破」。
	PriceChangePercent float64
}

// clamp 將值限制在 [-limit, limit] 範圍內。
func clamp(v, limit float64) float64 {
	if v > limit {
		return limit
	}
	if v < -limit {
		return -limit
	}
	return v
}
