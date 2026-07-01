"""
Volume Profile 支撐壓力偵測（Point of Control / Value Area）。

數學定義：
    1. 取最近 lookback 根K棒，將區間 [min(low), max(high)] 均分成 num_bins 個價格區間。
    2. 每根K棒的成交量以其 typical price = (H+L+C)/3 歸入對應的 bin
       （簡化假設：沒有 tick 資料可用來做真正的日內成交量分布，
       用單一代表價格近似整根K棒的成交量落點）。
    3. POC（Point of Control）= 成交量最大的 bin 中心價，strength = 1.0。
    4. Value Area = 以 POC 為中心，往上下擴張至累積成交量達
       value_area_pct（預設 70%）時的區間；其上緣 VAH、下緣 VAL 為次要 level，
       strength = 該 bin 占比。
    5. 依 level 價格與目前收盤價的相對位置分類：低於現價 → Support，
       高於現價 → Resistance（同一個 volume node 在價格穿越後，支撐/壓力
       角色本來就會互換，這是量價分析的標準用法）。
"""
from __future__ import annotations

import numpy as np
import pandas as pd

from ..types import Level, LevelType, SRLevels
from .base import SupportResistanceStrategy


class VolumeProfileSR(SupportResistanceStrategy):
    def __init__(
        self,
        lookback: int = 60,
        num_bins: int = 24,
        value_area_pct: float = 0.7,
    ) -> None:
        self.lookback = lookback
        self.num_bins = num_bins
        self.value_area_pct = value_area_pct

    @property
    def min_bars(self) -> int:
        return 10

    def calculate(self, df: pd.DataFrame) -> SRLevels:
        if len(df) < self.min_bars:
            return SRLevels()

        window = df.iloc[-self.lookback:] if len(df) > self.lookback else df
        highs = window["high"].to_numpy()
        lows = window["low"].to_numpy()
        closes = window["close"].to_numpy()
        volumes = window["volume"].to_numpy(dtype=float)

        typical = (highs + lows + closes) / 3.0
        price_min, price_max = float(lows.min()), float(highs.max())
        if price_max <= price_min or volumes.sum() <= 0:
            return SRLevels()

        bin_edges = np.linspace(price_min, price_max, self.num_bins + 1)
        bin_idx = np.clip(np.digitize(typical, bin_edges) - 1, 0, self.num_bins - 1)

        bin_volume = np.zeros(self.num_bins)
        for idx, vol in zip(bin_idx, volumes):
            bin_volume[idx] += vol

        bin_centers = (bin_edges[:-1] + bin_edges[1:]) / 2.0
        total_volume = bin_volume.sum()

        poc_bin = int(np.argmax(bin_volume))
        value_area_bins = self._expand_value_area(bin_volume, poc_bin, total_volume)

        current_price = float(closes[-1])
        levels: list[Level] = []

        def classify(price: float, strength: float, method: str) -> Level:
            level_type = LevelType.SUPPORT if price < current_price else LevelType.RESISTANCE
            return Level(price=price, type=level_type, strength=strength, method=method)

        levels.append(classify(float(bin_centers[poc_bin]), 1.0, "volume_profile_poc"))

        vah_bin = max(value_area_bins)
        val_bin = min(value_area_bins)
        if vah_bin != poc_bin:
            levels.append(classify(float(bin_centers[vah_bin]), bin_volume[vah_bin] / bin_volume[poc_bin], "volume_profile_vah"))
        if val_bin != poc_bin:
            levels.append(classify(float(bin_centers[val_bin]), bin_volume[val_bin] / bin_volume[poc_bin], "volume_profile_val"))

        supports = sorted([lv for lv in levels if lv.type == LevelType.SUPPORT], key=lambda lv: lv.strength, reverse=True)
        resistances = sorted([lv for lv in levels if lv.type == LevelType.RESISTANCE], key=lambda lv: lv.strength, reverse=True)
        return SRLevels(supports=supports, resistances=resistances)

    def _expand_value_area(self, bin_volume: np.ndarray, poc_bin: int, total_volume: float) -> set[int]:
        """從 POC 往左右擴張，每次納入量能較大的相鄰 bin，直到達到 value_area_pct。"""
        included = {poc_bin}
        cumulative = bin_volume[poc_bin]
        low, high = poc_bin, poc_bin
        n = len(bin_volume)

        while cumulative / total_volume < self.value_area_pct and (low > 0 or high < n - 1):
            left_vol = bin_volume[low - 1] if low > 0 else -1.0
            right_vol = bin_volume[high + 1] if high < n - 1 else -1.0

            if right_vol >= left_vol:
                high += 1
                cumulative += bin_volume[high]
                included.add(high)
            else:
                low -= 1
                cumulative += bin_volume[low]
                included.add(low)

        return included
