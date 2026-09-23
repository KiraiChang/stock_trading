"""I-074 Stage 2 的證據契約：共用的信任錨與封閉 layout 的 archive 核心。

規格見 issue.md I-074「③ Stage 2 evidence contract 計畫書」（⚠️ 現行版）。本檔目前只有
**③b 第一包**要用的部分：

* **Stage 1 信任錨**（「三之一」）的串流版：`load_stage1_anchor()` ＋ `validate_stage1_anchor_graph()`
  ——⚠️ 只常駐 keys、每列 digest 與 156 筆 cohort rows，⛔ 不整份載入 after（「五之一」）；
* `stage1_evidence` 欄位的 builder／validator——Stage 2 manifest 與 envcheck manifest
  **共用同一個定義**（⛔ 不各訂一份）；
* **封閉 layout 的 archive 核心** `ClosedArchiveWriter`——比照 Stage 1 `finalize_evidence()`
  的發布段：staging 建完整 tree → `rename_noreplace()` 是**唯一的 commit point** → fsync parent；
* 路徑安全 `resolve_repo_path()`。

Stage 2 archive 本身、failed-attempt record 與 check／recover 屬 ③d，⛔ 不在本檔。

⛔ **不改 Stage 1 的 `evidence.py`**——`ARCHIVE_LAYOUT` 是 9 檔封閉集合，動它會讓
`python/baselines/i074_stage1/` 已封存的證據立刻驗不過。這裡只**呼叫**它的 validator。
"""
from __future__ import annotations

import secrets
from dataclasses import dataclass, field
from pathlib import Path, PurePosixPath
from typing import Any, Mapping

from .artifacts import (
    COHORT_KIND,
    ArtifactError,
    assert_same_keys,
    load_canonical_evidence_artifact,
    validate_cohort_manifest,
)
from .canonical import canonical_gzip_bytes, canonical_json_bytes, sha256_hex
from .evidence import EVIDENCE_MANIFEST_KIND, _fsync_tree, validate_evidence_manifest
from .provenance import validate_provenance
from .publish import DurabilityUnconfirmed, fsync_dir, probe_no_clobber, remove_tree, rename_noreplace
from .run_identity import RUN_IDENTITY_KIND, validate_run_identity
from .stream import StreamLoad, stream_after_artifact

# ── 常數 ────────────────────────────────────────────────────────────────────

# ⚠️ **寫死常數，⛔ 不是執行期參數**：I-074 是一次性正式驗證，⛔ 不需要通用性；
# 留成可指定的話，另一份同 bundle／同 image 的合法 manifest 也能充數（「三之一」）。
STAGE1_EVIDENCE_MANIFEST_PATH = "python/baselines/i074_stage1/evidence_manifest.json"

STAGE1_ANCHOR_AFTER = "d1/after_artifact.json.gz"
STAGE1_ANCHOR_COHORT = "d1/cohort_manifest.json.gz"
STAGE1_ANCHOR_IDENTITY = "identity/run_identity.json.gz"
# ⛔ key 集合**恰好是這三個**，⛔ 不可增減。
STAGE1_ANCHOR_MEMBERS = (STAGE1_ANCHOR_AFTER, STAGE1_ANCHOR_COHORT, STAGE1_ANCHOR_IDENTITY)

# 有界診斷的 sample 上限。⚠️ **寫死的共用常數**，⛔ 不開 CLI 參數；failed record 與環境等價判定
# **共用**這個上限，但 sample 的欄位集合各自封閉。⛔ 不得套用到 B／C 的正式證據。
DIAGNOSTIC_SAMPLE_LIMIT = 20

# 空 tooling patch 的 SHA（空字串的 SHA-256）。
EMPTY_SHA256 = sha256_hex(b"")

_MEMBER_FIELDS = frozenset({"artifact_sha256", "stored_sha256"})
_STAGE1_REF_FIELDS = frozenset({"manifest_path", "manifest_sha256", "members"})
FILE_ENTRY_FIELDS = frozenset({"artifact_sha256", "stored_sha256", "stored_bytes"})


def is_hex64(value: object) -> bool:
    return (
        isinstance(value, str) and len(value) == 64 and value == value.lower()
        and all(c in "0123456789abcdef" for c in value)
    )


def is_image_id(value: object) -> bool:
    return isinstance(value, str) and value.startswith("sha256:") and is_hex64(value[7:])


def validate_generated_at(value: object, label: str) -> None:
    """ISO-8601 且**含時區**——⛔ 只驗「非空字串」的話，裸日期字串也會過。"""
    from datetime import datetime

    if not isinstance(value, str) or not value:
        raise ArtifactError(f"{label} 的 generated_at 必須是非空字串")
    try:
        parsed = datetime.fromisoformat(value)
    except ValueError as exc:
        raise ArtifactError(f"{label} 的 generated_at 不是合法 ISO-8601：{exc}") from exc
    if parsed.tzinfo is None:
        raise ArtifactError(f"{label} 的 generated_at ⛔ 必須含時區")


# ── 路徑安全 ────────────────────────────────────────────────────────────────

def resolve_repo_path(python_root: str | Path, rel: str) -> Path:
    """把 `python/…` 形式的 repo 相對路徑解析到 `python_root` 底下。

    ⚠️ 威脅模型與 `scripts/check-doc-refs.py` 的 `_is_acceptable_local_file()` 相同
    （⛔ 不另造一套語意）：拒絕**絕對路徑**、含 `..`／`.` 的成分、**任何一段是 symlink**，
    以及**解析後逃出 `python_root`** 的路徑。
    """
    if not isinstance(rel, str) or not rel:
        raise ArtifactError(f"repo 相對路徑必須是非空字串：{rel!r}")
    pure = PurePosixPath(rel)
    if pure.is_absolute() or "\\" in rel:
        raise ArtifactError(f"⛔ 不接受絕對路徑：{rel!r}")
    parts = pure.parts
    if not parts or parts[0] != "python" or len(parts) < 2:
        raise ArtifactError(f"repo 相對路徑必須位於 python/ 底下：{rel!r}")
    if any(part in ("..", ".") for part in rel.split("/")):
        raise ArtifactError(f"⛔ 路徑不得含 '..' 或 '.'：{rel!r}")
    root = Path(python_root)
    probe = root
    for part in parts[1:]:
        probe = probe / part
        # ⚠️ 逐段檢查：⛔ 只看最後一段不夠，中間目錄是 symlink 一樣能跳出 root。
        if probe.is_symlink():
            raise ArtifactError(f"⛔ 路徑中有 symlink：{probe}")
    if not probe.resolve().is_relative_to(root.resolve()):
        raise ArtifactError(f"⛔ 路徑解析後逃出 python root：{rel!r}")
    return probe


# ── Stage 1 信任錨（「三之一」，串流版） ───────────────────────────────────

@dataclass(frozen=True)
class Stage1Anchor:
    """`load_stage1_anchor()` 的 snapshot。⚠️ 每一項都來自**同一次讀取**（⛔ 之後不再重讀）。"""

    manifest_path: str
    manifest_sha256: str
    manifest_payload: dict[str, Any]
    members: dict[str, dict[str, str]]
    identity: dict[str, Any]
    bundle_id: str          # 取自已驗證的 manifest
    expected_image_id: str  # 取自已驗證的 manifest
    after_base_commit: str  # 取自已驗證的 after provenance（checker 推導 base 要用）
    after_top: dict[str, Any]
    after_keys: list[tuple[str, str, str]]
    after_row_digests: dict[tuple[str, str, str], bytes]
    after_candidate_keys: list[tuple[str, str, str]]
    cohort_payload: dict[str, Any]
    cohort_keys: list[tuple[str, str, str]]
    cohort_rows: dict[tuple[str, str, str], dict[str, Any]] = field(repr=False)


def _check_member_shas(rel: str, manifest_files: Mapping[str, Any], *, artifact_sha: str,
                       stored_sha: str) -> dict[str, str]:
    entry = manifest_files[rel]
    if entry["artifact_sha256"] != artifact_sha or entry["stored_sha256"] != stored_sha:
        raise ArtifactError(
            f"Stage 1 信任錨：{rel} 重算的 SHA 與 Stage 1 manifest 記的不符——"
            "⚠️ Stage 2 archive 與 python/baselines/i074_stage1/ **必須一起保存**"
        )
    return {"artifact_sha256": artifact_sha, "stored_sha256": stored_sha}


def load_stage1_anchor(python_root: str | Path) -> Stage1Anchor:
    """單次讀取 ＋ 單檔驗證（「三之一」第 1～5、7 道）。

    ⚠️ **hash、validator 與回傳值全部來自同一次讀取**——⛔ 不得為了省記憶體之後再讀一次。
    ⚠️ after 用串流讀（「五之一」）：只常駐 keys、每列 digest 與 cohort 那幾列的完整 row。
    """
    manifest_path = resolve_repo_path(python_root, STAGE1_EVIDENCE_MANIFEST_PATH)
    try:
        manifest_load = load_canonical_evidence_artifact(manifest_path, EVIDENCE_MANIFEST_KIND)
    except ArtifactError as exc:
        raise ArtifactError(
            f"Stage 1 信任錨：{exc}——⚠️ Stage 2 的證據必須與 python/baselines/i074_stage1/ 一起保存"
        ) from exc
    manifest = manifest_load.parsed
    validate_evidence_manifest(manifest)                                   # 道 2
    files = manifest["files"]
    base = STAGE1_EVIDENCE_MANIFEST_PATH.rsplit("/", 1)[0]

    def member_path(rel: str) -> Path:
        return resolve_repo_path(python_root, f"{base}/{rel}")

    members: dict[str, dict[str, str]] = {}

    # cohort 先讀（小）——才知道 after 串流時要留哪幾列的完整 row。
    cohort_load = load_canonical_evidence_artifact(member_path(STAGE1_ANCHOR_COHORT), COHORT_KIND)
    members[STAGE1_ANCHOR_COHORT] = _check_member_shas(
        STAGE1_ANCHOR_COHORT, files,
        artifact_sha=cohort_load.artifact_sha256, stored_sha=cohort_load.stored_sha256,
    )
    cohort = cohort_load.parsed
    cohort_keys = validate_cohort_manifest(cohort)
    validate_provenance(cohort["provenance"], role="stage1")

    identity_load = load_canonical_evidence_artifact(member_path(STAGE1_ANCHOR_IDENTITY), RUN_IDENTITY_KIND)
    members[STAGE1_ANCHOR_IDENTITY] = _check_member_shas(
        STAGE1_ANCHOR_IDENTITY, files,
        artifact_sha=identity_load.artifact_sha256, stored_sha=identity_load.stored_sha256,
    )
    identity = identity_load.parsed
    validate_run_identity(identity)

    after = stream_after_artifact(
        member_path(STAGE1_ANCHOR_AFTER), label="Stage 1 D+1 after artifact", side="after",
        keep_keys=set(cohort_keys),
    )
    members[STAGE1_ANCHOR_AFTER] = _check_member_shas(
        STAGE1_ANCHOR_AFTER, files,
        artifact_sha=after.load.artifact_sha256, stored_sha=after.load.stored_sha256,
    )
    after_top = after.load.top
    validate_provenance(after_top["provenance"], role="stage1")

    return Stage1Anchor(
        manifest_path=STAGE1_EVIDENCE_MANIFEST_PATH,
        manifest_sha256=manifest_load.stored_sha256,
        manifest_payload=manifest,
        members={rel: members[rel] for rel in STAGE1_ANCHOR_MEMBERS},
        identity=identity,
        bundle_id=manifest["bundle_id"],
        expected_image_id=manifest["expected_image_id"],
        after_base_commit=after_top["provenance"]["base_commit"],
        after_top=after_top,
        after_keys=after.keys,
        after_row_digests=after.row_digests,
        after_candidate_keys=after.candidate_keys,
        cohort_payload=cohort,
        cohort_keys=cohort_keys,
        cohort_rows=after.kept_rows,
    )


def validate_stage1_anchor_graph(
    anchor: Stage1Anchor,
    *,
    stage2_identity: Mapping[str, Any],
    envcheck_identity: Mapping[str, Any],
    expected_bundle_id: str,
) -> None:
    """跨檔關係（「三之一」第 6、8、9、10 道 ＋ 等式鏈 6-a／6-b）。

    ⚠️ **三個呼叫端（preflight／finalizer／recovery）都要呼叫**——⛔ 第 8、9 道⛔ 不得只留到
    finalizer：cohort 壞掉要跑完數小時 replay 才被發現就太晚了。
    """
    validate_run_identity(stage2_identity)
    validate_run_identity(envcheck_identity)

    if anchor.bundle_id != expected_bundle_id:                                     # 道 6
        raise ArtifactError(
            f"Stage 1 manifest 的 bundle_id={anchor.bundle_id!r} 與 Stage 2 的 "
            f"{expected_bundle_id!r} 不符——⛔ 不得拿別次執行的 evidence 充數"
        )
    cohort_after_sha = anchor.cohort_payload.get("after_artifact_sha256")          # 道 8
    if cohort_after_sha != anchor.members[STAGE1_ANCHOR_AFTER]["artifact_sha256"]:
        raise ArtifactError("Stage 1 cohort 的 after_artifact_sha256 與已錨定的 after 不符")
    assert_same_keys(anchor.cohort_keys, anchor.after_candidate_keys,             # 道 9
                     "Stage 1 cohort vs 已錨定 after 的候選列")

    # 道 10（⚠️ v10 改寫）：Stage 2 identity 必須**逐位元**等於 envcheck 封存的那一份；
    # ⛔ v4～v9 的「等於 Stage 1 archived identity」已不可能成立（Stage 1 的 image 已遺失）。
    if dict(stage2_identity) != dict(envcheck_identity):
        raise ArtifactError(
            "Stage 2 的 run identity 與環境見證封存的那一份⛔ 不完全相同"
            f"（本次={dict(stage2_identity)!r}、envcheck={dict(envcheck_identity)!r}）"
        )
    if stage2_identity["bundle_id"] != anchor.bundle_id:
        raise ArtifactError("Stage 2 identity 的 bundle_id 與 Stage 1 manifest 不符")

    # 6-a：bundle 鏈。⛔ 單檔 validator 只驗型別與 role，擋不住「各檔都合法、鏈條對不起來」。
    bundle_chain = {
        "Stage 1 manifest": anchor.manifest_payload["bundle_id"],
        "Stage 1 identity": anchor.identity["bundle_id"],
        "Stage 1 after": anchor.after_top["bundle_id"],
        "Stage 1 cohort": anchor.cohort_payload["bundle_id"],
        "Stage 2": expected_bundle_id,
    }
    if len(set(bundle_chain.values())) != 1:
        raise ArtifactError(f"bundle_id 等式鏈斷了：{bundle_chain}")
    # 6-b：image 鏈——⚠️ v10 起**只在 Stage 1 內部成立**，⛔ 不再延伸到 Stage 2 的 image。
    image_chain = {
        "Stage 1 manifest": anchor.manifest_payload["expected_image_id"],
        "Stage 1 identity": anchor.identity["expected_image_id"],
        "Stage 1 after": anchor.after_top["provenance"]["image_digest"],
        "Stage 1 cohort": anchor.cohort_payload["provenance"]["image_digest"],
    }
    if len(set(image_chain.values())) != 1:
        raise ArtifactError(f"Stage 1 內部的 image 等式鏈斷了：{image_chain}")


def build_stage1_evidence_ref(anchor: Stage1Anchor) -> dict[str, Any]:
    """Stage 2 manifest 與 envcheck manifest 的 `stage1_evidence` 欄位（⚠️ 同一個定義）。"""
    return {
        "manifest_path": STAGE1_EVIDENCE_MANIFEST_PATH,
        "manifest_sha256": anchor.manifest_sha256,
        "members": {rel: dict(anchor.members[rel]) for rel in STAGE1_ANCHOR_MEMBERS},
    }


def validate_stage1_evidence_ref(ref: object, *, anchor: Stage1Anchor | None = None) -> None:
    """封閉 schema；有 `anchor` 時再比「宣告值 ＝ 本次重算的值」。"""
    if not isinstance(ref, dict) or set(ref) != _STAGE1_REF_FIELDS:
        raise ArtifactError(f"stage1_evidence 的欄位集合必須恰好是 {sorted(_STAGE1_REF_FIELDS)}")
    if ref["manifest_path"] != STAGE1_EVIDENCE_MANIFEST_PATH:
        raise ArtifactError(
            f"stage1_evidence.manifest_path 必須等於寫死常數 {STAGE1_EVIDENCE_MANIFEST_PATH!r}"
        )
    if not is_hex64(ref["manifest_sha256"]):
        raise ArtifactError("stage1_evidence.manifest_sha256 必須是 64 字元小寫 hex")
    members = ref["members"]
    if not isinstance(members, dict) or set(members) != set(STAGE1_ANCHOR_MEMBERS):
        raise ArtifactError(f"stage1_evidence.members 的 key 必須恰好是 {list(STAGE1_ANCHOR_MEMBERS)}")
    for rel, entry in members.items():
        if not isinstance(entry, dict) or set(entry) != _MEMBER_FIELDS:
            raise ArtifactError(f"stage1_evidence.members[{rel}] 的欄位必須恰好是 {sorted(_MEMBER_FIELDS)}")
        if not all(is_hex64(entry[f]) for f in _MEMBER_FIELDS):
            raise ArtifactError(f"stage1_evidence.members[{rel}] 的 SHA 必須是 64 字元小寫 hex")
    if anchor is not None and ref != build_stage1_evidence_ref(anchor):
        raise ArtifactError("stage1_evidence 的宣告值與本次重算的 Stage 1 信任錨不符")


def validate_file_entry(rel: str, entry: object) -> None:
    """`canonical_artifact` entry：`artifact_sha256`／`stored_sha256`／`stored_bytes`。"""
    if not isinstance(entry, dict) or set(entry) != FILE_ENTRY_FIELDS:
        raise ArtifactError(f"files[{rel}] 的欄位集合必須恰好是 {sorted(FILE_ENTRY_FIELDS)}")
    for name in ("artifact_sha256", "stored_sha256"):
        if not is_hex64(entry[name]):
            raise ArtifactError(f"files[{rel}].{name} 必須是 64 字元小寫 hex")
    size = entry["stored_bytes"]
    if type(size) is not int or size <= 0:
        raise ArtifactError(f"files[{rel}].stored_bytes 必須是正整數：{size!r}")


# ── 封閉 layout 的 archive 核心 ─────────────────────────────────────────────

class ClosedArchiveWriter:
    """整包原子發布：sibling staging 建完整 tree → `rename_noreplace()` → fsync parent。

    ⚠️ **只接受 layout 內的相對路徑**，每個路徑恰好寫一次；commit 時檔案集合必須**恰好**
    等於 layout——⛔ 多一個、少一個都中止。
    ⚠️ **rename 之前的任何失敗**：只清 staging，正式路徑仍不存在（`with` 區塊自動處理）。
    ⚠️ **rename 成功⛔ 不等於落盤**：之後 parent fsync 失敗時⛔ 不刪除，拋
    `DurabilityUnconfirmed`（結束碼 3），由呼叫端提示對應的 recovery 模式。
    """

    def __init__(self, root: str | Path, layout: tuple[str, ...], *, recover_hint: str) -> None:
        self.root = Path(root)
        self.layout = tuple(layout)
        self.recover_hint = recover_hint
        if self.root.exists():
            raise ArtifactError(
                f"archive root 已存在：{self.root}——⛔ 不覆蓋。（durability 未確認時請用 {recover_hint}）"
            )
        parent = self.root.parent
        parent.mkdir(parents=True, exist_ok=True)
        # ⚠️ 正式發布前先實測 no-clobber rename 的**目錄**語意。
        probe_no_clobber(parent)
        self.staging = parent / f".{self.root.name}.staging-{secrets.token_hex(8)}"
        self.staging.mkdir()
        self.files: dict[str, dict[str, Any]] = {}
        self._committed = False

    def __enter__(self) -> "ClosedArchiveWriter":
        return self

    def __exit__(self, exc_type, exc, tb) -> None:
        if not self._committed:
            remove_tree(self.staging)

    def path_for(self, rel: str) -> Path:
        """staging 內 `rel` 的路徑（⚠️ 先驗 layout 與「只寫一次」，並建好父目錄）。"""
        if rel not in self.layout:
            raise ArtifactError(f"{rel} 不在封閉 layout 內：{list(self.layout)}")
        if rel in self.files:
            raise ArtifactError(f"{rel} 已經寫過——⛔ 每個路徑只寫一次")
        target = self.staging / rel
        target.parent.mkdir(parents=True, exist_ok=True)
        return target

    def add_canonical(self, rel: str, payload: dict[str, Any]) -> dict[str, Any]:
        """小型 payload：整份編碼成 canonical gzip。⚠️ 三項 metadata 來自同一次寫入的 bytes。"""
        target = self.path_for(rel)
        raw = canonical_json_bytes(payload)
        blob = canonical_gzip_bytes(raw)
        target.write_bytes(blob)
        entry = {"artifact_sha256": sha256_hex(raw), "stored_sha256": sha256_hex(blob),
                 "stored_bytes": len(blob)}
        self.files[rel] = entry
        return entry

    def record_streamed(self, rel: str, load: StreamLoad) -> dict[str, Any]:
        """全量 payload：`stream_canonical_artifact(copy_to=path_for(rel))` 在**驗證的同一趟**寫出的副本。"""
        if rel not in self.layout or rel in self.files:
            raise ArtifactError(f"{rel} 不在 layout 內或已經寫過")
        if load.copy_sha256 is None or load.copy_bytes is None:
            raise ArtifactError(f"{rel}：沒有同一趟寫出的副本——⛔ 不得事後另讀一次補寫")
        if not (self.staging / rel).is_file():
            raise ArtifactError(f"{rel}：staging 內找不到同一趟寫出的副本")
        entry = {"artifact_sha256": load.artifact_sha256, "stored_sha256": load.copy_sha256,
                 "stored_bytes": load.copy_bytes}
        self.files[rel] = entry
        return entry

    def commit(self, manifest_name: str, manifest: dict[str, Any], *, verify=None) -> None:
        """寫 manifest →（`verify(staging)`：**對 staging 跑與 recovery 相同的完整驗證**）→ rename。

        ⚠️ `verify` 排在 rename **之前**：驗的是真正要發布的那一棵 tree（含 manifest），
        ⛔ 不是記憶體裡的物件。
        """
        if set(self.files) != set(self.layout):
            raise ArtifactError(
                f"archive 的檔案集合不符 layout：缺={sorted(set(self.layout) - set(self.files))}"
            )
        (self.staging / manifest_name).write_bytes(canonical_json_bytes(manifest))
        if verify is not None:
            verify(self.staging)
        _fsync_tree(self.staging)
        rename_noreplace(self.staging, self.root)          # ← ⚠️ 唯一的 commit point
        self._committed = True
        fsync_parent_or_unconfirmed(self.root, published=True, recover_hint=self.recover_hint)


def fsync_parent_or_unconfirmed(root: Path, *, published: bool, recover_hint: str) -> None:
    """⚠️ **commit point 之後**：parent fsync 失敗⛔ 不刪除，回報 durability 未確認。"""
    try:
        fsync_dir(Path(root).parent)
    except OSError as exc:
        raise DurabilityUnconfirmed(
            f"archive {root} " + ("已發布" if published else "既有那份有效")
            + f"，但 {Path(root).parent} 的 fsync 失敗（{exc}）——durability 未確認。"
            f"⛔ 不刪除、不重產：用 {recover_hint} 重新驗證並 fsync。",
            published=published,
        ) from exc


def archive_file_set(root: Path) -> set[str]:
    """archive 內實際的檔案（相對路徑）。⚠️ symlink 一律拒絕。"""
    out: set[str] = set()
    for path in root.rglob("*"):
        if path.is_symlink():
            raise ArtifactError(f"archive 內⛔ 不得有 symlink：{path}")
        if path.is_file():
            out.add(path.relative_to(root).as_posix())
    return out
