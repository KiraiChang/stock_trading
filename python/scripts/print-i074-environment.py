#!/usr/bin/env python3
"""I-074 Stage 2：印出執行環境的套件指紋（`pin-replay-image.sh --stage 2 --adopt-image` 用）。

用法：

    print-i074-environment.py --stage1    # host：已封存的 Stage 1 D+1 after 當時的環境
    print-i074-environment.py --current   # 容器內：這個 image 的環境

stdout **只印一行 canonical JSON**：`{"pip_freeze_sha256": …, "python_version": …}`；失敗時 stdout ⛔ 無輸出。

⚠️ **為什麼要比這兩欄**：Stage 1 釘住的 image 已不在本機（issue.md I-074 Stage 2 計畫書「二、⑤」）。
2026-09-23 實測 `stock_trading-python-server:latest`（與遺失的 image 同一份 Dockerfile、只晚 16 分鐘
build）的 `pip_freeze_sha256` 與 Stage 1 **完全相同**，而今天重新 build 的會裝到不同版本——所以
Stage 2 **採用既有 image**，⛔ 不重新 build。採用前必須證明它的環境確實等於 Stage 1 當時的環境。

⚠️ 兩種模式用**同一套**計算：`--stage1` 取自 `load_stage1_anchor()` 已驗證的 after provenance
（⛔ 不是直接讀檔）；`--current` 用 `provenance.py` 的 `pip_freeze_sha256()` 與 `sys.version`
——與 `build_provenance()` 寫進 provenance 的是**同一個來源**。
"""
from __future__ import annotations

import argparse
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from _i074_bootstrap import STAGE2_MODULES, load_replay_bundle  # noqa: E402


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(allow_abbrev=False)
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument("--stage1", action="store_true")
    mode.add_argument("--current", action="store_true")
    args = parser.parse_args(argv)

    repo_python = Path(__file__).resolve().parent.parent
    try:
        if args.stage1:
            mods = load_replay_bundle(repo_python, STAGE2_MODULES)
            prov = mods["stage2_evidence"].load_stage1_anchor(repo_python).after_top["provenance"]
            env = {"pip_freeze_sha256": prov["pip_freeze_sha256"], "python_version": prov["python_version"]}
        else:
            mods = load_replay_bundle(repo_python, ("canonical", "provenance"))
            env = {
                "pip_freeze_sha256": mods["provenance"].pip_freeze_sha256(),
                "python_version": sys.version.split()[0],
            }
        line = mods["canonical"].canonical_json_bytes(env).decode("utf-8")
    except Exception as exc:  # noqa: BLE001 - 任何失敗都 fail-closed，stdout ⛔ 無輸出
        print(f"ERROR: {type(exc).__name__}: {exc}", file=sys.stderr)
        return 1
    print(line)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
