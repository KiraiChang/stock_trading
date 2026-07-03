from __future__ import annotations

import sys
import types

import numpy as np
import pytest

from ..model import load_model
from ..train import run_training
from .conftest import make_df


def _volatile_df(n: int = 400, seed: int = 1) -> "pd.DataFrame":
    """比 bullish_trend_df 更劇烈震盪的合成走勢，確保用預設 DatasetConfig
    （threshold_pct=0.03, forward_bars=5）也能累積到足夠的 touch 事件數
    （train_model 至少需要 20 筆才能訓練）。"""
    rng = np.random.default_rng(seed)
    x = np.arange(n)
    closes = 100 + 0.05 * x + 8 * np.sin(x / 8.0) + rng.normal(0, 0.5, n)
    highs = closes + np.abs(rng.normal(0.3, 0.2, n))
    lows = closes - np.abs(rng.normal(0.3, 0.2, n))
    opens = closes - 0.1
    volumes = rng.uniform(800, 1500, n)
    return make_df(list(zip(opens, highs, lows, closes, volumes)))


def _fake_db_module(rows_by_symbol: dict[str, list[dict]]) -> types.ModuleType:
    """訓練流程用 `from db import fetch_candles` 動態匯入，這裡塞一個假的
    `db` module 進 sys.modules，讓 _load_db_sources 抓到假資料而不用碰真的
    DB（比照其他測試對 fetch_candles 的 monkeypatch 慣例）。"""
    module = types.ModuleType("db")

    def fetch_candles(symbol: str, timeframe: str, limit: int) -> list[dict]:
        return rows_by_symbol.get(symbol, [])

    module.fetch_candles = fetch_candles
    return module


def _candle_rows(df) -> list[dict]:
    rows = []
    for ts, row in df.iterrows():
        rows.append({
            "open": row["open"], "high": row["high"], "low": row["low"],
            "close": row["close"], "volume": row["volume"],
            "timestamp": int(ts.timestamp()),
        })
    return rows


def test_run_training_raises_when_no_sources():
    with pytest.raises(ValueError):
        run_training(symbols=None, csv_sources=None)


def test_run_training_raises_when_symbol_has_no_candles(monkeypatch):
    monkeypatch.setitem(sys.modules, "db", _fake_db_module({}))
    with pytest.raises(ValueError):
        run_training(symbols=["2330"])


def test_run_training_returns_summary_and_saves_model(monkeypatch, tmp_path):
    df = _volatile_df()
    monkeypatch.setitem(sys.modules, "db", _fake_db_module({"2330": _candle_rows(df)}))

    output = str(tmp_path / "model.joblib")
    result = run_training(symbols=["2330"], model_type="logistic_regression", output=output)

    assert result["sources"] == 1
    assert result["rows"] > 0
    assert result["model_path"] == output
    assert set(result["metrics"]) == {"hold", "break"}
    assert result["split_method"] == "time"
    assert result["dataset_summary"]["rows"] == result["rows"]
    assert "2330" in result["dataset_summary"]["rows_by_symbol"]
    assert len(result["config_hash"]) == 12

    import os
    assert os.path.exists(output)

    loaded = load_model(output)
    assert loaded.config_hash == result["config_hash"]
    assert "dataset_config" in loaded.training_config
    assert "zone_builders" in loaded.training_config
    assert set(loaded.training_config["zone_builders"]) == {"ATRZoneBuilder", "VolumeProfileZoneBuilder"}


def test_run_training_passes_limit_to_fetch_candles(monkeypatch, tmp_path):
    df = _volatile_df()
    captured = {}

    module = types.ModuleType("db")

    def fetch_candles(symbol, timeframe, limit):
        captured["limit"] = limit
        return _candle_rows(df)

    module.fetch_candles = fetch_candles
    monkeypatch.setitem(sys.modules, "db", module)

    run_training(symbols=["2330"], limit=777, output=str(tmp_path / "model.joblib"))

    assert captured["limit"] == 777
