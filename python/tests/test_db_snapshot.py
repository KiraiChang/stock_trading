"""`db` 的 as-of 上界、optional `conn` 與唯讀快照（I-100 Phase C）。

**自動化的界線**（計畫書十二-B）：三個 driver 的**語句與順序**在這裡斷言；
sqlite 的**實際**唯讀強制、`rollback`、`query_only` 復原與 concurrent-writer 快照也在這裡。
⛔ PostgreSQL／MySQL 的**實機**一致性快照不在 CI 內——python 測試容器不連那兩者，
驗證步驟列在 docs/development-workflow.md 的手動章節。
"""
from __future__ import annotations

import sqlite3
from datetime import date, datetime, timedelta, timezone

import pytest
from sqlalchemy import create_engine, text

import db


TAIPEI = timezone(timedelta(hours=8))


# ── as-of 上界 ──────────────────────────────────────────────────────────────

def test_as_of_cutoff_is_next_taipei_midnight():
    cutoff = db.as_of_cutoff_epoch("2026-09-01")
    assert datetime.fromtimestamp(cutoff, tz=TAIPEI) == datetime(2026, 9, 2, tzinfo=TAIPEI)


def test_as_of_cutoff_accepts_date_object_and_rejects_junk():
    assert db.as_of_cutoff_epoch(date(2026, 9, 1)) == db.as_of_cutoff_epoch("2026-09-01")
    with pytest.raises(ValueError):
        db.as_of_cutoff_epoch("2026/09/01")


def test_epoch_to_taipei_date_handles_none():
    assert db.epoch_to_taipei_date(None) is None
    # 台北的 00:30 在 UTC 還是前一天——這正是「⛔ 不直接拿 date 去比 timestamp」的理由。
    epoch = datetime(2026, 9, 1, 0, 30, tzinfo=TAIPEI).timestamp()
    assert db.epoch_to_taipei_date(epoch) == "2026-09-01"
    assert db.epoch_to_taipei_date(epoch) != datetime.fromtimestamp(
        epoch, tz=timezone.utc
    ).date().isoformat()


def _sqlite_engine(tmp_path, rows):
    path = tmp_path / "candles.sqlite"
    # ⚠️ WAL 必須在交易之外設定，否則 `PRAGMA journal_mode` 靜默無效——
    # 而 concurrent-writer 測試就是要驗 WAL 的快照語意。
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
        for i, ts in enumerate(rows):
            conn.execute(
                text("""INSERT INTO candles (symbol, timeframe, open, high, low, close,
                                             volume, amount, adj_factor, vol_factor, ts)
                        VALUES ('2330','1d',1,2,0.5,1.5,10,100,1,1,:ts)"""),
                {"ts": ts},
            )
    return engine


# 台北 2026-08-31 / 09-01 / 09-02 的收盤（13:30 台北 ＝ 05:30 UTC）
_TS = ["2026-08-31 05:30:00", "2026-09-01 05:30:00", "2026-09-02 05:30:00"]


@pytest.mark.parametrize("as_of,expected_count", [
    ("2026-08-31", 1),   # 邊界：含當日
    ("2026-09-01", 2),   # 邊界：含當日，⛔ 不含次日
    ("2026-09-02", 3),
])
def test_as_of_boundaries_on_sqlite(tmp_path, monkeypatch, as_of, expected_count):
    engine = _sqlite_engine(tmp_path, _TS)
    monkeypatch.setattr(db, "engine", engine)
    monkeypatch.setattr(db, "DB_DRIVER", "sqlite")
    rows = db.fetch_candles("2330", "1d", limit=100, as_of=as_of)
    assert len(rows) == expected_count


def test_without_as_of_all_rows_are_returned(tmp_path, monkeypatch):
    """⛔ 不改既有預設行為：不傳 `as_of` 就是現行語意。"""
    engine = _sqlite_engine(tmp_path, _TS)
    monkeypatch.setattr(db, "engine", engine)
    monkeypatch.setattr(db, "DB_DRIVER", "sqlite")
    assert len(db.fetch_candles("2330", "1d", limit=100)) == 3


def test_market_latest_trading_date(tmp_path, monkeypatch):
    engine = _sqlite_engine(tmp_path, _TS)
    monkeypatch.setattr(db, "engine", engine)
    monkeypatch.setattr(db, "DB_DRIVER", "sqlite")
    assert db.fetch_market_latest_trading_date("1d") == "2026-09-02"
    assert db.fetch_market_latest_trading_date("5m") is None


# ── 三個 driver 的語句與順序 ────────────────────────────────────────────────

class _RecordingConn:
    def __init__(self, log, dbapi=None):
        self.log = log
        self.closed = False
        self._dbapi = dbapi

    def execution_options(self, **kwargs):
        self.log.append(("execution_options", kwargs))
        return self

    def begin(self):
        self.log.append(("begin", None))

    def exec_driver_sql(self, sql):
        self.log.append(("sql", " ".join(str(sql).split())))

    def execute(self, sql, params=None):
        self.log.append(("query", " ".join(str(sql).split())))
        raise AssertionError("這一組測試不該真的查資料")

    def close(self):
        self.closed = True

    @property
    def connection(self):
        return self._dbapi


class _FakeEngine:
    def __init__(self, conn):
        self._conn = conn

    def connect(self):
        return self._conn


def _run_snapshot(monkeypatch, driver, dbapi=None):
    log: list = []
    conn = _RecordingConn(log, dbapi=dbapi)
    monkeypatch.setattr(db, "DB_DRIVER", driver)
    with db.readonly_snapshot(engine_override=_FakeEngine(conn)) as handle:
        assert handle is conn
    return log, conn


def test_postgres_statement_order(monkeypatch):
    """PG：`execution_options` → `BEGIN` → `SET TRANSACTION READ ONLY`，全在第一個 SELECT 之前。"""
    log, conn = _run_snapshot(monkeypatch, "postgres")
    assert log[0] == ("execution_options", {"isolation_level": "REPEATABLE READ"})
    assert log[1] == ("begin", None)
    assert log[2] == ("sql", "SET TRANSACTION READ ONLY")
    assert log[-1] == ("sql", "ROLLBACK")
    assert conn.closed


def test_mysql_statement_order_sets_isolation_explicitly(monkeypatch):
    """⛔ 只送 `START TRANSACTION READ ONLY` 不夠——那設的是存取模式，不是 isolation。"""
    log, _conn = _run_snapshot(monkeypatch, "mysql")
    assert log[0] == ("execution_options", {"isolation_level": "REPEATABLE READ"})
    assert log[1] == ("sql", "START TRANSACTION READ ONLY")
    assert log[-1] == ("sql", "ROLLBACK")


def test_mysql_switches_back_from_read_committed(monkeypatch):
    """先把 session 設成 `READ COMMITTED`，Stage 0 必須**主動切回** `REPEATABLE READ`。"""
    log, _conn = _run_snapshot(monkeypatch, "mysql")
    isolation = [kwargs for kind, kwargs in log if kind == "execution_options"]
    assert isolation == [{"isolation_level": "REPEATABLE READ"}]
    # ⛔ 不得自己送 SET SESSION（那是 session 層級的污染，漏復原會跟著連線回到 pool）
    assert not any(kind == "sql" and "SET SESSION" in sql for kind, sql in log)


def test_sqlite_statement_order_and_query_only_restore(monkeypatch):
    class _Raw:
        isolation_level = ""

    class _DBAPI:
        dbapi_connection = _Raw()

    log, _conn = _run_snapshot(monkeypatch, "sqlite", dbapi=_DBAPI())
    sqls = [sql for kind, sql in log if kind == "sql"]
    assert sqls[0] == "PRAGMA query_only = 1"
    assert sqls[1] == "BEGIN DEFERRED"
    assert "ROLLBACK" in sqls
    # ⚠️ `query_only` 是連線層級的設定，連線會回到 pool——不復原就會害到後續的寫入路徑。
    assert sqls[-1] == "PRAGMA query_only = 0"
    assert _DBAPI.dbapi_connection.isolation_level == ""


def test_rollback_and_restore_happen_even_on_exception(monkeypatch):
    class _Raw:
        isolation_level = ""

    class _DBAPI:
        dbapi_connection = _Raw()

    log: list = []
    conn = _RecordingConn(log, dbapi=_DBAPI())
    monkeypatch.setattr(db, "DB_DRIVER", "sqlite")
    with pytest.raises(RuntimeError):
        with db.readonly_snapshot(engine_override=_FakeEngine(conn)):
            raise RuntimeError("injected mid-snapshot failure")
    sqls = [sql for kind, sql in log if kind == "sql"]
    assert "ROLLBACK" in sqls and sqls[-1] == "PRAGMA query_only = 0"
    assert conn.closed


# ── sqlite 的實際行為 ───────────────────────────────────────────────────────

def test_sqlite_snapshot_actually_rejects_writes(tmp_path, monkeypatch):
    engine = _sqlite_engine(tmp_path, _TS)
    monkeypatch.setattr(db, "DB_DRIVER", "sqlite")
    with db.readonly_snapshot(engine_override=engine) as conn:
        with pytest.raises(Exception):
            conn.exec_driver_sql("INSERT INTO candles (symbol) VALUES ('X')")


def test_sqlite_query_only_is_restored_for_later_writers(tmp_path, monkeypatch):
    """快照結束後，同一條連線回到 pool 仍必須寫得進去。"""
    engine = _sqlite_engine(tmp_path, _TS)
    monkeypatch.setattr(db, "DB_DRIVER", "sqlite")
    with db.readonly_snapshot(engine_override=engine):
        pass
    with engine.begin() as conn:
        conn.execute(text("INSERT INTO candles (symbol, timeframe, ts) VALUES ('9999','1d','2026-09-03 05:30:00')"))
    with engine.connect() as conn:
        assert conn.execute(text("SELECT COUNT(*) FROM candles")).scalar() == 4


def test_sqlite_snapshot_is_not_polluted_by_a_concurrent_writer(tmp_path, monkeypatch):
    """concurrent writer：擷取進行中由另一條連線 commit → bundle 必須全部是 commit 前。

    ⛔ 不得混合：一份 candles 停在 commit 前、另一份已是 commit 後的組合，
    **在資料庫裡從未同時存在過**。
    """
    engine = _sqlite_engine(tmp_path, _TS)
    monkeypatch.setattr(db, "engine", engine)
    monkeypatch.setattr(db, "DB_DRIVER", "sqlite")

    writer = sqlite3.connect(str(tmp_path / "candles.sqlite"))
    try:
        with db.readonly_snapshot(engine_override=engine) as conn:
            first = db.fetch_candles("2330", "1d", limit=100, conn=conn)
            writer.execute(
                "INSERT INTO candles (symbol, timeframe, open, high, low, close, volume, amount,"
                " adj_factor, vol_factor, ts) VALUES ('2330','1d',1,2,0.5,1.5,10,100,1,1,"
                "'2026-09-03 05:30:00')"
            )
            writer.commit()
            second = db.fetch_candles("2330", "1d", limit=100, conn=conn)
        assert len(first) == len(second) == 3
        # 快照之外看得到新資料——證明上面那次 commit 真的成功了（否則這條測試沒有意義）。
        assert len(db.fetch_candles("2330", "1d", limit=100)) == 4
    finally:
        writer.close()


def test_helpers_reuse_the_given_connection(tmp_path, monkeypatch):
    """四個擷取路徑都必須走同一條連線——⛔ 不得各自 `engine.connect()`。"""
    engine = _sqlite_engine(tmp_path, _TS)
    monkeypatch.setattr(db, "engine", engine)
    monkeypatch.setattr(db, "DB_DRIVER", "sqlite")

    used: list = []

    class _Spy:
        def __init__(self, inner):
            self.inner = inner

        def execute(self, *args, **kwargs):
            used.append(1)
            return self.inner.execute(*args, **kwargs)

    with db.readonly_snapshot(engine_override=engine) as conn:
        spy = _Spy(conn)
        db.fetch_market_latest_trading_date("1d", conn=spy)
        db.fetch_candles("2330", "1d", limit=10, conn=spy)
    assert len(used) == 2


# ── as-of 邊界：三個 engine 的實際 SQL 與 bind 參數 ────────────────────────
#
# ⚠️ **只有 sqlite 能在容器內真的跑**（測試容器不連 PG／MySQL，見 development-workflow.md
# 的手動章節）。所以另外兩個 engine 這裡驗的是「送出去的 SQL 與上界參數對不對」——
# 這正是 as-of 唯一會出錯的地方：取數邊界必須與回傳的 timestamp **同一個表示式**，
# 不然邊界與資料會不同源。

class _CapturingConn:
    def __init__(self):
        self.sql = None
        self.params = None

    def execute(self, sql, params=None):
        self.sql = " ".join(str(sql).split())
        self.params = params

        class _Result:
            def mappings(self_inner):
                return self_inner

            def all(self_inner):
                return []

            def first(self_inner):
                return None

        return _Result()


@pytest.mark.parametrize("driver,expr", [
    ("sqlite", "CAST(strftime('%s', ts) AS INTEGER)"),
    ("postgres", "EXTRACT(EPOCH FROM ts)::BIGINT"),
    ("mysql", "UNIX_TIMESTAMP(ts)"),
])
def test_as_of_clause_uses_the_same_expression_as_the_select(monkeypatch, driver, expr):
    monkeypatch.setattr(db, "DB_DRIVER", driver)
    conn = _CapturingConn()
    db.fetch_candles("2330", "1d", limit=10, as_of="2026-09-01", conn=conn)
    assert f"{expr} AS timestamp" in conn.sql
    assert f"AND {expr} < :as_of_cutoff" in conn.sql
    assert conn.params["as_of_cutoff"] == db.as_of_cutoff_epoch("2026-09-01")


@pytest.mark.parametrize("driver", ["sqlite", "postgres", "mysql"])
@pytest.mark.parametrize("as_of", ["2026-08-31", "2026-09-01", "2026-09-02"])
def test_as_of_boundary_cutoff_is_next_taipei_midnight_on_every_engine(monkeypatch, driver, as_of):
    """三邊界 × 三 engine：上界一律是**次日 00:00（台北）**，含當日、⛔ 不含次日。"""
    monkeypatch.setattr(db, "DB_DRIVER", driver)
    conn = _CapturingConn()
    db.fetch_candles("2330", "1d", limit=10, as_of=as_of, conn=conn)
    cutoff = datetime.fromtimestamp(conn.params["as_of_cutoff"], tz=TAIPEI)
    expected = datetime.fromisoformat(as_of).replace(tzinfo=TAIPEI) + timedelta(days=1)
    assert cutoff == expected


@pytest.mark.parametrize("driver", ["sqlite", "postgres", "mysql"])
def test_no_as_of_means_no_clause_and_no_param(monkeypatch, driver):
    """⛔ 不改既有預設行為：不傳 `as_of` 時 SQL 與參數都不該多出東西。"""
    monkeypatch.setattr(db, "DB_DRIVER", driver)
    conn = _CapturingConn()
    db.fetch_candles("2330", "1d", limit=10, conn=conn)
    assert "as_of_cutoff" not in conn.sql
    assert "as_of_cutoff" not in conn.params


# ── 收尾失敗不得被吞掉 ──────────────────────────────────────────────────────

def test_failed_query_only_restore_invalidates_and_raises(monkeypatch):
    """⛔ 復原失敗卻照常歸還連線的話，後續拿到它的**正常寫入路徑會突然變成唯讀**。"""
    log: list = []
    invalidated: list[bool] = []

    class _Raw:
        isolation_level = ""

    class _DBAPI:
        dbapi_connection = _Raw()

    class _BadRestoreConn(_RecordingConn):
        def exec_driver_sql(self, sql):
            if "query_only = 0" in str(sql):
                raise RuntimeError("injected restore failure")
            return super().exec_driver_sql(sql)

        def invalidate(self):
            invalidated.append(True)

    conn = _BadRestoreConn(log, dbapi=_DBAPI())
    monkeypatch.setattr(db, "DB_DRIVER", "sqlite")
    with pytest.raises(RuntimeError) as exc:
        with db.readonly_snapshot(engine_override=_FakeEngine(conn)):
            pass
    assert "連線已 invalidate" in str(exc.value)
    assert invalidated == [True]
    assert conn.closed


def test_restore_failure_does_not_mask_the_original_exception(monkeypatch, caplog):
    """⚠️ 已經有例外在往上傳時，⛔ 不要用收尾失敗蓋掉原始成因。"""
    log: list = []
    invalidated: list[bool] = []

    class _Raw:
        isolation_level = ""

    class _DBAPI:
        dbapi_connection = _Raw()

    class _BadRestoreConn(_RecordingConn):
        def exec_driver_sql(self, sql):
            if "query_only = 0" in str(sql):
                raise RuntimeError("injected restore failure")
            return super().exec_driver_sql(sql)

        def invalidate(self):
            invalidated.append(True)

    conn = _BadRestoreConn(log, dbapi=_DBAPI())
    monkeypatch.setattr(db, "DB_DRIVER", "sqlite")
    with pytest.raises(ValueError, match="original"):
        with db.readonly_snapshot(engine_override=_FakeEngine(conn)):
            raise ValueError("original failure")
    # 連線仍然要被丟棄——只是不改寫往上傳的例外。
    assert invalidated == [True]
