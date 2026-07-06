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

【2026-07 review 修復】max_excursion 原本分別用 max(high) 與 min(low) 各自
獨立判斷「有沒有漲超過門檻」與「有沒有跌超過門檻」，如果同一個 forward
window 內先大漲又大跌（或先大跌又大漲）都超過門檻，會同時得到 hold_label=1
且 break_label=1。下游對雙標籤的處理方式互相矛盾：
average_bounce_break_returns() 用 if/elif（hold 優先，永遠不會算進 break）、
_touch_confidence() 對 hold/break 各自累加（雙標籤同時計入兩邊）、
_recent_validation() 用 break 優先（雙標籤會被視為 EXPIRED）——同一個事件在
不同地方被當成不同結果。現在改成逐根掃描 forward window，找出「有利方向
（hold）」與「不利方向（break）」哪個先發生，用先發生的當唯一 label，
兩者不可能同時為 1。同一根K棒內上下同時穿越門檻（無法從 OHLC 判斷盤中
先後順序）時，保守地判定為 break 優先。
"""
from __future__ import annotations

import pandas as pd

from .types import ZoneTouch, ZoneType


def _first_excursion_labels(
    highs, lows, touch_price: float, threshold_pct: float, is_support: bool
) -> tuple[int, int]:
    """逐根掃描 forward window，找出「有利方向超過門檻」（hold）與「不利方向
    超過門檻」（break）哪個先發生，回傳互斥的 (hold_label, break_label)——
    不可能同時為 1，避免視窗內先出現有利、後出現不利（或反過來）都超過門檻
    時被同時判定成立（見本檔案開頭的 review 修復說明）。同一根K棒內上下
    同時穿越門檻時（無法從 OHLC 判斷盤中先後順序），保守地判定為 break
    優先（風險控管：不確定時往壞的方向假設；用 role-relative 的 hold/break
    直接判斷 tie-break，而不是在 up/down 層級判斷，才能讓 support 跟
    resistance 兩種角色都一致地「不確定時偏向 break」，而不是像 up/down
    層級的 tie-break 那樣，因為方向跟角色的映射相反，同一個 tie-break 規則
    對 resistance 會變成偏向 hold）。"""
    up_idx = next(
        (i for i, h in enumerate(highs) if (float(h) - touch_price) / touch_price > threshold_pct), None
    )
    down_idx = next(
        (i for i, l in enumerate(lows) if (float(l) - touch_price) / touch_price < -threshold_pct), None
    )

    favorable_idx, unfavorable_idx = (up_idx, down_idx) if is_support else (down_idx, up_idx)

    if favorable_idx is None and unfavorable_idx is None:
        return 0, 0
    if favorable_idx is not None and (unfavorable_idx is None or favorable_idx < unfavorable_idx):
        return 1, 0
    return 0, 1


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
        hold_label, break_label = _first_excursion_labels(highs, lows, touch_price, threshold_pct, is_support)
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
