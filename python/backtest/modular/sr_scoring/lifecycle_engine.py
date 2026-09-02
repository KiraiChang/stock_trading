"""SR Zone Lifecycle Engine.

**職責只有一件事**：依 Event 的演進，決定目前處於哪一個生命週期狀態。

不做建議、不看風險報酬比、不管策略模式——那些是 Decision Engine 的事。
拆開的理由是：同一段價格行為在 RR 合格與不合格時，「發生了什麼」應該是同一個答案，
只有「所以該怎麼做」才會不同。原本 `CONTINUATION` 的判定條件裡有 `rr_gate.qualified`，
等於讓 RR 這個**策略條件**去改寫**事件事實**，於是「現在處於什麼階段」無法被獨立回答。

## 這裡的 lifecycle 是哪一個 lifecycle

專案裡有四套同名不同義的狀態詞彙（見 docs/sr-zone-scoring.md
「分層原則：lifecycle 不看 RR」），最容易混淆的是這兩層：

- **單一事件**的狀態機（`event_engine.py` 的 `LIFECYCLE_CANDIDATE/CONFIRMED/ACTIVE/
  RESOLVED/EXPIRED`）——每個事件自己的生老病死，是本模組的**輸入**。
- **整體事件演進**（本模組的 `LIFECYCLE_*`）——把所有事件的當前狀態綜合成一個階段，
  是本模組的**輸出**。

## 現階段是 snapshot-based，不是 chain-based

輸入是 `build_event_state_summary` 的**當前狀態分桶**，不是完整事件鏈。
真正的 chain 目前只存在於 Go 端（T-045 P1 由 DB 快照重建，供顯示用），
要讓本引擎吃 chain 必須另補 Go→Python 的 request contract（T-045 的 `runtime_chain`）。
在那之前，本模組看得到「現在是什麼狀態」，看不到「經歷過幾次測試」。
"""
from __future__ import annotations

from typing import Any, Optional

from .types import RecentValidation, ZoneScore, ZoneType

# ── 生命週期狀態 ────────────────────────────────────────────────
# 沿用原本 `_decision_semantic_pipeline` 的七個值，**刻意不改名**：
# 它們已經寫進 DB（stock_sr_decisions）、API 與 replay 報告，改名的漣漪遠大於收益。
LIFECYCLE_INVALIDATED = "INVALIDATED"
LIFECYCLE_BREAKDOWN = "BREAKDOWN"
LIFECYCLE_CONTINUATION = "CONTINUATION"
LIFECYCLE_CONFIRMED = "CONFIRMED"
LIFECYCLE_TESTING = "TESTING"
LIFECYCLE_NO_PRIMARY_ZONE = "NO_PRIMARY_ZONE"
LIFECYCLE_NORMAL = "NORMAL"

# ── 事件訊號：把當前事件狀態歸納成一句「發生了什麼」 ─────────────
EVENT_SIGNAL_CLOSE_BREAKDOWN = "CLOSE_BREAKDOWN"
EVENT_SIGNAL_CLOSE_RECLAIM = "CLOSE_RECLAIM"
EVENT_SIGNAL_SUPPORT_TEST = "SUPPORT_TEST"
EVENT_SIGNAL_VOLUME_CONTEXT = "VOLUME_CONTEXT"
EVENT_SIGNAL_NO_EVENT = "NO_EVENT"

# 明確突破的門檻：收在 zone 上緣之上 3%。
# 這是**價格結構**判斷，不是風險偏好，所以留在 lifecycle 而不是 decision。
CLEAR_BREAKOUT_MARGIN = 1.03


def event_state_types(states: list[dict[str, Any]]) -> set[str]:
    """把一組事件狀態攤平成型別集合。

    三個鍵都收：`latest_event_type` 是最新事件、`root_event_type` 是鏈的起點、
    `type` 是這一筆本身。判斷「現在有沒有某種事件在作用」時三者都算數。
    """
    out: set[str] = set()
    for state in states:
        for key in ("latest_event_type", "root_event_type", "type"):
            value = state.get(key)
            if value:
                out.add(str(value))
    return out


def event_state_max_age(states: list[dict[str, Any]], event_type: str) -> int:
    """某種事件在這些狀態裡存活最久的 K 棒數。用來分辨「剛發生」與「撐過一天」。"""
    ages = [
        int(state.get("age_bars") or 0)
        for state in states
        if event_type in {
            str(state.get("latest_event_type") or ""),
            str(state.get("root_event_type") or ""),
            str(state.get("type") or ""),
        }
    ]
    return max(ages, default=0)


def resolve_event_signal(
    event_state_summary: dict[str, Any],
    primary_zone: Optional[ZoneScore],
    structure_state: str,
) -> tuple[str, list[str]]:
    """把當前事件狀態歸納成單一訊號，回傳 (signal, reason_codes)。

    優先序即判定順序：**偏空事件優先於偏多**——同時成立時要先講風險。
    """
    active_states = list(event_state_summary.get("active") or [])
    candidate_states = list(event_state_summary.get("candidates") or [])
    active_bearish_states = list(event_state_summary.get("active_bearish_events") or [])
    active_types = event_state_types(active_states)
    candidate_types = event_state_types(candidate_states)

    # **判準與 resolve_lifecycle 的 BREAKDOWN 分支對齊**：那裡用的是
    # `active_bearish_states` 的 truthiness，這裡若只認 HIGH_VOLUME_BREAKDOWN，
    # 一筆 direction=BEARISH 但型別不同的 carried state 會產出
    # `lifecycle_phase=BREAKDOWN` 搭配 `event_signal=CLOSE_RECLAIM` 的自相矛盾輸出。
    # （`event_engine._normalize_previous_event_state` 會原樣採信外部帶進來的 direction，
    # 所以這不是純理論情境。）
    if active_bearish_states:
        return EVENT_SIGNAL_CLOSE_BREAKDOWN, ["ACTIVE_BEARISH_EVENT"]
    if "INTRADAY_RECLAIM" in active_types or structure_state in (
        "SUPPORT_RECLAIM_CANDIDATE",
        "SUPPORT_RECLAIM_CONFIRMED",
    ):
        return EVENT_SIGNAL_CLOSE_RECLAIM, ["CLOSE_RECLAIM"]
    if "REVERSAL_CANDIDATE" in candidate_types:
        return EVENT_SIGNAL_SUPPORT_TEST, ["REVERSAL_CANDIDATE"]
    # `SUPPORT_TEST_CANDIDATE`（只碰到帶子、沒有 UNDERCUT_RECLAIM）**不得產生
    # `CLOSE_RECLAIM`**——那正是 2026-08-28 拆掉的東西：碰觸被命名成收復，再被
    # Lifecycle 當成收復證據回饋到 market_state 與 Bias。它走 SUPPORT_TEST。
    #
    # **位置是刻意選的**：擺在 `REVERSAL_CANDIDATE` 之後、`PENDING_ZONE_VALIDATION`
    # 之前，新狀態只會在原本要落到 EXTREME_VOLUME / NO_EVENT 的情況下改變答案。
    # 有 candidate event 佐證的 REVERSAL_CANDIDATE 優先序不變；
    # PENDING_ZONE_VALIDATION 與它同樣回 SUPPORT_TEST、只差 reason code。
    if structure_state == "SUPPORT_TEST_CANDIDATE":
        return EVENT_SIGNAL_SUPPORT_TEST, ["STRUCTURE_SUPPORT_TOUCH"]
    if (
        primary_zone is not None
        and primary_zone.role == ZoneType.SUPPORT.value
        and primary_zone.recent_validation == RecentValidation.PENDING_VALIDATION.value
    ):
        return EVENT_SIGNAL_SUPPORT_TEST, ["PENDING_ZONE_VALIDATION"]
    if "EXTREME_VOLUME" in active_types:
        return EVENT_SIGNAL_VOLUME_CONTEXT, ["EXTREME_VOLUME_CONTEXT"]
    return EVENT_SIGNAL_NO_EVENT, []


def resolve_lifecycle(
    event_state_summary: dict[str, Any],
    primary_zone: Optional[ZoneScore],
    structure_state: str,
    daily_price_action: Optional[dict[str, Any]],
    current_price: float,
) -> dict[str, Any]:
    """依事件演進決定生命週期狀態。

    **參數裡沒有 rr_gate，是刻意的。** 風險報酬比是進場與策略條件，不是事件事實；
    要維持保守度應該由 Decision Engine 用 RR Gate 去擋，而不是讓 lifecycle 說謊。

    回傳 `{"event_signal", "lifecycle_phase", "reason_codes"}`。
    """
    active_states = list(event_state_summary.get("active") or [])
    active_bearish_states = list(event_state_summary.get("active_bearish_events") or [])

    event_signal, reason_codes = resolve_event_signal(
        event_state_summary, primary_zone, structure_state
    )
    reason_codes = list(reason_codes)

    price_follow_through = str((daily_price_action or {}).get("price_follow_through_state") or "UNKNOWN")
    momentum_state = str((daily_price_action or {}).get("momentum_confirmation_state") or "UNKNOWN")
    reclaim_age = event_state_max_age(active_states, "INTRADAY_RECLAIM")
    clear_zone_breakout = (
        primary_zone is not None
        and primary_zone.role == ZoneType.SUPPORT.value
        and current_price >= primary_zone.price_high * CLEAR_BREAKOUT_MARGIN
    )

    # 判定順序即優先序，第一個成立的就是答案。
    if active_bearish_states or structure_state in ("SUPPORT_RECLAIM_INVALIDATED", "BREAKDOWN"):
        # 收復被證偽 vs 單純跌破，是兩種不同的失敗，不要合併。
        lifecycle_phase = (
            LIFECYCLE_INVALIDATED
            if structure_state == "SUPPORT_RECLAIM_INVALIDATED"
            else LIFECYCLE_BREAKDOWN
        )
    elif event_signal == EVENT_SIGNAL_CLOSE_RECLAIM and (
        price_follow_through == "PRICE_UPSIDE_FOLLOW_THROUGH"
        and momentum_state == "MOMENTUM_CONFIRMED"
        and clear_zone_breakout
    ):
        # **這裡原本還要求 `rr_gate.qualified`**（見 docs/sr-zone-scoring.md
        # 「分層原則：lifecycle 不看 RR」）。
        # 那讓同一段價格行為在 RR 不合格時被判成 CONFIRMED、合格時才是 CONTINUATION——
        # 事件事實被策略條件改寫。移除後 lifecycle 只描述「價格延續且動能確認且明確突破」，
        # RR 由 Decision Engine 在 entry gate 端處理。
        lifecycle_phase = LIFECYCLE_CONTINUATION
        reason_codes.append("PRICE_UPSIDE_FOLLOW_THROUGH")
    elif event_signal == EVENT_SIGNAL_CLOSE_RECLAIM and (
        structure_state == "SUPPORT_RECLAIM_CONFIRMED" or reclaim_age >= 1
    ):
        lifecycle_phase = LIFECYCLE_CONFIRMED
    elif event_signal in (EVENT_SIGNAL_CLOSE_RECLAIM, EVENT_SIGNAL_SUPPORT_TEST):
        lifecycle_phase = LIFECYCLE_TESTING
    elif primary_zone is None:
        lifecycle_phase = LIFECYCLE_NO_PRIMARY_ZONE
    else:
        lifecycle_phase = LIFECYCLE_NORMAL

    return {
        "event_signal": event_signal,
        "lifecycle_phase": lifecycle_phase,
        "reason_codes": reason_codes,
    }
