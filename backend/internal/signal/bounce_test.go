package signal

import (
	"strings"
	"testing"
	"time"

	"github.com/trading/backend/internal/store"
)

func bounceCandle(open, high, low, close float64, volume int64, ts time.Time) store.Candle {
	return store.Candle{
		Symbol: "2330", Timeframe: "1d",
		Open: open, High: high, Low: low, Close: close,
		Volume: volume, Timestamp: ts,
	}
}

func TestCheckSupportBounce_TriggersWhenLowTouchesSupportAndClosesInUpperHalf(t *testing.T) {
	ts := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	// 支撐 100，容許帶 ±1（=100*0.01）。最低點 99.5 落在帶內，收盤 102 高於
	// 支撐，且收在當日振幅（99.5~103）的上半部。
	candle := bounceCandle(100.5, 103, 99.5, 102, 2_000_000, ts)
	snap := &store.IndicatorSnapshot{VolRatio: 1.2}
	supports := []Level{{Price: 100, Strength: 1.0, Type: "Support"}}

	sig := CheckSupportBounce("2330", snap, candle, supports)

	if sig == nil {
		t.Fatal("expected a SUPPORT_BOUNCE signal, got nil")
	}
	if sig.SignalType != "SUPPORT_BOUNCE" || sig.Direction != "WATCH" {
		t.Errorf("got SignalType=%s Direction=%s, want SUPPORT_BOUNCE/WATCH", sig.SignalType, sig.Direction)
	}
	if sig.Support != 100 {
		t.Errorf("Support = %v, want 100", sig.Support)
	}
	if sig.Strength != 1.0 {
		t.Errorf("Strength = %v, want 1.0 (base strength before chip weighting)", sig.Strength)
	}
	if !strings.Contains(sig.Note, "貼近支撐") {
		t.Errorf("Note = %q, expected to mention 貼近支撐", sig.Note)
	}
}

func TestCheckSupportBounce_NoSignalWhenLowDoesNotTouchSupport(t *testing.T) {
	candle := bounceCandle(110, 112, 108, 111, 2_000_000, time.Now())
	snap := &store.IndicatorSnapshot{VolRatio: 1.2}
	supports := []Level{{Price: 100, Strength: 1.0, Type: "Support"}}

	sig := CheckSupportBounce("2330", snap, candle, supports)
	if sig != nil {
		t.Errorf("expected nil (low never approached support), got %+v", sig)
	}
}

func TestCheckSupportBounce_NoSignalWhenCloseStillBelowSupport(t *testing.T) {
	// 最低點碰到支撐帶內，但收盤沒有收回支撐之上（沒有真的「反彈」）
	candle := bounceCandle(99, 100, 99.5, 99.2, 2_000_000, time.Now())
	snap := &store.IndicatorSnapshot{VolRatio: 1.2}
	supports := []Level{{Price: 100, Strength: 1.0, Type: "Support"}}

	sig := CheckSupportBounce("2330", snap, candle, supports)
	if sig != nil {
		t.Errorf("expected nil (close didn't recover above support), got %+v", sig)
	}
}

func TestCheckSupportBounce_NoSignalWhenClosedInLowerHalfOfRange(t *testing.T) {
	// 收盤雖然高於支撐，但收在當日振幅下半部（賣壓仍重，不是拒絕下跌的訊號）
	candle := bounceCandle(102, 104, 99.5, 100.2, 2_000_000, time.Now())
	snap := &store.IndicatorSnapshot{VolRatio: 1.2}
	supports := []Level{{Price: 100, Strength: 1.0, Type: "Support"}}

	sig := CheckSupportBounce("2330", snap, candle, supports)
	if sig != nil {
		t.Errorf("expected nil (closed in lower half of day's range), got %+v", sig)
	}
}

func TestCheckSupportBounce_NoSignalOnZeroRangeCandle(t *testing.T) {
	candle := bounceCandle(100, 100, 100, 100, 2_000_000, time.Now())
	snap := &store.IndicatorSnapshot{VolRatio: 1.2}
	supports := []Level{{Price: 100, Strength: 1.0, Type: "Support"}}

	sig := CheckSupportBounce("2330", snap, candle, supports)
	if sig != nil {
		t.Errorf("expected nil (zero range candle must not panic or false-trigger), got %+v", sig)
	}
}

func TestCheckSupportBounce_ChecksAllSupportsUntilMatch(t *testing.T) {
	ts := time.Now()
	candle := bounceCandle(50.5, 53, 49.5, 52, 2_000_000, ts)
	snap := &store.IndicatorSnapshot{VolRatio: 1.2}
	supports := []Level{
		{Price: 100, Strength: 1.0, Type: "Support"}, // 不匹配
		{Price: 50, Strength: 0.5, Type: "Support"},  // 匹配
	}

	sig := CheckSupportBounce("2330", snap, candle, supports)
	if sig == nil {
		t.Fatal("expected a signal from the second support level")
	}
	if sig.Support != 50 {
		t.Errorf("Support = %v, want 50", sig.Support)
	}
}

func TestCheckSupportBounce_NoSignalWhenNoSupports(t *testing.T) {
	candle := bounceCandle(100.5, 103, 99.5, 102, 2_000_000, time.Now())
	snap := &store.IndicatorSnapshot{VolRatio: 1.2}

	sig := CheckSupportBounce("2330", snap, candle, nil)
	if sig != nil {
		t.Errorf("expected nil with no support levels, got %+v", sig)
	}
}
