package store

import (
	"context"
	"database/sql"
	"os"
	"sync"
	"testing"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/database"
)

// 回測取消的條件式更新（I-134）：只有 pending 才能取消；跟 python 的認領同時發生時只有一方成功。

func backtestRepoTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	tmp, err := os.CreateTemp("", "backtest-repo-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	t.Cleanup(func() { os.Remove(tmp.Name()) })

	db, err := NewDB(config.DatabaseConfig{Driver: "sqlite", DSN: tmp.Name()})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.RunMigrations(context.Background(), db, "sqlite", zap.NewNop()); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	return db
}

func insertBacktestJob(t *testing.T, db *sqlx.DB, jobID, status string, owner any) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO backtest_jobs
		(job_id, type, strategy, symbols, timeframe, start_date, end_date, status, trigger_source, execution_owner)
		VALUES (?, 'backtest', 'breakout_v1', '["2330"]', '1d', '2026-01-01', '2026-06-30', ?, 'manual', ?)`,
		jobID, status, owner)
	if err != nil {
		t.Fatalf("insert job: %v", err)
	}
}

type backtestJobState struct {
	Status     string         `db:"status"`
	Error      sql.NullString `db:"error"`
	Owner      sql.NullString `db:"execution_owner"`
	FinishedAt sql.NullString `db:"finished_at"`
}

func readBacktestJobState(t *testing.T, db *sqlx.DB, jobID string) backtestJobState {
	t.Helper()
	var s backtestJobState
	if err := db.Get(&s, `SELECT status, error, execution_owner, CAST(finished_at AS TEXT) AS finished_at
		FROM backtest_jobs WHERE job_id=?`, jobID); err != nil {
		t.Fatalf("read job: %v", err)
	}
	return s
}

func TestCancelPendingJobCancelsOnlyPending(t *testing.T) {
	db := backtestRepoTestDB(t)
	repo := NewBacktestRepo(db)
	ctx := context.Background()
	insertBacktestJob(t, db, "j1", "pending", nil)

	ok, err := repo.CancelPendingJob(ctx, "j1")
	if err != nil || !ok {
		t.Fatalf("cancel pending: ok=%v err=%v", ok, err)
	}
	got := readBacktestJobState(t, db, "j1")
	if got.Status != "failed" || got.Error.String != CancelUserMessage || !got.FinishedAt.Valid {
		t.Fatalf("after cancel: %+v", got)
	}
}

func TestCancelPendingJobLeavesNonPendingUntouched(t *testing.T) {
	cases := []struct {
		status string
		owner  any
	}{
		{"running", "server"},
		{"running", "worker"},
		{"done", "worker"},
		{"failed", nil},
	}
	for _, tc := range cases {
		t.Run(tc.status, func(t *testing.T) {
			db := backtestRepoTestDB(t)
			repo := NewBacktestRepo(db)
			insertBacktestJob(t, db, "j1", tc.status, tc.owner)
			before := readBacktestJobState(t, db, "j1")

			ok, err := repo.CancelPendingJob(context.Background(), "j1")
			if err != nil || ok {
				t.Fatalf("cancel %s: ok=%v err=%v", tc.status, ok, err)
			}
			if after := readBacktestJobState(t, db, "j1"); after != before {
				t.Fatalf("row changed: before=%+v after=%+v", before, after)
			}
		})
	}
}

func TestCancelPendingJobMissing(t *testing.T) {
	repo := NewBacktestRepo(backtestRepoTestDB(t))
	ok, err := repo.CancelPendingJob(context.Background(), "nope")
	if err != nil || ok {
		t.Fatalf("missing: ok=%v err=%v", ok, err)
	}
}

// python 的認領 SQL（python/backtest/db_writer.py 的 claim_job），用來驗兩邊同時發生時只有一方成功。
const pythonClaimSQL = `UPDATE backtest_jobs
	SET status='running', execution_owner=?, started_at=CURRENT_TIMESTAMP
	WHERE job_id=? AND status='pending'`

func TestCancelRacingClaimOnlyOneWins(t *testing.T) {
	db := backtestRepoTestDB(t)
	repo := NewBacktestRepo(db)
	ctx := context.Background()
	const n = 30
	for i := 0; i < n; i++ {
		insertBacktestJob(t, db, jobName(i), "pending", nil)
	}
	for i := 0; i < n; i++ {
		id := jobName(i)
		var wg sync.WaitGroup
		start := make(chan struct{})
		var cancelled bool
		var cancelErr, claimErr error
		var claimed int64
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			cancelled, cancelErr = repo.CancelPendingJob(ctx, id)
		}()
		go func() {
			defer wg.Done()
			<-start
			res, err := db.ExecContext(ctx, pythonClaimSQL, "server", id)
			if err != nil {
				claimErr = err
				return
			}
			claimed, claimErr = res.RowsAffected()
		}()
		close(start)
		wg.Wait()
		if cancelErr != nil || claimErr != nil {
			t.Fatalf("%s: cancelErr=%v claimErr=%v", id, cancelErr, claimErr)
		}
		won := 0
		if cancelled {
			won++
		}
		won += int(claimed)
		if won != 1 {
			t.Fatalf("%s: cancelled=%v claimed=%d", id, cancelled, claimed)
		}
		got := readBacktestJobState(t, db, id)
		if cancelled && got.Status != "failed" {
			t.Fatalf("%s: cancel won but status=%s", id, got.Status)
		}
		if claimed == 1 && (got.Status != "running" || got.Owner.String != "server") {
			t.Fatalf("%s: claim won but state=%+v", id, got)
		}
	}
}

func jobName(i int) string {
	return "race" + string(rune('a'+i/26)) + string(rune('a'+i%26))
}

// pending／running 的 error 是 NULL（backend 建立、python 認領都不寫），GetJob／ListJobs 要讀得到。
func TestGetAndListJobsWithNullError(t *testing.T) {
	db := backtestRepoTestDB(t)
	repo := NewBacktestRepo(db)
	ctx := context.Background()
	insertBacktestJob(t, db, "p1", "pending", nil)
	insertBacktestJob(t, db, "r1", "running", "server")
	if _, err := db.Exec(`UPDATE backtest_jobs SET error='boom' WHERE job_id='r1'`); err != nil {
		t.Fatal(err)
	}
	insertBacktestJob(t, db, "r2", "running", "worker")

	for _, id := range []string{"p1", "r2"} {
		job, err := repo.GetJob(ctx, id)
		if err != nil {
			t.Fatalf("get %s: %v", id, err)
		}
		if job.Error != "" {
			t.Fatalf("%s: error=%q", id, job.Error)
		}
	}
	if job, err := repo.GetJob(ctx, "r1"); err != nil || job.Error != "boom" {
		t.Fatalf("r1: %v %+v", err, job)
	}
	jobs, err := repo.ListJobs(ctx, 10)
	if err != nil || len(jobs) != 3 {
		t.Fatalf("list: err=%v n=%d", err, len(jobs))
	}
}

// GetJob／ListJobs 明列欄位：077 加的 execution_owner ⛔ 不會進到 BacktestJob。
func TestGetJobIgnoresExecutionOwner(t *testing.T) {
	db := backtestRepoTestDB(t)
	repo := NewBacktestRepo(db)
	insertBacktestJob(t, db, "j1", "running", "worker")

	job, err := repo.GetJob(context.Background(), "j1")
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if job.Status != "running" || job.Trigger != "manual" {
		t.Fatalf("job: %+v", job)
	}
	jobs, err := repo.ListJobs(context.Background(), 10)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("list: %v %d", err, len(jobs))
	}
}
