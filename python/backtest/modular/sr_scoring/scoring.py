"""
對外主入口：score_symbol(symbol, timeframe) -> dict，供 FastAPI /sr-zones
端點與 Go internal/analysis.Client.ScoreZones 呼叫。

流程：抓K棒 → 用 ATR / Volume Profile 兩種方法建立 zone → 對每個 zone
分別算「作為支撐」與「作為壓力」的歷史特徵 → 用訓練好的模型算出
hold/break 機率 → confidence（觸碰次數的貝式收縮）→ support_score /
resistance_score 由 confidence 收縮後的 hold 機率直接推導 → 依現價判斷
角色 → bounce_probability / break_probability / expected_value /
risk_reward_ratio（角色為 AT_ZONE 時皆為 None）。

【score 與 probability 的關係，2026-07 重新設計】
舊版 support_score/resistance_score 是獨立於機率模型之外的規則式加權公式，
量的是「歷史觸碰結構」；bounce_probability/break_probability 則是兩個獨立
訓練的二元分類器，量的是「未來走勢的預測」。兩者測的東西不同，數值上可能
互相矛盾（規則分數很高，但模型預測反彈機率很低），也可能同時給出不合理的
組合（bounce_probability + break_probability 加起來超過 100%，邏輯上不可能
同時高機率反彈又高機率跌破）。

現在改成：
  1. hold_model / break_model 的原始輸出先做等比例正規化，確保
     hold + break <= 1（_normalize_probabilities）。
  2. support_score / resistance_score 直接由（正規化後的）hold 機率經
     confidence 貝式收縮推導（_derive_score），不再是獨立公式——score 跟
     probability 永遠同方向，差異只在 confidence 高低造成的收縮幅度，不會
     再互相矛盾。
  3. confidence 只由觸碰次數決定（_confidence），觸碰次數越少，score 跟
     expected_value 都會被往中性值收縮，避免「只驗證過 1、2 次」的 zone
     因為那一兩次剛好都反彈，就被判成高分。

模型未訓練時 get_model() 會拋 RuntimeError，這裡刻意不 catch —— 讓
/sr-zones 在模型就緒前明確失敗（fail-fast），而不是靜默回傳中性機率。
這也代表 support_score/resistance_score 現在一定需要模型才能算，不再是
「永遠可算」的規則式分數——這是刻意的設計轉向：把這個功能從「描述市場」
推進到「指導交易」，數字背後一定要有校準過的機率支撐。
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

# 貝式收縮的虛擬樣本數（pseudo-count）：confidence = touch_count / (touch_count + K)。
# K 越大，需要越多觸碰次數 confidence 才會接近 1；K=5 代表觸碰 5 次時
# confidence=0.5，觸碰 20 次時 confidence≈0.8。可調，非規格強制。
CONFIDENCE_PSEUDO_COUNT = 5

NEUTRAL_PROBABILITY = 0.5


def _to_dataframe(rows: list[dict]) -> pd.DataFrame:
    df = pd.DataFrame(rows)
    df["datetime"] = pd.to_datetime(df["timestamp"], unit="s", utc=True)
    df = df.set_index("datetime").sort_index()
    return df[["open", "high", "low", "close", "volume"]].astype(float)


def _default_builders() -> list[ZoneBuilder]:
    return [ATRZoneBuilder(), VolumeProfileZoneBuilder()]


def _confidence(touch_count: int, pseudo_count: int = CONFIDENCE_PSEUDO_COUNT) -> float:
    """觸碰次數越少，confidence 越低（貝式收縮：touch_count=0 時 confidence=0，
    touch_count 越多 confidence 越接近 1），避免「尚未驗證」的 zone 因為
    樣本數太少就被判成高分或給出誇大的期望值。"""
    return float(touch_count / (touch_count + pseudo_count))


def _normalize_probabilities(hold_p: float, break_p: float) -> tuple[float, float]:
    """hold_model 與 break_model 是兩個獨立訓練的二元分類器，個別預測不保證
    hold_p + break_p <= 1（理論上可能同時輸出高機率，邏輯上矛盾：不可能同時
    高機率反彈又高機率跌破）。這裡做等比例正規化：加總超過 100% 時等比例
    縮小，讓兩者維持「至多其中一個發生」的合理上限；1 - hold_p - break_p
    隱含為「兩者皆未發生（盤整/不明確）」的機率。"""
    total = hold_p + break_p
    if total > 1.0:
        return hold_p / total, break_p / total
    return hold_p, break_p


def _derive_score(hold_probability: float, confidence: float, neutral: float = NEUTRAL_PROBABILITY) -> float:
    """score 直接由（正規化後的）hold 機率貝式收縮而來：confidence 高時
    score 趨近模型機率本身，confidence 低時往中性值 0.5 收縮。score 不再是
    獨立於機率之外的規則式公式，因此不會再跟 probability 互相矛盾。"""
    return float(confidence * hold_probability + (1 - confidence) * neutral)


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

    # touch_count 是聚合值（不分方向），兩邊 features 算出來的值相同，
    # confidence 用哪一邊都一樣，只需要算一次。
    confidence = _confidence(features_as_support.touch_count)

    support_hold, support_break = _normalize_probabilities(
        predict_hold_probability(bundle, features_as_support, is_support=True),
        predict_break_probability(bundle, features_as_support, is_support=True),
    )
    resistance_hold, resistance_break = _normalize_probabilities(
        predict_hold_probability(bundle, features_as_resistance, is_support=False),
        predict_break_probability(bundle, features_as_resistance, is_support=False),
    )

    support_score = _derive_score(support_hold, confidence)
    resistance_score = _derive_score(resistance_hold, confidence)

    role = _resolve_role(zone, current_price)

    bounce_probability: Optional[float] = None
    break_probability: Optional[float] = None
    expected_value: Optional[float] = None
    risk_reward_ratio: Optional[float] = None

    if role != ZoneType.AT_ZONE.value:
        is_support = role == ZoneType.SUPPORT.value
        role_features = features_as_support if is_support else features_as_resistance
        hold_p, break_p = (support_hold, support_break) if is_support else (resistance_hold, resistance_break)

        bounce_probability = hold_p
        break_probability = break_p

        # 報酬用這個 zone 自己「觸碰後平均報酬」的歷史經驗值；風險用 zone
        # 自身寬度相對現價的比例（價格跌破/漲破 zone 另一側邊界的估計損失），
        # 兩者都是這個 zone 既有的資料，不用額外去抓其他價位當停利/停損參考。
        reward_pct = abs(role_features.avg_return_after_touch)
        risk_pct = zone.width / current_price if current_price > 0 else 0.0
        if risk_pct > 0:
            risk_reward_ratio = float(reward_pct / risk_pct)
            raw_ev = hold_p * reward_pct - break_p * risk_pct
            # EV 也用 confidence 收縮：觸碰次數太少時，即使算出來的 EV 很
            # 誘人，也不該讓使用者看到一個「還沒被驗證過」的滿版期望值。
            expected_value = float(confidence * raw_ev)

    return ZoneScore(
        price_low=zone.price_low,
        price_high=zone.price_high,
        method=zone.method.value,
        role=role,
        support_score=support_score,
        resistance_score=resistance_score,
        confidence=confidence,
        bounce_probability=bounce_probability,
        break_probability=break_probability,
        expected_value=expected_value,
        risk_reward_ratio=risk_reward_ratio,
        features_as_support=features_as_support,
        features_as_resistance=features_as_resistance,
    )


def score_symbol(
    symbol: str,
    timeframe: str = "1d",
    limit: int = DEFAULT_FETCH_LIMIT,
    builders: Optional[list[ZoneBuilder]] = None,
) -> dict[str, Any]:
    """limit 為抓取的歷史K棒根數（不是天數），預設 DEFAULT_FETCH_LIMIT=250；
    呼叫端（FastAPI /sr-zones、Go handler）可覆寫。"""
    rows = fetch_candles(symbol, timeframe, limit=limit)
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
        "confidence": z.confidence,
        "bounce_probability": z.bounce_probability,
        "break_probability": z.break_probability,
        "expected_value": z.expected_value,
        "risk_reward_ratio": z.risk_reward_ratio,
        "features_as_support": _features_to_dict(z.features_as_support),
        "features_as_resistance": _features_to_dict(z.features_as_resistance),
    }
