"""
串接既有 backtest pipeline 的入口。

run_modular_backtest() 回傳格式與 backtest/engine.py 的 run_backtest() 完全相同
（{"result": {...}, "trades": [...]}）， 讓 worker.py（Method A）/ http_server.py
（Method B）/ Go 端的 backtest.Manager 都不需要修改；建立回測任務時把
`strategy` 欄位填成 STRATEGY_PRESETS 裡任一名稱（見 strategy.py）即可路由到
這個模組化引擎，而不是走 backtrader。
"""
from __future__ import annotations

import logging
from typing import Any

import pandas as pd

from db import fetch_chip_scores

from ..data_range import load_task_candles
from .backtester import BacktestEngine
from .strategy import STRATEGY_PRESETS, build_strategy
from .types import BacktestReport, Direction, Trade

log = logging.getLogger(__name__)

MODULAR_STRATEGIES = set(STRATEGY_PRESETS)


def run_modular_backtest(
    strategy: str,
    symbols: list[str],
    timeframe: str,
    start_date: str,
    end_date: str,
    initial_cash: float = 1_000_000.0,
    commission_rate: float = 0.001425,
    tax_rate: float = 0.003,
    use_chip_filter: bool = False,
    chip_min_score: float = 0.0,
    *,
    warmup_bars: int,
    max_task_bars: int,
    max_symbol_bars: int | None = None,
) -> dict[str, Any]:
    """依日期範圍取資料（I-137）：先取完所有標的（總根數受上限），再逐檔跑；每一檔從範圍內第一根開始交易與記權益，
    暖身段只用來算指標。日期、標的數等請求檢查在共用入口 `engine.run_backtest` 做。"""
    loaded, coverage = load_task_candles(
        symbols, timeframe, start_date, end_date,
        warmup_bars=warmup_bars, max_task_bars=max_task_bars, max_symbol_bars=max_symbol_bars,
    )
    engine = BacktestEngine(
        initial_cash=initial_cash,
        commission_rate=commission_rate,
        tax_rate=tax_rate,
        chip_min_score=chip_min_score if use_chip_filter else None,
    )

    reports: list[BacktestReport] = []
    for sd in loaded:
        chip_scores = _chip_score_map(sd.symbol, start_date, end_date) if use_chip_filter else None
        report = engine.run(sd.symbol, sd.df, build_strategy(strategy), chip_scores=chip_scores, trade_start_index=sd.r0)
        reports.append(report)
        log.info(
            "modular backtest done symbol=%s trades=%d total_return=%.2f%%",
            sd.symbol, report.total_trades, report.total_return * 100,
        )

    result = _aggregate_result(strategy, reports, initial_cash)
    result["data_coverage"] = coverage
    return {
        "result": result,
        "trades": [_trade_to_dict(t) for r in reports for t in r.trades],
    }


def _chip_score_map(symbol: str, start_date: str, end_date: str) -> dict[str, float]:
    """把 fetch_chip_scores 的列表轉成 {date_str: total_score} 查找表，供
    BacktestEngine 逐 bar 比對（見 backtester.py::_passes_chip_filter）。"""
    rows = fetch_chip_scores(symbol, start_date, end_date)
    result: dict[str, float] = {}
    for r in rows:
        d = r["trade_date"]
        key = d.strftime("%Y-%m-%d") if hasattr(d, "strftime") else str(d)[:10]
        result[key] = float(r["total_score"])
    return result


def _aggregate_result(strategy: str, reports: list[BacktestReport], initial_cash: float) -> dict:
    """
    多檔股票各自獨立跑（各自視為 100% 資金、彼此互不影響），不是共用資金池的
    組合回測。win_rate/avg_pnl 用交易筆數加權彙總；total_return/annual_return/
    sharpe 用各symbol結果的簡單平均近似整體表現，max_drawdown 取最差者。
    若要做真正的多檔共用資金、再平衡的組合回測，需要另外設計，不在本次需求範圍內。
    """
    total_trades = int(sum(r.total_trades for r in reports))
    win_trades = int(sum(r.win_trades for r in reports))
    loss_trades = int(sum(r.loss_trades for r in reports))
    total_pnl = sum(t.pnl for r in reports for t in r.trades)
    n = len(reports)

    # 所有數值欄位在回傳前都明確轉成原生 float/int：即使上游任何一個計算
    # （numpy/pandas 運算、Wilder smoothing 累加等）不小心洩漏出 np.float64，
    # 也不會流進 SQLAlchemy 的 bind 參數——這是實際發生過的問題，numpy>=2.0
    # 的 np.float64 repr 是 "np.float64(x)"，被 psycopg2 當成純文字塞進 SQL
    # 後會被 Postgres 誤判成 schema-qualified 識別字（"schema np does not exist"）。
    return {
        "strategy": strategy,
        "total_return": float(round(sum(r.total_return for r in reports) / n, 6)),
        "annual_return": float(round(sum(r.annual_return for r in reports) / n, 6)),
        "win_rate": float(round(win_trades / total_trades, 4)) if total_trades else 0.0,
        "max_drawdown": float(round(max((r.max_drawdown for r in reports), default=0.0), 6)),
        "sharpe_ratio": float(round(sum(r.sharpe_ratio for r in reports) / n, 4)),
        "total_trades": total_trades,
        "win_trades": win_trades,
        "loss_trades": loss_trades,
        "avg_pnl": float(round(total_pnl / total_trades, 4)) if total_trades else 0.0,
    }


def _trade_to_dict(t: Trade) -> dict:
    return {
        "symbol": t.symbol,
        "direction": "BUY" if t.direction == Direction.LONG else "SELL",
        "entry_time": _iso(t.entry_time),
        "exit_time": _iso(t.exit_time),
        "entry_price": float(round(t.entry_price, 2)),
        "exit_price": float(round(t.exit_price, 2)),
        "size": float(t.size),
        "pnl": float(t.pnl),
        "pnl_pct": float(t.pnl_pct),
        "commission": float(t.commission),
    }


def _iso(ts: object) -> str | None:
    if ts is None:
        return None
    return pd.Timestamp(ts).isoformat()
