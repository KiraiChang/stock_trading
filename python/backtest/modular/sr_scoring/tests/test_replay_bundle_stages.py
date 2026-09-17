"""Stage 1／2：從 bundle 載入、四道集合檢查、候選欄位守門與原子發布（I-100 Phase E）。

⚠️ 這一檔用 stub 取代 `_decision_replay_rows`：真的跑 decision engine 需要一份實際模型，
而這裡要驗的是**流程與護欄**（key 集合、欄位守門、發布順序），不是 replay 的數值。
replay 本身的行為由 test_evaluation.py 覆蓋。
"""
from __future__ import annotations

import json
import pathlib
from types import SimpleNamespace

import pytest

from .. import evaluation as evaluation_module
from ..evaluation import run_bundle_stage
from ..replay_bundle import (
    AFTER_ARTIFACT_NAME,
    COHORT_MANIFEST_NAME,
    COMPARISON_ARTIFACT_NAME,
    REPORT_NAME,
    ArtifactError,
    emit_bundle,
)
from ..replay_bundle import DIAGNOSTIC_NO_ZONE_FALLBACK
from ..replay_bundle.canonical import canonical_json_bytes, sha256_hex
from .replay_bundle_fixtures import make_manifest_and_payloads


def _bundle_with_bars(tmp_path, bars: int = 90, symbols=("2330",)):
    """一份可跑 Stage 1／2 的 bundle：每檔 `bars` 根，足夠 min_history_bars=80 ＋ forward=5。"""
    candles = {}
    base_ts = 1_700_000_000
    for symbol in symbols:
        candles[symbol] = [
            {"timestamp": base_ts + i * 86400, "timeframe": "1d",
             "open": 100.0 + i, "high": 101.0 + i, "low": 99.0 + i, "close": 100.5 + i,
             "volume": 1000.0}
            for i in range(bars)
        ]
    manifest, payloads = make_manifest_and_payloads(
        symbols=list(symbols), candles_by_symbol=candles,
    )
    return emit_bundle(tmp_path / "baselines", payloads, manifest).path


@pytest.fixture
def stub_replay(monkeypatch):
    """把 replay 換成「每個 universe key 產一列」的決定性 stub。"""
    state = {"candidate_keys": set(), "drop_field": False, "extra_key": None, "drop_key": None,
             "duplicate": False, "mutate": None}

    def fake_rows(sources, dataset_config, quota, **kwargs):
        keys = evaluation_module._bundle_universe_keys(sources, dataset_config)
        rows = []
        for symbol, timeframe, as_of in keys:
            # ⚠️ stub 要產出**完整的九個診斷欄位**，而且 candidate 必須滿足等價式
            # `candidate == (lifecycle_phase == "CONTINUATION" and not setup_rr_qualified)`
            # ——test double 與真實 contract 不同步的話，測到的是一份不存在的資料形狀。
            is_candidate = (symbol, timeframe, as_of) in state["candidate_keys"]
            structure = "SUPPORT_RECLAIM_CONFIRMED"
            action = "HOLD"
            row = {
                "symbol": symbol, "timeframe": timeframe, "as_of": as_of,
                "lifecycle_phase": "CONTINUATION" if is_candidate else "TESTING",
                "final_entry_state": "NO_ENTRY",
                "clear_zone_breakout": is_candidate,
                "continuation_price_evidence_met": is_candidate,
                "setup_rr_qualified": False,
                "event_signal": "CLOSE_RECLAIM",
                "structure_state": structure,
                "action_state": action,
                "position_action_condition": {"state": action, "structure_state": structure},
                "position_action": action,
            }
            if not state["drop_field"]:
                row["rr_decoupling_candidate"] = is_candidate
            if state["mutate"]:
                state["mutate"](row)
            rows.append(row)
        if state["drop_key"] is not None:
            del rows[state["drop_key"]]
        if state["extra_key"] is not None:
            rows.append(dict(rows[0], as_of=state["extra_key"]))
        if state["duplicate"]:
            rows.append(dict(rows[0]))
        return rows

    monkeypatch.setattr(evaluation_module, "_decision_replay_rows", fake_rows)
    monkeypatch.setattr(evaluation_module, "load_model", lambda path: None)
    monkeypatch.setattr(evaluation_module, "_model_metadata", lambda bundle: {"available": False})
    return state


def _args(bundle_path, output_dir, **overrides):
    base = {
        "bundle": str(bundle_path), "output_dir": str(output_dir), "before_ref": "ecbc141^",
        "image_digest": "sha256:" + "a" * 64, "source_root": "/app",
        "runner_sha256": "c" * 64,
        "base_commit": "0" * 40, "tooling_patch_sha256": "1" * 64,
        "run_id": "test-run", "pipeline_version": "sr_zone_decision_replay_p1",
        "after_artifact": None, "cohort_manifest": None, "report_max_rows": 200,
    }
    base.update(overrides)
    return SimpleNamespace(**base)


def _run_stage1(tmp_path, bundle_path, stub_replay, out="stage1"):
    output = tmp_path / out
    result = run_bundle_stage(_args(bundle_path, output), stage=1, argv=["--bundle"])
    return result, output


# ── Stage 1 ─────────────────────────────────────────────────────────────────

def test_stage1_publishes_after_artifact_and_cohort_manifest(tmp_path, stub_replay):
    bundle = _bundle_with_bars(tmp_path)
    result, output = _run_stage1(tmp_path, bundle, stub_replay)
    assert result["stage"] == 1
    # candidate 範圍是 [min_history_bars, len - forward_bars - 1] ＝ [80, 84] → 5 列
    assert result["rows"] == (90 - 5 - 1) - 80 + 1
    assert result["candidates"] == 0
    after = json.loads((output / AFTER_ARTIFACT_NAME).read_text(encoding="utf-8"))
    cohort = json.loads((output / COHORT_MANIFEST_NAME).read_text(encoding="utf-8"))
    assert after["kind"] == "sr_zone_replay_after"
    assert after["replay_scope"] == "all_candidates"
    assert len(after["rows"]) == result["rows"]
    assert cohort["after_artifact_sha256"] == result["after_artifact_sha256"]
    assert cohort["keys"] == []


def test_empty_cohort_is_legal_when_the_field_is_present(tmp_path, stub_replay):
    """⚠️ 欄位完整、全列為 `false` → **空 cohort 是合法結果**，⛔ 不得判成錯誤。"""
    bundle = _bundle_with_bars(tmp_path)
    result, output = _run_stage1(tmp_path, bundle, stub_replay)
    assert result["candidates"] == 0
    assert (output / COHORT_MANIFEST_NAME).is_file()


def test_missing_candidate_field_aborts_before_publishing(tmp_path, stub_replay):
    """⛔ 缺欄位而**靜默變成空 cohort** 正是要禁止的——而且要在發布之前擋下。"""
    stub_replay["drop_field"] = True
    bundle = _bundle_with_bars(tmp_path)
    output = tmp_path / "stage1"
    with pytest.raises(ArtifactError) as exc:
        run_bundle_stage(_args(bundle, output), stage=1, argv=[])
    assert "I-074 Stage 0" in str(exc.value)
    # ⛔ 不得留下一份看起來可用的 after artifact 或 cohort manifest。
    assert list(output.iterdir()) == []


@pytest.mark.parametrize("bad", [0, 1, "true", None])
def test_non_strict_boolean_candidate_field_aborts(tmp_path, stub_replay, bad):
    """⛔ 不收 `0`／`1`／`"true"`／`None`——bool 是 int 的子類，鬆一點就會放行 `1`。"""
    stub_replay["mutate"] = lambda row: row.__setitem__("rr_decoupling_candidate", bad)
    bundle = _bundle_with_bars(tmp_path)
    with pytest.raises(ArtifactError) as exc:
        run_bundle_stage(_args(bundle, tmp_path / "stage1"), stage=1, argv=[])
    assert "true/false" in str(exc.value) or "I-074" in str(exc.value)


def test_stage1_detects_duplicate_rows(tmp_path, stub_replay):
    stub_replay["duplicate"] = True
    bundle = _bundle_with_bars(tmp_path)
    with pytest.raises(ArtifactError) as exc:
        run_bundle_stage(_args(bundle, tmp_path / "stage1"), stage=1, argv=[])
    assert "重複" in str(exc.value)


def test_stage1_detects_rows_outside_the_bundle_universe(tmp_path, stub_replay):
    stub_replay["extra_key"] = "1999-01-01T00:00:00+00:00"
    bundle = _bundle_with_bars(tmp_path)
    with pytest.raises(ArtifactError) as exc:
        run_bundle_stage(_args(bundle, tmp_path / "stage1"), stage=1, argv=[])
    assert "①" in str(exc.value)


def test_output_dir_must_be_empty(tmp_path, stub_replay):
    bundle = _bundle_with_bars(tmp_path)
    output = tmp_path / "stage1"
    output.mkdir()
    (output / "leftover.json").write_text("{}", encoding="utf-8")
    with pytest.raises(ArtifactError) as exc:
        run_bundle_stage(_args(bundle, output), stage=1, argv=[])
    assert "非空" in str(exc.value)


def test_cohort_manifest_is_published_last(tmp_path, stub_replay, monkeypatch):
    """⚠️ 順序就是契約：cohort manifest 存在即代表 after artifact 已完整落地。"""
    bundle = _bundle_with_bars(tmp_path)
    output = tmp_path / "stage1"
    from ..replay_bundle import artifacts as artifacts_module

    # ⚠️ **spy 在 `os.replace`**——⛔ 不要綁定某一支寫檔函式：after artifact 走
    # `write_canonical_atomic()`（串流，避免 OOM）、cohort manifest 走
    # `publish_artifacts()`，但**兩條路徑最後都以 `os.replace` 原子發布**。
    # 盯住發布動作本身，換實作也不會讓這條契約失去守衛。
    seen: list[tuple[str, bool]] = []
    real = artifacts_module.os.replace

    def spy(src, dst):
        result = real(src, dst)
        name = pathlib.Path(dst).name
        if name in (AFTER_ARTIFACT_NAME, COHORT_MANIFEST_NAME):
            seen.append((name, name != AFTER_ARTIFACT_NAME
                         and (output / AFTER_ARTIFACT_NAME).is_file()))
        return result

    monkeypatch.setattr(artifacts_module.os, "replace", spy)
    run_bundle_stage(_args(bundle, output), stage=1, argv=[])
    assert seen == [(AFTER_ARTIFACT_NAME, False), (COHORT_MANIFEST_NAME, True)]


# ── Stage 2 ─────────────────────────────────────────────────────────────────

def _stage1_then_stage2(tmp_path, stub_replay, candidates=(), **stage2_overrides):
    bundle = _bundle_with_bars(tmp_path)
    stage1_out = tmp_path / "stage1"
    run_bundle_stage(_args(bundle, stage1_out), stage=1, argv=[])
    stage2_out = tmp_path / "stage2"
    args = _args(bundle, stage2_out,
                 after_artifact=str(stage1_out / AFTER_ARTIFACT_NAME),
                 cohort_manifest=str(stage1_out / COHORT_MANIFEST_NAME),
                 **stage2_overrides)
    return bundle, stage1_out, stage2_out, args


def _with_candidates(tmp_path, stub_replay, count=3):
    """先讓 Stage 1 產出帶候選的 after artifact。"""
    bundle = _bundle_with_bars(tmp_path)
    sources_keys = None

    # 先跑一次拿到 universe，再指定前 count 個當候選。
    stage1_probe = tmp_path / "probe"
    run_bundle_stage(_args(bundle, stage1_probe), stage=1, argv=[])
    after = json.loads((stage1_probe / AFTER_ARTIFACT_NAME).read_text(encoding="utf-8"))
    sources_keys = [(r["symbol"], r["timeframe"], r["as_of"]) for r in after["rows"]]
    stub_replay["candidate_keys"] = set(sources_keys[:count])

    stage1_out = tmp_path / "stage1"
    run_bundle_stage(_args(bundle, stage1_out), stage=1, argv=[])
    return bundle, stage1_out, sources_keys[:count]


def test_stage2_produces_comparison_and_report(tmp_path, stub_replay):
    bundle, stage1_out, candidates = _with_candidates(tmp_path, stub_replay)
    stage2_out = tmp_path / "stage2"
    result = run_bundle_stage(
        _args(bundle, stage2_out,
              after_artifact=str(stage1_out / AFTER_ARTIFACT_NAME),
              cohort_manifest=str(stage1_out / COHORT_MANIFEST_NAME)),
        stage=2, argv=[],
    )
    assert result["stage"] == 2
    assert result["candidates"] == len(candidates)
    comparison = json.loads((stage2_out / COMPARISON_ARTIFACT_NAME).read_text(encoding="utf-8"))
    report = json.loads((stage2_out / REPORT_NAME).read_text(encoding="utf-8"))
    assert len(comparison["rows"]) == len(candidates)  # ⛔ 不截斷
    assert report["comparison_artifact_sha256"] == result["comparison_artifact_sha256"]
    assert sha256_hex(canonical_json_bytes(comparison)) == result["comparison_artifact_sha256"]


def test_report_max_rows_truncates_only_the_human_report(tmp_path, stub_replay, monkeypatch):
    """⚠️ 截斷長度來自 **bundle 記下的** `report_max_rows`，⛔ 不是 Stage 2 的參數。"""
    bundle, stage1_out, candidates = _with_candidates(tmp_path, stub_replay, count=4)
    # bundle 的 manifest 是 fixture 產的（report_max_rows=200）；這裡改成 2 來驗它真的生效。
    from ..replay_bundle import bundle as bundle_module

    real_load = bundle_module.load_bundle

    def load_with_small_report(path):
        loaded = real_load(path)
        object.__setattr__(loaded, "manifest", {**loaded.manifest, "report_max_rows": 2})
        return loaded

    monkeypatch.setattr(evaluation_module, "load_bundle", load_with_small_report, raising=False)
    import backtest.modular.sr_scoring.replay_bundle as pkg

    monkeypatch.setattr(pkg, "load_bundle", load_with_small_report)
    stage2_out = tmp_path / "stage2"
    run_bundle_stage(
        _args(bundle, stage2_out,
              after_artifact=str(stage1_out / AFTER_ARTIFACT_NAME),
              cohort_manifest=str(stage1_out / COHORT_MANIFEST_NAME)),
        stage=2, argv=[],
    )
    comparison = json.loads((stage2_out / COMPARISON_ARTIFACT_NAME).read_text(encoding="utf-8"))
    report = json.loads((stage2_out / REPORT_NAME).read_text(encoding="utf-8"))
    assert len(comparison["rows"]) == 4
    assert report["candidate_rows"] == 4 and report["rows_shown"] == 2


def test_stage2_rejects_mismatched_after_artifact_hash(tmp_path, stub_replay):
    bundle, stage1_out, _ = _with_candidates(tmp_path, stub_replay)
    after_path = stage1_out / AFTER_ARTIFACT_NAME
    data = json.loads(after_path.read_text(encoding="utf-8"))
    data["generated_at"] = "1999-01-01T00:00:00+00:00"
    after_path.write_bytes(canonical_json_bytes(data))
    with pytest.raises(ArtifactError) as exc:
        run_bundle_stage(
            _args(bundle, tmp_path / "stage2",
                  after_artifact=str(after_path),
                  cohort_manifest=str(stage1_out / COHORT_MANIFEST_NAME)),
            stage=2, argv=[],
        )
    assert "SHA-256" in str(exc.value)


def test_stage2_rejects_artifacts_from_another_bundle(tmp_path, stub_replay):
    bundle, stage1_out, _ = _with_candidates(tmp_path, stub_replay)
    other = _bundle_with_bars(tmp_path / "other", bars=91)
    with pytest.raises(ArtifactError) as exc:
        run_bundle_stage(
            _args(other, tmp_path / "stage2",
                  after_artifact=str(stage1_out / AFTER_ARTIFACT_NAME),
                  cohort_manifest=str(stage1_out / COHORT_MANIFEST_NAME)),
            stage=2, argv=[],
        )
    assert "bundle" in str(exc.value)


def test_stage2_detects_before_row_drift(tmp_path, stub_replay):
    """②`before 全範圍` 少算一列時，只看 after 與 manifest 完全看不到——所以這道不能省。"""
    bundle, stage1_out, _ = _with_candidates(tmp_path, stub_replay)
    stub_replay["drop_key"] = -1
    with pytest.raises(ArtifactError) as exc:
        run_bundle_stage(
            _args(bundle, tmp_path / "stage2",
                  after_artifact=str(stage1_out / AFTER_ARTIFACT_NAME),
                  cohort_manifest=str(stage1_out / COHORT_MANIFEST_NAME)),
            stage=2, argv=[],
        )
    message = str(exc.value)
    assert "①" in message or "②" in message


def test_stage2_detects_cohort_drift(tmp_path, stub_replay):
    """③cohort manifest 與 after artifact 的候選集合必須一致。"""
    bundle, stage1_out, candidates = _with_candidates(tmp_path, stub_replay)
    cohort_path = stage1_out / COHORT_MANIFEST_NAME
    data = json.loads(cohort_path.read_text(encoding="utf-8"))
    data["keys"] = data["keys"][:-1]
    cohort_path.write_bytes(canonical_json_bytes(data))
    # cohort 改了 → 它記的 after hash 仍然對得上，所以這裡驗到的是 ③ 而不是 hash。
    with pytest.raises(ArtifactError) as exc:
        run_bundle_stage(
            _args(bundle, tmp_path / "stage2",
                  after_artifact=str(stage1_out / AFTER_ARTIFACT_NAME),
                  cohort_manifest=str(cohort_path)),
            stage=2, argv=[],
        )
    assert "③" in str(exc.value)


def test_stage2_rejects_after_artifact_missing_candidate_field(tmp_path, stub_replay):
    """Stage 2 載入時**再驗一次** schema——⛔ 不得重算 predicate 來補。"""
    bundle, stage1_out, _ = _with_candidates(tmp_path, stub_replay)
    after_path = stage1_out / AFTER_ARTIFACT_NAME
    data = json.loads(after_path.read_text(encoding="utf-8"))
    for row in data["rows"]:
        row.pop("rr_decoupling_candidate", None)
    after_path.write_bytes(canonical_json_bytes(data))
    cohort_path = stage1_out / COHORT_MANIFEST_NAME
    cohort = json.loads(cohort_path.read_text(encoding="utf-8"))
    cohort["after_artifact_sha256"] = sha256_hex(after_path.read_bytes())
    cohort_path.write_bytes(canonical_json_bytes(cohort))
    with pytest.raises(ArtifactError) as exc:
        run_bundle_stage(
            _args(bundle, tmp_path / "stage2",
                  after_artifact=str(after_path), cohort_manifest=str(cohort_path)),
            stage=2, argv=[],
        )
    assert "I-074 Stage 0" in str(exc.value)


def test_unknown_artifact_schema_version_aborts(tmp_path, stub_replay):
    bundle, stage1_out, _ = _with_candidates(tmp_path, stub_replay)
    after_path = stage1_out / AFTER_ARTIFACT_NAME
    data = json.loads(after_path.read_text(encoding="utf-8"))
    data["schema_version"] = 99
    after_path.write_bytes(canonical_json_bytes(data))
    with pytest.raises(ArtifactError) as exc:
        run_bundle_stage(
            _args(bundle, tmp_path / "stage2", after_artifact=str(after_path),
                  cohort_manifest=str(stage1_out / COHORT_MANIFEST_NAME)),
            stage=2, argv=[],
        )
    assert "schema_version" in str(exc.value)


def test_report_is_published_last(tmp_path, stub_replay, monkeypatch):
    bundle, stage1_out, _ = _with_candidates(tmp_path, stub_replay)
    from ..replay_bundle import artifacts as artifacts_module

    # ⚠️ 同上：盯 `os.replace` 這個共同的原子發布點。
    stage2_out = tmp_path / "stage2"
    seen: list[tuple[str, bool]] = []
    real = artifacts_module.os.replace

    def spy(src, dst):
        result = real(src, dst)
        name = pathlib.Path(dst).name
        if name in (COMPARISON_ARTIFACT_NAME, REPORT_NAME):
            seen.append((name, name != COMPARISON_ARTIFACT_NAME
                         and (stage2_out / COMPARISON_ARTIFACT_NAME).is_file()))
        return result

    monkeypatch.setattr(artifacts_module.os, "replace", spy)
    run_bundle_stage(
        _args(bundle, stage2_out,
              after_artifact=str(stage1_out / AFTER_ARTIFACT_NAME),
              cohort_manifest=str(stage1_out / COHORT_MANIFEST_NAME)),
        stage=2, argv=[],
    )
    assert seen == [(COMPARISON_ARTIFACT_NAME, False), (REPORT_NAME, True)]


# ── 壞掉的 artifact 必須在 replay **之前**中止 ──────────────────────────────
#
# ⚠️ Stage 2 的 replay 要跑約 3.7 小時。等 replay 跑完才發現 after artifact 是壞的，
# 等於白燒一個下午——而這些檢查全都只看檔案內容，沒有任何一項需要 replay 的輸出。

def _stage2_args_with(tmp_path, stub_replay, mutate_after=None, mutate_cohort=None):
    bundle, stage1_out, _ = _with_candidates(tmp_path, stub_replay)
    after_path = stage1_out / AFTER_ARTIFACT_NAME
    cohort_path = stage1_out / COHORT_MANIFEST_NAME
    if mutate_after is not None:
        data = json.loads(after_path.read_text(encoding="utf-8"))
        mutate_after(data)
        after_path.write_bytes(canonical_json_bytes(data))
        cohort = json.loads(cohort_path.read_text(encoding="utf-8"))
        cohort["after_artifact_sha256"] = sha256_hex(after_path.read_bytes())
        cohort_path.write_bytes(canonical_json_bytes(cohort))
    if mutate_cohort is not None:
        cohort = json.loads(cohort_path.read_text(encoding="utf-8"))
        mutate_cohort(cohort)
        cohort_path.write_bytes(canonical_json_bytes(cohort))
    return _args(bundle, tmp_path / "stage2", after_artifact=str(after_path),
                 cohort_manifest=str(cohort_path))


@pytest.mark.parametrize("mutate_after,mutate_cohort,reason", [
    (lambda d: d.__setitem__("surprise", 1), None, "after 多欄位"),
    (lambda d: d.__setitem__("timeframe", 123), None, "timeframe 不是字串"),
    (lambda d: d.__setitem__("replay_scope", None), None, "replay_scope 是 null"),
    (lambda d: d.__setitem__("generated_at", 0), None, "generated_at 型別錯"),
    (lambda d: d.__setitem__("provenance", "nope"), None, "provenance 型別錯"),
    (lambda d: d.__setitem__("run_id", 7), None, "run_id 型別錯"),
    (lambda d: d["rows"][0].__setitem__("symbol", 2330), None, "symbol 不是字串"),
    (lambda d: d["rows"][0].__setitem__("timeframe", 1), None, "row timeframe 不是字串"),
    (lambda d: d["rows"][0].__setitem__("as_of", 0), None, "as_of 不是字串"),
    (lambda d: d["rows"][0].__setitem__("symbol", ""), None, "symbol 是空字串"),
    (lambda d: d.__setitem__("timeframe", "5m"), None, "timeframe 與 bundle 不一致"),
    (lambda d: d.__setitem__("replay_scope", "sampled"), None, "replay_scope 與 bundle 不一致"),
    (None, lambda d: d.__setitem__("after_artifact_sha256", "abc"), "cohort hash 形狀錯"),
    (None, lambda d: d.__setitem__("generated_at", 1), "cohort generated_at 型別錯"),
    (None, lambda d: d.__setitem__("provenance", []), "cohort provenance 型別錯"),
    (lambda d: d.pop("replay_scope"), None, "after 缺欄位"),
    (lambda d: d.__setitem__("rows", "not-a-list"), None, "rows 不是陣列"),
    (lambda d: d["rows"].append(dict(d["rows"][0])), None, "after 有重複列"),
    (lambda d: [r.pop("rr_decoupling_candidate") for r in d["rows"]], None, "缺候選欄位"),
    (lambda d: d["rows"][0].__setitem__("rr_decoupling_candidate", 1), None, "候選欄位不是 bool"),
    (None, lambda d: d.__setitem__("surprise", 1), "cohort 多欄位"),
    (None, lambda d: d.__setitem__("keys", [["2330", "1d"]]), "cohort key 形狀錯"),
    (None, lambda d: d.__setitem__("keys", d["keys"] + d["keys"][:1]), "cohort 有重複 key"),
])
def test_broken_artifacts_abort_before_replay(tmp_path, stub_replay, monkeypatch,
                                              mutate_after, mutate_cohort, reason):
    args = _stage2_args_with(tmp_path, stub_replay, mutate_after, mutate_cohort)

    def must_not_run(*a, **kw):
        raise AssertionError(f"{reason}：replay 在 artifact 驗證之前就被啟動了")

    monkeypatch.setattr(evaluation_module, "_replay_from_bundle", must_not_run)
    with pytest.raises(ArtifactError):
        run_bundle_stage(args, stage=2, argv=[])


def test_cohort_drift_also_aborts_before_replay(tmp_path, stub_replay, monkeypatch):
    """③ 只看檔案內容，所以它也排在 replay 之前。"""
    args = _stage2_args_with(tmp_path, stub_replay,
                             mutate_cohort=lambda d: d.__setitem__("keys", d["keys"][:-1]))

    def must_not_run(*a, **kw):
        raise AssertionError("③ 在 replay 之前就該中止")

    monkeypatch.setattr(evaluation_module, "_replay_from_bundle", must_not_run)
    with pytest.raises(ArtifactError) as exc:
        run_bundle_stage(args, stage=2, argv=[])
    assert "③" in str(exc.value)


def test_provenance_is_built_after_the_replay(tmp_path, stub_replay, monkeypatch):
    """⚠️ replay 會 lazy import 模組——provenance 太早建就會少記實際跑過的那些檔案。"""
    order: list[str] = []
    from ..replay_bundle import provenance as provenance_module

    real_replay = evaluation_module._replay_from_bundle

    def spy_replay(loaded, **kwargs):
        # ⚠️ `**kwargs`：I-074 Stage 1 讓 `_replay_from_bundle()` 多了 `quota_override`／
        # `on_context`（capacity probe 與 preflight-pre 用），spy 要原樣轉交。
        order.append("replay")
        return real_replay(loaded, **kwargs)

    def spy_provenance(**kwargs):
        order.append("provenance")
        return {"argv": kwargs.get("argv", [])}

    monkeypatch.setattr(evaluation_module, "_replay_from_bundle", spy_replay)
    monkeypatch.setattr(provenance_module, "build_provenance", spy_provenance)
    import backtest.modular.sr_scoring.replay_bundle as pkg

    monkeypatch.setattr(pkg, "build_provenance", spy_provenance)
    bundle = _bundle_with_bars(tmp_path)
    run_bundle_stage(_args(bundle, tmp_path / "stage1"), stage=1, argv=[])
    assert order == ["replay", "provenance"]


# ── 第五道集合檢查：Stage 2 的第二種 terminal outcome ──────────────────────

from ..replay_bundle import (  # noqa: E402
    MISMATCH_ARTIFACT_NAME,
    CandidateMismatch,
)


def _stage2_with_before_candidates(tmp_path, stub_replay, before_extra):
    """先讓 Stage 1 產出固定 cohort，再讓 Stage 2 的 before 側多／少一個候選。"""
    bundle, stage1_out, candidates = _with_candidates(tmp_path, stub_replay, count=2)
    stub_replay["candidate_keys"] = before_extra
    args = _args(bundle, tmp_path / "stage2",
                 after_artifact=str(stage1_out / AFTER_ARTIFACT_NAME),
                 cohort_manifest=str(stage1_out / COHORT_MANIFEST_NAME))
    return args, candidates


def test_before_has_extra_candidate_publishes_mismatch(tmp_path, stub_replay):
    """before 多一個候選 → 發布 mismatch、⛔ 不產 comparison／report。"""
    bundle, stage1_out, candidates = _with_candidates(tmp_path, stub_replay, count=2)
    after = json.loads((stage1_out / AFTER_ARTIFACT_NAME).read_text(encoding="utf-8"))
    all_keys = [(r["symbol"], r["timeframe"], r["as_of"]) for r in after["rows"]]
    # before 側多一個（第三列也算候選）
    stub_replay["candidate_keys"] = set(all_keys[:3])

    stage2_out = tmp_path / "stage2"
    with pytest.raises(CandidateMismatch) as exc:
        run_bundle_stage(
            _args(bundle, stage2_out,
                  after_artifact=str(stage1_out / AFTER_ARTIFACT_NAME),
                  cohort_manifest=str(stage1_out / COHORT_MANIFEST_NAME)),
            stage=2, argv=[],
        )
    assert "分支 C" in str(exc.value)

    mismatch = json.loads((stage2_out / MISMATCH_ARTIFACT_NAME).read_text(encoding="utf-8"))
    assert len(mismatch["before_only"]) == 1 and mismatch["after_only"] == []
    assert mismatch["before_candidate_count"] == 3
    assert mismatch["after_candidate_count"] == 2
    # ⚠️ **綁定的必須是實際載入的那份 after artifact**，⛔ 不只是「同一個 bundle_id」。
    # 少了這一刀，一份指向別份 artifact 的 mismatch 照樣通過，之後就無從判讀分支 C。
    from ..replay_bundle import AFTER_KIND, load_artifact

    _, actual_after_sha = load_artifact(stage1_out / AFTER_ARTIFACT_NAME, AFTER_KIND)
    assert mismatch["after_artifact_sha256"] == actual_after_sha
    # ⛔ 這兩份**不得產出**——這是預期的終止狀態，不是失敗殘骸。
    assert not (stage2_out / COMPARISON_ARTIFACT_NAME).exists()
    assert not (stage2_out / REPORT_NAME).exists()


def test_before_missing_candidate_also_publishes_mismatch(tmp_path, stub_replay):
    bundle, stage1_out, candidates = _with_candidates(tmp_path, stub_replay, count=2)
    stub_replay["candidate_keys"] = set(list(stub_replay["candidate_keys"])[:1])
    stage2_out = tmp_path / "stage2"
    with pytest.raises(CandidateMismatch):
        run_bundle_stage(
            _args(bundle, stage2_out,
                  after_artifact=str(stage1_out / AFTER_ARTIFACT_NAME),
                  cohort_manifest=str(stage1_out / COHORT_MANIFEST_NAME)),
            stage=2, argv=[],
        )
    mismatch = json.loads((stage2_out / MISMATCH_ARTIFACT_NAME).read_text(encoding="utf-8"))
    assert mismatch["after_only"] and not mismatch["before_only"]


def test_matching_cohorts_take_the_normal_path(tmp_path, stub_replay):
    """兩邊相同 → 照常產 comparison／report，⛔ 不產 mismatch。"""
    bundle, stage1_out, _ = _with_candidates(tmp_path, stub_replay, count=2)
    stage2_out = tmp_path / "stage2"
    run_bundle_stage(
        _args(bundle, stage2_out,
              after_artifact=str(stage1_out / AFTER_ARTIFACT_NAME),
              cohort_manifest=str(stage1_out / COHORT_MANIFEST_NAME)),
        stage=2, argv=[],
    )
    assert (stage2_out / COMPARISON_ARTIFACT_NAME).exists()
    assert not (stage2_out / MISMATCH_ARTIFACT_NAME).exists()


def test_mismatch_validator_failure_leaves_no_artifact(tmp_path, stub_replay, monkeypatch):
    """⚠️ validator 失敗 → ⛔ 不留下沒通過驗證的證據，且⛔ 不是 exit 4 的語意。"""
    bundle, stage1_out, _ = _with_candidates(tmp_path, stub_replay, count=2)
    after = json.loads((stage1_out / AFTER_ARTIFACT_NAME).read_text(encoding="utf-8"))
    all_keys = [(r["symbol"], r["timeframe"], r["as_of"]) for r in after["rows"]]
    stub_replay["candidate_keys"] = set(all_keys[:3])

    from ..replay_bundle import artifacts as artifacts_module

    # ⚠️ `**kw`：validator 的三個來源參數是**必填**的，簽章不吃 kwargs 的 stub
    # 會以 TypeError 收場——那就不是在測「validator 拒絕」這條路徑了。
    def _reject(artifact, **kw):
        raise ArtifactError("injected")

    monkeypatch.setattr(artifacts_module, "validate_candidate_mismatch", _reject)
    import backtest.modular.sr_scoring.replay_bundle as pkg

    monkeypatch.setattr(pkg, "validate_candidate_mismatch", _reject)
    stage2_out = tmp_path / "stage2"
    # ⚠️ 是一般中止（ArtifactError），⛔ 不是 CandidateMismatch。
    with pytest.raises(ArtifactError):
        run_bundle_stage(
            _args(bundle, stage2_out,
                  after_artifact=str(stage1_out / AFTER_ARTIFACT_NAME),
                  cohort_manifest=str(stage1_out / COHORT_MANIFEST_NAME)),
            stage=2, argv=[],
        )
    assert not (stage2_out / MISMATCH_ARTIFACT_NAME).exists()


# ── 運算失敗 fail-closed ────────────────────────────────────────────────────

@pytest.mark.parametrize("error_field", ["zone_score_error", "decision_error"])
def test_replay_errors_abort_before_publishing(tmp_path, stub_replay, error_field):
    """⛔ 除 NO_ZONE_SCORES 外，任一運算錯誤都不得發布 artifact。"""
    stub_replay["mutate"] = lambda row: row.__setitem__(error_field, "boom")
    bundle = _bundle_with_bars(tmp_path)
    output = tmp_path / "stage1"
    with pytest.raises(ValueError) as exc:
        run_bundle_stage(_args(bundle, output), stage=1, argv=[])
    assert "運算失敗" in str(exc.value)
    assert list(output.iterdir()) == []


def test_legal_no_zone_rows_do_not_abort(tmp_path, stub_replay):
    """⚠️ 對照組：`NO_ZONE_SCORES` 是**合法零 zone**，⛔ 不是運算失敗。"""
    def mark_no_zone(row):
        row["zone_score_available"] = False
        row["zone_score_error"] = "NO_ZONE_SCORES"
        row["lifecycle_phase"] = None
        row["position_action_condition"] = None
        row["position_action"] = None
        row["event_signal"] = "NO_EVENT"
        row["structure_state"] = "UNKNOWN"
        row["action_state"] = "WATCH"

    stub_replay["mutate"] = mark_no_zone
    bundle = _bundle_with_bars(tmp_path)
    output = tmp_path / "stage1"
    result = run_bundle_stage(_args(bundle, output), stage=1, argv=[])
    assert result["candidates"] == 0
    assert (output / AFTER_ARTIFACT_NAME).is_file()


@pytest.mark.parametrize("error_field", ["zone_score_error", "decision_error"])
def test_stage2_replay_errors_also_abort(tmp_path, stub_replay, error_field):
    """⚠️ **Stage 2 走的是同一條會吞 exception 的 replay 路徑**——before 出錯同樣不得
    產出比較 artifact。⛔ 先前只測了 Stage 1。
    """
    bundle, stage1_out, _ = _with_candidates(tmp_path, stub_replay, count=2)
    stub_replay["mutate"] = lambda row: row.__setitem__(error_field, "boom")
    stage2_out = tmp_path / "stage2"
    with pytest.raises(ValueError) as exc:
        run_bundle_stage(
            _args(bundle, stage2_out,
                  after_artifact=str(stage1_out / AFTER_ARTIFACT_NAME),
                  cohort_manifest=str(stage1_out / COHORT_MANIFEST_NAME)),
            stage=2, argv=[],
        )
    assert "運算失敗" in str(exc.value)
    assert not (stage2_out / COMPARISON_ARTIFACT_NAME).exists()
    assert not (stage2_out / MISMATCH_ARTIFACT_NAME).exists()


def test_zone_present_but_metrics_missing_is_not_a_legal_fallback(tmp_path, monkeypatch):
    """⚠️ **對照組**：zone 在、但 metrics 缺 → 那是**異常**，⛔ 不得套 no-zone fallback。

    ⚠️ **這條走真正的 `_decision_replay_rows()`**（⛔ 沒有用 `stub_replay`）。
    舊版是在 stub 的輸出上手改欄位，那只證明了「這種形狀的 row 會被 validator 擋下」——
    ⛔ 完全沒有執行到產品程式的分支判斷。要是哪天 `elif` 的條件被放寬成「只要沒有
    decision summary 就套 fallback」，錯套的正是這種列，而舊版測試照樣綠。

    這裡改成把 `_historical_zone_score_summary()` 換成一份**完整資料形狀**的回傳：
    zone 確實在（`available=True`、`error=None`、`zone_count=1`），只有 `global_metrics`
    是空的。於是產品程式的兩個分支都不成立，九個診斷欄位維持全 `None`，由 Stage 1 的
    守門中止。
    """
    def zone_present_without_metrics(df, idx, bundle, dataset_config, builder_config):
        return {
            "available": True,
            "error": None,
            "zone_count": 1,
            # ⚠️ 給 None 是刻意的：`_daily_confirmation_outcome()` 在那個 try 之外，
            # 這條要驗的是**診斷欄位走哪個分支**，⛔ 不是下游對 zone 形狀的容忍度。
            "primary_zone": None,
            "global_trend": 0.03,
            "global_volatility": 0.02,
            "global_metrics": {},          # ← 唯一的異常點
            "zone_scores": [],
        }

    monkeypatch.setattr(evaluation_module, "_historical_zone_score_summary",
                        zone_present_without_metrics)
    # ⚠️ `load_model` ⛔ 不能回 None：`bundle is None` 會讓整段 zone 計算被跳過，
    # 那時測到的是「沒有 model」而不是「zone 在、metrics 缺」。
    monkeypatch.setattr(evaluation_module, "load_model", lambda path: SimpleNamespace())
    monkeypatch.setattr(evaluation_module, "_model_metadata", lambda bundle: {"available": False})

    bundle = _bundle_with_bars(tmp_path)
    output = tmp_path / "stage1"
    with pytest.raises(ArtifactError) as exc:
        run_bundle_stage(_args(bundle, output), stage=1, argv=[])

    # ⛔ 九欄位必須維持全 None——由 bool 型別守門抓到，而不是拿到一組看起來正常的預設值。
    assert "必須是 JSON true/false" in str(exc.value)
    assert list(output.iterdir()) == []


def test_real_no_zone_rows_do_get_the_fallback(tmp_path, monkeypatch):
    """⚠️ **上一條的對照組，兩條要一起讀**：真的沒有 zone 時，產品程式**應該**套 fallback。

    少了這一條，上一條會在「fallback 根本沒被實作」時假綠——那時它證明的是
    「這條路徑什麼都沒填」，⛔ 不是「異常列不套 fallback」。
    """
    def truly_no_zone(df, idx, bundle, dataset_config, builder_config):
        return {
            "available": False,
            "error": "NO_ZONE_SCORES",
            "zone_count": 0,
            "primary_zone": None,
            "global_trend": None,
            "global_volatility": None,
            "global_metrics": None,
            "zone_scores": [],
        }

    monkeypatch.setattr(evaluation_module, "_historical_zone_score_summary", truly_no_zone)
    monkeypatch.setattr(evaluation_module, "load_model", lambda path: SimpleNamespace())
    monkeypatch.setattr(evaluation_module, "_model_metadata", lambda bundle: {"available": False})

    bundle = _bundle_with_bars(tmp_path)
    output = tmp_path / "stage1"
    result = run_bundle_stage(_args(bundle, output), stage=1, argv=[])

    assert result["candidates"] == 0
    after = json.loads((output / AFTER_ARTIFACT_NAME).read_text(encoding="utf-8"))
    for row in after["rows"]:
        assert row["zone_score_error"] == "NO_ZONE_SCORES"
        # ⚠️ 逐欄對照契約值——⛔ 不是「有值就好」。
        for field, want in DIAGNOSTIC_NO_ZONE_FALLBACK.items():
            assert row[field] == want, field
        assert row["lifecycle_phase"] is None


def test_replay_primary_zone_follows_decision_primary_even_when_it_is_none(tmp_path, monkeypatch):
    """⚠️ **regression**：decision primary 是 `None` 時，replay row 的 `primary_zone`
    也必須是 `None`——⛔ 不得 fallback 回 `_sort_zone_scores()[0]`。

    守的是 I-074 Stage 0 的雙來源修正。`_pick_primary_zone()` 在**沒有任何非 AT_ZONE
    zone** 時合法回 `None`（單元契約在 `test_decision_engine.py`）。舊寫法
    `if decision_primary is not None: primary_zone = decision_primary` 在那時會**留著
    排序第一筆**，同一列於是混用兩顆 zone，而且⛔ 沒有任何東西會報錯。

    ⚠️ fixture 刻意讓 `zone_summary["primary_zone"]`（＝排序第一筆）**非空**：
    ⛔ 兩邊都是 `None` 的話，這條測試不管實作怎麼寫都會綠。
    """
    sorted_first = {"role": "AT_ZONE", "price_low": 98.0, "price_high": 100.0}

    def zone_summary(df, idx, bundle, dataset_config, builder_config):
        return {
            "available": True, "error": None, "zone_count": 1,
            "primary_zone": sorted_first,          # ← 排序第一筆，**非空**
            "global_trend": 0.03, "global_volatility": 0.02,
            "global_metrics": {"confidence": 0.7}, "zone_scores": [],
        }

    def decision_without_primary(*args, **kwargs):
        """decision 真正看的那顆是 `None`（所有 zone 都是 AT_ZONE 的情形）。"""
        return {
            "primary_zone": None,
            "event_state_summary": {"states": [], "active": [], "resolved": [], "expired": []},
            "position_action_condition": {"state": "WATCH", "structure_state": "UNKNOWN"},
            "position_action": "WATCH",
            "decision_derived_view": {"semantic_pipeline": {
                "lifecycle_phase": "TESTING",
                "clear_zone_breakout": False,
                "continuation_price_evidence_met": False,
                "setup_rr_qualified": False,
                "rr_decoupling_candidate": False,
                "event_signal": "NO_EVENT",
                "action_state": "WATCH",
            }},
        }

    monkeypatch.setattr(evaluation_module, "_historical_zone_score_summary", zone_summary)
    monkeypatch.setattr(evaluation_module, "build_decision_summary", decision_without_primary)
    monkeypatch.setattr(evaluation_module, "load_model", lambda path: SimpleNamespace())
    monkeypatch.setattr(evaluation_module, "_model_metadata", lambda bundle: {"available": False})

    bundle = _bundle_with_bars(tmp_path)
    output = tmp_path / "stage1"
    result = run_bundle_stage(_args(bundle, output), stage=1, argv=[])

    after = json.loads((output / AFTER_ARTIFACT_NAME).read_text(encoding="utf-8"))
    assert after["rows"], "fixture 沒產出任何列，這條測試就沒有在測東西"
    for row in after["rows"]:
        assert row["primary_zone"] is None, (
            "replay row 留了排序第一筆——⛔ 那正是 I-074 要消除的雙來源"
        )
    assert result["candidates"] == 0


# ── canonical 串流寫出：bytes 必須與一次成形的版本逐字相同 ──────────────────

def test_canonical_json_chunks_are_byte_identical():
    """⚠️ **這是整套 artifact 契約的根**：SHA 由這些 bytes 算出來。

    `canonical_json_chunks()` 存在的理由是避免整份 JSON 一次成形（I-074 的 OOM
    成因，實測單次序列化峰值 275 MiB）。⛔ 但它一旦與 `canonical_json_bytes()`
    產出不同的 bytes，所有既有 artifact 的 SHA 都會對不上。
    """
    from ..replay_bundle import canonical_json_bytes, canonical_json_chunks

    cases = [
        {},
        [],
        {"k": []},
        {"b": 1, "a": 2, "C": 3},                      # sort_keys
        {"中文": "值", "nested": {"a": [1, 2.5, None, True]}},
        {"esc": 'quote" back\\ slash/ tab\t nl\n'},
        {"深": {"層": {"巢": [{"狀": 1}]}}},
        {"big": list(range(3000))},
        {"rows": [{"symbol": "2330", "as_of": "2026-09-01", "v": i} for i in range(500)]},
    ]
    for case in cases:
        assert b"".join(canonical_json_chunks(case)) == canonical_json_bytes(case), case


def test_canonical_json_chunks_reject_non_finite():
    """⛔ 兩條路徑都要擋非有限值——⛔ 不能只有一條擋。"""
    from ..replay_bundle import canonical_json_bytes, canonical_json_chunks
    from ..replay_bundle.canonical import CanonicalError

    for produce in (canonical_json_bytes,
                    lambda obj: b"".join(canonical_json_chunks(obj))):
        with pytest.raises(CanonicalError):
            produce({"x": float("nan")})
        with pytest.raises(CanonicalError):
            produce({"x": float("inf")})


def test_write_canonical_atomic_matches_one_shot_sha(tmp_path):
    """串流寫出的 SHA 必須等於「一次成形再算」的 SHA，且檔案內容逐字相同。"""
    from ..replay_bundle import canonical_json_bytes, sha256_hex
    from ..replay_bundle.artifacts import write_canonical_atomic

    payload = {"rows": [{"i": i, "s": "值" * 20} for i in range(2000)], "z": None}
    expected = canonical_json_bytes(payload)
    got_sha = write_canonical_atomic(tmp_path, "x.json", payload)
    assert got_sha == sha256_hex(expected)
    assert (tmp_path / "x.json").read_bytes() == expected
    # ⛔ 不得留下 temp 檔
    assert [p.name for p in tmp_path.iterdir()] == ["x.json"]
