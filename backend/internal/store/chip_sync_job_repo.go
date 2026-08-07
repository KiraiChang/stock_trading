package store

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type ChipSyncJobRepo interface {
	// Create 建立一筆 pending 狀態的任務
	Create(ctx context.Context, job *ChipSyncJob) error
	UpdateProgress(ctx context.Context, jobID string, done, failed int, failures RawJSON) error
	Finish(ctx context.Context, jobID, status, errMsg string) error
	GetByJobID(ctx context.Context, jobID string) (*ChipSyncJob, error)
	ListRecent(ctx context.Context, limit int) ([]ChipSyncJob, error)
}

type chipSyncJobRepo struct {
	db     *sqlx.DB
	driver string
}

func NewChipSyncJobRepo(db *sqlx.DB) ChipSyncJobRepo {
	return &chipSyncJobRepo{db: db, driver: db.DriverName()}
}

const chipSyncJobColumns = `id, job_id, mode, symbols, data_types, from_date, to_date, force_sync, status,
	symbols_total, symbols_done, symbols_failed, failures, error, started_at, finished_at, created_at`

func (r *chipSyncJobRepo) Create(ctx context.Context, job *ChipSyncJob) error {
	if job.Status == "" {
		job.Status = "pending"
	}
	if job.Failures == "" {
		job.Failures = "[]"
	}

	if r.driver == "pgx" {
		// pgx（postgres）不支援 LastInsertId，需改用 RETURNING id
		return r.db.QueryRowContext(ctx, `
			INSERT INTO chip_sync_jobs (job_id, mode, symbols, data_types, from_date, to_date, force_sync, status, symbols_total)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			RETURNING id
		`, job.JobID, job.Mode, job.Symbols, job.DataTypes, job.FromDate, job.ToDate, job.Force, job.Status, job.SymbolsTotal).Scan(&job.ID)
	}

	result, err := r.db.ExecContext(ctx, r.db.Rebind(`
		INSERT INTO chip_sync_jobs (job_id, mode, symbols, data_types, from_date, to_date, force_sync, status, symbols_total)
		VALUES (?,?,?,?,?,?,?,?,?)
	`), job.JobID, job.Mode, job.Symbols, job.DataTypes, job.FromDate, job.ToDate, job.Force, job.Status, job.SymbolsTotal)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	job.ID = uint64(id)
	return nil
}

func (r *chipSyncJobRepo) UpdateProgress(ctx context.Context, jobID string, done, failed int, failures RawJSON) error {
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`
		UPDATE chip_sync_jobs
		SET status='running', symbols_done=?, symbols_failed=?, failures=?, started_at=COALESCE(started_at, CURRENT_TIMESTAMP)
		WHERE job_id=?
	`), done, failed, failures, jobID)
	return err
}

func (r *chipSyncJobRepo) Finish(ctx context.Context, jobID, status, errMsg string) error {
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`
		UPDATE chip_sync_jobs SET status=?, error=?, finished_at=CURRENT_TIMESTAMP WHERE job_id=?
	`), status, errMsg, jobID)
	return err
}

func (r *chipSyncJobRepo) GetByJobID(ctx context.Context, jobID string) (*ChipSyncJob, error) {
	var job ChipSyncJob
	err := r.db.GetContext(ctx, &job, r.db.Rebind(`
		SELECT `+chipSyncJobColumns+` FROM chip_sync_jobs WHERE job_id=?
	`), jobID)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *chipSyncJobRepo) ListRecent(ctx context.Context, limit int) ([]ChipSyncJob, error) {
	var jobs []ChipSyncJob
	err := r.db.SelectContext(ctx, &jobs, r.db.Rebind(`
		SELECT `+chipSyncJobColumns+` FROM chip_sync_jobs ORDER BY created_at DESC, id DESC LIMIT ?
	`), limit)
	return jobs, err
}
