from __future__ import annotations

import numpy as np
import pytest

from ....indicators import calc_atr
from ..types import ZoneMethod
from ..zone_builder import ATRZoneBuilder, VolumeProfileZoneBuilder
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
