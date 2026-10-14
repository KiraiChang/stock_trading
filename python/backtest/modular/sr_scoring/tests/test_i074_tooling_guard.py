"""I-074 Stage 2 步驟 ⑨：tooling 非語意 guard 的 comparator（`python/scripts/i074_tooling_guard.py`）。

以合成的四個輸出目錄（Stage 1 A／B、Stage 2 A／B）驗「五」的逐檔封閉規則、hash 自洽與 provenance 期望值
（docs/issue.md「Stage 2 步驟 ⑨ 細部計畫 v1」「八」#4）。
"""
from __future__ import annotations

import copy
import hashlib
import importlib.util
import json
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


tg = _load("i074_tooling_guard")
TOOL = "7" * 64
BASE = "e1cbbbdab44f8cf2d152e6ade9235d844f590d7f"
ROWS = [{"symbol": "6243", "timeframe": "1d", "as_of": f"2026-08-{d:02d}", "value": d} for d in range(1, 6)]


def _prov(side, stage):
    out = f"/work/{side}{stage}"
    return {
        "source_root": "/app", "image_digest": "sha256:" + "1" * 64, "python_version": "3.11.17",
        "pip_freeze_sha256": "2" * 64, "runner_sha256": "3" * 64, "base_commit": BASE,
        "tooling_patch_sha256": tg.EMPTY_SHA256 if side == "A" else TOOL,
        "project_modules_sha256": {"backtest.modular.sr_scoring.decision_engine": "d",
                                   "backtest.modular.sr_scoring.evaluation": "ea" if side == "A" else "eb",
                                   **({"backtest.modular.sr_scoring.replay_bundle.stream": "s"} if side == "B" else {})},
        "argv": ["evaluation.py", "--bundle", "/b", "--output-dir", out, "--tooling-patch-sha256",
                 tg.EMPTY_SHA256 if side == "A" else TOOL, "--before-ref", BASE],
        "runtime_settings": {"x": 1},
    }


def _write(path: Path, doc) -> str:
    data = json.dumps(doc, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode("utf-8")
    path.write_bytes(data)
    return hashlib.sha256(data).hexdigest()


def _build(root: Path, mutate=None):
    """mutate(docs) 可以在寫檔之前改任何一份；hash 欄位在寫檔時才填（除非 mutate 指定）。"""
    docs = {}
    for side in ("A", "B"):
        docs[(1, side, tg.AFTER)] = {"schema_version": 1, "kind": "after", "bundle_id": "b1", "timeframe": "1d",
                                     "replay_scope": "full", "run_id": None, "pipeline_version": None,
                                     "generated_at": f"2026-10-12T00:00:0{1 if side == 'A' else 2}Z",
                                     "provenance": _prov(side, 1), "rows": copy.deepcopy(ROWS)}
        docs[(1, side, tg.COHORT)] = {"schema_version": 1, "kind": "cohort", "bundle_id": "b1",
                                      "generated_at": f"2026-10-12T00:00:0{3 if side == 'A' else 4}Z",
                                      "provenance": _prov(side, 1), "keys": [["6243", "1d", "2026-08-02"]]}
        docs[(2, side, tg.COMPARISON)] = {"schema_version": 1, "kind": "comparison", "bundle_id": "b1",
                                          "before_ref": BASE, "generated_at": f"t{side}", "provenance": _prov(side, 2),
                                          "rows": [{"symbol": "6243", "differences": []}]}
        docs[(2, side, tg.REPORT)] = {"schema_version": 1, "kind": "report", "bundle_id": "b1", "before_ref": BASE,
                                      "generated_at": f"r{side}", "candidate_rows": 1, "rows_shown": 1,
                                      "difference_field_counts": {}, "rows": [{"symbol": "6243", "differences": []}]}
    if mutate:
        mutate(docs)
    dirs = {}
    for (stage, side) in ((1, "A"), (1, "B"), (2, "A"), (2, "B")):
        d = root / f"s{stage}{side}"
        d.mkdir()
        dirs[(stage, side)] = d
    after_sha = {}
    for side in ("A", "B"):
        after_sha[side] = _write(dirs[(1, side)] / tg.AFTER, docs[(1, side, tg.AFTER)])
    for side in ("A", "B"):
        cohort = docs[(1, side, tg.COHORT)]
        cohort.setdefault("after_artifact_sha256", after_sha[side])
        _write(dirs[(1, side)] / tg.COHORT, cohort)
        comparison = docs[(2, side, tg.COMPARISON)]
        comparison.setdefault("after_artifact_sha256", after_sha["A"])
        comp_sha = _write(dirs[(2, side)] / tg.COMPARISON, comparison)
        report = docs[(2, side, tg.REPORT)]
        report.setdefault("comparison_artifact_sha256", comp_sha)
        _write(dirs[(2, side)] / tg.REPORT, report)
    return dirs


def _run(tmp_path, mutate=None):
    return tg.run(_build(tmp_path, mutate), TOOL, BASE)


def test_identical_outputs_pass(tmp_path):
    result = _run(tmp_path)
    assert result["ok"], result["problems"]
    assert result["after_rows"] == len(ROWS)


def test_removable_keys_are_exactly_the_plan_table():
    assert tg.REMOVABLE == {
        "after_artifact.json": ("generated_at", "provenance"),
        "cohort_manifest.json": ("generated_at", "provenance", "after_artifact_sha256"),
        "comparison_artifact.json": ("generated_at", "provenance"),
        "report.json": ("generated_at", "comparison_artifact_sha256"),
    }
    assert all(k not in v for v in tg.REMOVABLE.values() for k in tg.NEVER_REMOVED)


def _set(stage, side, name, key, value):
    def mutate(docs):
        docs[(stage, side, name)][key] = value
    return mutate


@pytest.mark.parametrize("mutate,needle", [
    (lambda d: d[(1, "B", tg.AFTER)]["rows"][3].update(value=99), "after_artifact.json"),
    (_set(1, "B", tg.AFTER, "run_id", "r2"), "after_artifact.json"),
    (_set(1, "B", tg.AFTER, "pipeline_version", "p2"), "after_artifact.json"),
    (_set(1, "B", tg.AFTER, "unexpected", 1), "after_artifact.json"),
    (_set(1, "B", tg.COHORT, "keys", []), "cohort_manifest.json"),
    (lambda d: d[(2, "B", tg.COMPARISON)]["rows"][0].update(differences=["x"]), "comparison_artifact.json"),
    (_set(2, "B", tg.REPORT, "difference_field_counts", {"x": 1}), "report.json"),
    (_set(2, "B", tg.REPORT, "candidate_rows", 2), "report.json"),
])
def test_any_difference_outside_the_removable_keys_fails(tmp_path, mutate, needle):
    result = _run(tmp_path, mutate)
    assert not result["ok"]
    assert any(needle in p and "正規化之後" in p for p in result["problems"]), result["problems"]


@pytest.mark.parametrize("mutate,needle", [
    (_set(1, "A", tg.COHORT, "after_artifact_sha256", "0" * 64), "cohort 的 after_artifact_sha256"),
    (_set(2, "B", tg.REPORT, "comparison_artifact_sha256", "0" * 64), "report 的 comparison_artifact_sha256"),
    (_set(2, "B", tg.COMPARISON, "after_artifact_sha256", "0" * 64), "comparison 的 after_artifact_sha256"),
])
def test_hashes_must_be_self_consistent_before_removal(tmp_path, mutate, needle):
    result = _run(tmp_path, mutate)
    assert not result["ok"]
    assert any(needle in p for p in result["problems"]), result["problems"]


def _prov_set(stage, side, field, value):
    def mutate(docs):
        name = tg.AFTER if stage == 1 else tg.COMPARISON
        docs[(stage, side, name)]["provenance"][field] = value
        if stage == 1:
            docs[(1, side, tg.COHORT)]["provenance"][field] = copy.deepcopy(value)
    return mutate


@pytest.mark.parametrize("mutate,needle", [
    (_prov_set(1, "A", "tooling_patch_sha256", TOOL), "A：tooling_patch_sha256"),
    (_prov_set(2, "B", "tooling_patch_sha256", "8" * 64), "B：tooling_patch_sha256"),
    (_prov_set(1, "B", "base_commit", "0" * 40), "base_commit"),
    (_prov_set(2, "B", "image_digest", "sha256:" + "9" * 64), "image_digest"),
    (_prov_set(1, "B", "runner_sha256", "4" * 64), "runner_sha256"),
    (_prov_set(1, "B", "project_modules_sha256",
               {"backtest.modular.sr_scoring.decision_engine": "changed", "backtest.modular.sr_scoring.evaluation": "eb"}),
     "tooling 以外的模組"),
])
def test_provenance_expectations(tmp_path, mutate, needle):
    result = _run(tmp_path, mutate)
    assert not result["ok"]
    assert any(needle in p for p in result["problems"]), result["problems"]


def test_argv_may_differ_only_in_the_two_variable_values(tmp_path):
    def mutate(docs):
        for name, stage in ((tg.AFTER, 1), (tg.COHORT, 1)):
            docs[(stage, "B", name)]["provenance"]["argv"][2] = "/other-bundle"
    result = _run(tmp_path, mutate)
    assert not result["ok"]
    assert any("argv 第 2 個 token" in p for p in result["problems"]), result["problems"]


def test_argv_length_must_match(tmp_path):
    def mutate(docs):
        docs[(2, "B", tg.COMPARISON)]["provenance"]["argv"].append("--extra")
    result = _run(tmp_path, mutate)
    assert any("argv 長度不同" in p for p in result["problems"]), result["problems"]


def test_provenance_fields_are_closed(tmp_path):
    def mutate(docs):
        docs[(2, "A", tg.COMPARISON)]["provenance"]["extra"] = 1
    result = _run(tmp_path, mutate)
    assert any("PROVENANCE_FIELDS" in p for p in result["problems"]), result["problems"]


def test_cross_stage_consistency_on_the_same_side(tmp_path):
    def mutate(docs):
        docs[(2, "A", tg.COMPARISON)]["provenance"]["image_digest"] = "sha256:" + "5" * 64
        docs[(2, "B", tg.COMPARISON)]["provenance"]["image_digest"] = "sha256:" + "5" * 64
    result = _run(tmp_path, mutate)
    assert any("Stage 1 與 Stage 2 的 provenance.image_digest" in p for p in result["problems"]), result["problems"]


def test_output_directory_must_hold_exactly_the_two_files(tmp_path):
    dirs = _build(tmp_path)
    (dirs[(1, "B")] / "extra.log").write_text("x", encoding="utf-8")
    result = tg.run(dirs, TOOL, BASE)
    assert not result["ok"]
    assert any("檔案集合" in p for p in result["problems"]), result["problems"]


def test_cli_rejects_an_empty_tooling_sha(tmp_path, capsys):
    rc = tg.main(["--stage1-a", "a", "--stage1-b", "b", "--stage2-a", "c", "--stage2-b", "d",
                  "--expected-tooling-sha256", tg.EMPTY_SHA256, "--base-commit", BASE,
                  "--out", str(tmp_path / "r.json")])
    assert rc == 1
    assert "非空" in capsys.readouterr().err
