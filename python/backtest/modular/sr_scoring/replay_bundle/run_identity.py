"""跨日執行身分：probe／D／D+1／comparator／finalizer 共用的 `run_identity`（I-074 Stage 1）。

⚠️ **為什麼需要它**：probe／D／D+1 是**跨日、分開啟動**的三個行程。D+1 ⛔ 無從知道前兩趟
當時用的是哪個 image ID——沒有共同狀態的話，「三趟必須用同一個 image」就只能靠操作者手動
傳對值，那不是守門。

⛔ **這個模組必須 dependency-light**：`validate-i074-run-identity.py` 會在 **host**（沒有
pandas）以 `python3 -S` 載入它做 Docker 前的守門。它只能 import 標準庫與同 package 的
`canonical`／`publish`／`artifacts`，⛔ 不得（直接或間接）碰 pandas／sklearn／lightgbm。

兩份 identity 的關係：

* **repo 外的協調檔**（`~/.local/share/stock_trading/i074_stage1/run_identity.json`）——
  操作期用，producer 是 `scripts/pin-replay-image.sh`；
* **archived copy**（`identity/run_identity.json.gz`）——進證據包，⚠️ `--recover-durability`
  **只讀這一份**，所以外部那份遺失時仍能 recovery。
"""
from __future__ import annotations

from pathlib import Path
from typing import Any

from .artifacts import ArtifactError, load_canonical_evidence_artifact
from .canonical import canonical_json_bytes, sha256_hex
from .publish import DurabilityUnconfirmed, fsync_dir, fsync_file, rename_noreplace

RUN_IDENTITY_KIND = "sr_zone_run_identity"
RUN_IDENTITY_NAME = "run_identity.json"
RUN_IDENTITY_ARCHIVE_NAME = "run_identity.json.gz"
RUN_IDENTITY_SCHEMA_VERSION = 1

# 封閉欄位集合。⛔ 缺欄或多一個未知欄位都拒絕。
RUN_IDENTITY_FIELDS = frozenset(
    {"schema_version", "kind", "bundle_id", "expected_image_id", "created_at"}
)


def default_run_identity_path() -> Path:
    """固定推導值。⛔ **不放 `/tmp`**——這份要跨日存活。

    ⚠️ ⛔ 沒有「只給測試用」的路徑覆寫參數（那在實作上強制不了）：測試改為覆寫
    `XDG_DATA_HOME`。
    """
    import os

    base = os.environ.get("XDG_DATA_HOME") or str(Path.home() / ".local" / "share")
    return Path(base) / "stock_trading" / "i074_stage1" / RUN_IDENTITY_NAME


def _is_image_id(value: object) -> bool:
    if not isinstance(value, str) or not value.startswith("sha256:"):
        return False
    digest = value[len("sha256:"):]
    return (
        len(digest) == 64
        and digest == digest.lower()
        and all(c in "0123456789abcdef" for c in digest)
    )


def validate_run_identity(payload: object) -> None:
    """封閉 schema ＋ 精確型別。

    ⚠️ 型別要嚴格：Python 裡 `True == 1`，只寫「`schema_version == 1`」會放行 `True`。
    """
    if not isinstance(payload, dict):
        raise ArtifactError(f"run identity 必須是 object，實際 {type(payload).__name__}")
    actual = set(payload)
    if actual != set(RUN_IDENTITY_FIELDS):
        raise ArtifactError(
            "run identity 欄位集合不符："
            f"多={sorted(actual - RUN_IDENTITY_FIELDS)}、缺={sorted(RUN_IDENTITY_FIELDS - actual)}"
        )
    version = payload["schema_version"]
    if type(version) is not int or version != RUN_IDENTITY_SCHEMA_VERSION:
        raise ArtifactError(
            f"未知的 run identity schema_version={version!r}"
            f"（本實作只支援 {RUN_IDENTITY_SCHEMA_VERSION}）"
        )
    if payload["kind"] != RUN_IDENTITY_KIND:
        raise ArtifactError(f"run identity 的 kind={payload['kind']!r}，預期 {RUN_IDENTITY_KIND!r}")
    bundle_id = payload["bundle_id"]
    if not isinstance(bundle_id, str) or not bundle_id:
        raise ArtifactError(f"run identity 的 bundle_id 必須是非空字串：{bundle_id!r}")
    if not _is_image_id(payload["expected_image_id"]):
        raise ArtifactError(
            f"expected_image_id 必須是 sha256: ＋ 64 字元小寫 hex：{payload['expected_image_id']!r}"
        )
    created_at = payload["created_at"]
    if not isinstance(created_at, str) or not created_at:
        raise ArtifactError(f"created_at 必須是非空字串：{created_at!r}")
    from datetime import datetime

    try:
        parsed = datetime.fromisoformat(created_at)
    except ValueError as exc:
        raise ArtifactError(f"created_at 不是合法的 ISO-8601：{created_at!r}（{exc}）") from exc
    if parsed.tzinfo is None:
        raise ArtifactError(f"created_at ⛔ 必須含時區：{created_at!r}")


def build_run_identity(*, bundle_id: str, expected_image_id: str, created_at: str) -> dict[str, Any]:
    payload = {
        "schema_version": RUN_IDENTITY_SCHEMA_VERSION,
        "kind": RUN_IDENTITY_KIND,
        "bundle_id": bundle_id,
        "expected_image_id": expected_image_id,
        "created_at": created_at,
    }
    validate_run_identity(payload)
    return payload


def load_run_identity(path: str | Path) -> dict[str, Any]:
    """讀 `.json` 或 `.json.gz`，驗 canonical ＋ 封閉 schema。

    ⚠️ 兩種格式共用**同一個** `validate_run_identity()`——⛔ 不讓 shell 另寫一套 schema 檢查。
    """
    loaded = load_canonical_evidence_artifact(path, RUN_IDENTITY_KIND)
    validate_run_identity(loaded.parsed)
    return loaded.parsed


def publish_run_identity(path: str | Path, payload: dict[str, Any]) -> None:
    """原子建立 ＋ **兩次 fsync**。

    ⚠️ **commit point 的三段分流**（與 evidence root 同一條理由）：

    | 階段 | 處置 |
    |---|---|
    | temp 寫入後 `fsync_file` 失敗 | pre-commit：清 temp、拋一般錯誤（正式路徑仍不存在） |
    | `rename_noreplace()` 成功、parent `fsync_dir` 失敗 | ⚠️ **保留檔案**、拋 `DurabilityUnconfirmed` |

    ⛔ 少了後者，「rename 成功但沒落盤」會變成一個**永遠修不好**的狀態——下次執行走
    no-op 分支直接回傳 ID 而不重新 fsync（修復路徑見 `ensure_run_identity()`）。
    """
    import os
    import secrets

    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    validate_run_identity(payload)
    blob = canonical_json_bytes(payload)

    tmp = path.parent / f".{path.name}.{secrets.token_hex(8)}.tmp"
    try:
        tmp.write_bytes(blob)
        fsync_file(tmp)
    except OSError as exc:
        tmp.unlink(missing_ok=True)
        raise ArtifactError(f"run identity 的 temp 寫入或 fsync 失敗：{exc}") from exc

    try:
        rename_noreplace(tmp, path)
    except FileExistsError:
        tmp.unlink(missing_ok=True)
        raise ArtifactError(f"run identity 已存在：{path}——⛔ 不覆蓋") from None
    except OSError as exc:
        tmp.unlink(missing_ok=True)
        raise ArtifactError(f"run identity 的 rename 失敗：{exc}") from exc

    _fsync_parent_or_unconfirmed(path, published=True)


def _fsync_parent_or_unconfirmed(path: Path, *, published: bool) -> None:
    """⚠️ commit point 之後：parent fsync 失敗 ⛔ 不刪檔案，回報 durability 未確認。"""
    try:
        fsync_dir(path.parent)
    except OSError as exc:
        raise DurabilityUnconfirmed(
            f"run identity {path} "
            + ("已發布" if published else "既有那份有效（no-op）")
            + f"，但 {path.parent} 的 fsync 失敗（{exc}）——durability 未確認。"
            "⛔ 不刪除、不重建：重跑同一條指令會走 no-op 並重新 fsync。",
            published=published,
        ) from exc


def ensure_run_identity(
    path: str | Path,
    *,
    bundle_id: str,
    expected_image_id: str,
    created_at: str,
) -> dict[str, Any]:
    """**依「檔案存不存在」分流**，⛔ 不比對 `created_at`。

    ⚠️ ⛔ 不能用「所有欄位完全相同才 no-op」：`created_at` 每次都會變，那條件**永遠不成立**。

    | identity | 行為 |
    |---|---|
    | 不存在 | 建立並發布（⚠️ `created_at` 只在這裡產生一次） |
    | 已存在且 `bundle_id` 相符 | ⛔ **不重寫**，直接回傳既有內容；⚠️ **仍重新 fsync file ＋ parent**——那是 durability 未確認狀態的**唯一修復路徑** |
    | 已存在但 `bundle_id` 不符 | 拒絕——那代表換了 bundle |

    ⚠️ 「既有的 image 是否仍在本機」⛔ 不在這裡驗：容器內的 Python **做不到
    `docker image inspect`**，那一層屬於 shell（`pin-replay-image.sh`）。
    """
    path = Path(path)
    if not path.exists():
        payload = build_run_identity(
            bundle_id=bundle_id, expected_image_id=expected_image_id, created_at=created_at
        )
        publish_run_identity(path, payload)
        return payload

    existing = load_run_identity(path)
    if existing["bundle_id"] != bundle_id:
        raise ArtifactError(
            f"既有的 run identity 記的是 bundle_id={existing['bundle_id']!r}，"
            f"本次請求的是 {bundle_id!r}——⛔ 不覆蓋，也⛔ 不沿用。"
        )
    # ⚠️ no-op 也要重新 fsync：若上次是「rename 成功、parent fsync 失敗」，這裡才修得回來。
    # ⛔ **兩次 fsync 的失敗處置必須一致**：檔案內容是有效的，⛔ 不得因為失敗在 file 這一步
    # 就退化成一般錯誤（exit 1）——那會讓呼叫端分不出「要重跑」與「輸入壞了」。
    try:
        fsync_file(path)
    except OSError as exc:
        raise DurabilityUnconfirmed(
            f"run identity {path} 既有那份有效（no-op），但檔案 fsync 失敗（{exc}）"
            "——durability 未確認。⛔ 不刪除、不重建：重跑會走 no-op 並重新 fsync。",
            published=False,
        ) from exc
    _fsync_parent_or_unconfirmed(path, published=False)
    return existing
