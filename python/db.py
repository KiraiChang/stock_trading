"""SQLAlchemy engine + session factory（支援 SQLite / MySQL）。"""
from __future__ import annotations
import json
import logging
from datetime import date, datetime

from sqlalchemy import bindparam, create_engine, text
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

SessionLocal = sessionmaker(bind=engine, autocommit=False, autoflush=False)


def check_connection() -> None:
    """確認 DB 連線可用（SELECT 1），失敗即 raise。由服務啟動路徑（http_server /
    worker / CLI）明確呼叫，不在 module import 時執行——import db 不應該有連線副作用，
    否則純單元測試或離線工具會被連不到 DB 綁架（見 development-workflow.md「開發慣例」）。"""
    try:
        with engine.connect() as conn:
            conn.execute(text("SELECT 1"))
        log.info("DB connection OK")
    except Exception as e:
        log.error("DB connection FAILED: %s", e)
        raise


def get_session() -> Session:
    return SessionLocal()


def fetch_candles(
    symbol: str, timeframe: str, limit: int = 200, adjusted: bool = True
) -> list[dict]:
    """從 DB 讀取 K 棒，回傳欄位與 Go Candle struct 對齊。

    **預設回傳還原價**（`adjusted=True`）。DB 存的是原始成交價，跨越分割的序列會出現
    假跳空——0050 在 2025-06-18 的 1:4 分割讓價格從 188.65 掉到 47.57，任何跨過那天的
    MA / ATR / zone 建構都會看到一個從未發生的 −75%（見 docs/database-schema.md 的「股價還原」）。

    這裡是 **Python 端唯一的 candles 進入點**，所以還原一次做在這裡；若讓每個消費者
    自己乘係數，漏掉的那個不會有任何東西報錯。要原始成交價（例如顯示「當時實際成交在
    哪裡」）時明確傳 `adjusted=False`。

    價乘係數、量除係數——方向相反，因為分割讓股數變多：歷史價要縮小、歷史量要放大。
    `amount`（成交金額）不動，錢不隨股數重新定義。

    **價與量用不同的係數**：`adj_factor` 給價、`vol_factor` 給量。現金股利讓價格下修
    但**股數沒有改變**，所以成交量不可以跟著調整；只有分割與配股會改變股數。
    因此 `adj_close * adj_volume == close * volume` **只在兩個係數相等時成立**。
    """
    if DB_DRIVER == "sqlite":
        sql = text("""
            SELECT symbol, timeframe, open, high, low, close, volume, amount, adj_factor, vol_factor,
                   CAST(strftime('%s', ts) AS INTEGER) AS timestamp
            FROM candles
            WHERE symbol = :symbol AND timeframe = :tf
            ORDER BY ts DESC
            LIMIT :limit
        """)
    elif DB_DRIVER in ("postgres", "postgresql"):
        sql = text("""
            SELECT symbol, timeframe, open, high, low, close, volume, amount, adj_factor, vol_factor,
                   EXTRACT(EPOCH FROM ts)::BIGINT AS timestamp
            FROM candles
            WHERE symbol = :symbol AND timeframe = :tf
            ORDER BY ts DESC
            LIMIT :limit
        """)
    else:
        sql = text("""
            SELECT symbol, timeframe, open, high, low, close, volume, amount, adj_factor, vol_factor,
                   UNIX_TIMESTAMP(ts) AS timestamp
            FROM candles
            WHERE symbol = :symbol AND timeframe = :tf
            ORDER BY ts DESC
            LIMIT :limit
        """)

    with engine.connect() as conn:
        rows = conn.execute(sql, {"symbol": symbol, "tf": timeframe, "limit": limit}).mappings().all()

    result = list(reversed([dict(r) for r in rows]))
    if adjusted:
        for row in result:
            # 係數為 None/0（欄位還沒被重算過，或舊資料）時當作 1。
            # 「沒有係數」的正解是「不調整」，不是「價格歸零」。
            factor = float(row.get("adj_factor") or 1.0)
            if factor <= 0:
                factor = 1.0
            # vol_factor 缺值（Phase 1 的舊資料）時退回 adj_factor：
            # 那時只有分割，價量本來就共用一個係數。
            vol_factor = float(row.get("vol_factor") or 0.0)
            if vol_factor <= 0:
                vol_factor = factor
            for field in ("open", "high", "low", "close"):
                if row.get(field) is not None:
                    row[field] = float(row[field]) * factor
            if row.get("volume") is not None:
                row["volume"] = float(row["volume"]) / vol_factor
    log.debug("fetch_candles symbol=%s tf=%s adjusted=%s → %d rows",
              symbol, timeframe, adjusted, len(result))
    return result


def fetch_market_trading_days(timeframe: str = "1d", limit: int = 60) -> list[str]:
    """最近 N 個**市場交易日**（全庫 distinct 日期），由新到舊。

    **為什麼需要它**：candles 只有「有成交的日子」才會有列——沒成交的日子根本沒有那一列。
    所以「某檔近 60 根裡有幾天有成交」恆等於 60，量不出流動性。
    真正的分母是市場交易日：拿它去比對某檔在同一區間內有幾根 K 線，才看得出
    「這檔有多少天根本沒人交易」（見 docs/evaluation-universe-selection-plan.md 的已知缺陷）。
    """
    if DB_DRIVER == "sqlite":
        sql = text("""
            SELECT DISTINCT date(ts) AS d FROM candles WHERE timeframe = :tf
            ORDER BY d DESC LIMIT :limit
        """)
    elif DB_DRIVER in ("postgres", "postgresql"):
        # ts 存 UTC，必須轉台北時區再取日期，否則整整差一天（見 docs/database-schema.md）。
        sql = text("""
            SELECT DISTINCT (ts AT TIME ZONE 'Asia/Taipei')::date AS d
            FROM candles WHERE timeframe = :tf
            ORDER BY d DESC LIMIT :limit
        """)
    else:
        sql = text("""
            SELECT DISTINCT DATE(ts) AS d FROM candles WHERE timeframe = :tf
            ORDER BY d DESC LIMIT :limit
        """)
    with engine.connect() as conn:
        rows = conn.execute(sql, {"tf": timeframe, "limit": limit}).mappings().all()
    return [str(r["d"]) for r in rows]


def fetch_watchlist_symbols() -> list[str]:
    """watchlist 的代號（唯讀），由小到大。

    **成員資格就是「`watchlists` 裡有這一列」**，不要用 `watched` 過濾——那個布林是
    「要不要即時監聽」的開關（有併發上限，見 `watchlist_repo.SetWatched`），
    實測 11 檔裡只有 2 檔為 true。拿它當成員條件會漏掉 9 檔。
    """
    with engine.connect() as conn:
        rows = conn.execute(text("SELECT symbol FROM watchlists ORDER BY symbol")).mappings().all()
    return [str(r["symbol"]) for r in rows]


def fetch_candle_depths(timeframe: str = "1d") -> dict[str, int]:
    """每檔的**總 K 棒數**（全歷史），symbol → count。

    **不要拿 `fetch_candles()` 回傳的長度當深度**：那個函式帶 `limit`，
    selection report 只抓近 60 根算波動 profile，所以它的 `len()` 上限就是 60，
    量的是「抓了幾根」不是「有幾根」。要判斷標的撐不撐得起 walk-forward 的
    `--limit 1500`，只能用這裡的全表聚合。

    單一 GROUP BY，實測 857 檔約 0.5 秒。
    """
    sql = text("SELECT symbol, COUNT(*) AS n FROM candles WHERE timeframe = :tf GROUP BY symbol")
    with engine.connect() as conn:
        rows = conn.execute(sql, {"tf": timeframe}).mappings().all()
    return {str(r["symbol"]): int(r["n"]) for r in rows}


def fetch_symbol_universe(symbols: list[str] | None = None) -> list[dict]:
    """stock_symbols 主檔（唯讀）。symbols 為 None 時回傳全部仍上市者。"""
    base = """
        SELECT symbol, name, security_type, industry, listed_date, is_listed
        FROM stock_symbols
    """
    params: dict = {}
    if symbols:
        # SQLAlchemy 的 expanding bindparam 讓 IN 清單在三種 engine 上都能用。
        sql = text(base + " WHERE symbol IN :symbols").bindparams(
            bindparam("symbols", expanding=True)
        )
        params["symbols"] = list(symbols)
    else:
        sql = text(base + " WHERE is_listed = :listed")
        params["listed"] = True
    with engine.connect() as conn:
        rows = conn.execute(sql, params).mappings().all()
    return [
        {
            "symbol": str(r["symbol"]),
            "name": str(r["name"] or ""),
            "security_type": str(r["security_type"] or ""),
            "industry": str(r["industry"] or ""),
            "listed_date": _iso_value(r["listed_date"]),
            "is_listed": bool(r["is_listed"]),
        }
        for r in rows
    ]


def fetch_latest_chip_score(symbol: str, before_date: str | None = None) -> dict | None:
    """查詢最新一筆籌碼分析分數（見 backend internal/chip 套件），供
    sr_scoring 當作 trading_score 的第六個加權分量。before_date 指定時只取
    該日（含）以前最新一筆；score_symbol() 會依 analyzed_at 換算並傳入
    before_date，避免看到分析當下之後才產生的籌碼資料。省略 before_date
    只適合測試、診斷，或明確需要全庫最新資料的用途。查無資料回傳 None，
    呼叫端必須 fallback 為中性值，不可讓籌碼資料缺漏中斷整個 SR Zone 評分流程。"""
    params: dict[str, object] = {"symbol": symbol}
    where_date = ""
    if before_date is not None:
        where_date = "AND trade_date <= :before_date"
        params["before_date"] = before_date

    sql = text(f"""
        SELECT symbol, trade_date, institutional_score, margin_score, broker_score,
               concentration_score, total_score, signal_type, reason
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
               concentration_score, total_score, signal_type
        FROM chip_scores
        WHERE symbol = :symbol AND trade_date BETWEEN :f AND :t
        ORDER BY trade_date ASC
    """)
    with engine.connect() as conn:
        rows = conn.execute(sql, {"symbol": symbol, "f": from_date, "t": to_date}).mappings().all()
    return [dict(r) for r in rows]


def fetch_sr_model_governance(symbol: str, timeframe: str, from_ts: str, to_ts: str) -> list[dict]:
    """查詢 SR model governance 歷史快照，供 decision replay 依 as-of 時間重建
    當時模型健康度 gate。回傳欄位對齊 Go API 注入到 Python 的
    model_governance_by_symbol row 形狀。"""
    sql = text("""
        SELECT analyzed_at, created_at, symbol, timeframe, model_version, model_config_hash,
               health_state, average_edge_pp, directional_zone_count, zone_count,
               allow_entry, max_entry_state, quality_flags, warning_flags, blocking_flags,
               confidence_gate_json, calibration_report_json, walk_forward_report_json,
               dataset_diagnostics_json, governance_json
        FROM stock_sr_model_governance
        WHERE symbol = :symbol AND timeframe = :tf AND analyzed_at BETWEEN :f AND :t
        ORDER BY analyzed_at ASC, id ASC
    """)
    with engine.connect() as conn:
        rows = conn.execute(sql, {"symbol": symbol, "tf": timeframe, "f": from_ts, "t": to_ts}).mappings().all()

    result: list[dict] = []
    for row in rows:
        item = dict(row)
        result.append({
            "as_of": _iso_value(item.get("analyzed_at")),
            "created_at": _iso_value(item.get("created_at")),
            "symbol": item.get("symbol"),
            "timeframe": item.get("timeframe"),
            "model_version": item.get("model_version"),
            "model_config_hash": item.get("model_config_hash"),
            "health_state": item.get("health_state"),
            "average_edge_pp": item.get("average_edge_pp"),
            "directional_zone_count": item.get("directional_zone_count"),
            "zone_count": item.get("zone_count"),
            "allow_entry": item.get("allow_entry"),
            "max_entry_state": item.get("max_entry_state"),
            "quality_flags": _json_value(item.get("quality_flags"), []),
            "warning_flags": _json_value(item.get("warning_flags"), []),
            "blocking_flags": _json_value(item.get("blocking_flags"), []),
            "confidence_gate": _json_value(item.get("confidence_gate_json"), {}),
            "calibration_report": _json_value(item.get("calibration_report_json"), {}),
            "walk_forward_report": _json_value(item.get("walk_forward_report_json"), {}),
            "dataset_diagnostics": _json_value(item.get("dataset_diagnostics_json"), {}),
            "governance": _json_value(item.get("governance_json"), {}),
        })
    return result


def fetch_latest_sr_regression_governance(
    model_config_hash: str,
    schema_version: str = "sr_zone_decision_replay_p0",
) -> dict | None:
    """查詢同一 model_config_hash 最新 decision replay governance gate。

    這是 production analysis 的外層模型治理 gate：若最近 replay 判定目前模型
    UNRELIABLE / DEGRADED，正式決策會透過 confidence_gate 保守化。查無資料
    回傳 None，呼叫端應維持原本模型治理邏輯。
    """
    if not model_config_hash:
        return None
    sql = text("""
        SELECT run_id, model_config_hash, pipeline_version, schema_version, passed,
               governance_health_state, governance_strict_passed, metrics_json, created_at
        FROM stock_sr_regression_results
        WHERE schema_version = :schema_version
          AND model_config_hash = :model_config_hash
          AND governance_health_state <> ''
        ORDER BY created_at DESC, id DESC
        LIMIT 1
    """)
    with engine.connect() as conn:
        row = conn.execute(sql, {
            "schema_version": schema_version,
            "model_config_hash": model_config_hash,
        }).mappings().first()
    if row is None:
        return None

    item = dict(row)
    metrics = _json_value(item.get("metrics_json"), {})
    governance = metrics.get("governance_evaluation") if isinstance(metrics, dict) else None
    confidence_gate = {}
    if isinstance(governance, dict):
        confidence_gate = governance.get("confidence_gate") or {}
    if not isinstance(confidence_gate, dict):
        confidence_gate = {}
    health_state = item.get("governance_health_state") or (
        governance.get("health_state") if isinstance(governance, dict) else None
    )
    strict_passed = item.get("governance_strict_passed")
    return {
        "source": "LATEST_REGRESSION_RESULT",
        "schema_version": schema_version,
        "run_id": item.get("run_id"),
        "model_config_hash": item.get("model_config_hash"),
        "pipeline_version": item.get("pipeline_version"),
        "created_at": _iso_value(item.get("created_at")),
        "health_state": health_state,
        "passed": item.get("passed"),
        "strict_passed": strict_passed,
        "confidence_gate": confidence_gate,
        "governance_evaluation": governance if isinstance(governance, dict) else None,
    }


def _iso_value(value: object) -> object:
    if isinstance(value, (datetime, date)):
        return value.isoformat()
    return value


def _json_value(value: object, fallback: object) -> object:
    if value is None:
        return fallback
    if isinstance(value, (dict, list)):
        return value
    if isinstance(value, str):
        try:
            return json.loads(value)
        except (TypeError, ValueError):
            return fallback
    return fallback
