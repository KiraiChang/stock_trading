"""SQLAlchemy engine + session factory（支援 SQLite / MySQL）。"""
from __future__ import annotations
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
