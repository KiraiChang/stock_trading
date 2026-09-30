#!/usr/bin/env python3
"""I-074 Stage 2：取出合成守門要比對的**宣告值**（只由 `scripts/finalize-stage2-evidence.sh` 呼叫）。

用法（四選一）：

    i074-stage2-patch-claims.py --finalize-run-dir <run 目錄>   # --finalize：comparison 的 provenance ＋ 凍結 patch
    i074-stage2-patch-claims.py --failure-run-dir <run 目錄>    # --publish-failed-record：中繼檔的 provenance ＋ 凍結 patch
    i074-stage2-patch-claims.py --archive                       # --recover-durability：成功 archive 的 manifest
    i074-stage2-patch-claims.py --failed-record <record 目錄>   # --recover-failed-record：該份 record

stdout **只印一行**：`<base_commit> <counterfactual_sha256> <tooling_sha256> <composed_sha256>`；
⚠️ `--failed-record` 另外在尾端加第 5 個 token：record **宣告的** `counterfactual_semantic_sha256`
（shell 從 record 內的實際 patch 重算並比對它，相等才以 `--verified-counterfactual-semantic-sha256` 注入）。
失敗時 stdout ⛔ 無輸出、結束碼 1。

⚠️ **這只是「取出宣告」，⛔ 不是驗證**（見 `stage2_archive.patch_claims()`）：shell 拿它們在隔離
worktree 重建三個 SHA（issue.md I-074 ③ evidence contract「四之二」、F8-a），⛔ 任一不符即中止、
⛔ 不呼叫 Python finalizer；之後容器內的 Python 段仍會完整驗證全部內容。
⚠️ 只讀小檔（comparison、中繼檔、manifest、record），⛔ 不讀全量 before source。
"""
from __future__ import annotations

import argparse
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from _i074_bootstrap import STAGE2_ARCHIVE_MODULES, load_replay_bundle  # noqa: E402


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(allow_abbrev=False)
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument("--finalize-run-dir")
    mode.add_argument("--failure-run-dir")
    mode.add_argument("--archive", action="store_true")
    mode.add_argument("--failed-record")
    args = parser.parse_args(argv)

    repo_python = Path(__file__).resolve().parent.parent
    try:
        s2a = load_replay_bundle(repo_python, STAGE2_ARCHIVE_MODULES)["stage2_archive"]
        if args.finalize_run_dir:
            claims = s2a.patch_claims(mode="finalize", python_root=repo_python, run_dir=args.finalize_run_dir)
        elif args.failure_run_dir:
            claims = s2a.patch_claims(mode="failure", python_root=repo_python, run_dir=args.failure_run_dir)
        elif args.archive:
            claims = s2a.patch_claims(mode="archive", python_root=repo_python)
        else:
            claims = s2a.patch_claims(mode="failed-record", python_root=repo_python, record_dir=args.failed_record)
    except Exception as exc:  # noqa: BLE001 - 任何失敗都 fail-closed，stdout ⛔ 無輸出
        print(f"ERROR: 取不到合成守門的宣告值：{type(exc).__name__}: {exc}", file=sys.stderr)
        return 1
    fields = ["base_commit", "counterfactual_patch_sha256", "tooling_patch_sha256", "composed_sha256"]
    if args.failed_record:
        fields.append("counterfactual_semantic_sha256")
    print(" ".join(claims[f] for f in fields))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
