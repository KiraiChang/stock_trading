"""`db.fetch_candles` 的還原算術與型別。

**volume 還原後是 float，這是刻意的**（T-042）：`volume / vol_factor` 是除法，
把它截成整數會無聲丟掉還原的精度。Python 端所有消費者都以 float 取用
（`.astype(float)` / `to_numpy(dtype=float)`），而原始 volume 不跨 Python→Go 邊界
（Go 收的是 `relative_volume` 這類 float），所以沒有「`1234.0` 打進 int64 欄位」的
解析風險。這一檔把該行為鎖住——改成回整數是行為改變，不是修 bug。

測試不需要 DB：`db.engine` 是 module global 且 `create_engine` 不會連線，換掉即可。
"""
from __future__ import annotations

import pytest

import db


class _FakeResult:
    def __init__(self, rows):
        self._rows = rows

    def mappings(self):
        return self

    def all(self):
        return self._rows


class _FakeConn:
    def __init__(self, rows):
        self._rows = rows
        self.params: dict | None = None

    def execute(self, sql, params):
        self.params = params
        return _FakeResult(self._rows)

    def __enter__(self):
        return self

    def __exit__(self, *exc):
        return False


class _FakeEngine:
    def __init__(self, rows):
        self.conn = _FakeConn(rows)

    def connect(self):
        return self.conn


def _row(**overrides) -> dict:
    """一列 candle。DB 回來的順序是 ts DESC，fetch_candles 會反轉成 ASC。"""
    base = {
        "symbol": "2330",
        "timeframe": "1d",
        "open": 100.0,
        "high": 110.0,
        "low": 90.0,
        "close": 105.0,
        "volume": 1000,
        "amount": 105000.0,
        "adj_factor": 1.0,
        "vol_factor": 1.0,
        "timestamp": 1750000000,
    }
    base.update(overrides)
    return base


@pytest.fixture
def fake_rows(monkeypatch):
    def _install(rows):
        engine = _FakeEngine(rows)
        monkeypatch.setattr(db, "engine", engine)
        return engine

    return _install


def test_volume_is_float_even_when_factor_is_one(fake_rows):
    """**本檔重點**：沒有任何公司行動時 volume 仍是 float。

    `vol_factor == 1.0` 的除法結果就是 float，這條是刻意保留的行為。
    """
    fake_rows([_row(volume=1000)])

    result = db.fetch_candles("2330", "1d")

    assert result[0]["volume"] == 1000.0
    assert isinstance(result[0]["volume"], float)


def test_price_multiplied_and_volume_divided(fake_rows):
    """價乘係數、量除係數——方向相反，因為分割讓股數變多。"""
    fake_rows([_row(close=200.0, open=190.0, high=210.0, low=180.0, volume=1000,
                    adj_factor=0.25, vol_factor=0.25)])

    row = db.fetch_candles("2330", "1d")[0]

    assert row["close"] == 50.0
    assert row["open"] == 47.5
    assert row["high"] == 52.5
    assert row["low"] == 45.0
    assert row["volume"] == 4000.0


def test_amount_is_not_adjusted(fake_rows):
    """成交金額不動——錢不隨股數重新定義。"""
    fake_rows([_row(amount=105000.0, adj_factor=0.25, vol_factor=0.25)])

    assert db.fetch_candles("2330", "1d")[0]["amount"] == 105000.0


def test_cash_dividend_adjusts_price_but_not_volume(fake_rows):
    """現金股利下修價格但**股數沒變**，所以 vol_factor 與 adj_factor 不相等，
    成交量不可以跟著調整。這正是兩個係數必須分開的理由。"""
    fake_rows([_row(close=100.0, volume=1000, adj_factor=0.98, vol_factor=1.0)])

    row = db.fetch_candles("2330", "1d")[0]

    assert row["close"] == pytest.approx(98.0)
    assert row["volume"] == 1000.0


def test_missing_vol_factor_falls_back_to_adj_factor(fake_rows):
    """Phase 1 的舊資料沒有 vol_factor，那時只有分割、價量共用一個係數。"""
    fake_rows([_row(close=200.0, volume=1000, adj_factor=0.5, vol_factor=None)])

    row = db.fetch_candles("2330", "1d")[0]

    assert row["close"] == 100.0
    assert row["volume"] == 2000.0


def test_missing_adj_factor_means_no_adjustment_not_zero(fake_rows):
    """係數是 None/0 時當作 1——「沒有係數」的正解是不調整，不是把價格歸零。"""
    for factor in (None, 0, -1.0):
        fake_rows([_row(close=105.0, volume=1000, adj_factor=factor, vol_factor=factor)])

        row = db.fetch_candles("2330", "1d")[0]

        assert row["close"] == 105.0, f"adj_factor={factor}"
        assert row["volume"] == 1000.0, f"vol_factor={factor}"


def test_unadjusted_mode_leaves_rows_untouched(fake_rows):
    """`adjusted=False` 要拿到原始成交價，連型別都不動（volume 維持 int）。"""
    fake_rows([_row(close=200.0, volume=1000, adj_factor=0.25, vol_factor=0.25)])

    row = db.fetch_candles("2330", "1d", adjusted=False)[0]

    assert row["close"] == 200.0
    assert row["volume"] == 1000
    assert isinstance(row["volume"], int)


def test_rows_are_reversed_to_ascending(fake_rows):
    """SQL 是 ts DESC（為了配 LIMIT 取最近 N 根），回傳前要反轉成由舊到新。"""
    fake_rows([_row(timestamp=300), _row(timestamp=200), _row(timestamp=100)])

    result = db.fetch_candles("2330", "1d")

    assert [r["timestamp"] for r in result] == [100, 200, 300]


def test_query_params_are_forwarded(fake_rows):
    engine = fake_rows([_row()])

    db.fetch_candles("0050", "5m", limit=42)

    assert engine.conn.params == {"symbol": "0050", "tf": "5m", "limit": 42}
