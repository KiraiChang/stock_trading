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

十一、Zone 必須可排序：Tier 1（主結構）/ Tier 2（交易區）/ Tier 3（短期）
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

from db import fetch_candles, fetch_latest_chip_score

from .decision_engine import build_decision_summary
from .features import compute_zone_features, find_touches, trend_slope, zone_momentum, zone_volatility
from .labeling import label_touch
from .model import (
    ModelBundle,
    chip_features_from_score_row,
    get_model,
    neutral_chip_features,
    predict_break_probability,
    predict_hold_probability,
    reward_risk_percentile,
)
from .types import (
    ApproachDirection,
    ConfidenceLevel,
    NetScoreLabel,
    RecentValidation,
    VolumeConfirmation,
    Zone,
    ZoneDirection,
    ZoneScore,
    ZoneTier,
    ZoneTouch,
    ZoneType,
)
from .zone_builder import ATRZoneBuilder, RecentMicrostructureZoneBuilder, VolumeProfileZoneBuilder, ZoneBuilder
from .pipeline_types import ZoneFeatureSet
from .labels import TIER_LABEL_TEXT
from .ranking import (
    OVERLAP_GROUP_THRESHOLD,
    _assign_tiers,
    _evidence_family,
    _group_overlapping_zones,
    _sort_zone_scores,
    _zone_overlap_ratio,
)
from .scoring_rules import (
    TRADING_SCORE_WEIGHTS,
    TRADING_SCORE_WEIGHTS_NO_DIRECT_CHIP,
    _entry_relevance_breakdown,
    _entry_relevance_score,
    _trading_recommendation,
    _trading_score,
    _trading_score_breakdown,
    _trading_score_breakdown_no_direct_chip,
)
from .serialization import _zone_score_to_dict
from .summaries import _build_period_summaries, _pick_period_pair
from .tips import _build_analysis_tips

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

# 籌碼分數（chip_scores.total_score，-100~100）五段訊號門檻。SR Zone 以有效
# 影響分（effective_score）做方向判讀，避免低覆蓋率但單一分量極端時被解讀成
# 強訊號；_chip_direction 用弱門檻判斷是否已有明確方向。
CHIP_SIGNAL_WEAK_THRESHOLD = 10.0
CHIP_SIGNAL_STRONG_THRESHOLD = 30.0
CHIP_SIGNAL_THRESHOLD = CHIP_SIGNAL_WEAK_THRESHOLD
CHIP_COMPONENT_WEIGHTS = {
    "institutional_score": 0.35,
    "margin_score": 0.20,
    "broker_score": 0.30,
    "concentration_score": 0.15,
}

def _to_dataframe(rows: list[dict]) -> pd.DataFrame:
    df = pd.DataFrame(rows)
    df["datetime"] = pd.to_datetime(df["timestamp"], unit="s", utc=True)
    df = df.set_index("datetime").sort_index()
    return df[["open", "high", "low", "close", "volume"]].astype(float)


def _default_builders() -> list[ZoneBuilder]:
    return [ATRZoneBuilder(), VolumeProfileZoneBuilder(), RecentMicrostructureZoneBuilder()]


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


def _date_at_index(df: pd.DataFrame, index: Optional[int]) -> Optional[str]:
    if index is None or index < 0 or index >= len(df.index):
        return None
    value = df.index[index]
    if hasattr(value, "date"):
        return value.date().isoformat()
    return str(value)


def _validation_debug_metadata(
    df: pd.DataFrame,
    zone: Zone,
    touches: list[ZoneTouch],
    classified: list[tuple[ZoneTouch, int, int, float]],
    as_of_index: int,
    validation_start_index: int,
) -> dict[str, Any]:
    latest_touch = touches[-1] if touches else None
    latest_classified = classified[-1][0] if classified else None
    return {
        "analysis_date": _date_at_index(df, as_of_index),
        "zone_generation_end_date": _date_at_index(df, zone.formed_at_index),
        "validation_start_date": _date_at_index(df, validation_start_index),
        "validation_end_date": _date_at_index(df, as_of_index),
        "latest_touch_bar_date": _date_at_index(df, latest_touch.touch_index if latest_touch else None),
        "latest_validation_bar_date": _date_at_index(df, latest_classified.touch_index if latest_classified else None),
        "latest_touch_index": latest_touch.touch_index if latest_touch else None,
        "latest_validation_index": latest_classified.touch_index if latest_classified else None,
    }


def _filter_validation_touches(
    touches: list[ZoneTouch],
    classified: list[tuple[ZoneTouch, int, int, float]],
    zone: Zone,
) -> tuple[list[ZoneTouch], list[tuple[ZoneTouch, int, int, float]]]:
    filtered_touches = [touch for touch in touches if touch.touch_index > zone.formed_at_index]
    valid_touch_indexes = {touch.touch_index for touch in filtered_touches}
    filtered_classified = [
        item for item in classified
        if item[0].touch_index in valid_touch_indexes
    ]
    return filtered_touches, filtered_classified


def _touch_confidence(
    touches: list[ZoneTouch],
    classified: list[tuple[ZoneTouch, int, int, float]],
    as_of_index: int,
) -> float:
    """依「單一方向」的觸碰計算 confidence——呼叫端要先依 approach_direction
    篩好 touches/classified 再傳進來，這裡本身不做篩選。"""
    hold_c = sum(1 for _, hold_label, _, _ in classified if hold_label)
    break_c = sum(1 for _, _, break_label, _ in classified if break_label)
    bars_since = (as_of_index - touches[-1].touch_index) if touches else None
    return _confidence(len(touches), bars_since, hold_c, break_c)


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


def _chip_direction(chip_score: Optional[float]) -> str:
    """整檔籌碼原始方向（未依角色翻轉）：bullish/bearish/neutral/none。
    none = 查無籌碼資料（跟 neutral「有資料但中性」不同，前端要分開顯示）。
    角色化的加分/扣分效果由 chip 貢獻分與機率邊際貢獻（bounce/break delta）
    表達，不在這裡翻號。"""
    if chip_score is None:
        return "none"
    if chip_score >= CHIP_SIGNAL_WEAK_THRESHOLD:
        return "bullish"
    if chip_score <= -CHIP_SIGNAL_WEAK_THRESHOLD:
        return "bearish"
    return "neutral"


def _chip_signal(score: Optional[float]) -> Optional[str]:
    if score is None:
        return None
    if score >= CHIP_SIGNAL_STRONG_THRESHOLD:
        return "BULLISH"
    if score >= CHIP_SIGNAL_WEAK_THRESHOLD:
        return "WEAK_BULLISH"
    if score <= -CHIP_SIGNAL_STRONG_THRESHOLD:
        return "BEARISH"
    if score <= -CHIP_SIGNAL_WEAK_THRESHOLD:
        return "WEAK_BEARISH"
    return "NEUTRAL"


def _build_chip_summary(chip_row: Optional[dict]) -> dict[str, Any]:
    """整檔層級籌碼拆解，供前端「共用面板」一次顯示（不逐 zone 重複）。查無
    資料時 missing=True、各分數 None，跟「中性（分數接近 0）」明確區分。分數
    範圍見 internal/chip：total/法人/融資/分點為 -100~100，集中度為 0~100。
    這是分析快照當下對齊的籌碼（見 score_symbol 的 before_date 說明），不是即時
    最新值。"""
    if chip_row is None:
        return {
            "missing": True,
            "score": None,
            "raw_score": None,
            "effective_score": None,
            "coverage": 0.0,
            "confidence": 0.0,
            "confidence_level": "NONE",
            "signal": None,
            "trade_date": None,
            "institutional_score": None,
            "margin_score": None,
            "broker_score": None,
            "concentration_score": None,
        }
    components = {
        key: chip_row.get(key)
        for key in CHIP_COMPONENT_WEIGHTS
    }
    available_weight = sum(
        weight for key, weight in CHIP_COMPONENT_WEIGHTS.items()
        if components.get(key) is not None
    )
    weighted_sum = sum(
        float(components[key]) * weight
        for key, weight in CHIP_COMPONENT_WEIGHTS.items()
        if components.get(key) is not None
    )
    raw_score = weighted_sum / available_weight if available_weight > 0 else None
    coverage = min(max(available_weight, 0.0), 1.0)
    # effective_score 依 coverage 降權，代表缺分量後的實際影響強度（= raw_score * coverage
    # = weighted_sum）。不直接用 DB total_score：total_score 未依覆蓋率降權，低覆蓋率時會回報
    # 未降權的全量分數，誤導成籌碼影響比實際更強（見 sr-zone-scoring.md「Chip missingness」）。
    effective_score = weighted_sum if raw_score is not None else None
    confidence_level = "HIGH" if coverage >= 0.8 else "MEDIUM" if coverage >= 0.5 else "LOW" if coverage > 0 else "NONE"
    signal_score = effective_score if effective_score is not None else raw_score
    return {
        "missing": False,
        "score": raw_score,
        "raw_score": raw_score,
        "effective_score": effective_score,
        "coverage": coverage,
        "confidence": coverage,
        "confidence_level": confidence_level,
        "signal": _chip_signal(signal_score),
        "source_signal": chip_row.get("signal"),
        "trade_date": chip_row.get("trade_date"),
        "institutional_score": float(components["institutional_score"]) if components["institutional_score"] is not None else None,
        "margin_score": float(components["margin_score"]) if components["margin_score"] is not None else None,
        "broker_score": float(components["broker_score"]) if components["broker_score"] is not None else None,
        "concentration_score": float(components["concentration_score"]) if components["concentration_score"] is not None else None,
    }


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
    overlap_group: Optional[int] = None,
    confluence_count: int = 1,
    confluence_family_count: int = 1,
    confluence_families: tuple[str, ...] = (),
    chip_score: Optional[float] = None,
    chip_features: Optional[dict[str, float]] = None,
    feature_set: Optional[ZoneFeatureSet] = None,
) -> ZoneScore:
    if as_of_index is None:
        as_of_index = len(df) - 1

    if feature_set is None:
        features_as_support = compute_zone_features(
            df, zone, as_of_index=as_of_index, approach=ApproachDirection.FROM_ABOVE,
            lookback_bars=lookback_bars, forward_bars=forward_bars, threshold_pct=threshold_pct,
        )
        features_as_resistance = compute_zone_features(
            df, zone, as_of_index=as_of_index, approach=ApproachDirection.FROM_BELOW,
            lookback_bars=lookback_bars, forward_bars=forward_bars, threshold_pct=threshold_pct,
        )
        all_touches = find_touches(df, zone, as_of_index, lookback_bars)
        support_touches = [t for t in all_touches if t.approach_direction == ApproachDirection.FROM_ABOVE]
        resistance_touches = [t for t in all_touches if t.approach_direction == ApproachDirection.FROM_BELOW]
        support_classified = _classify_touches(df, support_touches, forward_bars, threshold_pct, as_of_index)
        resistance_classified = _classify_touches(df, resistance_touches, forward_bars, threshold_pct, as_of_index)
    else:
        features_as_support = feature_set.support.values
        features_as_resistance = feature_set.resistance.values
        all_touches = list(feature_set.all_touches)
        support_touches = list(feature_set.support_touches)
        resistance_touches = list(feature_set.resistance_touches)
        support_classified = list(feature_set.support_labels)
        resistance_classified = list(feature_set.resistance_labels)

    # confidence 依角色方向分開計算（role=SUPPORT 只用 support_touches 的樣本
    # 數/穩定度，不會被 resistance 方向的觸碰稀釋或拉抬），見 types.py::
    # ZoneScore 的可信度說明。support_score/resistance_score 兩者都要算（供
    # net_score 使用），所以兩個方向的 confidence 都要先算出來。
    confidence_as_support = _touch_confidence(support_touches, support_classified, as_of_index)
    confidence_as_resistance = _touch_confidence(resistance_touches, resistance_classified, as_of_index)

    if chip_features is None:
        chip_features = chip_features_from_score_row({
            "total_score": chip_score,
            "institutional_score": 0.0,
            "margin_score": 0.0,
            "broker_score": 0.0,
            "concentration_score": 0.0,
        } if chip_score is not None else None)
    support_hold, support_break = _normalize_probabilities(
        predict_hold_probability(bundle, features_as_support, is_support=True, chip_features=chip_features),
        predict_break_probability(bundle, features_as_support, is_support=True, chip_features=chip_features),
    )
    resistance_hold, resistance_break = _normalize_probabilities(
        predict_hold_probability(bundle, features_as_resistance, is_support=False, chip_features=chip_features),
        predict_break_probability(bundle, features_as_resistance, is_support=False, chip_features=chip_features),
    )

    support_score = _derive_score(support_hold, confidence_as_support)
    resistance_score = _derive_score(resistance_hold, confidence_as_resistance)
    net_score = support_score - resistance_score
    net_score_label = _net_score_label(net_score)

    role = _resolve_role(zone, current_price)

    if role == ZoneType.SUPPORT.value:
        role_touches, role_classified, confidence = support_touches, support_classified, confidence_as_support
    elif role == ZoneType.RESISTANCE.value:
        role_touches, role_classified, confidence = resistance_touches, resistance_classified, confidence_as_resistance
    else:
        # AT_ZONE：方向還沒解析出來，用全部觸碰（兩個方向合計）計算 confidence，
        # 跟 R1 設計前的行為一致。
        role_touches = all_touches
        role_classified = _classify_touches(df, all_touches, forward_bars, threshold_pct, as_of_index)
        confidence = _touch_confidence(all_touches, role_classified, as_of_index)

    confidence_level = _confidence_level(confidence)
    validation_touches, validation_classified = _filter_validation_touches(role_touches, role_classified, zone)
    recent_validation = _recent_validation(validation_touches, validation_classified, as_of_index)
    validation_start_index = max(1, zone.formed_at_index + 1, as_of_index - lookback_bars + 1)
    validation_debug = _validation_debug_metadata(
        df, zone, validation_touches, validation_classified, as_of_index, validation_start_index
    )

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
    chip_bounce_delta: Optional[float] = None
    chip_break_delta: Optional[float] = None

    if role != ZoneType.AT_ZONE.value:
        is_support = role == ZoneType.SUPPORT.value
        role_features = features_as_support if is_support else features_as_resistance
        hold_p, break_p = (support_hold, support_break) if is_support else (resistance_hold, resistance_break)

        bounce_probability = hold_p
        break_probability = break_p

        # 【籌碼機率邊際貢獻】反事實：把實際籌碼換成中性籌碼（neutral_chip_features），
        # 用同一組 zone 特徵重算本角色的 hold/break 機率，差值（百分點）就是「這檔
        # 籌碼相對中性籌碼把反彈/跌破機率推了多少」。查無籌碼資料（chip_missing）時
        # 無從比較，留 None。這是模型路徑的貢獻，跟 trading_score 的 chip 直接加權
        # 分量（15%）是兩條獨立路徑，前端會分開標示。
        if chip_features is not None and not chip_features.get("chip_missing"):
            base_hold, base_break = _normalize_probabilities(
                predict_hold_probability(bundle, role_features, is_support=is_support, chip_features=neutral_chip_features()),
                predict_break_probability(bundle, role_features, is_support=is_support, chip_features=neutral_chip_features()),
            )
            chip_bounce_delta = (hold_p - base_hold) * 100.0
            chip_break_delta = (break_p - base_break) * 100.0
        expected_gain = role_features.average_bounce_return
        expected_loss = role_features.average_break_return
        # 一、修正 EV：不再用單一 average_return，改成 hold機率×平均反彈報酬 +
        # break機率×平均跌破報酬，兩個方向分開加權。
        expected_value = hold_p * expected_gain + break_p * expected_loss

        relative_volume = role_features.relative_volume
        # 跨層改名點：特徵層叫 rejection_count（ZoneFeatures），從這裡起對外一律
        # 叫 reject_count（ZoneScore / API / DB / Go / TS / Svelte）。兩者是同一個量，
        # 只是所處的層不同，沒有第二個獨立數值。命名對照見 docs/sr-zone-scoring.md。
        reject_count = role_features.rejection_count
        break_count_field = role_features.breakout_count

        if expected_loss != 0:
            risk_reward_ratio = abs(expected_gain / expected_loss)
            reward_risk_percentile_value = reward_risk_percentile(bundle, risk_reward_ratio)

        volume_confirmation = _volume_confirmation(relative_volume, recent_validation)

    trading_score_breakdown = _trading_score_breakdown(
        role, confidence, expected_value, risk_reward_ratio, overall_trend, volume_confirmation,
        chip_score=chip_score,
    )
    trading_score_value = _trading_score(trading_score_breakdown)
    trading_recommendation = _trading_recommendation(trading_score_value, role)
    entry_relevance_breakdown = _entry_relevance_breakdown(
        role=role,
        current_price=current_price,
        price_low=zone.price_low,
        price_high=zone.price_high,
        confidence=confidence,
        expected_value=expected_value,
        risk_reward_ratio=risk_reward_ratio,
        recent_validation=recent_validation,
        volume_confirmation=volume_confirmation,
    )
    entry_relevance_value = _entry_relevance_score(entry_relevance_breakdown)
    chip_direction = _chip_direction(chip_score)

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
        support_touch_count=len(support_touches),
        resistance_touch_count=len(resistance_touches),
        reject_count=reject_count,
        break_count=break_count_field,
        zone_momentum=zone_momentum_value,
        zone_direction=zone_direction,
        recent_validation=recent_validation,
        trading_score=trading_score_value,
        trading_score_breakdown=trading_score_breakdown,
        trading_recommendation=trading_recommendation,
        overlap_group=overlap_group,
        confluence_count=confluence_count,
        confluence_family_count=confluence_family_count,
        confluence_families=confluence_families or (_evidence_family(zone.method),),
        chip_direction=chip_direction,
        chip_bounce_delta=chip_bounce_delta,
        chip_break_delta=chip_break_delta,
        zone_quality_score=trading_score_value,
        entry_relevance_score=entry_relevance_value,
        entry_relevance_breakdown=entry_relevance_breakdown,
        validation_debug=validation_debug,
    )


def score_symbol(
    symbol: str,
    timeframe: str = "1d",
    limit: int = DEFAULT_FETCH_LIMIT,
    builders: Optional[list[ZoneBuilder]] = None,
) -> dict[str, Any]:
    """limit 為抓取的歷史K棒根數（不是天數），預設 DEFAULT_FETCH_LIMIT=250；
    呼叫端（FastAPI /sr-zones、Go handler）可覆寫。"""
    from .pipeline import run_pipeline

    return run_pipeline(
        symbol,
        timeframe,
        limit,
        builders or _default_builders(),
        fetch_candles_fn=fetch_candles,
        fetch_chip_fn=fetch_latest_chip_score,
        get_model_fn=get_model,
    )
