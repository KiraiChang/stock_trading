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
    relative_volume: float = 1.2,
    volume_confirmation: str = VolumeConfirmation.CONFIRMED.value,
    tier: str = ZoneTier.TIER_1_MAIN_STRUCTURE.value,
    tier_label: str = "主結構",
) -> ZoneScore:
    return ZoneScore(
        price_low=low,
        price_high=high,
        method="atr",
        role=role,
        tier=tier,
        tier_label=tier_label,
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
        relative_volume=relative_volume if role != ZoneType.AT_ZONE.value else None,
        volume_confirmation=volume_confirmation if role != ZoneType.AT_ZONE.value else None,
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
    assert ds["primary_zone"]["entry_relevance_score"] >= 75
    assert ds["primary_zone"]["zone_quality_score"] == ds["primary_zone"]["trading_score"]


def test_high_quality_far_from_zone_cannot_be_buy():
    far = _zone(low=80.0, high=82.0, trading_score=98.0, confidence=0.9, risk_reward_ratio=3.0)

    ds = _summary([far], current_price=110.0)

    assert ds["action"] != "Buy"
    assert ds["market_action"] == "BUY_SMALL"
    assert ds["primary_zone"]["entry_relevance_score"] < 75
    assert any("不適合追價" in note for note in ds["risk_notes"])


def test_primary_high_volume_breakdown_event_forces_exit():
    zone = _zone(
        low=98.0,
        high=100.0,
        risk_reward_ratio=2.5,
        relative_volume=2.2,
        volume_confirmation=VolumeConfirmation.FAILED.value,
    )

    ds = _summary([zone], current_price=97.5, candle_high=100.5, candle_low=97.0, candle_close=97.5)

    assert ds["market_events"][0]["type"] == "HIGH_VOLUME_BREAKDOWN"
    assert ds["market_action"] == "AVOID"
    assert ds["position_action"] == "EXIT"
    # 對外回報的 entry_relevance 是 base 值，不把市場事件修正灌進同名分數／breakdown，
    # 才能跟 zones[].entry_relevance_score 保持同定義（見 decision_engine 說明）。
    assert "market_event" not in ds["primary_zone"]["entry_relevance_breakdown"]


def test_extreme_volume_outputs_context_event_without_direct_action_override():
    zone = _zone(relative_volume=2.8, volume_confirmation=VolumeConfirmation.CONFIRMED.value)

    ds = _summary([zone])

    assert ds["market_events"][0]["type"] == "EXTREME_VOLUME"
    assert ds["market_action"] == "BUY"


def test_short_term_non_primary_high_volume_breakdown_reduces_without_exit():
    main = _zone(low=80.0, high=82.0, trading_score=70.0, risk_reward_ratio=2.5)
    short = _zone(
        low=90.0,
        high=91.0,
        trading_score=95.0,
        confidence=0.2,
        confidence_level=ConfidenceLevel.LOW.value,
        risk_reward_ratio=2.5,
        relative_volume=2.2,
        volume_confirmation=VolumeConfirmation.FAILED.value,
        tier=ZoneTier.TIER_3_SHORT_TERM.value,
        tier_label="短期支撐/壓力",
    )

    ds = _summary([main, short], current_price=89.0, candle_high=91.5, candle_low=88.5, candle_close=89.0)

    assert any(event["type"] == "HIGH_VOLUME_BREAKDOWN" for event in ds["market_events"])
    assert ds["primary_zone"]["price_low"] == main.price_low
    assert ds["market_action"] == "AVOID"
    assert ds["position_action"] == "REDUCE_ON_BREAKDOWN"


def test_pending_validation_buy_small_is_probe_entry_not_confirmed_small_entry():
    zone = _zone(
        trading_score=95.0,
        confidence=0.9,
        expected_value=0.08,
        risk_reward_ratio=1.8,
        recent_validation=RecentValidation.PENDING_VALIDATION.value,
    )

    ds = _summary([zone])

    assert ds["action"] == "BuySmall"
    assert ds["entry_action_state"] == "PROBE_ENTRY"
    assert ds["entry_action_label"] == "觀察性試探"


def test_confirmed_buy_small_is_small_entry():
    zone = _zone(trading_score=95.0, confidence=0.9, expected_value=0.08, risk_reward_ratio=1.8)

    ds = _summary([zone])

    assert ds["action"] == "BuySmall"
    assert ds["entry_action_state"] == "SMALL_ENTRY"


def test_high_volatility_downgrades_buy_to_buy_small():
    ds = _summary([_zone()], global_volatility=0.04)

    assert ds["action"] == "BuySmall"
    assert ds["market_action"] == "BUY_SMALL"
    assert any("波動偏高" in note for note in ds["risk_notes"])
    assert ds["market_regime"]["structural_trend"] == "TREND_UP"
    assert ds["market_regime"]["short_term_regime"] == "NORMAL"
    assert set(ds["defense_lines"].keys()) == {"tactical", "swing", "strategic"}


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


def test_primary_zone_ranking_prefers_near_relevant_zone_over_far_high_quality():
    near = _zone(low=98.0, high=100.0, trading_score=62.0, confidence=0.62, risk_reward_ratio=2.0)
    far = _zone(low=70.0, high=72.0, trading_score=99.0, confidence=0.95, risk_reward_ratio=3.0)

    ds = _summary([far, near], current_price=102.0)

    assert ds["primary_zone"]["price_low"] == near.price_low


def test_primary_zone_ranking_does_not_choose_direction_mismatch_on_quality_alone():
    aligned_support = _zone(low=98.0, high=100.0, trading_score=60.0, confidence=0.62, risk_reward_ratio=2.0)
    mismatched_resistance = _zone(
        role=ZoneType.RESISTANCE.value,
        low=116.0,
        high=118.0,
        trading_score=99.0,
        confidence=0.95,
        risk_reward_ratio=3.0,
    )

    ds = _summary([mismatched_resistance, aligned_support], current_price=102.0, global_trend=0.03)

    assert ds["primary_zone"]["role"] == ZoneType.SUPPORT.value


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
    assert ds["market_regime"]["structure_state"] == "SUPPORT_RECLAIM_INVALIDATED"
    assert "長期偏多，但支撐收復失效" in ds["market_regime"]["label"]
    assert ds["market_action"] == "AVOID"
    assert ds["position_action"] == "REDUCE_ON_BREAKDOWN"
    assert ds["position_action_condition"]["invalidation_price"] == 98.0
    assert "SUPPORT_BREAKDOWN_RISK" in ds["position_action_condition"]["reason_codes"]


def test_zone_interaction_uses_intraday_high_low_close_not_only_current_price():
    zone = _zone(low=98.0, high=100.0, risk_reward_ratio=2.5)

    ds = _summary([zone], current_price=101.0, candle_high=101.5, candle_low=99.0, candle_close=101.0)

    interaction = ds["primary_zone"]["zone_interaction"]
    assert interaction["touched"] is True
    assert interaction["closed_above"] is True
    assert interaction["closed_inside"] is False
    assert interaction["state_label"] == "收回區間上方"
    assert ds["market_regime"]["structure_state"] == "SUPPORT_RECLAIM_CANDIDATE"
    assert ds["position_action_condition"]["recovery_price"] == 100.0


def test_support_reclaim_confirmed_requires_following_bar_not_breaking_back_down():
    zone = _zone(low=98.0, high=100.0, risk_reward_ratio=2.5)

    ds = _summary(
        [zone],
        current_price=101.0,
        candle_high=102.0,
        candle_low=100.5,
        candle_close=101.0,
        # build_decision_summary callers pass these directly in production.
    )

    assert ds["market_regime"]["structure_state"] == "NORMAL"

    ds = build_decision_summary(
        [zone],
        101.0,
        0.03,
        0.02,
        {"confidence": 0.72, "expected_value": 0.01, "risk_reward_ratio": 1.5},
        {"missing": False, "score": 55.0, "signal": "BULLISH"},
        _bundle(),
        candle_high=102.0,
        candle_low=100.5,
        candle_close=101.0,
        previous_candle_high=101.0,
        previous_candle_low=99.0,
        previous_candle_close=100.5,
    )

    assert ds["market_regime"]["structure_state"] == "SUPPORT_RECLAIM_CONFIRMED"
    assert "SUPPORT_RECLAIM_CONFIRMED" in ds["position_action_condition"]["reason_codes"]


def test_chip_missing_is_exposed_in_context():
    ds = _summary([_zone()], chip_summary={"missing": True})

    assert any(item.get("key") == "chip" and item.get("effect") == "warning" for item in ds["market_context"])
    assert any("籌碼資料缺漏" in reason for reason in ds["market_regime"]["reasons"])
