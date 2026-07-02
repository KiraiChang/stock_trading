from __future__ import annotations

import numpy as np
import pytest

from .. import analysis
from ..trend import Trend
from ..types import Direction, Level, LevelType, SRLevels
from .conftest import bearish_trend_df, bullish_trend_df, make_df


def _candle_rows(df) -> list[dict]:
    """把測試用 DataFrame 轉成 db.fetch_candles() 會回傳的 row 格式。"""
    rows = []
    for ts, row in df.iterrows():
        rows.append({
            "open": row["open"], "high": row["high"], "low": row["low"],
            "close": row["close"], "volume": row["volume"],
            "timestamp": int(ts.timestamp()),
        })
    return rows


def test_analyze_symbol_raises_when_no_candles(monkeypatch):
    monkeypatch.setattr(analysis, "fetch_candles", lambda *a, **kw: [])
    with pytest.raises(ValueError):
        analysis.analyze_symbol("2330")


def test_analyze_symbol_raises_when_insufficient_bars(monkeypatch):
    df = bullish_trend_df(n=10)
    monkeypatch.setattr(analysis, "fetch_candles", lambda *a, **kw: _candle_rows(df))
    with pytest.raises(ValueError):
        analysis.analyze_symbol("2330")


def test_analyze_symbol_passes_limit_through_to_fetch_candles(monkeypatch):
    df = bullish_trend_df(n=60)
    captured = {}

    def fake_fetch_candles(symbol, timeframe, limit):
        captured["limit"] = limit
        return _candle_rows(df)

    monkeypatch.setattr(analysis, "fetch_candles", fake_fetch_candles)
    analysis.analyze_symbol("2330", "1d", limit=500)

    assert captured["limit"] == 500


def test_analyze_symbol_defaults_limit_to_default_fetch_limit(monkeypatch):
    df = bullish_trend_df(n=60)
    captured = {}

    def fake_fetch_candles(symbol, timeframe, limit):
        captured["limit"] = limit
        return _candle_rows(df)

    monkeypatch.setattr(analysis, "fetch_candles", fake_fetch_candles)
    analysis.analyze_symbol("2330")

    assert captured["limit"] == analysis.DEFAULT_FETCH_LIMIT


def test_analyze_symbol_returns_expected_shape(monkeypatch):
    df = bullish_trend_df(n=60)
    monkeypatch.setattr(analysis, "fetch_candles", lambda *a, **kw: _candle_rows(df))

    result = analysis.analyze_symbol("2330", "1d")

    assert result["symbol"] == "2330"
    assert result["timeframe"] == "1d"
    assert isinstance(result["current_price"], float)
    assert result["trend"] in ("BULLISH", "BEARISH", "SIDEWAYS")
    assert isinstance(result["supports"], list)
    assert isinstance(result["resistances"], list)
    assert result["entry"]["status"] in ("ACTIVE", "WATCHING")
    assert result["entry"]["direction"] in ("LONG", "SHORT", "NONE")
    assert set(result["stop_loss"]) == {"atr", "structural", "composite"}
    assert set(result["take_profit"]) == {"next_level", "risk_reward", "atr_multiple"}

    for lv in result["supports"] + result["resistances"]:
        assert set(lv) == {"price", "strength", "method"}
        assert type(lv["price"]) is float
        assert type(lv["strength"]) is float


def test_watching_entry_bullish_prefers_closer_level():
    levels = SRLevels(
        supports=[Level(95.0, LevelType.SUPPORT, 0.8, "swing")],
        resistances=[Level(101.0, LevelType.RESISTANCE, 0.9, "swing")],
    )
    # 現價 100，支撐 95 距離 5，壓力 101 距離 1 → 應該選壓力當觀察目標
    direction, price, reason = analysis._watching_entry(Trend.BULLISH, 100.0, levels)
    assert direction == "LONG"
    assert price == 101.0
    assert "突破" in reason


def test_watching_entry_bearish_uses_nearest_support():
    levels = SRLevels(supports=[Level(90.0, LevelType.SUPPORT, 0.8, "swing")])
    direction, price, reason = analysis._watching_entry(Trend.BEARISH, 100.0, levels)
    assert direction == "SHORT"
    assert price == 90.0
    assert "跌破" in reason


def test_watching_entry_sideways_has_no_direction():
    levels = SRLevels(
        supports=[Level(90.0, LevelType.SUPPORT, 0.8, "swing")],
        resistances=[Level(110.0, LevelType.RESISTANCE, 0.8, "swing")],
    )
    direction, _, reason = analysis._watching_entry(Trend.SIDEWAYS, 100.0, levels)
    assert direction == "NONE"
    assert "盤整" in reason


def test_stop_losses_long_are_below_entry(bullish_df):
    entry = {"direction": "LONG", "price": float(bullish_df["close"].iloc[-1])}
    stops = analysis._stop_losses(bullish_df, entry)
    for key in ("atr", "structural", "composite"):
        assert stops[key] < entry["price"], f"{key} 停損應低於進場價（多單）"


def test_take_profits_long_are_above_entry(bullish_df):
    entry = {"direction": "LONG", "price": float(bullish_df["close"].iloc[-1])}
    stops = analysis._stop_losses(bullish_df, entry)
    levels = analysis._merge_levels(bullish_df)
    targets = analysis._take_profits(bullish_df, levels, entry, stops)
    assert targets["risk_reward"] > entry["price"]
    assert targets["atr_multiple"] > entry["price"]
