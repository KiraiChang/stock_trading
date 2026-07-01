"""
Swing High / Swing Low 支撐壓力偵測。

數學定義：
    1. 在最近 lookback 根K棒內，找出所有 swing high / swing low
       （左右各 pivot_window 根皆較低/較高的局部極值）。
    2. 將距離在 merge_pct 以內的候選點合併成同一個 level（取平均價）。
    3. strength = 該 level 的觸碰次數 / 所有 level 中的最大觸碰次數（正規化到 0~1）。
    4. 依 strength 由高到低排序，取前 max_levels 個。

與 Go internal/signal/support_resistance.go、backtest/strategy/breakout_v1.py
的既有邏輯等價（pivot_window=1、merge_pct=1%、max_levels=3 為預設值，
與既有系統一致）。
"""
from __future__ import annotations

import numpy as np
import pandas as pd

from ..trend import find_swing_highs, find_swing_lows
from ..types import Level, LevelType, SRLevels
from .base import SupportResistanceStrategy


class SwingHighLowSR(SupportResistanceStrategy):
    def __init__(
        self,
        lookback: int = 60,
        pivot_window: int = 1,
        merge_pct: float = 0.01,
        max_levels: int = 3,
    ) -> None:
        self.lookback = lookback
        self.pivot_window = pivot_window
        self.merge_pct = merge_pct
        self.max_levels = max_levels

    @property
    def min_bars(self) -> int:
        return max(10, self.pivot_window * 2 + 2)

    def calculate(self, df: pd.DataFrame) -> SRLevels:
        if len(df) < self.min_bars:
            return SRLevels()

        window = df.iloc[-self.lookback:] if len(df) > self.lookback else df
        highs = window["high"].to_numpy()
        lows = window["low"].to_numpy()

        res_candidates = [p for _, p in find_swing_highs(highs, self.pivot_window, self.pivot_window)]
        sup_candidates = [p for _, p in find_swing_lows(lows, self.pivot_window, self.pivot_window)]

        resistances = self._build_levels(res_candidates, LevelType.RESISTANCE)
        supports = self._build_levels(sup_candidates, LevelType.SUPPORT)
        return SRLevels(supports=supports, resistances=resistances)

    def _build_levels(self, candidates: list[float], level_type: LevelType) -> list[Level]:
        if not candidates:
            return []

        clusters: list[list[float]] = [[candidates[0]]]
        for price in candidates[1:]:
            center = float(np.mean(clusters[-1]))
            if center != 0 and abs(price - center) / center < self.merge_pct:
                clusters[-1].append(price)
            else:
                clusters.append([price])

        max_count = max(len(c) for c in clusters)
        levels = [
            Level(
                price=float(np.mean(cluster)),
                type=level_type,
                strength=len(cluster) / max_count,
                method="swing",
            )
            for cluster in clusters
        ]
        levels.sort(key=lambda lv: lv.strength, reverse=True)
        return levels[: self.max_levels]
