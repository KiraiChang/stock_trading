package indicator

func CalcMA(closes []float64, period int) float64 {
	if len(closes) < period {
		return 0
	}
	slice := closes[len(closes)-period:]
	sum := 0.0
	for _, v := range slice {
		sum += v
	}
	return sum / float64(period)
}

// RollingMA 用上一個週期的 sum 做滾動更新，避免全量掃描
func RollingMA(prevSum, oldClose, newClose float64, period int) float64 {
	return (prevSum - oldClose + newClose) / float64(period)
}
