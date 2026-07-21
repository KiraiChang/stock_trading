"""Typed contracts between the five SR Zone analysis stages."""
from __future__ import annotations

from dataclasses import dataclass
from typing import Any

import pandas as pd

from .model import ModelBundle
from .types import Zone, ZoneFeatures, ZoneScore, ZoneTouch


PIPELINE_VERSION = "v2"


@dataclass(frozen=True)
class AnalysisData:
    symbol: str
    timeframe: str
    frame: pd.DataFrame
    analyzed_at: pd.Timestamp
    current_price: float
    zones: tuple[Zone, ...]
    model: ModelBundle
    chip_row: dict[str, Any] | None
    chip_features: dict[str, float]


@dataclass(frozen=True)
class DirectionFeatures:
    role: str
    values: ZoneFeatures
    model_vector: pd.DataFrame


@dataclass(frozen=True)
class ZoneFeatureSet:
    zone: Zone
    support: DirectionFeatures
    resistance: DirectionFeatures
    all_touches: tuple[ZoneTouch, ...]
    support_touches: tuple[ZoneTouch, ...]
    resistance_touches: tuple[ZoneTouch, ...]
    support_labels: tuple[tuple[ZoneTouch, int, int, float], ...]
    resistance_labels: tuple[tuple[ZoneTouch, int, int, float], ...]


@dataclass(frozen=True)
class AnalysisFeatures:
    data: AnalysisData
    global_trend: float
    global_volatility: float
    ma5: float | None
    zones: tuple[ZoneFeatureSet, ...]


@dataclass(frozen=True)
class AnalysisScores:
    features: AnalysisFeatures
    zones: tuple[ZoneScore, ...]
    global_metrics: dict[str, float | None]
    chip_summary: dict[str, Any]


@dataclass(frozen=True)
class AnalysisEvidence:
    scores: AnalysisScores
    global_evidence: dict[str, Any]
    zone_evidence: tuple[dict[str, Any], ...]


@dataclass(frozen=True)
class AnalysisDecision:
    evidence: AnalysisEvidence
    summary: dict[str, Any]
