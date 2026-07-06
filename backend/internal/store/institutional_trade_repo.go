package store

import (
	"context"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

const institutionalTradeBulkBatchSize = 50

type InstitutionalTradeRepo interface {
	Upsert(ctx context.Context, t *InstitutionalTrade) error
	BulkUpsert(ctx context.Context, ts []InstitutionalTrade) error
	GetByDate(ctx context.Context, symbol string, date time.Time) (*InstitutionalTrade, error)
	GetRange(ctx context.Context, symbol string, from, to time.Time) ([]InstitutionalTrade, error)
	// GetLatestN 回傳最近 n 筆，依 trade_date 升冪排序，供連續買賣超天數/
	// 累積買賣超計算使用。
	GetLatestN(ctx context.Context, symbol string, n int) ([]InstitutionalTrade, error)
}

type institutionalTradeRepo struct {
	db     *sqlx.DB
	driver string
}

func NewInstitutionalTradeRepo(db *sqlx.DB) InstitutionalTradeRepo {
	return &institutionalTradeRepo{db: db, driver: db.DriverName()}
}

const institutionalTradeColumns = `id, symbol, trade_date, foreign_net_buy, investment_trust_net_buy, dealer_net_buy, total_net_buy, created_at, updated_at`

func (r *institutionalTradeRepo) upsertSQL() string {
	if r.driver == "mysql" {
		return `
			INSERT INTO institutional_trades (symbol, trade_date, foreign_net_buy, investment_trust_net_buy, dealer_net_buy, total_net_buy)
			VALUES (:symbol, :trade_date, :foreign_net_buy, :investment_trust_net_buy, :dealer_net_buy, :total_net_buy)
			ON DUPLICATE KEY UPDATE
				foreign_net_buy=VALUES(foreign_net_buy), investment_trust_net_buy=VALUES(investment_trust_net_buy),
				dealer_net_buy=VALUES(dealer_net_buy), total_net_buy=VALUES(total_net_buy), updated_at=CURRENT_TIMESTAMP`
	}
	// SQLite 和 PostgreSQL 均支援 ON CONFLICT 語法
	return `
		INSERT INTO institutional_trades (symbol, trade_date, foreign_net_buy, investment_trust_net_buy, dealer_net_buy, total_net_buy)
		VALUES (:symbol, :trade_date, :foreign_net_buy, :investment_trust_net_buy, :dealer_net_buy, :total_net_buy)
		ON CONFLICT(symbol, trade_date) DO UPDATE SET
			foreign_net_buy=excluded.foreign_net_buy, investment_trust_net_buy=excluded.investment_trust_net_buy,
			dealer_net_buy=excluded.dealer_net_buy, total_net_buy=excluded.total_net_buy, updated_at=CURRENT_TIMESTAMP`
}

func (r *institutionalTradeRepo) Upsert(ctx context.Context, t *InstitutionalTrade) error {
	_, err := r.db.NamedExecContext(ctx, r.upsertSQL(), t)
	return err
}

func (r *institutionalTradeRepo) BulkUpsert(ctx context.Context, ts []InstitutionalTrade) error {
	if len(ts) == 0 {
		return nil
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for start := 0; start < len(ts); start += institutionalTradeBulkBatchSize {
		end := start + institutionalTradeBulkBatchSize
		if end > len(ts) {
			end = len(ts)
		}
		if err := r.bulkUpsert(ctx, tx, ts[start:end]); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *institutionalTradeRepo) bulkUpsert(ctx context.Context, tx *sqlx.Tx, ts []InstitutionalTrade) error {
	args := make([]any, 0, len(ts)*6)
	var values strings.Builder
	for i, t := range ts {
		if i > 0 {
			values.WriteString(", ")
		}
		values.WriteString("(?, ?, ?, ?, ?, ?)")
		args = append(args, t.Symbol, t.TradeDate, t.ForeignNetBuy, t.InvestmentTrustNetBuy, t.DealerNetBuy, t.TotalNetBuy)
	}

	query := `
		INSERT INTO institutional_trades (symbol, trade_date, foreign_net_buy, investment_trust_net_buy, dealer_net_buy, total_net_buy)
		VALUES ` + values.String() + r.upsertSuffix()
	_, err := tx.ExecContext(ctx, r.db.Rebind(query), args...)
	return err
}

func (r *institutionalTradeRepo) upsertSuffix() string {
	if r.driver == "mysql" {
		return `
			ON DUPLICATE KEY UPDATE
				foreign_net_buy=VALUES(foreign_net_buy), investment_trust_net_buy=VALUES(investment_trust_net_buy),
				dealer_net_buy=VALUES(dealer_net_buy), total_net_buy=VALUES(total_net_buy), updated_at=CURRENT_TIMESTAMP`
	}
	return `
		ON CONFLICT(symbol, trade_date) DO UPDATE SET
			foreign_net_buy=excluded.foreign_net_buy, investment_trust_net_buy=excluded.investment_trust_net_buy,
			dealer_net_buy=excluded.dealer_net_buy, total_net_buy=excluded.total_net_buy, updated_at=CURRENT_TIMESTAMP`
}

func (r *institutionalTradeRepo) GetByDate(ctx context.Context, symbol string, date time.Time) (*InstitutionalTrade, error) {
	var t InstitutionalTrade
	sql := r.db.Rebind(`SELECT ` + institutionalTradeColumns + ` FROM institutional_trades WHERE symbol=? AND trade_date=?`)
	if err := r.db.GetContext(ctx, &t, sql, symbol, date); err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *institutionalTradeRepo) GetRange(ctx context.Context, symbol string, from, to time.Time) ([]InstitutionalTrade, error) {
	var rows []InstitutionalTrade
	sql := r.db.Rebind(`
		SELECT ` + institutionalTradeColumns + `
		FROM institutional_trades
		WHERE symbol=? AND trade_date BETWEEN ? AND ?
		ORDER BY trade_date ASC
	`)
	err := r.db.SelectContext(ctx, &rows, sql, symbol, from, to)
	return rows, err
}

func (r *institutionalTradeRepo) GetLatestN(ctx context.Context, symbol string, n int) ([]InstitutionalTrade, error) {
	var rows []InstitutionalTrade
	sql := r.db.Rebind(`
		SELECT ` + institutionalTradeColumns + `
		FROM institutional_trades
		WHERE symbol=?
		ORDER BY trade_date DESC
		LIMIT ?
	`)
	if err := r.db.SelectContext(ctx, &rows, sql, symbol, n); err != nil {
		return nil, err
	}
	// 反轉回升冪排序，供分數計算使用
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	return rows, nil
}
