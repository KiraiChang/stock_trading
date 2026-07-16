from __future__ import annotations

from ..event_engine import (
    EXTREME_VOLUME_THRESHOLD,
    detect_market_events,
    zone_interaction,
)
from ..types import RecentValidation, VolumeConfirmation, ZoneType

# 沿用 decision 測試的 ZoneScore 建構 helper，避免重複整包 dataclass 欄位。
from .test_decision_engine import _zone


def _types(events: list[dict]) -> set[str]:
    return {e["type"] for e in events}


def test_zone_interaction_reports_breakdown_and_penetration():
    z = _zone(low=98.0, high=100.0)

    interaction = zone_interaction(z, current_price=99.0, candle_high=101.0, candle_low=96.0, candle_close=97.0)

    assert interaction["touched"] is True
    assert interaction["closed_below"] is True
    assert interaction["closed_above"] is False
    assert interaction["penetration_pct"] > 0
    assert interaction["price_action_evidence"]["close_relative_to_zone"] == "BELOW_ZONE"
    assert interaction["price_action_evidence"]["reclaim_type"] == "NONE"


def test_zone_interaction_reports_price_action_reclaim_evidence():
    z = _zone(low=98.0, high=100.0)

    interaction = zone_interaction(z, current_price=101.0, candle_high=102.0, candle_low=97.0, candle_close=101.0)

    evidence = interaction["price_action_evidence"]
    assert evidence["reclaim_type"] == "UNDERCUT_RECLAIM"
    assert evidence["close_relative_to_zone"] == "ABOVE_ZONE"
    assert evidence["penetration_ratio"] > 0


def test_extreme_volume_is_symbol_level_context_event_with_null_zone_ref():
    # 極端量能是整檔層級事件：即使沒有任何 zone 被觸碰也要輸出，且不綁定 zone。
    z = _zone(low=98.0, high=100.0, relative_volume=EXTREME_VOLUME_THRESHOLD + 0.3)

    events = detect_market_events([z], current_price=110.0, candle_high=None, candle_low=None, candle_close=None)

    assert [e["type"] for e in events] == ["EXTREME_VOLUME"]
    assert events[0]["direction"] == "NEUTRAL"
    assert events[0]["zone_ref"] is None


def test_extreme_volume_not_emitted_below_threshold():
    z = _zone(low=98.0, high=100.0, relative_volume=EXTREME_VOLUME_THRESHOLD - 0.5)

    events = detect_market_events([z], current_price=110.0, candle_high=None, candle_low=None, candle_close=None)

    assert "EXTREME_VOLUME" not in _types(events)


def test_high_volume_breakdown_on_support_closed_below():
    z = _zone(low=98.0, high=100.0, relative_volume=2.0)

    events = detect_market_events([z], current_price=99.0, candle_high=99.0, candle_low=96.0, candle_close=97.0)

    breakdown = [e for e in events if e["type"] == "HIGH_VOLUME_BREAKDOWN"]
    assert len(breakdown) == 1
    assert breakdown[0]["direction"] == "BEARISH"
    assert breakdown[0]["zone_ref"] is not None


def test_intraday_reclaim_excludes_reversal_candidate_for_same_zone():
    # 收回區間上緣同時滿足 reclaim 與 reversal 條件時，只掛更明確的 INTRADAY_RECLAIM。
    z = _zone(low=98.0, high=100.0, relative_volume=1.0)

    events = detect_market_events([z], current_price=99.0, candle_high=101.0, candle_low=97.0, candle_close=101.0)

    assert _types(events) == {"INTRADAY_RECLAIM"}


def test_high_volume_intraday_break_and_reclaim_keeps_full_event_chain():
    z = _zone(low=98.0, high=100.0, relative_volume=3.0, volume_confirmation=VolumeConfirmation.FAILED.value)

    events = detect_market_events([z], current_price=101.0, candle_high=102.0, candle_low=97.0, candle_close=101.0)

    assert [event["type"] for event in events] == [
        "EXTREME_VOLUME",
        "HIGH_VOLUME_BREAKDOWN",
        "INTRADAY_RECLAIM",
        "REVERSAL_CANDIDATE",
    ]


def test_reversal_candidate_when_support_held_inside():
    z = _zone(low=98.0, high=100.0, relative_volume=1.0)

    events = detect_market_events([z], current_price=99.0, candle_high=99.5, candle_low=97.0, candle_close=99.0)

    assert _types(events) == {"REVERSAL_CANDIDATE"}


def test_expired_support_held_inside_does_not_emit_reversal_candidate():
    z = _zone(low=98.0, high=100.0, relative_volume=1.0, recent_validation=RecentValidation.EXPIRED.value)

    events = detect_market_events([z], current_price=99.0, candle_high=99.5, candle_low=97.0, candle_close=99.0)

    assert "REVERSAL_CANDIDATE" not in _types(events)


def test_resistance_zone_does_not_produce_support_events():
    z = _zone(role=ZoneType.RESISTANCE.value, low=98.0, high=100.0, relative_volume=2.0)

    events = detect_market_events([z], current_price=99.0, candle_high=101.0, candle_low=96.0, candle_close=97.0)

    assert all(e["type"] == "EXTREME_VOLUME" for e in events) or events == []
