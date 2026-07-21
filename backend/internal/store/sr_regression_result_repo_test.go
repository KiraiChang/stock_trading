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

func newTestSRRegressionResultRepo(t *testing.T) SRRegressionResultRepo {
	t.Helper()

	tmp, err := os.CreateTemp("", "sr-regression-result-test-*.db")
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

	return NewSRRegressionResultRepo(db)
}

func TestSRRegressionResultRepoCreateAndGet(t *testing.T) {
	repo := newTestSRRegressionResultRepo(t)
	ctx := context.Background()
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)

	id, err := repo.Create(ctx, &SRRegressionResult{
		RunID:           "sr_regression_001",
		ModelConfigHash: "hash-v3",
		PipelineVersion: "sr_zone_pipeline_v3",
		DatasetFrom:     NullTime{NullTime: sql.NullTime{Time: from, Valid: true}},
		DatasetTo:       NullTime{NullTime: sql.NullTime{Time: to, Valid: true}},
		SplitMethod:     "time",
		HoldAUC:         NullFloat64{NullFloat64: sql.NullFloat64{Float64: 0.81, Valid: true}},
		HoldBrierScore:  NullFloat64{NullFloat64: sql.NullFloat64{Float64: 0.12, Valid: true}},
		BreakAUC:        NullFloat64{NullFloat64: sql.NullFloat64{Float64: 0.76, Valid: true}},
		BreakBrierScore: NullFloat64{NullFloat64: sql.NullFloat64{Float64: 0.16, Valid: true}},
		Passed:          NullBool{NullBool: sql.NullBool{Bool: true, Valid: true}},
		MetricsJSON:     RawJSON(`{"thresholds":{"hold_auc":0.75},"fixture":"sr_v3"}`),
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero id")
	}

	got, err := repo.Get(ctx, "sr_regression_001")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.RunID != "sr_regression_001" || got.ModelConfigHash != "hash-v3" || got.PipelineVersion != "sr_zone_pipeline_v3" {
		t.Fatalf("unexpected metadata: %+v", got)
	}
	if !got.DatasetFrom.Valid || !got.DatasetTo.Valid {
		t.Fatalf("expected dataset range to be persisted: %+v", got)
	}
	if !got.HoldAUC.Valid || got.HoldAUC.Float64 != 0.81 {
		t.Fatalf("unexpected hold_auc: %+v", got.HoldAUC)
	}
	if !got.Passed.Valid || !got.Passed.Bool {
		t.Fatalf("unexpected passed flag: %+v", got.Passed)
	}
	if string(got.MetricsJSON) != `{"thresholds":{"hold_auc":0.75},"fixture":"sr_v3"}` {
		t.Fatalf("expected metrics_json round-trip, got %s", got.MetricsJSON)
	}
}

func TestSRRegressionResultRepoDefaultsInvalidMetricsJSON(t *testing.T) {
	repo := newTestSRRegressionResultRepo(t)
	ctx := context.Background()

	if _, err := repo.Create(ctx, &SRRegressionResult{RunID: "sr_regression_002", MetricsJSON: RawJSON(`{bad json`)}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := repo.Get(ctx, "sr_regression_002")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if string(got.MetricsJSON) != "null" {
		t.Fatalf("expected invalid metrics_json to default to null, got %s", got.MetricsJSON)
	}
}

func TestSRRegressionResultRepoListOrdersNewestFirst(t *testing.T) {
	repo := newTestSRRegressionResultRepo(t)
	ctx := context.Background()

	for _, runID := range []string{"sr_regression_003", "sr_regression_004", "sr_regression_005"} {
		if _, err := repo.Create(ctx, &SRRegressionResult{RunID: runID, MetricsJSON: RawJSON(`{"ok":true}`)}); err != nil {
			t.Fatalf("Create %s failed: %v", runID, err)
		}
	}

	rows, err := repo.List(ctx, 2)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].RunID != "sr_regression_005" || rows[1].RunID != "sr_regression_004" {
		t.Fatalf("expected newest first by id tie-breaker, got %+v", rows)
	}
}
