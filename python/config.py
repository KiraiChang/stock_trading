"""載入 python/config.yaml，提供全域設定物件。"""
import os
import yaml
from pathlib import Path

_ROOT = Path(__file__).parent
_cfg_path = os.getenv("TRADING_CONFIG", str(_ROOT / "config.yaml"))

with open(_cfg_path, encoding="utf-8") as f:
    _raw = yaml.safe_load(f)

# ── Database ──────────────────────────────────────────────────
# 環境變數優先，方便 Docker 覆寫而不需掛載 config 檔
DB_DRIVER: str = os.getenv("DATABASE_DRIVER") or _raw["database"]["driver"]
DB_DSN_RAW: str = os.getenv("DATABASE_DSN") or _raw["database"]["dsn"]

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
SERVICE_PORT: int = int(os.getenv("PYTHON_SERVICE_PORT") or _raw["python_service"]["port"])
WORKER_POLL_INTERVAL: int = _raw["python_service"]["worker_poll_interval"]

# ── Backtest ──────────────────────────────────────────────────
COMMISSION_RATE: float = _raw["backtest"]["commission_rate"]
TAX_RATE: float = _raw["backtest"]["tax_rate"]
INITIAL_CASH: float = _raw["backtest"]["initial_cash"]

# ── SR Scoring ────────────────────────────────────────────────
SR_SCORING_MODEL_PATH: str = os.getenv("SR_SCORING_MODEL_PATH") or str(
    (_ROOT / _raw["sr_scoring"]["model_path"]).resolve()
)


def _env_bool(name: str, default: bool) -> bool:
    raw = os.getenv(name)
    if raw is None or raw == "":
        return default
    return raw.strip().lower() in ("1", "true", "yes", "on")


# evidence（SHAP 貢獻）可降級：關閉或缺 shap/background 時 /sr-zones 不再 503，
# 詳見 evidence.build_evidence 與 docs/sr-zone-scoring.md。
SR_SCORING_EVIDENCE_ENABLED: bool = _env_bool(
    "SR_SCORING_EVIDENCE_ENABLED", bool(_raw["sr_scoring"].get("evidence_enabled", True))
)
SR_SCORING_EVIDENCE_MAX_ZONES: int = int(
    os.getenv("SR_SCORING_EVIDENCE_MAX_ZONES") or _raw["sr_scoring"].get("evidence_max_zones", 8)
)
