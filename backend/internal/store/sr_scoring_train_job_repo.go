package store

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type SRScoringTrainJobRepo interface {
	// Create 建立一筆 pending 狀態的任務，回傳新建的 id
	Create(ctx context.Context, job *SRScoringTrainJob) (uint64, error)
	MarkRunning(ctx context.Context, jobID string) error
	MarkDone(ctx context.Context, jobID string, rows, sources int, metrics RawJSON, modelPath, modelVersion, splitMethod string, datasetSummary RawJSON) error
	MarkFailed(ctx context.Context, jobID string, errMsg string) error
	Get(ctx context.Context, jobID string) (*SRScoringTrainJob, error)
	List(ctx context.Context, limit int) ([]SRScoringTrainJob, error)
}

type srScoringTrainJobRepo struct {
	db     *sqlx.DB
	driver string
}

func NewSRScoringTrainJobRepo(db *sqlx.DB) SRScoringTrainJobRepo {
	return &srScoringTrainJobRepo{db: db, driver: db.DriverName()}
}

const srScoringTrainJobColumns = `id, job_id, status, symbols, timeframe, fetch_limit, model_type,
	rows, sources, metrics, model_path, model_version, split_method, dataset_summary, error, started_at, finished_at, created_at`

func (r *srScoringTrainJobRepo) Create(ctx context.Context, job *SRScoringTrainJob) (uint64, error) {
	if job.Status == "" {
		job.Status = "pending"
	}

	if r.driver == "pgx" {
		// pgx（postgres）不支援 LastInsertId，需改用 RETURNING id
		var id uint64
		err := r.db.QueryRowContext(ctx, `
			INSERT INTO sr_scoring_train_jobs (job_id, status, symbols, timeframe, fetch_limit, model_type)
			VALUES ($1,$2,$3,$4,$5,$6)
			RETURNING id
		`, job.JobID, job.Status, job.Symbols, job.Timeframe, job.FetchLimit, job.ModelType).Scan(&id)
		return id, err
	}

	result, err := r.db.ExecContext(ctx, r.db.Rebind(`
		INSERT INTO sr_scoring_train_jobs (job_id, status, symbols, timeframe, fetch_limit, model_type)
		VALUES (?,?,?,?,?,?)
	`), job.JobID, job.Status, job.Symbols, job.Timeframe, job.FetchLimit, job.ModelType)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return uint64(id), nil
}

func (r *srScoringTrainJobRepo) MarkRunning(ctx context.Context, jobID string) error {
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`
		UPDATE sr_scoring_train_jobs SET status='running', started_at=CURRENT_TIMESTAMP WHERE job_id=?
	`), jobID)
	return err
}

func (r *srScoringTrainJobRepo) MarkDone(ctx context.Context, jobID string, rows, sources int, metrics RawJSON, modelPath, modelVersion, splitMethod string, datasetSummary RawJSON) error {
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`
		UPDATE sr_scoring_train_jobs
		SET status='done', rows=?, sources=?, metrics=?, model_path=?, model_version=?, split_method=?, dataset_summary=?, finished_at=CURRENT_TIMESTAMP
		WHERE job_id=?
	`), rows, sources, metrics, modelPath, modelVersion, splitMethod, datasetSummary, jobID)
	return err
}

func (r *srScoringTrainJobRepo) MarkFailed(ctx context.Context, jobID string, errMsg string) error {
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`
		UPDATE sr_scoring_train_jobs SET status='failed', error=?, finished_at=CURRENT_TIMESTAMP WHERE job_id=?
	`), errMsg, jobID)
	return err
}

func (r *srScoringTrainJobRepo) Get(ctx context.Context, jobID string) (*SRScoringTrainJob, error) {
	var job SRScoringTrainJob
	err := r.db.GetContext(ctx, &job, r.db.Rebind(`
		SELECT `+srScoringTrainJobColumns+` FROM sr_scoring_train_jobs WHERE job_id=?
	`), jobID)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *srScoringTrainJobRepo) List(ctx context.Context, limit int) ([]SRScoringTrainJob, error) {
	var jobs []SRScoringTrainJob
	// id DESC 當 created_at 的 tiebreaker：同一秒內連續建立多筆任務時
	// （sqlite CURRENT_TIMESTAMP 只有秒級精度），單靠 created_at 排序順序
	// 不確定，id 是嚴格遞增的，能確保「最新建立的排最前面」。
	err := r.db.SelectContext(ctx, &jobs, r.db.Rebind(`
		SELECT `+srScoringTrainJobColumns+` FROM sr_scoring_train_jobs ORDER BY created_at DESC, id DESC LIMIT ?
	`), limit)
	return jobs, err
}
