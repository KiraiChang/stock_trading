package store

import (
	"context"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

const brokerTradeBulkBatchSize = 50

type BrokerTradeRepo interface {
	BulkUpsert(ctx context.Context, ts []BrokerTrade) error
	// GetByDate 回傳該日全部分點列，依 net_buy DESC 排序，供排行/集中度計算使用。
	GetByDate(ctx context.Context, symbol string, date time.Time) ([]BrokerTrade, error)
}

type brokerTradeRepo struct {
	db     *sqlx.DB
	driver string
}

func NewBrokerTradeRepo(db *sqlx.DB) BrokerTradeRepo {
	return &brokerTradeRepo{db: db, driver: db.DriverName()}
}

const brokerTradeColumns = `id, symbol, trade_date, broker_name, branch_name, buy_volume, sell_volume, net_buy, created_at`

func (r *brokerTradeRepo) BulkUpsert(ctx context.Context, ts []BrokerTrade) error {
	if len(ts) == 0 {
		return nil
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for start := 0; start < len(ts); start += brokerTradeBulkBatchSize {
		end := start + brokerTradeBulkBatchSize
		if end > len(ts) {
			end = len(ts)
		}
		if err := r.bulkUpsert(ctx, tx, ts[start:end]); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *brokerTradeRepo) bulkUpsert(ctx context.Context, tx *sqlx.Tx, ts []BrokerTrade) error {
	args := make([]any, 0, len(ts)*7)
	var values strings.Builder
	for i, t := range ts {
		if i > 0 {
			values.WriteString(", ")
		}
		values.WriteString("(?, ?, ?, ?, ?, ?, ?)")
		args = append(args, t.Symbol, t.TradeDate, t.BrokerName, t.BranchName, t.BuyVolume, t.SellVolume, t.NetBuy)
	}

	query := `
		INSERT INTO broker_trades (symbol, trade_date, broker_name, branch_name, buy_volume, sell_volume, net_buy)
		VALUES ` + values.String() + r.upsertSuffix()
	_, err := tx.ExecContext(ctx, r.db.Rebind(query), args...)
	return err
}

func (r *brokerTradeRepo) upsertSuffix() string {
	if r.driver == "mysql" {
		return `
			ON DUPLICATE KEY UPDATE
				buy_volume=VALUES(buy_volume), sell_volume=VALUES(sell_volume), net_buy=VALUES(net_buy)`
	}
	return `
		ON CONFLICT(symbol, trade_date, broker_name, branch_name) DO UPDATE SET
			buy_volume=excluded.buy_volume, sell_volume=excluded.sell_volume, net_buy=excluded.net_buy`
}

func (r *brokerTradeRepo) GetByDate(ctx context.Context, symbol string, date time.Time) ([]BrokerTrade, error) {
	var rows []BrokerTrade
	sql := r.db.Rebind(`
		SELECT ` + brokerTradeColumns + `
		FROM broker_trades
		WHERE symbol=? AND trade_date=?
		ORDER BY net_buy DESC
	`)
	err := r.db.SelectContext(ctx, &rows, sql, symbol, date)
	return rows, err
}
