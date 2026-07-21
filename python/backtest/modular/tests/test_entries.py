from __future__ import annotations

from ..entries.breakout import BreakoutEntry
from ..entries.pullback import PullbackSupportEntry
from ..types import Direction, Level, LevelType, SRLevels
from .conftest import bearish_trend_df, bullish_trend_df


def test_breakout_entry_triggers_long_on_confirmed_breakout(bullish_df):
    df = bullish_df.copy()
    avg_volume = float(df["volume"].iloc[-24:-4].mean())

    _set_last_closes(df, [95, 110, 108, 109])
    df.iloc[-3, df.columns.get_loc("volume")] = avg_volume * 5

    levels = SRLevels(resistances=[Level(100.0, LevelType.RESISTANCE, 1.0, "test")])
    signal = BreakoutEntry(vol_multiplier=2.0, vol_period=20).evaluate(df, levels)

    assert signal is not None
    assert signal.direction == Direction.LONG
    assert signal.reference_level == 100.0


def test_breakout_entry_no_signal_without_volume_confirmation(bullish_df):
    df = bullish_df.copy()
    _set_last_closes(df, [95, 110, 108, 109])

    levels = SRLevels(resistances=[Level(100.0, LevelType.RESISTANCE, 1.0, "test")])
    signal = BreakoutEntry(vol_multiplier=2.0, vol_period=20).evaluate(df, levels)
    assert signal is None


def test_breakout_entry_no_long_signal_when_trend_not_bullish(bearish_df):
    df = bearish_df.copy()
    avg_volume = float(df["volume"].iloc[-24:-4].mean())
    _set_last_closes(df, [95, 110, 108, 109])
    df.iloc[-3, df.columns.get_loc("volume")] = avg_volume * 5

    levels = SRLevels(resistances=[Level(100.0, LevelType.RESISTANCE, 1.0, "test")])
    signal = BreakoutEntry(vol_multiplier=2.0, vol_period=20).evaluate(df, levels)
    assert signal is None


def _set_last_closes(df, values):
    """覆寫尾端 N 根的收盤價（不動 high/low，保持 detect_trend 結構不變）。"""
    n = len(values)
    loc = df.columns.get_loc("close")
    for i, v in enumerate(values):
        df.iloc[-n + i, loc] = v


def test_breakout_entry_no_long_on_breakout_candle_before_window(bullish_df):
    df = bullish_df.copy()
    avg_volume = float(df["volume"].iloc[-22:-2].mean())
    _set_last_closes(df, [95, 94, 92, 110])
    df.iloc[-1, df.columns.get_loc("volume")] = avg_volume * 5

    levels = SRLevels(resistances=[Level(100.0, LevelType.RESISTANCE, 1.0, "test")])
    signal = BreakoutEntry(vol_multiplier=2.0, vol_period=20).evaluate(df, levels)
    assert signal is None


def test_breakout_entry_no_long_on_first_confirmation_candle(bullish_df):
    df = bullish_df.copy()
    avg_volume = float(df["volume"].iloc[-23:-3].mean())
    _set_last_closes(df, [95, 93, 110, 108])
    df.iloc[-2, df.columns.get_loc("volume")] = avg_volume * 5

    levels = SRLevels(resistances=[Level(100.0, LevelType.RESISTANCE, 1.0, "test")])
    signal = BreakoutEntry(vol_multiplier=2.0, vol_period=20).evaluate(df, levels)
    assert signal is None


def test_breakout_entry_no_long_when_resistance_lost_in_window(bullish_df):
    df = bullish_df.copy()
    avg_volume = float(df["volume"].iloc[-24:-4].mean())
    _set_last_closes(df, [95, 110, 99, 109])
    df.iloc[-3, df.columns.get_loc("volume")] = avg_volume * 5

    levels = SRLevels(resistances=[Level(100.0, LevelType.RESISTANCE, 1.0, "test")])
    signal = BreakoutEntry(vol_multiplier=2.0, vol_period=20).evaluate(df, levels)
    assert signal is None


def test_breakout_entry_no_long_when_rsi_overbought(bullish_df):
    df = bullish_df.copy()
    avg_volume = float(df["volume"].iloc[-24:-4].mean())
    _set_last_closes(df, [95, 110, 108, 109])
    df.iloc[-3, df.columns.get_loc("volume")] = avg_volume * 5
    df["rsi14"] = 60.0
    df.iloc[-1, df.columns.get_loc("rsi14")] = 80.0

    levels = SRLevels(resistances=[Level(100.0, LevelType.RESISTANCE, 1.0, "test")])
    signal = BreakoutEntry(vol_multiplier=2.0, vol_period=20).evaluate(df, levels)
    assert signal is None


def test_breakout_entry_long_uses_nearest_crossed_resistance(bullish_df):
    df = bullish_df.copy()
    avg_volume = float(df["volume"].iloc[-24:-4].mean())
    _set_last_closes(df, [95, 115, 114, 113])
    df.iloc[-3, df.columns.get_loc("volume")] = avg_volume * 5

    levels = SRLevels(
        resistances=[
            Level(100.0, LevelType.RESISTANCE, 1.0, "test"),
            Level(110.0, LevelType.RESISTANCE, 0.5, "test"),
        ]
    )
    signal = BreakoutEntry(vol_multiplier=2.0, vol_period=20).evaluate(df, levels)

    assert signal is not None
    assert signal.direction == Direction.LONG
    assert signal.reference_level == 110.0


def test_breakout_entry_triggers_short_after_breakdown_confirmation(bearish_df):
    df = bearish_df.copy()
    # close[t-3]=92 仍在支撐 90 之上，t-2 跌破，t-1/t 連續未收回
    _set_last_closes(df, [92, 88, 87, 86])

    levels = SRLevels(supports=[Level(90.0, LevelType.SUPPORT, 1.0, "test")])
    signal = BreakoutEntry(vol_multiplier=2.0, vol_period=20).evaluate(df, levels)

    assert signal is not None
    assert signal.direction == Direction.SHORT
    assert signal.reference_level == 90.0


def test_breakout_entry_no_short_on_breakdown_candle_before_window(bearish_df):
    df = bearish_df.copy()
    # 跌破發生在最新一根（t），確認窗尚未形成 → break_idx 處不成立跨越
    _set_last_closes(df, [95, 94, 92, 88])

    levels = SRLevels(supports=[Level(90.0, LevelType.SUPPORT, 1.0, "test")])
    signal = BreakoutEntry(vol_multiplier=2.0, vol_period=20).evaluate(df, levels)
    assert signal is None


def test_breakout_entry_no_short_on_first_confirmation_candle(bearish_df):
    df = bearish_df.copy()
    # 跌破在 t-1，僅一根確認（t），未滿 2 根 → 不觸發
    _set_last_closes(df, [95, 93, 88, 87])

    levels = SRLevels(supports=[Level(90.0, LevelType.SUPPORT, 1.0, "test")])
    signal = BreakoutEntry(vol_multiplier=2.0, vol_period=20).evaluate(df, levels)
    assert signal is None


def test_breakout_entry_no_short_when_support_recovered_in_window(bearish_df):
    df = bearish_df.copy()
    # t-1 收回站上支撐 90 → 確認窗失敗
    _set_last_closes(df, [92, 88, 91, 86])

    levels = SRLevels(supports=[Level(90.0, LevelType.SUPPORT, 1.0, "test")])
    signal = BreakoutEntry(vol_multiplier=2.0, vol_period=20).evaluate(df, levels)
    assert signal is None


def test_breakout_entry_short_uses_nearest_crossed_support(bearish_df):
    df = bearish_df.copy()
    # 兩個支撐皆被跨越且未收回，應回報 nearest crossed（價位最低者 88）
    _set_last_closes(df, [95, 85, 84, 83])

    levels = SRLevels(
        supports=[
            Level(90.0, LevelType.SUPPORT, 1.0, "test"),
            Level(88.0, LevelType.SUPPORT, 0.5, "test"),
        ]
    )
    signal = BreakoutEntry(vol_multiplier=2.0, vol_period=20).evaluate(df, levels)

    assert signal is not None
    assert signal.direction == Direction.SHORT
    assert signal.reference_level == 88.0


def test_breakout_entry_no_short_signal_when_trend_not_bearish(bullish_df):
    df = bullish_df.copy()
    _set_last_closes(df, [92, 88, 87, 86])

    levels = SRLevels(supports=[Level(90.0, LevelType.SUPPORT, 1.0, "test")])
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
