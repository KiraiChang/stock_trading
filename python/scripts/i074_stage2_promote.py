#!/usr/bin/env python3
"""I-074 Stage 2 ⑦c：⑩ 的晉升——複本內的終態 → 真正 repo。

    python3 -B <複本>/python/scripts/i074_stage2_promote.py --work-dir <work> --real-repo <真正 repo>

規格：issue.md I-074 v29「八之三」與「Stage 2 步驟 ⑦c 細部計畫 v1」「二之三」。只由 `scripts/run-i074-stage2.sh`
的 `promote()` 呼叫——第 1 步（`/proc/locks`、preflight 0、`state-check --require run,preflight`）已在 shell 做完。

結束碼（「結束碼的分類」）：
  0 ＝ 成功 archive 已在真正 repo durable；6 ＝ failed record 已在真正 repo durable；
  3 ＝ 只有在已核對的 fd 上 `fsync()` 本身失敗（重跑 `--promote`）；
  8 ＝ 只有目的端的容量與寫入 I/O（空間不足；staging／`failed/` 的建立、寫入、rename 之前的 fsync 回
       ENOSPC／EDQUOT／EIO）與 rename 的 EEXIST／ENOTEMPTY——可以安全重跑；
  9 ＝ 其他一切（完整性漂移、驗證不過、終態不唯一、ignore 守門不通過……）——交人工。⛔ 不猜。

⚠️ **路徑錨定**：完整路徑的 `O_NOFOLLOW` 只保護最後一層（2026-10-01 實測），所以每一個檔案系統操作都錨定在
持有的 fd 上：兩個 repo 根各以 canonical 路徑開一次，之下逐層 `openat(…, O_DIRECTORY | O_NOFOLLOW)` 並持有；
stat、mkdir、開檔、listdir、刪除、statvfs、rename 全部用 `dir_fd`，⛔ 不以完整路徑做任何寫入或 fsync。
⚠️ 照實的界線：rename 只能以名稱指定來源、已持有的父目錄被搬走時 record 會落在它的新位置（可能在 repo 外）——
rename 前後的鏈檢查只能把它們**事後**變成 9；所以 ⑩ 期間另有操作契約（「二之三」）。repo 根以上的路徑⛔ 不在保證內。
⚠️ ⛔ 不呼叫 ③ 的 recovery、replay、finalize、publish：唯一呼叫的外部程式是唯讀的 `--verify-promotion-staging`
與受信任的 git。本檔只用標準庫 ＋ `_i074_bootstrap` 載入的 `canonical`／`publish`（host 3.9 可載入）。
"""
from __future__ import annotations

import argparse
import errno
import gzip
import hashlib
import io
import json
import os
import re
import secrets
import stat
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Callable, Dict, List, Optional, Tuple

sys.path.insert(0, str(Path(__file__).resolve().parent))
from _i074_bootstrap import load_replay_bundle  # noqa: E402

_RB = load_replay_bundle(Path(__file__).resolve().parent.parent, ("canonical", "publish"))
canonical = _RB["canonical"]
publish = _RB["publish"]

GIT = "/usr/bin/git"
STAGE2_PARTS = ("python", "baselines", "i074_stage2")
STAGE2_REL = "/".join(STAGE2_PARTS)
EVIDENCE = "evidence"
FAILED = "failed"
STAGING_PREFIX = ".promote-staging-"
STAGING_RE = re.compile(r"^\.promote-staging-[0-9a-f]{16}$")
# 複本內可辨識的 ③ orphan（只**忽略**、⛔ 不刪除——「三」#6）：writer 的 staging 與 probe。
_ORPHAN_RES = (re.compile(r"^\..+\.staging-[0-9a-f]{16}$"), re.compile(r"^\.probe-[0-9a-f]{16}-"))
_FAILED_NAME_RE = re.compile(r"^[^/]+-[0-9a-f]{64}$")
# 鏡像 stage2_archive（它會 import 整個 replay_bundle 圖，host 上⛔ 不載入；由測試斷言相等）。
STAGE2_MANIFEST = "evidence_manifest.json"
FAILED_RECORD = "failure_record.json"
EVIDENCE_IDENTITY = "identity/run_identity.json.gz"
_IDENTITY_MAX_BYTES = 1 << 20

EXIT_OK = 0
EXIT_DURABILITY_UNCONFIRMED = publish.EXIT_DURABILITY_UNCONFIRMED
EXIT_FAILED_RECORD = publish.EXIT_COUNTERFACTUAL_INEFFECTIVE
EXIT_PROMOTION_FAILED = publish.EXIT_PROMOTION_FAILED
EXIT_PROMOTION_BLOCKED = publish.EXIT_PROMOTION_BLOCKED
_RETRYABLE_ERRNOS = frozenset({errno.ENOSPC, errno.EDQUOT, errno.EIO})

_DIR_FLAGS = os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC
# ⚠️ O_NONBLOCK：inventory 之後被換成 FIFO 的話，開啟⛔ 不會掛住（之後的 fstat 會判 9）；對一般檔案沒有作用。
_FILE_FLAGS = os.O_RDONLY | os.O_NOFOLLOW | os.O_CLOEXEC | os.O_NONBLOCK
_CREATE_FLAGS = os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW | os.O_CLOEXEC
_CHUNK = 1 << 20
_GIT_TIMEOUT = 300
_VERIFY_KEYS = {EVIDENCE: {"kind", "target", "manifest_sha256", "base_commit", "identity_sha256"},
                FAILED: {"kind", "target", "record_sha256", "base_commit", "identity_sha256"}}


class PromotionExit(Exception):
    def __init__(self, code: int, message: str) -> None:
        super().__init__(message)
        self.code = code


def blocked(message: str) -> PromotionExit:
    return PromotionExit(EXIT_PROMOTION_BLOCKED, message)


def _dest_error(exc: OSError, what: str) -> PromotionExit:
    """目的端的錯誤：只有容量與寫入 I/O 是 8（重跑可解），其他 errno 一律 9。"""
    if exc.errno in _RETRYABLE_ERRNOS:
        return PromotionExit(EXIT_PROMOTION_FAILED, f"{what}：{exc}——目的端的容量或寫入 I/O，排除原因後重跑 --promote")
    return blocked(f"{what}：{exc}")


# ── 可以在測試中替換的系統呼叫（⛔ 沒有執行期的覆寫口：只有 pytest 以 monkeypatch 換掉） ──────────

def _fsync(fd: int) -> None:
    os.fsync(fd)


def _write(fd: int, data: bytes) -> int:
    return os.write(fd, data)


def _mkdir(name: str, mode: int, dir_fd: int) -> None:
    os.mkdir(name, mode, dir_fd=dir_fd)


def _statvfs(fd: int):
    return os.fstatvfs(fd)


def _lstat_at(dir_fd: int, name: str) -> os.stat_result:
    return os.stat(name, dir_fd=dir_fd, follow_symlinks=False)


def _rmdir(name: str, dir_fd: int) -> None:
    os.rmdir(name, dir_fd=dir_fd)


def _rename_at(src_dir_fd: int, src_name: str, dst_dir_fd: int, dst_name: str) -> None:
    publish.rename_noreplace_at(src_dir_fd, src_name, dst_dir_fd, dst_name)


# ── fd 錨定的小工具 ─────────────────────────────────────────────────────────

def _check_name(name: object) -> str:
    if type(name) is not str or not name or name in (".", "..") or "/" in name or "\x00" in name:
        raise blocked(f"名稱必須是單一 component（⛔ 不接受 /、.、..、空字串、NUL）：{name!r}")
    return name


def _open_dir(parent_fd: int, name: str) -> int:
    return os.open(_check_name(name), _DIR_FLAGS, dir_fd=parent_fd)


def _ident(fd_or_stat) -> Tuple[int, int]:
    st = os.fstat(fd_or_stat) if isinstance(fd_or_stat, int) else fd_or_stat
    return st.st_dev, st.st_ino


def _close(fd: Optional[int]) -> None:
    if fd is not None:
        try:
            os.close(fd)
        except OSError:
            pass


class StagingReplaced(Exception):
    """要清的 staging 名稱已經不是本程序建立的那一個 inode（被換掉，或名稱不見了）——⛔ 不刪除，保留現場。"""


def remove_tree_at(dir_fd: int, name: str, expect: Optional[Tuple[int, int]] = None) -> None:
    """fd 錨定的刪除：`unlinkat`／`rmdir` 加 `dir_fd`、⛔ 不跟隨 symlink（symlink 只刪連結本身）。

    沒有 `expect`（第 2 步清孤兒、遞迴刪子項目）：不存在 → 什麼都不做（冪等）。
    `expect`（裝置, inode）＝本程序建立的 staging：改走 `_remove_own_staging()`，每一個時點都核對名稱 → inode。
    """
    _check_name(name)
    if expect is not None:
        _remove_own_staging(dir_fd, name, expect)
        return
    try:
        st = _lstat_at(dir_fd, name)
    except FileNotFoundError:
        return
    if not stat.S_ISDIR(st.st_mode):
        os.unlink(name, dir_fd=dir_fd)
        return
    fd = _open_dir(dir_fd, name)
    try:
        if _ident(fd) != _ident(st):
            raise OSError(errno.ESTALE, f"{name} 在 stat 與開啟之間被換掉")
        for child in os.listdir(fd):
            remove_tree_at(fd, child)
    finally:
        os.close(fd)
    _rmdir(name, dir_fd)


def _remove_own_staging(dir_fd: int, name: str, expect: Tuple[int, int]) -> None:
    """刪本程序建立的 staging。名稱不見了或指向別的 inode 一律拋 `StagingReplaced`（→ 9）、⛔ 不刪換進來的東西：
    清理之前（實作第一、二輪 review）、stat 與開啟之間、遞迴之後 rmdir 之前、rmdir 本身（實作第三輪 review；
    rmdir 的 `ENOTDIR`／`ELOOP`——換成 symlink 或一般檔案——是第四輪 review）。

    ⚠️ 照實的界線：rmdir 只能以名稱指定——最後一次核對之後才換成**空**目錄時，rmdir 會刪掉那個空目錄（沒有內容遺失）；
    之後以仍開著的 fd 的 `st_nlink` ≠ 0 發現本程序的 staging 還連在別處 → 一樣是 9（ext4、overlayfs 實測：
    刪掉的是自己時 `st_nlink` ＝ 0）。
    """
    def check(when: str) -> None:
        try:
            st = _lstat_at(dir_fd, name)
        except FileNotFoundError:
            raise StagingReplaced(f"{when}：{name} 不見了（被搬走或刪掉），⛔ 不是本程序清掉的——原本的 {expect} 下落不明") from None
        if _ident(st) != expect:
            raise StagingReplaced(f"{when}：{name} 目前是 (dev, ino)＝{_ident(st)}，⛔ 不是本程序建立的 {expect}")

    check("清理之前")
    try:
        fd = _open_dir(dir_fd, name)
    except OSError as exc:
        if exc.errno in (errno.ENOENT, errno.ELOOP, errno.ENOTDIR):
            raise StagingReplaced(f"stat 與開啟之間：{name} 不見了或已不是目錄（{exc}），⛔ 不是本程序建立的 {expect}") from exc
        raise
    try:
        if _ident(fd) != expect:
            raise StagingReplaced(f"stat 與開啟之間：{name} 被換成 (dev, ino)＝{_ident(fd)}，⛔ 不是本程序建立的 {expect}")
        for child in os.listdir(fd):
            remove_tree_at(fd, child)
        check("rmdir 之前")
        try:
            _rmdir(name, dir_fd)
        except OSError as exc:
            # ⚠️ 實作第四輪 review：換成 symlink 或一般檔案時 rmdir 回 ENOTDIR；ELOOP 與開啟階段的分類一致
            if exc.errno in (errno.ENOENT, errno.ENOTEMPTY, errno.EEXIST, errno.ENOTDIR, errno.ELOOP):
                raise StagingReplaced(f"rmdir 時：{name} 不見了或已不是本程序清空的那個目錄（{exc}）") from exc
            raise
        if os.fstat(fd).st_nlink != 0:
            raise StagingReplaced(f"rmdir 刪掉的⛔ 不是本程序的 staging：{expect} 仍連在別處（被搬走），"
                                  f"名稱 {name} 上換進來的空目錄已被刪掉")
    finally:
        os.close(fd)


def _hash_fd(fd: int) -> Tuple[str, int]:
    h = hashlib.sha256()
    n = 0
    while True:
        chunk = os.read(fd, _CHUNK)
        if not chunk:
            return h.hexdigest(), n
        h.update(chunk)
        n += len(chunk)


@dataclass
class Entry:
    rel: str            # "" ＝ 根目錄
    kind: str           # "d" 或 "f"
    dev: int
    ino: int
    size: int
    mode: int
    blocks: int
    sha256: Optional[str] = None


class Inventory:
    """`lstat` 的封閉 inventory：只接受一般檔案（`st_nlink` ＝ 1）與目錄；之後每次開啟都以 `fstat` 核對。

    ⚠️ 逐層 `openat(…, O_NOFOLLOW)`，⛔ 不跟隨 symlink；symlink、FIFO、socket、device、hardlink → 9，
    且⛔ 不開啟它的目標。⛔ 不重用會跟隨 symlink 的 `_fsync_tree()`。
    """

    def __init__(self, root_fd: int, label: str) -> None:
        self.root_fd = root_fd
        self.label = label
        st = os.fstat(root_fd)
        if not stat.S_ISDIR(st.st_mode):
            raise blocked(f"{label} 的根不是目錄")
        self.entries: List[Entry] = [Entry("", "d", st.st_dev, st.st_ino, 0, stat.S_IMODE(st.st_mode), st.st_blocks)]
        try:
            self._walk(root_fd, "")
        except OSError as exc:
            raise blocked(f"{label} 的 inventory 讀取失敗：{exc}") from exc
        self.by_rel: Dict[str, Entry] = {e.rel: e for e in self.entries}

    def _walk(self, dir_fd: int, prefix: str) -> None:
        for name in sorted(os.listdir(dir_fd)):
            rel = prefix + name
            st = os.stat(_check_name(name), dir_fd=dir_fd, follow_symlinks=False)
            if stat.S_ISDIR(st.st_mode):
                self.entries.append(Entry(rel, "d", st.st_dev, st.st_ino, 0, stat.S_IMODE(st.st_mode), st.st_blocks))
                sub = _open_dir(dir_fd, name)
                try:
                    if _ident(sub) != _ident(st):
                        raise blocked(f"{self.label}：{rel} 在 lstat 與開啟之間被換掉")
                    self._walk(sub, rel + "/")
                finally:
                    os.close(sub)
            elif stat.S_ISREG(st.st_mode):
                if st.st_nlink != 1:
                    raise blocked(f"{self.label}：{rel} 是 hardlink（st_nlink={st.st_nlink}）——讀取與 fsync 會被帶到樹外的 inode")
                self.entries.append(Entry(rel, "f", st.st_dev, st.st_ino, st.st_size,
                                          stat.S_IMODE(st.st_mode), st.st_blocks))
            else:
                raise blocked(f"{self.label}：{rel} 不是一般檔案或目錄（symlink、FIFO、socket、device）——⛔ 不開啟它的目標")

    def files(self) -> List[Entry]:
        return [e for e in self.entries if e.kind == "f"]

    def allocated_bytes(self) -> int:
        return sum(e.blocks * 512 for e in self.entries)

    def open(self, entry: Entry) -> int:
        """依 inventory 從根逐層開啟；每一層與目標本身的 `fstat` 都必須與 inventory 相同（不符 → 9）。"""
        if entry.rel == "":
            fd = os.dup(self.root_fd)
            self._expect(fd, entry)
            return fd
        parts = entry.rel.split("/")
        cur, opened = self.root_fd, []
        try:
            for i, part in enumerate(parts[:-1]):
                nxt = _open_dir(cur, part)
                opened.append(nxt)
                self._expect(nxt, self.by_rel["/".join(parts[:i + 1])])
                cur = nxt
            flags = _DIR_FLAGS if entry.kind == "d" else _FILE_FLAGS
            fd = os.open(_check_name(parts[-1]), flags, dir_fd=cur)
            try:
                self._expect(fd, entry)
            except BaseException:
                os.close(fd)
                raise
            return fd
        finally:
            for fd in opened:
                os.close(fd)

    def _expect(self, fd: int, entry: Entry) -> None:
        st = os.fstat(fd)
        kind_ok = stat.S_ISDIR(st.st_mode) if entry.kind == "d" else stat.S_ISREG(st.st_mode)
        if not kind_ok or (st.st_dev, st.st_ino) != (entry.dev, entry.ino) or \
                (entry.kind == "f" and st.st_nlink != 1):
            raise blocked(f"{self.label}：{entry.rel or '<根>'} 與 inventory 不符（型別、裝置、inode 或 st_nlink）——完整性漂移")


def _default_git(argv: List[str], stdin: Optional[bytes] = None, timeout: Optional[float] = None) -> Tuple[int, bytes]:
    proc = subprocess.run([GIT, *argv], input=stdin, stdout=subprocess.PIPE,
                          stdin=None if stdin is not None else subprocess.DEVNULL, timeout=timeout)
    return proc.returncode, proc.stdout


def _default_verify(clone: str, path: str, target: str) -> Tuple[int, bytes]:
    proc = subprocess.run([f"{clone}/scripts/finalize-stage2-evidence.sh", "--verify-promotion-staging", path,
                           "--target", target], stdin=subprocess.DEVNULL, stdout=subprocess.PIPE)
    return proc.returncode, proc.stdout


def _default_states(work_dir: str, real_repo: str) -> Dict[str, dict]:
    import i074_stage2_preflight as pf  # noqa: PLC0415 - 同目錄的 host 模組

    return pf.check_states(Path(work_dir) / "state", require=("run", "preflight"), work_dir=work_dir,
                           real_repo=real_repo)


class Promoter:
    def __init__(self, work_dir: str, real_repo: str, *,
                 git: Callable[..., Tuple[int, bytes]] = _default_git,
                 verify: Callable[[str, str, str], Tuple[int, bytes]] = _default_verify,
                 load_states: Callable[[str, str], Dict[str, dict]] = _default_states,
                 hooks: Optional[Dict[str, Callable[[], None]]] = None) -> None:
        self.work = work_dir
        self.real = real_repo
        self.clone = f"{work_dir}/repo"
        self.git = git
        self.verifier = verify
        self.load_states = load_states
        self.hooks = hooks or {}
        self.fds: List[int] = []
        self.real_chain: List[Tuple[str, int]] = []     # (名稱, fd)：python、baselines、i074_stage2
        self.real_root_fd: Optional[int] = None
        self.real_failed_fd: Optional[int] = None
        self.staging_name: Optional[str] = None
        self.staging_fd: Optional[int] = None
        self.staging_ident: Optional[Tuple[int, int]] = None

    # ── 共用 ──────────────────────────────────────────────────────────────

    def _hook(self, name: str) -> None:
        if name in self.hooks:
            self.hooks[name]()

    def _hold(self, fd: int) -> int:
        self.fds.append(fd)
        return fd

    def _open_root(self, path: str, label: str) -> int:
        try:
            return self._hold(os.open(path, _DIR_FLAGS))
        except OSError as exc:
            raise blocked(f"開不了{label}的根：{exc}") from exc

    def _open_chain(self, root_fd: int, names, label: str) -> List[Tuple[str, int]]:
        chain, cur = [], root_fd
        for name in names:
            try:
                cur = self._hold(_open_dir(cur, name))
            except OSError as exc:
                raise blocked(f"{label}：開不了 {name}/（不存在、是 symlink 或不是目錄）：{exc}") from exc
            chain.append((name, cur))
        return chain

    def _git(self, argv: List[str], *, stdin: Optional[bytes] = None) -> Tuple[int, bytes]:
        try:
            return self.git(argv, stdin=stdin, timeout=_GIT_TIMEOUT)
        except subprocess.TimeoutExpired as exc:
            raise blocked(f"git {' '.join(argv[:3])} 逾時") from exc
        except OSError as exc:
            raise blocked(f"git 執行失敗：{exc}") from exc

    # ── 第 2 步 ──────────────────────────────────────────────────────────

    def step2_prune_and_clean(self) -> None:
        rc, _ = self._git(["-C", self.clone, "worktree", "prune"])
        if rc != 0:
            raise blocked(f"複本的 git worktree prune 失敗（rc={rc}）")
        self.real_root_fd = self._open_root(self.real, "真正 repo")
        self.real_chain = self._open_chain(self.real_root_fd, STAGE2_PARTS, "真正 repo")
        i074 = self.real_chain[-1][1]
        try:
            for name in sorted(os.listdir(i074)):
                if STAGING_RE.match(name):
                    remove_tree_at(i074, name)
        except OSError as exc:
            raise blocked(f"清不掉真正 repo 的 orphan staging：{exc}") from exc

    # ── 第 3 步 ──────────────────────────────────────────────────────────

    def step3_terminal(self, pre: dict, done: Optional[dict]) -> Tuple[str, List[str]]:
        rc, out = self._git(["-C", self.clone, "status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignored",
                             "--", STAGE2_REL + "/"])
        if rc != 0:
            raise blocked(f"讀不到複本的 git status（rc={rc}）")
        groups: Dict[str, List[str]] = {}
        unknown: List[str] = []
        prefix = STAGE2_REL + "/"
        for record in out.split(b"\x00"):
            if not record:
                continue
            line = os.fsdecode(record)
            if len(line) < 4 or line[2] != " " or line[:2] not in ("??", "!!") or not line[3:].startswith(prefix):
                raise blocked(f"複本的 {STAGE2_REL}/ 有不認得的狀態：{line!r}")
            parts = line[3 + len(prefix):].split("/")
            if any(r.match(parts[0]) for r in _ORPHAN_RES):
                continue
            if parts[0] == EVIDENCE and len(parts) > 1:
                groups.setdefault(EVIDENCE, []).append("/".join(parts[1:]))
            elif parts[0] == FAILED and len(parts) > 2:
                if any(r.match(parts[1]) for r in _ORPHAN_RES):
                    continue
                groups.setdefault(f"{FAILED}/{parts[1]}", []).append("/".join(parts[2:]))
            else:
                unknown.append("/".join(parts))
        if unknown:
            raise blocked(f"複本的 {STAGE2_REL}/ 有不認得的項目：{sorted(unknown)[:5]}")
        if len(groups) != 1:
            raise blocked(f"本次的終態必須恰好一組，實際 {sorted(groups)}（零組、或 evidence/ 與 failed record 同時存在）")
        target, files = next(iter(groups.items()))
        if target != EVIDENCE:
            want = f"{pre['bundle_id']}-{pre['counterfactual_semantic_sha256']}"
            if target != f"{FAILED}/{want}" or not _FAILED_NAME_RE.match(want):
                raise blocked(f"failed record 的名稱 {target} ≠ 本次的 {FAILED}/{want}")
        if done is not None:
            want_rc = 0 if target == EVIDENCE else 6
            if done["rc"] != want_rc:
                raise blocked(f"replay_done 的 rc={done['rc']} 與終態 {target} 不對應")
        return target, sorted(files)

    # ── 第 3b、3c、4 步 ──────────────────────────────────────────────────

    def open_source(self, target: str) -> Tuple[Inventory, List[int]]:
        clone_root = self._open_root(self.clone, "複本")
        chain = self._open_chain(clone_root, STAGE2_PARTS, "複本")
        parents = [chain[-1][1]]
        parent = parents[0]
        if target != EVIDENCE:
            parent = self._open_chain(parent, (FAILED,), "複本")[0][1]
            parents.append(parent)
        root = self._open_chain(parent, (target.split("/")[-1],), "複本")[0][1]
        return Inventory(root, f"複本的終態 {target}"), parents

    def step3c_identity(self, inv: Inventory, target: str, want_sha: str) -> None:
        rel = EVIDENCE_IDENTITY if target == EVIDENCE else FAILED_RECORD
        entry = inv.by_rel.get(rel)
        if entry is None or entry.kind != "f":
            raise blocked(f"終態裡沒有 {rel}——無法綁定 identity")
        try:
            fd = inv.open(entry)
            try:
                raw = b"".join(iter(lambda: os.read(fd, _CHUNK), b""))
            finally:
                os.close(fd)
        except OSError as exc:
            raise blocked(f"讀不到 {rel}：{exc}") from exc
        try:
            if target == EVIDENCE:
                with gzip.GzipFile(fileobj=io.BytesIO(raw), mode="rb") as gz:
                    payload = gz.read(_IDENTITY_MAX_BYTES + 1)
                if len(payload) > _IDENTITY_MAX_BYTES:
                    raise ValueError("identity 解壓後過大")
            else:
                record = json.loads(raw.decode("utf-8"))
                payload = canonical.canonical_json_bytes(record["run_identity"])
        except (OSError, ValueError, KeyError, TypeError, EOFError) as exc:
            raise blocked(f"終態封存的 identity 讀不懂：{exc}") from exc
        if hashlib.sha256(payload).hexdigest() != want_sha:
            raise blocked("終態封存的 identity ≠ preflight 記錄的 Stage 2 identity（identity_sha256）")

    def step4_source_durability(self, inv: Inventory, parents: List[int]) -> None:
        """依 inventory 開啟（錯誤或不符 → 9）、讀一次算 SHA、fsync（只有 fsync 本身失敗才是 3）。"""
        for entry in reversed(inv.entries):
            try:
                fd = inv.open(entry)
            except OSError as exc:
                raise blocked(f"來源 {entry.rel or '<根>'} 在 inventory 之後開不了：{exc}——完整性漂移") from exc
            try:
                if entry.kind == "f":
                    try:
                        entry.sha256, size = _hash_fd(fd)
                    except OSError as exc:
                        raise blocked(f"讀不到來源 {entry.rel}：{exc}") from exc
                    if size != entry.size:
                        raise blocked(f"來源 {entry.rel} 的大小與 inventory 不符——完整性漂移")
                try:
                    _fsync(fd)
                except OSError as exc:
                    raise PromotionExit(EXIT_DURABILITY_UNCONFIRMED, f"來源 {entry.rel or '<根>'} 的 fsync 失敗：{exc}") from exc
            finally:
                os.close(fd)
        for fd in reversed(parents):
            try:
                _fsync(fd)
            except OSError as exc:
                raise PromotionExit(EXIT_DURABILITY_UNCONFIRMED, f"複本 {STAGE2_REL}/ 的 fsync 失敗：{exc}") from exc

    # ── 第 5、5b 步 ──────────────────────────────────────────────────────

    def step5_destination(self, target: str) -> Tuple[int, str, Optional[os.stat_result]]:
        """回傳 (parent fd, 目的地名稱, 目的地的 lstat 或 None)。failed 的 parent 不存在 → (i074 fd, …, None)，6b 再建。"""
        i074 = self.real_chain[-1][1]
        name = target.split("/")[-1]
        parent = i074
        if target != EVIDENCE:
            try:
                self.real_failed_fd = self._hold(_open_dir(i074, FAILED))
            except FileNotFoundError:
                return i074, name, None
            except OSError as exc:
                raise blocked(f"真正 repo 的 {FAILED}/ 是 symlink 或不是目錄：{exc}") from exc
            parent = self.real_failed_fd
        try:
            st = os.stat(_check_name(name), dir_fd=parent, follow_symlinks=False)
        except FileNotFoundError:
            return parent, name, None
        except OSError as exc:
            raise blocked(f"讀不到目的地 {target}：{exc}") from exc
        if not stat.S_ISDIR(st.st_mode):
            raise blocked(f"目的地 {target} 存在但不是目錄（或是 symlink）")
        return parent, name, st

    def check_ignore(self, paths: List[str]) -> Tuple[int, set]:
        """`git check-ignore --no-index -z --stdin`（⛔ 不加 `-v`：它連否定規則的命中也回 0）。回傳 (rc, 被忽略的集合)。"""
        if not paths:
            raise blocked("ignore 守門：終態沒有任何檔案")
        rc, out = self._git(["-C", self.real, "check-ignore", "--no-index", "-z", "--stdin"],
                            stdin=b"".join(os.fsencode(p) + b"\x00" for p in paths))
        if rc not in (0, 1):
            raise blocked(f"ignore 守門：git check-ignore 失敗（rc={rc}）")
        if out and not out.endswith(b"\x00"):
            raise blocked("ignore 守門：git check-ignore 的輸出無法解析")
        ignored = {os.fsdecode(p) for p in out.split(b"\x00") if p}
        if not ignored <= set(paths) or (rc == 0) != bool(ignored):
            raise blocked("ignore 守門：git check-ignore 的輸出與輸入不符")
        return rc, ignored

    def step5b_destination_ignore(self, target: str, inv: Inventory) -> None:
        _rc, ignored = self.check_ignore([f"{STAGE2_REL}/{target}/{e.rel}" for e in inv.files()])
        if ignored:
            raise blocked(f"目的地的 ignore 守門：{sorted(ignored)[:3]} 會被 git 忽略（過廣的 ignore 規則？）——"
                          "目的地進不了版控，修好規則之後重跑 --promote")

    # ── 驗證模式、鏈檢查 ─────────────────────────────────────────────────

    def verify(self, path: str, target: str, binding_sha: str, identity_sha: str, repo_head: str) -> None:
        rc, out = self.verifier(self.clone, path, target)
        if rc != 0:
            raise blocked(f"--verify-promotion-staging 不通過（rc={rc}）：{path}")
        kind = EVIDENCE if target == EVIDENCE else FAILED
        try:
            text = out.decode("utf-8")
            if text.count("\n") != 1 or not text.endswith("\n"):
                raise ValueError("輸出必須恰好一行")
            doc = json.loads(text)
        except ValueError as exc:
            raise blocked(f"驗證模式的輸出讀不懂：{exc}") from exc
        if not isinstance(doc, dict) or set(doc) != _VERIFY_KEYS[kind]:
            raise blocked(f"驗證模式的輸出不是封閉的 key set：{sorted(doc) if isinstance(doc, dict) else doc!r}")
        sha_key = "manifest_sha256" if kind == EVIDENCE else "record_sha256"
        want = {"kind": "evidence" if kind == EVIDENCE else "failed_record", "target": target,
                sha_key: binding_sha, "identity_sha256": identity_sha, "base_commit": repo_head}
        diff = sorted(k for k in want if doc.get(k) != want[k])
        if diff:
            raise blocked(f"驗證模式的輸出與本程序經 fd 寫入／讀到的不符：{diff}——驗過的不是這一份")

    def chain_check(self, extra: List[Tuple[int, str, Tuple[int, int]]]) -> None:
        """從根路徑重走一遍：每層 (st_dev, st_ino) ＝ 持有的 fd；`extra` 是 (持有的 parent fd, 名稱, 預期)。"""
        reopened: List[int] = []
        try:
            cur = os.open(self.real, _DIR_FLAGS)
            reopened.append(cur)
            mapping = {self.real_root_fd: cur}
            if _ident(cur) != _ident(self.real_root_fd):
                raise blocked("鏈檢查：真正 repo 的根被換掉了")
            held = list(self.real_chain)
            if self.real_failed_fd is not None:
                held.append((FAILED, self.real_failed_fd))
            for name, fd in held:
                nxt = _open_dir(cur, name)
                reopened.append(nxt)
                if _ident(nxt) != _ident(fd):
                    raise blocked(f"鏈檢查：{name}/ 被換掉或搬走了——交人工")
                mapping[fd] = cur = nxt
            for parent_fd, name, want in extra:
                st = os.stat(_check_name(name), dir_fd=mapping[parent_fd], follow_symlinks=False)
                if (st.st_dev, st.st_ino) != want:
                    raise blocked(f"鏈檢查：{name} 不是預期的那一個 inode——交人工")
        except OSError as exc:
            raise blocked(f"鏈檢查：{exc}——真正 repo 的目錄在晉升期間被換掉或搬走了，交人工") from exc
        finally:
            for fd in reopened:
                _close(fd)

    # ── 6a ───────────────────────────────────────────────────────────────

    def step6a(self, src: Inventory, parent: int, name: str, target: str, pre: dict, repo_head: str) -> int:
        try:
            dest_root = self._hold(_open_dir(parent, name))
        except OSError as exc:
            raise blocked(f"開不了既有的目的地 {target}：{exc}") from exc
        dest = Inventory(dest_root, f"目的地 {target}")
        if [(e.rel, e.kind) for e in dest.entries] != [(e.rel, e.kind) for e in src.entries]:
            raise blocked(f"目的地 {target} 已存在且與來源的檔案集合不同——⛔ 不覆寫，交人工")
        self._compare(dest, src, f"目的地 {target} 已存在且與來源的內容不同——⛔ 不覆寫，交人工")
        # 目的地 ≡ 來源（剛逐位元比對過）：綁定值就是來源在第 4 步經 fd 算出的那一個。
        binding = src.by_rel[STAGE2_MANIFEST if target == EVIDENCE else FAILED_RECORD]
        self.verify(f"{self.real}/{STAGE2_REL}/{target}", target, binding.sha256, pre["identity_sha256"], repo_head)
        self._compare(dest, src, f"收尾重算：目的地 {target} 在驗證之後被改寫——⛔ 不回 0／6，交人工")
        self._hook("after_final_rehash")
        self.chain_check([(parent, name, (dest.entries[0].dev, dest.entries[0].ino))])
        for entry in reversed(dest.entries):
            try:
                fd = dest.open(entry)
            except OSError as exc:
                raise blocked(f"目的地 {entry.rel or '<根>'} 開不了：{exc}") from exc
            try:
                _fsync(fd)
            except OSError as exc:
                raise PromotionExit(EXIT_DURABILITY_UNCONFIRMED, f"目的地 {entry.rel or '<根>'} 的 fsync 失敗：{exc}") from exc
            finally:
                os.close(fd)
        # ⚠️ 實作第一輪 review（高）：failed record 的目的地在 failed/ 底下，failed/ 自己的目錄項目在 i074_stage2/——一併 fsync。
        parents = [parent] if target == EVIDENCE else [parent, self.real_chain[-1][1]]
        for fd in parents:
            try:
                _fsync(fd)
            except OSError as exc:
                raise PromotionExit(EXIT_DURABILITY_UNCONFIRMED, f"目的地 parent 的 fsync 失敗：{exc}") from exc
        return EXIT_OK if target == EVIDENCE else EXIT_FAILED_RECORD

    def _compare(self, other: Inventory, src: Inventory, message: str) -> None:
        """`other` 的每個檔案經 fd 重算 SHA ＝ 來源在第 4 步算出的值（讀取錯誤或不符 → 9）。"""
        for entry in other.files():
            try:
                fd = other.open(entry)
                try:
                    got, size = _hash_fd(fd)
                finally:
                    os.close(fd)
            except OSError as exc:
                raise blocked(f"{message}（{entry.rel}：{exc}）") from exc
            want = src.by_rel[entry.rel]
            if (got, size) != (want.sha256, want.size):
                raise blocked(f"{message}（{entry.rel}）")

    # ── 6b ───────────────────────────────────────────────────────────────

    def step6b(self, src: Inventory, parent: int, name: str, target: str, pre: dict, repo_head: str) -> int:
        i074 = self.real_chain[-1][1]
        staging = STAGING_PREFIX + secrets.token_hex(8)
        _rc, ignored = self.check_ignore([f"{STAGE2_REL}/{staging}/{e.rel}" for e in src.files()])
        if ignored != {f"{STAGE2_REL}/{staging}/{e.rel}" for e in src.files()}:
            raise blocked("staging 的 ignore 守門：staging 不會被 git 完全忽略（.gitignore 缺少 "
                          f"{STAGE2_REL}/.promote-staging-* 或被否定規則蓋掉）——⛔ 不建 staging")
        try:
            sv = _statvfs(i074)
        except OSError as exc:
            raise blocked(f"讀不到真正 repo 的可用空間：{exc}") from exc
        need = 2 * src.allocated_bytes()
        if sv.f_bavail * sv.f_frsize < need:
            raise PromotionExit(EXIT_PROMOTION_FAILED, f"空間不足：可用 {sv.f_bavail * sv.f_frsize} < 需要 {need}"
                                "（來源 allocated bytes 的 2 倍）——釋出空間之後重跑 --promote")
        try:
            _mkdir(staging, src.entries[0].mode, i074)
        except OSError as exc:
            raise _dest_error(exc, "建不了 staging")
        self.staging_name = staging
        try:
            self.staging_fd = self._hold(_open_dir(i074, staging))
        except OSError as exc:
            raise blocked(f"剛建立的 staging 開不了：{exc}") from exc
        self.staging_ident = _ident(self.staging_fd)
        self._hook("before_copy")
        made = self._copy(src, self.staging_fd)
        self._compare_made(made, src, "staging 與來源逐位元比對不符")
        binding = src.by_rel[STAGE2_MANIFEST if target == EVIDENCE else FAILED_RECORD]
        self.verify(f"{self.real}/{STAGE2_REL}/{staging}", target, binding.sha256, pre["identity_sha256"], repo_head)
        for entry in reversed(src.entries):        # 先子後父
            fd = self._open_made(made, entry.rel)
            try:
                _fsync(fd)
            except OSError as exc:
                raise _dest_error(exc, f"staging {entry.rel or '<根>'} 的 fsync 失敗")
            finally:
                os.close(fd)
        if target != EVIDENCE:
            if self.real_failed_fd is None:
                try:
                    _mkdir(FAILED, 0o755, i074)
                except OSError as exc:
                    raise _dest_error(exc, f"建不了真正 repo 的 {FAILED}/")
                try:
                    self.real_failed_fd = self._hold(_open_dir(i074, FAILED))
                except OSError as exc:
                    raise blocked(f"剛建立的 {FAILED}/ 開不了：{exc}") from exc
                parent = self.real_failed_fd
            # ⚠️ 實作第一輪 review（高）：⛔ 不能只在新建時才 fsync——上一次可能建了 failed/ 卻在 fsync 失敗後回 8，重跑時
            #    failed/ 已存在；它在 i074_stage2/ 裡的目錄項目每一次都要在 rename 之前確認落盤。
            try:
                _fsync(i074)
            except OSError as exc:
                raise _dest_error(exc, f"fsync {STAGE2_REL}/（{FAILED}/ 的目錄項目）失敗")
        self._compare_made(made, src, "收尾重算：staging 在驗證之後被改寫——⛔ 不發布")
        self._hook("after_final_rehash")
        staging_ident = made[""][:2]
        self.chain_check([(i074, staging, staging_ident)])
        self._hook("before_rename")
        try:
            _rename_at(i074, staging, parent, name)
        except FileExistsError as exc:
            raise PromotionExit(EXIT_PROMOTION_FAILED, f"目的地 {target} 已存在（EEXIST）——重跑 --promote 走 6a") from exc
        except (OSError, ValueError, publish.PublishError) as exc:     # 含 NoClobberUnsupported（EXDEV／EINVAL／ENOSYS）
            raise blocked(f"no-clobber rename 失敗：{exc}") from exc
        self.staging_name = None
        self.chain_check([(parent, name, staging_ident)])
        # ⚠️ 實作第一輪 review（高）：跨目錄 rename（staging 在 i074_stage2/、failed record 的目的地在 failed/）之後，
        #    目的地的 parent 與來源的 parent 都要 fsync；任一失敗 → 3。
        parents = [(f"{name} 的 parent", parent)]
        if parent != i074:
            parents.append((f"{STAGE2_REL}/", i074))
        for label, fd in parents:
            try:
                _fsync(fd)
            except OSError as exc:
                raise PromotionExit(EXIT_DURABILITY_UNCONFIRMED, f"rename 之後 fsync {label} 失敗：{exc}") from exc
        return EXIT_OK if target == EVIDENCE else EXIT_FAILED_RECORD

    def _copy(self, src: Inventory, staging_fd: int) -> Dict[str, Tuple[int, int, str]]:
        """依 inventory 逐項建立與逐位元複製；回傳 staging 內每一項的 (dev, ino, 型別)（"" ＝ staging 本身）。"""
        made: Dict[str, Tuple[int, int, str]] = {"": (*_ident(staging_fd), "d")}
        dir_fds: Dict[str, int] = {"": staging_fd}
        try:
            for entry in src.entries[1:]:
                head, _, base = entry.rel.rpartition("/")
                parent = dir_fds[head]
                if entry.kind == "d":
                    try:
                        _mkdir(base, entry.mode, parent)
                    except OSError as exc:
                        raise _dest_error(exc, f"staging 建不了目錄 {entry.rel}")
                    try:
                        fd = _open_dir(parent, base)
                    except OSError as exc:
                        raise blocked(f"staging 剛建立的 {entry.rel} 開不了：{exc}") from exc
                    dir_fds[entry.rel] = fd
                    made[entry.rel] = (*_ident(fd), "d")
                    continue
                try:
                    src_fd = src.open(entry)
                except OSError as exc:
                    raise blocked(f"複製途中來源 {entry.rel} 開不了：{exc}——完整性漂移") from exc
                try:
                    try:
                        dst_fd = os.open(_check_name(base), _CREATE_FLAGS, entry.mode, dir_fd=parent)
                    except OSError as exc:
                        raise _dest_error(exc, f"staging 建不了檔案 {entry.rel}")
                    try:
                        made[entry.rel] = (*_ident(dst_fd), "f")
                        got = self._copy_bytes(src_fd, dst_fd, entry)
                    finally:
                        os.close(dst_fd)
                finally:
                    os.close(src_fd)
                if got != (entry.sha256, entry.size):
                    raise blocked(f"複製途中來源 {entry.rel} 的內容變了——完整性漂移")
        finally:
            for rel, fd in dir_fds.items():
                if rel:
                    _close(fd)
        return made

    @staticmethod
    def _copy_bytes(src_fd: int, dst_fd: int, entry: Entry) -> Tuple[str, int]:
        h, n = hashlib.sha256(), 0
        while True:
            try:
                chunk = os.read(src_fd, _CHUNK)
            except OSError as exc:
                raise blocked(f"讀不到來源 {entry.rel}：{exc}") from exc
            if not chunk:
                return h.hexdigest(), n
            h.update(chunk)
            n += len(chunk)
            view = memoryview(chunk)
            while view:
                try:
                    written = _write(dst_fd, view)
                except OSError as exc:
                    raise _dest_error(exc, f"寫入 staging 的 {entry.rel} 失敗")
                view = view[written:]

    def _open_made(self, made: Dict[str, Tuple[int, int, str]], rel: str) -> int:
        """從 staging 的 fd 逐層開啟、核對 (dev, ino) ＝ 建立時的值（開不了或不符 → 9）。"""
        try:
            if rel == "":
                fd = os.dup(self.staging_fd)
            else:
                cur, opened, parts = self.staging_fd, [], rel.split("/")
                try:
                    for i, part in enumerate(parts[:-1]):
                        cur = _open_dir(cur, part)
                        opened.append(cur)
                        if _ident(cur) != made["/".join(parts[:i + 1])][:2]:
                            raise blocked(f"staging 的 {'/'.join(parts[:i + 1])} 被換掉了")
                    flags = _DIR_FLAGS if made[rel][2] == "d" else _FILE_FLAGS
                    fd = os.open(_check_name(parts[-1]), flags, dir_fd=cur)
                finally:
                    for o in opened:
                        os.close(o)
            if _ident(fd) != made[rel][:2]:
                os.close(fd)
                raise blocked(f"staging 的 {rel or '<根>'} 被換掉了")
            return fd
        except OSError as exc:
            raise blocked(f"staging 的 {rel or '<根>'} 開不了：{exc}——完整性漂移") from exc

    def _compare_made(self, made: Dict[str, Tuple[int, int, str]], src: Inventory, message: str) -> None:
        """staging 的每個檔案經 fd 重算 SHA ＝ 來源在第 4 步算出的值（讀取錯誤或不符 → 9）。"""
        for entry in src.files():
            fd = self._open_made(made, entry.rel)
            try:
                got = _hash_fd(fd)
            except OSError as exc:
                raise blocked(f"{message}（{entry.rel}：{exc}）") from exc
            finally:
                os.close(fd)
            if got != (entry.sha256, entry.size):
                raise blocked(f"{message}（{entry.rel}）")

    # ── 主流程 ───────────────────────────────────────────────────────────

    def run(self) -> int:
        try:
            states = self.load_states(self.work, self.real)
        except Exception as exc:  # noqa: BLE001 - state 不符一律 9（⛔ 不猜）
            raise blocked(f"state 不符：{exc}") from exc
        pre, done, repo_head = states["preflight"], states.get("replay_done"), states["run"]["repo_head"]
        try:
            self.step2_prune_and_clean()
            target, listed = self.step3_terminal(pre, done)
            src, clone_parents = self.open_source(target)
            if sorted(e.rel for e in src.files()) != listed:
                raise blocked(f"終態 {target} 的檔案與複本 git status 列出的不符——完整性漂移")
            self._hook("after_inventory")
            self.step3c_identity(src, target, pre["identity_sha256"])
            self.step4_source_durability(src, clone_parents)
            parent, name, existing = self.step5_destination(target)
            self.step5b_destination_ignore(target, src)
            if existing is not None:
                return self.step6a(src, parent, name, target, pre, repo_head)
            return self.step6b(src, parent, name, target, pre, repo_head)
        finally:
            kept = self._cleanup_staging()
            for fd in reversed(self.fds):
                _close(fd)
            self.fds.clear()
            if kept is not None:
                original = sys.exc_info()[1]
                raise blocked(f"{kept}（原本的結束原因：{original}）" if original is not None else kept)

    def _cleanup_staging(self) -> Optional[str]:
        """rename 之前的任何失敗都清掉**本次建立的** staging。⚠️ 實作第一～三輪 review（中）：清理途中任何時點名稱不見了
        或⛔ 不是建立時的 inode（被搬走、刪掉或換掉）就⛔ 不刪換進來的東西，回傳訊息（→ 9、保留現場交人工）——⛔ 不沿用
        代表「可安全重跑」的 8（時點見 `_remove_own_staging()`）。其他清理錯誤只回報（下一次 --promote 的第 2 步會清）。"""
        if self.staging_name is None or not self.real_chain:
            return None
        if self.staging_ident is None:
            print(f"ERROR: 本次的 staging {self.staging_name} 建立之後開不了——⛔ 不確定它是哪一個 inode，⛔ 不刪"
                  "（下一次 --promote 的第 2 步會清）", file=sys.stderr)
            return None
        try:
            remove_tree_at(self.real_chain[-1][1], self.staging_name, expect=self.staging_ident)
        except StagingReplaced as exc:
            return f"本次的 staging 已被換掉、搬走或刪掉——⛔ 不刪除、保留現場，交人工：{exc}"
        except (OSError, PromotionExit) as exc:
            print(f"ERROR: 清不掉本次的 staging {self.staging_name}：{exc}（下一次 --promote 的第 2 步會清）",
                  file=sys.stderr)
        return None


def main(argv: Optional[List[str]] = None) -> int:
    parser = argparse.ArgumentParser(prog="i074_stage2_promote", allow_abbrev=False)
    parser.add_argument("--work-dir", required=True)
    parser.add_argument("--real-repo", required=True)
    args = parser.parse_args(argv)
    try:
        rc = Promoter(args.work_dir, args.real_repo).run()
    except PromotionExit as exc:
        print(f"ERROR: 晉升（rc={exc.code}）：{exc}", file=sys.stderr)
        return exc.code
    except Exception as exc:  # noqa: BLE001 - 未列出的任何例外一律 9（⛔ 不猜）
        print(f"ERROR: 晉升：未預期的例外 {type(exc).__name__}: {exc}——⛔ 不猜，交人工", file=sys.stderr)
        return EXIT_PROMOTION_BLOCKED
    print(f"==> 晉升完成（rc={rc}）：終態已在真正 repo durable", file=sys.stderr)
    return rc


if __name__ == "__main__":
    raise SystemExit(main())
