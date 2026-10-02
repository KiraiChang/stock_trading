"""I-074 Stage 2：B／C 判讀規則——**唯一的定義處**（issue.md I-074「Stage 2 步驟 ⑦c 細部計畫 v1」「二之四」）。

規則來自「關閉條件」的**唯一 artifact 判讀矩陣**（2026-09-22 實測後定案）＋ v26 補充的差異表：
逐列判 B／C，**任一列是 C → 整體 C**（⛔ 不以多數決）。

⚠️ **只有純函式**：⛔ 沒有自己的 CLI、⛔ 不自己讀檔。唯一的呼叫端是 `stage2_archive` 驗證模式的
`--judge`——在**同一個程序**、以 `verify_stage2_graph()` 剛驗過、留在記憶體的 comparison 物件呼叫，
驗與判之間⛔ 沒有第二次讀取（「三」#9）。
⚠️ 輸出是**封閉 schema**（頂層五欄、每列六欄、`class` 與 reason code 都是封閉集合），⛔ 不輸出自由文字——
判讀結果是正式紀錄，⛔ 不讓它的形狀隨實作細節漂移（「三」#17）。
本模組必須 dependency-light：只用標準庫。
"""
from __future__ import annotations

from typing import Any, Mapping

VERDICT_SCHEMA = "i074_stage2_verdict/v1"
VERDICT_B = "B"
VERDICT_C = "C"

# 分類只看 **after 的 `action_state`**（`market_action` ⛔ 不在 replay row 裡，⛔ 不得當 classifier）。
CLASS_NON_AVOID = "NON_AVOID"
CLASS_AVOID = "AVOID"
CLASS_UNCLASSIFIED = "UNCLASSIFIED"
_CLASS_OF_AFTER_ACTION = {"HOLD": CLASS_NON_AVOID, "AVOID": CLASS_AVOID}

# B 分支允許出現在 `differences` 裡的欄位（v26 的差異表）。top-level `position_action` 有變 → C。
ALLOWED_DIFFERENCES = frozenset({"lifecycle_phase", "market_bias", "action_state",
                                 "position_action_condition", "rr_decoupling_candidate"})

ROW_SHAPE_INVALID = "ROW_SHAPE_INVALID"
POSITION_ACTION_CHANGED = "POSITION_ACTION_CHANGED"
DIFF_FIELD_NOT_ALLOWED = "DIFF_FIELD_NOT_ALLOWED"
POSITION_ACTION_CONDITION_NON_STATE_DIFF = "POSITION_ACTION_CONDITION_NON_STATE_DIFF"
RR_DECOUPLING_NOT_FALSE_TO_TRUE = "RR_DECOUPLING_NOT_FALSE_TO_TRUE"
ACTION_STATE_UNCLASSIFIED = "ACTION_STATE_UNCLASSIFIED"
CONDITION_STATE_MISMATCH = "CONDITION_STATE_MISMATCH"
LIFECYCLE_PHASE_UNEXPECTED = "LIFECYCLE_PHASE_UNEXPECTED"
MARKET_BIAS_UNEXPECTED = "MARKET_BIAS_UNEXPECTED"
ACTION_STATE_TRANSITION_UNEXPECTED = "ACTION_STATE_TRANSITION_UNEXPECTED"
FINAL_ENTRY_STATE_NOT_BLOCKED = "FINAL_ENTRY_STATE_NOT_BLOCKED"
REASON_CODES = frozenset({
    ROW_SHAPE_INVALID, POSITION_ACTION_CHANGED, DIFF_FIELD_NOT_ALLOWED, POSITION_ACTION_CONDITION_NON_STATE_DIFF,
    RR_DECOUPLING_NOT_FALSE_TO_TRUE, ACTION_STATE_UNCLASSIFIED, CONDITION_STATE_MISMATCH,
    LIFECYCLE_PHASE_UNEXPECTED, MARKET_BIAS_UNEXPECTED, ACTION_STATE_TRANSITION_UNEXPECTED,
    FINAL_ENTRY_STATE_NOT_BLOCKED,
})

_BEFORE_LIFECYCLES = ("TESTING", "CONFIRMED")
_AFTER_LIFECYCLE = "CONTINUATION"
# market_bias：(before, after)。AVOID 類由 `_market_bias()` 的短路保持不變。
_MARKET_BIAS = {CLASS_NON_AVOID: ("BULLISH_BIAS", "BULLISH_CONTINUATION"), CLASS_AVOID: ("BEARISH_BIAS", "BEARISH_BIAS")}
# action_state：類別 × before.lifecycle_phase → (before, after)。
_ACTION_STATE = {
    CLASS_NON_AVOID: {"TESTING": ("CONDITIONAL_HOLD", "HOLD"), "CONFIRMED": ("HOLD", "HOLD")},
    CLASS_AVOID: {"TESTING": ("AVOID", "AVOID"), "CONFIRMED": ("AVOID", "AVOID")},
}
_FINAL_ENTRY_STATE = "BLOCKED"
_STR_FIELDS = ("lifecycle_phase", "market_bias", "action_state", "final_entry_state")

ROW_FIELDS = ("symbol", "timeframe", "as_of", "class", "verdict", "reasons")
TOP_FIELDS = ("schema", "verdict", "row_count", "c_row_count", "rows")


class VerdictError(ValueError):
    """拒絕判讀（例如 comparison 是 0 列）——⛔ 不是 C。"""


def _shape_ok(side: object) -> bool:
    """必備欄位與型別（`type(x) is …`：⛔ 不接受 `null`、⛔ 不讓 `bool` 冒充 `int`）。"""
    if type(side) is not dict:
        return False
    if any(type(side.get(f)) is not str for f in _STR_FIELDS):
        return False
    if type(side.get("rr_decoupling_candidate")) is not bool:
        return False
    condition = side.get("position_action_condition")
    return type(condition) is dict and type(condition.get("state")) is str


def _class_of(after: object) -> str:
    action = after.get("action_state") if type(after) is dict else None
    return _CLASS_OF_AFTER_ACTION.get(action, CLASS_UNCLASSIFIED) if type(action) is str else CLASS_UNCLASSIFIED


def judge_row(row: Mapping[str, Any]) -> dict[str, Any]:
    """一列 → `{symbol, timeframe, as_of, class, verdict, reasons}`。

    除了 `ROW_SHAPE_INVALID`（只有它一個、⛔ 不再檢查）與 `ACTION_STATE_UNCLASSIFIED`（⛔ 不做依分類的兩項）
    的略過，每一項檢查**全部執行**、命中的 code 全部記下（⛔ 不在第一個不符就停）。
    """
    before, after = row.get("before"), row.get("after")
    cls = _class_of(after)
    out = {"symbol": row["symbol"], "timeframe": row["timeframe"], "as_of": row["as_of"], "class": cls}
    if not (_shape_ok(before) and _shape_ok(after)):
        return {**out, "verdict": VERDICT_C, "reasons": [ROW_SHAPE_INVALID]}

    reasons: set[str] = set()
    differences = row["differences"]
    if "position_action" in differences:
        reasons.add(POSITION_ACTION_CHANGED)
    if any(f not in ALLOWED_DIFFERENCES and f != "position_action" for f in differences):
        reasons.add(DIFF_FIELD_NOT_ALLOWED)
    b_cond, a_cond = before["position_action_condition"], after["position_action_condition"]
    if {k: v for k, v in b_cond.items() if k != "state"} != {k: v for k, v in a_cond.items() if k != "state"}:
        reasons.add(POSITION_ACTION_CONDITION_NON_STATE_DIFF)
    if not (before["rr_decoupling_candidate"] is False and after["rr_decoupling_candidate"] is True):
        reasons.add(RR_DECOUPLING_NOT_FALSE_TO_TRUE)
    if cls == CLASS_UNCLASSIFIED:
        reasons.add(ACTION_STATE_UNCLASSIFIED)
    if any(side["position_action_condition"]["state"] != side["action_state"] for side in (before, after)):
        reasons.add(CONDITION_STATE_MISMATCH)
    before_phase = before["lifecycle_phase"]
    if before_phase not in _BEFORE_LIFECYCLES or after["lifecycle_phase"] != _AFTER_LIFECYCLE:
        reasons.add(LIFECYCLE_PHASE_UNEXPECTED)
    if cls != CLASS_UNCLASSIFIED:
        if (before["market_bias"], after["market_bias"]) != _MARKET_BIAS[cls]:
            reasons.add(MARKET_BIAS_UNEXPECTED)
        # before 不是 TESTING／CONFIRMED 時只記 LIFECYCLE_PHASE_UNEXPECTED（⛔ 不重複記這一項）。
        expected = _ACTION_STATE[cls].get(before_phase)
        if expected is not None and (before["action_state"], after["action_state"]) != expected:
            reasons.add(ACTION_STATE_TRANSITION_UNEXPECTED)
    if before["final_entry_state"] != _FINAL_ENTRY_STATE or after["final_entry_state"] != _FINAL_ENTRY_STATE:
        reasons.add(FINAL_ENTRY_STATE_NOT_BLOCKED)
    return {**out, "verdict": VERDICT_C if reasons else VERDICT_B, "reasons": sorted(reasons)}


def judge_comparison(comparison: Mapping[str, Any]) -> dict[str, Any]:
    """整份 comparison（⚠️ 必須是 `verify_stage2_graph()` 剛驗過的那一個物件）→ 封閉 schema 的判讀結果。

    列的順序 ＝ comparison 的順序（`validate_comparison_artifact()` 已驗排序且唯一；⛔ 不重排）。
    0 列 → `VerdictError`（拒絕判讀，⛔ 不以空集合判 B）。
    """
    rows = comparison.get("rows")
    if not isinstance(rows, list) or not rows:
        raise VerdictError("comparison 沒有任何一列——⛔ 不以空集合判 B，拒絕判讀")
    judged = [judge_row(row) for row in rows]
    c_rows = sum(1 for r in judged if r["verdict"] == VERDICT_C)
    return {"schema": VERDICT_SCHEMA, "verdict": VERDICT_C if c_rows else VERDICT_B,
            "row_count": len(judged), "c_row_count": c_rows, "rows": judged}
