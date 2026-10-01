"""I-074 Stage 2 ⑦b：`python/scripts/i074_stage2_freeze_record.py`（v29「八之二」的封閉 schema、交叉條件、寫入端）。

對應 issue.md I-074「Stage 2 步驟 ⑦b 細部計畫 v1」「五」的 n7（freeze record 違反「八之二」的每一條）與 n10（寫入端）。
需要 git 的 `head_file_sha256()` 在 host 的 unittest（`scripts/tests/test_i074_stage2_host.py`）。
"""
from __future__ import annotations

import hashlib
import importlib.util
import json
import sys
from pathlib import Path

import pytest

from ..replay_bundle.canonical import canonical_json_bytes

_SCRIPTS = Path(__file__).resolve().parents[4] / "scripts"
sys.path.insert(0, str(_SCRIPTS))
_spec = importlib.util.spec_from_file_location("i074_stage2_freeze_record", _SCRIPTS / "i074_stage2_freeze_record.py")
fr = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(fr)

IMG = "sha256:" + "9" * 64
CF, TOOL = hashlib.sha256(b"cf").hexdigest(), hashlib.sha256(b"tool").hexdigest()


def _meta(**overrides):
    meta = {"run_id": "20260930T010203Z-42", "mode": "formal", "repo_head": "1" * 40, "clone_head": "1" * 40,
            "base_commit": fr.BASE_COMMIT, "harness_sha256": "2" * 64, "shim_sha256": "3" * 64,
            "helper_sha256": "4" * 64, "image": IMG, "identity_sha256": "5" * 64,
            "counterfactual_sha256": CF, "counterfactual_canonical_sha256": CF,
            "tooling_sha256": TOOL, "tooling_canonical_sha256": TOOL}
    meta.update(overrides)
    return meta


def _report(mode="formal", status="ok", p_b=1000, **meta):
    return {"schema": "i074_stage2_sizing_report_v1", "mode": mode, "status": status, "P_B": p_b,
            "meta": _meta(mode=mode, **meta)}


def _record(report: dict) -> dict:
    record = {"schema": fr.SCHEMA, "mode": "formal", "status": "ok", "p_b_bytes": report["P_B"],
              "report_sha256": hashlib.sha256(canonical_json_bytes(report)).hexdigest()}
    for m, r in fr.META_PAIRS:
        record[r] = report["meta"][m]
    return record


def test_good_record_passes():
    fr.validate_record(_record(_report()))


@pytest.mark.parametrize("change", [
    {"schema": "x/v1"}, {"mode": "validation"}, {"status": "assumption_violated"}, {"run_id": "2026-09-30"},
    {"repo_head": "1" * 39}, {"repo_head": "A" * 40}, {"clone_head": "2" * 40}, {"base_commit": "3" * 40},
    {"p_b_bytes": True}, {"p_b_bytes": 0}, {"p_b_bytes": 167_772_161}, {"p_b_bytes": 1.0},
    {"report_sha256": "A" * 64}, {"harness_sha256": "x"}, {"counterfactual_patch_sha256": "6" * 64},
    {"tooling_patch_sha256": "6" * 64},
    {"tooling_patch_raw_sha256": hashlib.sha256(b"").hexdigest(), "tooling_patch_sha256": hashlib.sha256(b"").hexdigest()},
    {"image_id": "sha256:" + "A" * 64}, {"image_id": "stock:latest"}, {"identity_sha256": 5},
])
def test_each_rule_of_the_closed_schema(change):
    with pytest.raises(fr.FreezeRecordError):
        fr.validate_record(dict(_record(_report()), **change))


def test_extra_and_missing_keys():
    good = _record(_report())
    with pytest.raises(fr.FreezeRecordError, match="鍵集合"):
        fr.validate_record(dict(good, extra=1))
    missing = dict(good)
    del missing["tooling_patch_raw_sha256"]
    with pytest.raises(fr.FreezeRecordError, match="鍵集合"):
        fr.validate_record(missing)


def test_load_requires_canonical(tmp_path):
    record = _record(_report())
    path = tmp_path / "freeze_record.json"
    path.write_bytes(canonical_json_bytes(record))
    assert fr.load_record(path)[0] == record
    path.write_text(json.dumps(record, indent=2))
    with pytest.raises(fr.FreezeRecordError, match="canonical"):
        fr.load_record(path)
    link = tmp_path / "link.json"
    link.symlink_to(path)
    with pytest.raises(fr.FreezeRecordError, match="一般檔案"):
        fr.load_record(link)


def test_cross_check_report_each_pair():
    report = _report()
    record = _record(report)
    fr.cross_check_report(record, canonical_json_bytes(report))
    for meta_key, _field in fr.META_PAIRS:
        bad = _report(**{meta_key: ("7" * 64 if meta_key.endswith("sha256") else "x")})
        bad_record = dict(record, report_sha256=hashlib.sha256(canonical_json_bytes(bad)).hexdigest())
        with pytest.raises(fr.FreezeRecordError, match=f"meta.{meta_key}"):
            fr.cross_check_report(bad_record, canonical_json_bytes(bad))
    with pytest.raises(fr.FreezeRecordError, match="report_sha256"):
        fr.cross_check_report(record, canonical_json_bytes(report) + b" ")
    for change, name in (({"P_B": 999}, "P_B"), ({"status": "assumption_violated"}, "status")):
        bad = dict(report, **change)
        with pytest.raises(fr.FreezeRecordError, match=name):
            fr.cross_check_report(dict(record, report_sha256=hashlib.sha256(canonical_json_bytes(bad)).hexdigest()),
                                  canonical_json_bytes(bad))


@pytest.fixture
def build_env(tmp_path, monkeypatch):
    identity = tmp_path / "run_identity.json"
    identity.write_bytes(canonical_json_bytes({"expected_image_id": IMG, "x": 1}))
    ident_sha = hashlib.sha256(identity.read_bytes()).hexdigest()
    scripts = {"scripts/i074-stage2-sizing.sh": "2" * 64, "scripts/lib/i074-sizing-docker-shim.sh": "3" * 64,
               "python/scripts/i074_stage2_sizing.py": "4" * 64}
    monkeypatch.setattr(fr, "head_file_sha256", lambda repo, rev, rel: scripts[rel])

    def write(report):
        path = tmp_path / "sizing_report.json"
        path.write_bytes(canonical_json_bytes(report))
        return path
    return write, identity, ident_sha, scripts


def test_n10_build_writes_only_formal_ok_within_budget(build_env, tmp_path):
    write, identity, ident_sha, _ = build_env
    record, reason = fr.build_record(report_path=write(_report(identity_sha256=ident_sha)), repo="/r", identity=identity)
    assert reason == "" and record is not None
    fr.validate_record(record)
    for report, why in ((_report(mode="validation", identity_sha256=ident_sha), "mode"),
                        (_report(status="assumption_violated", identity_sha256=ident_sha), "status"),
                        (_report(p_b=167_772_161, identity_sha256=ident_sha), "P_B_BUDGET")):
        record, reason = fr.build_record(report_path=write(report), repo="/r", identity=identity)
        assert record is None and why in reason
    out = tmp_path / "out.json"
    assert fr.main(["build", "--report", str(write(_report(mode="validation", identity_sha256=ident_sha))),
                    "--repo", "/r", "--identity", str(identity), "--out", str(out)]) == 0
    assert not out.exists()                                 # ⛔ 不寫，結束碼仍 0


def test_n10_build_rejects_inconsistent_inputs(build_env, monkeypatch):
    write, identity, ident_sha, scripts = build_env
    with pytest.raises(fr.FreezeRecordError, match="內容 SHA"):
        fr.build_record(report_path=write(_report(identity_sha256=ident_sha, harness_sha256="8" * 64)), repo="/r",
                        identity=identity)
    with pytest.raises(fr.FreezeRecordError, match="identity"):
        fr.build_record(report_path=write(_report()), repo="/r", identity=identity)
    with pytest.raises(fr.FreezeRecordError, match="meta 缺少"):
        fr.build_record(report_path=write(_report(identity_sha256=ident_sha, tooling_sha256="")), repo="/r",
                        identity=identity)


def test_write_is_exclusive_and_rereads(build_env, tmp_path):
    write, identity, ident_sha, _ = build_env
    record, _ = fr.build_record(report_path=write(_report(identity_sha256=ident_sha)), repo="/r", identity=identity)
    out = tmp_path / "freeze_record.json"
    fr.write_record(out, record)
    assert fr.load_record(out)[0] == record
    with pytest.raises(FileExistsError):
        fr.write_record(out, record)


def test_check_full(tmp_path):
    clone = tmp_path / "clone"
    for rel, text in fr.SCRIPT_FIELDS.items():
        (clone / text).parent.mkdir(parents=True, exist_ok=True)
        (clone / text).write_text(rel)
    sha = {f: hashlib.sha256(f.encode()).hexdigest() for f in fr.SCRIPT_FIELDS}
    cf, tool = tmp_path / "cf.patch", tmp_path / "tool.patch"
    cf.write_bytes(b"cf")
    tool.write_bytes(b"tool")
    identity = tmp_path / "identity.json"
    identity.write_bytes(canonical_json_bytes({"expected_image_id": IMG}))
    report = _report(harness_sha256=sha["harness_sha256"], shim_sha256=sha["shim_sha256"],
                     helper_sha256=sha["helper_sha256"], identity_sha256=hashlib.sha256(identity.read_bytes()).hexdigest())
    (tmp_path / "sizing_report.json").write_bytes(canonical_json_bytes(report))
    (tmp_path / "freeze_record.json").write_bytes(canonical_json_bytes(_record(report)))
    kwargs = dict(record_path=tmp_path / "freeze_record.json", report_path=tmp_path / "sizing_report.json", clone=clone,
                  frozen_counterfactual=cf, frozen_tooling=tool, counterfactual_canonical=CF, tooling_canonical=TOOL,
                  identity=identity, image=IMG)
    fr.check_full(**kwargs)
    for label, change in (("canonical 的 cf", {"counterfactual_canonical": "0" * 64}),
                          ("canonical 的 tooling", {"tooling_canonical": "0" * 64}),
                          ("REPLAY_IMAGE_ID", {"image": "sha256:" + "8" * 64})):
        with pytest.raises(fr.FreezeRecordError):
            fr.check_full(**dict(kwargs, **change)), label
    tool.write_bytes(b"changed")
    with pytest.raises(fr.FreezeRecordError, match="tooling"):
        fr.check_full(**kwargs)
    tool.write_bytes(b"tool")
    (clone / fr.SCRIPT_FIELDS["shim_sha256"]).write_text("changed")
    with pytest.raises(fr.FreezeRecordError, match="shim_sha256"):
        fr.check_full(**kwargs)
    (clone / fr.SCRIPT_FIELDS["shim_sha256"]).write_text("shim_sha256")
    identity.write_bytes(canonical_json_bytes({"expected_image_id": IMG, "created_at": "x"}))
    with pytest.raises(fr.FreezeRecordError, match="identity"):
        fr.check_full(**kwargs)


def test_base_commit_is_e1cbbbd():
    assert fr.BASE_COMMIT == "e1cbbbdab44f8cf2d152e6ade9235d844f590d7f"
