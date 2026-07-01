package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/jmoiron/sqlx"
)

type AnalysisRepo interface {
	// Create 寫入一筆分析快照與其支撐/壓力位，回傳新建的 analysis id
	Create(ctx context.Context, a *StockAnalysis, levels []StockAnalysisLevel) (uint64, error)
	Get(ctx context.Context, id uint64) (*StockAnalysis, error)
	List(ctx context.Context, symbol string, limit int) ([]StockAnalysis, error)
	GetLevels(ctx context.Context, analysisID uint64) ([]StockAnalysisLevel, error)
	UpdateVerification(ctx context.Context, analysisID uint64, tradeVerificationJSON string) error
	UpdateLevelStatus(ctx context.Context, levelID uint64, status string, brokenAt *time.Time, brokenPrice *float64) error
	// Delete 刪除一筆分析快照及其所有支撐/壓力位
	Delete(ctx context.Context, id uint64) error
}

type analysisRepo struct {
	db     *sqlx.DB
	driver string
}

func NewAnalysisRepo(db *sqlx.DB) AnalysisRepo {
	return &analysisRepo{db: db, driver: db.DriverName()}
}

func (r *analysisRepo) Create(ctx context.Context, a *StockAnalysis, levels []StockAnalysisLevel) (uint64, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	const cols = `symbol, timeframe, analyzed_at, current_price, trend,
		entry_status, entry_direction, entry_price, entry_reason,
		stop_loss_atr, stop_loss_structural, stop_loss_composite,
		take_profit_next_level, take_profit_risk_reward, take_profit_atr`

	var id uint64
	if r.driver == "pgx" {
		// pgx（postgres）不支援 LastInsertId，需改用 RETURNING id
		err = tx.QueryRowContext(ctx, `
			INSERT INTO stock_analyses (`+cols+`)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
			RETURNING id
		`,
			a.Symbol, a.Timeframe, a.AnalyzedAt, a.CurrentPrice, a.Trend,
			a.EntryStatus, a.EntryDirection, a.EntryPrice, a.EntryReason,
			a.StopLossATR, a.StopLossStructural, a.StopLossComposite,
			a.TakeProfitNextLevel, a.TakeProfitRiskReward, a.TakeProfitATR,
		).Scan(&id)
	} else {
		var result sql.Result
		result, err = tx.ExecContext(ctx, tx.Rebind(`
			INSERT INTO stock_analyses (`+cols+`)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		`),
			a.Symbol, a.Timeframe, a.AnalyzedAt, a.CurrentPrice, a.Trend,
			a.EntryStatus, a.EntryDirection, a.EntryPrice, a.EntryReason,
			a.StopLossATR, a.StopLossStructural, a.StopLossComposite,
			a.TakeProfitNextLevel, a.TakeProfitRiskReward, a.TakeProfitATR,
		)
		if err == nil {
			var lastID int64
			lastID, err = result.LastInsertId()
			id = uint64(lastID)
		}
	}
	if err != nil {
		return 0, err
	}

	for i := range levels {
		levels[i].AnalysisID = id
		if levels[i].Status == "" {
			levels[i].Status = "PENDING"
		}
		if _, err := tx.NamedExecContext(ctx, `
			INSERT INTO stock_analysis_levels (analysis_id, price, type, strength, method, status)
			VALUES (:analysis_id, :price, :type, :strength, :method, :status)
		`, levels[i]); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

const analysisColumns = `id, symbol, timeframe, analyzed_at, current_price, trend,
	entry_status, entry_direction, entry_price, entry_reason,
	stop_loss_atr, stop_loss_structural, stop_loss_composite,
	take_profit_next_level, take_profit_risk_reward, take_profit_atr,
	trade_verification, verified_at, created_at`

func (r *analysisRepo) Get(ctx context.Context, id uint64) (*StockAnalysis, error) {
	var a StockAnalysis
	err := r.db.GetContext(ctx, &a, r.db.Rebind(`
		SELECT `+analysisColumns+` FROM stock_analyses WHERE id=?
	`), id)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *analysisRepo) List(ctx context.Context, symbol string, limit int) ([]StockAnalysis, error) {
	var rows []StockAnalysis
	var err error
	if symbol != "" {
		err = r.db.SelectContext(ctx, &rows, r.db.Rebind(`
			SELECT `+analysisColumns+`
			FROM stock_analyses WHERE symbol=? ORDER BY created_at DESC LIMIT ?
		`), symbol, limit)
	} else {
		err = r.db.SelectContext(ctx, &rows, r.db.Rebind(`
			SELECT `+analysisColumns+`
			FROM stock_analyses ORDER BY created_at DESC LIMIT ?
		`), limit)
	}
	return rows, err
}

func (r *analysisRepo) GetLevels(ctx context.Context, analysisID uint64) ([]StockAnalysisLevel, error) {
	var rows []StockAnalysisLevel
	err := r.db.SelectContext(ctx, &rows, r.db.Rebind(`
		SELECT id, analysis_id, price, type, strength, method, status, broken_at, broken_price
		FROM stock_analysis_levels WHERE analysis_id=? ORDER BY strength DESC
	`), analysisID)
	return rows, err
}

func (r *analysisRepo) UpdateVerification(ctx context.Context, analysisID uint64, tradeVerificationJSON string) error {
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`
		UPDATE stock_analyses SET trade_verification=?, verified_at=CURRENT_TIMESTAMP WHERE id=?
	`), tradeVerificationJSON, analysisID)
	return err
}

func (r *analysisRepo) UpdateLevelStatus(ctx context.Context, levelID uint64, status string, brokenAt *time.Time, brokenPrice *float64) error {
	_, err := r.db.ExecContext(ctx, r.db.Rebind(`
		UPDATE stock_analysis_levels SET status=?, broken_at=?, broken_price=? WHERE id=?
	`), status, brokenAt, brokenPrice, levelID)
	return err
}

func (r *analysisRepo) Delete(ctx context.Context, id uint64) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 先刪子表（stock_analysis_levels 有 FK 指向 stock_analyses），再刪主紀錄
	if _, err := tx.ExecContext(ctx, tx.Rebind(`DELETE FROM stock_analysis_levels WHERE analysis_id=?`), id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, tx.Rebind(`DELETE FROM stock_analyses WHERE id=?`), id); err != nil {
		return err
	}
	return tx.Commit()
}
