"""回測任務的狀態轉移與結果寫回（backtest_jobs／backtest_results／backtest_trades）。

兩條執行路徑（python-worker 輪詢 `pending`、python-server 的 `POST /backtest`）共用這裡的函式。
狀態轉移一律是**條件式**的 UPDATE，影響 0 列就代表這一筆已經不是自己的，⛔ 不再往下寫：

* `claim_job`：`pending` → `running`，同時寫 `execution_owner` 與 `started_at`。
* `finish_job`：結果、交易明細與 `done` 在**同一個交易**提交；`done` 的條件是「仍是自己的 `running`」。
* `fail_job`：自己的 `running` → `failed`。
* `reap_interrupted`：服務啟動時，把**同一種服務**上一個程序留下的 `running` 收成 `failed`。
  依據是「每種服務只有一個程序」，前提與會破壞它的變更見 docs/database-schema.md 的 backtest_jobs。

時間一律用 DB 的 `CURRENT_TIMESTAMP`，⛔ 不由 python 組時間字串。
"""
from __future__ import annotations

import json
import logging
import threading
import time
from typing import Callable

from sqlalchemy import text

import sys, os
sys.path.insert(0, os.path.dirname(os.path.dirname(__file__)))
from db import engine

log = logging.getLogger(__name__)

OWNERS = ("worker", "server")
INTERRUPTED_ERROR = "interrupted_on_restart"

SCHEMA_WAIT_INTERVAL_SEC = 2.0
SCHEMA_WAIT_TIMEOUT_SEC = 120.0


class SchemaNotReady(RuntimeError):
    """等不到需要的欄位：`backtest_jobs.execution_owner`（migration 077）或 `backtest_results.data_coverage`（078）。"""


class _NotOwner(Exception):
    """`done` 影響 0 列：這一筆已經不是自己的 `running`，整個交易要 rollback。"""


def _check_owner(owner: str) -> None:
    if owner not in OWNERS:
        raise ValueError(f"execution owner must be one of {OWNERS}, got {owner!r}")


_RESULT_SQL = text("""
    INSERT INTO backtest_results
        (job_id, strategy, total_return, annual_return, win_rate,
         max_drawdown, sharpe_ratio, total_trades, win_trades, loss_trades, avg_pnl, data_coverage)
    VALUES
        (:job_id, :strategy, :total_return, :annual_return, :win_rate,
         :max_drawdown, :sharpe_ratio, :total_trades, :win_trades, :loss_trades, :avg_pnl, :data_coverage)
    ON CONFLICT(job_id) DO UPDATE SET
        total_return=excluded.total_return, annual_return=excluded.annual_return,
        win_rate=excluded.win_rate, max_drawdown=excluded.max_drawdown,
        sharpe_ratio=excluded.sharpe_ratio, total_trades=excluded.total_trades,
        win_trades=excluded.win_trades, loss_trades=excluded.loss_trades,
        avg_pnl=excluded.avg_pnl, data_coverage=excluded.data_coverage
""")  # SQLite／PostgreSQL 的 ON CONFLICT 語法；MySQL 需改為 ON DUPLICATE KEY UPDATE

_TRADE_SQL = text("""
    INSERT INTO backtest_trades
        (job_id, symbol, direction, entry_time, exit_time,
         entry_price, exit_price, size, pnl, pnl_pct, commission)
    VALUES
        (:job_id, :symbol, :direction, :entry_time, :exit_time,
         :entry_price, :exit_price, :size, :pnl, :pnl_pct, :commission)
""")


def claim_job(job_id: str, owner: str) -> bool:
    """`pending` → `running`。回傳是否認領成功（影響剛好 1 列）。

    成功與失敗都記一行 log（帶 job_id 與 owner）：dev 驗收用它判斷兩條路徑是否都嘗試過同一筆。
    """
    _check_owner(owner)
    sql = text("""
        UPDATE backtest_jobs
        SET status = 'running', execution_owner = :owner, started_at = CURRENT_TIMESTAMP
        WHERE job_id = :id AND status = 'pending'
    """)
    with engine.begin() as conn:
        rows = conn.execute(sql, {"owner": owner, "id": job_id}).rowcount
    if rows == 1:
        log.info("claim ok job=%s owner=%s", job_id, owner)
        return True
    log.info("claim lost job=%s owner=%s (not pending)", job_id, owner)
    return False


def finish_job(job_id: str, owner: str, result: dict, trades: list[dict]) -> bool:
    """結果、交易明細與 `done` 在同一個交易提交。

    `done` 影響 0 列（已經不是自己的 `running`，例如被收掉或被別的路徑改過）就整個 rollback，
    結果與明細都⛔ 不留，回傳 False。順序是結果 → 明細 → `done`。
    """
    _check_owner(owner)
    done_sql = text("""
        UPDATE backtest_jobs
        SET status = 'done', error = '', finished_at = CURRENT_TIMESTAMP
        WHERE job_id = :id AND status = 'running' AND execution_owner = :owner
    """)
    params = {"job_id": job_id, **result}
    # 覆蓋範圍（I-137）存成 JSON 字串；沒有的話（例如舊的呼叫端）是 NULL
    coverage = result.get("data_coverage")
    params["data_coverage"] = json.dumps(coverage, ensure_ascii=False) if coverage is not None else None
    try:
        with engine.begin() as conn:
            conn.execute(_RESULT_SQL, params)
            for t in trades:
                conn.execute(_TRADE_SQL, {"job_id": job_id, **t})
            if conn.execute(done_sql, {"id": job_id, "owner": owner}).rowcount != 1:
                raise _NotOwner
    except _NotOwner:
        log.warning("finish rolled back job=%s owner=%s: no longer this owner's running job",
                    job_id, owner)
        return False
    log.info("job=%s status → done (owner=%s, trades=%d)", job_id, owner, len(trades))
    return True


def fail_job(job_id: str, owner: str, error: str) -> bool:
    """自己的 `running` → `failed`。影響 0 列就只記 log，⛔ 不再寫。"""
    _check_owner(owner)
    sql = text("""
        UPDATE backtest_jobs
        SET status = 'failed', error = :e, finished_at = CURRENT_TIMESTAMP
        WHERE job_id = :id AND status = 'running' AND execution_owner = :owner
    """)
    with engine.begin() as conn:
        rows = conn.execute(sql, {"e": error, "id": job_id, "owner": owner}).rowcount
    if rows == 1:
        log.info("job=%s status → failed (owner=%s)", job_id, owner)
        return True
    log.warning("fail not applied job=%s owner=%s: no longer this owner's running job",
                job_id, owner)
    return False


def reap_interrupted(owner: str) -> list[str]:
    """服務啟動時，把 `execution_owner = owner` 的遺留 `running` 收成 `failed`。

    ⚠️ 只有在「這種服務只有一個程序」時才正確：所有 owner 是自己的 `running` 都是上一個程序留下的。
    前提與會破壞它的變更（`--scale`、uvicorn `workers>1`、`--reload`、滾動更新）見
    docs/database-schema.md 的 backtest_jobs。另一種服務的與 NULL 的⛔ 不動。
    """
    _check_owner(owner)
    select_sql = text("""
        SELECT job_id FROM backtest_jobs
        WHERE status = 'running' AND execution_owner = :owner
        ORDER BY job_id
    """)
    update_sql = text("""
        UPDATE backtest_jobs
        SET status = 'failed', error = :e, finished_at = CURRENT_TIMESTAMP
        WHERE status = 'running' AND execution_owner = :owner
    """)
    with engine.begin() as conn:
        job_ids = [r[0] for r in conn.execute(select_sql, {"owner": owner})]
        conn.execute(update_sql, {"e": INTERRUPTED_ERROR, "owner": owner})
    if job_ids:
        log.warning("reaped %d interrupted job(s) owner=%s: %s",
                    len(job_ids), owner, ",".join(job_ids))
    else:
        log.info("no interrupted jobs to reap owner=%s", owner)
    return job_ids


# 兩個 python 服務在接任務之前要等的欄位（migration 由 backend 啟動時套用）。
# 078 的 data_coverage（I-137）也要等：⛔ 改成「寫入時才報錯」會讓正常任務跑完整段才因為上版時序變成 failed。
SCHEMA_REQUIRED = "backtest_jobs.execution_owner (077), backtest_results.data_coverage (078)"


def _schema_ready() -> bool:
    try:
        with engine.connect() as conn:
            conn.execute(text("SELECT execution_owner FROM backtest_jobs WHERE 1 = 0"))
            conn.execute(text("SELECT data_coverage FROM backtest_results WHERE 1 = 0"))
        return True
    except Exception as exc:  # 欄位不存在或 DB 暫時連不上，都等下一輪
        log.info("schema not ready yet: %s", type(exc).__name__)
        return False


def _probe_within(probe: Callable[[], bool], limit: float) -> bool | None:
    """在 daemon thread 裡跑探測，最多等 `limit` 秒；逾時回 None。

    DB 卡住時（連線或查詢沒有回應）探測可能遠超過整體上限，所以⛔ 不能直接呼叫。用 daemon thread：
    逾時之後呼叫端以非 0 結束，卡住的 thread ⛔ 不會擋住程序結束（ThreadPoolExecutor 的 thread 會）。
    """
    result: list[bool] = []
    th = threading.Thread(target=lambda: result.append(bool(probe())), name="schema-probe", daemon=True)
    th.start()
    th.join(max(limit, 0.0))
    if th.is_alive() or not result:
        return None
    return result[0]


def wait_for_schema(
    timeout: float = SCHEMA_WAIT_TIMEOUT_SEC,
    interval: float = SCHEMA_WAIT_INTERVAL_SEC,
    *,
    sleep: Callable[[float], None] = time.sleep,
    clock: Callable[[], float] = time.monotonic,
    probe: Callable[[], bool] = _schema_ready,
) -> None:
    """等需要的欄位出現（`SCHEMA_REQUIRED`；migration 由 backend 啟動時套用）。

    compose 裡 python ⛔ 不等 backend，新的 python 可能比 077／078 先啟動。每 `interval` 秒查一次，整體最多 `timeout` 秒：
    每次探測的等待時間是剩餘時間（探測卡住也不會超過）、探測回來之後再核對一次期限（回來時已經過期就算逾時，
    ⛔ 不回報成功）。逾時 raise `SchemaNotReady`，由呼叫端以非 0 結束、交給 compose 重啟。
    ⛔ 不在 python 端建立欄位（schema 只由 goose 管）。
    """
    deadline = clock() + timeout

    def not_ready(why: str) -> SchemaNotReady:
        return SchemaNotReady(
            f"required columns not found within {timeout:.0f}s ({why}; need {SCHEMA_REQUIRED}; "
            "backend applies migrations on startup)"
        )

    while True:
        ok = _probe_within(probe, deadline - clock())
        if ok is None:
            raise not_ready("schema probe did not return in time")
        if clock() > deadline:
            raise not_ready("schema probe returned after the deadline")
        if ok:
            log.info("schema ready: %s exist", SCHEMA_REQUIRED)
            return
        left = deadline - clock()
        if left <= 0:
            raise not_ready("timed out")
        log.info("waiting for schema (%s), retry in %.0fs", SCHEMA_REQUIRED, interval)
        sleep(min(interval, left))
