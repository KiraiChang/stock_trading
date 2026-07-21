"""Period summary serialization for SR Zone analysis."""
from __future__ import annotations

from typing import Any, Optional

from .labels import TIER_LABEL_TEXT, display_label as _display_label, role_label as _role_label
from .types import ConfidenceLevel, RecentValidation, VolumeConfirmation, ZoneScore, ZoneTier, ZoneType
from .formatting import fmt_price
from .utils import distance_pct_to_zone_bounds


PERIOD_SUMMARY_CONFIG = [
    ("short", "短期", ZoneTier.TIER_3_SHORT_TERM.value),
    ("mid", "中期", ZoneTier.TIER_2_TRADING_ZONE.value),
    ("long", "長期", ZoneTier.TIER_1_MAIN_STRUCTURE.value),
]


def _moving_average_state(current_price: float, ma5: Optional[float]) -> str:
    if ma5 is None:
        return "5日均線資料不足，先以區間本身觀察。"
    diff = (current_price - ma5) / ma5 if ma5 else 0.0
    if diff > 0.01:
        return f"收盤站上5日均線（{fmt_price(ma5)}），短線動能偏穩。"
    if diff < -0.01:
        return f"收盤跌破5日均線（{fmt_price(ma5)}），短線動能偏弱。"
    return f"收盤接近5日均線（{fmt_price(ma5)}），方向仍在整理。"


def _volume_reason(z: ZoneScore) -> Optional[str]:
    if z.volume_confirmation == VolumeConfirmation.CONFIRMED.value:
        return "量能有確認，這個區間的參考性較高。"
    if z.volume_confirmation == VolumeConfirmation.WEAK.value:
        return "量能不足，先降低這個區間的信心。"
    if z.volume_confirmation == VolumeConfirmation.FAILED.value:
        return "高量但驗證失敗，這個區間可能已被破壞。"
    return None


def _validation_reason(z: ZoneScore) -> str:
    if z.recent_validation == RecentValidation.VALIDATED_RECENTLY.value:
        return "最近一次測試有守住，短線仍有參考價值。"
    if z.recent_validation == RecentValidation.EXPIRED.value:
        return "最近一次測試偏失效，需等待重新站回或跌回確認。"
    if z.recent_validation == RecentValidation.NOT_TESTED_RECENTLY.value:
        return "近期沒有重新測試，參考性會隨時間下降。"
    return "尚待後續K棒驗證，不宜單獨當成進出依據。"


def _zone_summary(z: ZoneScore, side: str, current_price: float, ma5: Optional[float]) -> dict[str, Any]:
    reasons = [
        _moving_average_state(current_price, ma5),
        _validation_reason(z),
    ]
    volume = _volume_reason(z)
    if volume:
        reasons.append(volume)
    if z.confidence_level in (ConfidenceLevel.HIGH.value, ConfidenceLevel.VERY_HIGH.value):
        reasons.append("信心分級偏高，可列為主要觀察區。")
    elif z.confidence_level == ConfidenceLevel.LOW.value:
        reasons.append("信心分級偏低，代表樣本少或近期驗證不足。")
    family_count = z.confluence_family_count or 1
    if family_count > 1:
        reasons.append(f"有{family_count}個證據族群指向相近區間，屬於多方法共振。")

    return {
        "price_low": z.price_low,
        "price_high": z.price_high,
        "label": f"{fmt_price(z.price_low)} ~ {fmt_price(z.price_high)}",
        "role": z.role,
        "role_label": _role_label(z.role),
        "side": side,
        "tier": z.tier,
        "tier_label": z.tier_label,
        "display_label": _display_label(z.tier, z.role),
        "confidence": z.confidence,
        "confidence_level": z.confidence_level,
        "trading_score": z.trading_score,
        "recent_validation": z.recent_validation,
        "volume_confirmation": z.volume_confirmation,
        "confluence_count": z.confluence_count,
        "confluence_family_count": z.confluence_family_count,
        "confluence_families": list(z.confluence_families),
        "chip": {
            "direction": z.chip_direction,
            "contribution": z.trading_score_breakdown.get("chip"),
            "bounce_delta_pp": z.chip_bounce_delta,
            "break_delta_pp": z.chip_break_delta,
        },
        "reasons": reasons[:5],
    }


def _pick_period_pair(zones: list[ZoneScore], current_price: float) -> tuple[Optional[ZoneScore], Optional[ZoneScore]]:
    supports = [z for z in zones if z.role == ZoneType.SUPPORT.value and z.price_high < current_price]
    resistances = [z for z in zones if z.role == ZoneType.RESISTANCE.value and z.price_low > current_price]
    supports.sort(key=lambda z: _period_summary_rank(z, current_price), reverse=True)
    resistances.sort(key=lambda z: _period_summary_rank(z, current_price), reverse=True)

    for support in supports or [None]:
        for resistance in resistances or [None]:
            if support is not None and resistance is not None and support.price_high >= resistance.price_low:
                continue
            return support, resistance
    return None, None


def _period_summary_rank(z: ZoneScore, current_price: float) -> tuple[float, float, float]:
    distance_pct = distance_pct_to_zone_bounds(z.price_low, z.price_high, current_price)
    distance_score = 1.0 - min(distance_pct / 0.08, 1.0)
    confluence_score = min(float(z.confluence_family_count or z.confluence_count or 1) / 3.0, 1.0)
    relevance = (
        (z.trading_score / 100.0) * 0.50
        + z.confidence * 0.20
        + distance_score * 0.20
        + confluence_score * 0.10
    )
    if z.role == ZoneType.SUPPORT.value:
        location_tiebreaker = z.price_high
    else:
        location_tiebreaker = -z.price_low
    return (float(relevance), float(z.trading_score), float(location_tiebreaker))


def _build_period_summaries(
    zone_scores: list[ZoneScore], current_price: float, ma5: Optional[float]
) -> list[dict[str, Any]]:
    summaries = []
    for key, label, tier in PERIOD_SUMMARY_CONFIG:
        tier_zones = [z for z in zone_scores if z.tier == tier]
        support, resistance = _pick_period_pair(tier_zones, current_price)
        summary = {
            "key": key,
            "label": label,
            "tier": tier,
            "support": _zone_summary(support, "support", current_price, ma5) if support else None,
            "resistance": _zone_summary(resistance, "resistance", current_price, ma5) if resistance else None,
        }
        if support is None:
            summary["support_note"] = "暫無明確支撐"
        if resistance is None:
            summary["resistance_note"] = "暫無明確壓力"
        summaries.append(summary)
    return summaries
