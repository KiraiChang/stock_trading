"""I-074 Stage 2 ⑦b／⑦d：host 端的單元測試（Python 3.9；issue.md I-074「Stage 2 步驟 ⑦b 細部計畫 v1」「五」與
「Stage 2 步驟 ⑦d 細部計畫 v1」「五」）。

由 `scripts/test-i074-stage2.sh` 以 host 的 `python3 -m unittest` 執行（host 沒有 pytest；測試 image 裡沒有 git 與 docker）。
涵蓋 supervisor 的內部函式（sentinel 的封閉 schema、鎖檔與 sentinel 的屬性、收回與正常釋放的 inode 比對、故障注入）、
需要 git 的 freeze record helper，以及三個 host 模組在 3.9 可 import。

⚠️ ⛔ 不碰 `/run/lock`：鎖檔、sentinel 的路徑在**本程序內**改成暫存目錄（⛔ 不是正式檔案的執行期覆寫口）；
會設 child subreaper 或 fork 的測試一律在子程序裡跑。
"""
from __future__ import annotations

import fcntl
import hashlib
import importlib.util
import json
import os
import secrets
import shutil
import signal
import stat
import subprocess
import sys
import tempfile
import textwrap
import time
import unittest
from pathlib import Path
from unittest import mock

ROOT = Path(__file__).resolve().parents[2]
SUPERVISOR = ROOT / "scripts" / "lib" / "i074-stage2-supervisor.py"
sys.path.insert(0, str(ROOT / "python" / "scripts"))


def load_supervisor():
    spec = importlib.util.spec_from_file_location("i074_stage2_supervisor_under_test", SUPERVISOR)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def lock_is_free(path: str) -> bool:
    fd = os.open(path, os.O_RDWR)
    try:
        fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        return True
    except BlockingIOError:
        return False
    finally:
        os.close(fd)


class SupervisorCase(unittest.TestCase):
    def setUp(self) -> None:
        self.sup = load_supervisor()
        self.tmp = tempfile.mkdtemp()
        self.sup.LOCK_PATH = f"{self.tmp}/i074-stage2.lock"
        self.sup.SENTINEL_PATH = f"{self.tmp}/i074-stage2.active"
        self.sup.RUN_UID = os.getuid()
        self.env = {"REPLAY_IMAGE_ID": "sha256:" + "a" * 64, "HOME": "/tmp"}
        # ⚠️ `release()` 會把 TERM／INT／HUP 設成 SIG_IGN（釋放途中不被打斷）——在本程序裡呼叫它的測試結束後要還原，
        #    否則之後 fork 的子程序會繼承「忽略 TERM」（這正是第一次出現 20 秒 TERM→KILL 升級的原因）。
        self.saved_signals = {s: signal.getsignal(s) for s in (signal.SIGTERM, signal.SIGINT, signal.SIGHUP)}
        self.token = "i074h1-" + secrets.token_hex(8)                   # ⑧「四」：起長存程序的測試都帶它
        self.addCleanup(lambda: reap_token_processes(self.token))

    def restore_signals(self) -> None:
        for s, handler in self.saved_signals.items():
            signal.signal(s, handler)

    def tearDown(self) -> None:
        self.restore_signals()
        shutil.rmtree(self.tmp, ignore_errors=True)

    def make(self, mode: str = "run") -> "object":
        s = self.sup.Supervisor(mode, f"{self.tmp}/work", None if mode != "run" else "/x", dict(self.env))
        s.real = {"docker": "/usr/bin/docker"}
        return s


class SentinelSchema(SupervisorCase):
    def good(self) -> dict:
        boot = self.sup.boot_id()
        return {"schema": self.sup.SENTINEL_SCHEMA, "token": "a" * 64, "boot_id": boot, "uid": os.getuid(),
                "supervisor_pid": os.getpid(), "supervisor_start_time": 12345, "mode": "orchestrator"}

    def test_good_passes(self) -> None:
        self.sup.validate_sentinel(self.good(), current_boot_id=self.sup.boot_id())

    def test_each_field_rejected(self) -> None:
        boot = self.sup.boot_id()
        cases = {
            "uid 是 bool": {"uid": True}, "uid 為負": {"uid": -1}, "uid 不是執行帳號": {"uid": os.getuid() + 1},
            "pid 為 0": {"supervisor_pid": 0}, "pid 是 bool": {"supervisor_pid": True},
            "starttime 為 0": {"supervisor_start_time": 0}, "starttime 是字串": {"supervisor_start_time": "1"},
            "boot_id 大寫": {"boot_id": boot.upper()}, "boot_id 格式錯": {"boot_id": "x" * 36},
            "boot_id 不是目前的": {"boot_id": "00000000-0000-0000-0000-000000000000"},
            "token 大寫": {"token": "A" * 64}, "token 太短": {"token": "a" * 63},
            "schema 不符": {"schema": "x/v1"}, "mode 不符": {"mode": "run"},
        }
        for label, override in cases.items():
            with self.subTest(label):
                with self.assertRaises(self.sup.Abort):
                    self.sup.validate_sentinel(dict(self.good(), **override), current_boot_id=boot)
        extra = dict(self.good(), extra=1)
        missing = dict(self.good())
        del missing["mode"]
        for label, payload in (("多欄", extra), ("缺欄", missing), ("不是 object", [])):
            with self.subTest(label):
                with self.assertRaises(self.sup.Abort):
                    self.sup.validate_sentinel(payload, current_boot_id=boot)


class TrustRoot(SupervisorCase):
    def test_uid_map(self) -> None:
        self.sup.check_uid_map(["0", "0", "4294967295"])
        for fields in (["0", "1001", "1"], ["0", "0", "1"], []):
            with self.subTest(fields):
                with self.assertRaises(self.sup.Abort):
                    self.sup.check_uid_map(fields)

    def test_real_system_programs_are_trusted(self) -> None:
        for prog in ("/usr/bin/git", "/usr/bin/docker", "/usr/bin/python3", "/bin/bash"):
            with self.subTest(prog):
                self.sup.check_trusted_program(prog)

    def test_conditions(self) -> None:
        self.sup.TRUSTED_OWNER_UID = os.getuid()
        d = Path(self.tmp) / "bin"
        d.mkdir(mode=0o755)
        prog = d / "tool"
        prog.write_text("#!/bin/sh\n")
        prog.chmod(0o755)
        self.sup.check_trusted_program(str(prog))
        for label, setup, undo in (
            ("檔案 group 可寫", lambda: prog.chmod(0o775), lambda: prog.chmod(0o755)),
            ("檔案不可執行", lambda: prog.chmod(0o644), lambda: prog.chmod(0o755)),
            ("上層目錄 other 可寫（非 sticky root）", lambda: d.chmod(0o757), lambda: d.chmod(0o755)),
        ):
            with self.subTest(label):
                setup()
                try:
                    with self.assertRaises(self.sup.Abort):
                        self.sup.check_trusted_program(str(prog))
                finally:
                    undo()
        with self.assertRaises(self.sup.Abort):
            self.sup.check_trusted_program(str(d))          # 非 regular
        self.sup.TRUSTED_OWNER_UID = os.getuid() + 12345
        link = Path(self.tmp) / "owned-by-me"
        link.write_text("x")
        link.chmod(0o755)
        with self.assertRaises(self.sup.Abort):
            self.sup.check_trusted_program(str(link))       # owner 不是信任 uid 也不是 root


class Locking(SupervisorCase):
    def test_acquire_creates_0600_and_conflict(self) -> None:
        a = self.make()
        a.acquire_lock()
        st = os.stat(self.sup.LOCK_PATH)
        self.assertEqual(stat.S_IMODE(st.st_mode), 0o600)
        b = self.make()
        with self.assertRaises(self.sup.Abort):
            b.acquire_lock()
        os.close(a.lock_fd)
        self.assertTrue(lock_is_free(self.sup.LOCK_PATH))

    def test_attribute_failures(self) -> None:
        other = Path(self.tmp) / "other"
        other.write_text("x")
        other.chmod(0o600)
        cases = {
            "symlink": lambda p: os.symlink(other, p),
            "目錄": lambda p: os.mkdir(p),
            "mode 0644": lambda p: (Path(p).write_text(""), os.chmod(p, 0o644)),
            "link count 2": lambda p: os.link(other, p),
        }
        for label, setup in cases.items():
            with self.subTest(label):
                path = self.sup.LOCK_PATH
                if os.path.lexists(path):
                    shutil.rmtree(path) if os.path.isdir(path) and not os.path.islink(path) else os.unlink(path)
                setup(path)
                with self.assertRaises(self.sup.Abort):
                    self.make().acquire_lock()

    def test_inode_swapped_after_open(self) -> None:
        real_lstat = os.lstat
        decoy = Path(self.tmp) / "decoy"
        decoy.write_text("")

        def fake_lstat(path, *args, **kwargs):
            if str(path) == self.sup.LOCK_PATH:
                return real_lstat(decoy)
            return real_lstat(path, *args, **kwargs)

        with mock.patch.object(self.sup.os, "lstat", side_effect=fake_lstat):
            with self.assertRaises(self.sup.Abort):
                self.make().acquire_lock()
        self.assertTrue(os.path.exists(self.sup.LOCK_PATH))     # ⛔ 永不 unlink

    def test_uid_mismatch_and_env_and_image(self) -> None:
        for label, change, mode, want in (
            ("非固定帳號（run）", lambda s: setattr(self.sup, "RUN_UID", os.getuid() + 1), "run", 1),
            ("非固定帳號（promote）", lambda s: setattr(self.sup, "RUN_UID", os.getuid() + 1), "promote", 8),
            ("環境帶 I074_STAGE2_*", lambda s: s.environ.update(I074_STAGE2_TOKEN="x"), "run", 1),
            ("REPLAY_IMAGE_ID 格式錯", lambda s: s.environ.update(REPLAY_IMAGE_ID="--help"), "promote", 8),
        ):
            with self.subTest(label):
                self.sup.RUN_UID = os.getuid()
                s = self.make(mode)
                change(s)
                with mock.patch.object(s, "check_self"), mock.patch.object(self.sup, "check_trust_root", return_value={}):
                    self.assertEqual(s.main(), want)
                self.assertFalse(os.path.exists(self.sup.LOCK_PATH))   # 取鎖之前就中止
                self.assertFalse(os.path.exists(self.sup.SENTINEL_PATH))


class SentinelLifecycle(SupervisorCase):
    def prepared(self, mode: str = "run"):
        s = self.make(mode)
        s.acquire_lock()
        s.token = "b" * 64
        return s

    def test_existing_sentinel_blocks(self) -> None:
        Path(self.sup.SENTINEL_PATH).write_text("garbage")
        s = self.prepared()
        with self.assertRaises(self.sup.Abort) as cm:
            s.check_no_sentinel()
        self.assertEqual(cm.exception.kind, "sentinel")
        self.assertIn("需要重開機", str(cm.exception))
        self.assertEqual(Path(self.sup.SENTINEL_PATH).read_text(), "garbage")

    def _blocked_and_untouched(self, label: str) -> None:
        """sentinel 已存在（任何型別、任何內容）→ `check_no_sentinel()` 以 `sentinel` 拒絕、⛔ 不動那個項目。"""
        path = self.sup.SENTINEL_PATH
        before = os.lstat(path)
        content = None if not stat.S_ISREG(before.st_mode) else Path(path).read_bytes()
        with self.assertRaises(self.sup.Abort, msg=label) as cm:
            self.make().check_no_sentinel()
        self.assertEqual(cm.exception.kind, "sentinel", label)
        after = os.lstat(path)
        self.assertEqual((after.st_ino, after.st_mode), (before.st_ino, before.st_mode), label)
        if content is not None:
            self.assertEqual(Path(path).read_bytes(), content, label)

    def _clear_sentinel(self) -> None:
        path = self.sup.SENTINEL_PATH
        if os.path.isdir(path) and not os.path.islink(path):
            os.rmdir(path)
        elif os.path.lexists(path):
            os.unlink(path)

    def test_existing_sentinel_of_any_type_blocks(self) -> None:
        """n7b（⑧ 補齊）：sentinel 的屬性——預先建成 dangling symlink、指向檔案的 symlink、目錄、FIFO → 一律拒絕。"""
        other = Path(self.tmp) / "other"
        other.write_text("x")
        for label, setup in (("dangling symlink", lambda p: os.symlink(f"{self.tmp}/missing", p)),
                             ("symlink 指向檔案", lambda p: os.symlink(other, p)),
                             ("目錄", lambda p: os.mkdir(p)),
                             ("FIFO", lambda p: os.mkfifo(p))):
            with self.subTest(label):
                self._clear_sentinel()
                setup(self.sup.SENTINEL_PATH)
                self._blocked_and_untouched(label)
        self._clear_sentinel()

    def test_unreadable_sentinel_contents_block(self) -> None:
        """n7b（⑧ 補齊）：sentinel 讀不懂——空檔、部分寫入（截斷的 canonical JSON）、schema 不符的合法 JSON → 照樣拒絕、內容不變。"""
        payload = {"schema": self.sup.SENTINEL_SCHEMA, "token": "a" * 64, "boot_id": self.sup.boot_id(), "uid": os.getuid(),
                   "supervisor_pid": os.getpid(), "supervisor_start_time": 12345, "mode": "orchestrator"}
        raw = self.sup.canonical(payload)
        for label, data in (("空檔", b""), ("部分寫入", raw[: len(raw) // 2]),
                            ("schema 不符", self.sup.canonical(dict(payload, schema="x/v1")))):
            with self.subTest(label):
                self._clear_sentinel()
                Path(self.sup.SENTINEL_PATH).write_bytes(data)
                self._blocked_and_untouched(label)
        self._clear_sentinel()

    def test_create_is_canonical_and_removal_checks_inode(self) -> None:
        s = self.prepared()
        s.create_sentinel()
        raw = Path(self.sup.SENTINEL_PATH).read_bytes()
        payload = json.loads(raw)
        self.assertEqual(raw, self.sup.canonical(payload))
        self.sup.validate_sentinel(payload, current_boot_id=self.sup.boot_id())
        self.assertEqual(stat.S_IMODE(os.stat(self.sup.SENTINEL_PATH).st_mode), 0o600)
        swapped = Path(self.tmp) / "swap"
        swapped.write_text("someone else")
        os.replace(swapped, self.sup.SENTINEL_PATH)          # 路徑被換成別的 inode
        self.assertFalse(s.remove_sentinel())
        self.assertEqual(Path(self.sup.SENTINEL_PATH).read_text(), "someone else")   # ⛔ 不 unlink

    def run_main_with(self, mode: str, **patches) -> int:
        s = self.make(mode)
        base = {"preflight": mock.DEFAULT, "check_containers": mock.DEFAULT}
        with mock.patch.multiple(s, **base), \
                mock.patch.object(s, "own_containers", patches.pop("own_containers", mock.Mock(return_value=[]))), \
                mock.patch.object(s, "spawn", patches.pop("spawn", mock.Mock(side_effect=OSError("exec 失敗")))):
            ctx = patches.pop("ctx", None)
            if ctx is not None:
                with ctx:
                    return s.main()
            return s.main()

    def test_rollback_after_spawn_failure(self) -> None:
        for mode, want in (("run", 1), ("promote", 8)):
            with self.subTest(mode):
                self.assertEqual(self.run_main_with(mode), want)
                self.assertFalse(os.path.exists(self.sup.SENTINEL_PATH))
                self.assertTrue(lock_is_free(self.sup.LOCK_PATH))

    def test_rollback_after_fsync_failure(self) -> None:
        real_fsync = os.fsync
        calls = {"n": 0}

        def failing_fsync(fd):
            calls["n"] += 1
            if calls["n"] == 1:
                raise OSError("fsync 失敗（注入）")
            return real_fsync(fd)

        rc = self.run_main_with("run", ctx=mock.patch.object(self.sup.os, "fsync", side_effect=failing_fsync))
        self.assertEqual(rc, 1)
        self.assertFalse(os.path.exists(self.sup.SENTINEL_PATH))
        self.assertTrue(lock_is_free(self.sup.LOCK_PATH))

    def test_rollback_refuses_when_inode_swapped(self) -> None:
        def swap_and_fail():
            other = Path(self.tmp) / "other"
            other.write_text("not ours")
            os.replace(other, self.sup.SENTINEL_PATH)
            raise OSError("exec 失敗")

        for mode, want in (("promote", 9), ("run", 1)):
            with self.subTest(mode):
                if os.path.exists(self.sup.SENTINEL_PATH):
                    os.unlink(self.sup.SENTINEL_PATH)
                self.assertEqual(self.run_main_with(mode, spawn=mock.Mock(side_effect=swap_and_fail)), want)
                self.assertEqual(Path(self.sup.SENTINEL_PATH).read_text(), "not ours")

    def test_rollback_keeps_sentinel_when_containers_unknown(self) -> None:
        rc = self.run_main_with("promote", own_containers=mock.Mock(return_value=None))
        self.assertEqual(rc, 9)
        self.assertTrue(os.path.exists(self.sup.SENTINEL_PATH))

    def test_release_keeps_lock_when_inode_swapped(self) -> None:
        s = self.prepared()
        s.create_sentinel()
        other = Path(self.tmp) / "other"
        other.write_text("not ours")
        os.replace(other, self.sup.SENTINEL_PATH)

        class Stop(Exception):
            pass

        with mock.patch.object(s, "own_containers", return_value=[]), \
                mock.patch.object(self.sup.time, "sleep", side_effect=Stop):
            with self.assertRaises(Stop):
                s.release()
        self.assertEqual(Path(self.sup.SENTINEL_PATH).read_text(), "not ours")
        self.assertFalse(lock_is_free(self.sup.LOCK_PATH))      # ⛔ 不放鎖

    def test_release_keeps_everything_when_docker_ps_fails(self) -> None:
        s = self.prepared()
        s.create_sentinel()

        class Stop(Exception):
            pass

        with mock.patch.object(s, "own_containers", return_value=None), \
                mock.patch.object(self.sup.time, "sleep", side_effect=Stop):
            with self.assertRaises(Stop):
                s.release()
        self.assertTrue(os.path.exists(self.sup.SENTINEL_PATH))
        self.assertFalse(lock_is_free(self.sup.LOCK_PATH))


class DurableRemoval(SupervisorCase):
    """⑦b 實作第一輪 review（高）：sentinel 刪掉之後、目錄 fsync 成功之前，⛔ 放鎖；清理途中的任何 OSError 也⛔ 讓 supervisor 結束。"""

    def prepared(self, mode: str = "run"):
        s = self.make(mode)
        s.acquire_lock()
        s.token = "c" * 64
        s.create_sentinel()                       # 建立時的 fsync 照常成功
        return s

    def flaky(self, s, failures: int):
        real = s.fsync_lock_dir
        calls = {"n": 0}

        def fsync_dir():
            calls["n"] += 1
            if calls["n"] <= failures:
                raise OSError("EIO（注入：刪除 sentinel 之後的目錄 fsync）")
            real()
        return fsync_dir

    def observing_sleep(self, states, limit: int = 10):
        """記錄每次重試時 sentinel 與鎖的狀態；超過 `limit` 次就失敗（⛔ 讓卡在錯誤分支的 supervisor 把測試掛住）。"""
        def sleep(_seconds):
            states.append((os.path.exists(self.sup.SENTINEL_PATH), lock_is_free(self.sup.LOCK_PATH)))
            if len(states) > limit:
                raise AssertionError(f"重試超過 {limit} 次——疑似卡在錯誤的分支：{states[-3:]}")
        return sleep

    def test_release_holds_lock_until_the_removal_is_durable(self) -> None:
        s = self.prepared()
        states: list = []
        with mock.patch.object(s, "own_containers", return_value=[]), \
                mock.patch.object(s, "fsync_lock_dir", side_effect=self.flaky(s, 2)), \
                mock.patch.object(self.sup.time, "sleep", side_effect=self.observing_sleep(states)):
            s.release()
        self.assertEqual(states, [(False, False), (False, False)])     # sentinel 已刪，但鎖一直持有
        self.assertFalse(os.path.exists(self.sup.SENTINEL_PATH))
        self.assertTrue(lock_is_free(self.sup.LOCK_PATH))

    def test_release_survives_arbitrary_oserror_in_cleanup(self) -> None:
        s = self.prepared()

        class Stop(Exception):
            pass

        states: list = []

        def sleep(_seconds):
            states.append((os.path.exists(self.sup.SENTINEL_PATH), lock_is_free(self.sup.LOCK_PATH)))
            raise Stop

        with mock.patch.object(s, "own_containers", side_effect=OSError("docker 不見了")), \
                mock.patch.object(self.sup.time, "sleep", side_effect=sleep):
            with self.assertRaises(Stop):
                s.release()
        self.assertEqual(states, [(True, False)])                        # ⛔ 刪 sentinel、⛔ 放鎖、⛔ 結束

    def test_sentinel_fd_close_failure_does_not_strand_the_removal(self) -> None:
        """⑦b 實作第二輪 review（低）：unlink 之後關 fd 失敗，⛔ 卡在「回頭重做 inode 比對」的分支、目錄 fsync 照樣重試到成功。"""
        for cleanup in ("release", "rollback"):
            with self.subTest(cleanup):
                if os.path.exists(self.sup.SENTINEL_PATH):
                    os.unlink(self.sup.SENTINEL_PATH)
                s = self.prepared()
                sentinel_fd = s.sentinel_fd
                real_close = os.close
                injected = {"done": False}

                def close(fd):                    # 只注入一次（之後同一個 fd 號碼可能被別的檔案重用）
                    if fd == sentinel_fd and not injected["done"]:
                        injected["done"] = True
                        real_close(fd)
                        raise OSError("EBADF（注入：關閉 sentinel 的 fd）")
                    return real_close(fd)

                states: list = []
                with mock.patch.object(s, "own_containers", return_value=[]), \
                        mock.patch.object(s, "fsync_lock_dir", side_effect=self.flaky(s, 1)), \
                        mock.patch.object(self.sup.os, "close", side_effect=close), \
                        mock.patch.object(self.sup.time, "sleep", side_effect=self.observing_sleep(states)):
                    if cleanup == "release":
                        s.release()
                    else:
                        self.assertEqual(s.rollback(), 1)
                self.assertEqual(states, [(False, False)])                 # fsync 重試期間持鎖
                self.assertFalse(s.removal_pending)
                self.assertFalse(os.path.exists(self.sup.SENTINEL_PATH))
                self.assertTrue(lock_is_free(self.sup.LOCK_PATH))
                self.restore_signals()

    def test_rollback_holds_lock_until_the_removal_is_durable(self) -> None:
        s = self.prepared()
        states: list = []
        with mock.patch.object(s, "own_containers", return_value=[]), \
                mock.patch.object(s, "fsync_lock_dir", side_effect=self.flaky(s, 1)), \
                mock.patch.object(self.sup.time, "sleep", side_effect=self.observing_sleep(states)):
            self.assertEqual(s.rollback(), 1)
        self.assertEqual(states, [(False, False)])
        self.assertTrue(lock_is_free(self.sup.LOCK_PATH))

    def test_main_never_releases_the_lock_while_the_removal_is_pending(self) -> None:
        s = self.make("promote")
        real_fsync_dir = s.fsync_lock_dir
        calls = {"n": 0}

        def fsync_dir():                          # 第 1 次（建立）成功；第 2、3 次（刪除之後）失敗
            calls["n"] += 1
            if calls["n"] in (2, 3):
                raise OSError("EIO（注入）")
            real_fsync_dir()

        states: list = []
        with mock.patch.multiple(s, preflight=mock.DEFAULT, check_containers=mock.DEFAULT), \
                mock.patch.object(s, "own_containers", return_value=[]), \
                mock.patch.object(s, "spawn", side_effect=OSError("exec 失敗")), \
                mock.patch.object(s, "fsync_lock_dir", side_effect=fsync_dir), \
                mock.patch.object(self.sup.time, "sleep", side_effect=self.observing_sleep(states)):
            self.assertEqual(s.main(), 8)
        self.assertEqual(states, [(False, False), (False, False)])
        self.assertTrue(lock_is_free(self.sup.LOCK_PATH))


class ForkedPaths(SupervisorCase):
    """設 subreaper、真的 fork 的路徑：在子程序裡跑（⛔ 讓測試程序本身變成 subreaper）。"""

    def run_child(self, body: str) -> dict:
        code = textwrap.dedent(f"""
            import importlib.util, json, os, sys
            from unittest import mock
            spec = importlib.util.spec_from_file_location("sup", {str(SUPERVISOR)!r})
            sup = importlib.util.module_from_spec(spec); spec.loader.exec_module(sup)
            sup.LOCK_PATH = {self.sup.LOCK_PATH!r}; sup.SENTINEL_PATH = {self.sup.SENTINEL_PATH!r}
            sup.RUN_UID = os.getuid()
            TOKEN = {self.token!r}
            s = sup.Supervisor("run", {self.tmp + "/work"!r}, "/x", {{"REPLAY_IMAGE_ID": "sha256:" + "a" * 64}})
            s.real = {{"docker": "/usr/bin/docker"}}
        """) + textwrap.dedent(body)
        out = subprocess.run([sys.executable, "-c", code], stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=False,
                             env={"PATH": "/usr/bin:/bin", "PYTHONDONTWRITEBYTECODE": "1"})
        self.assertEqual(out.returncode, 0, out.stderr.decode())
        return json.loads(out.stdout.decode().strip().splitlines()[-1])

    def test_fork_succeeds_but_exec_fails(self) -> None:
        result = self.run_child("""
            sup.BASH = "/nonexistent/bash"
            with mock.patch.object(s, "preflight"), mock.patch.object(s, "check_containers"), \\
                    mock.patch.object(s, "own_containers", return_value=[]):
                rc = s.main()
            print(json.dumps({"rc": rc, "sentinel": os.path.exists(sup.SENTINEL_PATH)}))
        """)
        self.assertEqual(result, {"rc": 1, "sentinel": False})
        self.assertTrue(lock_is_free(self.sup.LOCK_PATH))

    def test_descendants_include_setsid_orphans(self) -> None:
        result = self.run_child("""
            import ctypes, subprocess, time
            libc = ctypes.CDLL(None, use_errno=True)
            assert libc.prctl(sup.PR_SET_CHILD_SUBREAPER, 1, 0, 0, 0) == 0
            p = subprocess.Popen(["/bin/sh", "-c", 'setsid "$1" -c "import time; time.sleep(30)" "$0" & exit 0', TOKEN, sys.executable])
            p.wait()
            time.sleep(0.3)
            live = s.live_descendants()
            ok = s.terminate_descendants()
            print(json.dumps({"orphans": len(live), "cleaned": ok, "after": s.live_descendants()}))
        """)
        self.assertEqual(result["orphans"], 1)
        self.assertTrue(result["cleaned"])
        self.assertEqual(result["after"], [])


class Misc(SupervisorCase):
    def test_clean_env(self) -> None:
        env = {"GIT_DIR": "x", "DOCKER_HOST": "x", "MEM": "1", "MEM_FORCE": "1", "SIZING_X": "1", "LD_PRELOAD": "x",
               "PYTHONPATH": "x", "I074_STAGE2_TOKEN": "x", "BASH_ENV": "x", "ENV": "x", "PATH": "/evil",
               "TOOLING_PATCH": "x", "COUNTERFACTUAL_PATCH": "x", "PY_IMAGE": "x", "AFTER_REF": "x",
               "REPLAY_DRY_RUN": "1", "MEASURE_PEAK": "1", "REPLAY_ARGS_SELFTEST": "1", "CPUS": "1",
               "I074_SIZING_FAULT": "x", "HOME": "/h", "XDG_DATA_HOME": "/x", "REPLAY_IMAGE_ID": "i"}
        self.assertEqual(self.sup.clean_env(env), {"HOME": "/h", "XDG_DATA_HOME": "/x", "REPLAY_IMAGE_ID": "i"})

    def test_exit_codes(self) -> None:
        self.assertEqual((self.sup.EXIT_ABORT, self.sup.EXIT_LOOKUP_HIT, self.sup.EXIT_PROMOTION_FAILED,
                          self.sup.EXIT_PROMOTION_BLOCKED), (1, 2, 8, 9))
        self.assertEqual([self.sup.exit_code(k, m) for k in ("pre", "sentinel") for m in ("run", "promote")], [1, 8, 1, 9])

    def test_parse_args_has_no_unlock_entry(self) -> None:
        for argv in (["unlock"], ["run", "--work-dir", "/w"], ["promote", "--work-dir", "/w", "--freeze-record", "/f"],
                     ["run", "--work-dir", "/w", "--freeze-record", "/f", "--work-dir", "/x"]):
            with self.subTest(argv):
                with self.assertRaises(self.sup.Abort):
                    self.sup.parse_args(argv)
        self.assertEqual(self.sup.parse_args(["resume", "--work-dir", "/w"]), ("resume", "/w", None))


class FreezeRecordGit(unittest.TestCase):
    def test_head_file_sha256_is_content_not_blob_oid(self) -> None:
        import i074_stage2_freeze_record as fr

        tmp = tempfile.mkdtemp()
        try:
            env = {"PATH": "/usr/bin:/bin", "HOME": tmp}
            subprocess.run(["git", "init", "-q", tmp], check=True, env=env)
            (Path(tmp) / "f.txt").write_bytes(b"hello\n")
            subprocess.run(["git", "-C", tmp, "add", "f.txt"], check=True, env=env)
            subprocess.run(["git", "-C", tmp, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qm", "x"],
                           check=True, env=env)
            head = subprocess.run(["git", "-C", tmp, "rev-parse", "HEAD"], stdout=subprocess.PIPE, check=True,
                                  env=env).stdout.decode().strip()
            blob = subprocess.run(["git", "-C", tmp, "rev-parse", "HEAD:f.txt"], stdout=subprocess.PIPE, check=True,
                                  env=env).stdout.decode().strip()
            got = fr.head_file_sha256(tmp, head, "f.txt")
            self.assertEqual(got, hashlib.sha256(b"hello\n").hexdigest())
            self.assertNotEqual(got[:40], blob)
            with self.assertRaises(fr.FreezeRecordError):
                fr.head_file_sha256(tmp, head, "missing.txt")
        finally:
            shutil.rmtree(tmp, ignore_errors=True)


class HostImports(unittest.TestCase):
    def test_modules_import_on_host_python(self) -> None:
        import i074_stage2_freeze_record as fr
        import i074_stage2_preflight as pf

        self.assertEqual(pf.REQUIRED_BYTES, 1_241_513_984)
        self.assertEqual(fr.P_B_BUDGET, pf.P_B_BUDGET)
        self.assertEqual(sys.version_info[:2] >= (3, 9), True)
        load_supervisor()


# ── ⑦d：acceptance 的 host 端（issue.md I-074「Stage 2 步驟 ⑦d 細部計畫 v1」「五」的 ac9、ac12b、ac13、ac16） ──────────

SIZING = ROOT / "python" / "scripts" / "i074_stage2_sizing.py"
MIB = 1024 * 1024


def load_sizing():
    spec = importlib.util.spec_from_file_location("i074_stage2_sizing_under_test", SIZING)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class AcceptanceHost(unittest.TestCase):
    def setUp(self) -> None:
        self.sz = load_sizing()
        self.tmp = Path(tempfile.mkdtemp(prefix="i074-acc-host-"))
        self.addCleanup(shutil.rmtree, self.tmp, True)
        self.token = "i074h1-" + secrets.token_hex(8)                   # ⑧「四」
        self.addCleanup(lambda: reap_token_processes(self.token))

    def test_clean_env_is_the_supervisors(self) -> None:
        """ac9：clean-env 就是 supervisor 的 clean_env()（唯一定義），⛔ 不另抄清單。"""
        env = {"HOME": "/h", "PATH": "/x", "GIT_DIR": "g", "SIZING_X": "1", "TOOLING_PATCH": "t", "XDG_DATA_HOME": "/d",
               "PYTHONPATH": "p", "MEASURE_PEAK": "1", "LANG": "C"}
        self.assertEqual(self.sz.clone_clean_env(ROOT, env), load_supervisor().clean_env(env))
        self.assertEqual(set(self.sz.clone_clean_env(ROOT, env)), {"HOME", "XDG_DATA_HOME", "LANG"})

    def test_promotion_states_use_the_preflight_algorithm(self) -> None:
        """ac13：晉升讀的五個欄位；identity_sha256 ＝ orchestrator 寫 preflight.json 時的算法。"""
        identity = self.tmp / "run_identity.json"
        identity.write_text('{"x": 1}\n')
        states = self.sz.promotion_states(ROOT, identity=identity, bundle_id="b1", semantic="s" * 64, repo_head="a" * 40, rc=6)
        spec = importlib.util.spec_from_file_location("pf_under_test", ROOT / "python" / "scripts" / "i074_stage2_preflight.py")
        pf = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(pf)
        self.assertEqual(states, {
            "preflight": {"bundle_id": "b1", "counterfactual_semantic_sha256": "s" * 64,
                          "identity_sha256": pf._sha256_file(identity)},
            "replay_done": {"rc": 6}, "run": {"repo_head": "a" * 40}})
        self.assertEqual(states["preflight"]["identity_sha256"], hashlib.sha256(identity.read_bytes()).hexdigest())
        with self.assertRaises(self.sz.SizingError):
            self.sz.promotion_states(ROOT, identity=identity, bundle_id="b1", semantic="s" * 64, repo_head="a" * 40, rc=7)

    def _host_run(self, code: str, step: str = "s") -> tuple[int, dict]:
        """在自己的 session 裡跑 host-run（它是 process group 的 leader，與 harness 的 run_in_group 相同）。"""
        state = self.tmp / "S"
        proc = subprocess.run([sys.executable, str(SIZING), "host-run", "--state", str(state), "--step", step, "--",
                               sys.executable, "-c", code], start_new_session=True,
                              env=dict(os.environ, PYTHONDONTWRITEBYTECODE="1"), timeout=120)
        return proc.returncode, json.loads((state / "host" / f"{step}.json").read_text())

    def test_host_run_max_single_is_the_largest_descendant(self) -> None:
        """ac16：最大單一 RSS 取自 RUSAGE_CHILDREN——孫程序較大時取到孫程序；結束碼原樣傳回。"""
        code = textwrap.dedent("""
            import subprocess, sys
            small = bytearray(24 * 1024 * 1024)
            rc = subprocess.run([sys.executable, "-c", "x = bytearray(64 * 1024 * 1024); import time; time.sleep(0.5)"]).returncode
            sys.exit(3 if rc == 0 else 9)
        """)
        rc, rec = self._host_run(code)
        self.assertEqual((rc, rec["rc"]), (3, 3))
        self.assertGreaterEqual(rec["max_single_rss_bytes"], 64 * MIB)
        self.assertLess(rec["max_single_rss_bytes"], 64 * MIB + 40 * MIB)
        self.assertIn("PATH", rec["env_keys"])
        self.assertEqual(rec["cmd"][0], sys.executable)

    def test_host_run_group_sum_adds_concurrent_processes(self) -> None:
        """ac16：兩個同時存活的子程序 → 群組的取樣總和 ≥ 兩者之和、最大單一 ＝ 較大的那一個。"""
        code = textwrap.dedent("""
            import subprocess, sys, time
            a = subprocess.Popen([sys.executable, "-c", "x = bytearray(40 * 1024 * 1024); import time; time.sleep(1.5)"])
            b = subprocess.Popen([sys.executable, "-c", "x = bytearray(56 * 1024 * 1024); import time; time.sleep(1.5)"])
            sys.exit(a.wait() | b.wait())
        """)
        rc, rec = self._host_run(code)
        self.assertEqual(rc, 0)
        self.assertGreaterEqual(rec["group_rss_peak_sampled_bytes"], (40 + 56) * MIB)
        self.assertGreaterEqual(rec["max_single_rss_bytes"], 56 * MIB)
        self.assertLess(rec["max_single_rss_bytes"], (40 + 56) * MIB)

    def test_host_run_ends_with_its_group(self) -> None:
        """ac16：harness 對整個 process group 送 TERM → host-run 與它的子程序一起結束。"""
        state = self.tmp / "S2"
        proc = subprocess.Popen([sys.executable, str(SIZING), "host-run", "--state", str(state), "--step", "t", "--",
                                 *_sleeper(self.token)], start_new_session=True, env=dict(os.environ, PYTHONDONTWRITEBYTECODE="1"))
        child = None
        for _ in range(100):
            kids = [p for p in Path("/proc").iterdir() if p.name.isdigit()
                    and (p / "stat").exists() and self._ppid(p) == proc.pid]
            if kids:
                child = int(kids[0].name)
                break
            subprocess.run(["sleep", "0.05"])
        self.assertIsNotNone(child)
        os.killpg(proc.pid, signal.SIGTERM)
        proc.wait(timeout=10)
        for _ in range(100):
            if not Path(f"/proc/{child}").exists():
                break
            subprocess.run(["sleep", "0.05"])
        self.assertFalse(Path(f"/proc/{child}").exists())

    @staticmethod
    def _ppid(p: Path) -> int:
        try:
            return int((p / "stat").read_text().rsplit(")", 1)[1].split()[1])
        except (OSError, IndexError, ValueError):
            return -1

    def test_observer_runs_on_host_python(self) -> None:
        """ac12：observer 的核心在 host 3.9 可用（fake docker ＋ 可覆寫的 cgroup 根目錄）。"""
        d = self.tmp / "d"
        d.mkdir()
        cid = "a" * 64
        docker = d / "docker"
        docker.write_text(textwrap.dedent(f"""\
            #!/bin/sh
            case "$1" in
              ps) n=$(cat {d}/n 2>/dev/null || echo 0); echo $((n + 1)) > {d}/n; [ "$n" = 1 ] && echo {cid}; exit 0 ;;
              inspect) echo '["python", "-m", "backtest.modular.sr_scoring.evaluation"]' ;;
            esac
        """))
        docker.chmod(0o755)
        cg = self.tmp / "cg" / "memory" / "docker" / cid
        cg.mkdir(parents=True)
        (cg / "memory.max_usage_in_bytes").write_text("123456789")
        obs = self.sz.Observer(self.tmp / "work", docker=str(docker), cgroup_root=self.tmp / "cg", fs_path=str(self.tmp))
        for t in (1.0, 2.0):
            obs.poll_containers(t)
        obs.read_peaks(2.5)
        obs.poll_containers(3.0)
        report = obs.report()
        self.assertEqual(report["containers"][0]["peak_bytes"], 123456789)
        self.assertTrue(report["replay_seen"])
        self.assertFalse(report["observation_complete"])          # 只看到 replay：其他預期的角色缺席
        self.assertEqual(report["missing_expected_containers"], ["anchors", "lookup", "terminal", "promotion_verify"])

    def test_label_key_is_the_single_constant(self) -> None:
        """ac12b：observer 的鍵 ＝ label shim ＝ orchestrator ＝ supervisor 的常數（鍵的漂移）。"""
        import re
        shim = (ROOT / "scripts" / "lib" / "i074-stage2-docker-label-shim.sh").read_text()
        orch = (ROOT / "scripts" / "run-i074-stage2.sh").read_text()
        keys = {self.sz.OBSERVE_LABEL_KEY, load_supervisor().LABEL_KEY,
                re.search(r"^I074_LABEL_KEY=(\S+)$", shim, re.M).group(1),
                re.search(r"^I074_LABEL_KEY=(\S+)$", orch, re.M).group(1)}
        self.assertEqual(keys, {"i074.stage2.run"})

    def test_snapshot_lists_match_the_entry_scripts(self) -> None:
        """ac19：兩個入口的 I074_BOOT_FILES ＝ helper 的 SNAPSHOT_FILES；兩份 bootstrap 除了常數之外逐字相同。"""
        import re
        blocks = {}
        for profile, rel in (("sizing", "scripts/i074-stage2-sizing.sh"), ("acceptance", "scripts/i074-stage2-acceptance.sh")):
            text = (ROOT / rel).read_text()
            files = re.search(r"^I074_BOOT_FILES=\((.*?)\)$", text, re.M | re.S).group(1).split()
            self.assertEqual(tuple(files), self.sz.SNAPSHOT_FILES[profile])
            self.assertEqual(re.search(r"^I074_BOOT_SELF=(\S+)$", text, re.M).group(1), rel)
            self.assertIn(rel, files)
            blocks[profile] = text[text.index("# >>> I074-STAGE2-BOOTSTRAP"):text.index("# <<< I074-STAGE2-BOOTSTRAP")]
        self.assertEqual(blocks["sizing"], blocks["acceptance"])



# ── ⑦d 增補（issue.md I-074「Stage 2 步驟 ⑦d 增補計畫：容器記憶體改以 RSS 判定」「五」）：ac24、ac24b、ac28b ──────────

def _starttime(pid: int) -> int:
    return int(Path(f"/proc/{pid}/stat").read_text().rsplit(")", 1)[1].split()[19])


def _alive(pid: int) -> bool:
    """存活 ＝ /proc 還在、而且⛔ 不是 zombie。"""
    try:
        return Path(f"/proc/{pid}/stat").read_text().rsplit(")", 1)[1].split()[0] not in ("Z", "X")
    except (OSError, IndexError):
        return False


def _wait_until(pred, timeout: float = 10.0) -> bool:
    end = __import__("time").monotonic() + timeout
    while __import__("time").monotonic() < end:
        if pred():
            return True
        __import__("time").sleep(0.05)
    return pred()


def _kill_quietly(pid: int) -> None:
    """測試自己啟動、記下 PID 的程序：以 PID 收掉（⛔ 不用 pkill -f）。"""
    try:
        os.kill(pid, signal.SIGKILL)
    except (ProcessLookupError, PermissionError):
        pass



# ── 測試衛生（issue.md I-074「Stage 2 步驟 ⑧ 計畫 v4」「四」）：收掉帶 cleanup token 的程序 ─────────────────────
#   ⚠️ 獨立於被測的產品函式（⛔ 不呼叫 host-run、reap_adopted、kill_pinned）；只看同一個 UID；排除本程序與它的祖先；
#   以 PID ＋ starttime 釘住，送任何訊號之前都重新比對（⛔ 不對被重用的 PID 送訊號）；TERM → 等待 → KILL → 重新掃描到
#   固定點。列舉之後才消失（`/proc/<pid>` 已不存在）是正常的競爭；PID 仍在卻讀不到或解析失敗、逾時仍有存活 → 失敗。


def _proc_entry(pid: int):
    """(ppid, starttime, argv)；已消失或是 zombie → None；PID 仍在卻讀不到 → AssertionError。"""
    base = f"/proc/{pid}"
    try:
        stat_text = Path(base, "stat").read_text()
        argv = Path(base, "cmdline").read_bytes().split(b"\0")
    except (FileNotFoundError, ProcessLookupError):
        if os.path.exists(base):
            raise AssertionError(f"{base} 仍在、卻讀不到身分")
        return None
    except OSError as exc:
        if not os.path.exists(base):
            return None
        raise AssertionError(f"讀不到 {base}：{exc}") from exc
    try:
        rest = stat_text.rsplit(")", 1)[1].split()
        state, ppid, start = rest[0], int(rest[1]), int(rest[19])
    except (IndexError, ValueError) as exc:
        if not os.path.exists(base):
            return None
        raise AssertionError(f"{base}/stat 解析失敗：{stat_text!r}") from exc
    if state in ("Z", "X"):
        return None
    return ppid, start, argv


def _same_uid_processes() -> dict:
    uid, out = os.getuid(), {}
    for name in os.listdir("/proc"):
        if not name.isdigit():
            continue
        pid = int(name)
        try:
            if os.stat(f"/proc/{pid}").st_uid != uid:
                continue
        except FileNotFoundError:
            continue
        entry = _proc_entry(pid)
        if entry is not None:
            out[pid] = entry
    return out


def _self_and_ancestors() -> set:
    pids, pid = set(), os.getpid()
    while pid > 1 and pid not in pids:
        pids.add(pid)
        entry = _proc_entry(pid)
        if entry is None:
            break
        pid = entry[0]
    return pids


def reap_token_processes(token: str, *, term_wait: float = 3.0, total: float = 20.0) -> list:
    """收掉 argv 帶 `token` 的程序與它們（掃描當時）的子孫；冪等。回傳本次送過訊號的 (pid, argv[0])。"""
    tok, exclude = token.encode(), _self_and_ancestors()
    pinned, reaped, deadline = {}, [], time.monotonic() + total
    while True:
        snap = _same_uid_processes()
        marked = {pid for pid, (_ppid, _start, argv) in snap.items() if any(tok in a for a in argv)}
        children: dict = {}
        for pid, (ppid, _start, _argv) in snap.items():
            children.setdefault(ppid, []).append(pid)
        stack = list(marked)
        while stack:
            for child in children.get(stack.pop(), []):
                if child not in marked:
                    marked.add(child)
                    stack.append(child)
        marked |= {pid for pid, (start, _t, _s) in pinned.items() if pid in snap and snap[pid][1] == start}
        marked -= exclude
        if not marked:
            return reaped
        if time.monotonic() > deadline:
            raise AssertionError(f"測試衛生：逾時仍存活 {sorted((p, snap[p][2][0]) for p in marked if p in snap)}")
        now = time.monotonic()
        for pid in sorted(marked):
            start = snap[pid][1]
            entry = pinned.get(pid)
            if entry is None or entry[0] != start:
                pinned[pid] = [start, now, signal.SIGTERM]
                reaped.append((pid, snap[pid][2][0].decode("utf-8", "replace")))
                sig = signal.SIGTERM
            elif entry[2] == signal.SIGTERM and now - entry[1] >= term_wait:
                entry[1], entry[2] = now, signal.SIGKILL
                sig = signal.SIGKILL
            else:
                continue
            current = _proc_entry(pid)
            if current is None or current[1] != start:
                continue                                    # 已消失或 PID 已被重用：⛔ 不送
            try:
                os.kill(pid, sig)
            except ProcessLookupError:
                pass
        time.sleep(0.05)


# ⑧ 第一輪 review：長存的測試程序一律是 argv 明確帶 token 的 Python sleeper（⛔ 不用不帶 token 的 `sleep 30`——
# 它被收養之後與 token 失去關聯，driver 中途失敗時 teardown 收不到）。sh 的寫法以 exec 換成它（$0＝token、$1＝python；
# 忽略 TERM 的版本先 trap 再 exec，SIG_IGN 跨 exec 保留，PID 與 starttime 都不變）。
SLEEPER_CODE = "import time; time.sleep(30)"
SH_SLEEP30 = 'exec "$1" -c "import time; time.sleep(30)" "$0"'
SH_SLEEP30_IGNORE_TERM = "trap '' TERM; " + SH_SLEEP30


def _sleeper(token: str) -> list:
    return [sys.executable, "-c", SLEEPER_CODE, token]


MARKER_LOOP = "while :; do date +%s%N > \"$0\"; sleep 0.1; done"           # 存活的子孫：持續寫標記檔


class RssAddendumHost(unittest.TestCase):
    """⑦d 增補的 host 端：`host-run` 的收養與清理、`reap-adopted`、`kill-pinned`（3.9）。"""

    def setUp(self) -> None:
        self.sz = load_sizing()
        self.tmp = Path(tempfile.mkdtemp(prefix="i074-rss-host-"))
        self.addCleanup(shutil.rmtree, self.tmp, True)
        self.pids: list[int] = []
        self.addCleanup(lambda: [_kill_quietly(p) for p in self.pids])
        # ⑧「四」：每支測試一個 cleanup token，起的長存程序都在 argv 帶它；teardown 再收一次當保險（失敗即紅）。
        self.token = "i074h1-" + secrets.token_hex(8)
        self.addCleanup(lambda: reap_token_processes(self.token))

    def _driver(self, code: str, timeout: float = 120) -> subprocess.CompletedProcess:
        """以 driver 在自己的 session 裡執行（sz ＝ 本 repo 的 helper）。"""
        prelude = textwrap.dedent(f"""\
            import importlib.util, json, os, signal, subprocess, sys, time
            from pathlib import Path
            spec = importlib.util.spec_from_file_location("sz", {str(SIZING)!r})
            sz = importlib.util.module_from_spec(spec); spec.loader.exec_module(sz)
        """)
        return subprocess.run([sys.executable, "-c", prelude + textwrap.dedent(code)], capture_output=True, text=True,
                              timeout=timeout, start_new_session=True, env=dict(os.environ, PYTHONDONTWRITEBYTECODE="1"))

    def _record(self, step: str = "s") -> dict:
        return json.loads((self.tmp / "S" / "host" / f"{step}.json").read_text())

    def _marker_stopped(self, marker: Path) -> bool:
        if not marker.exists():
            return True
        before = marker.read_text()
        __import__("time").sleep(0.4)
        return marker.read_text() == before

    # ── ac24：host-run ──────────────────────────────────────────────────────

    def test_ac24_unwaited_grandchild_is_adopted_and_counted(self) -> None:
        leader = ("import subprocess, sys, os; subprocess.Popen([sys.executable, '-c', "
                  "'import time; x = bytearray(64 * 1024 * 1024); time.sleep(0.5)']); os._exit(0)")
        proc = self._driver(f"sys.exit(sz.host_run(Path({str(self.tmp / 'S')!r}), 's', [sys.executable, '-c', {leader!r}]))")
        rec = self._record()
        self.assertEqual((proc.returncode, rec["rc"]), (0, 0), proc.stderr)
        self.assertGreaterEqual(rec["children_max_rss_bytes"], 64 * MIB)
        self.assertTrue(rec["all_descendants_reaped"])
        self.assertEqual(self.sz.host_record_problems(rec, step="s", rc=0), [])

    def test_ac24_host_run_itself_is_in_the_exact_gate_and_the_group_sample(self) -> None:
        proc = self._driver(f"""
            keep = bytearray(96 * 1024 * 1024)
            sys.exit(sz.host_run(Path({str(self.tmp / 'S')!r}), 's', ['sleep', '0.6']))
        """)
        rec = self._record()
        self.assertEqual(proc.returncode, 0, proc.stderr)
        self.assertGreaterEqual(rec["self_max_rss_bytes"], 96 * MIB)
        self.assertGreaterEqual(rec["group_rss_peak_sampled_bytes"], 96 * MIB)
        self.assertEqual(rec["max_single_rss_bytes"], max(rec["self_max_rss_bytes"], rec["children_max_rss_bytes"]))

    def test_ac24_group_sample_follows_the_ppid_tree_including_setsid(self) -> None:
        """ac16 改寫：取樣以 ppid 鏈追到的全部子孫（含 setsid 的），⛔ 不只看 process group；無關的程序⛔ 不算。"""
        outsider = subprocess.Popen([sys.executable, "-c", "import time; x = bytearray(128 * 1024 * 1024); time.sleep(5)",
                                     self.token])
        self.pids.append(outsider.pid)
        leader = ("import subprocess, sys; subprocess.run(['setsid', sys.executable, '-c', "
                  "'import time; x = bytearray(64 * 1024 * 1024); time.sleep(1.0)'])")
        proc = self._driver(f"sys.exit(sz.host_run(Path({str(self.tmp / 'S')!r}), 's', [sys.executable, '-c', {leader!r}]))")
        rec = self._record()
        self.assertEqual(proc.returncode, 0, proc.stderr)
        self.assertGreaterEqual(rec["group_rss_peak_sampled_bytes"], 64 * MIB + 20 * MIB)
        self.assertLess(rec["group_rss_peak_sampled_bytes"], 128 * MIB + 64 * MIB)
        outsider.kill()
        outsider.wait()

    def _orphan_case(self, script: str, **kwargs) -> tuple[subprocess.CompletedProcess, dict, Path]:
        marker = self.tmp / "marker"
        leader = (f"import subprocess, os; p = subprocess.Popen(['setsid', 'sh', '-c', {script!r}, {str(marker)!r}, {self.token!r}], stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL); "
                  f"open({str(self.tmp / 'orphan.pid')!r}, 'w').write(str(p.pid)); os._exit(0)")
        args = ", ".join(f"{k}={v!r}" for k, v in kwargs.items())
        proc = self._driver(f"sys.exit(sz.host_run(Path({str(self.tmp / 'S')!r}), 's', [sys.executable, '-c', {leader!r}], {args}))")
        pid_file = self.tmp / "orphan.pid"
        if pid_file.exists():
            self.pids.append(int(pid_file.read_text()))
        return proc, self._record(), marker

    def test_ac24_live_setsid_descendant_beyond_the_grace_is_cleaned(self) -> None:
        proc, rec, marker = self._orphan_case(MARKER_LOOP, grace_s=1.0, term_wait_s=2.0, cleanup_total_s=8.0)
        self.assertEqual(proc.returncode, 0, proc.stderr)
        self.assertFalse(rec["all_descendants_reaped"])
        self.assertTrue(rec["cleanup_complete"])
        self.assertEqual(rec["leftover_pids"], [])
        self.assertTrue(self._marker_stopped(marker))
        self.assertTrue(self.sz.host_record_problems(rec, step="s", rc=0))          # 量測必然失敗
        self.assertEqual(reap_token_processes(self.token), [])      # ⑧「四」：產品已收乾淨，helper 沒有東西可收

    def test_ac24_term_ignoring_descendant_is_escalated_to_kill(self) -> None:
        proc, rec, marker = self._orphan_case("trap '' TERM; " + MARKER_LOOP, grace_s=1.0, term_wait_s=1.0,
                                              cleanup_total_s=8.0)
        self.assertEqual(proc.returncode, 0, proc.stderr)
        self.assertTrue(rec["cleanup_complete"])
        self.assertTrue(self._marker_stopped(marker))
        self.assertEqual(reap_token_processes(self.token), [])      # ⑧「四」：產品已收乾淨，helper 沒有東西可收

    def test_ac24_cleanup_that_cannot_finish_exits_70(self) -> None:
        marker = self.tmp / "marker"
        leader = (f"import subprocess, os; p = subprocess.Popen(['setsid', 'sh', '-c', {MARKER_LOOP!r}, {str(marker)!r}, {self.token!r}], stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL); "
                  f"open({str(self.tmp / 'orphan.pid')!r}, 'w').write(str(p.pid)); os._exit(0)")
        proc = self._driver(f"""
            os.kill = lambda pid, sig: None                     # 送訊號換成 no-op
            sys.exit(sz.host_run(Path({str(self.tmp / 'S')!r}), 's', [sys.executable, '-c', {leader!r}],
                                 grace_s=0.5, term_wait_s=0.5, cleanup_total_s=2.0))
        """)
        self.pids.append(int((self.tmp / "orphan.pid").read_text()))
        rec = self._record()
        self.assertEqual(proc.returncode, 70, proc.stderr)
        self.assertFalse(rec["cleanup_complete"])
        self.assertEqual(rec["leftover_pids"], [self.pids[-1]])
        # 實作第一輪 review：紀錄的 rc ＝ 實際的結束碼（70），rc 那一條⛔ 沒有問題（量測仍因清不乾淨而無效）
        self.assertEqual(rec["rc"], 70)
        problems = self.sz.host_record_problems(rec, step="s", rc=70)
        self.assertFalse([p for p in problems if p.startswith("rc=")], problems)
        self.assertTrue(problems)
        # ⑧「四」：產品刻意收不掉的那一個，由 helper 收（⛔ 不留到 teardown 才以 PID 殺）
        self.assertIn(self.pids[-1], [pid for pid, _ in reap_token_processes(self.token)])
        self.assertTrue(self._marker_stopped(marker))

    def test_h1_helper_reaps_what_a_broken_product_cleanup_leaves(self) -> None:
        """⑧「四」⑤：以 driver 把產品的清理換成 no-op（送訊號全部失效）→ 存活的子孫留下；獨立的 helper 仍收得掉、
        收掉之後⛔ 沒有存活，而且冪等（第二次沒有候選）。"""
        marker = self.tmp / "marker"
        leader = (f"import subprocess, os; p = subprocess.Popen(['setsid', 'sh', '-c', {MARKER_LOOP!r}, {str(marker)!r}, {self.token!r}], stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL); "
                  f"open({str(self.tmp / 'orphan.pid')!r}, 'w').write(str(p.pid)); os._exit(0)")
        self._driver(f"""
            os.kill = lambda pid, sig: None                     # 產品的清理失效
            sys.exit(sz.host_run(Path({str(self.tmp / 'S')!r}), 's', [sys.executable, '-c', {leader!r}],
                                 grace_s=0.3, term_wait_s=0.3, cleanup_total_s=1.0))
        """)
        orphan = int((self.tmp / "orphan.pid").read_text())
        self.pids.append(orphan)                                # 最後一道保險（helper 若壞掉，teardown 仍以 PID 收）
        self.assertTrue(_alive(orphan))                          # 產品確實沒收（⛔ 不是自己結束的）
        self.assertIn(orphan, [pid for pid, _ in reap_token_processes(self.token)])
        self.assertFalse(_alive(orphan))
        self.assertTrue(self._marker_stopped(marker))
        self.assertEqual(reap_token_processes(self.token), [])

    def test_ac24_sigchld_ignored_descendant_is_detected(self) -> None:
        leader = "import signal, time; signal.signal(signal.SIGCHLD, signal.SIG_IGN); time.sleep(0.8)"
        proc = self._driver(f"sys.exit(sz.host_run(Path({str(self.tmp / 'S')!r}), 's', [sys.executable, '-c', {leader!r}]))")
        self.assertEqual(proc.returncode, 0, proc.stderr)
        self.assertTrue(self._record()["auto_reap_detected"])

    def test_ac24_signals_to_host_run_clean_up_and_set_the_exit_code(self) -> None:
        for sig, want in ((signal.SIGTERM, 143), (signal.SIGINT, 130), (signal.SIGHUP, 129)):
            with self.subTest(sig=sig):
                tmp = self.tmp / sig.name
                marker = tmp / "marker"
                tmp.mkdir()
                leader = (f"import subprocess, time; subprocess.Popen(['setsid', 'sh', '-c', {MARKER_LOOP!r}, {str(marker)!r}, {self.token!r}], stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL); "
                          "time.sleep(60)")
                host = subprocess.Popen([sys.executable, str(SIZING), "host-run", "--state", str(tmp / "S"), "--step", "s",
                                         "--", sys.executable, "-c", leader], start_new_session=True,
                                        env=dict(os.environ, PYTHONDONTWRITEBYTECODE="1"))
                self.pids.append(host.pid)
                self.assertTrue(_wait_until(marker.exists))
                os.kill(host.pid, sig)                               # 只送給 host-run 自己（⛔ 不送給整個 group）
                self.assertEqual(host.wait(timeout=30), want)
                self.assertTrue(self._marker_stopped(marker))
                rec = json.loads((tmp / "S" / "host" / "s.json").read_text())
                self.assertTrue(rec["errors"] and rec["cleanup_complete"])
                # 實作第一輪 review：紀錄的 rc ＝ 實際的結束碼（143／130／129）
                self.assertEqual(rec["rc"], want)
                problems = self.sz.host_record_problems(rec, step="s", rc=want)
                self.assertFalse([p for p in problems if p.startswith("rc=")], problems)
                self.assertTrue(problems)                                   # 量測仍因中斷而無效
                self.assertEqual(reap_token_processes(self.token), [])      # ⑧「四」：產品已收乾淨，helper 沒有東西可收

    # ── 增補實作第二輪 review：訊號的 linearization point（在紀錄寫入邊界注入訊號） ─────────────────

    def _linearization_case(self, inject: str) -> tuple[subprocess.CompletedProcess, dict, dict]:
        proc = self._driver(inject + f"""
        rc = sz.host_run(Path({str(self.tmp / 'S')!r}), "s", ["true"])
        print(json.dumps({{"return_rc": rc}}), flush=True)
        """)
        return proc, json.loads(proc.stdout or "{}"), self._record()

    def test_ac24_signal_before_the_linearization_point_is_reflected(self) -> None:
        """L 之前送達（handler 還在）→ 錯誤、紀錄的 rc 與回傳碼都反映它。"""
        proc, out, rec = self._linearization_case("""
        orig = sz._signal_linearization_point
        def wrapped(received):
            os.kill(os.getpid(), signal.SIGTERM)
            return orig(received)
        sz._signal_linearization_point = wrapped
        """)
        self.assertEqual(out.get("return_rc"), 143, proc.stderr)
        self.assertEqual(rec["rc"], 143)
        self.assertTrue(any("SIGTERM" in e for e in rec["errors"]), rec["errors"])
        problems = self.sz.host_record_problems(rec, step="s", rc=143)
        self.assertFalse([p for p in problems if p.startswith("rc=")], problems)

    def test_ac24_signal_pending_at_the_linearization_point_is_reflected(self) -> None:
        """已經被擋住、還在 pending 的訊號 → L 把它收進來，同樣反映。"""
        proc, out, rec = self._linearization_case("""
        orig = sz._signal_linearization_point
        def wrapped(received):
            signal.pthread_sigmask(signal.SIG_BLOCK, {signal.SIGHUP})
            os.kill(os.getpid(), signal.SIGHUP)
            return orig(received)
        sz._signal_linearization_point = wrapped
        """)
        self.assertEqual(out.get("return_rc"), 129, proc.stderr)
        self.assertEqual(rec["rc"], 129)
        self.assertTrue(any("SIGHUP" in e for e in rec["errors"]), rec["errors"])

    def test_ac24_signal_during_the_record_write_is_not_swallowed(self) -> None:
        """review 的注入：寫紀錄的入口才收到 TERM（L 之後）→ 紀錄已定案、⛔ 不能改，但回傳碼是 143、stderr 照實說明，
        紀錄的 rc 與實際結束碼不符 → host_record_problems 擋下（fail-closed，⛔ 不會宣稱成功）。"""
        proc, out, rec = self._linearization_case("""
        orig = sz.write_exclusive
        def inject(path, payload):
            if path.parent.name == "host":
                os.kill(os.getpid(), signal.SIGTERM)
            return orig(path, payload)
        sz.write_exclusive = inject
        """)
        self.assertEqual(out.get("return_rc"), 143, proc.stderr)
        self.assertIn("紀錄寫出之後才收到訊號", proc.stderr)
        self.assertEqual(rec["rc"], 0)
        problems = self.sz.host_record_problems(rec, step="s", rc=143)
        self.assertTrue([p for p in problems if p.startswith("rc=")], problems)

    def test_ac24_late_signal_keeps_the_cleanup_failure_priority(self) -> None:
        """增補實作第三輪 review：清不乾淨（70）＋ 寫紀錄期間才收到 TERM → 仍是 70（優先序：清不乾淨 ＞ 訊號），
        紀錄與實際結束碼一致；stderr 照樣說明晚到的訊號。"""
        proc = self._driver(f"""
        sz._reap_nohang = lambda: False                              # 收不到 ECHILD → 進清理
        sz._cleanup_direct_children = lambda me, **kw: [999999]      # 清理失敗（殘留一個 PID）
        orig = sz.write_exclusive
        def inject(path, payload):
            if path.parent.name == "host":
                os.kill(os.getpid(), signal.SIGTERM)                 # L 之後、寫紀錄的入口
            return orig(path, payload)
        sz.write_exclusive = inject
        rc = sz.host_run(Path({str(self.tmp / 'S')!r}), "s", ["true"], grace_s=0.2)
        print(json.dumps({{"return_rc": rc}}), flush=True)
        """)
        out = json.loads(proc.stdout or "{}")
        rec = self._record()
        self.assertEqual(out.get("return_rc"), 70, proc.stderr)
        self.assertEqual(rec["rc"], 70)
        self.assertEqual((rec["cleanup_complete"], rec["leftover_pids"]), (False, [999999]))
        self.assertIn("紀錄寫出之後才收到訊號", proc.stderr)
        problems = self.sz.host_record_problems(rec, step="s", rc=70)
        self.assertFalse([p for p in problems if p.startswith("rc=")], problems)

    # ── ac24b：harness 的收養（測試以一個 subreaper 的 driver 扮演 harness） ─────────────────────

    FAKE_HARNESS = """
        import ctypes
        if ctypes.CDLL(None, use_errno=True).prctl(36, 1, 0, 0, 0) != 0:
            sys.exit(99)
    """

    def _reap_child(self, extra: str = "", **kwargs) -> str:
        args = ", ".join(f"{k}={v!r}" for k, v in kwargs.items())
        return f"""
        rp = os.fork()
        if rp == 0:
            pp = os.getppid(); st = sz.read_proc_stat(pp)[1]          # 先記下 parent 的身分，再套用注入
            {extra}
            os._exit(sz.reap_adopted(Path({str(self.tmp / 'S')!r}), parent=pp, parent_starttime=st, exclude=[], {args}))
        reap_rc = os.waitstatus_to_exitcode(os.waitpid(rp, 0)[1])
        """

    def test_ac24b_descendants_left_by_a_killed_host_run_are_adopted_and_reaped(self) -> None:
        marker = self.tmp / "marker"
        leader = (f"import subprocess, time; p = subprocess.Popen(['setsid', 'sh', '-c', {MARKER_LOOP!r}, {str(marker)!r}, {self.token!r}], stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL); "
                  f"open({str(self.tmp / 'orphan.pid')!r}, 'w').write(str(p.pid)); time.sleep(60)")
        proc = self._driver(self.FAKE_HARNESS + f"""
        (Path({str(self.tmp)!r}) / "S").mkdir(exist_ok=True)
        host = subprocess.Popen([sys.executable, {str(SIZING)!r}, "host-run", "--state", {str(self.tmp / 'S')!r},
                                 "--step", "s", "--", sys.executable, "-c", {leader!r}])
        while not Path({str(marker)!r}).exists():
            time.sleep(0.05)
        host.kill(); host.wait()                                     # host-run 被 KILL：清理來不及跑
        """ + self._reap_child(term_wait_s=2.0, kill_wait_s=2.0, total_s=20.0) + """
        print(json.dumps({"reap_rc": reap_rc}), flush=True)
        """)
        self.pids.append(int((self.tmp / "orphan.pid").read_text()))
        self.assertEqual(json.loads(proc.stdout)["reap_rc"], 0, proc.stderr)
        self.assertFalse(_alive(self.pids[-1]))
        self.assertTrue(self._marker_stopped(marker))
        self.assertEqual(reap_token_processes(self.token), [])      # ⑧「四」：reap_adopted 已收乾淨

    def test_ac24b_process_forked_during_the_term_grace_is_adopted_and_reaped(self) -> None:
        marker = self.tmp / "marker"
        leader = textwrap.dedent(f"""\
            import os, signal, subprocess, time
            def on_term(*_):
                p = subprocess.Popen(['setsid', 'sh', '-c', {MARKER_LOOP!r}, {str(marker)!r}, {self.token!r}], stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
                open({str(self.tmp / 'late.pid')!r}, 'w').write(str(p.pid))
                os._exit(0)
            signal.signal(signal.SIGTERM, on_term)
            open({str(self.tmp / 'ready')!r}, 'w').write('1')
            time.sleep(60)
        """)
        proc = self._driver(self.FAKE_HARNESS + f"""
        (Path({str(self.tmp)!r}) / "S").mkdir(exist_ok=True)
        host = subprocess.Popen([sys.executable, {str(SIZING)!r}, "host-run", "--state", {str(self.tmp / 'S')!r},
                                 "--step", "s", "--", sys.executable, "-c", {leader!r}])
        while not Path({str(self.tmp / 'ready')!r}).exists():
            time.sleep(0.05)
        host.terminate()                                             # host-run 開始清理 → leader 在 TERM 時才 fork
        while not Path({str(marker)!r}).exists():
            time.sleep(0.02)
        host.kill(); host.wait()                                     # 清理途中被 KILL
        """ + self._reap_child(term_wait_s=2.0, kill_wait_s=2.0, total_s=20.0) + """
        print(json.dumps({"reap_rc": reap_rc}), flush=True)
        """)
        self.pids.append(int((self.tmp / "late.pid").read_text()))
        self.assertEqual(json.loads(proc.stdout)["reap_rc"], 0, proc.stderr)
        self.assertFalse(_alive(self.pids[-1]))
        self.assertTrue(self._marker_stopped(marker))
        self.assertEqual(reap_token_processes(self.token), [])      # ⑧「四」：reap_adopted 已收乾淨

    def _target_case(self, reap_extra: str, script: str = SH_SLEEP30_IGNORE_TERM, **kwargs) -> tuple[int, int]:
        """driver（subreaper）啟動一個目標程序（預設忽略 TERM），再以 fork 出的子程序呼叫 reap_adopted。回傳 (rc, 目標 PID)。"""
        proc = self._driver(self.FAKE_HARNESS + f"""
        (Path({str(self.tmp)!r}) / "S").mkdir(exist_ok=True)
        target = subprocess.Popen(["sh", "-c", {script!r}, {self.token!r}, {sys.executable!r}], stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        time.sleep(0.2)
        """ + self._reap_child(reap_extra, **kwargs) + """
        print(json.dumps({"reap_rc": reap_rc, "target": target.pid}), flush=True)
        os._exit(0)                                                  # ⛔ 不收目標（讓測試確認它是否還活著）
        """)
        out = json.loads(proc.stdout)
        self.pids.append(out["target"])
        return out["reap_rc"], out["target"]

    def test_ac24b_starttime_changing_before_the_signal_means_no_signal(self) -> None:
        extra = """counter = iter(range(10 ** 9))
            real = sz.read_proc_stat
            sz.read_proc_stat = lambda pid: (real(pid)[0], real(pid)[1] + next(counter)) if pid != os.getppid() else real(pid)"""
        # ⚠️ 目標⛔ 不忽略 TERM（反向驗證抓到：忽略 TERM 的目標在「不驗 starttime」時也活得下來，測不出差別）
        rc, target = self._target_case(extra, script=SH_SLEEP30, term_wait_s=0.3, kill_wait_s=0.3, total_s=1.5)
        self.assertEqual(rc, 1)
        self.assertTrue(_alive(target))                              # ⛔ 沒有送任何訊號（一個 TERM 就會讓它結束）
        self.assertTrue((self.tmp / "S" / "leftover-pids.json").exists())

    def test_ac24b_parent_identity_mismatch_is_2_without_signals(self) -> None:
        proc = self._driver(self.FAKE_HARNESS + f"""
        (Path({str(self.tmp)!r}) / "S").mkdir(exist_ok=True)
        target = subprocess.Popen({_sleeper(self.token)!r}, stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        rp = os.fork()
        if rp == 0:
            os._exit(sz.reap_adopted(Path({str(self.tmp / 'S')!r}), parent=os.getppid(), parent_starttime=1, exclude=[]))
        print(json.dumps({{"reap_rc": os.waitstatus_to_exitcode(os.waitpid(rp, 0)[1]), "target": target.pid}}), flush=True)
        os._exit(0)
        """)
        out = json.loads(proc.stdout)
        self.pids.append(out["target"])
        self.assertEqual(out["reap_rc"], 2)
        self.assertTrue(_alive(out["target"]))

    def test_ac24b_parent_changing_during_the_run_stops_the_signals(self) -> None:
        extra = """calls = iter(range(10 ** 9))
            real_ppid = os.getppid
            os.getppid = lambda: real_ppid() if next(calls) < 3 else 1        # 第一輪 TERM 之後 parent 就「消失」"""
        rc, target = self._target_case(extra, term_wait_s=0.5, kill_wait_s=0.5, total_s=5.0)
        self.assertEqual(rc, 2)
        self.assertTrue(_alive(target))                              # 目標忽略 TERM；⛔ 沒有升級到 KILL

    def test_ac24b_check_only_and_leftovers_and_proc_failures(self) -> None:
        rc, _ = self._target_case("", check_only=True)
        self.assertEqual(rc, 1)
        rc, target = self._target_case("os.kill = lambda pid, sig: None", term_wait_s=0.3, kill_wait_s=0.3, total_s=1.0)
        self.assertEqual(rc, 1)
        doc = json.loads((self.tmp / "S" / "leftover-pids.json").read_text())
        self.assertEqual(set(doc), {"schema", "parent", "processes"})
        self.assertEqual(doc["schema"], "i074_stage2_leftover_pids_v1")
        self.assertEqual([p["pid"] for p in doc["processes"]], [target])
        self.assertEqual(set(doc["processes"][0]), {"pid", "starttime", "cmdline"})
        rc, _ = self._target_case("sz.PROC_ROOT = Path('/nonexistent-proc')")
        self.assertEqual(rc, 2)

    # ── ac27（scripts/ 那一半）與 ac20（3.9 相容） ─────────────────────────────────────

    def test_ac27_repo_never_sets_auto_reaping(self) -> None:
        """測試容器只掛 python/——scripts/ 在 host 以同一個語意掃描涵蓋（python/ 也再掃一次）。"""
        self.assertTrue((ROOT / "scripts" / "lib" / "i074-stage2-measure.sh").is_file())     # ⛔ 不是空掃描
        self.assertEqual(self.sz.scan_auto_reaping([ROOT / "scripts", ROOT / "python"], ROOT), [])

    def test_ac20_wrapper_runs_on_host_python(self) -> None:
        cg = self.tmp / "cg" / "memory"
        cg.mkdir(parents=True)
        (cg / "memory.stat").write_text("total_rss 4096\n")
        (cg / "memory.max_usage_in_bytes").write_text("8192\n")
        peak = self.tmp / "peak"
        peak.mkdir()
        proc = subprocess.run([sys.executable, str(ROOT / "python" / "scripts" / "i074_stage2_rss_wrapper.py"),
                               sys.executable, "-c", "import sys; sys.exit(3)"], capture_output=True, timeout=60,
                              env=dict(os.environ, SIZING_CGROUP_ROOT=str(self.tmp / "cg"), SIZING_PEAK_DIR=str(peak),
                                       PYTHONDONTWRITEBYTECODE="1"))
        self.assertEqual((proc.returncode, proc.stdout, proc.stderr), (3, b"", b""))
        rec = json.loads((peak / "rss.json").read_text())
        self.assertEqual(self.sz.rss_record_problems(rec, require_pid1=False), [])
        self.assertEqual(rec["reaper"], "subreaper")

    # ── ac28b：kill-pinned ──────────────────────────────────────────────────

    def _child(self, script: str) -> int:
        proc = subprocess.Popen(["sh", "-c", script, self.token, sys.executable])
        self.pids.append(proc.pid)
        self.addCleanup(lambda: (_kill_quietly(proc.pid), proc.wait()))   # 測試結束時才殺、才收
        __import__("time").sleep(0.2)
        return proc.pid                                               # ⚠️ 測試⛔ 不收它：被殺之後是 zombie

    def _check_record(self, rec: dict, rc: int) -> None:
        self.assertEqual(set(rec), {"schema", "pid", "starttime", "result", "signals_sent", "error"})
        self.assertEqual(rec["schema"], "i074_stage2_kill_pinned_v1")
        want = {"already_gone": 0, "identity_mismatch": 0, "terminated_by_term": 0, "terminated_by_kill": 0,
                "alive_after_kill": 1, "error_before_signal": 2, "error_after_term": 2}
        self.assertEqual(want[rec["result"]], rc)
        self.assertIn(rec["signals_sent"], ([], ["TERM"], ["TERM", "KILL"]))

    def test_ac28b_already_gone_and_identity_mismatch(self) -> None:
        done = subprocess.Popen(["true"])
        done.wait()
        rc, rec = self.sz.kill_pinned(done.pid, 1, term_wait_s=0.5, kill_wait_s=0.5)
        self._check_record(rec, rc)
        self.assertEqual((rc, rec["result"], rec["signals_sent"]), (0, "already_gone", []))
        pid = self._child(SH_SLEEP30)
        rc, rec = self.sz.kill_pinned(pid, _starttime(pid) + 1, term_wait_s=0.5, kill_wait_s=0.5)
        self._check_record(rec, rc)
        self.assertEqual((rc, rec["result"], rec["signals_sent"]), (0, "identity_mismatch", []))
        self.assertTrue(_alive(pid))

    def test_ac28b_term_kill_and_zombies_count_as_gone(self) -> None:
        pid = self._child(SH_SLEEP30)
        rc, rec = self.sz.kill_pinned(pid, _starttime(pid), term_wait_s=2.0, kill_wait_s=2.0)
        self._check_record(rec, rc)
        self.assertEqual((rc, rec["result"]), (0, "terminated_by_term"))          # 成了 zombie（測試⛔ 不收）→ 已消失
        pid = self._child(SH_SLEEP30_IGNORE_TERM)
        rc, rec = self.sz.kill_pinned(pid, _starttime(pid), term_wait_s=0.5, kill_wait_s=2.0)
        self._check_record(rec, rc)
        self.assertEqual((rc, rec["result"], rec["signals_sent"]), (0, "terminated_by_kill", ["TERM", "KILL"]))

    def test_ac28b_alive_after_kill_and_errors(self) -> None:
        pid = self._child(SH_SLEEP30_IGNORE_TERM)
        st = _starttime(pid)
        with mock.patch.object(self.sz.os, "kill", lambda p, s: None):
            rc, rec = self.sz.kill_pinned(pid, st, term_wait_s=0.3, kill_wait_s=0.3)
        self._check_record(rec, rc)
        self.assertEqual((rc, rec["result"]), (1, "alive_after_kill"))
        with mock.patch.object(self.sz, "read_proc_stat", side_effect=PermissionError("denied")):
            rc, rec = self.sz.kill_pinned(pid, st)
        self._check_record(rec, rc)
        self.assertEqual((rc, rec["result"], rec["signals_sent"]), (2, "error_before_signal", []))
        real = self.sz.read_proc_stat
        calls = iter(range(10 ** 6))
        def flaky(p):
            if next(calls) >= 2:
                raise PermissionError("denied")
            return real(p)
        with mock.patch.object(self.sz, "read_proc_stat", side_effect=flaky):
            rc, rec = self.sz.kill_pinned(pid, st, term_wait_s=0.3, kill_wait_s=0.3)
        self._check_record(rec, rc)
        self.assertEqual((rc, rec["result"], rec["signals_sent"]), (2, "error_after_term", ["TERM"]))
        self.assertTrue(_alive(pid))                                  # 忽略 TERM、⛔ 沒有升級 KILL
        cli = subprocess.run([sys.executable, str(SIZING), "kill-pinned", "--pid", "x", "--starttime", "1"],
                             capture_output=True, text=True, env=dict(os.environ, PYTHONDONTWRITEBYTECODE="1"))
        self.assertEqual((cli.returncode, cli.stdout), (2, ""))


# ── 有效性條件計畫：offline 的讀取端（`precheck-verdict`、`raw-manifest`）的信任模型 ─────────────────────────────
#   issue.md I-074「Stage 2 量測的有效性條件與 ⑨-1 fail-fast 計畫」「六」：外部的 commit 錨點、C1／C2 的標記、原始量測裡的程式
#   ⛔ 不被執行。需要 git，所以在 host（測試 image 裡沒有 git）；原始量測以 helper 自己的 writer 建（與 pytest 的 fixture 同形）。

IMG = "sha256:" + "f" * 64
GIT = ["git", "-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false", "-c", "tag.gpgsign=false"]
RECOMPUTE_ANCHOR = '    raw = Path(raw)\n    _require_dir(raw, "--raw")\n    data = (raw / "precheck.json").read_bytes()\n'
MANIFEST_ANCHOR = '    raw = Path(raw)\n    _require_dir(raw, "--raw")\n    if raw.name not in RAW_ROOT_NAMES:\n'


def _marked(source: str, anchor: str, marker: Path) -> str:
    """在 recompute／產生演算法的**入口**插入寫標記檔的一行（第七輪 review：⛔ 不放在模組載入或對外的 frontend）。"""
    assert source.count(anchor) == 1, anchor
    return source.replace(anchor, f"    Path({str(marker)!r}).write_text('x')\n" + anchor)


class OfflineReadersHost(unittest.TestCase):
    def setUp(self) -> None:
        self.sz = load_sizing()
        self.tmp = Path(tempfile.mkdtemp(prefix="i074-offline-"))
        self.addCleanup(shutil.rmtree, self.tmp, True)
        self.pf = importlib.util.module_from_spec(importlib.util.spec_from_file_location(
            "pf_offline", ROOT / "python" / "scripts" / "i074_stage2_preflight.py"))
        self.pf.__spec__.loader.exec_module(self.pf)
        self.sup = load_supervisor()
        self.repo = self.tmp / "repo"
        for rel in self.sz.SNAPSHOT_FILES["acceptance"]:
            (self.repo / rel).parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(ROOT / rel, self.repo / rel)
        self._git("init", "-q")
        self.c1 = self._commit("c1")

    def _git(self, *args: str) -> str:
        return subprocess.run([*GIT, "-C", str(self.repo), *args], check=True, capture_output=True, text=True).stdout.strip()

    def _commit(self, message: str) -> str:
        self._git("add", "-A")
        self._git("commit", "-q", "--allow-empty", "-m", message)
        return self._git("rev-parse", "HEAD")

    def _raw(self, name: str, commit: str, *, harness_patch=None) -> Path:
        """量測趟之前中止的原始量測（形狀 A；目錄名 raw-failed），快照 ＝ `commit` 的檔案（`harness_patch` 可以改掉其中一個）。"""
        sz = self.sz
        s = self.tmp / name / "raw-failed"
        meta = {"run_id": name, "mode": "validation", "image": IMG, "l0_dev": "42", "docker_root_dev": "42",
                "repo_dev": "42", "state_fstype": "tmpfs", "replay_compute": "full", "repo_head": commit,
                "clone_head": commit}
        s.mkdir(parents=True)
        (s / "meta.tsv").write_text("".join(f"{k}\t{v}\n" for k, v in meta.items()))
        (s / "components.tsv").write_text("".join(f"{k}\t{v}\n" for k, v in {
            "wt_head": 30 * MIB, "wt_base_patched": 15 * MIB, "git_head": 131072, "git_base_patched": 131072,
            "index_head": 98304, "index_base_patched": 98304, "snapshot": 262144, "probe": 24576,
            "frozen_patched": 266240}.items()))
        steps, invocations = sz.acceptance_expected("full", "before_full")
        want = {"replay_failure": 6, "publish_failed_record": 1, "promote_failure": 6, "check_failed_record": 2,
                "recover_failed_record": 1}
        (s / "rc.tsv").write_text("".join(f"{n}\t{want.get(n, 0)}\t{want.get(n, 0)}\n" for n, _ in steps))
        for seq, (phase, role) in enumerate(invocations, start=1):
            cid = sz.container_id(seq)
            argv = ["--memory=444m", "--memory-swap=444m", "--cidfile", f"/s/{cid}", "--name", f"n-{cid}", IMG, "run"]
            index = sz.write_index(s, sequence=seq, cid=cid, phase=phase, role=role,
                                   included=phase in sz.measured_phases("acceptance"), image=IMG, argv=argv,
                                   profile="acceptance")
            sz.write_exclusive(s / "containers" / f"{cid}.json", sz.canonical_dumps({
                "id": cid, "sequence": seq, "container_spec_sha256": index["container_spec_sha256"], "rc": 0,
                "container": "c", "size_rw": 4096, "log_config": {"Type": "json-file", "Config": {}}, "log_bound": 4096,
                "peak_bytes": 100 * MIB, "memory_limit_bytes": 444 * MIB, "memory_swap_limit_bytes": 444 * MIB,
                "rss_peak_sampled_bytes": 150 * MIB, "rss_samples": 20, "self_max_rss_bytes": 12 * MIB,
                "children_max_rss_bytes": 200 * MIB, "max_single_rss_bytes": 200 * MIB, "reaper": "pid1",
                "status": "ok", "failures": []}))
            sz.write_exclusive(s / "twins" / f"{cid}.json", sz.canonical_dumps({
                "id": cid, "sequence": seq, "container_spec_sha256": index["container_spec_sha256"],
                "raw_bytes": [8192] * 3, "inspect_lengths": [5000] * 3, "adopted_bytes": 131072, "status": "ok",
                "failures": []}))
            sz.record_event(s, cid, "create_begin")
            sz.record_event(s, cid, "rm_done")
        for step, _phase in steps:
            cmd = ["env", "COUNTERFACTUAL_PATCH=/p/c", "TOOLING_PATCH=/p/t", "/repo/scripts/run-replay-offline.sh"] \
                if step.startswith("replay_") else ["/repo/scripts/finalize-stage2-evidence.sh"]
            sz.write_exclusive(s / "host" / f"{step}.json", sz.canonical_dumps({
                "schema": sz.HOST_RECORD_SCHEMA, "step": step, "rc": want.get(step, 0), "self_max_rss_bytes": 20 * MIB,
                "children_max_rss_bytes": 50 * MIB, "max_single_rss_bytes": 50 * MIB,
                "group_rss_peak_sampled_bytes": 60 * MIB, "group_samples": 3, "group_interval_ms": sz.HOST_INTERVAL_MS,
                "reaper": "subreaper", "all_descendants_reaped": True, "auto_reap_detected": False,
                "cleanup_complete": True, "leftover_pids": [], "errors": [],
                "env_keys": ["HOME", "PATH", "PYTHONDONTWRITEBYTECODE", "REPLAY_IMAGE_ID", "SIZING_STATE", "TMPDIR"],
                "cmd": cmd}))
        for phase, run_mb in (("success", 85), ("failure", 1), ("promote_success", 3), ("promote_failure", 3)):
            pdir = s / "phases" / phase
            pdir.mkdir(parents=True)
            locs = ["L1", "L2", "L3", "L5"] if phase in ("success", "failure") else ["L3", "L5", "L6"]
            sz.write_exclusive(pdir / "baseline.json", sz.canonical_dumps({
                "locations": dict.fromkeys(locs, 0), "L0_used": 10 << 30, "devices": dict.fromkeys(locs + ["L0", "L4"], "42")}))
            sample = {"t_ns": 1, "L0": (10 << 30) + run_mb * MIB, **dict.fromkeys(locs, 0)}
            sample[locs[0]] = run_mb * MIB
            (pdir / "samples.jsonl").write_text(json.dumps(sample) + "\n")
            sz.write_exclusive(pdir / "end.json", sz.canonical_dumps({
                "run_dir": {"allocated": run_mb * MIB, "dirs": 3}, "archive": {"allocated": 7 * MIB, "dirs": 4},
                "inventory_violations": []}))
        lines = []
        for rel in sz.SNAPSHOT_FILES["acceptance"]:
            data = subprocess.run([*GIT, "-C", str(self.repo), "cat-file", "blob", f"{commit}:{rel}"], check=True,
                                  capture_output=True).stdout
            if harness_patch and rel in harness_patch:
                data = harness_patch[rel]
            (s / "harness" / rel).parent.mkdir(parents=True, exist_ok=True)
            (s / "harness" / rel).write_bytes(data)
            lines.append(f"{hashlib.sha256(data).hexdigest()}  {rel}")
        (s / "harness" / "MANIFEST").write_text("\n".join(lines) + "\n")
        doc = sz.build_acceptance_precheck(s, p_b_budget=self.pf.P_B_BUDGET, drop_names=tuple(self.sup.ENV_DROP_NAMES),
                                           drop_prefixes=tuple(self.sup.ENV_DROP_PREFIXES))
        sz.write_exclusive(s / "precheck.json", sz.canonical_dumps(doc))
        return s

    def _cli(self, *args: str, helper: Path = SIZING) -> subprocess.CompletedProcess:
        return subprocess.run([sys.executable, str(helper), *args], capture_output=True, text=True,
                              env=dict(os.environ, PYTHONDONTWRITEBYTECODE="1"), timeout=300)

    def _verdict(self, raw: Path, expected: str, helper: Path = SIZING) -> subprocess.CompletedProcess:
        return self._cli("precheck-verdict", "--raw", str(raw), "--repo", str(self.repo), "--expected-repo-head", expected,
                         helper=helper)

    def _front(self, *markers: tuple[str, Path]) -> Path:
        """目前工作樹的 helper，但 recompute／產生演算法的入口會寫標記檔（frontend 本身照常）。"""
        source = SIZING.read_text(encoding="utf-8")
        for anchor, marker in markers:
            source = _marked(source, anchor, marker)
        root = self.tmp / "front"
        for rel in (self.sz.FROZEN_PREFLIGHT, self.sz.FROZEN_SUPERVISOR):      # 對照組（直接呼叫本地 recompute）要用
            (root / rel).parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(ROOT / rel, root / rel)
        front = root / self.sz.HELPER_REL
        front.parent.mkdir(parents=True, exist_ok=True)
        front.write_text(source, encoding="utf-8")
        return front

    def test_verdict_reads_shape_a_with_the_external_anchor(self) -> None:
        raw = self._raw("ok", self.c1)
        out = self._verdict(raw, self.c1)
        self.assertEqual((out.returncode, out.stdout.strip()), (0, "ok"), out.stderr)

    def test_anchor_must_be_a_full_commit_oid(self) -> None:
        raw = self._raw("ok", self.c1)
        self._git("tag", "-a", "v1", "-m", "t")
        tag = self._git("rev-parse", "v1")
        self.assertNotEqual(tag, self.c1)
        for bad in ("HEAD", self.c1[:12], tag, "0" * 40, self.c1.upper()):
            out = self._verdict(raw, bad)
            self.assertEqual(out.returncode, 1, bad)
            self.assertIn("無法判讀", out.stderr, bad)
        missing = self._cli("precheck-verdict", "--raw", str(raw), "--repo", str(self.repo))
        self.assertEqual(missing.returncode, 2)                         # 缺 --expected-repo-head → 用法錯誤

    def test_the_artifact_cannot_choose_the_commit(self) -> None:
        """第四輪 review：受信任 repo 同時有 C1 與 C2（C2 的 precheck-recompute 入口一被呼叫就寫標記檔）；artifact 與 C2
        完全一致、錨點是 C1 → 在取出或執行任何程式之前拒絕，標記檔⛔ 沒有出現。對照組：錨點是 C2 → 標記檔出現。"""
        marker = self.tmp / "c2-ran"
        helper = self.repo / self.sz.HELPER_REL
        helper.write_text(_marked(helper.read_text(encoding="utf-8"), RECOMPUTE_ANCHOR, marker), encoding="utf-8")
        c2 = self._commit("c2")
        raw = self._raw("c2", c2)
        out = self._verdict(raw, self.c1)
        self.assertEqual(out.returncode, 1)
        self.assertIn("identity.repo_head", out.stderr)
        self.assertFalse(marker.exists())
        control = self._verdict(raw, c2)                                 # 對照組：證明標記檔測得到執行
        self.assertEqual(control.returncode, 0, control.stderr)
        self.assertTrue(marker.exists())

    def test_the_working_tree_recompute_is_never_called(self) -> None:
        """第七輪 review：frontend 本來就由工作樹的 helper 執行；它的 recompute 入口寫標記檔——正常判讀⛔ 不得觸發它。"""
        raw = self._raw("ok", self.c1)
        marker = self.tmp / "front-ran"
        front = self._front((RECOMPUTE_ANCHOR, marker))
        out = self._verdict(raw, self.c1, helper=front)
        self.assertEqual((out.returncode, out.stdout.strip()), (0, "ok"), out.stderr)
        self.assertFalse(marker.exists())
        direct = self._cli("precheck-recompute", "--raw", str(raw), "--trusted-root", str(self.tmp / "front"), helper=front)
        self.assertEqual(direct.returncode, 0, direct.stderr)                # 對照組：直接呼叫才會執行本地的 recompute
        self.assertTrue(marker.exists())
        refused = self._cli("precheck-recompute", "--raw", str(raw), "--trusted-root", str(self.tmp))
        self.assertEqual(refused.returncode, 1)
        self.assertIn("內部命令只能由", refused.stderr)

    def test_code_in_the_raw_snapshot_is_never_imported(self) -> None:
        """第三輪 review：原始量測的快照放一份與 MANIFEST、identity 一致、但 import 就寫標記檔的 preflight → 無法判讀、⛔ 沒有執行。"""
        marker = self.tmp / "raw-imported"
        evil = (ROOT / self.sz.FROZEN_PREFLIGHT).read_bytes() + f"\nopen({str(marker)!r}, 'w').write('x')\n".encode()
        raw = self._raw("evil", self.c1, harness_patch={self.sz.FROZEN_PREFLIGHT: evil})
        out = self._verdict(raw, self.c1)
        self.assertEqual(out.returncode, 1)
        self.assertIn(self.sz.FROZEN_PREFLIGHT, out.stderr)
        self.assertFalse(marker.exists())
        one_byte = self._raw("byte", self.c1, harness_patch={
            self.sz.HELPER_REL: (ROOT / self.sz.HELPER_REL).read_bytes() + b"#"})
        self.assertEqual(self._verdict(one_byte, self.c1).returncode, 1)

    def test_raw_manifest_runs_only_the_anchored_algorithm(self) -> None:
        """第六、七輪 review：工作樹與 C2 的 manifest 產生演算法入口都會寫標記檔；錨點 ＝ C1 → 只執行 C1 取出的版本。"""
        raw = self._raw("m", self.c1)
        front_marker, c2_marker = self.tmp / "front-manifest", self.tmp / "c2-manifest"
        helper = self.repo / self.sz.HELPER_REL
        helper.write_text(_marked(helper.read_text(encoding="utf-8"), MANIFEST_ANCHOR, c2_marker), encoding="utf-8")
        c2 = self._commit("c2")
        front = self._front((MANIFEST_ANCHOR, front_marker))
        out_path = self.tmp / "m.json"
        out = self._cli("raw-manifest", "--raw", str(raw), "--repo", str(self.repo), "--expected-repo-head", self.c1,
                        "--out", str(out_path), helper=front)
        self.assertEqual(out.returncode, 0, out.stderr)
        self.assertFalse(front_marker.exists() or c2_marker.exists())
        data = out_path.read_bytes()
        self.assertEqual(data, self.sz.build_raw_manifest(raw, self.c1))     # ＝ C1 的演算法
        summary = json.loads(out.stdout)
        self.assertEqual(summary["sha256"], hashlib.sha256(data).hexdigest())
        check = self._cli("raw-manifest", "--raw", str(raw), "--repo", str(self.repo), "--expected-repo-head", self.c1,
                          "--check-sha256", summary["sha256"], helper=front)
        self.assertEqual(check.returncode, 0, check.stderr)
        (raw / "extra").write_text("x")
        again = self.tmp / "again.json"
        bad = self._cli("raw-manifest", "--raw", str(raw), "--repo", str(self.repo), "--expected-repo-head", self.c1,
                        "--check-sha256", summary["sha256"], "--out", str(again), helper=front)
        self.assertEqual(bad.returncode, 1)
        self.assertFalse(again.exists())                                     # 不符 → ⛔ 不寫任何檔案
        control = self._cli("raw-manifest", "--raw", str(raw), "--repo", str(self.repo), "--expected-repo-head", c2,
                            "--out", str(self.tmp / "c2.json"), helper=front)
        self.assertEqual(control.returncode, 0, control.stderr)             # 對照組：錨點 ＝ C2 才會執行 C2 的演算法
        self.assertTrue(c2_marker.exists() and not front_marker.exists())
        no_repo = self._cli("raw-manifest", "--raw", str(raw), "--expected-repo-head", self.c1, "--out", str(self.tmp / "x"))
        self.assertEqual(no_repo.returncode, 2)
        self._git("tag", "-a", "v1", "-m", "t")
        for bad_anchor in ("HEAD", self.c1[:12], "0" * 40, self._git("rev-parse", "v1")):     # 最後一個是 tag object
            res = self._cli("raw-manifest", "--raw", str(raw), "--repo", str(self.repo), "--expected-repo-head", bad_anchor,
                            "--out", str(self.tmp / "y"))
            self.assertEqual(res.returncode, 1, bad_anchor)
            self.assertFalse((self.tmp / "y").exists())

    def _env_cli(self, extra: dict, *args: str) -> subprocess.CompletedProcess:
        env = dict(os.environ, PYTHONDONTWRITEBYTECODE="1", **extra)
        return subprocess.run([sys.executable, str(SIZING), *args], capture_output=True, text=True, env=env, timeout=300)

    def test_git_ignores_the_callers_path_and_git_environment(self) -> None:
        """實作第一輪 review（高）：offline 讀取端的 git 固定是 /usr/bin/git、子程序一律是最小化的環境——PATH 上的假 git
        ⛔ 不被執行；GIT_DIR／GIT_OBJECT_DIRECTORY ⛔ 不能把查詢導向別的 repo，也⛔ 不能替不是 repo 的 --repo 撐腰；
        --repo 是子目錄時⛔ 不往上找到別的 repo。"""
        raw = self._raw("ok", self.c1)
        marker = self.tmp / "fake-git-ran"
        fakebin = self.tmp / "fakebin"
        fakebin.mkdir()
        (fakebin / "git").write_text(f"#!/bin/sh\necho x > {marker}\necho {self.c1}\nexit 0\n")
        (fakebin / "git").chmod(0o755)
        other = self.tmp / "other"
        subprocess.run([*GIT, "init", "-q", str(other)], check=True)
        verdict = ("precheck-verdict", "--raw", str(raw), "--repo", str(self.repo), "--expected-repo-head", self.c1)
        fake_path = {"PATH": f"{fakebin}:{os.environ['PATH']}"}
        out = self._env_cli(fake_path, *verdict)
        self.assertEqual((out.returncode, out.stdout.strip()), (0, "ok"), out.stderr)
        out = self._env_cli(fake_path, "raw-manifest", "--raw", str(raw), "--repo", str(self.repo), "--expected-repo-head",
                            self.c1, "--out", str(self.tmp / "m.json"))
        self.assertEqual(out.returncode, 0, out.stderr)
        self.assertFalse(marker.exists())                                    # 假 git ⛔ 沒有被執行
        for var, value in (("GIT_DIR", str(other / ".git")), ("GIT_OBJECT_DIRECTORY", str(other / ".git" / "objects")),
                           ("GIT_ALTERNATE_OBJECT_DIRECTORIES", str(other / ".git" / "objects"))):
            out = self._env_cli({var: value}, *verdict)
            self.assertEqual((out.returncode, out.stdout.strip()), (0, "ok"), (var, out.stderr))
        plain = self.tmp / "plain"
        plain.mkdir()
        out = self._env_cli({"GIT_DIR": str(self.repo / ".git")}, "precheck-verdict", "--raw", str(raw), "--repo", str(plain),
                            "--expected-repo-head", self.c1)
        self.assertEqual(out.returncode, 1, out.stdout)                     # ⛔ 不是 repo 的 --repo：GIT_DIR ⛔ 不能撐腰
        out = self._env_cli({}, "precheck-verdict", "--raw", str(raw), "--repo", str(self.repo / "scripts"),
                            "--expected-repo-head", self.c1)
        self.assertEqual(out.returncode, 1, out.stdout)                     # 子目錄：⛔ 不往上找

    def test_offline_children_get_a_minimal_environment(self) -> None:
        """實作第一輪 review（高）：git 與取出的 helper 這兩種子程序都⛔ 不繼承呼叫端的 GIT_*、LD_*、PYTHON*、PATH。"""
        raw = self._raw("ok", self.c1)
        calls = []
        real_run = subprocess.run

        def spy(argv, *args, **kwargs):
            calls.append((list(argv), kwargs.get("env")))
            return real_run(argv, *args, **kwargs)

        polluted = {"GIT_DIR": "/nowhere", "GIT_TRACE": "1", "LD_I074_PROBE": "1", "PYTHONPATH": "/evil", "PATH": "/evil:/usr/bin:/bin"}
        with mock.patch.dict(os.environ, polluted), mock.patch.object(self.sz.subprocess, "run", spy):
            self.assertEqual(self.sz.precheck_verdict(raw, self.repo, self.c1), 0)
        self.assertTrue(any(argv[0] == "/usr/bin/git" for argv, _ in calls))
        self.assertTrue(any("-I" in argv for argv, _ in calls))
        allowed_git = {"GIT_CONFIG_NOSYSTEM", "GIT_NO_REPLACE_OBJECTS", "GIT_TERMINAL_PROMPT", "GIT_CEILING_DIRECTORIES"}
        for argv, env in calls:
            self.assertIsNotNone(env, argv)
            self.assertNotEqual(argv[0], "git", argv)                        # ⛔ 不從 PATH 找 git
            self.assertEqual(env["PATH"], "/usr/bin:/bin")
            self.assertFalse({k for k in env if k.startswith(("LD_", "PYTHON"))}, argv)
            self.assertFalse({k for k in env if k.startswith("GIT_")} - allowed_git, argv)

    def _live_state(self, profile: str) -> tuple[Path, Path]:
        """live 的 S（快照 ＋ MANIFEST）與工作複本，兩者的檔案都取自目前的 repo。"""
        s, clone = self.tmp / "S", self.tmp / "clone"
        lines = []
        for rel in self.sz.SNAPSHOT_FILES[profile]:
            for root in (s / "harness", clone):
                (root / rel).parent.mkdir(parents=True, exist_ok=True)
                shutil.copyfile(ROOT / rel, root / rel)
            lines.append(f"{hashlib.sha256((ROOT / rel).read_bytes()).hexdigest()}  {rel}")
        (s / "harness" / "MANIFEST").write_text("\n".join(lines) + "\n")
        return s, clone

    def test_live_constants_come_from_the_snapshot(self) -> None:
        """「權威常數的來源」：live 一律取自 S 的快照（先驗 MANIFEST），並斷言與工作複本逐位元相同。"""
        for profile in ("sizing", "acceptance"):
            with self.subTest(profile=profile):
                shutil.rmtree(self.tmp / "S", True)
                shutil.rmtree(self.tmp / "clone", True)
                s, clone = self._live_state(profile)
                consts = self.sz.live_constants(s, clone, profile)
                self.assertEqual(consts["p_b_budget"], self.pf.P_B_BUDGET)
                if profile == "acceptance":
                    self.assertEqual(set(consts["drop_names"]), set(self.sup.ENV_DROP_NAMES))
                    self.assertEqual(consts["drop_prefixes"], tuple(self.sup.ENV_DROP_PREFIXES))
                else:
                    self.assertEqual(set(consts), {"p_b_budget"})
                rel = self.sz.FROZEN_SUPERVISOR if profile == "acceptance" else self.sz.FROZEN_PREFLIGHT
                with open(clone / rel, "a") as fh:                       # 工作複本與快照不同 → fail-closed
                    fh.write("\n# drift\n")
                with self.assertRaisesRegex(self.sz.SizingError, "工作複本"):
                    self.sz.live_constants(s, clone, profile)
                shutil.copyfile(s / "harness" / rel, clone / rel)
                with open(s / "harness" / rel, "a") as fh:               # 快照與 MANIFEST 不符 → fail-closed
                    fh.write("\n# tampered\n")
                shutil.copyfile(s / "harness" / rel, clone / rel)
                with self.assertRaisesRegex(self.sz.SizingError, "MANIFEST"):
                    self.sz.live_constants(s, clone, profile)


if __name__ == "__main__":
    unittest.main()
