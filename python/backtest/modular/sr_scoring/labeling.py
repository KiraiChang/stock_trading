"""
Touch 事件 → 訓練 label：hold_label（反彈/有效）與 break_label（跌破/突破）。

role=SUPPORT（價格由上往下觸碰）：
    hold  = 觸碰後 forward_bars 根內明確上漲 > threshold_pct（反彈成立）
    break = 觸碰後 forward_bars 根內明確下跌 > threshold_pct（支撐失守，鏡射事件）
role=RESISTANCE（價格由下往上觸碰）：
    hold  = 觸碰後 forward_bars 根內明確下跌 > threshold_pct（壓力有效）
    break = 觸碰後 forward_bars 根內明確上漲 > threshold_pct（壓力突破）

兩個方法：
    max_excursion（預設）：用觸碰後窗口內的最高/最低價判斷是否觸及 threshold
                            （抓到「當下有沒有發生過」，較符合直覺的支撐/壓力測試）
    close_at_n：           只看第 N 根收盤價相對觸碰價的報酬（較簡單、雜訊較低）
"""
from __future__ import annotations

import pandas as pd

from .types import ZoneTouch, ZoneType


def label_touch(
    df: pd.DataFrame,
    touch: ZoneTouch,
    forward_bars: int,
    threshold_pct: float,
    method: str = "max_excursion",
) -> tuple[int, int, float] | None:
    """回傳 (hold_label, break_label, forward_return)；資料不足以判斷未來窗口時回傳 None。"""
    n = len(df)
    target = touch.touch_index + forward_bars
    if target >= n:
        return None

    touch_price = touch.touch_price
    if touch_price == 0:
        return None

    is_support = touch.role == ZoneType.SUPPORT

    if method == "max_excursion":
        highs = df["high"].to_numpy()[touch.touch_index + 1 : target + 1]
        lows = df["low"].to_numpy()[touch.touch_index + 1 : target + 1]
        if len(highs) == 0:
            return None
        max_up = (float(highs.max()) - touch_price) / touch_price
        max_down = (float(lows.min()) - touch_price) / touch_price
        if is_support:
            hold_label = 1 if max_up > threshold_pct else 0
            break_label = 1 if max_down < -threshold_pct else 0
        else:
            hold_label = 1 if max_down < -threshold_pct else 0
            break_label = 1 if max_up > threshold_pct else 0
    elif method == "close_at_n":
        raw_return = (float(df["close"].to_numpy()[target]) - touch_price) / touch_price
        if is_support:
            hold_label = 1 if raw_return > threshold_pct else 0
            break_label = 1 if raw_return < -threshold_pct else 0
        else:
            hold_label = 1 if raw_return < -threshold_pct else 0
            break_label = 1 if raw_return > threshold_pct else 0
    else:
        raise ValueError(f"unknown label method: {method}")

    raw_return = (float(df["close"].to_numpy()[target]) - touch_price) / touch_price
    forward_return = raw_return if is_support else -raw_return

    return hold_label, break_label, forward_return
