"""I-129 的量測欄位（validation-only；見 docs/sr-zone-scoring.md「I-129 的量測欄位」）。

兩件事：
1. 匯出端：`_decision_fields_from_summary()` 的每個量測欄位都等於 decision summary 的對應值，
   ⛔ 不重算。
2. 檢查端：`check_measurement_row()`／`validate_measurement_fields()` 擋得住缺欄位、
   有效列是 null、無效列沿用舊值、digest 格式錯、全部無效列這幾種情況。

replay 列真的帶出這些欄位，由 `test_evaluation.py` 的兩個 `run_decision_replay` 測試驗。
"""
from __future__ import annotations

import hashlib

import pytest

from ..evaluation import _decision_fields_from_summary
from ..replay_bundle import (
    DECISION_MEASUREMENT_FIELDS,
    DECISION_MEASUREMENT_LIST_FIELDS,
    DIAGNOSTIC_FIELDS,
    ArtifactError,
    check_measurement_row,
    risk_notes_digest,
    validate_measurement_fields,
)
from ..replay_bundle.canonical import canonical_json_bytes
from .test_decision_engine import _continuation_full_entry_case


def _expected(ds: dict) -> dict:
    """直接從 summary 讀出每個欄位的期望值（⛔ 不經過被測的函式）。"""
    derived = ds["decision_derived_view"]
    semantic = derived["semantic_pipeline"]
    return {
        "market_action": ds["market_action"],
        "action": ds["action"],
        "entry_action_state": ds["entry_action_state"],
        "semantic_entry_permission_state": semantic["entry_permission_state"],
        "semantic_reason_codes": semantic["reason_codes"],
        "path_gate_state": derived["path_gate_state"],
        "final_entry_reason_codes": ds["final_entry_permission"]["reason_codes"],
        "risk_notes_digest": hashlib.sha256(canonical_json_bytes(ds["risk_notes"])).hexdigest()[:16],
    }


@pytest.mark.parametrize("resistance_low", [122.0, 118.0])
def test_measurement_fields_equal_decision_summary(resistance_low):
    """122.0：兩層 RR 都過（ENTRY）；118.0：secondary 不合格、封頂成 PROBE＋`RR_BELOW_FULL_ENTRY`。"""
    ds = _continuation_full_entry_case(resistance_low)
    fields = _decision_fields_from_summary(ds)

    assert set(DECISION_MEASUREMENT_FIELDS) <= set(fields)
    assert {name: fields[name] for name in DECISION_MEASUREMENT_FIELDS} == _expected(ds)
    # ⛔ 不能和九個診斷欄位重疊：那份契約不動。
    assert not set(DECISION_MEASUREMENT_FIELDS) & set(DIAGNOSTIC_FIELDS)
    # 匯出端產出的值要過得了檢查端（同一份清單、同一份規則）。
    row = {"symbol": "2330", "timeframe": "1d", "as_of": "2026-09-01T00:00:00",
           "decision_fields_available": True, **fields}
    assert check_measurement_row(row, "unit") is True


def test_measurement_fields_are_not_vacuous():
    """⛔ 不讓比對在空 list／相同值上蒙混過關：封頂那一組要真的帶出 reason code，
    而且兩組的 semantic 許可要不同。"""
    entry = _decision_fields_from_summary(_continuation_full_entry_case(122.0))
    capped = _decision_fields_from_summary(_continuation_full_entry_case(118.0))
    assert entry["semantic_entry_permission_state"] == "ENTRY_ALLOWED"
    assert capped["semantic_entry_permission_state"] == "PROBE_ALLOWED"
    assert "RR_BELOW_FULL_ENTRY" in capped["semantic_reason_codes"]
    assert "RR_BELOW_FULL_ENTRY" in capped["final_entry_reason_codes"]


def test_measurement_lists_are_copies():
    """匯出的 list ⛔ 不能和 summary 共用同一個物件（之後改 summary 不得改到列）。"""
    ds = _continuation_full_entry_case(118.0)
    fields = _decision_fields_from_summary(ds)
    assert fields["semantic_reason_codes"] is not ds["decision_derived_view"]["semantic_pipeline"]["reason_codes"]
    assert fields["final_entry_reason_codes"] is not ds["final_entry_permission"]["reason_codes"]


def test_risk_notes_digest_tracks_content():
    notes = ["波動偏高，倉位需保守。", {"code": "RR_BELOW_FULL_ENTRY", "text": "x"}]
    digest = risk_notes_digest(notes)
    assert len(digest) == 16 and digest == digest.lower()
    assert risk_notes_digest(list(notes)) == digest
    assert risk_notes_digest(notes + ["多一句"]) != digest
    assert risk_notes_digest(list(reversed(notes))) != digest, "順序也算內容"
    assert risk_notes_digest([]) != digest
    assert risk_notes_digest(None) is None


def test_missing_summary_parts_give_none_not_crash():
    fields = _decision_fields_from_summary({})
    for name in DECISION_MEASUREMENT_FIELDS:
        assert fields[name] is None, name


# ── 檢查端 ──────────────────────────────────────────────────────────────────

def _available_row(**overrides) -> dict:
    row = {
        "symbol": "2330", "timeframe": "1d", "as_of": "2026-09-01T00:00:00",
        "decision_fields_available": True,
        "market_action": "Hold", "action": "Hold", "entry_action_state": "WAIT",
        "semantic_entry_permission_state": "WAIT_CONFIRMATION", "path_gate_state": "WAIT_CONFIRMATION",
        "risk_notes_digest": "0123456789abcdef",
        "semantic_reason_codes": [], "final_entry_reason_codes": ["RR_INSUFFICIENT"],
    }
    row.update(overrides)
    return row


def _unavailable_row(**overrides) -> dict:
    row = {"symbol": "2330", "timeframe": "1d", "as_of": "2026-09-02T00:00:00",
           "decision_fields_available": False, **dict.fromkeys(DECISION_MEASUREMENT_FIELDS)}
    row.update(overrides)
    return row


def test_check_accepts_valid_rows():
    assert check_measurement_row(_available_row(), "unit") is True
    assert check_measurement_row(_unavailable_row(), "unit") is False
    assert validate_measurement_fields([_available_row(), _unavailable_row()], "unit") == 1


@pytest.mark.parametrize("field", DECISION_MEASUREMENT_FIELDS)
def test_check_rejects_missing_field(field):
    for row in (_available_row(), _unavailable_row()):
        del row[field]
        with pytest.raises(ArtifactError, match=field):
            check_measurement_row(row, "unit")


@pytest.mark.parametrize("field", DECISION_MEASUREMENT_FIELDS)
def test_check_rejects_null_on_available_row(field):
    with pytest.raises(ArtifactError, match=field):
        check_measurement_row(_available_row(**{field: None}), "unit")


@pytest.mark.parametrize("field", DECISION_MEASUREMENT_FIELDS)
def test_check_rejects_stale_value_on_unavailable_row(field):
    """decision 不可用的列⛔ 不得沿用上一列的值。"""
    stale = ["X"] if field in DECISION_MEASUREMENT_LIST_FIELDS else "0123456789abcdef"
    with pytest.raises(ArtifactError, match=field):
        check_measurement_row(_unavailable_row(**{field: stale}), "unit")


@pytest.mark.parametrize("bad", ["", 1, ["ok", ""], ["ok", 1], ("RR",)])
def test_check_rejects_bad_types(bad):
    with pytest.raises(ArtifactError):
        check_measurement_row(_available_row(final_entry_reason_codes=bad), "unit")
    if not isinstance(bad, (list, tuple)):
        with pytest.raises(ArtifactError):
            check_measurement_row(_available_row(market_action=bad), "unit")


@pytest.mark.parametrize("digest", ["0123456789abcde", "0123456789abcdef0", "0123456789ABCDEF", "0123456789abcdeg"])
def test_check_rejects_bad_digest(digest):
    with pytest.raises(ArtifactError, match="risk_notes_digest"):
        check_measurement_row(_available_row(risk_notes_digest=digest), "unit")


@pytest.mark.parametrize("flag", [None, 1, 0, "true"])
def test_check_rejects_non_bool_availability(flag):
    with pytest.raises(ArtifactError, match="decision_fields_available"):
        check_measurement_row(_available_row(decision_fields_available=flag), "unit")


def test_validate_rejects_all_unavailable():
    """空集合⛔ 不能算通過：全部是 null 時逐列檢查會全部通過，等於什麼都沒驗到。"""
    with pytest.raises(ArtifactError, match="沒有任何一列"):
        validate_measurement_fields([_unavailable_row(), _unavailable_row(as_of="2026-09-03T00:00:00")], "unit")
    with pytest.raises(ArtifactError, match="沒有任何一列"):
        validate_measurement_fields([], "unit")
