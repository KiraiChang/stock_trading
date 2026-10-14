#!/usr/bin/env python3
"""I-074 Stage 2 步驟 ⑨：differential guard 的 producer（docs/issue.md「Stage 2 步驟 ⑨ 細部計畫 v1」「四」）。

在**容器內**、以 `--source-root`（某一側 worktree 的 `python/`，唯讀掛在 `/app`）為**唯一**的產品程式來源，
對 fixture 的每一格呼叫 `build_decision_summary()`，輸出 canonical JSON（schema `i074_decision_summary_set_v1`）：
summary、fixture 與本檔的 SHA-256、Python 版本、`/app` 底下**實際載入**的每一個模組的 SHA-256。

⚠️ 以 `python -I -B` 執行（⛔ 不讀 `PYTHONPATH`、⛔ 不把腳本目錄放進 `sys.path`）；只把 `--source-root` 插在最前面。
⚠️ 只用該 worktree 的**公開**介面（`ZoneScore(**欄位)`、固定參數的 `ModelBundle`、`build_decision_summary(...)`）；
⛔ 不 monkeypatch、⛔ 不呼叫私有函式、⛔ 不改模組狀態。fixture 只當參數值，⛔ 不 `eval`、⛔ 不依它決定 import 什麼。
⚠️ fixture 的驗證與 canonical 輸出沿用同目錄 `i074_differential_guard.py` 的同一份函式——以**檔案路徑**載入，
⛔ 不把這個目錄放進 `sys.path`。
"""
from __future__ import annotations

import argparse
import hashlib
import importlib.util
import json
import sys
from pathlib import Path
from typing import Any, Dict, List, Optional

GUARD_FILE = Path(__file__).resolve().parent / "i074_differential_guard.py"
PRODUCER_SCHEMA = "i074_decision_summary_set_v1"
SIDES = ("original", "patched")
POSITIONAL = ("current_price", "global_trend", "global_volatility", "global_metrics", "chip_summary")


def _load_guard():
    spec = importlib.util.spec_from_file_location("_i074_differential_guard_for_producer", GUARD_FILE)
    if spec is None or spec.loader is None:
        raise SystemExit(f"ERROR: 載入不了 {GUARD_FILE}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def _json_default(value: Any) -> Any:
    raise TypeError(f"decision summary 含有 JSON 無法表達的型別 {type(value).__name__}：{value!r}")


def build(fixture: dict, source_root: Path) -> List[Dict[str, Any]]:
    sys.path.insert(0, str(source_root))
    from backtest.modular.sr_scoring.decision_engine import build_decision_summary
    from backtest.modular.sr_scoring.model import ModelBundle
    from backtest.modular.sr_scoring.types import ZoneScore

    bundle = ModelBundle(**fixture["model_bundle"])
    out = []
    for case in fixture["cases"]:
        inputs = dict(fixture["input_defaults"])
        inputs.update(case["inputs"])
        zones = [ZoneScore(**dict(fixture["zone_defaults"], **zone)) for zone in inputs.pop("zones")]
        positional = [inputs.pop(name) for name in POSITIONAL]
        summary = build_decision_summary(zones, *positional, bundle, **inputs)
        # ⛔ 不用 default=str 偷偷轉型：出現 JSON 表達不了的型別就失敗。
        text = json.dumps(summary, ensure_ascii=False, sort_keys=True, allow_nan=False, default=_json_default)
        out.append({"id": case["id"], "summary": json.loads(text)})
    return out


def loaded_modules(source_root: Path) -> Dict[str, str]:
    root = source_root.resolve()
    mods: Dict[str, str] = {}
    for module in list(sys.modules.values()):
        file = getattr(module, "__file__", None)
        if not file:
            continue
        path = Path(file).resolve()
        try:
            rel = path.relative_to(root)
        except ValueError:
            continue
        mods[rel.as_posix()] = hashlib.sha256(path.read_bytes()).hexdigest()
    return dict(sorted(mods.items()))


def main(argv: Optional[List[str]] = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0], allow_abbrev=False)
    parser.add_argument("--fixture", required=True, type=Path)
    parser.add_argument("--source-root", required=True, type=Path)
    parser.add_argument("--side", required=True, choices=SIDES)
    parser.add_argument("--out", required=True, type=Path)
    args = parser.parse_args(argv)
    guard = _load_guard()
    try:
        fixture, fixture_raw = guard.load_json_strict(args.fixture)
        guard.validate_fixture(fixture)
    except guard.GuardInputError as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 1
    if not (args.source_root / "backtest" / "modular" / "sr_scoring").is_dir():
        print(f"ERROR: --source-root 不是 python/ 的根：{args.source_root}", file=sys.stderr)
        return 1
    cases = build(fixture, args.source_root)
    doc = {
        "schema": PRODUCER_SCHEMA,
        "side": args.side,
        "fixture_sha256": hashlib.sha256(fixture_raw).hexdigest(),
        "producer_sha256": hashlib.sha256(Path(__file__).resolve().read_bytes()).hexdigest(),
        "python_version": sys.version.split()[0],
        "loaded_modules": loaded_modules(args.source_root),
        "cases": cases,
    }
    args.out.write_text(guard.canonical_dump(doc), encoding="utf-8")
    print(f"OK: {args.side}：{len(cases)} 格、{len(doc['loaded_modules'])} 個模組")
    return 0


if __name__ == "__main__":
    sys.exit(main())
