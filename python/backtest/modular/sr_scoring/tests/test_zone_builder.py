from __future__ import annotations

import numpy as np
import pytest

from ....indicators import calc_atr
from ..types import ZoneMethod
from ..zone_builder import (
    ATRZoneBuilder,
    ATRZoneBuilderConfig,
    RecentMicrostructureZoneBuilder,
    VolumeProfileZoneBuilder,
    ZoneBuilderConfig,
    _merge_zone_candidates,
    build_zone_builders,
    resolve_zone_builder_config_for_profile,
    volatility_bucket_from_profile,
    zone_builder_config_snapshot,
)
from .conftest import bullish_trend_df, make_df


def test_atr_zone_width_matches_atr_multiplier():
    # 兩個孤立的單根尖峰/尖谷，彼此相距夠遠，確保候選區間不會被合併，
    # 才能驗證「未合併」的 zone 寬度就是 atr_width_multiplier * ATR。
    n = 60
    closes = np.full(n, 100.0)
    closes[20] = 110.0
    closes[40] = 90.0
    highs = closes + 0.3
    lows = closes - 0.3
    opens = closes - 0.05
    volumes = np.full(n, 1000.0)
    df = make_df(list(zip(opens, highs, lows, closes, volumes)))

    builder = ATRZoneBuilder(lookback=60, atr_period=14, atr_width_multiplier=1.5, merge_pct=0.0, max_zones_per_type=10)
    zones = builder.build(df)
    assert zones

    atr = calc_atr(df["high"].to_numpy(), df["low"].to_numpy(), df["close"].to_numpy(), 14)
    expected_width = 1.5 * atr
    for zone in zones:
        assert zone.method == ZoneMethod.ATR
        assert zone.width == pytest.approx(expected_width, rel=1e-6)


def test_build_zone_builders_uses_shared_config_and_runtime_microstructure_toggle():
    config = ZoneBuilderConfig(atr=ATRZoneBuilderConfig(atr_width_multiplier=1.25, max_merge_width_multiple=1.5))

    training_builders = build_zone_builders(config, include_recent_microstructure=False)
    runtime_builders = build_zone_builders(config, include_recent_microstructure=True)

    assert [type(builder).__name__ for builder in training_builders] == [
        "ATRZoneBuilder", "VolumeProfileZoneBuilder",
    ]
    assert isinstance(runtime_builders[-1], RecentMicrostructureZoneBuilder)
    assert training_builders[0].atr_width_multiplier == pytest.approx(1.25)
    assert training_builders[0].max_merge_width_multiple == pytest.approx(1.5)
    assert zone_builder_config_snapshot(config)["ATRZoneBuilder"]["atr_width_multiplier"] == pytest.approx(1.25)


def test_resolve_zone_builder_config_for_profile_uses_volatility_bucket_configs():
    # 取值對照 2026-08-17 重定後的門檻（LOW < 4.61%、HIGH > 6.28%），
    # 而不是舊的 1.5% / 3.5%。門檻是凍結的全市場分位數，見 zone_builder 的
    # VOLATILITY_THRESHOLD_PROVENANCE。
    assert volatility_bucket_from_profile(0.030, 0.028) == "LOW_VOLATILITY"
    assert volatility_bucket_from_profile(0.055, 0.050) == "NORMAL_VOLATILITY"
    assert volatility_bucket_from_profile(0.080, 0.070) == "HIGH_VOLATILITY"
    assert volatility_bucket_from_profile(None, None) == "UNKNOWN_VOLATILITY"

    # 基準是 max(atr_pct, average_range_pct)——第二個參數較大時由它決定 bucket
    assert volatility_bucket_from_profile(0.030, 0.070) == "HIGH_VOLATILITY"

    low_config, low_meta = resolve_zone_builder_config_for_profile(0.030, 0.028)
    high_config, high_meta = resolve_zone_builder_config_for_profile(0.080, 0.070)
    unknown_config, unknown_meta = resolve_zone_builder_config_for_profile(None, None)

    assert low_meta["enabled"] is True
    assert low_meta["bucket"] == "LOW_VOLATILITY"
    assert low_config.atr.atr_width_multiplier == pytest.approx(1.25)
    assert low_config.atr.max_merge_width_multiple == pytest.approx(1.75)
    assert high_meta["enabled"] is True
    assert high_meta["bucket"] == "HIGH_VOLATILITY"
    assert high_config.atr.atr_width_multiplier == pytest.approx(1.75)
    assert high_config.atr.max_merge_width_multiple == pytest.approx(2.25)
    assert unknown_meta["enabled"] is False
    assert unknown_meta["reason_code"] == "UNKNOWN_VOLATILITY_BUCKET"
    assert unknown_config.atr.atr_width_multiplier == pytest.approx(1.5)


def test_atr_zone_builder_respects_min_bars():
    df = bullish_trend_df(n=5)
    builder = ATRZoneBuilder()
    assert builder.build(df) == []


def test_atr_zone_max_zones_per_type_caps_output():
    df = bullish_trend_df(n=80)
    builder = ATRZoneBuilder(max_zones_per_type=1)
    zones = builder.build(df)
    # 最多 1 個來自 pivot-high 候選池、1 個來自 pivot-low 候選池
    assert len(zones) <= 2


# ── 合併演算法：避免 single-linkage chaining ─────────────────────────────
#
# 修正前的 bug：_merge_zone_candidates 只比較新候選跟「目前已經合併到多大」
# 的邊界夠不夠近，一串前後緊鄰的候選（例如一串相鄰的 swing pivot）會像滾
# 雪球一樣被吃成一個涵蓋過大範圍的區間——本該是彼此獨立的關鍵價位（例如
# 40.0／40.7／42.3 三個壓力），結果全部合併成單一個模糊區間，還可能把現價
# 一起吃進去，讓角色被誤判成 AT_ZONE，損失方向性的機率/EV/RR 輸出。


def test_merge_zone_candidates_caps_width_to_avoid_chaining():
    # 一串前後緊鄰、寬度 3 的候選（間距只有 1，會兩兩重疊觸發合併）：舊版
    # 演算法會一路吃成一個涵蓋整個範圍（6 元寬以上）的巨大區間，新版應該
    # 在合併寬度超過上限（2 倍原始寬度=6）後停止繼續吃，拆成多段。
    width = 3.0
    centers = [100.0, 101.0, 102.0, 103.0, 104.0, 105.0, 106.0]
    candidates = [(c - width / 2, c + width / 2, c, i) for i, c in enumerate(centers)]

    merged = _merge_zone_candidates(candidates, merge_pct=0.0, max_merge_width_multiple=2.0)

    max_allowed = 2.0 * width
    for lo, hi, _, _ in merged:
        assert hi - lo <= max_allowed + 1e-9

    # 範圍遠超過上限，不能全部被合併成一個
    assert len(merged) > 1

    # 合併後仍要完整覆蓋所有輸入候選的範圍（沒有遺漏候選）
    total_lo = min(c[0] for c in candidates)
    total_hi = max(c[1] for c in candidates)
    assert min(m[0] for m in merged) == pytest.approx(total_lo)
    assert max(m[1] for m in merged) == pytest.approx(total_hi)


def test_merge_zone_candidates_far_apart_groups_stay_separate():
    # 兩群候選，同一群內間距很小會合併，但兩群之間差距很大：不該因為鏈式
    # 合併而被合在一起（回歸「支撐/壓力被吃進同一個區間」的 bug）。
    width = 1.0
    near_group = [(c - width / 2, c + width / 2, c, i) for i, c in enumerate([100.0, 100.5, 101.0])]
    far_group = [
        (c - width / 2, c + width / 2, c, i) for i, c in enumerate([120.0, 120.5, 121.0], start=10)
    ]

    merged = _merge_zone_candidates(near_group + far_group, merge_pct=0.01, max_merge_width_multiple=2.0)

    assert len(merged) == 2
    for lo, hi, _, _ in merged:
        assert hi < 110.0 or lo > 110.0


def test_atr_zone_builder_does_not_produce_single_giant_zone_from_pivot_chain():
    """對照實際案例：一串間距小的 swing pivot 不該被合併成單一涵蓋現價前後
    9 元以上的巨大區間，否則現價會落在區間內被誤判成 AT_ZONE。"""
    n = 80
    x = np.arange(n)
    closes = 40.0 + 0.05 * x + 1.2 * np.sin(x / 2.0)
    highs = closes + 0.3
    lows = closes - 0.3
    opens = closes - 0.05
    volumes = np.full(n, 1000.0)
    df = make_df(list(zip(opens, highs, lows, closes, volumes)))

    builder = ATRZoneBuilder(
        lookback=60, atr_period=14, atr_width_multiplier=1.5, merge_pct=0.01, max_zones_per_type=10
    )
    zones = builder.build(df)
    assert zones

    atr = calc_atr(df["high"].to_numpy(), df["low"].to_numpy(), df["close"].to_numpy(), 14)
    max_allowed_width = builder.max_merge_width_multiple * builder.atr_width_multiplier * atr
    for zone in zones:
        assert zone.width <= max_allowed_width + 1e-6


def test_volume_profile_zone_covers_high_volume_price_range():
    n = 60
    closes = np.linspace(90.0, 110.0, n)
    highs = closes + 0.3
    lows = closes - 0.3
    opens = closes - 0.05
    volumes = np.where((closes >= 95.0) & (closes <= 97.0), 5000.0, 200.0)
    df = make_df(list(zip(opens, highs, lows, closes, volumes)))

    builder = VolumeProfileZoneBuilder(lookback=60, num_bins=20, high_volume_percentile=0.9, max_zones_per_type=5)
    zones = builder.build(df)

    assert zones
    assert all(z.method == ZoneMethod.VOLUME_PROFILE for z in zones)
    assert any(z.intersects(95.0, 97.0) for z in zones)


def test_volume_profile_zone_builder_respects_min_bars():
    df = make_df([(1.0, 1.0, 1.0, 1.0, 1.0)] * 5)
    builder = VolumeProfileZoneBuilder()
    assert builder.build(df) == []


def test_volume_profile_max_zones_per_type_caps_output():
    n = 60
    closes = np.linspace(90.0, 130.0, n)
    highs = closes + 0.3
    lows = closes - 0.3
    opens = closes - 0.05
    volumes = np.full(n, 1000.0)
    for center in (95.0, 105.0, 115.0, 125.0):
        volumes[np.abs(closes - center) < 1.0] = 5000.0
    df = make_df(list(zip(opens, highs, lows, closes, volumes)))

    builder = VolumeProfileZoneBuilder(lookback=60, num_bins=40, high_volume_percentile=0.85, max_zones_per_type=1)
    zones = builder.build(df)
    assert len(zones) <= 2  # 最多 1 個 below-current-price + 1 個 above-current-price
