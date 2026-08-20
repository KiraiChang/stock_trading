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

sys.path.insert(0, os.path.dirname(__file__))
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "backtest"))

from logging_setup import configure_logging

configure_logging("python-server")
logging.getLogger("sqlalchemy.engine").setLevel(logging.WARNING)

log = logging.getLogger(__name__)

import json

log.info("loading config...")
from config import SERVICE_PORT

log.info("loading database module...")
from db import engine, check_connection

from contextlib import asynccontextmanager
from datetime import date
from fastapi import FastAPI, HTTPException, BackgroundTasks
from pydantic import BaseModel, Field, field_validator
from typing import Any, List, Optional, Union
from sqlalchemy import text
from backtest.engine import run_backtest
from backtest.db_writer import update_job_status, write_result, write_trades
from backtest.modular.analysis import DEFAULT_FETCH_LIMIT, analyze_symbol
from backtest.modular.sr_scoring.scoring import DEFAULT_FETCH_LIMIT as SR_SCORING_DEFAULT_FETCH_LIMIT
from backtest.modular.sr_scoring.scoring import score_symbol
from backtest.modular.sr_scoring.dataset import DatasetConfig
from backtest.modular.sr_scoring.evaluation import (
    DEFAULT_EVALUATION_LIMIT,
    DEFAULT_PIPELINE_VERSION,
    DEFAULT_REPLAY_MAX_ROWS,
    run_decision_replay,
    run_evaluation,
    write_evaluation_result,
)
from backtest.modular.sr_scoring.model import get_model
from backtest.modular.sr_scoring.train import run_training
from backtest.modular.sr_scoring.zone_builder import ATRZoneBuilderConfig, ZoneBuilderConfig
from backtest.modular.sr_scoring import zone_matcher


@asynccontextmanager
async def lifespan(app: FastAPI):
    """DB 連線檢查放在啟動事件，不放在 module import 期。

    兩條啟動路徑都會經過 lifespan，fail-fast 行為與先前一致（startup 期 raise 時 uvicorn 會記
    `Application startup failed. Exiting.` 並以非 0 退出，container 照樣重啟）：
      - `python http_server.py`（dev / live compose 用）→ 檔尾的 uvicorn.run(app)
      - `uvicorn http_server:app`（start_server.sh 用）

    放在 import 期則等於「import 這個模組 == 必須連得到 DB」，會綁架所有離線工具與
    FastAPI TestClient——`/sr-scoring/evaluate` 長期沒有測試就是被這一行擋住的。
    見 docs/development-workflow.md §4「模組 import 不得有連線等副作用」。
    """
    log.info("connecting to database...")
    check_connection()
    yield


app = FastAPI(title="Trading Backtest Service", version="1.0.0", lifespan=lifespan)


class BacktestRequest(BaseModel):
    job_id: str
    strategy: str
    symbols: Union[str, List[str]]   # 接受 JSON string 或 list
    timeframe: str = "1d"
    start_date: str
    end_date: str
    use_chip_filter: bool = False
    chip_min_score: float = 0.0

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
            use_chip_filter=req.use_chip_filter,
            chip_min_score=req.chip_min_score,
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


def _backtest_job_payload(job_row) -> dict:
    """把 backtest_jobs 的 DB 列轉成對外的 job 形狀。

    `SELECT *` 會讓任何 schema 變動直接漏到 API 回應，所以 DB 欄位名與對外欄位名
    不同的地方要在這裡明確轉回來。目前只有一個：`trigger` 是 MySQL 保留字，
    DB 欄位已改名為 `trigger_source`（migration 059，見 issue.md I-054），
    但對外的欄位名維持 `trigger`——Go 端 `store.BacktestJob` 的 json tag 也是這樣。
    """
    job = dict(job_row)
    if "trigger_source" in job:
        job["trigger"] = job.pop("trigger_source")
    return job


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
        "job":    _backtest_job_payload(job_row),
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
    previous_event_states: List[dict] = Field(default_factory=list)
    # 產生 previous_event_states 那次分析站在哪根 K 棒（RFC3339）。只用來決定事件要不要
    # 老化——同一根 K 棒重複分析不應該讓事件提早 EXPIRED（issue.md I-077）。
    # 省略＝維持舊行為。
    previous_analyzed_at: Optional[str] = None


@app.post("/sr-zones")
async def sr_zones(req: ScoreZonesRequest):
    """支撐/壓力機率評分：對每個 zone 回傳 support_score/resistance_score
    （規則式，永遠可算）與 bounce_probability/break_probability（需要先跑過
    sr_scoring/train.py 產生模型，否則回 503）。回應頂層另外帶
    model_version/model_trained_at/model_feature_names，來自產生這次結果的
    ModelBundle，供追蹤「這筆分析是哪個模型版本算出來的」。"""
    log.info(
        "POST /sr-zones symbol=%s tf=%s limit=%d previous_event_states=%d",
        req.symbol, req.timeframe, req.limit, len(req.previous_event_states),
    )
    try:
        return score_symbol(
            req.symbol, req.timeframe, req.limit,
            previous_event_states=req.previous_event_states,
            previous_analyzed_at=req.previous_analyzed_at,
        )
    except ValueError as exc:
        raise HTTPException(status_code=404, detail=str(exc))
    except RuntimeError as exc:
        raise HTTPException(status_code=503, detail=str(exc))


class TrainRequest(BaseModel):
    symbols: Optional[List[str]] = None
    timeframe: str = "1d"
    limit: int = 1500
    model_type: str = "gradient_boosting"
    # split_method="time"（預設，依 touch_time 逐股票切分 holdout，避免用
    # 未來資料驗證過去高估表現）或 "random"（舊行為，保留供比較）。
    # calibration_method 資料太少時會自動降級為不校準，見 model.py 說明。
    split_method: str = "time"
    calibration_method: Optional[str] = "sigmoid"


@app.post("/sr-scoring/train")
async def sr_scoring_train(req: TrainRequest):
    """手動觸發 sr_scoring 機率模型訓練（同步執行，視資料量可能耗時數十秒到
    數分鐘；Go 端會用背景 goroutine 呼叫，不會卡住 HTTP 回應）。"""
    log.info(
        "POST /sr-scoring/train symbols=%s tf=%s limit=%d model_type=%s split_method=%s",
        req.symbols, req.timeframe, req.limit, req.model_type, req.split_method,
    )
    try:
        return run_training(
            symbols=req.symbols,
            timeframe=req.timeframe,
            limit=req.limit,
            model_type=req.model_type,
            split_method=req.split_method,
            calibration_method=req.calibration_method,
        )
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc))


class SREvaluationRequest(BaseModel):
    symbols: List[str]
    timeframe: str = "1d"
    limit: int = DEFAULT_EVALUATION_LIMIT
    model_path: Optional[str] = None
    write_db: bool = False
    decision_replay: bool = False
    replay_max_rows: int = DEFAULT_REPLAY_MAX_ROWS
    run_id: Optional[str] = None
    pipeline_version: Optional[str] = None
    passed: Optional[bool] = None
    min_history_bars: int = 80
    rebuild_every_bars: int = 5
    forward_bars: int = 5
    threshold_pct: float = 0.03
    atr_width_multiplier: float = 1.5
    max_merge_width_multiple: float = 2.0
    atr_lookback: int = 60
    atr_period: int = 14
    chip_scores_by_symbol: Optional[dict[str, list[dict[str, Any]]]] = None
    model_governance_by_symbol: Optional[dict[str, list[dict[str, Any]]]] = None


@app.post("/sr-scoring/evaluate")
async def sr_scoring_evaluate(req: SREvaluationRequest):
    """手動觸發 SR Zone evaluation / decision replay。

    這支 API 是 CLI evaluation 的 HTTP 包裝：Go 端可用它手動產生 regression
    report，並在 write_db=true 時由 Python 寫入 stock_sr_regression_results。
    第一版只接受 DB symbols，不開放遠端指定 CSV 路徑。
    """
    import config

    symbols = [symbol.strip() for symbol in req.symbols if symbol.strip()]
    if not symbols:
        raise HTTPException(status_code=400, detail="symbols is required")
    if req.limit <= 0:
        raise HTTPException(status_code=400, detail="limit must be > 0")
    # replay_max_rows 只有 decision_replay 模式才有意義；非 replay 模式送 0 是 Go 端的正常
    # 語意，不該回 400。
    if req.decision_replay and req.replay_max_rows <= 0:
        raise HTTPException(status_code=400, detail="replay_max_rows must be > 0 when decision_replay is enabled")

    log.info(
        "POST /sr-scoring/evaluate symbols=%s tf=%s limit=%d decision_replay=%s write_db=%s",
        symbols, req.timeframe, req.limit, req.decision_replay, req.write_db,
    )

    dataset_config = DatasetConfig(
        min_history_bars=req.min_history_bars,
        rebuild_every_bars=req.rebuild_every_bars,
        forward_bars_support=req.forward_bars,
        forward_bars_resistance=req.forward_bars,
        threshold_pct_support=req.threshold_pct,
        threshold_pct_resistance=req.threshold_pct,
    )
    model_path = req.model_path or config.SR_SCORING_MODEL_PATH
    pipeline_version = req.pipeline_version or (
        # 與 evaluation.run_decision_replay 的預設一致；schema_version 仍是 p0（gate 用它過濾）。
        "sr_zone_decision_replay_p1" if req.decision_replay else DEFAULT_PIPELINE_VERSION
    )

    # builder config 在分支外組好給兩條路徑共用。先前只有 evaluation 分支組，decision replay
    # 分支漏傳，四個 ATR 參數在 replay 模式被靜默忽略（與 CLI 曾有的陷阱相同）。放在分支外
    # 就不可能再有哪一支忘了帶。四個欄位的預設值與 ATRZoneBuilderConfig 相同，所以呼叫端
    # 沒指定時等同於不傳。
    builder_config = ZoneBuilderConfig(
        atr=ATRZoneBuilderConfig(
            lookback=req.atr_lookback,
            atr_period=req.atr_period,
            atr_width_multiplier=req.atr_width_multiplier,
            max_merge_width_multiple=req.max_merge_width_multiple,
        )
    )

    try:
        if req.decision_replay:
            report = run_decision_replay(
                symbols=symbols,
                timeframe=req.timeframe,
                limit=req.limit,
                model_path=model_path,
                dataset_config=dataset_config,
                builder_config=builder_config,
                replay_max_rows=req.replay_max_rows,
                chip_scores_by_symbol=req.chip_scores_by_symbol,
                model_governance_by_symbol=req.model_governance_by_symbol,
                run_id=req.run_id,
                pipeline_version=pipeline_version,
            )
        else:
            report = run_evaluation(
                symbols=symbols,
                timeframe=req.timeframe,
                limit=req.limit,
                model_path=model_path,
                dataset_config=dataset_config,
                builder_config=builder_config,
                run_id=req.run_id,
                pipeline_version=pipeline_version,
            )
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc))
    except RuntimeError as exc:
        raise HTTPException(status_code=503, detail=str(exc))

    if req.write_db:
        write_evaluation_result(report, passed=req.passed)
    return report


class ZoneIdentityZone(BaseModel):
    """matcher 需要的最小 zone 形狀。`price_low`/`price_high` 不是身分，只是形狀。"""

    price_low: float
    price_high: float
    method: str
    role: str
    # 以下只有 previous 側會帶
    zone_uid: Optional[str] = None
    incarnation_role: Optional[str] = None
    last_seen_at: Optional[str] = None      # YYYY-MM-DD
    observed_absences: int = 0


class ZoneIdentityMatchRequest(BaseModel):
    as_of: Optional[str] = None             # YYYY-MM-DD，缺就不套時間軸閘門
    # 市場交易日。**可以是降冪**——from_iterable 會排序去重，
    # 直接把 db.fetch_market_trading_days() 的輸出丟進來即可。
    trading_days: List[str] = Field(default_factory=list)
    previous: List[ZoneIdentityZone] = Field(default_factory=list)
    current: List[ZoneIdentityZone] = Field(default_factory=list)


@app.post("/zone-identity/match")
async def zone_identity_match(req: ZoneIdentityMatchRequest):
    """Zone 跨交易日的身分匹配（T-048 階段 B 接線）。

    **刻意是獨立端點而不是塞進 /sr-zones**：階段 B 是「只寫不讀」，沒有任何決策依賴
    它的輸出。把它掛進 /sr-zones 就得動 scoring.py / pipeline.py——那是有決策責任的
    核心路徑，為一個還沒有讀者的功能去動它，風險與收益不成比例。

    這支是 `zone_matcher.match_zones` 的純函數包裝：不碰 DB、不改任何既有回應。
    """
    log.info("POST /zone-identity/match previous=%d current=%d",
             len(req.previous), len(req.current))

    calendar = (
        zone_matcher.TradingCalendar.from_iterable(req.trading_days)
        if req.trading_days else None
    )
    as_of = date.fromisoformat(req.as_of) if req.as_of else None

    previous = [
        zone_matcher.PreviousZone(
            zone_uid=z.zone_uid or "",
            price_low=z.price_low,
            price_high=z.price_high,
            method=z.method,
            role=z.role,
            incarnation_role=z.incarnation_role,
            last_seen_at=date.fromisoformat(z.last_seen_at) if z.last_seen_at else None,
            observed_absences=z.observed_absences,
        )
        for z in req.previous
    ]
    current = [
        zone_matcher.CandidateZone(z.price_low, z.price_high, z.method, z.role)
        for z in req.current
    ]

    result = zone_matcher.match_zones(previous, current, as_of=as_of, calendar=calendar)

    return {
        "zone_uids": result.zone_uids,
        "incarnation_roles": result.incarnation_roles,
        "relations": [
            {"parent_zone_uid": r.parent_zone_uid,
             "child_zone_uid": r.child_zone_uid,
             "relation": r.relation}
            for r in result.relations
        ],
        "role_transitions": [
            {"zone_uid": t.zone_uid, "kind": t.kind,
             "from_role": t.from_role, "to_role": t.to_role}
            for t in result.role_transitions
        ],
        "unmatched_previous": result.unmatched_previous,
        "terminated_previous": result.terminated_previous,
        "expired_previous": result.expired_previous,
        "next_observed_absences": result.next_observed_absences,
    }


@app.get("/sr-scoring/model-status")
async def sr_scoring_model_status():
    """查詢目前機率模型的狀態：存不存在、版本、訓練時間、路徑、metrics 摘要、
    feature 名稱。不像 /sr-zones 那樣在模型不存在時丟 503——這支端點的用途
    就是讓前端在呼叫 /sr-zones 之前先知道「模型準備好了沒」，所以永遠回
    200，用 exists 欄位表示狀態。"""
    import config

    try:
        bundle = get_model()
    except RuntimeError:
        return {
            "exists": False, "version": None, "trained_at": None,
            "model_path": config.SR_SCORING_MODEL_PATH, "split_method": None,
            "metrics": None, "feature_names": None, "config_hash": None, "training_config": None,
        }

    return {
        "exists": True,
        "version": bundle.version,
        "trained_at": bundle.trained_at,
        "model_path": config.SR_SCORING_MODEL_PATH,
        "split_method": bundle.split_method,
        "metrics": bundle.metrics,
        "feature_names": bundle.feature_names,
        "config_hash": bundle.config_hash,
        "training_config": bundle.training_config,
    }


@app.get("/health")
async def health():
    return {"status": "ok"}


if __name__ == "__main__":
    import uvicorn

    # log_config=None：不讓 uvicorn 套用它自己的 logging 設定。uvicorn 預設會把 uvicorn /
    # uvicorn.error / uvicorn.access 設成 propagate=False 並掛自己的 stderr/stdout handler，
    # 導致 startup / access / error（含 500 的 ASGI traceback）都停在 uvicorn logger、傳不到
    # root，因此不會進 configure_logging 設定的持久化檔。設成 None 後，這些 log 交由 root 的
    # stdout + 每日輪替檔案 handler 統一承接，全部一起持久化（見 development-workflow.md「開發慣例」）。
    uvicorn.run(app, host="0.0.0.0", port=SERVICE_PORT, log_config=None)
