#!/usr/bin/env python3
"""I-074 Stage 2 步驟 ④：sizing harness 的 Python 段。

規格見 issue.md I-074「Stage 2 步驟 ④：sizing harness 計畫書」（v7 ＋ 差異 1，✅ 已確認）。
由 `scripts/i074-stage2-sizing.sh` 與 `scripts/lib/i074-sizing-docker-shim.sh` 呼叫；⛔ 不是正式的證據入口。

子指令分兩類：

* **容器內**（Stage 2 image、唯讀 rootfs）：`fixture`——合成三條路徑的 operational 輸出；
  全量檔用本檔的**串流 writer**（差異 1），小檔用正式的 `publish_artifacts()`。
* **host**（只用標準庫；⛔ 不 import `backtest.*`——host 沒有 pandas）：量測（`allocated`、`sample`、
  `baseline`、`phase-end`）、shim 的簿記（`seq`、`index`、`event`、`sidecar`）、`twins`、`record-diff`、
  `report`、`failure-summary`，以及 `spec-hash`／`logbound` 這兩個純函式的 CLI。

⚠️ **單位**：落在檔案系統上的一律是 **allocated bytes**（`st_blocks × 512`，目錄一併計入，hard link 只算一次）；
內容長度只用在讀不到的 docker log 與容器 metadata 的保守推導，且以 block 上捨。
"""
from __future__ import annotations

import argparse
import fcntl
import hashlib
import json
import os
import secrets
import stat
import subprocess
import sys
import time
from pathlib import Path
from typing import Any, Iterable, Iterator, Mapping

# ── 常數（⚠️ 與計畫書逐項對應，⛔ 不開執行期參數） ─────────────────────────────

BLOCK = 4096                        # log／metadata 推導值的上捨單位
LOG_PIECE = 16 * 1024               # json-file 的長行分段
LOG_ESCAPE_FACTOR = 6               # JSON 逐 byte 跳脫的最壞倍數（\u00XX）
LOG_ENTRY_OVERHEAD = 128            # {"log":…,"stream":…,"time":…}\n 的固定開銷（實際約 70）
META_ROUND = 64 * 1024
META_START_MARGIN = 64 * 1024       # 啟動時才建立的 hosts／hostname／resolv.conf 與目錄項
TWIN_COUNT = 3
STREAM_CHUNK = 64 * 1024            # 與 artifacts.write_canonical_atomic() 相同

PHASES = ("witness", "success", "failure", "memory_only")
DISK_PHASES = ("witness", "success", "failure")
ROLES = ("fixture", "finalizer", "recovery", "check")
# ⚠️ 每條路徑預期的 invocation（依 sequence 順序）——寫死，⛔ 由 harness 以外的地方推論。
EXPECTED_ROLES = {
    "witness": ("fixture", "finalizer"),
    "success": ("fixture", "finalizer"),
    "failure": ("fixture", "finalizer"),
    "memory_only": ("recovery", "recovery", "check", "recovery"),
}
LOCATIONS = ("L1", "L2", "L3", "L5")
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


def write_index(state: Path, *, sequence: int, cid: str, phase: str, role: str, included: bool,
                image: str, argv: list[str]) -> dict[str, Any]:
    if phase not in PHASES:
        raise SizingError(f"未知的 phase：{phase!r}")
    if role not in ROLES:
        raise SizingError(f"未知的 role：{role!r}")
    if included != (phase in DISK_PHASES):
        raise SizingError(f"included_in_disk_path={included} 與 phase={phase} 不相符")
    entry = {
        "sequence": sequence, "id": cid, "phase": phase, "role": role, "included_in_disk_path": included,
        "container_spec_sha256": spec_sha256(argv, image), "image": image, "argv": argv,
        "sidecar_path": str(state / "containers" / f"{cid}.json"),
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
                  failures: list[str]) -> dict[str, Any]:
    """shim 的計量結果。⚠️ 任何一項讀不到都記成「量測失敗」，⛔ 不寫成 0。"""
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
    entry = {
        "id": cid, "sequence": sequence, "container_spec_sha256": index["container_spec_sha256"],
        "rc": rc, "container": container, "size_rw": size_rw_value, "log_config": log_config,
        "log_bound": bound, "peak_bytes": peak,
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

def _fixture_prov(cache: Path, patch: Path) -> dict[str, Any]:
    """before 側的 provenance：沿用見證趟 after' 的（新 image、`e1cbbbd`），tooling ＝ 合成 hash。"""
    from backtest.modular.sr_scoring.replay_bundle.stream import stream_canonical_artifact

    load = stream_canonical_artifact(cache / "envcheck" / "witness" / "after_artifact.json.gz",
                                     "sr_zone_replay_after", on_row=lambda r, b: None)
    return dict(load.top["provenance"], tooling_patch_sha256=hashlib.sha256(patch.read_bytes()).hexdigest(),
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


def fixture_success(python_root: Path, cache: Path, out: Path, patch: Path) -> dict[str, Any]:
    from backtest.modular.sr_scoring.replay_bundle import stage2_archive as sa
    from backtest.modular.sr_scoring.replay_bundle import stage2_evidence as s2
    from backtest.modular.sr_scoring.replay_bundle.artifacts import (
        build_comparison_artifact, build_report, compare_rows, prepare_output_dir, publish_artifacts, row_key)
    anchor = s2.load_stage1_anchor(python_root)
    prov = _fixture_prov(cache, patch)
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


def fixture_failure(python_root: Path, cache: Path, out: Path, patch: Path) -> dict[str, Any]:
    from backtest.modular.sr_scoring.replay_bundle import stage2_archive as sa
    from backtest.modular.sr_scoring.replay_bundle import stage2_evidence as s2
    from backtest.modular.sr_scoring.replay_bundle.artifacts import prepare_output_dir, publish_artifacts

    anchor = s2.load_stage1_anchor(python_root)
    check = sa.CounterfactualEffectCheck()
    for key in anchor.cohort_keys:            # RR 沒加回去：156 列候選原封不動 → rr_not_restored
        check.feed(anchor.cohort_rows[key])
    payload = sa.build_counterfactual_failure(bundle_id=anchor.bundle_id, generated_at="2026-09-24T13:00:00+08:00",
                                              provenance=_fixture_prov(cache, patch), effect=check.result())
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


def load_invocations(state: Path) -> list[dict[str, Any]]:
    """索引、sidecar、twin 三者的完整性（計畫書「二」的 invocation 索引）。"""
    indexes = sorted((state / "index").glob("*.json"))
    entries = [read_json(p) for p in indexes]
    sequences = [e["sequence"] for e in entries]
    if sequences != list(range(1, len(entries) + 1)):
        raise SizingError(f"invocation 的 sequence 不連續或重複：{sequences}")
    expected: list[tuple[str, str]] = [(phase, role) for phase in PHASES for role in EXPECTED_ROLES[phase]]
    got = [(e["phase"], e["role"]) for e in entries]
    if got != expected:
        raise SizingError(f"invocation 的 phase／role 與預期不符：預期 {expected}，實際 {got}")
    sidecars = {p.stem for p in (state / "containers").glob("*.json")}
    twins = {p.stem for p in (state / "twins").glob("*.json")}
    for e in entries:
        if e["included_in_disk_path"] != (e["phase"] in DISK_PHASES):
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
        if e["sidecar"]["size_rw"] != 0:
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
        per = {loc: max(s[loc] - baseline["locations"][loc], 0) for loc in LOCATIONS}
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
    for key in ("run_id", "mode", "image", "l0_dev", "docker_root_dev", "repo_dev", "state_fstype"):
        if not meta.get(key):
            raise SizingError(f"meta 缺少 {key}")
    if meta["state_fstype"] != "tmpfs":
        raise SizingError(f"S 不是 tmpfs：{meta['state_fstype']}")
    for key in ("docker_root_dev", "repo_dev"):
        if meta[key] != meta["l0_dev"]:
            raise SizingError(f"{key}={meta[key]} 與 L0 的 st_dev={meta['l0_dev']} 不同——⛔ 單一檔案系統的前提不成立")
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
        "index_head", "index_base", "index_base_patched", "snapshot", "probe")}
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


# ── CLI ───────────────────────────────────────────────────────────────────────

def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(prog="i074_stage2_sizing", allow_abbrev=False)
    sub = parser.add_subparsers(dest="cmd", required=True)

    p = sub.add_parser("fixture")
    p.add_argument("--phase", required=True, choices=DISK_PHASES)
    p.add_argument("--python-root", required=True)
    p.add_argument("--cache", required=True)
    p.add_argument("--out", required=True)
    p.add_argument("--patch")

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
    p.add_argument("argv", nargs=argparse.REMAINDER)
    p = sub.add_parser("event")
    for name in ("--state", "--id", "--kind"):
        p.add_argument(name, required=True)
    p = sub.add_parser("sidecar")
    for name in ("--state", "--sequence", "--rc", "--container", "--size-rw", "--log-config", "--stdout-log",
                 "--stderr-log", "--peak-file"):
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
            result = fixture_success(py, cache, out, Path(args.patch))
        else:
            result = fixture_failure(py, cache, out, Path(args.patch))
        print(json.dumps({"phase": args.phase, **result}, sort_keys=True))
        return 0
    if cmd == "anchor-base":
        # host：經 `_i074_bootstrap` 載入 dependency-light 的模組，用**已驗證的** Stage 1 信任錨取 base。
        sys.path.insert(0, str(Path(__file__).resolve().parent))
        from _i074_bootstrap import STAGE2_MODULES, load_replay_bundle

        root = Path(args.python_root)
        mods = load_replay_bundle(root, STAGE2_MODULES)
        print(mods["stage2_evidence"].load_stage1_anchor(root).after_base_commit)
        return 0
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
                    included=args.included == "true", image=args.image, argv=argv)
        return 0
    if cmd == "event":
        record_event(state, args.id, args.kind)
        return 0
    if cmd == "sidecar":
        sequence = int(args.sequence)
        write_sidecar(state, cid=container_id(sequence), sequence=sequence, rc=int(args.rc),
                      container=args.container, size_rw=args.size_rw, log_config_json=args.log_config,
                      stdout_log=Path(args.stdout_log), stderr_log=Path(args.stderr_log),
                      peak_file=Path(args.peak_file), failures=args.failure)
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


if __name__ == "__main__":
    raise SystemExit(main())
