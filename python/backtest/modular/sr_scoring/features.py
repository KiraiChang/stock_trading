"""
每個 zone 的特徵計算：touch_count / rejection_count / breakout_count /
avg_return_after_touch / relative_volume / volatility / trend_strength。

所有函式都以「as_of_index 當下累積至今的歷史表現」為語意 —— 這讓同一份
compute_zone_features() 同時適用於：
  (a) 訓練資料：以某次歷史觸碰事件的 touch_index 當作 as_of_index
  (b) 即時評分：以資料最後一根K棒當作 as_of_index

避免 lookahead：avg_return_after_touch 只採計 touch_index + forward_bars
<= as_of_index 的觸碰；其餘函式本來就只掃描 [:as_of_index+1] 範圍內的資料。
"""
from __future__ import annotations

import numpy as np
import pandas as pd

from ...indicators import calc_atr
from .types import ApproachDirection, Zone, ZoneFeatures, ZoneTouch, ZoneType


def touch_starting_at(df: pd.DataFrame, zone: Zone, index: int) -> ZoneTouch | None:
    """檢查 index 是否為一次新觸碰事件的起點（index 在 zone 內、index-1 不在
    zone 內或不存在）。用全域（非窗口切片）的 index-1 判斷，讓「觸碰起點」
    的定義不受呼叫端傳入的 lookback 窗口邊界影響。

    approach_direction 依觸碰前一根收盤價判斷：高於 zone 上緣 → FROM_ABOVE
    （候選支撐）；低於 zone 下緣 → FROM_BELOW（候選壓力）；若前一根收盤價
    本身已落在 zone 內（例如 zone 剛擴大），用與 center_price 的相對位置
    當 tie-break。index=0 沒有「前一根」可判斷，回傳 None（極端邊界情況）。
    """
    if index == 0:
        return None
    lows = df["low"].to_numpy()
    highs = df["high"].to_numpy()
    closes = df["close"].to_numpy()

    if not (lows[index] <= zone.price_high and highs[index] >= zone.price_low):
        return None
    if lows[index - 1] <= zone.price_high and highs[index - 1] >= zone.price_low:
        return None  # 屬於前一個 run 的延續，非新起點

    prev_close = closes[index - 1]
    if prev_close > zone.price_high:
        direction = ApproachDirection.FROM_ABOVE
    elif prev_close < zone.price_low:
        direction = ApproachDirection.FROM_BELOW
    else:
        direction = (
            ApproachDirection.FROM_ABOVE
            if prev_close >= zone.center_price
            else ApproachDirection.FROM_BELOW
        )
    role = ZoneType.SUPPORT if direction == ApproachDirection.FROM_ABOVE else ZoneType.RESISTANCE

    return ZoneTouch(
        zone=zone,
        touch_index=index,
        touch_time=df.index[index],
        touch_price=float(closes[index]),
        approach_direction=direction,
        role=role,
    )


def find_touches(df: pd.DataFrame, zone: Zone, as_of_index: int, lookback_bars: int) -> list[ZoneTouch]:
    """觸碰事件 = 連續 K 棒範圍（[low, high]）與 zone 相交的區段，回傳每個
    區段的起點。只統計「起點」落在 [as_of_index-lookback_bars+1, as_of_index]
    範圍內的觸碰（起點本身用全域資料判斷，見 touch_starting_at）。"""
    start = max(1, as_of_index - lookback_bars + 1)
    touches: list[ZoneTouch] = []
    for i in range(start, as_of_index + 1):
        touch = touch_starting_at(df, zone, i)
        if touch is not None:
            touches.append(touch)
    return touches


def count_rejections(
    df: pd.DataFrame, touches: list[ZoneTouch], rejection_window: int, as_of_index: int
) -> int:
    """觸碰後在 rejection_window 根內，收盤價明確反向遠離 zone 視為一次拒絕。
    n 以 as_of_index+1 為上限（而非 len(df)），避免在訓練資料的 walk-forward
    迴圈中偷看 as_of_index 之後的未來K棒。"""
    closes = df["close"].to_numpy()
    n = min(len(closes), as_of_index + 1)
    count = 0
    for touch in touches:
        end = min(n, touch.touch_index + 1 + rejection_window)
        future = closes[touch.touch_index + 1 : end]
        if len(future) == 0:
            continue
        if touch.role == ZoneType.SUPPORT:
            if np.any(future > touch.zone.price_high):
                count += 1
        else:
            if np.any(future < touch.zone.price_low):
                count += 1
    return count


def count_breakouts(
    df: pd.DataFrame,
    zone: Zone,
    as_of_index: int,
    lookback_bars: int,
    confirmation_bars: int,
    approach: ApproachDirection,
) -> int:
    """連續收盤朝「不利於 zone 成立」的方向突破邊界達 confirmation_bars 根
    視為一次突破（state machine 逐根掃描，同一段持續行情只計一次，避免重複
    計數）。方向依 approach 而定：支撐（FROM_ABOVE）只看跌破下緣；壓力
    （FROM_BELOW）只看漲破上緣——單純「價格在 zone 上方」對支撐來說是正常
    狀態，不能算突破。"""
    start = max(0, as_of_index - lookback_bars + 1)
    closes = df["close"].to_numpy()[start : as_of_index + 1]

    count = 0
    streak = 0
    counted_this_streak = False
    for c in closes:
        broken = c < zone.price_low if approach == ApproachDirection.FROM_ABOVE else c > zone.price_high

        if broken:
            streak += 1
        else:
            streak = 0
            counted_this_streak = False

        if broken and streak >= confirmation_bars and not counted_this_streak:
            count += 1
            counted_this_streak = True
    return count


def avg_return_after_touch(
    df: pd.DataFrame, touches: list[ZoneTouch], forward_bars: int, as_of_index: int
) -> float:
    """觸碰後 forward_bars 根的報酬，正值 = 有利於 zone 成立的方向。"""
    closes = df["close"].to_numpy()
    returns: list[float] = []
    for touch in touches:
        target = touch.touch_index + forward_bars
        if target > as_of_index:
            continue
        base = closes[touch.touch_index]
        if base == 0:
            continue
        raw = (closes[target] - base) / base
        returns.append(raw if touch.role == ZoneType.SUPPORT else -raw)
    return float(np.mean(returns)) if returns else 0.0


def relative_volume_at_touches(
    df: pd.DataFrame, touches: list[ZoneTouch], volume_ma_period: int
) -> float:
    """觸碰量 / 觸碰前 MA(volume)（排除當根，比照 calc_volume_spike 慣例）。"""
    volumes = df["volume"].to_numpy(dtype=float)
    ratios: list[float] = []
    for touch in touches:
        idx = touch.touch_index
        start = idx - volume_ma_period
        if start < 0:
            continue
        baseline = volumes[start:idx]
        ma = float(np.mean(baseline)) if len(baseline) > 0 else 0.0
        if ma <= 0:
            continue
        ratios.append(volumes[idx] / ma)
    return float(np.mean(ratios)) if ratios else 0.0


def zone_volatility(df: pd.DataFrame, as_of_index: int, atr_period: int = 14) -> float:
    """ATR / close，正規化後跨股可比。"""
    highs = df["high"].to_numpy()[: as_of_index + 1]
    lows = df["low"].to_numpy()[: as_of_index + 1]
    closes = df["close"].to_numpy()[: as_of_index + 1]
    atr = calc_atr(highs, lows, closes, atr_period)
    close = closes[-1]
    return float(atr / close) if close else 0.0


def trend_slope(df: pd.DataFrame, as_of_index: int, ma_period: int = 20, lookback: int = 20) -> float:
    """MA(ma_period) 序列在最近 lookback 根的線性回歸斜率，正規化為 slope * lookback / price。"""
    closes = df["close"].iloc[: as_of_index + 1]
    if len(closes) < ma_period + lookback:
        return 0.0
    ma_series = closes.rolling(ma_period).mean().to_numpy()[-lookback:]
    if np.isnan(ma_series).any():
        return 0.0
    x = np.arange(lookback)
    slope = float(np.polyfit(x, ma_series, 1)[0])
    price = float(closes.iloc[-1])
    return float(slope * lookback / price) if price else 0.0


def compute_zone_features(
    df: pd.DataFrame,
    zone: Zone,
    as_of_index: int,
    approach: ApproachDirection,
    lookback_bars: int = 60,
    confirmation_bars: int = 2,
    rejection_window: int = 3,
    forward_bars_for_return: int = 5,
    volume_ma_period: int = 20,
    trend_ma_period: int = 20,
    trend_lookback: int = 20,
) -> ZoneFeatures:
    """touch_count 採計所有方向的觸碰（zone 整體活躍度）；rejection_count /
    avg_return_after_touch / relative_volume 只採計與 approach 同方向的觸碰
    （分別評估「作為支撐」或「作為壓力」的歷史表現）。"""
    all_touches = find_touches(df, zone, as_of_index, lookback_bars)
    role_touches = [t for t in all_touches if t.approach_direction == approach]

    return ZoneFeatures(
        touch_count=len(all_touches),
        rejection_count=count_rejections(df, role_touches, rejection_window, as_of_index),
        breakout_count=count_breakouts(df, zone, as_of_index, lookback_bars, confirmation_bars, approach),
        avg_return_after_touch=avg_return_after_touch(df, role_touches, forward_bars_for_return, as_of_index),
        relative_volume=relative_volume_at_touches(df, role_touches, volume_ma_period),
        volatility=zone_volatility(df, as_of_index, atr_period=14),
        trend_strength=trend_slope(df, as_of_index, trend_ma_period, trend_lookback),
    )
