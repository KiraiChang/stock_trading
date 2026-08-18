"""GET /sr-scoring/model-status——模型不存在時**刻意回 200 而不是 503**。

這是全檔最容易被「順手修好」破壞的設計：`/sr-zones` 在模型不存在時回 503，
所以會有人覺得這支也該一致。**不能一致。** 這支端點存在的理由就是讓前端在呼叫
`/sr-zones` 之前先問「模型準備好了沒」——如果它自己也回 503，前端就得靠錯誤處理
去判斷一個正常的狀態，等於沒有這支端點。

狀態放在 `exists` 欄位，HTTP 層永遠是 200。
"""
from __future__ import annotations

from types import SimpleNamespace

import pytest

import config
import http_server

ENDPOINT = "/sr-scoring/model-status"


@pytest.fixture
def fake_bundle() -> SimpleNamespace:
    return SimpleNamespace(
        version="v-test",
        trained_at="2026-08-18T00:00:00Z",
        split_method="time",
        metrics={"hold": {"auc": 0.71}, "break": {"auc": 0.68}},
        feature_names=["atr", "volume_ratio"],
        config_hash="abc123def456",
        training_config={"model_type": "gradient_boosting"},
    )


def test_missing_model_returns_200_with_exists_false(client, monkeypatch):
    """**這是本檔的重點**：`get_model()` 丟 RuntimeError 時是 200 + exists:false。

    斷言 `status_code != 503` 是刻意寫出來的——把它改成 503 才是這條測試要抓的迴歸。
    """

    def raise_runtime_error(*args, **kwargs):
        raise RuntimeError("model file not found")

    monkeypatch.setattr(http_server, "get_model", raise_runtime_error)
    monkeypatch.setattr(config, "SR_SCORING_MODEL_PATH", "/models/sr_scoring.joblib")

    response = client.get(ENDPOINT)

    assert response.status_code == 200
    assert response.status_code != 503
    body = response.json()
    assert body["exists"] is False
    # 模型不存在時，除了路徑之外每個欄位都要是 None——不要用空字串或 0 假裝有值，
    # 前端是靠 exists 分支，但拿到半真半假的欄位會讓除錯變得很難。
    assert body["model_path"] == "/models/sr_scoring.joblib"
    for field in (
        "version", "trained_at", "split_method", "metrics",
        "feature_names", "config_hash", "training_config",
    ):
        assert body[field] is None, f"{field} 應為 None"


def test_existing_model_returns_bundle_fields(client, monkeypatch, fake_bundle):
    monkeypatch.setattr(http_server, "get_model", lambda *a, **k: fake_bundle)
    monkeypatch.setattr(config, "SR_SCORING_MODEL_PATH", "/models/sr_scoring.joblib")

    response = client.get(ENDPOINT)

    assert response.status_code == 200
    body = response.json()
    assert body["exists"] is True
    assert body["version"] == "v-test"
    assert body["trained_at"] == "2026-08-18T00:00:00Z"
    assert body["split_method"] == "time"
    assert body["metrics"] == {"hold": {"auc": 0.71}, "break": {"auc": 0.68}}
    assert body["feature_names"] == ["atr", "volume_ratio"]
    assert body["model_path"] == "/models/sr_scoring.joblib"


def test_config_hash_is_exposed(client, monkeypatch, fake_bundle):
    """`config_hash` 是「這筆分析出自哪組訓練設定」的追蹤鍵，
    與 /sr-zones 回傳的 model_config_hash 是同一個值。漏掉它，重訓後就分不出舊分析。
    """
    monkeypatch.setattr(http_server, "get_model", lambda *a, **k: fake_bundle)

    response = client.get(ENDPOINT)

    assert response.status_code == 200
    assert response.json()["config_hash"] == "abc123def456"
    assert response.json()["training_config"] == {"model_type": "gradient_boosting"}
