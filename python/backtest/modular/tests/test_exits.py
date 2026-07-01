from __future__ import annotations

from ...indicators import calc_atr
from ..exits.atr_stop import ATRStopLoss
from ..exits.composite import CompositeStopLoss
from ..exits.structural_stop import StructuralStopLoss
from ..types import Direction, Position
from .conftest import bullish_trend_df, make_df


def test_atr_stop_initial_stop_below_entry_for_long(bullish_df):
    entry_price = float(bullish_df["close"].iloc[-1])
    stop_loss = ATRStopLoss(atr_period=14, atr_multiplier=2.0)
    stop = stop_loss.initial_stop(bullish_df, Direction.LONG, entry_price)

    expected_atr = calc_atr(
        bullish_df["high"].to_numpy(), bullish_df["low"].to_numpy(), bullish_df["close"].to_numpy(), 14
    )
    assert stop == entry_price - 2.0 * expected_atr
    assert stop < entry_price


def test_atr_stop_initial_stop_above_entry_for_short(bullish_df):
    entry_price = float(bullish_df["close"].iloc[-1])
    stop_loss = ATRStopLoss(atr_period=14, atr_multiplier=2.0)
    stop = stop_loss.initial_stop(bullish_df, Direction.SHORT, entry_price)
    assert stop > entry_price


def test_atr_stop_does_not_move_after_entry(bullish_df):
    entry_price = float(bullish_df["close"].iloc[-2])
    stop_loss = ATRStopLoss()
    initial = stop_loss.initial_stop(bullish_df.iloc[:-1], Direction.LONG, entry_price)
    position = Position(Direction.LONG, entry_index=0, entry_time=bullish_df.index[-2], entry_price=entry_price, stop_price=initial)
    updated = stop_loss.update(bullish_df, position)
    assert updated == initial


def test_structural_stop_only_ratchets_up_for_long():
    # lows 依序為 [99, 97, 100, 103, 100, 105, 108, 98, 106]
    # 確認過的 swing low（pivot_window=1）依序是 idx1=97 → idx4=100 → idx7=98
    # 第二個比第一個高（應推升停損），第三個比第二個低（不應讓停損下修）
    lows = [99, 97, 100, 103, 100, 105, 108, 98, 106]
    rows = [(low + 2, low + 5, low, low + 3, 1000) for low in lows]
    df = make_df(rows)
    stop_loss = StructuralStopLoss(pivot_window=1, lookback=len(df))

    entry_price = float(df["close"].iloc[1])
    # 只看到 idx0~2，此時唯一已確認的 swing low 是 idx1=97
    stop_after_first_low = stop_loss.initial_stop(df.iloc[:3], Direction.LONG, entry_price)
    assert stop_after_first_low == 97.0

    position = Position(Direction.LONG, entry_index=1, entry_time=df.index[1], entry_price=entry_price, stop_price=stop_after_first_low)

    # 看到 idx0~5，新確認的 swing low 是 idx4=100（比 97 高）→ 應推升停損
    stop_after_second_low = stop_loss.update(df.iloc[:6], position)
    assert stop_after_second_low == 100.0
    assert stop_after_second_low > stop_after_first_low
    position.stop_price = stop_after_second_low

    # 看到全部 9 根，新確認的 swing low 是 idx7=98（比 100 低）→ 不應讓停損下修
    stop_after_lower_low = stop_loss.update(df.iloc[:9], position)
    assert stop_after_lower_low == stop_after_second_low, "較低的新 swing low 不應讓停損下修"


def test_composite_stop_is_tighter_of_atr_and_structural(bullish_df):
    entry_price = float(bullish_df["close"].iloc[-1])
    atr_stop = ATRStopLoss(atr_period=14, atr_multiplier=2.0)
    structural_stop = StructuralStopLoss(pivot_window=1, lookback=len(bullish_df))
    composite = CompositeStopLoss(atr_stop=atr_stop, structural_stop=structural_stop)

    atr_level = atr_stop.initial_stop(bullish_df, Direction.LONG, entry_price)
    structural_level = structural_stop.initial_stop(bullish_df, Direction.LONG, entry_price)
    composite_level = composite.initial_stop(bullish_df, Direction.LONG, entry_price)

    assert composite_level == max(atr_level, structural_level)
