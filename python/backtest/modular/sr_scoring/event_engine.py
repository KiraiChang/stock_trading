"""Market event detection for SR Zone decision context."""
from __future__ import annotations

from typing import Any, Optional

from .formatting import fmt_price as _fmt_price
from .types import RecentValidation, VolumeConfirmation, ZoneScore, ZoneType


EXTREME_VOLUME_THRESHOLD = 2.5
HIGH_VOLUME_BREAKDOWN_THRESHOLD = 1.5

LIFECYCLE_CANDIDATE = "CANDIDATE"
LIFECYCLE_CONFIRMED = "CONFIRMED"
LIFECYCLE_ACTIVE = "ACTIVE"
LIFECYCLE_RESOLVED = "RESOLVED"
LIFECYCLE_EXPIRED = "EXPIRED"

# carried active 事件若未被 resolve，最多存活這麼多根 K 棒（analysis）即轉 EXPIRED。
# 事件自帶 expires_after_bars 時以事件值為準；沒帶（如 EXTREME_VOLUME context）套此
# 預設，確保沒有任何 carried 事件會無限期停留在 active。
DEFAULT_EVENT_EXPIRES_AFTER_BARS = 3

EVENT_FAMILY_LIFECYCLE_RULES = {
    "VOLUME_CONTEXT": {
        "gating_states": (LIFECYCLE_ACTIVE,),
        "expires_after_bars": 1,
    },
    "SUPPORT_BREAKDOWN": {
        "gating_states": (LIFECYCLE_CONFIRMED, LIFECYCLE_ACTIVE),
        "expires_after_bars": 2,
    },
    "SUPPORT_RECLAIM": {
        "gating_states": (LIFECYCLE_CONFIRMED, LIFECYCLE_ACTIVE),
        "expires_after_bars": 2,
    },
    "SUPPORT_REVERSAL": {
        "gating_states": (LIFECYCLE_ACTIVE,),
        "expires_after_bars": 2,
    },
    # ── 階段 D 新增的兩個 family（decision_visible=False，見下方 EVENT_TYPE_META） ──
    "SUPPORT_RETEST": {
        "gating_states": (LIFECYCLE_CONFIRMED, LIFECYCLE_ACTIVE),
        "expires_after_bars": 2,
    },
    "RESISTANCE_BREAKOUT": {
        "gating_states": (LIFECYCLE_CONFIRMED, LIFECYCLE_ACTIVE),
        "expires_after_bars": 2,
    },
}

# 一次分析最多輸出幾個事件。**改這個值等於改決策**（截斷點一移，決策看得到的事件集合
# 就變了），所以階段 D 沿用原本的 8，只改「先留誰」的順序，見 _truncate_events。
MAX_EVENTS_PER_ANALYSIS = 8

# decision_visible 缺鍵時的預設。既有四個型別都是決策可見的，而 042 之前寫進
# market_event_states 的列不會有這個鍵——**缺鍵有合理預設，與 carried_from_previous
# 「缺鍵是異常」的處理刻意不同**。
DEFAULT_EVENT_DECISION_VISIBLE = True

# `decision_visible` 是階段 D 的隔離旗標。**它必須是顯式的**：決策端對事件的消費有兩類，
# 「名字型」（market_state_from_event_states / lifecycle_engine.resolve_event_signal /
# decision_engine 的 event_types）逐一比對既有型別名，新名字自然落到 NORMAL；但
# 「方向型」的 active_bearish_events / active_bullish_events **只看 direction**，
# 而 resolve_lifecycle 對前者取 truthiness 就直接判 lifecycle_phase=BREAKDOWN。
# 也就是說，一個 active 且帶方向的新事件**不必被任何人認識就會改決策**。
#
# 所以隔離不能靠「決策端不認識新名字」，也不靠把 direction 設成 NEUTRAL（那是副作用，
# 下一個加事件的人不會知道要維持它，而且沒有東西會報錯）。桶構建改成顯式跳過
# decision_visible=False——Python 在 build_event_state_summary，Go 在
# sr_zones.go 的 eventStateSummaryJSON（carry-forward 的回程走 Go 那份）。
EVENT_TYPE_META = {
    "EXTREME_VOLUME": {
        "family": "VOLUME_CONTEXT",
        "direction": "NEUTRAL",
        "default_state": LIFECYCLE_ACTIVE,
        "resolves": (),
        "decision_visible": True,
    },
    "HIGH_VOLUME_BREAKDOWN": {
        "family": "SUPPORT_BREAKDOWN",
        "direction": "BEARISH",
        "default_state": LIFECYCLE_CANDIDATE,
        "resolves": (),
        "decision_visible": True,
    },
    "INTRADAY_RECLAIM": {
        "family": "SUPPORT_RECLAIM",
        "direction": "BULLISH",
        "default_state": LIFECYCLE_CONFIRMED,
        "resolves": ("SUPPORT_BREAKDOWN",),
        "decision_visible": True,
    },
    "REVERSAL_CANDIDATE": {
        "family": "SUPPORT_REVERSAL",
        "direction": "BULLISH",
        "default_state": LIFECYCLE_CANDIDATE,
        "resolves": ("SUPPORT_BREAKDOWN",),
        "decision_visible": True,
    },
    # ── 階段 D 新增：只寫不讀 ──────────────────────────────────────────
    # resolves 兩個都刻意留空：resolve 會把既有 family 改成 RESOLVED / active=False，
    # 那是決策可見的改變。「壓力突破是否 resolve 支撐側事件」的語意留給 T-049。
    "SUPPORT_RETEST_HELD": {
        "family": "SUPPORT_RETEST",
        "direction": "BULLISH",
        "default_state": LIFECYCLE_CONFIRMED,
        "resolves": (),
        "decision_visible": False,
    },
    "RESISTANCE_BREAKOUT": {
        "family": "RESISTANCE_BREAKOUT",
        "direction": "BULLISH",
        "default_state": LIFECYCLE_CANDIDATE,
        "resolves": (),
        "decision_visible": False,
    },
}

EVENT_ORDER = {
    "EXTREME_VOLUME": 10,
    "HIGH_VOLUME_BREAKDOWN": 20,
    "INTRADAY_RECLAIM": 30,
    "REVERSAL_CANDIDATE": 40,
    # 新事件排在既有之後：EVENT_ORDER 決定 build_event_state_summary 的合併順序與
    # latest_event_type，排前面會讓既有事件的相對順序改變。
    "SUPPORT_RETEST_HELD": 50,
    "RESISTANCE_BREAKOUT": 60,
}


def _distance_pct_to_zone(z: ZoneScore, current_price: float) -> float:
    if z.price_low <= current_price <= z.price_high:
        return 0.0
    if current_price < z.price_low:
        return (z.price_low - current_price) / current_price
    return (current_price - z.price_high) / current_price


def _clamp_relevance(value: float) -> float:
    return float(max(0.0, min(100.0, value)))


def entry_relevance_base_breakdown(z: ZoneScore, current_price: float) -> dict[str, float]:
    """base entry relevance 的逐項拆解：優先沿用 scoring 算好的 breakdown，只有沒有
    breakdown 的合成 zone（例如測試）才用這裡的簡化 fallback。event_engine 與
    decision_engine 共用同一份定義，避免同一 zone 在不同模組算出不同 base relevance。"""
    base = dict(z.entry_relevance_breakdown or {})
    if not base:
        distance_pct = _distance_pct_to_zone(z, current_price)
        base = {
            "distance": max(0.0, 1.0 - min(distance_pct / 0.08, 1.0)) * 30.0,
            "ev_rr": (
                (max(0.0, min(((z.expected_value or 0.0) + 0.02) / 0.07, 1.0)) * 15.0)
                + (min((z.risk_reward_ratio or 0.0) / 2.5, 1.0) * 15.0)
            ),
            "validation": 0.0 if z.recent_validation == RecentValidation.EXPIRED.value else 12.0,
            "volume": 5.0,
            "role_readiness": 0.0 if z.role == ZoneType.AT_ZONE.value else 10.0,
            "confidence": z.confidence * 10.0,
        }
    return base


def entry_relevance_base_score(z: ZoneScore, current_price: float) -> float:
    return _clamp_relevance(sum(entry_relevance_base_breakdown(z, current_price).values()))


def zone_interaction(
    z: ZoneScore,
    current_price: float,
    candle_high: Optional[float] = None,
    candle_low: Optional[float] = None,
    candle_close: Optional[float] = None,
) -> dict[str, Any]:
    high = current_price if candle_high is None else candle_high
    low = current_price if candle_low is None else candle_low
    close = current_price if candle_close is None else candle_close
    touched = low <= z.price_high and high >= z.price_low
    closed_inside = z.price_low <= close <= z.price_high
    closed_above = close > z.price_high
    closed_below = close < z.price_low
    penetration_pct = 0.0
    if low < z.price_low:
        penetration_pct = max(penetration_pct, (z.price_low - low) / z.price_low)
    if high > z.price_high:
        penetration_pct = max(penetration_pct, (high - z.price_high) / z.price_high)

    if not touched:
        state_label = "尚未測試"
    elif closed_inside:
        state_label = "進入區間"
    elif z.role == ZoneType.SUPPORT.value and closed_below:
        state_label = "有效跌破"
    elif z.role == ZoneType.SUPPORT.value and closed_above:
        state_label = "收回區間上方"
    else:
        state_label = "今日已測試"

    distance_pct = _distance_pct_to_zone(z, close)
    if closed_above:
        close_relative_to_zone = "ABOVE_ZONE"
    elif closed_below:
        close_relative_to_zone = "BELOW_ZONE"
    else:
        close_relative_to_zone = "INSIDE_ZONE"
    reclaim_type = "NONE"
    rejection_type = "NONE"
    if z.role == ZoneType.SUPPORT.value and touched and closed_above and penetration_pct > 0:
        reclaim_type = "UNDERCUT_RECLAIM"
    elif z.role == ZoneType.RESISTANCE.value and touched and closed_below and penetration_pct > 0:
        reclaim_type = "OVERTHROW_REJECTED"
    elif z.role == ZoneType.SUPPORT.value and touched and not closed_below:
        rejection_type = "SUPPORT_HELD"
    elif z.role == ZoneType.RESISTANCE.value and touched and not closed_above:
        rejection_type = "RESISTANCE_HELD"
    evidence = {
        "reclaim_type": reclaim_type,
        "rejection_type": rejection_type,
        "penetration_ratio": penetration_pct,
        "close_relative_to_zone": close_relative_to_zone,
        "follow_through": "UNKNOWN",
        "touched": touched,
        "closed_above": closed_above,
        "closed_below": closed_below,
    }
    return {
        "distance_pct": distance_pct,
        "distance_label": f"{distance_pct * 100:.1f}%",
        "touched": touched,
        "penetration_pct": penetration_pct,
        "closed_inside": closed_inside,
        "closed_above": closed_above,
        "closed_below": closed_below,
        "state_label": state_label,
        "price_action_evidence": evidence,
    }


def event_zone_ref(z: ZoneScore, current_price: float) -> dict[str, Any]:
    return {
        "price_low": z.price_low,
        "price_high": z.price_high,
        "label": f"{_fmt_price(z.price_low)} ~ {_fmt_price(z.price_high)}",
        "role": z.role,
        "tier": z.tier,
        "tier_label": z.tier_label,
        "distance_pct": _distance_pct_to_zone(z, current_price),
        "entry_relevance_score": entry_relevance_base_score(z, current_price),
    }


def _zone_key(zone_ref: Optional[dict[str, Any]]) -> str:
    if not zone_ref:
        return "SYMBOL"
    role = zone_ref.get("role") or "UNKNOWN"
    return f"{role}:{float(zone_ref.get('price_low', 0.0)):.4f}:{float(zone_ref.get('price_high', 0.0)):.4f}"


def zone_identity_key(z: ZoneScore) -> str:
    """zone 與它身上事件之間的關聯鍵。**這是該鍵唯一的產生點。**

    Go 端要把事件掛到 `zone_instances.zone_uid` 上，需要一個能把「這次分析的 zone」
    與「這次分析偵測到的事件」對起來的鍵。讓 Go 自己用 `%.4f` 重建一份，等於做出
    `_zone_key()` 的平行實作——兩份浮點格式化哪天分歧，關聯會**靜默**失敗：事件掛不到
    zone，資料看起來就像「這次沒有 zone 事件」，沒有任何東西會報錯。

    所以 zone 的序列化直接輸出這個鍵（見 serialization.py），Go 只做字串比對。
    """
    return _zone_key({
        "role": z.role,
        "price_low": z.price_low,
        "price_high": z.price_high,
    })


def _lifecycle_rule(event_family: str) -> dict[str, Any]:
    return EVENT_FAMILY_LIFECYCLE_RULES.get(event_family, {
        "gating_states": (LIFECYCLE_CONFIRMED, LIFECYCLE_ACTIVE),
        "expires_after_bars": DEFAULT_EVENT_EXPIRES_AFTER_BARS,
    })


def _event_expires_after_bars(event_type: str, event_family: str, explicit_value: Any = None) -> int:
    if explicit_value is not None:
        return int(explicit_value)
    meta = EVENT_TYPE_META.get(event_type, {})
    family = str(meta.get("family") or event_family)
    return int(_lifecycle_rule(family).get("expires_after_bars") or DEFAULT_EVENT_EXPIRES_AFTER_BARS)


def _state_allows_gating(event_family: str, state_name: str) -> bool:
    return state_name in set(_lifecycle_rule(event_family).get("gating_states") or ())


def _event_decision_visible(event_type: str, explicit_value: Any = None) -> bool:
    """這個事件型別能不能被決策看到。**這是階段 D 隔離的唯一判準。**"""
    if explicit_value is not None:
        return bool(explicit_value)
    meta = EVENT_TYPE_META.get(event_type, {})
    return bool(meta.get("decision_visible", DEFAULT_EVENT_DECISION_VISIBLE))


def is_decision_visible(event: dict[str, Any]) -> bool:
    """這一筆事件（或事件狀態）能不能被決策看到。**跨模組的唯一判準。**

    決策端有三個地方要用同一個答案：`build_event_state_summary` 的桶構建、
    `_truncate_events` 的截斷優先序、以及 `decision_engine._event_sequence`
    （它會寫進 `stock_sr_decisions.event_sequence_json`，是決策表的既有欄位）。
    """
    return _event_decision_visible(str(event.get("type") or ""), event.get("decision_visible"))


def _truncate_events(events: list[dict[str, Any]]) -> list[dict[str, Any]]:
    """截到 MAX_EVENTS_PER_ANALYSIS，**決策可見的優先留下**。

    原本是 `events[:8]`，也就是照**插入序**截斷——而排序（EVENT_ORDER）發生在截斷之後
    （build_event_state_summary 才排）。支撐與壓力兩側都會產事件之後，同一個上限要塞
    更多事件，插入序截斷會**靜默擠掉**後面 zone 的支撐事件，那是決策可見的改變。

    這裡先依「decision_visible 優先、其次插入序」挑出要留的，再**還原成插入序**輸出：
    當事件全部是決策可見時（新事件出現前的所有情形），結果與 `events[:8]` 逐項相同。
    """
    ranked = sorted(
        enumerate(events),
        key=lambda pair: (0 if is_decision_visible(pair[1]) else 1, pair[0]),
    )
    kept = sorted(index for index, _ in ranked[:MAX_EVENTS_PER_ANALYSIS])
    return [events[index] for index in kept]


def normalize_market_event(event: dict[str, Any]) -> dict[str, Any]:
    event_type = str(event.get("type") or "UNKNOWN")
    meta = EVENT_TYPE_META.get(event_type, {})
    zone_ref = event.get("zone_ref")
    event_scope = "SYMBOL" if zone_ref is None else "ZONE"
    event_family = str(meta.get("family") or event_type)
    lifecycle_state = str(event.get("lifecycle_state") or meta.get("default_state") or LIFECYCLE_CANDIDATE)
    normalized = dict(event)
    normalized.setdefault("event_family", event_family)
    normalized.setdefault("event_scope", event_scope)
    normalized.setdefault("event_key", f"{event_scope}:{event_family}:{_zone_key(zone_ref)}")
    normalized.setdefault("zone_key", _zone_key(zone_ref))
    normalized.setdefault("state", lifecycle_state)
    normalized.setdefault("lifecycle_state", normalized["state"])
    normalized.setdefault("expires_after_bars", _event_expires_after_bars(event_type, event_family, event.get("expires_after_bars")))
    normalized.setdefault("confirmation_state", "CONFIRMED" if normalized["state"] in (LIFECYCLE_CONFIRMED, LIFECYCLE_ACTIVE) else "PENDING")
    normalized["decision_visible"] = _event_decision_visible(event_type, event.get("decision_visible"))
    normalized["active"] = bool(normalized.get("active")) and _state_allows_gating(event_family, str(normalized["state"]))
    normalized.setdefault("reason_codes", [event_type])
    return normalized


def normalize_market_events(events: list[dict[str, Any]]) -> list[dict[str, Any]]:
    return [normalize_market_event(event) for event in events]


def _normalize_previous_event_state(state: dict[str, Any]) -> dict[str, Any]:
    event_type = str(state.get("type") or state.get("event_type") or state.get("latest_event_type") or "UNKNOWN")
    meta = EVENT_TYPE_META.get(event_type, {})
    event_family = str(state.get("event_family") or meta.get("family") or event_type)
    zone_key = str(state.get("zone_key") or _zone_key(state.get("zone_ref")))
    state_name = str(state.get("state") or state.get("lifecycle_state") or LIFECYCLE_ACTIVE)
    active = bool(state.get("active")) and _state_allows_gating(event_family, state_name)

    # 每被 carry 一次代表多存活一根 K 棒（analysis）；被當根新偵測覆蓋時，會在 merge
    # 迴圈以 age_bars=0 的新 state 取代（等於重置存活計數）。未被 resolve 的 carried
    # active 事件老化到 expires_after_bars 門檻即轉 EXPIRED，完成 …→Resolved→Expired
    # 生命週期，避免事件無限期停留在 active。
    age_bars = int(state.get("age_bars") or 0) + 1
    raw_expires = state.get("expires_after_bars")
    expires_after = _event_expires_after_bars(event_type, event_family, raw_expires)
    expired = state_name != LIFECYCLE_EXPIRED and age_bars >= expires_after

    normalized = dict(state)
    normalized.setdefault("event_key", f"{state.get('event_scope') or 'ZONE'}:{event_family}:{zone_key}")
    normalized.setdefault("type", event_type)
    normalized.setdefault("event_family", event_family)
    normalized.setdefault("event_scope", state.get("event_scope") or ("SYMBOL" if zone_key == "SYMBOL" else "ZONE"))
    normalized.setdefault("zone_key", zone_key)
    normalized.setdefault("root_event_type", state.get("root_event_type") or event_type)
    normalized.setdefault("latest_event_type", state.get("latest_event_type") or event_type)
    normalized.setdefault("direction", state.get("direction") or meta.get("direction"))
    normalized["age_bars"] = age_bars
    normalized["expires_after_bars"] = expires_after
    reason_codes = list(state.get("reason_codes") or [event_type])
    if expired:
        normalized["state"] = LIFECYCLE_EXPIRED
        normalized["active"] = False
        normalized["confirmation_state"] = "EXPIRED_STALE"
        normalized["reason_codes"] = [*reason_codes, "EVENT_EXPIRED_STALE"]
    else:
        normalized["state"] = state_name
        normalized["active"] = active
        normalized.setdefault(
            "confirmation_state",
            state.get("confirmation_state")
            or ("RESOLVED" if state_name == LIFECYCLE_RESOLVED else ("CONFIRMED" if active else "PENDING")),
        )
        normalized.setdefault("reason_codes", reason_codes)
    normalized["decision_visible"] = _event_decision_visible(event_type, state.get("decision_visible"))
    normalized["carried_from_previous"] = True
    normalized.setdefault("resolved_by", state.get("resolved_by"))
    return normalized


def build_event_state_summary(
    events: list[dict[str, Any]],
    previous_states: Optional[list[dict[str, Any]]] = None,
) -> dict[str, Any]:
    """Build an in-memory event lifecycle summary from latest detected events.

    P2 stays schema-light: Go persists normalized states per analysis and passes
    the latest active states back as previous_states; this pure function merges
    them with latest-candle detections and emits a deterministic lifecycle view.
    """
    normalized = sorted(normalize_market_events(events), key=lambda e: EVENT_ORDER.get(str(e.get("type")), 999))
    states: dict[tuple[str, str], dict[str, Any]] = {}

    for previous in previous_states or []:
        event = _normalize_previous_event_state(previous)
        key = (str(event.get("zone_key") or "SYMBOL"), str(event.get("event_family") or event.get("type")))
        states[key] = event

    for event in normalized:
        zone_key = str(event.get("zone_key") or "SYMBOL")
        event_family = str(event.get("event_family") or event.get("type"))
        event_type = str(event.get("type") or "UNKNOWN")
        key = (zone_key, event_family)
        state_name = str(event.get("state") or event.get("lifecycle_state") or LIFECYCLE_CANDIDATE)
        is_active = bool(event.get("active")) and _state_allows_gating(event_family, state_name)

        # **root_event_type 要延續，不能被新偵測蓋掉**（todo.md T-045 P2）。
        # 這一行 `states[key] = state` 是整筆覆寫，先前把 root 設成新事件的 type，
        # 等於欄位名叫 root 卻永遠等於 latest——事件鏈的起點因此無法還原。
        #
        # 延續的條件與 Go 端摺疊 timeline 的規則刻意對稱（internal/analysis/event_timeline.go）：
        # **前一個狀態尚未終結才算同一條鏈**；已 RESOLVED／EXPIRED 之後再出現同家族事件，
        # 那是新的一條鏈，root 應該是新事件本身。
        previous_state = states.get(key)
        root_event_type = event_type
        if previous_state is not None and str(previous_state.get("state")) not in (
            LIFECYCLE_RESOLVED,
            LIFECYCLE_EXPIRED,
        ):
            root_event_type = str(previous_state.get("root_event_type") or event_type)

        state = {
            "event_key": event.get("event_key"),
            "type": event_type,
            "zone_key": zone_key,
            "event_family": event_family,
            "event_scope": event.get("event_scope"),
            "root_event_type": root_event_type,
            "latest_event_type": event_type,
            "direction": event.get("direction"),
            "state": state_name,
            "active": is_active,
            "confirmation_state": event.get("confirmation_state"),
            "expires_after_bars": _event_expires_after_bars(event_type, event_family, event.get("expires_after_bars")),
            "age_bars": int(event.get("age_bars") or 0),
            "zone_ref": event.get("zone_ref"),
            "price_level": event.get("price_level"),
            "confidence": event.get("confidence"),
            "reason_codes": list(event.get("reason_codes") or [event_type]),
            "resolved_by": None,
            "price_action_evidence": event.get("price_action_evidence"),
            # 階段 D 的隔離旗標，要跟著寫進 state_json——Go 端的桶構建讀的就是它。
            "decision_visible": _event_decision_visible(event_type, event.get("decision_visible")),
            # **兩條路徑都要寫這個旗標**：carry forward 那條在
            # _normalize_previous_event_state 無條件寫 True，這條是「這次真的偵測到」
            # 所以是 False。缺鍵不是「不是 carried」而是**異常**——Go 端據此計數
            # （sr_zones.go 的 carried_parse_failed），只寫 True 的話每一筆新事件
            # 都會被算成解析失敗，警訊就永遠不會歸零、也就沒有訊號可言。
            "carried_from_previous": False,
        }
        states[key] = state

        if not is_active:
            continue
        for family in EVENT_TYPE_META.get(event_type, {}).get("resolves", ()):
            target_key = (zone_key, str(family))
            target = states.get(target_key)
            if not target or target.get("state") in (LIFECYCLE_RESOLVED, LIFECYCLE_EXPIRED):
                continue
            target["state"] = LIFECYCLE_RESOLVED
            target["active"] = False
            target["latest_event_type"] = event_type
            target["resolved_by"] = event_type
            target["confirmation_state"] = f"RESOLVED_BY_{event_type}"
            target["reason_codes"] = [*target.get("reason_codes", []), f"RESOLVED_BY_{event_type}"]

    # **除了 states 之外的每一個桶都只收 decision_visible 的狀態**（階段 D）。
    # states 保留全部，因為它是持久化的來源——新事件要寫進 market_event_states 與
    # event_instances，只是不能被決策看到。
    #
    # 特別注意 active_bearish / active_bullish：它們**只看 direction**，
    # lifecycle_engine.resolve_lifecycle 對 active_bearish 取 truthiness 就判
    # BREAKDOWN。少了這道過濾，一個 active 且帶方向的新事件不必被任何人認識就會改決策。
    visible = [state for state in states.values() if is_decision_visible(state)]
    active = [state for state in visible if state.get("active")]
    candidates = [state for state in visible if state.get("state") == LIFECYCLE_CANDIDATE]
    confirmed = [state for state in visible if state.get("state") == LIFECYCLE_CONFIRMED]
    resolved = [state for state in visible if state.get("state") == LIFECYCLE_RESOLVED]
    expired = [state for state in visible if state.get("state") == LIFECYCLE_EXPIRED]
    active_bearish = [state for state in active if state.get("direction") == "BEARISH"]
    active_bullish = [state for state in active if state.get("direction") == "BULLISH"]
    # latest_event_type 同樣只看得見決策可見的事件：新事件的 EVENT_ORDER 排在最後，
    # 不過濾的話它會頂掉既有事件成為「最新」，而那個值是對外輸出的。
    visible_normalized = [event for event in normalized if is_decision_visible(event)]
    latest_type = visible_normalized[-1]["type"] if visible_normalized else None
    return {
        "version": "event-lifecycle-p2",
        "states": list(states.values()),
        "active": active,
        "candidates": candidates,
        "confirmed": confirmed,
        "resolved": resolved,
        "expired": expired,
        "active_bearish_events": active_bearish,
        "active_bullish_events": active_bullish,
        "latest_event_type": latest_type,
        "market_state": market_state_from_event_states(active),
    }


def market_state_from_event_states(active_states: list[dict[str, Any]]) -> str:
    active_types = {state.get("latest_event_type") or state.get("root_event_type") for state in active_states}
    if "HIGH_VOLUME_BREAKDOWN" in active_types:
        return "BREAKDOWN_RISK"
    if "INTRADAY_RECLAIM" in active_types:
        return "RECLAIM_ATTEMPT"
    if "REVERSAL_CANDIDATE" in active_types:
        return "REVERSAL_CANDIDATE"
    return "NORMAL"


def _resistance_zone_events(
    z: ZoneScore,
    current_price: float,
    candle_high: Optional[float],
    candle_low: Optional[float],
    candle_close: Optional[float],
) -> list[dict[str, Any]]:
    """壓力側事件（階段 D 新增，decision_visible=False）。

    **壓力 zone 在階段 D 之前完全不產生事件**——`detect_market_events` 的迴圈第一行是
    `if z.role != ZoneType.SUPPORT.value: continue`。這裡刻意寫成獨立的一段，而不是
    放寬那個守衛：迴圈內既有的三個分支（高量跌破 / UNDERCUT_RECLAIM / REVERSAL_CANDIDATE）
    **全部假設 support**，放寬守衛會讓壓力 zone 掉進它們，那是決策可見的改變。

    觸發條件鏡像 HIGH_VOLUME_BREAKDOWN，門檻常數沿用不新增。
    """
    out: list[dict[str, Any]] = []
    interaction = zone_interaction(z, current_price, candle_high, candle_low, candle_close)
    if not interaction["touched"]:
        return out
    evidence = interaction["price_action_evidence"]
    relative_volume = z.relative_volume or 0.0
    high_volume = relative_volume >= HIGH_VOLUME_BREAKDOWN_THRESHOLD or z.volume_confirmation == VolumeConfirmation.FAILED.value
    broke_out = bool(evidence["closed_above"]) or (candle_high is not None and candle_high > z.price_high)
    if broke_out and high_volume:
        confirmed_breakout = bool(evidence["closed_above"])
        out.append({
            "type": "RESISTANCE_BREAKOUT",
            "direction": "BULLISH",
            "lifecycle_state": LIFECYCLE_CONFIRMED if confirmed_breakout else LIFECYCLE_CANDIDATE,
            "confirmation_state": "CONFIRMED_CLOSE_ABOVE_ZONE" if confirmed_breakout else "PENDING_CLOSE_CONFIRMATION",
            "active": confirmed_breakout,
            "expires_after_bars": 2,
            "confidence": min(1.0, max(0.45, relative_volume / 3.0)),
            "zone_ref": event_zone_ref(z, current_price),
            "price_level": z.price_high,
            "reason": "壓力區被盤中或收盤突破，且量能放大或量能狀態確認失敗。",
            "detected_at": "latest_candle",
            "reason_codes": ["RESISTANCE_CLOSED_ABOVE" if confirmed_breakout else "RESISTANCE_INTRABAR_BREAK"],
            "price_action_evidence": evidence,
        })
    return out


def detect_market_events(
    zone_scores: list[ZoneScore],
    current_price: float,
    candle_high: Optional[float],
    candle_low: Optional[float],
    candle_close: Optional[float],
) -> list[dict[str, Any]]:
    events: list[dict[str, Any]] = []
    max_relative_volume = max((z.relative_volume or 0.0 for z in zone_scores), default=0.0)
    if max_relative_volume >= EXTREME_VOLUME_THRESHOLD:
        events.append({
            "type": "EXTREME_VOLUME",
            "direction": "NEUTRAL",
            "lifecycle_state": LIFECYCLE_ACTIVE,
            "confirmation_state": "CONTEXT",
            "active": True,
            "confidence": min(1.0, max_relative_volume / 4.0),
            "zone_ref": None,
            "price_level": candle_close if candle_close is not None else current_price,
            "reason": "最新量能達極端放大門檻，需搭配跌破或收復事件解讀。",
            "detected_at": "latest_candle",
            "reason_codes": ["EXTREME_VOLUME_CONTEXT"],
        })

    for z in zone_scores:
        # 依 role 分派到兩段互斥的邏輯。支撐側以下的程式碼與階段 D 之前**逐行相同**。
        if z.role == ZoneType.RESISTANCE.value:
            events.extend(_resistance_zone_events(z, current_price, candle_high, candle_low, candle_close))
            continue
        if z.role != ZoneType.SUPPORT.value:
            continue
        interaction = zone_interaction(z, current_price, candle_high, candle_low, candle_close)
        if not interaction["touched"]:
            continue
        relative_volume = z.relative_volume or 0.0
        high_volume = relative_volume >= HIGH_VOLUME_BREAKDOWN_THRESHOLD or z.volume_confirmation == VolumeConfirmation.FAILED.value
        breakdown_event_added = False
        evidence = interaction["price_action_evidence"]
        if (evidence["closed_below"] or (candle_low is not None and candle_low < z.price_low)) and high_volume:
            confirmed_breakdown = bool(evidence["closed_below"])
            events.append({
                "type": "HIGH_VOLUME_BREAKDOWN",
                "direction": "BEARISH",
                "lifecycle_state": LIFECYCLE_CONFIRMED if confirmed_breakdown else LIFECYCLE_CANDIDATE,
                "confirmation_state": "CONFIRMED_CLOSE_BELOW" if confirmed_breakdown else "PENDING_CLOSE_CONFIRMATION",
                "active": confirmed_breakdown,
                "expires_after_bars": 2,
                "confidence": min(1.0, max(0.45, relative_volume / 3.0)),
                "zone_ref": event_zone_ref(z, current_price),
                "price_level": z.price_low,
                "reason": "支撐區被盤中或收盤跌破，且量能放大或量能狀態確認失敗。",
                "detected_at": "latest_candle",
                "reason_codes": ["SUPPORT_CLOSED_BELOW" if confirmed_breakdown else "SUPPORT_INTRABAR_BREAK"],
                "price_action_evidence": evidence,
            })
            breakdown_event_added = True
            if evidence["closed_below"]:
                continue
        if evidence["reclaim_type"] == "UNDERCUT_RECLAIM":
            events.append({
                "type": "INTRADAY_RECLAIM",
                "direction": "BULLISH",
                "lifecycle_state": LIFECYCLE_CONFIRMED,
                "confirmation_state": "CONFIRMED_CLOSE_ABOVE_ZONE",
                "active": True,
                "expires_after_bars": 2,
                "confidence": min(1.0, 0.50 + z.confidence * 0.35),
                "zone_ref": event_zone_ref(z, current_price),
                "price_level": z.price_high,
                "reason": "日 K 測試支撐後收盤收回區間上緣。",
                "detected_at": "latest_candle",
                "reason_codes": ["SUPPORT_RECLAIM_CONFIRMED"],
                "price_action_evidence": evidence,
            })
            if breakdown_event_added:
                events.append({
                    "type": "REVERSAL_CANDIDATE",
                    "direction": "BULLISH",
                    "lifecycle_state": LIFECYCLE_CANDIDATE,
                    "confirmation_state": "PENDING_FOLLOW_THROUGH",
                    "active": False,
                    "expires_after_bars": 2,
                    "confidence": min(1.0, 0.50 + z.confidence * 0.35),
                    "zone_ref": event_zone_ref(z, current_price),
                    "price_level": z.price_high,
                    "reason": "高量跌破後收回支撐區上緣，形成反轉候選事件。",
                    "detected_at": "latest_candle",
                    "reason_codes": ["REVERSAL_CANDIDATE_AWAIT_CONFIRMATION"],
                    "price_action_evidence": evidence,
                })
        elif (
            not interaction["closed_below"]
            and z.confidence >= 0.45
            and (z.expected_value or 0.0) >= 0
            and z.recent_validation != RecentValidation.EXPIRED.value
        ):
            events.append({
                "type": "REVERSAL_CANDIDATE",
                "direction": "BULLISH",
                "lifecycle_state": LIFECYCLE_CANDIDATE,
                "confirmation_state": "PENDING_FOLLOW_THROUGH",
                "active": False,
                "expires_after_bars": 2,
                "confidence": min(1.0, 0.45 + z.confidence * 0.30),
                "zone_ref": event_zone_ref(z, current_price),
                "price_level": z.price_high,
                "reason": "支撐測試未失守，且 EV 與區間信心未轉弱。",
                "detected_at": "latest_candle",
                "reason_codes": ["REVERSAL_CANDIDATE_AWAIT_CONFIRMATION"],
                "price_action_evidence": evidence,
            })
        # ── 階段 D 新增：SUPPORT_RETEST_HELD（decision_visible=False） ──
        # 這是**事實**：碰到 zone 且沒有收破，不帶任何品質門檻。與上面的
        # REVERSAL_CANDIDATE 分工——後者是「守住**且** EV／信心／驗證都合格」的方向性
        # 候選，條件較嚴。兩者是不同 family，同一根 K 同時成立是正常的，互不 resolve。
        # 放在最後 append，插入序上不會影響任何既有事件。
        if not evidence["closed_below"]:
            events.append({
                "type": "SUPPORT_RETEST_HELD",
                "direction": "BULLISH",
                "lifecycle_state": LIFECYCLE_CONFIRMED,
                "confirmation_state": "CONFIRMED_CLOSE_NOT_BELOW",
                "active": True,
                "expires_after_bars": 2,
                "confidence": min(1.0, max(0.40, z.confidence)),
                "zone_ref": event_zone_ref(z, current_price),
                "price_level": z.price_low,
                "reason": "支撐區被測試且收盤未跌破區間下緣。",
                "detected_at": "latest_candle",
                "reason_codes": ["SUPPORT_RETEST_HELD"],
                "price_action_evidence": evidence,
            })
    return normalize_market_events(_truncate_events(events))
