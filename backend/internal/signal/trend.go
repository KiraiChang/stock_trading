package signal

import "github.com/trading/backend/internal/store"

type TrendState string

const (
	Bullish  TrendState = "BULLISH"
	Bearish  TrendState = "BEARISH"
	Sideways TrendState = "SIDEWAYS"
)

type swingPoint struct {
	Price float64
	Index int
}

// DetectTrend 識別市場結構（HH/HL → Bullish，LH/LL → Bearish）
// candles 需按時間升冪排列
func DetectTrend(candles []store.Candle) TrendState {
	if len(candles) < 10 {
		return Sideways
	}

	swingHighs := findSwingHighs(candles)
	swingLows := findSwingLows(candles)

	if len(swingHighs) < 2 || len(swingLows) < 2 {
		return Sideways
	}

	lastH1 := swingHighs[len(swingHighs)-1].Price
	lastH2 := swingHighs[len(swingHighs)-2].Price
	lastL1 := swingLows[len(swingLows)-1].Price
	lastL2 := swingLows[len(swingLows)-2].Price

	// HH + HL → Bullish
	if lastH1 > lastH2 && lastL1 > lastL2 {
		return Bullish
	}
	// LH + LL → Bearish
	if lastH1 < lastH2 && lastL1 < lastL2 {
		return Bearish
	}
	return Sideways
}

// findSwingHighs 找出 Swing High（window=3：左右各一根確認）
func findSwingHighs(candles []store.Candle) []swingPoint {
	var points []swingPoint
	for i := 1; i < len(candles)-1; i++ {
		if candles[i].High > candles[i-1].High && candles[i].High > candles[i+1].High {
			points = append(points, swingPoint{Price: candles[i].High, Index: i})
		}
	}
	return points
}

// findSwingLows 找出 Swing Low
func findSwingLows(candles []store.Candle) []swingPoint {
	var points []swingPoint
	for i := 1; i < len(candles)-1; i++ {
		if candles[i].Low < candles[i-1].Low && candles[i].Low < candles[i+1].Low {
			points = append(points, swingPoint{Price: candles[i].Low, Index: i})
		}
	}
	return points
}
