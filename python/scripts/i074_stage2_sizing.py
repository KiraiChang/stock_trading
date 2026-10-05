#!/usr/bin/env python3
"""I-074 Stage 2 步驟 ④：sizing harness 的 Python 段；⑦d 起也是 memory／disk acceptance harness 的量測原語。

規格見 issue.md I-074「Stage 2 步驟 ④：sizing harness 計畫書」（v7 ＋ 差異 1，✅ 已確認）與「Stage 2 步驟 ⑦d 細部計畫 v1」。
由 `scripts/i074-stage2-sizing.sh`、`scripts/i074-stage2-acceptance.sh` 與 `scripts/lib/i074-sizing-docker-shim.sh` 呼叫；
⛔ 不是正式的證據入口。⚠️ ⑦d：harness 執行的是 S 裡的**快照**（`<S>/harness/…`），⛔ 不是活路徑的這個檔案。

子指令分兩類：

* **容器內**（Stage 2 image、唯讀 rootfs）：`fixture`——合成三條路徑的 operational 輸出；
  全量檔用本檔的**串流 writer**（差異 1），小檔用正式的 `publish_artifacts()`。
* **host**（只用標準庫；⛔ 不 import `backtest.*`——host 沒有 pandas）：量測（`allocated`、`sample`、
  `baseline`、`phase-end`）、shim 的簿記（`seq`、`index`、`event`、`sidecar`）、`twins`、`record-diff`、
  `report`、`failure-summary`，以及 `spec-hash`／`logbound` 這兩個純函式的 CLI；
* ⑦d（acceptance，host）：`acceptance-report`、`acceptance-anchors`、`replay-argv`、`clean-env`、`promote-measure`、
  `host-run`，與 ⑩ 期間由操作者另外啟動的 `observe`。⚠️ ⑩ 的正式程式（`i074_stage2_preflight.py`、
  `i074_stage2_promote.py`、supervisor 的 `clean_env()`）一律以明確的 `--clone <工作複本>` 載入（⛔ 不從活路徑）。

⚠️ **單位**：落在檔案系統上的一律是 **allocated bytes**（`st_blocks × 512`，目錄一併計入，hard link 只算一次）；
內容長度只用在讀不到的 docker log 與容器 metadata 的保守推導，且以 block 上捨。
"""
from __future__ import annotations

import argparse
import fcntl
import hashlib
import importlib.util
import json
import os
import re
import resource
import secrets
import signal
import stat
import subprocess
import sys
import time
from pathlib import Path
from typing import Any, Callable, Iterable, Iterator, Mapping

# ── 常數（⚠️ 與計畫書逐項對應，⛔ 不開執行期參數） ─────────────────────────────

BLOCK = 4096                        # log／metadata 推導值的上捨單位
LOG_PIECE = 16 * 1024               # json-file 的長行分段
LOG_ESCAPE_FACTOR = 6               # JSON 逐 byte 跳脫的最壞倍數（\u00XX）
LOG_ENTRY_OVERHEAD = 128            # {"log":…,"stream":…,"time":…}\n 的固定開銷（實際約 70）
META_ROUND = 64 * 1024
META_START_MARGIN = 64 * 1024       # 啟動時才建立的 hosts／hostname／resolv.conf 與目錄項
TWIN_COUNT = 3
STREAM_CHUNK = 64 * 1024            # 與 artifacts.write_canonical_atomic() 相同

# ⚠️ ⑦d：封閉列舉依 profile 各自寫死（「Stage 2 步驟 ⑦d 細部計畫 v1」「二之四」）；profile 未設定 ＝ sizing（④ 的行為）。
#   每條路徑預期的 invocation（依 sequence 順序）——寫死，⛔ 不由 harness 以外的地方推論。
PROFILES: dict[str, dict[str, Any]] = {
    "sizing": {
        "phases": ("witness", "success", "failure", "memory_only"),
        "disk_phases": ("witness", "success", "failure"),
        "info_phases": (),
        "roles": ("fixture", "finalizer", "recovery", "check"),
        "expected": {
            "witness": ("fixture", "finalizer"),
            "success": ("fixture", "finalizer"),
            "failure": ("fixture", "finalizer"),
            "memory_only": ("recovery", "recovery", "check", "recovery"),
        },
        "size_rw_must_be_zero": True,       # ④：唯讀 rootfs
    },
    "acceptance": {
        "phases": ("preflight", "success", "promote_success", "failure", "promote_failure", "memory_only"),
        "disk_phases": ("success", "failure"),                    # 與 P_B_BUDGET 比較的兩條
        "info_phases": ("promote_success", "promote_failure"),    # 晉升：只列資訊值（P_B 的起訖⛔ 不變）
        "roles": ("check", "replay", "finalizer", "promotion", "recovery"),
        "expected": {
            "preflight": ("check",),
            "success": ("replay", "finalizer"),
            "promote_success": ("promotion",),
            "failure": ("replay", "finalizer"),
            "promote_failure": ("promotion",),
            # ⚠️ i：先以封存的見證輸出重新發布 envcheck（finalizer），再量 --recover-envcheck（recovery）
            "memory_only": ("check", "recovery", "recovery", "finalizer", "recovery"),
        },
        "size_rw_must_be_zero": False,      # ⑩ ⛔ 不加 --read-only：SizeRw 照實計入（v29「六、1」差異列）
    },
}
FULL_COMPUTE_EXTRA = ("memory_only", "replay")   # --replay-compute full：memory_only 最後多一個量測趟
REPLAY_COMPUTES = ("stub", "full")
PHASES = PROFILES["sizing"]["phases"]
DISK_PHASES = PROFILES["sizing"]["disk_phases"]
ROLES = PROFILES["sizing"]["roles"]
EXPECTED_ROLES = PROFILES["sizing"]["expected"]
LOCATIONS = ("L1", "L2", "L3", "L5")
# ⚠️ ⑦d「快照與來源的清單」①——與兩個入口的 `I074_BOOT_FILES` 必須相同（測試釘住）。
SNAPSHOT_FILES = {
    "sizing": ("scripts/i074-stage2-sizing.sh", "scripts/lib/i074-stage2-measure.sh",
               "scripts/lib/i074-sizing-docker-shim.sh", "scripts/lib/mem-guard.sh", "python/scripts/i074_stage2_sizing.py"),
    "acceptance": ("scripts/i074-stage2-acceptance.sh", "scripts/lib/i074-stage2-measure.sh",
                   "scripts/lib/i074-sizing-docker-shim.sh", "python/scripts/i074_stage2_sizing.py",
                   "python/scripts/i074_stage2_replay_stub.py"),
}
# ⚠️ v29「六、1」：每一個程序的峰值 < 450 MiB（嚴格小於）——唯一定義（⑦d「三」#14）。
ACCEPTANCE_MEMORY_LIMIT = 450 * 1024 * 1024
ROTATION_OPTIONS = ("max-size", "max-file")
PROBE_SHAPE = (("a", b"a"), ("b-src", b"src"), ("b-dst", b"dst"))


class SizingError(RuntimeError):
    """量測不完整或不一致——⛔ fail-closed。"""


def ceil_to(value: int, unit: int) -> int:
    return -(-int(value) // unit) * unit


def canonical_dumps(obj: Any) -> bytes:
    """與 `replay_bundle.canonical.canonical_json_bytes()` 同一組參數（host 端 ⛔ 不 import `backtest.*`）。"""
    return json.dumps(obj, sort_keys=True, separators=(",", ":"), ensure_ascii=False,
                      allow_nan=False).encode("utf-8")


def write_exclusive(path: Path, payload: bytes) -> None:
    """不可變紀錄：exclusive create，已存在即失敗（⛔ 不覆寫）。"""
    path.parent.mkdir(parents=True, exist_ok=True)
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o644)
    with os.fdopen(fd, "wb") as fh:
        fh.write(payload)


def read_json(path: Path) -> Any:
    return json.loads(Path(path).read_text(encoding="utf-8"))


# ── allocated bytes 與 inventory ───────────────────────────────────────────────

def allocated_tree(root: str | Path) -> dict[str, int]:
    """整棵 tree 的 allocated bytes（含目錄；hard link 只算一次；⛔ 不跨 device、⛔ 不跟隨 symlink）。"""
    root = Path(root)
    top = os.lstat(root)
    seen: set[tuple[int, int]] = set()
    total = dirs = 0
    stack = [(root, top)]
    while stack:
        path, st = stack.pop()
        key = (st.st_dev, st.st_ino)
        if key not in seen:
            seen.add(key)
            total += st.st_blocks * 512
        if stat.S_ISDIR(st.st_mode):
            dirs += 1
            try:
                entries = list(os.scandir(path))
            except FileNotFoundError:
                continue      # 取樣時目錄剛被刪掉——下一個取樣點會看到
            for entry in entries:
                try:
                    est = entry.stat(follow_symlinks=False)
                except FileNotFoundError:
                    continue
                if est.st_dev != top.st_dev:
                    continue
                stack.append((Path(entry.path), est))
    return {"allocated": total, "dirs": dirs}


def fs_used(path: str | Path) -> int:
    st = os.statvfs(path)
    return (st.f_blocks - st.f_bfree) * st.f_frsize


def inventory(roots: Iterable[str | Path]) -> dict[str, list]:
    """路徑 → [型別, allocated, size, mtime_ns, inode]。⚠️ `.git` 一併記錄。"""
    out: dict[str, list] = {}
    for root in roots:
        root = Path(root)
        stack = [root]
        while stack:
            path = stack.pop()
            try:
                st = os.lstat(path)
            except FileNotFoundError:
                continue
            mode = st.st_mode
            kind = "d" if stat.S_ISDIR(mode) else "f" if stat.S_ISREG(mode) else "l" if stat.S_ISLNK(mode) else "o"
            out[str(path)] = [kind, st.st_blocks * 512, st.st_size, st.st_mtime_ns, st.st_ino]
            if kind == "d":
                try:
                    stack.extend(Path(e.path) for e in os.scandir(path))
                except FileNotFoundError:
                    continue
    return out


def _inside(path: str, roots: Iterable[str]) -> bool:
    return any(path == r or path.startswith(r.rstrip("/") + "/") for r in roots)


def inventory_diff(before: Mapping[str, list], after: Mapping[str, list], *, allowed: Iterable[str],
                   source_roots: Iterable[str] = ()) -> list[str]:
    """完整 inventory 比較（計畫書「二」的自我檢查）。回傳違規清單。

    * baseline 已存在的項目被**刪除**或**改變型別** → 任何位置都違規；
    * 新建或修改 → 只允許在 `allowed`（根目錄本身及其下所有項目）；
    * `source_roots` 底下出現 `__pycache__`／`.pyc` → 違規（host 端 bytecode）。
    """
    allowed = [str(a) for a in allowed]
    source_roots = [str(r) for r in source_roots]
    problems = []
    for path, entry in before.items():
        if path not in after:
            problems.append(f"刪除：{path}")
        elif after[path][0] != entry[0]:
            problems.append(f"型別改變：{path}（{entry[0]} → {after[path][0]}）")
        elif after[path] != entry and not _inside(path, allowed):
            problems.append(f"允許位置以外的修改：{path}")
    for path in after:
        if path not in before and not _inside(path, allowed):
            problems.append(f"允許位置以外的新建：{path}")
    for path in after:
        name = path.rsplit("/", 1)[-1]
        if path not in before and _inside(path, source_roots) and (name == "__pycache__" or name.endswith(".pyc")):
            problems.append(f"source tree 出現 bytecode：{path}")
    return problems


# ── container spec 與 shim 的簿記 ─────────────────────────────────────────────

def _image_position(argv: list[str], image: str) -> int:
    """docker options 與容器指令的分界：**第一個**等於 image ID 的 token。"""
    try:
        return argv.index(image)
    except ValueError:
        raise SizingError(f"container spec 裡找不到 image {image}") from None


def normalize_spec(argv: Iterable[str], image: str) -> list[str]:
    """`container_spec_argv`：**image 之前**的 `--cidfile`／`--name` 的值換成固定佔位字，其餘 token 逐字保留。

    ⚠️ 輸入必須已經排除 docker 執行檔與 operation（`run`／`create`）——兩者因此能得到同一個 hash。
    ⚠️ 只動 image 之前的 docker options：容器指令裡同名的參數⛔ 不正規化（改了它 hash 就必須不同）。
    """
    argv = list(argv)
    pos = _image_position(argv, image)
    out: list[str] = []
    i = 0
    while i < pos:
        tok = argv[i]
        if tok in ("--cidfile", "--name"):
            if i + 1 >= pos:
                raise SizingError(f"{tok} 缺值")
            out += [tok, "<CIDFILE>" if tok == "--cidfile" else "<NAME>"]
            i += 2
            continue
        if tok.startswith("--cidfile="):
            out.append("--cidfile=<CIDFILE>")
        elif tok.startswith("--name="):
            out.append("--name=<NAME>")
        else:
            out.append(tok)
        i += 1
    return out + argv[pos:]


def spec_sha256(argv: Iterable[str], image: str) -> str:
    return hashlib.sha256(b"\0".join(t.encode("utf-8") for t in normalize_spec(argv, image))).hexdigest()


def _locked_increment(counter: Path) -> int:
    counter.parent.mkdir(parents=True, exist_ok=True)
    with open(counter.with_suffix(".lock"), "a+") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        try:
            current = int(counter.read_text()) if counter.exists() else 0
            counter.write_text(str(current + 1))
            return current + 1
        finally:
            fcntl.flock(lock, fcntl.LOCK_UN)


def container_id(sequence: int, *, twin: int = 0) -> str:
    """正式 `o<seq 三位>0`、twin `t<seq 三位><1..3>`——**等長**。"""
    if not 1 <= sequence <= 999 or not 0 <= twin <= 9:
        raise SizingError(f"sequence／twin 超出範圍：{sequence}／{twin}")
    return f"{'t' if twin else 'o'}{sequence:03d}{twin}"


def profile_spec(profile: str) -> dict[str, Any]:
    if profile not in PROFILES:
        raise SizingError(f"未知的 profile：{profile!r}（只接受 {sorted(PROFILES)}）")
    return PROFILES[profile]


def measured_phases(profile: str) -> tuple[str, ...]:
    """有量測窗口（容器足跡要計入）的 phase：磁碟路徑 ＋ 資訊值的窗口。"""
    spec = profile_spec(profile)
    return tuple(spec["disk_phases"]) + tuple(spec["info_phases"])


def write_index(state: Path, *, sequence: int, cid: str, phase: str, role: str, included: bool,
                image: str, argv: list[str], profile: str = "sizing") -> dict[str, Any]:
    spec = profile_spec(profile)
    if phase not in spec["phases"]:
        raise SizingError(f"未知的 phase：{phase!r}（profile {profile}）")
    if role not in spec["roles"]:
        raise SizingError(f"未知的 role：{role!r}（profile {profile}）")
    if included != (phase in measured_phases(profile)):
        raise SizingError(f"included_in_disk_path={included} 與 phase={phase} 不相符")
    entry = {
        "sequence": sequence, "id": cid, "phase": phase, "role": role, "included_in_disk_path": included,
        "container_spec_sha256": spec_sha256(argv, image), "image": image, "argv": argv,
        "sidecar_path": str(state / "containers" / f"{cid}.json"), "profile": profile,
    }
    write_exclusive(state / "index" / f"{sequence:04d}.json", canonical_dumps(entry))
    return entry


def record_event(state: Path, cid: str, kind: str) -> int:
    if kind not in ("create_begin", "rm_done"):
        raise SizingError(f"未知的 lifecycle event：{kind!r}")
    number = _locked_increment(state / "events" / "counter")
    line = canonical_dumps({"event": number, "id": cid, "kind": kind, "monotonic_ns": time.monotonic_ns()})
    with open(state / "events" / "events.jsonl", "ab") as fh:
        fh.write(line + b"\n")
    return number


# ── json-file log 的上界 ──────────────────────────────────────────────────────

def log_bound(stdout: bytes, stderr: bytes) -> int:
    """json-file 的**上界**：逐行（含無結尾換行的最後一段）切成 ≤ 16 KiB 的片段，每段 `6×長度＋128`，取 4 KiB 上捨。"""
    total = 0
    for data in (stdout, stderr):
        if not data:
            continue
        lines = data.split(b"\n")
        pieces = [line + b"\n" for line in lines[:-1]]
        if lines[-1]:
            pieces.append(lines[-1])
        for piece in pieces:
            for start in range(0, len(piece), LOG_PIECE):
                chunk = piece[start:start + LOG_PIECE]
                total += LOG_ESCAPE_FACTOR * len(chunk) + LOG_ENTRY_OVERHEAD
    return ceil_to(total, BLOCK)


def log_config_problems(log_config: Mapping[str, Any]) -> list[str]:
    problems = []
    if log_config.get("Type") != "json-file":
        problems.append(f"logging driver 不是 json-file：{log_config.get('Type')!r}")
    config = log_config.get("Config") or {}
    for option in ROTATION_OPTIONS:
        if option in config:
            problems.append(f"log 有 rotation 設定 {option}={config[option]!r}（上界模型不成立）")
    return problems


def write_sidecar(state: Path, *, cid: str, sequence: int, rc: int, container: str, size_rw: str,
                  log_config_json: str, stdout_log: Path, stderr_log: Path, peak_file: Path,
                  failures: list[str], memory_limit: str, memory_swap_limit: str) -> dict[str, Any]:
    """shim 的計量結果。⚠️ 任何一項讀不到都記成「量測失敗」，⛔ 不寫成 0。

    ⑦d 實作第一輪 review：另記 daemon 實際套用的記憶體上限（`docker inspect` 的 `HostConfig.Memory`／`MemorySwap`；
    0 照實記成 0 ＝ 沒有上限，由 acceptance 的報告判定）。"""
    index = read_json(state / "index" / f"{sequence:04d}.json")
    problems = list(failures)
    try:
        size_rw_value = int(size_rw)
    except ValueError:
        size_rw_value = None
        problems.append(f"SizeRw 讀不到：{size_rw!r}")
    try:
        log_config = json.loads(log_config_json)
        problems += log_config_problems(log_config)
    except json.JSONDecodeError:
        log_config = None
        problems.append("LogConfig 讀不到")
    try:
        bound = log_bound(stdout_log.read_bytes(), stderr_log.read_bytes())
    except OSError as exc:
        bound = None
        problems.append(f"docker logs 讀不到：{exc}")
    peak_text = peak_file.read_text().strip() if peak_file.is_file() else ""
    peak = int(peak_text) if peak_text.isdigit() else None
    if not peak:
        problems.append(f"cgroup 峰值讀不到或為 0：{peak_text!r}")
    limits = {}
    for key, text in (("memory_limit_bytes", memory_limit), ("memory_swap_limit_bytes", memory_swap_limit)):
        limits[key] = int(text) if text.isdigit() else None
        if limits[key] is None:
            problems.append(f"容器的記憶體上限讀不到（{key}）：{text!r}")
    entry = {
        "id": cid, "sequence": sequence, "container_spec_sha256": index["container_spec_sha256"],
        "rc": rc, "container": container, "size_rw": size_rw_value, "log_config": log_config,
        "log_bound": bound, "peak_bytes": peak, **limits,
        "status": "ok" if not problems else "measure_failed", "failures": problems,
    }
    write_exclusive(state / "containers" / f"{cid}.json", canonical_dumps(entry))
    return entry


# ── metadata twin（所有量測窗口結束後） ─────────────────────────────────────────

def twin_argv(argv: list[str], image: str, *, cidfile: str, name: str) -> list[str]:
    """只替換 **image 之前**的 `--cidfile`／`--name` 的值（等長、唯一），其餘 token 逐字不變。"""
    pos = _image_position(argv, image)
    out, i = [], 0
    while i < pos:
        if argv[i] == "--cidfile":
            out += ["--cidfile", cidfile]
            i += 2
        elif argv[i] == "--name":
            out += ["--name", name]
            i += 2
        else:
            out.append(argv[i])
            i += 1
    return out + argv[pos:]


def adopted_metadata(raw: Iterable[int], inspect_lengths: Iterable[int]) -> int:
    raw = list(raw)
    inspect_lengths = list(inspect_lengths)
    return max(ceil_to(max(raw), META_ROUND), 2 * ceil_to(max(inspect_lengths), BLOCK)) + META_START_MARGIN


def run_twins(state: Path, *, docker: str, run_id: str, fs_path: str) -> None:
    """對每一份 invocation 索引建三個 twin（`docker create`，⛔ 不執行）量 metadata。

    ⚠️ `docker create` ⛔ 不會建立不存在的 bind 來源（2026-09-24 實查；那是 `start` 才做的事），所以正式
    worktree 刪掉之後照樣能以完全相同的 spec 建 twin。
    """
    (state / "cid").mkdir(parents=True, exist_ok=True)
    for index_path in sorted((state / "index").glob("*.json")):
        index = read_json(index_path)
        sequence = index["sequence"]
        raws, lengths, problems, cids = [], [], [], []
        for k in range(1, TWIN_COUNT + 1):
            tid = container_id(sequence, twin=k)
            cidfile = state / "cid" / f"{tid}.cid"
            name = f"i074sz-{run_id}-{tid}"
            spec = twin_argv(index["argv"], index["image"], cidfile=str(cidfile), name=name)
            if spec_sha256(spec, index["image"]) != index["container_spec_sha256"]:
                problems.append(f"twin {tid} 的 container spec hash 與索引不符")
                break
            if cidfile.exists():
                problems.append(f"twin {tid} 的 cidfile 已存在：{cidfile}")
                break
            before = fs_used(fs_path)
            created = subprocess.run([docker, "create", *spec], capture_output=True, text=True)
            after = fs_used(fs_path)
            if created.returncode != 0:
                problems.append(f"twin {tid} 的 docker create 失敗：{created.stderr.strip()[:300]}")
                break
            cid = cidfile.read_text().strip() if cidfile.is_file() else created.stdout.strip()
            cids.append(cid)
            inspected = subprocess.run([docker, "inspect", cid], capture_output=True)
            removed = subprocess.run([docker, "rm", cid], capture_output=True, text=True)
            if removed.returncode == 0:
                cidfile.unlink(missing_ok=True)
            else:
                # ⚠️ 容器還在：**保留 cidfile**（外層的失敗清理靠它 `docker rm -f`；⛔ 刪了就找不到 CID）。
                if not cidfile.is_file():
                    cidfile.write_text(cid)
                problems.append(f"twin {tid}（CID {cid}）的 docker rm 失敗：{removed.stderr.strip()[:200]}")
                break
            if inspected.returncode != 0:
                problems.append(f"twin {tid} 的 docker inspect 失敗")
                break
            raws.append(max(after - before, 0))
            lengths.append(len(inspected.stdout))
        entry = {"id": index["id"], "sequence": sequence, "container_spec_sha256": index["container_spec_sha256"],
                 "raw_bytes": raws, "inspect_lengths": lengths, "twin_containers": cids,
                 "adopted_bytes": adopted_metadata(raws, lengths) if len(raws) == TWIN_COUNT and not problems else None,
                 "status": "ok" if not problems and len(raws) == TWIN_COUNT else "measure_failed",
                 "failures": problems}
        write_exclusive(state / "twins" / f"{index['id']}.json", canonical_dumps(entry))


def cleanup_cids(state: Path, *, docker: str) -> list[str]:
    """失敗清理：對 S 裡**每一份** cidfile 記錄的 CID `docker rm -f`（含 twin；⛔ 不以名稱猜）。

    成功移除才刪 cidfile；回傳移除失敗的 CID（呼叫端回報，⛔ 不吞掉）。
    """
    leftovers = []
    for cidfile in sorted((state / "cid").glob("*.cid")) if (state / "cid").is_dir() else []:
        cid = cidfile.read_text().strip()
        if not cid:
            cidfile.unlink(missing_ok=True)
            continue
        removed = subprocess.run([docker, "rm", "-f", cid], capture_output=True, text=True)
        gone = removed.returncode == 0 or "No such container" in removed.stderr
        if gone:
            cidfile.unlink(missing_ok=True)
        else:
            leftovers.append(cid)
    return leftovers


# ── 取樣 ──────────────────────────────────────────────────────────────────────

def measure_locations(locations: Mapping[str, str]) -> dict[str, int]:
    return {name: allocated_tree(path)["allocated"] for name, path in locations.items()}


def run_sampler(state: Path, phase: str, *, locations: Mapping[str, str], fs_path: str, interval: float) -> None:
    """每 `interval` 秒同步量 L1、L2、L3、L5 與 L0，直到 stop 檔出現；結束前再量一次。"""
    pdir = state / "phases" / phase
    stop = pdir / "sampler.stop"
    out = open(pdir / "samples.jsonl", "ab")
    mem = open(state / "memavail.tsv", "a")
    last_mem = 0.0
    while True:
        stopping = stop.exists()
        sample = {"t_ns": time.monotonic_ns(), "L0": fs_used(fs_path), **measure_locations(locations)}
        out.write(canonical_dumps(sample) + b"\n")
        out.flush()
        now = time.monotonic()
        if now - last_mem >= 2.0:
            last_mem = now
            for line in Path("/proc/meminfo").read_text().splitlines():
                if line.startswith("MemAvailable:"):
                    mem.write(f"{time.monotonic_ns()}\t{int(line.split()[1]) * 1024}\n")
                    mem.flush()
        if stopping:
            break
        time.sleep(interval)
    out.close()
    mem.close()


# ── 串流 writer（差異 1） ─────────────────────────────────────────────────────

def _fsync_fd(fd: int) -> None:
    os.fsync(fd)


def _fsync_dir(directory: Path) -> None:
    fd = os.open(str(directory), os.O_RDONLY | getattr(os, "O_DIRECTORY", 0))
    try:
        os.fsync(fd)
    finally:
        os.close(fd)


def _replace(src: Path, dst: Path) -> None:
    os.replace(src, dst)


def stream_write_canonical(directory: Path, name: str, top: Mapping[str, Any], rows: Iterator[Any]) -> str:
    """把 `top ∪ {"rows": rows}` 以 canonical bytes **逐列**寫出，回傳 SHA-256。

    ⚠️ 磁碟語意與 `artifacts.write_canonical_atomic()` 逐項相同：同目錄 temp → 64 KiB 分塊 →
    flush ＋ fsync → `os.replace` → fsync 目錄。`os.replace` **之前**的任何失敗在 `finally` 刪除 temp、
    ⛔ 不留正式檔；**之後**的目錄 fsync 失敗時正式檔可能已存在（durability 未確認），例外照樣往外拋，
    呼叫端⛔ 不得發布後續的指標檔。rows 必須是一次性 iterator——⛔ 不得 `list(rows)`。
    """
    from backtest.modular.sr_scoring.replay_bundle.canonical import canonical_json_bytes

    if "rows" in top:
        raise SizingError("top ⛔ 不得含 rows（由 rows iterator 提供）")
    envelope = canonical_json_bytes({**top, "rows": []})
    marker = b'"rows":[]'
    if envelope.count(marker) != 1:
        raise SizingError(f'頂層編碼裡的 "rows":[] 必須恰好出現一次，實際 {envelope.count(marker)}')
    head, tail = envelope.split(marker)
    directory = Path(directory)
    tmp = directory / f".{name}.{secrets.token_hex(6)}.tmp"
    digest = hashlib.sha256()
    replaced = False
    try:
        with open(tmp, "xb") as fh:
            buf = bytearray()

            def emit(piece: bytes) -> None:
                digest.update(piece)
                buf.extend(piece)
                if len(buf) >= STREAM_CHUNK:
                    fh.write(buf)
                    buf.clear()

            emit(head + b'"rows":[')
            first = True
            for row in rows:
                emit((b"" if first else b",") + canonical_json_bytes(row))
                first = False
            emit(b"]" + tail)
            if buf:
                fh.write(buf)
            fh.flush()
            _fsync_fd(fh.fileno())
        _replace(tmp, directory / name)
        replaced = True
        _fsync_dir(directory)     # ⚠️ 這一步失敗時正式檔已存在——例外往外拋，⛔ 不刪
    finally:
        if not replaced and tmp.exists():
            tmp.unlink()
    return digest.hexdigest()


# ── fixture（容器內） ──────────────────────────────────────────────────────────

def _fixture_prov(cache: Path, composed_sha256: str) -> dict[str, Any]:
    """before 側的 provenance：沿用見證趟 after' 的（新 image、`e1cbbbd`），tooling ＝ 合成 hash。

    ⚠️ ⑦b：合成 hash 由 host 以唯一的合成函式算好傳進來（真實 tooling 非空之後，它⛔ 不再等於
    `sha256(counterfactual patch)`，否則 finalize 的合成守門會擋下）。
    """
    if not (isinstance(composed_sha256, str) and len(composed_sha256) == 64
            and all(c in "0123456789abcdef" for c in composed_sha256)):
        raise SizingError(f"--composed-sha256 必須是 64 位小寫 hex：{composed_sha256!r}")
    from backtest.modular.sr_scoring.replay_bundle.stream import stream_canonical_artifact

    load = stream_canonical_artifact(cache / "envcheck" / "witness" / "after_artifact.json.gz",
                                     "sr_zone_replay_after", on_row=lambda r, b: None)
    return dict(load.top["provenance"], tooling_patch_sha256=composed_sha256,
                argv=["--i074-counterfactual", "--output-dir", "/out/stage2"])


def _streamed_rows(source: Path) -> Iterator[Any]:
    """把 callback 式的 `stream_canonical_artifact()` 轉成**一次性** iterator。

    ⚠️ 常駐的列數有**固定上限 3**：佇列 1 列（`maxsize=1`）＋ 讀取端正在 `put` 的 1 列 ＋ 寫出端正在編碼的 1 列。

    ⚠️ 讀取端的例外（含 canonical／hash 不符——那要讀完才知道）在最後一列之後重拋，
    串流 writer 因此在 `os.replace` 之前失敗、刪除 temp。
    """
    import queue
    import threading

    from backtest.modular.sr_scoring.replay_bundle.stream import stream_canonical_artifact

    q: "queue.Queue[Any]" = queue.Queue(maxsize=1)     # ⚠️ 常駐上限：佇列 1 列 ＋ 讀取端與寫出端手上各 1 列
    done = object()
    errors: list[BaseException] = []

    def producer() -> None:
        try:
            stream_canonical_artifact(source, "sr_zone_replay_after", on_row=lambda r, b: q.put(r))
        except BaseException as exc:  # noqa: BLE001 - 交給消費端重拋
            errors.append(exc)
        finally:
            q.put(done)

    threading.Thread(target=producer, daemon=True).start()
    while True:
        item = q.get()
        if item is done:
            break
        yield item
    if errors:
        raise errors[0]


def fixture_witness(cache: Path, out: Path) -> dict[str, Any]:
    from backtest.modular.sr_scoring.replay_bundle.artifacts import (
        load_canonical_evidence_artifact, prepare_output_dir, publish_artifacts)
    from backtest.modular.sr_scoring.replay_bundle.stream import stream_canonical_artifact

    target = prepare_output_dir(out / "witness")
    source = cache / "envcheck" / "witness" / "after_artifact.json.gz"
    cohort = load_canonical_evidence_artifact(cache / "envcheck" / "witness" / "cohort_manifest.json.gz",
                                              "sr_zone_replay_cohort").parsed
    # ⚠️ 兩趟：第一趟只取頂層（⛔ 不留 rows），第二趟邊讀邊寫——常駐上限 3 列（見 `_streamed_rows()`）。
    top = stream_canonical_artifact(source, "sr_zone_replay_after", on_row=lambda r, b: None).top
    sha = stream_write_canonical(target, "after_artifact.json", top, _streamed_rows(source))
    if sha != cohort["after_artifact_sha256"]:
        raise SizingError(f"after' 的 SHA {sha} ≠ cohort' 引用的 {cohort['after_artifact_sha256']}——⛔ 不發布 cohort'")
    publish_artifacts(target, [("cohort_manifest.json", cohort)])      # 指標檔，最後
    return {"after_sha256": sha}


def fixture_success(python_root: Path, cache: Path, out: Path, composed_sha256: str) -> dict[str, Any]:
    from backtest.modular.sr_scoring.replay_bundle import stage2_archive as sa
    from backtest.modular.sr_scoring.replay_bundle import stage2_evidence as s2
    from backtest.modular.sr_scoring.replay_bundle.artifacts import (
        build_comparison_artifact, build_report, compare_rows, prepare_output_dir, publish_artifacts, row_key)
    anchor = s2.load_stage1_anchor(python_root)
    prov = _fixture_prov(cache, composed_sha256)
    target = prepare_output_dir(out / "stage2")
    cohort = set(anchor.cohort_keys)
    kept: dict[tuple, dict] = {}
    source = s2.resolve_repo_path(python_root, f"python/baselines/i074_stage1/{s2.STAGE1_ANCHOR_AFTER}")

    def rows() -> Iterator[Any]:
        for row in _streamed_rows(source):
            key = row_key(row)
            if key in cohort:
                # 反事實生效：RR 被加回去 → 不再是 CONTINUATION、flag 為 false。
                row = dict(row, lifecycle_phase="TESTING", rr_decoupling_candidate=False)
                kept[key] = row
            yield row

    top = {k: v for k, v in sa.build_before_source(
        bundle_id=anchor.bundle_id, before_ref=prov["base_commit"], timeframe=anchor.after_top["timeframe"],
        replay_scope=anchor.after_top["replay_scope"], generated_at="2026-09-24T12:00:00+08:00",
        provenance=prov, rows=[]).items() if k != "rows"}
    before_sha = stream_write_canonical(target, "before_source_artifact.json", top, rows())
    comparison = build_comparison_artifact(
        bundle_id=anchor.bundle_id, before_ref=prov["base_commit"],
        after_artifact_sha256=anchor.members[s2.STAGE1_ANCHOR_AFTER]["artifact_sha256"],
        generated_at="2026-09-24T12:00:00+08:00", provenance=prov,
        rows=[compare_rows(kept[k], anchor.cohort_rows[k]) for k in sorted(cohort)])
    from backtest.modular.sr_scoring.replay_bundle.canonical import canonical_json_bytes

    report = build_report(bundle_id=anchor.bundle_id, before_ref=prov["base_commit"],
                          comparison_sha256=hashlib.sha256(canonical_json_bytes(comparison)).hexdigest(),
                          generated_at="2026-09-24T12:00:00+08:00", rows=comparison["rows"],
                          report_max_rows=sa.STAGE2_REPORT_MAX_ROWS)
    publish_artifacts(target, [("comparison_artifact.json", comparison), ("report.json", report)])  # report 最後
    return {"before_sha256": before_sha, "cohort_rows": len(kept)}


def fixture_failure(python_root: Path, cache: Path, out: Path, composed_sha256: str) -> dict[str, Any]:
    from backtest.modular.sr_scoring.replay_bundle import stage2_archive as sa
    from backtest.modular.sr_scoring.replay_bundle import stage2_evidence as s2
    from backtest.modular.sr_scoring.replay_bundle.artifacts import prepare_output_dir, publish_artifacts

    anchor = s2.load_stage1_anchor(python_root)
    check = sa.CounterfactualEffectCheck()
    for key in anchor.cohort_keys:            # RR 沒加回去：156 列候選原封不動 → rr_not_restored
        check.feed(anchor.cohort_rows[key])
    payload = sa.build_counterfactual_failure(bundle_id=anchor.bundle_id, generated_at="2026-09-24T13:00:00+08:00",
                                              provenance=_fixture_prov(cache, composed_sha256), effect=check.result())
    sa.validate_counterfactual_failure(payload)
    target = prepare_output_dir(out / "stage2")
    publish_artifacts(target, [("bounded_diagnostics.json", payload)])
    return {"failure_reason": payload["failure_reason"]}


# ── record 目錄（發布前後的集合差） ─────────────────────────────────────────────

def record_diff(before: Iterable[str], after: Iterable[str], *, published: str, failed_root: str) -> str:
    new = sorted(set(after) - set(before))
    if len(new) != 1:
        raise SizingError(f"發布前後 failed/ 的集合差必須恰好一筆，實際 {new}")
    record = str(Path(failed_root) / new[0])
    if record != published:
        raise SizingError(f"集合差 {record} ≠ CLI 輸出的 published {published}")
    return record


# ── 報告 ──────────────────────────────────────────────────────────────────────

def load_harness_manifest(state: Path, profile: str) -> tuple[dict[str, str], str]:
    """⑦d：快照的 `MANIFEST`（`sha256sum` 的輸出）→ 封閉的 {相對路徑: SHA-256} 與它的 canonical SHA。

    集合必須**恰好**是本 profile 的清單 ①，而且每個 SHA ＝ 快照檔案的實際 SHA（報告綁住實際執行的檔案集合）。
    """
    path = state / "harness" / "MANIFEST"
    mapping: dict[str, str] = {}
    for line in path.read_text(encoding="utf-8").splitlines():
        sha, sep, rel = line.partition("  ")
        if not sep or len(sha) != 64 or any(c not in "0123456789abcdef" for c in sha) or rel in mapping:
            raise SizingError(f"MANIFEST 的格式不符：{line!r}")
        mapping[rel] = sha
    if set(mapping) != set(SNAPSHOT_FILES[profile]):
        raise SizingError(f"MANIFEST 的檔案集合 ≠ 清單 ①：{sorted(mapping)}")
    for rel, sha in mapping.items():
        if hashlib.sha256((state / "harness" / rel).read_bytes()).hexdigest() != sha:
            raise SizingError(f"快照的 {rel} 與 MANIFEST 不符")
    return mapping, hashlib.sha256(canonical_dumps(mapping)).hexdigest()


def _read_tsv(path: Path) -> dict[str, str]:
    out: dict[str, str] = {}
    if path.is_file():
        for line in path.read_text(encoding="utf-8").splitlines():
            if line.strip():
                key, _, value = line.partition("\t")
                if key in out:
                    raise SizingError(f"{path.name} 的 {key} 重複")
                out[key] = value
    return out


def _require_int(table: Mapping[str, str], key: str, label: str) -> int:
    value = table.get(key, "")
    if not value.isdigit():
        raise SizingError(f"{label} 缺少 {key}（⛔ 不得當成 0）")
    return int(value)


def expected_invocations(profile: str, replay_compute: str | None = None) -> list[tuple[str, str]]:
    spec = profile_spec(profile)
    expected = [(phase, role) for phase in spec["phases"] for role in spec["expected"][phase]]
    if profile == "acceptance":
        if replay_compute not in REPLAY_COMPUTES:
            raise SizingError(f"replay_compute 只接受 {REPLAY_COMPUTES}：{replay_compute!r}")
        if replay_compute == "full":
            expected.append(FULL_COMPUTE_EXTRA)
    return expected


def load_invocations(state: Path, profile: str = "sizing", replay_compute: str | None = None) -> list[dict[str, Any]]:
    """索引、sidecar、twin 三者的完整性（計畫書「二」的 invocation 索引；⑦d：依 profile 的封閉列舉）。"""
    spec = profile_spec(profile)
    indexes = sorted((state / "index").glob("*.json"))
    entries = [read_json(p) for p in indexes]
    sequences = [e["sequence"] for e in entries]
    if sequences != list(range(1, len(entries) + 1)):
        raise SizingError(f"invocation 的 sequence 不連續或重複：{sequences}")
    expected = expected_invocations(profile, replay_compute)
    got = [(e["phase"], e["role"]) for e in entries]
    if got != expected:
        raise SizingError(f"invocation 的 phase／role 與預期不符：預期 {expected}，實際 {got}")
    wrong = [e["sequence"] for e in entries if e.get("profile") != profile]
    if wrong:
        raise SizingError(f"invocation 索引的 profile 與本次（{profile}）不符：sequence {wrong}")
    sidecars = {p.stem for p in (state / "containers").glob("*.json")}
    twins = {p.stem for p in (state / "twins").glob("*.json")}
    for e in entries:
        if e["included_in_disk_path"] != (e["phase"] in measured_phases(profile)):
            raise SizingError(f"sequence {e['sequence']}：included_in_disk_path 與 phase 不相符")
        if e["id"] != container_id(e["sequence"]):
            raise SizingError(f"sequence {e['sequence']} 的容器 ID 不符：{e['id']}")
        if e["container_spec_sha256"] != spec_sha256(e["argv"], e["image"]):
            raise SizingError(f"sequence {e['sequence']} 的索引 hash 與 argv 不一致")
        for kind, pool, folder in (("sidecar", sidecars, "containers"), ("twin", twins, "twins")):
            if e["id"] not in pool:
                raise SizingError(f"sequence {e['sequence']} 缺 {kind}")
            other = read_json(state / folder / f"{e['id']}.json")
            if other["container_spec_sha256"] != e["container_spec_sha256"] or other["sequence"] != e["sequence"]:
                raise SizingError(f"sequence {e['sequence']} 的 {kind} 與索引不一致")
            if other["status"] != "ok":
                raise SizingError(f"sequence {e['sequence']} 的 {kind} 量測失敗：{other['failures']}")
            e[kind] = other
        if spec["size_rw_must_be_zero"] and e["sidecar"]["size_rw"] != 0:
            raise SizingError(f"sequence {e['sequence']} 的 SizeRw={e['sidecar']['size_rw']}（唯讀 rootfs 下必須是 0）")
    extra = (sidecars | twins) - {e["id"] for e in entries}
    if extra:
        raise SizingError(f"有沒有索引的 sidecar／twin：{sorted(extra)}")
    return entries


def load_events(state: Path) -> dict[tuple[str, str], list[int]]:
    events: dict[tuple[str, str], list[int]] = {}
    path = state / "events" / "events.jsonl"
    if path.is_file():
        for line in path.read_text(encoding="utf-8").splitlines():
            if line.strip():
                e = json.loads(line)
                events.setdefault((e["id"], e["kind"]), []).append(e["event"])
    return events


def combine_footprints(containers: list[dict[str, Any]], events: Mapping[tuple[str, str], list[int]]) -> tuple[int, str]:
    """同一路徑內的容器足跡：可證明不重疊 → 取最大；否則 → 相加（計畫書「二」的 lifecycle event）。

    可證明不重疊的條件（全部成立才算）：
    * 事件號在所有事件之間唯一，且只有 `create_begin`／`rm_done` 兩種；
    * 每個容器 `create_begin`、`rm_done` **各恰好一筆**，且 **`create_begin < rm_done`**；
    * 相鄰容器 `前一個.rm_done < 下一個.create_begin`。
    """
    sizes = [c["footprint"] for c in containers]
    numbers: list[int] = [n for key in events for n in events[key]]
    proven = len(numbers) == len(set(numbers)) and all(kind in ("create_begin", "rm_done") for _, kind in events)
    ordered = sorted(containers, key=lambda c: c["sequence"])
    spans: list[tuple[int, int]] = []
    for c in ordered:
        created = events.get((c["id"], "create_begin"), [])
        removed = events.get((c["id"], "rm_done"), [])
        if len(created) != 1 or len(removed) != 1 or not created[0] < removed[0]:
            proven = False
            break
        spans.append((created[0], removed[0]))
    if proven:
        proven = all(a[1] < b[0] for a, b in zip(spans, spans[1:]))
    return (max(sizes), "max_non_overlapping") if proven else (sum(sizes), "sum_overlap_or_unproven")


def phase_peaks(baseline: Mapping[str, Any], samples: list[Mapping[str, Any]]) -> dict[str, Any]:
    if not samples:
        raise SizingError("沒有任何取樣")
    dirs_peak, fs_peak, at_peak = 0, None, {}
    for s in samples:
        per = {loc: max(s[loc] - baseline["locations"][loc], 0) for loc in baseline["locations"]}
        total = sum(per.values())
        if total >= dirs_peak:
            dirs_peak, at_peak = total, per
        delta_fs = s["L0"] - baseline["L0_used"]
        fs_peak = delta_fs if fs_peak is None else max(fs_peak, delta_fs)
    return {"dirs_peak": dirs_peak, "dirs_peak_by_location": at_peak, "fs_peak": fs_peak, "samples": len(samples)}


def build_report(state: Path) -> dict[str, Any]:
    meta = _read_tsv(state / "meta.tsv")
    comp = _read_tsv(state / "components.tsv")
    rc = _read_tsv(state / "rc.tsv")
    for key in ("run_id", "mode", "image", "l0_dev", "docker_root_dev", "repo_dev", "state_fstype",
                "repo_head", "clone_head"):
        if not meta.get(key):
            raise SizingError(f"meta 缺少 {key}")
    if meta["state_fstype"] != "tmpfs":
        raise SizingError(f"S 不是 tmpfs：{meta['state_fstype']}")
    for key in ("docker_root_dev", "repo_dev"):
        if meta[key] != meta["l0_dev"]:
            raise SizingError(f"{key}={meta[key]} 與 L0 的 st_dev={meta['l0_dev']} 不同——⛔ 單一檔案系統的前提不成立")
    # ⚠️ ⑦d 實作第一輪 review：快照、工作複本、報告與 freeze record 綁 bootstrap 解析的同一個 commit（repo_head）。
    if meta["clone_head"] != meta["repo_head"]:
        raise SizingError(f"工作複本的 HEAD {meta['clone_head']} ≠ 記下的 repo_head {meta['repo_head']}")
    for step, value in rc.items():
        expected, _, actual = value.partition("\t")
        if expected != actual:
            raise SizingError(f"{step} 的結束碼 {actual} ≠ 預期 {expected}")
    for step in ("fixture_witness", "envcheck", "fixture_success", "finalize", "fixture_failure",
                 "publish_failed_record", "recover_envcheck", "recover_durability", "check_failed_record",
                 "recover_failed_record"):
        if step not in rc:
            raise SizingError(f"缺少 {step} 的結束碼")

    invocations = load_invocations(state)
    events = load_events(state)
    # ⚠️ 每一個 lifecycle event 的容器 ID 都必須屬於 invocation 索引——⛔ 不認得的 ID 直接 fail-closed
    #   （忽略它就可能在「看不到的容器」存在時錯用 max）。之後才依路徑過濾，交給 combine_footprints()。
    known_ids = {inv["id"] for inv in invocations}
    unknown = sorted({cid for cid, _kind in events} - known_ids)
    if unknown:
        raise SizingError(f"lifecycle event 裡有不屬於 invocation 索引的容器 ID：{unknown}")
    # ⚠️ 事件號來自**全域**的單調計數器——⛔ 必須在依路徑過濾**之前**驗唯一性（過濾之後，跨路徑的重複在每條路徑內
    #   都各自唯一，會被看漏）。重複代表計數器壞了，整份量測⛔ 不可信 → fail-closed。
    numbers = [n for key in events for n in events[key]]
    duplicated = sorted({n for n in numbers if numbers.count(n) > 1})
    if duplicated:
        raise SizingError(f"lifecycle event 的事件號重複（全域計數器損壞）：{duplicated}")
    for inv in invocations:
        inv["footprint"] = inv["sidecar"]["log_bound"] + inv["twin"]["adopted_bytes"] + inv["sidecar"]["size_rw"]

    wt = {name: _require_int(comp, name, "components") for name in (
        "wt_head", "wt_base", "wt_base_patched", "git_head", "git_base", "git_base_patched",
        "index_head", "index_base", "index_base_patched", "snapshot", "probe", "frozen_witness", "frozen_patched")}
    manifest, manifest_sha = load_harness_manifest(state, "sizing")
    paths: dict[str, Any] = {}
    for phase in DISK_PHASES:
        pdir = state / "phases" / phase
        if (pdir / "aborted").exists():
            raise SizingError(f"{phase} 的量測窗口已標成 aborted")
        baseline = read_json(pdir / "baseline.json")
        end = read_json(pdir / "end.json")
        for loc in LOCATIONS + ("L0",):
            if str(baseline["devices"][loc]) != meta["l0_dev"]:
                raise SizingError(f"{phase} 的 {loc} 與 L0 不在同一個檔案系統")
        if end["inventory_violations"]:
            raise SizingError(f"{phase} 的自我檢查不過：{end['inventory_violations'][:5]}")
        samples = [json.loads(line) for line in (pdir / "samples.jsonl").read_text().splitlines() if line.strip()]
        peaks = phase_peaks(baseline, samples)
        containers = [i for i in invocations if i["phase"] == phase]
        phase_ids = {c["id"] for c in containers}
        footprint, rule = combine_footprints(
            containers, {key: value for key, value in events.items() if key[0] in phase_ids})
        replay_key = "base" if phase == "witness" else "base_patched"
        parts = {
            "replay_worktree": wt[f"wt_{replay_key}"] + wt[f"git_{replay_key}"] + wt[f"index_{replay_key}"],
            "run_dir": end["run_dir"]["allocated"] + end["run_dir"]["dirs"] * BLOCK,
            "code_worktree": wt["wt_head"] + wt["git_head"] + wt["index_head"],
            "archive": end["archive"]["allocated"] + end["archive"]["dirs"] * BLOCK,
            "probe": wt["probe"],
            "containers": footprint,
            # ⚠️ ⑦d（「三」#15）：runner 以 exec docker run 結束，凍結副本留在 L3（⑤ 的模型沒有這一項）。
            "runner_frozen_patches": wt["frozen_witness" if phase == "witness" else "frozen_patched"],
        }
        if phase != "witness":
            parts["compose_worktree"] = wt["wt_base_patched"] + wt["git_base_patched"] + wt["index_base_patched"]
            parts["snapshot"] = wt["snapshot"]
        accounted = sum(parts.values())
        p_path = max(peaks["dirs_peak"], peaks["fs_peak"], accounted)
        paths[phase] = {"baseline": baseline, "peaks": peaks, "accounted": accounted, "accounted_parts": parts,
                        "container_rule": rule, "P_path": p_path}

    p_b = max(paths[p]["P_path"] for p in DISK_PHASES)
    for phase in DISK_PHASES:
        if paths[phase]["P_path"] > p_b:
            raise SizingError("報告自我檢查失敗：P_path > P_B")
    status = "ok" if paths["failure"]["P_path"] <= paths["success"]["P_path"] else "assumption_violated"
    mem_rows = sorted(({"sequence": i["sequence"], "phase": i["phase"], "role": i["role"],
                        "peak_bytes": i["sidecar"]["peak_bytes"], "rc": i["sidecar"]["rc"]}
                       for i in invocations), key=lambda r: r["sequence"])
    memavail = [int(line.split("\t")[1]) for line in (state / "memavail.tsv").read_text().splitlines() if line.strip()] \
        if (state / "memavail.tsv").is_file() else []
    return {
        "schema": "i074_stage2_sizing_report_v1",
        "status": status,
        "mode": meta["mode"],
        "meta": meta,
        "components": comp,
        "paths": paths,
        "P_B": p_b,
        "host_measurement_artifact_bytes": 0,
        "docker_instrumentation_overhead": "included_unseparated",
        "memory": mem_rows,
        "host_memavailable_low": min(memavail) if memavail else None,
        "harness_manifest": manifest,
        "harness_manifest_sha256": manifest_sha,
        "notes": ["check_failed_record 的 Python 段代量 preflight（「六、1」的 h）",
                  "fixture 的記憶體⛔ 不列入程序峰值"],
    }


def report_text(report: Mapping[str, Any]) -> str:
    mib = lambda b: f"{b / 1048576:.1f} MiB" if b is not None else "—"  # noqa: E731
    lines = [f"status: {report['status']}（mode={report['mode']}）", f"P_B = {report['P_B']} bytes（{mib(report['P_B'])}）", ""]
    for phase, info in report["paths"].items():
        pk = info["peaks"]
        lines.append(f"[{phase}] P_path={mib(info['P_path'])}  dirs_peak={mib(pk['dirs_peak'])}  "
                     f"fs_peak={mib(pk['fs_peak'])}  accounted={mib(info['accounted'])}  samples={pk['samples']}")
        for part, value in info["accounted_parts"].items():
            lines.append(f"    {part:<18} {mib(value)}")
        lines.append(f"    containers 合併規則：{info['container_rule']}")
    lines.append("")
    lines.append("記憶體（cgroup 峰值，含 page cache）：")
    for row in report["memory"]:
        lines.append(f"  #{row['sequence']:<3} {row['phase']:<12} {row['role']:<10} rc={row['rc']}  {mib(row['peak_bytes'])}")
    lines.append(f"host MemAvailable 低點：{mib(report['host_memavailable_low'])}")
    return "\n".join(lines) + "\n"


# ── ⑦d：⑩ 的正式程式（一律從工作複本載入，⛔ 不從活路徑） ─────────────────────────────

def _load_module(name: str, path: Path):
    """以路徑載入工作複本裡的 host 模組（`sys.modules` 先登記，dataclass 才找得到自己的模組）。"""
    if not path.is_file():
        raise SizingError(f"工作複本裡沒有 {path}")
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module


def clone_bootstrap(python_root: str | Path):
    """`<python root>/scripts/_i074_bootstrap.py`（⑦d：⛔ 不從 helper 自己的目錄——快照裡沒有它）。"""
    scripts = str(Path(python_root).resolve() / "scripts")
    if scripts not in sys.path:
        sys.path.insert(0, scripts)
    from _i074_bootstrap import STAGE2_MODULES, load_replay_bundle  # noqa: PLC0415
    return STAGE2_MODULES, load_replay_bundle


def acceptance_anchors(python_root: str | Path) -> dict[str, str]:
    """Stage 1 信任錨的 bundle 與兩個錨定檔的相對路徑（與 `i074_stage2_preflight.run_anchors()` 同一組推導，host 版）。"""
    from pathlib import PurePosixPath  # noqa: PLC0415

    modules, load = clone_bootstrap(python_root)
    s2 = load(Path(python_root), modules)["stage2_evidence"]
    anchor = s2.load_stage1_anchor(Path(python_root))
    root = PurePosixPath(s2.STAGE1_EVIDENCE_MANIFEST_PATH).parent
    return {"bundle_id": anchor.bundle_id, "after_base_commit": anchor.after_base_commit,
            "after_artifact": str(root / s2.STAGE1_ANCHOR_AFTER), "cohort_manifest": str(root / s2.STAGE1_ANCHOR_COHORT)}


_ARGV_SENTINEL = "/__i074_acceptance_work__"


def acceptance_replay_argv(clone: str | Path, run_dir: str | Path, anchors: Mapping[str, str]) -> list[str]:
    """⑩ 的 replay argv（**唯一的組裝處** `i074_stage2_preflight.replay_argv()`），再把兩個路徑前綴對應到 acceptance。"""
    pf = _load_module("i074_stage2_preflight", Path(clone) / "python" / "scripts" / "i074_stage2_preflight.py")
    pre = {"bundle_id": anchors["bundle_id"], "base_commit": anchors["after_base_commit"],
           "after_artifact": anchors["after_artifact"], "cohort_manifest": anchors["cohort_manifest"]}
    out = []
    for tok in pf.replay_argv(_ARGV_SENTINEL, pre):
        if tok == f"{_ARGV_SENTINEL}/run/stage2":
            tok = f"{run_dir}/stage2"
        elif tok.startswith(f"{_ARGV_SENTINEL}/repo/"):
            tok = f"{clone}/{tok[len(_ARGV_SENTINEL) + len('/repo/'):]}"
        if _ARGV_SENTINEL in tok:
            raise SizingError(f"replay argv 有對應不到的路徑：{tok}")
        out.append(tok)
    return out


def clone_clean_env(clone: str | Path, environ: Mapping[str, str]) -> dict[str, str]:
    """⑩ 的環境清理（supervisor 的 `clean_env()`，唯一定義）。"""
    sup = _load_module("i074_stage2_supervisor", Path(clone) / "scripts" / "lib" / "i074-stage2-supervisor.py")
    return sup.clean_env(dict(environ))


def promotion_states(clone: str | Path, *, identity: str | Path, bundle_id: str, semantic: str, repo_head: str,
                     rc: int) -> dict[str, dict[str, Any]]:
    """晉升從 state 讀的五個欄位（⑦d「二之三」）；`identity_sha256` 用 orchestrator 寫 preflight.json 的同一個算法。"""
    pf = _load_module("i074_stage2_preflight", Path(clone) / "python" / "scripts" / "i074_stage2_preflight.py")
    if rc not in (0, 6):
        raise SizingError(f"replay 的結束碼只能是 0 或 6：{rc}")
    return {"preflight": {"bundle_id": bundle_id, "counterfactual_semantic_sha256": semantic,
                          "identity_sha256": pf._sha256_file(identity)},
            "replay_done": {"rc": rc}, "run": {"repo_head": repo_head}}


def promote_measure(clone: str | Path, *, work: str | Path, real: str | Path, states: Mapping[str, Any]) -> int:
    """以工作複本的晉升模組跑第 2～7 步（git 與驗證模式都是真的；state 由 harness 注入、⛔ 不寫 state 檔）。

    ⚠️ 第 1 步（`/proc/locks`、preflight 0、state 檔的驗證）屬 orchestrator，⛔ 不在這裡。結束碼的對應與 `main()` 相同。
    """
    pm = _load_module("i074_stage2_promote", Path(clone) / "python" / "scripts" / "i074_stage2_promote.py")
    try:
        rc = pm.Promoter(str(work), str(real), load_states=lambda _w, _r: dict(states)).run()
    except pm.PromotionExit as exc:
        print(f"ERROR: 晉升（rc={exc.code}）：{exc}", file=sys.stderr)
        return exc.code
    except Exception as exc:  # noqa: BLE001 - 與 main() 相同：未列出的例外一律 9
        print(f"ERROR: 晉升：未預期的例外 {type(exc).__name__}: {exc}", file=sys.stderr)
        return pm.EXIT_PROMOTION_BLOCKED
    return rc


# ── ⑦d：host 端的程序樹（每一步都經 host-run） ─────────────────────────────────

def group_rss(pgid: int, proc: Path = Path("/proc")) -> int:
    """同一個 process group 目前的 RSS 總和（`VmRSS`；zombie 與讀不到的程序略過）。⚠️ 取樣值，⛔ 不是峰值的證明。"""
    total = 0
    for entry in proc.iterdir():
        if not entry.name.isdigit():
            continue
        try:
            fields = (entry / "stat").read_text().rsplit(")", 1)[1].split()
            if int(fields[2]) != pgid:
                continue
            for line in (entry / "status").read_text().splitlines():
                if line.startswith("VmRSS:"):
                    total += int(line.split()[1]) * 1024
                    break
        except (OSError, IndexError, ValueError):
            continue
    return total


def host_run(state: Path, step: str, cmd: list[str], *, interval: float = 0.1) -> int:
    """執行一步並記 host 端程序樹的記憶體（⑦d「二之三」）：

    * `max_single_rss_bytes`：`RUSAGE_CHILDREN` 的 `ru_maxrss`——所有已 wait 的子孫裡最大的**單一**程序（各自的高水位，
      **精確**；v26 的契約是每一個程序各自，所以它是門檻）；
    * `group_rss_peak_sampled_bytes`：每 `interval` 秒把同一個 process group 的 `VmRSS` 加總、取最大（**下界**，只作單向警報）。
    ⚠️ RSS ⛔ 不含 page cache，與容器的 cgroup 峰值是不同的量法；容器裡的程序屬於 docker daemon，⛔ 不在這棵樹裡。
    """
    if not cmd:
        raise SizingError("host-run 沒有指令")
    pgid = os.getpgrp()
    proc = subprocess.Popen(cmd)
    peak = samples = 0
    while proc.poll() is None:
        peak = max(peak, group_rss(pgid))
        samples += 1
        time.sleep(interval)
    rc = proc.returncode if proc.returncode >= 0 else 128 - proc.returncode
    maxrss = resource.getrusage(resource.RUSAGE_CHILDREN).ru_maxrss * 1024
    # ⚠️ 環境只記**名稱**（⛔ 不記值）：報告以它驗「clean_env() ＋ 固定 PATH ＋ 量測變數」與兩份 patch 只給 replay（「三」#12）。
    write_exclusive(state / "host" / f"{step}.json", canonical_dumps({
        "step": step, "rc": rc, "max_single_rss_bytes": maxrss, "group_rss_peak_sampled_bytes": peak,
        "samples": samples, "interval_s": interval, "env_keys": sorted(os.environ), "cmd": list(cmd)}))
    return rc


# ── ⑦d：acceptance 的報告 ──────────────────────────────────────────────────────

ACCEPTANCE_STEPS = (  # (步驟名, phase)；⚠️ 順序 ＝ harness 執行的順序
    ("check_failed_record_preflight", "preflight"), ("replay_success", "success"), ("finalize", "success"),
    ("promote_success", "promote_success"), ("replay_failure", "failure"), ("publish_failed_record", "failure"),
    ("promote_failure", "promote_failure"), ("check_failed_record", "memory_only"),
    ("recover_durability", "memory_only"), ("recover_failed_record", "memory_only"), ("envcheck", "memory_only"),
    ("recover_envcheck", "memory_only"),
)
ACCEPTANCE_FULL_STEP = ("replay_full", "memory_only")


def _check_events(invocations: list[dict[str, Any]], events: Mapping[tuple[str, str], list[int]]) -> None:
    known_ids = {inv["id"] for inv in invocations}
    unknown = sorted({cid for cid, _kind in events} - known_ids)
    if unknown:
        raise SizingError(f"lifecycle event 裡有不屬於 invocation 索引的容器 ID：{unknown}")
    numbers = [n for key in events for n in events[key]]
    duplicated = sorted({n for n in numbers if numbers.count(n) > 1})
    if duplicated:
        raise SizingError(f"lifecycle event 的事件號重複（全域計數器損壞）：{duplicated}")


def _phase_window(state: Path, phase: str, meta: Mapping[str, str]) -> tuple[dict, dict, dict]:
    pdir = state / "phases" / phase
    if (pdir / "aborted").exists():
        raise SizingError(f"{phase} 的量測窗口已標成 aborted")
    baseline = read_json(pdir / "baseline.json")
    end = read_json(pdir / "end.json")
    for loc, dev in baseline["devices"].items():
        if str(dev) != meta["l0_dev"]:
            raise SizingError(f"{phase} 的 {loc} 與 L0 不在同一個檔案系統")
    if end["inventory_violations"]:
        raise SizingError(f"{phase} 的自我檢查不過：{end['inventory_violations'][:5]}")
    samples = [json.loads(line) for line in (pdir / "samples.jsonl").read_text().splitlines() if line.strip()]
    return baseline, end, phase_peaks(baseline, samples)


HARNESS_ENV_ADDED = ("PATH", "PYTHONDONTWRITEBYTECODE")     # clean_env() 會丟掉、由 harness 固定加回的
PATCH_ENV_NAMES = ("COUNTERFACTUAL_PATCH", "TOOLING_PATCH")


def check_step_environment(rec: Mapping[str, Any], *, drop_names: Iterable[str], drop_prefixes: tuple[str, ...]) -> list[str]:
    """⑦d「三」#12：每一步的環境 ＝ supervisor 的 `clean_env()` ＋ 固定的 PATH ＋ 量測變數（`SIZING_*`）；兩份 patch 的變數
    **只**出現在 replay 那一步的指令（`env COUNTERFACTUAL_PATCH=… TOOLING_PATCH=… <runner>`）。回傳違規。"""
    drop_names = set(drop_names)
    problems = []
    for key in rec["env_keys"]:
        dropped = key in drop_names or key.startswith(drop_prefixes)
        if dropped and key not in HARNESS_ENV_ADDED and not key.startswith("SIZING_"):
            problems.append(f"{rec['step']}：環境帶 {key}（clean_env() 應該丟掉它）")
    in_cmd = {name for name in PATCH_ENV_NAMES for tok in rec["cmd"] if tok.startswith(name + "=")}
    if rec["step"].startswith("replay_"):
        if in_cmd != set(PATCH_ENV_NAMES):
            problems.append(f"{rec['step']}：replay 的指令缺少兩份 patch 的變數：{sorted(set(PATCH_ENV_NAMES) - in_cmd)}")
    elif in_cmd:
        problems.append(f"{rec['step']}：兩份 patch 的變數只能給 replay，卻出現在 {sorted(in_cmd)}")
    if set(PATCH_ENV_NAMES) & set(rec["env_keys"]):
        problems.append(f"{rec['step']}：兩份 patch 的變數出現在 host-run 自己的環境（應該只在 replay 的指令裡）")
    return problems


# ⑦d 實作第一輪 review #2：容器當次的實際記憶體上限（mem-guard 下修後的 --memory）。docker 的大小字串照 go-units 的
#   RAMInBytes：1024 進位，單位 b／k／m／g／t／p（大小寫不拘，後面可接 i、b）；⛔ 不接受負數（`--memory-swap=-1` ＝ swap 無上限）
#   與其他寫法——讀不懂一律 fail-closed。
_DOCKER_SIZE = re.compile(r"(\d+(?:\.\d+)?) ?([kmgtp])?i?b?", re.IGNORECASE)
_DOCKER_UNIT_POWER = {"": 0, "k": 1, "m": 2, "g": 3, "t": 4, "p": 5}


def docker_memory_bytes(text: str) -> int:
    match = _DOCKER_SIZE.fullmatch(text)
    if not match:
        raise SizingError(f"讀不懂 docker 的記憶體大小：{text!r}")
    return int(float(match.group(1)) * 1024 ** _DOCKER_UNIT_POWER[(match.group(2) or "").lower()])


def argv_memory_options(argv: list[str], image: str) -> tuple[str, str]:
    """封存的 container spec（**image 之前**）裡恰好一個 `--memory`（或 `-m`）與恰好一個 `--memory-swap` 的值。"""
    pos = _image_position(argv, image)
    found: dict[str, list[str]] = {"--memory": [], "--memory-swap": []}
    i = 0
    while i < pos:
        token = argv[i]
        name = "--memory" if token in ("--memory", "-m") else "--memory-swap" if token == "--memory-swap" else ""
        if name:
            if i + 1 >= pos:
                raise SizingError(f"{token} 沒有值（下一個 token 就是 image）")
            found[name].append(argv[i + 1])
            i += 2
            continue
        for flag in ("--memory=", "--memory-swap="):
            if token.startswith(flag):
                found[flag[:-1]].append(token[len(flag):])
        i += 1
    for name, values in found.items():
        if len(values) != 1:
            raise SizingError(f"container spec 裡要恰好一個 {name}，實際 {len(values)} 個：{values}")
    return found["--memory"][0], found["--memory-swap"][0]


def container_memory_limit(inv: Mapping[str, Any]) -> int:
    """一個 invocation 當次的實際上限：sidecar 以 `docker inspect` 讀到的 `HostConfig.Memory`／`MemorySwap`（daemon 實際套用的）
    必須各自 ＝ 封存的 argv（index；hash 綁住）裡的 `--memory`／`--memory-swap`，而且 swap ＝ memory——可以用 swap 時
    cgroup v1 的 `memory.max_usage_in_bytes` 不含被換出的部分，峰值會低估。任何一項不成立 → fail-closed（⛔ 不產報告）。"""
    seq, side = inv["sequence"], inv["sidecar"]
    mem, swap = side.get("memory_limit_bytes"), side.get("memory_swap_limit_bytes")
    if type(mem) is not int or type(swap) is not int:
        raise SizingError(f"sequence {seq} 的容器記憶體上限讀不到（sidecar：{mem!r}／{swap!r}）")
    if mem <= 0:
        raise SizingError(f"sequence {seq} 的容器沒有記憶體上限（HostConfig.Memory={mem}）")
    want_mem, want_swap = argv_memory_options(inv["argv"], inv["image"])
    for flag, text, actual in (("--memory", want_mem, mem), ("--memory-swap", want_swap, swap)):
        if docker_memory_bytes(text) != actual:
            raise SizingError(f"sequence {seq}：封存的 argv 的 {flag}={text}（{docker_memory_bytes(text)} bytes）"
                              f"≠ daemon 實際套用的 {actual}")
    if swap != mem:
        raise SizingError(f"sequence {seq} 的 MemorySwap={swap} ≠ Memory={mem}：容器可以用 swap，cgroup 峰值會低估")
    return mem


def build_acceptance_report(state: Path, *, p_b_budget: int, drop_names: Iterable[str] = (),
                            drop_prefixes: tuple[str, ...] = ()) -> dict[str, Any]:
    """⑦d「二之五」：兩條磁碟路徑（門檻）、兩次晉升（資訊值）、每個容器與每一步 host 程序樹的記憶體（門檻與單向警報）。"""
    meta = _read_tsv(state / "meta.tsv")
    comp = _read_tsv(state / "components.tsv")
    rc = _read_tsv(state / "rc.tsv")
    for key in ("run_id", "mode", "image", "l0_dev", "docker_root_dev", "repo_dev", "state_fstype", "replay_compute",
                "repo_head", "clone_head"):
        if not meta.get(key):
            raise SizingError(f"meta 缺少 {key}")
    if meta["state_fstype"] != "tmpfs":
        raise SizingError(f"S 不是 tmpfs：{meta['state_fstype']}")
    for key in ("docker_root_dev", "repo_dev"):
        if meta[key] != meta["l0_dev"]:
            raise SizingError(f"{key}={meta[key]} 與 L0 的 st_dev={meta['l0_dev']} 不同——⛔ 單一檔案系統的前提不成立")
    if meta["clone_head"] != meta["repo_head"]:
        raise SizingError(f"工作複本的 HEAD {meta['clone_head']} ≠ 記下的 repo_head {meta['repo_head']}")
    compute = meta["replay_compute"]
    steps = list(ACCEPTANCE_STEPS) + ([ACCEPTANCE_FULL_STEP] if compute == "full" else [])
    for step, value in rc.items():
        expected, _, actual = value.partition("\t")
        if expected != actual:
            raise SizingError(f"{step} 的結束碼 {actual} ≠ 預期 {expected}")
    if list(rc) != [s for s, _ in steps]:
        raise SizingError(f"步驟與預期不符：預期 {[s for s, _ in steps]}，實際 {list(rc)}")

    invocations = load_invocations(state, "acceptance", compute)
    events = load_events(state)
    _check_events(invocations, events)
    for inv in invocations:
        inv["footprint"] = inv["sidecar"]["log_bound"] + inv["twin"]["adopted_bytes"] + inv["sidecar"]["size_rw"]

    wt = {name: _require_int(comp, name, "components") for name in (
        "wt_head", "wt_base_patched", "git_head", "git_base_patched", "index_head", "index_base_patched",
        "snapshot", "probe", "frozen_patched")}
    manifest, manifest_sha = load_harness_manifest(state, "acceptance")
    violations: list[str] = []
    paths: dict[str, Any] = {}
    for phase in PROFILES["acceptance"]["disk_phases"]:
        baseline, end, peaks = _phase_window(state, phase, meta)
        containers = [i for i in invocations if i["phase"] == phase]
        ids = {c["id"] for c in containers}
        footprint, rule = combine_footprints(containers, {k: v for k, v in events.items() if k[0] in ids})
        replay = wt["wt_base_patched"] + wt["git_base_patched"] + wt["index_base_patched"]
        parts = {
            "replay_worktree": replay,
            "compose_worktree": replay,
            "code_worktree": wt["wt_head"] + wt["git_head"] + wt["index_head"],
            "snapshot": wt["snapshot"],
            "runner_frozen_patches": wt["frozen_patched"],
            "run_dir": end["run_dir"]["allocated"] + end["run_dir"]["dirs"] * BLOCK,
            "archive": end["archive"]["allocated"] + end["archive"]["dirs"] * BLOCK,
            "probe": wt["probe"],
            "containers": footprint,
        }
        accounted = sum(parts.values())
        p_path = max(peaks["dirs_peak"], peaks["fs_peak"], accounted)
        if p_path > p_b_budget:
            violations.append(f"磁碟：{phase} 的 P_path={p_path} > P_B_BUDGET={p_b_budget}")
        paths[phase] = {"peaks": peaks, "accounted": accounted, "accounted_parts": parts, "container_rule": rule,
                        "P_path": p_path, "read_only_gap": sum(c["sidecar"]["size_rw"] for c in containers)}
    promotion: dict[str, Any] = {}
    for phase, path in (("promote_success", "success"), ("promote_failure", "failure")):
        baseline, end, peaks = _phase_window(state, phase, meta)
        containers = [i for i in invocations if i["phase"] == phase]
        footprint = sum(c["footprint"] for c in containers)
        accounted = end["archive"]["allocated"] + end["archive"]["dirs"] * BLOCK + footprint
        p_prom = max(peaks["dirs_peak"], peaks["fs_peak"], accounted)
        promotion[phase] = {"peaks": peaks, "accounted": accounted, "P_promotion": p_prom,
                            "P_path_plus_promotion": paths[path]["P_path"] + p_prom,
                            "kind": "informational（⛔ 不與 P_B_BUDGET 比較；v29「八之一」容量列）"}
    memory = []
    for inv in invocations:
        peak = inv["sidecar"]["peak_bytes"]
        limit = container_memory_limit(inv)
        ok = peak < ACCEPTANCE_MEMORY_LIMIT
        if not ok:
            violations.append(f"記憶體：#{inv['sequence']} {inv['phase']}/{inv['role']} 的 cgroup 峰值 {peak} ≥ {ACCEPTANCE_MEMORY_LIMIT}")
        memory.append({"sequence": inv["sequence"], "phase": inv["phase"], "role": inv["role"], "rc": inv["sidecar"]["rc"],
                       "cgroup_peak_bytes": peak, "memory_limit_bytes": limit, "below_limit": ok})
    container_limits = sorted({row["memory_limit_bytes"] for row in memory})
    host = []
    for step, phase in steps:
        path = state / "host" / f"{step}.json"
        if not path.is_file():
            raise SizingError(f"缺少 {step} 的 host 端量測")
        rec = read_json(path)
        env_problems = check_step_environment(rec, drop_names=drop_names, drop_prefixes=drop_prefixes)
        if env_problems:
            raise SizingError(f"環境的契約不符（harness 的缺陷，⛔ 不產報告）：{env_problems}")
        single, sampled = rec["max_single_rss_bytes"], rec["group_rss_peak_sampled_bytes"]
        if not single or not isinstance(single, int):
            raise SizingError(f"{step} 的最大單一程序 RSS 讀不到或為 0")
        if single >= ACCEPTANCE_MEMORY_LIMIT:
            violations.append(f"記憶體：{step} 的 host 最大單一程序 RSS {single} ≥ {ACCEPTANCE_MEMORY_LIMIT}")
        alarm = sampled >= ACCEPTANCE_MEMORY_LIMIT
        if alarm:
            violations.append(f"單向警報：{step} 的 host 程序群組 RSS 取樣總和 {sampled} ≥ {ACCEPTANCE_MEMORY_LIMIT}")
        host.append({"step": step, "phase": phase, "rc": rec["rc"], "max_single_rss_bytes": single,
                     "group_rss_peak_sampled_bytes": sampled, "group_alarm": alarm,
                     "group_note": "觀察到超標" if alarm else "未觀察到超標（⛔ 不是通過的證據）"})
    memavail = [int(line.split("\t")[1]) for line in (state / "memavail.tsv").read_text().splitlines() if line.strip()] \
        if (state / "memavail.tsv").is_file() else []
    notes = ["anchors 以 preflight 的 --check-failed-record 代量（⑦d「三」#6）",
             "host 端是 RSS（⛔ 不含 page cache）：最大單一程序是門檻、程序群組的取樣總和只作單向警報",
             "晉升的磁碟只列資訊值（P_B 的起訖⛔ 不變）"]
    # ⚠️ mem-guard 每一次都依當下的 MemAvailable 下修，上限因容器而異（實測 402～532 MiB）——兩則說明各自只列它涵蓋的容器。
    seqs = lambda rows: "、".join(f"#{row['sequence']}" for row in rows)  # noqa: E731
    bounded = [row for row in memory if row["memory_limit_bytes"] <= ACCEPTANCE_MEMORY_LIMIT]
    rising = [row for row in memory if row["memory_limit_bytes"] > ACCEPTANCE_MEMORY_LIMIT]
    if bounded:
        notes.append(f"容器 {seqs(bounded)} 的 cgroup 上限（當次 mem-guard 下修後的 --memory）不高於門檻：含 page cache 的峰值"
                     "不會超過上限（逼近上限時 page cache 先被回收），這幾個容器的門檻判定實質是「在這個上限內、⛔ 不用 swap、"
                     "以預期的結束碼跑完」")
    if rising:
        notes.append(f"容器 {seqs(rising)} 的上限高於門檻：cgroup 峰值含 page cache，而 page cache 要逼近上限才被回收——"
                     "峰值會隨當次的上限上升（同一個程序在較高的上限下可能量到較高的峰值；偏保守的方向）")
    if compute == "stub":
        notes.insert(0, "⚠️ replay_compute=stub：計算工作集⛔ 未涵蓋——⛔ 不得當成 ⑨-1 的正式驗收")
    return {
        "schema": "i074_stage2_acceptance_report_v1",
        "status": "ok" if not violations else "threshold_exceeded",
        "violations": violations,
        "mode": meta["mode"],
        "replay_compute": compute,
        "meta": meta,
        "components": comp,
        "harness_manifest": manifest,
        "harness_manifest_sha256": manifest_sha,
        "limits": {"memory_bytes": ACCEPTANCE_MEMORY_LIMIT, "P_B_BUDGET": p_b_budget,
                   "container_memory_limit_bytes": container_limits},
        "paths": paths,
        "promotion": promotion,
        "memory": memory,
        "host": host,
        "host_memavailable_low": min(memavail) if memavail else None,
        "notes": notes,
    }


def acceptance_report_text(report: Mapping[str, Any]) -> str:
    mib = lambda b: f"{b / 1048576:.1f} MiB" if b is not None else "—"  # noqa: E731
    lines = [f"status: {report['status']}（mode={report['mode']}、replay_compute={report['replay_compute']}）"]
    lines += [f"  ⚠️ {v}" for v in report["violations"]]
    lines += [f"門檻：記憶體 < {mib(report['limits']['memory_bytes'])}、磁碟 ≤ P_B_BUDGET {mib(report['limits']['P_B_BUDGET'])}", ""]
    for phase, info in report["paths"].items():
        pk = info["peaks"]
        lines.append(f"[{phase}] P_path={mib(info['P_path'])}  dirs_peak={mib(pk['dirs_peak'])}  fs_peak={mib(pk['fs_peak'])}  "
                     f"accounted={mib(info['accounted'])}  read_only_gap={mib(info['read_only_gap'])}")
        for part, value in info["accounted_parts"].items():
            lines.append(f"    {part:<22} {mib(value)}")
    for phase, info in report["promotion"].items():
        lines.append(f"[{phase}]（資訊值）P_promotion={mib(info['P_promotion'])}  P_path＋晉升={mib(info['P_path_plus_promotion'])}")
    lines += ["", "容器（cgroup 峰值，含 page cache；上限 ＝ 當次 mem-guard 下修後的 --memory）："]
    for row in report["memory"]:
        lines.append(f"  #{row['sequence']:<3} {row['phase']:<16} {row['role']:<10} rc={row['rc']}  "
                     f"{mib(row['cgroup_peak_bytes'])}／上限 {mib(row['memory_limit_bytes'])}")
    lines.append("host 端程序樹（RSS，⛔ 不含 page cache）：")
    for row in report["host"]:
        lines.append(f"  {row['step']:<30} 最大單一 {mib(row['max_single_rss_bytes'])}  群組取樣 "
                     f"{mib(row['group_rss_peak_sampled_bytes'])}（{row['group_note']}）")
    lines.append(f"host MemAvailable 低點：{mib(report['host_memavailable_low'])}")
    lines += [""] + [f"註：{n}" for n in report["notes"]]
    return "\n".join(lines) + "\n"


# ── ⑦d：⑩ 期間的 observer（外部、唯讀；⛔ 不是 ⑩ 的一部分） ───────────────────────────

OBSERVE_LABEL_KEY = "i074.stage2.run"     # ⚠️ ＝ label shim、orchestrator、supervisor 的常數（測試釘住）
OBSERVE_EXPECTED = ("anchors", "lookup", "replay", "terminal", "promotion_verify")
_SHIM_MARKERS = ("I074-STAGE2-LABEL-SHIM", "I074-SIZING-DOCKER-SHIM")


def classify_container(cmd: list[str]) -> str:
    """依容器指令辨識 ⑩ 的角色（⛔ 不猜：認不得 → other）。"""
    if "backtest.modular.sr_scoring.evaluation" in cmd:
        return "replay"
    for flag, role in (("--verify-promotion-staging", "promotion_verify"), ("--check-failed-record", "lookup"),
                       ("--publish-failed-record", "publish"), ("--finalize", "finalize")):
        if flag in cmd:
            return role
    if "anchors" in cmd and any(t.endswith("i074_stage2_preflight.py") for t in cmd):
        return "anchors"
    return "other"


def cgroup_peak_paths(cid: str, root: str | Path = "/sys/fs/cgroup") -> list[Path]:
    root = Path(root)
    return [root / "memory" / "docker" / cid / "memory.max_usage_in_bytes",         # v1、cgroupfs（本 host）
            root / "system.slice" / f"docker-{cid}.scope" / "memory.peak",          # v2、systemd
            root / "docker" / cid / "memory.peak"]                                   # v2、cgroupfs


class Observer:
    """⑩ 的實際峰值紀錄（⑦d「二之六」）：記憶體是 cgroup high-water mark 的**下界**、磁碟是取樣值。"""

    def __init__(self, work_dir: str | Path, *, docker: str = "docker", cgroup_root: str | Path = "/sys/fs/cgroup",
                 fs_path: str | Path = "/") -> None:
        self.work = Path(work_dir)
        self.docker = docker
        self.cgroup_root = cgroup_root
        self.fs_path = fs_path
        self.containers: dict[str, dict[str, Any]] = {}
        self.polls = 0
        self.first_poll_empty: bool | None = None
        self.last_poll_ids: set[str] = set()
        self.fs_baseline = fs_used(fs_path)
        self.fs_peak_delta = 0
        self.work_peak = 0
        self.memavail_low: int | None = None

    def poll_containers(self, now: float) -> None:
        out = subprocess.run([self.docker, "ps", "-a", "-q", "--no-trunc", "--filter", f"label={OBSERVE_LABEL_KEY}"],
                             capture_output=True, text=True)
        if out.returncode != 0:
            raise SizingError(f"docker ps 失敗：{out.stderr.strip()[:200]}")
        ids = {line.strip() for line in out.stdout.splitlines() if line.strip()}
        self.polls += 1
        if self.first_poll_empty is None:
            self.first_poll_empty = not ids
        for cid in ids - set(self.containers):
            ins = subprocess.run([self.docker, "inspect", "--format", "{{json .Config.Cmd}}", cid],
                                 capture_output=True, text=True)
            try:
                cmd = json.loads(ins.stdout) if ins.returncode == 0 else []
            except json.JSONDecodeError:
                cmd = []
            self.containers[cid] = {"id": cid, "role": classify_container(cmd or []), "cmd_head": (cmd or [])[:6],
                                    "first_seen": now, "gone_at": None, "peak_bytes": None, "reads": 0, "last_read": None}
        for cid in set(self.containers) - ids:
            if self.containers[cid]["gone_at"] is None:
                self.containers[cid]["gone_at"] = now
        self.last_poll_ids = ids

    def read_peaks(self, now: float) -> None:
        for info in self.containers.values():
            if info["gone_at"] is not None:
                continue
            for path in cgroup_peak_paths(info["id"], self.cgroup_root):
                try:
                    value = int(path.read_text().strip())
                except (OSError, ValueError):
                    continue
                info["peak_bytes"] = value if info["peak_bytes"] is None else max(info["peak_bytes"], value)
                info["reads"] += 1
                info["last_read"] = now
                break

    def sample_fs(self) -> None:
        self.fs_peak_delta = max(self.fs_peak_delta, fs_used(self.fs_path) - self.fs_baseline)

    def sample_work(self) -> None:
        if self.work.is_dir():
            self.work_peak = max(self.work_peak, allocated_tree(self.work)["allocated"])

    def sample_memavail(self) -> None:
        for line in Path("/proc/meminfo").read_text().splitlines():
            if line.startswith("MemAvailable:"):
                value = int(line.split()[1]) * 1024
                self.memavail_low = value if self.memavail_low is None else min(self.memavail_low, value)

    def report(self) -> dict[str, Any]:
        roles = {c["role"] for c in self.containers.values() if c["reads"] > 0}
        seen = {"anchors": "anchors" in roles, "lookup": "lookup" in roles, "replay": "replay" in roles,
                "terminal": bool(roles & {"finalize", "publish"}), "promotion_verify": "promotion_verify" in roles}
        missing = [r for r in OBSERVE_EXPECTED if not seen[r]]
        unavailable = sorted(c["id"] for c in self.containers.values() if c["peak_bytes"] is None)
        complete = (seen["replay"] and not missing and not unavailable and bool(self.first_poll_empty)
                    and not self.last_poll_ids)
        return {
            "schema": "i074_stage2_observation_v1",
            "label_key": OBSERVE_LABEL_KEY,
            "observation_complete": complete,
            "replay_seen": seen["replay"],
            "missing_expected_containers": missing,
            "unavailable_containers": unavailable,
            "started_before_first_container": bool(self.first_poll_empty),
            "stopped_after_last_container": not self.last_poll_ids,
            "containers": [dict(c, peak_kind="observed_lower_bound") for c in
                           sorted(self.containers.values(), key=lambda c: c["first_seen"])],
            "fs_peak_delta_bytes": {"value": self.fs_peak_delta, "kind": "sampled"},
            "work_dir_peak_allocated_bytes": {"value": self.work_peak, "kind": "sampled"},
            "host_memavailable_low": {"value": self.memavail_low, "kind": "sampled"},
            "polls": self.polls,
            "notes": ["記憶體是 cgroup high-water mark 的**下界**：最後一次讀取之後、容器結束之前的尾段峰值可能漏記（run-evaluation.sh 的教訓）",
                      "磁碟是取樣值（live 服務的寫入會墊高或壓低 L0）——⛔ 不是 P_B 的量法、⛔ 不與 P_B_BUDGET 比較",
                      "只額外記錄（v29「八」）：⛔ 不作為 ⑩ 的前置、⛔ 不為量測而重跑"],
        }


def observe_guards(work_dir: str, out: str, environ: Mapping[str, str], docker_path: str | None,
                   repo_root: str | Path) -> None:
    bad = sorted(k for k in environ if k.startswith(("I074_STAGE2_", "SIZING_")))
    if bad:
        raise SizingError(f"環境帶 {bad}——observer ⛔ 不在 ⑩ 或 harness 的流程裡執行")
    if not docker_path:
        raise SizingError("找不到 docker")
    try:
        head = Path(docker_path).read_bytes()[:4096].decode("utf-8", "replace")
    except OSError:
        head = ""
    if any(marker in head for marker in _SHIM_MARKERS):
        raise SizingError(f"PATH 上的 docker 是 shim：{docker_path}（經 shim 看到的⛔ 不是 ⑩ 的真實狀態）")
    out_real = os.path.realpath(out)
    if os.path.lexists(out):
        raise SizingError(f"--out 已存在：{out}（⛔ 不覆蓋）")
    if not os.path.isdir(os.path.dirname(out_real)):
        raise SizingError(f"--out 的上層目錄不存在：{out}")
    for root in (os.path.realpath(repo_root), os.path.realpath(work_dir)):
        if out_real == root or out_real.startswith(root.rstrip("/") + "/"):
            raise SizingError(f"--out ⛔ 不得在 repo 或 --work-dir 內：{out}")


def run_observer(observer: Observer, state: Path, stop: Callable[[], bool], *, tick: float = 0.5) -> dict[str, Any]:
    """每 0.5 秒讀 cgroup 與 L0；每 2 秒 `docker ps` 與 MemAvailable；每 10 秒量 --work-dir；收到停止訊號才結束。"""
    n = 0
    while True:
        now = time.monotonic()
        if n % 4 == 0:
            observer.poll_containers(now)
            observer.sample_memavail()
        observer.read_peaks(now)
        observer.sample_fs()
        if n % 20 == 0:
            observer.sample_work()
            (state / "observation.partial.json").write_bytes(canonical_dumps(observer.report()))
        if stop():
            observer.poll_containers(time.monotonic())
            observer.read_peaks(time.monotonic())
            return observer.report()
        n += 1
        time.sleep(tick)


def observation_text(report: Mapping[str, Any]) -> str:
    mib = lambda b: f"{b / 1048576:.1f} MiB" if b is not None else "—"  # noqa: E731
    lines = [f"observation_complete: {report['observation_complete']}（replay_seen={report['replay_seen']}、"
             f"missing={report['missing_expected_containers']}、unavailable={report['unavailable_containers']}）",
             "記憶體（cgroup high-water mark 的下界）："]
    for c in report["containers"]:
        lines.append(f"  {c['role']:<18} {c['id'][:12]}  {mib(c['peak_bytes'])}（讀 {c['reads']} 次）")
    lines.append(f"L0 增量（取樣）：{mib(report['fs_peak_delta_bytes']['value'])}；--work-dir（取樣）："
                 f"{mib(report['work_dir_peak_allocated_bytes']['value'])}")
    lines += [f"註：{n}" for n in report["notes"]]
    return "\n".join(lines) + "\n"


# ── CLI ───────────────────────────────────────────────────────────────────────

def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(prog="i074_stage2_sizing", allow_abbrev=False)
    sub = parser.add_subparsers(dest="cmd", required=True)

    p = sub.add_parser("fixture")
    p.add_argument("--phase", required=True, choices=DISK_PHASES)
    p.add_argument("--python-root", required=True)
    p.add_argument("--cache", required=True)
    p.add_argument("--out", required=True)
    p.add_argument("--composed-sha256")

    p = sub.add_parser("anchor-base")
    p.add_argument("--python-root", required=True)
    p = sub.add_parser("allocated")
    p.add_argument("path")
    p = sub.add_parser("fs-used")
    p.add_argument("path")
    p = sub.add_parser("spec-hash")
    p.add_argument("--image", required=True)
    p.add_argument("argv", nargs=argparse.REMAINDER)
    p = sub.add_parser("logbound")
    p.add_argument("--stdout", required=True)
    p.add_argument("--stderr", required=True)
    p = sub.add_parser("seq")
    p.add_argument("--state", required=True)
    p = sub.add_parser("index")
    for name in ("--state", "--sequence", "--phase", "--role", "--included", "--image"):
        p.add_argument(name, required=True)
    p.add_argument("--profile", default="sizing")
    p.add_argument("argv", nargs=argparse.REMAINDER)
    p = sub.add_parser("event")
    for name in ("--state", "--id", "--kind"):
        p.add_argument(name, required=True)
    p = sub.add_parser("sidecar")
    for name in ("--state", "--sequence", "--rc", "--container", "--size-rw", "--log-config", "--stdout-log",
                 "--stderr-log", "--peak-file", "--memory-limit", "--memory-swap-limit"):
        p.add_argument(name, required=True)
    p.add_argument("--failure", action="append", default=[])
    p = sub.add_parser("twins")
    for name in ("--state", "--docker", "--run-id", "--fs-path"):
        p.add_argument(name, required=True)
    p = sub.add_parser("cleanup-cids")
    for name in ("--state", "--docker"):
        p.add_argument(name, required=True)
    p = sub.add_parser("sample")
    for name in ("--state", "--phase", "--locations", "--fs-path"):
        p.add_argument(name, required=True)
    p.add_argument("--interval", type=float, default=0.2)
    p = sub.add_parser("baseline")
    for name in ("--state", "--phase", "--locations", "--fs-path", "--docker-root", "--inventory-roots"):
        p.add_argument(name, required=True)
    p = sub.add_parser("phase-end")
    for name in ("--state", "--phase", "--run-dir", "--archive", "--inventory-roots", "--allowed", "--source-roots"):
        p.add_argument(name, required=True)
    p = sub.add_parser("record-diff")
    for name in ("--before", "--after", "--published", "--failed-root"):
        p.add_argument(name, required=True)
    p = sub.add_parser("report")
    for name in ("--state", "--json-out", "--text-out"):
        p.add_argument(name, required=True)
    p = sub.add_parser("failure-summary")
    for name in ("--state", "--stage", "--rc", "--out"):
        p.add_argument(name, required=True)
    p.add_argument("--leftover-cids")
    # ── ⑦d ──
    p = sub.add_parser("acceptance-anchors")
    p.add_argument("--python-root", required=True)
    p = sub.add_parser("replay-argv")
    for name in ("--clone", "--run-dir", "--anchors"):
        p.add_argument(name, required=True)
    p = sub.add_parser("clean-env")
    p.add_argument("--clone", required=True)
    p = sub.add_parser("promote-measure")
    for name in ("--clone", "--work", "--real", "--identity", "--bundle-id", "--semantic", "--repo-head", "--rc"):
        p.add_argument(name, required=True)
    p = sub.add_parser("host-run")
    for name in ("--state", "--step"):
        p.add_argument(name, required=True)
    p.add_argument("argv", nargs=argparse.REMAINDER)
    p = sub.add_parser("acceptance-report")
    for name in ("--state", "--clone", "--json-out", "--text-out"):
        p.add_argument(name, required=True)
    p = sub.add_parser("observe")
    for name in ("--work-dir", "--out"):
        p.add_argument(name, required=True)

    args = parser.parse_args(argv)
    try:
        return _dispatch(args)
    except (SizingError, OSError, ValueError, KeyError) as exc:
        print(f"ERROR: {type(exc).__name__}: {exc}", file=sys.stderr)
        return 1


def _dispatch(args) -> int:
    cmd = args.cmd
    if cmd == "fixture":
        py, cache, out = Path(args.python_root), Path(args.cache), Path(args.out)
        if args.phase == "witness":
            result = fixture_witness(cache, out)
        elif args.phase == "success":
            result = fixture_success(py, cache, out, args.composed_sha256)
        else:
            result = fixture_failure(py, cache, out, args.composed_sha256)
        print(json.dumps({"phase": args.phase, **result}, sort_keys=True))
        return 0
    if cmd == "anchor-base":
        # host：經 `_i074_bootstrap` 載入 dependency-light 的模組，用**已驗證的** Stage 1 信任錨取 base。
        # ⚠️ ⑦d：`_i074_bootstrap` 取自 --python-root（工作複本），⛔ 不從 helper 自己的目錄（快照裡沒有它）。
        modules, load = clone_bootstrap(args.python_root)
        root = Path(args.python_root)
        print(load(root, modules)["stage2_evidence"].load_stage1_anchor(root).after_base_commit)
        return 0
    if cmd == "acceptance-anchors":
        print(json.dumps(acceptance_anchors(args.python_root), sort_keys=True))
        return 0
    if cmd == "replay-argv":
        argv = acceptance_replay_argv(args.clone, args.run_dir, json.loads(Path(args.anchors).read_text()))
        sys.stdout.write("".join(f"{tok}\0" for tok in argv))
        return 0
    if cmd == "clean-env":
        env = clone_clean_env(args.clone, os.environ)
        sys.stdout.write("".join(f"{k}={v}\0" for k, v in sorted(env.items())))
        return 0
    if cmd == "promote-measure":
        states = promotion_states(args.clone, identity=args.identity, bundle_id=args.bundle_id, semantic=args.semantic,
                                  repo_head=args.repo_head, rc=int(args.rc))
        return promote_measure(args.clone, work=args.work, real=args.real, states=states)
    if cmd == "host-run":
        return host_run(Path(args.state), args.step, args.argv[1:] if args.argv[:1] == ["--"] else args.argv)
    if cmd == "acceptance-report":
        pf = _load_module("i074_stage2_preflight", Path(args.clone) / "python" / "scripts" / "i074_stage2_preflight.py")
        sup = _load_module("i074_stage2_supervisor", Path(args.clone) / "scripts" / "lib" / "i074-stage2-supervisor.py")
        report = build_acceptance_report(Path(args.state), p_b_budget=pf.P_B_BUDGET, drop_names=sup.ENV_DROP_NAMES,
                                         drop_prefixes=tuple(sup.ENV_DROP_PREFIXES))
        Path(args.json_out).write_bytes(canonical_dumps(report))
        Path(args.text_out).write_text(acceptance_report_text(report), encoding="utf-8")
        print(acceptance_report_text(report), end="", file=sys.stderr)
        return 0 if report["status"] == "ok" else 2
    if cmd == "observe":
        return _observe_main(args.work_dir, args.out)
    if cmd == "allocated":
        print(json.dumps(allocated_tree(args.path), sort_keys=True))
        return 0
    if cmd == "fs-used":
        print(fs_used(args.path))
        return 0
    if cmd == "spec-hash":
        print(spec_sha256(args.argv[1:] if args.argv[:1] == ["--"] else args.argv, args.image))
        return 0
    if cmd == "logbound":
        print(log_bound(Path(args.stdout).read_bytes(), Path(args.stderr).read_bytes()))
        return 0
    state = Path(args.state) if hasattr(args, "state") else None
    if cmd == "seq":
        print(_locked_increment(state / "seq" / "counter"))
        return 0
    if cmd == "index":
        argv = args.argv[1:] if args.argv[:1] == ["--"] else args.argv
        if args.included not in ("true", "false"):
            raise SizingError(f"--included 只接受 true／false：{args.included!r}")
        sequence = int(args.sequence)
        write_index(state, sequence=sequence, cid=container_id(sequence), phase=args.phase, role=args.role,
                    included=args.included == "true", image=args.image, argv=argv, profile=args.profile)
        return 0
    if cmd == "event":
        record_event(state, args.id, args.kind)
        return 0
    if cmd == "sidecar":
        sequence = int(args.sequence)
        write_sidecar(state, cid=container_id(sequence), sequence=sequence, rc=int(args.rc),
                      container=args.container, size_rw=args.size_rw, log_config_json=args.log_config,
                      stdout_log=Path(args.stdout_log), stderr_log=Path(args.stderr_log),
                      peak_file=Path(args.peak_file), failures=args.failure, memory_limit=args.memory_limit,
                      memory_swap_limit=args.memory_swap_limit)
        return 0
    if cmd == "twins":
        run_twins(state, docker=args.docker, run_id=args.run_id, fs_path=args.fs_path)
        return 0
    if cmd == "cleanup-cids":
        leftovers = cleanup_cids(state, docker=args.docker)
        for cid in leftovers:
            print(f"ERROR: 移除不了容器 {cid}", file=sys.stderr)
        return 1 if leftovers else 0
    if cmd == "sample":
        run_sampler(state, args.phase, locations=json.loads(args.locations), fs_path=args.fs_path,
                    interval=args.interval)
        return 0
    if cmd == "baseline":
        locations = json.loads(args.locations)
        pdir = state / "phases" / args.phase
        pdir.mkdir(parents=True, exist_ok=False)
        devices = {loc: os.stat(path).st_dev for loc, path in locations.items()}
        devices["L0"] = os.stat(args.fs_path).st_dev
        devices["L4"] = os.stat(args.docker_root).st_dev
        baseline = {"locations": measure_locations(locations), "L0_used": fs_used(args.fs_path),
                    "devices": devices, "paths": locations}
        write_exclusive(pdir / "baseline.json", canonical_dumps(baseline))
        write_exclusive(pdir / "inventory-before.json",
                        canonical_dumps(inventory(json.loads(args.inventory_roots))))
        return 0
    if cmd == "phase-end":
        pdir = state / "phases" / args.phase
        before = read_json(pdir / "inventory-before.json")
        after = inventory(json.loads(args.inventory_roots))
        violations = inventory_diff(before, after, allowed=json.loads(args.allowed),
                                    source_roots=json.loads(args.source_roots))
        end = {"run_dir": allocated_tree(args.run_dir), "archive": allocated_tree(args.archive),
               "inventory_violations": violations}
        write_exclusive(pdir / "end.json", canonical_dumps(end))
        if violations:
            for v in violations[:20]:
                print(f"ERROR: 自我檢查：{v}", file=sys.stderr)
            return 1
        return 0
    if cmd == "record-diff":
        before = [l for l in Path(args.before).read_text().splitlines() if l]
        after = [l for l in Path(args.after).read_text().splitlines() if l]
        print(record_diff(before, after, published=args.published, failed_root=args.failed_root))
        return 0
    if cmd == "report":
        report = build_report(state)
        Path(args.json_out).write_bytes(canonical_dumps(report))
        Path(args.text_out).write_text(report_text(report), encoding="utf-8")
        print(report_text(report), end="", file=sys.stderr)
        return 0
    if cmd == "failure-summary":
        phases = {}
        for pdir in sorted((state / "phases").glob("*")) if (state / "phases").is_dir() else []:
            phases[pdir.name] = "aborted" if (pdir / "aborted").exists() else \
                "complete" if (pdir / "end.json").exists() else "incomplete"
        leftovers = sorted({line.strip() for line in Path(args.leftover_cids).read_text().splitlines() if line.strip()}) \
            if args.leftover_cids and Path(args.leftover_cids).is_file() else []
        summary = {"schema": "i074_stage2_sizing_failure_v1", "failed_stage": args.stage, "rc": int(args.rc),
                   "phases": phases, "P_B": None, "leftover_containers": leftovers,
                   "note": "⛔ 本檔不宣稱 P_B；原始量測見 raw-failed/"}
        Path(args.out).write_bytes(canonical_dumps(summary))
        return 0
    raise SizingError(f"未知的子指令：{cmd}")


def _observe_main(work_dir: str, out: str) -> int:
    import shutil  # noqa: PLC0415

    observe_guards(work_dir, out, os.environ, shutil.which("docker"), Path(__file__).resolve().parents[2])
    run_id = f"{time.strftime('%Y%m%dT%H%M%SZ', time.gmtime())}-{os.getpid()}"
    state = Path(f"/dev/shm/i074-observe-{run_id}")
    state.mkdir(mode=0o700)
    stopping = {"flag": False}

    def on_signal(_signum, _frame) -> None:
        stopping["flag"] = True

    signal.signal(signal.SIGINT, on_signal)
    signal.signal(signal.SIGTERM, on_signal)
    print(f"==> observer：--work-dir {work_dir}；狀態 {state}；⑩ 結束之後以 Ctrl-C 停止", file=sys.stderr)
    report = run_observer(Observer(work_dir), state, lambda: stopping["flag"])
    os.mkdir(out)
    Path(out, "observation.json").write_bytes(canonical_dumps(report))
    Path(out, "observation.txt").write_text(observation_text(report), encoding="utf-8")
    print(observation_text(report), end="", file=sys.stderr)
    shutil.rmtree(state)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
