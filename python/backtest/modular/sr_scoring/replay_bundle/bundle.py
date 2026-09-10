"""凍結輸入 bundle 的規格、產生與載入。

**bundle 回答的問題是「用什麼輸入算出來的」**——candles、chip、governance、交易日曆、
模型與設定的**實際內容**，不只是它們的指紋。manifest 回答「要觀察／比對哪些列」的那一半
在 `cohort_manifest.json`（Stage 1 產出），兩者不能互相取代：
`--as-of` 固定的是**列範圍**，指紋能告訴你內容變了，但當 DB 歷史被修正、還原係數更新或
model bundle 被替換時，**能做的只有中止**——沒有 bundle 就沒有任何機制重新執行原來那份輸入。

規格見 docs/issue.md I-100 計畫書「六、bundle 規格與 canonical 規則」。
"""
from __future__ import annotations

import json
import re
import secrets
from dataclasses import dataclass
from pathlib import Path
from typing import Any

from .calendar import CalendarError, validate_calendar_payload
from .canonical import (
    CanonicalError,
    canonical_gzip_bytes,
    canonical_json_bytes,
    content_hash8,
    gunzip_bytes,
    sha256_file,
    sha256_hex,
    symbols_hash8,
)
from .publish import (
    DurabilityUnconfirmed,
    remove_tree,
    fsync_dir,
    fsync_file,
    probe_no_clobber,
    rename_noreplace,
)

MANIFEST_SCHEMA_VERSION = 1
MANIFEST_NAME = "manifest.json"
MANIFEST_SHA_NAME = "manifest.sha256"

# ⚠️ payload 是**封閉清單**：bundle 目錄裡多一個檔或少一個檔都要中止。
# 多出來的檔案不會進 `files` mapping → 不進 content_hash8 → 等於一份沒有被身分涵蓋的輸入。
PAYLOAD_FILES = (
    "candles.json.gz",
    "chip.json.gz",
    "governance.json.gz",
    "replay_config.json",
    "trading_calendar.json",
    "model.joblib",
)

_BUNDLE_ID_RE = re.compile(r"^b[0-9]+_[0-9]{8}_[0-9a-z]+_[0-9a-f]{8}_[0-9a-f]{8}$")

# `replay_config.json` 的封閉欄位集合。⛔ 多一個未知欄位就中止——它是 payload，
# 多出來的欄位會進 content_hash8，但沒有任何一端會去讀它。
_REPLAY_CONFIG_FIELDS = {
    "as_of", "timeframe", "limit", "replay_scope",
    "dataset_config", "builder_config", "dataset_from", "dataset_to",
}
# ⚠️ 這四個同時出現在 manifest 與 replay_config 裡，**兩份必須一致**：
# Stage 1／2 讀 replay_config 決定怎麼算，卻用 manifest 的欄位對外宣稱身分。
_REPLAY_CONFIG_MIRRORED = ("as_of", "timeframe", "limit", "replay_scope")

# manifest 的 top-level 欄位是封閉集合。⛔ 多一個未知欄位就中止：
# manifest 不進 content_hash8，所以「多出來的欄位」不會被身分抓到，只能靠這裡擋。
_MANIFEST_FIELDS = {
    "schema_version",
    "bundle_id",
    "as_of",
    "timeframe",
    "symbols",
    "limit",
    "replay_scope",
    "report_max_rows",
    "captured_at",
    "files",
    "content_hash8",
    "symbols_hash8",
    "readiness",
    "calendar",
    "provenance",
}


class BundleError(ValueError):
    """bundle 的產生或載入被拒絕。"""


@dataclass(frozen=True)
class LoadedBundle:
    """已通過完整驗證的 bundle。⛔ 未經 `load_bundle()` 的資料一律不得使用。"""

    path: Path
    bundle_id: str
    manifest: dict[str, Any]
    candles: dict[str, list[dict]]
    chip: dict[str, list[dict]]
    governance: dict[str, list[dict]]
    replay_config: dict[str, Any]
    trading_calendar: dict[str, Any]
    model_path: Path

    @property
    def file_hashes(self) -> dict[str, str]:
        return dict(self.manifest["files"])


@dataclass(frozen=True)
class PublishOutcome:
    """發布結果。`published=False` 代表既有那份與本次 payload 完全相同（no-op）。"""

    bundle_id: str
    path: Path
    published: bool
    probe: dict[str, str]


# ── 排序：三份 payload 的 total order ────────────────────────────────────────
#
# ⚠️ **一律以「整列 canonical JSON bytes」當最終 tie-breaker**：排序鍵相同的兩列若順序不定，
# 同一份輸入就會產出不同的 payload bytes → 不同的 bundle_id。
# ⛔ 不加 `id` 當 tie-breaker——那會動到 Go 注入 Python 的 row 形狀。

def _row_bytes(row: dict) -> bytes:
    return canonical_json_bytes(row)


def _sorted_rows(rows: list[dict], key_fields: tuple[str, ...]) -> list[dict]:
    def sort_key(row: dict):
        return tuple(_key_token(row.get(field)) for field in key_fields) + (_row_bytes(row),)

    return sorted(rows, key=sort_key)


def _key_token(value: Any) -> tuple[int, str]:
    """把排序鍵正規化成 (型別序, 字串)。

    ⚠️ 直接排原始值會在同一欄混入 `None` 與字串時丟 TypeError；而 payload 的欄位
    （例如 governance 的 `model_version`）確實可能是 NULL。把 `None` 排在最前面，
    其餘一律轉字串比較——**排序只需要決定性，不需要語意上的大小**。
    """
    if value is None:
        return (0, "")
    return (1, str(value))


def build_candles_payload(candles_by_symbol: dict[str, list[dict]]) -> list[dict]:
    rows: list[dict] = []
    for symbol, items in candles_by_symbol.items():
        for row in items:
            item = dict(row)
            item["symbol"] = str(symbol)
            rows.append(item)
    return _sorted_rows(rows, ("symbol", "timestamp"))


def build_chip_payload(chip_by_symbol: dict[str, list[dict]]) -> list[dict]:
    rows: list[dict] = []
    for symbol, items in chip_by_symbol.items():
        for row in items:
            item = dict(row)
            item.setdefault("symbol", str(symbol))
            rows.append(item)
    return _sorted_rows(rows, ("symbol", "trade_date"))


def build_governance_payload(governance_by_symbol: dict[str, list[dict]]) -> list[dict]:
    rows: list[dict] = []
    for symbol, items in governance_by_symbol.items():
        for row in items:
            item = dict(row)
            item.setdefault("symbol", str(symbol))
            rows.append(item)
    return _sorted_rows(
        rows, ("symbol", "timeframe", "as_of", "created_at", "model_version", "model_config_hash")
    )


def _group_by_symbol(rows: list[dict]) -> dict[str, list[dict]]:
    """payload（扁平列）→ 呼叫端要的 `{symbol: [row, ...]}`，維持 payload 的順序。"""
    out: dict[str, list[dict]] = {}
    for row in rows:
        out.setdefault(str(row.get("symbol")), []).append(row)
    return out


# ── bundle id ───────────────────────────────────────────────────────────────

def compute_bundle_id(
    *,
    schema_version: int,
    as_of: str,
    timeframe: str,
    symbols,
    file_hashes: dict[str, str],
) -> str:
    """`b<schema>_<as_of:YYYYMMDD>_<timeframe>_<symbols_hash8>_<content_hash8>`。

    ⛔ **不放 `run_id`／`pipeline_version`／`captured_at`**：前兩者是 Stage 1／2 的使用者
    參數（同一份 bundle 會被不同 run 重複消費），後者一變 ID 就變，與決定性直接衝突。
    """
    compact = str(as_of).replace("-", "")
    if not re.fullmatch(r"[0-9]{8}", compact):
        raise BundleError(f"as_of 必須是 YYYY-MM-DD：{as_of!r}")
    tf = str(timeframe)
    if not re.fullmatch(r"[0-9a-z]+", tf):
        raise BundleError(f"timeframe 只能是小寫英數（bundle_id 要當目錄名）：{timeframe!r}")
    bundle_id = (
        f"b{int(schema_version)}_{compact}_{tf}_"
        f"{symbols_hash8(symbols)}_{content_hash8(file_hashes)}"
    )
    if not _BUNDLE_ID_RE.match(bundle_id):
        raise BundleError(f"算出來的 bundle_id 不符合格式：{bundle_id!r}")
    return bundle_id


# ── 產生 ────────────────────────────────────────────────────────────────────

def render_payloads(
    *,
    candles_by_symbol: dict[str, list[dict]],
    chip_by_symbol: dict[str, list[dict]],
    governance_by_symbol: dict[str, list[dict]],
    replay_config: dict[str, Any],
    trading_calendar: dict[str, Any],
    model_bytes: bytes,
) -> dict[str, bytes]:
    """把六份 payload 序列化成**最終落地的 bytes**。

    ⚠️ hash 的對象就是這裡回傳的 bytes（`.gz` 是**壓縮後**的 bytes，不是解壓內容）。
    """
    return {
        "candles.json.gz": canonical_gzip_bytes(
            canonical_json_bytes(build_candles_payload(candles_by_symbol))
        ),
        "chip.json.gz": canonical_gzip_bytes(
            canonical_json_bytes(build_chip_payload(chip_by_symbol))
        ),
        "governance.json.gz": canonical_gzip_bytes(
            canonical_json_bytes(build_governance_payload(governance_by_symbol))
        ),
        "replay_config.json": canonical_json_bytes(replay_config),
        "trading_calendar.json": canonical_json_bytes(trading_calendar),
        "model.joblib": model_bytes,
    }


def build_manifest(
    *,
    payloads: dict[str, bytes],
    as_of: str,
    timeframe: str,
    symbols: list[str],
    limit: int,
    replay_scope: str,
    report_max_rows: int | None,
    captured_at: str,
    readiness: dict[str, Any],
    calendar: dict[str, Any],
    provenance: dict[str, Any],
) -> tuple[str, dict[str, Any]]:
    missing = set(PAYLOAD_FILES) - set(payloads)
    extra = set(payloads) - set(PAYLOAD_FILES)
    if missing or extra:
        raise BundleError(f"payload 檔案清單不符：缺 {sorted(missing)}，多 {sorted(extra)}")

    file_hashes = {name: sha256_hex(payloads[name]) for name in sorted(payloads)}
    bundle_id = compute_bundle_id(
        schema_version=MANIFEST_SCHEMA_VERSION,
        as_of=as_of,
        timeframe=timeframe,
        symbols=symbols,
        file_hashes=file_hashes,
    )
    manifest = {
        "schema_version": MANIFEST_SCHEMA_VERSION,
        "bundle_id": bundle_id,
        "as_of": as_of,
        "timeframe": timeframe,
        "symbols": sorted(set(symbols)),
        "limit": int(limit),
        "replay_scope": replay_scope,
        "report_max_rows": report_max_rows,
        # ⚠️ 唯一允許在「同輸入重產」時不同的欄位（連同 calendar provenance 的時間欄位）。
        "captured_at": captured_at,
        "files": file_hashes,
        "content_hash8": content_hash8(file_hashes),
        "symbols_hash8": symbols_hash8(symbols),
        "readiness": readiness,
        "calendar": calendar,
        "provenance": provenance,
    }
    unknown = set(manifest) - _MANIFEST_FIELDS
    if unknown:
        raise BundleError(f"manifest 出現未知欄位：{sorted(unknown)}")
    return bundle_id, manifest


def write_staging_bundle(staging_dir: Path, payloads: dict[str, bytes], manifest: dict[str, Any]) -> Path:
    """把 bundle 寫進 staging。

    ⚠️ **子目錄名必須正好是 `bundle_id`**——loader 的三方相等契約要驗目錄 basename，
    而發布是把這個子目錄整包 rename 過去。
    """
    bundle_dir = staging_dir / manifest["bundle_id"]
    bundle_dir.mkdir(parents=True)
    for name in sorted(payloads):
        path = bundle_dir / name
        path.write_bytes(payloads[name])
        fsync_file(path)
    manifest_bytes = canonical_json_bytes(manifest)
    (bundle_dir / MANIFEST_NAME).write_bytes(manifest_bytes)
    fsync_file(bundle_dir / MANIFEST_NAME)
    (bundle_dir / MANIFEST_SHA_NAME).write_text(sha256_hex(manifest_bytes) + "\n", encoding="utf-8")
    fsync_file(bundle_dir / MANIFEST_SHA_NAME)
    fsync_dir(bundle_dir)
    fsync_dir(staging_dir)
    return bundle_dir


def emit_bundle(
    baselines_dir: Path,
    payloads: dict[str, bytes],
    manifest: dict[str, Any],
    *,
    log=None,
) -> PublishOutcome:
    """七-B 的整包原子發布。流程與失敗語意見 `publish` 模組的說明。

    ⛔ **任何情況都不碰既有正式目錄**——包含失敗路徑與 `finally`。
    """
    baselines_dir = Path(baselines_dir)
    baselines_dir.mkdir(parents=True, exist_ok=True)
    bundle_id = manifest["bundle_id"]
    final_dir = baselines_dir / bundle_id

    probe = probe_no_clobber(baselines_dir)
    if log is not None:
        log(f"no-clobber probe: {probe}")

    staging_dir = baselines_dir / f".staging-{secrets.token_hex(8)}"
    published = False
    try:
        staging_dir.mkdir()
        staged = write_staging_bundle(staging_dir, payloads, manifest)
        # ③ 發布前先用**正式 loader** 完整驗證 staging——⛔ 不信任「剛剛才寫出去」。
        load_bundle(staged)
        try:
            rename_noreplace(staged, final_dir)
            published = True
        except FileExistsError:
            # ⑤ 本次是競爭的輸家，或本來就有同 ID bundle。
            _assert_existing_matches(final_dir, manifest)
            published = False
    finally:
        # ⑦ 只刪本次的 .staging-<random>。⛔ 任何情況都不碰正式路徑。
        remove_tree(staging_dir)

    # ⑥ 兩條成功路徑（rename 成功、no-op）都要在回傳前 fsync <baselines>。
    try:
        fsync_dir(baselines_dir)
    except OSError as exc:
        raise DurabilityUnconfirmed(
            f"bundle {bundle_id} "
            + ("已發布" if published else "既有那份有效（no-op）")
            + f"且通過完整驗證，但 {baselines_dir} 的 fsync 失敗（{exc}）——durability 未確認。"
            "⛔ 不刪除、不重來：重跑同一條指令會走 no-op 並重新 fsync。",
            published=published,
        ) from exc

    return PublishOutcome(bundle_id=bundle_id, path=final_dir, published=published, probe=probe)


def _assert_existing_matches(final_dir: Path, manifest: dict[str, Any]) -> None:
    """既有 bundle 必須先通過正式 loader，再比「六份 payload 的完整 SHA-256 mapping」。

    ⛔ **不比 `manifest.json`／`manifest.sha256` 的 bytes**：那兩份本來就允許 volatile
    欄位不同（`captured_at`、calendar 的時間欄位），拿它們比會讓**同一份 payload 的第二次
    產生被判成不同而中止**，與「payload 相同 → no-op」矛盾。
    """
    try:
        existing = load_bundle(final_dir)
    except BundleError as exc:
        raise BundleError(
            f"正式路徑已存在同 ID bundle（{final_dir}），但它沒通過完整驗證：{exc}\n"
            "⛔ 不覆蓋、⛔ 不刪除——請人工處理後再重跑。"
        ) from exc
    if existing.file_hashes != dict(manifest["files"]):
        raise BundleError(
            f"正式路徑已存在同 ID bundle（{final_dir}），但 payload 的 SHA-256 mapping 不同。\n"
            f"既有：{existing.file_hashes}\n本次：{manifest['files']}\n"
            "⛔ 不覆蓋——同 ID 不同內容代表 8-hex 短碼碰撞或既有證據被動過，兩種都要人工處理。"
        )


# ── 載入 ────────────────────────────────────────────────────────────────────

def load_bundle(bundle_dir: Path) -> LoadedBundle:
    """完整驗證後載入。任何一項不符即 `BundleError`。

    驗的三件事：①`manifest.sha256` 與 manifest bytes 相符；②**逐檔完整 SHA-256** 與
    manifest 的 `files` 相符、且目錄內容恰好是那份清單；③**三方相等契約**——
    目錄 basename == `manifest.bundle_id` == 由 `(schema_version, as_of, timeframe, symbols,
    payload 逐檔 hash)` 重新計算的值。

    ⚠️ 少了 ③，**被改名的 bundle、或 manifest 身分欄位（`as_of`／`timeframe`／`symbols`）
    被竄改的 bundle 照樣載得進來**——逐檔 hash 只證明「payload 沒被動過」，
    證明不了「這份 payload 就是這個身分宣稱的那一份」，而那些欄位會被 Stage 1／2
    當成事實寫進 artifact。
    """
    bundle_dir = Path(bundle_dir)
    if not bundle_dir.is_dir():
        raise BundleError(f"bundle 目錄不存在：{bundle_dir}")

    manifest_path = bundle_dir / MANIFEST_NAME
    sha_path = bundle_dir / MANIFEST_SHA_NAME
    if not manifest_path.is_file() or not sha_path.is_file():
        raise BundleError(f"bundle 缺少 {MANIFEST_NAME} 或 {MANIFEST_SHA_NAME}：{bundle_dir}")

    manifest_bytes = manifest_path.read_bytes()
    recorded = sha_path.read_text(encoding="utf-8").strip()
    actual = sha256_hex(manifest_bytes)
    if recorded != actual:
        raise BundleError(f"{MANIFEST_NAME} 的 SHA-256 不符：記錄 {recorded}，實際 {actual}")

    try:
        manifest = json.loads(manifest_bytes.decode("utf-8"))
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise BundleError(f"{MANIFEST_NAME} 不是合法 JSON：{exc}") from exc
    if not isinstance(manifest, dict):
        raise BundleError(f"{MANIFEST_NAME} 必須是 object")

    version = manifest.get("schema_version")
    if type(version) is not int or version != MANIFEST_SCHEMA_VERSION:
        raise BundleError(
            f"未知的 manifest schema_version={version!r}（本實作只支援 {MANIFEST_SCHEMA_VERSION}）"
        )
    missing = _MANIFEST_FIELDS - set(manifest)
    unknown = set(manifest) - _MANIFEST_FIELDS
    if missing or unknown:
        raise BundleError(f"{MANIFEST_NAME} 欄位不符：缺 {sorted(missing)}，多 {sorted(unknown)}")

    file_hashes = manifest["files"]
    if not isinstance(file_hashes, dict):
        raise BundleError("manifest.files 必須是 object")
    if set(file_hashes) != set(PAYLOAD_FILES):
        raise BundleError(
            f"manifest.files 的檔案清單不符：{sorted(file_hashes)} != {sorted(PAYLOAD_FILES)}"
        )

    on_disk = {p.name for p in bundle_dir.iterdir()}
    expected_on_disk = set(PAYLOAD_FILES) | {MANIFEST_NAME, MANIFEST_SHA_NAME}
    if on_disk != expected_on_disk:
        raise BundleError(
            f"bundle 目錄內容不符：多 {sorted(on_disk - expected_on_disk)}，"
            f"缺 {sorted(expected_on_disk - on_disk)}"
        )

    for name in sorted(PAYLOAD_FILES):
        actual_hash = sha256_file(bundle_dir / name)
        if actual_hash != file_hashes[name]:
            raise BundleError(
                f"{name} 的 SHA-256 不符：manifest 記 {file_hashes[name]}，實際 {actual_hash}"
            )

    try:
        recomputed = compute_bundle_id(
            schema_version=version,
            as_of=manifest["as_of"],
            timeframe=manifest["timeframe"],
            symbols=manifest["symbols"],
            file_hashes=file_hashes,
        )
    except (BundleError, CanonicalError) as exc:
        raise BundleError(f"無法由 manifest 重算 bundle_id：{exc}") from exc
    if not (bundle_dir.name == manifest["bundle_id"] == recomputed):
        raise BundleError(
            "三方相等契約不成立："
            f"目錄 basename={bundle_dir.name!r}、manifest.bundle_id={manifest['bundle_id']!r}、"
            f"重算值={recomputed!r}"
        )

    # manifest 自己記的兩個指紋也要與重算值相符——它們是人在讀 manifest 時會直接引用的
    # 欄位，只驗 bundle_id 的話，這兩格被改掉不會有任何東西報錯。
    for field, expected in (
        ("content_hash8", content_hash8(file_hashes)),
        ("symbols_hash8", symbols_hash8(manifest["symbols"])),
    ):
        if manifest[field] != expected:
            raise BundleError(
                f"manifest.{field} 是 {manifest[field]!r}，重算值是 {expected!r}"
            )

    candles = _load_gz_rows(bundle_dir / "candles.json.gz")
    chip = _load_gz_rows(bundle_dir / "chip.json.gz")
    governance = _load_gz_rows(bundle_dir / "governance.json.gz")
    replay_config = _load_json_object(bundle_dir / "replay_config.json")
    _validate_replay_config(replay_config, manifest)
    trading_calendar = _load_calendar_payload(bundle_dir / "trading_calendar.json")

    return LoadedBundle(
        path=bundle_dir,
        bundle_id=manifest["bundle_id"],
        manifest=manifest,
        candles=_group_by_symbol(candles),
        chip=_group_by_symbol(chip),
        governance=_group_by_symbol(governance),
        replay_config=replay_config,
        trading_calendar=trading_calendar,
        model_path=bundle_dir / "model.joblib",
    )


def _validate_replay_config(replay_config: dict[str, Any], manifest: dict[str, Any]) -> None:
    unknown = set(replay_config) - _REPLAY_CONFIG_FIELDS
    missing = _REPLAY_CONFIG_FIELDS - set(replay_config)
    if unknown or missing:
        raise BundleError(
            f"replay_config.json 欄位不符：缺 {sorted(missing)}，多 {sorted(unknown)}"
        )
    for field in _REPLAY_CONFIG_MIRRORED:
        if replay_config[field] != manifest[field]:
            raise BundleError(
                f"replay_config.{field}={replay_config[field]!r} 與 "
                f"manifest.{field}={manifest[field]!r} 不一致"
            )
    for field in ("dataset_config", "builder_config"):
        if not isinstance(replay_config[field], dict):
            raise BundleError(f"replay_config.{field} 必須是 object")


def _load_calendar_payload(path: Path) -> dict[str, Any]:
    """⚠️ **loader 這一端也要驗日曆**——`row_type` 與 `is_trading_day` 的映射、完整年度
    不變條件、封閉欄位，contract 明訂「builder 與 loader 兩端都要驗」。
    只驗檔案 hash 只證明「進來時是什麼就是什麼」，證明不了那份內容本身合法。

    另外重做一次 canonical 序列化並逐位元比對：bundle 內的每一份 payload 都是
    canonical 形式，不是的話身分計算就跟著漂。
    """
    raw = path.read_bytes()
    try:
        payload = json.loads(raw.decode("utf-8"))
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise BundleError(f"{path.name} 不是合法 JSON：{exc}") from exc
    try:
        validate_calendar_payload(payload)
    except CalendarError as exc:
        raise BundleError(f"{path.name} 沒通過交易日曆 schema 驗證：{exc}") from exc
    if canonical_json_bytes(payload) != raw:
        raise BundleError(f"{path.name} 不是 canonical 形式（縮排／鍵序／多餘空白都算）")
    return payload


def _load_gz_rows(path: Path) -> list[dict]:
    try:
        data = json.loads(gunzip_bytes(path.read_bytes()).decode("utf-8"))
    except Exception as exc:  # noqa: BLE001 - 壞檔的成因很多，統一轉成 BundleError。
        raise BundleError(f"{path.name} 無法解壓或解析：{exc}") from exc
    if not isinstance(data, list) or any(not isinstance(row, dict) for row in data):
        raise BundleError(f"{path.name} 必須是 object 的陣列")
    return data


def _load_json_object(path: Path) -> dict[str, Any]:
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
    except Exception as exc:  # noqa: BLE001
        raise BundleError(f"{path.name} 無法解析：{exc}") from exc
    if not isinstance(data, dict):
        raise BundleError(f"{path.name} 必須是 object")
    return data
