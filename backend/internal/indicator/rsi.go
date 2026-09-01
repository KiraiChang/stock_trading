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
		// 完全沒有波動（整個視窗每一根都平盤）時 avgGain 也會是 0——
		// diff == 0 走的是上面的 else 分支，兩邊都加 0，於是「都沒動」與
		// 「一路上漲、沒有任何下跌」在這裡是同一個狀態。但它們的語意相反：
		// 前者是中性，後者才是極度強勢。回 100 會讓一檔完全沒成交波動的股票
		// 被當成超買（isRSIOverbought 會據此擋掉 BUY），也曾讓 rsi14 溢位
		// （2026-09-01 的 2454）。邊界語意的現況規格見 docs/indicator-spec.md 的 RSI 段。
		if avgGain == 0 {
			return 50
		}
		return 100
	}
	rs := avgGain / avgLoss
	return 100 - (100 / (1 + rs))
}
