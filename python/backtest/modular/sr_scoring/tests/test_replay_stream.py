"""串流讀 canonical artifact（I-074 Stage 2 ③b，issue.md I-074 ③ evidence contract「五之一」）。

⚠️ 核心斷言是**與整份載入版同強度**：同一份檔案，串流版與 `load_canonical_evidence_artifact()`
得到的 SHA／bytes／內容必須相同，而整份載入版會拒絕的檔案，串流版也必須拒絕。
"""
from __future__ import annotations

import gzip
import io
import json

import pytest

from ..replay_bundle import stream as st
from ..replay_bundle.artifacts import AFTER_KIND, ArtifactError, load_canonical_evidence_artifact
from ..replay_bundle.canonical import canonical_gzip_bytes, canonical_json_bytes, sha256_hex
from .test_replay_evidence import _after, _row


def _artifact(n=5):
    rows = [_row(f"2026-08-{10 + i:02d}") for i in range(n)]
    if n > 1:
        rows[1]["rr_decoupling_candidate"] = True
        rows[1]["lifecycle_phase"] = "CONTINUATION"
    return _after("2026-09-15T09:00:00+08:00", rows, "/tmp/d1")


def _collect(path, **kwargs):
    rows, row_bytes = [], []

    def on_row(row, raw):
        rows.append(row)
        row_bytes.append(raw)

    load = st.stream_canonical_artifact(path, AFTER_KIND, on_row=on_row, **kwargs)
    return load, rows, row_bytes


@pytest.fixture(params=[1, 7, 64, 1 << 20], ids=["chunk1", "chunk7", "chunk64", "chunk1MiB"])
def chunk(request, monkeypatch):
    """⚠️ 把讀取 chunk 調到極小，逼出「一個值被切在兩段之間」的每一種邊界。"""
    monkeypatch.setattr(st, "READ_CHUNK", request.param)
    return request.param


@pytest.mark.parametrize("suffix", [".json", ".json.gz"])
def test_parity_with_whole_file_loader(tmp_path, chunk, suffix):
    artifact = _artifact()
    payload = canonical_json_bytes(artifact)
    path = tmp_path / f"after{suffix}"
    path.write_bytes(payload if suffix == ".json" else canonical_gzip_bytes(payload))

    whole = load_canonical_evidence_artifact(path, AFTER_KIND)
    load, rows, row_bytes = _collect(path)
    assert load.artifact_sha256 == whole.artifact_sha256
    assert load.stored_sha256 == whole.stored_sha256
    assert load.stored_bytes == whole.stored_bytes
    assert rows == whole.parsed["rows"]
    assert load.top == {k: v for k, v in whole.parsed.items() if k != "rows"}
    assert load.row_count == len(rows)
    assert row_bytes == [canonical_json_bytes(r) for r in rows]


def test_copy_to_is_the_canonical_gzip_of_the_same_read(tmp_path, chunk):
    artifact = _artifact()
    src = tmp_path / "after.json"
    src.write_bytes(canonical_json_bytes(artifact))
    dst = tmp_path / "copy.json.gz"
    load, _, _ = _collect(src, copy_to=dst)
    blob = dst.read_bytes()
    assert blob == canonical_gzip_bytes(canonical_json_bytes(artifact))
    assert load.copy_sha256 == sha256_hex(blob) and load.copy_bytes == len(blob)


def test_empty_rows_is_legal(tmp_path):
    path = tmp_path / "after.json"
    path.write_bytes(canonical_json_bytes(_artifact(0)))
    load, rows, _ = _collect(path)
    assert rows == [] and load.row_count == 0


# ── 整份載入版會拒絕的，串流版也要拒絕 ─────────────────────────────────────

def _write(tmp_path, raw: bytes, name="after.json"):
    path = tmp_path / name
    path.write_bytes(raw)
    return path


def _both_reject(path):
    # ⚠️ 整份載入版遇到 NaN 會從 `canonical_json_bytes()` 拋 `CanonicalError`——它與
    # `ArtifactError` 都是 `ValueError`（CLI 的 generic catch 接的就是 ValueError）。
    with pytest.raises(ValueError):
        load_canonical_evidence_artifact(path, AFTER_KIND)
    with pytest.raises(ArtifactError):
        _collect(path)


def test_rejects_whitespace_formatting(tmp_path, chunk):
    raw = json.dumps(_artifact(), sort_keys=True, ensure_ascii=False, indent=1).encode()
    _both_reject(_write(tmp_path, raw))


def test_rejects_trailing_newline(tmp_path):
    _both_reject(_write(tmp_path, canonical_json_bytes(_artifact()) + b"\n"))


def test_rejects_unsorted_top_level_keys(tmp_path):
    artifact = _artifact()
    raw = json.dumps(dict(reversed(list(artifact.items()))), separators=(",", ":"),
                     ensure_ascii=False).encode()
    _both_reject(_write(tmp_path, raw))


def test_rejects_unsorted_keys_inside_a_row(tmp_path, chunk):
    artifact = _artifact()
    body = canonical_json_bytes(artifact).decode()
    row = artifact["rows"][0]
    canonical_row = canonical_json_bytes(row).decode()
    shuffled = json.dumps(dict(reversed(list(sorted(row.items())))), separators=(",", ":"),
                          ensure_ascii=False)
    _both_reject(_write(tmp_path, body.replace(canonical_row, shuffled, 1).encode()))


def test_rejects_non_canonical_gzip_level(tmp_path, chunk):
    buf = io.BytesIO()
    with gzip.GzipFile(filename="", mode="wb", fileobj=buf, compresslevel=1, mtime=0) as fh:
        fh.write(canonical_json_bytes(_artifact()))
    _both_reject(_write(tmp_path, buf.getvalue(), "after.json.gz"))


def test_rejects_gzip_with_trailing_member(tmp_path):
    blob = canonical_gzip_bytes(canonical_json_bytes(_artifact()))
    _both_reject(_write(tmp_path, blob + canonical_gzip_bytes(b"x"), "after.json.gz"))


def test_rejects_truncated_gzip(tmp_path):
    blob = canonical_gzip_bytes(canonical_json_bytes(_artifact()))
    _both_reject(_write(tmp_path, blob[:-9], "after.json.gz"))


def test_rejects_wrong_kind_and_schema_version(tmp_path):
    artifact = _artifact()
    _both_reject(_write(tmp_path, canonical_json_bytes(artifact | {"kind": "other"}), "a.json"))
    _both_reject(_write(tmp_path, canonical_json_bytes(artifact | {"schema_version": 2}), "b.json"))


def test_rejects_nan_inside_a_row(tmp_path):
    body = canonical_json_bytes(_artifact()).decode()
    raw = body.replace('"clear_zone_breakout":false', '"clear_zone_breakout":NaN', 1).encode()
    _both_reject(_write(tmp_path, raw))


def test_rejects_missing_stream_field(tmp_path):
    artifact = _artifact()
    del artifact["rows"]
    with pytest.raises(ArtifactError, match="缺少要串流的欄位"):
        _collect(_write(tmp_path, canonical_json_bytes(artifact)))


def test_failed_read_removes_the_partial_copy(tmp_path):
    src = _write(tmp_path, canonical_json_bytes(_artifact()) + b"\n")
    dst = tmp_path / "copy.json.gz"
    with pytest.raises(ArtifactError):
        _collect(src, copy_to=dst)
    assert not dst.exists()


# ── stream_after_artifact：摘要與驗證 ───────────────────────────────────────

def test_after_stream_summary(tmp_path, chunk):
    artifact = _artifact()
    path = _write(tmp_path, canonical_json_bytes(artifact))
    keep = {("2330", "1d", "2026-08-11")}
    result = st.stream_after_artifact(path, label="t", keep_keys=keep)
    assert result.keys == [("2330", "1d", r["as_of"]) for r in artifact["rows"]]
    assert result.candidate_keys == [("2330", "1d", "2026-08-11")]
    assert set(result.kept_rows) == keep
    assert result.kept_rows[("2330", "1d", "2026-08-11")] == artifact["rows"][1]
    assert all(len(d) == 32 for d in result.row_digests.values())


def test_after_stream_rejects_envelope_problems(tmp_path):
    artifact = _artifact() | {"unexpected": 1}
    with pytest.raises(ArtifactError, match="欄位不符"):
        st.stream_after_artifact(_write(tmp_path, canonical_json_bytes(artifact)), label="t")


def test_after_stream_failure_removes_copy(tmp_path):
    artifact = _artifact()
    artifact["rows"][2] = dict(artifact["rows"][1])  # 重複 key
    dst = tmp_path / "copy.json.gz"
    with pytest.raises(ArtifactError, match="重複的 key"):
        st.stream_after_artifact(_write(tmp_path, canonical_json_bytes(artifact)), label="t",
                                 copy_to=dst)
    assert not dst.exists()
