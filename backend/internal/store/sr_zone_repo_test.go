package store

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/database"
)

func newTestSRZoneRepo(t *testing.T) SRZoneRepo {
	t.Helper()

	tmp, err := os.CreateTemp("", "sr-zone-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	t.Cleanup(func() { os.Remove(tmp.Name()) })

	db, err := NewDB(config.DatabaseConfig{Driver: "sqlite", DSN: tmp.Name()})
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := database.RunMigrations(context.Background(), db, "sqlite", zap.NewNop()); err != nil {
		t.Fatalf("migrations failed: %v", err)
	}

	return NewSRZoneRepo(db)
}

func testAnalysis() *SRZoneAnalysis {
	return &SRZoneAnalysis{
		Symbol:       "2330",
		Timeframe:    "1d",
		AnalyzedAt:   time.Now().UTC().Truncate(time.Second),
		CurrentPrice: 600.0,
		ModelVersion: "v1",
	}
}

func testZones() []SRZone {
	return []SRZone{
		{
			PriceLow: 580.0, PriceHigh: 585.0, Method: "atr", Role: "SUPPORT",
			SupportScore: 0.8, ResistanceScore: 0.1,
			BounceProbability:   NullFloat64{sql.NullFloat64{Float64: 0.72, Valid: true}},
			BreakProbability:    NullFloat64{sql.NullFloat64{Float64: 0.2, Valid: true}},
			TouchCount:          4, RejectionCount: 3, BreakoutCount: 0,
			AvgReturnAfterTouch: 0.02, RelativeVolume: 1.4, Volatility: 0.015, TrendStrength: 0.03,
		},
		{
			PriceLow: 610.0, PriceHigh: 615.0, Method: "volume_profile", Role: "RESISTANCE",
			SupportScore: 0.1, ResistanceScore: 0.65,
			TouchCount: 2, RejectionCount: 1, BreakoutCount: 1,
			AvgReturnAfterTouch: -0.01, RelativeVolume: 1.1, Volatility: 0.018, TrendStrength: -0.01,
		},
	}
}

func TestSRZoneRepoCreateGetRoundTrip(t *testing.T) {
	repo := newTestSRZoneRepo(t)
	ctx := context.Background()

	id, err := repo.Create(ctx, testAnalysis(), testZones())
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if id == 0 {
		t.Fatalf("expected non-zero id")
	}

	saved, err := repo.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if saved.Symbol != "2330" || saved.Timeframe != "1d" || saved.ModelVersion != "v1" {
		t.Fatalf("unexpected saved analysis: %+v", saved)
	}

	zones, err := repo.GetZones(ctx, id)
	if err != nil {
		t.Fatalf("GetZones failed: %v", err)
	}
	if len(zones) != 2 {
		t.Fatalf("expected 2 zones, got %d", len(zones))
	}
	for _, z := range zones {
		if z.AnalysisID != id {
			t.Fatalf("expected zone.AnalysisID=%d, got %d", id, z.AnalysisID)
		}
		if z.Status != "PENDING" {
			t.Fatalf("expected default status PENDING, got %s", z.Status)
		}
	}
}

func TestSRZoneRepoListFiltersBySymbol(t *testing.T) {
	repo := newTestSRZoneRepo(t)
	ctx := context.Background()

	a1 := testAnalysis()
	a1.Symbol = "2330"
	if _, err := repo.Create(ctx, a1, testZones()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	a2 := testAnalysis()
	a2.Symbol = "2454"
	if _, err := repo.Create(ctx, a2, testZones()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	rows, err := repo.List(ctx, "2330", 20)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(rows) != 1 || rows[0].Symbol != "2330" {
		t.Fatalf("expected 1 row for symbol=2330, got %+v", rows)
	}

	all, err := repo.List(ctx, "", 20)
	if err != nil {
		t.Fatalf("List(all) failed: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 rows total, got %d", len(all))
	}
}

func TestSRZoneRepoDeleteCascadesZones(t *testing.T) {
	repo := newTestSRZoneRepo(t)
	ctx := context.Background()

	id, err := repo.Create(ctx, testAnalysis(), testZones())
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := repo.Delete(ctx, id); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if _, err := repo.Get(ctx, id); err == nil {
		t.Fatalf("expected error getting deleted analysis")
	}
	zones, err := repo.GetZones(ctx, id)
	if err != nil {
		t.Fatalf("GetZones after delete failed: %v", err)
	}
	if len(zones) != 0 {
		t.Fatalf("expected 0 zones after cascade delete, got %d", len(zones))
	}
}
