"""SupportResistanceStrategy 介面：所有支撐/壓力演算法的共同抽象。"""
from __future__ import annotations

from abc import ABC, abstractmethod

import pandas as pd

from ..types import SRLevels


class SupportResistanceStrategy(ABC):
    """輸入 OHLCV DataFrame（時間升冪排列），輸出目前有效的支撐/壓力位。"""

    @abstractmethod
    def calculate(self, df: pd.DataFrame) -> SRLevels:
        """df 只包含「至今」的資料（呼叫端負責切片避免 lookahead bias）。"""
        raise NotImplementedError

    @property
    @abstractmethod
    def min_bars(self) -> int:
        """calculate() 所需的最少K棒數，資料不足時呼叫端應跳過。"""
        raise NotImplementedError
