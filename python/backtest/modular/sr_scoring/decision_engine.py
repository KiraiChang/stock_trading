"""SR Zone Decision Engine v2.

This module turns SR Zone scoring output into market/position decision context.
It is intentionally portfolio-agnostic: no holding cost, shares, PnL, position
sizing, or order execution logic belongs here.
"""
from __future__ import annotations

from datetime import date, datetime
from typing import Any, Optional

from .event_engine import (
    detect_market_events,
    entry_relevance_base_breakdown as _entry_relevance_base_breakdown,
    zone_interaction as _zone_interaction,
    _clamp_relevance,
    _distance_pct_to_zone,
    _fmt_price,
)
from .model import ModelBundle
from .types import ConfidenceLevel, RecentValidation, ZoneScore, ZoneTier, ZoneType
from .pipeline_types import AnalysisEvidence


DEFAULT_STALE_AFTER_DAYS = 1


# entry_relevance_score 有兩個層次，兩者都 clamp 到 [0,100]：
#   1. base：純粹的「單一 zone 進場相關性」，與 scoring.py 的 entry_relevance_score
#      同定義（優先沿用 scoring 算好的 breakdown，合成 zone 才用簡化 fallback）。base
#      breakdown 的定義集中在 event_engine.entry_relevance_base_breakdown，讓 event_engine
#      與 decision_engine 共用同一份、不會同一 zone 在兩個模組算出不同值。
#      這是「回報值」——decision_summary 各 zone 對外輸出的 entry_relevance_score
#      一律用 base，確保跟 zones[] 的同名欄位一致、不會同名不同義。
#   2. with-events：base 再疊加 market_event 修正（reclaim/reversal +8、高量跌破 -25），
#      僅供 decision 內部 gating（_decision_action / _pick_primary_zone）使用，不對外輸出。
#      市場事件對外已有 market_events / short_term_regime 專門呈現，不重複灌進回報分數。
def _market_event_adjustment(z: ZoneScore, market_events: Optional[list[dict[str, Any]]]) -> float:
    event_score = 0.0
    for event in market_events or []:
        ref = event.get("zone_ref") or {}
        same_zone = ref.get("price_low") == z.price_low and ref.get("price_high") == z.price_high
        if not same_zone:
            continue
        if event.get("type") in ("INTRADAY_RECLAIM", "REVERSAL_CANDIDATE"):
            event_score = max(event_score, 8.0)
        elif event.get("type") == "HIGH_VOLUME_BREAKDOWN":
            event_score = min(event_score, -25.0)
    return float(event_score)


def _entry_relevance_score(z: ZoneScore, current_price: float) -> float:
    """對外回報用的 base entry relevance（不含市場事件修正）。"""
    return _clamp_relevance(sum(_entry_relevance_base_breakdown(z, current_price).values()))


def _entry_relevance_score_with_events(
    z: ZoneScore, current_price: float, market_events: Optional[list[dict[str, Any]]] = None
) -> float:
    """decision 內部 gating 用的 event-aware relevance（base + 市場事件修正）。"""
    return _clamp_relevance(
        sum(_entry_relevance_base_breakdown(z, current_price).values())
        + _market_event_adjustment(z, market_events)
    )


def _zone_quality_score(z: ZoneScore) -> float:
    return float(z.zone_quality_score if z.zone_quality_score is not None else z.trading_score)


def _event_matches_zone(event: dict[str, Any], zone: ZoneScore) -> bool:
    ref = event.get("zone_ref") or {}
    return ref.get("price_low") == zone.price_low and ref.get("price_high") == zone.price_high


def _decision_summary_zone(
    z: ZoneScore,
    current_price: float,
    reason: str,
    candle_high: Optional[float] = None,
    candle_low: Optional[float] = None,
    candle_close: Optional[float] = None,
    source: str = "HISTORICAL_SR",
    decision_role: str = "REFERENCE",
) -> dict[str, Any]:
    interaction = _zone_interaction(z, current_price, candle_high, candle_low, candle_close)
    structural_score = _zone_quality_score(z)
    decision_relevance_score = _entry_relevance_score(z, current_price)
    tradability_score = float(z.trading_score)
    return {
        "price_low": z.price_low,
        "price_high": z.price_high,
        "label": f"{_fmt_price(z.price_low)} ~ {_fmt_price(z.price_high)}",
        "role": z.role,
        "tier": z.tier,
        "tier_label": z.tier_label,
        "trading_score": z.trading_score,
        "zone_quality_score": structural_score,
        "structural_score": structural_score,
        # 對外一律回報 base entry relevance（不含市場事件），與 zones[] 的
        # entry_relevance_score 同定義；事件影響另由 market_events / short_term_regime 呈現。
        "entry_relevance_score": decision_relevance_score,
        "decision_relevance_score": decision_relevance_score,
        "tradability_score": tradability_score,
        "entry_relevance_breakdown": _entry_relevance_base_breakdown(z, current_price),
        "confidence": z.confidence,
        "confidence_level": z.confidence_level,
        "expected_value": z.expected_value,
        "risk_reward_ratio": z.risk_reward_ratio,
        "distance_pct": interaction["distance_pct"],
        "distance_label": interaction["distance_label"],
        "zone_interaction": interaction,
        "recent_validation": z.recent_validation,
        "volume_confirmation": z.volume_confirmation,
        "confluence_count": z.confluence_count,
        "confluence_family_count": z.confluence_family_count,
        "confluence_families": list(z.confluence_families),
        "source": source,
        "lifecycle": _zone_lifecycle(z, interaction),
        "decision_role": decision_role,
        "reason": reason,
    }


def _zone_lifecycle(z: ZoneScore, interaction: Optional[dict[str, Any]] = None) -> str:
    if z.recent_validation == RecentValidation.EXPIRED.value:
        return "INVALIDATED"
    if interaction and interaction.get("closed_below") and z.role == ZoneType.SUPPORT.value:
        return "BROKEN"
    if z.confidence_level == ConfidenceLevel.LOW.value:
        return "WEAKENING"
    if z.recent_validation == RecentValidation.PENDING_VALIDATION.value:
        return "CANDIDATE"
    if interaction and interaction.get("closed_above") and z.role == ZoneType.SUPPORT.value:
        return "CONFIRMED"
    if interaction and interaction.get("touched") and not interaction.get("closed_below"):
        return "VALIDATED"
    return "VALIDATED"


def _position_action_condition(primary_zone: Optional[ZoneScore], structure_state: str) -> dict[str, Any]:
    if primary_zone is None:
        return {
            "state": structure_state,
            "invalidation_price": None,
            "recovery_price": None,
            "reason_codes": ["NO_PRIMARY_ZONE"],
        }

    reason_codes: list[str] = []
    if primary_zone.role == ZoneType.SUPPORT.value:
        reason_codes.append("PRIMARY_SUPPORT")
        if structure_state == "SUPPORT_RECLAIM_CANDIDATE":
            reason_codes.append("SUPPORT_RECLAIM_AWAIT_CONFIRMATION")
        elif structure_state == "SUPPORT_RECLAIM_CONFIRMED":
            reason_codes.append("SUPPORT_RECLAIM_CONFIRMED")
        elif structure_state in ("SUPPORT_RECLAIM_INVALIDATED", "BREAKDOWN"):
            reason_codes.append("SUPPORT_BREAKDOWN_RISK")
        else:
            reason_codes.append("SUPPORT_DEFENSE")
        return {
            "state": structure_state,
            "invalidation_price": primary_zone.price_low,
            "recovery_price": primary_zone.price_high,
            "reason_codes": reason_codes,
        }

    if primary_zone.role == ZoneType.RESISTANCE.value:
        return {
            "state": structure_state,
            "invalidation_price": primary_zone.price_high,
            "recovery_price": primary_zone.price_low,
            "reason_codes": ["PRIMARY_RESISTANCE", "UPSIDE_BREAKOUT_REQUIRED"],
        }

    return {
        "state": structure_state,
        "invalidation_price": primary_zone.price_low,
        "recovery_price": primary_zone.price_high,
        "reason_codes": ["WAIT_FOR_DIRECTION"],
    }


def _confidence_label(value: Optional[float], level: Optional[str]) -> str:
    if value is None:
        return "無資料"
    level_text = {
        ConfidenceLevel.LOW.value: "低",
        ConfidenceLevel.MEDIUM.value: "中",
        ConfidenceLevel.HIGH.value: "高",
        ConfidenceLevel.VERY_HIGH.value: "極高",
    }.get(level or "", level or "未分級")
    return f"{value * 100:.0f}%（{level_text}）"


def _market_regime(
    global_trend: float,
    global_volatility: float,
    global_confidence: Optional[float],
    chip_summary: dict[str, Any],
    structure_state: str = "NORMAL",
    market_events: Optional[list[dict[str, Any]]] = None,
) -> dict[str, Any]:
    if global_trend >= 0.015:
        trend_regime = "TREND_UP"
        trend_label = "偏多趨勢"
    elif global_trend <= -0.015:
        trend_regime = "TREND_DOWN"
        trend_label = "偏空趨勢"
    else:
        trend_regime = "RANGE_BOUND"
        trend_label = "區間盤"

    flags: list[str] = []
    reasons = [f"整體趨勢 {global_trend * 100:.1f}%"]
    volatility_state = "HIGH_VOLATILITY" if global_volatility >= 0.035 else "NORMAL"
    if global_volatility >= 0.035:
        flags.append("HIGH_VOLATILITY")
        reasons.append(f"波動偏高（{global_volatility * 100:.1f}%）")
    if global_confidence is not None and global_confidence < 0.45:
        flags.append("LOW_CONFIDENCE")
        reasons.append(f"整體信心偏低（{global_confidence * 100:.0f}%）")
    if chip_summary.get("missing"):
        reasons.append("籌碼資料缺漏，籌碼面以中性或缺資料解讀")
    elif chip_summary.get("score") is not None:
        reasons.append(f"籌碼總分 {float(chip_summary['score']):.1f}")

    structure_label = {
        "NORMAL": "",
        "SUPPORT_RECLAIM_CANDIDATE": "支撐收復候選",
        "SUPPORT_RECLAIM_CONFIRMED": "支撐收復確認",
        "SUPPORT_RECLAIM_INVALIDATED": "支撐收復失效",
        "BREAKDOWN": "短線結構跌破",
    }.get(structure_state, structure_state)
    event_types = {event.get("type") for event in market_events or []}
    if "HIGH_VOLUME_BREAKDOWN" in event_types or structure_state in ("SUPPORT_RECLAIM_INVALIDATED", "BREAKDOWN"):
        short_term_regime = "BREAKDOWN_RISK"
    elif "INTRADAY_RECLAIM" in event_types:
        short_term_regime = "RECLAIM_ATTEMPT"
    elif "REVERSAL_CANDIDATE" in event_types:
        short_term_regime = "REVERSAL_CANDIDATE"
    else:
        short_term_regime = "NORMAL"
    tactical_regime = short_term_regime
    recovery_state = structure_state

    label = trend_label
    if structure_state in ("SUPPORT_RECLAIM_INVALIDATED", "BREAKDOWN"):
        label = f"長期{trend_label.replace('趨勢', '')}，但{structure_label}"
    elif structure_label:
        label += f"、{structure_label}"
    if "HIGH_VOLATILITY" in flags:
        label += "但波動偏高"
    if "LOW_CONFIDENCE" in flags:
        label += "且信心不足"

    return {
        "primary": trend_regime,
        "trend_regime": trend_regime,
        "structural_trend": trend_regime,
        "short_term_regime": short_term_regime,
        "tactical_regime": tactical_regime,
        "structure_state": structure_state,
        "recovery_state": recovery_state,
        "volatility_state": volatility_state,
        "flags": flags,
        "label": label,
        "reasons": reasons[:4],
    }


def _primary_zone_score(
    z: ZoneScore,
    current_price: float,
    regime_primary: str,
    market_events: Optional[list[dict[str, Any]]] = None,
) -> float:
    confluence_score = min((z.confluence_family_count or 1) / 3.0, 1.0)
    role_alignment = 0.0
    if regime_primary in ("TREND_UP", "RANGE_BOUND") and z.role == ZoneType.SUPPORT.value:
        role_alignment = 1.0
    elif regime_primary == "TREND_DOWN" and z.role == ZoneType.RESISTANCE.value:
        role_alignment = 1.0
    return (
        _entry_relevance_score_with_events(z, current_price, market_events) / 100.0 * 0.72
        + _zone_quality_score(z) / 100.0 * 0.15
        + confluence_score * 0.08
        + role_alignment * 0.05
    )


def _pick_primary_zone(
    zone_scores: list[ZoneScore],
    current_price: float,
    regime_primary: str,
    market_events: Optional[list[dict[str, Any]]] = None,
) -> Optional[ZoneScore]:
    candidates = [
        z for z in zone_scores
        if z.role != ZoneType.AT_ZONE.value
        and z.recent_validation != RecentValidation.EXPIRED.value
        and z.confidence_level != ConfidenceLevel.LOW.value
        and z.expected_value is not None
    ]
    if not candidates:
        candidates = [z for z in zone_scores if z.role != ZoneType.AT_ZONE.value]
    if not candidates:
        return None
    return max(candidates, key=lambda z: _primary_zone_score(z, current_price, regime_primary, market_events))


def _high_volume_breakdown_severity(
    event: dict[str, Any],
    primary_zone: ZoneScore,
) -> str:
    if _event_matches_zone(event, primary_zone):
        return "critical"

    ref = event.get("zone_ref") or {}
    tier = ref.get("tier")
    distance_pct = ref.get("distance_pct")
    relevance = ref.get("entry_relevance_score")
    if tier == ZoneTier.TIER_1_MAIN_STRUCTURE.value:
        return "critical"
    if tier != ZoneTier.TIER_3_SHORT_TERM.value and relevance is not None and distance_pct is not None:
        if float(relevance) >= 70.0 and float(distance_pct) <= 0.05:
            return "critical"
        if float(relevance) >= 55.0 or float(distance_pct) <= 0.08:
            return "moderate"
    if tier == ZoneTier.TIER_3_SHORT_TERM.value and relevance is not None:
        if float(relevance) >= 55.0:
            return "moderate"
    if tier == ZoneTier.TIER_2_TRADING_ZONE.value:
        return "moderate"
    return "minor"


def _high_volume_breakdown_action(
    market_events: Optional[list[dict[str, Any]]],
    primary_zone: ZoneScore,
) -> tuple[str, str, str, str, list[str]] | None:
    severities = [
        _high_volume_breakdown_severity(event, primary_zone)
        for event in market_events or []
        if event.get("type") == "HIGH_VOLUME_BREAKDOWN"
    ]
    if not severities:
        return None
    if "critical" in severities:
        return (
            "AVOID",
            "EXIT",
            "Avoid",
            "避開",
            ["出現主結構或主交易區高量跌破事件，先以防守優先。"],
        )
    if "moderate" in severities:
        return (
            "WATCH",
            "REDUCE_ON_BREAKDOWN",
            "Hold",
            "等待",
            ["出現相關支撐高量跌破事件，先降低風險暴露。"],
        )
    return None


def _decision_action(
    regime: dict[str, Any],
    primary_zone: Optional[ZoneScore],
    current_price: float,
    interaction: Optional[dict[str, Any]],
    market_events: Optional[list[dict[str, Any]]] = None,
) -> tuple[str, str, str, str, list[str]]:
    risk_notes: list[str] = []
    flags = set(regime.get("flags") or [])
    if "HIGH_VOLATILITY" in flags:
        risk_notes.append("波動偏高，倉位需保守。")
    if "LOW_CONFIDENCE" in flags:
        risk_notes.append("整體信心不足，應等待更多確認。")
    if primary_zone is None:
        risk_notes.append("沒有足夠明確的主交易區。")
        return "WATCH", "HOLD", "Hold", "等待", risk_notes

    breakdown_action = _high_volume_breakdown_action(market_events, primary_zone)
    if breakdown_action is not None:
        event_risk_notes = breakdown_action[4]
        return breakdown_action[0], breakdown_action[1], breakdown_action[2], breakdown_action[3], risk_notes + event_risk_notes
    if any(event.get("type") == "HIGH_VOLUME_BREAKDOWN" for event in market_events or []):
        risk_notes.append("短線支撐出現高量跌破，但尚未達主結構防守門檻。")

    distance_pct = _distance_pct_to_zone(primary_zone, current_price)
    if distance_pct > 0.08:
        risk_notes.append("現價離主交易區較遠，不適合追價。")
    rr = primary_zone.risk_reward_ratio
    if rr is None:
        risk_notes.append("主交易區缺少風險報酬比，先觀察。")
    elif rr < 1.5:
        risk_notes.append("主交易區風險報酬比不足。")
    elif rr < 2.0:
        risk_notes.append("風險報酬比未達完整買進門檻，最多小量試單。")
    if primary_zone.recent_validation == RecentValidation.EXPIRED.value:
        risk_notes.append("主交易區近期驗證偏失效。")

    primary = regime.get("primary")
    structure_state = regime.get("structure_state")
    bullish_setup = primary in ("TREND_UP", "RANGE_BOUND") and primary_zone.role == ZoneType.SUPPORT.value
    bearish_setup = primary == "TREND_DOWN" or primary_zone.role == ZoneType.RESISTANCE.value
    structure_broken = structure_state in ("SUPPORT_RECLAIM_INVALIDATED", "BREAKDOWN")
    if structure_broken:
        risk_notes.append("短線結構轉弱，市場訊號不得覆蓋跌破風險。")
        position_action = "EXIT" if structure_state == "BREAKDOWN" else "REDUCE_ON_BREAKDOWN"
        return "AVOID", position_action, "Avoid", "避開", risk_notes
    if bearish_setup and not bullish_setup:
        return "AVOID", "REDUCE", "Avoid", "避開", risk_notes
    if rr is None or rr < 1.5:
        return "WATCH", "HOLD", "Hold", "等待", risk_notes

    relevance = _entry_relevance_score_with_events(primary_zone, current_price, market_events)
    strong = (
        relevance >= 75
        and primary_zone.confidence >= 0.65
        and (primary_zone.expected_value or 0) > 0
        and rr >= 2.0
        and distance_pct <= 0.05
        and not flags
    )
    constructive = (
        bullish_setup
        and relevance >= 55
        and primary_zone.confidence >= 0.45
        and (primary_zone.expected_value or 0) >= 0
    )

    if strong:
        return "BUY", "HOLD", "Buy", "買進", risk_notes
    if constructive:
        return "BUY_SMALL", "HOLD", "BuySmall", "小量試單", risk_notes
    if risk_notes:
        return "WATCH", "HOLD", "Hold", "等待", risk_notes
    return "WATCH", "HOLD", "Hold", "等待", ["尚未形成足夠明確的進場優勢。"]


def _entry_action_state(
    action: str,
    primary_zone: Optional[ZoneScore],
    structure_state: str,
    current_price: float,
    market_events: Optional[list[dict[str, Any]]] = None,
) -> str:
    if primary_zone is None or action in ("Avoid", "Hold"):
        return "WAIT_CONFIRMATION"
    if (
        primary_zone.recent_validation == RecentValidation.PENDING_VALIDATION.value
        or structure_state == "SUPPORT_RECLAIM_CANDIDATE"
    ):
        return "PROBE_ENTRY" if action == "BuySmall" else "WAIT_CONFIRMATION"
    if action == "BuySmall":
        return "SMALL_ENTRY"
    if action == "Buy":
        relevance = _entry_relevance_score_with_events(primary_zone, current_price, market_events)
        rr = primary_zone.risk_reward_ratio or 0.0
        if relevance >= 85.0 and rr >= 2.5:
            return "BUY"
        return "ACCUMULATE"
    return "WAIT_CONFIRMATION"


def _entry_action_label(state: str) -> str:
    return {
        "WAIT_CONFIRMATION": "等待確認",
        "PROBE_ENTRY": "觀察性試探",
        "SMALL_ENTRY": "小量進場",
        "ACCUMULATE": "分批累積",
        "BUY": "買進",
    }.get(state, state)


def _market_bias(
    regime: dict[str, Any],
    primary_zone: Optional[ZoneScore],
    market_action: str,
    market_events: Optional[list[dict[str, Any]]] = None,
) -> tuple[str, str]:
    event_types = {event.get("type") for event in market_events or []}
    if "REVERSAL_CANDIDATE" in event_types or "INTRADAY_RECLAIM" in event_types:
        return "REVERSAL_BIAS", "反轉觀察"
    if market_action == "AVOID":
        return "BEARISH_BIAS", "偏空觀察"
    if market_action in ("BUY", "BUY_SMALL"):
        return "BULLISH_BIAS", "偏多觀察"
    if regime.get("primary") == "TREND_DOWN" or (primary_zone and primary_zone.role == ZoneType.RESISTANCE.value):
        return "BEARISH_BIAS", "偏空觀察"
    if regime.get("primary") == "TREND_UP" and (primary_zone is None or primary_zone.role == ZoneType.SUPPORT.value):
        return "BULLISH_BIAS", "偏多觀察"
    return "NEUTRAL_BIAS", "中性觀察"


def _minimum_rr(primary_zone: Optional[ZoneScore], entry_action_state: str) -> float:
    if primary_zone is None:
        return 0.0
    if entry_action_state == "PROBE_ENTRY":
        return 1.5
    if primary_zone.tier == ZoneTier.TIER_1_MAIN_STRUCTURE.value:
        return 2.0
    if primary_zone.role == ZoneType.RESISTANCE.value:
        return 2.0
    return 1.8


def _rr_gate(primary_zone: Optional[ZoneScore], entry_action_state: str) -> dict[str, Any]:
    minimum = _minimum_rr(primary_zone, entry_action_state)
    rr = primary_zone.risk_reward_ratio if primary_zone else None
    if primary_zone is None:
        return {
            "minimum_rr": None,
            "actual_rr": None,
            "qualified": False,
            "reason_code": "NO_PRIMARY_ZONE",
        }
    if rr is None:
        return {
            "minimum_rr": minimum,
            "actual_rr": None,
            "qualified": False,
            "reason_code": "RR_UNAVAILABLE",
        }
    qualified = float(rr) >= minimum
    return {
        "minimum_rr": minimum,
        "actual_rr": float(rr),
        "qualified": qualified,
        "reason_code": "RR_QUALIFIED" if qualified else "RR_INSUFFICIENT",
    }


def _nearest_decision_zone(zone_scores: list[ZoneScore], current_price: float) -> Optional[ZoneScore]:
    candidates = [
        z for z in zone_scores
        if z.role != ZoneType.AT_ZONE.value
        and z.recent_validation != RecentValidation.EXPIRED.value
        and z.confidence_level != ConfidenceLevel.LOW.value
    ]
    if not candidates:
        candidates = [z for z in zone_scores if z.role != ZoneType.AT_ZONE.value]
    return min(candidates, key=lambda z: _distance_pct_to_zone(z, current_price), default=None)


def _primary_structural_zone(zone_scores: list[ZoneScore]) -> Optional[ZoneScore]:
    candidates = [
        z for z in zone_scores
        if z.role != ZoneType.AT_ZONE.value
        and z.tier == ZoneTier.TIER_1_MAIN_STRUCTURE.value
        and z.recent_validation != RecentValidation.EXPIRED.value
    ]
    return max(
        candidates,
        key=lambda z: (
            _zone_quality_score(z),
            z.confluence_family_count or 1,
            z.confidence,
        ),
        default=None,
    )


def _to_date(value: Any) -> Optional[date]:
    if value is None:
        return None
    if isinstance(value, datetime):
        return value.date()
    if isinstance(value, date):
        return value
    try:
        parsed = datetime.fromisoformat(str(value).replace("Z", "+00:00"))
        return parsed.date()
    except ValueError:
        try:
            return date.fromisoformat(str(value)[:10])
        except ValueError:
            return None


def _metadata_dict(metadata: Optional[dict[str, Any]], key: str) -> dict[str, Any]:
    value = (metadata or {}).get(key)
    return value if isinstance(value, dict) else {}


def _metadata_list(metadata: Optional[dict[str, Any]], key: str, feature_key: str) -> list[str]:
    value = _metadata_dict(metadata, key).get(feature_key)
    if value is None:
        return []
    if isinstance(value, (list, tuple, set)):
        return [str(item) for item in value]
    return [str(value)]


def _format_metadata_date(value: Any) -> Optional[str]:
    parsed = _to_date(value)
    if parsed is not None:
        return parsed.isoformat()
    return str(value) if value is not None else None


def _is_stale(updated_at: Any, analysis_as_of: Any, stale_after_days: int) -> bool:
    updated_date = _to_date(updated_at)
    analysis_date = _to_date(analysis_as_of)
    if updated_date is None or analysis_date is None:
        return False
    return (analysis_date - updated_date).days > stale_after_days


def _data_quality(
    chip_summary: dict[str, Any],
    candle_high: Optional[float],
    candle_low: Optional[float],
    candle_close: Optional[float],
    previous_candle_close: Optional[float],
    global_confidence: Optional[float],
    primary_zone: Optional[ZoneScore],
    metadata: Optional[dict[str, Any]] = None,
) -> dict[str, Any]:
    features: dict[str, dict[str, Any]] = {}
    metadata = metadata or {}
    updated_at_by_feature = _metadata_dict(metadata, "updated_at")
    analysis_as_of = metadata.get("analysis_as_of")
    stale_after_days = int(metadata.get("stale_after_days", DEFAULT_STALE_AFTER_DAYS))
    ohlc_errors: dict[str, list[str]] = {}
    if candle_high is not None and candle_low is not None and candle_high < candle_low:
        ohlc_errors.setdefault("daily_price", []).append("HIGH_BELOW_LOW")
    if (
        candle_high is not None
        and candle_low is not None
        and candle_close is not None
        and not candle_low <= candle_close <= candle_high
    ):
        ohlc_errors.setdefault("daily_price", []).append("CLOSE_OUTSIDE_RANGE")

    def add_feature(
        key: str,
        status: str,
        confidence: float,
        source: str,
        interpretation: str = "AVAILABLE",
        value: Optional[float] = None,
    ) -> None:
        updated_at = updated_at_by_feature.get(key)
        reason_codes = [
            *_metadata_list(metadata, "validation_errors", key),
            *ohlc_errors.get(key, []),
        ]
        if reason_codes:
            status = "INVALID"
            confidence = 0.0
            interpretation = "UNAVAILABLE"
        elif status == "AVAILABLE" and _is_stale(updated_at, analysis_as_of, stale_after_days):
            status = "STALE"
            confidence = min(confidence, 0.35)
            interpretation = "UNAVAILABLE"
            reason_codes.append("DATA_STALE")
        features[key] = {
            "status": status,
            "confidence": confidence,
            "source": source,
            "interpretation": interpretation,
            "value": value,
            "updated_at": _format_metadata_date(updated_at),
            "reason_codes": reason_codes,
        }

    price_complete = all(v is not None for v in (candle_high, candle_low, candle_close))
    add_feature(
        "daily_price",
        "AVAILABLE" if price_complete else "MISSING",
        1.0 if price_complete else 0.0,
        "OHLC",
        value=candle_close,
    )
    add_feature(
        "previous_daily_price",
        "AVAILABLE" if previous_candle_close is not None else "MISSING",
        1.0 if previous_candle_close is not None else 0.0,
        "OHLC",
        value=previous_candle_close,
    )
    chip_missing = bool(chip_summary.get("missing"))
    chip_score = chip_summary.get("score")
    chip_interpretation = "UNAVAILABLE" if chip_missing else "NEUTRAL"
    if chip_score is not None:
        if float(chip_score) < -20.0:
            chip_interpretation = "NEGATIVE"
        elif float(chip_score) > 20.0:
            chip_interpretation = "POSITIVE"
        else:
            chip_interpretation = "NEUTRAL"
    add_feature(
        "chip",
        "MISSING" if chip_missing else "AVAILABLE",
        0.0 if chip_missing else 0.75,
        "CHIP_SUMMARY",
        chip_interpretation,
        float(chip_score) if chip_score is not None else None,
    )
    add_feature(
        "global_confidence",
        "AVAILABLE" if global_confidence is not None else "MISSING",
        0.8 if global_confidence is not None else 0.0,
        "GLOBAL_MODEL",
        "NEGATIVE" if global_confidence is not None and global_confidence < 0.45 else "AVAILABLE",
        global_confidence,
    )
    expected_value = primary_zone.expected_value if primary_zone else None
    risk_reward_ratio = primary_zone.risk_reward_ratio if primary_zone else None
    add_feature(
        "expected_value",
        "AVAILABLE" if expected_value is not None else "MISSING",
        0.7 if expected_value is not None else 0.0,
        "ZONE_SCORE",
        "NEGATIVE" if expected_value is not None and expected_value < 0 else "AVAILABLE",
        expected_value,
    )
    add_feature(
        "risk_reward_ratio",
        "AVAILABLE" if risk_reward_ratio is not None else "MISSING",
        0.7 if risk_reward_ratio is not None else 0.0,
        "ZONE_SCORE",
        "NEGATIVE" if risk_reward_ratio is not None and risk_reward_ratio < 1.5 else "AVAILABLE",
        risk_reward_ratio,
    )

    missing_features = [key for key, item in features.items() if item["status"] == "MISSING"]
    stale_features = [key for key, item in features.items() if item["status"] == "STALE"]
    invalid_features = [key for key, item in features.items() if item["status"] == "INVALID"]
    neutral_features = [key for key, item in features.items() if item["interpretation"] == "NEUTRAL"]
    negative_features = [key for key, item in features.items() if item["interpretation"] == "NEGATIVE"]
    positive_features = [key for key, item in features.items() if item["interpretation"] == "POSITIVE"]

    chip_coverage = 0.0 if chip_summary.get("missing") else 1.0
    price_coverage = 1.0 if price_complete and not any(
        features[key]["status"] == "INVALID" for key in ("daily_price", "previous_daily_price")
    ) else 0.0
    overall = price_coverage * 0.7 + chip_coverage * 0.3
    return {
        "data_mode": "END_OF_DAY",
        "overall_completeness": round(overall, 4),
        "price_data_complete": price_coverage == 1.0,
        "chip_coverage": chip_coverage,
        "missing_features": missing_features,
        "unavailable_features": missing_features,
        "neutral_features": neutral_features,
        "negative_features": negative_features,
        "positive_features": positive_features,
        "features": features,
        "stale_features": stale_features,
        "invalid_features": invalid_features,
        "notes": ["盤後日 K 模式，不含盤中即時確認。"],
    }


def _daily_price_action(
    current_price: float,
    global_volatility: float,
    candle_open: Optional[float],
    candle_high: Optional[float],
    candle_low: Optional[float],
    candle_close: Optional[float],
    previous_candle_open: Optional[float],
    previous_candle_high: Optional[float],
    previous_candle_low: Optional[float],
    previous_candle_close: Optional[float],
) -> dict[str, Any]:
    if candle_high is None or candle_low is None or candle_close is None:
        return {
            "available": False,
            "close_location": None,
            "body_proxy_ratio": None,
            "body_ratio": None,
            "body_ratio_source": "UNAVAILABLE",
            "lower_wick_ratio": None,
            "upper_wick_ratio": None,
            "close_location_state": "UNKNOWN",
            "range_pct": None,
            "range_state": "UNKNOWN",
            "gap_state": "UNKNOWN",
            "follow_through_state": "UNKNOWN",
            "reclaim_rejection_state": "UNKNOWN",
            "signals": [],
        }

    bar_range = max(candle_high - candle_low, 0.0)
    close_location = 0.5 if bar_range == 0 else (candle_close - candle_low) / bar_range
    range_pct = bar_range / max(abs(candle_close), 1e-9)
    if bar_range == 0:
        body_proxy_ratio = 0.0
        body_ratio = 0.0
        body_ratio_source = "DAILY_OPEN" if candle_open is not None else "PREVIOUS_CLOSE_PROXY"
        lower_wick_ratio = 0.0
        upper_wick_ratio = 0.0
    else:
        body_reference = previous_candle_close if previous_candle_close is not None else current_price
        body_ratio_source = "DAILY_OPEN" if candle_open is not None else "PREVIOUS_CLOSE_PROXY"
        body_open = candle_open if candle_open is not None else body_reference
        body_low = min(body_reference, candle_close)
        body_high = max(body_reference, candle_close)
        body_proxy_ratio = abs(candle_close - body_reference) / bar_range
        body_ratio = abs(candle_close - body_open) / bar_range
        lower_wick_ratio = max(0.0, body_low - candle_low) / bar_range
        upper_wick_ratio = max(0.0, candle_high - body_high) / bar_range
    if close_location >= 0.67:
        close_location_state = "CLOSE_NEAR_HIGH"
    elif close_location <= 0.33:
        close_location_state = "CLOSE_NEAR_LOW"
    else:
        close_location_state = "CLOSE_MID_RANGE"

    expanded_threshold = max(0.03, global_volatility * 1.2)
    narrow_threshold = max(0.01, global_volatility * 0.5)
    if range_pct >= expanded_threshold:
        range_state = "RANGE_EXPANSION"
    elif range_pct <= narrow_threshold:
        range_state = "NARROW_RANGE"
    else:
        range_state = "NORMAL_RANGE"

    gap_state = "NO_GAP"
    if previous_candle_close is None:
        gap_state = "UNKNOWN"
    elif candle_low > previous_candle_close:
        gap_state = "GAP_UP_APPROX"
    elif candle_high < previous_candle_close:
        gap_state = "GAP_DOWN_APPROX"

    follow_through_state = "NO_FOLLOW_THROUGH"
    reclaim_rejection_state = "NONE"
    if previous_candle_close is None:
        follow_through_state = "UNKNOWN"
        reclaim_rejection_state = "UNKNOWN"
    else:
        if candle_close > previous_candle_close and close_location >= 0.60:
            follow_through_state = "UPSIDE_FOLLOW_THROUGH"
        elif candle_close < previous_candle_close and close_location <= 0.40:
            follow_through_state = "DOWNSIDE_FOLLOW_THROUGH"
        if candle_low < previous_candle_close < candle_close:
            reclaim_rejection_state = "PREVIOUS_CLOSE_RECLAIM"
        elif candle_high > previous_candle_close > candle_close:
            reclaim_rejection_state = "PREVIOUS_CLOSE_REJECTION"

    signals = [
        state for state in (
            close_location_state,
            range_state,
            gap_state if gap_state != "NO_GAP" else None,
            follow_through_state if follow_through_state not in ("NO_FOLLOW_THROUGH", "UNKNOWN") else None,
            reclaim_rejection_state if reclaim_rejection_state not in ("NONE", "UNKNOWN") else None,
        )
        if state
    ]
    return {
        "available": True,
        "close_location": round(float(close_location), 4),
        # 沒有 daily open 時不可宣稱是精準 K 棒 body；先用 previous close/current close 近似。
        "body_proxy_ratio": round(float(body_proxy_ratio), 4),
        "body_ratio": round(float(body_ratio), 4),
        "body_ratio_source": body_ratio_source,
        "lower_wick_ratio": round(float(lower_wick_ratio), 4),
        "upper_wick_ratio": round(float(upper_wick_ratio), 4),
        "close_location_state": close_location_state,
        "range_pct": round(float(range_pct), 4),
        "range_state": range_state,
        "gap_state": gap_state,
        "follow_through_state": follow_through_state,
        "reclaim_rejection_state": reclaim_rejection_state,
        "signals": signals,
        "reference_prices": {
            "high": candle_high,
            "low": candle_low,
            "open": candle_open,
            "close": candle_close,
            "previous_open": previous_candle_open,
            "previous_high": previous_candle_high,
            "previous_low": previous_candle_low,
            "previous_close": previous_candle_close,
            "current_price": current_price,
        },
    }


def _event_sequence(market_events: list[dict[str, Any]]) -> list[dict[str, Any]]:
    order = {
        "EXTREME_VOLUME": 10,
        "HIGH_VOLUME_BREAKDOWN": 20,
        "INTRADAY_RECLAIM": 30,
        "REVERSAL_CANDIDATE": 40,
    }
    labels = {
        "EXTREME_VOLUME": "極端量能",
        "HIGH_VOLUME_BREAKDOWN": "放量破位",
        "INTRADAY_RECLAIM": "盤中收復",
        "REVERSAL_CANDIDATE": "反轉候選",
    }
    seen: set[str] = set()
    sequence: list[dict[str, Any]] = []
    for event in sorted(market_events, key=lambda e: order.get(str(e.get("type")), 999)):
        event_type = str(event.get("type"))
        if event_type in seen:
            continue
        seen.add(event_type)
        sequence.append({
            "type": event_type,
            "label": labels.get(event_type, event_type),
            "direction": event.get("direction"),
            "confidence": event.get("confidence"),
            "price_level": event.get("price_level"),
        })
    return sequence


def _daily_candidate_zone(
    price_low: float,
    price_high: float,
    role: str,
    current_price: float,
    reason: str,
    event_refs: list[str],
) -> dict[str, Any]:
    distance_pct = 0.0
    if current_price < price_low:
        distance_pct = (price_low - current_price) / current_price
    elif current_price > price_high:
        distance_pct = (current_price - price_high) / current_price
    return {
        "price_low": price_low,
        "price_high": price_high,
        "label": f"{_fmt_price(price_low)} ~ {_fmt_price(price_high)}",
        "role": role,
        "source": "DAILY_CANDLE",
        "lifecycle": "CANDIDATE",
        "decision_role": "TACTICAL",
        "distance_pct": distance_pct,
        "distance_label": f"{distance_pct * 100:.1f}%",
        "reason": reason,
        "event_refs": event_refs,
    }


def _daily_candidate_zones(
    current_price: float,
    candle_high: Optional[float],
    candle_low: Optional[float],
    candle_close: Optional[float],
    daily_price_action: dict[str, Any],
    market_events: list[dict[str, Any]],
    nearest_zone: Optional[ZoneScore],
) -> list[dict[str, Any]]:
    if candle_high is None or candle_low is None or candle_close is None:
        return []
    nearest_distance = _distance_pct_to_zone(nearest_zone, current_price) if nearest_zone else None
    needs_daily_candidate = nearest_distance is None or nearest_distance > 0.03
    event_types = [str(event.get("type")) for event in market_events]
    if "INTRADAY_RECLAIM" in event_types or "REVERSAL_CANDIDATE" in event_types:
        needs_daily_candidate = True
    if not needs_daily_candidate:
        return []

    width = max(abs(candle_close) * 0.0025, (candle_high - candle_low) * 0.08)
    support_high = min(candle_close, candle_low + width)
    resistance_low = max(candle_close, candle_high - width)
    candidates = [
        _daily_candidate_zone(
            candle_low,
            max(candle_low, support_high),
            ZoneType.SUPPORT.value,
            current_price,
            "日 K 低點與收盤位置形成的短線支撐候選。",
            [s for s in daily_price_action.get("signals", []) if "RECLAIM" in s or "LOW" in s],
        ),
        _daily_candidate_zone(
            min(candle_high, resistance_low),
            candle_high,
            ZoneType.RESISTANCE.value,
            current_price,
            "日 K 高點與收盤位置形成的短線壓力候選。",
            [s for s in daily_price_action.get("signals", []) if "REJECTION" in s or "HIGH" in s],
        ),
    ]
    return candidates


def _price_path(
    zone_scores: list[ZoneScore],
    current_price: float,
    primary_zone: Optional[ZoneScore],
    nearest_zone: Optional[ZoneScore],
    structural_zone: Optional[ZoneScore],
    daily_candidate_zones: list[dict[str, Any]],
    structure_state: str,
    rr_gate: dict[str, Any],
) -> dict[str, Any]:
    if primary_zone is not None and primary_zone.role == ZoneType.SUPPORT.value:
        invalidation_price = primary_zone.price_low
        recovery_price = primary_zone.price_high
    elif primary_zone is not None and primary_zone.role == ZoneType.RESISTANCE.value:
        invalidation_price = primary_zone.price_high
        recovery_price = primary_zone.price_low
    else:
        invalidation_price = None
        recovery_price = None

    next_decision_price = None
    next_decision_source = None
    if nearest_zone is not None:
        if current_price < nearest_zone.price_low:
            next_decision_price = nearest_zone.price_low
        elif current_price > nearest_zone.price_high:
            next_decision_price = nearest_zone.price_high
        else:
            next_decision_price = current_price
        next_decision_source = "nearest_decision_zone"
    elif daily_candidate_zones:
        candidate = min(daily_candidate_zones, key=lambda z: abs(float(z["price_low"]) - current_price))
        next_decision_price = candidate["price_low"] if current_price < candidate["price_low"] else candidate["price_high"]
        next_decision_source = "daily_candidate_zone"

    resistance_candidates = [
        z for z in zone_scores
        if z.role == ZoneType.RESISTANCE.value and z.price_high >= current_price
    ]
    blocking_zone: Optional[dict[str, Any]] = None
    if resistance_candidates:
        zone = min(resistance_candidates, key=lambda z: _distance_pct_to_zone(z, current_price))
        blocking_zone = {
            "price_low": zone.price_low,
            "price_high": zone.price_high,
            "label": f"{_fmt_price(zone.price_low)} ~ {_fmt_price(zone.price_high)}",
            "role": zone.role,
            "source": "HISTORICAL_SR",
        }
    elif daily_candidate_zones:
        candidate_resistance = next((z for z in daily_candidate_zones if z["role"] == ZoneType.RESISTANCE.value), None)
        if candidate_resistance:
            blocking_zone = {
                "price_low": candidate_resistance["price_low"],
                "price_high": candidate_resistance["price_high"],
                "label": candidate_resistance["label"],
                "role": candidate_resistance["role"],
                "source": candidate_resistance["source"],
            }

    if structure_state in ("SUPPORT_RECLAIM_INVALIDATED", "BREAKDOWN"):
        path_state = "INVALIDATION_RISK"
    elif primary_zone is None and daily_candidate_zones:
        path_state = "DAILY_CANDIDATE_ONLY"
    elif not rr_gate.get("qualified"):
        path_state = "RR_BLOCKED"
    elif blocking_zone is not None and current_price < blocking_zone["price_low"]:
        path_state = "BLOCKING_ZONE_AHEAD"
    else:
        path_state = "OPEN_PATH"

    transitions: list[dict[str, Any]] = []
    if invalidation_price is not None:
        transitions.append({
            "if": f"close_below_{_fmt_price(invalidation_price)}",
            "then": "INVALIDATION_RISK",
            "price": invalidation_price,
        })
    if recovery_price is not None:
        transitions.append({
            "if": f"close_above_{_fmt_price(recovery_price)}",
            "then": "RECOVERY_CONFIRMED",
            "price": recovery_price,
        })
    if next_decision_price is not None:
        transitions.append({
            "if": f"approach_{_fmt_price(float(next_decision_price))}",
            "then": "RECHECK_ENTRY_STATE",
            "price": next_decision_price,
        })
    if structural_zone is not None and structural_zone is not primary_zone:
        transitions.append({
            "if": f"approach_structural_{_fmt_price(structural_zone.price_low)}_{_fmt_price(structural_zone.price_high)}",
            "then": "RECHECK_STRUCTURAL_ZONE",
            "price": structural_zone.price_low,
        })

    return {
        "path_state": path_state,
        "invalidation_price": invalidation_price,
        "recovery_price": recovery_price,
        "next_decision_price": next_decision_price,
        "next_decision_source": next_decision_source,
        "blocking_zone": blocking_zone,
        "transitions": transitions,
    }


def _daily_confirmation(
    primary_zone: Optional[ZoneScore],
    primary_interaction: Optional[dict[str, Any]],
    daily_price_action: dict[str, Any],
    rr_gate: dict[str, Any],
    event_sequence: list[dict[str, Any]],
    entry_action_state: str,
    daily_candidate_zones: list[dict[str, Any]],
    current_price: float,
) -> dict[str, Any]:
    reason_codes: list[str] = []
    event_types = {event.get("type") for event in event_sequence}
    if primary_zone is None:
        state = "WAIT_DAILY_CONFIRM" if daily_candidate_zones else "NO_SETUP"
        reason_codes.append("DAILY_CANDIDATE_ONLY" if daily_candidate_zones else "NO_PRIMARY_ZONE")
    elif primary_interaction and primary_interaction.get("closed_below") and primary_zone.role == ZoneType.SUPPORT.value:
        state = "INVALIDATED"
        reason_codes.append("SUPPORT_CLOSED_BELOW")
    elif not rr_gate.get("qualified"):
        state = "NO_SETUP"
        reason_codes.append(str(rr_gate.get("reason_code") or "RR_NOT_QUALIFIED"))
    elif _distance_pct_to_zone(primary_zone, current_price) > 0.08:
        state = "CHASING_RISK"
        reason_codes.append("PRICE_TOO_FAR_FROM_ZONE")
    elif (
        primary_zone.role == ZoneType.SUPPORT.value
        and primary_interaction
        and primary_interaction.get("touched")
        and primary_interaction.get("closed_above")
        and daily_price_action.get("close_location", 0.0) >= 0.60
    ):
        if entry_action_state in ("ACCUMULATE", "BUY"):
            state = "ENTRY_READY"
            reason_codes.append("DAILY_SUPPORT_RECLAIM_CONFIRMED")
        else:
            state = "PROBE_ALLOWED"
            reason_codes.append("DAILY_SUPPORT_RECLAIM")
    elif "REVERSAL_CANDIDATE" in event_types or "INTRADAY_RECLAIM" in event_types:
        state = "WAIT_DAILY_CONFIRM"
        reason_codes.append("REVERSAL_AWAIT_NEXT_DAILY_CONFIRM")
    elif entry_action_state in ("ACCUMULATE", "BUY"):
        state = "ENTRY_READY"
        reason_codes.append("ENTRY_STATE_READY")
    elif entry_action_state in ("PROBE_ENTRY", "SMALL_ENTRY"):
        state = "PROBE_ALLOWED"
        reason_codes.append("ENTRY_STATE_PROBE")
    else:
        state = "WAIT_DAILY_CONFIRM"
        reason_codes.append("DAILY_CONFIRMATION_REQUIRED")

    labels = {
        "NO_SETUP": "無設定",
        "WAIT_DAILY_CONFIRM": "等待日 K 確認",
        "PROBE_ALLOWED": "允許觀察性試探",
        "ENTRY_READY": "日 K 進場條件成立",
        "CHASING_RISK": "追價風險",
        "INVALIDATED": "設定失效",
    }
    return {
        "state": state,
        "label": labels.get(state, state),
        "reason_codes": reason_codes,
        "requires_next_daily_close": state in ("WAIT_DAILY_CONFIRM", "PROBE_ALLOWED"),
        "source": "END_OF_DAY_DAILY_RULE",
    }


def _structure_state(
    primary_zone: Optional[ZoneScore],
    interaction: Optional[dict[str, Any]],
    previous_interaction: Optional[dict[str, Any]] = None,
) -> str:
    if primary_zone is None or interaction is None:
        return "NORMAL"
    if primary_zone.role == ZoneType.SUPPORT.value:
        if primary_zone.recent_validation == RecentValidation.EXPIRED.value:
            return "BREAKDOWN"
        if interaction["closed_below"]:
            return "SUPPORT_RECLAIM_INVALIDATED"
        if previous_interaction and previous_interaction["touched"] and previous_interaction["closed_above"] and not interaction["closed_below"]:
            return "SUPPORT_RECLAIM_CONFIRMED"
        if interaction["touched"] and interaction["closed_above"]:
            return "SUPPORT_RECLAIM_CANDIDATE"
        if interaction["touched"]:
            return "SUPPORT_RECLAIM_CANDIDATE"
    return "NORMAL"


def _defense_lines(
    zone_scores: list[ZoneScore],
    primary_zone: Optional[ZoneScore],
    current_price: float,
    market_events: Optional[list[dict[str, Any]]] = None,
) -> dict[str, Any]:
    def line(zone: Optional[ZoneScore], source: str) -> Optional[dict[str, Any]]:
        if zone is None:
            return None
        if zone.role == ZoneType.SUPPORT.value:
            invalidation = zone.price_low
            recovery = zone.price_high
        elif zone.role == ZoneType.RESISTANCE.value:
            invalidation = zone.price_high
            recovery = zone.price_low
        else:
            invalidation = zone.price_low
            recovery = zone.price_high
        return {
            "price_low": zone.price_low,
            "price_high": zone.price_high,
            "label": f"{_fmt_price(zone.price_low)} ~ {_fmt_price(zone.price_high)}",
            "role": zone.role,
            "source": source,
            "invalidation_price": invalidation,
            "recovery_price": recovery,
        }

    tactical_zone: Optional[ZoneScore] = None
    for event in market_events or []:
        ref = event.get("zone_ref") or {}
        tactical_zone = next(
            (
                z for z in zone_scores
                if z.price_low == ref.get("price_low") and z.price_high == ref.get("price_high")
            ),
            tactical_zone,
        )
        if tactical_zone is not None:
            break
    if tactical_zone is None:
        tactical_candidates = [
            z for z in zone_scores
            if z.tier == ZoneTier.TIER_3_SHORT_TERM.value and z.role != ZoneType.AT_ZONE.value
        ]
        tactical_zone = min(tactical_candidates, key=lambda z: _distance_pct_to_zone(z, current_price), default=None)

    strategic_candidates = [
        z for z in zone_scores
        if z.tier == ZoneTier.TIER_1_MAIN_STRUCTURE.value and z.role != ZoneType.AT_ZONE.value
    ]
    strategic_zone = max(
        strategic_candidates,
        key=lambda z: ((z.zone_quality_score if z.zone_quality_score is not None else z.trading_score), z.confidence),
        default=None,
    )
    return {
        "tactical": line(tactical_zone, "recent_microstructure"),
        "swing": line(primary_zone, "primary_zone"),
        "strategic": line(strategic_zone, "main_structure"),
    }


def build_decision_summary(
    zone_scores: list[ZoneScore],
    current_price: float,
    global_trend: float,
    global_volatility: float,
    global_metrics: dict[str, Optional[float]],
    chip_summary: dict[str, Any],
    bundle: ModelBundle,
    candle_open: Optional[float] = None,
    candle_high: Optional[float] = None,
    candle_low: Optional[float] = None,
    candle_close: Optional[float] = None,
    previous_candle_open: Optional[float] = None,
    previous_candle_high: Optional[float] = None,
    previous_candle_low: Optional[float] = None,
    previous_candle_close: Optional[float] = None,
    data_quality_metadata: Optional[dict[str, Any]] = None,
) -> dict[str, Any]:
    global_confidence = global_metrics.get("confidence")
    market_events = detect_market_events(zone_scores, current_price, candle_high, candle_low, candle_close)
    trend_regime = _market_regime(global_trend, global_volatility, global_confidence, chip_summary, market_events=market_events)["trend_regime"]
    primary_zone = _pick_primary_zone(zone_scores, current_price, trend_regime, market_events)
    primary_interaction = (
        _zone_interaction(primary_zone, current_price, candle_high, candle_low, candle_close)
        if primary_zone else None
    )
    previous_interaction = (
        _zone_interaction(primary_zone, current_price, previous_candle_high, previous_candle_low, previous_candle_close)
        if primary_zone and previous_candle_close is not None else None
    )
    structure_state = _structure_state(primary_zone, primary_interaction, previous_interaction)
    regime = _market_regime(
        global_trend,
        global_volatility,
        global_confidence,
        chip_summary,
        structure_state,
        market_events,
    )
    market_action, position_action, action, action_label, risk_notes = _decision_action(
        regime, primary_zone, current_price, primary_interaction, market_events
    )
    entry_action_state = _entry_action_state(action, primary_zone, structure_state, current_price, market_events)
    market_bias, market_bias_label = _market_bias(regime, primary_zone, market_action, market_events)
    rr_gate = _rr_gate(primary_zone, entry_action_state)
    nearest_zone = _nearest_decision_zone(zone_scores, current_price)
    structural_zone = _primary_structural_zone(zone_scores)
    daily_price_action = _daily_price_action(
        current_price,
        global_volatility,
        candle_open,
        candle_high,
        candle_low,
        candle_close,
        previous_candle_open,
        previous_candle_high,
        previous_candle_low,
        previous_candle_close,
    )
    event_sequence = _event_sequence(market_events)
    daily_candidate_zones = _daily_candidate_zones(
        current_price,
        candle_high,
        candle_low,
        candle_close,
        daily_price_action,
        market_events,
        nearest_zone,
    )
    price_path = _price_path(
        zone_scores,
        current_price,
        primary_zone,
        nearest_zone,
        structural_zone,
        daily_candidate_zones,
        structure_state,
        rr_gate,
    )
    daily_confirmation = _daily_confirmation(
        primary_zone,
        primary_interaction,
        daily_price_action,
        rr_gate,
        event_sequence,
        entry_action_state,
        daily_candidate_zones,
        current_price,
    )
    best_trade_zone = primary_zone if rr_gate["qualified"] and entry_action_state in (
        "PROBE_ENTRY",
        "SMALL_ENTRY",
        "ACCUMULATE",
        "BUY",
    ) else None

    context = [
        {"key": "trend", "label": "整體趨勢", "value": f"{global_trend * 100:.1f}%"},
        {"key": "volatility", "label": "整體波動", "value": f"{global_volatility * 100:.1f}%"},
        {"key": "global_confidence", "label": "整體信心", "value": _confidence_label(global_confidence, None)},
        {"key": "model", "label": "模型版本", "value": f"{bundle.version}/{bundle.config_hash}" if bundle.config_hash else bundle.version},
    ]
    if chip_summary.get("missing"):
        context.append({"key": "chip", "label": "籌碼", "value": "查無籌碼資料", "effect": "warning"})
    elif chip_summary.get("score") is not None:
        context.append({"key": "chip", "label": "籌碼", "value": f"{float(chip_summary['score']):.1f}", "effect": chip_summary.get("signal")})

    confidence_source = primary_zone
    confidence_explanation = {
        "value": confidence_source.confidence if confidence_source else global_confidence,
        "level": confidence_source.confidence_level if confidence_source else None,
        "label": _confidence_label(
            confidence_source.confidence if confidence_source else global_confidence,
            confidence_source.confidence_level if confidence_source else None,
        ),
        "formula_factors": [
            {
                "key": "sample_factor",
                "value": None,
                "label": "樣本數",
                "description": "目前 API 尚未輸出原始 sample_factor；先以 touch_count 與支撐/壓力方向觸碰數輔助判讀。",
            },
            {
                "key": "recency_factor",
                "value": None,
                "label": "近期性",
                "description": "目前 API 尚未輸出原始 recency_factor；先以 recent_validation 輔助判讀。",
            },
            {
                "key": "stability_factor",
                "value": None,
                "label": "穩定度",
                "description": "目前 API 尚未輸出原始 stability_factor；先以 reject_count/break_count 輔助判讀。",
            },
        ],
        "context_factors": [],
    }
    if primary_zone:
        confidence_explanation["context_factors"] = [
            {"key": "touch_count", "effect": "context", "label": f"觸碰 {primary_zone.touch_count} 次"},
            {"key": "recent_validation", "effect": "context", "label": primary_zone.recent_validation},
            {"key": "confluence", "effect": "supportive" if primary_zone.confluence_family_count > 1 else "neutral", "label": f"證據族群 ×{primary_zone.confluence_family_count}"},
        ]

    secondary = [
        _decision_summary_zone(
            z, current_price, "次要參考區",
            candle_high, candle_low, candle_close,
            decision_role="REFERENCE",
        )
        for z in zone_scores
        if primary_zone is None or z is not primary_zone
    ][:5]
    defense_lines = _defense_lines(zone_scores, primary_zone, current_price, market_events)

    return {
        "data_mode": "END_OF_DAY",
        "data_quality": _data_quality(
            chip_summary,
            candle_high,
            candle_low,
            candle_close,
            previous_candle_close,
            global_confidence,
            primary_zone,
            data_quality_metadata,
        ),
        "market_regime": regime,
        "market_events": market_events,
        "event_sequence": event_sequence,
        "daily_price_action": daily_price_action,
        "daily_candidate_zones": daily_candidate_zones,
        "price_path": price_path,
        "daily_confirmation": daily_confirmation,
        "defense_lines": defense_lines,
        "market_bias": market_bias,
        "market_bias_label": market_bias_label,
        "market_action": market_action,
        "position_action": position_action,
        "position_action_condition": _position_action_condition(primary_zone, structure_state),
        "action": action,
        "action_label": action_label,
        "entry_action_state": entry_action_state,
        "entry_action_label": _entry_action_label(entry_action_state),
        "daily_entry_state": daily_confirmation["state"],
        "daily_entry_label": daily_confirmation["label"],
        "rr_gate": rr_gate,
        "nearest_decision_zone": _decision_summary_zone(
            nearest_zone, current_price, "離現價最近的決策參考區",
            candle_high, candle_low, candle_close,
            decision_role="TACTICAL",
        ) if nearest_zone else None,
        "primary_structural_zone": _decision_summary_zone(
            structural_zone, current_price, "主要結構區",
            candle_high, candle_low, candle_close,
            decision_role="STRUCTURAL",
        ) if structural_zone else None,
        "best_trade_zone": _decision_summary_zone(
            best_trade_zone, current_price, "通過 RR 與進場條件的交易候選區",
            candle_high, candle_low, candle_close,
            decision_role="TRADE_CANDIDATE",
        ) if best_trade_zone else None,
        "primary_zone": _decision_summary_zone(
            primary_zone, current_price, "目前最具決策意義的主交易區",
            candle_high, candle_low, candle_close,
            decision_role="PRIMARY",
        ) if primary_zone else None,
        "market_context": context,
        "confidence_explanation": confidence_explanation,
        "risk_notes": risk_notes,
        "secondary_zones": secondary,
    }


def build_decision_from_evidence(evidence: AnalysisEvidence) -> dict[str, Any]:
    """Decision's sole public input is the immutable Evidence stage output."""
    scores = evidence.scores
    frame = scores.features.data.frame
    last = frame.iloc[-1]
    previous = frame.iloc[-2] if len(frame) >= 2 else None
    last_timestamp = frame.index[-1] if len(frame.index) else None
    previous_timestamp = frame.index[-2] if previous is not None else None
    return build_decision_summary(
        list(scores.zones),
        scores.features.data.current_price,
        scores.features.global_trend,
        scores.features.global_volatility,
        scores.global_metrics,
        scores.chip_summary,
        scores.features.data.model,
        candle_open=float(last["open"]),
        candle_high=float(last["high"]),
        candle_low=float(last["low"]),
        candle_close=float(last["close"]),
        previous_candle_open=float(previous["open"]) if previous is not None else None,
        previous_candle_high=float(previous["high"]) if previous is not None else None,
        previous_candle_low=float(previous["low"]) if previous is not None else None,
        previous_candle_close=float(previous["close"]) if previous is not None else None,
        data_quality_metadata={
            "analysis_as_of": scores.features.data.analyzed_at,
            "updated_at": {
                "daily_price": last_timestamp,
                "previous_daily_price": previous_timestamp,
                "chip": scores.chip_summary.get("trade_date"),
            },
        },
    )
