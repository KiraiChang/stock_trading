#!/usr/bin/env python3
"""建立（或 no-op 沿用）run identity。⚠️ 只由 `scripts/pin-replay-image.sh` 呼叫。

    ensure-i074-run-identity.py --path <p> --bundle-id <id> --image-id sha256:…

契約與 validator 一致：**stdout 只印 `expected_image_id` 一行**，其餘走 stderr；
⚠️ **非零結果時 stdout ⛔ 無輸出**——下游是靠 stdout 取 ID 的。

結束碼：0 ＝ 成功（含 no-op）；**3 ＝ durability 未確認**（檔案有效但落盤沒確認，
⛔ 不得刪除、重跑同一條指令會走 no-op 並重新 fsync）；1 ＝ 其他失敗。
"""
from __future__ import annotations

import argparse
import sys
from datetime import datetime, timezone
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from _i074_bootstrap import load_replay_bundle  # noqa: E402


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(allow_abbrev=False)
    # ⚠️ ⛔ **沒有 `--path` 覆寫**：v25 裁決「正式流程固定由 XDG 推導，測試改覆寫
    # `XDG_DATA_HOME`」——⛔ 「只給測試用」的路徑參數在實作上強制不了，等於後門。
    # ⚠️ **一律經 `load_bundle()` 取得 bundle_id**（三方相等 ＋ 完整 hash 驗證）——
    # ⛔ 不收「已經取好的值」：取目錄 basename／直接讀 manifest 是強度不同的做法。
    parser.add_argument("--bundle", required=True, help="bundle 目錄")
    parser.add_argument("--image-id", default=None,
                        help="建立時必填；`--peek` 時忽略（沿用既有值）")
    parser.add_argument("--print-path", action="store_true",
                        help="只印出 identity 的絕對路徑（給 shell 掛載用）")
    # ⚠️ **stage 是封閉列舉**，⛔ 不是路徑：只決定推導出哪一份 identity（Stage 2 用自己的，
    # 見 issue.md I-074 Stage 2 計畫書「二、⑤」）。預設 1，Stage 1 的既有用法逐字不變。
    parser.add_argument("--stage", type=int, choices=(1, 2), default=1)
    parser.add_argument("--peek", action="store_true",
                        help="⚠️ 既有才沿用：存在→印既有 ID 並**重新 fsync**（durability 的修復路徑）；"
                             "不存在→**exit 2**（呼叫端據此決定要不要 build）")
    args = parser.parse_args(argv)

    mods = load_replay_bundle()
    run_identity, publish = mods["run_identity"], mods["publish"]
    path = run_identity.default_run_identity_path(args.stage)
    if args.print_path:
        print(path)
        return 0
    bundle_id = mods["bundle"].load_bundle(Path(args.bundle)).bundle_id
    try:
        if args.peek:
            if not path.exists():
                # ⚠️ **exit 2**：⛔ 不是失敗，是「還沒 pin」——呼叫端據此去 build。
                print(f"run identity 尚未建立：{path}", file=sys.stderr)
                return 2
            existing = run_identity.load_run_identity(path)
            # ⚠️ 用**既有的** image id 再走一次 `ensure`：這樣 bundle_id 相符檢查與
            # **重新 fsync**（durability 的唯一修復路徑）都會跑到。
            payload = run_identity.ensure_run_identity(
                path,
                bundle_id=bundle_id,
                expected_image_id=existing["expected_image_id"],
                created_at=existing["created_at"],
            )
            print(payload["expected_image_id"])
            return 0
        if not args.image_id:
            print("ERROR: 建立 identity 時 --image-id 必填", file=sys.stderr)
            return 1
        payload = run_identity.ensure_run_identity(
            path,
            bundle_id=bundle_id,
            expected_image_id=args.image_id,
            created_at=datetime.now(timezone.utc).isoformat(),
        )
    except publish.DurabilityUnconfirmed as exc:
        # ⚠️ 檔案有效但落盤未確認——⛔ 不刪除，回專屬碼讓呼叫端分得出來。
        print(f"ERROR: {exc}", file=sys.stderr)
        return publish.EXIT_DURABILITY_UNCONFIRMED
    except Exception as exc:  # noqa: BLE001
        print(f"ERROR: {type(exc).__name__}: {exc}", file=sys.stderr)
        return 1
    print(payload["expected_image_id"])
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
