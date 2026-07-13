from __future__ import annotations

from types import SimpleNamespace

import numpy as np
import pytest

from .. import evidence as evidence_mod
from ..evidence import SHAP_TOLERANCE, _additivity_error, build_evidence, explain_direction
from ..model import FEATURE_COLUMNS, ModelBundle
from ..pipeline_types import DirectionFeatures
from ..types import ZoneFeatures


class _LinearProbability:
    def __init__(self, sign: float):
        self.sign = sign

    def predict_proba(self, matrix):
        values = np.asarray(matrix, dtype=float)
        probability = np.clip(0.5 + self.sign * values[:, 0] * 0.02, 0.01, 0.99)
        return np.column_stack([1.0 - probability, probability])


def test_permutation_shap_reconstructs_normalized_final_probability():
    pytest.importorskip("shap")
    background = np.zeros((2, len(FEATURE_COLUMNS)), dtype=float)
    bundle = ModelBundle(
        hold_model=_LinearProbability(1.0),
        break_model=_LinearProbability(-1.0),
        feature_names=list(FEATURE_COLUMNS),
        trained_at="2026-07-09T00:00:00Z",
        version="v4",
        explanation_background=background.tolist(),
    )
    zone_features = ZoneFeatures(4, 2, 1, 0.03, -0.02, 1.2, 0.02, 0.01)
    vector = np.zeros((1, len(FEATURE_COLUMNS)), dtype=float)
    vector[0, 0] = 4.0

    result = explain_direction(bundle, DirectionFeatures("SUPPORT", zone_features, vector))

    for target in ("hold", "break"):
        item = result["targets"][target]
        reconstructed = item["baseline_probability"] + sum(
            contribution["contribution"] for contribution in item["contributions"]
        )
        assert reconstructed == pytest.approx(item["final_probability"], abs=1e-6)
        assert item["additivity_error"] <= SHAP_TOLERANCE
        assert len(item["contributions"]) == len(FEATURE_COLUMNS)


def test_additivity_allows_expected_float32_rounding():
    assert _additivity_error(0.500002, 0.5) == pytest.approx(0.000002)


def test_additivity_rejects_material_mismatch():
    with pytest.raises(RuntimeError, match="SHAP additivity failed"):
        _additivity_error(0.5 + SHAP_TOLERANCE * 2, 0.5)


# ── build_evidence 降級 / top-N ──────────────────────────────────


def _fake_scores(trading_scores: list[float], *, with_background: bool, confidence: float = 0.7):
    """用 SimpleNamespace 組最小 AnalysisScores；build_evidence 只讀特定屬性。"""
    n = len(trading_scores)
    bg = np.zeros((2, len(FEATURE_COLUMNS)), dtype=float)
    bundle = ModelBundle(
        hold_model=_LinearProbability(1.0),
        break_model=_LinearProbability(-1.0),
        feature_names=list(FEATURE_COLUMNS),
        trained_at="2026-07-13T00:00:00Z",
        version="v4" if with_background else "v3",
        config_hash="",  # 空字串→_stored_background 以 id(bundle) 當 key，測試間不撞
        explanation_background=(bg.tolist() if with_background else []),
    )
    vec = np.zeros(len(FEATURE_COLUMNS), dtype=float)
    zone_feature_sets = tuple(
        SimpleNamespace(
            support=SimpleNamespace(role="SUPPORT", model_vector=vec),
            resistance=SimpleNamespace(role="RESISTANCE", model_vector=vec),
        )
        for _ in range(n)
    )
    features = SimpleNamespace(
        data=SimpleNamespace(model=bundle),
        global_trend=0.02,
        global_volatility=0.02,
        zones=zone_feature_sets,
    )
    zone_scores = tuple(
        SimpleNamespace(
            price_low=90.0,
            price_high=95.0,
            confidence=confidence,
            recent_validation="PENDING_VALIDATION",
            role="SUPPORT",
            trading_score=ts,
        )
        for ts in trading_scores
    )
    return SimpleNamespace(
        features=features,
        zones=zone_scores,
        global_metrics={"confidence": confidence},
        chip_summary={"missing": True, "score": None},
    )


def test_build_evidence_degrades_without_background():
    evidence_mod._BACKGROUND_CACHE.clear()
    scores = _fake_scores([80.0, 60.0], with_background=False, confidence=0.3)

    evidence = build_evidence(scores, evidence_enabled=True, evidence_max_zones=8)

    assert evidence.global_evidence["model"]["explainer"] is None
    assert evidence.global_evidence["model"]["evidence_available"] is False
    for zone in evidence.zone_evidence:
        assert zone["support"] is None and zone["resistance"] is None
        # risk_flags 為純規則，降級仍要保留（confidence 0.3 → LOW_CONFIDENCE）
        assert "LOW_CONFIDENCE" in zone["risk_flags"]


def test_build_evidence_degrades_when_disabled():
    evidence_mod._BACKGROUND_CACHE.clear()
    scores = _fake_scores([80.0], with_background=True)

    evidence = build_evidence(scores, evidence_enabled=False, evidence_max_zones=8)

    assert evidence.global_evidence["model"]["evidence_available"] is False
    assert evidence.zone_evidence[0]["support"] is None


def test_build_evidence_degrades_when_shap_missing(monkeypatch):
    evidence_mod._BACKGROUND_CACHE.clear()
    monkeypatch.setattr(evidence_mod, "_shap_available", lambda: False)
    scores = _fake_scores([80.0], with_background=True)

    evidence = build_evidence(scores, evidence_enabled=True, evidence_max_zones=8)

    assert evidence.global_evidence["model"]["evidence_available"] is False
    assert evidence.zone_evidence[0]["support"] is None


def test_build_evidence_top_n_limits_shap_zones():
    pytest.importorskip("shap")
    evidence_mod._BACKGROUND_CACHE.clear()
    # 最高 trading_score 故意放在 index 1，驗證是「依分數選」而非「取前幾筆」；
    # 且 by_zone 需維持原 zone 順序（前端 zip 對齊）。
    scores = _fake_scores([10.0, 90.0, 50.0], with_background=True)

    evidence = build_evidence(scores, evidence_enabled=True, evidence_max_zones=1)

    assert evidence.global_evidence["model"]["evidence_available"] is True
    # 只有最高分（index 1）拿到 SHAP targets，其餘降級為 None
    assert evidence.zone_evidence[1]["support"] is not None
    assert evidence.zone_evidence[0]["support"] is None
    assert evidence.zone_evidence[2]["support"] is None
    # 未選中的 zone 仍保有 risk_flags 鍵
    for zone in evidence.zone_evidence:
        assert "risk_flags" in zone
