"""跨日逐列比對（I-074 Stage 1）：輸入有效性、outcome 真值表、來源綁定與 tamper。

對應計畫書的五～七與測試矩陣 3／4／5。
"""
from __future__ import annotations

import copy

import pytest

from ..replay_bundle import crossday as cd
from ..replay_bundle.artifacts import ArtifactError

BUNDLE = "b1_20260901_1d_74350966_5d7ecb10"
D_SHA = "a1" * 32   # ⚠️ 要含字母，純數字的 .upper() 測不到 lowercase 檢查
D1_SHA = "b2" * 32
D_TIME = "2026-09-14T09:00:00+08:00"
D1_TIME = "2026-09-15T09:00:00+08:00"


def _prov(**overrides):
    prov = {
        "source_root": "/app",
        "image_digest": "sha256:" + "a" * 64,
        "python_version": "3.11.16",
        "pip_freeze_sha256": "b" * 64,
        "project_modules_sha256": {"backtest.modular.sr_scoring.evaluation": "c" * 64},
        "runner_sha256": "d" * 64,
        "base_commit": "e" * 40,
        "tooling_patch_sha256": "f" * 64,
        "argv": ["--bundle", "/x", "--output-dir", "/tmp/d"],
        "runtime_settings": {"db_driver": "postgres"},
    }
    prov.update(overrides)
    return prov


def _row(as_of, **overrides):
    structure, action = "SUPPORT_RECLAIM_CONFIRMED", "HOLD"
    row = {
        "symbol": "2330", "timeframe": "1d", "as_of": as_of,
        "lifecycle_phase": "TESTING",
        "clear_zone_breakout": False,
        "continuation_price_evidence_met": False,
        "setup_rr_qualified": False,
        "rr_decoupling_candidate": False,
        "event_signal": "CLOSE_RECLAIM",
        "structure_state": structure,
        "action_state": action,
        "position_action_condition": {"state": action, "structure_state": structure},
        "position_action": action,
    }
    row.update(overrides)
    return row


def _after(*, generated_at, rows=None, prov=None, **overrides):
    artifact = {
        "schema_version": 1,
        "kind": "sr_zone_replay_after",
        "bundle_id": BUNDLE,
        "timeframe": "1d",
        "replay_scope": "all_candidates",
        "run_id": "i074-stage1",
        "pipeline_version": "sr_zone_decision_replay_p1",
        "generated_at": generated_at,
        "provenance": prov or _prov(),
        "rows": rows if rows is not None else [_row("2026-08-20"), _row("2026-08-21")],
    }
    artifact.update(overrides)
    return artifact


def _pair(*, d_rows=None, d1_rows=None, d_prov=None, d1_prov=None):
    d = _after(generated_at=D_TIME, rows=d_rows, prov=d_prov)
    d1 = _after(generated_at=D1_TIME, rows=d1_rows,
                prov=d1_prov or _prov(argv=["--bundle", "/x", "--output-dir", "/tmp/d1"]))
    return d, d1


def _build(d, d1, *, comparator=None, generated_at="2026-09-15T10:00:00+08:00"):
    comparator = comparator or _prov(argv=["--compare", "--output-dir", "/tmp/cd"])
    artifact = cd.build_crossday(
        d=d, d1=d1, d_sha=D_SHA, d1_sha=D1_SHA,
        comparator_provenance=comparator, generated_at=generated_at,
    )
    return artifact, comparator


def _validate(artifact, d, d1, comparator):
    cd.validate_crossday(
        artifact, d=d, d1=d1, d_sha=D_SHA, d1_sha=D1_SHA,
        comparator_provenance=comparator,
    )


# ── 正向 ────────────────────────────────────────────────────────────────────

def test_identical_rows_only_differing_in_time_and_output_dir_is_match():
    """⚠️ 這條就是「⛔ 不能用 artifact SHA 比」的可執行證明。

    兩份的 `generated_at` 與 argv 的 output dir 必然不同，SHA 一定不相等——
    但逐列結果相同，所以必須是 `MATCH`。
    """
    d, d1 = _pair()
    artifact, comparator = _build(d, d1)
    assert artifact["outcome"] == cd.OUTCOME_MATCH
    assert artifact["matched"] is True
    _validate(artifact, d, d1, comparator)


# ── outcome 真值表 ──────────────────────────────────────────────────────────

def test_row_mismatch():
    d, d1 = _pair(d1_rows=[_row("2026-08-20", lifecycle_phase="CONFIRMED"), _row("2026-08-21")])
    artifact, comparator = _build(d, d1)
    assert (artifact["outcome"], artifact["rows_match"], artifact["provenance_match"]) == (
        cd.OUTCOME_ROW, False, True)
    assert artifact["row_differences"][0]["differences"] == ["lifecycle_phase"]
    _validate(artifact, d, d1, comparator)


def test_provenance_mismatch_when_image_differs():
    """⚠️ image_digest 不同代表**執行環境不一致**，⛔ 不是可忽略的差異。"""
    d, d1 = _pair(d1_prov=_prov(image_digest="sha256:" + "9" * 64,
                                argv=["--bundle", "/x", "--output-dir", "/tmp/d1"]))
    artifact, comparator = _build(d, d1)
    assert (artifact["outcome"], artifact["rows_match"], artifact["provenance_match"]) == (
        cd.OUTCOME_PROVENANCE, True, False)
    assert [x["field"] for x in artifact["provenance_differences"]] == ["image_digest"]
    _validate(artifact, d, d1, comparator)


def test_both_mismatch():
    d, d1 = _pair(
        d1_rows=[_row("2026-08-20", action_state="WATCH",
                      position_action_condition={"state": "WATCH",
                                                 "structure_state": "SUPPORT_RECLAIM_CONFIRMED"}),
                 _row("2026-08-21")],
        d1_prov=_prov(runner_sha256="9" * 64, argv=["--bundle", "/x", "--output-dir", "/tmp/d1"]),
    )
    artifact, comparator = _build(d, d1)
    assert artifact["outcome"] == cd.OUTCOME_BOTH
    _validate(artifact, d, d1, comparator)


def test_row_order_difference_is_visible_in_key_order():
    """⚠️ 只有順序不同時，三個差異集合都會是空的——所以必須保存兩側完整 key 序列。

    replay 靠 `previous_event_states` 串起相鄰列，**順序本身有語意**。
    """
    rows = [_row("2026-08-20"), _row("2026-08-21")]
    d, d1 = _pair(d_rows=rows, d1_rows=list(reversed(rows)))
    artifact, comparator = _build(d, d1)
    assert artifact["rows_match"] is False
    assert artifact["d_only"] == [] and artifact["d1_only"] == []
    assert artifact["row_differences"] == []
    assert artifact["d_key_order"] != artifact["d1_key_order"], "⛔ 順序差異必須看得出來"
    _validate(artifact, d, d1, comparator)


def test_exclusive_rows_are_preserved():
    d, d1 = _pair(d_rows=[_row("2026-08-20"), _row("2026-08-21")],
                  d1_rows=[_row("2026-08-21"), _row("2026-08-22")])
    artifact, comparator = _build(d, d1)
    assert artifact["d_only"] == [["2330", "1d", "2026-08-20"]]
    assert artifact["d1_only"] == [["2330", "1d", "2026-08-22"]]
    assert artifact["d_only_rows"][0]["as_of"] == "2026-08-20"
    assert artifact["d1_only_rows"][0]["as_of"] == "2026-08-22"
    _validate(artifact, d, d1, comparator)


# ── invalid input（⛔ exit 1，不是 mismatch）────────────────────────────────

def test_same_artifact_twice_is_invalid_input():
    """⚠️ **假跨日**：同一份傳兩次，三個差異集合都空——⛔ 那不是 MATCH，是無效輸入。"""
    d, _ = _pair()
    with pytest.raises(ArtifactError, match="同一份"):
        cd.assert_valid_crossday_inputs(d=d, d1=d, d_sha=D_SHA, d1_sha=D_SHA)


def test_same_day_is_invalid_input():
    d = _after(generated_at="2026-09-14T09:00:00+08:00")
    d1 = _after(generated_at="2026-09-14T18:00:00+08:00")
    with pytest.raises(ArtifactError, match="不是相鄰兩天"):
        cd.assert_valid_crossday_inputs(d=d, d1=d1, d_sha=D_SHA, d1_sha=D1_SHA)


def test_reversed_order_is_invalid_input():
    d = _after(generated_at=D1_TIME)
    d1 = _after(generated_at=D_TIME)
    with pytest.raises(ArtifactError, match="時間順序"):
        cd.assert_valid_crossday_inputs(d=d, d1=d1, d_sha=D_SHA, d1_sha=D1_SHA)


@pytest.mark.parametrize("field", ["bundle_id", "timeframe", "replay_scope", "run_id",
                                   "pipeline_version"])
def test_identity_field_mismatch_is_invalid_input(field):
    d, d1 = _pair()
    d1[field] = "OTHER"
    with pytest.raises(ArtifactError, match="不一致"):
        cd.assert_valid_crossday_inputs(d=d, d1=d1, d_sha=D_SHA, d1_sha=D1_SHA)


def test_malformed_source_provenance_is_invalid_input():
    d, d1 = _pair()
    del d1["provenance"]["runner_sha256"]
    with pytest.raises(Exception):
        cd.assert_valid_crossday_inputs(d=d, d1=d1, d_sha=D_SHA, d1_sha=D1_SHA)


# ── argv 正規化 ─────────────────────────────────────────────────────────────

@pytest.mark.parametrize("argv", [
    ["--output-dir", "/a"],
    ["--output-dir=/a"],
])
def test_output_dir_both_spellings_normalize(argv):
    assert cd.normalize_argv(argv) in (
        ["--output-dir", "<output-dir>"], ["--output-dir=<output-dir>"],
    )


@pytest.mark.parametrize("argv", [
    [], ["--output-dir"], ["--output-dir", "/a", "--output-dir", "/b"],
])
def test_output_dir_missing_or_duplicated_is_rejected(argv):
    with pytest.raises(Exception):
        cd.normalize_argv(argv)


# ── 型別 tamper（⚠️ 來源重算⛔ 取代不了型別驗證）────────────────────────────

@pytest.mark.parametrize("field", ["rows_match", "provenance_match", "matched"])
def test_flags_must_be_strict_bool(field):
    """⚠️ Python 裡 `1 == True`——用 `1` 冒充會通過所有「重算後相等」的比對。"""
    d, d1 = _pair()
    artifact, comparator = _build(d, d1)
    artifact[field] = 1 if artifact[field] else 0
    with pytest.raises(ArtifactError, match="true/false"):
        _validate(artifact, d, d1, comparator)


@pytest.mark.parametrize("field", ["d_artifact_sha256", "d1_artifact_sha256"])
def test_sha_must_be_lowercase_hex64(field):
    d, d1 = _pair()
    artifact, comparator = _build(d, d1)
    artifact[field] = artifact[field].upper()
    with pytest.raises(ArtifactError):
        _validate(artifact, d, d1, comparator)


@pytest.mark.parametrize("bad", [True, 2.0, "2", -1])
def test_row_count_must_be_strict_int(bad):
    d, d1 = _pair()
    artifact, comparator = _build(d, d1)
    artifact["d_row_count"] = bad
    with pytest.raises(ArtifactError):
        _validate(artifact, d, d1, comparator)


def test_closed_field_set():
    d, d1 = _pair()
    artifact, comparator = _build(d, d1)
    artifact["surprise"] = 1
    with pytest.raises(ArtifactError, match="欄位集合"):
        _validate(artifact, d, d1, comparator)


@pytest.mark.parametrize("outcome", [cd.OUTCOME_ROW, cd.OUTCOME_PROVENANCE, cd.OUTCOME_BOTH])
def test_outcome_must_match_the_truth_table(outcome):
    d, d1 = _pair()
    artifact, comparator = _build(d, d1)
    assert artifact["outcome"] == cd.OUTCOME_MATCH
    artifact["outcome"] = outcome
    with pytest.raises(ArtifactError, match="與旗標組合不符"):
        _validate(artifact, d, d1, comparator)


# ── 來源綁定 tamper（⚠️ 內部自洽但與來源不符）──────────────────────────────

def test_dropping_a_real_difference_is_caught_by_recomputation():
    """⚠️ **這條是來源綁定存在的理由**：artifact 內部完全自洽，但少記了一筆真實差異。"""
    d, d1 = _pair(d1_rows=[_row("2026-08-20", lifecycle_phase="CONFIRMED"), _row("2026-08-21")])
    artifact, comparator = _build(d, d1)
    assert artifact["row_differences"], "fixture 沒產生差異，這條就沒在測東西"
    artifact["row_differences"] = []
    artifact["rows_match"] = True
    artifact["matched"] = True
    artifact["outcome"] = cd.OUTCOME_MATCH
    with pytest.raises(ArtifactError, match="row_differences"):
        _validate(artifact, d, d1, comparator)


def test_exclusive_rows_tampering_is_caught():
    d, d1 = _pair(d_rows=[_row("2026-08-20"), _row("2026-08-21")],
                  d1_rows=[_row("2026-08-21")])
    artifact, comparator = _build(d, d1)
    artifact["d_only_rows"][0] = dict(artifact["d_only_rows"][0], lifecycle_phase="X")
    with pytest.raises(ArtifactError, match="與實際來源的 row 不符"):
        _validate(artifact, d, d1, comparator)


def test_provenance_differences_tampering_is_caught():
    d, d1 = _pair(d1_prov=_prov(image_digest="sha256:" + "9" * 64,
                                argv=["--bundle", "/x", "--output-dir", "/tmp/d1"]))
    artifact, comparator = _build(d, d1)
    artifact["provenance_differences"] = []
    artifact["provenance_match"] = True
    artifact["matched"] = True
    artifact["outcome"] = cd.OUTCOME_MATCH
    with pytest.raises(ArtifactError, match="provenance_differences"):
        _validate(artifact, d, d1, comparator)


def test_forged_comparator_provenance_is_caught():
    """⚠️ 只驗 schema ⛔ 抓不到「格式合法但內容偽造」——要與本次實際產生的逐欄比對。"""
    d, d1 = _pair()
    artifact, comparator = _build(d, d1)
    artifact["comparator_provenance"] = _prov(runner_sha256="0" * 64,
                                              argv=["--compare", "--output-dir", "/tmp/cd"])
    with pytest.raises(ArtifactError, match="偽造"):
        _validate(artifact, d, d1, comparator)


def test_sha_not_matching_actual_source_is_caught():
    d, d1 = _pair()
    artifact, comparator = _build(d, d1)
    with pytest.raises(ArtifactError, match="與實際載入的不符"):
        cd.validate_crossday(artifact, d=d, d1=d1, d_sha="c3" * 32, d1_sha=D1_SHA,
                             comparator_provenance=comparator)


def test_key_order_tampering_is_caught():
    d, d1 = _pair()
    artifact, comparator = _build(d, d1)
    artifact["d_key_order"] = list(reversed(artifact["d_key_order"]))
    with pytest.raises(ArtifactError, match="key 序列不符"):
        _validate(artifact, d, d1, comparator)


def test_mismatch_exception_is_not_a_valueerror():
    """⛔ 繼承 `ValueError` 的話，CLI 的 generic catch 會把 exit 5 吃掉。"""
    assert not issubclass(cd.CrossdayMismatch, ValueError)


# ── CLI：ownership 與核心產品分支（測試矩陣 12-A）────────────────────────────

def _write(tmp_path, name, payload):
    from ..replay_bundle.canonical import canonical_json_bytes

    path = tmp_path / name
    path.write_bytes(canonical_json_bytes(payload))
    return path


def _identity(bundle_id=BUNDLE):
    from ..replay_bundle.run_identity import build_run_identity

    return build_run_identity(
        bundle_id=bundle_id,
        expected_image_id="sha256:" + "a" * 64,
        created_at="2026-09-14T08:00:00+08:00",
    )


def _cli_argv(tmp_path, *, d, d1, identity, out):
    return [
        "--d", str(d), "--d1", str(d1), "--output-dir", str(out),
        "--run-identity", str(identity),
        "--image-digest", "sha256:" + "a" * 64,
        "--base-commit", "e" * 40,
        "--tooling-patch-sha256", "f" * 64,
        "--source-root", "/app",
        "--runner-sha256", "d" * 64,
    ]


def test_identity_is_the_independent_second_source_for_bundle_id(tmp_path):
    """⚠️ **這是 comparator 掛 identity 的理由**——兩份 after 只能互比。

    D 與 D+1 **都合法且帶相同的 `bundle_id = B`**，identity 合法但記 `bundle_id = A`：
    ⛔ 沒有 identity 當獨立第二來源，這種情況沒有任何東西能發現。
    """
    d, d1 = _pair()
    d_path = _write(tmp_path, "d.json", d)
    d1_path = _write(tmp_path, "d1.json", d1)
    ident = _write(tmp_path, "identity.json", _identity(bundle_id="b1_OTHER_BUNDLE"))
    out = tmp_path / "out"

    with pytest.raises(ArtifactError, match="run identity"):
        cd.run_crossday(_cli_argv(tmp_path, d=d_path, d1=d1_path, identity=ident, out=out))

    assert not (out / cd.CROSSDAY_ARTIFACT_NAME).exists(), "⛔ 不得發布 crossday artifact"


def test_duplicate_run_identity_is_rejected():
    """⛔ 注入參數重複 → 中止，⛔ 不靜默採用最後一個。"""
    with pytest.raises(cd.CrossdayUsageError, match="出現 2 次"):
        cd.assert_crossday_arg_ownership(
            ["--run-identity", "/a", "--run-identity", "/b"]
        )


def test_abbreviation_is_not_expanded():
    """⚠️ `--run-id` 是 `--run-identity` 的前綴——⛔ argparse 預設會把它展開。

    crossday 的 parser 設了 `allow_abbrev=False`，所以縮寫會被當成未知參數拒絕。
    ⛔ 這也是為什麼它⛔ 不共用 `REPLAY_INJECTED_ARGS` 的前綴清單：那份會反過來把既有
    合法的 `--run-id` 誤殺（實測過）。
    """
    parser = cd.build_crossday_parser()
    # ⚠️ **給一組完整合法的 argv，只把 `--run-identity` 換成縮寫**——
    # ⛔ 只丟 `["--run-ident", "/x"]` 是 **false pass**：即使 argparse 接受了縮寫，
    # 也會因為其他必填參數缺失而同樣 SystemExit，測不到縮寫這件事。
    full = [
        "--d", "/d.json", "--d1", "/d1.json", "--output-dir", "/out",
        "--image-digest", "sha256:" + "a" * 64, "--base-commit", "e" * 40,
        "--tooling-patch-sha256", "f" * 64, "--source-root", "/app",
        "--runner-sha256", "d" * 64,
    ]
    parser.parse_args(full + ["--run-identity", "/x"])   # 完整形式：必須可解析
    with pytest.raises(SystemExit):
        parser.parse_args(full + ["--run-ident", "/x"])  # 縮寫：⛔ 不得被展開
