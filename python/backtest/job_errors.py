"""回測任務的請求驗證與對外錯誤代碼（I-137）。

`backtest_jobs.error` 會由 Go 的 `GET /api/v1/backtest`、`/:job_id` 與 python-server 的 `GET /backtest/{job_id}` 回傳、
前端顯示，所以**只能寫封閉值域的代碼**（docs/architecture.md「寫入失敗的一致性契約」）：

* 已知的錯誤用 `BacktestJobError` 帶出代碼，參數只放本專案自己組的值（欄位名、整數、格式合法的股票代號）。
* 其他所有例外一律 `internal_error`，原始例外只進 log——⛔ 不寫 `str(exc)`（可能帶 SQL、參數或其他內部細節）。
* 讀出時 `public_job_error` 再比對一次允許規則：I-137 之前留下的原始文字一律改回 `internal_error`。
  Go 端（`backend/internal/backtest`）實作同一份規則，兩邊的測試案例檔內容必須相同
  （`scripts/check-backtest-contract.py` 比對）。
"""
from __future__ import annotations

import re
from datetime import date

# 取數的上界是「結束日的隔天 00:00（台北）」，9999-12-31 加一天會溢位。Go 端用同一個值（腳本比對）。
MAX_DATE = "2099-12-31"
# 年份 0000 python 的 date 本來就不收；Go 端的 MinDate 跟這裡相同（腳本比對）。
MIN_DATE = "0001-01-01"

INTERNAL_ERROR = "internal_error"
NO_DATA_IN_RANGE = "no_data_in_range"
UNKNOWN_STRATEGY = "unknown_strategy"
START_AFTER_END = "start_after_end"

_SYMBOL_RE = re.compile(r"[0-9A-Z]{1,12}")

# 允許的對外形式（整串比對：`fullmatch`，⛔ 不用 `$`——它會放過結尾多一個換行；Go 的 `^…$` 沒有這個問題）。
# `cancelled by user` 是 Go 取消任務寫的既有文字，`interrupted_on_restart` 是 I-134 的既有代碼。
_PUBLIC_PATTERNS = [
    re.compile(r"invalid_date:(start_date|end_date)"),
    re.compile(r"start_after_end"),
    re.compile(r"too_many_symbols:[0-9]{1,9}:[0-9]{1,9}"),
    re.compile(r"backtrader_single_symbol:[0-9]{1,9}"),
    re.compile(r"task_too_large:([0-9A-Z]{1,12}|\?):[0-9]{1,12}:[0-9]{1,12}"),
    re.compile(r"symbol_too_large:([0-9A-Z]{1,12}|\?):[0-9]{1,12}:[0-9]{1,12}"),
    re.compile(r"insufficient_data:([0-9A-Z]{1,12}|\?):[0-9]{1,12}:[0-9]{1,12}"),
    re.compile(r"no_data_in_range"),
    re.compile(r"unknown_strategy"),
    re.compile(r"interrupted_on_restart"),
    re.compile(r"internal_error"),
    re.compile(r"cancelled by user"),
]


class BacktestJobError(ValueError):
    """已知、可以公開原因的回測失敗。`code` 是寫進 `backtest_jobs.error` 的值。"""

    def __init__(self, code: str):
        super().__init__(code)
        self.code = code


def safe_symbol(symbol: object) -> str:
    """錯誤代碼裡的股票代號：格式不合法（例如使用者送了奇怪的字串）就用 `?`，⛔ 不把原始輸入放進代碼。"""
    s = str(symbol)
    return s if _SYMBOL_RE.fullmatch(s) else "?"


def task_too_large(symbol: object, at_least: int, maximum: int) -> BacktestJobError:
    """先碰到任務的總額度（所有標的合計）：要減少檔數或縮短範圍。根數是下限。
    ⚠️ ⛔ 不代表這一檔本身合格：查詢在總額度用完就停了，它本身有沒有超過每一檔的上限不知道。"""
    return BacktestJobError(f"task_too_large:{safe_symbol(symbol)}:{int(at_least)}:{int(maximum)}")


def symbol_too_large(symbol: object, at_least: int, maximum: int) -> BacktestJobError:
    """單一檔的範圍段＋暖身段超過每一檔的上限：要縮短日期範圍（減少檔數沒有用）。根數是下限。"""
    return BacktestJobError(f"symbol_too_large:{safe_symbol(symbol)}:{int(at_least)}:{int(maximum)}")


def insufficient_data(symbol: object, got: int, need: int) -> BacktestJobError:
    """backtrader：實際載入的範圍段＋暖身段比策略的 minperiod 還少，指標初始化不了（backtrader 會丟 IndexError）。"""
    return BacktestJobError(f"insufficient_data:{safe_symbol(symbol)}:{int(got)}:{int(need)}")


def parse_date(field: str, raw: object) -> date:
    """`YYYY-MM-DD`、存在的日期、⛔ 晚於 MAX_DATE。不符就 `invalid_date:<欄位>`。"""
    s = raw.isoformat() if isinstance(raw, date) else raw
    if not isinstance(s, str) or not re.fullmatch(r"[0-9]{4}-[0-9]{2}-[0-9]{2}", s):
        raise BacktestJobError(f"invalid_date:{field}")
    try:
        d = date.fromisoformat(s)
    except ValueError:
        raise BacktestJobError(f"invalid_date:{field}") from None
    if d > date.fromisoformat(MAX_DATE):
        raise BacktestJobError(f"invalid_date:{field}")
    return d


def validate_request(
    strategy: str,
    symbols: list[str],
    start_date: object,
    end_date: object,
    *,
    known_strategies: set[str],
    backtrader_strategies: set[str],
    max_symbols: int,
) -> tuple[str, str]:
    """兩條執行路徑（server、worker）共用的請求檢查。回傳正規化的 `(start_date, end_date)` 字串。

    跟 Go 的 `POST /backtest` 做同樣的檢查（Go 回 400），這裡讓繞過 Go 或修正之前已經排隊的任務明確 `failed`。
    """
    if strategy not in known_strategies:
        raise BacktestJobError(UNKNOWN_STRATEGY)
    start = parse_date("start_date", start_date)
    end = parse_date("end_date", end_date)
    if start > end:
        raise BacktestJobError(START_AFTER_END)
    n = len(symbols)
    if n > max_symbols:
        raise BacktestJobError(f"too_many_symbols:{n}:{max_symbols}")
    if strategy in backtrader_strategies and n > 1:
        raise BacktestJobError(f"backtrader_single_symbol:{n}")
    return start.isoformat(), end.isoformat()


def job_error_code(exc: BaseException) -> str:
    """寫進 `backtest_jobs.error` 的值：已知的錯誤用它的代碼，其他一律 `internal_error`。"""
    if isinstance(exc, BacktestJobError) and is_public_job_error(exc.code):
        return exc.code
    return INTERNAL_ERROR


def is_public_job_error(value: str) -> bool:
    return any(p.fullmatch(value) for p in _PUBLIC_PATTERNS)


def public_job_error(value: object) -> object:
    """讀出端點回傳 `error` 之前的轉換：NULL 與空字串照舊，符合允許規則的照舊，其他一律 `internal_error`。"""
    if value is None or value == "":
        return value
    if isinstance(value, str) and is_public_job_error(value):
        return value
    return INTERNAL_ERROR
