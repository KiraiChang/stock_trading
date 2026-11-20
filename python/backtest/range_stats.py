"""backtrader 路徑只算範圍內的統計（I-137）。

原本的 `Returns`、`SharpeRatio`（含它內部的 `TimeReturn` 子分析器）、`DrawDown` 都從第一根資料開始算，沒有排除暖身段的參數；
暖身段空手、資產不變，會稀釋年化報酬與夏普。這裡改成：

* `RangeValue` 分析器記錄**範圍內每一根**收盤後的 broker 資產。backtrader 在指標達到 minperiod 之前呼叫的是
  `prenext`／`nextstart`，分析器的這兩個預設都轉呼叫 `next`，所以三種時機都會記到（⛔ 不在策略的 `next()` 記：
  暖身是 0 或很短時會漏掉範圍前段）。
* `range_stats` 用跟三個分析器**相同的公式**算（照 backtrader 1.9.78 的原始碼）：
  * 年化報酬與夏普是**以日為期**（原本兩個分析器都是 `timeframe=Days`；`PandasData` 的預設 timeframe 也是 Days）：
    先把範圍內每一根依 backtrader 的日期鍵（bar 時間的日期，⛔ 不是根數）分組、取每一天最後一根的資產。分 K 一天多根時，⛔ 不能把每一根當成一天
    （review 2026-11-19：那樣會讓 1m 的結果在成交完全相同時也改變）。
  * 年化報酬（`Returns` 的 `rnorm`）：`expm1(ln(期末 / 期初) / 天數 × 252)`。
  * 夏普（`SharpeRatio`，`timeframe=Days`、`annualize=True`、`riskfreerate=0.01`、`convertrate=True`、`stddev_sample=False`）：
    每天期末資產的報酬扣掉 `(1.01)^(1/252) − 1`，平均 ÷ 母體標準差 × √252；算不出來（標準差 0、沒有報酬）時是 None。
  * 最大回撤（`DrawDown` 的 `max.drawdown`）：**逐根**的 `100 × (目前為止的最高資產 − 資產) / 最高資產` 取最大。
  測試鎖住「同一組成交與權益序列下，數值等於原本三個分析器」，⛔ 不改公式。
"""
from __future__ import annotations

import math
from dataclasses import dataclass

import backtrader as bt

from strategy.base import bar_taipei_date

TRADING_DAYS = 252
RISK_FREE_RATE = 0.01


class RangeValue(bt.Analyzer):
    """範圍內每一根收盤後的 broker 資產。`trade_start` 是範圍首根的台北日期（None＝每一根都算）。"""

    params = (("trade_start", None),)

    def start(self):
        self.values: list[float] = []
        self.days: list[object] = []  # 每一根的日期鍵（跟 backtrader 的 TimeFrame.Days 分期相同：bar 時間的日期）
        self._value = None
        # 範圍首根之前的資產：暖身段沒有交易，就是期初資產；跟原本的 `Returns`／`TimeReturn` 的期初值相同。
        self.start_value = self.strategy.broker.getvalue()

    def notify_fund(self, cash, value, fundvalue, shares):
        self._value = value

    def next(self):
        if self.p.trade_start is not None and bar_taipei_date(self.strategy.datas[0]) < self.p.trade_start:
            self.start_value = self._value  # 暖身段：資產不變，記成範圍的期初值
            return
        self.values.append(self._value)
        self.days.append(self.strategy.datetime.date(0))

    def get_analysis(self):
        return {"start_value": self.start_value, "values": list(self.values), "days": list(self.days)}


@dataclass
class RangeStats:
    annual_return: float
    sharpe_ratio: float | None
    max_drawdown_pct: float  # 百分比（跟 DrawDown 的 max.drawdown 相同單位）


def _daily_closes(values: list[float], days: list[object] | None) -> list[float]:
    """每一天最後一根的資產（依日期鍵分組，保持順序）。沒有日期鍵時當成一根一天。"""
    if days is None:
        return list(values)
    out: list[float] = []
    prev = object()
    for v, d in zip(values, days):
        if d == prev:
            out[-1] = v
        else:
            out.append(v)
            prev = d
    return out


def range_stats(start_value: float, values: list[float], final_value: float, days: list[object] | None = None) -> RangeStats:
    daily = _daily_closes(values, days)
    n = len(daily)
    # 年化報酬（Returns）
    if n == 0:
        annual = 0.0
    else:
        try:
            ratio = final_value / start_value
        except ZeroDivisionError:
            rtot = float("-inf")
        else:
            rtot = float("-inf") if ratio < 0.0 else (math.log(ratio) if ratio > 0 else float("-inf"))
        ravg = rtot / n
        annual = math.expm1(ravg * TRADING_DAYS) if ravg > float("-inf") else ravg

    # 夏普（SharpeRatio + TimeReturn）
    sharpe: float | None = None
    if n:
        prev = start_value
        rets = []
        for v in daily:
            rets.append(v / prev - 1.0)
            prev = v
        rate = pow(1.0 + RISK_FREE_RATE, 1.0 / TRADING_DAYS) - 1.0
        free = [r - rate for r in rets]
        avg = math.fsum(free) / len(free)
        dev = math.sqrt(math.fsum((x - avg) ** 2 for x in free) / len(free))
        try:
            sharpe = math.sqrt(TRADING_DAYS) * (avg / dev)
        except ZeroDivisionError:
            sharpe = None

    # 最大回撤（DrawDown）：逐根
    peak = float("-inf")
    max_dd = 0.0
    for v in values:
        peak = max(peak, v)
        max_dd = max(max_dd, 100.0 * (peak - v) / peak)
    return RangeStats(annual_return=annual, sharpe_ratio=sharpe, max_drawdown_pct=max_dd)
