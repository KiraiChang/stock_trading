package market

import "encoding/json"

// ── Yahoo 股市內部端點 FinanceChartService.ApacLibraCharts 的回應結構 ──────
//
// 實測（非官方文件）：回應為 JSON 陣列，每檔一個 element。端點名稱的 type=tick
// 有誤導性——實際回傳的是 1 分鐘 OHLCV K 棒（chart.timestamp 配 chart.indicators.quote[0]）
// 外加 chart.quote 即時快照，並非逐筆 tick。詳見 docs/yahoo-intraday-integration.md。

type yahooChartEntry struct {
	Symbol string     `json:"symbol"`
	Chart  yahooChart `json:"chart"`
}

type yahooChart struct {
	Meta       yahooMeta       `json:"meta"`
	Timestamp  []int64         `json:"timestamp"`
	Indicators yahooIndicators `json:"indicators"`
	// Quote 為即時快照，組 K 棒時不需要；保留原始 JSON 供 cmd/yahoo-check 檢視。
	Quote json.RawMessage `json:"quote"`
}

type yahooMeta struct {
	Name      string `json:"name"`
	QuoteType string `json:"quoteType"` // EQUITY / ETF
	GMTOffset int    `json:"gmtoffset"`
}

type yahooIndicators struct {
	Quote []yahooOHLCV `json:"quote"`
}

// yahooOHLCV 的各陣列與 chart.timestamp 一一對齊；盤前/盤後或缺值的位置為 null，
// 故用指標型別以區分「0」與「無資料」。
type yahooOHLCV struct {
	Open   []*float64 `json:"open"`
	High   []*float64 `json:"high"`
	Low    []*float64 `json:"low"`
	Close  []*float64 `json:"close"`
	Volume []*int64   `json:"volume"`
}
