from __future__ import annotations

from ..entries.breakout import BreakoutEntry
from ..entries.pullback import PullbackSupportEntry
from ..types import Direction, Level, LevelType, SRLevels
from .conftest import bearish_trend_df, bullish_trend_df


def test_breakout_entry_triggers_long_on_confirmed_breakout(bullish_df):
    df = bullish_df.copy()
    prior_close = float(df["close"].iloc[-2])
    avg_volume = float(df["volume"].iloc[:-1].mean())

    # 最後一根大漲並帶量突破先前收盤價（當作壓力位）
    df.iloc[-1, df.columns.get_loc("close")] = prior_close + 10
    df.iloc[-1, df.columns.get_loc("high")] = prior_close + 10.5
    df.iloc[-1, df.columns.get_loc("volume")] = avg_volume * 5

    levels = SRLevels(resistances=[Level(prior_close, LevelType.RESISTANCE, 1.0, "test")])
    signal = BreakoutEntry(vol_multiplier=2.0, vol_period=20).evaluate(df, levels)

    assert signal is not None
    assert signal.direction == Direction.LONG
    assert signal.reference_level == prior_close


def test_breakout_entry_no_signal_without_volume_confirmation(bullish_df):
    df = bullish_df.copy()
    prior_close = float(df["close"].iloc[-2])
    df.iloc[-1, df.columns.get_loc("close")] = prior_close + 10
    df.iloc[-1, df.columns.get_loc("high")] = prior_close + 10.5
    # volume 維持原樣（沒有爆量）

    levels = SRLevels(resistances=[Level(prior_close, LevelType.RESISTANCE, 1.0, "test")])
    signal = BreakoutEntry(vol_multiplier=2.0, vol_period=20).evaluate(df, levels)
    assert signal is None


def test_breakout_entry_no_long_signal_when_trend_not_bullish(bearish_df):
    df = bearish_df.copy()
    prior_close = float(df["close"].iloc[-2])
    avg_volume = float(df["volume"].iloc[:-1].mean())
    df.iloc[-1, df.columns.get_loc("close")] = prior_close + 10
    df.iloc[-1, df.columns.get_loc("high")] = prior_close + 10.5
    df.iloc[-1, df.columns.get_loc("volume")] = avg_volume * 5

    levels = SRLevels(resistances=[Level(prior_close, LevelType.RESISTANCE, 1.0, "test")])
    signal = BreakoutEntry(vol_multiplier=2.0, vol_period=20).evaluate(df, levels)
    assert signal is None


def test_pullback_entry_triggers_on_held_support(bullish_df):
    df = bullish_df.copy()
    support_price = float(df["low"].iloc[-1]) * 1.001  # 在容忍帶內，略高於當根最低價
    df.iloc[-1, df.columns.get_loc("open")] = support_price - 0.2
    df.iloc[-1, df.columns.get_loc("close")] = support_price + 0.5  # 收在支撐之上且是陽線

    levels = SRLevels(supports=[Level(support_price, LevelType.SUPPORT, 1.0, "test")])
    signal = PullbackSupportEntry(tolerance_pct=0.01).evaluate(df, levels)

    assert signal is not None
    assert signal.direction == Direction.LONG
    assert signal.reference_level == support_price


def test_pullback_entry_no_signal_when_support_breaks(bullish_df):
    df = bullish_df.copy()
    support_price = float(df["low"].iloc[-1]) * 1.001
    df.iloc[-1, df.columns.get_loc("open")] = support_price + 0.5
    df.iloc[-1, df.columns.get_loc("close")] = support_price - 0.5  # 收在支撐之下，跌破而非守住

    levels = SRLevels(supports=[Level(support_price, LevelType.SUPPORT, 1.0, "test")])
    signal = PullbackSupportEntry(tolerance_pct=0.01).evaluate(df, levels)
    assert signal is None


def test_pullback_entry_requires_bullish_trend(bearish_df):
    df = bearish_df.copy()
    support_price = float(df["low"].iloc[-1]) * 1.001
    df.iloc[-1, df.columns.get_loc("open")] = support_price - 0.2
    df.iloc[-1, df.columns.get_loc("close")] = support_price + 0.5

    levels = SRLevels(supports=[Level(support_price, LevelType.SUPPORT, 1.0, "test")])
    signal = PullbackSupportEntry(tolerance_pct=0.01).evaluate(df, levels)
    assert signal is None
