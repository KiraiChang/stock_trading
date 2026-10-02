"""I-074 Stage 2 ⑦c：B／C 判讀規則（`replay_bundle/stage2_verdict.py`）。

對應 issue.md I-074「Stage 2 步驟 ⑦c 細部計畫 v1」「五」的「六、9」a～l 與「判讀輸出的 schema」：
四格 → B；每一項 C 條件各一支，⚠️ 每一支斷言**完整的** `reasons` 陣列（⛔ 不只斷言含某個 code）；
多項同時命中全部記下、字典序；`ROW_SHAPE_INVALID` 只有它一個；`UNCLASSIFIED` 不再記依分類的 code；
列順序 ＝ comparison；0 列拒絕判讀；固定 fixture 的 canonical bytes 與 golden 逐位元相同。
驗證模式 `--judge` 的整合（同一個程序、comparison 只讀一次）在 `test_replay_stage2_archive.py`。
"""
from __future__ import annotations

import copy

import pytest

from ..replay_bundle import stage2_verdict as v
from ..replay_bundle.artifacts import compare_rows
from ..replay_bundle.canonical import canonical_json_bytes, sha256_hex


def _side(lifecycle, bias, action, rr, *, final="BLOCKED", structure="SUPPORT_RECLAIM_CONFIRMED", **extra):
    side = {
        "symbol": "2330", "timeframe": "1d", "as_of": "2026-08-21",
        "lifecycle_phase": lifecycle, "market_bias": bias, "action_state": action,
        "position_action_condition": {"state": action, "structure_state": structure},
        "rr_decoupling_candidate": rr, "final_entry_state": final, "position_action": "HOLD",
        "structure_state": structure,
    }
    side.update(extra)
    return side


# 「關閉條件」的唯一 artifact 判讀矩陣：四格（before lifecycle × 非 AVOID／AVOID）
CELLS = {
    "a_testing_non_avoid": (_side("TESTING", "BULLISH_BIAS", "CONDITIONAL_HOLD", False),
                            _side("CONTINUATION", "BULLISH_CONTINUATION", "HOLD", True)),
    "b_confirmed_non_avoid": (_side("CONFIRMED", "BULLISH_BIAS", "HOLD", False),
                              _side("CONTINUATION", "BULLISH_CONTINUATION", "HOLD", True)),
    "c_testing_avoid": (_side("TESTING", "BEARISH_BIAS", "AVOID", False),
                        _side("CONTINUATION", "BEARISH_BIAS", "AVOID", True)),
    "d_confirmed_avoid": (_side("CONFIRMED", "BEARISH_BIAS", "AVOID", False),
                          _side("CONTINUATION", "BEARISH_BIAS", "AVOID", True)),
}


def _row(before, after, as_of="2026-08-21"):
    before = dict(before, as_of=as_of)
    after = dict(after, as_of=as_of)
    return compare_rows(before, after)


def _mutated(cell, side, **changes):
    before, after = copy.deepcopy(CELLS[cell])
    target = before if side == "before" else after
    for key, value in changes.items():
        if key == "condition_state":
            target["position_action_condition"]["state"] = value
        elif key == "condition_structure":
            target["position_action_condition"]["structure_state"] = value
        else:
            target[key] = value
    return _row(before, after)


@pytest.mark.parametrize("cell", sorted(CELLS))
def test_a_to_d_the_four_cells_are_b(cell):
    out = v.judge_row(_row(*CELLS[cell]))
    assert out == {"symbol": "2330", "timeframe": "1d", "as_of": "2026-08-21",
                   "class": "AVOID" if "avoid" in cell and "non_avoid" not in cell else "NON_AVOID",
                   "verdict": "B", "reasons": []}


@pytest.mark.parametrize("label,row,reasons", [
    # e：lifecycle 沒有翻轉（after 不是 CONTINUATION）
    ("e_lifecycle_not_flipped", lambda: _mutated("a_testing_non_avoid", "after", lifecycle_phase="TESTING"),
     ["LIFECYCLE_PHASE_UNEXPECTED"]),
    # f：非 AVOID 的 market_bias 沒翻
    ("f_non_avoid_bias_not_flipped", lambda: _mutated("b_confirmed_non_avoid", "after", market_bias="BULLISH_BIAS"),
     ["MARKET_BIAS_UNEXPECTED"]),
    # g：AVOID 的 market_bias 變了
    ("g_avoid_bias_changed", lambda: _mutated("c_testing_avoid", "after", market_bias="BEARISH_CONTINUATION"),
     ["MARKET_BIAS_UNEXPECTED"]),
    # h：final_entry_state 變了
    ("h_final_entry_changed", lambda: _mutated("d_confirmed_avoid", "after", final_entry_state="ALLOWED"),
     ["DIFF_FIELD_NOT_ALLOWED", "FINAL_ENTRY_STATE_NOT_BLOCKED"]),
    # i：action_state 與 position_action_condition.state 不一致
    ("i_condition_state_mismatch", lambda: _mutated("a_testing_non_avoid", "after", condition_state="CONDITIONAL_HOLD"),
     ["CONDITION_STATE_MISMATCH"]),
    # j：允許清單以外的差異
    ("j_field_outside_allowlist", lambda: _mutated("b_confirmed_non_avoid", "after", structure_state="OTHER"),
     ["DIFF_FIELD_NOT_ALLOWED"]),
    # k：top-level position_action 有變
    ("k_position_action_changed", lambda: _mutated("a_testing_non_avoid", "after", position_action="AVOID"),
     ["POSITION_ACTION_CHANGED"]),
    # 本包補的三件
    ("condition_non_state_field_differs", lambda: _mutated("a_testing_non_avoid", "after", condition_structure="X"),
     ["POSITION_ACTION_CONDITION_NON_STATE_DIFF"]),
    ("before_rr_flag_true", lambda: _mutated("a_testing_non_avoid", "before", rr_decoupling_candidate=True),
     ["RR_DECOUPLING_NOT_FALSE_TO_TRUE"]),
    ("after_action_unclassified", lambda: _mutated("a_testing_non_avoid", "after", action_state="CONDITIONAL_HOLD",
                                                   condition_state="CONDITIONAL_HOLD"),
     ["ACTION_STATE_UNCLASSIFIED"]),
    # 轉換錯格：TESTING 的非 AVOID 卻是 HOLD → HOLD
    ("action_transition_wrong_cell", lambda: _mutated("a_testing_non_avoid", "before", action_state="HOLD",
                                                      condition_state="HOLD"),
     ["ACTION_STATE_TRANSITION_UNEXPECTED"]),
    # before lifecycle 不在 TESTING／CONFIRMED：只記 LIFECYCLE_PHASE_UNEXPECTED、⛔ 不重複記轉換
    ("before_lifecycle_other", lambda: _mutated("b_confirmed_non_avoid", "before", lifecycle_phase="CONTINUATION"),
     ["LIFECYCLE_PHASE_UNEXPECTED"]),
])
def test_e_to_k_each_c_condition_has_the_complete_reasons(label, row, reasons):
    out = v.judge_row(row())
    assert (out["verdict"], out["reasons"]) == ("C", reasons), label


def test_k_identical_position_action_does_not_affect_b():
    before, after = copy.deepcopy(CELLS["c_testing_avoid"])
    before["position_action"] = after["position_action"] = "AVOID"
    assert v.judge_row(_row(before, after))["verdict"] == "B"


def test_multiple_violations_are_all_recorded_sorted():
    before, after = copy.deepcopy(CELLS["a_testing_non_avoid"])
    after.update(lifecycle_phase="TESTING", market_bias="BULLISH_BIAS", final_entry_state="ALLOWED", position_action="AVOID")
    before["rr_decoupling_candidate"] = True
    out = v.judge_row(_row(before, after))
    assert out["reasons"] == sorted({"LIFECYCLE_PHASE_UNEXPECTED", "MARKET_BIAS_UNEXPECTED", "FINAL_ENTRY_STATE_NOT_BLOCKED",
                                     "POSITION_ACTION_CHANGED", "DIFF_FIELD_NOT_ALLOWED", "RR_DECOUPLING_NOT_FALSE_TO_TRUE"})
    assert out["reasons"] == sorted(out["reasons"]) and len(set(out["reasons"])) == len(out["reasons"])


def test_unclassified_skips_the_class_dependent_codes():
    before, after = copy.deepcopy(CELLS["a_testing_non_avoid"])
    after.update(action_state="WATCH", market_bias="WHATEVER")
    after["position_action_condition"]["state"] = "WATCH"
    out = v.judge_row(_row(before, after))
    assert (out["class"], out["reasons"]) == ("UNCLASSIFIED", ["ACTION_STATE_UNCLASSIFIED"])


@pytest.mark.parametrize("field,value", [
    ("market_bias", None), ("final_entry_state", 1), ("lifecycle_phase", None),
    ("rr_decoupling_candidate", 0), ("rr_decoupling_candidate", None), ("position_action_condition", None),
    ("position_action_condition", {"structure_state": "x"}), ("action_state", None),
])
@pytest.mark.parametrize("side", ["before", "after"])
def test_row_shape_invalid_is_the_only_reason(field, value, side):
    """必備欄位與型別（`type(x) is …`：⛔ 不接受 null、⛔ 不讓 bool 冒充 int（或反過來））。"""
    before, after = copy.deepcopy(CELLS["b_confirmed_non_avoid"])
    (before if side == "before" else after)[field] = value
    row = {"symbol": "2330", "timeframe": "1d", "as_of": "2026-08-21", "differences": [], "before": before, "after": after}
    out = v.judge_row(row)
    assert (out["verdict"], out["reasons"]) == ("C", ["ROW_SHAPE_INVALID"])
    assert set(out) == set(v.ROW_FIELDS)


def test_missing_field_is_row_shape_invalid():
    before, after = copy.deepcopy(CELLS["b_confirmed_non_avoid"])
    del after["market_bias"]
    row = {"symbol": "2330", "timeframe": "1d", "as_of": "2026-08-21", "differences": [], "before": before, "after": after}
    assert v.judge_row(row)["reasons"] == ["ROW_SHAPE_INVALID"]


def _comparison(rows):
    return {"rows": rows}


def test_l_one_c_row_makes_the_whole_verdict_c_and_schema_is_closed():
    rows = [_row(*CELLS["a_testing_non_avoid"], as_of="2026-08-21"),
            _mutated("b_confirmed_non_avoid", "after", market_bias="BULLISH_BIAS"),
            _row(*CELLS["d_confirmed_avoid"], as_of="2026-08-25")]
    rows[1] = dict(rows[1], as_of="2026-08-22")
    out = v.judge_comparison(_comparison(rows))
    assert set(out) == set(v.TOP_FIELDS)
    assert (out["schema"], out["verdict"], out["row_count"], out["c_row_count"]) == ("i074_stage2_verdict/v1", "C", 3, 1)
    assert [r["as_of"] for r in out["rows"]] == ["2026-08-21", "2026-08-22", "2026-08-25"]   # ＝ comparison 的順序
    for r in out["rows"]:
        assert set(r) == set(v.ROW_FIELDS)
        assert r["class"] in ("NON_AVOID", "AVOID", "UNCLASSIFIED") and r["verdict"] in ("B", "C")
        assert (r["verdict"] == "B") == (r["reasons"] == [])
        assert set(r["reasons"]) <= v.REASON_CODES


def test_all_b_and_no_majority_vote():
    rows = [_row(*CELLS[c], as_of=f"2026-08-2{i}") for i, c in enumerate(sorted(CELLS))]
    assert v.judge_comparison(_comparison(rows))["verdict"] == "B"
    rows.append(dict(_mutated("a_testing_non_avoid", "after", position_action="AVOID"), as_of="2026-08-29"))
    out = v.judge_comparison(_comparison(rows))                 # 四 B 一 C → 整體 C（⛔ 不以多數決）
    assert (out["verdict"], out["c_row_count"]) == ("C", 1)


@pytest.mark.parametrize("rows", [[], None, "x"])
def test_zero_rows_are_refused_not_judged_b(rows):
    with pytest.raises(v.VerdictError):
        v.judge_comparison({"rows": rows})


def test_rows_are_never_reordered():
    rows = [_row(*CELLS["d_confirmed_avoid"], as_of="2026-08-25"), _row(*CELLS["a_testing_non_avoid"], as_of="2026-08-21")]
    assert [r["as_of"] for r in v.judge_comparison(_comparison(rows))["rows"]] == ["2026-08-25", "2026-08-21"]


# ⚠️ golden：規則在看到結果之前就寫死——固定 fixture 的 canonical bytes 一改就紅。
GOLDEN_SHA256 = "2208032cf691a00394fc46552459b1e2423d5fc56270d38a7f9c4260677c7a68"


def test_golden_canonical_bytes():
    rows = [_row(*CELLS[c], as_of=f"2026-08-2{i}") for i, c in enumerate(sorted(CELLS))]
    rows.append(dict(_mutated("b_confirmed_non_avoid", "after", final_entry_state="ALLOWED", market_bias="BULLISH_BIAS"),
                     as_of="2026-08-29"))
    out = v.judge_comparison(_comparison(rows))
    assert sha256_hex(canonical_json_bytes(out)) == GOLDEN_SHA256, canonical_json_bytes(out).decode()
