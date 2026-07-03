package analysis

import (
	"testing"
	"time"

	"github.com/trading/backend/internal/store"
)

func candleAt(t time.Time, o, h, l, c float64) store.Candle {
	return store.Candle{Open: o, High: h, Low: l, Close: c, Timestamp: t}
}

func day(n int) time.Time {
	return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, n)
}

func TestVerifySRZoneSupportHeldWhenTouchedButNotBroken(t *testing.T) {
	z := store.SRZone{Role: "SUPPORT", PriceLow: 95.0, PriceHigh: 100.0}
	candles := []store.Candle{
		candleAt(day(1), 105, 106, 96, 97), // 觸碰區間但收在區間內，不算跌破
		candleAt(day(2), 97, 107, 96, 106), // 反彈離開
	}

	status, brokenAt, brokenPrice := verifySRZone(z, candles, DefaultConfirmationBars)

	if status != "HELD_SO_FAR" {
		t.Fatalf("expected HELD_SO_FAR, got %s", status)
	}
	if brokenAt != nil || brokenPrice != nil {
		t.Fatalf("expected no broken_at/broken_price, got %v/%v", brokenAt, brokenPrice)
	}
}

func TestVerifySRZonePendingWhenNeverTouched(t *testing.T) {
	z := store.SRZone{Role: "SUPPORT", PriceLow: 95.0, PriceHigh: 100.0}
	candles := []store.Candle{
		candleAt(day(1), 120, 122, 118, 121),
		candleAt(day(2), 121, 123, 119, 122),
	}

	status, brokenAt, _ := verifySRZone(z, candles, DefaultConfirmationBars)

	if status != "PENDING" {
		t.Fatalf("expected PENDING, got %s", status)
	}
	if brokenAt != nil {
		t.Fatalf("expected no broken_at, got %v", brokenAt)
	}
}

func TestVerifySRZoneSupportBrokenAfterConsecutiveCloses(t *testing.T) {
	z := store.SRZone{Role: "SUPPORT", PriceLow: 95.0, PriceHigh: 100.0}
	candles := []store.Candle{
		candleAt(day(1), 98, 99, 90, 92),   // 第一根跌破，streak=1，還沒確認
		candleAt(day(2), 92, 93, 89, 91),   // 第二根跌破，streak=2，達到 confirmationBars=2，判定 BROKEN
		candleAt(day(3), 91, 105, 91, 104), // 之後反彈，不應該影響結果
	}

	status, brokenAt, brokenPrice := verifySRZone(z, candles, DefaultConfirmationBars)

	if status != "BROKEN" {
		t.Fatalf("expected BROKEN, got %s", status)
	}
	if brokenAt == nil || !brokenAt.Equal(day(1)) {
		t.Fatalf("expected broken_at=day(1) (first bar of the confirmed streak), got %v", brokenAt)
	}
	if brokenPrice == nil || *brokenPrice != 92.0 {
		t.Fatalf("expected broken_price=92.0 (close of day(1)), got %v", brokenPrice)
	}
}

func TestVerifySRZoneSupportSingleBarBreakDoesNotConfirm(t *testing.T) {
	z := store.SRZone{Role: "SUPPORT", PriceLow: 95.0, PriceHigh: 100.0}
	candles := []store.Candle{
		candleAt(day(1), 98, 99, 90, 92), // 跌破一天就反彈，未達 confirmationBars=2，不算 BROKEN
		candleAt(day(2), 92, 105, 92, 104),
	}

	status, _, _ := verifySRZone(z, candles, DefaultConfirmationBars)

	if status != "HELD_SO_FAR" {
		t.Fatalf("expected HELD_SO_FAR (single-bar break doesn't confirm), got %s", status)
	}
}

func TestVerifySRZoneResistanceBrokenAfterConsecutiveCloses(t *testing.T) {
	z := store.SRZone{Role: "RESISTANCE", PriceLow: 95.0, PriceHigh: 100.0}
	candles := []store.Candle{
		candleAt(day(1), 98, 106, 97, 105),
		candleAt(day(2), 105, 108, 104, 107),
	}

	status, brokenAt, brokenPrice := verifySRZone(z, candles, DefaultConfirmationBars)

	if status != "BROKEN" {
		t.Fatalf("expected BROKEN, got %s", status)
	}
	if brokenAt == nil || !brokenAt.Equal(day(1)) {
		t.Fatalf("unexpected broken_at: %v", brokenAt)
	}
	if brokenPrice == nil || *brokenPrice != 105.0 {
		t.Fatalf("unexpected broken_price: %v", brokenPrice)
	}
}

func TestVerifySRZoneAtZoneStaysPendingWhilePriceRemainsInside(t *testing.T) {
	z := store.SRZone{Role: "AT_ZONE", PriceLow: 95.0, PriceHigh: 100.0}
	candles := []store.Candle{
		candleAt(day(1), 97, 99, 96, 98),
		candleAt(day(2), 98, 99, 96, 97),
	}

	status, brokenAt, _ := verifySRZone(z, candles, DefaultConfirmationBars)

	if status != "PENDING" {
		t.Fatalf("expected PENDING while price stays inside the zone, got %s", status)
	}
	if brokenAt != nil {
		t.Fatalf("expected no broken_at, got %v", brokenAt)
	}
}

func TestVerifySRZoneAtZoneResolvesRoleAfterExitingAbove(t *testing.T) {
	z := store.SRZone{Role: "AT_ZONE", PriceLow: 95.0, PriceHigh: 100.0}
	candles := []store.Candle{
		candleAt(day(1), 98, 99, 96, 97),   // 還在區間內
		candleAt(day(2), 97, 106, 97, 105), // 收盤離開區間上方 → 之後這個 zone 對它而言是支撐
		candleAt(day(3), 105, 106, 96, 97), // 跌回區間但只有一天，未達 confirmationBars
	}

	status, _, _ := verifySRZone(z, candles, DefaultConfirmationBars)

	// 離開區間後只被觸碰一次、沒有連續 2 根確認跌破，應該是 HELD_SO_FAR
	if status != "HELD_SO_FAR" {
		t.Fatalf("expected HELD_SO_FAR after resolving to SUPPORT and surviving a single-bar re-test, got %s", status)
	}
}

func TestVerifySRZoneAtZoneResolvesRoleAfterExitingBelowThenBreaks(t *testing.T) {
	z := store.SRZone{Role: "AT_ZONE", PriceLow: 95.0, PriceHigh: 100.0}
	candles := []store.Candle{
		candleAt(day(1), 97, 99, 94, 95.5), // 還在區間內
		candleAt(day(2), 95, 96, 88, 90),   // 收盤離開區間下方 → 之後這個 zone 對它而言是壓力
		candleAt(day(3), 90, 102, 90, 101), // 連續兩根收在區間上方，確認突破壓力
		candleAt(day(4), 101, 103, 100, 102),
	}

	status, brokenAt, _ := verifySRZone(z, candles, DefaultConfirmationBars)

	if status != "BROKEN" {
		t.Fatalf("expected BROKEN after resolving to RESISTANCE and then breaking out, got %s", status)
	}
	if brokenAt == nil || !brokenAt.Equal(day(3)) {
		t.Fatalf("unexpected broken_at: %v", brokenAt)
	}
}

func TestVerifySRZoneNoFutureCandlesKeepsPending(t *testing.T) {
	z := store.SRZone{Role: "SUPPORT", PriceLow: 95.0, PriceHigh: 100.0}

	status, brokenAt, brokenPrice := verifySRZone(z, nil, DefaultConfirmationBars)

	if status != "PENDING" {
		t.Fatalf("expected PENDING with no future candles, got %s", status)
	}
	if brokenAt != nil || brokenPrice != nil {
		t.Fatalf("expected no broken_at/broken_price, got %v/%v", brokenAt, brokenPrice)
	}
}

func TestVerifySRZoneIsIdempotentAfterBrokenEvenWithLaterRecovery(t *testing.T) {
	z := store.SRZone{Role: "SUPPORT", PriceLow: 95.0, PriceHigh: 100.0}
	candles := []store.Candle{
		candleAt(day(1), 98, 99, 90, 92),
		candleAt(day(2), 92, 93, 89, 91),
		candleAt(day(3), 91, 110, 91, 109), // 之後強力反彈也不該改回 HELD_SO_FAR
		candleAt(day(4), 109, 111, 108, 110),
	}

	status1, brokenAt1, _ := verifySRZone(z, candles, DefaultConfirmationBars)
	status2, brokenAt2, _ := verifySRZone(z, candles, DefaultConfirmationBars)

	if status1 != "BROKEN" || status2 != "BROKEN" {
		t.Fatalf("expected BROKEN on both runs, got %s / %s", status1, status2)
	}
	if !brokenAt1.Equal(*brokenAt2) {
		t.Fatalf("expected identical broken_at across repeated verify calls (idempotent), got %v vs %v", brokenAt1, brokenAt2)
	}
}
