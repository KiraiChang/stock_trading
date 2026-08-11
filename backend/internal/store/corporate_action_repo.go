package store

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
)

type CorporateActionRepo interface {
	// Upsert 以 (symbol, event_date, action_type) 為鍵寫入。
	// **必須是 upsert 而不是 insert**：重複抓取若產生第二筆事件，同一次分割會被乘兩次，
	// 還原係數就不再是冪等的（見 todo.md T-042 的冪等性設計）。
	Upsert(ctx context.Context, actions []CorporateAction) error
	// ListBySymbol 依 event_date 升冪回傳，順序固定——連乘順序不固定會產生浮點尾差。
	ListBySymbol(ctx context.Context, symbol string) ([]CorporateAction, error)
	// Symbols 回傳所有有事件的 symbol，供重算時決定要處理哪些檔。
	Symbols(ctx context.Context) ([]string, error)
}

type corporateActionRepo struct {
	db     *sqlx.DB
	driver string
}

func NewCorporateActionRepo(db *sqlx.DB) CorporateActionRepo {
	return &corporateActionRepo{db: db, driver: db.DriverName()}
}

func (r *corporateActionRepo) upsertSQL() string {
	if r.driver == "mysql" {
		return `
			INSERT INTO corporate_actions
				(symbol, event_date, action_type, before_price, after_price, factor, volume_factor, source)
			VALUES (:symbol, :event_date, :action_type, :before_price, :after_price, :factor, :volume_factor, :source)
			ON DUPLICATE KEY UPDATE
				before_price=VALUES(before_price), after_price=VALUES(after_price),
				factor=VALUES(factor), volume_factor=VALUES(volume_factor), source=VALUES(source)`
	}
	return `
		INSERT INTO corporate_actions
			(symbol, event_date, action_type, before_price, after_price, factor, volume_factor, source)
		VALUES (:symbol, :event_date, :action_type, :before_price, :after_price, :factor, :volume_factor, :source)
		ON CONFLICT(symbol, event_date, action_type) DO UPDATE SET
			before_price=excluded.before_price, after_price=excluded.after_price,
			factor=excluded.factor, volume_factor=excluded.volume_factor, source=excluded.source`
}

func (r *corporateActionRepo) Upsert(ctx context.Context, actions []CorporateAction) error {
	if len(actions) == 0 {
		return nil
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	sql := r.upsertSQL()
	for i := range actions {
		if _, err := tx.NamedExecContext(ctx, sql, actions[i]); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *corporateActionRepo) ListBySymbol(ctx context.Context, symbol string) ([]CorporateAction, error) {
	var rows []CorporateAction
	sql := r.db.Rebind(`
		SELECT id, symbol, event_date, action_type, before_price, after_price, factor, volume_factor, source
		FROM corporate_actions
		WHERE symbol=?
		ORDER BY event_date ASC
	`)
	err := r.db.SelectContext(ctx, &rows, sql, symbol)
	return rows, err
}

func (r *corporateActionRepo) Symbols(ctx context.Context) ([]string, error) {
	var rows []string
	err := r.db.SelectContext(ctx, &rows,
		`SELECT DISTINCT symbol FROM corporate_actions ORDER BY symbol`)
	return rows, err
}

// AdjFactorRange 是一段 [From, To) 區間要套用的係數。To 為零值代表沒有右界。
//
// Price 與 Volume 分開：現金股利改價不改股數，兩者會不相等（T-042 Phase 2）。
type AdjFactorRange struct {
	From   time.Time
	To     time.Time
	Price  float64
	Volume float64
}

// ApplyAdjFactors 在**單一交易內**把某 symbol 的係數全部歸 1 再依 ranges 覆寫。
//
// **為什麼一定要在同一個交易裡**：歸零與寫入之間的那段時間，讀取端看到的 adj_factor
// 是 1——也就是「未調整」。對跨越分割的歷史來說，那等於在重算期間對外供應假跳空的價格，
// 而且完全不會報錯。分開兩次 Exec 時這個窗口真實存在（0050 有 4,800 根 K 棒要更新）。
//
// **一定是覆寫（=），不是累乘（*=）**。這是冪等性的核心：重算永遠從事件表重新推導出
// 整段的值再寫回，不讀舊值，所以跑一次跟跑十次結果相同。若寫成 `adj_factor = adj_factor * ?`，
// 跑兩次就平方了，而且沒有任何東西會報錯。
func (r *candleRepo) ApplyAdjFactors(ctx context.Context, symbol string, ranges []AdjFactorRange) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// 先歸零。少了這一步，被刪掉的事件所留下的舊係數會留在原地沒人清，
	// 重算就不再是「事件表的純函數」。
	if _, err := tx.ExecContext(ctx, r.db.Rebind(
		`UPDATE candles SET adj_factor = 1, vol_factor = 1 WHERE symbol = ?`), symbol); err != nil {
		return err
	}

	set := r.db.Rebind(
		`UPDATE candles SET adj_factor = ?, vol_factor = ? WHERE symbol = ? AND ts >= ? AND ts < ?`)
	for _, rg := range ranges {
		if _, err := tx.ExecContext(ctx, set, rg.Price, rg.Volume, symbol, rg.From, rg.To); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// CountUnadjustedBefore 回傳某 symbol 在 ts < before 且 adj_factor = 1 的 K 棒數。
// 用來偵測「回補插入了比事件更早的 K 棒但沒重算」——那些列會靜靜地帶著未還原的價格。
func (r *candleRepo) CountUnadjustedBefore(ctx context.Context, symbol string, before time.Time) (int, error) {
	var n int
	sql := r.db.Rebind(`
		SELECT COUNT(*) FROM candles
		WHERE symbol = ? AND ts < ? AND adj_factor = 1
	`)
	err := r.db.GetContext(ctx, &n, sql, symbol, before)
	return n, err
}

// Symbols 回傳 candles 內所有相異 symbol。
//
// 走 (symbol, timeframe, ts) 這個既有索引，不需要全表掃描。
func (r *candleRepo) Symbols(ctx context.Context) ([]string, error) {
	var rows []string
	err := r.db.SelectContext(ctx, &rows, `SELECT DISTINCT symbol FROM candles ORDER BY symbol`)
	return rows, err
}
