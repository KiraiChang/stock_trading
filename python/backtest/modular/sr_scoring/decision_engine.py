"""SR Zone Decision Engine v2.

This module turns SR Zone scoring output into market/position decision context.
It is intentionally portfolio-agnostic: no holding cost, shares, PnL, position
sizing, or order execution logic belongs here.
"""
from __future__ import annotations

from datetime import date, datetime
from typing import Any, Optional

from .event_engine import (
    build_event_state_summary,
    detect_market_events,
    is_decision_visible,
    entry_relevance_base_breakdown as _entry_relevance_base_breakdown,
    zone_interaction as _zone_interaction,
    _clamp_relevance,
    _distance_pct_to_zone,
    _fmt_price,
)
from .lifecycle_engine import (
    event_state_types as _event_state_types,
    resolve_lifecycle,
)
from .model import ModelBundle
from .types import ConfidenceLevel, RecentValidation, ZoneScore, ZoneTier, ZoneType
from .pipeline_types import AnalysisEvidence
from .probability_engine import build_model_governance_context
from .labels import role_label as _role_label, display_label as _display_label


DEFAULT_STALE_AFTER_DAYS = 1
MARKET_PRICE_ENTRY_BASES = {"RECLAIM_CLOSE", "CONTINUATION_MARKET_PRICE"}


def _risk_note(code: str, text: str) -> dict[str, str]:
    return {"code": code, "text": text}


def _risk_note_text(note: Any) -> str:
    if isinstance(note, dict):
        return str(note.get("text") or "")
    return str(note)


def _risk_note_code(note: Any) -> str:
    if isinstance(note, dict):
        return str(note.get("code") or "")
    return ""


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


def _zone_width_penalty(z: ZoneScore, current_price: float) -> float:
    width_pct = (z.price_high - z.price_low) / max(abs(current_price), 1e-9)
    if width_pct <= 0.03:
        return 0.0
    return min((width_pct - 0.03) / 0.04, 1.0)


def _decision_distance_score(z: ZoneScore, current_price: float) -> float:
    return _distance_pct_to_zone(z, current_price) + _zone_width_penalty(z, current_price) * 0.08


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
    zone_width_pct = (z.price_high - z.price_low) / max(abs(current_price), 1e-9)
    zone_width_penalty = _zone_width_penalty(z, current_price)
    structural_score = _zone_quality_score(z)
    decision_relevance_score = _entry_relevance_score(z, current_price)
    tradability_score = float(z.trading_score)
    return {
        "price_low": z.price_low,
        "price_high": z.price_high,
        "label": f"{_fmt_price(z.price_low)} ~ {_fmt_price(z.price_high)}",
        "role": z.role,
        "role_label": _role_label(z.role),
        "tier": z.tier,
        "tier_label": z.tier_label,
        "display_label": _display_label(z.tier, z.role, z.tier_label),
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
        "zone_width_pct": round(float(zone_width_pct), 4),
        "zone_width_penalty": round(float(zone_width_penalty), 4),
        "zone_interaction": interaction,
        "recent_validation": z.recent_validation,
        "volume_confirmation": z.volume_confirmation,
        "confluence_count": z.confluence_count,
        "confluence_family_count": z.confluence_family_count,
        "confluence_families": list(z.confluence_families),
        "source": source,
        # zone_health_state 是 zone 本身的健康度，與 semantic_pipeline.lifecycle_phase
        # （整體事件演進）是**不同的軸**。兩個都叫 lifecycle 是這一塊長年難讀的主因，
        # 所以新增語意清楚的鍵。舊的 "lifecycle" 保留不動——前端 SRZones.svelte 有 5 處
        # 在消費它，破壞性改名會把「引擎抽離」與「API/前端 contract 遷移」綁成同一批，
        # 兩者的風險性質完全不同。顯示名稱的收斂留給 T-041。
        "zone_health_state": _zone_health_state(z, interaction),
        "lifecycle": _zone_health_state(z, interaction),  # deprecated：改用 zone_health_state
        "decision_role": decision_role,
        "reason": reason,
    }


def _zone_health_state(z: ZoneScore, interaction: Optional[dict[str, Any]] = None) -> str:
    """zone **本身**的健康度：候選 → 驗證過 → 確認 / 轉弱 / 跌破 / 失效。

    與 lifecycle_engine 的 `lifecycle_phase`（整體事件演進）是不同的軸，
    也與 scenario_engine._zone_state（場景判定：SUPPORT_RETEST / RETEST_REQUIRED…）不同——
    **三者都在描述 zone，但問的是三個不同的問題**，這正是原本命名混亂的來源。
    值域也不同——這裡的 CONFIRMED 指「zone 被收復確認」，那裡的 CONFIRMED 指
    「收復事件已確認」。原名 `_zone_lifecycle` 與另外三套 lifecycle 同名不同義，
    是這一塊難讀的主因，因此更名（見 todo.md T-044）。
    """
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


def _position_action_condition(
    primary_zone: Optional[ZoneScore],
    structure_state: str,
    derived_view: Optional[dict[str, Any]] = None,
) -> dict[str, Any]:
    semantic_pipeline = (derived_view or {}).get("semantic_pipeline") or {}
    position_state = str(semantic_pipeline.get("action_state") or "WATCH")
    derived_reasons = list((derived_view or {}).get("position_reason_codes") or [])
    if primary_zone is None:
        return {
            "state": position_state,
            "structure_state": structure_state,
            "invalidation_price": None,
            "recovery_price": None,
            "reason_codes": _unique_reason_codes(["NO_PRIMARY_ZONE", *derived_reasons]),
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
            "state": position_state,
            "structure_state": structure_state,
            "invalidation_price": primary_zone.price_low,
            "recovery_price": primary_zone.price_high,
            "reason_codes": _unique_reason_codes([*reason_codes, *derived_reasons]),
        }

    if primary_zone.role == ZoneType.RESISTANCE.value:
        return {
            "state": position_state,
            "structure_state": structure_state,
            "invalidation_price": primary_zone.price_high,
            "recovery_price": primary_zone.price_low,
            "reason_codes": _unique_reason_codes(["PRIMARY_RESISTANCE", "UPSIDE_BREAKOUT_REQUIRED", *derived_reasons]),
        }

    return {
        "state": position_state,
        "structure_state": structure_state,
        "invalidation_price": primary_zone.price_low,
        "recovery_price": primary_zone.price_high,
        "reason_codes": _unique_reason_codes(["WAIT_FOR_DIRECTION", *derived_reasons]),
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
    event_state_summary: Optional[dict[str, Any]] = None,
    model_governance: Optional[dict[str, Any]] = None,
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
    model_health_state = str((model_governance or {}).get("health_state") or "UNKNOWN")
    if model_health_state == "UNRELIABLE":
        flags.append("MODEL_UNRELIABLE")
        reasons.append("模型健康度不可用，進場條件需阻擋。")
    elif model_health_state == "DEGRADED":
        flags.append("MODEL_DEGRADED")
        reasons.append("模型健康度降級，進場條件需保守。")
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
    market_state = str((event_state_summary or {}).get("market_state") or "NORMAL")
    if structure_state == "SUPPORT_RECLAIM_CONFIRMED":
        short_term_regime = "RECOVERY"
    elif market_state == "BREAKDOWN_RISK" or structure_state in ("SUPPORT_RECLAIM_INVALIDATED", "BREAKDOWN"):
        short_term_regime = "BREAKDOWN_RISK"
    elif market_state == "RECLAIM_ATTEMPT" or "INTRADAY_RECLAIM" in event_types:
        short_term_regime = "RECLAIM_ATTEMPT"
    elif market_state == "REVERSAL_CANDIDATE" or "REVERSAL_CANDIDATE" in event_types:
        short_term_regime = "REVERSAL_CANDIDATE"
    elif trend_regime == "RANGE_BOUND" and global_trend > 0 and global_confidence is not None and global_confidence >= 0.55:
        short_term_regime = "EARLY_TREND"
    else:
        short_term_regime = "NORMAL"
    tactical_regime = short_term_regime
    recovery_state = "RECOVERY" if structure_state == "SUPPORT_RECLAIM_CONFIRMED" else structure_state

    label = trend_label
    if structure_state in ("SUPPORT_RECLAIM_INVALIDATED", "BREAKDOWN"):
        label = f"長期{trend_label.replace('趨勢', '')}，但{structure_label}"
    elif structure_state == "SUPPORT_RECLAIM_CONFIRMED":
        label += "、短線收復確認"
    elif short_term_regime == "EARLY_TREND":
        label += "、早期趨勢"
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
        "market_state": market_state,
        "tactical_regime": tactical_regime,
        "structure_state": structure_state,
        "recovery_state": recovery_state,
        "volatility_state": volatility_state,
        "model_health_state": model_health_state,
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
    structure_state: str = "NORMAL",
) -> tuple[str, str, str, str, list[str]] | None:
    if structure_state == "SUPPORT_RECLAIM_CONFIRMED":
        return None
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
    if "MODEL_UNRELIABLE" in flags:
        risk_notes.append("模型健康度不可用，暫不允許依機率模型進場。")
    elif "MODEL_DEGRADED" in flags:
        risk_notes.append(_risk_note("MODEL_DEGRADED_ENTRY_TONE", "模型健康度降級，最多小量或觀察。"))
    if primary_zone is None:
        risk_notes.append("沒有足夠明確的主交易區。")
        return "WATCH", "HOLD", "Hold", "等待", risk_notes
    if "MODEL_UNRELIABLE" in flags:
        return "WATCH", "HOLD", "Hold", "等待", risk_notes

    structure_state = regime.get("structure_state")
    breakdown_action = _high_volume_breakdown_action(market_events, primary_zone, str(structure_state))
    if breakdown_action is not None:
        event_risk_notes = breakdown_action[4]
        return breakdown_action[0], breakdown_action[1], breakdown_action[2], breakdown_action[3], risk_notes + event_risk_notes
    if (
        structure_state != "SUPPORT_RECLAIM_CONFIRMED"
        and any(event.get("type") == "HIGH_VOLUME_BREAKDOWN" for event in market_events or [])
    ):
        risk_notes.append("短線支撐出現高量跌破，但尚未達主結構防守門檻。")

    distance_pct = _distance_pct_to_zone(primary_zone, current_price)
    if distance_pct > 0.08:
        risk_notes.append("現價離主交易區較遠，不適合追價。")
    rr = primary_zone.risk_reward_ratio
    if rr is None:
        risk_notes.append("主交易區缺少風險報酬比，先觀察。")
    elif rr < 1.5:
        risk_notes.append("主交易區風險報酬比不足。")
    elif rr < FULL_ENTRY_MIN_RR:
        # 這裡用 **setup RR** 先給一個初值——`_decision_action` 跑在 execution gate 之前，
        # 拿不到 `secondary_gate`。真正的判定由 `_final_entry_risk_notes` 依
        # `secondary_gate.qualified` 校正（T-055），兩者共用 FULL_ENTRY_MIN_RR。
        risk_notes.append(_risk_note("RR_BELOW_FULL_ENTRY", "風險報酬比未達完整買進門檻，最多小量試單。"))
    if primary_zone.recent_validation == RecentValidation.EXPIRED.value:
        risk_notes.append("主交易區近期驗證偏失效。")

    primary = regime.get("primary")
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


def _final_action_from_entry(
    final_entry_permission: dict[str, Any],
    market_action: str,
    position_action: str,
    action: str,
    action_label: str,
    rr_gate: Optional[dict[str, Any]] = None,
) -> tuple[str, str, str, str]:
    # ── T-055：`strong` 讀 setup RR，但完整部位要由 `secondary_gate` 說了算 ──────
    #
    # `_decision_action` 的 `strong` 條件含 `rr >= 2.0`，而那個 `rr` 是
    # **primary_zone.risk_reward_ratio（setup RR）**；execution gate 要到本函式之前
    # 才建立。於是存在這條路徑：
    #
    #   setup RR >= 2.0 → strong → action=Buy
    #   executable RR 介於 probe 門檻與 2.0 之間 → 主 gate qualified=true
    #   secondary_gate.qualified=false
    #   → 畫面同時出現「Buy」與「完整部位門檻未通過」
    #
    # 那是 T-055 明定不得發生的組合。**risk_notes 的事後校正只能改文字、改不了 action**，
    # 所以降級要在這裡做：probe 過得了但 full entry 過不了 → 最多小量試單。
    if action == "Buy":
        secondary = (rr_gate or {}).get("secondary_gate") or {}
        if secondary.get("qualified") is False:
            action, action_label = "BuySmall", "小量試單"
            if market_action == "BUY":
                market_action = "BUY_SMALL"

    state = str(final_entry_permission.get("state") or "WAIT_CONFIRMATION")
    if state == "BLOCKED":
        if market_action == "AVOID" or action == "Avoid":
            return "AVOID", position_action, "Avoid", "避開"
        return "WATCH", position_action, "Hold", "等待"
    if state == "WAIT_CONFIRMATION":
        return "WATCH", position_action, "Hold", "等待"
    if state == "PROBE_ALLOWED":
        if action == "Buy":
            return "BUY_SMALL", position_action, "BuySmall", "小量試單"
        return market_action, position_action, action, action_label
    return market_action, position_action, action, action_label


def _final_entry_risk_notes(
    risk_notes: list[Any],
    final_entry_permission: dict[str, Any],
    entry_executability: Optional[dict[str, Any]] = None,
    entry_blocking_zone: Optional[dict[str, Any]] = None,
    rr_context: Optional[dict[str, Any]] = None,
    rr_gate: Optional[dict[str, Any]] = None,
) -> list[str]:
    state = str(final_entry_permission.get("state") or "WAIT_CONFIRMATION")
    reason_codes = set(str(code) for code in final_entry_permission.get("reason_codes") or [])
    notes = list(risk_notes)

    # ── T-055：`RR_BELOW_FULL_ENTRY` 綁定到 `secondary_gate` ──────────────────
    #
    # `_decision_action` 依 **setup RR** 給初值，但那可能與 gate 實際測的
    # `actual_rr` 不同——0050 就是 setup 5.71（不會出這條 note）而
    # executable 0.87（secondary gate 不合格）。不校正的話，畫面會出現
    # 「gate 說未達完整買進門檻」但 risk_notes 一句都沒提，或反過來。
    #
    # **必須在下面那個文字改寫迴圈之前做**：那個迴圈會把帶 code 的 dict 換成純字串，
    # 之後就對不回 `RR_BELOW_FULL_ENTRY` 了。
    #
    # ⚠️ **`qualified is False` 不足以掛這條 note**：`actual_rr=null`（`NO_PRIMARY_ZONE` /
    # `RR_UNAVAILABLE`——連 zone 統計都不存在）時 secondary 也是 False，但
    # 「未達門檻」是一個**需要數字支撐**的判斷，沒有數字就不能宣稱。那種情況畫面上
    # 已經有「主交易區缺少風險報酬比，先觀察。」，再補一句「未達門檻」是兩句互相矛盾。
    # 所以掛 note 的條件是 **actual_rr 有值 且 secondary 不合格**；其餘一律移除，
    # 連 `_decision_action` 依 setup RR 給的初值也要清掉（那個初值同樣沒有 gate 數字撐）。
    secondary = (rr_gate or {}).get("secondary_gate") or {}
    if "qualified" in secondary:
        has_note = any(_risk_note_code(n) == "RR_BELOW_FULL_ENTRY" for n in notes)
        supported = (rr_gate or {}).get("actual_rr") is not None and not secondary.get("qualified")
        if supported and not has_note:
            notes.append(_risk_note(
                "RR_BELOW_FULL_ENTRY", "風險報酬比未達完整買進門檻，最多小量試單。"))
        elif not supported and has_note:
            notes = [n for n in notes if _risk_note_code(n) != "RR_BELOW_FULL_ENTRY"]
    if state in ("BLOCKED", "WAIT_CONFIRMATION"):
        cleaned_notes: list[Any] = []
        for note in notes:
            code = _risk_note_code(note)
            if code == "MODEL_DEGRADED_ENTRY_TONE":
                cleaned_notes.append("模型健康度降級，Final Entry 需保守觀察。")
            elif code == "RR_BELOW_FULL_ENTRY":
                cleaned_notes.append("風險報酬比未達完整買進門檻，Final Entry 需保守觀察。")
            else:
                cleaned_notes.append(note)
        notes = cleaned_notes
    price_basis = str((entry_executability or {}).get("price_basis") or "")
    stop_distance_pct = (rr_context or {}).get("stop_distance_pct")
    stop_note = ""
    if price_basis in MARKET_PRICE_ENTRY_BASES and stop_distance_pct is not None:
        stop_note = f"市價進場距停損約 {float(stop_distance_pct) * 100:.1f}%，需確認部位風險可承受。"
    if state == "WAIT_CONFIRMATION":
        if entry_executability and not entry_executability.get("executable_now"):
            reason = str(entry_executability.get("reason_code") or "ENTRY_NOT_EXECUTABLE")
            if reason == "ENTRY_ZONE_OVERSHOT":
                notes.append("Final Entry 尚未放行：現價已高於可執行進場區，不追價。")
            elif reason == "ENTRY_ZONE_UNDERSHOT":
                notes.append("Final Entry 尚未放行：現價已低於可執行進場區，需等待重新站回。")
            else:
                notes.append("Final Entry 尚未放行：目前進場價位不可執行。")
        elif entry_blocking_zone and entry_blocking_zone.get("blocked"):
            notes.append("Final Entry 尚未放行：前方壓力過近，先等待突破或回測。")
        elif "EXECUTION_RR_UNAVAILABLE" in reason_codes:
            notes.append(f"Final Entry 尚未放行：市價進場尚無可量化目標價，需等待前方壓力或回測區明確。{stop_note}".strip())
        elif "EXECUTION_RR_INSUFFICIENT" in reason_codes:
            notes.append(f"Final Entry 尚未放行：以市價、停損與前方目標重算後 RR 不足。{stop_note}".strip())
        elif "MODEL_ENTRY_BLOCKED" in reason_codes or "MODEL_ENTRY_CAPPED" in reason_codes:
            notes.append("Final Entry 已依模型健康度降級，先以觀察為主。")
        else:
            notes.append("Final Entry 尚未放行，對外操作維持觀察等待。")
    elif state == "BLOCKED":
        notes.append("Final Entry 禁止進場，需等待阻擋條件解除後重新評估。")
    elif state == "PROBE_ALLOWED":
        notes.append("Final Entry 僅允許觀察性試探，不代表完整部位進場。")
    elif state == "ENTRY_ALLOWED" and price_basis in MARKET_PRICE_ENTRY_BASES:
        target_known = bool((rr_context or {}).get("target_known"))
        if target_known:
            notes.append(f"Final Entry 已放行市價型進場，需以 entry_price、stop_price 與 target_price 控制風險。{stop_note}".strip())
        else:
            notes.append(f"Final Entry 已放行市價型進場；目前上方無明確壓力目標，target 尚未量化。{stop_note}".strip())
    return _unique_reason_codes([_risk_note_text(note) for note in notes])


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
        "BLOCKED": "禁止進場",
        "WAIT_CONFIRMATION": "等待確認",
        "PROBE_ENTRY": "觀察性試探",
        "PROBE_ALLOWED": "允許觀察性試探",
        "SMALL_ENTRY": "小量進場",
        "ACCUMULATE": "分批累積",
        "BUY": "買進",
        "ENTRY_ALLOWED": "允許進場",
    }.get(state, state)


def _normalize_final_entry_state(state: str) -> str:
    if state in ("INVALIDATED", "BLOCKED"):
        return "BLOCKED"
    if state in ("PROBE_ENTRY", "PROBE_ALLOWED", "SMALL_ENTRY"):
        return "PROBE_ALLOWED"
    if state in ("ENTRY_ALLOWED", "BUY", "BUY_READY", "ENTRY_READY", "ACCUMULATE"):
        return "ENTRY_ALLOWED"
    return "WAIT_CONFIRMATION"


def _entry_state_rank(state: str) -> int:
    return {
        "INVALIDATED": 0,
        "BLOCKED": 0,
        "WAIT_CONFIRMATION": 1,
        "NO_SETUP": 1,
        "WAIT_DAILY_CONFIRM": 1,
        "CHASING_RISK": 1,
        "PROBE_ENTRY": 2,
        "PROBE_ALLOWED": 2,
        "SMALL_ENTRY": 3,
        "ACCUMULATE": 4,
        "ENTRY_READY": 4,
        "ENTRY_ALLOWED": 5,
        "BUY": 5,
        "BUY_READY": 5,
    }.get(state, 1)


def _entry_cap_state(state: str, max_state: str) -> str:
    normalized_state = _normalize_final_entry_state(state)
    cap_rank = _entry_state_rank(max_state)
    if _entry_state_rank(normalized_state) <= cap_rank:
        return normalized_state
    if cap_rank <= 1:
        return "WAIT_CONFIRMATION"
    if cap_rank <= 4:
        return "PROBE_ALLOWED"
    return normalized_state


def _entry_zone_tolerance(primary_zone: Optional[ZoneScore], current_price: float) -> Optional[float]:
    if primary_zone is None:
        return None
    width = max(primary_zone.price_high - primary_zone.price_low, 0.0)
    return max(abs(current_price) * 0.002, width * 0.1)


def _is_market_price_entry(entry_executability: Optional[dict[str, Any]]) -> bool:
    return str((entry_executability or {}).get("price_basis") or "") in MARKET_PRICE_ENTRY_BASES


def _entry_executability(
    primary_zone: Optional[ZoneScore],
    current_price: float,
    derived_view: Optional[dict[str, Any]] = None,
) -> dict[str, Any]:
    if primary_zone is None:
        return {
            "entry_price": None,
            "entry_zone_lower": None,
            "entry_zone_upper": None,
            "tolerance": None,
            "executable_now": False,
            "reason_code": "NO_ENTRY_ZONE",
            "price_basis": "UNAVAILABLE",
        }
    if primary_zone.role != ZoneType.SUPPORT.value:
        return {
            "entry_price": None,
            "entry_zone_lower": primary_zone.price_low,
            "entry_zone_upper": primary_zone.price_high,
            "tolerance": _entry_zone_tolerance(primary_zone, current_price),
            "executable_now": False,
            "reason_code": "ENTRY_ZONE_NOT_SUPPORT",
            "price_basis": "UNSUPPORTED_ENTRY_ZONE",
        }
    tolerance = float(_entry_zone_tolerance(primary_zone, current_price) or 0.0)
    lower = float(primary_zone.price_low)
    upper = float(primary_zone.price_high)
    semantic_pipeline = (derived_view or {}).get("semantic_pipeline") or {}
    lifecycle_phase = str(semantic_pipeline.get("lifecycle_phase") or "NORMAL")
    event_signal = str(semantic_pipeline.get("event_signal") or "NO_EVENT")
    entry_permission_state = str(semantic_pipeline.get("entry_permission_state") or "WAIT_CONFIRMATION")

    if current_price < lower - tolerance:
        return {
            "entry_price": upper,
            "entry_zone_lower": lower,
            "entry_zone_upper": upper,
            "tolerance": tolerance,
            "executable_now": False,
            "reason_code": "ENTRY_ZONE_UNDERSHOT",
            "price_basis": "PRIMARY_SUPPORT_UPPER",
        }

    reclaim_or_continuation = (
        event_signal == "CLOSE_RECLAIM"
        and lifecycle_phase in ("TESTING", "CONFIRMED", "CONTINUATION")
        and entry_permission_state in ("PROBE_ALLOWED", "ENTRY_ALLOWED")
    )
    if reclaim_or_continuation:
        price_basis = "CONTINUATION_MARKET_PRICE" if lifecycle_phase == "CONTINUATION" else "RECLAIM_CLOSE"
        return {
            "entry_price": float(current_price),
            "entry_zone_lower": lower,
            "entry_zone_upper": upper,
            "tolerance": tolerance,
            "executable_now": True,
            "reason_code": "EXECUTABLE_NOW",
            "price_basis": price_basis,
        }

    executable_now = current_price <= upper + tolerance
    return {
        "entry_price": upper,
        "entry_zone_lower": lower,
        "entry_zone_upper": upper,
        "tolerance": tolerance,
        "executable_now": executable_now,
        "reason_code": "EXECUTABLE_NOW" if executable_now else "ENTRY_ZONE_OVERSHOT",
        "price_basis": "PRIMARY_SUPPORT_UPPER",
    }


def _unique_reason_codes(codes: list[str]) -> list[str]:
    out: list[str] = []
    for code in codes:
        if code and code not in out:
            out.append(code)
    return out


def _apply_full_entry_gate_cap(
    derived_view: Optional[dict[str, Any]],
    execution_rr_gate: Optional[dict[str, Any]],
) -> None:
    """把 full-entry gate 的封頂**同步**套到 semantic pipeline（T-055，2026-08-27 裁決）。

    `decision_derived_view` 與 `final_entry_permission` **兩個都列在
    `decision_contract.authoritative_fields`**，所以不能只封頂後者——那會讓契約消費者
    在同一份輸出裡讀到 `ENTRY_ALLOWED`（semantic）與 `PROBE_ALLOWED`（final）兩種進場許可。
    「final entry 才是 authoritative」不能拿來解釋這個落差，因為契約沒有這樣分層。

    **為什麼是事後校正，不是讓 gate 直接參與仲裁**：資料流上是一個環——
    execution gate 需要 `entry_executability`（要 entry_price 算 target），
    `entry_executability` 需要 `decision_derived_view`，而 derived view 需要 gate。
    把 gate 前移得先解這個環，屬 T-065 的範圍。

    **就地改是安全的**，唯一的其他內部消費者 `_entry_executability:806` 對
    `PROBE_ALLOWED` 與 `ENTRY_ALLOWED` 走**同一個分支**，所以封頂不會回頭改變它，
    不存在回饋迴圈。（改那一行之前先確認這句仍然成立。）

    ⚠️ **封頂是單調遞減的，只擋高估、擋不了低估。** 反方向
    （`executable_rr` 高於 `setup_rr`）的矛盾出在更上游的提前 return，這裡碰不到——
    見 `docs/sr-zone-scoring.md`「RR 語意分層／已知的單向性」與 `docs/todo.md` T-065。
    """
    if not derived_view or not execution_rr_gate:
        return
    secondary = execution_rr_gate.get("secondary_gate") or {}
    if secondary.get("qualified") is not False:
        return
    semantic = derived_view.get("semantic_pipeline") or {}
    state = str(semantic.get("entry_permission_state") or "")
    if not state:
        return
    capped = _entry_cap_state(state, "PROBE_ALLOWED")
    if capped == state:
        return
    semantic["entry_permission_state"] = capped
    semantic["reason_codes"] = _unique_reason_codes([
        *list(semantic.get("reason_codes") or []),
        "RR_BELOW_FULL_ENTRY",
    ])


def _final_entry_permission(
    entry_action_state: str,
    daily_confirmation: dict[str, Any],
    derived_view: Optional[dict[str, Any]] = None,
    entry_executability: Optional[dict[str, Any]] = None,
    entry_blocking_zone: Optional[dict[str, Any]] = None,
    model_governance: Optional[dict[str, Any]] = None,
    execution_rr_gate: Optional[dict[str, Any]] = None,
) -> dict[str, Any]:
    daily_state = str(daily_confirmation.get("state") or "WAIT_DAILY_CONFIRM")
    semantic_pipeline = (derived_view or {}).get("semantic_pipeline") or {}
    semantic_entry_state = str(semantic_pipeline.get("entry_permission_state") or "")
    if daily_state in ("INVALIDATED", "BLOCKED") or semantic_entry_state == "BLOCKED":
        state = "BLOCKED"
    elif daily_state == "CHASING_RISK":
        state = "WAIT_CONFIRMATION"
    elif semantic_entry_state:
        state = semantic_entry_state
    else:
        entry_rank = _entry_state_rank(entry_action_state)
        daily_rank = _entry_state_rank(daily_state)
        final_rank = min(entry_rank, daily_rank)
        if final_rank >= 5:
            state = "ENTRY_ALLOWED"
        elif final_rank == 4:
            state = "ENTRY_ALLOWED"
        elif final_rank in (2, 3):
            state = "PROBE_ALLOWED"
        elif final_rank == 0:
            state = "BLOCKED"
        else:
            state = "WAIT_CONFIRMATION"
    state = _normalize_final_entry_state(state)
    reason_codes = list(daily_confirmation.get("reason_codes") or [])
    # 硬阻擋時不回填「等待延續 / 動能確認」這類 derived reason，避免
    # final entry 同時輸出 blocked 與 probe/wait 文案。
    if state != "BLOCKED":
        reason_codes = _unique_reason_codes([
            *reason_codes,
            *list(semantic_pipeline.get("reason_codes") or []),
            *list((derived_view or {}).get("final_entry_reason_codes") or []),
        ])
    if state != "BLOCKED" and entry_executability and not entry_executability.get("executable_now"):
        state = "WAIT_CONFIRMATION"
        reason_codes = _unique_reason_codes([
            *reason_codes,
            str(entry_executability.get("reason_code") or "ENTRY_NOT_EXECUTABLE"),
        ])
    if state != "BLOCKED" and entry_blocking_zone and entry_blocking_zone.get("blocked"):
        state = "WAIT_CONFIRMATION"
        reason_codes = _unique_reason_codes([
            *reason_codes,
            str(entry_blocking_zone.get("reason_code") or "NEAR_RESISTANCE_BLOCKING_ENTRY"),
        ])
    if state != "BLOCKED" and execution_rr_gate and not execution_rr_gate.get("qualified"):
        state = "WAIT_CONFIRMATION"
        reason_codes = _unique_reason_codes([
            *reason_codes,
            str(execution_rr_gate.get("reason_code") or "EXECUTION_RR_UNAVAILABLE"),
        ])
    # ── T-055：完整部位 gate 也要能限制 authoritative 欄位 ────────────────────
    #
    # 上面那段只讀主 gate（probe 門檻）。probe 過了但 `secondary_gate` 沒過時，
    # `state` 會停在 `ENTRY_ALLOWED`，而 `final_entry_permission` **才是契約宣告的
    # authoritative 欄位**——於是畫面同時出現「正式進場已放行」與「完整部位未通過」。
    # 只在 `_final_action_from_entry` 把 `action` 降成 `BuySmall` 不夠：那是 deprecated
    # 欄位，改它不會改變 authoritative 的宣告。
    #
    # 封頂而不是擋掉：secondary 沒過的語意是「試單可以、完整部位不行」，
    # 正好就是 `PROBE_ALLOWED`。用 `_entry_cap_state` 所以不會把已經更低的狀態拉高。
    if state != "BLOCKED" and execution_rr_gate:
        secondary_gate = execution_rr_gate.get("secondary_gate") or {}
        if secondary_gate.get("qualified") is False:
            capped_by_secondary = _entry_cap_state(state, "PROBE_ALLOWED")
            if capped_by_secondary != state:
                state = capped_by_secondary
                reason_codes = _unique_reason_codes([
                    *reason_codes,
                    "RR_BELOW_FULL_ENTRY",
                ])
    confidence_gate = (model_governance or {}).get("confidence_gate") or {}
    if state != "BLOCKED" and confidence_gate.get("allow_entry") is False:
        state = "WAIT_CONFIRMATION"
        reason_codes = _unique_reason_codes([
            *reason_codes,
            *list(confidence_gate.get("reason_codes") or []),
            "MODEL_ENTRY_BLOCKED",
        ])
    elif state != "BLOCKED" and confidence_gate.get("max_entry_state"):
        capped_state = _entry_cap_state(state, str(confidence_gate.get("max_entry_state")))
        if capped_state != state:
            state = capped_state
            reason_codes = _unique_reason_codes([
                *reason_codes,
                *list(confidence_gate.get("reason_codes") or []),
                "MODEL_ENTRY_CAPPED",
            ])

    return {
        "state": state,
        "label": _entry_action_label(state),
        "entry_action_state": entry_action_state,
        "daily_confirmation_state": daily_state,
        "reason_codes": reason_codes,
    }


def _position_context_reason_codes(
    primary_zone: Optional[ZoneScore],
    structure_state: str,
    short_term_regime: str,
    active_event_types: set[str],
) -> list[str]:
    if (
        structure_state == "SUPPORT_RECLAIM_CONFIRMED"
        or short_term_regime in ("RECLAIM_ATTEMPT", "RECOVERY")
        or "INTRADAY_RECLAIM" in active_event_types
    ):
        return ["POSITION_RECLAIM_DEFENSE"]
    if primary_zone and primary_zone.role == ZoneType.SUPPORT.value:
        return ["POSITION_SUPPORT_DEFENSE"]
    if primary_zone and primary_zone.role == ZoneType.RESISTANCE.value:
        return ["POSITION_RESISTANCE_OVERHEAD"]
    return []


def _decision_semantic_pipeline(
    regime: dict[str, Any],
    primary_zone: Optional[ZoneScore],
    market_action: str,
    event_state_summary: dict[str, Any],
    daily_price_action: Optional[dict[str, Any]],
    rr_gate: Optional[dict[str, Any]],
    structure_state: str,
    blocking_zone_ahead: bool,
    current_price: float,
) -> dict[str, Any]:
    # 生命週期由獨立的 Lifecycle Engine 判定（見 lifecycle_engine.py 與 todo.md T-044）。
    # **它不吃 rr_gate**：RR 是進場與策略條件，不是事件事實。原本 CONTINUATION 的條件
    # 含 rr_qualified，等於讓策略條件改寫事件事實；移除後保守度改由下方的 entry gate 負責。
    lifecycle = resolve_lifecycle(
        event_state_summary=event_state_summary,
        primary_zone=primary_zone,
        structure_state=structure_state,
        daily_price_action=daily_price_action,
        current_price=current_price,
    )
    event_signal = lifecycle["event_signal"]
    lifecycle_phase = lifecycle["lifecycle_phase"]
    reason_codes: list[str] = list(lifecycle["reason_codes"])

    rr_qualified = bool((rr_gate or {}).get("qualified"))

    if lifecycle_phase in ("BREAKDOWN", "INVALIDATED"):
        market_state = "BREAKDOWN_RISK"
    elif lifecycle_phase == "CONTINUATION":
        market_state = "BULLISH_CONTINUATION"
    elif lifecycle_phase == "TESTING" and event_signal == "SUPPORT_TEST":
        market_state = "REVERSAL_CANDIDATE"
    elif lifecycle_phase in ("TESTING", "CONFIRMED"):
        market_state = "BULLISH_RECOVERY"
    else:
        market_state = str(regime.get("short_term_regime") or event_state_summary.get("market_state") or "NORMAL")

    if market_state == "BREAKDOWN_RISK":
        bias_state = "BEARISH_BIAS"
    elif market_state == "BULLISH_CONTINUATION":
        bias_state = "BULLISH_CONTINUATION"
    elif market_state == "REVERSAL_CANDIDATE":
        bias_state = "REVERSAL_BIAS"
    elif market_state in ("BULLISH_RECOVERY", "RECOVERY", "RECLAIM_ATTEMPT", "EARLY_TREND"):
        bias_state = "BULLISH_BIAS"
    elif regime.get("primary") == "TREND_DOWN" or (primary_zone and primary_zone.role == ZoneType.RESISTANCE.value):
        bias_state = "BEARISH_BIAS"
    elif regime.get("primary") == "TREND_UP" and (primary_zone is None or primary_zone.role == ZoneType.SUPPORT.value):
        bias_state = "BULLISH_BIAS"
    else:
        bias_state = "NEUTRAL_BIAS"

    if market_action == "AVOID":
        bias_state = "BEARISH_BIAS"
        action_state = "AVOID"
        entry_permission_state = "BLOCKED"
        reason_codes.append("MARKET_ACTION_AVOID")
    else:
        if market_state == "BREAKDOWN_RISK":
            action_state = "DEFEND_BREAKDOWN"
        elif lifecycle_phase == "TESTING":
            action_state = "CONDITIONAL_HOLD"
        elif lifecycle_phase in ("CONFIRMED", "CONTINUATION"):
            action_state = "HOLD"
        elif bias_state == "BEARISH_BIAS":
            action_state = "AVOID"
        else:
            action_state = "WATCH"

        if market_state == "BREAKDOWN_RISK":
            entry_permission_state = "BLOCKED"
        elif not rr_qualified:
            entry_permission_state = "BLOCKED"
            reason_codes.append(str((rr_gate or {}).get("reason_code") or "RR_NOT_QUALIFIED"))
        elif blocking_zone_ahead:
            entry_permission_state = "WAIT_CONFIRMATION"
            reason_codes.append("BLOCKING_ZONE_AHEAD")
        elif action_state == "CONDITIONAL_HOLD":
            entry_permission_state = "PROBE_ALLOWED"
        elif action_state == "HOLD" and lifecycle_phase == "CONTINUATION":
            entry_permission_state = "ENTRY_ALLOWED"
        elif action_state == "HOLD" and market_state == "BULLISH_RECOVERY":
            entry_permission_state = "PROBE_ALLOWED"
        else:
            entry_permission_state = "WAIT_CONFIRMATION"

    return {
        "version": "decision-semantic-pipeline-p4",
        "event_signal": event_signal,
        "lifecycle_phase": lifecycle_phase,
        "market_state": market_state,
        "bias_state": bias_state,
        "action_state": action_state,
        "entry_permission_state": entry_permission_state,
        "reason_codes": _unique_reason_codes(reason_codes),
        "source_order": ["Event", "Lifecycle", "Market State", "Bias", "Action", "Entry"],
    }


def _decision_derived_view(
    regime: dict[str, Any],
    primary_zone: Optional[ZoneScore],
    market_action: str,
    entry_action_state: str,
    event_state_summary: dict[str, Any],
    daily_price_action: Optional[dict[str, Any]] = None,
    rr_gate: Optional[dict[str, Any]] = None,
    structure_state: str = "NORMAL",
    daily_candidate_zones: Optional[list[dict[str, Any]]] = None,
    blocking_zone_ahead: bool = False,
    entry_blocking_zone_ahead: Optional[bool] = None,
) -> dict[str, Any]:
    active_states = list(event_state_summary.get("active") or [])
    candidate_states = list(event_state_summary.get("candidates") or [])
    active_bearish_states = list(event_state_summary.get("active_bearish_events") or [])
    active_event_types = _event_state_types(active_states)
    candidate_event_types = _event_state_types(candidate_states)
    short_term_regime = str(regime.get("short_term_regime") or "NORMAL")
    bias_reason_codes: list[str] = []
    semantic_pipeline = _decision_semantic_pipeline(
        regime,
        primary_zone,
        market_action,
        event_state_summary,
        daily_price_action,
        rr_gate,
        structure_state,
        blocking_zone_ahead if entry_blocking_zone_ahead is None else entry_blocking_zone_ahead,
        (daily_price_action or {}).get("reference_prices", {}).get("current_price") or 0.0,
    )

    if market_action == "AVOID":
        bias_state = "BEARISH_BIAS"
        bias_reason_codes.append("MARKET_ACTION_AVOID")
    else:
        bias_state = str(semantic_pipeline.get("bias_state") or "NEUTRAL_BIAS")
        semantic_market_state = str(semantic_pipeline.get("market_state") or "UNKNOWN")
        bias_reason_codes.append(f"SEMANTIC_{semantic_market_state}")

    daily_reason_codes: list[str] = []
    if "REVERSAL_CANDIDATE" in candidate_event_types:
        daily_reason_codes.append("REVERSAL_AWAIT_NEXT_DAILY_CONFIRM")
    if (
        (short_term_regime == "RECLAIM_ATTEMPT" or "INTRADAY_RECLAIM" in active_event_types)
        and daily_price_action
        and daily_price_action.get("price_follow_through_state") == "NO_PRICE_FOLLOW_THROUGH"
    ):
        daily_reason_codes.append("WAIT_PRICE_FOLLOW_THROUGH")
    if (
        entry_action_state in ("PROBE_ENTRY", "SMALL_ENTRY")
        and daily_price_action
        and daily_price_action.get("momentum_confirmation_state") in ("NO_MOMENTUM_CONFIRMATION", "MOMENTUM_UNCONFIRMED")
    ):
        daily_reason_codes.append(str(daily_price_action.get("momentum_confirmation_state")))

    final_entry_reason_codes = _unique_reason_codes([
        *daily_reason_codes,
        *(["MARKET_ACTION_AVOID"] if market_action == "AVOID" else []),
        *(["ENTRY_GATE_BLOCKED"] if entry_action_state == "BLOCKED" else []),
        *(["ENTRY_GATE_WAIT_CONFIRMATION"] if entry_action_state in ("WAIT_CONFIRMATION", "NO_SETUP") else []),
    ])
    if active_bearish_states:
        path_gate_state = "EVENT_RISK"
        path_reason_codes = ["ACTIVE_BEARISH_EVENT"]
        position_reason_codes = ["ACTIVE_BEARISH_EVENT", "POSITION_DEFENSE_REQUIRED"]
    elif structure_state in ("SUPPORT_RECLAIM_INVALIDATED", "BREAKDOWN"):
        path_gate_state = "INVALIDATION_RISK"
        path_reason_codes = ["SUPPORT_BREAKDOWN_RISK"]
        position_reason_codes = ["POSITION_DEFENSE_REQUIRED", "SUPPORT_BREAKDOWN_RISK"]
    elif primary_zone is None and daily_candidate_zones:
        path_gate_state = "DAILY_CANDIDATE_ONLY"
        path_reason_codes = ["DAILY_CANDIDATE_ONLY"]
        position_reason_codes = []
    elif rr_gate is not None and not rr_gate.get("qualified"):
        path_gate_state = "RR_BLOCKED"
        reason = str(rr_gate.get("reason_code") or "RR_NOT_QUALIFIED")
        path_reason_codes = [reason]
        position_reason_codes = _position_context_reason_codes(
            primary_zone,
            structure_state,
            short_term_regime,
            active_event_types,
        )
    elif "WAIT_PRICE_FOLLOW_THROUGH" in daily_reason_codes:
        path_gate_state = "WAIT_PRICE_FOLLOW_THROUGH"
        path_reason_codes = ["WAIT_PRICE_FOLLOW_THROUGH"]
        position_reason_codes = _unique_reason_codes([
            *_position_context_reason_codes(primary_zone, structure_state, short_term_regime, active_event_types),
            "WAIT_PRICE_FOLLOW_THROUGH",
        ])
    elif blocking_zone_ahead:
        path_gate_state = "BLOCKING_ZONE_AHEAD"
        path_reason_codes = ["BLOCKING_ZONE_AHEAD"]
        position_reason_codes = _position_context_reason_codes(
            primary_zone,
            structure_state,
            short_term_regime,
            active_event_types,
        )
    elif (
        structure_state == "SUPPORT_RECLAIM_CONFIRMED"
        or short_term_regime in ("RECLAIM_ATTEMPT", "RECOVERY")
        or "INTRADAY_RECLAIM" in active_event_types
    ):
        path_gate_state = "OPEN_PATH"
        path_reason_codes = []
        position_reason_codes = _position_context_reason_codes(
            primary_zone,
            structure_state,
            short_term_regime,
            active_event_types,
        )
    elif primary_zone and primary_zone.role == ZoneType.SUPPORT.value:
        path_gate_state = "OPEN_PATH"
        path_reason_codes = []
        position_reason_codes = _position_context_reason_codes(
            primary_zone,
            structure_state,
            short_term_regime,
            active_event_types,
        )
    elif primary_zone and primary_zone.role == ZoneType.RESISTANCE.value:
        path_gate_state = "OPEN_PATH"
        path_reason_codes = []
        position_reason_codes = _position_context_reason_codes(
            primary_zone,
            structure_state,
            short_term_regime,
            active_event_types,
        )
    else:
        path_gate_state = "OPEN_PATH"
        path_reason_codes = []
        position_reason_codes = []

    position_gate_state = str(semantic_pipeline.get("action_state") or "WATCH")

    return {
        "version": "decision-derived-view-p2",
        "bias_state": bias_state,
        "bias_label": _market_bias_label(bias_state),
        "bias_reason_codes": bias_reason_codes,
        "active_event_types": sorted(active_event_types),
        "candidate_event_types": sorted(candidate_event_types),
        "final_entry_reason_codes": final_entry_reason_codes,
        "path_gate_state": path_gate_state,
        "path_reason_codes": path_reason_codes,
        "position_gate_state": position_gate_state,
        "position_reason_codes": position_reason_codes,
        "daily_reason_codes": daily_reason_codes,
        "semantic_pipeline": semantic_pipeline,
        "authority_reason_codes": _unique_reason_codes([
            *bias_reason_codes,
            *daily_reason_codes,
            *final_entry_reason_codes,
            *path_reason_codes,
            *position_reason_codes,
        ]),
    }


def _market_bias_label(state: str) -> str:
    return {
        "BULLISH_CONTINUATION": "多頭延續",
        "REVERSAL_BIAS": "反轉觀察",
        "BEARISH_BIAS": "偏空觀察",
        "BULLISH_BIAS": "偏多觀察",
        "NEUTRAL_BIAS": "中性觀察",
    }.get(state, state)


def _market_bias(
    regime: dict[str, Any],
    primary_zone: Optional[ZoneScore],
    market_action: str,
    market_events: Optional[list[dict[str, Any]]] = None,
    derived_view: Optional[dict[str, Any]] = None,
) -> tuple[str, str]:
    if market_action == "AVOID":
        return "BEARISH_BIAS", "偏空觀察"
    if derived_view:
        semantic = derived_view.get("semantic_pipeline") or {}
        state = str(semantic.get("bias_state") or derived_view.get("bias_state") or "NEUTRAL_BIAS")
        return state, _market_bias_label(state)
    event_types = {event.get("type") for event in market_events or []}
    if regime.get("short_term_regime") in ("RECOVERY", "RECLAIM_ATTEMPT", "EARLY_TREND") and market_action != "AVOID":
        return "BULLISH_BIAS", "偏多觀察"
    if "REVERSAL_CANDIDATE" in event_types or "INTRADAY_RECLAIM" in event_types:
        return "REVERSAL_BIAS", "反轉觀察"
    if market_action in ("BUY", "BUY_SMALL"):
        return "BULLISH_BIAS", "偏多觀察"
    if regime.get("primary") == "TREND_DOWN" or (primary_zone and primary_zone.role == ZoneType.RESISTANCE.value):
        return "BEARISH_BIAS", "偏空觀察"
    if regime.get("primary") == "TREND_UP" and (primary_zone is None or primary_zone.role == ZoneType.SUPPORT.value):
        return "BULLISH_BIAS", "偏多觀察"
    return "NEUTRAL_BIAS", "中性觀察"


# 兩層具名門檻（T-055）。**不是把判斷折成一個值**——那會吃掉 Full Entry 語意。
#
#   probe_min_rr（= _minimum_rr）：這次提議的動作能不能放行 → rr_gate.minimum_rr
#   full_entry_min_rr             ：夠不夠格做完整部位   → rr_gate.secondary_gate
#
# 兩層**測同一個 actual_rr，只有門檻不同**——這正是讓「通過」與「未達完整買進門檻」
# 不再互相矛盾的關鍵性質，所以 secondary_gate 不帶自己的 actual_rr。
#
# **本筆不動 1.5 / 1.8 / 2.0 的分級與 `strong` 的 rr >= 2.0**（既有調校結果），
# 只替它們命名並讓對外顯示指明是哪一層。
FULL_ENTRY_MIN_RR = 2.0

GATE_KIND_PROBE = "PROBE"
GATE_KIND_FULL_ENTRY = "FULL_ENTRY"


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


def _execution_target(
    nearest_resistance: Optional[ZoneScore],
    blocking_resistance: Optional[ZoneScore],
    entry_price: Optional[float],
) -> Optional[dict[str, Any]]:
    """Executable RR 的 target：**entry 前方第一道可量化阻力**（T-055 定案）。

    候選是 `_nearest_resistance_above_entry()` 與 `_blocking_resistance_zone()`
    （後者同時是 `entry_blocking_zone.blocking_zone` 與 `blocking_resistance_zone` 的來源，
    三者恆指同一個 zone）。只納入 `price_low > entry_price` 的候選，**取最低的 `price_low`**。

    **為什麼取最低而不是「最相關」**：Executable RR 問的是「entry 到第一道可量化前方阻力
    還有多少空間」，target **不得穿越任何前方壓力**。取較遠的那個會把一段根本走不到的距離
    算進 reward。

    **`blocked=false` 時仍然封頂**：`blocked` 只代表「近到足以擋單」，
    不代表「可以把 target 設在更後面的壓力」——兩件事的門檻不同。

    回傳 None 代表前方沒有可量化阻力；呼叫端要輸出 target unknown，
    **不得用 setup RR 反推 target**（那正是 T-055 要修的成因二）。
    """
    if entry_price is None:
        return None
    candidates: list[tuple[float, str]] = []
    if nearest_resistance is not None and float(nearest_resistance.price_low) > float(entry_price):
        candidates.append((float(nearest_resistance.price_low), "nearest_resistance_above_entry"))
    if blocking_resistance is not None and float(blocking_resistance.price_low) > float(entry_price):
        candidates.append((float(blocking_resistance.price_low), "blocking_resistance_zone"))
    if not candidates:
        return None
    price, source = min(candidates, key=lambda c: c[0])
    return {"price": price, "basis": "FIRST_RESISTANCE_CAP", "source": source}


def _rr_context(
    primary_zone: Optional[ZoneScore],
    position_zone: Optional[ZoneScore] = None,
    entry_executability: Optional[dict[str, Any]] = None,
    defense_lines: Optional[dict[str, Any]] = None,
    target_zone: Optional[ZoneScore] = None,
    execution_target: Optional[dict[str, Any]] = None,
) -> dict[str, Any]:
    entry_rr = primary_zone.risk_reward_ratio if primary_zone else None
    position_rr = position_zone.risk_reward_ratio if position_zone else None
    entry_price = (entry_executability or {}).get("entry_price")
    entry_zone_lower = (entry_executability or {}).get("entry_zone_lower")
    entry_zone_upper = (entry_executability or {}).get("entry_zone_upper")
    price_basis = (entry_executability or {}).get("price_basis") or "UNAVAILABLE"
    market_price_entry = _is_market_price_entry(entry_executability)
    tactical_line = (defense_lines or {}).get("tactical") or {}
    swing_line = (defense_lines or {}).get("swing") or {}
    strategic_line = (defense_lines or {}).get("strategic") or {}
    stop_price = None
    stop_basis = "UNAVAILABLE"
    if primary_zone is not None:
        if tactical_line.get("role") == ZoneType.SUPPORT.value and tactical_line.get("invalidation_price") is not None:
            stop_price = float(tactical_line["invalidation_price"])
            stop_basis = "TACTICAL_STOP"
        elif swing_line.get("role") == ZoneType.SUPPORT.value and swing_line.get("invalidation_price") is not None:
            stop_price = float(swing_line["invalidation_price"])
            stop_basis = "PRIMARY_ZONE_STOP"
    target_price = None
    target_basis = "UNAVAILABLE"
    risk_price = None
    reward_price = None
    execution_rr = None
    execution_rr_source = "UNAVAILABLE"
    # **兩條 entry 路徑用同一套 target**（T-055 定案）。導入前只有市價型會算真 target，
    # 限價型直接 `execution_rr = entry_rr` 並用 setup RR **反推** target——
    # 那讓 Executable RR 變成 Setup RR 的別名，gate 等於在測同一個數字兩次。
    #
    # **沒有可量化 target 時輸出 unknown，不得抄 setup RR**：那是本筆要修的成因二。
    if entry_price is not None and stop_price is not None:
        risk = max(float(entry_price) - float(stop_price), 0.0)
        if risk > 0:
            risk_price = risk
            capped = (execution_target or {}).get("price")
            if capped is not None and float(capped) > float(entry_price):
                target_price = float(capped)
                target_basis = "FIRST_RESISTANCE_CAP"
                reward_price = target_price - float(entry_price)
                execution_rr = reward_price / risk
                execution_rr_source = "ENTRY_STOP_TARGET"
            elif market_price_entry:
                target_basis = "MARKET_ENTRY_TARGET_UNAVAILABLE"
            else:
                target_basis = "TARGET_UNAVAILABLE"
    structural_stop_price = (
        float(strategic_line["invalidation_price"])
        if strategic_line.get("role") == ZoneType.SUPPORT.value and strategic_line.get("invalidation_price") is not None
        else None
    )
    stop_distance_pct = (
        risk_price / max(abs(float(entry_price)), 1e-9)
        if risk_price is not None and entry_price is not None
        else None
    )
    return {
        # setup_rr 是新的正式名稱；entry_rr 保留為 legacy alias（T-055 契約方案 C），
        # 兩者**恆同值**——歷史資料、evaluation 統計與前端都還在消費 entry_rr。
        "setup_rr": float(entry_rr) if entry_rr is not None else None,
        "entry_rr": float(entry_rr) if entry_rr is not None else None,
        "entry_rr_source": (
            "PRIMARY_ZONE_STATISTIC" if market_price_entry and primary_zone and entry_rr is not None
            else "PRIMARY_ZONE" if primary_zone and entry_rr is not None
            else "UNAVAILABLE"
        ),
        # executable_rr 是新的正式名稱；execution_rr 保留為 alias（一版後再評估 deprecate）。
        "executable_rr": float(execution_rr) if execution_rr is not None else None,
        "execution_rr": float(execution_rr) if execution_rr is not None else None,
        "execution_rr_source": execution_rr_source,
        "execution_target": execution_target,
        "position_rr": float(position_rr) if position_rr is not None else None,
        "position_rr_source": "POSITION_ZONE" if position_zone and position_rr is not None else "UNAVAILABLE",
        "entry_price": float(entry_price) if entry_price is not None else None,
        "entry_zone_lower": float(entry_zone_lower) if entry_zone_lower is not None else None,
        "entry_zone_upper": float(entry_zone_upper) if entry_zone_upper is not None else None,
        "stop_price": stop_price,
        "target_price": target_price,
        "price_basis": price_basis,
        "stop_basis": stop_basis,
        "target_basis": target_basis,
        "structural_stop_price": structural_stop_price,
        "risk_price": risk_price,
        "reward_price": reward_price,
        "stop_distance_pct": stop_distance_pct,
        "target_known": target_price is not None,
        "executable_now": bool((entry_executability or {}).get("executable_now")),
        "entry_executability_reason_code": (entry_executability or {}).get("reason_code"),
        "rr_formula_available": entry_price is not None and stop_price is not None and target_price is not None,
    }


def _execution_rr_gate(
    primary_zone: Optional[ZoneScore],
    entry_action_state: str,
    rr_context: dict[str, Any],
    base_rr_gate: dict[str, Any],
) -> dict[str, Any]:
    # **不再只對市價型生效**（T-055）：限價路徑導入前直接回 base_rr_gate，
    # 而那個 gate 測的是 setup RR——於是「限價進場」的 gate 從來沒有測過可執行性。
    minimum = _minimum_rr(primary_zone, entry_action_state)
    zone_actual_rr = base_rr_gate.get("actual_rr")
    execution_rr = rr_context.get("executable_rr")
    market_entry = str(rr_context.get("price_basis") or "") in MARKET_PRICE_ENTRY_BASES

    # `actual` 未知時「不算不合格」的語意**只適用於 target 未知**——
    # 那是「這次算不出來」，不是「沒有東西可算」。
    # `NO_PRIMARY_ZONE` / `RR_UNAVAILABLE` 是後者：**連 zone 統計都不存在**，
    # 把 full-entry gate 標成通過會讓畫面出現「完整部位門檻通過」而底下什麼都沒有。
    UNKNOWN_BUT_NOT_DISQUALIFYING = {"EXECUTION_RR_UNAVAILABLE"}

    def _with_secondary(gate: dict[str, Any], actual: Optional[float]) -> dict[str, Any]:
        # secondary 測的是**同一個 actual**，只換門檻。
        gate["gate_kind"] = GATE_KIND_PROBE
        if actual is None:
            secondary_qualified = str(gate.get("reason_code") or "") in UNKNOWN_BUT_NOT_DISQUALIFYING
        else:
            secondary_qualified = float(actual) >= FULL_ENTRY_MIN_RR
        gate["secondary_gate"] = {
            "gate_kind": GATE_KIND_FULL_ENTRY,
            "minimum_rr": FULL_ENTRY_MIN_RR,
            "qualified": secondary_qualified,
        }
        return gate

    if execution_rr is None and not market_entry:
        # **限價路徑且沒有可量化 target → 沿用 setup-RR 判定**（T-055 實作中發現的契約缺口，
        # 2026-08-27 裁決採方案 A）。
        #
        # 契約原本只寫「target 未知 → qualified=true」，但那條語意**只存在於市價路徑**：
        # 市價型沒有別的判準可用。限價路徑導入前是用 setup RR 判 gate 的，照搬會讓
        # **setup RR 1.49（低於 1.5 門檻）的 zone 從 `qualified=false` 變成 `true`**——
        # 既有守門被靜默放寬，方向與本筆「target 封頂 → RR 下修 → 偏保守」的預期相反。
        #
        # `gate_basis` 因此擴出第三個值 `ZONE_STATISTIC`：它描述的是
        # **「actual_rr 來自 zone 歷史統計，不是 entry/stop/target 算出來的」**，
        # 與另外兩個值同一個軸（actual_rr 的推導方式），沒有破壞 F1 定案的正交性。
        # ⚠️ `gate_basis` 只有這三個值——`TARGET_UNAVAILABLE` 屬於 `rr_context.target_basis`
        # 那一軸（target 的來源），兩者名字像但不互通。
        # **不要讓這個分支直接回傳 base_rr_gate**——那份 dict 沒有 `gate_basis` 與
        # `zone_actual_rr`，會讓「一律輸出」在這裡靜默失效。
        fallback = dict(base_rr_gate)
        fallback["gate_basis"] = "ZONE_STATISTIC"
        fallback["zone_actual_rr"] = zone_actual_rr
        fallback["target_known"] = False
        return _with_secondary(fallback, base_rr_gate.get("actual_rr"))

    if execution_rr is None:
        # 市價路徑：target 未知 ≠ RR 不合格（既有語意，`qualified=True`）。
        return _with_secondary({
            "minimum_rr": minimum,
            "actual_rr": None,
            "qualified": True,
            "reason_code": "EXECUTION_RR_UNAVAILABLE",
            "gate_basis": "MARKET_ENTRY_TARGET_UNAVAILABLE",
            "zone_actual_rr": zone_actual_rr,
            "target_known": False,
        }, None)

    qualified = float(execution_rr) >= minimum
    return _with_secondary({
        "minimum_rr": minimum,
        "actual_rr": float(execution_rr),
        "qualified": qualified,
        "reason_code": "RR_QUALIFIED" if qualified else "EXECUTION_RR_INSUFFICIENT",
        "gate_basis": "ENTRY_STOP_TARGET",
        "zone_actual_rr": zone_actual_rr,
        "target_known": True,
    }, float(execution_rr))


def _nearest_decision_zone(zone_scores: list[ZoneScore], current_price: float) -> Optional[ZoneScore]:
    candidates = [
        z for z in zone_scores
        if z.role != ZoneType.AT_ZONE.value
        and z.recent_validation != RecentValidation.EXPIRED.value
        and z.confidence_level != ConfidenceLevel.LOW.value
    ]
    if not candidates:
        candidates = [z for z in zone_scores if z.role != ZoneType.AT_ZONE.value]
    return min(candidates, key=lambda z: _decision_distance_score(z, current_price), default=None)


def _nearest_zone_by_role(zone_scores: list[ZoneScore], current_price: float, role: str) -> Optional[ZoneScore]:
    candidates = [
        z for z in zone_scores
        if z.role == role
        and z.recent_validation != RecentValidation.EXPIRED.value
        and z.confidence_level != ConfidenceLevel.LOW.value
    ]
    if not candidates:
        candidates = [z for z in zone_scores if z.role == role]
    return min(candidates, key=lambda z: _decision_distance_score(z, current_price), default=None)


def _nearest_resistance_above_entry(zone_scores: list[ZoneScore], entry_price: Optional[float]) -> Optional[ZoneScore]:
    if entry_price is None:
        return None
    strict_candidates = [
        z for z in zone_scores
        if z.role == ZoneType.RESISTANCE.value
        and z.recent_validation != RecentValidation.EXPIRED.value
        and z.confidence_level != ConfidenceLevel.LOW.value
        and float(z.price_low) > float(entry_price)
    ]
    if strict_candidates:
        return min(strict_candidates, key=lambda z: float(z.price_low) - float(entry_price))
    fallback_candidates = [
        z for z in zone_scores
        if z.role == ZoneType.RESISTANCE.value
        and z.recent_validation != RecentValidation.EXPIRED.value
        and float(z.price_low) > float(entry_price)
    ]
    return min(fallback_candidates, key=lambda z: float(z.price_low) - float(entry_price), default=None)


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
    chip_confidence = float(chip_summary.get("confidence") if chip_summary.get("confidence") is not None else (0.0 if chip_missing else 0.75))
    chip_coverage_value = float(chip_summary.get("coverage") if chip_summary.get("coverage") is not None else (0.0 if chip_missing else 1.0))
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
        chip_confidence,
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

    chip_coverage = chip_coverage_value
    price_coverage = 1.0 if price_complete and not any(
        features[key]["status"] == "INVALID" for key in ("daily_price", "previous_daily_price")
    ) else 0.0
    overall = price_coverage * 0.7 + chip_coverage * 0.3
    rr_completeness = 1.0 if risk_reward_ratio is not None and expected_value is not None else 0.0
    trade_qualification_completeness = 1.0 if primary_zone is not None and rr_completeness == 1.0 else 0.0
    return {
        "data_mode": "END_OF_DAY",
        "overall_completeness": round(overall, 4),
        "market_data_completeness": round(overall, 4),
        "rr_completeness": rr_completeness,
        "trade_qualification_completeness": trade_qualification_completeness,
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
            "price_follow_through_state": "UNKNOWN",
            "momentum_confirmation_state": "UNKNOWN",
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
    if follow_through_state == "UPSIDE_FOLLOW_THROUGH":
        price_follow_through_state = "PRICE_UPSIDE_FOLLOW_THROUGH"
    elif follow_through_state == "DOWNSIDE_FOLLOW_THROUGH":
        price_follow_through_state = "PRICE_DOWNSIDE_FOLLOW_THROUGH"
    elif follow_through_state == "UNKNOWN":
        price_follow_through_state = "UNKNOWN"
    else:
        price_follow_through_state = "NO_PRICE_FOLLOW_THROUGH"

    if range_state == "RANGE_EXPANSION" and follow_through_state in ("UPSIDE_FOLLOW_THROUGH", "DOWNSIDE_FOLLOW_THROUGH"):
        momentum_confirmation_state = "MOMENTUM_CONFIRMED"
    elif follow_through_state in ("UPSIDE_FOLLOW_THROUGH", "DOWNSIDE_FOLLOW_THROUGH"):
        momentum_confirmation_state = "MOMENTUM_UNCONFIRMED"
    elif follow_through_state == "UNKNOWN":
        momentum_confirmation_state = "UNKNOWN"
    else:
        momentum_confirmation_state = "NO_MOMENTUM_CONFIRMATION"

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
        "price_follow_through_state": price_follow_through_state,
        "momentum_confirmation_state": momentum_confirmation_state,
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
        "INTRADAY_RECLAIM": "收盤收復",
        "REVERSAL_CANDIDATE": "反轉候選",
    }
    seen: set[str] = set()
    sequence: list[dict[str, Any]] = []
    # **只收決策可見的事件**：這個投影會寫進 stock_sr_decisions.event_sequence_json，
    # 是決策表的既有欄位。階段 D 新增的事件要進 market_events（→ market_event_detections）
    # 與 event_state_summary["states"]，但不能出現在這裡，否則「決策逐欄相同」就破了。
    visible_events = [event for event in market_events if is_decision_visible(event)]
    for event in sorted(visible_events, key=lambda e: order.get(str(e.get("type")), 999)):
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
    zero_width = abs(price_high - price_low) <= max(abs(price_low), 1.0) * 1e-9
    zone_kind = "DAILY_ZONE"
    trigger_price = None
    label = f"{_fmt_price(price_low)} ~ {_fmt_price(price_high)}"
    if zero_width:
        zone_kind = "BREAKOUT_TRIGGER" if role == ZoneType.RESISTANCE.value else "BREAKDOWN_TRIGGER"
        trigger_price = price_low
        label = f"{zone_kind} {_fmt_price(price_low)}"
    return {
        "price_low": price_low,
        "price_high": price_high,
        "label": label,
        "role": role,
        "source": "DAILY_CANDLE",
        "lifecycle": "CANDIDATE",
        "decision_role": "TACTICAL",
        "zone_kind": zone_kind,
        "trigger_price": trigger_price,
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


def _select_blocking_resistance(
    zone_scores: list[ZoneScore],
    daily_candidate_zones: list[dict[str, Any]],
    current_price: float,
) -> Optional[tuple[str, Any]]:
    """挑出目前擋在前方的壓力 zone（歷史 SR 優先，否則日 K 候選壓力）。

    回傳 `("zone", ZoneScore)` / `("daily", dict)` / `None`，讓 `_has_blocking_zone_ahead`
    與 `_price_path` 共用同一份選擇邏輯，避免兩處各自實作而漂移（兩者的 `price_low`
    門檻必須一致，才能保證 `path_state=BLOCKING_ZONE_AHEAD` 與 `blocking_zone` 輸出不分歧）。
    """
    resistance_candidates = [
        z for z in zone_scores
        if z.role == ZoneType.RESISTANCE.value and z.price_high >= current_price
    ]
    if resistance_candidates:
        return ("zone", min(resistance_candidates, key=lambda z: _distance_pct_to_zone(z, current_price)))
    candidate_resistance = next(
        (z for z in daily_candidate_zones if z["role"] == ZoneType.RESISTANCE.value),
        None,
    )
    if candidate_resistance is not None:
        return ("daily", candidate_resistance)
    return None


def _blocking_zone_price_low(blocker: Optional[tuple[str, Any]]) -> Optional[float]:
    if blocker is None:
        return None
    kind, obj = blocker
    return float(obj.price_low) if kind == "zone" else float(obj["price_low"])


def _has_blocking_zone_ahead(
    zone_scores: list[ZoneScore],
    daily_candidate_zones: list[dict[str, Any]],
    current_price: float,
) -> bool:
    price_low = _blocking_zone_price_low(
        _select_blocking_resistance(zone_scores, daily_candidate_zones, current_price)
    )
    return price_low is not None and current_price < price_low


def _blocking_resistance_zone(zone_scores: list[ZoneScore], current_price: float) -> Optional[ZoneScore]:
    """前方第一道擋路壓力：現價之上（或現價還在其中）最近的有效 resistance。

    **它不保證是結構性的**——這裡沒有任何 tier 過濾，第一道擋路壓力完全可能是 Tier-3
    短期壓力。對外顯示一律用「前方擋路壓力」，**不要標成「結構壓力」**，否則 Tier-3 擋路時
    會出現「標著結構壓力的短期壓力」（T-056 的 F2）。「大結構參考」是
    `primary_structural_zone`（Tier-1 品質最高），兩者不保證相同。

    選法**刻意與 `_nearest_zone_by_role` 不同**：這裡用純距離（`_distance_pct_to_zone`），
    不加 `_zone_width_penalty`。擋路與否是物理問題——寬區間一樣擋路，不該因為「寬而模糊」
    就被推到後面；而 `nearest_*_zone` 要的是「最值得參考的價位」，寬度懲罰在那裡才合理。

    `_entry_blocking_zone_detail` 與 `zone_summaries.blocking_resistance_zone` 共用這個
    函式，避免兩處選法漂移。
    """
    candidates = [
        z for z in zone_scores
        if z.role == ZoneType.RESISTANCE.value
        and z.recent_validation != RecentValidation.EXPIRED.value
        and z.price_high >= current_price
    ]
    if not candidates:
        return None
    return min(candidates, key=lambda z: _distance_pct_to_zone(z, current_price))


def _entry_blocking_zone_detail(zone_scores: list[ZoneScore], current_price: float) -> dict[str, Any]:
    zone = _blocking_resistance_zone(zone_scores, current_price)
    if zone is None:
        return {
            "blocked": False,
            "reason_code": None,
            "distance_to_nearest_resistance": None,
            "threshold": None,
            "distance_price": None,
            "threshold_price": None,
            "distance_pct": None,
            "threshold_pct": None,
            "threshold_basis": "ZONE_WIDTH_OR_0_5_PERCENT_PROXY",
            "blocking_zone": None,
        }
    distance_abs = max(float(zone.price_low) - current_price, 0.0)
    width = max(float(zone.price_high) - float(zone.price_low), 0.0)
    threshold_abs = max(width * 0.5, abs(current_price) * 0.005)
    distance_pct = distance_abs / max(abs(current_price), 1e-9)
    threshold_pct = threshold_abs / max(abs(current_price), 1e-9)
    blocked = distance_abs <= threshold_abs
    return {
        "blocked": blocked,
        "reason_code": "NEAR_RESISTANCE_BLOCKING_ENTRY" if blocked else None,
        "distance_to_nearest_resistance": distance_pct,
        "threshold": threshold_pct,
        "distance_price": distance_abs,
        "threshold_price": threshold_abs,
        "distance_pct": distance_pct,
        "threshold_pct": threshold_pct,
        "threshold_basis": "ZONE_WIDTH_OR_0_5_PERCENT_PROXY",
        "blocking_zone": {
            "price_low": zone.price_low,
            "price_high": zone.price_high,
            "label": f"{_fmt_price(zone.price_low)} ~ {_fmt_price(zone.price_high)}",
            "role": zone.role,
            "tier": zone.tier,
            "tier_label": zone.tier_label,
            "source_scope": "ZONE_SCORE_POOL",
            "method": zone.method,
            "confidence": zone.confidence,
        },
    }


def _price_path(
    zone_scores: list[ZoneScore],
    current_price: float,
    primary_zone: Optional[ZoneScore],
    nearest_support_zone: Optional[ZoneScore],
    nearest_resistance_zone: Optional[ZoneScore],
    structural_zone: Optional[ZoneScore],
    daily_candidate_zones: list[dict[str, Any]],
    structure_state: str,
    rr_gate: dict[str, Any],
    event_state_summary: Optional[dict[str, Any]] = None,
    derived_view: Optional[dict[str, Any]] = None,
) -> dict[str, Any]:
    def zone_blocking_ref(zone: ZoneScore) -> dict[str, Any]:
        selected = any(
            zone is selected_zone
            for selected_zone in (primary_zone, nearest_support_zone, nearest_resistance_zone, structural_zone)
            if selected_zone is not None
        )
        return {
            "price_low": zone.price_low,
            "price_high": zone.price_high,
            "label": f"{_fmt_price(zone.price_low)} ~ {_fmt_price(zone.price_high)}",
            "role": zone.role,
            "role_label": _role_label(zone.role),
            "source": "HISTORICAL_SR",
            "source_scope": "ZONE_SCORE_POOL",
            "zone_id": None,
            "method": zone.method,
            "timeframe": None,
            "tier": zone.tier,
            "tier_label": zone.tier_label,
            "display_label": _display_label(zone.tier, zone.role, zone.tier_label),
            "confidence": zone.confidence,
            "confidence_level": zone.confidence_level,
            "distance_pct": _distance_pct_to_zone(zone, current_price),
            "selected_summary_zone": selected,
        }

    def daily_blocking_ref(zone: dict[str, Any]) -> dict[str, Any]:
        return {
            "price_low": zone["price_low"],
            "price_high": zone["price_high"],
            "label": zone["label"],
            "role": zone["role"],
            "role_label": _role_label(zone["role"]),
            "source": zone["source"],
            "source_scope": "DAILY_CANDIDATE",
            "zone_id": None,
            "method": zone.get("method", "daily_candle"),
            "timeframe": "1d",
            "tier": None,
            "tier_label": None,
            "display_label": _display_label(None, zone["role"], "日K"),
            "confidence": None,
            "confidence_level": None,
            "distance_pct": abs(float(zone["price_low"]) - current_price) / max(abs(current_price), 1e-9),
            "selected_summary_zone": False,
        }

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
    if nearest_support_zone is not None or nearest_resistance_zone is not None:
        candidates: list[tuple[ZoneScore, str]] = []
        if nearest_support_zone is not None:
            candidates.append((nearest_support_zone, "nearest_support_zone"))
        if nearest_resistance_zone is not None:
            candidates.append((nearest_resistance_zone, "nearest_resistance_zone"))
        zone, source = min(candidates, key=lambda item: _decision_distance_score(item[0], current_price))
        if current_price < zone.price_low:
            next_decision_price = zone.price_low
        elif current_price > zone.price_high:
            next_decision_price = zone.price_high
        else:
            next_decision_price = current_price
        next_decision_source = source
    elif daily_candidate_zones:
        candidate = min(daily_candidate_zones, key=lambda z: abs(float(z["price_low"]) - current_price))
        next_decision_price = candidate["price_low"] if current_price < candidate["price_low"] else candidate["price_high"]
        next_decision_source = "daily_candidate_zone"

    # 與 _has_blocking_zone_ahead 共用同一份壓力 zone 選擇，兩處只在「輸出 ref」與
    # 「回傳 bool」上不同，選出的 zone 與 price_low 門檻保證一致。
    blocker = _select_blocking_resistance(zone_scores, daily_candidate_zones, current_price)
    blocking_zone: Optional[dict[str, Any]] = None
    if blocker is not None:
        kind, obj = blocker
        blocking_zone = zone_blocking_ref(obj) if kind == "zone" else daily_blocking_ref(obj)

    active_events = list((event_state_summary or {}).get("active") or [])
    active_bearish_events = list((event_state_summary or {}).get("active_bearish_events") or [])
    derived_path_state = str((derived_view or {}).get("path_gate_state") or "OPEN_PATH")
    derived_path_reasons = list((derived_view or {}).get("path_reason_codes") or [])
    if derived_view is not None:
        path_state = derived_path_state
    elif active_bearish_events:
        path_state = "EVENT_RISK"
    elif structure_state in ("SUPPORT_RECLAIM_INVALIDATED", "BREAKDOWN"):
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
        "event_state": (event_state_summary or {}).get("market_state", "NORMAL"),
        "active_event_types": [str(event.get("type") or event.get("latest_event_type")) for event in active_events],
        "blocked_by_event": active_bearish_events[0] if active_bearish_events else None,
        "reason_codes": (
            ["ACTIVE_BEARISH_EVENT"]
            if active_bearish_events else
            derived_path_reasons if derived_view is not None and derived_path_reasons else
            [str(rr_gate.get("reason_code"))] if path_state == "RR_BLOCKED" and rr_gate.get("reason_code") else []
        ),
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
    derived_view: dict[str, Any],
    entry_action_state: str,
    daily_candidate_zones: list[dict[str, Any]],
    current_price: float,
) -> dict[str, Any]:
    reason_codes: list[str] = []
    derived_daily_reasons = list(derived_view.get("daily_reason_codes") or [])
    if primary_zone is None:
        state = "WAIT_DAILY_CONFIRM"
        reason_codes.append("DAILY_CANDIDATE_ONLY" if daily_candidate_zones else "NO_PRIMARY_ZONE")
    elif primary_interaction and primary_interaction.get("closed_below") and primary_zone.role == ZoneType.SUPPORT.value:
        state = "INVALIDATED"
        reason_codes.append("SUPPORT_CLOSED_BELOW")
    elif not rr_gate.get("qualified"):
        state = "BLOCKED"
        reason_codes.append(str(rr_gate.get("reason_code") or "RR_NOT_QUALIFIED"))
    elif _distance_pct_to_zone(primary_zone, current_price) > 0.08:
        state = "CHASING_RISK"
        reason_codes.append("PRICE_TOO_FAR_FROM_ZONE")
    elif "WAIT_PRICE_FOLLOW_THROUGH" in derived_daily_reasons:
        if entry_action_state in ("PROBE_ENTRY", "SMALL_ENTRY"):
            state = "PROBE_ALLOWED"
        else:
            state = "WAIT_DAILY_CONFIRM"
        reason_codes.append("WAIT_PRICE_FOLLOW_THROUGH")
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
    elif "REVERSAL_AWAIT_NEXT_DAILY_CONFIRM" in derived_daily_reasons:
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

    # derived daily reasons 只在「進場軌道」的結論上補回。硬性阻擋（無主 zone、設定失效、
    # RR 不過、追價風險）與進場條件無關，補這些 code 只會產生「禁止進場又說等待價格延續」
    # 的矛盾標籤——正是 I-002 要消除的訊號不一致。
    entry_track = primary_zone is not None and state not in ("INVALIDATED", "BLOCKED", "CHASING_RISK")
    if entry_track:
        for code in derived_daily_reasons:
            if code not in reason_codes:
                reason_codes.append(code)

    labels = {
        "BLOCKED": "禁止進場",
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
        evidence = interaction.get("price_action_evidence") or {}
        previous_evidence = previous_interaction.get("price_action_evidence") if previous_interaction else {}
        if primary_zone.recent_validation == RecentValidation.EXPIRED.value:
            return "BREAKDOWN"
        if evidence.get("closed_below", interaction["closed_below"]):
            return "SUPPORT_RECLAIM_INVALIDATED"
        if (
            previous_interaction
            and previous_evidence.get("touched", previous_interaction["touched"])
            and previous_evidence.get("reclaim_type") == "UNDERCUT_RECLAIM"
            and not evidence.get("closed_below", interaction["closed_below"])
        ):
            return "SUPPORT_RECLAIM_CONFIRMED"
        if evidence.get("reclaim_type") == "UNDERCUT_RECLAIM":
            return "SUPPORT_RECLAIM_CANDIDATE"
        if evidence.get("touched", interaction["touched"]):
            return "SUPPORT_RECLAIM_CANDIDATE"
    return "NORMAL"


# tactical 防守線的兩個來源。**兩者必須在輸出上分得開**（現況規格見
# docs/sr-zone-scoring.md 的「事件的決策可見性」；原記於 issue.md I-093，已收斂）：
# `_defense_lines` 是四個 shadow 過濾點之一（見 docs/sr-zone-scoring.md
# 「事件的決策可見性」），而「它有沒有跳過 decision_visible=False 的事件」
# 過去無法從輸出判斷——fallback 出來的 zone 也標成 recent_microstructure，
# 於是「過濾成功後退到最近的 TIER_3」與「過濾失效、shadow 事件產生了防守線」
# 在資料上長得一模一樣。2026-08-25 的 live 盤點就是據此誤判成洩漏，
# 翻原始碼才確認是 fallback。
TACTICAL_SOURCE_EVENT = "recent_microstructure"
TACTICAL_SOURCE_FALLBACK = "nearest_tier3"


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
    # 預設是事件路徑；只有真的退到距離 fallback 才改值。**不要改成事後從結果反推**
    # （例如比對 zone 是否等於最近的 TIER_3）——事件比對到的 zone 剛好就是最近的
    # TIER_3 時兩者無法區分，那會把這個欄位變回不可稽核。
    tactical_source = TACTICAL_SOURCE_EVENT
    for event in market_events or []:
        # **只收決策可見的事件**：這個迴圈是**位置型**讀者——取「第一個 zone_ref 對得上
        # 的事件」當戰術防守線，不比對型別名。階段 D 的新事件（SUPPORT_RETEST_HELD
        # 不帶品質門檻，碰到未收破就成立）一旦進 raw market_events 就會插隊換掉
        # tactical zone，而 defense_lines 與其下游的 rr_context.stop_basis /
        # stop_price 都是 stock_sr_decisions 的既有欄位。實測（2026-08-20，四檔 21 階）
        # 未過濾時 84 筆決策裡有 7 筆的 defense_lines.tactical 被換掉。
        if not is_decision_visible(event):
            continue
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
        tactical_source = TACTICAL_SOURCE_FALLBACK
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
        "tactical": line(tactical_zone, tactical_source),
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
    model_governance: Optional[dict[str, Any]] = None,
    previous_event_states: Optional[list[dict[str, Any]]] = None,
    bar_advanced: bool = True,
) -> dict[str, Any]:
    global_confidence = global_metrics.get("confidence")
    market_events = detect_market_events(zone_scores, current_price, candle_high, candle_low, candle_close)
    event_state_summary = build_event_state_summary(
        market_events, previous_states=previous_event_states, bar_advanced=bar_advanced,
    )
    active_market_events = list(event_state_summary.get("active") or [])
    model_governance = model_governance or {
        "health_state": "UNKNOWN",
        "quality_flags": [],
        "warning_flags": [],
        "blocking_flags": [],
        "confidence_gate": {
            "state": "UNKNOWN",
            "allow_entry": True,
            "max_entry_state": "BUY",
            "reason_codes": [],
        },
    }
    trend_regime = _market_regime(
        global_trend,
        global_volatility,
        global_confidence,
        chip_summary,
        market_events=market_events,
        event_state_summary=event_state_summary,
        model_governance=model_governance,
    )["trend_regime"]
    primary_zone = _pick_primary_zone(zone_scores, current_price, trend_regime, active_market_events)
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
        event_state_summary,
        model_governance,
    )
    # Decision gating（primary zone 選擇、action、entry state、bias、event-aware relevance）
    # 一律只吃 active_market_events：已被 reclaim/reversal resolve 的 breakdown 不得再影響決策。
    # 完整 raw chain 僅供對外呈現（market_events / event_sequence / event_state_summary）。
    market_action, position_action, action, action_label, risk_notes = _decision_action(
        regime, primary_zone, current_price, primary_interaction, active_market_events
    )
    entry_action_state = _entry_action_state(action, primary_zone, structure_state, current_price, active_market_events)
    rr_gate = _rr_gate(primary_zone, entry_action_state)
    nearest_zone = _nearest_decision_zone(zone_scores, current_price)
    nearest_support_zone = _nearest_zone_by_role(zone_scores, current_price, ZoneType.SUPPORT.value)
    nearest_resistance_zone = _nearest_zone_by_role(zone_scores, current_price, ZoneType.RESISTANCE.value)
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
    defense_lines = _defense_lines(zone_scores, primary_zone, current_price, market_events)
    entry_blocking_zone = _entry_blocking_zone_detail(zone_scores, current_price)
    # T-056：擋路壓力升格成獨立的 summary 欄位。與 entry_blocking_zone 共用同一個選法
    # （`_blocking_resistance_zone`），所以兩者恆指同一個 zone。
    blocking_resistance_zone = _blocking_resistance_zone(zone_scores, current_price)
    # 戰術壓力的 summary 只組一次，兩個鍵共用同一份（新鍵與 legacy alias 不得漂移）。
    tactical_resistance_summary = _decision_summary_zone(
        nearest_resistance_zone, current_price, "品質加權後最相關的戰術壓力（不是價格最近）",
        candle_high, candle_low, candle_close,
        decision_role="TACTICAL_RESISTANCE",
    ) if nearest_resistance_zone else None
    blocking_zone_ahead = _has_blocking_zone_ahead(zone_scores, daily_candidate_zones, current_price)
    entry_blocking_zone_ahead = bool(entry_blocking_zone.get("blocked"))
    decision_derived_view = _decision_derived_view(
        regime,
        primary_zone,
        market_action,
        entry_action_state,
        event_state_summary,
        daily_price_action,
        rr_gate,
        structure_state,
        daily_candidate_zones,
        blocking_zone_ahead,
        entry_blocking_zone_ahead,
    )
    entry_executability = _entry_executability(primary_zone, current_price, decision_derived_view)
    market_bias, market_bias_label = _market_bias(
        regime,
        primary_zone,
        market_action,
        active_market_events,
        decision_derived_view,
    )
    price_path = _price_path(
        zone_scores,
        current_price,
        primary_zone,
        nearest_support_zone,
        nearest_resistance_zone,
        structural_zone,
        daily_candidate_zones,
        structure_state,
        rr_gate,
        event_state_summary,
        decision_derived_view,
    )
    daily_confirmation = _daily_confirmation(
        primary_zone,
        primary_interaction,
        daily_price_action,
        rr_gate,
        decision_derived_view,
        entry_action_state,
        daily_candidate_zones,
        current_price,
    )
    # **target 由呼叫端算好再傳進去**（T-055 定案）：`_rr_context()` 不自己找 zone。
    # `blocking_resistance_zone` 在 :2474 就已經算完，這裡直接沿用同一個物件——
    # 三個對外欄位（`entry_blocking_zone` / `blocking_resistance_zone` / target 封頂）
    # 因此恆指同一個 zone，不會出現「畫面上擋路壓力是 A、target 卻用 B」。
    _entry_price_for_target = entry_executability.get("entry_price") if entry_executability else None
    execution_target = _execution_target(
        _nearest_resistance_above_entry(zone_scores, _entry_price_for_target),
        blocking_resistance_zone,
        _entry_price_for_target,
    )
    rr_context = _rr_context(
        primary_zone,
        entry_executability=entry_executability,
        defense_lines=defense_lines,
        target_zone=_nearest_resistance_above_entry(zone_scores, _entry_price_for_target),
        execution_target=execution_target,
    )
    rr_gate = _execution_rr_gate(primary_zone, entry_action_state, rr_context, rr_gate)
    # 兩個 authoritative 欄位要一起降，順序不能反——封頂必須在 `_final_entry_permission`
    # 讀 `semantic_entry_state` 之前，否則它會先讀到未封頂的 `ENTRY_ALLOWED`。
    _apply_full_entry_gate_cap(decision_derived_view, rr_gate)
    final_entry_permission = _final_entry_permission(
        entry_action_state,
        daily_confirmation,
        decision_derived_view,
        entry_executability,
        entry_blocking_zone,
        model_governance,
        rr_gate,
    )
    market_action, position_action, action, action_label = _final_action_from_entry(
        final_entry_permission,
        market_action,
        position_action,
        action,
        action_label,
        rr_gate,
    )
    risk_notes = _final_entry_risk_notes(
        risk_notes,
        final_entry_permission,
        entry_executability,
        entry_blocking_zone,
        rr_context,
        rr_gate,
    )
    final_entry_state = str(final_entry_permission.get("state") or "WAIT_CONFIRMATION")
    best_trade_zone = (
        primary_zone
        if rr_gate["qualified"]
        and final_entry_state in ("PROBE_ALLOWED", "ENTRY_ALLOWED")
        and entry_executability.get("executable_now")
        and not entry_blocking_zone.get("blocked")
        and not _is_market_price_entry(entry_executability)
        else None
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
        "model_governance": model_governance,
        "market_events": market_events,
        "event_state_summary": event_state_summary,
        "event_sequence": event_sequence,
        "decision_derived_view": decision_derived_view,
        "daily_price_action": daily_price_action,
        "daily_candidate_zones": daily_candidate_zones,
        "price_path": price_path,
        "daily_confirmation": daily_confirmation,
        "entry_executability": entry_executability,
        "entry_blocking_zone": entry_blocking_zone,
        "defense_lines": defense_lines,
        "rr_context": rr_context,
        "market_bias": market_bias,
        "market_bias_label": market_bias_label,
        "decision_contract": {
            "version": "sr-zone-decision-p0",
            "authoritative_fields": [
                "decision_derived_view",
                "market_bias",
                "final_entry_permission",
                "position_action",
                "rr_gate",
                "price_path",
            ],
            "deprecated_fields": [
                "market_action",
                "action",
                "action_label",
            ],
        },
        "market_action": market_action,
        "position_action": position_action,
        "position_action_condition": _position_action_condition(primary_zone, structure_state, decision_derived_view),
        "action": action,
        "action_label": action_label,
        "entry_action_state": entry_action_state,
        "entry_action_label": _entry_action_label(entry_action_state),
        "final_entry_permission": final_entry_permission,
        "daily_entry_state": daily_confirmation["state"],
        "daily_entry_label": daily_confirmation["label"],
        "rr_gate": rr_gate,
        "nearest_decision_zone": _decision_summary_zone(
            nearest_zone, current_price, "離現價最近的決策參考區",
            candle_high, candle_low, candle_close,
            decision_role="TACTICAL",
        ) if nearest_zone else None,
        "nearest_support_zone": _decision_summary_zone(
            nearest_support_zone, current_price, "離現價最近的支撐決策參考區",
            candle_high, candle_low, candle_close,
            decision_role="TACTICAL_SUPPORT",
        ) if nearest_support_zone else None,
        # T-056：戰術壓力與擋路壓力是兩層，必須同時輸出。
        # `nearest_resistance_zone` **不是價格最近的壓力**——它經過 `_zone_width_penalty`
        # 加權（見 sr-zone-scoring.md），所以更名為 `tactical_resistance_zone`；
        # 舊鍵保留為 legacy alias，值完全相同。
        "tactical_resistance_zone": tactical_resistance_summary,
        "nearest_resistance_zone": tactical_resistance_summary,  # deprecated alias of tactical_resistance_zone
        "blocking_resistance_zone": _decision_summary_zone(
            blocking_resistance_zone, current_price, "前方第一道擋路壓力",
            candle_high, candle_low, candle_close,
            decision_role="BLOCKING_RESISTANCE",
        ) if blocking_resistance_zone else None,
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


def _bar_advanced_since(analyzed_at: Any, previous_analyzed_at: Optional[str]) -> bool:
    """這次分析站的 K 棒有沒有比上次新（issue.md I-077 的老化單位）。

    **缺值或比不出來一律回 True＝維持舊行為**（照樣 age_bars +1）：沒有 previous states
    時本來就沒有東西要老化，而舊呼叫端（evaluation / replay）不送這個值時，行為必須與
    修改前逐項相同。

    時間**沒有前進反而倒退**（as-of 回放、資料修正）時回 False——保守側，不老化。
    """
    if not previous_analyzed_at or analyzed_at is None:
        return True
    try:
        previous = datetime.fromisoformat(str(previous_analyzed_at).replace("Z", "+00:00"))
        return analyzed_at > previous
    except (TypeError, ValueError):
        return True


def build_decision_from_evidence(
    evidence: AnalysisEvidence,
    previous_event_states: Optional[list[dict[str, Any]]] = None,
    model_governance: Optional[dict[str, Any]] = None,
    previous_analyzed_at: Optional[str] = None,
) -> dict[str, Any]:
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
        model_governance=model_governance or build_model_governance_context(scores),
        previous_event_states=previous_event_states,
        # 推導只發生在這一層——它是唯一同時握有「這次的 analyzed_at」與「上次的」的地方。
        bar_advanced=_bar_advanced_since(
            scores.features.data.analyzed_at, previous_analyzed_at,
        ),
    )
