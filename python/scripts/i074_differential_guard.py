#!/usr/bin/env python3
"""I-074 Stage 2 步驟 ⑨：differential guard 的第三方 comparator（docs/issue.md「Stage 2 步驟 ⑨ 細部計畫 v1」「四」）。

兩個獨立的容器程序各自以 `i074_decision_summary_producer.py` 在**原始 `e1cbbbd`** 與 **T2（`e1cbbbd` ＋ counterfactual
＋ tooling）** 上產出 decision summary；本程式是**第三個程序**，依 v29「六、3」的 allowlist 比對兩份輸出。

⚠️ **方向寫死**：formal before ＝ **patched**、formal after ＝ **原始**——參數一律以角色命名（`--before-patched`、
`--after-original`），⛔ 不用位置參數。

⚠️ **只用標準庫、⛔ 不 import 任何產品模組**（它要在 host 的 Python 3.9 上跑，也⛔ 不能被任何一側的程式影響）；
唯一的外部程式是 `git cat-file`（以 git 物件重算兩側的預期模組 SHA，G2）。

結束碼：0 ＝ 通過（含 `--self-test-tampers` 時每一類 tamper 都以預期的原因碼失敗）；1 ＝ 不通過或輸入不合法。
"""
from __future__ import annotations

import argparse
import copy
import hashlib
import json
import re
import subprocess
import sys
from pathlib import Path
from typing import Any, Callable, Dict, List, Optional, Tuple

FIXTURE_SCHEMA = "i074_differential_guard_cases_v1"
SUMMARY_SET_SCHEMA = "i074_decision_summary_set_v1"
REPORT_SCHEMA = "i074_differential_guard_report_v1"

# 九格 ＋ 強制補充格 8b（D2(a)），順序固定。
CELLS = ("1", "2", "3", "4", "5", "6", "7", "8", "8b", "9")
TARGET_CELLS = ("1", "2", "3", "4")
OUTSIDE_DOMAIN_CELL = "8"

FIXTURE_TOP_KEYS = {"schema", "description", "zone_defaults", "model_bundle", "input_defaults", "cases"}
FIXTURE_CASE_KEYS = {"id", "cell", "class", "outside_replay_domain", "description", "inputs"}
FIXTURE_INPUT_KEYS = {
    "zones", "current_price", "global_trend", "global_volatility", "global_metrics", "chip_summary",
    "candle_open", "candle_high", "candle_low", "candle_close",
    "previous_candle_open", "previous_candle_high", "previous_candle_low", "previous_candle_close",
    "previous_event_states", "bar_advanced", "data_quality_metadata", "model_governance",
}
FIXTURE_REQUIRED_INPUT_KEYS = {"zones", "current_price", "candle_close"}
MODEL_BUNDLE_KEYS = {"hold_model", "break_model", "feature_names", "trained_at", "version", "config_hash"}
SUMMARY_SET_KEYS = {"schema", "side", "fixture_sha256", "producer_sha256", "python_version", "loaded_modules", "cases"}

# 兩個產品檔（`scripts/lib/replay-args.sh` 的 `I074_CF_PRODUCT_PATHS`，相對於 `python/`）與 tooling 路徑。
CF_PRODUCT_MODULES = ("backtest/modular/sr_scoring/decision_engine.py", "backtest/modular/sr_scoring/lifecycle_engine.py")
TOOLING_EVAL_MODULE = "backtest/modular/sr_scoring/evaluation.py"
TOOLING_DIR_PREFIX = "backtest/modular/sr_scoring/replay_bundle/"

SP = ("decision_derived_view", "semantic_pipeline")
DDV = ("decision_derived_view",)
Path_ = Tuple[str, ...]

# ⚠️ **精確 JSON path allowlist**（v29「六、3」的表，14 條；⛔ 不用前綴、⛔ 不用父路徑、⛔ 不用萬用字元）。
# 要增列必須先證明該欄位依賴 `lifecycle_phase` 並更新計畫、經確認。
ALLOWLIST: Tuple[Path_, ...] = (
    SP + ("lifecycle_phase",),
    SP + ("market_state",),
    SP + ("action_state",),
    SP + ("bias_state",),
    SP + ("reason_codes",),
    SP + ("rr_decoupling_candidate",),
    DDV + ("bias_state",),
    DDV + ("bias_label",),
    DDV + ("bias_reason_codes",),
    DDV + ("authority_reason_codes",),
    DDV + ("position_gate_state",),
    ("market_bias",),
    ("market_bias_label",),
    ("position_action_condition", "state"),
)
# 以「整個 list」比對的 allowlist 路徑（其餘 list 一律攤平成逐項的葉節點）。
ATOMIC_LISTS: Tuple[Path_, ...] = (SP + ("reason_codes",), DDV + ("bias_reason_codes",), DDV + ("authority_reason_codes",))
# 目標格的上游輸入：必須完全相同、⛔ 不允許任何差異（另以專屬原因碼報告）。
UPSTREAM: Tuple[Path_, ...] = (
    SP + ("setup_rr_qualified",), SP + ("event_signal",), ("event_state_summary",),
    ("market_regime", "structure_state"), ("primary_zone",), ("rr_gate",),
)
AUTHORITY_PARTS = ("bias_reason_codes", "daily_reason_codes", "final_entry_reason_codes", "path_reason_codes",
                   "position_reason_codes")
# `e1cbbbd` 的 `_market_bias_label()` 對這三個 state 的值（BEARISH_BIAS 是 AVOID 兩格的 bias）。
BIAS_LABEL = {"BULLISH_BIAS": "偏多觀察", "BULLISH_CONTINUATION": "多頭延續", "BEARISH_BIAS": "偏空觀察"}
TARGET_CODE = "PRICE_UPSIDE_FOLLOW_THROUGH"

# 原因碼（tamper 測試以它們斷言是**哪一道**檢查擋下的）。
G1_SCHEMA = "G1_SCHEMA"
G1_FIXTURE_SHA = "G1_FIXTURE_SHA"
G1_PRODUCER_SHA = "G1_PRODUCER_SHA"
G1_PYTHON_VERSION = "G1_PYTHON_VERSION"
G1_SIDE = "G1_SIDE"
G1_CASE_IDS = "G1_CASE_IDS"
G2_IDENTITY_PATCHED = "G2_IDENTITY_PATCHED"
G2_IDENTITY_ORIGINAL = "G2_IDENTITY_ORIGINAL"
G2_DIFF_OUTSIDE_SCOPE = "G2_DIFF_OUTSIDE_SCOPE"
G2_PRODUCT_NOT_LOADED = "G2_PRODUCT_NOT_LOADED"
G2_PRODUCT_NOT_DIFFERENT = "G2_PRODUCT_NOT_DIFFERENT"
G3_SHAPE = "G3_SHAPE"
G4_CLASS = "G4_CLASS"
G5_CONTROL_DIFFERS = "G5_CONTROL_DIFFERS"
G6_UPSTREAM = "G6_UPSTREAM"
G6_PATH_NOT_ALLOWED = "G6_PATH_NOT_ALLOWED"
G6_UNEXPECTED_VALUE = "G6_UNEXPECTED_VALUE"
G6_REASON_CODES = "G6_REASON_CODES"
G6_REASON_CODES_ORDER = "G6_REASON_CODES_ORDER"
G6_AUTHORITY = "G6_AUTHORITY"
G6_CANDIDATE = "G6_CANDIDATE"


class GuardInputError(ValueError):
    """輸入（fixture、producer 輸出）本身不合法——⛔ 不進入比對，結束碼 1。"""


# ── fixture ────────────────────────────────────────────────────────────────

def sha256_bytes(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def _reject_constant(name: str) -> Any:
    raise GuardInputError(f"JSON 含有非有限的數值 {name}")


def load_json_strict(path: Path) -> Tuple[Any, bytes]:
    raw = path.read_bytes()
    try:
        return json.loads(raw.decode("utf-8"), parse_constant=_reject_constant), raw
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise GuardInputError(f"{path} 不是合法的 UTF-8 JSON：{exc}") from exc


def validate_fixture(fixture: Any) -> None:
    """封閉 schema、十個 case 恰好依序、類別、資料域標註（只有第 8 格可以而且必須 `current_price ≠ candle_close`）。"""
    if not isinstance(fixture, dict) or set(fixture) != FIXTURE_TOP_KEYS:
        raise GuardInputError(f"fixture 的頂層鍵必須恰好是 {sorted(FIXTURE_TOP_KEYS)}")
    if fixture["schema"] != FIXTURE_SCHEMA:
        raise GuardInputError(f"fixture schema 不符：{fixture['schema']!r}")
    if not isinstance(fixture["zone_defaults"], dict) or not isinstance(fixture["input_defaults"], dict):
        raise GuardInputError("zone_defaults／input_defaults 必須是 object")
    if set(fixture["input_defaults"]) - FIXTURE_INPUT_KEYS:
        raise GuardInputError(f"input_defaults 有未知鍵：{sorted(set(fixture['input_defaults']) - FIXTURE_INPUT_KEYS)}")
    if not isinstance(fixture["model_bundle"], dict) or set(fixture["model_bundle"]) != MODEL_BUNDLE_KEYS:
        raise GuardInputError(f"model_bundle 的鍵必須恰好是 {sorted(MODEL_BUNDLE_KEYS)}")
    cases = fixture["cases"]
    if not isinstance(cases, list):
        raise GuardInputError("cases 必須是 list")
    if [c.get("cell") if isinstance(c, dict) else None for c in cases] != list(CELLS):
        raise GuardInputError(f"cases 的 cell 必須恰好依序是 {list(CELLS)}（九格 ＋ 8b）")
    ids = []
    for case in cases:
        if set(case) != FIXTURE_CASE_KEYS:
            raise GuardInputError(f"case {case.get('id')!r} 的鍵必須恰好是 {sorted(FIXTURE_CASE_KEYS)}")
        if not isinstance(case["id"], str) or not re.fullmatch(r"[a-z0-9_]+", case["id"]):
            raise GuardInputError(f"case id 不合法：{case['id']!r}")
        ids.append(case["id"])
        expected_class = "target" if case["cell"] in TARGET_CELLS else "control"
        if case["class"] != expected_class:
            raise GuardInputError(f"case {case['id']}：第 {case['cell']} 格的類別必須是 {expected_class}")
        inputs = case["inputs"]
        if not isinstance(inputs, dict) or set(inputs) - FIXTURE_INPUT_KEYS:
            raise GuardInputError(f"case {case['id']} 的 inputs 有未知鍵")
        if FIXTURE_REQUIRED_INPUT_KEYS - set(inputs):
            raise GuardInputError(f"case {case['id']} 的 inputs 缺 {sorted(FIXTURE_REQUIRED_INPUT_KEYS - set(inputs))}")
        if not isinstance(inputs["zones"], list) or not all(isinstance(z, dict) for z in inputs["zones"]):
            raise GuardInputError(f"case {case['id']} 的 zones 必須是 object 的 list")
        if not isinstance(case["outside_replay_domain"], bool):
            raise GuardInputError(f"case {case['id']} 的 outside_replay_domain 必須是 boolean")
        differs = inputs["current_price"] != inputs["candle_close"]
        if case["cell"] == OUTSIDE_DOMAIN_CELL:
            if not (case["outside_replay_domain"] and differs):
                raise GuardInputError("第 8 格必須標 outside_replay_domain: true，而且 current_price ≠ candle_close")
        elif case["outside_replay_domain"] or differs:
            raise GuardInputError(f"case {case['id']}：只有第 8 格可以 current_price ≠ candle_close／標 outside_replay_domain")
    if len(set(ids)) != len(ids):
        raise GuardInputError("case id 重複")


# ── JSON 路徑 ──────────────────────────────────────────────────────────────

def render(path: Path_) -> str:
    return ".".join(path) if path else "<root>"


def get_path(doc: Any, path: Path_) -> Any:
    cur = doc
    for key in path:
        if not isinstance(cur, dict) or key not in cur:
            return _MISSING
        cur = cur[key]
    return cur


class _Missing:
    def __repr__(self) -> str:
        return "<absent>"


_MISSING = _Missing()


def flatten(doc: Any, prefix: Path_ = ()) -> Dict[Path_, Any]:
    """葉節點 → 值。list 以 `[i]` 當路徑元素；`ATOMIC_LISTS` 整個當成一個值。"""
    out: Dict[Path_, Any] = {}
    if prefix in ATOMIC_LISTS:
        out[prefix] = doc
    elif isinstance(doc, dict) and doc:
        for key, value in doc.items():
            out.update(flatten(value, prefix + (str(key),)))
    elif isinstance(doc, list) and doc:
        for index, value in enumerate(doc):
            out.update(flatten(value, prefix + (f"[{index}]",)))
    else:
        out[prefix] = doc
    return out


def path_allowed(path: Path_) -> bool:
    """⚠️ 完整路徑相等——⛔ 不用字串前綴（tamper c3 專門抓這種錯誤實作）。"""
    return path in ALLOWLIST


def differing_paths(before: Any, after: Any) -> List[Path_]:
    fb, fa = flatten(before), flatten(after)
    return sorted(p for p in set(fb) | set(fa) if fb.get(p, _MISSING) != fa.get(p, _MISSING)
                  or (p in fb) != (p in fa))


# ── G1、G2 ─────────────────────────────────────────────────────────────────

def validate_summary_set(doc: Any, side: str) -> None:
    if not isinstance(doc, dict) or set(doc) != SUMMARY_SET_KEYS:
        raise GuardInputError(f"{side} 輸出的鍵必須恰好是 {sorted(SUMMARY_SET_KEYS)}")
    if not isinstance(doc["loaded_modules"], dict) or not isinstance(doc["cases"], list):
        raise GuardInputError(f"{side} 輸出的 loaded_modules／cases 型別不符")
    for case in doc["cases"]:
        if not isinstance(case, dict) or set(case) != {"id", "summary"} or not isinstance(case["summary"], dict):
            raise GuardInputError(f"{side} 輸出的 case 必須恰好是 {{id, summary}}")


def check_g1(fixture_ids: List[str], fixture_sha: str, before: dict, after: dict) -> List[str]:
    reasons: List[str] = []
    if before["schema"] != SUMMARY_SET_SCHEMA or after["schema"] != SUMMARY_SET_SCHEMA:
        reasons.append(G1_SCHEMA)
    if before["fixture_sha256"] != fixture_sha or after["fixture_sha256"] != fixture_sha:
        reasons.append(G1_FIXTURE_SHA)
    if before["producer_sha256"] != after["producer_sha256"]:
        reasons.append(G1_PRODUCER_SHA)
    if before["python_version"] != after["python_version"]:
        reasons.append(G1_PYTHON_VERSION)
    if before["side"] != "patched" or after["side"] != "original":
        reasons.append(G1_SIDE)
    for doc in (before, after):
        if [c["id"] for c in doc["cases"]] != fixture_ids:
            reasons.append(G1_CASE_IDS)
            break
    return reasons


def git_blob_sha256(repo: Path, tree: str, rel: str) -> Optional[str]:
    proc = subprocess.run(["git", "-C", str(repo), "cat-file", "blob", f"{tree}:python/{rel}"],
                          stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
    if proc.returncode != 0:
        return None
    return sha256_bytes(proc.stdout)


def is_tooling_module(rel: str) -> bool:
    return rel == TOOLING_EVAL_MODULE or rel.startswith(TOOLING_DIR_PREFIX)


def check_g2(before: dict, after: dict,
             expected_sha: Callable[[str, str], Optional[str]]) -> Tuple[List[str], Dict[str, Any]]:
    """身分（各自 ＝ 自己 tree 的 git 物件）＋ 差異只能落在兩個產品檔與 tooling 路徑（D1）。"""
    reasons: List[str] = []
    mb, ma = before["loaded_modules"], after["loaded_modules"]
    for side, mods, reason in (("patched", mb, G2_IDENTITY_PATCHED), ("original", ma, G2_IDENTITY_ORIGINAL)):
        if any(expected_sha(side, rel) != sha for rel, sha in mods.items()):
            reasons.append(reason)
    diff = sorted(rel for rel in set(mb) | set(ma) if mb.get(rel) != ma.get(rel))
    outside = [rel for rel in diff if rel not in CF_PRODUCT_MODULES and not is_tooling_module(rel)]
    if outside:
        reasons.append(G2_DIFF_OUTSIDE_SCOPE)
    if any(rel not in mb or rel not in ma for rel in CF_PRODUCT_MODULES):
        reasons.append(G2_PRODUCT_NOT_LOADED)
    elif any(mb[rel] == ma[rel] for rel in CF_PRODUCT_MODULES):
        reasons.append(G2_PRODUCT_NOT_DIFFERENT)
    info = {"differing_modules": diff, "outside_scope": outside,
            "tooling_modules_loaded_patched": sorted(r for r in mb if is_tooling_module(r)),
            "tooling_modules_loaded_original": sorted(r for r in ma if is_tooling_module(r))}
    return reasons, info


# ── G3～G6（逐格）──────────────────────────────────────────────────────────

def _sp(summary: dict, key: str) -> Any:
    return get_path(summary, SP + (key,))


def _shape_ok(cell: str, pat: dict, orig: dict) -> bool:
    def both(fn: Callable[[dict], bool]) -> bool:
        return fn(pat) and fn(orig)

    pe = lambda s: _sp(s, "continuation_price_evidence_met") is True  # noqa: E731
    rr_bad = lambda s: _sp(s, "setup_rr_qualified") is False  # noqa: E731
    cand_false = lambda s: _sp(s, "rr_decoupling_candidate") is False  # noqa: E731
    lc = lambda s: _sp(s, "lifecycle_phase")  # noqa: E731
    structure = lambda s: get_path(s, ("market_regime", "structure_state"))  # noqa: E731
    avoid = lambda s: _sp(s, "action_state") == "AVOID"  # noqa: E731
    if cell in TARGET_CELLS:
        want_lc = "TESTING" if cell in ("1", "3") else "CONFIRMED"
        want_avoid = cell in ("3", "4")
        return (_sp(orig, "rr_decoupling_candidate") is True and lc(orig) == "CONTINUATION"
                and _sp(pat, "rr_decoupling_candidate") is False and lc(pat) == want_lc
                and avoid(pat) is want_avoid and avoid(orig) is want_avoid)
    if cell == "5":
        return both(lambda s: lc(s) == "CONTINUATION" and _sp(s, "setup_rr_qualified") is True and cand_false(s))
    if cell == "6":
        return both(lambda s: pe(s) and rr_bad(s) and _sp(s, "event_signal") != "CLOSE_RECLAIM" and cand_false(s))
    if cell == "7":
        return both(lambda s: pe(s) and rr_bad(s) and bool(get_path(s, ("event_state_summary", "active_bearish_events")))
                    and get_path(s, ("event_state_summary", "active_bearish_events")) is not _MISSING
                    and structure(s) not in ("SUPPORT_RECLAIM_INVALIDATED", "BREAKDOWN")
                    and lc(s) == "BREAKDOWN" and cand_false(s))
    if cell == "8":
        return both(lambda s: pe(s) and rr_bad(s) and structure(s) == "SUPPORT_RECLAIM_INVALIDATED"
                    and lc(s) == "INVALIDATED" and cand_false(s))
    if cell == "8b":
        return both(lambda s: _sp(s, "continuation_price_evidence_met") is False and rr_bad(s)
                    and structure(s) == "SUPPORT_RECLAIM_INVALIDATED" and lc(s) == "INVALIDATED" and cand_false(s))
    if cell == "9":
        return both(lambda s: pe(s) and rr_bad(s) and structure(s) == "BREAKDOWN" and lc(s) == "BREAKDOWN"
                    and cand_false(s))
    return False


def _without_target(codes: Any) -> Any:
    return [c for c in codes if c != TARGET_CODE] if isinstance(codes, list) else codes


def _authority(view: dict) -> Any:
    parts = [view.get(name) for name in AUTHORITY_PARTS]
    if not all(isinstance(p, list) for p in parts):
        return _MISSING
    out: List[Any] = []
    for part in parts:
        for code in part:
            if code not in out:
                out.append(code)
    return out


def _check_target_paths(pat: dict, orig: dict, avoid: bool) -> List[str]:
    """目標格：每一條不同的路徑都在 allowlist 內，而且值符合逐欄規則（before ＝ patched、after ＝ 原始）。"""
    reasons: List[str] = []
    for path in UPSTREAM:
        if get_path(pat, path) != get_path(orig, path):
            reasons.append(G6_UPSTREAM)
            break
    diffs = differing_paths(pat, orig)
    if any(not path_allowed(p) for p in diffs):
        reasons.append(G6_PATH_NOT_ALLOWED)

    def changed(path: Path_) -> bool:
        return get_path(pat, path) != get_path(orig, path)

    def transition(path: Path_, before_ok: Callable[[Any], bool], after_value: Any) -> None:
        if not (before_ok(get_path(pat, path)) and get_path(orig, path) == after_value):
            reasons.append(G6_UNEXPECTED_VALUE)

    def unchanged(path: Path_) -> None:
        if changed(path):
            reasons.append(G6_UNEXPECTED_VALUE)

    transition(SP + ("lifecycle_phase",), lambda v: v in ("TESTING", "CONFIRMED"), "CONTINUATION")
    transition(SP + ("market_state",), lambda v: v == "BULLISH_RECOVERY", "BULLISH_CONTINUATION")
    # rr_decoupling_candidate：false → true（兩欄相同）
    if not (_sp(pat, "rr_decoupling_candidate") is False and _sp(orig, "rr_decoupling_candidate") is True):
        reasons.append(G6_CANDIDATE)
    # semantic reason_codes：移除目標 code 之後逐項相同（含順序）；before 無、after 有
    rb, ra = _sp(pat, "reason_codes"), _sp(orig, "reason_codes")
    if not (isinstance(rb, list) and isinstance(ra, list)) or TARGET_CODE in rb or TARGET_CODE not in ra:
        reasons.append(G6_REASON_CODES)
    elif _without_target(rb) != _without_target(ra):
        same_members = sorted(map(str, _without_target(rb))) == sorted(map(str, _without_target(ra)))
        reasons.append(G6_REASON_CODES_ORDER if same_members else G6_REASON_CODES)
    if avoid:
        for path in (SP + ("action_state",), SP + ("bias_state",), DDV + ("bias_state",), DDV + ("bias_label",),
                     DDV + ("bias_reason_codes",), DDV + ("authority_reason_codes",), DDV + ("position_gate_state",),
                     ("market_bias",), ("market_bias_label",), ("position_action_condition", "state")):
            unchanged(path)
        # ⚠️ 「兩側相同」之外還要驗值域（⑨ 實作第一輪 review）：兩側一起被改成相同的錯值也要擋下。
        for side in (pat, orig):
            if _sp(side, "action_state") != "AVOID":
                reasons.append(G6_UNEXPECTED_VALUE)
            for path in (SP + ("bias_state",), DDV + ("bias_state",), ("market_bias",)):
                if get_path(side, path) != "BEARISH_BIAS":
                    reasons.append(G6_UNEXPECTED_VALUE)
            for path in (DDV + ("bias_label",), ("market_bias_label",)):
                if get_path(side, path) != BIAS_LABEL["BEARISH_BIAS"]:
                    reasons.append(G6_UNEXPECTED_VALUE)
    else:
        transition(SP + ("action_state",), lambda v: v in ("CONDITIONAL_HOLD", "HOLD"), "HOLD")
        for path in (SP + ("bias_state",), DDV + ("bias_state",), ("market_bias",)):
            transition(path, lambda v: v == "BULLISH_BIAS", "BULLISH_CONTINUATION")
        for path in (DDV + ("bias_label",), ("market_bias_label",)):
            transition(path, lambda v: v == BIAS_LABEL["BULLISH_BIAS"], BIAS_LABEL["BULLISH_CONTINUATION"])
        bb, ba = get_path(pat, DDV + ("bias_reason_codes",)), get_path(orig, DDV + ("bias_reason_codes",))
        if not (isinstance(bb, list) and isinstance(ba, list) and "SEMANTIC_BULLISH_RECOVERY" in bb
                and "SEMANTIC_BULLISH_CONTINUATION" in ba
                and [("SEMANTIC_BULLISH_CONTINUATION" if c == "SEMANTIC_BULLISH_RECOVERY" else c) for c in bb] == ba):
            reasons.append(G6_UNEXPECTED_VALUE)
    # position_gate_state、position_action_condition.state：各自 ＝ 該側的 semantic action_state（AVOID 與否都一樣）
    for side in (pat, orig):
        action = _sp(side, "action_state")
        if (get_path(side, DDV + ("position_gate_state",)) != action
                or get_path(side, ("position_action_condition", "state")) != action):
            reasons.append(G6_UNEXPECTED_VALUE)
    # authority_reason_codes：兩側各自重算 ＝ 實際值；四個其他來源兩側相同（只剩 bias 那一項的差異）
    vb, va = get_path(pat, DDV), get_path(orig, DDV)
    if not (isinstance(vb, dict) and isinstance(va, dict)):
        reasons.append(G6_AUTHORITY)
    else:
        if _authority(vb) != vb.get("authority_reason_codes") or _authority(va) != va.get("authority_reason_codes"):
            reasons.append(G6_AUTHORITY)
        if any(vb.get(name) != va.get(name) for name in AUTHORITY_PARTS[1:]):
            reasons.append(G6_AUTHORITY)
    return sorted(set(reasons))


def compare_cases(fixture: dict, before: dict, after: dict) -> List[Dict[str, Any]]:
    by_id_b = {c["id"]: c["summary"] for c in before["cases"]}
    by_id_a = {c["id"]: c["summary"] for c in after["cases"]}
    results = []
    for case in fixture["cases"]:
        cid, cell = case["id"], case["cell"]
        pat, orig = by_id_b.get(cid), by_id_a.get(cid)
        reasons: List[str] = []
        if pat is None or orig is None:
            results.append({"id": cid, "cell": cell, "reasons": [G1_CASE_IDS], "differing_paths": []})
            continue
        if not _shape_ok(cell, pat, orig):
            reasons.append(G3_SHAPE)
        observed_target = _sp(orig, "rr_decoupling_candidate") is True     # ⚠️ 只看**原始側**
        if observed_target != (case["class"] == "target"):
            reasons.append(G4_CLASS)
        diffs = differing_paths(pat, orig)
        if observed_target:
            avoid = _sp(orig, "action_state") == "AVOID"
            reasons.extend(_check_target_paths(pat, orig, avoid))
        elif diffs:
            reasons.append(G5_CONTROL_DIFFERS)
        results.append({"id": cid, "cell": cell, "class": case["class"], "reasons": sorted(set(reasons)),
                        "differing_paths": [render(p) for p in diffs][:50]})
    return results


def compare(fixture: dict, fixture_sha: str, before: dict, after: dict,
            expected_sha: Callable[[str, str], Optional[str]]) -> Dict[str, Any]:
    validate_summary_set(before, "--before-patched")
    validate_summary_set(after, "--after-original")
    g1 = check_g1([c["id"] for c in fixture["cases"]], fixture_sha, before, after)
    g2, g2_info = check_g2(before, after, expected_sha)
    cases = compare_cases(fixture, before, after)
    ok = not g1 and not g2 and all(not c["reasons"] for c in cases)
    return {"ok": ok, "document_reasons": sorted(set(g1 + g2)), "modules": g2_info, "cases": cases}


def all_reasons(result: Dict[str, Any]) -> set:
    out = set(result["document_reasons"])
    for case in result["cases"]:
        out.update(case["reasons"])
    return out


# ── tamper（⑨ 實跑以真實輸出為基底；pytest 以合成資料各測一次）────────────────

def _first_target(doc: dict, fixture: dict, avoid: Optional[bool] = None) -> dict:
    for case, item in zip(fixture["cases"], doc["cases"]):
        if case["class"] != "target":
            continue
        if avoid is None or (case["cell"] in ("3", "4")) is avoid:
            return item["summary"]
    raise GuardInputError("找不到目標格")


def _set(doc: dict, path: Path_, value: Any) -> None:
    cur = doc
    for key in path[:-1]:
        cur = cur[key]
    cur[path[-1]] = value


def _swap_two_remaining_codes(codes: List[str]) -> List[str]:
    """e2：交換兩個**都不是**目標 code、而且彼此不同的 code（第二輪 review）；前提不成立即拒絕。"""
    idx = [i for i, c in enumerate(codes) if c != TARGET_CODE]
    for a in range(len(idx)):
        for b in range(a + 1, len(idx)):
            i, j = idx[a], idx[b]
            if codes[i] != codes[j]:
                out = list(codes)
                out[i], out[j] = out[j], out[i]
                return out
    raise GuardInputError("e2 的前提不成立：移除目標 code 之後剩不到兩個不同的 code")


def tamper_variants(fixture: dict, before: dict, after: dict) -> Dict[str, Tuple[dict, dict, set]]:
    """回傳 名稱 → (tampered before, tampered after, 必須出現的原因碼集合)。"""
    out: Dict[str, Tuple[dict, dict, set]] = {}
    out["a1_swap_documents"] = (copy.deepcopy(after), copy.deepcopy(before), {G2_IDENTITY_PATCHED, G2_IDENTITY_ORIGINAL})

    b2, a2 = copy.deepcopy(before), copy.deepcopy(after)
    b2["cases"], a2["cases"] = a2["cases"], b2["cases"]
    out["a2_swap_cases"] = (b2, a2, {G3_SHAPE, G4_CLASS})

    # a3：lifecycle 與 candidate 不動（G3／G4 照樣通過），只把三條 allowlist 欄位兩側的值對調 → 逐欄規則必須擋下。
    b3, a3 = copy.deepcopy(before), copy.deepcopy(after)
    pt, ot = _first_target(b3, fixture, avoid=False), _first_target(a3, fixture, avoid=False)
    for path in (SP + ("market_state",), SP + ("bias_state",), DDV + ("bias_reason_codes",)):
        vp, vo = copy.deepcopy(get_path(pt, path)), copy.deepcopy(get_path(ot, path))
        _set(pt, path, vo)
        _set(ot, path, vp)
    out["a3_swap_allowlisted_values"] = (b3, a3, {G6_UNEXPECTED_VALUE})

    bb = copy.deepcopy(before)
    _set(_first_target(bb, fixture), SP + ("lifecycle_phase",), "BREAKDOWN")
    out["b_unexpected_lifecycle"] = (bb, copy.deepcopy(after), {G6_UNEXPECTED_VALUE})

    bc1 = copy.deepcopy(before)
    _set(_first_target(bc1, fixture), ("market_regime", "structure_state"), "TAMPERED_STRUCTURE")
    out["c1_upstream_structure"] = (bc1, copy.deepcopy(after), {G6_UPSTREAM})

    bc2 = copy.deepcopy(before)
    target = _first_target(bc2, fixture)
    target["data_mode"] = "TAMPERED_UNLISTED"
    out["c2_unlisted_field"] = (bc2, copy.deepcopy(after), {G6_PATH_NOT_ALLOWED})

    bc3 = copy.deepcopy(before)
    target = _first_target(bc3, fixture)
    shadow = ("market_bias_label_shadow",)
    if not (any(render(shadow).startswith(render(p)) for p in ALLOWLIST) and shadow not in ALLOWLIST):
        raise GuardInputError("c3 的前提不成立：shadow 路徑必須以某條 allowlist 路徑為字串前綴、且⛔ 不在 allowlist")
    target[shadow[0]] = "TAMPERED_PREFIX"
    out["c3_prefix_collision"] = (bc3, copy.deepcopy(after), {G6_PATH_NOT_ALLOWED})

    bd = copy.deepcopy(before)
    target = _first_target(bd, fixture)
    pac = target.get("position_action_condition")
    if not isinstance(pac, dict):
        raise GuardInputError("目標格沒有 position_action_condition")
    pac["reason_codes"] = list(pac.get("reason_codes") or []) + ["TAMPERED_POSITION_CODE"]
    out["d_position_reason_codes"] = (bd, copy.deepcopy(after), {G6_PATH_NOT_ALLOWED})

    be1 = copy.deepcopy(before)
    target = _first_target(be1, fixture)
    _set(target, SP + ("reason_codes",), list(_sp(target, "reason_codes")) + ["TAMPERED_EXTRA_CODE"])
    out["e1_extra_reason_code"] = (be1, copy.deepcopy(after), {G6_REASON_CODES})

    be2 = copy.deepcopy(before)
    target = _first_target(be2, fixture)
    swapped = _swap_two_remaining_codes(list(_sp(target, "reason_codes")))
    orig_codes = _sp(_first_target(after, fixture), "reason_codes")
    if not (sorted(_without_target(swapped)) == sorted(_without_target(orig_codes))
            and _without_target(swapped) != _without_target(orig_codes)):
        raise GuardInputError("e2 的前提不成立：移除目標 code 之後必須成員相同、順序不同")
    _set(target, SP + ("reason_codes",), swapped)
    out["e2_reason_code_order"] = (be2, copy.deepcopy(after), {G6_REASON_CODES_ORDER})

    bf = copy.deepcopy(before)
    _set(_first_target(bf, fixture), SP + ("rr_decoupling_candidate",), True)
    out["f_candidate_not_false_to_true"] = (bf, copy.deepcopy(after), {G3_SHAPE, G6_CANDIDATE})
    return out


def run_tampers(fixture: dict, fixture_sha: str, before: dict, after: dict,
                expected_sha: Callable[[str, str], Optional[str]]) -> List[Dict[str, Any]]:
    results = []
    for name, (tb, ta, required) in tamper_variants(fixture, before, after).items():
        result = compare(fixture, fixture_sha, tb, ta, expected_sha)
        got = all_reasons(result)
        results.append({"tamper": name, "failed": not result["ok"], "required_reasons": sorted(required),
                        "missing_reasons": sorted(required - got), "reasons": sorted(got)})
    return results


# ── CLI ────────────────────────────────────────────────────────────────────

def canonical_dump(obj: Any) -> str:
    return json.dumps(obj, ensure_ascii=False, sort_keys=True, separators=(",", ":"), allow_nan=False) + "\n"


def main(argv: Optional[List[str]] = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0], allow_abbrev=False)
    parser.add_argument("--fixture", required=True, type=Path)
    parser.add_argument("--before-patched", required=True, type=Path)
    parser.add_argument("--after-original", required=True, type=Path)
    parser.add_argument("--repo", required=True, type=Path, help="以 git 物件重算預期模組 SHA 的 repo")
    parser.add_argument("--patched-tree", required=True)
    parser.add_argument("--original-tree", required=True)
    parser.add_argument("--self-test-tampers", action="store_true")
    parser.add_argument("--out", required=True, type=Path)
    args = parser.parse_args(argv)
    for name in ("patched_tree", "original_tree"):
        if not re.fullmatch(r"[0-9a-f]{40}", getattr(args, name)):
            print(f"ERROR: --{name.replace('_', '-')} 必須是 40 碼 tree OID", file=sys.stderr)
            return 1
    try:
        fixture, fixture_raw = load_json_strict(args.fixture)
        validate_fixture(fixture)
        before, _ = load_json_strict(args.before_patched)
        after, _ = load_json_strict(args.after_original)
        trees = {"patched": args.patched_tree, "original": args.original_tree}
        cache: Dict[Tuple[str, str], Optional[str]] = {}

        def expected_sha(side: str, rel: str) -> Optional[str]:
            key = (side, rel)
            if key not in cache:
                cache[key] = git_blob_sha256(args.repo, trees[side], rel)
            return cache[key]

        fixture_sha = sha256_bytes(fixture_raw)
        result = compare(fixture, fixture_sha, before, after, expected_sha)
        report: Dict[str, Any] = {"schema": REPORT_SCHEMA, "fixture_sha256": fixture_sha, "result": result}
        ok = result["ok"]
        if args.self_test_tampers:
            tampers = run_tampers(fixture, fixture_sha, before, after, expected_sha)
            report["tampers"] = tampers
            ok = ok and all(t["failed"] and not t["missing_reasons"] for t in tampers)
        report["ok"] = ok
    except GuardInputError as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 1
    args.out.write_text(canonical_dump(report), encoding="utf-8")
    if not ok:
        print("ERROR: differential guard 不通過（見 " + str(args.out) + "）", file=sys.stderr)
        return 1
    print("OK: differential guard 通過")
    return 0


if __name__ == "__main__":
    sys.exit(main())
