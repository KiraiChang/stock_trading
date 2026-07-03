"""
sr_scoring 訓練邏輯 + CLI。

run_training() 是核心可重用函式，被兩種入口共用：
  1. CLI：本檔案的 main()（python -m backtest.modular.sr_scoring.train ...）
  2. FastAPI：POST /sr-scoring/train（http_server.py），供 Go 端
     internal/api/handler/sr_zones.go 的 Train handler 觸發，讓使用者不用
     連進伺服器下指令就能重新訓練機率模型。

用法：
    cd python
    python -m backtest.modular.sr_scoring.train --symbols 2330,2454,0050 --timeframe 1d --limit 1500
    python -m backtest.modular.sr_scoring.train --csv data/2330.csv:2330

輸出：models/sr_scoring_<version>.joblib（sklearn Pipeline + metadata，
joblib 序列化，路徑預設取自 config.SR_SCORING_MODEL_PATH）。

train/holdout 切分改用 --holdout-after 在記憶體內依日期切片（不修改
db.py 既有的「只支援最新 N 根」限制，見 sr_scoring 套件的設計說明）。
"""
from __future__ import annotations

import argparse
import json
import sys
from typing import Any, Optional

import pandas as pd

from .dataset import DatasetConfig, build_training_dataset, load_ohlcv_csv, summarize_training_dataset
from .model import save_model, train_model
from .zone_builder import ATRZoneBuilder, VolumeProfileZoneBuilder

DEFAULT_TRAIN_LIMIT = 1500


def _load_db_sources(symbols: list[str], timeframe: str, limit: int) -> list[tuple[str, str, pd.DataFrame]]:
    from db import fetch_candles

    sources: list[tuple[str, str, pd.DataFrame]] = []
    for symbol in symbols:
        symbol = symbol.strip()
        if not symbol:
            continue
        rows = fetch_candles(symbol, timeframe, limit=limit)
        if not rows:
            print(f"[warn] no candles for symbol={symbol}, skipped", file=sys.stderr)
            continue
        df = pd.DataFrame(rows)
        df["datetime"] = pd.to_datetime(df["timestamp"], unit="s", utc=True)
        df = df.set_index("datetime").sort_index()
        df = df[["open", "high", "low", "close", "volume"]].astype(float)
        sources.append((symbol, timeframe, df))
    return sources


def _load_csv_sources(items: list[str], timeframe: str) -> list[tuple[str, str, pd.DataFrame]]:
    sources: list[tuple[str, str, pd.DataFrame]] = []
    for item in items:
        path, _, symbol = item.partition(":")
        symbol = symbol or path
        df = load_ohlcv_csv(path)
        sources.append((symbol, timeframe, df))
    return sources


def run_training(
    symbols: Optional[list[str]] = None,
    csv_sources: Optional[list[str]] = None,
    timeframe: str = "1d",
    limit: int = DEFAULT_TRAIN_LIMIT,
    model_type: str = "gradient_boosting",
    holdout_after: Optional[str] = None,
    output: Optional[str] = None,
    split_method: str = "time",
    calibration_method: Optional[str] = "sigmoid",
) -> dict[str, Any]:
    """組裝訓練資料、訓練模型、存檔，回傳可直接序列化成 JSON 的結果摘要。
    symbols/csv_sources 至少要有一個非空；資料集為空或沒有來源時拋
    ValueError（呼叫端——CLI 或 FastAPI handler——各自決定怎麼呈現錯誤）。"""
    sources: list[tuple[str, str, pd.DataFrame]] = []
    if symbols:
        sources.extend(_load_db_sources(symbols, timeframe, limit))
    if csv_sources:
        sources.extend(_load_csv_sources(csv_sources, timeframe))

    if not sources:
        raise ValueError("沒有任何可用的資料來源（請指定 symbols 或 csv）")

    if holdout_after:
        cutoff = pd.Timestamp(holdout_after, tz="UTC")
        sources = [(sym, tf, df[df.index < cutoff]) for sym, tf, df in sources]

    dataset = build_training_dataset(
        sources, [ATRZoneBuilder(), VolumeProfileZoneBuilder()], DatasetConfig()
    )
    if dataset.empty:
        raise ValueError("資料集為空，無法訓練（來源股票可能歷史資料太少，或都沒有偵測到 zone 觸碰事件）")

    dataset_summary = summarize_training_dataset(dataset)
    bundle = train_model(
        dataset, model_type=model_type, split_method=split_method, calibration_method=calibration_method
    )

    resolved_output = output
    if resolved_output is None:
        import config

        resolved_output = config.SR_SCORING_MODEL_PATH
    save_model(bundle, resolved_output)

    return {
        "rows": len(dataset),
        "sources": len(sources),
        "model_type": model_type,
        "split_method": bundle.split_method,
        "metrics": bundle.metrics,
        "model_path": resolved_output,
        "trained_at": bundle.trained_at,
        "version": bundle.version,
        "dataset_summary": dataset_summary,
    }


def main() -> None:
    parser = argparse.ArgumentParser(description="訓練 sr_scoring bounce/break 機率模型")
    parser.add_argument("--symbols", help="逗號分隔的股票代碼，從 DB 讀取")
    parser.add_argument("--csv", action="append", default=[], help="CSV 路徑，格式 path[:symbol]，可重複給多筆")
    parser.add_argument("--timeframe", default="1d")
    parser.add_argument("--limit", type=int, default=DEFAULT_TRAIN_LIMIT)
    parser.add_argument(
        "--model-type", default="gradient_boosting", choices=["gradient_boosting", "logistic_regression"]
    )
    parser.add_argument("--output", default=None, help="預設讀取 config.SR_SCORING_MODEL_PATH")
    parser.add_argument("--holdout-after", default=None, help="ISO 日期，訓練只使用此日期之前的資料")
    parser.add_argument(
        "--split-method", default="time", choices=["time", "random"],
        help="time（預設，依 touch_time 逐股票切分，避免用未來資料驗證過去）或 random（舊行為，保留供比較）",
    )
    parser.add_argument(
        "--calibration-method", default="sigmoid", choices=["sigmoid", "isotonic", "none"],
        help="機率校準方式，資料太少時會自動降級為不校準",
    )
    args = parser.parse_args()

    try:
        result = run_training(
            symbols=args.symbols.split(",") if args.symbols else None,
            csv_sources=args.csv or None,
            timeframe=args.timeframe,
            limit=args.limit,
            model_type=args.model_type,
            holdout_after=args.holdout_after,
            output=args.output,
            split_method=args.split_method,
            calibration_method=None if args.calibration_method == "none" else args.calibration_method,
        )
    except ValueError as exc:
        print(f"[error] {exc}", file=sys.stderr)
        sys.exit(1)

    print(f"[info] 訓練資料集：{result['rows']} 筆 touch 事件（來自 {result['sources']} 個來源，split_method={result['split_method']}）")
    print(json.dumps(result["metrics"], indent=2, ensure_ascii=False))
    print(f"[info] 模型已儲存：{result['model_path']}")
    print(f"[info] 資料集摘要：{json.dumps(result['dataset_summary'], indent=2, ensure_ascii=False)}")


if __name__ == "__main__":
    main()
