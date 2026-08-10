"""drawdown-like failure window 的成本量測（T-028）。

**為什麼是一支 env gate 的測試而不是一次性指令**：`development-workflow.md` 的
「測試腳本優先」要求新的檢查類型補進腳本再用腳本執行。這支掛在
`python/scripts/test.sh` 底下，用 `SR_EXCURSION_BENCH=1` 開啟；不設就 skip，
所以常態測試回合不受影響。

**為什麼在容器內用 `perf_counter` 而不是在呼叫端量 wallclock**：這個沙箱的 `sleep`
不等比推進 wallclock（見 `development-workflow.md` 的「資源特性」），呼叫端量到的
時間不可信。行程內的 `perf_counter` 不受影響。

量的是**比值**而非絕對值：既有 per-row 成本（重建 zone ＋ 決策 pipeline）對
新增窗口計算的倍數，用來判定 todo 上「成本高一個量級」的前提成不成立。
"""
from __future__ import annotations

import os
import time

import pytest

from ..dataset import DatasetConfig
from .. import evaluation as evaluation_module
from .conftest import bullish_trend_df

pytestmark = pytest.mark.skipif(
    os.environ.get("SR_EXCURSION_BENCH") != "1",
    reason="成本量測不在常態回合跑；用 SR_EXCURSION_BENCH=1 開啟",
)


def _median_seconds(fn, repeats: int) -> float:
    samples = []
    for _ in range(repeats):
        start = time.perf_counter()
        fn()
        samples.append(time.perf_counter() - start)
    samples.sort()
    return samples[len(samples) // 2]


def test_excursion_cost_relative_to_existing_per_row_cost(capsys):
    """新增的窗口計算 vs 既有 per-row 成本。"""
    df = bullish_trend_df(n=800)
    dataset_config = DatasetConfig(min_history_bars=120, forward_bars_support=5, forward_bars_resistance=5)
    idx = 600
    current_price = float(df["close"].iloc[idx])
    zone = {"role": "SUPPORT", "price_low": current_price * 0.97, "price_high": current_price * 1.03}
    rr_context = {"stop_distance_pct": 0.04}

    # (a) 既有 per-row 成本的主體：從 idx 之前的歷史重建 zone
    #     （`_historical_zone_score_summary` 的時間幾乎都花在這個 builder 迴圈）。
    #     刻意只量這一段而不含 decision pipeline，得到的比值因此是**保守**的
    #     ——低估既有成本，等於高估新增計算的相對占比。
    builders = evaluation_module.build_zone_builders(None, include_recent_microstructure=True)
    history = df.iloc[: idx + 1]

    def build_zones():
        for builder in builders:
            if len(history) >= builder.min_bars:
                builder.build(history)

    # (b) 新增的窗口計算。
    def excursion():
        evaluation_module._excursion_window(df, idx, current_price, zone, 5, rr_context)

    zone_seconds = _median_seconds(build_zones, 20)
    excursion_seconds = _median_seconds(excursion, 200)
    ratio = zone_seconds / excursion_seconds if excursion_seconds > 0 else float("inf")

    with capsys.disabled():
        print(
            f"\n[T-028 成本量測] zone 重建 {zone_seconds * 1e6:.1f}us"
            f" / 窗口計算 {excursion_seconds * 1e6:.1f}us"
            f" → 既有成本是新增的 {ratio:.1f} 倍"
        )

    assert excursion_seconds > 0
