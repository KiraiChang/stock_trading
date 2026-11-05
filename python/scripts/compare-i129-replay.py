#!/usr/bin/env python3
"""I-129 的離線影響量測：逐列比較 commit A（改前）與 commit B（改後）的 Stage 1 after artifact。

用法（在 python image 裡執行；bundle 目錄的名稱必須等於 bundle_id）：
  python scripts/compare-i129-replay.py --bundle <bundle 目錄> \\
      --a <A 的 after_artifact.json> --b <B 的 after_artifact.json> \\
      --a-commit <A 的完整 commit> --b-commit <B 的完整 commit> --output <report.json>
  兩份 artifact 的 provenance.base_commit 必須分別等於 --a-commit／--b-commit（⛔ 不符就⛔ 不出結論）。
  --mode probe（預設）：量試單的設計（計畫書「6.5」）；--mode fallback：驗收退回版（「6.7」，⛔ 不判績效）。

結束碼（依報告的 `verdict`）：
  0 PASS                         放寬可以上 live（仍要走部署計畫）
  0 FALLBACK_ACCEPTED            退回版通過驗收（--mode fallback）
  3 EVIDENCE_INSUFFICIENT／SAFETY_FAILED   退回「不放行」（計畫書「4.5」）
  4 INVARIANT_VIOLATION          程式錯誤，停下來修
  5 UNATTRIBUTABLE               輸入不一致或欄位檢查不過，⛔ 不出結論（含讀檔失敗）

判準與規則都在計畫書（docs/issue.md I-129 指向的 plans/i129-execution-gate-authority.md「6.5」），
實作在 `backtest/modular/sr_scoring/i129_measurement.py`。
"""
from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

EXIT_BY_VERDICT = {
    "PASS": 0,
    "FALLBACK_ACCEPTED": 0,
    "EVIDENCE_INSUFFICIENT": 3,
    "SAFETY_FAILED": 3,
    "INVARIANT_VIOLATION": 4,
    "UNATTRIBUTABLE": 5,
}


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(allow_abbrev=False)
    parser.add_argument("--bundle", required=True, help="凍結 bundle 目錄（basename = bundle_id）")
    parser.add_argument("--a", required=True, help="commit A（改前）的 Stage 1 after artifact")
    parser.add_argument("--b", required=True, help="commit B（改後）的 Stage 1 after artifact")
    parser.add_argument("--a-commit", required=True, help="commit A 的完整 OID（40 字元）")
    parser.add_argument("--b-commit", required=True, help="commit B 的完整 OID（40 字元）")
    parser.add_argument("--output", required=True, help="報告輸出路徑（⛔ 不覆寫既有檔案）")
    parser.add_argument("--mode", choices=("probe", "fallback"), default="probe",
                        help="probe：量試單的設計（「6.5」）；fallback：驗收退回版（「6.7」）")
    args = parser.parse_args(argv)

    from backtest.modular.sr_scoring.i129_measurement import (
        build_candle_index,
        bundle_universe_keys,
        compare_artifacts,
    )
    from backtest.modular.sr_scoring.replay_bundle import ArtifactError, BundleError, load_bundle

    output = Path(args.output)
    if output.exists():
        print(f"中止：{output} 已存在（⛔ 不覆寫）", file=sys.stderr)
        return 5
    try:
        loaded = load_bundle(Path(args.bundle))
        report = compare_artifacts(
            args.a, args.b, build_candle_index(loaded.candles), bundle_id=loaded.bundle_id,
            expected_a_commit=args.a_commit, expected_b_commit=args.b_commit, mode=args.mode,
            universe=bundle_universe_keys(loaded),
        )
    except (ArtifactError, BundleError) as exc:
        # 讀檔、驗證失敗：⛔ 不出結論
        print(f"UNATTRIBUTABLE：{exc}", file=sys.stderr)
        return 5
    with open(output, "x", encoding="utf-8") as fh:
        json.dump(report, fh, ensure_ascii=False, indent=2, sort_keys=True)
        fh.write("\n")
    print(f"verdict={report['verdict']}")
    for reason in report["verdict_reasons"]:
        print(f"  - {reason}")
    print(f"報告：{output}")
    return EXIT_BY_VERDICT[report["verdict"]]


if __name__ == "__main__":
    sys.exit(main())
