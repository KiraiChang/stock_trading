package bouncereplay

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/trading/backend/internal/store"
)

// LoadOptions：窗口與暖身、起始歷史的範圍。
type LoadOptions struct {
	From, To    time.Time // data_at ∈ [From, To)
	WarmupDays  int       // 1 分 K 往前多取幾天（要涵蓋前一個交易日；預設 4）
	HistoryDays int       // signals 起始歷史往前取幾天（預設 7）
}

type evalRow struct {
	RunID       uint64           `db:"run_id"`
	Symbol      string           `db:"symbol"`
	DataAt      time.Time        `db:"data_at"`
	EvalAt      store.NullTime   `db:"eval_at"`
	LastBarTS   store.NullTime   `db:"last_bar_ts"`
	Fingerprint store.NullString `db:"fingerprint"`
	EvalStatus  string           `db:"eval_status"`
	EvalCause   store.NullString `db:"eval_cause"`
}

// Load 從隔離的來源庫（live 交接資料還原進來的 database，唯讀）讀出重播的輸入。
func Load(ctx context.Context, src *sqlx.DB, o LoadOptions) (*Input, error) {
	if o.WarmupDays <= 0 {
		o.WarmupDays = 4
	}
	if o.HistoryDays <= 0 {
		o.HistoryDays = 7
	}
	in := &Input{From: o.From, To: o.To, Candles: map[string][]store.Candle{}}

	var rows []evalRow
	if err := src.SelectContext(ctx, &rows, src.Rebind(`
		SELECT s.run_id, s.symbol, s.data_at, s.eval_at, s.last_bar_ts, s.fingerprint, s.eval_status, s.eval_cause
		FROM intraday_symbol_runs s
		JOIN intraday_rounds r ON r.run_id = s.run_id
		WHERE s.source = 'scheduled' AND r.source = 'scheduled'
		  AND s.data_at >= ? AND s.data_at < ?
		  AND s.eval_status IS NOT NULL AND s.eval_status <> 'not_evaluated'
		ORDER BY s.eval_at, s.symbol`), o.From, o.To); err != nil {
		return nil, fmt.Errorf("load evals: %w", err)
	}
	symbols := map[string]bool{}
	for _, r := range rows {
		if !r.EvalAt.Valid || !r.LastBarTS.Valid {
			return nil, fmt.Errorf("run %d %s：評估過卻沒有 eval_at 或 last_bar_ts", r.RunID, r.Symbol)
		}
		in.Evals = append(in.Evals, Eval{
			RunID: r.RunID, Symbol: r.Symbol, DataAt: r.DataAt, EvalAt: r.EvalAt.Time, LastBarTS: r.LastBarTS.Time,
			Fingerprint: r.Fingerprint.String, EvalStatus: r.EvalStatus, EvalCause: r.EvalCause.String,
		})
		symbols[r.Symbol] = true
	}
	sort.SliceStable(in.Evals, func(i, j int) bool { return in.Evals[i].EvalAt.Before(in.Evals[j].EvalAt) })

	candles := store.NewCandleRepo(src)
	for sym := range symbols {
		cs, err := candles.GetRange(ctx, sym, Timeframe, o.From.AddDate(0, 0, -o.WarmupDays), o.To)
		if err != nil {
			return nil, fmt.Errorf("load candles %s: %w", sym, err)
		}
		in.Candles[sym] = cs
	}

	syms := make([]string, 0, len(symbols))
	for s := range symbols {
		syms = append(syms, s)
	}
	sort.Strings(syms)
	if len(syms) > 0 {
		q, args, err := sqlx.In(`
			SELECT id, symbol, signal_type, direction, price, volume, COALESCE(vol_ratio, 0) AS vol_ratio,
			       COALESCE(resistance, 0) AS resistance, COALESCE(support, 0) AS support, COALESCE(trend, '') AS trend,
			       COALESCE(note, '') AS note, strength, chip_signal, ts, created_at
			FROM signals
			WHERE symbol IN (?) AND created_at >= ? AND created_at < ?
			ORDER BY created_at, id`, syms, o.From.AddDate(0, 0, -o.HistoryDays), o.To)
		if err != nil {
			return nil, err
		}
		if err := src.SelectContext(ctx, &in.Signals, src.Rebind(q), args...); err != nil {
			return nil, fmt.Errorf("load signals: %w", err)
		}
	}
	return in, nil
}
