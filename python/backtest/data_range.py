"""回測依日期範圍取 K 棒（I-137）：兩條路徑（backtrader、modular）共用。

* 每一檔取「範圍段」（台北日期 `start_date`～`end_date`，含頭含尾）與起點之前最多 `warmup_bars` 根「暖身段」。
  ⛔ 不再先取最近 N 根再切（範圍的前段會被靜默截掉）。
* **上限**（範圍段＋暖身段）：每一檔 `max_symbol_bars`、所有標的合計 `max_task_bars`。依序取完所有標的才開始跑；每一檔帶
  `min(每一檔的上限, 任務剩下的額度)`，單一檔超過 raise `symbol_too_large`、合計超過 raise `task_too_large`（⛔ 不截斷、⛔ 不跑一半）。
  代碼裡的根數是**下限**（查詢在額度 + 1 根就停了）。
* **覆蓋範圍**：每一檔記實際的首尾日期、根數、暖身根數與是否被略過；範圍內 0 根的那一檔略過（⛔ 不查暖身、⛔ 不扣額度）。
  範圍內的缺漏⛔ 不偵測（沒有獨立的交易日依據）。全部被略過就 raise `no_data_in_range`。
"""
from __future__ import annotations

import logging
from dataclasses import dataclass
from typing import Callable

import pandas as pd

from db import fetch_candles_range

from .job_errors import NO_DATA_IN_RANGE, BacktestJobError, symbol_too_large, task_too_large

log = logging.getLogger(__name__)

_COLS = ["open", "high", "low", "close", "volume"]


@dataclass
class SymbolData:
    symbol: str
    df: pd.DataFrame  # 暖身段＋範圍段，索引是台北時間，OHLCV 是 float
    r0: int  # 範圍內第一根在 df 裡的位置（＝暖身根數）
    coverage: dict


def to_dataframe(rows: list[dict]) -> pd.DataFrame:
    df = pd.DataFrame(rows)
    df["datetime"] = pd.to_datetime(df["timestamp"], unit="s", utc=True).dt.tz_convert("Asia/Taipei")
    df = df.set_index("datetime").sort_index()
    # candles 的價格在 Postgres/MySQL 是 DECIMAL，psycopg2/pymysql 回傳 decimal.Decimal；統一轉 float，
    # 避免後續數值運算跟 Decimal 混用出現 TypeError。
    return df[_COLS].astype(float)


def _taipei_date(epoch: int | float) -> str:
    return pd.Timestamp(int(epoch), unit="s", tz="UTC").tz_convert("Asia/Taipei").date().isoformat()


def load_task_candles(
    symbols: list[str],
    timeframe: str,
    start_date: str,
    end_date: str,
    *,
    warmup_bars: int,
    max_task_bars: int,
    max_symbol_bars: int | None = None,
    fetch: Callable | None = None,
) -> tuple[list[SymbolData], list[dict]]:
    """回傳 (有資料的標的, 每一檔的覆蓋範圍)。覆蓋範圍的順序跟 `symbols` 相同，含被略過的。

    `fetch` 預設是 `db.fetch_candles_range`（呼叫時才查名稱，測試可以換掉模組的 `fetch_candles_range`）。
    """
    fetch = fetch or fetch_candles_range
    used = 0
    loaded: list[SymbolData] = []
    coverage: list[dict] = []
    for symbol in symbols:
        remaining = max_task_bars - used
        # 每一檔的額度＝min(每一檔的上限, 任務剩下的額度)。查詢在額度 + 1 根就停，所以只知道「先碰到哪一個」：
        # 每一檔的上限比較小 → symbol_too_large（這一檔本身就超過，要縮短範圍）；
        # 任務剩下的額度比較小 → task_too_large（先碰到任務的總額度）。⚠️ 這時⛔ 不知道這一檔本身有沒有超過每一檔的上限
        # （例如只剩 1,000 根額度、這一檔有 6,000 根，查到第 1,001 根就停了），訊息只能說「合計超過」。
        symbol_cap_binds = max_symbol_bars is not None and max_symbol_bars <= remaining
        budget = max_symbol_bars if symbol_cap_binds else remaining
        cr = fetch(symbol, timeframe, start_date, end_date, warmup_bars, budget)
        if cr.exceeded:
            if symbol_cap_binds:
                raise symbol_too_large(symbol, cr.bars_read, max_symbol_bars)
            raise task_too_large(symbol, used + cr.bars_read, max_task_bars)
        if not cr.in_range:
            log.warning("symbol=%s tf=%s — no data in range %s~%s, skipped", symbol, timeframe, start_date, end_date)
            coverage.append({
                "symbol": symbol, "first_bar": None, "last_bar": None, "bars": 0,
                "warmup_bars": 0, "warmup_required": warmup_bars, "skipped": True,
            })
            continue
        used += cr.bars_read
        cov = {
            "symbol": symbol,
            "first_bar": _taipei_date(cr.in_range[0]["timestamp"]),
            "last_bar": _taipei_date(cr.in_range[-1]["timestamp"]),
            "bars": len(cr.in_range),
            "warmup_bars": len(cr.warmup),
            "warmup_required": warmup_bars,
            "skipped": False,
        }
        coverage.append(cov)
        loaded.append(SymbolData(symbol=symbol, df=to_dataframe(cr.warmup + cr.in_range), r0=len(cr.warmup), coverage=cov))
        log.info("symbol=%s loaded %d candles in range (%s ~ %s) + %d warmup",
                 symbol, cov["bars"], cov["first_bar"], cov["last_bar"], cov["warmup_bars"])
    if not loaded:
        raise BacktestJobError(NO_DATA_IN_RANGE)
    return loaded, coverage
