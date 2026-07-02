"""
Zone（價格區間）建立：至少支援 ATR-based 與 Volume Profile 兩種方法。

與 support_resistance/atr_channel.py、volume_profile.py 的差異：那兩個模組
回傳「單一價位 + 強度」（Level），這裡回傳「價格區間」（Zone），且區間邊界
本身就是輸出的一部分（供後續 touch/rejection/breakout 特徵計算使用），故
不直接沿用既有實作，只重用底層的 calc_atr 與 trend.find_swing_highs/lows。

角色（SUPPORT/RESISTANCE）不在建立階段決定：同一個 volume node 或 ATR 通道
在價格穿越後，支撐/壓力角色本來就會互換，實際角色由 scoring.py 依「當下
價格」動態判斷。這裡的 pivot-high/pivot-low、below/above-current-price 分類
僅是建立階段用來分別限制候選數量（max_zones_per_type）的內部機制。
"""
from __future__ import annotations

from abc import ABC, abstractmethod

import numpy as np
import pandas as pd

from ...indicators import calc_atr
from ..trend import find_swing_highs, find_swing_lows
from .types import Zone, ZoneMethod


class ZoneBuilder(ABC):
    """輸入 OHLCV DataFrame（時間升冪排列，只含至今的資料），輸出目前有效的價格區間。"""

    @abstractmethod
    def build(self, df: pd.DataFrame) -> list[Zone]:
        """df 只包含「至今」的資料（呼叫端負責切片避免 lookahead bias）。"""
        raise NotImplementedError

    @property
    @abstractmethod
    def min_bars(self) -> int:
        """build() 所需的最少K棒數，資料不足時呼叫端應跳過。"""
        raise NotImplementedError


def _merge_zone_candidates(
    candidates: list[tuple[float, float, float, int]], merge_pct: float
) -> list[tuple[float, float, float, int]]:
    """合併重疊或相近（間距 < merge_pct * price）的區間候選。

    candidates: [(price_low, price_high, center_price, formed_at_index), ...]
    合併後 center_price 取平均、formed_at_index 取最新（較貼近最近一次形成）。
    """
    if not candidates:
        return []
    ordered = sorted(candidates, key=lambda c: c[0])
    merged = [list(ordered[0])]
    for lo, hi, center, idx in ordered[1:]:
        last = merged[-1]
        gap_threshold = merge_pct * max(last[2], center)
        if lo <= last[1] + gap_threshold:
            last[0] = min(last[0], lo)
            last[1] = max(last[1], hi)
            last[2] = (last[2] + center) / 2.0
            last[3] = max(last[3], idx)
        else:
            merged.append([lo, hi, center, idx])
    return [(m[0], m[1], m[2], m[3]) for m in merged]


class ATRZoneBuilder(ZoneBuilder):
    """swing pivot 為候選中心價，zone_width = atr_width_multiplier * ATR(atr_period)。"""

    def __init__(
        self,
        lookback: int = 60,
        atr_period: int = 14,
        atr_width_multiplier: float = 1.5,
        pivot_window: int = 1,
        merge_pct: float = 0.01,
        max_zones_per_type: int = 5,
    ) -> None:
        self.lookback = lookback
        self.atr_period = atr_period
        self.atr_width_multiplier = atr_width_multiplier
        self.pivot_window = pivot_window
        self.merge_pct = merge_pct
        self.max_zones_per_type = max_zones_per_type

    @property
    def min_bars(self) -> int:
        return max(self.atr_period + 1, self.pivot_window * 2 + 3)

    def build(self, df: pd.DataFrame) -> list[Zone]:
        if len(df) < self.min_bars:
            return []

        atr = calc_atr(
            df["high"].to_numpy(),
            df["low"].to_numpy(),
            df["close"].to_numpy(),
            self.atr_period,
        )
        if atr <= 0:
            return []
        width = self.atr_width_multiplier * atr

        window = df.iloc[-self.lookback:] if len(df) > self.lookback else df
        offset = len(df) - len(window)
        current_price = float(df["close"].iloc[-1])

        high_pivots = find_swing_highs(window["high"].to_numpy(), self.pivot_window, self.pivot_window)
        low_pivots = find_swing_lows(window["low"].to_numpy(), self.pivot_window, self.pivot_window)

        high_candidates = [
            (center - width / 2.0, center + width / 2.0, center, offset + i) for i, center in high_pivots
        ]
        low_candidates = [
            (center - width / 2.0, center + width / 2.0, center, offset + i) for i, center in low_pivots
        ]

        merged_high = _merge_zone_candidates(high_candidates, self.merge_pct)
        merged_low = _merge_zone_candidates(low_candidates, self.merge_pct)

        merged_high.sort(key=lambda c: abs(c[2] - current_price))
        merged_low.sort(key=lambda c: abs(c[2] - current_price))

        selected = merged_high[: self.max_zones_per_type] + merged_low[: self.max_zones_per_type]
        return [
            Zone(price_low=lo, price_high=hi, method=ZoneMethod.ATR, center_price=c, formed_at_index=idx)
            for lo, hi, c, idx in selected
        ]


def _merge_qualifying_bins(qualifying: np.ndarray, max_gap_bins: int) -> list[tuple[int, int]]:
    """把 qualifying=True 的 bin index 合併成連續區段，允許中間夾雜最多
    max_gap_bins 個 False（成交量沒到門檻但仍在同一個聚集區內）。"""
    idxs = np.flatnonzero(qualifying)
    if len(idxs) == 0:
        return []
    runs: list[tuple[int, int]] = []
    start = prev = int(idxs[0])
    for raw_i in idxs[1:]:
        i = int(raw_i)
        if i - prev - 1 <= max_gap_bins:
            prev = i
        else:
            runs.append((start, prev))
            start = prev = i
    runs.append((start, prev))
    return runs


class VolumeProfileZoneBuilder(ZoneBuilder):
    """typical price = (H+L+C)/3 分箱，合併相鄰高量 bin 成一個區間。"""

    def __init__(
        self,
        lookback: int = 60,
        num_bins: int = 24,
        high_volume_percentile: float = 0.7,
        max_gap_bins: int = 1,
        max_zones_per_type: int = 5,
    ) -> None:
        self.lookback = lookback
        self.num_bins = num_bins
        self.high_volume_percentile = high_volume_percentile
        self.max_gap_bins = max_gap_bins
        self.max_zones_per_type = max_zones_per_type

    @property
    def min_bars(self) -> int:
        return 10

    def build(self, df: pd.DataFrame) -> list[Zone]:
        if len(df) < self.min_bars:
            return []

        window = df.iloc[-self.lookback:] if len(df) > self.lookback else df
        offset = len(df) - len(window)
        highs = window["high"].to_numpy()
        lows = window["low"].to_numpy()
        closes = window["close"].to_numpy()
        volumes = window["volume"].to_numpy(dtype=float)
        current_price = float(df["close"].iloc[-1])

        typical = (highs + lows + closes) / 3.0
        price_min, price_max = float(lows.min()), float(highs.max())
        if price_max <= price_min or volumes.sum() <= 0:
            return []

        bin_edges = np.linspace(price_min, price_max, self.num_bins + 1)
        bin_idx = np.clip(np.digitize(typical, bin_edges) - 1, 0, self.num_bins - 1)

        bin_volume = np.zeros(self.num_bins)
        bin_last_index = np.full(self.num_bins, -1, dtype=int)
        for i, (idx, vol) in enumerate(zip(bin_idx, volumes)):
            bin_volume[idx] += vol
            bin_last_index[idx] = i

        if bin_volume.max() <= 0:
            return []
        threshold = np.quantile(bin_volume, self.high_volume_percentile)
        qualifying = bin_volume >= threshold

        runs = _merge_qualifying_bins(qualifying, self.max_gap_bins)

        candidates: list[tuple[float, float, float, int, float]] = []
        for start, end in runs:
            run_volume = float(bin_volume[start : end + 1].sum())
            price_low = float(bin_edges[start])
            price_high = float(bin_edges[end + 1])
            center = (price_low + price_high) / 2.0
            formed_idx = offset + int(bin_last_index[start : end + 1].max())
            candidates.append((price_low, price_high, center, formed_idx, run_volume))

        below = sorted((c for c in candidates if c[2] < current_price), key=lambda c: c[4], reverse=True)
        above = sorted((c for c in candidates if c[2] >= current_price), key=lambda c: c[4], reverse=True)
        selected = below[: self.max_zones_per_type] + above[: self.max_zones_per_type]

        return [
            Zone(
                price_low=lo,
                price_high=hi,
                method=ZoneMethod.VOLUME_PROFILE,
                center_price=c,
                formed_at_index=idx,
            )
            for lo, hi, c, idx, _ in selected
        ]
