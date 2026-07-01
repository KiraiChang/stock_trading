from __future__ import annotations

import numpy as np

from ..backtester import BacktestEngine
from ..entries.breakout import BreakoutEntry
from ..exits.atr_stop import ATRStopLoss
from ..strategy import TradingStrategy, build_strategy
from ..support_resistance.swing_points import SwingHighLowSR
from ..types import Direction, ExitReason
from .conftest import make_df


def _breakout_then_crash_df() -> "pd.DataFrame":  # noqa: F821 (type hint only, pandas imported via make_df)
    n = 35
    step, amp = 0.6, 1.0
    x = np.arange(n)
    closes = 100 + step * x + amp * np.sin(x)
    highs = closes + 0.8
    lows = closes - 0.8
    opens = closes - 0.1
    volumes = np.full(n, 1000.0)

    # bar 31：帶量突破（用來觸發 BreakoutEntry；下一根 bar32 開盤成交）
    closes[31] = closes[30] + 20
    highs[31] = closes[31] + 0.5
    lows[31] = closes[30]
    opens[31] = closes[30] + 0.2
    volumes[31] = 6000.0

    # bar32（成交當根，之後才開始檢查停損）維持小幅整理
    opens[32] = closes[31] + 0.1
    closes[32] = closes[31] + 0.2
    highs[32] = closes[32] + 0.3
    lows[32] = closes[32] - 0.3

    # bar33：大幅崩跌，跌破 ATR 停損
    opens[33] = closes[32] - 0.1
    closes[33] = closes[32] - 15
    highs[33] = opens[33] + 0.2
    lows[33] = closes[33] - 0.2

    opens[34] = closes[33]
    closes[34] = closes[33] + 0.1
    highs[34] = closes[34] + 0.3
    lows[34] = closes[34] - 0.3

    rows = list(zip(opens, highs, lows, closes, volumes))
    return make_df(rows)


def test_backtest_engine_records_breakout_entry_and_stop_loss_exit():
    df = _breakout_then_crash_df()
    strategy = TradingStrategy(
        name="test_breakout_atr",
        sr_strategy=SwingHighLowSR(lookback=60),
        entry_strategy=BreakoutEntry(vol_multiplier=2.0, vol_period=20),
        stop_loss_strategy=ATRStopLoss(atr_period=14, atr_multiplier=2.0),
    )
    engine = BacktestEngine(initial_cash=1_000_000.0, commission_rate=0.001425, tax_rate=0.003)

    report = engine.run("TEST", df, strategy)

    assert report.total_trades == 1, "應該恰好觸發一筆交易"
    trade = report.trades[0]
    assert trade.direction == Direction.LONG
    assert trade.entry_time == df.index[32], "訊號在 bar31 產生，應在 bar32 開盤成交"
    assert trade.entry_price == df["open"].iloc[32]
    assert trade.exit_reason == ExitReason.STOP_LOSS
    assert trade.pnl < 0, "崩跌後停損出場應為虧損"
    assert report.loss_trades == 1
    assert report.win_trades == 0
    assert report.avg_pnl == trade.pnl


def test_backtest_engine_no_data_returns_empty_report():
    df = _breakout_then_crash_df().iloc[:5]
    strategy = TradingStrategy(
        name="test_insufficient_data",
        sr_strategy=SwingHighLowSR(),
        entry_strategy=BreakoutEntry(),
        stop_loss_strategy=ATRStopLoss(),
    )
    report = BacktestEngine().run("TEST", df, strategy)
    assert report.total_trades == 0
    assert report.trades == []


def test_build_strategy_unknown_name_raises():
    import pytest

    with pytest.raises(ValueError):
        build_strategy("does_not_exist")


def test_build_strategy_known_presets_are_constructible():
    from ..strategy import STRATEGY_PRESETS

    for name in STRATEGY_PRESETS:
        strategy = build_strategy(name)
        assert strategy.name == name
