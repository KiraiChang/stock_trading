from __future__ import annotations

import pytest

from ..decision_engine import (
    TACTICAL_SOURCE_EVENT,
    TACTICAL_SOURCE_FALLBACK,
    _daily_confirmation,
    _decision_action,
    _decision_derived_view,
    _defense_lines,
    _final_action_from_entry,
    _final_entry_permission,
    _final_entry_risk_notes,
    _market_regime,
    _position_action_condition,
    _risk_note,
    _risk_note_code,
    _risk_note_text,
    _structure_state,
    _zone_interaction,
    build_decision_summary,
)
from ..event_engine import detect_market_events
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
    current_price: float = 100.1,
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
    model_governance: dict | None = None,
    previous_event_states: list[dict] | None = None,
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
        model_governance=model_governance,
        previous_event_states=previous_event_states,
    )


def test_buy_for_bullish_high_quality_near_support():
    ds = _summary([_zone()])

    assert ds["action"] == "Hold"
    assert ds["market_action"] == "WATCH"
    assert ds["entry_action_state"] == "ACCUMULATE"
    assert ds["final_entry_permission"]["state"] == "WAIT_CONFIRMATION"
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
    assert ds["primary_zone"]["role_label"] == "支撐"
    assert ds["primary_zone"]["display_label"] == "主結構支撐"
    assert ds["rr_gate"]["qualified"] is True
    assert ds["best_trade_zone"] is None
    assert ds["nearest_decision_zone"]["label"] == "98.00 ~ 100.00"
    assert ds["primary_zone"]["source"] == "HISTORICAL_SR"
    assert ds["decision_contract"]["version"] == "sr-zone-decision-p0"
    assert "final_entry_permission" in ds["decision_contract"]["authoritative_fields"]
    assert "action" in ds["decision_contract"]["deprecated_fields"]
    assert ds["primary_zone"]["decision_role"] == "PRIMARY"
    assert ds["market_regime"]["tactical_regime"] == ds["market_regime"]["short_term_regime"]
    assert ds["market_regime"]["recovery_state"] == ds["market_regime"]["structure_state"]


def test_unreliable_model_governance_blocks_entry():
    ds = _summary(
        [_zone()],
        model_governance={
            "health_state": "UNRELIABLE",
            "blocking_flags": ["HOLD_LOW_TEST_ROWS"],
            "warning_flags": [],
            "confidence_gate": {
                "state": "UNRELIABLE",
                "allow_entry": False,
                "max_entry_state": "WAIT_CONFIRMATION",
                "reason_codes": ["HOLD_LOW_TEST_ROWS"],
            },
        },
    )

    assert ds["model_governance"]["health_state"] == "UNRELIABLE"
    assert "MODEL_UNRELIABLE" in ds["market_regime"]["flags"]
    assert ds["action"] == "Hold"
    assert ds["market_action"] == "WATCH"
    assert ds["final_entry_permission"]["state"] == "WAIT_CONFIRMATION"
    assert any("模型健康度不可用" in note for note in ds["risk_notes"])


def test_degraded_model_governance_downgrades_strong_buy_to_small_entry():
    ds = _summary(
        [_zone()],
        model_governance={
            "health_state": "DEGRADED",
            "blocking_flags": [],
            "warning_flags": ["HOLD_NOT_CALIBRATED"],
            "confidence_gate": {
                "state": "DEGRADED",
                "allow_entry": True,
                "max_entry_state": "SMALL_ENTRY",
                "reason_codes": ["HOLD_NOT_CALIBRATED"],
            },
        },
    )

    assert ds["model_governance"]["health_state"] == "DEGRADED"
    assert "MODEL_DEGRADED" in ds["market_regime"]["flags"]
    assert ds["action"] == "Hold"
    assert ds["market_action"] == "WATCH"
    assert ds["entry_action_state"] in ("PROBE_ENTRY", "SMALL_ENTRY")
    assert any("模型健康度降級" in note for note in ds["risk_notes"])


def test_high_quality_far_from_zone_cannot_be_buy():
    far = _zone(low=80.0, high=82.0, trading_score=98.0, confidence=0.9, risk_reward_ratio=3.0)

    ds = _summary([far], current_price=110.0)

    assert ds["action"] != "Buy"
    assert ds["market_action"] == "WATCH"
    assert ds["entry_executability"]["executable_now"] is False
    assert ds["final_entry_permission"]["state"] == "WAIT_CONFIRMATION"
    assert ds["primary_zone"]["entry_relevance_score"] < 75
    assert any("不適合追價" in note for note in ds["risk_notes"])


def test_entry_zone_overshot_blocks_best_trade_zone_and_downgrades_action():
    zone = _zone(low=98.0, high=100.0, risk_reward_ratio=2.8, confidence=0.82, trading_score=92.0)

    ds = _summary([zone], current_price=102.0)

    assert ds["entry_action_state"] in ("ACCUMULATE", "BUY")
    assert ds["entry_executability"]["executable_now"] is False
    assert ds["entry_executability"]["reason_code"] == "ENTRY_ZONE_OVERSHOT"
    assert ds["final_entry_permission"]["state"] == "WAIT_CONFIRMATION"
    assert "ENTRY_ZONE_OVERSHOT" in ds["final_entry_permission"]["reason_codes"]
    assert ds["market_action"] == "WATCH"
    assert ds["action"] == "Hold"
    assert ds["best_trade_zone"] is None


def test_entry_zone_undershot_blocks_best_trade_zone_and_downgrades_action():
    zone = _zone(low=98.0, high=100.0, risk_reward_ratio=2.8, confidence=0.82, trading_score=92.0)

    ds = _summary([zone], current_price=97.0)

    assert ds["entry_executability"]["executable_now"] is False
    assert ds["entry_executability"]["reason_code"] == "ENTRY_ZONE_UNDERSHOT"
    assert ds["final_entry_permission"]["state"] in ("BLOCKED", "WAIT_CONFIRMATION")
    assert ds["best_trade_zone"] is None


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
    assert ds["event_state_summary"]["market_state"] == "BREAKDOWN_RISK"
    assert [event["type"] for event in ds["event_state_summary"]["active_bearish_events"]] == ["HIGH_VOLUME_BREAKDOWN"]
    assert ds["price_path"]["path_state"] == "EVENT_RISK"
    assert ds["price_path"]["blocked_by_event"]["type"] == "HIGH_VOLUME_BREAKDOWN"
    assert ds["market_action"] == "AVOID"
    assert ds["position_action"] == "EXIT"
    # 對外回報的 entry_relevance 是 base 值，不把市場事件修正灌進同名分數／breakdown，
    # 才能跟 zones[].entry_relevance_score 保持同定義（見 decision_engine 說明）。
    assert "market_event" not in ds["primary_zone"]["entry_relevance_breakdown"]


def test_previous_active_breakdown_carries_into_decision_gate_without_new_event():
    zone = _zone(low=98.0, high=100.0, risk_reward_ratio=2.5, relative_volume=1.0)
    previous = [{
        "type": "HIGH_VOLUME_BREAKDOWN",
        "event_family": "SUPPORT_BREAKDOWN",
        "event_scope": "ZONE",
        "zone_key": "SUPPORT:98.0000:100.0000",
        "root_event_type": "HIGH_VOLUME_BREAKDOWN",
        "latest_event_type": "HIGH_VOLUME_BREAKDOWN",
        "direction": "BEARISH",
        "state": "CONFIRMED",
        "active": True,
        "reason_codes": ["SUPPORT_CLOSED_BELOW"],
    }]

    ds = _summary(
        [zone],
        current_price=102.0,
        candle_high=103.0,
        candle_low=101.0,
        candle_close=102.0,
        previous_event_states=previous,
    )

    assert ds["market_events"] == []
    assert [event["type"] for event in ds["event_state_summary"]["active_bearish_events"]] == ["HIGH_VOLUME_BREAKDOWN"]
    assert ds["price_path"]["path_state"] == "EVENT_RISK"
    assert ds["price_path"]["blocked_by_event"]["carried_from_previous"] is True


def test_extreme_volume_outputs_context_event_without_direct_action_override():
    zone = _zone(relative_volume=2.8, volume_confirmation=VolumeConfirmation.CONFIRMED.value)

    ds = _summary([zone])

    assert ds["market_events"][0]["type"] == "EXTREME_VOLUME"
    assert ds["entry_action_state"] == "ACCUMULATE"
    assert ds["market_action"] == "WATCH"


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
        tier_label="短期",
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
        tier_label="短期",
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
    assert ds["final_entry_permission"]["state"] == "PROBE_ALLOWED"
    assert ds["final_entry_permission"]["label"] == "允許觀察性試探"
    assert ds["decision_derived_view"]["semantic_pipeline"]["event_signal"] == "SUPPORT_TEST"


def test_confirmed_buy_small_is_small_entry():
    zone = _zone(trading_score=95.0, confidence=0.9, expected_value=0.08, risk_reward_ratio=1.8)

    ds = _summary([zone])

    assert ds["action"] == "Hold"
    assert ds["entry_action_state"] == "SMALL_ENTRY"
    assert ds["final_entry_permission"]["state"] == "BLOCKED"


def test_high_volatility_downgrades_buy_to_buy_small():
    ds = _summary([_zone()], global_volatility=0.04)

    assert ds["action"] == "Hold"
    assert ds["market_action"] == "WATCH"
    assert ds["entry_action_state"] == "SMALL_ENTRY"
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

    assert expired["primary_zone"]["zone_health_state"] == "INVALIDATED"
    assert weak["primary_zone"]["zone_health_state"] == "WEAKENING"
    assert confirmed["primary_zone"]["zone_health_state"] == "CONFIRMED"
    assert pending["primary_zone"]["zone_health_state"] == "CANDIDATE"


def test_zone_health_state_and_deprecated_lifecycle_alias_stay_in_sync():
    """新鍵與 deprecated alias 必須同時存在且同值。

    T-044 刻意採增量更名：`SRZones.svelte` 有 5 處在讀舊的 `lifecycle`，
    破壞性改名會把「引擎抽離」與「前端 contract 遷移」綁成同一批。
    但兩個鍵一旦漂移，前端會依讀到哪一個而得到不同結果——所以要鎖住。
    """
    for ds in (
        _summary([_zone(recent_validation=RecentValidation.EXPIRED.value)]),
        _summary([_zone(confidence=0.2, confidence_level=ConfidenceLevel.LOW.value)]),
        _summary([_zone(recent_validation=RecentValidation.PENDING_VALIDATION.value)]),
    ):
        zone = ds["primary_zone"]
        assert "zone_health_state" in zone, "新鍵不見了"
        assert "lifecycle" in zone, "deprecated alias 被移除會讓前端 5 處消費點壞掉"
        assert zone["zone_health_state"] == zone["lifecycle"], (
            f"兩個鍵漂移了：zone_health_state={zone['zone_health_state']!r} "
            f"lifecycle={zone['lifecycle']!r}"
        )


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
    assert ds["daily_confirmation"]["state"] == "BLOCKED"
    assert ds["final_entry_permission"]["state"] == "BLOCKED"
    assert ds["price_path"]["path_state"] == "RR_BLOCKED"
    assert any("風險報酬比不足" in note for note in ds["risk_notes"])


def test_rr_between_gates_allows_only_buy_small():
    zone = _zone(trading_score=95.0, confidence=0.9, expected_value=0.08, risk_reward_ratio=1.8)

    ds = _summary([zone])

    assert ds["market_action"] == "WATCH"
    assert ds["action"] == "Hold"
    assert ds["entry_action_state"] == "SMALL_ENTRY"
    assert any("完整買進門檻" in note and "Final Entry 需保守觀察" in note for note in ds["risk_notes"])
    assert not any("最多小量試單" in note for note in ds["risk_notes"])


def test_final_entry_risk_note_rewrite_is_code_driven_not_text_driven():
    notes = _final_entry_risk_notes(
        [_risk_note("RR_BELOW_FULL_ENTRY", "RR 文案改掉也不得影響改寫。")],
        {"state": "WAIT_CONFIRMATION", "reason_codes": []},
    )

    assert notes[0] == "風險報酬比未達完整買進門檻，Final Entry 需保守觀察。"


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
    assert ds["market_bias"] == "BULLISH_BIAS"
    assert ds["position_action_condition"]["state"] == "HOLD"
    assert ds["entry_executability"]["executable_now"] is True
    assert ds["entry_executability"]["price_basis"] == "RECLAIM_CLOSE"
    assert ds["final_entry_permission"]["state"] == "PROBE_ALLOWED"
    assert "EXECUTION_RR_UNAVAILABLE" not in ds["final_entry_permission"]["reason_codes"]
    assert "ENTRY_ZONE_OVERSHOT" not in ds["final_entry_permission"]["reason_codes"]


def test_early_trend_outputs_bullish_bias_without_continuation():
    zone = _zone(low=98.0, high=100.0, risk_reward_ratio=2.5)

    ds = _summary([zone], current_price=102.0, global_trend=0.01, global_confidence=0.55)

    assert ds["market_regime"]["trend_regime"] == "RANGE_BOUND"
    assert ds["market_regime"]["short_term_regime"] == "EARLY_TREND"
    assert ds["market_bias"] == "BULLISH_BIAS"


def test_carried_active_reclaim_in_uptrend_outputs_bullish_bias_before_continuation():
    zone = _zone(low=98.0, high=100.0, risk_reward_ratio=2.5)
    previous = [{
        "type": "INTRADAY_RECLAIM",
        "event_family": "SUPPORT_RECLAIM",
        "event_scope": "ZONE",
        "zone_key": "SUPPORT:98.0000:100.0000",
        "root_event_type": "INTRADAY_RECLAIM",
        "latest_event_type": "INTRADAY_RECLAIM",
        "direction": "BULLISH",
        "state": "ACTIVE",
        "active": True,
        "age_bars": 0,
        "expires_after_bars": 3,
        "reason_codes": ["INTRADAY_RECLAIM"],
    }]

    ds = _summary(
        [zone],
        current_price=102.0,
        candle_high=103.0,
        candle_low=101.0,
        candle_close=102.0,
        previous_event_states=previous,
    )

    assert ds["market_events"] == []
    assert ds["event_state_summary"]["market_state"] == "RECLAIM_ATTEMPT"
    assert ds["market_regime"]["short_term_regime"] == "RECLAIM_ATTEMPT"
    assert ds["market_bias"] == "BULLISH_BIAS"
    assert ds["decision_derived_view"]["bias_state"] == "BULLISH_BIAS"
    assert ds["decision_derived_view"]["semantic_pipeline"]["lifecycle_phase"] == "CONFIRMED"


def test_carried_active_reclaim_does_not_override_avoid_bias():
    zone = _zone(low=98.0, high=100.0, risk_reward_ratio=2.5)
    previous = [{
        "type": "INTRADAY_RECLAIM",
        "event_family": "SUPPORT_RECLAIM",
        "event_scope": "ZONE",
        "zone_key": "SUPPORT:98.0000:100.0000",
        "root_event_type": "INTRADAY_RECLAIM",
        "latest_event_type": "INTRADAY_RECLAIM",
        "direction": "BULLISH",
        "state": "ACTIVE",
        "active": True,
        "age_bars": 0,
        "expires_after_bars": 3,
        "reason_codes": ["INTRADAY_RECLAIM"],
    }]

    ds = _summary(
        [zone],
        global_trend=-0.03,
        current_price=102.0,
        candle_high=103.0,
        candle_low=101.0,
        candle_close=102.0,
        previous_event_states=previous,
    )

    assert ds["event_state_summary"]["market_state"] == "RECLAIM_ATTEMPT"
    assert ds["market_action"] == "AVOID"
    assert ds["market_bias"] == "BEARISH_BIAS"
    assert ds["decision_derived_view"]["bias_reason_codes"] == ["MARKET_ACTION_AVOID"]


def test_rr_qualified_probe_waits_for_price_follow_through():
    zone = _zone(
        low=98.89375,
        high=101.9875,
        risk_reward_ratio=2.58,
        confidence=0.72,
        trading_score=82.0,
    )
    previous = [{
        "type": "INTRADAY_RECLAIM",
        "event_family": "SUPPORT_RECLAIM",
        "event_scope": "ZONE",
        "zone_key": "SUPPORT:98.8937:101.9875",
        "root_event_type": "INTRADAY_RECLAIM",
        "latest_event_type": "INTRADAY_RECLAIM",
        "direction": "BULLISH",
        "state": "ACTIVE",
        "active": True,
        "age_bars": 1,
        "expires_after_bars": 3,
        "reason_codes": ["INTRADAY_RECLAIM"],
    }]

    ds = _summary(
        [zone],
        current_price=103.85,
        global_trend=0.017,
        chip_summary={"missing": False, "score": 3.6, "signal": "NEUTRAL"},
        candle_open=104.5,
        candle_high=104.8,
        candle_low=103.3,
        candle_close=103.85,
        previous_candle_open=100.0,
        previous_candle_high=102.5,
        previous_candle_low=99.85,
        previous_candle_close=102.5,
        model_governance={
            "health_state": "DEGRADED",
            "blocking_flags": [],
            "warning_flags": ["HOLD_NOT_CALIBRATED"],
            "confidence_gate": {
                "state": "DEGRADED",
                "allow_entry": True,
                "max_entry_state": "SMALL_ENTRY",
                "reason_codes": ["HOLD_NOT_CALIBRATED"],
            },
        },
        previous_event_states=previous,
    )

    assert ds["rr_gate"]["qualified"] is True
    assert ds["rr_gate"]["reason_code"] == "EXECUTION_RR_UNAVAILABLE"
    assert ds["rr_gate"]["target_known"] is False
    assert ds["daily_price_action"]["price_follow_through_state"] == "NO_PRICE_FOLLOW_THROUGH"
    assert ds["decision_derived_view"]["version"] == "decision-derived-view-p2"
    assert "WAIT_PRICE_FOLLOW_THROUGH" in ds["decision_derived_view"]["final_entry_reason_codes"]
    assert ds["decision_derived_view"]["path_gate_state"] == "WAIT_PRICE_FOLLOW_THROUGH"
    assert ds["decision_derived_view"]["position_gate_state"] == "HOLD"
    assert ds["daily_confirmation"]["state"] == "PROBE_ALLOWED"
    assert "WAIT_PRICE_FOLLOW_THROUGH" in ds["daily_confirmation"]["reason_codes"]
    assert ds["entry_executability"]["executable_now"] is True
    assert ds["entry_executability"]["price_basis"] == "RECLAIM_CLOSE"
    assert ds["final_entry_permission"]["state"] == "PROBE_ALLOWED"
    assert "WAIT_PRICE_FOLLOW_THROUGH" in ds["final_entry_permission"]["reason_codes"]
    assert "EXECUTION_RR_UNAVAILABLE" not in ds["final_entry_permission"]["reason_codes"]
    assert "ENTRY_ZONE_OVERSHOT" not in ds["final_entry_permission"]["reason_codes"]
    assert ds["price_path"]["path_state"] == "WAIT_PRICE_FOLLOW_THROUGH"
    assert ds["price_path"]["reason_codes"] == ["WAIT_PRICE_FOLLOW_THROUGH"]
    assert ds["position_action_condition"]["state"] == "HOLD"
    assert ds["position_action_condition"]["structure_state"] == "SUPPORT_RECLAIM_CONFIRMED"
    assert "POSITION_RECLAIM_DEFENSE" in ds["position_action_condition"]["reason_codes"]


def test_semantic_pipeline_close_reclaim_testing_probe_allowed():
    zone = _zone(low=98.0, high=100.0, risk_reward_ratio=2.5)

    ds = _summary(
        [zone],
        current_price=101.0,
        candle_high=102.0,
        candle_low=97.8,
        candle_close=101.0,
    )

    semantic = ds["decision_derived_view"]["semantic_pipeline"]
    assert semantic["source_order"] == ["Event", "Lifecycle", "Market State", "Bias", "Action", "Entry"]
    assert semantic["event_signal"] == "CLOSE_RECLAIM"
    assert semantic["lifecycle_phase"] == "TESTING"
    assert semantic["market_state"] == "BULLISH_RECOVERY"
    assert semantic["action_state"] == "CONDITIONAL_HOLD"
    assert semantic["entry_permission_state"] == "PROBE_ALLOWED"
    assert ds["position_action_condition"]["state"] == "CONDITIONAL_HOLD"
    assert ds["entry_executability"]["executable_now"] is True
    assert ds["entry_executability"]["price_basis"] == "RECLAIM_CLOSE"
    assert ds["final_entry_permission"]["state"] == "PROBE_ALLOWED"
    assert "EXECUTION_RR_UNAVAILABLE" not in ds["final_entry_permission"]["reason_codes"]
    assert "ENTRY_ZONE_OVERSHOT" not in ds["final_entry_permission"]["reason_codes"]


def test_semantic_pipeline_carried_close_reclaim_confirmed_probe_allowed():
    zone = _zone(low=98.0, high=100.0, risk_reward_ratio=2.5)
    previous = [{
        "type": "INTRADAY_RECLAIM",
        "event_family": "SUPPORT_RECLAIM",
        "event_scope": "ZONE",
        "zone_key": "SUPPORT:98.0000:100.0000",
        "root_event_type": "INTRADAY_RECLAIM",
        "latest_event_type": "INTRADAY_RECLAIM",
        "direction": "BULLISH",
        "state": "ACTIVE",
        "active": True,
        "age_bars": 0,
        "expires_after_bars": 3,
        "reason_codes": ["INTRADAY_RECLAIM"],
    }]

    ds = _summary(
        [zone],
        current_price=102.0,
        candle_high=103.0,
        candle_low=101.0,
        candle_close=102.0,
        previous_candle_close=101.0,
        previous_event_states=previous,
    )

    semantic = ds["decision_derived_view"]["semantic_pipeline"]
    assert semantic["event_signal"] == "CLOSE_RECLAIM"
    assert semantic["lifecycle_phase"] == "CONFIRMED"
    assert semantic["market_state"] == "BULLISH_RECOVERY"
    assert semantic["action_state"] == "HOLD"
    assert semantic["entry_permission_state"] == "PROBE_ALLOWED"
    assert ds["position_action_condition"]["state"] == "HOLD"
    assert ds["entry_executability"]["executable_now"] is True
    assert ds["entry_executability"]["price_basis"] == "RECLAIM_CLOSE"
    assert ds["final_entry_permission"]["state"] == "PROBE_ALLOWED"
    assert "EXECUTION_RR_UNAVAILABLE" not in ds["final_entry_permission"]["reason_codes"]
    assert "ENTRY_ZONE_OVERSHOT" not in ds["final_entry_permission"]["reason_codes"]


def test_semantic_pipeline_breakout_continuation_entry_allowed():
    zone = _zone(low=98.0, high=100.0, risk_reward_ratio=2.8, confidence=0.78, trading_score=88.0)
    previous = [{
        "type": "INTRADAY_RECLAIM",
        "event_family": "SUPPORT_RECLAIM",
        "event_scope": "ZONE",
        "zone_key": "SUPPORT:98.0000:100.0000",
        "root_event_type": "INTRADAY_RECLAIM",
        "latest_event_type": "INTRADAY_RECLAIM",
        "direction": "BULLISH",
        "state": "ACTIVE",
        "active": True,
        "age_bars": 1,
        "expires_after_bars": 3,
        "reason_codes": ["INTRADAY_RECLAIM"],
    }]

    ds = _summary(
        [zone],
        current_price=105.0,
        candle_high=106.0,
        candle_low=102.0,
        candle_close=105.0,
        previous_candle_close=102.0,
        previous_event_states=previous,
    )

    semantic = ds["decision_derived_view"]["semantic_pipeline"]
    assert semantic["event_signal"] == "CLOSE_RECLAIM"
    assert semantic["lifecycle_phase"] == "CONTINUATION"
    assert semantic["market_state"] == "BULLISH_CONTINUATION"
    assert semantic["action_state"] == "HOLD"
    assert semantic["entry_permission_state"] == "ENTRY_ALLOWED"
    assert ds["market_bias"] == "BULLISH_CONTINUATION"
    assert ds["position_action_condition"]["state"] == "HOLD"
    assert ds["entry_executability"]["executable_now"] is True
    assert ds["entry_executability"]["price_basis"] == "CONTINUATION_MARKET_PRICE"
    assert ds["rr_context"]["target_price"] is None
    assert ds["rr_context"]["target_basis"] == "MARKET_ENTRY_TARGET_UNAVAILABLE"
    assert ds["rr_context"]["execution_rr"] is None
    assert ds["rr_gate"]["qualified"] is True
    assert ds["rr_gate"]["reason_code"] == "EXECUTION_RR_UNAVAILABLE"
    assert ds["rr_gate"]["gate_basis"] == "MARKET_ENTRY_TARGET_UNAVAILABLE"
    assert ds["rr_gate"]["target_known"] is False
    assert ds["final_entry_permission"]["state"] == "ENTRY_ALLOWED"
    assert "EXECUTION_RR_UNAVAILABLE" not in ds["final_entry_permission"]["reason_codes"]
    assert "ENTRY_ZONE_OVERSHOT" not in ds["final_entry_permission"]["reason_codes"]
    assert ds["best_trade_zone"] is None
    assert any("target 尚未量化" in note and "停損" in note for note in ds["risk_notes"])


def test_market_price_continuation_uses_resistance_target_without_historical_best_trade_zone():
    support = _zone(low=98.0, high=100.0, risk_reward_ratio=2.8, confidence=0.78, trading_score=88.0)
    resistance = _zone(role=ZoneType.RESISTANCE.value, low=125.0, high=127.0, risk_reward_ratio=1.2)
    previous = [{
        "type": "INTRADAY_RECLAIM",
        "event_family": "SUPPORT_RECLAIM",
        "event_scope": "ZONE",
        "zone_key": "SUPPORT:98.0000:100.0000",
        "root_event_type": "INTRADAY_RECLAIM",
        "latest_event_type": "INTRADAY_RECLAIM",
        "direction": "BULLISH",
        "state": "ACTIVE",
        "active": True,
        "age_bars": 1,
        "expires_after_bars": 3,
        "reason_codes": ["INTRADAY_RECLAIM"],
    }]

    ds = _summary(
        [support, resistance],
        current_price=105.0,
        candle_high=106.0,
        candle_low=102.0,
        candle_close=105.0,
        previous_candle_close=102.0,
        previous_event_states=previous,
    )

    semantic = ds["decision_derived_view"]["semantic_pipeline"]
    assert semantic["entry_permission_state"] == "ENTRY_ALLOWED"
    assert ds["entry_executability"]["price_basis"] == "CONTINUATION_MARKET_PRICE"
    assert ds["rr_context"]["target_price"] == 125.0
    # T-055：target 統一走「前方第一道阻力封頂」，basis 改名為 FIRST_RESISTANCE_CAP。
    # **價位不變**——原本 NEAREST_RESISTANCE_TARGET 選的就是同一個 zone，
    # 改的是「這個值代表什麼」的說法，不是選法。
    assert ds["rr_context"]["target_basis"] == "FIRST_RESISTANCE_CAP"
    assert ds["rr_context"]["execution_target"]["price"] == 125.0
    assert ds["rr_context"]["execution_target"]["basis"] == "FIRST_RESISTANCE_CAP"
    assert ds["rr_context"]["execution_rr"] == (125.0 - 105.0) / (105.0 - 98.0)
    assert ds["rr_gate"]["qualified"] is True
    assert ds["rr_gate"]["gate_basis"] == "ENTRY_STOP_TARGET"
    assert ds["final_entry_permission"]["state"] == "ENTRY_ALLOWED"
    assert ds["best_trade_zone"] is None
    assert any("市價型進場" in note and "停損" in note for note in ds["risk_notes"])


def test_market_price_target_uses_nearest_resistance_above_entry_not_below_entry():
    support = _zone(low=98.0, high=100.0, risk_reward_ratio=2.8, confidence=0.78, trading_score=88.0)
    below_entry = _zone(role=ZoneType.RESISTANCE.value, low=103.0, high=104.0, risk_reward_ratio=1.2)
    above_entry = _zone(role=ZoneType.RESISTANCE.value, low=118.0, high=120.0, risk_reward_ratio=1.2)
    previous = [{
        "type": "INTRADAY_RECLAIM",
        "event_family": "SUPPORT_RECLAIM",
        "event_scope": "ZONE",
        "zone_key": "SUPPORT:98.0000:100.0000",
        "root_event_type": "INTRADAY_RECLAIM",
        "latest_event_type": "INTRADAY_RECLAIM",
        "direction": "BULLISH",
        "state": "ACTIVE",
        "active": True,
        "age_bars": 1,
        "expires_after_bars": 3,
        "reason_codes": ["INTRADAY_RECLAIM"],
    }]

    ds = _summary(
        [support, below_entry, above_entry],
        current_price=105.0,
        candle_high=106.0,
        candle_low=102.0,
        candle_close=105.0,
        previous_candle_close=102.0,
        previous_event_states=previous,
    )

    assert ds["entry_executability"]["price_basis"] == "CONTINUATION_MARKET_PRICE"
    assert ds["rr_context"]["target_price"] == 118.0
    assert ds["rr_context"]["target_basis"] == "FIRST_RESISTANCE_CAP"


def test_market_price_target_ignores_expired_resistance_above_entry():
    support = _zone(low=98.0, high=100.0, risk_reward_ratio=2.8, confidence=0.78, trading_score=88.0)
    expired = _zone(
        role=ZoneType.RESISTANCE.value,
        low=118.0,
        high=120.0,
        risk_reward_ratio=1.2,
        recent_validation=RecentValidation.EXPIRED.value,
    )
    previous = [{
        "type": "INTRADAY_RECLAIM",
        "event_family": "SUPPORT_RECLAIM",
        "event_scope": "ZONE",
        "zone_key": "SUPPORT:98.0000:100.0000",
        "root_event_type": "INTRADAY_RECLAIM",
        "latest_event_type": "INTRADAY_RECLAIM",
        "direction": "BULLISH",
        "state": "ACTIVE",
        "active": True,
        "age_bars": 1,
        "expires_after_bars": 3,
        "reason_codes": ["INTRADAY_RECLAIM"],
    }]

    ds = _summary(
        [support, expired],
        current_price=105.0,
        candle_high=106.0,
        candle_low=102.0,
        candle_close=105.0,
        previous_candle_close=102.0,
        previous_event_states=previous,
    )

    assert ds["rr_context"]["target_price"] is None
    assert ds["rr_context"]["target_basis"] == "MARKET_ENTRY_TARGET_UNAVAILABLE"
    assert ds["rr_gate"]["target_known"] is False
    assert ds["final_entry_permission"]["state"] == "ENTRY_ALLOWED"


def test_semantic_pipeline_blocking_zone_ahead_waits_confirmation():
    support = _zone(low=98.0, high=100.0, risk_reward_ratio=2.8, confidence=0.78, trading_score=88.0)
    resistance = _zone(role=ZoneType.RESISTANCE.value, low=106.0, high=108.0, risk_reward_ratio=1.2)
    previous = [{
        "type": "INTRADAY_RECLAIM",
        "event_family": "SUPPORT_RECLAIM",
        "event_scope": "ZONE",
        "zone_key": "SUPPORT:98.0000:100.0000",
        "root_event_type": "INTRADAY_RECLAIM",
        "latest_event_type": "INTRADAY_RECLAIM",
        "direction": "BULLISH",
        "state": "ACTIVE",
        "active": True,
        "age_bars": 1,
        "expires_after_bars": 3,
        "reason_codes": ["INTRADAY_RECLAIM"],
    }]

    ds = _summary(
        [support, resistance],
        current_price=105.0,
        candle_high=106.0,
        candle_low=102.0,
        candle_close=105.0,
        previous_candle_close=102.0,
        previous_event_states=previous,
    )

    semantic = ds["decision_derived_view"]["semantic_pipeline"]
    assert semantic["lifecycle_phase"] == "CONTINUATION"
    assert semantic["action_state"] == "HOLD"
    assert semantic["entry_permission_state"] == "WAIT_CONFIRMATION"
    assert "BLOCKING_ZONE_AHEAD" in semantic["reason_codes"]
    assert ds["position_action_condition"]["state"] == "HOLD"
    assert ds["final_entry_permission"]["state"] == "WAIT_CONFIRMATION"
    assert "BLOCKING_ZONE_AHEAD" in ds["final_entry_permission"]["reason_codes"]


def test_near_resistance_blocks_probe_entry_and_best_trade_zone():
    support = _zone(
        low=98.0,
        high=100.0,
        risk_reward_ratio=2.5,
        confidence=0.82,
        trading_score=90.0,
        recent_validation=RecentValidation.PENDING_VALIDATION.value,
    )
    resistance = _zone(
        role=ZoneType.RESISTANCE.value,
        low=100.35,
        high=101.0,
        risk_reward_ratio=1.2,
        confidence=0.7,
    )

    ds = _summary([support, resistance], current_price=100.1)

    assert ds["decision_derived_view"]["semantic_pipeline"]["entry_permission_state"] == "WAIT_CONFIRMATION"
    assert "BLOCKING_ZONE_AHEAD" in ds["decision_derived_view"]["semantic_pipeline"]["reason_codes"]
    assert ds["entry_executability"]["executable_now"] is True
    assert ds["entry_blocking_zone"]["blocked"] is True
    assert ds["entry_blocking_zone"]["reason_code"] == "NEAR_RESISTANCE_BLOCKING_ENTRY"
    assert ds["final_entry_permission"]["state"] == "WAIT_CONFIRMATION"
    assert "NEAR_RESISTANCE_BLOCKING_ENTRY" in ds["final_entry_permission"]["reason_codes"]
    assert ds["best_trade_zone"] is None


def test_hard_block_daily_confirmation_does_not_backfill_derived_reasons():
    # 硬性阻擋（此處 RR 不過 → BLOCKED）不得回填 derived daily reasons，否則會產生
    # 「禁止進場」卻同時掛「等待價格延續」的矛盾標籤（I-002 要消除的訊號不一致）。
    zone = _zone(low=98.0, high=100.0)
    derived_view = {"daily_reason_codes": ["WAIT_PRICE_FOLLOW_THROUGH", "NO_MOMENTUM_CONFIRMATION"]}

    daily = _daily_confirmation(
        primary_zone=zone,
        primary_interaction=None,
        daily_price_action={},
        rr_gate={"qualified": False, "reason_code": "RR_NOT_QUALIFIED"},
        derived_view=derived_view,
        entry_action_state="PROBE_ENTRY",
        daily_candidate_zones=[],
        current_price=99.0,
    )

    assert daily["state"] == "BLOCKED"
    assert daily["reason_codes"] == ["RR_NOT_QUALIFIED"]
    assert "WAIT_PRICE_FOLLOW_THROUGH" not in daily["reason_codes"]
    assert "NO_MOMENTUM_CONFIRMATION" not in daily["reason_codes"]


def test_entry_track_daily_confirmation_still_backfills_derived_reasons():
    # 對照組：進場軌道（此處 PROBE_ALLOWED）仍要保留 derived daily reasons，確認上一個
    # 測試擋掉的是「硬阻擋」而非把回填整個關掉。
    zone = _zone(low=98.0, high=100.0)
    derived_view = {"daily_reason_codes": ["NO_MOMENTUM_CONFIRMATION"]}

    daily = _daily_confirmation(
        primary_zone=zone,
        primary_interaction=None,
        daily_price_action={},
        rr_gate={"qualified": True},
        derived_view=derived_view,
        entry_action_state="PROBE_ENTRY",
        daily_candidate_zones=[],
        current_price=99.0,
    )

    assert daily["state"] == "PROBE_ALLOWED"
    assert "NO_MOMENTUM_CONFIRMATION" in daily["reason_codes"]


def _derived(
    *,
    primary_zone=None,
    market_action="WATCH",
    entry_action_state="WAIT_CONFIRMATION",
    short_term_regime="NORMAL",
    primary="RANGE_BOUND",
    active_bearish=False,
    structure_state="NORMAL",
    rr_gate=None,
    daily_candidate_zones=None,
    blocking_zone_ahead=False,
) -> dict:
    event_state_summary = {
        "active": [],
        "candidates": [],
        "active_bearish_events": [{"type": "HIGH_VOLUME_BREAKDOWN"}] if active_bearish else [],
    }
    return _decision_derived_view(
        {"short_term_regime": short_term_regime, "primary": primary},
        primary_zone,
        market_action,
        entry_action_state,
        event_state_summary,
        None,
        rr_gate if rr_gate is not None else {"qualified": True},
        structure_state,
        daily_candidate_zones or [],
        blocking_zone_ahead,
    )


def test_derived_position_gate_defend_breakdown_on_active_bearish():
    dv = _derived(primary_zone=_zone(role=ZoneType.SUPPORT.value), active_bearish=True)
    assert dv["path_gate_state"] == "EVENT_RISK"
    assert dv["position_gate_state"] == "DEFEND_BREAKDOWN"
    assert dv["semantic_pipeline"]["action_state"] == dv["position_gate_state"]
    assert "POSITION_DEFENSE_REQUIRED" in dv["position_reason_codes"]


def test_derived_position_gate_defend_breakdown_on_structure_breakdown():
    dv = _derived(primary_zone=_zone(role=ZoneType.SUPPORT.value), structure_state="BREAKDOWN")
    assert dv["path_gate_state"] == "INVALIDATION_RISK"
    assert dv["position_gate_state"] == "DEFEND_BREAKDOWN"
    assert dv["semantic_pipeline"]["action_state"] == dv["position_gate_state"]
    assert "SUPPORT_BREAKDOWN_RISK" in dv["position_reason_codes"]


def test_derived_position_gate_support_defense_on_clean_support():
    dv = _derived(primary_zone=_zone(role=ZoneType.SUPPORT.value))
    assert dv["path_gate_state"] == "OPEN_PATH"
    assert dv["position_gate_state"] == "WATCH"
    assert dv["position_reason_codes"] == ["POSITION_SUPPORT_DEFENSE"]
    assert dv["semantic_pipeline"]["action_state"] == dv["position_gate_state"]


def test_derived_position_gate_upside_breakout_required_on_resistance():
    dv = _derived(primary_zone=_zone(role=ZoneType.RESISTANCE.value))
    assert dv["path_gate_state"] == "OPEN_PATH"
    assert dv["position_gate_state"] == "AVOID"
    assert dv["position_reason_codes"] == ["POSITION_RESISTANCE_OVERHEAD"]
    assert dv["semantic_pipeline"]["action_state"] == dv["position_gate_state"]


def test_derived_position_gate_rr_blocked_support_defense():
    dv = _derived(
        primary_zone=_zone(role=ZoneType.SUPPORT.value),
        rr_gate={"qualified": False, "reason_code": "RR_NOT_QUALIFIED"},
    )
    assert dv["path_gate_state"] == "RR_BLOCKED"
    assert dv["path_reason_codes"] == ["RR_NOT_QUALIFIED"]
    assert dv["position_gate_state"] == "WATCH"
    assert dv["semantic_pipeline"]["entry_permission_state"] == "BLOCKED"


def test_semantic_pipeline_market_action_avoid_blocks_action_and_entry():
    dv = _derived(
        primary_zone=_zone(role=ZoneType.SUPPORT.value),
        market_action="AVOID",
        short_term_regime="RECOVERY",
        structure_state="SUPPORT_RECLAIM_CONFIRMED",
    )

    semantic = dv["semantic_pipeline"]
    assert semantic["market_state"] == "BULLISH_RECOVERY"
    assert semantic["bias_state"] == "BEARISH_BIAS"
    assert semantic["action_state"] == "AVOID"
    assert semantic["entry_permission_state"] == "BLOCKED"
    assert "MARKET_ACTION_AVOID" in semantic["reason_codes"]
    assert dv["position_gate_state"] == "AVOID"


def test_position_action_condition_ignores_deprecated_position_gate_fallback():
    condition = _position_action_condition(
        _zone(role=ZoneType.SUPPORT.value),
        "NORMAL",
        {"position_gate_state": "DEFEND_BREAKDOWN", "position_reason_codes": ["POSITION_SUPPORT_DEFENSE"]},
    )

    assert condition["state"] == "WATCH"
    assert "POSITION_SUPPORT_DEFENSE" in condition["reason_codes"]


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
    assert ds["decision_derived_view"]["semantic_pipeline"]["action_state"] == "AVOID"
    assert ds["position_action_condition"]["state"] == "AVOID"


def test_final_entry_permission_keeps_invalidated_distinct_from_waiting():
    permission = _final_entry_permission("BUY", {"state": "INVALIDATED", "reason_codes": ["SUPPORT_CLOSED_BELOW"]})

    assert permission["state"] == "BLOCKED"
    assert permission["label"] == "禁止進場"
    assert permission["daily_confirmation_state"] == "INVALIDATED"
    assert permission["reason_codes"] == ["SUPPORT_CLOSED_BELOW"]


def test_final_entry_permission_never_outputs_no_setup():
    for daily_state in ("NO_SETUP", "WAIT_DAILY_CONFIRM", "INVALIDATED", "BLOCKED"):
        permission = _final_entry_permission("WAIT_CONFIRMATION", {"state": daily_state, "reason_codes": ["TEST_REASON"]})
        assert permission["state"] != "NO_SETUP"
        assert permission["label"] != "無設定"


def test_final_entry_permission_merges_derived_final_reasons_and_normalizes_state():
    permission = _final_entry_permission(
        "BUY",
        {"state": "ENTRY_READY", "reason_codes": ["ENTRY_STATE_READY"]},
        {
            "final_entry_reason_codes": ["ENTRY_GATE_WAIT_CONFIRMATION"],
        },
    )

    assert permission["state"] == "ENTRY_ALLOWED"
    assert permission["entry_action_state"] == "BUY"
    assert permission["daily_confirmation_state"] == "ENTRY_READY"
    assert permission["reason_codes"] == ["ENTRY_STATE_READY", "ENTRY_GATE_WAIT_CONFIRMATION"]


def test_final_entry_permission_normalizes_buy_ready_to_entry_allowed():
    permission = _final_entry_permission("BUY", {"state": "ENTRY_READY", "reason_codes": ["ENTRY_STATE_READY"]})

    assert permission["state"] == "ENTRY_ALLOWED"
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
    assert ds["primary_zone"]["zone_interaction"]["price_action_evidence"]["reclaim_type"] == "UNDERCUT_RECLAIM"
    assert ds["market_action"] != "AVOID"
    assert ds["position_action"] != "EXIT"
    assert ds["position_action_condition"]["state"] == "HOLD"
    assert ds["position_action_condition"]["structure_state"] == "SUPPORT_RECLAIM_CONFIRMED"
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
    assert ds["event_state_summary"]["active_bearish_events"] == []
    assert ds["event_state_summary"]["market_state"] == "RECLAIM_ATTEMPT"
    assert ds["market_regime"]["short_term_regime"] == "RECLAIM_ATTEMPT"
    assert ds["market_action"] != "AVOID"
    assert ds["position_action"] != "EXIT"
    assert ds["price_path"]["path_state"] != "EVENT_RISK"
    assert ds["price_path"]["blocked_by_event"] is None


def test_resolved_breakdown_is_excluded_from_bias_and_primary_zone_gating():
    # I-016：已被 reclaim/reversal resolve 的 breakdown 只保留在 raw chain 呈現，
    # 不得再影響 gating——primary zone 選擇、market_bias 與 event-aware relevance
    # 一律只吃 active 事件。
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

    # raw chain 仍保留完整 break→reclaim→reversal，但 active bearish gate 已清空。
    assert "HIGH_VOLUME_BREAKDOWN" in [item["type"] for item in ds["event_sequence"]]
    assert ds["event_state_summary"]["active_bearish_events"] == []
    # bias 不得因已收復的歷史 breakdown 標成偏空。
    assert ds["market_bias"] != "BEARISH_BIAS"
    # 主交易區仍是被收復的支撐區，不因 resolved breakdown 的 relevance 懲罰被降級/剔除。
    assert ds["primary_zone"] is not None
    assert (ds["primary_zone"]["price_low"], ds["primary_zone"]["price_high"]) == (98.0, 100.0)


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
    assert ds["price_path"]["blocking_zone"]["source_scope"] == "DAILY_CANDIDATE"
    assert ds["price_path"]["blocking_zone"]["method"] == "daily_candle"


def test_zero_width_daily_candidate_zones_output_triggers():
    ds = _summary(
        [],
        current_price=2405.0,
        candle_high=2405.0,
        candle_low=2405.0,
        candle_close=2405.0,
        previous_candle_close=2400.0,
    )

    support, resistance = ds["daily_candidate_zones"]
    assert support["zone_kind"] == "BREAKDOWN_TRIGGER"
    assert support["trigger_price"] == 2405.0
    assert support["label"] == "BREAKDOWN_TRIGGER 2405.00"
    assert resistance["zone_kind"] == "BREAKOUT_TRIGGER"
    assert resistance["trigger_price"] == 2405.0
    assert resistance["label"] == "BREAKOUT_TRIGGER 2405.00"
    assert ds["best_trade_zone"] is None


def test_price_path_reports_blocking_zone_and_next_decision_price():
    support = _zone(low=98.0, high=100.0, risk_reward_ratio=2.5)
    resistance = _zone(role=ZoneType.RESISTANCE.value, low=106.0, high=108.0, risk_reward_ratio=2.0)

    ds = _summary([support, resistance], current_price=102.0)

    assert ds["price_path"]["next_decision_price"] == 100.0
    assert ds["price_path"]["next_decision_source"] == "nearest_support_zone"
    assert ds["price_path"]["blocking_zone"]["label"] == "106.00 ~ 108.00"
    assert ds["price_path"]["blocking_zone"]["source_scope"] == "ZONE_SCORE_POOL"
    assert ds["price_path"]["blocking_zone"]["method"] == "atr"
    assert ds["price_path"]["blocking_zone"]["tier"] == ZoneTier.TIER_1_MAIN_STRUCTURE.value
    assert ds["price_path"]["blocking_zone"]["confidence"] == resistance.confidence
    assert ds["price_path"]["blocking_zone"]["selected_summary_zone"] is True
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
    assert ds["rr_context"]["entry_price"] == 100.0
    assert ds["rr_context"]["entry_zone_lower"] == 98.0
    assert ds["rr_context"]["entry_zone_upper"] == 100.0
    assert ds["rr_context"]["stop_price"] == 98.0
    assert ds["rr_context"]["price_basis"] == "PRIMARY_SUPPORT_UPPER"
    assert ds["rr_context"]["stop_basis"] == "PRIMARY_ZONE_STOP"
    assert ds["rr_context"]["executable_now"] is True

    # ── T-055：限價路徑不得用 setup RR 反推 target ────────────────────────────
    #
    # 這裡原本斷言 `target_price == 104.8` / `target_basis == "PRIMARY_ZONE_RR"`——
    # 那個 104.8 是 `entry + risk × entry_rr` **反推**出來的，等於用 setup RR 去證明
    # setup RR，`execution_rr` 因此恆等於 `entry_rr`，gate 在測同一個數字兩次。
    #
    # 這組 fixture 只有一個 support zone、前方沒有任何 resistance，
    # 所以**正確答案是 target unknown**，不是編一個出來。
    assert ds["rr_context"]["target_price"] is None
    assert ds["rr_context"]["target_basis"] == "TARGET_UNAVAILABLE"
    assert ds["rr_context"]["execution_target"] is None
    assert ds["rr_context"]["executable_rr"] is None
    assert ds["rr_context"]["rr_formula_available"] is False

    # setup RR 仍然要有——它是 zone 統計，與能不能執行無關。
    assert ds["rr_context"]["setup_rr"] == 2.4
    assert ds["rr_context"]["entry_rr"] == 2.4  # legacy alias，恆同值
    quality = ds["data_quality"]
    assert quality["market_data_completeness"] == quality["overall_completeness"]
    assert quality["rr_completeness"] == 1.0
    assert quality["trade_qualification_completeness"] == 1.0


def test_tactical_stop_is_separate_from_structural_stop_in_rr_context():
    primary = _zone(low=98.0, high=100.0, risk_reward_ratio=2.4, trading_score=92.0, confidence=0.82)
    tactical = _zone(
        low=99.0,
        high=99.5,
        trading_score=40.0,
        confidence=0.2,
        confidence_level=ConfidenceLevel.LOW.value,
        risk_reward_ratio=1.0,
        tier=ZoneTier.TIER_3_SHORT_TERM.value,
        tier_label="短期",
    )

    ds = _summary([primary, tactical], current_price=100.1)

    assert ds["primary_zone"]["price_low"] == 98.0
    assert ds["defense_lines"]["tactical"]["invalidation_price"] == 99.0
    assert ds["defense_lines"]["strategic"]["invalidation_price"] == 98.0
    assert ds["rr_context"]["stop_price"] == 99.0
    assert ds["rr_context"]["stop_basis"] == "TACTICAL_STOP"
    assert ds["rr_context"]["structural_stop_price"] == 98.0


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


def test_data_quality_uses_chip_coverage_and_confidence_from_summary():
    ds = _summary(
        [_zone()],
        chip_summary={
            "missing": False,
            "score": -55.0,
            "effective_score": -19.25,
            "coverage": 0.35,
            "confidence": 0.35,
            "signal": "BEARISH",
        },
    )

    chip = ds["data_quality"]["features"]["chip"]
    assert ds["data_quality"]["chip_coverage"] == 0.35
    assert chip["confidence"] == 0.35
    assert chip["value"] == -55.0
    assert chip["interpretation"] == "NEGATIVE"


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


def test_defense_lines_tactical_skips_decision_invisible_events():
    """階段 D：`_defense_lines` 是**位置型**讀者，必須跳過 `decision_visible=False` 的事件。

    它取「raw `market_events` 裡第一個 `zone_ref` 對得上的事件」當戰術防守線，不比對型別名，
    所以名字型的隔離（決策端不認識新名字）對它完全無效。`SUPPORT_RETEST_HELD` 不帶品質門檻
    ——碰到 zone 且未收破就成立——會在低品質 zone 上先成立而插到清單最前面，把 tactical
    換成那個 zone；`rr_context.stop_basis` / `stop_price` 吃 tactical，所以會一路改到
    `stock_sr_decisions` 的既有欄位。實測（2026-08-20，四檔 21 階）未過濾時 84 筆決策
    有 7 筆的 `defense_lines.tactical` 被換掉。
    """
    # weak 收在區間內（closed_inside）：既有分支全部不成立——沒有跌破、沒有 UNDERCUT_RECLAIM，
    # REVERSAL_CANDIDATE 也被品質門檻擋掉——所以它**只**產出 SUPPORT_RETEST_HELD。
    weak = _zone(
        low=99.0,
        high=100.0,
        confidence=0.2,
        confidence_level=ConfidenceLevel.LOW.value,
        expected_value=-0.01,
        tier=ZoneTier.TIER_3_SHORT_TERM.value,
        tier_label="短期",
    )
    # good 被跌破後收回上緣，產出決策可見的 INTRADAY_RECLAIM。
    good = _zone(low=97.0, high=98.0)
    zones = [weak, good]

    events = detect_market_events(
        zones, current_price=99.5, candle_high=100.2, candle_low=96.5, candle_close=99.5
    )

    types = [event["type"] for event in events]
    # 前提：新事件確實排在既有事件之前（低品質 zone 先成立），否則這條測試證明不到東西。
    assert types[0] == "SUPPORT_RETEST_HELD"
    assert "INTRADAY_RECLAIM" in types

    lines = _defense_lines(zones, good, 99.5, events)

    # tactical 要維持階段 D 之前的答案：INTRADAY_RECLAIM 那個 zone，不是先插隊的 weak。
    assert lines["tactical"]["price_low"] == 97.0
    assert lines["tactical"]["price_high"] == 98.0
    # 這條走的是**事件路徑**（有 visible 事件可退），source 要能與 fallback 分得開。
    assert lines["tactical"]["source"] == TACTICAL_SOURCE_EVENT


def test_defense_lines_tactical_fallback_is_labelled_separately():
    """所有對得上的事件都是 shadow 時，tactical 退到「最近的 TIER_3」且 **source 要換值**。

    這是 `test_defense_lines_tactical_skips_decision_invisible_events` 沒涵蓋的另一半：
    那條有一個 visible 的 `INTRADAY_RECLAIM` 可退，這條**一個 visible 的 zone 事件都沒有**。
    live 上真的會出現（2026-08-25 `5490`：當日唯一的 zone 範圍事件是 3 筆
    `RESISTANCE_BREAKOUT`，全部 decision_visible=False）。

    重點在 source：行為（退到最近的 TIER_3）本來就對，但兩條路徑過去標成同一個值，
    於是「過濾成功」與「過濾失效」在輸出上分不開
    （見 docs/sr-zone-scoring.md「事件的決策可見性」；原記於 issue.md I-093，已收斂）。
    """
    # near/far 都是 TIER_3，near 距離現價較近；兩者都不會被事件比對到。
    near = _zone(low=99.0, high=100.0, tier=ZoneTier.TIER_3_SHORT_TERM.value, tier_label="短期")
    far = _zone(low=90.0, high=91.0, tier=ZoneTier.TIER_3_SHORT_TERM.value, tier_label="短期")
    zones = [far, near]

    # 唯一的事件是 shadow，且它的 zone_ref 對得上 far——沒被過濾的話 tactical 會變成 far。
    shadow_event = {
        "type": "SUPPORT_RETEST_HELD",
        "decision_visible": False,
        "zone_ref": {"price_low": 90.0, "price_high": 91.0},
    }

    lines = _defense_lines(zones, near, 99.5, [shadow_event])

    # 行為不變：退到最近的 TIER_3（near），不是 shadow 指向的 far。
    assert lines["tactical"]["price_low"] == 99.0
    assert lines["tactical"]["price_high"] == 100.0
    # **正向等值斷言**：寫成 `!= TACTICAL_SOURCE_EVENT` 的話，拼字錯誤、空字串、None
    # 全都會通過，而這條測試存在的唯一理由就是釘住這個值。
    assert lines["tactical"]["source"] == TACTICAL_SOURCE_FALLBACK


def test_defense_lines_tactical_source_is_not_inferred_from_result():
    """事件指向的 zone **剛好就是最近的 TIER_3** 時，source 仍必須是事件路徑的值。

    這條鎖住的是 `_defense_lines` 的實作方式，不是它的輸出值：`tactical_source` 必須在
    **分支裡**決定，不能事後從結果反推（例如比對「tactical 是不是等於最近的 TIER_3」）。
    反推在這個形狀下會得到相反的答案——兩條路徑產生同一個 zone，結果無法區分來源，
    而那正好讓這個欄位回到修正前的狀態：不可稽核
    （見 docs/sr-zone-scoring.md「事件的決策可見性」）。

    其餘兩支 fallback 測試證明不了這件事：那裡事件指向的 zone 是 TIER_1，
    與最近的 TIER_3 是不同的 zone，反推式實作照樣會通過。
    """
    # near_t3 同時是「事件指向的 zone」與「距離現價最近的 TIER_3」。
    near_t3 = _zone(low=99.0, high=100.0, tier=ZoneTier.TIER_3_SHORT_TERM.value, tier_label="短期")
    far_t3 = _zone(low=90.0, high=91.0, tier=ZoneTier.TIER_3_SHORT_TERM.value, tier_label="短期")
    zones = [far_t3, near_t3]

    visible_event = {
        "type": "INTRADAY_RECLAIM",
        "decision_visible": True,
        "zone_ref": {"price_low": 99.0, "price_high": 100.0},
    }

    lines = _defense_lines(zones, near_t3, 99.5, [visible_event])

    # 前提：兩條路徑會挑到同一個 zone——否則這條測試證明不到「無法從結果反推」。
    assert lines["tactical"]["price_low"] == 99.0
    assert lines["tactical"]["price_high"] == 100.0
    # 值必須來自事件路徑。反推式實作會在這裡回 nearest_tier3。
    assert lines["tactical"]["source"] == TACTICAL_SOURCE_EVENT


def test_defense_lines_tactical_fallback_when_no_events():
    """完全沒有事件時同樣走 fallback，source 也必須是 fallback 值。

    邊界：`market_events` 為空（或 None）時迴圈根本不執行，與「事件全被過濾掉」
    走到同一個分支，標籤不能因為來源不同而漂移。
    """
    near = _zone(low=99.0, high=100.0, tier=ZoneTier.TIER_3_SHORT_TERM.value, tier_label="短期")
    zones = [near]

    for events in ([], None):
        lines = _defense_lines(zones, near, 99.5, events)
        assert lines["tactical"]["price_low"] == 99.0
        assert lines["tactical"]["source"] == TACTICAL_SOURCE_FALLBACK


# ── I-082：EXPIRED 的 primary zone 不得升級到 `Buy` ────────────────────────────
#
# `sr-zone-scoring.md`「Legacy action pipeline」第 4 條要求 EXPIRED 的 primary zone
# 「不應升級到 `Buy`」。這個保證**不是**由 `_decision_action` 的 `strong` 條件式提供的
# （它只看 regime flags，不看 zone 層級的 recent_validation），而是由更上游的
# `_structure_state` 提供：SUPPORT + EXPIRED → `BREAKDOWN` → `structure_broken` 提前
# return `Avoid`，根本走不到 `strong`；RESISTANCE 則被 `bearish_setup` 擋下。
#
# 也就是說，**守門是隱含的**：任何人放寬 `_structure_state` 的 EXPIRED 規則
# （例如改成「跌破後已收回就不算 BREAKDOWN」），Buy 路徑就會靜默打開，而在這組測試
# 出現之前沒有任何東西會報錯。詳見 `docs/sr-zone-scoring.md`「Legacy action pipeline」第 4 條
# （原記於 issue.md I-082，已收斂）。


def _expired_guard_zone(recent_validation: str, role: str = ZoneType.SUPPORT.value) -> ZoneScore:
    """把 `strong` 的其餘五個條件全部拉到達標，只留 `recent_validation` 當變因。

    relevance 對 EXPIRED 仍有 84.8（>= 75）——EXPIRED 只讓 validation 分量歸零，
    其餘分量（距離 30 / ev_rr 30 / volume 10 / role_readiness 10 / confidence 10）補得回來。
    """
    return _zone(
        role=role,
        low=99.9,
        high=100.05,          # 距 current_price=100.1 僅 0.05%，distance 分量近滿分
        confidence=1.0,       # >= 0.65
        expected_value=0.05,  # > 0
        risk_reward_ratio=3.0,  # >= 2.0
        recent_validation=recent_validation,
        volume_confirmation=VolumeConfirmation.CONFIRMED.value,
    )


def _legacy_action(zone: ZoneScore, global_trend: float) -> str:
    """跑 legacy action pipeline（`_decision_action`），回傳 action。

    刻意停在這一層而不是 `build_decision_summary`：文件那句「不應升級到 `Buy`」講的就是
    legacy pipeline，而端到端還會再經 `final_entry_permission` 降級（實測對照組會變成
    `Hold`/`WAIT_CONFIRMATION`），那樣就分不出「被 EXPIRED 擋下」還是「被進場閘降級」。
    """
    interaction = _zone_interaction(zone, 100.1, None, None, None)
    structure_state = _structure_state(zone, interaction, None)
    regime = _market_regime(
        global_trend,
        0.01,
        1.0,
        {"missing": False, "score": 55.0, "signal": "BULLISH"},
        structure_state,
        [],
        None,
        None,
    )
    return _decision_action(regime, zone, 100.1, interaction, [])[2]


def test_expired_primary_zone_never_upgrades_to_buy():
    """EXPIRED 的 primary zone 在任何 role × regime 組合下都不得輸出 `Buy`。"""
    for role in (ZoneType.SUPPORT.value, ZoneType.RESISTANCE.value):
        for global_trend in (0.05, 0.0, -0.05):
            zone = _expired_guard_zone(RecentValidation.EXPIRED.value, role=role)
            action = _legacy_action(zone, global_trend)
            assert action != "Buy", (
                f"EXPIRED primary zone 升級到 Buy（role={role} global_trend={global_trend}）"
                "——守門失效，見 docs/sr-zone-scoring.md「Legacy action pipeline」第 4 條"
            )
            assert action == "Avoid", (
                f"預期 Avoid，實際 {action}（role={role} global_trend={global_trend}）"
            )


def test_expired_guard_control_group_would_otherwise_reach_buy():
    """對照組：同一個 fixture 只把 EXPIRED 換成 VALIDATED_RECENTLY，必須拿到 `Buy`。

    **這條測試是上一條的前提，不能刪。** 少了它，上一條會在 fixture 退化成
    「根本達不到 `strong`」時假綠——那時它證明的是「這組數字進不了 Buy」，
    而不是「EXPIRED 擋住了 Buy」。
    """
    zone = _expired_guard_zone(RecentValidation.VALIDATED_RECENTLY.value)

    assert _legacy_action(zone, 0.05) == "Buy"


def test_expired_primary_zone_end_to_end_is_not_buy():
    """端到端補一刀：EXPIRED 真的成為 primary（fallback 生效）時，對外 action 不得是 `Buy`。

    `_pick_primary_zone` 的嚴格清單會排除 EXPIRED，但清單落空時 fallback 只保留
    `role != AT_ZONE`——所以只有一個 EXPIRED zone 時它仍會被選為 primary。
    這裡順便斷言 primary 真的是 EXPIRED，避免測試變成空轉。
    """
    ds = _summary([_expired_guard_zone(RecentValidation.EXPIRED.value)])

    assert ds["primary_zone"]["recent_validation"] == RecentValidation.EXPIRED.value, (
        "fixture 沒讓 EXPIRED zone 成為 primary，這條測試就沒有在測 I-082 的情境"
    )
    assert ds["action"] != "Buy"


# ── T-056：戰術壓力與前方擋路壓力是兩層 ───────────────────────────────────────
#
# `nearest_resistance_zone` **不是價格最近的壓力**：它的選法是
# `_decision_distance_score = _distance_pct_to_zone + _zone_width_penalty × 0.08`，
# 寬區間會被推到後面（這是 sr-zone-scoring.md 寫明的刻意設計，不是 bug）。
# 於是「顯示的最近壓力」與「實際擋住進場的壓力」可以是不同的 zone——live 0050
# （2026-08-20）就是這樣：顯示 107.18，擋路的卻是 0.51% 外的 105.19。
#
# T-056 的作法是**正名 ＋ 兩層並列**，不動選法。下面這組 fixture 直接用 0050 的真實數字。


def _zones_0050() -> list[ZoneScore]:
    """live 0050（`analyzed_at=2026-08-20`，`current_price=104.65`）的三個關鍵 zone。"""
    return [
        # 主交易區：短期支撐
        _zone(
            role=ZoneType.SUPPORT.value, low=103.4886, high=104.1114,
            tier=ZoneTier.TIER_3_SHORT_TERM.value, tier_label="短期",
            confidence=0.6553, risk_reward_ratio=1.8664,
        ),
        # 主結構壓力：**寬 5.55%**，距現價僅 0.51%——擋路的就是它
        _zone(
            role=ZoneType.RESISTANCE.value, low=105.18616618688597, high=110.99830961886403,
            tier=ZoneTier.TIER_1_MAIN_STRUCTURE.value, tier_label="主結構",
            confidence=0.6841, risk_reward_ratio=2.5514,
        ),
        # 交易區壓力：窄，距現價 2.42%——因為窄而沒有寬度懲罰，反而被選成 "nearest"
        _zone(
            role=ZoneType.RESISTANCE.value, low=107.1775, high=107.8225,
            tier=ZoneTier.TIER_2_TRADING_ZONE.value, tier_label="交易區",
            confidence=0.62, risk_reward_ratio=2.1,
        ),
    ]


def test_tactical_resistance_keeps_the_existing_width_weighted_selection():
    """`tactical_resistance_zone` 必須與改動前的 `nearest_resistance_zone` 逐字相同。

    T-056 是呈現層調整，**不得改 `_decision_distance_score`**。這條就是那個界線的驗收：
    0050 這組數字下，答案必須仍然是 107.18 那個窄區，而不是距離更近的 105.19。
    """
    ds = _summary(_zones_0050(), current_price=104.65)

    assert ds["tactical_resistance_zone"]["price_low"] == 107.1775
    assert ds["tactical_resistance_zone"]["decision_role"] == "TACTICAL_RESISTANCE"


def test_nearest_resistance_zone_stays_as_legacy_alias():
    """舊鍵保留且與新鍵**同值**——前端與 Go projection 還在讀它，漂移會靜默改變顯示。"""
    ds = _summary(_zones_0050(), current_price=104.65)

    assert ds["nearest_resistance_zone"] == ds["tactical_resistance_zone"]


def test_blocking_resistance_zone_is_the_price_nearest_one_not_the_tactical_one():
    """擋路壓力用純距離選，所以它可以比 `tactical_resistance_zone` 更近。

    這正是 live 0050 的矛盾點：「最近壓力 107.18」與「擋住進場的 105.19」並存。
    兩層都輸出之後才自洽。
    """
    ds = _summary(_zones_0050(), current_price=104.65)

    blocking = ds["blocking_resistance_zone"]
    tactical = ds["tactical_resistance_zone"]

    assert blocking["price_low"] == 105.18616618688597
    assert blocking["decision_role"] == "BLOCKING_RESISTANCE"
    # 擋路的那個**在價格上更近**——這就是 "nearest" 這個舊名字誤導人的地方
    assert blocking["distance_pct"] < tactical["distance_pct"]


def test_blocking_resistance_zone_and_entry_blocking_zone_are_the_same_zone():
    """兩者共用 `_blocking_resistance_zone`，恆指同一個 zone；分開實作就會漂移。"""
    ds = _summary(_zones_0050(), current_price=104.65)

    blocking = ds["blocking_resistance_zone"]
    gate_zone = ds["entry_blocking_zone"]["blocking_zone"]

    assert (blocking["price_low"], blocking["price_high"]) == (
        gate_zone["price_low"], gate_zone["price_high"]
    )


def test_primary_structural_zone_is_not_redefined_by_t056():
    """`primary_structural_zone` 語意不變：Tier-1 品質最高的**大結構參考**。

    它與 `blocking_resistance_zone` 在 0050 碰巧是同一個 zone，**但那是巧合**——
    前者取 Tier-1 品質最高者、後者取距離最近者。所以 T-056 刻意不建
    `structural_resistance_zone` 把兩者混成一個欄位。
    """
    ds = _summary(_zones_0050(), current_price=104.65)

    assert ds["primary_structural_zone"]["tier"] == ZoneTier.TIER_1_MAIN_STRUCTURE.value
    assert ds["primary_structural_zone"]["decision_role"] == "STRUCTURAL"


def test_blocking_resistance_zone_may_be_short_term_and_must_not_claim_structural():
    """**F2 迴歸保護**：第一道擋路壓力不保證是結構性的。

    `_blocking_resistance_zone` 沒有任何 tier 過濾，所以它可以是 Tier-3 短期壓力。
    欄位不得因此改名或被標成結構層——UI 標籤要用「前方擋路壓力」而不是「結構壓力」。
    這條測試就是在釘住「不要把它當結構壓力」這件事。
    """
    short_term_resistance = _zone(
        role=ZoneType.RESISTANCE.value, low=100.4, high=100.6,
        tier=ZoneTier.TIER_3_SHORT_TERM.value, tier_label="短期",
    )
    support = _zone(role=ZoneType.SUPPORT.value, low=99.0, high=100.0)

    ds = _summary([support, short_term_resistance], current_price=100.1)

    blocking = ds["blocking_resistance_zone"]
    assert blocking["tier"] == ZoneTier.TIER_3_SHORT_TERM.value
    assert blocking["decision_role"] == "BLOCKING_RESISTANCE"
    # 大結構參考是另一個欄位，不會因為擋路的是短期壓力就被改寫
    assert ds["primary_structural_zone"] is None or (
        ds["primary_structural_zone"]["tier"] == ZoneTier.TIER_1_MAIN_STRUCTURE.value
    )


# ── T-055：RR 語意分層的契約測試 ──────────────────────────────────────────────

# ⚠️ **`rr_gate.gate_basis` 與 `rr_context.target_basis` 是兩個不同的值域**，不要混用：
#
#   rr_gate.gate_basis      ENTRY_STOP_TARGET / MARKET_ENTRY_TARGET_UNAVAILABLE / ZONE_STATISTIC
#   rr_context.target_basis FIRST_RESISTANCE_CAP / MARKET_ENTRY_TARGET_UNAVAILABLE /
#                           TARGET_UNAVAILABLE / UNAVAILABLE
#
# `TARGET_UNAVAILABLE` **不會**由 `_execution_rr_gate` 輸出——限價型且無 target 時
# gate 走的是 `ZONE_STATISTIC`（沿用 setup-RR 判定）。
GATE_BASIS_DOMAIN = {
    "ENTRY_STOP_TARGET",                # actual_rr = entry/stop/target 算出來的
    "MARKET_ENTRY_TARGET_UNAVAILABLE",  # 市價型但前方找不到可量化壓力
    "ZONE_STATISTIC",                   # actual_rr 來自 zone 歷史統計（2026-08-27 裁決擴出）
}


def _gate_of(zones, **kw):
    return _summary(zones, **kw)["rr_gate"]


def test_rr_gate_contract_fields_are_always_present():
    """`gate_kind` / `gate_basis` / `zone_actual_rr` **一律輸出**，且值域受限。

    這條守的是 F1 定案：`gate_kind` 只有 `PROBE` / `FULL_ENTRY`（出現 `EXECUTION` 即失敗）、
    `gate_basis` 一律輸出且只在既定值域內、**不得存在 `actual_rr_source`**
    （它與 `gate_basis` 一對一重複，是刻意不新增的欄位）。
    """
    gate = _gate_of([_zone(risk_reward_ratio=2.4)])

    assert gate["gate_kind"] == "PROBE"
    assert gate["gate_basis"] in GATE_BASIS_DOMAIN
    assert "zone_actual_rr" in gate
    assert "actual_rr_source" not in gate, "F1 定案：不新增這個欄位"


def test_secondary_gate_shares_actual_rr_and_only_differs_by_threshold():
    """`secondary_gate` **不得帶自己的 `actual_rr`**——兩層測的是同一個數字。

    這正是讓「通過」與「未達完整買進門檻」不再互相矛盾的關鍵性質：
    兩句話說的是同一個 RR 對上兩個不同門檻，而不是兩個不同的 RR。
    """
    # **tier 要用 TIER_3**：`_zone` 預設是 TIER_1，而 `_minimum_rr` 對 TIER_1 回 2.0，
    # probe 與 full entry 剛好同值就測不出「兩層門檻分岔」——那正是 0050 會踩到、
    # 而 Tier-1 zone 不會踩到的原因（見 T-055 的門檻分層表）。
    gate = _gate_of([_zone(
        risk_reward_ratio=1.8,
        tier=ZoneTier.TIER_3_SHORT_TERM.value,
        tier_label="短期",
    )])
    secondary = gate["secondary_gate"]

    assert secondary["gate_kind"] == "FULL_ENTRY"
    assert secondary["minimum_rr"] == 2.0
    assert "actual_rr" not in secondary, "兩層必須共用主 gate 的 actual_rr"
    # 1.8 通過 probe（1.5/1.8）但不到 full entry（2.0）——這就是原始現象。
    assert gate["qualified"] is True
    assert secondary["qualified"] is False


def test_limit_entry_without_target_keeps_setup_rr_verdict():
    """**限價路徑且無可量化 target 時，不得放寬既有守門**（2026-08-27 裁決方案 A）。

    契約原本只寫「target 未知 → qualified=true」，但那條語意只適用於市價路徑。
    這組 fixture 是單一 support zone、setup RR 1.49（低於 1.5 門檻）、前方沒有任何
    resistance：導入 execution gate 之前它是 `qualified=false`，
    **照搬那條語意會讓它變成 true**，等於把守門靜默拆掉。
    """
    gate = _gate_of([_zone(risk_reward_ratio=1.49, trading_score=95.0, confidence=0.9)])

    assert gate["qualified"] is False, "1.49 低於門檻，不得因為 target 未知就放行"
    assert gate["reason_code"] == "RR_INSUFFICIENT"
    assert gate["gate_basis"] == "ZONE_STATISTIC", "actual_rr 來自 zone 統計，要標明"
    assert gate["target_known"] is False
    assert gate["actual_rr"] == 1.49


def test_setup_and_executable_rr_are_separate_numbers():
    """`setup_rr` 與 `executable_rr` 是兩個數字，legacy alias 恆同值。"""
    ctx = _summary([_zone(risk_reward_ratio=2.4)])["rr_context"]

    assert ctx["setup_rr"] == ctx["entry_rr"] == 2.4
    assert ctx["executable_rr"] == ctx["execution_rr"]
    # 這組沒有前方壓力 → executable 未知，但 setup 仍在。
    assert ctx["executable_rr"] is None
    assert ctx["execution_target"] is None


def test_rr_below_full_entry_note_follows_secondary_gate_not_setup_rr():
    """`RR_BELOW_FULL_ENTRY` 要跟著 `secondary_gate`，不是跟著 setup RR（T-055）。

    0050 的形狀就是這條：setup RR **5.71**（照舊邏輯不會出這條 note）而
    executable RR **0.87**（secondary gate 不合格）。不校正的話，畫面會出現
    「gate 說未達完整買進門檻」但 risk_notes 一句都沒提。

    這裡用 `_final_entry_risk_notes` 直接驗校正邏輯——`_decision_action` 跑在
    execution gate 之前，端到端 fixture 反而看不出是誰決定的。
    """
    permission = {"state": "ALLOWED", "reason_codes": []}

    # ⚠️ **斷言文字不是 code**：`_final_entry_risk_notes` 回傳的是 `list[str]`
    # （最後一行 `[_risk_note_text(note) for note in notes]`），code 只存在於函式內部。

    # setup RR 高（沒有初始 note），但 secondary gate 不合格 → 必須補上
    # `actual_rr` 一定要給——這條 note 是「量到了，但不夠」，沒有數字就不掛（見下一支測試）。
    notes = _final_entry_risk_notes(
        [], permission,
        rr_gate={"actual_rr": 0.87,
                 "secondary_gate": {"gate_kind": "FULL_ENTRY", "minimum_rr": 2.0, "qualified": False}},
    )
    assert any("未達完整買進門檻" in n for n in notes), \
        "secondary gate 不合格時必須有 note，即使 setup RR 很高"

    # 反向：setup RR 低（有初始 note），但 secondary gate 合格 → 必須移除
    stale = [_risk_note("RR_BELOW_FULL_ENTRY", "風險報酬比未達完整買進門檻，最多小量試單。")]
    notes = _final_entry_risk_notes(
        stale, permission,
        rr_gate={"actual_rr": 5.71,
                 "secondary_gate": {"gate_kind": "FULL_ENTRY", "minimum_rr": 2.0, "qualified": True}},
    )
    assert not any("未達完整買進門檻" in n for n in notes), \
        "secondary gate 合格時不得留著依 setup RR 產生的過時 note"


def test_rr_below_full_entry_note_survives_text_rewrite_order():
    """校正必須發生在文字改寫**之前**。

    `BLOCKED` / `WAIT_CONFIRMATION` 時那個迴圈會把帶 code 的 dict 換成純字串，
    之後就對不回 `RR_BELOW_FULL_ENTRY`。順序寫反的話這條會失敗。
    """
    notes = _final_entry_risk_notes(
        [], {"state": "BLOCKED", "reason_codes": []},
        rr_gate={"actual_rr": 0.87,
                 "secondary_gate": {"gate_kind": "FULL_ENTRY", "minimum_rr": 2.0, "qualified": False}},
    )
    assert any("未達完整買進門檻" in n for n in notes)


def test_rr_below_full_entry_note_is_not_added_when_no_rr_exists():
    """`actual_rr=null` 時**不得**宣稱「未達門檻」。

    `NO_PRIMARY_ZONE` / `RR_UNAVAILABLE` 連 zone 統計都沒有，secondary gate 因此是
    `False`（那是對的，見 `test_secondary_gate_not_qualified_when_no_statistic_exists`），
    但「未達門檻」是需要數字支撐的判斷。掛上去會讓畫面同時出現
    「缺少風險報酬比」與「未達風險報酬比門檻」——兩句互相矛盾。
    """
    permission = {"state": "ALLOWED", "reason_codes": []}
    gate = {"actual_rr": None, "reason_code": "RR_UNAVAILABLE",
            "secondary_gate": {"gate_kind": "FULL_ENTRY", "minimum_rr": 2.0, "qualified": False}}

    notes = _final_entry_risk_notes(
        ["主交易區缺少風險報酬比，先觀察。"], permission, rr_gate=gate)
    assert any("缺少風險報酬比" in n for n in notes), "unavailable 文案要保留"
    assert not any("未達完整買進門檻" in n for n in notes), \
        "沒有 actual_rr 就不得宣稱未達門檻"

    # 連 `_decision_action` 依 setup RR 給的初值也要清掉——它同樣沒有 gate 數字撐。
    stale = [_risk_note("RR_BELOW_FULL_ENTRY", "風險報酬比未達完整買進門檻，最多小量試單。")]
    notes = _final_entry_risk_notes(stale, permission, rr_gate=gate)
    assert not any("未達完整買進門檻" in n for n in notes), \
        "actual_rr 缺席時，依 setup RR 給的初值也要移除"


# ── T-055 的核心資料流：限價 entry 的 target 封頂 ────────────────────────────
#
# `_execution_target → _rr_context → _execution_rr_gate` 這條路在既有測試裡完全沒有覆蓋——
# 現有的 FIRST_RESISTANCE_CAP 斷言都來自**市價 continuation** 路徑。
# 而 T-055 要修的成因二**正是限價路徑**（導入前它用 setup RR 反推 target）。

def test_limit_entry_target_is_capped_at_first_resistance_not_derived_from_setup_rr():
    """0050 限價 entry：target 必須是 **105.1862**，不是用 setup RR 反推的 105.2738。

    這是 T-055 計畫書點名的迴歸案例。三個 zone 裡符合「`price_low > entry_price`」的
    resistance 有 105.1862（主結構，擋路）與 107.1775（交易區），**取最低者**——
    target 不得穿越任何前方壓力。
    """
    ds = _summary(_zones_0050(), current_price=104.65)
    ctx = ds["rr_context"]

    # 前提：這組確實走限價路徑（不是市價型），否則測不到成因二。
    assert ctx["price_basis"] == "PRIMARY_SUPPORT_UPPER"

    assert ctx["target_price"] == pytest.approx(105.18616618688597)
    assert ctx["target_basis"] == "FIRST_RESISTANCE_CAP"
    assert ctx["execution_target"]["price"] == pytest.approx(105.18616618688597)
    assert ctx["execution_target"]["basis"] == "FIRST_RESISTANCE_CAP"

    # **不得退回 setup RR**：導入前 execution_rr 恆等於 entry_rr（1.8664）。
    assert ctx["setup_rr"] == pytest.approx(1.8664)
    assert ctx["executable_rr"] != pytest.approx(ctx["setup_rr"])
    # entry 104.1114 / stop 103.4886 / target 105.1862 → 約 1.726
    assert ctx["executable_rr"] == pytest.approx(1.726, abs=0.01)


def test_limit_entry_target_does_not_cross_blocking_resistance():
    """target **不得穿越**擋路壓力，且 `blocked=false` 時同樣封頂。

    `blocked` 只代表「近到足以擋單」，不代表可以把 target 設在更後面的壓力——
    兩件事的門檻不同。這條把那個定案釘住。
    """
    ds = _summary(_zones_0050(), current_price=104.65)
    blocking = ds["entry_blocking_zone"]
    target = ds["rr_context"]["target_price"]

    blocking_low = (blocking.get("blocking_zone") or {}).get("price_low")
    assert blocking_low is not None
    # 定案是「target 等於第一道壓力的 price_low」，所以是 **不得大於**，不是不得等於。
    assert target <= blocking_low


def test_limit_entry_gate_arbitrates_executable_rr_not_setup_rr():
    """gate 必須拿 executable RR 去比門檻，而不是 setup RR。"""
    ds = _summary(_zones_0050(), current_price=104.65)
    gate = ds["rr_gate"]

    assert gate["actual_rr"] == pytest.approx(ds["rr_context"]["executable_rr"])
    assert gate["zone_actual_rr"] == pytest.approx(ds["rr_context"]["setup_rr"])
    assert gate["gate_basis"] == "ENTRY_STOP_TARGET"
    assert gate["actual_rr"] != pytest.approx(gate["zone_actual_rr"])


def test_buy_is_downgraded_when_full_entry_gate_fails():
    """**不得同時出現 `action=Buy` 與 `secondary_gate.qualified=false`**。

    `strong` 讀的是 setup RR，而 execution gate 要到 `_final_action_from_entry` 之前才建立，
    所以「setup >= 2.0 但 executable < 2.0」會讓 Buy 與「完整部位門檻未通過」並存。
    T-055 明定這不得發生；**risk_notes 的事後校正只能改文字、改不了 action**。

    **直接測 `_final_action_from_entry`，不走端到端**：要讓端到端輸出 `Buy` 還得同時滿足
    `final_entry_permission=ALLOWED`（daily confirmation、executability…）等一長串條件，
    那些與本條要守的不變式無關，卻會讓 fixture 變得脆弱且難以判讀它到底測到了什麼。
    """
    allowed = {"state": "ALLOWED", "reason_codes": []}
    fails_full_entry = {"secondary_gate": {"gate_kind": "FULL_ENTRY", "minimum_rr": 2.0, "qualified": False}}
    passes_full_entry = {"secondary_gate": {"gate_kind": "FULL_ENTRY", "minimum_rr": 2.0, "qualified": True}}

    market, position, action, label = _final_action_from_entry(
        allowed, "BUY", "ADD", "Buy", "買進", fails_full_entry)
    assert action == "BuySmall", "full entry gate 未通過時，Buy 必須降級"
    assert label == "小量試單"
    assert market == "BUY_SMALL"

    # 反向：full entry gate 通過時不得誤降級
    _, _, action, label = _final_action_from_entry(
        allowed, "BUY", "ADD", "Buy", "買進", passes_full_entry)
    assert action == "Buy"
    assert label == "買進"

    # 舊分析沒有 secondary_gate → 安全降級成「不干預」
    _, _, action, _ = _final_action_from_entry(allowed, "BUY", "ADD", "Buy", "買進", {})
    assert action == "Buy"


def test_full_entry_gate_failure_is_reachable_with_capped_target():
    """確認上面那個不變式**測得到的情境真的存在**，不是理論。

    這組（把 0050 的主結構壓力挪到 105.25）會產生
    executable **1.83**——**通過 probe 門檻 1.8、但不到 full entry 的 2.0**。
    沒有這條，上面那支就只是在測一個永遠不會發生的組合。
    """
    zones = [
        _zone(role=ZoneType.SUPPORT.value, low=103.4886, high=104.1114,
              tier=ZoneTier.TIER_3_SHORT_TERM.value, tier_label="短期",
              confidence=0.9, risk_reward_ratio=3.2, trading_score=95.0, expected_value=0.08),
        _zone(role=ZoneType.RESISTANCE.value, low=105.25, high=110.998,
              tier=ZoneTier.TIER_1_MAIN_STRUCTURE.value, tier_label="主結構",
              confidence=0.6841, risk_reward_ratio=2.5514),
    ]
    ds = _summary(zones, current_price=104.65)
    gate = ds["rr_gate"]

    assert gate["qualified"] is True, "probe 門檻要通過，否則測不到降級那條路"
    assert gate["secondary_gate"]["qualified"] is False
    assert 1.8 <= ds["rr_context"]["executable_rr"] < 2.0
    # 不變式本身：這一輪不得輸出 Buy
    assert ds["action"] != "Buy"


def _continuation_full_entry_case(resistance_low: float) -> dict:
    """CONTINUATION 路徑（`test_semantic_pipeline_breakout_continuation_entry_allowed` 的形狀），
    加上一道 entry 之上的壓力，用它的 `price_low` 當 target 封頂。

    support 用 **TIER_3**（probe 門檻 1.8）——TIER_1 的 `_minimum_rr` 剛好是 2.0，
    與 full entry 門檻重合，probe 和 secondary 會同進同出，測不出兩層的差別。
    entry=105.0（市價）、stop 距離固定，所以 target 一動 RR 就跟著動。
    """
    zones = [
        _zone(low=98.0, high=100.0, risk_reward_ratio=2.8, confidence=0.78, trading_score=88.0,
              tier=ZoneTier.TIER_3_SHORT_TERM.value, tier_label="短期"),
        _zone(role=ZoneType.RESISTANCE.value, low=resistance_low, high=resistance_low + 4.0,
              tier=ZoneTier.TIER_1_MAIN_STRUCTURE.value, tier_label="主結構",
              confidence=0.7, risk_reward_ratio=2.4),
    ]
    previous = [{
        "type": "INTRADAY_RECLAIM", "event_family": "SUPPORT_RECLAIM", "event_scope": "ZONE",
        "zone_key": "SUPPORT:98.0000:100.0000", "root_event_type": "INTRADAY_RECLAIM",
        "latest_event_type": "INTRADAY_RECLAIM", "direction": "BULLISH", "state": "ACTIVE",
        "active": True, "age_bars": 1, "expires_after_bars": 3,
        "reason_codes": ["INTRADAY_RECLAIM"],
    }]
    return _summary(
        zones, current_price=105.0, candle_high=106.0, candle_low=102.0,
        candle_close=105.0, previous_candle_close=102.0, previous_event_states=previous,
    )


def test_full_entry_gate_caps_authoritative_permission_end_to_end():
    """**端到端不變式**：secondary gate 不合格時，`final_entry_permission` 不得停在
    `ENTRY_ALLOWED`。

    `final_entry_permission` 才是契約宣告的 authoritative 欄位。只把 deprecated 的
    `action` 降成 `BuySmall` 不夠——那樣畫面會同時出現「正式進場已放行」與
    「完整部位門檻未通過」。本測試同時釘住四個欄位彼此一致。

    ⚠️ 這裡刻意**不是** helper 層測試：`_final_entry_permission` 拿到的是 execution gate，
    但 `decision_derived_view` 的 semantic pipeline 拿到的是**更早的 setup-RR gate**
    （`decision_engine.py` 的呼叫順序），矛盾只在完整流程跑完才看得見。
    """
    ds = _continuation_full_entry_case(118.0)
    gate = ds["rr_gate"]
    permission = ds["final_entry_permission"]

    # 先確認真的踩在兩層門檻之間，否則測的是別的東西
    assert gate["minimum_rr"] == 1.8
    assert gate["qualified"] is True, "probe 要過，才測得到『probe 過但 full entry 沒過』"
    assert gate["secondary_gate"]["qualified"] is False
    assert 1.8 <= gate["actual_rr"] < 2.0

    # **兩個 authoritative 欄位必須一起降**（2026-08-27 裁決，選項 1）。
    # `decision_contract.authoritative_fields` 同時列了 `decision_derived_view` 與
    # `final_entry_permission`，只降後者會讓契約消費者在同一份輸出裡讀到兩種進場許可。
    #
    # ⚠️ 這條原本斷言 semantic **維持** ENTRY_ALLOWED，等於把矛盾釘成預期行為。
    # 「這條路徑本來就到得了 ENTRY_ALLOWED」改由下面的對照組證明，不是靠這裡。
    semantic = ds["decision_derived_view"]["semantic_pipeline"]
    assert semantic["entry_permission_state"] == "PROBE_ALLOWED", \
        "semantic pipeline 也要封頂，不能只降 final_entry_permission"
    assert "RR_BELOW_FULL_ENTRY" in semantic["reason_codes"]

    # 五個欄位一致
    assert permission["state"] == "PROBE_ALLOWED", "authoritative 欄位必須被封頂"
    assert "RR_BELOW_FULL_ENTRY" in permission["reason_codes"], "封頂要留下可追溯的理由"
    assert ds["action"] == "BuySmall"
    assert ds["market_action"] == "BUY_SMALL"
    assert any("未達完整買進門檻" in n for n in ds["risk_notes"])


def test_full_entry_gate_cap_control_group_would_otherwise_be_entry_allowed():
    """**對照組，不可刪**。

    同一個 fixture 只把壓力從 118.0 挪到 119.0（RR 剛好到 2.0），secondary gate 通過，
    `final_entry_permission` 就必須是 `ENTRY_ALLOWED`。沒有這條，上面那支在
    fixture 退化成「本來就到不了 ENTRY_ALLOWED」時會假綠。
    """
    ds = _continuation_full_entry_case(119.0)
    gate = ds["rr_gate"]

    assert gate["actual_rr"] >= 2.0
    assert gate["secondary_gate"]["qualified"] is True
    # 這兩條就是「封頂前本來是什麼」的證據：secondary 一過，兩個 authoritative
    # 欄位都回到 ENTRY_ALLOWED，證明上一支測到的 PROBE_ALLOWED 確實是封頂造成的。
    assert ds["decision_derived_view"]["semantic_pipeline"]["entry_permission_state"] == "ENTRY_ALLOWED"
    assert ds["final_entry_permission"]["state"] == "ENTRY_ALLOWED"
    assert "RR_BELOW_FULL_ENTRY" not in ds["final_entry_permission"]["reason_codes"]
    assert not any("未達完整買進門檻" in n for n in ds["risk_notes"])


def test_secondary_gate_not_qualified_when_no_statistic_exists():
    """沒有 primary zone / RR 時，full-entry gate **不得**標成通過。

    「未知不算不合格」只適用於 target 未知（這次算不出來），
    不適用於 `NO_PRIMARY_ZONE` / `RR_UNAVAILABLE`（連 zone 統計都不存在）——
    後者標成通過會讓畫面出現「完整部位門檻通過」而底下什麼都沒有。
    """
    ds = _summary([_zone(risk_reward_ratio=None)])
    gate = ds["rr_gate"]

    assert gate["qualified"] is False
    assert gate["reason_code"] == "RR_UNAVAILABLE"
    assert gate["secondary_gate"]["qualified"] is False, "沒有統計可比時不得標成通過"
