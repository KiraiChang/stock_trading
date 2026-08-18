"""POST /sr-scoring/train 的六個參數轉發與例外映射。

**`ValueError` 映射到 400 而不是 404**：這支端點的 `ValueError` 來自訓練資料不足或
參數組合不合法（例如 `split_method` 不認得），那是請求的問題，不是「查無資料」。
與 `/analyze`、`/sr-zones` 的 404 是刻意不同的——三支端點都靠 `ValueError` 表達失敗，
但意義不一樣，混用會讓前端無從分辨。
"""
from __future__ import annotations

import pytest

import http_server

ENDPOINT = "/sr-scoring/train"

# 六個參數都取非預設值（預設為 None / "1d" / 1500 / "gradient_boosting" / "time" / "sigmoid"），
# 否則「有轉發」與「沒轉發但下游用自己的預設」看起來一模一樣。
NON_DEFAULT = {
    "symbols": ["2330", "0050"],
    "timeframe": "5m",
    "limit": 900,
    "model_type": "random_forest",
    "split_method": "random",
    "calibration_method": "isotonic",
}


@pytest.fixture
def train_calls(monkeypatch) -> list[dict]:
    calls: list[dict] = []

    def fake_run_training(**kwargs):
        calls.append(kwargs)
        return {"version": "v-test", "metrics": {}}

    monkeypatch.setattr(http_server, "run_training", fake_run_training)
    return calls


def test_all_params_reach_run_training(client, train_calls):
    response = client.post(ENDPOINT, json=NON_DEFAULT)

    assert response.status_code == 200
    assert train_calls == [NON_DEFAULT]


def test_omitted_params_fall_back_to_defaults(client, train_calls):
    """空 body 也是合法請求（全部欄位都有預設），要落在 TrainRequest 的預設值上。"""
    response = client.post(ENDPOINT, json={})

    assert response.status_code == 200
    assert train_calls == [
        {
            "symbols": None,
            "timeframe": "1d",
            "limit": 1500,
            "model_type": "gradient_boosting",
            "split_method": "time",
            "calibration_method": "sigmoid",
        }
    ]


def test_calibration_method_none_is_forwarded_as_none(client, train_calls):
    """`calibration_method: null` 的語意是「明確不要校準」，不能被當成沒給而套回 sigmoid。"""
    response = client.post(ENDPOINT, json={"calibration_method": None})

    assert response.status_code == 200
    assert train_calls[0]["calibration_method"] is None


def test_value_error_maps_to_400(client, monkeypatch):
    def raise_value_error(**kwargs):
        raise ValueError("not enough training samples")

    monkeypatch.setattr(http_server, "run_training", raise_value_error)

    response = client.post(ENDPOINT, json={})

    assert response.status_code == 400
    assert response.json()["detail"] == "not enough training samples"
