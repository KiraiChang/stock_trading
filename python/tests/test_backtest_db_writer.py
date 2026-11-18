"""回測任務的狀態轉移（I-134）：原子認領、結果同一交易提交、條件式失敗、重啟收尾、schema 等待。

用檔案 SQLite，schema 由 repo 的 SQLite migration 建立（`backtest_sqlite` fixture，見 conftest）。
規格見 docs/database-schema.md 的 backtest_jobs。
"""
from __future__ import annotations

import threading
import time

import pytest
from sqlalchemy import text

from backtest import db_writer

RESULT = {
    "strategy": "breakout_v1",
    "total_return": 0.1,
    "annual_return": 0.2,
    "win_rate": 0.5,
    "max_drawdown": 0.05,
    "sharpe_ratio": 1.1,
    "total_trades": 2,
    "win_trades": 1,
    "loss_trades": 1,
    "avg_pnl": 10.0,
}


def _trade(i: int) -> dict:
    return {
        "symbol": "2330",
        "direction": "long",
        "entry_time": f"2026-01-0{i} 09:00:00",
        "exit_time": f"2026-01-0{i} 13:00:00",
        "entry_price": 100.0,
        "exit_price": 101.0,
        "size": 1.0,
        "pnl": 1.0,
        "pnl_pct": 0.01,
        "commission": 0.1,
    }


TRADES = [_trade(1), _trade(2)]


def _insert(eng, job_id: str, status: str = "pending", owner: str | None = None) -> None:
    with eng.begin() as conn:
        conn.execute(
            text("""
                INSERT INTO backtest_jobs
                    (job_id, type, strategy, symbols, timeframe, start_date, end_date,
                     status, trigger_source, execution_owner)
                VALUES (:id, 'backtest', 'breakout_v1', '["2330"]', '1d', '2026-01-01', '2026-06-30',
                        :st, 'manual', :owner)
            """),
            {"id": job_id, "st": status, "owner": owner},
        )


def _job(eng, job_id: str) -> dict | None:
    with eng.connect() as conn:
        row = conn.execute(
            text("SELECT * FROM backtest_jobs WHERE job_id = :id"), {"id": job_id}
        ).mappings().first()
    return dict(row) if row else None


def _count(eng, table: str, job_id: str) -> int:
    with eng.connect() as conn:
        return conn.execute(
            text(f"SELECT COUNT(*) FROM {table} WHERE job_id = :id"), {"id": job_id}
        ).scalar_one()


# ── 認領 ─────────────────────────────────────────────────────────────


@pytest.mark.parametrize("owner", ["worker", "server"])
def test_claim_pending_sets_running_owner_and_started_at(backtest_sqlite, owner):
    eng = backtest_sqlite()
    _insert(eng, "j1")

    assert db_writer.claim_job("j1", owner) is True

    job = _job(eng, "j1")
    assert job["status"] == "running"
    assert job["execution_owner"] == owner
    assert job["started_at"] is not None


@pytest.mark.parametrize(
    "status,owner",
    [("running", "server"), ("done", "server"), ("failed", None), ("running", None)],
)
def test_claim_non_pending_returns_false_and_writes_nothing(backtest_sqlite, status, owner):
    """拿掉認領的 `AND status = 'pending'`，這條會紅：非 pending 的列⛔ 不能被改寫。"""
    eng = backtest_sqlite()
    _insert(eng, "j1", status=status, owner=owner)
    before = _job(eng, "j1")

    assert db_writer.claim_job("j1", "worker") is False

    assert _job(eng, "j1") == before


def test_claim_missing_job_returns_false(backtest_sqlite):
    backtest_sqlite()
    assert db_writer.claim_job("nope", "worker") is False


def test_concurrent_claims_only_one_wins(backtest_sqlite):
    """兩條連線（兩個執行緒）同時認領同一筆，只有一個成功，owner 是贏的那一方。"""
    eng = backtest_sqlite()
    for i in range(20):
        _insert(eng, f"race{i}")

    for i in range(20):
        job_id = f"race{i}"
        barrier = threading.Barrier(2)
        results: dict[str, bool] = {}

        def attempt(owner: str) -> None:
            barrier.wait()
            results[owner] = db_writer.claim_job(job_id, owner)

        threads = [threading.Thread(target=attempt, args=(o,)) for o in ("worker", "server")]
        for t in threads:
            t.start()
        for t in threads:
            t.join()

        winners = [o for o, ok in results.items() if ok]
        assert len(winners) == 1, results
        assert _job(eng, job_id)["execution_owner"] == winners[0]


def test_claim_logs_ok_and_lost_with_job_id(backtest_sqlite, caplog):
    """dev 驗收靠這兩行判斷兩條路徑是否都嘗試過同一筆。"""
    eng = backtest_sqlite()
    _insert(eng, "j1")

    with caplog.at_level("INFO", logger=db_writer.log.name):
        db_writer.claim_job("j1", "worker")
        db_writer.claim_job("j1", "server")

    messages = [r.getMessage() for r in caplog.records]
    assert "claim ok job=j1 owner=worker" in messages
    assert any(m.startswith("claim lost job=j1 owner=server") for m in messages)


# ── 完成：結果、明細與 done 同一個交易 ─────────────────────────────────


def test_finish_commits_result_trades_and_done_together(backtest_sqlite):
    eng = backtest_sqlite()
    _insert(eng, "j1", status="running", owner="worker")

    assert db_writer.finish_job("j1", "worker", RESULT, TRADES) is True

    job = _job(eng, "j1")
    assert job["status"] == "done"
    assert job["finished_at"] is not None
    assert _count(eng, "backtest_results", "j1") == 1
    assert _count(eng, "backtest_trades", "j1") == 2


def test_finish_failure_while_writing_trades_rolls_back_result(backtest_sqlite):
    """寫明細時出錯：結果（已在同一個交易寫入）也要回滾，狀態仍是 running。

    把 `finish_job` 拆成兩個交易（結果先提交），這條會紅。
    """
    eng = backtest_sqlite()
    _insert(eng, "j1", status="running", owner="worker")
    bad = [_trade(1), {k: v for k, v in _trade(2).items() if k != "symbol"}]  # 缺 bind 參數

    with pytest.raises(Exception):
        db_writer.finish_job("j1", "worker", RESULT, bad)

    assert _count(eng, "backtest_results", "j1") == 0
    assert _count(eng, "backtest_trades", "j1") == 0
    job = _job(eng, "j1")
    assert job["status"] == "running"
    assert job["execution_owner"] == "worker"


@pytest.mark.parametrize(
    "status,owner",
    [("running", "server"), ("failed", "worker"), ("done", "worker"), ("pending", None)],
)
def test_finish_when_not_own_running_rolls_back(backtest_sqlite, status, owner):
    """已不是自己的 running（被收掉、被別的路徑改過）：結果與明細⛔ 不留，回傳 False。"""
    eng = backtest_sqlite()
    _insert(eng, "j1", status=status, owner=owner)
    before = _job(eng, "j1")

    assert db_writer.finish_job("j1", "worker", RESULT, TRADES) is False

    assert _job(eng, "j1") == before
    assert _count(eng, "backtest_results", "j1") == 0
    assert _count(eng, "backtest_trades", "j1") == 0


def test_finish_with_no_trades_still_marks_done(backtest_sqlite):
    eng = backtest_sqlite()
    _insert(eng, "j1", status="running", owner="server")

    assert db_writer.finish_job("j1", "server", RESULT, []) is True

    assert _job(eng, "j1")["status"] == "done"
    assert _count(eng, "backtest_results", "j1") == 1


# ── 失敗 ─────────────────────────────────────────────────────────────


def test_fail_own_running(backtest_sqlite):
    eng = backtest_sqlite()
    _insert(eng, "j1", status="running", owner="server")

    assert db_writer.fail_job("j1", "server", "boom") is True

    job = _job(eng, "j1")
    assert (job["status"], job["error"]) == ("failed", "boom")
    assert job["finished_at"] is not None


@pytest.mark.parametrize("status,owner", [("running", "worker"), ("done", "server"), ("pending", None)])
def test_fail_not_own_running_writes_nothing(backtest_sqlite, status, owner):
    eng = backtest_sqlite()
    _insert(eng, "j1", status=status, owner=owner)
    before = _job(eng, "j1")

    assert db_writer.fail_job("j1", "server", "boom") is False

    assert _job(eng, "j1") == before


# ── 重啟收尾 ─────────────────────────────────────────────────────────


@pytest.mark.parametrize("owner,other", [("worker", "server"), ("server", "worker")])
def test_reap_only_touches_own_running(backtest_sqlite, owner, other):
    """拿掉收尾的 owner 條件，這條會紅：另一種服務的與 NULL 的 running ⛔ 不動。"""
    eng = backtest_sqlite()
    _insert(eng, "mine1", status="running", owner=owner)
    _insert(eng, "mine2", status="running", owner=owner)
    _insert(eng, "theirs", status="running", owner=other)
    _insert(eng, "legacy", status="running", owner=None)
    _insert(eng, "queued", status="pending")
    _insert(eng, "finished", status="done", owner=owner)
    untouched = {j: _job(eng, j) for j in ("theirs", "legacy", "queued", "finished")}

    assert db_writer.reap_interrupted(owner) == ["mine1", "mine2"]

    for j in ("mine1", "mine2"):
        job = _job(eng, j)
        assert (job["status"], job["error"]) == ("failed", db_writer.INTERRUPTED_ERROR)
        assert job["finished_at"] is not None
        assert job["execution_owner"] == owner
    for j, before in untouched.items():
        assert _job(eng, j) == before


def test_reap_with_nothing_to_do(backtest_sqlite):
    backtest_sqlite()
    assert db_writer.reap_interrupted("worker") == []


def test_reaped_job_cannot_be_finished_by_old_owner(backtest_sqlite):
    """收掉之後，舊程序（若其實還活著）的 finish 會被擋下、整個回滾。"""
    eng = backtest_sqlite()
    _insert(eng, "j1", status="running", owner="worker")
    db_writer.reap_interrupted("worker")

    assert db_writer.finish_job("j1", "worker", RESULT, TRADES) is False
    assert _count(eng, "backtest_results", "j1") == 0


# ── owner 的值檢查 ───────────────────────────────────────────────────


@pytest.mark.parametrize(
    "call",
    [
        lambda: db_writer.claim_job("j1", "bogus"),
        lambda: db_writer.finish_job("j1", "", RESULT, TRADES),
        lambda: db_writer.fail_job("j1", "Worker", "x"),
        lambda: db_writer.reap_interrupted(None),
    ],
)
def test_owner_must_be_worker_or_server(backtest_sqlite, call):
    backtest_sqlite()
    with pytest.raises(ValueError):
        call()


# ── schema 等待 ─────────────────────────────────────────────────────


class _FakeClock:
    def __init__(self):
        self.now = 0.0
        self.sleeps: list[float] = []

    def __call__(self) -> float:
        return self.now

    def sleep(self, sec: float) -> None:
        self.sleeps.append(sec)
        self.now += sec


def test_wait_for_schema_returns_immediately_when_column_exists(backtest_sqlite):
    backtest_sqlite()
    clock = _FakeClock()

    db_writer.wait_for_schema(sleep=clock.sleep, clock=clock)

    assert clock.sleeps == []


def test_wait_for_schema_times_out_without_077(backtest_sqlite):
    backtest_sqlite(upto=76)
    clock = _FakeClock()

    with pytest.raises(db_writer.SchemaNotReady):
        db_writer.wait_for_schema(timeout=120, interval=2, sleep=clock.sleep, clock=clock)

    assert clock.sleeps == [2] * 60


def test_wait_for_schema_continues_once_column_appears(backtest_sqlite):
    eng = backtest_sqlite(upto=76)
    clock = _FakeClock()

    def sleep_then_migrate(sec: float) -> None:
        clock.sleep(sec)
        if len(clock.sleeps) == 3:
            with eng.begin() as conn:
                conn.execute(text("ALTER TABLE backtest_jobs ADD COLUMN execution_owner TEXT"))

    db_writer.wait_for_schema(timeout=120, interval=2, sleep=sleep_then_migrate, clock=clock)

    assert clock.sleeps == [2, 2, 2]


def test_wait_for_schema_bounds_a_hanging_probe():
    """探測卡住（DB 沒有回應）時，整體仍在上限內結束（review：查詢沒有逾時，模擬卡 300 秒仍會等下去）。"""
    release = threading.Event()

    def hanging_probe() -> bool:
        release.wait(30)
        return True

    t0 = time.monotonic()
    try:
        with pytest.raises(db_writer.SchemaNotReady, match="did not return in time"):
            db_writer.wait_for_schema(timeout=0.3, interval=0.1, probe=hanging_probe)
    finally:
        release.set()
    assert time.monotonic() - t0 < 2


def test_wait_for_schema_success_after_deadline_is_timeout():
    """探測回來時已經過了期限：算逾時，⛔ 不回報成功（review：探測耗時 300 秒、上限 120 秒仍回報成功）。"""
    clock = _FakeClock()

    def slow_probe() -> bool:
        clock.now += 300
        return True

    with pytest.raises(db_writer.SchemaNotReady, match="after the deadline"):
        db_writer.wait_for_schema(timeout=120, interval=2, sleep=clock.sleep, clock=clock, probe=slow_probe)


def test_wait_for_schema_last_sleep_is_clipped_to_deadline():
    """最後一次等待只睡到期限為止，總時間⛔ 不超過上限。"""
    clock = _FakeClock()

    with pytest.raises(db_writer.SchemaNotReady):
        db_writer.wait_for_schema(timeout=5, interval=2, sleep=clock.sleep, clock=clock, probe=lambda: False)

    assert clock.sleeps == [2, 2, 1]
    assert clock.now == 5
