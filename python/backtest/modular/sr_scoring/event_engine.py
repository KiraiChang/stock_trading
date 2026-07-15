"""Market event detection for SR Zone decision context."""
from __future__ import annotations

from typing import Any, Optional

from .types import RecentValidation, VolumeConfirmation, ZoneScore, ZoneType


EXTREME_VOLUME_THRESHOLD = 2.5
HIGH_VOLUME_BREAKDOWN_THRESHOLD = 1.5


def _fmt_price(v: float) -> str:
    return f"{v:.2f}"


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
    return {
        "distance_pct": distance_pct,
        "distance_label": f"{distance_pct * 100:.1f}%",
        "touched": touched,
        "penetration_pct": penetration_pct,
        "closed_inside": closed_inside,
        "closed_above": closed_above,
        "closed_below": closed_below,
        "state_label": state_label,
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
        if (interaction["closed_below"] or (candle_low is not None and candle_low < z.price_low)) and high_volume:
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
            if interaction["closed_below"]:
                continue
        if interaction["closed_above"] and interaction["penetration_pct"] > 0:
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
    return events[:8]
