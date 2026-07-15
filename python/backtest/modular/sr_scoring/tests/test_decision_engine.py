from __future__ import annotations

from ..decision_engine import _final_entry_permission, build_decision_summary
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
    candle_open: float | None = None,
    candle_high: float | None = None,
    candle_low: float | None = None,
    candle_close: float | None = None,
    previous_candle_open: float | None = None,
    previous_candle_high: float | None = None,
    previous_candle_low: float | None = None,
    previous_candle_close: float | None = None,
    data_quality_metadata: dict | None = None,
) -> dict:
    return build_decision_summary(
        zones,
        current_price,
        global_trend,
        global_volatility,
        {"confidence": global_confidence, "expected_value": 0.01, "risk_reward_ratio": 1.5},
        chip_summary or {"missing": False, "score": 55.0, "signal": "BULLISH"},
        _bundle(),
        candle_open=candle_open,
        candle_high=candle_high,
        candle_low=candle_low,
        candle_close=candle_close,
        previous_candle_open=previous_candle_open,
        previous_candle_high=previous_candle_high,
        previous_candle_low=previous_candle_low,
        previous_candle_close=previous_candle_close,
        data_quality_metadata=data_quality_metadata,
    )


def test_buy_for_bullish_high_quality_near_support():
    ds = _summary([_zone()])

    assert ds["action"] == "Buy"
    assert ds["market_action"] == "BUY"
    assert ds["market_bias"] == "BULLISH_BIAS"
    assert ds["market_bias_label"] == "偏多觀察"
    assert ds["position_action"] == "HOLD"
    assert ds["primary_zone"]["role"] == ZoneType.SUPPORT.value
    assert ds["primary_zone"]["label"] == "98.00 ~ 100.00"
    assert ds["primary_zone"]["entry_relevance_score"] >= 75
    assert ds["primary_zone"]["zone_quality_score"] == ds["primary_zone"]["trading_score"]
    assert ds["primary_zone"]["structural_score"] == ds["primary_zone"]["zone_quality_score"]
    assert ds["primary_zone"]["decision_relevance_score"] == ds["primary_zone"]["entry_relevance_score"]
    assert ds["primary_zone"]["tradability_score"] == ds["primary_zone"]["trading_score"]
    assert ds["rr_gate"]["qualified"] is True
    assert ds["best_trade_zone"]["label"] == "98.00 ~ 100.00"
    assert ds["nearest_decision_zone"]["label"] == "98.00 ~ 100.00"
    assert ds["primary_zone"]["source"] == "HISTORICAL_SR"
    assert ds["primary_zone"]["decision_role"] == "PRIMARY"
    assert ds["market_regime"]["tactical_regime"] == ds["market_regime"]["short_term_regime"]
    assert ds["market_regime"]["recovery_state"] == ds["market_regime"]["structure_state"]


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
    assert ds["market_action"] == "WATCH"
    assert ds["action"] == "Hold"
    assert ds["position_action"] == "REDUCE_ON_BREAKDOWN"


def test_minor_high_volume_breakdown_only_adds_risk_note_without_forced_action():
    main = _zone(low=80.0, high=82.0, trading_score=70.0, risk_reward_ratio=2.5)
    short = _zone(
        low=90.0,
        high=91.0,
        trading_score=35.0,
        confidence=0.2,
        confidence_level=ConfidenceLevel.LOW.value,
        expected_value=-0.01,
        risk_reward_ratio=0.8,
        recent_validation=RecentValidation.EXPIRED.value,
        relative_volume=2.2,
        volume_confirmation=VolumeConfirmation.FAILED.value,
        tier=ZoneTier.TIER_3_SHORT_TERM.value,
        tier_label="短期支撐/壓力",
    )

    ds = _summary([main, short], current_price=89.0, candle_high=91.5, candle_low=88.5, candle_close=89.0)

    assert any(event["type"] == "HIGH_VOLUME_BREAKDOWN" for event in ds["market_events"])
    assert ds["primary_zone"]["price_low"] == main.price_low
    assert ds["market_action"] != "AVOID"
    assert ds["position_action"] != "EXIT"
    assert any("尚未達主結構防守門檻" in note for note in ds["risk_notes"])


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
    assert ds["final_entry_permission"]["state"] == "PROBE_ENTRY"


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


def test_zone_lifecycle_outputs_supported_states():
    expired = _summary([_zone(recent_validation=RecentValidation.EXPIRED.value)])
    weak = _summary([_zone(confidence=0.2, confidence_level=ConfidenceLevel.LOW.value)])
    confirmed = _summary(
        [_zone()],
        current_price=101.0,
        candle_high=101.5,
        candle_low=99.0,
        candle_close=101.0,
    )
    pending = _summary([_zone(recent_validation=RecentValidation.PENDING_VALIDATION.value)])

    assert expired["primary_zone"]["lifecycle"] == "INVALIDATED"
    assert weak["primary_zone"]["lifecycle"] == "WEAKENING"
    assert confirmed["primary_zone"]["lifecycle"] == "CONFIRMED"
    assert pending["primary_zone"]["lifecycle"] == "CANDIDATE"


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
    assert ds["rr_gate"]["qualified"] is False
    assert ds["rr_gate"]["reason_code"] == "RR_INSUFFICIENT"
    assert ds["best_trade_zone"] is None
    assert ds["daily_confirmation"]["state"] == "NO_SETUP"
    assert ds["price_path"]["path_state"] == "RR_BLOCKED"
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
    assert ds["price_path"]["path_state"] == "INVALIDATION_RISK"
    assert ds["price_path"]["invalidation_price"] == 98.0
    assert ds["daily_confirmation"]["state"] == "INVALIDATED"


def test_recovery_confirmed_outputs_recovery_regime_and_final_permission():
    zone = _zone(low=98.0, high=100.0, risk_reward_ratio=2.5)

    ds = _summary(
        [zone],
        current_price=101.0,
        candle_high=102.0,
        candle_low=100.5,
        candle_close=101.0,
        previous_candle_high=101.0,
        previous_candle_low=99.0,
        previous_candle_close=100.5,
    )

    assert ds["market_regime"]["recovery_state"] == "RECOVERY"
    assert ds["market_regime"]["short_term_regime"] == "RECOVERY"
    assert ds["market_bias"] == "BULLISH_CONTINUATION"
    assert ds["final_entry_permission"]["state"] in ("ACCUMULATE", "BUY")


def test_early_trend_outputs_bullish_continuation_bias():
    zone = _zone(low=98.0, high=100.0, risk_reward_ratio=2.5)

    ds = _summary([zone], current_price=102.0, global_trend=0.01, global_confidence=0.55)

    assert ds["market_regime"]["trend_regime"] == "RANGE_BOUND"
    assert ds["market_regime"]["short_term_regime"] == "EARLY_TREND"
    assert ds["market_bias"] == "BULLISH_CONTINUATION"


def test_recovery_regime_does_not_force_bullish_continuation_when_action_avoids():
    # 長期偏空但短線收復確認：short_term_regime=RECOVERY，market_action 仍可能為 AVOID。
    # market_bias 不得因 RECOVERY 就標成多頭延續，需與 action 語意一致（偏空）。
    zone = _zone(low=98.0, high=100.0, risk_reward_ratio=2.5)

    ds = _summary(
        [zone],
        global_trend=-0.03,
        current_price=101.0,
        candle_high=102.0,
        candle_low=100.5,
        candle_close=101.0,
        previous_candle_high=101.0,
        previous_candle_low=99.0,
        previous_candle_close=100.5,
    )

    assert ds["market_regime"]["short_term_regime"] == "RECOVERY"
    assert ds["market_action"] == "AVOID"
    assert ds["market_bias"] != "BULLISH_CONTINUATION"
    assert ds["market_bias"] == "BEARISH_BIAS"


def test_final_entry_permission_keeps_invalidated_distinct_from_waiting():
    permission = _final_entry_permission("BUY", {"state": "INVALIDATED", "reason_codes": ["SUPPORT_CLOSED_BELOW"]})

    assert permission["state"] == "NO_SETUP"
    assert permission["label"] == "無設定"
    assert permission["daily_confirmation_state"] == "INVALIDATED"
    assert permission["reason_codes"] == ["SUPPORT_CLOSED_BELOW"]


def test_final_entry_permission_does_not_upgrade_buy_without_daily_buy_ready():
    permission = _final_entry_permission("BUY", {"state": "ENTRY_READY", "reason_codes": ["ENTRY_STATE_READY"]})

    assert permission["state"] == "ACCUMULATE"
    assert permission["entry_action_state"] == "BUY"
    assert permission["daily_confirmation_state"] == "ENTRY_READY"


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


def test_confirmed_reclaim_clears_same_zone_breakdown_exit_gate():
    zone = _zone(
        low=98.0,
        high=100.0,
        risk_reward_ratio=2.5,
        relative_volume=2.2,
        volume_confirmation=VolumeConfirmation.FAILED.value,
    )

    ds = _summary(
        [zone],
        current_price=101.0,
        candle_high=102.0,
        candle_low=97.0,
        candle_close=101.0,
        previous_candle_high=101.0,
        previous_candle_low=97.5,
        previous_candle_close=100.5,
    )

    assert ds["market_regime"]["structure_state"] == "SUPPORT_RECLAIM_CONFIRMED"
    assert ds["market_action"] != "AVOID"
    assert ds["position_action"] != "EXIT"
    assert ds["position_action_condition"]["state"] == "SUPPORT_RECLAIM_CONFIRMED"
    assert "SUPPORT_RECLAIM_CONFIRMED" in ds["position_action_condition"]["reason_codes"]


def test_reclaim_evidence_is_consistent_for_reclaimed_zone():
    zone = _zone(low=28.06, high=28.37, risk_reward_ratio=2.5)

    ds = _summary(
        [zone],
        current_price=28.5,
        candle_high=28.65,
        candle_low=27.95,
        candle_close=28.5,
    )

    interaction = ds["primary_zone"]["zone_interaction"]
    assert interaction["touched"] is True
    assert interaction["closed_above"] is True
    assert interaction["state_label"] == "收回區間上方"
    assert ds["primary_zone"]["recent_validation"] != RecentValidation.PENDING_VALIDATION.value
    assert ds["primary_zone"]["lifecycle"] == "CONFIRMED"
    assert ds["market_regime"]["structure_state"] == "SUPPORT_RECLAIM_CANDIDATE"


def test_chip_missing_is_exposed_in_context():
    ds = _summary([_zone()], chip_summary={"missing": True})

    assert any(item.get("key") == "chip" and item.get("effect") == "warning" for item in ds["market_context"])
    assert any("籌碼資料缺漏" in reason for reason in ds["market_regime"]["reasons"])
    assert ds["data_mode"] == "END_OF_DAY"
    assert ds["data_quality"]["chip_coverage"] == 0.0
    assert "chip" in ds["data_quality"]["missing_features"]
    assert ds["data_quality"]["features"]["chip"]["status"] == "MISSING"
    assert ds["data_quality"]["features"]["chip"]["interpretation"] == "UNAVAILABLE"


def test_structural_and_nearest_zones_are_split_from_best_trade_zone():
    near = _zone(
        low=100.5,
        high=101.0,
        trading_score=62.0,
        confidence=0.58,
        risk_reward_ratio=1.6,
        tier=ZoneTier.TIER_2_TRADING_ZONE.value,
        tier_label="交易區",
    )
    structural = _zone(low=88.0, high=90.0, trading_score=96.0, confidence=0.9, risk_reward_ratio=3.0)

    ds = _summary([structural, near], current_price=102.0)

    assert ds["nearest_decision_zone"]["price_low"] == near.price_low
    assert ds["primary_structural_zone"]["price_low"] == structural.price_low
    assert ds["best_trade_zone"] is None
    assert ds["rr_gate"]["qualified"] is False
    assert ds["rr_gate"]["reason_code"] == "RR_INSUFFICIENT"


def test_event_sequence_keeps_break_reclaim_reversal_order():
    zone = _zone(
        low=98.0,
        high=100.0,
        risk_reward_ratio=2.5,
        relative_volume=3.0,
        volume_confirmation=VolumeConfirmation.FAILED.value,
    )

    ds = _summary(
        [zone],
        current_price=101.0,
        candle_high=102.0,
        candle_low=97.0,
        candle_close=101.0,
    )

    assert [item["type"] for item in ds["event_sequence"]] == [
        "EXTREME_VOLUME",
        "HIGH_VOLUME_BREAKDOWN",
        "INTRADAY_RECLAIM",
        "REVERSAL_CANDIDATE",
    ]
    assert [item["label"] for item in ds["event_sequence"]] == ["極端量能", "放量破位", "收盤收復", "反轉候選"]


def test_daily_price_action_outputs_eod_bar_states():
    ds = _summary(
        [_zone()],
        current_price=105.0,
        candle_high=106.0,
        candle_low=100.0,
        candle_close=105.0,
        previous_candle_high=103.0,
        previous_candle_low=98.0,
        previous_candle_close=102.0,
    )

    action = ds["daily_price_action"]
    assert action["available"] is True
    assert action["close_location_state"] == "CLOSE_NEAR_HIGH"
    assert action["range_state"] == "RANGE_EXPANSION"
    assert action["follow_through_state"] == "UPSIDE_FOLLOW_THROUGH"
    assert action["price_follow_through_state"] == "PRICE_UPSIDE_FOLLOW_THROUGH"
    assert action["momentum_confirmation_state"] == "MOMENTUM_CONFIRMED"
    assert action["reclaim_rejection_state"] == "PREVIOUS_CLOSE_RECLAIM"
    assert action["body_proxy_ratio"] == 0.5
    assert action["body_ratio"] == action["body_proxy_ratio"]
    assert action["body_ratio_source"] == "PREVIOUS_CLOSE_PROXY"
    assert action["lower_wick_ratio"] == 0.3333
    assert action["upper_wick_ratio"] == 0.1667


def test_daily_price_action_uses_daily_open_for_body_ratio_when_available():
    ds = _summary(
        [_zone()],
        current_price=105.0,
        candle_open=101.0,
        candle_high=106.0,
        candle_low=100.0,
        candle_close=105.0,
        previous_candle_open=100.0,
        previous_candle_close=102.0,
    )

    action = ds["daily_price_action"]
    assert action["body_proxy_ratio"] == 0.5
    assert action["body_ratio"] == 0.6667
    assert action["body_ratio_source"] == "DAILY_OPEN"
    assert action["reference_prices"]["open"] == 101.0
    assert action["reference_prices"]["previous_open"] == 100.0


def test_daily_candidate_zones_are_tactical_candidates_not_best_trade_zone():
    ds = _summary(
        [],
        current_price=104.0,
        candle_high=105.0,
        candle_low=100.0,
        candle_close=104.0,
        previous_candle_close=102.0,
    )

    assert ds["primary_zone"] is None
    assert ds["best_trade_zone"] is None
    assert [z["role"] for z in ds["daily_candidate_zones"]] == [ZoneType.SUPPORT.value, ZoneType.RESISTANCE.value]
    assert {z["source"] for z in ds["daily_candidate_zones"]} == {"DAILY_CANDLE"}
    assert {z["lifecycle"] for z in ds["daily_candidate_zones"]} == {"CANDIDATE"}
    assert ds["daily_confirmation"]["state"] == "WAIT_DAILY_CONFIRM"
    assert ds["daily_entry_state"] == "WAIT_DAILY_CONFIRM"
    assert ds["price_path"]["path_state"] == "DAILY_CANDIDATE_ONLY"


def test_price_path_reports_blocking_zone_and_next_decision_price():
    support = _zone(low=98.0, high=100.0, risk_reward_ratio=2.5)
    resistance = _zone(role=ZoneType.RESISTANCE.value, low=106.0, high=108.0, risk_reward_ratio=2.0)

    ds = _summary([support, resistance], current_price=102.0)

    assert ds["price_path"]["next_decision_price"] == 100.0
    assert ds["price_path"]["next_decision_source"] == "nearest_support_zone"
    assert ds["price_path"]["blocking_zone"]["label"] == "106.00 ~ 108.00"
    assert any(item["then"] == "RECHECK_ENTRY_STATE" for item in ds["price_path"]["transitions"])


def test_nearest_support_and_resistance_are_split_for_price_path():
    support = _zone(low=98.0, high=100.0, risk_reward_ratio=2.5)
    resistance = _zone(role=ZoneType.RESISTANCE.value, low=106.0, high=108.0, risk_reward_ratio=2.0)

    ds = _summary([support, resistance], current_price=102.0)

    assert ds["nearest_support_zone"]["label"] == "98.00 ~ 100.00"
    assert ds["nearest_resistance_zone"]["label"] == "106.00 ~ 108.00"
    assert ds["price_path"]["next_decision_source"] == "nearest_support_zone"
    assert ds["price_path"]["next_decision_price"] == 100.0


def test_wide_zone_penalty_prevents_broad_zone_from_winning_nearest_decision():
    narrow = _zone(low=96.0, high=97.0, trading_score=70.0, risk_reward_ratio=2.5)
    wide = _zone(low=95.0, high=101.0, trading_score=95.0, risk_reward_ratio=2.5)

    ds = _summary([wide, narrow], current_price=102.0)

    assert ds["nearest_support_zone"]["price_low"] == narrow.price_low
    assert ds["nearest_decision_zone"]["price_low"] == narrow.price_low
    assert ds["secondary_zones"][0]["zone_interaction"]["distance_pct"] >= 0.0


def test_rr_context_and_completeness_layers_are_split():
    zone = _zone(risk_reward_ratio=2.4)

    ds = _summary([zone])

    assert ds["rr_context"]["entry_rr"] == 2.4
    assert ds["rr_context"]["position_rr"] is None
    assert ds["rr_context"]["entry_rr_source"] == "PRIMARY_ZONE"
    assert ds["rr_context"]["position_rr_source"] == "UNAVAILABLE"
    quality = ds["data_quality"]
    assert quality["market_data_completeness"] == quality["overall_completeness"]
    assert quality["rr_completeness"] == 1.0
    assert quality["trade_qualification_completeness"] == 1.0


def test_data_quality_separates_missing_neutral_and_negative_features():
    zone = _zone(expected_value=-0.01, risk_reward_ratio=1.2)

    neutral = _summary([zone], global_confidence=0.3, chip_summary={"missing": False, "score": 0.0})

    assert "chip" in neutral["data_quality"]["neutral_features"]
    assert "global_confidence" in neutral["data_quality"]["negative_features"]
    assert "expected_value" in neutral["data_quality"]["negative_features"]
    assert "risk_reward_ratio" in neutral["data_quality"]["negative_features"]
    assert "daily_price" in neutral["data_quality"]["unavailable_features"]

    positive = _summary([zone], chip_summary={"missing": False, "score": 50.0})
    negative = _summary([zone], chip_summary={"missing": False, "score": -50.0})

    assert "chip" in positive["data_quality"]["positive_features"]
    assert positive["data_quality"]["features"]["chip"]["interpretation"] == "POSITIVE"
    assert "chip" in negative["data_quality"]["negative_features"]
    assert negative["data_quality"]["features"]["chip"]["interpretation"] == "NEGATIVE"


def test_data_quality_marks_stale_features_from_updated_at_metadata():
    ds = _summary(
        [_zone()],
        candle_high=106.0,
        candle_low=100.0,
        candle_close=105.0,
        previous_candle_close=102.0,
        data_quality_metadata={
            "analysis_as_of": "2026-07-15",
            "stale_after_days": 1,
            "updated_at": {
                "daily_price": "2026-07-13",
                "previous_daily_price": "2026-07-14",
                "chip": "2026-07-12",
            },
        },
    )

    quality = ds["data_quality"]
    assert quality["features"]["daily_price"]["status"] == "STALE"
    assert quality["features"]["previous_daily_price"]["status"] == "AVAILABLE"
    assert quality["features"]["chip"]["status"] == "STALE"
    assert set(quality["stale_features"]) == {"daily_price", "chip"}
    assert quality["features"]["daily_price"]["reason_codes"] == ["DATA_STALE"]


def test_data_quality_marks_invalid_ohlc_and_validation_errors():
    ds = _summary(
        [_zone()],
        candle_high=100.0,
        candle_low=106.0,
        candle_close=105.0,
        previous_candle_close=102.0,
        data_quality_metadata={
            "validation_errors": {
                "chip": ["SCORE_OUT_OF_RANGE"],
            },
        },
    )

    quality = ds["data_quality"]
    assert quality["features"]["daily_price"]["status"] == "INVALID"
    assert "HIGH_BELOW_LOW" in quality["features"]["daily_price"]["reason_codes"]
    assert quality["features"]["chip"]["status"] == "INVALID"
    assert quality["features"]["chip"]["reason_codes"] == ["SCORE_OUT_OF_RANGE"]
    assert set(quality["invalid_features"]) == {"daily_price", "chip"}
    assert quality["price_data_complete"] is False
