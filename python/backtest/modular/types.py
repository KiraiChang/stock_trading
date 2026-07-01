"""共用資料型別：Level / EntrySignal / Position / Trade / BacktestReport。"""
from __future__ import annotations

from dataclasses import dataclass, field
from enum import Enum


class Direction(str, Enum):
    LONG = "LONG"
    SHORT = "SHORT"


class LevelType(str, Enum):
    SUPPORT = "SUPPORT"
    RESISTANCE = "RESISTANCE"


@dataclass(frozen=True)
class Level:
    """一個支撐或壓力價位。"""
    price: float
    type: LevelType
    strength: float  # 0.0 ~ 1.0，觸碰次數/量能占比等正規化後的強度
    method: str  # 產生此 level 的方法名稱，例如 "swing" / "atr_channel" / "volume_profile"


@dataclass(frozen=True)
class SRLevels:
    supports: list[Level] = field(default_factory=list)
    resistances: list[Level] = field(default_factory=list)

    def nearest_support_below(self, price: float) -> Level | None:
        below = [lv for lv in self.supports if lv.price < price]
        return max(below, key=lambda lv: lv.price) if below else None

    def nearest_resistance_above(self, price: float) -> Level | None:
        above = [lv for lv in self.resistances if lv.price > price]
        return min(above, key=lambda lv: lv.price) if above else None


@dataclass(frozen=True)
class EntrySignal:
    direction: Direction
    reference_level: float  # 觸發用的支撐/壓力價位
    reason: str


@dataclass
class Position:
    direction: Direction
    entry_index: int
    entry_time: object  # pandas.Timestamp
    entry_price: float
    stop_price: float


class ExitReason(str, Enum):
    STOP_LOSS = "STOP_LOSS"
    EOD_FORCE_CLOSE = "EOD_FORCE_CLOSE"  # 資料結束時強制平倉（回測慣例）


@dataclass(frozen=True)
class Trade:
    symbol: str
    direction: Direction
    entry_time: object
    exit_time: object
    entry_price: float
    exit_price: float
    size: float
    pnl: float
    pnl_pct: float
    commission: float
    exit_reason: ExitReason


@dataclass(frozen=True)
class BacktestReport:
    strategy: str
    total_return: float
    annual_return: float
    win_rate: float
    max_drawdown: float
    sharpe_ratio: float
    total_trades: int
    win_trades: int
    loss_trades: int
    avg_pnl: float
    trades: list[Trade]
