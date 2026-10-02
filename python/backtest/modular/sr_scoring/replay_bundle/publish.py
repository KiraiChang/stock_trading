"""Stage 0 的整包原子發布——`renameat2(RENAME_NOREPLACE)`。

**為什麼不是逐檔 `os.replace`**：bundle 是**多檔目錄**。先建正式目錄再逐檔替換的話，
①別的程序會在發布途中看到一個半成品的正式目錄；②中途失敗時正式目錄會存在，
與「執行前不存在 → 失敗後仍不存在」直接矛盾；③兩個 producer 同時產同一 ID 時，
「先 `exists()` 再發布」本身就有 TOCTOU。

**為什麼不是 `os.mkdir` claim 之後再 rename**：那會先把一個**空的正式目錄公開出去**，
reader 看得到空目錄、輸家只能在空目錄上中止（達不到 no-op）、rename 前崩潰就留下空的
正式目錄。所以用真正的 no-clobber rename，讓正式路徑**一次完整出現**。

⛔ **`os.rename` / `os.replace` 不能用來發布目錄**：POSIX 的 `rename()` 會**直接蓋掉
既有的空目錄**。

⛔ **probe 不通過就 fail-closed，本模組不提供弱化的 fallback**：`O_EXCL` 發布鎖的
no-clobber 只在合作者之間成立，用弱化保證去發布「不可竄改的證據」是本末倒置。

完整規格見 docs/issue.md I-100 計畫書「七-B、Stage 0 的整包原子發布」。
"""
from __future__ import annotations

import ctypes
import ctypes.util
import errno
import os
import platform
import secrets
from pathlib import Path
from typing import Callable

AT_FDCWD = -100
RENAME_NOREPLACE = 1

# 一般失敗（輸入未就位、驗證不過、rename 之前的任何一步）走 exit 1；
# 「已發布／既有 bundle 有效，但 durability 未確認」是**另一種狀態**，不是發布失敗，
# 所以給它專屬碼——呼叫端才分得出「要重跑」與「重跑會走 no-op」。
EXIT_ABORT = 1
EXIT_DURABILITY_UNCONFIRMED = 3
# Stage 2 的候選集合不一致——⚠️ 這是**終止狀態**（分支 C 的證據已完整記錄），
# ⛔ 不是一般失敗，所以與 EXIT_ABORT 分開。
EXIT_CANDIDATE_MISMATCH = 4
# I-074 Stage 1 的 crossday：兩份 after artifact 的逐列結果或執行身分不一致。
# ⚠️ 與 4 同樣是**終止狀態**（證據已完整發布），⛔ 不是一般失敗。
EXIT_CROSSDAY_MISMATCH = 5
# I-074 Stage 2 的反事實 replay：counterfactual patch **沒有生效**（「①之三」的檢查順序得出
# `candidate_flag_inconsistent` 或 `rr_not_restored`）。⚠️ 與 4 不同：它是 **tooling 缺陷信號**，
# ⛔ 不是產品發現；replay 已在 operational 目錄留下 `bounded_diagnostics.json`，由 orchestrator
# 發布 failed-attempt record（issue.md I-074 Stage 2 計畫書「二、①」）。
EXIT_COUNTERFACTUAL_INEFFECTIVE = 6
# I-074 Stage 2 的環境見證：新 image 上的 after' 與 Stage 1 D+1 **不等價**。
# ⚠️ 與 5 同樣是**終止狀態**（見證證據已完整發布），⛔ 不是一般失敗；
# 它讓 Stage 2 停在 before 之前（issue.md I-074 Stage 2 計畫書「二、⑤」）。
EXIT_ENV_NOT_EQUIVALENT = 7
# I-074 Stage 2 ⑩ 的晉升（issue.md I-074 v29「八之三」、「Stage 2 步驟 ⑦c 細部計畫 v1」「二之三」）。
# ⚠️ **唯一定義處**：supervisor 與 orchestrator ⛔ 不 import repo 模組，各自鏡像這兩個值，由測試斷言相等。
# 8 ＝ 本次呼叫沒有確認晉升完成，但可以安全重跑 `--promote`（只用在目的端的容量與寫入 I/O、
#     no-clobber rename 的 EEXIST、取不到鎖）；⛔ 不承諾目的地不存在。
# 9 ＝ 需要人工判斷（完整性漂移、驗證不過、終態不唯一……）。
EXIT_PROMOTION_FAILED = 8
EXIT_PROMOTION_BLOCKED = 9

# syscall 號碼是 **per-ABI** 的，猜錯會呼叫到完全不同的系統呼叫。
# 只在找不到 libc 的 renameat2 symbol 時才會用到，且未知架構一律 fail-closed。
_RENAMEAT2_SYSCALL_NR = {
    "x86_64": 316,
    "aarch64": 276,
}


class PublishError(RuntimeError):
    """發布失敗（rename 之前）。正式路徑不存在或維持原狀。"""


class NoClobberUnsupported(PublishError):
    """檔案系統不支援 `RENAME_NOREPLACE`，或 staging 與正式路徑不同 filesystem。"""


class DurabilityUnconfirmed(RuntimeError):
    """⚠️ **不是發布失敗**：bundle 已發布（或既有的那份有效）且可載入，只是還沒確認落盤。

    重跑同一條指令會走 no-op 並重新 fsync，⛔ 不需要也不應該刪除重來。
    """

    def __init__(self, message: str, *, published: bool) -> None:
        super().__init__(message)
        # published=True → 這次 rename 成功才發生；False → 走的是 no-op 路徑。
        self.published = published


def _load_renameat2() -> Callable[[bytes, bytes], int]:
    """回傳 `(src, dst) -> 0 或 -1`（兩個路徑都相對於目前目錄，即 `AT_FDCWD`；失敗時 errno 由 `ctypes.get_errno()` 取得）。

    ⚠️ 這個兩參數的簽章是**既有契約**：⑩ 的 replay 跑在 `e1cbbbd` ＋ tooling patch 上，`e1cbbbd` 的測試以兩參數的
    fake 替換它（`test_exdev_is_fail_closed`）——⛔ 不能改成帶 dirfd。dirfd 版是 `_load_renameat2_at()`
    （I-074 ⑦c 實作結果「與計畫的差異」）。
    """
    call = _load_renameat2_at()
    return lambda src, dst: call(AT_FDCWD, src, AT_FDCWD, dst)


def _load_renameat2_at() -> Callable[[int, bytes, int, bytes], int]:
    """回傳 `(src_dir_fd, src, dst_dir_fd, dst) -> 0 或 -1`（失敗時 errno 由 `ctypes.get_errno()` 取得）。

    1. **優先用 libc 的 `renameat2` symbol**（glibc >= 2.28 有這個包裝，
       `python:3.11-slim` 的 Debian glibc 符合）——這樣就不必自己維護 syscall 號碼；
    2. 找不到 symbol 時才退回 `syscall()`，且限定架構 allowlist。
    """
    libc = ctypes.CDLL(None, use_errno=True)

    fn = getattr(libc, "renameat2", None)
    if fn is not None:
        fn.argtypes = [ctypes.c_int, ctypes.c_char_p, ctypes.c_int, ctypes.c_char_p, ctypes.c_uint]
        fn.restype = ctypes.c_int

        def _call(src_dir_fd: int, src: bytes, dst_dir_fd: int, dst: bytes) -> int:
            ctypes.set_errno(0)
            return fn(src_dir_fd, src, dst_dir_fd, dst, RENAME_NOREPLACE)

        return _call

    machine = platform.machine()
    nr = _RENAMEAT2_SYSCALL_NR.get(machine)
    if nr is None:
        raise NoClobberUnsupported(
            f"這個環境的 libc 沒有 renameat2 symbol，而架構 {machine!r} 不在已知的 syscall "
            f"號碼清單內（{sorted(_RENAMEAT2_SYSCALL_NR)}）。syscall 號碼是 per-ABI 的，"
            "猜錯會呼叫到完全不同的系統呼叫，所以這裡 fail-closed，不套用任何一個號碼。"
        )

    syscall = libc.syscall
    syscall.argtypes = [ctypes.c_long, ctypes.c_int, ctypes.c_char_p, ctypes.c_int,
                        ctypes.c_char_p, ctypes.c_uint]
    syscall.restype = ctypes.c_long

    def _call_syscall(src_dir_fd: int, src: bytes, dst_dir_fd: int, dst: bytes) -> int:
        ctypes.set_errno(0)
        return int(syscall(nr, src_dir_fd, src, dst_dir_fd, dst, RENAME_NOREPLACE))

    return _call_syscall


def rename_noreplace(src: Path, dst: Path) -> None:
    """no-clobber rename（完整路徑，相對於目前目錄）。目的已存在時 raise `FileExistsError`。

    ⛔ 不吞任何 errno：`EEXIST` 交給呼叫端走 no-op 判定，`EXDEV`／`EINVAL`／`ENOSYS`
    一律轉成 `NoClobberUnsupported`，其餘原樣拋 `OSError`。
    ⚠️ 多層路徑照舊接受（既有的發布路徑都傳完整路徑）；單一 component 的規則只屬於 `rename_noreplace_at()`。
    """
    src_b, dst_b = _encode_rename_paths(str(src), str(dst))
    _raise_for_rename(_load_renameat2()(src_b, dst_b), src_b, dst_b)


def rename_noreplace_at(src_dir_fd: int, src_name: str, dst_dir_fd: int, dst_name: str) -> None:
    """no-clobber rename，**錨定在兩個已持有的目錄 fd 上**（I-074 ⑦c 的晉升用）。

    兩個名稱都必須是單一 component（⛔ 不接受 `/`、`.`、`..`、空字串、NUL）、兩個 fd 都必須是非負整數
    （`type(fd) is int`——⛔ 不接受 `bool`（`True`／`False` 會被當成 fd 1／0）與 `AT_FDCWD`）；
    不符一律 `ValueError`，⛔ 不呼叫 syscall。errno 的對應與 `rename_noreplace()` 相同。
    """
    for label, fd in (("src_dir_fd", src_dir_fd), ("dst_dir_fd", dst_dir_fd)):
        if type(fd) is not int or fd < 0:
            raise ValueError(f"{label} 必須是非負整數的目錄 fd（⛔ 不接受 bool 與 AT_FDCWD）：{fd!r}")
    for label, name in (("src_name", src_name), ("dst_name", dst_name)):
        if type(name) is not str or not name or name in (".", "..") or "/" in name or "\x00" in name:
            raise ValueError(f"{label} 必須是單一 component 的名稱（⛔ 不接受 /、.、..、空字串、NUL）：{name!r}")
    src_b, dst_b = _encode_rename_paths(src_name, dst_name)
    _raise_for_rename(_load_renameat2_at()(src_dir_fd, src_b, dst_dir_fd, dst_b), src_b, dst_b)


def _encode_rename_paths(src_path, dst_path) -> tuple[bytes, bytes]:
    """兩個公開函式共用：路徑含 NUL 一律 `ValueError`——`ctypes.c_char_p` 會在 NUL 截斷（2026-10-01 實測
    `a\\x00ignored` 會把 `a` rename 掉），⛔ 不讓它去 rename 另一個路徑。"""
    src, dst = os.fsencode(src_path), os.fsencode(dst_path)
    if b"\x00" in src or b"\x00" in dst:
        raise ValueError(f"rename 的路徑含 NUL（ctypes 會在 NUL 截斷）：{src!r} → {dst!r}")
    return src, dst


def _raise_for_rename(rc: int, src: bytes, dst: bytes) -> None:
    """兩個公開函式共用的 errno 對應（⚠️ 必須緊接在呼叫之後：errno 取自 ctypes 的私有副本）。"""
    if rc == 0:
        return
    err = ctypes.get_errno()
    src, dst = os.fsdecode(src), os.fsdecode(dst)
    if err == errno.EEXIST or err == errno.ENOTEMPTY:
        raise FileExistsError(err, os.strerror(err), str(dst))
    if err == errno.EXDEV:
        raise NoClobberUnsupported(
            f"staging（{src}）與正式路徑（{dst}）不在同一個 filesystem，renameat2 回 EXDEV。"
            "⛔ 不改用 copy——那會讓正式路徑暴露半成品並失去 no-clobber。"
            f"{_REMEDIATION}"
        )
    if err in (errno.ENOSYS, errno.EINVAL, errno.EOPNOTSUPP):
        raise NoClobberUnsupported(
            f"這個 filesystem 不支援 RENAME_NOREPLACE（errno={errno.errorcode.get(err, err)}）。"
            f"{_REMEDIATION}"
        )
    raise OSError(err, os.strerror(err), str(src))


_REMEDIATION = (
    "\n補救方式：**讓最終的 python/baselines/ 本身落在支援 RENAME_NOREPLACE 的 filesystem 上**"
    "（搬移或重新掛載整個 repo／該目錄）再重跑。"
    "\n⛔ 不要「產在別處再搬進來」：跨 filesystem 時 renameat2 直接回 EXDEV，而 mv 會退化成 "
    "copy ＋ delete——正式路徑又暴露半成品，no-clobber 也沒了。"
    "\n要支援「從別處匯入既有 bundle」的話，得另定一套同樣原子且 no-clobber 的匯入流程，本計畫不做。"
)


def fsync_dir(path: Path) -> None:
    """fsync 一個目錄——讓目錄項目本身落盤，不是裡面的檔案。"""
    fd = os.open(str(path), os.O_RDONLY | getattr(os, "O_DIRECTORY", 0))
    try:
        os.fsync(fd)
    finally:
        os.close(fd)


def fsync_file(path: Path) -> None:
    fd = os.open(str(path), os.O_RDONLY)
    try:
        os.fsync(fd)
    finally:
        os.close(fd)


def probe_no_clobber(base_dir: Path) -> dict[str, str]:
    """在 `base_dir` 內實測 no-clobber rename，回傳寫進執行 log 的結果。

    ⚠️ **probe 要完全複製正式操作的形狀：rename 的是「目錄」，⛔ 不是檔案**——
    檔案 rename 通過不代表目錄 rename 也通過。

    | 組 | 形狀 | 預期 |
    |---|---|---|
    | A | 隨機 source 目錄 → **不存在**的 destination 目錄 | 成功 |
    | B | 隨機 source 目錄 → **已存在**的 destination 目錄 | `EEXIST`，且兩邊都完全不變 |

    ⚠️ 結果只寫執行 log，⛔ **不寫進 manifest**——那是內容身分，與環境無關。
    """
    token = secrets.token_hex(8)
    a_src = base_dir / f".probe-{token}-a-src"
    a_dst = base_dir / f".probe-{token}-a-dst"
    b_src = base_dir / f".probe-{token}-b-src"
    b_dst = base_dir / f".probe-{token}-b-dst"
    created = [a_src, a_dst, b_src, b_dst]
    try:
        # A：目的不存在 → 應成功
        a_src.mkdir()
        (a_src / "marker").write_text("a", encoding="utf-8")
        rename_noreplace(a_src, a_dst)
        if a_src.exists() or not (a_dst / "marker").is_file():
            raise NoClobberUnsupported(
                f"probe A 異常：rename 後 source 仍在或 destination 內容不完整（{base_dir}）。{_REMEDIATION}"
            )

        # B：目的已存在 → 應回 EEXIST，且 source 與 destination 都不變
        b_src.mkdir()
        (b_src / "marker").write_text("src", encoding="utf-8")
        b_dst.mkdir()
        (b_dst / "marker").write_text("dst", encoding="utf-8")
        try:
            rename_noreplace(b_src, b_dst)
        except FileExistsError:
            pass
        else:
            raise NoClobberUnsupported(
                f"probe B 異常：destination 已存在卻 rename 成功——這個 filesystem 沒有真的支援 "
                f"RENAME_NOREPLACE（{base_dir}）。{_REMEDIATION}"
            )
        if (b_src / "marker").read_text(encoding="utf-8") != "src" or \
           (b_dst / "marker").read_text(encoding="utf-8") != "dst":
            raise NoClobberUnsupported(
                f"probe B 異常：EEXIST 之後 source 或 destination 被動到了（{base_dir}）。{_REMEDIATION}"
            )
        return {"renameat2": "ok", "probe_a": "renamed", "probe_b": "eexist"}
    finally:
        # ⚠️ 所有 probe 目錄在每一條路徑（含例外）都要清掉。
        for path in created:
            remove_tree(path)


def remove_tree(path: Path) -> None:
    """盡力刪除一棵樹。⛔ 只用於 staging 與 probe，**任何情況都不得用在正式路徑上**。"""
    import shutil

    try:
        shutil.rmtree(path)
    except FileNotFoundError:
        return
    except NotADirectoryError:
        try:
            path.unlink()
        except FileNotFoundError:
            return
