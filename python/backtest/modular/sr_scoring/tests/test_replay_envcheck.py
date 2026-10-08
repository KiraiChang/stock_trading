"""I-074 Stage 2 的環境見證（③b）：Stage 1 信任錨、環境等價判定、`envcheck/` 的發布與 recovery。

對應 issue.md I-074 ③ evidence contract（⚠️ 現行版）的「三之一」「三之三」「五之一」「七之四」，
測試編號 bd～bd4、be／be2、bf、bk，以及 Stage 2 計畫書「二、⑤」。

⚠️ fixture 用**真的 Stage 1 finalizer** 產出一份合法的 Stage 1 evidence（⛔ 不手捏 manifest）：
手捏的能過測試卻過不了正式 validator，那種 fixture 只會讓測試對不到現實。
"""
from __future__ import annotations

import gzip
from pathlib import Path

import pytest

from ..replay_bundle import envcheck as ec
from ..replay_bundle import evidence as ev
from ..replay_bundle import stage2_evidence as s2
from ..replay_bundle import stream as st
from ..replay_bundle.artifacts import AFTER_KIND, ArtifactError
from ..replay_bundle.canonical import canonical_gzip_bytes, canonical_json_bytes, sha256_hex
from ..replay_bundle.provenance import pip_freeze_sha256
from ..replay_bundle.publish import EXIT_ENV_NOT_EQUIVALENT, DurabilityUnconfirmed
from ..replay_bundle.run_identity import build_run_identity, default_run_identity_path
from . import test_i074_preflight as tip
from . import test_replay_evidence as tre
from .test_i074_preflight import _prov

OLD_IMG = tre.IMG                      # Stage 1 釘住的 image（fixture 裡的 stand-in）
NEW_IMG = "sha256:" + "9" * 64         # Stage 2 的新 image
BUNDLE = tre.BUNDLE
DISTS = [["numpy", "1.26.4"], ["pandas", "2.2.2"], ["scikit-learn", "1.9.1"]]


def _stage1_prov():
    # ⚠️ 真實的 Stage 1 after 沒套任何 patch：tooling_patch_sha256 是空字串的 SHA。
    return _prov() | {"tooling_patch_sha256": s2.EMPTY_SHA256}


@pytest.fixture
def python_root(tmp_path, monkeypatch):
    """`<tmp>/python/baselines/i074_stage1/` 是一份由正式 finalizer 產出的合法 Stage 1 evidence。"""
    # ⚠️ 兩個模組各有一份 `_prov`（probe 三份走 test_i074_preflight 那份）——都要換，
    # 否則 probe 的 completion 與 computation 的 provenance 會對不上。
    monkeypatch.setattr(tre, "_prov", _stage1_prov)
    monkeypatch.setattr(tip, "_prov", _stage1_prov)
    rows = [tre._row("2026-08-20"), tre._row("2026-08-21"), tre._row("2026-08-22")]
    sources = tre._write_sources(tmp_path, d1_rows=rows)
    # d 與 d1 用同一組列，crossday 才會是 MATCH（與真實的 Stage 1 相同）。
    root = tmp_path / "python"
    stage1 = root / "baselines" / "i074_stage1"
    ev.finalize_evidence(
        evidence_root=stage1, sources=_same_d_rows(sources, rows),
        expected_image_id=OLD_IMG,
        provenance_factory=lambda: _stage1_prov() | {"argv": ["--output-dir", "/tmp/fin"]},
        generated_at="2026-09-18T12:00:00+08:00",
    )
    return root


def _same_d_rows(sources, rows):
    """讓 d 的列與 d1 相同，並重建 d 的 cohort 與 crossday（保持彼此對得起來）。"""
    from ..replay_bundle import crossday as cd

    d = tre._after("2026-09-17T09:00:00+08:00", rows, "/tmp/d")
    d1 = tre._after("2026-09-18T09:00:00+08:00", rows, "/tmp/d1")
    crossday = cd.build_crossday(
        d=d, d1=d1,
        d_sha=sha256_hex(canonical_json_bytes(d)), d1_sha=sha256_hex(canonical_json_bytes(d1)),
        comparator_provenance=_stage1_prov() | {"argv": ["--compare", "--output-dir", "/tmp/cd"]},
        generated_at="2026-09-18T10:00:00+08:00",
    )
    replaced = {
        "d/after_artifact.json.gz": d,
        "d/cohort_manifest.json.gz": tre._cohort(d, "2026-09-17T09:00:00+08:00"),
        "d1/after_artifact.json.gz": d1,
        "d1/cohort_manifest.json.gz": tre._cohort(d1, "2026-09-18T09:00:00+08:00"),
        "crossday/crossday_artifact.json.gz": crossday,
    }
    for rel, payload in replaced.items():
        sources[rel].write_bytes(canonical_json_bytes(payload))
    return sources


def _witness_prov(**overrides):
    prov = _stage1_prov() | {
        "image_digest": NEW_IMG,
        "pip_freeze_sha256": pip_freeze_sha256([tuple(d) for d in DISTS]),
        "python_version": "3.11.20",
        "runner_sha256": "7" * 64,
        "argv": ["--output-dir", "/tmp/witness", "--i074-preflight"],
    }
    return prov | overrides


def _identity(**overrides):
    fields = {"bundle_id": BUNDLE, "expected_image_id": NEW_IMG,
              "created_at": "2026-09-24T09:00:00+08:00"} | overrides
    return build_run_identity(**fields)


def _write_witness(tmp_path, *, rows=None, prov=None, cohort_keys=None):
    """見證趟的 operational 輸出：`<run>/witness/after_artifact.json` ＋ `cohort_manifest.json`。"""
    rows = rows if rows is not None else [tre._row(d) for d in ("2026-08-20", "2026-08-21", "2026-08-22")]
    after = tre._after("2026-09-24T12:00:00+08:00", rows, "/tmp/witness")
    after["provenance"] = prov or _witness_prov()
    cohort = tre._cohort(after, "2026-09-24T12:00:00+08:00")
    cohort["provenance"] = after["provenance"]
    if cohort_keys is not None:
        cohort["keys"] = cohort_keys
    run = tmp_path / "run"
    (run / "witness").mkdir(parents=True)
    (run / "witness" / "after_artifact.json").write_bytes(canonical_json_bytes(after))
    (run / "witness" / "cohort_manifest.json").write_bytes(canonical_json_bytes(cohort))
    return run


def _factory():
    return lambda: _witness_prov(argv=["--envcheck", "--run-dir", "/tmp/run"])


def _publish(python_root, run, **overrides):
    kwargs = {
        "python_root": python_root, "run_dir": run, "stage2_identity": _identity(),
        "provenance_factory": _factory(), "generated_at": "2026-09-24T13:00:00+08:00",
        "distributions": DISTS,
    }
    kwargs.update(overrides)
    return ec.publish_envcheck(**kwargs)


def _envcheck_root(python_root) -> Path:
    return python_root / "baselines" / "i074_stage2" / "envcheck"


def _recover(python_root, **overrides):
    return ec.recover_envcheck(python_root=python_root, provenance=_factory()() | overrides)


# ── 正向：EQUIVALENT ─────────────────────────────────────────────────────────

def test_publish_equivalent(python_root, tmp_path):
    assert _publish(python_root, _write_witness(tmp_path)) == 0
    root = _envcheck_root(python_root)
    assert s2.archive_file_set(root) == set(ec.ENVCHECK_LAYOUT) | {ec.ENVCHECK_MANIFEST_NAME}
    eq = ev.load_canonical_evidence_artifact(root / ec.EQUIVALENCE, ec.ENV_EQUIVALENCE_KIND).parsed
    assert eq["outcome"] == ec.OUTCOME_EQUIVALENT
    assert (eq["rows_compared"], eq["key_mismatch_count"], eq["row_mismatch_count"]) == (3, 0, 0)
    assert eq["provenance_allowed_differences"]["image_digest"] == {"reference": OLD_IMG, "witness": NEW_IMG}
    assert eq["witness_distributions"] == DISTS
    manifest = ev.load_canonical_evidence_artifact(
        root / ec.ENVCHECK_MANIFEST_NAME, ec.ENVCHECK_MANIFEST_KIND).parsed
    assert manifest["terminal_outcome"] == 0 and manifest["expected_image_id"] == NEW_IMG


def test_recover_equivalent_returns_zero(python_root, tmp_path):
    _publish(python_root, _write_witness(tmp_path))
    assert _recover(python_root) == 0


def test_stage2_eligibility_passes_for_equivalent(python_root, tmp_path):
    _publish(python_root, _write_witness(tmp_path))
    result = ec.verify_envcheck_for_stage2(python_root, _identity())
    assert result.outcome == ec.OUTCOME_EQUIVALENT


def test_republish_is_rejected(python_root, tmp_path):
    run = _write_witness(tmp_path)
    _publish(python_root, run)
    with pytest.raises(ArtifactError, match="已存在"):
        _publish(python_root, run)


# ── be／be2：NOT_EQUIVALENT 的形狀（⚠️ 證據照樣發布，回 7） ────────────────

def _rows(*dates, **overrides_by_date):
    rows = []
    for d in dates:
        row = tre._row(d)
        row.update(overrides_by_date.get(d.replace("-", "_"), {}))
        rows.append(row)
    return rows


@pytest.mark.parametrize("shape", ["reference_only", "witness_only", "bytes_differ"])
def test_be2_key_and_row_mismatch_shapes(python_root, tmp_path, shape):
    if shape == "reference_only":
        rows = _rows("2026-08-20", "2026-08-21")
    elif shape == "witness_only":
        rows = _rows("2026-08-20", "2026-08-21", "2026-08-22", "2026-08-23")
    else:
        rows = _rows("2026-08-20", "2026-08-21", "2026-08-22",
                     **{"2026_08_21": {"lifecycle_phase": "CONFIRMED"}})
    assert _publish(python_root, _write_witness(tmp_path, rows=rows)) == EXIT_ENV_NOT_EQUIVALENT
    eq = ev.load_canonical_evidence_artifact(
        _envcheck_root(python_root) / ec.EQUIVALENCE, ec.ENV_EQUIVALENCE_KIND).parsed
    assert eq["outcome"] == ec.OUTCOME_NOT_EQUIVALENT
    if shape == "reference_only":
        assert (eq["reference_key_count"], eq["witness_key_count"], eq["rows_compared"]) == (3, 2, 2)
        assert eq["key_mismatch_sample_keys"] == [
            {"symbol": "2330", "timeframe": "1d", "as_of": "2026-08-22", "side": ec.SIDE_REFERENCE_ONLY}]
        assert eq["row_mismatch_count"] == 0   # ⚠️ 單側 key ⛔ 不計入 row_mismatch
    elif shape == "witness_only":
        assert (eq["reference_key_count"], eq["witness_key_count"], eq["rows_compared"]) == (3, 4, 3)
        assert eq["key_mismatch_sample_keys"][0]["side"] == ec.SIDE_WITNESS_ONLY
        assert eq["row_mismatch_count"] == 0
    else:
        assert eq["key_mismatch_count"] == 0 and eq["rows_compared"] == 3
        assert eq["mismatch_sample_keys"] == [{"symbol": "2330", "timeframe": "1d", "as_of": "2026-08-21"}]
    # ⚠️ recovery 還原 7（⛔ 不得固定回 0）；Stage 2 的使用資格被 E3b 擋下。
    assert _recover(python_root) == EXIT_ENV_NOT_EQUIVALENT
    with pytest.raises(ArtifactError, match="E3b"):
        ec.verify_envcheck_for_stage2(python_root, _identity())


@pytest.mark.parametrize("field,value", [
    ("project_modules_sha256", {"config": "0" * 64}),
    ("runtime_settings", {"db_driver": "sqlite"}),
])
def test_be_required_provenance_difference_is_not_equivalent(python_root, tmp_path, field, value):
    prov = _witness_prov(**{field: value})
    assert _publish(python_root, _write_witness(tmp_path, prov=prov),
                    provenance_factory=lambda: prov) == EXIT_ENV_NOT_EQUIVALENT


def test_be_allowed_differences_do_not_flip_the_outcome(python_root, tmp_path):
    # ⚠️ image／pip freeze／python／runner／argv 都不同——仍是 EQUIVALENT。
    assert _publish(python_root, _write_witness(tmp_path)) == 0


@pytest.mark.parametrize("field,value", [
    ("base_commit", "1" * 40),
    ("tooling_patch_sha256", "2" * 64),
])
def test_e7_invalid_witness_is_rejected_not_published(python_root, tmp_path, field, value):
    # ⚠️ 見證趟⛔ 跑錯版本或套了 patch：那⛔ 不是環境見證，⛔ 不發布任何證據（E7 fail-closed）。
    prov = _witness_prov(**{field: value})
    with pytest.raises(ArtifactError, match="E7"):
        _publish(python_root, _write_witness(tmp_path, prov=prov), provenance_factory=lambda: prov)
    assert not _envcheck_root(python_root).exists()


def test_cohort_difference_is_not_equivalent(python_root, tmp_path):
    rows = _rows("2026-08-20", "2026-08-21", "2026-08-22",
                 **{"2026_08_21": {"lifecycle_phase": "CONTINUATION", "rr_decoupling_candidate": True}})
    run = _write_witness(tmp_path, rows=rows, cohort_keys=[["2330", "1d", "2026-08-21"]])
    assert _publish(python_root, run) == EXIT_ENV_NOT_EQUIVALENT
    eq = ev.load_canonical_evidence_artifact(
        _envcheck_root(python_root) / ec.EQUIVALENCE, ec.ENV_EQUIVALENCE_KIND).parsed
    assert eq["cohort_equal"] is False


def test_bk_sample_is_capped_at_the_shared_limit(python_root, tmp_path):
    assert ec.DIAGNOSTIC_SAMPLE_LIMIT is s2.DIAGNOSTIC_SAMPLE_LIMIT and s2.DIAGNOSTIC_SAMPLE_LIMIT == 20
    extra = [f"2026-07-{d:02d}" for d in range(1, 26)]
    rows = _rows("2026-08-20", "2026-08-21", "2026-08-22", *extra)
    _publish(python_root, _write_witness(tmp_path, rows=rows))
    eq = ev.load_canonical_evidence_artifact(
        _envcheck_root(python_root) / ec.EQUIVALENCE, ec.ENV_EQUIVALENCE_KIND).parsed
    assert eq["key_mismatch_count"] == 25          # ⚠️ 完整計數照樣保留
    assert len(eq["key_mismatch_sample_keys"]) == 20


def _python_root_with(tmp_path, monkeypatch, rows):
    """與 `python_root` fixture 相同，但 Stage 1 D+1 用指定的列（⑧ 補齊：要造出超過 sample 上限的 row 差異）。"""
    monkeypatch.setattr(tre, "_prov", _stage1_prov)
    monkeypatch.setattr(tip, "_prov", _stage1_prov)
    sources = tre._write_sources(tmp_path, d1_rows=rows)
    root = tmp_path / "python"
    ev.finalize_evidence(
        evidence_root=root / "baselines" / "i074_stage1", sources=_same_d_rows(sources, rows),
        expected_image_id=OLD_IMG,
        provenance_factory=lambda: _stage1_prov() | {"argv": ["--output-dir", "/tmp/fin"]},
        generated_at="2026-09-18T12:00:00+08:00",
    )
    return root


def test_bk_row_mismatch_count_is_not_capped(tmp_path, monkeypatch):
    """bk（⑧ 補齊）：25 列 key 相同、bytes 不同 → `row_mismatch_count` ＝ 25（⛔ 不受 N 截斷）、`mismatch_sample_keys` 20 列。"""
    days = [f"2026-07-{d:02d}" for d in range(1, 26)]
    python_root = _python_root_with(tmp_path, monkeypatch, [tre._row(d) for d in days])
    witness = [tre._row(d, event_signal="DIFFERENT") for d in days]
    assert _publish(python_root, _write_witness(tmp_path / "w", rows=witness)) == EXIT_ENV_NOT_EQUIVALENT
    eq = ev.load_canonical_evidence_artifact(
        _envcheck_root(python_root) / ec.EQUIVALENCE, ec.ENV_EQUIVALENCE_KIND).parsed
    assert (eq["key_mismatch_count"], eq["row_mismatch_count"], eq["rows_compared"]) == (0, 25, 25)
    assert len(eq["mismatch_sample_keys"]) == s2.DIAGNOSTIC_SAMPLE_LIMIT == 20
    assert [k["as_of"] for k in eq["mismatch_sample_keys"]] == days[:20]


# ── bd：E1～E7 在 recovery 時各自 fail-closed ────────────────────────────────

def _rewrite_member(root: Path, rel: str, payload: dict) -> None:
    """改寫一個成員，並**同步 manifest 的 metadata**——讓 E1 通過，隔離出要測的那一道。"""
    raw = canonical_json_bytes(payload)
    blob = canonical_gzip_bytes(raw)
    (root / rel).write_bytes(blob)
    manifest_path = root / ec.ENVCHECK_MANIFEST_NAME
    manifest = ev.load_canonical_evidence_artifact(manifest_path, ec.ENVCHECK_MANIFEST_KIND).parsed
    manifest["files"][rel] = {"artifact_sha256": sha256_hex(raw), "stored_sha256": sha256_hex(blob),
                              "stored_bytes": len(blob)}
    manifest_path.write_bytes(canonical_json_bytes(manifest))


def _member(root: Path, rel: str, kind: str) -> dict:
    return ev.load_canonical_evidence_artifact(root / rel, kind).parsed


def _rewrite_manifest(root: Path, **changes) -> None:
    path = root / ec.ENVCHECK_MANIFEST_NAME
    manifest = ev.load_canonical_evidence_artifact(path, ec.ENVCHECK_MANIFEST_KIND).parsed
    manifest.update(changes)
    path.write_bytes(canonical_json_bytes(manifest))


@pytest.fixture
def published(python_root, tmp_path):
    _publish(python_root, _write_witness(tmp_path))
    return python_root, _envcheck_root(python_root)


def test_bd_e1_unknown_file(published):
    python_root, root = published
    (root / "extra.json").write_text("{}")
    with pytest.raises(ArtifactError, match="E1"):
        _recover(python_root)


def test_bd_e1_metadata_mismatch(published):
    python_root, root = published
    eq = _member(root, ec.EQUIVALENCE, ec.ENV_EQUIVALENCE_KIND)
    (root / ec.EQUIVALENCE).write_bytes(canonical_gzip_bytes(canonical_json_bytes(eq | {"generated_at": "2026-09-25T00:00:00+08:00"})))
    with pytest.raises(ArtifactError, match="E1"):
        _recover(python_root)


def test_bd_e2_cohort_not_bound_to_witness(published):
    python_root, root = published
    cohort = _member(root, ec.WITNESS_COHORT, "sr_zone_replay_cohort")
    _rewrite_member(root, ec.WITNESS_COHORT, cohort | {"after_artifact_sha256": "0" * 64})
    with pytest.raises(ArtifactError, match="E2"):
        _recover(python_root)


def test_bd_e3a_stored_value_differs_from_recomputed(published):
    python_root, root = published
    eq = _member(root, ec.EQUIVALENCE, ec.ENV_EQUIVALENCE_KIND)
    eq["rows_compared"] = 2
    eq["reference_key_count"] = 2   # ⚠️ 讓計數不變條件仍成立——只剩「與重算值不符」這一道
    eq["witness_key_count"] = 2
    _rewrite_member(root, ec.EQUIVALENCE, eq)
    with pytest.raises(ArtifactError, match="E3a"):
        _recover(python_root)


def test_bd_e4_distributions_do_not_match_pip_freeze(published):
    python_root, root = published
    eq = _member(root, ec.EQUIVALENCE, ec.ENV_EQUIVALENCE_KIND)
    _rewrite_member(root, ec.EQUIVALENCE, eq | {"witness_distributions": [["numpy", "2.0.0"]]})
    with pytest.raises(ArtifactError, match="E4"):
        _recover(python_root)


def test_bd_e5_image_chain(published):
    python_root, root = published
    _rewrite_member(root, ec.ENVCHECK_IDENTITY, _identity(expected_image_id="sha256:" + "8" * 64))
    with pytest.raises(ArtifactError, match="E5"):
        _recover(python_root)


def test_bd_e6_bundle_chain(published):
    python_root, root = published
    _rewrite_manifest(root, bundle_id="b1_other")
    with pytest.raises(ArtifactError, match="E6"):
        _recover(python_root)


def test_bd_e5_stage2_identity_must_equal_the_archived_one(published):
    python_root, _root = published
    # ⚠️ 只有 created_at 不同也⛔ 不行（信任錨第 10 道／F2-a 的同一條理由）。
    with pytest.raises(ArtifactError, match="不完全相同"):
        ec.verify_envcheck_for_stage2(python_root, _identity(created_at="2026-09-25T09:00:00+08:00"))


def test_bd_stage1_evidence_ref_must_match_the_anchor(published):
    python_root, root = published
    path = root / ec.ENVCHECK_MANIFEST_NAME
    manifest = ev.load_canonical_evidence_artifact(path, ec.ENVCHECK_MANIFEST_KIND).parsed
    manifest["stage1_evidence"]["manifest_sha256"] = "0" * 64
    path.write_bytes(canonical_json_bytes(manifest))
    with pytest.raises(ArtifactError, match="stage1_evidence"):
        _recover(python_root)


# ── bd2／bd3：NOT_EQUIVALENT 的 archive 合法；outcome 三者要一致 ───────────

def test_bd3_manifest_terminal_outcome_disagrees(published):
    python_root, root = published
    _rewrite_manifest(root, terminal_outcome=EXIT_ENV_NOT_EQUIVALENT)
    with pytest.raises(ArtifactError, match="三者不一致"):
        _recover(python_root)


def test_bd3_stored_outcome_disagrees_with_its_own_fields(published):
    python_root, root = published
    eq = _member(root, ec.EQUIVALENCE, ec.ENV_EQUIVALENCE_KIND)
    _rewrite_member(root, ec.EQUIVALENCE, eq | {"outcome": ec.OUTCOME_NOT_EQUIVALENT})
    with pytest.raises(ArtifactError, match="outcome"):
        _recover(python_root)


# ── recovery 的執行身分與 durability ────────────────────────────────────────

@pytest.mark.parametrize("field", ev.RECOVERY_IDENTITY_FIELDS)
def test_recovery_rejects_execution_identity_drift(published, field):
    python_root, _root = published
    drifted = {
        "image_digest": "sha256:" + "5" * 64, "base_commit": "5" * 40,
        "tooling_patch_sha256": "5" * 64, "runner_sha256": "5" * 64,
        "source_root": "/elsewhere", "runtime_settings": {"db_driver": "mysql"},
    }[field]
    with pytest.raises(ArtifactError, match="執行身分"):
        _recover(python_root, **{field: drifted})


def test_publish_parent_fsync_failure_keeps_archive(python_root, tmp_path, monkeypatch):
    real = s2.fsync_dir

    def failing(path):
        if Path(path) == _envcheck_root(python_root).parent:
            raise OSError("boom")
        return real(path)

    monkeypatch.setattr(s2, "fsync_dir", failing)
    with pytest.raises(DurabilityUnconfirmed):
        _publish(python_root, _write_witness(tmp_path))
    assert _envcheck_root(python_root).is_dir()   # ⚠️ commit point 之後⛔ 不刪除
    monkeypatch.setattr(s2, "fsync_dir", real)
    assert _recover(python_root) == 0


def test_failure_before_rename_leaves_no_archive_and_no_staging(python_root, tmp_path):
    run = _write_witness(tmp_path)
    (run / "witness" / "cohort_manifest.json").write_bytes(b"{}")
    with pytest.raises(ArtifactError):
        _publish(python_root, run)
    parent = _envcheck_root(python_root).parent
    assert not _envcheck_root(python_root).exists()
    assert not [p for p in parent.iterdir() if p.name.startswith(".envcheck.staging")]


# ── bf：全量 artifact 一律串流、每份只讀一次 ──────────────────────────────────

def test_bf_full_artifacts_are_streamed_once(python_root, tmp_path, monkeypatch):
    real_load = ev.load_canonical_evidence_artifact
    calls: list[str] = []
    real_stream = st.stream_after_artifact

    def guarded_load(path, kind):
        # ⛔ 全量的 after artifact⛔ 不得走整份載入的路徑。
        assert kind != AFTER_KIND, f"after artifact 被整份載入：{path}"
        return real_load(path, kind)

    def counting_stream(path, **kwargs):
        calls.append(str(path))
        return real_stream(path, **kwargs)

    for module in (ec, s2):
        monkeypatch.setattr(module, "load_canonical_evidence_artifact", guarded_load)
        monkeypatch.setattr(module, "stream_after_artifact", counting_stream)

    _publish(python_root, _write_witness(tmp_path))
    assert len(calls) == len(set(calls)) == 3     # D+1、operational after'、staging 內的副本
    calls.clear()
    _recover(python_root)
    assert len(calls) == len(set(calls)) == 2     # D+1、封存的 after'


# ── bd4：envcheck manifest 的封閉 schema ───────────────────────────────────

def _valid_manifest(published_root):
    return ev.load_canonical_evidence_artifact(
        published_root / ec.ENVCHECK_MANIFEST_NAME, ec.ENVCHECK_MANIFEST_KIND).parsed


@pytest.mark.parametrize("mutate", [
    lambda m: m | {"extra": 1},
    lambda m: {k: v for k, v in m.items() if k != "stage1_evidence"},
    lambda m: m | {"kind": "sr_zone_evidence_manifest"},
    lambda m: m | {"files": m["files"] | {"extra.json.gz": m["files"][ec.EQUIVALENCE]}},
    lambda m: m | {"files": {k: v for k, v in m["files"].items() if k != ec.EQUIVALENCE}},
    lambda m: m | {"files": m["files"] | {ec.EQUIVALENCE: {"stored_sha256": "0" * 64, "stored_bytes": 1}}},
    lambda m: m | {"finalizer_provenance": m["finalizer_provenance"] | {"image_digest": OLD_IMG}},
    lambda m: m | {"terminal_outcome": True},
    lambda m: m | {"terminal_outcome": 5},
    lambda m: m | {"generated_at": "2026-09-24"},
], ids=["extra_field", "missing_field", "wrong_kind", "extra_file", "missing_file", "raw_blob_entry",
        "finalizer_image", "outcome_bool", "outcome_unknown", "naive_generated_at"])
def test_bd4_manifest_schema(published, mutate):
    _python_root, root = published
    manifest = _valid_manifest(root)
    ec.validate_envcheck_manifest(manifest)
    with pytest.raises(ArtifactError):
        ec.validate_envcheck_manifest(mutate(manifest))


# ── equivalence artifact 的封閉 schema ─────────────────────────────────────

@pytest.mark.parametrize("mutate", [
    lambda e: e | {"key_mismatch_count": 1},                       # 計數不變條件
    lambda e: e | {"rows_compared": 4},                            # 交集大於單側
    lambda e: e | {"row_mismatch_count": 1},                       # sample 長度不符
    lambda e: e | {"cohort_equal": 1},                             # 非嚴格 boolean
    lambda e: e | {"provenance_required_equal": {"base_commit": True}},
    lambda e: e | {"witness_distributions": [["b", "1"], ["a", "1"]]},
    lambda e: e | {"outcome": "MAYBE"},
    lambda e: e | {"kind": "other"},
], ids=["key_count_invariant", "compared_exceeds_side", "sample_length", "cohort_equal_int",
        "required_fields", "dists_unsorted", "outcome_enum", "kind"])
def test_equivalence_schema(published, mutate):
    _python_root, root = published
    eq = _member(root, ec.EQUIVALENCE, ec.ENV_EQUIVALENCE_KIND)
    ec.validate_equivalence_artifact(eq)
    with pytest.raises(ArtifactError):
        ec.validate_equivalence_artifact(mutate(eq))


def test_equivalence_sample_side_enum_and_order():
    base = {"symbol": "2330", "timeframe": "1d"}
    with pytest.raises(ArtifactError, match="side"):
        ec._validate_key_sample([base | {"as_of": "a", "side": "BOTH"}], name="k",
                                fields=ec._KEY_SAMPLE_FIELDS, count=1)
    with pytest.raises(ArtifactError, match="排序"):
        ec._validate_key_sample([base | {"as_of": "b"}, base | {"as_of": "a"}], name="k",
                                fields=ec._ROW_SAMPLE_FIELDS, count=2)


# ── Stage 1 信任錨 ──────────────────────────────────────────────────────────

def test_anchor_loads_the_fixture(python_root):
    anchor = s2.load_stage1_anchor(python_root)
    assert anchor.bundle_id == BUNDLE and anchor.expected_image_id == OLD_IMG
    assert len(anchor.after_keys) == 3 and anchor.after_candidate_keys == anchor.cohort_keys == []


def test_anchor_rejects_tampered_member(python_root):
    after = python_root / "baselines" / "i074_stage1" / s2.STAGE1_ANCHOR_AFTER
    payload = gzip.decompress(after.read_bytes())
    after.write_bytes(canonical_gzip_bytes(payload.replace(b"TESTING", b"CONFIRMED", 1)))
    with pytest.raises(ArtifactError, match="一起保存"):
        s2.load_stage1_anchor(python_root)


def test_anchor_missing_stage1_evidence(tmp_path):
    with pytest.raises(ArtifactError, match="一起保存"):
        s2.load_stage1_anchor(tmp_path / "python")


def test_anchor_graph_rules(python_root):
    anchor = s2.load_stage1_anchor(python_root)
    identity = _identity()
    s2.validate_stage1_anchor_graph(anchor, stage2_identity=identity, envcheck_identity=identity,
                                    expected_bundle_id=BUNDLE)
    with pytest.raises(ArtifactError, match="bundle_id"):
        s2.validate_stage1_anchor_graph(anchor, stage2_identity=identity, envcheck_identity=identity,
                                        expected_bundle_id="b1_other")
    with pytest.raises(ArtifactError, match="不完全相同"):
        s2.validate_stage1_anchor_graph(
            anchor, stage2_identity=identity,
            envcheck_identity=_identity(created_at="2026-09-25T09:00:00+08:00"),
            expected_bundle_id=BUNDLE)


def test_stage1_evidence_ref_schema(python_root):
    anchor = s2.load_stage1_anchor(python_root)
    ref = s2.build_stage1_evidence_ref(anchor)
    s2.validate_stage1_evidence_ref(ref, anchor=anchor)
    for bad in (
        ref | {"manifest_path": "python/baselines/other/evidence_manifest.json"},
        ref | {"members": {k: v for k, v in ref["members"].items() if k != s2.STAGE1_ANCHOR_IDENTITY}},
        ref | {"extra": 1},
    ):
        with pytest.raises(ArtifactError):
            s2.validate_stage1_evidence_ref(bad)


def test_w_members_with_an_extra_key_are_rejected(python_root):
    """w（⑧ 補齊）：`members` 的 key 集合「增」也要拒絕（⛔ 不只測「減」）。"""
    anchor = s2.load_stage1_anchor(python_root)
    ref = s2.build_stage1_evidence_ref(anchor)
    extra = ref | {"members": ref["members"] | {"d/after_artifact.json.gz": ref["members"][s2.STAGE1_ANCHOR_AFTER]}}
    for kwargs in ({}, {"anchor": anchor}):
        with pytest.raises(ArtifactError, match="members"):
            s2.validate_stage1_evidence_ref(extra, **kwargs)


@pytest.mark.parametrize("rel", [
    "/abs/python/x", "python/../x", "python/./x", "other/x", "python",
])
def test_resolve_repo_path_rejects_unsafe(tmp_path, rel):
    with pytest.raises(ArtifactError):
        s2.resolve_repo_path(tmp_path / "python", rel)


def test_resolve_repo_path_rejects_symlink_component(tmp_path):
    root = tmp_path / "python"
    (tmp_path / "elsewhere").mkdir()
    root.mkdir()
    (root / "baselines").symlink_to(tmp_path / "elsewhere")
    with pytest.raises(ArtifactError, match="symlink"):
        s2.resolve_repo_path(root, "python/baselines/x.json")


def test_real_stage1_evidence_anchors():
    """⚠️ 對**真正封存的** Stage 1 evidence 跑串流信任錨（測試容器把 python/ 掛在 /app）。"""
    python_root = Path(__file__).resolve().parents[4]
    if not (python_root / "baselines" / "i074_stage1" / "evidence_manifest.json").is_file():
        pytest.skip("找不到封存的 Stage 1 evidence")
    anchor = s2.load_stage1_anchor(python_root)
    assert (len(anchor.after_keys), len(anchor.after_candidate_keys), len(anchor.cohort_rows)) == (13417, 156, 156)
    assert anchor.members[s2.STAGE1_ANCHOR_AFTER]["artifact_sha256"] == \
        "33a6b1666487abcfc88353020afbd1e35a8cbab71b40c9ef17ea1194a8cc8b8c"
    s2.validate_stage1_anchor_graph(anchor, stage2_identity=anchor.identity,
                                    envcheck_identity=anchor.identity, expected_bundle_id=anchor.bundle_id)


# ── stage-scoped identity ───────────────────────────────────────────────────

def test_identity_path_is_derived_per_stage(tmp_path, monkeypatch):
    monkeypatch.setenv("XDG_DATA_HOME", str(tmp_path))
    assert default_run_identity_path() == tmp_path / "stock_trading" / "i074_stage1" / "run_identity.json"
    assert default_run_identity_path(2) == tmp_path / "stock_trading" / "i074_stage2" / "run_identity.json"
    for bad in (3, 0, True, "2"):
        with pytest.raises(ArtifactError):
            default_run_identity_path(bad)


# ── CLI（七之四：envcheck／recover-envcheck） ───────────────────────────────

_INJECTED = ["--python-root", "/p", "--image-digest", NEW_IMG, "--base-commit", "e" * 40,
             "--tooling-patch-sha256", s2.EMPTY_SHA256, "--source-root", "/app",
             "--runner-sha256", "7" * 64]


@pytest.mark.parametrize("argv,match", [
    (["--envcheck", "--recover-envcheck"] + _INJECTED, "恰好給一個"),
    (_INJECTED, "恰好給一個"),
    (["--envcheck", "--run-dir", "/r"] + _INJECTED, "需要 --run-dir 與 --run-identity"),
    (["--recover-envcheck", "--run-dir", "/r"] + _INJECTED, "⛔ 不接受"),
    (["--recover-envcheck", "--run-identity", "/i"] + _INJECTED, "⛔ 不接受"),
    (["--recover-envcheck", "--image-digest", NEW_IMG] + _INJECTED, "只能由官方腳本注入"),
])
def test_cli_matrix_usage_errors(argv, match):
    with pytest.raises(ec.EnvcheckUsageError, match=match):
        ec.run_envcheck(argv)


def test_cli_rejects_identity_image_mismatch(tmp_path):
    from ..replay_bundle.run_identity import publish_run_identity

    identity_path = tmp_path / "run_identity.json"
    publish_run_identity(identity_path, _identity(expected_image_id="sha256:" + "8" * 64))
    argv = ["--envcheck", "--run-dir", str(tmp_path), "--run-identity", str(identity_path)] + _INJECTED
    with pytest.raises(ec.EnvcheckUsageError, match="不符"):
        ec.run_envcheck(argv)


def test_main_maps_outcomes_to_exit_codes(monkeypatch):
    for terminal in (0, EXIT_ENV_NOT_EQUIVALENT):
        monkeypatch.setattr(ec, "run_envcheck", lambda argv, t=terminal: {
            "mode": "recover_envcheck", "outcome": "x", "terminal_outcome": t})
        assert ec.main([]) == terminal

    def durability(argv):
        raise DurabilityUnconfirmed("x", published=True)

    monkeypatch.setattr(ec, "run_envcheck", durability)
    assert ec.main([]) == 3

    def broken(argv):
        raise ArtifactError("x")

    monkeypatch.setattr(ec, "run_envcheck", broken)
    assert ec.main([]) == 1
