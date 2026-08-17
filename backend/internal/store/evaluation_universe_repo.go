package store

import (
	"context"
	"errors"

	"github.com/jmoiron/sqlx"
)

// SelectedAt 是研究紀錄的一部分（何時入池），零值會被寫成 0001-01-01，
// 之後分不清「沒填」與「真的很久以前入池」，所以直接拒絕。
var errEvaluationUniverseSelectedAtRequired = errors.New(
	"evaluation universe: selected_at is required")

// EvaluationUniverseRepo 管理評估標的池（T-040 Step 5）。
//
// **這個池不是 watchlist**：它只驅動「每日盤後更新日 K」一件事，不進盤中掃描、籌碼同步、
// signal 或 production SR 分析。規格見
// docs/evaluation-universe-selection-plan.md 的「Step 5 執行計畫書」。
type EvaluationUniverseRepo interface {
	// ListActive 回傳仍納入每日維護的成員，依 symbol 升冪。
	// **這是排程的唯一熱路徑**，對應 idx_evaluation_universe_active。
	ListActive(ctx context.Context) ([]EvaluationUniverseEntry, error)
	// List 回傳全部成員（含已停用者），供人工檢視入退池歷史。
	List(ctx context.Context) ([]EvaluationUniverseEntry, error)
	// Upsert 以 symbol 為鍵寫入。**必須是 upsert 而不是 insert**：重新匯入
	// selection report 是常態動作（門檻重定或母體變化後），insert 會直接撞 UNIQUE。
	// 不動 active——停用是獨立的人工決定，不該被一次重新匯入靜默覆寫。
	Upsert(ctx context.Context, entries []EvaluationUniverseEntry) error
	// SetActive 切換單一標的是否納入每日維護。回傳是否真的有那一列。
	SetActive(ctx context.Context, symbol string, active bool) (bool, error)
}

type evaluationUniverseRepo struct {
	db     *sqlx.DB
	driver string
}

func NewEvaluationUniverseRepo(db *sqlx.DB) EvaluationUniverseRepo {
	return &evaluationUniverseRepo{db: db, driver: db.DriverName()}
}

const evaluationUniverseColumns = `id, symbol, bucket_hint, bucket_edge_low, bucket_edge_high,
	universe_version, universe_role, selected_at, source, active, note`

func (r *evaluationUniverseRepo) upsertSQL() string {
	if r.driver == "mysql" {
		return `
			INSERT INTO evaluation_universe
				(symbol, bucket_hint, bucket_edge_low, bucket_edge_high,
				 universe_version, universe_role, selected_at, source, note)
			VALUES (:symbol, :bucket_hint, :bucket_edge_low, :bucket_edge_high,
				:universe_version, :universe_role, :selected_at, :source, :note)
			ON DUPLICATE KEY UPDATE
				bucket_hint=VALUES(bucket_hint),
				bucket_edge_low=VALUES(bucket_edge_low),
				bucket_edge_high=VALUES(bucket_edge_high),
				universe_version=VALUES(universe_version),
				universe_role=VALUES(universe_role),
				selected_at=VALUES(selected_at),
				source=VALUES(source),
				note=VALUES(note),
				updated_at=CURRENT_TIMESTAMP`
	}
	return `
		INSERT INTO evaluation_universe
			(symbol, bucket_hint, bucket_edge_low, bucket_edge_high,
			 universe_version, universe_role, selected_at, source, note)
		VALUES (:symbol, :bucket_hint, :bucket_edge_low, :bucket_edge_high,
			:universe_version, :universe_role, :selected_at, :source, :note)
		ON CONFLICT(symbol) DO UPDATE SET
			bucket_hint=excluded.bucket_hint,
			bucket_edge_low=excluded.bucket_edge_low,
			bucket_edge_high=excluded.bucket_edge_high,
			universe_version=excluded.universe_version,
			universe_role=excluded.universe_role,
			selected_at=excluded.selected_at,
			source=excluded.source,
			note=excluded.note,
			updated_at=CURRENT_TIMESTAMP`
}

func (r *evaluationUniverseRepo) Upsert(ctx context.Context, entries []EvaluationUniverseEntry) error {
	if len(entries) == 0 {
		return nil
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	sql := r.upsertSQL()
	for i := range entries {
		if entries[i].SelectedAt.IsZero() {
			// 交給 DB 的 DEFAULT 不可行：這是 named exec，欄位一定會被帶上。
			// 用零值會寫進 0001-01-01，之後分不清「沒填」與「真的很久以前入池」。
			return errEvaluationUniverseSelectedAtRequired
		}
		// **補預設值時用副本，不要改寫呼叫端的 slice。** handler 解出 entries 後可能拿
		// 同一份資料回傳或寫 log，原地修改會讓它看到「使用者其實沒送的」role 值，
		// 據此判斷「使用者有沒有指定 role」就會判斷錯誤。
		e := entries[i]
		if e.UniverseRole == "" {
			e.UniverseRole = "primary"
		}
		if _, err := tx.NamedExecContext(ctx, sql, e); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *evaluationUniverseRepo) ListActive(ctx context.Context) ([]EvaluationUniverseEntry, error) {
	var rows []EvaluationUniverseEntry
	sql := r.db.Rebind(`SELECT ` + evaluationUniverseColumns + `
		FROM evaluation_universe WHERE active = ? ORDER BY symbol ASC`)
	// 用 bind 參數而非字面 1/0：postgres 的 active 是原生 BOOLEAN，
	// "active = 1" 在 postgres 會直接報型別錯誤（比照 watchlist_repo.SetWatched 的註解）。
	err := r.db.SelectContext(ctx, &rows, sql, true)
	return rows, err
}

func (r *evaluationUniverseRepo) List(ctx context.Context) ([]EvaluationUniverseEntry, error) {
	var rows []EvaluationUniverseEntry
	err := r.db.SelectContext(ctx, &rows, `SELECT `+evaluationUniverseColumns+`
		FROM evaluation_universe ORDER BY symbol ASC`)
	return rows, err
}

func (r *evaluationUniverseRepo) SetActive(ctx context.Context, symbol string, active bool) (bool, error) {
	sql := r.db.Rebind(`UPDATE evaluation_universe
		SET active = ?, updated_at = CURRENT_TIMESTAMP WHERE symbol = ?`)
	res, err := r.db.ExecContext(ctx, sql, active, symbol)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		// 有些 driver 不保證支援 RowsAffected；此時不能謊稱找不到那一列。
		return true, nil
	}
	return n > 0, nil
}
