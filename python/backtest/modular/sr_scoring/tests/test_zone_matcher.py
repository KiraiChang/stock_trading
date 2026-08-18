"""ZoneMatcher（T-048 階段 A）。

**fixture 用的是 live 撈下來的真實 zone 形狀**，不是編出來的數字——判準是靠實測
543 組相鄰配對定下來的（`docs/todo.md` T-048「門檻掃描實測」），用假數字測等於
測另一個世界的參數。價格取自 2026-08-18 匯出的 `stock_sr_zones`（0050，311 列）。
"""
from __future__ import annotations

import itertools
from datetime import date

import pytest

from backtest.modular.sr_scoring.zone_matcher import (
    RELATION_CONTINUE,
    RELATION_MERGE,
    RELATION_RESHAPE,
    RELATION_SPLIT,
    ROLE_FLIPPED,
    ROLE_RESOLVED,
    ROLE_UNRESOLVED,
    CandidateZone,
    PreviousZone,
    TradingCalendar,
    is_same_zone,
    match_zones,
)


@pytest.fixture
def uid_factory():
    """可預測的身分產生器。真實實作用 uuid4，測試不該面對隨機值。"""
    counter = itertools.count(1)
    return lambda: f"Z{next(counter):03d}"


def prev(uid, low, high, role, method="atr", incarnation_role=None, last_seen_at=None,
         observed_absences=0):
    return PreviousZone(uid, low, high, method, role, incarnation_role, last_seen_at,
                        observed_absences)


def cur(low, high, role, method="atr"):
    return CandidateZone(low, high, method, role)


# ─────────────────────────────────────────────────────────────
# 身分延續：真實漂移
# ─────────────────────────────────────────────────────────────


def test_real_drift_2026_08_04_to_08_05_continues(uid_factory):
    """0050 atr 在 08-04 → 08-05 的四個 zone 都只是邊界微幅重算，身分必須延續。

    這些正是現行 `_zone_key()` 會判成「四個全新 zone」的案例——它把身分綁在
    小數點後 4 位的價格上。
    """
    previous = [
        prev("Z-A", 89.32, 97.18, "SUPPORT"),
        prev("Z-B", 95.12, 104.73, "AT_ZONE"),
        prev("Z-C", 100.32, 108.63, "AT_ZONE"),
        prev("Z-D", 100.37, 110.03, "AT_ZONE"),
    ]
    current = [
        cur(89.28, 97.22, "SUPPORT"),
        cur(95.08, 104.77, "AT_ZONE"),
        cur(100.28, 108.67, "AT_ZONE"),
        cur(101.08, 110.37, "AT_ZONE"),
    ]

    result = match_zones(previous, current, uid_factory)

    assert result.zone_uids == ["Z-A", "Z-B", "Z-C", "Z-D"]
    # 身分延續不寫血緣邊——那會是 parent == child 的自環，讓祖先回溯查詢無法終止。
    assert result.relations == []
    assert result.unmatched_previous == []
    assert result.terminated_previous == []


def test_zone_that_moved_too_far_is_a_new_identity(uid_factory):
    """同一次分析裡的 [105.37,114.78] → [107.08,114.82]：中心位移 0.102，超過門檻。

    這是**保守失敗**：多產生一個身分、鏈變短。相對於「把兩個真實 zone 併成一個、
    事件張冠李戴」的危險失敗，這個方向是刻意選的。
    """
    result = match_zones(
        [prev("Z-OLD", 105.37, 114.78, "RESISTANCE")],
        [cur(107.08, 114.82, "RESISTANCE")],
        uid_factory,
    )

    assert result.zone_uids == ["Z001"]
    assert result.relations == []
    assert result.unmatched_previous == ["Z-OLD"]


# ─────────────────────────────────────────────────────────────
# 角色翻轉：身分必須跨越它
# ─────────────────────────────────────────────────────────────


def test_role_flip_with_identical_bounds_keeps_identity(uid_factory):
    """0050 recent_pivot [104.73,105.37]：07-16 SUPPORT → 07-21 RESISTANCE。

    **邊界一動也沒動**，只有角色翻轉。現行 `_zone_key()` 含 role，所以這在今天
    必然是兩個不同的 key、鏈直接斷——這條測試就是那個 bug 的回歸測試。
    """
    result = match_zones(
        [prev("Z-PIVOT", 104.73, 105.37, "SUPPORT", method="recent_pivot")],
        [cur(104.73, 105.37, "RESISTANCE", method="recent_pivot")],
        uid_factory,
    )

    assert result.zone_uids == ["Z-PIVOT"]
    assert [(t.kind, t.from_role, t.to_role) for t in result.role_transitions] == [
        (ROLE_FLIPPED, "SUPPORT", "RESISTANCE")
    ]


def test_flip_through_at_zone_is_detected(uid_factory):
    """`RESISTANCE → AT_ZONE → SUPPORT`：兩個有向角色**並不相鄰**。

    live 有這個實例（0050 recent_pivot）。只比對相鄰兩次的 role 會漏掉它，
    所以判定規則是「新解析出的方向 ≠ 當前這一世的 role」。
    """
    # 第二步：方向變得無法解析。incarnation_role 仍是 RESISTANCE。
    step2 = match_zones(
        [prev("Z-P", 104.73, 105.37, "RESISTANCE", method="recent_pivot",
              incarnation_role="RESISTANCE")],
        [cur(104.73, 105.37, "AT_ZONE", method="recent_pivot")],
        uid_factory,
    )
    assert step2.zone_uids == ["Z-P"]
    assert [t.kind for t in step2.role_transitions] == [ROLE_UNRESOLVED]

    # 第三步：解析出 SUPPORT。相鄰的 role 是 AT_ZONE→SUPPORT，看起來像單純解析，
    # 但對照這一世的角色（RESISTANCE）就是翻轉。
    step3 = match_zones(
        [prev("Z-P", 104.73, 105.37, "AT_ZONE", method="recent_pivot",
              incarnation_role="RESISTANCE")],
        [cur(104.73, 105.37, "SUPPORT", method="recent_pivot")],
        uid_factory,
    )
    assert [(t.kind, t.from_role, t.to_role) for t in step3.role_transitions] == [
        (ROLE_FLIPPED, "RESISTANCE", "SUPPORT")
    ]


def test_at_zone_returning_to_same_role_is_resolved_not_flipped(uid_factory):
    """`RESISTANCE → AT_ZONE → RESISTANCE` 是解析回原角色，不是翻轉。

    live 的 3 筆 `X→AT_ZONE→Y` 裡有 2 筆是這種。把它算成翻轉會讓真正的翻轉
    被雜訊淹沒。
    """
    result = match_zones(
        [prev("Z-P", 104.73, 105.37, "AT_ZONE", incarnation_role="RESISTANCE")],
        [cur(104.73, 105.37, "RESISTANCE")],
        uid_factory,
    )

    assert [(t.kind, t.from_role, t.to_role) for t in result.role_transitions] == [
        (ROLE_RESOLVED, "AT_ZONE", "RESISTANCE")
    ]


def test_long_at_zone_chain_produces_no_role_transition(uid_factory):
    """live 有一條連續 16 次分析都是 AT_ZONE 的鏈。

    一直沒解析出方向不是「轉換」，不該每天記一筆——那會把 transition 表灌成流水帳。
    """
    result = match_zones(
        [prev("Z-P", 95.12, 104.73, "AT_ZONE")],
        [cur(95.08, 104.77, "AT_ZONE")],
        uid_factory,
    )

    assert result.zone_uids == ["Z-P"]
    assert result.role_transitions == []


def test_first_resolution_is_resolved_not_flipped(uid_factory):
    """身分第一次解析出方向：沒有「前一世的角色」可比，是 RESOLVED 不是 FLIPPED。"""
    result = match_zones(
        [prev("Z-P", 95.12, 104.73, "AT_ZONE")],
        [cur(95.08, 104.77, "SUPPORT")],
        uid_factory,
    )

    assert [(t.kind, t.to_role) for t in result.role_transitions] == [
        (ROLE_RESOLVED, "SUPPORT")
    ]


# ─────────────────────────────────────────────────────────────
# 血緣：分裂 / 合併 / 重整
# ─────────────────────────────────────────────────────────────


def test_real_duplication_falls_below_threshold_and_becomes_new_identity(uid_factory):
    """0050 atr 08-05 的重複現場：[95.12,104.73] 之後同時存在
    [95.08,104.77] 與 [95.73,105.37]。

    第二個的中心位移是 0.0649，**剛好超過 0.06 門檻**，所以判成新生而不是分裂。
    這條把「門檻就在這個案例附近」這件事釘住——日後有人放寬門檻時，
    這條會變紅並強迫他面對 N→M 的代價（實測 0.10 時 N→M 從 3 筆暴增到 10 筆）。
    """
    result = match_zones(
        [prev("Z-B", 95.12, 104.73, "AT_ZONE")],
        [cur(95.08, 104.77, "AT_ZONE"), cur(95.73, 105.37, "AT_ZONE")],
        uid_factory,
    )

    assert result.zone_uids == ["Z-B", "Z001"]
    assert result.relations == []          # 一個延續、一個新生，都不是血緣邊


def test_split_gives_all_children_new_identities(uid_factory):
    """1→N：parent 終止，**沒有任何 child 繼承 parent 的身分**。

    讓某個 child 繼承必然是任意選擇，而且會讓血緣圖說謊。血緣由 relations 表達。
    """
    result = match_zones(
        [prev("Z-P", 100.00, 110.00, "SUPPORT")],
        [cur(100.10, 109.90, "SUPPORT"), cur(100.20, 110.10, "SUPPORT")],
        uid_factory,
    )

    assert result.zone_uids == ["Z001", "Z002"]
    assert {(r.parent_zone_uid, r.child_zone_uid, r.relation) for r in result.relations} == {
        ("Z-P", "Z001", RELATION_SPLIT),
        ("Z-P", "Z002", RELATION_SPLIT),
    }
    assert result.role_transitions == []   # 新身分沒有「轉換」，只有誕生
    # parent 有 child，所以不是 unmatched；但身分本身結束了，階段 B 要寫成終態。
    assert result.terminated_previous == ["Z-P"]
    assert result.unmatched_previous == []


def test_merge_records_both_parents(uid_factory):
    result = match_zones(
        [prev("Z-P1", 100.00, 110.00, "SUPPORT"), prev("Z-P2", 100.10, 109.90, "SUPPORT")],
        [cur(100.05, 109.95, "SUPPORT")],
        uid_factory,
    )

    assert result.zone_uids == ["Z001"]
    assert {(r.parent_zone_uid, r.relation) for r in result.relations} == {
        ("Z-P1", RELATION_MERGE),
        ("Z-P2", RELATION_MERGE),
    }


def test_chained_component_does_not_fabricate_unmatched_edges(uid_factory):
    """**元件連通 ≠ 全交叉。**

    P0–C0、P1–C0、P1–C1 相連但 P0 與 C1 並不匹配（中心差 1.5 = 寬度的 15%，
    是判準明確否決的）。對整個元件做笛卡爾積會憑空生出 P0→C1 這條父子關係——
    那與本模組「不猜血緣」的立場直接矛盾，而且邊數會隨元件大小平方成長。
    """
    previous = [
        prev("Z-P0", 100.0, 110.0, "SUPPORT"),
        prev("Z-P1", 101.0, 111.0, "SUPPORT"),
    ]
    current = [cur(100.5, 110.5, "SUPPORT"), cur(101.5, 111.5, "SUPPORT")]

    result = match_zones(previous, current, uid_factory)

    edges = {(r.parent_zone_uid, r.child_zone_uid) for r in result.relations}
    assert ("Z-P0", "Z002") not in edges, "P0 與 C1 不匹配，不該有這條邊"
    assert edges == {("Z-P0", "Z001"), ("Z-P1", "Z001"), ("Z-P1", "Z002")}


def test_incarnation_role_is_returned_so_callers_need_no_bookkeeping(uid_factory):
    """`incarnation_role` 的前進規則必須由本模組給，不能讓每個呼叫端自己重寫。

    漏掉「翻轉後要前進」的後果是**靜默的**：`RESISTANCE → SUPPORT → RESISTANCE`
    只會報出第一次翻轉，第二次因為 `cur.role == incarnation_role` 而被判成沒有轉換——
    那正是這個模組要消滅的斷鏈，卻發生在模組外面、沒有測試抓得到。
    """
    first = match_zones(
        [prev("Z-P", 104.73, 105.37, "RESISTANCE", incarnation_role="RESISTANCE")],
        [cur(104.73, 105.37, "SUPPORT")],
        uid_factory,
    )
    assert [t.kind for t in first.role_transitions] == [ROLE_FLIPPED]
    assert first.incarnation_roles == ["SUPPORT"]      # 已前進

    # 把上一輪算好的值原樣帶回來，第二次翻轉就抓得到
    second = match_zones(
        [prev("Z-P", 104.73, 105.37, "SUPPORT",
              incarnation_role=first.incarnation_roles[0])],
        [cur(104.73, 105.37, "RESISTANCE")],
        uid_factory,
    )
    assert [(t.kind, t.from_role, t.to_role) for t in second.role_transitions] == [
        (ROLE_FLIPPED, "SUPPORT", "RESISTANCE")
    ]


def test_incarnation_role_carries_through_at_zone(uid_factory):
    """AT_ZONE 期間沿用這一世原本的角色，不能歸零——歸零會讓下一次解析變成
    「第一次解析」而不是翻轉。"""
    result = match_zones(
        [prev("Z-P", 104.73, 105.37, "RESISTANCE", incarnation_role="RESISTANCE")],
        [cur(104.73, 105.37, "AT_ZONE")],
        uid_factory,
    )

    assert result.incarnation_roles == ["RESISTANCE"]


def test_new_identity_starts_a_fresh_incarnation(uid_factory):
    """新身分沒有前一世，AT_ZONE 起手時 incarnation_role 應為 None。"""
    result = match_zones([], [cur(100.0, 110.0, "AT_ZONE"), cur(200.0, 210.0, "SUPPORT")],
                         uid_factory)

    assert result.incarnation_roles == [None, "SUPPORT"]


def test_unknown_previous_role_does_not_emit_a_transition(uid_factory):
    """`role` 從 DB 讀回來是自由字串。未知值進 AT_ZONE 不該被說成
    「有向 → AT_ZONE」——那個語意是假的。"""
    result = match_zones(
        [prev("Z-P", 100.0, 110.0, "UNKNOWN")],
        [cur(100.0, 110.0, "AT_ZONE")],
        uid_factory,
    )

    assert result.role_transitions == []


def test_reshape_records_every_edge_and_guesses_nothing(uid_factory):
    """N→M：2 個 parent 與 2 個 child 全部互相匹配。

    這裡沒有任何一條邊是「真的」，所以四條邊全部記為 RESHAPE。
    誠實記錄一次無法解析的重整，好過編一組看起來合理的父子關係。
    """
    result = match_zones(
        [prev("Z-P1", 100.00, 110.00, "SUPPORT"), prev("Z-P2", 100.10, 110.10, "SUPPORT")],
        [cur(100.05, 110.05, "SUPPORT"), cur(100.15, 110.15, "SUPPORT")],
        uid_factory,
    )

    assert result.zone_uids == ["Z001", "Z002"]
    assert len(result.relations) == 4
    assert {r.relation for r in result.relations} == {RELATION_RESHAPE}


# ─────────────────────────────────────────────────────────────
# 判準本身
# ─────────────────────────────────────────────────────────────


def test_method_never_crosses():
    """ATR zone 與 volume profile zone 是不同方法算出來的東西，
    價格碰巧重疊不代表是同一個結構。"""
    assert not is_same_zone(
        prev("Z", 100.0, 110.0, "SUPPORT", method="atr"),
        cur(100.0, 110.0, "SUPPORT", method="volume_profile"),
    )


def test_role_is_not_part_of_identity():
    assert is_same_zone(
        prev("Z", 104.73, 105.37, "SUPPORT"),
        cur(104.73, 105.37, "RESISTANCE"),
    )


def test_wide_zone_is_not_penalised_for_being_wide():
    """判準刻意是尺度無關的。純 IoU 會讓寬 zone 漂一點點就掉出門檻——
    live 有 IoU 0.878 但中心位移只有 0.065 的案例。"""
    narrow = is_same_zone(prev("Z", 100.00, 101.00, "SUPPORT"), cur(100.03, 101.03, "SUPPORT"))
    wide = is_same_zone(prev("Z", 100.00, 110.00, "SUPPORT"), cur(100.30, 110.30, "SUPPORT"))
    assert narrow == wide is True


def test_width_change_beyond_threshold_breaks_match():
    """中心沒動但寬度暴增：那是重新算出來的另一個結構，不是同一個 zone 漂移。"""
    assert not is_same_zone(
        prev("Z", 100.0, 110.0, "SUPPORT"),
        cur(97.0, 113.0, "SUPPORT"),
    )


def test_degenerate_zero_width_zone_does_not_divide_by_zero():
    assert is_same_zone(prev("Z", 100.0, 100.0, "SUPPORT"), cur(100.0, 100.0, "SUPPORT"))
    assert not is_same_zone(prev("Z", 100.0, 100.0, "SUPPORT"), cur(100.1, 100.1, "SUPPORT"))


# 台股 2026-08 的真實交易日（週末不交易）。matcher 不自己判斷假日——
# 專案既有做法是從 candles 的 distinct 日期推得，這裡沿用同一個事實來源。
AUG = TradingCalendar(tuple(
    date(2026, 8, d) for d in
    (3, 4, 5, 6, 7, 10, 11, 12, 13, 14, 17, 18, 19, 20, 21, 24, 25, 26, 27, 28, 31)
))


def test_trading_calendar_skips_weekends():
    """週五 → 週一是 1 個交易日，不是 3 天。用日曆天算會讓每個週末都虛胖 2 天。"""
    assert AUG.sessions_between(date(2026, 8, 14), date(2026, 8, 17)) == 1
    assert (date(2026, 8, 17) - date(2026, 8, 14)).days == 3


def test_trading_calendar_ignores_non_trading_endpoints():
    """分析可能落在盤後或假日，端點本身不必是交易日。"""
    assert AUG.sessions_between(date(2026, 8, 15), date(2026, 8, 18)) == 2  # 17、18
    assert AUG.sessions_between(date(2026, 8, 18), date(2026, 8, 18)) == 0


def test_absence_beyond_trading_day_limit_loses_eligibility(uid_factory):
    """缺席太久的身分**根本不進候選集合**，不是「進去但配不上」。

    這是持久化之後才出現的風險：previous 從「上一次分析的 zone」變成
    「所有還活著的身分」，沒有閘門的話一個消失數月的 zone 會被 ATR 碰巧算出的
    相近區間接回來。
    """
    old = prev("Z-OLD", 104.73, 105.37, "SUPPORT", last_seen_at=date(2026, 8, 3))

    result = match_zones([old], [cur(104.73, 105.37, "SUPPORT")], uid_factory,
                         as_of=date(2026, 8, 31), calendar=AUG,
                         max_absence_trading_days=5)

    assert result.zone_uids == ["Z001"], "失格的身分不該延續"
    assert result.expired_previous == ["Z-OLD"]
    # 失格與「這次沒配到」語意不同，不可混在 unmatched_previous
    assert result.unmatched_previous == []


def test_absence_within_trading_day_limit_still_continues(uid_factory):
    old = prev("Z-OLD", 104.73, 105.37, "SUPPORT", last_seen_at=date(2026, 8, 14))

    result = match_zones([old], [cur(104.73, 105.37, "SUPPORT")], uid_factory,
                         as_of=date(2026, 8, 18), calendar=AUG,
                         max_absence_trading_days=5)

    assert result.zone_uids == ["Z-OLD"]
    assert result.expired_previous == []


def test_third_observed_absence_loses_eligibility(uid_factory):
    """`MAX_OBSERVED_ABSENCES = 3` 是「小於才有資格」：缺席 2 次還在，第 3 次收攤。

    這一軸與交易日無關——它量的是「我們看了幾次都沒看到」，
    而交易日量的是 wall-clock 陳舊度。單靠時間軸分不出「zone 消失了」與
    「我們根本沒看」（實測 2330 全期只有 4 次分析、橫跨 5 週）。
    """
    still_ok = prev("Z-A", 104.73, 105.37, "SUPPORT", observed_absences=2)
    expired = prev("Z-B", 200.00, 201.00, "SUPPORT", observed_absences=3)

    result = match_zones([still_ok, expired],
                         [cur(104.73, 105.37, "SUPPORT"), cur(200.00, 201.00, "SUPPORT")],
                         uid_factory)

    assert result.zone_uids[0] == "Z-A"
    assert result.zone_uids[1] == "Z001", "缺席 3 次的身分不該被接回來"
    assert result.expired_previous == ["Z-B"]


def test_observed_absences_are_advanced_by_the_module(uid_factory):
    """配到歸零、沒配到 +1——**由模組算**，不讓呼叫端自己加減。

    比照 incarnation_roles 的教訓：這種 bookkeeping 算錯是靜默的，
    而且錯誤發生在模組外面、沒有測試抓得到。
    """
    matched = prev("Z-HIT", 104.73, 105.37, "SUPPORT", observed_absences=1)
    missed = prev("Z-MISS", 300.00, 301.00, "SUPPORT", observed_absences=1)

    result = match_zones([matched, missed], [cur(104.73, 105.37, "SUPPORT")], uid_factory)

    assert result.next_observed_absences == {"Z-HIT": 0, "Z-MISS": 2}


def test_eligibility_gate_is_skipped_without_persistence(uid_factory):
    """還沒接上持久化的呼叫端沒有 calendar／last_seen_at，行為要與階段 A 相同。"""
    result = match_zones(
        [prev("Z-OLD", 104.73, 105.37, "SUPPORT")],
        [cur(104.73, 105.37, "SUPPORT")],
        uid_factory,
        as_of=date(2026, 8, 18),   # 有 as_of 但沒有 calendar
    )

    assert result.zone_uids == ["Z-OLD"]
    assert result.expired_previous == []


def test_empty_previous_makes_everything_a_birth(uid_factory):
    result = match_zones([], [cur(100.0, 110.0, "SUPPORT")], uid_factory)

    assert result.zone_uids == ["Z001"]
    assert result.relations == []
    assert result.unmatched_previous == []


def test_empty_current_reports_all_previous_as_unmatched(uid_factory):
    result = match_zones([prev("Z-P", 100.0, 110.0, "SUPPORT")], [], uid_factory)

    assert result.zone_uids == []
    assert result.relations == []
    assert result.unmatched_previous == ["Z-P"]


# ── review 修正的回歸測試 ──


def test_calendar_rejects_descending_input():
    """`db.fetch_market_trading_days` 是 `ORDER BY d DESC` 的**字串**清單。

    照文件直接接上去、只轉型別卻忘了 reverse 的話，`sessions_between` 會回 0 或負數，
    時間軸閘門**被靜默關掉**——正是這一批要修的問題。所以建構時就要擋。
    """
    with pytest.raises(ValueError):
        TradingCalendar((date(2026, 8, 18), date(2026, 8, 17)))


def test_calendar_rejects_strings():
    with pytest.raises(TypeError):
        TradingCalendar(("2026-08-17", "2026-08-18"))


def test_calendar_from_iterable_accepts_the_real_source_shape():
    """`fetch_market_trading_days()` 的輸出（降冪字串）可以直接丟進來。"""
    cal = TradingCalendar.from_iterable(["2026-08-18", "2026-08-17", "2026-08-14"])

    assert cal.days == (date(2026, 8, 14), date(2026, 8, 17), date(2026, 8, 18))
    assert cal.sessions_between(date(2026, 8, 14), date(2026, 8, 18)) == 2


def test_expired_identities_also_advance_their_absence_count(uid_factory):
    """失格的身分也要 +1，這是與 repo 端的握手。

    `ListLive` 用 `observed_absences <= 上限` 才撈得到剛達上限的身分（否則收攤流程
    是不可達的死碼）。但少了這裡的 +1，它每次分析都會再被撈出來、再失格一次，
    `EXPIRED_BY_ABSENCE` 會被重複寫入。
    """
    expired = prev("Z-EXP", 104.73, 105.37, "SUPPORT", observed_absences=3)

    result = match_zones([expired], [], uid_factory)

    assert result.expired_previous == ["Z-EXP"]
    assert result.next_observed_absences == {"Z-EXP": 4}


def test_terminated_parents_are_not_given_an_absence_count(uid_factory):
    """SPLIT / MERGE / RESHAPE 的 parent 身分已經結束、child 另取新 uid。

    照欄位說明「只負責存回去」而遍歷整個 dict 去 upsert 的呼叫端，
    會把剛判定終止的身分又寫回一筆 observed_absences=0 的活躍狀態。
    """
    result = match_zones(
        [prev("Z-P", 100.00, 110.00, "SUPPORT")],
        [cur(100.10, 109.90, "SUPPORT"), cur(100.20, 110.10, "SUPPORT")],
        uid_factory,
    )

    assert result.terminated_previous == ["Z-P"]
    assert "Z-P" not in result.next_observed_absences
