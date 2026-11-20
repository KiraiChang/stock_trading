"""測試用的 `db.fetch_candles_range` 替身（I-137）：在記憶體裡的 K 棒列上做同樣的事。

範圍段＝台北日期 `start_date`～`end_date`（含頭含尾），`LIMIT budget + 1`；範圍段是 0 根時⛔ 不查暖身段；
暖身段＝起點之前最近 `min(warmup_bars, 剩下 + 1)` 根。真的 SQL 版本由 `test_db_fetch_candles_range.py` 在 SQLite 上驗證，
兩者的語意要一致。`calls` 記下每一次呼叫（含暖身段有沒有查），讓測試確認查詢次數。
"""
from __future__ import annotations

from datetime import date, datetime, timedelta, timezone

from db import CandleRange

TAIPEI = timezone(timedelta(hours=8))


def _day_start(d: str) -> int:
    return int(datetime.combine(date.fromisoformat(d), datetime.min.time(), tzinfo=TAIPEI).timestamp())


class FakeRangeFetch:
    def __init__(self, data: dict[str, list[dict]]):
        self.data = data
        self.calls: list[dict] = []

    def __call__(self, symbol, timeframe, start_date, end_date, warmup_bars, budget, adjusted=True, conn=None):
        rows = sorted(self.data.get(symbol, []), key=lambda r: r["timestamp"])
        lo = _day_start(str(start_date))
        hi = _day_start((date.fromisoformat(str(end_date)) + timedelta(days=1)).isoformat())
        in_range = [dict(r) for r in rows if lo <= r["timestamp"] < hi][: budget + 1]
        call = {"symbol": symbol, "budget": budget, "warmup_queried": False, "warmup_limit": None}
        self.calls.append(call)
        out = CandleRange(in_range=in_range)
        if len(in_range) > budget:
            out.exceeded = True
        elif in_range:
            left = budget - len(in_range)
            limit = min(warmup_bars, left + 1)
            call["warmup_limit"] = limit
            if limit > 0:
                call["warmup_queried"] = True
                before = [dict(r) for r in rows if r["timestamp"] < lo]
                out.warmup = before[-limit:] if limit else []
            if len(out.warmup) > left:
                out.exceeded = True
        return out
