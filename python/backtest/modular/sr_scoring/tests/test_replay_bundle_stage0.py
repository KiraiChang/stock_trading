"""Stage 0：唯讀快照、readiness、strict loader 與 bundle 產出（I-100 Phase D）。"""
from __future__ import annotations

import sqlite3
from datetime import date, timedelta

import pytest
from sqlalchemy import create_engine, text

import db as db_module

from .. import evaluation as evaluation_module
from ..evaluation import emit_replay_bundle
from ..replay_bundle import build_calendar_payload, load_bundle
from ..replay_bundle.canonical import canonical_json_bytes


def _calendar_file(tmp_path, years=(2025, 2026)):
    payload = build_calendar_payload(years, {})
    path = tmp_path / "trading_calendar.json"
    path.write_bytes(canonical_json_bytes(payload))
    return path


def _seed_db(tmp_path, symbols=("2330",), bars: int = 100, last_day: str = "2026-09-01"):
    """建一份 sqlite candles，最後一根落在 `last_day`（台北）。"""
    path = tmp_path / "candles.sqlite"
    boot = sqlite3.connect(str(path))
    boot.execute("PRAGMA journal_mode=WAL")
    boot.close()
    engine = create_engine(f"sqlite:///{path}", connect_args={"check_same_thread": False})
    with engine.begin() as conn:
        conn.execute(text("""
            CREATE TABLE candles (
                id INTEGER PRIMARY KEY, symbol TEXT, timeframe TEXT,
                open REAL, high REAL, low REAL, close REAL, volume REAL, amount REAL,
                adj_factor REAL, vol_factor REAL, ts DATETIME
            )
        """))
        conn.execute(text("""
            CREATE TABLE chip_scores (
                symbol TEXT, trade_date TEXT, institutional_score REAL, margin_score REAL,
                broker_score REAL, concentration_score REAL, total_score REAL,
                signal_type TEXT, reason TEXT
            )
        """))
        conn.execute(text("""
            CREATE TABLE stock_sr_model_governance (
                id INTEGER PRIMARY KEY, analyzed_at TEXT, created_at TEXT, symbol TEXT,
                timeframe TEXT, model_version TEXT, model_config_hash TEXT, health_state TEXT,
                average_edge_pp REAL, directional_zone_count INTEGER, zone_count INTEGER,
                allow_entry INTEGER, max_entry_state TEXT, quality_flags TEXT,
                warning_flags TEXT, blocking_flags TEXT, confidence_gate_json TEXT,
                calibration_report_json TEXT, walk_forward_report_json TEXT,
                dataset_diagnostics_json TEXT, governance_json TEXT
            )
        """))
        end = date.fromisoformat(last_day)
        for symbol in symbols:
            for i in range(bars):
                day = end - timedelta(days=bars - 1 - i)
                conn.execute(
                    text("""INSERT INTO candles (symbol, timeframe, open, high, low, close,
                                                 volume, amount, adj_factor, vol_factor, ts)
                            VALUES (:s,'1d',:o,:h,:l,:c,1000,10000,1,1,:ts)"""),
                    {"s": symbol, "o": 100 + i, "h": 101 + i, "l": 99 + i, "c": 100.5 + i,
                     "ts": f"{day.isoformat()} 05:30:00"},
                )
            conn.execute(
                text("""INSERT INTO chip_scores (symbol, trade_date, total_score, signal_type)
                        VALUES (:s, :d, 55.0, 'NEUTRAL')"""),
                {"s": symbol, "d": end.isoformat()},
            )
    return engine


@pytest.fixture
def stage0_env(tmp_path, monkeypatch):
    engine = _seed_db(tmp_path)
    monkeypatch.setattr(db_module, "engine", engine)
    monkeypatch.setattr(db_module, "DB_DRIVER", "sqlite")
    model = tmp_path / "model.joblib"
    model.write_bytes(b"fake-model")
    return {
        "engine": engine,
        "model": model,
        "calendar": _calendar_file(tmp_path),
        "baselines": tmp_path / "baselines",
    }


def _emit(env, **overrides):
    kwargs = {
        "symbols": ["2330"],
        "timeframe": "1d",
        "limit": 1500,
        "as_of": "2026-09-01",
        "model_path": str(env["model"]),
        "baselines_dir": str(env["baselines"]),
        "image_digest": "sha256:" + "a" * 64,
        "source_root": "/app",
        "runner_sha256": "c" * 64,
        "trading_calendar_path": str(env["calendar"]),
        "argv": ["--as-of", "2026-09-01"],
        "log": lambda message: None,
    }
    kwargs.update(overrides)
    return emit_replay_bundle(**kwargs)


def test_stage0_publishes_a_loadable_bundle(stage0_env):
    outcome = _emit(stage0_env)
    assert outcome.published is True
    loaded = load_bundle(outcome.path)
    assert loaded.manifest["as_of"] == "2026-09-01"
    assert loaded.manifest["symbols"] == ["2330"]
    assert loaded.manifest["replay_scope"] == "all_candidates"
    assert loaded.manifest["readiness"] == {
        "timeframe": "1d", "expected_latest": "2026-09-01", "market_latest": "2026-09-01",
    }
    assert loaded.manifest["calendar"]["mode"] == "frozen"
    assert len(loaded.candles["2330"]) == 100
    assert loaded.chip["2330"][0]["total_score"] == 55.0
    assert loaded.model_path.read_bytes() == b"fake-model"
    assert loaded.replay_config["replay_scope"] == "all_candidates"


def test_stage0_is_deterministic_apart_from_captured_at(stage0_env):
    first = _emit(stage0_env)
    second = _emit(stage0_env)
    assert second.published is False  # 同 payload → no-op
    assert first.bundle_id == second.bundle_id


def test_stage0_never_calls_the_replay_engine(stage0_env, monkeypatch):
    """⛔ Stage 0 不得跑 replay——它只讀 DB 並封存輸入。"""
    def boom(*args, **kwargs):
        raise AssertionError("Stage 0 呼叫了 replay engine")

    monkeypatch.setattr(evaluation_module, "_decision_replay_rows", boom)
    monkeypatch.setattr(evaluation_module, "run_decision_replay", boom)
    _emit(stage0_env)


def test_as_of_clamps_the_tail(stage0_env):
    """as-of 往前釘 → bundle 只含到那一天為止的列。"""
    outcome = _emit(stage0_env, as_of="2026-08-30")
    loaded = load_bundle(outcome.path)
    rows = loaded.candles["2330"]
    assert len(rows) == 98
    assert loaded.manifest["as_of"] == "2026-08-30"


# ── readiness ───────────────────────────────────────────────────────────────

def test_as_of_on_a_weekend_passes(tmp_path, monkeypatch):
    """`as_of` 落在週末：`expected_latest` 退回前一個交易日 → **應通過**。

    ⚠️ v7 的公式 `as_of > market_latest → 中止` 與這條測試直接矛盾——週六 > 週五必然成立。
    """
    # 資料只到 2026-09-04（週五），as_of 給 2026-09-05（週六）。
    engine = _seed_db(tmp_path, last_day="2026-09-04")
    monkeypatch.setattr(db_module, "engine", engine)
    monkeypatch.setattr(db_module, "DB_DRIVER", "sqlite")
    model = tmp_path / "m.joblib"
    model.write_bytes(b"m")
    env = {"model": model, "calendar": _calendar_file(tmp_path), "baselines": tmp_path / "b"}
    outcome = _emit(env, as_of="2026-09-05")
    readiness = load_bundle(outcome.path).manifest["readiness"]
    assert readiness["expected_latest"] == "2026-09-04"
    assert readiness["market_latest"] == "2026-09-04"


def test_market_data_behind_expected_latest_aborts(stage0_env):
    """`market_latest < expected_latest` → **應中止**。"""
    with pytest.raises(ValueError) as exc:
        _emit(stage0_env, as_of="2026-09-04")
    assert "資料還沒到齊" in str(exc.value)
    assert not stage0_env["baselines"].exists() or not any(stage0_env["baselines"].iterdir())


def test_delisted_symbol_with_older_last_bar_still_passes(tmp_path, monkeypatch):
    """某檔最後交易日早於 `as_of`（下市／停牌）→ **應通過**且該檔照常入 bundle。

    ⛔ 逐檔判「最後一根必須等於 as_of」對這類標的永遠不成立。
    """
    engine = _seed_db(tmp_path, symbols=("2330",), last_day="2026-09-01")
    with engine.begin() as conn:
        for i in range(100):
            day = date(2026, 9, 1) - timedelta(days=199 - i)
            conn.execute(
                text("""INSERT INTO candles (symbol, timeframe, open, high, low, close,
                                             volume, amount, adj_factor, vol_factor, ts)
                        VALUES ('9999','1d',10,11,9,10.5,100,1000,1,1,:ts)"""),
                {"ts": f"{day.isoformat()} 05:30:00"},
            )
    monkeypatch.setattr(db_module, "engine", engine)
    monkeypatch.setattr(db_module, "DB_DRIVER", "sqlite")
    model = tmp_path / "m.joblib"
    model.write_bytes(b"m")
    env = {"model": model, "calendar": _calendar_file(tmp_path), "baselines": tmp_path / "b"}
    outcome = _emit(env, symbols=["2330", "9999"])
    loaded = load_bundle(outcome.path)
    assert set(loaded.candles) == {"2330", "9999"}
    assert loaded.candles["9999"][-1]["timestamp"] < loaded.candles["2330"][-1]["timestamp"]


def test_symbol_without_any_candle_aborts(stage0_env):
    with pytest.raises(ValueError) as exc:
        _emit(stage0_env, symbols=["2330", "0000"])
    assert "沒有任何 candle" in str(exc.value)


def test_symbol_with_too_few_bars_aborts(tmp_path, monkeypatch):
    engine = _seed_db(tmp_path, bars=40)
    monkeypatch.setattr(db_module, "engine", engine)
    monkeypatch.setattr(db_module, "DB_DRIVER", "sqlite")
    model = tmp_path / "m.joblib"
    model.write_bytes(b"m")
    with pytest.raises(ValueError) as exc:
        _emit({"model": model, "calendar": _calendar_file(tmp_path), "baselines": tmp_path / "b"})
    assert "歷史根數不足" in str(exc.value)


def test_missing_model_file_aborts(stage0_env, tmp_path):
    with pytest.raises(ValueError) as exc:
        _emit(stage0_env, model_path=str(tmp_path / "nope.joblib"))
    assert "model 檔不存在" in str(exc.value)


# ── strict：六個 fail-open 分支 ＋ readiness ＋ candles ─────────────────────

@pytest.mark.parametrize("branch", [
    "chip_range_missing", "chip_import", "chip_query",
    "governance_range_missing", "governance_import", "governance_query",
    "readiness", "candles",
])
def test_strict_branches_abort_without_publishing(stage0_env, monkeypatch, branch):
    """八條：六個 fail-open 分支 ＋ readiness ＋ candles 中途失敗。

    每一條都要①非零結束（這裡是 raise）、②`rollback` 被呼叫、
    ③正式 bundle 目錄不存在且沒有殘留可載入的 temp。
    """
    rolled_back: list[str] = []
    real_snapshot = db_module.readonly_snapshot

    import contextlib

    @contextlib.contextmanager
    def watching_snapshot(*args, **kwargs):
        """在**同一個 connection 物件**上掛記錄器——`readonly_snapshot` 的 `finally`
        用的就是它，包一層 proxy 反而看不到收尾的 ROLLBACK。"""
        with real_snapshot(*args, **kwargs) as conn:
            original = conn.exec_driver_sql

            def recording(sql, *a, **kw):
                if "ROLLBACK" in str(sql):
                    rolled_back.append(str(sql))
                return original(sql, *a, **kw)

            conn.exec_driver_sql = recording
            yield conn

    monkeypatch.setattr(db_module, "readonly_snapshot", watching_snapshot)

    def boom(*args, **kwargs):
        raise RuntimeError(f"injected {branch}")

    if branch == "readiness":
        monkeypatch.setattr(db_module, "fetch_market_latest_trading_date", boom)
    elif branch == "candles":
        monkeypatch.setattr(evaluation_module, "_load_db_candle_rows", boom)
    elif branch == "chip_range_missing":
        monkeypatch.setattr(evaluation_module, "_date_range_for_context", lambda *a: None)
    elif branch == "governance_range_missing":
        monkeypatch.setattr(evaluation_module, "_dataset_range", lambda sources: (None, None))
    elif branch in ("chip_import", "governance_import"):
        import builtins

        real_import = builtins.__import__

        def failing_import(name, *args, **kwargs):
            if name == "db" and args and args[2] and (
                ("fetch_chip_scores" in args[2] and branch == "chip_import")
                or ("fetch_sr_model_governance" in args[2] and branch == "governance_import")
            ):
                raise ImportError(f"injected {branch}")
            return real_import(name, *args, **kwargs)

        monkeypatch.setattr(builtins, "__import__", failing_import)
    elif branch == "chip_query":
        monkeypatch.setattr(db_module, "fetch_chip_scores", boom)
    else:
        monkeypatch.setattr(db_module, "fetch_sr_model_governance", boom)

    with pytest.raises(Exception):
        _emit(stage0_env)

    baselines = stage0_env["baselines"]
    assert not baselines.exists() or not any(
        p for p in baselines.iterdir() if not p.name.startswith(".probe-")
    )
    if branch != "readiness":
        assert rolled_back, "rollback 沒有被呼叫"


def test_legal_zero_chip_rows_still_produces_a_bundle(tmp_path, monkeypatch):
    """對照組：某檔 chip 為**合法零筆** → 正常產出 bundle，⛔ 不得因此中止。"""
    engine = _seed_db(tmp_path)
    with engine.begin() as conn:
        conn.execute(text("DELETE FROM chip_scores"))
    monkeypatch.setattr(db_module, "engine", engine)
    monkeypatch.setattr(db_module, "DB_DRIVER", "sqlite")
    model = tmp_path / "m.joblib"
    model.write_bytes(b"m")
    outcome = _emit({"model": model, "calendar": _calendar_file(tmp_path), "baselines": tmp_path / "b"})
    loaded = load_bundle(outcome.path)
    assert loaded.chip == {}


def test_existing_valid_bundle_survives_a_later_failure(stage0_env, monkeypatch):
    """執行前已有同 ID 有效 bundle，之後某次失敗 → 既有目錄逐位元不變且仍可載入。"""
    outcome = _emit(stage0_env)
    snapshot = {p.name: p.read_bytes() for p in outcome.path.iterdir()}

    def boom(*args, **kwargs):
        raise RuntimeError("injected chip failure")

    monkeypatch.setattr(db_module, "fetch_chip_scores", boom)
    with pytest.raises(Exception):
        _emit(stage0_env)
    assert {p.name: p.read_bytes() for p in outcome.path.iterdir()} == snapshot
    assert load_bundle(outcome.path).bundle_id == outcome.bundle_id
