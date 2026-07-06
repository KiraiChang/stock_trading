package store

import (
	"context"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

const chipScoreBulkBatchSize = 50

type ChipScoreRepo interface {
	Upsert(ctx context.Context, s *ChipScore) error
	BulkUpsert(ctx context.Context, ss []ChipScore) error
	GetByDate(ctx context.Context, symbol string, date time.Time) (*ChipScore, error)
	GetLatest(ctx context.Context, symbol string) (*ChipScore, error)
	GetRange(ctx context.Context, symbol string, from, to time.Time) ([]ChipScore, error)
}

type chipScoreRepo struct {
	db     *sqlx.DB
	driver string
}

func NewChipScoreRepo(db *sqlx.DB) ChipScoreRepo {
	return &chipScoreRepo{db: db, driver: db.DriverName()}
}

const chipScoreColumns = `id, symbol, trade_date, institutional_score, margin_score, broker_score, concentration_score, total_score, signal, reason, created_at, updated_at`

func (r *chipScoreRepo) upsertSQL() string {
	if r.driver == "mysql" {
		return `
			INSERT INTO chip_scores (symbol, trade_date, institutional_score, margin_score, broker_score, concentration_score, total_score, signal, reason)
			VALUES (:symbol, :trade_date, :institutional_score, :margin_score, :broker_score, :concentration_score, :total_score, :signal, :reason)
			ON DUPLICATE KEY UPDATE
				institutional_score=VALUES(institutional_score), margin_score=VALUES(margin_score),
				broker_score=VALUES(broker_score), concentration_score=VALUES(concentration_score),
				total_score=VALUES(total_score), signal=VALUES(signal), reason=VALUES(reason),
				updated_at=CURRENT_TIMESTAMP`
	}
	return `
		INSERT INTO chip_scores (symbol, trade_date, institutional_score, margin_score, broker_score, concentration_score, total_score, signal, reason)
		VALUES (:symbol, :trade_date, :institutional_score, :margin_score, :broker_score, :concentration_score, :total_score, :signal, :reason)
		ON CONFLICT(symbol, trade_date) DO UPDATE SET
			institutional_score=excluded.institutional_score, margin_score=excluded.margin_score,
			broker_score=excluded.broker_score, concentration_score=excluded.concentration_score,
			total_score=excluded.total_score, signal=excluded.signal, reason=excluded.reason,
			updated_at=CURRENT_TIMESTAMP`
}

func (r *chipScoreRepo) Upsert(ctx context.Context, s *ChipScore) error {
	_, err := r.db.NamedExecContext(ctx, r.upsertSQL(), s)
	return err
}

func (r *chipScoreRepo) BulkUpsert(ctx context.Context, ss []ChipScore) error {
	if len(ss) == 0 {
		return nil
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for start := 0; start < len(ss); start += chipScoreBulkBatchSize {
		end := start + chipScoreBulkBatchSize
		if end > len(ss) {
			end = len(ss)
		}
		if err := r.bulkUpsert(ctx, tx, ss[start:end]); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *chipScoreRepo) bulkUpsert(ctx context.Context, tx *sqlx.Tx, ss []ChipScore) error {
	args := make([]any, 0, len(ss)*9)
	var values strings.Builder
	for i, s := range ss {
		if i > 0 {
			values.WriteString(", ")
		}
		values.WriteString("(?, ?, ?, ?, ?, ?, ?, ?, ?)")
		args = append(args,
			s.Symbol, s.TradeDate, s.InstitutionalScore, s.MarginScore, s.BrokerScore,
			s.ConcentrationScore, s.TotalScore, s.Signal, s.Reason,
		)
	}

	query := `
		INSERT INTO chip_scores (symbol, trade_date, institutional_score, margin_score, broker_score, concentration_score, total_score, signal, reason)
		VALUES ` + values.String() + r.upsertSuffix()
	_, err := tx.ExecContext(ctx, r.db.Rebind(query), args...)
	return err
}

func (r *chipScoreRepo) upsertSuffix() string {
	if r.driver == "mysql" {
		return `
			ON DUPLICATE KEY UPDATE
				institutional_score=VALUES(institutional_score), margin_score=VALUES(margin_score),
				broker_score=VALUES(broker_score), concentration_score=VALUES(concentration_score),
				total_score=VALUES(total_score), signal=VALUES(signal), reason=VALUES(reason),
				updated_at=CURRENT_TIMESTAMP`
	}
	return `
		ON CONFLICT(symbol, trade_date) DO UPDATE SET
			institutional_score=excluded.institutional_score, margin_score=excluded.margin_score,
			broker_score=excluded.broker_score, concentration_score=excluded.concentration_score,
			total_score=excluded.total_score, signal=excluded.signal, reason=excluded.reason,
			updated_at=CURRENT_TIMESTAMP`
}

func (r *chipScoreRepo) GetByDate(ctx context.Context, symbol string, date time.Time) (*ChipScore, error) {
	var s ChipScore
	sql := r.db.Rebind(`SELECT ` + chipScoreColumns + ` FROM chip_scores WHERE symbol=? AND trade_date=?`)
	if err := r.db.GetContext(ctx, &s, sql, symbol, date); err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *chipScoreRepo) GetLatest(ctx context.Context, symbol string) (*ChipScore, error) {
	var s ChipScore
	sql := r.db.Rebind(`
		SELECT ` + chipScoreColumns + `
		FROM chip_scores
		WHERE symbol=?
		ORDER BY trade_date DESC
		LIMIT 1
	`)
	if err := r.db.GetContext(ctx, &s, sql, symbol); err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *chipScoreRepo) GetRange(ctx context.Context, symbol string, from, to time.Time) ([]ChipScore, error) {
	var rows []ChipScore
	sql := r.db.Rebind(`
		SELECT ` + chipScoreColumns + `
		FROM chip_scores
		WHERE symbol=? AND trade_date BETWEEN ? AND ?
		ORDER BY trade_date ASC
	`)
	err := r.db.SelectContext(ctx, &rows, sql, symbol, from, to)
	return rows, err
}
