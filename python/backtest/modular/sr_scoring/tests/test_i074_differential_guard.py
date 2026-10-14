"""I-074 Stage 2 步驟 ⑨：differential guard 的 comparator（`python/scripts/i074_differential_guard.py`）。

以**合成**的 decision summary 對組出九格 ＋ 8b（真實輸出的驗證在 ⑨ 的實跑；見 docs/issue.md「Stage 2 步驟 ⑨ 細部計畫 v1」
「八」#1～#3）。每一支 tamper 都斷言**它專屬的原因碼**，⛔ 不只看 fail。
"""
from __future__ import annotations

import copy
import importlib.util
import json
import sys
from pathlib import Path

import pytest

_SCRIPTS = Path(__file__).resolve().parents[4] / "scripts"
FIXTURE_PATH = _SCRIPTS / "fixtures" / "i074_differential_guard_cases.json"


def _load(name: str):
    spec = importlib.util.spec_from_file_location(name, _SCRIPTS / f"{name}.py")
    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module


dg = _load("i074_differential_guard")
FIXTURE = json.loads(FIXTURE_PATH.read_text(encoding="utf-8"))
FIXTURE_SHA = dg.sha256_bytes(FIXTURE_PATH.read_bytes())

PATCHED_MODULES = {
    "backtest/modular/sr_scoring/decision_engine.py": "p1",
    "backtest/modular/sr_scoring/lifecycle_engine.py": "p2",
    "backtest/modular/sr_scoring/evaluation.py": "pe",
    "backtest/modular/sr_scoring/replay_bundle/__init__.py": "pr",
    "backtest/modular/sr_scoring/replay_bundle/stream.py": "ps",
    "backtest/modular/sr_scoring/types.py": "t",
}
ORIGINAL_MODULES = {
    "backtest/modular/sr_scoring/decision_engine.py": "o1",
    "backtest/modular/sr_scoring/lifecycle_engine.py": "o2",
    "backtest/modular/sr_scoring/evaluation.py": "oe",
    "backtest/modular/sr_scoring/replay_bundle/__init__.py": "or",
    "backtest/modular/sr_scoring/types.py": "t",
}


def _expected_from(patched: dict, original: dict):
    def expected(side, rel):
        return (patched if side == "patched" else original).get(rel)
    return expected


EXPECTED = _expected_from(PATCHED_MODULES, ORIGINAL_MODULES)

CONTROL_SHAPES = {
    "5": dict(lc="CONTINUATION", event="CLOSE_RECLAIM", rr=True, pe=True, structure="SUPPORT_RECLAIM_CANDIDATE"),
    "6": dict(lc="TESTING", event="SUPPORT_TEST", rr=False, pe=True, structure="NORMAL"),
    "7": dict(lc="BREAKDOWN", event="CLOSE_BREAKDOWN", rr=False, pe=True, structure="NORMAL", bearish=True),
    "8": dict(lc="INVALIDATED", event="NO_EVENT", rr=False, pe=True, structure="SUPPORT_RECLAIM_INVALIDATED"),
    "8b": dict(lc="INVALIDATED", event="NO_EVENT", rr=False, pe=False, structure="SUPPORT_RECLAIM_INVALIDATED"),
    "9": dict(lc="BREAKDOWN", event="CLOSE_RECLAIM", rr=False, pe=True, structure="BREAKDOWN"),
}


def _summary(*, lc, event, rr, pe, structure, cand=False, action="WATCH", market_state="NORMAL", bias="NEUTRAL_BIAS",
             codes=("X",), bias_codes=("B",), bearish=False):
    view = {
        "semantic_pipeline": {
            "lifecycle_phase": lc, "market_state": market_state, "action_state": action, "bias_state": bias,
            "reason_codes": list(codes), "rr_decoupling_candidate": cand, "setup_rr_qualified": rr,
            "event_signal": event, "continuation_price_evidence_met": pe, "entry_permission_state": "BLOCKED",
        },
        "bias_state": bias, "bias_label": dg.BIAS_LABEL.get(bias, bias), "bias_reason_codes": list(bias_codes),
        "daily_reason_codes": ["DAILY_X"], "final_entry_reason_codes": ["FINAL_X"], "path_reason_codes": [],
        "position_reason_codes": ["POS_X"], "position_gate_state": action,
    }
    view["authority_reason_codes"] = dg._authority(view)
    return {
        "decision_derived_view": view, "market_bias": bias, "market_bias_label": dg.BIAS_LABEL.get(bias, bias),
        "position_action_condition": {"state": action, "reason_codes": ["PAC_X"], "structure_state": structure},
        "market_regime": {"structure_state": structure},
        "event_state_summary": {"active_bearish_events": [{"type": "HIGH_VOLUME_BREAKDOWN"}] if bearish else [],
                                "active": []},
        "primary_zone": {"label": "98.00 ~ 100.00"}, "rr_gate": {"qualified": False, "reason_code": "RR_INSUFFICIENT"},
        "data_mode": "END_OF_DAY",
    }


def _target(cell, side):
    avoid = cell in ("3", "4")
    structure = "SUPPORT_RECLAIM_CANDIDATE" if cell in ("1", "3") else "SUPPORT_RECLAIM_CONFIRMED"
    tail = "MARKET_ACTION_AVOID" if avoid else "RR_INSUFFICIENT"
    common = dict(event="CLOSE_RECLAIM", rr=False, pe=True, structure=structure)
    if side == "original":
        return _summary(lc="CONTINUATION", cand=True, market_state="BULLISH_CONTINUATION",
                        action="AVOID" if avoid else "HOLD", bias="BEARISH_BIAS" if avoid else "BULLISH_CONTINUATION",
                        codes=("CLOSE_RECLAIM", dg.TARGET_CODE, tail),
                        bias_codes=("MARKET_ACTION_AVOID",) if avoid else ("SEMANTIC_BULLISH_CONTINUATION",), **common)
    lc = "TESTING" if cell in ("1", "3") else "CONFIRMED"
    action = "AVOID" if avoid else ("CONDITIONAL_HOLD" if lc == "TESTING" else "HOLD")
    return _summary(lc=lc, cand=False, market_state="BULLISH_RECOVERY", action=action,
                    bias="BEARISH_BIAS" if avoid else "BULLISH_BIAS", codes=("CLOSE_RECLAIM", tail),
                    bias_codes=("MARKET_ACTION_AVOID",) if avoid else ("SEMANTIC_BULLISH_RECOVERY",), **common)


def _doc(side):
    cases = []
    for case in FIXTURE["cases"]:
        cell = case["cell"]
        summary = _target(cell, side) if cell in dg.TARGET_CELLS else _summary(**CONTROL_SHAPES[cell])
        cases.append({"id": case["id"], "summary": summary})
    return {"schema": dg.SUMMARY_SET_SCHEMA, "side": side, "fixture_sha256": FIXTURE_SHA, "producer_sha256": "f" * 64,
            "python_version": "3.11.17",
            "loaded_modules": dict(PATCHED_MODULES if side == "patched" else ORIGINAL_MODULES), "cases": cases}


def _compare(before, after, expected=EXPECTED, fixture=FIXTURE):
    return dg.compare(fixture, FIXTURE_SHA, before, after, expected)


def _case(doc, cell):
    for case, item in zip(FIXTURE["cases"], doc["cases"]):
        if case["cell"] == cell:
            return item["summary"]
    raise KeyError(cell)


# ── 正向：九格 ＋ 8b ───────────────────────────────────────────────────────

def test_the_real_fixture_is_valid():
    dg.validate_fixture(FIXTURE)
    assert [c["cell"] for c in FIXTURE["cases"]] == list(dg.CELLS)


def test_positive_matrix_all_ten_cells_pass():
    result = _compare(_doc("patched"), _doc("original"))
    assert result["ok"], result
    assert [c["cell"] for c in result["cases"]] == list(dg.CELLS)
    assert all(c["reasons"] == [] for c in result["cases"])
    # 目標格的差異只在 allowlist 內；對照格完全相同。
    for case in result["cases"]:
        if case["cell"] in dg.TARGET_CELLS:
            assert case["differing_paths"]
        else:
            assert case["differing_paths"] == []


def test_tooling_modules_may_differ_but_are_reported():
    result = _compare(_doc("patched"), _doc("original"))
    assert "backtest/modular/sr_scoring/replay_bundle/stream.py" in result["modules"]["differing_modules"]
    assert result["modules"]["outside_scope"] == []


# ── tamper：每一類都以它專屬的原因碼失敗 ───────────────────────────────────

TAMPERS = sorted(dg.tamper_variants(FIXTURE, _doc("patched"), _doc("original")))


@pytest.mark.parametrize("name", TAMPERS)
def test_every_tamper_fails_with_its_own_reason(name):
    before, after, required = dg.tamper_variants(FIXTURE, _doc("patched"), _doc("original"))[name]
    result = _compare(before, after)
    assert not result["ok"]
    assert required <= dg.all_reasons(result), (name, dg.all_reasons(result))


def test_tamper_set_covers_the_six_v29_classes_split_as_planned():
    assert TAMPERS == sorted([
        "a1_swap_documents", "a2_swap_cases", "a3_swap_allowlisted_values", "b_unexpected_lifecycle",
        "c1_upstream_structure", "c2_unlisted_field", "c3_prefix_collision", "d_position_reason_codes",
        "e1_extra_reason_code", "e2_reason_code_order", "f_candidate_not_false_to_true",
    ])


def test_run_tampers_reports_every_tamper_as_failed_with_no_missing_reason():
    tampers = dg.run_tampers(FIXTURE, FIXTURE_SHA, _doc("patched"), _doc("original"), EXPECTED)
    assert tampers and all(t["failed"] and t["missing_reasons"] == [] for t in tampers)


def test_e2_swaps_only_codes_that_survive_removing_the_target_code():
    swapped = dg._swap_two_remaining_codes(["CLOSE_RECLAIM", dg.TARGET_CODE, "RR_INSUFFICIENT"])
    assert swapped == ["RR_INSUFFICIENT", dg.TARGET_CODE, "CLOSE_RECLAIM"]
    with pytest.raises(dg.GuardInputError):
        dg._swap_two_remaining_codes(["CLOSE_RECLAIM", dg.TARGET_CODE])


def test_reason_codes_set_equal_but_reordered_is_an_order_failure():
    before, after = _doc("patched"), _doc("original")
    _case(after, "1")["decision_derived_view"]["semantic_pipeline"]["reason_codes"] = [
        "RR_INSUFFICIENT", dg.TARGET_CODE, "CLOSE_RECLAIM"]
    assert dg.G6_REASON_CODES_ORDER in dg.all_reasons(_compare(before, after))


# ── allowlist ───────────────────────────────────────────────────────────────

def test_allowlist_is_exactly_the_v29_table():
    assert [dg.render(p) for p in dg.ALLOWLIST] == [
        "decision_derived_view.semantic_pipeline.lifecycle_phase",
        "decision_derived_view.semantic_pipeline.market_state",
        "decision_derived_view.semantic_pipeline.action_state",
        "decision_derived_view.semantic_pipeline.bias_state",
        "decision_derived_view.semantic_pipeline.reason_codes",
        "decision_derived_view.semantic_pipeline.rr_decoupling_candidate",
        "decision_derived_view.bias_state",
        "decision_derived_view.bias_label",
        "decision_derived_view.bias_reason_codes",
        "decision_derived_view.authority_reason_codes",
        "decision_derived_view.position_gate_state",
        "market_bias",
        "market_bias_label",
        "position_action_condition.state",
    ]
    assert ("position_action_condition", "reason_codes") not in dg.ALLOWLIST


def test_path_allowed_is_exact_equality_not_a_prefix():
    assert dg.path_allowed(("market_bias_label",))
    assert not dg.path_allowed(("market_bias_label_shadow",))
    assert not dg.path_allowed(("position_action_condition",))
    assert not dg.path_allowed(("position_action_condition", "reason_codes", "[0]"))


def test_authority_may_differ_only_through_bias_reason_codes():
    before, after = _doc("patched"), _doc("original")
    view = _case(before, "1")["decision_derived_view"]
    view["daily_reason_codes"] = ["DAILY_TAMPERED"]
    view["authority_reason_codes"] = dg._authority(view)
    reasons = dg.all_reasons(_compare(before, after))
    assert dg.G6_AUTHORITY in reasons and dg.G6_PATH_NOT_ALLOWED in reasons


# ── G1 ─────────────────────────────────────────────────────────────────────

@pytest.mark.parametrize("field,value,reason", [
    ("fixture_sha256", "0" * 64, "G1_FIXTURE_SHA"),
    ("producer_sha256", "e" * 64, "G1_PRODUCER_SHA"),
    ("python_version", "3.11.0", "G1_PYTHON_VERSION"),
    ("side", "original", "G1_SIDE"),
    ("schema", "other", "G1_SCHEMA"),
])
def test_g1_document_identity(field, value, reason):
    before = _doc("patched")
    before[field] = value
    assert reason in _compare(before, _doc("original"))["document_reasons"]


@pytest.mark.parametrize("mutate", ["reorder", "drop", "duplicate"])
def test_g1_case_ids_must_match_the_fixture(mutate):
    before = _doc("patched")
    if mutate == "reorder":
        before["cases"][0], before["cases"][1] = before["cases"][1], before["cases"][0]
    elif mutate == "drop":
        before["cases"].pop()
    else:
        before["cases"].append(copy.deepcopy(before["cases"][0]))
    assert dg.G1_CASE_IDS in dg.all_reasons(_compare(before, _doc("original")))


def test_malformed_summary_set_is_an_input_error():
    before = _doc("patched")
    before["extra"] = 1
    with pytest.raises(dg.GuardInputError):
        _compare(before, _doc("original"))


# ── G2 ─────────────────────────────────────────────────────────────────────

def test_g2_identity_mismatch_on_each_side():
    patched = dict(PATCHED_MODULES, **{"backtest/modular/sr_scoring/decision_engine.py": "zz"})
    reasons = _compare(_doc("patched"), _doc("original"), expected=_expected_from(patched, ORIGINAL_MODULES))
    assert dg.G2_IDENTITY_PATCHED in reasons["document_reasons"]
    original = dict(ORIGINAL_MODULES, **{"backtest/modular/sr_scoring/types.py": "zz"})
    reasons = _compare(_doc("patched"), _doc("original"), expected=_expected_from(PATCHED_MODULES, original))
    assert dg.G2_IDENTITY_ORIGINAL in reasons["document_reasons"]


def test_g2_difference_outside_products_and_tooling():
    before = _doc("patched")
    before["loaded_modules"]["backtest/modular/sr_scoring/types.py"] = "t2"
    patched = dict(PATCHED_MODULES, **{"backtest/modular/sr_scoring/types.py": "t2"})
    result = _compare(before, _doc("original"), expected=_expected_from(patched, ORIGINAL_MODULES))
    assert dg.G2_DIFF_OUTSIDE_SCOPE in result["document_reasons"]


def test_g2_non_tooling_module_loaded_on_one_side_only():
    before = _doc("patched")
    before["loaded_modules"]["backtest/modular/sr_scoring/labels.py"] = "l"
    patched = dict(PATCHED_MODULES, **{"backtest/modular/sr_scoring/labels.py": "l"})
    result = _compare(before, _doc("original"), expected=_expected_from(patched, ORIGINAL_MODULES))
    assert dg.G2_DIFF_OUTSIDE_SCOPE in result["document_reasons"]


def test_g2_product_files_must_be_loaded_and_different():
    before = _doc("patched")
    del before["loaded_modules"]["backtest/modular/sr_scoring/lifecycle_engine.py"]
    assert dg.G2_PRODUCT_NOT_LOADED in _compare(before, _doc("original"))["document_reasons"]
    before = _doc("patched")
    before["loaded_modules"]["backtest/modular/sr_scoring/lifecycle_engine.py"] = "o2"
    patched = dict(PATCHED_MODULES, **{"backtest/modular/sr_scoring/lifecycle_engine.py": "o2"})
    result = _compare(before, _doc("original"), expected=_expected_from(patched, ORIGINAL_MODULES))
    assert dg.G2_PRODUCT_NOT_DIFFERENT in result["document_reasons"]


# ── G3／G4：形狀自證與分類 ──────────────────────────────────────────────────

@pytest.mark.parametrize("cell", ["6", "7", "8", "9"])
def test_g3_controls_6_to_9_need_price_evidence_on_both_sides(cell):
    before, after = _doc("patched"), _doc("original")
    for doc in (before, after):
        _case(doc, cell)["decision_derived_view"]["semantic_pipeline"]["continuation_price_evidence_met"] = False
    reasons = {r for c in _compare(before, after)["cases"] if c["cell"] == cell for r in c["reasons"]}
    assert dg.G3_SHAPE in reasons


def test_g3_8b_must_not_have_price_evidence():
    before, after = _doc("patched"), _doc("original")
    for doc in (before, after):
        _case(doc, "8b")["decision_derived_view"]["semantic_pipeline"]["continuation_price_evidence_met"] = True
    reasons = {r for c in _compare(before, after)["cases"] if c["cell"] == "8b" for r in c["reasons"]}
    assert dg.G3_SHAPE in reasons


def test_g3_declared_target_lifecycle_must_match():
    before = _doc("patched")
    sp = _case(before, "1")["decision_derived_view"]["semantic_pipeline"]
    sp["lifecycle_phase"] = "CONFIRMED"
    reasons = {r for c in _compare(before, _doc("original"))["cases"] if c["cell"] == "1" for r in c["reasons"]}
    assert dg.G3_SHAPE in reasons


def test_g4_declared_target_that_is_not_a_candidate_on_the_original_side():
    before, after = _doc("patched"), _doc("original")
    _case(after, "2").update(copy.deepcopy(_case(before, "2")))
    reasons = {r for c in _compare(before, after)["cases"] if c["cell"] == "2" for r in c["reasons"]}
    assert {dg.G3_SHAPE, dg.G4_CLASS} <= reasons


def test_g5_a_control_must_be_identical():
    before = _doc("patched")
    _case(before, "5")["data_mode"] = "TAMPERED"
    reasons = {r for c in _compare(before, _doc("original"))["cases"] if c["cell"] == "5" for r in c["reasons"]}
    assert dg.G5_CONTROL_DIFFERS in reasons


# ⑨ 實作第一輪 review：AVOID 格的值域——「兩側相同」不夠，兩側一起改成相同的錯值也必須被擋下。
AVOID_SAME_WRONG = [
    (("decision_derived_view", "semantic_pipeline", "bias_state"), "NEUTRAL_BIAS"),
    (("decision_derived_view", "bias_state"), "NEUTRAL_BIAS"),
    (("market_bias",), "NEUTRAL_BIAS"),
    (("decision_derived_view", "bias_label"), "中性觀察"),
    (("market_bias_label",), "中性觀察"),
    (("decision_derived_view", "position_gate_state"), "HOLD"),
    (("position_action_condition", "state"), "HOLD"),
]


@pytest.mark.parametrize("path, wrong", AVOID_SAME_WRONG, ids=[dg.render(p) for p, _ in AVOID_SAME_WRONG])
@pytest.mark.parametrize("cell", ["3", "4"])
def test_avoid_cell_both_sides_changed_to_the_same_wrong_value(cell, path, wrong):
    before, after = _doc("patched"), _doc("original")
    for doc in (before, after):
        dg._set(_case(doc, cell), path, wrong)
    result = _compare(before, after)
    reasons = {r for c in result["cases"] if c["cell"] == cell for r in c["reasons"]}
    assert not result["ok"]
    assert dg.G6_UNEXPECTED_VALUE in reasons


# ── fixture 的驗證 ─────────────────────────────────────────────────────────

def _bad_fixture(mutate):
    fixture = copy.deepcopy(FIXTURE)
    mutate(fixture)
    return fixture


@pytest.mark.parametrize("mutate", [
    lambda f: f["cases"][7].update(outside_replay_domain=False),                    # 第 8 格沒有標註
    lambda f: f["cases"][7]["inputs"].update(current_price=97.0),                   # 第 8 格 current ＝ close
    lambda f: f["cases"][0].update(outside_replay_domain=True),                     # 其他格標註
    lambda f: f["cases"][0]["inputs"].update(current_price=103.0),                  # 其他格 current ≠ close
    lambda f: f["cases"].pop(8),                                                    # 缺 8b
    lambda f: f["cases"][1].update(id=f["cases"][0]["id"]),                         # id 重複
    lambda f: f["cases"][0].update(code="import os"),                               # 未知鍵
    lambda f: f["cases"][0]["inputs"].update(module="os"),                          # inputs 的未知鍵
    lambda f: f["cases"][4].update({"class": "target"}),                            # 類別錯
    lambda f: f.update(schema="other"),
])
def test_validate_fixture_rejects(mutate):
    with pytest.raises(dg.GuardInputError):
        dg.validate_fixture(_bad_fixture(mutate))


def test_load_json_strict_rejects_nan(tmp_path):
    path = tmp_path / "x.json"
    path.write_text('{"a": NaN}', encoding="utf-8")
    with pytest.raises(dg.GuardInputError):
        dg.load_json_strict(path)


def test_cli_rejects_non_tree_arguments(tmp_path, capsys):
    rc = dg.main(["--fixture", str(FIXTURE_PATH), "--before-patched", "x", "--after-original", "y",
                  "--repo", str(tmp_path), "--patched-tree", "HEAD", "--original-tree", "0" * 40,
                  "--out", str(tmp_path / "r.json")])
    assert rc == 1
    assert "40 碼" in capsys.readouterr().err
