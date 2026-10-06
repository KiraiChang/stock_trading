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


def _rss_fields(**override):
    """sidecar 從 rss.json 帶進來的欄位（⑦d 增補）。"""
    rec = {"rss_peak_sampled_bytes": 150 << 20, "rss_samples": 20, "self_max_rss_bytes": 12 << 20,
           "children_max_rss_bytes": 200 << 20, "reaper": "pid1"}
    rec.update(override)
    rec["max_single_rss_bytes"] = max(rec["self_max_rss_bytes"], rec["children_max_rss_bytes"])
    return rec


def _acc_state(tmp_path, *, compute="stub", size_rw=4096, peaks=None, limits=None, rss=None):
    """`limits`：sequence → （argv 裡的記憶體參數、sidecar 的 Memory、sidecar 的 MemorySwap；`...` ＝ 沒有這一欄）；
    `peaks`：sequence → 含 page cache 的 cgroup 峰值（資訊值）；`rss`：sequence → `_rss_fields()` 的覆寫。"""
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
            "memory_limit_bytes": mem, "memory_swap_limit_bytes": swap, **_rss_fields(**(rss or {}).get(seq, {})),
            "status": "ok", "failures": []}
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


def _host_rec(name, rc, *, children, sampled, cmd, self_rss=20 << 20, **override):
    """⑦d 增補：`host-run` 紀錄的封閉 schema（十七個鍵）。"""
    rec = {"schema": "i074_stage2_host_run_v1", "step": name, "rc": rc, "self_max_rss_bytes": self_rss,
           "children_max_rss_bytes": children, "max_single_rss_bytes": max(self_rss, children),
           "group_rss_peak_sampled_bytes": sampled, "group_samples": 3, "group_interval_ms": 100, "reaper": "subreaper",
           "all_descendants_reaped": True, "auto_reap_detected": False, "cleanup_complete": True, "leftover_pids": [],
           "errors": [], "env_keys": ["HOME", "PATH", "PYTHONDONTWRITEBYTECODE", "REPLAY_IMAGE_ID", "SIZING_STATE", "TMPDIR"],
           "cmd": cmd}
    rec.update(override)
    return rec


def _acc_full_state(tmp_path, *, compute="stub", peaks=None, host=None, sampled=None, promo_mb=3, limits=None, rss=None):
    s = _acc_state(tmp_path, compute=compute, peaks=peaks, limits=limits, rss=rss)
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
        rec = _host_rec(name, want.get(name, 0), children=(host or {}).get(name, 50 << 20),
                        sampled=(sampled or {}).get(name, 60 << 20), cmd=cmd)
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
    """⑦d 增補改寫：容器那一半改成精確閘與偵測器的邊界；host 那一半照舊。"""
    assert _report(_acc_full_state(tmp_path / "c", rss={2: {"children_max_rss_bytes": value}}))["status"] == status
    assert _report(_acc_full_state(tmp_path / "s", rss={2: {"rss_peak_sampled_bytes": value}}))["status"] == status
    assert _report(_acc_full_state(tmp_path / "h", host={"finalize": value}))["status"] == status


def test_ac6_cgroup_peak_with_page_cache_is_informational_only(tmp_path):
    """⑦d 增補：含 page cache 的 cgroup 峰值 ＝ 門檻 → ⛔ 不影響 status（資訊值）。"""
    r = _report(_acc_full_state(tmp_path, peaks={2: LIMIT, 5: LIMIT + 1}))
    assert r["status"] == "ok" and r["violations"] == []
    assert next(row for row in r["memory"] if row["sequence"] == 2)["cgroup_peak_bytes"] == LIMIT


def test_ac6_sampled_group_sum_is_a_one_way_alarm(tmp_path):
    hit = _report(_acc_full_state(tmp_path / "a", sampled={"promote_success": LIMIT}))
    assert hit["status"] == "threshold_exceeded" and [v["kind"] for v in hit["violations"]] == ["host_group_rss_sampled"]
    below = _report(_acc_full_state(tmp_path / "b", sampled={"promote_success": LIMIT - 1}))
    assert below["status"] == "ok"
    row = next(h for h in below["host"] if h["step"] == "promote_success")
    assert row["group_alarm"] is False and "⛔ 不是通過的證據" in row["group_note"]


def test_ac6_disk_budget_boundary_and_promotion_is_informational(tmp_path):
    s = _acc_full_state(tmp_path, promo_mb=500)          # 晉升很大：⛔ 不影響 status
    p_path = _report(s)["paths"]["success"]["P_path"]
    assert _report(s, budget=p_path)["status"] == "ok"
    over = _report(s, budget=p_path - 1)
    assert over["status"] == "threshold_exceeded" and [v["kind"] for v in over["violations"]] == ["disk_p_path"]
    assert over["violations"][0] == {"kind": "disk_p_path", "subject": "success", "value": p_path, "limit": p_path - 1}


@pytest.mark.parametrize("damage,match", [
    ("missing_host", "host 端量測"), ("zero_host", "max_single_rss_bytes"), ("rc", "結束碼"), ("clone_head", "HEAD"),
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
    ("finalize", lambda r: r.__setitem__("env_keys", sorted(r["env_keys"] + ["TOOLING_PATCH"])), "clean_env"),
    ("replay_success", lambda r: r.__setitem__("cmd", ["/repo/scripts/run-replay-offline.sh"]), "缺少兩份 patch"),
    ("recover_envcheck", lambda r: r.__setitem__("env_keys", sorted(r["env_keys"] + ["MEASURE_PEAK"])), "clean_env"),
    ("check_failed_record", lambda r: r.__setitem__("env_keys", sorted(r["env_keys"] + ["GIT_DIR"])), "clean_env"),
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
    """第一輪 review #2 ＋ ⑦d 增補「二」③：逐列記上限；notes 只列上限**嚴格低於**門檻的容器。"""
    bounded = lambda r: [n for n in r["notes"] if "嚴格低於門檻" in n]       # noqa: E731
    r = _report(_acc_full_state(tmp_path))
    assert {row["memory_limit_bytes"] for row in r["memory"]} == {MEM_444}
    assert r["limits"]["container_memory_limit_bytes"] == [MEM_444]
    assert len(bounded(r)) == 1 and "精確的閘仍逐一比較" in bounded(r)[0]
    assert any("cgroup 峰值只列資訊值" in n for n in r["notes"])
    assert "上限 444.0 MiB" in sz.acceptance_report_text(r)
    big = 600 << 20                                                    # 上限各自不同：逐一照實記錄
    r2 = _report(_acc_full_state(tmp_path / "b", limits={
        n: (["--memory=600m", "--memory-swap=600m"], big, big) for n in range(1, 13)}))
    assert {row["memory_limit_bytes"] for row in r2["memory"]} == {big}
    assert r2["limits"]["container_memory_limit_bytes"] == [big] and not bounded(r2)
    r3 = _report(_acc_full_state(tmp_path / "c", limits={2: (["--memory=600m", "--memory-swap=600m"], big, big)}))
    assert r3["limits"]["container_memory_limit_bytes"] == [MEM_444, big]
    assert next(row for row in r3["memory"] if row["sequence"] == 2)["memory_limit_bytes"] == big
    assert "#2、" not in bounded(r3)[0] and "#1、#3" in bounded(r3)[0]
    # 上限 ＝ 門檻（450m）的容器⛔ 不列（判定是嚴格 <）
    r4 = _report(_acc_full_state(tmp_path / "d", limits={3: (["--memory=450m", "--memory-swap=450m"], LIMIT, LIMIT)}))
    assert "#3、" not in bounded(r4)[0] and "#2、#4" in bounded(r4)[0]


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
    (tmp_path / "rss.json").write_text(json.dumps(_rss_ok()))      # ⑦d 增補：acceptance 的 sidecar 必須有有效的 rss.json
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


def _cg(root, cid, value, rss=True):
    path = root / "memory" / "docker" / cid / "memory.max_usage_in_bytes"
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(str(value))
    if rss:                                                     # ⑦d 增補：v1 的 memory.stat（total_rss ＝ 峰值的一半）
        (path.parent / "memory.stat").write_text(f"cache 1\ntotal_cache 2\ntotal_rss {value // 2}\n")


ROLE_CMDS = {
    "anchors": ["python", "scripts/i074_stage2_preflight.py", "anchors"],
    "lookup": ["python", "-m", "x.stage2_archive", "--check-failed-record"],
    "replay": ["python", "-m", "backtest.modular.sr_scoring.evaluation"],
    "finalize": ["python", "-m", "x.stage2_archive", "--finalize"],
    "promotion_verify": ["python", "-m", "x.stage2_archive", "--verify-promotion-staging", "/p"],
}


def _observer(tmp_path, *, roles, unreadable=(), no_rss=()):
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
            _cg(cg, cid, 100, rss=cid not in {ids[i] for i in no_rss})
    obs = sz.Observer(tmp_path / "work", docker=str(_fake_docker(d)), cgroup_root=cg, fs_path=str(tmp_path))
    obs.poll_containers(1.0)
    obs.poll_containers(2.0)
    obs.read_peaks(2.5)
    for cid in ids:
        if (cg / "memory" / "docker" / cid).exists():
            _cg(cg, cid, 300, rss=cid not in {ids[i] for i in no_rss})     # high-water 變大
    obs.read_peaks(3.0)
    for cid in ids:
        if (cg / "memory" / "docker" / cid).exists():
            _cg(cg, cid, 200, rss=cid not in {ids[i] for i in no_rss})     # 讀到較小的值：保留最大
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


def test_ac23_observer_samples_total_rss_as_a_lower_bound(tmp_path):
    """⑦d 增補「二」⑤：observer 另記 v1 memory.stat 的 total_rss（取樣的最大值、讀取次數）。"""
    obs, ids = _observer(tmp_path, roles=list(ROLE_CMDS))
    report = obs.report()
    assert [c["rss_peak_bytes"] for c in report["containers"]] == [150] * len(ids)
    assert all(c["rss_reads"] == 3 for c in report["containers"])
    assert report["rss_unavailable_containers"] == [] and report["observation_complete"] is True
    assert any("total_rss" in n for n in report["notes"])
    assert "total_rss" in sz.observation_text(report)


def test_ac23_unreadable_total_rss_is_incomplete_and_v2_is_never_read(tmp_path):
    obs, ids = _observer(tmp_path, roles=list(ROLE_CMDS), no_rss=(1,))
    v2 = tmp_path / "cg" / "docker" / ids[1]                    # 只有 v2 的位置有 memory.stat：⛔ 不讀
    v2.mkdir(parents=True)
    (v2 / "memory.stat").write_text("anon 999\n")
    obs.read_peaks(3.8)
    report = obs.report()
    row = next(c for c in report["containers"] if c["id"] == ids[1])
    assert row["rss_peak_bytes"] is None and report["rss_unavailable_containers"] == [ids[1]]
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


# ── ⑦d 增補（issue.md I-074「Stage 2 步驟 ⑦d 增補計畫：容器記憶體改以 RSS 判定」「五」）：ac20 容器內的 wrapper ─────────

import signal as _signal  # noqa: E402
import subprocess  # noqa: E402
import textwrap  # noqa: E402
import time  # noqa: E402

WRAPPER = PYTHON_ROOT / "scripts" / "i074_stage2_rss_wrapper.py"
MIB = 1024 * 1024
_rw_spec = importlib.util.spec_from_file_location("i074_stage2_rss_wrapper", WRAPPER)
rw = importlib.util.module_from_spec(_rw_spec)
_rw_spec.loader.exec_module(rw)


def _cgroup(tmp_path, *, total_rss=4096, v1=True, v2=False):
    root = tmp_path / "cg"
    if v1:
        (root / "memory").mkdir(parents=True)
        (root / "memory" / "memory.stat").write_text(f"cache 1\nrss 2\ntotal_cache 3\ntotal_rss {total_rss}\n")
        (root / "memory" / "memory.max_usage_in_bytes").write_text("123456789\n")
    if v2:
        root.mkdir(parents=True, exist_ok=True)
        (root / "memory.stat").write_text("anon 777\nfile 888\n")
        (root / "memory.peak").write_text("999\n")
    return root


def _wrap(tmp_path, leader_code, *, pre_alloc_mb=0, grace_s=10.0, cgroup=None, timeout=60):
    """以 driver 執行 wrapper（⛔ 不是 PID 1 → subreaper）；回傳 (rc, stdout, stderr, rss.json)。"""
    cgroup = cgroup if cgroup is not None else _cgroup(tmp_path)
    peak = tmp_path / "peak"
    peak.mkdir(exist_ok=True)
    driver = textwrap.dedent(f"""
        import sys
        sys.path.insert(0, {str(WRAPPER.parent)!r})
        import i074_stage2_rss_wrapper as w
        keep = bytearray({pre_alloc_mb} * 1024 * 1024)
        sys.exit(w.run({[sys.executable, "-c", leader_code]!r}, cgroup_root={str(cgroup)!r}, peak_dir={str(peak)!r},
                       grace_s={grace_s}))
    """)
    proc = subprocess.run([sys.executable, "-c", driver], capture_output=True, timeout=timeout,
                          env=dict(os.environ, PYTHONDONTWRITEBYTECODE="1"))
    record = json.loads((peak / "rss.json").read_text()) if (peak / "rss.json").exists() else None
    return proc.returncode, proc.stdout, proc.stderr, record


def _kill_pid_file(path):
    """測試自己啟動、寫下 PID 的孤兒：以 PID 收掉（⛔ 不用 pkill -f）。"""
    if path.exists():
        try:
            os.kill(int(path.read_text()), _signal.SIGKILL)
        except ProcessLookupError:
            pass


def test_ac20_sampled_total_rss_is_the_max_seen_and_the_record_is_closed(tmp_path):
    cg = _cgroup(tmp_path, total_rss=100)
    stat = cg / "memory" / "memory.stat"
    code = textwrap.dedent(f"""
        import time, pathlib
        p = pathlib.Path({str(stat)!r})
        for v in (100, 300 * 1024 * 1024, 200):
            p.write_text("total_rss %d\\n" % v)
            time.sleep(0.4)
    """)
    rc, out, err, rec = _wrap(tmp_path, code, cgroup=cg)
    assert (rc, out, err) == (0, b"", b"")
    assert tuple(sorted(rec)) == tuple(sorted(rw.RECORD_KEYS))
    assert rec["rss_peak_sampled_bytes"] == 300 * MIB and rec["rss_samples"] > 3
    assert rec["rss_source"] == "v1:total_rss" and rec["rss_interval_ms"] == 50 and rec["reaper"] == "subreaper"
    assert rec["all_descendants_reaped"] is True and rec["auto_reap_detected"] is False and rec["errors"] == []
    assert rec["max_single_rss_bytes"] == max(rec["self_max_rss_bytes"], rec["children_max_rss_bytes"])
    assert (tmp_path / "peak" / "peak").read_text() == "123456789\n"          # cgroup 峰值（資訊值），格式與 sizing 相同


def test_ac20_exact_gate_counts_the_leader_and_the_wrapper_itself(tmp_path):
    rc, _, _, rec = _wrap(tmp_path / "a", "x = bytearray(64 * 1024 * 1024)")
    assert rc == 0 and rec["children_max_rss_bytes"] >= 64 * MIB
    rc, _, _, rec = _wrap(tmp_path / "b", "pass", pre_alloc_mb=96)          # wrapper 自己膨脹
    assert rc == 0 and rec["self_max_rss_bytes"] >= 96 * MIB and rec["max_single_rss_bytes"] >= 96 * MIB
    # ⚠️ fork 之後、exec 之前，子程序與 parent 共用常駐頁：它的 ru_maxrss 含 parent 當時的 RSS（實測），但⛔ 不會超過
    #   parent 自己的 high-water——所以取大的結果不變（偏保守）。
    assert rec["children_max_rss_bytes"] <= rec["self_max_rss_bytes"] + 8 * MIB


def test_ac20_unwaited_descendant_is_adopted_reaped_and_counted(tmp_path):
    code = textwrap.dedent("""
        import subprocess, sys, os
        subprocess.Popen([sys.executable, "-c", "import time; x = bytearray(64 * 1024 * 1024); time.sleep(0.5)"])
        os._exit(0)                                   # ⛔ 不 wait 就結束：孫程序成了孤兒
    """)
    rc, _, _, rec = _wrap(tmp_path, code, grace_s=10)
    assert rc == 0 and rec["all_descendants_reaped"] is True and rec["errors"] == []
    assert rec["children_max_rss_bytes"] >= 64 * MIB


def test_ac20_live_orphan_beyond_the_grace_is_a_measurement_error(tmp_path):
    pid_file = tmp_path / "orphan.pid"
    code = textwrap.dedent(f"""
        import subprocess, os
        p = subprocess.Popen(["sleep", "30"])
        open({str(pid_file)!r}, "w").write(str(p.pid))
        os._exit(4)
    """)
    try:
        rc, _, _, rec = _wrap(tmp_path, code, grace_s=1)
    finally:
        _kill_pid_file(pid_file)
    assert rc == 4                                                  # 結束碼照傳
    assert rec["all_descendants_reaped"] is False and any("存活的子孫" in e for e in rec["errors"])


def test_ac20_sigchld_ignored_parent_is_detected_and_its_child_is_not_in_the_exact_gate(tmp_path):
    """反例：parent 把 SIGCHLD 設成 SIG_IGN，它的子程序被 kernel 自動回收——確實⛔ 不在精確閘裡，而且被偵測。"""
    code = textwrap.dedent("""
        import signal, subprocess, sys, time
        signal.signal(signal.SIGCHLD, signal.SIG_IGN)
        subprocess.Popen([sys.executable, "-c", "x = bytearray(96 * 1024 * 1024); import time; time.sleep(0.3)"])
        time.sleep(1.2)
    """)
    rc, _, _, rec = _wrap(tmp_path, code)
    assert rc == 0
    assert rec["auto_reap_detected"] is True                          # ⛔ 不會被當成通過
    assert rec["children_max_rss_bytes"] < 96 * MIB                   # 自動回收：resource usage 被丟棄


@pytest.mark.parametrize("code,want", [("import sys; sys.exit(3)", 3), ("import os; os.kill(os.getpid(), 9)", 137)])
def test_ac20_exit_code_is_passed_through(tmp_path, code, want):
    rc, _, _, rec = _wrap(tmp_path, code)
    assert rc == want and rec["errors"] == []


def test_ac20_io_is_transparent(tmp_path):
    rc, out, err, _ = _wrap(tmp_path, "import sys; sys.stdout.write('OUT\\n'); sys.stderr.write('ERR\\n')")
    assert (rc, out, err) == (0, b"OUT\n", b"ERR\n")


@pytest.mark.parametrize("v1,v2", [(False, False), (False, True)], ids=["none", "v2-only"])
def test_ac20_v1_total_rss_is_required_and_v2_anon_is_never_read(tmp_path, v1, v2):
    rc, _, _, rec = _wrap(tmp_path, "import sys; sys.exit(5)", cgroup=_cgroup(tmp_path, v1=v1, v2=v2))
    assert rc == 5 and rec["rss_samples"] == 0 and rec["rss_peak_sampled_bytes"] == 0
    assert any("total_rss" in e for e in rec["errors"]) and rec["rss_source"] == "v1:total_rss"


def test_ac20_missing_command_is_127_like_sh(tmp_path):
    peak = tmp_path / "peak"
    peak.mkdir()
    rc = rw.run(["/nonexistent/command"], cgroup_root=_cgroup(tmp_path), peak_dir=peak, grace_s=1)
    assert rc == 127 and json.loads((peak / "rss.json").read_text())["errors"]


# ── ac21：rss.json 的封閉 schema（sidecar 逐條驗） ─────────────────────────────────

def _rss_ok(**override):
    rec = {"schema": "i074_stage2_rss_v1", "rss_source": "v1:total_rss", "rss_interval_ms": 50, "rss_samples": 12,
           "rss_peak_sampled_bytes": 200 * MIB, "self_max_rss_bytes": 12 * MIB, "children_max_rss_bytes": 300 * MIB,
           "max_single_rss_bytes": 300 * MIB, "reaper": "pid1", "all_descendants_reaped": True,
           "auto_reap_detected": False, "errors": []}
    rec.update(override)
    return rec


def test_ac21_helper_constants_equal_the_wrappers():
    assert sz.RSS_RECORD_SCHEMA == rw.SCHEMA and sz.RSS_SOURCE == rw.RSS_SOURCE
    assert sz.RSS_INTERVAL_MS == rw.INTERVAL_MS and sz.RSS_RECORD_KEYS == rw.RECORD_KEYS


def test_ac21_valid_record_has_no_problems():
    assert sz.rss_record_problems(_rss_ok(), require_pid1=True) == []
    assert sz.rss_record_problems(_rss_ok(reaper="subreaper"), require_pid1=False) == []


@pytest.mark.parametrize("mutate", [
    lambda r: r.pop("errors"),                                         # 缺欄
    lambda r: r.__setitem__("extra", 1),                               # 多欄
    lambda r: r.__setitem__("rss_samples", True),                      # bool 當整數
    lambda r: r.__setitem__("all_descendants_reaped", 1),              # 整數當布林
    lambda r: r.__setitem__("rss_samples", 0),
    lambda r: r.__setitem__("rss_peak_sampled_bytes", 0),
    lambda r: r.__setitem__("self_max_rss_bytes", 0),
    lambda r: r.__setitem__("rss_interval_ms", 100),
    lambda r: r.__setitem__("rss_interval_ms", 50.0),
    lambda r: r.__setitem__("rss_source", "v2:anon"),
    lambda r: r.__setitem__("schema", "i074_stage2_rss_v0"),
    lambda r: r.__setitem__("reaper", "subreaper"),                    # 容器內必須是 pid1
    lambda r: r.__setitem__("reaper", "init"),
    lambda r: r.__setitem__("errors", ["x"]),
    lambda r: r.__setitem__("errors", [1]),
    lambda r: r.__setitem__("max_single_rss_bytes", 299 * MIB),        # ≠ 兩者取大
    lambda r: r.__setitem__("all_descendants_reaped", False),
    lambda r: r.__setitem__("auto_reap_detected", True),
])
def test_ac21_each_tampered_field_is_a_problem(mutate):
    rec = _rss_ok()
    mutate(rec)
    assert sz.rss_record_problems(rec, require_pid1=True)


def _sidecar_with_rss(tmp_path, profile, rss):
    s = tmp_path / "S"
    sz.write_index(s, sequence=1, cid="o0010", phase="success", role="replay" if profile == "acceptance" else "finalizer",
                   included=True, image=tsz.IMG, argv=[tsz.IMG], profile=profile)
    peak_dir = tmp_path / "peakdir"
    peak_dir.mkdir()
    (peak_dir / "peak").write_text("123\n")
    if rss is not None:
        (peak_dir / "rss.json").write_text(json.dumps(rss))
    log = tmp_path / "log"
    log.write_bytes(b"")
    return sz.write_sidecar(s, cid="o0010", sequence=1, rc=0, container="c", size_rw="0",
                            log_config_json='{"Type":"json-file","Config":{}}', stdout_log=log, stderr_log=log,
                            peak_file=peak_dir / "peak", failures=[], memory_limit="465567744",
                            memory_swap_limit="465567744")


def test_ac21_acceptance_sidecar_reads_and_validates_rss_json(tmp_path):
    ok = _sidecar_with_rss(tmp_path / "ok", "acceptance", _rss_ok())
    assert ok["status"] == "ok" and ok["max_single_rss_bytes"] == 300 * MIB and ok["rss_samples"] == 12
    assert ok["rss_peak_sampled_bytes"] == 200 * MIB and ok["reaper"] == "pid1"
    bad = _sidecar_with_rss(tmp_path / "bad", "acceptance", _rss_ok(auto_reap_detected=True))
    assert bad["status"] == "measure_failed" and any("rss.json" in f for f in bad["failures"])
    missing = _sidecar_with_rss(tmp_path / "missing", "acceptance", None)
    assert missing["status"] == "measure_failed"


def test_ac21_sizing_sidecar_neither_reads_nor_requires_rss_json(tmp_path):
    entry = _sidecar_with_rss(tmp_path, "sizing", None)
    assert entry["status"] == "ok" and "max_single_rss_bytes" not in entry


# ── ac22：報告 v2（封閉欄位、結構化的 violations、validator 的封閉 schema 與語意交叉） ─────────────────

import copy  # noqa: E402

BIG_BUDGET = 1 << 40


def test_ac22_report_v2_fixed_fields_and_structured_violations(tmp_path):
    r = _report(_acc_full_state(tmp_path))
    assert r["schema"] == "i074_stage2_acceptance_report_v2"
    assert r["contract"] == "per_process_rss_within_wait_chain"
    assert r["out_of_contract"] == ["descendants_auto_reaped_by_kernel"]
    assert r["auto_reap_detection"] == {"sigchld_sig_ign": "sampled_fail_closed", "sa_nocldwait": "unobservable"}
    assert r["memory_measures"] == {"max_single_rss_bytes": "exact_within_wait_chain",
                                    "rss_peak_sampled_bytes": "sampled_lower_bound",
                                    "cgroup_peak_bytes": "informational_includes_page_cache"}
    assert sz.validate_acceptance_report_v2(r, p_b_budget=BIG_BUDGET) == []
    hit = _report(_acc_full_state(tmp_path / "x", rss={2: {"children_max_rss_bytes": LIMIT}}))
    assert hit["violations"] == [{"kind": "container_max_single_rss", "subject": "#2 success/replay", "value": LIMIT,
                                  "limit": LIMIT}]
    assert "container_max_single_rss" in sz.acceptance_report_text(hit)


def _violating(tmp_path):
    return _report(_acc_full_state(tmp_path, rss={2: {"children_max_rss_bytes": LIMIT}}))


def _set(path, value):
    def mutate(r):
        node = r
        for key in path[:-1]:
            node = node[key]
        node[path[-1]] = value
    return mutate


def _pop(path):
    def mutate(r):
        node = r
        for key in path[:-1]:
            node = node[key]
        node.pop(path[-1])
    return mutate


@pytest.mark.parametrize("mutate", [
    _pop(["notes"]), _set(["extra"], 1), _set(["schema"], "i074_stage2_acceptance_report_v1"),
    _set(["contract"], "per_process"), _set(["out_of_contract"], "descendants_auto_reaped_by_kernel"),
    _set(["out_of_contract"], ["descendants_auto_reaped_by_kernel", "x"]),
    _set(["auto_reap_detection", "sa_nocldwait"], "detected"), _set(["memory_measures", "cgroup_peak_bytes"], "exact"),
    _set(["status"], "maybe"), _set(["mode"], "x"), _set(["replay_compute"], "fast"),
    _pop(["memory", 0, "reaper"]), _set(["memory", 0, "extra"], 1), _set(["memory", 0, "rss_samples"], True),
    _set(["memory", 0, "reaper"], "subreaper"), _pop(["host", 0, "group_note"]), _set(["host", 0, "rc"], True),
    _pop(["paths", "success", "P_path"]), _set(["paths", "success", "peaks", "extra"], 1),
    _set(["promotion", "promote_success", "kind"], "x"), _set(["limits", "extra"], 1),
    _set(["host_memavailable_low"], "1"),
], ids=lambda f: None)
def test_ac22_validator_rejects_schema_tampering(tmp_path, mutate):
    r = _report(_acc_full_state(tmp_path))
    mutate(r)
    assert sz.validate_acceptance_report_v2(r, p_b_budget=BIG_BUDGET)


def test_ac22_validator_rejects_semantic_cross_tampering(tmp_path):
    ok = _report(_acc_full_state(tmp_path / "ok"))
    bad = _violating(tmp_path / "bad")
    cases = []
    r = copy.deepcopy(ok); r["violations"].append(copy.deepcopy(bad["violations"][0])); cases.append(r)          # ok ＋ 違反
    r = copy.deepcopy(bad); r["violations"] = []; cases.append(r)                                                # 超標 ＋ 空
    r = copy.deepcopy(bad); r["violations"] = []; r["status"] = "ok"; cases.append(r)                           # 刪掉違反
    r = copy.deepcopy(ok)
    r["memory"][1]["children_max_rss_bytes"] = r["memory"][1]["max_single_rss_bytes"] = LIMIT
    r["memory"][1]["below_limit"] = False
    cases.append(r)                                                                                            # 列達門檻、沒有違反
    r = copy.deepcopy(ok); r["paths"]["success"]["P_path"] = BIG_BUDGET + 1; cases.append(r)
    for case in cases:
        assert sz.validate_acceptance_report_v2(case, p_b_budget=BIG_BUDGET)


def test_ac22_validator_pins_the_formal_budget(tmp_path):
    """預算調高、移除磁碟的違反、其餘一致 → 拒絕（它必須等於正式常數）。"""
    s = _acc_full_state(tmp_path)
    p_path = _report(s)["paths"]["success"]["P_path"]
    over = _report(s, budget=p_path - 1)
    assert [v["kind"] for v in over["violations"]] == ["disk_p_path"]
    forged = copy.deepcopy(over)
    forged["limits"]["P_B_BUDGET"] = p_path
    forged["violations"], forged["status"] = [], "ok"
    assert sz.validate_acceptance_report_v2(forged, p_b_budget=p_path - 1)
    assert sz.validate_acceptance_report_v2(over, p_b_budget=p_path - 1) == []


@pytest.mark.parametrize("mutate", [
    _set(["memory", 0, "self_max_rss_bytes"], 500 * MIB),                       # self 改大、max_single 不變
    _set(["host", 0, "self_max_rss_bytes"], 500 * MIB),
    _set(["paths", "success", "accounted"], 1),
    _set(["paths", "success", "P_path"], 1),
    _set(["promotion", "promote_success", "P_promotion"], 1),
    _set(["promotion", "promote_failure", "P_path_plus_promotion"], 1),
    _set(["memory", 0, "below_limit"], False),
    _set(["host", 0, "group_alarm"], True),
    _set(["limits", "container_memory_limit_bytes"], [1]),
    _set(["limits", "memory_bytes"], LIMIT + 1),
], ids=lambda f: None)
def test_ac22_validator_rejects_derived_field_tampering(tmp_path, mutate):
    r = _report(_acc_full_state(tmp_path))
    mutate(r)
    assert sz.validate_acceptance_report_v2(r, p_b_budget=BIG_BUDGET)


def test_ac22_producer_and_validator_share_one_derivation(tmp_path, monkeypatch):
    """產出端與 validator 呼叫同一個 derive_acceptance_violations()：改它，兩邊一起變（⛔ 不是兩份門檻邏輯）。"""
    extra = {"kind": "disk_p_path", "subject": "success", "value": 1, "limit": 0}
    original = sz.derive_acceptance_violations
    monkeypatch.setattr(sz, "derive_acceptance_violations", lambda r: original(r) + [extra])
    r = _report(_acc_full_state(tmp_path))
    assert r["status"] == "threshold_exceeded" and r["violations"] == [extra]
    assert sz.validate_acceptance_report_v2(r, p_b_budget=BIG_BUDGET) == []


def _leaf_paths(node, prefix=()):
    """報告裡每一個節點（含容器本身）的路徑。"""
    yield prefix
    if isinstance(node, dict):
        for key, value in node.items():
            yield from _leaf_paths(value, prefix + (key,))
    elif isinstance(node, list):
        for i, value in enumerate(node):
            yield from _leaf_paths(value, prefix + (i,))


def _wrong_types(value):
    """與原值型別不同的替代值（bool 與 int 互換也算錯）。"""
    if type(value) is bool:
        return [1, "x", None]
    if type(value) is int:
        return ["x", None, True, 1.5, []]
    if isinstance(value, str):
        return [1, None, []]
    if isinstance(value, list):
        return ["x", {}, None]
    if isinstance(value, dict):
        return ["x", [], None]
    return ["x"]


@pytest.mark.parametrize("which", ["ok", "violating"])
def test_ac22_validator_is_closed_over_every_nested_type(tmp_path, which):
    """實作第一輪 review：每一個節點換成錯的型別 → validator 一律回傳問題清單、⛔ 不拋例外、⛔ 不放行。"""
    base = _report(_acc_full_state(tmp_path)) if which == "ok" else _violating(tmp_path)
    skipped = {("host_memavailable_low",)}                     # 允許 null（另有一支測它的型別）
    checked = 0
    for path in list(_leaf_paths(base)):
        if not path or path in skipped:
            continue
        node = base
        for key in path[:-1]:
            node = node[key]
        for wrong in _wrong_types(node[path[-1]]):
            r = copy.deepcopy(base)
            target = r
            for key in path[:-1]:
                target = target[key]
            target[path[-1]] = wrong
            problems = sz.validate_acceptance_report_v2(r, p_b_budget=BIG_BUDGET)
            assert problems, (path, wrong)
            checked += 1
    assert checked > 500                                          # ⛔ 不是空的矩陣


@pytest.mark.parametrize("mutate", [
    _set(["limits", "container_memory_limit_bytes"], "not-a-list"),
    _set(["limits", "container_memory_limit_bytes"], [True]),
    _set(["paths", "success", "peaks"], []),
    _set(["paths", "success", "peaks", "dirs_peak"], "x"),
    _set(["paths", "success", "peaks", "dirs_peak_by_location", "L1"], "x"),
    _set(["paths", "success", "accounted"], "1"),
    _set(["paths", "success"], []),
    _set(["promotion", "promote_success", "P_promotion"], "x"),
    _set(["promotion", "promote_success", "peaks"], None),
    _set(["host", 0, "group_alarm"], []),
    _set(["memory", 0, "self_max_rss_bytes"], "x"),
    _set(["host_memavailable_low"], True),
], ids=lambda f: None)
def test_ac22_validator_reports_wrong_nested_types_without_raising(tmp_path, mutate):
    r = _report(_acc_full_state(tmp_path))
    mutate(r)
    assert sz.validate_acceptance_report_v2(r, p_b_budget=BIG_BUDGET)


def test_ac22_validator_checks_violation_field_types(tmp_path):
    bad = _violating(tmp_path)
    for key, wrong in (("value", "x"), ("limit", True), ("subject", 1)):
        r = copy.deepcopy(bad)
        r["violations"][0][key] = wrong
        assert sz.validate_acceptance_report_v2(r, p_b_budget=BIG_BUDGET), key


def test_ac22_build_refuses_to_write_an_invalid_report(tmp_path, monkeypatch):
    """產出端一定經過 validator：validator 不通過 → ⛔ 不產報告（harness 失敗）。"""
    monkeypatch.setattr(sz, "validate_acceptance_report_v2", lambda r, *, p_b_budget: ["注入的問題"])
    with pytest.raises(sz.SizingError, match="注入的問題"):
        _report(_acc_full_state(tmp_path))


# ── ac24c：host 紀錄的封閉 schema ──────────────────────────────────────────────

def _host_ok(**override):
    return _host_rec("finalize", 0, children=50 << 20, sampled=60 << 20, cmd=["/repo/x.sh"], **override)


def test_ac24c_valid_host_record_has_no_problems():
    assert sz.host_record_problems(_host_ok(), step="finalize", rc=0) == []


@pytest.mark.parametrize("mutate,step,rc", [
    (lambda r: r.pop("errors"), "finalize", 0), (lambda r: r.__setitem__("extra", 1), "finalize", 0),
    (lambda r: r.__setitem__("rc", True), "finalize", 0), (lambda r: r.__setitem__("cleanup_complete", 1), "finalize", 0),
    (lambda r: None, "envcheck", 0),                                            # step ≠ 檔名
    (lambda r: None, "finalize", 1),                                            # rc ≠ rc.tsv
    (lambda r: r.__setitem__("rc", 256), "finalize", 256),
    (lambda r: r.__setitem__("max_single_rss_bytes", 1), "finalize", 0),
    (lambda r: r.__setitem__("group_interval_ms", 50), "finalize", 0),
    (lambda r: r.__setitem__("group_samples", 0), "finalize", 0),
    (lambda r: r.__setitem__("reaper", "pid1"), "finalize", 0),
    (lambda r: r.__setitem__("env_keys", ["PATH", "HOME"]), "finalize", 0),     # 未排序
    (lambda r: r.__setitem__("env_keys", ["HOME", "HOME"]), "finalize", 0),     # 重複
    (lambda r: r.__setitem__("cmd", []), "finalize", 0),
    (lambda r: r.__setitem__("leftover_pids", [True]), "finalize", 0),
    (lambda r: r.__setitem__("all_descendants_reaped", False), "finalize", 0),
    (lambda r: r.__setitem__("auto_reap_detected", True), "finalize", 0),
    (lambda r: r.__setitem__("cleanup_complete", False), "finalize", 0),
    (lambda r: r.__setitem__("leftover_pids", [123]), "finalize", 0),
    (lambda r: r.__setitem__("errors", ["中斷"]), "finalize", 0),
], ids=lambda x: None)
def test_ac24c_each_tampered_host_field_is_a_problem(mutate, step, rc):
    rec = _host_ok()
    mutate(rec)
    assert sz.host_record_problems(rec, step=step, rc=rc)


def test_ac24c_invalid_host_record_fails_the_report_closed(tmp_path):
    s = _acc_full_state(tmp_path)
    path = s / "host" / "finalize.json"
    rec = json.loads(path.read_text())
    rec["all_descendants_reaped"] = False
    path.write_bytes(sz.canonical_dumps(rec))
    with pytest.raises(sz.SizingError, match="all_descendants_reaped"):
        _report(s)


# ── ac26（check-step 本身）與 guard 的身分（ac28 的 helper 部分） ────────────────────────────

def _step_state(tmp_path, *, rc_actual=0, host=None, sidecar_status="ok"):
    s = _acc_state(tmp_path)
    (s / "rc.tsv").write_text(f"finalize\t0\t{rc_actual}\n")
    rec = host if host is not None else _host_rec("finalize", rc_actual, children=50 << 20, sampled=60 << 20, cmd=["/x"])
    sz.write_exclusive(s / "host" / "finalize.json", sz.canonical_dumps(rec))
    if sidecar_status != "ok":
        path = s / "containers" / "o0030.json"
        side = json.loads(path.read_text())
        side.update(status=sidecar_status, failures=["rss.json：x"])
        path.write_bytes(sz.canonical_dumps(side))
    return s


def test_ac26_check_step_passes_and_fails_closed(tmp_path):
    assert sz.check_step(_step_state(tmp_path / "ok"), "finalize") == []
    assert sz.check_step(_step_state(tmp_path / "side", sidecar_status="measure_failed"), "finalize")
    bad_host = _host_rec("finalize", 0, children=50 << 20, sampled=60 << 20, cmd=["/x"], all_descendants_reaped=False)
    assert any("all_descendants_reaped" in p for p in sz.check_step(_step_state(tmp_path / "host", host=bad_host), "finalize"))
    s = _step_state(tmp_path / "missing")
    (s / "containers" / "o0050.json").unlink()
    assert any("缺 sidecar" in p for p in sz.check_step(s, "finalize"))
    s = _step_state(tmp_path / "rc", rc_actual=1)
    host = _host_rec("finalize", 0, children=50 << 20, sampled=60 << 20, cmd=["/x"])
    (s / "host" / "finalize.json").write_bytes(sz.canonical_dumps(host))
    assert any("rc" in p for p in sz.check_step(s, "finalize"))
    assert sz.check_step(_step_state(tmp_path / "unknown"), "envcheck") == ["rc.tsv 沒有 envcheck"]


def test_ac28_guard_identity_requires_both_sources_and_ignores_the_status_file(tmp_path):
    me = os.getpid()
    st = sz.read_proc_stat(me)[1]
    s = tmp_path / "S"
    s.mkdir()
    (s / "guard-probe.json").write_text(json.dumps({"pid": me, "starttime": st}))
    assert sz.guard_identity(s, me) == (me, st)
    # 只供診斷的狀態檔⛔ 不是身分來源：偽造一份（宣稱另一個 PID、setsid 失敗）→ 判斷⛔ 不變
    (s / "guard-probe-status.json").write_text(json.dumps({"schema": "i074_stage2_guard_probe_status_v1", "pid": me + 7,
                                                           "stage": "setsid", "errno": "EPERM"}))
    assert sz.guard_identity(s, me) == (me, st)
    for doc, shell_pid in (({"pid": me, "starttime": st}, me + 1),                      # JSON 的 pid ≠ $!
                           ({"pid": me, "starttime": st + 1}, me),                      # starttime 不符
                           ({"pid": me, "starttime": st, "x": 1}, me),                  # 多欄
                           ({"pid": True, "starttime": st}, 1)):                         # bool 冒充整數
        (s / "guard-probe.json").write_text(json.dumps(doc))
        with pytest.raises(sz.SizingError):
            sz.guard_identity(s, shell_pid)
    (s / "guard-probe.json").unlink()
    with pytest.raises(sz.SizingError):
        sz.guard_identity(s, me)


@pytest.mark.parametrize("override,rc", [({}, 0), ({"rss_source": "v2:anon"}, 1), ({"reaper": "subreaper"}, 1)])
def test_ac25_rss_capability_cli(tmp_path, override, rc):
    path = tmp_path / "rss.json"
    path.write_text(json.dumps(_rss_ok(**override)))
    assert sz.main(["rss-capability", "--file", str(path)]) == rc


# ── ac27：靜態（語意）——本 repo 的程式⛔ 不把 SIGCHLD 設成 SIG_IGN、⛔ 沒有 SA_NOCLDWAIT ─────────────────

_auto_reap_violations_py = sz.auto_reap_violations
_SHELL_TRAP_CHLD = sz.SHELL_TRAP_CHLD


@pytest.mark.parametrize("source", [
    "import signal\nsignal.signal(signal.SIGCHLD, signal.SIG_IGN)\n",
    "import signal as sig\nsig.signal(sig.SIGCHLD, sig.SIG_IGN)\n",
    "from signal import signal, SIGCHLD, SIG_IGN\nsignal(SIGCHLD, SIG_IGN)\n",
    "from signal import signal as s, SIGCHLD as C, SIG_IGN as I\ns(C, I)\n",
    "import signal\nsignal.signal(17, signal.SIG_IGN)\n",
    "import signal\nsignal.signal(signal.Signals.SIGCHLD, handler=signal.SIG_IGN)\n",
    "import signal\nsignal.signal(signal.SIGCLD, signal.Handlers.SIG_IGN)\n",
    "FLAGS = SA_NOCLDWAIT | 1\n",
    "import signal\nf = signal.SA_NOCLDWAIT\n",
])
def test_ac27_scanner_catches_every_form(source):
    assert _auto_reap_violations_py(source)


@pytest.mark.parametrize("source", [
    "import signal\nMASK = 1 << (int(signal.SIGCHLD) - 1)\n",                      # 偵測器的用法：只提到 SIGCHLD
    "import signal\nsignal.signal(signal.SIGCHLD, signal.SIG_DFL)\n",
    "import signal\nsignal.signal(signal.SIGTERM, signal.SIG_IGN)\n",
    "NOTE = '契約外：SA_NOCLDWAIT 看不到'\n",                                     # 說明文字提到它：⛔ 不算
])
def test_ac27_scanner_allows_detector_uses(source):
    assert _auto_reap_violations_py(source) == []


@pytest.mark.parametrize("line,hit", [("trap '' CHLD", True), ('trap -- "" SIGCHLD', True), ("trap '' TERM", False),
                                      ("trap 'x' CHLD", False), ("# trap '' CHLD", True)])
def test_ac27_shell_pattern(line, hit):
    assert bool(_SHELL_TRAP_CHLD.search(line)) is hit


def test_ac27_repo_never_sets_auto_reaping():
    """⚠️ 測試容器只掛了 python/——scripts/ 由 host unittest 的同一個掃描（sz.scan_auto_reaping）涵蓋。"""
    assert sz.scan_auto_reaping([PYTHON_ROOT], PYTHON_ROOT.parent) == []
    assert (PYTHON_ROOT / "scripts" / "i074_stage2_sizing.py").is_file()               # ⛔ 不是空掃描
