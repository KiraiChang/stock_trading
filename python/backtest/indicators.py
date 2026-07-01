"""
與 Go indicator engine 完全對齊的純函數實作。
用途：
  1. 驗證 Python backtest 結果與 Go 生產系統的一致性
  2. 在不使用 backtrader 的場合直接計算（e.g. 批次分析）
"""
import math
import numpy as np
import pandas as pd
from typing import Sequence


# ── MA ────────────────────────────────────────────────────────

def calc_ma(closes: Sequence[float], period: int) -> float:
    """與 Go CalcMA 完全一致：算術平均最後 N 根收盤價。"""
    if len(closes) < period:
        return 0.0
    return float(np.mean(closes[-period:]))


# ── RSI (Wilder smoothing) ────────────────────────────────────

def calc_rsi(closes: Sequence[float], period: int = 14) -> float:
    """與 Go CalcRSI 完全一致：Wilder smoothing。"""
    arr = np.asarray(closes, dtype=float)
    if len(arr) < period + 1:
        return 0.0

    deltas = np.diff(arr)
    gains  = np.where(deltas > 0, deltas, 0.0)
    losses = np.where(deltas < 0, -deltas, 0.0)

    # 初始均值（算術平均前 period 根）
    avg_gain = float(np.mean(gains[:period]))
    avg_loss = float(np.mean(losses[:period]))

    # Wilder smoothing for remaining
    for g, l in zip(gains[period:], losses[period:]):
        avg_gain = (avg_gain * (period - 1) + g) / period
        avg_loss = (avg_loss * (period - 1) + l) / period

    if avg_loss == 0:
        return 100.0
    rs = avg_gain / avg_loss
    # avg_gain/avg_loss 在迴圈中曾與 numpy 陣列元素相加，型別會被提升成
    # np.float64；顯式轉回 float 避免這個值後續被當成 SQL bind 參數時
    # （例如 numpy>=2.0 的 repr 是 "np.float64(...)"）被誤判成識別字。
    return float(100.0 - 100.0 / (1.0 + rs))


# ── MACD ──────────────────────────────────────────────────────

def _calc_ema_series(closes: np.ndarray, period: int) -> np.ndarray:
    multiplier = 2.0 / (period + 1)
    ema = np.zeros(len(closes))
    ema[period - 1] = float(np.mean(closes[:period]))
    for i in range(period, len(closes)):
        ema[i] = closes[i] * multiplier + ema[i - 1] * (1 - multiplier)
    return ema


def calc_macd(closes: Sequence[float], fast: int = 12, slow: int = 26, signal_period: int = 9):
    """與 Go CalcMACD 完全一致。回傳 (macd, signal, histogram)。"""
    arr = np.asarray(closes, dtype=float)
    if len(arr) < slow + signal_period - 1:
        return 0.0, 0.0, 0.0

    fast_ema = _calc_ema_series(arr, fast)
    slow_ema = _calc_ema_series(arr, slow)
    macd_line = fast_ema - slow_ema  # valid from index slow-1

    valid = macd_line[slow - 1:]
    multiplier = 2.0 / (signal_period + 1)
    sig_val = float(np.mean(valid[:signal_period]))
    for v in valid[signal_period:]:
        sig_val = v * multiplier + sig_val * (1 - multiplier)

    last_macd = float(macd_line[-1])
    # sig_val 在迴圈中曾與 numpy 陣列元素相加，型別會被提升成 np.float64，
    # 顯式轉回 float（理由同 calc_rsi）
    sig_val = float(sig_val)
    return last_macd, sig_val, last_macd - sig_val


# ── Bollinger Bands ───────────────────────────────────────────

def calc_bollinger(closes: Sequence[float], period: int = 20, multiplier: float = 2.0):
    """回傳 (upper, middle, lower)，母體標準差（與 Go 一致）。"""
    arr = np.asarray(closes[-period:], dtype=float)
    if len(arr) < period:
        return 0.0, 0.0, 0.0
    mean = float(np.mean(arr))
    std = float(np.sqrt(np.mean((arr - mean) ** 2)))  # population std
    return mean + multiplier * std, mean, mean - multiplier * std


# ── ATR (Wilder) ──────────────────────────────────────────────

def calc_atr(highs: Sequence[float], lows: Sequence[float],
             closes: Sequence[float], period: int = 14) -> float:
    """與 Go CalcATR 完全一致：Wilder smoothing。"""
    h, l, c = np.asarray(highs), np.asarray(lows), np.asarray(closes)
    n = len(c)
    if n < period + 1:
        return 0.0

    tr = np.zeros(n)
    tr[0] = h[0] - l[0]
    for i in range(1, n):
        tr[i] = max(h[i] - l[i], abs(h[i] - c[i - 1]), abs(l[i] - c[i - 1]))

    atr = float(np.mean(tr[1:period + 1]))
    for i in range(period + 1, n):
        atr = (atr * (period - 1) + tr[i]) / period
    # atr 在迴圈中與 numpy 陣列元素 tr[i] 相加，型別會被提升成 np.float64，
    # 顯式轉回 float（理由同 calc_rsi：避免 numpy>=2.0 的 repr 汙染 SQL bind 參數）
    return float(atr)


# ── VWAP ─────────────────────────────────────────────────────

def calc_vwap(highs: Sequence[float], lows: Sequence[float],
              closes: Sequence[float], volumes: Sequence[int]) -> float:
    """與 Go CalcVWAP 一致：TypicalPrice = (H+L+C)/3。"""
    h, l, c, v = map(np.asarray, [highs, lows, closes, volumes])
    tp = (h + l + c) / 3.0
    total_v = float(v.sum())
    if total_v == 0:
        return 0.0
    return float((tp * v).sum() / total_v)


# ── Volume Spike ──────────────────────────────────────────────

VOLUME_SPIKE_MULTIPLIER = 2.0

def calc_volume_spike(volumes: Sequence[int], period: int = 20) -> dict:
    """與 Go CalcVolumeSpike 一致：MA20 不含當日。"""
    arr = np.asarray(volumes, dtype=float)
    if len(arr) < period + 1:
        return {"ma20": 0, "ratio": 0.0, "is_spike": False}
    ma = float(np.mean(arr[-(period + 1):-1]))
    current = float(arr[-1])
    ratio = current / ma if ma > 0 else 0.0
    return {"ma20": int(ma), "ratio": ratio, "is_spike": ratio >= VOLUME_SPIKE_MULTIPLIER}
