"""bundle 身分、決定性與 loader 的三方相等契約（I-100 Phase A）。"""
from __future__ import annotations

import json

import pytest

from ..replay_bundle import (
    MANIFEST_NAME,
    MANIFEST_SHA_NAME,
    PAYLOAD_FILES,
    BundleError,
    build_manifest,
    compute_bundle_id,
    emit_bundle,
    load_bundle,
    render_payloads,
)
from ..replay_bundle.canonical import canonical_json_bytes, sha256_hex
from ..replay_bundle import emit_bundle
from .replay_bundle_fixtures import (
    make_manifest_and_payloads,
    make_payload_inputs,
    make_replay_config,
)


def _publish(tmp_path, **kwargs):
    manifest, payloads = make_manifest_and_payloads(**kwargs)
    return emit_bundle(tmp_path, payloads, manifest).path, manifest


# ── 決定性 ──────────────────────────────────────────────────────────────────

def test_same_input_twice_gives_bit_identical_payload_and_id():
    first = render_payloads(**make_payload_inputs())
    second = render_payloads(**make_payload_inputs())
    assert first == second
    m1, m2 = make_manifest_and_payloads()[0], make_manifest_and_payloads()[0]
    assert m1["bundle_id"] == m2["bundle_id"]
    assert m1["files"] == m2["files"]


def test_only_volatile_fields_may_differ_between_runs():
    """⚠️ `captured_at` 與 calendar provenance 的時間欄位以外，其餘必須完全相同。"""
    a, _ = make_manifest_and_payloads(captured_at="2026-09-01T00:00:00+00:00")
    b, _ = make_manifest_and_payloads(captured_at="2026-09-02T00:00:00+00:00")
    differing = {k for k in set(a) | set(b) if a.get(k) != b.get(k)}
    assert differing == {"captured_at", "calendar"}
    assert a["calendar"]["mode"] == b["calendar"]["mode"] == "online"


def test_payload_row_order_is_independent_of_input_order():
    """排序鍵決定順序，⛔ 不是輸入順序——否則同一份資料會有兩個 bundle_id。"""
    base = make_payload_inputs()
    reversed_candles = {"2330": list(reversed(base["candles_by_symbol"]["2330"]))}
    assert render_payloads(**base) == render_payloads(
        **make_payload_inputs(candles_by_symbol=reversed_candles)
    )


def test_row_bytes_tie_breaker_orders_rows_with_equal_sort_keys():
    """排序鍵完全相同的兩列，靠**整列 canonical JSON bytes** 決定順序。"""
    rows = [
        {"trade_date": "2026-09-01", "total_score": 9.0},
        {"trade_date": "2026-09-01", "total_score": 1.0},
    ]
    forward = render_payloads(**make_payload_inputs(chip_by_symbol={"2330": rows}))
    backward = render_payloads(**make_payload_inputs(chip_by_symbol={"2330": list(reversed(rows))}))
    assert forward == backward


def test_bundle_id_changes_when_any_payload_changes():
    a, _ = make_manifest_and_payloads()
    b, _ = make_manifest_and_payloads(
        replay_config=make_replay_config(dataset_config={"min_history_bars": 81})
    )
    assert a["bundle_id"] != b["bundle_id"]


def test_bundle_id_format():
    manifest, _ = make_manifest_and_payloads()
    assert __import__("re").fullmatch(
        r"b[0-9]+_[0-9]{8}_[0-9a-z]+_[0-9a-f]{8}_[0-9a-f]{8}", manifest["bundle_id"]
    )


def test_compute_bundle_id_rejects_bad_as_of_and_timeframe():
    files = {name: "a" * 64 for name in PAYLOAD_FILES}
    with pytest.raises(BundleError):
        compute_bundle_id(schema_version=1, as_of="2026/09/01", timeframe="1d",
                          symbols=["2330"], file_hashes=files)
    with pytest.raises(BundleError):
        compute_bundle_id(schema_version=1, as_of="2026-09-01", timeframe="1D",
                          symbols=["2330"], file_hashes=files)


def test_build_manifest_rejects_wrong_payload_file_set():
    payloads = render_payloads(**make_payload_inputs())
    payloads.pop("chip.json.gz")
    with pytest.raises(BundleError):
        build_manifest(payloads=payloads, as_of="2026-09-01", timeframe="1d", symbols=["2330"],
                       limit=1500, replay_scope="all_candidates", report_max_rows=200,
                       captured_at="2026-09-01T00:00:00+00:00", readiness={}, calendar={},
                       provenance={})


# ── 語意等價：載入後的值與擷取前相同 ────────────────────────────────────────

def test_loader_round_trips_values_and_types(tmp_path):
    path, _ = _publish(tmp_path)
    loaded = load_bundle(path)
    rows = loaded.candles["2330"]
    assert [r["timestamp"] for r in rows] == [1756684800, 1756771200]
    assert all(isinstance(r["close"], float) for r in rows)
    assert loaded.chip["2330"][0]["total_score"] == 55.0
    assert loaded.governance["2330"][0]["model_config_hash"] == "abc"
    assert loaded.replay_config["replay_scope"] == "all_candidates"
    assert loaded.trading_calendar["covered_years"] == [2025, 2026]
    assert loaded.model_path.read_bytes() == b"fake-model-bytes"


# ── loader 的完整性與三方相等 ───────────────────────────────────────────────

@pytest.mark.parametrize("name", sorted(PAYLOAD_FILES))
def test_tampering_any_payload_is_caught(tmp_path, name):
    """含 `trading_calendar.json`——竄改日曆必須被 loader 抓到。"""
    path, _ = _publish(tmp_path)
    target = path / name
    target.write_bytes(target.read_bytes() + b"\x00")
    with pytest.raises(BundleError) as exc:
        load_bundle(path)
    assert name in str(exc.value)


def test_renamed_bundle_directory_is_rejected(tmp_path):
    path, manifest = _publish(tmp_path)
    moved = path.parent / (manifest["bundle_id"].replace("_1d_", "_1m_"))
    path.rename(moved)
    with pytest.raises(BundleError) as exc:
        load_bundle(moved)
    assert "三方相等契約" in str(exc.value)


def _rewrite_manifest(path, mutate):
    manifest = json.loads((path / MANIFEST_NAME).read_text(encoding="utf-8"))
    mutate(manifest)
    raw = canonical_json_bytes(manifest)
    (path / MANIFEST_NAME).write_bytes(raw)
    (path / MANIFEST_SHA_NAME).write_text(sha256_hex(raw) + "\n", encoding="utf-8")


@pytest.mark.parametrize("mutate", [
    pytest.param(lambda m: m.__setitem__("bundle_id", m["bundle_id"][:-1] + "0"), id="bundle_id"),
    pytest.param(lambda m: m.__setitem__("as_of", "2026-08-31"), id="as_of"),
    pytest.param(lambda m: m.__setitem__("timeframe", "5m"), id="timeframe"),
    pytest.param(lambda m: m.__setitem__("symbols", ["2330", "2454"]), id="symbols_added"),
    pytest.param(lambda m: m.__setitem__("symbols", []), id="symbols_removed"),
])
def test_identity_field_tampering_is_rejected(tmp_path, mutate):
    """⚠️ 逐檔 hash 只證明 payload 沒被動過，證明不了「這份 payload 就是這個身分」。"""
    path, _ = _publish(tmp_path)
    _rewrite_manifest(path, mutate)
    with pytest.raises(BundleError):
        load_bundle(path)


def test_manifest_sha_mismatch_is_rejected(tmp_path):
    path, _ = _publish(tmp_path)
    (path / MANIFEST_SHA_NAME).write_text("0" * 64 + "\n", encoding="utf-8")
    with pytest.raises(BundleError) as exc:
        load_bundle(path)
    assert "SHA-256 不符" in str(exc.value)


def test_unknown_manifest_schema_version_is_rejected(tmp_path):
    path, _ = _publish(tmp_path)
    _rewrite_manifest(path, lambda m: m.__setitem__("schema_version", 2))
    with pytest.raises(BundleError) as exc:
        load_bundle(path)
    assert "schema_version" in str(exc.value)


def test_manifest_unknown_or_missing_field_is_rejected(tmp_path):
    path, _ = _publish(tmp_path)
    _rewrite_manifest(path, lambda m: m.__setitem__("surprise", 1))
    with pytest.raises(BundleError):
        load_bundle(path)

    path2, _ = _publish(tmp_path, as_of="2026-08-31")
    _rewrite_manifest(path2, lambda m: m.pop("limit"))
    with pytest.raises(BundleError):
        load_bundle(path2)


def test_extra_file_in_bundle_directory_is_rejected(tmp_path):
    """多出來的檔案不進 `files` mapping → 不進 content_hash8，只能靠目錄清單擋。"""
    path, _ = _publish(tmp_path)
    (path / "notes.txt").write_text("hi", encoding="utf-8")
    with pytest.raises(BundleError) as exc:
        load_bundle(path)
    assert "目錄內容不符" in str(exc.value)


def test_missing_payload_file_is_rejected(tmp_path):
    path, _ = _publish(tmp_path)
    (path / "chip.json.gz").unlink()
    with pytest.raises(BundleError):
        load_bundle(path)


# ── loader 也要驗內容，不只驗 hash ──────────────────────────────────────────

def _tweaked_calendar(**changes):
    from .replay_bundle_fixtures import MINIMAL_CALENDAR

    calendar = json.loads(json.dumps(MINIMAL_CALENDAR))
    calendar.update(changes)
    return calendar


def test_loader_rejects_a_calendar_that_fails_schema(tmp_path):
    """⚠️ contract 是「builder 與 loader 兩端都驗」：只驗檔案 hash 只證明「進來時是什麼
    就是什麼」，證明不了那份內容本身合法——手捏的片段日曆就會這樣溜進來。

    （`emit_bundle` 在 staging 階段就用正式 loader 驗過，所以這裡發布就會被擋下。）
    """
    calendar = _tweaked_calendar(covered_years=[2025])
    calendar["days"] = calendar["days"][:1]          # ⛔ 不是完整年度
    manifest, payloads = make_manifest_and_payloads(trading_calendar=calendar)
    with pytest.raises(BundleError) as exc:
        emit_bundle(tmp_path, payloads, manifest)
    assert "交易日曆" in str(exc.value)


def test_loader_rejects_row_type_and_flag_contradiction(tmp_path):
    calendar = _tweaked_calendar()
    calendar["days"][0]["is_trading_day"] = not calendar["days"][0]["is_trading_day"]
    manifest, payloads = make_manifest_and_payloads(trading_calendar=calendar)
    with pytest.raises(BundleError) as exc:
        emit_bundle(tmp_path, payloads, manifest)
    assert "矛盾" in str(exc.value)


def test_loader_rejects_replay_config_inconsistent_with_manifest(tmp_path):
    """⚠️ Stage 1／2 讀 replay_config 決定怎麼算，卻用 manifest 的欄位對外宣稱身分——
    兩份不一致等於算的和說的不是同一件事。
    """
    bad = make_replay_config(as_of="2026-08-31")   # manifest 是 2026-09-01
    manifest, payloads = make_manifest_and_payloads(replay_config=bad)
    with pytest.raises(BundleError) as exc:
        emit_bundle(tmp_path, payloads, manifest)
    assert "不一致" in str(exc.value)


def test_loader_rejects_unknown_replay_config_field(tmp_path):
    bad = {**make_replay_config(), "surprise": 1}
    manifest, payloads = make_manifest_and_payloads(replay_config=bad)
    with pytest.raises(BundleError) as exc:
        emit_bundle(tmp_path, payloads, manifest)
    assert "replay_config.json 欄位不符" in str(exc.value)


@pytest.mark.parametrize("field", ["content_hash8", "symbols_hash8"])
def test_loader_rejects_tampered_hash_fields(tmp_path, field):
    """只驗 bundle_id 的話，這兩格被改掉不會有任何東西報錯——但人在讀 manifest 時會直接引用它們。"""
    path, _ = _publish(tmp_path)
    _rewrite_manifest(path, lambda m: m.__setitem__(field, "deadbeef"))
    with pytest.raises(BundleError) as exc:
        load_bundle(path)
    assert field in str(exc.value)
