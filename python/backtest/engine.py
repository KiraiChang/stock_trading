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
import math
import time
from datetime import datetime, timezone
from typing import Any

import backtrader as bt
import pandas as pd

import sys, os
sys.path.insert(0, os.path.dirname(os.path.dirname(__file__)))

from config import INITIAL_CASH, COMMISSION_RATE, TAX_RATE
from db import TAIPEI, fetch_candles
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

    # tradehistory=True：交易明細要從每次成交（history 的 event）算進出場價與股數（I-135）。
    # ⛔ 不能關：交易結束後 Trade 的 size／value 都歸 0，沒有成交紀錄就算不出來。
    cerebro = bt.Cerebro(stdstats=False, tradehistory=True)
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


class TradeExtractionError(RuntimeError):
    """交易的成交紀錄不符合預期（沒有 history、部位變號、毛損益或費用對不上）。寧可讓任務 failed，
    ⛔ 不寫出看起來正常的錯誤數字。"""


# 絕對容許值（⛔ 不乘上金額）：未四捨五入的毛損益與 t.pnl、Σ event.commission 與 t.commission、部位歸零，
# 差距都要在這個值以內。採用的值見 docs/database-schema.md 的 backtest_trades；大額、多筆成交的測試支持它夠用。
_FLOAT_TOL = 1e-6


def _extract_trades(strat, symbols: list[str]) -> list[dict]:
    """把已結束的交易轉成 backtest_trades 的列（規格見 docs/database-schema.md 的 backtest_trades）。

    `strat._trades` 是兩層：data → tradeid → 交易的 list（I-135 之前只走了一層，拿到的是 tradeid）。
    還沒平倉的部位⛔ 不列。
    """
    trades = []
    for by_id in strat._trades.values():
        for trade_list in by_id.values():
            for t in trade_list:
                if not t.isclosed:
                    continue
                trades.append(_trade_row(t))
    return trades


def _trade_row(t) -> dict:
    """一筆已結束的交易，全部從成交紀錄（`t.history` 的 event）算。

    ⛔ 不用 `t.size`／`t.value`（結束後歸 0），也⛔ 不用 `t.price` 當進場價（減碼後再加碼時，它是剩餘部位的
    平均成本，⛔ 不是全部進場成交的加權均價）。費用用 `t.commission`（分攤到這筆交易的金額），並核對它等於
    這筆的 Σ event.commission——⛔ 不能用委託的 `order.executed.comm`（委託累計；反手的委託同時涵蓋平倉與開倉）。
    """
    events = [h.event for h in t.history]
    dts = [h.status.dt for h in t.history]
    if not events:
        raise TradeExtractionError(f"trade {t.ref}: no history (tradehistory disabled?)")
    # 先確認要用到的數值都是有限數，才做任何運算：NaN 的大小比較一律不成立，後面的核對會被繞過
    values = [("pnl", t.pnl), ("pnlcomm", t.pnlcomm), ("commission", t.commission)]
    for i, (ev, dt) in enumerate(zip(events, dts)):
        values += [(f"event[{i}].size", ev.size), (f"event[{i}].price", ev.price),
                   (f"event[{i}].commission", ev.commission), (f"event[{i}].dt", dt)]
    for name, v in values:
        if not math.isfinite(v):
            raise TradeExtractionError(f"trade {t.ref}: {name} is not finite ({v})")
    sign = 1 if events[0].size > 0 else -1
    pos = 0.0
    open_qty = open_amt = close_qty = close_amt = 0.0
    for ev in events:
        pos += ev.size
        if pos * sign < -_FLOAT_TOL:
            raise TradeExtractionError(f"trade {t.ref}: position changed sign within one trade")
        if ev.size * sign > 0:
            open_qty += abs(ev.size)
            open_amt += abs(ev.size) * ev.price
        else:
            close_qty += abs(ev.size)
            close_amt += abs(ev.size) * ev.price
    if abs(pos) > _FLOAT_TOL or open_qty <= 0 or close_qty <= 0:
        raise TradeExtractionError(f"trade {t.ref}: position does not return to 0")
    gross = sign * (close_amt - open_amt)
    if abs(gross - t.pnl) > _FLOAT_TOL:
        raise TradeExtractionError(f"trade {t.ref}: gross pnl {gross} != backtrader pnl {t.pnl}")
    comm_events = sum(ev.commission for ev in events)
    if abs(comm_events - t.commission) > _FLOAT_TOL:
        raise TradeExtractionError(
            f"trade {t.ref}: commission {t.commission} != sum of event commission {comm_events}")
    # 進場時刻＝第一筆成交；出場時刻＝最後一筆成交（都是出場方向，因為部位最後回到 0）
    return {
        "symbol":      t.data._name,
        "direction":   "BUY" if sign > 0 else "SELL",
        "entry_time":  _bt_time_to_taipei_iso(dts[0]),
        "exit_time":   _bt_time_to_taipei_iso(dts[-1]),
        "entry_price": round(open_amt / open_qty, 2),
        "exit_price":  round(close_amt / close_qty, 2),
        "size":        open_qty,
        "pnl":         round(t.pnlcomm, 2),
        # 淨損益 ÷ 未四捨五入的進場成交金額，比例值（0.01＝1%）
        "pnl_pct":     round(t.pnlcomm / open_amt, 6),
        "commission":  round(t.commission, 2),
    }


def _bt_time_to_taipei_iso(bt_date) -> str | None:
    """backtrader 的時間是 UTC 時刻（`num2date` 回傳⛔ 不帶時區的 UTC）：先標成 UTC、再轉成台北時間，輸出帶 `+08:00`。

    ⛔ 不能直接把不帶時區的值貼上 `+08:00`（會差 8 小時）。帶 offset 的字串寫進 PostgreSQL 的 TIMESTAMPTZ 時，
    存的時刻跟連線的 TimeZone 無關；⚠️ MySQL 的 DATETIME 仍會換成連線的時區，SQLite 存字串本身。
    """
    if bt_date is None:
        return None
    dt = bt.num2date(bt_date).replace(tzinfo=timezone.utc)
    return dt.astimezone(TAIPEI).isoformat()
