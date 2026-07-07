package analysis

import (
	"database/sql"
	"testing"
	"time"

	"github.com/trading/backend/internal/store"
)

func nf(v float64) store.NullFloat64 {
	return store.NullFloat64{NullFloat64: sql.NullFloat64{Float64: v, Valid: true}}
}

// activeLong 建一個進場為 ACTIVE / LONG 的分析快照，停損 95、停利 110。
func activeLong() *store.StockAnalysis {
	return &store.StockAnalysis{
		Symbol:              "2330",
		Timeframe:           "1d",
		EntryStatus:         "ACTIVE",
		EntryDirection:      "LONG",
		EntryPrice:          100,
		StopLossATR:         nf(95),
		TakeProfitNextLevel: nf(110),
	}
}

func candle(day int, low, high float64) store.Candle {
	return store.Candle{
		Low:       low,
		High:      high,
		Timestamp: time.Date(2026, 7, day, 13, 30, 0, 0, time.UTC),
	}
}

func TestResolveExitOnlyStopLoss(t *testing.T) {
	a := activeLong()
	// day1 Low 94 觸停損；沒有任何K棒 High >= 110
	candles := []store.Candle{candle(1, 94, 105), candle(2, 96, 108)}

	v := verifyTrade(a, candles)
	if v.Resolution == nil || v.Resolution.FirstExit != "STOP_LOSS" {
		t.Fatalf("expected FirstExit=STOP_LOSS, got %+v", v.Resolution)
	}
	if v.Resolution.SameBarTie {
		t.Fatalf("expected SameBarTie=false, got true")
	}
}

func TestResolveExitOnlyTakeProfit(t *testing.T) {
	a := activeLong()
	// day1 High 111 觸停利；沒有任何K棒 Low <= 95
	candles := []store.Candle{candle(1, 97, 111), candle(2, 98, 112)}

	v := verifyTrade(a, candles)
	if v.Resolution == nil || v.Resolution.FirstExit != "TAKE_PROFIT" {
		t.Fatalf("expected FirstExit=TAKE_PROFIT, got %+v", v.Resolution)
	}
	if v.Resolution.SameBarTie {
		t.Fatalf("expected SameBarTie=false, got true")
	}
}

func TestResolveExitStopBeforeTarget(t *testing.T) {
	a := activeLong()
	// day1 只觸停損（Low 94, High 105）；day2 才觸停利（High 111）
	candles := []store.Candle{candle(1, 94, 105), candle(2, 98, 111)}

	v := verifyTrade(a, candles)
	if v.Resolution == nil || v.Resolution.FirstExit != "STOP_LOSS" || v.Resolution.SameBarTie {
		t.Fatalf("expected STOP_LOSS without tie, got %+v", v.Resolution)
	}
	want := candle(1, 94, 105).Timestamp
	if v.Resolution.ResolvedAt == nil || !v.Resolution.ResolvedAt.Equal(want) {
		t.Fatalf("expected ResolvedAt=%v, got %+v", want, v.Resolution.ResolvedAt)
	}
}

func TestResolveExitTargetBeforeStop(t *testing.T) {
	a := activeLong()
	// day1 只觸停利（High 111, Low 97）；day2 才觸停損（Low 94）
	candles := []store.Candle{candle(1, 97, 111), candle(2, 94, 108)}

	v := verifyTrade(a, candles)
	if v.Resolution == nil || v.Resolution.FirstExit != "TAKE_PROFIT" || v.Resolution.SameBarTie {
		t.Fatalf("expected TAKE_PROFIT without tie, got %+v", v.Resolution)
	}
}

func TestResolveExitSameBarTieFavorsStopLoss(t *testing.T) {
	a := activeLong()
	// 同一根K棒同時觸及停損（Low 94）與停利（High 111）→ 保守判定停損先
	candles := []store.Candle{candle(1, 94, 111)}

	v := verifyTrade(a, candles)
	if v.Resolution == nil || v.Resolution.FirstExit != "STOP_LOSS" {
		t.Fatalf("expected FirstExit=STOP_LOSS, got %+v", v.Resolution)
	}
	if !v.Resolution.SameBarTie {
		t.Fatalf("expected SameBarTie=true on same-bar collision, got false")
	}
	// 原始 hit_at 仍必須各自保留
	if !v.StopLoss["atr"].Hit || !v.TakeProfit["next_level"].Hit {
		t.Fatalf("expected raw stop/target hits to be preserved, got %+v", v)
	}
}

func TestResolveExitNoneWhenNeitherHit(t *testing.T) {
	a := activeLong()
	// 從未觸及停損或停利
	candles := []store.Candle{candle(1, 97, 105), candle(2, 98, 108)}

	v := verifyTrade(a, candles)
	if v.Resolution == nil || v.Resolution.FirstExit != "NONE" {
		t.Fatalf("expected FirstExit=NONE, got %+v", v.Resolution)
	}
	if v.Resolution.ResolvedAt != nil {
		t.Fatalf("expected ResolvedAt=nil, got %+v", v.Resolution.ResolvedAt)
	}
}

func TestVerifyTradeNotApplicableWhenWatching(t *testing.T) {
	a := activeLong()
	a.EntryStatus = "WATCHING"

	v := verifyTrade(a, []store.Candle{candle(1, 94, 111)})
	if v.Applicable {
		t.Fatalf("expected Applicable=false for WATCHING, got true")
	}
	if v.Resolution != nil {
		t.Fatalf("expected no Resolution for WATCHING, got %+v", v.Resolution)
	}
}
