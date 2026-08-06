"""POST /sr-scoring/evaluate 的請求驗證與例外 → HTTP 狀態碼映射。"""
from __future__ import annotations

import pytest

import http_server

ENDPOINT = "/sr-scoring/evaluate"


def test_empty_symbols_rejected(client, runners, evaluate_payload):
    response = client.post(ENDPOINT, json=evaluate_payload(symbols=[]))

    assert response.status_code == 400
    assert response.json()["detail"] == "symbols is required"
    assert runners.evaluation == []


def test_blank_symbols_rejected(client, runners, evaluate_payload):
    """全是空白的 symbols 會在 strip 後變成空清單，等同沒給。"""
    response = client.post(ENDPOINT, json=evaluate_payload(symbols=["  ", ""]))

    assert response.status_code == 400
    assert response.json()["detail"] == "symbols is required"
    assert runners.evaluation == []


def test_non_positive_limit_rejected(client, runners, evaluate_payload):
    response = client.post(ENDPOINT, json=evaluate_payload(limit=0))

    assert response.status_code == 400
    assert response.json()["detail"] == "limit must be > 0"
    assert runners.evaluation == []


def test_replay_mode_requires_positive_replay_max_rows(client, runners, evaluate_payload):
    response = client.post(
        ENDPOINT, json=evaluate_payload(decision_replay=True, replay_max_rows=0)
    )

    assert response.status_code == 400
    assert response.json()["detail"] == "replay_max_rows must be > 0 when decision_replay is enabled"
    assert runners.replay == []


def test_non_replay_mode_accepts_zero_replay_max_rows(client, runners, evaluate_payload):
    """非 replay 模式送 replay_max_rows=0 是 Go 端的正常語意（該欄位在這個模式沒有意義），
    不該回 400。這條刻意鎖住，免得日後有人把驗證條件放寬成無條件檢查。"""
    response = client.post(
        ENDPOINT, json=evaluate_payload(decision_replay=False, replay_max_rows=0)
    )

    assert response.status_code == 200
    assert len(runners.evaluation) == 1


@pytest.mark.parametrize("decision_replay", [False, True])
def test_value_error_maps_to_400(client, monkeypatch, evaluate_payload, decision_replay):
    """兩條分支共用同一個 try，一起鎖——只測一邊會讓另一邊的映射悄悄改掉。"""

    def boom(**kwargs):
        raise ValueError("沒有任何可用的資料來源（請指定 symbols 或 csv）")

    monkeypatch.setattr(http_server, "run_evaluation", boom)
    monkeypatch.setattr(http_server, "run_decision_replay", boom)

    response = client.post(ENDPOINT, json=evaluate_payload(decision_replay=decision_replay))

    assert response.status_code == 400
    assert response.json()["detail"] == "沒有任何可用的資料來源（請指定 symbols 或 csv）"


@pytest.mark.parametrize("decision_replay", [False, True])
def test_runtime_error_maps_to_503(client, monkeypatch, evaluate_payload, decision_replay):
    """RuntimeError 代表模型未就緒之類的暫時性狀態，對齊 /sr-zones 用 503 而不是 500。"""

    def boom(**kwargs):
        raise RuntimeError("model not trained")

    monkeypatch.setattr(http_server, "run_evaluation", boom)
    monkeypatch.setattr(http_server, "run_decision_replay", boom)

    response = client.post(ENDPOINT, json=evaluate_payload(decision_replay=decision_replay))

    assert response.status_code == 503
    assert response.json()["detail"] == "model not trained"


def test_failed_run_does_not_write_db(client, monkeypatch, runners, evaluate_payload):
    """跑失敗時不該留下 regression result——寫 DB 在 try 之後，這條鎖住這個順序。"""

    def boom(**kwargs):
        raise ValueError("boom")

    monkeypatch.setattr(http_server, "run_evaluation", boom)

    response = client.post(ENDPOINT, json=evaluate_payload(write_db=True))

    assert response.status_code == 400
    assert runners.write_db == []
