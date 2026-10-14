#!/usr/bin/env python3
"""I-074 Stage 2 步驟 ⑨：tooling 非語意 guard 的 comparator（docs/issue.md「Stage 2 步驟 ⑨ 細部計畫 v1」「五」）。

同一份切片 bundle、一般 Stage 1／2（⛔ 不帶 `--i074-counterfactual`），A ＝ `e1cbbbd`、B ＝ `e1cbbbd` ＋ tooling：
**成對**比較 Stage 1 A／B 與 Stage 2 A／B 的輸出——依**逐檔的封閉規則**只移除列在表裡的鍵（`rows`／`keys` 永遠
⛔ 不移除），其餘以 canonical JSON 逐位元比對；兩個涵蓋 provenance 的 hash 先驗**自洽**才移除；provenance 本身
⛔ 不參與 bytes 比對，但逐欄驗期望值。

⚠️ 只用標準庫、⛔ 不 import 任何產品模組。結束碼：0 ＝ 通過；1 ＝ 不通過或輸入不合法。
"""
from __future__ import annotations

import argparse
import hashlib
import json
import re
import sys
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple

REPORT_SCHEMA = "i074_tooling_guard_report_v1"
EMPTY_SHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

AFTER = "after_artifact.json"
COHORT = "cohort_manifest.json"
COMPARISON = "comparison_artifact.json"
REPORT = "report.json"
STAGE_FILES = {1: (AFTER, COHORT), 2: (COMPARISON, REPORT)}

# ⚠️ **逐檔的封閉規則**（計畫「五」的表）：唯一可移除的頂層鍵。`rows`、`keys` ⛔ 不得移除（任何一份都一樣）。
REMOVABLE: Dict[str, Tuple[str, ...]] = {
    AFTER: ("generated_at", "provenance"),
    COHORT: ("generated_at", "provenance", "after_artifact_sha256"),
    COMPARISON: ("generated_at", "provenance"),
    REPORT: ("generated_at", "comparison_artifact_sha256"),
}
NEVER_REMOVED = ("rows", "keys")

PROVENANCE_FIELDS = ("source_root", "image_digest", "python_version", "pip_freeze_sha256", "project_modules_sha256",
                     "runner_sha256", "base_commit", "tooling_patch_sha256", "argv", "runtime_settings")
PROVENANCE_SAME = ("image_digest", "python_version", "pip_freeze_sha256", "runner_sha256", "source_root",
                   "runtime_settings")
# argv 在同一個 stage 的 A／B 之間，只允許這兩個參數的**值**不同。
ARGV_VARIABLE_VALUES = ("--tooling-patch-sha256", "--output-dir")
TOOLING_MODULE = re.compile(r"backtest\.modular\.sr_scoring\.(evaluation|replay_bundle)(\..+)?")


class ToolingGuardError(ValueError):
    pass


def sha256_file(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def canonical(obj: Any) -> bytes:
    return json.dumps(obj, ensure_ascii=False, sort_keys=True, separators=(",", ":"), allow_nan=False).encode("utf-8")


def _reject_constant(name: str) -> Any:
    raise ToolingGuardError(f"JSON 含有非有限的數值 {name}")


def load(path: Path) -> Dict[str, Any]:
    try:
        doc = json.loads(path.read_bytes().decode("utf-8"), parse_constant=_reject_constant)
    except (OSError, UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise ToolingGuardError(f"讀不到或不是合法 JSON：{path}：{exc}") from exc
    if not isinstance(doc, dict):
        raise ToolingGuardError(f"頂層必須是 object：{path}")
    return doc


def normalize(name: str, doc: Dict[str, Any]) -> Dict[str, Any]:
    for key in NEVER_REMOVED:
        if key in REMOVABLE[name]:
            raise ToolingGuardError(f"規則表錯誤：{key} ⛔ 不得被移除")
    return {k: v for k, v in doc.items() if k not in REMOVABLE[name]}


def _first_diff(a: Dict[str, Any], b: Dict[str, Any]) -> List[str]:
    keys = sorted(set(a) | set(b))
    out = [k for k in keys if k not in a or k not in b or canonical(a[k]) != canonical(b[k])]
    detail = []
    for k in out:
        if k in ("rows", "keys") and isinstance(a.get(k), list) and isinstance(b.get(k), list):
            ra, rb = a[k], b[k]
            idx = next((i for i in range(min(len(ra), len(rb))) if canonical(ra[i]) != canonical(rb[i])), None)
            detail.append(f"{k}（A {len(ra)} 列、B {len(rb)} 列，第一個不同的列：{idx}）")
        else:
            detail.append(k)
    return detail


def check_files(dirs: Dict[Tuple[int, str], Path]) -> List[str]:
    problems = []
    for (stage, side), path in sorted(dirs.items()):
        if not path.is_dir():
            problems.append(f"Stage {stage} {side}：輸出目錄不存在：{path}")
            continue
        names = sorted(p.name for p in path.iterdir())
        if names != sorted(STAGE_FILES[stage]):
            problems.append(f"Stage {stage} {side}：輸出的檔案集合 {names} ≠ {sorted(STAGE_FILES[stage])}")
    return problems


def _argv_problems(label: str, a: Any, b: Any) -> List[str]:
    if not (isinstance(a, list) and isinstance(b, list)):
        return [f"{label}：argv 不是 list"]
    if len(a) != len(b):
        return [f"{label}：argv 長度不同（{len(a)}／{len(b)}）"]
    out = []
    for i, (x, y) in enumerate(zip(a, b)):
        if x != y and not (i > 0 and a[i - 1] == b[i - 1] and a[i - 1] in ARGV_VARIABLE_VALUES):
            out.append(f"{label}：argv 第 {i} 個 token 不同（{x!r}／{y!r}）")
    return out


def provenance_problems(label: str, pa: Any, pb: Any, expected_tooling: str, base_commit: str) -> List[str]:
    out: List[str] = []
    for side, prov in (("A", pa), ("B", pb)):
        if not isinstance(prov, dict) or set(prov) != set(PROVENANCE_FIELDS):
            out.append(f"{label} {side}：provenance 的欄位必須恰好是 PROVENANCE_FIELDS")
    if out:
        return out
    if pa["tooling_patch_sha256"] != EMPTY_SHA256:
        out.append(f"{label} A：tooling_patch_sha256 ≠ 空字串的 SHA（{pa['tooling_patch_sha256']}）")
    if pb["tooling_patch_sha256"] != expected_tooling:
        out.append(f"{label} B：tooling_patch_sha256 ≠ 封存的 tooling canonical SHA（{pb['tooling_patch_sha256']}）")
    for side, prov in (("A", pa), ("B", pb)):
        if prov["base_commit"] != base_commit:
            out.append(f"{label} {side}：base_commit ≠ {base_commit}")
    for field in PROVENANCE_SAME:
        if canonical(pa[field]) != canonical(pb[field]):
            out.append(f"{label}：provenance.{field} 兩側不同")
    ma, mb = pa["project_modules_sha256"], pb["project_modules_sha256"]
    if not (isinstance(ma, dict) and isinstance(mb, dict)):
        out.append(f"{label}：project_modules_sha256 不是 object")
    else:
        bad = sorted(k for k in set(ma) | set(mb) if ma.get(k) != mb.get(k) and not TOOLING_MODULE.fullmatch(k))
        if bad:
            out.append(f"{label}：tooling 以外的模組兩側不同：{bad}")
    out.extend(_argv_problems(label, pa["argv"], pb["argv"]))
    return out


def run(dirs: Dict[Tuple[int, str], Path], expected_tooling: str, base_commit: str) -> Dict[str, Any]:
    problems = check_files(dirs)
    if problems:
        return {"ok": False, "problems": problems}
    docs = {(stage, side, name): load(dirs[(stage, side)] / name)
            for (stage, side) in dirs for name in STAGE_FILES[stage]}
    sha = {(stage, side, name): sha256_file(dirs[(stage, side)] / name)
           for (stage, side) in dirs for name in STAGE_FILES[stage]}
    # hash 自洽（先驗、才移除）
    for side in ("A", "B"):
        if docs[(1, side, COHORT)].get("after_artifact_sha256") != sha[(1, side, AFTER)]:
            problems.append(f"Stage 1 {side}：cohort 的 after_artifact_sha256 ≠ 同目錄 after 檔的 SHA")
        if docs[(2, side, REPORT)].get("comparison_artifact_sha256") != sha[(2, side, COMPARISON)]:
            problems.append(f"Stage 2 {side}：report 的 comparison_artifact_sha256 ≠ 同目錄 comparison 檔的 SHA")
        if docs[(2, side, COMPARISON)].get("after_artifact_sha256") != sha[(1, "A", AFTER)]:
            problems.append(f"Stage 2 {side}：comparison 的 after_artifact_sha256 ≠ 共用的 after（Stage 1 A）的 SHA")
        if canonical(docs[(1, side, COHORT)].get("provenance")) != canonical(docs[(1, side, AFTER)].get("provenance")):
            problems.append(f"Stage 1 {side}：cohort 與 after 的 provenance 不同")
    # 逐檔的封閉正規化 ＋ 逐位元比對（成對：同一個 stage 的 A 對 B）
    compared = []
    for stage, names in STAGE_FILES.items():
        for name in names:
            na, nb = normalize(name, docs[(stage, "A", name)]), normalize(name, docs[(stage, "B", name)])
            same = canonical(na) == canonical(nb)
            compared.append({"file": name, "stage": stage, "identical_after_normalization": same,
                             "removed": list(REMOVABLE[name])})
            if not same:
                problems.append(f"Stage {stage} {name}：正規化之後 A ≠ B：{_first_diff(na, nb)}")
    # provenance 的期望值（成對；另驗同一側跨 stage 的一致性）
    for stage, name in ((1, AFTER), (2, COMPARISON)):
        problems.extend(provenance_problems(f"Stage {stage} {name}", docs[(stage, "A", name)].get("provenance"),
                                            docs[(stage, "B", name)].get("provenance"), expected_tooling, base_commit))
    for side in ("A", "B"):
        p1, p2 = docs[(1, side, AFTER)].get("provenance") or {}, docs[(2, side, COMPARISON)].get("provenance") or {}
        for field in ("tooling_patch_sha256", "base_commit", "image_digest"):
            if p1.get(field) != p2.get(field):
                problems.append(f"{side}：Stage 1 與 Stage 2 的 provenance.{field} 不同")
    rows = len(docs[(1, "A", AFTER)].get("rows") or [])
    return {"ok": not problems, "problems": problems, "compared": compared, "after_rows": rows,
            "comparison_rows": len(docs[(2, "A", COMPARISON)].get("rows") or [])}


def main(argv: Optional[List[str]] = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0], allow_abbrev=False)
    for opt in ("--stage1-a", "--stage1-b", "--stage2-a", "--stage2-b"):
        parser.add_argument(opt, required=True, type=Path)
    parser.add_argument("--expected-tooling-sha256", required=True)
    parser.add_argument("--base-commit", required=True)
    parser.add_argument("--out", required=True, type=Path)
    args = parser.parse_args(argv)
    if not re.fullmatch(r"[0-9a-f]{64}", args.expected_tooling_sha256) or args.expected_tooling_sha256 == EMPTY_SHA256:
        print("ERROR: --expected-tooling-sha256 必須是非空 diff 的 64 碼 SHA-256", file=sys.stderr)
        return 1
    if not re.fullmatch(r"[0-9a-f]{40}", args.base_commit):
        print("ERROR: --base-commit 必須是 40 碼 OID", file=sys.stderr)
        return 1
    dirs = {(1, "A"): args.stage1_a, (1, "B"): args.stage1_b, (2, "A"): args.stage2_a, (2, "B"): args.stage2_b}
    try:
        result = run(dirs, args.expected_tooling_sha256, args.base_commit)
    except ToolingGuardError as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 1
    report = {"schema": REPORT_SCHEMA, **result}
    args.out.write_text(json.dumps(report, ensure_ascii=False, sort_keys=True, indent=1) + "\n", encoding="utf-8")
    if not result["ok"]:
        for problem in result["problems"]:
            print(f"ERROR: {problem}", file=sys.stderr)
        return 1
    print(f"OK: tooling 非語意 guard 通過（after {result['after_rows']} 列、comparison {result['comparison_rows']} 列）")
    return 0


if __name__ == "__main__":
    sys.exit(main())
