package indicator

import "math"

type BollingerResult struct {
	Upper  float64
	Middle float64
	Lower  float64
}

// CalcBollinger 計算布林通道，標準參數：period=20, multiplier=2.0
func CalcBollinger(closes []float64, period int, multiplier float64) BollingerResult {
	if len(closes) < period {
		return BollingerResult{}
	}

	slice := closes[len(closes)-period:]
	sum := 0.0
	for _, v := range slice {
		sum += v
	}
	mean := sum / float64(period)

	varSum := 0.0
	for _, v := range slice {
		diff := v - mean
		varSum += diff * diff
	}
	stddev := math.Sqrt(varSum / float64(period))

	return BollingerResult{
		Upper:  mean + multiplier*stddev,
		Middle: mean,
		Lower:  mean - multiplier*stddev,
	}
}
