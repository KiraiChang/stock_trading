"""Orchestration for Data -> Features -> Score -> Evidence -> Decision."""
from __future__ import annotations

from typing import Any, Optional

from db import fetch_candles, fetch_latest_chip_score

try:
    from db import fetch_latest_sr_regression_governance
except ImportError:  # pragma: no cover - older CLI/test db shims may not expose this helper.
    def fetch_latest_sr_regression_governance(model_config_hash: str) -> dict | None:
        return None

from .decision_engine import build_decision_from_evidence
from .evidence import build_evidence
from .explain_engine import build_explanation, explain_zone
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
from .probability_engine import (
    build_analysis_probability_context,
    build_zone_probability_context,
    model_quality_flags,
)
from .scenario_engine import build_analysis_scenario, build_zone_scenario
from .types import ApproachDirection, ZoneType
from .zone_builder import ZoneBuilder, ZoneBuilderConfig, build_zone_builders, zone_builder_config_snapshot


def _resolve_runtime_builders(frame, builders: list[ZoneBuilder] | None) -> tuple[list[ZoneBuilder], dict[str, Any]]:
    if builders:
        return builders, {
            "enabled": False,
            "reason_code": "EXPLICIT_BUILDERS",
        }

    try:
        from .scoring import _adaptive_zone_builder_enabled, _adaptive_zone_builder_profile
        from .zone_builder import resolve_zone_builder_config_for_profile

        if _adaptive_zone_builder_enabled():
            atr_pct, average_range_pct = _adaptive_zone_builder_profile(frame)
            config, metadata = resolve_zone_builder_config_for_profile(atr_pct, average_range_pct)
            return build_zone_builders(config, include_recent_microstructure=True), metadata
    except Exception as exc:
        return build_zone_builders(include_recent_microstructure=True), {
            "enabled": False,
            "reason_code": "ADAPTIVE_ZONE_BUILDERS_ERROR",
            "error": str(exc),
            "config": zone_builder_config_snapshot(ZoneBuilderConfig()),
        }

    return build_zone_builders(include_recent_microstructure=True), {
        "enabled": False,
        "reason_code": "ADAPTIVE_ZONE_BUILDERS_DISABLED",
        "config": zone_builder_config_snapshot(ZoneBuilderConfig()),
    }


def load_data(
    symbol: str,
    timeframe: str,
    limit: int,
    builders: list[ZoneBuilder] | None,
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
    builders, builder_runtime_config = _resolve_runtime_builders(frame, builders)
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
        zone_builder_runtime_config=builder_runtime_config,
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
    from .ranking import _assign_tiers, _group_overlapping_zones, _sort_zone_scores
    from .scoring import (
        _build_chip_summary,
        _compute_global_metrics,
        score_zone,
    )

    data = features.data
    tiers = _assign_tiers([zone.width for zone in data.zones])
    groups, counts, family_counts, families = _group_overlapping_zones(list(data.zones))
    chip_score = (
        float(data.chip_row["total_score"])
        if data.chip_row is not None and data.chip_row.get("total_score") is not None
        else None
    )
    scores = [
        score_zone(
            data.frame, item.zone, data.current_price, data.model, features.global_trend,
            tier=tier, as_of_index=len(data.frame) - 1, overlap_group=group,
            confluence_count=count, confluence_family_count=family_count,
            confluence_families=family, chip_score=chip_score, chip_features=data.chip_features,
            feature_set=item,
        )
        for item, tier, group, count, family_count, family in zip(
            features.zones, tiers, groups, counts, family_counts, families
        )
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


def decide(
    evidence,
    previous_event_states: Optional[list[dict[str, Any]]] = None,
    model_governance: Optional[dict[str, Any]] = None,
) -> AnalysisDecision:
    return AnalysisDecision(
        evidence=evidence,
        summary=build_decision_from_evidence(
            evidence,
            previous_event_states=previous_event_states,
            model_governance=model_governance,
        ),
    )


def _entry_rank(state: str) -> int:
    return {
        "WAIT_CONFIRMATION": 1,
        "WAIT_DAILY_CONFIRM": 1,
        "PROBE_ALLOWED": 2,
        "PROBE_ENTRY": 2,
        "SMALL_ENTRY": 3,
        "ENTRY_ALLOWED": 5,
        "BUY": 5,
    }.get(str(state or ""), 5)


def _str_list(values: object) -> list[str]:
    if not isinstance(values, list):
        return []
    return [str(value) for value in values if value]


def _unique(items: list[str]) -> list[str]:
    out: list[str] = []
    for item in items:
        if item and item not in out:
            out.append(item)
    return out


_HEALTH_SEVERITY = {"UNRELIABLE": 3, "DEGRADED": 2, "HEALTHY": 1}


def _health_severity(state: str) -> int:
    # 未知狀態回 0（低於 HEALTHY），刻意不誤擋——與 gate 的「缺資料不誤擋」原則一致
    # （見 docs/sr-zone-scoring.md「gate 在該模型首次 decision-replay 寫入前是 no-op」）。
    # 但單靠這個會讓格式壞掉時完全沒有訊號，所以 _merge_regression_governance_gate 會另外
    # 記一筆 REGRESSION_GOVERNANCE_STATE_UNKNOWN warning
    # （見 docs/sr-zone-scoring.md「Production 端的 regression governance gate」）。
    return _HEALTH_SEVERITY.get(str(state or ""), 0)


def _is_known_health_state(state: str) -> bool:
    return str(state or "") in _HEALTH_SEVERITY


def _merge_regression_governance_gate(
    base: dict[str, Any],
    regression: dict[str, Any] | None,
) -> dict[str, Any]:
    if not regression:
        return base
    merged = dict(base)
    base_gate = dict((base.get("confidence_gate") or {}))
    regression_gate = dict((regression.get("confidence_gate") or {}))
    base_state = str(base.get("health_state") or "UNKNOWN")
    regression_state = str(regression.get("health_state") or regression_gate.get("state") or "UNKNOWN")
    final_state = base_state
    if _health_severity(regression_state) > _health_severity(base_state):
        final_state = regression_state

    quality_flags = _unique([*_str_list(base.get("quality_flags"))])
    warning_flags = _unique([*_str_list(base.get("warning_flags"))])
    blocking_flags = _unique([*_str_list(base.get("blocking_flags"))])
    reason_codes = _unique([
        *_str_list(base_gate.get("reason_codes")),
        *_str_list(regression_gate.get("reason_codes")),
    ])

    if regression_state == "UNRELIABLE":
        blocking_flags = _unique([*blocking_flags, "REGRESSION_GOVERNANCE_UNRELIABLE"])
        reason_codes = _unique([*reason_codes, "REGRESSION_GOVERNANCE_UNRELIABLE"])
    elif regression_state == "DEGRADED":
        warning_flags = _unique([*warning_flags, "REGRESSION_GOVERNANCE_DEGRADED"])
        reason_codes = _unique([*reason_codes, "REGRESSION_GOVERNANCE_DEGRADED"])
    elif not _is_known_health_state(regression_state):
        # 認不得的 health_state 不會升嚴重度（不誤擋），但一定要留下訊號，否則欄位改名或
        # 上游格式變動會讓整個 gate 靜默失效而沒人發現。
        warning_flags = _unique([*warning_flags, "REGRESSION_GOVERNANCE_STATE_UNKNOWN"])
        reason_codes = _unique([*reason_codes, "REGRESSION_GOVERNANCE_STATE_UNKNOWN"])

    allow_entry = bool(base_gate.get("allow_entry", True))
    if regression_gate.get("allow_entry") is False or final_state == "UNRELIABLE":
        allow_entry = False
        final_state = "UNRELIABLE"
    base_max = str(base_gate.get("max_entry_state") or "BUY")
    regression_max = str(regression_gate.get("max_entry_state") or "BUY")
    max_entry_state = base_max if _entry_rank(base_max) <= _entry_rank(regression_max) else regression_max
    if not allow_entry:
        max_entry_state = "WAIT_CONFIRMATION"
    elif final_state == "DEGRADED" and _entry_rank(max_entry_state) > _entry_rank("SMALL_ENTRY"):
        max_entry_state = "SMALL_ENTRY"

    merged.update({
        "health_state": final_state,
        "quality_flags": quality_flags,
        "warning_flags": warning_flags,
        "blocking_flags": blocking_flags,
        "confidence_gate": {
            "state": final_state,
            "allow_entry": allow_entry,
            "max_entry_state": max_entry_state,
            "reason_codes": reason_codes,
        },
        "regression_governance": regression,
    })
    return merged


def _latest_regression_governance(model_config_hash: str) -> dict[str, Any] | None:
    try:
        return fetch_latest_sr_regression_governance(model_config_hash)
    except Exception:
        return None


def run_pipeline(
    symbol: str,
    timeframe: str,
    limit: int,
    builders: list[ZoneBuilder] | None,
    fetch_candles_fn=fetch_candles,
    fetch_chip_fn=fetch_latest_chip_score,
    get_model_fn=get_model,
    previous_event_states: Optional[list[dict[str, Any]]] = None,
) -> dict[str, Any]:
    from .serialization import _zone_score_to_dict
    from .summaries import _build_period_summaries
    from .tips import _build_analysis_tips

    data = load_data(
        symbol, timeframe, limit, builders,
        fetch_candles_fn=fetch_candles_fn,
        fetch_chip_fn=fetch_chip_fn,
        get_model_fn=get_model_fn,
    )
    features = extract_features(data)
    scores = calculate_scores(features)
    evidence = build_evidence(scores)
    zone_probability_flags = model_quality_flags(scores)
    zone_probability_contexts = [
        build_zone_probability_context(score, zone_probability_flags)
        for score in scores.zones
    ]
    probability_context = build_analysis_probability_context(
        scores, zone_probability_contexts, zone_probability_flags
    )
    probability_context["health"] = _merge_regression_governance_gate(
        probability_context.get("health") or {},
        _latest_regression_governance(data.model.config_hash),
    )
    decision = decide(
        evidence,
        previous_event_states=previous_event_states,
        model_governance=probability_context["health"],
    )
    explanation = build_explanation(evidence, decision.summary)
    scenario = build_analysis_scenario(evidence, decision.summary)
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
            "zone_builder_runtime_config": data.zone_builder_runtime_config,
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
        "explanation": explanation,
        "scenario": scenario,
        "probability_context": probability_context,
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
                "explanation": explain_zone(score, zone_evidence),
                "scenario": build_zone_scenario(score),
                "probability_context": zone_probability_context,
                "lifecycle": {"status": "PENDING", "resolved_role": None},
            }
            for item, score, zone_evidence, zone_probability_context in zip(
                scores.features.zones,
                scores.zones,
                evidence.zone_evidence,
                zone_probability_contexts,
            )
        ],
    }
