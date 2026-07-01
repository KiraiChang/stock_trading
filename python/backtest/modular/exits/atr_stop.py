"""
ATR 停損：以進場當下的波動度（ATR）決定固定風險距離。

數學定義：
    LONG:  stop = entry_price - atr_multiplier * ATR(atr_period)_at_entry
    SHORT: stop = entry_price + atr_multiplier * ATR(atr_period)_at_entry

進場後停損價固定不變（不隨盤勢調整），代表「這筆交易願意承擔的最大波動風險」，
是最基本、最常見的停損方式，用來跟 StructuralStopLoss 的追蹤停損互補。
"""
from __future__ import annotations

import pandas as pd

from ...indicators import calc_atr
from ..types import Direction, Position
from .base import StopLossStrategy


class ATRStopLoss(StopLossStrategy):
    def __init__(self, atr_period: int = 14, atr_multiplier: float = 2.0) -> None:
        self.atr_period = atr_period
        self.atr_multiplier = atr_multiplier

    def _atr(self, df: pd.DataFrame) -> float:
        return calc_atr(
            df["high"].to_numpy(),
            df["low"].to_numpy(),
            df["close"].to_numpy(),
            self.atr_period,
        )

    def initial_stop(self, df: pd.DataFrame, direction: Direction, entry_price: float) -> float:
        atr = self._atr(df)
        offset = self.atr_multiplier * atr
        return entry_price - offset if direction == Direction.LONG else entry_price + offset

    def update(self, df: pd.DataFrame, position: Position) -> float:
        # 固定風險距離，不隨盤勢變動
        return position.stop_price
