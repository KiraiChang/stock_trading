"""
對外主入口：score_symbol(symbol, timeframe) -> dict，供 FastAPI /sr-zones
端點與 Go internal/analysis.Client.ScoreZones 呼叫。

流程：抓K棒 → 用 ATR / Volume Profile 兩種方法建立 zone → 對每個 zone
分別算「作為支撐」與「作為壓力」的歷史特徵 → 規則式分數
（support_score/resistance_score，永遠可算，不需訓練模型）→ 依現價判斷
角色 → 套用訓練好的模型算 bounce_probability/break_probability（角色為
AT_ZONE 時為 None）。

模型未訓練時 get_model() 會拋 RuntimeError，這裡刻意不 catch —— 讓
/sr-zones 在模型就緒前明確失敗（fail-fast），而不是靜默回傳中性機率。
"""
from __future__ import annotations

from typing import Any, Optional

import pandas as pd

from db import fetch_candles

from .features import compute_zone_features
from .model import ModelBundle, get_model, predict_break_probability, predict_hold_probability
from .types import ApproachDirection, Zone, ZoneFeatures, ZoneScore, ZoneType
from .zone_builder import ATRZoneBuilder, VolumeProfileZoneBuilder, ZoneBuilder

DEFAULT_FETCH_LIMIT = 250


def _to_dataframe(rows: list[dict]) -> pd.DataFrame:
    df = pd.DataFrame(rows)
    df["datetime"] = pd.to_datetime(df["timestamp"], unit="s", utc=True)
    df = df.set_index("datetime").sort_index()
    return df[["open", "high", "low", "close", "volume"]].astype(float)


def _default_builders() -> list[ZoneBuilder]:
    return [ATRZoneBuilder(), VolumeProfileZoneBuilder()]


def _rule_score(
    features: Optional[ZoneFeatures],
    direction: str,
    touch_count_norm_cap: int = 5,
    relative_volume_norm_cap: float = 2.0,
) -> float:
    """規則式強度分數，永遠可算（不需訓練資料）。權重為明確標註的可調
    heuristic，非規格強制：rejection 比率 0.4 + touch_count 正規化 0.25 +
    relative_volume 正規化 0.2 + 趨勢同向加成 0.15。"""
    if features is None or features.touch_count == 0:
        return 0.0

    rejection_ratio = min(1.0, features.rejection_count / features.touch_count)
    touch_norm = min(1.0, features.touch_count / touch_count_norm_cap)
    volume_norm = min(1.0, max(0.0, features.relative_volume) / relative_volume_norm_cap)
    trend_bonus = (
        max(0.0, min(1.0, features.trend_strength))
        if direction == "support"
        else max(0.0, min(1.0, -features.trend_strength))
    )

    score = 0.40 * rejection_ratio + 0.25 * touch_norm + 0.20 * volume_norm + 0.15 * trend_bonus
    return float(max(0.0, min(1.0, score)))


def _resolve_role(zone: Zone, current_price: float) -> str:
    if current_price > zone.price_high:
        return ZoneType.SUPPORT.value
    if current_price < zone.price_low:
        return ZoneType.RESISTANCE.value
    return ZoneType.AT_ZONE.value


def score_zone(df: pd.DataFrame, zone: Zone, current_price: float, bundle: ModelBundle) -> ZoneScore:
    as_of_index = len(df) - 1

    features_as_support = compute_zone_features(
        df, zone, as_of_index=as_of_index, approach=ApproachDirection.FROM_ABOVE
    )
    features_as_resistance = compute_zone_features(
        df, zone, as_of_index=as_of_index, approach=ApproachDirection.FROM_BELOW
    )

    support_score = _rule_score(features_as_support, "support")
    resistance_score = _rule_score(features_as_resistance, "resistance")
    role = _resolve_role(zone, current_price)

    bounce_probability: Optional[float] = None
    break_probability: Optional[float] = None
    if role != ZoneType.AT_ZONE.value:
        is_support = role == ZoneType.SUPPORT.value
        role_features = features_as_support if is_support else features_as_resistance
        bounce_probability = predict_hold_probability(bundle, role_features, is_support)
        break_probability = predict_break_probability(bundle, role_features, is_support)

    return ZoneScore(
        price_low=zone.price_low,
        price_high=zone.price_high,
        method=zone.method.value,
        role=role,
        support_score=support_score,
        resistance_score=resistance_score,
        bounce_probability=bounce_probability,
        break_probability=break_probability,
        features_as_support=features_as_support,
        features_as_resistance=features_as_resistance,
    )


def score_symbol(
    symbol: str, timeframe: str = "1d", builders: Optional[list[ZoneBuilder]] = None
) -> dict[str, Any]:
    rows = fetch_candles(symbol, timeframe, limit=DEFAULT_FETCH_LIMIT)
    if not rows:
        raise ValueError(f"no candles found for symbol={symbol} timeframe={timeframe}")

    df = _to_dataframe(rows)
    builders = builders or _default_builders()
    min_bars = max(b.min_bars for b in builders)
    if len(df) < min_bars:
        raise ValueError(
            f"not enough candles for sr_scoring: symbol={symbol} got={len(df)}, need>={min_bars}"
        )

    zones: list[Zone] = []
    for builder in builders:
        zones.extend(builder.build(df))

    current_price = float(df["close"].iloc[-1])
    analyzed_at = df.index[-1]
    bundle = get_model()

    zone_scores = [score_zone(df, zone, current_price, bundle) for zone in zones]

    return {
        "symbol": symbol,
        "timeframe": timeframe,
        "analyzed_at": analyzed_at.isoformat(),
        "current_price": current_price,
        "zones": [_zone_score_to_dict(z) for z in zone_scores],
    }


def _features_to_dict(features: Optional[ZoneFeatures]) -> Optional[dict[str, Any]]:
    if features is None:
        return None
    return {
        "touch_count": features.touch_count,
        "rejection_count": features.rejection_count,
        "breakout_count": features.breakout_count,
        "avg_return_after_touch": features.avg_return_after_touch,
        "relative_volume": features.relative_volume,
        "volatility": features.volatility,
        "trend_strength": features.trend_strength,
    }


def _zone_score_to_dict(z: ZoneScore) -> dict[str, Any]:
    return {
        "price_low": z.price_low,
        "price_high": z.price_high,
        "method": z.method,
        "role": z.role,
        "support_score": z.support_score,
        "resistance_score": z.resistance_score,
        "bounce_probability": z.bounce_probability,
        "break_probability": z.break_probability,
        "features_as_support": _features_to_dict(z.features_as_support),
        "features_as_resistance": _features_to_dict(z.features_as_resistance),
    }
