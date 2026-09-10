"""Canonical 序列化與內容指紋——bundle 身分（`bundle_id`）的唯一來源。

**為什麼要 canonical**：bundle 是「可被獨立複核的證據」，同一份輸入在不同機器、不同
Python 版本重產時必須得到**逐位元相同**的檔案，否則 `content_hash8` 會漂移，
「同輸入 → 同 bundle ID」這個契約就不成立（見 docs/issue.md I-100 計畫書「六、bundle
規格與 canonical 規則」）。

三組規則（⛔ 任何一條都不能只在某一端實作——builder 與 loader 都走這裡）：

* **JSON**：`sort_keys` ／固定 separators ／`ensure_ascii=False` ／`allow_nan=False` ／無結尾換行；
  datetime→ISO-8601 UTC、Decimal→字串、float→最短往返 `repr`。
* **gzip**：`mtime=0` ＋ header 不寫 filename ＋ `compresslevel=9`。gzip header 預設會寫入
  「壓縮當下的時間」與「原始檔名」，兩者都會讓同內容產出不同 bytes。
* **content_hash8**：逐檔完整 SHA-256 →「檔名→hash」mapping → 依檔名 UTF-8 位元組序
  canonical JSON → 取 SHA-256 前 8 個 hex。⚠️ **必須是 mapping 不是把 hash 串起來**：
  後者在 `candles.json.gz` 與 `chip.json.gz` 內容互換時會得到相同的 hash，
  等於沒驗「哪一份資料放在哪個角色」。
"""
from __future__ import annotations

import gzip
import hashlib
import io
import json
import math
from datetime import date, datetime, timezone
from decimal import Decimal
from typing import Any

# canonical JSON 的固定 separators：預設的 ", " / ": " 會多出空白，
# 而空白同樣進 payload bytes → 進 content_hash8。
_SEPARATORS = (",", ":")


class CanonicalError(ValueError):
    """canonical 序列化拒絕輸入（NaN / Infinity / 不支援的型別）。"""


def _default(value: Any) -> Any:
    """把不是 JSON 原生型別的值轉成穩定表示。

    ⛔ 這裡**不做四捨五入也不做格式化**：任何「看起來比較漂亮」的轉換都是資訊損失，
    而 bundle 的用途正是「用什麼輸入算出來的」。
    """
    if isinstance(value, datetime):
        # tz-naive 一律當成 UTC：DB 端存的就是 UTC（見 docs/database-schema.md），
        # 這裡若猜成本地時區會讓同一份資料在不同機器序列化出不同字串。
        if value.tzinfo is None:
            value = value.replace(tzinfo=timezone.utc)
        return value.astimezone(timezone.utc).isoformat()
    if isinstance(value, date):
        return value.isoformat()
    if isinstance(value, Decimal):
        # Decimal→字串，⛔ 不轉 float：那會把 DB 的精確十進位值換成最近的二進位近似。
        return str(value)
    if isinstance(value, (bytes, bytearray)):
        raise CanonicalError("canonical JSON 不接受 bytes——payload 要自己決定編碼方式")
    raise CanonicalError(f"canonical JSON 不支援的型別：{type(value).__name__}")


def _reject_non_finite(obj: Any) -> None:
    """先掃一遍拒絕 NaN / Infinity。

    `json.dumps(allow_nan=False)` 本身就會擋，但它丟的是 `ValueError: Out of range
    float values are not JSON compliant`——看不出是哪一個欄位。bundle 產不出來時
    使用者要能直接知道問題在哪，所以這裡自己走一遍並帶出路徑。
    """
    stack: list[tuple[str, Any]] = [("$", obj)]
    while stack:
        path, node = stack.pop()
        if isinstance(node, dict):
            for key, value in node.items():
                if not isinstance(key, str):
                    raise CanonicalError(f"canonical JSON 的 key 必須是字串：{path} 的 {key!r}")
                stack.append((f"{path}.{key}", value))
        elif isinstance(node, (list, tuple)):
            for idx, value in enumerate(node):
                stack.append((f"{path}[{idx}]", value))
        elif isinstance(node, float):
            if math.isnan(node) or math.isinf(node):
                raise CanonicalError(f"canonical JSON 不接受非有限值：{path} = {node!r}")


def canonical_json_bytes(obj: Any) -> bytes:
    """canonical JSON 的 bytes。**無結尾換行**——多一個 `\\n` 就是不同的 hash。"""
    _reject_non_finite(obj)
    text = json.dumps(
        obj,
        sort_keys=True,
        separators=_SEPARATORS,
        ensure_ascii=False,
        allow_nan=False,
        default=_default,
    )
    return text.encode("utf-8")


def canonical_gzip_bytes(payload: bytes) -> bytes:
    """canonical gzip：`mtime=0`、header 不寫 filename、`compresslevel=9`。

    ⚠️ 用 `GzipFile(filename="")` 而不是 `gzip.compress()`：後者在 3.11 才有
    `mtime` 參數，而且**永遠不寫 filename** 這件事沒有明確保證。這裡把兩個欄位都寫死。
    """
    buf = io.BytesIO()
    with gzip.GzipFile(filename="", mode="wb", fileobj=buf, compresslevel=9, mtime=0) as fh:
        fh.write(payload)
    return buf.getvalue()


def gunzip_bytes(blob: bytes) -> bytes:
    return gzip.decompress(blob)


def sha256_hex(payload: bytes) -> str:
    return hashlib.sha256(payload).hexdigest()


def sha256_file(path) -> str:
    """檔案內容的完整 SHA-256（十六進位小寫）。

    分塊讀：bundle 內的 `model.joblib` 可能有數 MB，而這個專案的 host 只有 2GiB
    （見 docs/development-workflow.md），不必要的整檔載入能省則省。
    """
    digest = hashlib.sha256()
    with open(path, "rb") as fh:
        for chunk in iter(lambda: fh.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def content_hash8(file_hashes: dict[str, str]) -> str:
    """由「bundle 內相對檔名 → 完整 SHA-256」的 mapping 算出 8 hex 的內容指紋。

    ⚠️ **8 hex ＝ 32 bit，本來就不是防碰撞用的**：真正的完整性由 loader 的逐檔完整
    SHA-256 保證（見 `bundle.load_bundle`）。這個短碼只是讓 `bundle_id` 可以當目錄名。
    """
    if not file_hashes:
        raise CanonicalError("content_hash8 需要至少一個檔案")
    for name, digest in file_hashes.items():
        if not isinstance(name, str) or not name:
            raise CanonicalError(f"檔名必須是非空字串：{name!r}")
        if not isinstance(digest, str) or len(digest) != 64 or digest != digest.lower():
            raise CanonicalError(f"{name} 的 SHA-256 必須是 64 字元十六進位小寫：{digest!r}")
    # sort_keys 依 str 的比較順序排——Python 的 str 比較是 code point 序，
    # 而 UTF-8 的位元組序與 code point 序在同一個方向上一致，兩者結果相同。
    return sha256_hex(canonical_json_bytes(dict(file_hashes)))[:8]


def symbols_hash8(symbols) -> str:
    """去重、UTF-8 位元組序排序、`\\n` 連接後 SHA-256 前 8 碼。"""
    unique = {str(s).strip() for s in symbols if str(s).strip()}
    if not unique:
        raise CanonicalError("symbols_hash8 需要至少一個 symbol")
    joined = "\n".join(sorted(unique, key=lambda s: s.encode("utf-8")))
    return sha256_hex(joined.encode("utf-8"))[:8]
