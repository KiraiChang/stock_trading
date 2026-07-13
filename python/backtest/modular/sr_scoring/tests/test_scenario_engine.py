from __future__ import annotations

import pandas as pd

from ..model import ModelBundle
from ..pipeline_types import AnalysisData, AnalysisEvidence, AnalysisFeatures, AnalysisScores
from ..scenario_engine import SCENARIO_SCHEMA_VERSION, build_analysis_scenario, build_zone_scenario
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
        trained_at="2026-07-13T00:00:00Z",
        version="scenario-test",
        config_hash="cfg123",
    )


def _zone(role: str = ZoneType.SUPPORT.value, recent_validation: str = RecentValidation.VALIDATED_RECENTLY.value) -> ZoneScore:
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
        confidence=0.72,
        confidence_level=ConfidenceLevel.HIGH.value,
        bounce_probability=0.68 if role != ZoneType.AT_ZONE.value else None,
        break_probability=0.32 if role != ZoneType.AT_ZONE.value else None,
        expected_gain=0.04 if role != ZoneType.AT_ZONE.value else None,
        expected_loss=-0.02 if role != ZoneType.AT_ZONE.value else None,
        expected_value=0.02 if role != ZoneType.AT_ZONE.value else None,
        risk_reward_ratio=1.8 if role != ZoneType.AT_ZONE.value else None,
        reward_risk_percentile=80.0 if role != ZoneType.AT_ZONE.value else None,
        relative_volume=1.2 if role != ZoneType.AT_ZONE.value else None,
        volume_confirmation=VolumeConfirmation.CONFIRMED.value if role != ZoneType.AT_ZONE.value else None,
        touch_count=5,
        support_touch_count=5 if role == ZoneType.SUPPORT.value else 0,
        resistance_touch_count=5 if role == ZoneType.RESISTANCE.value else 0,
        reject_count=4,
        break_count=1,
        zone_momentum=0.01,
        zone_direction=ZoneDirection.UP.value,
        recent_validation=recent_validation,
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


def test_support_zone_scenario_contract():
    scenario = build_zone_scenario(_zone(ZoneType.SUPPORT.value))

    assert scenario["schema_version"] == SCENARIO_SCHEMA_VERSION
    assert scenario["state"] == "SUPPORT_RETEST"
    assert "支撐" in scenario["title"]
    assert any("收盤跌破 98.00" in item for item in scenario["invalidation_conditions"])


def test_resistance_zone_scenario_contract():
    scenario = build_zone_scenario(_zone(ZoneType.RESISTANCE.value))

    assert scenario["state"] == "RESISTANCE_REJECTION"
    assert "壓力" in scenario["title"]
    assert any("收盤突破 100.00" in item for item in scenario["invalidation_conditions"])


def test_at_zone_scenario_waits_for_direction():
    scenario = build_zone_scenario(_zone(ZoneType.AT_ZONE.value))

    assert scenario["state"] == "WAIT_FOR_DIRECTION"
    assert "等待方向" in scenario["title"]
    assert any("收盤站上 100.00" in item for item in scenario["trigger_conditions"])
    assert any("收盤跌破 98.00" in item for item in scenario["trigger_conditions"])


def test_analysis_scenario_uses_decision_context():
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
    features = AnalysisFeatures(data=data, global_trend=0.02, global_volatility=0.02, ma5=None, zones=tuple())
    scores = AnalysisScores(
        features=features,
        zones=(zone,),
        global_metrics={"confidence": 0.6, "expected_value": 0.01, "risk_reward_ratio": 1.5},
        chip_summary={"missing": True, "score": None},
    )
    evidence = AnalysisEvidence(scores=scores, global_evidence={}, zone_evidence=tuple())

    scenario = build_analysis_scenario(evidence, {
        "action": "BuySmall",
        "action_label": "小量試單",
        "market_regime": {"primary": "TREND_UP", "label": "偏多趨勢"},
        "primary_zone": {"price_low": 98.0, "price_high": 100.0, "label": "98.00 ~ 100.00"},
        "risk_notes": ["波動偏高，倉位需保守。"],
    })

    assert scenario["schema_version"] == SCENARIO_SCHEMA_VERSION
    assert scenario["state"] == "BuySmall"
    assert "小量試單" in scenario["title"]
    assert "98.00 ~ 100.00" in scenario["summary"]
    assert scenario["global_confidence"] == 0.6
