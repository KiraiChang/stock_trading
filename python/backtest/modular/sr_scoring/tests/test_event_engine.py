from __future__ import annotations

from ..event_engine import (
    EXTREME_VOLUME_THRESHOLD,
    LIFECYCLE_CANDIDATE,
    LIFECYCLE_CONFIRMED,
    LIFECYCLE_EXPIRED,
    LIFECYCLE_RESOLVED,
    build_event_state_summary,
    detect_market_events,
    normalize_market_events,
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


def test_0050_break_reclaim_reversal_resolves_active_bearish_event():
    # 0050 fixture：同一個支撐區先高量跌破、再收回並形成反轉候選時，
    # raw event chain 要完整保留，但 active bearish gate 必須被解除。
    z = _zone(low=98.0, high=100.0, relative_volume=3.0, volume_confirmation=VolumeConfirmation.FAILED.value)

    events = detect_market_events([z], current_price=101.0, candle_high=102.0, candle_low=97.0, candle_close=101.0)
    summary = build_event_state_summary(events)

    assert [event["type"] for event in events] == [
        "EXTREME_VOLUME",
        "HIGH_VOLUME_BREAKDOWN",
        "INTRADAY_RECLAIM",
        "REVERSAL_CANDIDATE",
    ]
    assert summary["version"] == "event-lifecycle-p2"
    assert summary["active_bearish_events"] == []
    assert summary["market_state"] == "RECLAIM_ATTEMPT"
    breakdown = next(state for state in summary["states"] if state["event_family"] == "SUPPORT_BREAKDOWN")
    assert breakdown["active"] is False
    assert breakdown["state"] == LIFECYCLE_RESOLVED
    assert breakdown["resolved_by"] == "INTRADAY_RECLAIM"
    reclaim = next(state for state in summary["states"] if state["event_family"] == "SUPPORT_RECLAIM")
    assert reclaim["state"] == LIFECYCLE_CONFIRMED
    assert reclaim["active"] is True
    reversal = next(state for state in summary["states"] if state["event_family"] == "SUPPORT_REVERSAL")
    assert reversal["state"] == LIFECYCLE_CANDIDATE
    assert reversal["active"] is False


def test_unresolved_breakdown_remains_active_bearish_event():
    z = _zone(low=98.0, high=100.0, relative_volume=2.0, volume_confirmation=VolumeConfirmation.FAILED.value)

    events = detect_market_events([z], current_price=97.0, candle_high=99.0, candle_low=96.0, candle_close=97.0)
    summary = build_event_state_summary(events)

    assert [event["type"] for event in events] == ["HIGH_VOLUME_BREAKDOWN"]
    assert summary["market_state"] == "BREAKDOWN_RISK"
    assert [event["type"] for event in summary["active_bearish_events"]] == ["HIGH_VOLUME_BREAKDOWN"]
    breakdown = summary["active_bearish_events"][0]
    assert breakdown["state"] == LIFECYCLE_CONFIRMED
    assert breakdown["confirmation_state"] == "CONFIRMED_CLOSE_BELOW"


def test_intrabar_breakdown_without_close_confirmation_is_candidate_not_active():
    z = _zone(low=98.0, high=100.0, relative_volume=2.0, volume_confirmation=VolumeConfirmation.FAILED.value)

    events = detect_market_events([z], current_price=101.0, candle_high=102.0, candle_low=97.0, candle_close=99.0)
    summary = build_event_state_summary(events)

    breakdown = next(event for event in summary["states"] if event["event_family"] == "SUPPORT_BREAKDOWN")
    assert breakdown["state"] == LIFECYCLE_CANDIDATE
    assert breakdown["active"] is False
    assert summary["active_bearish_events"] == []
    assert summary["market_state"] != "BREAKDOWN_RISK"


def test_reversal_candidate_when_support_held_inside():
    z = _zone(low=98.0, high=100.0, relative_volume=1.0)

    events = detect_market_events([z], current_price=99.0, candle_high=99.5, candle_low=97.0, candle_close=99.0)

    assert _types(events) == {"REVERSAL_CANDIDATE"}
    summary = build_event_state_summary(events)
    reversal = summary["candidates"][0]
    assert reversal["state"] == LIFECYCLE_CANDIDATE
    assert reversal["active"] is False
    assert summary["active_bullish_events"] == []


def test_expired_support_held_inside_does_not_emit_reversal_candidate():
    z = _zone(low=98.0, high=100.0, relative_volume=1.0, recent_validation=RecentValidation.EXPIRED.value)

    events = detect_market_events([z], current_price=99.0, candle_high=99.5, candle_low=97.0, candle_close=99.0)

    assert "REVERSAL_CANDIDATE" not in _types(events)


def test_resistance_zone_does_not_produce_support_events():
    z = _zone(role=ZoneType.RESISTANCE.value, low=98.0, high=100.0, relative_volume=2.0)

    events = detect_market_events([z], current_price=99.0, candle_high=101.0, candle_low=96.0, candle_close=97.0)

    assert all(e["type"] == "EXTREME_VOLUME" for e in events) or events == []


def test_summary_groups_confirmed_and_candidate_states_separately():
    events = normalize_market_events([
        {
            "type": "HIGH_VOLUME_BREAKDOWN",
            "direction": "BEARISH",
            "lifecycle_state": LIFECYCLE_CONFIRMED,
            "active": True,
            "zone_ref": {"role": "SUPPORT", "price_low": 98.0, "price_high": 100.0},
        },
        {
            "type": "REVERSAL_CANDIDATE",
            "direction": "BULLISH",
            "lifecycle_state": LIFECYCLE_CANDIDATE,
            "active": False,
            "zone_ref": {"role": "SUPPORT", "price_low": 90.0, "price_high": 92.0},
        },
    ])

    summary = build_event_state_summary(events)

    assert [state["type"] for state in summary["confirmed"]] == ["HIGH_VOLUME_BREAKDOWN"]
    assert [state["type"] for state in summary["candidates"]] == ["REVERSAL_CANDIDATE"]
    assert [state["type"] for state in summary["active_bearish_events"]] == ["HIGH_VOLUME_BREAKDOWN"]


def test_previous_active_breakdown_carries_forward_without_new_events():
    previous = [{
        "type": "HIGH_VOLUME_BREAKDOWN",
        "event_family": "SUPPORT_BREAKDOWN",
        "event_scope": "ZONE",
        "zone_key": "SUPPORT:98.0000:100.0000",
        "root_event_type": "HIGH_VOLUME_BREAKDOWN",
        "latest_event_type": "HIGH_VOLUME_BREAKDOWN",
        "direction": "BEARISH",
        "state": LIFECYCLE_CONFIRMED,
        "active": True,
        "reason_codes": ["SUPPORT_CLOSED_BELOW"],
    }]

    summary = build_event_state_summary([], previous_states=previous)

    assert [state["type"] for state in summary["active_bearish_events"]] == ["HIGH_VOLUME_BREAKDOWN"]
    assert summary["market_state"] == "BREAKDOWN_RISK"
    assert summary["active_bearish_events"][0]["carried_from_previous"] is True


def test_current_reclaim_resolves_previous_active_breakdown():
    previous = [{
        "type": "HIGH_VOLUME_BREAKDOWN",
        "event_family": "SUPPORT_BREAKDOWN",
        "event_scope": "ZONE",
        "zone_key": "SUPPORT:98.0000:100.0000",
        "root_event_type": "HIGH_VOLUME_BREAKDOWN",
        "latest_event_type": "HIGH_VOLUME_BREAKDOWN",
        "direction": "BEARISH",
        "state": LIFECYCLE_CONFIRMED,
        "active": True,
        "reason_codes": ["SUPPORT_CLOSED_BELOW"],
    }]
    events = normalize_market_events([{
        "type": "INTRADAY_RECLAIM",
        "direction": "BULLISH",
        "lifecycle_state": LIFECYCLE_CONFIRMED,
        "active": True,
        "zone_ref": {"role": "SUPPORT", "price_low": 98.0, "price_high": 100.0},
    }])

    summary = build_event_state_summary(events, previous_states=previous)

    assert summary["active_bearish_events"] == []
    assert summary["market_state"] == "RECLAIM_ATTEMPT"
    breakdown = next(state for state in summary["states"] if state["event_family"] == "SUPPORT_BREAKDOWN")
    assert breakdown["state"] == LIFECYCLE_RESOLVED
    assert breakdown["resolved_by"] == "INTRADAY_RECLAIM"


def _carried_breakdown(**overrides) -> dict:
    base = {
        "type": "HIGH_VOLUME_BREAKDOWN",
        "event_family": "SUPPORT_BREAKDOWN",
        "event_scope": "ZONE",
        "zone_key": "SUPPORT:98.0000:100.0000",
        "root_event_type": "HIGH_VOLUME_BREAKDOWN",
        "latest_event_type": "HIGH_VOLUME_BREAKDOWN",
        "direction": "BEARISH",
        "state": LIFECYCLE_CONFIRMED,
        "active": True,
        "reason_codes": ["SUPPORT_CLOSED_BELOW"],
        "expires_after_bars": 2,
    }
    base.update(overrides)
    return base


def test_carried_active_event_expires_after_reaching_bar_threshold():
    # age_bars=1 再 carry 一次 → 2，達 expires_after_bars=2 門檻 → EXPIRED。
    previous = [_carried_breakdown(age_bars=1)]

    summary = build_event_state_summary([], previous_states=previous)

    breakdown = next(s for s in summary["states"] if s["event_family"] == "SUPPORT_BREAKDOWN")
    assert breakdown["state"] == LIFECYCLE_EXPIRED
    assert breakdown["active"] is False
    assert breakdown["age_bars"] == 2
    assert "EVENT_EXPIRED_STALE" in breakdown["reason_codes"]
    assert [s["type"] for s in summary["expired"]] == ["HIGH_VOLUME_BREAKDOWN"]
    assert summary["active_bearish_events"] == []


def test_carried_active_event_survives_below_expiry_threshold():
    # age_bars=0 再 carry 一次 → 1，未達 expires_after_bars=2 → 續 active。
    previous = [_carried_breakdown(age_bars=0)]

    summary = build_event_state_summary([], previous_states=previous)

    breakdown = next(s for s in summary["states"] if s["event_family"] == "SUPPORT_BREAKDOWN")
    assert breakdown["state"] == LIFECYCLE_CONFIRMED
    assert breakdown["active"] is True
    assert breakdown["age_bars"] == 1
    assert summary["expired"] == []
    assert [s["type"] for s in summary["active_bearish_events"]] == ["HIGH_VOLUME_BREAKDOWN"]


def test_fresh_detection_resets_carried_event_age():
    # 已接近過期（age_bars=1、門檻2）的 carried 事件，被當根新偵測同區事件覆蓋 → 存活
    # 計數歸零、續 active，不會過期。
    previous = [_carried_breakdown(age_bars=1)]
    events = normalize_market_events([{
        "type": "HIGH_VOLUME_BREAKDOWN",
        "direction": "BEARISH",
        "lifecycle_state": LIFECYCLE_CONFIRMED,
        "active": True,
        "expires_after_bars": 2,
        "zone_ref": {"role": "SUPPORT", "price_low": 98.0, "price_high": 100.0},
    }])

    summary = build_event_state_summary(events, previous_states=previous)

    breakdown = next(s for s in summary["states"] if s["event_family"] == "SUPPORT_BREAKDOWN")
    assert breakdown["state"] == LIFECYCLE_CONFIRMED
    assert breakdown["active"] is True
    assert breakdown["age_bars"] == 0
    assert summary["expired"] == []


def test_carried_event_without_expires_uses_default_threshold():
    # 未帶 expires_after_bars 的 carried 事件套 DEFAULT_EVENT_EXPIRES_AFTER_BARS(3)：
    # age_bars=2 再 carry → 3，達預設門檻 → EXPIRED，確保無永生事件。
    previous = [_carried_breakdown(age_bars=2)]
    previous[0].pop("expires_after_bars")

    summary = build_event_state_summary([], previous_states=previous)

    breakdown = next(s for s in summary["states"] if s["event_family"] == "SUPPORT_BREAKDOWN")
    assert breakdown["state"] == LIFECYCLE_EXPIRED
    assert breakdown["active"] is False
    assert breakdown["age_bars"] == 3
