package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jmoiron/sqlx"
)

const (
	PositionEventOpeningBalance = "OPENING_BALANCE"
	PositionEventBuy            = "BUY"
	PositionEventSell           = "SELL"
	PositionEventAdjustment     = "ADJUSTMENT"
)

var (
	ErrPositionVersionConflict = errors.New("position version conflict")
	ErrPositionInvalidEvent    = errors.New("invalid position event")
)

type PositionRepo interface {
	List(ctx context.Context, portfolioID uint64) ([]Position, error)
	Get(ctx context.Context, portfolioID uint64, symbol string) (*Position, error)
	ListTransactions(ctx context.Context, portfolioID uint64, symbol string, limit int) ([]PositionTransaction, error)
	ApplyEvent(ctx context.Context, portfolioID uint64, event *PositionTransaction, expectedVersion int64) (*Position, error)
	CreateAnalysis(ctx context.Context, analysis *PositionAnalysis) (uint64, error)
	GetAnalysis(ctx context.Context, portfolioID uint64, id uint64) (*PositionAnalysis, error)
	ListAnalyses(ctx context.Context, portfolioID uint64, symbol string, limit int) ([]PositionAnalysis, error)
}

type positionRepo struct {
	db     *sqlx.DB
	driver string
}

func NewPositionRepo(db *sqlx.DB) PositionRepo {
	return &positionRepo{db: db, driver: db.DriverName()}
}

func normalizePortfolioID(portfolioID uint64) uint64 {
	if portfolioID == 0 {
		return DefaultPortfolioID
	}
	return portfolioID
}

func (r *positionRepo) List(ctx context.Context, portfolioID uint64) ([]Position, error) {
	portfolioID = normalizePortfolioID(portfolioID)
	var rows []Position
	err := r.db.SelectContext(ctx, &rows, r.db.Rebind(`
			SELECT portfolio_id,symbol,shares,avg_cost,realized_pnl,version,last_event_id,updated_at
			FROM positions WHERE portfolio_id=? AND shares>0 ORDER BY updated_at DESC,symbol
		`), portfolioID)
	return rows, err
}

func (r *positionRepo) Get(ctx context.Context, portfolioID uint64, symbol string) (*Position, error) {
	portfolioID = normalizePortfolioID(portfolioID)
	var row Position
	err := r.db.GetContext(ctx, &row, r.db.Rebind(`
			SELECT portfolio_id,symbol,shares,avg_cost,realized_pnl,version,last_event_id,updated_at
			FROM positions WHERE portfolio_id=? AND symbol=?
		`), portfolioID, symbol)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *positionRepo) ListTransactions(ctx context.Context, portfolioID uint64, symbol string, limit int) ([]PositionTransaction, error) {
	portfolioID = normalizePortfolioID(portfolioID)
	var rows []PositionTransaction
	err := r.db.SelectContext(ctx, &rows, r.db.Rebind(`
			SELECT id,portfolio_id,symbol,event_type,occurred_at,shares,price,fee,tax,
			       target_shares,target_avg_cost,note,created_at
			FROM position_transactions WHERE portfolio_id=? AND symbol=?
			ORDER BY occurred_at DESC,id DESC LIMIT ?
		`), portfolioID, symbol, limit)
	return rows, err
}

func (r *positionRepo) ApplyEvent(ctx context.Context, portfolioID uint64, event *PositionTransaction, expectedVersion int64) (*Position, error) {
	portfolioID = normalizePortfolioID(portfolioID)
	event.PortfolioID = portfolioID
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	current := Position{PortfolioID: portfolioID, Symbol: event.Symbol}
	lockClause := ""
	if r.driver == "pgx" || r.driver == "mysql" {
		lockClause = " FOR UPDATE"
	}
	err = tx.GetContext(ctx, &current, tx.Rebind(`
			SELECT portfolio_id,symbol,shares,avg_cost,realized_pnl,version,last_event_id,updated_at
			FROM positions WHERE portfolio_id=? AND symbol=?
		`+lockClause), portfolioID, event.Symbol)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	existed := !errors.Is(err, sql.ErrNoRows)
	if !existed {
		current = Position{PortfolioID: portfolioID, Symbol: event.Symbol}
	}
	if current.Version != expectedVersion {
		return nil, ErrPositionVersionConflict
	}
	eventID, err := r.insertEvent(ctx, tx, event)
	if err != nil {
		return nil, err
	}
	var events []PositionTransaction
	if err := tx.SelectContext(ctx, &events, tx.Rebind(`
			SELECT id,portfolio_id,symbol,event_type,occurred_at,shares,price,fee,tax,
			       target_shares,target_avg_cost,note,created_at
			FROM position_transactions WHERE portfolio_id=? AND symbol=? ORDER BY occurred_at,id
		`), portfolioID, event.Symbol); err != nil {
		return nil, err
	}
	nextVersion := current.Version + 1
	current = Position{PortfolioID: portfolioID, Symbol: event.Symbol}
	for i := range events {
		if err := applyPositionEvent(&current, &events[i]); err != nil {
			return nil, fmt.Errorf("replay position event %d: %w", events[i].ID, err)
		}
	}
	current.Version = nextVersion
	current.LastEventID = eventID
	if err := r.upsertPosition(ctx, tx, &current, existed, expectedVersion); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.Get(ctx, portfolioID, event.Symbol)
}

func applyPositionEvent(position *Position, event *PositionTransaction) error {
	switch event.EventType {
	case PositionEventOpeningBalance, PositionEventBuy:
		if !event.Shares.Valid || !event.Price.Valid || event.Shares.Float64 <= 0 || event.Price.Float64 <= 0 {
			return fmt.Errorf("%w: BUY requires positive shares and price", ErrPositionInvalidEvent)
		}
		qty := event.Shares.Float64
		totalCost := position.Shares*position.AvgCost + qty*event.Price.Float64 + event.Fee
		position.Shares += qty
		position.AvgCost = totalCost / position.Shares
	case PositionEventSell:
		if !event.Shares.Valid || !event.Price.Valid || event.Shares.Float64 <= 0 || event.Price.Float64 <= 0 {
			return fmt.Errorf("%w: SELL requires positive shares and price", ErrPositionInvalidEvent)
		}
		qty := event.Shares.Float64
		if qty > position.Shares+1e-9 {
			return fmt.Errorf("%w: SELL shares exceed current position", ErrPositionInvalidEvent)
		}
		position.RealizedPnL += qty*(event.Price.Float64-position.AvgCost) - event.Fee - event.Tax
		position.Shares -= qty
		if math.Abs(position.Shares) < 1e-9 {
			position.Shares = 0
			position.AvgCost = 0
		}
	case PositionEventAdjustment:
		if !event.TargetShares.Valid || !event.TargetAvgCost.Valid ||
			event.TargetShares.Float64 < 0 || event.TargetAvgCost.Float64 < 0 ||
			(event.TargetShares.Float64 > 0 && event.TargetAvgCost.Float64 <= 0) ||
			(event.TargetShares.Float64 == 0 && event.TargetAvgCost.Float64 != 0) {
			return fmt.Errorf("%w: ADJUSTMENT requires a valid target shares/AVG state", ErrPositionInvalidEvent)
		}
		if event.Note == "" {
			return fmt.Errorf("%w: ADJUSTMENT reason is required", ErrPositionInvalidEvent)
		}
		position.Shares = event.TargetShares.Float64
		position.AvgCost = event.TargetAvgCost.Float64
	default:
		return fmt.Errorf("%w: unsupported position event type %q", ErrPositionInvalidEvent, event.EventType)
	}
	return nil
}

func (r *positionRepo) insertEvent(ctx context.Context, tx *sqlx.Tx, event *PositionTransaction) (uint64, error) {
	const columns = `portfolio_id,symbol,event_type,occurred_at,shares,price,fee,tax,target_shares,target_avg_cost,note`
	args := []any{
		event.PortfolioID, event.Symbol, event.EventType, event.OccurredAt, nullFloat64Value(event.Shares),
		nullFloat64Value(event.Price), event.Fee, event.Tax,
		nullFloat64Value(event.TargetShares), nullFloat64Value(event.TargetAvgCost), event.Note,
	}
	if r.driver == "pgx" {
		var id uint64
		err := tx.QueryRowContext(ctx, `INSERT INTO position_transactions (`+columns+`)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id`, args...).Scan(&id)
		return id, err
	}
	res, err := tx.ExecContext(ctx, tx.Rebind(`INSERT INTO position_transactions (`+columns+`)
			VALUES (?,?,?,?,?,?,?,?,?,?,?)`), args...)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	return uint64(id), err
}

func (r *positionRepo) upsertPosition(ctx context.Context, tx *sqlx.Tx, p *Position, existed bool, expectedVersion int64) error {
	if existed {
		// version-guarded 更新：只有現存列的 version 仍等於這次讀到的
		// expectedVersion 時才寫入。RowsAffected==0 代表期間被其他請求改過，
		// 回傳版本衝突。不依賴 FOR UPDATE（sqlite 沒有、且對尚不存在的列鎖不到），
		// 讓樂觀鎖在三種 driver 上都成立。
		res, err := tx.ExecContext(ctx, tx.Rebind(`
				UPDATE positions
				SET shares=?, avg_cost=?, realized_pnl=?, version=?, last_event_id=?, updated_at=CURRENT_TIMESTAMP
				WHERE portfolio_id=? AND symbol=? AND version=?
			`), p.Shares, p.AvgCost, p.RealizedPnL, p.Version, p.LastEventID, p.PortfolioID, p.Symbol, expectedVersion)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrPositionVersionConflict
		}
		return nil
	}
	// 新部位：靠 positions(portfolio_id, symbol) 唯一鍵防止並發重複建立；若另一個請求搶先建立，
	// 這次 INSERT 會因唯一鍵衝突失敗（回傳錯誤，不會靜默覆蓋對方的寫入）。
	_, err := tx.ExecContext(ctx, tx.Rebind(`
			INSERT INTO positions(portfolio_id,symbol,shares,avg_cost,realized_pnl,version,last_event_id)
			VALUES(?,?,?,?,?,?,?)
		`), p.PortfolioID, p.Symbol, p.Shares, p.AvgCost, p.RealizedPnL, p.Version, p.LastEventID)
	return err
}

const positionAnalysisColumns = `id,portfolio_id,symbol,position_state,position_version,shares,avg_cost,
		realized_pnl,analyzed_at,current_price,sr_zone_analysis_id,action,action_label,
	target_shares,adjustment_shares,adjustment_side,adjustment_amount,entry_price,
	stop_loss_price,take_profit_price,risk_amount,expected_reward_amount,risk_reward_ratio,
	unrealized_pnl,unrealized_pnl_pct,config_json,reason,evidence,trigger_conditions,
	invalidation_conditions,rule_version,created_at`

func (r *positionRepo) CreateAnalysis(ctx context.Context, a *PositionAnalysis) (uint64, error) {
	a.PortfolioID = normalizePortfolioID(a.PortfolioID)
	const columns = `portfolio_id,symbol,position_state,position_version,shares,avg_cost,realized_pnl,
			analyzed_at,current_price,sr_zone_analysis_id,action,action_label,target_shares,
			adjustment_shares,adjustment_side,adjustment_amount,entry_price,stop_loss_price,
			take_profit_price,risk_amount,expected_reward_amount,risk_reward_ratio,unrealized_pnl,
			unrealized_pnl_pct,config_json,reason,evidence,trigger_conditions,invalidation_conditions,rule_version`
	args := []any{
		a.PortfolioID, a.Symbol, a.PositionState, a.PositionVersion, a.Shares, a.AvgCost, a.RealizedPnL,
		a.AnalyzedAt, a.CurrentPrice, nullInt64Value(a.SRZoneAnalysisID), a.Action, a.ActionLabel,
		a.TargetShares, a.AdjustmentShares, a.AdjustmentSide, a.AdjustmentAmount,
		nullFloat64Value(a.EntryPrice), nullFloat64Value(a.StopLossPrice), nullFloat64Value(a.TakeProfitPrice),
		nullFloat64Value(a.RiskAmount), nullFloat64Value(a.ExpectedRewardAmount), nullFloat64Value(a.RiskRewardRatio),
		a.UnrealizedPnL, a.UnrealizedPnLPct, a.ConfigJSON, a.Reason, a.Evidence,
		a.TriggerConditions, a.InvalidationConditions, a.RuleVersion,
	}
	if r.driver == "pgx" {
		var id uint64
		placeholders := `$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30`
		err := r.db.QueryRowContext(ctx, `INSERT INTO position_analyses (`+columns+`) VALUES (`+placeholders+`) RETURNING id`, args...).Scan(&id)
		return id, err
	}
	res, err := r.db.ExecContext(ctx, r.db.Rebind(`INSERT INTO position_analyses (`+columns+`)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`), args...)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	return uint64(id), err
}

func (r *positionRepo) GetAnalysis(ctx context.Context, portfolioID uint64, id uint64) (*PositionAnalysis, error) {
	portfolioID = normalizePortfolioID(portfolioID)
	var row PositionAnalysis
	err := r.db.GetContext(ctx, &row, r.db.Rebind(`SELECT `+positionAnalysisColumns+` FROM position_analyses WHERE portfolio_id=? AND id=?`), portfolioID, id)
	return &row, err
}

func (r *positionRepo) ListAnalyses(ctx context.Context, portfolioID uint64, symbol string, limit int) ([]PositionAnalysis, error) {
	portfolioID = normalizePortfolioID(portfolioID)
	var rows []PositionAnalysis
	query := `SELECT ` + positionAnalysisColumns + ` FROM position_analyses WHERE portfolio_id=?`
	args := []any{portfolioID}
	if symbol != "" {
		query += ` AND symbol=?`
		args = append(args, symbol)
	}
	query += ` ORDER BY created_at DESC,id DESC LIMIT ?`
	args = append(args, limit)
	err := r.db.SelectContext(ctx, &rows, r.db.Rebind(query), args...)
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
