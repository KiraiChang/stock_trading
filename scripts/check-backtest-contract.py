#!/usr/bin/env python3
"""回測的跨 runtime 契約是否一致（I-137）。由 frontend/scripts/test.sh 在最後呼叫（比照 check-job-names.sh）。

比對四件事，任何一項不一致就以非 0 結束：

1. **backtrader 策略名單**（只接受 1 檔）：python 的 `STRATEGY_MAP`（python/backtest/engine.py，準）、
   Go 的 `BacktraderStrategies`（backend/internal/backtest/validate.go）、前端的 `BACKTRADER_STRATEGIES`
   （frontend/src/lib/api/backtest.ts）。三份要是**相同的集合**（雙向）。
2. **標的數上限的預設值**：Go 的 `SetDefault("backtest.max_symbols", N)`、backend/config.yaml 與 python/config.yaml 的
   `backtest.max_symbols`。
3. **最大、最小日期**：python `job_errors.MAX_DATE`／`MIN_DATE` 與 Go `MaxDate`／`MinDate`。
4. **兩份案例檔**（兩邊的單元測試各自跑全部案例）：錯誤允許規則（python/tests/data/backtest_job_error_cases.json 與
   backend/internal/backtest/testdata/job_error_cases.json）、請求驗證（python/tests/data/backtest_request_validation_cases.json 與
   backend/internal/backtest/testdata/request_validation_cases.json），各自內容相同。

**為什麼是 repo 層的腳本**：要同時讀三個 runtime 的原始碼；pytest 的 container 只掛載 python/、
backend/scripts/test.sh 只掛載 backend/。這裡只做靜態的文字比對，⛔ 不 import 任何一邊的程式。
"""
from __future__ import annotations

import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
PY_ENGINE = ROOT / "python/backtest/engine.py"
PY_ERRORS = ROOT / "python/backtest/job_errors.py"
PY_CONFIG = ROOT / "python/config.yaml"
GO_VALIDATE = ROOT / "backend/internal/backtest/validate.go"
GO_CONFIG = ROOT / "backend/internal/config/config.go"
BE_CONFIG = ROOT / "backend/config.yaml"
TS_API = ROOT / "frontend/src/lib/api/backtest.ts"
PY_CASES = ROOT / "python/tests/data/backtest_job_error_cases.json"
GO_CASES = ROOT / "backend/internal/backtest/testdata/job_error_cases.json"
PY_REQ_CASES = ROOT / "python/tests/data/backtest_request_validation_cases.json"
GO_REQ_CASES = ROOT / "backend/internal/backtest/testdata/request_validation_cases.json"


def _block(text: str, start_re: str, end: str, what: str) -> str:
    m = re.search(start_re, text, re.S)
    if not m:
        raise SystemExit(f"找不到 {what}")
    body = text[m.end():]
    i = body.find(end)
    if i < 0:
        raise SystemExit(f"{what} 沒有結尾 {end!r}")
    return body[:i]


def _strip_comments(s: str, marker: str) -> str:
    return "\n".join(line.split(marker, 1)[0] for line in s.splitlines())


def _yaml_max_symbols(path: Path) -> int:
    text = path.read_text(encoding="utf-8")
    block = re.search(r"^backtest:\s*\n((?:[ \t]+.*\n?)+)", text, re.M)
    if not block:
        raise SystemExit(f"{path.relative_to(ROOT)} 沒有 backtest 區段")
    m = re.search(r"^[ \t]+max_symbols:[ \t]*([0-9]+)", block.group(1), re.M)
    if not m:
        raise SystemExit(f"{path.relative_to(ROOT)} 的 backtest 區段沒有 max_symbols")
    return int(m.group(1))


def main() -> int:
    errors: list[str] = []

    py_map = _strip_comments(_block(PY_ENGINE.read_text(encoding="utf-8"), r"STRATEGY_MAP[^=]*=\s*\{", "}", "python STRATEGY_MAP"), "#")
    py = set(re.findall(r'"([A-Za-z0-9_]+)"\s*:', py_map))
    go_map = _strip_comments(_block(GO_VALIDATE.read_text(encoding="utf-8"), r"var BacktraderStrategies = map\[string\]bool\{", "}", "Go BacktraderStrategies"), "//")
    go = set(re.findall(r'"([A-Za-z0-9_]+)"\s*:\s*true', go_map))
    ts_list = _strip_comments(_block(TS_API.read_text(encoding="utf-8"), r"export const BACKTRADER_STRATEGIES\s*=\s*\[", "]", "前端 BACKTRADER_STRATEGIES"), "//")
    ts = set(re.findall(r"'([A-Za-z0-9_]+)'", ts_list))
    if not py:
        errors.append("python 的 STRATEGY_MAP 是空的（解析失敗？）")
    for name, other in (("Go BacktraderStrategies", go), ("前端 BACKTRADER_STRATEGIES", ts)):
        if other != py:
            errors.append(f"{name} 跟 python 的 STRATEGY_MAP 不一致：多了 {sorted(other - py)}、少了 {sorted(py - other)}")

    m = re.search(r'viper\.SetDefault\("backtest\.max_symbols",\s*([0-9]+)\)', GO_CONFIG.read_text(encoding="utf-8"))
    go_default = int(m.group(1)) if m else None
    defaults = {"Go SetDefault": go_default, "backend/config.yaml": _yaml_max_symbols(BE_CONFIG),
                "python/config.yaml": _yaml_max_symbols(PY_CONFIG)}
    if go_default is None or len(set(defaults.values())) != 1:
        errors.append(f"backtest.max_symbols 的預設值不一致：{defaults}")

    for py_name, go_name in (("MAX_DATE", "MaxDate"), ("MIN_DATE", "MinDate")):
        m_py = re.search(rf'^{py_name}\s*=\s*"([0-9-]+)"', PY_ERRORS.read_text(encoding="utf-8"), re.M)
        m_go = re.search(rf'^const {go_name}\s*=\s*"([0-9-]+)"', GO_VALIDATE.read_text(encoding="utf-8"), re.M)
        if not m_py or not m_go or m_py.group(1) != m_go.group(1):
            errors.append(f"{py_name}／{go_name} 不一致：python={m_py and m_py.group(1)}、Go={m_go and m_go.group(1)}")
        if go_name == "MaxDate" and m_go:
            max_date = m_go.group(1)

    counts = []
    for a, b in ((PY_CASES, GO_CASES), (PY_REQ_CASES, GO_REQ_CASES)):
        ca = json.loads(a.read_text(encoding="utf-8"))
        cb = json.loads(b.read_text(encoding="utf-8"))
        if ca != cb:
            errors.append(f"案例檔內容不同：{a.relative_to(ROOT)} 與 {b.relative_to(ROOT)}")
        elif not ca:
            errors.append(f"案例檔是空的：{a.relative_to(ROOT)}")
        counts.append(len(ca))

    if errors:
        for e in errors:
            print(f"check-backtest-contract: {e}", file=sys.stderr)
        return 1
    print(f"==> check-backtest-contract：backtrader 名單 {sorted(py)}、max_symbols={go_default}、"
          f"MaxDate={max_date}、錯誤案例 {counts[0]} 筆、請求驗證案例 {counts[1]} 筆，三個 runtime 一致")
    return 0


if __name__ == "__main__":
    sys.exit(main())
