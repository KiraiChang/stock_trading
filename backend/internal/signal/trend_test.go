package signal

import (
	"testing"
	"time"

	"github.com/trading/backend/internal/store"
)

// makeTrendCandles 依 highs/lows 建立按時間升冪排列的合成K棒，僅供趨勢/支撐壓力
// 測試使用；Open/Close 用高低點中間值即可，這兩個函式都不看 Open/Close。
func makeTrendCandles(highs, lows []float64) []store.Candle {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := make([]store.Candle, len(highs))
	for i := range highs {
		mid := (highs[i] + lows[i]) / 2
		candles[i] = store.Candle{
			Symbol:    "TEST",
			Timeframe: "1d",
			Open:      mid,
			High:      highs[i],
			Low:       lows[i],
			Close:     mid,
			Volume:    1000,
			Timestamp: base.AddDate(0, 0, i),
		}
	}
	return candles
}

func TestDetectTrend_TooFewCandles(t *testing.T) {
	highs := []float64{100, 101, 102}
	lows := []float64{95, 96, 97}
	if got := DetectTrend(makeTrendCandles(highs, lows)); got != Sideways {
		t.Errorf("expected Sideways for <10 candles, got %v", got)
	}
}

func TestDetectTrend_InsufficientSwingPoints(t *testing.T) {
	// 單調上升，沒有任何 local high/low，找不到 swing 點（左右各一根都無法確認）
	highs := []float64{90, 92, 94, 96, 98, 100, 102, 104, 106, 108}
	lows := []float64{85, 87, 89, 91, 93, 95, 97, 99, 101, 103}
	if got := DetectTrend(makeTrendCandles(highs, lows)); got != Sideways {
		t.Errorf("expected Sideways when swing points insufficient, got %v", got)
	}
}

func TestDetectTrend_Bullish(t *testing.T) {
	// 兩段上升 zigzag：谷1(90) → 峰1(105) → 谷2(95，比谷1高→HL) → 峰2(123，比峰1高→HH)
	base := []float64{90, 100, 95, 108, 102, 118, 110, 100, 95, 90}
	lows := base
	highs := make([]float64, len(base))
	for i, b := range base {
		highs[i] = b + 5
	}
	if got := DetectTrend(makeTrendCandles(highs, lows)); got != Bullish {
		t.Errorf("expected Bullish, got %v", got)
	}
}

func TestDetectTrend_Bearish(t *testing.T) {
	// 上面 Bullish 測試序列的時間反轉：兩段下降 zigzag（LH + LL）
	reversed := []float64{90, 95, 100, 110, 118, 102, 108, 95, 100, 90}
	lows := reversed
	highs := make([]float64, len(reversed))
	for i, b := range reversed {
		highs[i] = b + 5
	}
	if got := DetectTrend(makeTrendCandles(highs, lows)); got != Bearish {
		t.Errorf("expected Bearish, got %v", got)
	}
}

func TestDetectTrend_MixedStructureIsSideways(t *testing.T) {
	// 壓力墊高（HH）但支撐同時破底（LL）→ 結構矛盾，判定為盤整
	highs := []float64{95, 105, 100, 113, 107, 123, 115, 105, 100, 95}
	lows := []float64{90, 80, 85, 70, 75, 60, 65, 70, 68, 66}
	if got := DetectTrend(makeTrendCandles(highs, lows)); got != Sideways {
		t.Errorf("expected Sideways for mixed HH+LL structure, got %v", got)
	}
}
