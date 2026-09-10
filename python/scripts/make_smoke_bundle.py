"""產一份**小型**凍結 bundle，供 `scripts/smoke-replay-offline.sh` 端到端驗證用。

⛔ **不讀 DB**：Stage 0 的 DB 路徑由 pytest 覆蓋（`test_replay_bundle_stage0.py`）。
這裡要驗的是「bundle 在離線容器裡真的載入得了、Stage 1／2 真的跑得完」，
所以直接用 package 的公開 API 組一份最小但**完全合法**的 bundle。

模型是真的 joblib——`load_model()` 會驗 feature schema，假 bytes 過不了。
"""
from __future__ import annotations

import sys
from datetime import datetime, timezone
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import joblib  # noqa: E402
import numpy as np  # noqa: E402
from sklearn.linear_model import LogisticRegression  # noqa: E402

from backtest.modular.sr_scoring.model import FEATURE_COLUMNS, ModelBundle  # noqa: E402
from backtest.modular.sr_scoring.replay_bundle import (  # noqa: E402
    build_calendar_payload,
    build_manifest,
    build_provenance,
    emit_bundle,
    render_payloads,
)

BARS = 120
SYMBOL = "2330"
AS_OF = "2026-09-01"


def _fitted(seed: int):
    rng = np.random.default_rng(seed)
    x = rng.normal(size=(40, len(FEATURE_COLUMNS)))
    y = (rng.random(40) > 0.5).astype(int)
    y[0], y[1] = 0, 1  # 兩個 class 都要有，LogisticRegression 才 fit 得起來
    return LogisticRegression(max_iter=200).fit(x, y)


def main() -> None:
    out_dir = Path(sys.argv[1])
    model_path = out_dir / "smoke_model.joblib"
    joblib.dump(
        ModelBundle(
            hold_model=_fitted(1),
            break_model=_fitted(2),
            feature_names=list(FEATURE_COLUMNS),
            trained_at="2026-09-01T00:00:00+00:00",
            version="smoke",
        ),
        model_path,
    )

    base_ts = 1_735_689_600  # 2025-01-01T00:00:00Z
    candles = [
        {
            "timestamp": base_ts + i * 86400,
            "timeframe": "1d",
            "open": 100.0 + i * 0.4,
            "high": 101.0 + i * 0.4,
            "low": 99.0 + i * 0.4,
            "close": 100.5 + i * 0.4 + (0.6 if i % 5 == 0 else 0.0),
            "volume": 1000.0 + (500.0 if i % 7 == 0 else 0.0),
        }
        for i in range(BARS)
    ]
    replay_config = {
        "as_of": AS_OF,
        "timeframe": "1d",
        "limit": 1500,
        "replay_scope": "all_candidates",
        "dataset_config": {"min_history_bars": 80, "forward_bars_support": 5,
                           "forward_bars_resistance": 5},
        "builder_config": {},
        "dataset_from": "2025-01-01T00:00:00+00:00",
        "dataset_to": "2025-04-30T00:00:00+00:00",
    }
    payloads = render_payloads(
        candles_by_symbol={SYMBOL: candles},
        chip_by_symbol={},
        governance_by_symbol={},
        replay_config=replay_config,
        trading_calendar=build_calendar_payload([2025, 2026], {}),
        model_bytes=model_path.read_bytes(),
    )
    _bundle_id, manifest = build_manifest(
        payloads=payloads,
        as_of=AS_OF,
        timeframe="1d",
        symbols=[SYMBOL],
        limit=1500,
        replay_scope="all_candidates",
        report_max_rows=50,
        captured_at=datetime.now(timezone.utc).isoformat(),
        readiness={"timeframe": "1d", "expected_latest": AS_OF, "market_latest": AS_OF},
        calendar={"mode": "online", "years": [{"year": 2025, "fetched_at": "x", "raw_row_count": 1},
                                              {"year": 2026, "fetched_at": "x", "raw_row_count": 1}]},
        provenance=build_provenance(
            source_root="/app",
            image_digest="sha256:" + "0" * 64,
            base_commit=None,
            tooling_patch_sha256=None,
            runner_sha256="s" * 64,
            argv=["make_smoke_bundle.py"],
        ),
    )
    outcome = emit_bundle(out_dir, payloads, manifest, log=lambda m: print(m, file=sys.stderr))
    model_path.unlink()
    print(outcome.bundle_id)


if __name__ == "__main__":
    main()
