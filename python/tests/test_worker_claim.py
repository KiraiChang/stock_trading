"""python-worker 的認領與啟動順序（I-134）。

* 每一筆先認領；認領失敗（已被 python-server 認領或已不是 pending）⛔ 不執行。
* 啟動：連線檢查 → 等 schema → 收掉上一個 worker 留下的 running → 才開始輪詢；等不到 schema 以非 0 結束。
"""
from __future__ import annotations

import datetime

import pytest

import worker

JOB = {
    "job_id": "j1",
    "strategy": "breakout_v1",
    "symbols": '["2330"]',
    "timeframe": "1d",
    "start_date": "2026-01-01",
    "end_date": "2026-06-30",
    "use_chip_filter": False,
    "chip_min_score": 0,
}

OUTPUT = {"result": {"total_trades": 0, "total_return": 0.0, "sharpe_ratio": 0.0}, "trades": []}


@pytest.fixture
def calls(monkeypatch) -> list:
    log: list = []
    monkeypatch.setattr(worker, "run_backtest", lambda **kw: log.append(("run", kw["strategy"])) or OUTPUT)
    monkeypatch.setattr(
        worker, "finish_job", lambda job_id, owner, result, trades: log.append(("finish", job_id, owner)) or True
    )
    monkeypatch.setattr(worker, "fail_job", lambda job_id, owner, err: log.append(("fail", job_id, owner, err)))
    return log


def test_lost_claim_is_not_executed(monkeypatch, calls):
    monkeypatch.setattr(worker, "claim_job", lambda job_id, owner: calls.append(("claim", job_id, owner)) and False)

    worker._process(dict(JOB))

    assert calls == [("claim", "j1", "worker")]


def test_won_claim_runs_then_finishes_as_worker(monkeypatch, calls):
    monkeypatch.setattr(worker, "claim_job", lambda job_id, owner: calls.append(("claim", job_id, owner)) or True)

    worker._process(dict(JOB))

    assert calls == [("claim", "j1", "worker"), ("run", "breakout_v1"), ("finish", "j1", "worker")]


@pytest.mark.parametrize("raw", ['["2330", "0050"]', ["2330", "0050"]])
def test_symbols_accepts_json_string_and_list(monkeypatch, raw):
    """PostgreSQL 的 JSONB 經 psycopg2 讀出來是 list；SQLite／MySQL 是 JSON 字串。兩種都要交給回測成 list。"""
    seen: list = []
    monkeypatch.setattr(worker, "claim_job", lambda job_id, owner: True)
    monkeypatch.setattr(worker, "run_backtest", lambda **kw: seen.append(kw["symbols"]) or OUTPUT)
    monkeypatch.setattr(worker, "finish_job", lambda *a: True)
    monkeypatch.setattr(worker, "fail_job", lambda *a: pytest.fail(f"fail_job called: {a}"))

    worker._process({**JOB, "symbols": raw})

    assert seen == [["2330", "0050"]]


@pytest.mark.parametrize(
    "start,end",
    [("2026-01-01", "2026-06-30"), (datetime.date(2026, 1, 1), datetime.date(2026, 6, 30))],
)
def test_dates_are_passed_as_iso_strings(monkeypatch, start, end):
    """PostgreSQL 的 DATE 經 psycopg2 讀出來是 date 物件；回測一律拿到 `YYYY-MM-DD` 字串。"""
    seen: list = []
    monkeypatch.setattr(worker, "claim_job", lambda job_id, owner: True)
    monkeypatch.setattr(worker, "run_backtest", lambda **kw: seen.append((kw["start_date"], kw["end_date"])) or OUTPUT)
    monkeypatch.setattr(worker, "finish_job", lambda *a: True)
    monkeypatch.setattr(worker, "fail_job", lambda *a: pytest.fail(f"fail_job called: {a}"))

    worker._process({**JOB, "start_date": start, "end_date": end})

    assert seen == [("2026-01-01", "2026-06-30")]


def test_run_error_fails_as_worker(monkeypatch, calls):
    monkeypatch.setattr(worker, "claim_job", lambda job_id, owner: True)

    def boom(**kw):
        raise RuntimeError("no candles")

    monkeypatch.setattr(worker, "run_backtest", boom)

    worker._process(dict(JOB))

    assert calls == [("fail", "j1", "worker", "no candles")]


class _StopLoop(BaseException):
    """跳出 main 的無窮迴圈（poll 迴圈只攔 Exception）。"""


def test_startup_order_schema_then_reap_then_poll(monkeypatch):
    order: list = []
    monkeypatch.setattr(worker, "check_connection", lambda: order.append("check"))
    monkeypatch.setattr(worker, "wait_for_schema", lambda: order.append("schema"))
    monkeypatch.setattr(worker, "reap_interrupted", lambda owner: order.append(("reap", owner)) or [])

    def first_poll():
        order.append("poll")
        raise _StopLoop

    monkeypatch.setattr(worker, "_fetch_pending", first_poll)

    with pytest.raises(_StopLoop):
        worker.main()

    assert order == ["check", "schema", ("reap", "worker"), "poll"]


def test_schema_timeout_exits_nonzero_without_reap_or_poll(monkeypatch):
    order: list = []
    monkeypatch.setattr(worker, "check_connection", lambda: None)

    def not_ready():
        raise worker.SchemaNotReady("missing")

    monkeypatch.setattr(worker, "wait_for_schema", not_ready)
    monkeypatch.setattr(worker, "reap_interrupted", lambda owner: order.append("reap"))
    monkeypatch.setattr(worker, "_fetch_pending", lambda: order.append("poll") or [])

    with pytest.raises(SystemExit) as exc:
        worker.main()

    assert exc.value.code == 1
    assert order == []


def test_worker_and_server_claim_real_db_once(backtest_sqlite, monkeypatch):
    """同一筆用真的 DB：server 先認領之後，worker 的 `_process` ⛔ 不執行（不呼叫 run_backtest）。"""
    from sqlalchemy import text

    from backtest import db_writer

    eng = backtest_sqlite()
    with eng.begin() as conn:
        conn.execute(text("""
            INSERT INTO backtest_jobs (job_id, type, strategy, symbols, timeframe, start_date, end_date,
                                       status, trigger_source)
            VALUES ('j1', 'backtest', 'breakout_v1', '["2330"]', '1d', '2026-01-01', '2026-06-30',
                    'pending', 'manual')
        """))
    assert db_writer.claim_job("j1", "server") is True

    ran: list = []
    monkeypatch.setattr(worker, "run_backtest", lambda **kw: ran.append(kw) or OUTPUT)

    worker._process(dict(JOB))

    assert ran == []
