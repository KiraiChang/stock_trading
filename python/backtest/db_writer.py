"""將回測結果寫回 DB（backtest_results + backtest_trades）。"""
from __future__ import annotations
import logging
from datetime import datetime

from sqlalchemy import text

import sys, os
sys.path.insert(0, os.path.dirname(os.path.dirname(__file__)))
from db import engine

log = logging.getLogger(__name__)


def write_result(job_id: str, result: dict) -> None:
    sql = text("""
        INSERT INTO backtest_results
            (job_id, strategy, total_return, annual_return, win_rate,
             max_drawdown, sharpe_ratio, total_trades, win_trades, loss_trades, avg_pnl)
        VALUES
            (:job_id, :strategy, :total_return, :annual_return, :win_rate,
             :max_drawdown, :sharpe_ratio, :total_trades, :win_trades, :loss_trades, :avg_pnl)
        ON CONFLICT(job_id) DO UPDATE SET
            total_return=excluded.total_return, annual_return=excluded.annual_return,
            win_rate=excluded.win_rate, max_drawdown=excluded.max_drawdown,
            sharpe_ratio=excluded.sharpe_ratio, total_trades=excluded.total_trades,
            win_trades=excluded.win_trades, loss_trades=excluded.loss_trades,
            avg_pnl=excluded.avg_pnl
    """)  # SQLite ON CONFLICT 語法；MySQL 需改為 ON DUPLICATE KEY UPDATE
    with engine.begin() as conn:
        conn.execute(sql, {"job_id": job_id, **result})
    log.info("write_result OK  job=%s", job_id)


def write_trades(job_id: str, trades: list[dict]) -> None:
    if not trades:
        log.info("write_trades job=%s — 0 trades, skipped", job_id)
        return
    sql = text("""
        INSERT INTO backtest_trades
            (job_id, symbol, direction, entry_time, exit_time,
             entry_price, exit_price, size, pnl, pnl_pct, commission)
        VALUES
            (:job_id, :symbol, :direction, :entry_time, :exit_time,
             :entry_price, :exit_price, :size, :pnl, :pnl_pct, :commission)
    """)
    with engine.begin() as conn:
        for t in trades:
            conn.execute(sql, {"job_id": job_id, **t})
    log.info("write_trades OK  job=%s  count=%d", job_id, len(trades))


def update_job_status(job_id: str, status: str, error: str = "") -> None:
    now = datetime.utcnow().isoformat()
    if status == "running":
        sql = text("UPDATE backtest_jobs SET status=:s, started_at=:now WHERE job_id=:id")
        params = {"s": status, "now": now, "id": job_id}
    elif status in ("done", "failed"):
        sql = text("UPDATE backtest_jobs SET status=:s, error=:e, finished_at=:now WHERE job_id=:id")
        params = {"s": status, "e": error, "now": now, "id": job_id}
    else:
        sql = text("UPDATE backtest_jobs SET status=:s WHERE job_id=:id")
        params = {"s": status, "id": job_id}
    with engine.begin() as conn:
        conn.execute(sql, params)
    log.info("job=%s status → %s", job_id, status)
