"""I-074 Stage 2 ⑦c：晉升（`python/scripts/i074_stage2_promote.py`）。

對應 issue.md I-074「Stage 2 步驟 ⑦c 細部計畫 v1」「五」的 n11、路徑錨定、結束碼的分類、ignore 守門（注入的 git
runner——⚠️ 測試 image 沒有 git；真 git 的案例在 `scripts/test-i074-stage2.sh`）、操作契約與收尾重算、結束碼常數。
⚠️ 驗證模式由 fake 代替（它讀 staging／目的地的 manifest 或 record，回報**它讀到的** SHA）；驗證模式本身的
Python 段在 `test_replay_stage2_archive.py`，shell 段在 `scripts/test-replay-args.sh`。
"""
from __future__ import annotations

import errno
import gzip
import hashlib
import importlib.util
import io
import json
import os
import shutil
import socket
import stat
import subprocess
import sys
from pathlib import Path

import pytest

from ..replay_bundle import publish as rb_publish
from ..replay_bundle import stage2_archive as sa
from ..replay_bundle.canonical import canonical_json_bytes

_SCRIPTS = Path(__file__).resolve().parents[4] / "scripts"
sys.path.insert(0, str(_SCRIPTS))


def _load(name: str):
    spec = importlib.util.spec_from_file_location(name, _SCRIPTS / f"{name}.py")
    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module          # ⚠️ dataclass 解析延後的註解時要從 sys.modules 找到模組
    spec.loader.exec_module(module)
    return module


pm = _load("i074_stage2_promote")
STAGE2_REL = pm.STAGE2_REL
BUNDLE = "b1_test"
SEM = "5" * 64
REPO_HEAD = "a" * 40
IDENTITY = {"bundle_id": BUNDLE, "created_at": "2026-09-30T00:00:00+00:00", "expected_image_id": "sha256:" + "1" * 64,
            "kind": "sr_zone_run_identity", "schema_version": 1}
IDENTITY_SHA = hashlib.sha256(canonical_json_bytes(IDENTITY)).hexdigest()
FAILED_NAME = f"{BUNDLE}-{SEM}"


def _gz(data: bytes) -> bytes:
    buf = io.BytesIO()
    with gzip.GzipFile(filename="", mode="wb", fileobj=buf, mtime=0) as fh:
        fh.write(data)
    return buf.getvalue()


def _sha(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def _tree(root: Path) -> dict:
    """marker 的 inventory：路徑 → (型別, bytes, inode, mtime_ns)。⛔ 不跟隨 symlink。"""
    out = {}
    for dirpath, dirnames, filenames in os.walk(root, followlinks=False):
        for name in dirnames + filenames:
            p = Path(dirpath) / name
            st = p.lstat()
            out[str(p.relative_to(root))] = (stat.S_IFMT(st.st_mode), p.read_bytes() if p.is_file() and not p.is_symlink()
                                             else None, st.st_ino, st.st_mtime_ns)
    return out


class World:
    """合成的 <work>／複本／真正 repo ＋ fake git ＋ fake 驗證模式。"""

    def __init__(self, tmp: Path, target: str = "evidence") -> None:
        self.tmp = tmp
        self.work = tmp / "work"
        self.clone = self.work / "repo"
        self.real = tmp / "real"
        self.target = target
        self.clone_i074 = self.clone / STAGE2_REL
        self.real_i074 = self.real / STAGE2_REL
        for d in (self.clone_i074 / "envcheck", self.real_i074 / "envcheck", self.real / ".git"):
            d.mkdir(parents=True)
        (self.clone_i074 / "envcheck" / "tracked.json").write_text("{}", encoding="utf-8")
        (self.real_i074 / "envcheck" / "tracked.json").write_text("{}", encoding="utf-8")
        (self.real / ".git" / "HEAD").write_text("ref: refs/heads/main\n", encoding="utf-8")
        self.tracked = {f"{STAGE2_REL}/envcheck/tracked.json"}
        self.marker = tmp / "marker"
        (self.marker / "failed").mkdir(parents=True)
        (self.marker / "keep.txt").write_text("marker", encoding="utf-8")
        self.states = {"run": {"repo_head": REPO_HEAD},
                       "preflight": {"bundle_id": BUNDLE, "counterfactual_semantic_sha256": SEM,
                                     "identity_sha256": IDENTITY_SHA},
                       "replay_done": {"rc": 0 if target == "evidence" else 6}}
        self.git_calls = []
        self.verify_calls = []
        self.ignored = lambda path: path.startswith(f"{STAGE2_REL}/.promote-staging-")   # ＝ .gitignore 的規則
        self.check_ignore_override = None
        self.verify_rc = 0
        self.verify_hook = None
        self.verify_override = {}
        self.verify_text = None
        self.hooks = {}
        if target == "evidence":
            self.make_evidence(self.clone_i074 / "evidence")
        else:
            self.make_failed(self.clone_i074 / "failed" / FAILED_NAME)

    @staticmethod
    def make_evidence(root: Path) -> None:
        files = {
            "evidence_manifest.json": canonical_json_bytes({"kind": "manifest", "n": 1}),
            sa.IDENTITY: _gz(canonical_json_bytes(IDENTITY)),
            sa.BEFORE_SOURCE: _gz(b"before" * 1000),
            sa.COMPARISON: _gz(b"comparison"),
            sa.REPORT: _gz(b"report"),
            sa.COUNTERFACTUAL_PATCH: b"cf\n",
            sa.TOOLING_PATCH: b"tool\n",
        }
        for rel, data in files.items():
            (root / rel).parent.mkdir(parents=True, exist_ok=True)
            (root / rel).write_bytes(data)

    @staticmethod
    def make_failed(root: Path) -> None:
        root.mkdir(parents=True)
        (root / sa.FAILED_RECORD_NAME).write_bytes(canonical_json_bytes({"run_identity": IDENTITY, "x": 1}))
        (root / "patch").mkdir()
        (root / sa.COUNTERFACTUAL_PATCH).write_bytes(b"cf\n")
        (root / sa.TOOLING_PATCH).write_bytes(b"tool\n")

    @property
    def terminal(self) -> Path:
        return self.clone_i074 / ("evidence" if self.target == "evidence" else f"failed/{FAILED_NAME}")

    @property
    def dest(self) -> Path:
        return self.real_i074 / ("evidence" if self.target == "evidence" else f"failed/{FAILED_NAME}")

    def git(self, argv, stdin=None, timeout=None):
        self.git_calls.append((list(argv), stdin))
        if argv[1] == str(self.clone) and argv[2:4] == ["worktree", "prune"]:
            return 0, b""
        if argv[1] == str(self.clone) and argv[2] == "status":
            out = b""
            for dirpath, dirnames, filenames in os.walk(self.clone_i074, followlinks=False):
                for name in sorted(filenames + [d for d in dirnames if (Path(dirpath) / d).is_symlink()]):
                    rel = str((Path(dirpath) / name).relative_to(self.clone))
                    if rel not in self.tracked:
                        out += b"?? " + rel.encode() + b"\x00"
            return 0, out
        if argv[1] == str(self.real) and argv[2:] == ["check-ignore", "--no-index", "-z", "--stdin"]:
            if self.check_ignore_override is not None:
                return self.check_ignore_override(stdin)
            paths = [p.decode() for p in stdin.split(b"\x00") if p]
            hit = [p for p in paths if self.ignored(p)]
            return (0 if hit else 1), b"".join(p.encode() + b"\x00" for p in hit)
        raise AssertionError(f"非預期的 git 呼叫：{argv}")

    def verify(self, clone, path, target):
        self.verify_calls.append((path, target))
        if self.verify_hook is not None:
            self.verify_hook(Path(path))
        if self.verify_text is not None:
            return 0, self.verify_text.encode()
        if self.verify_rc != 0:
            return self.verify_rc, b""
        name = "evidence_manifest.json" if target == "evidence" else sa.FAILED_RECORD_NAME
        doc = {"kind": "evidence" if target == "evidence" else "failed_record", "target": target,
               ("manifest_sha256" if target == "evidence" else "record_sha256"): _sha(Path(path) / name),
               "base_commit": REPO_HEAD, "identity_sha256": IDENTITY_SHA}
        doc.update(self.verify_override)
        return 0, canonical_json_bytes(doc) + b"\n"

    def promoter(self):
        return pm.Promoter(str(self.work), str(self.real), git=self.git, verify=self.verify,
                           load_states=lambda w, r: self.states, hooks=self.hooks)

    def run(self) -> int:
        try:
            return self.promoter().run()
        except pm.PromotionExit as exc:
            return exc.code

    def stagings(self):
        return sorted(p.name for p in self.real_i074.iterdir() if p.name.startswith(".promote-staging-"))


@pytest.fixture
def world(tmp_path):
    return World(tmp_path)


@pytest.fixture
def failed_world(tmp_path):
    return World(tmp_path, target="failed")


@pytest.fixture
def fsyncs(monkeypatch):
    """記下每一個被 fsync 的 fd 的 (st_dev, st_ino)——分辨來源與目的地（第七輪 review）。"""
    seen = []
    real = pm._fsync

    def spy(fd):
        st = os.fstat(fd)
        seen.append((st.st_dev, st.st_ino))
        return real(fd)

    monkeypatch.setattr(pm, "_fsync", spy)
    return seen


def _idents(root: Path) -> set:
    out = {(root.lstat().st_dev, root.lstat().st_ino)}
    for p in root.rglob("*"):
        st = p.lstat()
        out.add((st.st_dev, st.st_ino))
    return out


def _same_bytes(a: Path, b: Path) -> bool:
    fa = sorted(str(p.relative_to(a)) for p in a.rglob("*"))
    fb = sorted(str(p.relative_to(b)) for p in b.rglob("*"))
    return fa == fb and all((a / r).read_bytes() == (b / r).read_bytes() for r in fa if (a / r).is_file())


# ── 正向、冪等 ───────────────────────────────────────────────────────────────

def test_evidence_6b_then_6a_is_idempotent(world, fsyncs):
    before = {str(p.relative_to(world.terminal)): (p.read_bytes(), p.stat().st_ino) for p in world.terminal.rglob("*")
              if p.is_file()}
    assert world.run() == 0
    assert _same_bytes(world.terminal, world.dest) and world.stagings() == []
    after = {str(p.relative_to(world.terminal)): (p.read_bytes(), p.stat().st_ino) for p in world.terminal.rglob("*")
             if p.is_file()}
    assert after == before                                            # ⛔ 不改複本內的終態（只 fsync）
    assert _idents(world.terminal) <= set(fsyncs)                      # 第 4 步：來源的每個檔案與目錄
    inode = world.dest.stat().st_ino
    fsyncs.clear()
    assert world.run() == 0                                           # 重跑走 6a：只補 fsync
    assert world.dest.stat().st_ino == inode and _idents(world.dest) <= set(fsyncs)
    assert [c[1] for c in world.verify_calls] == ["evidence", "evidence"]


def test_failed_record_creates_failed_dir_and_returns_6(failed_world):
    w = failed_world
    assert not (w.real_i074 / "failed").exists()
    assert w.run() == 6
    assert _same_bytes(w.terminal, w.dest) and w.stagings() == []


def test_only_verification_and_trusted_git_are_called(world):
    """③ 的 recovery、replay、finalize、publish ⛔ 不被呼叫：外部呼叫只有驗證模式與三種 git 子指令。"""
    assert world.run() == 0
    kinds = {tuple(a[2:4]) if a[2] != "status" else ("status",) for a, _ in world.git_calls}
    assert kinds == {("worktree", "prune"), ("status",), ("check-ignore", "--no-index")}
    assert all(a[2:] == ["check-ignore", "--no-index", "-z", "--stdin"] and "-v" not in a
               for a, _ in world.git_calls if a[2] == "check-ignore")
    assert len(world.verify_calls) == 1 and world.verify_calls[0][0].startswith(str(world.real_i074) + "/.promote-staging-")
    assert (world.real / ".git" / "HEAD").read_text() == "ref: refs/heads/main\n"


def test_probes_are_member_paths(world):
    assert world.run() == 0
    stdins = [s.decode() for a, s in world.git_calls if a[2] == "check-ignore"]
    dest_probe, staging_probe = (sorted(p for p in s.split("\x00") if p) for s in stdins)
    assert dest_probe == sorted(f"{STAGE2_REL}/evidence/{r}" for r in
                                ("evidence_manifest.json", sa.IDENTITY, sa.BEFORE_SOURCE, sa.COMPARISON, sa.REPORT,
                                 sa.COUNTERFACTUAL_PATCH, sa.TOOLING_PATCH))
    assert all(p.startswith(f"{STAGE2_REL}/.promote-staging-") and p.count("/") >= 4 for p in staging_probe)


# ── 第 2 步 ──────────────────────────────────────────────────────────────────

def test_step2_cleans_only_recognizable_orphan_staging(world):
    orphan = world.real_i074 / ".promote-staging-0123456789abcdef"
    (orphan / "sub").mkdir(parents=True)
    (orphan / "sub" / "f").write_text("x", encoding="utf-8")
    (orphan / "link").symlink_to(world.marker)                     # ⚠️ 只刪連結本身，⛔ 不跟著刪 marker
    (world.real_i074 / ".promote-staging-fedcba9876543210").symlink_to(world.marker / "keep.txt")
    (world.real_i074 / ".promote-staging-short").mkdir()
    (world.real_i074 / "other.txt").write_text("keep", encoding="utf-8")
    marker = _tree(world.marker)
    assert world.run() == 0
    assert world.stagings() == [".promote-staging-short"]
    assert (world.real_i074 / "other.txt").read_text() == "keep" and _tree(world.marker) == marker


def test_step2_prune_failure_is_9(world):
    real_git = world.git
    world.git = lambda argv, stdin=None, timeout=None: (1, b"") if "prune" in argv else real_git(argv, stdin, timeout)
    assert world.run() == 9


# ── 第 3 步：唯一終態 ─────────────────────────────────────────────────────────

def test_step3_zero_groups_is_9(world):
    shutil.rmtree(world.terminal)
    assert world.run() == 9 and not world.dest.exists()


def test_step3_two_groups_is_9(world):
    World.make_failed(world.clone_i074 / "failed" / FAILED_NAME)
    assert world.run() == 9 and not world.dest.exists()


@pytest.mark.parametrize("stray", ["stray.txt", "failed/loose.txt", ".promote-staging-0123456789abcdef/x"])
def test_step3_unrecognized_is_9(world, stray):
    (world.clone_i074 / stray).parent.mkdir(parents=True, exist_ok=True)
    (world.clone_i074 / stray).write_text("x", encoding="utf-8")
    assert world.run() == 9


def test_step3_failed_name_mismatch_is_9(failed_world):
    w = failed_world
    w.states["preflight"]["counterfactual_semantic_sha256"] = "6" * 64
    assert w.run() == 9


def test_step3_replay_rc_must_match_the_terminal(world):
    world.states["replay_done"]["rc"] = 6
    assert world.run() == 9
    del world.states["replay_done"]                                   # 沒有 replay_done（finalize 之前就中止的 --promote）
    assert world.run() == 0


def test_step3_ignores_and_keeps_recognizable_orphans_in_the_clone(world):
    for rel in (".evidence.staging-0123456789abcdef/x", ".probe-0123456789abcdef-a-dst/marker",
                f"failed/.{FAILED_NAME}.staging-0123456789abcdef/x", "failed/.probe-0123456789abcdef-b-src/marker"):
        (world.clone_i074 / rel).parent.mkdir(parents=True, exist_ok=True)
        (world.clone_i074 / rel).write_text("o", encoding="utf-8")
    assert world.run() == 0
    assert (world.clone_i074 / ".evidence.staging-0123456789abcdef/x").read_text() == "o"   # ⛔ 不刪除


def test_listing_must_match_the_inventory(world, monkeypatch):
    world.tracked.add(f"{STAGE2_REL}/evidence/{sa.REPORT}")          # git 說它是追蹤檔、inventory 看得到
    assert world.run() == 9


# ── 3b：封閉 inventory ───────────────────────────────────────────────────────

@pytest.fixture
def opens(monkeypatch):
    """記下每一個 os.open 成功開出的 fd 的 (st_dev, st_ino)。"""
    seen = []
    real = os.open

    def spy(*a, **k):
        fd = real(*a, **k)
        st = os.fstat(fd)
        seen.append((st.st_dev, st.st_ino))
        return fd

    monkeypatch.setattr(pm.os, "open", spy)
    return seen


@pytest.mark.parametrize("which", ["member", "identity"])
def test_3b_symlink_to_outside_is_9_and_never_opened(world, opens, fsyncs, which):
    outside = world.marker / "outside.bin"
    outside.write_bytes(_gz(canonical_json_bytes(IDENTITY)))
    rel = sa.REPORT if which == "member" else sa.IDENTITY
    (world.terminal / rel).unlink()
    (world.terminal / rel).symlink_to(outside)
    marker = _tree(world.marker)
    assert world.run() == 9
    target = (outside.stat().st_dev, outside.stat().st_ino)
    assert target not in opens and fsyncs == []                       # ⛔ 不開啟、整趟⛔ 沒有任何 fsync（第 4 步之前）
    assert _tree(world.marker) == marker and world.stagings() == [] and not world.dest.exists()


def test_3b_fifo_is_9(world, fsyncs):
    os.mkfifo(world.terminal / "comparison" / "fifo")
    assert world.run() == 9 and fsyncs == []


def test_3b_socket_is_9(world, fsyncs, monkeypatch):
    sock = socket.socket(socket.AF_UNIX)
    monkeypatch.chdir(world.terminal)
    try:
        sock.bind("sock")
        assert world.run() == 9 and fsyncs == []
    finally:
        sock.close()


def test_3b_hardlink_is_9_and_the_other_inode_is_never_opened_or_fsynced(world, opens, fsyncs):
    os.link(world.terminal / sa.REPORT, world.marker / "hard")
    ident = ((world.marker / "hard").stat().st_dev, (world.marker / "hard").stat().st_ino)
    assert world.run() == 9
    assert ident not in opens and ident not in fsyncs


# ── 3c：identity ─────────────────────────────────────────────────────────────

def test_3c_identity_mismatch_is_9(world):
    world.states["preflight"]["identity_sha256"] = "7" * 64
    assert world.run() == 9


def test_3c_failed_record_identity_mismatch_is_9(failed_world):
    rec = failed_world.terminal / sa.FAILED_RECORD_NAME
    rec.write_bytes(canonical_json_bytes({"run_identity": dict(IDENTITY, created_at="x"), "x": 1}))
    assert failed_world.run() == 9


# ── 第 4 步：結束碼的分類 ────────────────────────────────────────────────────

def _drift(kind):
    def mutate(w):
        target = w.terminal / sa.REPORT
        if kind == "deleted":
            target.unlink()                                   # ENOENT
        elif kind == "symlink":
            target.unlink()
            target.symlink_to(w.marker / "keep.txt")          # ELOOP
        elif kind == "parent_replaced":
            shutil.rmtree(w.terminal / "comparison")
            (w.terminal / "comparison").write_text("file")    # ENOTDIR
        else:
            fresh = target.with_name(target.name + ".new")
            fresh.write_bytes(target.read_bytes())            # 先建新檔再換名：保證是不同的 inode（⛔ 不讓 inode 被重用）
            os.replace(fresh, target)                         # 同名、同內容、不同 inode
    return mutate


@pytest.mark.parametrize("kind", ["deleted", "symlink", "parent_replaced", "inode_replaced"])
def test_step4_drift_after_inventory_is_9(world, kind):
    world.hooks["after_inventory"] = lambda: _drift(kind)(world)
    assert world.run() == 9 and world.stagings() == [] and not world.dest.exists()


def test_step4_fsync_failure_is_3_and_no_staging(world, monkeypatch):
    def failing(fd):
        raise OSError(errno.EIO, "injected")

    monkeypatch.setattr(pm, "_fsync", failing)
    assert world.run() == 3 and world.stagings() == [] and not world.dest.exists()


# ── 第 5 步與路徑錨定 ────────────────────────────────────────────────────────

def test_step5_failed_dir_symlink_to_marker_is_9(failed_world):
    w = failed_world
    (w.real_i074 / "failed").symlink_to(w.marker / "failed")
    marker = _tree(w.marker)
    assert w.run() == 9 and _tree(w.marker) == marker


@pytest.mark.parametrize("shape", ["file", "symlink"])
def test_step5_destination_of_wrong_type_is_9(world, shape):
    if shape == "file":
        world.dest.write_text("x")
    else:
        world.dest.symlink_to(world.marker)
    marker = _tree(world.marker)
    assert world.run() == 9 and _tree(world.marker) == marker


@pytest.mark.parametrize("layer", ["python/baselines", STAGE2_REL])
def test_middle_layer_symlink_to_marker_is_9(world, layer):
    moved = world.tmp / "moved_layer"
    shutil.move(str(world.real / layer), str(moved))
    shutil.copytree(moved, world.marker / "mirror")
    (world.real / layer).symlink_to(world.marker / "mirror")
    marker = _tree(world.marker)
    assert world.run() == 9 and _tree(world.marker) == marker


def _barrier_move(w, layer: str, where: str):
    """驗證之後、rename 之前（鏈檢查之後）：把 `layer` 搬走（或 rm -rf）、換成指向 marker 的 symlink。"""
    def hook():
        src = w.real_i074 if layer == "i074_stage2" else w.real_i074 / "failed"
        if where == "repo":
            shutil.move(str(src), str(src.parent / (src.name + ".moved")))
        elif where == "outside":
            shutil.move(str(src), str(w.tmp / "outside_moved"))
        else:
            shutil.rmtree(src)
        src.symlink_to(w.marker / ("failed" if layer == "failed" else ""))
    return hook


@pytest.mark.parametrize("where", ["repo", "outside", "rmrf"])
def test_barrier_failed_parent_replaced_before_rename(failed_world, where):
    w = failed_world
    (w.real_i074 / "failed").mkdir()                                  # 已存在的 failed/（持有它的 fd）
    w.hooks["before_rename"] = _barrier_move(w, "failed", where)
    marker = _tree(w.marker)
    assert w.run() == 9
    assert _tree(w.marker) == marker                                  # ⚠️ symlink 指向的目標⛔ 不被寫入
    if where == "repo":       # 照實：record 落在被搬走的舊目錄（repo 內）
        assert _same_bytes(w.terminal, w.real_i074 / "failed.moved" / FAILED_NAME)
    elif where == "outside":  # 照實：被搬到 repo 外就是寫到 repo 外——操作契約禁止的情況，事後只能 9
        assert _same_bytes(w.terminal, w.tmp / "outside_moved" / FAILED_NAME)
    else:                     # rm -rf：rename 進已刪除的目錄 → ENOENT、⛔ 沒有 record、staging 已清
        assert w.stagings() == [] and not (w.tmp / "outside_moved").exists()


def test_barrier_i074_replaced_before_rename(world):
    world.hooks["before_rename"] = _barrier_move(world, "i074_stage2", "repo")
    marker = _tree(world.marker)
    assert world.run() == 9
    assert _tree(world.marker) == marker
    assert _same_bytes(world.terminal, world.real_i074.parent / "i074_stage2.moved" / "evidence")


def test_barrier_staging_swapped_during_verification_is_never_renamed(world):
    seen = {}

    def swap(path):
        shutil.move(str(path), str(world.tmp / "orig_staging"))
        path.mkdir()
        (path / "intruder").write_text("i")
        shutil.copy(world.tmp / "orig_staging" / "evidence_manifest.json", path)   # 讓 fake 驗證模式讀得到
        seen.update(path=path, ino=path.stat().st_ino)

    world.verify_hook = swap
    assert world.run() == 9
    assert not world.dest.exists()                                    # 被換進來的目錄⛔ 沒有被 rename
    # ⚠️ 實作第一輪 review（中）：錯誤清理⛔ 不得刪掉不屬於自己的 inode——被換進來的目錄原封不動
    assert seen["path"].is_dir() and seen["path"].stat().st_ino == seen["ino"]
    assert (seen["path"] / "intruder").read_text() == "i"


@pytest.mark.parametrize("how", ["moved", "deleted"])
def test_impl_r2_staging_gone_during_write_is_9_not_8(world, monkeypatch, how):
    """⚠️ 實作第二輪 review（中）：staging 在寫入途中被搬走／刪掉、寫入再回 ENOSPC——名稱不見⛔ 不等於「已清除」，
    ⛔ 不得沿用代表「可安全重跑」的 8；搬走的目錄原封不動。"""
    seen = {}

    def failing(fd, data):
        if not seen:
            staging = world.real_i074 / world.stagings()[0]
            if how == "moved":
                moved = world.tmp / "moved_staging"
                shutil.move(str(staging), str(moved))
                seen.update(path=moved, ino=moved.stat().st_ino, tree=_tree(moved))
            else:
                shutil.rmtree(staging)
                seen.update(path=None)
        raise OSError(errno.ENOSPC, "injected")

    monkeypatch.setattr(pm, "_write", failing)
    assert world.run() == 9
    assert world.stagings() == [] and not world.dest.exists()
    if how == "moved":
        assert seen["path"].is_dir() and seen["path"].stat().st_ino == seen["ino"]
        assert _tree(seen["path"]) == seen["tree"]


def _fail_first_write(monkeypatch, state):
    def failing(fd, data):
        state["failed"] = True
        raise OSError(errno.ENOSPC, "injected")

    monkeypatch.setattr(pm, "_write", failing)


def _swap_staging(world, how, seen):
    """把本次的 staging 搬到 tmp/moved_staging，原名稱改放：moved＝什麼都不放、replaced＝有內容的目錄、
    empty＝空目錄、symlink＝指向 marker 的 symlink、file＝一般檔案。"""
    staging = world.real_i074 / world.stagings()[0]
    moved = world.tmp / "moved_staging"
    shutil.move(str(staging), str(moved))
    seen.update(moved=moved, moved_ino=moved.stat().st_ino, moved_tree=_tree(moved), how=how)
    if how == "replaced":
        staging.mkdir()
        (staging / "intruder").write_text("i")
    elif how == "empty":
        staging.mkdir()
    elif how == "symlink":
        staging.symlink_to(world.marker)
    elif how == "file":
        staging.write_text("f")
    if how != "moved":
        seen.update(intruder=staging, intruder_ino=staging.lstat().st_ino)


def _assert_swap_preserved(world, seen, marker, intruder_kept=True):
    moved = seen["moved"]
    assert moved.is_dir() and moved.stat().st_ino == seen["moved_ino"] and _tree(moved) == seen["moved_tree"]
    if "intruder" in seen:
        p = seen["intruder"]
        assert os.path.lexists(p) == intruder_kept
        if intruder_kept:
            assert p.lstat().st_ino == seen["intruder_ino"]
            if seen["how"] == "replaced":
                assert (p / "intruder").read_text() == "i"
            elif seen["how"] == "symlink":
                assert p.is_symlink() and os.readlink(p) == str(world.marker)
            elif seen["how"] == "file":
                assert p.is_file() and not p.is_symlink() and p.read_text() == "f"
    assert _tree(world.marker) == marker and not world.dest.exists()


@pytest.mark.parametrize("how", ["moved", "replaced", "symlink", "file"])
def test_impl_r3_staging_swapped_between_stat_and_open_is_9(world, monkeypatch, how):
    """⚠️ 實作第三輪 review（中）：清理時第一次 stat 核對通過、開啟之前才被搬走或換掉——開啟的 ENOENT／ELOOP、
    開啟之後的 inode 不符都是 9，⛔ 不刪換進來的東西。"""
    state, seen = {}, {}
    _fail_first_write(monkeypatch, state)
    real_open = pm._open_dir

    def racing(parent_fd, name):
        if state.get("failed") and not seen and pm.STAGING_RE.match(name):
            _swap_staging(world, how, seen)
        return real_open(parent_fd, name)

    monkeypatch.setattr(pm, "_open_dir", racing)
    marker = _tree(world.marker)
    assert world.run() == 9
    assert seen, "競爭沒有發生"
    _assert_swap_preserved(world, seen, marker)


@pytest.mark.parametrize("how", ["moved", "replaced", "empty", "symlink", "file"])
def test_impl_r3_staging_swapped_after_recursion_before_rmdir_is_9(world, monkeypatch, how):
    """⚠️ 實作第三輪 review（中）：遞迴刪完子項目之後、rmdir 之前被搬走或換掉——rmdir 之前再以 expect 核對一次，
    ⛔ 不刪換進來的目錄（連空目錄也⛔ 不刪）。"""
    state, seen, calls = {}, {}, []
    _fail_first_write(monkeypatch, state)
    real_lstat = pm._lstat_at

    def racing(dir_fd, name):
        if state.get("failed") and pm.STAGING_RE.match(name):
            calls.append(name)
            if len(calls) == 2:                                       # 第二次＝rmdir 之前的核對
                _swap_staging(world, how, seen)
        return real_lstat(dir_fd, name)

    monkeypatch.setattr(pm, "_lstat_at", racing)
    marker = _tree(world.marker)
    assert world.run() == 9
    assert len(calls) == 2 and seen["moved_tree"] == {}               # 子項目已經刪完（遞迴完成之後）
    _assert_swap_preserved(world, seen, marker)


@pytest.mark.parametrize("how,intruder_kept", [("moved", True), ("replaced", True), ("empty", False),
                                               ("symlink", True), ("file", True)])
def test_impl_r3_staging_swapped_after_the_last_check_is_9(world, monkeypatch, how, intruder_kept):
    """照實的界線：rmdir 只能以名稱指定——最後一次核對之後才換掉時，rmdir 的 ENOENT／ENOTEMPTY 是 9，換成 symlink 或
    一般檔案時的 ENOTDIR 也是 9（⚠️ 實作第四輪 review：symlink 與 marker ⛔ 不被改動）；換成**空**目錄時
    rmdir 會刪掉那個空目錄（沒有內容遺失），之後以仍開著的 fd 的 st_nlink ≠ 0 發現本程序的 staging 還在別處 → 9。"""
    state, seen = {}, {}
    _fail_first_write(monkeypatch, state)
    real_rmdir = pm._rmdir

    def racing(name, dir_fd):
        if state.get("failed") and not seen and pm.STAGING_RE.match(name):
            _swap_staging(world, how, seen)
        return real_rmdir(name, dir_fd)

    monkeypatch.setattr(pm, "_rmdir", racing)
    marker = _tree(world.marker)
    assert world.run() == 9
    assert seen, "競爭沒有發生"
    _assert_swap_preserved(world, seen, marker, intruder_kept=intruder_kept)


def test_impl_r2_remove_tree_at_enoent(tmp_path):
    """ENOENT：沒有 expect（第 2 步清孤兒）→ 冪等 no-op；有 expect（本次的 staging）→ `StagingReplaced`。"""
    fd = os.open(tmp_path, os.O_RDONLY | os.O_DIRECTORY)
    try:
        pm.remove_tree_at(fd, ".promote-staging-0123456789abcdef")
        with pytest.raises(pm.StagingReplaced):
            pm.remove_tree_at(fd, ".promote-staging-0123456789abcdef", expect=(1, 2))
    finally:
        os.close(fd)


def test_barrier_staging_swapped_after_chain_check_is_9_after_the_fact(world):
    """照實的界線①：rename 只能以名稱指定來源——鏈檢查之後才換掉的 staging 會被 rename，事後的鏈檢查回 9。"""
    def swap():
        staging = world.real_i074 / world.stagings()[0]
        shutil.move(str(staging), str(world.tmp / "orig_staging"))
        staging.mkdir()
        (staging / "intruder").write_text("i")

    world.hooks["before_rename"] = swap
    assert world.run() == 9
    assert (world.dest / "intruder").read_text() == "i"               # 照實：被換進來的那一個落在目的地


@pytest.mark.parametrize("name", ["a/b", "..", ".", "", "x\x00y"])
def test_names_must_be_single_components(name, tmp_path):
    fd = os.open(tmp_path, os.O_RDONLY | os.O_DIRECTORY)
    try:
        with pytest.raises(pm.PromotionExit) as exc:
            pm.remove_tree_at(fd, name)
        assert exc.value.code == 9
    finally:
        os.close(fd)


# ── 5b、6b：ignore 守門 ───────────────────────────────────────────────────────

@pytest.fixture
def mkdirs(monkeypatch):
    seen = []
    real = pm._mkdir
    monkeypatch.setattr(pm, "_mkdir", lambda name, mode, dir_fd: seen.append(name) or real(name, mode, dir_fd))
    return seen


def test_5b_destination_ignored_is_9_and_no_staging(world, mkdirs):
    world.ignored = lambda p: True                                    # 過廣規則：目的地也被忽略
    assert world.run() == 9 and mkdirs == [] and world.stagings() == []


def test_6b_staging_not_ignored_is_9_and_no_staging(world, mkdirs):
    world.ignored = lambda p: False                                   # 規則被刪掉
    assert world.run() == 9 and mkdirs == []


def test_6b_staging_only_partially_ignored_is_9(world, mkdirs):
    world.ignored = lambda p: p.startswith(f"{STAGE2_REL}/.promote-staging-") and not p.endswith(".patch")
    assert world.run() == 9 and mkdirs == []


@pytest.mark.parametrize("reply", [
    lambda s: (128, b""),
    lambda s: (0, b"python/baselines/i074_stage2/evidence/not-an-input\x00"),
    lambda s: (0, b""),                                               # rc 0 卻沒有輸出
    lambda s: (1, s.split(b"\x00")[0] + b"\x00"),                     # rc 1 卻有輸出
    lambda s: (1, b"unterminated"),
])
def test_check_ignore_bad_replies_are_9(world, mkdirs, reply):
    world.check_ignore_override = reply
    assert world.run() == 9 and mkdirs == []


def test_check_ignore_timeout_is_9(world, mkdirs):
    real_git = world.git

    def git(argv, stdin=None, timeout=None):
        if "check-ignore" in argv:
            raise subprocess.TimeoutExpired(argv, timeout)
        return real_git(argv, stdin, timeout)

    world.git = git
    assert world.run() == 9 and mkdirs == []


def test_sixth_round_recovery_sequence(world, monkeypatch, fsyncs):
    """6b rename 之後 parent fsync 失敗 → 3 → 規則變過廣 → 重跑走 6a → 9（來源照常 fsync、目的地⛔ 不被 fsync、
    ⛔ 不呼叫驗證模式）→ 規則修好 → 0／6。"""
    state = {"fail": False}
    real = pm._fsync

    def maybe_fail(fd):
        if state["fail"] and os.fstat(fd).st_ino == world.real_i074.stat().st_ino:
            raise OSError(errno.EIO, "injected parent fsync")
        return real(fd)

    monkeypatch.setattr(pm, "_fsync", maybe_fail)
    world.hooks["before_rename"] = lambda: state.update(fail=True)
    assert world.run() == 3 and world.dest.is_dir()
    state["fail"] = False
    world.hooks.clear()
    world.ignored = lambda p: True
    world.verify_calls.clear()
    fsyncs_before = len(fsyncs)
    dest_idents = _idents(world.dest)
    assert world.run() == 9
    assert world.verify_calls == [] and not (set(fsyncs[fsyncs_before:]) & dest_idents)
    assert _idents(world.terminal) <= set(fsyncs[fsyncs_before:])     # 第 4 步的來源 fsync 照常
    world.ignored = lambda p: p.startswith(f"{STAGE2_REL}/.promote-staging-")
    assert world.run() == 0


# ── 6a ───────────────────────────────────────────────────────────────────────

def _promoted(w):
    assert w.run() in (0, 6)
    w.verify_calls.clear()


@pytest.mark.parametrize("change", ["bytes", "extra", "missing", "symlink"])
def test_6a_destination_differs_is_9_and_untouched(world, change):
    _promoted(world)
    if change == "bytes":
        (world.dest / sa.REPORT).write_bytes(b"other")
    elif change == "extra":
        (world.dest / "extra").write_text("x")
    elif change == "missing":
        (world.dest / sa.REPORT).unlink()
    else:
        (world.dest / sa.REPORT).unlink()
        (world.dest / sa.REPORT).symlink_to(world.marker / "keep.txt")
    snapshot = _tree(world.dest)
    assert world.run() == 9 and _tree(world.dest) == snapshot and world.verify_calls == []


def test_6a_verification_failure_is_9(world):
    _promoted(world)
    world.verify_rc = 1
    assert world.run() == 9


@pytest.mark.parametrize("override", [{"manifest_sha256": "0" * 64}, {"identity_sha256": "0" * 64},
                                      {"base_commit": "b" * 40}, {"target": "failed/x"}, {"extra": 1}])
def test_verification_output_must_bind(world, override):
    world.verify_override = override
    assert world.run() == 9 and not world.dest.exists() and world.stagings() == []


@pytest.mark.parametrize("text", ["", "{}\n", "not json\n", '{"a":1}\n{"a":1}\n'])
def test_verification_output_must_be_one_closed_line(world, text):
    world.verify_text = text
    assert world.run() == 9


def test_6a_in_place_rewrite_after_verification_is_9_and_not_fsynced(world, fsyncs):
    _promoted(world)
    target = world.dest / sa.REPORT

    def rewrite(path):
        with open(target, "r+b") as fh:                               # 原地改寫：inode 不變
            fh.write(b"X")

    world.verify_hook = rewrite
    fsyncs.clear()
    dest_idents = _idents(world.dest)
    assert world.run() == 9
    assert not (set(fsyncs) & dest_idents)                            # ⛔ 不 fsync 目的地、⛔ 不回 0／6


def test_6a_rewrite_after_the_final_rehash_is_not_detected(world):
    """照實：收尾重算之後的改寫，晉升⛔ 無法偵測（由判讀器、再跑 --promote 與 commit 前的 review 把關）。"""
    _promoted(world)
    world.hooks["after_final_rehash"] = lambda: (world.dest / sa.REPORT).write_bytes(b"late")
    assert world.run() == 0
    assert world.run() == 9                                           # 再跑一次 --promote（6a）就抓到


# ── 6b：結束碼的分類、收尾重算 ───────────────────────────────────────────────

def test_6b_space_shortage_is_8_then_rerun_succeeds(world, monkeypatch):
    class Tiny:
        f_bavail, f_frsize = 1, 1

    monkeypatch.setattr(pm, "_statvfs", lambda fd: Tiny())
    assert world.run() == 8 and world.stagings() == [] and not world.dest.exists()
    monkeypatch.undo()
    assert world.run() == 0


@pytest.mark.parametrize("err,code", [(errno.ENOSPC, 8), (errno.EDQUOT, 8), (errno.EIO, 8), (errno.EACCES, 9)])
def test_6b_staging_write_errors(world, monkeypatch, err, code):
    def failing(fd, data):
        raise OSError(err, "injected")

    monkeypatch.setattr(pm, "_write", failing)
    assert world.run() == code and world.stagings() == [] and not world.dest.exists()


@pytest.mark.parametrize("err,code", [(errno.ENOSPC, 8), (errno.EACCES, 9)])
def test_6b_staging_mkdir_errors(world, monkeypatch, err, code):
    def failing(name, mode, dir_fd):
        raise OSError(err, "injected")

    monkeypatch.setattr(pm, "_mkdir", failing)
    assert world.run() == code and world.stagings() == []


@pytest.mark.parametrize("exc,code", [
    (FileExistsError(errno.EEXIST, "exists"), 8),
    (pm.publish.NoClobberUnsupported("EXDEV"), 9),            # ⚠️ 晉升模組經 bootstrap 載入的那一份類別
    (OSError(errno.EINVAL, "einval"), 9),
    (OSError(errno.ENOENT, "gone"), 9),
])
def test_6b_rename_errors(world, monkeypatch, exc, code):
    def failing(*a):
        raise exc

    monkeypatch.setattr(pm, "_rename_at", failing)
    assert world.run() == code and world.stagings() == [] and not world.dest.exists()


def test_6b_eexist_then_rerun_takes_6a(world, monkeypatch):
    def racing(src_fd, src, dst_fd, dst):
        shutil.copytree(world.terminal, world.dest)                   # 另一個 --promote 先發布了同一份
        raise FileExistsError(errno.EEXIST, "exists")

    monkeypatch.setattr(pm, "_rename_at", racing)
    assert world.run() == 8
    monkeypatch.undo()
    assert world.run() == 0 and len(world.verify_calls) == 2


def test_6b_verification_failure_is_9_and_cleans_staging(world):
    world.verify_rc = 1
    assert world.run() == 9 and world.stagings() == [] and not world.dest.exists()


def test_6b_source_changed_during_copy_is_9(world):
    world.hooks["before_copy"] = lambda: (world.terminal / sa.REPORT).write_bytes(b"changed")
    assert world.run() == 9 and world.stagings() == [] and not world.dest.exists()


def test_6b_in_place_rewrite_of_staging_after_verification_is_9(world):
    def rewrite(path):
        with open(path / sa.TOOLING_PATCH, "r+b") as fh:              # 非 manifest 成員、inode 不變
            fh.write(b"X")

    world.verify_hook = rewrite
    assert world.run() == 9 and not world.dest.exists() and world.stagings() == []


def test_6b_parent_fsync_failure_after_rename_is_3(world, monkeypatch):
    state = {"renamed": False}
    real_fsync, real_rename = pm._fsync, pm._rename_at

    def rename(*a):
        real_rename(*a)
        state["renamed"] = True

    def fsync(fd):
        if state["renamed"]:
            raise OSError(errno.EIO, "injected")
        return real_fsync(fd)

    monkeypatch.setattr(pm, "_rename_at", rename)
    monkeypatch.setattr(pm, "_fsync", fsync)
    assert world.run() == 3 and world.dest.is_dir()
    monkeypatch.undo()
    assert world.run() == 0


def test_6b_failed_dir_creation_error_is_classified(failed_world, monkeypatch):
    real = pm._mkdir

    def mkdir(name, mode, dir_fd):
        if name == "failed":
            raise OSError(errno.ENOSPC, "injected")
        return real(name, mode, dir_fd)

    monkeypatch.setattr(pm, "_mkdir", mkdir)
    assert failed_world.run() == 8 and failed_world.stagings() == []


# ── 實作第一輪 review（高）：failed/ 的目錄項目一定要在 i074_stage2/ 落盤 ──────────────────────

def _dir_ident(path: Path):
    st = path.stat()
    return st.st_dev, st.st_ino


def test_impl_r1_failed_parent_fsync_failure_then_rerun_refsyncs_i074(failed_world, monkeypatch, fsyncs):
    """建立 failed/ 之後 fsync i074_stage2/ 失敗 → 8、failed/ 留在磁碟；重跑時 failed/ 已存在，⚠️ 仍必須重新 fsync
    真正 repo 的 i074_stage2/ 才能回 6（review 的重現：重跑跳過那一步、只 fsync failed/ 就回 6）。"""
    w = failed_world
    i074 = _dir_ident(w.real_i074)
    state = {"armed": True}
    spy = pm._fsync

    def failing_once(fd):
        st = os.fstat(fd)
        if state["armed"] and (st.st_dev, st.st_ino) == i074:
            state["armed"] = False
            raise OSError(errno.EIO, "injected parent fsync")
        return spy(fd)

    monkeypatch.setattr(pm, "_fsync", failing_once)
    assert w.run() == 8
    assert (w.real_i074 / "failed").is_dir() and not w.dest.exists() and w.stagings() == []
    fsyncs.clear()
    real_rename = pm._rename_at
    monkeypatch.setattr(pm, "_rename_at", lambda *a: fsyncs.append("RENAME") or real_rename(*a))
    assert w.run() == 6
    # ⚠️ 重跑時 failed/ 已存在，i074_stage2/ 仍要在 **rename 之前** fsync（⛔ 不能只靠 rename 之後那一次——兩道各自都要在）
    assert i074 in fsyncs[:fsyncs.index("RENAME")]
    assert _dir_ident(w.real_i074 / "failed") in fsyncs[fsyncs.index("RENAME"):]


@pytest.mark.parametrize("which", ["failed", "i074"])
def test_impl_r1_cross_directory_rename_fsyncs_both_parents(failed_world, monkeypatch, which):
    """跨目錄 rename（staging 在 i074_stage2/、目的地在 failed/）之後，兩個 parent 都要 fsync；任一失敗 → 3。"""
    w = failed_world
    state = {"renamed": False}
    real_rename, real_fsync = pm._rename_at, pm._fsync

    def rename(*a):
        real_rename(*a)
        state["renamed"] = True

    def fsync(fd):
        if state["renamed"]:
            st = os.fstat(fd)
            target = w.real_i074 / "failed" if which == "failed" else w.real_i074
            if (st.st_dev, st.st_ino) == _dir_ident(target):
                raise OSError(errno.EIO, "injected")
        return real_fsync(fd)

    monkeypatch.setattr(pm, "_rename_at", rename)
    monkeypatch.setattr(pm, "_fsync", fsync)
    assert w.run() == 3 and w.dest.is_dir()
    monkeypatch.undo()
    assert w.run() == 6


def test_impl_r1_6a_of_a_failed_record_fsyncs_i074_too(failed_world, fsyncs):
    w = failed_world
    assert w.run() == 6
    fsyncs.clear()
    assert w.run() == 6                                               # 6a
    assert _dir_ident(w.real_i074) in fsyncs and _dir_ident(w.real_i074 / "failed") in fsyncs


# ── 常數、main ───────────────────────────────────────────────────────────────

def test_constants_mirror_the_single_definitions():
    assert (pm.EXIT_DURABILITY_UNCONFIRMED, pm.EXIT_FAILED_RECORD, pm.EXIT_PROMOTION_FAILED, pm.EXIT_PROMOTION_BLOCKED) \
        == (3, 6, 8, 9) == (rb_publish.EXIT_DURABILITY_UNCONFIRMED, rb_publish.EXIT_COUNTERFACTUAL_INEFFECTIVE,
                            rb_publish.EXIT_PROMOTION_FAILED, rb_publish.EXIT_PROMOTION_BLOCKED)
    assert pm.STAGE2_MANIFEST == sa.STAGE2_MANIFEST_NAME and pm.FAILED_RECORD == sa.FAILED_RECORD_NAME
    assert pm.EVIDENCE_IDENTITY == sa.IDENTITY
    assert f"{STAGE2_REL}/evidence" == sa.STAGE2_EVIDENCE_ROOT_PATH and f"{STAGE2_REL}/failed" == sa.STAGE2_FAILED_ROOT_PATH
    assert pm.STAGING_RE.match(".promote-staging-0123456789abcdef") and not pm.STAGING_RE.match(".promote-staging-x")


def test_main_maps_unexpected_exceptions_to_9(monkeypatch, capsys):
    def boom(self):
        raise RuntimeError("boom")

    monkeypatch.setattr(pm.Promoter, "run", boom)
    assert pm.main(["--work-dir", "/w", "--real-repo", "/r"]) == 9
    monkeypatch.setattr(pm.Promoter, "run", lambda self: (_ for _ in ()).throw(pm.PromotionExit(8, "x")))
    assert pm.main(["--work-dir", "/w", "--real-repo", "/r"]) == 8


def test_state_mismatch_is_9(world):
    def bad(w, r):
        raise ValueError("chain")

    world_promoter = pm.Promoter(str(world.work), str(world.real), git=world.git, verify=world.verify, load_states=bad)
    with pytest.raises(pm.PromotionExit) as exc:
        world_promoter.run()
    assert exc.value.code == 9
