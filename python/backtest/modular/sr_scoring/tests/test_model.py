from __future__ import annotations

import numpy as np
import pandas as pd
import pytest

from ..model import (
    FEATURE_COLUMNS,
    MIN_ROWS_FOR_CALIBRATION,
    _time_split_indices,
    compute_config_hash,
    load_model,
    predict_break_probability,
    predict_hold_probability,
    reward_risk_percentile,
    save_model,
    train_model,
)
from ..types import ZoneFeatures


def synthetic_dataset(n: int = 200, seed: int = 0, num_symbols: int = 4) -> pd.DataFrame:
    """合成一份帶有可學習訊號的資料集：hold_label 與 rejection 比率/平均反彈
    報酬正相關，break_label 與 breakout_count 正相關、與平均反彈報酬負相關。
    average_bounce_return 恆為正（或 0）、average_break_return 恆為負（或 0），
    對齊 features.py::average_bounce_break_returns 的實際輸出慣例。

    symbol/touch_time 是 train_model() 預設 split_method="time" 所需的欄位
    （見 _time_split_indices），這裡分成 num_symbols 檔「股票」、每檔各自
    有一段遞增的 touch_time，模擬真實 build_training_dataset() 的輸出形狀。"""
    rng = np.random.default_rng(seed)
    touch_count = rng.integers(1, 10, n)
    rejection_count = np.array([rng.integers(0, tc + 1) for tc in touch_count])
    breakout_count = rng.integers(0, 3, n)
    average_bounce_return = rng.uniform(0.0, 0.06, n)
    average_break_return = -rng.uniform(0.0, 0.06, n)
    relative_volume = rng.uniform(0.5, 3.0, n)
    volatility = rng.uniform(0.01, 0.05, n)
    trend_strength = rng.normal(0, 0.05, n)
    is_support = rng.integers(0, 2, n)
    chip_total_score = rng.normal(0, 35, n)
    chip_institutional_score = rng.normal(0, 30, n)
    chip_margin_score = rng.normal(0, 20, n)
    chip_broker_score = rng.normal(0, 20, n)
    chip_concentration_score = rng.normal(0, 15, n)
    chip_missing = np.zeros(n)

    rejection_ratio = rejection_count / np.maximum(touch_count, 1)
    hold_score = rejection_ratio + average_bounce_return * 5 + rng.normal(0, 0.3, n)
    hold_label = (hold_score > np.median(hold_score)).astype(int)

    break_score = breakout_count - average_break_return * 5 + rng.normal(0, 0.3, n)
    break_label = (break_score > np.median(break_score)).astype(int)

    symbols = [f"SYM{i % num_symbols}" for i in range(n)]
    touch_time = pd.date_range("2024-01-01", periods=n, freq="h", tz="UTC")

    return pd.DataFrame({
        "symbol": symbols,
        "touch_time": touch_time,
        "touch_count": touch_count,
        "rejection_count": rejection_count,
        "breakout_count": breakout_count,
        "average_bounce_return": average_bounce_return,
        "average_break_return": average_break_return,
        "relative_volume": relative_volume,
        "volatility": volatility,
        "trend_strength": trend_strength,
        "is_support": is_support,
        "chip_total_score": chip_total_score,
        "chip_institutional_score": chip_institutional_score,
        "chip_margin_score": chip_margin_score,
        "chip_broker_score": chip_broker_score,
        "chip_concentration_score": chip_concentration_score,
        "chip_missing": chip_missing,
        "hold_label": hold_label,
        "break_label": break_label,
    })


def test_train_model_produces_bundle_with_metrics():
    bundle = train_model(synthetic_dataset(), model_type="logistic_regression")

    assert bundle.feature_names == FEATURE_COLUMNS
    assert set(bundle.metrics) == {"hold", "break"}
    assert bundle.version == "v4"
    assert 1 <= len(bundle.explanation_background) <= 32
    assert len(bundle.explanation_background[0]) == len(FEATURE_COLUMNS)
    for metrics in bundle.metrics.values():
        assert 0.0 <= metrics["accuracy"] <= 1.0
        assert 0.0 <= metrics["precision"] <= 1.0
        assert 0.0 <= metrics["recall"] <= 1.0


def test_train_model_raises_when_too_few_rows():
    empty = pd.DataFrame({c: [] for c in FEATURE_COLUMNS + ["hold_label", "break_label"]})
    with pytest.raises(ValueError):
        train_model(empty)


def test_train_model_builds_rr_reference_distribution():
    bundle = train_model(synthetic_dataset(), model_type="logistic_regression")

    assert len(bundle.rr_reference) > 0
    assert bundle.rr_reference == sorted(bundle.rr_reference)
    assert all(v >= 0 for v in bundle.rr_reference)


def test_reward_risk_percentile_monotonic():
    bundle = train_model(synthetic_dataset(), model_type="logistic_regression")

    low = reward_risk_percentile(bundle, 0.0)
    high = reward_risk_percentile(bundle, 1e9)
    assert low is not None and high is not None
    assert low <= high
    assert 0.0 <= low <= 100.0
    assert 0.0 <= high <= 100.0


def test_reward_risk_percentile_none_when_no_reference():
    from ..model import ModelBundle

    empty_bundle = ModelBundle(
        hold_model=None, break_model=None, feature_names=FEATURE_COLUMNS,
        trained_at="2026-01-01T00:00:00+00:00", version="v3", rr_reference=[],
    )
    assert reward_risk_percentile(empty_bundle, 1.5) is None


def test_save_load_round_trip(tmp_path):
    bundle = train_model(synthetic_dataset(), model_type="logistic_regression")
    path = str(tmp_path / "nested" / "model.joblib")

    save_model(bundle, path)
    loaded = load_model(path)

    assert loaded.version == bundle.version
    assert loaded.feature_names == bundle.feature_names
    assert loaded.metrics == bundle.metrics
    assert loaded.rr_reference == bundle.rr_reference
    assert loaded.training_config == bundle.training_config
    assert loaded.config_hash == bundle.config_hash


def test_train_model_stores_training_config_and_hash():
    bundle = train_model(
        synthetic_dataset(), model_type="logistic_regression",
        training_config={"dataset_config": {"forward_bars_support": 5}},
    )

    assert bundle.training_config["dataset_config"] == {"forward_bars_support": 5}
    assert bundle.training_config["model_type"] == "logistic_regression"
    assert bundle.training_config["split_method"] == "time"
    assert bundle.config_hash == compute_config_hash(bundle.training_config)
    assert len(bundle.config_hash) == 12


def test_compute_config_hash_is_deterministic_regardless_of_key_order():
    assert compute_config_hash({"a": 1, "b": 2}) == compute_config_hash({"b": 2, "a": 1})


def test_compute_config_hash_differs_when_values_differ():
    assert compute_config_hash({"a": 1}) != compute_config_hash({"a": 2})


def test_train_model_supports_hist_gradient_boosting():
    bundle = train_model(synthetic_dataset(), model_type="hist_gradient_boosting")
    assert bundle.training_config["model_type"] == "hist_gradient_boosting"


def test_predict_probabilities_are_in_unit_interval():
    bundle = train_model(synthetic_dataset(), model_type="gradient_boosting")
    features = ZoneFeatures(
        touch_count=4, rejection_count=3, breakout_count=0,
        average_bounce_return=0.02, average_break_return=-0.01,
        relative_volume=1.5, volatility=0.02, trend_strength=0.01,
    )

    chip_features = {
        "chip_total_score": 30.0,
        "chip_institutional_score": 10.0,
        "chip_margin_score": -5.0,
        "chip_broker_score": 2.0,
        "chip_concentration_score": 3.0,
        "chip_missing": 0.0,
    }
    hold_p = predict_hold_probability(bundle, features, is_support=True, chip_features=chip_features)
    break_p = predict_break_probability(bundle, features, is_support=True, chip_features=chip_features)

    assert 0.0 <= hold_p <= 1.0
    assert 0.0 <= break_p <= 1.0


# ── 三、1：時間序列 holdout 切分（預設 split_method="time"）────────────


def test_time_split_indices_every_symbol_contributes_to_train_and_test():
    # 4 檔股票各 20 筆，test_size=0.2 → 每檔應該有 train 且有 test，不會
    # 出現「整批股票只在 train、另一批只在 test」的情況。
    df = synthetic_dataset(n=80, num_symbols=4)
    train_idx, test_idx = _time_split_indices(df, test_size=0.2)

    train_symbols = set(df.loc[train_idx, "symbol"])
    test_symbols = set(df.loc[test_idx, "symbol"])
    assert train_symbols == test_symbols == set(df["symbol"].unique())


def test_time_split_indices_test_is_chronologically_after_train_per_symbol():
    df = synthetic_dataset(n=80, num_symbols=4)
    train_idx, test_idx = _time_split_indices(df, test_size=0.25)

    for sym in df["symbol"].unique():
        train_times = df.loc[train_idx][df.loc[train_idx, "symbol"] == sym]["touch_time"]
        test_times = df.loc[test_idx][df.loc[test_idx, "symbol"] == sym]["touch_time"]
        if len(train_times) and len(test_times):
            assert train_times.max() < test_times.min()


def test_train_model_defaults_to_time_split():
    bundle = train_model(synthetic_dataset(), model_type="logistic_regression")
    assert bundle.split_method == "time"


def test_train_model_random_split_still_supported():
    bundle = train_model(synthetic_dataset(), model_type="logistic_regression", split_method="random")
    assert bundle.split_method == "random"


def test_train_model_time_split_requires_symbol_and_touch_time_columns():
    df = synthetic_dataset().drop(columns=["symbol", "touch_time"])
    with pytest.raises(ValueError):
        train_model(df, split_method="time")


def test_train_model_metrics_include_time_series_diagnostics():
    bundle = train_model(synthetic_dataset(), model_type="logistic_regression")

    for metrics in bundle.metrics.values():
        assert "positive_rate_train" in metrics
        assert "positive_rate_test" in metrics
        assert "brier_score" in metrics
        assert "log_loss" in metrics
        assert "calibrated" in metrics
        assert 0.0 <= metrics["brier_score"] <= 1.0
        assert metrics["log_loss"] >= 0.0


# ── 三、2：機率校準 ──────────────────────────────────────────────


def test_train_model_calibrates_when_enough_data():
    bundle = train_model(synthetic_dataset(n=200), model_type="logistic_regression")
    for metrics in bundle.metrics.values():
        assert metrics["calibrated"] == 1.0


def test_train_model_skips_calibration_when_too_few_rows():
    # 20 筆剛好過 train_model() 的最低門檻，但遠低於 MIN_ROWS_FOR_CALIBRATION，
    # 應該降級為不校準，而不是讓 CalibratedClassifierCV 在極小樣本上硬跑。
    small = synthetic_dataset(n=20, num_symbols=2)
    assert 20 < MIN_ROWS_FOR_CALIBRATION

    bundle = train_model(small, model_type="logistic_regression")
    for metrics in bundle.metrics.values():
        assert metrics["calibrated"] == 0.0


def test_train_model_calibration_can_be_disabled():
    bundle = train_model(synthetic_dataset(), model_type="logistic_regression", calibration_method="none")
    for metrics in bundle.metrics.values():
        assert metrics["calibrated"] == 0.0
