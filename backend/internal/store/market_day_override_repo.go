package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
)

// 臨時休市（颱風假）與推翻日曆的手動設定（I-139；migration 080）。現況規格見 docs/database-schema.md 的
// market_day_overrides 與 docs/architecture.md 的「休市日的行為」。
//
// **只新增、⛔ 不改寫**：每次儲存與取消各一列，留下誰、何時、做什麼、為什麼。某一天目前的狀態＝該日 id 最大的那一列；
// 最新一列是 cleared、或那天沒有任何列 → 依日曆。

const (
	MarketDayClosed  = "closed"  // 手動休市
	MarketDayOpen    = "open"    // 手動照常開市（推翻日曆的休市日）
	MarketDayCleared = "cleared" // 取消手動設定，回到日曆

	// MarketDayReasonMaxRunes 是原因文字的上限（三份 migration 的 VARCHAR(200)）。
	MarketDayReasonMaxRunes = 200
)

// MarketDayOverride 是 market_day_overrides 的一列。TradeDate 是台北的日曆日期 YYYY-MM-DD。
type MarketDayOverride struct {
	ID        uint64    `db:"id" json:"id"`
	TradeDate string    `db:"trade_date" json:"trade_date"`
	Action    string    `db:"action" json:"action"`
	Reason    string    `db:"reason" json:"reason"`
	UserID    uint64    `db:"user_id" json:"user_id"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// Effective 回報這一列是不是生效中的手動設定（cleared ⛔ 不算）。
func (o *MarketDayOverride) Effective() bool {
	return o != nil && (o.Action == MarketDayClosed || o.Action == MarketDayOpen)
}

type MarketDayOverrideRepo interface {
	// Insert 新增一列（id 與 created_at 由 DB 給），回傳**這次寫入的那一列**。寫入與讀回在同一步完成（postgres 用 RETURNING、
	// 其他用同一個 transaction 依 id 讀回）：⛔ 不另外查「那一天最新的一列」——別的頁面剛好也寫入時會拿到對方的紀錄；
	// 讀回失敗時整筆 rollback，⛔ 不會出現「回報失敗、設定卻已生效」。
	Insert(ctx context.Context, o MarketDayOverride) (MarketDayOverride, error)
	// Latest 回傳那一天 id 最大的那一列；那天沒有任何列時回 nil, nil。
	Latest(ctx context.Context, tradeDate string) (*MarketDayOverride, error)
	// ListSince 回傳 trade_date >= fromDate 的所有列（依 trade_date、id 排序），供畫面列出目前狀態與操作紀錄。
	ListSince(ctx context.Context, fromDate string) ([]MarketDayOverride, error)
}

type marketDayOverrideRepo struct {
	db     *sqlx.DB
	driver string
}

func NewMarketDayOverrideRepo(db *sqlx.DB) MarketDayOverrideRepo {
	return &marketDayOverrideRepo{db: db, driver: db.DriverName()}
}

const marketDayOverrideCols = `id, trade_date, action, reason, user_id, created_at`

func (r *marketDayOverrideRepo) Insert(ctx context.Context, o MarketDayOverride) (MarketDayOverride, error) {
	var out MarketDayOverride
	// pgx（postgres）不支援 LastInsertId，用 RETURNING 一次拿回整列（同 jobRunRepo.Start）。
	if r.driver == "pgx" {
		err := r.db.QueryRowxContext(ctx, `
			INSERT INTO market_day_overrides (trade_date, action, reason, user_id) VALUES ($1, $2, $3, $4)
			RETURNING `+marketDayOverrideCols, o.TradeDate, o.Action, o.Reason, o.UserID).StructScan(&out)
		return out, err
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback() //nolint:errcheck // commit 之後 rollback 是 no-op
	res, err := tx.ExecContext(ctx, tx.Rebind(`
		INSERT INTO market_day_overrides (trade_date, action, reason, user_id) VALUES (?, ?, ?, ?)`),
		o.TradeDate, o.Action, o.Reason, o.UserID)
	if err != nil {
		return out, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return out, err
	}
	if err := tx.GetContext(ctx, &out, tx.Rebind(`SELECT `+marketDayOverrideCols+` FROM market_day_overrides WHERE id=?`), id); err != nil {
		return MarketDayOverride{}, err
	}
	if err := tx.Commit(); err != nil {
		return MarketDayOverride{}, err
	}
	return out, nil
}

func (r *marketDayOverrideRepo) Latest(ctx context.Context, tradeDate string) (*MarketDayOverride, error) {
	var out MarketDayOverride
	err := r.db.GetContext(ctx, &out, r.db.Rebind(`
		SELECT `+marketDayOverrideCols+` FROM market_day_overrides WHERE trade_date=? ORDER BY id DESC LIMIT 1`), tradeDate)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *marketDayOverrideRepo) ListSince(ctx context.Context, fromDate string) ([]MarketDayOverride, error) {
	out := []MarketDayOverride{}
	err := r.db.SelectContext(ctx, &out, r.db.Rebind(`
		SELECT `+marketDayOverrideCols+` FROM market_day_overrides WHERE trade_date>=? ORDER BY trade_date, id`), fromDate)
	return out, err
}
