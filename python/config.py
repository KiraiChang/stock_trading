"""載入 python/config.yaml，提供全域設定物件。"""
import os
import yaml
from pathlib import Path

_ROOT = Path(__file__).parent
_cfg_path = os.getenv("TRADING_CONFIG", str(_ROOT / "config.yaml"))

with open(_cfg_path, encoding="utf-8") as f:
    _raw = yaml.safe_load(f)

# ── Database ──────────────────────────────────────────────────
DB_DRIVER: str = _raw["database"]["driver"]
DB_DSN_RAW: str = _raw["database"]["dsn"]

def get_db_url() -> str:
    """回傳 SQLAlchemy 格式的連線字串。"""
    if DB_DRIVER == "sqlite":
        path = Path(DB_DSN_RAW)
        if not path.is_absolute():
            path = (_ROOT / path).resolve()
        return f"sqlite:///{path}"
    if DB_DRIVER == "mysql":
        return DB_DSN_RAW  # 已是 mysql+pymysql:// 格式
    if DB_DRIVER in ("postgres", "postgresql"):
        return DB_DSN_RAW  # 已是 postgresql+psycopg2:// 格式
    raise ValueError(f"Unsupported driver: {DB_DRIVER}")

# ── Python service ────────────────────────────────────────────
SERVICE_PORT: int = _raw["python_service"]["port"]
WORKER_POLL_INTERVAL: int = _raw["python_service"]["worker_poll_interval"]

# ── Backtest ──────────────────────────────────────────────────
COMMISSION_RATE: float = _raw["backtest"]["commission_rate"]
TAX_RATE: float = _raw["backtest"]["tax_rate"]
INITIAL_CASH: float = _raw["backtest"]["initial_cash"]
