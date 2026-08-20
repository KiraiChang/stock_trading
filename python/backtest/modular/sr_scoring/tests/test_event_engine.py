from __future__ import annotations

from ..event_engine import (
    EXTREME_VOLUME_THRESHOLD,
    LIFECYCLE_ACTIVE,
    LIFECYCLE_CANDIDATE,
    LIFECYCLE_CONFIRMED,
    LIFECYCLE_EXPIRED,
    LIFECYCLE_RESOLVED,
    build_event_state_summary,
    detect_market_events,
    normalize_market_events,
    zone_identity_key,
    zone_interaction,
)
from ..types import RecentValidation, VolumeConfirmation, ZoneType

# 沿用 decision 測試的 ZoneScore 建構 helper，避免重複整包 dataclass 欄位。
from .test_decision_engine import _zone


def _types(events: list[dict]) -> set[str]:
    return {e["type"] for e in events}


def test_zone_interaction_reports_breakdown_and_penetration():
    z = _zone(low=98.0, high=100.0)

    interaction = zone_interaction(z, current_price=99.0, candle_high=101.0, candle_low=96.0, candle_close=97.0)

    assert interaction["touched"] is True
    assert interaction["closed_below"] is True
    assert interaction["closed_above"] is False
    assert interaction["penetration_pct"] > 0
    assert interaction["price_action_evidence"]["close_relative_to_zone"] == "BELOW_ZONE"
    assert interaction["price_action_evidence"]["reclaim_type"] == "NONE"


def test_zone_interaction_reports_price_action_reclaim_evidence():
    z = _zone(low=98.0, high=100.0)

    interaction = zone_interaction(z, current_price=101.0, candle_high=102.0, candle_low=97.0, candle_close=101.0)

    evidence = interaction["price_action_evidence"]
    assert evidence["reclaim_type"] == "UNDERCUT_RECLAIM"
    assert evidence["close_relative_to_zone"] == "ABOVE_ZONE"
    assert evidence["penetration_ratio"] > 0


def test_extreme_volume_is_symbol_level_context_event_with_null_zone_ref():
    # 極端量能是整檔層級事件：即使沒有任何 zone 被觸碰也要輸出，且不綁定 zone。
    z = _zone(low=98.0, high=100.0, relative_volume=EXTREME_VOLUME_THRESHOLD + 0.3)

    events = detect_market_events([z], current_price=110.0, candle_high=None, candle_low=None, candle_close=None)

    assert [e["type"] for e in events] == ["EXTREME_VOLUME"]
    assert events[0]["direction"] == "NEUTRAL"
    assert events[0]["zone_ref"] is None


def test_extreme_volume_not_emitted_below_threshold():
    z = _zone(low=98.0, high=100.0, relative_volume=EXTREME_VOLUME_THRESHOLD - 0.5)

    events = detect_market_events([z], current_price=110.0, candle_high=None, candle_low=None, candle_close=None)

    assert "EXTREME_VOLUME" not in _types(events)


def test_high_volume_breakdown_on_support_closed_below():
    z = _zone(low=98.0, high=100.0, relative_volume=2.0)

    events = detect_market_events([z], current_price=99.0, candle_high=99.0, candle_low=96.0, candle_close=97.0)

    breakdown = [e for e in events if e["type"] == "HIGH_VOLUME_BREAKDOWN"]
    assert len(breakdown) == 1
    assert breakdown[0]["direction"] == "BEARISH"
    assert breakdown[0]["zone_ref"] is not None


def test_intraday_reclaim_excludes_reversal_candidate_for_same_zone():
    # 收回區間上緣同時滿足 reclaim 與 reversal 條件時，只掛更明確的 INTRADAY_RECLAIM。
    z = _zone(low=98.0, high=100.0, relative_volume=1.0)

    events = detect_market_events([z], current_price=99.0, candle_high=101.0, candle_low=97.0, candle_close=101.0)

    # SUPPORT_RETEST_HELD 是階段 D 新增的事實事件（decision_visible=False），
    # 與這條測試要釘的「只掛更明確的 INTRADAY_RECLAIM」互不衝突。
    assert _types(events) == {"INTRADAY_RECLAIM", "SUPPORT_RETEST_HELD"}


def test_high_volume_intraday_break_and_reclaim_keeps_full_event_chain():
    z = _zone(low=98.0, high=100.0, relative_volume=3.0, volume_confirmation=VolumeConfirmation.FAILED.value)

    events = detect_market_events([z], current_price=101.0, candle_high=102.0, candle_low=97.0, candle_close=101.0)

    assert [event["type"] for event in events] == [
        "EXTREME_VOLUME",
        "HIGH_VOLUME_BREAKDOWN",
        "INTRADAY_RECLAIM",
        "REVERSAL_CANDIDATE",
        "SUPPORT_RETEST_HELD",
    ]


def test_0050_break_reclaim_reversal_resolves_active_bearish_event():
    # 0050 fixture：同一個支撐區先高量跌破、再收回並形成反轉候選時，
    # raw event chain 要完整保留，但 active bearish gate 必須被解除。
    z = _zone(low=98.0, high=100.0, relative_volume=3.0, volume_confirmation=VolumeConfirmation.FAILED.value)

    events = detect_market_events([z], current_price=101.0, candle_high=102.0, candle_low=97.0, candle_close=101.0)
    summary = build_event_state_summary(events)

    assert [event["type"] for event in events] == [
        "EXTREME_VOLUME",
        "HIGH_VOLUME_BREAKDOWN",
        "INTRADAY_RECLAIM",
        "REVERSAL_CANDIDATE",
        "SUPPORT_RETEST_HELD",
    ]
    assert summary["version"] == "event-lifecycle-p2"
    assert summary["active_bearish_events"] == []
    assert summary["market_state"] == "RECLAIM_ATTEMPT"
    breakdown = next(state for state in summary["states"] if state["event_family"] == "SUPPORT_BREAKDOWN")
    assert breakdown["active"] is False
    assert breakdown["state"] == LIFECYCLE_RESOLVED
    assert breakdown["resolved_by"] == "INTRADAY_RECLAIM"
    reclaim = next(state for state in summary["states"] if state["event_family"] == "SUPPORT_RECLAIM")
    assert reclaim["state"] == LIFECYCLE_CONFIRMED
    assert reclaim["active"] is True
    reversal = next(state for state in summary["states"] if state["event_family"] == "SUPPORT_REVERSAL")
    assert reversal["state"] == LIFECYCLE_CANDIDATE
    assert reversal["active"] is False


def test_unresolved_breakdown_remains_active_bearish_event():
    z = _zone(low=98.0, high=100.0, relative_volume=2.0, volume_confirmation=VolumeConfirmation.FAILED.value)

    events = detect_market_events([z], current_price=97.0, candle_high=99.0, candle_low=96.0, candle_close=97.0)
    summary = build_event_state_summary(events)

    assert [event["type"] for event in events] == ["HIGH_VOLUME_BREAKDOWN"]
    assert summary["market_state"] == "BREAKDOWN_RISK"
    assert [event["type"] for event in summary["active_bearish_events"]] == ["HIGH_VOLUME_BREAKDOWN"]
    breakdown = summary["active_bearish_events"][0]
    assert breakdown["state"] == LIFECYCLE_CONFIRMED
    assert breakdown["confirmation_state"] == "CONFIRMED_CLOSE_BELOW"


def test_intrabar_breakdown_without_close_confirmation_is_candidate_not_active():
    z = _zone(low=98.0, high=100.0, relative_volume=2.0, volume_confirmation=VolumeConfirmation.FAILED.value)

    events = detect_market_events([z], current_price=101.0, candle_high=102.0, candle_low=97.0, candle_close=99.0)
    summary = build_event_state_summary(events)

    breakdown = next(event for event in summary["states"] if event["event_family"] == "SUPPORT_BREAKDOWN")
    assert breakdown["state"] == LIFECYCLE_CANDIDATE
    assert breakdown["active"] is False
    assert summary["active_bearish_events"] == []
    assert summary["market_state"] != "BREAKDOWN_RISK"


def test_reversal_candidate_when_support_held_inside():
    z = _zone(low=98.0, high=100.0, relative_volume=1.0)

    events = detect_market_events([z], current_price=99.0, candle_high=99.5, candle_low=97.0, candle_close=99.0)

    assert _types(events) == {"REVERSAL_CANDIDATE", "SUPPORT_RETEST_HELD"}
    summary = build_event_state_summary(events)
    reversal = summary["candidates"][0]
    assert reversal["state"] == LIFECYCLE_CANDIDATE
    assert reversal["active"] is False
    assert summary["active_bullish_events"] == []


def test_expired_support_held_inside_does_not_emit_reversal_candidate():
    z = _zone(low=98.0, high=100.0, relative_volume=1.0, recent_validation=RecentValidation.EXPIRED.value)

    events = detect_market_events([z], current_price=99.0, candle_high=99.5, candle_low=97.0, candle_close=99.0)

    assert "REVERSAL_CANDIDATE" not in _types(events)


def test_resistance_zone_does_not_produce_support_events():
    # 階段 D 之後壓力 zone 會產生 RESISTANCE_BREAKOUT，但**不能**掉進支撐側的三個分支。
    z = _zone(role=ZoneType.RESISTANCE.value, low=98.0, high=100.0, relative_volume=2.0)

    events = detect_market_events([z], current_price=99.0, candle_high=101.0, candle_low=96.0, candle_close=97.0)

    support_side = {"HIGH_VOLUME_BREAKDOWN", "INTRADAY_RECLAIM", "REVERSAL_CANDIDATE", "SUPPORT_RETEST_HELD"}
    assert _types(events) & support_side == set()


def test_summary_groups_confirmed_and_candidate_states_separately():
    events = normalize_market_events([
        {
            "type": "HIGH_VOLUME_BREAKDOWN",
            "direction": "BEARISH",
            "lifecycle_state": LIFECYCLE_CONFIRMED,
            "active": True,
            "zone_ref": {"role": "SUPPORT", "price_low": 98.0, "price_high": 100.0},
        },
        {
            "type": "REVERSAL_CANDIDATE",
            "direction": "BULLISH",
            "lifecycle_state": LIFECYCLE_CANDIDATE,
            "active": False,
            "zone_ref": {"role": "SUPPORT", "price_low": 90.0, "price_high": 92.0},
        },
    ])

    summary = build_event_state_summary(events)

    assert [state["type"] for state in summary["confirmed"]] == ["HIGH_VOLUME_BREAKDOWN"]
    assert [state["type"] for state in summary["candidates"]] == ["REVERSAL_CANDIDATE"]
    assert [state["type"] for state in summary["active_bearish_events"]] == ["HIGH_VOLUME_BREAKDOWN"]


def test_previous_active_breakdown_carries_forward_without_new_events():
    previous = [{
        "type": "HIGH_VOLUME_BREAKDOWN",
        "event_family": "SUPPORT_BREAKDOWN",
        "event_scope": "ZONE",
        "zone_key": "SUPPORT:98.0000:100.0000",
        "root_event_type": "HIGH_VOLUME_BREAKDOWN",
        "latest_event_type": "HIGH_VOLUME_BREAKDOWN",
        "direction": "BEARISH",
        "state": LIFECYCLE_CONFIRMED,
        "active": True,
        "reason_codes": ["SUPPORT_CLOSED_BELOW"],
    }]

    summary = build_event_state_summary([], previous_states=previous)

    assert [state["type"] for state in summary["active_bearish_events"]] == ["HIGH_VOLUME_BREAKDOWN"]
    assert summary["market_state"] == "BREAKDOWN_RISK"
    assert summary["active_bearish_events"][0]["carried_from_previous"] is True


def test_current_reclaim_resolves_previous_active_breakdown():
    previous = [{
        "type": "HIGH_VOLUME_BREAKDOWN",
        "event_family": "SUPPORT_BREAKDOWN",
        "event_scope": "ZONE",
        "zone_key": "SUPPORT:98.0000:100.0000",
        "root_event_type": "HIGH_VOLUME_BREAKDOWN",
        "latest_event_type": "HIGH_VOLUME_BREAKDOWN",
        "direction": "BEARISH",
        "state": LIFECYCLE_CONFIRMED,
        "active": True,
        "reason_codes": ["SUPPORT_CLOSED_BELOW"],
    }]
    events = normalize_market_events([{
        "type": "INTRADAY_RECLAIM",
        "direction": "BULLISH",
        "lifecycle_state": LIFECYCLE_CONFIRMED,
        "active": True,
        "zone_ref": {"role": "SUPPORT", "price_low": 98.0, "price_high": 100.0},
    }])

    summary = build_event_state_summary(events, previous_states=previous)

    assert summary["active_bearish_events"] == []
    assert summary["market_state"] == "RECLAIM_ATTEMPT"
    breakdown = next(state for state in summary["states"] if state["event_family"] == "SUPPORT_BREAKDOWN")
    assert breakdown["state"] == LIFECYCLE_RESOLVED
    assert breakdown["resolved_by"] == "INTRADAY_RECLAIM"


def _carried_breakdown(**overrides) -> dict:
    base = {
        "type": "HIGH_VOLUME_BREAKDOWN",
        "event_family": "SUPPORT_BREAKDOWN",
        "event_scope": "ZONE",
        "zone_key": "SUPPORT:98.0000:100.0000",
        "root_event_type": "HIGH_VOLUME_BREAKDOWN",
        "latest_event_type": "HIGH_VOLUME_BREAKDOWN",
        "direction": "BEARISH",
        "state": LIFECYCLE_CONFIRMED,
        "active": True,
        "reason_codes": ["SUPPORT_CLOSED_BELOW"],
        "expires_after_bars": 2,
    }
    base.update(overrides)
    return base


def test_carried_active_event_expires_after_reaching_bar_threshold():
    # age_bars=1 再 carry 一次 → 2，達 expires_after_bars=2 門檻 → EXPIRED。
    previous = [_carried_breakdown(age_bars=1)]

    summary = build_event_state_summary([], previous_states=previous)

    breakdown = next(s for s in summary["states"] if s["event_family"] == "SUPPORT_BREAKDOWN")
    assert breakdown["state"] == LIFECYCLE_EXPIRED
    assert breakdown["active"] is False
    assert breakdown["age_bars"] == 2
    assert "EVENT_EXPIRED_STALE" in breakdown["reason_codes"]
    assert [s["type"] for s in summary["expired"]] == ["HIGH_VOLUME_BREAKDOWN"]
    assert summary["active_bearish_events"] == []


def test_carried_active_event_survives_below_expiry_threshold():
    # age_bars=0 再 carry 一次 → 1，未達 expires_after_bars=2 → 續 active。
    previous = [_carried_breakdown(age_bars=0)]

    summary = build_event_state_summary([], previous_states=previous)

    breakdown = next(s for s in summary["states"] if s["event_family"] == "SUPPORT_BREAKDOWN")
    assert breakdown["state"] == LIFECYCLE_CONFIRMED
    assert breakdown["active"] is True
    assert breakdown["age_bars"] == 1
    assert summary["expired"] == []
    assert [s["type"] for s in summary["active_bearish_events"]] == ["HIGH_VOLUME_BREAKDOWN"]


def test_fresh_detection_resets_carried_event_age():
    # 已接近過期（age_bars=1、門檻2）的 carried 事件，被當根新偵測同區事件覆蓋 → 存活
    # 計數歸零、續 active，不會過期。
    previous = [_carried_breakdown(age_bars=1)]
    events = normalize_market_events([{
        "type": "HIGH_VOLUME_BREAKDOWN",
        "direction": "BEARISH",
        "lifecycle_state": LIFECYCLE_CONFIRMED,
        "active": True,
        "expires_after_bars": 2,
        "zone_ref": {"role": "SUPPORT", "price_low": 98.0, "price_high": 100.0},
    }])

    summary = build_event_state_summary(events, previous_states=previous)

    breakdown = next(s for s in summary["states"] if s["event_family"] == "SUPPORT_BREAKDOWN")
    assert breakdown["state"] == LIFECYCLE_CONFIRMED
    assert breakdown["active"] is True
    assert breakdown["age_bars"] == 0
    assert summary["expired"] == []


def test_carried_event_without_expires_uses_default_threshold():
    # 未帶 expires_after_bars 的 carried 事件套 DEFAULT_EVENT_EXPIRES_AFTER_BARS(3)：
    # age_bars=2 再 carry → 3，達預設門檻 → EXPIRED，確保無永生事件。
    previous = [_carried_breakdown(age_bars=2)]
    previous[0].pop("expires_after_bars")

    summary = build_event_state_summary([], previous_states=previous)

    breakdown = next(s for s in summary["states"] if s["event_family"] == "SUPPORT_BREAKDOWN")
    assert breakdown["state"] == LIFECYCLE_EXPIRED
    assert breakdown["active"] is False
    assert breakdown["age_bars"] == 3


def test_candidate_state_never_enters_active_gating_even_if_active_flag_is_true():
    events = normalize_market_events([{
        "type": "HIGH_VOLUME_BREAKDOWN",
        "direction": "BEARISH",
        "lifecycle_state": LIFECYCLE_CANDIDATE,
        "active": True,
        "zone_ref": {"role": "SUPPORT", "price_low": 98.0, "price_high": 100.0},
    }])

    summary = build_event_state_summary(events)

    breakdown = next(s for s in summary["states"] if s["event_family"] == "SUPPORT_BREAKDOWN")
    assert breakdown["state"] == LIFECYCLE_CANDIDATE
    assert breakdown["active"] is False
    assert summary["active_bearish_events"] == []
    assert summary["market_state"] == "NORMAL"


def test_reversal_family_requires_active_state_before_gating():
    events = normalize_market_events([{
        "type": "REVERSAL_CANDIDATE",
        "direction": "BULLISH",
        "lifecycle_state": LIFECYCLE_CONFIRMED,
        "active": True,
        "zone_ref": {"role": "SUPPORT", "price_low": 98.0, "price_high": 100.0},
    }])

    confirmed_summary = build_event_state_summary(events)
    confirmed = next(s for s in confirmed_summary["states"] if s["event_family"] == "SUPPORT_REVERSAL")
    assert confirmed["state"] == LIFECYCLE_CONFIRMED
    assert confirmed["active"] is False
    assert confirmed_summary["active_bullish_events"] == []

    active_events = normalize_market_events([{
        "type": "REVERSAL_CANDIDATE",
        "direction": "BULLISH",
        "lifecycle_state": LIFECYCLE_ACTIVE,
        "active": True,
        "zone_ref": {"role": "SUPPORT", "price_low": 98.0, "price_high": 100.0},
    }])
    active_summary = build_event_state_summary(active_events)

    active = next(s for s in active_summary["states"] if s["event_family"] == "SUPPORT_REVERSAL")
    assert active["state"] == LIFECYCLE_ACTIVE
    assert active["active"] is True
    assert [s["type"] for s in active_summary["active_bullish_events"]] == ["REVERSAL_CANDIDATE"]


def test_previous_resolved_state_round_trips_without_reactivation():
    previous = [_carried_breakdown(
        state=LIFECYCLE_RESOLVED,
        active=False,
        resolved_by="INTRADAY_RECLAIM",
        latest_event_type="INTRADAY_RECLAIM",
        age_bars=0,
    )]

    summary = build_event_state_summary([], previous_states=previous)

    breakdown = next(s for s in summary["states"] if s["event_family"] == "SUPPORT_BREAKDOWN")
    assert breakdown["state"] == LIFECYCLE_RESOLVED
    assert breakdown["active"] is False
    assert breakdown["resolved_by"] == "INTRADAY_RECLAIM"
    assert breakdown["age_bars"] == 1
    assert [s["type"] for s in summary["resolved"]] == ["HIGH_VOLUME_BREAKDOWN"]
    assert summary["active_bearish_events"] == []
    assert summary["market_state"] == "NORMAL"


def test_previous_resolved_state_expires_after_threshold():
    previous = [_carried_breakdown(
        state=LIFECYCLE_RESOLVED,
        active=False,
        resolved_by="INTRADAY_RECLAIM",
        latest_event_type="INTRADAY_RECLAIM",
        age_bars=1,
    )]

    summary = build_event_state_summary([], previous_states=previous)

    breakdown = next(s for s in summary["states"] if s["event_family"] == "SUPPORT_BREAKDOWN")
    assert breakdown["state"] == LIFECYCLE_EXPIRED
    assert breakdown["active"] is False
    assert "EVENT_EXPIRED_STALE" in breakdown["reason_codes"]
    assert [s["type"] for s in summary["expired"]] == ["HIGH_VOLUME_BREAKDOWN"]
    assert summary["resolved"] == []


def test_previous_expired_state_round_trips_without_reactivation():
    previous = [_carried_breakdown(
        state=LIFECYCLE_EXPIRED,
        active=False,
        age_bars=2,
        reason_codes=["SUPPORT_CLOSED_BELOW", "EVENT_EXPIRED_STALE"],
    )]

    summary = build_event_state_summary([], previous_states=previous)

    breakdown = next(s for s in summary["states"] if s["event_family"] == "SUPPORT_BREAKDOWN")
    assert breakdown["state"] == LIFECYCLE_EXPIRED
    assert breakdown["active"] is False
    assert breakdown["age_bars"] == 3
    assert [s["type"] for s in summary["expired"]] == ["HIGH_VOLUME_BREAKDOWN"]
    assert summary["active_bearish_events"] == []


def test_fresh_detection_preserves_root_event_type():
    """root_event_type 必須留住鏈的**起點**，不能被當根新偵測蓋成新事件的 type。

    這是 todo.md T-045 P2 修的 bug：merge 迴圈 `states[key] = state` 是整筆覆寫，
    先前把 root 設成新偵測的 type，欄位名叫 root 卻永遠等於 latest，
    事件鏈的起點因此無法還原。

    **這個情境目前在正式資料裡不會發生，是刻意構造的**：EVENT_TYPE_META 現在的四種
    事件類型各自對應一個獨立 family（一個 family 只有一種 type），所以同一個
    (zone_key, event_family) 的 root 與新偵測的 type 永遠相同，覆寫不會遺失資訊。
    `INTRABAR_BREAKDOWN` 是本測試虛構的型別名，repo 裡並不存在
    （真實的「intrabar 跌破」是 HIGH_VOLUME_BREAKDOWN 的 CANDIDATE 狀態）。

    保留這支測試的理由是**擋住未來**：哪一天 SUPPORT_BREAKDOWN 家族多出第二種 type，
    覆寫就會開始默默吃掉鏈的起點，而那時不會有任何東西報錯。
    """
    previous = [_carried_breakdown(
        root_event_type="INTRABAR_BREAKDOWN",
        latest_event_type="INTRABAR_BREAKDOWN",
        type="INTRABAR_BREAKDOWN",
    )]
    events = normalize_market_events([{
        "type": "HIGH_VOLUME_BREAKDOWN",
        "direction": "BEARISH",
        "lifecycle_state": LIFECYCLE_CONFIRMED,
        "active": True,
        "zone_ref": {"role": "SUPPORT", "price_low": 98.0, "price_high": 100.0},
    }])

    summary = build_event_state_summary(events, previous_states=previous)

    state = next(s for s in summary["states"] if s["event_family"] == "SUPPORT_BREAKDOWN")
    assert state["root_event_type"] == "INTRABAR_BREAKDOWN", "鏈的起點被新偵測蓋掉了"
    assert state["latest_event_type"] == "HIGH_VOLUME_BREAKDOWN", "latest 應反映最新事件"


def test_detection_after_terminal_state_starts_new_root():
    """已終結（EXPIRED／RESOLVED）之後再出現同家族事件，是**新的一條鏈**，root 應為新事件。

    規則與 Go 端摺疊 timeline 的 chain 邊界刻意對稱
    （internal/analysis/event_timeline.go 的 isClosedEventState）。
    """
    for terminal in (LIFECYCLE_EXPIRED, LIFECYCLE_RESOLVED):
        previous = [_carried_breakdown(
            root_event_type="INTRABAR_BREAKDOWN",
            latest_event_type="INTRABAR_BREAKDOWN",
            type="INTRABAR_BREAKDOWN",
            state=terminal,
            active=False,
        )]
        events = normalize_market_events([{
            "type": "HIGH_VOLUME_BREAKDOWN",
            "direction": "BEARISH",
            "lifecycle_state": LIFECYCLE_CONFIRMED,
            "active": True,
            "zone_ref": {"role": "SUPPORT", "price_low": 98.0, "price_high": 100.0},
        }])

        summary = build_event_state_summary(events, previous_states=previous)

        state = next(s for s in summary["states"] if s["event_family"] == "SUPPORT_BREAKDOWN")
        assert state["root_event_type"] == "HIGH_VOLUME_BREAKDOWN", (
            f"前一狀態為 {terminal} 時應開新鏈，root 該是新事件而不是沿用舊的"
        )


# ── zone 與事件的關聯鍵（T-048 階段 C）──

def test_zone_identity_key_matches_the_key_events_carry():
    """序列化輸出的 zone_key 必須與事件身上的 zone_key **逐字元相同**。

    Go 端靠這個鍵把事件掛回 zone 的穩定身分。兩邊只要有一邊改了格式化方式，
    關聯就會靜默失敗——事件掛不到 zone，看起來像「這次沒有 zone 事件」。
    """
    from ..serialization import _zone_score_to_dict

    z = _zone(low=98.0, high=100.0)
    events = normalize_market_events([
        {"type": "SUPPORT_BREAKDOWN", "zone_ref": {
            "role": z.role, "price_low": z.price_low, "price_high": z.price_high,
        }},
    ])

    assert _zone_score_to_dict(z)["zone_key"] == events[0]["zone_key"]
    assert zone_identity_key(z) == events[0]["zone_key"]


def test_zone_identity_key_ignores_fields_that_are_not_identity():
    # 鍵只由 role 與邊界構成。帶進分數這類每次分析都會變的欄位，會讓同一個 zone
    # 每次拿到不同的鍵——那正是 zone_key 當身分時的原始毛病。
    a = _zone(low=98.0, high=100.0, confidence=0.72, trading_score=78.0)
    b = _zone(low=98.0, high=100.0, confidence=0.31, trading_score=12.0)

    assert zone_identity_key(a) == zone_identity_key(b)


def test_every_state_carries_the_carried_flag():
    """`carried_from_previous` 在**每一筆** state 上都要有值。

    Go 端用它決定「找不到活鏈時要不要開新 occurrence」（T-048 階段 C 的 F2 護欄），
    並把讀不到的筆數計成 carried_parse_failed。只在 carry forward 那條寫 True 的話，
    每一筆新偵測都會被算成解析失敗，那個計數就沒有訊號了。
    """
    first = build_event_state_summary([
        {"type": "SUPPORT_BREAKDOWN", "zone_ref": {"role": "SUPPORT", "price_low": 98.0, "price_high": 100.0}},
    ])
    for state in first["states"]:
        assert state["carried_from_previous"] is False, "這一輪偵測到的事件不是重報"

    # 下一輪沒有任何新偵測，前一輪的狀態被抄過來。
    second = build_event_state_summary([], previous_states=first["states"])
    for state in second["states"]:
        assert state["carried_from_previous"] is True, "沒有新偵測時抄過來的狀態是重報"


# ── 階段 D：SUPPORT_RETEST_HELD / RESISTANCE_BREAKOUT 與只寫不讀的隔離 ──────────


def test_resistance_breakout_confirmed_when_closed_above_with_volume():
    z = _zone(role=ZoneType.RESISTANCE.value, low=98.0, high=100.0, relative_volume=2.0)

    events = detect_market_events([z], current_price=101.0, candle_high=101.5, candle_low=99.0, candle_close=101.0)

    assert _types(events) == {"RESISTANCE_BREAKOUT"}
    breakout = events[0]
    assert breakout["state"] == LIFECYCLE_CONFIRMED
    assert breakout["active"] is True
    assert breakout["direction"] == "BULLISH"
    assert breakout["reason_codes"] == ["RESISTANCE_CLOSED_ABOVE"]
    assert breakout["decision_visible"] is False


def test_resistance_breakout_is_candidate_when_only_intrabar():
    z = _zone(role=ZoneType.RESISTANCE.value, low=98.0, high=100.0, relative_volume=2.0)

    events = detect_market_events([z], current_price=99.0, candle_high=101.0, candle_low=98.5, candle_close=99.0)

    breakout = next(e for e in events if e["type"] == "RESISTANCE_BREAKOUT")
    assert breakout["state"] == LIFECYCLE_CANDIDATE
    assert breakout["active"] is False
    assert breakout["reason_codes"] == ["RESISTANCE_INTRABAR_BREAK"]


def test_resistance_breakout_requires_volume_confirmation():
    # 量能未達門檻且 volume_confirmation 沒有失敗時不成立，與跌破事件同一組門檻。
    z = _zone(role=ZoneType.RESISTANCE.value, low=98.0, high=100.0, relative_volume=1.0)

    events = detect_market_events([z], current_price=101.0, candle_high=101.5, candle_low=99.0, candle_close=101.0)

    assert "RESISTANCE_BREAKOUT" not in _types(events)


def test_support_retest_held_is_a_fact_without_quality_gate():
    # 與 REVERSAL_CANDIDATE 的分工：後者要 EV／信心／驗證都合格，這個只要「碰到且未收破」。
    z = _zone(low=98.0, high=100.0, relative_volume=1.0, recent_validation=RecentValidation.EXPIRED.value)

    events = detect_market_events([z], current_price=99.0, candle_high=99.5, candle_low=97.0, candle_close=99.0)

    assert "REVERSAL_CANDIDATE" not in _types(events)
    retest = next(e for e in events if e["type"] == "SUPPORT_RETEST_HELD")
    assert retest["event_family"] == "SUPPORT_RETEST"
    assert retest["decision_visible"] is False


def test_support_retest_held_not_emitted_when_closed_below():
    z = _zone(low=98.0, high=100.0, relative_volume=2.0)

    events = detect_market_events([z], current_price=97.0, candle_high=99.0, candle_low=96.0, candle_close=97.0)

    assert "SUPPORT_RETEST_HELD" not in _types(events)


def test_stage_d_events_never_enter_decision_buckets():
    # 這條是階段 D 的核心：新事件要寫得進 states（持久化），但不能出現在任何決策桶。
    # 特別是 active_bullish_events——它只看 direction，少了過濾就會改決策。
    z = _zone(low=98.0, high=100.0, relative_volume=1.0)

    events = detect_market_events([z], current_price=99.0, candle_high=99.5, candle_low=97.0, candle_close=99.0)
    summary = build_event_state_summary(events)

    assert "SUPPORT_RETEST_HELD" in {state["type"] for state in summary["states"]}
    for bucket in ("active", "candidates", "confirmed", "resolved", "expired",
                   "active_bearish_events", "active_bullish_events"):
        assert "SUPPORT_RETEST_HELD" not in {state["type"] for state in summary[bucket]}
    assert summary["market_state"] == "NORMAL"
    assert summary["latest_event_type"] == "REVERSAL_CANDIDATE"


def test_truncation_drops_stage_d_events_before_decision_visible_ones():
    # 舊寫法是 events[:8]（插入序），5 個 zone 各產一組 (REVERSAL_CANDIDATE,
    # SUPPORT_RETEST_HELD) 時會把第 5 個 zone 的 REVERSAL_CANDIDATE 擠掉——那是決策可見的改變。
    zones = [_zone(low=98.0 + i * 0.1, high=99.0 + i * 0.1, relative_volume=1.0) for i in range(5)]
    price = 99.0

    events = detect_market_events(
        [zones[0]], current_price=price, candle_high=99.5, candle_low=97.0, candle_close=price
    )
    assert len(events) == 2  # 前提：單一 zone 會產出一組兩個事件

    events = detect_market_events(
        zones,
        current_price=price,
        candle_high=99.5,
        candle_low=97.0,
        candle_close=price,
    )

    types = [event["type"] for event in events]
    assert len(types) == 8
    assert types.count("REVERSAL_CANDIDATE") == 5
    assert types.count("SUPPORT_RETEST_HELD") == 3


def test_carried_state_without_flag_defaults_to_decision_visible():
    # 階段 D 之前寫進 market_event_states 的列沒有這個鍵，缺鍵必須當成「可見」，
    # 否則既有事件會整批從決策桶消失。
    previous = [{
        "type": "HIGH_VOLUME_BREAKDOWN",
        "event_family": "SUPPORT_BREAKDOWN",
        "zone_key": "SUPPORT:98.0000:100.0000",
        "direction": "BEARISH",
        "state": LIFECYCLE_CONFIRMED,
        "active": True,
    }]

    summary = build_event_state_summary([], previous_states=previous)

    assert [state["type"] for state in summary["active_bearish_events"]] == ["HIGH_VOLUME_BREAKDOWN"]
    assert summary["states"][0]["decision_visible"] is True


def test_stage_d_events_are_excluded_from_event_sequence_projection():
    # event_sequence 會寫進 stock_sr_decisions.event_sequence_json（決策表既有欄位），
    # 所以它跟決策桶一樣不能看到階段 D 的事件。
    from ..decision_engine import _event_sequence

    z = _zone(low=98.0, high=100.0, relative_volume=1.0)
    events = detect_market_events([z], current_price=99.0, candle_high=99.5, candle_low=97.0, candle_close=99.0)

    assert "SUPPORT_RETEST_HELD" in _types(events)
    assert [item["type"] for item in _event_sequence(events)] == ["REVERSAL_CANDIDATE"]


# ── 老化的單位是 K 棒推進，不是分析次數（issue.md I-077）──


def _aging_previous(age_bars: int, expires_after: int = 2) -> list[dict]:
    return [{
        "type": "HIGH_VOLUME_BREAKDOWN",
        "event_family": "SUPPORT_BREAKDOWN",
        "event_scope": "ZONE",
        "zone_key": "SUPPORT:98.0000:100.0000",
        "root_event_type": "HIGH_VOLUME_BREAKDOWN",
        "latest_event_type": "HIGH_VOLUME_BREAKDOWN",
        "direction": "BEARISH",
        "state": LIFECYCLE_CONFIRMED,
        "active": True,
        "age_bars": age_bars,
        "expires_after_bars": expires_after,
        "reason_codes": ["SUPPORT_CLOSED_BELOW"],
    }]


def test_same_bar_reanalysis_does_not_age_events():
    """同一根 K 棒重打分析不能讓事件老化。

    這是 I-077 的直接回歸：修法前每 carry 一次就 +1，於是「candles 一根都沒變、只是把
    同一階再打一次」就足以把 CONFIRMED 推到 EXPIRED，而 market_state_from_event_states
    只看 active——那會實際改變 Market State。
    """
    previous = _aging_previous(age_bars=1, expires_after=2)

    summary = build_event_state_summary([], previous_states=previous, bar_advanced=False)

    state = summary["states"][0]
    assert state["age_bars"] == 1, "K 棒沒推進就不該累加"
    assert state["state"] == LIFECYCLE_CONFIRMED
    assert state["active"] is True
    assert summary["active_bearish_events"] != [], "不該因為重打而退出 gating"


def test_bar_advance_still_ages_to_expired():
    """K 棒真的推進時，老化行為與修改前逐項相同。"""
    previous = _aging_previous(age_bars=1, expires_after=2)

    summary = build_event_state_summary([], previous_states=previous, bar_advanced=True)

    state = summary["states"][0]
    assert state["age_bars"] == 2
    assert state["state"] == LIFECYCLE_EXPIRED
    assert state["active"] is False
    assert "EVENT_EXPIRED_STALE" in state["reason_codes"]
    assert summary["active_bearish_events"] == []


def test_bar_advanced_defaults_to_true():
    """**缺值必須等於舊行為。** 沒帶這個參數的呼叫端（evaluation / replay / 既有測試）
    行為不能改變，否則這筆修改的影響面就不只是「同一根 K 棒重打」。"""
    previous = _aging_previous(age_bars=0, expires_after=3)

    default = build_event_state_summary([], previous_states=previous)
    explicit = build_event_state_summary([], previous_states=previous, bar_advanced=True)

    assert default["states"][0]["age_bars"] == explicit["states"][0]["age_bars"] == 1


# ── _bar_advanced_since 的邊界（issue.md I-077）──


def test_bar_advanced_since_boundaries():
    """**每一種「比不出來」都要退回 True＝舊行為。**

    這個函式的失敗模式是靜默的：回錯 False 會讓事件永不老化、回錯 True 會讓同日重打
    照樣老化，兩者都沒有任何東西會報錯，所以邊界逐條釘住。
    """
    import pandas as pd

    from ..decision_engine import _bar_advanced_since

    now = pd.Timestamp("2026-08-18T16:00:00Z")

    # 推進：上一次站在更早的 K 棒
    assert _bar_advanced_since(now, "2026-08-15T16:00:00Z") is True
    # 同一根 K 棒：這就是 I-077 的情境
    assert _bar_advanced_since(now, "2026-08-18T16:00:00Z") is False
    # 時間倒退（as-of 回放、資料修正）：保守側，不老化
    assert _bar_advanced_since(now, "2026-08-19T16:00:00Z") is False
    # 缺值／無法解析／沒有 analyzed_at → 一律維持舊行為
    assert _bar_advanced_since(now, None) is True
    assert _bar_advanced_since(now, "") is True
    assert _bar_advanced_since(now, "not-a-timestamp") is True
    assert _bar_advanced_since(None, "2026-08-15T16:00:00Z") is True
    # 帶時區偏移的 RFC3339（Go 不一定送 Z）
    assert _bar_advanced_since(now, "2026-08-16T00:00:00+08:00") is True
