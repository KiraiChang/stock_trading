"""I-074 Stage 2 步驟 ⑨：完整驗證與 ⑧ baseline 的逐項比對（`python/scripts/i074_step9_runs.py`）。

baseline 是版控的 ⑧ 第三次全量執行 log；「本次」的 log 由它改寫而來（docs/issue.md「Stage 2 步驟 ⑨ 細部計畫 v1」
「八」#8 的規則）。
"""
from __future__ import annotations

import importlib.util
import sys
from pathlib import Path

import pytest

_SCRIPTS = Path(__file__).resolve().parents[4] / "scripts"


def _load(name: str):
    spec = importlib.util.spec_from_file_location(name, _SCRIPTS / f"{name}.py")
    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module


rn = _load("i074_step9_runs")
BASELINE_TEXT = rn.BASELINE_PATH.read_bytes().decode("utf-8")
BASE_NODE_COUNT = len(rn.parse(BASELINE_TEXT)["nodes"])
NEW_FILE = rn.NEW_TEST_FILES[0]


@pytest.fixture(scope="module")
def baseline():
    return rn.load_baseline()


def _lines():
    return BASELINE_TEXT.splitlines()


def _first_node_line(lines, outcome="PASSED"):
    return next(i for i, line in enumerate(lines) if rn.NODE_RE.match(line) and f" {outcome} " in line)


def _with_extra_nodes(lines, extra):
    """在第一個 node 之前插入 extra 行，並把 collected 加上同樣的數量。"""
    idx = _first_node_line(lines)
    out = lines[:idx] + extra + lines[idx:]
    before, after = f"collected {BASE_NODE_COUNT} items", f"collected {BASE_NODE_COUNT + len(extra)} items"
    return [line.replace(before, after) for line in out]


def _compare(baseline, lines):
    return rn.compare(baseline, rn.parse("\n".join(lines) + "\n"))


def test_baseline_constants_match_the_versioned_file(baseline):
    assert len(baseline["nodes"]) == rn.BASELINE_COUNTS["pytest_nodes"]
    assert sum(v == "ok" for v in baseline["labels"].values()) == rn.BASELINE_COUNTS["shell_ok"]
    assert "host unittest：Ran 72 tests" in baseline["labels"]


def test_identical_run_passes(baseline):
    result = _compare(baseline, _lines())
    assert result["ok"], result["problems"]
    assert result["new_items"] == []


def test_host_unittest_timing_is_the_only_normalization(baseline):
    lines = [line.replace("Ran 72 tests in 46.219s", "Ran 72 tests in 51.002s") for line in _lines()]
    assert _compare(baseline, lines)["ok"]
    assert rn.normalize_label("host unittest：Ran 72 tests in 1.5s") == "host unittest：Ran 72 tests"
    assert rn.normalize_label("other in 1.5s") == "other in 1.5s"


def test_passed_to_failed_is_a_regression(baseline):
    lines = _lines()
    i = _first_node_line(lines)
    lines[i] = lines[i].replace(" PASSED ", " FAILED ")
    result = _compare(baseline, lines)
    assert not result["ok"] and result["regressions"]


def test_passed_to_skipped_is_a_regression(baseline):
    lines = _lines()
    i = _first_node_line(lines)
    lines[i] = lines[i].replace(" PASSED ", " SKIPPED ")
    assert _compare(baseline, lines)["regressions"]


def test_shell_ok_to_fail_is_a_regression(baseline):
    lines = _lines()
    i = next(i for i, line in enumerate(lines) if line.startswith("  ok "))
    lines[i] = lines[i].replace("  ok ", "  FAIL ", 1)
    assert _compare(baseline, lines)["regressions"]


def test_missing_existing_item_fails(baseline):
    lines = _lines()
    i = next(i for i, line in enumerate(lines) if line.startswith("  ok "))
    del lines[i]
    result = _compare(baseline, lines)
    assert not result["ok"] and result["missing"]


def test_new_items_in_scope_must_all_pass(baseline):
    ok = _with_extra_nodes(_lines(), [f"{NEW_FILE}::test_new PASSED [ 50%]"]) + ["  ok   step9：入口 n1"]
    result = _compare(baseline, ok)
    assert result["ok"], result["problems"]
    assert len(result["new_items"]) == 2
    failing = _with_extra_nodes(_lines(), [f"{NEW_FILE}::test_new FAILED [ 50%]"])
    result = _compare(baseline, failing)
    assert not result["ok"] and result["new_failed"]
    failing_label = _lines() + ["  FAIL step9：入口 n1"]
    assert _compare(baseline, failing_label)["new_failed"]


def test_new_items_out_of_scope_fail(baseline):
    lines = _with_extra_nodes(_lines(), ["backtest/modular/sr_scoring/tests/test_other.py::test_x PASSED [ 50%]"])
    result = _compare(baseline, lines)
    assert not result["ok"] and result["out_of_scope"]
    assert _compare(baseline, _lines() + ["  ok   新的標籤沒有前綴"])["out_of_scope"]


def test_renamed_existing_test_shows_as_missing_and_out_of_scope(baseline):
    lines = _lines()
    i = _first_node_line(lines)
    lines[i] = lines[i].replace("::test_", "::renamed_test_", 1)
    result = _compare(baseline, lines)
    assert result["missing"] and result["out_of_scope"]


@pytest.mark.parametrize("damage", ["no_summary", "collected_mismatch", "no_labels"])
def test_incomplete_new_log_fails(baseline, damage):
    lines = _lines()
    if damage == "no_summary":
        lines = [line for line in lines if not rn.SUMMARY_RE.match(line)]
    elif damage == "collected_mismatch":
        lines = [line.replace("collected 2295 items", "collected 2296 items") for line in lines]
    else:
        lines = [line for line in lines if not rn.LABEL_RE.match(line)]
    assert not _compare(baseline, lines)["ok"]


@pytest.mark.parametrize("damage", ["truncate", "byte", "empty"])
def test_baseline_that_is_not_the_recorded_file_is_rejected(tmp_path, damage):
    raw = rn.BASELINE_PATH.read_bytes()
    if damage == "truncate":
        raw = raw[: len(raw) // 2]
    elif damage == "byte":
        raw = raw.replace(b"PASSED", b"FAILED", 1)
    else:
        raw = b""
    path = tmp_path / "baseline.log"
    path.write_bytes(raw)
    with pytest.raises(rn.RunsError):
        rn.load_baseline(path)


def test_cli_rejects_an_empty_new_log(tmp_path, capsys):
    empty = tmp_path / "new.log"
    empty.write_bytes(b"")
    assert rn.main(["--new-log", str(empty), "--out", str(tmp_path / "r.json")]) == 1
    assert "空的" in capsys.readouterr().err
