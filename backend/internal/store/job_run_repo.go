package store

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
)

type JobRunRepo interface {
	Start(ctx context.Context, jobName string) (uint64, error)
	Finish(ctx context.Context, runID uint64, status string, symbolsTotal, symbolsFailed int, errMsg string) error
	GetRecent(ctx context.Context, limit int) ([]JobRun, error)
	DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error)
}

type jobRunRepo struct {
	db     *sqlx.DB
	driver string
}

func NewJobRunRepo(db *sqlx.DB) JobRunRepo {
	return &jobRunRepo{db: db, driver: db.DriverName()}
}

func (r *jobRunRepo) Start(ctx context.Context, jobName string) (uint64, error) {
	// pgx（postgres）不支援 LastInsertId，需改用 RETURNING id
	if r.driver == "pgx" {
		var id uint64
		err := r.db.QueryRowContext(ctx, `
			INSERT INTO job_runs (job_name, status) VALUES ($1, 'running') RETURNING id
		`, jobName).Scan(&id)
		return id, err
	}

	result, err := r.db.ExecContext(ctx, r.db.Rebind(`
		INSERT INTO job_runs (job_name, status) VALUES (?, 'running')
	`), jobName)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return uint64(id), nil
}

func (r *jobRunRepo) Finish(ctx context.Context, runID uint64, status string, symbolsTotal, symbolsFailed int, errMsg string) error {
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`
		UPDATE job_runs
		SET status=?, symbols_total=?, symbols_failed=?, error=?, finished_at=CURRENT_TIMESTAMP
		WHERE id=?
	`), status, symbolsTotal, symbolsFailed, errMsg, runID)
	return err
}

func (r *jobRunRepo) GetRecent(ctx context.Context, limit int) ([]JobRun, error) {
	var rows []JobRun
	err := r.db.SelectContext(ctx, &rows, r.db.Rebind(`
		SELECT id, job_name, status, symbols_total, symbols_failed, error, started_at, finished_at
		FROM job_runs
		ORDER BY started_at DESC
		LIMIT ?
	`), limit)
	return rows, err
}

// DeleteBefore 刪除 cutoff 之前的執行紀錄，用於只保留當天資料
func (r *jobRunRepo) DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	result, err := r.db.ExecContext(ctx, r.db.Rebind(`
		DELETE FROM job_runs WHERE started_at < ?
	`), cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
