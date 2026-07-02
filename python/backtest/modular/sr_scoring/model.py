"""
訓練/預測/持久化 bounce_probability（hold_model）與 break_probability
（break_model）。兩個模型用相同的 pooled 特徵集（支撐與壓力觸碰事件一起
訓練，用 is_support one-hot 特徵區分角色），比分別訓練兩組小樣本模型有更
好的泛化性。

模型檔不存在時 get_model() 直接拋錯（fail-fast），比照
internal/analysis/client.go 對「python service url 未設定」的處理風格，
不靜默回傳中性機率——代表 /sr-zones 在第一次執行 train.py 之前是不可用
的，這是部署時的必要步驟，而非執行期異常。
"""
from __future__ import annotations

import os
from dataclasses import dataclass, field
from datetime import datetime, timezone
from typing import Any, Optional

import joblib
import numpy as np
import pandas as pd
from sklearn.ensemble import GradientBoostingClassifier
from sklearn.linear_model import LogisticRegression
from sklearn.metrics import accuracy_score, precision_score, recall_score, roc_auc_score
from sklearn.model_selection import train_test_split
from sklearn.pipeline import Pipeline
from sklearn.preprocessing import StandardScaler

from .types import ZoneFeatures

FEATURE_COLUMNS = [
    "touch_count",
    "rejection_count",
    "breakout_count",
    "avg_return_after_touch",
    "relative_volume",
    "volatility",
    "trend_strength",
    "is_support",
]

MODEL_VERSION = "v1"


@dataclass
class ModelBundle:
    hold_model: Any
    break_model: Any
    feature_names: list[str]
    trained_at: str
    version: str
    metrics: dict[str, dict[str, float]] = field(default_factory=dict)


def _build_estimator(model_type: str, random_state: int):
    if model_type == "logistic_regression":
        return Pipeline([
            ("scaler", StandardScaler()),
            ("clf", LogisticRegression(max_iter=1000, random_state=random_state)),
        ])
    if model_type == "gradient_boosting":
        return GradientBoostingClassifier(random_state=random_state)
    raise ValueError(f"unknown model_type: {model_type}")


def _fit_one(
    dataset: pd.DataFrame, label_col: str, model_type: str, test_size: float, random_state: int
) -> tuple[Any, dict[str, float]]:
    X = dataset[FEATURE_COLUMNS].to_numpy(dtype=float)
    y = dataset[label_col].to_numpy(dtype=int)

    stratify = y if len(np.unique(y)) > 1 else None
    X_train, X_test, y_train, y_test = train_test_split(
        X, y, test_size=test_size, random_state=random_state, stratify=stratify
    )

    model = _build_estimator(model_type, random_state)
    model.fit(X_train, y_train)

    y_pred = model.predict(X_test)
    metrics: dict[str, float] = {
        "accuracy": float(accuracy_score(y_test, y_pred)),
        "precision": float(precision_score(y_test, y_pred, zero_division=0)),
        "recall": float(recall_score(y_test, y_pred, zero_division=0)),
        "train_rows": float(len(y_train)),
        "test_rows": float(len(y_test)),
        "positive_rate": float(y.mean()),
    }
    if len(np.unique(y_test)) > 1:
        y_proba = model.predict_proba(X_test)[:, 1]
        metrics["auc"] = float(roc_auc_score(y_test, y_proba))
    else:
        metrics["auc"] = float("nan")

    return model, metrics


def train_model(
    dataset: pd.DataFrame,
    model_type: str = "gradient_boosting",
    test_size: float = 0.2,
    random_state: int = 42,
) -> ModelBundle:
    if len(dataset) < 20:
        raise ValueError(f"訓練資料太少（{len(dataset)} 筆），至少需要 20 筆 touch 事件")

    hold_model, hold_metrics = _fit_one(dataset, "hold_label", model_type, test_size, random_state)
    break_model, break_metrics = _fit_one(dataset, "break_label", model_type, test_size, random_state)

    return ModelBundle(
        hold_model=hold_model,
        break_model=break_model,
        feature_names=list(FEATURE_COLUMNS),
        trained_at=datetime.now(timezone.utc).isoformat(),
        version=MODEL_VERSION,
        metrics={"hold": hold_metrics, "break": break_metrics},
    )


def save_model(bundle: ModelBundle, path: str) -> None:
    parent = os.path.dirname(path)
    if parent:
        os.makedirs(parent, exist_ok=True)
    joblib.dump(bundle, path)


def load_model(path: str) -> ModelBundle:
    return joblib.load(path)


_MODEL_CACHE: Optional[ModelBundle] = None
_MODEL_CACHE_PATH: Optional[str] = None


def get_model(path: str | None = None) -> ModelBundle:
    """Lazy singleton；path 為 None 時取自 config.SR_SCORING_MODEL_PATH。"""
    global _MODEL_CACHE, _MODEL_CACHE_PATH

    resolved_path = path
    if resolved_path is None:
        import config

        resolved_path = config.SR_SCORING_MODEL_PATH

    if _MODEL_CACHE is not None and _MODEL_CACHE_PATH == resolved_path:
        return _MODEL_CACHE

    try:
        bundle = load_model(resolved_path)
    except FileNotFoundError as exc:
        raise RuntimeError(
            f"sr_scoring 模型檔不存在：{resolved_path}（請先執行 "
            "`python -m backtest.modular.sr_scoring.train` 產生模型）"
        ) from exc

    _MODEL_CACHE = bundle
    _MODEL_CACHE_PATH = resolved_path
    return bundle


def _feature_vector(features: ZoneFeatures, is_support: bool) -> np.ndarray:
    return np.array(
        [[
            features.touch_count,
            features.rejection_count,
            features.breakout_count,
            features.avg_return_after_touch,
            features.relative_volume,
            features.volatility,
            features.trend_strength,
            1.0 if is_support else 0.0,
        ]],
        dtype=float,
    )


def predict_hold_probability(bundle: ModelBundle, features: ZoneFeatures, is_support: bool) -> float:
    X = _feature_vector(features, is_support)
    return float(bundle.hold_model.predict_proba(X)[0, 1])


def predict_break_probability(bundle: ModelBundle, features: ZoneFeatures, is_support: bool) -> float:
    X = _feature_vector(features, is_support)
    return float(bundle.break_model.predict_proba(X)[0, 1])
