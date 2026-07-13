"""Structured probability interpretation for SR Zone scoring.

This layer exposes probability semantics and quality flags without changing
the model output, score derivation, EV/RR, or decision thresholds.
"""
from __future__ import annotations

from typing import Any

from .pipeline_types import AnalysisScores
from .types import ConfidenceLevel, ZoneScore, ZoneType


PROBABILITY_CONTEXT_SCHEMA_VERSION = "sr_probability_context_v1"


def _neutral_probability(bounce_probability: float | None, break_probability: float | None) -> float | None:
    if bounce_probability is None or break_probability is None:
        return None
    return max(0.0, 1.0 - bounce_probability - break_probability)


def _dominant_outcome(bounce_probability: float | None, break_probability: float | None) -> str:
    neutral = _neutral_probability(bounce_probability, break_probability)
    if bounce_probability is None or break_probability is None or neutral is None:
        return "NO_DIRECTION"
    outcomes = {
        "BOUNCE": bounce_probability,
        "BREAK": break_probability,
        "NEUTRAL": neutral,
    }
    return max(outcomes.items(), key=lambda item: (item[1], item[0]))[0]


def _edge_pp(bounce_probability: float | None, break_probability: float | None) -> float | None:
    if bounce_probability is None or break_probability is None:
        return None
    return abs(bounce_probability - break_probability) * 100.0


def _metric(metrics: dict[str, Any], model: str, key: str) -> float | None:
    value = metrics.get(model, {}).get(key)
    if value is None:
        return None
    try:
        return float(value)
    except (TypeError, ValueError):
        return None


def model_quality_flags(scores: AnalysisScores) -> list[str]:
    metrics = scores.features.data.model.metrics or {}
    flags: list[str] = []
    for model in ("hold", "break"):
        if _metric(metrics, model, "calibrated") == 0.0:
            flags.append(f"{model.upper()}_NOT_CALIBRATED")
        test_rows = _metric(metrics, model, "test_rows")
        if test_rows is not None and test_rows < 20:
            flags.append(f"{model.upper()}_LOW_TEST_ROWS")
    return flags


def build_zone_probability_context(score: ZoneScore, model_flags: list[str] | None = None) -> dict[str, Any]:
    neutral = _neutral_probability(score.bounce_probability, score.break_probability)
    dominant = _dominant_outcome(score.bounce_probability, score.break_probability)
    edge = _edge_pp(score.bounce_probability, score.break_probability)

    quality_flags = list(model_flags or [])
    if score.role == ZoneType.AT_ZONE.value:
        quality_flags.append("NO_DIRECTION")
    if score.confidence_level == ConfidenceLevel.LOW.value:
        quality_flags.append("LOW_CONFIDENCE")
    if edge is not None and edge < 10.0:
        quality_flags.append("LOW_PROBABILITY_EDGE")
    if score.bounce_probability is None or score.break_probability is None:
        quality_flags.append("MISSING_DIRECTIONAL_PROBABILITY")

    return {
        "schema_version": PROBABILITY_CONTEXT_SCHEMA_VERSION,
        "bounce_probability": score.bounce_probability,
        "break_probability": score.break_probability,
        "neutral_probability": neutral,
        "dominant_outcome": dominant,
        "edge_pp": edge,
        "quality_flags": sorted(set(quality_flags)),
    }


def build_analysis_probability_context(
    scores: AnalysisScores,
    zone_contexts: list[dict[str, Any]] | None = None,
    model_flags: list[str] | None = None,
) -> dict[str, Any]:
    metrics = scores.features.data.model.metrics or {}
    if model_flags is None:
        model_flags = model_quality_flags(scores)
    if zone_contexts is None:
        zone_contexts = [
            build_zone_probability_context(score, model_flags)
            for score in scores.zones
        ]
    usable = [
        item for item in zone_contexts
        if item["bounce_probability"] is not None and item["break_probability"] is not None
    ]
    avg_edge = (
        sum(float(item["edge_pp"]) for item in usable) / len(usable)
        if usable else None
    )
    return {
        "schema_version": PROBABILITY_CONTEXT_SCHEMA_VERSION,
        "model_metrics": {
            "hold": {
                "auc": _metric(metrics, "hold", "auc"),
                "brier_score": _metric(metrics, "hold", "brier_score"),
                "log_loss": _metric(metrics, "hold", "log_loss"),
                "calibrated": _metric(metrics, "hold", "calibrated"),
                "test_rows": _metric(metrics, "hold", "test_rows"),
            },
            "break": {
                "auc": _metric(metrics, "break", "auc"),
                "brier_score": _metric(metrics, "break", "brier_score"),
                "log_loss": _metric(metrics, "break", "log_loss"),
                "calibrated": _metric(metrics, "break", "calibrated"),
                "test_rows": _metric(metrics, "break", "test_rows"),
            },
        },
        "health": {
            "quality_flags": sorted(set(model_flags)),
            "average_edge_pp": avg_edge,
            "directional_zone_count": len(usable),
            "zone_count": len(zone_contexts),
        },
    }
