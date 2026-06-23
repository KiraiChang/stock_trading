package indicator

// CalcVWAP 計算成交量加權平均價
// TypicalPrice = (High + Low + Close) / 3
func CalcVWAP(highs, lows, closes []float64, volumes []int64) float64 {
	var totalPV, totalV float64
	for i := range closes {
		tp := (highs[i] + lows[i] + closes[i]) / 3
		v := float64(volumes[i])
		totalPV += tp * v
		totalV += v
	}
	if totalV == 0 {
		return 0
	}
	return totalPV / totalV
}
