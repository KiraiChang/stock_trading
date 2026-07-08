package store

import (
	"context"
	"os"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/database"
)

func newTestHoldingRepo(t *testing.T) HoldingRepo {
	t.Helper()

	tmp, err := os.CreateTemp("", "holding-test-*.db")
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

	return NewHoldingRepo(db)
}

func TestHoldingRepoCreateUpdateListDelete(t *testing.T) {
	repo := newTestHoldingRepo(t)
	ctx := context.Background()

	id, err := repo.Create(ctx, &Holding{Symbol: "2330", Shares: 1000, CostPrice: 600, Note: "core"})
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
	if saved.Symbol != "2330" || saved.Shares != 1000 || saved.CostPrice != 600 || saved.Note != "core" {
		t.Fatalf("unexpected saved holding: %+v", saved)
	}

	saved.Symbol = "2317"
	saved.Shares = 2000
	saved.CostPrice = 100
	saved.Note = "updated"
	if err := repo.Update(ctx, saved); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	updated, err := repo.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get updated failed: %v", err)
	}
	if updated.Symbol != "2317" || updated.Shares != 2000 || updated.CostPrice != 100 || updated.Note != "updated" {
		t.Fatalf("unexpected updated holding: %+v", updated)
	}

	rows, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != id {
		t.Fatalf("unexpected list result: %+v", rows)
	}

	if err := repo.Delete(ctx, id); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if _, err := repo.Get(ctx, id); err == nil {
		t.Fatalf("expected deleted holding to be missing")
	}
}

func TestHoldingRepoAnalysisRoundTripAndHistorySurvivesHoldingDelete(t *testing.T) {
	repo := newTestHoldingRepo(t)
	ctx := context.Background()

	holdingID, err := repo.Create(ctx, &Holding{Symbol: "2330", Shares: 1000, CostPrice: 600})
	if err != nil {
		t.Fatalf("Create holding failed: %v", err)
	}

	first := &HoldingAnalysis{
		HoldingID:         holdingID,
		Symbol:            "2330",
		Shares:            1000,
		CostPrice:         600,
		AnalyzedAt:        time.Date(2026, 7, 1, 13, 30, 0, 0, time.UTC),
		CurrentPrice:      650,
		SRZoneAnalysisID:  NewNullInt64(7),
		Action:            "HOLD",
		ActionLabel:       "繼續持有",
		StopLossPrice:     NewNullFloat64(620),
		StopLossAmount:    NewNullFloat64(0),
		TakeProfitPrice:   NewNullFloat64(680),
		TakeProfitAmount:  NewNullFloat64(80000),
		AddOnTriggerPrice: NewNullFloat64(690),
		AddOnAmount:       NewNullFloat64(162500),
		UnrealizedPnL:     50000,
		UnrealizedPnLPct:  0.083333,
		Reason:            RawJSON(`["以 650.00 的最新收盤價與 SR Zone 快照評估。"]`),
		DetailJSON:        RawJSON(`{"rule_version":"holding_sr_zone_v1"}`),
	}
	firstID, err := repo.CreateAnalysis(ctx, first)
	if err != nil {
		t.Fatalf("CreateAnalysis first failed: %v", err)
	}

	second := *first
	second.CurrentPrice = 640
	second.Action = "REDUCE"
	second.ActionLabel = "減碼"
	second.Reason = ""
	second.DetailJSON = ""
	secondID, err := repo.CreateAnalysis(ctx, &second)
	if err != nil {
		t.Fatalf("CreateAnalysis second failed: %v", err)
	}

	saved, err := repo.GetAnalysis(ctx, firstID)
	if err != nil {
		t.Fatalf("GetAnalysis failed: %v", err)
	}
	if saved.HoldingID != holdingID || saved.Symbol != "2330" || saved.CurrentPrice != 650 {
		t.Fatalf("unexpected saved analysis: %+v", saved)
	}
	if !saved.SRZoneAnalysisID.Valid || saved.SRZoneAnalysisID.Int64 != 7 {
		t.Fatalf("expected sr_zone_analysis_id=7, got %+v", saved.SRZoneAnalysisID)
	}
	if string(saved.Reason) != `["以 650.00 的最新收盤價與 SR Zone 快照評估。"]` {
		t.Fatalf("unexpected reason JSON: %s", saved.Reason)
	}

	rows, err := repo.ListAnalyses(ctx, holdingID, 1)
	if err != nil {
		t.Fatalf("ListAnalyses failed: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != secondID {
		t.Fatalf("expected latest one analysis id=%d, got %+v", secondID, rows)
	}
	if string(rows[0].Reason) != `[]` || string(rows[0].DetailJSON) != `{}` {
		t.Fatalf("expected default JSON fields, got reason=%s detail=%s", rows[0].Reason, rows[0].DetailJSON)
	}

	if err := repo.Delete(ctx, holdingID); err != nil {
		t.Fatalf("Delete holding failed: %v", err)
	}
	if _, err := repo.GetAnalysis(ctx, firstID); err != nil {
		t.Fatalf("expected analysis snapshot to survive holding delete: %v", err)
	}
}
