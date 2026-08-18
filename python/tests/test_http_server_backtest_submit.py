"""POST /backtest 的 symbols 解析與背景排程接線。

`symbols` 用 `field_validator` 同時接受 **JSON string 與 list**——Go 端把它塞成
字串送過來，人手打的請求則習慣送 list。少了測試，把 validator 拿掉只會在 Go 那條
路徑上炸掉，而所有既有測試都還是綠的。

**關於「background task 不阻塞」能驗到什麼**：TestClient 會在回應產生**之後**、
`client.post()` 回來**之前**執行 background task，所以這裡無法用時序證明不阻塞。
能鎖住的是「工作有被排進背景、且回應內容不含執行結果」——真正的不阻塞來自
`BackgroundTasks.add_task` 而不是 `await`，那是結構上的事實。
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
def scheduled(monkeypatch) -> list:
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
