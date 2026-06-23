"""
backtrader 封裝層。
負責：
  1. 從 DB 載入 K 棒資料
  2. 依 strategy 名稱載入對應策略類別
  3. 執行回測
  4. 萃取 trades / metrics 回傳標準格式
"""
import io
from datetime import datetime, timezone
from typing import Any

import backtrader as bt
import pandas as pd

import sys, os
sys.path.insert(0, os.path.dirname(os.path.dirname(__file__)))

from config import INITIAL_CASH, COMMISSION_RATE, TAX_RATE
from db import fetch_candles
from strategy.base import TWCommission
from strategy.breakout_v1 import BreakoutV1

STRATEGY_MAP: dict[str, type] = {
    "breakout_v1": BreakoutV1,
}


def run_backtest(
    strategy: str,
    symbols: list[str],
    timeframe: str,
    start_date: str,
    end_date: str,
) -> dict[str, Any]:
    """執行回測，回傳 result + trades 兩個區塊。"""
    strategy_cls = STRATEGY_MAP.get(strategy)
    if strategy_cls is None:
        raise ValueError(f"Unknown strategy: {strategy}. Available: {list(STRATEGY_MAP)}")

    cerebro = bt.Cerebro(stdstats=True)
    cerebro.broker.setcash(INITIAL_CASH)
    cerebro.broker.addcommissioninfo(TWCommission())
    cerebro.addstrategy(strategy_cls)
    cerebro.addanalyzer(bt.analyzers.TradeAnalyzer, _name="trades")
    cerebro.addanalyzer(bt.analyzers.SharpeRatio, _name="sharpe",
                        timeframe=bt.TimeFrame.Days, annualize=True)
    cerebro.addanalyzer(bt.analyzers.DrawDown, _name="drawdown")
    cerebro.addanalyzer(bt.analyzers.Returns, _name="returns")

    for symbol in symbols:
        rows = fetch_candles(symbol, timeframe, limit=1000)
        if not rows:
            continue
        df = _to_dataframe(rows, start_date, end_date)
        if df.empty:
            continue
        data = bt.feeds.PandasData(
            dataname=df,
            datetime=None,
            open="open", high="high", low="low",
            close="close", volume="volume", openinterest=-1,
        )
        data._name = symbol
        cerebro.adddata(data)

    if not cerebro.datas:
        raise ValueError("No data loaded for any symbol in the given date range")

    initial_value = cerebro.broker.getvalue()
    results = cerebro.run()
    final_value = cerebro.broker.getvalue()

    strat = results[0]
    return {
        "result":  _extract_result(strat, strategy, initial_value, final_value),
        "trades":  _extract_trades(strat, symbols),
    }


def _to_dataframe(rows: list[dict], start_date: str, end_date: str) -> pd.DataFrame:
    df = pd.DataFrame(rows)
    df["datetime"] = pd.to_datetime(df["timestamp"], unit="s", utc=True).dt.tz_convert("Asia/Taipei")
    df = df.set_index("datetime").sort_index()
    df = df[["open", "high", "low", "close", "volume"]]
    df = df.loc[start_date:end_date]
    return df


def _extract_result(strat, strategy: str, initial: float, final: float) -> dict:
    ta = strat.analyzers.trades.get_analysis()
    sharpe_val = strat.analyzers.sharpe.get_analysis().get("sharperatio", 0.0) or 0.0
    dd = strat.analyzers.drawdown.get_analysis()
    ret = strat.analyzers.returns.get_analysis()

    total_closed = ta.get("total", {}).get("closed", 0)
    won = ta.get("won", {}).get("total", 0)
    lost = ta.get("lost", {}).get("total", 0)
    pnl_net = ta.get("pnl", {}).get("net", {}).get("total", 0.0)

    total_return = (final - initial) / initial if initial else 0.0
    annual_return = float(ret.get("rnorm100", 0.0)) / 100.0

    return {
        "strategy":      strategy,
        "total_return":  round(total_return, 6),
        "annual_return": round(annual_return, 6),
        "win_rate":      round(won / total_closed, 4) if total_closed else 0.0,
        "max_drawdown":  round(float(dd.get("max", {}).get("drawdown", 0.0)) / 100.0, 6),
        "sharpe_ratio":  round(float(sharpe_val), 4),
        "total_trades":  total_closed,
        "win_trades":    won,
        "loss_trades":   lost,
        "avg_pnl":       round(pnl_net / total_closed, 4) if total_closed else 0.0,
    }


def _extract_trades(strat, symbols: list[str]) -> list[dict]:
    trades = []
    for trade in strat._trades.values():
        for t in trade:
            if not t.isclosed:
                continue
            trades.append({
                "symbol":      t.data._name,
                "direction":   "BUY",   # 台股先簡化為多方進出
                "entry_time":  _unix_to_iso(t.dtopen),
                "exit_time":   _unix_to_iso(t.dtclose),
                "entry_price": round(t.price, 2),
                "exit_price":  round(t.price + t.pnl / t.size if t.size else 0, 2),
                "size":        abs(t.size),
                "pnl":         round(t.pnlcomm, 2),
                "pnl_pct":     round(t.pnlcomm / (t.price * abs(t.size)) if t.price and t.size else 0, 6),
                "commission":  round(abs(t.pnlcomm - t.pnl), 2),
            })
    return trades


def _unix_to_iso(bt_date) -> str | None:
    if bt_date is None:
        return None
    try:
        dt = bt.num2date(bt_date)
        return dt.isoformat()
    except Exception:
        return None
