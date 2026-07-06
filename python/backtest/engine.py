"""
backtrader 封裝層。
負責：
  1. 從 DB 載入 K 棒資料
  2. 依 strategy 名稱載入對應策略類別
  3. 執行回測
  4. 萃取 trades / metrics 回傳標準格式
"""
from __future__ import annotations
import logging
import time
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
from .modular.service import MODULAR_STRATEGIES, run_modular_backtest

log = logging.getLogger(__name__)

STRATEGY_MAP: dict[str, type] = {
    "breakout_v1": BreakoutV1,
}


def run_backtest(
    strategy: str,
    symbols: list[str],
    timeframe: str,
    start_date: str,
    end_date: str,
    use_chip_filter: bool = False,
    chip_min_score: float = 0.0,
) -> dict[str, Any]:
    """執行回測，回傳 result + trades 兩個區塊。

    strategy 若命中 backtest.modular.strategy.STRATEGY_PRESETS，改走
    modular（純 pandas/numpy，可獨立替換 S/R、進場、停損元件）的回測引擎；
    否則沿用既有的 backtrader 引擎（STRATEGY_MAP）。

    use_chip_filter/chip_min_score 只有 modular 策略支援（見
    docs/chip-analysis-design.md 第9節）；legacy backtrader 策略沒有這個
    掛勾點，若請求帶了 use_chip_filter=True 只記警告並忽略，不中斷回測
    （這是選填的加分項，不該讓整個任務失敗）。
    """
    if strategy in MODULAR_STRATEGIES:
        return run_modular_backtest(
            strategy, symbols, timeframe, start_date, end_date,
            initial_cash=INITIAL_CASH, commission_rate=COMMISSION_RATE, tax_rate=TAX_RATE,
            use_chip_filter=use_chip_filter, chip_min_score=chip_min_score,
        )

    if use_chip_filter:
        log.warning("chip filter requested but strategy=%s is a legacy backtrader strategy — ignored", strategy)

    log.info("backtest start — strategy=%s  symbols=%s  tf=%s  %s~%s",
             strategy, symbols, timeframe, start_date, end_date)
    t0 = time.monotonic()

    strategy_cls = STRATEGY_MAP.get(strategy)
    if strategy_cls is None:
        raise ValueError(f"Unknown strategy: {strategy}. Available: {list(STRATEGY_MAP)} or modular: {list(MODULAR_STRATEGIES)}")

    cerebro = bt.Cerebro(stdstats=False)
    cerebro.broker.setcash(INITIAL_CASH)
    cerebro.broker.addcommissioninfo(TWCommission())
    cerebro.addstrategy(strategy_cls)
    cerebro.addanalyzer(bt.analyzers.TradeAnalyzer, _name="trades")
    cerebro.addanalyzer(bt.analyzers.SharpeRatio, _name="sharpe",
                        timeframe=bt.TimeFrame.Days, annualize=True)
    cerebro.addanalyzer(bt.analyzers.DrawDown, _name="drawdown")
    cerebro.addanalyzer(bt.analyzers.Returns, _name="returns")

    loaded = 0
    for symbol in symbols:
        rows = fetch_candles(symbol, timeframe, limit=1000)
        if not rows:
            log.warning("symbol=%s tf=%s — no candles in DB, skipped", symbol, timeframe)
            continue
        df = _to_dataframe(rows, start_date, end_date)
        if df.empty:
            log.warning("symbol=%s tf=%s — no data in range %s~%s, skipped",
                        symbol, timeframe, start_date, end_date)
            continue
        log.info("symbol=%s loaded %d candles (%s ~ %s)",
                 symbol, len(df), df.index[0].date(), df.index[-1].date())
        data = bt.feeds.PandasData(
            dataname=df,
            datetime=None,
            open="open", high="high", low="low",
            close="close", volume="volume", openinterest=-1,
        )
        data._name = symbol
        cerebro.adddata(data)
        loaded += 1

    if not cerebro.datas:
        raise ValueError("No data loaded for any symbol in the given date range")

    log.info("running cerebro with %d symbol(s), initial_cash=%.0f ...", loaded, INITIAL_CASH)
    initial_value = cerebro.broker.getvalue()
    results = cerebro.run()
    final_value = cerebro.broker.getvalue()
    elapsed = time.monotonic() - t0

    strat = results[0]
    result = _extract_result(strat, strategy, initial_value, final_value)
    trades = _extract_trades(strat, symbols)

    log.info("backtest done in %.2fs — total_return=%.2f%%  trades=%d  win_rate=%.1f%%",
             elapsed,
             result["total_return"] * 100,
             result["total_trades"],
             result["win_rate"] * 100)

    return {"result": result, "trades": trades}


def _to_dataframe(rows: list[dict], start_date: str, end_date: str) -> pd.DataFrame:
    df = pd.DataFrame(rows)
    df["datetime"] = pd.to_datetime(df["timestamp"], unit="s", utc=True).dt.tz_convert("Asia/Taipei")
    df = df.set_index("datetime").sort_index()
    # candles 的 open/high/low/close/amount 在 Postgres/MySQL 是 DECIMAL 欄位，
    # psycopg2/pymysql 預設回傳 decimal.Decimal 而非 float（SQLite 動態型別則
    # 天生回傳 float，本地開發用 SQLite 才沒發現這個問題）。astype(float) 統一轉型，
    # 避免後續數值運算跟 Decimal 混用出現 TypeError。
    df = df[["open", "high", "low", "close", "volume"]].astype(float)
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
