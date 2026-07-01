"""測試用的合成 OHLCV 資料產生器。"""
from __future__ import annotations

import numpy as np
import pandas as pd
import pytest


def make_df(rows: list[tuple[float, float, float, float, float]]) -> pd.DataFrame:
    """rows: [(open, high, low, close, volume), ...]，一天一根。"""
    idx = pd.date_range("2024-01-01", periods=len(rows), freq="B")
    return pd.DataFrame(rows, columns=["open", "high", "low", "close", "volume"], index=idx)


def bullish_trend_df(n: int = 40, base: float = 100.0, step: float = 0.4, amp: float = 1.5, volume: float = 1000.0) -> pd.DataFrame:
    """持續墊高的多頭結構（HH/HL），疊加正弦震盪製造明確的 swing high/low。"""
    x = np.arange(n)
    closes = base + step * x + amp * np.sin(x)
    highs = closes + 0.8
    lows = closes - 0.8
    opens = closes - 0.1
    volumes = np.full(n, volume)
    return make_df(list(zip(opens, highs, lows, closes, volumes)))


def bearish_trend_df(n: int = 40, base: float = 100.0, step: float = 0.4, amp: float = 1.5, volume: float = 1000.0) -> pd.DataFrame:
    return bullish_trend_df(n=n, base=base, step=-step, amp=amp, volume=volume)


@pytest.fixture
def bullish_df() -> pd.DataFrame:
    return bullish_trend_df()


@pytest.fixture
def bearish_df() -> pd.DataFrame:
    return bearish_trend_df()
