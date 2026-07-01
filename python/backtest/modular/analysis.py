"""
個股現況分析：給人工判斷用的支撐/壓力/進場/停損/停利報告。

跟 backtester.py 不同，這裡不模擬完整交易過程、不輸出逐筆 Trade，只針對
「現在」（資料最後一根K棒）算一次快照，重用既有的 S/R / 進場 / 停損元件。
純函式、不寫 DB——DB 寫入與後續驗證由 Go 端負責（backend/internal/analysis）。
"""
from __future__ import annotations

from typing import Any

import pandas as pd

from db import fetch_candles

from ..indicators import calc_atr
from .entries import BreakoutEntry, PullbackSupportEntry
from .exits import ATRStopLoss, CompositeStopLoss, StructuralStopLoss
from .support_resistance import ATRChannelSR, SwingHighLowSR, VolumeProfileSR
from .trend import Trend, detect_trend
from .types import Direction, Level, SRLevels

DEFAULT_FETCH_LIMIT = 250
RISK_REWARD_MULTIPLE = 2.0
ATR_TARGET_MULTIPLE = 3.0
ATR_STOP_PERIOD = 14


def analyze_symbol(symbol: str, timeframe: str = "1d") -> dict[str, Any]:
    rows = fetch_candles(symbol, timeframe, limit=DEFAULT_FETCH_LIMIT)
    if not rows:
        raise ValueError(f"no candles found for symbol={symbol} timeframe={timeframe}")

    df = _to_dataframe(rows)
    if len(df) < 35:
        raise ValueError(f"not enough candles for analysis: symbol={symbol} got={len(df)}, need>=35")

    levels = _merge_levels(df)
    trend = detect_trend(df)
    current_price = float(df["close"].iloc[-1])
    analyzed_at = df.index[-1]

    entry = _determine_entry(df, levels, trend, current_price)
    stop_loss = _stop_losses(df, entry) if entry["direction"] != "NONE" else _empty_stop_loss()
    take_profit = (
        _take_profits(df, levels, entry, stop_loss)
        if entry["direction"] != "NONE"
        else _empty_take_profit()
    )

    return {
        "symbol": symbol,
        "timeframe": timeframe,
        "analyzed_at": analyzed_at.isoformat(),
        "current_price": current_price,
        "trend": trend.value,
        "supports": [_level_to_dict(lv) for lv in levels.supports],
        "resistances": [_level_to_dict(lv) for lv in levels.resistances],
        "entry": entry,
        "stop_loss": stop_loss,
        "take_profit": take_profit,
    }


def _to_dataframe(rows: list[dict]) -> pd.DataFrame:
    df = pd.DataFrame(rows)
    df["datetime"] = pd.to_datetime(df["timestamp"], unit="s", utc=True).dt.tz_convert("Asia/Taipei")
    df = df.set_index("datetime").sort_index()
    # 理由同 backtest/engine.py、backtest/modular/service.py 的 _to_dataframe：
    # Postgres/MySQL 的 candles 是 DECIMAL 欄位，驅動預設回傳 decimal.Decimal
    return df[["open", "high", "low", "close", "volume"]].astype(float)


def _merge_levels(df: pd.DataFrame) -> SRLevels:
    """合併三種支撐壓力演算法的結果，個股分析要看的是「全貌」而非單一方法。"""
    supports: list[Level] = []
    resistances: list[Level] = []
    for sr in (SwingHighLowSR(), ATRChannelSR(), VolumeProfileSR()):
        result = sr.calculate(df)
        supports.extend(result.supports)
        resistances.extend(result.resistances)
    supports.sort(key=lambda lv: lv.strength, reverse=True)
    resistances.sort(key=lambda lv: lv.strength, reverse=True)
    return SRLevels(supports=supports, resistances=resistances)


def _level_to_dict(level: Level) -> dict[str, Any]:
    return {
        "price": float(level.price),
        "strength": float(level.strength),
        "method": level.method,
    }


def _determine_entry(df: pd.DataFrame, levels: SRLevels, trend: Trend, current_price: float) -> dict[str, Any]:
    """先檢查『現在』是否已經觸發真正的進場條件；沒有的話回報一個觀察中的
    觸發價位，讓使用者知道要盯哪個價位，而不是直接說『沒訊號』。"""
    signal = BreakoutEntry().evaluate(df, levels) or PullbackSupportEntry().evaluate(df, levels)
    if signal is not None:
        return {
            "status": "ACTIVE",
            "direction": signal.direction.value,
            "price": current_price,
            "reason": signal.reason,
        }

    direction, price, reason = _watching_entry(trend, current_price, levels)
    return {"status": "WATCHING", "direction": direction, "price": price, "reason": reason}


def _watching_entry(trend: Trend, current_price: float, levels: SRLevels) -> tuple[str, float, str]:
    if trend == Trend.BULLISH:
        candidates: list[tuple[str, float, str]] = []
        r = levels.nearest_resistance_above(current_price)
        if r is not None:
            candidates.append(("LONG", r.price, f"等待突破壓力 {r.price:.2f}（來源：{r.method}）"))
        s = levels.nearest_support_below(current_price)
        if s is not None:
            candidates.append(("LONG", s.price, f"等待拉回測試支撐 {s.price:.2f}（來源：{s.method}）"))
        if not candidates:
            return "NONE", current_price, "多頭格局，但支撐/壓力資料不足以給出觀察價位"
        # 取離現價較近的當主要觀察目標，較貼近「近期可能發生」
        return min(candidates, key=lambda c: abs(c[1] - current_price))

    if trend == Trend.BEARISH:
        s = levels.nearest_support_below(current_price)
        if s is not None:
            return "SHORT", s.price, f"等待跌破支撐 {s.price:.2f}（來源：{s.method}）"
        return "NONE", current_price, "空頭格局，但支撐資料不足以給出觀察價位"

    return "NONE", current_price, "盤整格局，無明確方向建議"


def _stop_losses(df: pd.DataFrame, entry: dict[str, Any]) -> dict[str, Any]:
    direction = Direction(entry["direction"])
    price = float(entry["price"])
    return {
        "atr": float(ATRStopLoss(atr_period=ATR_STOP_PERIOD).initial_stop(df, direction, price)),
        "structural": float(StructuralStopLoss().initial_stop(df, direction, price)),
        "composite": float(CompositeStopLoss().initial_stop(df, direction, price)),
    }


def _take_profits(
    df: pd.DataFrame, levels: SRLevels, entry: dict[str, Any], stop_loss: dict[str, Any]
) -> dict[str, Any]:
    direction = Direction(entry["direction"])
    entry_price = float(entry["price"])
    # 用複合停損（ATR + 結構取較保守者）當風險距離的基準，是三個停損裡最貼近
    # 「實際會用哪個出場」的估計
    risk = abs(entry_price - stop_loss["composite"])
    atr = calc_atr(df["high"].to_numpy(), df["low"].to_numpy(), df["close"].to_numpy(), ATR_STOP_PERIOD)

    if direction == Direction.LONG:
        next_level = levels.nearest_resistance_above(entry_price)
        next_level_price = next_level.price if next_level else None
        risk_reward_price = entry_price + RISK_REWARD_MULTIPLE * risk
        atr_price = entry_price + ATR_TARGET_MULTIPLE * atr
    else:
        next_level = levels.nearest_support_below(entry_price)
        next_level_price = next_level.price if next_level else None
        risk_reward_price = entry_price - RISK_REWARD_MULTIPLE * risk
        atr_price = entry_price - ATR_TARGET_MULTIPLE * atr

    return {
        "next_level": float(next_level_price) if next_level_price is not None else None,
        "risk_reward": float(risk_reward_price),
        "atr_multiple": float(atr_price),
    }


def _empty_stop_loss() -> dict[str, Any]:
    return {"atr": None, "structural": None, "composite": None}


def _empty_take_profit() -> dict[str, Any]:
    return {"next_level": None, "risk_reward": None, "atr_multiple": None}
