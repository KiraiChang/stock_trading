"""Structured probability interpretation for SR Zone scoring.

This layer exposes probability semantics and quality flags without changing
the model output, score derivation, EV/RR, or decision thresholds.
"""
from __future__ import annotations

from typing import Any

from .pipeline_types import AnalysisScores
from .types import ConfidenceLevel, ZoneScore, ZoneType


PROBABILITY_CONTEXT_SCHEMA_VERSION = "sr_probability_context_v1"
LOW_AVERAGE_EDGE_THRESHOLD_PP = 10.0


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


def _model_metrics_summary(metrics: dict[str, Any]) -> dict[str, dict[str, float | None]]:
    return {
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
    }


def _calibration_report(scores: AnalysisScores, metrics: dict[str, Any]) -> dict[str, Any]:
    training_config = scores.features.data.model.training_config or {}
    method = training_config.get("calibration_method")
    models = {}
    for model in ("hold", "break"):
        calibrated = _metric(metrics, model, "calibrated")
        models[model] = {
            "state": "AVAILABLE" if calibrated is not None else "UNAVAILABLE",
            "method": method,
            "calibrated": bool(calibrated) if calibrated is not None else None,
            "brier_score": _metric(metrics, model, "brier_score"),
            "log_loss": _metric(metrics, model, "log_loss"),
        }
    return {"schema_version": "sr_calibration_report_v1", "models": models}


def _walk_forward_report(scores: AnalysisScores, metrics: dict[str, Any]) -> dict[str, Any]:
    split_method = scores.features.data.model.split_method or (scores.features.data.model.training_config or {}).get("split_method")
    hold_rows = _metric(metrics, "hold", "test_rows")
    break_rows = _metric(metrics, "break", "test_rows")
    state = "AVAILABLE" if split_method == "time" and hold_rows is not None and break_rows is not None else "UNAVAILABLE"
    return {
        "schema_version": "sr_walk_forward_report_v1",
        "state": state,
        "split_method": split_method,
        "test_rows": {
            "hold": hold_rows,
            "break": break_rows,
        },
        "positive_rate": {
            "hold_train": _metric(metrics, "hold", "positive_rate_train"),
            "hold_test": _metric(metrics, "hold", "positive_rate_test"),
            "break_train": _metric(metrics, "break", "positive_rate_train"),
            "break_test": _metric(metrics, "break", "positive_rate_test"),
        },
    }


def _dataset_diagnostics(scores: AnalysisScores, metrics: dict[str, Any]) -> dict[str, Any]:
    training_config = scores.features.data.model.training_config or {}
    return {
        "schema_version": "sr_dataset_diagnostics_v1",
        "state": "AVAILABLE" if training_config else "UNAVAILABLE",
        "dataset_config": training_config.get("dataset_config"),
        "zone_builders": training_config.get("zone_builders"),
        "split_method": scores.features.data.model.split_method or training_config.get("split_method"),
        "test_rows_total": sum(
            value for value in (
                _metric(metrics, "hold", "test_rows"),
                _metric(metrics, "break", "test_rows"),
            )
            if value is not None
        ),
    }


def build_model_governance_context(
    scores: AnalysisScores,
    zone_contexts: list[dict[str, Any]] | None = None,
    model_flags: list[str] | None = None,
) -> dict[str, Any]:
    metrics = scores.features.data.model.metrics or {}
    if model_flags is None:
        model_flags = model_quality_flags(scores)
    if zone_contexts is None:
        zone_contexts = [build_zone_probability_context(score, model_flags) for score in scores.zones]
    usable = [
        item for item in zone_contexts
        if item["bounce_probability"] is not None and item["break_probability"] is not None
    ]
    avg_edge = (
        sum(float(item["edge_pp"]) for item in usable) / len(usable)
        if usable else None
    )
    warning_flags = set(model_flags)
    blocking_flags: set[str] = set()
    if not usable:
        blocking_flags.add("NO_DIRECTIONAL_ZONES")
    if avg_edge is not None and avg_edge < LOW_AVERAGE_EDGE_THRESHOLD_PP:
        warning_flags.add("LOW_AVERAGE_EDGE")
    for model in ("HOLD", "BREAK"):
        if f"{model}_LOW_TEST_ROWS" in warning_flags:
            blocking_flags.add(f"{model}_LOW_TEST_ROWS")

    if blocking_flags:
        health_state = "UNRELIABLE"
        max_entry_state = "WAIT_CONFIRMATION"
    elif warning_flags:
        health_state = "DEGRADED"
        max_entry_state = "SMALL_ENTRY"
    else:
        health_state = "HEALTHY"
        max_entry_state = "BUY"

    return {
        "quality_flags": sorted(set(model_flags)),
        "warning_flags": sorted(warning_flags - blocking_flags),
        "blocking_flags": sorted(blocking_flags),
        "health_state": health_state,
        "average_edge_pp": avg_edge,
        "directional_zone_count": len(usable),
        "zone_count": len(zone_contexts),
        "confidence_gate": {
            "state": health_state,
            "allow_entry": health_state != "UNRELIABLE",
            "max_entry_state": max_entry_state,
            "reason_codes": sorted(blocking_flags or warning_flags),
        },
        "reports": {
            "calibration_report": _calibration_report(scores, metrics),
            "walk_forward_report": _walk_forward_report(scores, metrics),
            "dataset_diagnostics": _dataset_diagnostics(scores, metrics),
        },
    }


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
    health = build_model_governance_context(scores, zone_contexts, model_flags)
    return {
        "schema_version": PROBABILITY_CONTEXT_SCHEMA_VERSION,
        "model_metrics": _model_metrics_summary(metrics),
        "health": health,
        "model_reports": health["reports"],
    }
