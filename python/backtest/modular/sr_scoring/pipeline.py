"""Orchestration for Data -> Features -> Score -> Evidence -> Decision."""
from __future__ import annotations

from typing import Any, Optional

from db import fetch_candles, fetch_latest_chip_score

from .decision_engine import build_decision_from_evidence
from .evidence import build_evidence
from .features import compute_zone_features, find_touches, trend_slope, zone_volatility
from .model import chip_features_from_score_row, feature_vector, get_model
from .pipeline_types import (
    PIPELINE_VERSION,
    AnalysisData,
    AnalysisDecision,
    AnalysisFeatures,
    AnalysisScores,
    DirectionFeatures,
    ZoneFeatureSet,
)
from .types import ApproachDirection, ZoneType
from .zone_builder import ZoneBuilder


def load_data(
    symbol: str,
    timeframe: str,
    limit: int,
    builders: list[ZoneBuilder],
    fetch_candles_fn=fetch_candles,
    fetch_chip_fn=fetch_latest_chip_score,
    get_model_fn=get_model,
) -> AnalysisData:
    # Imports are local to keep the legacy public scoring helpers importable.
    from .scoring import _to_dataframe

    rows = fetch_candles_fn(symbol, timeframe, limit=limit)
    if not rows:
        raise ValueError(f"no candles found for symbol={symbol} timeframe={timeframe}")
    frame = _to_dataframe(rows)
    min_bars = max(builder.min_bars for builder in builders)
    if len(frame) < min_bars:
        raise ValueError(f"not enough candles for sr_scoring: symbol={symbol} got={len(frame)}, need>={min_bars}")
    zones = tuple(zone for builder in builders for zone in builder.build(frame))
    analyzed_at = frame.index[-1]
    before_date = analyzed_at.tz_convert("Asia/Taipei").strftime("%Y-%m-%d")
    chip_row = fetch_chip_fn(symbol, before_date=before_date)
    return AnalysisData(
        symbol=symbol,
        timeframe=timeframe,
        frame=frame,
        analyzed_at=analyzed_at,
        current_price=float(frame["close"].iloc[-1]),
        zones=zones,
        model=get_model_fn(),
        chip_row=chip_row,
        chip_features=chip_features_from_score_row(chip_row),
    )


def extract_features(data: AnalysisData) -> AnalysisFeatures:
    from .scoring import (
        DEFAULT_FORWARD_BARS,
        DEFAULT_THRESHOLD_PCT,
        DEFAULT_ZONE_LOOKBACK_BARS,
        _classify_touches,
    )

    frame = data.frame
    as_of = len(frame) - 1
    zone_sets = []
    for zone in data.zones:
        support = compute_zone_features(
            frame, zone, as_of, ApproachDirection.FROM_ABOVE,
            lookback_bars=DEFAULT_ZONE_LOOKBACK_BARS,
            forward_bars=DEFAULT_FORWARD_BARS,
            threshold_pct=DEFAULT_THRESHOLD_PCT,
        )
        resistance = compute_zone_features(
            frame, zone, as_of, ApproachDirection.FROM_BELOW,
            lookback_bars=DEFAULT_ZONE_LOOKBACK_BARS,
            forward_bars=DEFAULT_FORWARD_BARS,
            threshold_pct=DEFAULT_THRESHOLD_PCT,
        )
        all_touches = tuple(find_touches(frame, zone, as_of, DEFAULT_ZONE_LOOKBACK_BARS))
        support_touches = tuple(t for t in all_touches if t.approach_direction == ApproachDirection.FROM_ABOVE)
        resistance_touches = tuple(t for t in all_touches if t.approach_direction == ApproachDirection.FROM_BELOW)
        zone_sets.append(ZoneFeatureSet(
            zone=zone,
            support=DirectionFeatures(
                ZoneType.SUPPORT.value, support,
                feature_vector(support, True, data.chip_features),
            ),
            resistance=DirectionFeatures(
                ZoneType.RESISTANCE.value, resistance,
                feature_vector(resistance, False, data.chip_features),
            ),
            all_touches=all_touches,
            support_touches=support_touches,
            resistance_touches=resistance_touches,
            support_labels=tuple(_classify_touches(frame, list(support_touches), DEFAULT_FORWARD_BARS, DEFAULT_THRESHOLD_PCT, as_of)),
            resistance_labels=tuple(_classify_touches(frame, list(resistance_touches), DEFAULT_FORWARD_BARS, DEFAULT_THRESHOLD_PCT, as_of)),
        ))
    return AnalysisFeatures(
        data=data,
        global_trend=trend_slope(frame, as_of),
        global_volatility=zone_volatility(frame, as_of),
        ma5=float(frame["close"].tail(5).mean()) if len(frame) >= 5 else None,
        zones=tuple(zone_sets),
    )


def calculate_scores(features: AnalysisFeatures) -> AnalysisScores:
    from .scoring import (
        _assign_tiers,
        _build_chip_summary,
        _compute_global_metrics,
        _group_overlapping_zones,
        _sort_zone_scores,
        score_zone,
    )

    data = features.data
    tiers = _assign_tiers([zone.width for zone in data.zones])
    groups, counts = _group_overlapping_zones(list(data.zones))
    chip_score = float(data.chip_row["total_score"]) if data.chip_row is not None else None
    scores = [
        score_zone(
            data.frame, item.zone, data.current_price, data.model, features.global_trend,
            tier=tier, as_of_index=len(data.frame) - 1, overlap_group=group,
            confluence_count=count, chip_score=chip_score, chip_features=data.chip_features,
            feature_set=item,
        )
        for item, tier, group, count in zip(features.zones, tiers, groups, counts)
    ]
    # Keep feature/evidence alignment after score sorting.
    indexed = list(zip(features.zones, scores))
    sorted_scores = _sort_zone_scores(scores)
    lookup = {id(score): item for item, score in indexed}
    sorted_features = tuple(lookup[id(score)] for score in sorted_scores)
    aligned = AnalysisFeatures(
        data=data,
        global_trend=features.global_trend,
        global_volatility=features.global_volatility,
        ma5=features.ma5,
        zones=sorted_features,
    )
    return AnalysisScores(
        features=aligned,
        zones=tuple(sorted_scores),
        global_metrics=_compute_global_metrics(sorted_scores),
        chip_summary=_build_chip_summary(data.chip_row),
    )


def decide(evidence) -> AnalysisDecision:
    return AnalysisDecision(evidence=evidence, summary=build_decision_from_evidence(evidence))


def run_pipeline(
    symbol: str,
    timeframe: str,
    limit: int,
    builders: list[ZoneBuilder],
    fetch_candles_fn=fetch_candles,
    fetch_chip_fn=fetch_latest_chip_score,
    get_model_fn=get_model,
) -> dict[str, Any]:
    from .scoring import (
        _build_analysis_tips,
        _build_period_summaries,
        _zone_score_to_dict,
    )

    data = load_data(
        symbol, timeframe, limit, builders,
        fetch_candles_fn=fetch_candles_fn,
        fetch_chip_fn=fetch_chip_fn,
        get_model_fn=get_model_fn,
    )
    features = extract_features(data)
    scores = calculate_scores(features)
    evidence = build_evidence(scores)
    decision = decide(evidence)
    period_summaries = _build_period_summaries(
        list(scores.zones), data.current_price, features.ma5
    )
    chip_score = (
        float(data.chip_row["total_score"])
        if data.chip_row is not None and data.chip_row.get("total_score") is not None
        else None
    )
    analysis_tips = _build_analysis_tips(
        period_summaries, data.current_price, features.ma5, chip_score
    )
    return {
        "pipeline_version": PIPELINE_VERSION,
        "analysis": {
            "symbol": data.symbol,
            "timeframe": data.timeframe,
            "analyzed_at": data.analyzed_at.isoformat(),
            "current_price": data.current_price,
            "period_summaries": period_summaries,
            "analysis_tips": analysis_tips,
            "chip_summary": scores.chip_summary,
            "model": {
                "version": data.model.version,
                "trained_at": data.model.trained_at,
                "config_hash": data.model.config_hash,
                "feature_names": data.model.feature_names,
            },
        },
        "features": {
            "global_trend": features.global_trend,
            "global_volatility": features.global_volatility,
        },
        "score": {
            "global_expected_value": scores.global_metrics["expected_value"],
            "global_confidence": scores.global_metrics["confidence"],
            "global_risk_reward_ratio": scores.global_metrics["risk_reward_ratio"],
        },
        "evidence": evidence.global_evidence,
        "decision": decision.summary,
        "zones": [
            {
                "data": {
                    "price_low": score.price_low,
                    "price_high": score.price_high,
                    "method": score.method,
                    "role": score.role,
                },
                "features": {
                    "support": item.support.values.__dict__,
                    "resistance": item.resistance.values.__dict__,
                },
                "score": _zone_score_to_dict(score),
                "evidence": zone_evidence,
                "lifecycle": {"status": "PENDING", "resolved_role": None},
            }
            for item, score, zone_evidence in zip(scores.features.zones, scores.zones, evidence.zone_evidence)
        ],
    }
