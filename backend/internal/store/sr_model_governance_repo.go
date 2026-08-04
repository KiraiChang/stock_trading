package store

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
)

type SRModelGovernanceRepo interface {
	ListRange(ctx context.Context, symbol, timeframe string, from, to time.Time) ([]SRModelGovernance, error)
}

type srModelGovernanceRepo struct {
	db *sqlx.DB
}

func NewSRModelGovernanceRepo(db *sqlx.DB) SRModelGovernanceRepo {
	return &srModelGovernanceRepo{db: db}
}

func (r *srModelGovernanceRepo) ListRange(ctx context.Context, symbol, timeframe string, from, to time.Time) ([]SRModelGovernance, error) {
	var rows []SRModelGovernance
	err := r.db.SelectContext(ctx, &rows, r.db.Rebind(`
		SELECT id, analysis_id, symbol, timeframe, analyzed_at, model_version, model_config_hash,
			health_state, average_edge_pp, directional_zone_count, zone_count,
			allow_entry, max_entry_state, quality_flags, warning_flags, blocking_flags,
			confidence_gate_json, calibration_report_json, walk_forward_report_json,
			dataset_diagnostics_json, governance_json, created_at
		FROM stock_sr_model_governance
		WHERE symbol=? AND timeframe=? AND analyzed_at BETWEEN ? AND ?
		ORDER BY analyzed_at ASC, id ASC
	`), symbol, timeframe, from, to)
	return rows, err
}
