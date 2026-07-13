"""Deterministic scenario contract for SR Zone analysis.

Scenario is the structured "what is happening / what confirms it / what
invalidates it" layer. It does not change probabilities, scores, or decisions.
"""
from __future__ import annotations

from typing import Any

from .pipeline_types import AnalysisEvidence
from .types import RecentValidation, ZoneScore, ZoneType


SCENARIO_SCHEMA_VERSION = "sr_scenario_v1"


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
        ZoneType.AT_ZONE.value: "方向未定",
    }.get(role, role)


def _zone_state(score: ZoneScore, status: str = "PENDING") -> str:
    if status == "BROKEN":
        return "BROKEN"
    if score.role == ZoneType.AT_ZONE.value:
        return "WAIT_FOR_DIRECTION"
    if score.recent_validation == RecentValidation.EXPIRED.value:
        return "RETEST_REQUIRED"
    if score.role == ZoneType.SUPPORT.value:
        return "SUPPORT_RETEST"
    if score.role == ZoneType.RESISTANCE.value:
        return "RESISTANCE_REJECTION"
    return "UNKNOWN"


def build_zone_scenario(score: ZoneScore, status: str = "PENDING") -> dict[str, Any]:
    low = _fmt_price(score.price_low)
    high = _fmt_price(score.price_high)
    state = _zone_state(score, status)

    if state == "BROKEN":
        return {
            "schema_version": SCENARIO_SCHEMA_VERSION,
            "state": state,
            "title": "區間已失效",
            "summary": f"{low} ~ {high} 已被後續價格突破或跌破，這筆舊情境只能作為歷史參考。",
            "trigger_conditions": ["等待重新分析產生新的支撐/壓力區間"],
            "invalidation_conditions": ["此情境已失效，不再作為進場或風險依據"],
        }

    if score.role == ZoneType.AT_ZONE.value:
        return {
            "schema_version": SCENARIO_SCHEMA_VERSION,
            "state": state,
            "title": "等待方向解析",
            "summary": f"現價仍在 {low} ~ {high} 內，尚未能判斷這個區間會成為支撐或壓力。",
            "trigger_conditions": [
                f"收盤站上 {high} 後，觀察是否轉為支撐回測",
                f"收盤跌破 {low} 後，觀察是否轉為壓力反彈",
            ],
            "invalidation_conditions": ["價格持續在區間內震盪時，暫不給方向性情境"],
        }

    role = _role_label(score.role)
    probability = _fmt_pct(score.bounce_probability)
    break_probability = _fmt_pct(score.break_probability)
    ev = _fmt_signed_pct(score.expected_value)
    if score.role == ZoneType.SUPPORT.value:
        title = "支撐回測情境"
        trigger_conditions = [
            f"價格回測 {low} ~ {high} 並收盤守住",
            f"反彈/守住機率維持在 {probability} 附近，且期望值不轉負",
        ]
        invalidation_conditions = [
            f"收盤跌破 {low}",
            "跌破時量能放大或近期驗證轉為失效",
        ]
    else:
        title = "壓力受壓情境"
        trigger_conditions = [
            f"價格反彈接近 {low} ~ {high} 並出現受壓",
            f"突破壓力機率維持在 {break_probability} 附近，且期望值不轉負",
        ]
        invalidation_conditions = [
            f"收盤突破 {high}",
            "突破時量能放大或近期驗證轉為失效",
        ]

    if state == "RETEST_REQUIRED":
        trigger_conditions.insert(0, "近期驗證偏失效，需要新的價格測試重新確認")

    return {
        "schema_version": SCENARIO_SCHEMA_VERSION,
        "state": state,
        "title": title,
        "summary": f"此區間目前按{role}解讀，守住機率 {probability}、跌破/突破機率 {break_probability}，期望值 {ev}。",
        "trigger_conditions": trigger_conditions,
        "invalidation_conditions": invalidation_conditions,
    }


def build_analysis_scenario(evidence: AnalysisEvidence, decision_summary: dict[str, Any]) -> dict[str, Any]:
    action_label = decision_summary.get("action_label") or decision_summary.get("action") or "等待"
    regime = decision_summary.get("market_regime") or {}
    regime_label = regime.get("label") or regime.get("primary") or "未分級盤勢"
    primary = decision_summary.get("primary_zone")
    risk_notes = list(decision_summary.get("risk_notes") or [])

    if primary:
        zone_label = primary.get("label") or f"{_fmt_price(float(primary['price_low']))} ~ {_fmt_price(float(primary['price_high']))}"
        title = f"{action_label}情境"
        summary = f"目前盤勢為{regime_label}，主要參考區間是 {zone_label}。"
        trigger_conditions = [
            f"價格接近主要區間 {zone_label} 時，優先觀察該區間的支撐/壓力反應",
            "整體信心與主要區間的期望值維持不惡化",
        ]
        invalidation_conditions = risk_notes[:3] or ["主要區間失效或整體風險升高時，情境需重新評估"]
    else:
        title = "等待確認情境"
        summary = f"目前盤勢為{regime_label}，尚未形成足夠明確的主交易區。"
        trigger_conditions = ["等待價格接近明確支撐/壓力區，或重新分析產生主交易區"]
        invalidation_conditions = risk_notes[:3] or ["整體信心下降或波動升高時，維持觀望"]

    # market_regime / primary_zone 已完整存在於同筆分析的 decision_summary，
    # 不在 scenario 內重複內嵌（前端 scenario 區塊只讀 title/summary/state/
    # trigger_conditions/invalidation_conditions，需要 regime/zone 明細時讀 decision）。
    return {
        "schema_version": SCENARIO_SCHEMA_VERSION,
        "state": decision_summary.get("action") or "Hold",
        "title": title,
        "summary": summary,
        "trigger_conditions": trigger_conditions,
        "invalidation_conditions": invalidation_conditions,
        "global_confidence": evidence.scores.global_metrics.get("confidence"),
    }
