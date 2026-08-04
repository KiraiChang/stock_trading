from __future__ import annotations

import pandas as pd
import pytest

from .. import evidence as evidence_mod
from .. import pipeline as pipeline_mod
from .. import scoring
from ..features import trend_slope
from ..model import ModelBundle, train_model
from ..scoring import (
    CONFIDENCE_SAMPLE_PSEUDO_COUNT,
    TRADING_SCORE_WEIGHTS,
    TRADING_SCORE_WEIGHTS_NO_DIRECT_CHIP,
    _assign_tiers,
    _compute_global_metrics,
    _confidence,
    _derive_score,
    OVERLAP_GROUP_THRESHOLD,
    _group_overlapping_zones,
    _net_score_label,
    _zone_overlap_ratio,
    _normalize_probabilities,
    _pick_period_pair,
    _recent_validation,
    _sample_factor,
    _sort_zone_scores,
    _stability_factor,
    _touch_confidence,
    _trading_recommendation,
    _trading_score,
    _trading_score_breakdown,
    _trading_score_breakdown_no_direct_chip,
    _volume_confirmation,
    _zone_direction,
    score_symbol,
    score_zone,
)
from ..types import ApproachDirection, Zone, ZoneMethod, ZoneScore, ZoneTier, ZoneTouch, ZoneType
from .conftest import bullish_trend_df
from .test_model import synthetic_dataset


@pytest.fixture(scope="module")
def bundle() -> ModelBundle:
    return train_model(synthetic_dataset(), model_type="logistic_regression")


def _trend(df) -> float:
    return trend_slope(df, len(df) - 1)


def _zone_score_for_summary(
    *,
    low: float,
    high: float,
    role: str,
    trading_score: float,
    confidence: float = 0.7,
    confluence_family_count: int = 1,
) -> ZoneScore:
    return ZoneScore(
        price_low=low,
        price_high=high,
        method=ZoneMethod.ATR.value,
        role=role,
        tier=ZoneTier.TIER_2_TRADING_ZONE.value,
        tier_label="中期",
        support_score=0.0,
        resistance_score=0.0,
        net_score=0.0,
        net_score_label="NEUTRAL",
        confidence=confidence,
        confidence_level="MEDIUM",
        bounce_probability=None,
        break_probability=None,
        expected_gain=None,
        expected_loss=None,
        expected_value=None,
        risk_reward_ratio=None,
        reward_risk_percentile=None,
        relative_volume=None,
        volume_confirmation=None,
        touch_count=1,
        support_touch_count=1 if role == ZoneType.SUPPORT.value else 0,
        resistance_touch_count=1 if role == ZoneType.RESISTANCE.value else 0,
        reject_count=0,
        break_count=0,
        zone_momentum=0.0,
        zone_direction="NEUTRAL",
        recent_validation="NOT_TESTED_RECENTLY",
        trading_score=trading_score,
        trading_score_breakdown={},
        trading_recommendation="WATCH",
        overlap_group=None,
        confluence_count=confluence_family_count,
        confluence_family_count=confluence_family_count,
        confluence_families=(),
    )


def _v2_zone_scores(result: dict) -> list[dict]:
    return [item["score"] for item in result["zones"]]


def test_score_zone_scores_are_in_unit_interval(bundle):
    df = bullish_trend_df(n=80)
    low = float(df["close"].min())
    zone = Zone(price_low=low, price_high=low + 1.0, method=ZoneMethod.ATR, center_price=low + 0.5, formed_at_index=0)
    current_price = float(df["close"].iloc[-1])

    score = score_zone(df, zone, current_price, bundle, _trend(df))

    assert 0.0 <= score.support_score <= 1.0
    assert 0.0 <= score.resistance_score <= 1.0
    assert -1.0 <= score.net_score <= 1.0
    assert 0.0 <= score.confidence <= 1.0
    assert 0.0 <= score.trading_score <= 100.0
    if score.bounce_probability is not None:
        assert 0.0 <= score.bounce_probability <= 1.0
    if score.break_probability is not None:
        assert 0.0 <= score.break_probability <= 1.0


def test_score_zone_role_reflects_current_price(bundle):
    df = bullish_trend_df(n=80)
    current_price = float(df["close"].iloc[-1])
    trend = _trend(df)
    below_zone = Zone(
        price_low=current_price - 10, price_high=current_price - 8,
        method=ZoneMethod.ATR, center_price=current_price - 9, formed_at_index=0,
    )
    above_zone = Zone(
        price_low=current_price + 8, price_high=current_price + 10,
        method=ZoneMethod.ATR, center_price=current_price + 9, formed_at_index=0,
    )

    below_score = score_zone(df, below_zone, current_price, bundle, trend)
    above_score = score_zone(df, above_zone, current_price, bundle, trend)

    assert below_score.role == "SUPPORT"
    assert above_score.role == "RESISTANCE"
    assert below_score.bounce_probability is not None
    assert above_score.bounce_probability is not None
    # role=SUPPORT/RESISTANCE 一定要有明確的交易建議（不是 WATCH/NEUTRAL 之外皆可，
    # 只要求不是 AT_ZONE 專屬的空字串）
    assert below_score.trading_recommendation in (
        "STRONG_BUY", "BUY", "WATCH", "NEUTRAL", "AVOID", "STRONG_SELL",
    )


def test_score_zone_touch_count_splits_by_approach_direction(bundle):
    """touch_count 維持兩個方向加總（zone 整體活躍度），support_touch_count/
    resistance_touch_count 分開統計，兩者相加要等於 touch_count——確保拆分
    後的欄位跟舊欄位語意一致，不會不小心漏掉或重複計算某個方向的觸碰。"""
    df = bullish_trend_df(n=80)
    low = float(df["close"].min())
    zone = Zone(price_low=low, price_high=low + 1.0, method=ZoneMethod.ATR, center_price=low + 0.5, formed_at_index=0)
    current_price = float(df["close"].iloc[-1])

    score = score_zone(df, zone, current_price, bundle, _trend(df))

    assert score.support_touch_count >= 0
    assert score.resistance_touch_count >= 0
    assert score.touch_count == score.support_touch_count + score.resistance_touch_count


def test_score_zone_at_zone_has_no_probability(bundle):
    df = bullish_trend_df(n=80)
    current_price = float(df["close"].iloc[-1])
    zone = Zone(
        price_low=current_price - 1, price_high=current_price + 1,
        method=ZoneMethod.ATR, center_price=current_price, formed_at_index=0,
    )

    score = score_zone(df, zone, current_price, bundle, _trend(df))

    assert score.role == "AT_ZONE"
    assert score.bounce_probability is None
    assert score.break_probability is None
    assert score.expected_gain is None
    assert score.expected_loss is None
    assert score.expected_value is None
    assert score.risk_reward_ratio is None
    assert score.volume_confirmation is None


def test_score_symbol_returns_well_formed_zones(monkeypatch, bundle):
    df = bullish_trend_df(n=250)
    rows = [
        {
            "open": row["open"], "high": row["high"], "low": row["low"],
            "close": row["close"], "volume": row["volume"], "timestamp": int(ts.timestamp()),
        }
        for ts, row in df.iterrows()
    ]

    monkeypatch.setattr(scoring, "fetch_candles", lambda *a, **kw: rows)
    monkeypatch.setattr(scoring, "get_model", lambda: bundle)
    monkeypatch.setattr(scoring, "fetch_latest_chip_score", lambda *a, **kw: None)

    result = score_symbol("2330", "1d")

    assert result["pipeline_version"] == "v2"
    assert result["analysis"]["symbol"] == "2330"
    assert result["analysis"]["timeframe"] == "1d"
    assert set(result["features"]) >= {"global_trend", "global_volatility"}
    assert set(result["score"]) >= {
        "global_expected_value", "global_confidence", "global_risk_reward_ratio",
    }
    assert result["analysis"]["model"]["version"] == bundle.version
    assert result["analysis"]["model"]["trained_at"] == bundle.trained_at
    assert result["analysis"]["model"]["feature_names"] == bundle.feature_names
    assert result["analysis"]["model"]["config_hash"] == bundle.config_hash
    assert result["analysis"]["zone_builder_runtime_config"]["enabled"] is False
    assert result["analysis"]["zone_builder_runtime_config"]["reason_code"] == "ADAPTIVE_ZONE_BUILDERS_DISABLED"
    assert [p["key"] for p in result["analysis"]["period_summaries"]] == ["short", "mid", "long"]
    assert result["analysis"]["analysis_tips"]
    tips_text = "\n".join(result["analysis"]["analysis_tips"])
    for category in ("指標小辭典", "價位語意", "事件語意", "判讀提醒"):
        assert category in tips_text
    for product_copy in ("預設只列", "完整 zone", "硬湊數字"):
        assert product_copy not in tips_text
    assert result["analysis"]["chip_summary"]["missing"] is True
    assert result["evidence"]["model"]["explainer"] == "permutation_shap"
    assert set(result["explanation"]) >= {"schema_version", "summary", "action_reason", "market_drivers", "risk_notes", "model_context"}
    assert set(result["scenario"]) >= {"schema_version", "state", "title", "summary", "trigger_conditions", "invalidation_conditions"}
    assert result["scenario"]["schema_version"] == "sr_scenario_v1"
    assert set(result["probability_context"]) >= {"schema_version", "model_metrics", "health"}
    assert result["probability_context"]["schema_version"] == "sr_probability_context_v1"
    ds = result["decision"]
    assert ds["market_regime"]["primary"] in ("TREND_UP", "TREND_DOWN", "RANGE_BOUND")
    assert isinstance(ds["market_regime"]["flags"], list)
    assert ds["action"] in ("Buy", "BuySmall", "Hold", "Avoid")
    assert "market_context" in ds and isinstance(ds["market_context"], list)
    assert "confidence_explanation" in ds
    assert set(ds["confidence_explanation"].keys()) >= {"value", "level", "label", "formula_factors", "context_factors"}
    assert isinstance(ds["secondary_zones"], list)
    if ds["primary_zone"] is not None:
        assert ds["primary_zone"]["role"] in ("SUPPORT", "RESISTANCE")
        assert "distance_pct" in ds["primary_zone"]
    assert isinstance(result["zones"], list)
    zones = _v2_zone_scores(result)
    for item, z in zip(result["zones"], zones):
        assert set(item) == {"data", "features", "score", "evidence", "explanation", "scenario", "probability_context", "lifecycle"}
        assert set(item["evidence"]) >= {"support", "resistance", "risk_flags"}
        assert set(item["explanation"]) >= {"schema_version", "role_summary", "score_reason", "probability_reason", "confidence_reason"}
        assert set(item["scenario"]) >= {"schema_version", "state", "title", "summary", "trigger_conditions", "invalidation_conditions"}
        assert item["scenario"]["schema_version"] == "sr_scenario_v1"
        assert set(item["probability_context"]) >= {"schema_version", "bounce_probability", "break_probability", "neutral_probability", "dominant_outcome", "edge_pp", "quality_flags"}
        assert item["probability_context"]["schema_version"] == "sr_probability_context_v1"
        assert "advanced_refs" not in item["explanation"]
        assert 0.0 <= z["support_score"] <= 1.0
        assert 0.0 <= z["resistance_score"] <= 1.0
        assert z["role"] in ("SUPPORT", "RESISTANCE", "AT_ZONE")
        assert z["tier"] in ("TIER_1_MAIN_STRUCTURE", "TIER_2_TRADING_ZONE", "TIER_3_SHORT_TERM")
        assert z["net_score_label"] in ("STRONG_SUPPORT", "NEUTRAL", "STRONG_RESISTANCE")
        assert z["confidence_level"] in ("LOW", "MEDIUM", "HIGH", "VERY_HIGH")
        assert z["recent_validation"] in (
            "VALIDATED_RECENTLY", "PENDING_VALIDATION", "NOT_TESTED_RECENTLY", "EXPIRED",
        )
        assert z["zone_direction"] in ("UP", "DOWN", "FLAT")
        assert set(z["trading_score_breakdown"].keys()) == {
            "expected_value", "risk_reward", "trend", "volume", "confidence", "chip",
        }
        assert z["trading_score"] == pytest.approx(sum(z["trading_score_breakdown"].values()))
        assert z["support_touch_count"] + z["resistance_touch_count"] == z["touch_count"]
        assert z["confluence_count"] >= 1
        if z["confluence_count"] == 1:
            assert z["overlap_group"] is None
        # global_trend/global_volatility 不應該在每個 zone 裡重複出現
        assert "global_trend" not in z
        assert "global_volatility" not in z
        assert "trend_strength" not in z
        assert "volatility" not in z

    # zones 必須可排序：tier 由粗到細，同層內 trading_score 由高到低
    tier_order = {"TIER_1_MAIN_STRUCTURE": 1, "TIER_2_TRADING_ZONE": 2, "TIER_3_SHORT_TERM": 3}
    ranks = [tier_order[z["tier"]] for z in zones]
    assert ranks == sorted(ranks)
    for i in range(len(zones) - 1):
        a, b = zones[i], zones[i + 1]
        if a["tier"] == b["tier"]:
            assert a["trading_score"] >= b["trading_score"]


def test_score_symbol_raises_when_no_candles(monkeypatch):
    monkeypatch.setattr(scoring, "fetch_candles", lambda *a, **kw: [])
    with pytest.raises(ValueError):
        score_symbol("2330")


# ── confidence（多因子）/ probability normalization / score derivation ──


def test_sample_factor_zero_touches_is_zero():
    assert _sample_factor(0) == 0.0


def test_sample_factor_increases_with_touch_count():
    low = _sample_factor(1)
    mid = _sample_factor(CONFIDENCE_SAMPLE_PSEUDO_COUNT)
    high = _sample_factor(50)
    assert 0.0 < low < mid < high < 1.0
    assert mid == pytest.approx(0.5)


def test_stability_factor_neutral_when_no_history():
    assert _stability_factor(0, 0) == pytest.approx(0.5)


def test_stability_factor_high_when_consistent():
    assert _stability_factor(hold_count=8, break_count=0) == pytest.approx(1.0)
    assert _stability_factor(hold_count=4, break_count=4) == pytest.approx(0.5)


def test_confidence_combines_three_factors():
    # touch_count=0、從未測試過、無歷史結果 → 三個因子分別是 0, 0, 0.5
    never_touched = _confidence(touch_count=0, bars_since_last_touch=None, hold_count=0, break_count=0)
    assert never_touched == pytest.approx((0.0 + 0.0 + 0.5) / 3.0)

    # 樣本多、剛測試過、結果一致 → confidence 應該明顯更高
    well_validated = _confidence(
        touch_count=20, bars_since_last_touch=0, hold_count=10, break_count=0
    )
    assert well_validated > never_touched
    assert well_validated > 0.7


def _touch(idx: int, direction: ApproachDirection) -> ZoneTouch:
    zone = Zone(price_low=95.0, price_high=100.0, method=ZoneMethod.ATR, center_price=97.5, formed_at_index=0)
    role = ZoneType.SUPPORT if direction == ApproachDirection.FROM_ABOVE else ZoneType.RESISTANCE
    return ZoneTouch(zone=zone, touch_index=idx, touch_time=None, touch_price=97.5, approach_direction=direction, role=role)


def test_touch_confidence_high_when_direction_specific_history_is_consistent():
    touches = [_touch(i, ApproachDirection.FROM_ABOVE) for i in range(10)]
    classified = [(t, 1, 0, 0.02) for t in touches]  # 全部守住（hold_label=1）

    confidence = _touch_confidence(touches, classified, as_of_index=15)

    assert confidence > 0.7


def test_touch_confidence_differs_from_mixed_direction_calculation():
    """role=SUPPORT 的 confidence 只該用 support 方向的觸碰計算——這是拆分
    touch_count 語意前的行為缺陷：以前不分方向，直接把兩個方向的樣本/歷史
    結果混在一起，導致「作為支撐」的可信度被「作為壓力」的表現拖累或拉抬。
    這裡驗證兩種算法確實會得到不同結果，防止之後不小心又改回混合計算。"""
    support_touches = [_touch(i, ApproachDirection.FROM_ABOVE) for i in range(10)]
    support_classified = [(t, 1, 0, 0.02) for t in support_touches]  # 全部守住

    resistance_touches = [_touch(20 + i, ApproachDirection.FROM_BELOW) for i in range(4)]
    # 4 次裡 3 次跌破、1 次守住：跟 support 方向的表現不一致
    resistance_classified = [
        (resistance_touches[0], 1, 0, 0.01),
        (resistance_touches[1], 0, 1, -0.02),
        (resistance_touches[2], 0, 1, -0.02),
        (resistance_touches[3], 0, 1, -0.02),
    ]

    as_of_index = 25
    support_only = _touch_confidence(support_touches, support_classified, as_of_index)
    mixed = _touch_confidence(
        support_touches + resistance_touches, support_classified + resistance_classified, as_of_index
    )

    assert support_only != pytest.approx(mixed)


def test_normalize_probabilities_rescales_when_sum_exceeds_one():
    hold, brk = _normalize_probabilities(0.7, 0.5)
    assert hold + brk == pytest.approx(1.0)
    assert hold == pytest.approx(0.7 / 1.2)
    assert brk == pytest.approx(0.5 / 1.2)


def test_normalize_probabilities_unchanged_when_sum_within_range():
    hold, brk = _normalize_probabilities(0.3, 0.2)
    assert hold == pytest.approx(0.3)
    assert brk == pytest.approx(0.2)


def test_derive_score_at_zero_confidence_is_neutral():
    assert _derive_score(hold_probability=0.95, confidence=0.0) == pytest.approx(0.5)


def test_derive_score_at_full_confidence_matches_probability():
    assert _derive_score(hold_probability=0.95, confidence=1.0) == pytest.approx(0.95)


def test_derive_score_partial_confidence_is_between_neutral_and_probability():
    score = _derive_score(hold_probability=0.9, confidence=0.4)
    assert 0.5 < score < 0.9


# ── net_score / zone_direction / volume_confirmation / recent_validation ─


def test_net_score_label_thresholds():
    assert _net_score_label(0.5) == "STRONG_SUPPORT"
    assert _net_score_label(0.0) == "NEUTRAL"
    assert _net_score_label(-0.5) == "STRONG_RESISTANCE"


def test_zone_direction_thresholds():
    assert _zone_direction(0.05) == "UP"
    assert _zone_direction(-0.05) == "DOWN"
    assert _zone_direction(0.0) == "FLAT"


def test_volume_confirmation_confirmed_when_high_volume_and_recently_validated():
    assert _volume_confirmation(1.5, "VALIDATED_RECENTLY") == "CONFIRMED"


def test_volume_confirmation_failed_when_high_volume_and_expired():
    assert _volume_confirmation(1.5, "EXPIRED") == "FAILED"


def test_volume_confirmation_weak_when_low_volume():
    assert _volume_confirmation(0.3, "PENDING_VALIDATION") == "WEAK"


def test_recent_validation_pending_when_never_touched():
    assert _recent_validation([], [], as_of_index=100) == "PENDING_VALIDATION"


def test_recent_validation_reclaimed_support_hold_is_not_pending():
    zone = Zone(price_low=28.06, price_high=28.37, method=ZoneMethod.ATR, center_price=28.215, formed_at_index=0)
    touch = ZoneTouch(
        zone=zone,
        touch_index=98,
        touch_time=pd.Timestamp("2026-07-15"),
        touch_price=28.10,
        approach_direction=ApproachDirection.FROM_ABOVE,
        role=ZoneType.SUPPORT,
    )

    assert _recent_validation([touch], [(touch, 1, 0, 0.02)], as_of_index=100) == "VALIDATED_RECENTLY"


def test_score_zone_validation_window_starts_after_zone_generation(bundle):
    df = pd.DataFrame(
        {
            "open": [30.0, 30.0, 29.0, 29.0, 28.2, 28.6, 28.8, 29.0],
            "high": [30.2, 29.5, 29.2, 29.1, 29.3, 29.0, 29.2, 29.4],
            "low": [29.8, 27.9, 28.8, 28.7, 27.34, 28.5, 28.7, 28.9],
            "close": [30.0, 28.5, 29.0, 29.0, 28.5, 28.8, 29.0, 29.1],
            "volume": [1000, 1000, 1000, 1000, 1300, 1200, 1100, 1000],
        },
        index=pd.date_range("2026-07-08", periods=8, freq="D"),
    )
    zone = Zone(price_low=28.06, price_high=28.37, method=ZoneMethod.ATR, center_price=28.215, formed_at_index=2)

    score = score_zone(
        df,
        zone,
        current_price=29.1,
        bundle=bundle,
        overall_trend=_trend(df),
        as_of_index=7,
        lookback_bars=8,
        forward_bars=2,
        threshold_pct=0.005,
    )

    assert score.recent_validation != "PENDING_VALIDATION"
    assert score.validation_debug["zone_generation_end_date"] == "2026-07-10"
    assert score.validation_debug["validation_start_date"] == "2026-07-11"
    assert score.validation_debug["latest_validation_bar_date"] == "2026-07-12"


def test_trading_recommendation_support_strong_buy_at_high_score():
    assert _trading_recommendation(90.0, "SUPPORT") == "STRONG_BUY"


def test_trading_recommendation_resistance_strong_sell_at_high_score():
    assert _trading_recommendation(90.0, "RESISTANCE") == "STRONG_SELL"


def test_trading_recommendation_at_zone_is_watch_or_neutral():
    assert _trading_recommendation(70.0, "AT_ZONE") == "WATCH"
    assert _trading_recommendation(10.0, "AT_ZONE") == "NEUTRAL"


# ── 十一、Zone Tier（可排序）──────────────────────────────────────────


def test_assign_tiers_widest_is_tier_1_narrowest_is_tier_3():
    # 9 個 zone，寬度由大到小：tier 應該剛好各 3 個
    widths = [33.0, 30.0, 27.0, 9.0, 8.0, 7.0, 4.0, 3.0, 2.0]
    tiers = _assign_tiers(widths)
    assert tiers[0:3] == ["TIER_1_MAIN_STRUCTURE"] * 3
    assert tiers[3:6] == ["TIER_2_TRADING_ZONE"] * 3
    assert tiers[6:9] == ["TIER_3_SHORT_TERM"] * 3


def test_assign_tiers_empty_input():
    assert _assign_tiers([]) == []


def test_assign_tiers_preserves_input_order():
    # 回傳值要跟輸入順序一一對應，不是排序後的結果
    widths = [2.0, 33.0, 9.0]
    tiers = _assign_tiers(widths)
    assert tiers[1] == "TIER_1_MAIN_STRUCTURE"  # 33.0 最寬
    assert tiers[0] == "TIER_3_SHORT_TERM"  # 2.0 最窄


# ── 十六：跨方法重疊分群（confluence）────────────────────────────────


def _zone(low: float, high: float, method: ZoneMethod) -> Zone:
    return Zone(price_low=low, price_high=high, method=method, center_price=(low + high) / 2, formed_at_index=0)


def test_group_overlapping_zones_groups_cross_method_overlap():
    zones = [
        _zone(100.0, 110.0, ZoneMethod.ATR),           # 0：跟 1 高度重疊（跨方法）
        _zone(101.0, 109.0, ZoneMethod.VOLUME_PROFILE),  # 1
        _zone(200.0, 210.0, ZoneMethod.ATR),           # 2：獨立，沒有重疊
    ]

    groups, confluence, family_counts, families = _group_overlapping_zones(zones)

    assert groups[0] == groups[1]
    assert groups[0] is not None
    assert confluence[0] == 2 and confluence[1] == 2
    assert family_counts[0] == 2 and family_counts[1] == 2
    assert families[0] == ("STRUCTURAL_ATR", "VOLUME_PROFILE")
    assert groups[2] is None
    assert confluence[2] == 1
    assert family_counts[2] == 1


def test_group_overlapping_zones_ignores_same_method_overlap():
    """同一種方法建出來的 zone 已經在各自 builder 內做過合併，
    _group_overlapping_zones 只處理跨方法重疊，同方法重疊不分群。"""
    zones = [
        _zone(100.0, 110.0, ZoneMethod.ATR),
        _zone(101.0, 109.0, ZoneMethod.ATR),
    ]

    groups, confluence, family_counts, families = _group_overlapping_zones(zones)

    assert groups == [None, None]
    assert confluence == [1, 1]
    assert family_counts == [1, 1]


def test_group_overlapping_zones_below_threshold_not_grouped():
    # overlap 只有 20%（相對於較窄 zone 寬度），低於 0.6 門檻
    zones = [
        _zone(100.0, 110.0, ZoneMethod.ATR),
        _zone(108.0, 118.0, ZoneMethod.VOLUME_PROFILE),
    ]

    groups, confluence, family_counts, families = _group_overlapping_zones(zones)

    assert groups == [None, None]
    assert confluence == [1, 1]
    assert family_counts == [1, 1]


def test_group_overlapping_zones_transitively_connected_zones_share_one_group():
    # A-B overlap ratio 0.7、B-C overlap ratio 0.7，A-C 本身只有 0.4（不到
    # 門檻），但透過 B 仍應被歸為同一群組（union-find 的傳遞性）
    zones = [
        _zone(100.0, 110.0, ZoneMethod.ATR),             # A
        _zone(103.0, 113.0, ZoneMethod.VOLUME_PROFILE),  # B：跟 A、C 都重疊
        _zone(106.0, 116.0, ZoneMethod.ATR),             # C
    ]
    assert _zone_overlap_ratio(zones[0], zones[2]) < OVERLAP_GROUP_THRESHOLD  # 前提：A-C 本身不到門檻

    groups, confluence, family_counts, families = _group_overlapping_zones(zones)

    assert groups[0] == groups[1] == groups[2]
    assert confluence == [3, 3, 3]
    assert family_counts == [2, 2, 2]


def test_group_overlapping_zones_deduplicates_correlated_evidence_families():
    zones = [
        _zone(100.0, 110.0, ZoneMethod.RECENT_PIVOT),
        _zone(101.0, 109.0, ZoneMethod.BREAKDOWN_RECLAIM),
        _zone(102.0, 108.0, ZoneMethod.VWAP_RECLAIM),
        _zone(100.5, 109.5, ZoneMethod.ATR),
    ]

    groups, confluence, family_counts, families = _group_overlapping_zones(zones)

    assert confluence == [4, 4, 4, 4]
    assert family_counts == [3, 3, 3, 3]
    assert set(families[0]) == {
        "RECENT_MICROSTRUCTURE",
        "VWAP_OR_AVERAGE_RECLAIM",
        "STRUCTURAL_ATR",
    }


def test_score_symbol_confluence_reflects_cross_method_overlap(monkeypatch, bundle):
    """整合測試：直接建構兩個跨方法重疊的 zone（monkeypatch 掉 builder），
    確認 score_symbol() 輸出的 confluence_count/overlap_group 正確反映。"""
    df = bullish_trend_df(n=250)
    rows = [
        {
            "open": row["open"], "high": row["high"], "low": row["low"],
            "close": row["close"], "volume": row["volume"], "timestamp": int(ts.timestamp()),
        }
        for ts, row in df.iterrows()
    ]
    low = float(df["close"].min())

    class _FixedBuilder:
        def __init__(self, method: ZoneMethod, low_offset: float, high_offset: float):
            self._method = method
            self._low = low + low_offset
            self._high = low + high_offset

        @property
        def min_bars(self) -> int:
            return 1

        def build(self, df):
            return [Zone(price_low=self._low, price_high=self._high, method=self._method, center_price=(self._low + self._high) / 2, formed_at_index=0)]

    monkeypatch.setattr(scoring, "fetch_candles", lambda *a, **kw: rows)
    monkeypatch.setattr(scoring, "get_model", lambda: bundle)
    monkeypatch.setattr(scoring, "fetch_latest_chip_score", lambda *a, **kw: None)

    overlapping_builders = [
        _FixedBuilder(ZoneMethod.ATR, 0.0, 2.0),
        _FixedBuilder(ZoneMethod.VOLUME_PROFILE, 0.2, 1.8),
    ]
    result = score_symbol("2330", "1d", builders=overlapping_builders)
    zones = _v2_zone_scores(result)
    assert len(zones) == 2
    assert zones[0]["confluence_count"] == 2
    assert zones[1]["confluence_count"] == 2
    assert zones[0]["confluence_family_count"] == 2
    assert set(zones[0]["confluence_families"]) == {"STRUCTURAL_ATR", "VOLUME_PROFILE"}
    assert zones[0]["overlap_group"] == zones[1]["overlap_group"]
    assert zones[0]["overlap_group"] is not None


# ── 十三、Trading Score（可拆解）─────────────────────────────────────


def test_trading_score_breakdown_weights_sum_to_100():
    assert sum(TRADING_SCORE_WEIGHTS.values()) == pytest.approx(100.0)


def test_trading_score_no_direct_chip_weights_sum_to_100():
    assert sum(TRADING_SCORE_WEIGHTS_NO_DIRECT_CHIP.values()) == pytest.approx(100.0)
    assert "chip" not in TRADING_SCORE_WEIGHTS_NO_DIRECT_CHIP


def test_trading_score_breakdown_keys_match_weights():
    breakdown = _trading_score_breakdown(
        role="SUPPORT", confidence=0.8, expected_value=0.02, risk_reward_ratio=1.5,
        overall_trend=0.05, volume_confirmation="CONFIRMED",
    )
    assert set(breakdown.keys()) == set(TRADING_SCORE_WEIGHTS.keys())


def test_trading_score_equals_sum_of_breakdown():
    breakdown = _trading_score_breakdown(
        role="RESISTANCE", confidence=0.5, expected_value=-0.01, risk_reward_ratio=0.8,
        overall_trend=-0.02, volume_confirmation="WEAK",
    )
    assert _trading_score(breakdown) == pytest.approx(sum(breakdown.values()))


def test_trading_score_no_direct_chip_shadow_policy_redistributes_weights():
    breakdown = _trading_score_breakdown_no_direct_chip(
        role="SUPPORT", confidence=0.8, expected_value=0.02, risk_reward_ratio=1.5,
        overall_trend=0.05, volume_confirmation="CONFIRMED",
    )
    assert set(breakdown.keys()) == set(TRADING_SCORE_WEIGHTS_NO_DIRECT_CHIP.keys())
    assert "chip" not in breakdown

    production_neutral = _trading_score_breakdown(
        role="SUPPORT", confidence=0.8, expected_value=0.02, risk_reward_ratio=1.5,
        overall_trend=0.05, volume_confirmation="CONFIRMED", chip_score=None,
    )
    assert breakdown["expected_value"] == pytest.approx(
        production_neutral["expected_value"]
        / TRADING_SCORE_WEIGHTS["expected_value"]
        * TRADING_SCORE_WEIGHTS_NO_DIRECT_CHIP["expected_value"]
    )


def test_trading_score_breakdown_uses_neutral_defaults_when_role_unresolved():
    # AT_ZONE 或缺值時，EV/RR/Volume 分量該用中性值 0.5 計算，而不是 0
    breakdown = _trading_score_breakdown(
        role="AT_ZONE", confidence=0.6, expected_value=None, risk_reward_ratio=None,
        overall_trend=0.0, volume_confirmation=None,
    )
    assert breakdown["expected_value"] == pytest.approx(0.5 * TRADING_SCORE_WEIGHTS["expected_value"])
    assert breakdown["risk_reward"] == pytest.approx(0.5 * TRADING_SCORE_WEIGHTS["risk_reward"])
    assert breakdown["volume"] == pytest.approx(0.5 * TRADING_SCORE_WEIGHTS["volume"])


def test_trading_score_breakdown_confidence_component_is_direct():
    breakdown = _trading_score_breakdown(
        role="SUPPORT", confidence=1.0, expected_value=None, risk_reward_ratio=None,
        overall_trend=0.0, volume_confirmation=None,
    )
    assert breakdown["confidence"] == pytest.approx(TRADING_SCORE_WEIGHTS["confidence"])


def test_period_summary_prefers_nearby_support_over_far_slightly_higher_score():
    current_price = 100.0
    far_high_score = _zone_score_for_summary(
        low=70.0, high=72.0, role=ZoneType.SUPPORT.value, trading_score=82.0, confidence=0.75
    )
    nearby = _zone_score_for_summary(
        low=94.0, high=95.0, role=ZoneType.SUPPORT.value, trading_score=74.0, confidence=0.75
    )

    support, _ = _pick_period_pair([far_high_score, nearby], current_price)

    assert support is nearby


def test_period_summary_prefers_nearby_resistance_over_far_slightly_higher_score():
    current_price = 100.0
    far_high_score = _zone_score_for_summary(
        low=128.0, high=130.0, role=ZoneType.RESISTANCE.value, trading_score=82.0, confidence=0.75
    )
    nearby = _zone_score_for_summary(
        low=105.0, high=106.0, role=ZoneType.RESISTANCE.value, trading_score=74.0, confidence=0.75
    )

    _, resistance = _pick_period_pair([far_high_score, nearby], current_price)

    assert resistance is nearby


# ── 【2026-07 籌碼分析整合】chip 分量 ──────────────────────────────────


def test_trading_score_breakdown_chip_missing_data_is_neutral():
    # 查無籌碼資料（chip_score=None，預設值）時該分量用中性值 0.5，不能是 0
    breakdown = _trading_score_breakdown(
        role="SUPPORT", confidence=0.5, expected_value=None, risk_reward_ratio=None,
        overall_trend=0.0, volume_confirmation=None,
    )
    assert breakdown["chip"] == pytest.approx(0.5 * TRADING_SCORE_WEIGHTS["chip"])


def test_trading_score_breakdown_chip_bullish_increases_support_score():
    neutral = _trading_score_breakdown(
        role="SUPPORT", confidence=0.5, expected_value=None, risk_reward_ratio=None,
        overall_trend=0.0, volume_confirmation=None, chip_score=None,
    )
    bullish = _trading_score_breakdown(
        role="SUPPORT", confidence=0.5, expected_value=None, risk_reward_ratio=None,
        overall_trend=0.0, volume_confirmation=None, chip_score=80.0,
    )
    bearish = _trading_score_breakdown(
        role="SUPPORT", confidence=0.5, expected_value=None, risk_reward_ratio=None,
        overall_trend=0.0, volume_confirmation=None, chip_score=-80.0,
    )
    assert bullish["chip"] > neutral["chip"] > bearish["chip"]
    # cap=100 時，chip_score=80 應正規化為 (80+100)/200=0.9
    assert bullish["chip"] == pytest.approx(0.9 * TRADING_SCORE_WEIGHTS["chip"])


def test_trading_score_breakdown_chip_sign_flips_for_resistance():
    """籌碼偏多對 SUPPORT 是加分（有支撐買盤），但對 RESISTANCE 是減分
    （買盤較強，壓力較容易被站上），跟 trend 分量的角色翻轉邏輯一致。"""
    support = _trading_score_breakdown(
        role="SUPPORT", confidence=0.5, expected_value=None, risk_reward_ratio=None,
        overall_trend=0.0, volume_confirmation=None, chip_score=80.0,
    )
    resistance = _trading_score_breakdown(
        role="RESISTANCE", confidence=0.5, expected_value=None, risk_reward_ratio=None,
        overall_trend=0.0, volume_confirmation=None, chip_score=80.0,
    )
    assert support["chip"] > TRADING_SCORE_WEIGHTS["chip"] * 0.5
    assert resistance["chip"] < TRADING_SCORE_WEIGHTS["chip"] * 0.5
    assert support["chip"] + resistance["chip"] == pytest.approx(TRADING_SCORE_WEIGHTS["chip"])


def test_trading_score_breakdown_chip_at_zone_uses_unflipped_sign():
    at_zone = _trading_score_breakdown(
        role="AT_ZONE", confidence=0.5, expected_value=None, risk_reward_ratio=None,
        overall_trend=0.0, volume_confirmation=None, chip_score=80.0,
    )
    assert at_zone["chip"] == pytest.approx(0.9 * TRADING_SCORE_WEIGHTS["chip"])


def test_score_symbol_passes_chip_score_from_db_into_breakdown(monkeypatch, bundle):
    """整合測試：score_symbol 應該只查一次 fetch_latest_chip_score（股票層級，
    比照 global_trend 的做法），並把結果套進每個 zone 的 trading_score_breakdown。"""
    df = bullish_trend_df(n=250)
    rows = [
        {
            "open": row["open"], "high": row["high"], "low": row["low"],
            "close": row["close"], "volume": row["volume"], "timestamp": int(ts.timestamp()),
        }
        for ts, row in df.iterrows()
    ]
    low = float(df["close"].min())

    call_count = {"n": 0}
    captured = {}

    def _fake_fetch_chip(symbol, before_date=None, **kw):
        call_count["n"] += 1
        assert symbol == "2330"
        captured["before_date"] = before_date
        return {"symbol": "2330", "total_score": 60.0}

    monkeypatch.setattr(scoring, "fetch_candles", lambda *a, **kw: rows)
    monkeypatch.setattr(scoring, "get_model", lambda: bundle)
    monkeypatch.setattr(scoring, "fetch_latest_chip_score", _fake_fetch_chip)

    zone = Zone(price_low=low, price_high=low + 1.0, method=ZoneMethod.ATR, center_price=low + 0.5, formed_at_index=0)
    result = score_symbol("2330", "1d", builders=[_SingleZoneBuilder(zone)])

    assert call_count["n"] == 1  # 股票層級只查一次，不對每個 zone 重複查詢
    assert len(result["zones"]) == 1
    z = _v2_zone_scores(result)[0]
    # chip_score=60 正規化為 (60+100)/200=0.8；zone 在最低價附近，role=SUPPORT，
    # 不需要翻轉正負號
    assert z["role"] == "SUPPORT"
    assert z["trading_score_breakdown"]["chip"] == pytest.approx(0.8 * TRADING_SCORE_WEIGHTS["chip"])

    # 【review 修復】必須帶 before_date，且要對齊這次分析最後一根K棒換算
    # 成 Asia/Taipei 之後的交易日，不能省略（省略會撈到資料庫最新一筆
    # chip_scores，可能是「未來」的籌碼資料，見 lookahead bias 說明）。
    expected_before_date = pd.Timestamp(result["analysis"]["analyzed_at"]).tz_convert("Asia/Taipei").strftime("%Y-%m-%d")
    assert captured["before_date"] == expected_before_date


def test_score_symbol_applies_latest_regression_governance_gate(monkeypatch, bundle):
    df = bullish_trend_df(n=250)
    rows = [
        {
            "open": row["open"], "high": row["high"], "low": row["low"],
            "close": row["close"], "volume": row["volume"], "timestamp": int(ts.timestamp()),
        }
        for ts, row in df.iterrows()
    ]

    monkeypatch.setattr(scoring, "fetch_candles", lambda *a, **kw: rows)
    monkeypatch.setattr(scoring, "get_model", lambda: bundle)
    monkeypatch.setattr(scoring, "fetch_latest_chip_score", lambda *a, **kw: None)
    monkeypatch.setattr(pipeline_mod, "fetch_latest_sr_regression_governance", lambda model_config_hash: {
        "source": "LATEST_REGRESSION_RESULT",
        "run_id": "sr_replay_gate_001",
        "model_config_hash": model_config_hash,
        "health_state": "UNRELIABLE",
        "passed": False,
        "strict_passed": False,
        "confidence_gate": {
            "state": "UNRELIABLE",
            "allow_entry": False,
            "max_entry_state": "WAIT_CONFIRMATION",
            "reason_codes": ["ENTRY_OUTCOME_NEGATIVE"],
        },
    })

    result = score_symbol("2330", "1d")

    health = result["probability_context"]["health"]
    assert health["health_state"] == "UNRELIABLE"
    assert health["confidence_gate"]["allow_entry"] is False
    assert health["confidence_gate"]["max_entry_state"] == "WAIT_CONFIRMATION"
    assert health["regression_governance"]["run_id"] == "sr_replay_gate_001"
    assert "REGRESSION_GOVERNANCE_UNRELIABLE" in health["blocking_flags"]

    decision = result["decision"]
    assert decision["model_governance"]["health_state"] == "UNRELIABLE"
    assert decision["model_governance"]["confidence_gate"]["allow_entry"] is False
    assert decision["model_governance"]["regression_governance"]["run_id"] == "sr_replay_gate_001"
    assert decision["final_entry_permission"]["state"] != "ENTRY_ALLOWED"


def test_score_symbol_uses_adaptive_zone_builder_config_when_enabled(monkeypatch, bundle):
    df = bullish_trend_df(n=250)
    rows = [
        {
            "open": row["open"], "high": row["high"], "low": row["low"],
            "close": row["close"], "volume": row["volume"], "timestamp": int(ts.timestamp()),
        }
        for ts, row in df.iterrows()
    ]

    monkeypatch.setattr(scoring, "fetch_candles", lambda *a, **kw: rows)
    monkeypatch.setattr(scoring, "get_model", lambda: bundle)
    monkeypatch.setattr(scoring, "fetch_latest_chip_score", lambda *a, **kw: None)
    monkeypatch.setattr(scoring, "_adaptive_zone_builder_enabled", lambda: True)
    monkeypatch.setattr(scoring, "_adaptive_zone_builder_profile", lambda frame: (0.04, 0.02))

    result = score_symbol("2330", "1d")

    runtime_config = result["analysis"]["zone_builder_runtime_config"]
    atr_config = runtime_config["config"]["ATRZoneBuilder"]
    assert runtime_config["enabled"] is True
    assert runtime_config["bucket"] == "HIGH_VOLATILITY"
    assert runtime_config["reason_code"] == "VOLATILITY_BUCKET_CONFIG"
    assert atr_config["atr_width_multiplier"] == pytest.approx(1.75)
    assert atr_config["max_merge_width_multiple"] == pytest.approx(2.25)


def test_score_symbol_chip_before_date_uses_analyzed_at_not_wall_clock_today(monkeypatch, bundle):
    """即使系統「今天」跟資料裡最後一根K棒的日期不同（例如重算歷史資料），
    before_date 也要用 analyzed_at（K棒日期），不能不小心用 wall-clock 的
    今天，否則舊資料重算時一樣可能撈到「未來」的籌碼分數。"""
    df = bullish_trend_df(n=250)
    # 刻意用一個確定跟「今天」不同、且已知的歷史日期範圍
    base = pd.Timestamp("2020-01-01", tz="UTC")
    rows = [
        {
            "open": row["open"], "high": row["high"], "low": row["low"],
            "close": row["close"], "volume": row["volume"],
            "timestamp": int((base + pd.Timedelta(days=i)).timestamp()),
        }
        for i, (_, row) in enumerate(df.iterrows())
    ]

    captured = {}

    def _fake_fetch_chip(symbol, before_date=None, **kw):
        captured["before_date"] = before_date
        return None

    monkeypatch.setattr(scoring, "fetch_candles", lambda *a, **kw: rows)
    monkeypatch.setattr(scoring, "get_model", lambda: bundle)
    monkeypatch.setattr(scoring, "fetch_latest_chip_score", _fake_fetch_chip)

    result = score_symbol("2330", "1d")

    expected_before_date = pd.Timestamp(result["analysis"]["analyzed_at"]).tz_convert("Asia/Taipei").strftime("%Y-%m-%d")
    assert captured["before_date"] == expected_before_date
    # 確認這個日期真的落在歷史資料範圍（2020年），不是 wall-clock 今天，
    # 證明用的是 analyzed_at 而非系統當下時間。
    assert captured["before_date"].startswith("2020")


def test_score_symbol_survives_chip_row_with_null_total_score(monkeypatch, bundle):
    """chip row 存在但 total_score 為 NULL（partial coverage：子分數有值、綜合分數尚未算出）時，
    calculate_scores 不得因 float(None) 拋 TypeError 中止評分；chip_score 視為缺值走中性。"""
    df = bullish_trend_df(n=250)
    rows = [
        {
            "open": row["open"], "high": row["high"], "low": row["low"],
            "close": row["close"], "volume": row["volume"], "timestamp": int(ts.timestamp()),
        }
        for ts, row in df.iterrows()
    ]
    low = float(df["close"].min())

    def _fake_fetch_chip(symbol, before_date=None, **kw):
        return {"symbol": "2330", "total_score": None, "institutional_score": -55.0}

    monkeypatch.setattr(scoring, "fetch_candles", lambda *a, **kw: rows)
    monkeypatch.setattr(scoring, "get_model", lambda: bundle)
    monkeypatch.setattr(scoring, "fetch_latest_chip_score", _fake_fetch_chip)

    zone = Zone(price_low=low, price_high=low + 1.0, method=ZoneMethod.ATR, center_price=low + 0.5, formed_at_index=0)
    result = score_symbol("2330", "1d", builders=[_SingleZoneBuilder(zone)])

    assert len(result["zones"]) == 1
    z = _v2_zone_scores(result)[0]
    # total_score 缺值 → chip_score=None → trading_score 的 chip 分量走中性 0.5。
    assert z["trading_score_breakdown"]["chip"] == pytest.approx(0.5 * TRADING_SCORE_WEIGHTS["chip"])


class _SingleZoneBuilder:
    def __init__(self, zone: Zone):
        self._zone = zone

    @property
    def min_bars(self) -> int:
        return 1

    def build(self, df):
        return [self._zone]


# ── 十二、Global EV/Confidence/RR（唯一收斂）───────────────────────────


def test_compute_global_metrics_empty_zones_returns_none():
    metrics = _compute_global_metrics([])
    assert metrics == {"expected_value": None, "confidence": None, "risk_reward_ratio": None}


def test_compute_global_metrics_confidence_weighted_average(bundle):
    df = bullish_trend_df(n=80)
    low = float(df["close"].min())
    zone = Zone(price_low=low, price_high=low + 1.0, method=ZoneMethod.ATR, center_price=low + 0.5, formed_at_index=0)
    current_price = float(df["close"].iloc[-1])
    score = score_zone(df, zone, current_price, bundle, _trend(df))

    metrics = _compute_global_metrics([score])

    # 只有一個 zone 時，加權平均就是那個 zone 自己的值
    assert metrics["confidence"] == pytest.approx(score.confidence)
    if score.expected_value is not None:
        assert metrics["expected_value"] == pytest.approx(score.expected_value)
    if score.risk_reward_ratio is not None:
        assert metrics["risk_reward_ratio"] == pytest.approx(score.risk_reward_ratio)


# ── score_zone: confidence/EV/RR/trading 整合測試 ────────────────────────


def test_score_zone_never_touched_has_low_confidence_and_zero_ev(bundle):
    """完全沒被觸碰過的 zone（touch_count=0）：expected_gain/expected_loss
    都是 0（沒有歷史報酬可算），expected_value 因此也是 0；risk_reward_ratio
    因為分母是 0 而無法計算，回傳 None。confidence 因為缺乏樣本、缺乏最近
    驗證，會明顯偏低（但不是剛好 0，因為 stability_factor 在無資料時給中性
    值 0.5，避免三因子其中一個直接把整體拖到 0 這種過度懲罰）。"""
    df = bullish_trend_df(n=80)
    far_low = float(df["high"].max()) + 1000.0
    zone = Zone(price_low=far_low, price_high=far_low + 1.0, method=ZoneMethod.ATR, center_price=far_low + 0.5, formed_at_index=0)
    current_price = float(df["close"].iloc[-1])

    score = score_zone(df, zone, current_price, bundle, _trend(df))

    assert score.role == "RESISTANCE"  # zone 在現價之上
    assert score.confidence < 0.3  # LOW
    assert score.confidence_level == "LOW"
    assert score.expected_gain == pytest.approx(0.0)
    assert score.expected_loss == pytest.approx(0.0)
    assert score.expected_value == pytest.approx(0.0)
    assert score.risk_reward_ratio is None
    assert score.reward_risk_percentile is None
    assert score.recent_validation == "PENDING_VALIDATION"


def test_score_zone_probabilities_never_exceed_one_combined(bundle):
    df = bullish_trend_df(n=80)
    low = float(df["close"].min())
    zone = Zone(price_low=low, price_high=low + 1.0, method=ZoneMethod.ATR, center_price=low + 0.5, formed_at_index=0)
    current_price = float(df["close"].iloc[-1])

    score = score_zone(df, zone, current_price, bundle, _trend(df))

    if score.bounce_probability is not None and score.break_probability is not None:
        assert score.bounce_probability + score.break_probability <= 1.0 + 1e-9


def test_score_zone_expected_value_matches_weighted_formula(bundle):
    """一、修正 EV：驗證 expected_value 確實等於
    bounce機率×expected_gain + break機率×expected_loss，而不是用單一
    average_return 硬算出來的舊公式。"""
    df = bullish_trend_df(n=80)
    current_price = float(df["close"].iloc[-1])
    low = float(df["close"].min())
    zone = Zone(price_low=low, price_high=low + 1.0, method=ZoneMethod.ATR, center_price=low + 0.5, formed_at_index=0)

    score = score_zone(df, zone, current_price, bundle, _trend(df))

    if score.role != "AT_ZONE" and score.expected_gain is not None:
        expected = score.bounce_probability * score.expected_gain + score.break_probability * score.expected_loss
        assert score.expected_value == pytest.approx(expected)


def test_score_zone_at_zone_has_no_expected_value_or_risk_reward(bundle):
    df = bullish_trend_df(n=80)
    current_price = float(df["close"].iloc[-1])
    zone = Zone(
        price_low=current_price - 1, price_high=current_price + 1,
        method=ZoneMethod.ATR, center_price=current_price, formed_at_index=0,
    )

    score = score_zone(df, zone, current_price, bundle, _trend(df))

    assert score.role == "AT_ZONE"
    assert score.expected_value is None
    assert score.risk_reward_ratio is None


def test_score_symbol_zone_dict_includes_institutional_fields(monkeypatch, bundle):
    df = bullish_trend_df(n=250)
    rows = [
        {
            "open": row["open"], "high": row["high"], "low": row["low"],
            "close": row["close"], "volume": row["volume"], "timestamp": int(ts.timestamp()),
        }
        for ts, row in df.iterrows()
    ]

    monkeypatch.setattr(scoring, "fetch_candles", lambda *a, **kw: rows)
    monkeypatch.setattr(scoring, "get_model", lambda: bundle)
    monkeypatch.setattr(scoring, "fetch_latest_chip_score", lambda *a, **kw: None)

    result = score_symbol("2330", "1d")

    expected_keys = {
        "price_low", "price_high", "method", "role", "tier", "tier_label", "role_label", "display_label",
        "support_score", "resistance_score", "net_score", "net_score_label",
        "confidence", "confidence_level",
        "bounce_probability", "break_probability",
        "expected_gain", "expected_loss", "expected_value",
        "risk_reward_ratio", "reward_risk_percentile",
        "relative_volume", "volume_confirmation",
        "touch_count", "reject_count", "break_count",
        "zone_momentum", "zone_direction",
        "recent_validation", "trading_score", "trading_score_breakdown", "trading_recommendation",
        "zone_quality_score", "entry_relevance_score", "entry_relevance_breakdown",
        "confluence_family_count", "confluence_families",
    }
    for z in _v2_zone_scores(result):
        assert expected_keys <= set(z.keys())
        if z["role"] == "AT_ZONE":
            assert z["expected_value"] is None
            assert z["risk_reward_ratio"] is None
            assert z["volume_confirmation"] is None
        else:
            assert z["trading_recommendation"] in (
                "STRONG_BUY", "BUY", "WATCH", "NEUTRAL", "AVOID", "STRONG_SELL",
            )



# ── 【2026-07 籌碼數字化 A+B+C】chip_summary / 角色化摘要 / 機率邊際貢獻 ──────


def test_chip_direction_thresholds():
    assert scoring._chip_direction(None) == "none"
    assert scoring._chip_direction(50.0) == "bullish"
    assert scoring._chip_direction(-50.0) == "bearish"
    assert scoring._chip_direction(5.0) == "neutral"
    # 方向門檻使用弱訊號（±10）；強弱程度由 _chip_signal 五段化輸出。
    assert scoring._chip_direction(scoring.CHIP_SIGNAL_THRESHOLD) == "bullish"
    assert scoring._chip_direction(-scoring.CHIP_SIGNAL_THRESHOLD) == "bearish"


def test_chip_signal_uses_five_bands():
    assert scoring._chip_signal(None) is None
    assert scoring._chip_signal(30.0) == "BULLISH"
    assert scoring._chip_signal(10.0) == "WEAK_BULLISH"
    assert scoring._chip_signal(0.0) == "NEUTRAL"
    assert scoring._chip_signal(-10.0) == "WEAK_BEARISH"
    assert scoring._chip_signal(-30.0) == "BEARISH"


def test_build_chip_summary_missing():
    s = scoring._build_chip_summary(None)
    assert s["missing"] is True
    assert s["score"] is None
    assert s["coverage"] == 0.0
    assert s["confidence"] == 0.0
    assert s["effective_score"] is None
    assert s["signal"] is None
    assert s["institutional_score"] is None
    assert s["margin_score"] is None
    assert s["broker_score"] is None
    assert s["concentration_score"] is None


def test_build_chip_summary_present():
    s = scoring._build_chip_summary({
        "total_score": 42.5, "signal": "BULLISH",
        "institutional_score": 60.0, "margin_score": -10.0,
        "broker_score": 30.0, "concentration_score": 40.0,
    })
    assert s["missing"] is False
    assert s["score"] == pytest.approx(34.0)
    assert s["raw_score"] == pytest.approx(34.0)
    # 滿覆蓋時 effective_score = raw_score * coverage = 34.0，不採用未降權的 DB total_score(42.5)。
    assert s["effective_score"] == pytest.approx(34.0)
    assert s["coverage"] == pytest.approx(1.0)
    assert s["confidence_level"] == "HIGH"
    assert s["signal"] == "BULLISH"
    assert s["source_signal"] == "BULLISH"
    assert s["institutional_score"] == pytest.approx(60.0)
    assert s["margin_score"] == pytest.approx(-10.0)
    assert s["broker_score"] == pytest.approx(30.0)
    assert s["concentration_score"] == pytest.approx(40.0)


def test_build_chip_summary_partial_coverage_separates_raw_and_effective():
    s = scoring._build_chip_summary({
        "total_score": -19.25, "signal": "BEARISH",
        "institutional_score": -55.0, "margin_score": None,
        "broker_score": None, "concentration_score": None,
    })

    assert s["missing"] is False
    assert s["score"] == pytest.approx(-55.0)
    assert s["coverage"] == pytest.approx(0.35)
    assert s["confidence"] == pytest.approx(0.35)
    assert s["confidence_level"] == "LOW"
    assert s["effective_score"] == pytest.approx(-19.25)
    assert s["signal"] == "WEAK_BEARISH"
    assert s["source_signal"] == "BEARISH"


def test_build_chip_summary_effective_score_deweights_ignoring_total_score():
    # total_score 與 raw_score * coverage 不相等時，effective_score 必須採降權值，
    # 而非 DB total_score（鎖住 sr-zone-scoring.md「effective_score = raw_score * coverage」語意）。
    s = scoring._build_chip_summary({
        "total_score": -40.0, "signal": "BEARISH",
        "institutional_score": -55.0, "margin_score": None,
        "broker_score": None, "concentration_score": None,
    })

    assert s["raw_score"] == pytest.approx(-55.0)
    assert s["coverage"] == pytest.approx(0.35)
    # raw_score * coverage = -19.25，明確不等於 total_score(-40.0)。
    assert s["effective_score"] == pytest.approx(-19.25)
    assert s["effective_score"] != pytest.approx(-40.0)
    assert s["signal"] == "WEAK_BEARISH"


def test_build_chip_summary_present_but_all_subscores_none():
    # chip row 存在但四個子分數全 None（total_score 仍存在）：raw/effective 皆 None、
    # coverage=0、missing=False（保守，不因無可用分量而回報未降權的 total_score）。
    s = scoring._build_chip_summary({
        "total_score": 30.0, "signal": "BULLISH",
        "institutional_score": None, "margin_score": None,
        "broker_score": None, "concentration_score": None,
    })

    assert s["missing"] is False
    assert s["raw_score"] is None
    assert s["score"] is None
    assert s["effective_score"] is None
    assert s["coverage"] == pytest.approx(0.0)
    assert s["confidence_level"] == "NONE"
    assert s["signal"] is None


def test_score_zone_chip_delta_none_when_no_chip_data(bundle):
    df = bullish_trend_df(n=80)
    low = float(df["close"].min())
    zone = Zone(price_low=low, price_high=low + 1.0, method=ZoneMethod.ATR, center_price=low + 0.5, formed_at_index=0)
    current_price = float(df["close"].iloc[-1])

    # 不帶籌碼 → chip_missing，機率邊際貢獻無從比較，deltas 應為 None
    score = score_zone(df, zone, current_price, bundle, _trend(df))

    assert score.chip_direction == "none"
    assert score.chip_bounce_delta is None
    assert score.chip_break_delta is None


def test_score_zone_chip_delta_present_when_chip_data(bundle):
    from ..model import chip_features_from_score_row

    df = bullish_trend_df(n=80)
    low = float(df["close"].min())
    zone = Zone(price_low=low, price_high=low + 1.0, method=ZoneMethod.ATR, center_price=low + 0.5, formed_at_index=0)
    current_price = float(df["close"].iloc[-1])
    chip_features = chip_features_from_score_row({
        "total_score": 80.0, "institutional_score": 80.0, "margin_score": 40.0,
        "broker_score": 60.0, "concentration_score": 50.0,
    })

    score = score_zone(
        df, zone, current_price, bundle, _trend(df),
        chip_score=80.0, chip_features=chip_features,
    )

    assert score.chip_direction == "bullish"
    # zone 在最低價附近、現價偏高 → role=SUPPORT（有方向），delta 應為 float（非 None）
    assert score.role == "SUPPORT"
    assert isinstance(score.chip_bounce_delta, float)
    assert isinstance(score.chip_break_delta, float)


def _bullish_rows(n: int = 250):
    df = bullish_trend_df(n=n)
    return [
        {
            "open": row["open"], "high": row["high"], "low": row["low"],
            "close": row["close"], "volume": row["volume"], "timestamp": int(ts.timestamp()),
        }
        for ts, row in df.iterrows()
    ]


def test_score_symbol_includes_chip_summary_and_card_chip(monkeypatch, bundle):
    monkeypatch.setattr(scoring, "fetch_candles", lambda *a, **kw: _bullish_rows())
    monkeypatch.setattr(scoring, "get_model", lambda: bundle)
    monkeypatch.setattr(scoring, "fetch_latest_chip_score", lambda *a, **kw: {
        "symbol": "2330", "total_score": 38.25, "signal": "BULLISH",
        "institutional_score": 55.0, "margin_score": 20.0,
        "broker_score": 30.0, "concentration_score": 40.0,
    })

    result = score_symbol("2330", "1d")

    cs = result["analysis"]["chip_summary"]
    assert cs["missing"] is False
    assert cs["score"] == pytest.approx(38.25)
    assert cs["raw_score"] == pytest.approx(38.25)
    assert cs["effective_score"] == pytest.approx(38.25)
    assert cs["coverage"] == pytest.approx(1.0)
    assert cs["signal"] == "BULLISH"
    assert cs["institutional_score"] == pytest.approx(55.0)
    assert cs == result["evidence"]["chip"]

    assert all("evidence" in item for item in result["zones"])


def test_score_symbol_chip_summary_missing_when_no_chip_data(monkeypatch, bundle):
    monkeypatch.setattr(scoring, "fetch_candles", lambda *a, **kw: _bullish_rows())
    monkeypatch.setattr(scoring, "get_model", lambda: bundle)
    monkeypatch.setattr(scoring, "fetch_latest_chip_score", lambda *a, **kw: None)

    result = score_symbol("2330", "1d")

    assert result["evidence"]["chip"]["missing"] is True
    assert result["evidence"]["chip"]["score"] is None
    assert result["analysis"]["chip_summary"] == result["evidence"]["chip"]


def test_score_symbol_degrades_evidence_when_shap_unavailable(monkeypatch, bundle):
    # shap 不可用時，/sr-zones 對應的 score_symbol 應仍回完整結果（非 503/例外），
    # evidence 標記未產生、zone 的 support/resistance 降級為 None、risk_flags 保留、
    # explanation 的 uses_shap_evidence 為 False。
    df = bullish_trend_df(n=250)
    rows = [
        {
            "open": row["open"], "high": row["high"], "low": row["low"],
            "close": row["close"], "volume": row["volume"], "timestamp": int(ts.timestamp()),
        }
        for ts, row in df.iterrows()
    ]
    monkeypatch.setattr(scoring, "fetch_candles", lambda *a, **kw: rows)
    monkeypatch.setattr(scoring, "get_model", lambda: bundle)
    monkeypatch.setattr(scoring, "fetch_latest_chip_score", lambda *a, **kw: None)
    monkeypatch.setattr(evidence_mod, "_shap_available", lambda: False)

    result = score_symbol("2330", "1d")

    assert result["evidence"]["model"]["explainer"] is None
    assert result["evidence"]["model"]["evidence_available"] is False
    assert result["explanation"]["model_context"]["uses_shap_evidence"] is False
    assert len(result["zones"]) > 0
    for item in result["zones"]:
        assert item["evidence"]["support"] is None
        assert item["evidence"]["resistance"] is None
        assert "risk_flags" in item["evidence"]
