"""Stage 1／2 的三份 artifact：schema、原子發布與四道集合檢查。

**這一層是純資料**：它不知道 replay 怎麼跑，只負責「產出什麼、怎麼落地、怎麼驗」。
replay 的流程協調在 `evaluation.py`——⛔ 本 package 不 import 它。

**原子發布契約**：`--output-dir` 必須不存在或為空；逐檔先寫同目錄 temp 再 `os.replace`；
**Stage 1 最後才發布 `cohort_manifest.json`；Stage 2 先驗 `comparison_artifact.json` 的 hash，
最後才發布 `report.json`**——「manifest／report 存在」即代表其指向的證據已完整落地。
"""
from __future__ import annotations

import os
import secrets
from pathlib import Path
from typing import Any, Iterable, Sequence

from .canonical import canonical_json_bytes, sha256_hex
from .publish import fsync_dir, fsync_file

ARTIFACT_SCHEMA_VERSION = 1

AFTER_ARTIFACT_NAME = "after_artifact.json"
COHORT_MANIFEST_NAME = "cohort_manifest.json"
COMPARISON_ARTIFACT_NAME = "comparison_artifact.json"
REPORT_NAME = "report.json"

AFTER_KIND = "sr_zone_replay_after"
COHORT_KIND = "sr_zone_replay_cohort"
COMPARISON_KIND = "sr_zone_replay_comparison"
REPORT_KIND = "sr_zone_replay_report"

# ⚠️ 候選的定義**不在這裡重算**：它是 I-074 Stage 0 由 decision semantic pipeline 產出的
# 診斷欄位，本筆只消費。⛔ 不得用近似欄位反推——那會產生偽陽性。
CANDIDATE_FIELD = "rr_decoupling_candidate"

_I074_HINT = (
    f"replay row 缺少嚴格 boolean 的 {CANDIDATE_FIELD}。"
    "這個欄位由 issue.md I-074 Stage 0 的診斷欄位補上（lifecycle_engine 產三項價格證據與 "
    "clear_zone_breakout，decision_engine 的 semantic pipeline 組出本欄位）。"
    "⛔ 本工具只消費、不重算、也不用近似欄位反推，所以在欄位就位前 Stage 1 跑不動。"
)


class ArtifactError(ValueError):
    """artifact 的產生、載入或集合檢查被拒絕。"""


# 三份 artifact 的封閉欄位集合。⛔ 多一個未知欄位、少一個必要欄位都中止。
_AFTER_FIELDS = {"schema_version", "kind", "bundle_id", "timeframe", "replay_scope",
                 "run_id", "pipeline_version", "generated_at", "provenance", "rows"}
_COHORT_FIELDS = {"schema_version", "kind", "bundle_id", "after_artifact_sha256",
                  "generated_at", "provenance", "keys"}


# ── key 與集合檢查 ──────────────────────────────────────────────────────────

def row_key(row: dict[str, Any]) -> tuple[str, str, str]:
    """`(symbol, timeframe, as_of)`。三個欄位缺一或型別不對即中止。

    ⛔ **不用 `str()` 硬轉**：那會讓 `timeframe: 123` 這種列悄悄變成 `"123"` 而通過，
    而這三個欄位是逐列比較的**身分**——身分不該由轉型湊出來。
    """
    out: list[str] = []
    for field in ("symbol", "timeframe", "as_of"):
        if field not in row:
            raise ArtifactError(f"replay row 缺少 key 欄位 {field!r}")
        value = row[field]
        if not isinstance(value, str) or not value:
            raise ArtifactError(f"replay row 的 {field} 必須是非空字串：{value!r}")
        out.append(value)
    return (out[0], out[1], out[2])


def assert_unique_keys(keys: Sequence[tuple], label: str) -> None:
    """⛔ `keys(...) == keys(...)` 不能用 set 實作——重複列會被折疊掉，
    而「before 版重複算了一列」正是要抓的錯誤之一。所以先各自驗唯一性。
    """
    if len(keys) != len(set(keys)):
        duplicates = sorted({k for k in keys if list(keys).count(k) > 1})
        raise ArtifactError(f"{label} 有重複的 key：{duplicates[:5]}（共 {len(duplicates)} 組）")


def assert_same_keys(left: Sequence[tuple], right: Sequence[tuple], label: str) -> None:
    if sorted(left) != sorted(right):
        only_left = sorted(set(left) - set(right))[:5]
        only_right = sorted(set(right) - set(left))[:5]
        raise ArtifactError(
            f"{label} 的 key 集合不相等：只在左邊 {only_left}、只在右邊 {only_right}"
        )


def validate_candidate_flags(rows: Iterable[dict[str, Any]], label: str) -> None:
    """每一列都必須有嚴格 boolean 的 `rr_decoupling_candidate`。

    ⚠️ **欄位完整、但所有列都明確為 `false` 時，空 cohort 是合法結果**——⛔ 不得把
    「真正零命中」判成錯誤。這道守門禁止的是**因欄位缺失而靜默變成空 cohort**。
    """
    for row in rows:
        if CANDIDATE_FIELD not in row:
            raise ArtifactError(f"{label}：{_I074_HINT}")
        value = row[CANDIDATE_FIELD]
        # ⛔ 不能用 `isinstance(x, int)`：bool 是 int 的子類，`0`／`1` 會通過。
        if not isinstance(value, bool):
            raise ArtifactError(
                f"{label}：{CANDIDATE_FIELD} 必須是 JSON true/false，實際是 {value!r}。{_I074_HINT}"
            )


def _require_fields(data: dict[str, Any], fields: set[str], label: str) -> None:
    unknown, missing = set(data) - fields, fields - set(data)
    if unknown or missing:
        raise ArtifactError(f"{label} 欄位不符：缺 {sorted(missing)}，多 {sorted(unknown)}")


def _require_type(data: dict[str, Any], field: str, types, label: str, *, optional=False) -> None:
    value = data[field]
    if optional and value is None:
        return
    if not isinstance(value, types) or isinstance(value, bool):
        raise ArtifactError(f"{label}.{field} 型別不符：{value!r}")


def validate_after_artifact(artifact: dict[str, Any]) -> list[tuple[str, str, str]]:
    """after artifact 的完整結構檢查，回傳逐列 key。

    ⚠️ **這一整組必須在跑 replay 之前做完**：Stage 2 的 replay 要跑約 3.7 小時，
    壞掉的 artifact 沒有理由等到那之後才被發現。
    """
    _require_fields(artifact, _AFTER_FIELDS, "after artifact")
    for field in ("bundle_id", "timeframe", "replay_scope", "generated_at"):
        _require_type(artifact, field, str, "after artifact")
    for field in ("run_id", "pipeline_version"):
        _require_type(artifact, field, str, "after artifact", optional=True)
    _require_type(artifact, "provenance", dict, "after artifact")
    rows = artifact["rows"]
    if not isinstance(rows, list):
        raise ArtifactError("after artifact 的 rows 必須是陣列")
    for row in rows:
        if not isinstance(row, dict):
            raise ArtifactError("after artifact 的 rows[] 每一列都必須是 object")
    keys = [row_key(row) for row in rows]
    assert_unique_keys(keys, "after artifact")
    validate_candidate_flags(rows, "after artifact")
    return keys


def assert_matches_bundle(artifact: dict[str, Any], manifest: dict[str, Any], label: str) -> None:
    """artifact 宣稱的身分必須與 bundle 的 manifest 一致。

    ⚠️ 只比 `bundle_id` 不夠：`timeframe`／`replay_scope` 會被 Stage 2 當成事實寫進
    comparison artifact，兩份不一致等於「算的」和「說的」不是同一件事。
    """
    for field in ("bundle_id", "timeframe", "replay_scope"):
        if field not in artifact:
            continue
        if artifact[field] != manifest.get(field):
            raise ArtifactError(
                f"{label}.{field}={artifact[field]!r} 與 bundle manifest 的 "
                f"{manifest.get(field)!r} 不一致"
            )


def validate_cohort_manifest(artifact: dict[str, Any]) -> list[tuple[str, str, str]]:
    """cohort manifest 的完整結構檢查，回傳 key 列表。"""
    _require_fields(artifact, _COHORT_FIELDS, "cohort manifest")
    for field in ("bundle_id", "after_artifact_sha256", "generated_at"):
        _require_type(artifact, field, str, "cohort manifest")
    _require_type(artifact, "provenance", dict, "cohort manifest")
    digest = artifact["after_artifact_sha256"]
    if len(digest) != 64 or digest != digest.lower() or not all(c in "0123456789abcdef" for c in digest):
        raise ArtifactError(f"cohort manifest 的 after_artifact_sha256 不是 64 字元十六進位小寫：{digest!r}")
    raw_keys = artifact["keys"]
    if not isinstance(raw_keys, list):
        raise ArtifactError("cohort manifest 的 keys 必須是陣列")
    keys: list[tuple[str, str, str]] = []
    for item in raw_keys:
        if not isinstance(item, list) or len(item) != 3 or any(not isinstance(v, str) for v in item):
            raise ArtifactError(
                f"cohort manifest 的 key 必須是 [symbol, timeframe, as_of] 三個字串：{item!r}"
            )
        keys.append((item[0], item[1], item[2]))
    assert_unique_keys(keys, "cohort manifest")
    return keys


def candidate_keys(rows: Iterable[dict[str, Any]]) -> list[tuple[str, str, str]]:
    return [row_key(row) for row in rows if row[CANDIDATE_FIELD] is True]


# ── 產生 ────────────────────────────────────────────────────────────────────

def _envelope(kind: str, **fields: Any) -> dict[str, Any]:
    return {"schema_version": ARTIFACT_SCHEMA_VERSION, "kind": kind, **fields}


def build_after_artifact(*, bundle_id: str, timeframe: str, replay_scope: str,
                         run_id: str | None, pipeline_version: str | None,
                         generated_at: str, provenance: dict[str, Any],
                         rows: list[dict[str, Any]]) -> dict[str, Any]:
    """⛔ **不截斷**：I-074 要「全部候選逐列資料都能重算分支」。"""
    return _envelope(
        AFTER_KIND, bundle_id=bundle_id, timeframe=timeframe, replay_scope=replay_scope,
        run_id=run_id, pipeline_version=pipeline_version, generated_at=generated_at,
        provenance=provenance, rows=rows,
    )


def build_cohort_manifest(*, bundle_id: str, after_artifact_sha256: str,
                          keys: Sequence[tuple[str, str, str]], generated_at: str,
                          provenance: dict[str, Any]) -> dict[str, Any]:
    return _envelope(
        COHORT_KIND, bundle_id=bundle_id, after_artifact_sha256=after_artifact_sha256,
        generated_at=generated_at, provenance=provenance,
        keys=[list(key) for key in sorted(keys)],
    )


def build_comparison_artifact(*, bundle_id: str, before_ref: str,
                              after_artifact_sha256: str, generated_at: str,
                              provenance: dict[str, Any],
                              rows: list[dict[str, Any]]) -> dict[str, Any]:
    return _envelope(
        COMPARISON_KIND, bundle_id=bundle_id, before_ref=before_ref,
        after_artifact_sha256=after_artifact_sha256, generated_at=generated_at,
        provenance=provenance, rows=rows,
    )


def compare_rows(before: dict[str, Any], after: dict[str, Any]) -> dict[str, Any]:
    """逐列比較。⛔ 不挑欄位比——兩邊都是同一條決定性流程的輸出，全欄位比才不會漏。"""
    fields = sorted(set(before) | set(after))
    differences = [f for f in fields if before.get(f) != after.get(f)]
    key = row_key(after)
    return {
        "symbol": key[0], "timeframe": key[1], "as_of": key[2],
        "differences": differences,
        "before": before,
        "after": after,
    }


def build_report(*, bundle_id: str, before_ref: str, comparison_sha256: str,
                 generated_at: str, rows: list[dict[str, Any]],
                 report_max_rows: int | None) -> dict[str, Any]:
    """人讀報告。⚠️ `report_max_rows` **只截斷這裡**，⛔ 不影響實際運算範圍與比較 artifact。"""
    shown = rows if report_max_rows is None else rows[:report_max_rows]
    difference_counts: dict[str, int] = {}
    for row in rows:
        for field in row["differences"]:
            difference_counts[field] = difference_counts.get(field, 0) + 1
    return _envelope(
        REPORT_KIND, bundle_id=bundle_id, before_ref=before_ref,
        comparison_artifact_sha256=comparison_sha256, generated_at=generated_at,
        candidate_rows=len(rows), rows_shown=len(shown),
        difference_field_counts=dict(sorted(difference_counts.items())),
        rows=shown,
    )


# ── 載入 ────────────────────────────────────────────────────────────────────

def load_artifact(path: str | Path, kind: str) -> tuple[dict[str, Any], str]:
    """回傳 `(artifact, sha256)`。未知 `schema_version` 或 `kind` 不符即中止。"""
    import json

    path = Path(path)
    try:
        raw = path.read_bytes()
    except OSError as exc:
        raise ArtifactError(f"讀不到 artifact {path}：{exc}") from exc
    try:
        data = json.loads(raw.decode("utf-8"))
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise ArtifactError(f"{path.name} 不是合法 JSON：{exc}") from exc
    if not isinstance(data, dict):
        raise ArtifactError(f"{path.name} 必須是 object")
    version = data.get("schema_version")
    if type(version) is not int or version != ARTIFACT_SCHEMA_VERSION:
        raise ArtifactError(
            f"未知的 {path.name} schema_version={version!r}（本實作只支援 {ARTIFACT_SCHEMA_VERSION}）"
        )
    if data.get("kind") != kind:
        raise ArtifactError(f"{path.name} 的 kind={data.get('kind')!r}，預期 {kind!r}")
    return data, sha256_hex(raw)


# ── 原子發布 ────────────────────────────────────────────────────────────────

def prepare_output_dir(output_dir: str | Path) -> Path:
    """`--output-dir` 必須不存在或為空。"""
    path = Path(output_dir)
    if path.exists():
        if not path.is_dir():
            raise ArtifactError(f"--output-dir 不是目錄：{path}")
        if any(path.iterdir()):
            raise ArtifactError(f"--output-dir 非空：{path}——⛔ 不覆蓋既有輸出")
    else:
        path.mkdir(parents=True)
    return path


def write_atomic(directory: Path, name: str, payload: bytes) -> str:
    """同目錄 temp ＋ `os.replace`。回傳內容的 SHA-256。

    ⚠️ 單檔用 `os.replace` 是安全的——它是原子的；**目錄**才不能這樣做（見 publish 模組）。
    """
    tmp = directory / f".{name}.{secrets.token_hex(6)}.tmp"
    try:
        tmp.write_bytes(payload)
        fsync_file(tmp)
        os.replace(tmp, directory / name)
        fsync_dir(directory)
    finally:
        if tmp.exists():
            tmp.unlink()
    return sha256_hex(payload)


def publish_artifacts(directory: Path, artifacts: Sequence[tuple[str, dict[str, Any]]]) -> dict[str, str]:
    """依序寫出；**最後一個是「指標」檔**（Stage 1 的 cohort manifest／Stage 2 的 report）。

    ⚠️ 順序就是契約：指標檔存在即代表它指向的證據已完整落地。
    """
    hashes: dict[str, str] = {}
    for name, data in artifacts:
        hashes[name] = write_atomic(directory, name, canonical_json_bytes(data))
    return hashes
