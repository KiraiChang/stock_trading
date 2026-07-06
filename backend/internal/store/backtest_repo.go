package store

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type BacktestRepo interface {
	CreateJob(ctx context.Context, job *BacktestJob) error
	UpdateJobStatus(ctx context.Context, jobID, status, errMsg string) error
	GetJob(ctx context.Context, jobID string) (*BacktestJob, error)
	ListJobs(ctx context.Context, limit int) ([]BacktestJob, error)

	UpsertResult(ctx context.Context, r *BacktestResult) error
	GetResult(ctx context.Context, jobID string) (*BacktestResult, error)

	InsertTrades(ctx context.Context, trades []BacktestTrade) error
	GetTrades(ctx context.Context, jobID string) ([]BacktestTrade, error)
}

type backtestRepo struct {
	db     *sqlx.DB
	driver string
}

func NewBacktestRepo(db *sqlx.DB) BacktestRepo {
	return &backtestRepo{db: db, driver: db.DriverName()}
}

// ── Job ───────────────────────────────────────────────────────

func (r *backtestRepo) CreateJob(ctx context.Context, job *BacktestJob) error {
	_, err := r.db.NamedExecContext(ctx, `
		INSERT INTO backtest_jobs
			(job_id, type, strategy, symbols, timeframe, start_date, end_date, status, trigger, use_chip_filter, chip_min_score)
		VALUES
			(:job_id, :type, :strategy, :symbols, :timeframe, :start_date, :end_date, :status, :trigger, :use_chip_filter, :chip_min_score)
	`, job)
	return err
}

func (r *backtestRepo) UpdateJobStatus(ctx context.Context, jobID, status, errMsg string) error {
	switch status {
	case "running":
		q := r.db.Rebind(`UPDATE backtest_jobs SET status=?, started_at=CURRENT_TIMESTAMP WHERE job_id=?`)
		_, err := r.db.ExecContext(ctx, q, status, jobID)
		return err
	case "done", "failed":
		q := r.db.Rebind(`UPDATE backtest_jobs SET status=?, error=?, finished_at=CURRENT_TIMESTAMP WHERE job_id=?`)
		_, err := r.db.ExecContext(ctx, q, status, errMsg, jobID)
		return err
	default:
		q := r.db.Rebind(`UPDATE backtest_jobs SET status=? WHERE job_id=?`)
		_, err := r.db.ExecContext(ctx, q, status, jobID)
		return err
	}
}

func (r *backtestRepo) GetJob(ctx context.Context, jobID string) (*BacktestJob, error) {
	var job BacktestJob
	err := r.db.GetContext(ctx, &job, r.db.Rebind(`
		SELECT id, job_id, type, strategy, symbols, timeframe, start_date, end_date,
		       status, trigger, error, use_chip_filter, chip_min_score, created_at, started_at, finished_at
		FROM backtest_jobs WHERE job_id=?
	`), jobID)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *backtestRepo) ListJobs(ctx context.Context, limit int) ([]BacktestJob, error) {
	var jobs []BacktestJob
	err := r.db.SelectContext(ctx, &jobs, r.db.Rebind(`
		SELECT id, job_id, type, strategy, symbols, timeframe, start_date, end_date,
		       status, trigger, error, use_chip_filter, chip_min_score, created_at, started_at, finished_at
		FROM backtest_jobs
		ORDER BY created_at DESC
		LIMIT ?
	`), limit)
	return jobs, err
}

// ── Result ────────────────────────────────────────────────────

func (r *backtestRepo) upsertResultSQL() string {
	if r.driver == "mysql" {
		return `
			INSERT INTO backtest_results
				(job_id, strategy, total_return, annual_return, win_rate, max_drawdown,
				 sharpe_ratio, total_trades, win_trades, loss_trades, avg_pnl)
			VALUES
				(:job_id, :strategy, :total_return, :annual_return, :win_rate, :max_drawdown,
				 :sharpe_ratio, :total_trades, :win_trades, :loss_trades, :avg_pnl)
			ON DUPLICATE KEY UPDATE
				total_return=VALUES(total_return), annual_return=VALUES(annual_return),
				win_rate=VALUES(win_rate), max_drawdown=VALUES(max_drawdown),
				sharpe_ratio=VALUES(sharpe_ratio), total_trades=VALUES(total_trades),
				win_trades=VALUES(win_trades), loss_trades=VALUES(loss_trades),
				avg_pnl=VALUES(avg_pnl)`
	}
	// sqlite 和 postgres 均支援 ON CONFLICT 語法
	return `
		INSERT INTO backtest_results
			(job_id, strategy, total_return, annual_return, win_rate, max_drawdown,
			 sharpe_ratio, total_trades, win_trades, loss_trades, avg_pnl)
		VALUES
			(:job_id, :strategy, :total_return, :annual_return, :win_rate, :max_drawdown,
			 :sharpe_ratio, :total_trades, :win_trades, :loss_trades, :avg_pnl)
		ON CONFLICT(job_id) DO UPDATE SET
			total_return=excluded.total_return, annual_return=excluded.annual_return,
			win_rate=excluded.win_rate, max_drawdown=excluded.max_drawdown,
			sharpe_ratio=excluded.sharpe_ratio, total_trades=excluded.total_trades,
			win_trades=excluded.win_trades, loss_trades=excluded.loss_trades,
			avg_pnl=excluded.avg_pnl`
}

func (r *backtestRepo) UpsertResult(ctx context.Context, res *BacktestResult) error {
	_, err := r.db.NamedExecContext(ctx, r.upsertResultSQL(), res)
	return err
}

func (r *backtestRepo) GetResult(ctx context.Context, jobID string) (*BacktestResult, error) {
	var result BacktestResult
	err := r.db.GetContext(ctx, &result, r.db.Rebind(`
		SELECT id, job_id, strategy, total_return, annual_return, win_rate, max_drawdown,
		       sharpe_ratio, total_trades, win_trades, loss_trades, avg_pnl, created_at
		FROM backtest_results WHERE job_id=?
	`), jobID)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// ── Trades ────────────────────────────────────────────────────

func (r *backtestRepo) InsertTrades(ctx context.Context, trades []BacktestTrade) error {
	if len(trades) == 0 {
		return nil
	}
	for _, t := range trades {
		_, err := r.db.NamedExecContext(ctx, `
			INSERT INTO backtest_trades
				(job_id, symbol, direction, entry_time, exit_time,
				 entry_price, exit_price, size, pnl, pnl_pct, commission)
			VALUES
				(:job_id, :symbol, :direction, :entry_time, :exit_time,
				 :entry_price, :exit_price, :size, :pnl, :pnl_pct, :commission)
		`, t)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *backtestRepo) GetTrades(ctx context.Context, jobID string) ([]BacktestTrade, error) {
	var trades []BacktestTrade
	err := r.db.SelectContext(ctx, &trades, r.db.Rebind(`
		SELECT id, job_id, symbol, direction, entry_time, exit_time,
		       entry_price, exit_price, size, pnl, pnl_pct, commission, created_at
		FROM backtest_trades
		WHERE job_id=?
		ORDER BY entry_time ASC
	`), jobID)
	return trades, err
}
