"""
市場結構趨勢判斷，被 support_resistance 與 entries 元件共用。

與 Go signal/trend.go、backtest/strategy/breakout_v1.py 的 _detect_trend 邏輯一致：
    Swing High/Low（window=1，左右各一根確認）→ 比較最近兩個 swing 點
    HH + HL → BULLISH；LH + LL → BEARISH；其餘 → SIDEWAYS
"""
from __future__ import annotations

from enum import Enum

import numpy as np
import pandas as pd


class Trend(str, Enum):
    BULLISH = "BULLISH"
    BEARISH = "BEARISH"
    SIDEWAYS = "SIDEWAYS"


def find_swing_highs(highs: np.ndarray, left: int = 1, right: int = 1) -> list[tuple[int, float]]:
    """回傳 [(index, price), ...]，index 為在傳入陣列中的位置。"""
    points: list[tuple[int, float]] = []
    n = len(highs)
    for i in range(left, n - right):
        window_left = highs[i - left:i]
        window_right = highs[i + 1:i + 1 + right]
        if highs[i] > window_left.max() and highs[i] > window_right.max():
            points.append((i, float(highs[i])))
    return points


def find_swing_lows(lows: np.ndarray, left: int = 1, right: int = 1) -> list[tuple[int, float]]:
    points: list[tuple[int, float]] = []
    n = len(lows)
    for i in range(left, n - right):
        window_left = lows[i - left:i]
        window_right = lows[i + 1:i + 1 + right]
        if lows[i] < window_left.min() and lows[i] < window_right.min():
            points.append((i, float(lows[i])))
    return points


def detect_trend(df: pd.DataFrame, left: int = 1, right: int = 1) -> Trend:
    """df 需按時間升冪排列，至少要有 high/low 欄位。"""
    if len(df) < 10:
        return Trend.SIDEWAYS

    highs = find_swing_highs(df["high"].to_numpy(), left, right)
    lows = find_swing_lows(df["low"].to_numpy(), left, right)
    if len(highs) < 2 or len(lows) < 2:
        return Trend.SIDEWAYS

    h1, h2 = highs[-1][1], highs[-2][1]
    l1, l2 = lows[-1][1], lows[-2][1]

    if h1 > h2 and l1 > l2:
        return Trend.BULLISH
    if h1 < h2 and l1 < l2:
        return Trend.BEARISH
    return Trend.SIDEWAYS
