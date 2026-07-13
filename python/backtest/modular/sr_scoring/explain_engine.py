"""Deterministic plain-language explanations for SR Zone scoring.

The explain engine is a presentation layer over Score/Evidence/Decision. It
does not infer new trading signals and must stay deterministic for the same
input snapshot.
"""
from __future__ import annotations

from typing import Any

from .pipeline_types import AnalysisEvidence
from .types import ConfidenceLevel, RecentValidation, VolumeConfirmation, ZoneScore, ZoneType


def _fmt_price(v: float) -> str:
    return f"{v:.2f}"


def _fmt_pct(v: float | None, digits: int = 1) -> str:
    if v is None:
        return "無資料"
    return f"{v * 100:.{digits}f}%"


def _fmt_signed_pct(v: float | None, digits: int = 2) -> str:
    if v is None:
        return "無資料"
    sign = "+" if v > 0 else ""
    return f"{sign}{v * 100:.{digits}f}%"


def _role_label(role: str) -> str:
    return {
        ZoneType.SUPPORT.value: "支撐",
        ZoneType.RESISTANCE.value: "壓力",
        ZoneType.AT_ZONE.value: "方向未定區",
    }.get(role, role)


def _action_label(action: str) -> str:
    return {
        "Buy": "買進",
        "BuySmall": "小量試單",
        "Hold": "等待",
        "Avoid": "避開",
    }.get(action, action)


def _confidence_label(level: str) -> str:
    return {
        ConfidenceLevel.LOW.value: "低",
        ConfidenceLevel.MEDIUM.value: "中",
        ConfidenceLevel.HIGH.value: "高",
        ConfidenceLevel.VERY_HIGH.value: "極高",
    }.get(level, level)


def _validation_label(value: str) -> str:
    return {
        RecentValidation.VALIDATED_RECENTLY.value: "最近有守住驗證",
        RecentValidation.PENDING_VALIDATION.value: "尚待新觸碰驗證",
        RecentValidation.NOT_TESTED_RECENTLY.value: "近期沒有重新測試",
        RecentValidation.EXPIRED.value: "最近驗證顯示可能失效",
    }.get(value, value)


def _score_breakdown_extremes(score: ZoneScore) -> tuple[dict[str, Any] | None, dict[str, Any] | None]:
    labels = {
        "expected_value": "期望值",
        "risk_reward": "風險報酬比",
        "trend": "趨勢",
        "volume": "量能",
        "confidence": "信心",
        "chip": "籌碼",
    }
    items = [
        {"key": key, "label": labels.get(key, key), "value": float(value)}
        for key, value in sorted(score.trading_score_breakdown.items())
    ]
    if not items:
        return None, None
    top = max(items, key=lambda item: (item["value"], item["key"]))
    bottom = min(items, key=lambda item: (item["value"], item["key"]))
    return top, bottom


def _score_reason(score: ZoneScore) -> str:
    top, bottom = _score_breakdown_extremes(score)
    if top is None or bottom is None:
        return f"Trading Score {score.trading_score:.1f}，目前沒有分量拆解可供排序。"
    return (
        f"Trading Score {score.trading_score:.1f} 主要由{top['label']}貢獻 "
        f"{top['value']:.1f} 分推動；最低分量是{bottom['label']} "
        f"{bottom['value']:.1f} 分。"
    )


def _probability_reason(score: ZoneScore) -> str:
    if score.role == ZoneType.AT_ZONE.value:
        return "現價仍在區間內，尚未解析為支撐或壓力，因此不給出方向性的反彈/跌破結論。"
    role = _role_label(score.role)
    return (
        f"此區間目前按{role}解讀，反彈/守住機率為 {_fmt_pct(score.bounce_probability)}，"
        f"跌破/突破機率為 {_fmt_pct(score.break_probability)}；"
        f"期望值為 {_fmt_signed_pct(score.expected_value)}。"
    )


def _confidence_reason(score: ZoneScore) -> str:
    if score.role == ZoneType.SUPPORT.value:
        role_samples = score.support_touch_count
    elif score.role == ZoneType.RESISTANCE.value:
        role_samples = score.resistance_touch_count
    else:
        role_samples = score.touch_count
    return (
        f"信心為 {_fmt_pct(score.confidence, 0)}（{_confidence_label(score.confidence_level)}），"
        f"主要參考目前角色方向樣本 {role_samples} 次、整體觸碰 {score.touch_count} 次、"
        f"守住 {score.reject_count or 0} 次、跌破/突破 {score.break_count or 0} 次；"
        f"近期性為「{_validation_label(score.recent_validation)}」。"
    )


def _positive_factors(score: ZoneScore) -> list[str]:
    factors: list[str] = []
    if score.confidence_level in (ConfidenceLevel.HIGH.value, ConfidenceLevel.VERY_HIGH.value):
        factors.append(f"信心等級{_confidence_label(score.confidence_level)}")
    if score.recent_validation == RecentValidation.VALIDATED_RECENTLY.value:
        factors.append("最近有有效驗證")
    if score.volume_confirmation == VolumeConfirmation.CONFIRMED.value:
        factors.append("量能確認有效")
    if score.confluence_count > 1:
        factors.append(f"多方法共振 ×{score.confluence_count}")
    if score.expected_value is not None and score.expected_value > 0:
        factors.append(f"期望值為正（{_fmt_signed_pct(score.expected_value)}）")
    if score.risk_reward_ratio is not None and score.risk_reward_ratio >= 1.5:
        factors.append(f"風險報酬比 {score.risk_reward_ratio:.2f}R")
    if not factors:
        factors.append("目前沒有明顯加分因素")
    return factors


def _negative_factors(score: ZoneScore, risk_flags: list[str]) -> list[str]:
    factors: list[str] = []
    if "LOW_CONFIDENCE" in risk_flags or score.confidence_level == ConfidenceLevel.LOW.value:
        factors.append("信心偏低，樣本或穩定度不足")
    if "EXPIRED" in risk_flags or score.recent_validation == RecentValidation.EXPIRED.value:
        factors.append("最近驗證顯示此區可能已失效")
    if "NO_DIRECTION" in risk_flags or score.role == ZoneType.AT_ZONE.value:
        factors.append("現價在區間內，方向尚未解析")
    if score.volume_confirmation in (VolumeConfirmation.WEAK.value, VolumeConfirmation.FAILED.value):
        factors.append("量能沒有確認，或量能確認了失敗")
    if score.expected_value is not None and score.expected_value < 0:
        factors.append(f"期望值為負（{_fmt_signed_pct(score.expected_value)}）")
    if score.risk_reward_ratio is not None and score.risk_reward_ratio < 1.0:
        factors.append(f"風險報酬比不足（{score.risk_reward_ratio:.2f}R）")
    if not factors:
        factors.append("目前沒有明顯扣分因素")
    return factors


def _watch_conditions(score: ZoneScore) -> list[str]:
    low = _fmt_price(score.price_low)
    high = _fmt_price(score.price_high)
    if score.role == ZoneType.SUPPORT.value:
        return [
            f"觀察價格回測 {low} ~ {high} 時是否止跌",
            f"若收盤跌破 {low}，支撐判斷失效風險升高",
            "回測時量能若放大但無法守住，需降低信心",
        ]
    if score.role == ZoneType.RESISTANCE.value:
        return [
            f"觀察價格接近 {low} ~ {high} 時是否受壓",
            f"若收盤突破 {high}，壓力判斷失效風險升高",
            "突破時若量能同步放大，需改以突破情境解讀",
        ]
    return [
        f"現價在 {low} ~ {high} 內，先等待收盤離開區間",
        f"向上離開 {high} 後再觀察是否轉為支撐回測",
        f"向下離開 {low} 後再觀察是否轉為壓力反彈",
    ]


def _role_summary(score: ZoneScore) -> str:
    label = f"{_fmt_price(score.price_low)} ~ {_fmt_price(score.price_high)}"
    if score.role == ZoneType.SUPPORT.value:
        return f"{label} 位於現價下方或回測區，暫以支撐解讀。"
    if score.role == ZoneType.RESISTANCE.value:
        return f"{label} 位於現價上方或反彈區，暫以壓力解讀。"
    return f"{label} 包含現價，方向尚未解析，不能直接視為支撐或壓力。"


def _advanced_refs(zone_evidence: dict[str, Any], score: ZoneScore) -> dict[str, Any]:
    active = None
    if score.role == ZoneType.RESISTANCE.value:
        active = zone_evidence.get("resistance")
    elif score.role == ZoneType.SUPPORT.value:
        active = zone_evidence.get("support")
    top_contributions = []
    if isinstance(active, dict):
        hold = ((active.get("targets") or {}).get("hold") or {})
        top_contributions = (hold.get("contributions") or [])[:3]
    return {
        "score_breakdown_keys": list(score.trading_score_breakdown.keys()),
        "risk_flags": zone_evidence.get("risk_flags") or [],
        "shap_top_contributions": top_contributions,
    }


def explain_zone(score: ZoneScore, zone_evidence: dict[str, Any]) -> dict[str, Any]:
    risk_flags = list(zone_evidence.get("risk_flags") or [])
    return {
        "role_summary": _role_summary(score),
        "score_reason": _score_reason(score),
        "probability_reason": _probability_reason(score),
        "confidence_reason": _confidence_reason(score),
        "positive_factors": _positive_factors(score),
        "negative_factors": _negative_factors(score, risk_flags),
        "watch_conditions": _watch_conditions(score),
        "advanced_refs": _advanced_refs(zone_evidence, score),
    }


def _market_drivers(evidence: AnalysisEvidence) -> list[str]:
    scores = evidence.scores
    drivers = [
        f"整體趨勢 {_fmt_signed_pct(scores.features.global_trend, 1)}",
        f"整體波動 {_fmt_pct(scores.features.global_volatility, 1)}",
    ]
    confidence = scores.global_metrics.get("confidence")
    if confidence is not None:
        drivers.append(f"整體信心 {_fmt_pct(float(confidence), 0)}")
    chip = scores.chip_summary
    if chip.get("missing"):
        drivers.append("籌碼資料缺漏，籌碼面不加強結論")
    elif chip.get("score") is not None:
        drivers.append(f"籌碼總分 {float(chip['score']):.1f}")
    return drivers


def _risk_notes(evidence: AnalysisEvidence, decision_summary: dict[str, Any]) -> list[str]:
    notes = list(decision_summary.get("risk_notes") or [])
    if evidence.scores.features.global_volatility >= 0.035 and not any("波動" in note for note in notes):
        notes.append("整體波動偏高，區間失效速度可能加快。")
    if evidence.scores.global_metrics.get("confidence") is not None:
        confidence = float(evidence.scores.global_metrics["confidence"])
        if confidence < 0.45 and not any("信心" in note for note in notes):
            notes.append("整體信心偏低，應等待更多價格驗證。")
    if any(score.recent_validation == RecentValidation.EXPIRED.value for score in evidence.scores.zones):
        notes.append("部分區間最近驗證已失效，不宜只看歷史分數。")
    if not notes:
        notes.append("目前沒有額外系統性風險提醒，仍需以停損與倉位控管為準。")
    return notes


def build_explanation(evidence: AnalysisEvidence, decision_summary: dict[str, Any]) -> dict[str, Any]:
    scores = evidence.scores
    action = decision_summary.get("action", "Hold")
    action_label = decision_summary.get("action_label") or _action_label(action)
    primary_zone = decision_summary.get("primary_zone")
    if primary_zone:
        action_reason = (
            f"Action 為「{action_label}」，主因是主交易區 {primary_zone.get('label')} "
            f"目前被判定為{_role_label(primary_zone.get('role', ''))}，"
            f"交易分數 {float(primary_zone.get('trading_score', 0)):.1f}。"
        )
    else:
        action_reason = f"Action 為「{action_label}」，因為目前沒有足夠明確的主交易區。"

    model = scores.features.data.model
    uses_shap = bool((evidence.global_evidence.get("model") or {}).get("explainer"))
    return {
        "summary": f"{scores.features.data.symbol} 目前建議以「{action_label}」解讀 SR Zone 結果。",
        "action_reason": action_reason,
        "market_drivers": _market_drivers(evidence),
        "risk_notes": _risk_notes(evidence, decision_summary),
        "model_context": {
            "version": model.version,
            "config_hash": model.config_hash,
            "uses_shap_evidence": uses_shap,
        },
    }
