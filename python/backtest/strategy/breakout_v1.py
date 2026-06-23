"""
Breakout V1 策略 — 與 Go signal engine 邏輯 1:1 對齊。

觸發條件（與 Go breakout.go 完全相同）：
  BUY:  Close > Resistance AND VolRatio >= 2.0 AND Trend == BULLISH
  SELL: Close < Support    AND Trend == BEARISH
  EXIT: 部位已持有且上述 SELL 條件成立時平倉

支撐阻力識別（與 Go support_resistance.go 相同）：
  Local Max/Min（window=3），差距 < 1% 合併，取最強前 3 個
"""
from __future__ import annotations
import backtrader as bt
import numpy as np
from strategy.base import BaseTWStrategy

SR_LOOKBACK = 60
BREAKOUT_VOL_THRESH = 2.0
VOL_SPIKE_THRESH = 3.0
MERGE_THRESH = 0.01
MAX_LEVELS = 3


class BreakoutV1(BaseTWStrategy):
    params = (
        ("ma_fast", 5),
        ("ma_slow", 20),
        ("rsi_period", 14),
        ("vol_period", 20),
    )

    def __init__(self):
        self.ma_fast = bt.indicators.SMA(self.data.close, period=self.p.ma_fast)
        self.ma_slow = bt.indicators.SMA(self.data.close, period=self.p.ma_slow)
        self.rsi = bt.indicators.RSI(self.data.close, period=self.p.rsi_period,
                                     safediv=True)
        self.vol_ma = bt.indicators.SMA(self.data.volume, period=self.p.vol_period)
        self.order = None

    def next(self):
        if self.order:
            return  # 等待掛單完成

        # 取近 SR_LOOKBACK 根 K 棒的收盤 / 量
        closes  = np.array(self.data.close.get(size=SR_LOOKBACK))
        highs   = np.array(self.data.high.get(size=SR_LOOKBACK))
        lows    = np.array(self.data.low.get(size=SR_LOOKBACK))
        volumes = np.array(self.data.volume.get(size=SR_LOOKBACK))

        if len(closes) < SR_LOOKBACK:
            return

        resistances = _find_levels(highs, "Resistance")
        supports    = _find_levels(lows,  "Support")
        trend       = _detect_trend(highs, lows)
        vol_ratio   = (float(self.data.volume[0]) / float(self.vol_ma[0])
                       if float(self.vol_ma[0]) > 0 else 0.0)
        price       = float(self.data.close[0])

        if not self.position:
            # BUY signal
            for r in resistances:
                if price > r and vol_ratio >= BREAKOUT_VOL_THRESH and trend == "BULLISH":
                    self.log(f"BREAKOUT BUY @ {price:.2f}, res={r:.2f}, vol_ratio={vol_ratio:.2f}x")
                    self.order = self.buy()
                    break
        else:
            # SELL signal
            for s in supports:
                if price < s and trend == "BEARISH":
                    self.log(f"BREAKDOWN SELL @ {price:.2f}, sup={s:.2f}")
                    self.order = self.sell()
                    break

    def notify_order(self, order):
        super().notify_order(order)
        if order.status not in [order.Submitted, order.Accepted]:
            self.order = None


# ── 支撐阻力 / 趨勢輔助函數（對齊 Go 邏輯）────────────────────

def _find_levels(prices: np.ndarray, level_type: str) -> list[float]:
    """Local Max（阻力）或 Local Min（支撐），與 Go CalcSupportResistance 一致。"""
    candidates = []
    for i in range(1, len(prices) - 1):
        if level_type == "Resistance":
            if prices[i] > prices[i - 1] and prices[i] > prices[i + 1]:
                candidates.append(float(prices[i]))
        else:
            if prices[i] < prices[i - 1] and prices[i] < prices[i + 1]:
                candidates.append(float(prices[i]))

    if not candidates:
        return []

    # 合併相近 < 1% 的候選點
    clusters: list[list[float]] = [[candidates[0]]]
    for price in candidates[1:]:
        center = sum(clusters[-1]) / len(clusters[-1])
        if abs(price - center) / center < MERGE_THRESH:
            clusters[-1].append(price)
        else:
            clusters.append([price])

    # 以觸碰次數排序，取最多 MAX_LEVELS 個
    clusters.sort(key=len, reverse=True)
    return [sum(cl) / len(cl) for cl in clusters[:MAX_LEVELS]]


def _detect_trend(highs: np.ndarray, lows: np.ndarray) -> str:
    """與 Go DetectTrend 一致：window=3 的 Swing High/Low。"""
    swing_highs = [highs[i] for i in range(1, len(highs) - 1)
                   if highs[i] > highs[i - 1] and highs[i] > highs[i + 1]]
    swing_lows  = [lows[i]  for i in range(1, len(lows) - 1)
                   if lows[i]  < lows[i - 1]  and lows[i]  < lows[i + 1]]

    if len(swing_highs) < 2 or len(swing_lows) < 2:
        return "SIDEWAYS"

    hh = swing_highs[-1] > swing_highs[-2]
    hl = swing_lows[-1]  > swing_lows[-2]
    lh = swing_highs[-1] < swing_highs[-2]
    ll = swing_lows[-1]  < swing_lows[-2]

    if hh and hl:
        return "BULLISH"
    if lh and ll:
        return "BEARISH"
    return "SIDEWAYS"
