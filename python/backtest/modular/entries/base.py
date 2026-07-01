"""EntryStrategy 介面：所有進場訊號產生器的共同抽象。"""
from __future__ import annotations

from abc import ABC, abstractmethod

import pandas as pd

from ..types import EntrySignal, SRLevels


class EntryStrategy(ABC):
    @abstractmethod
    def evaluate(self, df: pd.DataFrame, levels: SRLevels) -> EntrySignal | None:
        """
        df: 只包含「至今」的資料（含當前 bar，即訊號評估當下最後一根K棒）。
        levels: 由某個 SupportResistanceStrategy 算出的當前支撐/壓力位。
        回傳 None 代表這根K棒沒有進場訊號。
        """
        raise NotImplementedError

    @property
    @abstractmethod
    def min_bars(self) -> int:
        raise NotImplementedError
