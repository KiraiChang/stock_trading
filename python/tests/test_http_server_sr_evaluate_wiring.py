"""POST /sr-scoring/evaluate 的參數轉發。

這一組是 T-037 C 的重點。2026-08-05 發生過的 bug 是 decision replay 分支漏傳
`builder_config`——請求收下了四個 ATR 參數、回應也正常，但參數完全沒生效。這種靜默失效
不會讓任何既有測試變紅，只能靠「斷言下游真的收到了什麼」來鎖。
"""
from __future__ import annotations

import pytest

import config
import http_server
from backtest.modular.sr_scoring.evaluation import DEFAULT_PIPELINE_VERSION
from backtest.modular.sr_scoring.zone_builder import ATRZoneBuilderConfig

ENDPOINT = "/sr-scoring/evaluate"

# 四個 ATR 參數都刻意取非預設值（預設為 1.5 / 2.0 / 60 / 14），
# 否則「有轉發」與「沒轉發但下游自己用預設」看起來會一模一樣。
ATR_PARAMS = {
    "atr_width_multiplier": 1.25,
    "max_merge_width_multiple": 2.5,
    "atr_lookback": 45,
    "atr_period": 21,
}


@pytest.mark.parametrize("decision_replay", [False, True])
def test_atr_params_reach_builder_config(client, runners, evaluate_payload, decision_replay):
    """兩條分支都必須收到帶著四個 ATR 參數的 builder_config。

    這就是漏傳 bug 的直接回歸測試。builder_config 目前組在分支外正是為了避免再漏一次，
    但那只是結構上的自律，這條測試才是真正鎖住它的東西。
    """
    response = client.post(
        ENDPOINT, json=evaluate_payload(decision_replay=decision_replay, **ATR_PARAMS)
    )

    assert response.status_code == 200
    calls = runners.replay if decision_replay else runners.evaluation
    atr = calls[0]["builder_config"].atr
    assert atr.atr_width_multiplier == ATR_PARAMS["atr_width_multiplier"]
    assert atr.max_merge_width_multiple == ATR_PARAMS["max_merge_width_multiple"]
    assert atr.lookback == ATR_PARAMS["atr_lookback"]
    assert atr.atr_period == ATR_PARAMS["atr_period"]


@pytest.mark.parametrize("decision_replay", [False, True])
def test_omitted_atr_params_fall_back_to_defaults(
    client, runners, evaluate_payload, decision_replay
):
    """四個鍵完全不在 body 裡時要落在 ATRZoneBuilderConfig 的預設值上。

    這才是真實流量最常見的情況：前端的語意是「留白＝不送鍵」，Go 端那四個欄位又是
    `omitempty`，所以絕大多數請求根本沒有這幾個鍵。上面那條測「有送要照送」，這條測
    「沒送不要送出奇怪的東西」——兩條合起來才涵蓋整條前端鏈路。
    """
    response = client.post(ENDPOINT, json=evaluate_payload(decision_replay=decision_replay))

    assert response.status_code == 200
    calls = runners.replay if decision_replay else runners.evaluation
    assert calls[0]["builder_config"].atr == ATRZoneBuilderConfig()


@pytest.mark.parametrize("decision_replay", [False, True])
def test_dataset_config_fan_out(client, runners, evaluate_payload, decision_replay):
    """request 的 forward_bars / threshold_pct 各要展開成 support 與 resistance 兩個欄位。
    一對多的展開最容易只填一邊，兩邊都斷言。"""
    response = client.post(
        ENDPOINT,
        json=evaluate_payload(
            decision_replay=decision_replay,
            min_history_bars=120,
            rebuild_every_bars=3,
            forward_bars=7,
            threshold_pct=0.05,
        ),
    )

    assert response.status_code == 200
    calls = runners.replay if decision_replay else runners.evaluation
    dataset_config = calls[0]["dataset_config"]
    assert dataset_config.min_history_bars == 120
    assert dataset_config.rebuild_every_bars == 3
    assert dataset_config.forward_bars_support == 7
    assert dataset_config.forward_bars_resistance == 7
    assert dataset_config.threshold_pct_support == 0.05
    assert dataset_config.threshold_pct_resistance == 0.05


def test_symbols_are_trimmed_and_blanks_dropped(client, runners, evaluate_payload):
    response = client.post(ENDPOINT, json=evaluate_payload(symbols=["  2330 ", "", "2317", "  "]))

    assert response.status_code == 200
    assert runners.evaluation[0]["symbols"] == ["2330", "2317"]


def test_evaluation_mode_does_not_call_replay(client, runners, evaluate_payload):
    response = client.post(ENDPOINT, json=evaluate_payload(decision_replay=False))

    assert response.status_code == 200
    assert len(runners.evaluation) == 1
    assert runners.replay == []


def test_replay_mode_does_not_call_evaluation(client, runners, evaluate_payload):
    response = client.post(ENDPOINT, json=evaluate_payload(decision_replay=True))

    assert response.status_code == 200
    assert len(runners.replay) == 1
    assert runners.evaluation == []


def test_default_pipeline_version_differs_by_mode(client, runners, evaluate_payload):
    """兩種模式的預設 pipeline_version 不同，是刻意的（要能區分新舊 report）。"""
    assert client.post(ENDPOINT, json=evaluate_payload()).status_code == 200
    assert client.post(ENDPOINT, json=evaluate_payload(decision_replay=True)).status_code == 200

    assert runners.evaluation[0]["pipeline_version"] == DEFAULT_PIPELINE_VERSION
    assert runners.replay[0]["pipeline_version"] == "sr_zone_decision_replay_p1"


@pytest.mark.parametrize("decision_replay", [False, True])
def test_explicit_pipeline_version_overrides_default(
    client, runners, evaluate_payload, decision_replay
):
    response = client.post(
        ENDPOINT,
        json=evaluate_payload(decision_replay=decision_replay, pipeline_version="custom_v9"),
    )

    assert response.status_code == 200
    calls = runners.replay if decision_replay else runners.evaluation
    assert calls[0]["pipeline_version"] == "custom_v9"


def test_model_path_defaults_to_config(client, runners, evaluate_payload, monkeypatch):
    monkeypatch.setattr(config, "SR_SCORING_MODEL_PATH", "/tmp/from-config.joblib")

    response = client.post(ENDPOINT, json=evaluate_payload())

    assert response.status_code == 200
    assert runners.evaluation[0]["model_path"] == "/tmp/from-config.joblib"


def test_explicit_model_path_overrides_config(client, runners, evaluate_payload, monkeypatch):
    monkeypatch.setattr(config, "SR_SCORING_MODEL_PATH", "/tmp/from-config.joblib")

    response = client.post(ENDPOINT, json=evaluate_payload(model_path="/tmp/explicit.joblib"))

    assert response.status_code == 200
    assert runners.evaluation[0]["model_path"] == "/tmp/explicit.joblib"


def test_replay_only_params_reach_decision_replay(client, runners, evaluate_payload):
    """chip / governance 快照與 replay_max_rows 只有 replay 分支收得到，是 Go 端注入的
    as-of 資料，漏傳會讓 replay 靜默退化成沒有籌碼與治理資訊的版本。"""
    chip_scores = {"2330": [{"trade_date": "2026-01-02", "total_score": 60.0}]}
    governance = {"2330": [{"as_of": "2026-01-02T00:00:00", "health_state": "HEALTHY"}]}

    response = client.post(
        ENDPOINT,
        json=evaluate_payload(
            decision_replay=True,
            replay_max_rows=42,
            chip_scores_by_symbol=chip_scores,
            model_governance_by_symbol=governance,
        ),
    )

    assert response.status_code == 200
    call = runners.replay[0]
    assert call["replay_max_rows"] == 42
    assert call["chip_scores_by_symbol"] == chip_scores
    assert call["model_governance_by_symbol"] == governance


def test_run_id_and_timeframe_and_limit_are_forwarded(client, runners, evaluate_payload):
    response = client.post(
        ENDPOINT, json=evaluate_payload(timeframe="1h", limit=300, run_id="run-abc")
    )

    assert response.status_code == 200
    call = runners.evaluation[0]
    assert call["timeframe"] == "1h"
    assert call["limit"] == 300
    assert call["run_id"] == "run-abc"


def test_write_db_persists_report_with_passed(client, runners, evaluate_payload):
    response = client.post(ENDPOINT, json=evaluate_payload(write_db=True, passed=False))

    assert response.status_code == 200
    assert len(runners.write_db) == 1
    report, passed = runners.write_db[0]
    assert report == response.json()
    assert passed is False


def test_write_db_disabled_by_default(client, runners, evaluate_payload):
    response = client.post(ENDPOINT, json=evaluate_payload())

    assert response.status_code == 200
    assert runners.write_db == []


def test_response_body_is_the_report_itself(client, runners, evaluate_payload):
    """回應不包裝、不改名：Go 端 RunSREvaluation 是原樣穿透成 map[string]any 的。"""
    response = client.post(ENDPOINT, json=evaluate_payload())

    assert response.status_code == 200
    assert response.json()["run_id"] == "test-run-id"
    assert response.json()["schema_version"] == "sr_zone_evaluation_p0"
