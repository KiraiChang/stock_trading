"""第五道集合檢查與 `candidate_mismatch.json`（I-074 Stage 0）。

⚠️ 這是 Stage 2 的**第二種 terminal outcome**：before／after 的候選集合不一致
**本身就是分支 C**，⛔ 不是失敗殘骸。
"""
from __future__ import annotations

import copy
import json

import pytest

from ..replay_bundle import (
    EXIT_CANDIDATE_MISMATCH,
    MISMATCH_ARTIFACT_NAME,
    CandidateMismatch,
    build_candidate_mismatch,
    validate_candidate_mismatch,
)
from ..replay_bundle.artifacts import ArtifactError


def _row(as_of, candidate, **overrides):
    row = {
        "symbol": "2330", "timeframe": "1d", "as_of": as_of,
        "lifecycle_phase": "CONTINUATION" if candidate else "TESTING",
        "setup_rr_qualified": False,
        "rr_decoupling_candidate": candidate,
    }
    row.update(overrides)
    return row


K1 = ("2330", "1d", "2026-01-01T00:00:00+00:00")
K2 = ("2330", "1d", "2026-01-02T00:00:00+00:00")


# ⚠️ **來源必須是 pristine copy**：`compare_rows()` 把來源 dict **直接**放進 artifact
# （`artifacts.py` 的 `"before": before`），所以 `artifact["rows"][i]["before"]`
# **就是** `before_by_key[key]` 那個物件。測試竄改 artifact 的內嵌 row 時會一併改到來源，
# 來源對照就變成「自己跟自己比」而**永遠通過**——那是假綠。
# 這裡用 `id(artifact)` 記住當次 build 的深拷貝，一個測試 build 兩份也不會互相覆蓋。
_SOURCES: dict[int, dict] = {}


def _validate(artifact, **overrides):
    """用**該份 artifact 當次 build 的 pristine 來源**驗證。"""
    kwargs = dict(_SOURCES[id(artifact)])
    kwargs.update(overrides)
    validate_candidate_mismatch(artifact, **kwargs)


def _build(**overrides):
    kwargs = dict(
        bundle_id="b1_20260901_1d_aaaaaaaa_bbbbbbbb",
        after_artifact_sha256="a" * 64,
        before_ref="ecbc141^",
        generated_at="2026-09-14T00:00:00+00:00",
        provenance={"image_digest": "sha256:x"},
        before_only=[K1],
        after_only=[],
        before_candidate_count=1,
        after_candidate_count=0,
        before_by_key={K1: _row(K1[2], True)},
        after_by_key={K1: _row(K1[2], False)},
    )
    kwargs.update(overrides)
    artifact = build_candidate_mismatch(**kwargs)
    _SOURCES[id(artifact)] = {
        "before_by_key": copy.deepcopy(kwargs["before_by_key"]),
        "after_by_key": copy.deepcopy(kwargs["after_by_key"]),
        "expected_after_sha256": kwargs["after_artifact_sha256"],
    }
    return artifact


def test_build_and_validate_round_trip():
    artifact = _build()
    _validate(artifact)
    assert artifact["kind"] == "sr_zone_replay_candidate_mismatch"
    assert artifact["before_only"] == [list(K1)]
    assert artifact["after_only"] == []


def test_rows_cover_the_union_and_keep_both_sides():
    """⚠️ 這是 v9→v11 反覆修的那一條：**差集裡每個 key 的兩側 row 都要存**。"""
    artifact = _build(
        before_only=[K1], after_only=[K2],
        before_candidate_count=1, after_candidate_count=1,
        before_by_key={K1: _row(K1[2], True), K2: _row(K2[2], False)},
        after_by_key={K1: _row(K1[2], False), K2: _row(K2[2], True)},
    )
    _validate(artifact)
    keys = [(r["symbol"], r["timeframe"], r["as_of"]) for r in artifact["rows"]]
    assert keys == sorted([K1, K2])          # 涵蓋聯集且依固定順序
    for row in artifact["rows"]:
        assert isinstance(row["before"], dict) and isinstance(row["after"], dict)


def test_outer_key_matches_both_embedded_rows():
    artifact = _build()
    item = artifact["rows"][0]
    assert (item["symbol"], item["timeframe"], item["as_of"]) == K1
    assert item["before"]["as_of"] == item["after"]["as_of"] == K1[2]


def test_differences_always_contains_the_candidate_field():
    """mismatch key 來自 symmetric difference → candidate 兩側必然不同。"""
    artifact = _build()
    assert "rr_decoupling_candidate" in artifact["rows"][0]["differences"]


# ── validator 的拒絕條件 ────────────────────────────────────────────────────

def test_empty_diff_is_rejected():
    """⛔ 零差集卻宣稱 mismatch。"""
    artifact = _build()
    artifact["before_only"] = []
    artifact["rows"] = []
    artifact["before_candidate_count"] = 0
    with pytest.raises(ArtifactError) as exc:
        _validate(artifact)
    assert "不構成 mismatch" in str(exc.value)


def test_count_lower_bound_catches_impossible_combination():
    """⚠️ 「兩側 count 都是 0、兩側差集各一筆」會通過差值公式，但集合上不可能。"""
    artifact = _build(
        before_only=[K1], after_only=[K2],
        before_candidate_count=0, after_candidate_count=0,
        before_by_key={K1: _row(K1[2], True), K2: _row(K2[2], False)},
        after_by_key={K1: _row(K1[2], False), K2: _row(K2[2], True)},
    )
    # 差值公式：0 − 0 == 1 − 1 ✅ 通過；但下界會擋下來。
    with pytest.raises(ArtifactError) as exc:
        _validate(artifact)
    assert "集合上不可能" in str(exc.value)


def test_count_difference_formula_is_enforced():
    artifact = _build(before_candidate_count=5)
    with pytest.raises(ArtifactError) as exc:
        _validate(artifact)
    assert "不符" in str(exc.value)


@pytest.mark.parametrize("bad", ["abc", "A" * 64, 123, None])
def test_after_artifact_sha256_must_be_lowercase_hex64(bad):
    artifact = _build()
    artifact["after_artifact_sha256"] = bad
    with pytest.raises(ArtifactError):
        _validate(artifact)


@pytest.mark.parametrize("field", ["bundle_id", "before_ref", "generated_at"])
def test_required_strings_reject_empty(field):
    artifact = _build()
    artifact[field] = ""
    with pytest.raises(ArtifactError):
        _validate(artifact)


@pytest.mark.parametrize("value", [True, -1, "1", 1.0])
def test_counts_reject_bool_negative_and_wrong_type(value):
    """⚠️ `True` 要被擋掉——bool 是 int 的子類。"""
    artifact = _build()
    artifact["before_candidate_count"] = value
    with pytest.raises(ArtifactError):
        _validate(artifact)


def test_top_level_closed_field_set():
    artifact = _build()
    artifact["surprise"] = 1
    with pytest.raises(ArtifactError):
        _validate(artifact)

    artifact2 = _build()
    del artifact2["provenance"]
    with pytest.raises(ArtifactError):
        _validate(artifact2)


def test_rows_closed_field_set():
    artifact = _build()
    artifact["rows"][0]["surprise"] = 1
    with pytest.raises(ArtifactError):
        _validate(artifact)


@pytest.mark.parametrize("mutate", [
    pytest.param(lambda d: d.pop(), id="刪欄"),
    pytest.param(lambda d: d.append("not_a_real_field"), id="加欄"),
    pytest.param(lambda d: d.append(d[0]), id="重複"),
    pytest.param(lambda d: d.reverse(), id="改順序"),
])
def test_differences_tampering_is_rejected(mutate):
    """`differences` 是正式證據——⛔ 不接受空陣列／錯欄位／重複／未排序。"""
    artifact = _build(
        before_by_key={K1: _row(K1[2], True, extra_a=1, extra_b=2)},
        after_by_key={K1: _row(K1[2], False, extra_a=9, extra_b=8)},
    )
    diffs = artifact["rows"][0]["differences"]
    assert len(diffs) >= 2
    mutate(diffs)
    with pytest.raises(ArtifactError) as exc:
        _validate(artifact)
    assert "differences" in str(exc.value)


def test_null_side_row_is_rejected():
    artifact = _build()
    artifact["rows"][0]["before"] = None
    with pytest.raises(ArtifactError) as exc:
        _validate(artifact)
    assert "不得為 null" in str(exc.value)


def test_exception_is_not_a_valueerror():
    """⛔ 繼承 `ValueError` 的話，CLI 的 generic catch 會把專屬碼吃掉。"""
    assert not issubclass(CandidateMismatch, ValueError)
    assert EXIT_CANDIDATE_MISMATCH == 4


def test_wrong_schema_version_is_rejected():
    artifact = _build()
    artifact["schema_version"] = 2
    with pytest.raises(ArtifactError) as exc:
        _validate(artifact)
    assert "schema_version" in str(exc.value)


def test_wrong_kind_is_rejected():
    artifact = _build()
    artifact["kind"] = "sr_zone_replay_comparison"
    with pytest.raises(ArtifactError) as exc:
        _validate(artifact)
    assert "kind" in str(exc.value)


def test_direction_must_match_the_embedded_candidate_values():
    """⚠️ `before_only` 的列必然是 before=true／after=false——⛔ 顛倒要被抓到。

    少了方向檢查，一份**格式完全合法但內容顛倒**的 artifact 照樣通過。
    """
    artifact = _build()
    item = artifact["rows"][0]
    # 把兩側的 candidate 值互換（格式仍然合法）
    item["before"]["rr_decoupling_candidate"] = False
    item["after"]["rr_decoupling_candidate"] = True
    item["differences"] = sorted(
        f for f in set(item["before"]) | set(item["after"])
        if item["before"].get(f) != item["after"].get(f)
    )
    with pytest.raises(ArtifactError) as exc:
        _validate(artifact)
    assert "方向不符" in str(exc.value)


def test_after_only_direction_is_checked_too():
    artifact = _build(
        before_only=[], after_only=[K1],
        before_candidate_count=0, after_candidate_count=1,
        before_by_key={K1: _row(K1[2], False)},
        after_by_key={K1: _row(K1[2], True)},
    )
    _validate(artifact)   # 正確方向：通過
    item = artifact["rows"][0]
    item["before"]["rr_decoupling_candidate"] = True
    item["after"]["rr_decoupling_candidate"] = False
    item["differences"] = sorted(
        f for f in set(item["before"]) | set(item["after"])
        if item["before"].get(f) != item["after"].get(f)
    )
    with pytest.raises(ArtifactError):
        _validate(artifact)


# ── 與實際來源的比對（第三層）──────────────────────────────────────────────
#
# ⚠️ 前面每一條驗的都是**內部自洽**：artifact 自己跟自己對得起來。
# ⛔ 但一份「漏掉一個真實 mismatch」的產物可以完全內部自洽——集合、差值公式、下界、
# 方向、differences 全部通過，只是少記了一筆。唯一能發現它的東西是本次 replay 的實際 row。

def test_artifact_that_silently_drops_a_real_mismatch_is_rejected():
    """⚠️ **這條是整個第三層存在的理由**：內部完全自洽，但少記了一筆真實差異。

    來源其實有兩筆：K1 是 before-only、K2 是 after-only。artifact 只宣告 K1，
    而且把 count 也一起調成自洽的值——前兩層⛔ 完全抓不到。
    """
    artifact = _build(
        before_only=[K1], after_only=[],
        before_candidate_count=1, after_candidate_count=0,
        before_by_key={K1: _row(K1[2], True), K2: _row(K2[2], False)},
        after_by_key={K1: _row(K1[2], False), K2: _row(K2[2], True)},
    )

    with pytest.raises(ArtifactError) as exc:
        _validate(artifact)

    assert "after_only 與實際來源不符" in str(exc.value)


def test_counts_must_equal_the_actual_source_counts():
    """⚠️ 差集對了，**計數**仍可能是錯的——它是判讀分支 B／C 的母數。"""
    artifact = _build(
        before_only=[K1], after_only=[],
        before_candidate_count=1, after_candidate_count=0,
        before_by_key={K1: _row(K1[2], True), K2: _row(K2[2], True)},
        after_by_key={K1: _row(K1[2], False), K2: _row(K2[2], True)},
    )

    with pytest.raises(ArtifactError) as exc:
        _validate(artifact)

    # 來源算出 before=2／after=1，artifact 卻記 1／0。
    assert "與實際來源的候選列數" in str(exc.value)


def test_embedded_rows_must_equal_the_actual_source_rows():
    """⚠️ 內嵌的兩側 row 必須**就是**實際 row，⛔ 不只是「形狀對、方向對」。

    這裡**兩側同步**改同一個欄位，所以 `differences` 完全不變——前兩層一律通過。
    ⚠️ 沒有 pristine copy 的話這條測不出來：竄改會一併改到來源物件本身。
    """
    artifact = _build()
    item = artifact["rows"][0]
    assert item["before"]["setup_rr_qualified"] is False
    item["before"]["setup_rr_qualified"] = True
    item["after"]["setup_rr_qualified"] = True

    with pytest.raises(ArtifactError) as exc:
        _validate(artifact)

    assert "與實際來源的 row 不符" in str(exc.value)


def test_after_sha_must_equal_the_actually_loaded_artifact():
    """⛔ 格式合法的 SHA 不代表它指向本次比較的那份 after artifact。"""
    artifact = _build()

    with pytest.raises(ArtifactError) as exc:
        _validate(artifact, expected_after_sha256="b" * 64)

    assert "指向的不是本次比較的那個 artifact" in str(exc.value)


def test_source_candidate_must_be_a_strict_boolean():
    """⚠️ 來源列的 candidate 缺值時⛔ 不得當成 false。

    當成 false 的話，一個真實候選會從重建出來的集合裡**靜默消失**，
    比對反而「通過」——那正是第三層要防的那種假綠。
    """
    artifact = _build()

    with pytest.raises(ArtifactError) as exc:
        _validate(artifact, before_by_key={K1: _row(K1[2], None)})

    assert "不是嚴格 boolean" in str(exc.value)


def test_source_arguments_are_mandatory():
    """⛔ 不提供「不給就跳過」的模式——可選參數等於留一道 fail-open。"""
    artifact = _build()

    with pytest.raises(TypeError):
        validate_candidate_mismatch(artifact)
