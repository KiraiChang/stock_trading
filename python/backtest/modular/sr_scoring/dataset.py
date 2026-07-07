"""
Walk-forward 組裝訓練資料集：對每一根K棒，用「至今」的資料重建 zone 集合、
偵測新觸碰事件、算出當下特徵與未來 label，彙整成一份可直接餵給
sklearn 的 flat DataFrame。

zone 集合每 rebuild_every_bars 根才重建一次（效能考量：zone_builder.build()
會做 pivot/histogram 掃描，不需要每根都重跑），但觸碰偵測仍逐根進行，
確保不會漏掉任兩次 rebuild 之間發生的觸碰。
"""
from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any

import pandas as pd

from .features import compute_zone_features, touch_starting_at
from .labeling import label_touch
from .model import FEATURE_COLUMNS, chip_features_from_score_row, compute_rr_reference
from .types import Zone, ZoneLabel, ZoneType
from .zone_builder import ZoneBuilder

REQUIRED_CSV_COLUMNS = {"timestamp", "open", "high", "low", "close", "volume"}


@dataclass
class DatasetConfig:
    forward_bars_support: int = 5
    forward_bars_resistance: int = 5
    threshold_pct_support: float = 0.03
    threshold_pct_resistance: float = 0.03
    zone_lookback_bars: int = 60
    rebuild_every_bars: int = 5
    min_history_bars: int = 80
    label_method: str = "max_excursion"


def _chip_lookup_for_touch(chip_rows: list[dict] | None, touch_time: object) -> dict[str, float]:
    if not chip_rows:
        return chip_features_from_score_row(None)

    ts = pd.Timestamp(touch_time)
    if ts.tzinfo is None:
        ts = ts.tz_localize("UTC")
    touch_date = ts.tz_convert("Asia/Taipei").date()
    latest: dict | None = None
    for row in chip_rows:
        row_date = pd.Timestamp(row["trade_date"]).date()
        if row_date <= touch_date:
            latest = row
        else:
            break
    return chip_features_from_score_row(latest)


def build_training_rows(
    df: pd.DataFrame,
    symbol: str,
    timeframe: str,
    zone_builders: list[ZoneBuilder],
    config: DatasetConfig,
    chip_rows: list[dict] | None = None,
) -> list[ZoneLabel]:
    n = len(df)
    if n <= config.min_history_bars:
        return []

    rows: list[ZoneLabel] = []
    zones: list[Zone] = []

    for i in range(config.min_history_bars, n):
        if not zones or (i - config.min_history_bars) % config.rebuild_every_bars == 0:
            available = df.iloc[: i + 1]
            zones = [
                zone
                for builder in zone_builders
                if len(available) >= builder.min_bars
                for zone in builder.build(available)
            ]

        for zone in zones:
            touch = touch_starting_at(df, zone, i)
            if touch is None:
                continue

            is_support = touch.role == ZoneType.SUPPORT
            forward_bars = config.forward_bars_support if is_support else config.forward_bars_resistance
            threshold_pct = config.threshold_pct_support if is_support else config.threshold_pct_resistance

            label = label_touch(df, touch, forward_bars, threshold_pct, config.label_method)
            if label is None:
                continue
            hold_label, break_label, forward_return = label

            features = compute_zone_features(
                df,
                zone,
                as_of_index=i,
                approach=touch.approach_direction,
                lookback_bars=config.zone_lookback_bars,
                forward_bars=forward_bars,
                threshold_pct=threshold_pct,
                label_method=config.label_method,
            )

            chip_features = _chip_lookup_for_touch(chip_rows, touch.touch_time)

            rows.append(
                ZoneLabel(
                    symbol=symbol,
                    timeframe=timeframe,
                    touch_time=touch.touch_time,
                    zone_price_low=zone.price_low,
                    zone_price_high=zone.price_high,
                    method=zone.method,
                    role=touch.role,
                    features=features,
                    forward_bars=forward_bars,
                    threshold_pct=threshold_pct,
                    hold_label=hold_label,
                    break_label=break_label,
                    forward_return=forward_return,
                    chip_features=chip_features,
                )
            )

    return rows


def build_training_dataset(
    sources: list[tuple[str, str, pd.DataFrame]],
    zone_builders: list[ZoneBuilder],
    config: DatasetConfig,
    chip_scores_by_symbol: dict[str, list[dict]] | None = None,
) -> pd.DataFrame:
    """彙整多個 (symbol, timeframe, df) 來源的訓練列成一份扁平 DataFrame。"""
    all_rows: list[ZoneLabel] = []
    chip_scores_by_symbol = chip_scores_by_symbol or {}
    for symbol, timeframe, df in sources:
        all_rows.extend(
            build_training_rows(
                df, symbol, timeframe, zone_builders, config,
                chip_rows=chip_scores_by_symbol.get(symbol),
            )
        )

    columns = [
        "symbol", "timeframe", "touch_time", "zone_price_low", "zone_price_high",
        "method", "role", "is_support", "touch_count", "rejection_count", "breakout_count",
        "average_bounce_return", "average_break_return", "relative_volume", "volatility", "trend_strength",
        "forward_bars", "threshold_pct", "hold_label", "break_label", "forward_return",
        "chip_total_score", "chip_institutional_score", "chip_margin_score",
        "chip_broker_score", "chip_concentration_score", "chip_missing",
    ]
    if not all_rows:
        return pd.DataFrame(columns=columns)

    records = [
        {
            "symbol": row.symbol,
            "timeframe": row.timeframe,
            "touch_time": row.touch_time,
            "zone_price_low": row.zone_price_low,
            "zone_price_high": row.zone_price_high,
            "method": row.method.value,
            "role": row.role.value,
            "is_support": 1 if row.role == ZoneType.SUPPORT else 0,
            "touch_count": row.features.touch_count,
            "rejection_count": row.features.rejection_count,
            "breakout_count": row.features.breakout_count,
            "average_bounce_return": row.features.average_bounce_return,
            "average_break_return": row.features.average_break_return,
            "relative_volume": row.features.relative_volume,
            "volatility": row.features.volatility,
            "trend_strength": row.features.trend_strength,
            "forward_bars": row.forward_bars,
            "threshold_pct": row.threshold_pct,
            "hold_label": row.hold_label,
            "break_label": row.break_label,
            "forward_return": row.forward_return,
            **row.chip_features,
        }
        for row in all_rows
    ]
    return pd.DataFrame.from_records(records, columns=columns)


def summarize_training_dataset(dataset: pd.DataFrame) -> dict[str, Any]:
    """訓練資料診斷摘要：判斷「這次訓練出來的模型可不可信」用，不是給
    train_model() 本身用的（不影響訓練結果，只是報告）。例如資料集中在少數
    幾檔股票、或某個特徵幾乎永遠是 0/缺值，模型的泛化能力就值得懷疑，但
    單看 accuracy/AUC 這類整體指標看不出這件事。"""
    if dataset.empty:
        return {
            "rows": 0, "rows_by_symbol": {}, "role_counts": {},
            "hold_positive_rate": None, "break_positive_rate": None,
            "feature_zero_rate": {}, "rr_reference_count": 0,
        }

    rows_by_symbol = {str(k): int(v) for k, v in dataset["symbol"].value_counts().to_dict().items()}
    role_counts = {str(k): int(v) for k, v in dataset["role"].value_counts().to_dict().items()}
    feature_zero_rate = {
        col: float((dataset[col] == 0).mean()) for col in FEATURE_COLUMNS if col in dataset.columns
    }

    return {
        "rows": int(len(dataset)),
        "rows_by_symbol": rows_by_symbol,
        "role_counts": role_counts,
        "hold_positive_rate": float(dataset["hold_label"].mean()),
        "break_positive_rate": float(dataset["break_label"].mean()),
        "feature_zero_rate": feature_zero_rate,
        "rr_reference_count": len(compute_rr_reference(dataset)),
    }


def load_ohlcv_csv(path: str, symbol: str = "", timeframe: str = "1d") -> pd.DataFrame:
    """讀取 CSV（欄位：timestamp,open,high,low,close,volume），對齊
    analysis.py::_to_dataframe 的慣例：datetime index、升冪排序、float 欄位。
    timestamp 可為 unix 秒數或 ISO 字串。"""
    raw = pd.read_csv(path)
    missing = REQUIRED_CSV_COLUMNS - set(raw.columns)
    if missing:
        raise ValueError(f"CSV 缺少必要欄位: {sorted(missing)}")

    if pd.api.types.is_numeric_dtype(raw["timestamp"]):
        dt = pd.to_datetime(raw["timestamp"], unit="s", utc=True)
    else:
        dt = pd.to_datetime(raw["timestamp"], utc=True)

    df = raw.assign(datetime=dt).set_index("datetime").sort_index()
    return df[["open", "high", "low", "close", "volume"]].astype(float)
