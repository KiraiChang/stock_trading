"""I-137：回測依日期範圍取 K 棒——請求驗證、任務額度、覆蓋範圍、暖身與交易起點、範圍內的統計、截斷的回歸。

* 額度與覆蓋範圍用 `candle_range_fake.FakeRangeFetch`（跟 `db.fetch_candles_range` 同語意，後者由
  `test_db_fetch_candles_range.py` 在 SQLite 上驗）；截斷的回歸走真的 `db.fetch_candles_range`＋SQLite。
* backtrader 的統計比較：暖身本身可能改變訊號與成交，所以用「照日期下單」的策略讓兩邊的成交與權益序列相同。
"""
from __future__ import annotations

import json
import math
import sqlite3
import sys
from datetime import date, datetime, timedelta, timezone
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "backtest"))

import backtrader as bt
import numpy as np
import pytest
from sqlalchemy import create_engine, text

import db
from backtest import data_range, engine, job_errors
from backtest.job_errors import BacktestJobError
from backtest.modular import service as modular_service
from backtest.modular.backtester import BacktestEngine
from backtest.modular import backtester as modular_backtester
from backtest.modular.strategy import STRATEGY_PRESETS, build_strategy
from backtest.range_stats import RangeValue, range_stats
from candle_range_fake import FakeRangeFetch
from config import BACKTEST_WARMUP_BARS
from strategy.base import BaseTWStrategy, bar_taipei_date
from strategy import breakout_v1

TAIPEI = timezone(timedelta(hours=8))
D0 = date(2020, 1, 1)
CASES = Path(__file__).resolve().parent / "data" / "backtest_job_error_cases.json"


def _rows(n: int, start: date = D0, *, prices=None, volumes=None) -> list[dict]:
    """從 `start` 起每天一根、台北 00:00 的日 K。"""
    rows = []
    for i in range(n):
        p = float(prices[i]) if prices is not None else 100.0
        rows.append({
            "timestamp": int(datetime.combine(start + timedelta(days=i), datetime.min.time(), tzinfo=TAIPEI).timestamp()),
            "open": p, "high": p * 1.01, "low": p * 0.99, "close": p,
            "volume": float(volumes[i]) if volumes is not None else 1000.0,
        })
    return rows


def _day(i: int) -> str:
    return (D0 + timedelta(days=i)).isoformat()


# ── 請求驗證（共用入口） ─────────────────────────────────────────────────


def _validate(strategy="breakout_v1", symbols=("2330",), start="2025-01-01", end="2025-12-31", max_symbols=20):
    return job_errors.validate_request(
        strategy, list(symbols), start, end,
        known_strategies={"breakout_v1", "breakout_swing_atr_v1"}, backtrader_strategies={"breakout_v1"},
        max_symbols=max_symbols,
    )


@pytest.mark.parametrize("start,end,code", [
    ("2025/01/01", "2025-12-31", "invalid_date:start_date"),
    ("2025-1-1", "2025-12-31", "invalid_date:start_date"),
    ("2025-02-30", "2025-12-31", "invalid_date:start_date"),
    ("2025-01-01", "", "invalid_date:end_date"),
    ("2025-01-01", "9999-12-31", "invalid_date:end_date"),  # 合法日期，但加一天會溢位
    ("2025-01-01", "2100-01-01", "invalid_date:end_date"),
    ("2025-06-02", "2025-06-01", "start_after_end"),
])
def test_invalid_dates_are_rejected(start, end, code):
    with pytest.raises(BacktestJobError) as e:
        _validate(start=start, end=end)
    assert e.value.code == code


def test_same_day_and_max_date_pass():
    assert _validate(start="2025-06-02", end="2025-06-02") == ("2025-06-02", "2025-06-02")
    assert _validate(start="2099-12-31", end="2099-12-31") == ("2099-12-31", "2099-12-31")


def test_date_objects_from_postgres_are_accepted():
    assert _validate(start=date(2025, 1, 1), end=date(2025, 2, 1)) == ("2025-01-01", "2025-02-01")


def test_symbol_limit_and_backtrader_single_symbol():
    assert _validate("breakout_swing_atr_v1", symbols=("A", "B"), max_symbols=2)
    with pytest.raises(BacktestJobError) as e:
        _validate("breakout_swing_atr_v1", symbols=("A", "B", "C"), max_symbols=2)
    assert e.value.code == "too_many_symbols:3:2"
    with pytest.raises(BacktestJobError) as e:
        _validate("breakout_v1", symbols=("A", "B"))
    assert e.value.code == "backtrader_single_symbol:2"
    with pytest.raises(BacktestJobError) as e:
        _validate("nope")
    assert e.value.code == "unknown_strategy"


def test_run_backtest_validates_before_touching_the_db(monkeypatch):
    """兩條執行路徑共用 `run_backtest`：不合法的任務在取資料之前就失敗。"""
    def no_db(*a, **k):
        raise AssertionError("⛔ 不能查資料")

    monkeypatch.setattr(data_range, "fetch_candles_range", no_db)
    monkeypatch.setattr(engine, "BACKTEST_MAX_SYMBOLS", 2)
    for kwargs, code in [
        ({"strategy": "breakout_v1", "symbols": ["A", "B"]}, "backtrader_single_symbol:2"),
        ({"strategy": "breakout_swing_atr_v1", "symbols": ["A", "B", "C"]}, "too_many_symbols:3:2"),
        ({"strategy": "breakout_v1", "symbols": ["A"], "end_date": "9999-12-31"}, "invalid_date:end_date"),
    ]:
        args = {"timeframe": "1d", "start_date": "2025-01-01", "end_date": "2025-12-31", **kwargs}
        with pytest.raises(BacktestJobError) as e:
            engine.run_backtest(**args)
        assert e.value.code == code


# ── 對外錯誤代碼（跟 Go 共用同一份案例檔） ─────────────────────────────────


def test_public_job_error_cases_shared_with_go():
    cases = json.loads(CASES.read_text(encoding="utf-8"))
    assert cases, "案例檔不能是空的"
    for c in cases:
        assert job_errors.public_job_error(c["input"]) == c["want"], c


def test_job_error_code_hides_unexpected_exceptions():
    assert job_errors.job_error_code(RuntimeError("boom at SELECT * FROM users WHERE pw='x'")) == "internal_error"
    assert job_errors.job_error_code(BacktestJobError("no_data_in_range")) == "no_data_in_range"
    # 代碼本身不合規則（例如程式組錯）也⛔ 不放行
    assert job_errors.job_error_code(BacktestJobError("task_too_large:<script>:1:2")) == "internal_error"


def test_unsafe_symbol_is_replaced_in_codes():
    assert job_errors.task_too_large("2330", 10, 9).code == "task_too_large:2330:10:9"
    assert job_errors.task_too_large("x'; DROP", 10, 9).code == "task_too_large:?:10:9"


# ── 任務額度與覆蓋範圍 ───────────────────────────────────────────────────


def _load(data, start, end, *, warmup=5, budget=100, fake=None):
    fake = fake or FakeRangeFetch(data)
    return data_range.load_task_candles(list(data), "1d", start, end, warmup_bars=warmup, max_task_bars=budget, fetch=fake), fake


def test_coverage_records_actual_dates_counts_and_warmup():
    (loaded, cov), _ = _load({"A": _rows(30)}, _day(10), _day(19))
    assert cov == [{"symbol": "A", "first_bar": _day(10), "last_bar": _day(19), "bars": 10,
                    "warmup_bars": 5, "warmup_required": 5, "skipped": False}]
    assert loaded[0].r0 == 5 and len(loaded[0].df) == 15


def test_start_on_weekend_shows_next_bar_and_is_not_warmup_short():
    rows = [r for r in _rows(40) if datetime.fromtimestamp(r["timestamp"], TAIPEI).weekday() < 5]
    (_, cov), _ = _load({"A": rows}, "2020-01-11", "2020-01-20", warmup=3)  # 2020-01-11 是週六
    assert cov[0]["first_bar"] == "2020-01-13" and cov[0]["warmup_bars"] == 3


def test_first_bar_on_start_day_without_history_is_complete_dates_but_zero_warmup():
    (_, cov), _ = _load({"A": _rows(10)}, _day(0), _day(9), warmup=250)
    assert cov[0]["first_bar"] == _day(0) and cov[0]["warmup_bars"] == 0 and cov[0]["warmup_required"] == 250


def test_gap_inside_range_is_not_detected():
    """範圍中間缺一段：只記首尾與根數（⛔ 不偵測缺漏：沒有獨立的交易日依據）。"""
    rows = _rows(40)
    rows = rows[:15] + rows[30:]
    (_, cov), _ = _load({"A": rows}, _day(10), _day(39))
    assert (cov[0]["first_bar"], cov[0]["last_bar"], cov[0]["bars"]) == (_day(10), _day(39), 15)


def test_warmup_pushes_task_over_the_limit():
    """範圍段沒超過、加上暖身才超過 → task_too_large，訊息是「至少 n 根」。"""
    with pytest.raises(BacktestJobError) as e:
        _load({"A": _rows(300)}, _day(290), _day(299), warmup=250, budget=10)
    assert e.value.code == "task_too_large:A:11:10"


def test_second_symbol_over_the_limit_fails_before_running(monkeypatch):
    ran = []
    monkeypatch.setattr(BacktestEngine, "run", lambda self, *a, **k: ran.append(a))
    monkeypatch.setattr(data_range, "fetch_candles_range", FakeRangeFetch({"A": _rows(40), "B": _rows(40)}))
    with pytest.raises(BacktestJobError) as e:
        modular_service.run_modular_backtest("breakout_swing_atr_v1", ["A", "B"], "1d", _day(20), _day(39),
                                             warmup_bars=5, max_task_bars=30)
    assert e.value.code == "task_too_large:B:31:30"  # A 用了 25 根，B 取到 5 + 1 根就停
    assert ran == []  # 先取後跑：⛔ 沒有開始跑任何一檔


def test_exhausted_budget_then_symbol_with_only_warmup_history_is_skipped():
    """額度剛好用完，下一檔範圍內沒有資料、只有暖身段的歷史 → 那一檔略過（⛔ 不查暖身段），任務照常。"""
    data = {"A": _rows(30), "B": _rows(5)}  # B 只有範圍之前的 5 根
    (loaded, cov), fake = _load(data, _day(10), _day(29), warmup=10, budget=30)
    assert [c["symbol"] for c in cov] == ["A", "B"] and [sd.symbol for sd in loaded] == ["A"]
    assert cov[1] == {"symbol": "B", "first_bar": None, "last_bar": None, "bars": 0,
                      "warmup_bars": 0, "warmup_required": 10, "skipped": True}
    assert fake.calls[1]["budget"] == 0 and fake.calls[1]["warmup_queried"] is False


def test_all_symbols_skipped_is_no_data_in_range():
    with pytest.raises(BacktestJobError) as e:
        _load({"A": _rows(5)}, _day(100), _day(120))
    assert e.value.code == "no_data_in_range"


def test_exact_task_limit_passes():
    (_, cov), _ = _load({"A": _rows(30)}, _day(10), _day(19), warmup=5, budget=15)
    assert cov[0]["bars"] + cov[0]["warmup_bars"] == 15


# ── 暖身、交易起點（backtrader） ───────────────────────────────────────────


def _feed(rows):
    data = bt.feeds.PandasData(dataname=data_range.to_dataframe(rows), datetime=None, open="open", high="high",
                               low="low", close="close", volume="volume", openinterest=-1)
    data._name = "T"
    return data


def test_first_bar_at_taipei_midnight_counts_as_in_range():
    """backtrader 的 bar 時間是不帶時區的 UTC（台北 00:00 → 前一天 16:00）；交易門檻用台北日期，範圍首根當天就算在範圍內。"""
    seen = []

    class Probe(BaseTWStrategy):
        def next(self):
            seen.append((bar_taipei_date(self.data).isoformat(), self.in_trade_range()))

    cerebro = bt.Cerebro(stdstats=False)
    cerebro.adddata(_feed(_rows(6)))
    cerebro.addstrategy(Probe, trade_start=D0 + timedelta(days=3))
    cerebro.run()
    assert seen == [(_day(i), i >= 3) for i in range(6)]


def _breakout_rows(n: int, seed: int) -> list[dict]:
    rng = np.random.RandomState(seed)
    closes = 100 * np.cumprod(1 + rng.normal(0.0005, 0.02, n))
    volumes = 1000 * (1 + rng.exponential(0.6, n))
    return _rows(n, prices=closes, volumes=volumes)


def _run_breakout(rows, start, end, warmup, monkeypatch):
    monkeypatch.setattr(data_range, "fetch_candles_range", FakeRangeFetch({"T": rows}))
    monkeypatch.setattr(engine, "BACKTEST_WARMUP_BARS", warmup)
    return engine.run_backtest("breakout_v1", ["T"], "1d", start, end)


def test_no_trade_before_trade_start(monkeypatch):
    """暖身段⛔ 不下單：先找出整段當範圍時最早的一筆進場，再把範圍起點放在它之後、讓它落進暖身段。"""
    rows = _breakout_rows(400, seed=7)
    full = _run_breakout(rows, _day(0), _day(399), 0, monkeypatch)
    entries = sorted(t["entry_time"] for t in full["trades"])
    assert entries, "這組種子要能產生交易"
    first_entry = datetime.fromisoformat(entries[0]).date()
    start = first_entry + timedelta(days=1)
    out = _run_breakout(rows, start.isoformat(), _day(399), 250, monkeypatch)
    assert all(datetime.fromisoformat(t["entry_time"]).date() >= start for t in out["trades"])
    assert out["result"]["data_coverage"][0]["first_bar"] == start.isoformat()


def test_signal_on_last_warmup_bar_does_not_fill_on_first_range_bar(monkeypatch):
    """暖身段最後一根的形態⛔ 不能在範圍首根成交（委託在下一根開盤成交）。"""
    rows = _breakout_rows(400, seed=7)
    full = _run_breakout(rows, _day(0), _day(399), 0, monkeypatch)
    entry = datetime.fromisoformat(sorted(t["entry_time"] for t in full["trades"])[0]).date()  # 成交在訊號的下一根
    out = _run_breakout(rows, entry.isoformat(), _day(399), 250, monkeypatch)  # 訊號那根是暖身段最後一根
    assert all(datetime.fromisoformat(t["entry_time"]).date() > entry for t in out["trades"])


def test_warmup_makes_first_bar_indicators_match_longer_history():
    """範圍首根的 RSI：暖身 250 根跟暖身 600 根的差在容許值內，而且⛔ 等於冷啟動（證明暖身有用到）。"""
    rows = _breakout_rows(900, seed=3)

    def rsi_at(first_row: int) -> float:
        got = []

        class Probe(BaseTWStrategy):
            def __init__(self):
                self.rsi = bt.indicators.RSI(self.data.close, period=14, safediv=True)

            def next(self):
                if bar_taipei_date(self.data) == D0 + timedelta(days=800):
                    got.append(float(self.rsi[0]))

        cerebro = bt.Cerebro(stdstats=False)
        cerebro.adddata(_feed(rows[first_row:]))
        cerebro.addstrategy(Probe)
        cerebro.run()
        return got[0]

    with_250, with_600, cold = rsi_at(800 - 250), rsi_at(800 - 600), rsi_at(800 - 15)
    assert abs(with_250 - with_600) < 1e-6
    assert abs(with_250 - cold) > 1e-3


def _lookbacks(obj, seen=None) -> list[int]:
    seen = seen if seen is not None else set()
    if id(obj) in seen or not hasattr(obj, "__dict__"):
        return []
    seen.add(id(obj))
    out = []
    for k, v in vars(obj).items():
        if isinstance(v, int) and not isinstance(v, bool) and any(s in k for s in ("lookback", "period", "window")):
            out.append(v)
        elif isinstance(v, (list, tuple)):
            for x in v:
                out += _lookbacks(x, seen)
        elif hasattr(v, "__dict__"):
            out += _lookbacks(v, seen)
    for prop in ("min_bars",):
        if hasattr(type(obj), prop):
            out.append(getattr(obj, prop))
    return out


def test_warmup_covers_every_strategy_lookback():
    p = dict(breakout_v1.BreakoutV1.params._getitems())
    longest = [breakout_v1.SR_LOOKBACK, *[v for k, v in p.items() if isinstance(v, int) and k != "trade_start"]]
    for name in STRATEGY_PRESETS:
        longest += _lookbacks(build_strategy(name))
    assert max(longest) <= BACKTEST_WARMUP_BARS == 250


# ── 範圍內的統計（backtrader） ─────────────────────────────────────────────


def _dated(actions: dict[str, tuple[str, int]]):
    """照日期下單（⛔ 不看指標）：暖身段不影響成交。actions：{台北日期: (buy|sell, 股數)}。"""

    class Dated(BaseTWStrategy):
        def next(self):
            a = actions.get(bar_taipei_date(self.data).isoformat())
            if a and self.in_trade_range():
                (self.buy if a[0] == "buy" else self.sell)(size=a[1])

    return Dated


def _cerebro(rows, strategy, trade_start=None, old=True):
    cerebro = bt.Cerebro(stdstats=False)
    cerebro.broker.setcash(1_000_000)
    cerebro.adddata(_feed(rows))
    cerebro.addstrategy(strategy, trade_start=trade_start)
    if old:
        cerebro.addanalyzer(bt.analyzers.SharpeRatio, _name="sharpe", timeframe=bt.TimeFrame.Days, annualize=True)
        cerebro.addanalyzer(bt.analyzers.DrawDown, _name="drawdown")
        cerebro.addanalyzer(bt.analyzers.Returns, _name="returns")
    cerebro.addanalyzer(RangeValue, _name="range_value", trade_start=trade_start)
    strat = cerebro.run()[0]
    rv = strat.analyzers.range_value.get_analysis()
    return strat, range_stats(rv["start_value"], rv["values"], cerebro.broker.getvalue(), rv["days"]), rv


def _old(strat):
    return (strat.analyzers.returns.get_analysis()["rnorm"],
            strat.analyzers.sharpe.get_analysis()["sharperatio"],
            strat.analyzers.drawdown.get_analysis()["max"]["drawdown"])


PRICES = [100, 101, 99, 104, 108, 103, 97, 95, 101, 106, 110, 107, 112, 109, 115, 118, 113, 120, 117, 121]
ACTIONS = {_day(2): ("buy", 500), _day(6): ("sell", 500), _day(9): ("buy", 300), _day(15): ("sell", 300)}


def test_range_stats_equal_backtrader_analyzers_on_the_same_run():
    strat, new, _ = _cerebro(_rows(20, prices=PRICES), _dated(ACTIONS))
    annual, sharpe, dd = _old(strat)
    assert new.annual_return == pytest.approx(annual, abs=1e-12)
    assert new.sharpe_ratio == pytest.approx(sharpe, abs=1e-12)
    assert new.max_drawdown_pct == pytest.approx(dd, abs=1e-12)


def test_warmup_does_not_change_range_stats():
    """同一組成交：(a) 只餵範圍段、原本的三個分析器；(b) 暖身＋範圍、新的分析器與 trade_start → 三個值相同。"""
    warm = _rows(30, prices=[90 + (i % 7) for i in range(30)])
    rng = _rows(20, start=D0 + timedelta(days=30), prices=PRICES)
    acts = {(date.fromisoformat(k) + timedelta(days=30)).isoformat(): v for k, v in ACTIONS.items()}
    a, _, _ = _cerebro(rng, _dated(acts))
    _, b, rv = _cerebro(warm + rng, _dated(acts), trade_start=D0 + timedelta(days=30), old=False)
    annual, sharpe, dd = _old(a)
    assert len(rv["values"]) == 20
    assert b.annual_return == pytest.approx(annual, abs=1e-12)
    assert b.sharpe_ratio == pytest.approx(sharpe, abs=1e-12)
    assert b.max_drawdown_pct == pytest.approx(dd, abs=1e-12)


def test_short_range_without_warmup_still_records_every_bar(monkeypatch):
    """暖身 0、範圍 30 根：前 19 根還沒達到 SMA(20) 的 minperiod，策略走的是 prenext（之後的 next 也因為不到 60 根而不判斷），
    分析器仍記到範圍內每一根，統計是 0。
    ⚠️ 資料比指標期數還短（例如 10 根）時，backtrader 預設的 runonce 模式會丟 IndexError——修改前就是這樣，見實作備註。"""
    seen = {}
    real = engine.range_stats

    def spy(start_value, values, final, days=None):
        seen["n"] = len(values)
        return real(start_value, values, final, days)

    monkeypatch.setattr(engine, "range_stats", spy)
    out = _run_breakout(_rows(30), _day(0), _day(29), 0, monkeypatch)
    assert seen["n"] == 30
    r = out["result"]
    assert (r["total_trades"], r["annual_return"], r["sharpe_ratio"], r["max_drawdown"]) == (0, 0.0, 0.0, 0.0)


# ── modular：交易起點與統計起點 ─────────────────────────────────────────────


def _capture_curve(monkeypatch):
    got = {}
    real = modular_backtester._build_report

    def spy(name, initial_cash, final_equity, equity_curve, trades):
        got["curve"] = list(equity_curve)
        got["trades"] = list(trades)
        return real(name, initial_cash, final_equity, equity_curve, trades)

    monkeypatch.setattr(modular_backtester, "_build_report", spy)
    return got


def test_modular_curve_starts_at_first_range_bar(monkeypatch):
    got = _capture_curve(monkeypatch)
    df = data_range.to_dataframe(_breakout_rows(300, seed=5))
    BacktestEngine().run("T", df, build_strategy("breakout_swing_atr_v1"), trade_start_index=250)
    assert got["curve"][0][0] == df.index[250] and len(got["curve"]) == 50
    assert all(t.entry_time >= df.index[250] for t in got["trades"])


def test_modular_short_warmup_records_initial_equity_until_min_bars(monkeypatch):
    got = _capture_curve(monkeypatch)
    strat = build_strategy("breakout_swing_atr_v1")
    df = data_range.to_dataframe(_breakout_rows(120, seed=5))
    BacktestEngine(initial_cash=1_000_000).run("T", df, strat, trade_start_index=5)  # 暖身只有 5 根
    curve = got["curve"]
    assert curve[0][0] == df.index[5] and len(curve) == 115
    assert all(v == 1_000_000 for _, v in curve[: strat.min_bars - 5])


def test_modular_never_reaches_min_bars_still_has_a_flat_curve(monkeypatch):
    got = _capture_curve(monkeypatch)
    strat = build_strategy("breakout_swing_atr_v1")
    df = data_range.to_dataframe(_rows(strat.min_bars - 3))
    report = BacktestEngine(initial_cash=1_000_000).run("T", df, strat, trade_start_index=0)
    assert len(got["curve"]) == len(df) and {v for _, v in got["curve"]} == {1_000_000}
    assert (report.total_trades, report.total_return, report.annual_return, report.sharpe_ratio, report.max_drawdown) == (0, 0.0, 0.0, 0.0, 0.0)


def test_modular_without_trade_start_index_keeps_old_behaviour():
    strat = build_strategy("breakout_swing_atr_v1")
    report = BacktestEngine().run("T", data_range.to_dataframe(_rows(strat.min_bars)), strat)
    assert report.total_trades == 0  # 資料不到 min_bars + 2 根 → 空報告（原本的行為）


# ── 截斷的回歸：真的 db.fetch_candles_range ＋ SQLite（issue 要求的驗收） ─────────


@pytest.fixture
def sqlite_candles(tmp_path, monkeypatch):
    path = tmp_path / "c.sqlite"
    sqlite3.connect(str(path)).close()
    eng = create_engine(f"sqlite:///{path}", connect_args={"check_same_thread": False})
    with eng.begin() as conn:
        conn.execute(text("""CREATE TABLE candles (id INTEGER PRIMARY KEY, symbol TEXT, timeframe TEXT, open REAL, high REAL,
                             low REAL, close REAL, volume REAL, amount REAL, adj_factor REAL, vol_factor REAL, ts DATETIME)"""))
    monkeypatch.setattr(db, "engine", eng)
    monkeypatch.setattr(db, "DB_DRIVER", "sqlite")

    def install(symbol: str, rows: list[dict]) -> None:
        with eng.begin() as conn:
            for r in rows:
                ts = datetime.fromtimestamp(r["timestamp"], timezone.utc).strftime("%Y-%m-%d %H:%M:%S")
                conn.execute(text("""INSERT INTO candles (symbol, timeframe, open, high, low, close, volume, amount,
                                     adj_factor, vol_factor, ts) VALUES (:s,'1d',:o,:h,:l,:c,:v,0,1,1,:ts)"""),
                             {"s": symbol, "o": r["open"], "h": r["high"], "l": r["low"], "c": r["close"], "v": r["volume"], "ts": ts})

    return install


@pytest.mark.parametrize("n,start,end,want_bars", [
    (1500, 100, 1300, 1201),  # 部分被截掉：起點早於最近 1000 根、終點在 1000 根之內
    (1400, 0, 599, 600),      # 範圍內 600 根、範圍之後還有 800 根：最近 1000 根只留得下最後 200 根
    (1500, 0, 300, 301),      # 整段在最近 1000 根之前
])
def test_backtrader_range_is_not_truncated(sqlite_candles, monkeypatch, n, start, end, want_bars):
    sqlite_candles("2330", _breakout_rows(n, seed=11))
    monkeypatch.setattr(engine, "BACKTEST_MAX_TASK_BARS", 10_000)
    out = engine.run_backtest("breakout_v1", ["2330"], "1d", _day(start), _day(end))
    cov = out["result"]["data_coverage"][0]
    assert (cov["first_bar"], cov["last_bar"], cov["bars"]) == (_day(start), _day(end), want_bars)
    assert cov["warmup_bars"] == min(start, 250)


@pytest.mark.parametrize("n,start,end,want_bars", [
    (2500, 100, 2300, 2201),  # modular 原本是最近 2000 根
    (2400, 0, 599, 600),
    (2500, 0, 300, 301),
])
def test_modular_range_is_not_truncated(sqlite_candles, monkeypatch, n, start, end, want_bars):
    sqlite_candles("2330", _breakout_rows(n, seed=11))
    monkeypatch.setattr(engine, "BACKTEST_MAX_TASK_BARS", 10_000)
    seen = {}

    def fake_run(self, symbol, df, strategy, chip_scores=None, trade_start_index=None):
        seen["len"], seen["r0"] = len(df), trade_start_index
        return modular_backtester._empty_report(strategy.name)

    monkeypatch.setattr(BacktestEngine, "run", fake_run)
    out = engine.run_backtest("breakout_swing_atr_v1", ["2330"], "1d", _day(start), _day(end))
    cov = out["result"]["data_coverage"][0]
    assert (cov["first_bar"], cov["last_bar"], cov["bars"]) == (_day(start), _day(end), want_bars)
    assert seen == {"len": want_bars + min(start, 250), "r0": min(start, 250)}


# ── 設定值 ───────────────────────────────────────────────────────────────


@pytest.mark.parametrize("raw,want", [(20, 20), ("2", 2), (" 7 ", 7)])
def test_positive_int_config(raw, want):
    import config
    assert config._positive_int("X", raw) == want


@pytest.mark.parametrize("raw", [0, -1, "0", "abc", "1.5", "", None, "1e3"])
def test_positive_int_config_rejects(raw):
    import config
    with pytest.raises(ValueError):
        config._positive_int("X", raw)


# ── v1.5：每一檔的上限、資料不足（insufficient_data） ───────────────────────


def test_single_symbol_over_its_own_limit_is_symbol_too_large():
    """單一檔的範圍段＋暖身段超過每檔上限 → symbol_too_large（要縮短範圍，減少檔數沒有用）。"""
    with pytest.raises(BacktestJobError) as e:
        data_range.load_task_candles(["A"], "1d", _day(20), _day(39), warmup_bars=5, max_task_bars=1000,
                                     max_symbol_bars=10, fetch=FakeRangeFetch({"A": _rows(40)}))
    assert e.value.code == "symbol_too_large:A:11:10"


def test_both_limits_exceeded_but_task_budget_hit_first_is_task_too_large():
    """兩種上限都會超過：B 本身 20 根（> 每檔上限 10），但任務只剩 7 根額度、先碰到總額度 → task_too_large
    （查詢在剩下的額度 + 1 根就停，⛔ 不知道 B 本身超過每檔上限）。"""
    data = {"A": _rows(8), "B": _rows(20)}
    with pytest.raises(BacktestJobError) as e:
        data_range.load_task_candles(["A", "B"], "1d", _day(0), _day(19), warmup_bars=5, max_task_bars=15,
                                     max_symbol_bars=10, fetch=FakeRangeFetch(data))
    assert e.value.code == "task_too_large:B:16:15"


def test_each_symbol_ok_but_total_over_is_task_too_large():
    """先碰到任務的總額度（這個案例裡每一檔本身也都合格）→ task_too_large。"""
    data = {"A": _rows(8), "B": _rows(8)}
    with pytest.raises(BacktestJobError) as e:
        data_range.load_task_candles(["A", "B"], "1d", _day(0), _day(7), warmup_bars=5, max_task_bars=12,
                                     max_symbol_bars=10, fetch=FakeRangeFetch(data))
    assert e.value.code == "task_too_large:B:13:12"  # A 用了 8 根，B 的額度剩 4 根、取到 5 根就停


def test_symbol_exactly_at_its_limit_passes():
    (loaded, cov) = data_range.load_task_candles(["A"], "1d", _day(5), _day(14), warmup_bars=5, max_task_bars=1000,
                                                 max_symbol_bars=15, fetch=FakeRangeFetch({"A": _rows(40)}))
    assert cov[0]["bars"] + cov[0]["warmup_bars"] == 15


def test_engine_applies_the_per_symbol_limit(monkeypatch):
    monkeypatch.setattr(engine, "BACKTEST_MAX_SYMBOL_BARS", 100)
    monkeypatch.setattr(data_range, "fetch_candles_range", FakeRangeFetch({"T": _rows(400)}))
    with pytest.raises(BacktestJobError) as e:
        engine.run_backtest("breakout_swing_atr_v1", ["T"], "1d", _day(0), _day(399))
    assert e.value.code == "symbol_too_large:T:101:100"


def test_required_bars_matches_backtrader_minperiod():
    """`BreakoutV1.required_bars()` 要等於 backtrader 實際的 minperiod（nextstart 那一根的 len）。"""
    seen = {}

    class Probe(breakout_v1.BreakoutV1):
        def nextstart(self):
            seen["minperiod"] = len(self)
            super().nextstart()

    cerebro = bt.Cerebro(stdstats=False)
    cerebro.adddata(_feed(_rows(60)))
    cerebro.addstrategy(Probe)
    cerebro.run()
    assert seen["minperiod"] == breakout_v1.BreakoutV1.required_bars() == 20


@pytest.mark.parametrize("n,code", [(19, "insufficient_data:T:19:20"), (20, None)])
def test_insufficient_data_boundary(monkeypatch, n, code):
    """minperiod − 1 根 → insufficient_data（⛔ 讓 backtrader 丟 IndexError、變成 internal_error）；剛好達標可以執行。"""
    if code:
        with pytest.raises(BacktestJobError) as e:
            _run_breakout(_rows(n), _day(0), _day(n - 1), 0, monkeypatch)
        assert e.value.code == code
    else:
        out = _run_breakout(_rows(n), _day(0), _day(n - 1), 0, monkeypatch)
        assert out["result"]["total_trades"] == 0


def test_short_range_with_enough_warmup_runs(monkeypatch):
    """範圍只有 3 根、但暖身足夠：照常執行，覆蓋範圍照實記。"""
    out = _run_breakout(_rows(300), _day(297), _day(299), 250, monkeypatch)
    cov = out["result"]["data_coverage"][0]
    assert (cov["bars"], cov["warmup_bars"]) == (3, 250)


def test_warmup_short_but_enough_to_initialise_runs_and_first_range_bar_in_prenext_is_recorded(monkeypatch):
    """暖身只有 5 根（沒滿 250、但總資料 35 根夠初始化）：照常執行、標暖身不足。範圍首根（第 6 根）還在 prenext，
    分析器仍從範圍首根起記到每一根。"""
    seen = {}
    real = engine.range_stats

    def spy(start_value, values, final, days=None):
        seen["n"] = len(values)
        return real(start_value, values, final, days)

    monkeypatch.setattr(engine, "range_stats", spy)
    out = _run_breakout(_rows(35), _day(5), _day(34), 250, monkeypatch)
    cov = out["result"]["data_coverage"][0]
    assert (cov["first_bar"], cov["bars"], cov["warmup_bars"], cov["warmup_required"]) == (_day(5), 30, 5, 250)
    assert seen["n"] == 30


# ── 分 K：年化與夏普以日為期、最大回撤逐根（review 2026-11-19） ─────────────────


def _intraday_rows(days: int, per_day: int, start: date, prices) -> list[dict]:
    """每天 per_day 根、台北 09:00 起每小時一根。"""
    rows, k = [], 0
    for d in range(days):
        for h in range(per_day):
            p = float(prices[k % len(prices)])
            ts = datetime.combine(start + timedelta(days=d), datetime.min.time(), tzinfo=TAIPEI) + timedelta(hours=9 + h)
            rows.append({"timestamp": int(ts.timestamp()), "open": p, "high": p * 1.01, "low": p * 0.99, "close": p, "volume": 1000.0})
            k += 1
    return rows


def _by_bar(actions: dict[str, tuple[str, int]]):
    """照 bar 的台北時刻下單（⛔ 不看指標）。"""

    class ByBar(BaseTWStrategy):
        def next(self):
            key = bt.num2date(self.data.datetime[0]).replace(tzinfo=timezone.utc).astimezone(TAIPEI).strftime("%Y-%m-%d %H")
            a = actions.get(key)
            if a and self.in_trade_range():
                (self.buy if a[0] == "buy" else self.sell)(size=a[1])

    return ByBar


INTRA_PRICES = [100, 101, 99, 104, 108, 103, 97, 95, 101, 106, 110, 107, 112, 109, 115, 118, 113, 120, 117, 121, 116, 111]
INTRA_ACTIONS = {"2020-02-03 10": ("buy", 500), "2020-02-04 12": ("sell", 500), "2020-02-05 09": ("buy", 300), "2020-02-06 11": ("sell", 300)}


def test_intraday_stats_equal_backtrader_analyzers_on_the_same_run():
    """一天 4 根的分 K、同一次執行：年化與夏普（以日為期）、最大回撤（逐根）都等於原本的三個分析器。"""
    rows = _intraday_rows(6, 4, date(2020, 2, 2), INTRA_PRICES)
    strat, new, rv = _cerebro(rows, _by_bar(INTRA_ACTIONS))
    annual, sharpe, dd = _old(strat)
    assert len(rv["values"]) == 24 and len(set(rv["days"])) == 6
    assert new.annual_return == pytest.approx(annual, abs=1e-12)
    assert new.sharpe_ratio == pytest.approx(sharpe, abs=1e-12)
    assert new.max_drawdown_pct == pytest.approx(dd, abs=1e-12)


def test_intraday_warmup_does_not_change_range_stats():
    """分 K 有暖身段（前幾天）：跟「只餵範圍段、原本的分析器」相同。"""
    warm = _intraday_rows(3, 4, date(2020, 1, 30), [90, 91, 92, 93, 94])
    rng = _intraday_rows(6, 4, date(2020, 2, 2), INTRA_PRICES)
    a, _, _ = _cerebro(rng, _by_bar(INTRA_ACTIONS))
    _, b, rv = _cerebro(warm + rng, _by_bar(INTRA_ACTIONS), trade_start=date(2020, 2, 2), old=False)
    annual, sharpe, dd = _old(a)
    assert len(rv["values"]) == 24
    assert b.annual_return == pytest.approx(annual, abs=1e-12)
    assert b.sharpe_ratio == pytest.approx(sharpe, abs=1e-12)
    assert b.max_drawdown_pct == pytest.approx(dd, abs=1e-12)


def test_daily_grouping_is_what_makes_intraday_match():
    """把每一根當成一天（⛔ 舊版不是這樣算）會跟原本的分析器不同——證明上面的對照真的有驗到以日為期。"""
    rows = _intraday_rows(6, 4, date(2020, 2, 2), INTRA_PRICES)
    strat, _, rv = _cerebro(rows, _by_bar(INTRA_ACTIONS))
    per_bar = range_stats(rv["start_value"], rv["values"], strat.broker.getvalue(), None)
    annual, sharpe, _ = _old(strat)
    assert per_bar.annual_return != pytest.approx(annual, abs=1e-9)
    assert per_bar.sharpe_ratio != pytest.approx(sharpe, abs=1e-9)


REQ_CASES = Path(__file__).resolve().parent / "data" / "backtest_request_validation_cases.json"


@pytest.mark.parametrize("case", json.loads(REQ_CASES.read_text(encoding="utf-8")), ids=lambda c: c["name"])
def test_request_validation_cases_shared_with_go(case):
    """跟 Go 的 internal/backtest/testdata/request_validation_cases.json 內容相同（check-backtest-contract.py 比對）。"""
    try:
        _validate(case["strategy"], case["symbols"], case["start_date"], case["end_date"], case["max_symbols"])
        got = ""
    except BacktestJobError as e:
        got = e.code
    assert got == case["want"]
