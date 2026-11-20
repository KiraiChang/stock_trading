"""GET /backtest/{job_id} 的查無映射與對外欄位改名。

**`trigger_source` → `trigger` 的改名是這支端點唯一的轉換**，而且是必要的：
`trigger` 是 MySQL 保留字，DB 欄位因此改名（migration 059），但對外欄位名維持
`trigger`——Go 端 `store.BacktestJob` 的 json tag 就是這樣。

**回應契約（I-134）**：job 的鍵集合要剛好等於對外欄位——內部欄位（`execution_owner`）⛔ 不能出現。
SQL 明列欄位，payload 也只留對外的鍵，DB 列多出欄位時一樣不會漏出去。
"""
from __future__ import annotations

import pytest

import http_server

ENDPOINT = "/backtest/{job_id}"


class _FakeResult:
    def __init__(self, row):
        self._row = row

    def mappings(self):
        return self

    def first(self):
        return self._row


class _FakeConn:
    """依 SQL 內容分派：先查 backtest_jobs，找到才查 backtest_results。"""

    def __init__(self, job_row, result_row):
        self._job_row = job_row
        self._result_row = result_row
        self.executed: list[str] = []

    def execute(self, sql, params):
        statement = str(sql)
        self.executed.append(statement)
        if "backtest_jobs" in statement:
            return _FakeResult(self._job_row)
        return _FakeResult(self._result_row)

    def __enter__(self):
        return self

    def __exit__(self, *exc):
        return False


class _FakeEngine:
    def __init__(self, conn):
        self._conn = conn

    def connect(self):
        return self._conn


@pytest.fixture
def fake_engine(monkeypatch):
    """把 http_server.engine 換掉——這支端點是全檔唯一直接碰 DB 的，
    conftest 的 client fixture 不跑 lifespan，所以沒有真實連線可用。"""

    def _install(job_row=None, result_row=None):
        conn = _FakeConn(job_row, result_row)
        monkeypatch.setattr(http_server, "engine", _FakeEngine(conn))
        return conn

    return _install


def test_unknown_job_maps_to_404(client, fake_engine):
    conn = fake_engine(job_row=None)

    response = client.get(ENDPOINT.format(job_id="nope"))

    assert response.status_code == 404
    assert response.json()["detail"] == "Job not found"
    # 找不到 job 就不該再去查 results
    assert all("backtest_results" not in sql for sql in conn.executed)


def test_trigger_source_is_renamed_to_trigger(client, fake_engine):
    """DB 欄位 `trigger_source` 對外要變回 `trigger`，且原名不留在回應裡。"""
    fake_engine(
        job_row={"job_id": "job-1", "status": "done", "trigger_source": "manual"},
        result_row=None,
    )

    response = client.get(ENDPOINT.format(job_id="job-1"))

    assert response.status_code == 200
    job = response.json()["job"]
    assert job["trigger"] == "manual"
    assert "trigger_source" not in job


def test_result_is_null_when_not_written_yet(client, fake_engine):
    """job 還在跑時 backtest_results 沒有列——result 要是 null 而不是 404 或 {}。"""
    fake_engine(job_row={"job_id": "job-1", "status": "running"}, result_row=None)

    response = client.get(ENDPOINT.format(job_id="job-1"))

    assert response.status_code == 200
    assert response.json()["result"] is None
    assert response.json()["job"]["status"] == "running"


def test_result_is_returned_when_present(client, fake_engine):
    fake_engine(
        job_row={"job_id": "job-1", "status": "done"},
        result_row={"job_id": "job-1", "total_trades": 12, "sharpe_ratio": 1.23},
    )

    response = client.get(ENDPOINT.format(job_id="job-1"))

    assert response.status_code == 200
    assert response.json()["result"]["total_trades"] == 12
    assert response.json()["result"]["sharpe_ratio"] == 1.23


def test_job_without_trigger_source_passes_through(client, fake_engine):
    """沒有 trigger_source 欄位時不該憑空生出 trigger 鍵。"""
    fake_engine(job_row={"job_id": "job-1", "status": "done"}, result_row=None)

    response = client.get(ENDPOINT.format(job_id="job-1"))

    assert response.status_code == 200
    assert "trigger" not in response.json()["job"]


# 對外的 job 鍵集合，跟 Go 端 `store.BacktestJob` 的 json tag 同一組。
PUBLIC_JOB_KEYS = {
    "id", "job_id", "type", "strategy", "symbols", "timeframe", "start_date", "end_date",
    "status", "trigger", "error", "use_chip_filter", "chip_min_score",
    "created_at", "started_at", "finished_at",
}


def _full_db_row(**extra) -> dict:
    row = {k: None for k in PUBLIC_JOB_KEYS if k != "trigger"}
    row.update(job_id="job-1", status="done", trigger_source="manual")
    row.update(extra)
    return row


def test_job_keys_are_exactly_the_public_contract(client, fake_engine):
    """DB 列帶著內部欄位（含之後才加的欄位）：回應的鍵集合仍剛好是對外那一組。"""
    fake_engine(
        job_row=_full_db_row(execution_owner="server", some_future_internal_column=1),
        result_row=None,
    )

    response = client.get(ENDPOINT.format(job_id="job-1"))

    assert response.status_code == 200
    assert set(response.json()["job"]) == PUBLIC_JOB_KEYS


def test_job_query_lists_columns_and_never_selects_owner(client, fake_engine):
    conn = fake_engine(job_row=_full_db_row(), result_row=None)

    client.get(ENDPOINT.format(job_id="job-1"))

    job_sql = next(sql for sql in conn.executed if "backtest_jobs" in sql)
    assert "*" not in job_sql
    assert "execution_owner" not in job_sql


def test_job_query_against_real_schema_returns_public_keys(client, backtest_sqlite, monkeypatch):
    """用真的 schema（含 077）：明列的欄位都存在，回應的鍵集合剛好是對外那一組。"""
    from sqlalchemy import text

    eng = backtest_sqlite()
    with eng.begin() as conn:
        conn.execute(text("""
            INSERT INTO backtest_jobs (job_id, type, strategy, symbols, timeframe, start_date, end_date,
                                       status, trigger_source, execution_owner)
            VALUES ('job-1', 'backtest', 'breakout_v1', '["2330"]', '1d', '2026-01-01', '2026-06-30',
                    'running', 'manual', 'server')
        """))
    monkeypatch.setattr(http_server, "engine", eng)

    response = client.get(ENDPOINT.format(job_id="job-1"))

    assert response.status_code == 200
    assert set(response.json()["job"]) == PUBLIC_JOB_KEYS


# ── I-137：error 只回分類過的代碼、data_coverage 是陣列或 null ─────────────────

import json  # noqa: E402
from pathlib import Path  # noqa: E402

_CASES = json.loads((Path(__file__).resolve().parent / "data" / "backtest_job_error_cases.json").read_text(encoding="utf-8"))


@pytest.mark.parametrize("case", _CASES, ids=lambda c: repr(c["input"])[:40])
def test_error_follows_the_shared_public_rule(client, fake_engine, case):
    """跟 Go 的兩個 GET 同一份案例檔：舊的原始文字（含 SQL）改回 internal_error，合法代碼、取消與中斷照舊。"""
    fake_engine(job_row={"job_id": "job-1", "status": "failed", "error": case["input"]}, result_row=None)

    response = client.get(ENDPOINT.format(job_id="job-1"))

    assert response.status_code == 200
    assert response.json()["job"]["error"] == case["want"]


def test_raw_exception_text_never_reaches_the_response(client, fake_engine):
    raw = "(psycopg2.errors.UndefinedColumn) column \"secret_col\" does not exist [SQL: SELECT secret_col FROM t]"
    fake_engine(job_row={"job_id": "job-1", "status": "failed", "error": raw}, result_row=None)

    response = client.get(ENDPOINT.format(job_id="job-1"))

    assert "secret_col" not in response.text and "SELECT" not in response.text
    assert response.json()["job"]["error"] == "internal_error"


def test_data_coverage_is_parsed_into_a_list(client, fake_engine):
    cov = [{"symbol": "2330", "first_bar": "2024-01-02", "last_bar": "2025-12-31", "bars": 487,
            "warmup_bars": 250, "warmup_required": 250, "skipped": False}]
    fake_engine(job_row={"job_id": "job-1", "status": "done"},
                result_row={"job_id": "job-1", "total_trades": 3, "data_coverage": json.dumps(cov)})

    response = client.get(ENDPOINT.format(job_id="job-1"))

    assert response.json()["result"]["data_coverage"] == cov  # 陣列，⛔ 不是字串


def test_data_coverage_of_old_results_is_null(client, fake_engine):
    fake_engine(job_row={"job_id": "job-1", "status": "done"},
                result_row={"job_id": "job-1", "total_trades": 3, "data_coverage": None})

    response = client.get(ENDPOINT.format(job_id="job-1"))

    assert response.json()["result"]["data_coverage"] is None


def test_finish_job_writes_coverage_and_get_returns_it(client, backtest_sqlite, monkeypatch):
    """真的 schema（含 078）：finish_job 把覆蓋範圍寫成 JSON 字串，GET 讀回陣列。"""
    from sqlalchemy import text

    from backtest import db_writer

    eng = backtest_sqlite()
    with eng.begin() as conn:
        conn.execute(text("""
            INSERT INTO backtest_jobs (job_id, type, strategy, symbols, timeframe, start_date, end_date,
                                       status, trigger_source, execution_owner)
            VALUES ('job-1', 'backtest', 'breakout_v1', '["2330"]', '1d', '2026-01-01', '2026-06-30',
                    'running', 'manual', 'server')
        """))
    monkeypatch.setattr(http_server, "engine", eng)
    cov = [{"symbol": "2330", "first_bar": "2026-01-02", "last_bar": "2026-06-30", "bars": 120,
            "warmup_bars": 250, "warmup_required": 250, "skipped": False}]
    result = {"strategy": "breakout_v1", "total_return": 0.0, "annual_return": 0.0, "win_rate": 0.0,
              "max_drawdown": 0.0, "sharpe_ratio": 0.0, "total_trades": 0, "win_trades": 0, "loss_trades": 0,
              "avg_pnl": 0.0, "data_coverage": cov}
    assert db_writer.finish_job("job-1", "server", result, [])

    body = client.get(ENDPOINT.format(job_id="job-1")).json()

    assert body["result"]["data_coverage"] == cov
    assert body["job"]["status"] == "done"
