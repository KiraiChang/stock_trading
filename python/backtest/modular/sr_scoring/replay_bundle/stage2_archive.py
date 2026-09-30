"""I-074 Stage 2 的正式證據（③d）：成功 archive、failed-attempt record 與 check／recover。

規格見 issue.md I-074「③ Stage 2 evidence contract 計畫書」（⚠️ 現行版）的「三」～「七之四」。

* **成功 archive**（`python/baselines/i074_stage2/evidence/`，封閉 7 檔）：before source 全量、
  comparison、report、兩份 raw patch、Stage 2 identity ＋ manifest；`verify_stage2_graph()` 的
  十六道由 finalize（對 staging）與 recovery **共用同一段程式**；
* **failed-attempt record**（`failed/<bundle_id>-<counterfactual_semantic_sha256>/`）：反事實沒有生效
  （「①之三」的檢查順序得出 `candidate_flag_inconsistent` 或 `rr_not_restored`）時的事故紀錄，
  ⛔ 不是成功 archive；⚠️ 目錄鍵是**語意 SHA**（只涵蓋兩個產品檔的 canonical diff，⑦ 總綱 v1 決策表
  第 9 列）——只改測試檔⛔ 換不了鍵；語意 SHA 要用 git 算，由 shell 以
  `--verified-counterfactual-semantic-sha256` 交給本模組；
* check／recover 的 Python 段——⚠️ **合成守門（F8-a、「四之二」）在 shell**
  （`scripts/finalize-stage2-evidence.sh`），⛔ 本模組不碰 git；shell 把驗過的四個值以 `--verified-*`
  交給本模組（`VerifiedComposition`），commit point／fsync 之前必須與**實際要封存的內容**相符
  （2026-09-24 review：兩段之間輸入被換掉的 TOCTOU）。

⚠️ **為什麼⛔ 放進 `stage2_evidence.py`**：`envcheck.py` import 它（③b），而這裡要用 `envcheck.py`
的 E1～E7——放同一檔會循環 import；延後到函式內 import 又會踩到「階段 B 之後⛔ 不得有新的
project import」。所以共用核心留在 `stage2_evidence.py`，③d 放本檔。

⚠️ **記憶體**（「五之一」）：全量的 before source 一律串流，只常駐 keys 與 cohort 那幾列的完整 row；
⛔ 不得同時持有兩份全量 rows。
"""
from __future__ import annotations

import bisect
import re
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Mapping, Sequence

from .artifacts import (
    ARTIFACT_SCHEMA_VERSION,
    CANDIDATE_FIELD,
    COMPARISON_KIND,
    REPORT_KIND,
    ArtifactError,
    StreamRowValidator,
    load_canonical_evidence_artifact,
    validate_comparison_artifact,
    validate_report,
)
from .canonical import sha256_hex
from .envcheck import (
    ENVCHECK_IDENTITY,
    ENVCHECK_LAYOUT,
    ENVCHECK_MANIFEST_NAME,
    ENVCHECK_ROOT_PATH,
    OUTCOME_EQUIVALENT,
    EnvcheckResult,
    verify_envcheck,
)
from .evidence import RECOVERY_IDENTITY_FIELDS, _fsync_tree
from .provenance import validate_provenance
from .publish import DurabilityUnconfirmed
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
from .stream import StreamLoad, stream_canonical_artifact

# ── 常數 ────────────────────────────────────────────────────────────────────

# ⚠️ 全部寫死常數（repo 相對），⛔ 不是執行期參數。
STAGE2_EVIDENCE_ROOT_PATH = "python/baselines/i074_stage2/evidence"
STAGE2_FAILED_ROOT_PATH = "python/baselines/i074_stage2/failed"
ENVCHECK_MANIFEST_PATH = f"{ENVCHECK_ROOT_PATH}/{ENVCHECK_MANIFEST_NAME}"

STAGE2_MANIFEST_NAME = "evidence_manifest.json"
STAGE2_MANIFEST_KIND = "sr_zone_stage2_evidence_manifest"
BEFORE_SOURCE_KIND = "sr_zone_stage2_before_source"
FAILED_ATTEMPT_KIND = "sr_zone_stage2_failed_attempt"
# replay 在反事實沒有生效（結束碼 6）時寫出的中繼檔（⚠️ ③d 訂定，見 `validate_counterfactual_failure()`）。
COUNTERFACTUAL_FAILURE_KIND = "sr_zone_stage2_counterfactual_failure"

BEFORE_SOURCE = "before/before_source_artifact.json.gz"
COMPARISON = "comparison/comparison_artifact.json.gz"
REPORT = "comparison/report.json.gz"
COUNTERFACTUAL_PATCH = "patch/counterfactual.patch"
TOOLING_PATCH = "patch/tooling.patch"
IDENTITY = "identity/run_identity.json.gz"
# ⚠️ **封閉 layout**：6 個 payload ＋ manifest ＝ 7 檔。⚠️ entry 的型別**由這裡決定**，⛔ 不由 manifest 宣告
# ——否則竄改者可以把 `.json.gz` 宣告成 `raw_blob` 來跳過 canonical 驗證。
CANONICAL_MEMBERS = (BEFORE_SOURCE, COMPARISON, REPORT, IDENTITY)
RAW_MEMBERS = (COUNTERFACTUAL_PATCH, TOOLING_PATCH)
STAGE2_LAYOUT = (BEFORE_SOURCE, COMPARISON, REPORT, COUNTERFACTUAL_PATCH, TOOLING_PATCH, IDENTITY)

FAILED_RECORD_NAME = "failure_record.json"
FAILED_LAYOUT = RAW_MEMBERS

# operational 輸入（⚠️ 一律由 orchestrator 的 run 目錄固定推導，⛔ 不開逐檔覆寫）。
OPERATIONAL_BEFORE_SOURCE = "stage2/before_source_artifact.json"
OPERATIONAL_COMPARISON = "stage2/comparison_artifact.json"
OPERATIONAL_REPORT = "stage2/report.json"
OPERATIONAL_FAILURE = "stage2/bounded_diagnostics.json"
FROZEN_COUNTERFACTUAL_PATCH = "patches/counterfactual.patch"
FROZEN_TOOLING_PATCH = "patches/tooling.patch"

# ⚠️ 順序即套用順序，由此**強制**——⛔ 不靠「hash 必然不同」（改不同檔案時交換順序得到同一個 tree）。
ORDERED_COMPONENTS = ("counterfactual", "tooling")
# report 的截斷上限。⚠️ 它是 bundle manifest 的 `report_max_rows`（實查＝200）；Stage 2 archive 不含
# bundle manifest，所以寫死，並由測試斷言它等於該 bundle 的值（測試 bj）。
STAGE2_REPORT_MAX_ROWS = 200
# ⚠️ v3 模型下恆為 0——⛔ 仍要寫進 manifest、recovery ⛔ 仍要讀回並回傳，⛔ 不得寫死在回傳路徑。
STAGE2_TERMINAL_OUTCOMES = (0,)

FAILURE_CANDIDATE_FLAG_INCONSISTENT = "candidate_flag_inconsistent"
FAILURE_RR_NOT_RESTORED = "rr_not_restored"
# ⚠️ **封閉列舉，恰好兩個值**；優先序由 Stage 2 計畫書「①之三」的檢查順序決定。
FAILURE_REASONS = (FAILURE_CANDIDATE_FLAG_INCONSISTENT, FAILURE_RR_NOT_RESTORED)
FAILURE_COUNT_FIELD = {
    FAILURE_CANDIDATE_FLAG_INCONSISTENT: "inconsistent_row_count",
    FAILURE_RR_NOT_RESTORED: "before_candidate_count",
}
SAMPLE_KEY_FIELDS = frozenset({"symbol", "timeframe", "as_of", "lifecycle_phase",
                               "setup_rr_qualified", CANDIDATE_FIELD})

RECOVER_DURABILITY_HINT = "scripts/finalize-stage2-evidence.sh --recover-durability"
RECOVER_FAILED_HINT = "scripts/finalize-stage2-evidence.sh --recover-failed-record"

_BEFORE_SOURCE_FIELDS = frozenset({"schema_version", "kind", "bundle_id", "before_ref", "timeframe",
                                   "replay_scope", "generated_at", "provenance", "rows"})
_MANIFEST_FIELDS = frozenset({"schema_version", "kind", "bundle_id", "expected_image_id", "files", "patches",
                              "stage1_evidence", "environment_witness", "terminal_outcome",
                              "generated_at", "finalizer_provenance"})
_PATCHES_FIELDS = frozenset({"counterfactual_patch_sha256", "tooling_patch_sha256", "composed_sha256",
                             "ordered_components"})
RAW_BLOB_FIELDS = frozenset({"stored_sha256", "stored_bytes"})
_WITNESS_REF_FIELDS = frozenset({"manifest_path", "manifest_sha256", "members"})
_MEMBER_FIELDS = frozenset({"artifact_sha256", "stored_sha256"})
_FAILED_RECORD_FIELDS = frozenset({"schema_version", "kind", "bundle_id", "expected_image_id", "run_identity",
                                   "patches", "files", "bounded_diagnostics", "failure_reason",
                                   "generated_at", "provenance", "counterfactual_semantic_sha256"})
_FAILURE_SOURCE_FIELDS = frozenset({"schema_version", "kind", "bundle_id", "generated_at", "provenance",
                                    "failure_reason", "bounded_diagnostics"})
_FAILED_DIR_RE = re.compile(r"^(?P<bundle>[^/]+)-(?P<sha>[0-9a-f]{64})$")

# 第 7 道：provenance 的位置與 role **逐一列舉**——⛔ 不用 `.get("provenance") or …` 通用推測
# （Stage 1 的教訓：那樣會整份漏掉 `comparator_provenance`）。沒有 provenance 的是 report
# （靠 `comparison_artifact_sha256` 綁回 comparison）、identity（身分宣告）與兩份 raw patch。
PROVENANCE_MAP = (
    (BEFORE_SOURCE, "provenance", "stage1"),
    (COMPARISON, "provenance", "stage1"),
    (STAGE2_MANIFEST_NAME, "finalizer_provenance", "finalizer"),
)


def _key_tuple(item: Mapping[str, Any]) -> tuple[str, str, str]:
    return (item["symbol"], item["timeframe"], item["as_of"])


# ── 「①之三」：反事實是否真的生效（執行期與證據層共用） ─────────────────────

class BoundedSample:
    """**計數完整、sample 有界**：只保留依 `(symbol, timeframe, as_of)` 排序最前面的 `limit` 筆。

    ⚠️ 2026-09-24 review：原本把每一筆違規都留下、最後才排序截斷——**輸出有界、計算過程卻無界**，
    「大量列同時違規」時記憶體隨違規列數成長，⛔ 與「五之一」的串流記憶體模型不符。
    這裡的 buffer 任何時刻都⛔ 超過 `limit` 筆（插入後立即丟掉最大的那一筆）。
    ⚠️ 以 `(key, 序號)` 排序：序號唯一，⛔ 永遠不會拿 sample 本身（dict）來比大小。
    """

    def __init__(self, limit: int = DIAGNOSTIC_SAMPLE_LIMIT) -> None:
        self.limit = limit
        self.count = 0
        self._buffer: list[tuple[tuple[str, str, str], int, dict[str, Any]]] = []

    def add(self, key: tuple[str, str, str], sample: dict[str, Any]) -> None:
        self.count += 1
        if len(self._buffer) >= self.limit and key >= self._buffer[-1][0]:
            return
        bisect.insort(self._buffer, (key, self.count, sample))
        if len(self._buffer) > self.limit:
            self._buffer.pop()

    def __len__(self) -> int:
        """目前 buffer 內的筆數（⛔ 不是完整計數；完整計數是 `count`）。"""
        return len(self._buffer)

    def samples(self) -> list[dict[str, Any]]:
        return [sample for _key, _seq, sample in self._buffer]


class CounterfactualIneffective(Exception):
    """反事實 replay 的 counterfactual patch **沒有生效**（結束碼 6）。

    ⛔ **刻意不繼承 `ValueError`**（比照 `CandidateMismatch`）：evaluation 的 CLI 以
    `except (CliUsageError, ValueError, OSError) → sys.exit(1)` 統一收斂，繼承下去的話專屬碼出不來。
    ⚠️ 拋出之前，`bounded_diagnostics.json` 必須已經發布（`path` 指向它）。
    """

    def __init__(self, message: str, *, path) -> None:
        super().__init__(message)
        self.path = path


class CounterfactualEffectCheck:
    """Stage 2 計畫書「①之三」的兩條不變條件與**檢查順序**。

    ⚠️ `before_candidates == ∅` 單獨成立⛔ 證明不了 RR 真的加回去：flag 若恆為 `False`、而 RR 其實
    沒加回 `CONTINUATION`，那道守門會空洞通過。所以固定順序驗：

    1. 每列 `rr_decoupling_candidate == (lifecycle_phase == "CONTINUATION" and not setup_rr_qualified)`
       ——不成立 → `candidate_flag_inconsistent`（flag 不可信，由它算的候選集合也不可信）；
    2. 第 1 條成立後，`before_candidates == ∅`（此時等同「沒有任何一列是 CONTINUATION 且
       setup_rr_qualified == false」）——不成立 → `rr_not_restored`。

    ⚠️ 只能餵**已通過 row-level 原語**的列（型別由 `StreamRowValidator` 保證）。
    ⚠️ sample 依 `(symbol, timeframe, as_of)` 排序取前 `DIAGNOSTIC_SAMPLE_LIMIT` 個；完整計數照樣保留。
    ⚠️ 兩種違規各自一個 `BoundedSample`：**計算過程**的記憶體也有界（⛔ 不是只有輸出有界）。
    """

    def __init__(self) -> None:
        self.inconsistent = BoundedSample()
        self.candidates = BoundedSample()

    @staticmethod
    def _sample(row: Mapping[str, Any]) -> dict[str, Any]:
        return {f: row[f] for f in ("symbol", "timeframe", "as_of", "lifecycle_phase",
                                    "setup_rr_qualified", CANDIDATE_FIELD)}

    def feed(self, row: Mapping[str, Any]) -> None:
        expected = bool(row.get("lifecycle_phase") == "CONTINUATION" and not row["setup_rr_qualified"])
        if row[CANDIDATE_FIELD] is not expected:
            self.inconsistent.add(_key_tuple(row), self._sample(row))
        elif row[CANDIDATE_FIELD] is True:
            self.candidates.add(_key_tuple(row), self._sample(row))

    def result(self) -> dict[str, Any] | None:
        """生效時回 `None`；否則回 `{failure_reason, bounded_diagnostics}`。"""
        if self.inconsistent.count:
            reason, bucket = FAILURE_CANDIDATE_FLAG_INCONSISTENT, self.inconsistent
        elif self.candidates.count:
            reason, bucket = FAILURE_RR_NOT_RESTORED, self.candidates
        else:
            return None
        return {
            "failure_reason": reason,
            "bounded_diagnostics": {
                FAILURE_COUNT_FIELD[reason]: bucket.count,
                "sample_keys": bucket.samples(),
            },
        }


def validate_bounded_diagnostics(failure_reason: object, diagnostics: object) -> None:
    """F5～F7、F10：依 `failure_reason` 分流的封閉 union。"""
    if not isinstance(failure_reason, str) or failure_reason not in FAILURE_REASONS:     # F5
        raise ArtifactError(f"F5：failure_reason 只接受 {FAILURE_REASONS}：{failure_reason!r}")
    count_field = FAILURE_COUNT_FIELD[failure_reason]
    want = {count_field, "sample_keys"}
    if not isinstance(diagnostics, dict) or set(diagnostics) != want:                    # F6
        actual = sorted(diagnostics) if isinstance(diagnostics, dict) else diagnostics
        raise ArtifactError(
            f"F6：failure_reason={failure_reason} 的 bounded_diagnostics 欄位必須恰好是 {sorted(want)}，實際 {actual!r}"
        )
    count = diagnostics[count_field]
    if type(count) is not int or count <= 0:                                             # F6-a
        raise ArtifactError(f"F6-a：{count_field} 必須是正整數：{count!r}")
    samples = diagnostics["sample_keys"]
    if not isinstance(samples, list) or len(samples) != min(DIAGNOSTIC_SAMPLE_LIMIT, count):
        raise ArtifactError(
            f"F6-a：sample_keys 的長度必須恰好是 min({DIAGNOSTIC_SAMPLE_LIMIT}, {count})"
        )
    keys = []
    for item in samples:
        if not isinstance(item, dict) or set(item) != SAMPLE_KEY_FIELDS:                 # F6-b
            raise ArtifactError(f"F6-b：sample_keys[] 的欄位集合必須恰好是 {sorted(SAMPLE_KEY_FIELDS)}")
        for f in ("symbol", "timeframe", "as_of", "lifecycle_phase"):                    # F6-c
            if not isinstance(item[f], str) or not item[f]:
                raise ArtifactError(f"F6-c：sample_keys[].{f} 必須是非空字串：{item[f]!r}")
        for f in ("setup_rr_qualified", CANDIDATE_FIELD):                                # F7
            if not isinstance(item[f], bool):
                raise ArtifactError(f"F7：sample_keys[].{f} 必須是嚴格 boolean：{item[f]!r}")
        continuation = item["lifecycle_phase"] == "CONTINUATION"
        if failure_reason == FAILURE_CANDIDATE_FLAG_INCONSISTENT:                       # F10
            violates = item[CANDIDATE_FIELD] != (continuation and not item["setup_rr_qualified"])
        else:
            violates = continuation and item["setup_rr_qualified"] is False and item[CANDIDATE_FIELD] is True
        if not violates:
            raise ArtifactError(
                f"F10：sample {_key_tuple(item)} 沒有違反 {failure_reason} 宣稱的那一條——⛔ 不接受無關的列"
            )
        keys.append(_key_tuple(item))
    if any(a >= b for a, b in zip(keys, keys[1:])):
        raise ArtifactError("F6-a：sample_keys 必須依 (symbol, timeframe, as_of) 排序且⛔ 不重複")


# ── before source artifact（「三之二」①） ───────────────────────────────────

def build_before_source(*, bundle_id: str, before_ref: str, timeframe: str, replay_scope: str,
                        generated_at: str, provenance: dict[str, Any],
                        rows: list[dict[str, Any]]) -> dict[str, Any]:
    """⚠️ **新 kind**，⛔ 不冒用 `AFTER_KIND`（那是 after 側的語意）。⛔ 不截斷。"""
    return {
        "schema_version": ARTIFACT_SCHEMA_VERSION, "kind": BEFORE_SOURCE_KIND,
        "bundle_id": bundle_id, "before_ref": before_ref, "timeframe": timeframe,
        "replay_scope": replay_scope, "generated_at": generated_at, "provenance": provenance,
        "rows": rows,
    }


def validate_before_source_envelope(top: Mapping[str, Any]) -> None:
    """頂層封閉欄位（串流時 `rows` 由呼叫端補佔位）。"""
    if set(top) != _BEFORE_SOURCE_FIELDS:
        raise ArtifactError(
            "before source 的欄位集合不符："
            f"多={sorted(set(top) - _BEFORE_SOURCE_FIELDS)}、缺={sorted(_BEFORE_SOURCE_FIELDS - set(top))}"
        )
    if type(top["schema_version"]) is not int or top["schema_version"] != ARTIFACT_SCHEMA_VERSION:
        raise ArtifactError(f"未知的 before source schema_version={top['schema_version']!r}")
    if top["kind"] != BEFORE_SOURCE_KIND:
        raise ArtifactError(f"before source 的 kind={top['kind']!r}，預期 {BEFORE_SOURCE_KIND!r}")
    for f in ("bundle_id", "before_ref", "timeframe", "replay_scope"):
        if not isinstance(top[f], str) or not top[f]:
            raise ArtifactError(f"before source 的 {f} 必須是非空字串")
    validate_generated_at(top["generated_at"], "before source")
    validate_provenance(top["provenance"], role="stage1")


@dataclass(frozen=True)
class BeforeStream:
    """一份 before source 的串流摘要。⚠️ 只常駐 keys、候選 keys 與指定要留的完整列。"""

    load: StreamLoad
    keys: list[tuple[str, str, str]]
    candidate_keys: list[tuple[str, str, str]]
    kept_rows: dict[tuple[str, str, str], dict[str, Any]]
    effect: dict[str, Any] | None   # `CounterfactualEffectCheck.result()`


def stream_before_source(path: str | Path, *, keep_keys=frozenset(),
                         copy_to: str | Path | None = None) -> BeforeStream:
    """串流讀 before source：row-level 的**沿用三道**（`assert_unique_keys()`／`validate_replay_errors()`／
    `validate_diagnostics(side="before")`，由 `StreamRowValidator` 以同一組原語逐列套用）＋「①之三」。

    ⚠️ `side="before"` 的 `validate_diagnostics()` 刻意**不驗等價式**（Stage 0 已驗收的公開行為，⛔ 不改）；
    等價式由 `CounterfactualEffectCheck` 在這裡另外驗——⛔ 不是只看 flag。
    """
    validator = StreamRowValidator("before source", side="before")
    effect = CounterfactualEffectCheck()
    kept: dict[tuple[str, str, str], dict[str, Any]] = {}
    keep = frozenset(keep_keys)

    def on_row(row: Any, _row_bytes: bytes) -> None:
        key = validator.feed(row)
        effect.feed(row)
        if key in keep:
            kept[key] = row

    load = stream_canonical_artifact(path, BEFORE_SOURCE_KIND, on_row=on_row, copy_to=copy_to)
    try:
        validator.finish()
        validate_before_source_envelope(load.top | {"rows": []})
    except BaseException:
        if copy_to is not None:
            Path(copy_to).unlink(missing_ok=True)
        raise
    return BeforeStream(load=load, keys=validator.keys, candidate_keys=validator.candidate_keys,
                        kept_rows=kept, effect=effect.result())


# ── patches 與 raw blob（「四」「四之一」） ─────────────────────────────────

def validate_raw_blob_entry(rel: str, entry: object) -> None:
    """`raw_blob` entry：只有 `stored_sha256`／`stored_bytes`（⛔ 沒有 `artifact_sha256`）。

    ⚠️ **只有 `patch/tooling.patch` 允許 0 bytes**——空的反事實 patch ＝ 根本沒做反事實。
    """
    if not isinstance(entry, dict) or set(entry) != RAW_BLOB_FIELDS:
        raise ArtifactError(f"files[{rel}] 是 raw_blob，欄位集合必須恰好是 {sorted(RAW_BLOB_FIELDS)}")
    if not is_hex64(entry["stored_sha256"]):
        raise ArtifactError(f"files[{rel}].stored_sha256 必須是 64 字元小寫 hex")
    size = entry["stored_bytes"]
    if type(size) is not int or size < 0:
        raise ArtifactError(f"files[{rel}].stored_bytes 必須是非負整數：{size!r}")
    if size == 0 and rel != TOOLING_PATCH:
        raise ArtifactError(f"files[{rel}] ⛔ 不得為 0 bytes——空的反事實 patch 等於根本沒做反事實")
    if size == 0 and entry["stored_sha256"] != EMPTY_SHA256:
        raise ArtifactError(f"files[{rel}] 是 0 bytes，stored_sha256 必須是空字串的 SHA")


def validate_patches(patches: object) -> None:
    """`patches` 的封閉 schema ＋ 第 4 條（`ordered_components` 恰好是固定順序）。"""
    if not isinstance(patches, dict) or set(patches) != _PATCHES_FIELDS:
        raise ArtifactError(f"patches 的欄位集合必須恰好是 {sorted(_PATCHES_FIELDS)}")
    for f in ("counterfactual_patch_sha256", "tooling_patch_sha256", "composed_sha256"):
        if not is_hex64(patches[f]):
            raise ArtifactError(f"patches.{f} 必須是 64 字元小寫 hex（⛔ 不得省略或寫 null）")
    if patches["ordered_components"] != list(ORDERED_COMPONENTS):
        raise ArtifactError(
            f"patches.ordered_components 必須恰好是 {list(ORDERED_COMPONENTS)}——順序即套用順序，由此強制"
        )
    # tooling 為空時 T2 ＝ T1，合成 diff 必然等於 counterfactual 的 diff。
    if patches["tooling_patch_sha256"] == EMPTY_SHA256 and patches["composed_sha256"] != patches["counterfactual_patch_sha256"]:
        raise ArtifactError("tooling patch 為空時 composed_sha256 必須等於 counterfactual_patch_sha256")


def check_patch_bindings(files: Mapping[str, Any], patches: Mapping[str, Any]) -> None:
    """「四之一」第 1、2 條：raw patch 的 `stored_sha256` 等於 `patches` 的宣告值。"""
    if files[COUNTERFACTUAL_PATCH]["stored_sha256"] != patches["counterfactual_patch_sha256"]:
        raise ArtifactError("files[patch/counterfactual.patch].stored_sha256 ≠ patches.counterfactual_patch_sha256")
    if files[TOOLING_PATCH]["stored_sha256"] != patches["tooling_patch_sha256"]:
        raise ArtifactError("files[patch/tooling.patch].stored_sha256 ≠ patches.tooling_patch_sha256")


def _read_raw_member(root: Path, rel: str, entry: Mapping[str, Any]) -> bytes:
    """讀 raw blob **一次**，重算 SHA 與長度並與 entry 比對（F8、recovery 的 metadata）。"""
    path = root / rel
    if path.is_symlink() or not path.is_file():
        raise ArtifactError(f"{rel} 必須是一般檔案")
    blob = path.read_bytes()
    if (sha256_hex(blob), len(blob)) != (entry["stored_sha256"], entry["stored_bytes"]):
        raise ArtifactError(f"{rel} 的實際 bytes 與宣告的 stored_sha256／stored_bytes 不符")
    return blob


# ── 環境見證錨的宣告值（「四」的 `environment_witness`） ─────────────────────

def build_environment_witness_ref(envcheck: EnvcheckResult) -> dict[str, Any]:
    files = envcheck.manifest["files"]
    return {
        "manifest_path": ENVCHECK_MANIFEST_PATH,
        "manifest_sha256": envcheck.manifest_sha256,
        "members": {rel: {"artifact_sha256": files[rel]["artifact_sha256"],
                          "stored_sha256": files[rel]["stored_sha256"]} for rel in ENVCHECK_LAYOUT},
    }


def validate_environment_witness_ref(ref: object, *, envcheck: EnvcheckResult | None = None) -> None:
    """封閉 schema；有 `envcheck` 時再比「宣告值 ＝ 本次重新驗證的環境見證錨」。"""
    if not isinstance(ref, dict) or set(ref) != _WITNESS_REF_FIELDS:
        raise ArtifactError(f"environment_witness 的欄位集合必須恰好是 {sorted(_WITNESS_REF_FIELDS)}")
    if ref["manifest_path"] != ENVCHECK_MANIFEST_PATH:
        raise ArtifactError(f"environment_witness.manifest_path 必須等於寫死常數 {ENVCHECK_MANIFEST_PATH!r}")
    if not is_hex64(ref["manifest_sha256"]):
        raise ArtifactError("environment_witness.manifest_sha256 必須是 64 字元小寫 hex")
    members = ref["members"]
    if not isinstance(members, dict) or set(members) != set(ENVCHECK_LAYOUT):
        raise ArtifactError(f"environment_witness.members 的 key 必須恰好是 {list(ENVCHECK_LAYOUT)}")
    for rel, entry in members.items():
        if not isinstance(entry, dict) or set(entry) != _MEMBER_FIELDS or not all(
                is_hex64(entry[f]) for f in _MEMBER_FIELDS):
            raise ArtifactError(f"environment_witness.members[{rel}] 必須恰好是兩個 64 字元小寫 hex 的 SHA")
    if envcheck is not None and ref != build_environment_witness_ref(envcheck):
        raise ArtifactError("environment_witness 的宣告值與本次重新驗證的環境見證錨不符")


# ── Stage 2 manifest（「四」） ───────────────────────────────────────────────

def build_stage2_manifest(
    *, bundle_id: str, expected_image_id: str, files: Mapping[str, Mapping[str, Any]],
    patches: dict[str, Any], stage1_evidence: dict[str, Any], environment_witness: dict[str, Any],
    terminal_outcome: int, generated_at: str, finalizer_provenance: dict[str, Any],
) -> dict[str, Any]:
    return {
        "schema_version": ARTIFACT_SCHEMA_VERSION,
        "kind": STAGE2_MANIFEST_KIND,
        "bundle_id": bundle_id,
        "expected_image_id": expected_image_id,
        "files": {rel: dict(files[rel]) for rel in sorted(files)},
        "patches": patches,
        "stage1_evidence": stage1_evidence,
        "environment_witness": environment_witness,
        "terminal_outcome": terminal_outcome,
        "generated_at": generated_at,
        "finalizer_provenance": finalizer_provenance,
    }


def validate_stage2_manifest(manifest: object) -> None:
    """封閉 schema。⛔ 多欄、缺欄、型別不符一律中止。"""
    if not isinstance(manifest, dict) or set(manifest) != _MANIFEST_FIELDS:
        actual = set(manifest) if isinstance(manifest, dict) else set()
        raise ArtifactError(
            "Stage 2 manifest 的欄位集合不符："
            f"多={sorted(actual - _MANIFEST_FIELDS)}、缺={sorted(_MANIFEST_FIELDS - actual)}"
        )
    if type(manifest["schema_version"]) is not int or manifest["schema_version"] != ARTIFACT_SCHEMA_VERSION:
        raise ArtifactError(f"未知的 Stage 2 manifest schema_version={manifest['schema_version']!r}")
    if manifest["kind"] != STAGE2_MANIFEST_KIND:
        raise ArtifactError(f"Stage 2 manifest 的 kind={manifest['kind']!r}，預期 {STAGE2_MANIFEST_KIND!r}")
    if not isinstance(manifest["bundle_id"], str) or not manifest["bundle_id"]:
        raise ArtifactError("Stage 2 manifest 的 bundle_id 必須是非空字串")
    if not is_image_id(manifest["expected_image_id"]):
        raise ArtifactError("Stage 2 manifest 的 expected_image_id 必須是 sha256: ＋ 64 字元小寫 hex")
    files = manifest["files"]
    if not isinstance(files, dict) or set(files) != set(STAGE2_LAYOUT):
        actual = set(files) if isinstance(files, dict) else set()
        raise ArtifactError(
            f"Stage 2 manifest 的 files 必須恰好是 layout 的 {len(STAGE2_LAYOUT)} 項："
            f"多={sorted(actual - set(STAGE2_LAYOUT))}、缺={sorted(set(STAGE2_LAYOUT) - actual)}"
        )
    for rel in CANONICAL_MEMBERS:
        validate_file_entry(rel, files[rel])
    for rel in RAW_MEMBERS:
        validate_raw_blob_entry(rel, files[rel])
    validate_patches(manifest["patches"])
    check_patch_bindings(files, manifest["patches"])
    validate_stage1_evidence_ref(manifest["stage1_evidence"])
    validate_environment_witness_ref(manifest["environment_witness"])
    outcome = manifest["terminal_outcome"]
    if type(outcome) is not int or outcome not in STAGE2_TERMINAL_OUTCOMES:
        raise ArtifactError(f"Stage 2 manifest 的 terminal_outcome 只接受 {STAGE2_TERMINAL_OUTCOMES}：{outcome!r}")
    validate_generated_at(manifest["generated_at"], "Stage 2 manifest")
    validate_provenance(manifest["finalizer_provenance"], role="finalizer")


# ── 全圖（「五之零」的十六道） ───────────────────────────────────────────────

@dataclass(frozen=True)
class Stage2Verified:
    manifest: dict[str, Any]
    identity: dict[str, Any]
    terminal_outcome: int
    before_base_commit: str   # before source 的 provenance.base_commit（道 8 已綁到 Stage 1 after 的 base）


def _check_canonical_entry(rel: str, entry: Mapping[str, Any], load) -> None:
    got = (load.artifact_sha256, load.stored_sha256, load.stored_bytes)
    if got != (entry["artifact_sha256"], entry["stored_sha256"], entry["stored_bytes"]):
        raise ArtifactError(f"manifest 記的 {rel} metadata 與實際檔案不符")


def verify_stage2_graph(
    root: str | Path,
    *,
    anchor: Stage1Anchor,
    envcheck: EnvcheckResult,
    stage2_identity: Mapping[str, Any] | None = None,
) -> Stage2Verified:
    """十六道（「五之零」）＋ 封閉檔案集合 ＋ manifest 每一項由實際檔案重算。

    ⚠️ finalize（對 staging）與 recovery **呼叫同一支**。`anchor` 與 `envcheck` 由呼叫端以
    `load_stage1_anchor()`／`verify_envcheck(require_equivalent=True)` 在**同一個程序**重新取得
    ——⛔ 不接受跨程序傳遞的 snapshot。
    ⚠️ 每份檔案**只讀一次**；before source 用串流。
    """
    root = Path(root)
    manifest_load = load_canonical_evidence_artifact(root / STAGE2_MANIFEST_NAME, STAGE2_MANIFEST_KIND)
    manifest = manifest_load.parsed
    validate_stage2_manifest(manifest)
    actual = archive_file_set(root)
    expected = set(STAGE2_LAYOUT) | {STAGE2_MANIFEST_NAME}
    if actual != expected:
        raise ArtifactError(
            f"Stage 2 archive 的檔案集合不符：多={sorted(actual - expected)}、缺={sorted(expected - actual)}"
        )
    files = manifest["files"]

    # 道 2（Stage 1 信任錨）與道 16（環境見證錨）：manifest 的宣告值必須等於本次重算的值。
    validate_stage1_evidence_ref(manifest["stage1_evidence"], anchor=anchor)
    validate_environment_witness_ref(manifest["environment_witness"], envcheck=envcheck)
    if envcheck.outcome != OUTCOME_EQUIVALENT:
        raise ArtifactError("道 16／E3b：環境見證的結果不是 EQUIVALENT——⛔ 不得放行 Stage 2")

    # ── 讀檔（每份一次）＋ metadata 重算 ＋ 單檔 validator（道 1） ──────────────
    for rel in RAW_MEMBERS:
        _read_raw_member(root, rel, files[rel])
    identity_load = load_canonical_evidence_artifact(root / IDENTITY, RUN_IDENTITY_KIND)
    _check_canonical_entry(IDENTITY, files[IDENTITY], identity_load)
    identity = identity_load.parsed
    validate_run_identity(identity)
    comparison_load = load_canonical_evidence_artifact(root / COMPARISON, COMPARISON_KIND)
    _check_canonical_entry(COMPARISON, files[COMPARISON], comparison_load)
    comparison = comparison_load.parsed
    comparison_keys = validate_comparison_artifact(comparison)                      # 道 1、13
    report_load = load_canonical_evidence_artifact(root / REPORT, REPORT_KIND)
    _check_canonical_entry(REPORT, files[REPORT], report_load)
    report = report_load.parsed
    validate_report(report, comparison=comparison, comparison_sha256=comparison_load.artifact_sha256,
                    report_max_rows=STAGE2_REPORT_MAX_ROWS)                          # 道 1、5、11、12
    before = stream_before_source(root / BEFORE_SOURCE, keep_keys=set(anchor.cohort_keys))
    _check_canonical_entry(BEFORE_SOURCE, files[BEFORE_SOURCE], before.load)
    before_top = before.load.top

    # ── 道 2：Stage 1 信任錨的跨檔關係（第 6、8、9、10 道） ───────────────────
    validate_stage1_anchor_graph(anchor, stage2_identity=identity, envcheck_identity=envcheck.identity,
                                 expected_bundle_id=manifest["bundle_id"])
    if stage2_identity is not None and dict(stage2_identity) != identity:
        raise ArtifactError("本次的 Stage 2 run identity 與 archive 封存的那一份⛔ 不完全相同")

    # ── 道 16：identity 逐位元等於 envcheck 封存的那一份 ─────────────────────
    env_entry = envcheck.manifest["files"][ENVCHECK_IDENTITY]
    if (identity_load.artifact_sha256, identity_load.stored_sha256) != \
            (env_entry["artifact_sha256"], env_entry["stored_sha256"]):
        raise ArtifactError("道 16：identity/run_identity.json.gz 與 envcheck 封存的 identity ⛔ 不是逐位元相同")

    # ── 道 3、4：comparison 的每列等於來源 row；keys 恰好是 Stage 1 D+1 cohort ────
    if comparison_keys != sorted(anchor.cohort_keys):
        only_c = sorted(set(comparison_keys) - set(anchor.cohort_keys))[:5]
        only_a = sorted(set(anchor.cohort_keys) - set(comparison_keys))[:5]
        raise ArtifactError(
            f"道 4：comparison 的 keys ≠ Stage 1 D+1 cohort（只在 comparison {only_c}、只在 cohort {only_a}）"
        )
    for row in comparison["rows"]:
        key = _key_tuple(row)
        if key not in before.kept_rows or row["before"] != before.kept_rows[key]:
            raise ArtifactError(f"道 3：comparison {key} 的 before ≠ before source 中同 key 的那一列")
        if row["after"] != anchor.cohort_rows[key]:
            raise ArtifactError(f"道 3：comparison {key} 的 after ≠ Stage 1 D+1 after 中同 key 的那一列")
    if comparison["after_artifact_sha256"] != anchor.members[STAGE1_ANCHOR_AFTER]["artifact_sha256"]:
        raise ArtifactError("comparison 的 after_artifact_sha256 ≠ 已錨定的 Stage 1 D+1 after")

    # ── 道 6：bundle 鏈 ─────────────────────────────────────────────────────
    bundle_chain = {
        "Stage 2 manifest": manifest["bundle_id"], "identity": identity["bundle_id"],
        "before source": before_top["bundle_id"], "comparison": comparison["bundle_id"],
        "report": report["bundle_id"], "Stage 1 manifest": anchor.bundle_id,
        "envcheck manifest": envcheck.manifest["bundle_id"],
    }
    if len(set(bundle_chain.values())) != 1:
        raise ArtifactError(f"道 6：bundle_id 等式鏈斷了：{bundle_chain}")

    # ── 道 7：provenance 逐一列舉的位置與 role ──────────────────────────────
    holders = {BEFORE_SOURCE: before_top, COMPARISON: comparison, STAGE2_MANIFEST_NAME: manifest}
    for rel, field, role in PROVENANCE_MAP:
        holder = holders[rel]
        if field not in holder:
            raise ArtifactError(f"道 7：{rel} 缺少 {field}")
        validate_provenance(holder[field], role=role)
        if holder[field]["image_digest"] != manifest["expected_image_id"]:
            raise ArtifactError(f"道 7：{rel}.{field}.image_digest ≠ manifest 的 expected_image_id")

    # ── 道 8：before_ref ────────────────────────────────────────────────────
    before_prov, comparison_prov = before_top["provenance"], comparison["provenance"]
    refs = {
        "before source.before_ref": before_top["before_ref"],
        "comparison.before_ref": comparison["before_ref"],
        "before source.provenance.base_commit": before_prov["base_commit"],
        "comparison.provenance.base_commit": comparison_prov["base_commit"],
        # ⚠️ ③d 補：before 必須跑在 Stage 1 after 的同一個 base 上（v3 模型：e1cbbbd ＋ 反事實 patch）
        # ——合成守門與 failed record 的查找鍵都以它為 base，⛔ 不得各自漂移。
        "Stage 1 after.provenance.base_commit": anchor.after_base_commit,
    }
    if len(set(refs.values())) != 1:
        raise ArtifactError(f"道 8：before_ref／base_commit 不一致：{refs}")

    # ── 道 9：timeframe／replay_scope ───────────────────────────────────────
    for f in ("timeframe", "replay_scope"):
        if before_top[f] != anchor.after_top[f]:
            raise ArtifactError(f"道 9：before source 的 {f}={before_top[f]!r} ≠ Stage 1 D+1 的 {anchor.after_top[f]!r}")

    # ── 道 10：同一次 runner invocation → provenance 逐欄相等 ──────────────────
    if before_prov != comparison_prov:
        diff = sorted(f for f in before_prov if before_prov[f] != comparison_prov.get(f))
        raise ArtifactError(f"道 10：before source 與 comparison 的 provenance 不相等（{diff}）")

    # ── 全量守門：before keys 恰好等於已錨定 D+1 的全量 keys（唯一、固定排序） ────
    if any(a >= b for a, b in zip(before.keys, before.keys[1:])):
        raise ArtifactError("before source 的 rows 必須依 (symbol, timeframe, as_of) 排序且⛔ 不重複")
    if before.keys != anchor.after_keys:
        only_b = sorted(set(before.keys) - set(anchor.after_keys))[:5]
        only_a = sorted(set(anchor.after_keys) - set(before.keys))[:5]
        raise ArtifactError(
            f"before source 的 keys ≠ 已錨定 Stage 1 D+1 的全量 keys（只在 before {only_b}、只在 D+1 {only_a}）"
        )

    # ── 道 15 → 14：「①之三」的檢查順序（flag 不可信時，候選集合也不可信） ───────
    if before.effect is not None:
        raise ArtifactError(
            f"道 15／14：反事實沒有生效（{before.effect['failure_reason']}，"
            f"{before.effect['bounded_diagnostics']}）——⛔ 這不是成功 archive"
        )
    if before.candidate_keys:
        raise ArtifactError(f"道 14：before source 的 candidate_keys 必須為空：{before.candidate_keys[:5]}")

    # ── 道 16：image 鏈 ─────────────────────────────────────────────────────
    image_chain = {
        "before source": before_prov["image_digest"], "comparison": comparison_prov["image_digest"],
        "Stage 2 manifest": manifest["expected_image_id"], "identity": identity["expected_image_id"],
        "envcheck identity": envcheck.identity["expected_image_id"],
        "finalizer": manifest["finalizer_provenance"]["image_digest"],
    }
    if len(set(image_chain.values())) != 1:
        raise ArtifactError(f"道 16：image 等式鏈斷了：{image_chain}")

    # ── 「四之一」第 3 條：合成 hash 綁到 before 實際跑的那一份 ─────────────────
    # ⚠️ 這是「⛔ 不讓語意修改偽裝成 instrumentation」的那道綁定。
    if manifest["patches"]["composed_sha256"] != before_prov["tooling_patch_sha256"]:
        raise ArtifactError(
            "patches.composed_sha256 ≠ before source 的 provenance.tooling_patch_sha256——⛔ 合成關係對不上"
        )
    return Stage2Verified(manifest=manifest, identity=identity, terminal_outcome=manifest["terminal_outcome"],
                          before_base_commit=before_prov["base_commit"])


# ── shell 合成守門的交接（2026-09-24 review 高 1） ───────────────────────────

@dataclass(frozen=True)
class VerifiedComposition:
    """shell **已在隔離 worktree 重建並比對過**的合成關係（由 `finalize-stage2-evidence.sh` 以
    `--verified-*` 注入）。

    ⚠️ **為什麼需要交接**：合成守門（F8-a、「四之二」）在 host、Docker 之前做，Python 之後**重新讀**
    operational artifact 與 patch。兩段之間來源若被一致地換成 B，shell 證明的是 A 的合成關係、Python
    驗證並封存的卻是 B——而 Python ⛔ 不碰 git，只能驗 B 的宣告值彼此一致。`:ro` 掛載只擋容器寫入，
    ⛔ 凍結不了 host 上的內容。所以 Python 必須拿**自己實際讀到、要封存的**內容與這四個值比對，
    相符才允許 commit point（fsync 之前）。
    ⚠️ `base_commit` 是**兩份 patch 所依附的 base**，⛔ 不是 `--base-commit`（那是 finalizer 程式碼的來源）。
    """

    base_commit: str
    counterfactual_patch_sha256: str
    tooling_patch_sha256: str
    composed_sha256: str
    # ⚠️ ⑦a：shell 從**實際 patch** 重算的語意 SHA（只有 publish／recover-failed-record 會帶）。
    counterfactual_semantic_sha256: str | None = None

    def __post_init__(self) -> None:
        _claims(self.base_commit, self.counterfactual_patch_sha256, self.tooling_patch_sha256,
                self.composed_sha256)
        if self.counterfactual_semantic_sha256 is not None:
            validate_semantic_sha256(self.counterfactual_semantic_sha256, "交接的語意 SHA")


def check_verified_composition(verified: VerifiedComposition, *, base_commit: str,
                               patches: Mapping[str, Any], label: str,
                               semantic_sha256: str | None = None) -> None:
    """實際讀到、要封存（或要 fsync）的內容 ⇔ shell 合成守門驗過的值。⛔ 任一不符即中止。

    `semantic_sha256`：failed record 實際的 `counterfactual_semantic_sha256`（成功 archive ⛔ 不存語意 SHA，
    傳 `None`）。⚠️ 交接值與實際欄位必須**同時存在**且相等——一邊有、一邊沒有也是不符。
    """
    if not isinstance(verified, VerifiedComposition):
        raise ArtifactError(f"{label}：缺少 shell 合成守門的交接值——⛔ 沒有它就無法證明封存的就是驗過的那一份")
    if (semantic_sha256 is None) != (verified.counterfactual_semantic_sha256 is None):
        raise ArtifactError(
            f"{label}：語意 SHA 的交接值與實際欄位⛔ 必須同時存在（交接 "
            f"{verified.counterfactual_semantic_sha256!r}、實際 {semantic_sha256!r}）"
        )
    if semantic_sha256 is not None and semantic_sha256 != verified.counterfactual_semantic_sha256:
        raise ArtifactError(
            f"{label}：record 的 counterfactual_semantic_sha256 與 shell 由實際 patch 重算的值不符——"
            "⛔ 不發布、⛔ 不 fsync"
        )
    actual = {"base_commit": base_commit,
              **{f: patches[f] for f in ("counterfactual_patch_sha256", "tooling_patch_sha256", "composed_sha256")}}
    expected = {"base_commit": verified.base_commit,
                "counterfactual_patch_sha256": verified.counterfactual_patch_sha256,
                "tooling_patch_sha256": verified.tooling_patch_sha256,
                "composed_sha256": verified.composed_sha256}
    diff = sorted(f for f in expected if actual[f] != expected[f])
    if diff:
        raise ArtifactError(
            f"{label}：實際讀到的 {diff} 與 shell 合成守門驗過的值不符——合成守門之後輸入被換掉了（TOCTOU），"
            "⛔ 不發布、⛔ 不 fsync"
        )


# ── 發布與 recovery（「五」「六」） ──────────────────────────────────────────

def _load_trust_anchors(python_root: str | Path, stage2_identity: Mapping[str, Any] | None):
    """Stage 1 信任錨（第 1～10 道）＋ 環境見證錨（E1～E7 ＋ E3b）。⚠️ 每次呼叫都重新載入。"""
    anchor = load_stage1_anchor(python_root)
    envcheck = verify_envcheck(resolve_repo_path(python_root, ENVCHECK_ROOT_PATH), anchor,
                               require_equivalent=True, stage2_identity=stage2_identity)
    identity = envcheck.identity if stage2_identity is None else dict(stage2_identity)
    validate_stage1_anchor_graph(anchor, stage2_identity=identity, envcheck_identity=envcheck.identity,
                                 expected_bundle_id=identity["bundle_id"])
    return anchor, envcheck


def _read_frozen_patch(run_dir: Path, rel: str) -> bytes:
    path = run_dir / rel
    if path.is_symlink() or not path.is_file():
        raise ArtifactError(f"凍結 patch 必須是 run 目錄內的一般檔案：{path}")
    return path.read_bytes()


def finalize_stage2_evidence(
    *,
    python_root: str | Path,
    run_dir: str | Path,
    stage2_identity: Mapping[str, Any],
    provenance_factory,
    generated_at: str,
    verified: VerifiedComposition,
) -> int:
    """run 目錄的 operational 輸出 ＋ 凍結 patch → 發布成功 archive。回傳 manifest 的 `terminal_outcome`。

    ⚠️ **合成守門已由 shell 在呼叫前做完**（「四之二」）；這裡驗內部一致性與全圖，並在 commit point
    之前確認 staging 裡要封存的內容**就是** shell 驗過的那一份（`verified`）。
    階段 A：信任錨 ＋ 環境見證錨 ＋ 讀入全部來源（所有 project import 在此發生；before source
    在**驗證的同一趟**寫出封存副本）；階段 B：才建 finalizer_provenance 與 manifest，對 staging 跑
    **與 recovery 相同的**十六道，最後 `rename_noreplace()`——唯一的 commit point。
    """
    validate_run_identity(stage2_identity)
    stage2_identity = dict(stage2_identity)
    run_dir = Path(run_dir)
    root = resolve_repo_path(python_root, STAGE2_EVIDENCE_ROOT_PATH)
    anchor, envcheck = _load_trust_anchors(python_root, stage2_identity)

    with ClosedArchiveWriter(root, STAGE2_LAYOUT, recover_hint=RECOVER_DURABILITY_HINT) as writer:
        # ── 階段 A ──────────────────────────────────────────────────────────
        comparison = load_canonical_evidence_artifact(run_dir / OPERATIONAL_COMPARISON, COMPARISON_KIND)
        report = load_canonical_evidence_artifact(run_dir / OPERATIONAL_REPORT, REPORT_KIND)
        before = stream_before_source(run_dir / OPERATIONAL_BEFORE_SOURCE, keep_keys=set(anchor.cohort_keys),
                                      copy_to=writer.path_for(BEFORE_SOURCE))
        writer.record_streamed(BEFORE_SOURCE, before.load)
        writer.add_canonical(COMPARISON, comparison.parsed)
        writer.add_canonical(REPORT, report.parsed)
        writer.add_canonical(IDENTITY, stage2_identity)
        # ⚠️ 只讀 orchestrator 的**凍結副本**，⛔ 不讀使用者原始路徑（「七之三」）。
        counterfactual = writer.add_raw(COUNTERFACTUAL_PATCH, _read_frozen_patch(run_dir, FROZEN_COUNTERFACTUAL_PATCH))
        tooling = writer.add_raw(TOOLING_PATCH, _read_frozen_patch(run_dir, FROZEN_TOOLING_PATCH))
        patches = {
            "counterfactual_patch_sha256": counterfactual["stored_sha256"],
            "tooling_patch_sha256": tooling["stored_sha256"],
            "composed_sha256": before.load.top["provenance"]["tooling_patch_sha256"],
            "ordered_components": list(ORDERED_COMPONENTS),
        }

        # ── 階段 B ──────────────────────────────────────────────────────────
        provenance = provenance_factory()
        validate_provenance(provenance, role="finalizer")
        manifest = build_stage2_manifest(
            bundle_id=stage2_identity["bundle_id"],
            expected_image_id=stage2_identity["expected_image_id"],
            files=writer.files, patches=patches,
            stage1_evidence=build_stage1_evidence_ref(anchor),
            environment_witness=build_environment_witness_ref(envcheck),
            terminal_outcome=STAGE2_TERMINAL_OUTCOMES[0],
            generated_at=generated_at, finalizer_provenance=provenance,
        )
        validate_stage2_manifest(manifest)

        def verify_staging(staging: Path) -> None:
            graph = verify_stage2_graph(staging, anchor=anchor, envcheck=envcheck, stage2_identity=stage2_identity)
            check_verified_composition(verified, base_commit=graph.before_base_commit,
                                       patches=graph.manifest["patches"], label="finalize")
            # ⚠️ ⛔ 階段 B 之後⛔ 不得有新的 project import——重算一次，與封存的比對。
            if provenance_factory()["project_modules_sha256"] != provenance["project_modules_sha256"]:
                raise ArtifactError("階段 B 之後又載入了新的專案模組——⛔ 封存的 provenance 已經記不全了")

        writer.commit(STAGE2_MANIFEST_NAME, manifest, verify=verify_staging)
    return manifest["terminal_outcome"]


def recover_stage2_durability(*, python_root: str | Path, provenance: dict[str, Any],
                              verified: VerifiedComposition) -> int:
    """⚠️ 只重新驗證既有的成功 archive 並重新 fsync——⛔ 不讀 operational 來源、⛔ 不重建、⛔ 不重跑 replay。

    完整重做 Stage 1 信任錨、環境見證錨（含 E3a 全量重比 ＋ E3b）與十六道，**在 fsync 之前**
    比對執行身分。成功時回 manifest 的 `terminal_outcome`（⛔ 不得寫死）。再失敗 ⛔ 不刪除。
    """
    root = resolve_repo_path(python_root, STAGE2_EVIDENCE_ROOT_PATH)
    if not root.is_dir():
        raise ArtifactError(f"--recover-durability 要求成功 archive 已存在：{root}")
    anchor, envcheck = _load_trust_anchors(python_root, None)
    graph = verify_stage2_graph(root, anchor=anchor, envcheck=envcheck)
    check_verified_composition(verified, base_commit=graph.before_base_commit,
                               patches=graph.manifest["patches"], label="recover-durability")
    archived = graph.manifest["finalizer_provenance"]
    validate_provenance(provenance, role="finalizer")
    for name in RECOVERY_IDENTITY_FIELDS:
        if provenance.get(name) != archived.get(name):
            raise ArtifactError(
                f"recovery 的執行身分與封存的 finalizer_provenance 的 {name} 不符——⛔ 在 fsync 之前中止"
            )
    _fsync_tree(root)
    fsync_parent_or_unconfirmed(root, published=False, recover_hint=RECOVER_DURABILITY_HINT)
    return graph.terminal_outcome


# ── failed-attempt record（「七」） ─────────────────────────────────────────

def build_counterfactual_failure(*, bundle_id: str, generated_at: str, provenance: dict[str, Any],
                                 effect: Mapping[str, Any]) -> dict[str, Any]:
    """replay 在結束碼 6 時寫出的中繼檔（`<run>/stage2/bounded_diagnostics.json`）。

    ⚠️ **③d 訂定**：除了 `failure_reason` ＋ 分流欄位，還帶 `bundle_id` 與該次 replay 的
    `provenance`——F3 要求 record 的 provenance 是**那一次 replay** 的（`tooling_patch_sha256` ＝ 合成
    hash），而 finalizer 自己的 provenance 記的是 finalizer 的 argv 與 base，⛔ 不能代替。
    """
    return {
        "schema_version": ARTIFACT_SCHEMA_VERSION, "kind": COUNTERFACTUAL_FAILURE_KIND,
        "bundle_id": bundle_id, "generated_at": generated_at, "provenance": provenance,
        "failure_reason": effect["failure_reason"], "bounded_diagnostics": effect["bounded_diagnostics"],
    }


def validate_counterfactual_failure(payload: object) -> None:
    if not isinstance(payload, dict) or set(payload) != _FAILURE_SOURCE_FIELDS:
        actual = set(payload) if isinstance(payload, dict) else set()
        raise ArtifactError(
            "bounded_diagnostics 中繼檔的欄位集合不符："
            f"多={sorted(actual - _FAILURE_SOURCE_FIELDS)}、缺={sorted(_FAILURE_SOURCE_FIELDS - actual)}"
        )
    if type(payload["schema_version"]) is not int or payload["schema_version"] != ARTIFACT_SCHEMA_VERSION:
        raise ArtifactError(f"未知的中繼檔 schema_version={payload['schema_version']!r}")
    if payload["kind"] != COUNTERFACTUAL_FAILURE_KIND:
        raise ArtifactError(f"中繼檔的 kind={payload['kind']!r}，預期 {COUNTERFACTUAL_FAILURE_KIND!r}")
    if not isinstance(payload["bundle_id"], str) or not payload["bundle_id"]:
        raise ArtifactError("中繼檔的 bundle_id 必須是非空字串")
    validate_generated_at(payload["generated_at"], "bounded_diagnostics 中繼檔")
    validate_provenance(payload["provenance"], role="stage1")
    validate_bounded_diagnostics(payload["failure_reason"], payload["bounded_diagnostics"])


def validate_semantic_sha256(value: object, label: str) -> None:
    """語意 SHA：64 字元小寫 hex，⛔ 不得是空 diff 的 SHA（語意 diff 必須非空）。"""
    if not is_hex64(value):
        raise ArtifactError(f"{label} 必須是 64 字元小寫 hex：{value!r}")
    if value == EMPTY_SHA256:
        raise ArtifactError(f"{label} ⛔ 不得是空 diff 的 SHA——反事實在兩個產品檔上必須有改動")


def failed_record_dir_name(bundle_id: str, counterfactual_semantic_sha256: str) -> str:
    """`<bundle_id>-<counterfactual_semantic_sha256>`。

    ⚠️ 完整 64 碼，⛔ 不截斷——截成 12 碼會讓不同的完整 SHA 映到同一路徑。⚠️ 鍵是**語意 SHA**
    （⑦ 總綱 v1 決策表第 9 列）：只改測試檔的 counterfactual 得到同一個目錄，⛔ 不能靠它解鎖重跑。
    """
    validate_semantic_sha256(counterfactual_semantic_sha256, "counterfactual_semantic_sha256")
    if not isinstance(bundle_id, str) or not bundle_id or "/" in bundle_id or bundle_id in (".", ".."):
        raise ArtifactError(f"bundle_id ⛔ 不能當目錄名：{bundle_id!r}")
    return f"{bundle_id}-{counterfactual_semantic_sha256}"


def build_failed_record(*, identity: Mapping[str, Any], patches: dict[str, Any],
                        files: Mapping[str, Mapping[str, Any]], failure: Mapping[str, Any],
                        generated_at: str, counterfactual_semantic_sha256: str) -> dict[str, Any]:
    return {
        "schema_version": ARTIFACT_SCHEMA_VERSION,
        "kind": FAILED_ATTEMPT_KIND,
        "bundle_id": identity["bundle_id"],
        "expected_image_id": identity["expected_image_id"],
        "run_identity": dict(identity),
        "patches": patches,
        "files": {rel: dict(files[rel]) for rel in sorted(files)},
        "bounded_diagnostics": failure["bounded_diagnostics"],
        "failure_reason": failure["failure_reason"],
        "generated_at": generated_at,
        "provenance": failure["provenance"],
        "counterfactual_semantic_sha256": counterfactual_semantic_sha256,
    }


def validate_failed_record(record: object) -> None:
    """單份 record 的封閉 schema：F1、F2、F3（role／image／合成 hash）、F5～F7、F10，patch 宣告值。"""
    if not isinstance(record, dict) or set(record) != _FAILED_RECORD_FIELDS:
        actual = set(record) if isinstance(record, dict) else set()
        raise ArtifactError(
            "failed record 的欄位集合不符："
            f"多={sorted(actual - _FAILED_RECORD_FIELDS)}、缺={sorted(_FAILED_RECORD_FIELDS - actual)}"
        )
    if type(record["schema_version"]) is not int or record["schema_version"] != ARTIFACT_SCHEMA_VERSION:
        raise ArtifactError(f"未知的 failed record schema_version={record['schema_version']!r}")
    if record["kind"] != FAILED_ATTEMPT_KIND:
        raise ArtifactError(f"failed record 的 kind={record['kind']!r}，預期 {FAILED_ATTEMPT_KIND!r}")
    identity = record["run_identity"]
    validate_run_identity(identity)                                                          # F1
    for f in ("bundle_id", "expected_image_id"):                                             # F2
        if record[f] != identity[f]:
            raise ArtifactError(f"F2：failed record 的 {f} ≠ embedded run_identity 的同名欄位")
    validate_generated_at(record["generated_at"], "failed record")
    validate_semantic_sha256(record["counterfactual_semantic_sha256"], "failed record 的 counterfactual_semantic_sha256")
    validate_patches(record["patches"])
    files = record["files"]
    if not isinstance(files, dict) or set(files) != set(FAILED_LAYOUT):
        raise ArtifactError(f"failed record 的 files 必須恰好是 {list(FAILED_LAYOUT)}")
    for rel in FAILED_LAYOUT:
        validate_raw_blob_entry(rel, files[rel])
    check_patch_bindings(files, record["patches"])
    prov = record["provenance"]
    validate_provenance(prov, role="stage1")                                                 # F3
    if prov["image_digest"] != record["expected_image_id"]:
        raise ArtifactError("F3：provenance.image_digest ≠ expected_image_id")
    if prov["tooling_patch_sha256"] != record["patches"]["composed_sha256"]:
        raise ArtifactError("F3：provenance.tooling_patch_sha256 ≠ patches.composed_sha256")
    validate_bounded_diagnostics(record["failure_reason"], record["bounded_diagnostics"])    # F5～F7、F10


def verify_failed_record(record_dir: str | Path, *, envcheck_identity: Mapping[str, Any], base_commit: str,
                         dir_name: str | None = None) -> dict[str, Any]:
    """F1～F10 中⛔ 不需要 git 的各項（⚠️ F8-a 的合成守門在 shell）。回傳已驗證的 record。

    ⚠️ 發布（對 staging，`dir_name` 給正式目錄名）、lookup 與 recovery 三處都呼叫這一支。
    """
    record_dir = Path(record_dir)
    actual = archive_file_set(record_dir)                                                    # F9
    expected = set(FAILED_LAYOUT) | {FAILED_RECORD_NAME}
    if actual != expected:
        raise ArtifactError(
            f"F9：failed record {record_dir.name} 的檔案集合不符：多={sorted(actual - expected)}、缺={sorted(expected - actual)}"
        )
    record = load_canonical_evidence_artifact(record_dir / FAILED_RECORD_NAME, FAILED_ATTEMPT_KIND).parsed
    validate_failed_record(record)
    if record["run_identity"] != dict(envcheck_identity):                                    # F2-a
        raise ArtifactError(
            "F2-a：embedded run_identity ⛔ 不完全等於唯一固定的 Stage 2 identity（環境見證錨封存的那一份）"
        )
    if record["provenance"]["base_commit"] != base_commit:                                   # F3
        raise ArtifactError(
            f"F3：provenance.base_commit={record['provenance']['base_commit']} ≠ 兩份 patch 所依附的 base {base_commit}"
        )
    # F4 的 Python 層：目錄名 ＝ `<bundle_id>-<宣告的語意 SHA>`。⚠️ 宣告值是否等於由封存 patch **重算**的
    # 語意 SHA 要用 git，是 F4 的 shell 層（`finalize-stage2-evidence.sh`）；宣告值 ＝ 交接值由
    # `check_verified_composition()` 驗。
    name = record_dir.name if dir_name is None else dir_name
    if name != failed_record_dir_name(record["bundle_id"], record["counterfactual_semantic_sha256"]):
        raise ArtifactError(f"F4：目錄名 {name} 與 record 內容不符——⛔ 改目錄名不能繞過 lookup")
    for rel in FAILED_LAYOUT:                                                                # F8
        _read_raw_member(record_dir, rel, record["files"][rel])
    return record


def publish_failed_record(*, python_root: str | Path, run_dir: str | Path,
                          stage2_identity: Mapping[str, Any], generated_at: str,
                          verified: VerifiedComposition) -> Path:
    """中繼檔 ＋ 凍結 patch → `failed/<bundle_id>-<counterfactual_semantic_sha256>/`。回傳發布的目錄。

    ⚠️ **合成守門（F8-a）與語意 SHA 已由 shell 在呼叫前做完**；目錄名與 record 的語意欄位都取交接值，
    staging 驗證時再從磁碟重讀 record，確認「目錄名 ＝ 宣告值 ＝ 交接值」才 rename。⚠️ 呼叫端（CLI）⛔ 不得把成功發布當成 0——
    那一次執行本來就是失敗的（結束碼 1）；rename 成功、fsync 失敗拋 `DurabilityUnconfirmed`（3）。
    """
    validate_run_identity(stage2_identity)
    stage2_identity = dict(stage2_identity)
    if not isinstance(verified, VerifiedComposition) or verified.counterfactual_semantic_sha256 is None:
        raise ArtifactError("publish-failed-record：缺少 shell 交接的語意 SHA——⛔ 無從決定目錄名")
    semantic = verified.counterfactual_semantic_sha256
    run_dir = Path(run_dir)
    anchor, envcheck = _load_trust_anchors(python_root, stage2_identity)
    failure = load_canonical_evidence_artifact(run_dir / OPERATIONAL_FAILURE, COUNTERFACTUAL_FAILURE_KIND).parsed
    validate_counterfactual_failure(failure)
    if failure["bundle_id"] != stage2_identity["bundle_id"]:
        raise ArtifactError("中繼檔的 bundle_id ≠ Stage 2 identity 的")
    counterfactual = _read_frozen_patch(run_dir, FROZEN_COUNTERFACTUAL_PATCH)
    tooling = _read_frozen_patch(run_dir, FROZEN_TOOLING_PATCH)
    patches = {
        "counterfactual_patch_sha256": sha256_hex(counterfactual),
        "tooling_patch_sha256": sha256_hex(tooling),
        "composed_sha256": failure["provenance"]["tooling_patch_sha256"],
        "ordered_components": list(ORDERED_COMPONENTS),
    }
    failed_root = resolve_repo_path(python_root, STAGE2_FAILED_ROOT_PATH)
    name = failed_record_dir_name(stage2_identity["bundle_id"], semantic)
    root = failed_root / name
    hint = f"{RECOVER_FAILED_HINT} {STAGE2_FAILED_ROOT_PATH}/{name}"
    with ClosedArchiveWriter(root, FAILED_LAYOUT, recover_hint=hint) as writer:
        writer.add_raw(COUNTERFACTUAL_PATCH, counterfactual)
        writer.add_raw(TOOLING_PATCH, tooling)
        record = build_failed_record(identity=stage2_identity, patches=patches, files=writer.files,
                                     failure=failure, generated_at=generated_at,
                                     counterfactual_semantic_sha256=semantic)
        validate_failed_record(record)

        def verify_staging(staging: Path) -> None:
            staged = verify_failed_record(staging, envcheck_identity=envcheck.identity,
                                          base_commit=anchor.after_base_commit, dir_name=name)
            check_verified_composition(verified, base_commit=staged["provenance"]["base_commit"],
                                       patches=staged["patches"], label="publish-failed-record",
                                       semantic_sha256=staged["counterfactual_semantic_sha256"])

        writer.commit(FAILED_RECORD_NAME, record, verify=verify_staging)
    return root


def _record_summary(record_dir: Path, record: Mapping[str, Any]) -> dict[str, Any]:
    return {
        "dir": str(record_dir),
        "base_commit": record["provenance"]["base_commit"],
        **{f: record["patches"][f] for f in ("counterfactual_patch_sha256", "tooling_patch_sha256",
                                             "composed_sha256")},
        "counterfactual_semantic_sha256": record["counterfactual_semantic_sha256"],
    }


def _record_dirs(failed_root: Path) -> list[Path]:
    """failed root 底下的每一項都必須是合法的 record 目錄——⛔ 不認得的東西一律 fail-closed。"""
    if not failed_root.exists():
        return []
    if failed_root.is_symlink() or not failed_root.is_dir():
        raise ArtifactError(f"failed root 必須是目錄：{failed_root}")
    out = []
    for entry in sorted(failed_root.iterdir()):
        if entry.is_symlink() or not entry.is_dir() or not _FAILED_DIR_RE.match(entry.name):
            raise ArtifactError(
                f"failed root 內有無法辨識的項目：{entry.name}——⛔ 不得忽略後繼續（可能是發布中斷的殘骸，需人工處理）"
            )
        out.append(entry)
    return out


def check_failed_records(*, python_root: str | Path) -> dict[str, Any]:
    """`--check-failed-record` 的 **Python 段**：逐份完整驗 F1～F10（⛔ 不含 git 的 F8-a）。

    回傳 `{base_commit, records: [...]}`——`base_commit` 取自**已驗證的** Stage 1 after provenance，
    供 shell 推導本次的 canonical 比對鍵；每份 record 附宣告值，供 shell 做 F8-a。
    ⚠️ 任一份損壞就整個 fail-closed（⛔ 不得「跳過壞的繼續掃」）。
    """
    anchor, envcheck = _load_trust_anchors(python_root, None)
    failed_root = resolve_repo_path(python_root, STAGE2_FAILED_ROOT_PATH)
    records = []
    for record_dir in _record_dirs(failed_root):
        record = verify_failed_record(record_dir, envcheck_identity=envcheck.identity,
                                      base_commit=anchor.after_base_commit)
        records.append(_record_summary(record_dir, record))
    return {"base_commit": anchor.after_base_commit, "records": records}


def recover_failed_record(*, python_root: str | Path, record_dir: str | Path,
                          verified: VerifiedComposition) -> dict[str, Any]:
    """rc=3 的出口：完整重驗 F1～F10（F8-a 由 shell 先做）→ `_fsync_tree` ＋ fsync parent。

    ⛔ 不重建、⛔ 不重跑 replay。⚠️ 呼叫端成功時仍回 1——那一次執行本來就是失敗的；
    重跑資格仍是「⛔ 同語意 SHA 不得重跑」。⚠️ fsync 之前比對 record 宣告的語意 SHA ＝ shell 由 record 內
    實際 patch 重算的交接值。
    """
    failed_root = resolve_repo_path(python_root, STAGE2_FAILED_ROOT_PATH)
    record_dir = Path(record_dir)
    if record_dir.is_symlink() or not record_dir.is_dir() or \
            record_dir.resolve().parent != failed_root.resolve() or not _FAILED_DIR_RE.match(record_dir.name):
        raise ArtifactError(f"--recover-failed-record 只接受 {STAGE2_FAILED_ROOT_PATH}/ 底下的 record 目錄：{record_dir}")
    anchor, envcheck = _load_trust_anchors(python_root, None)
    record = verify_failed_record(record_dir, envcheck_identity=envcheck.identity, base_commit=anchor.after_base_commit)
    check_verified_composition(verified, base_commit=record["provenance"]["base_commit"],
                               patches=record["patches"], label="recover-failed-record",
                               semantic_sha256=record["counterfactual_semantic_sha256"])
    _fsync_tree(record_dir)
    fsync_parent_or_unconfirmed(record_dir, published=False,
                                recover_hint=f"{RECOVER_FAILED_HINT} {STAGE2_FAILED_ROOT_PATH}/{record_dir.name}")
    return _record_summary(record_dir, record)


# ── 合成守門的宣告值（給 shell，⛔ 不是 validator） ──────────────────────────

def _claims(base_commit: object, counterfactual: object, tooling: object, composed: object) -> dict[str, str]:
    if not isinstance(base_commit, str) or len(base_commit) != 40 or base_commit != base_commit.lower() \
            or not all(c in "0123456789abcdef" for c in base_commit):
        raise ArtifactError(f"base_commit 必須是 40 字元小寫 hex：{base_commit!r}")
    for name, value in (("counterfactual", counterfactual), ("tooling", tooling), ("composed", composed)):
        if not is_hex64(value):
            raise ArtifactError(f"{name} 的 SHA 必須是 64 字元小寫 hex：{value!r}")
    return {"base_commit": base_commit, "counterfactual_patch_sha256": counterfactual,
            "tooling_patch_sha256": tooling, "composed_sha256": composed}


def patch_claims(*, mode: str, python_root: str | Path, run_dir: str | Path | None = None,
                 record_dir: str | Path | None = None) -> dict[str, str]:
    """shell 合成守門要比對的**宣告值**：`{base_commit, counterfactual／tooling／composed 的 SHA}`。

    ⚠️ 這裡只「取出宣告」，⛔ 不是驗證——shell 用它們在隔離 worktree 重建並比對，之後 Python 段
    仍會完整驗證全部內容。只讀小檔（comparison、中繼檔、manifest、record），⛔ 不讀全量 before source。
    """
    if mode == "finalize":
        run_dir = Path(run_dir)
        prov = load_canonical_evidence_artifact(run_dir / OPERATIONAL_COMPARISON, COMPARISON_KIND).parsed["provenance"]
        return _claims(prov.get("base_commit"),
                       sha256_hex(_read_frozen_patch(run_dir, FROZEN_COUNTERFACTUAL_PATCH)),
                       sha256_hex(_read_frozen_patch(run_dir, FROZEN_TOOLING_PATCH)),
                       prov.get("tooling_patch_sha256"))
    if mode == "failure":
        run_dir = Path(run_dir)
        prov = load_canonical_evidence_artifact(run_dir / OPERATIONAL_FAILURE, COUNTERFACTUAL_FAILURE_KIND).parsed["provenance"]
        return _claims(prov.get("base_commit"),
                       sha256_hex(_read_frozen_patch(run_dir, FROZEN_COUNTERFACTUAL_PATCH)),
                       sha256_hex(_read_frozen_patch(run_dir, FROZEN_TOOLING_PATCH)),
                       prov.get("tooling_patch_sha256"))
    if mode == "archive":
        root = resolve_repo_path(python_root, STAGE2_EVIDENCE_ROOT_PATH)
        patches = load_canonical_evidence_artifact(root / STAGE2_MANIFEST_NAME, STAGE2_MANIFEST_KIND).parsed["patches"]
        prov = load_canonical_evidence_artifact(root / COMPARISON, COMPARISON_KIND).parsed["provenance"]
        return _claims(prov.get("base_commit"), patches.get("counterfactual_patch_sha256"),
                       patches.get("tooling_patch_sha256"), patches.get("composed_sha256"))
    if mode == "failed-record":
        record = load_canonical_evidence_artifact(Path(record_dir) / FAILED_RECORD_NAME, FAILED_ATTEMPT_KIND).parsed
        patches = record.get("patches") or {}
        claims = _claims((record.get("provenance") or {}).get("base_commit"),
                         patches.get("counterfactual_patch_sha256"), patches.get("tooling_patch_sha256"),
                         patches.get("composed_sha256"))
        # ⚠️ ⑦a：record **宣告的**語意 SHA——shell 從 record 內的實際 patch 重算並比對它，相等才注入。
        semantic = record.get("counterfactual_semantic_sha256")
        validate_semantic_sha256(semantic, "record 宣告的 counterfactual_semantic_sha256")
        return {**claims, "counterfactual_semantic_sha256": semantic}
    raise ArtifactError(f"未知的 claims 模式：{mode!r}")


# ── CLI（`scripts/finalize-stage2-evidence.sh` 的 ③d 五種模式） ─────────────

class Stage2UsageError(ValueError):
    """CLI 用法錯誤。"""


# ⚠️ 只能由官方腳本注入。精確比對（⛔ 不用前綴清單）。
STAGE2_INJECTED_ARGS = (
    "--run-identity", "--python-root", "--image-digest", "--base-commit",
    "--tooling-patch-sha256", "--source-root", "--runner-sha256",
    "--verified-patch-base", "--verified-counterfactual-sha256", "--verified-tooling-sha256",
    "--verified-composed-sha256", "--verified-counterfactual-semantic-sha256",
)
# shell 合成守門的交接值（`VerifiedComposition`）。⚠️ 除了 check，其餘四種模式**必須**有；check 的 F8-a 在
# Python 段之後由 shell 做，⛔ 不接受。
VERIFIED_COMPOSITION_ARGS = ("verified_patch_base", "verified_counterfactual_sha256",
                             "verified_tooling_sha256", "verified_composed_sha256")
MODES_WITH_COMPOSITION = ("finalize", "recover_durability", "publish_failed_record", "recover_failed_record")
# ⚠️ ⑦a：shell 由實際 patch 重算的語意 SHA。⚠️ **只有**這兩種模式必須帶（failed record 以它為目錄鍵）；
# 成功 archive ⛔ 不存語意 SHA，check 的命中判定在 shell——其餘模式帶了就拒。
MODES_WITH_SEMANTIC = ("publish_failed_record", "recover_failed_record")
STAGE2_MODES = ("finalize", "recover_durability", "publish_failed_record", "check_failed_record",
                "recover_failed_record")


def assert_stage2_arg_ownership(argv: Sequence[str]) -> None:
    for injected in STAGE2_INJECTED_ARGS:
        count = sum(1 for a in argv if a == injected or a.startswith(injected + "="))
        if count > 1:
            raise Stage2UsageError(f"{injected} 出現 {count} 次——它只能由官方腳本注入。⛔ 不靜默採用最後一個。")


def build_stage2_parser():
    import argparse

    parser = argparse.ArgumentParser(prog="stage2_archive", allow_abbrev=False)
    parser.add_argument("--finalize", action="store_true")
    parser.add_argument("--recover-durability", action="store_true")
    parser.add_argument("--publish-failed-record", action="store_true")
    parser.add_argument("--check-failed-record", action="store_true")
    parser.add_argument("--recover-failed-record", default=None, metavar="RECORD_DIR")
    parser.add_argument("--run-dir", default=None)
    parser.add_argument("--run-identity", default=None)
    parser.add_argument("--python-root", required=True)
    parser.add_argument("--image-digest", required=True)
    parser.add_argument("--base-commit", required=True)
    parser.add_argument("--tooling-patch-sha256", required=True)
    parser.add_argument("--source-root", required=True)
    parser.add_argument("--runner-sha256", required=True)
    parser.add_argument("--verified-patch-base", default=None)
    parser.add_argument("--verified-counterfactual-sha256", default=None)
    parser.add_argument("--verified-tooling-sha256", default=None)
    parser.add_argument("--verified-composed-sha256", default=None)
    parser.add_argument("--verified-counterfactual-semantic-sha256", default=None)
    return parser


def run_stage2(argv: Sequence[str]) -> tuple[int, dict[str, Any]]:
    """回傳 `(結束碼, 結果)`。⚠️ publish-failed-record 與 recover-failed-record **成功時也回 1**。"""
    from datetime import datetime, timezone

    from .provenance import build_provenance

    argv = list(argv)
    assert_stage2_arg_ownership(argv)
    args = build_stage2_parser().parse_args(argv)
    chosen = [m for m, on in (
        ("finalize", args.finalize), ("recover_durability", args.recover_durability),
        ("publish_failed_record", args.publish_failed_record), ("check_failed_record", args.check_failed_record),
        ("recover_failed_record", args.recover_failed_record is not None),
    ) if on]
    if len(chosen) != 1:
        raise Stage2UsageError(f"模式旗標必須恰好給一個，實際 {chosen}")
    mode = chosen[0]
    needs_run = mode in ("finalize", "publish_failed_record")
    if needs_run and (not args.run_dir or not args.run_identity):
        raise Stage2UsageError(f"--{mode.replace('_', '-')} 需要 --run-dir 與 --run-identity")
    if not needs_run and (args.run_dir or args.run_identity):
        raise Stage2UsageError(
            f"--{mode.replace('_', '-')} ⛔ 不接受 --run-dir 或 --run-identity：它只依賴既有的證據與寫死常數"
        )

    given = [name for name in VERIFIED_COMPOSITION_ARGS if getattr(args, name) is not None]
    if mode in MODES_WITH_COMPOSITION:
        if len(given) != len(VERIFIED_COMPOSITION_ARGS):
            raise Stage2UsageError(
                f"--{mode.replace('_', '-')} 需要 shell 合成守門注入的四個 --verified-* 值（缺 "
                f"{sorted(set(VERIFIED_COMPOSITION_ARGS) - set(given))}）"
            )
        try:
            verified = VerifiedComposition(
                base_commit=args.verified_patch_base,
                counterfactual_patch_sha256=args.verified_counterfactual_sha256,
                tooling_patch_sha256=args.verified_tooling_sha256,
                composed_sha256=args.verified_composed_sha256,
                counterfactual_semantic_sha256=args.verified_counterfactual_semantic_sha256,
            )
        except ArtifactError as exc:
            raise Stage2UsageError(f"--verified-* 的格式不符：{exc}") from exc
    elif given:
        raise Stage2UsageError(f"--{mode.replace('_', '-')} ⛔ 不接受 --verified-*（F8-a 在 Python 段之後由 shell 做）")
    semantic = args.verified_counterfactual_semantic_sha256
    if mode in MODES_WITH_SEMANTIC and semantic is None:
        raise Stage2UsageError(
            f"--{mode.replace('_', '-')} 需要 shell 注入的 --verified-counterfactual-semantic-sha256（failed record 的目錄鍵）"
        )
    if mode not in MODES_WITH_SEMANTIC and semantic is not None:
        raise Stage2UsageError(
            f"--{mode.replace('_', '-')} ⛔ 不接受 --verified-counterfactual-semantic-sha256（只屬於 failed record 的發布與 recovery）"
        )

    # ⚠️ 階段 A 明確載入 `config`（理由同 Stage 1 finalizer：⛔ 靠間接 import 是 incidental）。
    import config as _config_module

    def _provenance_factory():
        return build_provenance(
            config_module=_config_module, source_root=args.source_root, image_digest=args.image_digest,
            base_commit=args.base_commit, tooling_patch_sha256=args.tooling_patch_sha256,
            runner_sha256=args.runner_sha256, argv=argv,
        )

    now = datetime.now(timezone.utc).isoformat()
    if needs_run:
        identity = load_run_identity(args.run_identity)
        if identity["expected_image_id"] != args.image_digest:
            raise Stage2UsageError(
                f"Stage 2 identity 記的 image {identity['expected_image_id']} 與本次執行的 {args.image_digest} 不符"
            )
    if mode == "finalize":
        terminal = finalize_stage2_evidence(
            python_root=args.python_root, run_dir=args.run_dir, stage2_identity=identity,
            provenance_factory=_provenance_factory, generated_at=now, verified=verified,
        )
        return terminal, {"mode": mode, "terminal_outcome": terminal}
    if mode == "recover_durability":
        terminal = recover_stage2_durability(python_root=args.python_root, provenance=_provenance_factory(),
                                             verified=verified)
        return terminal, {"mode": mode, "terminal_outcome": terminal}
    if mode == "publish_failed_record":
        root = publish_failed_record(python_root=args.python_root, run_dir=args.run_dir,
                                     stage2_identity=identity, generated_at=now, verified=verified)
        # ⚠️ 完整發布仍回 1：那一次執行本來就是失敗的（「七之三」的重跑資格表）。
        return 1, {"mode": mode, "published": str(root)}
    if mode == "check_failed_record":
        result = check_failed_records(python_root=args.python_root)
        return 0, {"mode": mode, **result}
    summary = recover_failed_record(python_root=args.python_root, record_dir=args.recover_failed_record,
                                    verified=verified)
    return 1, {"mode": mode, "recovered": summary}


def main(argv=None) -> int:
    import json
    import sys

    from .publish import EXIT_ABORT, EXIT_DURABILITY_UNCONFIRMED

    argv = list(sys.argv[1:] if argv is None else argv)
    try:
        rc, result = run_stage2(argv)
    except DurabilityUnconfirmed as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return EXIT_DURABILITY_UNCONFIRMED
    except (Stage2UsageError, ValueError, OSError) as exc:
        print(f"ERROR: {type(exc).__name__}: {exc}", file=sys.stderr)
        return EXIT_ABORT
    print(json.dumps(result, ensure_ascii=False, sort_keys=True))
    return rc


if __name__ == "__main__":
    raise SystemExit(main())
