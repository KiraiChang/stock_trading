"""I-074 Stage 2 的環境見證：新 image 上的 after' 與 Stage 1 D+1 是否等價，以及 `envcheck/` 小 archive。

⚠️ **為什麼需要它**：Stage 1 釘住的 image（`sha256:d66030dca485…`）已不在本機。使用者裁決
**兩側都在新 image 重跑**：先在新 image 上以原始 `e1cbbbd` 跑一趟 after'，逐列與已封存的 D+1
比對——**逐位元相同**才證明兩個 image 對這份 bundle 等價，才能繼續拿封存的 D+1 當 after 側。
規格見 issue.md I-074 Stage 2 計畫書「二、⑤」與 ③ evidence contract「三之三」（⚠️ 現行版）。

⚠️ **after' 只是見證**：⛔ 不是新的正式 Stage 1 scan，⛔ 結果不得取代 D+1。

**兩層驗證**（v11 由原本的 E3 拆開）：

| 層 | 誰做 | 結果 |
|---|---|---|
| **E3a** archive 自洽 | 發布（`--envcheck`）、`--recover-envcheck`、Stage 2 的三個呼叫端 | ⚠️ EQUIVALENT 與 NOT_EQUIVALENT **都合法**；封存值必須等於重算值 |
| **E3b** Stage 2 使用資格 | Stage 2 的 preflight、`--finalize`、`--recover-durability` | ⛔ 只接受 EQUIVALENT |

⚠️ 全量比對一律**串流**（「五之一」）：⛔ 不得把兩份 rows 同時整份載入——那正是 I-117 的 730 MiB。
"""
from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
from typing import Any, Mapping, Sequence

from .artifacts import (
    ARTIFACT_SCHEMA_VERSION,
    COHORT_KIND,
    ArtifactError,
    assert_same_keys,
    load_canonical_evidence_artifact,
    validate_cohort_manifest,
)
from .evidence import RECOVERY_IDENTITY_FIELDS, _fsync_tree
from .provenance import PROVENANCE_FIELDS, installed_distributions, pip_freeze_sha256, validate_provenance
from .publish import EXIT_ENV_NOT_EQUIVALENT, DurabilityUnconfirmed
from .run_identity import RUN_IDENTITY_KIND, load_run_identity, validate_run_identity
from .stage2_evidence import (
    DIAGNOSTIC_SAMPLE_LIMIT,
    EMPTY_SHA256,
    STAGE1_ANCHOR_AFTER,
    ClosedArchiveWriter,
    Stage1Anchor,
    archive_file_set,
    build_stage1_evidence_ref,
    fsync_parent_or_unconfirmed,
    is_hex64,
    is_image_id,
    load_stage1_anchor,
    resolve_repo_path,
    validate_file_entry,
    validate_generated_at,
    validate_stage1_anchor_graph,
    validate_stage1_evidence_ref,
)
from .stream import AfterStream, stream_after_artifact

# ── 常數 ────────────────────────────────────────────────────────────────────

# ⚠️ 寫死常數（repo 相對），⛔ 不是執行期參數。
ENVCHECK_ROOT_PATH = "python/baselines/i074_stage2/envcheck"
ENVCHECK_MANIFEST_NAME = "evidence_manifest.json"
ENVCHECK_MANIFEST_KIND = "sr_zone_stage2_envcheck_manifest"
ENV_EQUIVALENCE_KIND = "sr_zone_stage2_env_equivalence"

WITNESS_AFTER = "witness/after_artifact.json.gz"
WITNESS_COHORT = "witness/cohort_manifest.json.gz"
EQUIVALENCE = "equivalence/equivalence.json.gz"
ENVCHECK_IDENTITY = "identity/run_identity.json.gz"
# ⚠️ **封閉 layout**：4 個 payload ＋ manifest ＝ 5 檔，⛔ 多一個未知檔案即中止。
ENVCHECK_LAYOUT = (WITNESS_AFTER, WITNESS_COHORT, EQUIVALENCE, ENVCHECK_IDENTITY)

# operational 的見證輸出（`--run-dir` 底下；after' 以 Stage 1 模式跑，`--output-dir <run-dir>/witness`）。
OPERATIONAL_WITNESS_AFTER = "witness/after_artifact.json"
OPERATIONAL_WITNESS_COHORT = "witness/cohort_manifest.json"

OUTCOME_EQUIVALENT = "EQUIVALENT"
OUTCOME_NOT_EQUIVALENT = "NOT_EQUIVALENT"
# ⚠️ **outcome 的唯一來源是 equivalence artifact**；manifest 的 `terminal_outcome` 只是由它推導的值。
TERMINAL_OUTCOME = {OUTCOME_EQUIVALENT: 0, OUTCOME_NOT_EQUIVALENT: EXIT_ENV_NOT_EQUIVALENT}

SIDE_REFERENCE_ONLY = "REFERENCE_ONLY"   # 只在 Stage 1 D+1
SIDE_WITNESS_ONLY = "WITNESS_ONLY"       # 只在 after'
_SIDES = (SIDE_REFERENCE_ONLY, SIDE_WITNESS_ONLY)

# ⚠️ **判定表的第 3、4 條，合起來恰好是 provenance 的 10 欄**——⛔ 不留未分類的欄位。
REQUIRED_EQUAL_FIELDS = ("base_commit", "tooling_patch_sha256", "project_modules_sha256", "runtime_settings")
ALLOWED_DIFFERENT_FIELDS = ("image_digest", "pip_freeze_sha256", "python_version", "runner_sha256",
                            "argv", "source_root")
if set(REQUIRED_EQUAL_FIELDS) & set(ALLOWED_DIFFERENT_FIELDS) or \
        set(REQUIRED_EQUAL_FIELDS) | set(ALLOWED_DIFFERENT_FIELDS) != set(PROVENANCE_FIELDS):
    raise RuntimeError("環境等價判定的欄位分類與 PROVENANCE_FIELDS 不一致——⛔ 每一欄都必須恰好分到一類")

# 由比對重算出來的欄位（E3a 逐欄比對的對象）。
_RECOMPUTED_FIELDS = (
    "bundle_id", "reference", "witness",
    "reference_key_count", "witness_key_count", "key_mismatch_count", "key_mismatch_sample_keys",
    "rows_compared", "row_mismatch_count", "mismatch_sample_keys",
    "cohort_equal", "provenance_required_equal", "provenance_allowed_differences", "outcome",
)
_EQUIVALENCE_FIELDS = frozenset(
    _RECOMPUTED_FIELDS
    + ("schema_version", "kind", "generated_at", "witness_distributions", "comparator_provenance")
)
_REFERENCE_FIELDS = frozenset({"stage1_manifest_sha256", "after_artifact_sha256"})
_WITNESS_FIELDS = frozenset({"after_artifact_sha256", "cohort_manifest_sha256"})
_KEY_SAMPLE_FIELDS = frozenset({"symbol", "timeframe", "as_of", "side"})
_ROW_SAMPLE_FIELDS = frozenset({"symbol", "timeframe", "as_of"})
_ENVCHECK_MANIFEST_FIELDS = frozenset({
    "schema_version", "kind", "bundle_id", "expected_image_id", "files", "stage1_evidence",
    "terminal_outcome", "generated_at", "finalizer_provenance",
})

RECOVER_HINT = "scripts/finalize-stage2-evidence.sh --recover-envcheck"


# ── 比對 ────────────────────────────────────────────────────────────────────

def _key_item(key: tuple[str, str, str], **extra: str) -> dict[str, str]:
    return {"symbol": key[0], "timeframe": key[1], "as_of": key[2], **extra}


def derive_outcome(eq: Mapping[str, Any]) -> str:
    """⚠️ 判定式寫死（v12）：`EQUIVALENT` ⇔ key 集合相同 ＋ 交集內逐列相同 ＋ cohort 相同 ＋
    四個必須相同的 provenance 欄位都相同。"""
    equivalent = (
        eq["key_mismatch_count"] == 0
        and eq["row_mismatch_count"] == 0
        and eq["cohort_equal"] is True
        and all(eq["provenance_required_equal"][f] is True for f in REQUIRED_EQUAL_FIELDS)
    )
    return OUTCOME_EQUIVALENT if equivalent else OUTCOME_NOT_EQUIVALENT


def compare_environment(
    *,
    anchor: Stage1Anchor,
    witness: AfterStream,
    witness_cohort_keys: Sequence[tuple[str, str, str]],
    witness_cohort_sha256: str,
) -> dict[str, Any]:
    """由已驗證的兩側摘要算出判定（equivalence artifact 中「重算得出」的那些欄位）。

    ⚠️ 只用**每列 digest** 比，⛔ 不同時持有兩份全量 rows。
    """
    reference = anchor.after_row_digests
    witnessed = witness.row_digests
    ref_keys, wit_keys = set(reference), set(witnessed)
    common = ref_keys & wit_keys
    key_mismatch = sorted(
        [(key, SIDE_REFERENCE_ONLY) for key in ref_keys - wit_keys]
        + [(key, SIDE_WITNESS_ONLY) for key in wit_keys - ref_keys]
    )
    # ⚠️ `row_mismatch` 只計**兩側都有**、但 canonical row bytes 不同的 key（v12）。
    row_mismatch = sorted(key for key in common if reference[key] != witnessed[key])

    ref_prov = anchor.after_top["provenance"]
    wit_prov = witness.load.top["provenance"]
    eq: dict[str, Any] = {
        "bundle_id": anchor.bundle_id,
        "reference": {
            "stage1_manifest_sha256": anchor.manifest_sha256,
            "after_artifact_sha256": anchor.members[STAGE1_ANCHOR_AFTER]["artifact_sha256"],
        },
        "witness": {
            "after_artifact_sha256": witness.load.artifact_sha256,
            "cohort_manifest_sha256": witness_cohort_sha256,
        },
        "reference_key_count": len(ref_keys),
        "witness_key_count": len(wit_keys),
        "key_mismatch_count": len(key_mismatch),
        "key_mismatch_sample_keys": [
            _key_item(key, side=side) for key, side in key_mismatch[:DIAGNOSTIC_SAMPLE_LIMIT]
        ],
        "rows_compared": len(common),
        "row_mismatch_count": len(row_mismatch),
        "mismatch_sample_keys": [_key_item(key) for key in row_mismatch[:DIAGNOSTIC_SAMPLE_LIMIT]],
        "cohort_equal": sorted(witness_cohort_keys) == sorted(anchor.cohort_keys),
        "provenance_required_equal": {f: ref_prov[f] == wit_prov[f] for f in REQUIRED_EQUAL_FIELDS},
        "provenance_allowed_differences": {
            f: {"reference": ref_prov[f], "witness": wit_prov[f]} for f in ALLOWED_DIFFERENT_FIELDS
        },
    }
    eq["outcome"] = derive_outcome(eq)
    return eq


# ── equivalence artifact 的封閉 schema ─────────────────────────────────────

def _require_count(eq: Mapping[str, Any], name: str) -> int:
    value = eq[name]
    if type(value) is not int or value < 0:
        raise ArtifactError(f"equivalence.{name} 必須是非負整數：{value!r}")
    return value


def _validate_key_sample(items: object, *, name: str, fields: frozenset, count: int) -> list[tuple]:
    if not isinstance(items, list):
        raise ArtifactError(f"equivalence.{name} 必須是陣列")
    if len(items) != min(DIAGNOSTIC_SAMPLE_LIMIT, count):
        raise ArtifactError(
            f"equivalence.{name} 的長度必須恰好是 min({DIAGNOSTIC_SAMPLE_LIMIT}, {count})，實際 {len(items)}"
        )
    keys: list[tuple] = []
    for item in items:
        if not isinstance(item, dict) or set(item) != fields:
            raise ArtifactError(f"equivalence.{name}[] 的欄位集合必須恰好是 {sorted(fields)}")
        for f in ("symbol", "timeframe", "as_of"):
            if not isinstance(item[f], str) or not item[f]:
                raise ArtifactError(f"equivalence.{name}[].{f} 必須是非空字串")
        if "side" in fields and item["side"] not in _SIDES:
            raise ArtifactError(f"equivalence.{name}[].side 只接受 {_SIDES}：{item['side']!r}")
        keys.append((item["symbol"], item["timeframe"], item["as_of"]))
    if any(a >= b for a, b in zip(keys, keys[1:])):
        raise ArtifactError(f"equivalence.{name} 必須依 (symbol, timeframe, as_of) 排序且⛔ 不重複")
    return keys


def validate_equivalence_artifact(eq: object) -> None:
    """封閉 schema ＋ 計數不變條件 ＋ outcome 由各欄重算（⛔ 不信任封存值）。"""
    if not isinstance(eq, dict) or set(eq) != _EQUIVALENCE_FIELDS:
        actual = set(eq) if isinstance(eq, dict) else set()
        raise ArtifactError(
            "equivalence 的欄位集合不符："
            f"多={sorted(actual - _EQUIVALENCE_FIELDS)}、缺={sorted(_EQUIVALENCE_FIELDS - actual)}"
        )
    if type(eq["schema_version"]) is not int or eq["schema_version"] != ARTIFACT_SCHEMA_VERSION:
        raise ArtifactError(f"未知的 equivalence schema_version={eq['schema_version']!r}")
    if eq["kind"] != ENV_EQUIVALENCE_KIND:
        raise ArtifactError(f"equivalence 的 kind={eq['kind']!r}，預期 {ENV_EQUIVALENCE_KIND!r}")
    if not isinstance(eq["bundle_id"], str) or not eq["bundle_id"]:
        raise ArtifactError("equivalence.bundle_id 必須是非空字串")
    validate_generated_at(eq["generated_at"], "equivalence")
    for name, fields in (("reference", _REFERENCE_FIELDS), ("witness", _WITNESS_FIELDS)):
        block = eq[name]
        if not isinstance(block, dict) or set(block) != fields:
            raise ArtifactError(f"equivalence.{name} 的欄位集合必須恰好是 {sorted(fields)}")
        if not all(is_hex64(block[f]) for f in fields):
            raise ArtifactError(f"equivalence.{name} 的 SHA 必須是 64 字元小寫 hex")

    ref_count = _require_count(eq, "reference_key_count")
    wit_count = _require_count(eq, "witness_key_count")
    key_mismatch = _require_count(eq, "key_mismatch_count")
    compared = _require_count(eq, "rows_compared")
    row_mismatch = _require_count(eq, "row_mismatch_count")
    # ⚠️ 計數不變條件（v12）：rows_compared ＝ 交集；兩側「只在單側」的數量由總數與交集推得。
    if compared > ref_count or compared > wit_count:
        raise ArtifactError("equivalence.rows_compared（交集）⛔ 不得大於任一側的 key 數")
    ref_only, wit_only = ref_count - compared, wit_count - compared
    if key_mismatch != ref_only + wit_only:
        raise ArtifactError(
            "equivalence 的計數不變條件不成立：key_mismatch_count 必須等於 "
            "(reference_key_count − rows_compared) ＋ (witness_key_count − rows_compared)"
        )
    if row_mismatch > compared:
        raise ArtifactError("equivalence.row_mismatch_count ⛔ 不得大於 rows_compared（它只計交集）")
    _validate_key_sample(eq["key_mismatch_sample_keys"], name="key_mismatch_sample_keys",
                         fields=_KEY_SAMPLE_FIELDS, count=key_mismatch)
    sides = [item["side"] for item in eq["key_mismatch_sample_keys"]]
    if sides.count(SIDE_REFERENCE_ONLY) > ref_only or sides.count(SIDE_WITNESS_ONLY) > wit_only:
        raise ArtifactError("equivalence.key_mismatch_sample_keys 的 side 分佈與計數不符")
    _validate_key_sample(eq["mismatch_sample_keys"], name="mismatch_sample_keys",
                         fields=_ROW_SAMPLE_FIELDS, count=row_mismatch)

    if not isinstance(eq["cohort_equal"], bool):
        raise ArtifactError("equivalence.cohort_equal 必須是嚴格 boolean")
    required = eq["provenance_required_equal"]
    if not isinstance(required, dict) or set(required) != set(REQUIRED_EQUAL_FIELDS) \
            or not all(isinstance(v, bool) for v in required.values()):
        raise ArtifactError(
            f"equivalence.provenance_required_equal 必須恰好是 {list(REQUIRED_EQUAL_FIELDS)} 的嚴格 boolean"
        )
    allowed = eq["provenance_allowed_differences"]
    if not isinstance(allowed, dict) or set(allowed) != set(ALLOWED_DIFFERENT_FIELDS) or not all(
        isinstance(v, dict) and set(v) == {"reference", "witness"} for v in allowed.values()
    ):
        raise ArtifactError(
            f"equivalence.provenance_allowed_differences 必須恰好是 {list(ALLOWED_DIFFERENT_FIELDS)}，"
            "每一欄恰好 {reference, witness}"
        )

    dists = eq["witness_distributions"]
    if not isinstance(dists, list) or not all(
        isinstance(item, list) and len(item) == 2 and all(isinstance(v, str) for v in item) and item[0]
        for item in dists
    ):
        raise ArtifactError("equivalence.witness_distributions 必須是 [[name, version], …]")
    if any(tuple(a) >= tuple(b) for a, b in zip(dists, dists[1:])):
        raise ArtifactError("equivalence.witness_distributions 必須排序且⛔ 不重複")

    if eq["outcome"] not in TERMINAL_OUTCOME:
        raise ArtifactError(f"equivalence.outcome 只接受 {list(TERMINAL_OUTCOME)}：{eq['outcome']!r}")
    if eq["outcome"] != derive_outcome(eq):
        raise ArtifactError("equivalence.outcome 與各欄重算的結果不符——⛔ 不信任封存值")
    validate_provenance(eq["comparator_provenance"], role="comparator")


# ── envcheck manifest 的封閉 schema ────────────────────────────────────────

def build_envcheck_manifest(
    *, bundle_id: str, expected_image_id: str, files: Mapping[str, Mapping[str, Any]],
    stage1_evidence: dict[str, Any], terminal_outcome: int, generated_at: str,
    finalizer_provenance: dict[str, Any],
) -> dict[str, Any]:
    return {
        "schema_version": ARTIFACT_SCHEMA_VERSION,
        "kind": ENVCHECK_MANIFEST_KIND,
        "bundle_id": bundle_id,
        "expected_image_id": expected_image_id,
        "files": {rel: dict(files[rel]) for rel in sorted(files)},
        "stage1_evidence": stage1_evidence,
        "terminal_outcome": terminal_outcome,
        "generated_at": generated_at,
        "finalizer_provenance": finalizer_provenance,
    }


def validate_envcheck_manifest(manifest: object) -> None:
    if not isinstance(manifest, dict) or set(manifest) != _ENVCHECK_MANIFEST_FIELDS:
        actual = set(manifest) if isinstance(manifest, dict) else set()
        raise ArtifactError(
            "envcheck manifest 的欄位集合不符："
            f"多={sorted(actual - _ENVCHECK_MANIFEST_FIELDS)}、缺={sorted(_ENVCHECK_MANIFEST_FIELDS - actual)}"
        )
    if type(manifest["schema_version"]) is not int or manifest["schema_version"] != ARTIFACT_SCHEMA_VERSION:
        raise ArtifactError(f"未知的 envcheck manifest schema_version={manifest['schema_version']!r}")
    if manifest["kind"] != ENVCHECK_MANIFEST_KIND:
        raise ArtifactError(f"envcheck manifest 的 kind={manifest['kind']!r}，預期 {ENVCHECK_MANIFEST_KIND!r}")
    if not isinstance(manifest["bundle_id"], str) or not manifest["bundle_id"]:
        raise ArtifactError("envcheck manifest 的 bundle_id 必須是非空字串")
    if not is_image_id(manifest["expected_image_id"]):
        raise ArtifactError("envcheck manifest 的 expected_image_id 必須是 sha256: ＋ 64 字元小寫 hex")
    files = manifest["files"]
    if not isinstance(files, dict) or set(files) != set(ENVCHECK_LAYOUT):
        actual = set(files) if isinstance(files, dict) else set()
        raise ArtifactError(
            f"envcheck manifest 的 files 必須恰好是 {list(ENVCHECK_LAYOUT)}："
            f"多={sorted(actual - set(ENVCHECK_LAYOUT))}、缺={sorted(set(ENVCHECK_LAYOUT) - actual)}"
        )
    for rel, entry in files.items():
        # ⚠️ 四個都是 canonical artifact——⛔ 沒有 raw_blob（型別由 layout 決定，⛔ 不由 manifest 宣告）。
        validate_file_entry(rel, entry)
    validate_stage1_evidence_ref(manifest["stage1_evidence"])
    outcome = manifest["terminal_outcome"]
    if type(outcome) is not int or outcome not in TERMINAL_OUTCOME.values():
        raise ArtifactError(
            f"envcheck manifest 的 terminal_outcome 只接受 {sorted(TERMINAL_OUTCOME.values())}：{outcome!r}"
        )
    validate_generated_at(manifest["generated_at"], "envcheck manifest")
    validate_provenance(manifest["finalizer_provenance"], role="finalizer")
    if manifest["finalizer_provenance"]["image_digest"] != manifest["expected_image_id"]:
        raise ArtifactError("envcheck manifest 的 finalizer_provenance.image_digest 與 expected_image_id 不符")


# ── E1～E7 ──────────────────────────────────────────────────────────────────

@dataclass(frozen=True)
class EnvcheckResult:
    outcome: str
    terminal_outcome: int
    identity: dict[str, Any]
    manifest: dict[str, Any]
    # ⚠️ ③d：Stage 2 manifest 的 `environment_witness` 以它錨定（與 manifest 內容來自同一次讀取）。
    manifest_sha256: str


def _check_entry(rel: str, entry: Mapping[str, Any], *, artifact_sha: str, stored_sha: str,
                 stored_bytes: int) -> None:
    if (entry["artifact_sha256"], entry["stored_sha256"], entry["stored_bytes"]) != \
            (artifact_sha, stored_sha, stored_bytes):
        raise ArtifactError(f"E1：manifest 記的 {rel} metadata 與實際檔案不符")


def verify_envcheck(
    root: str | Path,
    anchor: Stage1Anchor,
    *,
    require_equivalent: bool,
    stage2_identity: Mapping[str, Any] | None = None,
) -> EnvcheckResult:
    """E1～E7。`require_equivalent=True` 時再加 **E3b**（Stage 2 的使用資格）。

    ⚠️ 每份檔案**只讀一次**；全量的 after' 用串流讀。
    """
    root = Path(root)
    manifest_load = load_canonical_evidence_artifact(root / ENVCHECK_MANIFEST_NAME, ENVCHECK_MANIFEST_KIND)
    manifest = manifest_load.parsed
    validate_envcheck_manifest(manifest)

    # ── E1：封閉 layout、每個成員重算 metadata、stage1_evidence 綁到本次重算的信任錨 ──
    actual = archive_file_set(root)
    expected = set(ENVCHECK_LAYOUT) | {ENVCHECK_MANIFEST_NAME}
    if actual != expected:
        raise ArtifactError(
            f"E1：envcheck 的檔案集合不符：多={sorted(actual - expected)}、缺={sorted(expected - actual)}"
        )
    validate_stage1_evidence_ref(manifest["stage1_evidence"], anchor=anchor)
    files = manifest["files"]

    cohort_load = load_canonical_evidence_artifact(root / WITNESS_COHORT, COHORT_KIND)
    _check_entry(WITNESS_COHORT, files[WITNESS_COHORT], artifact_sha=cohort_load.artifact_sha256,
                 stored_sha=cohort_load.stored_sha256, stored_bytes=cohort_load.stored_bytes)
    identity_load = load_canonical_evidence_artifact(root / ENVCHECK_IDENTITY, RUN_IDENTITY_KIND)
    _check_entry(ENVCHECK_IDENTITY, files[ENVCHECK_IDENTITY], artifact_sha=identity_load.artifact_sha256,
                 stored_sha=identity_load.stored_sha256, stored_bytes=identity_load.stored_bytes)
    eq_load = load_canonical_evidence_artifact(root / EQUIVALENCE, ENV_EQUIVALENCE_KIND)
    _check_entry(EQUIVALENCE, files[EQUIVALENCE], artifact_sha=eq_load.artifact_sha256,
                 stored_sha=eq_load.stored_sha256, stored_bytes=eq_load.stored_bytes)
    witness = stream_after_artifact(root / WITNESS_AFTER, label="環境見證 after'", side="after")
    _check_entry(WITNESS_AFTER, files[WITNESS_AFTER], artifact_sha=witness.load.artifact_sha256,
                 stored_sha=witness.load.stored_sha256, stored_bytes=witness.load.stored_bytes)

    # ── E2：完整 validator（⛔ 錨定⛔ 不等於驗過） ────────────────────────────
    cohort = cohort_load.parsed
    cohort_keys = validate_cohort_manifest(cohort)
    validate_provenance(cohort["provenance"], role="stage1")
    witness_prov = witness.load.top["provenance"]
    validate_provenance(witness_prov, role="stage1")
    identity = identity_load.parsed
    validate_run_identity(identity)
    eq = eq_load.parsed
    validate_equivalence_artifact(eq)
    # cohort' 必須綁到這一份 after'（比照 Stage 1 信任錨第 8、9 道；⛔ 不重算 predicate）。
    if cohort.get("after_artifact_sha256") != witness.load.artifact_sha256:
        raise ArtifactError("E2：cohort' 記的 after_artifact_sha256 與 after' 不符")
    assert_same_keys(cohort_keys, witness.candidate_keys, "E2：cohort' vs after' 的候選列")

    # ── E3a：重新串流比對，封存值必須等於重算值；outcome 三者一致 ──────────────
    recomputed = compare_environment(
        anchor=anchor, witness=witness, witness_cohort_keys=cohort_keys,
        witness_cohort_sha256=cohort_load.artifact_sha256,
    )
    for name in _RECOMPUTED_FIELDS:
        if eq[name] != recomputed[name]:
            raise ArtifactError(f"E3a：封存的 equivalence.{name} 與重算值不符——⛔ 不信任封存值")
    outcome = recomputed["outcome"]
    if manifest["terminal_outcome"] != TERMINAL_OUTCOME[outcome]:
        raise ArtifactError(
            f"E3a：outcome 三者不一致——重算／封存＝{outcome}，manifest 的 terminal_outcome="
            f"{manifest['terminal_outcome']}"
        )

    # ── E4：封存的套件清單與 after' 的 pip_freeze_sha256 交叉驗證 ───────────────
    if pip_freeze_sha256([tuple(item) for item in eq["witness_distributions"]]) != \
            witness_prov["pip_freeze_sha256"]:
        raise ArtifactError("E4：witness_distributions 的 hash 與 after' 的 pip_freeze_sha256 不符")

    # ── E5：image 鏈 ────────────────────────────────────────────────────────
    image_chain = {
        "envcheck identity": identity["expected_image_id"],
        "after'": witness_prov["image_digest"],
        "cohort'": cohort["provenance"]["image_digest"],
        "equivalence comparator": eq["comparator_provenance"]["image_digest"],
        "envcheck manifest": manifest["expected_image_id"],
    }
    if len(set(image_chain.values())) != 1:
        raise ArtifactError(f"E5：image 等式鏈斷了：{image_chain}")
    if stage2_identity is not None:
        validate_run_identity(stage2_identity)
        if dict(stage2_identity) != identity:
            raise ArtifactError("E5：Stage 2 的 run identity 與 envcheck 封存的那一份⛔ 不完全相同")

    # ── E6：bundle 鏈 ───────────────────────────────────────────────────────
    bundle_chain = {
        "Stage 1 manifest": anchor.bundle_id,
        "envcheck manifest": manifest["bundle_id"],
        "envcheck identity": identity["bundle_id"],
        "after'": witness.load.top["bundle_id"],
        "cohort'": cohort["bundle_id"],
        "equivalence": eq["bundle_id"],
    }
    if len(set(bundle_chain.values())) != 1:
        raise ArtifactError(f"E6：bundle_id 等式鏈斷了：{bundle_chain}")

    # ── E7：after' 跑的是原始 e1cbbbd、⛔ 沒有套任何 patch ─────────────────────
    if witness_prov["base_commit"] != anchor.after_base_commit:
        raise ArtifactError(
            f"E7：after' 的 base_commit={witness_prov['base_commit']} 與 Stage 1 after 的 "
            f"{anchor.after_base_commit} 不符"
        )
    if witness_prov["tooling_patch_sha256"] != EMPTY_SHA256:
        raise ArtifactError("E7：after' ⛔ 不得套任何 patch（tooling_patch_sha256 必須是空字串的 SHA）")

    # Stage 1 信任錨的跨檔關係（第 6、8、9、10 道）。
    validate_stage1_anchor_graph(
        anchor,
        stage2_identity=stage2_identity if stage2_identity is not None else identity,
        envcheck_identity=identity,
        expected_bundle_id=identity["bundle_id"],
    )

    # ── E3b：Stage 2 使用資格 ───────────────────────────────────────────────
    if require_equivalent and outcome != OUTCOME_EQUIVALENT:
        raise ArtifactError(
            "E3b：環境見證的結果是 NOT_EQUIVALENT——它是合法證據，⛔ 但⛔ 不得拿來放行 Stage 2"
        )
    return EnvcheckResult(outcome=outcome, terminal_outcome=TERMINAL_OUTCOME[outcome],
                          identity=identity, manifest=manifest,
                          manifest_sha256=manifest_load.stored_sha256)


def verify_envcheck_for_stage2(python_root: str | Path, stage2_identity: Mapping[str, Any]) -> EnvcheckResult:
    """Stage 2 的 preflight／finalize／recovery 用：E1～E7 ＋ **E3b**。"""
    anchor = load_stage1_anchor(python_root)
    root = resolve_repo_path(python_root, ENVCHECK_ROOT_PATH)
    return verify_envcheck(root, anchor, require_equivalent=True, stage2_identity=stage2_identity)


# ── 發布與 recovery ─────────────────────────────────────────────────────────

def publish_envcheck(
    *,
    python_root: str | Path,
    run_dir: str | Path,
    stage2_identity: Mapping[str, Any],
    provenance_factory,
    generated_at: str,
    distributions: list[list[str]] | None = None,
) -> int:
    """見證趟的 operational 輸出 → 串流比對 → 發布 `envcheck/`。回傳 0 或 7。

    ⚠️ **兩種結果都發布證據**：NOT_EQUIVALENT 是合法的終止狀態，⛔ 不是一般失敗。
    ⚠️ 階段 A 讀入並完成全部驗證（所有 project import 在此發生）；階段 B 才建 provenance。
    """
    validate_run_identity(stage2_identity)
    stage2_identity = dict(stage2_identity)
    root = resolve_repo_path(python_root, ENVCHECK_ROOT_PATH)
    run_dir = Path(run_dir)

    anchor = load_stage1_anchor(python_root)
    validate_stage1_anchor_graph(
        anchor, stage2_identity=stage2_identity, envcheck_identity=stage2_identity,
        expected_bundle_id=stage2_identity["bundle_id"],
    )

    with ClosedArchiveWriter(root, ENVCHECK_LAYOUT, recover_hint=RECOVER_HINT) as writer:
        # ── 階段 A ──────────────────────────────────────────────────────────
        cohort_load = load_canonical_evidence_artifact(run_dir / OPERATIONAL_WITNESS_COHORT, COHORT_KIND)
        cohort = cohort_load.parsed
        cohort_keys = validate_cohort_manifest(cohort)
        validate_provenance(cohort["provenance"], role="stage1")
        # ⚠️ after' 在**驗證的同一趟**寫出封存副本——⛔ 不得事後另讀一次。
        witness = stream_after_artifact(
            run_dir / OPERATIONAL_WITNESS_AFTER, label="環境見證 after'", side="after",
            copy_to=writer.path_for(WITNESS_AFTER),
        )
        writer.record_streamed(WITNESS_AFTER, witness.load)
        validate_provenance(witness.load.top["provenance"], role="stage1")
        writer.add_canonical(WITNESS_COHORT, cohort)
        writer.add_canonical(ENVCHECK_IDENTITY, stage2_identity)
        core = compare_environment(
            anchor=anchor, witness=witness, witness_cohort_keys=cohort_keys,
            witness_cohort_sha256=cohort_load.artifact_sha256,
        )
        dists = installed_distributions() if distributions is None else distributions

        # ── 階段 B ──────────────────────────────────────────────────────────
        provenance = provenance_factory()
        validate_provenance(provenance, role="comparator")
        validate_provenance(provenance, role="finalizer")
        equivalence = {
            "schema_version": ARTIFACT_SCHEMA_VERSION,
            "kind": ENV_EQUIVALENCE_KIND,
            "generated_at": generated_at,
            **core,
            "witness_distributions": dists,
            "comparator_provenance": provenance,
        }
        validate_equivalence_artifact(equivalence)
        writer.add_canonical(EQUIVALENCE, equivalence)
        terminal = TERMINAL_OUTCOME[core["outcome"]]
        manifest = build_envcheck_manifest(
            bundle_id=stage2_identity["bundle_id"],
            expected_image_id=stage2_identity["expected_image_id"],
            files=writer.files,
            stage1_evidence=build_stage1_evidence_ref(anchor),
            terminal_outcome=terminal,
            generated_at=generated_at,
            finalizer_provenance=provenance,
        )
        validate_envcheck_manifest(manifest)

        def verify_staging(staging: Path) -> None:
            # ⚠️ 發布前對 staging 跑**與 recovery 相同**的 E1～E7（E3 只做 E3a）。
            verify_envcheck(staging, anchor, require_equivalent=False, stage2_identity=stage2_identity)
            # ⚠️ ⛔ 階段 B 之後⛔ 不得有新的 project import——重算一次，與封存的比對。
            if provenance_factory()["project_modules_sha256"] != provenance["project_modules_sha256"]:
                raise ArtifactError("階段 B 之後又載入了新的專案模組——⛔ 封存的 provenance 已經記不全了")

        writer.commit(ENVCHECK_MANIFEST_NAME, manifest, verify=verify_staging)
    return terminal


def recover_envcheck(*, python_root: str | Path, provenance: dict[str, Any]) -> int:
    """⚠️ **只重新驗證既有 `envcheck/` 並重新 fsync**，⛔ 不讀 operational 輸入、⛔ 不重跑 replay。

    回傳 manifest 的 `terminal_outcome`（0 或 7）——⛔ 不得固定回 0。
    """
    root = resolve_repo_path(python_root, ENVCHECK_ROOT_PATH)
    if not root.is_dir():
        raise ArtifactError(f"--recover-envcheck 要求 envcheck/ 已存在：{root}")
    anchor = load_stage1_anchor(python_root)
    result = verify_envcheck(root, anchor, require_equivalent=False)
    # ⚠️ 本次執行身分必須與封存的 finalizer_provenance 相同——**在 fsync 之前**比。
    archived = result.manifest["finalizer_provenance"]
    validate_provenance(provenance, role="finalizer")
    for name in RECOVERY_IDENTITY_FIELDS:
        if provenance.get(name) != archived.get(name):
            raise ArtifactError(
                f"recovery 的執行身分與封存的 finalizer_provenance 的 {name} 不符——⛔ 在 fsync 之前中止"
            )
    _fsync_tree(root)
    fsync_parent_or_unconfirmed(root, published=False, recover_hint=RECOVER_HINT)
    return result.terminal_outcome


# ── CLI ─────────────────────────────────────────────────────────────────────

class EnvcheckUsageError(ValueError):
    """CLI 用法錯誤。"""


# ⚠️ 只能由 `scripts/finalize-stage2-evidence.sh` 注入。精確比對（⛔ 不用前綴清單）。
ENVCHECK_INJECTED_ARGS = (
    "--run-identity", "--python-root", "--image-digest", "--base-commit",
    "--tooling-patch-sha256", "--source-root", "--runner-sha256",
)


def assert_envcheck_arg_ownership(argv: Sequence[str]) -> None:
    for injected in ENVCHECK_INJECTED_ARGS:
        count = sum(1 for a in argv if a == injected or a.startswith(injected + "="))
        if count > 1:
            raise EnvcheckUsageError(
                f"{injected} 出現 {count} 次——它只能由官方腳本注入。⛔ 不靜默採用最後一個。"
            )


def build_envcheck_parser():
    import argparse

    parser = argparse.ArgumentParser(prog="envcheck", allow_abbrev=False)
    parser.add_argument("--envcheck", action="store_true")
    parser.add_argument("--recover-envcheck", action="store_true")
    parser.add_argument("--run-dir", default=None)
    parser.add_argument("--run-identity", default=None)
    parser.add_argument("--python-root", required=True)
    parser.add_argument("--image-digest", required=True)
    parser.add_argument("--base-commit", required=True)
    parser.add_argument("--tooling-patch-sha256", required=True)
    parser.add_argument("--source-root", required=True)
    parser.add_argument("--runner-sha256", required=True)
    return parser


def run_envcheck(argv: Sequence[str]) -> dict[str, Any]:
    from datetime import datetime, timezone

    from .provenance import build_provenance

    argv = list(argv)
    assert_envcheck_arg_ownership(argv)
    args = build_envcheck_parser().parse_args(argv)
    # ⚠️ **CLI matrix**（③ evidence contract「七之四」）：兩種模式互斥且必選其一。
    if args.envcheck == args.recover_envcheck:
        raise EnvcheckUsageError("--envcheck 與 --recover-envcheck 必須恰好給一個")
    if args.envcheck:
        if not args.run_dir or not args.run_identity:
            raise EnvcheckUsageError("--envcheck 需要 --run-dir 與 --run-identity")
    elif args.run_dir or args.run_identity:
        raise EnvcheckUsageError(
            "--recover-envcheck ⛔ 不接受 --run-dir 或 --run-identity：它只依賴既有的 envcheck/"
        )

    # ⚠️ 階段 A 明確載入 `config`（理由同 Stage 1 finalizer：⛔ 靠間接 import 是 incidental）。
    import config as _config_module

    def _provenance_factory():
        return build_provenance(
            config_module=_config_module,
            source_root=args.source_root,
            image_digest=args.image_digest,
            base_commit=args.base_commit,
            tooling_patch_sha256=args.tooling_patch_sha256,
            runner_sha256=args.runner_sha256,
            argv=argv,
        )

    if args.recover_envcheck:
        terminal = recover_envcheck(python_root=args.python_root, provenance=_provenance_factory())
        mode = "recover_envcheck"
    else:
        identity = load_run_identity(args.run_identity)
        if identity["expected_image_id"] != args.image_digest:
            raise EnvcheckUsageError(
                f"Stage 2 identity 記的 image {identity['expected_image_id']} 與本次執行的 "
                f"{args.image_digest} 不符"
            )
        terminal = publish_envcheck(
            python_root=args.python_root,
            run_dir=args.run_dir,
            stage2_identity=identity,
            provenance_factory=_provenance_factory,
            generated_at=datetime.now(timezone.utc).isoformat(),
        )
        mode = "envcheck"
    outcome = OUTCOME_EQUIVALENT if terminal == 0 else OUTCOME_NOT_EQUIVALENT
    return {"mode": mode, "outcome": outcome, "terminal_outcome": terminal}


def main(argv=None) -> int:
    import json
    import sys

    from .publish import EXIT_ABORT, EXIT_DURABILITY_UNCONFIRMED

    argv = list(sys.argv[1:] if argv is None else argv)
    try:
        result = run_envcheck(argv)
    except DurabilityUnconfirmed as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return EXIT_DURABILITY_UNCONFIRMED
    except (EnvcheckUsageError, ValueError, OSError) as exc:
        print(f"ERROR: {type(exc).__name__}: {exc}", file=sys.stderr)
        return EXIT_ABORT
    print(json.dumps(result, ensure_ascii=False, sort_keys=True))
    # ⚠️ ⛔ 不得固定回 0：NOT_EQUIVALENT（7）是要讓 Stage 2 停下來的終止狀態。
    return result["terminal_outcome"]


if __name__ == "__main__":
    raise SystemExit(main())
