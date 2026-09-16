"""run identity 的 schema、重入語意與 commit point（I-074 Stage 1）。

契約見 `docs/sr-zone-scoring.md`「I-074 Stage 1 的四個永久契約」與
`docs/development-workflow.md`「I-074 Stage 1 的正式執行程序」（計畫書已收斂）。
"""
from __future__ import annotations

import datetime
import gzip

import pytest

from ..replay_bundle import run_identity as ri
from ..replay_bundle.artifacts import ArtifactError
from ..replay_bundle.canonical import canonical_gzip_bytes, canonical_json_bytes
from ..replay_bundle.publish import DurabilityUnconfirmed

IMG = "sha256:" + "a" * 64
BUNDLE = "b1_20260901_1d_74350966_5d7ecb10"


def _now() -> str:
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def _payload(**overrides):
    payload = {
        "schema_version": 1,
        "kind": "sr_zone_run_identity",
        "bundle_id": BUNDLE,
        "expected_image_id": IMG,
        "created_at": _now(),
    }
    payload.update(overrides)
    return payload


# ── schema ──────────────────────────────────────────────────────────────────

def test_valid_payload_passes():
    ri.validate_run_identity(_payload())


@pytest.mark.parametrize("field", sorted(ri.RUN_IDENTITY_FIELDS))
def test_missing_field_is_rejected(field):
    payload = _payload()
    del payload[field]
    with pytest.raises(ArtifactError):
        ri.validate_run_identity(payload)


def test_unknown_field_is_rejected():
    with pytest.raises(ArtifactError):
        ri.validate_run_identity(_payload(surprise=1))


@pytest.mark.parametrize("bad", [True, 2, "1", 1.0])
def test_schema_version_must_be_strict_int_one(bad):
    """⚠️ `True == 1` 為真——只比值不比型別的話，`True` 會被放行。"""
    with pytest.raises(ArtifactError):
        ri.validate_run_identity(_payload(schema_version=bad))


@pytest.mark.parametrize("bad", [
    "sha256:" + "A" * 64,          # ⛔ 大寫
    "sha256:" + "a" * 63,          # 長度不足
    "a" * 64,                      # 缺前綴
    "sha256:" + "z" * 64,          # 非 hex
    None,
])
def test_image_id_format(bad):
    with pytest.raises(ArtifactError):
        ri.validate_run_identity(_payload(expected_image_id=bad))


@pytest.mark.parametrize("bad", ["2026-09-14T00:00:00", "not-a-time", ""])
def test_created_at_must_have_timezone(bad):
    """⛔ 缺時區的時間戳不接受——跨日比較會因此失準。"""
    with pytest.raises(ArtifactError):
        ri.validate_run_identity(_payload(created_at=bad))


# ── 兩種格式的載入 ───────────────────────────────────────────────────────────

def test_load_plain_json_and_gzip(tmp_path):
    payload = _payload()
    blob = canonical_json_bytes(payload)
    plain = tmp_path / "run_identity.json"
    plain.write_bytes(blob)
    archived = tmp_path / "run_identity.json.gz"
    archived.write_bytes(canonical_gzip_bytes(blob))

    assert ri.load_run_identity(plain) == payload
    assert ri.load_run_identity(archived) == payload


def test_non_canonical_json_is_rejected(tmp_path):
    """⚠️ 語意相同但編碼不同也要拒絕——SHA 的定義才唯一。"""
    import json

    path = tmp_path / "run_identity.json"
    path.write_bytes(json.dumps(_payload(), indent=4).encode("utf-8"))
    with pytest.raises(ArtifactError):
        ri.load_run_identity(path)


def test_non_canonical_gzip_is_rejected(tmp_path):
    """⛔ 只驗 mtime／filename 兩個 header 欄位擋不住 compression level 的差異。"""
    path = tmp_path / "run_identity.json.gz"
    path.write_bytes(gzip.compress(canonical_json_bytes(_payload()), 1, mtime=0))
    with pytest.raises(ArtifactError):
        ri.load_run_identity(path)


# ── 11-D：producer 的三條分支 ────────────────────────────────────────────────

def test_creates_when_absent(tmp_path):
    path = tmp_path / "run_identity.json"
    payload = ri.ensure_run_identity(
        path, bundle_id=BUNDLE, expected_image_id=IMG, created_at=_now()
    )
    assert path.is_file()
    assert payload["expected_image_id"] == IMG


def test_existing_is_a_noop_and_keeps_created_at(tmp_path):
    """⚠️ ⛔ 不比對 `created_at`：它每次都變，「所有欄位相同才 no-op」永遠不成立。"""
    path = tmp_path / "run_identity.json"
    first = ri.ensure_run_identity(
        path, bundle_id=BUNDLE, expected_image_id=IMG, created_at=_now()
    )
    before = path.read_bytes()

    again = ri.ensure_run_identity(
        path, bundle_id=BUNDLE, expected_image_id="sha256:" + "b" * 64,
        created_at="1999-01-01T00:00:00+00:00",
    )
    assert again == first, "no-op ⛔ 不得被新的參數覆寫"
    assert path.read_bytes() == before, "no-op ⛔ 不得重寫檔案"


def test_different_bundle_is_rejected(tmp_path):
    path = tmp_path / "run_identity.json"
    ri.ensure_run_identity(path, bundle_id=BUNDLE, expected_image_id=IMG, created_at=_now())
    with pytest.raises(ArtifactError):
        ri.ensure_run_identity(
            path, bundle_id="OTHER", expected_image_id=IMG, created_at=_now()
        )


# ── 11-E／11-E-2：commit point ──────────────────────────────────────────────

def test_parent_fsync_failure_keeps_file_and_reports_unconfirmed(tmp_path, monkeypatch):
    """⚠️ rename 成功、parent fsync 失敗 → **保留檔案**、回報 durability 未確認。

    ⛔ 不刪除——那份內容是有效的，刪掉反而要重建。
    """
    path = tmp_path / "run_identity.json"

    def boom(_):
        raise OSError("injected")

    monkeypatch.setattr(ri, "fsync_dir", boom)
    with pytest.raises(DurabilityUnconfirmed) as exc:
        ri.ensure_run_identity(path, bundle_id=BUNDLE, expected_image_id=IMG, created_at=_now())
    assert exc.value.published is True
    assert path.is_file(), "⛔ 不得刪除已 rename 成功的 identity"


def test_noop_path_refsyncs_and_is_the_repair_route(tmp_path, monkeypatch):
    """⚠️ **這條是上一個狀態的唯一修復路徑**。

    ⛔ 少了它，「rename 成功但沒落盤」永遠修不好——no-op 分支直接回傳 ID 而不重新 fsync。
    """
    path = tmp_path / "run_identity.json"
    ri.ensure_run_identity(path, bundle_id=BUNDLE, expected_image_id=IMG, created_at=_now())

    calls = []
    real_dir, real_file = ri.fsync_dir, ri.fsync_file
    monkeypatch.setattr(ri, "fsync_dir", lambda p: calls.append(("dir", p)) or real_dir(p))
    monkeypatch.setattr(ri, "fsync_file", lambda p: calls.append(("file", p)) or real_file(p))

    ri.ensure_run_identity(path, bundle_id=BUNDLE, expected_image_id=IMG, created_at=_now())
    kinds = [k for k, _ in calls]
    assert "file" in kinds and "dir" in kinds, f"no-op 必須重新 fsync file ＋ parent，實際 {kinds}"


@pytest.mark.parametrize("failing", ["file", "dir"])
def test_noop_refsync_failure_keeps_file(tmp_path, monkeypatch, failing):
    """11-E-2：no-op 的兩次 fsync 任一失敗 → 保留 identity、⛔ 不重寫。"""
    path = tmp_path / "run_identity.json"
    ri.ensure_run_identity(path, bundle_id=BUNDLE, expected_image_id=IMG, created_at=_now())
    before = path.read_bytes()

    def boom(_):
        raise OSError("injected")

    monkeypatch.setattr(ri, "fsync_file" if failing == "file" else "fsync_dir", boom)
    # ⚠️ **兩者都必須是 `DurabilityUnconfirmed`**（⛔ 不接受裸 OSError）：
    # 檔案內容是有效的，退化成一般錯誤會讓呼叫端分不出「要重跑」與「輸入壞了」。
    with pytest.raises(DurabilityUnconfirmed):
        ri.ensure_run_identity(path, bundle_id=BUNDLE, expected_image_id=IMG, created_at=_now())
    assert path.read_bytes() == before, "⛔ 不得刪除或重寫既有 identity"
