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

func checkBreakoutWithCandles(
	snap *store.IndicatorSnapshot,
	candles []store.Candle,
	resistances, supports []Level,
	trend TrendState,
) *store.Signal {
	return CheckBreakout("2330", snap, candles, resistances, supports, trend)
}

func TestCheckBreakout_TriggersBuyOnBreakoutWithVolumeAndBullishTrend(t *testing.T) {
	ts := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	previous := makeCandle(95, 1_000_000, ts.Add(-24*time.Hour))
	candle := makeCandle(110, 5_000_000, ts)
	snap := &store.IndicatorSnapshot{VolRatio: 2.5}
	resistances := []Level{{Price: 100, Strength: 1.0, Type: "Resistance"}}

	sig := checkBreakoutWithCandles(snap, []store.Candle{previous, candle}, resistances, nil, Bullish)

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
	previous := makeCandle(95, 1_000_000, time.Now().Add(-time.Hour))
	candle := makeCandle(110, 1_200_000, time.Now())
	snap := &store.IndicatorSnapshot{VolRatio: 1.5} // < breakoutVolThresh(2.0)
	resistances := []Level{{Price: 100, Strength: 1.0, Type: "Resistance"}}

	sig := checkBreakoutWithCandles(snap, []store.Candle{previous, candle}, resistances, nil, Bullish)
	if sig != nil {
		t.Errorf("expected nil (volume too low), got %+v", sig)
	}
}

func TestCheckBreakout_NoSignalWhenTrendNotBullish(t *testing.T) {
	previous := makeCandle(95, 1_000_000, time.Now().Add(-time.Hour))
	candle := makeCandle(110, 5_000_000, time.Now())
	snap := &store.IndicatorSnapshot{VolRatio: 2.5} // 夠爆量，但故意讓量比 < 3.0 避免誤觸發 VOLUME_SPIKE
	resistances := []Level{{Price: 100, Strength: 1.0, Type: "Resistance"}}

	sig := checkBreakoutWithCandles(snap, []store.Candle{previous, candle}, resistances, nil, Sideways)
	if sig != nil {
		t.Errorf("expected nil (trend not BULLISH), got %+v", sig)
	}
}

func TestCheckBreakout_NoSignalWhenPriceBelowResistance(t *testing.T) {
	previous := makeCandle(90, 1_000_000, time.Now().Add(-time.Hour))
	candle := makeCandle(95, 5_000_000, time.Now())
	snap := &store.IndicatorSnapshot{VolRatio: 2.5}
	resistances := []Level{{Price: 100, Strength: 1.0, Type: "Resistance"}}

	sig := checkBreakoutWithCandles(snap, []store.Candle{previous, candle}, resistances, nil, Bullish)
	if sig != nil {
		t.Errorf("expected nil (price hasn't broken resistance), got %+v", sig)
	}
}

func TestCheckBreakout_NoSignalWhenPriceWasAlreadyAboveResistance(t *testing.T) {
	previous := makeCandle(105, 1_000_000, time.Now().Add(-time.Hour))
	candle := makeCandle(110, 5_000_000, time.Now())
	snap := &store.IndicatorSnapshot{VolRatio: 2.5}
	resistances := []Level{{Price: 100, Strength: 1.0, Type: "Resistance"}}

	sig := checkBreakoutWithCandles(snap, []store.Candle{previous, candle}, resistances, nil, Bullish)
	if sig != nil {
		t.Errorf("expected nil (resistance was already crossed before latest candle), got %+v", sig)
	}
}

func TestCheckBreakout_UsesNearestCrossedResistance(t *testing.T) {
	candle := makeCandle(200, 5_000_000, time.Now())
	previous := makeCandle(90, 1_000_000, candle.Timestamp.Add(-time.Hour))
	snap := &store.IndicatorSnapshot{VolRatio: 2.5}
	resistances := []Level{
		{Price: 100, Strength: 1.0, Type: "Resistance"},
		{Price: 150, Strength: 0.5, Type: "Resistance"},
	}

	sig := checkBreakoutWithCandles(snap, []store.Candle{previous, candle}, resistances, nil, Bullish)
	if sig == nil {
		t.Fatal("expected a signal")
	}
	if sig.Resistance != 150 {
		t.Errorf("Resistance = %v, want 150 (nearest crossed resistance below close)", sig.Resistance)
	}
}

func TestCheckBreakout_TriggersSellAfterBreakdownConfirmation(t *testing.T) {
	ts := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	beforeBreak := makeCandle(105, 1_000_000, ts.AddDate(0, 0, -3))
	breakCandle := makeCandle(95, 800_000, ts.AddDate(0, 0, -2))
	firstConfirm := makeCandle(94, 700_000, ts.AddDate(0, 0, -1))
	secondConfirm := makeCandle(90, 600_000, ts)
	snap := &store.IndicatorSnapshot{VolRatio: 0.8} // 跌破不要求爆量
	supports := []Level{{Price: 100, Strength: 1.0, Type: "Support"}}

	sig := checkBreakoutWithCandles(
		snap,
		[]store.Candle{beforeBreak, breakCandle, firstConfirm, secondConfirm},
		nil,
		supports,
		Bearish,
	)

	if sig == nil {
		t.Fatal("expected a BREAKDOWN signal, got nil")
	}
	if sig.SignalType != "BREAKDOWN" || sig.Direction != "SELL" {
		t.Errorf("got SignalType=%s Direction=%s, want BREAKDOWN/SELL", sig.SignalType, sig.Direction)
	}
	if sig.Support != 100 {
		t.Errorf("Support = %v, want 100", sig.Support)
	}
	if sig.Timestamp != ts {
		t.Errorf("Timestamp = %v, want confirmation candle timestamp %v", sig.Timestamp, ts)
	}
	if !strings.Contains(sig.Note, "跌破支撐") {
		t.Errorf("Note = %q, expected to mention 跌破支撐", sig.Note)
	}
}

func TestCheckBreakout_NoBreakdownSignalOnBreakCandle(t *testing.T) {
	ts := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	beforeBreak := makeCandle(105, 1_000_000, ts.AddDate(0, 0, -1))
	breakCandle := makeCandle(95, 800_000, ts)
	snap := &store.IndicatorSnapshot{VolRatio: 0.8}
	supports := []Level{{Price: 100, Strength: 1.0, Type: "Support"}}

	sig := checkBreakoutWithCandles(snap, []store.Candle{beforeBreak, breakCandle}, nil, supports, Bearish)
	if sig != nil {
		t.Errorf("expected nil on breakdown candle before confirmation window, got %+v", sig)
	}
}

func TestCheckBreakout_NoBreakdownSignalOnFirstConfirmationCandle(t *testing.T) {
	ts := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	beforeBreak := makeCandle(105, 1_000_000, ts.AddDate(0, 0, -2))
	breakCandle := makeCandle(95, 800_000, ts.AddDate(0, 0, -1))
	firstConfirm := makeCandle(94, 700_000, ts)
	snap := &store.IndicatorSnapshot{VolRatio: 0.8}
	supports := []Level{{Price: 100, Strength: 1.0, Type: "Support"}}

	sig := checkBreakoutWithCandles(snap, []store.Candle{beforeBreak, breakCandle, firstConfirm}, nil, supports, Bearish)
	if sig != nil {
		t.Errorf("expected nil on first confirmation candle, got %+v", sig)
	}
}

func TestCheckBreakout_NoBreakdownSignalWhenSupportRecoveredDuringConfirmation(t *testing.T) {
	ts := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	beforeBreak := makeCandle(105, 1_000_000, ts.AddDate(0, 0, -3))
	breakCandle := makeCandle(95, 800_000, ts.AddDate(0, 0, -2))
	recovered := makeCandle(101, 700_000, ts.AddDate(0, 0, -1))
	secondConfirm := makeCandle(90, 600_000, ts)
	snap := &store.IndicatorSnapshot{VolRatio: 0.8}
	supports := []Level{{Price: 100, Strength: 1.0, Type: "Support"}}

	sig := checkBreakoutWithCandles(
		snap,
		[]store.Candle{beforeBreak, breakCandle, recovered, secondConfirm},
		nil,
		supports,
		Bearish,
	)
	if sig != nil {
		t.Errorf("expected nil when support was recovered during confirmation window, got %+v", sig)
	}
}

func TestCheckBreakout_NoSignalWhenTrendNotBearish(t *testing.T) {
	ts := time.Now()
	beforeBreak := makeCandle(105, 1_000_000, ts.AddDate(0, 0, -3))
	breakCandle := makeCandle(95, 800_000, ts.AddDate(0, 0, -2))
	firstConfirm := makeCandle(94, 700_000, ts.AddDate(0, 0, -1))
	secondConfirm := makeCandle(90, 600_000, ts)
	snap := &store.IndicatorSnapshot{VolRatio: 0.8}
	supports := []Level{{Price: 100, Strength: 1.0, Type: "Support"}}

	sig := checkBreakoutWithCandles(
		snap,
		[]store.Candle{beforeBreak, breakCandle, firstConfirm, secondConfirm},
		nil,
		supports,
		Sideways,
	)
	if sig != nil {
		t.Errorf("expected nil (trend not BEARISH), got %+v", sig)
	}
}

func TestCheckBreakout_UsesNearestCrossedSupport(t *testing.T) {
	ts := time.Now()
	beforeBreak := makeCandle(120, 1_000_000, ts.AddDate(0, 0, -3))
	breakCandle := makeCandle(80, 800_000, ts.AddDate(0, 0, -2))
	firstConfirm := makeCandle(79, 700_000, ts.AddDate(0, 0, -1))
	secondConfirm := makeCandle(78, 600_000, ts)
	snap := &store.IndicatorSnapshot{VolRatio: 0.8}
	supports := []Level{
		{Price: 100, Strength: 1.0, Type: "Support"},
		{Price: 90, Strength: 0.5, Type: "Support"},
	}

	sig := checkBreakoutWithCandles(
		snap,
		[]store.Candle{beforeBreak, breakCandle, firstConfirm, secondConfirm},
		nil,
		supports,
		Bearish,
	)
	if sig == nil {
		t.Fatal("expected a signal")
	}
	if sig.Support != 90 {
		t.Errorf("Support = %v, want 90 (nearest crossed support above close)", sig.Support)
	}
}

func TestCheckBreakout_VolumeSpikeAloneWhenNoDirectionalCondition(t *testing.T) {
	ts := time.Date(2026, 1, 15, 10, 5, 0, 0, time.UTC)
	previous := makeCandle(100, 1_000_000, ts.Add(-time.Minute))
	candle := makeCandle(100, 9_000_000, ts)
	snap := &store.IndicatorSnapshot{VolRatio: 3.5} // >= volSpikeThresh(3.0)

	sig := checkBreakoutWithCandles(snap, []store.Candle{previous, candle}, nil, nil, Sideways)

	if sig == nil {
		t.Fatal("expected a VOLUME_SPIKE signal, got nil")
	}
	if sig.SignalType != "VOLUME_SPIKE" || sig.Direction != "WATCH" {
		t.Errorf("got SignalType=%s Direction=%s, want VOLUME_SPIKE/WATCH", sig.SignalType, sig.Direction)
	}
	if sig.Timestamp != ts {
		t.Errorf("VOLUME_SPIKE Timestamp = %v, want latest candle timestamp %v", sig.Timestamp, ts)
	}
}

func TestCheckBreakout_BreakoutTakesPriorityOverVolumeSpike(t *testing.T) {
	// 量比同時滿足 breakout(>=2.0) 與 volume spike(>=3.0) 門檻時，
	// 因為程式先檢查 resistances 再檢查 supports 最後才檢查純爆量，
	// 應該回傳 BREAKOUT 而不是 VOLUME_SPIKE
	candle := makeCandle(110, 9_000_000, time.Now())
	previous := makeCandle(95, 1_000_000, candle.Timestamp.Add(-time.Hour))
	snap := &store.IndicatorSnapshot{VolRatio: 3.5}
	resistances := []Level{{Price: 100, Strength: 1.0, Type: "Resistance"}}

	sig := checkBreakoutWithCandles(snap, []store.Candle{previous, candle}, resistances, nil, Bullish)
	if sig == nil {
		t.Fatal("expected a signal")
	}
	if sig.SignalType != "BREAKOUT" {
		t.Errorf("SignalType = %s, want BREAKOUT to take priority over VOLUME_SPIKE", sig.SignalType)
	}
}

func TestCheckBreakout_NoConditionsMetReturnsNil(t *testing.T) {
	previous := makeCandle(100, 1_000_000, time.Now().Add(-time.Hour))
	candle := makeCandle(100, 1_000_000, time.Now())
	snap := &store.IndicatorSnapshot{VolRatio: 1.0}

	sig := checkBreakoutWithCandles(snap, []store.Candle{previous, candle}, nil, nil, Sideways)
	if sig != nil {
		t.Errorf("expected nil, got %+v", sig)
	}
}
