#!/usr/bin/env python3
"""I-074 Stage 2 ⑦b：freeze record 的寫入與驗證（issue.md I-074 v29「八之二」＋「Stage 2 步驟 ⑦b 細部計畫 v1」「二之九」）。

host 端（Python 3.9 相容）：只用標準庫、`_i074_bootstrap` 載入的 `replay_bundle/canonical.py`（canonical JSON 的唯一定義），
以及需要時的 `git`。⛔ 不 import `backtest.*`（host 沒有 pandas）。

子指令：

* `build`——sizing harness 在 `--formal` 的報告產出之後呼叫：只有報告 `mode = formal`、`status = ok` 且
  `P_B ≤ P_B_BUDGET` 才寫（⛔ 否則印原因、結束碼 0、⛔ 不寫；「三」#9）；寫之前以驗證端的同一組函式驗自己的輸出。
* `check-pair`——freeze record ＋ 同目錄的報告：封閉 schema、`report_sha256`、交叉不變條件（sizing 複製之後再驗一次）。
* `check-basic`——持鎖階段（真正 repo 的 HEAD 版）：canonical ＋ 封閉 schema 與逐欄規則；stdout 只印 `repo_head`。
* `check-full`——複本內 preflight 第 2 步：`check-pair` ＋ 複本的腳本內容 ＋ 凍結 patch ＋ compose 的增量 SHA ＋ identity／image。

⚠️ 它⛔ 不進證據鏈（「八之二」）：⑩ 只用它決定複本釘在哪個 OID，並證明那個 OID 經過確認重跑。
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import subprocess
import sys
from pathlib import Path
from typing import Any, Mapping

sys.path.insert(0, str(Path(__file__).resolve().parent))
from i074_stage2_preflight import P_B_BUDGET  # noqa: E402 - P_B_BUDGET 的唯一定義

SCHEMA = "i074_stage2_freeze_record/v1"
# ⚠️ ⑩ 的 before 版本（`e1cbbbd` 的完整 OID）。與 `scripts/make-i074-tooling-patch.sh` 的 `I074_TOOLING_BASE` 由測試斷言相等。
BASE_COMMIT = "e1cbbbdab44f8cf2d152e6ade9235d844f590d7f"
# 「八之二」：三個 sizing 腳本在 `repo_head` 中的**檔案內容** SHA-256（⛔ 不是 blob OID）。
SCRIPT_FIELDS = {
    "harness_sha256": "scripts/i074-stage2-sizing.sh",
    "shim_sha256": "scripts/lib/i074-sizing-docker-shim.sh",
    "helper_sha256": "python/scripts/i074_stage2_sizing.py",
}
EMPTY_SHA256 = hashlib.sha256(b"").hexdigest()

_HEX64 = re.compile(r"[0-9a-f]{64}")
_OID40 = re.compile(r"[0-9a-f]{40}")
_IMAGE = re.compile(r"sha256:[0-9a-f]{64}")
_RUN_ID = re.compile(r"[0-9]{8}T[0-9]{6}Z-[0-9]+")


def _hex64(v: object) -> bool:
    return isinstance(v, str) and _HEX64.fullmatch(v) is not None


def _oid40(v: object) -> bool:
    return isinstance(v, str) and _OID40.fullmatch(v) is not None


FIELDS: dict[str, Any] = {
    "schema": lambda v: v == SCHEMA,
    "mode": lambda v: v == "formal",
    "status": lambda v: v == "ok",
    "run_id": lambda v: isinstance(v, str) and _RUN_ID.fullmatch(v) is not None,
    "repo_head": _oid40,
    "clone_head": _oid40,
    "base_commit": _oid40,
    "p_b_bytes": lambda v: type(v) is int and v > 0,      # ⛔ bool
    "report_sha256": _hex64,
    "harness_sha256": _hex64,
    "shim_sha256": _hex64,
    "helper_sha256": _hex64,
    "counterfactual_patch_raw_sha256": _hex64,
    "counterfactual_patch_sha256": _hex64,
    "tooling_patch_raw_sha256": _hex64,
    "tooling_patch_sha256": _hex64,
    "image_id": lambda v: isinstance(v, str) and _IMAGE.fullmatch(v) is not None,
    "identity_sha256": _hex64,
}

# 報告 `meta` 的鍵 ↔ freeze record 的欄位（交叉不變條件，逐欄相等）。
META_PAIRS = (
    ("run_id", "run_id"), ("repo_head", "repo_head"), ("clone_head", "clone_head"), ("base_commit", "base_commit"),
    ("harness_sha256", "harness_sha256"), ("shim_sha256", "shim_sha256"), ("helper_sha256", "helper_sha256"),
    ("image", "image_id"), ("identity_sha256", "identity_sha256"),
    ("counterfactual_sha256", "counterfactual_patch_raw_sha256"),
    ("counterfactual_canonical_sha256", "counterfactual_patch_sha256"),
    ("tooling_sha256", "tooling_patch_raw_sha256"), ("tooling_canonical_sha256", "tooling_patch_sha256"),
)


class FreezeRecordError(ValueError):
    """freeze record 不符——⛔ fail-closed（replay ⛔ 未被呼叫、結束碼 1、⛔ 不計入正式 scan）。"""


def _canonical_json_bytes(obj: Any) -> bytes:
    from _i074_bootstrap import load_replay_bundle

    return load_replay_bundle(Path(__file__).resolve().parent.parent, ("canonical",))["canonical"].canonical_json_bytes(obj)


def sha256_file(path: str | Path) -> str:
    h = hashlib.sha256()
    with open(path, "rb") as fh:
        for chunk in iter(lambda: fh.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def validate_record(record: object) -> None:
    """「八之二」的封閉 schema 與逐欄規則（⛔ 不含需要外部檔案的交叉條件）。"""
    if not isinstance(record, dict):
        raise FreezeRecordError("freeze record 必須是 object")
    if set(record) != set(FIELDS):
        raise FreezeRecordError(f"freeze record 的鍵集合不符：多={sorted(set(record) - set(FIELDS))}、"
                                f"缺={sorted(set(FIELDS) - set(record))}")
    for key, check in FIELDS.items():
        if not check(record[key]):
            raise FreezeRecordError(f"freeze record 的 {key} 不符：{record[key]!r}")
    problems = []
    if record["clone_head"] != record["repo_head"]:
        problems.append("clone_head ≠ repo_head")
    if record["base_commit"] != BASE_COMMIT:
        problems.append(f"base_commit ≠ {BASE_COMMIT}")
    if record["p_b_bytes"] > P_B_BUDGET:
        problems.append(f"p_b_bytes {record['p_b_bytes']} > P_B_BUDGET {P_B_BUDGET}")
    if record["counterfactual_patch_raw_sha256"] != record["counterfactual_patch_sha256"]:
        problems.append("counterfactual 的 raw SHA ≠ canonical SHA")
    if record["tooling_patch_raw_sha256"] != record["tooling_patch_sha256"]:
        problems.append("tooling 的 raw SHA ≠ canonical SHA")
    if record["tooling_patch_raw_sha256"] == EMPTY_SHA256:
        problems.append("tooling patch ⛔ 不得為空")
    if problems:
        raise FreezeRecordError("freeze record 不符：" + "；".join(problems))


def load_record(path: str | Path) -> tuple[dict[str, Any], bytes]:
    """讀檔 → canonical（raw ＝ 重新序列化）→ `validate_record()`。回傳 `(record, raw)`。"""
    path = Path(path)
    if path.is_symlink() or not path.is_file():
        raise FreezeRecordError(f"freeze record 不存在或不是一般檔案：{path}")
    raw = path.read_bytes()
    try:
        record = json.loads(raw.decode("utf-8"))
    except (UnicodeDecodeError, ValueError) as exc:
        raise FreezeRecordError(f"freeze record 不是合法的 JSON：{exc}") from exc
    if _canonical_json_bytes(record) != raw:
        raise FreezeRecordError("freeze record 不是 canonical JSON")
    validate_record(record)
    return record, raw


def cross_check_report(record: Mapping[str, Any], report_raw: bytes) -> None:
    """`report_sha256` ＋ 報告的 `mode`、`status`、`P_B` 與 `meta` 逐欄相等（「八之二」的交叉不變條件）。"""
    if hashlib.sha256(report_raw).hexdigest() != record["report_sha256"]:
        raise FreezeRecordError("報告檔案的 SHA ≠ freeze record 的 report_sha256")
    try:
        report = json.loads(report_raw.decode("utf-8"))
    except (UnicodeDecodeError, ValueError) as exc:
        raise FreezeRecordError(f"報告不是合法的 JSON：{exc}") from exc
    meta = report.get("meta") if isinstance(report, dict) else None
    if not isinstance(meta, dict):
        raise FreezeRecordError("報告缺少 meta")
    problems = []
    if report.get("mode") != record["mode"] or meta.get("mode") != record["mode"]:
        problems.append("mode")
    if report.get("status") != record["status"]:
        problems.append("status")
    if type(report.get("P_B")) is not int or report["P_B"] != record["p_b_bytes"]:
        problems.append("P_B")
    problems += [f"meta.{m}" for m, r in META_PAIRS if meta.get(m) != record[r]]
    if problems:
        raise FreezeRecordError(f"報告與 freeze record 不符：{problems}")


def check_pair(record_path: str | Path, report_path: str | Path) -> dict[str, Any]:
    record, _raw = load_record(record_path)
    report_path = Path(report_path)
    if report_path.is_symlink() or not report_path.is_file():
        raise FreezeRecordError(f"報告不存在或不是一般檔案：{report_path}")
    cross_check_report(record, report_path.read_bytes())
    return record


def _identity_facts(identity_path: str | Path) -> tuple[str, str]:
    """identity 檔的 `(SHA-256, expected_image_id)`。⚠️ 完整的 schema 驗證在 anchors（容器內）與 runner／finalizer。"""
    path = Path(identity_path)
    if path.is_symlink() or not path.is_file():
        raise FreezeRecordError(f"Stage 2 identity 不存在或不是一般檔案：{path}")
    raw = path.read_bytes()
    try:
        image = json.loads(raw.decode("utf-8")).get("expected_image_id")
    except (UnicodeDecodeError, ValueError, AttributeError) as exc:
        raise FreezeRecordError(f"Stage 2 identity 讀不懂：{exc}") from exc
    return hashlib.sha256(raw).hexdigest(), image


def check_full(*, record_path: str | Path, report_path: str | Path, clone: str | Path, frozen_counterfactual: str | Path,
               frozen_tooling: str | Path, counterfactual_canonical: str, tooling_canonical: str,
               identity: str | Path, image: str) -> dict[str, Any]:
    """複本內 preflight 第 2 步（「二之九」的驗證端）。"""
    record = check_pair(record_path, report_path)
    clone = Path(clone)
    problems = [f for f, rel in SCRIPT_FIELDS.items() if sha256_file(clone / rel) != record[f]]
    if sha256_file(frozen_counterfactual) != record["counterfactual_patch_raw_sha256"]:
        problems.append("凍結的 counterfactual（raw）")
    if sha256_file(frozen_tooling) != record["tooling_patch_raw_sha256"]:
        problems.append("凍結的 tooling（raw）")
    if counterfactual_canonical != record["counterfactual_patch_sha256"]:
        problems.append("counterfactual 的增量 canonical SHA")
    if tooling_canonical != record["tooling_patch_sha256"]:
        problems.append("tooling 的增量 canonical SHA")
    identity_sha, identity_image = _identity_facts(identity)
    if identity_sha != record["identity_sha256"]:
        problems.append("Stage 2 identity 的 SHA")
    if not (identity_image == record["image_id"] == image):
        problems.append("image（identity／freeze record／REPLAY_IMAGE_ID）")
    if problems:
        raise FreezeRecordError(f"freeze record 與本次的事實不符：{problems}")
    return record


def head_file_sha256(repo: str | Path, rev: str, rel: str) -> str:
    """`git show <rev>:<rel>` 的**內容** SHA-256（⛔ 不是 blob OID——blob OID 有 `blob <len>\\0` 前綴）。"""
    if not _oid40(rev):
        raise FreezeRecordError(f"rev 必須是 40 碼 OID：{rev!r}")
    env = {k: v for k, v in os.environ.items() if not k.startswith("GIT_")}
    proc = subprocess.run(["git", "-C", str(repo), "cat-file", "blob", f"{rev}:{rel}"],
                          stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=env, check=False)
    if proc.returncode != 0:
        raise FreezeRecordError(f"讀不到 {rev}:{rel}：{proc.stderr.decode(errors='replace').strip()}")
    return hashlib.sha256(proc.stdout).hexdigest()


def build_record(*, report_path: str | Path, repo: str | Path, identity: str | Path) -> tuple[dict[str, Any] | None, str]:
    """sizing 的寫入端。回傳 `(record, "")`，或 `(None, 不寫的原因)`（⛔ 不是錯誤：量測本身有效，「三」#9）。

    不一致（腳本 SHA、identity、meta 缺欄）一律丟 `FreezeRecordError`——那代表報告本身不可信。
    """
    raw = Path(report_path).read_bytes()
    report = json.loads(raw.decode("utf-8"))
    meta = report.get("meta", {})
    if report.get("mode") != "formal" or meta.get("mode") != "formal":
        return None, f"mode={report.get('mode')!r}（只有 formal 才寫）"
    if report.get("status") != "ok":
        return None, f"status={report.get('status')!r}（只有 ok 才寫）"
    p_b = report.get("P_B")
    if type(p_b) is not int:
        raise FreezeRecordError(f"報告的 P_B 不是整數：{p_b!r}")
    if p_b > P_B_BUDGET:
        return None, f"P_B {p_b} > P_B_BUDGET {P_B_BUDGET}（走「六、1」的回退順序）"
    missing = [m for m, _r in META_PAIRS if not meta.get(m)]
    if missing:
        raise FreezeRecordError(f"報告的 meta 缺少：{missing}")
    record = {"schema": SCHEMA, "mode": "formal", "status": "ok", "p_b_bytes": p_b,
              "report_sha256": hashlib.sha256(raw).hexdigest()}
    for m, r in META_PAIRS:
        record[r] = meta[m]
    for field, rel in SCRIPT_FIELDS.items():
        actual = head_file_sha256(repo, record["repo_head"], rel)
        if actual != record[field]:
            raise FreezeRecordError(f"{rel} 在 repo_head 中的內容 SHA {actual} ≠ 報告記錄的 {record[field]}")
    identity_sha, identity_image = _identity_facts(identity)
    if identity_sha != record["identity_sha256"] or identity_image != record["image_id"]:
        raise FreezeRecordError("Stage 2 identity 與報告記錄的 SHA 或 image 不符")
    validate_record(record)
    cross_check_report(record, raw)
    return record, ""


def write_record(path: str | Path, record: Mapping[str, Any]) -> None:
    """exclusive create ＋ fsync（⛔ 不覆寫）；寫完重讀驗一次。"""
    raw = _canonical_json_bytes(dict(record))
    path = Path(path)
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o644)
    with os.fdopen(fd, "wb") as fh:
        fh.write(raw)
        fh.flush()
        os.fsync(fh.fileno())
    if load_record(path)[1] != raw:
        raise FreezeRecordError("寫出的 freeze record 重讀不一致")


# ── CLI ───────────────────────────────────────────────────────────────────────

def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(prog="i074_stage2_freeze_record", allow_abbrev=False)
    sub = parser.add_subparsers(dest="cmd", required=True)
    p = sub.add_parser("build")
    p.add_argument("--report", required=True)
    p.add_argument("--repo", required=True)
    p.add_argument("--identity", required=True)
    p.add_argument("--out", required=True)
    p = sub.add_parser("check-pair")
    p.add_argument("--record", required=True)
    p.add_argument("--report", required=True)
    p = sub.add_parser("check-basic")
    p.add_argument("--record", required=True)
    p = sub.add_parser("check-full")
    for opt in ("--record", "--report", "--clone", "--frozen-counterfactual", "--frozen-tooling",
                "--counterfactual-canonical", "--tooling-canonical", "--identity", "--image"):
        p.add_argument(opt, required=True)
    args = parser.parse_args(argv)
    try:
        if args.cmd == "build":
            record, reason = build_record(report_path=args.report, repo=args.repo, identity=args.identity)
            if record is None:
                print(f"⚠️ ⛔ 不寫 freeze record：{reason}", file=sys.stderr)
                return 0
            write_record(args.out, record)
            print(f"freeze record：{args.out}", file=sys.stderr)
            return 0
        if args.cmd == "check-pair":
            check_pair(args.record, args.report)
            return 0
        if args.cmd == "check-basic":
            print(load_record(args.record)[0]["repo_head"])
            return 0
        check_full(record_path=args.record, report_path=args.report, clone=args.clone,
                   frozen_counterfactual=args.frozen_counterfactual, frozen_tooling=args.frozen_tooling,
                   counterfactual_canonical=args.counterfactual_canonical, tooling_canonical=args.tooling_canonical,
                   identity=args.identity, image=args.image)
        return 0
    except (FreezeRecordError, OSError, ValueError) as exc:
        print(f"ERROR: {type(exc).__name__}: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
