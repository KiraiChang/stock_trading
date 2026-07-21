package store

import (
	"context"
	"encoding/json"

	"github.com/jmoiron/sqlx"
)

type SRRegressionResultRepo interface {
	Create(ctx context.Context, result *SRRegressionResult) (uint64, error)
	Get(ctx context.Context, runID string) (*SRRegressionResult, error)
	List(ctx context.Context, limit int) ([]SRRegressionResult, error)
}

type srRegressionResultRepo struct {
	db     *sqlx.DB
	driver string
}

func NewSRRegressionResultRepo(db *sqlx.DB) SRRegressionResultRepo {
	return &srRegressionResultRepo{db: db, driver: db.DriverName()}
}

const srRegressionResultColumns = `id, run_id, model_config_hash, pipeline_version, dataset_from, dataset_to, split_method,
	hold_auc, hold_brier_score, break_auc, break_brier_score, passed, metrics_json, created_at`

func (r *srRegressionResultRepo) Create(ctx context.Context, result *SRRegressionResult) (uint64, error) {
	if result.MetricsJSON == "" || !json.Valid([]byte(result.MetricsJSON)) {
		result.MetricsJSON = RawJSON("null")
	}

	if r.driver == "pgx" {
		var id uint64
		err := r.db.QueryRowxContext(ctx, `
			INSERT INTO stock_sr_regression_results (
				run_id, model_config_hash, pipeline_version, dataset_from, dataset_to, split_method,
				hold_auc, hold_brier_score, break_auc, break_brier_score, passed, metrics_json
			) VALUES (
				$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12
			)
			RETURNING id
		`,
			result.RunID, result.ModelConfigHash, result.PipelineVersion, result.DatasetFrom, result.DatasetTo, result.SplitMethod,
			result.HoldAUC, result.HoldBrierScore, result.BreakAUC, result.BreakBrierScore, result.Passed, result.MetricsJSON,
		).Scan(&id)
		return id, err
	}

	execResult, err := r.db.NamedExecContext(ctx, `
		INSERT INTO stock_sr_regression_results (
			run_id, model_config_hash, pipeline_version, dataset_from, dataset_to, split_method,
			hold_auc, hold_brier_score, break_auc, break_brier_score, passed, metrics_json
		) VALUES (
			:run_id, :model_config_hash, :pipeline_version, :dataset_from, :dataset_to, :split_method,
			:hold_auc, :hold_brier_score, :break_auc, :break_brier_score, :passed, :metrics_json
		)
	`, result)
	if err != nil {
		return 0, err
	}
	id, err := execResult.LastInsertId()
	if err != nil {
		return 0, err
	}
	return uint64(id), nil
}

func (r *srRegressionResultRepo) Get(ctx context.Context, runID string) (*SRRegressionResult, error) {
	var row SRRegressionResult
	err := r.db.GetContext(ctx, &row, r.db.Rebind(`
		SELECT `+srRegressionResultColumns+` FROM stock_sr_regression_results WHERE run_id=?
	`), runID)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *srRegressionResultRepo) List(ctx context.Context, limit int) ([]SRRegressionResult, error) {
	var rows []SRRegressionResult
	err := r.db.SelectContext(ctx, &rows, r.db.Rebind(`
		SELECT `+srRegressionResultColumns+` FROM stock_sr_regression_results ORDER BY created_at DESC, id DESC LIMIT ?
	`), limit)
	return rows, err
}
