"""
ATR 通道支撐壓力偵測。

數學定義：
    1. resistance = 最近 lookback 根K棒的最高價（rolling max of high）
       support    = 最近 lookback 根K棒的最低價（rolling min of low）
    2. ATR(atr_period) 定義「有效觸碰」的容忍帶：|close[i] - level| <= atr_multiplier * ATR
    3. strength = 落在容忍帶內的收盤價根數 / lookback 根數（正規化到 0~1）

與 SwingHighLowSR 的差異：不是找局部極值後用固定百分比合併，而是直接用
「近期最高/最低價」當作 channel 邊界，再用 ATR（而非固定百分比）衡量價格
在這個邊界附近的活躍程度，藉此讓通道寬度隨波動度自動調整。
"""
from __future__ import annotations

import numpy as np
import pandas as pd

from ...indicators import calc_atr
from ..types import Level, LevelType, SRLevels
from .base import SupportResistanceStrategy


class ATRChannelSR(SupportResistanceStrategy):
    def __init__(
        self,
        lookback: int = 20,
        atr_period: int = 14,
        atr_multiplier: float = 0.5,
    ) -> None:
        self.lookback = lookback
        self.atr_period = atr_period
        self.atr_multiplier = atr_multiplier

    @property
    def min_bars(self) -> int:
        return self.atr_period + 1

    def calculate(self, df: pd.DataFrame) -> SRLevels:
        if len(df) < self.min_bars:
            return SRLevels()

        window = df.iloc[-self.lookback:] if len(df) > self.lookback else df
        highs = window["high"].to_numpy()
        lows = window["low"].to_numpy()
        closes = window["close"].to_numpy()

        atr = calc_atr(
            df["high"].to_numpy(),
            df["low"].to_numpy(),
            df["close"].to_numpy(),
            self.atr_period,
        )
        if atr <= 0:
            return SRLevels()

        band = self.atr_multiplier * atr
        resistance_price = float(highs.max())
        support_price = float(lows.min())

        res_touches = int(np.sum(np.abs(closes - resistance_price) <= band))
        sup_touches = int(np.sum(np.abs(closes - support_price) <= band))
        n = len(window)

        resistances = [Level(resistance_price, LevelType.RESISTANCE, res_touches / n, "atr_channel")]
        supports = [Level(support_price, LevelType.SUPPORT, sup_touches / n, "atr_channel")]
        return SRLevels(supports=supports, resistances=resistances)
