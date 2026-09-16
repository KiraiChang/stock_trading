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
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Iterable, Sequence

from .canonical import canonical_gzip_bytes, canonical_json_bytes, gunzip_bytes, sha256_hex
from .publish import fsync_dir, fsync_file

ARTIFACT_SCHEMA_VERSION = 1

AFTER_ARTIFACT_NAME = "after_artifact.json"
COHORT_MANIFEST_NAME = "cohort_manifest.json"
COMPARISON_ARTIFACT_NAME = "comparison_artifact.json"
REPORT_NAME = "report.json"
MISMATCH_ARTIFACT_NAME = "candidate_mismatch.json"

AFTER_KIND = "sr_zone_replay_after"
COHORT_KIND = "sr_zone_replay_cohort"
COMPARISON_KIND = "sr_zone_replay_comparison"
REPORT_KIND = "sr_zone_replay_report"
MISMATCH_KIND = "sr_zone_replay_candidate_mismatch"

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


# ── I-074 Stage 0 的九個診斷欄位 ────────────────────────────────────────────
#
# ⚠️ 只驗 candidate 是不夠的：少了 `action_state`／`position_action_condition`／
# `structure_state` 或型別錯了，正式 artifact 照樣發布——而那份是**唯一一次正式 scan**
# 的產物，等到判讀分支 B／C 時才發現就來不及了。

DIAGNOSTIC_BOOL_FIELDS = (
    "clear_zone_breakout",
    "continuation_price_evidence_met",
    "setup_rr_qualified",
    CANDIDATE_FIELD,
)
DIAGNOSTIC_STR_FIELDS = ("event_signal", "structure_state", "action_state")
# ⚠️ 這兩個可以是 null——但**只有明確的 no-zone fallback 列**才允許（見 validate_diagnostics）。
DIAGNOSTIC_NULLABLE_FIELDS = ("position_action_condition", "position_action")

NO_ZONE_SCORES_ERROR = "NO_ZONE_SCORES"

# ⚠️ **no-zone 列的合法缺席 fallback**（契約見 docs/sr-zone-scoring.md「九個診斷欄位的完整 schema」；計畫書已收斂）。
#
# ⛔ 這**不是**重算上游判斷，語意是「這一列沒有 decision summary 可匯出」。
# ⚠️ 觸發條件**只有**「真的沒有 zone」：`zone_score_available == False` 且
# `zone_score_error == "NO_ZONE_SCORES"`。zone 在、但 trend／volatility／metrics 缺的情況
# **是異常**，⛔ 不得套這組預設值，要走 fail-closed。
#
# ⚠️ **這組值同時是匯出端的預設與驗證端的契約**，所以落在 package 這一層：
# `evaluation.py` 匯出 no-zone 列時用它，`validate_diagnostics()` 逐欄對照也用它。
# ⛔ 兩邊各寫一份的話，改了一邊而另一邊沒跟上時**沒有任何東西會報錯**——
# 這正是先前 `NO_ZONE_SCORES_ERROR` 在兩個模組各有一份同值常數的狀況。
DIAGNOSTIC_NO_ZONE_FALLBACK: dict[str, Any] = {
    "clear_zone_breakout": False,              # 沒有 primary zone 就不可能有突破
    "continuation_price_evidence_met": False,  # 三項證據之一必然不成立
    "setup_rr_qualified": False,               # 沒有 zone 就沒有 setup gate
    CANDIDATE_FIELD: False,                    # ⚠️ 真實的 false，⛔ 不是「不知道」
    "event_signal": "NO_EVENT",                # 與 resolve_event_signal() 的預設一致
    "structure_state": "UNKNOWN",              # ⛔ 不猜
    "action_state": "WATCH",                   # 與 semantic pipeline 的預設一致
    "position_action_condition": None,
    "position_action": None,
}

# 九個診斷欄位的名稱（給匯出與驗證兩端共用，⛔ 不在兩處各列一份）。
DIAGNOSTIC_FIELDS = tuple(DIAGNOSTIC_NO_ZONE_FALLBACK)


def validate_replay_errors(rows: Iterable[dict[str, Any]], label: str) -> None:
    """⚠️ **「合法零 zone」與「運算失敗」要分開。**

    `zone_score_error == "NO_ZONE_SCORES"` 是合法的——那一列本來就沒有 zone，
    `candidate = false` 是**真實的 false**。⛔ 其餘任何 error 值都是運算失敗，
    在發布 artifact 之前就要中止。

    ⚠️ **原本是 `evaluation.py` 的私有 `_assert_no_replay_errors()`**（I-074 Stage 1 搬來）：
    probe 與 finalizer 也要用它，而本 package ⛔ 不得反向 import `evaluation.py`；
    另抄一份就是**雙真相源**（`NO_ZONE_SCORES_ERROR` 已經這樣犯過一次）。
    ⛔ **行為一字不改**——`ArtifactError` 是 `ValueError` 的子類，既有的
    `pytest.raises(ValueError)` 與 CLI 的 generic catch 都照樣接得到。
    """
    failures: list[str] = []
    for row in rows:
        zone_error = row.get("zone_score_error")
        if zone_error and zone_error != NO_ZONE_SCORES_ERROR:
            failures.append(f"{row.get('symbol')}@{row.get('as_of')} zone_score_error={zone_error}")
        if row.get("decision_error"):
            failures.append(f"{row.get('symbol')}@{row.get('as_of')} decision_error={row['decision_error']}")
    if failures:
        raise ArtifactError(
            f"{label} 有 {len(failures)} 列運算失敗，⛔ 不發布任何 artifact："
            + "；".join(failures[:5]) + ("…" if len(failures) > 5 else "")
        )


def _is_no_zone_fallback_row(row: dict[str, Any]) -> bool:
    """這一列是不是「真的沒有 zone」的合法缺席列。

    ⚠️ 條件是**兩個都要成立**：zone 不可用，**且**成因正好是 `NO_ZONE_SCORES`。
    zone 在、但 trend／volatility／metrics 缺的情況⛔ 不算——那是異常。
    """
    return row.get("zone_score_available") is False and row.get("zone_score_error") == NO_ZONE_SCORES_ERROR


def validate_diagnostics(rows: Iterable[dict[str, Any]], label: str, *, side: str = "after") -> None:
    """九個診斷欄位 ＋ `lifecycle_phase` 的 schema 與交叉一致性。

    ⛔ **`side` 只接受 `"before"` / `"after"`**：打錯字（例如 `"After"`）會讓 after 的等價式
    被**靜默跳過**，那是 fail-open。

    `side="after"` 時額外驗 candidate 的等價式；`side="before"` ⛔ **不驗展開式**——
    展開式需要 `active_bearish_states`，那是 `event_state_summary` 的內容、lifecycle 的
    獨立輸入，**光靠 row 算不出來**。before 的展開式在 before tooling 內驗（它還持有原始
    `event_state_summary`），那是 Stage 2 的 patch 責任。
    """
    if side not in ("before", "after"):
        raise ArtifactError(f"validate_diagnostics 的 side 只接受 'before'／'after'，實際 {side!r}")

    for row in rows:
        key = row_key(row)
        where = f"{label} {key}"
        no_zone = _is_no_zone_fallback_row(row)

        for field in DIAGNOSTIC_BOOL_FIELDS:
            if field not in row:
                raise ArtifactError(f"{where}：缺少 {field}。{_I074_HINT}")
            value = row[field]
            # ⛔ 不能用 isinstance(x, int)：bool 是 int 的子類，`0`／`1` 會通過。
            if not isinstance(value, bool):
                raise ArtifactError(f"{where}：{field} 必須是 JSON true/false，實際 {value!r}")

        for field in DIAGNOSTIC_STR_FIELDS:
            if field not in row:
                raise ArtifactError(f"{where}：缺少 {field}")
            value = row[field]
            if not isinstance(value, str) or not value:
                raise ArtifactError(f"{where}：{field} 必須是非空字串，實際 {value!r}")

        for field in DIAGNOSTIC_NULLABLE_FIELDS:
            if field not in row:
                raise ArtifactError(f"{where}：缺少 {field}")

        # ⛔ `position_action` 可以是 null，但**非 null 時必須是非空字串**——
        # 只驗「存在」等於放行任何型別。
        action = row["position_action"]
        if action is not None and (not isinstance(action, str) or not action):
            raise ArtifactError(f"{where}：position_action 非 null 時必須是非空字串，實際 {action!r}")

        condition = row["position_action_condition"]
        if condition is None:
            # ⚠️ 只有明確的 no-zone fallback 列才允許為 null。
            if not no_zone:
                raise ArtifactError(
                    f"{where}：position_action_condition 為 null，但這一列**不是** no-zone "
                    f"fallback（zone_score_available={row.get('zone_score_available')!r}、"
                    f"zone_score_error={row.get('zone_score_error')!r}）"
                )
        else:
            if not isinstance(condition, dict):
                raise ArtifactError(f"{where}：position_action_condition 必須是 object")
            for sub in ("state", "structure_state"):
                value = condition.get(sub)
                if not isinstance(value, str) or not value:
                    raise ArtifactError(
                        f"{where}：position_action_condition.{sub} 必須是非空字串，實際 {value!r}"
                    )
            # ── 欄位交叉一致性 ──────────────────────────────────────────────
            # ⛔ 少了它，互相矛盾的證據仍會通過。
            if row["structure_state"] != condition["structure_state"]:
                raise ArtifactError(
                    f"{where}：structure_state={row['structure_state']!r} 與 "
                    f"position_action_condition.structure_state={condition['structure_state']!r} 不一致"
                )
            # ⚠️ `_position_action_condition()` 寫的是 `action_state or "WATCH"`，所以
            # `action_state` 為空字串時兩者會分岔——而空字串已被上面的「非空字串」擋掉，
            # 這裡只需驗非空時必須相等。
            if row["action_state"] != condition["state"]:
                raise ArtifactError(
                    f"{where}：action_state={row['action_state']!r} 與 "
                    f"position_action_condition.state={condition['state']!r} 不一致"
                )

        # ── no-zone 列：**逐欄對照完整的固定 mapping** ─────────────────────────
        #
        # ⛔ 「它是 fallback」不等於「它可以是任何值」——那九個值**整組都是契約**
        # （`DIAGNOSTIC_NO_ZONE_FALLBACK`，與匯出端同一份定義）。
        #
        # ⚠️ 這道檢查排在型別與交叉一致性**之後**：前面已保證型別合法，這裡只比值，
        # 失敗訊息才指得出「是固定 mapping 不符」而不是「型別錯」。
        #
        # ⚠️ **只釘三個 boolean 是不夠的**，先前漏掉的三類都會靜默通過：
        #   * `rr_decoupling_candidate=true`——after 側會被等價式間接擋下
        #     （lifecycle_phase 是 null），但 **before 側⛔ 不驗等價式**，於是照樣放行；
        #   * 三個字串被改成一般列的值（整列偽裝成「有算過」）；
        #   * `position_action_condition`／`position_action` 非 null。
        if no_zone:
            for field, want in DIAGNOSTIC_NO_ZONE_FALLBACK.items():
                actual = row[field]
                # bool 與 None 用 `is`（⛔ `0 == False` 會過）；字串用 `==`。
                matched = actual is want if want is None or isinstance(want, bool) else actual == want
                if not matched:
                    raise ArtifactError(
                        f"{where}：no-zone fallback 列的 {field} 必須是 {want!r}，實際 {actual!r}"
                        "——⛔ 那九個值整組都是契約，不是「可以是任何值」的預設"
                    )

        # ── 既有依賴欄位 `lifecycle_phase` ────────────────────────────────────
        # ⛔ 少了它，等價式會被空值蒙混：candidate 為 false 時
        # `false == (None == "CONTINUATION" and …)` → `false == false` → 照樣通過。
        if "lifecycle_phase" not in row:
            raise ArtifactError(f"{where}：缺少 lifecycle_phase（candidate 的等價式依賴它）")
        phase = row["lifecycle_phase"]
        if no_zone:
            if phase is not None:
                raise ArtifactError(
                    f"{where}：no-zone fallback 列的 lifecycle_phase 必須是 null，實際 {phase!r}"
                )
        elif not isinstance(phase, str) or not phase:
            raise ArtifactError(f"{where}：lifecycle_phase 必須是非空字串，實際 {phase!r}")

        if side == "after":
            expected = bool(phase == "CONTINUATION" and not row["setup_rr_qualified"])
            if row[CANDIDATE_FIELD] != expected:
                raise ArtifactError(
                    f"{where}：{CANDIDATE_FIELD}={row[CANDIDATE_FIELD]!r} 與等價式不符"
                    f"（lifecycle_phase={phase!r}、setup_rr_qualified={row['setup_rr_qualified']!r}）"
                )


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


# ── 候選集合不一致：終止狀態的證據 ──────────────────────────────────────────
#
# ⚠️ 這是 Stage 2 的**第二種 terminal outcome**，⛔ 不是失敗殘骸：
# before／after 的候選集合不一致**本身就是分支 C**（tooling 不對稱）。
# 此時⛔ 不產出 comparison／report，改發布這一份。

_MISMATCH_FIELDS = {
    "schema_version", "kind", "bundle_id", "after_artifact_sha256", "before_ref",
    "generated_at", "provenance", "before_only", "after_only",
    "before_candidate_count", "after_candidate_count", "rows",
}


class CandidateMismatch(Exception):
    """before／after 的候選集合不一致。

    ⛔ **刻意不繼承 `ValueError`**：`ArtifactError` 是 `ValueError` 的子類，而 CLI 的
    bundle 分支用 `except (CliUsageError, ValueError, OSError) → sys.exit(1)` 統一收斂。
    繼承下去的話，`EXIT_CANDIDATE_MISMATCH` 這個專屬碼**根本出不來**。
    """

    def __init__(self, message: str, *, path) -> None:
        super().__init__(message)
        self.path = path


def build_candidate_mismatch(
    *, bundle_id: str, after_artifact_sha256: str, before_ref: str, generated_at: str,
    provenance: dict[str, Any], before_only: Sequence[tuple], after_only: Sequence[tuple],
    before_candidate_count: int, after_candidate_count: int,
    before_by_key: dict, after_by_key: dict,
) -> dict[str, Any]:
    """⚠️ `rows` **直接重用 `compare_rows()`**，⛔ 不自訂 wrapper。

    理由：`differences` 是免費得到的，而它**直接指出兩邊差在哪些欄位**——那正是調查
    tooling 不對稱要看的；而且與 comparison artifact 真正同形狀，判讀工具可共用。

    ⚠️ **差集裡每個 key 的兩側 row 都要存**：Stage 2 的 before rows 只在記憶體，
    正常路徑靠 comparison artifact 保存，而這條路徑⛔ 不產 comparison。那是一次 3 小時
    以上、⛔ 不允許用重跑取代結果的正式 replay，證據必須一次到位。
    """
    mismatch_keys = sorted(set(before_only) | set(after_only))
    return _envelope(
        MISMATCH_KIND,
        bundle_id=bundle_id, after_artifact_sha256=after_artifact_sha256,
        before_ref=before_ref, generated_at=generated_at, provenance=provenance,
        before_only=[list(k) for k in sorted(before_only)],
        after_only=[list(k) for k in sorted(after_only)],
        before_candidate_count=int(before_candidate_count),
        after_candidate_count=int(after_candidate_count),
        rows=[compare_rows(before_by_key[k], after_by_key[k]) for k in mismatch_keys],
    )


def validate_candidate_mismatch(
    artifact: dict[str, Any],
    *,
    before_by_key: dict[tuple[str, str, str], dict[str, Any]],
    after_by_key: dict[tuple[str, str, str], dict[str, Any]],
    expected_after_sha256: str,
) -> None:
    """封閉 schema ＋ 全部不變條件 ＋ **與實際來源精確比對**。

    ⚠️ **必須在 publish 之前被實際呼叫**（契約順序：build → validate → publish → raise）。

    驗三層，缺一層都留得下一份看起來可用的錯誤證據：

    1. **格式**——封閉欄位集合、型別、排序、唯一、互斥；
    2. **內容自我一致**——差值公式、數量下界、`differences` 與重算值相同、
       差集方向與兩側 candidate 值相符（⛔ 少了它，格式合法但內容顛倒的 artifact 照樣通過）；
    3. **與實際來源比對**——集合、計數、每列兩側 row、after artifact 的 SHA。

    ⚠️ **三個來源參數是必填的**，⛔ 不提供「不給就跳過」的模式：可選等於留一道 fail-open，
    而 artifact 只要**內部自洽**就能通過前兩層——它可以漏掉一個真實的 mismatch，
    自己卻完全對得起來。那是一次 3 小時以上、⛔ 不允許用重跑取代結果的正式 replay，
    唯一能證明產物完整的東西就是本次 replay 的實際 row。

    ⚠️ 第三層⛔ **不是重算 predicate**：`rr_decoupling_candidate` 是上游產好的嚴格 boolean，
    這裡只消費它來重建集合與計數。
    """
    _require_fields(artifact, _MISMATCH_FIELDS, "candidate mismatch")
    # ⛔ 受管制檔案：未知 schema_version 一律中止（與 I-100 的其他 artifact 同一條規則）。
    version = artifact["schema_version"]
    if type(version) is not int or version != ARTIFACT_SCHEMA_VERSION:
        raise ArtifactError(
            f"未知的 candidate mismatch schema_version={version!r}"
            f"（本實作只支援 {ARTIFACT_SCHEMA_VERSION}）"
        )
    if artifact["kind"] != MISMATCH_KIND:
        raise ArtifactError(f"candidate mismatch 的 kind={artifact['kind']!r}，預期 {MISMATCH_KIND!r}")
    for field in ("bundle_id", "before_ref", "generated_at"):
        _require_type(artifact, field, str, "candidate mismatch")
        if not artifact[field]:
            raise ArtifactError(f"candidate mismatch 的 {field} 不得為空字串")
    _require_type(artifact, "provenance", dict, "candidate mismatch")

    digest = artifact["after_artifact_sha256"]
    if not isinstance(digest, str) or len(digest) != 64 or digest != digest.lower() \
            or not all(c in "0123456789abcdef" for c in digest):
        raise ArtifactError(f"after_artifact_sha256 必須是 64 字元十六進位小寫：{digest!r}")

    def _keys(field: str) -> list[tuple[str, str, str]]:
        raw = artifact[field]
        if not isinstance(raw, list):
            raise ArtifactError(f"candidate mismatch 的 {field} 必須是陣列")
        out = []
        for item in raw:
            if not isinstance(item, list) or len(item) != 3 \
                    or any(not isinstance(v, str) or not v for v in item):
                raise ArtifactError(f"{field} 的 key 必須是三個非空字串：{item!r}")
            out.append((item[0], item[1], item[2]))
        assert_unique_keys(out, f"candidate mismatch 的 {field}")
        if out != sorted(out):
            raise ArtifactError(f"{field} 必須排序")
        return out

    before_only, after_only = _keys("before_only"), _keys("after_only")
    # ⛔ 擋掉「零差集卻宣稱 mismatch」。
    if not before_only and not after_only:
        raise ArtifactError("candidate mismatch 的差集是空的——⛔ 那不構成 mismatch")
    overlap = set(before_only) & set(after_only)
    if overlap:
        raise ArtifactError(f"before_only 與 after_only 必須互斥，重疊：{sorted(overlap)[:5]}")

    counts = {}
    for field in ("before_candidate_count", "after_candidate_count"):
        value = artifact[field]
        # ⚠️ `type(x) is int`：⛔ 排除 bool（它是 int 的子類）。
        if type(value) is not int or value < 0:
            raise ArtifactError(f"{field} 必須是非負整數：{value!r}")
        counts[field] = value
    # 差值公式。
    if counts["before_candidate_count"] - counts["after_candidate_count"] != len(before_only) - len(after_only):
        raise ArtifactError(
            f"候選數差（{counts['before_candidate_count']} − {counts['after_candidate_count']}）"
            f"與差集長度差（{len(before_only)} − {len(after_only)}）不符"
        )
    # ⚠️ **下界**——⛔ 差值公式取代不了它：「兩側 count 都是 0、兩側差集各一筆」
    # 會通過 `0−0 == 1−1`，但那在集合上根本不可能成立。
    if counts["before_candidate_count"] < len(before_only):
        raise ArtifactError(
            f"before_candidate_count={counts['before_candidate_count']} < "
            f"len(before_only)={len(before_only)}——集合上不可能"
        )
    if counts["after_candidate_count"] < len(after_only):
        raise ArtifactError(
            f"after_candidate_count={counts['after_candidate_count']} < "
            f"len(after_only)={len(after_only)}——集合上不可能"
        )

    mismatch_keys = sorted(set(before_only) | set(after_only))
    rows = artifact["rows"]
    if not isinstance(rows, list):
        raise ArtifactError("candidate mismatch 的 rows 必須是陣列")
    row_fields = {"symbol", "timeframe", "as_of", "differences", "before", "after"}
    actual_keys = []
    for item in rows:
        if not isinstance(item, dict):
            raise ArtifactError("rows[] 的每一項都必須是 object")
        _require_fields(item, row_fields, "candidate mismatch 的 rows[]")
        for side in ("before", "after"):
            # ⚠️ 第五道檢查排在②之後，②已保證兩邊 row key 集合相同，
            # 所以差集裡每個 key **兩邊一定都有 row**。
            if not isinstance(item[side], dict):
                raise ArtifactError(f"rows[].{side} 必須是 object，⛔ 不得為 null")
        outer = row_key(item)
        if not (outer == row_key(item["before"]) == row_key(item["after"])):
            raise ArtifactError(
                f"rows[] 的外層 key {outer} 與內嵌兩側 row 的 key 不一致"
            )
        # `differences` 是正式證據，必須與 compare_rows() 算出完全相同的值。
        expected = sorted(
            f for f in set(item["before"]) | set(item["after"])
            if item["before"].get(f) != item["after"].get(f)
        )
        if item["differences"] != expected:
            raise ArtifactError(
                f"rows[] {outer} 的 differences 與重算值不符（⛔ 不接受空陣列／錯欄位／重複／未排序）"
            )
        # ⚠️ mismatch key 來自候選集合的 symmetric difference，所以 candidate 兩側必然不同。
        if CANDIDATE_FIELD not in item["differences"]:
            raise ArtifactError(
                f"rows[] {outer} 的 differences 不含 {CANDIDATE_FIELD}——"
                "但它來自 symmetric difference，兩側必然不同，產物本身有問題"
            )
        # ⚠️ **差集方向必須與兩側 candidate 值相符**：
        # `before_only` 的列 → before=true／after=false；`after_only` 的列 → 相反。
        expected_before = outer in set(before_only)
        for side, want in (("before", expected_before), ("after", not expected_before)):
            actual = item[side].get(CANDIDATE_FIELD)
            if actual is not want:
                raise ArtifactError(
                    f"rows[] {outer} 的方向不符：它在 "
                    f"{'before_only' if expected_before else 'after_only'} 裡，"
                    f"所以 {side} 側的 {CANDIDATE_FIELD} 應該是 {want}，實際 {actual!r}"
                )
        actual_keys.append(outer)
    if actual_keys != mismatch_keys:
        raise ArtifactError("rows 的 key 序列必須恰好等於 sorted(before_only ∪ after_only) 且同序")

    # ── 第三層：與實際來源精確比對 ──────────────────────────────────────────
    def _source_candidates(by_key: dict, label: str) -> set:
        keys = set()
        for key, row in by_key.items():
            value = row.get(CANDIDATE_FIELD)
            # ⛔ 來源列的 candidate 必須是嚴格 boolean：缺值或 None 被當成 false 的話，
            # 一個真實候選會從重建出來的集合裡靜默消失，比對反而「通過」。
            if not isinstance(value, bool):
                raise ArtifactError(
                    f"{label} 來源列 {key} 的 {CANDIDATE_FIELD}={value!r} 不是嚴格 boolean"
                )
            if value:
                keys.add(key)
        return keys

    source_before = _source_candidates(before_by_key, "before")
    source_after = _source_candidates(after_by_key, "after")

    expected_before_only = sorted(source_before - source_after)
    expected_after_only = sorted(source_after - source_before)
    if before_only != expected_before_only:
        raise ArtifactError(
            f"before_only 與實際來源不符：artifact 記 {len(before_only)} 筆、"
            f"來源算出 {len(expected_before_only)} 筆；"
            f"僅在 artifact={sorted(set(before_only) - set(expected_before_only))[:5]}、"
            f"僅在來源={sorted(set(expected_before_only) - set(before_only))[:5]}"
        )
    if after_only != expected_after_only:
        raise ArtifactError(
            f"after_only 與實際來源不符：artifact 記 {len(after_only)} 筆、"
            f"來源算出 {len(expected_after_only)} 筆；"
            f"僅在 artifact={sorted(set(after_only) - set(expected_after_only))[:5]}、"
            f"僅在來源={sorted(set(expected_after_only) - set(after_only))[:5]}"
        )
    for field, source in (("before_candidate_count", source_before),
                          ("after_candidate_count", source_after)):
        if counts[field] != len(source):
            raise ArtifactError(
                f"{field}={counts[field]} 與實際來源的候選列數 {len(source)} 不符"
            )

    if artifact["after_artifact_sha256"] != expected_after_sha256:
        raise ArtifactError(
            f"after_artifact_sha256={artifact['after_artifact_sha256']} 與實際載入的 "
            f"after artifact SHA-256 {expected_after_sha256} 不符——⛔ 這份 mismatch "
            "指向的不是本次比較的那個 artifact"
        )

    # ⚠️ 內嵌的兩側 row 必須**就是**本次 replay 的實際 row，⛔ 不只是「形狀對」。
    for item in rows:
        key = row_key(item)
        for side, by_key in (("before", before_by_key), ("after", after_by_key)):
            if key not in by_key:
                raise ArtifactError(f"rows[] {key} 在 {side} 的實際來源裡不存在")
            if item[side] != by_key[key]:
                raise ArtifactError(
                    f"rows[] {key} 的 {side} 側內容與實際來源的 row 不符"
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


@dataclass(frozen=True)
class EvidenceLoad:
    """`load_canonical_evidence_artifact()` 的回傳。

    ⚠️ **四個值都來自同一次讀取**——finalizer 要用 `stored_*` 建 manifest，若它自己再讀一次
    檔案，`parsed`／`artifact_sha256` 與那份 raw bytes 可能已經不是同一版本。
    """

    parsed: dict[str, Any]
    artifact_sha256: str   # 解壓後 bytes 的 SHA（`.json` 時即 raw bytes）
    stored_sha256: str     # **實際落地檔案** raw bytes 的 SHA
    stored_bytes: int      # 該 raw bytes 的長度


def load_canonical_evidence_artifact(path: str | Path, kind: str) -> EvidenceLoad:
    """evidence 的統一載入入口：`.json` 與 `.json.gz` 都收。

    ⚠️ 既有的 `load_artifact()` 直接把檔案當 UTF-8 JSON 讀，**⛔ 讀不了 gzip**；
    而本輪 archived evidence 是 `.json.gz`。⛔ **不改 `load_artifact()` 的公開 API**，
    改在這裡分流：

    * `.json`——呼叫既有 `load_artifact()`，再驗 **raw bytes 就是 canonical bytes**
      （⚠️ 用 SHA 對照，⛔ **不重讀檔案**：`load_artifact()` 只回傳 `(parsed, sha)`，
      raw bytes 不會傳出，再讀一次可能已不是同一版本）；
    * `.json.gz`——驗 **round-trip byte-identical**（⛔ 只驗 mtime／filename 兩個 header 欄位
      擋不住 compression level／XFL／OS byte 的差異），解壓後再驗 canonical 與 schema／kind。
    """
    import json

    path = Path(path)
    name = path.name
    if name.endswith(".json"):
        parsed, raw_sha = load_artifact(path, kind)
        if raw_sha != sha256_hex(canonical_json_bytes(parsed)):
            raise ArtifactError(
                f"{name} 的內容不是 canonical JSON——⛔ 語意相同但編碼不同也不接受"
            )
        return EvidenceLoad(parsed, raw_sha, raw_sha, path.stat().st_size)

    if not name.endswith(".json.gz"):
        raise ArtifactError(f"evidence 只接受 .json 或 .json.gz：{name}")

    try:
        raw = path.read_bytes()
    except OSError as exc:
        raise ArtifactError(f"讀不到 evidence {path}：{exc}") from exc
    try:
        payload = gunzip_bytes(raw)
    except Exception as exc:  # noqa: BLE001 - gzip 的例外型別依內容而異
        raise ArtifactError(f"{name} 不是合法 gzip：{exc}") from exc
    if canonical_gzip_bytes(payload) != raw:
        raise ArtifactError(
            f"{name} ⛔ 不是 canonical gzip（round-trip 不是逐位元相同）——"
            "compression level／header 欄位不同都會落在這裡"
        )
    try:
        parsed = json.loads(payload.decode("utf-8"))
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise ArtifactError(f"{name} 解壓後不是合法 JSON：{exc}") from exc
    if not isinstance(parsed, dict):
        raise ArtifactError(f"{name} 必須是 object")
    if payload != canonical_json_bytes(parsed):
        raise ArtifactError(f"{name} 解壓後的內容不是 canonical JSON")
    version = parsed.get("schema_version")
    if type(version) is not int or version != ARTIFACT_SCHEMA_VERSION:
        raise ArtifactError(
            f"未知的 {name} schema_version={version!r}（本實作只支援 {ARTIFACT_SCHEMA_VERSION}）"
        )
    if parsed.get("kind") != kind:
        raise ArtifactError(f"{name} 的 kind={parsed.get('kind')!r}，預期 {kind!r}")
    return EvidenceLoad(parsed, sha256_hex(payload), sha256_hex(raw), len(raw))


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
