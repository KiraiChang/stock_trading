"""Lifecycle Engine 的狀態機測試（todo.md T-044）。

重點在**優先序**與**RR 獨立性**：前者決定同時成立時誰贏，後者是本次抽離的目的。
"""
from __future__ import annotations

import pytest

from backtest.modular.sr_scoring.lifecycle_engine import (
    LIFECYCLE_BREAKDOWN,
    LIFECYCLE_CONFIRMED,
    LIFECYCLE_CONTINUATION,
    LIFECYCLE_INVALIDATED,
    LIFECYCLE_NO_PRIMARY_ZONE,
    LIFECYCLE_NORMAL,
    LIFECYCLE_TESTING,
    resolve_event_signal,
    resolve_lifecycle,
)
from backtest.modular.sr_scoring.types import RecentValidation, ZoneType

from .test_decision_engine import _zone

FOLLOW_THROUGH = {
    "price_follow_through_state": "PRICE_UPSIDE_FOLLOW_THROUGH",
    "momentum_confirmation_state": "MOMENTUM_CONFIRMED",
}


def _summary(active=None, candidates=None, bearish=None) -> dict:
    return {
        "active": active or [],
        "candidates": candidates or [],
        "active_bearish_events": bearish or [],
    }


def _state(event_type: str, age_bars: int = 0) -> dict:
    return {
        "type": event_type,
        "latest_event_type": event_type,
        "root_event_type": event_type,
        "age_bars": age_bars,
    }


# ── 這是整個 T-044 的目的：lifecycle 不看 RR ────────────────────
def test_continuation_only_needs_price_evidence():
    """延續的判定只看價格證據（跟進 ＋ 動能 ＋ 明確突破），與 RR 無關。

    `resolve_lifecycle` 的簽章裡沒有 `rr_gate`，所以這支測試**無法**防守
    「RR 被加回來」的回歸——真要加回來會是新增參數，這支照樣綠燈。
    它能防守的是**這三個條件被改動**（例如有人再塞第四個條件進來）。
    RR 獨立性的真正防線是 `test_widened_path_previously_testing_now_continuation`
    與 decision 層的 entry gate 測試。
    """
    result = resolve_lifecycle(
        event_state_summary=_summary(active=[_state("INTRADAY_RECLAIM", age_bars=1)]),
        primary_zone=_zone(low=98.0, high=100.0),
        structure_state="SUPPORT_RECLAIM_CONFIRMED",
        daily_price_action=FOLLOW_THROUGH,
        current_price=103.5,  # >= 100 * 1.03，明確突破
    )
    assert result["lifecycle_phase"] == LIFECYCLE_CONTINUATION
    assert "PRICE_UPSIDE_FOLLOW_THROUGH" in result["reason_codes"]


def test_widened_path_previously_testing_now_continuation():
    """**這是 RR 解耦真正變寬的那條路徑**，計畫書原本漏記、測試也沒蓋到。

    情境：收復尚未被結構確認（`SUPPORT_RECLAIM_CANDIDATE`）、`age_bars=0`，
    但價格跟進、動能確認、明確突破都成立。

    - 舊碼：CONTINUATION 分支因 `rr_qualified` 為 False 而不成立
      → CONFIRMED 分支也不成立（structure 未確認且 age=0）→ **TESTING**
    - 新碼：**CONTINUATION**

    下游影響是 `action_state` 由 `CONDITIONAL_HOLD` 變成 `HOLD`，
    再被 `_position_action_condition` 原樣採用——**持有建議這條線上沒有 RR gate**
    （entry 有，position 沒有）。詳見 docs/todo.md T-044 的行為改變清單。
    """
    result = resolve_lifecycle(
        event_state_summary=_summary(active=[_state("INTRADAY_RECLAIM", age_bars=0)]),
        primary_zone=_zone(low=98.0, high=100.0),
        structure_state="SUPPORT_RECLAIM_CANDIDATE",
        daily_price_action=FOLLOW_THROUGH,
        current_price=103.5,
    )
    assert result["lifecycle_phase"] == LIFECYCLE_CONTINUATION, (
        "這條路徑舊碼是 TESTING；若改回 TESTING 代表 RR 耦合被加回來了"
    )


# ── 優先序：同時成立時誰贏 ──────────────────────────────────────
def test_bearish_event_wins_over_reclaim():
    """偏空事件與收復同時成立時必須先講風險。"""
    result = resolve_lifecycle(
        event_state_summary=_summary(
            active=[_state("INTRADAY_RECLAIM", age_bars=1)],
            bearish=[_state("HIGH_VOLUME_BREAKDOWN")],
        ),
        primary_zone=_zone(),
        structure_state="SUPPORT_RECLAIM_CONFIRMED",
        daily_price_action=FOLLOW_THROUGH,
        current_price=103.5,
    )
    assert result["lifecycle_phase"] == LIFECYCLE_BREAKDOWN


def test_invalidated_and_breakdown_are_distinguished():
    """收復被證偽 vs 單純跌破是兩種不同的失敗，不該被合併成同一個狀態。"""
    invalidated = resolve_lifecycle(
        event_state_summary=_summary(),
        primary_zone=_zone(),
        structure_state="SUPPORT_RECLAIM_INVALIDATED",
        daily_price_action=None,
        current_price=99.0,
    )
    breakdown = resolve_lifecycle(
        event_state_summary=_summary(),
        primary_zone=_zone(),
        structure_state="BREAKDOWN",
        daily_price_action=None,
        current_price=99.0,
    )
    assert invalidated["lifecycle_phase"] == LIFECYCLE_INVALIDATED
    assert breakdown["lifecycle_phase"] == LIFECYCLE_BREAKDOWN


@pytest.mark.parametrize(
    "price_action,current_price,expected",
    [
        # 延續三條件（跟進、動能、明確突破）缺任一條就不是 CONTINUATION
        (FOLLOW_THROUGH, 103.5, LIFECYCLE_CONTINUATION),
        ({**FOLLOW_THROUGH, "momentum_confirmation_state": "UNKNOWN"}, 103.5, LIFECYCLE_CONFIRMED),
        ({**FOLLOW_THROUGH, "price_follow_through_state": "UNKNOWN"}, 103.5, LIFECYCLE_CONFIRMED),
        (FOLLOW_THROUGH, 102.9, LIFECYCLE_CONFIRMED),  # 未達 zone 上緣 ×1.03
        (None, 103.5, LIFECYCLE_CONFIRMED),
    ],
)
def test_continuation_requires_all_three_conditions(price_action, current_price, expected):
    result = resolve_lifecycle(
        event_state_summary=_summary(active=[_state("INTRADAY_RECLAIM", age_bars=1)]),
        primary_zone=_zone(low=98.0, high=100.0),
        structure_state="SUPPORT_RECLAIM_CONFIRMED",
        daily_price_action=price_action,
        current_price=current_price,
    )
    assert result["lifecycle_phase"] == expected


def test_reclaim_without_confirmation_is_testing():
    """剛收復、還沒撐過一根 K 棒也沒被結構確認 → 仍在測試中。"""
    result = resolve_lifecycle(
        event_state_summary=_summary(active=[_state("INTRADAY_RECLAIM", age_bars=0)]),
        primary_zone=_zone(),
        structure_state="SUPPORT_RECLAIM_CANDIDATE",
        daily_price_action=None,
        current_price=99.0,
    )
    assert result["lifecycle_phase"] == LIFECYCLE_TESTING


def test_reclaim_age_one_bar_is_confirmed():
    """撐過一根 K 棒即視為確認，即使結構狀態還沒到 CONFIRMED。"""
    result = resolve_lifecycle(
        event_state_summary=_summary(active=[_state("INTRADAY_RECLAIM", age_bars=1)]),
        primary_zone=_zone(),
        structure_state="SUPPORT_RECLAIM_CANDIDATE",
        daily_price_action=None,
        current_price=99.0,
    )
    assert result["lifecycle_phase"] == LIFECYCLE_CONFIRMED


def test_pending_validation_zone_is_support_test():
    """zone 還沒被驗證過，本身就是一種「正在測試」。"""
    result = resolve_lifecycle(
        event_state_summary=_summary(),
        primary_zone=_zone(recent_validation=RecentValidation.PENDING_VALIDATION.value),
        structure_state="NORMAL",
        daily_price_action=None,
        current_price=99.0,
    )
    assert result["event_signal"] == "SUPPORT_TEST"
    assert result["lifecycle_phase"] == LIFECYCLE_TESTING


def test_no_primary_zone():
    result = resolve_lifecycle(
        event_state_summary=_summary(),
        primary_zone=None,
        structure_state="NORMAL",
        daily_price_action=None,
        current_price=99.0,
    )
    assert result["lifecycle_phase"] == LIFECYCLE_NO_PRIMARY_ZONE


def test_normal_when_zone_exists_without_events():
    result = resolve_lifecycle(
        event_state_summary=_summary(),
        primary_zone=_zone(),
        structure_state="NORMAL",
        daily_price_action=None,
        current_price=99.0,
    )
    assert result["lifecycle_phase"] == LIFECYCLE_NORMAL


def test_extreme_volume_alone_is_context_not_a_phase():
    """爆量只是脈絡，不足以推進生命週期——狀態仍是 NORMAL。"""
    result = resolve_lifecycle(
        event_state_summary=_summary(active=[_state("EXTREME_VOLUME")]),
        primary_zone=_zone(),
        structure_state="NORMAL",
        daily_price_action=None,
        current_price=99.0,
    )
    assert result["event_signal"] == "VOLUME_CONTEXT"
    assert result["lifecycle_phase"] == LIFECYCLE_NORMAL


def test_resistance_zone_does_not_trigger_clear_breakout():
    """明確突破只對支撐 zone 有意義；壓力 zone 不該因為價格在上方就判成延續。"""
    result = resolve_lifecycle(
        event_state_summary=_summary(active=[_state("INTRADAY_RECLAIM", age_bars=1)]),
        primary_zone=_zone(role=ZoneType.RESISTANCE.value, low=98.0, high=100.0),
        structure_state="SUPPORT_RECLAIM_CONFIRMED",
        daily_price_action=FOLLOW_THROUGH,
        current_price=103.5,
    )
    assert result["lifecycle_phase"] == LIFECYCLE_CONFIRMED


# ── I-096：`SUPPORT_TEST_CANDIDATE` 不得驅動 CLOSE_RECLAIM ──────


def test_support_test_candidate_resolves_to_support_test_not_close_reclaim():
    """碰觸不是收復：新狀態走 `SUPPORT_TEST`，**不得**產生 `CLOSE_RECLAIM`。

    這是 I-096 的核心——拆分前 touched-only 頂著
    `SUPPORT_RECLAIM_CANDIDATE` 進到上面那個 `or`，於是「只是碰到帶子」
    會被 Lifecycle 當成收復證據，改到 `market_state` 與 Bias。
    """
    signal, reason_codes = resolve_event_signal(
        _summary(),
        _zone(low=98.0, high=100.0),
        "SUPPORT_TEST_CANDIDATE",
    )
    assert signal != "CLOSE_RECLAIM"
    assert reason_codes == ["STRUCTURE_SUPPORT_TOUCH"]


def test_reclaim_candidate_still_resolves_to_close_reclaim():
    """**回歸防線**：真正的 undercut-reclaim 那一支行為不變。"""
    signal, reason_codes = resolve_event_signal(
        _summary(),
        _zone(low=98.0, high=100.0),
        "SUPPORT_RECLAIM_CANDIDATE",
    )
    assert signal == "CLOSE_RECLAIM"
    assert reason_codes == ["CLOSE_RECLAIM"]


def test_reversal_candidate_outranks_structure_touch():
    """仲裁順序：有 candidate event 佐證的 `REVERSAL_CANDIDATE` 優先序不變。

    新分支刻意插在它**後面**，所以新狀態只會在原本要落到
    EXTREME_VOLUME / NO_EVENT 的情況下改變答案。
    """
    signal, reason_codes = resolve_event_signal(
        _summary(candidates=[_state("REVERSAL_CANDIDATE")]),
        _zone(low=98.0, high=100.0),
        "SUPPORT_TEST_CANDIDATE",
    )
    assert reason_codes == ["REVERSAL_CANDIDATE"]


def test_support_test_candidate_lifecycle_is_testing():
    """端到端：新狀態在 lifecycle 層落在 `TESTING`，不是 `CONFIRMED`。"""
    result = resolve_lifecycle(
        event_state_summary=_summary(),
        primary_zone=_zone(low=98.0, high=100.0),
        structure_state="SUPPORT_TEST_CANDIDATE",
        daily_price_action=FOLLOW_THROUGH,
        current_price=99.0,
    )
    assert result["lifecycle_phase"] == LIFECYCLE_TESTING
