"""row-level 共用原語：整批 validator 與串流 validator 的判定必須相同（I-074 ③ evidence contract 測試 bj0）。

⚠️ 重構的前提是**公開 contract 與 Stage 1 行為不變**：整批路徑的既有測試⛔ 不修改斷言、全數續跑；
這一檔補的是另一半——**同一組壞列**分別餵給整批與串流兩條路徑，被擋下的列與錯誤訊息必須相同。
"""
from __future__ import annotations

import pytest

from ..replay_bundle.artifacts import (
    ArtifactError,
    StreamRowValidator,
    validate_after_artifact,
    validate_diagnostics,
    validate_replay_errors,
)
from .test_replay_evidence import _after, _row

LABEL = "after artifact"   # 整批路徑的結構檢查寫死這個 label；兩邊用同一個才能逐字比訊息


def _rows():
    rows = [_row(f"2026-08-{10 + i:02d}") for i in range(4)]
    rows[1] |= {"lifecycle_phase": "CONTINUATION", "rr_decoupling_candidate": True}
    return rows


def _bulk(rows, side):
    artifact = _after("2026-09-15T09:00:00+08:00", rows, "/tmp/d1")
    keys = validate_after_artifact(artifact)
    validate_replay_errors(rows, LABEL)
    validate_diagnostics(rows, LABEL, side=side)
    return keys


def _stream(rows, side):
    validator = StreamRowValidator(LABEL, side=side)
    for row in rows:
        validator.feed(row)
    return validator.finish()


def _mutate(kind):
    rows = _rows()
    if kind == "not_object":
        rows[2] = ["not", "an", "object"]
    elif kind == "missing_key_field":
        del rows[2]["symbol"]
    elif kind == "duplicate_key":
        rows[3] = dict(rows[2])
    elif kind == "candidate_missing":
        del rows[2]["rr_decoupling_candidate"]
    elif kind == "candidate_int":
        rows[2]["rr_decoupling_candidate"] = 0
    elif kind == "zone_error":
        rows[2]["zone_score_error"] = "BOOM"
    elif kind == "decision_error":
        rows[2]["decision_error"] = "ValueError: x"
    elif kind == "missing_structure_state":
        del rows[2]["structure_state"]
    elif kind == "action_state_mismatch":
        rows[2]["action_state"] = "WATCH"
    elif kind == "equivalence_broken":
        rows[2]["rr_decoupling_candidate"] = True   # lifecycle 仍是 TESTING
    elif kind == "fake_no_zone":
        rows[2] |= {"zone_score_available": False, "zone_score_error": "NO_ZONE_SCORES"}
    else:  # pragma: no cover
        raise AssertionError(kind)
    return rows


CASES = [
    "not_object", "missing_key_field", "duplicate_key", "candidate_missing", "candidate_int",
    "zone_error", "decision_error", "missing_structure_state", "action_state_mismatch",
    "equivalence_broken", "fake_no_zone",
]


@pytest.mark.parametrize("side", ["after", "before"])
def test_valid_rows_pass_both_paths_with_same_keys(side):
    assert _bulk(_rows(), side) == _stream(_rows(), side)


@pytest.mark.parametrize("side", ["after", "before"])
@pytest.mark.parametrize("kind", CASES)
def test_same_bad_rows_same_verdict(kind, side):
    if kind == "equivalence_broken" and side == "before":
        # ⚠️ `side="before"` 的公開 contract 就是⛔ 不驗等價式——兩條路徑都要**放行**。
        assert _bulk(_mutate(kind), side) == _stream(_mutate(kind), side)
        return
    with pytest.raises(ArtifactError) as bulk:
        _bulk(_mutate(kind), side)
    with pytest.raises(ArtifactError) as stream:
        _stream(_mutate(kind), side)
    assert str(stream.value) == str(bulk.value)


def test_side_is_validated_up_front():
    with pytest.raises(ArtifactError, match="side 只接受"):
        StreamRowValidator(LABEL, side="After")


def test_finished_validator_rejects_more_rows():
    validator = StreamRowValidator(LABEL, side="after")
    validator.finish()
    with pytest.raises(ArtifactError, match="已 finish"):
        validator.feed(_rows()[0])
