"""Stage 1 專屬 preflight 與 capacity probe 的 schema（I-074 Stage 1）。

對應計畫書三／四／十二與測試矩陣 1／9。
"""
from __future__ import annotations

import pytest

from ..replay_bundle import i074_preflight as pf
from ..replay_bundle import probe as pb
from ..replay_bundle.artifacts import ArtifactError

# 末根 epoch 1788192000：UTC 日期是 2026-08-31，**台北**才是 2026-09-01。
LAST_TS = 1788192000


def _universe(n: int):
    """n 列，平均散在 11 檔上。"""
    symbols = sorted(pf.I074_SYMBOLS)
    return [(symbols[i % len(symbols)], "1d", f"2026-01-{i:05d}") for i in range(n)]


def _quota(total: int):
    symbols = sorted(pf.I074_SYMBOLS)
    base, extra = divmod(total, len(symbols))
    return {s: base + (1 if i < extra else 0) for i, s in enumerate(symbols)}


def _last_ts():
    return {s: LAST_TS for s in pf.I074_SYMBOLS}


def _pre(**overrides):
    kwargs = {
        "bundle_id": pf.I074_BUNDLE_ID,
        "universe": _universe(pf.I074_EXPECTED_ROWS),
        "quota": _quota(pf.I074_EXPECTED_ROWS),
        "last_timestamps": _last_ts(),
    }
    kwargs.update(overrides)
    return pf.preflight_pre(**kwargs)


# ── 台北日期 ────────────────────────────────────────────────────────────────

def test_taipei_date_is_not_utc_date():
    """⚠️ 這條釘死的是一個**會擋掉正確 bundle** 的錯誤：直接比 UTC 日期。"""
    import datetime as dt

    assert pf.taipei_date(LAST_TS) == "2026-09-01"
    utc_date = dt.datetime.fromtimestamp(LAST_TS, dt.timezone.utc).date().isoformat()
    assert utc_date == "2026-08-31", "fixture 沒踩到跨日邊界，這條就沒在測東西"


def test_pre_passes_with_correct_inputs():
    _pre()


def test_wrong_bundle_id_is_rejected():
    with pytest.raises(ArtifactError, match="bundle_id"):
        _pre(bundle_id="b1_other")


def test_missing_symbol_is_rejected():
    universe = [k for k in _universe(pf.I074_EXPECTED_ROWS) if k[0] != "2330"]
    with pytest.raises(ArtifactError, match="symbols"):
        _pre(universe=universe)


def test_last_bar_on_wrong_taipei_date_is_rejected():
    ts = dict(_last_ts())
    ts["2330"] = LAST_TS - 86400
    with pytest.raises(ArtifactError, match="台北日期"):
        _pre(last_timestamps=ts)


def test_row_count_must_be_13417():
    with pytest.raises(ArtifactError, match="候選列數"):
        _pre(quota=_quota(pf.I074_EXPECTED_ROWS - 1))


def test_post_checks_actual_output():
    universe = _universe(pf.I074_EXPECTED_ROWS)
    rows = [{"symbol": s, "timeframe": t, "as_of": a} for s, t, a in universe]
    pf.preflight_post(rows=rows, universe=universe)
    with pytest.raises(ArtifactError, match="實際產出"):
        pf.preflight_post(rows=rows[:-1], universe=universe)


# ── probe schema ────────────────────────────────────────────────────────────

def _prov():
    return {
        "source_root": "/app", "image_digest": "sha256:" + "a" * 64,
        "python_version": "3.11.16", "pip_freeze_sha256": "b" * 64,
        "project_modules_sha256": {"config": "c" * 64}, "runner_sha256": "d" * 64,
        "base_commit": "e" * 40, "tooling_patch_sha256": "f" * 64,
        "argv": ["--output-dir", "/tmp/x"], "runtime_settings": {"db_driver": "postgres"},
    }


def _probe_rows():
    symbols = sorted(pf.I074_SYMBOLS)
    quota = _quota(pb.PROBE_QUOTA)
    rows = []
    for symbol in symbols:
        for i in range(quota[symbol]):
            structure, action = "SUPPORT_RECLAIM_CONFIRMED", "HOLD"
            rows.append({
                "symbol": symbol, "timeframe": "1d", "as_of": f"2026-08-{i:05d}",
                "lifecycle_phase": "TESTING", "clear_zone_breakout": False,
                "continuation_price_evidence_met": False, "setup_rr_qualified": False,
                "rr_decoupling_candidate": False, "event_signal": "CLOSE_RECLAIM",
                "structure_state": structure, "action_state": action,
                "position_action_condition": {"state": action, "structure_state": structure},
                "position_action": action,
            })
    return rows, quota


def _computation(**overrides):
    rows, quota = _probe_rows()
    payload = {
        "schema_version": 1, "kind": pb.PROBE_COMPUTATION_KIND,
        "bundle_id": pf.I074_BUNDLE_ID, "generated_at": "2026-09-14T00:00:00+08:00",
        "provenance": _prov(), "quota_by_symbol": quota,
        "row_count": len(rows), "rows": rows, "elapsed_seconds": 12.5,
    }
    payload.update(overrides)
    return payload


def test_computation_passes():
    pb.validate_probe_computation(_computation())


@pytest.mark.parametrize("bad", [200.0, True])
def test_row_count_must_be_strict_int(bad):
    """⚠️ `200.0 == 200` 為真——只比值不比型別的話 float 會被放行。"""
    with pytest.raises(ArtifactError):
        pb.validate_probe_computation(_computation(row_count=bad))


@pytest.mark.parametrize("bad", [True, 12, "12.5"])
def test_elapsed_seconds_must_be_float(bad):
    with pytest.raises(ArtifactError):
        pb.validate_probe_computation(_computation(elapsed_seconds=bad))


def test_quota_must_cover_all_eleven_symbols():
    _rows, quota = _probe_rows()
    partial = {k: v for k, v in quota.items() if k != "2330"}
    with pytest.raises(ArtifactError, match="11 檔"):
        pb.validate_probe_computation(_computation(quota_by_symbol=partial))


def test_per_symbol_row_count_must_equal_quota():
    rows, quota = _probe_rows()
    with pytest.raises(ArtifactError):
        pb.validate_probe_computation(_computation(rows=rows[:-1], row_count=len(rows) - 1))


def _measurement(**overrides):
    payload = {
        "schema_version": 1, "kind": pb.PROBE_MEASUREMENT_KIND,
        "bundle_id": pf.I074_BUNDLE_ID, "generated_at": "2026-09-14T00:00:00+08:00",
        "provenance": _prov(), "peak_rss_bytes": 300 * 1024 * 1024,
        "host_low_bytes": 100 * 1024 * 1024, "cgroup_limit_bytes": 700 * 1024 * 1024,
    }
    payload.update(overrides)
    return payload


def test_measurement_passes():
    pb.validate_probe_measurement(_measurement())


@pytest.mark.parametrize("field", ["peak_rss_bytes", "host_low_bytes", "cgroup_limit_bytes"])
@pytest.mark.parametrize("bad", [0, None, True, -1])
def test_measurement_rejects_zero_placeholder(field, bad):
    """⚠️ ⛔ 不得把「量不到」歸檔成 0——那會讓後續判讀以為峰值極低。"""
    with pytest.raises(ArtifactError):
        pb.validate_probe_measurement(_measurement(**{field: bad}))


def _completion(comp_sha, meas_sha, **overrides):
    payload = {
        "schema_version": 1, "kind": pb.PROBE_COMPLETION_KIND,
        "bundle_id": pf.I074_BUNDLE_ID, "generated_at": "2026-09-14T00:00:00+08:00",
        "provenance": _prov(), "completed": True,
        "computation_artifact_sha256": comp_sha, "measurement_artifact_sha256": meas_sha,
    }
    payload.update(overrides)
    return payload


def test_completion_must_point_at_actual_artifacts():
    comp, meas = "1a" * 32, "2b" * 32
    pb.validate_probe_completion(_completion(comp, meas),
                                 computation_sha256=comp, measurement_sha256=meas)
    with pytest.raises(ArtifactError, match="不符"):
        pb.validate_probe_completion(_completion(comp, meas),
                                     computation_sha256="3c" * 32, measurement_sha256=meas)


@pytest.mark.parametrize("bad", [1, "true", False])
def test_completed_must_be_strict_true(bad):
    comp, meas = "1a" * 32, "2b" * 32
    with pytest.raises(ArtifactError):
        pb.validate_probe_completion(_completion(comp, meas, completed=bad),
                                     computation_sha256=comp, measurement_sha256=meas)
