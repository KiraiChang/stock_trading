#!/usr/bin/env python3
"""I-074 Stage 2 ⑦b：複本內 orchestrator 的 preflight helper（issue.md I-074「Stage 2 步驟 ⑦b 細部計畫 v1」）。

由 `scripts/run-i074-stage2.sh` 的**複本內** orchestrator 呼叫；⛔ 不是給人直接用的入口。

子指令分兩類：

* **host**（Python 3.9 相容、只用標準庫與 `_i074_bootstrap` 載入的 dependency-light 模組；⛔ 不 import `backtest.*`——
  host 沒有 pandas）：`disk`（v29「磁碟檢查」）、`state-write-*`／`state-check`（「二之八」的五份封閉 state）。
* **容器內**（Stage 2 image，`PYTHONPATH=/app`）：`anchors`——以**目前 XDG 的** Stage 2 identity 呼叫
  `stage2_archive._load_trust_anchors()`（第 1～10 道 ＋ E 系列；「三」#1：`--check-failed-record` 用的是 envcheck
  封存的 identity，驗不到 ai）。

⚠️ `P_B_BUDGET`／`M_SAFETY`／`REQUIRED_BYTES` 的**唯一定義**在這裡（v29 決策 1：寫死、⛔ 不開 CLI、⛔ 不取自執行期的報告）；
`i074_stage2_freeze_record.py` import 它。
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import sys
from pathlib import Path, PurePosixPath
from typing import Any, Callable, Mapping

# ── 常數（⚠️ 由測試釘住值與算術；n6） ─────────────────────────────────────────────

P_B_BUDGET = 167_772_160          # 160 MiB（v29 決策 1）
M_SAFETY = 1_073_741_824          # 1 GiB（⑤ 裁定）
REQUIRED_BYTES = P_B_BUDGET + M_SAFETY   # 1,241,513,984

# replay 的 operational 輸出（`<work>/run/stage2/` 底下）——鏡像 `stage2_archive` 的 `OPERATIONAL_*`，由測試斷言相等。
OPERATIONAL_SUCCESS = ("before_source_artifact.json", "comparison_artifact.json", "report.json")
OPERATIONAL_FAILURE = ("bounded_diagnostics.json",)

EMPTY_SHA256 = hashlib.sha256(b"").hexdigest()


class PreflightError(RuntimeError):
    """preflight 不通過——⛔ fail-closed（orchestrator 以結束碼 1 中止、⛔ 不進 replay）。"""


def _sha256_file(path: str | Path) -> str:
    h = hashlib.sha256()
    with open(path, "rb") as fh:
        for chunk in iter(lambda: fh.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


# ── 磁碟檢查（v29「磁碟檢查」；n1～n4） ─────────────────────────────────────────

def disk_check(locations: Mapping[str, str], docker_root: str, *,
               stat_fn: Callable[[str], Any] = os.stat,
               statvfs_fn: Callable[[str], Any] = os.statvfs) -> dict[str, Any]:
    """全部位置與 Docker Root Dir 同一個 `st_dev`，且 `run` 所在檔案系統的 `f_bavail × f_frsize ≥ REQUIRED_BYTES`。

    ⚠️ 任何 stat／statvfs 失敗都是**拒絕**（⛔ 不得當成「空間足夠」或「同一個裝置」）；⛔ 不用 `f_bfree`（n2）。
    """
    if "run" not in locations:
        raise PreflightError("磁碟檢查缺少 run 位置")
    if not docker_root:
        raise PreflightError("取不到 Docker Root Dir——⛔ 不得當成同一個裝置")
    devices: dict[str, int] = {}
    for name, path in list(locations.items()) + [("docker_root", docker_root)]:
        if not path:
            raise PreflightError(f"磁碟檢查的位置 {name} 是空的")
        try:
            devices[name] = int(stat_fn(path).st_dev)
        except OSError as exc:
            raise PreflightError(f"stat {name}={path} 失敗：{exc}——⛔ fail-closed") from exc
    if len(set(devices.values())) != 1:
        raise PreflightError(f"相關位置不在同一個裝置：{devices}——`P_B` 的單一檔案系統前提不成立")
    try:
        vfs = statvfs_fn(locations["run"])
    except OSError as exc:
        raise PreflightError(f"statvfs 失敗：{exc}——⛔ fail-closed") from exc
    available = int(vfs.f_bavail) * int(vfs.f_frsize)
    if available < REQUIRED_BYTES:
        raise PreflightError(
            f"可用空間 {available} bytes < required {REQUIRED_BYTES}（P_B_BUDGET {P_B_BUDGET} ＋ M_safety {M_SAFETY}）")
    return {"available": available, "required": REQUIRED_BYTES, "st_dev": next(iter(devices.values()))}


# ── state/ 的五份封閉 schema（「二之八」） ────────────────────────────────────────

STATE_ORDER = ("run", "preflight", "replay_started", "replay_done", "attempt")
STATE_FILES = {kind: f"{kind}.json" for kind in STATE_ORDER}
STATE_SCHEMAS = {kind: f"i074_stage2_orch_{kind}/v1" for kind in STATE_ORDER}
CHAIN_KEY = {"preflight": "run_sha256", "replay_started": "preflight_sha256",
             "replay_done": "replay_started_sha256", "attempt": "replay_done_sha256"}
EXCLUSIVE_KINDS = ("run", "replay_started")

_HEX64 = re.compile(r"[0-9a-f]{64}")
_OID40 = re.compile(r"[0-9a-f]{40}")
_IMAGE = re.compile(r"sha256:[0-9a-f]{64}")
_CTRL = re.compile(r"[\x00-\x1f\x7f]")


def is_hex64(v: object) -> bool:
    return isinstance(v, str) and _HEX64.fullmatch(v) is not None


def is_oid40(v: object) -> bool:
    return isinstance(v, str) and _OID40.fullmatch(v) is not None


def is_image(v: object) -> bool:
    return isinstance(v, str) and _IMAGE.fullmatch(v) is not None


def _is_abs(v: object) -> bool:
    return (isinstance(v, str) and v.startswith("/") and not _CTRL.search(v)
            and os.path.realpath(v) == v)


def _is_name(v: object) -> bool:
    return isinstance(v, str) and bool(v) and "/" not in v and not _CTRL.search(v) and v not in (".", "..")


def _is_relpath(v: object) -> bool:
    if not isinstance(v, str) or _CTRL.search(v) or not v.startswith("python/") or v.endswith("/"):
        return False
    return all(part not in ("", ".", "..") for part in v.split("/"))


def _is_int(v: object) -> bool:
    return type(v) is int  # ⛔ bool


def _is_argv(v: object) -> bool:
    return isinstance(v, list) and bool(v) and all(isinstance(a, str) and "\x00" not in a for a in v)


def _is_outputs(v: object) -> bool:
    return isinstance(v, dict) and all(isinstance(k, str) and is_hex64(x) for k, x in v.items())


_H, _O, _IMG, _ABS, _NAME, _REL, _INT, _ARGV, _OUT = (
    is_hex64, is_oid40, is_image, _is_abs, _is_name, _is_relpath, _is_int, _is_argv, _is_outputs)

STATE_FIELDS: dict[str, dict[str, Callable[[object], bool]]] = {
    "run": {"work_dir": _ABS, "real_repo": _ABS, "repo_head": _O, "freeze_record_sha256": _H, "report_sha256": _H},
    "preflight": {"run_sha256": _H, "counterfactual_raw_sha256": _H, "tooling_raw_sha256": _H,
                  "counterfactual_sha256": _H, "tooling_sha256": _H, "composed_sha256": _H,
                  "counterfactual_semantic_sha256": _H, "identity_sha256": _H, "base_commit": _O,
                  "image_id": _IMG, "bundle_id": _NAME, "after_artifact": _REL, "cohort_manifest": _REL},
    "replay_started": {"preflight_sha256": _H, "argv": _ARGV},
    "replay_done": {"replay_started_sha256": _H, "rc": _INT, "outputs": _OUT},
    "attempt": {"replay_done_sha256": _H, "kind": lambda v: v in ("finalize", "publish_failed_record"),
                "rc": _INT, "count": _INT},
}


def _canonical():
    """`replay_bundle/canonical.py`（經 `_i074_bootstrap`；host 沒有 pandas 也能載入）——canonical JSON 的唯一定義。"""
    sys.path.insert(0, str(Path(__file__).resolve().parent))
    from _i074_bootstrap import load_replay_bundle

    return load_replay_bundle(Path(__file__).resolve().parent.parent, ("canonical",))["canonical"]


def canonical_bytes(obj: Any) -> bytes:
    return _canonical().canonical_json_bytes(obj)


def validate_state(kind: str, payload: object) -> None:
    """封閉 schema：鍵集合恰好、型別與格式、單檔內的值域。⚠️ 跨檔條件另外驗（`check_states()`）。"""
    if kind not in STATE_FIELDS:
        raise PreflightError(f"未知的 state：{kind!r}")
    if not isinstance(payload, dict):
        raise PreflightError(f"state {kind} 必須是 object")
    fields = STATE_FIELDS[kind]
    expected = set(fields) | {"schema"}
    if set(payload) != expected:
        raise PreflightError(f"state {kind} 的鍵集合不符：多={sorted(set(payload) - expected)}、"
                             f"缺={sorted(expected - set(payload))}")
    if payload["schema"] != STATE_SCHEMAS[kind]:
        raise PreflightError(f"state {kind} 的 schema {payload['schema']!r} ≠ {STATE_SCHEMAS[kind]!r}")
    for key, check in fields.items():
        if not check(payload[key]):
            raise PreflightError(f"state {kind} 的 {key} 格式不符：{payload[key]!r}")
    if kind == "replay_done":
        if payload["rc"] not in (0, 6):
            raise PreflightError(f"replay_done 的 rc 只能是 0 或 6：{payload['rc']!r}")
        want = set(OPERATIONAL_SUCCESS if payload["rc"] == 0 else OPERATIONAL_FAILURE)
        if set(payload["outputs"]) != want:
            raise PreflightError(f"replay_done 的 outputs 鍵 {sorted(payload['outputs'])} ≠ {sorted(want)}")
    if kind == "attempt":
        if payload["rc"] != 1:
            raise PreflightError(f"attempt 的 rc 必須是 1：{payload['rc']!r}")
        if payload["count"] < 1:
            raise PreflightError(f"attempt 的 count 必須 ≥ 1：{payload['count']!r}")
    if kind == "preflight":
        if payload["counterfactual_raw_sha256"] != payload["counterfactual_sha256"]:
            raise PreflightError("preflight：counterfactual 的 raw SHA ≠ canonical SHA")
        if payload["tooling_raw_sha256"] != payload["tooling_sha256"]:
            raise PreflightError("preflight：tooling 的 raw SHA ≠ canonical SHA")
        if payload["tooling_raw_sha256"] == EMPTY_SHA256:
            raise PreflightError("preflight：tooling patch ⛔ 不得為空（ba 的 orchestrator 層）")


def read_state(state_dir: str | Path, kind: str) -> tuple[dict[str, Any], str]:
    """讀一份 state：canonical（raw ＝ 重新序列化）＋ 封閉 schema。回傳 `(payload, 檔案 SHA)`。"""
    path = Path(state_dir) / STATE_FILES[kind]
    if path.is_symlink() or not path.is_file():
        raise PreflightError(f"state {kind} 不存在或不是一般檔案：{path}")
    raw = path.read_bytes()
    try:
        payload = json.loads(raw.decode("utf-8"))
    except (UnicodeDecodeError, ValueError) as exc:
        raise PreflightError(f"state {kind} 不是合法的 JSON：{exc}") from exc
    if canonical_bytes(payload) != raw:
        raise PreflightError(f"state {kind} 不是 canonical JSON")
    validate_state(kind, payload)
    return payload, hashlib.sha256(raw).hexdigest()


def _fsync_dir(directory: Path) -> None:
    fd = os.open(directory, os.O_RDONLY | os.O_DIRECTORY)
    try:
        os.fsync(fd)
    finally:
        os.close(fd)


def write_state(state_dir: str | Path, kind: str, payload: dict[str, Any]) -> str:
    """同目錄 tmp → fsync → rename（`run`／`replay_started` 以 `link` 做 exclusive create）→ fsync 目錄。回傳檔案 SHA。"""
    validate_state(kind, payload)
    raw = canonical_bytes(payload)
    directory = Path(state_dir)
    final = directory / STATE_FILES[kind]
    tmp = directory / f".{STATE_FILES[kind]}.tmp.{os.getpid()}"
    fd = os.open(tmp, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    try:
        with os.fdopen(fd, "wb") as fh:
            fh.write(raw)
            fh.flush()
            os.fsync(fh.fileno())
        if kind in EXCLUSIVE_KINDS:
            os.link(tmp, final)          # 已存在 → FileExistsError（⛔ 不覆寫）
            os.unlink(tmp)
        else:
            os.replace(tmp, final)
    finally:
        if tmp.exists():
            tmp.unlink()
    _fsync_dir(directory)
    return hashlib.sha256(raw).hexdigest()


def replay_argv(work_dir: str, preflight: Mapping[str, Any]) -> list[str]:
    """replay 的完整 argv——**唯一的組裝處**（orchestrator 以它執行，`replay_started.argv` 以它驗）。"""
    clone = f"{work_dir}/repo"
    return [f"{clone}/scripts/run-replay-offline.sh",
            "--bundle", f"{clone}/python/baselines/{preflight['bundle_id']}",
            "--output-dir", f"{work_dir}/run/stage2",
            "--before-ref", preflight["base_commit"],
            "--after-artifact", f"{clone}/{preflight['after_artifact']}",
            "--cohort-manifest", f"{clone}/{preflight['cohort_manifest']}",
            "--i074-counterfactual"]


def _freeze_record_module():
    sys.path.insert(0, str(Path(__file__).resolve().parent))
    import i074_stage2_freeze_record as fr  # noqa: PLC0415 - 避免循環 import（它 import 本模組的常數）

    return fr


def _check_run_against_inputs(run: Mapping[str, Any], *, work_dir: str, real_repo: str,
                              freeze_record: Path, report: Path) -> dict[str, Any]:
    record, _raw = _freeze_record_module().load_record(freeze_record)
    problems = []
    if run["work_dir"] != work_dir:
        problems.append(f"work_dir {run['work_dir']} ≠ {work_dir}")
    if run["real_repo"] != real_repo:
        problems.append(f"real_repo {run['real_repo']} ≠ {real_repo}")
    if run["repo_head"] != record["repo_head"]:
        problems.append("repo_head ≠ freeze record")
    if run["freeze_record_sha256"] != _sha256_file(freeze_record):
        problems.append("freeze record 副本的 SHA 變了")
    report_sha = _sha256_file(report)
    if run["report_sha256"] != report_sha or report_sha != record["report_sha256"]:
        problems.append("報告副本的 SHA 與 run.json／freeze record 不符")
    if problems:
        raise PreflightError("state run 的交叉條件不符：" + "；".join(problems))
    return record


def _check_preflight_against_record(pre: Mapping[str, Any], record: Mapping[str, Any]) -> None:
    pairs = (("base_commit", "base_commit"), ("counterfactual_raw_sha256", "counterfactual_patch_raw_sha256"),
             ("counterfactual_sha256", "counterfactual_patch_sha256"), ("tooling_raw_sha256", "tooling_patch_raw_sha256"),
             ("tooling_sha256", "tooling_patch_sha256"), ("image_id", "image_id"), ("identity_sha256", "identity_sha256"))
    bad = [a for a, b in pairs if pre[a] != record[b]]
    if bad:
        raise PreflightError(f"state preflight 與 freeze record 不符：{bad}")


def check_states(state_dir: str | Path, *, require: tuple[str, ...], work_dir: str, real_repo: str,
                 image: str | None = None, identity: str | None = None,
                 verify_files: bool = False) -> dict[str, dict[str, Any]]:
    """驗 `require` 列出的每一份（與其餘已存在的每一份）：schema、SHA 鏈、交叉條件。

    `image`／`identity` 給了就比對**目前的**值與 `preflight.json`（resume 與 replay 之後的檢查點；只改 `created_at`
    也會讓 identity 的 SHA 不同）；`verify_files` 另驗兩份凍結 patch 與全部輸出檔的 SHA。
    """
    state_dir = Path(state_dir)
    present = [k for k in STATE_ORDER if (state_dir / STATE_FILES[k]).exists() or (state_dir / STATE_FILES[k]).is_symlink()]
    missing = [k for k in require if k not in present]
    if missing:
        raise PreflightError(f"缺少 state：{missing}")
    loaded: dict[str, tuple[dict[str, Any], str]] = {k: read_state(state_dir, k) for k in present}
    for kind in present:                       # SHA 鏈：每一份都要有前一份，且綁定它的檔案 SHA
        if kind in CHAIN_KEY:
            prev = STATE_ORDER[STATE_ORDER.index(kind) - 1]
            if prev not in loaded:
                raise PreflightError(f"state {kind} 存在，但前一份 {prev} 不存在")
            if loaded[kind][0][CHAIN_KEY[kind]] != loaded[prev][1]:
                raise PreflightError(f"state {kind} 的 {CHAIN_KEY[kind]} ≠ {prev}.json 的 SHA（鏈不符）")
    states = {k: v[0] for k, v in loaded.items()}
    wd = Path(work_dir)
    record = None
    if "run" in states:
        record = _check_run_against_inputs(states["run"], work_dir=work_dir, real_repo=real_repo,
                                           freeze_record=wd / "freeze" / "freeze_record.json",
                                           report=wd / "freeze" / "sizing_report.json")
    pre = states.get("preflight")
    if pre is not None:
        _check_preflight_against_record(pre, record)
        if image is not None and image != pre["image_id"]:
            raise PreflightError(f"目前的 REPLAY_IMAGE_ID {image} ≠ preflight 記錄的 {pre['image_id']}")
        if identity is not None:
            try:
                current = _sha256_file(identity)
            except OSError as exc:
                raise PreflightError(f"讀不到目前的 Stage 2 identity：{exc}") from exc
            if current != pre["identity_sha256"]:
                raise PreflightError("目前的 Stage 2 identity 與 preflight 記錄的 SHA 不符（只改 created_at 也算）")
        if verify_files:
            for name, key in (("counterfactual.patch", "counterfactual_raw_sha256"), ("tooling.patch", "tooling_raw_sha256")):
                path = wd / "run" / "patches" / name
                if path.is_symlink() or not path.is_file() or _sha256_file(path) != pre[key]:
                    raise PreflightError(f"凍結 patch {name} 不存在或 SHA 與 preflight 記錄的不符")
    started = states.get("replay_started")
    if started is not None and started["argv"] != replay_argv(work_dir, pre):
        raise PreflightError("replay_started 的 argv ≠ 由 preflight 重組的 argv")
    done = states.get("replay_done")
    if done is not None and verify_files:
        stage2 = wd / "run" / "stage2"
        actual = sorted(p.name for p in stage2.iterdir()) if stage2.is_dir() else []
        if actual != sorted(done["outputs"]):
            raise PreflightError(f"operational 輸出的檔案集合 {actual} ≠ 記錄的 {sorted(done['outputs'])}")
        for name, sha in done["outputs"].items():
            if (stage2 / name).is_symlink() or _sha256_file(stage2 / name) != sha:
                raise PreflightError(f"operational 輸出 {name} 的 SHA 變了")
    attempt = states.get("attempt")
    if attempt is not None:
        want = "finalize" if done["rc"] == 0 else "publish_failed_record"
        if attempt["kind"] != want:
            raise PreflightError(f"attempt 的 kind {attempt['kind']} 與 replay_done 的 rc {done['rc']} 不對應")
    return states


def _output_shape(stage2: Path, rc: int) -> dict[str, str]:
    """rc 0 ⇒ 恰好三個 operational 檔；rc 6 ⇒ 恰好 `bounded_diagnostics.json`；都是一般檔案。"""
    if rc not in (0, 6):
        raise PreflightError(f"replay 的結束碼 {rc} ∉ {{0, 6}}")
    want = OPERATIONAL_SUCCESS if rc == 0 else OPERATIONAL_FAILURE
    if stage2.is_symlink() or not stage2.is_dir():
        raise PreflightError(f"replay 的輸出目錄不存在：{stage2}")
    actual = sorted(p.name for p in stage2.iterdir())
    if actual != sorted(want):
        raise PreflightError(f"replay rc={rc} 的輸出形狀 {actual} ≠ {sorted(want)}")
    out = {}
    for name in want:
        path = stage2 / name
        if path.is_symlink() or not path.is_file():
            raise PreflightError(f"operational 輸出 {name} 不是一般檔案")
        out[name] = _sha256_file(path)
    return out


# ── 容器內：信任錨（preflight 第 3 步） ───────────────────────────────────────────

def run_anchors(python_root: str, identity_path: str, image: str) -> dict[str, str]:
    """⚠️ 容器內（Stage 2 image）。以**目前的** identity 走 `_load_trust_anchors()`——與 finalizer 同一個推導。"""
    from backtest.modular.sr_scoring.replay_bundle import stage2_archive as sa
    from backtest.modular.sr_scoring.replay_bundle import stage2_evidence as s2
    from backtest.modular.sr_scoring.replay_bundle.run_identity import load_run_identity

    if not is_image(image):
        raise PreflightError(f"--image-digest 格式不符：{image!r}")
    identity = load_run_identity(identity_path)
    if identity["expected_image_id"] != image:
        raise PreflightError(f"Stage 2 identity 的 expected_image_id {identity['expected_image_id']} ≠ 本次的 image {image}")
    anchor, _envcheck = sa._load_trust_anchors(python_root, identity)
    root = PurePosixPath(s2.STAGE1_EVIDENCE_MANIFEST_PATH).parent
    return {"bundle_id": anchor.bundle_id, "after_base_commit": anchor.after_base_commit,
            "after_artifact": str(root / s2.STAGE1_ANCHOR_AFTER),
            "cohort_manifest": str(root / s2.STAGE1_ANCHOR_COHORT)}


# ── CLI ───────────────────────────────────────────────────────────────────────

def _parse_locations(items: list[str]) -> dict[str, str]:
    out: dict[str, str] = {}
    for item in items:
        name, sep, path = item.partition("=")
        if not sep or not name or name in out:
            raise PreflightError(f"--location 格式不符或重複：{item!r}")
        out[name] = path
    return out


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(prog="i074_stage2_preflight", allow_abbrev=False)
    sub = parser.add_subparsers(dest="cmd", required=True)
    p = sub.add_parser("disk")
    p.add_argument("--location", action="append", default=[], required=True)
    p.add_argument("--docker-root", required=True)
    p = sub.add_parser("anchors")
    p.add_argument("--python-root", required=True)
    p.add_argument("--run-identity", required=True)
    p.add_argument("--image-digest", required=True)
    for name in ("state-write-run", "state-write-preflight", "state-write-replay-started",
                 "state-write-replay-done", "state-write-attempt", "state-check"):
        p = sub.add_parser(name)
        p.add_argument("--state-dir", required=True)
        p.add_argument("--work-dir", required=True)
        p.add_argument("--real-repo", required=True)
        if name == "state-write-preflight":
            for opt in ("--counterfactual-sha256", "--tooling-sha256", "--composed-sha256", "--semantic-sha256",
                        "--identity", "--image", "--anchors"):
                p.add_argument(opt, required=True)
        if name == "state-write-replay-done":
            p.add_argument("--rc", required=True, type=int)
        if name == "state-write-attempt":
            p.add_argument("--kind", required=True, choices=("finalize", "publish_failed_record"))
        if name == "state-check":
            p.add_argument("--require", required=True)
            p.add_argument("--image")
            p.add_argument("--identity")
            p.add_argument("--verify-files", action="store_true")
    args = parser.parse_args(argv)
    try:
        return _dispatch(args)
    except (PreflightError, OSError, ValueError) as exc:
        print(f"ERROR: {type(exc).__name__}: {exc}", file=sys.stderr)
        return 1


def _dispatch(args) -> int:
    if args.cmd == "disk":
        print(json.dumps(disk_check(_parse_locations(args.location), args.docker_root), sort_keys=True))
        return 0
    if args.cmd == "anchors":
        print(json.dumps(run_anchors(args.python_root, args.run_identity, args.image_digest), sort_keys=True))
        return 0
    state_dir, work_dir = Path(args.state_dir), args.work_dir
    if not _is_abs(work_dir):
        raise PreflightError(f"--work-dir 必須是 canonical 絕對路徑：{work_dir!r}")
    wd = Path(work_dir)
    if args.cmd == "state-write-run":
        freeze, report = wd / "freeze" / "freeze_record.json", wd / "freeze" / "sizing_report.json"
        record, _raw = _freeze_record_module().load_record(freeze)
        payload = {"schema": STATE_SCHEMAS["run"], "work_dir": work_dir, "real_repo": args.real_repo,
                   "repo_head": record["repo_head"], "freeze_record_sha256": _sha256_file(freeze),
                   "report_sha256": _sha256_file(report)}
        _check_run_against_inputs(payload, work_dir=work_dir, real_repo=args.real_repo, freeze_record=freeze, report=report)
        write_state(state_dir, "run", payload)
        return 0
    if args.cmd == "state-check":
        require = tuple(k for k in args.require.split(",") if k)
        states = check_states(state_dir, require=require, work_dir=work_dir, real_repo=args.real_repo,
                              image=args.image, identity=args.identity, verify_files=args.verify_files)
        summary = {"present": ",".join(states)}
        if "preflight" in states:
            summary.update(bundle_id=states["preflight"]["bundle_id"],
                           semantic=states["preflight"]["counterfactual_semantic_sha256"])
        if "replay_done" in states:
            summary["rc"] = str(states["replay_done"]["rc"])
        for key, value in summary.items():        # shell 以 `KEY=VALUE` 讀（值都是 hex、數字、逗號與 name）
            print(f"{key}={value}")
        return 0
    real_repo = args.real_repo
    states = check_states(state_dir, require=("run",), work_dir=work_dir, real_repo=real_repo)
    if args.cmd == "state-write-preflight":
        if set(states) != {"run"}:
            raise PreflightError(f"寫 preflight 時 state 只能有 run：{sorted(states)}")
        anchors = json.loads(Path(args.anchors).read_text(encoding="utf-8"))
        if not isinstance(anchors, dict) or set(anchors) != {"bundle_id", "after_base_commit", "after_artifact",
                                                               "cohort_manifest"}:
            raise PreflightError("anchors 的輸出格式不符")
        identity_sha = _sha256_file(args.identity)
        payload = {"schema": STATE_SCHEMAS["preflight"], "run_sha256": _state_sha(state_dir, "run"),
                   "counterfactual_raw_sha256": _sha256_file(wd / "run" / "patches" / "counterfactual.patch"),
                   "tooling_raw_sha256": _sha256_file(wd / "run" / "patches" / "tooling.patch"),
                   "counterfactual_sha256": args.counterfactual_sha256, "tooling_sha256": args.tooling_sha256,
                   "composed_sha256": args.composed_sha256, "counterfactual_semantic_sha256": args.semantic_sha256,
                   "identity_sha256": identity_sha, "base_commit": anchors["after_base_commit"],
                   "image_id": args.image, "bundle_id": anchors["bundle_id"],
                   "after_artifact": anchors["after_artifact"], "cohort_manifest": anchors["cohort_manifest"]}
        validate_state("preflight", payload)
        record, _raw = _freeze_record_module().load_record(wd / "freeze" / "freeze_record.json")
        _check_preflight_against_record(payload, record)
        write_state(state_dir, "preflight", payload)
        return 0
    if args.cmd == "state-write-replay-started":
        states = check_states(state_dir, require=("run", "preflight"), work_dir=work_dir,
                              real_repo=real_repo)
        if set(states) != {"run", "preflight"}:
            raise PreflightError(f"寫 replay_started 時 state 只能有 run、preflight：{sorted(states)}")
        argv = replay_argv(work_dir, states["preflight"])
        write_state(state_dir, "replay_started", {"schema": STATE_SCHEMAS["replay_started"],
                                                  "preflight_sha256": _state_sha(state_dir, "preflight"),
                                                  "argv": argv})
        sys.stdout.write("".join(a + "\0" for a in argv))   # shell 以 `mapfile -d ''` 讀
        return 0
    if args.cmd == "state-write-replay-done":
        states = check_states(state_dir, require=("run", "preflight", "replay_started"), work_dir=work_dir,
                              real_repo=real_repo)
        if "replay_done" in states:
            raise PreflightError("replay_done 已存在——⛔ 不覆寫")
        outputs = _output_shape(wd / "run" / "stage2", args.rc)
        write_state(state_dir, "replay_done", {"schema": STATE_SCHEMAS["replay_done"],
                                               "replay_started_sha256": _state_sha(state_dir, "replay_started"),
                                               "rc": args.rc, "outputs": outputs})
        return 0
    if args.cmd == "state-write-attempt":
        states = check_states(state_dir, require=("run", "preflight", "replay_started", "replay_done"),
                              work_dir=work_dir, real_repo=real_repo)
        count = states["attempt"]["count"] + 1 if "attempt" in states else 1
        write_state(state_dir, "attempt", {"schema": STATE_SCHEMAS["attempt"],
                                           "replay_done_sha256": _state_sha(state_dir, "replay_done"),
                                           "kind": args.kind, "rc": 1, "count": count})
        return 0
    raise PreflightError(f"未知的子指令：{args.cmd}")


def _state_sha(state_dir: Path, kind: str) -> str:
    return read_state(state_dir, kind)[1]



if __name__ == "__main__":
    raise SystemExit(main())
