"""測試用的合成 OHLCV 資料產生器。獨立複製自 modular/tests/conftest.py
（不跨測試套件相對 import，理由見 sr_scoring 套件說明：保持獨立）。"""
from __future__ import annotations

import numpy as np
import pandas as pd
import pytest


def make_df(rows: list[tuple[float, float, float, float, float]]) -> pd.DataFrame:
    """rows: [(open, high, low, close, volume), ...]，一天一根。"""
    idx = pd.date_range("2024-01-01", periods=len(rows), freq="B")
    return pd.DataFrame(rows, columns=["open", "high", "low", "close", "volume"], index=idx)


def bullish_trend_df(
    n: int = 80, base: float = 100.0, step: float = 0.4, amp: float = 1.5, volume: float = 1000.0
) -> pd.DataFrame:
    """持續墊高的多頭結構（HH/HL），疊加正弦震盪製造明確的 swing high/low。"""
    x = np.arange(n)
    closes = base + step * x + amp * np.sin(x)
    highs = closes + 0.8
    lows = closes - 0.8
    opens = closes - 0.1
    volumes = np.full(n, volume)
    return make_df(list(zip(opens, highs, lows, closes, volumes)))


def bearish_trend_df(
    n: int = 80, base: float = 100.0, step: float = 0.4, amp: float = 1.5, volume: float = 1000.0
) -> pd.DataFrame:
    return bullish_trend_df(n=n, base=base, step=-step, amp=amp, volume=volume)


def double_bottom_df(support: float = 100.0, volume: float = 1000.0) -> pd.DataFrame:
    """人工構造雙底 pattern：兩次觸及同一支撐價位並反彈，中間有明顯高點，
    且離開/返回時的波動幅度遠大於觸碰用的窄 zone，避免相鄰K棒誤判為同一次觸碰。"""
    closes = [
        110, 108, 104, 101, support,
        102, 105, 108, 111, 113,
        112, 109, 105, 101, support,
        103, 107, 110, 114, 116, 118,
    ]
    rows = [(c + 0.02, c + 0.05, c - 0.05, c, volume) for c in closes]
    return make_df(rows)


@pytest.fixture
def bullish_df() -> pd.DataFrame:
    return bullish_trend_df()


@pytest.fixture
def bearish_df() -> pd.DataFrame:
    return bearish_trend_df()
