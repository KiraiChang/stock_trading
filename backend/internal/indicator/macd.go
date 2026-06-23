package indicator

type MACDResult struct {
	MACD      float64
	Signal    float64
	Histogram float64
}

// calcEMA 計算完整 EMA 序列
func calcEMA(closes []float64, period int) []float64 {
	if len(closes) < period {
		return nil
	}
	multiplier := 2.0 / float64(period+1)
	emas := make([]float64, len(closes))

	// 初始 EMA = 前 period 根的算術平均
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += closes[i]
	}
	emas[period-1] = sum / float64(period)

	for i := period; i < len(closes); i++ {
		emas[i] = closes[i]*multiplier + emas[i-1]*(1-multiplier)
	}
	return emas
}

// CalcMACD 計算 MACD line、Signal line、Histogram
// 標準參數：fast=12, slow=26, signal=9
func CalcMACD(closes []float64, fast, slow, signalPeriod int) MACDResult {
	if len(closes) < slow+signalPeriod-1 {
		return MACDResult{}
	}

	fastEMA := calcEMA(closes, fast)
	slowEMA := calcEMA(closes, slow)
	if fastEMA == nil || slowEMA == nil {
		return MACDResult{}
	}

	// MACD line = fastEMA - slowEMA（從 slow-1 開始有效）
	macdLine := make([]float64, len(closes))
	for i := slow - 1; i < len(closes); i++ {
		macdLine[i] = fastEMA[i] - slowEMA[i]
	}

	// Signal line = EMA(macdLine, signalPeriod)
	// 只對 slow-1 之後的有效值計算
	validMACD := macdLine[slow-1:]
	multiplier := 2.0 / float64(signalPeriod+1)

	sum := 0.0
	for i := 0; i < signalPeriod; i++ {
		sum += validMACD[i]
	}
	sigVal := sum / float64(signalPeriod)
	for i := signalPeriod; i < len(validMACD); i++ {
		sigVal = validMACD[i]*multiplier + sigVal*(1-multiplier)
	}

	lastMACD := macdLine[len(macdLine)-1]
	return MACDResult{
		MACD:      lastMACD,
		Signal:    sigVal,
		Histogram: lastMACD - sigVal,
	}
}
