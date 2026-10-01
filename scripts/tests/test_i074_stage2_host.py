"""I-074 Stage 2 ⑦b：host 端的單元測試（Python 3.9；issue.md I-074「Stage 2 步驟 ⑦b 細部計畫 v1」「五」）。

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
import shutil
import signal
import stat
import subprocess
import sys
import tempfile
import textwrap
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
            p = subprocess.Popen(["/bin/sh", "-c", "setsid sleep 30 & exit 0"])
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


if __name__ == "__main__":
    unittest.main()
