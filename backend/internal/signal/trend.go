package signal

import "github.com/trading/backend/internal/store"

type TrendState string

const (
	Bullish  TrendState = "BULLISH"
	Bearish  TrendState = "BEARISH"
	Sideways TrendState = "SIDEWAYS"
)

// DetectTrend 識別市場結構（HH/HL → Bullish，LH/LL → Bearish）
// candles 需按時間升冪排列
func DetectTrend(candles []store.Candle) TrendState {
	if len(candles) < 10 {
		return Sideways
	}

	swingHighs := findLocalHighs(candles)
	swingLows := findLocalLows(candles)

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
