"""
TradingStrategy：Strategy Pattern 的組合根。

把 SupportResistanceStrategy / EntryStrategy / StopLossStrategy 三個獨立元件
組合成一個完整策略。三者互不相依、可任意搭配替換——這就是需求裡「每個指標
要可替換」的具體實作方式：不是用 if/else 切換演算法，而是在建構時注入不同
的物件。

STRATEGY_PRESETS 提供幾組預先搭配好、可直接用字串名稱指定的組合，
供 service.py／既有的 backtest job（strategy 欄位）使用；也可以完全不用
預設組合，直接在程式裡 `TradingStrategy(name=..., sr_strategy=..., ...)`
自由搭配。
"""
from __future__ import annotations

from dataclasses import dataclass
from typing import Callable

from .entries import BreakoutEntry, EntryStrategy, PullbackSupportEntry
from .exits import ATRStopLoss, CompositeStopLoss, StopLossStrategy, StructuralStopLoss
from .support_resistance import ATRChannelSR, SupportResistanceStrategy, SwingHighLowSR, VolumeProfileSR


@dataclass
class TradingStrategy:
    name: str
    sr_strategy: SupportResistanceStrategy
    entry_strategy: EntryStrategy
    stop_loss_strategy: StopLossStrategy

    @property
    def min_bars(self) -> int:
        return max(self.sr_strategy.min_bars, self.entry_strategy.min_bars)


def _breakout_swing_atr() -> TradingStrategy:
    return TradingStrategy(
        name="breakout_swing_atr_v1",
        sr_strategy=SwingHighLowSR(),
        entry_strategy=BreakoutEntry(),
        stop_loss_strategy=ATRStopLoss(),
    )


def _breakout_volprofile_composite() -> TradingStrategy:
    return TradingStrategy(
        name="breakout_volprofile_composite_v1",
        sr_strategy=VolumeProfileSR(),
        entry_strategy=BreakoutEntry(),
        stop_loss_strategy=CompositeStopLoss(),
    )


def _pullback_atrchannel_structural() -> TradingStrategy:
    return TradingStrategy(
        name="pullback_atrchannel_structural_v1",
        sr_strategy=ATRChannelSR(),
        entry_strategy=PullbackSupportEntry(),
        stop_loss_strategy=StructuralStopLoss(),
    )


def _pullback_swing_composite() -> TradingStrategy:
    return TradingStrategy(
        name="pullback_swing_composite_v1",
        sr_strategy=SwingHighLowSR(),
        entry_strategy=PullbackSupportEntry(),
        stop_loss_strategy=CompositeStopLoss(),
    )


# 字串名稱 → 建構函式。用函式而非現成物件是因為 StopLossStrategy/EntryStrategy
# 部分實作帶有可變狀態（例如未來若擴充快取），每次回測都應該拿到全新實例。
STRATEGY_PRESETS: dict[str, Callable[[], TradingStrategy]] = {
    "breakout_swing_atr_v1": _breakout_swing_atr,
    "breakout_volprofile_composite_v1": _breakout_volprofile_composite,
    "pullback_atrchannel_structural_v1": _pullback_atrchannel_structural,
    "pullback_swing_composite_v1": _pullback_swing_composite,
}


def build_strategy(name: str) -> TradingStrategy:
    factory = STRATEGY_PRESETS.get(name)
    if factory is None:
        raise ValueError(f"Unknown modular strategy: {name}. Available: {list(STRATEGY_PRESETS)}")
    return factory()
