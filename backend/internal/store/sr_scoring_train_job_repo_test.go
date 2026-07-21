package store

import (
	"context"
	"os"
	"testing"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/database"
)

func newTestSRScoringTrainJobRepo(t *testing.T) SRScoringTrainJobRepo {
	t.Helper()

	tmp, err := os.CreateTemp("", "sr-train-job-test-*.db")
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

	return NewSRScoringTrainJobRepo(db)
}

func testTrainJob(jobID string) *SRScoringTrainJob {
	return &SRScoringTrainJob{
		JobID:      jobID,
		Symbols:    `["2330","2454"]`,
		Timeframe:  "1d",
		FetchLimit: 1500,
		ModelType:  "gradient_boosting",
	}
}

func TestSRScoringTrainJobRepoCreateStartsAsPending(t *testing.T) {
	repo := newTestSRScoringTrainJobRepo(t)
	ctx := context.Background()

	if _, err := repo.Create(ctx, testTrainJob("sr_train_001")); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	job, err := repo.Get(ctx, "sr_train_001")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if job.Status != "pending" {
		t.Fatalf("expected status=pending, got %q", job.Status)
	}
	if job.Symbols != `["2330","2454"]` || job.Timeframe != "1d" || job.FetchLimit != 1500 || job.ModelType != "gradient_boosting" {
		t.Fatalf("unexpected job fields: %+v", job)
	}
	if job.Rows.Valid || job.Sources.Valid || job.ModelPath.Valid || job.ModelVersion.Valid || job.SplitMethod.Valid || job.Error.Valid {
		t.Fatalf("expected all result fields to be NULL before job runs, got %+v", job)
	}
	if job.StartedAt.Valid || job.FinishedAt.Valid {
		t.Fatalf("expected started_at/finished_at to be NULL for a pending job, got %+v", job)
	}
}

func TestSRScoringTrainJobRepoMarkRunningThenDone(t *testing.T) {
	repo := newTestSRScoringTrainJobRepo(t)
	ctx := context.Background()

	if _, err := repo.Create(ctx, testTrainJob("sr_train_002")); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if err := repo.MarkRunning(ctx, "sr_train_002"); err != nil {
		t.Fatalf("MarkRunning failed: %v", err)
	}

	running, err := repo.Get(ctx, "sr_train_002")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if running.Status != "running" || !running.StartedAt.Valid {
		t.Fatalf("expected status=running with started_at set, got %+v", running)
	}

	metrics := RawJSON(`{"hold":{"auc":0.81,"brier_score":0.12,"log_loss":0.34,"calibrated":1,"test_rows":44},"break":{"auc":0.77,"brier_score":0.16,"log_loss":0.41,"calibrated":0,"test_rows":36}}`)
	datasetSummary := RawJSON(`{"rows":128,"rows_by_symbol":{"2330":90,"2454":38}}`)
	if err := repo.MarkDone(ctx, "sr_train_002", 128, 3, metrics, "models/sr_scoring_v2.joblib", "v2", "time", datasetSummary); err != nil {
		t.Fatalf("MarkDone failed: %v", err)
	}

	done, err := repo.Get(ctx, "sr_train_002")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if done.Status != "done" || !done.FinishedAt.Valid {
		t.Fatalf("expected status=done with finished_at set, got %+v", done)
	}
	if !done.Rows.Valid || done.Rows.Int64 != 128 || !done.Sources.Valid || done.Sources.Int64 != 3 {
		t.Fatalf("expected rows=128 sources=3, got %+v", done)
	}
	if !done.ModelPath.Valid || done.ModelPath.String != "models/sr_scoring_v2.joblib" {
		t.Fatalf("unexpected model_path: %+v", done.ModelPath)
	}
	if !done.ModelVersion.Valid || done.ModelVersion.String != "v2" {
		t.Fatalf("unexpected model_version: %+v", done.ModelVersion)
	}
	if !done.SplitMethod.Valid || done.SplitMethod.String != "time" {
		t.Fatalf("unexpected split_method: %+v", done.SplitMethod)
	}
	if len(done.Metrics) == 0 {
		t.Fatalf("expected non-empty metrics JSON, got %+v", done.Metrics)
	}
	if len(done.DatasetSummary) == 0 {
		t.Fatalf("expected non-empty dataset_summary JSON, got %+v", done.DatasetSummary)
	}

	modelMetric, err := repo.GetModelMetric(ctx, "sr_train_002")
	if err != nil {
		t.Fatalf("GetModelMetric failed: %v", err)
	}
	if modelMetric.JobID != "sr_train_002" || modelMetric.ModelVersion != "v2" || modelMetric.ModelType != "gradient_boosting" {
		t.Fatalf("unexpected model metric metadata: %+v", modelMetric)
	}
	if !modelMetric.HoldAUC.Valid || modelMetric.HoldAUC.Float64 != 0.81 {
		t.Fatalf("unexpected hold_auc: %+v", modelMetric.HoldAUC)
	}
	if !modelMetric.HoldCalibrated.Valid || !modelMetric.HoldCalibrated.Bool {
		t.Fatalf("unexpected hold_calibrated: %+v", modelMetric.HoldCalibrated)
	}
	if !modelMetric.BreakCalibrated.Valid || modelMetric.BreakCalibrated.Bool {
		t.Fatalf("unexpected break_calibrated: %+v", modelMetric.BreakCalibrated)
	}
	if !modelMetric.BreakTestRows.Valid || modelMetric.BreakTestRows.Int64 != 36 {
		t.Fatalf("unexpected break_test_rows: %+v", modelMetric.BreakTestRows)
	}
	if string(modelMetric.MetricsJSON) != string(metrics) || string(modelMetric.DatasetSummaryJSON) != string(datasetSummary) {
		t.Fatalf("expected raw metrics/dataset summary to round-trip, got %+v", modelMetric)
	}
}

func TestSRScoringTrainJobRepoMarkFailed(t *testing.T) {
	repo := newTestSRScoringTrainJobRepo(t)
	ctx := context.Background()

	if _, err := repo.Create(ctx, testTrainJob("sr_train_003")); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if err := repo.MarkRunning(ctx, "sr_train_003"); err != nil {
		t.Fatalf("MarkRunning failed: %v", err)
	}
	if err := repo.MarkFailed(ctx, "sr_train_003", "python sr-scoring train request failed: dial tcp: connection refused"); err != nil {
		t.Fatalf("MarkFailed failed: %v", err)
	}

	job, err := repo.Get(ctx, "sr_train_003")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if job.Status != "failed" || !job.FinishedAt.Valid {
		t.Fatalf("expected status=failed with finished_at set, got %+v", job)
	}
	if !job.Error.Valid || job.Error.String == "" {
		t.Fatalf("expected non-empty error message, got %+v", job.Error)
	}
	if job.Rows.Valid || job.ModelVersion.Valid {
		t.Fatalf("expected result fields to stay NULL on failure, got %+v", job)
	}
}

func TestSRScoringTrainJobRepoGetUnknownJobIDReturnsError(t *testing.T) {
	repo := newTestSRScoringTrainJobRepo(t)
	if _, err := repo.Get(context.Background(), "does-not-exist"); err == nil {
		t.Fatal("expected error for unknown job_id")
	}
}

func TestSRScoringTrainJobRepoListOrdersByCreatedAtDesc(t *testing.T) {
	repo := newTestSRScoringTrainJobRepo(t)
	ctx := context.Background()

	if _, err := repo.Create(ctx, testTrainJob("sr_train_a")); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if _, err := repo.Create(ctx, testTrainJob("sr_train_b")); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	jobs, err := repo.List(ctx, 20)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
	// 後建立的排在前面（created_at DESC）
	if jobs[0].JobID != "sr_train_b" || jobs[1].JobID != "sr_train_a" {
		t.Fatalf("unexpected order: %+v", jobs)
	}
}

func TestSRScoringTrainJobRepoPruneTerminalKeepsNewestDoneFailedOnly(t *testing.T) {
	repo := newTestSRScoringTrainJobRepo(t)
	ctx := context.Background()

	for _, jobID := range []string{"sr_train_done_old", "sr_train_failed_old", "sr_train_done_new", "sr_train_pending", "sr_train_running"} {
		if _, err := repo.Create(ctx, testTrainJob(jobID)); err != nil {
			t.Fatalf("Create(%s) failed: %v", jobID, err)
		}
	}
	if err := repo.MarkDone(ctx, "sr_train_done_old", 10, 1, RawJSON(`{}`), "model.joblib", "v4", "time", RawJSON(`{}`)); err != nil {
		t.Fatalf("MarkDone old failed: %v", err)
	}
	if err := repo.MarkFailed(ctx, "sr_train_failed_old", "failed"); err != nil {
		t.Fatalf("MarkFailed old failed: %v", err)
	}
	if err := repo.MarkDone(ctx, "sr_train_done_new", 20, 2, RawJSON(`{}`), "model.joblib", "v4", "time", RawJSON(`{}`)); err != nil {
		t.Fatalf("MarkDone new failed: %v", err)
	}
	if err := repo.MarkRunning(ctx, "sr_train_running"); err != nil {
		t.Fatalf("MarkRunning failed: %v", err)
	}

	deleted, err := repo.PruneTerminal(ctx, 1)
	if err != nil {
		t.Fatalf("PruneTerminal failed: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("expected deleted=2, got %d", deleted)
	}

	if _, err := repo.Get(ctx, "sr_train_done_new"); err != nil {
		t.Fatalf("newest terminal job should remain: %v", err)
	}
	if _, err := repo.Get(ctx, "sr_train_pending"); err != nil {
		t.Fatalf("pending job should remain: %v", err)
	}
	if _, err := repo.Get(ctx, "sr_train_running"); err != nil {
		t.Fatalf("running job should remain: %v", err)
	}
	if _, err := repo.Get(ctx, "sr_train_done_old"); err == nil {
		t.Fatal("old done job should be pruned")
	}
	if _, err := repo.Get(ctx, "sr_train_failed_old"); err == nil {
		t.Fatal("old failed job should be pruned")
	}
}
