"""Market event detection for SR Zone decision context."""
from __future__ import annotations

from typing import Any, Optional

from .formatting import fmt_price as _fmt_price
from .types import RecentValidation, VolumeConfirmation, ZoneScore, ZoneType


EXTREME_VOLUME_THRESHOLD = 2.5
HIGH_VOLUME_BREAKDOWN_THRESHOLD = 1.5

EVENT_TYPE_META = {
    "EXTREME_VOLUME": {
        "family": "VOLUME_CONTEXT",
        "direction": "NEUTRAL",
        "terminal_state": "ACTIVE",
        "resolves": (),
    },
    "HIGH_VOLUME_BREAKDOWN": {
        "family": "SUPPORT_BREAKDOWN",
        "direction": "BEARISH",
        "terminal_state": "ACTIVE",
        "resolves": (),
    },
    "INTRADAY_RECLAIM": {
        "family": "SUPPORT_RECLAIM",
        "direction": "BULLISH",
        "terminal_state": "ACTIVE",
        "resolves": ("SUPPORT_BREAKDOWN",),
    },
    "REVERSAL_CANDIDATE": {
        "family": "SUPPORT_REVERSAL",
        "direction": "BULLISH",
        "terminal_state": "ACTIVE",
        "resolves": ("SUPPORT_BREAKDOWN",),
    },
}

EVENT_ORDER = {
    "EXTREME_VOLUME": 10,
    "HIGH_VOLUME_BREAKDOWN": 20,
    "INTRADAY_RECLAIM": 30,
    "REVERSAL_CANDIDATE": 40,
}


def _distance_pct_to_zone(z: ZoneScore, current_price: float) -> float:
    if z.price_low <= current_price <= z.price_high:
        return 0.0
    if current_price < z.price_low:
        return (z.price_low - current_price) / current_price
    return (current_price - z.price_high) / current_price


def _clamp_relevance(value: float) -> float:
    return float(max(0.0, min(100.0, value)))


def entry_relevance_base_breakdown(z: ZoneScore, current_price: float) -> dict[str, float]:
    """base entry relevance 的逐項拆解：優先沿用 scoring 算好的 breakdown，只有沒有
    breakdown 的合成 zone（例如測試）才用這裡的簡化 fallback。event_engine 與
    decision_engine 共用同一份定義，避免同一 zone 在不同模組算出不同 base relevance。"""
    base = dict(z.entry_relevance_breakdown or {})
    if not base:
        distance_pct = _distance_pct_to_zone(z, current_price)
        base = {
            "distance": max(0.0, 1.0 - min(distance_pct / 0.08, 1.0)) * 30.0,
            "ev_rr": (
                (max(0.0, min(((z.expected_value or 0.0) + 0.02) / 0.07, 1.0)) * 15.0)
                + (min((z.risk_reward_ratio or 0.0) / 2.5, 1.0) * 15.0)
            ),
            "validation": 0.0 if z.recent_validation == RecentValidation.EXPIRED.value else 12.0,
            "volume": 5.0,
            "role_readiness": 0.0 if z.role == ZoneType.AT_ZONE.value else 10.0,
            "confidence": z.confidence * 10.0,
        }
    return base


def entry_relevance_base_score(z: ZoneScore, current_price: float) -> float:
    return _clamp_relevance(sum(entry_relevance_base_breakdown(z, current_price).values()))


def zone_interaction(
    z: ZoneScore,
    current_price: float,
    candle_high: Optional[float] = None,
    candle_low: Optional[float] = None,
    candle_close: Optional[float] = None,
) -> dict[str, Any]:
    high = current_price if candle_high is None else candle_high
    low = current_price if candle_low is None else candle_low
    close = current_price if candle_close is None else candle_close
    touched = low <= z.price_high and high >= z.price_low
    closed_inside = z.price_low <= close <= z.price_high
    closed_above = close > z.price_high
    closed_below = close < z.price_low
    penetration_pct = 0.0
    if low < z.price_low:
        penetration_pct = max(penetration_pct, (z.price_low - low) / z.price_low)
    if high > z.price_high:
        penetration_pct = max(penetration_pct, (high - z.price_high) / z.price_high)

    if not touched:
        state_label = "尚未測試"
    elif closed_inside:
        state_label = "進入區間"
    elif z.role == ZoneType.SUPPORT.value and closed_below:
        state_label = "有效跌破"
    elif z.role == ZoneType.SUPPORT.value and closed_above:
        state_label = "收回區間上方"
    else:
        state_label = "今日已測試"

    distance_pct = _distance_pct_to_zone(z, close)
    if closed_above:
        close_relative_to_zone = "ABOVE_ZONE"
    elif closed_below:
        close_relative_to_zone = "BELOW_ZONE"
    else:
        close_relative_to_zone = "INSIDE_ZONE"
    reclaim_type = "NONE"
    rejection_type = "NONE"
    if z.role == ZoneType.SUPPORT.value and touched and closed_above and penetration_pct > 0:
        reclaim_type = "UNDERCUT_RECLAIM"
    elif z.role == ZoneType.RESISTANCE.value and touched and closed_below and penetration_pct > 0:
        reclaim_type = "OVERTHROW_REJECTED"
    elif z.role == ZoneType.SUPPORT.value and touched and not closed_below:
        rejection_type = "SUPPORT_HELD"
    elif z.role == ZoneType.RESISTANCE.value and touched and not closed_above:
        rejection_type = "RESISTANCE_HELD"
    evidence = {
        "reclaim_type": reclaim_type,
        "rejection_type": rejection_type,
        "penetration_ratio": penetration_pct,
        "close_relative_to_zone": close_relative_to_zone,
        "follow_through": "UNKNOWN",
        "touched": touched,
        "closed_above": closed_above,
        "closed_below": closed_below,
    }
    return {
        "distance_pct": distance_pct,
        "distance_label": f"{distance_pct * 100:.1f}%",
        "touched": touched,
        "penetration_pct": penetration_pct,
        "closed_inside": closed_inside,
        "closed_above": closed_above,
        "closed_below": closed_below,
        "state_label": state_label,
        "price_action_evidence": evidence,
    }


def event_zone_ref(z: ZoneScore, current_price: float) -> dict[str, Any]:
    return {
        "price_low": z.price_low,
        "price_high": z.price_high,
        "label": f"{_fmt_price(z.price_low)} ~ {_fmt_price(z.price_high)}",
        "role": z.role,
        "tier": z.tier,
        "tier_label": z.tier_label,
        "distance_pct": _distance_pct_to_zone(z, current_price),
        "entry_relevance_score": entry_relevance_base_score(z, current_price),
    }


def _zone_key(zone_ref: Optional[dict[str, Any]]) -> str:
    if not zone_ref:
        return "SYMBOL"
    role = zone_ref.get("role") or "UNKNOWN"
    return f"{role}:{float(zone_ref.get('price_low', 0.0)):.4f}:{float(zone_ref.get('price_high', 0.0)):.4f}"


def normalize_market_event(event: dict[str, Any]) -> dict[str, Any]:
    event_type = str(event.get("type") or "UNKNOWN")
    meta = EVENT_TYPE_META.get(event_type, {})
    zone_ref = event.get("zone_ref")
    event_scope = "SYMBOL" if zone_ref is None else "ZONE"
    event_family = str(meta.get("family") or event_type)
    normalized = dict(event)
    normalized.setdefault("event_family", event_family)
    normalized.setdefault("event_scope", event_scope)
    normalized.setdefault("event_key", f"{event_scope}:{event_family}:{_zone_key(zone_ref)}")
    normalized.setdefault("zone_key", _zone_key(zone_ref))
    normalized.setdefault("state", str(meta.get("terminal_state") or "ACTIVE"))
    normalized.setdefault("active", normalized["state"] == "ACTIVE")
    normalized.setdefault("reason_codes", [event_type])
    return normalized


def normalize_market_events(events: list[dict[str, Any]]) -> list[dict[str, Any]]:
    return [normalize_market_event(event) for event in events]


def build_event_state_summary(events: list[dict[str, Any]]) -> dict[str, Any]:
    """Build an in-memory event lifecycle summary from latest detected events.

    This is intentionally persistence-free for P1: it gives Decision a stable
    active/resolved view without introducing event tables before fixtures lock
    the transition semantics.
    """
    normalized = sorted(normalize_market_events(events), key=lambda e: EVENT_ORDER.get(str(e.get("type")), 999))
    states: dict[tuple[str, str], dict[str, Any]] = {}
    resolved: list[dict[str, Any]] = []

    for event in normalized:
        zone_key = str(event.get("zone_key") or "SYMBOL")
        event_family = str(event.get("event_family") or event.get("type"))
        event_type = str(event.get("type") or "UNKNOWN")
        key = (zone_key, event_family)
        state = {
            "event_key": event.get("event_key"),
            "type": event_type,
            "zone_key": zone_key,
            "event_family": event_family,
            "event_scope": event.get("event_scope"),
            "root_event_type": event_type,
            "latest_event_type": event_type,
            "direction": event.get("direction"),
            "state": "ACTIVE",
            "active": True,
            "zone_ref": event.get("zone_ref"),
            "price_level": event.get("price_level"),
            "confidence": event.get("confidence"),
            "reason_codes": list(event.get("reason_codes") or [event_type]),
            "resolved_by": None,
        }
        states[key] = state

        for family in EVENT_TYPE_META.get(event_type, {}).get("resolves", ()):
            target_key = (zone_key, str(family))
            target = states.get(target_key)
            if not target or not target.get("active"):
                continue
            target["state"] = "RESOLVED"
            target["active"] = False
            target["latest_event_type"] = event_type
            target["resolved_by"] = event_type
            target["reason_codes"] = [*target.get("reason_codes", []), f"RESOLVED_BY_{event_type}"]
            resolved.append(target)

    active = [state for state in states.values() if state.get("active")]
    active_bearish = [state for state in active if state.get("direction") == "BEARISH"]
    active_bullish = [state for state in active if state.get("direction") == "BULLISH"]
    latest_type = normalized[-1]["type"] if normalized else None
    return {
        "version": "event-lifecycle-p1",
        "states": list(states.values()),
        "active": active,
        "resolved": resolved,
        "active_bearish_events": active_bearish,
        "active_bullish_events": active_bullish,
        "latest_event_type": latest_type,
        "market_state": market_state_from_event_states(active),
    }


def market_state_from_event_states(active_states: list[dict[str, Any]]) -> str:
    active_types = {state.get("latest_event_type") or state.get("root_event_type") for state in active_states}
    if "HIGH_VOLUME_BREAKDOWN" in active_types:
        return "BREAKDOWN_RISK"
    if "INTRADAY_RECLAIM" in active_types:
        return "RECLAIM_ATTEMPT"
    if "REVERSAL_CANDIDATE" in active_types:
        return "REVERSAL_CANDIDATE"
    return "NORMAL"


def detect_market_events(
    zone_scores: list[ZoneScore],
    current_price: float,
    candle_high: Optional[float],
    candle_low: Optional[float],
    candle_close: Optional[float],
) -> list[dict[str, Any]]:
    events: list[dict[str, Any]] = []
    max_relative_volume = max((z.relative_volume or 0.0 for z in zone_scores), default=0.0)
    if max_relative_volume >= EXTREME_VOLUME_THRESHOLD:
        events.append({
            "type": "EXTREME_VOLUME",
            "direction": "NEUTRAL",
            "confidence": min(1.0, max_relative_volume / 4.0),
            "zone_ref": None,
            "price_level": candle_close if candle_close is not None else current_price,
            "reason": "最新量能達極端放大門檻，需搭配跌破或收復事件解讀。",
            "detected_at": "latest_candle",
        })

    for z in zone_scores:
        if z.role != ZoneType.SUPPORT.value:
            continue
        interaction = zone_interaction(z, current_price, candle_high, candle_low, candle_close)
        if not interaction["touched"]:
            continue
        relative_volume = z.relative_volume or 0.0
        high_volume = relative_volume >= HIGH_VOLUME_BREAKDOWN_THRESHOLD or z.volume_confirmation == VolumeConfirmation.FAILED.value
        breakdown_event_added = False
        evidence = interaction["price_action_evidence"]
        if (evidence["closed_below"] or (candle_low is not None and candle_low < z.price_low)) and high_volume:
            events.append({
                "type": "HIGH_VOLUME_BREAKDOWN",
                "direction": "BEARISH",
                "confidence": min(1.0, max(0.45, relative_volume / 3.0)),
                "zone_ref": event_zone_ref(z, current_price),
                "price_level": z.price_low,
                "reason": "支撐區被盤中或收盤跌破，且量能放大或量能狀態確認失敗。",
                "detected_at": "latest_candle",
            })
            breakdown_event_added = True
            if evidence["closed_below"]:
                continue
        if evidence["reclaim_type"] == "UNDERCUT_RECLAIM":
            events.append({
                "type": "INTRADAY_RECLAIM",
                "direction": "BULLISH",
                "confidence": min(1.0, 0.50 + z.confidence * 0.35),
                "zone_ref": event_zone_ref(z, current_price),
                "price_level": z.price_high,
                "reason": "日 K 測試支撐後收盤收回區間上緣。",
                "detected_at": "latest_candle",
            })
            if breakdown_event_added:
                events.append({
                    "type": "REVERSAL_CANDIDATE",
                    "direction": "BULLISH",
                    "confidence": min(1.0, 0.50 + z.confidence * 0.35),
                    "zone_ref": event_zone_ref(z, current_price),
                    "price_level": z.price_high,
                    "reason": "高量跌破後收回支撐區上緣，形成反轉候選事件。",
                    "detected_at": "latest_candle",
                })
        elif (
            not interaction["closed_below"]
            and z.confidence >= 0.45
            and (z.expected_value or 0.0) >= 0
            and z.recent_validation != RecentValidation.EXPIRED.value
        ):
            events.append({
                "type": "REVERSAL_CANDIDATE",
                "direction": "BULLISH",
                "confidence": min(1.0, 0.45 + z.confidence * 0.30),
                "zone_ref": event_zone_ref(z, current_price),
                "price_level": z.price_high,
                "reason": "支撐測試未失守，且 EV 與區間信心未轉弱。",
                "detected_at": "latest_candle",
            })
    return normalize_market_events(events[:8])
