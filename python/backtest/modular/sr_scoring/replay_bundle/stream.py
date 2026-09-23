"""串流讀 canonical artifact：一趟讀取完成 hash、canonical 檢查與逐列驗證（I-074 Stage 2）。

⚠️ **為什麼需要它**：Stage 2 的 finalizer／recovery／preflight 與環境等價比對都要讀**全量**
artifact（13,417 列；整份載入實測約 +365～449 MiB），而 mem-guard 常態只給得出 531m。
Stage 1 的 comparator 就是整份載入兩份才撞到約 730 MiB（issue.md I-117）。
規格見 issue.md I-074 ③ evidence contract「五之一：記憶體模型」。

**與 `load_canonical_evidence_artifact()` 同強度**（⛔ 驗證強度不得下降）：

| 檢查 | 整份載入版 | 本模組 |
|---|---|---|
| `.json`：raw bytes 就是 canonical bytes | 整份重新編碼比 SHA | 逐欄／逐列重新編碼，**增量** SHA 與 raw SHA 比 |
| `.json.gz`：round-trip 逐位元相同 | 整份重新壓縮比 bytes | 解壓的 payload **同時**餵進同參數的 gzip，比 SHA（⚠️ 分塊寫入與一次寫入的輸出逐位元相同——2026-09-23 以 Stage 1 D+1 實測） |
| `schema_version`／`kind` | ✅ | ✅ |

⚠️ **hash、驗證與回傳值全部來自同一次讀取**——⛔ 不得「先算 hash、再讀一次驗證」
（兩次讀取之間檔案可能被換掉）。要封存的副本也在**同一趟**寫出（`copy_to`）。

⚠️ **`on_row` 收到的列在函式正常回傳之前都只是暫定值**：canonical 與 hash 的比對要讀完
整份才知道結果。呼叫端只能累積，⛔ 不得在回傳前據以做任何不可逆的事。

⛔ **本模組必須 dependency-light**（只用標準庫與同 package）。
"""
from __future__ import annotations

import codecs
import gzip
import hashlib
import json
import os
import zlib
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Callable

from .artifacts import (
    AFTER_KIND,
    ARTIFACT_SCHEMA_VERSION,
    ArtifactError,
    StreamRowValidator,
    validate_after_envelope,
)
from .canonical import CanonicalError, canonical_json_bytes

# 每次讀取的 raw bytes。⚠️ 是模組層級變數，測試會把它調到極小，逼出「一列跨兩個 chunk」的邊界。
READ_CHUNK = 1 << 20
# 已消費的文字累積到這個長度就丟掉，避免 buffer 隨檔案長度成長。
_COMPACT_AT = 1 << 20
_WHITESPACE = " \t\n\r"


@dataclass(frozen=True)
class StreamLoad:
    """`stream_canonical_artifact()` 的回傳。⚠️ 全部來自**同一次讀取**。"""

    top: dict[str, Any]     # 頂層欄位，⛔ 不含被串流的陣列欄位
    artifact_sha256: str    # 解壓後 bytes 的 SHA（`.json` 時即 raw bytes）
    stored_sha256: str      # 實際落地檔案 raw bytes 的 SHA
    stored_bytes: int
    row_count: int
    # `copy_to` 有給時：寫出的 canonical gzip 副本的 SHA 與長度
    copy_sha256: str | None = None
    copy_bytes: int | None = None


class _HashSink:
    """給 `GzipFile` 當 fileobj：只算 hash，⛔ 不留 bytes。"""

    def __init__(self) -> None:
        self.digest = hashlib.sha256()
        self.size = 0

    def write(self, data: bytes) -> int:
        self.digest.update(data)
        self.size += len(data)
        return len(data)

    def flush(self) -> None:
        pass


class _HashingFile:
    """寫進真正的檔案，同時算 hash 與長度。"""

    def __init__(self, fh) -> None:
        self._fh = fh
        self.digest = hashlib.sha256()
        self.size = 0

    def write(self, data: bytes) -> int:
        self._fh.write(data)
        self.digest.update(data)
        self.size += len(data)
        return len(data)

    def flush(self) -> None:
        self._fh.flush()


def _canonical_gzip_writer(fileobj) -> gzip.GzipFile:
    """⚠️ 參數必須與 `canonical.canonical_gzip_bytes()` **完全相同**：`mtime=0`、
    header 不寫 filename、`compresslevel=9`。"""
    return gzip.GzipFile(filename="", mode="wb", fileobj=fileobj, compresslevel=9, mtime=0)


class _Source:
    """raw bytes →（gzip 解壓）→ UTF-8 文字，途中維護全部 hash 與副本。"""

    def __init__(self, fh, *, gz: bool, name: str, copy_fh=None) -> None:
        self._fh = fh
        self._gz = gz
        self._name = name
        self.stored = hashlib.sha256()
        self.stored_bytes = 0
        self.payload = hashlib.sha256()
        self._text = codecs.getincrementaldecoder("utf-8")()
        self._inflate = zlib.decompressobj(31) if gz else None
        self._regz_sink = _HashSink() if gz else None
        self._regz = _canonical_gzip_writer(self._regz_sink) if gz else None
        self._copy_file = _HashingFile(copy_fh) if copy_fh is not None else None
        self._copy = _canonical_gzip_writer(self._copy_file) if copy_fh is not None else None
        self.eof = False

    def _inflate_chunk(self, raw: bytes) -> bytes:
        if self._inflate.eof:
            raise ArtifactError(f"{self._name}：gzip 結尾之後還有多餘資料——⛔ 不是單一 canonical member")
        try:
            out = self._inflate.decompress(raw)
        except zlib.error as exc:
            raise ArtifactError(f"{self._name} 不是合法 gzip：{exc}") from exc
        if self._inflate.eof and self._inflate.unused_data:
            raise ArtifactError(f"{self._name}：gzip 結尾之後還有多餘資料——⛔ 不是單一 canonical member")
        return out

    def _finish_inflate(self) -> bytes:
        try:
            out = self._inflate.flush()
        except zlib.error as exc:
            raise ArtifactError(f"{self._name} 不是合法 gzip：{exc}") from exc
        if not self._inflate.eof:
            raise ArtifactError(f"{self._name} 的 gzip 不完整（被截斷）")
        return out

    def read_text(self) -> str:
        """下一段文字。讀到檔尾時設 `eof` 並回傳剩下的文字（可能是空字串）。"""
        if self.eof:
            return ""
        while True:
            raw = self._fh.read(READ_CHUNK)
            if raw:
                self.stored.update(raw)
                self.stored_bytes += len(raw)
                payload = self._inflate_chunk(raw) if self._gz else raw
            else:
                payload = self._finish_inflate() if self._gz else b""
            if payload:
                self.payload.update(payload)
                if self._regz is not None:
                    self._regz.write(payload)
                if self._copy is not None:
                    self._copy.write(payload)
            try:
                text = self._text.decode(payload, final=not raw)
            except UnicodeDecodeError as exc:
                raise ArtifactError(f"{self._name} 不是合法 UTF-8：{exc}") from exc
            if not raw:
                self.eof = True
                return text
            if text:
                return text

    def finish(self) -> tuple[str, str, int, str | None, int | None]:
        """回傳 `(payload_sha, stored_sha, stored_bytes, copy_sha, copy_bytes)`。"""
        if self._regz is not None:
            self._regz.close()
            if self._regz_sink.digest.hexdigest() != self.stored.hexdigest():
                raise ArtifactError(
                    f"{self._name} ⛔ 不是 canonical gzip（round-trip 不是逐位元相同）——"
                    "compression level／header 欄位不同都會落在這裡"
                )
        copy_sha = copy_bytes = None
        if self._copy is not None:
            self._copy.close()
            self._copy_file.flush()
            copy_sha, copy_bytes = self._copy_file.digest.hexdigest(), self._copy_file.size
        return (self.payload.hexdigest(), self.stored.hexdigest(), self.stored_bytes,
                copy_sha, copy_bytes)


class _Parser:
    """只懂 canonical artifact 形狀的最小串流解析器：頂層 object ＋ 其中一個欄位是陣列。

    ⚠️ 單一值（含一整列）交給標準庫的 `JSONDecoder.raw_decode()`——⛔ 不自己解析 JSON 語法。
    值可能被 chunk 切斷，所以「解不出來」或「剛好解到 buffer 尾端」（數字可能還沒讀完）
    都要再讀一段重試。
    """

    def __init__(self, source: _Source, name: str) -> None:
        self._src = source
        self._name = name
        self._buf = ""
        self._pos = 0
        self._decoder = json.JSONDecoder()

    def _fill(self) -> bool:
        if self._src.eof:
            return False
        if self._pos > _COMPACT_AT:
            self._buf = self._buf[self._pos:]
            self._pos = 0
        self._buf += self._src.read_text()
        return True

    def _peek(self) -> str:
        while self._pos >= len(self._buf):
            if not self._fill():
                return ""
        return self._buf[self._pos]

    def _skip_ws(self) -> None:
        while True:
            ch = self._peek()
            if ch and ch in _WHITESPACE:
                self._pos += 1
            else:
                return

    def _expect(self, ch: str) -> None:
        self._skip_ws()
        actual = self._peek()
        if actual != ch:
            raise ArtifactError(f"{self._name}：預期 {ch!r}，實際 {actual!r}")
        self._pos += 1

    def _value(self) -> Any:
        self._skip_ws()
        while True:
            try:
                obj, end = self._decoder.raw_decode(self._buf, self._pos)
            except json.JSONDecodeError as exc:
                if self._fill():
                    continue
                raise ArtifactError(f"{self._name} 不是合法 JSON：{exc}") from exc
            # ⚠️ 剛好解到 buffer 尾端：數字可能只讀到一半（`12` 其實是 `123`），要多讀再重試。
            if end >= len(self._buf) and self._fill():
                continue
            self._pos = end
            return obj

    @staticmethod
    def _encode(value: Any) -> bytes:
        try:
            return canonical_json_bytes(value)
        except CanonicalError as exc:
            raise ArtifactError(f"內容無法 canonical 編碼：{exc}") from exc

    def parse(self, *, stream_field: str, on_row: Callable[[Any, bytes], None],
              kind: str, canon) -> tuple[dict[str, Any], int]:
        top: dict[str, Any] = {}
        keys: list[str] = []
        row_count = None
        self._expect("{")
        canon.update(b"{")
        self._skip_ws()
        if self._peek() == "}":
            self._pos += 1
        else:
            while True:
                key = self._value()
                if not isinstance(key, str):
                    raise ArtifactError(f"{self._name}：頂層的 key 必須是字串：{key!r}")
                # ⚠️ canonical 要求 key 排序且不重複——⛔ 只靠重新編碼抓不到，因為這裡是
                # 依檔案裡的順序重新組回去的。
                if keys and key <= keys[-1]:
                    raise ArtifactError(
                        f"{self._name}：頂層 key 未排序或重複（{keys[-1]!r} 之後是 {key!r}）"
                        "——⛔ 不是 canonical JSON"
                    )
                canon.update((b"," if keys else b"") + self._encode(key) + b":")
                keys.append(key)
                self._expect(":")
                if key == stream_field:
                    row_count = self._array(on_row, canon)
                else:
                    value = self._value()
                    if key == "kind" and value != kind:
                        # ⚠️ 提早擋：`kind` 排在 `rows` 前面，⛔ 不必讀完整份才發現讀錯檔。
                        raise ArtifactError(f"{self._name} 的 kind={value!r}，預期 {kind!r}")
                    top[key] = value
                    canon.update(self._encode(value))
                self._skip_ws()
                ch = self._peek()
                if ch == ",":
                    self._pos += 1
                    continue
                if ch == "}":
                    self._pos += 1
                    break
                raise ArtifactError(f"{self._name}：頂層 object 之後預期 ',' 或 '}}'，實際 {ch!r}")
        canon.update(b"}")
        # ⚠️ 結尾⛔ 不得有任何多餘內容（含空白與換行——canonical 沒有結尾換行）。
        if self._peek():
            raise ArtifactError(f"{self._name}：頂層 object 之後還有多餘內容——⛔ 不是 canonical JSON")
        if row_count is None:
            raise ArtifactError(f"{self._name} 缺少要串流的欄位 {stream_field!r}")
        return top, row_count

    def _array(self, on_row, canon) -> int:
        self._expect("[")
        canon.update(b"[")
        count = 0
        self._skip_ws()
        if self._peek() == "]":
            self._pos += 1
        else:
            while True:
                row = self._value()
                row_bytes = self._encode(row)
                canon.update((b"," if count else b"") + row_bytes)
                on_row(row, row_bytes)
                count += 1
                self._skip_ws()
                ch = self._peek()
                if ch == ",":
                    self._pos += 1
                    continue
                if ch == "]":
                    self._pos += 1
                    break
                raise ArtifactError(f"{self._name}：陣列元素之後預期 ',' 或 ']'，實際 {ch!r}")
        canon.update(b"]")
        return count


def stream_canonical_artifact(
    path: str | Path,
    kind: str,
    *,
    on_row: Callable[[Any, bytes], None],
    stream_field: str = "rows",
    copy_to: str | Path | None = None,
) -> StreamLoad:
    """串流讀 `.json` 或 `.json.gz`，逐列呼叫 `on_row(row, row_canonical_bytes)`。

    `copy_to`：⚠️ 在**同一趟**把 payload 寫成 canonical gzip 副本（封存用）。檔案由呼叫端
    決定路徑；⚠️ 失敗時由本函式刪掉寫到一半的副本。
    """
    path = Path(path)
    name = path.name
    if name.endswith(".json.gz"):
        gz = True
    elif name.endswith(".json"):
        gz = False
    else:
        raise ArtifactError(f"evidence 只接受 .json 或 .json.gz：{name}")

    copy_path = Path(copy_to) if copy_to is not None else None
    copy_fh = None
    try:
        try:
            fh = open(path, "rb")
        except OSError as exc:
            raise ArtifactError(f"讀不到 artifact {path}：{exc}") from exc
        with fh:
            if copy_path is not None:
                copy_fh = open(copy_path, "xb")
            source = _Source(fh, gz=gz, name=name, copy_fh=copy_fh)
            canon = hashlib.sha256()
            top, row_count = _Parser(source, name).parse(
                stream_field=stream_field, on_row=on_row, kind=kind, canon=canon,
            )
            payload_sha, stored_sha, stored_bytes, copy_sha, copy_bytes = source.finish()
        if copy_fh is not None:
            copy_fh.flush()
            os.fsync(copy_fh.fileno())
            copy_fh.close()
    except BaseException:
        if copy_fh is not None:
            copy_fh.close()
            copy_path.unlink(missing_ok=True)
        raise

    if canon.hexdigest() != payload_sha:
        if copy_path is not None:
            copy_path.unlink(missing_ok=True)
        raise ArtifactError(f"{name} 的內容不是 canonical JSON——⛔ 語意相同但編碼不同也不接受")
    version = top.get("schema_version")
    if type(version) is not int or version != ARTIFACT_SCHEMA_VERSION:
        if copy_path is not None:
            copy_path.unlink(missing_ok=True)
        raise ArtifactError(
            f"未知的 {name} schema_version={version!r}（本實作只支援 {ARTIFACT_SCHEMA_VERSION}）"
        )
    if top.get("kind") != kind:
        if copy_path is not None:
            copy_path.unlink(missing_ok=True)
        raise ArtifactError(f"{name} 的 kind={top.get('kind')!r}，預期 {kind!r}")
    return StreamLoad(
        top=top, artifact_sha256=payload_sha, stored_sha256=stored_sha,
        stored_bytes=stored_bytes, row_count=row_count,
        copy_sha256=copy_sha, copy_bytes=copy_bytes,
    )


@dataclass(frozen=True)
class AfterStream:
    """一份 after artifact 的串流摘要。⚠️ 只常駐 keys、每列 digest 與指定要留的完整列。"""

    load: StreamLoad
    keys: list[tuple[str, str, str]]            # 依檔案順序
    candidate_keys: list[tuple[str, str, str]]  # 依檔案順序
    row_digests: dict[tuple[str, str, str], bytes]  # key → sha256(canonical row bytes)
    kept_rows: dict[tuple[str, str, str], dict[str, Any]]


def stream_after_artifact(
    path: str | Path,
    *,
    label: str,
    side: str = "after",
    keep_keys=frozenset(),
    copy_to: str | Path | None = None,
) -> AfterStream:
    """串流讀一份 after artifact，套用整批 validator 的**全部** row-level 規則。

    等同於整份載入後依序呼叫 `validate_after_artifact()`、`validate_replay_errors()`、
    `validate_diagnostics(side=)`——⚠️ 三者的每一道都由 `StreamRowValidator` 以**同一組原語**
    逐列套用（見 `artifacts.py`）。⛔ provenance 的 role 由呼叫端決定，這裡不驗。
    """
    validator = StreamRowValidator(label, side=side)
    digests: dict[tuple[str, str, str], bytes] = {}
    kept: dict[tuple[str, str, str], dict[str, Any]] = {}
    keep = frozenset(keep_keys)

    def on_row(row: Any, row_bytes: bytes) -> None:
        key = validator.feed(row)
        digests[key] = hashlib.sha256(row_bytes).digest()
        if key in keep:
            kept[key] = row

    load = stream_canonical_artifact(path, AFTER_KIND, on_row=on_row, copy_to=copy_to)
    try:
        validator.finish()
        # ⚠️ 封閉欄位集合要看到 `rows`——它是被串流的欄位，這裡補一個佔位。
        validate_after_envelope(load.top | {"rows": []})
    except BaseException:
        if copy_to is not None:
            Path(copy_to).unlink(missing_ok=True)
        raise
    return AfterStream(
        load=load, keys=validator.keys, candidate_keys=validator.candidate_keys,
        row_digests=digests, kept_rows=kept,
    )
