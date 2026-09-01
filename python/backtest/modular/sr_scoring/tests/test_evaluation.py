from __future__ import annotations

import json
import sys
import types

import numpy as np
import pandas as pd
import pytest
from sqlalchemy import create_engine, text

from ..dataset import DatasetConfig
from ..model import ModelBundle
from .. import evaluation as evaluation_module
from ..evaluation import (
    _daily_confirmation_summary,
    _decision_fields_from_summary,
    _excursion_window,
    _decision_replay_governance_evaluation,
    _load_symbol_rows_json,
    _rows_for_symbol,
    run_builder_sweep,
    run_decision_replay,
    run_evaluation,
    write_evaluation_result,
)
from ..zone_builder import ATRZoneBuilderConfig, ZoneBuilderConfig
from .conftest import bullish_trend_df


def _write_csv(path, df) -> None:
    data = df.reset_index().rename(columns={"datetime": "timestamp"})
    if "timestamp" not in data.columns:
        data = data.rename(columns={data.columns[0]: "timestamp"})
    data["timestamp"] = data["timestamp"].astype("int64") // 1_000_000_000
    data.to_csv(path, index=False)


def _write_json(path, data) -> None:
    path.write_text(json.dumps(data), encoding="utf-8")


def _candle_rows(df) -> list[dict]:
    data = df.reset_index().rename(columns={"datetime": "timestamp"})
    if "timestamp" not in data.columns:
        data = data.rename(columns={data.columns[0]: "timestamp"})
    data["timestamp"] = data["timestamp"].astype("int64") // 1_000_000_000
    return data[["open", "high", "low", "close", "volume", "timestamp"]].to_dict("records")


def _dataset_config() -> DatasetConfig:
    return DatasetConfig(
        min_history_bars=60,
        rebuild_every_bars=5,
        forward_bars_support=3,
        forward_bars_resistance=3,
        threshold_pct_support=0.01,
        threshold_pct_resistance=0.01,
        zone_lookback_bars=60,
    )


def _missing_chip_summary() -> dict:
    return {
        "missing": True,
        "score": None,
        "raw_score": None,
        "effective_score": None,
        "coverage": 0.0,
        "confidence": 0.0,
        "confidence_level": "NONE",
        "signal": None,
        "trade_date": None,
        "institutional_score": None,
        "margin_score": None,
        "broker_score": None,
        "concentration_score": None,
    }


class _FixedProbabilityModel:
    def __init__(self, probability: float) -> None:
        self.probability = probability

    def predict_proba(self, X):
        return np.array([[1.0 - self.probability, self.probability] for _ in range(len(X))])


def test_load_symbol_rows_json_accepts_symbol_object_and_default_list(tmp_path):
    object_path = tmp_path / "chip_object.json"
    list_path = tmp_path / "chip_list.json"
    _write_json(object_path, {"2330": [{"trade_date": "2024-01-01"}]})
    _write_json(list_path, [{"trade_date": "2024-01-01"}])

    assert _load_symbol_rows_json(str(object_path)) == {"2330": [{"trade_date": "2024-01-01"}]}
    assert _load_symbol_rows_json(str(list_path)) == {"__default__": [{"trade_date": "2024-01-01"}]}


def test_load_symbol_rows_json_rejects_invalid_shape(tmp_path):
    path = tmp_path / "invalid.json"
    _write_json(path, {"2330": {"trade_date": "2024-01-01"}})

    with pytest.raises(ValueError, match="symbol value"):
        _load_symbol_rows_json(str(path))


def test_decision_fields_from_summary_carries_lifecycle_phase():
    """`lifecycle_phase` 必須從 `decision_derived_view.semantic_pipeline` 帶進 replay row。

    **為什麼要釘住它**：decision replay 是唯一能對真實資料比較「事件演進階段有沒有被改掉」
    的工具。這個欄位沒帶出來時，A/B 兩版的分佈會長得一模一樣，而那**分不出**是那條路徑
    沒被觸發，還是改動真的沒有影響——2026-09-01 跑 issue.md I-074 的 replay 就踩到這件事。
    """
    summary = {
        "market_bias": "BULLISH_BIAS",
        "decision_derived_view": {"semantic_pipeline": {"lifecycle_phase": "CONTINUATION"}},
    }
    assert _decision_fields_from_summary(summary)["lifecycle_phase"] == "CONTINUATION"


def test_decision_fields_from_summary_lifecycle_phase_absent_is_none():
    """derived view 或 semantic pipeline 缺席時回 None，不得炸掉整份 replay。"""
    assert _decision_fields_from_summary({})["lifecycle_phase"] is None
    assert _decision_fields_from_summary({"decision_derived_view": {}})["lifecycle_phase"] is None


def test_decision_replay_governance_evaluation_blocks_unavailable_model():
    result = _decision_replay_governance_evaluation(
        {
            "rows": 10,
            "rows_with_decision_fields": 0,
            "rows_with_model_governance": 0,
            "rows_with_event_lifecycle": 0,
            "decision_error_counts": {},
            "zone_score_error_counts": {},
            "by_final_entry_state": {},
        },
        model_available=False,
    )

    assert result["health_state"] == "UNRELIABLE"
    assert result["passed"] is False
    assert result["confidence_gate"]["allow_entry"] is False
    assert result["confidence_gate"]["max_entry_state"] == "WAIT_CONFIRMATION"
    assert "MODEL_UNAVAILABLE" in result["blocking_flags"]


def test_decision_replay_governance_evaluation_degrades_partial_context():
    result = _decision_replay_governance_evaluation(
        {
            "rows": 40,
            "rows_with_decision_fields": 40,
            "rows_with_model_governance": 0,
            "rows_with_event_lifecycle": 40,
            "decision_error_counts": {},
            "zone_score_error_counts": {},
            "by_final_entry_state": {
                "PROBE_ALLOWED": {
                    "rows": 8,
                    "rows_with_forward_return": 8,
                    "average_forward_return": 0.01,
                    "positive_forward_return_rate": 0.625,
                    "negative_forward_return_rate": 0.25,
                },
            },
        },
        model_available=True,
    )

    assert result["health_state"] == "DEGRADED"
    assert result["passed"] is True
    assert result["strict_passed"] is False
    assert result["confidence_gate"]["allow_entry"] is True
    assert result["confidence_gate"]["max_entry_state"] == "SMALL_ENTRY"
    assert "MODEL_GOVERNANCE_CONTEXT_MISSING" in result["warning_flags"]


def test_decision_replay_governance_evaluation_passes_healthy_replay():
    result = _decision_replay_governance_evaluation(
        {
            "rows": 40,
            "rows_with_decision_fields": 40,
            "rows_with_model_governance": 40,
            "rows_with_event_lifecycle": 40,
            "decision_error_counts": {},
            "zone_score_error_counts": {},
            "by_final_entry_state": {
                "ENTRY_ALLOWED": {
                    "rows": 10,
                    "rows_with_forward_return": 10,
                    "average_forward_return": 0.02,
                    "positive_forward_return_rate": 0.7,
                    "negative_forward_return_rate": 0.2,
                },
                "PROBE_ALLOWED": {
                    "rows": 6,
                    "rows_with_forward_return": 6,
                    "average_forward_return": 0.01,
                    "positive_forward_return_rate": 0.5,
                    "negative_forward_return_rate": 0.3333,
                },
            },
        },
        model_available=True,
    )

    assert result["health_state"] == "HEALTHY"
    assert result["passed"] is True
    assert result["strict_passed"] is True
    assert result["confidence_gate"]["allow_entry"] is True
    assert result["confidence_gate"]["max_entry_state"] == "ENTRY_ALLOWED"
    assert result["confidence_gate"]["reason_codes"] == []


def test_daily_confirmation_summary_reports_zone_confirmation_rates():
    rows = [
        {
            "daily_confirmation_outcome": {
                "available": True,
                "state": "PROBE_ALLOWED",
                "primary_role": "SUPPORT",
                "next_zone_result": "SUPPORT_HELD",
                "two_bar_result": "SUPPORT_CONFIRMED",
                "next_close_return": 0.01,
                "two_bar_close_return": 0.02,
                "volume_context": "VOLUME_CONFIRMED",
                "event_sequence": "INTRADAY_RECLAIM",
                "market_event_types": "INTRADAY_RECLAIM",
                "event_market_state": "RECLAIM_ATTEMPT",
                "rr_gate": "RR_QUALIFIED",
                "rr_gate_reason_code": "RR_QUALIFIED",
                "rr_bucket": "RR_2_0_TO_3_0",
            }
        },
        {
            "daily_confirmation_outcome": {
                "available": True,
                "state": "PROBE_ALLOWED",
                "primary_role": "SUPPORT",
                "next_zone_result": "SUPPORT_BROKEN",
                "two_bar_result": "SUPPORT_FAILED",
                "next_close_return": -0.01,
                "two_bar_close_return": -0.02,
                "volume_context": "NO_VOLUME_CONFIRMATION",
                "event_sequence": "NO_EVENT",
                "market_event_types": "NO_EVENT",
                "event_market_state": "NORMAL",
                "rr_gate": "RR_BLOCKED",
                "rr_gate_reason_code": "RR_INSUFFICIENT",
                "rr_bucket": "RR_1_0_TO_1_5",
            }
        },
        {
            "daily_confirmation_outcome": {
                "available": True,
                "state": "WAIT_CONFIRMATION",
                "primary_role": "RESISTANCE",
                "next_zone_result": "RESISTANCE_BROKEN",
                "two_bar_result": "RESISTANCE_BREAKOUT_CONTINUATION",
                "next_close_return": 0.015,
                "two_bar_close_return": 0.03,
                "volume_context": "EXTREME_VOLUME",
                "event_sequence": "BREAKOUT_CANDIDATE",
                "market_event_types": "EXTREME_VOLUME",
                "event_market_state": "BREAKOUT_ATTEMPT",
                "rr_gate": "RR_QUALIFIED",
                "rr_gate_reason_code": "RR_QUALIFIED",
                "rr_bucket": "RR_GTE_3",
            }
        },
    ]

    summary = _daily_confirmation_summary(rows)

    assert summary["rows"] == 3
    assert summary["support_next_hold_rate"] == pytest.approx(0.5)
    assert summary["support_two_bar_confirm_rate"] == pytest.approx(0.5)
    assert summary["resistance_next_breakout_rate"] == pytest.approx(1.0)
    assert summary["resistance_two_bar_breakout_continuation_rate"] == pytest.approx(1.0)
    assert summary["positive_two_bar_return_rate"] == pytest.approx(2 / 3)
    assert summary["failure_distribution"] == {
        "RESISTANCE_BREAKOUT_CONTINUED": 1,
        "SUPPORT_CONFIRMATION_FAILED": 1,
        "SUPPORT_CONFIRMATION_OK": 1,
    }
    assert summary["by_state"]["PROBE_ALLOWED"]["rows"] == 2
    assert summary["by_primary_role"]["SUPPORT"]["two_bar_result_counts"] == {
        "SUPPORT_CONFIRMED": 1,
        "SUPPORT_FAILED": 1,
    }
    assert summary["by_volume_context"]["EXTREME_VOLUME"]["rows"] == 1
    assert summary["by_event_sequence"]["INTRADAY_RECLAIM"]["rows"] == 1
    assert summary["by_market_event_types"]["EXTREME_VOLUME"]["rows"] == 1
    assert summary["by_event_market_state"]["RECLAIM_ATTEMPT"]["rows"] == 1
    assert summary["by_rr_gate"]["RR_QUALIFIED"]["rows"] == 2
    assert summary["by_rr_gate_reason_code"]["RR_INSUFFICIENT"]["rows"] == 1
    assert summary["by_rr_bucket"]["RR_GTE_3"]["rows"] == 1


def _excursion_df(lows: list[float], highs: list[float]) -> pd.DataFrame:
    """idx=0 是確認日，之後是窗口內的每一根。確認日本身的 high/low 不該被納入計算。"""
    n = len(lows)
    return pd.DataFrame(
        {
            "open": [100.0] * n,
            "high": highs,
            "low": lows,
            "close": [100.0] * n,
            "volume": [1000] * n,
        },
        index=pd.date_range("2026-01-01", periods=n, freq="D"),
    )


def test_excursion_window_support_measures_path_not_endpoint():
    # 確認日收 100；窗口內先跌到 94（逆行 6%）再拉到 108，終點卻回到 100——
    # 這正是終點報酬看不見、但停損會被掃到的情況。
    df = _excursion_df(
        lows=[999.0, 98.0, 94.0, 99.0],
        highs=[999.0, 101.0, 100.0, 108.0],
    )
    zone = {"role": "SUPPORT", "price_low": 95.0, "price_high": 105.0}

    result = _excursion_window(df, 0, 100.0, zone, 3, {"stop_distance_pct": 0.04})

    assert result["excursion_window_bars"] == 3
    assert result["max_adverse_excursion_pct"] == pytest.approx(-0.06)
    assert result["max_favorable_excursion_pct"] == pytest.approx(0.08)
    # 94 < price_low 95，發生在窗口第 2 根。
    assert result["failure_state"] == "FAILED"
    assert result["bars_to_failure"] == 2
    # 逆行 6% 對停損距離 4% → 1.5 倍，> 1 代表窗口內曾掃到停損。
    assert result["mae_to_stop_ratio"] == pytest.approx(1.5)


def test_excursion_window_resistance_flips_sign_for_short_bias():
    # RESISTANCE 視為偏空：價格上漲才算不利，符號與原始報酬相反。
    df = _excursion_df(
        lows=[999.0, 97.0, 96.0],
        highs=[999.0, 103.0, 105.0],
    )
    zone = {"role": "RESISTANCE", "price_low": 95.0, "price_high": 104.0}

    result = _excursion_window(df, 0, 100.0, zone, 2, None)

    assert result["max_adverse_excursion_pct"] == pytest.approx(-0.05)
    assert result["max_favorable_excursion_pct"] == pytest.approx(0.04)
    assert result["failure_state"] == "FAILED"
    assert result["bars_to_failure"] == 2
    assert result["mae_to_stop_ratio"] is None


def test_excursion_window_clamps_when_price_never_moves_against_position():
    # 全程走高的 SUPPORT：MAE 是 0（從未逆行），不是正數。
    df = _excursion_df(lows=[999.0, 101.0, 103.0], highs=[999.0, 104.0, 106.0])
    zone = {"role": "SUPPORT", "price_low": 95.0}

    result = _excursion_window(df, 0, 100.0, zone, 2, {"stop_distance_pct": 0.05})

    assert result["max_adverse_excursion_pct"] == 0.0
    assert result["max_favorable_excursion_pct"] == pytest.approx(0.06)
    assert result["failure_state"] == "SURVIVED_WINDOW"
    assert result["bars_to_failure"] is None
    assert result["mae_to_stop_ratio"] == 0.0


def test_excursion_window_at_zone_has_no_defined_direction():
    # AT_ZONE 的既有 label 本身沒有方向偏誤，硬指定一邊會做出無法解釋的數字。
    df = _excursion_df(lows=[999.0, 94.0, 96.0], highs=[999.0, 106.0, 104.0])
    zone = {"role": "AT_ZONE", "price_low": 95.0, "price_high": 105.0}

    result = _excursion_window(df, 0, 100.0, zone, 2, {"stop_distance_pct": 0.05})

    assert result["max_adverse_excursion_pct"] is None
    assert result["max_favorable_excursion_pct"] is None
    assert result["failure_state"] == "DIRECTION_UNDEFINED"
    assert result["mae_to_stop_ratio"] is None


def test_excursion_window_without_boundary_still_reports_excursion():
    # 缺 price_low 只讓「失效」算不出來，不該連帶讓 MAE 消失。
    df = _excursion_df(lows=[999.0, 94.0], highs=[999.0, 101.0])
    zone = {"role": "SUPPORT", "price_low": None}

    result = _excursion_window(df, 0, 100.0, zone, 1, None)

    assert result["max_adverse_excursion_pct"] == pytest.approx(-0.06)
    assert result["failure_state"] == "BOUNDARY_UNAVAILABLE"
    assert result["bars_to_failure"] is None


def test_excursion_window_treats_nan_boundary_as_unavailable():
    # NaN 邊界拿去比較會全部得到 False，不能讓「算不出來」被報成「窗口內沒失效」。
    df = _excursion_df(lows=[999.0, 94.0], highs=[999.0, 101.0])
    zone = {"role": "SUPPORT", "price_low": float("nan")}

    result = _excursion_window(df, 0, 100.0, zone, 1, None)

    assert result["failure_state"] == "BOUNDARY_UNAVAILABLE"
    assert result["max_adverse_excursion_pct"] == pytest.approx(-0.06)


def test_excursion_window_stops_at_end_of_dataframe():
    # 窗口要 5 根但只剩 2 根：以實際根數為準，不能讀出界。
    df = _excursion_df(lows=[999.0, 98.0, 97.0], highs=[999.0, 101.0, 102.0])
    zone = {"role": "SUPPORT", "price_low": 90.0}

    result = _excursion_window(df, 0, 100.0, zone, 5, None)

    assert result["excursion_window_bars"] == 2
    assert result["max_adverse_excursion_pct"] == pytest.approx(-0.03)


def test_daily_confirmation_summary_reports_excursion_section():
    rows = [
        {
            "daily_confirmation_outcome": {
                "available": True,
                "state": "PROBE_ALLOWED",
                "primary_role": "SUPPORT",
                "max_adverse_excursion_pct": -0.06,
                "max_favorable_excursion_pct": 0.08,
                "bars_to_failure": 2,
                "failure_state": "FAILED",
                "mae_to_stop_ratio": 1.5,
            }
        },
        {
            "daily_confirmation_outcome": {
                "available": True,
                "state": "PROBE_ALLOWED",
                "primary_role": "SUPPORT",
                "max_adverse_excursion_pct": -0.02,
                "max_favorable_excursion_pct": 0.03,
                "bars_to_failure": None,
                "failure_state": "SURVIVED_WINDOW",
                "mae_to_stop_ratio": 0.5,
            }
        },
        {
            # AT_ZONE：不計入 excursion 的分母，這是預期而非漏算。
            "daily_confirmation_outcome": {
                "available": True,
                "state": "WAIT_CONFIRMATION",
                "primary_role": "AT_ZONE",
                "max_adverse_excursion_pct": None,
                "max_favorable_excursion_pct": None,
                "bars_to_failure": None,
                "failure_state": "DIRECTION_UNDEFINED",
                "mae_to_stop_ratio": None,
            }
        },
    ]

    excursion = _daily_confirmation_summary(rows)["excursion"]

    assert excursion["rows"] == 2
    assert excursion["average_max_adverse_excursion_pct"] == pytest.approx(-0.04)
    assert excursion["max_adverse_excursion_distribution"]["count"] == 2
    assert excursion["max_adverse_excursion_distribution"]["min"] == pytest.approx(-0.06)
    assert excursion["stop_sweep_rate"] == pytest.approx(0.5)
    assert excursion["failure_state_counts"] == {"FAILED": 1, "SURVIVED_WINDOW": 1}
    assert excursion["bars_to_failure_counts"] == {"2": 1}
    assert excursion["average_bars_to_failure"] == pytest.approx(2.0)
    # 分層也要帶上同一段，否則只有全體看得到過程。
    assert _daily_confirmation_summary(rows)["by_state"]["PROBE_ALLOWED"]["excursion"]["rows"] == 2


def test_run_evaluation_returns_zone_outcome_report(tmp_path):
    df = bullish_trend_df(n=170)
    path = tmp_path / "2330.csv"
    _write_csv(path, df)

    report = run_evaluation(
        csv_sources=[f"{path}:2330"],
        dataset_config=_dataset_config(),
        run_id="sr_eval_test_001",
        pipeline_version="sr_zone_evaluation_test",
    )

    assert report["schema_version"] == "sr_zone_evaluation_p0"
    assert report["run_id"] == "sr_eval_test_001"
    assert report["pipeline_version"] == "sr_zone_evaluation_test"
    assert report["split_method"] == "walk_forward"
    assert report["sources"] == 1
    assert report["symbols"] == ["2330"]
    assert report["rows"] > 0
    assert report["dataset_summary"]["rows"] == report["rows"]
    assert report["zone_outcomes"]["rows"] == report["rows"]
    assert report["volatility_profiles"]["2330"]["bucket"] in {
        "LOW_VOLATILITY",
        "NORMAL_VOLATILITY",
        "HIGH_VOLATILITY",
        "UNKNOWN_VOLATILITY",
    }
    assert report["volatility_profiles"]["2330"]["touch_count"] == report["rows"]
    assert report["zone_outcomes"]["by_volatility_bucket"]
    assert sum(bucket["rows"] for bucket in report["zone_outcomes"]["by_volatility_bucket"].values()) == report["rows"]

    # 分層必須輸出與頂層同名的比率欄位，而且要有實際數值。
    # 分層原本只回 hold_rate / break_rate，前端讀的是這三個 key，於是永遠顯示 `—`；
    # 當時的測試只斷言 by_volatility_bucket 非空與 rows 加總，剛好完全避開了出錯的欄位。
    # 所以這裡斷言「值不是 None 且落在 [0,1]」，不是只斷言 key 存在。
    zone_outcomes = report["zone_outcomes"]
    for grouping in ("by_role", "by_method", "by_volatility_bucket"):
        assert zone_outcomes[grouping], grouping
        for name, group in zone_outcomes[grouping].items():
            assert group["break_positive_rate"] is not None, (grouping, name)
            assert 0.0 <= group["break_positive_rate"] <= 1.0, (grouping, name)
            assert group["hold_rate"] is not None, (grouping, name)
            # 依角色拆開的兩個比率：by_role 只會有一種角色，另一個必然是 None；
            # 其餘分層兩者都該有值。至少要有一個不是 None，否則就是又對不上欄位了。
            role_rates = (group["support_hold_rate"], group["resistance_rejection_rate"])
            assert any(rate is not None for rate in role_rates), (grouping, name)
            assert "break_rate" not in group, (grouping, name)

    # by_role 的分層比率要跟頂層對得起來：SUPPORT 組的 support_hold_rate 就是頂層的值。
    by_role = zone_outcomes["by_role"]
    if "SUPPORT" in by_role:
        assert by_role["SUPPORT"]["support_hold_rate"] == pytest.approx(zone_outcomes["support_hold_rate"])
        assert by_role["SUPPORT"]["resistance_rejection_rate"] is None
    if "RESISTANCE" in by_role:
        assert by_role["RESISTANCE"]["resistance_rejection_rate"] == pytest.approx(
            zone_outcomes["resistance_rejection_rate"]
        )
        assert by_role["RESISTANCE"]["support_hold_rate"] is None

    assert "ATRZoneBuilder" in report["builder_config"]
    assert report["model_metrics"]["model_available"] is False


def test_run_evaluation_marks_unavailable_model_as_warning(tmp_path):
    df = bullish_trend_df(n=170)
    path = tmp_path / "2330.csv"
    _write_csv(path, df)

    report = run_evaluation(
        csv_sources=[f"{path}:2330"],
        model_path=str(tmp_path / "missing.joblib"),
        dataset_config=_dataset_config(),
    )

    assert report["model_metrics"]["model_available"] is False
    assert report["warnings"]
    assert "model unavailable" in report["warnings"][0]


def test_run_evaluation_records_custom_atr_builder_config(tmp_path):
    df = bullish_trend_df(n=170)
    path = tmp_path / "2330.csv"
    _write_csv(path, df)

    report = run_evaluation(
        csv_sources=[f"{path}:2330"],
        dataset_config=_dataset_config(),
        builder_config=ZoneBuilderConfig(
            atr=ATRZoneBuilderConfig(
                lookback=55,
                atr_period=10,
                atr_width_multiplier=1.25,
                max_merge_width_multiple=1.5,
            )
        ),
    )

    atr = report["builder_config"]["ATRZoneBuilder"]
    assert atr["lookback"] == 55
    assert atr["atr_period"] == 10
    assert atr["atr_width_multiplier"] == 1.25
    assert atr["max_merge_width_multiple"] == 1.5


def test_write_evaluation_result_inserts_regression_row(tmp_path):
    db_path = tmp_path / "eval.db"
    engine = create_engine(f"sqlite:///{db_path}")
    with engine.begin() as conn:
        conn.execute(text("""
            CREATE TABLE stock_sr_regression_results (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                run_id VARCHAR(64) NOT NULL UNIQUE,
                model_config_hash VARCHAR(40) NOT NULL DEFAULT '',
                pipeline_version VARCHAR(40) NOT NULL DEFAULT '',
                dataset_from DATETIME,
                dataset_to DATETIME,
                split_method VARCHAR(20) NOT NULL DEFAULT '',
                hold_auc REAL,
                hold_brier_score REAL,
                break_auc REAL,
                break_brier_score REAL,
                passed BOOLEAN,
                schema_version VARCHAR(64) NOT NULL DEFAULT '',
                result_rows INTEGER,
                source_count INTEGER,
                governance_health_state VARCHAR(40) NOT NULL DEFAULT '',
                governance_strict_passed BOOLEAN,
                metrics_json TEXT NOT NULL DEFAULT 'null',
                created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
            )
        """))

    report = {
        "run_id": "sr_eval_insert_001",
        "model_config_hash": "hash123",
        "pipeline_version": "sr_zone_evaluation_test",
        "dataset_from": "2024-01-01T00:00:00+00:00",
        "dataset_to": "2024-02-01T00:00:00+00:00",
        "split_method": "walk_forward",
        "model_metrics": {
            "hold": {"auc": 0.8, "brier_score": 0.12},
            "break": {"auc": 0.7, "brier_score": 0.2},
        },
        "schema_version": "sr_zone_evaluation_p0",
        "sources": 1,
        "rows": 12,
    }

    write_evaluation_result(report, passed=True, engine_override=engine)

    with engine.connect() as conn:
        row = conn.execute(text("SELECT * FROM stock_sr_regression_results WHERE run_id=:run_id"), {
            "run_id": "sr_eval_insert_001",
        }).mappings().one()

    assert row["model_config_hash"] == "hash123"
    assert row["pipeline_version"] == "sr_zone_evaluation_test"
    assert row["split_method"] == "walk_forward"
    assert row["hold_auc"] == 0.8
    assert row["hold_brier_score"] == 0.12
    assert row["break_auc"] == 0.7
    assert row["break_brier_score"] == 0.2
    assert row["passed"] == 1
    assert row["schema_version"] == "sr_zone_evaluation_p0"
    assert row["result_rows"] == 12
    assert row["source_count"] == 1
    assert row["governance_health_state"] == ""
    assert row["governance_strict_passed"] is None
    assert json.loads(row["metrics_json"])["rows"] == 12


def test_write_evaluation_result_accepts_decision_replay_report(tmp_path):
    db_path = tmp_path / "eval.db"
    engine = create_engine(f"sqlite:///{db_path}")
    with engine.begin() as conn:
        conn.execute(text("""
            CREATE TABLE stock_sr_regression_results (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                run_id VARCHAR(64) NOT NULL UNIQUE,
                model_config_hash VARCHAR(40) NOT NULL DEFAULT '',
                pipeline_version VARCHAR(40) NOT NULL DEFAULT '',
                dataset_from DATETIME,
                dataset_to DATETIME,
                split_method VARCHAR(20) NOT NULL DEFAULT '',
                hold_auc REAL,
                hold_brier_score REAL,
                break_auc REAL,
                break_brier_score REAL,
                passed BOOLEAN,
                schema_version VARCHAR(64) NOT NULL DEFAULT '',
                result_rows INTEGER,
                source_count INTEGER,
                governance_health_state VARCHAR(40) NOT NULL DEFAULT '',
                governance_strict_passed BOOLEAN,
                metrics_json TEXT NOT NULL DEFAULT 'null',
                created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
            )
        """))

    report = {
        "schema_version": "sr_zone_decision_replay_p0",
        "run_id": "sr_replay_insert_001",
        "pipeline_version": "sr_zone_decision_replay_test",
        "model_config_hash": "hash123",
        "dataset_from": "2024-01-01T00:00:00+00:00",
        "dataset_to": "2024-02-01T00:00:00+00:00",
        "sources": 1,
        "rows": 3,
        "outcome_summary": {"rows": 3},
        "governance_evaluation": {"health_state": "UNRELIABLE", "passed": False, "strict_passed": False},
    }

    write_evaluation_result(report, passed=None, engine_override=engine)

    with engine.connect() as conn:
        row = conn.execute(text("SELECT * FROM stock_sr_regression_results WHERE run_id=:run_id"), {
            "run_id": "sr_replay_insert_001",
        }).mappings().one()

    metrics = json.loads(row["metrics_json"])
    assert row["model_config_hash"] == "hash123"
    assert row["pipeline_version"] == "sr_zone_decision_replay_test"
    assert row["hold_auc"] is None
    assert row["hold_brier_score"] is None
    assert row["break_auc"] is None
    assert row["break_brier_score"] is None
    assert row["passed"] == 0
    assert row["schema_version"] == "sr_zone_decision_replay_p0"
    assert row["result_rows"] == 3
    assert row["source_count"] == 1
    assert row["governance_health_state"] == "UNRELIABLE"
    assert row["governance_strict_passed"] == 0
    assert metrics["schema_version"] == "sr_zone_decision_replay_p0"
    assert metrics["outcome_summary"]["rows"] == 3
    assert metrics["governance_evaluation"]["passed"] is False


def test_run_builder_sweep_returns_parameter_candidates(tmp_path):
    df = bullish_trend_df(n=170)
    path = tmp_path / "2330.csv"
    _write_csv(path, df)

    report = run_builder_sweep(
        csv_sources=[f"{path}:2330"],
        dataset_config=_dataset_config(),
        atr_width_multipliers=[1.0, 1.5],
        max_merge_width_multiples=[1.5, 2.0],
        run_id="sr_sweep_test_001",
        pipeline_version="sr_zone_builder_sweep_test",
    )

    assert report["schema_version"] == "sr_zone_builder_sweep_p1"
    assert report["run_id"] == "sr_sweep_test_001"
    assert report["pipeline_version"] == "sr_zone_builder_sweep_test"
    assert report["candidate_count"] == 4
    assert len(report["results"]) == 4
    assert report["parameter_grid"]["atr_width_multipliers"] == [1.0, 1.5]
    assert report["parameter_grid"]["max_merge_width_multiples"] == [1.5, 2.0]

    candidate = report["results"][0]
    assert candidate["run_id"].startswith("sr_sweep_test_001_w")
    assert candidate["rows"] > 0
    assert candidate["zone_outcomes"]["rows"] == candidate["rows"]
    assert candidate["zone_outcomes"]["by_volatility_bucket"]
    assert candidate["builder_config"]["ATRZoneBuilder"]["atr_width_multiplier"] in [1.0, 1.5]
    assert candidate["builder_config"]["ATRZoneBuilder"]["max_merge_width_multiple"] in [1.5, 2.0]
    assert set(report["best_by"]) == {
        "support_hold_rate",
        "resistance_rejection_rate",
        "break_positive_rate",
        "average_forward_return",
        "entry_average_forward_return",
    }
    # 預設不跑 decision replay，所以 decision 層的排名與欄位都是空的。
    assert report["decision_replay_enabled"] is False
    assert report["best_by"]["entry_average_forward_return"] is None
    assert candidate["decision_outcomes"] is None
    assert report["recommended_configs_by_bucket"]
    for bucket, recommendation in report["recommended_configs_by_bucket"].items():
        assert recommendation["bucket"] == bucket
        assert recommendation["minimum_rows"] == 20
        assert recommendation["ranking"]
        assert recommendation["ranking"][0]["rank"] == 1
        assert recommendation["ranking"][0]["atr_width_multiplier"] in [1.0, 1.5]
        assert recommendation["ranking"][0]["max_merge_width_multiple"] in [1.5, 2.0]
        if recommendation["insufficient_sample"]:
            assert recommendation["recommended_config"] is None
        else:
            assert recommendation["recommended_config"]["atr_width_multiplier"] in [1.0, 1.5]
            assert recommendation["recommended_config"]["max_merge_width_multiple"] in [1.5, 2.0]


def test_run_decision_replay_reports_unavailable_without_model(tmp_path):
    df = bullish_trend_df(n=170)
    path = tmp_path / "2330.csv"
    _write_csv(path, df)

    report = run_decision_replay(
        csv_sources=[f"{path}:2330"],
        dataset_config=_dataset_config(),
        replay_max_rows=4,
        run_id="sr_replay_test_001",
        pipeline_version="sr_zone_decision_replay_test",
    )

    assert report["schema_version"] == "sr_zone_decision_replay_p0"
    assert report["run_id"] == "sr_replay_test_001"
    assert report["pipeline_version"] == "sr_zone_decision_replay_test"
    assert report["symbols"] == ["2330"]
    assert report["decision_replay_available"] is False
    assert report["event_lifecycle_replay_available"] is False
    assert report["model_available"] is False
    assert report["model_metadata"]["available"] is False
    assert report["model_version"] is None
    assert report["model_config_hash"] == ""
    assert report["model_trained_at"] is None
    assert report["decision_fields_available"] is False
    assert report["zone_score_fields_available"] is False
    assert report["outcome_rows_available"] is True
    assert report["replay_max_rows"] == 4
    assert report["rows"] == 4
    assert len(report["replay_rows"]) == 4
    row = report["replay_rows"][0]
    forward_bars = max(_dataset_config().forward_bars_support, _dataset_config().forward_bars_resistance)
    # replay 取的是每檔「最新」的 replay_max_rows 根，所以第一列落在窗口起點
    # last_idx - quota + 1，不是資料最前面
    # （見 docs/sr-zone-scoring.md「Decision Replay 的取樣規則」）。
    last_idx = len(df) - forward_bars - 1
    window_start = last_idx - 4 + 1
    expected_forward_return = (
        float(df["close"].iloc[window_start + forward_bars]) / float(df["close"].iloc[window_start])
    ) - 1.0
    assert row["symbol"] == "2330"
    assert row["timeframe"] == "1d"
    assert row["current_price"] == float(df["close"].iloc[window_start])
    assert row["candle_open"] == float(df["open"].iloc[window_start])
    assert row["candle_high"] == float(df["high"].iloc[window_start])
    assert row["candle_low"] == float(df["low"].iloc[window_start])
    assert row["candle_close"] == float(df["close"].iloc[window_start])
    assert row["previous_candle_close"] == float(df["close"].iloc[window_start - 1])
    assert row["forward_bars"] == forward_bars
    assert row["forward_return"] == pytest.approx(expected_forward_return)
    assert row["next_close_return"] is not None
    assert row["two_bar_close_return"] is not None
    assert row["zone_score_available"] is False
    assert row["zone_score_error"] is None
    assert row["zone_count"] == 0
    assert row["primary_zone"] is None
    assert row["global_trend"] is None
    assert row["global_volatility"] is None
    assert row["global_metrics"] is None
    assert row["chip_summary"] == _missing_chip_summary()
    assert row["model_governance_available"] is False
    assert row["model_governance_source_time"] is None
    assert row["model_governance"] is None
    assert row["market_bias"] is None
    assert row["lifecycle_phase"] is None
    assert row["daily_confirmation_state"] is None
    assert row["daily_confirmation_outcome"]["available"] is False
    assert row["daily_confirmation_outcome"]["reason_code"] == "DAILY_CONFIRMATION_STATE_UNAVAILABLE"
    assert row["daily_confirmation_context"]["rr_gate"] == "RR_BLOCKED"
    assert row["daily_confirmation_context"]["volume_context"] == "NO_VOLUME_CONFIRMATION"
    assert row["final_entry_state"] is None
    assert row["rr_context"] is None
    assert row["rr_gate"] is None
    assert row["decision_fields_available"] is False
    assert row["decision_error"] is None
    assert row["event_lifecycle_replay_available"] is False
    assert row["event_state_count"] == 0
    assert row["active_event_count"] == 0
    assert row["resolved_event_count"] == 0
    assert row["expired_event_count"] == 0
    assert report["outcome_summary"]["rows"] == 4
    assert report["outcome_summary"]["rows_by_symbol"] == {"2330": 4}
    assert report["outcome_summary"]["average_forward_return"] is not None
    assert report["outcome_summary"]["rows_with_outcome"] == 4
    assert report["outcome_summary"]["rows_with_zone_score"] == 0
    assert report["outcome_summary"]["rows_with_global_context"] == 0
    assert report["outcome_summary"]["rows_with_chip_context"] == 4
    assert report["outcome_summary"]["rows_with_non_missing_chip"] == 0
    assert report["outcome_summary"]["chip_missing_rows"] == 4
    assert report["outcome_summary"]["rows_with_model_governance"] == 0
    assert report["outcome_summary"]["model_governance_missing_rows"] == 4
    assert report["outcome_summary"]["rows_with_decision_fields"] == 0
    assert report["outcome_summary"]["rows_with_event_lifecycle"] == 0
    assert report["outcome_summary"]["zone_score_error_counts"] == {}
    assert report["outcome_summary"]["decision_error_counts"] == {}
    assert report["outcome_summary"]["final_entry_state_counts"] == {}
    assert report["outcome_summary"]["daily_confirmation_state_counts"] == {}
    assert report["outcome_summary"]["daily_confirmation_summary"]["rows"] == 0
    assert report["outcome_summary"]["by_final_entry_state"] == {}
    assert report["outcome_summary"]["by_lifecycle_phase"] == {}
    assert report["outcome_summary"]["lifecycle_phase_counts"] == {}
    assert report["outcome_summary"]["by_daily_confirmation_state"] == {}
    assert report["outcome_summary"]["by_market_bias"] == {}
    empty_distribution = {
        "count": 0,
        "average": None,
        "stddev": None,
        "min": None,
        "p10": None,
        "p25": None,
        "median": None,
        "p75": None,
        "p90": None,
        "max": None,
    }
    assert report["outcome_summary"]["rr_summary"] == {
        "rows_with_entry_rr": 0,
        "average_entry_rr": None,
        "median_entry_rr": None,
        "rows_with_position_rr": 0,
        "average_position_rr": None,
        "median_position_rr": None,
        "entry_rr_source_counts": {},
        "position_rr_source_counts": {},
        "rows_with_execution_rr": 0,
        "average_execution_rr": None,
        "median_execution_rr": None,
        "execution_rr_source_counts": {},
        # 沒有樣本時分布是 count=0 ＋ 其餘 None，不是一堆 0
        "entry_rr_distribution": empty_distribution,
        "execution_rr_distribution": empty_distribution,
        "position_rr_distribution": empty_distribution,
    }
    assert report["replay_plan"][0]["symbol"] == "2330"
    assert report["replay_plan"][0]["candidate_bars"] > 0
    assert report["replay_plan"][0]["start_as_of"] is not None
    assert report["replay_plan"][0]["end_as_of"] is not None
    assert "final_entry_state" in report["planned_fields"]
    assert "lifecycle_phase" in report["planned_fields"]
    assert "daily_confirmation_outcome" in report["planned_fields"]
    assert "daily_confirmation_context" in report["planned_fields"]
    assert "rr_gate" in report["planned_fields"]
    assert "event_lifecycle_replay_available" in report["planned_fields"]
    assert report["governance_evaluation"]["health_state"] == "UNRELIABLE"
    assert report["governance_evaluation"]["passed"] is False
    assert "MODEL_UNAVAILABLE" in report["governance_evaluation"]["blocking_flags"]
    assert "production governance gate promotion" in report["pending_requirements"]
    assert report["warnings"] == ["model unavailable: decision replay requires --model-path"]
    assert report["volatility_profiles"]["2330"]["touch_count"] == 0


def test_run_decision_replay_reports_model_metadata_and_plan(tmp_path, monkeypatch):
    df = bullish_trend_df(n=170)
    path = tmp_path / "2330.csv"
    _write_csv(path, df)
    bundle = ModelBundle(
        hold_model=_FixedProbabilityModel(0.62),
        break_model=_FixedProbabilityModel(0.28),
        feature_names=[],
        trained_at="2026-07-01T00:00:00+00:00",
        version="v-test",
        config_hash="hash123",
    )
    monkeypatch.setattr(evaluation_module, "load_model", lambda _: bundle)

    report = run_decision_replay(
        csv_sources=[f"{path}:2330"],
        model_path=str(tmp_path / "model.joblib"),
        dataset_config=_dataset_config(),
        replay_max_rows=3,
        run_id="sr_replay_test_002",
    )

    assert report["decision_replay_available"] is True
    assert report["event_lifecycle_replay_available"] is True
    assert report["model_available"] is True
    assert report["zone_score_fields_available"] is True
    assert report["decision_fields_available"] is True
    assert report["model_metadata"] == {
        "available": True,
        "version": "v-test",
        "config_hash": "hash123",
        "trained_at": "2026-07-01T00:00:00+00:00",
    }
    assert report["model_version"] == "v-test"
    assert report["model_config_hash"] == "hash123"
    assert report["model_trained_at"] == "2026-07-01T00:00:00+00:00"
    assert report["decision_fields_available"] is True
    assert report["outcome_rows_available"] is True
    assert report["rows"] == 3
    assert len(report["replay_rows"]) == 3
    assert report["outcome_summary"]["rows"] == 3
    assert report["outcome_summary"]["rows_with_outcome"] == 3
    assert report["outcome_summary"]["rows_with_zone_score"] > 0
    assert report["outcome_summary"]["rows_with_global_context"] > 0
    assert report["outcome_summary"]["rows_with_chip_context"] == 3
    assert report["outcome_summary"]["rows_with_model_governance"] == 0
    assert report["outcome_summary"]["model_governance_missing_rows"] == 3
    assert report["outcome_summary"]["rows_with_decision_fields"] > 0
    assert report["outcome_summary"]["rows_with_event_lifecycle"] > 0
    assert report["outcome_summary"]["zone_score_error_counts"] == {}
    assert report["outcome_summary"]["decision_error_counts"] == {}
    assert report["outcome_summary"]["final_entry_state_counts"]
    # lifecycle 聚合與 final_entry_state 同級：沒有它，replay 的驗收就得回頭
    # 自己撈原始 JSON 重算（見 docs/issue.md I-074 的 2026-09-01 實測）。
    assert report["outcome_summary"]["lifecycle_phase_counts"]
    assert sum(report["outcome_summary"]["lifecycle_phase_counts"].values()) == len(
        [row for row in report["replay_rows"] if row.get("lifecycle_phase")]
    )
    assert report["outcome_summary"]["by_lifecycle_phase"]
    assert report["outcome_summary"]["daily_confirmation_state_counts"]
    assert report["outcome_summary"]["daily_confirmation_summary"]["rows"] > 0
    assert report["outcome_summary"]["daily_confirmation_summary"]["by_state"]
    assert report["outcome_summary"]["daily_confirmation_summary"]["by_volume_context"]
    assert report["outcome_summary"]["daily_confirmation_summary"]["by_event_sequence"]
    assert report["outcome_summary"]["daily_confirmation_summary"]["by_rr_gate"]
    assert report["outcome_summary"]["daily_confirmation_summary"]["failure_distribution"]
    assert report["outcome_summary"]["by_final_entry_state"]
    assert report["outcome_summary"]["by_daily_confirmation_state"]
    assert report["outcome_summary"]["by_market_bias"]
    assert sum(group["rows"] for group in report["outcome_summary"]["by_final_entry_state"].values()) == (
        report["outcome_summary"]["rows_with_decision_fields"]
    )
    first_group = next(iter(report["outcome_summary"]["by_final_entry_state"].values()))
    assert first_group["rows_with_forward_return"] > 0
    assert first_group["average_forward_return"] is not None
    assert first_group["positive_forward_return_rate"] is not None
    assert first_group["negative_forward_return_rate"] is not None
    rr_summary = report["outcome_summary"]["rr_summary"]
    # 刻意用精確的 key 集合而不是子集比對：欄位增減要被這裡擋一次，
    # 前端才不會在不知情的狀況下與後端形狀分岔。
    assert set(rr_summary) == {
        "rows_with_entry_rr",
        "average_entry_rr",
        "median_entry_rr",
        "rows_with_position_rr",
        "average_position_rr",
        "median_position_rr",
        "entry_rr_source_counts",
        "position_rr_source_counts",
        # 2026-08-07 新增（RR distribution）
        "rows_with_execution_rr",
        "average_execution_rr",
        "median_execution_rr",
        "execution_rr_source_counts",
        "entry_rr_distribution",
        "execution_rr_distribution",
        "position_rr_distribution",
    }
    assert isinstance(rr_summary["entry_rr_source_counts"], dict)
    assert isinstance(rr_summary["position_rr_source_counts"], dict)
    assert isinstance(rr_summary["execution_rr_source_counts"], dict)
    # 分布與既有的 median_* 必須一致，否則前端會顯示兩個互相矛盾的數字
    assert rr_summary["entry_rr_distribution"]["count"] == rr_summary["rows_with_entry_rr"]
    scored_row = next(row for row in report["replay_rows"] if row["zone_score_available"])
    assert scored_row["zone_count"] > 0
    assert scored_row["zone_score_error"] is None
    # replay row 的 primary_zone 是對外 projection，欄位增減要在這裡被擋一次。
    #
    # **這是 Python 與前端型別之間唯一的連結點。** TypeScript 偵測不到 Python 的變動
    # （這個不對稱正是 key 名寫錯與型別不符這兩類事故能潛伏數週的原因，見
    #   docs/development-workflow.md §3），
    # 所以由這一側主動失敗並提醒。改這裡的同時要確認
    # `frontend/src/lib/api/srZones.ts` 有沒有對應型別需要一起改
    # ——目前**刻意沒有**replay row 的 TS 型別（見 docs/development-workflow.md §3「什麼時候才該新增跨語言的型別宣告」）：前端不消費
    # replay_rows，加一個沒有消費者的宣告只會重蹈「型別沒被消費所以默默寫錯」的覆轍。
    # 這份清單就是日後真的要加型別時的權威來源，不要憑記憶手寫。
    assert set(scored_row["primary_zone"]) == {
        "role",
        "tier",
        "price_low",
        "price_high",
        "confidence",
        "trading_score",
        "risk_reward_ratio",
        "volume_confirmation",
        "relative_volume",
    }
    assert scored_row["primary_zone"]["role"] in {"SUPPORT", "RESISTANCE", "AT_ZONE"}
    assert scored_row["primary_zone"]["price_low"] < scored_row["primary_zone"]["price_high"]
    assert scored_row["primary_zone"]["confidence"] is not None
    assert scored_row["global_trend"] is not None
    assert scored_row["global_volatility"] is not None
    assert scored_row["global_metrics"]["confidence"] is not None
    assert scored_row["chip_summary"] == _missing_chip_summary()
    assert scored_row["model_governance_available"] is False
    assert scored_row["model_governance_source_time"] is None
    assert scored_row["model_governance"] is None
    assert scored_row["decision_fields_available"] is True
    assert scored_row["decision_error"] is None
    assert scored_row["event_lifecycle_replay_available"] is True
    assert scored_row["event_state_count"] >= 0
    assert scored_row["active_event_count"] >= 0
    assert scored_row["resolved_event_count"] >= 0
    assert scored_row["expired_event_count"] >= 0
    assert scored_row["market_bias"] is not None
    assert scored_row["lifecycle_phase"] is not None
    assert scored_row["daily_confirmation_state"] is not None
    assert scored_row["daily_confirmation_outcome"]["available"] is True
    assert scored_row["daily_confirmation_context"]["rr_gate"] in {"RR_QUALIFIED", "RR_BLOCKED"}
    assert scored_row["daily_confirmation_context"]["rr_bucket"]
    assert scored_row["next_close_return"] is not None
    assert scored_row["two_bar_close_return"] is not None
    assert scored_row["final_entry_state"] is not None
    assert scored_row["rr_context"] is not None
    assert scored_row["rr_gate"] is not None
    assert report["replay_plan"] == [{
        "symbol": "2330",
        "timeframe": "1d",
        "candle_count": 170,
        "candidate_bars": 107,
        "start_as_of": report["replay_plan"][0]["start_as_of"],
        "end_as_of": report["replay_plan"][0]["end_as_of"],
        "min_history_bars": 60,
        "forward_bars": 3,
    }]
    assert report["warnings"] == []


def test_run_decision_replay_uses_latest_historical_chip_row(tmp_path, monkeypatch):
    df = bullish_trend_df(n=170)
    path = tmp_path / "2330.csv"
    _write_csv(path, df)
    bundle = ModelBundle(
        hold_model=_FixedProbabilityModel(0.62),
        break_model=_FixedProbabilityModel(0.28),
        feature_names=[],
        trained_at="2026-07-01T00:00:00+00:00",
        version="v-test",
        config_hash="hash123",
    )
    monkeypatch.setattr(evaluation_module, "load_model", lambda _: bundle)
    chip_rows = [
        {
            "trade_date": "1970-01-01",
            "signal_type": "BULLISH",
            "institutional_score": 30.0,
            "margin_score": 10.0,
            "broker_score": 20.0,
            "concentration_score": 40.0,
        },
        {
            "trade_date": "2099-01-01",
            "signal_type": "BEARISH",
            "institutional_score": -50.0,
            "margin_score": -20.0,
            "broker_score": -30.0,
            "concentration_score": 10.0,
        },
    ]

    report = run_decision_replay(
        csv_sources=[f"{path}:2330"],
        model_path=str(tmp_path / "model.joblib"),
        dataset_config=_dataset_config(),
        replay_max_rows=2,
        chip_scores_by_symbol={"2330": chip_rows},
    )

    row = report["replay_rows"][0]
    assert row["chip_summary"]["missing"] is False
    assert row["chip_summary"]["trade_date"] == "1970-01-01"
    assert row["chip_summary"]["source_signal"] == "BULLISH"
    assert row["chip_summary"]["institutional_score"] == 30.0
    assert report["outcome_summary"]["rows_with_chip_context"] == 2
    assert report["outcome_summary"]["rows_with_non_missing_chip"] == 2
    assert report["outcome_summary"]["chip_missing_rows"] == 0


def test_run_decision_replay_uses_latest_model_governance_snapshot(tmp_path, monkeypatch):
    df = bullish_trend_df(n=170)
    path = tmp_path / "2330.csv"
    _write_csv(path, df)
    bundle = ModelBundle(
        hold_model=_FixedProbabilityModel(0.62),
        break_model=_FixedProbabilityModel(0.28),
        feature_names=[],
        trained_at="2026-07-01T00:00:00+00:00",
        version="v-test",
        config_hash="hash123",
    )
    monkeypatch.setattr(evaluation_module, "load_model", lambda _: bundle)
    governance = {
        "as_of": "1970-01-01T00:00:00+00:00",
        "health_state": "DEGRADED",
        "quality_flags": ["TEST_DEGRADED"],
        "warning_flags": [],
        "blocking_flags": [],
        "confidence_gate": {
            "state": "DEGRADED",
            "allow_entry": False,
            "max_entry_state": "WAIT_CONFIRMATION",
            "reason_codes": ["TEST_MODEL_GOVERNANCE"],
        },
    }

    report = run_decision_replay(
        csv_sources=[f"{path}:2330"],
        model_path=str(tmp_path / "model.joblib"),
        dataset_config=_dataset_config(),
        replay_max_rows=2,
        model_governance_by_symbol={"2330": [governance]},
    )

    assert report["outcome_summary"]["rows_with_model_governance"] == 2
    assert report["outcome_summary"]["model_governance_missing_rows"] == 0
    row = report["replay_rows"][0]
    assert row["model_governance_available"] is True
    assert row["model_governance_source_time"] == "1970-01-01T00:00:00+00:00"
    assert row["model_governance"]["health_state"] == "DEGRADED"
    assert row["decision_fields_available"] is True


def test_run_decision_replay_loads_db_context_for_symbol_replay(tmp_path, monkeypatch):
    df = bullish_trend_df(n=170)
    bundle = ModelBundle(
        hold_model=_FixedProbabilityModel(0.62),
        break_model=_FixedProbabilityModel(0.28),
        feature_names=[],
        trained_at="2026-07-01T00:00:00+00:00",
        version="v-test",
        config_hash="hash123",
    )
    fake_db = types.ModuleType("db")
    fake_db.fetch_candles = lambda symbol, timeframe, limit: _candle_rows(df)
    fake_db.fetch_chip_scores = lambda symbol, from_date, to_date: [{
        "trade_date": "1970-01-01",
        "signal_type": "BULLISH",
        "institutional_score": 30.0,
        "margin_score": 10.0,
        "broker_score": 20.0,
        "concentration_score": 40.0,
    }]
    fake_db.fetch_sr_model_governance = lambda symbol, timeframe, from_ts, to_ts: [{
        "as_of": "1970-01-01T00:00:00+00:00",
        "health_state": "HEALTHY",
        "quality_flags": [],
        "warning_flags": [],
        "blocking_flags": [],
        "confidence_gate": {
            "state": "HEALTHY",
            "allow_entry": True,
            "max_entry_state": "ENTRY_ALLOWED",
            "reason_codes": [],
        },
    }]
    monkeypatch.setitem(sys.modules, "db", fake_db)
    monkeypatch.setattr(evaluation_module, "load_model", lambda _: bundle)

    report = run_decision_replay(
        symbols=["2330"],
        timeframe="1d",
        limit=170,
        model_path=str(tmp_path / "model.joblib"),
        dataset_config=_dataset_config(),
        replay_max_rows=2,
    )

    row = report["replay_rows"][0]
    assert row["chip_summary"]["missing"] is False
    assert row["chip_summary"]["trade_date"] == "1970-01-01"
    assert row["model_governance_available"] is True
    assert row["model_governance"]["health_state"] == "HEALTHY"
    assert report["outcome_summary"]["rows_with_non_missing_chip"] == 2
    assert report["outcome_summary"]["rows_with_model_governance"] == 2


# ── I-042：replay_max_rows 是總預算，必須跨股票均分 ────────────────────────


def _multi_symbol_replay(tmp_path, symbol_bars: dict[str, int], replay_max_rows: int):
    csv_sources = []
    for symbol, bars in symbol_bars.items():
        path = tmp_path / f"{symbol}.csv"
        _write_csv(path, bullish_trend_df(n=bars))
        csv_sources.append(f"{path}:{symbol}")
    return run_decision_replay(
        csv_sources=csv_sources,
        dataset_config=_dataset_config(),
        replay_max_rows=replay_max_rows,
    )


def test_decision_replay_splits_budget_across_symbols(tmp_path):
    """預算跨股票均分：不能像修復前那樣被第一檔股票吃光。"""
    # 預算至少要 檔數 × MIN_ROWS_PER_SYMBOL，否則會走「少覆蓋幾檔」的分支（見下一個測試）。
    report = _multi_symbol_replay(tmp_path, {"2330": 170, "2454": 170, "1101": 170}, 15)

    rows_by_symbol = report["outcome_summary"]["rows_by_symbol"]
    assert rows_by_symbol == {"1101": 5, "2330": 5, "2454": 5}
    assert report["rows"] == 15

    coverage = report["replay_coverage"]
    assert coverage["symbols_requested"] == 3
    assert coverage["symbols_covered"] == 3
    assert coverage["symbols_skipped"] == []
    assert coverage["coverage_ratio"] == 1.0
    assert coverage["window_mode"] == "latest"


def test_decision_replay_redistributes_unused_quota(tmp_path):
    """短天期個股用不完的配額要還給其他股票，不能浪費總預算。"""
    # min_history_bars=60、forward_bars=3 → 64 根只剩 1 根可當 as-of。
    report = _multi_symbol_replay(tmp_path, {"2330": 170, "2454": 170, "1101": 64}, 15)

    rows_by_symbol = report["outcome_summary"]["rows_by_symbol"]
    assert rows_by_symbol["1101"] == 1
    assert report["rows"] == 15
    # 1101 只吃得下 1 列，剩下的 14 列要回流給另外兩檔，而不是浪費掉。
    assert rows_by_symbol["2330"] == 7
    assert rows_by_symbol["2454"] == 7
    assert report["replay_coverage"]["symbols_covered"] == 3


def test_decision_replay_skips_symbols_when_budget_below_floor(tmp_path):
    """預算不足以讓每檔拿到 MIN_ROWS_PER_SYMBOL 時，寧可少覆蓋幾檔並回報。"""
    report = _multi_symbol_replay(
        tmp_path,
        {"2330": 170, "2454": 170, "1101": 170, "2603": 170},
        evaluation_module.MIN_ROWS_PER_SYMBOL * 2,
    )

    coverage = report["replay_coverage"]
    assert coverage["symbols_requested"] == 4
    assert coverage["symbols_covered"] == 2
    assert len(coverage["symbols_skipped"]) == 2
    assert coverage["coverage_ratio"] == pytest.approx(0.5)
    # report 的 symbols/sources 描述「要求的範圍」，coverage 描述「實際驗證到的範圍」。
    assert len(report["symbols"]) == 4
    assert report["sources"] == 4
    assert any("replay coverage partial" in warning for warning in report["warnings"])


def test_decision_replay_uses_latest_window_per_symbol(tmp_path):
    """每檔取的是最新的窗口，不是最舊的（模型健康度要反映近期盤勢）。"""
    bars = 170
    report = _multi_symbol_replay(tmp_path, {"2330": bars, "2454": bars}, 10)

    quota = 5
    # 用 report 自己的 replay_plan 當基準，避免依賴 CSV 來回轉換後的時間表示法。
    plan_by_symbol = {entry["symbol"]: entry for entry in report["replay_plan"]}

    for symbol in ("2330", "2454"):
        plan = plan_by_symbol[symbol]
        symbol_rows = [row for row in report["replay_rows"] if row["symbol"] == symbol]
        assert len(symbol_rows) == quota
        # 走到最新的一根可用 as-of。
        assert symbol_rows[-1]["as_of"] == plan["end_as_of"]
        # 沒有從最舊的一根開始（candidate_bars 遠大於配額）。
        assert plan["candidate_bars"] > quota
        assert symbol_rows[0]["as_of"] != plan["start_as_of"]
        # 連續遞增區間，event lifecycle 才接得上前一根的狀態。
        as_of_values = [row["as_of"] for row in symbol_rows]
        assert as_of_values == sorted(as_of_values)


def test_partial_symbol_coverage_degrades_but_does_not_block():
    """覆蓋不足是 warning（DEGRADED），不能升成 blocking 把 production 全面停單。"""
    outcome_summary = {
        "rows": 120,
        "rows_with_decision_fields": 120,
        "rows_with_model_governance": 120,
        "rows_with_event_lifecycle": 120,
        "by_final_entry_state": {},
        "decision_error_counts": {},
        "zone_score_error_counts": {},
    }

    full = _decision_replay_governance_evaluation(
        outcome_summary,
        model_available=True,
        replay_coverage={"coverage_ratio": 1.0},
    )
    partial = _decision_replay_governance_evaluation(
        outcome_summary,
        model_available=True,
        replay_coverage={"coverage_ratio": 0.5},
    )

    assert "REPLAY_SYMBOL_COVERAGE_PARTIAL" not in full["warning_flags"]
    assert "REPLAY_SYMBOL_COVERAGE_PARTIAL" in partial["warning_flags"]
    assert partial["health_state"] == "DEGRADED"
    assert partial["confidence_gate"]["allow_entry"] is True
    assert partial["confidence_gate"]["max_entry_state"] == "SMALL_ENTRY"
    assert "REPLAY_SYMBOL_COVERAGE_PARTIAL" not in partial["blocking_flags"]


# ── I-044：context 不得跨股票外洩 ──────────────────────────────────────────


def test_rows_for_symbol_never_leaks_across_symbols_in_multi_source():
    """多股票 replay 時，只有一檔有 context 不得讓其他股票誤用那份資料。

    Go 端 sr_evaluation_context.go 只把「查得到資料」的股票寫進 map，所以
    「兩檔 replay、只有 2330 有籌碼」正好會讓 dict 只剩一組 key。
    """
    context = {"2330": [{"trade_date": "2024-01-01"}]}

    assert _rows_for_symbol(context, "2330", single_source=False) == context["2330"]
    assert _rows_for_symbol(context, "2454", single_source=False) is None


def test_rows_for_symbol_tolerates_key_mismatch_only_for_single_source():
    """單一來源時才容忍 key 命名不一致（例如 2330 vs 2330.TW）。"""
    context = {"2330.TW": [{"trade_date": "2024-01-01"}]}

    assert _rows_for_symbol(context, "2330", single_source=True) == context["2330.TW"]
    assert _rows_for_symbol(context, "2330", single_source=False) is None


def test_rows_for_symbol_honours_default_key_even_with_multiple_symbols():
    """__default__ 是 CLI 明示「套用到所有股票」的語意，多股票也要生效。"""
    context = {evaluation_module.DEFAULT_SYMBOL_ROWS_KEY: [{"trade_date": "2024-01-01"}]}

    for symbol in ("2330", "2454"):
        assert _rows_for_symbol(context, symbol, single_source=False) == context[
            evaluation_module.DEFAULT_SYMBOL_ROWS_KEY
        ]


def test_decision_replay_does_not_borrow_chip_context_from_another_symbol(tmp_path):
    """端到端：只有 2330 有籌碼資料時，2454 的 chip_summary 必須是 missing。"""
    csv_sources = []
    for symbol in ("2330", "2454"):
        path = tmp_path / f"{symbol}.csv"
        _write_csv(path, bullish_trend_df(n=170))
        csv_sources.append(f"{path}:{symbol}")

    report = run_decision_replay(
        csv_sources=csv_sources,
        dataset_config=_dataset_config(),
        replay_max_rows=10,
        chip_scores_by_symbol={
            "2330": [{
                "trade_date": "1970-01-01",
                "total_score": 3.5,
                "institutional_score": 1.5,
                "signal_type": "BULLISH",
            }],
        },
    )

    by_symbol: dict[str, list[dict]] = {}
    for row in report["replay_rows"]:
        by_symbol.setdefault(row["symbol"], []).append(row)

    assert set(by_symbol) == {"2330", "2454"}
    assert all(row["chip_summary"]["missing"] is False for row in by_symbol["2330"])
    assert all(row["chip_summary"] == _missing_chip_summary() for row in by_symbol["2454"])


# ── I-045：context 順序不可信，必須自己排 ──────────────────────────────────


def test_decision_replay_sorts_descending_context_before_as_of_lookup(tmp_path):
    """呼叫端自帶降冪 context 時，as-of 比對仍要取到「最新且不晚於 as_of」那筆。

    `_chip_row_for_as_of` 掃到第一筆超過 as_of 就 break，若不先排序，降冪輸入會讓結果
    停在最舊那筆（見 docs/sr-zone-scoring.md「Replay context 的股票比對規則」）。
    """
    path = tmp_path / "2330.csv"
    _write_csv(path, bullish_trend_df(n=170))

    descending = [
        {"trade_date": "1970-01-05", "total_score": 4.0, "signal_type": "BULLISH"},
        {"trade_date": "1970-01-03", "total_score": 2.0, "signal_type": "NEUTRAL"},
        {"trade_date": "1970-01-01", "total_score": 1.0, "signal_type": "BEARISH"},
    ]

    report = run_decision_replay(
        csv_sources=[f"{path}:2330"],
        dataset_config=_dataset_config(),
        replay_max_rows=5,
        chip_scores_by_symbol={"2330": descending},
    )

    # 這批 as_of 都遠晚於 1970-01-05，所以每一列都該取到最新的那筆（total_score=4.0）。
    for row in report["replay_rows"]:
        assert row["chip_summary"]["missing"] is False
        assert row["chip_summary"]["trade_date"] == "1970-01-05"


# ── 配額分配的邊界 ────────────────────────────────────────────────────────


def test_decision_replay_does_not_double_spend_budget_on_duplicate_symbols(tmp_path):
    """同一檔股票重複出現時只能算一份配額，否則總產出會超過 replay_max_rows。"""
    path = tmp_path / "2330.csv"
    _write_csv(path, bullish_trend_df(n=170))

    report = run_decision_replay(
        csv_sources=[f"{path}:2330", f"{path}:2330"],
        dataset_config=_dataset_config(),
        replay_max_rows=10,
    )

    assert report["rows"] == 10
    assert report["outcome_summary"]["rows_by_symbol"] == {"2330": 10}
    assert report["replay_coverage"]["symbols_covered"] == 1


def test_context_rows_without_timestamp_do_not_break_sorting(tmp_path):
    """context row 缺時間欄位時只能被當成最舊，不能讓整個 replay 拋例外。"""
    path = tmp_path / "2330.csv"
    _write_csv(path, bullish_trend_df(n=170))

    report = run_decision_replay(
        csv_sources=[f"{path}:2330"],
        dataset_config=_dataset_config(),
        replay_max_rows=5,
        chip_scores_by_symbol={"2330": [
            {"total_score": 9.9, "signal_type": "BULLISH"},  # 沒有 trade_date
            {"trade_date": "1970-01-02", "total_score": 2.0, "signal_type": "NEUTRAL"},
        ]},
    )

    assert report["rows"] == 5
    # 有時間戳的那筆才是有效的 as-of 命中對象。
    for row in report["replay_rows"]:
        assert row["chip_summary"]["trade_date"] == "1970-01-02"


# ── builder config 必須真的影響 decision replay ────────────────────────────


def test_decision_replay_applies_builder_config(tmp_path, monkeypatch):
    """不同 atr_width_multiplier 必須產生不同的 zone 重建結果。

    修復前 `_decision_replay_rows` 沒把 builder config 傳給
    `_historical_zone_score_summary`，replay 一律用 baseline 參數——sweep 的每個
    candidate 會得到一模一樣的 decision 指標，整個參數比較都是假的。這條測試鎖這件事。
    """
    path = tmp_path / "2330.csv"
    _write_csv(path, bullish_trend_df(n=170))
    bundle = ModelBundle(
        hold_model=_FixedProbabilityModel(0.62),
        break_model=_FixedProbabilityModel(0.28),
        feature_names=[],
        trained_at="2026-07-01T00:00:00+00:00",
        version="v-test",
        config_hash="hash123",
    )
    monkeypatch.setattr(evaluation_module, "load_model", lambda _: bundle)

    def replay(width: float):
        return run_decision_replay(
            csv_sources=[f"{path}:2330"],
            dataset_config=_dataset_config(),
            model_path=str(tmp_path / "model.joblib"),
            replay_max_rows=5,
            builder_config=ZoneBuilderConfig(
                atr=ATRZoneBuilderConfig(
                    atr_width_multiplier=width,
                    max_merge_width_multiple=2.0,
                )
            ),
        )

    narrow = replay(0.5)
    wide = replay(4.0)

    # report 要能追溯這次用了哪組參數。
    assert narrow["builder_config"]["ATRZoneBuilder"]["atr_width_multiplier"] == 0.5
    assert wide["builder_config"]["ATRZoneBuilder"]["atr_width_multiplier"] == 4.0

    narrow_zones = [row["zone_count"] for row in narrow["replay_rows"]]
    wide_zones = [row["zone_count"] for row in wide["replay_rows"]]
    assert narrow["replay_rows"] and wide["replay_rows"]
    assert narrow_zones != wide_zones, (
        f"builder config 沒有生效：narrow={narrow_zones} wide={wide_zones}"
    )


def test_decision_replay_builder_config_defaults_to_baseline(tmp_path):
    """未指定 builder_config 時維持 baseline 預設，既有呼叫端行為不變。"""
    path = tmp_path / "2330.csv"
    _write_csv(path, bullish_trend_df(n=170))

    report = run_decision_replay(
        csv_sources=[f"{path}:2330"],
        dataset_config=_dataset_config(),
        replay_max_rows=3,
    )

    atr = report["builder_config"]["ATRZoneBuilder"]
    assert atr["atr_width_multiplier"] == 1.5
    assert atr["max_merge_width_multiple"] == 2.0


# ── calibration bins（T-002 P0 遺留指標） ──────────────────────────────────


def test_calibration_bins_detects_well_calibrated_predictions():
    """完美校準的合成資料：ECE 應該接近 0。"""
    rng = np.random.default_rng(20260804)
    proba = rng.uniform(0.0, 1.0, size=4000)
    y = (rng.uniform(0.0, 1.0, size=4000) < proba).astype(int)

    result = evaluation_module._calibration_bins(y, proba)

    assert result["schema_version"] == "sr_evaluation_calibration_v1"
    assert result["bin_count"] == evaluation_module.CALIBRATION_BIN_COUNT
    assert len(result["bins"]) == evaluation_module.CALIBRATION_BIN_COUNT
    assert result["rows"] == 4000
    assert result["insufficient_sample"] is False
    assert result["expected_calibration_error"] < 0.05
    assert result["max_calibration_error"] < 0.15


def test_calibration_bins_detects_overconfident_predictions():
    """模型系統性高估時，ECE 必須明顯拉高。"""
    rng = np.random.default_rng(20260804)
    proba = rng.uniform(0.6, 1.0, size=2000)
    # 實際只有兩成會發生，模型卻說六到十成。
    y = (rng.uniform(0.0, 1.0, size=2000) < 0.2).astype(int)

    result = evaluation_module._calibration_bins(y, proba)

    assert result["expected_calibration_error"] > 0.3
    # 高估 → observed 低於 predicted → gap 為負。
    populated = [b for b in result["bins"] if b["rows"] > 0]
    assert all(b["gap"] < 0 for b in populated)


def test_calibration_bins_keeps_empty_bins_and_flags_small_sample():
    """空 bin 要保留（跨 candidate 對齊比較），樣本不足要標記。"""
    proba = np.full(10, 0.95)
    y = np.ones(10, dtype=int)

    result = evaluation_module._calibration_bins(y, proba)

    assert result["insufficient_sample"] is True
    assert len(result["bins"]) == evaluation_module.CALIBRATION_BIN_COUNT
    empty = [b for b in result["bins"] if b["rows"] == 0]
    assert len(empty) == evaluation_module.CALIBRATION_BIN_COUNT - 1
    assert all(b["mean_predicted"] is None and b["gap"] is None for b in empty)
    # proba=1.0 也必須落進最後一個 bin，不能被邊界條件漏掉。
    assert evaluation_module._calibration_bins(np.ones(3, dtype=int), np.ones(3))["bins"][-1]["rows"] == 3


def test_model_metrics_include_calibration_when_model_available(tmp_path, monkeypatch):
    """hold / break 兩邊都要自動帶上 calibration（共用 _binary_metrics 入口）。"""
    path = tmp_path / "2330.csv"
    _write_csv(path, bullish_trend_df(n=170))
    bundle = ModelBundle(
        hold_model=_FixedProbabilityModel(0.62),
        break_model=_FixedProbabilityModel(0.28),
        feature_names=[],
        trained_at="2026-07-01T00:00:00+00:00",
        version="v-test",
        config_hash="hash123",
    )
    monkeypatch.setattr(evaluation_module, "load_model", lambda _: bundle)

    report = run_evaluation(
        csv_sources=[f"{path}:2330"],
        dataset_config=_dataset_config(),
        model_path=str(tmp_path / "model.joblib"),
    )

    for model in ("hold", "break"):
        calibration = report["model_metrics"][model]["calibration"]
        assert calibration["schema_version"] == "sr_evaluation_calibration_v1"
        assert len(calibration["bins"]) == evaluation_module.CALIBRATION_BIN_COUNT
        assert calibration["rows"] == report["model_metrics"][model]["rows"]


# ── sweep 接 decision replay ───────────────────────────────────────────────


def test_sweep_decision_replay_produces_per_candidate_decision_outcomes(tmp_path, monkeypatch):
    """開啟後每個 candidate 都要跑到自己那一次 replay，並帶出 decision_outcomes。

    注意：這裡**不能**斷言「候選之間的 decision 指標必須不同」。實測過用合成資料 +
    固定機率模型時，所有列都落在 `BLOCKED`，兩組差異極大的 atr_width_multiplier 會得到
    完全相同的 `by_final_entry_state`——那是資料本身沒有進場訊號，不是 wiring 壞掉。
    「builder config 真的有生效」由 test_decision_replay_applies_builder_config 以
    zone_count 直接鎖住；這條負責的是「每組候選各自跑了一次、且對應得起來」。
    """
    path = tmp_path / "2330.csv"
    _write_csv(path, bullish_trend_df(n=170))
    bundle = ModelBundle(
        hold_model=_FixedProbabilityModel(0.62),
        break_model=_FixedProbabilityModel(0.28),
        feature_names=[],
        trained_at="2026-07-01T00:00:00+00:00",
        version="v-test",
        config_hash="hash123",
    )
    monkeypatch.setattr(evaluation_module, "load_model", lambda _: bundle)

    report = run_builder_sweep(
        csv_sources=[f"{path}:2330"],
        dataset_config=_dataset_config(),
        model_path=str(tmp_path / "model.joblib"),
        atr_width_multipliers=[0.5, 4.0],
        max_merge_width_multiples=[2.0],
        decision_replay=True,
        replay_max_rows=6,
        run_id="sr_sweep_replay_001",
    )

    assert report["decision_replay_enabled"] is True
    assert report["candidate_count"] == 2

    for candidate in report["results"]:
        decision = candidate["decision_outcomes"]
        assert decision is not None
        # replay 的 run_id 必須對得回它所屬的 candidate，證明每組各跑了一次而不是共用結果。
        assert decision["run_id"] == f"{candidate['run_id']}_replay"
        assert decision["rows"] == 6
        assert decision["replay_coverage"]["symbols_covered"] == 1
        assert "by_final_entry_state" in decision
        assert "rr_summary" in decision

    assert report["results"][0]["builder_config"]["ATRZoneBuilder"]["atr_width_multiplier"] == 0.5
    assert report["results"][1]["builder_config"]["ATRZoneBuilder"]["atr_width_multiplier"] == 4.0


def test_sweep_decision_replay_skips_without_model_path(tmp_path):
    """沒有 model_path 時只記 warning 並略過 replay，zone 層比較仍要完成。"""
    path = tmp_path / "2330.csv"
    _write_csv(path, bullish_trend_df(n=170))

    report = run_builder_sweep(
        csv_sources=[f"{path}:2330"],
        dataset_config=_dataset_config(),
        atr_width_multipliers=[1.0, 1.5],
        max_merge_width_multiples=[2.0],
        decision_replay=True,
        run_id="sr_sweep_replay_002",
    )

    assert report["decision_replay_enabled"] is False
    assert report["candidate_count"] == 2
    assert all(candidate["decision_outcomes"] is None for candidate in report["results"])
    assert any("decision replay skipped" in warning for warning in report["warnings"])


def test_best_sweep_entry_result_ignores_thin_samples():
    """樣本不足的 candidate 不能拿來挑參數。"""
    thin = {
        "run_id": "thin", "atr_width_multiplier": 1.0, "max_merge_width_multiple": 2.0,
        "decision_outcomes": {"by_final_entry_state": {
            "ENTRY_ALLOWED": {"rows_with_forward_return": 2, "average_forward_return": 0.9},
        }},
    }
    solid = {
        "run_id": "solid", "atr_width_multiplier": 1.5, "max_merge_width_multiple": 2.0,
        "decision_outcomes": {"by_final_entry_state": {
            "ENTRY_ALLOWED": {"rows_with_forward_return": 20, "average_forward_return": 0.02},
        }},
    }

    assert evaluation_module._best_sweep_entry_result([thin]) is None

    best = evaluation_module._best_sweep_entry_result([thin, solid])
    assert best["run_id"] == "solid"
    assert best["rows"] == 20
    assert best["minimum_rows"] == evaluation_module.MIN_ENTRY_OUTCOME_ROWS


def test_sweep_entry_performance_excludes_wait_states():
    """WAIT_CONFIRMATION 的後續報酬不代表這組參數的進場品質，不能計入。"""
    result = {"decision_outcomes": {"by_final_entry_state": {
        "ENTRY_ALLOWED": {"rows_with_forward_return": 10, "average_forward_return": 0.01},
        "WAIT_CONFIRMATION": {"rows_with_forward_return": 90, "average_forward_return": 0.50},
    }}}

    performance = evaluation_module._sweep_entry_performance(result)

    assert performance["rows"] == 10
    assert performance["average_forward_return"] == pytest.approx(0.01)


def test_calibration_uses_binned_rows_as_denominator():
    """落不進任何 bin 的機率（例如 NaN）不能讓 ECE 變成 0.0 而讀起來像完美校準。"""
    y = np.array([1, 0, 1, 0])
    proba = np.array([np.nan, np.nan, np.nan, np.nan])

    result = evaluation_module._calibration_bins(y, proba)

    assert result["rows"] == 4
    assert result["binned_rows"] == 0
    assert result["expected_calibration_error"] is None
    assert result["max_calibration_error"] is None


# ── AT_ZONE 比例（T-003 P1 的第 5 個比較面向） ─────────────────────────────


def test_outcome_summary_reports_primary_zone_roles_and_at_zone_rate():
    """AT_ZONE 比例只有 replay 路徑量得到（dataset 的 role 永遠是 SUPPORT/RESISTANCE）。"""
    rows = [
        {"symbol": "2330", "primary_zone": {"role": "AT_ZONE"}},
        {"symbol": "2330", "primary_zone": {"role": "AT_ZONE"}},
        {"symbol": "2330", "primary_zone": {"role": "SUPPORT"}},
        {"symbol": "2330", "primary_zone": {"role": "RESISTANCE"}},
        # 沒有 primary zone 的列不能算進分母。
        {"symbol": "2330", "primary_zone": None},
    ]

    summary = evaluation_module._decision_replay_outcome_summary(rows)

    assert summary["primary_zone_role_counts"] == {"AT_ZONE": 2, "RESISTANCE": 1, "SUPPORT": 1}
    assert summary["rows_with_primary_zone"] == 4
    assert summary["at_zone_rate"] == pytest.approx(0.5)


def test_outcome_summary_at_zone_rate_is_none_without_primary_zone():
    rows = [{"symbol": "2330", "primary_zone": None}]

    summary = evaluation_module._decision_replay_outcome_summary(rows)

    assert summary["rows_with_primary_zone"] == 0
    assert summary["at_zone_rate"] is None
    assert summary["primary_zone_role_counts"] == {}


def test_sweep_decision_outcomes_expose_at_zone_rate(tmp_path, monkeypatch):
    """sweep 的 candidate 摘要要帶出 AT_ZONE 比例，才能拿來比較不同 ATR 寬度。"""
    path = tmp_path / "2330.csv"
    _write_csv(path, bullish_trend_df(n=170))
    bundle = ModelBundle(
        hold_model=_FixedProbabilityModel(0.62),
        break_model=_FixedProbabilityModel(0.28),
        feature_names=[],
        trained_at="2026-07-01T00:00:00+00:00",
        version="v-test",
        config_hash="hash123",
    )
    monkeypatch.setattr(evaluation_module, "load_model", lambda _: bundle)

    report = run_builder_sweep(
        csv_sources=[f"{path}:2330"],
        dataset_config=_dataset_config(),
        model_path=str(tmp_path / "model.joblib"),
        atr_width_multipliers=[1.0, 2.0],
        max_merge_width_multiples=[2.0],
        decision_replay=True,
        replay_max_rows=6,
        run_id="sr_sweep_at_zone_001",
    )

    for candidate in report["results"]:
        decision = candidate["decision_outcomes"]
        assert "at_zone_rate" in decision
        assert "primary_zone_role_counts" in decision
        if decision["at_zone_rate"] is not None:
            assert 0.0 <= decision["at_zone_rate"] <= 1.0


# ── RR distribution 與更細分層（2026-08-07 補）─────────────────────────


def test_metric_distribution_values_are_correct():
    """percentiles 要驗實際數值，不是只驗 key 存在。

    用一組已知序列：1..10 的中位數是 5.5、p25=3.25、p75=7.75（numpy 線性插值）。
    只斷言「key 存在」的測試擋不住把 p25 和 p75 寫反這種錯。
    """
    dist = evaluation_module._metric_distribution([float(i) for i in range(1, 11)])

    assert dist["count"] == 10
    assert dist["min"] == pytest.approx(1.0)
    assert dist["max"] == pytest.approx(10.0)
    assert dist["median"] == pytest.approx(5.5)
    assert dist["average"] == pytest.approx(5.5)
    assert dist["p25"] == pytest.approx(3.25)
    assert dist["p75"] == pytest.approx(7.75)
    assert dist["p10"] == pytest.approx(1.9)
    assert dist["p90"] == pytest.approx(9.1)
    # 分位數必須單調遞增，寫反或取錯會在這裡被抓到
    order = [dist["min"], dist["p10"], dist["p25"], dist["median"], dist["p75"], dist["p90"], dist["max"]]
    assert order == sorted(order)


def test_metric_distribution_edge_cases():
    # 空集合：count=0，其餘為 None 而不是 0——「沒有樣本」與「樣本值是 0」是兩件事。
    empty = evaluation_module._metric_distribution([])
    assert empty["count"] == 0
    assert all(empty[key] is None for key in ("average", "stddev", "min", "median", "max", "p10", "p90"))

    # 單一元素：所有分位數都等於該值，標準差為 0（不是 None）。
    single = evaluation_module._metric_distribution([3.5])
    assert single["count"] == 1
    assert single["stddev"] == pytest.approx(0.0)
    for key in ("average", "min", "p10", "p25", "median", "p75", "p90", "max"):
        assert single[key] == pytest.approx(3.5), key


def test_rr_summary_covers_execution_rr_and_distributions():
    """execution_rr 先前完全沒有被統計，但它參與 rr_gate 判斷。"""
    rows = [
        {"rr_context": {"entry_rr": 1.0, "execution_rr": 0.5, "entry_rr_source": "PRIMARY_ZONE",
                        "execution_rr_source": "PRIMARY_ZONE"}},
        {"rr_context": {"entry_rr": 3.0, "execution_rr": 2.5, "entry_rr_source": "PRIMARY_ZONE",
                        "execution_rr_source": "PRIMARY_ZONE"}},
        {"rr_context": {"entry_rr": None, "execution_rr": None, "entry_rr_source": "UNAVAILABLE",
                        "execution_rr_source": "UNAVAILABLE"}},
    ]

    summary = evaluation_module._rr_summary(rows)

    # 既有欄位不能因為新增而改變語意
    assert summary["rows_with_entry_rr"] == 2
    assert summary["median_entry_rr"] == pytest.approx(2.0)
    assert summary["entry_rr_source_counts"] == {"PRIMARY_ZONE": 2, "UNAVAILABLE": 1}

    # 新增的 execution RR
    assert summary["rows_with_execution_rr"] == 2
    assert summary["median_execution_rr"] == pytest.approx(1.5)
    assert summary["execution_rr_source_counts"] == {"PRIMARY_ZONE": 2, "UNAVAILABLE": 1}

    # 分布的 median 必須與既有的 median_* 一致，否則兩個數字會在前端互相矛盾
    assert summary["entry_rr_distribution"]["median"] == pytest.approx(summary["median_entry_rr"])
    assert summary["execution_rr_distribution"]["median"] == pytest.approx(summary["median_execution_rr"])
    assert summary["entry_rr_distribution"]["count"] == summary["rows_with_entry_rr"]
    # 全部無值時是空分布，不是 0
    assert summary["position_rr_distribution"]["count"] == 0
    assert summary["position_rr_distribution"]["median"] is None


def test_volume_strength_bucket_reuses_existing_thresholds():
    """邊界必須跟 scoring/_volume_confirmation 與 event_engine 的門檻一致。

    分層若自訂邊界，同一筆資料在「分類」與「分層」會講出不一致的故事。
    """
    assert evaluation_module._volume_strength_bucket(None) == "VOLUME_UNAVAILABLE"
    assert evaluation_module._volume_strength_bucket(0.79) == "VOL_LT_0_8"
    # 邊界值屬於上一桶（>= 才進）
    assert evaluation_module._volume_strength_bucket(0.8) == "VOL_0_8_TO_1_2"
    assert evaluation_module._volume_strength_bucket(1.19) == "VOL_0_8_TO_1_2"
    assert evaluation_module._volume_strength_bucket(1.2) == "VOL_1_2_TO_2_5"
    assert evaluation_module._volume_strength_bucket(2.49) == "VOL_1_2_TO_2_5"
    assert evaluation_module._volume_strength_bucket(2.5) == "VOL_GTE_2_5"


def test_stop_distance_and_event_buckets():
    assert evaluation_module._stop_distance_bucket(None) == "STOP_DISTANCE_UNAVAILABLE"
    # 傳入的是比例（0.006 = 0.6%），不是百分比數值——搞混會讓所有列擠進同一桶
    assert evaluation_module._stop_distance_bucket(0.006) == "SD_LT_1PCT"
    assert evaluation_module._stop_distance_bucket(0.01) == "SD_1_TO_3PCT"
    assert evaluation_module._stop_distance_bucket(0.05) == "SD_3_TO_6PCT"
    assert evaluation_module._stop_distance_bucket(0.09) == "SD_6_TO_10PCT"
    assert evaluation_module._stop_distance_bucket(0.10) == "SD_GTE_10PCT"

    # 代表事件取的是傳入 list 的第一個——注意這個 helper 本身不排序，
    # 排序是 decision_engine._event_sequence() 做的（固定優先序，非時間序）。
    assert evaluation_module._priority_event_type([]) == "NO_EVENT"
    assert evaluation_module._priority_event_type(
        [{"type": "INTRADAY_RECLAIM"}, {"type": "EXTREME_VOLUME"}]
    ) == "INTRADAY_RECLAIM"
    # 沒有 type 的項目要跳過而不是當成事件
    assert evaluation_module._priority_event_type([{"other": 1}, {"type": "EXTREME_VOLUME"}]) == "EXTREME_VOLUME"

    assert evaluation_module._event_count_bucket([]) == "EVENTS_0"
    assert evaluation_module._event_count_bucket([{"type": "A"}]) == "EVENTS_1"
    assert evaluation_module._event_count_bucket([{"type": "A"}, {"type": "B"}]) == "EVENTS_2"
    assert evaluation_module._event_count_bucket(
        [{"type": "A"}, {"type": "B"}, {"type": "C"}, {"type": "D"}]
    ) == "EVENTS_3_PLUS"


def test_daily_confirmation_summary_includes_new_groups():
    """新分層要真的掛上 summary，而不是只算出來沒輸出。"""
    rows = [
        {
            "daily_confirmation_outcome": {
                "available": True, "state": "BLOCKED", "primary_role": "SUPPORT",
                "next_zone_result": "SUPPORT_HELD", "two_bar_result": "SUPPORT_CONFIRMED",
                "next_close_return": 0.01, "two_bar_close_return": 0.02,
            },
            "daily_confirmation_context": {
                "volume_strength": "VOL_GTE_2_5",
                "stop_distance_bucket": "SD_1_TO_3PCT",
                "entry_executability": "EXECUTABLE_NOW",
                "primary_market_event": "EXTREME_VOLUME",
                "market_event_count": "EVENTS_2",
            },
        }
    ]

    summary = evaluation_module._daily_confirmation_summary(rows)

    for group, key in (
        ("by_volume_strength", "VOL_GTE_2_5"),
        ("by_stop_distance_bucket", "SD_1_TO_3PCT"),
        ("by_entry_executability", "EXECUTABLE_NOW"),
        ("by_primary_market_event", "EXTREME_VOLUME"),
        ("by_market_event_count", "EVENTS_2"),
    ):
        assert group in summary, group
        assert key in summary[group], (group, key)
        assert summary[group][key]["rows"] == 1


def test_primary_market_event_is_priority_order_not_time_order():
    """鎖住 `primary_market_event` 的真實語意：固定優先序，不是時間序。

    先前這個欄位叫 `first_market_event`，程式註解與文件都寫「先發生什麼」——那是錯的
    （現況見 docs/sr-zone-scoring.md 的「再細的六個分層」）。
    `decision_engine._event_sequence()` 用固定優先序排序
    （EXTREME_VOLUME 10 → HIGH_VOLUME_BREAKDOWN 20 → INTRADAY_RECLAIM 30 →
    REVERSAL_CANDIDATE 40），而且同一列的事件全來自同一根 K 棒，根本沒有時間先後。

    這支測試刻意走**真實路徑**（decision engine 的排序 → context），而不是只餵手寫 list
    給 helper——原本的測試就是因為只測 helper，完全沒碰到排序語意才漏掉。
    """
    from ..decision_engine import _event_sequence

    # 故意用「優先序低的排在前面」的輸入：若語意真的是時間序，結果會是 INTRADAY_RECLAIM。
    market_events = [
        {"type": "INTRADAY_RECLAIM"},
        {"type": "EXTREME_VOLUME"},
    ]
    sequence = _event_sequence(market_events)
    assert [event["type"] for event in sequence] == ["EXTREME_VOLUME", "INTRADAY_RECLAIM"]

    context = evaluation_module._daily_confirmation_context(
        {"volume_confirmation": "NEUTRAL", "relative_volume": 1.0},
        {"event_sequence": sequence, "market_events": market_events, "rr_context": {}, "rr_gate": {}},
    )
    # 優先序最高的 EXTREME_VOLUME 勝出，即使它在輸入 list 裡排在後面
    assert context["primary_market_event"] == "EXTREME_VOLUME"

    # 而且它是事件類型集合的確定性函數——換順序輸入結果不變，
    # 這正是「它是 market_event_types 的粗化、不是新維度」的證明。
    reversed_sequence = _event_sequence(list(reversed(market_events)))
    reversed_context = evaluation_module._daily_confirmation_context(
        {"volume_confirmation": "NEUTRAL", "relative_volume": 1.0},
        {"event_sequence": reversed_sequence, "market_events": market_events, "rr_context": {}, "rr_gate": {}},
    )
    assert reversed_context["primary_market_event"] == context["primary_market_event"]


def test_rr_formula_state_buckets_are_all_reachable():
    """四個桶都要對應真實可能發生的上游狀態。

    只有 stop_distance 與 entry_executability 兩個分層時，RR_UNAVAILABLE
    （真實資料上佔 62%）完全拆不開，看不出是缺目標價還是缺停損。
    """
    state = evaluation_module._rr_formula_state
    assert state({"risk_price": 5.0, "reward_price": 12.0}) == "RR_FORMULA_COMPLETE"
    assert state({"risk_price": 5.0, "reward_price": None}) == "REWARD_MISSING"
    # entry 與 stop 都有、但 risk 不是正數（停損在進場之上）——先前這種列被靜靜併進
    # 「兩邊都缺」，與文件寫的「通常是沒有 primary zone」不符。
    assert state({"entry_price": 100.0, "stop_price": 105.0}) == "RISK_NOT_POSITIVE"
    assert state({"entry_price": 100.0, "stop_price": 100.0}) == "RISK_NOT_POSITIVE"
    # 連 entry / stop 都沒有
    assert state({"entry_price": 100.0}) == "ENTRY_OR_STOP_MISSING"
    assert state({}) == "ENTRY_OR_STOP_MISSING"


def test_rr_formula_state_has_no_unreachable_bucket():
    """鎖住「reward 有值必然蘊含 risk 有值」這個上游不變式。

    `decision_engine._rr_context()` 的 `reward_price` 只在 `if risk > 0:` 內賦值，
    所以「只有 reward、沒有 risk」不可能發生。先前的實作為此開了一個
    `RISK_MISSING` 桶，真實資料跑出 0 筆卻被當成「風險側從不缺」的結論依據。
    這支測試直接對**真實生產者**驗這個不變式——只測 helper 對手寫 dict 的行為抓不到。
    """
    import inspect

    from .. import decision_engine

    source = inspect.getsource(decision_engine._rr_context)
    risk_assign = source.index("risk_price = risk")
    # reward_price 的每一次賦值都必須出現在 risk_price = risk 之後（同一個 if risk > 0 區塊內）
    reward_assigns = [
        idx for idx in range(len(source))
        if source.startswith("reward_price = ", idx) and not source[:idx].rstrip().endswith("None")
    ]
    assert reward_assigns, "找不到 reward_price 的賦值，_rr_context 結構已變，請重新確認分桶語意"
    assert all(idx > risk_assign for idx in reward_assigns), (
        "reward_price 出現在 risk_price 之前——上游不變式已改變，"
        "_rr_formula_state 的分桶要重新設計（可能需要重新加回 RISK_MISSING）"
    )


def test_daily_confirmation_context_emits_rr_formula_state():
    """要走 context 這條真實路徑，不是只測 helper。"""
    context = evaluation_module._daily_confirmation_context(
        {"volume_confirmation": "NEUTRAL", "relative_volume": 1.0},
        {
            "event_sequence": [],
            "market_events": [],
            "rr_context": {"risk_price": 5.0, "reward_price": None, "stop_distance_pct": 0.02},
            "rr_gate": {},
        },
    )
    assert context["rr_formula_state"] == "REWARD_MISSING"

    summary = evaluation_module._daily_confirmation_summary([
        {
            "daily_confirmation_outcome": {
                "available": True, "state": "BLOCKED", "primary_role": "SUPPORT",
                "next_zone_result": "SUPPORT_HELD", "two_bar_result": "SUPPORT_CONFIRMED",
                "next_close_return": 0.01, "two_bar_close_return": 0.02,
            },
            "daily_confirmation_context": context,
        }
    ])
    assert summary["by_rr_formula_state"]["REWARD_MISSING"]["rows"] == 1
