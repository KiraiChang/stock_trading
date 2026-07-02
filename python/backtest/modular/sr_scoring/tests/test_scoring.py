from __future__ import annotations

import pytest

from .. import scoring
from ..model import ModelBundle, train_model
from ..scoring import score_symbol, score_zone
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
