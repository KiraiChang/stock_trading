"""
回測支撐策略（retest-support / pullback entry）：多頭結構中價格拉回測試
支撐後止穩的進場訊號。

進場數學條件（三者需同時成立）：
    1. trend(t) == BULLISH                                  — 只在多頭結構中做多
    2. |low[t] - support.price| / support.price <= tolerance — 當根K棒最低價
       曾經觸及/貼近支撐（在容忍帶內），代表發生了「回測」
    3. close[t] > support.price AND close[t] > open[t]        — 收盤收回支撐之上
       且收成陽線，代表買盤在支撐處守住、價格止穩反彈

這與突破策略互補：突破策略在「價格創新高」時進場，回測策略則是在既有
支撐（可能就是先前被突破的壓力位，也可能是 swing low／volume node）
被重新測試且守住時進場，兩者可分別對應不同市場情境。
"""
from __future__ import annotations

import pandas as pd

from ..trend import Trend, detect_trend
from ..types import Direction, EntrySignal, SRLevels
from .base import EntryStrategy


class PullbackSupportEntry(EntryStrategy):
    def __init__(self, tolerance_pct: float = 0.005) -> None:
        self.tolerance_pct = tolerance_pct

    @property
    def min_bars(self) -> int:
        return 21  # 與 trend 判斷所需的最少K棒數一致

    def evaluate(self, df: pd.DataFrame, levels: SRLevels) -> EntrySignal | None:
        if len(df) < self.min_bars:
            return None
        if detect_trend(df) != Trend.BULLISH:
            return None

        bar = df.iloc[-1]
        low, close, open_ = float(bar["low"]), float(bar["close"]), float(bar["open"])

        for s in levels.supports:
            if s.price <= 0:
                continue
            touched = abs(low - s.price) / s.price <= self.tolerance_pct
            held = close > s.price and close > open_
            if touched and held:
                return EntrySignal(
                    direction=Direction.LONG,
                    reference_level=s.price,
                    reason=f"pullback retest support {s.price:.2f}, held with bullish close {close:.2f}",
                )
        return None
