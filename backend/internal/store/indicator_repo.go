package store

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type IndicatorRepo interface {
	Upsert(ctx context.Context, s *IndicatorSnapshot) error
	GetLatest(ctx context.Context, symbol, timeframe string) (*IndicatorSnapshot, error)
}

type indicatorRepo struct {
	db     *sqlx.DB
	driver string
}

func NewIndicatorRepo(db *sqlx.DB) IndicatorRepo {
	return &indicatorRepo{db: db, driver: db.DriverName()}
}

func (r *indicatorRepo) upsertSQL() string {
	if r.driver == "sqlite" {
		return `
			INSERT INTO indicator_snapshots
				(symbol, timeframe, ts, ma5, ma10, ma20, ma60, rsi14, macd, macd_signal, macd_hist,
				 bb_upper, bb_middle, bb_lower, atr14, vwap, vol_ma20, vol_ratio)
			VALUES
				(:symbol, :timeframe, :ts, :ma5, :ma10, :ma20, :ma60, :rsi14, :macd, :macd_signal, :macd_hist,
				 :bb_upper, :bb_middle, :bb_lower, :atr14, :vwap, :vol_ma20, :vol_ratio)
			ON CONFLICT(symbol, timeframe, ts) DO UPDATE SET
				ma5=excluded.ma5, ma10=excluded.ma10, ma20=excluded.ma20, ma60=excluded.ma60,
				rsi14=excluded.rsi14,
				macd=excluded.macd, macd_signal=excluded.macd_signal, macd_hist=excluded.macd_hist,
				bb_upper=excluded.bb_upper, bb_middle=excluded.bb_middle, bb_lower=excluded.bb_lower,
				atr14=excluded.atr14, vwap=excluded.vwap,
				vol_ma20=excluded.vol_ma20, vol_ratio=excluded.vol_ratio`
	}
	return `
		INSERT INTO indicator_snapshots
			(symbol, timeframe, ts, ma5, ma10, ma20, ma60, rsi14, macd, macd_signal, macd_hist,
			 bb_upper, bb_middle, bb_lower, atr14, vwap, vol_ma20, vol_ratio)
		VALUES
			(:symbol, :timeframe, :ts, :ma5, :ma10, :ma20, :ma60, :rsi14, :macd, :macd_signal, :macd_hist,
			 :bb_upper, :bb_middle, :bb_lower, :atr14, :vwap, :vol_ma20, :vol_ratio)
		ON DUPLICATE KEY UPDATE
			ma5=VALUES(ma5), ma10=VALUES(ma10), ma20=VALUES(ma20), ma60=VALUES(ma60),
			rsi14=VALUES(rsi14), macd=VALUES(macd), macd_signal=VALUES(macd_signal), macd_hist=VALUES(macd_hist),
			bb_upper=VALUES(bb_upper), bb_middle=VALUES(bb_middle), bb_lower=VALUES(bb_lower),
			atr14=VALUES(atr14), vwap=VALUES(vwap), vol_ma20=VALUES(vol_ma20), vol_ratio=VALUES(vol_ratio)`
}

func (r *indicatorRepo) Upsert(ctx context.Context, s *IndicatorSnapshot) error {
	_, err := r.db.NamedExecContext(ctx, r.upsertSQL(), s)
	return err
}

func (r *indicatorRepo) GetLatest(ctx context.Context, symbol, timeframe string) (*IndicatorSnapshot, error) {
	var snap IndicatorSnapshot
	err := r.db.GetContext(ctx, &snap, `
		SELECT id, symbol, timeframe, ts,
			ma5, ma10, ma20, ma60, rsi14, macd, macd_signal, macd_hist,
			bb_upper, bb_middle, bb_lower, atr14, vwap, vol_ma20, vol_ratio
		FROM indicator_snapshots
		WHERE symbol=? AND timeframe=?
		ORDER BY ts DESC
		LIMIT 1
	`, symbol, timeframe)
	if err != nil {
		return nil, err
	}
	return &snap, nil
}
