"""共用資料型別：Zone / ZoneFeatures / ZoneTouch / ZoneLabel / ZoneScore。

刻意不重用 backtest/modular/types.py 的 Level/SRLevels：那是「單一價位 +
強度」語意，這裡的 Zone 是「價格區間 [price_low, price_high]」，語意不同；
獨立定義可避免耦合既有 /analyze pipeline（Go 已依賴其輸出格式）。

2026-07 機構級重新設計（見 scoring.py 開頭的完整說明）：
  - avg_return_after_touch 拆成 average_bounce_return / average_break_return，
    不再混合「反彈」與「跌破」兩種方向完全不同的結果。
  - volatility / trend_strength 維持在 ZoneFeatures 內（模型訓練/預測仍要用
    這兩個特徵），但不再是 API 對外輸出的「每個 zone 都重複顯示」欄位——
    這兩個本質上是股票層級的量，一次算好放在 score_symbol() 回傳值的
    overall_trend/overall_volatility，避免每個 zone 重複同一個數字。
"""
from __future__ import annotations

from dataclasses import dataclass
from enum import Enum
from typing import Optional


class ZoneType(str, Enum):
    SUPPORT = "SUPPORT"
    RESISTANCE = "RESISTANCE"
    AT_ZONE = "AT_ZONE"  # 現價目前落在區間內，尚未明確扮演支撐或壓力角色


class ZoneMethod(str, Enum):
    ATR = "atr"
    VOLUME_PROFILE = "volume_profile"
    RECENT_PIVOT = "recent_pivot"
    BREAKDOWN_RECLAIM = "breakdown_reclaim"
    VWAP_RECLAIM = "vwap_reclaim"


class EvidenceFamily(str, Enum):
    STRUCTURAL_ATR = "STRUCTURAL_ATR"
    VOLUME_PROFILE = "VOLUME_PROFILE"
    RECENT_MICROSTRUCTURE = "RECENT_MICROSTRUCTURE"
    VWAP_OR_AVERAGE_RECLAIM = "VWAP_OR_AVERAGE_RECLAIM"


class ApproachDirection(str, Enum):
    FROM_ABOVE = "FROM_ABOVE"  # 價格由上往下觸碰 → 候選支撐
    FROM_BELOW = "FROM_BELOW"  # 價格由下往上觸碰 → 候選壓力


class NetScoreLabel(str, Enum):
    """net_score = support_score - resistance_score 的分類，避免使用者只看
    單一 support_score 就下判斷（兩個分數只差 4 分時，光看 support_score 不
    容易看出這其實是勢均力敵）。"""

    STRONG_SUPPORT = "STRONG_SUPPORT"
    NEUTRAL = "NEUTRAL"
    STRONG_RESISTANCE = "STRONG_RESISTANCE"


class ConfidenceLevel(str, Enum):
    LOW = "LOW"  # 0~30
    MEDIUM = "MEDIUM"  # 30~60
    HIGH = "HIGH"  # 60~80
    VERY_HIGH = "VERY_HIGH"  # 80~100


class RecentValidation(str, Enum):
    """這個 zone 最近一次被「驗證」（觸碰後有足夠未來資料判斷結果）的狀態。"""

    VALIDATED_RECENTLY = "VALIDATED_RECENTLY"  # 最近一次觸碰守住，且發生在 recent_window 根之內
    PENDING_VALIDATION = "PENDING_VALIDATION"  # 從未被觸碰過，或最近一次觸碰還沒有足夠未來資料判斷結果
    NOT_TESTED_RECENTLY = "NOT_TESTED_RECENTLY"  # 過去守住過，但已經一段時間沒有新的觸碰
    EXPIRED = "EXPIRED"  # 最近一次觸碰的結果是跌破/突破，這個 zone 可能已經失效


class VolumeConfirmation(str, Enum):
    CONFIRMED = "CONFIRMED"  # 高量且守住
    WEAK = "WEAK"  # 量能不足，訊號不可靠
    NEUTRAL = "NEUTRAL"  # 量能普通
    FAILED = "FAILED"  # 高量但跌破/突破，量能確認了失敗


class ZoneDirection(str, Enum):
    UP = "UP"
    DOWN = "DOWN"
    FLAT = "FLAT"


class TradingRecommendation(str, Enum):
    STRONG_BUY = "STRONG_BUY"
    BUY = "BUY"
    WATCH = "WATCH"
    NEUTRAL = "NEUTRAL"
    AVOID = "AVOID"
    STRONG_SELL = "STRONG_SELL"


class ZoneTier(str, Enum):
    """zone 依寬度（price_high - price_low）在同一次分析裡的相對排名分三層，
    讓 zone 清單「可排序」成有意義的階層，而不是一堆平行、看不出主次的
    價格區間：Tier 1 最寬，代表宏觀主結構；Tier 3 最窄，代表短期戰術支撐/
    壓力。見 scoring.py::_assign_tiers。"""

    TIER_1_MAIN_STRUCTURE = "TIER_1_MAIN_STRUCTURE"  # 主結構
    TIER_2_TRADING_ZONE = "TIER_2_TRADING_ZONE"  # 交易區
    TIER_3_SHORT_TERM = "TIER_3_SHORT_TERM"  # 短期支撐/壓力


@dataclass(frozen=True)
class Zone:
    price_low: float
    price_high: float
    method: ZoneMethod
    center_price: float
    formed_at_index: int  # zone 建立當下所依據的最後一根K棒 index（lookahead-safety bookkeeping）

    @property
    def width(self) -> float:
        return self.price_high - self.price_low

    def contains(self, price: float) -> bool:
        return self.price_low <= price <= self.price_high

    def intersects(self, low: float, high: float) -> bool:
        return low <= self.price_high and high >= self.price_low


@dataclass(frozen=True)
class ZoneFeatures:
    touch_count: int
    # rejection_count：特徵層欄位。對外輸出時在 scoring.py 被複製成 ZoneScore.reject_count
    # （同一個量，跨層改名，見 scoring.py 的「跨層改名點」註解與 docs/sr-zone-scoring.md）。
    rejection_count: int
    breakout_count: int
    # average_bounce_return：觸碰後被分類為「守住/反彈」（hold_label=1）的
    # 那些歷史觸碰，其 forward_return 的平均值（恆為正或 0，代表有利方向的
    # 報酬幅度）。average_break_return：被分類為「跌破/突破」（break_label=1）
    # 的那些歷史觸碰，其 forward_return 平均值（恆為負或 0）。兩者分開統計，
    # 不像舊版 avg_return_after_touch 混合所有結果、掩蓋掉「反彈時漲多少」
    # 跟「跌破時虧多少」是完全不同分佈這件事。
    average_bounce_return: float
    average_break_return: float
    relative_volume: float
    volatility: float  # ATR / close；股票層級量，見本檔開頭說明
    trend_strength: float  # MA slope；股票層級量，見本檔開頭說明


@dataclass(frozen=True)
class ZoneTouch:
    zone: Zone
    touch_index: int
    touch_time: object  # pandas.Timestamp
    touch_price: float
    approach_direction: ApproachDirection
    role: ZoneType  # SUPPORT（FROM_ABOVE）或 RESISTANCE（FROM_BELOW）


@dataclass(frozen=True)
class ZoneLabel:
    """一筆訓練資料列：某次觸碰事件的「觸碰當下特徵」+「未來結果」。"""

    symbol: str
    timeframe: str
    touch_time: object
    zone_price_low: float
    zone_price_high: float
    method: ZoneMethod
    role: ZoneType
    features: ZoneFeatures
    forward_bars: int
    threshold_pct: float
    hold_label: int  # 0/1：支撐反彈 / 壓力有效
    break_label: int  # 0/1：支撐跌破 / 壓力突破（鏡射事件）
    forward_return: float  # 實際發生的方向性報酬，僅供診斷用
    chip_features: dict  # touch_time 當下以前最新 chip_scores 數值特徵；缺資料時 chip_missing=1


@dataclass(frozen=True)
class ZoneScore:
    """對外回傳的單一 zone 評分結果（機構級重新設計，見 scoring.py 開頭）。

    支撐/壓力強度：
      support_score/resistance_score 由 confidence 收縮過的 hold 機率推導
      （不是獨立規則式公式，不會跟機率互相矛盾）；net_score = support_score
      - resistance_score，net_score_label 是分類結果，避免只看單一分數。

    可信度：
      confidence 綜合樣本數、時間衰減（含「最近驗證」）、歷史結果穩定度
      三個因子（見 scoring.py::_confidence），confidence_level 是分級結果。
      role=SUPPORT/RESISTANCE 時，confidence 只用該角色方向的觸碰
      （support_touch_count/resistance_touch_count 其中之一）計算，不會被
      另一個方向的樣本數/穩定度稀釋或拉抬；role=AT_ZONE 時用全部觸碰
      （touch_count）計算，因為方向還沒解析出來。touch_count 恆為兩個方向
      加總（zone 整體活躍度），support_touch_count/resistance_touch_count
      分開統計，讓兩種角色各自的歷史樣本數可以被診斷。

    交易數字（只有 role 為 SUPPORT/RESISTANCE 時才有值，AT_ZONE 沒有明確
    方向可以算）：
      bounce_probability/break_probability：這個 zone 現在角色下的機率
      （已正規化，見 _normalize_probabilities）。
      expected_gain/expected_loss：分別對應 average_bounce_return/
      average_break_return（依角色解析後的方向）。
      expected_value = bounce_probability × expected_gain +
                        break_probability × expected_loss（見一、修正說明），
      不再直接用單一 average_return。
      risk_reward_ratio = |expected_gain / expected_loss|。
      reward_risk_percentile：這個 risk_reward_ratio 在訓練資料集歷史分佈中
      的百分位（見 model.py::ModelBundle.rr_reference）。

    量能：
      relative_volume 為角色解析後的值；volume_confirmation 是量能是否
      「確認」這個 zone 走勢的分類。

    區間自身動能（不是股票層級趨勢，見本檔開頭 ZoneFeatures 說明）：
      zone_momentum/zone_direction：這個 zone 過去每次被觸碰前的平均價格
      動能，同一檔股票的不同 zone 會有不同值。

    交易決策：
      recent_validation：最近一次測試的驗證狀態。
      trading_score = trading_score_breakdown 五個分量加總（EV 40% + RR 20%
      + Trend 15% + Volume 15% + Confidence 10%，見 scoring.py::_trading_score_breakdown），
      每個分量都拆開存在 trading_score_breakdown，讓分數「可拆解」，不是
      只有一個黑盒數字。trading_recommendation 是依 trading_score 映射的
      交易建議。

    層級（可排序）：
      tier/tier_label：依 zone 寬度在同一次分析裡的相對排名分三層（見
      ZoneTier），zones 清單依 tier 由粗到細、同層內依 trading_score 排序。
    """

    price_low: float
    price_high: float
    method: str
    role: str

    tier: str
    tier_label: str

    support_score: float
    resistance_score: float
    net_score: float
    net_score_label: str

    confidence: float
    confidence_level: str

    bounce_probability: Optional[float]
    break_probability: Optional[float]
    expected_gain: Optional[float]
    expected_loss: Optional[float]
    expected_value: Optional[float]
    risk_reward_ratio: Optional[float]
    reward_risk_percentile: Optional[float]

    relative_volume: Optional[float]
    volume_confirmation: Optional[str]

    touch_count: int
    support_touch_count: int
    resistance_touch_count: int
    # reject_count：API 輸出欄位，值直接複製自特徵層 ZoneFeatures.rejection_count
    # （同一個量，命名前綴不同，見 scoring.py 的「跨層改名點」與 docs/sr-zone-scoring.md）。
    reject_count: Optional[int]
    break_count: Optional[int]

    zone_momentum: float
    zone_direction: str

    recent_validation: str

    trading_score: float
    trading_score_breakdown: dict
    trading_recommendation: str

    # 跨方法重疊分群（見 scoring.py::_group_overlapping_zones）：不同方法
    # （ATR/volume_profile）各自建出來、但實際上指向同一價位帶的 zone 會
    # 有相同的 overlap_group id，confluence_count 是這個群組裡的 zone 數。
    # 不合併/刪除任何 zone，只標記供前端顯示「多方法共振」或當排序 tie-
    # breaker。confluence_count 恆 >= 1（自己）；overlap_group 只有
    # confluence_count > 1 時才有值，單獨一個 zone 沒有「群組」可言。
    overlap_group: Optional[int]
    confluence_count: int
    confluence_family_count: int = 1
    confluence_families: tuple[str, ...] = ()

    # 籌碼（角色化）——只在記憶體中傳遞給 period summary 的結構化 chip 欄位，
    # 不進 _zone_score_to_dict / zones 表（整檔籌碼另在 score_symbol 的 chip_summary
    # 一次輸出）。chip_direction：整檔原始方向 bullish/bearish/neutral/none。
    # chip_bounce_delta/chip_break_delta：籌碼相對中性籌碼對本 zone 反彈/跌破機率的
    # 邊際貢獻（百分點，模型路徑，見 scoring.py::score_zone）；查無籌碼資料時為 None。
    chip_direction: str = "none"
    chip_bounce_delta: Optional[float] = None
    chip_break_delta: Optional[float] = None
    zone_quality_score: Optional[float] = None
    entry_relevance_score: Optional[float] = None
    entry_relevance_breakdown: Optional[dict] = None
