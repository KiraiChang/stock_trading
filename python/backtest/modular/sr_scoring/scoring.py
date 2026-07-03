"""
對外主入口：score_symbol(symbol, timeframe) -> dict，供 FastAPI /sr-zones
端點與 Go internal/analysis.Client.ScoreZones 呼叫。

【2026-07 機構級重新設計 R1】把這個功能從「描述市場」的資訊展示，改成能
「指導交易」的量化決策輸出。改動的核心問題與對應設計：

一、EV 計算方式錯誤
  舊版：expected_value = hold機率 × reward - break機率 × risk，其中 reward/
  risk 是「zone 寬度」這種結構性估計，跟「反彈時實際漲多少」脫鉤，導致
  Bounce=65%/Break=35%/Average Return=-1.6% 這種輸入卻算出反直覺的 EV。
  新版：expected_value = bounce機率 × average_bounce_return +
                          break機率 × average_break_return
  兩個 average_return 分別是「歷史上真的反彈的那些觸碰」與「歷史上真的
  跌破的那些觸碰」各自的平均報酬（見 features.py::average_bounce_break_returns），
  不再混合使用單一 avg_return_after_touch。

二、Average Return 不再是單一數字
  average_bounce_return（恆為正/0）與 average_break_return（恆為負/0）
  分開統計，取代舊版容易混淆「反彈」「跌破」的 avg_return_after_touch。

三、Risk Reward 補齊細節
  risk_reward_ratio = |expected_gain / expected_loss|
  （expected_gain/expected_loss 是角色解析後的 average_bounce_return/
  average_break_return），讓使用者同時看到 Reward、Risk、R 三個數字，不是
  只有一個抽象的 R 值。

四、Support/Resistance Score 加上 Net Score
  net_score = support_score - resistance_score，並依門檻分類成
  STRONG_SUPPORT/NEUTRAL/STRONG_RESISTANCE，避免只看單一分數判斷。

五、Confidence 改成多因子綜合
  confidence = (sample_factor + recency_factor + stability_factor) / 3
  sample_factor：觸碰次數的貝式收縮（樣本數）。
  recency_factor：距離最近一次觸碰的時間衰減（越久沒測試過，這個因子越低）。
  stability_factor：歷史結果（守住 vs 跌破）的一致性。
  confidence_level 是 0~30/30~60/60~80/80~100 的分級結果。

六、Recent Validation 取代 Pending Validation
  依「最近一次觸碰的時間」與「有沒有守住」判斷 VALIDATED_RECENTLY /
  PENDING_VALIDATION / NOT_TESTED_RECENTLY / EXPIRED 四種狀態。

七、新增 Reward/Risk Percentile
  目前 risk_reward_ratio 在訓練資料集歷史 RR 分佈中的百分位（見
  model.py::reward_risk_percentile，參考分佈存在 ModelBundle.rr_reference）。

八、新增 Volume Confirmation
  依角色解析後的 relative_volume 與最近驗證結果分類。

九、Trend / Volatility 移出 Zone 層級
  這兩個本質上是股票層級的量（同一次分析裡所有 zone 算出來都一樣），只
  在 score_symbol() 算一次，放在回傳值的 global_trend/global_volatility，
  不再對每個 zone 重複輸出同一個數字。新增真正逐 zone 不同的
  zone_momentum/zone_direction，取代原本被誤用來代表「zone 趨勢」的股票
  層級 trend_strength。

【2026-07 機構級重新設計 R2】在 R1 基礎上進一步收斂：

十、只有一個 Global Model：Global Trend/Volatility/EV/Confidence/RR
  score_symbol() 只用同一份 get_model() 單例（本來就是——見 model.py 的
  lazy singleton 設計），輸出裡新增 global_expected_value/global_confidence/
  global_risk_reward_ratio，跟已經存在的 global_trend/global_volatility
  （R1 的 overall_trend/overall_volatility 改名）放在一起，構成單一、
  權威的「整體評估」區塊，取代「要看哪個 zone 才代表這檔股票」的曖昧。

十一、Zone 必須可排序：Tier 1（主結構）/ Tier 2（交易區）/ Tier 3（短期支撐）
  zones 依寬度（price_high - price_low）在同一次分析裡的相對排名分三層
  （見 _assign_tiers），最寬的三分之一是 Tier 1（宏觀主結構），最窄的
  三分之一是 Tier 3（短期戰術支撐/壓力）。回傳的 zones 陣列依 tier 由粗到
  細排序，同一層內依 trading_score 由高到低排序，不再是無序清單。

十二、EV 必須唯一收斂：Final EV = Σ(zone_EV × weight)
  global_expected_value 用 confidence 當權重，對所有「有明確方向」（role
  非 AT_ZONE、expected_value 非 None）的 zone 做加權平均（見
  _compute_global_metrics）。global_risk_reward_ratio 用同一套權重比照
  辦理。global_confidence 是所有 zone confidence 的簡單平均（不分角色，
  反映整體結構的可信程度，不用 confidence 加權自己）。

十三、Score 必須可拆解：Score = EV(40%) + RR(20%) + Trend(15%) + Volume(15%) + Confidence(10%)
  trading_score 不再是 7 個分量的黑盒公式（R1 版本混合了 role_score 與
  momentum），改成明確的 5 個分量、明確的權重，且每個分量的加權貢獻值都
  存在 trading_score_breakdown 裡一起回傳，使用者可以逐項檢視分數怎麼來
  的，而不是只看到一個總分（見十四、1. 可解釋）。

模型未訓練時 get_model() 會拋 RuntimeError，這裡刻意不 catch —— 讓
/sr-zones 在模型就緒前明確失敗（fail-fast），而不是靜默回傳中性機率。
"""
from __future__ import annotations

from typing import Any, Optional

import numpy as np
import pandas as pd

from db import fetch_candles

from .features import compute_zone_features, find_touches, trend_slope, zone_momentum, zone_volatility
from .labeling import label_touch
from .model import (
    ModelBundle,
    get_model,
    predict_break_probability,
    predict_hold_probability,
    reward_risk_percentile,
)
from .types import (
    ApproachDirection,
    ConfidenceLevel,
    NetScoreLabel,
    RecentValidation,
    TradingRecommendation,
    VolumeConfirmation,
    Zone,
    ZoneDirection,
    ZoneScore,
    ZoneTier,
    ZoneTouch,
    ZoneType,
)
from .zone_builder import ATRZoneBuilder, VolumeProfileZoneBuilder, ZoneBuilder

DEFAULT_FETCH_LIMIT = 250

# 跟 dataset.py::DatasetConfig 的預設值保持一致，讓「訓練時怎麼分類觸碰結果」
# 跟「即時評分時怎麼分類」用同一套標準（十四、2. 不互相矛盾）。
DEFAULT_FORWARD_BARS = 5
DEFAULT_THRESHOLD_PCT = 0.03
DEFAULT_ZONE_LOOKBACK_BARS = 60

# Confidence 三因子的可調參數
CONFIDENCE_SAMPLE_PSEUDO_COUNT = 5  # 貝式收縮虛擬樣本數：touch_count=5 時 sample_factor=0.5
CONFIDENCE_RECENCY_HALFLIFE_BARS = 40  # 每過 40 根，recency_factor 減半

# Recent Validation 的時間窗口
RECENT_VALIDATION_WINDOW_BARS = 20
STALE_VALIDATION_WINDOW_BARS = 60

# Volume Confirmation 的相對量能門檻
VOLUME_CONFIRMATION_HIGH = 1.2
VOLUME_CONFIRMATION_LOW = 0.8

# Net Score 分類門檻
NET_SCORE_STRONG_THRESHOLD = 0.15

# Zone Direction 分類門檻（zone_momentum 的絕對值超過此值才算有明確方向）
ZONE_DIRECTION_THRESHOLD = 0.01
ZONE_MOMENTUM_LOOKBACK = 5

NEUTRAL_PROBABILITY = 0.5

# 十三、Score 必須可拆解：五個分量、明確權重，總和 = 100。
TRADING_SCORE_WEIGHTS = {
    "expected_value": 40.0,
    "risk_reward": 20.0,
    "trend": 15.0,
    "volume": 15.0,
    "confidence": 10.0,
}

_VOLUME_CONFIRMATION_WEIGHT = {
    VolumeConfirmation.CONFIRMED.value: 1.0,
    VolumeConfirmation.NEUTRAL.value: 0.5,
    VolumeConfirmation.WEAK.value: 0.3,
    VolumeConfirmation.FAILED.value: 0.0,
}

TIER_LABEL_TEXT = {
    ZoneTier.TIER_1_MAIN_STRUCTURE.value: "主結構",
    ZoneTier.TIER_2_TRADING_ZONE.value: "交易區",
    ZoneTier.TIER_3_SHORT_TERM.value: "短期支撐",
}


def _to_dataframe(rows: list[dict]) -> pd.DataFrame:
    df = pd.DataFrame(rows)
    df["datetime"] = pd.to_datetime(df["timestamp"], unit="s", utc=True)
    df = df.set_index("datetime").sort_index()
    return df[["open", "high", "low", "close", "volume"]].astype(float)


def _default_builders() -> list[ZoneBuilder]:
    return [ATRZoneBuilder(), VolumeProfileZoneBuilder()]


# ── 機率正規化 / score 推導 ──────────────────────────────────


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


def _net_score_label(net_score: float, threshold: float = NET_SCORE_STRONG_THRESHOLD) -> str:
    if net_score >= threshold:
        return NetScoreLabel.STRONG_SUPPORT.value
    if net_score <= -threshold:
        return NetScoreLabel.STRONG_RESISTANCE.value
    return NetScoreLabel.NEUTRAL.value


# ── Confidence（多因子）────────────────────────────────────────


def _sample_factor(touch_count: int, pseudo_count: int = CONFIDENCE_SAMPLE_PSEUDO_COUNT) -> float:
    """觸碰次數越少，這個因子越低（貝式收縮）。"""
    return float(touch_count / (touch_count + pseudo_count))


def _recency_factor(
    bars_since_last_touch: Optional[int], halflife: int = CONFIDENCE_RECENCY_HALFLIFE_BARS
) -> float:
    """距離最近一次觸碰越久，這個因子越低（time decay，每 halflife 根減半）；
    從未被觸碰過回傳 0（沒有任何驗證資訊）。"""
    if bars_since_last_touch is None:
        return 0.0
    return float(0.5 ** (bars_since_last_touch / halflife))


def _stability_factor(hold_count: int, break_count: int) -> float:
    """歷史結果一致性：同一種結果（守住 or 跌破）佔比越高，這個 zone 的
    行為越穩定可預期。沒有可判定的歷史結果時回傳中性值 0.5（無資訊）。"""
    total = hold_count + break_count
    if total == 0:
        return 0.5
    return float(max(hold_count, break_count) / total)


def _confidence(touch_count: int, bars_since_last_touch: Optional[int], hold_count: int, break_count: int) -> float:
    """三個因子（樣本數/時間衰減/歷史穩定度）等權重平均。刻意選擇簡單、
    可解釋的公式：任一因子偏低都會拖低整體 confidence，避免「觸碰次數夠多
    但都是很久以前」或「次數多但結果很不穩定」被誤判成高可信度。"""
    sample = _sample_factor(touch_count)
    recency = _recency_factor(bars_since_last_touch)
    stability = _stability_factor(hold_count, break_count)
    return float((sample + recency + stability) / 3.0)


def _confidence_level(confidence: float) -> str:
    pct = confidence * 100
    if pct < 30:
        return ConfidenceLevel.LOW.value
    if pct < 60:
        return ConfidenceLevel.MEDIUM.value
    if pct < 80:
        return ConfidenceLevel.HIGH.value
    return ConfidenceLevel.VERY_HIGH.value


# ── 觸碰結果分類（reuse labeling.py 的判定邏輯）──────────────────


def _classify_touches(
    df: pd.DataFrame,
    touches: list[ZoneTouch],
    forward_bars: int,
    threshold_pct: float,
    as_of_index: int,
    label_method: str = "max_excursion",
) -> list[tuple[ZoneTouch, int, int, float]]:
    """回傳 [(touch, hold_label, break_label, forward_return), ...]，只含
    touch_index+forward_bars <= as_of_index（有足夠未來資料可判定、且不會
    lookahead）的觸碰，依 touch_index 由舊到新排序（find_touches 本來就是
    照這個順序掃描，這裡沿用不重新排序）。"""
    classified: list[tuple[ZoneTouch, int, int, float]] = []
    for touch in touches:
        if touch.touch_index + forward_bars > as_of_index:
            continue
        result = label_touch(df, touch, forward_bars, threshold_pct, label_method)
        if result is None:
            continue
        hold_label, break_label, forward_return = result
        classified.append((touch, hold_label, break_label, forward_return))
    return classified


def _recent_validation(
    touches: list[ZoneTouch],
    classified: list[tuple[ZoneTouch, int, int, float]],
    as_of_index: int,
) -> str:
    if not touches:
        return RecentValidation.PENDING_VALIDATION.value  # 從未被觸碰過

    last_touch = touches[-1]
    if not classified or classified[-1][0].touch_index != last_touch.touch_index:
        return RecentValidation.PENDING_VALIDATION.value  # 最後一次觸碰太新，還不知道結果

    _, hold_label, break_label, _ = classified[-1]
    bars_since = as_of_index - last_touch.touch_index

    if break_label:
        return RecentValidation.EXPIRED.value
    if hold_label and bars_since <= RECENT_VALIDATION_WINDOW_BARS:
        return RecentValidation.VALIDATED_RECENTLY.value
    if bars_since > STALE_VALIDATION_WINDOW_BARS:
        return RecentValidation.NOT_TESTED_RECENTLY.value
    return RecentValidation.VALIDATED_RECENTLY.value if hold_label else RecentValidation.NOT_TESTED_RECENTLY.value


def _volume_confirmation(relative_volume: float, recent_validation: str) -> str:
    """判斷依據：角色解析後的 relative_volume（觸碰量相對均量）+ 最近一次
    驗證結果。高量且守住 → CONFIRMED；高量但跌破/突破 → FAILED（量能確認了
    失敗）；量能不足 → WEAK；其餘 → NEUTRAL。"""
    if recent_validation == RecentValidation.EXPIRED.value and relative_volume >= VOLUME_CONFIRMATION_HIGH:
        return VolumeConfirmation.FAILED.value
    if recent_validation == RecentValidation.VALIDATED_RECENTLY.value and relative_volume >= VOLUME_CONFIRMATION_HIGH:
        return VolumeConfirmation.CONFIRMED.value
    if relative_volume < VOLUME_CONFIRMATION_LOW:
        return VolumeConfirmation.WEAK.value
    return VolumeConfirmation.NEUTRAL.value


def _zone_direction(momentum: float, threshold: float = ZONE_DIRECTION_THRESHOLD) -> str:
    if momentum > threshold:
        return ZoneDirection.UP.value
    if momentum < -threshold:
        return ZoneDirection.DOWN.value
    return ZoneDirection.FLAT.value


# ── 十一、Zone Tier（可排序）──────────────────────────────────


def _assign_tiers(widths: list[float]) -> list[str]:
    """依寬度（zone.price_high - zone.price_low）分三個 tier：最寬的 1/3
    是 Tier 1（主結構，涵蓋範圍最大的宏觀結構），中間 1/3 是 Tier 2
    （交易區），最窄的 1/3 是 Tier 3（短期支撐，最貼近盤中操作的精確價位）。
    用同一批 zone 的寬度分佈做相對分組（tercile），不用絕對門檻——不同
    股票的價格尺度差異很大，絕對寬度沒有可比性。回傳值跟輸入 widths 同順序
    對應（不是排序後的結果）。"""
    n = len(widths)
    if n == 0:
        return []
    order = sorted(range(n), key=lambda i: widths[i], reverse=True)
    third = -(-n // 3)  # ceil(n/3)
    tiers = [""] * n
    for rank, idx in enumerate(order):
        if rank < third:
            tiers[idx] = ZoneTier.TIER_1_MAIN_STRUCTURE.value
        elif rank < 2 * third:
            tiers[idx] = ZoneTier.TIER_2_TRADING_ZONE.value
        else:
            tiers[idx] = ZoneTier.TIER_3_SHORT_TERM.value
    return tiers


_TIER_ORDER = {
    ZoneTier.TIER_1_MAIN_STRUCTURE.value: 1,
    ZoneTier.TIER_2_TRADING_ZONE.value: 2,
    ZoneTier.TIER_3_SHORT_TERM.value: 3,
}


def _sort_zone_scores(zone_scores: list[ZoneScore]) -> list[ZoneScore]:
    """zones 必須「可排序」：先依 tier 由粗到細（主結構→交易區→短期支撐），
    同一層內再依 trading_score 由高到低，取代原本無序的清單。"""
    return sorted(zone_scores, key=lambda z: (_TIER_ORDER.get(z.tier, 99), -z.trading_score))


# ── 十三、Trading Score（可拆解）/ Trading Recommendation ────────


def _normalize_signed(value: float, cap: float) -> float:
    """把可正可負的訊號（例如 EV、trend）正規化到 [0,1]，0.5 為中性、+cap
    以上算滿分、-cap 以下算 0 分。"""
    return float(max(0.0, min(1.0, 0.5 + value / (2 * cap))))


def _trading_score_breakdown(
    role: str,
    confidence: float,
    expected_value: Optional[float],
    risk_reward_ratio: Optional[float],
    overall_trend: float,
    volume_confirmation: Optional[str],
) -> dict[str, float]:
    """Score = EV(40%) + RR(20%) + Trend(15%) + Volume(15%) + Confidence(10%)。
    每個分量先正規化到 [0,1] 再乘上對應權重，回傳值就是「這個分量對總分的
    實際貢獻」（加總即為 trading_score），不是抽象的 0~1 子分數——這樣使用
    者可以直接看出「總分裡有幾分來自 EV、幾分來自量能」，不用自己再乘一次
    權重（十四、1. 可解釋、十四、8. 明確定義計算公式）。

    role=AT_ZONE 或角色相關數值缺值（EV/RR/量能確認都要求 role 已解析）時，
    對應分量用中性值 0.5 計算，不直接給 0 分——沒有方向不代表這個 zone
    「不好」，只是還沒有可以評分的方向性資料。Trend/Confidence 不需要角色
    解析，任何情況都能算。"""
    is_support = role == ZoneType.SUPPORT.value
    is_resistance = role == ZoneType.RESISTANCE.value

    ev_norm = _normalize_signed(expected_value, cap=0.05) if expected_value is not None else 0.5
    rr_norm = float(max(0.0, min(1.0, risk_reward_ratio / 3.0))) if risk_reward_ratio is not None else 0.5

    if is_support:
        trend_norm = _normalize_signed(overall_trend, cap=0.1)
    elif is_resistance:
        trend_norm = _normalize_signed(-overall_trend, cap=0.1)
    else:
        trend_norm = _normalize_signed(overall_trend, cap=0.1)  # AT_ZONE：沒有方向可對齊，用原始值

    volume_norm = _VOLUME_CONFIRMATION_WEIGHT.get(volume_confirmation, 0.5) if volume_confirmation else 0.5

    w = TRADING_SCORE_WEIGHTS
    return {
        "expected_value": float(ev_norm * w["expected_value"]),
        "risk_reward": float(rr_norm * w["risk_reward"]),
        "trend": float(trend_norm * w["trend"]),
        "volume": float(volume_norm * w["volume"]),
        "confidence": float(confidence * w["confidence"]),
    }


def _trading_score(breakdown: dict[str, float]) -> float:
    return float(sum(breakdown.values()))


def _trading_recommendation(trading_score: float, role: str) -> str:
    """role=SUPPORT：分數高代表『守住訊號強』= 偏多（買進）訊號。
    role=RESISTANCE：分數高代表『壓力守住訊號強』= 偏空（避開做多/放空）
    訊號。role=AT_ZONE：沒有明確方向，只給 WATCH/NEUTRAL。6 個類別
    （Strong Buy/Buy/Watch/Neutral/Avoid/Strong Sell）與非對稱門檻（只有
    Strong Sell、沒有單獨的 Sell）是需求文件原始定義，這裡照實作。"""
    if role == ZoneType.AT_ZONE.value:
        return TradingRecommendation.WATCH.value if trading_score >= 50 else TradingRecommendation.NEUTRAL.value

    if role == ZoneType.SUPPORT.value:
        if trading_score >= 80:
            return TradingRecommendation.STRONG_BUY.value
        if trading_score >= 60:
            return TradingRecommendation.BUY.value
        if trading_score >= 40:
            return TradingRecommendation.WATCH.value
        if trading_score >= 20:
            return TradingRecommendation.NEUTRAL.value
        return TradingRecommendation.AVOID.value

    # RESISTANCE
    if trading_score >= 80:
        return TradingRecommendation.STRONG_SELL.value
    if trading_score >= 60:
        return TradingRecommendation.AVOID.value
    if trading_score >= 40:
        return TradingRecommendation.NEUTRAL.value
    if trading_score >= 20:
        return TradingRecommendation.WATCH.value
    return TradingRecommendation.NEUTRAL.value


# ── 十、十二：Global Model（Global Trend/Volatility/EV/Confidence/RR）──


def _compute_global_metrics(zone_scores: list[ZoneScore]) -> dict[str, Optional[float]]:
    """十二、EV 必須唯一收斂：Final EV = Σ(zone_EV × weight)，weight 採用
    confidence（越可信的 zone，對整體 EV 的影響力越大）。global_risk_reward_ratio
    比照辦理。global_confidence 是所有 zone confidence 的簡單平均（不分
    角色，不用 confidence 加權自己，避免循環）。zones 為空、或都沒有明確
    方向（EV/RR 皆為 None）時，對應欄位回傳 None。"""
    if not zone_scores:
        return {"expected_value": None, "confidence": None, "risk_reward_ratio": None}

    global_confidence = float(np.mean([z.confidence for z in zone_scores]))

    ev_weight_sum = ev_weighted_sum = 0.0
    rr_weight_sum = rr_weighted_sum = 0.0
    for z in zone_scores:
        if z.expected_value is not None:
            ev_weighted_sum += z.expected_value * z.confidence
            ev_weight_sum += z.confidence
        if z.risk_reward_ratio is not None:
            rr_weighted_sum += z.risk_reward_ratio * z.confidence
            rr_weight_sum += z.confidence

    global_ev = float(ev_weighted_sum / ev_weight_sum) if ev_weight_sum > 0 else None
    global_rr = float(rr_weighted_sum / rr_weight_sum) if rr_weight_sum > 0 else None

    return {"expected_value": global_ev, "confidence": global_confidence, "risk_reward_ratio": global_rr}


# ── 主流程 ────────────────────────────────────────────────────


def score_zone(
    df: pd.DataFrame,
    zone: Zone,
    current_price: float,
    bundle: ModelBundle,
    overall_trend: float,
    tier: str = ZoneTier.TIER_2_TRADING_ZONE.value,
    as_of_index: Optional[int] = None,
    lookback_bars: int = DEFAULT_ZONE_LOOKBACK_BARS,
    forward_bars: int = DEFAULT_FORWARD_BARS,
    threshold_pct: float = DEFAULT_THRESHOLD_PCT,
) -> ZoneScore:
    if as_of_index is None:
        as_of_index = len(df) - 1

    features_as_support = compute_zone_features(
        df, zone, as_of_index=as_of_index, approach=ApproachDirection.FROM_ABOVE,
        lookback_bars=lookback_bars, forward_bars=forward_bars, threshold_pct=threshold_pct,
    )
    features_as_resistance = compute_zone_features(
        df, zone, as_of_index=as_of_index, approach=ApproachDirection.FROM_BELOW,
        lookback_bars=lookback_bars, forward_bars=forward_bars, threshold_pct=threshold_pct,
    )

    all_touches = find_touches(df, zone, as_of_index, lookback_bars)
    all_classified = _classify_touches(df, all_touches, forward_bars, threshold_pct, as_of_index)
    hold_count = sum(1 for _, hold_label, _, _ in all_classified if hold_label)
    break_count = sum(1 for _, _, break_label, _ in all_classified if break_label)
    bars_since_last_touch = (as_of_index - all_touches[-1].touch_index) if all_touches else None

    confidence = _confidence(features_as_support.touch_count, bars_since_last_touch, hold_count, break_count)
    confidence_level = _confidence_level(confidence)

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
    net_score = support_score - resistance_score
    net_score_label = _net_score_label(net_score)

    role = _resolve_role(zone, current_price)

    if role == ZoneType.AT_ZONE.value:
        role_touches = all_touches
        role_classified = all_classified
    else:
        approach = ApproachDirection.FROM_ABOVE if role == ZoneType.SUPPORT.value else ApproachDirection.FROM_BELOW
        role_touches = [t for t in all_touches if t.approach_direction == approach]
        role_classified = _classify_touches(df, role_touches, forward_bars, threshold_pct, as_of_index)
    recent_validation = _recent_validation(role_touches, role_classified, as_of_index)

    zone_momentum_value = zone_momentum(df, all_touches, lookback=ZONE_MOMENTUM_LOOKBACK)
    zone_direction = _zone_direction(zone_momentum_value)

    bounce_probability: Optional[float] = None
    break_probability: Optional[float] = None
    expected_gain: Optional[float] = None
    expected_loss: Optional[float] = None
    expected_value: Optional[float] = None
    risk_reward_ratio: Optional[float] = None
    reward_risk_percentile_value: Optional[float] = None
    relative_volume: Optional[float] = None
    volume_confirmation: Optional[str] = None
    reject_count: Optional[int] = None
    break_count_field: Optional[int] = None

    if role != ZoneType.AT_ZONE.value:
        is_support = role == ZoneType.SUPPORT.value
        role_features = features_as_support if is_support else features_as_resistance
        hold_p, break_p = (support_hold, support_break) if is_support else (resistance_hold, resistance_break)

        bounce_probability = hold_p
        break_probability = break_p
        expected_gain = role_features.average_bounce_return
        expected_loss = role_features.average_break_return
        # 一、修正 EV：不再用單一 average_return，改成 hold機率×平均反彈報酬 +
        # break機率×平均跌破報酬，兩個方向分開加權。
        expected_value = hold_p * expected_gain + break_p * expected_loss

        relative_volume = role_features.relative_volume
        reject_count = role_features.rejection_count
        break_count_field = role_features.breakout_count

        if expected_loss != 0:
            risk_reward_ratio = abs(expected_gain / expected_loss)
            reward_risk_percentile_value = reward_risk_percentile(bundle, risk_reward_ratio)

        volume_confirmation = _volume_confirmation(relative_volume, recent_validation)

    trading_score_breakdown = _trading_score_breakdown(
        role, confidence, expected_value, risk_reward_ratio, overall_trend, volume_confirmation,
    )
    trading_score_value = _trading_score(trading_score_breakdown)
    trading_recommendation = _trading_recommendation(trading_score_value, role)

    return ZoneScore(
        price_low=zone.price_low,
        price_high=zone.price_high,
        method=zone.method.value,
        role=role,
        tier=tier,
        tier_label=TIER_LABEL_TEXT.get(tier, tier),
        support_score=support_score,
        resistance_score=resistance_score,
        net_score=net_score,
        net_score_label=net_score_label,
        confidence=confidence,
        confidence_level=confidence_level,
        bounce_probability=bounce_probability,
        break_probability=break_probability,
        expected_gain=expected_gain,
        expected_loss=expected_loss,
        expected_value=expected_value,
        risk_reward_ratio=risk_reward_ratio,
        reward_risk_percentile=reward_risk_percentile_value,
        relative_volume=relative_volume,
        volume_confirmation=volume_confirmation,
        touch_count=features_as_support.touch_count,
        reject_count=reject_count,
        break_count=break_count_field,
        zone_momentum=zone_momentum_value,
        zone_direction=zone_direction,
        recent_validation=recent_validation,
        trading_score=trading_score_value,
        trading_score_breakdown=trading_score_breakdown,
        trading_recommendation=trading_recommendation,
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
    as_of_index = len(df) - 1
    bundle = get_model()  # 只有一個 Global Model：整份分析共用同一個已訓練好的模型單例

    # 九：Trend/Volatility 是股票層級的量，只算一次，不對每個 zone 重複輸出。
    global_trend = trend_slope(df, as_of_index)
    global_volatility = zone_volatility(df, as_of_index)

    # 十一：Zone 必須可排序，先依寬度分好 tier 再逐一評分。
    tiers = _assign_tiers([z.width for z in zones])

    zone_scores = [
        score_zone(df, zone, current_price, bundle, global_trend, tier=tier, as_of_index=as_of_index)
        for zone, tier in zip(zones, tiers)
    ]
    zone_scores = _sort_zone_scores(zone_scores)

    global_metrics = _compute_global_metrics(zone_scores)

    return {
        "symbol": symbol,
        "timeframe": timeframe,
        "analyzed_at": analyzed_at.isoformat(),
        "current_price": current_price,
        "global_trend": global_trend,
        "global_volatility": global_volatility,
        "global_expected_value": global_metrics["expected_value"],
        "global_confidence": global_metrics["confidence"],
        "global_risk_reward_ratio": global_metrics["risk_reward_ratio"],
        # 模型可追蹤性：ModelBundle 本身已經有這些欄位，只是先前沒有透過
        # score_symbol() 輸出，導致 Go ToStore() 只能把 model_version 寫成空
        # 字串。model_feature_names 主要供 API 診斷/測試使用，不一定要進 DB。
        "model_version": bundle.version,
        "model_trained_at": bundle.trained_at,
        "model_feature_names": bundle.feature_names,
        "zones": [_zone_score_to_dict(z) for z in zone_scores],
    }


def _zone_score_to_dict(z: ZoneScore) -> dict[str, Any]:
    return {
        "price_low": z.price_low,
        "price_high": z.price_high,
        "method": z.method,
        "role": z.role,
        "tier": z.tier,
        "tier_label": z.tier_label,
        "support_score": z.support_score,
        "resistance_score": z.resistance_score,
        "net_score": z.net_score,
        "net_score_label": z.net_score_label,
        "confidence": z.confidence,
        "confidence_level": z.confidence_level,
        "bounce_probability": z.bounce_probability,
        "break_probability": z.break_probability,
        "expected_gain": z.expected_gain,
        "expected_loss": z.expected_loss,
        "expected_value": z.expected_value,
        "risk_reward_ratio": z.risk_reward_ratio,
        "reward_risk_percentile": z.reward_risk_percentile,
        "relative_volume": z.relative_volume,
        "volume_confirmation": z.volume_confirmation,
        "touch_count": z.touch_count,
        "reject_count": z.reject_count,
        "break_count": z.break_count,
        "zone_momentum": z.zone_momentum,
        "zone_direction": z.zone_direction,
        "recent_validation": z.recent_validation,
        "trading_score": z.trading_score,
        "trading_score_breakdown": z.trading_score_breakdown,
        "trading_recommendation": z.trading_recommendation,
    }
