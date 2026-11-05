"""I-129 的離線影響量測：逐列比較 commit A（改前）與 commit B（改後）的 Stage 1 after artifact。

規格與判準都在計畫書（docs/issue.md I-129 指向的 plans/i129-execution-gate-authority.md「6.5」），
⛔ 量測前就定好、⛔ 不在這裡另訂。CLI 在 `python/scripts/compare-i129-replay.py`。

⚠️ **記憶體**：⛔ 不整份載入兩邊（I-117：crossday 的整份載入要 730 MiB）。先串流 A、每列只留精簡投影，
再串流 B 邊讀邊比；成交判定用 bundle 的 candles（11 檔，小）。

⚠️ **結論分四種**（`verdict`），依序判定，前一種成立就⛔ 不往下看：

1. `UNATTRIBUTABLE`：兩份 artifact 的輸入不一致（bundle、runtime settings、列的 key 集合），
   或任一列的欄位檢查不過 → ⛔ 不出結論。
2. `INVARIANT_VIOLATION`：結構不變式不成立（U 組有異常、有升級或收緊、setup 缺值的列被放行）→ 程式錯誤，停下來修。
3. `EVIDENCE_INSUFFICIENT`：任一項判準的有效樣本不足，或 G_new 的檔數／集中度不符 → 退回「不放行」，
   ⛔ 不改用較寬的判準。
4. `SAFETY_FAILED`：安全判準有一項不成立 → 退回「不放行」。

全部通過才是 `PASS`。
"""
from __future__ import annotations

import hashlib
import math
import statistics
from collections import Counter, defaultdict
from dataclasses import dataclass, field
from typing import Any, Iterable, Optional, Sequence

from .replay_bundle import (
    AFTER_KIND,
    ArtifactError,
    ProvenanceError,
    StreamRowValidator,
    check_measurement_row,
    validate_after_envelope,
    validate_provenance,
)
from .replay_bundle.canonical import canonical_json_bytes, sha256_hex
from .replay_bundle.stream import stream_canonical_artifact

# ── 判準（計畫書「6.5」的預設值，2026-11-04 隨計畫書 v4 確認）──────────────────
WINDOW_BARS = 5
MIN_SAMPLES = 30
MIN_G_NEW_SYMBOLS = 5
MAX_G_NEW_SYMBOL_SHARE = 0.50
STOP_RATE_MARGIN = 0.05      # G_new 停損觸及率 ≤ P + 5pp
TARGET_RATE_MARGIN = 0.05    # G_new 先到目標比例 ≥ P_q − 5pp
MIN_MEAN_R = 0.0
MIN_MEAN_FORWARD_RETURN = 0.0
MIN_POSITIVE_FORWARD_RATE = 0.45
# 判準的邊界是**含等號**的（≤／≥）；比較時容許浮點誤差，0.40 − 0.05 才不會因為算成 0.35000000000000003 而判不合格。
EPS = 1e-9

# 兩趟 Stage 1 都⛔ 不帶 tooling patch（計畫書 D6）：空 patch 的 `tooling_patch_sha256` 是空字串的 SHA-256
# （`scripts/lib/replay-args.sh` 的 `replay_args_tooling_patch_sha256()`）。
EMPTY_PATCH_SHA256 = hashlib.sha256(b"").hexdigest()

# 結果標籤：只由同一份凍結 candles 與日期決定、與決策無關，所以 A、B 必須完全相同（第 8 步 review P2）。
LABEL_FIELDS = (
    "current_price", "candle_open", "candle_high", "candle_low", "candle_close", "previous_candle_close",
    "forward_bars", "forward_return", "next_close_return", "two_bar_close_return",
)
LABEL_TOLERANCE = 1e-9

MARKET_BASES = {"CONTINUATION_MARKET_PRICE", "RECLAIM_CLOSE"}
LIMIT_BASES = {"PRIMARY_SUPPORT_UPPER"}

ENTRY_RANK = {"BLOCKED": 0, "WAIT_CONFIRMATION": 1, "PROBE_ALLOWED": 2, "ENTRY_ALLOWED": 3}


# ── 成交與停損、目標的判定（計畫書「6.5」的成交規則）────────────────────────────

@dataclass(frozen=True)
class Bar:
    open: float
    high: float
    low: float
    close: float


@dataclass(frozen=True)
class TradeOutcome:
    """一筆訊號的模擬結果。`filled=False` 時其餘欄位都是 None。"""

    filled: bool
    fill_index: Optional[int] = None      # 窗口內第幾根成交（0 = as-of 的下一根）
    fill_price: Optional[float] = None
    fill_kind: Optional[str] = None       # MARKET_OPEN／LIMIT_OPEN／LIMIT_INTRADAY
    exit_kind: Optional[str] = None       # STOP／TARGET／WINDOW_END
    stop_touched: Optional[bool] = None
    target_first: Optional[bool] = None   # target_price 有值時才有意義
    realized_r: Optional[float] = None    # stop_price 有值時才有
    max_adverse_pct: Optional[float] = None
    has_stop: Optional[bool] = None       # stop_price 有沒有值（停損觸及率的分母要報告缺 stop 的列數）


def simulate_trade(
    price_basis: str,
    entry_price: Optional[float],
    stop_price: Optional[float],
    target_price: Optional[float],
    bars: Sequence[Bar],
) -> Optional[TradeOutcome]:
    """依「6.5」的成交規則模擬一筆訊號。`bars` 是 as-of 之後的窗口（決策在 as-of 收盤後做出）。

    回傳 None 代表這一列⛔ 不是可模擬的進場型（price_basis 不是市價型或限價型、或缺 entry）。

    規則：
    * 市價型：下一根的開盤價成交，整根都在成交之後。
    * 限價型：窗口內第一根 `low ≤ entry` 的 K 棒成交。開盤即成交（`open ≤ entry`）→ 成交價 = open、整根在成交之後；
      盤中觸價（`open > entry`）→ 成交價 = entry，當根**只判停損、⛔ 不認列目標**
      （開盤在 entry 之上，價格一定先穿過 entry 才到 low，所以 low 必然在成交之後；high 可能在成交之前）。
    * 同一根同時碰到停損和目標 → 一律算停損先到。
    * 實現 R：成交價 > 停損時，風險 = 成交價 − 停損；停損 → `(min(open, stop) − 成交價) / 風險`
      （盤中觸價那根停損時開盤在停損之上，就是 −1）；目標 → `(target − 成交價) / 風險`；
      都沒碰到 → 窗口最後一根收盤的未實現 R。**成交價 ≤ 停損**（風險無法定義）→ 算停損、實現 R 記 −1，⛔ 不排除。
    """
    if entry_price is None or price_basis not in MARKET_BASES | LIMIT_BASES:
        return None
    entry = float(entry_price)
    stop = float(stop_price) if stop_price is not None else None
    target = float(target_price) if target_price is not None else None

    fill_index = fill_price = fill_kind = None
    if price_basis in MARKET_BASES:
        if bars:
            fill_index, fill_price, fill_kind = 0, float(bars[0].open), "MARKET_OPEN"
    else:
        for i, bar in enumerate(bars):
            if bar.low <= entry:
                if bar.open <= entry:
                    fill_index, fill_price, fill_kind = i, float(bar.open), "LIMIT_OPEN"
                else:
                    fill_index, fill_price, fill_kind = i, entry, "LIMIT_INTRADAY"
                break
    if fill_index is None:
        return TradeOutcome(filled=False)

    after = bars[fill_index:]
    lows = [b.low for b in after]
    max_adverse_pct = max(0.0, (fill_price - min(lows)) / fill_price) if fill_price else None

    if stop is not None and fill_price <= stop:
        # 跳空成交在停損之下：風險無法定義，算成交當根停損、記 −1R（⛔ 不排除，否則最差的列會消失）
        return TradeOutcome(
            filled=True, fill_index=fill_index, fill_price=fill_price, fill_kind=fill_kind,
            exit_kind="STOP", stop_touched=True, target_first=False if target is not None else None,
            realized_r=-1.0, max_adverse_pct=max_adverse_pct, has_stop=stop is not None,
        )

    risk = (fill_price - stop) if stop is not None else None
    for offset, bar in enumerate(after):
        intraday_fill_bar = offset == 0 and fill_kind == "LIMIT_INTRADAY"
        stop_hit = stop is not None and bar.low <= stop
        target_hit = target is not None and not intraday_fill_bar and bar.high >= target
        if stop_hit:
            exit_price = min(bar.open, stop) if not intraday_fill_bar else stop
            return TradeOutcome(
                filled=True, fill_index=fill_index, fill_price=fill_price, fill_kind=fill_kind,
                exit_kind="STOP", stop_touched=True, target_first=False if target is not None else None,
                realized_r=(exit_price - fill_price) / risk, max_adverse_pct=max_adverse_pct, has_stop=stop is not None,
            )
        if target_hit:
            return TradeOutcome(
                filled=True, fill_index=fill_index, fill_price=fill_price, fill_kind=fill_kind,
                exit_kind="TARGET", stop_touched=False, target_first=True,
                realized_r=(target - fill_price) / risk if risk else None, max_adverse_pct=max_adverse_pct, has_stop=stop is not None,
            )
    last_close = after[-1].close
    return TradeOutcome(
        filled=True, fill_index=fill_index, fill_price=fill_price, fill_kind=fill_kind,
        exit_kind="WINDOW_END", stop_touched=False if stop is not None else None,
        target_first=False if target is not None else None,
        realized_r=(last_close - fill_price) / risk if risk else None, max_adverse_pct=max_adverse_pct, has_stop=stop is not None,
    )


# ── 逐列比較 ──────────────────────────────────────────────────────────────────

# U 組（setup 合格）**必須完全相同**的欄位（「6.5」的分組表）。
U_IDENTICAL_FIELDS = (
    "final_entry_state", "market_action", "action", "position_action", "risk_notes_digest", "rr_context",
)
# rr_gate 只能多 `setup_qualified`。
RR_GATE_ADDED_KEYS = ("setup_qualified",)

RR_REASON_CODES = frozenset({
    "RR_INSUFFICIENT", "RR_UNAVAILABLE", "NO_PRIMARY_ZONE", "RR_NOT_QUALIFIED",
    "EXECUTION_RR_INSUFFICIENT", "EXECUTION_RR_UNAVAILABLE", "RR_BELOW_FULL_ENTRY", "SETUP_RR_BELOW_MINIMUM",
})


def _digest(value: Any) -> bytes:
    return bytes.fromhex(sha256_hex(canonical_json_bytes(value)))[:8]


_KEY_ORDERS: dict[tuple[str, ...], tuple[str, ...]] = {}


def _field_blob(row: dict[str, Any], keys: tuple[str, ...]) -> bytes:
    """依 `keys` 的順序串起每個欄位的 8 bytes digest（缺的欄位用 8 個 0xff）。"""
    missing = b"\xff" * 8
    return b"".join(_digest(row[k]) if k in row else missing for k in keys)


@dataclass
class _Projection:
    """A 側每列只留下比較需要的部分（⛔ 不留整列）。

    ⚠️ 欄位 digest 存成一段 bytes、欄位順序共用同一個 tuple——每列一個 dict 的話，13,417 列約要上百 MiB。
    """

    decision_fields_available: bool
    label_digest: bytes
    final_entry_state: Optional[str]
    action: Optional[str]
    setup_rr_qualified: Optional[bool]
    rr_gate_digest: bytes
    field_keys: tuple[str, ...]
    field_blob: bytes
    # U 組的連帶差異要比實際內容，所以這三個欄位留原值（都很小）
    semantic_reason_codes: Optional[list[str]] = None
    final_entry_reason_codes: Optional[list[str]] = None
    position_action_condition: Optional[dict[str, Any]] = None


def _project(row: dict[str, Any]) -> _Projection:
    keys = tuple(sorted(row))
    keys = _KEY_ORDERS.setdefault(keys, keys)
    return _Projection(
        decision_fields_available=bool(row.get("decision_fields_available")),
        label_digest=_digest({f: row.get(f) for f in LABEL_FIELDS}),
        final_entry_state=row.get("final_entry_state"),
        action=row.get("action"),
        setup_rr_qualified=row.get("setup_rr_qualified"),
        rr_gate_digest=_digest(row.get("rr_gate")),
        field_keys=keys,
        field_blob=_field_blob(row, keys),
        semantic_reason_codes=row.get("semantic_reason_codes"),
        final_entry_reason_codes=row.get("final_entry_reason_codes"),
        position_action_condition=row.get("position_action_condition"),
    )


def _changed_fields(a: _Projection, b_row: dict[str, Any]) -> list[str]:
    keys = a.field_keys
    extra = sorted(set(b_row) - set(keys))
    b_blob = _field_blob(b_row, keys)
    if b_blob == a.field_blob:
        return extra
    changed = [k for i, k in enumerate(keys) if a.field_blob[i * 8:(i + 1) * 8] != b_blob[i * 8:(i + 1) * 8]]
    return sorted(changed + extra)


def _rank(state: Optional[str]) -> int:
    return ENTRY_RANK.get(str(state), -1)


@dataclass
class _Collector:
    groups: Counter = field(default_factory=Counter)
    group_by_phase: dict[str, Counter] = field(default_factory=lambda: defaultdict(Counter))
    group_by_symbol: dict[str, Counter] = field(default_factory=lambda: defaultdict(Counter))
    anomalies: Counter = field(default_factory=Counter)
    anomaly_examples: list[dict[str, Any]] = field(default_factory=list)
    upgrades: int = 0
    tightened: int = 0
    setup_missing_released: int = 0
    u_changed_fields: Counter = field(default_factory=Counter)
    outcomes: dict[str, list[TradeOutcome]] = field(default_factory=lambda: defaultdict(list))
    not_simulated: Counter = field(default_factory=Counter)
    forward_returns: dict[str, list[float]] = field(default_factory=lambda: defaultdict(list))
    g_new_symbols: Counter = field(default_factory=Counter)
    label_mismatch: int = 0
    label_not_matching_bundle: int = 0

    def anomaly(self, kind: str, key: tuple[str, str, str], detail: Any = None, limit: int = 20) -> None:
        self.anomalies[kind] += 1
        if len(self.anomaly_examples) < limit:
            self.anomaly_examples.append({"kind": kind, "key": list(key), "detail": detail})


def _u_anomalies(a: _Projection, b_row: dict[str, Any]) -> list[str]:
    """U 組（setup 合格）的差異是否都在允許範圍內。回傳⛔ 不允許的差異欄位。

    允許的只有：
    * `rr_gate` 只多 `setup_qualified`（其餘鍵與 A 完全相同）。
    * B 的主 gate 不合格時（semantic 套上 execution gate、path 改看 execution gate 的連帶結果）：
      * `semantic_entry_permission_state` 只能變成 WAIT；
      * `semantic_reason_codes` 與 `final_entry_reason_codes`：多出的只能是 RR 類的 code，少掉的只能是
        `RR_BELOW_FULL_ENTRY`（v6：主 gate 先封頂到 WAIT 之後，secondary 已經沒有東西可降，依既有規則⛔ 不附它），
        其餘只能是順序不同；
      * `path_gate_state` 只能變成 `RR_BLOCKED`；
      * `position_action_condition` 只能是 `reason_codes` 少了 `WAIT_PRICE_FOLLOW_THROUGH`
        （path 變成 `RR_BLOCKED` 之後，derived view ⛔ 不再帶那個 position reason），其餘鍵完全相同。
    其餘任何欄位不同都算異常（含 `U_IDENTICAL_FIELDS`）。
    """
    changed = _changed_fields(a, b_row)
    gate = b_row.get("rr_gate") or {}
    gate_failed = gate.get("qualified") is False
    bad: list[str] = []
    for name in changed:
        if name == "rr_gate":
            stripped = {k: v for k, v in gate.items() if k not in RR_GATE_ADDED_KEYS}
            if _digest(stripped) != a.rr_gate_digest:
                bad.append(name)
        elif name == "semantic_entry_permission_state":
            if not (gate_failed and b_row.get(name) == "WAIT_CONFIRMATION"):
                bad.append(name)
        elif name == "path_gate_state":
            if not (gate_failed and b_row.get(name) == "RR_BLOCKED"):
                bad.append(name)
        elif name in ("semantic_reason_codes", "final_entry_reason_codes"):
            a_list = a.semantic_reason_codes if name == "semantic_reason_codes" else a.final_entry_reason_codes
            if not (gate_failed and _only_gate_reason_changes(a_list, b_row.get(name))):
                bad.append(name)
        elif name == "position_action_condition":
            if not (gate_failed and _only_lost_follow_through(a.position_action_condition, b_row.get(name))):
                bad.append(name)
        else:
            bad.append(name)
    return bad


# v6：B 的主 gate 不合格時，reason codes 允許的變化（多出的只能是 RR 類、少掉的只能是 `RR_BELOW_FULL_ENTRY`）。
REASON_CODES_ALLOWED_TO_DISAPPEAR = frozenset({"RR_BELOW_FULL_ENTRY"})


def _only_gate_reason_changes(a_codes: Optional[list[str]], b_codes: Optional[list[str]]) -> bool:
    a_set, b_set = set(a_codes or []), set(b_codes or [])
    return (b_set - a_set) <= RR_REASON_CODES and (a_set - b_set) <= REASON_CODES_ALLOWED_TO_DISAPPEAR


def _only_lost_follow_through(a: Optional[dict[str, Any]], b: Optional[dict[str, Any]]) -> bool:
    if not isinstance(a, dict) or not isinstance(b, dict):
        return False
    if {k: v for k, v in a.items() if k != "reason_codes"} != {k: v for k, v in b.items() if k != "reason_codes"}:
        return False
    a_codes = list(a.get("reason_codes") or [])
    return [c for c in a_codes if c != "WAIT_PRICE_FOLLOW_THROUGH"] == list(b.get("reason_codes") or [])


@dataclass(frozen=True)
class CandleIndex:
    """每檔的 K 棒（依時間排序）與 `as_of` → 位置的對照。`as_of` 的格式與 replay 列相同。"""

    bars: dict[str, list[Bar]]
    position: dict[tuple[str, str], int]

    def label_matches(self, symbol: str, as_of: str, row: dict[str, Any]) -> bool:
        """`current_price` 與 `forward_return` 要對得上 bundle 的 candles（同 evaluation 的公式：
        `close[i + forward_bars] / close[i] − 1`）。"""
        pos = self.position.get((symbol, as_of))
        fb = row.get("forward_bars")
        if pos is None or not isinstance(fb, int) or isinstance(fb, bool) or fb <= 0:
            return False
        bars = self.bars[symbol]
        if pos + fb >= len(bars):
            return False
        close, future = bars[pos].close, bars[pos + fb].close
        price, fr = row.get("current_price"), row.get("forward_return")
        if price is None or fr is None:
            return False
        return (abs(float(price) - close) <= LABEL_TOLERANCE * max(1.0, abs(close))
                and abs(float(fr) - (future / close - 1.0)) <= LABEL_TOLERANCE)

    def window(self, symbol: str, as_of: str, n: int = WINDOW_BARS) -> Optional[list[Bar]]:
        pos = self.position.get((symbol, as_of))
        if pos is None:
            return None
        bars = self.bars[symbol][pos + 1: pos + 1 + n]
        return bars if len(bars) == n else None


def build_candle_index(candles_by_symbol: dict[str, list[dict]]) -> CandleIndex:
    """⚠️ 時間轉換與 replay 相同（`evaluation._sources_from_candle_rows()`：`unit="s"`、`utc=True`），
    `as_of` 才對得上 replay 列的 `pd.Timestamp(as_of).isoformat()`。"""
    import pandas as pd

    bars: dict[str, list[Bar]] = {}
    position: dict[tuple[str, str], int] = {}
    for symbol, rows in candles_by_symbol.items():
        ordered = sorted(rows, key=lambda r: int(r["timestamp"]))
        bars[str(symbol)] = [Bar(float(r["open"]), float(r["high"]), float(r["low"]), float(r["close"])) for r in ordered]
        for i, r in enumerate(ordered):
            iso = pd.Timestamp(pd.to_datetime(int(r["timestamp"]), unit="s", utc=True)).isoformat()
            position[(str(symbol), iso)] = i
    return CandleIndex(bars=bars, position=position)


def _close_enough(actual: Any, expected: float) -> bool:
    return actual is not None and abs(float(actual) - expected) <= LABEL_TOLERANCE * max(1.0, abs(expected))


def _d3_precondition_problems(b_row: dict[str, Any]) -> list[str]:
    """G_new（setup 不合格卻放行到試單）必須滿足 D3 的放行前提（第 8 步 review 兩輪）。

    ⚠️ 不成立就是程式錯誤（`INVARIANT_VIOLATION`），⛔ 不能由績效數字抵銷。⛔ 只看旗標與 gate 回報的數字不夠：
    * **價格完整、RR 可重算**：entry／stop／target 都有值、風險與報酬都 > 0；用價格重算的 RR 必須等於
      `rr_context.executable_rr` 與 `rr_gate.actual_rr`（三方一致）。
    * **門檻重算**：用 decision engine 的 `_minimum_rr()`（共用同一份規則、⛔ 不另抄）從 `primary_zone` 的 tier／role 與
      `entry_action_state` 算出門檻，必須等於 gate 回報的 `minimum_rr`，而且重算的 RR ≥ 門檻。
    * **旗標**：`gate_basis=ENTRY_STOP_TARGET`、target 已知、主 gate 合格、`setup_qualified=false`。
    * **封頂真的生效**：semantic 的許可就是試單（⛔ 不得高於 final 的 PROBE），semantic 與 final 都帶
      `SETUP_RR_BELOW_MINIMUM`。
    """
    from types import SimpleNamespace

    from .decision_engine import _minimum_rr

    gate = b_row.get("rr_gate") or {}
    ctx = b_row.get("rr_context") or {}
    problems = []
    if gate.get("qualified") is not True:
        problems.append("rr_gate.qualified 不是 true")
    if gate.get("gate_basis") != "ENTRY_STOP_TARGET":
        problems.append(f"gate_basis={gate.get('gate_basis')!r}，⛔ 不是可量化的 executable RR")
    if gate.get("setup_qualified") is not False:
        problems.append("rr_gate.setup_qualified 不是 false")
    entry, stop, target = ctx.get("entry_price"), ctx.get("stop_price"), ctx.get("target_price")
    if ctx.get("target_known") is not True or target is None:
        problems.append("target 未知（target_known 與 target_price 要一致且有值）")
    if entry is None or stop is None or target is None:
        problems.append(f"entry／stop／target 不完整：{entry!r}／{stop!r}／{target!r}")
    else:
        risk, reward = float(entry) - float(stop), float(target) - float(entry)
        if risk <= 0 or reward <= 0:
            problems.append(f"風險 {risk!r} 或報酬 {reward!r} ≤ 0")
        else:
            rr = reward / risk
            for name, claimed in (("rr_context.executable_rr", ctx.get("executable_rr")),
                                  ("rr_gate.actual_rr", gate.get("actual_rr"))):
                if not _close_enough(claimed, rr):
                    problems.append(f"{name}={claimed!r} 與價格重算的 RR {rr:.6f} 不符")
            zone = b_row.get("primary_zone")
            if not isinstance(zone, dict):
                problems.append("primary_zone 缺席，無法重算門檻")
            else:
                minimum = _minimum_rr(
                    SimpleNamespace(tier=zone.get("tier"), role=zone.get("role")), str(b_row.get("entry_action_state")),
                )
                if not _close_enough(gate.get("minimum_rr"), minimum):
                    problems.append(f"rr_gate.minimum_rr={gate.get('minimum_rr')!r} 與重算的門檻 {minimum} 不符")
                if rr < minimum - EPS:
                    problems.append(f"重算的 RR {rr:.6f} 未達門檻 {minimum}")
    if b_row.get("semantic_entry_permission_state") != "PROBE_ALLOWED":
        problems.append(f"semantic 許可是 {b_row.get('semantic_entry_permission_state')!r}，⛔ 不是試單（封頂沒有生效）")
    for field_name in ("semantic_reason_codes", "final_entry_reason_codes"):
        if "SETUP_RR_BELOW_MINIMUM" not in (b_row.get(field_name) or []):
            problems.append(f"{field_name} 缺少 SETUP_RR_BELOW_MINIMUM")
    return problems


def _group_of(a: _Projection, b_row: dict[str, Any]) -> str:
    b_state = b_row.get("final_entry_state")
    if not a.decision_fields_available and not b_row.get("decision_fields_available"):
        return "NO_DECISION"   # 沒有 zone 等：兩邊都沒有決策可比
    if a.setup_rr_qualified:
        if a.final_entry_state == "PROBE_ALLOWED" and b_state == "PROBE_ALLOWED":
            return "P"
        return "U"
    if b_state == "PROBE_ALLOWED" and a.final_entry_state == "BLOCKED":
        return "G_new"
    if b_state == "WAIT_CONFIRMATION":
        return "G_wait"
    if b_state == "BLOCKED":
        return "G_block"
    return "G_other"


def _is_commit(value: Any) -> bool:
    return isinstance(value, str) and len(value) == 40 and all(c in "0123456789abcdef" for c in value)


def _identity_problems(a_top: dict, b_top: dict, *, bundle_id: str, expected_a: str, expected_b: str) -> list[str]:
    """兩份 artifact 的來源完整性與執行身分（第 8 步 review P1-1）。

    ⛔ 任何一項不符都⛔ 不出結論：那時兩邊的差異⛔ 不能歸因於 I-129。檢查：
    頂層 envelope 與 provenance 的封閉欄位與型別（`validate_provenance(role="stage1")`）、
    bundle 相同、timeframe 與 replay_scope（全候選）相同、image 與套件環境相同、兩趟都⛔ 沒有 tooling patch、
    base commit 分別等於預期的 A／B（而且 A ≠ B）、runtime settings 相同。
    """
    problems: list[str] = []
    if not (_is_commit(expected_a) and _is_commit(expected_b)):
        problems.append("預期的 A／B commit 必須是 40 字元小寫 hex")
    elif expected_a == expected_b:
        problems.append("預期的 A、B commit 相同，比較沒有意義")
    provs: dict[str, dict] = {}
    for side, top, expected in (("A", a_top, expected_a), ("B", b_top, expected_b)):
        try:
            validate_after_envelope(top | {"rows": []})
        except ArtifactError as exc:
            problems.append(f"{side} 的 envelope 不合法：{exc}")
            continue
        try:
            validate_provenance(top.get("provenance"), role="stage1")
        except ProvenanceError as exc:
            problems.append(f"{side} 的 provenance 不合法：{exc}")
            continue
        prov = top["provenance"]
        provs[side] = prov
        if top.get("bundle_id") != bundle_id:
            problems.append(f"{side} 的 bundle_id={top.get('bundle_id')!r} 與 bundle {bundle_id!r} 不同")
        if top.get("replay_scope") != "all_candidates":
            problems.append(f"{side} 的 replay_scope={top.get('replay_scope')!r}，⛔ 不是全候選")
        if prov["tooling_patch_sha256"] != EMPTY_PATCH_SHA256:
            problems.append(f"{side} 帶了 tooling patch（tooling_patch_sha256={prov['tooling_patch_sha256']}）")
        if prov["base_commit"] != expected:
            problems.append(f"{side} 的 base_commit={prov['base_commit']} 與預期 {expected} 不同")
    if len(provs) == 2:
        a, b = provs["A"], provs["B"]
        for name in ("image_digest", "python_version", "pip_freeze_sha256", "runtime_settings"):
            if a[name] != b[name]:
                problems.append(f"兩趟的 {name} 不同")
        if a_top.get("timeframe") != b_top.get("timeframe"):
            problems.append("兩趟的 timeframe 不同")
    return problems


def compare_artifacts(
    a_path: str,
    b_path: str,
    candles: CandleIndex,
    *,
    bundle_id: str,
    expected_a_commit: str,
    expected_b_commit: str,
) -> dict[str, Any]:
    """串流比較兩份 Stage 1 after artifact，回傳報告（含 `verdict`）。"""
    unattributable: list[str] = []

    # ① 串流 A：全規則驗證＋量測欄位檢查，只留精簡投影
    a_rows: dict[tuple[str, str, str], _Projection] = {}
    a_available = 0
    a_validator = StreamRowValidator("A（改前）", side="after")

    def on_a(row: Any, _raw: bytes) -> None:
        nonlocal a_available
        key = a_validator.feed(row)
        if check_measurement_row(row, "A（改前）"):
            a_available += 1
        a_rows[key] = _project(row)

    a_load = stream_canonical_artifact(a_path, AFTER_KIND, on_row=on_a)
    a_validator.finish()

    # ② 串流 B：邊讀邊比
    col = _Collector()
    b_keys: set[tuple[str, str, str]] = set()
    b_available = 0
    b_validator = StreamRowValidator("B（改後）", side="after")

    def on_b(row: Any, _raw: bytes) -> None:
        nonlocal b_available
        key = b_validator.feed(row)
        if check_measurement_row(row, "B（改後）"):
            b_available += 1
        b_keys.add(key)
        a = a_rows.get(key)
        if a is None:
            return  # key 集合不一致：最後整體判 UNATTRIBUTABLE
        _compare_row(col, key, a, row, candles)

    b_load = stream_canonical_artifact(b_path, AFTER_KIND, on_row=on_b)
    b_validator.finish()

    # ③ 輸入一致性
    unattributable.extend(_identity_problems(
        a_load.top, b_load.top, bundle_id=bundle_id, expected_a=expected_a_commit, expected_b=expected_b_commit,
    ))
    if a_available == 0 or b_available == 0:
        unattributable.append("有一側沒有任何 decision_fields_available=true 的列")
    if col.label_mismatch:
        unattributable.append(f"結果標籤在兩趟之間不同：{col.label_mismatch} 列（同一份凍結資料、同一天必須相同）")
    if col.label_not_matching_bundle:
        unattributable.append(f"G_new／P 的結果標籤對不上 bundle 的 candles：{col.label_not_matching_bundle} 列")
    if set(a_rows) != b_keys:
        unattributable.append(
            f"列的 key 集合不同（只在 A：{len(set(a_rows) - b_keys)}、只在 B：{len(b_keys - set(a_rows))}）"
        )

    return _report(col, a_load, b_load, len(a_rows), unattributable)


def _compare_row(col: _Collector, key, a: _Projection, b_row: dict[str, Any], candles: CandleIndex) -> None:
    symbol = key[0]
    if a.label_digest != _digest({f: b_row.get(f) for f in LABEL_FIELDS}):
        col.label_mismatch += 1
        col.anomaly("LABEL_MISMATCH", key, {f: b_row.get(f) for f in ("forward_return", "current_price")})
    if a.decision_fields_available != bool(b_row.get("decision_fields_available")):
        col.anomaly("DECISION_AVAILABILITY_DIFFERS", key,
                    {"a": a.decision_fields_available, "b": b_row.get("decision_fields_available")})
    b_setup = b_row.get("setup_rr_qualified")
    if b_setup != a.setup_rr_qualified:
        col.anomaly("SETUP_RR_QUALIFIED_DIFFERS", key, {"a": a.setup_rr_qualified, "b": b_setup})
    group = _group_of(a, b_row)
    col.groups[group] += 1
    col.group_by_phase[group][str(b_row.get("lifecycle_phase"))] += 1
    col.group_by_symbol[group][symbol] += 1

    a_state, b_state = a.final_entry_state, b_row.get("final_entry_state")
    # 升級：PROBE → ENTRY，或 action BuySmall → Buy（必須是 0）
    if (a_state == "PROBE_ALLOWED" and b_state == "ENTRY_ALLOWED") or (
        a.action == "BuySmall" and b_row.get("action") == "Buy"
    ):
        col.upgrades += 1
        col.anomaly("UPGRADE", key, {"a": [a_state, a.action], "b": [b_state, b_row.get("action")]})
    # 收緊：A 是 PROBE／ENTRY、B 比 A 低（必須是 0）
    if a_state in ("PROBE_ALLOWED", "ENTRY_ALLOWED") and _rank(b_state) < _rank(a_state):
        col.tightened += 1
        col.anomaly("TIGHTENED", key, {"a": a_state, "b": b_state})
    # `rr_gate.setup_qualified` 與 validation-only 的 `setup_rr_qualified` 出自同一顆 setup gate
    gate = b_row.get("rr_gate") or {}
    if b_row.get("decision_fields_available") and gate.get("setup_qualified") is not b_setup:
        col.anomaly("SETUP_QUALIFIED_MISMATCH", key, {"rr_gate": gate.get("setup_qualified"), "diagnostic": b_setup})
    if b_row.get("decision_fields_available"):
        semantic_state = b_row.get("semantic_entry_permission_state")
        # final 只會再套更多 gate：⛔ 不得高於 semantic
        if _rank(b_state) > _rank(semantic_state):
            col.anomaly("FINAL_ABOVE_SEMANTIC", key, {"semantic": semantic_state, "final": b_state})
        # setup 不合格最多試單（D3）：semantic、final 都⛔ 不得高於試單，action ⛔ 不得是 Buy
        if b_setup is False and (
            _rank(semantic_state) > ENTRY_RANK["PROBE_ALLOWED"] or _rank(b_state) > ENTRY_RANK["PROBE_ALLOWED"]
            or b_row.get("action") == "Buy"
        ):
            col.anomaly("SETUP_FAIL_ABOVE_PROBE", key,
                        {"semantic": semantic_state, "final": b_state, "action": b_row.get("action")})
    # BLOCKED 只留給結構事實（D1）：理由⛔ 不得是 RR 類
    if b_state == "BLOCKED" and set(b_row.get("final_entry_reason_codes") or []) & RR_REASON_CODES:
        col.anomaly("BLOCKED_WITH_RR_REASON", key, b_row.get("final_entry_reason_codes"))
    # setup 缺值的列⛔ 不得放行（D2）
    setup_rr = (b_row.get("rr_context") or {}).get("setup_rr")
    if b_row.get("decision_fields_available") and setup_rr is None and b_state in ("PROBE_ALLOWED", "ENTRY_ALLOWED"):
        col.setup_missing_released += 1
        col.anomaly("SETUP_MISSING_RELEASED", key, b_state)

    if group in ("U", "P"):
        bad = _u_anomalies(a, b_row)
        for name in bad:
            col.u_changed_fields[name] += 1
        if bad:
            col.anomaly("U_FIELD_CHANGED", key, bad)
    elif group == "G_other":
        col.anomaly("SETUP_FAIL_UNEXPECTED_STATE", key, {"a": a_state, "b": b_state})
    elif group in ("G_wait", "G_block") and a_state != "BLOCKED":
        col.anomaly("SETUP_FAIL_A_NOT_BLOCKED", key, {"a": a_state, "b": b_state})

    if group == "G_new":
        col.g_new_symbols[symbol] += 1
        problems = _d3_precondition_problems(b_row)
        if problems:
            col.anomaly("G_NEW_PRECONDITION", key, problems)
    if group in ("G_new", "P"):
        if not candles.label_matches(symbol, key[2], b_row):
            col.label_not_matching_bundle += 1
            col.anomaly("LABEL_NOT_MATCHING_BUNDLE", key, {f: b_row.get(f) for f in ("forward_return", "current_price")})
        fr = b_row.get("forward_return")
        if fr is not None and math.isfinite(float(fr)):
            col.forward_returns[group].append(float(fr))
        ctx = b_row.get("rr_context") or {}
        window = candles.window(symbol, key[2])
        if window is None:
            col.not_simulated[f"{group}:NO_WINDOW"] += 1
            return
        outcome = simulate_trade(
            str(ctx.get("price_basis") or ""), ctx.get("entry_price"), ctx.get("stop_price"),
            ctx.get("target_price"), window,
        )
        if outcome is None:
            col.not_simulated[f"{group}:NOT_TRADEABLE"] += 1
            return
        col.outcomes[group].append(outcome)


# ── 指標與判準 ────────────────────────────────────────────────────────────────

def _rate(flags: Iterable[bool]) -> tuple[Optional[float], int]:
    values = [bool(f) for f in flags]
    return (sum(values) / len(values) if values else None), len(values)


def _mean(values: list[float]) -> Optional[float]:
    return statistics.fmean(values) if values else None


def _quantile(values: list[float], q: float) -> Optional[float]:
    if not values:
        return None
    ordered = sorted(values)
    idx = min(len(ordered) - 1, max(0, math.ceil(q * len(ordered)) - 1))
    return ordered[idx]


def outcome_metrics(outcomes: list[TradeOutcome], forward_returns: list[float]) -> dict[str, Any]:
    """一組的結果指標，每項都附分母（「6.5」的「結果指標與分母」）。"""
    filled = [o for o in outcomes if o.filled]
    with_target = [o for o in filled if o.target_first is not None]
    with_r = [o for o in filled if o.realized_r is not None]
    # ⚠️ 停損觸及率的分母是**已成交的列**（計畫書原定義）：stop 缺值的列算「沒碰到」、⛔ 不排除——
    # 排除會拉高對照組 P 的觸及率，等於放寬 G_new 的容許值（第 8 步 review）。
    stop_rate, stop_n = _rate(bool(o.stop_touched) for o in filled)
    target_rate, target_n = _rate(o.target_first for o in with_target)
    mae = [o.max_adverse_pct for o in filled if o.max_adverse_pct is not None]
    return {
        "signals": len(outcomes),
        "filled": len(filled),
        "fill_rate": (len(filled) / len(outcomes)) if outcomes else None,
        "fill_kinds": dict(Counter(o.fill_kind for o in filled)),
        "exit_kinds": dict(Counter(o.exit_kind for o in filled)),
        # 停損觸及率：已成交的列（stop 缺值的列算沒碰到；數量另外列出）
        "stop_touch_rate": stop_rate, "stop_touch_n": stop_n,
        "filled_without_stop": sum(1 for o in filled if not o.has_stop),
        # 先到目標的比例：已成交、而且 target 有值的列
        "target_first_rate": target_rate, "target_first_n": target_n,
        # 平均實現 R：已成交、而且 stop 有值的列（成交價 ≤ 停損記 −1，⛔ 不排除）
        "mean_realized_r": _mean([o.realized_r for o in with_r]), "realized_r_n": len(with_r),
        # governance 用的 forward_return
        "mean_forward_return": _mean(forward_returns), "forward_return_n": len(forward_returns),
        "positive_forward_rate": _rate(fr > 0 for fr in forward_returns)[0],
        # 只報告、⛔ 不當判準
        "max_adverse_pct_median": statistics.median(mae) if mae else None,
        "max_adverse_pct_p90": _quantile(mae, 0.90),
    }


def judge(
    *,
    unattributable: list[str],
    invariants: dict[str, int],
    g_new: dict[str, Any],
    p: dict[str, Any],
    g_new_symbols: Counter,
) -> tuple[str, list[str]]:
    """依「6.5」的判準依序判定。回傳 `(verdict, reasons)`。"""
    if unattributable:
        return "UNATTRIBUTABLE", list(unattributable)
    broken = [f"{k}={v}" for k, v in invariants.items() if v]
    if broken:
        return "INVARIANT_VIOLATION", broken

    insufficient: list[str] = []
    for name, metric, n_key in (
        ("G_new 停損觸及率", g_new, "stop_touch_n"), ("P 停損觸及率", p, "stop_touch_n"),
        ("G_new 先到目標", g_new, "target_first_n"), ("P_q 先到目標", p, "target_first_n"),
        ("G_new 實現 R", g_new, "realized_r_n"), ("P 實現 R", p, "realized_r_n"),
        ("G_new forward_return", g_new, "forward_return_n"),
    ):
        if metric[n_key] < MIN_SAMPLES:
            insufficient.append(f"{name} 有效樣本 {metric[n_key]} < {MIN_SAMPLES}")
    total = sum(g_new_symbols.values())
    if len(g_new_symbols) < MIN_G_NEW_SYMBOLS:
        insufficient.append(f"G_new 只有 {len(g_new_symbols)} 檔 < {MIN_G_NEW_SYMBOLS}")
    if total and max(g_new_symbols.values()) / total > MAX_G_NEW_SYMBOL_SHARE + EPS:
        top, n = g_new_symbols.most_common(1)[0]
        insufficient.append(f"G_new 單一檔 {top} 占 {n}/{total} > {MAX_G_NEW_SYMBOL_SHARE:.0%}")
    if insufficient:
        # ⛔ 不改用較寬的判準（計畫書 D5）
        return "EVIDENCE_INSUFFICIENT", insufficient

    failed: list[str] = []
    if g_new["stop_touch_rate"] > p["stop_touch_rate"] + STOP_RATE_MARGIN + EPS:
        failed.append(f"G_new 停損觸及率 {g_new['stop_touch_rate']:.3f} > P {p['stop_touch_rate']:.3f} + {STOP_RATE_MARGIN}")
    if g_new["target_first_rate"] < p["target_first_rate"] - TARGET_RATE_MARGIN - EPS:
        failed.append(f"G_new 先到目標 {g_new['target_first_rate']:.3f} < P_q {p['target_first_rate']:.3f} − {TARGET_RATE_MARGIN}")
    if g_new["mean_realized_r"] < MIN_MEAN_R - EPS:
        failed.append(f"G_new 平均實現 R {g_new['mean_realized_r']:.3f} < {MIN_MEAN_R}")
    if g_new["mean_forward_return"] < MIN_MEAN_FORWARD_RETURN - EPS:
        failed.append(f"G_new 平均 forward_return {g_new['mean_forward_return']:.4f} < {MIN_MEAN_FORWARD_RETURN}")
    if g_new["positive_forward_rate"] < MIN_POSITIVE_FORWARD_RATE - EPS:
        failed.append(f"G_new 正報酬比例 {g_new['positive_forward_rate']:.3f} < {MIN_POSITIVE_FORWARD_RATE}")
    if failed:
        return "SAFETY_FAILED", failed
    return "PASS", []


def _report(col: _Collector, a_load, b_load, row_count: int, unattributable: list[str]) -> dict[str, Any]:
    u_anomaly_rows = col.anomalies.get("U_FIELD_CHANGED", 0)
    invariants = {
        "u_field_changed_rows": u_anomaly_rows,
        "upgrades": col.upgrades,
        "tightened": col.tightened,
        "setup_missing_released": col.setup_missing_released,
        "setup_rr_qualified_differs": col.anomalies.get("SETUP_RR_QUALIFIED_DIFFERS", 0),
        "setup_fail_unexpected_state": col.anomalies.get("SETUP_FAIL_UNEXPECTED_STATE", 0),
        "setup_fail_a_not_blocked": col.anomalies.get("SETUP_FAIL_A_NOT_BLOCKED", 0),
        "decision_availability_differs": col.anomalies.get("DECISION_AVAILABILITY_DIFFERS", 0),
        "blocked_with_rr_reason": col.anomalies.get("BLOCKED_WITH_RR_REASON", 0),
        "g_new_precondition": col.anomalies.get("G_NEW_PRECONDITION", 0),
        "setup_qualified_mismatch": col.anomalies.get("SETUP_QUALIFIED_MISMATCH", 0),
        "final_above_semantic": col.anomalies.get("FINAL_ABOVE_SEMANTIC", 0),
        "setup_fail_above_probe": col.anomalies.get("SETUP_FAIL_ABOVE_PROBE", 0),
    }
    g_new = outcome_metrics(col.outcomes.get("G_new", []), col.forward_returns.get("G_new", []))
    p = outcome_metrics(col.outcomes.get("P", []), col.forward_returns.get("P", []))
    verdict, reasons = judge(
        unattributable=unattributable, invariants=invariants, g_new=g_new, p=p, g_new_symbols=col.g_new_symbols,
    )
    def prov(load) -> dict[str, Any]:
        pv = load.top.get("provenance") or {}
        return {"base_commit": pv.get("base_commit"), "image_digest": pv.get("image_digest"),
                "tooling_patch_sha256": pv.get("tooling_patch_sha256"), "artifact_sha256": load.artifact_sha256}
    return {
        "schema": "i129_measurement_report_v1",
        "verdict": verdict,
        "verdict_reasons": reasons,
        "rows": row_count,
        "inputs": {"a": prov(a_load), "b": prov(b_load), "bundle_id": a_load.top.get("bundle_id")},
        "groups": dict(col.groups),
        "groups_by_lifecycle_phase": {g: dict(c) for g, c in col.group_by_phase.items()},
        "groups_by_symbol": {g: dict(c) for g, c in col.group_by_symbol.items()},
        "invariants": invariants,
        "u_changed_fields": dict(col.u_changed_fields),
        "anomaly_examples": col.anomaly_examples,
        "not_simulated": dict(col.not_simulated),
        "metrics": {"G_new": g_new, "P": p},
        "g_new_symbols": dict(col.g_new_symbols),
        "criteria": {
            "min_samples": MIN_SAMPLES, "min_g_new_symbols": MIN_G_NEW_SYMBOLS,
            "max_g_new_symbol_share": MAX_G_NEW_SYMBOL_SHARE, "stop_rate_margin": STOP_RATE_MARGIN,
            "target_rate_margin": TARGET_RATE_MARGIN, "min_mean_r": MIN_MEAN_R,
            "min_mean_forward_return": MIN_MEAN_FORWARD_RETURN, "min_positive_forward_rate": MIN_POSITIVE_FORWARD_RATE,
            "window_bars": WINDOW_BARS,
        },
    }
