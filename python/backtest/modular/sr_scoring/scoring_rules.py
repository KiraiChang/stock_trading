"""Trading score and entry relevance rules for SR Zone analysis."""
from __future__ import annotations

from typing import Optional

from .types import RecentValidation, TradingRecommendation, VolumeConfirmation, ZoneType
from .utils import distance_pct_to_zone_bounds


TRADING_SCORE_WEIGHTS = {
    "expected_value": 34.0,
    "risk_reward": 17.0,
    "trend": 12.75,
    "volume": 12.75,
    "confidence": 8.5,
    "chip": 15.0,
}
TRADING_SCORE_WEIGHTS_NO_DIRECT_CHIP = {
    "expected_value": 40.0,
    "risk_reward": 20.0,
    "trend": 15.0,
    "volume": 15.0,
    "confidence": 10.0,
}

_VOLUME_CONFIRMATION_WEIGHT = {
    VolumeConfirmation.CONFIRMED.value: 1.0,
    VolumeConfirmation.NEUTRAL.value: 0.5,
    VolumeConfirmation.WEAK.value: 0.3,
    VolumeConfirmation.FAILED.value: 0.0,
}


def _normalize_signed(value: float, cap: float) -> float:
    """Normalize a signed signal into [0, 1], where 0.5 is neutral."""
    return float(max(0.0, min(1.0, 0.5 + value / (2 * cap))))


def _trading_score_breakdown(
    role: str,
    confidence: float,
    expected_value: Optional[float],
    risk_reward_ratio: Optional[float],
    overall_trend: float,
    volume_confirmation: Optional[str],
    chip_score: Optional[float] = None,
) -> dict[str, float]:
    is_support = role == ZoneType.SUPPORT.value
    is_resistance = role == ZoneType.RESISTANCE.value

    ev_norm = _normalize_signed(expected_value, cap=0.05) if expected_value is not None else 0.5
    rr_norm = float(max(0.0, min(1.0, risk_reward_ratio / 3.0))) if risk_reward_ratio is not None else 0.5

    if is_support:
        trend_norm = _normalize_signed(overall_trend, cap=0.1)
    elif is_resistance:
        trend_norm = _normalize_signed(-overall_trend, cap=0.1)
    else:
        trend_norm = _normalize_signed(overall_trend, cap=0.1)

    volume_norm = _VOLUME_CONFIRMATION_WEIGHT.get(volume_confirmation, 0.5) if volume_confirmation else 0.5

    if chip_score is None:
        chip_norm = 0.5
    elif is_resistance:
        chip_norm = _normalize_signed(-chip_score, cap=100.0)
    else:
        chip_norm = _normalize_signed(chip_score, cap=100.0)

    w = TRADING_SCORE_WEIGHTS
    return {
        "expected_value": float(ev_norm * w["expected_value"]),
        "risk_reward": float(rr_norm * w["risk_reward"]),
        "trend": float(trend_norm * w["trend"]),
        "volume": float(volume_norm * w["volume"]),
        "confidence": float(confidence * w["confidence"]),
        "chip": float(chip_norm * w["chip"]),
    }


def _trading_score(breakdown: dict[str, float]) -> float:
    return float(sum(breakdown.values()))


def _trading_score_breakdown_no_direct_chip(
    role: str,
    confidence: float,
    expected_value: Optional[float],
    risk_reward_ratio: Optional[float],
    overall_trend: float,
    volume_confirmation: Optional[str],
) -> dict[str, float]:
    current = _trading_score_breakdown(
        role=role,
        confidence=confidence,
        expected_value=expected_value,
        risk_reward_ratio=risk_reward_ratio,
        overall_trend=overall_trend,
        volume_confirmation=volume_confirmation,
        chip_score=None,
    )
    out: dict[str, float] = {}
    for key, weight in TRADING_SCORE_WEIGHTS_NO_DIRECT_CHIP.items():
        old_weight = TRADING_SCORE_WEIGHTS[key]
        out[key] = float((current[key] / old_weight) * weight) if old_weight else 0.0
    return out


def _trading_recommendation(trading_score: float, role: str) -> str:
    if role == ZoneType.AT_ZONE.value:
        return TradingRecommendation.WATCH.value if trading_score >= 50 else TradingRecommendation.NEUTRAL.value

    if role == ZoneType.SUPPORT.value:
        if trading_score >= 80:
            return TradingRecommendation.STRONG_BUY.value
        if trading_score >= 60:
            return TradingRecommendation.BUY.value
        if trading_score >= 40:
            return TradingRecommendation.WATCH.value
        if trading_score >= 20:
            return TradingRecommendation.NEUTRAL.value
        return TradingRecommendation.AVOID.value

    if trading_score >= 80:
        return TradingRecommendation.STRONG_SELL.value
    if trading_score >= 60:
        return TradingRecommendation.AVOID.value
    if trading_score >= 40:
        return TradingRecommendation.NEUTRAL.value
    if trading_score >= 20:
        return TradingRecommendation.WATCH.value
    return TradingRecommendation.NEUTRAL.value


def _entry_relevance_breakdown(
    *,
    role: str,
    current_price: float,
    price_low: float,
    price_high: float,
    confidence: float,
    expected_value: Optional[float],
    risk_reward_ratio: Optional[float],
    recent_validation: str,
    volume_confirmation: Optional[str],
) -> dict[str, float]:
    distance_pct = distance_pct_to_zone_bounds(price_low, price_high, current_price)
    distance = max(0.0, 1.0 - min(distance_pct / 0.08, 1.0)) * 30.0
    ev_rr = 0.0
    if expected_value is not None:
        ev_rr += max(0.0, min((expected_value + 0.02) / 0.07, 1.0)) * 15.0
    else:
        ev_rr += 7.5
    if risk_reward_ratio is not None:
        ev_rr += min(risk_reward_ratio / 2.5, 1.0) * 15.0
    else:
        ev_rr += 7.5
    validation_map = {
        RecentValidation.VALIDATED_RECENTLY.value: 20.0,
        RecentValidation.PENDING_VALIDATION.value: 12.0,
        RecentValidation.NOT_TESTED_RECENTLY.value: 10.0,
        RecentValidation.EXPIRED.value: 0.0,
    }
    validation = validation_map.get(recent_validation, 8.0)
    volume_map = {
        VolumeConfirmation.CONFIRMED.value: 10.0,
        VolumeConfirmation.NEUTRAL.value: 6.0,
        VolumeConfirmation.WEAK.value: 3.0,
        VolumeConfirmation.FAILED.value: 0.0,
    }
    volume = volume_map.get(volume_confirmation, 5.0)
    role_readiness = 0.0 if role == ZoneType.AT_ZONE.value else 10.0
    return {
        "distance": float(distance),
        "ev_rr": float(ev_rr),
        "validation": float(validation),
        "volume": float(volume),
        "role_readiness": float(role_readiness),
        "confidence": float(confidence * 10.0),
    }


def _entry_relevance_score(breakdown: dict[str, float]) -> float:
    return float(max(0.0, min(100.0, sum(breakdown.values()))))
