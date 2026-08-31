"""POST /sr-zones 的參數轉發與兩種例外映射。

這支端點有**兩條不同的錯誤路徑**，混在一起就沒有意義了：

* `ValueError` → **404**：這個 symbol 沒有 K 棒
* `RuntimeError` → **503**：模型還沒訓練過（規則式分數算得出來，機率算不出來）

503 那條是前端決定「要不要引導使用者去訓練模型」的依據，被改成 404 或 500 都會讓
那個引導失效，而且不會有任何既有測試變紅。
"""
from __future__ import annotations

import pytest

import http_server
from backtest.modular.sr_scoring.scoring import DEFAULT_FETCH_LIMIT

ENDPOINT = "/sr-zones"


@pytest.fixture
def score_calls(monkeypatch) -> list[dict]:
    calls: list[dict] = []

    def fake_score_symbol(symbol, timeframe, limit, previous_event_states=None, previous_analyzed_at=None):
        calls.append(
            {
                "symbol": symbol,
                "timeframe": timeframe,
                "limit": limit,
                "previous_event_states": previous_event_states,
                "previous_analyzed_at": previous_analyzed_at,
            }
        )
        return {"zones": [], "model_version": "v1"}

    monkeypatch.setattr(http_server, "score_symbol", fake_score_symbol)
    return calls


def test_params_reach_score_symbol(client, score_calls):
    previous = [{"zone_id": "z1", "state": "TESTING"}]

    response = client.post(
        ENDPOINT,
        json={
            "symbol": "2330",
            "timeframe": "5m",
            "limit": 77,
            "previous_event_states": previous,
            "previous_analyzed_at": "2026-08-18T16:00:00Z",
        },
    )

    assert response.status_code == 200
    assert score_calls == [
        {
            "symbol": "2330",
            "timeframe": "5m",
            "limit": 77,
            "previous_event_states": previous,
            "previous_analyzed_at": "2026-08-18T16:00:00Z",
        }
    ]


def test_omitted_params_fall_back_to_defaults(client, score_calls):
    """`previous_event_states` 省略時要是**空 list 而不是 None**。

    下游 `score_symbol` 兩者都收，但空 list 的語意是「這次沒有先前狀態」，
    None 的語意是「呼叫端沒表態」。端點的 `default_factory=list` 是刻意的，
    這條測試鎖住它——改成 `Optional[...] = None` 會讓事件鏈的 diff 邏輯
    收到一個它沒預期的型別。

    `previous_analyzed_at` 相反，**省略就是 None**：它的語意正是「呼叫端沒表態」，
    下游據此退回舊的老化行為（原記於 issue.md I-077，已收斂）。
    """
    response = client.post(ENDPOINT, json={"symbol": "2330"})

    assert response.status_code == 200
    assert score_calls == [
        {
            "symbol": "2330",
            "timeframe": "1d",
            "limit": DEFAULT_FETCH_LIMIT,
            "previous_event_states": [],
            "previous_analyzed_at": None,
        }
    ]


def test_value_error_maps_to_404(client, monkeypatch):
    def raise_value_error(symbol, timeframe, limit, previous_event_states=None, previous_analyzed_at=None):
        raise ValueError("no candles found for symbol=9999 timeframe=1d")

    monkeypatch.setattr(http_server, "score_symbol", raise_value_error)

    response = client.post(ENDPOINT, json={"symbol": "9999"})

    assert response.status_code == 404
    assert response.json()["detail"] == "no candles found for symbol=9999 timeframe=1d"


def test_runtime_error_maps_to_503(client, monkeypatch):
    """模型不存在是「服務暫時不可用」，不是「查無資料」——必須是 503。"""

    def raise_runtime_error(symbol, timeframe, limit, previous_event_states=None, previous_analyzed_at=None):
        raise RuntimeError("model not trained yet")

    monkeypatch.setattr(http_server, "score_symbol", raise_runtime_error)

    response = client.post(ENDPOINT, json={"symbol": "2330"})

    assert response.status_code == 503
    assert response.json()["detail"] == "model not trained yet"
