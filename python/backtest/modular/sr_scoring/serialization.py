"""Serialization helpers for SR Zone API response contracts."""
from __future__ import annotations

from typing import Any

from .event_engine import zone_identity_key as _zone_identity_key
from .labels import display_label as _display_label, role_label as _role_label
from .types import ZoneScore


def _zone_score_to_dict(z: ZoneScore) -> dict[str, Any]:
    return {
        "price_low": z.price_low,
        "price_high": z.price_high,
        "method": z.method,
        "role": z.role,
        # zone_key 讓 Go 端能把事件掛回這個 zone 的穩定身分（T-048 階段 C）。
        # 與 market_event_*.zone_key 同一個函數產生，不是重算的——理由見
        # event_engine.zone_identity_key。
        "zone_key": _zone_identity_key(z),
        "tier": z.tier,
        "tier_label": z.tier_label,
        "role_label": _role_label(z.role),
        "display_label": _display_label(z.tier, z.role),
        "support_score": z.support_score,
        "resistance_score": z.resistance_score,
        "net_score": z.net_score,
        "net_score_label": z.net_score_label,
        "confidence": z.confidence,
        "confidence_level": z.confidence_level,
        "bounce_probability": z.bounce_probability,
        "break_probability": z.break_probability,
        "expected_gain": z.expected_gain,
        "expected_loss": z.expected_loss,
        "expected_value": z.expected_value,
        "risk_reward_ratio": z.risk_reward_ratio,
        "reward_risk_percentile": z.reward_risk_percentile,
        "relative_volume": z.relative_volume,
        "volume_confirmation": z.volume_confirmation,
        "touch_count": z.touch_count,
        "support_touch_count": z.support_touch_count,
        "resistance_touch_count": z.resistance_touch_count,
        "reject_count": z.reject_count,
        "break_count": z.break_count,
        "zone_momentum": z.zone_momentum,
        "zone_direction": z.zone_direction,
        "recent_validation": z.recent_validation,
        "trading_score": z.trading_score,
        "trading_score_breakdown": z.trading_score_breakdown,
        "trading_recommendation": z.trading_recommendation,
        "zone_quality_score": z.zone_quality_score if z.zone_quality_score is not None else z.trading_score,
        "entry_relevance_score": z.entry_relevance_score,
        "entry_relevance_breakdown": z.entry_relevance_breakdown,
        "validation_debug": z.validation_debug,
        "overlap_group": z.overlap_group,
        "confluence_count": z.confluence_count,
        "confluence_family_count": z.confluence_family_count,
        "confluence_families": list(z.confluence_families),
    }
