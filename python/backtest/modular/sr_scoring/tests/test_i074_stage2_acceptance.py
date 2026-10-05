"""I-074 Stage 2 ⑦d：memory／disk acceptance harness 的 Python 段（launcher 與 `i074_stage2_sizing.py` 的 ⑦d 部分）。

對應 issue.md I-074「Stage 2 步驟 ⑦d 細部計畫 v1」「五」的 ac1～ac3b（launcher）、ac5（profile）、ac6（門檻）、ac9（環境的契約）、
ac10（replay argv）、ac12／ac12b（observer 的核心）、ac14（sizing 的 runner_frozen_patches）、ac19（harness_manifest）。
shim、harness 的守門、互斥、快照與 bootstrap 在 `scripts/test-replay-args.sh`／`scripts/test-i074-stage2.sh`；需要 `scripts/` 的
`clean-env`、晉升的 state、`host-run` 與 3.9 相容性在 host unittest（`scripts/tests/test_i074_stage2_host.py`）。
"""
from __future__ import annotations

import hashlib
import importlib.util
import json
import os
import sys
from pathlib import Path

import pytest

from .. import evaluation as evaluation_module
from ..replay_bundle import (
    AFTER_ARTIFACT_NAME,
    COHORT_MANIFEST_NAME,
    OPERATIONAL_BEFORE_SOURCE,
    OPERATIONAL_COMPARISON,
    OPERATIONAL_FAILURE,
    OPERATIONAL_REPORT,
)
from . import test_i074_stage2_sizing as tsz
from .test_replay_bundle_stages import stub_replay  # noqa: F401 - fixture
from .test_replay_counterfactual import CF_SHA, _cli_argv, _stage1

sz = tsz.sz
PYTHON_ROOT = Path(__file__).resolve().parents[4]
_spec = importlib.util.spec_from_file_location("i074_stage2_replay_stub", PYTHON_ROOT / "scripts" / "i074_stage2_replay_stub.py")
stub = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(stub)

NAMES = {Path(r).name for r in (OPERATIONAL_BEFORE_SOURCE, OPERATIONAL_COMPARISON, OPERATIONAL_REPORT)}
FAILURE_NAME = Path(OPERATIONAL_FAILURE).name
LIMIT = sz.ACCEPTANCE_MEMORY_LIMIT


# ── ac1～ac3b：launcher ─────────────────────────────────────────────────────────

@pytest.fixture
def launch(monkeypatch, stub_replay):
    """以 launcher 跑一次（`ev.main()` 的 SystemExit 轉成結束碼）；結束後還原它改過的屬性與 sys.argv。"""
    monkeypatch.setattr(evaluation_module, "_decision_replay_rows", evaluation_module._decision_replay_rows)
    monkeypatch.setattr(sys, "argv", list(sys.argv))
    monkeypatch.chdir(PYTHON_ROOT)

    def run(argv, *, mode="success", compute="stub", env=True):
        if env:
            monkeypatch.setenv("I074_ACCEPTANCE_REPLAY", mode)
            monkeypatch.setenv("I074_ACCEPTANCE_COMPUTE", compute)
        try:
            return stub.main(list(argv))
        except SystemExit as exc:
            return int(exc.code or 0)

    return run


def _cf(bundle, out, stage1_out):
    return _cli_argv(bundle, out, stage1_out, "--counterfactual-patch-sha256", CF_SHA, "--i074-counterfactual")


def _rows(path):
    return json.loads(Path(path).read_text(encoding="utf-8"))["rows"]


def test_ac2_stub_success_is_rc0_and_only_cohort_rows_change(tmp_path, stub_replay, launch):
    bundle, stage1_out, keys, cohort = _stage1(tmp_path, stub_replay)
    out = tmp_path / "stage2"
    assert launch(_cf(bundle, out, stage1_out)) == 0
    assert {p.name for p in out.iterdir()} == NAMES
    before = _rows(out / Path(OPERATIONAL_BEFORE_SOURCE).name)
    after = _rows(stage1_out / AFTER_ARTIFACT_NAME)
    assert len(before) == len(after) == len(keys)
    for b, a in zip(before, after):
        if (a["symbol"], a["timeframe"], a["as_of"]) in set(cohort):
            assert (b["lifecycle_phase"], b["rr_decoupling_candidate"]) == ("TESTING", False)
            assert {k: v for k, v in b.items() if k not in ("lifecycle_phase", "rr_decoupling_candidate")} == \
                   {k: v for k, v in a.items() if k not in ("lifecycle_phase", "rr_decoupling_candidate")}
        else:
            assert b == a


def test_ac2_stub_failure_is_rc6_with_only_the_bounded_diagnostics(tmp_path, stub_replay, launch):
    bundle, stage1_out, _keys, _cohort = _stage1(tmp_path, stub_replay)
    out = tmp_path / "stage2"
    assert launch(_cf(bundle, out, stage1_out), mode="failure") == 6
    assert [p.name for p in out.iterdir()] == [FAILURE_NAME]
    assert json.loads((out / FAILURE_NAME).read_text())["failure_reason"] == "rr_not_restored"


def test_ac1_only_the_computation_is_replaced_and_provenance_is_unchanged(tmp_path, stub_replay, launch, monkeypatch):
    bundle, stage1_out, _keys, _cohort = _stage1(tmp_path, stub_replay)
    out = tmp_path / "stage2"
    fake = evaluation_module._decision_replay_rows
    # 對照組：不經 launcher（stub_replay、RR 已加回），同一個輸出路徑——provenance 的 argv 含路徑，所以先跑完再搬開
    stub_replay["candidate_keys"] = set()
    monkeypatch.setattr(sys, "argv", ["evaluation.py", *_cf(bundle, out, stage1_out)])
    try:
        evaluation_module.main()
    except SystemExit as exc:              # 成功時 main() 正常返回；其他結束碼一律算失敗
        assert exc.code in (0, None)
    control = json.loads((out / Path(OPERATIONAL_BEFORE_SOURCE).name).read_text())["provenance"]
    out.rename(tmp_path / "control")
    monkeypatch.setattr(evaluation_module, "_decision_replay_rows", fake)
    before = {k: id(v) for k, v in vars(evaluation_module).items()}
    assert launch(_cf(bundle, out, stage1_out)) == 0
    after = {k: id(v) for k, v in vars(evaluation_module).items()}
    assert {k for k in before.keys() | after.keys() if before.get(k) != after.get(k)} == {"_decision_replay_rows"}
    assert json.loads((out / Path(OPERATIONAL_BEFORE_SOURCE).name).read_text())["provenance"] == control


@pytest.mark.parametrize("case", ["no-after", "dup-cohort", "eq-form", "no-mode", "bad-compute", "full-failure"])
def test_ac3_launcher_rejects(tmp_path, stub_replay, launch, monkeypatch, case):
    bundle, stage1_out, _keys, _cohort = _stage1(tmp_path, stub_replay)
    out = tmp_path / "stage2"
    argv = _cf(bundle, out, stage1_out)
    kwargs = {}
    if case == "no-after":
        i = argv.index("--after-artifact")
        del argv[i:i + 2]
    elif case == "dup-cohort":
        argv += ["--cohort-manifest", str(stage1_out / COHORT_MANIFEST_NAME)]
    elif case == "eq-form":
        i = argv.index("--after-artifact")
        argv[i:i + 2] = [f"--after-artifact={argv[i + 1]}"]
    elif case == "no-mode":
        monkeypatch.delenv("I074_ACCEPTANCE_REPLAY", raising=False)
        monkeypatch.setenv("I074_ACCEPTANCE_COMPUTE", "stub")
        kwargs["env"] = False
    elif case == "bad-compute":
        kwargs["compute"] = "fast"
    else:
        kwargs.update(mode="failure", compute="full")
    assert launch(argv, **kwargs) == 1
    assert not out.exists() or not any(out.iterdir())


def test_ac3b_full_runs_the_original_computation_then_synthesizes(tmp_path, stub_replay, launch, monkeypatch):
    bundle, stage1_out, _keys, _cohort = _stage1(tmp_path, stub_replay)      # cohort ＝ stub 的候選
    calls = []
    original = evaluation_module._decision_replay_rows

    def spy(*args, **kwargs):
        calls.append(1)
        return original(*args, **kwargs)

    monkeypatch.setattr(evaluation_module, "_decision_replay_rows", spy)
    out = tmp_path / "stage2"
    assert launch(_cf(bundle, out, stage1_out), compute="full") == 0
    assert calls == [1] and {p.name for p in out.iterdir()} == NAMES


def test_ac3b_full_refuses_rows_computed_with_the_counterfactual(tmp_path, stub_replay, launch):
    bundle, stage1_out, _keys, _cohort = _stage1(tmp_path, stub_replay)
    stub_replay["candidate_keys"] = set()        # 模擬 /app 的程式碼套了 counterfactual：cohort 的列⛔ 不再是候選
    out = tmp_path / "stage2"
    assert launch(_cf(bundle, out, stage1_out), compute="full") == 1
    assert not out.exists() or not any(out.iterdir())


# ── ac5：profile ───────────────────────────────────────────────────────────────

def test_ac5_closed_enumerations_and_expected_order():
    acc = sz.PROFILES["acceptance"]
    assert acc["phases"] == ("preflight", "success", "promote_success", "failure", "promote_failure", "memory_only")
    assert acc["disk_phases"] == ("success", "failure") and acc["info_phases"] == ("promote_success", "promote_failure")
    assert acc["roles"] == ("check", "replay", "finalizer", "promotion", "recovery")
    assert sz.expected_invocations("acceptance", "stub") == [
        ("preflight", "check"), ("success", "replay"), ("success", "finalizer"), ("promote_success", "promotion"),
        ("failure", "replay"), ("failure", "finalizer"), ("promote_failure", "promotion"),
        ("memory_only", "check"), ("memory_only", "recovery"), ("memory_only", "recovery"), ("memory_only", "finalizer"),
        ("memory_only", "recovery")]
    assert sz.expected_invocations("acceptance", "full")[-1] == ("memory_only", "replay")
    assert sz.PHASES == sz.PROFILES["sizing"]["phases"] and sz.EXPECTED_ROLES == sz.PROFILES["sizing"]["expected"]
    with pytest.raises(sz.SizingError):
        sz.expected_invocations("acceptance", None)
    with pytest.raises(sz.SizingError):
        sz.profile_spec("other")


def test_ac5_index_rules_per_profile(tmp_path):
    img = tsz.IMG
    with pytest.raises(sz.SizingError):
        sz.write_index(tmp_path, sequence=1, cid="o0010", phase="witness", role="check", included=True,
                       image=img, argv=[img], profile="acceptance")
    with pytest.raises(sz.SizingError):
        sz.write_index(tmp_path, sequence=1, cid="o0010", phase="success", role="fixture", included=True,
                       image=img, argv=[img], profile="acceptance")
    with pytest.raises(sz.SizingError):           # 晉升的窗口：容器足跡要計入（資訊值）
        sz.write_index(tmp_path, sequence=1, cid="o0010", phase="promote_success", role="promotion", included=False,
                       image=img, argv=[img], profile="acceptance")
    entry = sz.write_index(tmp_path, sequence=1, cid="o0010", phase="promote_success", role="promotion", included=True,
                           image=img, argv=[img], profile="acceptance")
    assert entry["profile"] == "acceptance"


MEM_444 = 444 << 20
DEFAULT_LIMIT = (["--memory=444m", "--memory-swap=444m"], MEM_444, MEM_444)


def _acc_state(tmp_path, *, compute="stub", size_rw=4096, peaks=None, limits=None):
    """`limits`：sequence → （argv 裡的記憶體參數、sidecar 的 Memory、sidecar 的 MemorySwap；`...` ＝ 沒有這一欄）。"""
    s = tmp_path / "S"
    for seq, (phase, role) in enumerate(sz.expected_invocations("acceptance", compute), start=1):
        cid = sz.container_id(seq)
        mem_args, mem, swap = (limits or {}).get(seq, DEFAULT_LIMIT)
        argv = [*mem_args, "--cidfile", f"/s/{cid}", "--name", f"n-{cid}", tsz.IMG, "run", str(seq)]
        index = sz.write_index(s, sequence=seq, cid=cid, phase=phase, role=role,
                               included=phase in sz.measured_phases("acceptance"), image=tsz.IMG, argv=argv,
                               profile="acceptance")
        sidecar = {
            "id": cid, "sequence": seq, "container_spec_sha256": index["container_spec_sha256"], "rc": 0,
            "container": "c", "size_rw": size_rw, "log_config": {"Type": "json-file", "Config": {}},
            "log_bound": 4096, "peak_bytes": (peaks or {}).get(seq, 100 << 20),
            "memory_limit_bytes": mem, "memory_swap_limit_bytes": swap, "status": "ok", "failures": []}
        sz.write_exclusive(s / "containers" / f"{cid}.json",
                           sz.canonical_dumps({k: v for k, v in sidecar.items() if v is not ...}))
        sz.write_exclusive(s / "twins" / f"{cid}.json", sz.canonical_dumps({
            "id": cid, "sequence": seq, "container_spec_sha256": index["container_spec_sha256"],
            "raw_bytes": [8192] * 3, "inspect_lengths": [5000] * 3, "adopted_bytes": 131072,
            "status": "ok", "failures": []}))
        sz.record_event(s, cid, "create_begin")
        sz.record_event(s, cid, "rm_done")
    return s


def test_ac5_size_rw_is_counted_in_acceptance_but_still_zero_in_sizing(tmp_path):
    entries = sz.load_invocations(_acc_state(tmp_path), "acceptance", "stub")
    assert {e["sidecar"]["size_rw"] for e in entries} == {4096}
    with pytest.raises(sz.SizingError):                       # 形狀就不符（sizing 的封閉列舉）
        sz.load_invocations(_acc_state(tmp_path / "x"), "sizing")
    with pytest.raises(sz.SizingError):
        sz.load_invocations(tsz._state(tmp_path / "sizing"), "acceptance", "stub")


def test_ac5_index_profile_must_match_this_run(tmp_path):
    s = _acc_state(tmp_path)
    path = s / "index" / "0005.json"
    entry = json.loads(path.read_text())
    entry["profile"] = "sizing"                               # 形狀相符、只有 profile 不同
    path.write_bytes(sz.canonical_dumps(entry))
    with pytest.raises(sz.SizingError, match="profile"):
        sz.load_invocations(s, "acceptance", "stub")


# ── ac6、ac9、ac19：acceptance 的報告 ────────────────────────────────────────────

def _manifest(s, profile):
    lines = []
    for rel in sz.SNAPSHOT_FILES[profile]:
        path = s / "harness" / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(f"content of {rel}".encode())
        lines.append(f"{hashlib.sha256(path.read_bytes()).hexdigest()}  {rel}")
    (s / "harness" / "MANIFEST").write_text("\n".join(lines) + "\n")


DROP_NAMES = ("TOOLING_PATCH", "COUNTERFACTUAL_PATCH", "MEASURE_PEAK", "PATH")
DROP_PREFIXES = ("GIT_", "DOCKER_", "MEM", "SIZING_", "LD_", "PYTHON", "I074_STAGE2_")


def _acc_full_state(tmp_path, *, compute="stub", peaks=None, host=None, sampled=None, promo_mb=3, limits=None):
    s = _acc_state(tmp_path, compute=compute, peaks=peaks, limits=limits)
    (s / "meta.tsv").write_text("".join(f"{k}\t{v}\n" for k, v in {
        "run_id": "r", "mode": "validation", "image": tsz.IMG, "l0_dev": "42", "docker_root_dev": "42", "repo_dev": "42",
        "state_fstype": "tmpfs", "replay_compute": compute, "repo_head": "a" * 40, "clone_head": "a" * 40}.items()))
    (s / "components.tsv").write_text("".join(f"{k}\t{v}\n" for k, v in {
        "wt_head": 30 << 20, "wt_base_patched": 15 << 20, "git_head": 131072, "git_base_patched": 131072,
        "index_head": 98304, "index_base_patched": 98304, "snapshot": 262144, "probe": 24576,
        "frozen_patched": 266240}.items()))
    steps = list(sz.ACCEPTANCE_STEPS) + ([sz.ACCEPTANCE_FULL_STEP] if compute == "full" else [])
    want = {"replay_failure": 6, "publish_failed_record": 1, "promote_failure": 6, "check_failed_record": 2,
            "recover_failed_record": 1}
    (s / "rc.tsv").write_text("".join(f"{n}\t{want.get(n, 0)}\t{want.get(n, 0)}\n" for n, _ in steps))
    for phase, run_mb in (("success", 85), ("failure", 1), ("promote_success", 0), ("promote_failure", 0)):
        pdir = s / "phases" / phase
        pdir.mkdir(parents=True)
        locs = ["L1", "L2", "L3", "L5"] if phase in ("success", "failure") else ["L3", "L5", "L6"]
        base = dict.fromkeys(locs, 0)
        (pdir / "baseline.json").write_bytes(sz.canonical_dumps({
            "locations": base, "L0_used": 10 << 30, "devices": dict.fromkeys(locs + ["L0", "L4"], "42")}))
        sample = {"t_ns": 1, "L0": (10 << 30) + ((run_mb or promo_mb) << 20), **dict.fromkeys(locs, 0)}
        sample[locs[0]] = (run_mb or promo_mb) << 20
        (pdir / "samples.jsonl").write_text(json.dumps(sample) + "\n")
        (pdir / "end.json").write_bytes(sz.canonical_dumps({
            "run_dir": {"allocated": run_mb << 20, "dirs": 3}, "archive": {"allocated": 7 << 20, "dirs": 4},
            "inventory_violations": []}))
    for name, _phase in steps:
        cmd = ["env", "COUNTERFACTUAL_PATCH=/p/c", "TOOLING_PATCH=/p/t", "/repo/scripts/run-replay-offline.sh"] \
            if name.startswith("replay_") else ["/repo/scripts/finalize-stage2-evidence.sh"]
        rec = {"step": name, "rc": want.get(name, 0), "max_single_rss_bytes": (host or {}).get(name, 50 << 20),
               "group_rss_peak_sampled_bytes": (sampled or {}).get(name, 60 << 20), "samples": 3, "interval_s": 0.1,
               "env_keys": ["HOME", "PATH", "PYTHONDONTWRITEBYTECODE", "REPLAY_IMAGE_ID", "SIZING_STATE", "TMPDIR"],
               "cmd": cmd}
        sz.write_exclusive(s / "host" / f"{name}.json", sz.canonical_dumps(rec))
    _manifest(s, "acceptance")
    return s


def _report(s, budget=1 << 40):
    return sz.build_acceptance_report(s, p_b_budget=budget, drop_names=DROP_NAMES, drop_prefixes=DROP_PREFIXES)


def test_ac6_report_ok_and_its_parts(tmp_path):
    r = _report(_acc_full_state(tmp_path))
    assert r["status"] == "ok" and r["violations"] == []
    assert set(r["paths"]) == {"success", "failure"} and set(r["promotion"]) == {"promote_success", "promote_failure"}
    parts = r["paths"]["success"]["accounted_parts"]
    assert parts["runner_frozen_patches"] == 266240
    # 容器足跡 ＝ log 上界 ＋ metadata 採用值 ＋ SizeRw（照實計入）；生命週期可證明不重疊 → 取最大
    assert parts["containers"] == 4096 + 131072 + 4096 and r["paths"]["success"]["container_rule"] == "max_non_overlapping"
    assert r["paths"]["success"]["read_only_gap"] == 2 * 4096                  # SizeRw 照實計入（replay、finalizer）
    assert any("計算工作集⛔ 未涵蓋" in n for n in r["notes"])
    assert set(r["harness_manifest"]) == set(sz.SNAPSHOT_FILES["acceptance"])
    assert sz.acceptance_report_text(r).startswith("status: ok")
    full = _report(_acc_full_state(tmp_path / "full", compute="full"))
    assert full["status"] == "ok" and not any("未涵蓋" in n for n in full["notes"])


@pytest.mark.parametrize("value,status", [(LIMIT, "threshold_exceeded"), (LIMIT - 1, "ok")])
def test_ac6_container_and_host_boundaries(tmp_path, value, status):
    assert _report(_acc_full_state(tmp_path / "c", peaks={2: value}))["status"] == status
    assert _report(_acc_full_state(tmp_path / "h", host={"finalize": value}))["status"] == status


def test_ac6_sampled_group_sum_is_a_one_way_alarm(tmp_path):
    hit = _report(_acc_full_state(tmp_path / "a", sampled={"promote_success": LIMIT}))
    assert hit["status"] == "threshold_exceeded" and any("單向警報" in v for v in hit["violations"])
    below = _report(_acc_full_state(tmp_path / "b", sampled={"promote_success": LIMIT - 1}))
    assert below["status"] == "ok"
    row = next(h for h in below["host"] if h["step"] == "promote_success")
    assert row["group_alarm"] is False and "⛔ 不是通過的證據" in row["group_note"]


def test_ac6_disk_budget_boundary_and_promotion_is_informational(tmp_path):
    s = _acc_full_state(tmp_path, promo_mb=500)          # 晉升很大：⛔ 不影響 status
    p_path = _report(s)["paths"]["success"]["P_path"]
    assert _report(s, budget=p_path)["status"] == "ok"
    over = _report(s, budget=p_path - 1)
    assert over["status"] == "threshold_exceeded" and any("P_path" in v for v in over["violations"])
    assert all("P_promotion" not in v for v in over["violations"])


@pytest.mark.parametrize("damage,match", [
    ("missing_host", "host 端量測"), ("zero_host", "讀不到或為 0"), ("rc", "結束碼"), ("clone_head", "HEAD"),
    ("manifest_extra", "清單"), ("manifest_sha", "MANIFEST 不符"),
])
def test_ac6_report_fails_closed(tmp_path, damage, match):
    s = _acc_full_state(tmp_path)
    if damage == "missing_host":
        (s / "host" / "finalize.json").unlink()
    elif damage == "zero_host":
        path = s / "host" / "finalize.json"
        rec = json.loads(path.read_text())
        rec["max_single_rss_bytes"] = 0
        path.write_bytes(sz.canonical_dumps(rec))
    elif damage == "rc":
        (s / "rc.tsv").write_text((s / "rc.tsv").read_text().replace("finalize\t0\t0", "finalize\t0\t1"))
    elif damage == "clone_head":
        (s / "meta.tsv").write_text((s / "meta.tsv").read_text().replace(f"clone_head\t{'a' * 40}", f"clone_head\t{'b' * 40}"))
    elif damage == "manifest_extra":
        with open(s / "harness" / "MANIFEST", "a") as fh:
            fh.write(f"{'0' * 64}  scripts/extra.sh\n")
    else:
        (s / "harness" / "python" / "scripts" / "i074_stage2_replay_stub.py").write_text("changed")
    with pytest.raises(sz.SizingError, match=match):
        _report(s)


@pytest.mark.parametrize("step,mutate,match", [
    ("finalize", lambda r: r["cmd"].insert(0, "TOOLING_PATCH=/p/t"), "只能給 replay"),
    ("finalize", lambda r: r["env_keys"].append("TOOLING_PATCH"), "clean_env"),
    ("replay_success", lambda r: r.__setitem__("cmd", ["/repo/scripts/run-replay-offline.sh"]), "缺少兩份 patch"),
    ("recover_envcheck", lambda r: r["env_keys"].append("MEASURE_PEAK"), "clean_env"),
    ("check_failed_record", lambda r: r["env_keys"].append("GIT_DIR"), "clean_env"),
])
def test_ac9_environment_contract(tmp_path, step, mutate, match):
    s = _acc_full_state(tmp_path)
    path = s / "host" / f"{step}.json"
    rec = json.loads(path.read_text())
    mutate(rec)
    path.write_bytes(sz.canonical_dumps(rec))
    with pytest.raises(sz.SizingError, match=match):
        _report(s)


def test_ac9_sizing_variables_and_fixed_path_are_allowed():
    rec = {"step": "finalize", "env_keys": ["PATH", "PYTHONDONTWRITEBYTECODE", "SIZING_ROLE", "HOME"],
           "cmd": ["/repo/scripts/finalize-stage2-evidence.sh"]}
    assert sz.check_step_environment(rec, drop_names=DROP_NAMES, drop_prefixes=DROP_PREFIXES) == []


def test_ac19_manifest_binds_every_listed_file(tmp_path):
    s = _acc_full_state(tmp_path)
    base = _report(s)
    for rel in sz.SNAPSHOT_FILES["acceptance"]:
        path = s / "harness" / rel
        path.write_bytes(path.read_bytes() + b"!")
        lines = [f"{hashlib.sha256((s / 'harness' / r).read_bytes()).hexdigest()}  {r}"
                 for r in sz.SNAPSHOT_FILES["acceptance"]]
        (s / "harness" / "MANIFEST").write_text("\n".join(lines) + "\n")
        changed = _report(s)
        assert changed["harness_manifest_sha256"] != base["harness_manifest_sha256"]
        assert {k for k in base["harness_manifest"] if base["harness_manifest"][k] != changed["harness_manifest"][k]} == {rel}
        base = changed


# ── ⑦d 實作第一輪 review #2：每個容器當次的實際記憶體上限（mem-guard 下修後的 --memory） ─────────────────

@pytest.mark.parametrize("text,value", [
    ("444m", MEM_444), ("444M", MEM_444), ("444mb", MEM_444), ("444MiB", MEM_444), ("1g", 1 << 30),
    ("1.5g", 3 << 29), ("512k", 512 << 10), ("465567744", 465567744), ("465567744b", 465567744), ("0", 0),
])
def test_review1_docker_memory_bytes(text, value):
    assert sz.docker_memory_bytes(text) == value


@pytest.mark.parametrize("text", ["", "m", "abc", "-1m", "1x", "1.g", "1 g g", "444m "])
def test_review1_docker_memory_bytes_rejects(text):
    with pytest.raises(sz.SizingError):
        sz.docker_memory_bytes(text)


@pytest.mark.parametrize("tokens,want", [
    (["--memory=444m", "--memory-swap=444m"], ("444m", "444m")),
    (["--memory", "444m", "--memory-swap", "500m"], ("444m", "500m")),
    (["-m", "1g", "--memory-swap=1g"], ("1g", "1g")),
    (["--memory-reservation=1m", "--memory-swappiness=0", "--memory=2g", "--memory-swap=2g"], ("2g", "2g")),
])
def test_review1_argv_memory_options(tokens, want):
    assert sz.argv_memory_options([*tokens, "--name", "x", tsz.IMG, "--memory=9g"], tsz.IMG) == want  # image 之後⛔ 不算


@pytest.mark.parametrize("tokens,match", [
    (["--memory-swap=444m"], "--memory"),
    (["--memory=444m"], "--memory-swap"),
    (["--memory=444m", "-m", "444m", "--memory-swap=444m"], "恰好一個"),
    (["--memory=444m", "--memory-swap=1g", "--memory-swap", "1g"], "恰好一個"),
    (["--memory-swap=1g", "--memory"], "沒有值"),                     # 值的位置就是 image
])
def test_review1_argv_memory_options_rejects(tokens, match):
    with pytest.raises(sz.SizingError, match=match):
        sz.argv_memory_options([*tokens, tsz.IMG], tsz.IMG)


def test_review1_report_records_each_container_memory_limit(tmp_path):
    bounded = lambda r: [n for n in r["notes"] if "上限內" in n]            # noqa: E731 - 上限 ≤ 門檻的容器
    rising = lambda r: [n for n in r["notes"] if "隨當次的上限上升" in n]    # noqa: E731 - 上限 > 門檻的容器
    r = _report(_acc_full_state(tmp_path))
    assert {row["memory_limit_bytes"] for row in r["memory"]} == {MEM_444}
    assert r["limits"]["container_memory_limit_bytes"] == [MEM_444]
    assert len(bounded(r)) == 1 and not rising(r)                        # 上限 444 MiB ≤ 門檻：照實說明門檻判定的實質
    assert "上限 444.0 MiB" in sz.acceptance_report_text(r)
    big = 600 << 20                                                    # 上限各自不同：逐一照實記錄
    r2 = _report(_acc_full_state(tmp_path / "b", limits={
        n: (["--memory=600m", "--memory-swap=600m"], big, big) for n in range(1, 13)}))
    assert {row["memory_limit_bytes"] for row in r2["memory"]} == {big}
    assert r2["limits"]["container_memory_limit_bytes"] == [big] and not bounded(r2) and len(rising(r2)) == 1
    r3 = _report(_acc_full_state(tmp_path / "c", limits={2: (["--memory=600m", "--memory-swap=600m"], big, big)}))
    assert r3["limits"]["container_memory_limit_bytes"] == [MEM_444, big]
    assert next(row for row in r3["memory"] if row["sequence"] == 2)["memory_limit_bytes"] == big
    # 混合：兩則說明各自只列它涵蓋的容器（⛔ 不以最低的上限概括全部）
    assert "#2 " in rising(r3)[0] and "#1、" not in rising(r3)[0]
    assert "#2、" not in bounded(r3)[0] and "#1、#3" in bounded(r3)[0]


@pytest.mark.parametrize("limit,match", [
    ((["--memory=444m", "--memory-swap=444m"], None, MEM_444), "上限讀不到"),
    ((["--memory=444m", "--memory-swap=444m"], ..., ...), "上限讀不到"),           # 舊格式的 sidecar（沒有這兩欄）
    ((["--memory=0", "--memory-swap=0"], 0, 0), "沒有記憶體上限"),
    ((["--memory-swap=444m"], MEM_444, MEM_444), "--memory"),
    ((["--memory=444m", "--memory=444m", "--memory-swap=444m"], MEM_444, MEM_444), "恰好一個"),
    ((["--memory=444m", "--memory-swap=444m"], 400 << 20, 400 << 20), "≠"),            # 封存的 argv ≠ daemon 實際套用的
    ((["--memory=444m", "--memory-swap=888m"], MEM_444, 888 << 20), "swap"),          # 可以用 swap：cgroup 峰值會低估
])
def test_review1_report_fails_closed_on_memory_limit(tmp_path, limit, match):
    with pytest.raises(sz.SizingError, match=match):
        _report(_acc_full_state(tmp_path, limits={5: limit}))


def test_review1_sidecar_records_the_inspected_memory_limit(tmp_path):
    s = tmp_path / "S"
    peak, log = tmp_path / "peak", tmp_path / "log"
    peak.write_text("123\n")
    log.write_bytes(b"")
    kw = {"rc": 0, "container": "c", "size_rw": "0", "log_config_json": '{"Type":"json-file","Config":{}}',
          "stdout_log": log, "stderr_log": log, "peak_file": peak, "failures": []}
    for seq in (1, 2, 3):
        sz.write_index(s, sequence=seq, cid=sz.container_id(seq), phase="success", role="replay", included=True,
                       image=tsz.IMG, argv=[tsz.IMG], profile="acceptance")
    ok = sz.write_sidecar(s, cid=sz.container_id(1), sequence=1, memory_limit="465567744",
                          memory_swap_limit="465567744", **kw)
    assert ok["status"] == "ok" and ok["memory_limit_bytes"] == ok["memory_swap_limit_bytes"] == 465567744
    for seq, (mem, swap) in ((2, ("", "465567744")), (3, ("465567744", "[{}]"))):
        bad = sz.write_sidecar(s, cid=sz.container_id(seq), sequence=seq, memory_limit=mem, memory_swap_limit=swap, **kw)
        assert bad["status"] == "measure_failed" and any("上限讀不到" in f for f in bad["failures"])


# ── ac10：replay argv ──────────────────────────────────────────────────────────

def test_ac10_replay_argv_is_the_single_assembly_with_two_prefixes_mapped(tmp_path):
    clone = tmp_path / "clone"
    clone.mkdir()
    (clone / "python").symlink_to(PYTHON_ROOT)
    anchors = {"bundle_id": "b1_x", "after_base_commit": "e" * 40,
               "after_artifact": "python/baselines/i074_stage1/d1/after_artifact.json.gz",
               "cohort_manifest": "python/baselines/i074_stage1/d1/cohort_manifest.json.gz"}
    got = sz.acceptance_replay_argv(clone, tmp_path / "runs" / "success", anchors)
    pf = sz._load_module("i074_stage2_preflight", PYTHON_ROOT / "scripts" / "i074_stage2_preflight.py")
    pre = {"bundle_id": "b1_x", "base_commit": "e" * 40, "after_artifact": anchors["after_artifact"],
           "cohort_manifest": anchors["cohort_manifest"]}
    want = [t.replace("/W/repo", str(clone)).replace("/W/run/stage2", str(tmp_path / "runs" / "success" / "stage2"))
            for t in pf.replay_argv("/W", pre)]
    assert got == want and got[0] == f"{clone}/scripts/run-replay-offline.sh"


# ── ac12、ac12b：observer 的核心 ────────────────────────────────────────────────

def _fake_docker(tmp_path):
    """fake docker：`ps` 依呼叫次數讀 ps.<n>（沒有就用最後一份）；`inspect` 依 cmd.<id> 回指令。"""
    script = tmp_path / "docker"
    script.write_text(f"""#!{sys.executable}
import json, pathlib, sys
d = pathlib.Path({str(tmp_path)!r})
if sys.argv[1] == "ps":
    n = int((d / "count").read_text()) if (d / "count").exists() else 0
    (d / "count").write_text(str(n + 1))
    files = sorted(d.glob("ps.*"), key=lambda p: int(p.suffix[1:]))
    pick = [p for p in files if int(p.suffix[1:]) <= n] or files[:1]
    sys.stdout.write(pick[-1].read_text())
elif sys.argv[1] == "inspect":
    sys.stdout.write((d / f"cmd.{{sys.argv[-1]}}").read_text())
""")
    script.chmod(0o755)
    return script


def _cg(root, cid, value):
    path = root / "memory" / "docker" / cid / "memory.max_usage_in_bytes"
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(str(value))


ROLE_CMDS = {
    "anchors": ["python", "scripts/i074_stage2_preflight.py", "anchors"],
    "lookup": ["python", "-m", "x.stage2_archive", "--check-failed-record"],
    "replay": ["python", "-m", "backtest.modular.sr_scoring.evaluation"],
    "finalize": ["python", "-m", "x.stage2_archive", "--finalize"],
    "promotion_verify": ["python", "-m", "x.stage2_archive", "--verify-promotion-staging", "/p"],
}


def _observer(tmp_path, *, roles, unreadable=()):
    d = tmp_path / "d"
    d.mkdir()
    cg = tmp_path / "cg"
    ids = [f"{i:064x}" for i in range(1, len(roles) + 1)]
    (d / "ps.0").write_text("")                                 # 第一次：還沒有容器
    (d / "ps.1").write_text("\n".join(ids) + "\n")
    (d / "ps.2").write_text("")                                 # 之後：全部消失
    for cid, role in zip(ids, roles):
        (d / f"cmd.{cid}").write_text(json.dumps(ROLE_CMDS[role]))
        if cid not in {ids[i] for i in unreadable}:
            _cg(cg, cid, 100)
    obs = sz.Observer(tmp_path / "work", docker=str(_fake_docker(d)), cgroup_root=cg, fs_path=str(tmp_path))
    obs.poll_containers(1.0)
    obs.poll_containers(2.0)
    obs.read_peaks(2.5)
    for cid in ids:
        if (cg / "memory" / "docker" / cid).exists():
            _cg(cg, cid, 300)                                   # high-water 變大
    obs.read_peaks(3.0)
    for cid in ids:
        if (cg / "memory" / "docker" / cid).exists():
            _cg(cg, cid, 200)                                   # 讀到較小的值：保留最大
    obs.read_peaks(3.5)
    obs.poll_containers(4.0)
    return obs, ids


def test_ac12_high_water_mark_appearance_and_disappearance(tmp_path):
    obs, ids = _observer(tmp_path, roles=list(ROLE_CMDS))
    report = obs.report()
    assert [c["peak_bytes"] for c in report["containers"]] == [300] * len(ids)
    assert all(c["gone_at"] == 4.0 and c["reads"] == 3 and c["peak_kind"] == "observed_lower_bound"
               for c in report["containers"])
    assert {c["role"] for c in report["containers"]} == set(ROLE_CMDS)
    assert report["observation_complete"] is True and report["missing_expected_containers"] == []


def test_ac12_unreadable_cgroup_is_unavailable_not_zero(tmp_path):
    obs, ids = _observer(tmp_path, roles=list(ROLE_CMDS), unreadable=(2,))
    report = obs.report()
    row = next(c for c in report["containers"] if c["id"] == ids[2])
    assert row["peak_bytes"] is None and ids[2] in report["unavailable_containers"]
    assert report["observation_complete"] is False


def test_ac12b_missing_replay_is_incomplete(tmp_path):
    obs, _ids = _observer(tmp_path, roles=["anchors", "lookup", "finalize", "promotion_verify"])
    report = obs.report()
    assert report["replay_seen"] is False and report["observation_complete"] is False
    assert report["missing_expected_containers"] == ["replay"]


def test_ac12b_started_late_or_stopped_early_is_incomplete(tmp_path):
    obs, _ids = _observer(tmp_path, roles=list(ROLE_CMDS))
    obs.first_poll_empty = False
    assert obs.report()["observation_complete"] is False
    obs.first_poll_empty = True
    obs.last_poll_ids = {"x"}
    assert obs.report()["observation_complete"] is False


def test_ac12_classify_container():
    for role, cmd in ROLE_CMDS.items():
        assert sz.classify_container(cmd) == role
    assert sz.classify_container(["python", "-m", "x", "--publish-failed-record"]) == "publish"
    assert sz.classify_container(["sleep", "1"]) == "other"


@pytest.mark.parametrize("case", ["env-stage2", "env-sizing", "docker-label-shim", "docker-sizing-shim",
                                  "out-in-repo", "out-in-work", "out-exists"])
def test_ac12_observe_guards(tmp_path, case):
    repo, work = tmp_path / "repo", tmp_path / "work"
    repo.mkdir()
    docker = tmp_path / "docker"
    docker.write_text("#!/bin/sh\nexec /usr/bin/docker \"$@\"\n")
    env, out = {"HOME": "/h"}, str(tmp_path / "out")
    if case == "env-stage2":
        env["I074_STAGE2_TOKEN"] = "x"
    elif case == "env-sizing":
        env["SIZING_STATE"] = "x"
    elif case == "docker-label-shim":
        docker.write_text("#!/bin/bash -p\n# I074-STAGE2-LABEL-SHIM\n")
    elif case == "docker-sizing-shim":
        docker.write_text("#!/usr/bin/env bash\n# I074-SIZING-DOCKER-SHIM\n")
    elif case == "out-in-repo":
        out = str(repo / "obs")
    elif case == "out-in-work":
        out = str(work / "obs")
        work.mkdir()
    else:
        (tmp_path / "out").mkdir()
    with pytest.raises(sz.SizingError):
        sz.observe_guards(str(work), out, env, str(docker), repo)
    sz.observe_guards(str(work), str(tmp_path / "ok"), {"HOME": "/h"}, str(tmp_path / "plain"), repo) \
        if (tmp_path / "plain").write_text("#!/bin/sh\n") else None


# ── ac14：sizing 的 runner_frozen_patches ─────────────────────────────────────────

def test_ac14_sizing_accounts_the_runner_frozen_patches(tmp_path):
    report = sz.build_report(tsz._full_state(tmp_path))
    comp = report["components"]
    assert report["paths"]["witness"]["accounted_parts"]["runner_frozen_patches"] == int(comp["frozen_witness"])
    for phase in ("success", "failure"):
        assert report["paths"][phase]["accounted_parts"]["runner_frozen_patches"] == int(comp["frozen_patched"])
    assert set(report["harness_manifest"]) == set(sz.SNAPSHOT_FILES["sizing"])
