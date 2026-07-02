from __future__ import annotations

import pytest

from ..features import (
    average_bounce_break_returns,
    compute_zone_features,
    count_breakouts,
    count_rejections,
    find_touches,
    relative_volume_at_touches,
    touch_starting_at,
    trend_slope,
    zone_momentum,
    zone_volatility,
)
from ..types import ApproachDirection, Zone, ZoneMethod
from .conftest import bearish_trend_df, bullish_trend_df, double_bottom_df, make_df


def _tight_support_zone(support: float = 100.0) -> Zone:
    return Zone(
        price_low=support - 0.1, price_high=support + 0.1, method=ZoneMethod.ATR,
        center_price=support, formed_at_index=0,
    )


def _bounce_then_break_df(support: float = 100.0) -> "pd.DataFrame":
    """兩次觸碰同一個支撐位：第一次強力反彈（守住），第二次跌破，兩次之間
    價格都遠離 zone 邊界，確保不會被誤判成同一次觸碰的延續。用來驗證
    average_bounce_return/average_break_return 真的有分開統計。"""
    closes = [
        110, 108, 104, 101, support,  # touch 1（idx4）：探底
        105, 110, 115, 120, 125,  # 強力反彈，遠離 zone
        120, 115, 110, 105, support,  # touch 2（idx14）：再次探底
        95, 90, 85, 80, 75,  # 這次跌破，持續破底
    ]
    rows = [(c + 0.02, c + 0.05, c - 0.05, c, 1000.0) for c in closes]
    return make_df(rows)


def test_find_touches_counts_double_bottom():
    df = double_bottom_df()
    zone = _tight_support_zone()
    touches = find_touches(df, zone, as_of_index=len(df) - 1, lookback_bars=len(df))
    assert len(touches) == 2
    assert all(t.approach_direction == ApproachDirection.FROM_ABOVE for t in touches)


def test_count_rejections_both_touches_bounce():
    df = double_bottom_df()
    zone = _tight_support_zone()
    touches = find_touches(df, zone, as_of_index=len(df) - 1, lookback_bars=len(df))
    assert count_rejections(df, touches, rejection_window=3, as_of_index=len(df) - 1) == 2


def test_count_breakouts_zero_when_price_never_closes_below_support():
    df = double_bottom_df()
    zone = _tight_support_zone()
    breakouts = count_breakouts(
        df, zone, as_of_index=len(df) - 1, lookback_bars=len(df), confirmation_bars=2,
        approach=ApproachDirection.FROM_ABOVE,
    )
    assert breakouts == 0


def test_count_breakouts_detects_sustained_close_below_support():
    closes = [105.0, 103.0, 101.0, 99.0, 98.0, 97.0, 102.0, 106.0]
    rows = [(c, c + 0.1, c - 0.1, c, 1000.0) for c in closes]
    df = make_df(rows)
    zone = Zone(price_low=100.0, price_high=100.5, method=ZoneMethod.ATR, center_price=100.25, formed_at_index=0)
    breakouts = count_breakouts(
        df, zone, as_of_index=len(df) - 1, lookback_bars=len(df), confirmation_bars=2,
        approach=ApproachDirection.FROM_ABOVE,
    )
    assert breakouts == 1


def test_count_breakouts_ignores_moves_on_the_normal_side_of_the_zone():
    # 支撐 zone：價格待在 zone 上方是正常狀態，不該被算成突破
    closes = [105.0, 110.0, 115.0, 120.0, 125.0]
    rows = [(c, c + 0.1, c - 0.1, c, 1000.0) for c in closes]
    df = make_df(rows)
    zone = Zone(price_low=100.0, price_high=100.5, method=ZoneMethod.ATR, center_price=100.25, formed_at_index=0)
    breakouts = count_breakouts(
        df, zone, as_of_index=len(df) - 1, lookback_bars=len(df), confirmation_bars=2,
        approach=ApproachDirection.FROM_ABOVE,
    )
    assert breakouts == 0


def test_average_bounce_break_returns_separates_hold_and_break():
    df = _bounce_then_break_df()
    zone = _tight_support_zone()
    touches = find_touches(df, zone, as_of_index=len(df) - 1, lookback_bars=len(df))
    assert len(touches) == 2

    bounce, brk = average_bounce_break_returns(
        df, touches, forward_bars=3, threshold_pct=0.03, as_of_index=len(df) - 1
    )
    assert bounce > 0.1  # 第一次觸碰強力反彈
    assert brk < -0.1  # 第二次觸碰跌破


def test_average_bounce_break_returns_excludes_touches_without_enough_future_bars():
    df = double_bottom_df()
    zone = _tight_support_zone()
    touches = find_touches(df, zone, as_of_index=len(df) - 1, lookback_bars=len(df))
    # as_of_index=14 剛好是第二次觸碰當下，該次觸碰沒有 forward_bars=3 的未來資料可用，
    # 只有第一次觸碰（index=4）會被納入計算，且第一次是反彈
    bounce, brk = average_bounce_break_returns(
        df, touches, forward_bars=3, threshold_pct=0.03, as_of_index=14
    )
    assert bounce > 0
    assert brk == 0.0


def test_relative_volume_at_touches_uses_trailing_ma_excluding_touch_bar():
    closes = [105.0] * 20 + [100.0]
    volumes = [1000.0] * 20 + [3000.0]
    rows = [(c, c + 0.1, c - 0.1, c, v) for c, v in zip(closes, volumes)]
    df = make_df(rows)
    zone = Zone(price_low=99.9, price_high=100.1, method=ZoneMethod.ATR, center_price=100.0, formed_at_index=0)
    touches = find_touches(df, zone, as_of_index=len(df) - 1, lookback_bars=len(df))
    assert len(touches) == 1
    rel_vol = relative_volume_at_touches(df, touches, volume_ma_period=20)
    assert rel_vol == pytest.approx(3.0)


def test_zone_volatility_positive_for_volatile_series():
    df = double_bottom_df()
    vol = zone_volatility(df, as_of_index=len(df) - 1, atr_period=14)
    assert vol > 0


def test_trend_slope_positive_for_uptrend():
    df = bullish_trend_df(n=80)
    slope = trend_slope(df, as_of_index=len(df) - 1, ma_period=20, lookback=20)
    assert slope > 0


def test_trend_slope_negative_for_downtrend():
    df = bearish_trend_df(n=80)
    slope = trend_slope(df, as_of_index=len(df) - 1, ma_period=20, lookback=20)
    assert slope < 0


def test_zone_momentum_negative_when_price_falls_into_zone():
    df = _bounce_then_break_df()
    zone = _tight_support_zone()
    touches = find_touches(df, zone, as_of_index=len(df) - 1, lookback_bars=len(df))
    momentum = zone_momentum(df, touches, lookback=4)
    assert momentum < 0  # 兩次觸碰前價格都是下跌趨向這個支撐位


def test_zone_momentum_zero_when_no_touches():
    df = bullish_trend_df(n=40)
    assert zone_momentum(df, []) == 0.0


def test_touch_starting_at_none_for_continuation_bar():
    df = double_bottom_df()
    zone = _tight_support_zone()
    assert touch_starting_at(df, zone, 4) is not None
    # index 5 已經離開 zone，根本不算觸碰
    assert touch_starting_at(df, zone, 5) is None


def test_compute_zone_features_role_specific_stats():
    df = double_bottom_df()
    zone = _tight_support_zone()

    features_support = compute_zone_features(df, zone, as_of_index=len(df) - 1, approach=ApproachDirection.FROM_ABOVE)
    assert features_support.touch_count == 2
    assert features_support.rejection_count == 2
    assert features_support.breakout_count == 0
    assert features_support.average_bounce_return > 0
    assert features_support.average_break_return == 0.0

    # 這個 zone 從未被「由下往上」觸碰過，FROM_BELOW 的 rejection 應為 0
    features_resistance = compute_zone_features(df, zone, as_of_index=len(df) - 1, approach=ApproachDirection.FROM_BELOW)
    assert features_resistance.touch_count == 2  # touch_count 是聚合值，不分方向
    assert features_resistance.rejection_count == 0
