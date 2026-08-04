package store

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/jmoiron/sqlx"
)

type SREvaluationJobRepo interface {
	Create(ctx context.Context, job *SREvaluationJob) (uint64, error)
	MarkRunning(ctx context.Context, jobID string) error
	MarkDone(ctx context.Context, jobID string, report RawJSON, runID, schemaVersion, pipelineVersion string, rows, sources int) error
	MarkFailed(ctx context.Context, jobID string, errMsg string) error
	Get(ctx context.Context, jobID string) (*SREvaluationJob, error)
	List(ctx context.Context, limit int) ([]SREvaluationJob, error)
}

type srEvaluationJobRepo struct {
	db     *sqlx.DB
	driver string
}

func NewSREvaluationJobRepo(db *sqlx.DB) SREvaluationJobRepo {
	return &srEvaluationJobRepo{db: db, driver: db.DriverName()}
}

// 欄位刻意命名為 result_rows / source_count 而非 rows / sources：ROWS 自 MySQL 8.0.2
// 起是保留字（window function 語法），當識別字必須加 backtick，否則 CREATE TABLE 與
// SELECT 都會 syntax error。stock_sr_regression_results 的同義欄位也是這組命名。
// 對外 JSON 仍是 rows / sources，由 model 的 json tag 轉換。
const srEvaluationJobColumns = `id, job_id, status, symbols, timeframe, fetch_limit, mode, write_db, replay_max_rows,
	run_id, schema_version, pipeline_version, result_rows, source_count, report, error, started_at, finished_at, created_at`

func (r *srEvaluationJobRepo) Create(ctx context.Context, job *SREvaluationJob) (uint64, error) {
	if job.Status == "" {
		job.Status = "pending"
	}
	if job.Symbols == "" || !json.Valid([]byte(job.Symbols)) {
		job.Symbols = "[]"
	}
	if job.Report == "" || !json.Valid([]byte(job.Report)) {
		job.Report = RawJSON("null")
	}
	if job.Timeframe == "" {
		job.Timeframe = "1d"
	}
	if job.Mode == "" {
		job.Mode = "evaluation"
	}

	if r.driver == "pgx" {
		var id uint64
		err := r.db.QueryRowContext(ctx, `
			INSERT INTO sr_evaluation_jobs (job_id, status, symbols, timeframe, fetch_limit, mode, write_db, replay_max_rows, report)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			RETURNING id
		`, job.JobID, job.Status, job.Symbols, job.Timeframe, job.FetchLimit, job.Mode, job.WriteDB, job.ReplayMaxRows, job.Report).Scan(&id)
		return id, err
	}

	result, err := r.db.ExecContext(ctx, r.db.Rebind(`
		INSERT INTO sr_evaluation_jobs (job_id, status, symbols, timeframe, fetch_limit, mode, write_db, replay_max_rows, report)
		VALUES (?,?,?,?,?,?,?,?,?)
	`), job.JobID, job.Status, job.Symbols, job.Timeframe, job.FetchLimit, job.Mode, job.WriteDB, job.ReplayMaxRows, job.Report)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return uint64(id), nil
}

func (r *srEvaluationJobRepo) MarkRunning(ctx context.Context, jobID string) error {
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`
		UPDATE sr_evaluation_jobs SET status='running', started_at=CURRENT_TIMESTAMP WHERE job_id=?
	`), jobID)
	return err
}

func (r *srEvaluationJobRepo) MarkDone(ctx context.Context, jobID string, report RawJSON, runID, schemaVersion, pipelineVersion string, rows, sources int) error {
	if report == "" || !json.Valid([]byte(report)) {
		report = RawJSON("null")
	}
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`
		UPDATE sr_evaluation_jobs
		SET status='done', report=?, run_id=?, schema_version=?, pipeline_version=?, result_rows=?, source_count=?, error=NULL, finished_at=CURRENT_TIMESTAMP
		WHERE job_id=?
	`), report, nullStringValue(runID), nullStringValue(schemaVersion), nullStringValue(pipelineVersion), nullIntValue(rows), nullIntValue(sources), jobID)
	return err
}

func (r *srEvaluationJobRepo) MarkFailed(ctx context.Context, jobID string, errMsg string) error {
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`
		UPDATE sr_evaluation_jobs SET status='failed', error=?, finished_at=CURRENT_TIMESTAMP WHERE job_id=?
	`), errMsg, jobID)
	return err
}

func (r *srEvaluationJobRepo) Get(ctx context.Context, jobID string) (*SREvaluationJob, error) {
	var job SREvaluationJob
	err := r.db.GetContext(ctx, &job, r.db.Rebind(`
		SELECT `+srEvaluationJobColumns+` FROM sr_evaluation_jobs WHERE job_id=?
	`), jobID)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *srEvaluationJobRepo) List(ctx context.Context, limit int) ([]SREvaluationJob, error) {
	var jobs []SREvaluationJob
	err := r.db.SelectContext(ctx, &jobs, r.db.Rebind(`
		SELECT `+srEvaluationJobColumns+` FROM sr_evaluation_jobs ORDER BY created_at DESC, id DESC LIMIT ?
	`), limit)
	return jobs, err
}

func nullStringValue(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func nullIntValue(value int) sql.NullInt64 {
	return sql.NullInt64{Int64: int64(value), Valid: value > 0}
}
