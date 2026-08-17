from __future__ import annotations

from ..pipeline import _merge_regression_governance_gate


def _base_gate(
    health_state: str = "HEALTHY",
    allow_entry: bool = True,
    max_entry_state: str = "ENTRY_ALLOWED",
) -> dict:
    return {
        "health_state": health_state,
        "quality_flags": [],
        "warning_flags": [],
        "blocking_flags": [],
        "confidence_gate": {
            "state": health_state,
            "allow_entry": allow_entry,
            "max_entry_state": max_entry_state,
            "reason_codes": [],
        },
    }


def _regression(health_state: str, allow_entry: bool = True, max_entry_state: str = "ENTRY_ALLOWED") -> dict:
    return {
        "health_state": health_state,
        "confidence_gate": {
            "state": health_state,
            "allow_entry": allow_entry,
            "max_entry_state": max_entry_state,
            "reason_codes": [],
        },
    }


def test_merge_without_regression_leaves_base_untouched():
    base = _base_gate()

    assert _merge_regression_governance_gate(base, None) is base


def test_merge_unreliable_regression_blocks_entry():
    merged = _merge_regression_governance_gate(
        _base_gate(),
        _regression("UNRELIABLE", allow_entry=False, max_entry_state="WAIT_CONFIRMATION"),
    )

    assert merged["health_state"] == "UNRELIABLE"
    assert merged["confidence_gate"]["allow_entry"] is False
    assert merged["confidence_gate"]["max_entry_state"] == "WAIT_CONFIRMATION"
    assert "REGRESSION_GOVERNANCE_UNRELIABLE" in merged["blocking_flags"]


def test_merge_degraded_regression_caps_entry_but_allows():
    merged = _merge_regression_governance_gate(_base_gate(), _regression("DEGRADED"))

    assert merged["health_state"] == "DEGRADED"
    assert merged["confidence_gate"]["allow_entry"] is True
    assert merged["confidence_gate"]["max_entry_state"] == "SMALL_ENTRY"
    assert "REGRESSION_GOVERNANCE_DEGRADED" in merged["warning_flags"]


def test_merge_never_relaxes_a_stricter_base():
    """gate 只趨保守：regression 說 HEALTHY 也不能放寬 base 已經封鎖的結論。"""
    base = _base_gate("UNRELIABLE", allow_entry=False, max_entry_state="WAIT_CONFIRMATION")

    merged = _merge_regression_governance_gate(base, _regression("HEALTHY"))

    assert merged["health_state"] == "UNRELIABLE"
    assert merged["confidence_gate"]["allow_entry"] is False
    assert merged["confidence_gate"]["max_entry_state"] == "WAIT_CONFIRMATION"


def test_merge_flags_unknown_health_state():
    """認不得的 health_state 不升嚴重度（不誤擋），但必須留下訊號。

    規格見 docs/sr-zone-scoring.md「未知的 `health_state`」。
    """
    merged = _merge_regression_governance_gate(
        _base_gate(),
        _regression("SOMETHING_ELSE"),
    )

    assert merged["health_state"] == "HEALTHY"
    assert merged["confidence_gate"]["allow_entry"] is True
    assert "REGRESSION_GOVERNANCE_STATE_UNKNOWN" in merged["warning_flags"]
    assert "REGRESSION_GOVERNANCE_STATE_UNKNOWN" in merged["confidence_gate"]["reason_codes"]
    assert merged["blocking_flags"] == []
