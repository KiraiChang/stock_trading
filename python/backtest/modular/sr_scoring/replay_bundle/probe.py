"""capacity probe 的三份 artifact（I-074 Stage 1）。

⚠️ **為什麼要 probe**：正式 Stage 1 是 11 檔合計 13417 列、約 3.2 小時，而這台 host 只有
2GiB、smoke 只驗過 35 列。⛔ 沒量過就開跑，被 OOM 砍掉等於白燒一天。

⛔ **⛔ 不是「單檔小範圍再外推」**：`run-evaluation.sh` 已明訂記憶體「用量隨標的數成長且
**不可線性外推**」。probe **載入同一份正式 bundle 的全部 11 檔與模型**，只把 quota 壓到
固定的小值——這樣量到的峰值才涵蓋「sources ＋ dataset ＋ 模型常駐」這個大宗。

**產出分三份**（⚠️ 產生者不同，所以⛔ 不能塞成一份）：

| 檔案 | 產生者 | 內容 |
|---|---|---|
| computation | **Python** | probe rows、每檔 quota、elapsed |
| measurement | **Shell** | peak RSS／host low／cgroup limit（容器結束後才拿得到） |
| completion | **Shell（最後）** | 前兩份的 `artifact_sha256` ＋ 完成旗標 |

⚠️ **completion 最後才寫**：容器或量測中止時⛔ 不得留下一份**看起來完整**的 probe。
"""
from __future__ import annotations

from datetime import datetime
from typing import Any, Iterable

from .artifacts import (
    ARTIFACT_SCHEMA_VERSION,
    ArtifactError,
    row_key,
    validate_diagnostics,
    validate_replay_errors,
)
from .i074_preflight import I074_SYMBOLS
from .provenance import validate_provenance

PROBE_COMPUTATION_KIND = "sr_zone_probe_computation"
PROBE_MEASUREMENT_KIND = "sr_zone_probe_measurement"
PROBE_COMPLETION_KIND = "sr_zone_probe_completion"

PROBE_COMPUTATION_NAME = "capacity_probe_computation.json"
PROBE_MEASUREMENT_NAME = "capacity_probe_measurement.json"
PROBE_COMPLETION_NAME = "capacity_probe.json"

# ⚠️ **固定 200**，⛔ 不由執行者臨時調整——量測值要能互相比較。
# `MIN_ROWS_PER_SYMBOL = 5`，11 檔的下限是 55，200 有餘裕。
PROBE_QUOTA = 200

_COMMON_FIELDS = {"schema_version", "kind", "bundle_id", "generated_at", "provenance"}
_COMPUTATION_FIELDS = _COMMON_FIELDS | {"quota_by_symbol", "row_count", "rows", "elapsed_seconds"}
_MEASUREMENT_FIELDS = _COMMON_FIELDS | {"peak_rss_bytes", "host_low_bytes", "cgroup_limit_bytes"}
_COMPLETION_FIELDS = _COMMON_FIELDS | {
    "computation_artifact_sha256", "measurement_artifact_sha256", "completed",
}


def _require_common(payload: object, *, kind: str, fields: set[str]) -> dict[str, Any]:
    """共同欄位 ＋ 嚴格型別。

    ⚠️ 型別要嚴格：Python 裡 `True == 1`，只寫「`schema_version == 1`」會放行 `True`。
    """
    if not isinstance(payload, dict):
        raise ArtifactError(f"{kind} 必須是 object")
    actual = set(payload)
    if actual != fields:
        raise ArtifactError(
            f"{kind} 欄位集合不符：多={sorted(actual - fields)}、缺={sorted(fields - actual)}"
        )
    version = payload["schema_version"]
    if type(version) is not int or version != ARTIFACT_SCHEMA_VERSION:
        raise ArtifactError(f"未知的 {kind} schema_version={version!r}")
    if payload["kind"] != kind:
        raise ArtifactError(f"kind={payload['kind']!r}，預期 {kind!r}")
    if not isinstance(payload["bundle_id"], str) or not payload["bundle_id"]:
        raise ArtifactError(f"{kind} 的 bundle_id 必須是非空字串")
    generated_at = payload["generated_at"]
    if not isinstance(generated_at, str) or not generated_at:
        raise ArtifactError(f"{kind} 的 generated_at 必須是非空字串")
    try:
        parsed = datetime.fromisoformat(generated_at)
    except ValueError as exc:
        raise ArtifactError(f"{kind} 的 generated_at 不是合法 ISO-8601：{exc}") from exc
    if parsed.tzinfo is None:
        raise ArtifactError(f"{kind} 的 generated_at ⛔ 必須含時區")
    # ⚠️ 三份都是**同一個 runner invocation 的外層產物**，role 一律 stage1。
    validate_provenance(payload["provenance"], role="stage1")
    return payload


def validate_probe_computation(payload: object) -> None:
    """封閉 schema ＋ **欄位間的不變條件**。

    ⚠️ ⛔ 只寫「`row_count == 200`」不夠：`200.0 == 200` 為真，float 會被放行。
    """
    payload = _require_common(payload, kind=PROBE_COMPUTATION_KIND, fields=_COMPUTATION_FIELDS)

    quota = payload["quota_by_symbol"]
    if not isinstance(quota, dict):
        raise ArtifactError("quota_by_symbol 必須是 object")
    if set(quota) != set(I074_SYMBOLS):
        raise ArtifactError(
            f"quota_by_symbol 的 key 必須恰為那 11 檔："
            f"多={sorted(set(quota) - set(I074_SYMBOLS))}、缺={sorted(set(I074_SYMBOLS) - set(quota))}"
        )
    for symbol, value in quota.items():
        # ⛔ bool 是 int 的子類，要先排除。
        if type(value) is not int or value <= 0:
            raise ArtifactError(f"quota_by_symbol[{symbol}] 必須是正整數：{value!r}")

    row_count = payload["row_count"]
    if type(row_count) is not int:
        raise ArtifactError(f"row_count 必須是 int（⛔ 不接受 float／bool）：{row_count!r}")
    rows = payload["rows"]
    if not isinstance(rows, list):
        raise ArtifactError("rows 必須是陣列")
    elapsed = payload["elapsed_seconds"]
    if type(elapsed) is not float or elapsed <= 0:
        raise ArtifactError(f"elapsed_seconds 必須是正的 float：{elapsed!r}")

    quota_total = sum(quota.values())
    if not (row_count == len(rows) == quota_total == PROBE_QUOTA):
        raise ArtifactError(
            f"row_count={row_count}、len(rows)={len(rows)}、sum(quota)={quota_total}，"
            f"四者必須都等於 {PROBE_QUOTA}"
        )

    keys = [row_key(row) for row in rows]
    if len(set(keys)) != len(keys):
        raise ArtifactError("probe rows 的 key ⛔ 不得重複")
    per_symbol: dict[str, int] = {}
    for symbol, _timeframe, _as_of in keys:
        per_symbol[symbol] = per_symbol.get(symbol, 0) + 1
    if per_symbol != dict(quota):
        raise ArtifactError(f"每檔實際列數 {per_symbol} 與 quota {dict(quota)} 不符")

    # ⚠️ probe 的 rows 同樣要過運算完整性與九欄位守門——⛔ 它是證據，不是暖機輸出。
    validate_replay_errors(rows, "capacity probe")
    validate_diagnostics(rows, "capacity probe", side="after")


def validate_probe_measurement(payload: object) -> None:
    """⚠️ 三個量測值都是**正整數 bytes**，⛔ 不得為 null 或 0 佔位。

    ⛔ `run-evaluation.sh` 現在的做法是「讀不到就警告 ＋ 寫 0」——正式 probe ⛔ 不得把
    「量不到」歸檔成 0 bytes，那會讓後續判讀以為峰值極低。
    """
    payload = _require_common(payload, kind=PROBE_MEASUREMENT_KIND, fields=_MEASUREMENT_FIELDS)
    for field in ("peak_rss_bytes", "host_low_bytes", "cgroup_limit_bytes"):
        value = payload[field]
        if type(value) is not int or value <= 0:
            raise ArtifactError(
                f"{field} 必須是正整數（單位 bytes）：{value!r}——"
                "⛔ 量不到就⛔ 不發布 completion，不得寫 0 佔位"
            )


def validate_probe_completion(
    payload: object, *, computation_sha256: str, measurement_sha256: str
) -> None:
    """completion 必須指向**實際的**前兩份。"""
    payload = _require_common(payload, kind=PROBE_COMPLETION_KIND, fields=_COMPLETION_FIELDS)
    completed = payload["completed"]
    if type(completed) is not bool or completed is not True:
        raise ArtifactError(f"completed 必須是 JSON true：{completed!r}")
    for field, expected in (
        ("computation_artifact_sha256", computation_sha256),
        ("measurement_artifact_sha256", measurement_sha256),
    ):
        value = payload[field]
        if not isinstance(value, str) or len(value) != 64 or value != value.lower():
            raise ArtifactError(f"{field} 必須是 64 字元小寫 hex：{value!r}")
        if value != expected:
            raise ArtifactError(f"{field}={value} 與實際的 {expected} 不符")
