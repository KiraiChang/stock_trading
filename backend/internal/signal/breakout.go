package signal

import (
	"fmt"

	"github.com/trading/backend/internal/store"
)

const (
	breakoutVolThresh             = 2.0 // 爆量門檻：成交量需超過 MA20 的 2 倍
	volSpikeThresh                = 3.0 // 純爆量警示門檻
	breakoutConfirmationCandles   = 2   // 突破後連續 2 根 K 棒站穩才確認
	breakdownConfirmationCandles  = 2   // 跌破後連續 2 根 K 棒未收回才確認
	breakoutMinCandlesForConfirm  = breakoutConfirmationCandles + 21
	breakdownMinCandlesForConfirm = breakdownConfirmationCandles + 2
	volumeMAPeriod                = 20
	rsiOverboughtThresh           = 80.0
)

// CheckBreakout 依據當前指標和 S/R 判斷是否觸發訊號
func CheckBreakout(
	symbol string,
	snap *store.IndicatorSnapshot,
	candles []store.Candle,
	resistances, supports []Level,
	trend TrendState,
) *store.Signal {
	if len(candles) == 0 {
		return nil
	}
	latestCandle := candles[len(candles)-1]
	price := latestCandle.Close
	vol := latestCandle.Volume
	volRatio := snap.VolRatio
	ts := latestCandle.Timestamp

	// 突破訊號：收盤 > 阻力 + 爆量 + 多頭結構 + 連續確認
	if trend == Bullish && !isRSIOverbought(snap) {
		if r, breakVolRatio, breakVolume, ok := confirmedBreakoutResistance(candles, resistances); ok {
			return &store.Signal{
				Symbol:     symbol,
				SignalType: "BREAKOUT",
				Direction:  "BUY",
				Price:      price,
				Volume:     breakVolume,
				VolRatio:   breakVolRatio,
				Resistance: r.Price,
				Trend:      string(trend),
				Strength:   1.0,
				Note:       fmt.Sprintf("突破阻力 %.2f，量比 %.2fx，連續 %d 根站穩", r.Price, breakVolRatio, breakoutConfirmationCandles),
				Timestamp:  ts,
			}
		}
	}

	// 跌破訊號：收盤 < 支撐 + 空頭結構
	if trend == Bearish {
		if s, ok := confirmedBreakdownSupport(candles, supports); ok {
			return &store.Signal{
				Symbol:     symbol,
				SignalType: "BREAKDOWN",
				Direction:  "SELL",
				Price:      price,
				Volume:     vol,
				VolRatio:   volRatio,
				Support:    s.Price,
				Trend:      string(trend),
				Strength:   1.0,
				Note:       fmt.Sprintf("跌破支撐 %.2f，連續 %d 根未收回", s.Price, breakdownConfirmationCandles),
				Timestamp:  ts,
			}
		}
	}

	// 純爆量警示（無需方向條件）
	if volRatio >= volSpikeThresh {
		return &store.Signal{
			Symbol:     symbol,
			SignalType: "VOLUME_SPIKE",
			Direction:  "WATCH",
			Price:      price,
			Volume:     vol,
			VolRatio:   volRatio,
			Trend:      string(trend),
			Strength:   1.0,
			Note:       fmt.Sprintf("異常爆量 %.2fx", volRatio),
			Timestamp:  ts,
		}
	}

	return nil
}

func isRSIOverbought(snap *store.IndicatorSnapshot) bool {
	return snap != nil && snap.RSI14 > 0 && snap.RSI14 >= rsiOverboughtThresh
}

func confirmedBreakoutResistance(candles []store.Candle, resistances []Level) (Level, float64, int64, bool) {
	if len(candles) < breakoutMinCandlesForConfirm {
		return Level{}, 0, 0, false
	}

	breakIdx := len(candles) - 1 - breakoutConfirmationCandles
	beforeBreakClose := candles[breakIdx-1].Close
	breakClose := candles[breakIdx].Close
	latestClose := candles[len(candles)-1].Close
	breakVolRatio := volumeRatioAt(candles, breakIdx, volumeMAPeriod)
	if breakVolRatio < breakoutVolThresh {
		return Level{}, 0, 0, false
	}

	candidates := make([]Level, 0, len(resistances))
	for _, r := range resistances {
		if beforeBreakClose <= r.Price && breakClose > r.Price {
			failed := false
			for i := breakIdx + 1; i < len(candles); i++ {
				if candles[i].Close <= r.Price {
					failed = true
					break
				}
			}
			if !failed {
				candidates = append(candidates, r)
			}
		}
	}

	r, ok := nearestCrossedResistance(beforeBreakClose, latestClose, candidates)
	return r, breakVolRatio, candles[breakIdx].Volume, ok
}

func volumeRatioAt(candles []store.Candle, idx int, period int) float64 {
	if idx < period || idx >= len(candles) {
		return 0
	}

	var sum int64
	for i := idx - period; i < idx; i++ {
		sum += candles[i].Volume
	}
	ma := float64(sum) / float64(period)
	if ma <= 0 {
		return 0
	}
	return float64(candles[idx].Volume) / ma
}

func nearestCrossedResistance(previousClose, latestClose float64, resistances []Level) (Level, bool) {
	var best Level
	found := false
	for _, r := range resistances {
		if previousClose <= r.Price && latestClose > r.Price {
			if !found || r.Price > best.Price || (r.Price == best.Price && r.Strength > best.Strength) {
				best = r
				found = true
			}
		}
	}
	return best, found
}

func confirmedBreakdownSupport(candles []store.Candle, supports []Level) (Level, bool) {
	if len(candles) < breakdownMinCandlesForConfirm {
		return Level{}, false
	}

	breakIdx := len(candles) - 1 - breakdownConfirmationCandles
	beforeBreakClose := candles[breakIdx-1].Close
	breakClose := candles[breakIdx].Close
	latestClose := candles[len(candles)-1].Close

	candidates := make([]Level, 0, len(supports))
	for _, s := range supports {
		if beforeBreakClose >= s.Price && breakClose < s.Price {
			recovered := false
			for i := breakIdx + 1; i < len(candles); i++ {
				if candles[i].Close >= s.Price {
					recovered = true
					break
				}
			}
			if !recovered {
				candidates = append(candidates, s)
			}
		}
	}
	return nearestCrossedSupport(beforeBreakClose, latestClose, candidates)
}

func nearestCrossedSupport(previousClose, latestClose float64, supports []Level) (Level, bool) {
	var best Level
	found := false
	for _, s := range supports {
		if previousClose >= s.Price && latestClose < s.Price {
			if !found || s.Price < best.Price || (s.Price == best.Price && s.Strength > best.Strength) {
				best = s
				found = true
			}
		}
	}
	return best, found
}
