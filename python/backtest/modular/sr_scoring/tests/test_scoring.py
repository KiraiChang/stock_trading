from __future__ import annotations

import pytest

from .. import scoring
from ..features import trend_slope
from ..model import ModelBundle, train_model
from ..scoring import (
    CONFIDENCE_SAMPLE_PSEUDO_COUNT,
    _confidence,
    _derive_score,
    _net_score_label,
    _normalize_probabilities,
    _recent_validation,
    _sample_factor,
    _stability_factor,
    _trading_recommendation,
    _volume_confirmation,
    _zone_direction,
    score_symbol,
    score_zone,
)
from ..types import Zone, ZoneMethod
from .conftest import bullish_trend_df
from .test_model import synthetic_dataset


@pytest.fixture(scope="module")
def bundle() -> ModelBundle:
    return train_model(synthetic_dataset(), model_type="logistic_regression")


def _trend(df) -> float:
    return trend_slope(df, len(df) - 1)


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

    result = score_symbol("2330", "1d")

    assert result["symbol"] == "2330"
    assert result["timeframe"] == "1d"
    assert "overall_trend" in result
    assert "overall_volatility" in result
    assert isinstance(result["zones"], list)
    for z in result["zones"]:
        assert 0.0 <= z["support_score"] <= 1.0
        assert 0.0 <= z["resistance_score"] <= 1.0
        assert z["role"] in ("SUPPORT", "RESISTANCE", "AT_ZONE")
        assert z["net_score_label"] in ("STRONG_SUPPORT", "NEUTRAL", "STRONG_RESISTANCE")
        assert z["confidence_level"] in ("LOW", "MEDIUM", "HIGH", "VERY_HIGH")
        assert z["recent_validation"] in (
            "VALIDATED_RECENTLY", "PENDING_VALIDATION", "NOT_TESTED_RECENTLY", "EXPIRED",
        )
        assert z["zone_direction"] in ("UP", "DOWN", "FLAT")
        # overall_trend/overall_volatility 不應該在每個 zone 裡重複出現
        assert "overall_trend" not in z
        assert "overall_volatility" not in z
        assert "trend_strength" not in z
        assert "volatility" not in z


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


def test_trading_recommendation_support_strong_buy_at_high_score():
    assert _trading_recommendation(90.0, "SUPPORT") == "STRONG_BUY"


def test_trading_recommendation_resistance_strong_sell_at_high_score():
    assert _trading_recommendation(90.0, "RESISTANCE") == "STRONG_SELL"


def test_trading_recommendation_at_zone_is_watch_or_neutral():
    assert _trading_recommendation(70.0, "AT_ZONE") == "WATCH"
    assert _trading_recommendation(10.0, "AT_ZONE") == "NEUTRAL"


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

    result = score_symbol("2330", "1d")

    expected_keys = {
        "price_low", "price_high", "method", "role",
        "support_score", "resistance_score", "net_score", "net_score_label",
        "confidence", "confidence_level",
        "bounce_probability", "break_probability",
        "expected_gain", "expected_loss", "expected_value",
        "risk_reward_ratio", "reward_risk_percentile",
        "relative_volume", "volume_confirmation",
        "touch_count", "reject_count", "break_count",
        "zone_momentum", "zone_direction",
        "recent_validation", "trading_score", "trading_recommendation",
    }
    for z in result["zones"]:
        assert expected_keys <= set(z.keys())
        if z["role"] == "AT_ZONE":
            assert z["expected_value"] is None
            assert z["risk_reward_ratio"] is None
            assert z["volume_confirmation"] is None
        else:
            assert z["trading_recommendation"] in (
                "STRONG_BUY", "BUY", "WATCH", "NEUTRAL", "AVOID", "STRONG_SELL",
            )
