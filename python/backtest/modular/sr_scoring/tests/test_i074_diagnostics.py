"""I-074 Stage 0 的診斷欄位：產出、透傳、fallback 與 validator。

對應 `docs/issue.md` I-074 Stage 0 的十、測試與驗證策略
（v16 定的原始策略 ＋ v17 的 review 修正）。
⚠️ **護欄的非回歸由既有測試負責**（`test_lifecycle_engine.py` 的 15 條逐鍵斷言、
`test_decision_engine.py` 的既有斷言全部不修改），這一檔只加新增行為。
"""
from __future__ import annotations

import pytest

from ..lifecycle_engine import resolve_lifecycle
from ..replay_bundle import validate_diagnostics
from ..replay_bundle.artifacts import ArtifactError
from ..replay_bundle import DIAGNOSTIC_FIELDS, DIAGNOSTIC_NO_ZONE_FALLBACK
from ..types import ZoneType
# ⚠️ 重用既有的 ZoneScore 建構 helper，⛔ 不自己拼一份——ZoneScore 的必填欄位會長，
# 各測試檔各拼一份的話，下次加欄位就要改好幾個地方（`test_lifecycle_engine.py` 也是這樣用的）。
from .test_decision_engine import _zone as _base_zone


def _zone(low=98.0, high=100.0, role=ZoneType.SUPPORT.value):
    return _base_zone(role=role, low=low, high=high)


def _lifecycle(*, follow_through, momentum, current_price, zone=None, signal_states=None):
    return resolve_lifecycle(
        event_state_summary={"active": signal_states or [], "active_bearish_events": []},
        primary_zone=zone if zone is not None else _zone(),
        structure_state="SUPPORT_RECLAIM_CONFIRMED",
        daily_price_action={
            "price_follow_through_state": follow_through,
            "momentum_confirmation_state": momentum,
        },
        current_price=current_price,
    )


# ── lifecycle 層 ────────────────────────────────────────────────────────────

def test_clear_zone_breakout_true_and_false():
    """`clear_zone_breakout` 原本是區域變數、從不回傳——現在要帶得出來。"""
    above = _lifecycle(follow_through="X", momentum="Y", current_price=150.0)
    below = _lifecycle(follow_through="X", momentum="Y", current_price=95.0)
    assert above["clear_zone_breakout"] is True
    assert below["clear_zone_breakout"] is False


@pytest.mark.parametrize("follow_through,momentum,price,expected", [
    ("PRICE_UPSIDE_FOLLOW_THROUGH", "MOMENTUM_CONFIRMED", 200.0, True),
    ("NO_FOLLOW_THROUGH", "MOMENTUM_CONFIRMED", 200.0, False),          # 缺第一項
    ("PRICE_UPSIDE_FOLLOW_THROUGH", "MOMENTUM_WEAK", 200.0, False),     # 缺第二項
    ("PRICE_UPSIDE_FOLLOW_THROUGH", "MOMENTUM_CONFIRMED", 100.0, False),  # 缺突破
])
def test_continuation_price_evidence_met_needs_all_three(follow_through, momentum, price, expected):
    """三項價格證據**逐項缺一**都不成立。⚠️ 它是診斷用，⛔ 不是 candidate 的定義。"""
    result = _lifecycle(follow_through=follow_through, momentum=momentum, current_price=price)
    assert result["continuation_price_evidence_met"] is expected


def test_lifecycle_diagnostics_are_strict_bool():
    result = _lifecycle(follow_through="X", momentum="Y", current_price=150.0)
    for field in ("clear_zone_breakout", "continuation_price_evidence_met"):
        assert isinstance(result[field], bool)


# ── no-zone fallback ────────────────────────────────────────────────────────

def test_fallback_covers_exactly_the_nine_fields():
    assert set(DIAGNOSTIC_FIELDS) == set(DIAGNOSTIC_NO_ZONE_FALLBACK)
    assert len(DIAGNOSTIC_FIELDS) == 9


def test_fallback_values_are_the_agreed_ones():
    """⚠️ `rr_decoupling_candidate=False` 是**真實的 false**，⛔ 不是「不知道」。"""
    assert DIAGNOSTIC_NO_ZONE_FALLBACK == {
        "clear_zone_breakout": False,
        "continuation_price_evidence_met": False,
        "setup_rr_qualified": False,
        "rr_decoupling_candidate": False,
        "event_signal": "NO_EVENT",
        "structure_state": "UNKNOWN",
        "action_state": "WATCH",
        "position_action_condition": None,
        "position_action": None,
    }


# ── validator ───────────────────────────────────────────────────────────────

def _row(**overrides):
    row = {
        "symbol": "2330", "timeframe": "1d", "as_of": "2026-09-01T00:00:00+00:00",
        "lifecycle_phase": "TESTING",
        "clear_zone_breakout": False,
        "continuation_price_evidence_met": False,
        "setup_rr_qualified": False,
        "rr_decoupling_candidate": False,
        "event_signal": "CLOSE_RECLAIM",
        "structure_state": "SUPPORT_RECLAIM_CONFIRMED",
        "action_state": "HOLD",
        "position_action_condition": {"state": "HOLD", "structure_state": "SUPPORT_RECLAIM_CONFIRMED"},
        "position_action": "HOLD",
    }
    row.update(overrides)
    return row


def _no_zone_row(**overrides):
    row = _row(zone_score_available=False, zone_score_error="NO_ZONE_SCORES",
               lifecycle_phase=None, position_action_condition=None, position_action=None,
               event_signal="NO_EVENT", structure_state="UNKNOWN", action_state="WATCH")
    row.update(overrides)
    return row


def test_valid_row_passes():
    validate_diagnostics([_row()], "t")


def test_no_zone_fallback_row_passes():
    validate_diagnostics([_no_zone_row()], "t")


@pytest.mark.parametrize("field", ["clear_zone_breakout", "continuation_price_evidence_met",
                                   "setup_rr_qualified", "rr_decoupling_candidate"])
@pytest.mark.parametrize("bad", [0, 1, "true", None])
def test_bool_fields_reject_non_bool(field, bad):
    """⛔ bool 是 int 的子類，鬆一點就會放行 `1`。"""
    with pytest.raises(ArtifactError):
        validate_diagnostics([_row(**{field: bad})], "t")


@pytest.mark.parametrize("field", ["event_signal", "structure_state", "action_state"])
@pytest.mark.parametrize("bad", ["", None, 123])
def test_str_fields_reject_empty_or_wrong_type(field, bad):
    with pytest.raises(ArtifactError):
        validate_diagnostics([_row(**{field: bad})], "t")


@pytest.mark.parametrize("field", list(DIAGNOSTIC_NO_ZONE_FALLBACK))
def test_missing_any_field_is_rejected(field):
    row = _row()
    del row[field]
    with pytest.raises(ArtifactError):
        validate_diagnostics([row], "t")


def test_null_condition_only_allowed_for_no_zone_rows():
    """⚠️ 一般列的 `position_action_condition` ⛔ 不得為 null。"""
    with pytest.raises(ArtifactError) as exc:
        validate_diagnostics([_row(position_action_condition=None)], "t")
    assert "不是** no-zone" in str(exc.value)


def test_cross_field_consistency_structure_state():
    bad = _row(structure_state="BREAKDOWN")   # 與 condition.structure_state 不一致
    with pytest.raises(ArtifactError) as exc:
        validate_diagnostics([bad], "t")
    assert "structure_state" in str(exc.value) and "不一致" in str(exc.value)


def test_cross_field_consistency_action_state():
    bad = _row(action_state="WATCH")   # condition.state 是 HOLD
    with pytest.raises(ArtifactError) as exc:
        validate_diagnostics([bad], "t")
    assert "action_state" in str(exc.value) and "不一致" in str(exc.value)


# ── lifecycle_phase 依賴 ────────────────────────────────────────────────────

def test_missing_lifecycle_phase_is_rejected_even_when_candidate_is_false():
    """⚠️ 這正是少了這道守門會漏掉的情況：

    `false == (None == "CONTINUATION" and …)` → `false == false` → 等價式**照樣通過**。
    """
    row = _row(rr_decoupling_candidate=False)
    del row["lifecycle_phase"]
    with pytest.raises(ArtifactError) as exc:
        validate_diagnostics([row], "t")
    assert "lifecycle_phase" in str(exc.value)


def test_null_lifecycle_phase_rejected_on_normal_row():
    with pytest.raises(ArtifactError):
        validate_diagnostics([_row(lifecycle_phase=None)], "t")


def test_no_zone_row_must_have_null_lifecycle_phase():
    with pytest.raises(ArtifactError):
        validate_diagnostics([_no_zone_row(lifecycle_phase="TESTING")], "t")


# ── candidate 等價式 ────────────────────────────────────────────────────────

@pytest.mark.parametrize("phase,setup_rr,expected", [
    ("CONTINUATION", False, True),
    ("CONTINUATION", True, False),
    ("TESTING", False, False),
    ("TESTING", True, False),
])
def test_candidate_equivalence_truth_table(phase, setup_rr, expected):
    row = _row(lifecycle_phase=phase, setup_rr_qualified=setup_rr,
               rr_decoupling_candidate=expected)
    validate_diagnostics([row], "t", side="after")


def test_candidate_not_matching_equivalence_is_rejected():
    row = _row(lifecycle_phase="CONTINUATION", setup_rr_qualified=False,
               rr_decoupling_candidate=False)   # 應該是 True
    with pytest.raises(ArtifactError) as exc:
        validate_diagnostics([row], "t", side="after")
    assert "等價式" in str(exc.value)


def test_before_side_does_not_apply_the_after_equivalence():
    """⛔ before 版沒有 `lifecycle_engine.py`，`lifecycle_phase` 本來就不會是 CONTINUATION。

    套 after 的等價式會**全部判成不符**——所以 before 側只驗 schema 與交叉一致性。
    """
    row = _row(lifecycle_phase="TESTING", setup_rr_qualified=False,
               rr_decoupling_candidate=True)   # 在 after 側會被判不符
    with pytest.raises(ArtifactError):
        validate_diagnostics([row], "t", side="after")
    validate_diagnostics([row], "t", side="before")   # ⛔ before 不驗等價式


# ── primary zone 來源切換 ───────────────────────────────────────────────────

def test_decision_summary_zone_carries_relative_volume():
    """⚠️ ⛔ 不補的話，切換來源後 `_volume_strength_bucket()` 會**靜默退化**成 unavailable。"""
    from ..decision_engine import _decision_summary_zone

    zone = _zone()
    summary = _decision_summary_zone(zone, current_price=99.0, reason="TEST")
    assert "relative_volume" in summary
    assert summary["relative_volume"] == zone.relative_volume


def test_replay_row_uses_decision_primary_for_all_three_consumers(monkeypatch):
    """replay row、daily_confirmation_context、daily_confirmation_outcome **共用同一顆**。

    ⛔ 只切一處會讓同一列混用兩顆 zone（排序第一筆 vs decision primary）。
    """
    from .. import evaluation as ev

    seen: dict[str, object] = {}
    real_ctx = ev._daily_confirmation_context
    real_outcome = ev._daily_confirmation_outcome

    def spy_ctx(primary_zone, *a, **kw):
        seen["context"] = primary_zone
        return real_ctx(primary_zone, *a, **kw)

    def spy_outcome(*a, **kw):
        # primary_zone 是第 4 個位置參數（df, idx, current_price, primary_zone, …）
        seen["outcome"] = a[3] if len(a) > 3 else kw.get("primary_zone")
        return real_outcome(*a, **kw)

    monkeypatch.setattr(ev, "_daily_confirmation_context", spy_ctx)
    monkeypatch.setattr(ev, "_daily_confirmation_outcome", spy_outcome)

    from .conftest import bullish_trend_df
    from ..dataset import DatasetConfig
    from ..model import ModelBundle
    from .test_evaluation import _FixedProbabilityModel

    df = bullish_trend_df(n=120)
    bundle = ModelBundle(
        hold_model=_FixedProbabilityModel(0.62), break_model=_FixedProbabilityModel(0.28),
        feature_names=[], trained_at="2026-07-01T00:00:00+00:00",
        version="v-test", config_hash="hash123",
    )
    config = DatasetConfig(min_history_bars=80, forward_bars_support=5, forward_bars_resistance=5)
    rows = ev._decision_replay_rows([("2330", "1d", df)], config, {"2330": 3}, bundle=bundle)

    scored = [r for r in rows if r.get("zone_score_available")]
    if not scored:
        pytest.skip("這組合成資料沒有產生 zone——identity 斷言需要至少一列有 zone")
    row = scored[-1]
    # 三處必須是**同一顆**。
    assert seen.get("context") is not None
    assert row["primary_zone"] == seen["context"] == seen["outcome"]
    # 且它是 decision primary 的形狀（⛔ 不是排序第一筆的 8 欄投影）。
    assert "decision_role" in row["primary_zone"]
    assert "relative_volume" in row["primary_zone"]


# ── producer 層：⚠️ 直接呼叫 `_decision_semantic_pipeline()` ────────────────
#
# ⛔ 先前只測「手工建的 row 能不能通過 validator」——那**沒有驗到產生端**。
# validator 再嚴，producer 算錯一樣會產出一致但錯誤的資料。

def _pipeline(*, lifecycle_phase_wanted, rr_qualified, zone=None):
    """驅動真正的 semantic pipeline。

    `lifecycle_phase` 由 lifecycle engine 決定，所以用價格證據去驅動它，
    ⛔ 不直接塞值——那樣測到的又是假的。
    """
    from ..decision_engine import _decision_semantic_pipeline

    want_continuation = lifecycle_phase_wanted == "CONTINUATION"
    return _decision_semantic_pipeline(
        regime={"structure_state": "SUPPORT_RECLAIM_CONFIRMED"},
        primary_zone=zone if zone is not None else _zone(),
        market_action="HOLD",
        event_state_summary={
            "active": [{"type": "INTRADAY_RECLAIM", "age_bars": 0}] if want_continuation else [],
            "active_bearish_events": [],
            "candidates": [{"type": "CLOSE_RECLAIM"}] if want_continuation else [],
        },
        daily_price_action={
            "price_follow_through_state":
                "PRICE_UPSIDE_FOLLOW_THROUGH" if want_continuation else "NO_FOLLOW_THROUGH",
            "momentum_confirmation_state":
                "MOMENTUM_CONFIRMED" if want_continuation else "MOMENTUM_WEAK",
        },
        rr_gate={"qualified": rr_qualified},
        structure_state="SUPPORT_RECLAIM_CONFIRMED",
        blocking_zone_ahead=False,
        current_price=150.0 if want_continuation else 99.0,
    )


def test_pipeline_passes_through_lifecycle_keys_unchanged(monkeypatch):
    """**pass-through identity**：lifecycle 回傳什麼，semantic pipeline 就有什麼。

    ⚠️ **用 sentinel，⛔ 不用「同一組輸入跑兩次」比對**：兩次都餵相同輸入時，
    pipeline 就算**自己重算同一條公式**也會得到相同答案，這條測試照樣綠——
    那時它證明的是「兩邊公式一致」，⛔ 不是「有透傳」。而重算正是會重蹈偽陽性的做法。

    改法是讓 lifecycle 回傳一組**與輸入推導相反**的值：透傳才拿得到它，
    重算一定拿到相反的結果。
    """
    from .. import decision_engine

    # 先確認這組輸入**真的**會推導出 True／True（否則 sentinel 的「相反」沒有意義，
    # 測試會退化成兩邊都是 False 的空轉）。
    real = _lifecycle(follow_through="PRICE_UPSIDE_FOLLOW_THROUGH",
                      momentum="MOMENTUM_CONFIRMED", current_price=150.0)
    assert real["clear_zone_breakout"] is True
    assert real["continuation_price_evidence_met"] is True

    sentinel = dict(real, lifecycle_phase="CONTINUATION",
                    clear_zone_breakout=False, continuation_price_evidence_met=False)
    monkeypatch.setattr(decision_engine, "resolve_lifecycle", lambda **kwargs: sentinel)

    pipeline = _pipeline(lifecycle_phase_wanted="CONTINUATION", rr_qualified=False)

    for field in ("clear_zone_breakout", "continuation_price_evidence_met"):
        assert pipeline[field] is False, (
            f"{field} 是 True——semantic pipeline 自行重算了，⛔ 沒有透傳 lifecycle 的結果"
        )
        assert isinstance(pipeline[field], bool)


@pytest.mark.parametrize("phase,rr_qualified,expected", [
    ("CONTINUATION", False, True),
    ("CONTINUATION", True, False),
    ("TESTING", False, False),
    ("TESTING", True, False),
])
def test_pipeline_candidate_truth_table(phase, rr_qualified, expected):
    """⚠️ 這次是驗**產生端**的四格真值表。"""
    pipeline = _pipeline(lifecycle_phase_wanted=phase, rr_qualified=rr_qualified)
    assert pipeline["rr_decoupling_candidate"] is expected
    # 等價式必須成立（⛔ 不得有第三個條件）
    assert pipeline["rr_decoupling_candidate"] is (
        pipeline["lifecycle_phase"] == "CONTINUATION" and not pipeline["setup_rr_qualified"]
    )


def test_pipeline_setup_rr_qualified_mirrors_the_setup_gate():
    for qualified in (True, False):
        pipeline = _pipeline(lifecycle_phase_wanted="TESTING", rr_qualified=qualified)
        assert pipeline["setup_rr_qualified"] is qualified


def test_setup_rr_qualified_can_differ_from_the_public_rr_gate():
    """⚠️ **實際對照**：`setup_rr_qualified` 與對外 `rr_gate.qualified` 是兩顆。

    對外那顆會被 `_execution_rr_gate()` 覆寫，所以同一列可能不同值——
    ⛔ 拿對外那顆去驗 candidate 的等價式會得到矛盾。
    """
    from ..decision_engine import _execution_rr_gate

    setup_gate = {"qualified": True, "actual_rr": 3.0, "reason_code": None}
    # ⚠️ key 是 **`executable_rr`**（⛔ 不是 `execution_rr`）——寫錯的話會落進
    # 「target 未知」分支而沿用 setup gate，測試就變成什麼都沒驗到。
    # 這裡給一個遠低於任何門檻的值，讓 execution gate 把 qualified 降下來。
    executed = _execution_rr_gate(
        _zone(), "ENTRY_ALLOWED",
        {"executable_rr": 0.05, "price_basis": "CURRENT_PRICE"},
        dict(setup_gate),
    )
    # setup 說可以、execution 說不行——**同一列、兩顆不同的答案**。
    assert setup_gate["qualified"] is True
    assert executed["qualified"] is False
    # ⛔ 所以拿對外那顆去驗 candidate 的等價式會得到矛盾。
    assert executed["qualified"] is not setup_gate["qualified"]


# ⚠️ **九欄全部逐欄驗**，而且每個案例**只動目標欄位**。
#
# ⛔ 舊版對三個字串欄位會**一併改 `lifecycle_phase`**，於是實際攔下它的是 no-zone 的
# 「phase 必須是 null」規則，⛔ **沒有證明字串欄位本身會被拒**。那是假綠的一種：
# 測試名字說驗 A，實際驗到的是 B。
#
# 壞值全部挑**型別與結構都合法**的：這樣唯一能擋下它的就是固定 mapping 對照本身。
#   * `position_action_condition` 給一個結構合法、且與 `structure_state`／`action_state`
#     **交叉一致**的 object——否則會先被交叉一致性檢查擋下，又變成驗到別的規則；
#   * `position_action` 給合法的非空字串。
#
# ⚠️ `lifecycle_phase` ⛔ **不在這九欄裡**，它由獨立的 null 規則測試涵蓋。
@pytest.mark.parametrize("field,bad", [
    ("clear_zone_breakout", True),
    ("continuation_price_evidence_met", True),
    ("setup_rr_qualified", True),
    ("rr_decoupling_candidate", True),
    ("event_signal", "CLOSE_RECLAIM"),
    ("structure_state", "SUPPORT_RECLAIM_CONFIRMED"),
    ("action_state", "HOLD"),
    ("position_action_condition", {"state": "WATCH", "structure_state": "UNKNOWN"}),
    ("position_action", "HOLD"),
])
def test_tampered_no_zone_fallback_values_are_rejected(field, bad):
    """⚠️ no-zone 列的固定值被竄改也要被抓到。

    ⛔ 「它是 fallback」不等於「它可以是任何值」——那九個值**整組**都是契約。

    ⚠️ 用 `side="before"` 跑：before 側⛔ 不驗等價式，所以能證明擋下 candidate 的是
    **no-zone 的固定 mapping**，而不是 after 的等價式順手擋掉的。那正是舊版漏掉的縫隙
    ——`rr_decoupling_candidate=true` 在 before 側原本會直接放行。
    """
    row = _no_zone_row(**{field: bad})

    with pytest.raises(ArtifactError) as exc:
        validate_diagnostics([row], "t", side="before")

    message = str(exc.value)
    # ⛔ 失敗原因必須就是「no-zone 固定值不符」而且指名該欄位，
    # 否則這條測試又會在「被別的規則攔下」時假綠。
    assert "no-zone fallback 列" in message, message
    assert field in message, message


def test_no_zone_candidate_is_rejected_by_the_mapping_not_the_equivalence():
    """⚠️ 對照組：after 側的 `rr_decoupling_candidate=true` 也要由**固定 mapping** 擋下。

    after 側的等價式（`lifecycle_phase == "CONTINUATION" and not setup_rr_qualified`）
    在 no-zone 列上算出 false，本來就會擋下 true——但那是**間接**的。
    ⛔ 如果哪天等價式改了或順序換了，這一列就會靜默放行；訊息驗的是 mapping 這一道。
    """
    row = _no_zone_row(rr_decoupling_candidate=True)

    with pytest.raises(ArtifactError) as exc:
        validate_diagnostics([row], "t", side="after")

    assert "no-zone fallback 列" in str(exc.value)


def test_no_zone_fallback_mapping_covers_every_diagnostic_field():
    """⛔ 上面的 parametrize 必須涵蓋九欄全部，漏一欄就等於那一欄沒人守。

    ⚠️ 這條是**測試的測試**：新增診斷欄位卻忘了加 tamper 案例時，它會紅。
    """
    cases = test_tampered_no_zone_fallback_values_are_rejected.pytestmark[0].args[1]
    covered = {field for field, _bad in cases}
    assert covered == set(DIAGNOSTIC_NO_ZONE_FALLBACK)


@pytest.mark.parametrize("side", ["After", "BEFORE", "", None, "both"])
def test_unknown_side_is_rejected(side):
    """⛔ 打錯字會讓 after 的等價式被**靜默跳過**——那是 fail-open。"""
    with pytest.raises(ArtifactError) as exc:
        validate_diagnostics([_row()], "t", side=side)
    assert "side" in str(exc.value)


@pytest.mark.parametrize("bad", [123, "", []])
def test_position_action_non_null_must_be_non_empty_str(bad):
    with pytest.raises(ArtifactError):
        validate_diagnostics([_row(position_action=bad)], "t")
