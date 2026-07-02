from __future__ import annotations

import numpy as np
import pandas as pd
import pytest

from ..model import (
    FEATURE_COLUMNS,
    load_model,
    predict_break_probability,
    predict_hold_probability,
    save_model,
    train_model,
)
from ..types import ZoneFeatures


def synthetic_dataset(n: int = 200, seed: int = 0) -> pd.DataFrame:
    """合成一份帶有可學習訊號的資料集：hold_label 與 rejection 比率/正向報酬
    正相關，break_label 與 breakout_count 正相關、與正向報酬負相關。"""
    rng = np.random.default_rng(seed)
    touch_count = rng.integers(1, 10, n)
    rejection_count = np.array([rng.integers(0, tc + 1) for tc in touch_count])
    breakout_count = rng.integers(0, 3, n)
    avg_return = rng.normal(0, 0.02, n)
    relative_volume = rng.uniform(0.5, 3.0, n)
    volatility = rng.uniform(0.01, 0.05, n)
    trend_strength = rng.normal(0, 0.05, n)
    is_support = rng.integers(0, 2, n)

    rejection_ratio = rejection_count / np.maximum(touch_count, 1)
    hold_score = rejection_ratio + avg_return * 5 + rng.normal(0, 0.3, n)
    hold_label = (hold_score > np.median(hold_score)).astype(int)

    break_score = breakout_count - avg_return * 5 + rng.normal(0, 0.3, n)
    break_label = (break_score > np.median(break_score)).astype(int)

    return pd.DataFrame({
        "touch_count": touch_count,
        "rejection_count": rejection_count,
        "breakout_count": breakout_count,
        "avg_return_after_touch": avg_return,
        "relative_volume": relative_volume,
        "volatility": volatility,
        "trend_strength": trend_strength,
        "is_support": is_support,
        "hold_label": hold_label,
        "break_label": break_label,
    })


def test_train_model_produces_bundle_with_metrics():
    bundle = train_model(synthetic_dataset(), model_type="logistic_regression")

    assert bundle.feature_names == FEATURE_COLUMNS
    assert set(bundle.metrics) == {"hold", "break"}
    for metrics in bundle.metrics.values():
        assert 0.0 <= metrics["accuracy"] <= 1.0
        assert 0.0 <= metrics["precision"] <= 1.0
        assert 0.0 <= metrics["recall"] <= 1.0


def test_train_model_raises_when_too_few_rows():
    empty = pd.DataFrame({c: [] for c in FEATURE_COLUMNS + ["hold_label", "break_label"]})
    with pytest.raises(ValueError):
        train_model(empty)


def test_save_load_round_trip(tmp_path):
    bundle = train_model(synthetic_dataset(), model_type="logistic_regression")
    path = str(tmp_path / "nested" / "model.joblib")

    save_model(bundle, path)
    loaded = load_model(path)

    assert loaded.version == bundle.version
    assert loaded.feature_names == bundle.feature_names
    assert loaded.metrics == bundle.metrics


def test_predict_probabilities_are_in_unit_interval():
    bundle = train_model(synthetic_dataset(), model_type="gradient_boosting")
    features = ZoneFeatures(
        touch_count=4, rejection_count=3, breakout_count=0,
        avg_return_after_touch=0.02, relative_volume=1.5, volatility=0.02, trend_strength=0.01,
    )

    hold_p = predict_hold_probability(bundle, features, is_support=True)
    break_p = predict_break_probability(bundle, features, is_support=True)

    assert 0.0 <= hold_p <= 1.0
    assert 0.0 <= break_p <= 1.0
