from __future__ import annotations

from ..labeling import label_touch
from ..types import ApproachDirection, Zone, ZoneMethod, ZoneTouch, ZoneType
from .conftest import make_df


def _touch(touch_index: int, role: ZoneType, touch_price: float = 100.0) -> ZoneTouch:
    zone = Zone(price_low=99.5, price_high=100.5, method=ZoneMethod.ATR, center_price=100.0, formed_at_index=0)
    direction = ApproachDirection.FROM_ABOVE if role == ZoneType.SUPPORT else ApproachDirection.FROM_BELOW
    return ZoneTouch(
        zone=zone, touch_index=touch_index, touch_time=None, touch_price=touch_price,
        approach_direction=direction, role=role,
    )


def test_support_hold_label_when_price_bounces():
    closes = [100.0, 101.0, 103.0, 106.0, 108.0, 110.0]
    rows = [(c, c + 0.2, c - 0.2, c, 1000.0) for c in closes]
    df = make_df(rows)
    touch = _touch(0, ZoneType.SUPPORT)

    result = label_touch(df, touch, forward_bars=3, threshold_pct=0.03, method="max_excursion")

    assert result is not None
    hold_label, break_label, forward_return = result
    assert hold_label == 1
    assert break_label == 0
    assert forward_return > 0


def test_support_break_label_when_price_falls_through():
    closes = [100.0, 98.0, 95.0, 92.0, 90.0, 88.0]
    rows = [(c, c + 0.2, c - 0.2, c, 1000.0) for c in closes]
    df = make_df(rows)
    touch = _touch(0, ZoneType.SUPPORT)

    result = label_touch(df, touch, forward_bars=3, threshold_pct=0.03, method="max_excursion")

    assert result is not None
    hold_label, break_label, forward_return = result
    assert hold_label == 0
    assert break_label == 1
    assert forward_return < 0


def test_resistance_labels_are_mirrored_relative_to_support():
    closes = [100.0, 102.0, 104.0, 106.0, 108.0, 110.0]
    rows = [(c, c + 0.2, c - 0.2, c, 1000.0) for c in closes]
    df = make_df(rows)
    touch = _touch(0, ZoneType.RESISTANCE)

    result = label_touch(df, touch, forward_bars=3, threshold_pct=0.03, method="max_excursion")

    assert result is not None
    hold_label, break_label, forward_return = result
    assert hold_label == 0  # 壓力要「跌」才算 hold，這裡是漲 → 不算
    assert break_label == 1  # 明確漲破 threshold → 視為壓力突破
    assert forward_return < 0  # forward_return 對壓力角色方向取反


def test_label_touch_returns_none_when_not_enough_future_bars():
    closes = [100.0, 101.0, 102.0]
    rows = [(c, c + 0.2, c - 0.2, c, 1000.0) for c in closes]
    df = make_df(rows)
    touch = _touch(1, ZoneType.SUPPORT)

    result = label_touch(df, touch, forward_bars=5, threshold_pct=0.03)

    assert result is None


def test_close_at_n_method_uses_simple_forward_return():
    closes = [100.0, 100.0, 100.0, 104.0]
    rows = [(c, c + 0.2, c - 0.2, c, 1000.0) for c in closes]
    df = make_df(rows)
    touch = _touch(0, ZoneType.SUPPORT)

    result = label_touch(df, touch, forward_bars=3, threshold_pct=0.03, method="close_at_n")

    assert result is not None
    hold_label, break_label, forward_return = result
    assert hold_label == 1  # (104-100)/100 = 0.04 > 0.03
    assert break_label == 0
    assert forward_return > 0
