"""
Method A：DB polling worker。
每隔 WORKER_POLL_INTERVAL 秒掃描 backtest_jobs 中 status='pending' 的任務並執行。

啟動方式：
    cd python
    python worker.py
"""
import json
import logging
import time
import sys, os

sys.path.insert(0, os.path.dirname(__file__))
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "backtest"))

from sqlalchemy import text
from config import WORKER_POLL_INTERVAL
from db import engine
from backtest.engine import run_backtest
from backtest.db_writer import update_job_status, write_result, write_trades

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [worker] %(levelname)s %(message)s",
)
log = logging.getLogger(__name__)


def _fetch_pending() -> list[dict]:
    sql = text("""
        SELECT job_id, strategy, symbols, timeframe, start_date, end_date
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
    log.info("processing job %s  strategy=%s", job_id, job["strategy"])

    update_job_status(job_id, "running")
    try:
        symbols = json.loads(job["symbols"])
        output = run_backtest(
            strategy=job["strategy"],
            symbols=symbols,
            timeframe=job["timeframe"],
            start_date=job["start_date"],
            end_date=job["end_date"],
        )
        write_result(job_id, output["result"])
        write_trades(job_id, output["trades"])
        update_job_status(job_id, "done")
        log.info("job %s done — total_trades=%d  total_return=%.2f%%",
                 job_id,
                 output["result"]["total_trades"],
                 output["result"]["total_return"] * 100)
    except Exception as exc:
        log.error("job %s failed: %s", job_id, exc, exc_info=True)
        update_job_status(job_id, "failed", str(exc))


def main() -> None:
    log.info("worker started, poll_interval=%ds", WORKER_POLL_INTERVAL)
    while True:
        try:
            jobs = _fetch_pending()
            for job in jobs:
                _process(job)
        except Exception as exc:
            log.error("poll error: %s", exc, exc_info=True)
        time.sleep(WORKER_POLL_INTERVAL)


if __name__ == "__main__":
    main()
