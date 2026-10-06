#!/usr/bin/env python3
"""I-074 Stage 2 ⑦d 增補：acceptance profile 的容器內 wrapper（issue.md I-074「Stage 2 步驟 ⑦d 增補計畫：容器記憶體改以 RSS 判定」「二」①②）。

    python -I /acceptance/rss_wrapper.py <原指令逐 token>        # 容器的 PID 1（shim 掛載快照裡的本檔）

* **精確的閘**：leader 結束後以 `waitpid(-1, WNOHANG)` 收掉其餘子孫直到 `ECHILD`（寬限 10 秒），再讀
  `max(RUSAGE_SELF, RUSAGE_CHILDREN).ru_maxrss`——涵蓋 wrapper 自己、leader 與經正常 wait 鏈保存 resource usage 的子孫
  （PID 1／subreaper 收養的孤兒也由本程序收掉）。⚠️ 被 kernel 自動回收的子孫（parent 把 SIGCHLD 設成 SIG_IGN 或用
  SA_NOCLDWAIT）⛔ 不在範圍內——那是契約外，SIG_IGN 由下面的偵測 fail-closed。
* **單向偵測器**：背景 thread 每 50 ms 讀 cgroup v1 `memory.stat` 的 `total_rss`（⛔ 不改讀 v2），並掃子孫的 `SigIgn`。
* **I/O 透明**：本程序⛔ 不寫 stdout／stderr；結果與錯誤只寫 `<peak dir>/rss.json`（封閉 schema）與 `<peak dir>/peak`
  （含 page cache 的 cgroup 峰值，資訊值，格式與 sizing 的 wrapper 相同）。
* **結束碼**：leader 的結束碼原樣傳回；被訊號結束 → 128 ＋ 訊號編號（與 `sh` 相同）；量測失敗⛔ 不改結束碼。

`SIZING_CGROUP_ROOT`／`SIZING_PEAK_DIR` 只給 host 上的測試覆寫；寬限、間隔與 `/proc` 的位置是 `run()` 的參數（⛔ 不開環境變數）。
⚠️ 3.9 相容（host 的測試用 python3）。
"""
from __future__ import annotations

import ctypes
import json
import os
import resource
import signal
import subprocess
import sys
import threading
import time
from pathlib import Path

SCHEMA = "i074_stage2_rss_v1"
RSS_SOURCE = "v1:total_rss"
INTERVAL_MS = 50
REAP_GRACE_S = 10.0
PR_SET_CHILD_SUBREAPER = 36
SIGCHLD_MASK = 1 << (int(signal.SIGCHLD) - 1)      # /proc/<pid>/status 的 SigIgn：第 (signum − 1) 個位元
RECORD_KEYS = ("schema", "rss_source", "rss_interval_ms", "rss_samples", "rss_peak_sampled_bytes", "self_max_rss_bytes",
               "children_max_rss_bytes", "max_single_rss_bytes", "reaper", "all_descendants_reaped",
               "auto_reap_detected", "errors")


def read_total_rss(cgroup_root: Path) -> int:
    """cgroup v1 的 `total_rss`（匿名頁 ＋ swap cache）。⚠️ 只認 v1——讀不到就拋例外，⛔ 不改讀 v2 的 `anon`。"""
    for line in (cgroup_root / "memory" / "memory.stat").read_text().splitlines():
        key, _, value = line.partition(" ")
        if key == "total_rss":
            return int(value)
    raise ValueError("memory.stat 沒有 total_rss")


def proc_parents(proc_root: Path) -> dict[int, int]:
    """pid → ppid（讀不到、程序已消失的略過）。"""
    parents: dict[int, int] = {}
    for entry in proc_root.iterdir():
        if not entry.name.isdigit():
            continue
        try:
            parents[int(entry.name)] = int((entry / "stat").read_text().rsplit(")", 1)[1].split()[1])
        except (OSError, IndexError, ValueError):
            continue
    return parents


def descendants(root_pid: int, parents: dict[int, int]) -> set[int]:
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


def sigchld_ignored(proc_root: Path, pid: int) -> bool:
    try:
        for line in (proc_root / str(pid) / "status").read_text().splitlines():
            if line.startswith("SigIgn:"):
                return bool(int(line.split()[1], 16) & SIGCHLD_MASK)
    except (OSError, IndexError, ValueError):
        return False                       # 程序已消失（取樣的競態）
    return False


class Sampler(threading.Thread):
    """單向偵測器：`total_rss` 的取樣峰值與子孫的 `SigIgn`（都是取樣，⛔ 不是證明）。"""

    def __init__(self, cgroup_root: Path, proc_root: Path, interval_ms: int) -> None:
        super().__init__(daemon=True)
        self.cgroup_root, self.proc_root, self.interval = cgroup_root, proc_root, interval_ms / 1000
        self.peak, self.samples, self.auto_reap, self.errors = 0, 0, False, []
        self._halt = threading.Event()

    def sample(self) -> None:
        try:
            self.peak = max(self.peak, read_total_rss(self.cgroup_root))
            self.samples += 1
        except (OSError, ValueError) as exc:
            message = f"讀不到 cgroup v1 的 total_rss：{type(exc).__name__}: {exc}"
            if message not in self.errors:
                self.errors.append(message)
        try:
            me = os.getpid()
            if any(sigchld_ignored(self.proc_root, pid) for pid in descendants(me, proc_parents(self.proc_root))):
                self.auto_reap = True
        except OSError as exc:
            message = f"掃描 /proc 失敗：{exc}"
            if message not in self.errors:
                self.errors.append(message)

    def run(self) -> None:
        while not self._halt.is_set():
            self.sample()
            self._halt.wait(self.interval)

    def stop(self) -> None:
        self._halt.set()


def become_reaper() -> tuple[str, str | None]:
    """PID 1 → 本來就收養孤兒；否則設成 subreaper（失敗 → 回傳錯誤）。"""
    if os.getpid() == 1:
        return "pid1", None
    libc = ctypes.CDLL(None, use_errno=True)
    if libc.prctl(PR_SET_CHILD_SUBREAPER, 1, 0, 0, 0) != 0:
        return "subreaper", f"prctl(PR_SET_CHILD_SUBREAPER) 失敗：errno {ctypes.get_errno()}"
    return "subreaper", None


def reap_all(grace_s: float) -> bool:
    """收掉其餘的子孫，直到 `ECHILD`；寬限內收斂不了 → False（⛔ 不殺它們）。"""
    deadline = time.monotonic() + grace_s
    while True:
        try:
            pid, _status = os.waitpid(-1, os.WNOHANG)
        except ChildProcessError:
            return True
        if pid == 0:
            if time.monotonic() >= deadline:
                return False
            time.sleep(0.05)


def cgroup_peak_text(cgroup_root: Path) -> str:
    """與 sizing 的 wrapper 相同的候選路徑與順序：v1 → v2；讀不到 → 空字串。"""
    for path in (cgroup_root / "memory" / "memory.max_usage_in_bytes", cgroup_root / "memory.peak"):
        try:
            return path.read_text().strip()
        except OSError:
            continue
    return ""


def _write_atomic(path: Path, data: bytes) -> None:
    tmp = path.with_name(f".{path.name}.tmp")
    tmp.write_bytes(data)
    os.replace(tmp, path)


def run(argv: list[str], *, cgroup_root: str | Path = "/sys/fs/cgroup", peak_dir: str | Path = "/peak",
        proc_root: str | Path = "/proc", interval_ms: int = INTERVAL_MS, grace_s: float = REAP_GRACE_S) -> int:
    cgroup_root, peak_dir, proc_root = Path(cgroup_root), Path(peak_dir), Path(proc_root)
    errors: list[str] = []
    reaper, error = become_reaper()
    if error:
        errors.append(error)
    sampler = Sampler(cgroup_root, proc_root, interval_ms)
    sampler.start()
    try:
        leader = subprocess.Popen(argv)
    except OSError as exc:
        errors.append(f"leader 啟動失敗：{exc}")
        rc = 126 if isinstance(exc, PermissionError) else 127
    else:
        status = leader.wait()
        rc = 128 - status if status < 0 else status
    reaped = reap_all(grace_s)
    if not reaped:
        errors.append(f"leader 結束後 {grace_s:g} 秒內仍有存活的子孫（high-water 讀不到）")
    sampler.stop()
    sampler.join()
    sampler.sample()                       # 收斂之後再讀最後一次
    errors += sampler.errors
    self_rss = resource.getrusage(resource.RUSAGE_SELF).ru_maxrss * 1024
    children_rss = resource.getrusage(resource.RUSAGE_CHILDREN).ru_maxrss * 1024
    record = {
        "schema": SCHEMA, "rss_source": RSS_SOURCE, "rss_interval_ms": interval_ms, "rss_samples": sampler.samples,
        "rss_peak_sampled_bytes": sampler.peak, "self_max_rss_bytes": self_rss, "children_max_rss_bytes": children_rss,
        "max_single_rss_bytes": max(self_rss, children_rss), "reaper": reaper, "all_descendants_reaped": reaped,
        "auto_reap_detected": sampler.auto_reap, "errors": errors,
    }
    try:
        _write_atomic(peak_dir / "peak", (cgroup_peak_text(cgroup_root) + "\n").encode())
        _write_atomic(peak_dir / "rss.json",
                      json.dumps(record, sort_keys=True, ensure_ascii=False, separators=(",", ":")).encode() + b"\n")
    except OSError:
        pass                               # 寫不了 → sidecar 讀不到 rss.json → 量測失敗（⛔ 不寫 stdout／stderr）
    return rc


def main() -> int:
    if len(sys.argv) < 2:
        return 125                         # ⛔ 不寫 stderr（I/O 透明）；shim 一定會給原指令
    return run(sys.argv[1:], cgroup_root=os.environ.get("SIZING_CGROUP_ROOT", "/sys/fs/cgroup"),
               peak_dir=os.environ.get("SIZING_PEAK_DIR", "/peak"))


if __name__ == "__main__":
    sys.exit(main())
