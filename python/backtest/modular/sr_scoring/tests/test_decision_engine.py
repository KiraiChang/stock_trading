from __future__ import annotations

from ..decision_engine import build_decision_summary
from ..model import ModelBundle
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
        version="decision-test",
        config_hash="abc123",
    )


def _zone(
    role: str = ZoneType.SUPPORT.value,
    low: float = 98.0,
    high: float = 100.0,
    trading_score: float = 78.0,
    confidence: float = 0.72,
    confidence_level: str = ConfidenceLevel.HIGH.value,
    expected_value: float | None = 0.02,
    risk_reward_ratio: float | None = 2.2,
    recent_validation: str = RecentValidation.VALIDATED_RECENTLY.value,
    confluence_count: int = 2,
) -> ZoneScore:
    return ZoneScore(
        price_low=low,
        price_high=high,
        method="atr",
        role=role,
        tier=ZoneTier.TIER_1_MAIN_STRUCTURE.value,
        tier_label="主結構",
        support_score=0.8 if role == ZoneType.SUPPORT.value else 0.2,
        resistance_score=0.8 if role == ZoneType.RESISTANCE.value else 0.2,
        net_score=0.6 if role == ZoneType.SUPPORT.value else -0.6,
        net_score_label=NetScoreLabel.STRONG_SUPPORT.value if role == ZoneType.SUPPORT.value else NetScoreLabel.STRONG_RESISTANCE.value,
        confidence=confidence,
        confidence_level=confidence_level,
        bounce_probability=0.68 if role != ZoneType.AT_ZONE.value else None,
        break_probability=0.32 if role != ZoneType.AT_ZONE.value else None,
        expected_gain=0.04 if role != ZoneType.AT_ZONE.value else None,
        expected_loss=-0.02 if role != ZoneType.AT_ZONE.value else None,
        expected_value=expected_value,
        risk_reward_ratio=risk_reward_ratio,
        reward_risk_percentile=80.0 if risk_reward_ratio is not None else None,
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
        trading_score=trading_score,
        trading_score_breakdown={
            "expected_value": 30,
            "risk_reward": 15,
            "trend": 12,
            "volume": 12,
            "confidence": 9,
            "chip": 0,
        },
        trading_recommendation=TradingRecommendation.BUY.value,
        overlap_group=1 if confluence_count > 1 else None,
        confluence_count=confluence_count,
    )


def _summary(
    zones: list[ZoneScore],
    current_price: float = 102.0,
    global_trend: float = 0.03,
    global_volatility: float = 0.02,
    global_confidence: float | None = 0.72,
    chip_summary: dict | None = None,
    candle_high: float | None = None,
    candle_low: float | None = None,
    candle_close: float | None = None,
) -> dict:
    return build_decision_summary(
        zones,
        current_price,
        global_trend,
        global_volatility,
        {"confidence": global_confidence, "expected_value": 0.01, "risk_reward_ratio": 1.5},
        chip_summary or {"missing": False, "score": 55.0, "signal": "BULLISH"},
        _bundle(),
        candle_high=candle_high,
        candle_low=candle_low,
        candle_close=candle_close,
    )


def test_buy_for_bullish_high_quality_near_support():
    ds = _summary([_zone()])

    assert ds["action"] == "Buy"
    assert ds["market_action"] == "BUY"
    assert ds["position_action"] == "HOLD"
    assert ds["primary_zone"]["role"] == ZoneType.SUPPORT.value
    assert ds["primary_zone"]["label"] == "98.00 ~ 100.00"


def test_high_volatility_downgrades_buy_to_buy_small():
    ds = _summary([_zone()], global_volatility=0.04)

    assert ds["action"] == "BuySmall"
    assert ds["market_action"] == "BUY_SMALL"
    assert any("波動偏高" in note for note in ds["risk_notes"])


def test_bearish_regime_with_resistance_primary_is_avoid():
    resistance = _zone(role=ZoneType.RESISTANCE.value, low=104.0, high=106.0)

    ds = _summary([resistance], global_trend=-0.03)

    assert ds["action"] == "Avoid"
    assert ds["market_action"] == "AVOID"
    assert ds["position_action"] == "REDUCE"
    assert ds["primary_zone"]["role"] == ZoneType.RESISTANCE.value


def test_no_clear_primary_zone_holds():
    at_zone = _zone(role=ZoneType.AT_ZONE.value, low=101.0, high=103.0, expected_value=None, risk_reward_ratio=None)

    ds = _summary([at_zone])

    assert ds["action"] == "Hold"
    assert ds["market_action"] == "WATCH"
    assert ds["primary_zone"] is None
    assert any("沒有足夠明確" in note for note in ds["risk_notes"])


def test_expired_and_low_confidence_zones_are_not_first_candidate():
    expired = _zone(low=98.0, high=100.0, trading_score=95.0, recent_validation=RecentValidation.EXPIRED.value)
    low_confidence = _zone(low=97.0, high=99.0, trading_score=90.0, confidence=0.2, confidence_level=ConfidenceLevel.LOW.value)
    valid = _zone(low=94.0, high=96.0, trading_score=70.0)

    ds = _summary([expired, low_confidence, valid])

    assert ds["primary_zone"]["price_low"] == valid.price_low
    assert ds["primary_zone"]["recent_validation"] != RecentValidation.EXPIRED.value


def test_missing_ev_rr_cannot_be_buy():
    incomplete = _zone(expected_value=None, risk_reward_ratio=None)

    ds = _summary([incomplete])

    assert ds["action"] != "Buy"
    assert ds["market_action"] == "WATCH"


def test_rr_below_hard_gate_stays_watch_even_with_high_score_and_ev():
    zone = _zone(trading_score=95.0, confidence=0.9, expected_value=0.08, risk_reward_ratio=1.49)

    ds = _summary([zone])

    assert ds["market_action"] == "WATCH"
    assert ds["action"] == "Hold"
    assert any("風險報酬比不足" in note for note in ds["risk_notes"])


def test_rr_between_gates_allows_only_buy_small():
    zone = _zone(trading_score=95.0, confidence=0.9, expected_value=0.08, risk_reward_ratio=1.8)

    ds = _summary([zone])

    assert ds["market_action"] == "BUY_SMALL"
    assert ds["action"] == "BuySmall"
    assert any("完整買進門檻" in note for note in ds["risk_notes"])


def test_recovery_invalidated_overrides_long_term_bullish_regime():
    zone = _zone(low=98.0, high=100.0, risk_reward_ratio=2.5)

    ds = _summary([zone], current_price=97.5, candle_high=100.5, candle_low=97.0, candle_close=97.5)

    assert ds["market_regime"]["trend_regime"] == "TREND_UP"
    assert ds["market_regime"]["structure_state"] == "RECOVERY_INVALIDATED"
    assert "長期偏多，但短線結構轉弱" in ds["market_regime"]["label"]
    assert ds["market_action"] == "AVOID"
    assert ds["position_action"] == "REDUCE_ON_BREAKDOWN"


def test_zone_interaction_uses_intraday_high_low_close_not_only_current_price():
    zone = _zone(low=98.0, high=100.0, risk_reward_ratio=2.5)

    ds = _summary([zone], current_price=101.0, candle_high=101.5, candle_low=99.0, candle_close=101.0)

    interaction = ds["primary_zone"]["zone_interaction"]
    assert interaction["touched"] is True
    assert interaction["closed_above"] is True
    assert interaction["closed_inside"] is False
    assert interaction["state_label"] == "收回區間上方"


def test_chip_missing_is_exposed_in_context():
    ds = _summary([_zone()], chip_summary={"missing": True})

    assert any(item.get("key") == "chip" and item.get("effect") == "warning" for item in ds["market_context"])
    assert any("籌碼資料缺漏" in reason for reason in ds["market_regime"]["reasons"])
