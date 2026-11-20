package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

// 盤中逐輪、逐檔的存證（T-088＋I-103；migration 079）。現況規格見 docs/database-schema.md 的
// intraday_rounds／intraday_symbol_runs 與 docs/yahoo-intraday-integration.md。

const (
	IntradaySourceScheduled = "scheduled"
	IntradaySourceRefill    = "refill"

	IntradayOutcomeExecuted           = "executed"
	IntradayOutcomeSkippedLockTimeout = "skipped_lock_timeout"
	// IntradayOutcomeFailed：在逐檔結果之前就失敗的輪次（例如讀不到自選清單），⛔ 沒有逐檔的列。
	IntradayOutcomeFailed = "failed"
)

// 評估結果的值域（intraday_symbol_runs.eval_status）。
const (
	IntradayEvalNoSignal         = "no_signal"
	IntradayEvalSignalSent       = "signal_sent"
	IntradayEvalSignalSuppressed = "signal_suppressed"
	IntradayEvalWithheldStale    = "withheld_stale"
	IntradayEvalDegraded         = "degraded"
	IntradayEvalFailed           = "evaluate_failed"
	IntradayEvalNotEvaluated     = "not_evaluated"
)

// IntradayEvalSucceeded 回報這個評估結果算不算「成功評估」：成功評估的 fingerprint 是下一輪比對的
// 「上一次成功評估」。評估失敗、⛔ 不評估、手動補抓（沒有評估段）都⛔ 不算。
func IntradayEvalSucceeded(status string) bool {
	switch status {
	case IntradayEvalNoSignal, IntradayEvalSignalSent, IntradayEvalSignalSuppressed,
		IntradayEvalWithheldStale, IntradayEvalDegraded:
		return true
	}
	return false
}

var intradaySuccessfulEvalStatuses = []string{
	IntradayEvalNoSignal, IntradayEvalSignalSent, IntradayEvalSignalSuppressed,
	IntradayEvalWithheldStale, IntradayEvalDegraded,
}

// IntradayRound 是一輪（intraday_rounds 的一列）。
type IntradayRound struct {
	RunID            uint64   `db:"run_id" json:"run_id"`
	Source           string   `db:"source" json:"source"`
	ScheduledAt      time.Time `db:"scheduled_at" json:"scheduled_at"`
	LockAcquiredAt   NullTime `db:"lock_acquired_at" json:"lock_acquired_at"`
	ExecFinishedAt   NullTime `db:"exec_finished_at" json:"exec_finished_at"`
	LockWaitMs       int64    `db:"lock_wait_ms" json:"lock_wait_ms"`
	Outcome          string   `db:"outcome" json:"outcome"`
	EvidenceFailures int      `db:"evidence_failures" json:"evidence_failures"`
}

// IntradaySymbolRun 是一輪、一檔（intraday_symbol_runs 的一列）。
type IntradaySymbolRun struct {
	RunID         uint64     `db:"run_id" json:"run_id"`
	Symbol        string     `db:"symbol" json:"symbol"`
	Source        string     `db:"source" json:"source"`
	DataAt        time.Time  `db:"data_at" json:"data_at"`
	DataStatus    string     `db:"data_status" json:"data_status"`
	DataCause     NullString `db:"data_cause" json:"data_cause"`
	LastBarTS     NullTime   `db:"last_bar_ts" json:"last_bar_ts"`
	Fingerprint   NullString `db:"fingerprint" json:"-"`
	FetchAttempts int        `db:"fetch_attempts" json:"fetch_attempts"`
	HTTPRecovered bool       `db:"http_recovered" json:"http_recovered"`
	EvalAt        NullTime   `db:"eval_at" json:"eval_at"`
	EvalStatus    NullString `db:"eval_status" json:"eval_status"`
	EvalCause     NullString `db:"eval_cause" json:"eval_cause"`
}

// ErrIntradayEvidenceNotFound 是 update 找不到那一列（run_id＋symbol 沒有 insert 成功）。
var ErrIntradayEvidenceNotFound = errors.New("intraday evidence row not found")

type IntradayEvidenceRepo interface {
	InsertRound(ctx context.Context, r IntradayRound) error
	// FinishRound 寫回實際結束時刻與存證失敗次數；outcome 非空時一併改寫（例如 failed），空字串保留原值。
	FinishRound(ctx context.Context, runID uint64, finishedAt time.Time, evidenceFailures int, outcome string) error
	// LatestRound 回傳 source 最新的一輪（run_id 最大）；沒有時回 nil。
	LatestRound(ctx context.Context, source string) (*IntradayRound, error)
	// InsertSymbolRuns 一次寫入一輪的行情段（同一個交易）。
	InsertSymbolRuns(ctx context.Context, rows []IntradaySymbolRun) error
	// UpdateSymbolEval 寫評估段；cause 空字串寫 NULL。那一列不存在時回 ErrIntradayEvidenceNotFound。
	UpdateSymbolEval(ctx context.Context, runID uint64, symbol string, at time.Time, status, cause string) error
	// LatestSymbolRuns 回傳每一檔在 source 最新的一列（沒有紀錄的標的不在 map 裡）。
	LatestSymbolRuns(ctx context.Context, source string, symbols []string) (map[string]IntradaySymbolRun, error)
	// LastSuccessfulEvals 回傳每一檔最近一次「成功評估」的那一列（只看 scheduled）。
	LastSuccessfulEvals(ctx context.Context, symbols []string) (map[string]IntradaySymbolRun, error)
	// ListSymbolRuns 回傳一輪的所有逐檔列（依 symbol 排序），逐輪對帳用。
	ListSymbolRuns(ctx context.Context, runID uint64) ([]IntradaySymbolRun, error)
	// DeleteBefore 刪掉 cutoff 之前的紀錄（跟 job_runs 的保留期同一步），回傳兩張表合計的筆數。
	DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error)
}

type intradayEvidenceRepo struct {
	db *sqlx.DB
}

func NewIntradayEvidenceRepo(db *sqlx.DB) IntradayEvidenceRepo {
	return &intradayEvidenceRepo{db: db}
}

func (r *intradayEvidenceRepo) InsertRound(ctx context.Context, round IntradayRound) error {
	_, err := r.db.NamedExecContext(ctx, `
		INSERT INTO intraday_rounds (run_id, source, scheduled_at, lock_acquired_at, exec_finished_at,
			lock_wait_ms, outcome, evidence_failures)
		VALUES (:run_id, :source, :scheduled_at, :lock_acquired_at, :exec_finished_at,
			:lock_wait_ms, :outcome, :evidence_failures)`, round)
	return err
}

func (r *intradayEvidenceRepo) FinishRound(ctx context.Context, runID uint64, finishedAt time.Time, evidenceFailures int, outcome string) error {
	res, err := r.db.ExecContext(ctx, r.db.Rebind(`
		UPDATE intraday_rounds SET exec_finished_at=?, evidence_failures=?, outcome=COALESCE(NULLIF(?, ''), outcome) WHERE run_id=?`),
		finishedAt, evidenceFailures, outcome, runID)
	if err != nil {
		return err
	}
	return requireAffected(res)
}

func (r *intradayEvidenceRepo) LatestRound(ctx context.Context, source string) (*IntradayRound, error) {
	var out IntradayRound
	err := r.db.GetContext(ctx, &out, r.db.Rebind(`
		SELECT run_id, source, scheduled_at, lock_acquired_at, exec_finished_at, lock_wait_ms, outcome, evidence_failures
		FROM intraday_rounds WHERE source=? ORDER BY run_id DESC LIMIT 1`), source)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &out, nil
}

const intradaySymbolRunCols = `run_id, symbol, source, data_at, data_status, data_cause, last_bar_ts, fingerprint,
	fetch_attempts, http_recovered, eval_at, eval_status, eval_cause`

func (r *intradayEvidenceRepo) InsertSymbolRuns(ctx context.Context, rows []IntradaySymbolRun) error {
	if len(rows) == 0 {
		return nil
	}
	_, err := r.db.NamedExecContext(ctx, `
		INSERT INTO intraday_symbol_runs (`+intradaySymbolRunCols+`)
		VALUES (:run_id, :symbol, :source, :data_at, :data_status, :data_cause, :last_bar_ts, :fingerprint,
			:fetch_attempts, :http_recovered, :eval_at, :eval_status, :eval_cause)`, rows)
	return err
}

func (r *intradayEvidenceRepo) UpdateSymbolEval(ctx context.Context, runID uint64, symbol string, at time.Time, status, cause string) error {
	var c NullString
	if cause != "" {
		c.String, c.Valid = cause, true
	}
	res, err := r.db.ExecContext(ctx, r.db.Rebind(`
		UPDATE intraday_symbol_runs SET eval_at=?, eval_status=?, eval_cause=? WHERE run_id=? AND symbol=?`),
		at, status, c, runID, symbol)
	if err != nil {
		return err
	}
	return requireAffected(res)
}

func (r *intradayEvidenceRepo) LatestSymbolRuns(ctx context.Context, source string, symbols []string) (map[string]IntradaySymbolRun, error) {
	return r.latestPerSymbol(ctx, `source = ?`, []any{source}, symbols)
}

func (r *intradayEvidenceRepo) LastSuccessfulEvals(ctx context.Context, symbols []string) (map[string]IntradaySymbolRun, error) {
	return r.latestPerSymbol(ctx, `source = ? AND eval_status IN (?)`,
		[]any{IntradaySourceScheduled, intradaySuccessfulEvalStatuses}, symbols)
}

// latestPerSymbol 取每一檔符合 cond 的 run_id 最大的那一列。**一句查詢**，⛔ 不逐檔查。
func (r *intradayEvidenceRepo) latestPerSymbol(ctx context.Context, cond string, condArgs []any, symbols []string) (map[string]IntradaySymbolRun, error) {
	out := map[string]IntradaySymbolRun{}
	if len(symbols) == 0 {
		return out, nil
	}
	args := append(append([]any{}, condArgs...), symbols)
	query, qargs, err := sqlx.In(`
		SELECT `+prefixCols("r.", intradaySymbolRunCols)+`
		FROM intraday_symbol_runs r
		JOIN (SELECT symbol, MAX(run_id) AS run_id FROM intraday_symbol_runs
		      WHERE `+cond+` AND symbol IN (?) GROUP BY symbol) m
		  ON r.symbol = m.symbol AND r.run_id = m.run_id`, args...)
	if err != nil {
		return nil, err
	}
	var rows []IntradaySymbolRun
	if err := r.db.SelectContext(ctx, &rows, r.db.Rebind(query), qargs...); err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.Symbol] = row
	}
	return out, nil
}

func (r *intradayEvidenceRepo) ListSymbolRuns(ctx context.Context, runID uint64) ([]IntradaySymbolRun, error) {
	var rows []IntradaySymbolRun
	err := r.db.SelectContext(ctx, &rows, r.db.Rebind(`
		SELECT `+intradaySymbolRunCols+` FROM intraday_symbol_runs WHERE run_id=? ORDER BY symbol`), runID)
	return rows, err
}

func (r *intradayEvidenceRepo) DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var total int64
	for _, q := range []string{
		`DELETE FROM intraday_symbol_runs WHERE data_at < ?`,
		`DELETE FROM intraday_rounds WHERE scheduled_at < ?`,
	} {
		res, err := tx.ExecContext(ctx, tx.Rebind(q), cutoff)
		if err != nil {
			return 0, err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return 0, err
		}
		total += n
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return total, nil
}

func requireAffected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrIntradayEvidenceNotFound
	}
	return nil
}

// prefixCols 把逗號分隔的欄位清單加上表別名。
func prefixCols(prefix, cols string) string {
	parts := strings.Split(cols, ",")
	for i, f := range parts {
		parts[i] = prefix + strings.TrimSpace(f)
	}
	return strings.Join(parts, ", ")
}
