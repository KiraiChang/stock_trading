package store

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/jmoiron/sqlx"
)

type SRScoringTrainJobRepo interface {
	// Create 建立一筆 pending 狀態的任務，回傳新建的 id
	Create(ctx context.Context, job *SRScoringTrainJob) (uint64, error)
	MarkRunning(ctx context.Context, jobID string) error
	MarkDone(ctx context.Context, jobID string, rows, sources int, metrics RawJSON, modelPath, modelVersion, splitMethod string, datasetSummary RawJSON) error
	MarkFailed(ctx context.Context, jobID string, errMsg string) error
	Get(ctx context.Context, jobID string) (*SRScoringTrainJob, error)
	GetModelMetric(ctx context.Context, jobID string) (*SRModelMetric, error)
	List(ctx context.Context, limit int) ([]SRScoringTrainJob, error)
	PruneTerminal(ctx context.Context, keep int) (int64, error)
}

type srScoringTrainJobRepo struct {
	db     *sqlx.DB
	driver string
}

func NewSRScoringTrainJobRepo(db *sqlx.DB) SRScoringTrainJobRepo {
	return &srScoringTrainJobRepo{db: db, driver: db.DriverName()}
}

const srScoringTrainJobColumns = `id, job_id, status, symbols, timeframe, fetch_limit, model_type,
	row_count, sources, metrics, model_path, model_version, split_method, dataset_summary, error, started_at, finished_at, created_at`

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
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, tx.Rebind(`
		UPDATE sr_scoring_train_jobs
		SET status='done', row_count=?, sources=?, metrics=?, model_path=?, model_version=?, split_method=?, dataset_summary=?, finished_at=CURRENT_TIMESTAMP
		WHERE job_id=?
	`), rows, sources, metrics, modelPath, modelVersion, splitMethod, datasetSummary, jobID); err != nil {
		return err
	}

	var job struct {
		ID        uint64 `db:"id"`
		JobID     string `db:"job_id"`
		ModelType string `db:"model_type"`
		Timeframe string `db:"timeframe"`
		Rows      int64  `db:"row_count"`
		Sources   int64  `db:"sources"`
	}
	if err := tx.GetContext(ctx, &job, tx.Rebind(`
		SELECT id, job_id, model_type, timeframe, row_count, sources
		FROM sr_scoring_train_jobs WHERE job_id=?
	`), jobID); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, tx.Rebind(`DELETE FROM stock_sr_model_metrics WHERE job_id=?`), jobID); err != nil {
		return err
	}

	metric := buildSRModelMetric(job.ID, job.JobID, modelVersion, job.ModelType, splitMethod, job.Timeframe, job.Rows, job.Sources, metrics, datasetSummary)
	if _, err := tx.NamedExecContext(ctx, `
		INSERT INTO stock_sr_model_metrics (
			train_job_id, job_id, model_version, model_type, split_method, timeframe,
			row_count, sources, hold_auc, hold_brier_score, hold_log_loss, hold_calibrated, hold_test_rows,
			break_auc, break_brier_score, break_log_loss, break_calibrated, break_test_rows,
			metrics_json, dataset_summary_json
		) VALUES (
			:train_job_id, :job_id, :model_version, :model_type, :split_method, :timeframe,
			:row_count, :sources, :hold_auc, :hold_brier_score, :hold_log_loss, :hold_calibrated, :hold_test_rows,
			:break_auc, :break_brier_score, :break_log_loss, :break_calibrated, :break_test_rows,
			:metrics_json, :dataset_summary_json
		)
	`, metric); err != nil {
		return err
	}

	return tx.Commit()
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

func (r *srScoringTrainJobRepo) GetModelMetric(ctx context.Context, jobID string) (*SRModelMetric, error) {
	var row SRModelMetric
	err := r.db.GetContext(ctx, &row, r.db.Rebind(`
		SELECT id, train_job_id, job_id, model_version, model_type, split_method, timeframe,
			row_count, sources, hold_auc, hold_brier_score, hold_log_loss, hold_calibrated, hold_test_rows,
			break_auc, break_brier_score, break_log_loss, break_calibrated, break_test_rows,
			metrics_json, dataset_summary_json, created_at
		FROM stock_sr_model_metrics WHERE job_id=?
	`), jobID)
	if err != nil {
		return nil, err
	}
	return &row, nil
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

func (r *srScoringTrainJobRepo) PruneTerminal(ctx context.Context, keep int) (int64, error) {
	if keep < 0 {
		keep = 0
	}

	var ids []uint64
	if err := r.db.SelectContext(ctx, &ids, r.db.Rebind(`
		SELECT id
		FROM sr_scoring_train_jobs
		WHERE status IN ('done', 'failed')
		ORDER BY created_at DESC, id DESC
	`)); err != nil {
		return 0, err
	}
	if len(ids) <= keep {
		return 0, nil
	}

	var deleted int64
	for _, id := range ids[keep:] {
		if _, err := r.db.ExecContext(ctx, r.db.Rebind(`
			DELETE FROM stock_sr_model_metrics WHERE train_job_id=?
		`), id); err != nil {
			return deleted, err
		}
		result, err := r.db.ExecContext(ctx, r.db.Rebind(`
			DELETE FROM sr_scoring_train_jobs WHERE id=? AND status IN ('done', 'failed')
		`), id)
		if err != nil {
			return deleted, err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return deleted, err
		}
		deleted += n
	}
	return deleted, nil
}

func buildSRModelMetric(trainJobID uint64, jobID, modelVersion, modelType, splitMethod, timeframe string, rows, sources int64, metrics, datasetSummary RawJSON) SRModelMetric {
	var raw map[string]any
	if json.Valid([]byte(metrics)) {
		_ = json.Unmarshal([]byte(metrics), &raw)
	}
	return SRModelMetric{
		TrainJobID:         trainJobID,
		JobID:              jobID,
		ModelVersion:       modelVersion,
		ModelType:          modelType,
		SplitMethod:        splitMethod,
		Timeframe:          timeframe,
		Rows:               NullInt64{NullInt64: sql.NullInt64{Int64: rows, Valid: true}},
		Sources:            NullInt64{NullInt64: sql.NullInt64{Int64: sources, Valid: true}},
		HoldAUC:            metricFloat(raw, "hold", "auc"),
		HoldBrierScore:     metricFloat(raw, "hold", "brier_score"),
		HoldLogLoss:        metricFloat(raw, "hold", "log_loss"),
		HoldCalibrated:     metricBool(raw, "hold", "calibrated"),
		HoldTestRows:       metricInt(raw, "hold", "test_rows"),
		BreakAUC:           metricFloat(raw, "break", "auc"),
		BreakBrierScore:    metricFloat(raw, "break", "brier_score"),
		BreakLogLoss:       metricFloat(raw, "break", "log_loss"),
		BreakCalibrated:    metricBool(raw, "break", "calibrated"),
		BreakTestRows:      metricInt(raw, "break", "test_rows"),
		MetricsJSON:        rawJSONDefault(metrics, "null"),
		DatasetSummaryJSON: rawJSONDefault(datasetSummary, "null"),
	}
}

func metricValue(metrics map[string]any, model, key string) (any, bool) {
	if metrics == nil {
		return nil, false
	}
	modelMetrics, ok := metrics[model].(map[string]any)
	if !ok {
		return nil, false
	}
	value, ok := modelMetrics[key]
	return value, ok
}

func metricFloat(metrics map[string]any, model, key string) NullFloat64 {
	value, ok := metricValue(metrics, model, key)
	if !ok || value == nil {
		return NullFloat64{NullFloat64: sql.NullFloat64{}}
	}
	number, ok := value.(float64)
	if !ok {
		return NullFloat64{NullFloat64: sql.NullFloat64{}}
	}
	return NullFloat64{NullFloat64: sql.NullFloat64{Float64: number, Valid: true}}
}

func metricInt(metrics map[string]any, model, key string) NullInt64 {
	value, ok := metricValue(metrics, model, key)
	if !ok || value == nil {
		return NullInt64{NullInt64: sql.NullInt64{}}
	}
	number, ok := value.(float64)
	if !ok {
		return NullInt64{NullInt64: sql.NullInt64{}}
	}
	return NullInt64{NullInt64: sql.NullInt64{Int64: int64(number), Valid: true}}
}

func metricBool(metrics map[string]any, model, key string) NullBool {
	value, ok := metricValue(metrics, model, key)
	if !ok || value == nil {
		return NullBool{NullBool: sql.NullBool{}}
	}
	switch typed := value.(type) {
	case bool:
		return NullBool{NullBool: sql.NullBool{Bool: typed, Valid: true}}
	case float64:
		return NullBool{NullBool: sql.NullBool{Bool: typed != 0, Valid: true}}
	default:
		return NullBool{NullBool: sql.NullBool{}}
	}
}

func rawJSONDefault(value RawJSON, fallback string) RawJSON {
	if value == "" || !json.Valid([]byte(value)) {
		return RawJSON(fallback)
	}
	return value
}
