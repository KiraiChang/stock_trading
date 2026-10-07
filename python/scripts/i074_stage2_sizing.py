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
* 有效性條件與 ⑨-1 的 fail-fast（issue.md I-074「Stage 2 量測的有效性條件與 ⑨-1 fail-fast 計畫」）：`acceptance-precheck`
  （live）；offline 的 `precheck-verdict` 與 `raw-manifest`——對外的命令只執行驗錨與取出的 frontend，內部的
  `precheck-recompute`／`raw-manifest-recompute` 只由從錨點 commit 取出的受信任版本執行。`report`、`acceptance-report`、
  `acceptance-precheck` 的權威常數（`P_B_BUDGET`、`ENV_DROP_*`）取自 S 的快照（`live_constants()`）。

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
#   ⚠️ issue.md「Stage 2 量測的有效性條件與 ⑨-1 fail-fast 計畫」「二」的「權威常數的來源」：`P_B_BUDGET`（preflight）與
#   `ENV_DROP_*`（supervisor）的唯一定義也進快照——live 一律從快照載入（`_load_frozen()`），⛔ 不讀活路徑、⛔ 不複製常數。
FROZEN_PREFLIGHT = "python/scripts/i074_stage2_preflight.py"
FROZEN_SUPERVISOR = "scripts/lib/i074-stage2-supervisor.py"
HELPER_REL = "python/scripts/i074_stage2_sizing.py"
SNAPSHOT_FILES = {
    "sizing": ("scripts/i074-stage2-sizing.sh", "scripts/lib/i074-stage2-measure.sh",
               "scripts/lib/i074-sizing-docker-shim.sh", "scripts/lib/mem-guard.sh", HELPER_REL, FROZEN_PREFLIGHT),
    "acceptance": ("scripts/i074-stage2-acceptance.sh", "scripts/lib/i074-stage2-measure.sh",
                   "scripts/lib/i074-sizing-docker-shim.sh", HELPER_REL,
                   "python/scripts/i074_stage2_replay_stub.py", "python/scripts/i074_stage2_rss_wrapper.py",
                   FROZEN_PREFLIGHT, FROZEN_SUPERVISOR),
}
# ⚠️ 有效性條件 ①：未解釋的**淨**成長的容差（1 MiB，以常數寫死、⛔ 不開參數；計畫「三」#1）。
FS_UNEXPLAINED_TOLERANCE = 1 << 20
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


# ⑦d 增補（issue.md「Stage 2 步驟 ⑦d 增補計畫：容器記憶體改以 RSS 判定」「二」②）：容器內 wrapper 的 `rss.json`。
#   ⚠️ 與 `i074_stage2_rss_wrapper.py` 的常數相同（測試釘住；sizing 的快照⛔ 沒有 wrapper，所以⛔ 不 import 它）。
RSS_RECORD_SCHEMA = "i074_stage2_rss_v1"
RSS_SOURCE = "v1:total_rss"
RSS_INTERVAL_MS = 50
RSS_RECORD_KEYS = ("schema", "rss_source", "rss_interval_ms", "rss_samples", "rss_peak_sampled_bytes", "self_max_rss_bytes",
                   "children_max_rss_bytes", "max_single_rss_bytes", "reaper", "all_descendants_reaped",
                   "auto_reap_detected", "errors")


def _is_int(value: Any) -> bool:
    return type(value) is int                 # ⚠️ ⛔ 不接受 bool（bool 是 int 的子類別）


def rss_record_problems(rec: Any, *, require_pid1: bool) -> list[str]:
    """`rss.json` 的封閉 schema（「二」②）：鍵集合恰好、型別、固定值、交叉條件與有效條件。回傳問題清單（空 ＝ 有效）。"""
    if not isinstance(rec, dict):
        return ["不是 JSON object"]
    if set(rec) != set(RSS_RECORD_KEYS):
        return [f"鍵集合不符：缺 {sorted(set(RSS_RECORD_KEYS) - set(rec))}、多 {sorted(set(rec) - set(RSS_RECORD_KEYS))}"]
    problems = []
    for key, want in (("schema", RSS_RECORD_SCHEMA), ("rss_source", RSS_SOURCE)):
        if rec[key] != want:
            problems.append(f"{key}={rec[key]!r}（必須是 {want!r}）")
    if not _is_int(rec["rss_interval_ms"]) or rec["rss_interval_ms"] != RSS_INTERVAL_MS:
        problems.append(f"rss_interval_ms={rec['rss_interval_ms']!r}（必須是整數 {RSS_INTERVAL_MS}）")
    for key in ("rss_samples", "rss_peak_sampled_bytes", "self_max_rss_bytes", "children_max_rss_bytes",
                "max_single_rss_bytes"):
        if not _is_int(rec[key]) or rec[key] <= 0:
            problems.append(f"{key}={rec[key]!r}（必須是 > 0 的整數）")
    if not problems and rec["max_single_rss_bytes"] != max(rec["self_max_rss_bytes"], rec["children_max_rss_bytes"]):
        problems.append("max_single_rss_bytes ≠ max(self_max_rss_bytes, children_max_rss_bytes)")
    if rec["reaper"] not in ("pid1", "subreaper"):
        problems.append(f"reaper={rec['reaper']!r}（只能是 pid1／subreaper）")
    elif require_pid1 and rec["reaper"] != "pid1":
        problems.append(f"reaper={rec['reaper']!r}（容器內必須是 pid1）")
    for key, want in (("all_descendants_reaped", True), ("auto_reap_detected", False)):
        if type(rec[key]) is not bool or rec[key] is not want:
            problems.append(f"{key}={rec[key]!r}（有效的量測必須是 {want}）")
    errors = rec["errors"]
    if not isinstance(errors, list) or not all(isinstance(e, str) for e in errors):
        problems.append("errors 必須是字串陣列")
    elif errors:
        problems.append(f"wrapper 記錄了錯誤：{errors}")
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
    rss: dict[str, Any] = {}
    if index.get("profile") == "acceptance":
        # ⑦d 增補：acceptance 的 wrapper 在同一個 peak 目錄寫 rss.json；依封閉 schema 驗（容器內必須是 pid1）。
        try:
            record = json.loads((peak_file.parent / "rss.json").read_text(encoding="utf-8"))
        except (OSError, ValueError) as exc:
            problems.append(f"rss.json 讀不到：{type(exc).__name__}: {exc}")
        else:
            rss_problems = rss_record_problems(record, require_pid1=True)
            problems += [f"rss.json：{p}" for p in rss_problems]
            if not rss_problems:
                rss = {k: record[k] for k in ("rss_peak_sampled_bytes", "rss_samples", "self_max_rss_bytes",
                                              "children_max_rss_bytes", "max_single_rss_bytes", "reaper")}
    entry = {
        "id": cid, "sequence": sequence, "container_spec_sha256": index["container_spec_sha256"],
        "rc": rc, "container": container, "size_rw": size_rw_value, "log_config": log_config,
        "log_bound": bound, "peak_bytes": peak, **limits, **rss,
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
    ⚠️ 有效性條件計畫「二」的「metadata twin 分兩次」：**只處理還沒有 twin 結果的 invocation**——acceptance 的 full 模式在
    量測趟之前先建一次、量測趟之後再補它那一個；已有的⛔ 不重建（每一份仍以 `write_exclusive` 寫）。
    """
    (state / "cid").mkdir(parents=True, exist_ok=True)
    for index_path in sorted((state / "index").glob("*.json")):
        index = read_json(index_path)
        if (state / "twins" / f"{index['id']}.json").exists():
            continue
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


def load_invocations(state: Path, profile: str = "sizing", replay_compute: str | None = None, *,
                     expected: list[tuple[str, str]] | None = None) -> list[dict[str, Any]]:
    """索引、sidecar、twin 三者的完整性（計畫書「二」的 invocation 索引；⑦d：依 profile 的封閉列舉）。

    `expected` 只給 acceptance 的 collector 傳「量測趟之前的精確前綴」（`acceptance_expected()` 斷言過）；未傳 ＝ 正式集合。
    """
    spec = profile_spec(profile)
    indexes = sorted((state / "index").glob("*.json"))
    entries = [read_json(p) for p in indexes]
    sequences = [e["sequence"] for e in entries]
    if sequences != list(range(1, len(entries) + 1)):
        raise SizingError(f"invocation 的 sequence 不連續或重複：{sequences}")
    if expected is None:
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


# ── 有效性條件（issue.md I-074「Stage 2 量測的有效性條件與 ⑨-1 fail-fast 計畫」「二」） ──────────────────────

VALIDITY_KINDS = ("fs_unexplained", "ambiguous_disk_exceed")


def disk_numbers(info: Mapping[str, Any]) -> dict[str, int]:
    """報告的一條磁碟路徑 → 判定用的數字。`P_basis = max(dirs_peak, accounted)`：兩者都⛔ 不受 host 其他程序影響；
    `fs_unexplained = fs_peak − P_basis`：`fs_peak` 是整個檔案系統已用量的**淨**變化（別人的寫入墊高它、刪除壓低它）。"""
    peaks = info["peaks"]
    basis = max(peaks["dirs_peak"], info["accounted"])
    return {"P_path": info["P_path"], "dirs_peak": peaks["dirs_peak"], "fs_peak": peaks["fs_peak"],
            "accounted": info["accounted"], "P_basis": basis, "fs_unexplained": peaks["fs_peak"] - basis}


def disk_validity_problems(disk: Mapping[str, Mapping[str, int]], p_b_budget: int) -> list[dict[str, Any]]:
    """有效性條件：① `fs_unexplained` ≤ 容差；② 模糊區——`P_path` ＞ 預算、`P_basis` ≤ 預算且 `fs_unexplained` ≤ 容差（超標完全由
    **容許範圍內**的未解釋成長造成；**容差⛔ 不得決定結果**；超過容差時只是 ①，⛔ 不重複記成模糊區——實作第一輪 review）。依路徑的順序、每條先 ① 後 ②。⚠️ 只偵測**淨**的未解釋正成長：外部的刪除可能
    抵銷外部的寫入或流程漏算的寫入，⛔ 不能證明量測期間沒有外部 I/O。模糊區只是有效性問題、⛔ 不是違反。"""
    out: list[dict[str, Any]] = []
    for phase, d in disk.items():
        if d["fs_unexplained"] > FS_UNEXPLAINED_TOLERANCE:
            out.append({"kind": "fs_unexplained", "subject": phase, "value": d["fs_unexplained"],
                        "limit": FS_UNEXPLAINED_TOLERANCE})
        if d["P_path"] > p_b_budget and d["P_basis"] <= p_b_budget and d["fs_unexplained"] <= FS_UNEXPLAINED_TOLERANCE:
            out.append({"kind": "ambiguous_disk_exceed", "subject": phase, "value": d["P_path"], "limit": p_b_budget})
    return out


def disk_basis_violations(disk: Mapping[str, Mapping[str, int]], p_b_budget: int) -> list[dict[str, Any]]:
    """**確定的**磁碟超標：`P_basis` ＞ 預算（與磁碟雜訊無關）；`value` 記 `P_basis`——裁決值就是證據值。
    ⚠️ 只有 `precheck.json` 用它（`disk_p_basis`）；報告 v2 照舊以 `P_path` 判（v2 只在沒有有效性問題時寫出，兩者等價）。"""
    return [{"kind": "disk_p_basis", "subject": phase, "value": d["P_basis"], "limit": p_b_budget}
            for phase, d in disk.items() if d["P_basis"] > p_b_budget]


def validity_text(problems: Iterable[Mapping[str, Any]]) -> str:
    names = {"fs_unexplained": "fs_unexplained（未解釋的淨成長）", "ambiguous_disk_exceed": "模糊區（P_path 超標、P_basis 沒有）"}
    return "；".join(f"{p['subject']}：{names[p['kind']]} {p['value']} bytes ＞ {p['limit']}" for p in problems)


def sizing_validity_problems(report: Mapping[str, Any], p_b_budget: int) -> list[dict[str, Any]]:
    return disk_validity_problems({phase: disk_numbers(report["paths"][phase]) for phase in DISK_PHASES}, p_b_budget)


def build_report(state: Path, *, p_b_budget: int) -> dict[str, Any]:
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
    # ⚠️ 有效性條件（計畫「二」）：確定的違反（某條路徑的 P_basis ＞ 預算）優先——照常寫報告（P_B ＞ 預算，freeze record
    #   照舊拒寫 → 回退順序；v1 的語意⛔ 不變）；沒有確定的違反、但有有效性問題 → 量測無效（⛔ 不是判定，可以重跑）。
    disk = {phase: disk_numbers(paths[phase]) for phase in DISK_PHASES}
    validity = disk_validity_problems(disk, p_b_budget)
    if validity and not disk_basis_violations(disk, p_b_budget):
        raise SizingError(f"量測無效（有效性條件；⛔ 不是判定，可以重跑）：{validity_text(validity)}")
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


def report_text(report: Mapping[str, Any], *, validity_problems: Iterable[Mapping[str, Any]] = ()) -> str:
    mib = lambda b: f"{b / 1048576:.1f} MiB" if b is not None else "—"  # noqa: E731
    lines = [f"status: {report['status']}（mode={report['mode']}）", f"P_B = {report['P_B']} bytes（{mib(report['P_B'])}）"]
    validity_problems = list(validity_problems)
    if validity_problems:
        lines.append(f"⚠️ 有效性問題（同一次有確定的超標，照常寫報告）：{validity_text(validity_problems)}")
    lines.append("")
    for phase, info in report["paths"].items():
        pk, d = info["peaks"], disk_numbers(info)
        lines.append(f"[{phase}] P_path={mib(info['P_path'])}  dirs_peak={mib(pk['dirs_peak'])}  "
                     f"fs_peak={mib(pk['fs_peak'])}  accounted={mib(info['accounted'])}  samples={pk['samples']}")
        lines.append(f"    P_basis={d['P_basis']}  fs_unexplained={d['fs_unexplained']} bytes（容差 {FS_UNEXPLAINED_TOLERANCE}）")
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


def _load_frozen(harness: Path, rel: str, mapping: Mapping[str, str], name: str):
    """**live 專用**：從 S 的快照載入權威常數的模組（S 由 bootstrap 從受信任的 repo 建立並驗證過）——先驗該檔的 SHA-256 ＝
    `MANIFEST` 的那一行才載入。⚠️ offline 的讀取端⛔ 不用它：原始量測目錄裡的程式⛔ 不執行、⛔ 不 import（見 `precheck_verdict()`）。"""
    if rel not in mapping:
        raise SizingError(f"快照的 MANIFEST 沒有 {rel}")
    path = harness / rel
    if hashlib.sha256(path.read_bytes()).hexdigest() != mapping[rel]:
        raise SizingError(f"快照的 {rel} 與 MANIFEST 不符")
    return _load_module(name, path)


def live_constants(state: Path, clone: str | Path, profile: str) -> dict[str, Any]:
    """live 的 `report`（sizing）、`acceptance-precheck`、`acceptance-report` 用的權威常數：`P_B_BUDGET`（preflight）與 acceptance 的
    `ENV_DROP_NAMES`／`ENV_DROP_PREFIXES`（supervisor），**一律取自 S 的快照**；另外斷言快照裡的這幾個檔案與工作複本**逐位元相同**
    （步驟實際執行的是工作複本的 `clean_env()`，freeze record 也從工作複本 import `P_B_BUDGET`）——不同即 fail-closed。"""
    mapping, _sha = load_harness_manifest(state, profile)
    rels = (FROZEN_PREFLIGHT,) if profile == "sizing" else (FROZEN_PREFLIGHT, FROZEN_SUPERVISOR)
    for rel in rels:
        if (state / "harness" / rel).read_bytes() != (Path(clone) / rel).read_bytes():
            raise SizingError(f"快照的 {rel} 與工作複本的不同（⛔ 不得混用兩個版本的權威常數）")
    pf = _load_frozen(state / "harness", FROZEN_PREFLIGHT, mapping, "i074_stage2_preflight_frozen")
    out: dict[str, Any] = {"p_b_budget": pf.P_B_BUDGET}
    if profile == "acceptance":
        sup = _load_frozen(state / "harness", FROZEN_SUPERVISOR, mapping, "i074_stage2_supervisor_frozen")
        out.update(drop_names=tuple(sup.ENV_DROP_NAMES), drop_prefixes=tuple(sup.ENV_DROP_PREFIXES))
    return out


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

# ⚠️ ⑦d 增補（issue.md「Stage 2 步驟 ⑦d 增補計畫：容器記憶體改以 RSS 判定」「二」④）：程序的身分與收養。
#   `/proc` 的位置是模組常數（測試可以換掉）；PID 一律以 starttime（`/proc/<pid>/stat` 第 22 欄）釘住。
PROC_ROOT = Path("/proc")
# `host-run` 紀錄的封閉 schema（十七個鍵；驗證在 host_record_problems()）
HOST_RECORD_SCHEMA = "i074_stage2_host_run_v1"
HOST_INTERVAL_MS = 100
HOST_RECORD_KEYS = ("schema", "step", "rc", "self_max_rss_bytes", "children_max_rss_bytes", "max_single_rss_bytes",
                    "group_rss_peak_sampled_bytes", "group_samples", "group_interval_ms", "reaper",
                    "all_descendants_reaped", "auto_reap_detected", "cleanup_complete", "leftover_pids", "errors",
                    "env_keys", "cmd")
PR_SET_CHILD_SUBREAPER = 36
SIGCHLD_MASK = 1 << (int(signal.SIGCHLD) - 1)      # /proc/<pid>/status 的 SigIgn：第 (signum − 1) 個位元


class ParentChanged(Exception):
    """`reap-adopted` 的 parent（harness）已經不是原本的那一個——立刻停止送訊號。"""


def read_proc_stat(pid: int) -> tuple[str, int] | None:
    """(state, starttime)；程序不在 → None。⚠️ ENOENT 以外的讀取錯誤照樣拋出（呼叫端 fail-closed）。"""
    try:
        text = (PROC_ROOT / str(pid) / "stat").read_text()
    except (FileNotFoundError, ProcessLookupError):
        return None
    fields = text.rsplit(")", 1)[1].split()
    return fields[0], int(fields[19])


def _gone(stat: tuple[str, int] | None, starttime: int) -> bool:
    """已消失 ＝ 不在、身分不符（PID 已被重用）、或已死只等收屍（Z／X）。"""
    return stat is None or stat[1] != starttime or stat[0] in ("Z", "X")


def proc_parents() -> dict[int, int]:
    """pid → ppid（讀不到、程序已消失的略過）。⚠️ `/proc` 本身讀不到 → 拋出。"""
    parents: dict[int, int] = {}
    for entry in PROC_ROOT.iterdir():
        if not entry.name.isdigit():
            continue
        try:
            parents[int(entry.name)] = int((entry / "stat").read_text().rsplit(")", 1)[1].split()[1])
        except (OSError, IndexError, ValueError):
            continue
    return parents


def proc_descendants(root_pid: int, parents: Mapping[int, int]) -> set[int]:
    children: dict[int, list[int]] = {}
    for pid, ppid in parents.items():
        children.setdefault(ppid, []).append(pid)
    found, stack = set(), [root_pid]
    while stack:
        for child in children.get(stack.pop(), []):
            if child not in found:
                found.add(child)
                stack.append(child)
    return found


def _proc_status_value(pid: int, key: str) -> str | None:
    try:
        for line in (PROC_ROOT / str(pid) / "status").read_text().splitlines():
            if line.startswith(key):
                return line.split(":", 1)[1].strip()
    except OSError:
        return None
    return None


def _vmrss(pid: int) -> int:
    value = _proc_status_value(pid, "VmRSS:")
    return int(value.split()[0]) * 1024 if value else 0


def _sigchld_ignored(pid: int) -> bool:
    value = _proc_status_value(pid, "SigIgn:")
    return bool(value and int(value, 16) & SIGCHLD_MASK)


def become_subreaper() -> str | None:
    """設成 subreaper；失敗 → 回傳錯誤（呼叫端 fail-closed）。"""
    import ctypes
    libc = ctypes.CDLL(None, use_errno=True)
    if libc.prctl(PR_SET_CHILD_SUBREAPER, 1, 0, 0, 0) != 0:
        return f"prctl(PR_SET_CHILD_SUBREAPER) 失敗：errno {ctypes.get_errno()}"
    return None


def _direct_children(me: int) -> dict[int, int]:
    """ppid ＝ me、仍存活（⛔ 不是 zombie）的程序 → starttime。"""
    alive = {}
    for pid, ppid in proc_parents().items():
        if ppid == me:
            stat = read_proc_stat(pid)
            if stat is not None and stat[0] not in ("Z", "X"):
                alive[pid] = stat[1]
    return alive


def _reap_nohang() -> bool:
    """收掉所有已結束的子程序；回傳 True ＝ 已經沒有任何子程序（ECHILD）。"""
    while True:
        try:
            pid, _status = os.waitpid(-1, os.WNOHANG)
        except ChildProcessError:
            return True
        if pid == 0:
            return False


MANAGED_SIGNALS = (signal.SIGTERM, signal.SIGINT, signal.SIGHUP)


def _drain_managed_signals() -> list[int]:
    """取走所有 pending 的 TERM／INT／HUP（它們已被擋住，⛔ 不會再交給 handler）。"""
    got = []
    while True:
        info = signal.sigtimedwait(MANAGED_SIGNALS, 0)
        if info is None:
            return got
        got.append(info.si_signo)


def _signal_linearization_point(received: list[int]) -> None:
    """增補實作第二輪 review：訊號的 linearization point（L）。先擋住 TERM／INT／HUP——CPython 的 `pthread_sigmask` 在回傳
    之前會先跑完已送達的 handler（`PyErr_CheckSignals`），所以 L 之前送達的都已在 `received`——再取走已經 pending 的。
    L 之後送達的留在 pending，直到紀錄寫完才處理（⛔ 不會被靜默吞掉）。"""
    signal.pthread_sigmask(signal.SIG_BLOCK, MANAGED_SIGNALS)
    received.extend(_drain_managed_signals())


def host_run(state: Path, step: str, cmd: list[str], *, interval_ms: int = HOST_INTERVAL_MS, grace_s: float = 10.0,
             term_wait_s: float = 5.0, cleanup_total_s: float = 15.0) -> int:
    """執行一步並記 host 端程序樹的記憶體（⑦d「二之三」；⑦d 增補「二」④）：

    * **精確閘**：`host-run` 是 subreaper；leader 結束後收到 `ECHILD`（寬限 `grace_s`），`max_single_rss_bytes` ＝
      `max(RUSAGE_SELF, RUSAGE_CHILDREN)`——涵蓋 `host-run` 自己、leader 與經正常 wait 鏈保存 resource usage 的子孫；
    * **單向警報**：每 `interval_ms` 以 ppid 鏈追到的全部子孫（含 `setsid` 的）加上自己的 `VmRSS` 總和；同一次掃描驗 `SigIgn`；
    * **清理**：逾時、或收到 TERM／INT／HUP → 先記量測失敗，再**只對直接子程序**送 TERM → KILL（只有本程序能收它們，
      收之前 PID ⛔ 不會被重用），一輪一輪收到 `ECHILD`（上限 `cleanup_total_s`）；清不乾淨 → 結束碼 70。
    ⚠️ RSS ⛔ 不含 page cache；容器裡的程序屬於 docker daemon，⛔ 不在這棵樹裡。
    """
    if not cmd:
        raise SizingError("host-run 沒有指令")
    me = os.getpid()
    errors: list[str] = []
    error = become_subreaper()
    if error:
        errors.append(error)
    received: list[int] = []
    previous = {sig: signal.signal(sig, lambda signum, _frame: received.append(signum)) for sig in MANAGED_SIGNALS}
    late: list[int] = []
    peak = samples = 0
    auto_reap = False

    def sample() -> None:
        nonlocal peak, samples, auto_reap
        tree = proc_descendants(me, proc_parents())
        peak = max(peak, _vmrss(me) + sum(_vmrss(pid) for pid in tree))
        samples += 1
        auto_reap = auto_reap or any(_sigchld_ignored(pid) for pid in tree)

    interval = interval_ms / 1000
    rc = 0
    reaped = False
    leftover: list[int] = []
    try:
        leader = subprocess.Popen(cmd)
        while leader.poll() is None and not received:
            sample()
            time.sleep(interval)
        if not received:
            rc = leader.returncode if leader.returncode >= 0 else 128 - leader.returncode
            deadline = time.monotonic() + grace_s
            while not received:
                sample()
                if _reap_nohang():
                    reaped = True
                    break
                if time.monotonic() >= deadline:
                    errors.append(f"leader 結束後 {grace_s:g} 秒內仍有存活的子孫（high-water 讀不到）")
                    break
                time.sleep(interval)
        if not reaped:
            leftover = _cleanup_direct_children(me, term_wait_s=term_wait_s, total_s=cleanup_total_s)
        sample()
        # ── L：之前送達的訊號都反映到錯誤、紀錄的 rc 與回傳碼；之後送達的見下方「紀錄寫出之後」 ──
        _signal_linearization_point(received)
        if received:
            errors.append(f"收到訊號 {signal.Signals(received[0]).name}（量測中斷）")
        self_rss = resource.getrusage(resource.RUSAGE_SELF).ru_maxrss * 1024
        children_rss = resource.getrusage(resource.RUSAGE_CHILDREN).ru_maxrss * 1024
        # ⚠️ 實作第一輪 review：先決定**唯一的**結束碼，紀錄與回傳用同一個值（清不乾淨 70 ＞ 訊號 128 ＋ N ＞ leader 的結束碼）。
        final_rc = 70 if leftover else 128 + received[0] if received else rc
        # ⚠️ 環境只記**名稱**（⛔ 不記值）：報告以它驗「clean_env() ＋ 固定 PATH ＋ 量測變數」與兩份 patch 只給 replay（「三」#12）。
        write_exclusive(state / "host" / f"{step}.json", canonical_dumps({
            "schema": HOST_RECORD_SCHEMA, "step": step, "rc": final_rc, "self_max_rss_bytes": self_rss,
            "children_max_rss_bytes": children_rss, "max_single_rss_bytes": max(self_rss, children_rss),
            "group_rss_peak_sampled_bytes": peak, "group_samples": samples, "group_interval_ms": interval_ms,
            "reaper": "subreaper", "all_descendants_reaped": reaped, "auto_reap_detected": auto_reap,
            "cleanup_complete": not leftover, "leftover_pids": leftover, "errors": errors,
            "env_keys": sorted(os.environ), "cmd": list(cmd)}))
        # L 之後（序列化與寫入期間）送達的：紀錄是 exclusive create、已經定案，⛔ 不能改。回傳碼套**同一個優先序**
        #   （實作第三輪 review）：清不乾淨 → 維持 70（紀錄與實際一致）；否則改成 128 ＋ N，紀錄的 rc 與實際結束碼
        #   因此不符，check-step 與報告會擋下（fail-closed，⛔ 不宣稱成功）。兩種情況 stderr 都說明晚到的訊號。
        late = _drain_managed_signals()
    finally:
        for sig, handler in previous.items():
            signal.signal(sig, handler)
        signal.pthread_sigmask(signal.SIG_UNBLOCK, MANAGED_SIGNALS)
    if leftover:
        print(f"ERROR: host-run 清不乾淨，仍存活的子程序：{leftover}", file=sys.stderr)
    if late and leftover:
        print(f"ERROR: host-run：紀錄寫出之後才收到訊號 {signal.Signals(late[0]).name}——清不乾淨優先，結束碼維持 "
              f"{final_rc}（與紀錄一致）", file=sys.stderr)
    elif late:
        print(f"ERROR: host-run：紀錄寫出之後才收到訊號 {signal.Signals(late[0]).name}——結束碼改成 {128 + late[0]}，"
              f"紀錄的 rc（{final_rc}）與實際結束碼不符，這一步的量測無效", file=sys.stderr)
        return 128 + late[0]
    return final_rc


def _cleanup_direct_children(me: int, *, term_wait_s: float, total_s: float) -> list[int]:
    """只對直接子程序送訊號（它們只有本程序能收，收之前 PID ⛔ 不會被重用）；孫程序被收養上來就下一輪處理。回傳殘留的 PID。"""
    deadline = time.monotonic() + total_s
    while time.monotonic() < deadline:
        if _reap_nohang():
            return []
        children = _direct_children(me)
        for pid in children:
            try:
                os.kill(pid, signal.SIGTERM)
            except ProcessLookupError:
                pass
        end = min(deadline, time.monotonic() + term_wait_s)
        while time.monotonic() < end and set(children) & set(_direct_children(me)):
            _reap_nohang()
            time.sleep(0.05)
        for pid in set(children) & set(_direct_children(me)):
            try:
                os.kill(pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
        end = min(deadline, time.monotonic() + term_wait_s)
        while time.monotonic() < end and set(children) & set(_direct_children(me)):
            _reap_nohang()
            time.sleep(0.05)
    _reap_nohang()
    return sorted(_direct_children(me))


LEFTOVER_SCHEMA = "i074_stage2_leftover_pids_v1"


def reap_adopted(state: Path, *, parent: int, parent_starttime: int, exclude: Iterable[int] = (), check_only: bool = False,
                 term_wait_s: float = 5.0, kill_wait_s: float = 5.0, total_s: float = 30.0) -> int:
    """⑦d 增補「二」④：清掉 harness（subreaper）收養的程序。結束碼 0 ＝ 沒有／全部清掉、1 ＝ 有（`check_only`）或清完仍有殘留、
    2 ＝ helper 自己失敗（含 parent 的身分不符或在執行期間改變——立刻停止送訊號）。"""
    me, skip = os.getpid(), set(exclude)

    def ensure_parent() -> None:
        if os.getppid() != parent:
            raise ParentChanged(f"getppid()={os.getppid()} ≠ parent {parent}")

    def adopted() -> dict[int, int]:
        ensure_parent()
        found = {}
        for pid, ppid in proc_parents().items():
            if ppid != parent or pid == me or pid in skip:
                continue
            stat = read_proc_stat(pid)
            if stat is not None and stat[0] not in ("Z", "X"):
                found[pid] = stat[1]
        return found

    def signal_pinned(pid: int, starttime: int, sig: int) -> None:
        ensure_parent()
        if not _gone(read_proc_stat(pid), starttime):
            try:
                os.kill(pid, sig)
            except ProcessLookupError:
                pass

    def wait_gone(targets: Mapping[int, int], wait_s: float) -> dict[int, int]:
        end = time.monotonic() + wait_s
        while True:
            alive = {pid: st for pid, st in targets.items() if not _gone(read_proc_stat(pid), st)}
            if not alive or time.monotonic() >= end:
                return alive
            time.sleep(0.05)

    try:
        ensure_parent()
        stat = read_proc_stat(parent)
        if stat is None or stat[1] != parent_starttime:
            raise ParentChanged(f"parent {parent} 的 starttime ≠ {parent_starttime}")
        found = adopted()
        if check_only:
            if found:
                print(f"harness 收養了程序（步驟把程序留在 host）：{sorted(found)}", file=sys.stderr)
            return 1 if found else 0
        deadline = time.monotonic() + total_s
        while found and time.monotonic() < deadline:
            for pid, st in found.items():
                signal_pinned(pid, st, signal.SIGTERM)
            alive = wait_gone(found, term_wait_s)
            for pid, st in alive.items():
                signal_pinned(pid, st, signal.SIGKILL)
            wait_gone(alive, kill_wait_s)
            found = adopted()
        if not found:
            return 0
        processes = []
        for pid, st in sorted(found.items()):
            try:
                cmdline = (PROC_ROOT / str(pid) / "cmdline").read_bytes().replace(b"\0", b" ").decode("utf-8", "replace")
            except OSError:
                cmdline = ""
            processes.append({"pid": pid, "starttime": st, "cmdline": cmdline.strip()})
        out = state / "leftover-pids.json"
        tmp = out.with_name(".leftover-pids.json.tmp")
        tmp.write_bytes(canonical_dumps({"schema": LEFTOVER_SCHEMA, "parent": parent, "processes": processes}))
        os.replace(tmp, out)
        print(f"清不掉 harness 收養的程序：{[p['pid'] for p in processes]}（見 {out}）", file=sys.stderr)
        return 1
    except ParentChanged as exc:
        print(f"ERROR: reap-adopted：parent 的身分不符或已改變——停止送訊號：{exc}", file=sys.stderr)
        return 2
    except Exception as exc:  # noqa: BLE001 - helper 自己的任何失敗一律 2（呼叫端保留 S）
        print(f"ERROR: reap-adopted：{type(exc).__name__}: {exc}", file=sys.stderr)
        return 2


KILL_PINNED_SCHEMA = "i074_stage2_kill_pinned_v1"


def kill_pinned(pid: int, starttime: int, *, term_wait_s: float = 2.0, kill_wait_s: float = 2.0) -> tuple[int, dict[str, Any]]:
    """⑦d 增補「二」④：只送訊號給**同時符合 PID 與 starttime** 的程序；送每一個訊號之前都再讀一次。回傳 (結束碼, 紀錄)。"""
    sent: list[str] = []

    def done(result: str, rc: int, error: str | None = None) -> tuple[int, dict[str, Any]]:
        return rc, {"schema": KILL_PINNED_SCHEMA, "pid": pid, "starttime": starttime, "result": result,
                    "signals_sent": list(sent), "error": error}

    try:
        stat = read_proc_stat(pid)
    except OSError as exc:
        return done("error_before_signal", 2, f"{type(exc).__name__}: {exc}")
    if stat is None or stat[0] in ("Z", "X"):
        return done("already_gone", 0)
    if stat[1] != starttime:
        return done("identity_mismatch", 0)
    for sig, name, wait_s, gone_result in ((signal.SIGTERM, "TERM", term_wait_s, "terminated_by_term"),
                                           (signal.SIGKILL, "KILL", kill_wait_s, "terminated_by_kill")):
        try:
            stat = read_proc_stat(pid)                    # 送這個訊號之前再確認一次身分
        except OSError as exc:
            return done("error_after_term" if sent else "error_before_signal", 2, f"{type(exc).__name__}: {exc}")
        if _gone(stat, starttime):
            # 第一個訊號之前就不在 → already_gone；TERM 之後、KILL 之前才消失 → 是 TERM 收掉的
            return done("terminated_by_term" if sent else "already_gone", 0)
        try:
            os.kill(pid, sig)
        except ProcessLookupError:
            pass
        sent.append(name)
        end = time.monotonic() + wait_s
        while True:
            try:
                stat = read_proc_stat(pid)
            except OSError as exc:
                return done("error_after_term", 2, f"{type(exc).__name__}: {exc}")
            if _gone(stat, starttime):
                return done(gone_result, 0)
            if time.monotonic() >= end:
                break
            time.sleep(0.05)
    return done("alive_after_kill", 1)


def guard_identity(state: Path, shell_pid: int) -> tuple[int, int]:
    """⑦d 增補「二」④③：guard 的測試程序的身分可信 ⟺ `guard-probe.json` 格式正確（恰好 pid、starttime，整數）、
    `pid` ＝ 子 shell 回報的 `$!`、`/proc/<pid>/stat` 的 starttime 相符。不可信 → SizingError（呼叫端⛔ 不送任何訊號）。"""
    try:
        doc = json.loads((state / "guard-probe.json").read_text(encoding="utf-8"))
    except (OSError, ValueError) as exc:
        raise SizingError(f"guard-probe.json 讀不到：{type(exc).__name__}: {exc}") from None
    if not isinstance(doc, dict) or set(doc) != {"pid", "starttime"} or not all(_is_int(v) for v in doc.values()):
        raise SizingError(f"guard-probe.json 的格式不符：{doc!r}")
    if doc["pid"] != shell_pid:
        raise SizingError(f"guard-probe.json 的 pid {doc['pid']} ≠ 子 shell 回報的 $! {shell_pid}")
    stat = read_proc_stat(doc["pid"])
    if stat is None or stat[1] != doc["starttime"]:
        raise SizingError(f"/proc/{doc['pid']}/stat 的 starttime ≠ 回報的 {doc['starttime']}")
    return doc["pid"], doc["starttime"]


def check_step(state: Path, step: str) -> list[str]:
    """⑦d 增補「二」⑦：每一步之後立刻檢查——這一步的 host 紀錄（封閉 schema 與有效條件）與目前為止的每一份 sidecar。"""
    problems: list[str] = []
    rcs = _read_tsv(state / "rc.tsv")
    if step not in rcs:
        return [f"rc.tsv 沒有 {step}"]
    host = state / "host" / f"{step}.json"
    if not host.is_file():
        problems.append(f"缺少 {step} 的 host 紀錄")
    else:
        try:
            rec = read_json(host)
        except ValueError as exc:
            problems.append(f"{step} 的 host 紀錄讀不懂：{exc}")
        else:
            problems += [f"host：{p}" for p in host_record_problems(rec, step=step, rc=int(rcs[step].partition("\t")[2]))]
    for index_path in sorted((state / "index").glob("*.json")) if (state / "index").is_dir() else []:
        index = read_json(index_path)
        side_path = state / "containers" / f"{index['id']}.json"
        if not side_path.is_file():
            problems.append(f"sequence {index['sequence']} 缺 sidecar")
            continue
        side = read_json(side_path)
        if side.get("status") != "ok":
            problems.append(f"sequence {index['sequence']} 量測失敗：{side.get('failures')}")
        elif index.get("profile") == "acceptance" and "max_single_rss_bytes" not in side:
            problems.append(f"sequence {index['sequence']} 的 sidecar 缺 RSS 欄位")
    return problems


# ── ⑦d 增補「二」⑧：靜態（語意）檢查——本 repo 的程式⛔ 不把 SIGCHLD 設成 SIG_IGN、⛔ 沒有 SA_NOCLDWAIT ─────────────
#   ⚠️ ⛔ 不禁止「提到 SIGCHLD」（偵測器要用它算 SigIgn 的位元）；動態寫法（getattr、exec、ctypes 直接呼叫 sigaction）
#   ⛔ 不在靜態檢查內——由執行期的 SigIgn 偵測涵蓋 SIG_IGN。

_CHLD_NAMES = {"SIGCHLD", "SIGCLD"}
SHELL_TRAP_CHLD = re.compile(r"\btrap\s+(?:--\s+)?(?:''|\"\")\s+[^#\n]*\b(?:SIG)?CHLD\b")


def auto_reap_violations(source: str) -> list[int]:
    """回傳違規的行號：先解析每個檔的 `signal` 名稱來源（import ／ import as ／ from-import [as]），再找「呼叫目標解析成
    signal.signal、訊號解析成 SIGCHLD／SIGCLD（含 Signals.* 與整數 17）、處理函式（位置或 handler=）解析成 SIG_IGN
    （含 Handlers.SIG_IGN）」，以及任何名為 SA_NOCLDWAIT 的識別字。⚠️ 字串⛔ 不算——說明文字與報告的 notes 會提到它
    （契約外的殘餘），字串本身也設不了任何旗標。"""
    import ast  # noqa: PLC0415
    tree = ast.parse(source)
    mods: set[str] = set()
    funcs: set[str] = set()
    chld: set[str] = set()
    ign: set[str] = set()
    signals_enum: set[str] = set()
    handlers_enum: set[str] = set()
    for node in ast.walk(tree):
        if isinstance(node, ast.Import):
            mods |= {a.asname or a.name for a in node.names if a.name == "signal"}
        elif isinstance(node, ast.ImportFrom) and node.module == "signal":
            for a in node.names:
                name = a.asname or a.name
                if a.name == "signal":
                    funcs.add(name)
                elif a.name == "SIG_IGN":
                    ign.add(name)
                elif a.name == "Signals":
                    signals_enum.add(name)
                elif a.name == "Handlers":
                    handlers_enum.add(name)
                elif a.name in _CHLD_NAMES:
                    chld.add(name)

    def is_mod(n: Any) -> bool:
        return isinstance(n, ast.Name) and n.id in mods

    def is_chld(n: Any) -> bool:
        if isinstance(n, ast.Constant) and type(n.value) is int and n.value == 17:
            return True
        if isinstance(n, ast.Name):
            return n.id in chld
        if isinstance(n, ast.Attribute) and n.attr in _CHLD_NAMES:
            v = n.value
            return is_mod(v) or (isinstance(v, ast.Name) and v.id in signals_enum) \
                or (isinstance(v, ast.Attribute) and v.attr == "Signals" and is_mod(v.value))
        return False

    def is_ign(n: Any) -> bool:
        if isinstance(n, ast.Name):
            return n.id in ign
        if isinstance(n, ast.Attribute) and n.attr == "SIG_IGN":
            v = n.value
            return is_mod(v) or (isinstance(v, ast.Name) and v.id in handlers_enum) \
                or (isinstance(v, ast.Attribute) and v.attr == "Handlers" and is_mod(v.value))
        return False

    bad = []
    for node in ast.walk(tree):
        if isinstance(node, ast.Name) and node.id == "SA_NOCLDWAIT" \
                or isinstance(node, ast.Attribute) and node.attr == "SA_NOCLDWAIT":
            bad.append(node.lineno)
        elif isinstance(node, ast.Call):
            f = node.func
            if (isinstance(f, ast.Attribute) and f.attr == "signal" and is_mod(f.value)) \
                    or (isinstance(f, ast.Name) and f.id in funcs):
                kw = {k.arg: k.value for k in node.keywords}
                sig = node.args[0] if node.args else kw.get("signalnum")
                handler = node.args[1] if len(node.args) > 1 else kw.get("handler")
                if sig is not None and handler is not None and is_chld(sig) and is_ign(handler):
                    bad.append(node.lineno)
    return bad


def scan_auto_reaping(roots: Iterable[Path], repo_root: Path) -> list[str]:
    """掃 roots 底下（測試除外）的 .py（AST）與 .sh（`trap '' … CHLD`）。回傳「路徑:行號」。"""
    found = []
    for root in roots:
        for path in sorted(Path(root).rglob("*")):
            rel = path.relative_to(repo_root)
            if not path.is_file() or "tests" in rel.parts or path.name.startswith(("test_", "test-")):
                continue
            if path.suffix == ".py":
                found += [f"{rel}:{n}" for n in auto_reap_violations(path.read_text(encoding="utf-8"))]
            elif path.suffix == ".sh":
                found += [f"{rel}:{i}" for i, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1)
                          if SHELL_TRAP_CHLD.search(line)]
    return found


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
    kinds = sorted({kind for _cid, kind in events} - {"create_begin", "rm_done"})
    if kinds:
        raise SizingError(f"lifecycle event 有不認得的種類：{kinds}")
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


ACCEPTANCE_PHASE_DIRS = ("failure", "promote_failure", "promote_success", "success")   # phases/ 恰好這四個量測窗口
COLLECT_STAGES = ("before_full", "complete")
ACCEPTANCE_DISK_PHASES = PROFILES["acceptance"]["disk_phases"]


def acceptance_expected(compute: str, stage: str) -> tuple[list[tuple[str, str]], list[tuple[str, str]]]:
    """（預期的步驟, 預期的 invocation）。`before_full` ＝ full 模式正式集合的**精確前綴**（少了最後的量測趟）——以
    「正式集合[:len(前綴)] ＝ 前綴、且恰好少一個」斷言（有效性條件計畫「二」的「共用的 collector」）。"""
    if stage not in COLLECT_STAGES:
        raise SizingError(f"stage 只接受 {COLLECT_STAGES}：{stage!r}")
    steps = list(ACCEPTANCE_STEPS) + ([ACCEPTANCE_FULL_STEP] if compute == "full" else [])
    invocations = expected_invocations("acceptance", compute)
    if stage == "complete":
        return steps, invocations
    if compute != "full":
        raise SizingError(f"before_full 只用在 replay_compute=full（本次 {compute!r}）")
    pre_steps, pre_invocations = list(ACCEPTANCE_STEPS), invocations[:-1]
    if steps[:len(pre_steps)] != pre_steps or len(steps) != len(pre_steps) + 1 \
            or invocations[:len(pre_invocations)] != pre_invocations or len(invocations) != len(pre_invocations) + 1:
        raise SizingError("before_full 的預期集合⛔ 不是正式集合的精確前綴")
    return pre_steps, pre_invocations


def _exact_names(directory: Path, want: Iterable[str], label: str) -> None:
    """目錄的成員必須**恰好**是預期的集合——⛔ 不靜默忽略多出的資料（第二輪 review）。"""
    got = sorted(p.name for p in directory.iterdir()) if directory.is_dir() else []
    if got != sorted(want):
        raise SizingError(f"{label} 的成員與預期不符：預期 {sorted(want)}，實際 {got}")


def collect_acceptance_measurements(state: Path, *, stage: str, drop_names: Iterable[str] = (),
                                    drop_prefixes: tuple[str, ...] = ()) -> dict[str, Any]:
    """⑦d「二之五」的量測蒐集（有效性條件計畫「二」的**共用 collector**）。回傳**內部的量測快照**——⛔ 不是
    `i074_stage2_acceptance_report_v2`（沒有 schema、contract、status、violations 等頂層鍵，v2 的 validator 一定拒絕它）。

    `stage="complete"`：本次 `replay_compute` 的正式集合（最後的報告）；`stage="before_full"`：full 模式在量測趟之前的精確前綴
    （precheck）。讀的每一個集合（`rc.tsv`、`index/`、`containers/`、`twins/`、lifecycle event、`host/`、`phases/`）都必須
    **恰好**等於預期——⛔ 不靜默忽略多出的資料。
    """
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
    steps, expected = acceptance_expected(compute, stage)
    for step, value in rc.items():
        want, _, actual = value.partition("\t")
        if want != actual:
            raise SizingError(f"{step} 的結束碼 {actual} ≠ 預期 {want}")
    if list(rc) != [s for s, _ in steps]:
        raise SizingError(f"步驟與預期不符：預期 {[s for s, _ in steps]}，實際 {list(rc)}")

    invocations = load_invocations(state, "acceptance", compute, expected=expected)
    events = load_events(state)
    _check_events(invocations, events)
    _exact_names(state / "phases", ACCEPTANCE_PHASE_DIRS, "phases/")
    for inv in invocations:
        inv["footprint"] = inv["sidecar"]["log_bound"] + inv["twin"]["adopted_bytes"] + inv["sidecar"]["size_rw"]

    wt = {name: _require_int(comp, name, "components") for name in (
        "wt_head", "wt_base_patched", "git_head", "git_base_patched", "index_head", "index_base_patched",
        "snapshot", "probe", "frozen_patched")}
    manifest, manifest_sha = load_harness_manifest(state, "acceptance")
    paths: dict[str, Any] = {}
    for phase in ACCEPTANCE_DISK_PHASES:
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
        paths[phase] = {"peaks": peaks, "accounted": accounted, "accounted_parts": parts, "container_rule": rule,
                        "P_path": max(peaks["dirs_peak"], peaks["fs_peak"], accounted),
                        "read_only_gap": sum(c["sidecar"]["size_rw"] for c in containers)}
    promotion: dict[str, Any] = {}
    for phase, path in (("promote_success", "success"), ("promote_failure", "failure")):
        baseline, end, peaks = _phase_window(state, phase, meta)
        containers = [i for i in invocations if i["phase"] == phase]
        footprint = sum(c["footprint"] for c in containers)
        accounted = end["archive"]["allocated"] + end["archive"]["dirs"] * BLOCK + footprint
        p_prom = max(peaks["dirs_peak"], peaks["fs_peak"], accounted)
        promotion[phase] = {"peaks": peaks, "accounted": accounted, "P_promotion": p_prom,
                            "P_path_plus_promotion": paths[path]["P_path"] + p_prom, "kind": PROMOTION_KIND}
    memory = []
    for inv in invocations:
        side = inv["sidecar"]
        missing = [k for k in ("max_single_rss_bytes", "self_max_rss_bytes", "children_max_rss_bytes",
                               "rss_peak_sampled_bytes", "rss_samples", "reaper") if k not in side]
        if missing:
            raise SizingError(f"sequence {inv['sequence']} 的 sidecar 缺 RSS 欄位（⑦d 增補）：{missing}")
        limit = container_memory_limit(inv)
        memory.append({"sequence": inv["sequence"], "phase": inv["phase"], "role": inv["role"], "rc": side["rc"],
                       "max_single_rss_bytes": side["max_single_rss_bytes"],
                       "self_max_rss_bytes": side["self_max_rss_bytes"],
                       "children_max_rss_bytes": side["children_max_rss_bytes"],
                       "rss_peak_sampled_bytes": side["rss_peak_sampled_bytes"], "rss_samples": side["rss_samples"],
                       "reaper": side["reaper"], "cgroup_peak_bytes": side["peak_bytes"], "memory_limit_bytes": limit,
                       "below_limit": side["max_single_rss_bytes"] < ACCEPTANCE_MEMORY_LIMIT
                       and side["rss_peak_sampled_bytes"] < ACCEPTANCE_MEMORY_LIMIT})
    host = []
    for step, phase in steps:
        path = state / "host" / f"{step}.json"
        if not path.is_file():
            raise SizingError(f"缺少 {step} 的 host 端量測")
        rec = read_json(path)
        problems = host_record_problems(rec, step=step, rc=int(rc[step].partition("\t")[2]))
        if problems:
            raise SizingError(f"{step} 的 host 紀錄不符（⑦d 增補「二」④）：{problems}")
        env_problems = check_step_environment(rec, drop_names=drop_names, drop_prefixes=drop_prefixes)
        if env_problems:
            raise SizingError(f"環境的契約不符（harness 的缺陷，⛔ 不產報告）：{env_problems}")
        alarm = rec["group_rss_peak_sampled_bytes"] >= ACCEPTANCE_MEMORY_LIMIT
        host.append({"step": step, "phase": phase, "rc": rec["rc"], "self_max_rss_bytes": rec["self_max_rss_bytes"],
                     "children_max_rss_bytes": rec["children_max_rss_bytes"],
                     "max_single_rss_bytes": rec["max_single_rss_bytes"],
                     "group_rss_peak_sampled_bytes": rec["group_rss_peak_sampled_bytes"], "group_alarm": alarm,
                     "group_note": GROUP_NOTES[alarm]})
    _exact_names(state / "host", [f"{s}.json" for s, _ in steps], "host/")      # 多出的 host 紀錄
    memavail = [int(line.split("\t")[1]) for line in (state / "memavail.tsv").read_text().splitlines() if line.strip()] \
        if (state / "memavail.tsv").is_file() else []
    return {"stage": stage, "meta": meta, "components": comp, "compute": compute, "steps": [s for s, _ in steps],
            "sequences": [inv["sequence"] for inv in invocations], "harness_manifest": manifest,
            "harness_manifest_sha256": manifest_sha, "paths": paths, "promotion": promotion, "memory": memory,
            "host": host, "host_memavailable_low": min(memavail) if memavail else None}


def acceptance_disk(paths: Mapping[str, Any]) -> dict[str, dict[str, int]]:
    """兩條磁碟路徑的判定數字，**依 success、failure 的順序**（有效性問題與違反的順序以它為準）。"""
    return {phase: disk_numbers(paths[phase]) for phase in ACCEPTANCE_DISK_PHASES}


def build_acceptance_report(state: Path, *, p_b_budget: int, drop_names: Iterable[str] = (),
                            drop_prefixes: tuple[str, ...] = ()) -> dict[str, Any]:
    """⑦d「二之五」：兩條磁碟路徑（門檻）、兩次晉升（資訊值）、每個容器與每一步 host 程序樹的記憶體（門檻與單向警報）。

    ⚠️ 有效性條件計畫「二」（第三輪 review：v2 **維持原語意**）：報告 v2 **只在沒有任何有效性問題時**寫出——有有效性問題
    （不論有沒有確定的違反）→ `SizingError`、⛔ 不產報告；混合狀態的判定只由 `precheck.json` 表達。full 模式的磁碟窗口在
    precheck 之前就結束、precheck 不是 ok 就⛔ 不跑量測趟，所以這裡出現有效性問題就是 harness 的缺陷。
    """
    snap = collect_acceptance_measurements(state, stage="complete", drop_names=drop_names, drop_prefixes=drop_prefixes)
    validity = disk_validity_problems(acceptance_disk(snap["paths"]), p_b_budget)
    if validity:
        raise SizingError(f"有效性問題（報告 v2 只在沒有任何有效性問題時寫出；⛔ 不產報告）：{validity_text(validity)}")
    compute, memory = snap["compute"], snap["memory"]
    notes = ["契約（⑦d 增補）：經正常 wait 鏈保存 resource usage 的每一個程序各自 < 450 MiB——通過由精確的閘證明"
             "（max(RUSAGE_SELF, RUSAGE_CHILDREN)：wrapper／host-run 自己、leader 與被收掉的子孫）；被 kernel 自動回收的子孫"
             "在契約外（SIGCHLD=SIG_IGN 以取樣偵測、偵測到即 fail-closed；SA_NOCLDWAIT 看不到）；取樣的 total_rss 只作單向偵測"
             "（間隔 50 ms 的下界，沒觀察到超標⛔ 不是通過的證據）",
             "ru_maxrss 含 file-backed 的常駐頁（共用函式庫、mmap）與 fork 之後、exec 之前和 parent 共用的頁；wrapper 自己"
             "（約 10 MiB）也計入——都偏保守",
             "含 page cache 的 cgroup 峰值只列資訊值：它會隨當次的上限上升（page cache 要逼近上限才被回收）",
             "anchors 以 preflight 的 --check-failed-record 代量（⑦d「三」#6）",
             "host 端是 RSS（⛔ 不含 page cache）：最大單一程序是門檻、程序群組的取樣總和只作單向警報",
             "晉升的磁碟只列資訊值（P_B 的起訖⛔ 不變）"]
    bounded = [row for row in memory if row["memory_limit_bytes"] < ACCEPTANCE_MEMORY_LIMIT]
    if bounded:
        listed = "、".join(f"#{row['sequence']}" for row in bounded)
        notes.append(f"容器 {listed} 的上限嚴格低於門檻：匿名頁受上限限制，total_rss 偵測器不可能觸發；⚠️ 精確的閘仍逐一比較"
                     "（file-backed 頁可能記在別的 cgroup，⛔ 不受這個上限限制）")
    if compute == "stub":
        notes.insert(0, "⚠️ replay_compute=stub：計算工作集⛔ 未涵蓋——⛔ 不得當成 ⑨-1 的正式驗收")
    report: dict[str, Any] = {
        "schema": REPORT_V2_SCHEMA,
        "contract": REPORT_CONTRACT,
        "out_of_contract": list(REPORT_OUT_OF_CONTRACT),
        "auto_reap_detection": dict(REPORT_AUTO_REAP_DETECTION),
        "memory_measures": dict(REPORT_MEMORY_MEASURES),
        "mode": snap["meta"]["mode"],
        "replay_compute": compute,
        "meta": snap["meta"],
        "components": snap["components"],
        "harness_manifest": snap["harness_manifest"],
        "harness_manifest_sha256": snap["harness_manifest_sha256"],
        "limits": {"memory_bytes": ACCEPTANCE_MEMORY_LIMIT, "P_B_BUDGET": p_b_budget,
                   "container_memory_limit_bytes": sorted({row["memory_limit_bytes"] for row in memory})},
        "paths": snap["paths"],
        "promotion": snap["promotion"],
        "memory": memory,
        "host": snap["host"],
        "host_memavailable_low": snap["host_memavailable_low"],
        "notes": notes,
    }
    report["violations"] = derive_acceptance_violations(report)
    report["status"] = "ok" if not report["violations"] else "threshold_exceeded"
    problems = validate_acceptance_report_v2(report, p_b_budget=p_b_budget)
    if problems:
        raise SizingError(f"報告的自我驗證不通過（⛔ 不寫報告）：{problems[:5]}")
    return report


# ── ⑨-1 的 fail-fast：precheck（有效性條件計畫「二」的「`precheck.json` 的契約」） ─────────────────────────────

PRECHECK_SCHEMA = "i074_stage2_acceptance_precheck_v1"
PRECHECK_STATUSES = ("ok", "threshold_exceeded", "invalid")
PRECHECK_EXIT = {"ok": 0, "threshold_exceeded": 2, "invalid": 3}
PRECHECK_KEYS = frozenset({"schema", "status", "full_trip", "identity", "limits", "steps", "sequences", "disk",
                           "validity_problems", "violations"})
PRECHECK_IDENTITY_KEYS = ("run_id", "mode", "replay_compute", "image", "repo_head", "clone_head", "harness_manifest_sha256")
_PRECHECK_LIMITS_KEYS = frozenset({"memory_bytes", "P_B_BUDGET", "fs_unexplained_tolerance_bytes"})
_DISK_KEYS = frozenset({"P_path", "dirs_peak", "fs_peak", "accounted", "P_basis", "fs_unexplained"})
MEMORY_VIOLATION_KINDS = ("container_max_single_rss", "container_rss_sampled", "host_max_single_rss",
                          "host_group_rss_sampled")
PRECHECK_VIOLATION_KINDS = MEMORY_VIOLATION_KINDS + ("disk_p_basis",)
_HEX40 = re.compile(r"[0-9a-f]{40}")
_HEX64 = re.compile(r"[0-9a-f]{64}")


def precheck_from_snapshot(snap: Mapping[str, Any], *, p_b_budget: int) -> dict[str, Any]:
    """量測趟之前的判定。優先序（第二輪 review）：① **確定的違反**（記憶體的每一類、`P_basis` ＞ 預算）→ threshold_exceeded，
    同一次的有效性問題照樣記錄；② 沒有確定的違反、但有有效性問題 → invalid；③ 都沒有 → ok。"""
    if snap["stage"] != "before_full":
        raise SizingError("precheck 只吃 before_full 的量測快照")
    disk = acceptance_disk(snap["paths"])
    validity = disk_validity_problems(disk, p_b_budget)
    violations = derive_memory_violations(snap["memory"], snap["host"], ACCEPTANCE_MEMORY_LIMIT) \
        + disk_basis_violations(disk, p_b_budget)
    status = "threshold_exceeded" if violations else "invalid" if validity else "ok"
    meta = snap["meta"]
    identity = {key: meta[key] for key in PRECHECK_IDENTITY_KEYS if key != "harness_manifest_sha256"}
    identity["harness_manifest_sha256"] = snap["harness_manifest_sha256"]
    return {"schema": PRECHECK_SCHEMA, "status": status, "full_trip": "allowed" if status == "ok" else "not_started",
            "identity": identity,
            "limits": {"memory_bytes": ACCEPTANCE_MEMORY_LIMIT, "P_B_BUDGET": p_b_budget,
                       "fs_unexplained_tolerance_bytes": FS_UNEXPLAINED_TOLERANCE},
            "steps": list(snap["steps"]), "sequences": list(snap["sequences"]), "disk": disk,
            "validity_problems": validity, "violations": violations}


def build_acceptance_precheck(state: Path, *, p_b_budget: int, drop_names: Iterable[str] = (),
                              drop_prefixes: tuple[str, ...] = ()) -> dict[str, Any]:
    snap = collect_acceptance_measurements(state, stage="before_full", drop_names=drop_names, drop_prefixes=drop_prefixes)
    doc = precheck_from_snapshot(snap, p_b_budget=p_b_budget)
    problems = validate_acceptance_precheck_v1(doc, p_b_budget=p_b_budget)
    if problems:
        raise SizingError(f"precheck 的自我驗證不通過（⛔ 不寫）：{problems[:5]}")
    return doc


def validate_acceptance_precheck_v1(doc: Any, *, p_b_budget: int) -> list[str]:
    """`precheck.json` 的封閉 schema、衍生欄位、由 `disk` 重新推導的有效性問題與磁碟違反、優先序。

    ⚠️ 與報告 v2 的 validator 同一套紀律：每一節先驗完型別、任何未預期的例外都轉成問題（⛔ 不拋、⛔ 不放行）。記憶體列的違反
    ⛔ 不在這裡推導（`precheck.json` 沒有記憶體列）——由讀取端從原始量測**重算逐位元比對**證明。回傳問題清單（空 ＝ 有效）。
    """
    problems: list[str] = []
    try:
        _validate_precheck(doc, p_b_budget, problems)
    except Exception as exc:  # noqa: BLE001 - 後援：任何未預期的結構都是「不符」
        problems.append(f"驗證時發生例外（結構不符）：{type(exc).__name__}: {exc}")
    return problems


def _rows_ok(problems: list[str], where: str, rows: Any, kinds: tuple[str, ...]) -> bool:
    if not isinstance(rows, list):
        problems.append(f"{where} 必須是陣列")
        return False
    ok = True
    for i, row in enumerate(rows):
        w = f"{where}[{i}]"
        if not _closed(problems, w, row, _VIOLATION_KEYS):
            ok = False
            continue
        if not _typed(problems, w, row, {"kind": "str", "subject": "str", "value": "int", "limit": "int"}):
            ok = False
            continue
        if row["kind"] not in kinds:
            problems.append(f"{w}.kind={row['kind']!r}（不在封閉的 enum 裡）")
            ok = False
    return ok


def _validate_precheck(doc: Any, p_b_budget: int, problems: list[str]) -> None:
    if not _closed(problems, "precheck", doc, PRECHECK_KEYS):
        return
    if not isinstance(doc["schema"], str) or doc["schema"] != PRECHECK_SCHEMA:
        problems.append(f"schema={doc['schema']!r}")
    for key, allowed in (("status", PRECHECK_STATUSES), ("full_trip", ("allowed", "not_started"))):
        if not isinstance(doc[key], str) or doc[key] not in allowed:
            problems.append(f"{key}={doc[key]!r}（只能是 {allowed}）")
    identity = doc["identity"]
    if _closed(problems, "identity", identity, frozenset(PRECHECK_IDENTITY_KEYS)):
        if _typed(problems, "identity", identity, dict.fromkeys(PRECHECK_IDENTITY_KEYS, "str")):
            if identity["mode"] not in ("validation", "formal"):
                problems.append(f"identity.mode={identity['mode']!r}")
            if identity["replay_compute"] != "full":
                problems.append(f"identity.replay_compute={identity['replay_compute']!r}（precheck 只在 full）")
            for key in ("repo_head", "clone_head"):
                if not _HEX40.fullmatch(identity[key]):
                    problems.append(f"identity.{key} 必須是 40 碼小寫 hex")
            if identity["repo_head"] != identity["clone_head"]:
                problems.append("identity.clone_head ≠ repo_head")
            if not _HEX64.fullmatch(identity["harness_manifest_sha256"]):
                problems.append("identity.harness_manifest_sha256 必須是 64 碼小寫 hex")
    limits = doc["limits"]
    if _closed(problems, "limits", limits, _PRECHECK_LIMITS_KEYS):
        for key, want in (("memory_bytes", ACCEPTANCE_MEMORY_LIMIT), ("P_B_BUDGET", p_b_budget),
                          ("fs_unexplained_tolerance_bytes", FS_UNEXPLAINED_TOLERANCE)):
            if not _is_int(limits[key]) or limits[key] != want:
                problems.append(f"limits.{key}={limits[key]!r} ≠ {want}")
    want_steps, want_invocations = acceptance_expected("full", "before_full")
    if doc["steps"] != [s for s, _ in want_steps]:
        problems.append("steps ≠ 量測趟之前的步驟（精確前綴）")
    sequences = doc["sequences"]
    if not isinstance(sequences, list) or not all(_is_int(v) for v in sequences) \
            or sequences != list(range(1, len(want_invocations) + 1)):
        problems.append("sequences ≠ 量測趟之前的 invocation（1..n）")
    disk = doc["disk"]
    disk_ok = isinstance(disk, dict) and set(disk) == set(ACCEPTANCE_DISK_PHASES)
    if not disk_ok:
        problems.append(f"disk 必須恰好是 {ACCEPTANCE_DISK_PHASES}")
    else:
        for phase in ACCEPTANCE_DISK_PHASES:
            where, d = f"disk.{phase}", disk[phase]
            if not _closed(problems, where, d, _DISK_KEYS) or not _typed(problems, where, d, {
                    "P_path": "nonneg", "dirs_peak": "nonneg", "fs_peak": "int", "accounted": "nonneg",
                    "P_basis": "nonneg", "fs_unexplained": "int"}):
                disk_ok = False
                continue
            if d["P_basis"] != max(d["dirs_peak"], d["accounted"]):
                problems.append(f"{where}.P_basis ≠ max(dirs_peak, accounted)")
            if d["fs_unexplained"] != d["fs_peak"] - d["P_basis"]:
                problems.append(f"{where}.fs_unexplained ≠ fs_peak − P_basis")
            if d["P_path"] != max(d["dirs_peak"], d["fs_peak"], d["accounted"]):
                problems.append(f"{where}.P_path ≠ max(dirs_peak, fs_peak, accounted)")
    rows_ok = _rows_ok(problems, "validity_problems", doc["validity_problems"], VALIDITY_KINDS)
    rows_ok = _rows_ok(problems, "violations", doc["violations"], PRECHECK_VIOLATION_KINDS) and rows_ok
    if problems or not disk_ok or not rows_ok:
        return
    ordered = {phase: disk[phase] for phase in ACCEPTANCE_DISK_PHASES}     # ⚠️ canonical JSON 會把鍵排序——順序以這裡為準
    if doc["validity_problems"] != disk_validity_problems(ordered, p_b_budget):
        problems.append("validity_problems 與由 disk 重新推導的結果不符")
    memory_part = [v for v in doc["violations"] if v["kind"] != "disk_p_basis"]
    if doc["violations"] != memory_part + disk_basis_violations(ordered, p_b_budget):
        problems.append("violations 的磁碟部分與由 disk 重新推導的結果不符（或⛔ 不在記憶體之後）")
    for v in memory_part:
        if v["limit"] != ACCEPTANCE_MEMORY_LIMIT or v["value"] < v["limit"]:
            problems.append(f"記憶體的違反 {v} 與門檻不符")
    want_status = "threshold_exceeded" if doc["violations"] else "invalid" if doc["validity_problems"] else "ok"
    if doc["status"] != want_status:
        problems.append(f"status={doc['status']!r} ≠ 優先序推導的 {want_status!r}")
    if doc["full_trip"] != ("allowed" if doc["status"] == "ok" else "not_started"):
        problems.append(f"full_trip={doc['full_trip']!r} 與 status 不符")


def precheck_text(doc: Mapping[str, Any]) -> str:
    mib = lambda b: f"{b / 1048576:.1f} MiB"  # noqa: E731
    head = {"ok": "ok——量測趟可以執行", "threshold_exceeded": "threshold_exceeded——**確定的違反**，⛔ 不得重跑（回退順序）；量測趟未執行、額度未消耗",
            "invalid": "invalid——量測無效（⛔ 不是判定，可以重跑）；量測趟未執行、額度未消耗"}[doc["status"]]
    lines = [f"precheck：{head}"]
    for v in doc["violations"]:
        lines.append(f"  ⚠️ {v['kind']}：{v['subject']} {mib(v['value'])}（門檻 {mib(v['limit'])}）")
    if doc["validity_problems"]:
        lines.append(f"  有效性問題：{validity_text(doc['validity_problems'])}")
    for phase in ACCEPTANCE_DISK_PHASES:
        d = doc["disk"][phase]
        lines.append(f"  [{phase}] P_path={d['P_path']}  P_basis={d['P_basis']}  fs_unexplained={d['fs_unexplained']}  "
                     f"（預算 {doc['limits']['P_B_BUDGET']}、容差 {doc['limits']['fs_unexplained_tolerance_bytes']}）")
    return "\n".join(lines) + "\n"


# ── ⑦d 增補：報告 v2 的封閉 schema、唯一的門檻推導與驗證 ─────────────────────────────

REPORT_V2_SCHEMA = "i074_stage2_acceptance_report_v2"
REPORT_CONTRACT = "per_process_rss_within_wait_chain"
REPORT_OUT_OF_CONTRACT = ("descendants_auto_reaped_by_kernel",)
REPORT_AUTO_REAP_DETECTION = {"sigchld_sig_ign": "sampled_fail_closed", "sa_nocldwait": "unobservable"}
REPORT_MEMORY_MEASURES = {"max_single_rss_bytes": "exact_within_wait_chain", "rss_peak_sampled_bytes": "sampled_lower_bound",
                          "cgroup_peak_bytes": "informational_includes_page_cache"}
REPORT_KEYS = frozenset({"schema", "status", "violations", "contract", "out_of_contract", "auto_reap_detection",
                         "memory_measures", "mode", "replay_compute", "meta", "components", "harness_manifest",
                         "harness_manifest_sha256", "limits", "paths", "promotion", "memory", "host",
                         "host_memavailable_low", "notes"})
PROMOTION_KIND = "informational（⛔ 不與 P_B_BUDGET 比較；v29「八之一」容量列）"
GROUP_NOTES = {True: "觀察到超標", False: "未觀察到超標（⛔ 不是通過的證據）"}
VIOLATION_KINDS = ("container_max_single_rss", "container_rss_sampled", "host_max_single_rss", "host_group_rss_sampled",
                   "disk_p_path")
_MEMORY_ROW_KEYS = frozenset({"sequence", "phase", "role", "rc", "max_single_rss_bytes", "self_max_rss_bytes",
                              "children_max_rss_bytes", "rss_peak_sampled_bytes", "rss_samples", "reaper",
                              "cgroup_peak_bytes", "memory_limit_bytes", "below_limit"})
_HOST_ROW_KEYS = frozenset({"step", "phase", "rc", "self_max_rss_bytes", "children_max_rss_bytes", "max_single_rss_bytes",
                            "group_rss_peak_sampled_bytes", "group_alarm", "group_note"})
_PATH_KEYS = frozenset({"peaks", "accounted", "accounted_parts", "container_rule", "P_path", "read_only_gap"})
_PROMOTION_KEYS = frozenset({"peaks", "accounted", "P_promotion", "P_path_plus_promotion", "kind"})
_PEAKS_KEYS = frozenset({"dirs_peak", "dirs_peak_by_location", "fs_peak", "samples"})
_LIMITS_KEYS = frozenset({"memory_bytes", "P_B_BUDGET", "container_memory_limit_bytes"})
_VIOLATION_KEYS = frozenset({"kind", "subject", "value", "limit"})


def _str_list(value: Any) -> bool:
    return isinstance(value, list) and all(isinstance(v, str) for v in value)


def host_record_problems(rec: Any, *, step: str, rc: int) -> list[str]:
    """⑦d 增補「二」④：`host-run` 紀錄的封閉 schema（十七個鍵）、交叉條件與有效條件。回傳問題清單（空 ＝ 有效）。"""
    if not isinstance(rec, dict):
        return ["不是 JSON object"]
    if set(rec) != set(HOST_RECORD_KEYS):
        return [f"鍵集合不符：缺 {sorted(set(HOST_RECORD_KEYS) - set(rec))}、多 {sorted(set(rec) - set(HOST_RECORD_KEYS))}"]
    problems = []
    if rec["schema"] != HOST_RECORD_SCHEMA:
        problems.append(f"schema={rec['schema']!r}")
    if rec["step"] != step:
        problems.append(f"step={rec['step']!r} ≠ 檔名 {step!r}")
    if not _is_int(rec["rc"]) or not 0 <= rec["rc"] <= 255 or rec["rc"] != rc:
        problems.append(f"rc={rec['rc']!r}（必須是 0～255 的整數、＝ rc.tsv 的 {rc}）")
    for key in ("self_max_rss_bytes", "children_max_rss_bytes", "max_single_rss_bytes", "group_rss_peak_sampled_bytes",
                "group_samples"):
        if not _is_int(rec[key]) or rec[key] <= 0:
            problems.append(f"{key}={rec[key]!r}（必須是 > 0 的整數）")
    if not problems and rec["max_single_rss_bytes"] != max(rec["self_max_rss_bytes"], rec["children_max_rss_bytes"]):
        problems.append("max_single_rss_bytes ≠ max(self_max_rss_bytes, children_max_rss_bytes)")
    if not _is_int(rec["group_interval_ms"]) or rec["group_interval_ms"] != HOST_INTERVAL_MS:
        problems.append(f"group_interval_ms={rec['group_interval_ms']!r}（必須是 {HOST_INTERVAL_MS}）")
    if rec["reaper"] != "subreaper":
        problems.append(f"reaper={rec['reaper']!r}（必須是 subreaper）")
    for key, want in (("all_descendants_reaped", True), ("auto_reap_detected", False), ("cleanup_complete", True)):
        if type(rec[key]) is not bool or rec[key] is not want:
            problems.append(f"{key}={rec[key]!r}（有效的量測必須是 {want}）")
    if not isinstance(rec["leftover_pids"], list) or not all(_is_int(v) for v in rec["leftover_pids"]):
        problems.append("leftover_pids 必須是整數陣列")
    elif rec["leftover_pids"]:
        problems.append(f"leftover_pids={rec['leftover_pids']}（有效的量測必須是空的）")
    if not _str_list(rec["errors"]):
        problems.append("errors 必須是字串陣列")
    elif rec["errors"]:
        problems.append(f"errors={rec['errors']}")
    if not _str_list(rec["env_keys"]) or rec["env_keys"] != sorted(set(rec["env_keys"])):
        problems.append("env_keys 必須是排序、不重複的字串陣列")
    if not _str_list(rec["cmd"]) or not rec["cmd"]:
        problems.append("cmd 必須是非空的字串陣列")
    return problems


def derive_memory_violations(memory: Iterable[Mapping[str, Any]], host: Iterable[Mapping[str, Any]],
                             limit: int) -> list[dict[str, Any]]:
    """記憶體的門檻推導（報告 v2 與 precheck 共用的唯一定義）。順序：容器列、host 列。"""
    out: list[dict[str, Any]] = []
    for row in memory:
        subject = f"#{row['sequence']} {row['phase']}/{row['role']}"
        if row["max_single_rss_bytes"] >= limit:
            out.append({"kind": "container_max_single_rss", "subject": subject, "value": row["max_single_rss_bytes"],
                        "limit": limit})
        if row["rss_peak_sampled_bytes"] >= limit:
            out.append({"kind": "container_rss_sampled", "subject": subject, "value": row["rss_peak_sampled_bytes"],
                        "limit": limit})
    for row in host:
        if row["max_single_rss_bytes"] >= limit:
            out.append({"kind": "host_max_single_rss", "subject": row["step"], "value": row["max_single_rss_bytes"],
                        "limit": limit})
        if row["group_rss_peak_sampled_bytes"] >= limit:
            out.append({"kind": "host_group_rss_sampled", "subject": row["step"],
                        "value": row["group_rss_peak_sampled_bytes"], "limit": limit})
    return out


def derive_acceptance_violations(report: Mapping[str, Any]) -> list[dict[str, Any]]:
    """⑦d 增補：報告 v2 **唯一的**門檻推導（產出端與 validator 共用）。順序：容器列、host 列、磁碟（success、failure）。
    ⚠️ 有效性條件計畫（第三輪 review）：輸出⛔ 不變——磁碟仍以 `P_path` 判（v2 只在沒有有效性問題時寫出）。"""
    limit, budget = report["limits"]["memory_bytes"], report["limits"]["P_B_BUDGET"]
    out = derive_memory_violations(report["memory"], report["host"], limit)
    for phase in ("success", "failure"):
        if report["paths"][phase]["P_path"] > budget:
            out.append({"kind": "disk_p_path", "subject": phase, "value": report["paths"][phase]["P_path"],
                        "limit": budget})
    return out


def _closed(problems: list[str], where: str, value: Any, keys: frozenset) -> bool:
    if not isinstance(value, dict) or set(value) != keys:
        got = sorted(value) if isinstance(value, dict) else type(value).__name__
        problems.append(f"{where} 的鍵集合不符：{got}")
        return False
    return True


def _typed(problems: list[str], where: str, row: Mapping[str, Any], spec: Mapping[str, str]) -> bool:
    """逐欄驗型別：int（⛔ 不接受 bool）、pos（> 0 的 int）、nonneg（≥ 0 的 int）、bool、str。全部符合才回傳 True。"""
    ok = True
    for key, kind in spec.items():
        value = row[key]
        good = {"int": _is_int(value), "pos": _is_int(value) and value > 0, "nonneg": _is_int(value) and value >= 0,
                "bool": type(value) is bool, "str": isinstance(value, str)}[kind]
        if not good:
            problems.append(f"{where}.{key}={value!r}（型別必須是 {kind}）")
            ok = False
    return ok


def _peaks_ok(problems: list[str], where: str, peaks: Any) -> bool:
    if not _closed(problems, where, peaks, _PEAKS_KEYS):
        return False
    ok = _typed(problems, where, peaks, {"dirs_peak": "nonneg", "fs_peak": "int", "samples": "pos"})
    by_loc = peaks["dirs_peak_by_location"]
    if not isinstance(by_loc, dict) or not all(isinstance(k, str) and _is_int(v) for k, v in by_loc.items()):
        problems.append(f"{where}.dirs_peak_by_location 必須是位置對整數的 object")
        ok = False
    return ok


def validate_acceptance_report_v2(report: Any, *, p_b_budget: int) -> list[str]:
    """⑦d 增補：報告 v2 的封閉 schema、衍生欄位的內部一致性、由列重新推導的門檻結果與 `status`。

    ⚠️ 每一節**先**驗完型別，型別不符就跳過那一節的交叉運算（實作第一輪 review）；任何未預期的例外也轉成問題——
    ⛔ 不拋例外、⛔ 不放行。⚠️ 驗的是**報告本身的一致性**；⛔ 不能驗證數字與原始量測（`raw/`）相符。`p_b_budget` 一律由
    呼叫端傳正式常數（`i074_stage2_preflight.P_B_BUDGET`），⛔ 不拿報告自己的值當基準。回傳問題清單（空 ＝ 有效）。
    """
    problems: list[str] = []
    try:
        _validate_report_v2(report, p_b_budget, problems)
    except Exception as exc:  # noqa: BLE001 - 後援：任何未預期的結構都是「不符」，⛔ 不是例外
        problems.append(f"驗證時發生例外（結構不符）：{type(exc).__name__}: {exc}")
    return problems


def _validate_report_v2(report: Any, p_b_budget: int, problems: list[str]) -> None:
    if not _closed(problems, "報告", report, REPORT_KEYS):
        return
    fixed = (("schema", REPORT_V2_SCHEMA), ("contract", REPORT_CONTRACT), ("out_of_contract", list(REPORT_OUT_OF_CONTRACT)),
             ("auto_reap_detection", REPORT_AUTO_REAP_DETECTION), ("memory_measures", REPORT_MEMORY_MEASURES))
    for key, want in fixed:
        if type(report[key]) is not type(want) or report[key] != want:
            problems.append(f"{key}={report[key]!r}（必須是固定值）")
    for key, allowed in (("status", ("ok", "threshold_exceeded")), ("mode", ("validation", "formal")),
                         ("replay_compute", ("stub", "full"))):
        if not isinstance(report[key], str) or report[key] not in allowed:
            problems.append(f"{key}={report[key]!r}（只能是 {allowed}）")
    for key in ("meta", "components"):
        if not isinstance(report[key], dict) or not all(isinstance(k, str) and isinstance(v, str)
                                                        for k, v in report[key].items()):
            problems.append(f"{key} 必須是字串對字串的 object")
    manifest = report["harness_manifest"]
    if not isinstance(manifest, dict) or not all(isinstance(v, str) and len(v) == 64 for v in manifest.values()):
        problems.append("harness_manifest 必須是路徑對 SHA-256 的 object")
    if not isinstance(report["harness_manifest_sha256"], str) or len(report["harness_manifest_sha256"]) != 64:
        problems.append("harness_manifest_sha256 必須是 SHA-256")
    if report["host_memavailable_low"] is not None and not _is_int(report["host_memavailable_low"]):
        problems.append("host_memavailable_low 必須是整數或 null")
    if not _str_list(report["notes"]):
        problems.append("notes 必須是字串陣列")
    # limits
    limits = report["limits"]
    limits_ok = _closed(problems, "limits", limits, _LIMITS_KEYS)
    if limits_ok:
        if not _is_int(limits["memory_bytes"]) or limits["memory_bytes"] != ACCEPTANCE_MEMORY_LIMIT:
            problems.append(f"limits.memory_bytes={limits['memory_bytes']!r} ≠ {ACCEPTANCE_MEMORY_LIMIT}")
        if not _is_int(limits["P_B_BUDGET"]) or limits["P_B_BUDGET"] != p_b_budget:
            problems.append(f"limits.P_B_BUDGET={limits['P_B_BUDGET']!r} ≠ 正式常數 {p_b_budget}")
        cml = limits["container_memory_limit_bytes"]
        if not isinstance(cml, list) or not all(_is_int(v) and v > 0 for v in cml):
            problems.append(f"limits.container_memory_limit_bytes={cml!r}（必須是正整數陣列）")
            limits_ok = False
    # paths
    paths, path_ok = report["paths"], {}
    if not isinstance(paths, dict) or set(paths) != {"success", "failure"}:
        problems.append("paths 必須恰好是 success、failure")
        paths = {}
    for phase, info in paths.items():
        where = f"paths.{phase}"
        if not _closed(problems, where, info, _PATH_KEYS):
            continue
        ok = _peaks_ok(problems, f"{where}.peaks", info["peaks"])
        ok = _typed(problems, where, info, {"accounted": "nonneg", "P_path": "nonneg", "read_only_gap": "nonneg",
                                            "container_rule": "str"}) and ok
        parts = info["accounted_parts"]
        if not isinstance(parts, dict) or not all(isinstance(k, str) and _is_int(v) for k, v in parts.items()):
            problems.append(f"{where}.accounted_parts 必須是名稱對整數的 object")
            ok = False
        if not ok:
            continue
        path_ok[phase] = True
        if info["accounted"] != sum(parts.values()):
            problems.append(f"{where}.accounted ≠ accounted_parts 的總和")
        if info["P_path"] != max(info["peaks"]["dirs_peak"], info["peaks"]["fs_peak"], info["accounted"]):
            problems.append(f"{where}.P_path ≠ max(dirs_peak, fs_peak, accounted)")
    # promotion
    promotion = report["promotion"]
    if not isinstance(promotion, dict) or set(promotion) != {"promote_success", "promote_failure"}:
        problems.append("promotion 必須恰好是 promote_success、promote_failure")
        promotion = {}
    for phase, info in promotion.items():
        where = f"promotion.{phase}"
        if not _closed(problems, where, info, _PROMOTION_KEYS):
            continue
        ok = _peaks_ok(problems, f"{where}.peaks", info["peaks"])
        ok = _typed(problems, where, info, {"accounted": "nonneg", "P_promotion": "nonneg",
                                            "P_path_plus_promotion": "nonneg", "kind": "str"}) and ok
        if isinstance(info["kind"], str) and info["kind"] != PROMOTION_KIND:
            problems.append(f"{where}.kind={info['kind']!r}")
        if not ok:
            continue
        if info["P_promotion"] != max(info["peaks"]["dirs_peak"], info["peaks"]["fs_peak"], info["accounted"]):
            problems.append(f"{where}.P_promotion ≠ max(dirs_peak, fs_peak, accounted)")
        base_phase = "success" if phase == "promote_success" else "failure"
        if path_ok.get(base_phase) and info["P_path_plus_promotion"] != paths[base_phase]["P_path"] + info["P_promotion"]:
            problems.append(f"{where}.P_path_plus_promotion ≠ 對應路徑的 P_path ＋ P_promotion")
    # memory 與 host 列
    memory, host = report["memory"], report["host"]
    rows_ok = True
    if not isinstance(memory, list) or not isinstance(host, list):
        problems.append("memory 與 host 必須是陣列")
        memory, host, rows_ok = [], [], False
    for i, row in enumerate(memory):
        where = f"memory[{i}]"
        if not _closed(problems, where, row, _MEMORY_ROW_KEYS):
            rows_ok = False
            continue
        if not _typed(problems, where, row, {
                "sequence": "pos", "phase": "str", "role": "str", "rc": "int", "max_single_rss_bytes": "pos",
                "self_max_rss_bytes": "pos", "children_max_rss_bytes": "pos", "rss_peak_sampled_bytes": "pos",
                "rss_samples": "pos", "reaper": "str", "cgroup_peak_bytes": "pos", "memory_limit_bytes": "pos",
                "below_limit": "bool"}):
            rows_ok = False
            continue
        if row["reaper"] != "pid1":
            problems.append(f"{where}.reaper={row['reaper']!r}（容器內必須是 pid1）")
        if row["max_single_rss_bytes"] != max(row["self_max_rss_bytes"], row["children_max_rss_bytes"]):
            problems.append(f"{where}.max_single_rss_bytes ≠ max(self, children)")
        if row["below_limit"] != (row["max_single_rss_bytes"] < ACCEPTANCE_MEMORY_LIMIT
                                  and row["rss_peak_sampled_bytes"] < ACCEPTANCE_MEMORY_LIMIT):
            problems.append(f"{where}.below_limit 與數值不符")
    for i, row in enumerate(host):
        where = f"host[{i}]"
        if not _closed(problems, where, row, _HOST_ROW_KEYS):
            continue
        if not _typed(problems, where, row, {
                "step": "str", "phase": "str", "rc": "int", "self_max_rss_bytes": "pos", "children_max_rss_bytes": "pos",
                "max_single_rss_bytes": "pos", "group_rss_peak_sampled_bytes": "pos", "group_alarm": "bool",
                "group_note": "str"}):
            continue
        if row["max_single_rss_bytes"] != max(row["self_max_rss_bytes"], row["children_max_rss_bytes"]):
            problems.append(f"{where}.max_single_rss_bytes ≠ max(self, children)")
        if row["group_alarm"] != (row["group_rss_peak_sampled_bytes"] >= ACCEPTANCE_MEMORY_LIMIT):
            problems.append(f"{where}.group_alarm 與數值不符")
        if row["group_note"] != GROUP_NOTES[row["group_alarm"]]:
            problems.append(f"{where}.group_note 與 group_alarm 不符")
    if limits_ok and rows_ok and limits["container_memory_limit_bytes"] != sorted({r["memory_limit_bytes"] for r in memory}):
        problems.append("limits.container_memory_limit_bytes 與記憶體列不符")
    # violations
    violations = report["violations"]
    if not isinstance(violations, list):
        problems.append("violations 必須是陣列")
    else:
        for i, v in enumerate(violations):
            where = f"violations[{i}]"
            if not _closed(problems, where, v, _VIOLATION_KEYS):
                continue
            if _typed(problems, where, v, {"kind": "str", "subject": "str", "value": "int", "limit": "int"}) \
                    and v["kind"] not in VIOLATION_KINDS:
                problems.append(f"{where}.kind={v['kind']!r}（不在封閉的 enum 裡）")
    if problems:
        return
    # ⚠️ 有效性條件計畫（第三輪 review）：v2 只在沒有任何有效性問題時寫出——只是收窄（既有的有效報告照樣通過）。
    validity = disk_validity_problems(acceptance_disk(paths), p_b_budget)
    if validity:
        problems.append(f"報告有有效性問題（v2 只在沒有任何有效性問題時寫出）：{validity_text(validity)}")
    if violations != derive_acceptance_violations(report):
        problems.append("violations 與由列重新推導的結果不符")
    if (report["status"] == "ok") != (not violations):
        problems.append(f"status={report['status']!r} 與 violations 是否為空不一致")


def acceptance_report_text(report: Mapping[str, Any]) -> str:
    mib = lambda b: f"{b / 1048576:.1f} MiB" if b is not None else "—"  # noqa: E731
    lines = [f"status: {report['status']}（mode={report['mode']}、replay_compute={report['replay_compute']}）",
             f"契約：{report['contract']}（契約外：{'、'.join(report['out_of_contract'])}）"]
    for v in report["violations"]:
        sign = "＞" if v["kind"] == "disk_p_path" else "≥"
        lines.append(f"  ⚠️ {v['kind']}：{v['subject']} {mib(v['value'])} {sign} {mib(v['limit'])}")
    lines += [f"門檻：記憶體 < {mib(report['limits']['memory_bytes'])}、磁碟 ≤ P_B_BUDGET {mib(report['limits']['P_B_BUDGET'])}", ""]
    for phase, info in report["paths"].items():
        pk, d = info["peaks"], disk_numbers(info)
        lines.append(f"[{phase}] P_path={mib(info['P_path'])}  dirs_peak={mib(pk['dirs_peak'])}  fs_peak={mib(pk['fs_peak'])}  "
                     f"accounted={mib(info['accounted'])}  read_only_gap={mib(info['read_only_gap'])}")
        lines.append(f"    P_basis={d['P_basis']}  fs_unexplained={d['fs_unexplained']} bytes（容差 {FS_UNEXPLAINED_TOLERANCE}）")
        for part, value in info["accounted_parts"].items():
            lines.append(f"    {part:<22} {mib(value)}")
    for phase, info in report["promotion"].items():
        lines.append(f"[{phase}]（資訊值）P_promotion={mib(info['P_promotion'])}  P_path＋晉升={mib(info['P_path_plus_promotion'])}")
    lines += ["", "容器（最大單一程序 RSS ＝ 精確閘；RSS 取樣 ＝ 單向偵測；cgroup ＝ 含 page cache 的資訊值；上限 ＝ 當次的 --memory）："]
    for row in report["memory"]:
        lines.append(f"  #{row['sequence']:<3} {row['phase']:<16} {row['role']:<10} rc={row['rc']}  "
                     f"最大單一 {mib(row['max_single_rss_bytes'])}  RSS 取樣 {mib(row['rss_peak_sampled_bytes'])}"
                     f"（{row['rss_samples']} 次）  cgroup {mib(row['cgroup_peak_bytes'])}／上限 {mib(row['memory_limit_bytes'])}")
    lines.append("host 端程序樹（RSS，⛔ 不含 page cache）：")
    for row in report["host"]:
        lines.append(f"  {row['step']:<30} 最大單一 {mib(row['max_single_rss_bytes'])}  群組取樣 "
                     f"{mib(row['group_rss_peak_sampled_bytes'])}（{row['group_note']}）")
    lines.append(f"host MemAvailable 低點：{mib(report['host_memavailable_low'])}")
    lines += [""] + [f"註：{n}" for n in report["notes"]]
    return "\n".join(lines) + "\n"


# ── offline 的讀取端：`precheck-verdict` 與 raw manifest（有效性條件計畫「二」的兩列） ───────────────────────────
#
# ⚠️ 信任模型（第三～七輪 review）：原始量測目錄（`raw/`、`raw-failed/`）裡的模組、`MANIFEST`、`identity` 與 `precheck.json`
#   只能證明彼此一致，⛔ 不是信任根。對外的命令（`precheck-verdict`、`raw-manifest`）只執行**固定的驗錨與取出 frontend**：
#   驗呼叫端給的 `--expected-repo-head`（外部的 commit 錨點，⛔ 不取自 artifact）→ 從受信任 repo 中**它的 git object** 取出
#   helper 與兩個常數模組 → 以 `python3 -I` 執行取出那一版的內部命令（`precheck-recompute`、`raw-manifest-recompute`）。
#   ⛔ 不呼叫工作樹的 recompute／產生演算法、⛔ 不執行或 import 原始量測目錄裡的任何程式。

def _require_dir(path: Path, label: str) -> None:
    try:
        st = os.lstat(path)
    except OSError as exc:
        raise SizingError(f"{label} 讀不到：{exc}") from exc
    if not stat.S_ISDIR(st.st_mode):
        raise SizingError(f"{label} 必須是目錄（⛔ 不接受 symlink）：{path}")


def verify_full_suffix(raw: Path) -> dict[str, Any]:
    """形狀 B 的量測趟 suffix 必須**恰好**是唯一合法的那一組（呼叫端已先以 collector（`complete`）驗過完整狀態的全部內容）：
    `rc.tsv` 最後一行 `replay_full`、sequence n＋1 是 full 模式正式集合的最後一個 invocation（索引、sidecar、twin 都在）、
    它的 lifecycle event 恰好 `create_begin`、`rm_done` 各一筆且先後正確、`host/replay_full.json` 存在。"""
    full = expected_invocations("acceptance", "full")
    sequence = len(full)
    cid = container_id(sequence)
    if list(_read_tsv(raw / "rc.tsv"))[-1:] != [ACCEPTANCE_FULL_STEP[0]]:
        raise SizingError("形狀 B：rc.tsv 的最後一行⛔ 不是 replay_full")
    index = read_json(raw / "index" / f"{sequence:04d}.json")
    if (index["phase"], index["role"]) != full[-1] or index["id"] != cid:
        raise SizingError(f"形狀 B：sequence {sequence} ⛔ 不是量測趟的 invocation")
    for folder in ("containers", "twins"):
        if not (raw / folder / f"{cid}.json").is_file():
            raise SizingError(f"形狀 B：缺量測趟的 {folder}/{cid}.json")
    created = load_events(raw).get((cid, "create_begin"), [])
    removed = load_events(raw).get((cid, "rm_done"), [])
    if len(created) != 1 or len(removed) != 1 or not created[0] < removed[0]:
        raise SizingError(f"形狀 B：量測趟的 lifecycle event 不完整或先後顛倒（create_begin {created}、rm_done {removed}）")
    if not (raw / "host" / f"{ACCEPTANCE_FULL_STEP[0]}.json").is_file():
        raise SizingError("形狀 B：缺 host/replay_full.json")
    return {"sequence": sequence, "id": cid, "step": ACCEPTANCE_FULL_STEP[0]}


def project_before_full(raw: Path, dest: Path, suffix: Mapping[str, Any]) -> None:
    """**明確地**投影掉驗過的 suffix：只複製 collector 讀的檔案，並且只拿掉 suffix 列舉的那幾項（每一項都必須恰好拿掉一次）。"""
    import shutil  # noqa: PLC0415

    cid, step, sequence = suffix["id"], suffix["step"], suffix["sequence"]
    for name in ("meta.tsv", "components.tsv", "memavail.tsv"):
        if (raw / name).is_file():
            shutil.copyfile(raw / name, dest / name)
    for folder in ("harness", "phases"):
        shutil.copytree(raw / folder, dest / folder, symlinks=True)
    removed: list[str] = []
    rc_lines = [line for line in (raw / "rc.tsv").read_text(encoding="utf-8").splitlines() if line.strip()]
    kept = [line for line in rc_lines if line.partition("\t")[0] != step]
    removed += ["rc"] * (len(rc_lines) - len(kept))
    (dest / "rc.tsv").write_text("".join(f"{line}\n" for line in kept), encoding="utf-8")
    for folder, skip in (("index", f"{sequence:04d}.json"), ("containers", f"{cid}.json"), ("twins", f"{cid}.json"),
                         ("host", f"{step}.json")):
        (dest / folder).mkdir()
        for path in sorted((raw / folder).iterdir()):
            if path.name == skip:
                removed.append(folder)
                continue
            shutil.copyfile(path, dest / folder / path.name)
    (dest / "events").mkdir()
    lines = [line for line in (raw / "events" / "events.jsonl").read_text(encoding="utf-8").splitlines() if line.strip()]
    kept = [line for line in lines if json.loads(line)["id"] != cid]
    removed += ["event"] * (len(lines) - len(kept))
    (dest / "events" / "events.jsonl").write_text("".join(f"{line}\n" for line in kept), encoding="utf-8")
    if sorted(removed) != sorted(["rc", "index", "containers", "twins", "host", "event", "event"]):
        raise SizingError(f"投影拿掉的項目⛔ 不恰好是 suffix：{sorted(removed)}")


def precheck_recompute(raw: Path, *, p_b_budget: int, drop_names: Iterable[str],
                       drop_prefixes: tuple[str, ...]) -> str:
    """`precheck.json` 的完整判讀（由受信任的版本執行）：封閉契約 → 身分綁定 → 形狀 A／B → 以 collector 重算、逐位元比對。
    回傳 `status`；任何一項不符 → `SizingError`（無法判讀）。"""
    import tempfile  # noqa: PLC0415

    raw = Path(raw)
    _require_dir(raw, "--raw")
    data = (raw / "precheck.json").read_bytes()
    doc = json.loads(data.decode("utf-8"))
    if canonical_dumps(doc) != data:
        raise SizingError("precheck.json ⛔ 不是 canonical JSON")
    problems = validate_acceptance_precheck_v1(doc, p_b_budget=p_b_budget)
    if problems:
        raise SizingError(f"precheck.json 的契約不符：{problems[:5]}")
    meta = _read_tsv(raw / "meta.tsv")
    for key in PRECHECK_IDENTITY_KEYS[:-1]:
        if doc["identity"][key] != meta.get(key):
            raise SizingError(f"身分綁定：identity.{key} ≠ 同一個目錄的 meta.tsv")
    _mapping, manifest_sha = load_harness_manifest(raw, "acceptance")
    if doc["identity"]["harness_manifest_sha256"] != manifest_sha:
        raise SizingError("身分綁定：identity.harness_manifest_sha256 ≠ 同一個目錄的 MANIFEST")
    prefix = [s for s, _ in ACCEPTANCE_STEPS]
    steps = list(_read_tsv(raw / "rc.tsv"))
    kwargs = {"p_b_budget": p_b_budget, "drop_names": drop_names, "drop_prefixes": drop_prefixes}
    if steps == prefix:                                              # 形狀 A：precheck 之後中止
        recomputed = build_acceptance_precheck(raw, **kwargs)
    elif steps == prefix + [ACCEPTANCE_FULL_STEP[0]]:                # 形狀 B：量測趟已執行（成功的 raw/）
        if doc["status"] != "ok":
            raise SizingError(f"形狀 B（量測趟已執行）但 precheck 的 status 是 {doc['status']!r}")
        collect_acceptance_measurements(raw, stage="complete", drop_names=drop_names, drop_prefixes=drop_prefixes)
        suffix = verify_full_suffix(raw)
        with tempfile.TemporaryDirectory(prefix="i074-precheck-") as tmp:
            project_before_full(raw, Path(tmp), suffix)
            recomputed = build_acceptance_precheck(Path(tmp), **kwargs)
    else:
        raise SizingError(f"形狀⛔ 不是 A（恰好前綴）也⛔ 不是 B（前綴 ＋ 量測趟）：{steps}")
    if canonical_dumps(recomputed) != data:
        raise SizingError("由原始量測重算的 precheck 與 precheck.json 不同（⛔ 不是逐位元相同）")
    return doc["status"]


GIT_BIN = "/usr/bin/git"          # ⚠️ 固定路徑——⛔ 不從 PATH 找（實作第一輪 review：PATH 上的假 git 能偽造錨點並回傳任意程式）
_EMPTY_HOME: list[str] = []


def _empty_home() -> str:
    """offline 子程序的空 `HOME`／`XDG_CONFIG_HOME`（⛔ 不讀使用者的 git 設定）；程序結束時刪除。"""
    if not _EMPTY_HOME:
        import atexit  # noqa: PLC0415
        import shutil  # noqa: PLC0415
        import tempfile  # noqa: PLC0415

        path = tempfile.mkdtemp(prefix="i074-offline-home-")
        atexit.register(shutil.rmtree, path, True)
        _EMPTY_HOME.append(path)
    return _EMPTY_HOME[0]


def offline_child_env(repo: str | Path | None = None) -> dict[str, str]:
    """offline 讀取端的子程序（git 與取出的 helper）的**最小化** allowlist 環境——⛔ 不繼承呼叫端的 `GIT_*`、`LD_*`、`PYTHON*`、
    `PATH` 等（實作第一輪 review：`GIT_DIR`／`GIT_OBJECT_DIRECTORY` 能讓 `--repo` 被忽略）。`GIT_NO_REPLACE_OBJECTS`：⛔ 不讓
    replace refs 換掉 object；`GIT_CEILING_DIRECTORIES`：`--repo` 必須是 repo 的根目錄，⛔ 不往上找到別的 repo。"""
    home = _empty_home()
    env = {"PATH": "/usr/bin:/bin", "HOME": home, "XDG_CONFIG_HOME": home, "LANG": "C.UTF-8", "LC_ALL": "C.UTF-8",
           "GIT_CONFIG_NOSYSTEM": "1", "GIT_NO_REPLACE_OBJECTS": "1", "GIT_TERMINAL_PROMPT": "0"}
    if repo is not None:
        env["GIT_CEILING_DIRECTORIES"] = str(Path(repo).resolve().parent)
    return env


def _git(repo: str | Path, *args: str, text: bool = False) -> subprocess.CompletedProcess:
    if not (os.path.isfile(GIT_BIN) and os.access(GIT_BIN, os.X_OK)):
        raise SizingError(f"找不到 {GIT_BIN}（offline 讀取端⛔ 不從 PATH 找 git）")
    return subprocess.run([GIT_BIN, "-C", str(repo), *args], capture_output=True, text=text, env=offline_child_env(repo))


def verify_anchor(repo: str | Path, expected: str) -> str:
    """外部的 commit 錨點（第四輪 review）：40 碼小寫 hex，且 `rev-parse --verify <expected>^{commit}` ＝ 它自己——⛔ 不接受
    ref、縮寫、tag 或不存在的 OID。"""
    if not isinstance(expected, str) or not _HEX40.fullmatch(expected):
        raise SizingError(f"--expected-repo-head 必須是 40 碼小寫 hex 的 commit OID（⛔ 不接受 ref、縮寫、tag）：{expected!r}")
    proc = _git(repo, "rev-parse", "--verify", "--quiet", f"{expected}^{{commit}}", text=True)
    if proc.returncode != 0 or proc.stdout.strip() != expected:
        raise SizingError(f"--expected-repo-head {expected} ⛔ 不是受信任 repo 裡的 commit")
    return expected


def git_blob(repo: str | Path, commit: str, rel: str) -> bytes:
    proc = _git(repo, "cat-file", "blob", f"{commit}:{rel}")
    if proc.returncode != 0:
        raise SizingError(f"受信任 repo 的 {commit} 沒有 {rel}")
    return proc.stdout


def run_trusted(repo: str | Path, expected: str, argv: list[str]) -> int:
    """兩個對外命令共用的 frontend：驗錨點 → 從**錨點的 git object** 取出 helper 與兩個常數模組到私有的暫存目錄 → 以
    `python3 -I` 執行取出那一版的內部命令。stdout／stderr 直接交給呼叫端。⛔ 不呼叫本地（工作樹）的 recompute。"""
    import tempfile  # noqa: PLC0415

    verify_anchor(repo, expected)
    with tempfile.TemporaryDirectory(prefix="i074-trusted-") as tmp:
        root = Path(tmp)
        for rel in (HELPER_REL, FROZEN_PREFLIGHT, FROZEN_SUPERVISOR):
            (root / rel).parent.mkdir(parents=True, exist_ok=True)
            (root / rel).write_bytes(git_blob(repo, expected, rel))
        return subprocess.run([sys.executable, "-I", str(root / HELPER_REL), *argv, "--trusted-root", str(root)],
                              env=offline_child_env()).returncode


def _manifest_rel(rel: str) -> str:
    parts = rel.split("/")
    if not rel or rel.startswith("/") or any(p in ("", ".", "..") for p in parts):
        raise SizingError(f"MANIFEST 的路徑不合法：{rel!r}")
    return rel


def precheck_verdict(raw: str | Path, repo: str | Path, expected: str) -> int:
    """`precheck-verdict` 的 frontend。⚠️ 原始量測目錄裡的東西**只當資料與 bytes**：`identity.repo_head` 必須 ＝ 外部錨點
    （**不符就在取出或執行任何程式之前拒絕**）、快照的每一個檔案逐位元 ＝ 錨點 commit 裡的同一路徑；之後才交給受信任的版本。"""
    verify_anchor(repo, expected)
    raw = Path(raw)
    _require_dir(raw, "--raw")
    doc = json.loads((raw / "precheck.json").read_bytes().decode("utf-8"))
    identity = doc.get("identity") if isinstance(doc, dict) else None
    head = identity.get("repo_head") if isinstance(identity, dict) else None
    if head != expected:
        raise SizingError(f"identity.repo_head {head!r} ≠ --expected-repo-head {expected}（在取出或執行任何程式之前拒絕）")
    seen: set[str] = set()
    for line in (raw / "harness" / "MANIFEST").read_text(encoding="utf-8").splitlines():
        _sha, sep, rel = line.partition("  ")
        if not sep or _manifest_rel(rel) in seen:
            raise SizingError(f"MANIFEST 的格式不符：{line!r}")
        seen.add(rel)
        path = raw / "harness" / rel
        if not stat.S_ISREG(os.lstat(path).st_mode) or path.read_bytes() != git_blob(repo, expected, rel):
            raise SizingError(f"快照的 {rel} ≠ {expected} 的內容（⛔ 不執行原始量測裡的程式）")
    return run_trusted(repo, expected, ["precheck-recompute", "--raw", str(raw)])


# raw manifest（`i074_stage2_raw_manifest_v1`）：原始量測的事後錨點（第五、六輪 review）。

RAW_MANIFEST_SCHEMA = "i074_stage2_raw_manifest_v1"
RAW_ROOT_NAMES = ("raw", "raw-failed")


def build_raw_manifest(raw: str | Path, expected_repo_head: str) -> bytes:
    """封閉的 canonical manifest：`lstat` 走訪、⛔ 不跟隨 symlink；一般檔案記 `path`、`size`、`sha256`，目錄（含空目錄、⛔ 不含
    根目錄）記路徑；symlink／FIFO／socket／裝置檔、非 UTF-8 的名稱 → 拒絕。兩個陣列各自依路徑的 UTF-8 bytes 排序。"""
    if not isinstance(expected_repo_head, str) or not _HEX40.fullmatch(expected_repo_head):
        raise SizingError(f"--expected-repo-head 必須是 40 碼小寫 hex：{expected_repo_head!r}")
    raw = Path(raw)
    _require_dir(raw, "--raw")
    if raw.name not in RAW_ROOT_NAMES:
        raise SizingError(f"--raw 的名稱只能是 {RAW_ROOT_NAMES}：{raw.name!r}")
    files: list[dict[str, Any]] = []
    dirs: list[str] = []

    def walk(directory: bytes, parts: list[str]) -> None:
        with os.scandir(directory) as it:
            entries = sorted(it, key=lambda e: e.name)
        for entry in entries:
            try:
                name = entry.name.decode("utf-8")
            except UnicodeDecodeError as exc:
                raise SizingError(f"名稱⛔ 不是合法的 UTF-8：{entry.path!r}") from exc
            if name in ("", ".", "..") or "/" in name:
                raise SizingError(f"名稱不合法：{name!r}")
            rel = "/".join([*parts, name])
            st = entry.stat(follow_symlinks=False)
            if stat.S_ISDIR(st.st_mode):
                dirs.append(rel)
                walk(entry.path, [*parts, name])
            elif stat.S_ISREG(st.st_mode):
                digest = hashlib.sha256()
                fd = os.open(entry.path, os.O_RDONLY | os.O_NOFOLLOW)
                with os.fdopen(fd, "rb") as fh:
                    for chunk in iter(lambda: fh.read(1 << 20), b""):
                        digest.update(chunk)
                files.append({"path": rel, "size": st.st_size, "sha256": digest.hexdigest()})
            else:
                raise SizingError(f"{rel}：symlink 或特殊檔案（⛔ 不產 manifest）")

    walk(os.fsencode(str(raw)), [])
    files.sort(key=lambda f: f["path"].encode("utf-8"))
    dirs.sort(key=lambda p: p.encode("utf-8"))
    if len({f["path"] for f in files}) != len(files) or len(set(dirs)) != len(dirs):
        raise SizingError("路徑重複")
    return canonical_dumps({"schema": RAW_MANIFEST_SCHEMA, "expected_repo_head": expected_repo_head, "root_name": raw.name,
                            "file_count": len(files), "dir_count": len(dirs), "files": files, "dirs": dirs})


def write_new_file(out: str | Path, data: bytes, *, outside: str | Path) -> None:
    """exclusive create（`O_CREAT | O_EXCL`：路徑已存在——含 symlink、懸空的 symlink——即失敗、⛔ 不覆寫）；父目錄必須已存在；
    解析之後落在 `outside` 之內 → 拒絕（避免自我雜湊）；寫入途中失敗 → 刪掉本次建立的那個檔案。"""
    out = Path(out)
    parent = os.path.realpath(out.parent)
    if not os.path.isdir(parent):
        raise SizingError(f"--out 的父目錄不存在：{out.parent}")
    target = os.path.join(parent, out.name)
    guard = os.path.realpath(outside)
    if os.path.commonpath([target, guard]) == guard:
        raise SizingError(f"--out 落在 --raw 之內（避免自我雜湊、⛔ 不覆寫原始量測）：{out}")
    fd = os.open(target, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o644)
    try:
        with os.fdopen(fd, "wb") as fh:
            fh.write(data)
            fh.flush()
            os.fsync(fh.fileno())
    except BaseException:
        os.unlink(target)
        raise


def raw_manifest_main(raw: str, expected: str, out: str | None, check: str | None) -> int:
    """`raw-manifest-recompute`（受信任的版本執行）：產生模式必須帶 `--out`；檢查模式先在記憶體重建、比對 SHA-256——
    ⛔ 不同 → rc＝1、⛔ 不寫任何檔案；相同且帶 `--out` 才以同樣的 exclusive 規則寫出。"""
    if check is None and out is None:
        raise SizingError("產生模式必須帶 --out")
    if check is not None and not _HEX64.fullmatch(check):
        raise SizingError(f"--check-sha256 必須是 64 碼小寫 hex：{check!r}")
    data = build_raw_manifest(raw, expected)
    digest = hashlib.sha256(data).hexdigest()
    doc = json.loads(data)
    summary = {"sha256": digest, "file_count": doc["file_count"], "dir_count": doc["dir_count"],
               "expected_repo_head": expected, "root_name": doc["root_name"]}
    if check is not None and digest != check:
        print(f"ERROR: raw manifest 的 SHA-256 {digest} ≠ 記下的 {check}（⛔ 不寫任何檔案）", file=sys.stderr)
        return 1
    if out is not None:
        write_new_file(out, data, outside=raw)
    print(json.dumps(summary, sort_keys=True))
    return 0


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
                                    "first_seen": now, "gone_at": None, "peak_bytes": None, "reads": 0, "last_read": None,
                                    "rss_peak_bytes": None, "rss_reads": 0}
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
            # ⑦d 增補「二」⑤：v1 memory.stat 的 total_rss（取樣的下界；⛔ 不讀 v2 的 anon）
            try:
                stat = (Path(self.cgroup_root) / "memory" / "docker" / info["id"] / "memory.stat").read_text()
                rss = next(int(line.split()[1]) for line in stat.splitlines() if line.startswith("total_rss "))
            except (OSError, ValueError, IndexError, StopIteration):
                continue
            info["rss_peak_bytes"] = rss if info["rss_peak_bytes"] is None else max(info["rss_peak_bytes"], rss)
            info["rss_reads"] += 1

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
        rss_unavailable = sorted(c["id"] for c in self.containers.values() if c["rss_peak_bytes"] is None)
        complete = (seen["replay"] and not missing and not unavailable and not rss_unavailable
                    and bool(self.first_poll_empty) and not self.last_poll_ids)
        return {
            "schema": "i074_stage2_observation_v1",
            "label_key": OBSERVE_LABEL_KEY,
            "observation_complete": complete,
            "replay_seen": seen["replay"],
            "missing_expected_containers": missing,
            "unavailable_containers": unavailable,
            "rss_unavailable_containers": rss_unavailable,
            "started_before_first_container": bool(self.first_poll_empty),
            "stopped_after_last_container": not self.last_poll_ids,
            "containers": [dict(c, peak_kind="observed_lower_bound") for c in
                           sorted(self.containers.values(), key=lambda c: c["first_seen"])],
            "fs_peak_delta_bytes": {"value": self.fs_peak_delta, "kind": "sampled"},
            "work_dir_peak_allocated_bytes": {"value": self.work_peak, "kind": "sampled"},
            "host_memavailable_low": {"value": self.memavail_low, "kind": "sampled"},
            "polls": self.polls,
            "notes": ["記憶體是 cgroup high-water mark 的**下界**：最後一次讀取之後、容器結束之前的尾段峰值可能漏記（run-evaluation.sh 的教訓）",
                      "⑦d 增補：rss_peak_bytes 是 cgroup v1 memory.stat 的 total_rss 的取樣最大值（不含 page cache；"
                      "與 acceptance 的單向偵測器同一種量，⛔ 不是證明）；讀不到 → rss_unavailable_containers、⛔ 不完整",
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
             f"missing={report['missing_expected_containers']}、unavailable={report['unavailable_containers']}、"
             f"rss_unavailable={report['rss_unavailable_containers']}）",
             "記憶體（cgroup high-water mark 的下界；total_rss 是取樣的最大值）："]
    for c in report["containers"]:
        lines.append(f"  {c['role']:<18} {c['id'][:12]}  {mib(c['peak_bytes'])}（讀 {c['reads']} 次）  "
                     f"total_rss {mib(c['rss_peak_bytes'])}（讀 {c['rss_reads']} 次）")
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
    for name in ("--state", "--clone", "--json-out", "--text-out"):
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
    p = sub.add_parser("reap-adopted")
    p.add_argument("--state", required=True)
    p.add_argument("--parent", required=True, type=int)
    p.add_argument("--parent-starttime", required=True, type=int)
    p.add_argument("--exclude", action="append", type=int, default=[])
    p.add_argument("--check-only", action="store_true")
    p = sub.add_parser("kill-pinned")
    p.add_argument("--pid", required=True, type=int)
    p.add_argument("--starttime", required=True, type=int)
    p = sub.add_parser("guard-identity")
    p.add_argument("--state", required=True)
    p.add_argument("--shell-pid", required=True, type=int)
    p = sub.add_parser("rss-capability")
    p.add_argument("--file", required=True)
    p = sub.add_parser("check-step")
    for name in ("--state", "--step"):
        p.add_argument(name, required=True)
    p = sub.add_parser("acceptance-report")
    for name in ("--state", "--clone", "--json-out", "--text-out"):
        p.add_argument(name, required=True)
    # ── 有效性條件計畫：⑨-1 的 fail-fast 與 offline 的讀取端 ──
    p = sub.add_parser("acceptance-precheck")
    for name in ("--state", "--clone"):
        p.add_argument(name, required=True)
    p = sub.add_parser("precheck-verdict")                 # 對外：只執行驗錨與取出的 frontend
    for name in ("--raw", "--repo", "--expected-repo-head"):
        p.add_argument(name, required=True)
    p = sub.add_parser("precheck-recompute")               # 內部：只由取出的受信任版本執行
    for name in ("--raw", "--trusted-root"):
        p.add_argument(name, required=True)
    p = sub.add_parser("raw-manifest")                     # 對外：只執行驗錨與取出的 frontend
    for name in ("--raw", "--repo", "--expected-repo-head"):
        p.add_argument(name, required=True)
    p.add_argument("--out")
    p.add_argument("--check-sha256")
    p = sub.add_parser("raw-manifest-recompute")           # 內部：只由取出的受信任版本執行
    for name in ("--raw", "--expected-repo-head", "--trusted-root"):
        p.add_argument(name, required=True)
    p.add_argument("--out")
    p.add_argument("--check-sha256")
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
    if cmd == "reap-adopted":
        return reap_adopted(Path(args.state), parent=args.parent, parent_starttime=args.parent_starttime,
                            exclude=args.exclude, check_only=args.check_only)
    if cmd == "kill-pinned":
        rc, record = kill_pinned(args.pid, args.starttime)
        sys.stdout.write(canonical_dumps(record).decode("utf-8") + "\n")     # 恰好一行（呼叫端附加到 kill-pinned.jsonl）
        return rc
    if cmd == "guard-identity":
        pid, starttime = guard_identity(Path(args.state), args.shell_pid)
        print(f"{pid} {starttime}")
        return 0
    if cmd == "rss-capability":
        try:
            record = json.loads(Path(args.file).read_text(encoding="utf-8"))
        except (OSError, ValueError) as exc:
            print(f"ERROR: 能力檢查的 rss.json 讀不到：{type(exc).__name__}: {exc}", file=sys.stderr)
            return 1
        problems = rss_record_problems(record, require_pid1=True)
        for problem in problems:
            print(f"ERROR: 能力檢查：{problem}", file=sys.stderr)
        return 1 if problems else 0
    if cmd == "check-step":
        problems = check_step(Path(args.state), args.step)
        for problem in problems:
            print(f"ERROR: {args.step}：{problem}", file=sys.stderr)
        return 1 if problems else 0
    if cmd == "acceptance-report":
        # ⚠️ 有效性條件計畫「二」：權威常數取自 S 的快照（並斷言與工作複本逐位元相同）。
        report = build_acceptance_report(Path(args.state), **live_constants(Path(args.state), args.clone, "acceptance"))
        Path(args.json_out).write_bytes(canonical_dumps(report))
        Path(args.text_out).write_text(acceptance_report_text(report), encoding="utf-8")
        print(acceptance_report_text(report), end="", file=sys.stderr)
        return 0 if report["status"] == "ok" else 2
    if cmd == "acceptance-precheck":
        state = Path(args.state)
        doc = build_acceptance_precheck(state, **live_constants(state, args.clone, "acceptance"))
        write_exclusive(state / "precheck.json", canonical_dumps(doc))
        print(precheck_text(doc), end="", file=sys.stderr)
        return PRECHECK_EXIT[doc["status"]]
    if cmd == "precheck-verdict":
        try:
            return precheck_verdict(args.raw, args.repo, args.expected_repo_head)
        except (SizingError, OSError, ValueError, KeyError, TypeError) as exc:
            print(f"ERROR: 無法判讀：{type(exc).__name__}: {exc}", file=sys.stderr)
            return 1
    if cmd == "precheck-recompute":
        root = _trusted_root(args.trusted_root)
        pf = _load_module("i074_stage2_preflight_trusted", root / FROZEN_PREFLIGHT)
        sup = _load_module("i074_stage2_supervisor_trusted", root / FROZEN_SUPERVISOR)
        try:
            status = precheck_recompute(Path(args.raw), p_b_budget=pf.P_B_BUDGET, drop_names=tuple(sup.ENV_DROP_NAMES),
                                        drop_prefixes=tuple(sup.ENV_DROP_PREFIXES))
        except (SizingError, OSError, ValueError, KeyError, TypeError) as exc:
            print(f"ERROR: 無法判讀：{type(exc).__name__}: {exc}", file=sys.stderr)
            return 1
        print(status)
        return 0
    if cmd == "raw-manifest":
        if args.check_sha256 is None and args.out is None:
            raise SizingError("產生模式必須帶 --out")
        argv = ["raw-manifest-recompute", "--raw", args.raw, "--expected-repo-head", args.expected_repo_head]
        argv += ["--out", args.out] if args.out is not None else []
        argv += ["--check-sha256", args.check_sha256] if args.check_sha256 is not None else []
        return run_trusted(args.repo, args.expected_repo_head, argv)
    if cmd == "raw-manifest-recompute":
        _trusted_root(args.trusted_root)
        return raw_manifest_main(args.raw, args.expected_repo_head, args.out, args.check_sha256)
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
        # ⚠️ 有效性條件計畫「二」：P_B_BUDGET 取自 S 的快照（並斷言與工作複本逐位元相同）。
        budget = live_constants(state, args.clone, "sizing")["p_b_budget"]
        report = build_report(state, p_b_budget=budget)
        text = report_text(report, validity_problems=sizing_validity_problems(report, budget))
        Path(args.json_out).write_bytes(canonical_dumps(report))
        Path(args.text_out).write_text(text, encoding="utf-8")
        print(text, end="", file=sys.stderr)
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
                   # ⑦d 增補：guard 的 kill-pinned 紀錄與收不掉的 host 程序（有才列）
                   "kill_pinned_log": "raw-failed/kill-pinned.jsonl" if (state / "kill-pinned.jsonl").is_file() else None,
                   "leftover_pids": "raw-failed/leftover-pids.json" if (state / "leftover-pids.json").is_file() else None,
                   # 有效性條件計畫：⑨-1 的 precheck（判讀一律經 precheck-verdict，⛔ 不憑這個欄位）
                   "precheck": "raw-failed/precheck.json" if (state / "precheck.json").is_file() else None,
                   "note": "⛔ 本檔不宣稱 P_B；原始量測見 raw-failed/"}
        Path(args.out).write_bytes(canonical_dumps(summary))
        return 0
    raise SizingError(f"未知的子指令：{cmd}")


def _trusted_root(root: str) -> Path:
    """內部命令（`precheck-recompute`、`raw-manifest-recompute`）只能由 frontend 從錨點 commit 取出的那一版執行。"""
    path = Path(root).resolve()
    if Path(__file__).resolve() != (path / HELPER_REL).resolve():
        raise SizingError("內部命令只能由 frontend 取出的受信任版本執行（⛔ 不得直接呼叫本地的 recompute）")
    return path


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
