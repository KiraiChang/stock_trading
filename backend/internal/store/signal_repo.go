package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
)

type SignalRepo interface {
	Insert(ctx context.Context, s *Signal) error
	GetRecent(ctx context.Context, limit int) ([]Signal, error)
	GetBySymbol(ctx context.Context, symbol string, limit int) ([]Signal, error)
	// LatestBySymbolTypeTimeframe 回同一檔、同一類型、同一時間框架 created_at 最新的一列（同一秒再依 id；沒有時回 nil）。
	// ⛔ 不能只依 id：created_at 是候選在 DB 判重前取樣的送出時間，寫入順序可能跟它相反（較晚寫入的舊時間會蓋掉有效冷卻）。
	// 盤中 SUPPORT_BOUNCE 的 B 規則用它找冷卻錨點（I-140）；timeframe 是 NULL 的舊列⛔ 不會被取到。
	LatestBySymbolTypeTimeframe(ctx context.Context, symbol, signalType, timeframe string) (*Signal, error)
}

type signalRepo struct {
	db     *sqlx.DB
	driver string
}

func NewSignalRepo(db *sqlx.DB) SignalRepo {
	return &signalRepo{db: db, driver: db.DriverName()}
}

func (r *signalRepo) Insert(ctx context.Context, s *Signal) error {
	// created_at 由程式寫入送出時間（I-140）。引擎一定會填；其他呼叫端（測試、工具）沒填時用現在。
	// 一律截到整秒：MySQL 的 DATETIME(0) 帶小數秒寫入會四捨五入（可能多出 1 秒），三種 DB 要存得一樣。
	// 一律轉 UTC：SQLite 把時間存成帶時區的文字、ORDER BY 比的是字串，時區混用時排序會錯（錨點查詢依 created_at 排序）。
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now()
	}
	s.CreatedAt = s.CreatedAt.UTC().Truncate(time.Second)
	// pgx（postgres）不支援 LastInsertId，需改用 RETURNING id
	if r.driver == "pgx" {
		rows, err := r.db.NamedQueryContext(ctx, `
			INSERT INTO signals
				(symbol, signal_type, direction, price, volume, vol_ratio, resistance, support, trend, note, strength, chip_signal, ts, timeframe, created_at)
			VALUES
				(:symbol, :signal_type, :direction, :price, :volume, :vol_ratio, :resistance, :support, :trend, :note, :strength, :chip_signal, :ts, :timeframe, :created_at)
			RETURNING id
		`, s)
		if err != nil {
			return err
		}
		defer rows.Close()
		if rows.Next() {
			return rows.Scan(&s.ID)
		}
		return rows.Err()
	}

	result, err := r.db.NamedExecContext(ctx, `
		INSERT INTO signals
			(symbol, signal_type, direction, price, volume, vol_ratio, resistance, support, trend, note, strength, chip_signal, ts, timeframe, created_at)
		VALUES
			(:symbol, :signal_type, :direction, :price, :volume, :vol_ratio, :resistance, :support, :trend, :note, :strength, :chip_signal, :ts, :timeframe, :created_at)
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
		SELECT id, symbol, signal_type, direction, price, volume, vol_ratio, resistance, support, trend, note, strength, chip_signal, ts, timeframe, created_at
		FROM signals
		ORDER BY ts DESC
		LIMIT ?
	`), limit)
	return rows, err
}

func (r *signalRepo) GetBySymbol(ctx context.Context, symbol string, limit int) ([]Signal, error) {
	var rows []Signal
	err := r.db.SelectContext(ctx, &rows, r.db.Rebind(`
		SELECT id, symbol, signal_type, direction, price, volume, vol_ratio, resistance, support, trend, note, strength, chip_signal, ts, timeframe, created_at
		FROM signals
		WHERE symbol=?
		ORDER BY ts DESC
		LIMIT ?
	`), symbol, limit)
	return rows, err
}

func (r *signalRepo) LatestBySymbolTypeTimeframe(ctx context.Context, symbol, signalType, timeframe string) (*Signal, error) {
	var s Signal
	err := r.db.GetContext(ctx, &s, r.db.Rebind(`
		SELECT id, symbol, signal_type, direction, price, volume, vol_ratio, resistance, support, trend, note, strength, chip_signal, ts, timeframe, created_at
		FROM signals
		WHERE symbol=? AND signal_type=? AND timeframe=?
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`), symbol, signalType, timeframe)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}
