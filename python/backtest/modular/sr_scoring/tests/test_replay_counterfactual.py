"""I-074 Stage 2 反事實路徑（`--i074-counterfactual`；⑦a 細部計畫 B4）：**真實的 `run_bundle_stage()`**。

⚠️ 與 `test_replay_bundle_stages.py` 相同，replay 本身用 `stub_replay` 取代（⛔ 不經過產品資料流）——
這裡驗的是反事實路徑的**邏輯與順序**：成對守門、replay 之前的讀取與版本守門、全量 key 守門、
「①之三」（`CounterfactualEffectCheck`）、終態與結束碼。真實產品資料流的非空翻轉由
`scripts/smoke-replay-offline.sh`（`REPLAY_SMOKE=1`）的正式 bundle 切片負責。

測試 id 對照 issue.md I-074「Stage 2 計畫書」「六、2」的 a～i、aa～ac2、o、p～r、y。
"""
from __future__ import annotations

import json
import sys

import pytest

from .. import evaluation as evaluation_module
from .. import replay_bundle as replay_bundle_pkg
from ..evaluation import CliUsageError, run_bundle_stage
from ..replay_bundle import (
    AFTER_ARTIFACT_NAME,
    COHORT_MANIFEST_NAME,
    EXIT_CANDIDATE_MISMATCH,
    EXIT_COUNTERFACTUAL_INEFFECTIVE,
    MISMATCH_ARTIFACT_NAME,
    OPERATIONAL_BEFORE_SOURCE,
    OPERATIONAL_COMPARISON,
    OPERATIONAL_FAILURE,
    OPERATIONAL_REPORT,
    ArtifactError,
    CandidateMismatch,
    CounterfactualIneffective,
    load_canonical_evidence_artifact,
    validate_comparison_artifact,
    validate_counterfactual_failure,
)
from ..replay_bundle import stream as stream_module
from ..replay_bundle.canonical import canonical_json_bytes, sha256_hex
from ..replay_bundle.stage2_archive import (
    BEFORE_SOURCE_KIND,
    COUNTERFACTUAL_FAILURE_KIND,
    FAILURE_CANDIDATE_FLAG_INCONSISTENT,
    FAILURE_RR_NOT_RESTORED,
    stream_before_source,
)
from .test_replay_bundle_stages import _args, _bundle_with_bars, stub_replay  # noqa: F401 - fixture

BASE = "e" * 40
CF_SHA = "f" * 64
# ⚠️ provenance 的 `argv` 必須是非空陣列（反事實路徑以 `validate_provenance(role="stage1")` 驗 after／cohort）。
S1_ARGV = ["--bundle", "b"]
CF_ARGV = ["--i074-counterfactual"]
NAMES = {rel.split("/")[-1] for rel in (OPERATIONAL_BEFORE_SOURCE, OPERATIONAL_COMPARISON, OPERATIONAL_REPORT)}
FAILURE_NAME = OPERATIONAL_FAILURE.split("/")[-1]


def _stage1(tmp_path, stub_replay, count=3):
    """Stage 1（一般路徑）產出帶 `count` 筆候選的 after artifact 與 cohort；base 是 `BASE`。"""
    bundle = _bundle_with_bars(tmp_path)
    probe = tmp_path / "probe"
    run_bundle_stage(_args(bundle, probe, base_commit=BASE), stage=1, argv=S1_ARGV)
    after = json.loads((probe / AFTER_ARTIFACT_NAME).read_text(encoding="utf-8"))
    keys = [(r["symbol"], r["timeframe"], r["as_of"]) for r in after["rows"]]
    stub_replay["candidate_keys"] = set(keys[:count])
    stage1_out = tmp_path / "stage1"
    run_bundle_stage(_args(bundle, stage1_out, base_commit=BASE), stage=1, argv=S1_ARGV)
    return bundle, stage1_out, keys, keys[:count]


def _cf_args(bundle, out, stage1_out, **overrides):
    base = dict(
        after_artifact=str(stage1_out / AFTER_ARTIFACT_NAME),
        cohort_manifest=str(stage1_out / COHORT_MANIFEST_NAME),
        before_ref=BASE, base_commit=BASE,
        i074_counterfactual=True, counterfactual_patch_sha256=CF_SHA,
    )
    base.update(overrides)
    return _args(bundle, out, **base)


def _restore_rr(stub_replay):
    """before 側：RR 已加回——原本的候選列不再是 CONTINUATION（flag 與等價式一致）。"""
    stub_replay["candidate_keys"] = set()


@pytest.fixture
def replay_spy(monkeypatch):
    """記錄 `_replay_from_bundle` 有沒有被呼叫（守門必須在它**之前**擋下）。"""
    calls = []
    real = evaluation_module._replay_from_bundle

    def spy(loaded, **kwargs):
        calls.append(loaded.bundle_id)
        return real(loaded, **kwargs)

    monkeypatch.setattr(evaluation_module, "_replay_from_bundle", spy)
    return calls


def _rewrite(path, mutate):
    """改一份 canonical artifact 並以 canonical bytes 寫回；回傳新的 payload SHA。"""
    data = json.loads(path.read_text(encoding="utf-8"))
    mutate(data)
    raw = canonical_json_bytes(data)
    path.write_bytes(raw)
    return sha256_hex(raw)


def _rebind_cohort(stage1_out, after_sha):
    """after 被改過之後，把 cohort 的 `after_artifact_sha256` 綁回去——讓測試只踩到要驗的那一道。"""
    _rewrite(stage1_out / COHORT_MANIFEST_NAME, lambda c: c.__setitem__("after_artifact_sha256", after_sha))


# ── 成功路徑（a、c、f、終態） ───────────────────────────────────────────────

def test_success_writes_exactly_three_operational_files_report_last(tmp_path, stub_replay, monkeypatch):
    bundle, stage1_out, keys, candidates = _stage1(tmp_path, stub_replay)
    _restore_rr(stub_replay)
    out = tmp_path / "stage2"
    import os

    order = []
    real_replace = os.replace

    def spy(src, dst, *a, **kw):
        order.append(str(dst).split("/")[-1])
        return real_replace(src, dst, *a, **kw)

    monkeypatch.setattr(os, "replace", spy)
    result = run_bundle_stage(_cf_args(bundle, out, stage1_out), stage=2, argv=CF_ARGV)

    assert {p.name for p in out.iterdir()} == NAMES                      # 終態恰好一種：三檔
    assert order[-1] == OPERATIONAL_REPORT.split("/")[-1]                 # report 最後寫
    assert result["mode"] == "counterfactual" and result["counterfactual_patch_sha256"] == CF_SHA
    assert result["candidates"] == len(candidates)                        # f：cohort 全部進 comparison
    comparison = json.loads((out / OPERATIONAL_COMPARISON.split("/")[-1]).read_text(encoding="utf-8"))
    assert [(r["symbol"], r["timeframe"], r["as_of"]) for r in comparison["rows"]] == sorted(candidates)
    assert validate_comparison_artifact(comparison) == sorted(candidates)
    for row in comparison["rows"]:                                       # 逐列翻轉
        assert row["after"]["rr_decoupling_candidate"] is True
        assert row["before"]["rr_decoupling_candidate"] is False
        assert "lifecycle_phase" in row["differences"]
    after_load = load_canonical_evidence_artifact(stage1_out / AFTER_ARTIFACT_NAME, "sr_zone_replay_after")
    assert comparison["after_artifact_sha256"] == after_load.artifact_sha256
    before = stream_before_source(out / OPERATIONAL_BEFORE_SOURCE.split("/")[-1], keep_keys=set(candidates))
    assert before.keys == keys and before.effect is None and before.candidate_keys == []
    assert before.load.top["kind"] == BEFORE_SOURCE_KIND
    assert before.load.top["provenance"] == comparison["provenance"]      # 全圖第 10 道：同一個物件
    assert before.load.top["before_ref"] == comparison["before_ref"] == BASE


def test_the_effect_check_is_fed_the_before_rows(tmp_path, stub_replay, monkeypatch):
    """決策 4：餵進 `CounterfactualEffectCheck` 的**一定是本次 replay 的 before rows**。

    after 有候選（CONTINUATION 且 RR 不合格）、before 已加回 RR：誤餵 after rows 的話會得出 rr_not_restored。
    """
    bundle, stage1_out, keys, candidates = _stage1(tmp_path, stub_replay)
    _restore_rr(stub_replay)
    from ..replay_bundle import stage2_archive

    fed = []
    real_feed = stage2_archive.CounterfactualEffectCheck.feed

    def spy(self, row):
        fed.append((row["symbol"], row["timeframe"], row["as_of"], row["lifecycle_phase"]))
        return real_feed(self, row)

    monkeypatch.setattr(stage2_archive.CounterfactualEffectCheck, "feed", spy)
    run_bundle_stage(_cf_args(bundle, tmp_path / "stage2", stage1_out), stage=2, argv=CF_ARGV)
    assert [f[:3] for f in fed] == keys
    assert all(phase != "CONTINUATION" for *_k, phase in fed)            # 全是 before 的列


# ── 反事實沒有生效（d、aa、ab、ac、ac2） ─────────────────────────────────────

def _expect_ineffective(tmp_path, bundle, stage1_out, reason, count):
    out = tmp_path / "stage2"
    with pytest.raises(CounterfactualIneffective) as exc:
        run_bundle_stage(_cf_args(bundle, out, stage1_out), stage=2, argv=CF_ARGV)
    assert {p.name for p in out.iterdir()} == {FAILURE_NAME}             # 終態：只有中繼檔
    assert not (out / MISMATCH_ARTIFACT_NAME).exists()                   # ⛔ 不是 rc=4 的那條路
    assert exc.value.path == out / FAILURE_NAME
    payload = load_canonical_evidence_artifact(out / FAILURE_NAME, COUNTERFACTUAL_FAILURE_KIND).parsed
    validate_counterfactual_failure(payload)
    assert payload["failure_reason"] == reason
    count_field = "before_candidate_count" if reason == FAILURE_RR_NOT_RESTORED else "inconsistent_row_count"
    assert payload["bounded_diagnostics"][count_field] == count
    assert payload["provenance"]["base_commit"] == BASE
    return payload


def test_rr_not_restored_is_exit_6_with_bounded_diagnostics(tmp_path, stub_replay):
    """d／aa：flag 一致、before 仍有候選 → rr_not_restored（⛔ 不是 1、⛔ 不是 4）。"""
    bundle, stage1_out, _keys, candidates = _stage1(tmp_path, stub_replay)
    _expect_ineffective(tmp_path, bundle, stage1_out, FAILURE_RR_NOT_RESTORED, len(candidates))


def test_flag_always_false_but_continuation_without_rr_is_inconsistent(tmp_path, stub_replay):
    """ab：flag 恆 false、卻有列是 CONTINUATION 且 RR 不合格——`before_candidates == ∅` 會假綠的形狀。"""
    bundle, stage1_out, _keys, candidates = _stage1(tmp_path, stub_replay)
    stub_replay["mutate"] = lambda row: row.__setitem__("rr_decoupling_candidate", False)
    _expect_ineffective(tmp_path, bundle, stage1_out, FAILURE_CANDIDATE_FLAG_INCONSISTENT, len(candidates))


def test_flag_true_on_a_non_continuation_row_is_inconsistent(tmp_path, stub_replay):
    """ac：flag 為 true 但該列不是 CONTINUATION。"""
    bundle, stage1_out, keys, _c = _stage1(tmp_path, stub_replay)
    _restore_rr(stub_replay)
    target = keys[-1]

    def mutate(row):
        if (row["symbol"], row["timeframe"], row["as_of"]) == target:
            row["rr_decoupling_candidate"] = True

    stub_replay["mutate"] = mutate
    _expect_ineffective(tmp_path, bundle, stage1_out, FAILURE_CANDIDATE_FLAG_INCONSISTENT, 1)


def test_both_shapes_at_once_are_always_flag_inconsistent(tmp_path, stub_replay):
    """ac2：有列 flag 不一致、另有列 flag 一致且是候選 → 一律 candidate_flag_inconsistent（檢查順序）。"""
    bundle, stage1_out, keys, candidates = _stage1(tmp_path, stub_replay)   # before 保留候選
    target = keys[-1]
    assert target not in candidates

    def mutate(row):
        if (row["symbol"], row["timeframe"], row["as_of"]) == target:
            row["rr_decoupling_candidate"] = True

    stub_replay["mutate"] = mutate
    payload = _expect_ineffective(tmp_path, bundle, stage1_out, FAILURE_CANDIDATE_FLAG_INCONSISTENT, 1)
    assert [(s["symbol"], s["timeframe"], s["as_of"]) for s in payload["bounded_diagnostics"]["sample_keys"]] \
        == [target]


# ── 全量 key 守門先於生效檢查（b、e） ────────────────────────────────────────

@pytest.mark.parametrize("shape", ["drop", "extra", "reorder"])
def test_key_guard_runs_before_the_effect_check(tmp_path, stub_replay, monkeypatch, shape):
    """b／e：before 少一個／多一個／換序 → fail-closed（rc=1）；before 同時仍有候選也⛔ 不得變成 rc=6。"""
    bundle, stage1_out, keys, candidates = _stage1(tmp_path, stub_replay)   # before 保留候選（生效檢查會失敗）
    if shape == "drop":
        stub_replay["drop_key"] = keys.index(candidates[0])                   # e：cohort key 在 before 缺席
    elif shape == "extra":
        stub_replay["extra_key"] = "2099-01-01T00:00:00+00:00"
    else:
        real = evaluation_module._replay_from_bundle

        def reversed_replay(loaded, **kwargs):
            rows, context = real(loaded, **kwargs)
            rows = list(reversed(rows))
            return rows, dict(context, keys=[(r["symbol"], r["timeframe"], r["as_of"]) for r in rows])

        monkeypatch.setattr(evaluation_module, "_replay_from_bundle", reversed_replay)
    out = tmp_path / "stage2"
    with pytest.raises(ArtifactError):
        run_bundle_stage(_cf_args(bundle, out, stage1_out), stage=2, argv=CF_ARGV)
    assert list(out.iterdir()) == []


# ── replay 之前的守門（g、h、i、provenance、版本） ───────────────────────────

def _drop_first_diag(row):
    row.pop("structure_state")


@pytest.mark.parametrize("mutate", [
    pytest.param(lambda a: a.pop("replay_scope"), id="envelope"),
    pytest.param(lambda a: a["rows"].__setitem__(0, "not-an-object"), id="row-not-object"),
    pytest.param(lambda a: a["rows"].append(dict(a["rows"][0])), id="duplicate-key"),
    pytest.param(lambda a: a["rows"][0].__setitem__("rr_decoupling_candidate", 1), id="flag-type"),
    pytest.param(lambda a: a["rows"][0].__setitem__("decision_error", "boom"), id="replay-error"),
    pytest.param(lambda a: _drop_first_diag(a["rows"][0]), id="diagnostic-field"),
])
def test_after_artifact_defects_abort_before_replay(tmp_path, stub_replay, replay_spy, mutate):
    """g：串流 loader 的逐列驗證與整批版本**同強度**，而且全部在 replay 之前。"""
    bundle, stage1_out, _keys, _c = _stage1(tmp_path, stub_replay)
    replay_spy.clear()
    after_sha = _rewrite(stage1_out / AFTER_ARTIFACT_NAME, mutate)
    _rebind_cohort(stage1_out, after_sha)
    with pytest.raises(ValueError):
        run_bundle_stage(_cf_args(bundle, tmp_path / "stage2", stage1_out), stage=2, argv=CF_ARGV)
    assert replay_spy == []


def test_after_artifact_is_read_exactly_once(tmp_path, stub_replay, monkeypatch):
    """i：after 只串流讀一次（⛔ 不重複消費、⛔ 不另外整份載入）。"""
    bundle, stage1_out, _keys, _c = _stage1(tmp_path, stub_replay)
    _restore_rr(stub_replay)
    reads = []
    real = stream_module.stream_canonical_artifact

    def spy(path, kind, **kwargs):
        reads.append(str(path))
        return real(path, kind, **kwargs)

    monkeypatch.setattr(stream_module, "stream_canonical_artifact", spy)

    def forbidden(*a, **kw):
        raise AssertionError("⛔ 反事實路徑不得用整份載入的 load_artifact()")

    monkeypatch.setattr(replay_bundle_pkg, "load_artifact", forbidden)
    run_bundle_stage(_cf_args(bundle, tmp_path / "stage2", stage1_out), stage=2, argv=CF_ARGV)
    assert reads == [str(stage1_out / AFTER_ARTIFACT_NAME)]


def test_cohort_sha_mismatch_aborts_before_replay(tmp_path, stub_replay, replay_spy):
    """h：cohort 記的 after SHA ≠ 串流算出的 payload SHA。"""
    bundle, stage1_out, _keys, _c = _stage1(tmp_path, stub_replay)
    replay_spy.clear()
    _rebind_cohort(stage1_out, "0" * 64)
    with pytest.raises(ArtifactError, match="必須相符"):
        run_bundle_stage(_cf_args(bundle, tmp_path / "stage2", stage1_out), stage=2, argv=CF_ARGV)
    assert replay_spy == []


@pytest.mark.parametrize("which", ["after", "cohort"])
@pytest.mark.parametrize("damage", [
    pytest.param(lambda p: p.pop("runner_sha256"), id="missing"),
    pytest.param(lambda p: p.__setitem__("extra", "x"), id="extra"),
    pytest.param(lambda p: p.__setitem__("base_commit", None), id="null-base"),
    pytest.param(lambda p: p.__setitem__("image_digest", "not-a-digest"), id="bad-type"),
])
def test_broken_provenance_aborts_before_replay(tmp_path, stub_replay, replay_spy, which, damage):
    """第一輪 review：after／cohort 的 provenance 以 `role="stage1"` 驗，損壞即在 replay 之前中止。"""
    bundle, stage1_out, _keys, _c = _stage1(tmp_path, stub_replay)
    replay_spy.clear()
    if which == "after":
        after_sha = _rewrite(stage1_out / AFTER_ARTIFACT_NAME, lambda a: damage(a["provenance"]))
        _rebind_cohort(stage1_out, after_sha)
    else:
        _rewrite(stage1_out / COHORT_MANIFEST_NAME, lambda c: damage(c["provenance"]))
    with pytest.raises(ValueError):
        run_bundle_stage(_cf_args(bundle, tmp_path / "stage2", stage1_out), stage=2, argv=CF_ARGV)
    assert replay_spy == []


@pytest.mark.parametrize("overrides", [
    pytest.param({"base_commit": "d" * 40, "before_ref": "d" * 40}, id="after-base-differs"),
    pytest.param({"before_ref": "d" * 40}, id="before-ref-differs"),
    pytest.param({"before_ref": "e" * 7}, id="before-ref-short"),
    pytest.param({"before_ref": "E" * 40}, id="before-ref-uppercase"),
])
def test_version_guard_aborts_before_replay(tmp_path, stub_replay, replay_spy, overrides):
    """「三」#3：CLI 的 `--before-ref`（40 碼）＝ CLI 的 `--base-commit` ＝ after artifact 的 `provenance.base_commit`。"""
    bundle, stage1_out, _keys, _c = _stage1(tmp_path, stub_replay)
    replay_spy.clear()
    with pytest.raises(ArtifactError):
        run_bundle_stage(_cf_args(bundle, tmp_path / "stage2", stage1_out, **overrides), stage=2, argv=CF_ARGV)
    assert replay_spy == []


# ── 成對守門（y）與 stage 限定（p） ───────────────────────────────────────────

@pytest.mark.parametrize("stage,flag,sha,match", [
    pytest.param(2, True, None, "缺少", id="flag-without-sha"),
    pytest.param(2, False, CF_SHA, "一般路徑", id="sha-without-flag"),
    pytest.param(1, False, CF_SHA, "Stage 1", id="stage1-with-sha"),
    pytest.param(1, True, CF_SHA, "只能用於 Stage 2", id="stage1-with-flag"),
    pytest.param(2, True, "F" * 64, "小寫 hex", id="uppercase-sha"),
    pytest.param(2, True, "f" * 63, "小寫 hex", id="short-sha"),
])
def test_pairing_guard_runs_before_loading_anything(tmp_path, monkeypatch, stage, flag, sha, match):
    def forbidden(*a, **kw):
        raise AssertionError("⛔ 成對守門必須在 load_bundle() 之前")

    monkeypatch.setattr(replay_bundle_pkg, "load_bundle", forbidden)
    args = _args(tmp_path / "bundle", tmp_path / "out", i074_counterfactual=flag,
                 counterfactual_patch_sha256=sha)
    with pytest.raises(CliUsageError, match=match):
        run_bundle_stage(args, stage=stage, argv=[])


def test_assert_i074_flags_limits_counterfactual_to_stage2():
    evaluation_module.assert_i074_flags({"i074_counterfactual"}, stage=2)
    with pytest.raises(CliUsageError):
        evaluation_module.assert_i074_flags({"i074_counterfactual"}, stage=1)


# ── o：一般路徑逐項不變 ─────────────────────────────────────────────────────

def test_without_the_flag_the_general_stage2_still_raises_candidate_mismatch(tmp_path, stub_replay):
    """o：⛔ 未帶 flag 時，集合相等檢查、`candidate_mismatch.json`、rc=4 都還在。"""
    bundle, stage1_out, _keys, _c = _stage1(tmp_path, stub_replay)
    _restore_rr(stub_replay)                          # before 的候選集合 ≠ after 的
    out = tmp_path / "stage2"
    args = _args(bundle, out, after_artifact=str(stage1_out / AFTER_ARTIFACT_NAME),
                 cohort_manifest=str(stage1_out / COHORT_MANIFEST_NAME))
    with pytest.raises(CandidateMismatch):
        run_bundle_stage(args, stage=2, argv=CF_ARGV)
    assert (out / MISMATCH_ARTIFACT_NAME).exists()


# ── 直接 CLI（`main()`）：結束碼與 argparse 層 ───────────────────────────────

def _main(monkeypatch, argv):
    monkeypatch.setattr(sys, "argv", ["evaluation.py", *argv])
    try:
        evaluation_module.main()
    except SystemExit as exc:
        return int(exc.code or 0)
    return 0


def _cli_argv(bundle, out, stage1_out, *extra):
    return [
        "--bundle", str(bundle), "--output-dir", str(out), "--before-ref", BASE,
        "--after-artifact", str(stage1_out / AFTER_ARTIFACT_NAME),
        "--cohort-manifest", str(stage1_out / COHORT_MANIFEST_NAME),
        "--image-digest", "sha256:" + "a" * 64, "--source-root", "/app", "--base-commit", BASE,
        "--tooling-patch-sha256", "1" * 64, "--runner-sha256", "c" * 64, *extra,
    ]


def test_main_maps_ineffective_to_exit_6(tmp_path, stub_replay, monkeypatch, capsys):
    bundle, stage1_out, _keys, _c = _stage1(tmp_path, stub_replay)
    code = _main(monkeypatch, _cli_argv(bundle, tmp_path / "stage2", stage1_out,
                                        "--counterfactual-patch-sha256", CF_SHA, "--i074-counterfactual"))
    assert code == EXIT_COUNTERFACTUAL_INEFFECTIVE == 6
    assert "[counterfactual-ineffective]" in capsys.readouterr().err


def test_main_success_prints_the_counterfactual_summary(tmp_path, stub_replay, monkeypatch, capsys):
    bundle, stage1_out, _keys, _c = _stage1(tmp_path, stub_replay)
    _restore_rr(stub_replay)
    code = _main(monkeypatch, _cli_argv(bundle, tmp_path / "stage2", stage1_out,
                                        "--counterfactual-patch-sha256", CF_SHA, "--i074-counterfactual"))
    assert code == 0
    summary = json.loads(capsys.readouterr().out)
    assert summary["mode"] == "counterfactual" and summary["counterfactual_patch_sha256"] == CF_SHA


@pytest.mark.parametrize("extra,expected", [
    pytest.param(["--i074-counterfactual"], 1, id="flag-without-sha"),
    pytest.param(["--counterfactual-patch-sha256", CF_SHA], 1, id="sha-without-flag"),
    pytest.param(["--i074-counterfactual", "--i074-counterfactual", "--counterfactual-patch-sha256", CF_SHA], 1,
                 id="duplicate-flag"),
    pytest.param(["--i074-counterfactual", "--counterfactual-patch-sha256", CF_SHA,
                  "--counterfactual-patch-sha256", CF_SHA], 1, id="duplicate-sha"),
])
def test_main_rejects_bad_combinations(tmp_path, stub_replay, monkeypatch, capsys, extra, expected):
    """y（直接 CLI 這條路徑）＋ q／r：重複的 flag 與重複的注入參數。"""
    bundle, stage1_out, _keys, _c = _stage1(tmp_path, stub_replay)
    assert _main(monkeypatch, _cli_argv(bundle, tmp_path / "stage2", stage1_out, *extra)) == expected


def test_main_rejects_the_flag_on_stage1(tmp_path, stub_replay, monkeypatch):
    """p：flag 帶在 Stage 1。"""
    bundle = _bundle_with_bars(tmp_path)
    argv = ["--bundle", str(bundle), "--output-dir", str(tmp_path / "s1"), "--before-ref", BASE,
            "--image-digest", "sha256:" + "a" * 64, "--source-root", "/app", "--base-commit", BASE,
            "--tooling-patch-sha256", "1" * 64, "--runner-sha256", "c" * 64, "--i074-counterfactual"]
    assert _main(monkeypatch, argv) == 1


def test_exit_codes_are_distinct():
    assert EXIT_COUNTERFACTUAL_INEFFECTIVE == 6 and EXIT_CANDIDATE_MISMATCH == 4
    for rel in (OPERATIONAL_BEFORE_SOURCE, OPERATIONAL_COMPARISON, OPERATIONAL_REPORT, OPERATIONAL_FAILURE):
        assert rel.split("/")[0] == "stage2"
