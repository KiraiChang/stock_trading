"""跨日逐列比對：D 與 D+1 兩份 after artifact（I-074 Stage 1 / I-100 關閉條件 2 後半）。

⚠️ **既有的 `compare_rows()` 比的是 before vs after 兩個「版本」**，⛔ 不是同版本跨日的兩份
after artifact，所以這一層是新的。

⛔ **不能用 after artifact 的 SHA-256 比**：它含 `generated_at` 與 argv，兩趟必然不同，
SHA 一定不相等——那⛔ 不代表逐列結果不同。要比的是 **`rows`**。

⚠️ **先擋「假跨日」**：把**同一份** artifact 傳兩次時，三個差異集合都會是空的、`matched`
會是 true——那是一份**假的**關閉證據。所以輸入有效性要先驗（見 `_assert_valid_inputs`），
且 ⛔ **同檔／同 SHA／同日算 invalid input（exit 1），不是 mismatch**：把它算成 mismatch
等於承認那是一次有效的跨日比較。
"""
from __future__ import annotations

from datetime import datetime, timedelta, timezone
from typing import Any, Sequence

from .artifacts import (
    ARTIFACT_SCHEMA_VERSION,
    AFTER_KIND,
    ArtifactError,
    compare_rows,
    load_canonical_evidence_artifact,
    row_key,
    validate_after_artifact,
    validate_diagnostics,
)
from .provenance import PROVENANCE_FIELDS, ProvenanceError, validate_provenance

CROSSDAY_KIND = "sr_zone_replay_crossday"
CROSSDAY_ARTIFACT_NAME = "crossday_artifact.json"

# 封閉欄位集合。⛔ 缺欄或多一個未知欄位都拒絕。
_CROSSDAY_FIELDS = frozenset({
    "schema_version", "kind", "bundle_id",
    "d_artifact_sha256", "d1_artifact_sha256",
    "d_generated_at", "d1_generated_at",
    "d_row_count", "d1_row_count",
    "d_key_order", "d1_key_order",
    "rows_match", "provenance_match", "matched", "outcome",
    "d_only", "d1_only", "d_only_rows", "d1_only_rows",
    "row_differences", "provenance_differences",
    "comparator_provenance", "generated_at",
})

OUTCOME_MATCH = "MATCH"
OUTCOME_ROW = "ROW_MISMATCH"
OUTCOME_PROVENANCE = "PROVENANCE_MISMATCH"
OUTCOME_BOTH = "ROW_AND_PROVENANCE_MISMATCH"
_OUTCOMES = (OUTCOME_MATCH, OUTCOME_ROW, OUTCOME_PROVENANCE, OUTCOME_BOTH)

# 兩份 after artifact 必須一致的身分欄位。
_IDENTITY_FIELDS = ("bundle_id", "timeframe", "replay_scope", "run_id", "pipeline_version")

_TAIPEI = timezone(timedelta(hours=8))
_OUTPUT_DIR_PLACEHOLDER = "<output-dir>"


class CrossdayMismatch(Exception):
    """兩份 after artifact 不一致。

    ⛔ **刻意不繼承 `ValueError`**——`ArtifactError` 是 `ValueError` 的子類而 CLI 用
    generic catch 收斂，繼承下去 `EXIT_CROSSDAY_MISMATCH` 這個專屬碼**根本出不來**
    （`CandidateMismatch` 同一個理由）。
    """

    def __init__(self, message: str, *, path, outcome: str) -> None:
        super().__init__(message)
        self.path = path
        self.outcome = outcome


# ── argv 正規化 ─────────────────────────────────────────────────────────────

def normalize_argv(argv: Sequence[str]) -> list[str]:
    """把 `--output-dir` 的**值**換成固定佔位符，其餘原樣。

    ⚠️ 兩種寫法都要收：`--output-dir value` 與 `--output-dir=value`。
    ⛔ **缺值或重複出現一律拒絕**——那時「哪一個才算數」沒有唯一答案。
    """
    if not isinstance(argv, list) or not all(isinstance(a, str) for a in argv):
        raise ProvenanceError("argv 必須是字串陣列")
    out: list[str] = []
    seen = 0
    i = 0
    while i < len(argv):
        token = argv[i]
        if token == "--output-dir":
            seen += 1
            if i + 1 >= len(argv):
                raise ProvenanceError("argv 的 --output-dir 缺少值")
            out.extend(["--output-dir", _OUTPUT_DIR_PLACEHOLDER])
            i += 2
            continue
        if token.startswith("--output-dir="):
            seen += 1
            out.append(f"--output-dir={_OUTPUT_DIR_PLACEHOLDER}")
            i += 1
            continue
        out.append(token)
        i += 1
    if seen != 1:
        raise ProvenanceError(f"argv 的 --output-dir 必須恰好出現一次，實際 {seen} 次")
    return out


def provenance_differences(d_prov: dict[str, Any], d1_prov: dict[str, Any]) -> list[dict[str, Any]]:
    """逐欄比對**全部 10 個** provenance 欄位，回傳 `{field, d, d1}` 的排序清單。

    ⚠️ **唯一允許不同的是 `argv` 裡的 `--output-dir` 值**——兩趟必須用不同的全新目錄
    （`prepare_output_dir()` 擋非空目錄），所以那一項要先正規化再比。
    """
    diffs: list[dict[str, Any]] = []
    for field in PROVENANCE_FIELDS:
        left, right = d_prov.get(field), d1_prov.get(field)
        if field == "argv":
            left, right = normalize_argv(left), normalize_argv(right)
        if left != right:
            diffs.append({"field": field, "d": d_prov.get(field), "d1": d1_prov.get(field)})
    return sorted(diffs, key=lambda item: item["field"])


# ── 輸入有效性 ──────────────────────────────────────────────────────────────

def _parse_tz_aware(value: object, label: str) -> datetime:
    if not isinstance(value, str) or not value:
        raise ArtifactError(f"{label} 必須是非空字串：{value!r}")
    try:
        parsed = datetime.fromisoformat(value)
    except ValueError as exc:
        raise ArtifactError(f"{label} 不是合法的 ISO-8601：{value!r}（{exc}）") from exc
    if parsed.tzinfo is None:
        raise ArtifactError(f"{label} ⛔ 必須含時區：{value!r}")
    return parsed


def assert_valid_crossday_inputs(
    *, d: dict[str, Any], d1: dict[str, Any], d_sha: str, d1_sha: str
) -> None:
    """⚠️ **先擋假跨日**——不合法一律 exit 1，⛔ 不產 artifact、⛔ 不算 mismatch。"""
    if d_sha == d1_sha:
        raise ArtifactError(
            "D 與 D+1 是**同一份** artifact（SHA 相同）——⛔ 這不是有效的跨日比較，"
            "把它算成 mismatch 等於承認那是一次有效的比較。"
        )
    d_time = _parse_tz_aware(d.get("generated_at"), "D 的 generated_at")
    d1_time = _parse_tz_aware(d1.get("generated_at"), "D+1 的 generated_at")
    if d1_time <= d_time:
        raise ArtifactError(
            f"時間順序不對：D={d_time.isoformat()}、D+1={d1_time.isoformat()}"
        )
    d_date = d_time.astimezone(_TAIPEI).date()
    d1_date = d1_time.astimezone(_TAIPEI).date()
    if d1_date != d_date + timedelta(days=1):
        raise ArtifactError(
            f"⛔ 不是相鄰兩天（台北日期）：D={d_date}、D+1={d1_date}——"
            "I-100 關閉條件 2 要的是**跨日**重現性。"
        )
    for field in _IDENTITY_FIELDS:
        if d.get(field) != d1.get(field):
            raise ArtifactError(
                f"兩份 after artifact 的 {field} 不一致：{d.get(field)!r} vs {d1.get(field)!r}"
            )
    # 來源 provenance 形狀錯 → 也是 invalid input，⛔ 不是 mismatch。
    validate_provenance(d.get("provenance"), role="stage1")
    validate_provenance(d1.get("provenance"), role="stage1")


def load_after_for_crossday(path) -> tuple[dict[str, Any], str]:
    """完整的來源驗證鏈。

    ⚠️ **`validate_after_artifact()` ⛔ 不驗九個 diagnostics**（它只做結構 ＋
    `validate_candidate_flags`），所以第三步⛔ 不能省——Stage 2 也是另外呼叫它的。
    """
    loaded = load_canonical_evidence_artifact(path, AFTER_KIND)
    validate_after_artifact(loaded.parsed)
    validate_diagnostics(loaded.parsed["rows"], f"crossday 來源 {getattr(path, 'name', path)}", side="after")
    return loaded.parsed, loaded.artifact_sha256


# ── 產生 ────────────────────────────────────────────────────────────────────

def build_crossday(
    *,
    d: dict[str, Any],
    d1: dict[str, Any],
    d_sha: str,
    d1_sha: str,
    comparator_provenance: dict[str, Any],
    generated_at: str,
) -> dict[str, Any]:
    """⚠️ 呼叫前必須已通過 `assert_valid_crossday_inputs()`。"""
    d_rows, d1_rows = d["rows"], d1["rows"]
    d_keys = [row_key(row) for row in d_rows]
    d1_keys = [row_key(row) for row in d1_rows]
    d_by_key = {key: row for key, row in zip(d_keys, d_rows)}
    d1_by_key = {key: row for key, row in zip(d1_keys, d1_rows)}

    d_only = sorted(set(d_keys) - set(d1_keys))
    d1_only = sorted(set(d1_keys) - set(d_keys))
    common = sorted(set(d_keys) & set(d1_keys))

    # ⚠️ **重用 `compare_rows()`**：`before` 位裝 **D**、`after` 位裝 **D+1**。
    # ⛔ 不改欄位名、⛔ 不複述形狀——形狀由函式本身定義，沒有第二份描述可以漂移。
    row_differences = [
        item for item in (compare_rows(d_by_key[key], d1_by_key[key]) for key in common)
        if item["differences"]
    ]
    prov_diffs = provenance_differences(d["provenance"], d1["provenance"])

    rows_match = d_keys == d1_keys and not d_only and not d1_only and not row_differences
    provenance_match = not prov_diffs
    if rows_match and provenance_match:
        outcome = OUTCOME_MATCH
    elif provenance_match:
        outcome = OUTCOME_ROW
    elif rows_match:
        outcome = OUTCOME_PROVENANCE
    else:
        outcome = OUTCOME_BOTH

    return {
        "schema_version": ARTIFACT_SCHEMA_VERSION,
        "kind": CROSSDAY_KIND,
        "bundle_id": d["bundle_id"],
        "d_artifact_sha256": d_sha,
        "d1_artifact_sha256": d1_sha,
        "d_generated_at": d["generated_at"],
        "d1_generated_at": d1["generated_at"],
        "d_row_count": len(d_rows),
        "d1_row_count": len(d1_rows),
        "d_key_order": [list(key) for key in d_keys],
        "d1_key_order": [list(key) for key in d1_keys],
        "rows_match": rows_match,
        "provenance_match": provenance_match,
        "matched": rows_match and provenance_match,
        "outcome": outcome,
        "d_only": [list(key) for key in d_only],
        "d1_only": [list(key) for key in d1_only],
        "d_only_rows": [d_by_key[key] for key in d_only],
        "d1_only_rows": [d1_by_key[key] for key in d1_only],
        "row_differences": row_differences,
        "provenance_differences": prov_diffs,
        "comparator_provenance": comparator_provenance,
        "generated_at": generated_at,
    }


# ── 驗證 ────────────────────────────────────────────────────────────────────

def _require_bool(artifact: dict[str, Any], field: str) -> bool:
    value = artifact[field]
    # ⛔ `isinstance(x, bool)`：用 `1`／`0` 冒充 boolean 的 artifact 可以通過所有
    # 「重算後相等」的比對（Python 裡 `1 == True`），型別驗證⛔ 取代不了。
    if not isinstance(value, bool):
        raise ArtifactError(f"{field} 必須是 JSON true/false，實際 {value!r}")
    return value


def _require_hex64(artifact: dict[str, Any], field: str) -> str:
    value = artifact[field]
    if not isinstance(value, str) or len(value) != 64 or value != value.lower() \
            or not all(c in "0123456789abcdef" for c in value):
        raise ArtifactError(f"{field} 必須是 64 字元小寫 hex：{value!r}")
    return value


def _require_key_list(raw: object, label: str) -> list[tuple[str, str, str]]:
    if not isinstance(raw, list):
        raise ArtifactError(f"{label} 必須是陣列")
    out: list[tuple[str, str, str]] = []
    for item in raw:
        if not isinstance(item, list) or len(item) != 3 \
                or any(not isinstance(v, str) or not v for v in item):
            raise ArtifactError(f"{label} 的元素必須是三個非空字串：{item!r}")
        out.append((item[0], item[1], item[2]))
    return out


def validate_crossday(
    artifact: dict[str, Any],
    *,
    d: dict[str, Any],
    d1: dict[str, Any],
    d_sha: str,
    d1_sha: str,
    comparator_provenance: dict[str, Any],
) -> None:
    """封閉 schema ＋ 型別 ＋ **由實際來源重算的逐項比對**。

    ⚠️ **五個來源參數都是必填**，⛔ 沒有「不給就跳過」的模式：一份**內部自洽**卻漏記真實
    差異的 artifact 可以通過所有自我檢查——那正是 Stage 0 的 candidate mismatch validator
    花了 v9～v12 四輪才修掉的坑。

    ⚠️ **`by_key` 由本函式自己從兩份 artifact 的 `rows` 建**，⛔ 不收外部傳入的版本——
    收了就得先逐項驗它與 rows 相同，那不如自己建（⛔ 不製造第三個真相源）。
    """
    # ⚠️ **durable validator 要自己驗來源有效性**，⛔ 不能只依賴「producer 當初呼叫過」。
    # finalizer 與 recovery 只呼叫本函式——少了這一行，重新封裝一套內部自洽的來源時，
    # 「同一台北日期／不相鄰／身分欄位不一致」都會被放行。
    assert_valid_crossday_inputs(d=d, d1=d1, d_sha=d_sha, d1_sha=d1_sha)

    if not isinstance(artifact, dict):
        raise ArtifactError("crossday artifact 必須是 object")
    actual = set(artifact)
    if actual != set(_CROSSDAY_FIELDS):
        raise ArtifactError(
            "crossday 欄位集合不符："
            f"多={sorted(actual - _CROSSDAY_FIELDS)}、缺={sorted(_CROSSDAY_FIELDS - actual)}"
        )

    version = artifact["schema_version"]
    if type(version) is not int or version != ARTIFACT_SCHEMA_VERSION:
        raise ArtifactError(f"未知的 crossday schema_version={version!r}")
    if artifact["kind"] != CROSSDAY_KIND:
        raise ArtifactError(f"crossday 的 kind={artifact['kind']!r}，預期 {CROSSDAY_KIND!r}")
    if not isinstance(artifact["bundle_id"], str) or not artifact["bundle_id"]:
        raise ArtifactError("crossday 的 bundle_id 必須是非空字串")

    left_sha = _require_hex64(artifact, "d_artifact_sha256")
    right_sha = _require_hex64(artifact, "d1_artifact_sha256")
    if left_sha == right_sha:
        raise ArtifactError("d 與 d1 的 SHA 相同——⛔ 那不是有效的跨日比較")

    # ── 與**實際來源**比對 ──────────────────────────────────────────────
    if left_sha != d_sha or right_sha != d1_sha:
        raise ArtifactError(
            f"artifact 記的 SHA 與實際載入的不符："
            f"d={left_sha}/{d_sha}、d1={right_sha}/{d1_sha}"
        )
    if artifact["bundle_id"] != d["bundle_id"]:
        raise ArtifactError("crossday 的 bundle_id 與來源不符")
    for field, source, label in (
        ("d_generated_at", d, "D"), ("d1_generated_at", d1, "D+1"),
    ):
        if artifact[field] != source["generated_at"]:
            raise ArtifactError(f"{field} 與{label}來源記的值不符")
        _parse_tz_aware(artifact[field], field)

    d_rows, d1_rows = d["rows"], d1["rows"]
    for field, rows in (("d_row_count", d_rows), ("d1_row_count", d1_rows)):
        value = artifact[field]
        if type(value) is not int or value < 0:
            raise ArtifactError(f"{field} 必須是非負整數：{value!r}")
        if value != len(rows):
            raise ArtifactError(f"{field}={value} 與實際來源的 {len(rows)} 列不符")

    expected_d_keys = [row_key(row) for row in d_rows]
    expected_d1_keys = [row_key(row) for row in d1_rows]
    if _require_key_list(artifact["d_key_order"], "d_key_order") != expected_d_keys:
        raise ArtifactError("d_key_order 與實際來源的 key 序列不符")
    if _require_key_list(artifact["d1_key_order"], "d1_key_order") != expected_d1_keys:
        raise ArtifactError("d1_key_order 與實際來源的 key 序列不符")

    d_by_key = {key: row for key, row in zip(expected_d_keys, d_rows)}
    d1_by_key = {key: row for key, row in zip(expected_d1_keys, d1_rows)}
    exp_d_only = sorted(set(expected_d_keys) - set(expected_d1_keys))
    exp_d1_only = sorted(set(expected_d1_keys) - set(expected_d_keys))
    if _require_key_list(artifact["d_only"], "d_only") != exp_d_only:
        raise ArtifactError("d_only 與實際來源重算的差集不符")
    if _require_key_list(artifact["d1_only"], "d1_only") != exp_d1_only:
        raise ArtifactError("d1_only 與實際來源重算的差集不符")

    for field, keys, by_key in (
        ("d_only_rows", exp_d_only, d_by_key), ("d1_only_rows", exp_d1_only, d1_by_key),
    ):
        rows = artifact[field]
        if not isinstance(rows, list) or len(rows) != len(keys):
            raise ArtifactError(f"{field} 的列數必須等於對應的差集（{len(keys)}）")
        for item, key in zip(rows, keys):
            if not isinstance(item, dict) or row_key(item) != key:
                raise ArtifactError(f"{field} 的 key 序列必須逐項等於對應的 key list")
            if item != by_key[key]:
                raise ArtifactError(f"{field} 的 {key} 與實際來源的 row 不符")

    common = sorted(set(expected_d_keys) & set(expected_d1_keys))
    expected_diffs = [
        item for item in (compare_rows(d_by_key[key], d1_by_key[key]) for key in common)
        if item["differences"]
    ]
    row_differences = artifact["row_differences"]
    if not isinstance(row_differences, list):
        raise ArtifactError("row_differences 必須是陣列")
    if any(not item.get("differences") for item in row_differences if isinstance(item, dict)):
        raise ArtifactError("row_differences ⛔ 不得包含空的 differences")
    if row_differences != expected_diffs:
        raise ArtifactError(
            f"row_differences 與實際來源重算的結果不符"
            f"（artifact {len(row_differences)} 列、重算 {len(expected_diffs)} 列）"
        )

    expected_prov_diffs = provenance_differences(d["provenance"], d1["provenance"])
    if artifact["provenance_differences"] != expected_prov_diffs:
        raise ArtifactError("provenance_differences 與實際來源重算的結果不符")

    validate_provenance(artifact["comparator_provenance"], role="comparator")
    if artifact["comparator_provenance"] != comparator_provenance:
        raise ArtifactError(
            "comparator_provenance 與本次執行實際產生的 provenance 不符——"
            "⚠️ 只驗 schema ⛔ 抓不到格式合法但內容偽造的值"
        )

    rows_match = _require_bool(artifact, "rows_match")
    provenance_match = _require_bool(artifact, "provenance_match")
    matched = _require_bool(artifact, "matched")
    expected_rows_match = (
        expected_d_keys == expected_d1_keys and not exp_d_only and not exp_d1_only
        and not expected_diffs
    )
    if rows_match != expected_rows_match:
        raise ArtifactError(f"rows_match={rows_match} 與重算的 {expected_rows_match} 不符")
    if provenance_match != (not expected_prov_diffs):
        raise ArtifactError("provenance_match 與重算結果不符")
    if matched != (rows_match and provenance_match):
        raise ArtifactError("matched 必須等於 rows_match and provenance_match")

    outcome = artifact["outcome"]
    if outcome not in _OUTCOMES:
        raise ArtifactError(f"未知的 outcome={outcome!r}")
    expected_outcome = {
        (True, True): OUTCOME_MATCH, (False, True): OUTCOME_ROW,
        (True, False): OUTCOME_PROVENANCE, (False, False): OUTCOME_BOTH,
    }[(rows_match, provenance_match)]
    if outcome != expected_outcome:
        raise ArtifactError(
            f"outcome={outcome} 與旗標組合不符（rows_match={rows_match}、"
            f"provenance_match={provenance_match} → 應為 {expected_outcome}）"
        )

    generated_at = _parse_tz_aware(artifact["generated_at"], "generated_at")
    if generated_at < _parse_tz_aware(artifact["d1_generated_at"], "d1_generated_at"):
        raise ArtifactError("crossday 的 generated_at ⛔ 不得早於 d1_generated_at")


# ── CLI（⚠️ **獨立入口**，⛔ 不走 `evaluation.py` 的 parser）──────────────────
#
# `resolve_cli_mode()` 只有 `stage0`／`bundle`／`legacy` 三種模式，`--d`／`--d1` ⛔ 沒有落點；
# 而 crossday 根本不跑 replay。所以它有自己的 `python -m …replay_bundle.crossday`。

# ⚠️ **自己的 ownership 清單**，⛔ **不共用 `REPLAY_INJECTED_ARGS`**：
# 那份會連「唯一前綴縮寫」一起擋，而 `--run-identity` 的前綴正好是既有的合法參數
# `--run-id`——共用清單會讓 Stage 1／2 的 `--run-id` 直接壞掉（實測過）。
CROSSDAY_INJECTED_ARGS = (
    "--run-identity", "--image-digest", "--base-commit",
    "--tooling-patch-sha256", "--source-root", "--runner-sha256",
)


class CrossdayUsageError(ValueError):
    """CLI 用法錯誤。"""


def assert_crossday_arg_ownership(argv: Sequence[str]) -> None:
    """⚠️ 注入參數只能由官方腳本給，且**⛔ 不得重複**。

    ⛔ 這裡⛔ 不做前綴比對——crossday 的 parser 已經 `allow_abbrev=False`，
    而前綴比對會誤殺別的合法參數（見上方註解）。
    """
    for injected in CROSSDAY_INJECTED_ARGS:
        count = sum(
            1 for a in argv if a == injected or a.startswith(injected + "=")
        )
        if count > 1:
            raise CrossdayUsageError(
                f"{injected} 出現 {count} 次——它只能由官方腳本注入，"
                "重複代表使用者也傳了一個。⛔ 不靜默採用最後一個。"
            )


def build_crossday_parser():
    import argparse

    # ⚠️ `allow_abbrev=False`：argparse 預設會把 `--run-id` 展開成 `--run-identity`。
    parser = argparse.ArgumentParser(prog="crossday", allow_abbrev=False)
    parser.add_argument("--d", required=True)
    parser.add_argument("--d1", required=True)
    parser.add_argument("--output-dir", required=True)
    parser.add_argument("--run-identity", required=True)
    parser.add_argument("--image-digest", required=True)
    parser.add_argument("--base-commit", required=True)
    parser.add_argument("--tooling-patch-sha256", required=True)
    parser.add_argument("--source-root", required=True)
    parser.add_argument("--runner-sha256", required=True)
    return parser


def run_crossday(argv: Sequence[str]) -> dict[str, Any]:
    """完整流程。`outcome != MATCH` 時**先發布再**拋 `CrossdayMismatch`。"""
    from datetime import datetime as _dt

    from .artifacts import prepare_output_dir, publish_artifacts
    from .provenance import build_provenance
    from .run_identity import load_run_identity

    assert_crossday_arg_ownership(argv)
    args = build_crossday_parser().parse_args(list(argv))

    identity = load_run_identity(args.run_identity)
    d, d_sha = load_after_for_crossday(args.d)
    d1, d1_sha = load_after_for_crossday(args.d1)

    # ⚠️ **identity 是獨立的第二來源**：兩份 after 只能互比，若**兩份都帶同一個錯誤
    # `bundle_id`**，沒有它就沒有任何東西能發現。
    for label, artifact in (("D", d), ("D+1", d1)):
        if artifact["bundle_id"] != identity["bundle_id"]:
            raise ArtifactError(
                f"{label} 的 bundle_id={artifact['bundle_id']!r} 與 run identity 記的 "
                f"{identity['bundle_id']!r} 不符"
            )

    assert_valid_crossday_inputs(d=d, d1=d1, d_sha=d_sha, d1_sha=d1_sha)
    output_dir = prepare_output_dir(args.output_dir)

    # ⚠️ provenance 建在**實際工作之後**：比對用到的模組要先 import 完，
    # 太早建的話 `project_modules_sha256` 少記的正是實際跑過的那些檔案。
    comparator_provenance = build_provenance(
        source_root=args.source_root,
        image_digest=args.image_digest,
        base_commit=args.base_commit,
        tooling_patch_sha256=args.tooling_patch_sha256,
        runner_sha256=args.runner_sha256,
        argv=list(argv),
    )
    generated_at = _dt.now(timezone.utc).isoformat()
    artifact = build_crossday(
        d=d, d1=d1, d_sha=d_sha, d1_sha=d1_sha,
        comparator_provenance=comparator_provenance, generated_at=generated_at,
    )
    # ⚠️ 順序定死：build → validate → publish → raise。
    # ⛔ validator 沒過就⛔ 不留下一份沒通過驗證的證據（那時是一般中止，⛔ 不是 exit 5）。
    validate_crossday(
        artifact, d=d, d1=d1, d_sha=d_sha, d1_sha=d1_sha,
        comparator_provenance=comparator_provenance,
    )
    hashes = publish_artifacts(output_dir, [(CROSSDAY_ARTIFACT_NAME, artifact)])

    result = {
        "outcome": artifact["outcome"],
        "matched": artifact["matched"],
        "output_dir": str(output_dir),
        "crossday_sha256": hashes[CROSSDAY_ARTIFACT_NAME],
    }
    if artifact["outcome"] != OUTCOME_MATCH:
        # ⚠️ **這是終止狀態，⛔ 不是失敗殘骸**——artifact 已經完整發布。
        raise CrossdayMismatch(
            f"跨日比對不一致：outcome={artifact['outcome']}"
            f"（rows_match={artifact['rows_match']}、"
            f"provenance_match={artifact['provenance_match']}）。"
            f"證據已寫入 {output_dir / CROSSDAY_ARTIFACT_NAME}"
            f"（SHA-256 {hashes[CROSSDAY_ARTIFACT_NAME]}）。",
            path=output_dir / CROSSDAY_ARTIFACT_NAME,
            outcome=artifact["outcome"],
        )
    return result


def main(argv: Sequence[str] | None = None) -> int:
    import json
    import sys

    from .publish import EXIT_ABORT, EXIT_CROSSDAY_MISMATCH

    argv = list(sys.argv[1:] if argv is None else argv)
    try:
        result = run_crossday(argv)
    except CrossdayMismatch as exc:
        # ⚠️ **必須排在 generic catch 之前**，而且 `CrossdayMismatch` ⛔ 不繼承 ValueError，
        # 否則這個專屬碼出不來。
        print(str(exc), file=sys.stderr)
        return EXIT_CROSSDAY_MISMATCH
    except (CrossdayUsageError, ValueError, OSError) as exc:
        print(f"ERROR: {type(exc).__name__}: {exc}", file=sys.stderr)
        return EXIT_ABORT
    print(json.dumps(result, ensure_ascii=False, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
