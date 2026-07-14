"""SR Zone Decision Engine v2.

This module turns SR Zone scoring output into market/position decision context.
It is intentionally portfolio-agnostic: no holding cost, shares, PnL, position
sizing, or order execution logic belongs here.
"""
from __future__ import annotations

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
) -> dict[str, Any]:
    interaction = _zone_interaction(z, current_price, candle_high, candle_low, candle_close)
    return {
        "price_low": z.price_low,
        "price_high": z.price_high,
        "label": f"{_fmt_price(z.price_low)} ~ {_fmt_price(z.price_high)}",
        "role": z.role,
        "tier": z.tier,
        "tier_label": z.tier_label,
        "trading_score": z.trading_score,
        "zone_quality_score": z.zone_quality_score if z.zone_quality_score is not None else z.trading_score,
        # 對外一律回報 base entry relevance（不含市場事件），與 zones[] 的
        # entry_relevance_score 同定義；事件影響另由 market_events / short_term_regime 呈現。
        "entry_relevance_score": _entry_relevance_score(z, current_price),
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
        "reason": reason,
    }


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
        "structure_state": structure_state,
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
    rr_score = 0.5 if z.risk_reward_ratio is None else min(z.risk_reward_ratio / 3.0, 1.0)
    confluence_score = min((z.confluence_family_count or 1) / 3.0, 1.0)
    role_alignment = 0.0
    if regime_primary in ("TREND_UP", "RANGE_BOUND") and z.role == ZoneType.SUPPORT.value:
        role_alignment = 1.0
    elif regime_primary == "TREND_DOWN" and z.role == ZoneType.RESISTANCE.value:
        role_alignment = 1.0
    return (
        _entry_relevance_score_with_events(z, current_price, market_events) / 100.0 * 0.65
        + _zone_quality_score(z) / 100.0 * 0.15
        + rr_score * 0.07
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
            "AVOID",
            "REDUCE_ON_BREAKDOWN",
            "Avoid",
            "避開",
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
    candle_high: Optional[float] = None,
    candle_low: Optional[float] = None,
    candle_close: Optional[float] = None,
    previous_candle_high: Optional[float] = None,
    previous_candle_low: Optional[float] = None,
    previous_candle_close: Optional[float] = None,
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
        _decision_summary_zone(z, current_price, "次要參考區", candle_high, candle_low, candle_close)
        for z in zone_scores
        if primary_zone is None or z is not primary_zone
    ][:5]
    defense_lines = _defense_lines(zone_scores, primary_zone, current_price, market_events)

    return {
        "market_regime": regime,
        "market_events": market_events,
        "defense_lines": defense_lines,
        "market_action": market_action,
        "position_action": position_action,
        "position_action_condition": _position_action_condition(primary_zone, structure_state),
        "action": action,
        "action_label": action_label,
        "entry_action_state": entry_action_state,
        "entry_action_label": _entry_action_label(entry_action_state),
        "primary_zone": _decision_summary_zone(
            primary_zone, current_price, "目前最具決策意義的主交易區",
            candle_high, candle_low, candle_close,
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
    return build_decision_summary(
        list(scores.zones),
        scores.features.data.current_price,
        scores.features.global_trend,
        scores.features.global_volatility,
        scores.global_metrics,
        scores.chip_summary,
        scores.features.data.model,
        candle_high=float(last["high"]),
        candle_low=float(last["low"]),
        candle_close=float(last["close"]),
        previous_candle_high=float(previous["high"]) if previous is not None else None,
        previous_candle_low=float(previous["low"]) if previous is not None else None,
        previous_candle_close=float(previous["close"]) if previous is not None else None,
    )
