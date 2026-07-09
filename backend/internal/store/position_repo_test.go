package store

import (
	"context"
	"errors"
	"math"
	"os"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/database"
)

func newTestPositionRepo(t *testing.T) PositionRepo {
	t.Helper()
	tmp, err := os.CreateTemp("", "position-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	t.Cleanup(func() { os.Remove(tmp.Name()) })
	db, err := NewDB(config.DatabaseConfig{Driver: "sqlite", DSN: tmp.Name()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.RunMigrations(context.Background(), db, "sqlite", zap.NewNop()); err != nil {
		t.Fatal(err)
	}
	return NewPositionRepo(db)
}

func TestPositionRepoAVGEventsAndAdjustment(t *testing.T) {
	repo := newTestPositionRepo(t)
	ctx := context.Background()
	now := time.Now().UTC()

	p, err := repo.ApplyEvent(ctx, &PositionTransaction{
		Symbol: "2330", EventType: PositionEventBuy, OccurredAt: now,
		Shares: NewNullFloat64(100), Price: NewNullFloat64(10), Fee: 10,
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if p.Shares != 100 || math.Abs(p.AvgCost-10.1) > 1e-9 || p.Version != 1 {
		t.Fatalf("unexpected first BUY: %+v", p)
	}
	p, err = repo.ApplyEvent(ctx, &PositionTransaction{
		Symbol: "2330", EventType: PositionEventBuy, OccurredAt: now.Add(time.Minute),
		Shares: NewNullFloat64(100), Price: NewNullFloat64(20),
	}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(p.AvgCost-15.05) > 1e-9 {
		t.Fatalf("unexpected AVG: %+v", p)
	}
	p, err = repo.ApplyEvent(ctx, &PositionTransaction{
		Symbol: "2330", EventType: PositionEventSell, OccurredAt: now.Add(2 * time.Minute),
		Shares: NewNullFloat64(50), Price: NewNullFloat64(30), Fee: 5, Tax: 2,
	}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if p.Shares != 150 || math.Abs(p.RealizedPnL-740.5) > 1e-9 || math.Abs(p.AvgCost-15.05) > 1e-9 {
		t.Fatalf("unexpected SELL: %+v", p)
	}
	p, err = repo.ApplyEvent(ctx, &PositionTransaction{
		Symbol: "2330", EventType: PositionEventAdjustment, OccurredAt: now.Add(3 * time.Minute),
		TargetShares: NewNullFloat64(120), TargetAvgCost: NewNullFloat64(16), Note: "broker reconciliation",
	}, 3)
	if err != nil {
		t.Fatal(err)
	}
	if p.Shares != 120 || p.AvgCost != 16 || p.Version != 4 {
		t.Fatalf("unexpected ADJUSTMENT: %+v", p)
	}
	if math.Abs(p.RealizedPnL-740.5) > 1e-9 {
		t.Fatalf("ADJUSTMENT must not invent cash flow or change realized PnL: %+v", p)
	}
	if _, err := repo.ApplyEvent(ctx, &PositionTransaction{
		Symbol: "2330", EventType: PositionEventSell, OccurredAt: now,
		Shares: NewNullFloat64(121), Price: NewNullFloat64(20),
	}, 4); !errors.Is(err, ErrPositionInvalidEvent) {
		t.Fatalf("expected invalid-event oversell rejection, got %v", err)
	}
	if _, err := repo.ApplyEvent(ctx, &PositionTransaction{
		Symbol: "2330", EventType: PositionEventBuy, OccurredAt: now,
		Shares: NewNullFloat64(1), Price: NewNullFloat64(20),
	}, 2); err != ErrPositionVersionConflict {
		t.Fatalf("expected version conflict, got %v", err)
	}
}

func TestPositionRepoAnalysisRoundTrip(t *testing.T) {
	repo := newTestPositionRepo(t)
	ctx := context.Background()
	a := &PositionAnalysis{
		Symbol: "2330", PositionState: "FLAT", AnalyzedAt: time.Now().UTC(),
		CurrentPrice: 100, Action: "ENTER_SMALL", ActionLabel: "小量建立",
		TargetShares: 500, AdjustmentShares: 500, AdjustmentSide: "BUY",
		AdjustmentAmount: 50000, ConfigJSON: RawJSON(`{"max_position_value":200000}`),
		Reason: RawJSON(`["rr ok"]`), Evidence: RawJSON(`{"support_zone":{"id":1}}`),
		TriggerConditions: RawJSON(`[]`), InvalidationConditions: RawJSON(`["close below 90"]`),
		RuleVersion: "position_sr_zone_v1",
	}
	id, err := repo.CreateAnalysis(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := repo.GetAnalysis(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Action != "ENTER_SMALL" || saved.TargetShares != 500 || string(saved.Reason) != `["rr ok"]` {
		t.Fatalf("unexpected analysis: %+v", saved)
	}
}
