"""SR Zone Decision Engine v2.

This module turns SR Zone scoring output into market/position decision context.
It is intentionally portfolio-agnostic: no holding cost, shares, PnL, position
sizing, or order execution logic belongs here.
"""
from __future__ import annotations

from typing import Any, Optional

from .model import ModelBundle
from .types import ConfidenceLevel, RecentValidation, ZoneScore, ZoneType
from .pipeline_types import AnalysisEvidence


def _fmt_price(v: float) -> str:
    return f"{v:.2f}"


def _distance_pct_to_zone(z: ZoneScore, current_price: float) -> float:
    if z.price_low <= current_price <= z.price_high:
        return 0.0
    if current_price < z.price_low:
        return (z.price_low - current_price) / current_price
    return (current_price - z.price_high) / current_price


def _zone_interaction(
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
        "reason": reason,
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
        "RECOVERY_CANDIDATE": "短線測試支撐",
        "RECOVERY": "短線收回支撐",
        "RECOVERY_INVALIDATED": "短線結構轉弱",
        "BREAKDOWN": "短線結構跌破",
    }.get(structure_state, structure_state)
    label = trend_label
    if structure_state in ("RECOVERY_INVALIDATED", "BREAKDOWN"):
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
        "structure_state": structure_state,
        "volatility_state": volatility_state,
        "flags": flags,
        "label": label,
        "reasons": reasons[:4],
    }


def _primary_zone_score(z: ZoneScore, current_price: float, regime_primary: str) -> float:
    distance_pct = _distance_pct_to_zone(z, current_price)
    distance_score = max(0.0, 1.0 - min(distance_pct / 0.08, 1.0))
    ev_score = 0.5 if z.expected_value is None else max(0.0, min((z.expected_value + 0.03) / 0.08, 1.0))
    rr_score = 0.5 if z.risk_reward_ratio is None else min(z.risk_reward_ratio / 3.0, 1.0)
    confluence_score = min(z.confluence_count / 3.0, 1.0)
    role_bonus = 0.0
    if regime_primary in ("TREND_UP", "RANGE_BOUND") and z.role == ZoneType.SUPPORT.value:
        role_bonus = 0.08
    elif regime_primary == "TREND_DOWN" and z.role == ZoneType.RESISTANCE.value:
        role_bonus = 0.08
    return (
        z.trading_score / 100.0 * 0.35
        + z.confidence * 0.20
        + distance_score * 0.18
        + ev_score * 0.12
        + rr_score * 0.10
        + confluence_score * 0.05
        + role_bonus
    )


def _pick_primary_zone(zone_scores: list[ZoneScore], current_price: float, regime_primary: str) -> Optional[ZoneScore]:
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
    return max(candidates, key=lambda z: _primary_zone_score(z, current_price, regime_primary))


def _decision_action(
    regime: dict[str, Any],
    primary_zone: Optional[ZoneScore],
    current_price: float,
    interaction: Optional[dict[str, Any]],
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
    structure_broken = structure_state in ("RECOVERY_INVALIDATED", "BREAKDOWN")
    if structure_broken:
        risk_notes.append("短線結構轉弱，市場訊號不得覆蓋跌破風險。")
        position_action = "EXIT" if structure_state == "BREAKDOWN" else "REDUCE_ON_BREAKDOWN"
        return "AVOID", position_action, "Avoid", "避開", risk_notes
    if bearish_setup and not bullish_setup:
        return "AVOID", "REDUCE", "Avoid", "避開", risk_notes
    if rr is None or rr < 1.5:
        return "WATCH", "HOLD", "Hold", "等待", risk_notes

    strong = (
        primary_zone.trading_score >= 70
        and primary_zone.confidence >= 0.65
        and (primary_zone.expected_value or 0) > 0
        and rr >= 2.0
        and distance_pct <= 0.05
        and not flags
    )
    constructive = (
        bullish_setup
        and primary_zone.trading_score >= 55
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


def _structure_state(primary_zone: Optional[ZoneScore], interaction: Optional[dict[str, Any]]) -> str:
    if primary_zone is None or interaction is None:
        return "NORMAL"
    if primary_zone.role == ZoneType.SUPPORT.value:
        if primary_zone.recent_validation == RecentValidation.EXPIRED.value:
            return "BREAKDOWN"
        if interaction["closed_below"]:
            return "RECOVERY_INVALIDATED"
        if interaction["touched"] and interaction["closed_above"]:
            return "RECOVERY"
        if interaction["touched"]:
            return "RECOVERY_CANDIDATE"
    return "NORMAL"


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
) -> dict[str, Any]:
    global_confidence = global_metrics.get("confidence")
    trend_regime = _market_regime(global_trend, global_volatility, global_confidence, chip_summary)["trend_regime"]
    primary_zone = _pick_primary_zone(zone_scores, current_price, trend_regime)
    primary_interaction = (
        _zone_interaction(primary_zone, current_price, candle_high, candle_low, candle_close)
        if primary_zone else None
    )
    regime = _market_regime(
        global_trend,
        global_volatility,
        global_confidence,
        chip_summary,
        _structure_state(primary_zone, primary_interaction),
    )
    market_action, position_action, action, action_label, risk_notes = _decision_action(
        regime, primary_zone, current_price, primary_interaction
    )

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
            {"key": "confluence", "effect": "supportive" if primary_zone.confluence_count > 1 else "neutral", "label": f"多方法共振 ×{primary_zone.confluence_count}"},
        ]

    secondary = [
        _decision_summary_zone(z, current_price, "次要參考區", candle_high, candle_low, candle_close)
        for z in zone_scores
        if primary_zone is None or z is not primary_zone
    ][:5]

    return {
        "market_regime": regime,
        "market_action": market_action,
        "position_action": position_action,
        "action": action,
        "action_label": action_label,
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
    )
