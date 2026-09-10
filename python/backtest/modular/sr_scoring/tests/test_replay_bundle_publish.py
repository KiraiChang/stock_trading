"""七-B 的整包原子發布（I-100 Phase A）。

這一檔對應計畫書「十二、測試與驗證策略」的**原子發布**那一列：兩 producer 在 rename 前
碰撞、既有空目錄／損壞 bundle 的不變性、rename 前失敗、同 payload 重產、fsync 失敗的
兩條路徑、probe 的 A／B 兩組，以及 `RENAME_NOREPLACE` 不受支援時的 fail-closed。
"""
from __future__ import annotations

import errno
import json
import os
from pathlib import Path

import pytest

from ..replay_bundle import (
    BundleError,
    DurabilityUnconfirmed,
    NoClobberUnsupported,
    build_manifest,
    emit_bundle,
    load_bundle,
    probe_no_clobber,
    rename_noreplace,
)
from ..replay_bundle import bundle as bundle_module
from ..replay_bundle import publish as publish_module
from .replay_bundle_fixtures import make_manifest_and_payloads, make_replay_config


# ── probe（A／B 兩組，目錄形狀） ─────────────────────────────────────────────

def test_probe_passes_and_cleans_up(tmp_path):
    before = set(tmp_path.iterdir())
    result = probe_no_clobber(tmp_path)
    assert result["probe_a"] == "renamed"
    assert result["probe_b"] == "eexist"
    # ⚠️ 所有 probe 目錄在每一條路徑都要清掉。
    assert set(tmp_path.iterdir()) == before


def test_probe_b_leaves_both_sides_untouched(tmp_path, monkeypatch):
    """B 組的斷言本身：destination 已存在時，source 與 destination 都不能被動到。"""
    seen: dict[str, bytes] = {}
    real = publish_module.rename_noreplace

    def spy(src: Path, dst: Path):
        if dst.name.endswith("b-dst"):
            seen["src"] = (src / "marker").read_bytes()
            seen["dst"] = (dst / "marker").read_bytes()
        return real(src, dst)

    monkeypatch.setattr(publish_module, "rename_noreplace", spy)
    probe_no_clobber(tmp_path)
    assert seen == {"src": b"src", "dst": b"dst"}


def test_rename_noreplace_refuses_existing_directory(tmp_path):
    src = tmp_path / "src"
    dst = tmp_path / "dst"
    src.mkdir()
    dst.mkdir()
    (dst / "keep").write_text("keep", encoding="utf-8")
    with pytest.raises(FileExistsError):
        rename_noreplace(src, dst)
    # ⛔ POSIX 的 rename() 會直接蓋掉既有空目錄——這裡必須沒有發生。
    assert src.is_dir()
    assert (dst / "keep").read_text(encoding="utf-8") == "keep"


def test_no_clobber_unsupported_is_fail_closed_with_remediation(tmp_path, monkeypatch):
    """⛔ 不受支援時不得有任何弱化的發布路徑，訊息要指出可行的補救。"""
    def unsupported(src, dst):
        raise NoClobberUnsupported("filesystem 不支援\n補救方式：**讓最終的 python/baselines/**")

    monkeypatch.setattr(publish_module, "rename_noreplace", unsupported)
    monkeypatch.setattr(bundle_module, "probe_no_clobber", lambda d: (_ for _ in ()).throw(
        NoClobberUnsupported("probe 不通過\n補救方式：**讓最終的 python/baselines/**")))
    manifest, payloads = make_manifest_and_payloads()
    with pytest.raises(NoClobberUnsupported) as exc:
        emit_bundle(tmp_path, payloads, manifest)
    assert "補救方式" in str(exc.value)
    assert not (tmp_path / manifest["bundle_id"]).exists()


def test_exdev_is_fail_closed(tmp_path, monkeypatch):
    """staging 與正式路徑不同 filesystem → EXDEV，⛔ 不得改用 copy。"""
    def fake_loader():
        def call(src: bytes, dst: bytes) -> int:
            import ctypes

            ctypes.set_errno(errno.EXDEV)
            return -1

        return call

    monkeypatch.setattr(publish_module, "_load_renameat2", fake_loader)
    with pytest.raises(NoClobberUnsupported) as exc:
        rename_noreplace(tmp_path / "a", tmp_path / "b")
    assert "EXDEV" in str(exc.value)
    assert "不改用 copy" in str(exc.value)


# ── 發布 ────────────────────────────────────────────────────────────────────

def test_publish_creates_final_path_once_and_leaves_no_staging(tmp_path):
    manifest, payloads = make_manifest_and_payloads()
    outcome = emit_bundle(tmp_path, payloads, manifest)
    assert outcome.published is True
    assert outcome.path == tmp_path / manifest["bundle_id"]
    loaded = load_bundle(outcome.path)
    assert loaded.bundle_id == manifest["bundle_id"]
    assert [p.name for p in tmp_path.iterdir() if p.name.startswith(".staging-")] == []


def test_final_path_never_appears_before_rename(tmp_path):
    """⚠️「發布本身不會製造中間狀態」：rename 之前正式路徑在任何時點都不存在。"""
    manifest, payloads = make_manifest_and_payloads()
    final_dir = tmp_path / manifest["bundle_id"]
    seen: list[bool] = []
    real = publish_module.rename_noreplace

    def spy(src, dst):
        seen.append(final_dir.exists())
        return real(src, dst)

    import unittest.mock as mock

    with mock.patch.object(bundle_module, "rename_noreplace", spy):
        emit_bundle(tmp_path, payloads, manifest)
    assert seen == [False]


def test_second_run_with_same_payload_is_noop_despite_captured_at(tmp_path):
    """⑤同 payload 重產（`captured_at` 不同）→ no-op，⛔ 不得因 manifest 差異中止。"""
    manifest, payloads = make_manifest_and_payloads(captured_at="2026-09-01T01:00:00+00:00")
    first = emit_bundle(tmp_path, payloads, manifest)
    original = (first.path / "manifest.json").read_bytes()

    manifest2, payloads2 = make_manifest_and_payloads(captured_at="2026-09-02T09:00:00+00:00")
    assert manifest2["bundle_id"] == manifest["bundle_id"]
    assert manifest2 != manifest
    second = emit_bundle(tmp_path, payloads2, manifest2)
    assert second.published is False
    # ⛔ 既有正式目錄逐位元不變。
    assert (first.path / "manifest.json").read_bytes() == original


def test_existing_bundle_with_different_payload_aborts(tmp_path, monkeypatch):
    """同 ID 但 payload 不同 → 中止。

    ⚠️ 8 hex ＝ 32 bit，本來就不是防碰撞用的，真的碰撞不可能手造——所以把
    `compute_bundle_id` 釘成固定值來模擬。**要驗的是碰撞被逐檔完整 SHA-256 抓到**，
    不是碰撞多容易發生。
    """
    manifest, payloads = make_manifest_and_payloads()
    emit_bundle(tmp_path, payloads, manifest)

    fixed = manifest["bundle_id"]
    monkeypatch.setattr(bundle_module, "compute_bundle_id", lambda **kwargs: fixed)
    other_manifest, other_payloads = make_manifest_and_payloads(
        replay_config=make_replay_config(dataset_config={"min_history_bars": 999})
    )
    assert other_manifest["files"] != manifest["files"]
    with pytest.raises(BundleError) as exc:
        emit_bundle(tmp_path, other_payloads, other_manifest)
    assert "不覆蓋" in str(exc.value)
    # ⛔ 既有那份不得被動到。
    assert load_bundle(tmp_path / fixed).file_hashes == manifest["files"]


def test_existing_empty_final_directory_aborts_and_stays_empty(tmp_path):
    """②執行前已有**空的正式目錄** → 中止且該目錄仍是空的。"""
    manifest, payloads = make_manifest_and_payloads()
    final_dir = tmp_path / manifest["bundle_id"]
    final_dir.mkdir()
    with pytest.raises(BundleError):
        emit_bundle(tmp_path, payloads, manifest)
    assert list(final_dir.iterdir()) == []


def test_existing_corrupt_bundle_aborts_and_stays_bit_identical(tmp_path):
    """③執行前已有**損壞的 bundle** → 中止且該目錄逐位元不變。"""
    manifest, payloads = make_manifest_and_payloads()
    published = emit_bundle(tmp_path, payloads, manifest)
    target = published.path / "replay_config.json"
    target.write_bytes(b'{"tampered":true}')
    snapshot = {p.name: p.read_bytes() for p in published.path.iterdir()}

    manifest2, payloads2 = make_manifest_and_payloads()
    with pytest.raises(BundleError) as exc:
        emit_bundle(tmp_path, payloads2, manifest2)
    assert "沒通過完整驗證" in str(exc.value)
    assert {p.name: p.read_bytes() for p in published.path.iterdir()} == snapshot


def test_failure_before_rename_leaves_no_final_path_and_no_staging(tmp_path):
    """④在 rename 呼叫前注入失敗 → 正式路徑不存在、staging 已清掉。"""
    manifest, payloads = make_manifest_and_payloads()

    def boom(src, dst):
        raise BundleError("injected before rename")

    import unittest.mock as mock

    with mock.patch.object(bundle_module, "rename_noreplace", boom):
        with pytest.raises(BundleError):
            emit_bundle(tmp_path, payloads, manifest)
    assert not (tmp_path / manifest["bundle_id"]).exists()
    assert [p.name for p in tmp_path.iterdir() if p.name.startswith(".staging-")] == []


def test_failure_during_staging_validation_leaves_no_final_path(tmp_path):
    """staging 驗證失敗（③～④之間）也一樣：正式路徑不存在、staging 已清掉。"""
    manifest, payloads = make_manifest_and_payloads()

    import unittest.mock as mock

    with mock.patch.object(bundle_module, "load_bundle", side_effect=BundleError("staging bad")):
        with pytest.raises(BundleError):
            emit_bundle(tmp_path, payloads, manifest)
    assert not (tmp_path / manifest["bundle_id"]).exists()
    assert [p.name for p in tmp_path.iterdir() if p.name.startswith(".staging-")] == []


def test_two_producers_collide_at_rename_one_publishes_one_noops(tmp_path):
    """①兩個 producer 同時發布同一 ID：在 rename **呼叫前**插入 deterministic barrier。"""
    manifest_a, payloads_a = make_manifest_and_payloads(captured_at="2026-09-01T01:00:00+00:00")
    manifest_b, payloads_b = make_manifest_and_payloads(captured_at="2026-09-01T02:00:00+00:00")
    assert manifest_a["bundle_id"] == manifest_b["bundle_id"]

    import threading
    import unittest.mock as mock

    barrier = threading.Barrier(2)
    real = publish_module.rename_noreplace

    def gated(src, dst):
        barrier.wait(timeout=10)
        return real(src, dst)

    results: dict[str, object] = {}

    def run(tag, manifest, payloads):
        try:
            results[tag] = emit_bundle(tmp_path, payloads, manifest)
        except Exception as exc:  # noqa: BLE001 - 測試要看兩邊各自的結果
            results[tag] = exc

    with mock.patch.object(bundle_module, "rename_noreplace", gated):
        threads = [
            threading.Thread(target=run, args=("a", manifest_a, payloads_a)),
            threading.Thread(target=run, args=("b", manifest_b, payloads_b)),
        ]
        for t in threads:
            t.start()
        for t in threads:
            t.join(timeout=30)

    outcomes = [results["a"], results["b"]]
    assert all(not isinstance(o, Exception) for o in outcomes), outcomes
    assert sorted(o.published for o in outcomes) == [False, True]
    # 最終內容等於先寫入的那一份——兩者 payload 相同，載得進來即可。
    assert load_bundle(tmp_path / manifest_a["bundle_id"]).bundle_id == manifest_a["bundle_id"]
    assert [p.name for p in tmp_path.iterdir() if p.name.startswith(".staging-")] == []


# ── commit point：rename 成功之後的 fsync 失敗 ──────────────────────────────

def _fail_parent_fsync(monkeypatch, baselines: Path):
    real = publish_module.fsync_dir

    def maybe_fail(path: Path):
        if Path(path) == baselines:
            raise OSError(errno.EIO, "injected fsync failure")
        return real(path)

    monkeypatch.setattr(bundle_module, "fsync_dir", maybe_fail)


def test_fsync_failure_after_rename_keeps_published_bundle(tmp_path, monkeypatch):
    """⑥rename 路徑：回專屬狀態，bundle 仍完整可載入，⛔ 未被刪除。"""
    manifest, payloads = make_manifest_and_payloads()
    _fail_parent_fsync(monkeypatch, tmp_path)
    with pytest.raises(DurabilityUnconfirmed) as exc:
        emit_bundle(tmp_path, payloads, manifest)
    assert exc.value.published is True
    assert load_bundle(tmp_path / manifest["bundle_id"]).bundle_id == manifest["bundle_id"]


def test_fsync_failure_on_noop_path_does_not_touch_existing(tmp_path, monkeypatch):
    """⑥no-op 路徑：同一類狀態，且⛔ 完全不修改正式目錄。"""
    manifest, payloads = make_manifest_and_payloads()
    published = emit_bundle(tmp_path, payloads, manifest)
    snapshot = {p.name: p.read_bytes() for p in published.path.iterdir()}

    manifest2, payloads2 = make_manifest_and_payloads(captured_at="2026-09-03T00:00:00+00:00")
    _fail_parent_fsync(monkeypatch, tmp_path)
    with pytest.raises(DurabilityUnconfirmed) as exc:
        emit_bundle(tmp_path, payloads2, manifest2)
    assert exc.value.published is False
    assert {p.name: p.read_bytes() for p in published.path.iterdir()} == snapshot


def test_rerun_after_durability_failure_is_noop_and_refsyncs(tmp_path, monkeypatch):
    """重跑該指令 → no-op 並**重新 fsync**（v22 修的正是這條）。"""
    manifest, payloads = make_manifest_and_payloads()
    _fail_parent_fsync(monkeypatch, tmp_path)
    with pytest.raises(DurabilityUnconfirmed):
        emit_bundle(tmp_path, payloads, manifest)

    monkeypatch.undo()
    calls: list[Path] = []
    real = publish_module.fsync_dir

    def record(path):
        calls.append(Path(path))
        return real(path)

    monkeypatch.setattr(bundle_module, "fsync_dir", record)
    manifest2, payloads2 = make_manifest_and_payloads(captured_at="2026-09-04T00:00:00+00:00")
    outcome = emit_bundle(tmp_path, payloads2, manifest2)
    assert outcome.published is False
    assert tmp_path in calls
