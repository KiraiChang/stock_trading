"""
sr_scoring 訓練 CLI。

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

import pandas as pd

from .dataset import DatasetConfig, build_training_dataset, load_ohlcv_csv
from .model import save_model, train_model
from .zone_builder import ATRZoneBuilder, VolumeProfileZoneBuilder


def _load_db_sources(symbols: str, timeframe: str, limit: int) -> list[tuple[str, str, pd.DataFrame]]:
    from db import fetch_candles

    sources: list[tuple[str, str, pd.DataFrame]] = []
    for symbol in symbols.split(","):
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


def main() -> None:
    parser = argparse.ArgumentParser(description="訓練 sr_scoring bounce/break 機率模型")
    parser.add_argument("--symbols", help="逗號分隔的股票代碼，從 DB 讀取")
    parser.add_argument("--csv", action="append", default=[], help="CSV 路徑，格式 path[:symbol]，可重複給多筆")
    parser.add_argument("--timeframe", default="1d")
    parser.add_argument("--limit", type=int, default=1500)
    parser.add_argument(
        "--model-type", default="gradient_boosting", choices=["gradient_boosting", "logistic_regression"]
    )
    parser.add_argument("--output", default=None, help="預設讀取 config.SR_SCORING_MODEL_PATH")
    parser.add_argument("--holdout-after", default=None, help="ISO 日期，訓練只使用此日期之前的資料")
    args = parser.parse_args()

    sources: list[tuple[str, str, pd.DataFrame]] = []
    if args.symbols:
        sources.extend(_load_db_sources(args.symbols, args.timeframe, args.limit))
    if args.csv:
        sources.extend(_load_csv_sources(args.csv, args.timeframe))

    if not sources:
        print("[error] 沒有任何可用的資料來源（請指定 --symbols 或 --csv）", file=sys.stderr)
        sys.exit(1)

    if args.holdout_after:
        cutoff = pd.Timestamp(args.holdout_after, tz="UTC")
        sources = [(sym, tf, df[df.index < cutoff]) for sym, tf, df in sources]

    dataset = build_training_dataset(
        sources, [ATRZoneBuilder(), VolumeProfileZoneBuilder()], DatasetConfig()
    )
    print(f"[info] 訓練資料集：{len(dataset)} 筆 touch 事件（來自 {len(sources)} 個來源）")
    if dataset.empty:
        print("[error] 資料集為空，無法訓練", file=sys.stderr)
        sys.exit(1)

    bundle = train_model(dataset, model_type=args.model_type)
    print(json.dumps(bundle.metrics, indent=2, ensure_ascii=False))

    output = args.output
    if output is None:
        import config

        output = config.SR_SCORING_MODEL_PATH
    save_model(bundle, output)
    print(f"[info] 模型已儲存：{output}")


if __name__ == "__main__":
    main()
