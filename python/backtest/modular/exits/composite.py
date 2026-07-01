"""
複合停損：同時採用 ATR 停損與結構停損，取「較保守（較貼近價格）」的一方
生效，模擬實務上「不管哪個先觸發都出場」的做法。

數學定義：
    LONG:  effective_stop = max(atr_stop, structural_stop)
    SHORT: effective_stop = min(atr_stop, structural_stop)
"""
from __future__ import annotations

import pandas as pd

from ..types import Direction, Position
from .atr_stop import ATRStopLoss
from .base import StopLossStrategy
from .structural_stop import StructuralStopLoss


class CompositeStopLoss(StopLossStrategy):
    def __init__(
        self,
        atr_stop: ATRStopLoss | None = None,
        structural_stop: StructuralStopLoss | None = None,
    ) -> None:
        self.atr_stop = atr_stop or ATRStopLoss()
        self.structural_stop = structural_stop or StructuralStopLoss()

    def initial_stop(self, df: pd.DataFrame, direction: Direction, entry_price: float) -> float:
        atr_level = self.atr_stop.initial_stop(df, direction, entry_price)
        structural_level = self.structural_stop.initial_stop(df, direction, entry_price)
        return max(atr_level, structural_level) if direction == Direction.LONG else min(atr_level, structural_level)

    def update(self, df: pd.DataFrame, position: Position) -> float:
        atr_level = self.atr_stop.update(df, position)
        structural_level = self.structural_stop.update(df, position)
        if position.direction == Direction.LONG:
            return max(position.stop_price, atr_level, structural_level)
        return min(position.stop_price, atr_level, structural_level)
