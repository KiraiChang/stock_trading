package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/jmoiron/sqlx"
)

type HoldingRepo interface {
	Create(ctx context.Context, h *Holding) (uint64, error)
	Update(ctx context.Context, h *Holding) error
	Delete(ctx context.Context, id uint64) error
	Get(ctx context.Context, id uint64) (*Holding, error)
	List(ctx context.Context) ([]Holding, error)
	CreateAnalysis(ctx context.Context, a *HoldingAnalysis) (uint64, error)
	GetAnalysis(ctx context.Context, id uint64) (*HoldingAnalysis, error)
	ListAnalyses(ctx context.Context, holdingID uint64, limit int) ([]HoldingAnalysis, error)
}

type holdingRepo struct {
	db     *sqlx.DB
	driver string
}

func NewHoldingRepo(db *sqlx.DB) HoldingRepo {
	return &holdingRepo{db: db, driver: db.DriverName()}
}

const holdingColumns = `id, symbol, shares, cost_price, note, created_at, updated_at`

func (r *holdingRepo) Create(ctx context.Context, h *Holding) (uint64, error) {
	var id uint64
	if r.driver == "pgx" {
		err := r.db.QueryRowContext(ctx, `
			INSERT INTO holdings (symbol, shares, cost_price, note)
			VALUES ($1,$2,$3,$4)
			RETURNING id
		`, h.Symbol, h.Shares, h.CostPrice, h.Note).Scan(&id)
		return id, err
	}
	res, err := r.db.ExecContext(ctx, r.db.Rebind(`
		INSERT INTO holdings (symbol, shares, cost_price, note)
		VALUES (?,?,?,?)
	`), h.Symbol, h.Shares, h.CostPrice, h.Note)
	if err != nil {
		return 0, err
	}
	lastID, err := res.LastInsertId()
	return uint64(lastID), err
}

func (r *holdingRepo) Update(ctx context.Context, h *Holding) error {
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`
		UPDATE holdings
		SET symbol=?, shares=?, cost_price=?, note=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=?
	`), h.Symbol, h.Shares, h.CostPrice, h.Note, h.ID)
	return err
}

func (r *holdingRepo) Delete(ctx context.Context, id uint64) error {
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`DELETE FROM holdings WHERE id=?`), id)
	return err
}

func (r *holdingRepo) Get(ctx context.Context, id uint64) (*Holding, error) {
	var h Holding
	err := r.db.GetContext(ctx, &h, r.db.Rebind(`
		SELECT `+holdingColumns+` FROM holdings WHERE id=?
	`), id)
	if err != nil {
		return nil, err
	}
	return &h, nil
}

func (r *holdingRepo) List(ctx context.Context) ([]Holding, error) {
	var rows []Holding
	err := r.db.SelectContext(ctx, &rows, `
		SELECT `+holdingColumns+` FROM holdings ORDER BY updated_at DESC, id DESC
	`)
	return rows, err
}

const holdingAnalysisColumns = `id, holding_id, symbol, shares, cost_price, analyzed_at, current_price,
	sr_zone_analysis_id, action, action_label, stop_loss_price, stop_loss_amount,
	take_profit_price, take_profit_amount, add_on_trigger_price, add_on_amount,
	unrealized_pnl, unrealized_pnl_pct, reason, detail_json, created_at`

func (r *holdingRepo) CreateAnalysis(ctx context.Context, a *HoldingAnalysis) (uint64, error) {
	if a.Reason == "" {
		a.Reason = RawJSON("[]")
	}
	if a.DetailJSON == "" {
		a.DetailJSON = RawJSON("{}")
	}
	var id uint64
	if r.driver == "pgx" {
		err := r.db.QueryRowContext(ctx, `
			INSERT INTO holding_analyses (
				holding_id, symbol, shares, cost_price, analyzed_at, current_price,
				sr_zone_analysis_id, action, action_label, stop_loss_price, stop_loss_amount,
				take_profit_price, take_profit_amount, add_on_trigger_price, add_on_amount,
				unrealized_pnl, unrealized_pnl_pct, reason, detail_json
			) VALUES (
				$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19
			)
			RETURNING id
		`,
			a.HoldingID, a.Symbol, a.Shares, a.CostPrice, a.AnalyzedAt, a.CurrentPrice,
			nullInt64Value(a.SRZoneAnalysisID), a.Action, a.ActionLabel, nullFloat64Value(a.StopLossPrice), nullFloat64Value(a.StopLossAmount),
			nullFloat64Value(a.TakeProfitPrice), nullFloat64Value(a.TakeProfitAmount), nullFloat64Value(a.AddOnTriggerPrice), nullFloat64Value(a.AddOnAmount),
			a.UnrealizedPnL, a.UnrealizedPnLPct, a.Reason, a.DetailJSON,
		).Scan(&id)
		return id, err
	}
	res, err := r.db.ExecContext(ctx, r.db.Rebind(`
		INSERT INTO holding_analyses (
			holding_id, symbol, shares, cost_price, analyzed_at, current_price,
			sr_zone_analysis_id, action, action_label, stop_loss_price, stop_loss_amount,
			take_profit_price, take_profit_amount, add_on_trigger_price, add_on_amount,
			unrealized_pnl, unrealized_pnl_pct, reason, detail_json
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	`),
		a.HoldingID, a.Symbol, a.Shares, a.CostPrice, a.AnalyzedAt, a.CurrentPrice,
		nullInt64Value(a.SRZoneAnalysisID), a.Action, a.ActionLabel, nullFloat64Value(a.StopLossPrice), nullFloat64Value(a.StopLossAmount),
		nullFloat64Value(a.TakeProfitPrice), nullFloat64Value(a.TakeProfitAmount), nullFloat64Value(a.AddOnTriggerPrice), nullFloat64Value(a.AddOnAmount),
		a.UnrealizedPnL, a.UnrealizedPnLPct, a.Reason, a.DetailJSON,
	)
	if err != nil {
		return 0, err
	}
	lastID, err := res.LastInsertId()
	return uint64(lastID), err
}

func (r *holdingRepo) GetAnalysis(ctx context.Context, id uint64) (*HoldingAnalysis, error) {
	var a HoldingAnalysis
	err := r.db.GetContext(ctx, &a, r.db.Rebind(`
		SELECT `+holdingAnalysisColumns+` FROM holding_analyses WHERE id=?
	`), id)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *holdingRepo) ListAnalyses(ctx context.Context, holdingID uint64, limit int) ([]HoldingAnalysis, error) {
	var rows []HoldingAnalysis
	err := r.db.SelectContext(ctx, &rows, r.db.Rebind(`
		SELECT `+holdingAnalysisColumns+`
		FROM holding_analyses
		WHERE holding_id=?
		ORDER BY created_at DESC, id DESC
		LIMIT ?
	`), holdingID, limit)
	return rows, err
}

func nullFloat64Value(n NullFloat64) any {
	if !n.Valid {
		return nil
	}
	return n.Float64
}

func nullInt64Value(n NullInt64) any {
	if !n.Valid {
		return nil
	}
	return n.Int64
}

func NewNullFloat64(v float64) NullFloat64 {
	return NullFloat64{NullFloat64: sql.NullFloat64{Float64: v, Valid: true}}
}

func NewNullInt64(v int64) NullInt64 {
	return NullInt64{NullInt64: sql.NullInt64{Int64: v, Valid: true}}
}

func NewNullTime(v time.Time) NullTime {
	return NullTime{NullTime: sql.NullTime{Time: v, Valid: true}}
}
