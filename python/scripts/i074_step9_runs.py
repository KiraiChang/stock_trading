#!/usr/bin/env python3
"""I-074 Stage 2 步驟 ⑨：完整驗證與 ⑧ baseline 的逐項比對（docs/issue.md「Stage 2 步驟 ⑨ 細部計畫 v1」「八」#8）。

baseline ＝ ⑧ 第三次全量執行的 `-v` log（版控在 `fixtures/i074_step9/step8_full_run3.log`）；它的 SHA-256、大小、
來源 tree 與項目數寫成下面的常數——檔案被換、截斷、空白、tree 錯，一律 fail-closed。

規則：
* 比對單位 ＝ pytest 的 node id、shell 段的 `ok`／`FAIL` 標籤（唯一的正規化：host unittest 標籤去掉 ` in <秒數>s`）；
* **只比兩邊都有的既有項目**：baseline 是 pass 的，本次⛔ 不得變成 fail／error／skip（baseline 是 skip 的維持 skip 或變 pass）；
  baseline 有、本次沒有 → fail；
* **新增的項目只接受明列的範圍**：node 的檔案 ∈ `NEW_TEST_FILES`、shell 標籤以 `step9：` 開頭——必須全數通過；
  範圍外的新項目 → fail；
* 本次的 log 也要完整（有摘要行、`collected N items` ＝ node 結果的行數）；⛔ 不比總測試數、⛔ 不逐位元比完整輸出。

只用標準庫。結束碼：0 ＝ 通過；1 ＝ 不通過或輸入不合法。
"""
from __future__ import annotations

import argparse
import hashlib
import json
import re
import sys
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple

BASELINE_PATH = Path(__file__).resolve().parent / "fixtures" / "i074_step9" / "step8_full_run3.log"
BASELINE_SHA256 = "901843273ad3ab6209777adcc2f5001ddb9be5e833980befef512de8a59488bb"
BASELINE_BYTES = 353982
BASELINE_TREE = "06d6ac7adad22eff41f7804aff4061d9e118eb59"
BASELINE_COUNTS = {"pytest_nodes": 2295, "pytest_passed": 2294, "pytest_skipped": 1, "shell_ok": 744, "shell_fail": 0}

NEW_TEST_FILES = (
    "backtest/modular/sr_scoring/tests/test_i074_differential_guard.py",
    "backtest/modular/sr_scoring/tests/test_i074_tooling_guard.py",
    "backtest/modular/sr_scoring/tests/test_i074_step9_runs.py",
)
NEW_LABEL_PREFIX = "step9："

NODE_RE = re.compile(r"^(\S+::.*) (PASSED|FAILED|SKIPPED|ERROR|XFAIL|XPASS) +\[ *\d+%\]$")
LABEL_RE = re.compile(r"^  (ok|FAIL) +(.*?)\s*$")
COLLECTED_RE = re.compile(r"^collecting \.\.\. collected (\d+) items")
SUMMARY_RE = re.compile(r"^=+ (.*\d+ (?:passed|failed).*) in [\d.]+s(?: \([\d:]+\))? =+$")
HOST_UNITTEST_RE = re.compile(r"^(host unittest：Ran \d+ tests) in \d+(?:\.\d+)?s$")


class RunsError(ValueError):
    pass


def normalize_label(label: str) -> str:
    m = HOST_UNITTEST_RE.fullmatch(label)
    return m.group(1) if m else label


def parse(text: str) -> Dict[str, Any]:
    nodes: Dict[str, str] = {}
    labels: Dict[str, str] = {}
    collected: List[int] = []
    summaries: List[str] = []
    for line in text.splitlines():
        m = NODE_RE.match(line)
        if m:
            if m.group(1) in nodes:
                raise RunsError(f"pytest node 重複：{m.group(1)}")
            nodes[m.group(1)] = m.group(2)
            continue
        m = LABEL_RE.match(line)
        if m:
            label = normalize_label(m.group(2))
            # 同一個標籤出現多次：任何一次 FAIL 就算 FAIL。
            labels[label] = "FAIL" if labels.get(label) == "FAIL" or m.group(1) == "FAIL" else "ok"
            continue
        m = COLLECTED_RE.match(line)
        if m:
            collected.append(int(m.group(1)))
            continue
        m = SUMMARY_RE.match(line)
        if m:
            summaries.append(m.group(1))
    return {"nodes": nodes, "labels": labels, "collected": collected, "summaries": summaries}


def check_complete(parsed: Dict[str, Any], what: str) -> List[str]:
    problems = []
    if len(parsed["summaries"]) != 1:
        problems.append(f"{what}：pytest 的摘要行必須恰好一行（實際 {len(parsed['summaries'])}）——log 可能被截斷")
    if parsed["collected"] != [len(parsed["nodes"])]:
        problems.append(f"{what}：collected {parsed['collected']} ≠ node 結果 {len(parsed['nodes'])} 行——log 可能被截斷")
    if not parsed["labels"]:
        problems.append(f"{what}：沒有任何 shell 標籤——log 可能是空白或只有 pytest")
    return problems


def load_baseline(path: Path = BASELINE_PATH) -> Dict[str, Any]:
    raw = path.read_bytes()
    if len(raw) != BASELINE_BYTES or hashlib.sha256(raw).hexdigest() != BASELINE_SHA256:
        raise RunsError(f"baseline 的大小或 SHA-256 不符（{len(raw)} bytes）——⛔ 不以別的 log 替代")
    parsed = parse(raw.decode("utf-8"))
    problems = check_complete(parsed, "baseline")
    counts = {
        "pytest_nodes": len(parsed["nodes"]),
        "pytest_passed": sum(1 for v in parsed["nodes"].values() if v == "PASSED"),
        "pytest_skipped": sum(1 for v in parsed["nodes"].values() if v == "SKIPPED"),
        "shell_ok": sum(1 for v in parsed["labels"].values() if v == "ok"),
        "shell_fail": sum(1 for v in parsed["labels"].values() if v == "FAIL"),
    }
    if counts != BASELINE_COUNTS:
        problems.append(f"baseline 的項目數 {counts} ≠ 記錄的 {BASELINE_COUNTS}")
    if problems:
        raise RunsError("；".join(problems))
    return parsed


def _new_item_in_scope(kind: str, key: str) -> bool:
    if kind == "node":
        return key.split("::", 1)[0] in NEW_TEST_FILES
    return key.startswith(NEW_LABEL_PREFIX)


def compare(baseline: Dict[str, Any], new: Dict[str, Any]) -> Dict[str, Any]:
    problems = check_complete(new, "本次")
    regressions: List[str] = []
    missing: List[str] = []
    out_of_scope: List[str] = []
    new_failed: List[str] = []
    new_items: List[str] = []
    for kind, field, ok_values in (("node", "nodes", ("PASSED",)), ("label", "labels", ("ok",))):
        base, cur = baseline[field], new[field]
        for key, outcome in base.items():
            if key not in cur:
                missing.append(f"{kind}：{key}")
            elif outcome in ok_values and cur[key] not in ok_values:
                regressions.append(f"{kind}：{key}（{outcome} → {cur[key]}）")
            elif kind == "node" and outcome == "SKIPPED" and cur[key] not in ("SKIPPED", "PASSED"):
                regressions.append(f"{kind}：{key}（SKIPPED → {cur[key]}）")
        for key, outcome in cur.items():
            if key in base:
                continue
            new_items.append(f"{kind}：{key}")
            if not _new_item_in_scope(kind, key):
                out_of_scope.append(f"{kind}：{key}")
            elif outcome not in ok_values:
                new_failed.append(f"{kind}：{key}（{outcome}）")
    for name, items in (("既有項目變差", regressions), ("baseline 有、本次沒有", missing),
                        ("範圍外的新項目", out_of_scope), ("新增項目沒有通過", new_failed)):
        if items:
            problems.append(f"{name}：{len(items)} 項")
    return {"ok": not problems, "problems": problems, "regressions": regressions, "missing": missing,
            "out_of_scope": out_of_scope, "new_failed": new_failed, "new_items": new_items,
            "counts": {"baseline_nodes": len(baseline["nodes"]), "baseline_labels": len(baseline["labels"]),
                       "new_nodes": len(new["nodes"]), "new_labels": len(new["labels"])}}


def main(argv: Optional[List[str]] = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0], allow_abbrev=False)
    parser.add_argument("--new-log", required=True, type=Path)
    parser.add_argument("--out", required=True, type=Path)
    args = parser.parse_args(argv)
    try:
        baseline = load_baseline()
        raw = args.new_log.read_bytes()
        if not raw:
            raise RunsError("本次的 log 是空的")
        result = compare(baseline, parse(raw.decode("utf-8")))
    except (OSError, UnicodeDecodeError, RunsError) as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 1
    report = {"schema": "i074_step9_full_compare_v1", "baseline_sha256": BASELINE_SHA256,
              "baseline_tree": BASELINE_TREE, **result}
    args.out.write_text(json.dumps(report, ensure_ascii=False, sort_keys=True, indent=1) + "\n", encoding="utf-8")
    if not result["ok"]:
        for problem in result["problems"]:
            print(f"ERROR: {problem}", file=sys.stderr)
        return 1
    print(f"OK: 與 ⑧ baseline 逐項比對通過（新增 {len(result['new_items'])} 項，全數通過）")
    return 0


if __name__ == "__main__":
    sys.exit(main())
