"""I-074 Stage 2 反事實 smoke 的 fixture：從已進版控的正式 bundle 切出**一小段真實資料**。

⚠️ **為什麼⛔ 不用 `make_smoke_bundle.py`**：那份是單調上漲的合成 K 棒，⛔ 不會出現「跌破支撐再收復」，
幾乎不可能產生 candidate——反事實 smoke 會以空 cohort 過關，驗不到「after 有候選 → before 消掉 →
comparison 非空」。⚠️ 2026-09-30 實測：本切片在 after 側產生 6 筆候選（與 D+1 cohort 中 6243 在這段期間的
6 筆逐 key 相同），反事實程式碼跑同一份切片 → 候選 0、6 筆全數翻轉、其餘 159 列逐列相同
（issue.md I-074「Stage 2 步驟 ⑦a 細部計畫」B7）。

用法（在 smoke 的容器裡；`/app` 唯讀、只有輸出目錄可寫）：

    python scripts/make_counterfactual_smoke_bundle.py <來源 bundle 目錄> <輸出目錄>

成功時 stdout 最後一行印出衍生 bundle 的 ID（⛔ 任一守門不過即非零結束、stdout ⛔ 無 ID）。

⚠️ **用途界線**：這份衍生 bundle ⛔ **不是證據**。佔位 provenance 只用來**標示** smoke／非正式身分——
⛔ loader 不會拒絕它（`load_bundle()` 不讀 manifest 的 provenance，Stage 1 也不驗）。真正的界線是：
只存在於 smoke 的暫存目錄、smoke 結束即刪；⛔ 不進版控；⛔ 不進 finalize 與 `python/baselines/` 下的任何正式歸檔。
"""
from __future__ import annotations

import sys
from datetime import datetime, timezone
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from backtest.modular.sr_scoring.replay_bundle import (  # noqa: E402
    BundleError,
    build_manifest,
    build_provenance,
    emit_bundle,
    load_bundle,
    render_payloads,
)

SOURCE_BUNDLE_ID = "b1_20260901_1d_74350966_5d7ecb10"
SYMBOL = "6243"
BARS = 250
# ⚠️ 由內容決定（`captured_at` 與 provenance ⛔ 不影響 ID）；2026-09-30 以下列規則實測得出。
EXPECTED_BUNDLE_ID = "b1_20260901_1d_de3ab843_a7c9ffb4"
# 合法的全 0 佔位值（⛔ 不是 `make_smoke_bundle.py` 的 `"s" * 64`——那個通不過 `validate_provenance()`）。
PLACEHOLDER_IMAGE = "sha256:" + "0" * 64
PLACEHOLDER_RUNNER = "0" * 64


class FixtureError(ValueError):
    """切片 fixture 的守門失敗。"""


def _is_within(path: Path, root: Path) -> bool:
    return path == root or root in path.parents


def derive(source_dir: str | Path, out_dir: str | Path) -> str:
    """發布衍生 bundle 並回傳它的 ID。⚠️ 任一守門不過即拋 `FixtureError`／`BundleError`。"""
    source_dir = Path(source_dir).resolve()
    out_dir = Path(out_dir).resolve()
    if source_dir.name != SOURCE_BUNDLE_ID:
        raise FixtureError(f"來源必須是 {SOURCE_BUNDLE_ID}，實際 {source_dir.name}")
    # ⚠️ 在**任何寫入之前**驗：輸出⛔ 不得等於、也⛔ 不得位在來源 bundle 之內。
    if _is_within(out_dir, source_dir):
        raise FixtureError(f"輸出目錄 {out_dir} ⛔ 不得落在來源 bundle {source_dir} 之內")

    source = load_bundle(source_dir)                  # manifest SHA、逐檔 SHA、目錄內容、三方相等
    candles = source.candles.get(SYMBOL, [])[-BARS:]  # payload 已依 timestamp 排序
    if len(candles) != BARS:
        raise FixtureError(f"來源的 {SYMBOL} 不足 {BARS} 根：{len(candles)}")
    first_iso = datetime.fromtimestamp(candles[0]["timestamp"], timezone.utc).isoformat()
    replay_config = dict(source.replay_config, dataset_from=first_iso)   # ⚠️ 只改 dataset_from
    payloads = render_payloads(
        candles_by_symbol={SYMBOL: candles},
        chip_by_symbol={SYMBOL: source.chip.get(SYMBOL, [])},
        governance_by_symbol={SYMBOL: source.governance.get(SYMBOL, [])},
        replay_config=replay_config,
        trading_calendar=source.trading_calendar,
        model_bytes=source.model_path.read_bytes(),
    )
    m0 = source.manifest
    _bundle_id, manifest = build_manifest(
        payloads=payloads, as_of=m0["as_of"], timeframe=m0["timeframe"], symbols=[SYMBOL],
        limit=m0["limit"], replay_scope=m0["replay_scope"], report_max_rows=m0["report_max_rows"],
        # ⚠️ 資料真正的擷取時間（切片⛔ 沒有重新擷取）；也讓 manifest bytes 可重現。
        captured_at=m0["captured_at"],
        readiness=m0["readiness"], calendar=m0["calendar"],
        # ⛔ 不沿用來源的 provenance（那是原 Stage 0 的 argv 與 image）。
        provenance=build_provenance(
            source_root="/app", image_digest=PLACEHOLDER_IMAGE, base_commit=None,
            tooling_patch_sha256=None, runner_sha256=PLACEHOLDER_RUNNER,
            argv=["make_counterfactual_smoke_bundle.py", "--source", SOURCE_BUNDLE_ID,
                  "--symbol", SYMBOL, "--bars", str(BARS)],
        ),
    )
    out_dir.mkdir(parents=True, exist_ok=True)
    bundle_id = emit_bundle(out_dir, payloads, manifest, log=lambda message: None).bundle_id

    # 發布後自我驗證：重新 load_bundle()。
    derived = load_bundle(out_dir / bundle_id)
    if derived.bundle_id != EXPECTED_BUNDLE_ID:
        raise FixtureError(f"衍生 bundle 的 ID {derived.bundle_id} ≠ 記錄的 {EXPECTED_BUNDLE_ID}——切片規則漂移了")
    if derived.manifest["symbols"] != [SYMBOL] or set(derived.candles) != {SYMBOL}:
        raise FixtureError(f"衍生 bundle 必須只有 {SYMBOL}：{derived.manifest['symbols']}")
    rows = derived.candles[SYMBOL]
    if len(rows) != BARS or any(str(row.get("symbol")) != SYMBOL for row in rows):
        raise FixtureError(f"衍生 bundle 的 candles 必須恰好 {BARS} 根且全是 {SYMBOL}")
    return bundle_id


def main(argv: list[str] | None = None) -> int:
    args = list(sys.argv[1:] if argv is None else argv)
    if len(args) != 2:
        print("用法: make_counterfactual_smoke_bundle.py <來源 bundle 目錄> <輸出目錄>", file=sys.stderr)
        return 1
    try:
        bundle_id = derive(args[0], args[1])
    except (FixtureError, BundleError, OSError) as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 1
    print(bundle_id)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
