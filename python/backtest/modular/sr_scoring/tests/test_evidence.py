from __future__ import annotations

import numpy as np
import pytest

pytest.importorskip("shap")

from ..evidence import SHAP_TOLERANCE, _additivity_error, explain_direction
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
