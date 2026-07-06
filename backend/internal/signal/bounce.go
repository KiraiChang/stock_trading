package signal

import (
	"fmt"

	"github.com/trading/backend/internal/store"
)

// supportBounceTolerancePct 定義「貼近支撐」的容許帶：最新K棒的最低點要落在
// 支撐價位 ±1% 內，才視為真的測試過這個支撐（而不是隨便一根跌很深或完全
// 沒碰到支撐的K棒）。
const supportBounceTolerancePct = 0.01

// CheckSupportBounce 偵測「價格貼近支撐後反彈」：最新K棒最低點觸及支撐
// （容許帶內），但收盤重新站回支撐之上，且收在當日振幅的上半部（長下影線
// /類鐵鎚線，代表買盤在支撐處承接）。不要求爆量或多頭結構——這是比
// BREAKOUT/BREAKDOWN 更早期、方向性較弱的觀察訊號，只在 CheckBreakout
// 沒有觸發任何訊號時才由 Engine.Evaluate 呼叫（見該檔案），實務上通常會
// 再搭配籌碼面判斷是否提高觀察優先度。
func CheckSupportBounce(symbol string, snap *store.IndicatorSnapshot, latestCandle store.Candle, supports []Level) *store.Signal {
	price := latestCandle.Close
	low := latestCandle.Low
	high := latestCandle.High
	rangeSize := high - low
	if rangeSize <= 0 {
		return nil
	}

	for _, s := range supports {
		band := s.Price * supportBounceTolerancePct
		touchedSupport := low >= s.Price-band && low <= s.Price+band
		if !touchedSupport {
			continue
		}
		closedAboveSupport := price > s.Price
		if !closedAboveSupport {
			continue
		}
		closedInUpperHalf := (price-low)/rangeSize >= 0.5
		if !closedInUpperHalf {
			continue
		}

		return &store.Signal{
			Symbol:     symbol,
			SignalType: "SUPPORT_BOUNCE",
			Direction:  "WATCH",
			Price:      price,
			Volume:     latestCandle.Volume,
			VolRatio:   snap.VolRatio,
			Support:    s.Price,
			Strength:   1.0,
			Note:       fmt.Sprintf("價格貼近支撐 %.2f 後反彈", s.Price),
			Timestamp:  latestCandle.Timestamp,
		}
	}
	return nil
}
