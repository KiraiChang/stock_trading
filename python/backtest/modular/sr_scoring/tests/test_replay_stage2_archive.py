"""I-074 Stage 2 的正式證據（③d）：成功 archive、failed-attempt record、check／recover 的 Python 段。

對應 issue.md I-074 ③ evidence contract（⚠️ 現行版）的「三」～「七之四」與測試矩陣 `a`～`aw`、
`ay`、`az`、`ah4`／`ah5`、`bb`～`bk`（⚠️ `ax`、`ba` 與 `n`／`ai`／`ay` 的「在 replay 之前」那一層屬
Stage 2 步驟 ⑦；shell 端的合成守門、argv 與 check 的決策在 `scripts/test-replay-args.sh`）。

⚠️ fixture 用**真的 Stage 1 finalizer** 與**真的 envcheck 發布**產出信任錨（⛔ 不手捏 manifest），
且 D+1 帶兩列候選——cohort 為空的話，comparison 的逐列檢查全都會空洞地通過。
"""
from __future__ import annotations

import gzip
import json
from dataclasses import dataclass
from pathlib import Path

import pytest

from ..replay_bundle import crossday as cd
from ..replay_bundle import envcheck as ec
from ..replay_bundle import evidence as ev
from ..replay_bundle import stage2_archive as sa
from ..replay_bundle import stage2_evidence as s2
from ..replay_bundle.artifacts import (
    AFTER_KIND,
    ArtifactError,
    build_comparison_artifact,
    build_report,
    compare_rows,
    row_key,
    validate_comparison_artifact,
    validate_report,
)
from ..replay_bundle.canonical import canonical_gzip_bytes, canonical_json_bytes, sha256_hex
from ..replay_bundle.provenance import ProvenanceError
from ..replay_bundle.publish import EXIT_ABORT, EXIT_DURABILITY_UNCONFIRMED, DurabilityUnconfirmed
from . import test_i074_preflight as tip
from . import test_replay_envcheck as tec
from . import test_replay_evidence as tre

BUNDLE = tec.BUNDLE
NEW_IMG = tec.NEW_IMG
OLD_IMG = tec.OLD_IMG
BASE = "e" * 40                         # fixture 的 Stage 1 after base_commit（`_prov()` 的值）
CF = b"diff --git a/lifecycle.py b/lifecycle.py\n--- a/lifecycle.py\n+++ b/lifecycle.py\n@@ -1 +1 @@\n-a\n+b\n"
TOOLING = b"diff --git a/probe.py b/probe.py\nnew file mode 100644\n--- /dev/null\n+++ b/probe.py\n@@ -0,0 +1 @@\n+x\n"
GEN = "2026-09-25T10:00:00+08:00"


def _d1_rows():
    return [
        tre._row("2026-08-20"),
        tre._row("2026-08-21", lifecycle_phase="CONTINUATION", rr_decoupling_candidate=True),
        tre._row("2026-08-22", lifecycle_phase="CONTINUATION", setup_rr_qualified=True),
        tre._row("2026-08-25", lifecycle_phase="CONTINUATION", rr_decoupling_candidate=True),
    ]


COHORT = [("2330", "1d", "2026-08-21"), ("2330", "1d", "2026-08-25")]


def _before_rows():
    """反事實生效：兩列候選的 RR 被加回去 → 不再是 CONTINUATION，flag 為 false。"""
    rows = _d1_rows()
    for row in rows:
        if row_key(row) in COHORT:
            row.update(lifecycle_phase="TESTING", rr_decoupling_candidate=False)
    return rows


def _cohort_for(after, generated_at):
    cohort = tre._cohort(after, generated_at)
    cohort["keys"] = [list(k) for k in sorted(row_key(r) for r in after["rows"] if r["rr_decoupling_candidate"])]
    return cohort


@dataclass
class Env:
    root: Path          # python root
    tmp: Path
    after_sha: str      # 已錨定的 D+1 artifact_sha256


def _build_env(tmp_path, monkeypatch, *, witness_rows=None) -> Env:
    monkeypatch.setattr(tre, "_prov", tec._stage1_prov)
    monkeypatch.setattr(tip, "_prov", tec._stage1_prov)
    rows = _d1_rows()
    sources = tre._write_sources(tmp_path, d1_rows=rows)
    d = tre._after("2026-09-17T09:00:00+08:00", rows, "/tmp/d")
    d1 = tre._after("2026-09-18T09:00:00+08:00", rows, "/tmp/d1")
    crossday = cd.build_crossday(
        d=d, d1=d1,
        d_sha=sha256_hex(canonical_json_bytes(d)), d1_sha=sha256_hex(canonical_json_bytes(d1)),
        comparator_provenance=tec._stage1_prov() | {"argv": ["--compare", "--output-dir", "/tmp/cd"]},
        generated_at="2026-09-18T10:00:00+08:00",
    )
    replaced = {
        "d/after_artifact.json.gz": d,
        "d/cohort_manifest.json.gz": _cohort_for(d, "2026-09-17T09:00:00+08:00"),
        "d1/after_artifact.json.gz": d1,
        "d1/cohort_manifest.json.gz": _cohort_for(d1, "2026-09-18T09:00:00+08:00"),
        "crossday/crossday_artifact.json.gz": crossday,
    }
    for rel, payload in replaced.items():
        sources[rel].write_bytes(canonical_json_bytes(payload))
    root = tmp_path / "python"
    ev.finalize_evidence(
        evidence_root=root / "baselines" / "i074_stage1", sources=sources, expected_image_id=OLD_IMG,
        provenance_factory=lambda: tec._stage1_prov() | {"argv": ["--output-dir", "/tmp/fin"]},
        generated_at="2026-09-18T12:00:00+08:00",
    )
    witness = tec._write_witness(tmp_path / "w", rows=witness_rows or _d1_rows(),
                                 cohort_keys=[list(k) for k in COHORT])
    tec._publish(root, witness)
    return Env(root=root, tmp=tmp_path, after_sha=sha256_hex(canonical_json_bytes(d1)))


@pytest.fixture
def env(tmp_path, monkeypatch):
    return _build_env(tmp_path, monkeypatch)


def _before_prov(**overrides):
    return tec._witness_prov(argv=["--i074-counterfactual", "--output-dir", "/tmp/s2run"],
                             tooling_patch_sha256=sha256_hex(CF)) | overrides


def _write_run(env, *, before_rows=None, prov=None, comparison_prov=None, cf=CF, tooling=b"",
               before_overrides=None, mutate_comparison=None, mutate_report=None, name="s2run"):
    """⑦ 的 orchestrator 產出的 run 目錄：`stage2/` 三份 operational ＋ `patches/` 兩份凍結副本。"""
    run = env.tmp / name
    (run / "stage2").mkdir(parents=True)
    (run / "patches").mkdir()
    prov = prov or _before_prov()
    before_rows = before_rows if before_rows is not None else _before_rows()
    before = sa.build_before_source(bundle_id=BUNDLE, before_ref=prov["base_commit"], timeframe="1d",
                                    replay_scope="all_candidates", generated_at=GEN, provenance=prov,
                                    rows=before_rows)
    before.update(before_overrides or {})
    before_by_key = {row_key(r): r for r in before_rows}
    after_by_key = {row_key(r): r for r in _d1_rows()}
    comparison = build_comparison_artifact(
        bundle_id=BUNDLE, before_ref=prov["base_commit"], after_artifact_sha256=env.after_sha,
        generated_at=GEN, provenance=comparison_prov or prov,
        rows=[compare_rows(before_by_key[k], after_by_key[k]) for k in COHORT if k in before_by_key],
    )
    if mutate_comparison:
        mutate_comparison(comparison)
    report = build_report(bundle_id=comparison["bundle_id"], before_ref=comparison["before_ref"],
                          comparison_sha256=sha256_hex(canonical_json_bytes(comparison)), generated_at=GEN,
                          rows=comparison["rows"], report_max_rows=sa.STAGE2_REPORT_MAX_ROWS)
    if mutate_report:
        mutate_report(report)
    (run / sa.OPERATIONAL_BEFORE_SOURCE).write_bytes(canonical_json_bytes(before))
    (run / sa.OPERATIONAL_COMPARISON).write_bytes(canonical_json_bytes(comparison))
    (run / sa.OPERATIONAL_REPORT).write_bytes(canonical_json_bytes(report))
    (run / sa.FROZEN_COUNTERFACTUAL_PATCH).write_bytes(cf)
    (run / sa.FROZEN_TOOLING_PATCH).write_bytes(tooling)
    return run


def _fin_prov(**overrides):
    return tec._witness_prov(argv=["--finalize", "--run-dir", "/tmp/s2run"]) | overrides


def _claimed(mode, env, **kwargs):
    """比照 shell：先取合成守門的宣告值（shell 驗過之後以 `--verified-*` 交給 Python）。"""
    return sa.VerifiedComposition(**sa.patch_claims(mode=mode, python_root=env.root, **kwargs))


def _verified(prov, cf=CF, tooling=b""):
    return sa.VerifiedComposition(base_commit=prov["base_commit"], counterfactual_patch_sha256=sha256_hex(cf),
                                  tooling_patch_sha256=sha256_hex(tooling),
                                  composed_sha256=prov["tooling_patch_sha256"])


def _finalize(env, run, **overrides):
    kwargs = {"python_root": env.root, "run_dir": run, "stage2_identity": tec._identity(),
              "provenance_factory": _fin_prov, "generated_at": GEN}
    kwargs.update(overrides)
    if "verified" not in kwargs:
        kwargs["verified"] = _claimed("finalize", env, run_dir=run)
    return sa.finalize_stage2_evidence(**kwargs)


def _recover(env, verified=None, **overrides):
    return sa.recover_stage2_durability(python_root=env.root, provenance=_fin_prov() | overrides,
                                        verified=verified or _claimed("archive", env))


def _archive(env) -> Path:
    return env.root / "baselines" / "i074_stage2" / "evidence"


def _load(path: Path, kind: str):
    return ev.load_canonical_evidence_artifact(path, kind).parsed


def _manifest(env):
    return _load(_archive(env) / sa.STAGE2_MANIFEST_NAME, sa.STAGE2_MANIFEST_KIND)


def _write_manifest(env, manifest):
    (_archive(env) / sa.STAGE2_MANIFEST_NAME).write_bytes(canonical_json_bytes(manifest))


def _resync_canonical(env, rel, payload):
    """改一個 canonical 成員，並**同步**改 manifest 的 metadata（讓 SHA 那一層擋不下，逼 graph 出手）。"""
    raw = canonical_json_bytes(payload)
    blob = canonical_gzip_bytes(raw)
    (_archive(env) / rel).write_bytes(blob)
    manifest = _manifest(env)
    manifest["files"][rel] = {"artifact_sha256": sha256_hex(raw), "stored_sha256": sha256_hex(blob),
                              "stored_bytes": len(blob)}
    _write_manifest(env, manifest)


def _rewrite_comparison(env, mutate):
    """改 comparison、依新內容重建 report，兩者都同步 manifest。"""
    comparison = _load(_archive(env) / sa.COMPARISON, sa.COMPARISON_KIND)
    mutate(comparison)
    _resync_canonical(env, sa.COMPARISON, comparison)
    report = build_report(bundle_id=comparison["bundle_id"], before_ref=comparison["before_ref"],
                          comparison_sha256=sha256_hex(canonical_json_bytes(comparison)), generated_at=GEN,
                          rows=comparison["rows"], report_max_rows=sa.STAGE2_REPORT_MAX_ROWS)
    _resync_canonical(env, sa.REPORT, report)


@pytest.fixture
def published(env):
    assert _finalize(env, _write_run(env)) == 0
    return env


# ── 正向 ────────────────────────────────────────────────────────────────────

def test_finalize_publishes_the_closed_layout(published):
    env = published
    root = _archive(env)
    assert s2.archive_file_set(root) == set(sa.STAGE2_LAYOUT) | {sa.STAGE2_MANIFEST_NAME}
    manifest = _manifest(env)
    assert manifest["terminal_outcome"] == 0 and manifest["expected_image_id"] == NEW_IMG
    assert manifest["files"][sa.COUNTERFACTUAL_PATCH] == {"stored_sha256": sha256_hex(CF), "stored_bytes": len(CF)}
    # ⚠️ patch 是 raw bytes：⛔ 不壓縮、⛔ 不包進 JSON。
    assert (root / sa.COUNTERFACTUAL_PATCH).read_bytes() == CF
    assert manifest["environment_witness"]["manifest_path"] == "python/baselines/i074_stage2/envcheck/evidence_manifest.json"
    comparison = _load(root / sa.COMPARISON, sa.COMPARISON_KIND)
    assert [row_key(r["after"]) for r in comparison["rows"]] == COHORT
    assert all("lifecycle_phase" in r["differences"] for r in comparison["rows"])


def test_f_empty_tooling_patch(published):
    patches = _manifest(published)["patches"]
    assert patches["tooling_patch_sha256"] == s2.EMPTY_SHA256
    assert patches["composed_sha256"] == patches["counterfactual_patch_sha256"] == sha256_hex(CF)
    assert patches["ordered_components"] == ["counterfactual", "tooling"]


def test_non_empty_tooling_patch(env):
    composed = "c" * 64
    run = _write_run(env, prov=_before_prov(tooling_patch_sha256=composed), tooling=TOOLING)
    assert _finalize(env, run) == 0
    patches = _manifest(env)["patches"]
    assert (patches["tooling_patch_sha256"], patches["composed_sha256"]) == (sha256_hex(TOOLING), composed)
    assert _recover(env) == 0


def test_l_recovery_returns_the_manifest_outcome(published, monkeypatch):
    assert _recover(published) == 0
    # ⚠️ ⛔ 不得寫死 0：把合法值集合擴大、改寫 manifest，recovery 要回讀到的那個值。
    monkeypatch.setattr(sa, "STAGE2_TERMINAL_OUTCOMES", (0, 4))
    manifest = _manifest(published)
    manifest["terminal_outcome"] = 4
    _write_manifest(published, manifest)
    assert _recover(published) == 4


def test_republish_is_rejected(published):
    with pytest.raises(ArtifactError, match="已存在"):
        _finalize(published, _write_run(published, name="again"))


# ── a／b／c／e／z：layout、entry 型別、patch 宣告值 ────────────────────────

def test_a_extra_and_missing_files(published):
    (_archive(published) / "extra.json").write_text("{}")
    with pytest.raises(ArtifactError, match="檔案集合"):
        _recover(published)
    (_archive(published) / "extra.json").unlink()
    (_archive(published) / sa.REPORT).unlink()
    with pytest.raises(ArtifactError):
        _recover(published)


@pytest.mark.parametrize("mutate,match", [
    (lambda m: m["files"].__setitem__(sa.BEFORE_SOURCE, {"stored_sha256": "0" * 64, "stored_bytes": 1}), "欄位集合"),
    (lambda m: m["files"].__setitem__(sa.COUNTERFACTUAL_PATCH, m["files"][sa.COUNTERFACTUAL_PATCH] | {"artifact_sha256": "0" * 64}), "raw_blob"),
    (lambda m: m["files"][sa.COUNTERFACTUAL_PATCH].__setitem__("stored_sha256", "0" * 64), "stored_sha256"),
    (lambda m: m["patches"].__setitem__("ordered_components", ["tooling", "counterfactual"]), "ordered_components"),
    (lambda m: m["patches"].__setitem__("ordered_components", ["counterfactual", "tooling", "extra"]), "ordered_components"),
    (lambda m: m["patches"].__setitem__("tooling_patch_sha256", None), "null"),
    (lambda m: m["files"].__setitem__(sa.COUNTERFACTUAL_PATCH, {"stored_sha256": s2.EMPTY_SHA256, "stored_bytes": 0}), "0 bytes"),
    (lambda m: m.__setitem__("extra", 1), "欄位集合"),
    (lambda m: m.__setitem__("kind", ec.ENVCHECK_MANIFEST_KIND), "kind"),
    (lambda m: m.__setitem__("terminal_outcome", 7), "terminal_outcome"),
    (lambda m: m.__setitem__("terminal_outcome", False), "terminal_outcome"),
    (lambda m: m["environment_witness"].__setitem__("manifest_path", "python/baselines/x/evidence_manifest.json"), "寫死常數"),
    (lambda m: m["environment_witness"]["members"].pop(ec.EQUIVALENCE), "members"),
], ids=["b_json_gz_as_raw_blob", "raw_blob_with_artifact_sha", "c_patch_sha_mismatch", "e_reversed",
        "e_extra_component", "tooling_sha_null", "z_empty_counterfactual", "extra_field", "wrong_kind",
        "outcome_not_allowed", "outcome_bool", "witness_path", "witness_member_missing"])
def test_manifest_schema(published, mutate, match):
    manifest = _manifest(published)
    mutate(manifest)
    with pytest.raises(ArtifactError, match=match):
        sa.validate_stage2_manifest(manifest)


def test_z_empty_counterfactual_patch_is_not_published(env):
    with pytest.raises(ArtifactError, match="0 bytes"):
        _finalize(env, _write_run(env, cf=b""))
    assert not _archive(env).exists()


def test_c_archived_patch_bytes_changed(published):
    (_archive(published) / sa.COUNTERFACTUAL_PATCH).write_bytes(CF + b"\n")
    with pytest.raises(ArtifactError, match="實際 bytes"):
        _recover(published)


def test_d_composed_must_bind_to_the_before_provenance(env):
    run = _write_run(env, prov=_before_prov(tooling_patch_sha256="c" * 64), tooling=TOOLING)
    _finalize(env, run)
    manifest = _manifest(env)
    manifest["patches"]["composed_sha256"] = "d" * 64
    _write_manifest(env, manifest)
    with pytest.raises(ArtifactError, match="合成關係"):
        _recover(env)


# ── g／h：Stage 1 信任錨在 recovery 也要重做 ─────────────────────────────────

def test_g_stage1_after_replaced(published):
    after = published.root / "baselines" / "i074_stage1" / s2.STAGE1_ANCHOR_AFTER
    payload = gzip.decompress(after.read_bytes())
    after.write_bytes(canonical_gzip_bytes(payload.replace(b"TESTING", b"CONFIRMED", 1)))
    with pytest.raises(ArtifactError, match="一起保存"):
        _recover(published)


def test_h_stage1_after_missing(published):
    (published.root / "baselines" / "i074_stage1" / s2.STAGE1_ANCHOR_AFTER).unlink()
    with pytest.raises(ArtifactError, match="一起保存"):
        _recover(published)


# ── i／j：rename 之前失敗 → 無 archive；rename 之後 fsync 失敗 → 保留、rc=3 ─────

def _no_staging(env):
    parent = _archive(env).parent
    return not [p for p in parent.iterdir() if p.name.startswith(".evidence.staging")]


def test_i_verify_failure_leaves_no_archive(env, monkeypatch):
    def boom(*a, **k):
        raise ArtifactError("staging 驗證失敗")
    monkeypatch.setattr(sa, "verify_stage2_graph", boom)
    with pytest.raises(ArtifactError, match="staging"):
        _finalize(env, _write_run(env))
    assert not _archive(env).exists() and _no_staging(env)


def test_i_staging_fsync_failure_leaves_no_archive(env, monkeypatch):
    def boom(root):
        raise OSError("fsync boom")
    monkeypatch.setattr(s2, "_fsync_tree", boom)
    with pytest.raises(OSError):
        _finalize(env, _write_run(env))
    assert not _archive(env).exists() and _no_staging(env)


def test_i_bad_source_leaves_no_archive(env):
    run = _write_run(env)
    (run / sa.OPERATIONAL_REPORT).write_bytes(b"{}")
    with pytest.raises(ArtifactError):
        _finalize(env, run)
    assert not _archive(env).exists() and _no_staging(env)


def test_j_parent_fsync_failure_keeps_archive(env, monkeypatch):
    real = s2.fsync_dir

    def failing(path):
        if Path(path) == _archive(env).parent:
            raise OSError("boom")
        return real(path)

    monkeypatch.setattr(s2, "fsync_dir", failing)
    with pytest.raises(DurabilityUnconfirmed):
        _finalize(env, _write_run(env))
    assert _archive(env).is_dir()
    monkeypatch.setattr(s2, "fsync_dir", real)
    assert _recover(env) == 0


# ── k／k2：recovery 的四道 ＋ 「metadata 全對、跨檔卻矛盾」 ─────────────────────

def test_k_metadata_mismatch(published):
    manifest = _manifest(published)
    manifest["files"][sa.REPORT]["stored_bytes"] += 1
    _write_manifest(published, manifest)
    with pytest.raises(ArtifactError, match="metadata"):
        _recover(published)


def test_k_member_bytes_changed(published):
    path = _archive(published) / sa.IDENTITY
    path.write_bytes(canonical_gzip_bytes(canonical_json_bytes(tec._identity(created_at="2026-09-30T00:00:00+08:00"))))
    with pytest.raises(ArtifactError, match="metadata"):
        _recover(published)


@pytest.mark.parametrize("field", ev.RECOVERY_IDENTITY_FIELDS)
def test_k_recovery_rejects_execution_identity_drift(published, field):
    other = {"image_digest": "sha256:" + "8" * 64, "base_commit": "1" * 40, "tooling_patch_sha256": "2" * 64,
             "runner_sha256": "3" * 64, "source_root": "/elsewhere", "runtime_settings": {"db_driver": "sqlite"}}
    with pytest.raises(ArtifactError, match=field):
        _recover(published, **{field: other[field]})


def test_k2_graph_catches_consistent_metadata_with_cross_file_contradiction(published):
    def mutate(comparison):
        row = comparison["rows"][0]
        after = dict(row["after"], event_signal="FORGED")
        comparison["rows"][0] = compare_rows(row["before"], after)
    _rewrite_comparison(published, mutate)
    with pytest.raises(ArtifactError, match="道 3"):
        _recover(published)


# ── r／bb／bc：「①之三」與 candidate_keys ─────────────────────────────────────

def _rows_with(**by_date):
    rows = _before_rows()
    for row in rows:
        row.update(by_date.get(row["as_of"], {}))
    return rows


@pytest.mark.parametrize("changes,reason", [
    # r：RR 沒加回去（flag 正確）→ rr_not_restored
    ({"2026-08-21": {"lifecycle_phase": "CONTINUATION", "rr_decoupling_candidate": True}}, "rr_not_restored"),
    # bb：CONTINUATION 且 setup_rr 為 false、但 flag 為 false（候選集合是空的，⛔ 只看它會放行）
    ({"2026-08-21": {"lifecycle_phase": "CONTINUATION", "rr_decoupling_candidate": False}}, "candidate_flag_inconsistent"),
    # bc：flag 與等價式不符
    ({"2026-08-20": {"rr_decoupling_candidate": True}}, "candidate_flag_inconsistent"),
], ids=["r_candidates_nonempty", "bb_flag_false_but_continuation", "bc_flag_mismatch"])
def test_counterfactual_effect_is_enforced(env, changes, reason):
    with pytest.raises(ArtifactError, match=reason):
        _finalize(env, _write_run(env, before_rows=_rows_with(**changes)))
    assert not _archive(env).exists()


def test_effect_check_order_and_sample_cap():
    check = sa.CounterfactualEffectCheck()
    rows = [tre._row(f"2026-07-{i:02d}", lifecycle_phase="CONTINUATION", rr_decoupling_candidate=True) for i in range(1, 26)]
    for row in rows:
        check.feed(row)
    result = check.result()
    assert result["failure_reason"] == sa.FAILURE_RR_NOT_RESTORED
    diag = result["bounded_diagnostics"]
    # bk：完整計數照樣保留，只有 sample 被截到共用上限。
    assert diag["before_candidate_count"] == 25 and len(diag["sample_keys"]) == s2.DIAGNOSTIC_SAMPLE_LIMIT == 20
    sa.validate_bounded_diagnostics(result["failure_reason"], diag)
    # ⚠️ 同時違反時一律是 candidate_flag_inconsistent（flag 不可信，候選集合也不可信）。
    check.feed(tre._row("2026-06-01", lifecycle_phase="CONTINUATION", rr_decoupling_candidate=False))
    assert check.result()["failure_reason"] == sa.FAILURE_CANDIDATE_FLAG_INCONSISTENT
    clean = sa.CounterfactualEffectCheck()
    for row in _before_rows():
        clean.feed(row)
    assert clean.result() is None


# ── s／t／u／v：comparison 與 report ────────────────────────────────────────

def test_s_comparison_before_differs_from_source(published):
    def mutate(comparison):
        row = comparison["rows"][0]
        comparison["rows"][0] = compare_rows(dict(row["before"], event_signal="FORGED"), row["after"])
    _rewrite_comparison(published, mutate)
    with pytest.raises(ArtifactError, match="道 3"):
        _recover(published)


def test_t_differences_tampered():
    comparison = _comparison_payload()
    comparison["rows"][0]["differences"] = []
    with pytest.raises(ArtifactError, match="重算"):
        validate_comparison_artifact(comparison)


@pytest.mark.parametrize("shape", ["fewer", "more", "reordered"])
def test_u_comparison_keys_must_equal_the_cohort(published, shape):
    def mutate(comparison):
        rows = comparison["rows"]
        if shape == "fewer":
            rows.pop()
        elif shape == "more":
            extra = tre._row("2026-08-20")
            rows.insert(0, compare_rows(extra, extra))
        else:
            rows.reverse()
    if shape == "reordered":
        # 換序在單檔 validator 就被擋下（道 13），⛔ 不需要走到 graph。
        comparison = _load(_archive(published) / sa.COMPARISON, sa.COMPARISON_KIND)
        mutate(comparison)
        with pytest.raises(ArtifactError, match="排序"):
            validate_comparison_artifact(comparison)
        return
    _rewrite_comparison(published, mutate)
    with pytest.raises(ArtifactError, match="道 4"):
        _recover(published)


def _comparison_payload(n=3):
    rows = []
    for i in range(n):
        before = tre._row(f"2026-08-{10 + i:02d}")
        after = dict(before, lifecycle_phase="CONTINUATION", rr_decoupling_candidate=True)
        rows.append(compare_rows(before, after))
    return build_comparison_artifact(bundle_id=BUNDLE, before_ref=BASE, after_artifact_sha256="a" * 64,
                                     generated_at=GEN, provenance=_before_prov(), rows=rows)


def _report_for(comparison, max_rows):
    return build_report(bundle_id=BUNDLE, before_ref=BASE, comparison_sha256="b" * 64, generated_at=GEN,
                        rows=comparison["rows"], report_max_rows=max_rows)


def test_v_report_validator():
    comparison = _comparison_payload()
    ok = _report_for(comparison, 2)
    validate_report(ok, comparison=comparison, comparison_sha256="b" * 64, report_max_rows=2)
    bad_order = dict(ok, rows=[ok["rows"][1], ok["rows"][0]])
    truncated_stats = dict(ok, difference_field_counts={"lifecycle_phase": 2, "rr_decoupling_candidate": 2})
    truncated_count = dict(ok, candidate_rows=2)
    empty = dict(ok, rows_shown=0, rows=[])
    other_sha = ok
    for bad, sha, match in (
        (bad_order, "b" * 64, "排序"), (truncated_stats, "b" * 64, "difference_field_counts"),
        (truncated_count, "b" * 64, "candidate_rows"), (empty, "b" * 64, "rows_shown"),
        (other_sha, "c" * 64, "comparison_artifact_sha256"),
        (dict(ok, before_ref="f" * 40), "b" * 64, "before_ref"),
        (dict(ok, extra=1), "b" * 64, "欄位不符"),
    ):
        with pytest.raises(ArtifactError, match=match):
            validate_report(bad, comparison=comparison, comparison_sha256=sha, report_max_rows=2)


def test_comparison_validator_rejects_key_mismatch_between_sides():
    comparison = _comparison_payload()
    row = comparison["rows"][0]
    comparison["rows"][0] = dict(row, before=dict(row["before"], as_of="2026-01-01"))
    with pytest.raises(ArtifactError, match="key"):
        validate_comparison_artifact(comparison)


# ── af：before keys 恰好等於已錨定 D+1 的全量 keys ───────────────────────────

@pytest.mark.parametrize("shape,match", [
    ("more", "全量 keys"), ("fewer", "全量 keys"), ("reordered", "排序"), ("duplicated", "重複"),
])
def test_af_before_keys_must_equal_the_anchored_after(env, shape, match):
    rows = _before_rows()
    if shape == "more":
        rows.append(tre._row("2026-08-26"))
    elif shape == "fewer":
        rows = rows[1:]
    elif shape == "reordered":
        rows[0], rows[1] = rows[1], rows[0]
    else:
        rows.append(dict(rows[0]))
    with pytest.raises(ArtifactError, match=match):
        _finalize(env, _write_run(env, before_rows=rows))


# ── ag／ag2／ag3：全圖第 6～13 道 ────────────────────────────────────────────

def _prov_mismatch_comparison():
    return _before_prov(argv=["--something-else"])


@pytest.mark.parametrize("case,match", [
    ("before_ref_differs", "道 8"),
    ("base_not_stage1", "道 8"),
    ("timeframe", "道 9"),
    ("provenance_differs", "道 10"),
    ("report_before_ref", "before_ref"),
    ("empty_report", "rows_shown"),
    ("comparison_unsorted", "排序"),
    ("bundle_chain", "道 6"),
    ("image_differs", "道 7"),
    ("before_without_provenance", "欄位集合"),
    ("stage0_role_null_base", "base_commit"),
])
def test_ag_graph_rules(env, case, match):
    kwargs = {}
    if case == "before_ref_differs":
        kwargs["before_overrides"] = {"before_ref": "f" * 40}
    elif case == "base_not_stage1":
        kwargs["prov"] = _before_prov(base_commit="1" * 40)
    elif case == "timeframe":
        kwargs["before_overrides"] = {"timeframe": "1h"}
    elif case == "provenance_differs":
        kwargs["comparison_prov"] = _prov_mismatch_comparison()
    elif case == "report_before_ref":
        kwargs["mutate_report"] = lambda r: r.update(before_ref="f" * 40)
    elif case == "empty_report":
        kwargs["mutate_report"] = lambda r: r.update(rows_shown=0, rows=[])
    elif case == "comparison_unsorted":
        kwargs["mutate_comparison"] = lambda c: c["rows"].reverse()
    elif case == "bundle_chain":
        kwargs["before_overrides"] = {"bundle_id": "b1_other_bundle"}
    elif case == "image_differs":
        kwargs["prov"] = _before_prov(image_digest=OLD_IMG)
    elif case == "before_without_provenance":
        kwargs["before_overrides"] = {"provenance": None}
    elif case == "stage0_role_null_base":
        kwargs["comparison_prov"] = _before_prov(base_commit=None)
    run = _write_run(env, **kwargs)
    verified = _verified(kwargs.get("prov") or _before_prov())
    if case == "before_without_provenance":
        before = json.loads((run / sa.OPERATIONAL_BEFORE_SOURCE).read_bytes())
        del before["provenance"]
        (run / sa.OPERATIONAL_BEFORE_SOURCE).write_bytes(canonical_json_bytes(before))
    with pytest.raises((ArtifactError, ProvenanceError), match=match):
        _finalize(env, run, verified=verified)
    assert not _archive(env).exists()


def test_ag3_finalizer_provenance_image_must_match(env):
    with pytest.raises(ArtifactError, match="道 7|image"):
        _finalize(env, _write_run(env), provenance_factory=lambda: _fin_prov(image_digest=OLD_IMG))


def test_manifest_stage1_evidence_must_match_the_anchor(published):
    manifest = _manifest(published)
    manifest["stage1_evidence"]["manifest_sha256"] = "0" * 64
    _write_manifest(published, manifest)
    with pytest.raises(ArtifactError, match="Stage 1 信任錨"):
        _recover(published)


def test_manifest_environment_witness_must_match(published):
    manifest = _manifest(published)
    manifest["environment_witness"]["manifest_sha256"] = "0" * 64
    _write_manifest(published, manifest)
    with pytest.raises(ArtifactError, match="環境見證錨"):
        _recover(published)


# ── ai／16：Stage 2 identity 必須逐位元等於 envcheck 封存的那一份 ────────────────

def test_ai_identity_differing_only_in_created_at_is_rejected(env):
    with pytest.raises(ArtifactError, match="不完全相同"):
        _finalize(env, _write_run(env), stage2_identity=tec._identity(created_at="2026-09-30T09:00:00+08:00"))


def test_16_archived_identity_must_be_the_envcheck_one(published):
    _resync_canonical(published, sa.IDENTITY, tec._identity(created_at="2026-09-30T09:00:00+08:00"))
    with pytest.raises(ArtifactError, match="不完全相同"):
        _recover(published)


# ── bd2：合法的 NOT_EQUIVALENT archive ⛔ 不得放行 Stage 2（E3b） ─────────────────

def test_bd2_not_equivalent_envcheck_blocks_stage2(tmp_path, monkeypatch):
    rows = _d1_rows()
    rows[0]["event_signal"] = "DIFFERENT"          # 見證趟有一列的 bytes 不同
    env = _build_env(tmp_path, monkeypatch, witness_rows=rows)
    with pytest.raises(ArtifactError, match="E3b"):
        _finalize(env, _write_run(env))
    with pytest.raises(ArtifactError, match="E3b"):
        sa.check_failed_records(python_root=env.root)
    with pytest.raises(ArtifactError, match="E3b"):
        _publish_failure(env, _write_failure_run(env))


# ── Stage 1 信任錨的補充（w／ac／ad／ay／az／aw） ──────────────────────────────

def _resync_stage1(env, rel, payload):
    base = env.root / "baselines" / "i074_stage1"
    raw = canonical_json_bytes(payload)
    blob = canonical_gzip_bytes(raw)
    (base / rel).write_bytes(blob)
    path = base / ev.EVIDENCE_MANIFEST_NAME
    manifest = json.loads(path.read_bytes())
    manifest["files"][rel] = {"artifact_sha256": sha256_hex(raw), "stored_sha256": sha256_hex(blob),
                              "stored_bytes": len(blob)}
    path.write_bytes(canonical_json_bytes(manifest))
    return sha256_hex(raw)


def _stage1_member(env, rel):
    return json.loads(gzip.decompress((env.root / "baselines" / "i074_stage1" / rel).read_bytes()))


def test_ac_anchored_after_must_pass_the_full_validator(env):
    after = _stage1_member(env, s2.STAGE1_ANCHOR_AFTER)
    del after["rows"][0]["setup_rr_qualified"]
    _resync_stage1(env, s2.STAGE1_ANCHOR_AFTER, after)     # ⚠️ SHA 全部同步——只有 validator 擋得下
    with pytest.raises(ArtifactError, match="setup_rr_qualified"):
        s2.load_stage1_anchor(env.root)


@pytest.mark.parametrize("case,match", [
    ("cohort_invalid", "cohort manifest"),
    ("cohort_after_sha", "after_artifact_sha256"),
    ("cohort_keys", "key 集合"),
])
def test_ad_ay_cohort_rules_are_covered_by_the_two_helpers(env, case, match):
    cohort = _stage1_member(env, s2.STAGE1_ANCHOR_COHORT)
    if case == "cohort_invalid":
        cohort["keys"] = "not-a-list"
    elif case == "cohort_after_sha":
        cohort["after_artifact_sha256"] = "0" * 64
    else:
        cohort["keys"] = cohort["keys"][:1]
    _resync_stage1(env, s2.STAGE1_ANCHOR_COHORT, cohort)
    # ⚠️ ay：preflight／finalize／recovery 共用的兩支 helper **合起來**要擋下（單檔的在第一支、跨檔的在第二支）。
    identity = tec._identity()
    with pytest.raises(ArtifactError, match=match):
        anchor = s2.load_stage1_anchor(env.root)
        s2.validate_stage1_anchor_graph(anchor, stage2_identity=identity, envcheck_identity=identity,
                                        expected_bundle_id=BUNDLE)
    # Stage 2 的三個呼叫端（經 `_load_trust_anchors()`）同樣 fail-closed。
    with pytest.raises(ArtifactError):
        sa._load_trust_anchors(env.root, None)


@pytest.mark.parametrize("field,value,match", [
    ("bundle_id", "b1_another_legal_bundle", "bundle_id 等式鏈"),
    ("image", "sha256:" + "5" * 64, "image 等式鏈"),
])
def test_az_legal_but_unchained_members_are_rejected(env, field, value, match):
    after = _stage1_member(env, s2.STAGE1_ANCHOR_AFTER)
    cohort = _stage1_member(env, s2.STAGE1_ANCHOR_COHORT)
    for payload in (after, cohort):
        if field == "bundle_id":
            payload["bundle_id"] = value
        else:
            payload["provenance"]["image_digest"] = value
    cohort["after_artifact_sha256"] = _resync_stage1(env, s2.STAGE1_ANCHOR_AFTER, after)
    _resync_stage1(env, s2.STAGE1_ANCHOR_COHORT, cohort)
    anchor = s2.load_stage1_anchor(env.root)         # ⚠️ 單檔層面全部合法
    identity = tec._identity()
    with pytest.raises(ArtifactError, match=match):
        s2.validate_stage1_anchor_graph(anchor, stage2_identity=identity, envcheck_identity=identity,
                                        expected_bundle_id=BUNDLE)


def test_w_stage1_manifest_sha_and_members_in_the_stage2_manifest(published):
    manifest = _manifest(published)
    manifest["stage1_evidence"]["members"][s2.STAGE1_ANCHOR_COHORT]["stored_sha256"] = "0" * 64
    _write_manifest(published, manifest)
    with pytest.raises(ArtifactError, match="Stage 1 信任錨"):
        _recover(published)


def test_aw_anchor_reads_each_member_once(env, monkeypatch):
    seen: list[str] = []
    real_load, real_stream = s2.load_canonical_evidence_artifact, s2.stream_after_artifact

    def load(path, kind):
        seen.append(str(path))
        return real_load(path, kind)

    def stream(path, **kwargs):
        seen.append(str(path))
        return real_stream(path, **kwargs)

    monkeypatch.setattr(s2, "load_canonical_evidence_artifact", load)
    monkeypatch.setattr(s2, "stream_after_artifact", stream)
    anchor = s2.load_stage1_anchor(env.root)
    assert len(seen) == len(set(seen)) == 4          # manifest、cohort、identity、after 各一次
    assert anchor.cohort_keys == COHORT and set(anchor.cohort_rows) == set(COHORT)


# ── bf：全量 artifact 一律串流、每份只讀一次 ──────────────────────────────────

def test_bf_full_artifacts_are_streamed_once(env, monkeypatch):
    calls: list[str] = []
    real_stream = sa.stream_canonical_artifact
    real_after = s2.stream_after_artifact
    real_load = ev.load_canonical_evidence_artifact

    def guarded_load(path, kind):
        assert kind not in (AFTER_KIND, sa.BEFORE_SOURCE_KIND), f"全量 artifact 被整份載入：{path}"
        return real_load(path, kind)

    def counting_before(path, kind, **kwargs):
        calls.append(str(path))
        return real_stream(path, kind, **kwargs)

    def counting_after(path, **kwargs):
        calls.append(str(path))
        return real_after(path, **kwargs)

    monkeypatch.setattr(sa, "stream_canonical_artifact", counting_before)
    for module in (s2, ec):
        monkeypatch.setattr(module, "stream_after_artifact", counting_after)
    for module in (sa, s2, ec):
        monkeypatch.setattr(module, "load_canonical_evidence_artifact", guarded_load)

    _finalize(env, _write_run(env))
    # D+1、after'、operational before、staging 內的 before 副本——⛔ 同一份不讀第二次。
    assert len(calls) == len(set(calls)) == 4
    calls.clear()
    _recover(env)
    assert len(calls) == len(set(calls)) == 3       # D+1、after'、封存的 before


# ── bj：report 的 200 就是 bundle 的 report_max_rows ─────────────────────────

def test_bj_report_cap_equals_the_bundle_value():
    python_root = Path(__file__).resolve().parents[4]
    manifest = python_root / "baselines" / "b1_20260901_1d_74350966_5d7ecb10" / "manifest.json"
    if not manifest.is_file():
        pytest.skip("找不到正式 bundle")
    assert json.loads(manifest.read_text(encoding="utf-8"))["report_max_rows"] == sa.STAGE2_REPORT_MAX_ROWS == 200


# ── failed-attempt record（「七」） ─────────────────────────────────────────

def _failure_rows():
    """rr_not_restored：兩列候選的 RR 沒被加回去（flag 正確）。"""
    return _d1_rows()


def _effect(rows):
    check = sa.CounterfactualEffectCheck()
    for row in rows:
        check.feed(row)
    return check.result()


def _write_failure_run(env, *, effect=None, prov=None, cf=CF, tooling=b"", name="fail_run", mutate=None):
    run = env.tmp / name
    (run / "stage2").mkdir(parents=True)
    (run / "patches").mkdir()
    payload = sa.build_counterfactual_failure(bundle_id=BUNDLE, generated_at=GEN,
                                              provenance=prov or _before_prov(),
                                              effect=effect or _effect(_failure_rows()))
    if mutate:
        mutate(payload)
    (run / sa.OPERATIONAL_FAILURE).write_bytes(canonical_json_bytes(payload))
    (run / sa.FROZEN_COUNTERFACTUAL_PATCH).write_bytes(cf)
    (run / sa.FROZEN_TOOLING_PATCH).write_bytes(tooling)
    return run


def _publish_failure(env, run, **overrides):
    kwargs = {"python_root": env.root, "run_dir": run, "stage2_identity": tec._identity(), "generated_at": GEN}
    kwargs.update(overrides)
    if "verified" not in kwargs:
        kwargs["verified"] = _claimed("failure", env, run_dir=run)
    return sa.publish_failed_record(**kwargs)


def _recover_record(env, root, **overrides):
    kwargs = {"python_root": env.root, "record_dir": root}
    kwargs.update(overrides)
    if "verified" not in kwargs:
        kwargs["verified"] = _claimed("failed-record", env, record_dir=root)
    return sa.recover_failed_record(**kwargs)


def _failed_root(env) -> Path:
    return env.root / "baselines" / "i074_stage2" / "failed"


@pytest.fixture
def recorded(env):
    root = _publish_failure(env, _write_failure_run(env))
    return env, root


def _record(root):
    return json.loads((root / sa.FAILED_RECORD_NAME).read_bytes())


def _write_record(root, record):
    (root / sa.FAILED_RECORD_NAME).write_bytes(canonical_json_bytes(record))


def test_m_failed_record_is_published_and_identifiable(recorded):
    env, root = recorded
    assert root.parent == _failed_root(env)
    assert root.name == f"{BUNDLE}-{sha256_hex(CF)}"               # ⚠️ 完整 64 碼
    assert s2.archive_file_set(root) == {sa.FAILED_RECORD_NAME, sa.COUNTERFACTUAL_PATCH, sa.TOOLING_PATCH}
    record = _record(root)
    assert record["kind"] == sa.FAILED_ATTEMPT_KIND and record["failure_reason"] == sa.FAILURE_RR_NOT_RESTORED
    assert record["bounded_diagnostics"]["before_candidate_count"] == 2
    assert record["run_identity"] == tec._identity()
    assert (root / sa.COUNTERFACTUAL_PATCH).read_bytes() == CF
    assert not _archive(env).exists()                                  # ⛔ 不是成功 archive


def test_check_lists_valid_records(recorded):
    env, root = recorded
    result = sa.check_failed_records(python_root=env.root)
    assert result["base_commit"] == BASE
    assert result["records"] == [{
        "dir": str(root), "base_commit": BASE, "counterfactual_patch_sha256": sha256_hex(CF),
        "tooling_patch_sha256": s2.EMPTY_SHA256, "composed_sha256": sha256_hex(CF),
    }]


def test_ak_no_or_empty_failed_root(env):
    assert sa.check_failed_records(python_root=env.root)["records"] == []
    _failed_root(env).mkdir(parents=True)
    assert sa.check_failed_records(python_root=env.root)["records"] == []


def test_al_two_records_with_different_shas(recorded):
    env, _ = recorded
    other = CF.replace(b"+b", b"+c")
    _publish_failure(env, _write_failure_run(env, cf=other, prov=_before_prov(tooling_patch_sha256=sha256_hex(other)),
                                             name="fail_run2"))
    shas = {r["counterfactual_patch_sha256"] for r in sa.check_failed_records(python_root=env.root)["records"]}
    assert shas == {sha256_hex(CF), sha256_hex(other)}


def test_same_sha_cannot_be_recorded_twice(recorded):
    env, _ = recorded
    with pytest.raises(ArtifactError, match="已存在"):
        _publish_failure(env, _write_failure_run(env, name="fail_again"))


def _tamper(recorded, mutate):
    env, root = recorded
    record = _record(root)
    mutate(record)
    _write_record(root, record)
    return env


@pytest.mark.parametrize("mutate,match", [
    (lambda r: r["run_identity"].__setitem__("kind", "other"), "run identity|kind"),              # F1
    (lambda r: r.__setitem__("bundle_id", "b1_other"), "F2"),                                      # F2
    (lambda r: r["provenance"].__setitem__("image_digest", OLD_IMG), "F3"),                        # F3 image
    (lambda r: r["provenance"].__setitem__("tooling_patch_sha256", "c" * 64), "F3"),               # F3 composed
    (lambda r: r.__setitem__("failure_reason", "before_candidates_nonempty"), "F5"),               # F5
    (lambda r: r["bounded_diagnostics"].__setitem__("before_candidate_count", 3), "F6-a"),         # 計數與 sample
    (lambda r: r["bounded_diagnostics"]["sample_keys"][0].__setitem__("setup_rr_qualified", 0), "F7"),
    (lambda r: r["bounded_diagnostics"]["sample_keys"][0].__setitem__("lifecycle_phase", ""), "F6-c"),
    (lambda r: r["bounded_diagnostics"]["sample_keys"][0].pop(sa.CANDIDATE_FIELD), "F6-b"),
    (lambda r: r.__setitem__("extra", 1), "欄位集合"),
    (lambda r: r["files"][sa.TOOLING_PATCH].__setitem__("stored_bytes", 1), "tooling|bytes|0"),
], ids=["F1", "F2", "F3_image", "F3_composed", "F5_old_value", "F6a_count", "F7_int_bool", "F6c_empty",
        "F6b_missing", "extra_field", "files_entry"])
def test_ah_invariants_fail_closed(recorded, mutate, match):
    env = _tamper(recorded, mutate)
    with pytest.raises((ArtifactError, ValueError), match=match):
        sa.check_failed_records(python_root=env.root)


def test_ah2_record_differing_only_in_created_at_is_rejected(recorded):
    env = _tamper(recorded, lambda r: r["run_identity"].__setitem__("created_at", "2026-09-30T09:00:00+08:00"))
    with pytest.raises(ArtifactError, match="F2-a|F2"):
        sa.check_failed_records(python_root=env.root)


def test_ah2_record_with_whole_other_identity_is_rejected(recorded):
    other = tec._identity(created_at="2026-09-30T09:00:00+08:00")
    env = _tamper(recorded, lambda r: r.__setitem__("run_identity", other))
    with pytest.raises(ArtifactError, match="F2-a"):
        sa.check_failed_records(python_root=env.root)


def test_f3_base_must_be_the_stage1_after_base(recorded):
    env = _tamper(recorded, lambda r: r["provenance"].__setitem__("base_commit", "1" * 40))
    with pytest.raises(ArtifactError, match="F3"):
        sa.check_failed_records(python_root=env.root)


def test_f4_renamed_directory_is_rejected(recorded):
    env, root = recorded
    root.rename(root.parent / f"{BUNDLE}-{'0' * 64}")
    with pytest.raises(ArtifactError, match="F4"):
        sa.check_failed_records(python_root=env.root)


def test_f8_patch_bytes_changed(recorded):
    env, root = recorded
    (root / sa.COUNTERFACTUAL_PATCH).write_bytes(CF + b" ")
    with pytest.raises(ArtifactError, match="實際 bytes"):
        sa.check_failed_records(python_root=env.root)


@pytest.mark.parametrize("damage", ["extra_file", "missing_patch", "missing_record", "stray_dir", "stray_file"])
def test_aa_an_damaged_records_fail_closed(recorded, damage):
    env, root = recorded
    if damage == "extra_file":
        (root / "notes.txt").write_text("x")
    elif damage == "missing_patch":
        (root / sa.TOOLING_PATCH).unlink()
    elif damage == "missing_record":
        (root / sa.FAILED_RECORD_NAME).unlink()
    elif damage == "stray_dir":
        (root.parent / ".stale.staging-0000").mkdir()      # 發布中斷的殘骸 ⛔ 不得忽略
    else:
        (root.parent / "README").write_text("x")
    with pytest.raises(ArtifactError):
        sa.check_failed_records(python_root=env.root)


def test_ab_recover_failed_record(recorded):
    env, root = recorded
    summary = _recover_record(env, root)
    assert summary["counterfactual_patch_sha256"] == sha256_hex(CF)


def test_recover_failed_record_rejects_paths_outside_the_failed_root(recorded, tmp_path):
    env, root = recorded
    elsewhere = tmp_path / "copy" / root.name
    elsewhere.parent.mkdir()
    import shutil
    shutil.copytree(root, elsewhere)
    with pytest.raises(ArtifactError, match="只接受"):
        _recover_record(env, elsewhere)


def test_p_publish_outcomes(env, monkeypatch):
    # ① rename 之前失敗 → 沒有任何紀錄（允許以同一 SHA 重跑）
    bad = _write_failure_run(env, name="bad", mutate=lambda p: p.update(failure_reason="nope"))
    with pytest.raises(ArtifactError, match="F5"):
        _publish_failure(env, bad)
    assert not _failed_root(env).exists() or not list(_failed_root(env).iterdir())
    # ② rename 成功、parent fsync 失敗 → rc=3，紀錄保留（lookup 讀得到＝視同已記錄）
    real = s2.fsync_dir

    def failing(path):
        if Path(path) == _failed_root(env):
            raise OSError("boom")
        return real(path)

    monkeypatch.setattr(s2, "fsync_dir", failing)
    with pytest.raises(DurabilityUnconfirmed):
        _publish_failure(env, _write_failure_run(env))
    monkeypatch.setattr(s2, "fsync_dir", real)
    assert len(sa.check_failed_records(python_root=env.root)["records"]) == 1
    # ③ recovery 補 fsync 之後仍是「同 SHA 不得重跑」
    _recover_record(env, _failed_root(env) / f"{BUNDLE}-{sha256_hex(CF)}")


def test_failure_intermediate_must_match_the_identity(env):
    with pytest.raises(ArtifactError, match="bundle_id"):
        _publish_failure(env, _write_failure_run(env, mutate=lambda p: p.update(bundle_id="b1_other")))


def test_failed_record_rejects_empty_counterfactual(env):
    with pytest.raises(ArtifactError, match="0 bytes"):
        _publish_failure(env, _write_failure_run(env, cf=b"", prov=_before_prov(tooling_patch_sha256=s2.EMPTY_SHA256)))


# ── ah4／ah5：bounded_diagnostics 的 union 與 F10 ─────────────────────────────

def _sample(as_of, phase, rr, flag):
    return {"symbol": "2330", "timeframe": "1d", "as_of": as_of, "lifecycle_phase": phase,
            "setup_rr_qualified": rr, sa.CANDIDATE_FIELD: flag}


def test_ah4_union_shapes():
    sa.validate_bounded_diagnostics(sa.FAILURE_CANDIDATE_FLAG_INCONSISTENT, {
        "inconsistent_row_count": 1, "sample_keys": [_sample("2026-08-01", "TESTING", False, True)]})
    sa.validate_bounded_diagnostics(sa.FAILURE_RR_NOT_RESTORED, {
        "before_candidate_count": 1, "sample_keys": [_sample("2026-08-01", "CONTINUATION", False, True)]})
    bad_cases = [
        (sa.FAILURE_RR_NOT_RESTORED, {"inconsistent_row_count": 1,
                                      "sample_keys": [_sample("2026-08-01", "CONTINUATION", False, True)]}, "F6"),
        (sa.FAILURE_RR_NOT_RESTORED, {"before_candidate_count": 0, "sample_keys": []}, "F6-a"),
        (sa.FAILURE_RR_NOT_RESTORED, {"before_candidate_count": True, "sample_keys": []}, "F6-a"),
        (sa.FAILURE_RR_NOT_RESTORED, {"before_candidate_count": 1, "sample_keys": [
            {k: v for k, v in _sample("2026-08-01", "CONTINUATION", False, True).items() if k != sa.CANDIDATE_FIELD}]},
         "F6-b"),
        ("before_candidates_nonempty", {"before_candidate_count": 1, "sample_keys": []}, "F5"),
        (sa.FAILURE_RR_NOT_RESTORED, {"before_candidate_count": 2, "sample_keys": [
            _sample("2026-08-02", "CONTINUATION", False, True), _sample("2026-08-01", "CONTINUATION", False, True)]},
         "排序"),
        (sa.FAILURE_RR_NOT_RESTORED, {"before_candidate_count": 2, "sample_keys": [
            _sample("2026-08-01", "CONTINUATION", False, True), _sample("2026-08-01", "CONTINUATION", False, True)]},
         "重複"),
    ]
    for reason, diag, match in bad_cases:
        with pytest.raises(ArtifactError, match=match):
            sa.validate_bounded_diagnostics(reason, diag)


@pytest.mark.parametrize("reason,sample", [
    (sa.FAILURE_CANDIDATE_FLAG_INCONSISTENT, _sample("2026-08-01", "CONTINUATION", False, True)),   # 其實一致
    (sa.FAILURE_RR_NOT_RESTORED, _sample("2026-08-01", "CONTINUATION", True, False)),              # 沒違反
])
def test_ah5_sample_must_violate_its_claimed_rule(reason, sample):
    count_field = sa.FAILURE_COUNT_FIELD[reason]
    with pytest.raises(ArtifactError, match="F10"):
        sa.validate_bounded_diagnostics(reason, {count_field: 1, "sample_keys": [sample]})


# ── CLI（Python 段） ────────────────────────────────────────────────────────

_INJECTED = ["--python-root", "/p", "--image-digest", NEW_IMG, "--base-commit", "a" * 40,
             "--tooling-patch-sha256", "b" * 64, "--source-root", "/app", "--runner-sha256", "c" * 64]


def _verified_argv(claims=None):
    claims = claims or {"base_commit": BASE, "counterfactual_patch_sha256": "1" * 64,
                        "tooling_patch_sha256": "2" * 64, "composed_sha256": "3" * 64}
    return ["--verified-patch-base", claims["base_commit"],
            "--verified-counterfactual-sha256", claims["counterfactual_patch_sha256"],
            "--verified-tooling-sha256", claims["tooling_patch_sha256"],
            "--verified-composed-sha256", claims["composed_sha256"]]


@pytest.mark.parametrize("argv,match", [
    ([], "恰好給一個"),
    (["--finalize", "--recover-durability"], "恰好給一個"),
    (["--finalize"], "--run-dir"),
    (["--publish-failed-record", "--run-dir", "/r"], "--run-dir"),
    (["--recover-durability", "--run-dir", "/r"], "不接受"),
    (["--check-failed-record", "--run-identity", "/i"], "不接受"),
    (["--recover-failed-record", "/d", "--run-dir", "/r"], "不接受"),
    (["--recover-durability"], "--verified"),
    (["--recover-durability", *_verified_argv()[:-2]], "verified_composed_sha256"),
    (["--check-failed-record", *_verified_argv()], "不接受 --verified"),
    (["--recover-durability", *_verified_argv()[:-1], "not-hex"], "格式"),
])
def test_cli_matrix_usage_errors(argv, match):
    with pytest.raises(sa.Stage2UsageError, match=match):
        sa.run_stage2(argv + _INJECTED)


def test_cli_rejects_duplicated_injected_args():
    with pytest.raises(sa.Stage2UsageError, match="只能由官方腳本注入"):
        sa.run_stage2(["--recover-durability", *_INJECTED, "--python-root", "/q"])
    with pytest.raises(sa.Stage2UsageError, match="只能由官方腳本注入"):
        sa.run_stage2(["--recover-durability", *_INJECTED, *_verified_argv(),
                       "--verified-composed-sha256", "4" * 64])


@pytest.mark.parametrize("mode,rc", [
    ("finalize", 0), ("recover_durability", 0), ("publish_failed_record", 1),
    ("check_failed_record", 0), ("recover_failed_record", 1),
])
def test_main_exit_codes(monkeypatch, mode, rc):
    monkeypatch.setattr(sa, "run_stage2", lambda argv: (rc, {"mode": mode}))
    assert sa.main([]) == rc


def test_main_maps_durability_and_errors(monkeypatch):
    def durability(argv):
        raise DurabilityUnconfirmed("x", published=True)

    def failure(argv):
        raise ArtifactError("x")

    monkeypatch.setattr(sa, "run_stage2", durability)
    assert sa.main([]) == EXIT_DURABILITY_UNCONFIRMED
    monkeypatch.setattr(sa, "run_stage2", failure)
    assert sa.main([]) == EXIT_ABORT


def test_publish_failed_record_cli_returns_one_even_when_published(recorded, monkeypatch):
    env, root = recorded
    root_other = _write_failure_run(env, cf=CF + b"\n", prov=_before_prov(tooling_patch_sha256=sha256_hex(CF + b"\n")),
                                    name="cli_run")
    identity = env.tmp / "identity.json"
    identity.write_bytes(canonical_json_bytes(tec._identity()))
    real = sa.publish_failed_record
    seen = {}

    def spy(**kwargs):
        seen.update(kwargs)
        return real(**(kwargs | {"python_root": env.root}))    # CLI 的 --python-root 是假值，換成 fixture 的

    monkeypatch.setattr(sa, "publish_failed_record", spy)
    claims = sa.patch_claims(mode="failure", python_root=env.root, run_dir=root_other)
    rc, result = sa.run_stage2(["--publish-failed-record", "--run-dir", str(root_other),
                                "--run-identity", str(identity), *_INJECTED, *_verified_argv(claims)])
    # ⚠️ 完整發布仍回 1——那一次執行本來就是失敗的。
    assert rc == 1 and result["published"].endswith(sha256_hex(CF + b"\n"))
    assert seen["stage2_identity"] == tec._identity() and seen["run_dir"] == str(root_other)


# ── 合成守門的宣告值（給 shell） ─────────────────────────────────────────────

def test_patch_claims(published, recorded):
    env = published
    run = env.tmp / "s2run"
    assert sa.patch_claims(mode="finalize", python_root=env.root, run_dir=run) == {
        "base_commit": BASE, "counterfactual_patch_sha256": sha256_hex(CF),
        "tooling_patch_sha256": s2.EMPTY_SHA256, "composed_sha256": sha256_hex(CF)}
    assert sa.patch_claims(mode="archive", python_root=env.root)["composed_sha256"] == sha256_hex(CF)
    _, root = recorded
    assert sa.patch_claims(mode="failed-record", python_root=env.root, record_dir=root)["base_commit"] == BASE
    assert sa.patch_claims(mode="failure", python_root=env.root,
                           run_dir=env.tmp / "fail_run")["counterfactual_patch_sha256"] == sha256_hex(CF)
    with pytest.raises(ArtifactError):
        sa.patch_claims(mode="other", python_root=env.root)


def test_before_source_envelope():
    top = sa.build_before_source(bundle_id=BUNDLE, before_ref=BASE, timeframe="1d", replay_scope="all_candidates",
                                 generated_at=GEN, provenance=_before_prov(), rows=[])
    sa.validate_before_source_envelope(top)
    for bad, match in ((top | {"extra": 1}, "欄位集合"), (top | {"kind": AFTER_KIND}, "kind"),
                       (top | {"generated_at": "2026-09-25"}, "時區"), (top | {"before_ref": ""}, "before_ref")):
        with pytest.raises(ArtifactError, match=match):
            sa.validate_before_source_envelope(bad)


# ── bg（Python 端）：shell 組出的 argv fixture，Python CLI 逐一接受且分派到對的模式 ──────

@pytest.mark.parametrize("key,mode,rc", [
    ("finalize_argv", "finalize", 0),
    ("recover_durability_argv", "recover_durability", 0),
    ("publish_failed_record_argv", "publish_failed_record", 1),
    ("check_failed_record_argv", "check_failed_record", 0),
    ("recover_failed_record_argv", "recover_failed_record", 1),
])
def test_bg_fixture_argv_is_accepted_by_the_python_cli(monkeypatch, key, mode, rc):
    fixture = Path(__file__).resolve().parents[4] / "scripts" / "fixtures" / "stage2_finalizer_argv.json"
    doc = json.loads(fixture.read_text(encoding="utf-8"))
    subst = {"<RUN_DIR>": "/r", "<IDENTITY>": "/i", "<PYTHON_ROOT>": "/p", "<IMAGE_ID>": NEW_IMG,
             "<BASE_COMMIT>": "a" * 40, "<TOOLING_PATCH_SHA256>": "b" * 64, "<RUNNER_SHA256>": "c" * 64,
             "<RECORD_DIR>": "/d", "<VERIFIED_PATCH_BASE>": BASE,
             "<VERIFIED_COUNTERFACTUAL_SHA256>": "1" * 64, "<VERIFIED_TOOLING_SHA256>": "2" * 64,
             "<VERIFIED_COMPOSED_SHA256>": "3" * 64}
    argv = [subst.get(t, t) for t in doc[key]]
    assert argv[:3] == ["python", "-m", "backtest.modular.sr_scoring.replay_bundle.stage2_archive"]
    called = []
    monkeypatch.setattr(sa, "load_run_identity", lambda path: tec._identity())
    monkeypatch.setattr(sa, "finalize_stage2_evidence", lambda **kw: called.append("finalize") or 0)
    monkeypatch.setattr(sa, "recover_stage2_durability", lambda **kw: called.append("recover_durability") or 0)
    monkeypatch.setattr(sa, "publish_failed_record", lambda **kw: called.append("publish_failed_record") or Path("/x"))
    monkeypatch.setattr(sa, "check_failed_records",
                        lambda **kw: called.append("check_failed_record") or {"base_commit": BASE, "records": []})
    monkeypatch.setattr(sa, "recover_failed_record", lambda **kw: called.append("recover_failed_record") or {})
    got_rc, _ = sa.run_stage2(argv[3:])
    assert (called, got_rc) == ([mode], rc)
    # ⚠️ 除了 check，四種模式的 fixture 都必須帶交接值（⛔ 漏了就等於沒有交接）。
    assert ("--verified-composed-sha256" in argv) == (mode != "check_failed_record")


# ── 高 1（2026-09-24 review）：合成守門與封存之間的 TOCTOU ──────────────────────
#
# shell 在 Docker 之前驗 A 的合成關係，Python 之後重新讀來源。兩段之間來源被**一致地**換成 B
# （B 自身的宣告值彼此相符、Python 的內部一致性全都過得了）——⛔ 必須靠交接值擋下，且⛔ 不留任何紀錄。

CF2 = CF.replace(b"+b", b"+c")


@pytest.mark.parametrize("swap", ["tooling", "counterfactual"])
def test_toctou_finalize_rejects_inputs_swapped_after_the_composition_guard(env, swap):
    run = _write_run(env)
    verified_a = _claimed("finalize", env, run_dir=run)          # shell 驗過的是 A
    if swap == "tooling":
        b_prov = _before_prov(tooling_patch_sha256="c" * 64)
        b_run = _write_run(env, prov=b_prov, tooling=TOOLING, name="b_run")
    else:
        b_prov = _before_prov(tooling_patch_sha256=sha256_hex(CF2))
        b_run = _write_run(env, prov=b_prov, cf=CF2, name="b_run")
    # ⚠️ B 本身是一致的：不帶交接值的話，Python 會接受它（⛔ 它證明不了 B 真的合成得出）。
    for rel in (sa.OPERATIONAL_BEFORE_SOURCE, sa.OPERATIONAL_COMPARISON, sa.OPERATIONAL_REPORT,
                sa.FROZEN_COUNTERFACTUAL_PATCH, sa.FROZEN_TOOLING_PATCH):
        (run / rel).write_bytes((b_run / rel).read_bytes())
    with pytest.raises(ArtifactError, match="TOCTOU"):
        _finalize(env, run, verified=verified_a)
    assert not _archive(env).exists() and _no_staging(env)
    # 對照組：同一份 B 配上 B 自己的值 → 通過（證明被擋下的原因就是交接值）。
    assert _finalize(env, run, verified=_verified(b_prov, cf=CF2 if swap == "counterfactual" else CF,
                                                  tooling=TOOLING if swap == "tooling" else b"")) == 0


def test_toctou_publish_failed_record(env):
    run = _write_failure_run(env)
    verified_a = _claimed("failure", env, run_dir=run)
    b_run = _write_failure_run(env, cf=CF2, prov=_before_prov(tooling_patch_sha256=sha256_hex(CF2)), name="b_fail")
    for rel in (sa.OPERATIONAL_FAILURE, sa.FROZEN_COUNTERFACTUAL_PATCH, sa.FROZEN_TOOLING_PATCH):
        (run / rel).write_bytes((b_run / rel).read_bytes())
    with pytest.raises(ArtifactError, match="TOCTOU"):
        _publish_failure(env, run, verified=verified_a)
    assert not _failed_root(env).exists() or not list(_failed_root(env).iterdir())   # ⛔ 沒有留下任何紀錄


def test_toctou_recover_durability(published):
    env = published
    verified_a = _claimed("archive", env)
    # 把 archive 一致地換成 B：tooling patch 與 manifest 的 files／patches 同步改（composed 不動，仍綁 before）。
    (_archive(env) / sa.TOOLING_PATCH).write_bytes(TOOLING)
    manifest = _manifest(env)
    manifest["files"][sa.TOOLING_PATCH] = {"stored_sha256": sha256_hex(TOOLING), "stored_bytes": len(TOOLING)}
    manifest["patches"]["tooling_patch_sha256"] = sha256_hex(TOOLING)
    _write_manifest(env, manifest)
    assert _recover(env, verified=_claimed("archive", env)) == 0     # B 自身一致：⛔ Python 單獨擋不下
    with pytest.raises(ArtifactError, match="TOCTOU"):
        _recover(env, verified=verified_a)


def test_toctou_recover_failed_record(recorded):
    env, root = recorded
    verified_a = _claimed("failed-record", env, record_dir=root)
    (root / sa.TOOLING_PATCH).write_bytes(TOOLING)
    record = _record(root)
    record["files"][sa.TOOLING_PATCH] = {"stored_sha256": sha256_hex(TOOLING), "stored_bytes": len(TOOLING)}
    record["patches"]["tooling_patch_sha256"] = sha256_hex(TOOLING)
    _write_record(root, record)
    _recover_record(env, root)                                         # B 自身一致
    with pytest.raises(ArtifactError, match="TOCTOU"):
        _recover_record(env, root, verified=verified_a)


def test_missing_handoff_is_rejected(env):
    with pytest.raises(ArtifactError, match="缺少"):
        _finalize(env, _write_run(env), verified=None)
    assert not _archive(env).exists()
    with pytest.raises(ArtifactError):
        sa.VerifiedComposition(base_commit="x", counterfactual_patch_sha256="1" * 64,
                               tooling_patch_sha256="2" * 64, composed_sha256="3" * 64)


# ── 中 2（2026-09-24 review）：bounded_diagnostics 的**計算過程**也要有界 ─────────────

@pytest.mark.parametrize("reason", [sa.FAILURE_RR_NOT_RESTORED, sa.FAILURE_CANDIDATE_FLAG_INCONSISTENT])
def test_sample_buffer_stays_bounded_under_mass_violation(reason):
    n = 1000
    check = sa.CounterfactualEffectCheck()
    peak = 0
    for i in reversed(range(n)):                  # ⚠️ 反向輸入：每一筆新來的都比 buffer 裡的小
        as_of = f"2026-{i:06d}"
        if reason == sa.FAILURE_RR_NOT_RESTORED:
            row = tre._row(as_of, lifecycle_phase="CONTINUATION", rr_decoupling_candidate=True)
        else:
            row = tre._row(as_of, rr_decoupling_candidate=True)          # TESTING 卻 flag＝true
        check.feed(row)
        peak = max(peak, len(check.inconsistent), len(check.candidates))
    assert peak <= s2.DIAGNOSTIC_SAMPLE_LIMIT
    result = check.result()
    diag = result["bounded_diagnostics"]
    assert result["failure_reason"] == reason
    assert diag[sa.FAILURE_COUNT_FIELD[reason]] == n                     # 完整計數照樣保留
    assert [s["as_of"] for s in diag["sample_keys"]] == [f"2026-{i:06d}" for i in range(20)]
    sa.validate_bounded_diagnostics(reason, diag)


def test_bounded_sample_matches_full_sort_on_shuffled_input():
    import random

    rng = random.Random(20260924)
    keys = [("s", "1d", f"{i:05d}") for i in range(500)]
    rng.shuffle(keys)
    sample = sa.BoundedSample(7)
    for k in keys:
        sample.add(k, {"k": k})
        assert len(sample) <= 7
    assert sample.count == 500
    assert [s["k"] for s in sample.samples()] == sorted(keys)[:7]
