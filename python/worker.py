"""
Method A：DB polling worker。
每隔 WORKER_POLL_INTERVAL 秒掃描 backtest_jobs 中 status='pending' 的任務並執行。

啟動方式：
    cd python
    python worker.py
"""
from __future__ import annotations
import logging
import sys
import os

# 在所有 app import 之前設定 logging，確保 import 期間的錯誤也能顯示
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [worker] %(levelname)-8s %(name)s — %(message)s",
    datefmt="%Y-%m-%d %H:%M:%S",
    stream=sys.stdout,
    force=True,
)
logging.getLogger("sqlalchemy.engine").setLevel(logging.WARNING)

log = logging.getLogger(__name__)

import json
import time

sys.path.insert(0, os.path.dirname(__file__))
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "backtest"))

log.info("loading config...")
from config import WORKER_POLL_INTERVAL

log.info("connecting to database...")
from db import engine

from sqlalchemy import text
from backtest.engine import run_backtest
from backtest.db_writer import update_job_status, write_result, write_trades


def _fetch_pending() -> list[dict]:
    sql = text("""
        SELECT job_id, strategy, symbols, timeframe, start_date, end_date,
               use_chip_filter, chip_min_score
        FROM backtest_jobs
        WHERE status = 'pending'
        ORDER BY created_at ASC
        LIMIT 5
    """)
    with engine.connect() as conn:
        rows = conn.execute(sql).mappings().all()
    return [dict(r) for r in rows]


def _process(job: dict) -> None:
    job_id = job["job_id"]
    log.info("processing job=%s  strategy=%s  symbols=%s  timeframe=%s",
             job_id, job["strategy"], job["symbols"], job["timeframe"])

    update_job_status(job_id, "running")
    try:
        symbols = json.loads(job["symbols"])
        output = run_backtest(
            strategy=job["strategy"],
            symbols=symbols,
            timeframe=job["timeframe"],
            start_date=job["start_date"],
            end_date=job["end_date"],
            use_chip_filter=bool(job.get("use_chip_filter")),
            chip_min_score=float(job.get("chip_min_score") or 0.0),
        )
        write_result(job_id, output["result"])
        write_trades(job_id, output["trades"])
        update_job_status(job_id, "done")
        log.info("job=%s done — total_trades=%d  total_return=%.2f%%  sharpe=%.4f",
                 job_id,
                 output["result"]["total_trades"],
                 output["result"]["total_return"] * 100,
                 output["result"]["sharpe_ratio"])
    except Exception as exc:
        log.error("job=%s failed: %s", job_id, exc, exc_info=True)
        update_job_status(job_id, "failed", str(exc))


def main() -> None:
    log.info("worker started — poll_interval=%ds", WORKER_POLL_INTERVAL)
    poll_count = 0
    while True:
        poll_count += 1
        try:
            jobs = _fetch_pending()
            if jobs:
                log.info("poll #%d: found %d pending job(s)", poll_count, len(jobs))
                for job in jobs:
                    _process(job)
            else:
                log.info("poll #%d: no pending jobs, sleeping %ds",
                         poll_count, WORKER_POLL_INTERVAL)
        except Exception as exc:
            log.error("poll #%d error: %s", poll_count, exc, exc_info=True)
        time.sleep(WORKER_POLL_INTERVAL)


if __name__ == "__main__":
    main()
