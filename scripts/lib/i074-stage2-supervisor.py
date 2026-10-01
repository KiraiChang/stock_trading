#!/usr/bin/python3 -I
"""I-074 Stage 2 ⑦b：⑩ 的 supervisor（issue.md I-074 v29「八之一之二」＋「Stage 2 步驟 ⑦b 細部計畫 v1」「二之四」）。

由 `scripts/run-i074-stage2.sh` 的入口以 `exec /usr/bin/python3 -I <本檔> {run|resume|promote} --work-dir <dir>
[--freeze-record <path>]` 啟動；⛔ 不是給人直接用的入口。host 端、**只用標準庫、相容 Python 3.9**（host 是 3.9.2）。

依序：取鎖之前的信任根（初始 user namespace、四個系統程式）→ 驗自身 ＝ HEAD → 拒絕帶 `I074_STAGE2_*` 的環境 → 執行帳號 →
取鎖（`O_NOFOLLOW`、`fstat`／`lstat` 驗屬性與 inode、非阻塞 `flock`）→ 沒有舊 sentinel → image 的 labels 沒有鍵 →
沒有殘留容器 → 建 sentinel → child subreaper → 以 `/bin/bash -p` 啟動持鎖階段（parent-death TERM）→ 等待 → 正常釋放。

⚠️ **沒有任何解除入口**（「八之一之二」第 9 列）：supervisor 被 SIGKILL 之後 sentinel 留下，只能重開機。
⚠️ 測試**⛔ 不碰 `/run/lock`**：把本檔複製到隔離的 repo、以 sed 改掉下方「常數」區塊（鎖檔、sentinel、uid、label 鍵、
信任根），並斷言與正式檔案只差那幾行——⛔ 沒有執行期的覆寫口。
"""
from __future__ import annotations

import ctypes
import fcntl
import hashlib
import json
import os
import re
import secrets
import signal
import stat
import subprocess
import sys
import time
from pathlib import Path

# ── 常數（⚠️ 測試以 sed 改的只有這一段：scripts/test-i074-stage2.sh） ──────────────────
LOCK_PATH = "/run/lock/i074-stage2.lock"
SENTINEL_PATH = "/run/lock/i074-stage2.active"
RUN_UID = 1001
LABEL_KEY = "i074.stage2.run"
TRUSTED_PATH = "/usr/bin:/bin"
GIT = "/usr/bin/git"
DOCKER = "/usr/bin/docker"
PYTHON = "/usr/bin/python3"
BASH = "/bin/bash"
TRUSTED_OWNER_UID = 0
# ── 常數結束 ────────────────────────────────────────────────────────────────────────

INITIAL_UID_MAP = ["0", "0", "4294967295"]

# 結束碼（鏡像；⑦c 才把 8、9 放進 `replay_bundle/publish.py`，屆時由測試斷言相等——「三」#2）
EXIT_ABORT = 1
EXIT_LOOKUP_HIT = 2
EXIT_PROMOTION_FAILED = 8
EXIT_PROMOTION_BLOCKED = 9

TERM_GRACE = 20.0
KILL_GRACE = 5.0
RETRY_SECONDS = 30.0
POLL = 0.1

SENTINEL_SCHEMA = "i074_stage2_active_run/v1"
SENTINEL_FIELDS = ("schema", "token", "boot_id", "uid", "supervisor_pid", "supervisor_start_time", "mode")
MODES = {"run": "orchestrator", "resume": "orchestrator", "promote": "promote"}
SELF_REL = "scripts/lib/i074-stage2-supervisor.py"
ORCH_REL = "scripts/run-i074-stage2.sh"
PR_SET_PDEATHSIG = 1
PR_SET_CHILD_SUBREAPER = 36
READ_LIMIT = 4096

# 環境清理（總綱「四」＋ ⑦b 第一輪 review）：這些前綴與名稱一律不傳給 workload。
ENV_DROP_PREFIXES = ("GIT_", "DOCKER_", "MEM", "SIZING_", "LD_", "PYTHON", "I074_STAGE2_")
ENV_DROP_NAMES = frozenset({"REPLAY_DRY_RUN", "MEASURE_PEAK", "TOOLING_PATCH", "COUNTERFACTUAL_PATCH", "PY_IMAGE",
                            "AFTER_REF", "REPLAY_ARGS_SELFTEST", "CPUS", "I074_SIZING_FAULT", "BASH_ENV", "ENV",
                            "PATH", "SHELLOPTS", "BASHOPTS", "CDPATH", "GLOBIGNORE"})

_HEX64 = re.compile(r"[0-9a-f]{64}")
_UUID = re.compile(r"[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}")
_IMAGE = re.compile(r"sha256:[0-9a-f]{64}")


class Abort(Exception):
    """可控的中止。`kind`：`pre`（1／8）、`sentinel`（1／9）。"""

    def __init__(self, message: str, kind: str = "pre") -> None:
        super().__init__(message)
        self.kind = kind


def exit_code(kind: str, mode: str) -> int:
    if kind == "sentinel":
        return EXIT_PROMOTION_BLOCKED if mode == "promote" else EXIT_ABORT
    return EXIT_PROMOTION_FAILED if mode == "promote" else EXIT_ABORT


def say(msg: str) -> None:
    print(f"i074-stage2-supervisor: {msg}", file=sys.stderr, flush=True)


# ── 信任根（「二之三之一」） ─────────────────────────────────────────────────────────

def read_uid_map(path: str = "/proc/self/uid_map") -> list[str]:
    with open(path, encoding="ascii") as fh:
        return fh.read().split()


def check_uid_map(fields: list[str]) -> None:
    if fields != INITIAL_UID_MAP:
        raise Abort(f"不是初始 user namespace（uid_map={' '.join(fields)!r}）——映射過的 namespace 裡 owner 不可信")


def _dir_ok(st: os.stat_result) -> bool:
    if st.st_uid not in (TRUSTED_OWNER_UID, 0):
        return False
    if st.st_mode & 0o022 == 0:
        return True
    # root 擁有、設了 sticky bit 的共用目錄（例如 /tmp）：別人不能改名或刪掉不屬於他的項目。
    return st.st_uid == 0 and bool(st.st_mode & stat.S_ISVTX)


def check_trusted_program(path: str) -> str:
    """realpath 是 regular、owner ＝ 信任 uid、⛔ group／other 可寫、可執行；每一層上層目錄同樣可信。回傳 realpath。"""
    real = os.path.realpath(path)
    try:
        st = os.stat(real)
    except OSError as exc:
        raise Abort(f"{path}（{real}）讀不到：{exc}") from exc
    # owner：信任 uid 或 root（正式環境兩者都是 0；測試把信任 uid 改成執行帳號時，系統的 /usr/bin 仍是 root）。
    if not stat.S_ISREG(st.st_mode) or st.st_uid not in (TRUSTED_OWNER_UID, 0) or st.st_mode & 0o022 \
            or not os.access(real, os.X_OK):
        raise Abort(f"{path}（{real}）不符合信任條件：owner={st.st_uid} mode={oct(stat.S_IMODE(st.st_mode))}")
    directory = os.path.dirname(real)
    while True:
        try:
            dst = os.stat(directory)
        except OSError as exc:
            raise Abort(f"{directory} 讀不到：{exc}") from exc
        if not _dir_ok(dst):
            raise Abort(f"{path} 的上層目錄 {directory} 不符合信任條件：owner={dst.st_uid} "
                        f"mode={oct(stat.S_IMODE(dst.st_mode))}")
        if directory == "/":
            return real
        directory = os.path.dirname(directory)


def check_trust_root() -> dict[str, str]:
    check_uid_map(read_uid_map())
    return {name: check_trusted_program(path) for name, path in
            (("git", GIT), ("docker", DOCKER), ("python", PYTHON), ("bash", BASH))}


# ── 小工具 ────────────────────────────────────────────────────────────────────────

def clean_env(environ: dict[str, str]) -> dict[str, str]:
    return {k: v for k, v in environ.items()
            if k not in ENV_DROP_NAMES and not k.startswith(ENV_DROP_PREFIXES)}


def run(argv: list[str], env: dict[str, str]) -> subprocess.CompletedProcess:
    return subprocess.run(argv, stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=env, check=False)


def proc_starttime(pid: int) -> int:
    with open(f"/proc/{pid}/stat", encoding="ascii", errors="replace") as fh:
        data = fh.read()
    return int(data[data.rfind(")") + 2:].split()[19])


def boot_id() -> str:
    with open("/proc/sys/kernel/random/boot_id", encoding="ascii") as fh:
        return fh.read().strip()


def canonical(obj: dict) -> bytes:
    """與 `replay_bundle/canonical.py` 同一組參數（本檔只用標準庫，⛔ 不 import repo 模組）。"""
    return json.dumps(obj, sort_keys=True, separators=(",", ":"), ensure_ascii=False, allow_nan=False).encode("utf-8")


def validate_sentinel(obj: object, *, current_boot_id: str) -> None:
    """「八之一之二」第 4 列的封閉 schema。⚠️ 只用於寫入前的自我驗證與診斷——sentinel **一旦存在就拒絕**。"""
    if not isinstance(obj, dict) or set(obj) != set(SENTINEL_FIELDS):
        raise Abort("sentinel 的鍵集合不符")
    if obj["schema"] != SENTINEL_SCHEMA:
        raise Abort("sentinel 的 schema 不符")
    if not (isinstance(obj["token"], str) and _HEX64.fullmatch(obj["token"])):
        raise Abort("sentinel 的 token 不是 64 位小寫 hex")
    if not (isinstance(obj["boot_id"], str) and _UUID.fullmatch(obj["boot_id"])) or obj["boot_id"] != current_boot_id:
        raise Abort("sentinel 的 boot_id 格式不符或不是目前的開機週期")
    if type(obj["uid"]) is not int or obj["uid"] < 0 or obj["uid"] != RUN_UID:
        raise Abort("sentinel 的 uid 不符")
    for key in ("supervisor_pid", "supervisor_start_time"):
        if type(obj[key]) is not int or obj[key] <= 0:
            raise Abort(f"sentinel 的 {key} 必須是正整數")
    if obj["mode"] not in ("orchestrator", "promote"):
        raise Abort("sentinel 的 mode 不符")


def descendants(root: int) -> dict[int, str]:
    """以 `/proc/*/stat` 的 ppid 建樹，回傳 `root` 的全部後代 `{pid: state}`（含 zombie）。"""
    children: dict[int, list[tuple[int, str]]] = {}
    for entry in os.listdir("/proc"):
        if not entry.isdigit():
            continue
        try:
            with open(f"/proc/{entry}/stat", encoding="ascii", errors="replace") as fh:
                data = fh.read()
        except OSError:
            continue
        fields = data[data.rfind(")") + 2:].split()
        children.setdefault(int(fields[1]), []).append((int(entry), fields[0]))
    found: dict[int, str] = {}
    stack = [root]
    while stack:
        for pid, state in children.get(stack.pop(), []):
            if pid not in found:
                found[pid] = state
                stack.append(pid)
    return found


class Supervisor:
    def __init__(self, mode: str, work: str, freeze_record: str | None, environ: dict[str, str]) -> None:
        self.mode = mode
        self.work = work
        self.freeze_record = freeze_record
        self.environ = environ
        self.repo = Path(__file__).resolve().parent.parent.parent
        self.env = clean_env(environ)
        self.env["PATH"] = TRUSTED_PATH
        self.lock_fd: int | None = None
        self.sentinel_fd: int | None = None
        self.removal_pending = False               # sentinel 已 unlink、目錄 fsync 還沒成功——⛔ 放鎖
        self.token = ""
        self.child: subprocess.Popen | None = None
        self.child_status: int | None = None
        self.signal: int | None = None
        self.real: dict[str, str] = {}

    # ── 取鎖之前 ──────────────────────────────────────────────────────────────
    def preflight(self) -> None:
        self.real = check_trust_root()
        self.check_self()
        bad = sorted(k for k in self.environ if k.startswith("I074_STAGE2_"))
        if bad:
            raise Abort(f"傳入的環境已帶 {bad}——協定變數只能由 supervisor 產生")
        if os.getuid() != RUN_UID:
            raise Abort(f"執行帳號 uid={os.getuid()} ≠ 固定的 {RUN_UID}")
        image = self.environ.get("REPLAY_IMAGE_ID", "")
        if not _IMAGE.fullmatch(image):
            raise Abort(f"REPLAY_IMAGE_ID 格式不符：{image!r}")
        if not self.work.startswith("/") or os.path.realpath(self.work) != self.work:
            raise Abort(f"--work-dir 必須是 canonical 絕對路徑：{self.work!r}")

    def check_self(self) -> None:
        own = self.repo / SELF_REL
        head = run([GIT, "-C", str(self.repo), "cat-file", "blob", f"HEAD:{SELF_REL}"], self.env)
        if head.returncode != 0:
            raise Abort(f"supervisor 不在 HEAD 裡（未追蹤？）：{head.stderr.decode(errors='replace').strip()}")
        if hashlib.sha256(head.stdout).digest() != hashlib.sha256(own.read_bytes()).digest():
            raise Abort("supervisor 的內容 ≠ HEAD 中的版本——⛔ 不執行未 commit 或不同版本的 supervisor")

    # ── 鎖（第 2 列） ────────────────────────────────────────────────────────────
    def acquire_lock(self) -> None:
        try:
            fd = os.open(LOCK_PATH, os.O_RDWR | os.O_CREAT | os.O_NOFOLLOW | os.O_CLOEXEC, 0o600)
        except OSError as exc:
            raise Abort(f"開不了鎖檔 {LOCK_PATH}：{exc}") from exc
        self.lock_fd = fd
        st = os.fstat(fd)
        if not stat.S_ISREG(st.st_mode) or st.st_uid != RUN_UID or stat.S_IMODE(st.st_mode) != 0o600 \
                or st.st_nlink != 1:
            raise Abort(f"鎖檔屬性不符：regular={stat.S_ISREG(st.st_mode)} owner={st.st_uid} "
                        f"mode={oct(stat.S_IMODE(st.st_mode))} nlink={st.st_nlink}")
        try:
            fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError as exc:
            raise Abort("鎖已被另一個 supervisor 持有——⛔ 不並行") from exc
        try:
            lst = os.lstat(LOCK_PATH)
        except OSError as exc:
            raise Abort(f"鎖檔路徑消失：{exc}") from exc
        if (lst.st_dev, lst.st_ino) != (st.st_dev, st.st_ino):
            raise Abort("鎖檔路徑在開啟之後被換成別的 inode")

    # ── sentinel（第 3、4、7 列） ───────────────────────────────────────────────
    def check_no_sentinel(self) -> None:
        try:
            os.lstat(SENTINEL_PATH)
        except FileNotFoundError:
            return
        except OSError as exc:
            raise Abort(f"讀不到 sentinel 的狀態：{exc}") from exc
        content = ""
        try:
            fd = os.open(SENTINEL_PATH, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
            try:
                content = os.read(fd, READ_LIMIT).decode("utf-8", errors="replace")
            finally:
                os.close(fd)
        except OSError as exc:
            content = f"（讀不到：{exc}）"
        raise Abort(f"sentinel {SENTINEL_PATH} 已存在——上一趟沒有正常結束；**需要重開機**才解除（⛔ 不得手動刪除）。"
                    f"內容：{content!r}", "sentinel")

    def check_containers(self) -> None:
        image = self.environ["REPLAY_IMAGE_ID"]
        inspect = run([DOCKER, "image", "inspect", "--format", "{{json .Config.Labels}}", image], self.env)
        if inspect.returncode != 0:
            raise Abort(f"docker image inspect {image} 失敗：{inspect.stderr.decode(errors='replace').strip()}")
        try:
            labels = json.loads(inspect.stdout.decode() or "null")
        except ValueError as exc:
            raise Abort(f"image 的 labels 讀不懂：{exc}") from exc
        if labels is not None and not isinstance(labels, dict):
            raise Abort("image 的 labels 讀不懂")
        if labels and LABEL_KEY in labels:
            raise Abort(f"image 的 Config.Labels 含 {LABEL_KEY}——殘留檢查會失去意義")
        ps = run([DOCKER, "ps", "-a", "-q", "--filter", f"label={LABEL_KEY}"], self.env)
        if ps.returncode != 0:
            raise Abort("docker ps 失敗——⛔ 查不到就不能當作沒有殘留")
        if ps.stdout.strip():
            raise Abort(f"有帶 {LABEL_KEY} 的殘留容器：{ps.stdout.decode().split()}")

    def create_sentinel(self) -> None:
        payload = {"schema": SENTINEL_SCHEMA, "token": self.token, "boot_id": boot_id(), "uid": os.getuid(),
                   "supervisor_pid": os.getpid(), "supervisor_start_time": proc_starttime(os.getpid()),
                   "mode": MODES[self.mode]}
        validate_sentinel(payload, current_boot_id=payload["boot_id"])
        fd = os.open(SENTINEL_PATH, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW | os.O_CLOEXEC, 0o600)
        self.sentinel_fd = fd                      # 從這裡起，任何失敗都要走收回
        st = os.fstat(fd)
        if not stat.S_ISREG(st.st_mode) or st.st_uid != RUN_UID or stat.S_IMODE(st.st_mode) != 0o600 \
                or st.st_nlink != 1:
            raise Abort("sentinel 屬性不符")
        raw = canonical(payload)
        written = 0
        while written < len(raw):
            written += os.write(fd, raw[written:])
        os.fsync(fd)
        self.fsync_lock_dir()

    def fsync_lock_dir(self) -> None:
        dfd = os.open(os.path.dirname(SENTINEL_PATH), os.O_RDONLY | os.O_DIRECTORY)
        try:
            os.fsync(dfd)
        finally:
            os.close(dfd)

    def sentinel_same_inode(self) -> bool:
        try:
            fst = os.fstat(self.sentinel_fd)
            lst = os.lstat(SENTINEL_PATH)
        except (OSError, TypeError):
            return False
        return stat.S_ISREG(lst.st_mode) and (fst.st_dev, fst.st_ino) == (lst.st_dev, lst.st_ino)

    def remove_sentinel(self) -> bool:
        """inode 相同才 unlink（第 3 列②③、第 8 列）。⛔ 不符或 unlink 失敗 → 不刪、回 False。

        ⚠️ 刪除的 durability（目錄 fsync）由 `finish_removal()` 負責：unlink 之後到 fsync 成功之前是 `removal_pending`，
        這段期間**⛔ 放鎖**（⑦b 實作第一輪 review：fsync 失敗原本會讓例外一路離開、由 `main()` 的 finally 放鎖）。
        """
        if not self.sentinel_same_inode():
            say(f"⚠️ sentinel 的路徑已不是自己建立的那個 inode——⛔ 不 unlink（{SENTINEL_PATH}）")
            return False
        try:
            os.unlink(SENTINEL_PATH)
        except OSError as exc:
            say(f"⚠️ unlink sentinel 失敗：{exc}——⛔ 放鎖")
            return False
        self.removal_pending = True
        fd, self.sentinel_fd = self.sentinel_fd, None
        try:
            os.close(fd)                           # best-effort：⛔ 妨礙「已 unlink → 等 fsync」的狀態轉移
        except OSError as exc:
            say(f"⚠️ 關閉 sentinel 的 fd 失敗（忽略，繼續等目錄 fsync）：{exc}")
        return True

    def finish_removal(self) -> None:
        """目錄 fsync 成功才算刪掉；失敗就**持鎖**、每 `RETRY_SECONDS` 重試並回報，⛔ 不結束。"""
        while True:
            try:
                self.fsync_lock_dir()
                self.removal_pending = False
                return
            except OSError as exc:
                say(f"⚠️ sentinel 已 unlink，但目錄 fsync 失敗：{exc}——⛔ 放鎖；{RETRY_SECONDS:.0f} 秒後重試")
                time.sleep(RETRY_SECONDS)

    # ── 範圍：後代與容器（第 5、8 列） ─────────────────────────────────────────
    def reap(self) -> None:
        while True:
            try:
                pid, status = os.waitpid(-1, os.WNOHANG)
            except ChildProcessError:
                return
            if pid == 0:
                return
            if self.child is not None and pid == self.child.pid:
                self.child_status = status
                self.child.returncode = os.WEXITSTATUS(status) if os.WIFEXITED(status) else -os.WTERMSIG(status)

    def live_descendants(self) -> list[int]:
        self.reap()
        return sorted(pid for pid, state in descendants(os.getpid()).items() if state != "Z")

    def own_containers(self) -> list[str] | None:
        ps = run([DOCKER, "ps", "-a", "-q", "--filter", f"label={LABEL_KEY}={self.token}"], self.env)
        if ps.returncode != 0:
            return None
        return ps.stdout.decode().split()

    def remove_containers(self) -> bool:
        ids = self.own_containers()
        if ids is None:
            say("⚠️ docker ps 失敗——⛔ 查不到就不能當作沒有")
            return False
        for cid in ids:
            rm = run([DOCKER, "rm", "-f", cid], self.env)
            if rm.returncode != 0:
                say(f"⚠️ 移除不了容器 {cid}：{rm.stderr.decode(errors='replace').strip()}")
        return True

    def terminate_descendants(self) -> bool:
        for sig, grace in ((signal.SIGTERM, TERM_GRACE), (signal.SIGKILL, KILL_GRACE)):
            deadline = time.monotonic() + grace
            signaled: set[int] = set()
            while True:
                live = self.live_descendants()
                if not live:
                    return True
                for pid in live:
                    if pid not in signaled:
                        try:
                            os.kill(pid, sig)
                        except ProcessLookupError:
                            pass
                        signaled.add(pid)
                if time.monotonic() >= deadline:
                    break
                time.sleep(POLL)
            if sig == signal.SIGTERM:
                say(f"⚠️ TERM 之後仍有後代 {self.live_descendants()}——升級 KILL")
        return not self.live_descendants()

    def clean_once(self) -> bool:
        """容器 → 後代 TERM／KILL → **確認**：沒有活著的後代、容器查詢成功且為空。"""
        containers_ok = self.remove_containers()
        procs_ok = self.terminate_descendants()
        remaining = self.own_containers()
        if remaining:
            self.remove_containers()
            remaining = self.own_containers()
        return containers_ok and procs_ok and remaining == []

    def release(self) -> None:
        """第 8 列的正常釋放。⚠️ 清不空、inode 不符、或清理途中**任何**例外 → ⛔ 不刪 sentinel、⛔ 不放鎖，持續回報
        （「三」#10）；sentinel 刪掉之後，目錄 fsync 成功才放鎖。⛔ 例外不得離開本函式（那會由 `main()` 結束、連帶放鎖）。"""
        for sig in (signal.SIGTERM, signal.SIGINT, signal.SIGHUP):
            signal.signal(sig, signal.SIG_IGN)     # 釋放途中不再被打斷
        while True:
            if self.removal_pending:               # 已 unlink：只剩刪除的 durability（⛔ 回頭重做清理與 inode 比對）
                cleaned = True
            else:
                try:
                    cleaned = self.clean_once() and self.remove_sentinel()
                except Exception as exc:  # noqa: BLE001 - 任何例外都等同「清不空」：持鎖、重試
                    say(f"⚠️ 清理途中發生例外：{type(exc).__name__}: {exc}")
                    cleaned = self.removal_pending
            if cleaned:
                self.finish_removal()
                self.close_lock()
                return
            say(f"⚠️ 本趟的容器或後代清不空（或 sentinel 被換掉）——⛔ 不刪 sentinel、⛔ 不放鎖；{RETRY_SECONDS:.0f} 秒後重試")
            time.sleep(RETRY_SECONDS)

    def rollback(self) -> int:
        """第 3 列的收回：workload child 成功啟動之前的可控失敗。回傳 `pre`（刪掉了）或 `sentinel`（留下）。"""
        try:
            self.reap()
            if self.live_descendants() or self.own_containers() != []:
                say("⚠️ 收回失敗：仍有後代或本趟的容器（或 docker ps 失敗）——sentinel 留下，需要重開機")
                return exit_code("sentinel", self.mode)
            if not self.remove_sentinel():
                return exit_code("sentinel", self.mode)
        except Exception as exc:  # noqa: BLE001 - 收回不成立：sentinel 留下（尚未 unlink）
            say(f"⚠️ 收回途中發生例外：{type(exc).__name__}: {exc}——sentinel 留下，需要重開機")
            if not self.removal_pending:
                return exit_code("sentinel", self.mode)
        self.finish_removal()                      # ③ 刪掉、目錄 fsync 成功之後才放鎖
        self.close_lock()
        return exit_code("pre", self.mode)

    # ── 啟動持鎖階段（第 11 列） ─────────────────────────────────────────────────
    def spawn(self) -> None:
        libc = ctypes.CDLL(None, use_errno=True)
        if libc.prctl(PR_SET_CHILD_SUBREAPER, 1, 0, 0, 0) != 0:
            raise Abort(f"設定 child subreaper 失敗：errno={ctypes.get_errno()}")
        me = os.getpid()
        argv = [BASH, "-p", str(self.repo / ORCH_REL), "--internal-locked-stage", self.mode, "--work-dir", self.work]
        if self.freeze_record is not None:
            argv += ["--freeze-record", self.freeze_record]
        env = dict(self.env)
        env.update({"PATH": f"{self.work}/bin:{TRUSTED_PATH}", "PYTHONDONTWRITEBYTECODE": "1",
                    "I074_STAGE2_TOKEN": self.token, "I074_STAGE2_SUPERVISOR_PID": str(me),
                    "I074_STAGE2_SUPERVISOR_START": str(proc_starttime(me)),
                    "I074_STAGE2_REAL_REPO": str(self.repo), "I074_STAGE2_REAL_DOCKER": self.real["docker"],
                    "I074_STAGE2_MODE": MODES[self.mode]})

        def preexec() -> None:
            if libc.prctl(PR_SET_PDEATHSIG, signal.SIGTERM, 0, 0, 0) != 0 or os.getppid() != me:
                os._exit(EXIT_ABORT)
            for sig in (signal.SIGTERM, signal.SIGINT, signal.SIGHUP):
                signal.signal(sig, signal.SIG_DFL)

        self.child = subprocess.Popen(argv, env=env, preexec_fn=preexec, close_fds=True)

    def on_signal(self, signum: int, _frame) -> None:
        if self.signal is None:
            self.signal = signum

    def wait_child(self) -> None:
        while self.child_status is None and self.signal is None:
            self.reap()
            if self.child_status is None and self.signal is None:
                time.sleep(POLL)

    # ── 主流程 ──────────────────────────────────────────────────────────────────
    def close_lock(self) -> None:
        """放鎖（關閉 lock fd）。⚠️ 只在 sentinel 已刪、或從未建立時呼叫——sentinel 留下時放鎖也無妨（它擋住下一個）。"""
        if self.lock_fd is not None:
            os.close(self.lock_fd)
            self.lock_fd = None

    def main(self) -> int:
        try:
            return self._main()
        finally:
            # ⚠️ sentinel 已 unlink 但刪除還沒 durable 時⛔ 放鎖：先把 fsync 做完（`finish_removal()` 失敗會持鎖重試）。
            if self.removal_pending:
                self.finish_removal()
            self.close_lock()

    def _main(self) -> int:
        try:
            self.preflight()
            self.acquire_lock()
        except Abort as exc:
            say(f"中止：{exc}")
            return exit_code(exc.kind, self.mode)
        for sig in (signal.SIGTERM, signal.SIGINT, signal.SIGHUP):
            signal.signal(sig, self.on_signal)
        try:
            self.check_no_sentinel()
            self.check_containers()
        except Abort as exc:
            say(f"中止：{exc}")
            return exit_code(exc.kind, self.mode)
        if self.signal is not None:
            return 128 + self.signal
        self.token = secrets.token_hex(32)
        try:
            self.create_sentinel()
            if self.signal is not None:
                raise Abort(f"收到訊號 {self.signal}")
            self.spawn()
        except (Abort, OSError, subprocess.SubprocessError) as exc:
            say(f"中止（workload child 啟動之前）：{exc}")
            if self.sentinel_fd is None:
                return exit_code("pre", self.mode) if self.signal is None else 128 + self.signal
            rc = self.rollback()
            return 128 + self.signal if self.signal is not None and rc == exit_code("pre", self.mode) else rc
        try:
            self.wait_child()
        finally:
            # ⚠️ 等待途中出現非預期的例外也照樣走第 8 列（⛔ 讓它直接結束：那會放鎖而後代與容器可能還在）。
            received = self.signal
            if received is not None:
                say(f"收到訊號 {received}——移除本趟的容器、結束所有後代")
            self.release()
        if received is not None:
            return 128 + received
        status = self.child_status
        if os.WIFSIGNALED(status):
            return 128 + os.WTERMSIG(status)
        return os.WEXITSTATUS(status)


def parse_args(argv: list[str]) -> tuple[str, str, str | None]:
    if not argv or argv[0] not in MODES:
        raise Abort(f"用法：{SELF_REL} {{run|resume|promote}} --work-dir <dir> [--freeze-record <path>]")
    mode, rest = argv[0], argv[1:]
    values: dict[str, str] = {}
    while rest:
        key = rest[0]
        if key not in ("--work-dir", "--freeze-record") or len(rest) < 2 or key in values:
            raise Abort(f"參數不符：{rest[0]!r}")
        values[key] = rest[1]
        rest = rest[2:]
    if "--work-dir" not in values:
        raise Abort("缺少 --work-dir")
    if (mode == "run") != ("--freeze-record" in values):
        raise Abort("--freeze-record 只屬於 run 模式，而且 run 模式必須有")
    return mode, values["--work-dir"], values.get("--freeze-record")


def main(argv: list[str] | None = None) -> int:
    argv = sys.argv[1:] if argv is None else argv
    try:
        mode, work, freeze = parse_args(argv)
    except Abort as exc:
        say(f"中止：{exc}")
        return exit_code("pre", "promote" if argv[:1] == ["promote"] else "run")
    return Supervisor(mode, work, freeze, dict(os.environ)).main()


if __name__ == "__main__":
    raise SystemExit(main())
