package indicator

// CalcRSI 使用 Wilder smoothing（需要 period+1 以上的收盤價）
func CalcRSI(closes []float64, period int) float64 {
	if len(closes) < period+1 {
		return 0
	}

	// 計算初始平均漲跌幅（算術平均）
	var gainSum, lossSum float64
	for i := 1; i <= period; i++ {
		diff := closes[i] - closes[i-1]
		if diff > 0 {
			gainSum += diff
		} else {
			lossSum -= diff
		}
	}
	avgGain := gainSum / float64(period)
	avgLoss := lossSum / float64(period)

	// 後續用 Wilder smoothing
	for i := period + 1; i < len(closes); i++ {
		diff := closes[i] - closes[i-1]
		if diff > 0 {
			avgGain = (avgGain*float64(period-1) + diff) / float64(period)
			avgLoss = (avgLoss * float64(period-1)) / float64(period)
		} else {
			avgGain = (avgGain * float64(period-1)) / float64(period)
			avgLoss = (avgLoss*float64(period-1) + (-diff)) / float64(period)
		}
	}

	if avgLoss == 0 {
		return 100
	}
	rs := avgGain / avgLoss
	return 100 - (100 / (1 + rs))
}
