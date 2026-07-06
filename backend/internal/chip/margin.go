package chip

import "github.com/trading/backend/internal/store"

// marginRiskThreshold 為融資使用率風險門檻，0~1 的比例（0.8 代表使用率 80%）。
const marginRiskThreshold = 0.8

// CalcMarginScore 依 docs/chip-analysis-design.md 第5節解讀規則計算融資融券
// 分數（-100~100）：
//   - 融資增加且價格下跌：偏弱
//   - 融資減少且價格上漲：籌碼沉澱，偏強
//   - 融券增加且價格上漲（可能軋空）：偏強
//
// 融資使用率過高的風險另由 IsMarginRisk 標記，用來覆寫最終 signal 為 RISK，
// 不透過分數表達（因為「風險」與「方向」是兩個不同維度）。
func CalcMarginScore(latest store.MarginTrade, priceChangePercent float64) (score float64, reasons []string) {
	switch {
	case latest.MarginChange > 0 && priceChangePercent < 0:
		score -= 30
		reasons = append(reasons, "融資增加且價格下跌，籌碼偏弱")
	case latest.MarginChange < 0 && priceChangePercent > 0:
		score += 30
		reasons = append(reasons, "融資減少且價格上漲，籌碼沉澱偏強")
	}

	if latest.ShortChange > 0 && priceChangePercent > 0 {
		score += 20
		reasons = append(reasons, "融券增加且價格上漲，可能軋空偏強")
	}

	if IsMarginRisk(latest.MarginUsageRate, marginRiskThreshold) {
		reasons = append(reasons, "融資使用率過高，風險升高")
	}

	return clamp(score, 100), reasons
}

// IsMarginRisk 判斷融資使用率是否達到風險門檻（threshold 與 usageRate 同單位，
// 皆為 0~1 的比例）。provider 未提供額度上限（usageRate 無效）時視為無風險。
func IsMarginRisk(usageRate store.NullFloat64, threshold float64) bool {
	return usageRate.Valid && usageRate.Float64 >= threshold
}
