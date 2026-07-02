from __future__ import annotations

import pytest

from .. import scoring
from ..model import ModelBundle, train_model
from ..scoring import (
    CONFIDENCE_PSEUDO_COUNT,
    _confidence,
    _derive_score,
    _normalize_probabilities,
    score_symbol,
    score_zone,
)
from ..types import Zone, ZoneMethod
from .conftest import bullish_trend_df
from .test_model import synthetic_dataset


@pytest.fixture(scope="module")
def bundle() -> ModelBundle:
    return train_model(synthetic_dataset(), model_type="logistic_regression")


def test_score_zone_scores_are_in_unit_interval(bundle):
    df = bullish_trend_df(n=80)
    low = float(df["close"].min())
    zone = Zone(price_low=low, price_high=low + 1.0, method=ZoneMethod.ATR, center_price=low + 0.5, formed_at_index=0)
    current_price = float(df["close"].iloc[-1])

    score = score_zone(df, zone, current_price, bundle)

    assert 0.0 <= score.support_score <= 1.0
    assert 0.0 <= score.resistance_score <= 1.0
    if score.bounce_probability is not None:
        assert 0.0 <= score.bounce_probability <= 1.0
    if score.break_probability is not None:
        assert 0.0 <= score.break_probability <= 1.0


def test_score_zone_role_reflects_current_price(bundle):
    df = bullish_trend_df(n=80)
    current_price = float(df["close"].iloc[-1])
    below_zone = Zone(
        price_low=current_price - 10, price_high=current_price - 8,
        method=ZoneMethod.ATR, center_price=current_price - 9, formed_at_index=0,
    )
    above_zone = Zone(
        price_low=current_price + 8, price_high=current_price + 10,
        method=ZoneMethod.ATR, center_price=current_price + 9, formed_at_index=0,
    )

    below_score = score_zone(df, below_zone, current_price, bundle)
    above_score = score_zone(df, above_zone, current_price, bundle)

    assert below_score.role == "SUPPORT"
    assert above_score.role == "RESISTANCE"
    assert below_score.bounce_probability is not None
    assert above_score.bounce_probability is not None


def test_score_zone_at_zone_has_no_probability(bundle):
    df = bullish_trend_df(n=80)
    current_price = float(df["close"].iloc[-1])
    zone = Zone(
        price_low=current_price - 1, price_high=current_price + 1,
        method=ZoneMethod.ATR, center_price=current_price, formed_at_index=0,
    )

    score = score_zone(df, zone, current_price, bundle)

    assert score.role == "AT_ZONE"
    assert score.bounce_probability is None
    assert score.break_probability is None


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
    assert isinstance(result["zones"], list)
    for z in result["zones"]:
        assert 0.0 <= z["support_score"] <= 1.0
        assert 0.0 <= z["resistance_score"] <= 1.0
        assert z["role"] in ("SUPPORT", "RESISTANCE", "AT_ZONE")


def test_score_symbol_raises_when_no_candles(monkeypatch):
    monkeypatch.setattr(scoring, "fetch_candles", lambda *a, **kw: [])
    with pytest.raises(ValueError):
        score_symbol("2330")


# ── confidence / probability normalization / score derivation ──────────


def test_confidence_zero_touches_is_zero():
    assert _confidence(0) == 0.0


def test_confidence_increases_with_touch_count():
    low = _confidence(1)
    mid = _confidence(CONFIDENCE_PSEUDO_COUNT)
    high = _confidence(50)
    assert 0.0 < low < mid < high < 1.0
    # touch_count == pseudo_count 時定義上剛好是 0.5
    assert mid == pytest.approx(0.5)


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


# ── score_zone: confidence/EV/RR integration ────────────────────────────


def test_score_zone_never_touched_shrinks_to_neutral_and_zero_ev(bundle):
    """完全沒被觸碰過的 zone（touch_count=0）：confidence=0，score 應該貼著
    中性值 0.5，expected_value/risk_reward_ratio 應該是 0（沒有歷史報酬可算，
    且 confidence=0 把 EV 收縮到 0），不該因為模型剛好給出偏高的原始機率就
    顯示出誘人的分數或期望值。"""
    df = bullish_trend_df(n=80)
    # zone 遠離整段走勢的價格範圍，確保 touch_count=0
    far_low = float(df["high"].max()) + 1000.0
    zone = Zone(price_low=far_low, price_high=far_low + 1.0, method=ZoneMethod.ATR, center_price=far_low + 0.5, formed_at_index=0)
    current_price = float(df["close"].iloc[-1])

    score = score_zone(df, zone, current_price, bundle)

    assert score.confidence == 0.0
    assert score.support_score == pytest.approx(0.5)
    assert score.resistance_score == pytest.approx(0.5)
    assert score.role == "RESISTANCE"  # zone 在現價之上
    assert score.expected_value == pytest.approx(0.0)
    assert score.risk_reward_ratio == pytest.approx(0.0)


def test_score_zone_probabilities_never_exceed_one_combined(bundle):
    df = bullish_trend_df(n=80)
    low = float(df["close"].min())
    zone = Zone(price_low=low, price_high=low + 1.0, method=ZoneMethod.ATR, center_price=low + 0.5, formed_at_index=0)
    current_price = float(df["close"].iloc[-1])

    score = score_zone(df, zone, current_price, bundle)

    if score.bounce_probability is not None and score.break_probability is not None:
        assert score.bounce_probability + score.break_probability <= 1.0 + 1e-9


def test_score_zone_at_zone_has_no_expected_value_or_risk_reward(bundle):
    df = bullish_trend_df(n=80)
    current_price = float(df["close"].iloc[-1])
    zone = Zone(
        price_low=current_price - 1, price_high=current_price + 1,
        method=ZoneMethod.ATR, center_price=current_price, formed_at_index=0,
    )

    score = score_zone(df, zone, current_price, bundle)

    assert score.role == "AT_ZONE"
    assert score.expected_value is None
    assert score.risk_reward_ratio is None


def test_score_symbol_zone_dict_includes_new_fields(monkeypatch, bundle):
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

    for z in result["zones"]:
        assert "confidence" in z
        assert 0.0 <= z["confidence"] <= 1.0
        assert "expected_value" in z
        assert "risk_reward_ratio" in z
        if z["role"] == "AT_ZONE":
            assert z["expected_value"] is None
            assert z["risk_reward_ratio"] is None
