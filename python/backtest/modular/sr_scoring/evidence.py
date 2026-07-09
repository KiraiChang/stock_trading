"""Model explanations and decision-ready evidence for SR Zone scoring."""
from __future__ import annotations

from typing import Any, Callable

import numpy as np

from .model import FEATURE_COLUMNS, ModelBundle
from .pipeline_types import AnalysisEvidence, AnalysisScores, DirectionFeatures

SHAP_MAX_EVALS = 2 * len(FEATURE_COLUMNS) + 1
SHAP_TOLERANCE = 1e-5


def _additivity_error(reconstructed: float, probability: float) -> float:
    error = abs(reconstructed - probability)
    if error > SHAP_TOLERANCE:
        raise RuntimeError(
            f"SHAP additivity failed: reconstructed={reconstructed} probability={probability} error={error}"
        )
    return error


def _positive_probability(model: Any, matrix: np.ndarray) -> np.ndarray:
    return np.asarray(model.predict_proba(matrix), dtype=float)[:, 1]


def _normalized_probability_fn(bundle: ModelBundle, target: str) -> Callable[[np.ndarray], np.ndarray]:
    def predict(matrix: np.ndarray) -> np.ndarray:
        hold = _positive_probability(bundle.hold_model, matrix)
        brk = _positive_probability(bundle.break_model, matrix)
        total = hold + brk
        scale = np.where(total > 1.0, total, 1.0)
        return (hold if target == "hold" else brk) / scale

    return predict


def _background(bundle: ModelBundle, fallback: np.ndarray) -> np.ndarray:
    stored = np.asarray(getattr(bundle, "explanation_background", []), dtype=float)
    if stored.ndim == 2 and stored.shape[1] == len(FEATURE_COLUMNS) and len(stored):
        return stored
    return fallback


def explain_direction(bundle: ModelBundle, features: DirectionFeatures) -> dict[str, Any]:
    """Explain calibrated, normalized hold and break probabilities.

    PermutationExplainer works against the public probability function and is
    therefore independent of the estimator and calibration wrapper in use.
    """
    try:
        import shap
    except ImportError as exc:  # pragma: no cover - deployment configuration
        raise RuntimeError("SHAP evidence requires the 'shap' package") from exc

    vector = np.asarray(features.model_vector, dtype=float).reshape(1, -1)
    background = _background(bundle, vector)
    targets: dict[str, Any] = {}
    for target in ("hold", "break"):
        fn = _normalized_probability_fn(bundle, target)
        explanation = shap.Explainer(
            fn,
            shap.maskers.Independent(background),
            algorithm="permutation",
            feature_names=FEATURE_COLUMNS,
        )(vector, max_evals=SHAP_MAX_EVALS, silent=True)
        baseline = float(np.asarray(explanation.base_values).reshape(-1)[0])
        values = np.asarray(explanation.values, dtype=float).reshape(-1)
        probability = float(fn(vector)[0])
        reconstructed = baseline + float(values.sum())
        additivity_error = _additivity_error(reconstructed, probability)
        contributions = [
            {
                "feature": name,
                "value": float(vector[0, i]),
                "contribution": float(values[i]),
                "direction": "supportive" if values[i] > 0 else "opposing" if values[i] < 0 else "neutral",
            }
            for i, name in enumerate(FEATURE_COLUMNS)
        ]
        contributions.sort(key=lambda item: abs(item["contribution"]), reverse=True)
        targets[target] = {
            "baseline_probability": baseline,
            "final_probability": probability,
            "additivity_error": additivity_error,
            "contributions": contributions,
        }
    return {"role": features.role, "targets": targets}


def build_evidence(scores: AnalysisScores) -> AnalysisEvidence:
    by_zone = []
    for feature_set, score in zip(scores.features.zones, scores.zones):
        by_zone.append({
            "price_low": score.price_low,
            "price_high": score.price_high,
            "support": explain_direction(scores.features.data.model, feature_set.support),
            "resistance": explain_direction(scores.features.data.model, feature_set.resistance),
            "risk_flags": [
                flag for flag, active in (
                    ("LOW_CONFIDENCE", score.confidence < 0.45),
                    ("EXPIRED", score.recent_validation == "EXPIRED"),
                    ("NO_DIRECTION", score.role == "AT_ZONE"),
                ) if active
            ],
        })
    global_evidence = {
        "trend": scores.features.global_trend,
        "volatility": scores.features.global_volatility,
        "metrics": scores.global_metrics,
        "chip": scores.chip_summary,
        "model": {
            "version": scores.features.data.model.version,
            "config_hash": scores.features.data.model.config_hash,
            "explainer": "permutation_shap",
            "explained_output": "calibrated_normalized_probability",
        },
    }
    return AnalysisEvidence(scores=scores, global_evidence=global_evidence, zone_evidence=tuple(by_zone))
