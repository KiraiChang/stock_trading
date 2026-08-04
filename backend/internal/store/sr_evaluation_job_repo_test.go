package store

import (
	"context"
	"os"
	"testing"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/database"
)

func newTestSREvaluationJobRepo(t *testing.T) SREvaluationJobRepo {
	t.Helper()

	tmp, err := os.CreateTemp("", "sr-evaluation-job-test-*.db")
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

	return NewSREvaluationJobRepo(db)
}

func TestSREvaluationJobRepoCreateAndMarkDone(t *testing.T) {
	repo := newTestSREvaluationJobRepo(t)
	ctx := context.Background()

	id, err := repo.Create(ctx, &SREvaluationJob{
		JobID:         "sr_eval_job_001",
		Symbols:       `["2330"]`,
		Timeframe:     "1d",
		FetchLimit:    1500,
		Mode:          "decision_replay",
		WriteDB:       true,
		ReplayMaxRows: 25,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero id")
	}

	if err := repo.MarkRunning(ctx, "sr_eval_job_001"); err != nil {
		t.Fatalf("MarkRunning failed: %v", err)
	}
	if err := repo.MarkDone(ctx, "sr_eval_job_001", RawJSON(`{"schema_version":"sr_zone_decision_replay_p0","rows":3}`), "sr_eval_001", "sr_zone_decision_replay_p0", "sr_zone_decision_replay_p0", 3, 1); err != nil {
		t.Fatalf("MarkDone failed: %v", err)
	}

	got, err := repo.Get(ctx, "sr_eval_job_001")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Status != "done" || !got.RunID.Valid || got.RunID.String != "sr_eval_001" {
		t.Fatalf("unexpected job metadata: %+v", got)
	}
	if !got.Rows.Valid || got.Rows.Int64 != 3 || string(got.Report) == "null" {
		t.Fatalf("expected done report fields, got %+v", got)
	}
}

func TestSREvaluationJobRepoMarkFailed(t *testing.T) {
	repo := newTestSREvaluationJobRepo(t)
	ctx := context.Background()

	if _, err := repo.Create(ctx, &SREvaluationJob{JobID: "sr_eval_job_002", Symbols: `["2330"]`}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if err := repo.MarkFailed(ctx, "sr_eval_job_002", "boom"); err != nil {
		t.Fatalf("MarkFailed failed: %v", err)
	}

	got, err := repo.Get(ctx, "sr_eval_job_002")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Status != "failed" || !got.Error.Valid || got.Error.String != "boom" {
		t.Fatalf("unexpected failed job: %+v", got)
	}
}

func TestSREvaluationJobRepoListOrdersNewestFirst(t *testing.T) {
	repo := newTestSREvaluationJobRepo(t)
	ctx := context.Background()

	for _, jobID := range []string{"sr_eval_job_003", "sr_eval_job_004", "sr_eval_job_005"} {
		if _, err := repo.Create(ctx, &SREvaluationJob{JobID: jobID, Symbols: `["2330"]`}); err != nil {
			t.Fatalf("Create %s failed: %v", jobID, err)
		}
	}

	jobs, err := repo.List(ctx, 2)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
	if jobs[0].JobID != "sr_eval_job_005" || jobs[1].JobID != "sr_eval_job_004" {
		t.Fatalf("expected newest first, got %+v", jobs)
	}
}
