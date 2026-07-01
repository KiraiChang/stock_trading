"""
事件驅動（bar-by-bar）回測引擎。

輸入：OHLCV DataFrame（index 為時間、需含 open/high/low/close/volume 欄位，
      依時間升冪排列）+ 一個 TradingStrategy。
輸出：BacktestReport（總覽指標 + 逐筆 Trade）。

核心規則（避免 lookahead bias、貼近真實成交行為）：
    1. 訊號在 bar t 收盤後產生，實際成交在 bar t+1 的開盤價（無法用未來資訊
       決定「現在」的成交價）。
    2. 停損在 bar t 收盤後依當時可得資訊重新計算，若 bar t 的高/低價觸及
       停損，視為當根K棒觸發出場（用停損價成交；若開盤已跳空穿越停損，
       改用開盤價成交，避免產生不切實際的成交價）。
    3. 進場當根K棒不做停損檢查（下一根才開始），避免「同一根K棒內先進場
       又立刻出場」的模糊情況；這與本系統「非高頻」的定位一致
       （見專案 CLAUDE.md：Non-HFT / Not low-latency）。
    4. 全額資金單一部位（同一時間只持有 0 或 1 個部位），不做加碼/分批。
"""
from __future__ import annotations

import numpy as np
import pandas as pd

from .strategy import TradingStrategy
from .types import BacktestReport, Direction, ExitReason, Position, Trade

TRADING_DAYS_PER_YEAR = 252


class BacktestEngine:
    def __init__(
        self,
        initial_cash: float = 1_000_000.0,
        commission_rate: float = 0.001425,
        tax_rate: float = 0.003,
    ) -> None:
        self.initial_cash = initial_cash
        self.commission_rate = commission_rate
        self.tax_rate = tax_rate

    def run(self, symbol: str, df: pd.DataFrame, strategy: TradingStrategy) -> BacktestReport:
        df = df.sort_index()
        n = len(df)
        warmup = strategy.min_bars
        if n <= warmup + 1:
            return _empty_report(strategy.name)

        equity = self.initial_cash
        equity_at_entry = equity
        equity_curve: list[tuple[object, float]] = []
        trades: list[Trade] = []

        position: Position | None = None
        pending_signal = None

        for t in range(warmup, n):
            bar = df.iloc[t]
            history = df.iloc[: t + 1]

            if position is None and pending_signal is not None:
                entry_price = float(bar["open"])
                stop = strategy.stop_loss_strategy.initial_stop(history, pending_signal.direction, entry_price)
                position = Position(
                    direction=pending_signal.direction,
                    entry_index=t,
                    entry_time=df.index[t],
                    entry_price=entry_price,
                    stop_price=stop,
                )
                equity_at_entry = equity
                pending_signal = None

            elif position is not None:
                position.stop_price = strategy.stop_loss_strategy.update(history, position)
                stopped_out = (
                    bar["low"] <= position.stop_price
                    if position.direction == Direction.LONG
                    else bar["high"] >= position.stop_price
                )
                if stopped_out:
                    exit_price = self._stop_fill_price(bar, position)
                    trade, equity = self._close(
                        symbol, position, df.index[t], exit_price, ExitReason.STOP_LOSS, equity_at_entry
                    )
                    trades.append(trade)
                    position = None

            else:  # 空手且沒有等待成交的訊號 → 評估是否要進場
                levels = strategy.sr_strategy.calculate(history)
                signal = strategy.entry_strategy.evaluate(history, levels)
                if signal is not None and t + 1 < n:
                    pending_signal = signal

            mtm = self._mark_to_market(equity, equity_at_entry, position, float(bar["close"]))
            equity_curve.append((df.index[t], mtm))

        if position is not None:
            last = df.iloc[-1]
            trade, equity = self._close(
                symbol, position, df.index[-1], float(last["close"]), ExitReason.EOD_FORCE_CLOSE, equity_at_entry
            )
            trades.append(trade)
            equity_curve[-1] = (df.index[-1], equity)

        return _build_report(strategy.name, self.initial_cash, equity, equity_curve, trades)

    @staticmethod
    def _stop_fill_price(bar: pd.Series, position: Position) -> float:
        open_price = float(bar["open"])
        if position.direction == Direction.LONG:
            return min(open_price, position.stop_price)
        return max(open_price, position.stop_price)

    @staticmethod
    def _mark_to_market(equity: float, equity_at_entry: float, position: Position | None, close: float) -> float:
        if position is None:
            return equity
        if position.direction == Direction.LONG:
            return equity_at_entry * (close / position.entry_price)
        return equity_at_entry * (2.0 - close / position.entry_price)

    def _close(
        self,
        symbol: str,
        position: Position,
        exit_time: object,
        exit_price: float,
        exit_reason: ExitReason,
        equity_at_entry: float,
    ) -> tuple[Trade, float]:
        size = equity_at_entry / position.entry_price
        entry_notional = size * position.entry_price
        exit_notional = size * exit_price

        entry_fee = entry_notional * self.commission_rate
        exit_fee = exit_notional * self.commission_rate
        if position.direction == Direction.SHORT:
            entry_fee += entry_notional * self.tax_rate  # 放空＝賣出在前，證交稅在進場時課
        else:
            exit_fee += exit_notional * self.tax_rate  # 做多的賣出發生在出場

        if position.direction == Direction.LONG:
            gross_pnl = (exit_price - position.entry_price) * size
        else:
            gross_pnl = (position.entry_price - exit_price) * size

        total_fee = entry_fee + exit_fee
        net_pnl = gross_pnl - total_fee
        new_equity = equity_at_entry + net_pnl

        trade = Trade(
            symbol=symbol,
            direction=position.direction,
            entry_time=position.entry_time,
            exit_time=exit_time,
            entry_price=position.entry_price,
            exit_price=exit_price,
            size=round(size, 4),
            pnl=round(net_pnl, 4),
            pnl_pct=round(net_pnl / equity_at_entry, 6) if equity_at_entry else 0.0,
            commission=round(total_fee, 4),
            exit_reason=exit_reason,
        )
        return trade, new_equity


def _empty_report(name: str) -> BacktestReport:
    return BacktestReport(
        strategy=name,
        total_return=0.0,
        annual_return=0.0,
        win_rate=0.0,
        max_drawdown=0.0,
        sharpe_ratio=0.0,
        total_trades=0,
        win_trades=0,
        loss_trades=0,
        avg_pnl=0.0,
        trades=[],
    )


def _build_report(
    name: str,
    initial_cash: float,
    final_equity: float,
    equity_curve: list[tuple[object, float]],
    trades: list[Trade],
) -> BacktestReport:
    total_trades = len(trades)
    win_trades = sum(1 for t in trades if t.pnl > 0)
    loss_trades = total_trades - win_trades
    total_return = (final_equity - initial_cash) / initial_cash if initial_cash else 0.0

    equity_series = pd.Series(
        [e for _, e in equity_curve], index=[ts for ts, _ in equity_curve], dtype=float
    )
    daily_returns = equity_series.pct_change().dropna()

    return BacktestReport(
        strategy=name,
        total_return=round(total_return, 6),
        annual_return=round(_annualize(total_return, len(equity_series)), 6),
        win_rate=round(win_trades / total_trades, 4) if total_trades else 0.0,
        max_drawdown=round(_max_drawdown(equity_series), 6),
        sharpe_ratio=round(_sharpe(daily_returns), 4),
        total_trades=total_trades,
        win_trades=win_trades,
        loss_trades=loss_trades,
        avg_pnl=round(sum(t.pnl for t in trades) / total_trades, 4) if total_trades else 0.0,
        trades=trades,
    )


def _annualize(total_return: float, n_bars: int) -> float:
    n_days = max(n_bars - 1, 1)
    base = 1.0 + total_return
    if base <= 0:
        return -1.0
    return base ** (TRADING_DAYS_PER_YEAR / n_days) - 1.0


def _max_drawdown(equity_series: pd.Series) -> float:
    if equity_series.empty:
        return 0.0
    running_max = equity_series.cummax()
    drawdown = (equity_series - running_max) / running_max
    return float(-drawdown.min())


def _sharpe(daily_returns: pd.Series, risk_free_annual: float = 0.0) -> float:
    if daily_returns.empty or daily_returns.std(ddof=0) == 0:
        return 0.0
    excess = daily_returns - risk_free_annual / TRADING_DAYS_PER_YEAR
    return float(excess.mean() / daily_returns.std(ddof=0) * np.sqrt(TRADING_DAYS_PER_YEAR))
