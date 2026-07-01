"""
迴歸測試：確保共用指標函式與 DB 寫入邊界回傳的是原生 float/int，而不是
numpy 純量型別。

實際發生過的事故：calc_atr 的 Wilder smoothing 迴圈把 Python float
累加器跟 numpy 陣列元素相加，型別被悄悄提升成 np.float64；numpy>=2.0
的 np.float64.__repr__ 是 "np.float64(0.1578)"，這個值被 SQLAlchemy/psycopg2
當成參數綁定失敗、退化成字面文字塞進 SQL 後，Postgres 把 "np" 解析成
schema 名稱，丟出 InvalidSchemaName。
"""
from __future__ import annotations

from decimal import Decimal

import numpy as np

from ...indicators import calc_atr, calc_macd, calc_rsi
from ..service import _aggregate_result, _to_dataframe, _trade_to_dict
from ..support_resistance.volume_profile import VolumeProfileSR
from ..types import BacktestReport, Direction, ExitReason, Trade
from .conftest import bullish_trend_df


def test_calc_atr_returns_native_float(bullish_df):
    atr = calc_atr(
        bullish_df["high"].to_numpy(), bullish_df["low"].to_numpy(), bullish_df["close"].to_numpy(), 14
    )
    assert type(atr) is float, f"calc_atr 洩漏了 {type(atr)}，應為原生 float"


def test_calc_rsi_returns_native_float(bullish_df):
    rsi = calc_rsi(bullish_df["close"].to_numpy(), 14)
    assert type(rsi) is float, f"calc_rsi 洩漏了 {type(rsi)}，應為原生 float"


def test_calc_macd_returns_native_floats(bullish_df):
    macd, signal, hist = calc_macd(bullish_df["close"].to_numpy(), 12, 26, 9)
    assert type(macd) is float
    assert type(signal) is float, f"calc_macd 的 signal 洩漏了 {type(signal)}，應為原生 float"
    assert type(hist) is float


def test_to_dataframe_casts_decimal_columns_to_float():
    """
    實際發生過的事故：candles 的 open/high/low/close 在 Postgres/MySQL 是
    DECIMAL 欄位，psycopg2/pymysql 預設回傳 decimal.Decimal，而不是 float
    （SQLite 動態型別天生回傳 float，本地開發用 SQLite 才沒發現）。
    VolumeProfileSR 對這些欄位做 (highs+lows+closes)/3.0 時，Decimal 沒辦法
    跟 float 混合運算，會直接丟 TypeError。
    """
    rows = [
        {"open": Decimal("100.50"), "high": Decimal("101.00"), "low": Decimal("99.80"),
         "close": Decimal("100.90"), "volume": 1000, "timestamp": 1700000000},
        {"open": Decimal("100.90"), "high": Decimal("102.00"), "low": Decimal("100.00"),
         "close": Decimal("101.50"), "volume": 1200, "timestamp": 1700086400},
        {"open": Decimal("101.50"), "high": Decimal("103.00"), "low": Decimal("101.00"),
         "close": Decimal("102.50"), "volume": 1300, "timestamp": 1700172800},
    ]

    df = _to_dataframe(rows, "2023-01-01", "2030-01-01")

    for col in ("open", "high", "low", "close", "volume"):
        assert df[col].dtype == float, f"{col} 應轉成 float64，實際是 {df[col].dtype}"

    # 直接重現事故現場：對混有 Decimal 來源的資料做除法運算不應該再丟 TypeError
    levels = VolumeProfileSR(lookback=3, num_bins=5).calculate(df)
    assert levels is not None


def _assert_all_db_safe(d: dict, path: str = "") -> None:
    for key, value in d.items():
        label = f"{path}.{key}" if path else key
        if isinstance(value, dict):
            _assert_all_db_safe(value, label)
            continue
        if value is None or isinstance(value, str):
            continue
        assert type(value) in (float, int), (
            f"{label}={value!r} 型別為 {type(value)}，不是原生 float/int，"
            "寫入 DB 前一定要先轉型，否則可能重現 np.float64 汙染 SQL 的問題"
        )


def test_aggregate_result_values_are_db_safe():
    report = BacktestReport(
        strategy="t",
        total_return=np.float64(0.05),
        annual_return=np.float64(0.1),
        win_rate=0.5,
        max_drawdown=np.float64(0.02),
        sharpe_ratio=np.float64(1.2),
        total_trades=np.int64(4),
        win_trades=np.int64(2),
        loss_trades=np.int64(2),
        avg_pnl=np.float64(100.0),
        trades=[],
    )
    result = _aggregate_result("t", [report], 1_000_000.0)
    _assert_all_db_safe(result)


def test_trade_to_dict_values_are_db_safe():
    trade = Trade(
        symbol="2330",
        direction=Direction.LONG,
        entry_time="2024-01-01",
        exit_time="2024-01-02",
        entry_price=np.float64(100.0),
        exit_price=np.float64(98.0),
        size=np.float64(10.0),
        pnl=np.float64(-20.0),
        pnl_pct=np.float64(-0.02),
        commission=np.float64(1.5),
        exit_reason=ExitReason.STOP_LOSS,
    )
    _assert_all_db_safe(_trade_to_dict(trade))
