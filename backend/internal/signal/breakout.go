package signal

import (
	"fmt"
	"time"

	"github.com/trading/backend/internal/store"
)

const (
	breakoutVolThresh = 2.0 // 爆量門檻：成交量需超過 MA20 的 2 倍
	volSpikeThresh    = 3.0 // 純爆量警示門檻
)

// CheckBreakout 依據當前指標和 S/R 判斷是否觸發訊號
func CheckBreakout(
	symbol string,
	snap *store.IndicatorSnapshot,
	latestCandle store.Candle,
	resistances, supports []Level,
	trend TrendState,
) *store.Signal {
	price := latestCandle.Close
	vol := latestCandle.Volume
	volRatio := snap.VolRatio
	ts := latestCandle.Timestamp

	// 突破訊號：收盤 > 阻力 + 爆量 + 多頭結構
	for _, r := range resistances {
		if price > r.Price && volRatio >= breakoutVolThresh && trend == Bullish {
			return &store.Signal{
				Symbol:     symbol,
				SignalType: "BREAKOUT",
				Direction:  "BUY",
				Price:      price,
				Volume:     vol,
				VolRatio:   volRatio,
				Resistance: r.Price,
				Trend:      string(trend),
				Strength:   1.0,
				Note:       fmt.Sprintf("突破阻力 %.2f，量比 %.2fx", r.Price, volRatio),
				Timestamp:  ts,
			}
		}
	}

	// 跌破訊號：收盤 < 支撐 + 空頭結構
	for _, s := range supports {
		if price < s.Price && trend == Bearish {
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
				Note:       fmt.Sprintf("跌破支撐 %.2f", s.Price),
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
			Timestamp:  time.Now(),
		}
	}

	return nil
}
