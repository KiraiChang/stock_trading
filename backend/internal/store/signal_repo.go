package store

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type SignalRepo interface {
	Insert(ctx context.Context, s *Signal) error
	GetRecent(ctx context.Context, limit int) ([]Signal, error)
	GetBySymbol(ctx context.Context, symbol string, limit int) ([]Signal, error)
}

type signalRepo struct {
	db *sqlx.DB
}

func NewSignalRepo(db *sqlx.DB) SignalRepo {
	return &signalRepo{db: db}
}

func (r *signalRepo) Insert(ctx context.Context, s *Signal) error {
	result, err := r.db.NamedExecContext(ctx, `
		INSERT INTO signals
			(symbol, signal_type, direction, price, volume, vol_ratio, resistance, support, trend, note, ts)
		VALUES
			(:symbol, :signal_type, :direction, :price, :volume, :vol_ratio, :resistance, :support, :trend, :note, :ts)
	`, s)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	s.ID = uint64(id)
	return nil
}

func (r *signalRepo) GetRecent(ctx context.Context, limit int) ([]Signal, error) {
	var rows []Signal
	err := r.db.SelectContext(ctx, &rows, r.db.Rebind(`
		SELECT id, symbol, signal_type, direction, price, volume, vol_ratio, resistance, support, trend, note, ts
		FROM signals
		ORDER BY ts DESC
		LIMIT ?
	`), limit)
	return rows, err
}

func (r *signalRepo) GetBySymbol(ctx context.Context, symbol string, limit int) ([]Signal, error) {
	var rows []Signal
	err := r.db.SelectContext(ctx, &rows, r.db.Rebind(`
		SELECT id, symbol, signal_type, direction, price, volume, vol_ratio, resistance, support, trend, note, ts
		FROM signals
		WHERE symbol=?
		ORDER BY ts DESC
		LIMIT ?
	`), symbol, limit)
	return rows, err
}
