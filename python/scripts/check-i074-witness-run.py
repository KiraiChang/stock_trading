#!/usr/bin/env python3
"""I-074 Stage 2：after' 見證趟的 **E7 前置守門**（在 `docker run` **之前**執行）。

用法（只由 `scripts/run-replay-offline.sh` 呼叫）：

    check-i074-witness-run.py --base-commit <40 hex> --tooling-patch-sha256 <64 hex>

⚠️ **為什麼要在 replay 之前**（2026-09-23 review）：E7 規定見證趟必須跑**原始的 Stage 1 base**
（`e1cbbbd`）且⛔ 不套任何 patch；不符的那一趟⛔ 不是環境見證（rc=1、⛔ 不發布）。若只在
`envcheck` 發布時才擋，錯設 `AFTER_REF` 或殘留 `TOOLING_PATCH` 會**先燒完約 180 分鐘**才被拒——
而計次政策規定 after' 只有一趟。所以 runner 在建完 worktree、算完兩個值之後、Docker 之前
就用**同一套 Stage 1 信任錨**（`load_stage1_anchor()`，串流驗證 D+1 並對 manifest 的 SHA）比對。

⚠️ 這裡是**提早擋**，⛔ 不是唯一的一道：`envcheck` 發布與 recovery 仍各自完整執行 E7。

結束碼：0 ＝ 通過（stdout ⛔ 無輸出）；1 ＝ 拒絕或讀不到信任錨。
"""
from __future__ import annotations

import argparse
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from _i074_bootstrap import STAGE2_MODULES, load_replay_bundle  # noqa: E402


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(allow_abbrev=False)
    parser.add_argument("--base-commit", required=True)
    parser.add_argument("--tooling-patch-sha256", required=True)
    args = parser.parse_args(argv)

    repo_python = Path(__file__).resolve().parent.parent
    mods = load_replay_bundle(repo_python, STAGE2_MODULES)
    s2 = mods["stage2_evidence"]
    try:
        anchor = s2.load_stage1_anchor(repo_python)
    except Exception as exc:  # noqa: BLE001 - 任何讀不到／驗不過都一律 fail-closed
        print(f"ERROR: 讀不到 Stage 1 信任錨，⛔ 無法確認見證趟的版本：{type(exc).__name__}: {exc}",
              file=sys.stderr)
        return 1
    problems = []
    if args.base_commit != anchor.after_base_commit:
        problems.append(
            f"base_commit={args.base_commit} ≠ Stage 1 after 的 {anchor.after_base_commit}"
            "（見證趟必須跑原始的 Stage 1 base——檢查 AFTER_REF）"
        )
    if args.tooling_patch_sha256 != s2.EMPTY_SHA256:
        problems.append(
            f"tooling_patch_sha256={args.tooling_patch_sha256} ≠ 空字串的 SHA"
            "（見證趟⛔ 不得套任何 patch——檢查 TOOLING_PATCH）"
        )
    if problems:
        print("ERROR: E7——這一趟⛔ 不是合法的環境見證，⛔ 在 replay 之前中止：", file=sys.stderr)
        for problem in problems:
            print(f"       - {problem}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
