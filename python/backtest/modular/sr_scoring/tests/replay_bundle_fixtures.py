"""I-100 bundle 測試共用的最小 payload。

**刻意放在 tests/ 而不是 conftest**：這些是要被多支測試檔 import 的建構函式，不是 fixture；
放 conftest 會讓 pytest 把它們當成 fixture 解析而拿不到參數。
"""
from __future__ import annotations

from ..replay_bundle import build_calendar_payload, build_manifest, render_payloads

# ⚠️ 用**真的建出來的完整年度日曆**，⛔ 不要手捏一天了事：正式 loader 會驗完整年度
# 不變條件，手捏的片段能過測試卻過不了 loader，那種 fixture 只會讓測試對不到現實。
MINIMAL_CALENDAR = build_calendar_payload([2025, 2026], {})

MINIMAL_PROVENANCE = {
    "image_digest": "sha256:" + "0" * 64,
    "python_version": "3.11.9",
    "pip_freeze_sha256": "1" * 64,
    "project_modules_sha256": {"backtest.modular.sr_scoring.evaluation": "2" * 64},
    "runner_sha256": "3" * 64,
    "argv": ["--as-of", "2026-09-01"],
    "runtime_settings": {"db_driver": "sqlite"},
}


def make_replay_config(*, as_of="2026-09-01", timeframe="1d", limit=1500,
                       replay_scope="all_candidates", dataset_config=None,
                       builder_config=None):
    """完整的 `replay_config.json`。

    ⚠️ 四個 mirrored 欄位（`as_of`／`timeframe`／`limit`／`replay_scope`）由同一組參數
    產生，manifest 與 replay_config 才不會不一致——正式 loader 兩邊都會比。
    """
    return {
        "as_of": as_of,
        "timeframe": timeframe,
        "limit": limit,
        "replay_scope": replay_scope,
        "dataset_config": dataset_config or {"min_history_bars": 80,
                                             "forward_bars_support": 5,
                                             "forward_bars_resistance": 5},
        "builder_config": builder_config or {},
        "dataset_from": "2026-01-01T00:00:00+00:00",
        "dataset_to": "2026-09-01T00:00:00+00:00",
    }


def make_payload_inputs(**overrides):
    data = {
        "candles_by_symbol": {
            "2330": [
                {"timestamp": 1756684800, "open": 1.0, "high": 2.0, "low": 0.5, "close": 1.5,
                 "volume": 10.0, "timeframe": "1d"},
                {"timestamp": 1756771200, "open": 1.5, "high": 2.5, "low": 1.0, "close": 2.0,
                 "volume": 20.0, "timeframe": "1d"},
            ]
        },
        "chip_by_symbol": {"2330": [{"trade_date": "2026-09-01", "total_score": 55.0}]},
        "governance_by_symbol": {
            "2330": [{"timeframe": "1d", "as_of": "2026-09-01T00:00:00+00:00",
                      "created_at": "2026-09-01T01:00:00+00:00", "model_version": "v4",
                      "model_config_hash": "abc", "health_state": "HEALTHY"}]
        },
        "replay_config": make_replay_config(),
        "trading_calendar": MINIMAL_CALENDAR,
        "model_bytes": b"fake-model-bytes",
    }
    data.update(overrides)
    return data


def make_manifest_and_payloads(
    *,
    captured_at: str = "2026-09-01T00:00:00+00:00",
    as_of: str = "2026-09-01",
    timeframe: str = "1d",
    symbols: list[str] | None = None,
    **payload_overrides,
):
    """回傳 `(manifest, payloads)`——兩者一致，可直接餵給 `emit_bundle`。"""
    symbols = symbols or ["2330"]
    payload_overrides.setdefault(
        "replay_config", make_replay_config(as_of=as_of, timeframe=timeframe)
    )
    payloads = render_payloads(**make_payload_inputs(**payload_overrides))
    _bundle_id, manifest = build_manifest(
        payloads=payloads,
        as_of=as_of,
        timeframe=timeframe,
        symbols=symbols,
        limit=1500,
        replay_scope="all_candidates",
        report_max_rows=200,
        captured_at=captured_at,
        readiness={"expected_latest": "2026-09-01", "market_latest": "2026-09-01"},
        calendar={"mode": "online", "years": [{"year": 2026, "fetched_at": captured_at,
                                               "raw_row_count": 27}]},
        provenance=MINIMAL_PROVENANCE,
    )
    return manifest, payloads
