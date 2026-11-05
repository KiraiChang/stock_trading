"""I-129 離線影響量測的比較工具（`i129_measurement.py`；規則見計畫書「6.5」）。

三部分：
1. 成交與停損、目標的判定（`simulate_trade`）：「6.5」列的每個邊界案例。
2. 判準的順序（`judge`）：無法歸因 → 不變式 → 證據不足（⛔ 不改用較寬的判準）→ 安全。
3. 端到端（`compare_artifacts`）：用合成的 A／B after artifact 串流比較。
"""
from __future__ import annotations

from collections import Counter

import pytest

from ..i129_measurement import (
    MIN_SAMPLES,
    Bar,
    judge,
    outcome_metrics,
    simulate_trade,
)

LIMIT = "PRIMARY_SUPPORT_UPPER"
MARKET = "CONTINUATION_MARKET_PRICE"


def _bars(*ohlc):
    return [Bar(*x) for x in ohlc]


# ── 1. 成交與停損、目標的判定 ─────────────────────────────────────────────────
# 除了市價型那一條，數字都是限價 100、停損 98、目標 104。

def test_review_counterexample_intraday_fill_bar_does_not_credit_target():
    """第三輪 review 的反例：開 103、高 105、低 99 → 以 100 成交，當根的 high 可能在成交之前，⛔ 不認列目標。"""
    first = (103.0, 105.0, 99.0, 101.0)
    # 之後一根開 99、低 97.5 → 停損、−1R
    out = simulate_trade(LIMIT, 100.0, 98.0, 104.0, _bars(first, (99.0, 100.0, 97.5, 98.5)) + _bars(*[(99.0, 99.5, 98.5, 99.0)] * 3))
    assert (out.fill_kind, out.fill_price) == ("LIMIT_INTRADAY", 100.0)
    assert out.exit_kind == "STOP" and out.realized_r == pytest.approx(-1.0)
    assert out.target_first is False
    # 改成之後一根開 101、高 104.5、低 99 → 目標、+2R
    out = simulate_trade(LIMIT, 100.0, 98.0, 104.0, _bars(first, (101.0, 104.5, 99.0, 104.0)) + _bars(*[(104.0, 104.0, 103.0, 103.5)] * 3))
    assert out.exit_kind == "TARGET" and out.realized_r == pytest.approx(2.0)
    assert out.target_first is True


def test_intraday_fill_alone_never_credits_target():
    """只有成交當根碰到目標、之後都沒碰到 → ⛔ 不算先到目標，記窗口最後一根收盤的未實現 R。"""
    out = simulate_trade(LIMIT, 100.0, 98.0, 104.0, _bars((103.0, 105.0, 99.0, 101.0), *[(101.0, 102.0, 100.5, 101.0)] * 4))
    assert out.exit_kind == "WINDOW_END"
    assert out.target_first is False
    assert out.realized_r == pytest.approx((101.0 - 100.0) / 2.0)


def test_open_fill_credits_target_on_fill_bar():
    """開盤即成交：開 99.5、高 105、低 99 → 以 99.5 成交、整根都在成交之後，當根認列目標（+3R）。"""
    out = simulate_trade(LIMIT, 100.0, 98.0, 104.0, _bars((99.5, 105.0, 99.0, 104.0), *[(104.0, 104.0, 103.0, 103.5)] * 4))
    assert (out.fill_kind, out.fill_price) == ("LIMIT_OPEN", 99.5)
    assert out.exit_kind == "TARGET" and out.realized_r == pytest.approx(3.0)


def test_same_bar_stop_and_target_counts_stop_first():
    """開盤即成交、同根碰停損與目標：開 99、高 105、低 97 → 停損先到（日 K 看不出先後，採保守判定）。"""
    out = simulate_trade(LIMIT, 100.0, 98.0, 104.0, _bars((99.0, 105.0, 97.0, 100.0), *[(100.0, 100.0, 99.0, 99.5)] * 4))
    assert out.exit_kind == "STOP"
    assert out.stop_touched is True and out.target_first is False
    assert out.realized_r == pytest.approx(-1.0)


def test_intraday_fill_bar_stop_is_judged():
    """盤中觸價、當根碰停損：開 103、高 105、低 97 → 當根停損、−1R（當根的 low 必然在成交之後）。"""
    out = simulate_trade(LIMIT, 100.0, 98.0, 104.0, _bars((103.0, 105.0, 97.0, 98.0), *[(98.0, 99.0, 97.0, 98.0)] * 4))
    assert out.fill_kind == "LIMIT_INTRADAY"
    assert out.exit_kind == "STOP" and out.realized_r == pytest.approx(-1.0)


def test_gap_fill_below_stop_counts_minus_one_and_stays_in_mean_r():
    """跳空成交在停損之下（市價型，停損 98、次根開盤 97.5）→ 當根停損、−1R，而且有算進平均實現 R。"""
    out = simulate_trade(MARKET, 105.0, 98.0, 122.0, _bars((97.5, 99.0, 96.0, 98.0), *[(98.0, 99.0, 97.0, 98.0)] * 4))
    assert out.fill_price == 97.5
    assert out.exit_kind == "STOP" and out.realized_r == -1.0
    metrics = outcome_metrics([out], [])
    assert metrics["realized_r_n"] == 1 and metrics["mean_realized_r"] == -1.0, "最差的列⛔ 不得被排除"


def test_later_gap_below_stop_is_worse_than_minus_one():
    """之後某根跳空低開在停損之下 → 按開盤價計，比 −1 更差。"""
    out = simulate_trade(MARKET, 105.0, 98.0, 122.0, _bars((105.0, 106.0, 104.0, 105.0), (96.0, 97.0, 95.0, 96.0), *[(96.0, 97.0, 95.0, 96.0)] * 3))
    assert out.exit_kind == "STOP"
    assert out.realized_r == pytest.approx((96.0 - 105.0) / 7.0)


def test_limit_never_touched_is_unfilled():
    """窗口內都沒碰到 entry → 未成交，不進結果指標、算進成交率。"""
    out = simulate_trade(LIMIT, 100.0, 98.0, 104.0, _bars(*[(102.0, 103.0, 100.5, 102.0)] * 5))
    assert out.filled is False
    metrics = outcome_metrics([out], [])
    assert metrics["signals"] == 1 and metrics["filled"] == 0 and metrics["fill_rate"] == 0.0
    assert metrics["stop_touch_n"] == 0 and metrics["realized_r_n"] == 0


def test_market_fills_at_next_open_and_judges_whole_bar():
    """市價型：下一根開盤成交，整根都在成交之後（當根可認列目標）。"""
    out = simulate_trade(MARKET, 105.0, 98.0, 110.0, _bars((105.5, 111.0, 104.0, 110.0), *[(110.0, 111.0, 109.0, 110.0)] * 4))
    assert (out.fill_kind, out.fill_price) == ("MARKET_OPEN", 105.5)
    assert out.exit_kind == "TARGET" and out.realized_r == pytest.approx((110.0 - 105.5) / 7.5)


def test_target_unknown_has_no_target_first_but_has_r():
    """target 未知：⛔ 不進「先到目標」的分母，但仍有實現 R（窗口結束的未實現 R）。"""
    out = simulate_trade(MARKET, 105.0, 98.0, None, _bars(*[(105.0, 106.0, 104.0, 106.0)] * 5))
    assert out.target_first is None
    assert out.realized_r == pytest.approx((106.0 - 105.0) / 7.0)
    assert outcome_metrics([out], [])["target_first_n"] == 0


@pytest.mark.parametrize("basis", ["UNAVAILABLE", "UNSUPPORTED_ENTRY_ZONE", ""])
def test_non_entry_basis_is_not_simulated(basis):
    assert simulate_trade(basis, 100.0, 98.0, 104.0, _bars(*[(100.0, 101.0, 99.0, 100.0)] * 5)) is None


def test_missing_entry_is_not_simulated():
    assert simulate_trade(LIMIT, None, 98.0, 104.0, _bars(*[(100.0, 101.0, 99.0, 100.0)] * 5)) is None


# ── 2. 判準的順序 ─────────────────────────────────────────────────────────────

def _metric(**overrides):
    base = {
        "stop_touch_n": MIN_SAMPLES, "target_first_n": MIN_SAMPLES, "realized_r_n": MIN_SAMPLES,
        "forward_return_n": MIN_SAMPLES, "stop_touch_rate": 0.30, "target_first_rate": 0.40,
        "mean_realized_r": 0.10, "mean_forward_return": 0.01, "positive_forward_rate": 0.55,
    }
    return base | overrides


ZERO = {"u_field_changed_rows": 0, "upgrades": 0, "tightened": 0, "setup_missing_released": 0}
SPREAD = Counter({f"S{i}": 10 for i in range(5)})


def _judge(**kw):
    args = {"unattributable": [], "invariants": dict(ZERO), "g_new": _metric(), "p": _metric(), "g_new_symbols": SPREAD}
    args.update(kw)
    return judge(**args)


def test_judge_pass():
    assert _judge() == ("PASS", [])


def test_judge_unattributable_comes_first():
    verdict, reasons = _judge(unattributable=["bundle 不同"], invariants=ZERO | {"upgrades": 3})
    assert verdict == "UNATTRIBUTABLE" and reasons == ["bundle 不同"]


def test_judge_invariant_before_evidence():
    verdict, _ = _judge(invariants=ZERO | {"tightened": 1}, g_new=_metric(stop_touch_n=0))
    assert verdict == "INVARIANT_VIOLATION"


@pytest.mark.parametrize("side, key", [
    ("g_new", "stop_touch_n"), ("p", "stop_touch_n"), ("g_new", "target_first_n"), ("p", "target_first_n"),
    ("g_new", "realized_r_n"), ("p", "realized_r_n"), ("g_new", "forward_return_n"),
])
def test_judge_any_insufficient_sample_falls_back(side, key):
    """任何一項判準的有效樣本不足 → 證據不足，⛔ 不改用較寬的判準（計畫書 D5）——即使其他數字都很好看。"""
    verdict, reasons = _judge(**{side: _metric(**{key: MIN_SAMPLES - 1})})
    assert verdict == "EVIDENCE_INSUFFICIENT"
    assert any(str(MIN_SAMPLES - 1) in r for r in reasons)


def test_judge_symbol_count_and_concentration():
    assert _judge(g_new_symbols=Counter({f"S{i}": 10 for i in range(4)}))[0] == "EVIDENCE_INSUFFICIENT"
    concentrated = Counter({"S0": 51, "S1": 13, "S2": 12, "S3": 12, "S4": 12})
    assert _judge(g_new_symbols=concentrated)[0] == "EVIDENCE_INSUFFICIENT"
    exactly_half = Counter({"S0": 50, "S1": 13, "S2": 13, "S3": 12, "S4": 12})
    assert _judge(g_new_symbols=exactly_half)[0] == "PASS", "≤ 50% 是合格"


@pytest.mark.parametrize("g_new, ok", [
    (_metric(stop_touch_rate=0.35), True),            # = P + 5pp
    (_metric(stop_touch_rate=0.351), False),
    (_metric(target_first_rate=0.35), True),          # = P_q − 5pp
    (_metric(target_first_rate=0.349), False),
    (_metric(mean_realized_r=0.0), True),
    (_metric(mean_realized_r=-0.001), False),
    (_metric(mean_forward_return=0.0), True),
    (_metric(mean_forward_return=-0.0001), False),
    (_metric(positive_forward_rate=0.45), True),
    (_metric(positive_forward_rate=0.449), False),
])
def test_judge_safety_boundaries(g_new, ok):
    verdict, _ = _judge(g_new=g_new)
    assert verdict == ("PASS" if ok else "SAFETY_FAILED")


# ── 3. 端到端：合成的 A／B after artifact ─────────────────────────────────────

import importlib.util  # noqa: E402
from pathlib import Path  # noqa: E402

import pandas as pd  # noqa: E402

from ..i129_measurement import EMPTY_PATCH_SHA256, build_candle_index, compare_artifacts  # noqa: E402
from ..replay_bundle import (  # noqa: E402
    DECISION_MEASUREMENT_FIELDS,
    DIAGNOSTIC_NO_ZONE_FALLBACK,
    NO_ZONE_SCORES_ERROR,
    ArtifactError,
    build_after_artifact,
)
from ..replay_bundle.canonical import canonical_json_bytes  # noqa: E402

BUNDLE_ID = "b1_test_bundle"
SYMBOLS = [f"10{i}" for i in range(5)]
BASE_TS = 1_735_689_600  # 2025-01-01T00:00:00Z
N_BARS = 60
FB = 5
A_COMMIT, B_COMMIT = "a" * 40, "b" * 40
IMAGE = "sha256:" + "1" * 64


def _close(i: int) -> float:
    # 收盤逐日微幅上升：forward_return > 0；開 100、高 105、低 99.5 → 限價 100 開盤即成交、當根碰到目標 104（+2R）
    return 101.0 + 0.01 * i


def _bar(i: int) -> dict:
    return {"open": 100.0, "high": 105.0, "low": 99.5, "close": _close(i)}


def _as_of(i: int) -> str:
    return pd.Timestamp(pd.to_datetime(BASE_TS + i * 86400, unit="s", utc=True)).isoformat()


def _candle_rows():
    return {s: [{"timestamp": BASE_TS + i * 86400, **_bar(i), "volume": 1.0} for i in range(N_BARS)] for s in SYMBOLS}


def _candles():
    return build_candle_index(_candle_rows())


def _labels(i: int) -> dict:
    """結果標籤：與 bundle 的 candles 一致（比較工具會跨版本核對、也會從 candles 重算）。"""
    return {"current_price": _close(i), "forward_bars": FB, "forward_return": _close(i + FB) / _close(i) - 1.0}


def _row(symbol, i, *, setup_ok, final, side, action="Hold", market_action="WATCH", gate_ok=True,
         gate_reason="RR_QUALIFIED", semantic=None, semantic_codes=(), path="OPEN_PATH", final_codes=(),
         position_codes=("PRIMARY_SUPPORT",), setup_rr="auto", d3=False, **extra):
    """一列合成的 replay 列。`d3=True`：setup 不合格、executable 可量化且合格（B 的 G_new 該長的樣子）。"""
    structure, action_state = "SUPPORT_RECLAIM_CONFIRMED", "HOLD"
    phase = extra.pop("phase", "TESTING")
    gate = {"qualified": gate_ok, "reason_code": gate_reason, "gate_basis": "ENTRY_STOP_TARGET",
            "minimum_rr": 1.8, "actual_rr": 2.0}
    if side == "B":
        gate["setup_qualified"] = setup_ok
    if d3:
        semantic_codes = tuple(semantic_codes) + ("SETUP_RR_BELOW_MINIMUM",)
        final_codes = tuple(final_codes) + ("SETUP_RR_BELOW_MINIMUM",)
    row = {
        "symbol": symbol, "timeframe": "1d", "as_of": _as_of(i),
        "zone_score_available": True, "zone_score_error": None, "decision_error": None,
        "lifecycle_phase": phase, "clear_zone_breakout": False, "continuation_price_evidence_met": False,
        "setup_rr_qualified": setup_ok, "rr_decoupling_candidate": phase == "CONTINUATION" and not setup_ok,
        "event_signal": "CLOSE_RECLAIM", "structure_state": structure, "action_state": action_state,
        "position_action_condition": {"state": action_state, "structure_state": structure,
                                      "reason_codes": list(position_codes)},
        "position_action": action_state,
        "decision_fields_available": True,
        "final_entry_state": final, "action": action, "market_action": market_action,
        "entry_action_state": "SMALL_ENTRY", "semantic_entry_permission_state": semantic or final,
        "semantic_reason_codes": list(semantic_codes), "path_gate_state": path,
        "final_entry_reason_codes": list(final_codes), "risk_notes_digest": "0123456789abcdef",
        "rr_gate": gate,
        # TIER_3 support、SMALL_ENTRY → 門檻 1.8；entry／stop／target 100／98／104 → RR (104−100)/(100−98)=2.0
        "primary_zone": {"tier": "TIER_3_SHORT_TERM", "role": "SUPPORT"},
        "rr_context": {"price_basis": "PRIMARY_SUPPORT_UPPER", "entry_price": 100.0, "stop_price": 98.0,
                       "target_price": 104.0, "target_known": True, "executable_rr": 2.0,
                       "setup_rr": (2.2 if setup_ok else 1.4) if setup_rr == "auto" else setup_rr},
        **_labels(i),
    }
    row.update(extra)
    return row


def _no_zone_row(symbol, i):
    row = {"symbol": symbol, "timeframe": "1d", "as_of": _as_of(i), "zone_score_available": False,
           "zone_score_error": NO_ZONE_SCORES_ERROR, "decision_error": None, "lifecycle_phase": None,
           "decision_fields_available": False, "final_entry_state": None, "rr_context": None, "rr_gate": None,
           **_labels(i), **DIAGNOSTIC_NO_ZONE_FALLBACK}
    return row | dict.fromkeys(DECISION_MEASUREMENT_FIELDS)


def _prov(commit, *, image=IMAGE, patch=EMPTY_PATCH_SHA256, settings=None):
    return {
        "source_root": "/app", "image_digest": image, "python_version": "3.11.9",
        "pip_freeze_sha256": "2" * 64, "project_modules_sha256": {"backtest.modular.sr_scoring.evaluation": "3" * 64},
        "runner_sha256": "4" * 64, "base_commit": commit, "tooling_patch_sha256": patch,
        "argv": ["--stage", "1"], "runtime_settings": settings or {"db_driver": "postgres"},
    }


def _write(tmp_path, name, rows, *, bundle_id=BUNDLE_ID, provenance=None, commit=None, **prov_kw):
    prov = provenance if provenance is not None else _prov(commit or (A_COMMIT if name == "A" else B_COMMIT), **prov_kw)
    art = build_after_artifact(
        bundle_id=bundle_id, timeframe="1d", replay_scope="all_candidates", run_id=None, pipeline_version=None,
        generated_at="2026-11-04T00:00:00+08:00", provenance=prov, rows=rows,
    )
    path = tmp_path / f"{name}.json"
    path.write_bytes(canonical_json_bytes(art))
    return str(path)


def _passing_pair():
    """G_new（setup 不合格：A BLOCKED → B PROBE）與 P（setup 合格：兩邊都 PROBE）各 40 列、分在 5 檔。"""
    a, b = [], []
    for s in SYMBOLS:
        for i in range(8):
            a.append(_row(s, i, setup_ok=False, final="BLOCKED", side="A"))
            b.append(_row(s, i, setup_ok=False, final="PROBE_ALLOWED", side="B", action="BuySmall",
                          market_action="BUY_SMALL", d3=True))
        for i in range(8, 16):
            a.append(_row(s, i, setup_ok=True, final="PROBE_ALLOWED", side="A", action="BuySmall", market_action="BUY_SMALL"))
            b.append(_row(s, i, setup_ok=True, final="PROBE_ALLOWED", side="B", action="BuySmall", market_action="BUY_SMALL"))
        a.append(_no_zone_row(s, 20))
        b.append(_no_zone_row(s, 20))
    return a, b


def _run(tmp_path, a_rows, b_rows, *, a_kw=None, b_kw=None, expect=(A_COMMIT, B_COMMIT)):
    return compare_artifacts(
        _write(tmp_path, "A", a_rows, **(a_kw or {})), _write(tmp_path, "B", b_rows, **(b_kw or {})), _candles(),
        bundle_id=BUNDLE_ID, expected_a_commit=expect[0], expected_b_commit=expect[1],
    )


def test_e2e_passing_pair(tmp_path):
    report = _run(tmp_path, *_passing_pair())
    assert report["verdict"] == "PASS", report["verdict_reasons"]
    assert report["groups"] == {"G_new": 40, "P": 40, "NO_DECISION": 5}
    g_new = report["metrics"]["G_new"]
    assert g_new["filled"] == 40 and g_new["target_first_rate"] == 1.0 and g_new["mean_realized_r"] == pytest.approx(2.0)
    assert g_new["positive_forward_rate"] == 1.0
    assert report["g_new_symbols"] == {s: 8 for s in SYMBOLS}


def test_e2e_small_sample_is_insufficient(tmp_path):
    a, b = _passing_pair()
    keep = [i for i, r in enumerate(b) if not (r.get("final_entry_state") == "PROBE_ALLOWED" and not r["setup_rr_qualified"]
                                               and r["symbol"] != SYMBOLS[0])]
    report = _run(tmp_path, [a[i] for i in keep], [b[i] for i in keep])
    assert report["verdict"] == "EVIDENCE_INSUFFICIENT"


# ── 輸入身分（第 8 步 review P1-1）：⛔ 不能把不同來源的差異歸因於 I-129 ──────────

@pytest.mark.parametrize("case", [
    "different_image", "non_empty_patch_a", "non_empty_patch_b", "missing_provenance_both",
    "provenance_field_missing", "a_commit_mismatch", "b_commit_mismatch", "same_commit_both_sides",
    "bundle", "settings", "keys",
])
def test_e2e_inconsistent_inputs_are_unattributable(tmp_path, case):
    a, b = _passing_pair()
    a_kw, b_kw, expect = {}, {}, (A_COMMIT, B_COMMIT)
    if case == "different_image":
        b_kw = {"image": "sha256:" + "9" * 64}
    elif case == "non_empty_patch_a":
        a_kw = {"patch": "5" * 64}
    elif case == "non_empty_patch_b":
        b_kw = {"patch": "5" * 64}
    elif case == "missing_provenance_both":
        a_kw = b_kw = {"provenance": {}}
    elif case == "provenance_field_missing":
        broken = _prov(B_COMMIT)
        del broken["pip_freeze_sha256"]
        b_kw = {"provenance": broken}
    elif case == "a_commit_mismatch":
        a_kw = {"commit": "c" * 40}
    elif case == "b_commit_mismatch":
        b_kw = {"commit": "c" * 40}
    elif case == "same_commit_both_sides":
        b_kw = {"commit": A_COMMIT}
        expect = (A_COMMIT, A_COMMIT)
    elif case == "bundle":
        b_kw = {"bundle_id": "b1_other"}
    elif case == "settings":
        b_kw = {"settings": {"db_driver": "sqlite"}}
    else:
        b = b[:-1]
        a = a[:-1] + [a[-1]]
        b_kw = {}
    report = _run(tmp_path, a, b, a_kw=a_kw, b_kw=b_kw, expect=expect)
    assert report["verdict"] == "UNATTRIBUTABLE", (case, report["verdict_reasons"])


# ── G_new 的 D3 放行前提（第 8 步 review P1-2）：⛔ 不能由績效數字抵銷 ──────────

@pytest.mark.parametrize("mutate", [
    lambda r: r["rr_gate"].update(qualified=False, reason_code="EXECUTION_RR_INSUFFICIENT"),
    lambda r: (r["rr_gate"].update(gate_basis="ZONE_STATISTIC", actual_rr=1.4),
               r["rr_context"].update(target_price=None, target_known=False, executable_rr=None)),
    lambda r: r["rr_context"].update(executable_rr=None),
    lambda r: r["rr_context"].update(target_known=False),
    lambda r: r["rr_gate"].update(actual_rr=1.5),                       # < minimum_rr 1.8
    lambda r: r.update(final_entry_reason_codes=[]),                    # 封頂理由缺席
    lambda r: r.update(semantic_reason_codes=[]),
    # ── 第 8 步第二輪 review P1：價格與 RR 要能重算、三方一致 ──
    lambda r: r["rr_context"].update(target_price=None),                # target_known 仍宣稱 true
    lambda r: r["rr_context"].update(executable_rr=1.0),                # context 低於門檻、gate 仍宣稱 2.0
    lambda r: r["rr_context"].update(target_price=101.0),               # 100／98／101 實際 RR 0.5，兩邊仍宣稱 2.0
    lambda r: r["rr_context"].update(stop_price=None),
    lambda r: r["rr_context"].update(stop_price=100.0),                 # 風險 = 0
    lambda r: r["rr_gate"].update(minimum_rr=0.4),                      # 門檻被改低：要從 primary_zone 重算
    lambda r: r.update(primary_zone=None),
    # ── 第 8 步第二輪 review P2：semantic 許可必須真的被封頂到試單 ──
    lambda r: r.update(semantic_entry_permission_state="ENTRY_ALLOWED"),
], ids=["gate_failed", "zone_statistic_target_unknown", "executable_missing", "target_unknown",
        "below_minimum", "final_reason_missing", "semantic_reason_missing",
        "target_price_missing", "context_rr_below_minimum", "prices_give_lower_rr", "stop_missing",
        "zero_risk", "minimum_tampered", "primary_zone_missing", "semantic_not_capped"])
def test_e2e_g_new_must_meet_d3_preconditions(tmp_path, mutate):
    a, b = _passing_pair()
    g_new = next(r for r in b if r.get("final_entry_state") == "PROBE_ALLOWED" and not r["setup_rr_qualified"])
    mutate(g_new)
    report = _run(tmp_path, a, b)
    assert report["invariants"]["g_new_precondition"] == 1, report["anomaly_examples"]
    assert report["verdict"] == "INVARIANT_VIOLATION"


def test_e2e_final_above_semantic_is_violation(tmp_path):
    """final 只會再套更多 gate：final 的許可⛔ 不得高於 semantic（任何組都一樣）。"""
    a, b = _passing_pair()
    p_row = next(r for r in b if r.get("final_entry_state") == "PROBE_ALLOWED" and r["setup_rr_qualified"])
    a_row = next(r for r in a if r["as_of"] == p_row["as_of"] and r["symbol"] == p_row["symbol"])
    for row in (a_row, p_row):
        row["semantic_entry_permission_state"] = "WAIT_CONFIRMATION"
    report = _run(tmp_path, a, b)
    assert report["invariants"]["final_above_semantic"] == 1
    assert report["verdict"] == "INVARIANT_VIOLATION"


def test_e2e_setup_fail_rows_never_exceed_probe_anywhere(tmp_path):
    """setup 不合格的列：semantic ⛔ 不得高於試單、action ⛔ 不得是 Buy（不只 G_new）。"""
    a, b = _passing_pair()
    a.append(_row("100", 30, setup_ok=False, final="BLOCKED", side="A"))
    b.append(_row("100", 30, setup_ok=False, final="WAIT_CONFIRMATION", side="B", semantic="ENTRY_ALLOWED"))
    report = _run(tmp_path, a, b)
    assert report["invariants"]["setup_fail_above_probe"] == 1
    assert report["verdict"] == "INVARIANT_VIOLATION"


def test_e2e_setup_qualified_must_match_diagnostic(tmp_path):
    """`rr_gate.setup_qualified` 與 validation-only 的 `setup_rr_qualified` 出自同一顆 setup gate，必須一致。"""
    a, b = _passing_pair()
    p_row = next(r for r in b if r.get("final_entry_state") == "PROBE_ALLOWED" and r["setup_rr_qualified"])
    p_row["rr_gate"]["setup_qualified"] = False
    report = _run(tmp_path, a, b)
    assert report["invariants"]["setup_qualified_mismatch"] == 1
    assert report["verdict"] == "INVARIANT_VIOLATION"


# ── 結果標籤（第 8 步 review P2）：跨版本必須相同、而且要對得上 bundle ──────────

def test_e2e_label_mismatch_across_versions_is_unattributable(tmp_path):
    """同一批 G_new，A 的 forward_return −20%、B +1%：⛔ 不能直接採用 B 的值。"""
    a, b = _passing_pair()
    for r in a:
        if r.get("final_entry_state") == "BLOCKED":
            r["forward_return"] = -0.20
    report = _run(tmp_path, a, b)
    assert report["verdict"] == "UNATTRIBUTABLE"
    assert any("結果標籤" in reason for reason in report["verdict_reasons"])


def test_e2e_label_not_matching_bundle_is_unattributable(tmp_path):
    """兩邊一致但與 bundle 的 candles 不符（例如兩邊都被改過）→ 同樣⛔ 不出結論。"""
    a, b = _passing_pair()
    for rows in (a, b):
        for r in rows:
            if r.get("decision_fields_available"):
                r["forward_return"] = 0.05
    report = _run(tmp_path, a, b)
    assert report["verdict"] == "UNATTRIBUTABLE"
    assert any("bundle" in reason for reason in report["verdict_reasons"])


# ── U 組（v5 的連帶差異）與其他不變式 ─────────────────────────────────────────

def test_e2e_u_group_allowed_gate_consequences(tmp_path):
    """U 組、B 的主 gate 不合格：只改了允許的欄位 → ⛔ 不算異常。"""
    a, b = _passing_pair()
    a.append(_row("100", 30, setup_ok=True, final="WAIT_CONFIRMATION", side="A", semantic="PROBE_ALLOWED",
                  semantic_codes=("CLOSE_RECLAIM",), path="WAIT_PRICE_FOLLOW_THROUGH",
                  final_codes=("ENTRY_STATE_PROBE", "WAIT_PRICE_FOLLOW_THROUGH", "EXECUTION_RR_INSUFFICIENT"),
                  position_codes=("PRIMARY_SUPPORT", "WAIT_PRICE_FOLLOW_THROUGH"),
                  gate_ok=False, gate_reason="EXECUTION_RR_INSUFFICIENT"))
    b.append(_row("100", 30, setup_ok=True, final="WAIT_CONFIRMATION", side="B", semantic="WAIT_CONFIRMATION",
                  semantic_codes=("CLOSE_RECLAIM", "EXECUTION_RR_INSUFFICIENT"), path="RR_BLOCKED",
                  final_codes=("ENTRY_STATE_PROBE", "EXECUTION_RR_INSUFFICIENT", "WAIT_PRICE_FOLLOW_THROUGH"),
                  position_codes=("PRIMARY_SUPPORT",), gate_ok=False, gate_reason="EXECUTION_RR_INSUFFICIENT"))
    report = _run(tmp_path, a, b)
    assert report["invariants"]["u_field_changed_rows"] == 0, report["anomaly_examples"]
    assert report["verdict"] == "PASS"


def test_e2e_u_group_v6_full_entry_reason_may_disappear(tmp_path):
    """v6（第 9 步正式量測的 7 列）：主 gate 與 secondary 都不合格時，A 的 semantic 只套了 secondary（PROBE＋
    `RR_BELOW_FULL_ENTRY`），B 先被主 gate 封頂到 WAIT、secondary 已經沒有東西可降 → reason 裡少了 `RR_BELOW_FULL_ENTRY`。"""
    a, b = _passing_pair()
    a.append(_row("100", 30, setup_ok=True, final="WAIT_CONFIRMATION", side="A", semantic="PROBE_ALLOWED",
                  semantic_codes=("CLOSE_RECLAIM", "RR_BELOW_FULL_ENTRY"), path="BLOCKING_ZONE_AHEAD",
                  final_codes=("PRICE_TOO_FAR_FROM_ZONE", "CLOSE_RECLAIM", "RR_BELOW_FULL_ENTRY", "EXECUTION_RR_INSUFFICIENT"),
                  gate_ok=False, gate_reason="EXECUTION_RR_INSUFFICIENT"))
    b.append(_row("100", 30, setup_ok=True, final="WAIT_CONFIRMATION", side="B", semantic="WAIT_CONFIRMATION",
                  semantic_codes=("CLOSE_RECLAIM", "EXECUTION_RR_INSUFFICIENT"), path="RR_BLOCKED",
                  final_codes=("PRICE_TOO_FAR_FROM_ZONE", "CLOSE_RECLAIM", "EXECUTION_RR_INSUFFICIENT"),
                  gate_ok=False, gate_reason="EXECUTION_RR_INSUFFICIENT"))
    report = _run(tmp_path, a, b)
    assert report["invariants"]["u_field_changed_rows"] == 0, report["anomaly_examples"]


@pytest.mark.parametrize("a_codes, b_codes, gate_ok", [
    (("CLOSE_RECLAIM", "RR_BELOW_FULL_ENTRY"), ("CLOSE_RECLAIM",), True),        # gate 合格時⛔ 不允許任何變化
    (("CLOSE_RECLAIM", "PRICE_UPSIDE_FOLLOW_THROUGH"), ("CLOSE_RECLAIM",), False),  # 少掉的⛔ 不是 RR_BELOW_FULL_ENTRY
    (("CLOSE_RECLAIM", "RR_INSUFFICIENT"), ("CLOSE_RECLAIM",), False),           # 少掉的是其他 RR code 也⛔ 不行
])
def test_e2e_u_group_v6_disallowed_disappearance(tmp_path, a_codes, b_codes, gate_ok):
    a, b = _passing_pair()
    reason = "RR_QUALIFIED" if gate_ok else "EXECUTION_RR_INSUFFICIENT"
    a.append(_row("100", 30, setup_ok=True, final="WAIT_CONFIRMATION", side="A", semantic_codes=a_codes,
                  gate_ok=gate_ok, gate_reason=reason))
    b.append(_row("100", 30, setup_ok=True, final="WAIT_CONFIRMATION", side="B", semantic_codes=b_codes,
                  gate_ok=gate_ok, gate_reason=reason))
    report = _run(tmp_path, a, b)
    assert report["u_changed_fields"].get("semantic_reason_codes") == 1


@pytest.mark.parametrize("change", [
    {"final_entry_state": "PROBE_ALLOWED"},             # 必須完全相同的欄位
    {"market_action": "BUY_SMALL"},
    {"risk_notes_digest": "fedcba9876543210"},
    {"semantic_reason_codes": ["CLOSE_RECLAIM", "SOMETHING_ELSE"]},   # 多出的⛔ 不是 RR 類
    {"path_gate_state": "BLOCKING_ZONE_AHEAD"},         # 只能變成 RR_BLOCKED
    {"event_signal": "SUPPORT_TEST"},                   # 其他任何欄位
])
def test_e2e_u_group_disallowed_change_is_invariant_violation(tmp_path, change):
    a, b = _passing_pair()
    a.append(_row("100", 30, setup_ok=True, final="WAIT_CONFIRMATION", side="A", semantic_codes=("CLOSE_RECLAIM",),
                  gate_ok=False, gate_reason="EXECUTION_RR_INSUFFICIENT"))
    b_row = _row("100", 30, setup_ok=True, final="WAIT_CONFIRMATION", side="B", semantic_codes=("CLOSE_RECLAIM",),
                 gate_ok=False, gate_reason="EXECUTION_RR_INSUFFICIENT")
    b_row.update(change)
    b.append(b_row)
    report = _run(tmp_path, a, b)
    assert report["verdict"] == "INVARIANT_VIOLATION"
    assert report["invariants"]["u_field_changed_rows"] == 1


def test_e2e_u_group_rr_gate_may_only_add_setup_qualified(tmp_path):
    a, b = _passing_pair()
    a.append(_row("100", 30, setup_ok=True, final="WAIT_CONFIRMATION", side="A"))
    b_row = _row("100", 30, setup_ok=True, final="WAIT_CONFIRMATION", side="B")
    b_row["rr_gate"] = b_row["rr_gate"] | {"target_known": True}
    b.append(b_row)
    report = _run(tmp_path, a, b)
    assert report["u_changed_fields"] == {"rr_gate": 1}


@pytest.mark.parametrize("a_kw, b_kw, invariant", [
    ({"final": "PROBE_ALLOWED"}, {"final": "ENTRY_ALLOWED"}, "upgrades"),
    ({"final": "ENTRY_ALLOWED", "action": "BuySmall"}, {"final": "ENTRY_ALLOWED", "action": "Buy"}, "upgrades"),
    ({"final": "ENTRY_ALLOWED"}, {"final": "WAIT_CONFIRMATION"}, "tightened"),
])
def test_e2e_upgrade_and_tighten_must_be_zero(tmp_path, a_kw, b_kw, invariant):
    a, b = _passing_pair()
    a.append(_row("100", 30, setup_ok=True, side="A", **a_kw))
    b.append(_row("100", 30, setup_ok=True, side="B", **b_kw))
    report = _run(tmp_path, a, b)
    assert report["verdict"] == "INVARIANT_VIOLATION"
    assert report["invariants"][invariant] == 1


def test_e2e_setup_missing_must_not_be_released(tmp_path):
    """D2：setup RR 缺值的列⛔ 不得放行。"""
    a, b = _passing_pair()
    a.append(_row("100", 30, setup_ok=False, final="BLOCKED", side="A", setup_rr=None))
    b.append(_row("100", 30, setup_ok=False, final="PROBE_ALLOWED", side="B", setup_rr=None, d3=True))
    report = _run(tmp_path, a, b)
    assert report["invariants"]["setup_missing_released"] == 1
    assert report["verdict"] == "INVARIANT_VIOLATION"


def test_e2e_blocked_with_rr_reason_is_violation(tmp_path):
    """D1：BLOCKED 只留給結構事實，理由⛔ 不得是 RR 類。"""
    a, b = _passing_pair()
    a.append(_row("100", 30, setup_ok=False, final="BLOCKED", side="A"))
    b.append(_row("100", 30, setup_ok=False, final="BLOCKED", side="B", final_codes=("RR_INSUFFICIENT",)))
    report = _run(tmp_path, a, b)
    assert report["invariants"]["blocked_with_rr_reason"] == 1
    assert report["verdict"] == "INVARIANT_VIOLATION"


def test_e2e_setup_fail_to_entry_is_violation(tmp_path):
    """setup 不合格最多試單（D3）：B 是 ENTRY 就是程式錯誤。"""
    a, b = _passing_pair()
    a.append(_row("100", 30, setup_ok=False, final="BLOCKED", side="A"))
    b.append(_row("100", 30, setup_ok=False, final="ENTRY_ALLOWED", side="B"))
    assert _run(tmp_path, a, b)["invariants"]["setup_fail_unexpected_state"] == 1


def test_e2e_measurement_field_missing_aborts(tmp_path):
    """欄位檢查跟 smoke 相同：任何一列不符就⛔ 不出結論。"""
    a, b = _passing_pair()
    del b[0]["risk_notes_digest"]
    with pytest.raises(ArtifactError, match="risk_notes_digest"):
        _run(tmp_path, a, b)


def test_stop_touch_denominator_keeps_rows_without_stop():
    """停損觸及率的分母是**已成交的列**（計畫書原定義）：stop 缺值的列算「沒碰到」、⛔ 不排除——
    排除會拉高對照組 P 的觸及率，等於放寬 G_new 的容許值（第 8 步 review）。"""
    no_stop = simulate_trade(MARKET, 105.0, None, None, _bars(*[(105.0, 106.0, 104.0, 105.0)] * 5))
    stopped = simulate_trade(MARKET, 105.0, 98.0, None, _bars((105.0, 106.0, 97.0, 98.0), *[(98.0, 99.0, 97.0, 98.0)] * 4))
    metrics = outcome_metrics([no_stop, stopped], [])
    assert metrics["stop_touch_n"] == 2 and metrics["stop_touch_rate"] == 0.5
    assert metrics["filled_without_stop"] == 1
    assert metrics["realized_r_n"] == 1, "實現 R 仍只取 stop 有值的列（計畫書原定義）"


# ── CLI ───────────────────────────────────────────────────────────────────────

def _cli():
    path = Path(__file__).resolve().parents[4] / "scripts" / "compare-i129-replay.py"
    spec = importlib.util.spec_from_file_location("compare_i129_replay", path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def _fake_bundle(monkeypatch):
    import types

    from .. import replay_bundle

    monkeypatch.setattr(replay_bundle, "load_bundle",
                        lambda _p: types.SimpleNamespace(bundle_id=BUNDLE_ID, candles=_candle_rows()))


def test_cli_exit_codes(tmp_path, monkeypatch):
    _fake_bundle(monkeypatch)
    cli = _cli()
    a, b = _passing_pair()
    a_path, b_path = _write(tmp_path, "A", a), _write(tmp_path, "B", b)
    out = tmp_path / "report.json"
    base = ["--bundle", str(tmp_path), "--a", a_path, "--b", b_path, "--a-commit", A_COMMIT, "--b-commit", B_COMMIT]
    assert cli.main(base + ["--output", str(out)]) == 0
    assert out.exists()
    assert cli.main(base + ["--output", str(out)]) == 5, "⛔ 不覆寫既有報告"
    # 預期的 commit 不符 → 無法歸因（5）
    wrong = ["--bundle", str(tmp_path), "--a", a_path, "--b", b_path, "--a-commit", B_COMMIT, "--b-commit", A_COMMIT]
    assert cli.main(wrong + ["--output", str(tmp_path / "r1.json")]) == 5
    # 欄位缺失 → 讀檔驗證失敗 → 5（⛔ 不出結論、⛔ 不留報告）
    del b[0]["risk_notes_digest"]
    bad_b = _write(tmp_path, "B_bad", b, commit=B_COMMIT)
    args = ["--bundle", str(tmp_path), "--a", a_path, "--b", bad_b, "--a-commit", A_COMMIT, "--b-commit", B_COMMIT]
    assert cli.main(args + ["--output", str(tmp_path / "r2.json")]) == 5
    assert not (tmp_path / "r2.json").exists()


@pytest.mark.parametrize("bad", ["abc", "A" * 40, "a" * 39])
def test_cli_rejects_malformed_commit(tmp_path, monkeypatch, bad):
    _fake_bundle(monkeypatch)
    cli = _cli()
    a, b = _passing_pair()
    args = ["--bundle", str(tmp_path), "--a", _write(tmp_path, "A", a), "--b", _write(tmp_path, "B", b),
            "--a-commit", bad, "--b-commit", B_COMMIT, "--output", str(tmp_path / "r.json")]
    assert cli.main(args) == 5
