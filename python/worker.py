"""
Method A：DB polling worker。
每隔 WORKER_POLL_INTERVAL 秒掃描 backtest_jobs 中 status='pending' 的任務並執行。

每一筆執行前先**認領**（條件式的 `pending` → `running`）；python-server 的 `POST /backtest`
可能已經先認領同一筆，認領失敗就跳過。啟動時先等 schema（migration 077），再收掉上一個
worker 程序留下的 `running`（見 backtest/db_writer.py）。

啟動方式：
    cd python
    python worker.py
"""
from __future__ import annotations
import logging
import sys
import os

sys.path.insert(0, os.path.dirname(__file__))
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "backtest"))

from logging_setup import configure_logging

configure_logging("python-worker")
logging.getLogger("sqlalchemy.engine").setLevel(logging.WARNING)

log = logging.getLogger(__name__)

import json
import time

log.info("loading config...")
from config import WORKER_POLL_INTERVAL

log.info("connecting to database...")
from db import engine, check_connection

from sqlalchemy import text
from backtest.engine import run_backtest
from backtest.db_writer import (
    SchemaNotReady,
    claim_job,
    fail_job,
    finish_job,
    reap_interrupted,
    wait_for_schema,
)

OWNER = "worker"


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


def _parse_symbols(raw) -> list[str]:
    """`backtest_jobs.symbols` 在 PostgreSQL 是 JSONB，psycopg2 讀出來已經是 list；SQLite／MySQL 是 JSON 字串。

    只有字串才 `json.loads`（I-134 dev 實跑發現：原本一律 `json.loads`，PostgreSQL 上 worker 的每一筆都失敗）。
    """
    if isinstance(raw, str):
        return json.loads(raw)
    return list(raw)


def _date_str(raw) -> str:
    """`start_date`／`end_date` 在 PostgreSQL 是 DATE，psycopg2 讀出來是 `datetime.date`；回測要的是 `YYYY-MM-DD` 字串
    （modular 回測用字串切帶時區的索引，給 date 物件會丟 tz-naive／tz-aware 比較的錯；python-server 收到的本來就是字串）。"""
    if isinstance(raw, str):
        return raw
    return raw.isoformat()


def _process(job: dict) -> None:
    job_id = job["job_id"]
    if not claim_job(job_id, OWNER):
        return  # 已被 python-server 認領或已不是 pending；claim_job 已記 log
    log.info("processing job=%s  strategy=%s  symbols=%s  timeframe=%s",
             job_id, job["strategy"], job["symbols"], job["timeframe"])

    try:
        symbols = _parse_symbols(job["symbols"])
        output = run_backtest(
            strategy=job["strategy"],
            symbols=symbols,
            timeframe=job["timeframe"],
            start_date=_date_str(job["start_date"]),
            end_date=_date_str(job["end_date"]),
            use_chip_filter=bool(job.get("use_chip_filter")),
            chip_min_score=float(job.get("chip_min_score") or 0.0),
        )
        if not finish_job(job_id, OWNER, output["result"], output["trades"]):
            return  # 已不是自己的 running：結果與明細已 rollback，finish_job 已記 log
        log.info("job=%s done — total_trades=%d  total_return=%.2f%%  sharpe=%.4f",
                 job_id,
                 output["result"]["total_trades"],
                 output["result"]["total_return"] * 100,
                 output["result"]["sharpe_ratio"])
    except Exception as exc:
        log.error("job=%s failed: %s", job_id, exc, exc_info=True)
        fail_job(job_id, OWNER, str(exc))


def main() -> None:
    check_connection()
    try:
        wait_for_schema()
    except SchemaNotReady as exc:
        log.error("worker exiting: %s", exc)
        sys.exit(1)
    reap_interrupted(OWNER)
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
