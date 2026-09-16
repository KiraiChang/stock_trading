"""**replay** evidence finalizer：整包原子發布、全圖驗證、durability 與 recovery（I-074 Stage 1）。

⚠️ ⛔ 這一檔與 `test_evidence.py` **無關**——後者測的是 `sr_scoring/evidence.py`（SHAP
additivity／降級／top-N），⛔ 不得混在一起。

對應計畫書十／十一與測試矩陣 10／11-B／11-C／11-F。
"""
from __future__ import annotations

import pytest

from ..replay_bundle import crossday as cd
from ..replay_bundle import evidence as ev
from ..replay_bundle import i074_preflight as pf
from ..replay_bundle import probe as pb
from ..replay_bundle.artifacts import ArtifactError
from ..replay_bundle.canonical import canonical_json_bytes, sha256_hex
from ..replay_bundle.publish import DurabilityUnconfirmed
from .test_i074_preflight import _computation, _measurement, _probe_rows, _prov

IMG = "sha256:" + "a" * 64
BUNDLE = pf.I074_BUNDLE_ID


def _row(as_of, **overrides):
    structure, action = "SUPPORT_RECLAIM_CONFIRMED", "HOLD"
    row = {
        "symbol": "2330", "timeframe": "1d", "as_of": as_of,
        "lifecycle_phase": "TESTING", "clear_zone_breakout": False,
        "continuation_price_evidence_met": False, "setup_rr_qualified": False,
        "rr_decoupling_candidate": False, "event_signal": "CLOSE_RECLAIM",
        "structure_state": structure, "action_state": action,
        "position_action_condition": {"state": action, "structure_state": structure},
        "position_action": action,
    }
    row.update(overrides)
    return row


def _after(generated_at, rows, argv_out):
    return {
        "schema_version": 1, "kind": "sr_zone_replay_after", "bundle_id": BUNDLE,
        "timeframe": "1d", "replay_scope": "all_candidates", "run_id": "i074",
        "pipeline_version": "sr_zone_decision_replay_p1", "generated_at": generated_at,
        "provenance": _prov() | {"argv": ["--output-dir", argv_out]}, "rows": rows,
    }


def _cohort(after, generated_at):
    return {
        "schema_version": 1, "kind": "sr_zone_replay_cohort", "bundle_id": BUNDLE,
        "after_artifact_sha256": sha256_hex(canonical_json_bytes(after)),
        "generated_at": generated_at, "provenance": after["provenance"],
        "keys": [],
    }


def _write_sources(tmp_path, *, d1_rows=None):
    """產出 9 份**合法且彼此對得起來**的 operational artifact。"""
    from ..replay_bundle.run_identity import build_run_identity

    d_rows = [_row("2026-08-20"), _row("2026-08-21")]
    d = _after("2026-09-14T09:00:00+08:00", d_rows, "/tmp/d")
    d1 = _after("2026-09-15T09:00:00+08:00", d1_rows or d_rows, "/tmp/d1")
    comparator = _prov() | {"argv": ["--compare", "--output-dir", "/tmp/cd"]}
    crossday = cd.build_crossday(
        d=d, d1=d1,
        d_sha=sha256_hex(canonical_json_bytes(d)), d1_sha=sha256_hex(canonical_json_bytes(d1)),
        comparator_provenance=comparator, generated_at="2026-09-15T10:00:00+08:00",
    )
    computation = _computation()
    measurement = _measurement()
    completion = {
        "schema_version": 1, "kind": pb.PROBE_COMPLETION_KIND, "bundle_id": BUNDLE,
        "generated_at": "2026-09-14T00:00:00+08:00", "provenance": _prov(),
        "completed": True,
        "computation_artifact_sha256": sha256_hex(canonical_json_bytes(computation)),
        "measurement_artifact_sha256": sha256_hex(canonical_json_bytes(measurement)),
    }
    identity = build_run_identity(
        bundle_id=BUNDLE, expected_image_id=IMG, created_at="2026-09-14T08:00:00+08:00"
    )
    payloads = {
        "d/after_artifact.json.gz": d,
        "d/cohort_manifest.json.gz": _cohort(d, "2026-09-14T09:00:00+08:00"),
        "d1/after_artifact.json.gz": d1,
        "d1/cohort_manifest.json.gz": _cohort(d1, "2026-09-15T09:00:00+08:00"),
        "crossday/crossday_artifact.json.gz": crossday,
        "probe/capacity_probe_computation.json.gz": computation,
        "probe/capacity_probe_measurement.json.gz": measurement,
        "probe/capacity_probe.json.gz": completion,
        "identity/run_identity.json.gz": identity,
    }
    src = tmp_path / "operational"
    src.mkdir()
    sources = {}
    for rel, payload in payloads.items():
        path = src / rel.replace("/", "_").replace(".gz", "")
        path.write_bytes(canonical_json_bytes(payload))
        sources[rel] = path
    return sources


def _finalize(tmp_path, sources, **overrides):
    kwargs = {
        "evidence_root": tmp_path / "i074_stage1",
        "sources": sources,
        "expected_image_id": IMG,
        "provenance_factory": lambda: _prov() | {"argv": ["--output-dir", "/tmp/fin"]},
        "generated_at": "2026-09-15T11:00:00+08:00",
    }
    kwargs.update(overrides)
    return ev.finalize_evidence(**kwargs)


# ── 正向 ────────────────────────────────────────────────────────────────────

def test_finalize_publishes_ten_files(tmp_path):
    sources = _write_sources(tmp_path)
    outcome = _finalize(tmp_path, sources)
    assert outcome == cd.OUTCOME_MATCH

    root = tmp_path / "i074_stage1"
    files = sorted(str(p.relative_to(root)) for p in root.rglob("*") if p.is_file())
    assert files == sorted(list(ev.ARCHIVE_PATHS) + [ev.EVIDENCE_MANIFEST_NAME])


def test_manifest_metadata_matches_actual_archives(tmp_path):
    import json

    sources = _write_sources(tmp_path)
    _finalize(tmp_path, sources)
    root = tmp_path / "i074_stage1"
    manifest = json.loads((root / ev.EVIDENCE_MANIFEST_NAME).read_text(encoding="utf-8"))
    ev.validate_evidence_manifest(manifest)
    for rel, entry in manifest["files"].items():
        raw = (root / rel).read_bytes()
        assert entry["stored_sha256"] == sha256_hex(raw)
        assert entry["stored_bytes"] == len(raw)


# ── 原子性 ──────────────────────────────────────────────────────────────────

def test_failure_before_rename_leaves_no_root_and_no_staging(tmp_path):
    """⚠️ ⛔ 不得留下**半包證據**——正式路徑由「不存在」直接變成「完整」。"""
    sources = _write_sources(tmp_path)
    # 讓 crossday 的 bundle_id 對不上 → 全圖驗證在階段 A 失敗
    broken = dict(sources)
    bad = tmp_path / "bad_identity.json"
    from ..replay_bundle.run_identity import build_run_identity

    bad.write_bytes(canonical_json_bytes(build_run_identity(
        bundle_id="OTHER", expected_image_id=IMG, created_at="2026-09-14T08:00:00+08:00")))
    broken["identity/run_identity.json.gz"] = bad

    root = tmp_path / "i074_stage1"
    with pytest.raises(ArtifactError):
        _finalize(tmp_path, broken)
    assert not root.exists(), "⛔ 正式 root 不得存在"
    assert not list(tmp_path.glob(".i074_stage1.staging-*")), "⛔ staging 必須已清除"


def test_republish_is_rejected(tmp_path):
    sources = _write_sources(tmp_path)
    _finalize(tmp_path, sources)
    with pytest.raises(ArtifactError, match="已存在"):
        _finalize(tmp_path, sources)


def test_parent_fsync_failure_keeps_root(tmp_path, monkeypatch):
    """⚠️ commit point 之後：root **已存在且有效** → ⛔ 不刪除，回報 durability 未確認。"""
    sources = _write_sources(tmp_path)
    real = ev.fsync_dir
    root = tmp_path / "i074_stage1"

    def boom(path):
        if path == root.parent:
            raise OSError("injected")
        return real(path)

    monkeypatch.setattr(ev, "fsync_dir", boom)
    with pytest.raises(DurabilityUnconfirmed) as exc:
        _finalize(tmp_path, sources)
    assert exc.value.published is True
    assert root.is_dir(), "⛔ 不得刪除已 rename 成功的 evidence"


# ── recovery ────────────────────────────────────────────────────────────────

def _recover(tmp_path):
    return ev.recover_durability(
        evidence_root=tmp_path / "i074_stage1",
        provenance=_prov() | {"argv": ["--output-dir", "/tmp/fin"]},
    )


def test_recovery_restores_match_outcome(tmp_path):
    sources = _write_sources(tmp_path)
    _finalize(tmp_path, sources)
    assert _recover(tmp_path) == cd.OUTCOME_MATCH


def test_recovery_restores_mismatch_outcome(tmp_path):
    """⚠️ **⛔ 不得固定回成功**：否則 mismatch 的 5 會被 recovery 吞成 0。"""
    sources = _write_sources(tmp_path, d1_rows=[_row("2026-08-20", lifecycle_phase="CONFIRMED"),
                                                 _row("2026-08-21")])
    _finalize(tmp_path, sources)
    assert _recover(tmp_path) == cd.OUTCOME_ROW


def test_recovery_needs_no_external_identity(tmp_path):
    """⚠️ **證據自足**：repo 外的協調檔不存在時，recovery 仍能完成。"""
    sources = _write_sources(tmp_path)
    _finalize(tmp_path, sources)
    # ⛔ recovery 只讀 evidence 內的 archived copy——這裡完全沒有外部 identity。
    assert _recover(tmp_path) == cd.OUTCOME_MATCH


def test_recovery_rejects_tampered_manifest_metadata(tmp_path):
    """⚠️ recovery **完全依賴既有 root**——只驗型別的話，索引值錯誤的 manifest 照樣通過。"""
    import json

    sources = _write_sources(tmp_path)
    _finalize(tmp_path, sources)
    root = tmp_path / "i074_stage1"
    manifest = json.loads((root / ev.EVIDENCE_MANIFEST_NAME).read_text(encoding="utf-8"))
    rel = ev.ARCHIVE_PATHS[0]
    manifest["files"][rel]["stored_bytes"] += 1
    (root / ev.EVIDENCE_MANIFEST_NAME).write_bytes(canonical_json_bytes(manifest))
    with pytest.raises(ArtifactError, match="metadata 與實際檔案不符"):
        _recover(tmp_path)


def test_recovery_rejects_unknown_evidence(tmp_path):
    sources = _write_sources(tmp_path)
    _finalize(tmp_path, sources)
    (tmp_path / "i074_stage1" / "surprise.json").write_text("{}", encoding="utf-8")
    with pytest.raises(ArtifactError, match="檔案集合不符"):
        _recover(tmp_path)


@pytest.mark.parametrize("field", ev.RECOVERY_IDENTITY_FIELDS)
def test_recovery_rejects_execution_identity_drift(tmp_path, field, monkeypatch):
    """⚠️ **改的是本次 recovery 這一側**——archived evidence 一個位元都不動。

    ⛔ 改 archived 端會先被 manifest／image 一致性守門攔下，就證明不了這個新分支生效。
    """
    sources = _write_sources(tmp_path)
    _finalize(tmp_path, sources)
    drifted = _prov() | {"argv": ["--output-dir", "/tmp/fin"]}
    drifted[field] = (
        {"db_driver": "sqlite"} if field == "runtime_settings"
        else ("sha256:" + "9" * 64 if field == "image_digest"
              else ("9" * 40 if field == "base_commit"
                    else ("/other" if field == "source_root" else "9" * 64)))
    )
    calls = []
    real = ev.fsync_dir
    monkeypatch.setattr(ev, "fsync_dir", lambda p: calls.append(p) or real(p))
    with pytest.raises(ArtifactError, match="執行身分"):
        ev.recover_durability(evidence_root=tmp_path / "i074_stage1", provenance=drifted)
    assert not calls, "⛔ 必須在 fsync 之前中止"


# ── review 修正後補的測試 ────────────────────────────────────────────────────

def test_main_maps_recovery_outcome_to_exit_code(monkeypatch):
    """⚠️ **高風險修正**：recovery ⛔ 不得固定回 0。

    鏈路「crossday mismatch（5）→ parent fsync 失敗（3 蓋掉 5）→ recovery 回 0」
    會讓 **mismatch 的 5 永遠消失**，機器端會把它讀成「整組 Stage 1 成功匹配」——
    那正好把這次驗證要回答的問題答反。

    ⚠️ 這裡直接測 `main()` 的**仲裁邏輯**：端到端跑的話，CLI 會用真實環境建 provenance，
    必然與 fixture 的假值不符（那是**正確**行為），反而測不到這一段。
    """
    from ..replay_bundle.publish import EXIT_CROSSDAY_MISMATCH

    def _stub(outcome, mode="recover_durability"):
        return lambda argv: {"mode": mode, "outcome": outcome, "evidence_root": "/x"}

    monkeypatch.setattr(ev, "run_evidence", _stub(cd.OUTCOME_ROW))
    assert ev.main([]) == EXIT_CROSSDAY_MISMATCH, "⛔ 已歸檔的 mismatch 不得被 recovery 吞成 0"

    monkeypatch.setattr(ev, "run_evidence", _stub(cd.OUTCOME_PROVENANCE))
    assert ev.main([]) == EXIT_CROSSDAY_MISMATCH

    monkeypatch.setattr(ev, "run_evidence", _stub(cd.OUTCOME_MATCH))
    assert ev.main([]) == 0

    # ⚠️ normal finalization 仍回 0——那時 crossday 的 rc 還在 orchestrator 手上，
    # ⛔ 讓 finalizer 也回 5 會被 orchestrator 判成「finalizer 失敗」。
    monkeypatch.setattr(ev, "run_evidence", _stub(cd.OUTCOME_ROW, mode="finalize"))
    assert ev.main([]) == 0


def test_finalize_rejects_external_identity_differing_from_archived(tmp_path):
    """⚠️ **高風險修正**：repo 外的協調檔與要歸檔的那份必須逐欄相同。

    ⛔ 少了這道，host 驗的是外部 identity A、Python 只信 archived 的 B，兩者從沒比過。
    """
    from ..replay_bundle.canonical import canonical_json_bytes
    from ..replay_bundle.run_identity import build_run_identity

    sources = _write_sources(tmp_path)
    external = tmp_path / "external_identity.json"
    external.write_bytes(canonical_json_bytes(build_run_identity(
        bundle_id=BUNDLE, expected_image_id=IMG,
        created_at="1999-01-01T00:00:00+08:00",   # ⚠️ 只差 created_at 也不行
    )))
    argv = [
        "--evidence-root", str(tmp_path / "i074_stage1"),
        "--run-identity", str(external),
        "--image-digest", IMG, "--base-commit", "e" * 40,
        "--tooling-patch-sha256", "f" * 64, "--source-root", "/app",
        "--runner-sha256", "d" * 64,
    ]
    for rel, path in sources.items():
        argv += ["--source", f"{rel}={path}"]
    assert ev.main(argv) == 1


def test_graph_rejects_cohort_with_wrong_keys(tmp_path):
    """⚠️ **高風險修正**：cohort 要走**完整** validator，⛔ 不只比 SHA。"""
    import json

    from ..replay_bundle.canonical import canonical_json_bytes, sha256_hex

    sources = _write_sources(tmp_path)
    cohort_path = sources["d/cohort_manifest.json.gz"]
    cohort = json.loads(cohort_path.read_text(encoding="utf-8"))
    # ⚠️ keys 多了一筆不存在的候選——SHA 仍指向正確的 after，⛔ 只比 SHA 抓不到。
    cohort["keys"] = [["2330", "1d", "2026-08-20"]]
    cohort_path.write_bytes(canonical_json_bytes(cohort))
    # after SHA 沒動，所以舊的「只比 SHA」會通過。
    with pytest.raises(ArtifactError):
        _finalize(tmp_path, sources)


def test_crossday_durable_validator_rejects_same_day(tmp_path):
    """⚠️ **高風險修正**：durable validator 要自己驗「有效跨日」。

    ⛔ finalizer／recovery 只呼叫 `validate_crossday()`——不能依賴「producer 當初呼叫過
    `assert_valid_crossday_inputs()`」，否則重新封裝一套內部自洽的來源就能放行同一天。
    """
    from ..replay_bundle.canonical import canonical_json_bytes, sha256_hex

    d = _after("2026-09-14T09:00:00+08:00", [_row("2026-08-20")], "/tmp/d")
    d1 = _after("2026-09-14T18:00:00+08:00", [_row("2026-08-20")], "/tmp/d1")  # ⚠️ 同一天
    comparator = _prov() | {"argv": ["--compare", "--output-dir", "/tmp/cd"]}
    artifact = cd.build_crossday(
        d=d, d1=d1,
        d_sha=sha256_hex(canonical_json_bytes(d)), d1_sha=sha256_hex(canonical_json_bytes(d1)),
        comparator_provenance=comparator, generated_at="2026-09-15T10:00:00+08:00",
    )
    with pytest.raises(ArtifactError, match="不是相鄰兩天"):
        cd.validate_crossday(
            artifact, d=d, d1=d1,
            d_sha=sha256_hex(canonical_json_bytes(d)),
            d1_sha=sha256_hex(canonical_json_bytes(d1)),
            comparator_provenance=comparator,
        )


def test_manifest_bundle_id_must_match_archived_identity(tmp_path):
    """⚠️ 單獨竄改 canonical manifest 的 `bundle_id` ⛔ 不得通過 recovery。"""
    import json

    from ..replay_bundle.canonical import canonical_json_bytes

    sources = _write_sources(tmp_path)
    _finalize(tmp_path, sources)
    root = tmp_path / "i074_stage1"
    manifest = json.loads((root / ev.EVIDENCE_MANIFEST_NAME).read_text(encoding="utf-8"))
    manifest["bundle_id"] = "b1_OTHER"
    (root / ev.EVIDENCE_MANIFEST_NAME).write_bytes(canonical_json_bytes(manifest))
    with pytest.raises(ArtifactError, match="archived identity"):
        _recover(tmp_path)


# ── 第三輪 review 的 regression ──────────────────────────────────────────────

def test_comparator_provenance_image_must_match(tmp_path):
    """⚠️ crossday 用的是 `comparator_provenance`——⛔ 通用的 `.get("provenance")`
    推測會**整份漏掉它**，comparator 就能跑在另一個 image 上而沒有東西報錯。"""
    import json

    from ..replay_bundle.canonical import canonical_json_bytes

    sources = _write_sources(tmp_path)
    path = sources["crossday/crossday_artifact.json.gz"]
    crossday = json.loads(path.read_text(encoding="utf-8"))
    crossday["comparator_provenance"]["image_digest"] = "sha256:" + "9" * 64
    path.write_bytes(canonical_json_bytes(crossday))
    with pytest.raises(ArtifactError, match="comparator_provenance.image_digest"):
        _finalize(tmp_path, sources)


def test_manifest_finalizer_provenance_image_must_match():
    """⚠️ manifest 的 `finalizer_provenance` ⛔ 不在那 9 份 archive 裡——
    ⛔ 少了 manifest validator 這一行，finalizer 就能跑在另一個 image 上。"""
    manifest = ev.build_evidence_manifest(
        bundle_id=BUNDLE,
        expected_image_id=IMG,
        files={rel: {"artifact_sha256": "a" * 64, "stored_sha256": "b" * 64, "stored_bytes": 1}
               for rel in ev.ARCHIVE_PATHS},
        generated_at="2026-09-15T11:00:00+08:00",
        finalizer_provenance=_prov() | {"image_digest": "sha256:" + "9" * 64,
                                        "argv": ["--output-dir", "/x"]},
    )
    with pytest.raises(ArtifactError, match="finalizer_provenance.image_digest"):
        ev.validate_evidence_manifest(manifest)


def test_archived_identity_is_loaded_exactly_once(tmp_path, monkeypatch):
    """⚠️ **⛔ 不得「先比對、再重讀」**：兩次讀取之間來源若被替換，實際封存的就不是
    先前與外部 identity 比過的那一份。

    ⚠️ **必須走 `run_evidence()`**：舊實作的多餘那次讀取發生在**那條路徑**上
    （先 `load_run_identity(sources[...])` 比對，再進 `finalize_evidence()` 由
    `_load_all()` 重讀）。⛔ 直接呼叫 `finalize_evidence()` 會完全繞過第一段，
    新舊實作都只會計數一次——那是**假綠燈**。
    """
    from ..replay_bundle import provenance as provenance_mod

    sources = _write_sources(tmp_path)
    target = sources["identity/run_identity.json.gz"]
    # ⚠️ external 是**另一個內容相同**的檔案——所以計數只會落在 archived 那一份上。
    external = tmp_path / "external_identity.json"
    external.write_bytes(target.read_bytes())

    loads: list[str] = []
    real_evidence_load = ev.load_canonical_evidence_artifact
    real_identity_load = ev.load_run_identity

    def counting_evidence_load(path, kind):
        if str(path) == str(target):
            loads.append("evidence_load")
        return real_evidence_load(path, kind)

    def counting_identity_load(path):
        if str(path) == str(target):
            loads.append("run_identity_load")
        return real_identity_load(path)

    monkeypatch.setattr(ev, "load_canonical_evidence_artifact", counting_evidence_load)
    monkeypatch.setattr(ev, "load_run_identity", counting_identity_load)
    monkeypatch.setattr(provenance_mod, "build_provenance",
                        lambda **kw: _prov() | {"argv": ["--output-dir", "/tmp/fin"]})

    argv = [
        "--evidence-root", str(tmp_path / "i074_stage1"),
        "--run-identity", str(external),
        "--image-digest", IMG, "--base-commit", "e" * 40,
        "--tooling-patch-sha256", "f" * 64, "--source-root", "/app",
        "--runner-sha256", "d" * 64,
    ]
    for rel, path in sources.items():
        argv += ["--source", f"{rel}={path}"]
    ev.run_evidence(argv)

    assert loads == ["evidence_load"], (
        f"archived identity 被載入 {len(loads)} 次（{loads}）——"
        "⛔ 只能由 `_load_all()` 讀那一次"
    )


def test_finalizer_passes_config_module_to_builder(tmp_path, monkeypatch):
    """⚠️ **phase-A 的 config 載入要是可執行保證**，⛔ 不是 incidental import。

    `build_provenance()` 的 dict literal 裡 `project_module_hashes()` 排在
    `runtime_settings()` **之前**求值，而後者在 `config_module is None` 時才 lazy-import
    `config`——⚠️ 而 `config` 正是專案模組，⛔ 那時它的 hash 會被漏記。
    """
    from ..replay_bundle import provenance as provenance_mod

    seen = {}
    prov = _prov() | {"argv": ["--output-dir", "/tmp/fin"]}

    def fake_build(**kwargs):
        seen.update(kwargs)
        return prov

    monkeypatch.setattr(provenance_mod, "build_provenance", fake_build)
    sources = _write_sources(tmp_path)
    argv = [
        "--evidence-root", str(tmp_path / "i074_stage1"),
        "--run-identity", str(sources["identity/run_identity.json.gz"]),
        "--image-digest", IMG, "--base-commit", "e" * 40,
        "--tooling-patch-sha256", "f" * 64, "--source-root", "/app",
        "--runner-sha256", "d" * 64,
    ]
    for rel, path in sources.items():
        argv += ["--source", f"{rel}={path}"]
    ev.run_evidence(argv)
    assert seen.get("config_module") is not None, \
        "⛔ builder 沒有收到 config_module——phase-A 的載入保證沒有生效"
