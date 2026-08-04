package store

import (
	"context"
	"database/sql"
	"encoding/json"
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
		MetricsJSON: RawJSON(`{
			"schema_version":"sr_zone_decision_replay_p0",
			"rows":42,
			"sources":2,
			"governance_evaluation":{"health_state":"DEGRADED","strict_passed":false},
			"thresholds":{"hold_auc":0.75},
			"fixture":"sr_v3"
		}`),
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
	if got.SchemaVersion != "sr_zone_decision_replay_p0" {
		t.Fatalf("unexpected schema_version: %s", got.SchemaVersion)
	}
	if !got.Rows.Valid || got.Rows.Int64 != 42 {
		t.Fatalf("unexpected rows: %+v", got.Rows)
	}
	if !got.Sources.Valid || got.Sources.Int64 != 2 {
		t.Fatalf("unexpected sources: %+v", got.Sources)
	}
	if got.GovernanceHealthState != "DEGRADED" {
		t.Fatalf("unexpected governance health state: %s", got.GovernanceHealthState)
	}
	if !got.GovernanceStrictPassed.Valid || got.GovernanceStrictPassed.Bool {
		t.Fatalf("unexpected governance strict passed: %+v", got.GovernanceStrictPassed)
	}
	var metrics map[string]any
	if err := json.Unmarshal([]byte(got.MetricsJSON), &metrics); err != nil {
		t.Fatalf("metrics_json should remain valid JSON: %v", err)
	}
	if metrics["fixture"] != "sr_v3" {
		t.Fatalf("expected metrics_json round-trip, got %+v", metrics)
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

func TestSRRegressionResultRepoListBySchemaVersion(t *testing.T) {
	repo := newTestSRRegressionResultRepo(t)
	ctx := context.Background()

	fixtures := []struct {
		runID       string
		metricsJSON RawJSON
	}{
		{runID: "sr_eval_001", metricsJSON: RawJSON(`{"schema_version":"sr_zone_evaluation_p0"}`)},
		{runID: "sr_replay_001", metricsJSON: RawJSON(`{"schema_version":"sr_zone_decision_replay_p0","ok":true}`)},
		{runID: "sr_replay_002", metricsJSON: RawJSON(`{"schema_version":"sr_zone_decision_replay_p0","ok":false}`)},
	}
	for _, fixture := range fixtures {
		if _, err := repo.Create(ctx, &SRRegressionResult{RunID: fixture.runID, MetricsJSON: fixture.metricsJSON}); err != nil {
			t.Fatalf("Create %s failed: %v", fixture.runID, err)
		}
	}

	rows, err := repo.ListBySchemaVersion(ctx, "sr_zone_decision_replay_p0", 10)
	if err != nil {
		t.Fatalf("ListBySchemaVersion failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 replay rows, got %d", len(rows))
	}
	if rows[0].RunID != "sr_replay_002" || rows[1].RunID != "sr_replay_001" {
		t.Fatalf("expected replay rows newest first, got %+v", rows)
	}
}
