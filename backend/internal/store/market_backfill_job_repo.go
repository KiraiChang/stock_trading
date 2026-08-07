package store

import (
	"context"

	"github.com/jmoiron/sqlx"
)

// MarketBackfillJobRepo 與 ChipSyncJobRepo 是同一套形狀，刻意保持一致：
// 前端同一個頁面上兩塊 UI（籌碼回補 / 股價回補）用同樣的輪詢流程。
// 不提供 ListRecent——籌碼那邊有但前端沒用到，先不做以免長出無人消費的介面。
type MarketBackfillJobRepo interface {
	// Create 建立一筆 pending 狀態的任務
	Create(ctx context.Context, job *MarketBackfillJob) error
	UpdateProgress(ctx context.Context, jobID string, done, failed int, failures RawJSON) error
	Finish(ctx context.Context, jobID, status, errMsg string) error
	GetByJobID(ctx context.Context, jobID string) (*MarketBackfillJob, error)
}

type marketBackfillJobRepo struct {
	db     *sqlx.DB
	driver string
}

func NewMarketBackfillJobRepo(db *sqlx.DB) MarketBackfillJobRepo {
	return &marketBackfillJobRepo{db: db, driver: db.DriverName()}
}

const marketBackfillJobColumns = `id, job_id, symbols, days, status,
	symbols_total, symbols_done, symbols_failed, failures, error, started_at, finished_at, created_at`

func (r *marketBackfillJobRepo) Create(ctx context.Context, job *MarketBackfillJob) error {
	if job.Status == "" {
		job.Status = "pending"
	}
	if job.Failures == "" {
		job.Failures = "[]"
	}

	if r.driver == "pgx" {
		// pgx（postgres）不支援 LastInsertId，需改用 RETURNING id
		return r.db.QueryRowContext(ctx, `
			INSERT INTO market_backfill_jobs (job_id, symbols, days, status, symbols_total)
			VALUES ($1,$2,$3,$4,$5)
			RETURNING id
		`, job.JobID, job.Symbols, job.Days, job.Status, job.SymbolsTotal).Scan(&job.ID)
	}

	result, err := r.db.ExecContext(ctx, r.db.Rebind(`
		INSERT INTO market_backfill_jobs (job_id, symbols, days, status, symbols_total)
		VALUES (?,?,?,?,?)
	`), job.JobID, job.Symbols, job.Days, job.Status, job.SymbolsTotal)
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

func (r *marketBackfillJobRepo) UpdateProgress(ctx context.Context, jobID string, done, failed int, failures RawJSON) error {
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`
		UPDATE market_backfill_jobs
		SET status='running', symbols_done=?, symbols_failed=?, failures=?, started_at=COALESCE(started_at, CURRENT_TIMESTAMP)
		WHERE job_id=?
	`), done, failed, failures, jobID)
	return err
}

func (r *marketBackfillJobRepo) Finish(ctx context.Context, jobID, status, errMsg string) error {
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`
		UPDATE market_backfill_jobs SET status=?, error=?, finished_at=CURRENT_TIMESTAMP WHERE job_id=?
	`), status, errMsg, jobID)
	return err
}

func (r *marketBackfillJobRepo) GetByJobID(ctx context.Context, jobID string) (*MarketBackfillJob, error) {
	var job MarketBackfillJob
	err := r.db.GetContext(ctx, &job, r.db.Rebind(`
		SELECT `+marketBackfillJobColumns+` FROM market_backfill_jobs WHERE job_id=?
	`), jobID)
	if err != nil {
		return nil, err
	}
	return &job, nil
}
