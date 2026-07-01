package signal

import (
	"math"
	"testing"
)

func TestBuildLevels_EmptyCandidatesReturnsNil(t *testing.T) {
	if got := buildLevels(nil, "Resistance", nil); got != nil {
		t.Errorf("expected nil for empty candidates, got %v", got)
	}
}

func TestBuildLevels_ClustersMergesWithinOnePercent(t *testing.T) {
	// 4 個群集，觸碰次數分別為 4/3/2/1，價位彼此相距遠（>1%），群內差距 <1%
	candidates := []float64{
		100, 100.1, 99.9, 100.05, // cluster A：4 次觸碰，中心 ~100.0125
		200, 200.1, 199.9, // cluster B：3 次觸碰，中心 200.0
		300, 300.1, // cluster C：2 次觸碰，中心 300.05
		400, // cluster D：1 次觸碰，強度最低，maxLevels=3 應該被砍掉
	}

	levels := buildLevels(candidates, "Resistance", nil)

	if len(levels) != maxLevels {
		t.Fatalf("expected %d levels (truncated), got %d: %+v", maxLevels, len(levels), levels)
	}

	// 依 strength 由高到低：A(1.0) > B(0.75) > C(0.5)，D(0.25) 應該被排除
	wantPrices := []float64{100.0125, 200.0, 300.05}
	wantStrengths := []float64{1.0, 0.75, 0.5}
	for i, lv := range levels {
		if math.Abs(lv.Price-wantPrices[i]) > 0.001 {
			t.Errorf("level[%d].Price = %v, want %v", i, lv.Price, wantPrices[i])
		}
		if math.Abs(lv.Strength-wantStrengths[i]) > 0.001 {
			t.Errorf("level[%d].Strength = %v, want %v", i, lv.Strength, wantStrengths[i])
		}
		if lv.Type != "Resistance" {
			t.Errorf("level[%d].Type = %v, want Resistance", i, lv.Type)
		}
	}

	for _, p := range []float64{400} {
		for _, lv := range levels {
			if math.Abs(lv.Price-p) < 1 {
				t.Errorf("weakest cluster (price ~%v) should have been truncated, found in result", p)
			}
		}
	}
}

func TestBuildLevels_SinglePriceHasFullStrength(t *testing.T) {
	levels := buildLevels([]float64{150}, "Support", nil)
	if len(levels) != 1 {
		t.Fatalf("expected 1 level, got %d", len(levels))
	}
	if levels[0].Price != 150 || levels[0].Strength != 1.0 || levels[0].Type != "Support" {
		t.Errorf("unexpected level: %+v", levels[0])
	}
}

func TestCalcSupportResistance_TooFewCandles(t *testing.T) {
	highs := []float64{100, 101, 102}
	lows := []float64{95, 96, 97}
	supports, resistances := CalcSupportResistance(makeTrendCandles(highs, lows))
	if supports != nil || resistances != nil {
		t.Errorf("expected (nil, nil) for <10 candles, got supports=%v resistances=%v", supports, resistances)
	}
}

func TestCalcSupportResistance_DetectsZigzagPivots(t *testing.T) {
	// 沿用 TestDetectTrend_Bullish 的 zigzag：兩個 swing high（105,123 皆 +5 偏移
	// 後為 110,128... 這裡直接用 base+5 產生 highs，兩個 swing low 在 base 本身）
	base := []float64{90, 100, 95, 108, 102, 118, 110, 100, 95, 90}
	lows := base
	highs := make([]float64, len(base))
	for i, b := range base {
		highs[i] = b + 5
	}
	candles := makeTrendCandles(highs, lows)

	supports, resistances := CalcSupportResistance(candles)

	assertContainsPrice(t, resistances, 105) // 100+5 峰1
	assertContainsPrice(t, resistances, 123) // 118+5 峰2
	assertContainsPrice(t, supports, 95)     // 谷1
	assertContainsPrice(t, supports, 102)    // 谷2
}

func assertContainsPrice(t *testing.T, levels []Level, price float64) {
	t.Helper()
	for _, lv := range levels {
		if math.Abs(lv.Price-price) < 0.01 {
			return
		}
	}
	t.Errorf("expected a level at price %v, got %+v", price, levels)
}
