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

import bisect
import os
from dataclasses import dataclass, field
from datetime import datetime, timezone
from typing import Any, Optional

import joblib
import numpy as np
import pandas as pd
from sklearn.calibration import CalibratedClassifierCV
from sklearn.ensemble import GradientBoostingClassifier
from sklearn.linear_model import LogisticRegression
from sklearn.metrics import (
    accuracy_score,
    brier_score_loss,
    log_loss,
    precision_score,
    recall_score,
    roc_auc_score,
)
from sklearn.model_selection import train_test_split
from sklearn.pipeline import Pipeline
from sklearn.preprocessing import StandardScaler

from .types import ZoneFeatures

FEATURE_COLUMNS = [
    "touch_count",
    "rejection_count",
    "breakout_count",
    "average_bounce_return",
    "average_break_return",
    "relative_volume",
    "volatility",
    "trend_strength",
    "is_support",
]

MODEL_VERSION = "v2"  # v1 的 feature schema 用 avg_return_after_touch，跟 v2 不相容，需重新訓練

# 機率校準（CalibratedClassifierCV）需要足夠樣本才能穩定：訓練集太小、或
# 任一類別樣本太少時，CalibratedClassifierCV 內部的 CV 切分會失敗或退化成
# 沒有意義的估計，這種情況下降級為不校準（比校準壞了更安全），並在
# metrics 用 calibrated=0.0 明確標記，而不是靜默用一個不可靠的校準結果。
MIN_ROWS_FOR_CALIBRATION = 40
MIN_CLASS_COUNT_FOR_CALIBRATION = 10


@dataclass
class ModelBundle:
    hold_model: Any
    break_model: Any
    feature_names: list[str]
    trained_at: str
    version: str
    metrics: dict[str, dict[str, float]] = field(default_factory=dict)
    # 訓練資料集裡每一列 risk_reward_ratio 的排序分佈，供 reward_risk_percentile()
    # 在推論時查表用（見 scoring.py 的 reward_risk_percentile 欄位說明）。
    rr_reference: list[float] = field(default_factory=list)
    # "time"（預設，依 touch_time 切，每檔股票各自切最後一段當 test，避免
    # 隨機切分讓未來資料混進訓練集高估表現）或 "random"（舊行為，保留供比較）。
    # 純量預設值會存成 class 屬性，用 joblib 讀取這個欄位加進來之前存的舊
    # 模型檔時仍能正常取得預設值，不需要為此再拉一個 MODEL_VERSION。
    split_method: str = "time"


def _build_estimator(model_type: str, random_state: int):
    if model_type == "logistic_regression":
        return Pipeline([
            ("scaler", StandardScaler()),
            ("clf", LogisticRegression(max_iter=1000, random_state=random_state)),
        ])
    if model_type == "gradient_boosting":
        return GradientBoostingClassifier(random_state=random_state)
    raise ValueError(f"unknown model_type: {model_type}")


def _time_split_indices(dataset: pd.DataFrame, test_size: float) -> tuple[pd.Index, pd.Index]:
    """依 touch_time 切分：每檔股票各自排序後取最後 test_size 比例當 test
    set，其餘當 train set，再合併所有股票的結果。

    不用「對整個 pooled 資料集做一次全域時間排序、取最後一段」的做法——
    訓練資料是跨多檔股票 pooled 的，若各股票歷史資料的時間範圍差很多（例如
    有的股票剛上市、資料比較新），全域時間切分可能讓 test set 集中在少數
    幾檔股票，不是每檔都均勻取樣，測出來的 metrics 沒有代表性。逐股票各自
    切分才能確保每檔股票的 train/test 都有取樣，同時仍然保證每檔股票內部
    「test 一定比 train 時間晚」（避免用未來資料驗證過去的模型，高估表現）。
    """
    train_idx: list = []
    test_idx: list = []
    for _, group in dataset.groupby("symbol", sort=False):
        ordered = group.sort_values("touch_time")
        n = len(ordered)
        n_test = int(round(n * test_size))
        if n > 1:
            n_test = min(max(n_test, 0), n - 1)  # 每檔股票至少留 1 筆給 train
        else:
            n_test = 0
        cutoff = n - n_test
        train_idx.extend(ordered.index[:cutoff].tolist())
        test_idx.extend(ordered.index[cutoff:].tolist())
    return pd.Index(train_idx), pd.Index(test_idx)


def _fit_with_optional_calibration(
    base_model: Any, X_train: np.ndarray, y_train: np.ndarray, calibration_method: Optional[str]
) -> tuple[Any, bool]:
    """回傳 (已 fit 好的 model, 是否真的做了校準)。樣本太少或
    calibration_method 為 None/"none" 時直接 fit 原始 estimator，不校準——
    校準需要在訓練集內部再切一次 CV，樣本不夠時這個切分本身就不可靠，寧可
    不校準也不要用一個看起來有校準、實際上是雜訊的結果。"""
    if calibration_method and calibration_method != "none" and len(X_train) >= MIN_ROWS_FOR_CALIBRATION:
        class_counts = np.bincount(y_train) if len(np.unique(y_train)) > 1 else np.array([len(y_train)])
        if len(class_counts) >= 2 and class_counts.min() >= MIN_CLASS_COUNT_FOR_CALIBRATION:
            calibrated_model = CalibratedClassifierCV(base_model, method=calibration_method, cv=3)
            calibrated_model.fit(X_train, y_train)
            return calibrated_model, True
    base_model.fit(X_train, y_train)
    return base_model, False


def _fit_one(
    train_df: pd.DataFrame,
    test_df: pd.DataFrame,
    label_col: str,
    model_type: str,
    random_state: int,
    calibration_method: Optional[str],
) -> tuple[Any, dict[str, float]]:
    X_train = train_df[FEATURE_COLUMNS].to_numpy(dtype=float)
    y_train = train_df[label_col].to_numpy(dtype=int)
    X_test = test_df[FEATURE_COLUMNS].to_numpy(dtype=float)
    y_test = test_df[label_col].to_numpy(dtype=int)

    base_model = _build_estimator(model_type, random_state)
    model, calibrated = _fit_with_optional_calibration(base_model, X_train, y_train, calibration_method)

    metrics: dict[str, float] = {
        "train_rows": float(len(y_train)),
        "test_rows": float(len(y_test)),
        "positive_rate_train": float(y_train.mean()) if len(y_train) else float("nan"),
        "positive_rate_test": float(y_test.mean()) if len(y_test) else float("nan"),
        "calibrated": 1.0 if calibrated else 0.0,
    }

    if len(y_test) == 0:
        # 理論上 train_model() 已經保證 test set 非空，這裡只是防禦性處理，
        # 避免 predict/predict_proba 在空陣列上出錯。
        metrics.update({
            "accuracy": float("nan"), "precision": float("nan"), "recall": float("nan"),
            "auc": float("nan"), "brier_score": float("nan"), "log_loss": float("nan"),
        })
        return model, metrics

    y_pred = model.predict(X_test)
    y_proba = model.predict_proba(X_test)[:, 1]

    metrics["accuracy"] = float(accuracy_score(y_test, y_pred))
    metrics["precision"] = float(precision_score(y_test, y_pred, zero_division=0))
    metrics["recall"] = float(recall_score(y_test, y_pred, zero_division=0))
    metrics["brier_score"] = float(brier_score_loss(y_test, y_proba))
    metrics["log_loss"] = float(log_loss(y_test, y_proba, labels=[0, 1]))
    if len(np.unique(y_test)) > 1:
        metrics["auc"] = float(roc_auc_score(y_test, y_proba))
    else:
        metrics["auc"] = float("nan")

    return model, metrics


def compute_rr_reference(dataset: pd.DataFrame) -> list[float]:
    """訓練資料集裡每一列的 risk_reward_ratio 分佈（由 average_bounce_return/
    average_break_return 算出），排序後供 reward_risk_percentile() 查表用。
    只保留兩個平均報酬都非 0 的列，避免除以 0 的退化情況混進參考分佈。"""
    bounce = dataset["average_bounce_return"].to_numpy(dtype=float)
    brk = dataset["average_break_return"].to_numpy(dtype=float)
    mask = (bounce != 0) & (brk != 0)
    if not np.any(mask):
        return []
    rr = np.abs(bounce[mask] / brk[mask])
    return sorted(float(v) for v in rr)


def train_model(
    dataset: pd.DataFrame,
    model_type: str = "gradient_boosting",
    test_size: float = 0.2,
    random_state: int = 42,
    split_method: str = "time",
    calibration_method: Optional[str] = "sigmoid",
) -> ModelBundle:
    if len(dataset) < 20:
        raise ValueError(f"訓練資料太少（{len(dataset)} 筆），至少需要 20 筆 touch 事件")
    if split_method not in ("time", "random"):
        raise ValueError(f"unknown split_method: {split_method}")

    if split_method == "time":
        if "symbol" not in dataset.columns or "touch_time" not in dataset.columns:
            raise ValueError("split_method='time' 需要 dataset 有 symbol/touch_time 欄位")
        train_idx, test_idx = _time_split_indices(dataset, test_size)
        train_df, test_df = dataset.loc[train_idx], dataset.loc[test_idx]
        if test_df.empty:
            raise ValueError("時間切分後 test set 為空，資料太少或都集中在極少數股票")
    else:
        # 舊行為，保留供比較：全域隨機切分，不保證 test 在時間上晚於 train，
        # 金融時間序列容易高估表現，不建議當作正式評估依據。
        train_df, test_df = train_test_split(dataset, test_size=test_size, random_state=random_state)

    hold_model, hold_metrics = _fit_one(train_df, test_df, "hold_label", model_type, random_state, calibration_method)
    break_model, break_metrics = _fit_one(train_df, test_df, "break_label", model_type, random_state, calibration_method)

    return ModelBundle(
        hold_model=hold_model,
        break_model=break_model,
        feature_names=list(FEATURE_COLUMNS),
        trained_at=datetime.now(timezone.utc).isoformat(),
        version=MODEL_VERSION,
        metrics={"hold": hold_metrics, "break": break_metrics},
        rr_reference=compute_rr_reference(dataset),
        split_method=split_method,
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
            features.average_bounce_return,
            features.average_break_return,
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


def reward_risk_percentile(bundle: ModelBundle, current_rr: float) -> Optional[float]:
    """current_rr 在訓練資料集歷史 risk_reward_ratio 分佈中的百分位（0~100，
    見八、新增 Reward/Risk Percentile）。參考分佈為空時回傳 None（模型是用
    太少/太乾淨的資料訓練，沒有足夠的 RR 樣本可比較）。"""
    reference = bundle.rr_reference
    if not reference:
        return None
    rank = bisect.bisect_left(reference, current_rr)
    return float(100.0 * rank / len(reference))
