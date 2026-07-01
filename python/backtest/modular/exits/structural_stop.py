"""
結構停損（追蹤止損）：以最近一個「已確認」的 swing low/high 作為停損，
價格結構持續延伸時停損跟著推進，但只會收緊、不會放寬。

數學定義：
    LONG:  stop = 進場前（或持倉期間）最近一個 confirmed swing low
           若持倉期間出現「更高」的新 swing low，停損上移至該價位
           （對應 CLAUDE.md 的「Structure Broken（HL失效）」概念：
            只要不斷創出更高的低點，代表多頭結構還在，停損就跟著上移；
            一旦收盤跌破目前的停損價，代表 HL 失守，出場）
    SHORT: 對稱邏輯，改用 swing high，只會下移不會上移

找不到任何 swing 點時（資料太短、盤整無明顯結構），退回用 lookback 期間
的最低/最高價當作安全網，避免停損為 None。
"""
from __future__ import annotations

import pandas as pd

from ..trend import find_swing_highs, find_swing_lows
from ..types import Direction, Position
from .base import StopLossStrategy


class StructuralStopLoss(StopLossStrategy):
    def __init__(self, pivot_window: int = 1, lookback: int = 60) -> None:
        self.pivot_window = pivot_window
        self.lookback = lookback

    def _window(self, df: pd.DataFrame) -> pd.DataFrame:
        return df.iloc[-self.lookback:] if len(df) > self.lookback else df

    def initial_stop(self, df: pd.DataFrame, direction: Direction, entry_price: float) -> float:
        window = self._window(df)
        if direction == Direction.LONG:
            lows = find_swing_lows(window["low"].to_numpy(), self.pivot_window, self.pivot_window)
            return lows[-1][1] if lows else float(window["low"].min())
        highs = find_swing_highs(window["high"].to_numpy(), self.pivot_window, self.pivot_window)
        return highs[-1][1] if highs else float(window["high"].max())

    def update(self, df: pd.DataFrame, position: Position) -> float:
        window = self._window(df)
        if position.direction == Direction.LONG:
            lows = find_swing_lows(window["low"].to_numpy(), self.pivot_window, self.pivot_window)
            candidate = lows[-1][1] if lows else position.stop_price
            return max(position.stop_price, candidate)  # 只上移，不下修
        highs = find_swing_highs(window["high"].to_numpy(), self.pivot_window, self.pivot_window)
        candidate = highs[-1][1] if highs else position.stop_price
        return min(position.stop_price, candidate)  # 只下移，不上修
