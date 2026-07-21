package signal

import "github.com/trading/backend/internal/store"

type extremePoint struct {
	Price float64
	Index int
}

func findLocalHighs(candles []store.Candle) []extremePoint {
	var points []extremePoint
	for i := 1; i < len(candles)-1; {
		price := candles[i].High
		end := i
		for end+1 < len(candles)-1 && candles[end+1].High == price {
			end++
		}
		if price > candles[i-1].High && price > candles[end+1].High {
			points = append(points, extremePoint{Price: price, Index: (i + end) / 2})
		}
		i = end + 1
	}
	return points
}

func findLocalLows(candles []store.Candle) []extremePoint {
	var points []extremePoint
	for i := 1; i < len(candles)-1; {
		price := candles[i].Low
		end := i
		for end+1 < len(candles)-1 && candles[end+1].Low == price {
			end++
		}
		if price < candles[i-1].Low && price < candles[end+1].Low {
			points = append(points, extremePoint{Price: price, Index: (i + end) / 2})
		}
		i = end + 1
	}
	return points
}
