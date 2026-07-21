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
    n = 37
    step, amp = 0.6, 1.0
    x = np.arange(n)
    closes = 100 + step * x + amp * np.sin(x)
    highs = closes + 0.8
    lows = closes - 0.8
    opens = closes - 0.1
    volumes = np.full(n, 1000.0)

    # bar29：建立一個明確的 swing high 壓力；bar30 收盤仍在壓力下方
    highs[29] = 120.0

    # bar 31：帶量突破 120 壓力；BreakoutEntry 需要後續 2 根確認，所以訊號會在 bar33 收盤後產生
    closes[31] = closes[30] + 20
    highs[31] = closes[31] + 0.5
    lows[31] = closes[30]
    opens[31] = closes[30] + 0.2
    volumes[31] = 6000.0

    # bar32、bar33：連續兩根站穩突破壓力上方，bar33 收盤後確認訊號
    opens[32] = closes[31] + 0.1
    closes[32] = closes[31] + 0.2
    highs[32] = closes[32] + 0.3
    lows[32] = closes[32] - 0.3

    opens[33] = closes[32] + 0.1
    closes[33] = closes[32] + 0.2
    highs[33] = closes[33] + 0.3
    lows[33] = closes[33] - 0.3

    # bar34：成交當根，之後才開始檢查停損
    opens[34] = closes[33] + 0.1
    closes[34] = closes[33] + 0.2
    highs[34] = closes[34] + 0.3
    lows[34] = closes[34] - 0.3

    # bar35：大幅崩跌，跌破 ATR 停損
    opens[35] = closes[34] - 0.1
    closes[35] = closes[34] - 15
    highs[35] = opens[35] + 0.2
    lows[35] = closes[35] - 0.2

    opens[36] = closes[35]
    closes[36] = closes[35] + 0.1
    highs[36] = closes[36] + 0.3
    lows[36] = closes[36] - 0.3

    rows = list(zip(opens, highs, lows, closes, volumes))
    return make_df(rows)


def test_backtest_engine_records_breakout_entry_and_stop_loss_exit():
    df = _breakout_then_crash_df()
    strategy = TradingStrategy(
        name="test_breakout_atr",
        sr_strategy=SwingHighLowSR(lookback=60, max_levels=5),
        entry_strategy=BreakoutEntry(vol_multiplier=2.0, vol_period=20),
        stop_loss_strategy=ATRStopLoss(atr_period=14, atr_multiplier=2.0),
    )
    engine = BacktestEngine(initial_cash=1_000_000.0, commission_rate=0.001425, tax_rate=0.003)

    report = engine.run("TEST", df, strategy)

    assert report.total_trades == 1, "應該恰好觸發一筆交易"
    trade = report.trades[0]
    assert trade.direction == Direction.LONG
    assert trade.entry_time == df.index[34], "訊號在 bar33 確認，應在 bar34 開盤成交"
    assert trade.entry_price == df["open"].iloc[34]
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


# ── 【2026-07 籌碼分析整合】chip_min_score filter ──────────────────────


def _strategy_for_chip_tests() -> TradingStrategy:
    return TradingStrategy(
        name="test_breakout_atr_chip",
        sr_strategy=SwingHighLowSR(lookback=60, max_levels=5),
        entry_strategy=BreakoutEntry(vol_multiplier=2.0, vol_period=20),
        stop_loss_strategy=ATRStopLoss(atr_period=14, atr_multiplier=2.0),
    )


def test_backtest_engine_chip_filter_blocks_entry_below_threshold():
    df = _breakout_then_crash_df()
    signal_date = df.index[33].strftime("%Y-%m-%d")  # 訊號在 bar33 確認產生
    engine = BacktestEngine(chip_min_score=50.0)

    report = engine.run("TEST", df, _strategy_for_chip_tests(), chip_scores={signal_date: 10.0})

    assert report.total_trades == 0, "籌碼分數低於門檻應濾掉進場訊號"


def test_backtest_engine_chip_filter_allows_entry_meeting_threshold():
    df = _breakout_then_crash_df()
    signal_date = df.index[33].strftime("%Y-%m-%d")
    engine = BacktestEngine(chip_min_score=50.0)

    report = engine.run("TEST", df, _strategy_for_chip_tests(), chip_scores={signal_date: 60.0})

    assert report.total_trades == 1, "籌碼分數達到門檻應放行進場訊號"


def test_backtest_engine_chip_filter_missing_date_treated_as_neutral_zero():
    """【review 修復】缺籌碼資料不再 fail-open 直接放行，而是視為中性分數
    0 分下去跟門檻比較。門檻 > 0 時，缺資料的訊號會被濾掉（等同於 0 分
    沒有達到門檻），避免資料庫籌碼資料不全時 filter 形同沒開。"""
    df = _breakout_then_crash_df()
    engine = BacktestEngine(chip_min_score=50.0)

    report = engine.run("TEST", df, _strategy_for_chip_tests(), chip_scores={})

    assert report.total_trades == 0, "缺籌碼資料應視為中性 0 分，未達正門檻應濾掉進場"


def test_backtest_engine_chip_filter_missing_date_passes_when_threshold_non_positive():
    """門檻 <= 0 時，中性 0 分仍然達標，缺資料的訊號應該放行——這樣「不設
    門檻」（chip_min_score=0）的行為才會等同於沒有實際限制。"""
    df = _breakout_then_crash_df()
    engine = BacktestEngine(chip_min_score=0.0)

    report = engine.run("TEST", df, _strategy_for_chip_tests(), chip_scores={})

    assert report.total_trades == 1, "門檻為 0 時，中性 0 分應該放行"


def test_backtest_engine_chip_filter_none_chip_scores_treated_as_neutral_zero():
    """chip_scores 整個是 None（呼叫端沒有提供任何籌碼資料）也要走同一套
    中性 0 分邏輯，不能因為 dict 是 None 就繞過 filter。"""
    df = _breakout_then_crash_df()
    engine = BacktestEngine(chip_min_score=50.0)

    report = engine.run("TEST", df, _strategy_for_chip_tests(), chip_scores=None)

    assert report.total_trades == 0, "chip_scores=None 也應視為中性 0 分，未達正門檻應濾掉進場"


def test_backtest_engine_chip_filter_disabled_ignores_chip_scores():
    df = _breakout_then_crash_df()
    signal_date = df.index[33].strftime("%Y-%m-%d")
    engine = BacktestEngine()  # chip_min_score 預設 None = 停用 filter

    report = engine.run("TEST", df, _strategy_for_chip_tests(), chip_scores={signal_date: -100.0})

    assert report.total_trades == 1, "未啟用籌碼 filter 時，即使傳入極低分數也不應影響進場"


def test_build_strategy_unknown_name_raises():
    import pytest

    with pytest.raises(ValueError):
        build_strategy("does_not_exist")


def test_build_strategy_known_presets_are_constructible():
    from ..strategy import STRATEGY_PRESETS

    for name in STRATEGY_PRESETS:
        strategy = build_strategy(name)
        assert strategy.name == name
