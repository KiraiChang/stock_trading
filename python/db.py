"""SQLAlchemy engine + session factory（支援 SQLite / MySQL）。"""
from sqlalchemy import create_engine, text
from sqlalchemy.orm import sessionmaker, Session
from config import get_db_url, DB_DRIVER

_connect_args = {"check_same_thread": False} if DB_DRIVER == "sqlite" else {}

engine = create_engine(
    get_db_url(),
    connect_args=_connect_args,
    pool_pre_ping=True,
)

SessionLocal = sessionmaker(bind=engine, autocommit=False, autoflush=False)


def get_session() -> Session:
    return SessionLocal()


def fetch_candles(symbol: str, timeframe: str, limit: int = 200) -> list[dict]:
    """從 DB 讀取 K 棒，回傳欄位與 Go Candle struct 對齊。"""
    sql = text("""
        SELECT symbol, timeframe, open, high, low, close, volume, amount,
               CAST(strftime('%s', ts) AS INTEGER) AS timestamp
        FROM candles
        WHERE symbol = :symbol AND timeframe = :tf
        ORDER BY ts DESC
        LIMIT :limit
    """) if DB_DRIVER == "sqlite" else text("""
        SELECT symbol, timeframe, open, high, low, close, volume, amount,
               UNIX_TIMESTAMP(ts) AS timestamp
        FROM candles
        WHERE symbol = :symbol AND timeframe = :tf
        ORDER BY ts DESC
        LIMIT :limit
    """)

    with engine.connect() as conn:
        rows = conn.execute(sql, {"symbol": symbol, "tf": timeframe, "limit": limit}).mappings().all()

    # 反轉為時間升冪（與 Go GetLatestN 一致）
    return list(reversed([dict(r) for r in rows]))
