"""POST /analyze 的參數轉發與例外 → HTTP 狀態碼映射。

`analyze_symbol` 找不到 K 棒時丟 `ValueError`，端點把它映射成 **404**——
語意是「這個 symbol 沒有資料」，不是「請求壞掉」。這個映射沒有測試守著時，
很容易在重構 error handling 時被順手改成 400 或 500，而前端是靠 404 分辨
「查無此股」與「服務出錯」的。
"""
from __future__ import annotations

import pytest

import http_server
from backtest.modular.analysis import DEFAULT_FETCH_LIMIT

ENDPOINT = "/analyze"


@pytest.fixture
def analyze_calls(monkeypatch) -> list[tuple]:
    """把 analyze_symbol 換成記錄器。端點以 module global 解析，setattr 在 http_server 上即可。"""
    calls: list[tuple] = []

    def fake_analyze_symbol(symbol, timeframe, limit):
        calls.append((symbol, timeframe, limit))
        return {"symbol": symbol, "support": [], "resistance": []}

    monkeypatch.setattr(http_server, "analyze_symbol", fake_analyze_symbol)
    return calls


def test_params_reach_analyze_symbol(client, analyze_calls):
    """三個參數都要照送。timeframe 與 limit 刻意取非預設值，
    否則「有轉發」與「沒轉發但下游用自己的預設」看起來一模一樣。"""
    response = client.post(ENDPOINT, json={"symbol": "2330", "timeframe": "5m", "limit": 77})

    assert response.status_code == 200
    assert analyze_calls == [("2330", "5m", 77)]


def test_omitted_params_fall_back_to_defaults(client, analyze_calls):
    """只給 symbol 時要落在 AnalyzeRequest 的預設值上（1d / DEFAULT_FETCH_LIMIT）。"""
    response = client.post(ENDPOINT, json={"symbol": "2330"})

    assert response.status_code == 200
    assert analyze_calls == [("2330", "1d", DEFAULT_FETCH_LIMIT)]


def test_value_error_maps_to_404(client, monkeypatch):
    """查無 K 棒 → 404，且 detail 保留下游訊息（前端會顯示它）。"""

    def raise_value_error(symbol, timeframe, limit):
        raise ValueError("no candles found for symbol=9999 timeframe=1d")

    monkeypatch.setattr(http_server, "analyze_symbol", raise_value_error)

    response = client.post(ENDPOINT, json={"symbol": "9999"})

    assert response.status_code == 404
    assert response.json()["detail"] == "no candles found for symbol=9999 timeframe=1d"


def test_missing_symbol_rejected_by_pydantic(client, analyze_calls):
    """symbol 是必填，缺了要在進到端點主體前就被擋下（FastAPI 的 422）。"""
    response = client.post(ENDPOINT, json={"timeframe": "1d"})

    assert response.status_code == 422
    assert analyze_calls == []
