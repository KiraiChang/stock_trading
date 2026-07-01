from __future__ import annotations

import numpy as np

from ..support_resistance.atr_channel import ATRChannelSR
from ..support_resistance.swing_points import SwingHighLowSR
from ..support_resistance.volume_profile import VolumeProfileSR
from ..types import LevelType
from .conftest import bullish_trend_df, make_df


def test_swing_high_low_detects_local_extremes():
    # W 型：兩個相近的低點中間夾一個高點，兩側各留緩衝避免邊界效應
    rows = [
        (100, 101, 99, 100, 1000),
        (100, 102, 100, 101, 1000),
        (101, 103, 100, 100, 1000),
        (100, 108, 100, 107, 1000),
        (107, 112, 106, 111, 1000),  # local high ~112
        (111, 111, 104, 105, 1000),
        (105, 106, 96, 100, 1000),  # local low ~96
        (100, 103, 100, 102, 1000),
        (102, 104, 101, 103, 1000),
        (103, 105, 102, 104, 1000),
    ]
    df = make_df(rows)
    sr = SwingHighLowSR(lookback=len(df), pivot_window=1, merge_pct=0.02, max_levels=3)
    levels = sr.calculate(df)

    assert levels.resistances, "應偵測到至少一個壓力位"
    assert any(abs(lv.price - 112) < 1 for lv in levels.resistances)
    assert levels.supports, "應偵測到至少一個支撐位"
    assert any(95 <= lv.price <= 101 for lv in levels.supports)
    assert all(lv.type == LevelType.RESISTANCE for lv in levels.resistances)
    assert all(lv.type == LevelType.SUPPORT for lv in levels.supports)


def test_swing_high_low_insufficient_data_returns_empty():
    df = make_df([(100, 101, 99, 100, 1000)] * 5)
    sr = SwingHighLowSR()
    levels = sr.calculate(df)
    assert levels.supports == []
    assert levels.resistances == []


def test_atr_channel_uses_rolling_high_low_as_levels():
    df = bullish_trend_df(n=30)
    sr = ATRChannelSR(lookback=20, atr_period=14, atr_multiplier=0.5)
    levels = sr.calculate(df)

    window = df.iloc[-20:]
    assert levels.resistances[0].price == float(window["high"].max())
    assert levels.supports[0].price == float(window["low"].min())
    # 上升趨勢中，近20根的最高價必然出現在窗口尾端附近，強度應該 > 0
    assert 0.0 <= levels.resistances[0].strength <= 1.0
    assert 0.0 <= levels.supports[0].strength <= 1.0


def test_atr_channel_insufficient_data_returns_empty():
    df = make_df([(100, 101, 99, 100, 1000)] * 5)
    sr = ATRChannelSR(atr_period=14)
    levels = sr.calculate(df)
    assert levels.supports == []
    assert levels.resistances == []


def test_volume_profile_poc_falls_in_high_volume_zone():
    # 前段資料量能平均分布，最後 5 根都在 110~111 附近且量能極大 → POC 應落在此區間
    n = 40
    closes = 100 + np.linspace(0, 5, n)
    highs = closes + 0.5
    lows = closes - 0.5
    opens = closes - 0.1
    volumes = np.full(n, 500.0)

    # 在尾端製造一個明顯的高量能區
    closes[-5:] = 110.5
    highs[-5:] = 111.0
    lows[-5:] = 110.0
    opens[-5:] = 110.2
    volumes[-5:] = 50_000.0

    df = make_df(list(zip(opens, highs, lows, closes, volumes)))
    sr = VolumeProfileSR(lookback=40, num_bins=20, value_area_pct=0.7)
    levels = sr.calculate(df)

    all_levels = levels.supports + levels.resistances
    assert all_levels, "應至少產生一個 level"
    poc = max(all_levels, key=lambda lv: lv.strength)
    assert 109.5 <= poc.price <= 111.5, f"POC 應落在高量能區間附近，實際 {poc.price}"


def test_volume_profile_insufficient_data_returns_empty():
    df = make_df([(100, 101, 99, 100, 1000)] * 5)
    sr = VolumeProfileSR()
    levels = sr.calculate(df)
    assert levels.supports == []
    assert levels.resistances == []
