from __future__ import annotations

import pandas as pd

from ..explain_engine import build_explanation, explain_zone
from ..model import ModelBundle
from ..pipeline_types import AnalysisData, AnalysisEvidence, AnalysisFeatures, AnalysisScores
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


def _bundle() -> ModelBundle:
    return ModelBundle(
        hold_model=None,
        break_model=None,
        feature_names=["touch_count"],
        trained_at="2026-07-08T00:00:00Z",
        version="explain-test",
        config_hash="cfg123",
    )


def _zone(
    role: str = ZoneType.SUPPORT.value,
    confidence_level: str = ConfidenceLevel.HIGH.value,
    recent_validation: str = RecentValidation.VALIDATED_RECENTLY.value,
    volume_confirmation: str | None = VolumeConfirmation.CONFIRMED.value,
    expected_value: float | None = 0.02,
    risk_reward_ratio: float | None = 1.8,
    trading_score_breakdown: dict[str, float] | None = None,
) -> ZoneScore:
    return ZoneScore(
        price_low=98.0,
        price_high=100.0,
        method="atr",
        role=role,
        tier=ZoneTier.TIER_1_MAIN_STRUCTURE.value,
        tier_label="主結構",
        support_score=0.8 if role == ZoneType.SUPPORT.value else 0.2,
        resistance_score=0.8 if role == ZoneType.RESISTANCE.value else 0.2,
        net_score=0.6 if role == ZoneType.SUPPORT.value else -0.6,
        net_score_label=NetScoreLabel.STRONG_SUPPORT.value if role == ZoneType.SUPPORT.value else NetScoreLabel.STRONG_RESISTANCE.value,
        confidence=0.72 if confidence_level != ConfidenceLevel.LOW.value else 0.2,
        confidence_level=confidence_level,
        bounce_probability=0.68 if role != ZoneType.AT_ZONE.value else None,
        break_probability=0.32 if role != ZoneType.AT_ZONE.value else None,
        expected_gain=0.04 if role != ZoneType.AT_ZONE.value else None,
        expected_loss=-0.02 if role != ZoneType.AT_ZONE.value else None,
        expected_value=expected_value,
        risk_reward_ratio=risk_reward_ratio,
        reward_risk_percentile=80.0 if risk_reward_ratio is not None else None,
        relative_volume=1.2 if role != ZoneType.AT_ZONE.value else None,
        volume_confirmation=volume_confirmation if role != ZoneType.AT_ZONE.value else None,
        touch_count=5,
        support_touch_count=5 if role == ZoneType.SUPPORT.value else 0,
        resistance_touch_count=5 if role == ZoneType.RESISTANCE.value else 0,
        reject_count=4,
        break_count=1,
        zone_momentum=0.01,
        zone_direction=ZoneDirection.UP.value,
        recent_validation=recent_validation,
        trading_score=78.0,
        trading_score_breakdown=trading_score_breakdown or {
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


def _zone_evidence(*risk_flags: str) -> dict:
    return {
        "risk_flags": list(risk_flags),
        "support": {"targets": {"hold": {"contributions": [{"feature": "touch_count", "contribution": 0.1}]}}},
        "resistance": {"targets": {"hold": {"contributions": [{"feature": "touch_count", "contribution": -0.1}]}}},
    }


def test_support_zone_explanation_uses_support_language():
    explanation = explain_zone(_zone(ZoneType.SUPPORT.value), _zone_evidence())

    assert "支撐" in explanation["role_summary"]
    assert any("信心等級高" in factor for factor in explanation["positive_factors"])
    assert any("跌破 98.00" in condition for condition in explanation["watch_conditions"])


def test_resistance_zone_explanation_uses_resistance_language():
    explanation = explain_zone(_zone(ZoneType.RESISTANCE.value), _zone_evidence())

    assert "壓力" in explanation["role_summary"]
    assert "按壓力解讀" in explanation["probability_reason"]
    assert any("突破 100.00" in condition for condition in explanation["watch_conditions"])


def test_at_zone_explanation_makes_direction_uncertain():
    explanation = explain_zone(
        _zone(ZoneType.AT_ZONE.value, expected_value=None, risk_reward_ratio=None),
        _zone_evidence("NO_DIRECTION"),
    )

    assert "方向尚未解析" in explanation["role_summary"]
    assert "不給出方向性" in explanation["probability_reason"]
    assert any("方向尚未解析" in factor for factor in explanation["negative_factors"])


def test_risk_flags_generate_plain_language_risks():
    explanation = explain_zone(
        _zone(
            ZoneType.SUPPORT.value,
            confidence_level=ConfidenceLevel.LOW.value,
            recent_validation=RecentValidation.EXPIRED.value,
            volume_confirmation=VolumeConfirmation.FAILED.value,
            expected_value=-0.01,
            risk_reward_ratio=0.8,
        ),
        _zone_evidence("LOW_CONFIDENCE", "EXPIRED"),
    )

    negatives = " ".join(explanation["negative_factors"])
    assert "信心偏低" in negatives
    assert "可能已失效" in negatives
    assert "期望值為負" in negatives


def test_score_breakdown_extremes_are_stably_sorted():
    explanation = explain_zone(
        _zone(trading_score_breakdown={
            "expected_value": 20,
            "risk_reward": 20,
            "trend": 8,
            "volume": 2,
            "confidence": 3,
            "chip": 9,
        }),
        _zone_evidence(),
    )

    assert "風險報酬比貢獻 20.0 分" in explanation["score_reason"]
    assert "量能 2.0 分" in explanation["score_reason"]


def test_analysis_explanation_includes_high_volatility_and_model_context():
    zone = _zone()
    data = AnalysisData(
        symbol="2330",
        timeframe="1d",
        frame=pd.DataFrame(),
        analyzed_at=pd.Timestamp("2026-07-13T00:00:00Z"),
        current_price=102.0,
        zones=tuple(),
        model=_bundle(),
        chip_row=None,
        chip_features={},
    )
    features = AnalysisFeatures(
        data=data,
        global_trend=0.02,
        global_volatility=0.04,
        ma5=None,
        zones=tuple(),
    )
    scores = AnalysisScores(
        features=features,
        zones=(zone,),
        global_metrics={"confidence": 0.4, "expected_value": 0.01, "risk_reward_ratio": 1.5},
        chip_summary={"missing": True, "score": None},
    )
    evidence = AnalysisEvidence(
        scores=scores,
        global_evidence={"model": {"explainer": "permutation_shap"}},
        zone_evidence=(_zone_evidence(),),
    )

    explanation = build_explanation(evidence, {
        "action": "Hold",
        "action_label": "等待",
        "primary_zone": None,
        "risk_notes": [],
    })

    assert "等待" in explanation["summary"]
    assert any("波動偏高" in note for note in explanation["risk_notes"])
    assert any("信心偏低" in note for note in explanation["risk_notes"])
    assert explanation["model_context"] == {
        "version": "explain-test",
        "config_hash": "cfg123",
        "uses_shap_evidence": True,
    }
