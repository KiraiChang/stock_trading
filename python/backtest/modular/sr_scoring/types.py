"""共用資料型別：Zone / ZoneFeatures / ZoneTouch / ZoneLabel / ZoneScore。

刻意不重用 backtest/modular/types.py 的 Level/SRLevels：那是「單一價位 +
強度」語意，這裡的 Zone 是「價格區間 [price_low, price_high]」，語意不同；
獨立定義可避免耦合既有 /analyze pipeline（Go 已依賴其輸出格式）。
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


class ApproachDirection(str, Enum):
    FROM_ABOVE = "FROM_ABOVE"  # 價格由上往下觸碰 → 候選支撐
    FROM_BELOW = "FROM_BELOW"  # 價格由下往上觸碰 → 候選壓力


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
    rejection_count: int
    breakout_count: int
    avg_return_after_touch: float  # 正值 = 有利於 zone 成立的方向
    relative_volume: float
    volatility: float  # ATR / close，正規化後跨股可比
    trend_strength: float  # MA slope，正規化為 slope * lookback / price


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


@dataclass(frozen=True)
class ZoneScore:
    """對外回傳的單一 zone 評分結果。

    support_score/resistance_score 由 confidence 收縮過的機率推導而來
    （見 scoring.py 開頭說明），不是獨立於 bounce/break_probability 之外
    的規則式分數，兩者不會互相矛盾。

    confidence：只由觸碰次數決定的貝式收縮係數（0~1），觸碰次數越少越低，
    用來避免「尚未驗證」的 zone 被判成高分或給出誇大的期望值。

    expected_value/risk_reward_ratio：只有 role 為 SUPPORT/RESISTANCE 時
    才有值（AT_ZONE 沒有明確方向可以算）；expected_value 是「觸碰後平均報酬
    × hold機率 - zone寬度風險 × break機率」再經 confidence 收縮，
    risk_reward_ratio 是純粹的報酬/風險幅度比，不受 confidence 影響。
    """

    price_low: float
    price_high: float
    method: str
    role: str
    support_score: float
    resistance_score: float
    confidence: float
    bounce_probability: Optional[float]
    break_probability: Optional[float]
    expected_value: Optional[float]
    risk_reward_ratio: Optional[float]
    features_as_support: Optional[ZoneFeatures]
    features_as_resistance: Optional[ZoneFeatures]
