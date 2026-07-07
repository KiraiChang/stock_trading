from __future__ import annotations

import pandas as pd

from ..dataset import DatasetConfig, build_training_dataset, build_training_rows, summarize_training_dataset
from ..features import compute_zone_features
from ..types import ApproachDirection, Zone, ZoneType
from ..zone_builder import ATRZoneBuilder, VolumeProfileZoneBuilder
from .conftest import bullish_trend_df

_BUILDERS = [ATRZoneBuilder(), VolumeProfileZoneBuilder()]
_CONFIG = DatasetConfig(
    min_history_bars=60,
    rebuild_every_bars=5,
    forward_bars_support=3,
    forward_bars_resistance=3,
    threshold_pct_support=0.01,
    threshold_pct_resistance=0.01,
    zone_lookback_bars=60,
)


def test_build_training_rows_produces_monotonic_touch_times():
    df = bullish_trend_df(n=150)
    rows = build_training_rows(df, "TEST", "1d", _BUILDERS, _CONFIG)
    assert rows

    touch_indices = [df.index.get_loc(r.touch_time) for r in rows]
    assert touch_indices == sorted(touch_indices)


def test_build_training_rows_labels_never_use_bars_beyond_dataframe():
    df = bullish_trend_df(n=150)
    rows = build_training_rows(df, "TEST", "1d", _BUILDERS, _CONFIG)
    assert rows

    for r in rows:
        touch_idx = df.index.get_loc(r.touch_time)
        assert touch_idx + r.forward_bars < len(df)


def test_build_training_rows_features_match_truncated_recomputation():
    """no-lookahead 檢查：把 df 截到 touch 當下重算特徵，結果必須與訓練列
    儲存的完全一致 —— 代表 walk-forward 迴圈中的特徵計算沒有偷看未來資料。"""
    df = bullish_trend_df(n=150)
    rows = build_training_rows(df, "TEST", "1d", _BUILDERS, _CONFIG)
    assert rows

    for r in rows[:5]:
        touch_idx = df.index.get_loc(r.touch_time)
        truncated = df.iloc[: touch_idx + 1]
        approach = ApproachDirection.FROM_ABOVE if r.role == ZoneType.SUPPORT else ApproachDirection.FROM_BELOW
        zone = Zone(
            price_low=r.zone_price_low, price_high=r.zone_price_high, method=r.method,
            center_price=(r.zone_price_low + r.zone_price_high) / 2.0, formed_at_index=0,
        )
        recomputed = compute_zone_features(
            truncated, zone, as_of_index=len(truncated) - 1, approach=approach,
            lookback_bars=_CONFIG.zone_lookback_bars, forward_bars=r.forward_bars, threshold_pct=r.threshold_pct,
        )
        assert recomputed.touch_count == r.features.touch_count
        assert recomputed.rejection_count == r.features.rejection_count
        assert recomputed.breakout_count == r.features.breakout_count


def test_build_training_dataset_concatenates_multiple_sources():
    df1 = bullish_trend_df(n=150, base=100.0)
    df2 = bullish_trend_df(n=150, base=50.0)

    dataset = build_training_dataset([("A", "1d", df1), ("B", "1d", df2)], _BUILDERS, _CONFIG)

    assert not dataset.empty
    assert set(dataset["symbol"].unique()) <= {"A", "B"}
    assert "hold_label" in dataset.columns
    assert "chip_total_score" in dataset.columns
    assert "chip_missing" in dataset.columns
    assert "break_label" in dataset.columns
    assert "is_support" in dataset.columns


def test_build_training_dataset_adds_chip_features_without_lookahead():
    df = bullish_trend_df(n=150, base=100.0)
    chip_rows = [
        {
            "trade_date": "2024-01-01",
            "total_score": 10.0,
            "institutional_score": 1.0,
            "margin_score": 2.0,
            "broker_score": 3.0,
            "concentration_score": 4.0,
        },
        {
            "trade_date": "2099-01-01",
            "total_score": 99.0,
            "institutional_score": 99.0,
            "margin_score": 99.0,
            "broker_score": 99.0,
            "concentration_score": 99.0,
        },
    ]

    dataset = build_training_dataset(
        [("A", "1d", df)], _BUILDERS, _CONFIG, chip_scores_by_symbol={"A": chip_rows}
    )

    assert not dataset.empty
    assert "chip_total_score" in dataset.columns
    assert set(dataset["chip_total_score"].unique()) == {10.0}
    assert set(dataset["chip_missing"].unique()) == {0.0}


def test_build_training_dataset_marks_missing_chip_features():
    df = bullish_trend_df(n=150, base=100.0)
    dataset = build_training_dataset([("A", "1d", df)], _BUILDERS, _CONFIG)

    assert not dataset.empty
    assert set(dataset["chip_total_score"].unique()) == {0.0}
    assert set(dataset["chip_missing"].unique()) == {1.0}


def test_build_training_dataset_empty_still_has_columns():
    df = bullish_trend_df(n=90)
    tight_config = DatasetConfig(min_history_bars=89)  # 幾乎沒有 walk-forward 空間

    dataset = build_training_dataset([("A", "1d", df)], _BUILDERS, tight_config)

    assert len(dataset) == 0
    assert "hold_label" in dataset.columns
    assert "chip_total_score" in dataset.columns
    assert "chip_missing" in dataset.columns


# ── 三、3：訓練資料診斷報告 ──────────────────────────────────────


def test_summarize_training_dataset_empty():
    df = bullish_trend_df(n=90)
    tight_config = DatasetConfig(min_history_bars=89)
    dataset = build_training_dataset([("A", "1d", df)], _BUILDERS, tight_config)

    summary = summarize_training_dataset(dataset)

    assert summary["rows"] == 0
    assert summary["rows_by_symbol"] == {}
    assert summary["rr_reference_count"] == 0


def test_summarize_training_dataset_reports_per_symbol_and_role_breakdown():
    df1 = bullish_trend_df(n=150, base=100.0)
    df2 = bullish_trend_df(n=150, base=50.0)
    dataset = build_training_dataset([("A", "1d", df1), ("B", "1d", df2)], _BUILDERS, _CONFIG)
    assert not dataset.empty

    summary = summarize_training_dataset(dataset)

    assert summary["rows"] == len(dataset)
    assert set(summary["rows_by_symbol"]) <= {"A", "B"}
    assert sum(summary["rows_by_symbol"].values()) == len(dataset)
    assert 0.0 <= summary["hold_positive_rate"] <= 1.0
    assert 0.0 <= summary["break_positive_rate"] <= 1.0
    # is_support 的 zero rate 反映「壓力方向」觸碰的比例，也算進特徵缺值/為 0 的診斷
    assert "is_support" in summary["feature_zero_rate"]
    assert "touch_count" in summary["feature_zero_rate"]
