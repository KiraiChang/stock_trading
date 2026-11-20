"""`db.fetch_candles_range`（I-137）在真的 SQLite 上的語意：範圍段含頭含尾（台北日期）、暖身段、額度、空範圍不查暖身。

日 K 的 ts 是台北 00:00，存成 UTC 是前一天 16:00——範圍的下界是台北 `start_date` 00:00，⛔ 不能拿 UTC 日期比。
"""
from __future__ import annotations

import sqlite3
from datetime import date, datetime, timedelta, timezone

import pytest
from sqlalchemy import create_engine, event, text

import db

TAIPEI = timezone(timedelta(hours=8))


def _ts(d: date) -> str:
    """台北 d 的 00:00 存成 UTC 字串（SQLite 的 DATETIME）。"""
    return datetime.combine(d, datetime.min.time(), tzinfo=TAIPEI).astimezone(timezone.utc).strftime("%Y-%m-%d %H:%M:%S")


@pytest.fixture
def candles(tmp_path, monkeypatch):
    """建 candles 表，`install(days, symbol="2330", adj=1.0)` 寫入台北 00:00 的日 K。回傳查詢紀錄（每一條 SELECT 的參數）。"""
    path = tmp_path / "candles.sqlite"
    sqlite3.connect(str(path)).close()
    engine = create_engine(f"sqlite:///{path}", connect_args={"check_same_thread": False})
    with engine.begin() as conn:
        conn.execute(text("""
            CREATE TABLE candles (
                id INTEGER PRIMARY KEY, symbol TEXT, timeframe TEXT,
                open REAL, high REAL, low REAL, close REAL, volume REAL, amount REAL,
                adj_factor REAL, vol_factor REAL, ts DATETIME
            )
        """))
    queries: list[dict] = []

    @event.listens_for(engine, "before_cursor_execute")
    def _record(conn, cursor, statement, parameters, context, executemany):
        if statement.lstrip().upper().startswith("SELECT") and "FROM candles" in statement:
            queries.append({"sql": statement, "params": parameters})

    monkeypatch.setattr(db, "engine", engine)
    monkeypatch.setattr(db, "DB_DRIVER", "sqlite")

    def install(days: list[date], symbol: str = "2330", adj: float = 1.0) -> None:
        with engine.begin() as conn:
            for i, d in enumerate(days):
                conn.execute(text("""INSERT INTO candles (symbol, timeframe, open, high, low, close, volume, amount,
                                                          adj_factor, vol_factor, ts)
                                     VALUES (:s, '1d', :p, :p, :p, :p, 1000, 1, :a, :a, :ts)"""),
                             {"s": symbol, "p": 100.0 + i, "a": adj, "ts": _ts(d)})

    install.queries = queries
    return install


def _days(start: date, n: int) -> list[date]:
    return [start + timedelta(days=i) for i in range(n)]


def _dates(rows: list[dict]) -> list[str]:
    return [db.epoch_to_taipei_date(r["timestamp"]) for r in rows]


def test_range_is_inclusive_in_taipei_dates_with_warmup(candles):
    candles(_days(date(2025, 1, 1), 20))  # 01-01 ~ 01-20

    out = db.fetch_candles_range("2330", "1d", "2025-01-10", "2025-01-15", warmup_bars=5, budget=100)

    assert _dates(out.in_range) == [f"2025-01-{d:02d}" for d in range(10, 16)]  # 起點、終點當天都含
    assert _dates(out.warmup) == [f"2025-01-{d:02d}" for d in range(5, 10)]  # 起點之前最近 5 根，依時間排序
    assert out.exceeded is False and out.bars_read == 11


def test_start_on_a_holiday_begins_at_next_bar(candles):
    days = [d for d in _days(date(2025, 1, 1), 15) if d.weekday() < 5]  # 只有平日
    candles(days)

    out = db.fetch_candles_range("2330", "1d", "2025-01-04", "2025-01-10", warmup_bars=250, budget=100)  # 01-04 是週六

    assert _dates(out.in_range)[0] == "2025-01-06"
    assert len(out.warmup) == 3  # 01-01 ~ 01-03：歷史不夠 250 根就照實取到的


def test_range_exceeds_budget_stops_at_budget_plus_one_and_skips_warmup(candles):
    candles(_days(date(2025, 1, 1), 30))

    out = db.fetch_candles_range("2330", "1d", "2025-01-10", "2025-01-30", warmup_bars=5, budget=10)

    assert out.exceeded is True
    assert len(out.in_range) == 11 and out.warmup == []  # 下限：查詢在額度 + 1 根就停
    assert len(candles.queries) == 1  # ⛔ 沒有查暖身段


def test_warmup_is_limited_by_remaining_budget(candles):
    """範圍段沒超過、加上暖身才超過：暖身段只取「剩下 + 1」根。"""
    candles(_days(date(2025, 1, 1), 30))

    out = db.fetch_candles_range("2330", "1d", "2025-01-21", "2025-01-30", warmup_bars=250, budget=10)

    assert len(out.in_range) == 10 and out.exceeded is True
    assert len(out.warmup) == 1  # 剩 0 根額度 → 暖身段最多取 1 根，取到就是超過
    assert candles.queries[1]["params"][-1] == 1  # LIMIT 是 min(250, 0 + 1)


def test_exact_budget_passes(candles):
    candles(_days(date(2025, 1, 1), 30))

    out = db.fetch_candles_range("2330", "1d", "2025-01-21", "2025-01-30", warmup_bars=5, budget=15)

    assert (len(out.in_range), len(out.warmup), out.exceeded) == (10, 5, False)


def test_empty_range_does_not_query_warmup(candles):
    """範圍內 0 根：⛔ 不查暖身段（前一檔用完額度時，下一檔不能因為一根舊 K 棒讓任務失敗）。"""
    candles(_days(date(2025, 1, 1), 10))

    out = db.fetch_candles_range("2330", "1d", "2025-03-01", "2025-03-31", warmup_bars=250, budget=0)

    assert out.in_range == [] and out.warmup == [] and out.exceeded is False
    assert len(candles.queries) == 1


def test_max_date_upper_bound_does_not_overflow(candles):
    candles(_days(date(2025, 1, 1), 3))
    out = db.fetch_candles_range("2330", "1d", "2025-01-01", "2099-12-31", warmup_bars=0, budget=100)
    assert len(out.in_range) == 3


def test_adjusted_prices_match_fetch_candles(candles):
    candles(_days(date(2025, 1, 1), 5), adj=0.5)

    out = db.fetch_candles_range("2330", "1d", "2025-01-03", "2025-01-05", warmup_bars=2, budget=100)
    ref = db.fetch_candles("2330", "1d", limit=5)

    got = out.warmup + out.in_range
    assert [r["close"] for r in got] == [r["close"] for r in ref]
    assert [r["volume"] for r in got] == [r["volume"] for r in ref]
    assert got[0]["close"] == pytest.approx(100.0 * 0.5)


def test_other_symbols_are_not_mixed_in(candles):
    candles(_days(date(2025, 1, 1), 5), symbol="2330")
    candles(_days(date(2025, 1, 1), 5), symbol="2317")

    out = db.fetch_candles_range("2330", "1d", "2025-01-01", "2025-01-05", warmup_bars=0, budget=100)

    assert {r["symbol"] for r in out.in_range} == {"2330"} and len(out.in_range) == 5
