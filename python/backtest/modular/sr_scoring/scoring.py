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
    TradingRecommendation,
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

# 十三、Score 必須可拆解：六個分量、明確權重，總和 = 100。
# 【2026-07 籌碼分析整合】新增 chip 分量（權重 15），其餘五個分量依原比例
# 縮小至 85（40/20/15/15/10 → 34/17/12.75/12.75/8.5）。v3 模型已將
# chip_features 納入 hold/break 機率模型，籌碼會透過機率與 expected_value /
# support_score / resistance_score 影響總分；這裡的 chip 權重則是第二條路徑，
# 直接用原始 chip_score 加權。這組數字是初始權重，之後可依實際回測結果調整。
TRADING_SCORE_WEIGHTS = {
    "expected_value": 34.0,
    "risk_reward": 17.0,
    "trend": 12.75,
    "volume": 12.75,
    "confidence": 8.5,
    "chip": 15.0,
}

# 籌碼分數（chip_scores.total_score，-100~100）判定偏多/偏空的門檻，對齊 Go
# internal/chip 的 signalThreshold（±20 才算 BULLISH/BEARISH，見 chip/score.go）。
# 摘要方向敘述（_chip_reason）與結構化方向（_chip_direction）共用同一個門檻。
CHIP_SIGNAL_THRESHOLD = 20.0

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

PERIOD_SUMMARY_CONFIG = [
    ("short", "短期", ZoneTier.TIER_3_SHORT_TERM.value),
    ("mid", "中期", ZoneTier.TIER_2_TRADING_ZONE.value),
    ("long", "長期", ZoneTier.TIER_1_MAIN_STRUCTURE.value),
]


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
    同一層內再依 trading_score 由高到低，不改變這個主要排序規則；
    confluence_count（多方法共振的 zone 數）只當第三順位的 tie-breaker，
    trading_score 幾乎不會真的相等，實務上很少真正影響排序結果。"""
    return sorted(
        zone_scores, key=lambda z: (_TIER_ORDER.get(z.tier, 99), -z.trading_score, -z.confluence_count)
    )


# ── 跨方法重疊分群（confluence）───────────────────────────────

OVERLAP_GROUP_THRESHOLD = 0.6  # overlap 相對於較窄 zone 寬度的比例達此門檻才算同一群組


def _zone_overlap_ratio(a: Zone, b: Zone) -> float:
    overlap = min(a.price_high, b.price_high) - max(a.price_low, b.price_low)
    if overlap <= 0:
        return 0.0
    return overlap / min(a.width, b.width)


def _group_overlapping_zones(zones: list[Zone]) -> tuple[list[Optional[int]], list[int]]:
    """標記「不同方法（ATR / volume_profile）各自建出來、但實際上指向同一
    價位帶」的 zone：只在乎跨方法的重疊，同一種方法建出來的 zone 已經在
    各自的 ZoneBuilder 內做過合併（見 zone_builder.py 的 merge_pct），這裡
    不重複處理。不合併、不刪除任何 zone——兩個 builder 各自的計算基礎不同
    （swing pivot + ATR vs. 成交量分布），合併會丟失「這是兩種方法都認同
    的價位」這個本身就有意義的資訊；只標記 overlap_group/confluence_count
    供前端顯示「多方法共振」、或在排序時當 tie-breaker（見 _sort_zone_scores）。

    用 union-find：兩兩比較找出跨方法且 overlap 達門檻的 pair 先 union，
    非傳遞相連的 zone 不會被誤併（例如 A-B 重疊、B-C 重疊但 A-C 不重疊，
    A/B/C 仍會被視為同一個群組——這是 union-find 的標準行為，跟「這個群組
    整體覆蓋的價位範圍有多寬」是分開的問題，這裡不處理後者）。

    回傳 (overlap_group, confluence_count)，皆與輸入 zones 同順序對應。
    confluence_count 恆 >= 1（自己）；overlap_group 只有 confluence_count > 1
    時才賦值，單獨一個 zone 沒有「群組」可言，回傳 None。"""
    n = len(zones)
    parent = list(range(n))

    def find(i: int) -> int:
        while parent[i] != i:
            parent[i] = parent[parent[i]]
            i = parent[i]
        return i

    def union(i: int, j: int) -> None:
        ri, rj = find(i), find(j)
        if ri != rj:
            parent[rj] = ri

    for i in range(n):
        for j in range(i + 1, n):
            if zones[i].method == zones[j].method:
                continue
            if _zone_overlap_ratio(zones[i], zones[j]) >= OVERLAP_GROUP_THRESHOLD:
                union(i, j)

    members_by_root: dict[int, list[int]] = {}
    for i in range(n):
        members_by_root.setdefault(find(i), []).append(i)

    overlap_group: list[Optional[int]] = [None] * n
    confluence_count: list[int] = [1] * n
    group_id = 0
    for members in members_by_root.values():
        confluence = len(members)
        for i in members:
            confluence_count[i] = confluence
        if confluence > 1:
            for i in members:
                overlap_group[i] = group_id
            group_id += 1

    return overlap_group, confluence_count


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
    chip_score: Optional[float] = None,
) -> dict[str, float]:
    """Score = EV(34%) + RR(17%) + Trend(12.75%) + Volume(12.75%) + Confidence(8.5%)
    + Chip(15%)。每個分量先正規化到 [0,1] 再乘上對應權重，回傳值就是「這個
    分量對總分的實際貢獻」（加總即為 trading_score），不是抽象的 0~1 子分數
    ——這樣使用者可以直接看出「總分裡有幾分來自 EV、幾分來自量能」，不用
    自己再乘一次權重（十四、1. 可解釋、十四、8. 明確定義計算公式）。

    role=AT_ZONE 或角色相關數值缺值（EV/RR/量能確認都要求 role 已解析）時，
    對應分量用中性值 0.5 計算，不直接給 0 分——沒有方向不代表這個 zone
    「不好」，只是還沒有可以評分的方向性資料。Trend/Confidence 不需要角色
    解析，任何情況都能算。

    【2026-07 籌碼分析整合】chip_score 是 chip_scores.total_score（-100~100，
    見 internal/chip 套件），跟 trend 一樣需要依角色翻轉正負號：籌碼偏多
    （chip_score 為正）代表股價有支撐、較不易跌破，對 SUPPORT 角色是加分；
    但對 RESISTANCE 角色代表買盤較強，反而較容易「站上」壓力（壓力較不易
    守住），要用負值計算。查無籌碼資料（chip_score=None，例如尚未同步）時
    用中性值 0.5，不阻塞整體評分。"""
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

    if chip_score is None:
        chip_norm = 0.5
    elif is_resistance:
        chip_norm = _normalize_signed(-chip_score, cap=100.0)
    else:
        chip_norm = _normalize_signed(chip_score, cap=100.0)  # SUPPORT 與 AT_ZONE 用原始值

    w = TRADING_SCORE_WEIGHTS
    return {
        "expected_value": float(ev_norm * w["expected_value"]),
        "risk_reward": float(rr_norm * w["risk_reward"]),
        "trend": float(trend_norm * w["trend"]),
        "volume": float(volume_norm * w["volume"]),
        "confidence": float(confidence * w["confidence"]),
        "chip": float(chip_norm * w["chip"]),
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


def _distance_pct_to_zone_bounds(price_low: float, price_high: float, current_price: float) -> float:
    if price_low <= current_price <= price_high:
        return 0.0
    if current_price < price_low:
        return (price_low - current_price) / current_price
    return (current_price - price_high) / current_price


def _entry_relevance_breakdown(
    *,
    role: str,
    current_price: float,
    price_low: float,
    price_high: float,
    confidence: float,
    expected_value: Optional[float],
    risk_reward_ratio: Optional[float],
    recent_validation: str,
    volume_confirmation: Optional[str],
) -> dict[str, float]:
    distance_pct = _distance_pct_to_zone_bounds(price_low, price_high, current_price)
    distance = max(0.0, 1.0 - min(distance_pct / 0.08, 1.0)) * 30.0
    ev_rr = 0.0
    if expected_value is not None:
        ev_rr += max(0.0, min((expected_value + 0.02) / 0.07, 1.0)) * 15.0
    else:
        ev_rr += 7.5
    if risk_reward_ratio is not None:
        ev_rr += min(risk_reward_ratio / 2.5, 1.0) * 15.0
    else:
        ev_rr += 7.5
    validation_map = {
        RecentValidation.VALIDATED_RECENTLY.value: 20.0,
        RecentValidation.PENDING_VALIDATION.value: 12.0,
        RecentValidation.NOT_TESTED_RECENTLY.value: 10.0,
        RecentValidation.EXPIRED.value: 0.0,
    }
    validation = validation_map.get(recent_validation, 8.0)
    volume_map = {
        VolumeConfirmation.CONFIRMED.value: 10.0,
        VolumeConfirmation.NEUTRAL.value: 6.0,
        VolumeConfirmation.WEAK.value: 3.0,
        VolumeConfirmation.FAILED.value: 0.0,
    }
    volume = volume_map.get(volume_confirmation, 5.0)
    role_readiness = 0.0 if role == ZoneType.AT_ZONE.value else 10.0
    return {
        "distance": float(distance),
        "ev_rr": float(ev_rr),
        "validation": float(validation),
        "volume": float(volume),
        "role_readiness": float(role_readiness),
        "confidence": float(confidence * 10.0),
    }


def _entry_relevance_score(breakdown: dict[str, float]) -> float:
    # clamp 到 [0,100]，讓 entry_relevance_score 是有界的百分制分數；也跟 decision_engine
    # 對外回報的 base entry relevance 同界線，避免同一 zone 兩處數值對不上。
    return float(max(0.0, min(100.0, sum(breakdown.values()))))


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


# ── 短中長期摘要與白話 tips ───────────────────────────────────


def _fmt_price(v: float) -> str:
    return f"{v:.2f}"


def _moving_average_state(current_price: float, ma5: Optional[float]) -> str:
    if ma5 is None:
        return "5日均線資料不足，先以區間本身觀察。"
    diff = (current_price - ma5) / ma5 if ma5 else 0.0
    if diff > 0.01:
        return f"收盤站上5日均線（{_fmt_price(ma5)}），短線動能偏穩。"
    if diff < -0.01:
        return f"收盤跌破5日均線（{_fmt_price(ma5)}），短線動能偏弱。"
    return f"收盤接近5日均線（{_fmt_price(ma5)}），方向仍在整理。"


def _chip_reason(chip_score: Optional[float], side: str) -> str:
    if chip_score is None:
        return "尚無籌碼分數，這一項先以中性看待。"
    if chip_score >= CHIP_SIGNAL_THRESHOLD:
        return "籌碼偏多，對支撐較有利。" if side == "support" else "籌碼偏多，壓力可能較容易被挑戰。"
    if chip_score <= -CHIP_SIGNAL_THRESHOLD:
        return "籌碼偏空，支撐需要更多確認。" if side == "support" else "籌碼偏空，壓力較容易形成壓制。"
    return "籌碼分數接近中性，暫無明顯加分或扣分。"


def _chip_direction(chip_score: Optional[float]) -> str:
    """整檔籌碼原始方向（未依角色翻轉）：bullish/bearish/neutral/none。
    none = 查無籌碼資料（跟 neutral「有資料但中性」不同，前端要分開顯示）。
    角色化的加分/扣分效果由 chip 貢獻分與機率邊際貢獻（bounce/break delta）
    表達，不在這裡翻號。"""
    if chip_score is None:
        return "none"
    if chip_score >= CHIP_SIGNAL_THRESHOLD:
        return "bullish"
    if chip_score <= -CHIP_SIGNAL_THRESHOLD:
        return "bearish"
    return "neutral"


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
            "signal": None,
            "institutional_score": None,
            "margin_score": None,
            "broker_score": None,
            "concentration_score": None,
        }
    return {
        "missing": False,
        "score": float(chip_row["total_score"]),
        "signal": chip_row.get("signal"),
        "institutional_score": float(chip_row.get("institutional_score") or 0.0),
        "margin_score": float(chip_row.get("margin_score") or 0.0),
        "broker_score": float(chip_row.get("broker_score") or 0.0),
        "concentration_score": float(chip_row.get("concentration_score") or 0.0),
    }


def _volume_reason(z: ZoneScore) -> Optional[str]:
    if z.volume_confirmation == VolumeConfirmation.CONFIRMED.value:
        return "量能有確認，這個區間的參考性較高。"
    if z.volume_confirmation == VolumeConfirmation.WEAK.value:
        return "量能不足，先降低這個區間的信心。"
    if z.volume_confirmation == VolumeConfirmation.FAILED.value:
        return "高量但驗證失敗，這個區間可能已被破壞。"
    return None


def _validation_reason(z: ZoneScore) -> str:
    if z.recent_validation == RecentValidation.VALIDATED_RECENTLY.value:
        return "最近一次測試有守住，短線仍有參考價值。"
    if z.recent_validation == RecentValidation.EXPIRED.value:
        return "最近一次測試偏失效，需等待重新站回或跌回確認。"
    if z.recent_validation == RecentValidation.NOT_TESTED_RECENTLY.value:
        return "近期沒有重新測試，參考性會隨時間下降。"
    return "尚待後續K棒驗證，不宜單獨當成進出依據。"


def _zone_summary(z: ZoneScore, side: str, current_price: float, ma5: Optional[float]) -> dict[str, Any]:
    # 籌碼不再擠進 reasons 那句話，改成結構化 chip 欄位（見下），讓前端可以
    # 用數字/徽章呈現而不只是一句文字。reasons 只留均線、驗證、量能、信心、
    # 共振等非籌碼理由。
    reasons = [
        _moving_average_state(current_price, ma5),
        _validation_reason(z),
    ]
    volume = _volume_reason(z)
    if volume:
        reasons.append(volume)
    if z.confidence_level in (ConfidenceLevel.HIGH.value, ConfidenceLevel.VERY_HIGH.value):
        reasons.append("信心分級偏高，可列為主要觀察區。")
    elif z.confidence_level == ConfidenceLevel.LOW.value:
        reasons.append("信心分級偏低，代表樣本少或近期驗證不足。")
    if z.confluence_count > 1:
        reasons.append(f"有{z.confluence_count}種方法指向相近區間，屬於多方法共振。")

    return {
        "price_low": z.price_low,
        "price_high": z.price_high,
        "label": f"{_fmt_price(z.price_low)} ~ {_fmt_price(z.price_high)}",
        "role": z.role,
        "side": side,
        "tier": z.tier,
        "tier_label": z.tier_label,
        "confidence": z.confidence,
        "confidence_level": z.confidence_level,
        "trading_score": z.trading_score,
        "recent_validation": z.recent_validation,
        "volume_confirmation": z.volume_confirmation,
        "confluence_count": z.confluence_count,
        # 結構化籌碼（角色化）：direction 是整檔原始方向（偏多/偏空/中性/無資料）；
        # contribution 是這個角色下籌碼對 trading_score 的直接加權貢獻（0~15，已依
        # 支撐/壓力翻號，見 _trading_score_breakdown）；bounce/break delta 是籌碼相對
        # 中性籌碼對本 zone 反彈/跌破機率的邊際貢獻（百分點，模型路徑，見 score_zone）。
        # 兩個數字分屬「直接權重」與「模型」兩條路徑，不是重複計分（見 todo T-014）。
        "chip": {
            "direction": z.chip_direction,
            "contribution": z.trading_score_breakdown.get("chip"),
            "bounce_delta_pp": z.chip_bounce_delta,
            "break_delta_pp": z.chip_break_delta,
        },
        "reasons": reasons[:5],
    }


def _pick_period_pair(zones: list[ZoneScore], current_price: float) -> tuple[Optional[ZoneScore], Optional[ZoneScore]]:
    supports = [z for z in zones if z.role == ZoneType.SUPPORT.value and z.price_high < current_price]
    resistances = [z for z in zones if z.role == ZoneType.RESISTANCE.value and z.price_low > current_price]
    supports.sort(key=lambda z: (z.trading_score, z.confidence, z.price_high), reverse=True)
    resistances.sort(key=lambda z: (z.trading_score, z.confidence, -z.price_low), reverse=True)

    for support in supports or [None]:
        for resistance in resistances or [None]:
            if support is not None and resistance is not None and support.price_high >= resistance.price_low:
                continue
            return support, resistance
    return None, None


def _build_period_summaries(
    zone_scores: list[ZoneScore], current_price: float, ma5: Optional[float]
) -> list[dict[str, Any]]:
    summaries = []
    for key, label, tier in PERIOD_SUMMARY_CONFIG:
        tier_zones = [z for z in zone_scores if z.tier == tier]
        support, resistance = _pick_period_pair(tier_zones, current_price)
        summary = {
            "key": key,
            "label": label,
            "tier": tier,
            "support": _zone_summary(support, "support", current_price, ma5) if support else None,
            "resistance": _zone_summary(resistance, "resistance", current_price, ma5) if resistance else None,
        }
        if support is None:
            summary["support_note"] = "暫無明確支撐"
        if resistance is None:
            summary["resistance_note"] = "暫無明確壓力"
        summaries.append(summary)
    return summaries


def _build_analysis_tips(
    period_summaries: list[dict[str, Any]], current_price: float, ma5: Optional[float], chip_score: Optional[float]
) -> list[str]:
    tips = [
        "預設只列短中長期各一個支撐與壓力；完整 zone 可在明細展開。",
        "若某期間沒有明確支撐或壓力，代表模型找不到符合現價位置的合理區間，不會硬湊數字。",
        "支撐應在現價下方、壓力應在現價上方；若區間不符合這個關係會被摘要排除。",
        _moving_average_state(current_price, ma5),
        _chip_reason(chip_score, "support"),
    ]
    for s in period_summaries:
        if s.get("support") and s.get("resistance"):
            tips.append(f"{s['label']}區間已同時找到支撐與壓力，可優先觀察價格接近哪一側。")
        elif s.get("support"):
            tips.append(f"{s['label']}目前只有支撐較明確，上方壓力仍需等待新結構形成。")
        elif s.get("resistance"):
            tips.append(f"{s['label']}目前只有壓力較明確，下方支撐仍需等待新結構形成。")
    return tips[:8]


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
        # 分量（15%）是兩條獨立路徑，前端會分開標示（見 todo T-014）。
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
        chip_direction=chip_direction,
        chip_bounce_delta=chip_bounce_delta,
        chip_break_delta=chip_break_delta,
        zone_quality_score=trading_score_value,
        entry_relevance_score=entry_relevance_value,
        entry_relevance_breakdown=entry_relevance_breakdown,
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
        "support_touch_count": z.support_touch_count,
        "resistance_touch_count": z.resistance_touch_count,
        "reject_count": z.reject_count,
        "break_count": z.break_count,
        "zone_momentum": z.zone_momentum,
        "zone_direction": z.zone_direction,
        "recent_validation": z.recent_validation,
        "trading_score": z.trading_score,
        "trading_score_breakdown": z.trading_score_breakdown,
        "trading_recommendation": z.trading_recommendation,
        "zone_quality_score": z.zone_quality_score if z.zone_quality_score is not None else z.trading_score,
        "entry_relevance_score": z.entry_relevance_score,
        "entry_relevance_breakdown": z.entry_relevance_breakdown,
        "overlap_group": z.overlap_group,
        "confluence_count": z.confluence_count,
    }
