"""
突破策略（雙向）：多方看突破壓力，空方看跌破支撐。

進場數學條件（與 Go internal/signal/breakout.go 的 CheckBreakout 對齊）：

    LONG:  close[t] > resistance.price
           AND volume[t] / MA(volume, vol_period)[t] >= vol_multiplier
           AND trend(t) == BULLISH

    SHORT: close[t] < support.price
           AND trend(t) == BEARISH
           （既有系統對跌破不要求爆量，因為恐慌性下跌常伴隨量縮而非量增）
"""
from __future__ import annotations

import pandas as pd

from ...indicators import calc_volume_spike
from ..trend import detect_trend
from ..types import Direction, EntrySignal, SRLevels
from .base import EntryStrategy


class BreakoutEntry(EntryStrategy):
    def __init__(self, vol_multiplier: float = 2.0, vol_period: int = 20) -> None:
        self.vol_multiplier = vol_multiplier
        self.vol_period = vol_period

    @property
    def min_bars(self) -> int:
        return self.vol_period + 11  # +11 給 trend 判斷所需的 swing 偵測空間

    def evaluate(self, df: pd.DataFrame, levels: SRLevels) -> EntrySignal | None:
        if len(df) < self.min_bars:
            return None

        close = float(df["close"].iloc[-1])
        trend = detect_trend(df)
        vol_stat = calc_volume_spike(df["volume"].to_numpy(), self.vol_period)
        vol_ratio = vol_stat["ratio"]

        for r in levels.resistances:
            if close > r.price and vol_ratio >= self.vol_multiplier and trend.value == "BULLISH":
                return EntrySignal(
                    direction=Direction.LONG,
                    reference_level=r.price,
                    reason=f"breakout above resistance {r.price:.2f}, vol_ratio={vol_ratio:.2f}x",
                )

        for s in levels.supports:
            if close < s.price and trend.value == "BEARISH":
                return EntrySignal(
                    direction=Direction.SHORT,
                    reference_level=s.price,
                    reason=f"breakdown below support {s.price:.2f}",
                )

        return None
