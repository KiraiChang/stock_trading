"""POST /backtest 的 symbols 解析與背景排程接線。

`symbols` 用 `field_validator` 同時接受 **JSON string 與 list**——Go 端把它塞成
字串送過來，人手打的請求則習慣送 list。少了測試，把 validator 拿掉只會在 Go 那條
路徑上炸掉，而所有既有測試都還是綠的。

**關於「background task 不阻塞」能驗到什麼**：TestClient 會在回應產生**之後**、
`client.post()` 回來**之前**執行 background task，所以這裡無法用時序證明不阻塞。
能鎖住的是「工作有被排進背景、且回應內容不含執行結果」——真正的不阻塞來自
`BackgroundTasks.add_task` 而不是 `await`，那是結構上的事實。

**認領（I-134）**：回應之前同步認領（`pending` → `running`／`server`），成功才排背景工作、回 202；
認領失敗回 409、⛔ 不排背景工作。認領在回應之前提交，所以回應遺失時任務已經是 `running`／`server`，
worker 撈不到。
"""
from __future__ import annotations

import json

import pytest

import http_server

ENDPOINT = "/backtest"

BASE_PAYLOAD = {
    "job_id": "job-1",
    "strategy": "breakout_v1",
    "symbols": ["2330", "0050"],
    "start_date": "2026-01-01",
    "end_date": "2026-06-30",
}


@pytest.fixture
def claims(monkeypatch) -> list:
    """把認領換成記錄器（預設成功）；`claims.result` 可改成 False 模擬認領失敗。"""

    class _Claims(list):
        result = True

    calls = _Claims()

    def fake_claim(job_id, owner):
        calls.append((job_id, owner))
        return calls.result

    monkeypatch.setattr(http_server, "claim_job", fake_claim)
    return calls


@pytest.fixture
def scheduled(monkeypatch, claims) -> list:
    """把真正跑回測的 `_run_and_write` 換成記錄器，記下它收到的 request 物件。"""
    calls: list = []
    monkeypatch.setattr(http_server, "_run_and_write", lambda req: calls.append(req))
    return calls


def test_returns_202_with_job_id(client, scheduled):
    """狀態碼是 202（Accepted）不是 200——語意是「收下了，還沒做完」。"""
    response = client.post(ENDPOINT, json=BASE_PAYLOAD)

    assert response.status_code == 202
    assert response.json() == {"job_id": "job-1", "status": "running"}


def test_symbols_accepts_json_string(client, scheduled):
    """Go 端送的是 JSON string，validator 要把它 parse 成 list 再交給下游。"""
    payload = {**BASE_PAYLOAD, "symbols": json.dumps(["2330", "0050"])}

    response = client.post(ENDPOINT, json=payload)

    assert response.status_code == 202
    assert scheduled[0].symbols == ["2330", "0050"]


def test_symbols_accepts_list(client, scheduled):
    response = client.post(ENDPOINT, json=BASE_PAYLOAD)

    assert response.status_code == 202
    assert scheduled[0].symbols == ["2330", "0050"]


def test_optional_chip_filter_defaults(client, scheduled):
    """省略籌碼過濾時預設關閉、門檻 0——開著會靜默改變回測母體。"""
    response = client.post(ENDPOINT, json=BASE_PAYLOAD)

    assert response.status_code == 202
    req = scheduled[0]
    assert req.use_chip_filter is False
    assert req.chip_min_score == 0.0
    assert req.timeframe == "1d"


def test_work_is_handed_to_background_not_run_inline(client, scheduled):
    """回應不帶任何執行結果——只有 job_id 與 running。

    這條在防的是「順手改成同步回傳結果」：那會讓 Go 端的 HTTP timeout 變成
    回測時長的函數，而回測可能跑數十秒到數分鐘。
    """
    response = client.post(ENDPOINT, json=BASE_PAYLOAD)

    assert set(response.json()) == {"job_id", "status"}
    assert scheduled[0].job_id == "job-1"


def test_missing_required_field_rejected(client, scheduled):
    """job_id / strategy / 日期都是必填，缺了要在進端點主體前被擋（422）。"""
    payload = {k: v for k, v in BASE_PAYLOAD.items() if k != "job_id"}

    response = client.post(ENDPOINT, json=payload)

    assert response.status_code == 422
    assert scheduled == []


def test_claims_as_server_before_scheduling(client, claims, monkeypatch):
    """認領在排背景工作之前、以 server 的身分。"""
    order: list = []
    monkeypatch.setattr(http_server, "_run_and_write", lambda req: order.append(("run", req.job_id)))
    real_claim = http_server.claim_job

    def claim_then_mark(job_id, owner):
        order.append(("claim", job_id, owner))
        return real_claim(job_id, owner)

    monkeypatch.setattr(http_server, "claim_job", claim_then_mark)

    response = client.post(ENDPOINT, json=BASE_PAYLOAD)

    assert response.status_code == 202
    assert order == [("claim", "job-1", "server"), ("run", "job-1")]


def test_lost_claim_returns_409_and_schedules_nothing(client, claims, scheduled):
    claims.result = False

    response = client.post(ENDPOINT, json=BASE_PAYLOAD)

    assert response.status_code == 409
    assert claims == [("job-1", "server")]
    assert scheduled == []


def test_claim_committed_before_response_survives_handler_failure(backtest_sqlite, monkeypatch):
    """認領提交之後、回應之前 handler 就失敗：任務仍是 running／server，worker 的認領回 False。

    這是「HTTP 回應遺失時 worker ⛔ 不重跑」的不變式（dev 驗收另外用不讀回應的 socket 驗一次）。
    """
    from fastapi import BackgroundTasks
    from fastapi.testclient import TestClient
    from sqlalchemy import text

    from backtest import db_writer

    eng = backtest_sqlite()
    with eng.begin() as conn:
        conn.execute(text("""
            INSERT INTO backtest_jobs (job_id, type, strategy, symbols, timeframe, start_date, end_date,
                                       status, trigger_source)
            VALUES ('job-1', 'backtest', 'breakout_v1', '["2330"]', '1d', '2026-01-01', '2026-06-30',
                    'pending', 'manual')
        """))

    def broken_add_task(self, *args, **kwargs):
        raise RuntimeError("handler died after claim")

    monkeypatch.setattr(BackgroundTasks, "add_task", broken_add_task)

    response = TestClient(http_server.app, raise_server_exceptions=False).post(ENDPOINT, json=BASE_PAYLOAD)

    assert response.status_code == 500
    with eng.connect() as conn:
        row = conn.execute(text(
            "SELECT status, execution_owner FROM backtest_jobs WHERE job_id = 'job-1'"
        )).one()
    assert tuple(row) == ("running", "server")
    assert db_writer.claim_job("job-1", "worker") is False
