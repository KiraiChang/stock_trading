"""Model explanations and decision-ready evidence for SR Zone scoring.

Evidence 的 SHAP 貢獻是可降級的展示層：`shap` 未安裝、或模型缺 v4
`explanation_background` 時，`build_evidence` 會把各 zone 的 support/resistance
設為 `None`（仍保留純規則的 `risk_flags`）、`global_evidence.model.explainer`
設為 `None`，而不是丟出例外讓 `/sr-zones` 整包 503。另外以 `evidence_max_zones`
只對 `trading_score` 前 N 的 zone 產生 SHAP evidence，控制熱路徑延遲。
"""
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


# 同一個 ModelBundle 的 explanation_background 只轉一次 np array，跨 zone/方向/
# 多次分析重用（bundle 由 get_model 快取成 singleton）。
_BACKGROUND_CACHE: dict[Any, np.ndarray] = {}


def _stored_background(bundle: ModelBundle) -> np.ndarray:
    key = getattr(bundle, "config_hash", None) or id(bundle)
    cached = _BACKGROUND_CACHE.get(key)
    if cached is None:
        cached = np.asarray(getattr(bundle, "explanation_background", []), dtype=float)
        _BACKGROUND_CACHE[key] = cached
    return cached


def _has_background(bundle: ModelBundle) -> bool:
    stored = _stored_background(bundle)
    return stored.ndim == 2 and len(stored) > 0 and stored.shape[1] == len(FEATURE_COLUMNS)


def _background(bundle: ModelBundle, fallback: np.ndarray) -> np.ndarray:
    stored = _stored_background(bundle)
    if stored.ndim == 2 and stored.shape[1] == len(FEATURE_COLUMNS) and len(stored):
        return stored
    return fallback


def _shap_available() -> bool:
    try:
        import shap  # noqa: F401
    except ImportError:  # pragma: no cover - deployment configuration
        return False
    return True


def _evidence_settings() -> tuple[bool, int]:
    """讀取 config 的 evidence 開關與 top-N 上限；config 不可用時回安全預設。"""
    try:
        import config

        return (
            bool(getattr(config, "SR_SCORING_EVIDENCE_ENABLED", True)),
            int(getattr(config, "SR_SCORING_EVIDENCE_MAX_ZONES", 8)),
        )
    except Exception:  # pragma: no cover - config 不可用時的防禦
        return True, 8


def _build_explainers(bundle: ModelBundle, background: np.ndarray) -> dict[str, tuple[Callable, Any]]:
    """每次分析建一次 masker 與 hold/break explainer，跨 zone 重用。"""
    import shap

    masker = shap.maskers.Independent(background)
    built: dict[str, tuple[Callable, Any]] = {}
    for target in ("hold", "break"):
        fn = _normalized_probability_fn(bundle, target)
        explainer = shap.Explainer(
            fn, masker, algorithm="permutation", feature_names=FEATURE_COLUMNS
        )
        built[target] = (fn, explainer)
    return built


def _explain_vector(explainers: dict[str, tuple[Callable, Any]], role: str, model_vector: Any) -> dict[str, Any]:
    vector = np.asarray(model_vector, dtype=float).reshape(1, -1)
    targets: dict[str, Any] = {}
    for target, (fn, explainer) in explainers.items():
        explanation = explainer(vector, max_evals=SHAP_MAX_EVALS, silent=True)
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
    return {"role": role, "targets": targets}


def explain_direction(bundle: ModelBundle, features: DirectionFeatures) -> dict[str, Any]:
    """Explain calibrated, normalized hold and break probabilities.

    PermutationExplainer works against the public probability function and is
    therefore independent of the estimator and calibration wrapper in use.
    需要 `shap`；缺套件時丟 RuntimeError（呼叫端 build_evidence 會先偵測能力以降級）。
    """
    try:
        import shap  # noqa: F401
    except ImportError as exc:  # pragma: no cover - deployment configuration
        raise RuntimeError("SHAP evidence requires the 'shap' package") from exc

    vector = np.asarray(features.model_vector, dtype=float).reshape(1, -1)
    background = _background(bundle, vector)
    explainers = _build_explainers(bundle, background)
    return _explain_vector(explainers, features.role, features.model_vector)


def _risk_flags(score: Any) -> list[str]:
    return [
        flag
        for flag, active in (
            ("LOW_CONFIDENCE", score.confidence < 0.45),
            ("EXPIRED", score.recent_validation == "EXPIRED"),
            ("NO_DIRECTION", score.role == "AT_ZONE"),
        )
        if active
    ]


def build_evidence(
    scores: AnalysisScores,
    *,
    evidence_enabled: bool | None = None,
    evidence_max_zones: int | None = None,
) -> AnalysisEvidence:
    bundle = scores.features.data.model
    if evidence_enabled is None or evidence_max_zones is None:
        cfg_enabled, cfg_max = _evidence_settings()
        if evidence_enabled is None:
            evidence_enabled = cfg_enabled
        if evidence_max_zones is None:
            evidence_max_zones = cfg_max

    # 能力偵測：開關開、shap 可用、模型有 v4 background 才產 SHAP evidence。
    shap_ready = bool(evidence_enabled) and _shap_available() and _has_background(bundle)

    # top-N：只對 trading_score 最高的前 N 個 zone 產 SHAP evidence（0 或負數=全部），
    # 其餘 zone 降級。以 (trading_score desc, index asc) 排序維持確定性。
    selected: set[int] = set()
    if shap_ready:
        order = sorted(
            range(len(scores.zones)),
            key=lambda i: (scores.zones[i].trading_score, -i),
            reverse=True,
        )
        limit = evidence_max_zones if evidence_max_zones and evidence_max_zones > 0 else len(order)
        selected = set(order[:limit])

    explainers = _build_explainers(bundle, _stored_background(bundle)) if selected else None

    by_zone = []
    for idx, (feature_set, score) in enumerate(zip(scores.features.zones, scores.zones)):
        if explainers is not None and idx in selected:
            support = _explain_vector(explainers, feature_set.support.role, feature_set.support.model_vector)
            resistance = _explain_vector(explainers, feature_set.resistance.role, feature_set.resistance.model_vector)
        else:
            support = None
            resistance = None
        by_zone.append({
            "price_low": score.price_low,
            "price_high": score.price_high,
            "support": support,
            "resistance": resistance,
            "risk_flags": _risk_flags(score),
        })

    global_evidence = {
        "trend": scores.features.global_trend,
        "volatility": scores.features.global_volatility,
        "metrics": scores.global_metrics,
        "chip": scores.chip_summary,
        "model": {
            "version": bundle.version,
            "config_hash": bundle.config_hash,
            # explainer 為 None 時代表 evidence 降級；explanation/scenario 據此判斷
            # uses_shap，前端 badge 顯示「rules only」。
            "explainer": "permutation_shap" if shap_ready else None,
            "explained_output": "calibrated_normalized_probability",
            "evidence_available": bool(shap_ready),
        },
    }
    return AnalysisEvidence(scores=scores, global_evidence=global_evidence, zone_evidence=tuple(by_zone))
