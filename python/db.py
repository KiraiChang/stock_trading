"""SQLAlchemy engine + session factory（支援 SQLite / MySQL）。"""
from __future__ import annotations
import json
import logging

from sqlalchemy import create_engine, text
from sqlalchemy.orm import sessionmaker, Session
from config import get_db_url, DB_DRIVER

log = logging.getLogger(__name__)

_connect_args = {"check_same_thread": False} if DB_DRIVER == "sqlite" else {}


_db_url = get_db_url()
log.info("DB engine: driver=%s  url=%s", DB_DRIVER,
         _db_url if DB_DRIVER == "sqlite" else _db_url.split("@")[-1])  # 隱藏密碼

engine = create_engine(
    _db_url,
    connect_args=_connect_args,
    pool_pre_ping=True,
)

# 啟動時確認連線可用
try:
    with engine.connect() as _conn:
        _conn.execute(text("SELECT 1"))
    log.info("DB connection OK")
except Exception as _e:
    log.error("DB connection FAILED: %s", _e)
    raise

SessionLocal = sessionmaker(bind=engine, autocommit=False, autoflush=False)


def get_session() -> Session:
    return SessionLocal()


def fetch_candles(symbol: str, timeframe: str, limit: int = 200) -> list[dict]:
    """從 DB 讀取 K 棒，回傳欄位與 Go Candle struct 對齊。"""
    if DB_DRIVER == "sqlite":
        sql = text("""
            SELECT symbol, timeframe, open, high, low, close, volume, amount,
                   CAST(strftime('%s', ts) AS INTEGER) AS timestamp
            FROM candles
            WHERE symbol = :symbol AND timeframe = :tf
            ORDER BY ts DESC
            LIMIT :limit
        """)
    elif DB_DRIVER in ("postgres", "postgresql"):
        sql = text("""
            SELECT symbol, timeframe, open, high, low, close, volume, amount,
                   EXTRACT(EPOCH FROM ts)::BIGINT AS timestamp
            FROM candles
            WHERE symbol = :symbol AND timeframe = :tf
            ORDER BY ts DESC
            LIMIT :limit
        """)
    else:
        sql = text("""
            SELECT symbol, timeframe, open, high, low, close, volume, amount,
                   UNIX_TIMESTAMP(ts) AS timestamp
            FROM candles
            WHERE symbol = :symbol AND timeframe = :tf
            ORDER BY ts DESC
            LIMIT :limit
        """)

    with engine.connect() as conn:
        rows = conn.execute(sql, {"symbol": symbol, "tf": timeframe, "limit": limit}).mappings().all()

    result = list(reversed([dict(r) for r in rows]))
    log.debug("fetch_candles symbol=%s tf=%s → %d rows", symbol, timeframe, len(result))
    return result


def fetch_latest_chip_score(symbol: str, before_date: str | None = None) -> dict | None:
    """查詢最新一筆籌碼分析分數（見 backend internal/chip 套件），供
    sr_scoring 當作 trading_score 的第六個加權分量。before_date 指定時只取
    該日（含）以前最新一筆；即時評分（score_symbol）不帶這個參數，直接取
    全庫最新一筆。查無資料回傳 None，呼叫端必須 fallback 為中性值，不可
    讓籌碼資料缺漏中斷整個 SR Zone 評分流程。"""
    params: dict[str, object] = {"symbol": symbol}
    where_date = ""
    if before_date is not None:
        where_date = "AND trade_date <= :before_date"
        params["before_date"] = before_date

    sql = text(f"""
        SELECT symbol, trade_date, institutional_score, margin_score, broker_score,
               concentration_score, total_score, signal, reason
        FROM chip_scores
        WHERE symbol = :symbol {where_date}
        ORDER BY trade_date DESC
        LIMIT 1
    """)

    with engine.connect() as conn:
        row = conn.execute(sql, params).mappings().first()

    if row is None:
        return None

    result = dict(row)
    if isinstance(result.get("reason"), str):
        try:
            result["reason"] = json.loads(result["reason"])
        except (TypeError, ValueError):
            result["reason"] = []
    return result


def fetch_chip_scores(symbol: str, from_date: str, to_date: str) -> list[dict]:
    """查詢一段區間的籌碼分數，供 sr_scoring（見 scoring.py 的
    fetch_latest_chip_score）以外的用途使用——目前給 modular 回測的籌碼
    filter（見 backtest/modular/service.py）逐 bar 比對 total_score。"""
    sql = text("""
        SELECT symbol, trade_date, institutional_score, margin_score, broker_score,
               concentration_score, total_score, signal
        FROM chip_scores
        WHERE symbol = :symbol AND trade_date BETWEEN :f AND :t
        ORDER BY trade_date ASC
    """)
    with engine.connect() as conn:
        rows = conn.execute(sql, {"symbol": symbol, "f": from_date, "t": to_date}).mappings().all()
    return [dict(r) for r in rows]
