"""backtrader 路徑的交易明細（I-135）：從成交紀錄算進出場價、股數、費用與時間。

用合成的日 K 與照腳本下單的測試策略，走 `run_backtest` 同一條路徑（`fetch_candles` 換成假的、`STRATEGY_MAP`
加上測試策略、費用換成測試自己的固定比例 1%）。費用用獨立的固定比例，跟 `TWCommission` 分開驗：
`TWCommission` 本身的扣費由 test_tw_commission.py 用真的類別驗（I-136）。
委託在第 n 根 K 棒送出、在第 n+1 根的開盤價成交。規格見 docs/database-schema.md 的 backtest_trades。
"""
from __future__ import annotations

import sys
from datetime import datetime, timedelta, timezone
from pathlib import Path
from types import SimpleNamespace

# engine 以 `from strategy.base import …` 載入策略（跟 http_server／worker 一樣要把 backtest/ 放進 sys.path）
sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "backtest"))

import backtrader as bt
import numpy as np
import pytest

from backtest import engine

RATE = 0.01  # 測試用的費率：每次成交 1%
T0 = datetime(2025, 1, 1, tzinfo=timezone(timedelta(hours=8)))  # 第 1 根 K 棒：台北 2025-01-01 00:00


class FixedRate(bt.CommInfoBase):
    params = (("commission", RATE), ("stocklike", True), ("commtype", bt.CommInfoBase.COMM_PERC), ("percabs", True))


def _rows(opens: list[float], volumes: list[float] | None = None, closes: list[float] | None = None) -> list[dict]:
    rows = []
    for i, o in enumerate(opens):
        c = closes[i] if closes else o
        rows.append({
            "timestamp": int((T0 + timedelta(days=i)).timestamp()),
            "open": o, "high": max(o, c) * 1.01, "low": min(o, c) * 0.99, "close": c,
            "volume": volumes[i] if volumes else 1000.0,
        })
    return rows


def _scripted(actions: list[tuple[int, str, str, int]]):
    """actions：(第幾根送出, 標的, buy|sell|close, 股數)。"""

    class Scripted(bt.Strategy):
        def next(self):
            for n, sym, kind, size in actions:
                if len(self) != n:
                    continue
                data = self.getdatabyname(sym)
                if kind == "buy":
                    self.buy(data=data, size=size)
                elif kind == "sell":
                    self.sell(data=data, size=size)
                else:
                    self.close(data=data)

    return Scripted


@pytest.fixture
def run(monkeypatch):
    """`engine` 用 `TWCommission(commission=…, tax=…)` 建費用模型（I-136）。這裡換成一個**工廠**：接受同樣的關鍵字參數、
    記下收到的值，但回傳⛔ 不帶參數的 `FixedRate()`——傳進去的費率會蓋掉測試的 1%，`tax` 也⛔ 不是 `FixedRate` 的參數
    （會丟 TypeError）。跑完確認 `engine` 真的傳了這兩個參數：介面改了卻沒跟上時會失敗，⛔ 不靜默。"""
    def _run(data: dict[str, list[dict]], actions=None, strategy: str = "scripted", commission=FixedRate):
        received: list[dict] = []
        commission_cls = commission  # 工廠的參數也叫 commission，先換個名字

        def factory(*, commission: float, tax: float):
            received.append({"commission": commission, "tax": tax})
            return commission_cls()

        monkeypatch.setattr(engine, "fetch_candles", lambda symbol, tf, limit: data[symbol])
        monkeypatch.setattr(engine, "TWCommission", factory)
        if actions is not None:
            monkeypatch.setitem(engine.STRATEGY_MAP, "scripted", _scripted(actions))
        trades = engine.run_backtest(strategy, list(data), "1d", "2025-01-01", "2025-12-31")["trades"]
        assert received == [{"commission": engine.COMMISSION_RATE, "tax": engine.TAX_RATE}]
        return trades

    return _run


def _taipei(day_index: int) -> str:
    return (T0 + timedelta(days=day_index)).isoformat()


def _within_bound(row: dict) -> bool:
    """存下來（四捨五入過）的值的精度界線：|d×(exit−entry)×size − (pnl+commission)| ≤ 0.01×size + 0.01。"""
    d = 1 if row["direction"] == "BUY" else -1
    lhs = d * (row["exit_price"] - row["entry_price"]) * row["size"] - (row["pnl"] + row["commission"])
    return abs(lhs) <= 0.005 * 2 * row["size"] + 0.01 + 1e-9


def test_single_round_trip_every_field(run):
    # 第 1 根送出買 10 → 第 2 根開盤 100 成交；第 3 根送出平倉 → 第 4 根開盤 110 成交
    trades = run({"T": _rows([100, 100, 105, 110, 110, 110])}, [(1, "T", "buy", 10), (3, "T", "close", 0)])

    assert len(trades) == 1
    t = trades[0]
    fee = 10 * 100 * RATE + 10 * 110 * RATE
    assert t == {
        "symbol": "T", "direction": "BUY",
        "entry_time": _taipei(1), "exit_time": _taipei(3),
        "entry_price": 100.0, "exit_price": 110.0, "size": 10.0,
        "pnl": round(100 - fee, 2),
        "pnl_pct": round((100 - fee) / 1000, 6),
        "commission": round(fee, 2),
    }


def test_split_exit_weighted_price_and_precision_bound(run):
    """review 的例子：買 3@102，賣 1@104、賣 2@105。毛損益 8；出場均價存成 104.67，反算是 8.01——落在界線內。"""
    trades = run({"T": _rows([100, 102, 104, 105, 105, 105])},
                 [(1, "T", "buy", 3), (2, "T", "sell", 1), (3, "T", "sell", 2)])

    (t,) = trades
    fee = (3 * 102 + 1 * 104 + 2 * 105) * RATE
    assert (t["entry_price"], t["exit_price"], t["size"]) == (102.0, 104.67, 3.0)
    assert t["exit_time"] == _taipei(3)  # 最後一次出場
    assert t["pnl"] == round(8 - fee, 2)
    assert t["commission"] == round(fee, 2)
    assert round((t["exit_price"] - t["entry_price"]) * t["size"], 2) == 8.01  # 四捨五入後⛔ 不能精確反算
    assert _within_bound(t)


def test_interleaved_add_and_reduce_uses_all_entry_fills(run):
    """review 的例子：買 10@100、賣 5@110、買 5@120、賣 10@130。t.price 最後是 110，但全部進場均價是 106.67、毛損益 250。"""
    trades = run({"T": _rows([100, 100, 110, 120, 130, 130])},
                 [(1, "T", "buy", 10), (2, "T", "sell", 5), (3, "T", "buy", 5), (4, "T", "sell", 10)])

    (t,) = trades
    fee = (10 * 100 + 5 * 110 + 5 * 120 + 10 * 130) * RATE
    assert (t["entry_price"], t["exit_price"], t["size"]) == (106.67, 123.33, 15.0)
    assert t["pnl"] == round(250 - fee, 2)  # 用 t.price 當進場價會得到 200
    assert t["pnl_pct"] == round((250 - fee) / 1600, 6)
    assert _within_bound(t)


def test_short_round_trip(run):
    """空方（必測）：放空 10@100、回補 10@90，毛利 100；界線要帶方向（⛔ 不帶的話差 200）。"""
    trades = run({"T": _rows([100, 100, 90, 90, 90])}, [(1, "T", "sell", 10), (2, "T", "close", 0)])

    (t,) = trades
    fee = (10 * 100 + 10 * 90) * RATE
    assert (t["direction"], t["entry_price"], t["exit_price"], t["size"]) == ("SELL", 100.0, 90.0, 10.0)
    assert t["pnl"] == round(100 - fee, 2)
    assert _within_bound(t)
    unsigned = (t["exit_price"] - t["entry_price"]) * t["size"] - (t["pnl"] + t["commission"])
    assert abs(unsigned) > 0.11  # ⛔ 不帶方向的公式會超出界線


def test_short_split_cover(run):
    trades = run({"T": _rows([100, 100, 95, 90, 90, 90])},
                 [(1, "T", "sell", 10), (2, "T", "buy", 4), (3, "T", "buy", 6)])

    (t,) = trades
    fee = (10 * 100 + 4 * 95 + 6 * 90) * RATE
    assert (t["direction"], t["entry_price"], t["exit_price"], t["size"]) == ("SELL", 100.0, 92.0, 10.0)
    assert t["pnl"] == round(80 - fee, 2)
    assert _within_bound(t)


def test_reversal_splits_into_two_trades_with_allocated_commission(run):
    """反手（必測）：多單 10 股時賣 20 股 → 平多 10、開空 10，費用 22 分攤成 11／11；之後回補 10 股。"""
    trades = run({"T": _rows([100, 100, 110, 90, 90, 90])},
                 [(1, "T", "buy", 10), (2, "T", "sell", 20), (3, "T", "buy", 10)])

    long_t, short_t = trades
    assert (long_t["direction"], long_t["entry_price"], long_t["exit_price"], long_t["size"]) == ("BUY", 100.0, 110.0, 10.0)
    assert (short_t["direction"], short_t["entry_price"], short_t["exit_price"], short_t["size"]) == ("SELL", 110.0, 90.0, 10.0)
    assert long_t["commission"] == 21.0   # 10（買）＋ 11（反手那張的一半）
    assert short_t["commission"] == 20.0  # 11（反手那張的另一半）＋ 9（回補）
    assert long_t["pnl"] == round(100 - 21, 2)
    assert short_t["pnl"] == round(200 - 20, 2)
    assert long_t["exit_time"] == short_t["entry_time"] == _taipei(2)


def test_multiple_trades_and_symbols(run):
    trades = run(
        {"A": _rows([100] * 8), "B": _rows([50] * 8)},
        [(1, "A", "buy", 1), (2, "A", "close", 0), (1, "B", "buy", 2), (2, "B", "close", 0),
         (4, "B", "buy", 3), (5, "B", "close", 0)],
    )

    assert sorted((t["symbol"], t["size"]) for t in trades) == [("A", 1.0), ("B", 2.0), ("B", 3.0)]


def test_no_trades_and_open_position_not_listed(run):
    assert run({"T": _rows([100] * 5)}, []) == []
    assert run({"T": _rows([100] * 5)}, [(1, "T", "buy", 5)]) == []  # 結束時還沒平倉


def test_times_are_taipei_with_offset_and_same_instant():
    """UTC 時刻轉成台北時間：日 K 2025-01-03 那根是 2025-01-03T00:00:00+08:00（⛔ 不是直接貼 +08:00 的 01-02T16:00）。"""
    utc_naive_of_bar = datetime(2025, 1, 2, 16, 0)  # backtrader 存的是不帶時區的 UTC
    out = engine._bt_time_to_taipei_iso(bt.date2num(utc_naive_of_bar))

    assert out == "2025-01-03T00:00:00+08:00"
    assert datetime.fromisoformat(out) == datetime(2025, 1, 2, 16, 0, tzinfo=timezone.utc)


# ── 成交紀錄不符合預期就 raise ─────────────────────────────────────────


def _fake_trade(events, pnl, commission, pnlcomm=None):
    hist = [SimpleNamespace(event=SimpleNamespace(size=s, price=p, commission=c), status=SimpleNamespace(dt=bt.date2num(datetime(2025, 1, 2 + i))))
            for i, (s, p, c) in enumerate(events)]
    return SimpleNamespace(ref=1, history=hist, pnl=pnl, commission=commission,
                           pnlcomm=pnl - commission if pnlcomm is None else pnlcomm,
                           data=SimpleNamespace(_name="T"))


@pytest.mark.parametrize(
    "events,pnl,commission,match",
    [
        ([], 0.0, 0.0, "no history"),
        ([(10, 100, 1), (-20, 110, 2), (10, 90, 1)], 0.0, 4.0, "changed sign"),
        ([(10, 100, 1), (-5, 110, 1)], 50.0, 2.0, "does not return to 0"),
        ([(10, 100, 1), (-10, 110, 1)], 99.0, 2.0, "gross pnl"),
        ([(10, 100, 1), (-10, 110, 1)], 100.0, 5.0, "commission"),
    ],
)
def test_inconsistent_history_raises(events, pnl, commission, match):
    with pytest.raises(engine.TradeExtractionError, match=match):
        engine._trade_row(_fake_trade(events, pnl, commission))


_NONFINITE = [float("nan"), float("inf"), float("-inf")]
_GOOD = dict(events=[(10, 100.0, 1.0), (-10, 110.0, 1.0)], pnl=100.0, commission=2.0)


def _with(field: str, v: float) -> SimpleNamespace:
    events = [list(e) for e in _GOOD["events"]]
    pnl, commission, pnlcomm = _GOOD["pnl"], _GOOD["commission"], None
    if field.startswith("event."):
        events[1][("event.size", "event.price", "event.commission").index(field)] = v
    elif field == "pnl":
        pnl = v
    elif field == "pnlcomm":
        pnlcomm = v
    else:
        commission = v
    return _fake_trade([tuple(e) for e in events], pnl, commission, pnlcomm)


@pytest.mark.parametrize("v", _NONFINITE, ids=["nan", "inf", "-inf"])
@pytest.mark.parametrize("field", ["event.size", "event.price", "event.commission", "pnl", "pnlcomm", "commission"])
def test_non_finite_values_raise_before_any_arithmetic(field, v):
    """NaN 的大小比較一律不成立，⛔ 不能靠後面的核對擋：任何一個必要數值不是有限數就報錯。"""
    with pytest.raises(engine.TradeExtractionError, match="not finite"):
        engine._trade_row(_with(field, v))


def test_non_finite_time_raises():
    t = _fake_trade(_GOOD["events"], _GOOD["pnl"], _GOOD["commission"])
    t.history[1].status.dt = float("nan")
    with pytest.raises(engine.TradeExtractionError, match="not finite"):
        engine._trade_row(t)


# 絕對容許值 1e-6 的邊界：差 5e-7 通過、差 2e-6 報錯（毛損益與費用各測一次）
@pytest.mark.parametrize("delta,ok", [(5e-7, True), (-5e-7, True), (2e-6, False), (-2e-6, False)])
def test_gross_pnl_tolerance_is_absolute_1e6(delta, ok):
    t = _fake_trade(_GOOD["events"], _GOOD["pnl"] + delta, _GOOD["commission"])
    if ok:
        assert engine._trade_row(t)["size"] == 10
    else:
        with pytest.raises(engine.TradeExtractionError, match="gross pnl"):
            engine._trade_row(t)


@pytest.mark.parametrize("delta,ok", [(5e-7, True), (-5e-7, True), (2e-6, False), (-2e-6, False)])
def test_commission_tolerance_is_absolute_1e6(delta, ok):
    t = _fake_trade(_GOOD["events"], _GOOD["pnl"], _GOOD["commission"] + delta)
    if ok:
        assert engine._trade_row(t)["size"] == 10
    else:
        with pytest.raises(engine.TradeExtractionError, match="commission"):
            engine._trade_row(t)


def test_large_base_half_unit_gap_raises():
    """大額基數（毛損益 1e6、費用 1e6）下差 0.5 也要報錯：容許值若乘上金額（相對誤差）會放行。"""
    events = [(1_000_000, 1000.0, 500_000.0), (-1_000_000, 1001.0, 500_000.0)]
    with pytest.raises(engine.TradeExtractionError, match="gross pnl"):
        engine._trade_row(_fake_trade(events, 1_000_000.5, 1_000_000.0))
    with pytest.raises(engine.TradeExtractionError, match="commission"):
        engine._trade_row(_fake_trade(events, 1_000_000.0, 1_000_000.5))


def test_large_multi_fill_trades_pass_absolute_tolerance(run, monkeypatch):
    """支持採用的容許值：大額（單筆成交金額約 1e8～4e8）、多筆加減碼交錯的多方與空方交易，走真的 backtrader，
    浮點誤差都在 1e-6 以內（⛔ 不是對任意金額的保證）。"""
    monkeypatch.setattr(engine, "INITIAL_CASH", 1e11)
    opens = [1234.57, 1234.57, 1301.13, 1187.91, 1350.39, 1288.07, 1199.99, 1273.41, 1311.17, 1222.23, 1222.23]
    trades = run({"L": _rows(opens), "S": _rows([o * 0.731 for o in opens])},
                 [(1, "L", "buy", 300_000), (2, "L", "sell", 170_000), (3, "L", "buy", 230_000),
                  (4, "L", "sell", 90_000), (5, "L", "buy", 41_333), (6, "L", "sell", 311_333),
                  (1, "S", "sell", 250_000), (3, "S", "buy", 120_001), (4, "S", "sell", 77_777),
                  (7, "S", "buy", 207_776)])
    assert sorted((t["symbol"], t["direction"]) for t in trades) == [("L", "BUY"), ("S", "SELL")]
    for t in trades:
        assert _within_bound(t)


# ── breakout_v1 本身 ───────────────────────────────────────────────


def _breakout_series(seed: int, n: int = 260) -> list[dict]:
    """固定種子的隨機漫步（套件版本已釘住，見 development-workflow.md「python 的套件版本與升級」）。"""
    rng = np.random.RandomState(seed)
    closes = 100 * np.cumprod(1 + rng.normal(0.0005, 0.02, n))
    volumes = 1000 * (1 + rng.exponential(0.6, n))
    return _rows([float(c) for c in closes], [float(v) for v in volumes], [float(c) for c in closes])


BREAKOUT_SEED = 16  # 這組資料 breakout_v1 會產生 3 筆交易（2026-11-18 查）


def test_breakout_v1_with_trades_completes(run):
    """I-135 的原始症狀：breakout_v1 只要有交易就丟 `'int' object has no attribute 'isclosed'`。"""
    trades = run({"T": _breakout_series(BREAKOUT_SEED)}, strategy="breakout_v1")

    assert len(trades) >= 1
    for t in trades:
        assert t["size"] > 0 and t["entry_price"] > 0 and t["exit_price"] > 0
        assert _within_bound(t)
