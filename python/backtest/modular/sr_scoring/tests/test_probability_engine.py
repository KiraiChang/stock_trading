from __future__ import annotations

import pandas as pd
import pytest

from ..model import ModelBundle
from ..pipeline_types import AnalysisData, AnalysisFeatures, AnalysisScores
from ..probability_engine import (
    PROBABILITY_CONTEXT_SCHEMA_VERSION,
    build_analysis_probability_context,
    build_model_governance_context,
    build_zone_probability_context,
)
from ..types import (
    ConfidenceLevel,
    NetScoreLabel,
    RecentValidation,
    TradingRecommendation,
    VolumeConfirmation,
    ZoneDirection,
    ZoneScore,
    ZoneTier,
    ZoneType,
)


def _zone(
    role: str = ZoneType.SUPPORT.value,
    bounce_probability: float | None = 0.62,
    break_probability: float | None = 0.28,
    confidence_level: str = ConfidenceLevel.HIGH.value,
) -> ZoneScore:
    directional = role != ZoneType.AT_ZONE.value
    return ZoneScore(
        price_low=98.0,
        price_high=100.0,
        method="atr",
        role=role,
        tier=ZoneTier.TIER_1_MAIN_STRUCTURE.value,
        tier_label="主結構",
        support_score=0.8,
        resistance_score=0.2,
        net_score=0.6,
        net_score_label=NetScoreLabel.STRONG_SUPPORT.value,
        confidence=0.72 if confidence_level != ConfidenceLevel.LOW.value else 0.2,
        confidence_level=confidence_level,
        bounce_probability=bounce_probability if directional else None,
        break_probability=break_probability if directional else None,
        expected_gain=0.04 if directional else None,
        expected_loss=-0.02 if directional else None,
        expected_value=0.02 if directional else None,
        risk_reward_ratio=1.8 if directional else None,
        reward_risk_percentile=80.0 if directional else None,
        relative_volume=1.2 if directional else None,
        volume_confirmation=VolumeConfirmation.CONFIRMED.value if directional else None,
        touch_count=5,
        support_touch_count=5 if role == ZoneType.SUPPORT.value else 0,
        resistance_touch_count=5 if role == ZoneType.RESISTANCE.value else 0,
        reject_count=4,
        break_count=1,
        zone_momentum=0.01,
        zone_direction=ZoneDirection.UP.value,
        recent_validation=RecentValidation.VALIDATED_RECENTLY.value,
        trading_score=78.0,
        trading_score_breakdown={
            "expected_value": 30,
            "risk_reward": 15,
            "trend": 12,
            "volume": 10,
            "confidence": 9,
            "chip": 2,
        },
        trading_recommendation=TradingRecommendation.BUY.value,
        overlap_group=1,
        confluence_count=2,
    )


def test_zone_probability_context_derives_neutral_and_edge():
    context = build_zone_probability_context(_zone())

    assert context["schema_version"] == PROBABILITY_CONTEXT_SCHEMA_VERSION
    assert context["neutral_probability"] == pytest.approx(0.10)
    assert context["dominant_outcome"] == "BOUNCE"
    assert context["edge_pp"] == pytest.approx(34.0)
    assert context["quality_flags"] == []


def test_zone_probability_context_flags_low_edge_and_missing_direction():
    low_edge = build_zone_probability_context(_zone(bounce_probability=0.45, break_probability=0.40))
    at_zone = build_zone_probability_context(_zone(ZoneType.AT_ZONE.value))

    assert "LOW_PROBABILITY_EDGE" in low_edge["quality_flags"]
    assert at_zone["dominant_outcome"] == "NO_DIRECTION"
    assert "NO_DIRECTION" in at_zone["quality_flags"]
    assert "MISSING_DIRECTIONAL_PROBABILITY" in at_zone["quality_flags"]


def test_analysis_probability_context_summarizes_model_quality():
    bundle = ModelBundle(
        hold_model=None,
        break_model=None,
        feature_names=["touch_count"],
        trained_at="2026-07-13T00:00:00Z",
        version="probability-test",
        config_hash="cfg123",
        metrics={
            "hold": {"auc": 0.7, "brier_score": 0.2, "log_loss": 0.5, "calibrated": 0.0, "test_rows": 12},
            "break": {"auc": 0.6, "brier_score": 0.22, "log_loss": 0.6, "calibrated": 1.0, "test_rows": 30},
        },
    )
    data = AnalysisData(
        symbol="2330",
        timeframe="1d",
        frame=pd.DataFrame(),
        analyzed_at=pd.Timestamp("2026-07-13T00:00:00Z"),
        current_price=102.0,
        zones=tuple(),
        model=bundle,
        chip_row=None,
        chip_features={},
    )
    scores = AnalysisScores(
        features=AnalysisFeatures(data=data, global_trend=0.02, global_volatility=0.02, ma5=None, zones=tuple()),
        zones=(_zone(), _zone(ZoneType.AT_ZONE.value)),
        global_metrics={"confidence": 0.6, "expected_value": 0.01, "risk_reward_ratio": 1.5},
        chip_summary={"missing": True},
    )

    context = build_analysis_probability_context(scores)

    assert context["schema_version"] == PROBABILITY_CONTEXT_SCHEMA_VERSION
    assert context["model_metrics"]["hold"]["auc"] == 0.7
    assert context["health"]["directional_zone_count"] == 1
    assert context["health"]["average_edge_pp"] == pytest.approx(34.0)
    assert context["health"]["health_state"] == "UNRELIABLE"
    assert context["health"]["confidence_gate"]["allow_entry"] is False
    assert "HOLD_LOW_TEST_ROWS" in context["health"]["blocking_flags"]
    assert "HOLD_NOT_CALIBRATED" in context["health"]["quality_flags"]
    assert "HOLD_LOW_TEST_ROWS" in context["health"]["quality_flags"]
    assert context["model_reports"]["calibration_report"]["schema_version"] == "sr_calibration_report_v1"
    assert context["model_reports"]["walk_forward_report"]["schema_version"] == "sr_walk_forward_report_v1"
    assert context["model_reports"]["dataset_diagnostics"]["schema_version"] == "sr_dataset_diagnostics_v1"


def test_model_governance_reports_degraded_when_uncalibrated_but_enough_rows():
    bundle = ModelBundle(
        hold_model=None,
        break_model=None,
        feature_names=["touch_count"],
        trained_at="2026-07-13T00:00:00Z",
        version="probability-test",
        config_hash="cfg123",
        split_method="time",
        training_config={"split_method": "time", "calibration_method": "none", "dataset_config": {"min_history_bars": 90}},
        metrics={
            "hold": {"auc": 0.7, "brier_score": 0.2, "log_loss": 0.5, "calibrated": 0.0, "test_rows": 50},
            "break": {"auc": 0.6, "brier_score": 0.22, "log_loss": 0.6, "calibrated": 1.0, "test_rows": 50},
        },
    )
    data = AnalysisData(
        symbol="2330",
        timeframe="1d",
        frame=pd.DataFrame(),
        analyzed_at=pd.Timestamp("2026-07-13T00:00:00Z"),
        current_price=102.0,
        zones=tuple(),
        model=bundle,
        chip_row=None,
        chip_features={},
    )
    scores = AnalysisScores(
        features=AnalysisFeatures(data=data, global_trend=0.02, global_volatility=0.02, ma5=None, zones=tuple()),
        zones=(_zone(),),
        global_metrics={"confidence": 0.6, "expected_value": 0.01, "risk_reward_ratio": 1.5},
        chip_summary={"missing": True},
    )

    health = build_model_governance_context(scores)

    assert health["health_state"] == "DEGRADED"
    assert health["confidence_gate"]["allow_entry"] is True
    assert health["confidence_gate"]["max_entry_state"] == "SMALL_ENTRY"
    assert "HOLD_NOT_CALIBRATED" in health["warning_flags"]
    assert health["reports"]["walk_forward_report"]["state"] == "AVAILABLE"
