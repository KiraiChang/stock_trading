package signal

import (
	"strings"
	"testing"
	"time"

	"github.com/trading/backend/internal/store"
)

func makeCandle(closePrice float64, volume int64, ts time.Time) store.Candle {
	return store.Candle{
		Symbol: "2330", Timeframe: "1d",
		Open: closePrice, High: closePrice, Low: closePrice, Close: closePrice,
		Volume: volume, Timestamp: ts,
	}
}

func TestCheckBreakout_TriggersBuyOnBreakoutWithVolumeAndBullishTrend(t *testing.T) {
	ts := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	candle := makeCandle(110, 5_000_000, ts)
	snap := &store.IndicatorSnapshot{VolRatio: 2.5}
	resistances := []Level{{Price: 100, Strength: 1.0, Type: "Resistance"}}

	sig := CheckBreakout("2330", snap, candle, resistances, nil, Bullish)

	if sig == nil {
		t.Fatal("expected a BREAKOUT signal, got nil")
	}
	if sig.SignalType != "BREAKOUT" || sig.Direction != "BUY" {
		t.Errorf("got SignalType=%s Direction=%s, want BREAKOUT/BUY", sig.SignalType, sig.Direction)
	}
	if sig.Resistance != 100 {
		t.Errorf("Resistance = %v, want 100", sig.Resistance)
	}
	if sig.Price != 110 || sig.Volume != 5_000_000 || sig.VolRatio != 2.5 {
		t.Errorf("unexpected price/volume/volRatio: %+v", sig)
	}
	if sig.Timestamp != ts {
		t.Errorf("Timestamp = %v, want %v (應該用當根K棒時間)", sig.Timestamp, ts)
	}
	if !strings.Contains(sig.Note, "突破阻力") {
		t.Errorf("Note = %q, expected to mention 突破阻力", sig.Note)
	}
}

func TestCheckBreakout_NoSignalWhenVolumeInsufficient(t *testing.T) {
	candle := makeCandle(110, 1_200_000, time.Now())
	snap := &store.IndicatorSnapshot{VolRatio: 1.5} // < breakoutVolThresh(2.0)
	resistances := []Level{{Price: 100, Strength: 1.0, Type: "Resistance"}}

	sig := CheckBreakout("2330", snap, candle, resistances, nil, Bullish)
	if sig != nil {
		t.Errorf("expected nil (volume too low), got %+v", sig)
	}
}

func TestCheckBreakout_NoSignalWhenTrendNotBullish(t *testing.T) {
	candle := makeCandle(110, 5_000_000, time.Now())
	snap := &store.IndicatorSnapshot{VolRatio: 2.5} // 夠爆量，但故意讓量比 < 3.0 避免誤觸發 VOLUME_SPIKE
	resistances := []Level{{Price: 100, Strength: 1.0, Type: "Resistance"}}

	sig := CheckBreakout("2330", snap, candle, resistances, nil, Sideways)
	if sig != nil {
		t.Errorf("expected nil (trend not BULLISH), got %+v", sig)
	}
}

func TestCheckBreakout_NoSignalWhenPriceBelowResistance(t *testing.T) {
	candle := makeCandle(95, 5_000_000, time.Now())
	snap := &store.IndicatorSnapshot{VolRatio: 2.5}
	resistances := []Level{{Price: 100, Strength: 1.0, Type: "Resistance"}}

	sig := CheckBreakout("2330", snap, candle, resistances, nil, Bullish)
	if sig != nil {
		t.Errorf("expected nil (price hasn't broken resistance), got %+v", sig)
	}
}

func TestCheckBreakout_UsesFirstMatchingResistanceInSliceOrder(t *testing.T) {
	// 目前實作是逐一走訪 resistances、遇到第一個滿足條件的就回傳，
	// 呼叫端要自己決定排序（例如按 Strength 排序後再傳進來）
	candle := makeCandle(200, 5_000_000, time.Now())
	snap := &store.IndicatorSnapshot{VolRatio: 2.5}
	resistances := []Level{
		{Price: 150, Strength: 0.5, Type: "Resistance"},
		{Price: 100, Strength: 1.0, Type: "Resistance"},
	}

	sig := CheckBreakout("2330", snap, candle, resistances, nil, Bullish)
	if sig == nil {
		t.Fatal("expected a signal")
	}
	if sig.Resistance != 150 {
		t.Errorf("Resistance = %v, want 150 (第一個滿足條件的 level)", sig.Resistance)
	}
}

func TestCheckBreakout_TriggersSellOnBreakdownWithBearishTrend(t *testing.T) {
	ts := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	candle := makeCandle(90, 800_000, ts)
	snap := &store.IndicatorSnapshot{VolRatio: 0.8} // 跌破不要求爆量
	supports := []Level{{Price: 100, Strength: 1.0, Type: "Support"}}

	sig := CheckBreakout("2330", snap, candle, nil, supports, Bearish)

	if sig == nil {
		t.Fatal("expected a BREAKDOWN signal, got nil")
	}
	if sig.SignalType != "BREAKDOWN" || sig.Direction != "SELL" {
		t.Errorf("got SignalType=%s Direction=%s, want BREAKDOWN/SELL", sig.SignalType, sig.Direction)
	}
	if sig.Support != 100 {
		t.Errorf("Support = %v, want 100", sig.Support)
	}
	if !strings.Contains(sig.Note, "跌破支撐") {
		t.Errorf("Note = %q, expected to mention 跌破支撐", sig.Note)
	}
}

func TestCheckBreakout_NoSignalWhenTrendNotBearish(t *testing.T) {
	candle := makeCandle(90, 800_000, time.Now())
	snap := &store.IndicatorSnapshot{VolRatio: 0.8}
	supports := []Level{{Price: 100, Strength: 1.0, Type: "Support"}}

	sig := CheckBreakout("2330", snap, candle, nil, supports, Sideways)
	if sig != nil {
		t.Errorf("expected nil (trend not BEARISH), got %+v", sig)
	}
}

func TestCheckBreakout_VolumeSpikeAloneWhenNoDirectionalCondition(t *testing.T) {
	before := time.Now()
	candle := makeCandle(100, 9_000_000, time.Now().Add(-time.Hour)) // 這根K棒的時間刻意跟 time.Now() 不同
	snap := &store.IndicatorSnapshot{VolRatio: 3.5}                  // >= volSpikeThresh(3.0)

	sig := CheckBreakout("2330", snap, candle, nil, nil, Sideways)
	after := time.Now()

	if sig == nil {
		t.Fatal("expected a VOLUME_SPIKE signal, got nil")
	}
	if sig.SignalType != "VOLUME_SPIKE" || sig.Direction != "WATCH" {
		t.Errorf("got SignalType=%s Direction=%s, want VOLUME_SPIKE/WATCH", sig.SignalType, sig.Direction)
	}
	// 目前實作對 VOLUME_SPIKE 用 time.Now()，不是 latestCandle.Timestamp
	// （跟 BREAKOUT/BREAKDOWN 不一致，這裡把現況釘住，行為改變時測試會提醒）
	if sig.Timestamp.Before(before) || sig.Timestamp.After(after) {
		t.Errorf("VOLUME_SPIKE Timestamp = %v, expected to be time.Now() at call time (between %v and %v)", sig.Timestamp, before, after)
	}
}

func TestCheckBreakout_BreakoutTakesPriorityOverVolumeSpike(t *testing.T) {
	// 量比同時滿足 breakout(>=2.0) 與 volume spike(>=3.0) 門檻時，
	// 因為程式先檢查 resistances 再檢查 supports 最後才檢查純爆量，
	// 應該回傳 BREAKOUT 而不是 VOLUME_SPIKE
	candle := makeCandle(110, 9_000_000, time.Now())
	snap := &store.IndicatorSnapshot{VolRatio: 3.5}
	resistances := []Level{{Price: 100, Strength: 1.0, Type: "Resistance"}}

	sig := CheckBreakout("2330", snap, candle, resistances, nil, Bullish)
	if sig == nil {
		t.Fatal("expected a signal")
	}
	if sig.SignalType != "BREAKOUT" {
		t.Errorf("SignalType = %s, want BREAKOUT to take priority over VOLUME_SPIKE", sig.SignalType)
	}
}

func TestCheckBreakout_NoConditionsMetReturnsNil(t *testing.T) {
	candle := makeCandle(100, 1_000_000, time.Now())
	snap := &store.IndicatorSnapshot{VolRatio: 1.0}

	sig := CheckBreakout("2330", snap, candle, nil, nil, Sideways)
	if sig != nil {
		t.Errorf("expected nil, got %+v", sig)
	}
}
