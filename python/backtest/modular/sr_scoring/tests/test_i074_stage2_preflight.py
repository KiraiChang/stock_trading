"""I-074 Stage 2 ⑦b：`python/scripts/i074_stage2_preflight.py`（磁碟檢查、state 的封閉 schema、anchors）。

對應 issue.md I-074「Stage 2 步驟 ⑦b 細部計畫 v1」「五」的 n1～n4、n6、state、ai／ay 的「在 replay 之前」那一層。
orchestrator 以 shell 串起這些子指令的整合測試在 `scripts/test-i074-stage2.sh`。
"""
from __future__ import annotations

import gzip
import hashlib
import importlib.util
import json
import os
import sys
from pathlib import Path
from types import SimpleNamespace

import pytest

from ..replay_bundle import stage2_archive as sa
from ..replay_bundle.canonical import canonical_json_bytes
from . import test_replay_envcheck as tec
from . import test_replay_stage2_archive as t2

_SCRIPTS = Path(__file__).resolve().parents[4] / "scripts"
sys.path.insert(0, str(_SCRIPTS))


def _load(name: str):
    spec = importlib.util.spec_from_file_location(name, _SCRIPTS / f"{name}.py")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


pf = _load("i074_stage2_preflight")
fr = _load("i074_stage2_freeze_record")
H = "a" * 64


# ── n6：常數 ─────────────────────────────────────────────────────────────────

def test_n6_constants_and_arithmetic():
    assert pf.P_B_BUDGET == 167_772_160
    assert pf.M_SAFETY == 1_073_741_824
    assert pf.REQUIRED_BYTES == 1_241_513_984 == pf.P_B_BUDGET + pf.M_SAFETY
    assert fr.P_B_BUDGET == pf.P_B_BUDGET
    # ⚠️ 唯一定義：freeze record 模組 import 它，⛔ 自己不再寫一份。
    assert "P_B_BUDGET =" not in (_SCRIPTS / "i074_stage2_freeze_record.py").read_text(encoding="utf-8")


def test_operational_names_mirror_stage2_archive():
    ops = (sa.OPERATIONAL_BEFORE_SOURCE, sa.OPERATIONAL_COMPARISON, sa.OPERATIONAL_REPORT)
    assert sorted(pf.OPERATIONAL_SUCCESS) == sorted(Path(p).name for p in ops)
    assert pf.OPERATIONAL_FAILURE == (Path(sa.OPERATIONAL_FAILURE).name,)
    assert {Path(p).parent.name for p in (*ops, sa.OPERATIONAL_FAILURE)} == {"stage2"}


# ── n1～n4：磁碟檢查 ────────────────────────────────────────────────────────────

def _stat(dev):
    return lambda path: SimpleNamespace(st_dev=dev)


def _vfs(bavail, frsize=4096, bfree=None):
    return lambda path: SimpleNamespace(f_bavail=bavail, f_frsize=frsize, f_bfree=bfree if bfree is not None else bavail)


LOCS = {"run": "/w/run", "tmp": "/w/tmp", "baselines": "/w/repo/python/baselines/i074_stage2", "git": "/w/repo/.git"}


def test_n1_boundary():
    exact = pf.REQUIRED_BYTES
    assert pf.disk_check(LOCS, "/var/lib/docker", stat_fn=_stat(7), statvfs_fn=_vfs(exact, 1))["available"] == exact
    with pytest.raises(pf.PreflightError, match="可用空間"):
        pf.disk_check(LOCS, "/var/lib/docker", stat_fn=_stat(7), statvfs_fn=_vfs(exact - 1, 1))


def test_n2_f_bfree_does_not_count():
    with pytest.raises(pf.PreflightError, match="可用空間"):
        pf.disk_check(LOCS, "/d", stat_fn=_stat(7), statvfs_fn=_vfs(10, 4096, bfree=10**9))


@pytest.mark.parametrize("odd", ["run", "tmp", "baselines", "git", "docker_root"])
def test_n3_every_location_must_share_the_device(odd):
    def stat_fn(path):
        name = next((k for k, v in LOCS.items() if v == path), "docker_root")
        return SimpleNamespace(st_dev=8 if name == odd else 7)

    with pytest.raises(pf.PreflightError, match="同一個裝置"):
        pf.disk_check(LOCS, "/d", stat_fn=stat_fn, statvfs_fn=_vfs(10**9))


def test_n4_failures_are_rejections():
    def boom(path):
        raise OSError("EIO")

    with pytest.raises(pf.PreflightError, match="fail-closed"):
        pf.disk_check(LOCS, "/d", stat_fn=boom, statvfs_fn=_vfs(10**9))
    with pytest.raises(pf.PreflightError, match="fail-closed"):
        pf.disk_check(LOCS, "/d", stat_fn=_stat(7), statvfs_fn=boom)
    with pytest.raises(pf.PreflightError, match="Docker Root Dir"):
        pf.disk_check(LOCS, "", stat_fn=_stat(7), statvfs_fn=_vfs(10**9))
    with pytest.raises(pf.PreflightError, match="run"):
        pf.disk_check({"tmp": "/w/tmp"}, "/d", stat_fn=_stat(7), statvfs_fn=_vfs(10**9))


# ── state：封閉 schema、canonical、SHA 鏈、交叉條件 ────────────────────────────────

def _record_and_report(work: Path, *, identity: bytes, image: str, cf: bytes, tooling: bytes) -> dict:
    cf_sha, tool_sha = hashlib.sha256(cf).hexdigest(), hashlib.sha256(tooling).hexdigest()
    meta = {"run_id": "20260930T000000Z-1", "mode": "formal", "repo_head": "1" * 40, "clone_head": "1" * 40,
            "base_commit": fr.BASE_COMMIT, "harness_sha256": "2" * 64, "shim_sha256": "3" * 64, "helper_sha256": "4" * 64,
            "image": image, "identity_sha256": hashlib.sha256(identity).hexdigest(),
            "counterfactual_sha256": cf_sha, "counterfactual_canonical_sha256": cf_sha,
            "tooling_sha256": tool_sha, "tooling_canonical_sha256": tool_sha}
    report = {"schema": "i074_stage2_sizing_report_v1", "mode": "formal", "status": "ok", "P_B": 1000, "meta": meta}
    report_raw = canonical_json_bytes(report)
    record = {"schema": fr.SCHEMA, "mode": "formal", "status": "ok", "p_b_bytes": 1000,
              "report_sha256": hashlib.sha256(report_raw).hexdigest()}
    for m, r in fr.META_PAIRS:
        record[r] = meta[m]
    (work / "freeze").mkdir(parents=True)
    (work / "freeze" / "sizing_report.json").write_bytes(report_raw)
    (work / "freeze" / "freeze_record.json").write_bytes(canonical_json_bytes(record))
    return record


@pytest.fixture
def work(tmp_path):
    w = tmp_path / "work"
    identity = canonical_json_bytes({"x": 1})
    image = "sha256:" + "9" * 64
    cf, tooling = b"cf-patch\n", b"tooling-patch\n"
    record = _record_and_report(w, identity=identity, image=image, cf=cf, tooling=tooling)
    for d in ("state", "run/patches", "logs"):
        (w / d).mkdir(parents=True)
    (w / "run" / "patches" / "counterfactual.patch").write_bytes(cf)
    (w / "run" / "patches" / "tooling.patch").write_bytes(tooling)
    (tmp_path / "identity.json").write_bytes(identity)
    anchors = {"bundle_id": "b1_x", "after_base_commit": fr.BASE_COMMIT,
               "after_artifact": "python/baselines/i074_stage1/d1/after_artifact.json.gz",
               "cohort_manifest": "python/baselines/i074_stage1/d1/cohort_manifest.json.gz"}
    (w / "logs" / "anchors.out").write_text(json.dumps(anchors))
    ns = SimpleNamespace(w=w, real=str(tmp_path / "real"), identity=str(tmp_path / "identity.json"), image=image,
                         record=record, cf=cf, tooling=tooling)
    ns.common = ["--state-dir", str(w / "state"), "--work-dir", str(w), "--real-repo", ns.real]
    return ns


def _write_all(ns, rc=0):
    assert pf.main(["state-write-run", *ns.common]) == 0
    assert pf.main(["state-write-preflight", *ns.common, "--counterfactual-sha256", hashlib.sha256(ns.cf).hexdigest(),
                    "--tooling-sha256", hashlib.sha256(ns.tooling).hexdigest(), "--composed-sha256", H,
                    "--semantic-sha256", "b" * 64, "--identity", ns.identity, "--image", ns.image,
                    "--anchors", str(ns.w / "logs" / "anchors.out")]) == 0
    assert pf.main(["state-write-replay-started", *ns.common]) == 0
    out = ns.w / "run" / "stage2"
    out.mkdir()
    for name in (pf.OPERATIONAL_SUCCESS if rc == 0 else pf.OPERATIONAL_FAILURE):
        (out / name).write_text(name)
    assert pf.main(["state-write-replay-done", *ns.common, "--rc", str(rc)]) == 0


def _check(ns, **kwargs):
    return pf.check_states(ns.w / "state", work_dir=str(ns.w), real_repo=ns.real, **kwargs)


def test_state_round_trip_and_chain(work, capsys):
    _write_all(work)
    assert pf.main(["state-write-attempt", *work.common, "--kind", "finalize"]) == 0
    assert pf.main(["state-write-attempt", *work.common, "--kind", "finalize"]) == 0
    states = _check(work, require=pf.STATE_ORDER, image=work.image, identity=work.identity, verify_files=True)
    assert states["attempt"]["count"] == 2
    assert states["replay_started"]["argv"] == pf.replay_argv(str(work.w), states["preflight"])
    for kind in pf.STATE_ORDER:
        raw = (work.w / "state" / pf.STATE_FILES[kind]).read_bytes()
        assert raw == canonical_json_bytes(json.loads(raw))
    capsys.readouterr()
    assert pf.main(["state-check", *work.common, "--require", "run,preflight,replay_started,replay_done,attempt",
                    "--image", work.image, "--identity", work.identity, "--verify-files"]) == 0
    out = dict(line.split("=", 1) for line in capsys.readouterr().out.split())
    assert out["rc"] == "0" and out["bundle_id"] == "b1_x" and out["semantic"] == "b" * 64


def test_replay_argv_is_the_single_definition(work):
    _write_all(work)
    pre = _check(work, require=("preflight",))["preflight"]
    w = str(work.w)
    assert pf.replay_argv(w, pre) == [
        f"{w}/repo/scripts/run-replay-offline.sh", "--bundle", f"{w}/repo/python/baselines/b1_x",
        "--output-dir", f"{w}/run/stage2", "--before-ref", fr.BASE_COMMIT,
        "--after-artifact", f"{w}/repo/python/baselines/i074_stage1/d1/after_artifact.json.gz",
        "--cohort-manifest", f"{w}/repo/python/baselines/i074_stage1/d1/cohort_manifest.json.gz", "--i074-counterfactual"]


def _rewrite(ns, kind, mutate, *, canonical=True):
    path = ns.w / "state" / pf.STATE_FILES[kind]
    payload = json.loads(path.read_bytes())
    mutate(payload)
    path.write_bytes(canonical_json_bytes(payload) if canonical else json.dumps(payload, indent=1).encode())


@pytest.mark.parametrize("kind", pf.STATE_ORDER[:4])
@pytest.mark.parametrize("mutation", ["extra", "missing", "bool", "schema", "noncanonical"])
def test_each_state_is_closed(work, kind, mutation):
    _write_all(work)
    int_key = {"replay_done": "rc"}.get(kind)
    key = sorted(k for k in pf.STATE_FIELDS[kind])[0]
    mutate = {
        "extra": lambda p: p.update(extra=1),
        "missing": lambda p: p.pop(key),
        "bool": lambda p: p.update({int_key or key: True}),
        "schema": lambda p: p.update(schema="x/v1"),
        "noncanonical": lambda p: None,
    }[mutation]
    _rewrite(work, kind, mutate, canonical=mutation != "noncanonical")
    with pytest.raises(pf.PreflightError):
        _check(work, require=("run",))


def test_chain_mismatch_and_missing_predecessor(work):
    _write_all(work)
    _rewrite(work, "replay_started", lambda p: p.update(preflight_sha256="c" * 64))
    with pytest.raises(pf.PreflightError, match="鏈不符"):
        _check(work, require=("run",))
    (work.w / "state" / "replay_started.json").unlink()
    with pytest.raises(pf.PreflightError, match="前一份"):
        _check(work, require=("run",))


def test_current_image_and_identity_must_match_preflight(work):
    _write_all(work)
    with pytest.raises(pf.PreflightError, match="REPLAY_IMAGE_ID"):
        _check(work, require=("preflight",), image="sha256:" + "8" * 64)
    Path(work.identity).write_bytes(canonical_json_bytes({"x": 2}))     # 只改一個欄位（例如 created_at）
    with pytest.raises(pf.PreflightError, match="identity"):
        _check(work, require=("preflight",), identity=work.identity)


def test_verify_files_catches_changed_patch_and_outputs(work):
    _write_all(work)
    (work.w / "run" / "stage2" / "report.json").write_text("x")
    with pytest.raises(pf.PreflightError, match="report.json"):
        _check(work, require=("replay_done",), verify_files=True)
    (work.w / "run" / "stage2" / "report.json").write_text("report.json")
    (work.w / "run" / "patches" / "tooling.patch").write_bytes(b"x")
    with pytest.raises(pf.PreflightError, match="tooling.patch"):
        _check(work, require=("replay_done",), verify_files=True)


def test_run_cross_conditions(work):
    _write_all(work)
    with pytest.raises(pf.PreflightError, match="real_repo"):
        pf.check_states(work.w / "state", require=("run",), work_dir=str(work.w), real_repo="/elsewhere")
    (work.w / "freeze" / "freeze_record.json").write_bytes(
        (work.w / "freeze" / "freeze_record.json").read_bytes())      # 相同 bytes → 仍通過
    _check(work, require=("run",))
    (work.w / "freeze" / "sizing_report.json").write_bytes(b"{}")
    with pytest.raises(pf.PreflightError, match="報告"):
        _check(work, require=("run",))


def test_preflight_must_match_the_freeze_record(work):
    assert pf.main(["state-write-run", *work.common]) == 0
    argv = ["state-write-preflight", *work.common, "--counterfactual-sha256", "d" * 64,
            "--tooling-sha256", hashlib.sha256(work.tooling).hexdigest(), "--composed-sha256", H, "--semantic-sha256", H,
            "--identity", work.identity, "--image", work.image, "--anchors", str(work.w / "logs" / "anchors.out")]
    assert pf.main(argv) == 1                     # canonical ≠ raw
    assert not (work.w / "state" / "preflight.json").exists()


def test_exclusive_and_ordering(work):
    assert pf.main(["state-write-run", *work.common]) == 0
    assert pf.main(["state-write-run", *work.common]) == 1          # exclusive：⛔ 覆寫
    assert pf.main(["state-write-replay-started", *work.common]) == 1   # 沒有 preflight
    assert pf.main(["state-write-attempt", *work.common, "--kind", "finalize"]) == 1


def test_replay_done_shape(work):
    _write_all(work, rc=6)
    with pytest.raises(pf.PreflightError, match="形狀"):
        pf._output_shape(work.w / "run" / "stage2", 0)
    (work.w / "run" / "stage2" / "comparison_artifact.json").write_text("c")
    with pytest.raises(pf.PreflightError, match="形狀"):
        pf._output_shape(work.w / "run" / "stage2", 6)
    with pytest.raises(pf.PreflightError, match="∉"):
        pf._output_shape(work.w / "run" / "stage2", 4)


def test_attempt_kind_must_match_the_replay_rc(work):
    _write_all(work, rc=6)
    assert pf.main(["state-write-attempt", *work.common, "--kind", "finalize"]) == 0
    with pytest.raises(pf.PreflightError, match="不對應"):
        _check(work, require=("attempt",))


def test_preflight_single_file_rules():
    good = {"schema": pf.STATE_SCHEMAS["preflight"], "run_sha256": H, "counterfactual_raw_sha256": H,
            "tooling_raw_sha256": "b" * 64, "counterfactual_sha256": H, "tooling_sha256": "b" * 64,
            "composed_sha256": H, "counterfactual_semantic_sha256": H, "identity_sha256": H, "base_commit": "1" * 40,
            "image_id": "sha256:" + H, "bundle_id": "b1", "after_artifact": "python/a.gz", "cohort_manifest": "python/c.gz"}
    pf.validate_state("preflight", good)
    for label, change in (("raw ≠ canonical", {"tooling_sha256": H}),
                          ("tooling 為空", {"tooling_raw_sha256": pf.EMPTY_SHA256, "tooling_sha256": pf.EMPTY_SHA256}),
                          ("路徑含 ..", {"after_artifact": "python/../x"}), ("路徑不在 python/", {"after_artifact": "/etc/x"}),
                          ("bundle 含 /", {"bundle_id": "a/b"}), ("image 大寫", {"image_id": "sha256:" + "A" * 64})):
        with pytest.raises(pf.PreflightError):
            pf.validate_state("preflight", dict(good, **change)), label


# ── anchors（容器內；ai／ay） ────────────────────────────────────────────────────

@pytest.fixture
def anchor_env(tmp_path, monkeypatch):
    env = t2._build_env(tmp_path, monkeypatch)
    identity = tmp_path / "run_identity.json"
    identity.write_bytes(canonical_json_bytes(tec._identity()))
    return env, identity


def test_anchors_uses_the_current_identity(anchor_env, monkeypatch):
    env, identity = anchor_env
    seen = []
    real = sa._load_trust_anchors
    monkeypatch.setattr(sa, "_load_trust_anchors", lambda root, ident: seen.append(ident) or real(root, ident))
    out = pf.run_anchors(str(env.root), str(identity), tec.NEW_IMG)
    assert seen and seen[0] == tec._identity()               # ⛔ 不是 None（那會改用 envcheck 封存的 identity）
    assert out == {"bundle_id": t2.BUNDLE, "after_base_commit": t2.BASE,
                   "after_artifact": "python/baselines/i074_stage1/d1/after_artifact.json.gz",
                   "cohort_manifest": "python/baselines/i074_stage1/d1/cohort_manifest.json.gz"}


def test_ai_identity_differing_only_in_created_at_is_rejected(anchor_env):
    env, identity = anchor_env
    identity.write_bytes(canonical_json_bytes(tec._identity(created_at="2026-09-25T09:00:00+08:00")))
    with pytest.raises(Exception):
        pf.run_anchors(str(env.root), str(identity), tec.NEW_IMG)


def test_image_must_match_the_identity(anchor_env):
    env, identity = anchor_env
    with pytest.raises(pf.PreflightError, match="expected_image_id"):
        pf.run_anchors(str(env.root), str(identity), "sha256:" + "7" * 64)
    with pytest.raises(pf.PreflightError, match="格式"):
        pf.run_anchors(str(env.root), str(identity), "--help")


def test_ay_tampered_cohort_is_rejected(anchor_env):
    env, identity = anchor_env
    cohort = env.root / "baselines" / "i074_stage1" / "d1" / "cohort_manifest.json.gz"
    os.chmod(cohort, 0o644)
    raw = gzip.decompress(cohort.read_bytes())
    tampered = raw.replace(b"2026-08-21", b"2026-08-22")
    assert tampered != raw
    cohort.write_bytes(gzip.compress(tampered, mtime=0))
    with pytest.raises(Exception):
        pf.run_anchors(str(env.root), str(identity), tec.NEW_IMG)
