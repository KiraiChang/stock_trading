"""Stage 1／2：從 bundle 載入、四道集合檢查、候選欄位守門與原子發布（I-100 Phase E）。

⚠️ 這一檔用 stub 取代 `_decision_replay_rows`：真的跑 decision engine 需要一份實際模型，
而這裡要驗的是**流程與護欄**（key 集合、欄位守門、發布順序），不是 replay 的數值。
replay 本身的行為由 test_evaluation.py 覆蓋。
"""
from __future__ import annotations

import json
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
            row = {"symbol": symbol, "timeframe": timeframe, "as_of": as_of,
                   "lifecycle_phase": "TESTING", "final_entry_state": "NO_ENTRY"}
            if not state["drop_field"]:
                row["rr_decoupling_candidate"] = (symbol, timeframe, as_of) in state["candidate_keys"]
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

    seen: list[tuple[str, bool]] = []
    real = artifacts_module.write_atomic

    def spy(directory, name, payload):
        seen.append((name, (directory / AFTER_ARTIFACT_NAME).is_file()))
        return real(directory, name, payload)

    monkeypatch.setattr(artifacts_module, "write_atomic", spy)
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

    seen: list[tuple[str, bool]] = []
    real = artifacts_module.write_atomic

    def spy(directory, name, payload):
        seen.append((name, (directory / COMPARISON_ARTIFACT_NAME).is_file()))
        return real(directory, name, payload)

    monkeypatch.setattr(artifacts_module, "write_atomic", spy)
    run_bundle_stage(
        _args(bundle, tmp_path / "stage2",
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

    def spy_replay(loaded):
        order.append("replay")
        return real_replay(loaded)

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
