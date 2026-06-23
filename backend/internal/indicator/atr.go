package indicator

import "math"

// CalcATR 使用 Wilder smoothing 計算 ATR
// candles 需按時間升冪排列
func CalcATR(highs, lows, closes []float64, period int) float64 {
	n := len(closes)
	if n < period+1 {
		return 0
	}

	trueRange := func(i int) float64 {
		if i == 0 {
			return highs[i] - lows[i]
		}
		hl := highs[i] - lows[i]
		hpc := math.Abs(highs[i] - closes[i-1])
		lpc := math.Abs(lows[i] - closes[i-1])
		return math.Max(hl, math.Max(hpc, lpc))
	}

	// 初始 ATR = 前 period 根的算術平均 TR
	sum := 0.0
	for i := 1; i <= period; i++ {
		sum += trueRange(i)
	}
	atr := sum / float64(period)

	// Wilder smoothing
	for i := period + 1; i < n; i++ {
		atr = (atr*float64(period-1) + trueRange(i)) / float64(period)
	}
	return atr
}
