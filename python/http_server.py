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
from typing import List, Optional, Union
from sqlalchemy import text
from backtest.engine import run_backtest
from backtest.db_writer import update_job_status, write_result, write_trades
from backtest.modular.analysis import DEFAULT_FETCH_LIMIT, analyze_symbol
from backtest.modular.sr_scoring.scoring import DEFAULT_FETCH_LIMIT as SR_SCORING_DEFAULT_FETCH_LIMIT
from backtest.modular.sr_scoring.scoring import score_symbol
from backtest.modular.sr_scoring.train import run_training

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


class AnalyzeRequest(BaseModel):
    symbol: str
    timeframe: str = "1d"
    limit: int = DEFAULT_FETCH_LIMIT  # 抓取的歷史K棒根數，可由呼叫端覆寫


@app.post("/analyze")
async def analyze(req: AnalyzeRequest):
    """個股現況分析：支撐/壓力/進場/停損/停利（同步計算，不寫 DB，由 Go 端負責持久化與驗證）。"""
    log.info("POST /analyze symbol=%s tf=%s limit=%d", req.symbol, req.timeframe, req.limit)
    try:
        return analyze_symbol(req.symbol, req.timeframe, req.limit)
    except ValueError as exc:
        raise HTTPException(status_code=404, detail=str(exc))


class ScoreZonesRequest(BaseModel):
    symbol: str
    timeframe: str = "1d"
    limit: int = SR_SCORING_DEFAULT_FETCH_LIMIT  # 抓取的歷史K棒根數，可由呼叫端覆寫


@app.post("/sr-zones")
async def sr_zones(req: ScoreZonesRequest):
    """支撐/壓力機率評分：對每個 zone 回傳 support_score/resistance_score
    （規則式，永遠可算）與 bounce_probability/break_probability（需要先跑過
    sr_scoring/train.py 產生模型，否則回 503）。回應頂層另外帶
    model_version/model_trained_at/model_feature_names，來自產生這次結果的
    ModelBundle，供追蹤「這筆分析是哪個模型版本算出來的」。"""
    log.info("POST /sr-zones symbol=%s tf=%s limit=%d", req.symbol, req.timeframe, req.limit)
    try:
        return score_symbol(req.symbol, req.timeframe, req.limit)
    except ValueError as exc:
        raise HTTPException(status_code=404, detail=str(exc))
    except RuntimeError as exc:
        raise HTTPException(status_code=503, detail=str(exc))


class TrainRequest(BaseModel):
    symbols: Optional[List[str]] = None
    timeframe: str = "1d"
    limit: int = 1500
    model_type: str = "gradient_boosting"


@app.post("/sr-scoring/train")
async def sr_scoring_train(req: TrainRequest):
    """手動觸發 sr_scoring 機率模型訓練（同步執行，視資料量可能耗時數十秒到
    數分鐘；Go 端會用背景 goroutine 呼叫，不會卡住 HTTP 回應）。"""
    log.info(
        "POST /sr-scoring/train symbols=%s tf=%s limit=%d model_type=%s",
        req.symbols, req.timeframe, req.limit, req.model_type,
    )
    try:
        return run_training(
            symbols=req.symbols,
            timeframe=req.timeframe,
            limit=req.limit,
            model_type=req.model_type,
        )
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc))


@app.get("/health")
async def health():
    return {"status": "ok"}
