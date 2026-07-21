"""
突破策略（雙向）：多方看突破壓力，空方看跌破支撐。

進場數學條件：

    LONG:  close[t] > resistance.price
           AND volume[t] / MA(volume, vol_period)[t] >= vol_multiplier
           AND trend(t) == BULLISH
           （單根突破；未含 Go CheckBreakout 的前根跨越判定）

    SHORT: close[t-3] >= support.price   （跌破前一根仍在支撐之上）
           AND close[t-2] < support.price （跌破當根收破支撐）
           AND close[t-1] < support.price AND close[t] < support.price
               （其後連續 2 根未收回支撐）
           AND trend(t) == BEARISH
           （跌破不要求爆量，因為恐慌性下跌常伴隨量縮而非量增）

SHORT 的確認窗鏡像 Go internal/signal/breakout.go 的 confirmedBreakdownSupport
（breakdownConfirmationCandles=2）：跌破當根與第一根確認都不發訊，第二根確認仍未
收回時才輸出；同一跌破跨越多個支撐時回報 nearest crossed（最接近收盤的最低支撐，
同價再取 strength 高者）。LONG 側維持單根突破，未跟進跨越判定。
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

        if trend.value == "BEARISH":
            s = self._confirmed_breakdown_support(df, levels)
            if s is not None:
                return EntrySignal(
                    direction=Direction.SHORT,
                    reference_level=s.price,
                    reason=f"breakdown below support {s.price:.2f}, 連續 {self._confirmation_candles} 根未收回",
                )

        return None

    # 跌破後需要的確認 K 棒數（對齊 Go breakdownConfirmationCandles）
    _confirmation_candles = 2

    def _confirmed_breakdown_support(self, df: pd.DataFrame, levels: SRLevels):
        """回傳已完成確認窗的跌破支撐，若無則 None。

        跌破當根位於 iloc[-(_confirmation_candles+1)]，其前一根需仍在支撐之上，
        其後 _confirmation_candles 根需連續未收回支撐（收盤 < support）。
        多個支撐符合時取 nearest crossed（價位最低者，同價再取 strength 高者）。
        """
        need = self._confirmation_candles + 2  # 破前一根 + 跌破當根 + 確認根
        if len(df) < need:
            return None

        closes = df["close"].to_numpy()
        break_idx = len(closes) - 1 - self._confirmation_candles
        before_break = float(closes[break_idx - 1])
        break_close = float(closes[break_idx])
        confirmations = [float(c) for c in closes[break_idx + 1 :]]

        best = None
        for s in levels.supports:
            crossed = before_break >= s.price and break_close < s.price
            not_recovered = all(c < s.price for c in confirmations)
            if crossed and not_recovered:
                if best is None or s.price < best.price or (s.price == best.price and s.strength > best.strength):
                    best = s
        return best
