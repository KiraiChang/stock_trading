"""StopLossStrategy 介面：所有停損演算法的共同抽象。"""
from __future__ import annotations

from abc import ABC, abstractmethod

import pandas as pd

from ..types import Direction, Position


class StopLossStrategy(ABC):
    @abstractmethod
    def initial_stop(self, df: pd.DataFrame, direction: Direction, entry_price: float) -> float:
        """進場當下算出的初始停損價。df 含進場那根K棒（含）為止的資料。"""
        raise NotImplementedError

    @abstractmethod
    def update(self, df: pd.DataFrame, position: Position) -> float:
        """
        持倉期間每根新K棒收盤後呼叫，回傳「這根K棒之後」應生效的停損價。
        必須是單調收緊（LONG 只能提高、SHORT 只能降低），不可對已持有部位放寬風險。
        可利用 position.extra 這個 dict 保存策略自身需要的狀態（例如目前追蹤到的結構價位）。
        """
        raise NotImplementedError
