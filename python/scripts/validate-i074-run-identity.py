#!/usr/bin/env python3
"""I-074 Stage 1：**host 端**的 run identity 守門（在 `docker run` **之前**執行）。

用法：

    validate-i074-run-identity.py <identity 路徑> --expect-image-id sha256:… [--bundle <bundle 目錄>]

契約：

* **stdout**：成功時**只印 `expected_image_id` 一行**（machine-readable）；其餘訊息一律走 stderr。
* **exit code**：0 ＝ 通過；**非 0 ＝ 拒絕，且 stdout ⛔ 無輸出**——下游是靠 stdout 取 ID 的，
  失敗卻照印，它會拿著一個沒驗過的值繼續跑。
* `<identity 路徑>` 同時接受 **`.json`**（repo 外的協調檔）與 **`.json.gz`**（archived copy）。
* `--bundle` **可選**：**probe／D／D+1 必須傳**，validator 自己呼叫 `load_bundle()`
  （三方相等 ＋ 完整 hash 驗證）取得 `bundle_id` 再比對；
  ⚠️ **comparator／normal finalizer／recovery ⛔ 不傳**——它們手上沒有可信的 bundle 第二來源。

⚠️ **為什麼要有這支腳本**：`--recover-durability` 只讀 evidence 內的 `.json.gz`，而 identity
必須在 Docker 啟動**之前**比對。⛔ 不能「先用沒驗過的 image 啟動容器再檢查」，也⛔ 不能讓每支
shell 各自解析 gzip／schema（那是雙真相源）。所以由這支腳本在 host 上用**同一份**
`validate_run_identity()` 完成。

⚠️ **host 沒有 pandas**，而 `backtest/modular/sr_scoring/__init__.py` 會 import 它。因此這裡
**用最小 package context 繞過那個 `__init__`**——⛔ bootstrap 只能待在這個檔案裡，
⛔ 不得散落到 shell heredoc。這也是為什麼 `replay_bundle` 的這幾個模組被約束成
**實際執行路徑 dependency-light**。
"""
from __future__ import annotations

import argparse
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from _i074_bootstrap import load_replay_bundle  # noqa: E402


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(add_help=True, allow_abbrev=False)
    parser.add_argument("identity", help="run identity 路徑（.json 或 .json.gz）")
    parser.add_argument("--expect-image-id", required=True)
    parser.add_argument("--bundle", default=None,
                        help="probe／D／D+1 必須傳；comparator／finalizer／recovery ⛔ 不傳")
    args = parser.parse_args(argv)

    repo_python = Path(__file__).resolve().parent.parent
    mods = load_replay_bundle(repo_python)
    run_identity, bundle_mod = mods["run_identity"], mods["bundle"]

    identity = run_identity.load_run_identity(args.identity)

    expected = identity["expected_image_id"]
    if expected != args.expect_image_id:
        print(
            f"ERROR: identity 記的 expected_image_id={expected} 與本次的 "
            f"{args.expect_image_id} 不符——⛔ 三趟必須用同一個 image。",
            file=sys.stderr,
        )
        return 1

    if args.bundle is not None:
        # ⚠️ **由 validator 自己載入 bundle**，⛔ 不收「已經取好的 bundle_id」：
        # 取目錄 basename／直接讀 manifest／經正式 loader 是三種強度不同的做法，
        # 只有正式 loader 做三方相等與完整 hash 驗證。順帶消除 shell 再讀一次的 TOCTOU。
        loaded = bundle_mod.load_bundle(Path(args.bundle))
        if loaded.bundle_id != identity["bundle_id"]:
            print(
                f"ERROR: bundle 的 bundle_id={loaded.bundle_id} 與 identity 記的 "
                f"{identity['bundle_id']} 不符。",
                file=sys.stderr,
            )
            return 1

    # ⚠️ 成功才輸出，且只有這一行。
    print(expected)
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except SystemExit:
        raise
    except Exception as exc:  # noqa: BLE001 - 任何驗證失敗都要以非零退出，且 stdout 無輸出
        print(f"ERROR: {type(exc).__name__}: {exc}", file=sys.stderr)
        raise SystemExit(1) from exc
