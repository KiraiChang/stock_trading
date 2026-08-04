package store

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/jmoiron/sqlx"
)

type SRRegressionResultRepo interface {
	Create(ctx context.Context, result *SRRegressionResult) (uint64, error)
	Get(ctx context.Context, runID string) (*SRRegressionResult, error)
	List(ctx context.Context, limit int) ([]SRRegressionResult, error)
	ListBySchemaVersion(ctx context.Context, schemaVersion string, limit int) ([]SRRegressionResult, error)
}

type srRegressionResultRepo struct {
	db     *sqlx.DB
	driver string
}

func NewSRRegressionResultRepo(db *sqlx.DB) SRRegressionResultRepo {
	return &srRegressionResultRepo{db: db, driver: db.DriverName()}
}

const srRegressionResultColumns = `id, run_id, model_config_hash, pipeline_version, dataset_from, dataset_to, split_method,
	hold_auc, hold_brier_score, break_auc, break_brier_score, passed,
	schema_version, result_rows, source_count, governance_health_state, governance_strict_passed,
	metrics_json, created_at`

func (r *srRegressionResultRepo) Create(ctx context.Context, result *SRRegressionResult) (uint64, error) {
	if result.MetricsJSON == "" || !json.Valid([]byte(result.MetricsJSON)) {
		result.MetricsJSON = RawJSON("null")
	}
	normalizeSRRegressionResult(result)

	if r.driver == "pgx" {
		var id uint64
		err := r.db.QueryRowxContext(ctx, `
			INSERT INTO stock_sr_regression_results (
				run_id, model_config_hash, pipeline_version, dataset_from, dataset_to, split_method,
				hold_auc, hold_brier_score, break_auc, break_brier_score, passed,
				schema_version, result_rows, source_count, governance_health_state, governance_strict_passed, metrics_json
			) VALUES (
				$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17
			)
			RETURNING id
		`,
			result.RunID, result.ModelConfigHash, result.PipelineVersion, result.DatasetFrom, result.DatasetTo, result.SplitMethod,
			result.HoldAUC, result.HoldBrierScore, result.BreakAUC, result.BreakBrierScore, result.Passed,
			result.SchemaVersion, result.Rows, result.Sources, result.GovernanceHealthState, result.GovernanceStrictPassed, result.MetricsJSON,
		).Scan(&id)
		return id, err
	}

	execResult, err := r.db.NamedExecContext(ctx, `
		INSERT INTO stock_sr_regression_results (
			run_id, model_config_hash, pipeline_version, dataset_from, dataset_to, split_method,
			hold_auc, hold_brier_score, break_auc, break_brier_score, passed,
			schema_version, result_rows, source_count, governance_health_state, governance_strict_passed, metrics_json
		) VALUES (
			:run_id, :model_config_hash, :pipeline_version, :dataset_from, :dataset_to, :split_method,
			:hold_auc, :hold_brier_score, :break_auc, :break_brier_score, :passed,
			:schema_version, :result_rows, :source_count, :governance_health_state, :governance_strict_passed, :metrics_json
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

func (r *srRegressionResultRepo) ListBySchemaVersion(ctx context.Context, schemaVersion string, limit int) ([]SRRegressionResult, error) {
	if schemaVersion == "" {
		return r.List(ctx, limit)
	}

	var rows []SRRegressionResult
	switch r.driver {
	case "pgx":
		err := r.db.SelectContext(ctx, &rows, `
			SELECT `+srRegressionResultColumns+` FROM stock_sr_regression_results
			WHERE schema_version = $1 OR metrics_json->>'schema_version' = $1
			ORDER BY created_at DESC, id DESC LIMIT $2
		`, schemaVersion, limit)
		return rows, err
	case "sqlite":
		err := r.db.SelectContext(ctx, &rows, `
			SELECT `+srRegressionResultColumns+` FROM stock_sr_regression_results
			WHERE schema_version = ? OR json_extract(metrics_json, '$.schema_version') = ?
			ORDER BY created_at DESC, id DESC LIMIT ?
		`, schemaVersion, schemaVersion, limit)
		return rows, err
	default:
		err := r.db.SelectContext(ctx, &rows, r.db.Rebind(`
			SELECT `+srRegressionResultColumns+` FROM stock_sr_regression_results
			WHERE schema_version = ? OR JSON_UNQUOTE(JSON_EXTRACT(metrics_json, '$.schema_version')) = ?
			ORDER BY created_at DESC, id DESC LIMIT ?
		`), schemaVersion, schemaVersion, limit)
		return rows, err
	}
}

func normalizeSRRegressionResult(result *SRRegressionResult) {
	var report map[string]any
	if err := json.Unmarshal([]byte(result.MetricsJSON), &report); err != nil {
		return
	}
	if result.SchemaVersion == "" {
		result.SchemaVersion = stringValue(report["schema_version"])
	}
	if !result.Rows.Valid {
		result.Rows = nullIntFromAny(report["rows"])
	}
	if !result.Sources.Valid {
		result.Sources = nullIntFromAny(report["sources"])
	}
	governance, _ := report["governance_evaluation"].(map[string]any)
	if result.GovernanceHealthState == "" {
		result.GovernanceHealthState = stringValue(governance["health_state"])
	}
	if !result.GovernanceStrictPassed.Valid {
		result.GovernanceStrictPassed = nullBoolFromAny(governance["strict_passed"])
	}
}

func stringValue(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func nullIntFromAny(value any) NullInt64 {
	switch v := value.(type) {
	case float64:
		return NullInt64{NullInt64: sql.NullInt64{Int64: int64(v), Valid: true}}
	case int:
		return NullInt64{NullInt64: sql.NullInt64{Int64: int64(v), Valid: true}}
	case int64:
		return NullInt64{NullInt64: sql.NullInt64{Int64: v, Valid: true}}
	default:
		return NullInt64{}
	}
}

func nullBoolFromAny(value any) NullBool {
	if b, ok := value.(bool); ok {
		return NullBool{NullBool: sql.NullBool{Bool: b, Valid: true}}
	}
	return NullBool{}
}
