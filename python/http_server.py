"""
Method B：FastAPI HTTP service。
Go 可 POST /backtest 主動觸發，不需等待 worker 輪詢。

啟動方式：
    cd python
    uvicorn http_server:app --port 8001 --reload

API：
  POST /backtest          執行回測（同步，可能耗時數秒）
  GET  /backtest/{job_id} 查詢結果（從 DB 讀）
  GET  /health            健康檢查
"""
from __future__ import annotations
import logging
import sys
import os

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [server] %(levelname)-8s %(name)s — %(message)s",
    datefmt="%Y-%m-%d %H:%M:%S",
    stream=sys.stdout,
    force=True,
)
logging.getLogger("sqlalchemy.engine").setLevel(logging.WARNING)

log = logging.getLogger(__name__)

import json

sys.path.insert(0, os.path.dirname(__file__))
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "backtest"))

log.info("loading config...")
from config import SERVICE_PORT

log.info("connecting to database...")
from db import engine

from fastapi import FastAPI, HTTPException, BackgroundTasks
from pydantic import BaseModel, field_validator
from typing import List, Union
from sqlalchemy import text
from backtest.engine import run_backtest
from backtest.db_writer import update_job_status, write_result, write_trades

app = FastAPI(title="Trading Backtest Service", version="1.0.0")


class BacktestRequest(BaseModel):
    job_id: str
    strategy: str
    symbols: Union[str, List[str]]   # 接受 JSON string 或 list
    timeframe: str = "1d"
    start_date: str
    end_date: str

    @field_validator("symbols")
    @classmethod
    def parse_symbols(cls, v):
        if isinstance(v, str):
            return json.loads(v)
        return v


def _run_and_write(req: BacktestRequest) -> None:
    job_id = req.job_id
    log.info("running job=%s  strategy=%s  symbols=%s  timeframe=%s",
             job_id, req.strategy, req.symbols, req.timeframe)
    try:
        update_job_status(job_id, "running")
        output = run_backtest(
            strategy=req.strategy,
            symbols=req.symbols,
            timeframe=req.timeframe,
            start_date=req.start_date,
            end_date=req.end_date,
        )
        write_result(job_id, output["result"])
        write_trades(job_id, output["trades"])
        update_job_status(job_id, "done")
        log.info("job=%s done — total_trades=%d  total_return=%.2f%%  sharpe=%.4f",
                 job_id,
                 output["result"]["total_trades"],
                 output["result"]["total_return"] * 100,
                 output["result"]["sharpe_ratio"])
    except Exception as exc:
        log.error("job=%s failed: %s", job_id, exc, exc_info=True)
        update_job_status(job_id, "failed", str(exc))


@app.post("/backtest", status_code=202)
async def submit_backtest(req: BacktestRequest, background_tasks: BackgroundTasks):
    """非同步執行回測（立即回傳 job_id，背景執行）。"""
    log.info("POST /backtest  job=%s  strategy=%s", req.job_id, req.strategy)
    background_tasks.add_task(_run_and_write, req)
    return {"job_id": req.job_id, "status": "running"}


@app.get("/backtest/{job_id}")
async def get_backtest(job_id: str):
    """查詢回測狀態與結果。"""
    log.info("GET /backtest/%s", job_id)
    job_sql = text("SELECT * FROM backtest_jobs WHERE job_id=:id")
    res_sql = text("SELECT * FROM backtest_results WHERE job_id=:id")

    with engine.connect() as conn:
        job_row = conn.execute(job_sql, {"id": job_id}).mappings().first()
        if not job_row:
            raise HTTPException(status_code=404, detail="Job not found")
        res_row = conn.execute(res_sql, {"id": job_id}).mappings().first()

    return {
        "job":    dict(job_row),
        "result": dict(res_row) if res_row else None,
    }


@app.get("/health")
async def health():
    return {"status": "ok"}
