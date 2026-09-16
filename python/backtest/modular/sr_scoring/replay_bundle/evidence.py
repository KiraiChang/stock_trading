"""evidence finalizer：把 operational 產物封存成**一次發布的整包證據**（I-074 Stage 1）。

兩層分明：

* **operational artifact**——`.json`（canonical），由各自的行程寫在 **repo 外**的工作目錄；
* **archived evidence**——`.json.gz`，⚠️ **只有 finalizer 產生**，進版控。

⚠️ **整包原子發布**：⛔ 不能一邊驗一邊往正式 root 寫——中途失敗會留下**半包證據**，
而下一次又會被「子目錄必須全新空白」擋住。所以在 sibling staging 建完整 tree、
全圖驗證通過後，用 `rename_noreplace()` **一次**把整個 root 換上去：正式路徑由
「**不存在**」直接變成「**完整**」。

⚠️ **rename 成功 ⛔ 不等於落盤**：commit point 之後 parent fsync 失敗時，正式 root **已經
存在且有效**——那時⛔ 不得刪除，要回 `EXIT_DURABILITY_UNCONFIRMED`，並由
`--recover-durability` 修復（⛔ 不重產任何證據）。
"""
from __future__ import annotations

import secrets
from pathlib import Path
from typing import Any, Mapping

from .artifacts import (
    AFTER_KIND,
    ARTIFACT_SCHEMA_VERSION,
    COHORT_KIND,
    ArtifactError,
    assert_same_keys,
    candidate_keys,
    validate_cohort_manifest,
    load_canonical_evidence_artifact,
    validate_after_artifact,
    validate_diagnostics,
    validate_replay_errors,
)
from .canonical import canonical_gzip_bytes, canonical_json_bytes, sha256_hex
from .crossday import CROSSDAY_KIND, validate_crossday
from .probe import (
    PROBE_COMPLETION_KIND,
    PROBE_COMPUTATION_KIND,
    PROBE_MEASUREMENT_KIND,
    validate_probe_completion,
    validate_probe_computation,
    validate_probe_measurement,
)
from .provenance import validate_provenance
from .publish import (
    DurabilityUnconfirmed,
    fsync_dir,
    fsync_file,
    probe_no_clobber,
    remove_tree,
    rename_noreplace,
)
from .run_identity import RUN_IDENTITY_KIND, load_run_identity, validate_run_identity

EVIDENCE_MANIFEST_NAME = "evidence_manifest.json"
EVIDENCE_MANIFEST_KIND = "sr_zone_evidence_manifest"

# ⚠️ **封閉的 archive 佈局**：9 個 `.json.gz` ＋ root 的 manifest ＝ 10 檔。
# ⛔ 多一個未知檔案即中止；manifest **排除自身**（它是索引，⛔ 不壓縮、要能直接讀）。
ARCHIVE_LAYOUT: tuple[tuple[str, str], ...] = (
    ("d/after_artifact.json.gz", AFTER_KIND),
    ("d/cohort_manifest.json.gz", COHORT_KIND),
    ("d1/after_artifact.json.gz", AFTER_KIND),
    ("d1/cohort_manifest.json.gz", COHORT_KIND),
    ("crossday/crossday_artifact.json.gz", CROSSDAY_KIND),
    ("probe/capacity_probe_computation.json.gz", PROBE_COMPUTATION_KIND),
    ("probe/capacity_probe_measurement.json.gz", PROBE_MEASUREMENT_KIND),
    ("probe/capacity_probe.json.gz", PROBE_COMPLETION_KIND),
    ("identity/run_identity.json.gz", RUN_IDENTITY_KIND),
)
ARCHIVE_PATHS = tuple(path for path, _ in ARCHIVE_LAYOUT)

# ⚠️ **明確列舉每一份 provenance 的位置**，⛔ 不用 `.get("provenance") or …` 推測——
# crossday 用的是 `comparator_provenance`，通用推測會**整份漏掉它**。
# ⚠️ identity 沒有 provenance（它是身分宣告，⛔ 不是執行紀錄）。
PROVENANCE_LOCATIONS: tuple[tuple[str, str, str], ...] = (
    ("d/after_artifact.json.gz", "provenance", "stage1"),
    ("d/cohort_manifest.json.gz", "provenance", "stage1"),
    ("d1/after_artifact.json.gz", "provenance", "stage1"),
    ("d1/cohort_manifest.json.gz", "provenance", "stage1"),
    ("crossday/crossday_artifact.json.gz", "comparator_provenance", "comparator"),
    ("probe/capacity_probe_computation.json.gz", "provenance", "stage1"),
    ("probe/capacity_probe_measurement.json.gz", "provenance", "stage1"),
    ("probe/capacity_probe.json.gz", "provenance", "stage1"),
)

_MANIFEST_FIELDS = frozenset({
    "schema_version", "kind", "bundle_id", "expected_image_id",
    "files", "generated_at", "finalizer_provenance",
})
_FILE_ENTRY_FIELDS = frozenset({"artifact_sha256", "stored_sha256", "stored_bytes"})


def _is_hex64(value: object) -> bool:
    return (
        isinstance(value, str) and len(value) == 64 and value == value.lower()
        and all(c in "0123456789abcdef" for c in value)
    )


def _is_image_id(value: object) -> bool:
    return isinstance(value, str) and value.startswith("sha256:") and _is_hex64(value[7:])


# ── manifest ────────────────────────────────────────────────────────────────

def build_evidence_manifest(
    *, bundle_id: str, expected_image_id: str, files: Mapping[str, Mapping[str, Any]],
    generated_at: str, finalizer_provenance: dict[str, Any],
) -> dict[str, Any]:
    return {
        "schema_version": ARTIFACT_SCHEMA_VERSION,
        "kind": EVIDENCE_MANIFEST_KIND,
        "bundle_id": bundle_id,
        "expected_image_id": expected_image_id,
        "files": {path: dict(files[path]) for path in sorted(files)},
        "generated_at": generated_at,
        "finalizer_provenance": finalizer_provenance,
    }


def validate_evidence_manifest(manifest: object) -> None:
    """封閉 schema ＋ 精確型別。⚠️ `files` 的 key **必須恰好是 9 個 archive 相對路徑**。"""
    if not isinstance(manifest, dict):
        raise ArtifactError("evidence manifest 必須是 object")
    actual = set(manifest)
    if actual != set(_MANIFEST_FIELDS):
        raise ArtifactError(
            "evidence manifest 欄位集合不符："
            f"多={sorted(actual - _MANIFEST_FIELDS)}、缺={sorted(_MANIFEST_FIELDS - actual)}"
        )
    version = manifest["schema_version"]
    if type(version) is not int or version != ARTIFACT_SCHEMA_VERSION:
        raise ArtifactError(f"未知的 evidence manifest schema_version={version!r}")
    if manifest["kind"] != EVIDENCE_MANIFEST_KIND:
        raise ArtifactError(f"evidence manifest 的 kind={manifest['kind']!r}")
    for field in ("bundle_id", "generated_at"):
        if not isinstance(manifest[field], str) or not manifest[field]:
            raise ArtifactError(f"evidence manifest 的 {field} 必須是非空字串")
    # ⚠️ `generated_at` 要**含時區**——⛔ 只驗「非空字串」的話，裸日期字串也會過。
    from datetime import datetime as _dt

    try:
        _parsed = _dt.fromisoformat(manifest["generated_at"])
    except ValueError as exc:
        raise ArtifactError(f"evidence manifest 的 generated_at 不是合法 ISO-8601：{exc}") from exc
    if _parsed.tzinfo is None:
        raise ArtifactError("evidence manifest 的 generated_at ⛔ 必須含時區")
    if not _is_image_id(manifest["expected_image_id"]):
        raise ArtifactError(
            f"expected_image_id 必須是 sha256: ＋ 64 字元小寫 hex：{manifest['expected_image_id']!r}"
        )
    files = manifest["files"]
    if not isinstance(files, dict):
        raise ArtifactError("evidence manifest 的 files 必須是 object")
    if set(files) != set(ARCHIVE_PATHS):
        raise ArtifactError(
            "files 的 key 必須恰好是 9 個 archive 相對路徑："
            f"多={sorted(set(files) - set(ARCHIVE_PATHS))}、"
            f"缺={sorted(set(ARCHIVE_PATHS) - set(files))}"
        )
    for path, entry in files.items():
        if not isinstance(entry, dict) or set(entry) != set(_FILE_ENTRY_FIELDS):
            raise ArtifactError(f"files[{path}] 的欄位集合必須是 {sorted(_FILE_ENTRY_FIELDS)}")
        for field in ("artifact_sha256", "stored_sha256"):
            if not _is_hex64(entry[field]):
                raise ArtifactError(f"files[{path}].{field} 必須是 64 字元小寫 hex")
        size = entry["stored_bytes"]
        if type(size) is not int or size <= 0:
            raise ArtifactError(f"files[{path}].stored_bytes 必須是正整數：{size!r}")
    validate_provenance(manifest["finalizer_provenance"], role="finalizer")
    # ⚠️ manifest 的 `finalizer_provenance` ⛔ 不在那 9 份 archive 裡，所以它的 image
    # 必須在這裡驗——⛔ 少了這行，finalizer 可以跑在另一個 image 上。
    if manifest["finalizer_provenance"]["image_digest"] != manifest["expected_image_id"]:
        raise ArtifactError(
            "evidence manifest 的 finalizer_provenance.image_digest 與 expected_image_id 不符"
        )


# ── 全圖驗證 ────────────────────────────────────────────────────────────────

def verify_evidence_graph(loaded: Mapping[str, Any], *, expected_image_id: str) -> str:
    """跨檔案的關聯驗證。回傳 crossday 的 `outcome`（recovery 要用它還原終端結果）。

    ⚠️ ⛔ 只驗「檔案集合與 SHA」是不夠的——那擋不住「各檔都合法、彼此卻對不起來」。
    """
    identity = loaded["identity/run_identity.json.gz"].parsed
    validate_run_identity(identity)
    bundle_id = identity["bundle_id"]

    # ① 兩趟的 after／cohort：schema ＋ **diagnostics**（⚠️ `validate_after_artifact()` 不驗它）
    for side in ("d", "d1"):
        after = loaded[f"{side}/after_artifact.json.gz"].parsed
        validate_after_artifact(after)
        validate_replay_errors(after["rows"], f"{side} after artifact")
        validate_diagnostics(after["rows"], f"{side} after artifact", side="after")
        validate_provenance(after["provenance"], role="stage1")
        # ② cohort：**完整 validator**（⛔ 不只比 SHA——格式損壞或 keys 不完整的
        #    cohort 照樣會被封存並通過 recovery）。
        cohort = loaded[f"{side}/cohort_manifest.json.gz"].parsed
        cohort_keys = validate_cohort_manifest(cohort)
        validate_provenance(cohort["provenance"], role="stage1")
        expected = sha256_hex(canonical_json_bytes(after))
        if cohort.get("after_artifact_sha256") != expected:
            raise ArtifactError(
                f"{side} 的 cohort manifest 記的 after SHA 與實際的 after artifact 不符"
            )
        # ③ cohort 的 keys 必須恰好是那份 after 的候選列（⛔ 不重算 predicate，只消費）。
        assert_same_keys(
            cohort_keys, candidate_keys(after["rows"]),
            f"{side} 的 cohort manifest vs after 的候選列",
        )

    # ③ crossday 以**這兩份實際 after** 重跑 validator
    crossday = loaded["crossday/crossday_artifact.json.gz"].parsed
    d_after = loaded["d/after_artifact.json.gz"].parsed
    d1_after = loaded["d1/after_artifact.json.gz"].parsed
    validate_crossday(
        crossday, d=d_after, d1=d1_after,
        d_sha=sha256_hex(canonical_json_bytes(d_after)),
        d1_sha=sha256_hex(canonical_json_bytes(d1_after)),
        comparator_provenance=crossday["comparator_provenance"],
    )

    # ④ probe 三份**完整** validator（⛔ 不只比對 completion 的兩個 SHA）
    computation = loaded["probe/capacity_probe_computation.json.gz"].parsed
    measurement = loaded["probe/capacity_probe_measurement.json.gz"].parsed
    completion = loaded["probe/capacity_probe.json.gz"].parsed
    validate_probe_computation(computation)
    validate_probe_measurement(measurement)
    validate_probe_completion(
        completion,
        computation_sha256=sha256_hex(canonical_json_bytes(computation)),
        measurement_sha256=sha256_hex(canonical_json_bytes(measurement)),
    )
    # ⚠️ 三份 provenance 必須完全相同——它們是**同一個 runner invocation 的外層產物**，
    # ⛔ 不是各自虛構一份執行身分。
    provs = [computation["provenance"], measurement["provenance"], completion["provenance"]]
    if any(prov != provs[0] for prov in provs[1:]):
        raise ArtifactError("probe 三份的 provenance 不完全相同——⛔ 它們必須來自同一次執行")

    # ⑤ 所有檔案同一個 bundle_id
    for path, entry in loaded.items():
        actual = entry.parsed.get("bundle_id")
        if actual != bundle_id:
            raise ArtifactError(f"{path} 的 bundle_id={actual!r} 與 identity 的 {bundle_id!r} 不符")

    # ⑥ 所有 provenance 的 image_digest 等於預期的 image ID
    if identity["expected_image_id"] != expected_image_id:
        raise ArtifactError(
            f"identity 的 expected_image_id={identity['expected_image_id']} 與本次的 "
            f"{expected_image_id} 不符"
        )
    # ⚠️ **逐一列舉**，⛔ 不用通用推測——少掉 crossday 的 `comparator_provenance` 的話，
    # comparator 就可以跑在另一個 image 上而沒有任何東西會報錯。
    for path, field, role in PROVENANCE_LOCATIONS:
        prov = loaded[path].parsed.get(field)
        validate_provenance(prov, role=role)
        if prov["image_digest"] != expected_image_id:
            raise ArtifactError(
                f"{path} 的 {field}.image_digest={prov['image_digest']} 與 "
                f"expected_image_id={expected_image_id} 不符"
            )
    return crossday["outcome"]


# ── 發布 ────────────────────────────────────────────────────────────────────

# recovery 要逐欄比對的執行身分。⛔ 不比 `argv`（模式天生不同）、
# `project_modules_sha256`（recovery ⛔ 不讀 operational inputs，載入集合本來就較少）、
# `python_version`／`pip_freeze_sha256`（由 image 決定，`image_digest` 已涵蓋）。
RECOVERY_IDENTITY_FIELDS = (
    "image_digest", "base_commit", "tooling_patch_sha256", "runner_sha256", "source_root",
    # ⚠️ `runtime_settings` **要比**：它⛔ 不是由 image 決定的——`config.py` 的值全是
    # `os.getenv(...) or config.yaml`，環境變數可覆寫。normal 與 recovery 本來就該在
    # 相同的封閉環境跑。
    "runtime_settings",
)


def _load_all(root: Path, *, sources: Mapping[str, Path] | None = None) -> dict[str, Any]:
    """載入 9 個 archive（或 operational 來源）。⚠️ 每個檔案**只讀一次**。"""
    loaded: dict[str, Any] = {}
    for rel, kind in ARCHIVE_LAYOUT:
        path = sources[rel] if sources is not None else root / rel
        loaded[rel] = load_canonical_evidence_artifact(path, kind)
    return loaded


def finalize_evidence(
    *,
    evidence_root: Path,
    sources: Mapping[str, Path],
    expected_image_id: str,
    provenance_factory,
    generated_at: str,
    external_identity: dict[str, Any] | None = None,
) -> str:
    """把 operational 產物封存成整包 evidence。回傳 crossday 的 `outcome`。

    ⚠️ **兩階段**：階段 A 讀入並完成 9 個 archive payload 的全部驗證（此時所有 project
    import 都已發生）；階段 B **才**建 `finalizer_provenance` 與 manifest。
    ⛔ 階段 B 之後⛔ 不得再產生任何新的 project import——由重算 mapping 的斷言守住。
    """
    evidence_root = Path(evidence_root)
    if evidence_root.exists():
        raise ArtifactError(
            f"evidence root 已存在：{evidence_root}——⛔ 不覆蓋。"
            "（durability 未確認時請用 --recover-durability，⛔ 不是重新 finalize）"
        )
    parent = evidence_root.parent
    parent.mkdir(parents=True, exist_ok=True)
    # ⚠️ 正式發布前先實測 no-clobber rename 的**目錄**語意（⛔ 檔案 rename 過不代表目錄也過）。
    probe_no_clobber(parent)

    if set(sources) != set(ARCHIVE_PATHS):
        raise ArtifactError(
            f"sources 的 key 必須恰好是 9 個 archive 相對路徑："
            f"多={sorted(set(sources) - set(ARCHIVE_PATHS))}、"
            f"缺={sorted(set(ARCHIVE_PATHS) - set(sources))}"
        )

    staging = parent / f".{evidence_root.name}.staging-{secrets.token_hex(8)}"
    try:
        # ── 階段 A：讀入 ＋ 全部驗證（所有 project import 在此發生）────────
        loaded = _load_all(evidence_root, sources=sources)
        # ⚠️ **用這一次載入的那份比對**，⛔ 不預先另讀一次：兩次讀取之間來源若被替換，
        # 實際封存的就不是先前與外部 identity 比過的那一份。
        if external_identity is not None:
            archived_identity = loaded["identity/run_identity.json.gz"].parsed
            if external_identity != archived_identity:
                raise ArtifactError(
                    "repo 外的 run identity 與要歸檔的那份⛔ 不一致——"
                    f"外部={external_identity!r}、archived={archived_identity!r}"
                )
        outcome = verify_evidence_graph(loaded, expected_image_id=expected_image_id)

        # ── 階段 B：才建 provenance 與 manifest ────────────────────────────
        finalizer_provenance = provenance_factory()
        validate_provenance(finalizer_provenance, role="finalizer")

        staging.mkdir(parents=True)
        files: dict[str, dict[str, Any]] = {}
        for rel, _kind in ARCHIVE_LAYOUT:
            payload = canonical_json_bytes(loaded[rel].parsed)
            blob = canonical_gzip_bytes(payload)
            target = staging / rel
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(blob)
            # ⚠️ 三項 metadata 都來自**同一次**寫入的 bytes。
            files[rel] = {
                "artifact_sha256": sha256_hex(payload),
                "stored_sha256": sha256_hex(blob),
                "stored_bytes": len(blob),
            }

        manifest = build_evidence_manifest(
            bundle_id=loaded["identity/run_identity.json.gz"].parsed["bundle_id"],
            expected_image_id=expected_image_id,
            files=files,
            generated_at=generated_at,
            finalizer_provenance=finalizer_provenance,
        )
        validate_evidence_manifest(manifest)
        (staging / EVIDENCE_MANIFEST_NAME).write_bytes(canonical_json_bytes(manifest))

        # ⚠️ **⛔ 不得新增 project import 的實際守門**：重算一次，與 manifest 裡的比對。
        if provenance_factory()["project_modules_sha256"] != \
                finalizer_provenance["project_modules_sha256"]:
            raise ArtifactError(
                "階段 B 之後又載入了新的專案模組——⛔ manifest 裡的 provenance 已經記不全了"
            )

        _fsync_tree(staging)
        rename_noreplace(staging, evidence_root)
    except BaseException:
        # ⛔ rename 之前的任何失敗：只清 staging，正式路徑仍不存在。
        remove_tree(staging)
        raise

    _fsync_parent(evidence_root, published=True)
    return outcome


def _fsync_tree(root: Path) -> None:
    """逐一 fsync 所有檔案與巢狀目錄。"""
    for path in sorted(root.rglob("*")):
        if path.is_file():
            fsync_file(path)
    for path in sorted(root.rglob("*"), reverse=True):
        if path.is_dir():
            fsync_dir(path)
    fsync_dir(root)


def _fsync_parent(evidence_root: Path, *, published: bool) -> None:
    """⚠️ **commit point 之後**：parent fsync 失敗⛔ 不刪除，回報 durability 未確認。"""
    try:
        fsync_dir(evidence_root.parent)
    except OSError as exc:
        raise DurabilityUnconfirmed(
            f"evidence {evidence_root} "
            + ("已發布" if published else "既有那份有效")
            + f"，但 {evidence_root.parent} 的 fsync 失敗（{exc}）——durability 未確認。"
            "⛔ 不刪除、不重產：用 --recover-durability 重新驗證並 fsync。",
            published=published,
        ) from exc


def recover_durability(*, evidence_root: Path, provenance: dict[str, Any]) -> str:
    """⚠️ **只重新驗證既有 root 並重新 fsync**，⛔ 不讀 operational inputs、⛔ 不重新壓縮。

    回傳 crossday 的 `outcome`——⚠️ **⛔ 不得固定回成功**：
    「crossday mismatch（5）→ parent fsync 失敗（3 蓋掉 5）→ recovery 回 0」這條鏈路會讓
    **mismatch 的 5 永遠消失**，機器端會把它讀成「整組 Stage 1 成功匹配」。
    """
    evidence_root = Path(evidence_root)
    if not evidence_root.is_dir():
        raise ArtifactError(f"--recover-durability 要求 evidence root 已存在：{evidence_root}")

    manifest_path = evidence_root / EVIDENCE_MANIFEST_NAME
    manifest_load = load_canonical_evidence_artifact(manifest_path, EVIDENCE_MANIFEST_KIND)
    manifest = manifest_load.parsed
    validate_evidence_manifest(manifest)

    # ⛔ 禁止未知 evidence：實際檔案集合必須恰好是 9 個 archive ＋ manifest。
    actual = {
        str(p.relative_to(evidence_root)) for p in evidence_root.rglob("*") if p.is_file()
    }
    expected = set(ARCHIVE_PATHS) | {EVIDENCE_MANIFEST_NAME}
    if actual != expected:
        raise ArtifactError(
            f"evidence root 的檔案集合不符：多={sorted(actual - expected)}、"
            f"缺={sorted(expected - actual)}"
        )

    loaded = _load_all(evidence_root)
    # ⚠️ **manifest 的每一項 metadata 都要從實際 archive 重算**——只驗型別的話，
    # 一份索引值錯誤的 manifest 照樣通過，而 recovery **完全依賴既有 root**。
    for rel in ARCHIVE_PATHS:
        entry, actual_load = manifest["files"][rel], loaded[rel]
        if entry["artifact_sha256"] != actual_load.artifact_sha256 \
                or entry["stored_sha256"] != actual_load.stored_sha256 \
                or entry["stored_bytes"] != actual_load.stored_bytes:
            raise ArtifactError(f"manifest 記的 {rel} metadata 與實際檔案不符")

    # ⚠️ **manifest 的 `bundle_id` 要綁 archived identity**——⛔ 少了這道，單獨竄改
    # canonical manifest 的 `bundle_id` 仍可能 recovery 成功。
    archived_identity = loaded["identity/run_identity.json.gz"].parsed
    if manifest["bundle_id"] != archived_identity["bundle_id"]:
        raise ArtifactError(
            f"manifest 的 bundle_id={manifest['bundle_id']!r} 與 archived identity 的 "
            f"{archived_identity['bundle_id']!r} 不符"
        )

    outcome = verify_evidence_graph(loaded, expected_image_id=manifest["expected_image_id"])

    # ⚠️ 本次執行身分必須與 archived `finalizer_provenance` 相同——**在 fsync 之前**比。
    archived = manifest["finalizer_provenance"]
    validate_provenance(provenance, role="finalizer")
    for field in RECOVERY_IDENTITY_FIELDS:
        if provenance.get(field) != archived.get(field):
            raise ArtifactError(
                f"recovery 的執行身分與 archived finalizer_provenance 的 {field} 不符——"
                "⛔ 在 fsync 之前中止"
            )

    _fsync_tree(evidence_root)
    _fsync_parent(evidence_root, published=False)
    return outcome


# ── CLI ─────────────────────────────────────────────────────────────────────

class EvidenceUsageError(ValueError):
    """CLI 用法錯誤。"""


# ⚠️ 注入參數只能由官方腳本給。⛔ **不共用 `REPLAY_INJECTED_ARGS`** 的前綴清單——
# 那會把既有合法的 `--run-id` 當成 `--run-identity` 的縮寫而拒絕（實測過）。
EVIDENCE_INJECTED_ARGS = (
    "--run-identity", "--image-digest", "--base-commit",
    "--tooling-patch-sha256", "--source-root", "--runner-sha256",
)


def assert_evidence_arg_ownership(argv) -> None:
    for injected in EVIDENCE_INJECTED_ARGS:
        count = sum(1 for a in argv if a == injected or a.startswith(injected + "="))
        if count > 1:
            raise EvidenceUsageError(
                f"{injected} 出現 {count} 次——它只能由官方腳本注入。⛔ 不靜默採用最後一個。"
            )


def build_evidence_parser():
    import argparse

    parser = argparse.ArgumentParser(prog="evidence", allow_abbrev=False)
    parser.add_argument("--evidence-root", required=True)
    parser.add_argument("--source", action="append", default=[],
                        help="`<archive 相對路徑>=<operational 檔案路徑>`，normal 模式需 9 個")
    parser.add_argument("--run-identity", default=None)
    parser.add_argument("--recover-durability", action="store_true")
    parser.add_argument("--image-digest", required=True)
    parser.add_argument("--base-commit", required=True)
    parser.add_argument("--tooling-patch-sha256", required=True)
    parser.add_argument("--source-root", required=True)
    parser.add_argument("--runner-sha256", required=True)
    return parser


def run_evidence(argv) -> dict[str, Any]:
    """normal finalization 或 `--recover-durability`。回傳含 `outcome` 的摘要。"""
    from datetime import datetime, timezone

    from .provenance import build_provenance

    assert_evidence_arg_ownership(argv)
    args = build_evidence_parser().parse_args(list(argv))

    # ⚠️ **CLI matrix 的模式規則**：normal 要求 identity 與 9 份 source；
    # recovery ⛔ 兩者都禁止（它只依賴既有 root）。
    if args.recover_durability:
        if args.run_identity or args.source:
            raise EvidenceUsageError(
                "--recover-durability ⛔ 不接受 --run-identity 或 --source："
                "它只依賴既有的 evidence root。"
            )
    else:
        if not args.run_identity:
            raise EvidenceUsageError("normal finalization 需要 --run-identity")
        if len(args.source) != len(ARCHIVE_PATHS):
            raise EvidenceUsageError(
                f"normal finalization 需要 {len(ARCHIVE_PATHS)} 份 --source，實際 {len(args.source)}"
            )

    # ⚠️ **階段 A 明確載入 `config`**（v25 中 5）：`build_provenance()` 的 dict literal 裡
    # `project_module_hashes()` 排在 `runtime_settings()` **之前**求值，而後者在
    # `config_module is None` 時才 lazy-import `config`——⚠️ 而 `config` 正是專案模組
    # （`PROJECT_MODULE_NAMES`）。⛔ 靠「父 package 恰好會間接載入它」是 incidental，
    # import topology 一變就會漏記，然後被階段 B 的重算守門擋下、整個 finalizer 發不出去。
    import config as _config_module

    def _provenance_factory():
        # ⚠️ **階段 B 才呼叫**：所有 project import 都已在階段 A 發生。
        return build_provenance(
            config_module=_config_module,
            source_root=args.source_root,
            image_digest=args.image_digest,
            base_commit=args.base_commit,
            tooling_patch_sha256=args.tooling_patch_sha256,
            runner_sha256=args.runner_sha256,
            argv=list(argv),
        )

    root = Path(args.evidence_root)
    if args.recover_durability:
        outcome = recover_durability(evidence_root=root, provenance=_provenance_factory())
        return {"mode": "recover_durability", "outcome": outcome, "evidence_root": str(root)}

    sources: dict[str, Path] = {}
    for item in args.source:
        rel, _, path = item.partition("=")
        if not rel or not path:
            raise EvidenceUsageError(f"--source 格式必須是 <rel>=<path>：{item!r}")
        sources[rel] = Path(path)
    # ⚠️ **外部協調檔與 archived copy 必須逐欄相同**（v25 十）——比對點在
    # `finalize_evidence()` 內、用它**實際要封存的那一份**，⛔ 不在這裡預先另讀。
    external = load_run_identity(args.run_identity)
    if "identity/run_identity.json.gz" not in sources:
        raise EvidenceUsageError("sources 缺少 identity/run_identity.json.gz")

    outcome = finalize_evidence(
        evidence_root=root,
        sources=sources,
        external_identity=external,
        expected_image_id=args.image_digest,
        provenance_factory=_provenance_factory,
        generated_at=datetime.now(timezone.utc).isoformat(),
    )
    return {"mode": "finalize", "outcome": outcome, "evidence_root": str(root)}


def main(argv=None) -> int:
    import json
    import sys

    from .publish import EXIT_ABORT, EXIT_DURABILITY_UNCONFIRMED

    from .crossday import OUTCOME_MATCH
    from .publish import EXIT_CROSSDAY_MISMATCH

    argv = list(sys.argv[1:] if argv is None else argv)
    try:
        result = run_evidence(argv)
    except DurabilityUnconfirmed as exc:
        # ⚠️ **證據已發布但落盤未確認**——⛔ 不刪除，回專屬碼 3，由
        # `--recover-durability` 修復（⛔ 不重產任何證據）。
        print(f"ERROR: {exc}", file=sys.stderr)
        return EXIT_DURABILITY_UNCONFIRMED
    except (EvidenceUsageError, ValueError, OSError) as exc:
        print(f"ERROR: {type(exc).__name__}: {exc}", file=sys.stderr)
        return EXIT_ABORT
    print(json.dumps(result, ensure_ascii=False, sort_keys=True))
    # ⚠️ **recovery ⛔ 不得固定回 0**：鏈路「crossday mismatch（5）→ parent fsync 失敗
    # （3 蓋掉 5）→ recovery 回 0」會讓 **mismatch 的 5 永遠消失**，機器端會把它讀成
    # 「整組 Stage 1 成功匹配」——那正好把這次驗證要回答的問題答反。
    # ⚠️ normal finalization 仍回 0：那時 crossday 的 rc 還在 orchestrator 手上。
    if result["mode"] == "recover_durability" and result["outcome"] != OUTCOME_MATCH:
        return EXIT_CROSSDAY_MISMATCH
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
