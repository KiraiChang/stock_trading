"""SR Zone walk-forward evaluation runner.

This module validates SR Zone probabilities and zone outcomes. It is not a
capital simulation backtest; sizing, portfolio policy and order execution are
kept out of P0 so model/zone quality can be inspected directly.
"""
from __future__ import annotations

import argparse
import json
import math
import sys
from dataclasses import asdict
from datetime import datetime, timezone
from typing import Any, Optional

import numpy as np
import pandas as pd
from sqlalchemy import text
from sklearn.metrics import brier_score_loss, log_loss, roc_auc_score

from .dataset import DatasetConfig, build_training_dataset, load_ohlcv_csv, summarize_training_dataset
from .decision_engine import build_decision_summary
from .features import trend_slope, zone_volatility
from .model import FEATURE_COLUMNS, ModelBundle, load_model
from .ranking import _assign_tiers, _sort_zone_scores
from .event_engine import EXTREME_VOLUME_THRESHOLD
from .scoring import (
    VOLUME_CONFIRMATION_HIGH,
    VOLUME_CONFIRMATION_LOW,
    _build_chip_summary,
    _compute_global_metrics,
    score_zone,
)
from .zone_builder import (
    ATRZoneBuilderConfig,
    HIGH_VOLATILITY_THRESHOLD,
    LOW_VOLATILITY_THRESHOLD,
    ZoneBuilderConfig,
    build_zone_builders,
    volatility_bucket_from_profile,
    zone_builder_config_snapshot,
)

DEFAULT_EVALUATION_LIMIT = 1500
DEFAULT_PIPELINE_VERSION = "sr_zone_evaluation_p1"
DEFAULT_SWEEP_WIDTHS = [1.0, 1.25, 1.5, 1.75, 2.0]
DEFAULT_SWEEP_MAX_MERGES = [1.5, 2.0, 2.5]
VOLATILITY_PROFILE_LOOKBACK = 60
MIN_BUCKET_RECOMMENDATION_ROWS = 20
DEFAULT_REPLAY_MAX_ROWS = 200
MIN_GOVERNANCE_REPLAY_ROWS = 30
MIN_ENTRY_OUTCOME_ROWS = 5
MIN_DECISION_FIELD_COVERAGE = 0.80
MAX_DECISION_ERROR_RATE = 0.10
MIN_ENTRY_POSITIVE_RETURN_RATE = 0.45
MIN_ENTRY_AVERAGE_FORWARD_RETURN = 0.0
# replay_max_rows 是「所有股票加起來」的總預算，會跨股票均分（見 _allocate_replay_quota）。
# 預算不足以讓每檔都拿到 MIN_ROWS_PER_SYMBOL 時，寧可少覆蓋幾檔也不要每檔只分到 1～2 列——
# 那種樣本量算不出有意義的 outcome 統計。被放棄的股票會列進 replay_coverage.symbols_skipped。
MIN_ROWS_PER_SYMBOL = 5
# 股票覆蓋率低於此值時，治理結論降為 DEGRADED（warning，不是 blocking）。不用嚴格 1.0 是為了
# 容忍 watchlist 裡少數上市不久、K 棒不足的個股，不因此把全體進場限縮到 SMALL_ENTRY。
MIN_REPLAY_SYMBOL_COVERAGE = 0.9
# calibration bins：等寬切 [0,1]，量測「模型說幾成」與「實際幾成」的落差。
CALIBRATION_BIN_COUNT = 10
# 總樣本低於此值時 bin 內的 observed_rate 抖動過大，ECE 只供參考、不應拿來挑參數。
MIN_CALIBRATION_ROWS = 50
# sweep 每個 candidate 的 replay 預算。decision replay 每一列都要重建 zone 並跑完整
# decision engine，grid 有 N 組候選就要跑 N 次，因此刻意比單次 replay 的 200 小很多。
SWEEP_DEFAULT_REPLAY_MAX_ROWS = 50
# CLI 以單一 list（而非 per-symbol object）傳入 chip / governance context 時的 key，
# 語意是「這份資料套用到所有股票」。見 _load_symbol_rows_json 與 _rows_for_symbol。
DEFAULT_SYMBOL_ROWS_KEY = "__default__"


def _load_db_sources(symbols: list[str], timeframe: str, limit: int) -> list[tuple[str, str, pd.DataFrame]]:
    from db import fetch_candles

    sources: list[tuple[str, str, pd.DataFrame]] = []
    for symbol in symbols:
        symbol = symbol.strip()
        if not symbol:
            continue
        rows = fetch_candles(symbol, timeframe, limit=limit)
        if not rows:
            print(f"[warn] no candles for symbol={symbol}, skipped", file=sys.stderr)
            continue
        df = pd.DataFrame(rows)
        df["datetime"] = pd.to_datetime(df["timestamp"], unit="s", utc=True)
        df = df.set_index("datetime").sort_index()
        sources.append((symbol, timeframe, df[["open", "high", "low", "close", "volume"]].astype(float)))
    return sources


def _load_csv_sources(items: list[str], timeframe: str) -> list[tuple[str, str, pd.DataFrame]]:
    sources: list[tuple[str, str, pd.DataFrame]] = []
    for item in items:
        path, _, symbol = item.partition(":")
        sources.append((symbol or path, timeframe, load_ohlcv_csv(path)))
    return sources


def _clean_metric(value: float) -> Optional[float]:
    if value is None or math.isnan(value) or math.isinf(value):
        return None
    return float(value)


def new_evaluation_run_id(now: datetime | None = None) -> str:
    now = now or datetime.now(timezone.utc)
    return f"sr_eval_{now.strftime('%Y%m%d%H%M%S')}"


def _calibration_bins(y: np.ndarray, y_proba: np.ndarray) -> dict[str, Any]:
    """實測 reliability：把預測機率等寬切 bin，比較「說幾成」與「實際幾成」。

    與 `probability_engine._calibration_report` 不同——那支描述「訓練時有沒有做校準」，
    這裡是拿 holdout 資料實際量出來的偏差。

    空 bin 會保留（rows=0、其餘 null）：sweep 要跨 candidate 對齊比較，schema 必須穩定。
    """
    edges = np.linspace(0.0, 1.0, CALIBRATION_BIN_COUNT + 1)
    bins: list[dict[str, Any]] = []
    weighted_gap = 0.0
    max_gap: float | None = None
    total = int(len(y))
    for index in range(CALIBRATION_BIN_COUNT):
        lower = float(edges[index])
        upper = float(edges[index + 1])
        # 最後一個 bin 含右端點，否則 proba=1.0 會落在所有 bin 之外。
        if index == CALIBRATION_BIN_COUNT - 1:
            mask = (y_proba >= lower) & (y_proba <= upper)
        else:
            mask = (y_proba >= lower) & (y_proba < upper)
        rows = int(mask.sum())
        if rows == 0:
            bins.append({
                "lower": lower, "upper": upper, "rows": 0,
                "mean_predicted": None, "observed_rate": None, "gap": None,
            })
            continue
        mean_predicted = float(y_proba[mask].mean())
        observed_rate = float(y[mask].mean())
        gap = observed_rate - mean_predicted
        weighted_gap += abs(gap) * rows
        max_gap = abs(gap) if max_gap is None else max(max_gap, abs(gap))
        bins.append({
            "lower": lower,
            "upper": upper,
            "rows": rows,
            "mean_predicted": _clean_metric(mean_predicted),
            "observed_rate": _clean_metric(observed_rate),
            "gap": _clean_metric(gap),
        })
    # 分母用「真的落進 bin 的列數」而不是總列數：機率若是 NaN 之類的異常值，會落不進任何
    # bin，用 total 當分母會把 ECE 算成 0.0，讀起來像「完美校準」——這種靜默誤導比缺值危險。
    binned_rows = sum(int(item["rows"]) for item in bins)
    return {
        "schema_version": "sr_evaluation_calibration_v1",
        "bin_count": CALIBRATION_BIN_COUNT,
        "rows": total,
        "binned_rows": binned_rows,
        "bins": bins,
        # ECE：以樣本數加權的 |gap| 平均。MCE：最差的單一 bin 偏差。
        "expected_calibration_error": _clean_metric(weighted_gap / binned_rows) if binned_rows else None,
        "max_calibration_error": _clean_metric(max_gap) if max_gap is not None else None,
        # 樣本太少時 bin 內的 observed_rate 抖動很大，ECE 不該拿來做參數決策。
        "insufficient_sample": total < MIN_CALIBRATION_ROWS,
    }


def _binary_metrics(y_true: pd.Series, y_proba: np.ndarray) -> dict[str, Optional[float] | int]:
    y = y_true.to_numpy(dtype=int)
    metrics: dict[str, Any] = {"rows": int(len(y)), "positive_rows": int(y.sum())}
    if len(y) == 0:
        metrics.update({"auc": None, "brier_score": None, "log_loss": None, "calibration": None})
        return metrics
    metrics["calibration"] = _calibration_bins(y, np.asarray(y_proba, dtype=float))
    metrics["brier_score"] = _clean_metric(float(brier_score_loss(y, y_proba)))
    try:
        metrics["log_loss"] = _clean_metric(float(log_loss(y, y_proba, labels=[0, 1])))
    except ValueError:
        metrics["log_loss"] = None
    metrics["auc"] = _clean_metric(float(roc_auc_score(y, y_proba))) if len(np.unique(y)) > 1 else None
    return metrics


def _model_metrics(dataset: pd.DataFrame, bundle: ModelBundle | None) -> dict[str, Any]:
    if bundle is None:
        return {"model_available": False, "hold": None, "break": None}
    X = dataset[FEATURE_COLUMNS].astype(float)
    hold_proba = bundle.hold_model.predict_proba(X)[:, 1]
    break_proba = bundle.break_model.predict_proba(X)[:, 1]
    return {
        "model_available": True,
        "model_version": bundle.version,
        "model_trained_at": bundle.trained_at,
        "model_config_hash": bundle.config_hash,
        "hold": _binary_metrics(dataset["hold_label"], hold_proba),
        "break": _binary_metrics(dataset["break_label"], break_proba),
    }


def _zone_outcome_group(group: pd.DataFrame) -> dict[str, Any]:
    """分層（by_method / by_role / by_volatility_bucket）的 zone outcome 指標。

    三個比率的名稱與算法與 `_zone_outcomes` 的頂層**完全一致**，這是刻意的：分層自己另立一套
    key，前端就無法用同一組欄位渲染分層與頂層，也無法直接對照「這一組比整體好還是差」。
    2026-08-06 修掉的那個 bug 就是這樣來的——分層回傳 `hold_rate`/`break_rate`，前端讀 `support_hold_rate` 等三個
    不存在的 key，三張分層表的比率欄位因此永遠顯示 `—`，而且前後端測試各自用虛構的形狀
    互相印證，誰也沒發現。**改這裡時要同步確認 `frontend/src/lib/api/srZones.ts` 的
    `SRZoneOutcomeGroup`。**

    `hold_rate` 與 `support_hold_rate` 不是同一件事，不要合併或刪掉其中之一：

    - `hold_rate`：整組（支撐與壓力混在一起）的 `hold_label` 平均，即「zone 守住率」，不分方向。
      `_bucket_candidate_score` 以 0.7 的權重用它排序 bucket 建議。
    - `support_hold_rate` / `resistance_rejection_rate`：同一份 `hold_label` 依角色拆開看。
      在 `by_role` 分層裡兩者必有一個是 None（該組只有一種角色），這是正常的。
    """
    supports = group[group["is_support"] == 1]
    resistances = group[group["is_support"] == 0]
    return {
        "rows": int(len(group)),
        "hold_rate": _clean_metric(float(group["hold_label"].mean())),
        "support_hold_rate": _clean_metric(float(supports["hold_label"].mean())) if not supports.empty else None,
        "resistance_rejection_rate": (
            _clean_metric(float(resistances["hold_label"].mean())) if not resistances.empty else None
        ),
        "break_positive_rate": _clean_metric(float(group["break_label"].mean())),
        "average_forward_return": _clean_metric(float(group["forward_return"].mean())),
    }


def _zone_outcomes(dataset: pd.DataFrame, volatility_profiles: dict[str, dict[str, Any]] | None = None) -> dict[str, Any]:
    if dataset.empty:
        return {
            "rows": 0,
            "support_hold_rate": None,
            "resistance_rejection_rate": None,
            "break_positive_rate": None,
            "average_forward_return": None,
            "by_method": {},
            "by_role": {},
            "by_volatility_bucket": {},
        }
    by_method: dict[str, dict[str, Any]] = {}
    for method, group in dataset.groupby("method", sort=True):
        by_method[str(method)] = _zone_outcome_group(group)

    by_role: dict[str, dict[str, Any]] = {}
    for role, group in dataset.groupby("role", sort=True):
        by_role[str(role)] = _zone_outcome_group(group)

    by_volatility_bucket: dict[str, dict[str, Any]] = {}
    if volatility_profiles:
        profiled = dataset.copy()
        profiled["volatility_bucket"] = profiled["symbol"].map(
            lambda symbol: (volatility_profiles.get(str(symbol)) or {}).get("bucket") or "UNKNOWN_VOLATILITY"
        )
        for bucket, group in profiled.groupby("volatility_bucket", sort=True):
            by_volatility_bucket[str(bucket)] = _zone_outcome_group(group)

    supports = dataset[dataset["is_support"] == 1]
    resistances = dataset[dataset["is_support"] == 0]
    return {
        "rows": int(len(dataset)),
        "support_hold_rate": _clean_metric(float(supports["hold_label"].mean())) if not supports.empty else None,
        "resistance_rejection_rate": _clean_metric(float(resistances["hold_label"].mean())) if not resistances.empty else None,
        "break_positive_rate": _clean_metric(float(dataset["break_label"].mean())),
        "average_forward_return": _clean_metric(float(dataset["forward_return"].mean())),
        "by_method": by_method,
        "by_role": by_role,
        "by_volatility_bucket": by_volatility_bucket,
    }


def _dataset_range(sources: list[tuple[str, str, pd.DataFrame]]) -> tuple[Optional[str], Optional[str]]:
    starts = [df.index.min() for _, _, df in sources if not df.empty]
    ends = [df.index.max() for _, _, df in sources if not df.empty]
    if not starts or not ends:
        return None, None
    return pd.Timestamp(min(starts)).isoformat(), pd.Timestamp(max(ends)).isoformat()


def _atr_pct(df: pd.DataFrame, atr_period: int = 14) -> Optional[float]:
    if df.empty or len(df) < 2:
        return None
    high = df["high"].astype(float)
    low = df["low"].astype(float)
    close = df["close"].astype(float)
    previous_close = close.shift(1)
    true_range = pd.concat([
        high - low,
        (high - previous_close).abs(),
        (low - previous_close).abs(),
    ], axis=1).max(axis=1).dropna()
    if true_range.empty:
        return None
    atr = float(true_range.tail(atr_period).mean())
    last_close = float(close.iloc[-1])
    if last_close <= 0:
        return None
    return _clean_metric(atr / last_close)


def _volatility_profiles(
    sources: list[tuple[str, str, pd.DataFrame]],
    dataset: pd.DataFrame,
    lookback: int = VOLATILITY_PROFILE_LOOKBACK,
) -> dict[str, dict[str, Any]]:
    profiles: dict[str, dict[str, Any]] = {}
    for symbol, timeframe, df in sources:
        symbol = str(symbol)
        recent = df.tail(lookback)
        average_range_pct = None
        if not recent.empty:
            range_pct = ((recent["high"].astype(float) - recent["low"].astype(float)) / recent["close"].astype(float))
            average_range_pct = _clean_metric(float(range_pct.replace([np.inf, -np.inf], np.nan).dropna().mean()))
        atr_pct = _atr_pct(recent)
        touch_count = int((dataset["symbol"].astype(str) == symbol).sum()) if not dataset.empty else 0
        candle_count = int(len(df))
        touch_density = _clean_metric((touch_count / candle_count) * 100.0) if candle_count > 0 else None
        profiles[symbol] = {
            "symbol": symbol,
            "timeframe": timeframe,
            "bucket": volatility_bucket_from_profile(atr_pct, average_range_pct),
            "atr_pct": atr_pct,
            "average_range_pct": average_range_pct,
            "touch_count": touch_count,
            "candle_count": candle_count,
            "touch_density_per_100_bars": touch_density,
            "lookback_bars": min(lookback, candle_count),
            "thresholds": {
                "low_volatility_max": LOW_VOLATILITY_THRESHOLD,
                "high_volatility_min": HIGH_VOLATILITY_THRESHOLD,
            },
        }
    return profiles


def run_evaluation(
    symbols: Optional[list[str]] = None,
    csv_sources: Optional[list[str]] = None,
    timeframe: str = "1d",
    limit: int = DEFAULT_EVALUATION_LIMIT,
    model_path: Optional[str] = None,
    dataset_config: DatasetConfig | None = None,
    builder_config: ZoneBuilderConfig | None = None,
    run_id: Optional[str] = None,
    pipeline_version: str = DEFAULT_PIPELINE_VERSION,
) -> dict[str, Any]:
    sources: list[tuple[str, str, pd.DataFrame]] = []
    if symbols:
        sources.extend(_load_db_sources(symbols, timeframe, limit))
    if csv_sources:
        sources.extend(_load_csv_sources(csv_sources, timeframe))
    if not sources:
        raise ValueError("沒有任何可用的資料來源（請指定 symbols 或 csv）")

    dataset_config = dataset_config or DatasetConfig()
    builder_config = builder_config or ZoneBuilderConfig()
    builders = build_zone_builders(builder_config, include_recent_microstructure=False)
    dataset = build_training_dataset(sources, builders, dataset_config)
    if dataset.empty:
        raise ValueError("evaluation 資料集為空（來源資料太少，或沒有偵測到可標記的 zone touch）")

    warnings: list[str] = []
    bundle: ModelBundle | None = None
    if model_path:
        try:
            bundle = load_model(model_path)
        except Exception as exc:  # noqa: BLE001 - CLI report should survive missing/unloadable model.
            warnings.append(f"model unavailable: {exc}")

    dataset_from, dataset_to = _dataset_range(sources)
    model_metrics = _model_metrics(dataset, bundle)
    volatility_profiles = _volatility_profiles(sources, dataset)
    return {
        "schema_version": "sr_zone_evaluation_p0",
        "run_id": run_id or new_evaluation_run_id(),
        "pipeline_version": pipeline_version,
        "rows": int(len(dataset)),
        "sources": len(sources),
        "symbols": sorted({symbol for symbol, _, _ in sources}),
        "timeframe": timeframe,
        "split_method": "walk_forward",
        "model_config_hash": model_metrics.get("model_config_hash") or "",
        "dataset_from": dataset_from,
        "dataset_to": dataset_to,
        "dataset_config": asdict(dataset_config),
        "builder_config": zone_builder_config_snapshot(builder_config),
        "volatility_profiles": volatility_profiles,
        "dataset_summary": summarize_training_dataset(dataset),
        "zone_outcomes": _zone_outcomes(dataset, volatility_profiles),
        "model_metrics": model_metrics,
        "warnings": warnings,
    }


def write_evaluation_result(report: dict[str, Any], passed: Optional[bool] = None, engine_override: Any = None) -> None:
    """Persist an evaluation report to stock_sr_regression_results.

    The full report is stored in metrics_json; scalar model metrics are projected
    into existing queryable columns. Duplicate run_id is intentionally left to the
    database UNIQUE constraint.
    """
    if engine_override is None:
        from db import engine as engine_override

    model_metrics = report.get("model_metrics") or {}
    hold = model_metrics.get("hold") or {}
    brk = model_metrics.get("break") or {}
    effective_passed = _report_passed(report, passed)
    params = {
        "run_id": report.get("run_id") or new_evaluation_run_id(),
        "model_config_hash": report.get("model_config_hash") or "",
        "pipeline_version": report.get("pipeline_version") or DEFAULT_PIPELINE_VERSION,
        "dataset_from": report.get("dataset_from"),
        "dataset_to": report.get("dataset_to"),
        "split_method": report.get("split_method") or "walk_forward",
        "hold_auc": hold.get("auc"),
        "hold_brier_score": hold.get("brier_score"),
        "break_auc": brk.get("auc"),
        "break_brier_score": brk.get("brier_score"),
        "passed": effective_passed,
        "schema_version": report.get("schema_version") or "",
        "rows": report.get("rows"),
        "sources": report.get("sources"),
        "governance_health_state": _governance_value(report, "health_state") or "",
        "governance_strict_passed": _governance_value(report, "strict_passed"),
        "metrics_json": json.dumps(report, ensure_ascii=False),
    }
    sql = text("""
        INSERT INTO stock_sr_regression_results (
            run_id, model_config_hash, pipeline_version, dataset_from, dataset_to, split_method,
            hold_auc, hold_brier_score, break_auc, break_brier_score, passed,
            schema_version, result_rows, source_count, governance_health_state, governance_strict_passed, metrics_json
        ) VALUES (
            :run_id, :model_config_hash, :pipeline_version, :dataset_from, :dataset_to, :split_method,
            :hold_auc, :hold_brier_score, :break_auc, :break_brier_score, :passed,
            :schema_version, :rows, :sources, :governance_health_state, :governance_strict_passed, :metrics_json
        )
    """)
    with engine_override.begin() as conn:
        conn.execute(sql, params)


def _report_passed(report: dict[str, Any], override: Optional[bool]) -> Optional[bool]:
    if override is not None:
        return override
    governance = report.get("governance_evaluation")
    if isinstance(governance, dict) and isinstance(governance.get("passed"), bool):
        return bool(governance["passed"])
    return None


def _governance_value(report: dict[str, Any], key: str) -> Any:
    governance = report.get("governance_evaluation")
    if not isinstance(governance, dict):
        return None
    return governance.get(key)


def _float_grid(value: str | None, default: list[float]) -> list[float]:
    if value is None:
        return list(default)
    items = [item.strip() for item in value.split(",") if item.strip()]
    if not items:
        raise ValueError("參數 sweep grid 不可為空")
    parsed = [float(item) for item in items]
    if any(item <= 0 for item in parsed):
        raise ValueError("參數 sweep grid 必須全部大於 0")
    return parsed


def _load_symbol_rows_json(path: str) -> dict[str, list[dict]]:
    with open(path, "r", encoding="utf-8") as fh:
        raw = json.load(fh)
    if isinstance(raw, list):
        if not all(isinstance(row, dict) for row in raw):
            raise ValueError(f"{path} JSON list 必須只包含 object rows")
        return {DEFAULT_SYMBOL_ROWS_KEY: raw}
    if isinstance(raw, dict):
        result: dict[str, list[dict]] = {}
        for symbol, rows in raw.items():
            if not isinstance(rows, list) or not all(isinstance(row, dict) for row in rows):
                raise ValueError(f"{path} JSON object 的每個 symbol value 必須是 object row list")
            result[str(symbol)] = rows
        return result
    raise ValueError(f"{path} JSON 必須是 object 或 list")


def _sweep_token(value: float) -> str:
    return str(value).replace(".", "p").replace("-", "n")


def _sweep_decision_outcomes(replay_report: dict[str, Any] | None) -> dict[str, Any] | None:
    """從 candidate 的 replay report 抽出 decision 層比較用的欄位。

    只取穩定的幾組，不整包塞進來——sweep report 已經很大，而且 replay_rows 對參數比較沒用。
    """
    if not replay_report:
        return None
    outcome_summary = replay_report.get("outcome_summary") or {}
    return {
        "run_id": replay_report.get("run_id"),
        "rows": replay_report.get("rows"),
        "decision_fields_available": replay_report.get("decision_fields_available"),
        "replay_coverage": replay_report.get("replay_coverage"),
        "by_final_entry_state": outcome_summary.get("by_final_entry_state") or {},
        # AT_ZONE 比例是 T-003 計畫列的比較面向之一：zone 畫太寬時現價會一直落在區間內。
        "primary_zone_role_counts": outcome_summary.get("primary_zone_role_counts") or {},
        "at_zone_rate": outcome_summary.get("at_zone_rate"),
        "rr_summary": outcome_summary.get("rr_summary"),
    }


def _sweep_entry_performance(result: dict[str, Any]) -> dict[str, Any] | None:
    """把 candidate 的 entry 類 decision 結果彙總成單一可排序指標。

    只計入實際放行進場的狀態（`ENTRY_ALLOWED` / `PROBE_ALLOWED`）——`WAIT_CONFIRMATION`
    的後續報酬不代表這組參數的進場品質。
    """
    decision = result.get("decision_outcomes") or {}
    by_state = decision.get("by_final_entry_state") or {}
    rows = 0
    weighted_return = 0.0
    for state in ("ENTRY_ALLOWED", "PROBE_ALLOWED"):
        group = by_state.get(state) or {}
        group_rows = int(group.get("rows_with_forward_return") or 0)
        average = group.get("average_forward_return")
        if group_rows <= 0 or average is None:
            continue
        rows += group_rows
        weighted_return += float(average) * group_rows
    if rows <= 0:
        return None
    return {"rows": rows, "average_forward_return": _clean_metric(weighted_return / rows)}


def _best_sweep_entry_result(results: list[dict[str, Any]]) -> dict[str, Any] | None:
    """依 entry 後續報酬挑最佳 candidate。

    樣本數低於 MIN_ENTRY_OUTCOME_ROWS 的候選直接排除——用 3、5 列的結果挑參數只是在挑雜訊。
    """
    scored = []
    for result in results:
        performance = _sweep_entry_performance(result)
        if performance is None or performance["rows"] < MIN_ENTRY_OUTCOME_ROWS:
            continue
        if performance["average_forward_return"] is None:
            continue
        scored.append((result, performance))
    if not scored:
        return None
    best, performance = max(scored, key=lambda item: item[1]["average_forward_return"])
    return {
        "run_id": best["run_id"],
        "atr_width_multiplier": best["atr_width_multiplier"],
        "max_merge_width_multiple": best["max_merge_width_multiple"],
        "metric": "entry_average_forward_return",
        "value": performance["average_forward_return"],
        "rows": performance["rows"],
        "minimum_rows": MIN_ENTRY_OUTCOME_ROWS,
    }


def _sweep_result_summary(
    report: dict[str, Any],
    width: float,
    max_merge: float,
    replay_report: dict[str, Any] | None = None,
) -> dict[str, Any]:
    return {
        "run_id": report["run_id"],
        "atr_width_multiplier": width,
        "max_merge_width_multiple": max_merge,
        "rows": report["rows"],
        "dataset_summary": report["dataset_summary"],
        "zone_outcomes": report["zone_outcomes"],
        "model_metrics": report["model_metrics"],
        "builder_config": report["builder_config"],
        "decision_outcomes": _sweep_decision_outcomes(replay_report),
        "warnings": report["warnings"],
    }


def _best_sweep_result(results: list[dict[str, Any]], metric_key: str) -> dict[str, Any] | None:
    candidates = [
        result
        for result in results
        if (result.get("zone_outcomes") or {}).get(metric_key) is not None
    ]
    if not candidates:
        return None
    best = max(candidates, key=lambda result: float((result.get("zone_outcomes") or {})[metric_key]))
    return {
        "run_id": best["run_id"],
        "atr_width_multiplier": best["atr_width_multiplier"],
        "max_merge_width_multiple": best["max_merge_width_multiple"],
        "metric": metric_key,
        "value": (best.get("zone_outcomes") or {}).get(metric_key),
    }


def _bucket_candidate_score(metrics: dict[str, Any]) -> Optional[float]:
    hold_rate = metrics.get("hold_rate")
    average_forward_return = metrics.get("average_forward_return")
    if hold_rate is None and average_forward_return is None:
        return None
    score = 0.0
    weight = 0.0
    if hold_rate is not None:
        score += float(hold_rate) * 0.7
        weight += 0.7
    if average_forward_return is not None:
        score += float(average_forward_return) * 0.3
        weight += 0.3
    return _clean_metric(score / weight) if weight > 0 else None


def _bucket_recommendations(results: list[dict[str, Any]]) -> dict[str, dict[str, Any]]:
    buckets = sorted({
        str(bucket)
        for result in results
        for bucket in ((result.get("zone_outcomes") or {}).get("by_volatility_bucket") or {})
    })
    recommendations: dict[str, dict[str, Any]] = {}
    for bucket in buckets:
        ranked: list[dict[str, Any]] = []
        max_rows = 0
        for result in results:
            bucket_metrics = ((result.get("zone_outcomes") or {}).get("by_volatility_bucket") or {}).get(bucket)
            if not bucket_metrics:
                continue
            rows = int(bucket_metrics.get("rows") or 0)
            max_rows = max(max_rows, rows)
            score = _bucket_candidate_score(bucket_metrics)
            ranked.append({
                "atr_width_multiplier": result["atr_width_multiplier"],
                "max_merge_width_multiple": result["max_merge_width_multiple"],
                "score": score,
                "rows": rows,
                "metrics": bucket_metrics,
                "run_id": result["run_id"],
            })

        ranked.sort(
            key=lambda item: (
                item["score"] is not None,
                float(item["score"] or -1.0),
                item["rows"],
            ),
            reverse=True,
        )
        for idx, item in enumerate(ranked, start=1):
            item["rank"] = idx
        insufficient_sample = max_rows < MIN_BUCKET_RECOMMENDATION_ROWS
        recommendations[bucket] = {
            "bucket": bucket,
            "minimum_rows": MIN_BUCKET_RECOMMENDATION_ROWS,
            "max_rows": max_rows,
            "insufficient_sample": insufficient_sample,
            "recommended_config": None if insufficient_sample or not ranked else {
                "atr_width_multiplier": ranked[0]["atr_width_multiplier"],
                "max_merge_width_multiple": ranked[0]["max_merge_width_multiple"],
                "score": ranked[0]["score"],
                "rows": ranked[0]["rows"],
                "run_id": ranked[0]["run_id"],
            },
            "ranking": ranked,
        }
    return recommendations


def _model_metadata(bundle: ModelBundle | None) -> dict[str, Any]:
    if bundle is None:
        return {
            "available": False,
            "version": None,
            "config_hash": "",
            "trained_at": None,
        }
    return {
        "available": True,
        "version": bundle.version,
        "config_hash": bundle.config_hash,
        "trained_at": bundle.trained_at,
    }


def _decision_replay_plan(
    sources: list[tuple[str, str, pd.DataFrame]],
    dataset_config: DatasetConfig,
) -> list[dict[str, Any]]:
    forward_bars = max(dataset_config.forward_bars_support, dataset_config.forward_bars_resistance)
    plan: list[dict[str, Any]] = []
    for symbol, timeframe, df in sources:
        n = len(df)
        first_idx, last_idx = _candidate_bar_range(df, dataset_config)
        candidate_bars = max(0, last_idx - first_idx + 1)
        plan.append({
            "symbol": str(symbol),
            "timeframe": timeframe,
            "candle_count": int(n),
            "candidate_bars": int(candidate_bars),
            "start_as_of": pd.Timestamp(df.index[first_idx]).isoformat() if candidate_bars > 0 else None,
            "end_as_of": pd.Timestamp(df.index[last_idx]).isoformat() if candidate_bars > 0 else None,
            "min_history_bars": dataset_config.min_history_bars,
            "forward_bars": forward_bars,
        })
    return plan


def _historical_zone_score_summary(
    df: pd.DataFrame,
    as_of_index: int,
    bundle: ModelBundle,
    dataset_config: DatasetConfig,
    builder_config: ZoneBuilderConfig | None = None,
) -> dict[str, Any]:
    available = df.iloc[: as_of_index + 1]
    current_price = float(available["close"].iloc[-1])
    builders = build_zone_builders(builder_config, include_recent_microstructure=True)
    zones = [
        zone
        for builder in builders
        if len(available) >= builder.min_bars
        for zone in builder.build(available)
    ]
    if not zones:
        return {
            "available": False,
            "zone_count": 0,
            "error": "NO_ZONES",
            "global_trend": None,
            "global_volatility": None,
            "global_metrics": None,
            "primary_zone": None,
            "zone_scores": [],
        }
    tiers = _assign_tiers([zone.width for zone in zones])
    overall_trend = trend_slope(available, len(available) - 1)
    overall_volatility = zone_volatility(available, len(available) - 1)
    zone_scores = [
        score_zone(
            available,
            zone,
            current_price,
            bundle,
            overall_trend,
            tier=tier,
            as_of_index=len(available) - 1,
            lookback_bars=dataset_config.zone_lookback_bars,
            forward_bars=max(dataset_config.forward_bars_support, dataset_config.forward_bars_resistance),
            threshold_pct=max(dataset_config.threshold_pct_support, dataset_config.threshold_pct_resistance),
        )
        for zone, tier in zip(zones, tiers)
    ]
    sorted_scores = _sort_zone_scores(zone_scores)
    primary = sorted_scores[0] if sorted_scores else None
    global_metrics = _compute_global_metrics(sorted_scores)
    return {
        "available": bool(sorted_scores),
        "zone_count": len(sorted_scores),
        "error": None if sorted_scores else "NO_ZONE_SCORES",
        "global_trend": overall_trend,
        "global_volatility": overall_volatility,
        "global_metrics": global_metrics,
        "zone_scores": sorted_scores,
        "primary_zone": None if primary is None else {
            "role": primary.role,
            "tier": primary.tier,
            "price_low": primary.price_low,
            "price_high": primary.price_high,
            "confidence": primary.confidence,
            "trading_score": primary.trading_score,
            "risk_reward_ratio": primary.risk_reward_ratio,
            "volume_confirmation": primary.volume_confirmation,
            # 帶出數值而不只是分類後的 volume_confirmation：分層要能依量能強弱切桶，
            # 光有 CONFIRMED/NEUTRAL/WEAK 三檔看不出「多強」。
            "relative_volume": primary.relative_volume,
        },
    }


def _decision_fields_from_summary(summary: dict[str, Any]) -> dict[str, Any]:
    daily_confirmation = summary.get("daily_confirmation") or {}
    final_entry_permission = summary.get("final_entry_permission") or {}
    event_state_summary = summary.get("event_state_summary") or {}
    rr_gate = summary.get("rr_gate") or {}
    return {
        "market_bias": summary.get("market_bias"),
        "daily_confirmation_state": daily_confirmation.get("state"),
        "final_entry_state": final_entry_permission.get("state"),
        "rr_context": summary.get("rr_context"),
        "rr_gate": rr_gate,
        "market_events": summary.get("market_events") or [],
        "event_sequence": summary.get("event_sequence") or [],
        "event_market_state": event_state_summary.get("market_state"),
        "daily_price_action": summary.get("daily_price_action") or {},
    }


def _event_key(items: list[dict[str, Any]], field: str = "type") -> str:
    values = sorted({str(item.get(field)) for item in items if item.get(field)})
    return "+".join(values) if values else "NO_EVENT"


def _rr_bucket(value: Any) -> str:
    if value is None:
        return "RR_UNAVAILABLE"
    rr = float(value)
    if rr < 1.0:
        return "RR_LT_1"
    if rr < 1.5:
        return "RR_1_0_TO_1_5"
    if rr < 2.0:
        return "RR_1_5_TO_2_0"
    if rr < 3.0:
        return "RR_2_0_TO_3_0"
    return "RR_GTE_3"


def _volume_strength_bucket(value: Any) -> str:
    """量能強弱分桶。

    邊界**沿用既有常數**而不是另外訂一套：`VOLUME_CONFIRMATION_LOW/HIGH` 是
    `scoring._volume_confirmation` 判定 WEAK/NEUTRAL/CONFIRMED 的門檻，
    `EXTREME_VOLUME_THRESHOLD` 是 event_engine 判定爆量事件的門檻。

    **但門檻相同不代表主體相同——這一點必須知道，否則會誤判**（2026-08-10 補）：
    本函式吃的是 **primary zone 的** `relative_volume`，而同一列的 `volume_context`
    在偵測到 `EXTREME_VOLUME` 事件時會被覆寫成該值，那個事件是
    `event_engine.detect_market_events()` 用**全體 zone 的最大** `relative_volume`
    判定的。所以「primary zone 量能很弱、但別的 zone 爆量」時，同一列會同時出現
    `volume_context=EXTREME_VOLUME` 與 `volume_strength=VOL_LT_0_8`——**這不是 bug，
    是兩個不同主體**。要比較兩者時記得這件事；replay row 只帶 primary zone，
    拿不到全體 zone 的最大值。
    """
    if value is None:
        return "VOLUME_UNAVAILABLE"
    rv = float(value)
    if rv < VOLUME_CONFIRMATION_LOW:
        return "VOL_LT_0_8"
    if rv < VOLUME_CONFIRMATION_HIGH:
        return "VOL_0_8_TO_1_2"
    if rv < EXTREME_VOLUME_THRESHOLD:
        return "VOL_1_2_TO_2_5"
    return "VOL_GTE_2_5"


def _stop_distance_bucket(value: Any) -> str:
    """停損距離分桶（百分比）。

    邊界取自 2026-08-07 的 4,998 筆真實 report 分布（p50≈1.8%、p75≈6.6%、p90≈9.2%），
    切出來每組 193～1,222 筆，都高於前端的樣本不足門檻 20。
    """
    if value is None:
        return "STOP_DISTANCE_UNAVAILABLE"
    pct = float(value) * 100.0
    if pct < 1.0:
        return "SD_LT_1PCT"
    if pct < 3.0:
        return "SD_1_TO_3PCT"
    if pct < 6.0:
        return "SD_3_TO_6PCT"
    if pct < 10.0:
        return "SD_6_TO_10PCT"
    return "SD_GTE_10PCT"


def _rr_formula_state(rr_context: dict[str, Any]) -> str:
    """RR 公式為什麼算不出來——依**上游的實際成因**分桶，不是依欄位有無。

    **為什麼需要單獨看這個**：真實資料上 `by_rr_gate_reason_code` 的 `RR_UNAVAILABLE`
    佔 62%（3,109/4,998）——最大的一組，卻無法再往下拆。

    分桶（四個都可能出現，實測分布見 sr-zone-scoring.md）：

    - `RR_FORMULA_COMPLETE`：risk 與 reward 都算得出來。
    - `REWARD_MISSING`：有 risk、沒有 reward——zone 上方沒有可用的壓力區當目標
      （`target_basis` 為 `UNAVAILABLE` 或 `MARKET_ENTRY_TARGET_UNAVAILABLE`）。
    - `RISK_NOT_POSITIVE`：**entry 與 stop 都有，但 `entry - stop <= 0`**，
      風險距離不是正數（停損價在進場價之上或同價，例如價格已跌破 zone 而 stop 仍在上方）。
    - `ENTRY_OR_STOP_MISSING`：連 entry 或 stop 都沒有，通常是根本沒有 primary zone。

    **不要用「risk_price / reward_price 各自有沒有值」的四象限來分**（2026-08-10 更正）：
    `decision_engine._rr_context()` 的 `reward_price` 只在 `if risk > 0:` 區塊內賦值
    （見該檔 `risk_price = risk` 之後），所以 **reward 有值必然蘊含 risk 有值**——
    「只有 reward、沒有 risk」這個象限**永遠不可能出現**。先前的版本就有這個空桶，
    真實資料跑出 0 筆卻被當成「風險側從不缺」的證據，而真正的
    `entry - stop <= 0` 案例被靜靜併進「兩邊都缺」，與文件寫的「通常是沒有 primary zone」不符。
    """
    if rr_context.get("risk_price") is not None:
        return "RR_FORMULA_COMPLETE" if rr_context.get("reward_price") is not None else "REWARD_MISSING"
    # risk_price 是 None 有兩種成因，處置完全不同，不能混為一談。
    if rr_context.get("entry_price") is not None and rr_context.get("stop_price") is not None:
        return "RISK_NOT_POSITIVE"
    return "ENTRY_OR_STOP_MISSING"


def _priority_event_type(event_sequence: list[dict[str, Any]]) -> str:
    """依**固定優先序**取出代表事件。

    **這不是「時間上最早發生的事件」**，別照字面理解成先後順序：
    `decision_engine._event_sequence()` 是用固定優先序排的（`EXTREME_VOLUME` 10 →
    `HIGH_VOLUME_BREAKDOWN` 20 → `INTRADAY_RECLAIM` 30 → `REVERSAL_CANDIDATE` 40），
    不是偵測時間。而且同一列的 market_events 全部來自**同一根 K 棒**，
    `normalize_market_event()` 也沒有任何時間欄位——**單根 K 棒內「誰先發生」根本沒有定義**。

    因此本欄位實際上是 `market_event_types` 的**低基數粗化**（同一個事件類型集合必然得到
    同一個代表事件），資訊量不會超過它。留著的價值只有一個：組數少，樣本不會被切碎
    （真實資料上 4 組 vs 7 組）。要問「哪個事件先發生」得靠 event state 的 `age_bars`，
    那是跨 K 棒的存活時間，與本欄位無關。
    """
    for event in event_sequence:
        event_type = event.get("type")
        if event_type:
            return str(event_type)
    return "NO_EVENT"


def _event_count_bucket(event_sequence: list[dict[str, Any]]) -> str:
    """同時發生的事件數。事件越多代表當下訊號越密集，與確認成效的關係值得單獨看。"""
    count = sum(1 for event in event_sequence if event.get("type"))
    if count == 0:
        return "EVENTS_0"
    if count == 1:
        return "EVENTS_1"
    if count == 2:
        return "EVENTS_2"
    return "EVENTS_3_PLUS"


def _daily_confirmation_context(
    primary_zone: dict[str, Any] | None,
    fields: dict[str, Any],
) -> dict[str, Any]:
    market_events = list(fields.get("market_events") or [])
    event_sequence = list(fields.get("event_sequence") or [])
    rr_gate = fields.get("rr_gate") or {}
    rr_context = fields.get("rr_context") or {}
    volume_context = str((primary_zone or {}).get("volume_confirmation") or "NO_VOLUME_CONFIRMATION")
    market_event_types = {str(event.get("type")) for event in market_events if event.get("type")}
    if "EXTREME_VOLUME" in market_event_types:
        volume_context = "EXTREME_VOLUME"
    elif "HIGH_VOLUME_BREAKDOWN" in market_event_types:
        volume_context = "HIGH_VOLUME_BREAKDOWN"
    actual_rr = rr_gate.get("actual_rr")
    if actual_rr is None:
        actual_rr = rr_context.get("entry_rr")
    return {
        "volume_context": volume_context,
        "event_sequence": _event_key(event_sequence, "type"),
        "market_event_types": _event_key(market_events, "type"),
        "event_market_state": str(fields.get("event_market_state") or "UNKNOWN"),
        "rr_gate": "RR_QUALIFIED" if rr_gate.get("qualified") else "RR_BLOCKED",
        "rr_gate_reason_code": str(rr_gate.get("reason_code") or "RR_REASON_UNAVAILABLE"),
        "rr_bucket": _rr_bucket(actual_rr),
        "daily_price_follow_through": str((fields.get("daily_price_action") or {}).get("price_follow_through_state") or "UNKNOWN"),
        "daily_momentum_confirmation": str((fields.get("daily_price_action") or {}).get("momentum_confirmation_state") or "UNKNOWN"),
        # ── 以下為更細的分層維度 ───────────────────────────────────
        # volume_context 只有分類（CONFIRMED/NEUTRAL/WEAK…），看不出「多強」。
        "volume_strength": _volume_strength_bucket((primary_zone or {}).get("relative_volume")),
        # RR gate 的原始基礎值：停損距離與「現在能不能執行」是 gate 判斷的兩個前提，
        # 分層看它們才知道 RR_BLOCKED 是因為距離太寬還是根本進不了場。
        "stop_distance_bucket": _stop_distance_bucket(rr_context.get("stop_distance_pct")),
        "entry_executability": str(rr_context.get("entry_executability_reason_code") or "ENTRY_EXECUTABILITY_UNKNOWN"),
        # risk / reward 的齊備性——沒有這個維度就看不出 RR_UNAVAILABLE（真實資料上佔 62%）
        # 到底是缺目標價還是缺停損。
        "rr_formula_state": _rr_formula_state(rr_context),
        # event_sequence 是排序後串接的字串，看不出先後也切得太碎（4,998 筆散在 7 組）。
        # 拆成「第一個事件」與「事件數」兩個低基數維度。
        "primary_market_event": _priority_event_type(event_sequence),
        "market_event_count": _event_count_bucket(event_sequence),
    }


def _close_return(df: pd.DataFrame, idx: int, bars: int, current_price: float) -> Optional[float]:
    target_idx = idx + bars
    if target_idx >= len(df) or current_price <= 0:
        return None
    return _clean_metric((float(df["close"].iloc[target_idx]) / current_price) - 1.0)


def _excursion_window(
    df: pd.DataFrame,
    idx: int,
    current_price: float,
    primary_zone: dict[str, Any] | None,
    window_bars: int,
    rr_context: dict[str, Any] | None,
) -> dict[str, Any]:
    """確認之後那段窗口的「過程」：最大不利／有利偏移，以及多久才失效。

    既有的 next_/two_bar_ 指標只看**終點**，看不出中間曾經逆行多少——而停損是被路徑
    掃到的，不是被終點掃到的。

    偏移是**相對部位方向**的，不是原始漲跌幅：`max_adverse_excursion_pct` 一律為負值
    或 0（0 代表窗口內從未逆行），`max_favorable_excursion_pct` 一律為正值或 0。
    RESISTANCE 視為偏空，所以價格「上漲」才算不利，符號因此與原始報酬相反——
    這樣兩種 role 的數字才放得進同一個分布看。

    分母用 `current_price`（確認日收盤）而不是 `rr_context.entry_price`：entry_price
    在相當比例的列上是 None（見 by_rr_formula_state 的 ENTRY_OR_STOP_MISSING），拿它當
    分母會讓 MAE 恰好在最需要看停損的那些列上消失；而 current_price 與
    two_bar_close_return 同分母，「過程 vs 終點」才相減得起來。停損那層意義改由
    mae_to_stop_ratio 承接，把不可用性隔離在單一欄位。
    """
    unavailable = {
        "excursion_window_bars": 0,
        "max_adverse_excursion_pct": None,
        "max_favorable_excursion_pct": None,
        "bars_to_failure": None,
        "failure_state": "DIRECTION_UNDEFINED",
        "mae_to_stop_ratio": None,
    }
    role = str((primary_zone or {}).get("role") or "")
    # AT_ZONE 刻意不算：它的既有 label（AT_ZONE_TWO_BAR_RESOLVED_UP/DOWN）本身就沒有
    # 方向偏誤，硬指定一邊會做出沒有人能解釋的數字。
    if role not in ("SUPPORT", "RESISTANCE") or current_price <= 0:
        return unavailable

    end = min(idx + 1 + window_bars, len(df))
    if end <= idx + 1:
        return unavailable
    bars = end - (idx + 1)
    lows = df["low"].iloc[idx + 1 : end]
    highs = df["high"].iloc[idx + 1 : end]
    down_pct = (float(lows.min()) / current_price) - 1.0
    up_pct = (float(highs.max()) / current_price) - 1.0

    # NaN 要和 None 一樣視為「沒有邊界」：拿 NaN 去比較會全部得到 False，
    # 那會把「算不出來」靜靜地報成「窗口內沒失效」。
    def _boundary(key: str) -> Optional[float]:
        value = (primary_zone or {}).get(key)
        if value is None or pd.isna(value):
            return None
        return float(value)

    if role == "SUPPORT":
        adverse, favorable = down_pct, up_pct
        boundary = _boundary("price_low")
        breached = lows < boundary if boundary is not None else None
    else:
        # 偏空：漲為不利、跌為有利，兩者都取反號。
        adverse, favorable = -up_pct, -down_pct
        boundary = _boundary("price_high")
        breached = highs > boundary if boundary is not None else None

    # 沒逆行就是 0，不是正數——MAE 的定義是「對部位最不利的那一刻」，
    # 而「從未逆行」的不利程度是零。
    adverse = min(adverse, 0.0)
    favorable = max(favorable, 0.0)

    if breached is None:
        failure_state = "BOUNDARY_UNAVAILABLE"
        bars_to_failure = None
    elif bool(breached.any()):
        failure_state = "FAILED"
        bars_to_failure = int(breached.values.argmax()) + 1
    else:
        failure_state = "SURVIVED_WINDOW"
        bars_to_failure = None

    stop_distance_pct = (rr_context or {}).get("stop_distance_pct")
    mae_to_stop_ratio = None
    if stop_distance_pct is not None and float(stop_distance_pct) > 0:
        mae_to_stop_ratio = _clean_metric(abs(adverse) / float(stop_distance_pct))

    return {
        "excursion_window_bars": bars,
        "max_adverse_excursion_pct": _clean_metric(adverse),
        "max_favorable_excursion_pct": _clean_metric(favorable),
        "bars_to_failure": bars_to_failure,
        "failure_state": failure_state,
        "mae_to_stop_ratio": mae_to_stop_ratio,
    }


def _daily_confirmation_outcome(
    df: pd.DataFrame,
    idx: int,
    current_price: float,
    primary_zone: dict[str, Any] | None,
    daily_confirmation_state: str | None,
    window_bars: int = 0,
    rr_context: dict[str, Any] | None = None,
) -> dict[str, Any]:
    next_return = _close_return(df, idx, 1, current_price)
    two_bar_return = _close_return(df, idx, 2, current_price)
    if not daily_confirmation_state:
        return {
            "available": False,
            "reason_code": "DAILY_CONFIRMATION_STATE_UNAVAILABLE",
            "next_close_return": next_return,
            "two_bar_close_return": two_bar_return,
        }
    if not primary_zone:
        return {
            "available": False,
            "reason_code": "PRIMARY_ZONE_UNAVAILABLE",
            "state": daily_confirmation_state,
            "next_close_return": next_return,
            "two_bar_close_return": two_bar_return,
        }
    if idx + 1 >= len(df):
        return {
            "available": False,
            "reason_code": "NEXT_CANDLE_UNAVAILABLE",
            "state": daily_confirmation_state,
            "primary_role": primary_zone.get("role"),
            "next_close_return": next_return,
            "two_bar_close_return": two_bar_return,
        }

    role = str(primary_zone.get("role") or "")
    price_low = primary_zone.get("price_low")
    price_high = primary_zone.get("price_high")
    next_low = float(df["low"].iloc[idx + 1])
    next_high = float(df["high"].iloc[idx + 1])
    next_close = float(df["close"].iloc[idx + 1])
    two_bar_result = None
    next_zone_result = "UNCLASSIFIED"
    if role == "SUPPORT" and price_low is not None:
        next_zone_result = "SUPPORT_HELD" if next_low >= float(price_low) else "SUPPORT_BROKEN"
        if idx + 2 < len(df):
            min_low = float(df["low"].iloc[idx + 1 : idx + 3].min())
            two_close = float(df["close"].iloc[idx + 2])
            two_bar_result = (
                "SUPPORT_CONFIRMED"
                if min_low >= float(price_low) and two_close >= current_price
                else "SUPPORT_FAILED"
            )
    elif role == "RESISTANCE" and price_high is not None:
        next_zone_result = "RESISTANCE_REJECTED" if next_high <= float(price_high) else "RESISTANCE_BROKEN"
        if idx + 2 < len(df):
            max_high = float(df["high"].iloc[idx + 1 : idx + 3].max())
            two_close = float(df["close"].iloc[idx + 2])
            if max_high <= float(price_high) and two_close <= current_price:
                two_bar_result = "RESISTANCE_REJECTION_CONFIRMED"
            elif two_close > float(price_high):
                two_bar_result = "RESISTANCE_BREAKOUT_CONTINUATION"
            else:
                two_bar_result = "RESISTANCE_UNRESOLVED"
    elif role == "AT_ZONE" and price_low is not None and price_high is not None:
        if next_close > float(price_high):
            next_zone_result = "AT_ZONE_RESOLVED_UP"
        elif next_close < float(price_low):
            next_zone_result = "AT_ZONE_RESOLVED_DOWN"
        else:
            next_zone_result = "AT_ZONE_STILL_INSIDE"
        if idx + 2 < len(df):
            two_close = float(df["close"].iloc[idx + 2])
            if two_close > float(price_high):
                two_bar_result = "AT_ZONE_TWO_BAR_RESOLVED_UP"
            elif two_close < float(price_low):
                two_bar_result = "AT_ZONE_TWO_BAR_RESOLVED_DOWN"
            else:
                two_bar_result = "AT_ZONE_TWO_BAR_STILL_INSIDE"

    outcome = {
        "available": True,
        "state": daily_confirmation_state,
        "primary_role": role or None,
        "next_zone_result": next_zone_result,
        "two_bar_result": two_bar_result,
        "next_close_return": next_return,
        "two_bar_close_return": two_bar_return,
    }
    # 只掛在 available 的分支：summary 本來就只吃 available 的列，
    # 不可用的那幾個 early return 保持原樣，contract 不必跟著變寬。
    outcome.update(_excursion_window(df, idx, current_price, primary_zone, window_bars, rr_context))
    return outcome


def _chip_row_for_as_of(chip_rows: list[dict] | None, as_of: object) -> dict | None:
    if not chip_rows:
        return None
    ts = pd.Timestamp(as_of)
    if ts.tzinfo is None:
        ts = ts.tz_localize("UTC")
    as_of_date = ts.tz_convert("Asia/Taipei").date()
    latest: dict | None = None
    for row in chip_rows:
        # 缺 trade_date 或格式壞掉的 row 只跳過，不能讓整個 replay 拋例外——這份 context
        # 可能來自 /sr-zones/evaluate 的呼叫端（該端點不保證 row 帶齊欄位）。姊妹函式
        # _snapshot_for_as_of 本來就是這個行為，這裡補齊一致性。
        raw_date = row.get("trade_date")
        if raw_date is None:
            continue
        try:
            row_date = pd.Timestamp(raw_date).date()
        except (ValueError, TypeError):
            continue
        if row_date <= as_of_date:
            latest = row
        else:
            break
    return latest


def _sort_key_for_context_row(row: dict[str, Any]) -> pd.Timestamp:
    """context row 的排序鍵；取不到或解析不了時排到最前面（等同視為最舊）。"""
    raw_time = _snapshot_time(row)
    if raw_time is None:
        return pd.Timestamp.min.tz_localize("UTC")
    try:
        ts = pd.Timestamp(raw_time)
    except (ValueError, TypeError):
        return pd.Timestamp.min.tz_localize("UTC")
    if ts.tzinfo is None:
        ts = ts.tz_localize("UTC")
    return ts.tz_convert("UTC")


def _sorted_context(rows_by_symbol: dict[str, list[dict]] | None) -> dict[str, list[dict]] | None:
    """把 context 依時間升冪排好。

    `_chip_row_for_as_of` / `_snapshot_for_as_of` 都是掃到第一筆超過 as_of 就 `break`，
    這隱含「輸入必須升冪」。排程路徑的 Go repo 都是 `ORDER BY ... ASC`，但
    `POST /sr-zones/evaluate` 允許呼叫端自帶 `chip_scores_by_symbol` /
    `model_governance_by_symbol`；若對方給降冪，`break` 會讓結果停在**最舊**那筆而不是最新，
    且完全無警告。這裡集中排一次，成本 O(n log n) 且只做一次。
    現況說明見 docs/sr-zone-scoring.md「Replay context 的股票比對規則」。
    """
    if not rows_by_symbol:
        return rows_by_symbol
    return {
        symbol: sorted(rows, key=_sort_key_for_context_row)
        for symbol, rows in rows_by_symbol.items()
    }


def _rows_for_symbol(
    rows_by_symbol: dict[str, list[dict]] | None,
    symbol: str,
    single_source: bool,
) -> list[dict] | None:
    """取某檔股票的 replay context（籌碼 / 模型治理快照）。

    比對順序刻意是「精確 key → __default__ → 單一來源容錯」：

    - `__default__` 是 CLI 傳入單一 list（而非 per-symbol object）時的明示語意，
      代表「這份資料套用到所有股票」，見 `_load_symbol_rows_json`。
    - 只有**單一來源**時才容忍 key 命名不一致（例如 2330 vs 2330.TW）。多股票時
      絕不可退回「dict 只有一組就拿來用」——Go 端只把查得到資料的股票寫進 map，
      所以「多檔 replay、只有一檔有籌碼資料」正好會讓其他股票誤用別檔的資料
      （現況說明見 docs/sr-zone-scoring.md「Replay context 的股票比對規則」）。
    """
    if not rows_by_symbol:
        return None
    if symbol in rows_by_symbol:
        return rows_by_symbol[symbol]
    if DEFAULT_SYMBOL_ROWS_KEY in rows_by_symbol:
        return rows_by_symbol[DEFAULT_SYMBOL_ROWS_KEY]
    if single_source and len(rows_by_symbol) == 1:
        return next(iter(rows_by_symbol.values()))
    return None


def _snapshot_time(snapshot: dict[str, Any]) -> Any:
    return snapshot.get("as_of") or snapshot.get("trade_date") or snapshot.get("created_at")


def _snapshot_for_as_of(snapshots: list[dict] | None, as_of: object) -> dict | None:
    if not snapshots:
        return None
    ts = pd.Timestamp(as_of)
    if ts.tzinfo is None:
        ts = ts.tz_localize("UTC")
    as_of_utc = ts.tz_convert("UTC")
    latest: dict | None = None
    for snapshot in snapshots:
        raw_time = _snapshot_time(snapshot)
        if raw_time is None:
            continue
        snapshot_ts = pd.Timestamp(raw_time)
        if snapshot_ts.tzinfo is None:
            snapshot_ts = snapshot_ts.tz_localize("UTC")
        if snapshot_ts.tz_convert("UTC") <= as_of_utc:
            latest = snapshot
        else:
            break
    return latest




def _candidate_bar_range(df: pd.DataFrame, dataset_config: DatasetConfig) -> tuple[int, int]:
    """回傳某檔股票可作為 as-of 的 index 範圍（含頭含尾）。

    起點要留足 min_history_bars 才有辦法建 zone，終點要留 forward_bars 才算得出 label。
    """
    forward_bars = max(dataset_config.forward_bars_support, dataset_config.forward_bars_resistance)
    return dataset_config.min_history_bars, len(df) - forward_bars - 1


def _allocate_replay_quota(
    sources: list[tuple[str, str, pd.DataFrame]],
    dataset_config: DatasetConfig,
    max_rows: int,
) -> tuple[dict[str, int], list[str]]:
    """把 max_rows 這個總預算分配給各股票。

    回傳 (每檔配額, 被放棄的股票)。分配規則：
    1. 先濾掉 candidate_bars 為 0 的股票（K 棒不足，無法產生任何 as-of）。
    2. 預算不足以讓每檔都拿到 MIN_ROWS_PER_SYMBOL 時，只覆蓋前 N 檔（依 sources 順序，
       決定性），其餘列入 skipped。
    3. 平均分配後，配額超過該檔 candidate_bars 的部分會被收回，重新分給還有餘裕的股票，
       重複到收斂為止——否則短天期個股會白白浪費預算。
    """
    candidates: list[tuple[str, int]] = []
    skipped: list[str] = []
    seen: set[str] = set()
    for symbol, _timeframe, df in sources:
        symbol_key = str(symbol)
        # 配額是以 symbol 為 key 的，同一檔重複出現（例如呼叫端送了重複的 symbols，或兩份
        # 指向同一檔的 CSV）只能算一次；否則 _decision_replay_rows 會對每個 source 都套用
        # 同一份配額，實際產出直接超過總預算。
        if symbol_key in seen:
            continue
        seen.add(symbol_key)
        first_idx, last_idx = _candidate_bar_range(df, dataset_config)
        candidate_bars = max(0, last_idx - first_idx + 1)
        if candidate_bars <= 0:
            skipped.append(symbol_key)
            continue
        candidates.append((symbol_key, candidate_bars))

    if not candidates or max_rows <= 0:
        return {}, skipped + [symbol for symbol, _ in candidates]

    affordable = max(1, max_rows // MIN_ROWS_PER_SYMBOL)
    if len(candidates) > affordable:
        skipped.extend(symbol for symbol, _ in candidates[affordable:])
        candidates = candidates[:affordable]

    quota = {symbol: 0 for symbol, _ in candidates}
    remaining = max_rows
    open_symbols = [symbol for symbol, _ in candidates]
    headroom = dict(candidates)
    while remaining > 0 and open_symbols:
        share = remaining // len(open_symbols)
        if share <= 0:
            # 餘數不足以再均分，依序每檔補 1 列直到用完。
            for symbol in list(open_symbols):
                if remaining <= 0:
                    break
                quota[symbol] += 1
                headroom[symbol] -= 1
                remaining -= 1
                if headroom[symbol] <= 0:
                    open_symbols.remove(symbol)
            break
        for symbol in list(open_symbols):
            take = min(share, headroom[symbol])
            quota[symbol] += take
            headroom[symbol] -= take
            remaining -= take
            if headroom[symbol] <= 0:
                open_symbols.remove(symbol)

    return {symbol: count for symbol, count in quota.items() if count > 0}, skipped


def _replay_coverage(
    sources: list[tuple[str, str, pd.DataFrame]],
    quota_by_symbol: dict[str, int],
    symbols_skipped: list[str],
) -> dict[str, Any]:
    """報告這次 replay 實際涵蓋了哪些股票。

    report 的 symbols / sources / replay_plan 描述的是「要求驗證的範圍」，這裡描述的是
    「實際驗證到的範圍」。兩者在預算不足時會不一致，必須讓讀報告的人看得出來。
    """
    requested = sorted({str(symbol) for symbol, _timeframe, _df in sources})
    covered = sorted(symbol for symbol, count in quota_by_symbol.items() if count > 0)
    ratio = (len(covered) / len(requested)) if requested else None
    return {
        "symbols_requested": len(requested),
        "symbols_covered": len(covered),
        "symbols_skipped": sorted(set(symbols_skipped)),
        "coverage_ratio": _clean_metric(ratio) if ratio is not None else None,
        "quota_by_symbol": dict(sorted(quota_by_symbol.items())),
        "window_mode": "latest",
    }


def _decision_replay_rows(
    sources: list[tuple[str, str, pd.DataFrame]],
    dataset_config: DatasetConfig,
    quota_by_symbol: dict[str, int],
    bundle: ModelBundle | None = None,
    chip_scores_by_symbol: dict[str, list[dict]] | None = None,
    model_governance_by_symbol: dict[str, list[dict]] | None = None,
    builder_config: ZoneBuilderConfig | None = None,
) -> list[dict[str, Any]]:
    forward_bars = max(dataset_config.forward_bars_support, dataset_config.forward_bars_resistance)
    rows: list[dict[str, Any]] = []
    previous_event_states_by_symbol: dict[str, list[dict[str, Any]]] = {}
    # 只有單一來源時，context 的 key 命名不一致才容許用「唯一那組」補上（見 _rows_for_symbol）。
    single_source = len(sources) == 1
    # 只排一次；as-of 比對靠 break 提前結束，依賴升冪輸入。
    chip_scores_by_symbol = _sorted_context(chip_scores_by_symbol)
    model_governance_by_symbol = _sorted_context(model_governance_by_symbol)
    replayed: set[str] = set()
    for symbol, timeframe, df in sources:
        symbol_key = str(symbol)
        # 配額以 symbol 為 key，同一檔只跑一次（配額分配端也是這樣去重的）。
        if symbol_key in replayed:
            continue
        quota = quota_by_symbol.get(symbol_key, 0)
        if quota <= 0:
            continue
        replayed.add(symbol_key)
        first_idx, last_idx = _candidate_bar_range(df, dataset_config)
        if last_idx < first_idx:
            continue
        # 取「最新」的 quota 根：模型健康度 gate 是拿來限制當下進場的，用最近盤勢驗證才有
        # 代表性。維持連續區間（而非等間距抽樣），event lifecycle 的
        # previous_event_states 才有連續的前一根狀態可以接。
        window_start = max(first_idx, last_idx - quota + 1)
        for idx in range(window_start, last_idx + 1):
            current_price = float(df["close"].iloc[idx])
            as_of = df.index[idx]
            future_price = float(df["close"].iloc[idx + forward_bars])
            forward_return = _clean_metric((future_price / current_price) - 1.0) if current_price > 0 else None
            candle_open = float(df["open"].iloc[idx])
            candle_high = float(df["high"].iloc[idx])
            candle_low = float(df["low"].iloc[idx])
            candle_close = float(df["close"].iloc[idx])
            previous_candle_close = float(df["close"].iloc[idx - 1]) if idx > 0 else None
            zone_score_available = False
            zone_score_error = None
            zone_count = 0
            primary_zone = None
            global_trend = None
            global_volatility = None
            global_metrics = None
            chip_row = _chip_row_for_as_of(
                _rows_for_symbol(chip_scores_by_symbol, symbol_key, single_source),
                as_of,
            )
            chip_summary = _build_chip_summary(chip_row)
            model_governance_snapshot = _snapshot_for_as_of(
                _rows_for_symbol(model_governance_by_symbol, symbol_key, single_source),
                as_of,
            )
            model_governance = None
            model_governance_source_time = None
            if model_governance_snapshot is not None:
                model_governance = dict(model_governance_snapshot)
                model_governance_source_time = _snapshot_time(model_governance_snapshot)
            market_bias = None
            daily_confirmation_state = None
            final_entry_state = None
            rr_context = None
            rr_gate = None
            decision_field_context: dict[str, Any] = {}
            decision_fields_available = False
            decision_error = None
            event_lifecycle_replay_available = False
            event_state_count = 0
            active_event_count = 0
            resolved_event_count = 0
            expired_event_count = 0
            if bundle is not None:
                try:
                    zone_summary = _historical_zone_score_summary(
                        df, idx, bundle, dataset_config, builder_config
                    )
                    zone_score_available = bool(zone_summary["available"])
                    zone_score_error = zone_summary["error"]
                    zone_count = int(zone_summary["zone_count"])
                    primary_zone = zone_summary["primary_zone"]
                    global_trend = zone_summary["global_trend"]
                    global_volatility = zone_summary["global_volatility"]
                    global_metrics = zone_summary["global_metrics"]
                    zone_scores = zone_summary["zone_scores"]
                    if zone_score_available and global_trend is not None and global_volatility is not None and global_metrics:
                        decision_summary = build_decision_summary(
                            zone_scores,
                            current_price,
                            global_trend,
                            global_volatility,
                            global_metrics,
                            dict(chip_summary),
                            bundle,
                            candle_open=candle_open,
                            candle_high=candle_high,
                            candle_low=candle_low,
                            candle_close=candle_close,
                            previous_candle_close=previous_candle_close,
                            model_governance=model_governance,
                            previous_event_states=previous_event_states_by_symbol.get(symbol_key),
                        )
                        fields = _decision_fields_from_summary(decision_summary)
                        market_bias = fields["market_bias"]
                        daily_confirmation_state = fields["daily_confirmation_state"]
                        final_entry_state = fields["final_entry_state"]
                        rr_context = fields["rr_context"]
                        rr_gate = fields["rr_gate"]
                        decision_field_context = fields
                        decision_fields_available = True
                        event_state_summary = decision_summary.get("event_state_summary") or {}
                        states = list(event_state_summary.get("states") or [])
                        active_events = list(event_state_summary.get("active") or [])
                        resolved_events = list(event_state_summary.get("resolved") or [])
                        expired_events = list(event_state_summary.get("expired") or [])
                        previous_event_states_by_symbol[symbol_key] = states
                        event_lifecycle_replay_available = True
                        event_state_count = len(states)
                        active_event_count = len(active_events)
                        resolved_event_count = len(resolved_events)
                        expired_event_count = len(expired_events)
                except Exception as exc:  # noqa: BLE001 - one replay row must not abort the whole report.
                    if zone_score_available:
                        decision_error = str(exc)
                    else:
                        zone_score_error = str(exc)
            daily_confirmation_outcome = _daily_confirmation_outcome(
                df,
                idx,
                current_price,
                primary_zone,
                daily_confirmation_state,
                # 窗口沿用 forward_bars：_candidate_bar_range 已經為它預留尾端，
                # idx + forward_bars 必定在界內，不必另立一個要解釋、要調、要測的旋鈕。
                forward_bars,
                rr_context,
            )
            daily_confirmation_context = _daily_confirmation_context(primary_zone, decision_field_context)
            rows.append({
                "symbol": str(symbol),
                "timeframe": timeframe,
                "as_of": pd.Timestamp(as_of).isoformat(),
                "current_price": current_price,
                "candle_open": candle_open,
                "candle_high": candle_high,
                "candle_low": candle_low,
                "candle_close": candle_close,
                "previous_candle_close": previous_candle_close,
                "forward_bars": forward_bars,
                "forward_return": forward_return,
                "next_close_return": daily_confirmation_outcome.get("next_close_return"),
                "two_bar_close_return": daily_confirmation_outcome.get("two_bar_close_return"),
                "zone_score_available": zone_score_available,
                "zone_score_error": zone_score_error,
                "zone_count": zone_count,
                "primary_zone": primary_zone,
                "global_trend": global_trend,
                "global_volatility": global_volatility,
                "global_metrics": global_metrics,
                "chip_summary": chip_summary,
                "model_governance_available": model_governance is not None,
                "model_governance_source_time": model_governance_source_time,
                "model_governance": model_governance,
                "market_bias": market_bias,
                "daily_confirmation_state": daily_confirmation_state,
                "daily_confirmation_outcome": daily_confirmation_outcome,
                "daily_confirmation_context": daily_confirmation_context,
                "final_entry_state": final_entry_state,
                "rr_context": rr_context,
                "rr_gate": rr_gate,
                "decision_fields_available": decision_fields_available,
                "decision_error": decision_error,
                "event_lifecycle_replay_available": event_lifecycle_replay_available,
                "event_state_count": event_state_count,
                "active_event_count": active_event_count,
                "resolved_event_count": resolved_event_count,
                "expired_event_count": expired_event_count,
            })
    return rows


def _decision_replay_outcome_summary(rows: list[dict[str, Any]]) -> dict[str, Any]:
    if not rows:
        return {
            "rows": 0,
            "rows_by_symbol": {},
            "average_forward_return": None,
            "daily_confirmation_summary": _daily_confirmation_summary([]),
            "by_final_entry_state": {},
            "by_daily_confirmation_state": {},
            "by_market_bias": {},
            "primary_zone_role_counts": {},
            "rows_with_primary_zone": 0,
            "at_zone_rate": None,
            "rr_summary": _rr_summary([]),
        }
    rows_by_symbol: dict[str, int] = {}
    zone_score_error_counts: dict[str, int] = {}
    decision_error_counts: dict[str, int] = {}
    final_entry_state_counts: dict[str, int] = {}
    daily_confirmation_state_counts: dict[str, int] = {}
    forward_returns: list[float] = []
    for row in rows:
        symbol = str(row["symbol"])
        rows_by_symbol[symbol] = rows_by_symbol.get(symbol, 0) + 1
        if row.get("zone_score_error"):
            error = str(row["zone_score_error"])
            zone_score_error_counts[error] = zone_score_error_counts.get(error, 0) + 1
        if row.get("decision_error"):
            error = str(row["decision_error"])
            decision_error_counts[error] = decision_error_counts.get(error, 0) + 1
        if row.get("final_entry_state"):
            state = str(row["final_entry_state"])
            final_entry_state_counts[state] = final_entry_state_counts.get(state, 0) + 1
        if row.get("daily_confirmation_state"):
            state = str(row["daily_confirmation_state"])
            daily_confirmation_state_counts[state] = daily_confirmation_state_counts.get(state, 0) + 1
        if row.get("forward_return") is not None:
            forward_returns.append(float(row["forward_return"]))

    # primary zone 的角色分布。AT_ZONE 代表現價還落在區間內、方向尚未解析出來——
    # 比例過高通常表示 zone 被畫得太寬（見 T-003 的 ATR 寬度調校）。
    # 只有 replay 路徑量得到這個：evaluation dataset 的 role 由 approach direction 二選一
    # 決定，永遠不會是 AT_ZONE。
    primary_zone_role_counts: dict[str, int] = {}
    for row in rows:
        primary = row.get("primary_zone")
        if not primary:
            continue
        role = str(primary.get("role") or "UNKNOWN")
        primary_zone_role_counts[role] = primary_zone_role_counts.get(role, 0) + 1
    rows_with_primary_zone = sum(primary_zone_role_counts.values())
    at_zone_rate = (
        _clean_metric(primary_zone_role_counts.get("AT_ZONE", 0) / rows_with_primary_zone)
        if rows_with_primary_zone
        else None
    )

    return {
        "rows": len(rows),
        "rows_by_symbol": dict(sorted(rows_by_symbol.items())),
        "average_forward_return": _clean_metric(float(np.mean(forward_returns))) if forward_returns else None,
        "rows_with_outcome": len([row for row in rows if row.get("forward_return") is not None]),
        "rows_with_zone_score": len([row for row in rows if row.get("zone_score_available")]),
        "rows_with_global_context": len([row for row in rows if row.get("global_metrics") is not None]),
        "rows_with_chip_context": len([row for row in rows if row.get("chip_summary") is not None]),
        "rows_with_non_missing_chip": len([
            row for row in rows if row.get("chip_summary") and not row["chip_summary"].get("missing")
        ]),
        "chip_missing_rows": len([
            row for row in rows if row.get("chip_summary") and row["chip_summary"].get("missing")
        ]),
        "rows_with_model_governance": len([row for row in rows if row.get("model_governance_available")]),
        "model_governance_missing_rows": len([row for row in rows if not row.get("model_governance_available")]),
        "rows_with_decision_fields": len([row for row in rows if row.get("decision_fields_available")]),
        "rows_with_event_lifecycle": len([row for row in rows if row.get("event_lifecycle_replay_available")]),
        "primary_zone_role_counts": dict(sorted(primary_zone_role_counts.items())),
        "rows_with_primary_zone": rows_with_primary_zone,
        "at_zone_rate": at_zone_rate,
        "zone_score_error_counts": dict(sorted(zone_score_error_counts.items())),
        "decision_error_counts": dict(sorted(decision_error_counts.items())),
        "final_entry_state_counts": dict(sorted(final_entry_state_counts.items())),
        "daily_confirmation_state_counts": dict(sorted(daily_confirmation_state_counts.items())),
        "daily_confirmation_summary": _daily_confirmation_summary(rows),
        "by_final_entry_state": _decision_outcome_groups(rows, "final_entry_state"),
        "by_daily_confirmation_state": _decision_outcome_groups(rows, "daily_confirmation_state"),
        "by_market_bias": _decision_outcome_groups(rows, "market_bias"),
        "rr_summary": _rr_summary(rows),
    }


def _daily_confirmation_summary(rows: list[dict[str, Any]]) -> dict[str, Any]:
    outcomes: list[dict[str, Any]] = []
    for row in rows:
        outcome = row.get("daily_confirmation_outcome") or {}
        if not outcome.get("available"):
            continue
        item = dict(outcome)
        item.update(row.get("daily_confirmation_context") or {})
        outcomes.append(item)
    return {
        "rows": len(outcomes),
        "support_next_hold_rate": _outcome_rate(outcomes, "next_zone_result", "SUPPORT_HELD", primary_role="SUPPORT"),
        "support_two_bar_confirm_rate": _outcome_rate(
            outcomes,
            "two_bar_result",
            "SUPPORT_CONFIRMED",
            primary_role="SUPPORT",
        ),
        "resistance_next_rejection_rate": _outcome_rate(
            outcomes,
            "next_zone_result",
            "RESISTANCE_REJECTED",
            primary_role="RESISTANCE",
        ),
        "resistance_next_breakout_rate": _outcome_rate(
            outcomes,
            "next_zone_result",
            "RESISTANCE_BROKEN",
            primary_role="RESISTANCE",
        ),
        "resistance_two_bar_breakout_continuation_rate": _outcome_rate(
            outcomes,
            "two_bar_result",
            "RESISTANCE_BREAKOUT_CONTINUATION",
            primary_role="RESISTANCE",
        ),
        "average_next_close_return": _metric_average(outcomes, "next_close_return"),
        "average_two_bar_close_return": _metric_average(outcomes, "two_bar_close_return"),
        "positive_two_bar_return_rate": _metric_positive_rate(outcomes, "two_bar_close_return"),
        "negative_two_bar_return_rate": _metric_negative_rate(outcomes, "two_bar_close_return"),
        "failure_distribution": _daily_confirmation_failure_distribution(outcomes),
        # 過程（drawdown-like failure window）——終點報酬看不出停損有沒有被路徑掃到。
        "excursion": _excursion_summary(outcomes),
        "by_state": _daily_confirmation_groups(outcomes, "state"),
        "by_primary_role": _daily_confirmation_groups(outcomes, "primary_role"),
        "by_volume_context": _daily_confirmation_groups(outcomes, "volume_context"),
        "by_event_sequence": _daily_confirmation_groups(outcomes, "event_sequence"),
        "by_market_event_types": _daily_confirmation_groups(outcomes, "market_event_types"),
        "by_event_market_state": _daily_confirmation_groups(outcomes, "event_market_state"),
        "by_rr_gate": _daily_confirmation_groups(outcomes, "rr_gate"),
        "by_rr_gate_reason_code": _daily_confirmation_groups(outcomes, "rr_gate_reason_code"),
        "by_rr_bucket": _daily_confirmation_groups(outcomes, "rr_bucket"),
        # 更細的分層（2026-08-07 補）：量能強弱數值、RR gate 的原始基礎值、事件順序。
        # 分桶邊界沿用既有常數或取自真實分布，見各 _*_bucket 函式的說明。
        "by_volume_strength": _daily_confirmation_groups(outcomes, "volume_strength"),
        "by_stop_distance_bucket": _daily_confirmation_groups(outcomes, "stop_distance_bucket"),
        "by_entry_executability": _daily_confirmation_groups(outcomes, "entry_executability"),
        "by_rr_formula_state": _daily_confirmation_groups(outcomes, "rr_formula_state"),
        "by_primary_market_event": _daily_confirmation_groups(outcomes, "primary_market_event"),
        "by_market_event_count": _daily_confirmation_groups(outcomes, "market_event_count"),
    }


def _outcome_rate(
    outcomes: list[dict[str, Any]],
    field: str,
    expected: str,
    primary_role: str | None = None,
) -> Optional[float]:
    filtered = [
        outcome
        for outcome in outcomes
        if primary_role is None or outcome.get("primary_role") == primary_role
    ]
    if not filtered:
        return None
    return _clean_metric(sum(1 for outcome in filtered if outcome.get(field) == expected) / len(filtered))


def _metric_values(rows: list[dict[str, Any]], field: str) -> list[float]:
    return [
        float(row[field])
        for row in rows
        if row.get(field) is not None
    ]


def _metric_average(rows: list[dict[str, Any]], field: str) -> Optional[float]:
    values = _metric_values(rows, field)
    return _clean_metric(float(np.mean(values))) if values else None


def _metric_positive_rate(rows: list[dict[str, Any]], field: str) -> Optional[float]:
    values = np.array(_metric_values(rows, field), dtype=float)
    return _clean_metric(float(np.mean(values > 0))) if len(values) else None


def _metric_negative_rate(rows: list[dict[str, Any]], field: str) -> Optional[float]:
    values = np.array(_metric_values(rows, field), dtype=float)
    return _clean_metric(float(np.mean(values < 0))) if len(values) else None


def _daily_confirmation_groups(outcomes: list[dict[str, Any]], field: str) -> dict[str, dict[str, Any]]:
    groups: dict[str, list[dict[str, Any]]] = {}
    for outcome in outcomes:
        value = outcome.get(field)
        if value is None:
            continue
        groups.setdefault(str(value), []).append(outcome)
    return {
        key: {
            "rows": len(group),
            "next_zone_result_counts": _value_counts(group, "next_zone_result"),
            "two_bar_result_counts": _value_counts(group, "two_bar_result"),
            "average_next_close_return": _metric_average(group, "next_close_return"),
            "average_two_bar_close_return": _metric_average(group, "two_bar_close_return"),
            "positive_two_bar_return_rate": _metric_positive_rate(group, "two_bar_close_return"),
            "negative_two_bar_return_rate": _metric_negative_rate(group, "two_bar_close_return"),
            "failure_distribution": _daily_confirmation_failure_distribution(group),
            "excursion": _excursion_summary(group),
        }
        for key, group in sorted(groups.items())
    }


def _excursion_summary(outcomes: list[dict[str, Any]]) -> dict[str, Any]:
    """窗口內「過程」的摘要——與終點指標並列，用來回答「確認之後曾經逆行多少」。

    只統計 role 為 SUPPORT/RESISTANCE 的列（AT_ZONE 的方向未定義，見 _excursion_window），
    所以 `rows` 通常小於外層的 rows，這是預期而非漏算。
    """
    scoped = [o for o in outcomes if o.get("max_adverse_excursion_pct") is not None]
    ratios = _metric_values(scoped, "mae_to_stop_ratio")
    return {
        "rows": len(scoped),
        "average_max_adverse_excursion_pct": _metric_average(scoped, "max_adverse_excursion_pct"),
        "average_max_favorable_excursion_pct": _metric_average(scoped, "max_favorable_excursion_pct"),
        "max_adverse_excursion_distribution": _metric_distribution(
            _metric_values(scoped, "max_adverse_excursion_pct")
        ),
        "max_favorable_excursion_distribution": _metric_distribution(
            _metric_values(scoped, "max_favorable_excursion_pct")
        ),
        "mae_to_stop_ratio_distribution": _metric_distribution(ratios),
        # 窗口內 MAE 曾經超過停損距離的比例——這是本節唯一直接對應「停損會不會被掃到」
        # 的數字。分母只算 stop_distance_pct 可用的列。
        "stop_sweep_rate": _clean_metric(float(np.mean(np.array(ratios) > 1.0))) if ratios else None,
        "failure_state_counts": _value_counts(scoped, "failure_state"),
        "bars_to_failure_counts": _value_counts(scoped, "bars_to_failure"),
        "average_bars_to_failure": _metric_average(scoped, "bars_to_failure"),
    }


def _daily_confirmation_failure_distribution(outcomes: list[dict[str, Any]]) -> dict[str, int]:
    counts: dict[str, int] = {}
    for outcome in outcomes:
        bucket = _daily_confirmation_failure_bucket(outcome)
        counts[bucket] = counts.get(bucket, 0) + 1
    return dict(sorted(counts.items()))


def _daily_confirmation_failure_bucket(outcome: dict[str, Any]) -> str:
    role = str(outcome.get("primary_role") or "")
    next_result = str(outcome.get("next_zone_result") or "")
    two_bar_result = str(outcome.get("two_bar_result") or "")
    two_bar_return = outcome.get("two_bar_close_return")
    if role == "SUPPORT":
        if next_result == "SUPPORT_BROKEN" or two_bar_result == "SUPPORT_FAILED":
            return "SUPPORT_CONFIRMATION_FAILED"
        return "SUPPORT_CONFIRMATION_OK"
    if role == "RESISTANCE":
        if two_bar_result == "RESISTANCE_BREAKOUT_CONTINUATION":
            return "RESISTANCE_BREAKOUT_CONTINUED"
        if next_result == "RESISTANCE_REJECTED" or two_bar_result == "RESISTANCE_REJECTION_CONFIRMED":
            return "RESISTANCE_REJECTION_OK"
        if next_result == "RESISTANCE_BROKEN":
            return "RESISTANCE_REJECTION_FAILED"
        return "RESISTANCE_UNRESOLVED"
    if two_bar_return is not None:
        if float(two_bar_return) > 0:
            return "TWO_BAR_POSITIVE"
        if float(two_bar_return) < 0:
            return "TWO_BAR_NEGATIVE"
    return "UNCLASSIFIED"


def _value_counts(rows: list[dict[str, Any]], field: str) -> dict[str, int]:
    counts: dict[str, int] = {}
    for row in rows:
        value = row.get(field)
        if value is None:
            continue
        key = str(value)
        counts[key] = counts.get(key, 0) + 1
    return dict(sorted(counts.items()))


def _weighted_group_metric(groups: list[dict[str, Any]], metric: str) -> Optional[float]:
    weighted_sum = 0.0
    weight = 0
    for group in groups:
        value = group.get(metric)
        rows = int(group.get("rows_with_forward_return") or group.get("rows") or 0)
        if value is None or rows <= 0:
            continue
        weighted_sum += float(value) * rows
        weight += rows
    return _clean_metric(weighted_sum / weight) if weight > 0 else None


def _decision_replay_entry_performance(outcome_summary: dict[str, Any]) -> dict[str, Any]:
    by_state = outcome_summary.get("by_final_entry_state") or {}
    actionable_groups = [
        by_state[state]
        for state in ("PROBE_ALLOWED", "ENTRY_ALLOWED")
        if isinstance(by_state.get(state), dict)
    ]
    rows = sum(int(group.get("rows") or 0) for group in actionable_groups)
    rows_with_forward_return = sum(int(group.get("rows_with_forward_return") or 0) for group in actionable_groups)
    return {
        "states": {
            state: by_state[state]
            for state in ("PROBE_ALLOWED", "ENTRY_ALLOWED")
            if isinstance(by_state.get(state), dict)
        },
        "rows": rows,
        "rows_with_forward_return": rows_with_forward_return,
        "average_forward_return": _weighted_group_metric(actionable_groups, "average_forward_return"),
        "positive_forward_return_rate": _weighted_group_metric(actionable_groups, "positive_forward_return_rate"),
        "negative_forward_return_rate": _weighted_group_metric(actionable_groups, "negative_forward_return_rate"),
    }


def _decision_replay_governance_evaluation(
    outcome_summary: dict[str, Any],
    model_available: bool,
    replay_coverage: dict[str, Any] | None = None,
) -> dict[str, Any]:
    rows = int(outcome_summary.get("rows") or 0)
    rows_with_decision = int(outcome_summary.get("rows_with_decision_fields") or 0)
    rows_with_governance = int(outcome_summary.get("rows_with_model_governance") or 0)
    rows_with_lifecycle = int(outcome_summary.get("rows_with_event_lifecycle") or 0)
    decision_errors = sum(int(value or 0) for value in (outcome_summary.get("decision_error_counts") or {}).values())
    zone_errors = sum(int(value or 0) for value in (outcome_summary.get("zone_score_error_counts") or {}).values())
    decision_field_coverage = _clean_metric(rows_with_decision / rows) if rows else None
    model_governance_coverage = _clean_metric(rows_with_governance / rows) if rows else None
    event_lifecycle_coverage = _clean_metric(rows_with_lifecycle / rows) if rows else None
    error_rate = _clean_metric((decision_errors + zone_errors) / rows) if rows else None
    entry_performance = _decision_replay_entry_performance(outcome_summary)

    blocking: list[str] = []
    warnings: list[str] = []
    if not model_available:
        blocking.append("MODEL_UNAVAILABLE")
    if rows < MIN_GOVERNANCE_REPLAY_ROWS:
        blocking.append("REPLAY_SAMPLE_TOO_SMALL")
    if decision_field_coverage is None or decision_field_coverage < MIN_DECISION_FIELD_COVERAGE:
        blocking.append("DECISION_REPLAY_COVERAGE_LOW")
    if error_rate is not None and error_rate > MAX_DECISION_ERROR_RATE:
        blocking.append("DECISION_REPLAY_ERROR_RATE_HIGH")
    if rows_with_governance == 0:
        warnings.append("MODEL_GOVERNANCE_CONTEXT_MISSING")
    elif model_governance_coverage is not None and model_governance_coverage < MIN_DECISION_FIELD_COVERAGE:
        warnings.append("MODEL_GOVERNANCE_CONTEXT_PARTIAL")
    if event_lifecycle_coverage is not None and event_lifecycle_coverage < MIN_DECISION_FIELD_COVERAGE:
        warnings.append("EVENT_LIFECYCLE_COVERAGE_LOW")
    # 只涵蓋部分股票的 replay 代表信心度下降，但不該讓一檔 K 棒不足的個股害全體停單，
    # 所以走 warning（DEGRADED / 上限 SMALL_ENTRY）而非 blocking（UNRELIABLE / 擋單）。
    symbol_coverage_ratio = (replay_coverage or {}).get("coverage_ratio")
    if symbol_coverage_ratio is not None and symbol_coverage_ratio < MIN_REPLAY_SYMBOL_COVERAGE:
        warnings.append("REPLAY_SYMBOL_COVERAGE_PARTIAL")

    actionable_rows = int(entry_performance["rows_with_forward_return"])
    average_forward_return = entry_performance["average_forward_return"]
    positive_rate = entry_performance["positive_forward_return_rate"]
    if actionable_rows < MIN_ENTRY_OUTCOME_ROWS:
        warnings.append("ENTRY_OUTCOME_SAMPLE_TOO_SMALL")
    else:
        if average_forward_return is not None and average_forward_return < MIN_ENTRY_AVERAGE_FORWARD_RETURN:
            blocking.append("ENTRY_OUTCOME_NEGATIVE")
        if positive_rate is not None and positive_rate < MIN_ENTRY_POSITIVE_RETURN_RATE:
            blocking.append("ENTRY_OUTCOME_LOW_POSITIVE_RATE")

    if blocking:
        health_state = "UNRELIABLE"
        allow_entry = False
        max_entry_state = "WAIT_CONFIRMATION"
    elif warnings:
        health_state = "DEGRADED"
        allow_entry = True
        max_entry_state = "SMALL_ENTRY"
    else:
        health_state = "HEALTHY"
        allow_entry = True
        max_entry_state = "ENTRY_ALLOWED"

    reason_codes = sorted(set(blocking or warnings))
    return {
        "schema_version": "sr_decision_replay_governance_evaluation_v1",
        "scope": "DECISION_REPLAY",
        "health_state": health_state,
        "passed": health_state != "UNRELIABLE",
        "strict_passed": health_state == "HEALTHY",
        "blocking_flags": sorted(set(blocking)),
        "warning_flags": sorted(set(warnings)),
        "confidence_gate": {
            "state": health_state,
            "allow_entry": allow_entry,
            "max_entry_state": max_entry_state,
            "reason_codes": reason_codes,
        },
        "coverage": {
            "rows": rows,
            "rows_with_decision_fields": rows_with_decision,
            "decision_field_coverage": decision_field_coverage,
            "rows_with_model_governance": rows_with_governance,
            "model_governance_coverage": model_governance_coverage,
            "rows_with_event_lifecycle": rows_with_lifecycle,
            "event_lifecycle_coverage": event_lifecycle_coverage,
            "decision_error_count": decision_errors,
            "zone_score_error_count": zone_errors,
            "error_rate": error_rate,
        },
        "entry_performance": entry_performance,
        "thresholds": {
            "min_replay_rows": MIN_GOVERNANCE_REPLAY_ROWS,
            "min_entry_outcome_rows": MIN_ENTRY_OUTCOME_ROWS,
            "min_decision_field_coverage": MIN_DECISION_FIELD_COVERAGE,
            "max_decision_error_rate": MAX_DECISION_ERROR_RATE,
            "min_entry_positive_return_rate": MIN_ENTRY_POSITIVE_RETURN_RATE,
            "min_entry_average_forward_return": MIN_ENTRY_AVERAGE_FORWARD_RETURN,
        },
    }


def _decision_outcome_groups(rows: list[dict[str, Any]], field: str) -> dict[str, dict[str, Any]]:
    groups: dict[str, list[dict[str, Any]]] = {}
    for row in rows:
        value = row.get(field)
        if value is None:
            continue
        groups.setdefault(str(value), []).append(row)
    return {
        key: _decision_outcome_group(group)
        for key, group in sorted(groups.items())
    }


def _decision_outcome_group(rows: list[dict[str, Any]]) -> dict[str, Any]:
    returns = [
        float(row["forward_return"])
        for row in rows
        if row.get("forward_return") is not None
    ]
    if not returns:
        return {
            "rows": len(rows),
            "rows_with_forward_return": 0,
            "average_forward_return": None,
            "positive_forward_return_rate": None,
            "negative_forward_return_rate": None,
        }
    values = np.array(returns, dtype=float)
    return {
        "rows": len(rows),
        "rows_with_forward_return": len(returns),
        "average_forward_return": _clean_metric(float(np.mean(values))),
        "positive_forward_return_rate": _clean_metric(float(np.mean(values > 0))),
        "negative_forward_return_rate": _clean_metric(float(np.mean(values < 0))),
    }


def _metric_distribution(values: list[float]) -> dict[str, Any]:
    """數值序列的分布摘要。

    **為什麼需要這個**：平均數單獨看會誤導。2026-08-07 的真實 report 裡
    `average_entry_rr = 6.45` 但 `median_entry_rr = 2.34`（最大值 1032），
    平均是中位數的 2.75 倍——只報平均會系統性高估這套規則的風險報酬。
    percentiles 才看得出分布的形狀與尾巴。

    空序列回傳 count=0 且其餘為 None（不是 0）：沒有樣本與「樣本值為 0」是兩件事。
    """
    if not values:
        return {
            "count": 0,
            "average": None,
            "stddev": None,
            "min": None,
            "p10": None,
            "p25": None,
            "median": None,
            "p75": None,
            "p90": None,
            "max": None,
        }
    array = np.asarray(values, dtype=float)
    return {
        "count": int(array.size),
        "average": _clean_metric(float(np.mean(array))),
        # 單一元素時 np.std 回 0.0（ddof=0），語意正確：只有一個點就沒有離散度。
        "stddev": _clean_metric(float(np.std(array))),
        "min": _clean_metric(float(np.min(array))),
        "p10": _clean_metric(float(np.percentile(array, 10))),
        "p25": _clean_metric(float(np.percentile(array, 25))),
        "median": _clean_metric(float(np.median(array))),
        "p75": _clean_metric(float(np.percentile(array, 75))),
        "p90": _clean_metric(float(np.percentile(array, 90))),
        "max": _clean_metric(float(np.max(array))),
    }


def _rr_values(rows: list[dict[str, Any]], field: str) -> list[float]:
    return [
        float((row.get("rr_context") or {}).get(field))
        for row in rows
        if (row.get("rr_context") or {}).get(field) is not None
    ]


def _rr_summary(rows: list[dict[str, Any]]) -> dict[str, Any]:
    entry_rr_values = _rr_values(rows, "entry_rr")
    position_rr_values = _rr_values(rows, "position_rr")
    # execution_rr 一直存在於 rr_context、也參與 rr_gate 的判斷
    # （by_rr_gate_reason_code 有 EXECUTION_RR_INSUFFICIENT / EXECUTION_RR_UNAVAILABLE），
    # 但先前完全沒有被統計。2026-08-07 的真實 report 有 931 筆有值。
    execution_rr_values = _rr_values(rows, "execution_rr")

    entry_rr_sources: dict[str, int] = {}
    position_rr_sources: dict[str, int] = {}
    execution_rr_sources: dict[str, int] = {}
    for row in rows:
        rr_context = row.get("rr_context") or {}
        for field, bucket in (
            ("entry_rr_source", entry_rr_sources),
            ("position_rr_source", position_rr_sources),
            ("execution_rr_source", execution_rr_sources),
        ):
            if rr_context.get(field):
                source = str(rr_context[field])
                bucket[source] = bucket.get(source, 0) + 1

    return {
        # ── 既有欄位，形狀不動（前端已在消費）──────────────────
        "rows_with_entry_rr": len(entry_rr_values),
        "average_entry_rr": _clean_metric(float(np.mean(entry_rr_values))) if entry_rr_values else None,
        "median_entry_rr": _clean_metric(float(np.median(entry_rr_values))) if entry_rr_values else None,
        "rows_with_position_rr": len(position_rr_values),
        "average_position_rr": _clean_metric(float(np.mean(position_rr_values))) if position_rr_values else None,
        "median_position_rr": _clean_metric(float(np.median(position_rr_values))) if position_rr_values else None,
        "entry_rr_source_counts": dict(sorted(entry_rr_sources.items())),
        "position_rr_source_counts": dict(sorted(position_rr_sources.items())),
        # ── 2026-08-07 新增：execution RR 與三者的完整分布 ──────
        "rows_with_execution_rr": len(execution_rr_values),
        "average_execution_rr": _clean_metric(float(np.mean(execution_rr_values))) if execution_rr_values else None,
        "median_execution_rr": _clean_metric(float(np.median(execution_rr_values))) if execution_rr_values else None,
        "execution_rr_source_counts": dict(sorted(execution_rr_sources.items())),
        "entry_rr_distribution": _metric_distribution(entry_rr_values),
        "execution_rr_distribution": _metric_distribution(execution_rr_values),
        "position_rr_distribution": _metric_distribution(position_rr_values),
    }


def run_builder_sweep(
    symbols: Optional[list[str]] = None,
    csv_sources: Optional[list[str]] = None,
    timeframe: str = "1d",
    limit: int = DEFAULT_EVALUATION_LIMIT,
    model_path: Optional[str] = None,
    dataset_config: DatasetConfig | None = None,
    atr_width_multipliers: Optional[list[float]] = None,
    max_merge_width_multiples: Optional[list[float]] = None,
    atr_lookback: int = 60,
    atr_period: int = 14,
    decision_replay: bool = False,
    replay_max_rows: int = SWEEP_DEFAULT_REPLAY_MAX_ROWS,
    run_id: Optional[str] = None,
    pipeline_version: str = "sr_zone_builder_sweep_p1",
) -> dict[str, Any]:
    """Evaluate a conservative ATR builder parameter grid.

    Sweep reports intentionally stay out of stock_sr_regression_results for now:
    each row is a candidate config, not one promoted model/pipeline regression run.
    """
    base_run_id = run_id or new_evaluation_run_id()
    widths = atr_width_multipliers or list(DEFAULT_SWEEP_WIDTHS)
    max_merges = max_merge_width_multiples or list(DEFAULT_SWEEP_MAX_MERGES)
    if any(item <= 0 for item in [*widths, *max_merges]):
        raise ValueError("builder sweep 參數必須全部大於 0")

    results: list[dict[str, Any]] = []
    warnings: list[str] = []

    # decision replay 需要模型才產得出 decision 欄位；沒有 model_path 時只記 warning 並
    # 略過 replay，不要讓整個 sweep 失敗（zone 層的比較仍然有價值）。
    replay_enabled = decision_replay
    if decision_replay and not model_path:
        replay_enabled = False
        warnings.append("decision replay skipped: --model-path is required for decision fields")

    # chip / governance context 只載入一次再傳給每個 candidate，否則 N 組候選會各查一次 DB。
    # 載入時機刻意放在第一個 candidate 的 run_evaluation 之後：所有候選吃的是同一份來源資料，
    # dataset range 相同，直接沿用該 report 的 dataset_from/to 就好——先前的寫法為了算這個
    # 範圍額外做了一次 _load_db_sources()，等於把全部 K 棒多讀一遍再丟掉。
    replay_chip_context: dict[str, list[dict]] | None = None
    replay_governance_context: dict[str, list[dict]] | None = None
    replay_context_loaded = False

    for width in widths:
        for max_merge in max_merges:
            candidate_run_id = f"{base_run_id}_w{_sweep_token(width)}_m{_sweep_token(max_merge)}"
            builder_config = ZoneBuilderConfig(
                atr=ATRZoneBuilderConfig(
                    lookback=atr_lookback,
                    atr_period=atr_period,
                    atr_width_multiplier=width,
                    max_merge_width_multiple=max_merge,
                )
            )
            report = run_evaluation(
                symbols=symbols,
                csv_sources=csv_sources,
                timeframe=timeframe,
                limit=limit,
                model_path=model_path,
                dataset_config=dataset_config,
                builder_config=builder_config,
                run_id=candidate_run_id,
                pipeline_version=pipeline_version,
            )
            warnings.extend(report.get("warnings") or [])
            if replay_enabled and symbols and not replay_context_loaded:
                context_warnings: list[str] = []
                replay_chip_context = _load_db_replay_chip_context(
                    symbols, report.get("dataset_from"), report.get("dataset_to"), context_warnings
                )
                replay_governance_context = _load_db_replay_model_governance_context(
                    symbols, timeframe, report.get("dataset_from"), report.get("dataset_to"), context_warnings
                )
                warnings.extend(context_warnings)
                replay_context_loaded = True
            replay_report = None
            if replay_enabled:
                replay_report = run_decision_replay(
                    symbols=symbols,
                    csv_sources=csv_sources,
                    timeframe=timeframe,
                    limit=limit,
                    model_path=model_path,
                    dataset_config=dataset_config,
                    replay_max_rows=replay_max_rows,
                    builder_config=builder_config,
                    chip_scores_by_symbol=replay_chip_context,
                    model_governance_by_symbol=replay_governance_context,
                    run_id=f"{candidate_run_id}_replay",
                )
                warnings.extend(replay_report.get("warnings") or [])
            results.append(_sweep_result_summary(report, width, max_merge, replay_report))

    return {
        "schema_version": "sr_zone_builder_sweep_p1",
        "run_id": base_run_id,
        "pipeline_version": pipeline_version,
        "timeframe": timeframe,
        "parameter_grid": {
            "atr_width_multipliers": widths,
            "max_merge_width_multiples": max_merges,
            "atr_lookback": atr_lookback,
            "atr_period": atr_period,
        },
        "candidate_count": len(results),
        "results": results,
        "decision_replay_enabled": replay_enabled,
        "best_by": {
            "support_hold_rate": _best_sweep_result(results, "support_hold_rate"),
            "resistance_rejection_rate": _best_sweep_result(results, "resistance_rejection_rate"),
            "break_positive_rate": _best_sweep_result(results, "break_positive_rate"),
            "average_forward_return": _best_sweep_result(results, "average_forward_return"),
            "entry_average_forward_return": _best_sweep_entry_result(results),
        },
        "recommended_configs_by_bucket": _bucket_recommendations(results),
        "warnings": sorted(set(warnings)),
    }


def run_decision_replay(
    symbols: Optional[list[str]] = None,
    csv_sources: Optional[list[str]] = None,
    timeframe: str = "1d",
    limit: int = DEFAULT_EVALUATION_LIMIT,
    model_path: Optional[str] = None,
    dataset_config: DatasetConfig | None = None,
    replay_max_rows: int = DEFAULT_REPLAY_MAX_ROWS,
    builder_config: ZoneBuilderConfig | None = None,
    chip_scores_by_symbol: dict[str, list[dict]] | None = None,
    model_governance_by_symbol: dict[str, list[dict]] | None = None,
    run_id: Optional[str] = None,
    # pipeline_version 升到 p1 標記「跨股票均分預算 + 每檔取最新窗口」的取樣方式變更，
    # 讓新舊 report 可區分。schema_version 刻意維持 p0——
    # fetch_latest_sr_regression_governance 是用 schema_version 過濾的，改了會讓 production
    # gate 查不到資料而靜默失效（見 docs/issue.md I-040）。
    pipeline_version: str = "sr_zone_decision_replay_p1",
) -> dict[str, Any]:
    """Prepare the historical decision replay report envelope.

    P0 intentionally does not synthesize decision states from zone touch labels.
    A real replay must rebuild ZoneScore and call build_decision_summary() for
    each historical as-of candle with model governance and event lifecycle
    context available.
    """
    sources: list[tuple[str, str, pd.DataFrame]] = []
    if symbols:
        sources.extend(_load_db_sources(symbols, timeframe, limit))
    if csv_sources:
        sources.extend(_load_csv_sources(csv_sources, timeframe))
    if not sources:
        raise ValueError("沒有任何可用的資料來源（請指定 symbols 或 csv）")
    if replay_max_rows <= 0:
        raise ValueError("replay_max_rows 必須大於 0")

    warnings: list[str] = []
    bundle: ModelBundle | None = None
    if model_path:
        try:
            bundle = load_model(model_path)
        except Exception as exc:  # noqa: BLE001 - replay report should describe unavailable model.
            warnings.append(f"model unavailable: {exc}")
    else:
        warnings.append("model unavailable: decision replay requires --model-path")

    dataset_config = dataset_config or DatasetConfig()
    dataset_from, dataset_to = _dataset_range(sources)
    if symbols and chip_scores_by_symbol is None:
        chip_scores_by_symbol = _load_db_replay_chip_context(symbols, dataset_from, dataset_to, warnings)
    if symbols and model_governance_by_symbol is None:
        model_governance_by_symbol = _load_db_replay_model_governance_context(
            symbols,
            timeframe,
            dataset_from,
            dataset_to,
            warnings,
        )
    volatility_profiles = _volatility_profiles(sources, pd.DataFrame())
    replay_plan = _decision_replay_plan(sources, dataset_config)
    model_metadata = _model_metadata(bundle)
    quota_by_symbol, symbols_skipped = _allocate_replay_quota(sources, dataset_config, replay_max_rows)
    replay_rows = _decision_replay_rows(
        sources,
        dataset_config,
        quota_by_symbol,
        bundle=bundle,
        chip_scores_by_symbol=chip_scores_by_symbol,
        model_governance_by_symbol=model_governance_by_symbol,
        builder_config=builder_config,
    )
    replay_coverage = _replay_coverage(sources, quota_by_symbol, symbols_skipped)
    if symbols_skipped:
        warnings.append(
            "replay coverage partial: "
            f"{replay_coverage['symbols_covered']}/{replay_coverage['symbols_requested']} symbols "
            f"（預算不足或 K 棒不足而略過：{', '.join(symbols_skipped)}）"
        )
    outcome_summary = _decision_replay_outcome_summary(replay_rows)
    governance_evaluation = _decision_replay_governance_evaluation(
        outcome_summary,
        model_available=bool(model_metadata["available"]),
        replay_coverage=replay_coverage,
    )

    return {
        "schema_version": "sr_zone_decision_replay_p0",
        "run_id": run_id or new_evaluation_run_id(),
        "pipeline_version": pipeline_version,
        "timeframe": timeframe,
        "sources": len(sources),
        "symbols": sorted({symbol for symbol, _, _ in sources}),
        "dataset_from": dataset_from,
        "dataset_to": dataset_to,
        "dataset_config": asdict(dataset_config),
        "volatility_profiles": volatility_profiles,
        "model_metadata": model_metadata,
        "model_version": model_metadata["version"],
        "model_config_hash": model_metadata["config_hash"],
        "model_trained_at": model_metadata["trained_at"],
        "replay_plan": replay_plan,
        "replay_coverage": replay_coverage,
        # 讓 replay 結果可追溯用了哪組 zone builder 參數；未指定時就是 baseline 預設。
        "builder_config": zone_builder_config_snapshot(builder_config),
        "decision_replay_available": any(row.get("decision_fields_available") for row in replay_rows),
        "event_lifecycle_replay_available": any(row.get("event_lifecycle_replay_available") for row in replay_rows),
        "zone_score_fields_available": any(row.get("zone_score_available") for row in replay_rows),
        "model_available": model_metadata["available"],
        "decision_fields_available": any(row.get("decision_fields_available") for row in replay_rows),
        "outcome_rows_available": bool(replay_rows),
        "replay_max_rows": replay_max_rows,
        "rows": len(replay_rows),
        "replay_rows": replay_rows,
        "outcome_summary": outcome_summary,
        "governance_evaluation": governance_evaluation,
        "planned_fields": [
            "symbol",
            "timeframe",
            "as_of",
            "current_price",
            "candle_open",
            "candle_high",
            "candle_low",
            "candle_close",
            "previous_candle_close",
            "next_close_return",
            "two_bar_close_return",
            "zone_score_available",
            "zone_count",
            "primary_zone",
            "global_trend",
            "global_volatility",
            "global_metrics",
            "chip_summary",
            "model_governance_available",
            "model_governance_source_time",
            "model_governance",
            "market_bias",
            "daily_confirmation_state",
            "daily_confirmation_outcome",
            "daily_confirmation_context",
            "final_entry_state",
            "rr_context",
            "rr_gate",
            "event_lifecycle_replay_available",
            "event_state_count",
            "active_event_count",
            "resolved_event_count",
            "expired_event_count",
            "forward_return",
        ],
        "pending_requirements": [
            "production governance gate promotion",
            "normalized regression metric columns",
        ],
        "warnings": warnings,
    }


def _date_range_for_context(dataset_from: str | None, dataset_to: str | None) -> tuple[str, str] | None:
    if not dataset_from or not dataset_to:
        return None
    start = pd.Timestamp(dataset_from)
    end = pd.Timestamp(dataset_to)
    if start.tzinfo is None:
        start = start.tz_localize("UTC")
    if end.tzinfo is None:
        end = end.tz_localize("UTC")
    return start.tz_convert("Asia/Taipei").date().isoformat(), end.tz_convert("Asia/Taipei").date().isoformat()


def _load_db_replay_chip_context(
    symbols: list[str],
    dataset_from: str | None,
    dataset_to: str | None,
    warnings: list[str],
) -> dict[str, list[dict]]:
    date_range = _date_range_for_context(dataset_from, dataset_to)
    if date_range is None:
        warnings.append("chip context unavailable: dataset range missing")
        return {}
    from_date, to_date = date_range
    try:
        from db import fetch_chip_scores
    except Exception as exc:  # noqa: BLE001 - CLI can still run with missing chip context.
        warnings.append(f"chip context unavailable: {exc}")
        return {}

    out: dict[str, list[dict]] = {}
    for symbol in symbols:
        symbol = symbol.strip()
        if not symbol:
            continue
        try:
            rows = fetch_chip_scores(symbol, from_date, to_date)
        except Exception as exc:  # noqa: BLE001 - one symbol should not abort replay.
            warnings.append(f"chip context unavailable for {symbol}: {exc}")
            continue
        if rows:
            out[symbol] = rows
    return out


def _load_db_replay_model_governance_context(
    symbols: list[str],
    timeframe: str,
    dataset_from: str | None,
    dataset_to: str | None,
    warnings: list[str],
) -> dict[str, list[dict]]:
    if not dataset_from or not dataset_to:
        warnings.append("model governance context unavailable: dataset range missing")
        return {}
    try:
        from db import fetch_sr_model_governance
    except Exception as exc:  # noqa: BLE001 - CLI can still run with missing governance context.
        warnings.append(f"model governance context unavailable: {exc}")
        return {}

    out: dict[str, list[dict]] = {}
    for symbol in symbols:
        symbol = symbol.strip()
        if not symbol:
            continue
        try:
            rows = fetch_sr_model_governance(symbol, timeframe, dataset_from, dataset_to)
        except Exception as exc:  # noqa: BLE001 - one symbol should not abort replay.
            warnings.append(f"model governance context unavailable for {symbol}: {exc}")
            continue
        if rows:
            out[symbol] = rows
    return out


def main() -> None:
    parser = argparse.ArgumentParser(description="Evaluate SR Zone walk-forward outcomes")
    parser.add_argument("--symbols", help="逗號分隔的股票代碼，從 DB 讀取")
    parser.add_argument("--csv", action="append", default=[], help="CSV 路徑，格式 path[:symbol]，可重複給多筆")
    parser.add_argument("--timeframe", default="1d")
    parser.add_argument("--limit", type=int, default=DEFAULT_EVALUATION_LIMIT)
    parser.add_argument("--model-path", default=None)
    parser.add_argument("--output", default=None, help="輸出 JSON 檔；未指定時印到 stdout")
    parser.add_argument("--write-db", action="store_true", help="寫入 stock_sr_regression_results")
    parser.add_argument("--run-id", default=None)
    parser.add_argument("--pipeline-version", default=DEFAULT_PIPELINE_VERSION)
    parser.add_argument("--passed", choices=["true", "false", "null"], default="null")
    parser.add_argument("--sweep", action="store_true", help="執行 ATR builder 參數 grid sweep，只輸出 JSON")
    parser.add_argument(
        "--sweep-decision-replay",
        action="store_true",
        help="sweep 時每組候選另跑一次 decision replay，產出 entry outcome / RR 比較（需 --model-path，耗時明顯增加）",
    )
    parser.add_argument("--decision-replay", action="store_true", help="輸出 historical decision replay P0 skeleton")
    parser.add_argument("--replay-max-rows", type=int, default=DEFAULT_REPLAY_MAX_ROWS, help="decision replay outcome rows 輸出上限")
    parser.add_argument("--chip-json", default=None, help="decision replay historical chip rows JSON")
    parser.add_argument("--model-governance-json", default=None, help="decision replay model governance snapshots JSON")
    parser.add_argument("--atr-width-grid", default=None, help="逗號分隔的 atr_width_multiplier 候選值")
    parser.add_argument("--max-merge-width-grid", default=None, help="逗號分隔的 max_merge_width_multiple 候選值")
    parser.add_argument("--min-history-bars", type=int, default=80)
    parser.add_argument("--rebuild-every-bars", type=int, default=5)
    parser.add_argument("--forward-bars", type=int, default=5)
    parser.add_argument("--threshold-pct", type=float, default=0.03)
    parser.add_argument("--atr-width-multiplier", type=float, default=1.5)
    parser.add_argument("--max-merge-width-multiple", type=float, default=2.0)
    parser.add_argument("--atr-lookback", type=int, default=60)
    parser.add_argument("--atr-period", type=int, default=14)
    args = parser.parse_args()

    if args.sweep and args.decision_replay:
        print("[error] --sweep 與 --decision-replay 不可同時使用", file=sys.stderr)
        sys.exit(1)

    if args.sweep and args.write_db:
        print("[error] --sweep 目前只輸出 JSON，不支援 --write-db", file=sys.stderr)
        sys.exit(1)

    if args.symbols or args.write_db:
        from db import check_connection
        check_connection()

    config = DatasetConfig(
        min_history_bars=args.min_history_bars,
        rebuild_every_bars=args.rebuild_every_bars,
        forward_bars_support=args.forward_bars,
        forward_bars_resistance=args.forward_bars,
        threshold_pct_support=args.threshold_pct,
        threshold_pct_resistance=args.threshold_pct,
    )
    # ATR builder 參數對 evaluation 與 decision replay 都要生效；先前 replay 沒吃這組參數，
    # CLI 給了也靜默無效。
    cli_builder_config = ZoneBuilderConfig(
        atr=ATRZoneBuilderConfig(
            lookback=args.atr_lookback,
            atr_period=args.atr_period,
            atr_width_multiplier=args.atr_width_multiplier,
            max_merge_width_multiple=args.max_merge_width_multiple,
        )
    )
    try:
        if args.decision_replay:
            chip_scores_by_symbol = _load_symbol_rows_json(args.chip_json) if args.chip_json else None
            model_governance_by_symbol = (
                _load_symbol_rows_json(args.model_governance_json) if args.model_governance_json else None
            )
            report = run_decision_replay(
                symbols=args.symbols.split(",") if args.symbols else None,
                csv_sources=args.csv or None,
                timeframe=args.timeframe,
                limit=args.limit,
                model_path=args.model_path,
                dataset_config=config,
                replay_max_rows=args.replay_max_rows,
                builder_config=cli_builder_config,
                chip_scores_by_symbol=chip_scores_by_symbol,
                model_governance_by_symbol=model_governance_by_symbol,
                run_id=args.run_id,
                pipeline_version=args.pipeline_version,
            )
        elif args.sweep:
            report = run_builder_sweep(
                symbols=args.symbols.split(",") if args.symbols else None,
                csv_sources=args.csv or None,
                timeframe=args.timeframe,
                limit=args.limit,
                model_path=args.model_path,
                dataset_config=config,
                atr_width_multipliers=_float_grid(args.atr_width_grid, DEFAULT_SWEEP_WIDTHS),
                max_merge_width_multiples=_float_grid(args.max_merge_width_grid, DEFAULT_SWEEP_MAX_MERGES),
                atr_lookback=args.atr_lookback,
                atr_period=args.atr_period,
                decision_replay=args.sweep_decision_replay,
                replay_max_rows=args.replay_max_rows if args.sweep_decision_replay else SWEEP_DEFAULT_REPLAY_MAX_ROWS,
                run_id=args.run_id,
                pipeline_version=args.pipeline_version,
            )
        else:
            report = run_evaluation(
                symbols=args.symbols.split(",") if args.symbols else None,
                csv_sources=args.csv or None,
                timeframe=args.timeframe,
                limit=args.limit,
                model_path=args.model_path,
                dataset_config=config,
                builder_config=cli_builder_config,
                run_id=args.run_id,
                pipeline_version=args.pipeline_version,
            )
    except ValueError as exc:
        print(f"[error] {exc}", file=sys.stderr)
        sys.exit(1)

    if args.write_db:
        passed = None if args.passed == "null" else args.passed == "true"
        write_evaluation_result(report, passed=passed)

    encoded = json.dumps(report, indent=2, ensure_ascii=False)
    if args.output:
        with open(args.output, "w", encoding="utf-8") as fh:
            fh.write(encoded)
            fh.write("\n")
    else:
        print(encoded)


if __name__ == "__main__":
    main()
