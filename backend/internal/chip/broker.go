package chip

import "github.com/trading/backend/internal/store"

// brokerConcentrationTopN 為集中度計算取樣的分點檔數（Top N 買超/賣超合計）。
const brokerConcentrationTopN = 10

// CalcTopNNetBuy 取前 n 名買超與賣超分點的合計。trades 需已依 net_buy DESC
// 排序（比照 store.BrokerTradeRepo.GetByDate 的回傳慣例）。topBuy 只加總正值
// （買超），topSell 只加總負值（賣超），避免全部買超或全部賣超時另一邊被
// 錯誤地計入。
func CalcTopNNetBuy(trades []store.BrokerTrade, n int) (topBuy, topSell int64) {
	for i := 0; i < len(trades) && i < n; i++ {
		if trades[i].NetBuy > 0 {
			topBuy += trades[i].NetBuy
		}
	}
	for i := len(trades) - 1; i >= 0 && len(trades)-i <= n; i-- {
		if trades[i].NetBuy < 0 {
			topSell += trades[i].NetBuy
		}
	}
	return topBuy, topSell
}

// CalcConcentration 計算籌碼集中度：abs(netBuy)/dailyVolume。dailyVolume<=0
// 時回傳 0，避免除以零。
func CalcConcentration(netBuy, dailyVolume int64) float64 {
	if dailyVolume <= 0 {
		return 0
	}
	c := float64(netBuy) / float64(dailyVolume)
	if c < 0 {
		c = -c
	}
	return c
}

// CalcBrokerScore 依分點集中度與買賣方向計算 broker_score（-100~100）。無
// 分點資料時（trades 為空，例如 FinMind 不支援此資料類型）fallback 為 0/
// 中性，不阻塞其他分數計算（見 docs/chip-analysis-design.md 第2節 fallback 策略）。
func CalcBrokerScore(trades []store.BrokerTrade, dailyVolume int64) (score float64, reasons []string) {
	if len(trades) == 0 {
		return 0, []string{"無分點資料，broker_score 中性"}
	}

	topBuy, topSell := CalcTopNNetBuy(trades, brokerConcentrationTopN)
	buyConcentration := CalcConcentration(topBuy, dailyVolume)
	sellConcentration := CalcConcentration(topSell, dailyVolume)

	score = clamp((buyConcentration-sellConcentration)*500, 100)

	switch {
	case buyConcentration > sellConcentration:
		reasons = append(reasons, "主力買超集中度提高")
	case sellConcentration > buyConcentration:
		reasons = append(reasons, "主力賣超集中度提高")
	}
	return score, reasons
}
