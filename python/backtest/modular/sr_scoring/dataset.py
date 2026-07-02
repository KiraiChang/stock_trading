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

import pandas as pd

from .features import compute_zone_features, touch_starting_at
from .labeling import label_touch
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


def build_training_rows(
    df: pd.DataFrame,
    symbol: str,
    timeframe: str,
    zone_builders: list[ZoneBuilder],
    config: DatasetConfig,
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
                forward_bars_for_return=forward_bars,
            )

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
                )
            )

    return rows


def build_training_dataset(
    sources: list[tuple[str, str, pd.DataFrame]],
    zone_builders: list[ZoneBuilder],
    config: DatasetConfig,
) -> pd.DataFrame:
    """彙整多個 (symbol, timeframe, df) 來源的訓練列成一份扁平 DataFrame。"""
    all_rows: list[ZoneLabel] = []
    for symbol, timeframe, df in sources:
        all_rows.extend(build_training_rows(df, symbol, timeframe, zone_builders, config))

    columns = [
        "symbol", "timeframe", "touch_time", "zone_price_low", "zone_price_high",
        "method", "role", "is_support", "touch_count", "rejection_count", "breakout_count",
        "avg_return_after_touch", "relative_volume", "volatility", "trend_strength",
        "forward_bars", "threshold_pct", "hold_label", "break_label", "forward_return",
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
            "avg_return_after_touch": row.features.avg_return_after_touch,
            "relative_volume": row.features.relative_volume,
            "volatility": row.features.volatility,
            "trend_strength": row.features.trend_strength,
            "forward_bars": row.forward_bars,
            "threshold_pct": row.threshold_pct,
            "hold_label": row.hold_label,
            "break_label": row.break_label,
            "forward_return": row.forward_return,
        }
        for row in all_rows
    ]
    return pd.DataFrame.from_records(records, columns=columns)


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
