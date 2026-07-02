package store

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"
)

type SRZoneRepo interface {
	// Create 寫入一筆 zone 評分快照與其所有 zone，回傳新建的 analysis id
	Create(ctx context.Context, a *SRZoneAnalysis, zones []SRZone) (uint64, error)
	Get(ctx context.Context, id uint64) (*SRZoneAnalysis, error)
	List(ctx context.Context, symbol string, limit int) ([]SRZoneAnalysis, error)
	GetZones(ctx context.Context, analysisID uint64) ([]SRZone, error)
	// Delete 刪除一筆 zone 評分快照及其所有 zone
	Delete(ctx context.Context, id uint64) error
}

type srZoneRepo struct {
	db     *sqlx.DB
	driver string
}

func NewSRZoneRepo(db *sqlx.DB) SRZoneRepo {
	return &srZoneRepo{db: db, driver: db.DriverName()}
}

func (r *srZoneRepo) Create(ctx context.Context, a *SRZoneAnalysis, zones []SRZone) (uint64, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	const cols = `symbol, timeframe, analyzed_at, current_price, model_version`

	var id uint64
	if r.driver == "pgx" {
		// pgx（postgres）不支援 LastInsertId，需改用 RETURNING id
		err = tx.QueryRowContext(ctx, `
			INSERT INTO stock_sr_zone_analyses (`+cols+`)
			VALUES ($1,$2,$3,$4,$5)
			RETURNING id
		`,
			a.Symbol, a.Timeframe, a.AnalyzedAt, a.CurrentPrice, a.ModelVersion,
		).Scan(&id)
	} else {
		var result sql.Result
		result, err = tx.ExecContext(ctx, tx.Rebind(`
			INSERT INTO stock_sr_zone_analyses (`+cols+`)
			VALUES (?,?,?,?,?)
		`),
			a.Symbol, a.Timeframe, a.AnalyzedAt, a.CurrentPrice, a.ModelVersion,
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

	for i := range zones {
		zones[i].AnalysisID = id
		if zones[i].Status == "" {
			zones[i].Status = "PENDING"
		}
		if _, err := tx.NamedExecContext(ctx, `
			INSERT INTO stock_sr_zones (
				analysis_id, price_low, price_high, method, role,
				support_score, resistance_score, bounce_probability, break_probability,
				touch_count, rejection_count, breakout_count, avg_return_after_touch,
				relative_volume, volatility, trend_strength, status
			) VALUES (
				:analysis_id, :price_low, :price_high, :method, :role,
				:support_score, :resistance_score, :bounce_probability, :break_probability,
				:touch_count, :rejection_count, :breakout_count, :avg_return_after_touch,
				:relative_volume, :volatility, :trend_strength, :status
			)
		`, zones[i]); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

const srZoneAnalysisColumns = `id, symbol, timeframe, analyzed_at, current_price, model_version, created_at`

func (r *srZoneRepo) Get(ctx context.Context, id uint64) (*SRZoneAnalysis, error) {
	var a SRZoneAnalysis
	err := r.db.GetContext(ctx, &a, r.db.Rebind(`
		SELECT `+srZoneAnalysisColumns+` FROM stock_sr_zone_analyses WHERE id=?
	`), id)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *srZoneRepo) List(ctx context.Context, symbol string, limit int) ([]SRZoneAnalysis, error) {
	var rows []SRZoneAnalysis
	var err error
	if symbol != "" {
		err = r.db.SelectContext(ctx, &rows, r.db.Rebind(`
			SELECT `+srZoneAnalysisColumns+`
			FROM stock_sr_zone_analyses WHERE symbol=? ORDER BY created_at DESC LIMIT ?
		`), symbol, limit)
	} else {
		err = r.db.SelectContext(ctx, &rows, r.db.Rebind(`
			SELECT `+srZoneAnalysisColumns+`
			FROM stock_sr_zone_analyses ORDER BY created_at DESC LIMIT ?
		`), limit)
	}
	return rows, err
}

func (r *srZoneRepo) GetZones(ctx context.Context, analysisID uint64) ([]SRZone, error) {
	var rows []SRZone
	err := r.db.SelectContext(ctx, &rows, r.db.Rebind(`
		SELECT id, analysis_id, price_low, price_high, method, role,
			support_score, resistance_score, bounce_probability, break_probability,
			touch_count, rejection_count, breakout_count, avg_return_after_touch,
			relative_volume, volatility, trend_strength, status, broken_at, broken_price
		FROM stock_sr_zones WHERE analysis_id=? ORDER BY support_score DESC, resistance_score DESC
	`), analysisID)
	return rows, err
}

func (r *srZoneRepo) Delete(ctx context.Context, id uint64) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 先刪子表（stock_sr_zones 有 FK 指向 stock_sr_zone_analyses），再刪主紀錄
	if _, err := tx.ExecContext(ctx, tx.Rebind(`DELETE FROM stock_sr_zones WHERE analysis_id=?`), id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, tx.Rebind(`DELETE FROM stock_sr_zone_analyses WHERE id=?`), id); err != nil {
		return err
	}
	return tx.Commit()
}
