"""backtrader 路徑的費用模型 `TWCommission`（I-136）：broker **實際扣的**費用。

一律用**真的 `TWCommission`** 跑 Cerebro（⛔ 不只看某個函式的回傳值）。每個案例核對：每次成交的費用、每筆交易的
`t.commission`（＝Σ `event.commission`）、淨損益（`t.pnlcomm`）、broker 的現金與資產變化。
broker 資產變化＝Σ 淨損益的等式只在**期末全部平倉**時成立（否則資產裡有未實現損益），用它的案例都先斷言部位是 0。
委託在第 n 根 K 棒送出、在第 n+1 根的開盤價成交。規格見 docs/database-schema.md 的 backtest_trades。
"""
from __future__ import annotations

import sys
from datetime import datetime, timedelta, timezone
from pathlib import Path

# engine 以 `from strategy.base import …` 載入策略（跟 http_server／worker 一樣要把 backtest/ 放進 sys.path）
sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "backtest"))

import backtrader as bt
import pandas as pd
import pytest

from backtest import engine
from backtest.modular import service as modular_service
from backtest.modular.backtester import BacktestEngine
from backtest.modular.types import Direction, ExitReason, Position
from strategy.base import TWCommission

C, T = 0.001425, 0.003  # 測試用的費率（跟 config 的值相同，但測試自己傳入）
CASH = 1_000_000.0
TOL = 1e-6
T0 = datetime(2025, 1, 1, tzinfo=timezone(timedelta(hours=8)))


def _df(opens: list[float]) -> pd.DataFrame:
    return pd.DataFrame({"open": opens, "high": opens, "low": opens, "close": opens, "volume": [1e9] * len(opens)},
                        index=pd.date_range("2025-01-01", periods=len(opens)))


def _cerebro(actions, opens, comminfo, filler=None) -> dict:
    """actions：(第幾根送出, buy|sell, 股數)。回傳每張委託的通知、已結束的交易與 broker 的期末狀態。"""
    orders: dict[int, dict] = {}
    trades: list = []

    class Scripted(bt.Strategy):
        def next(self):
            for n, kind, size in actions:
                if len(self) == n:
                    (self.buy if kind == "buy" else self.sell)(size=size)

        def notify_order(self, o):
            if o.status in (o.Partial, o.Completed):
                rec = orders.setdefault(o.ref, {"buy": o.isbuy(), "cum_comm": []})
                rec["cum_comm"].append(o.executed.comm)
                rec["exbits"] = [(b.size, b.comm) for b in o.executed.exbits]
                rec["done"] = o.status == o.Completed

        def notify_trade(self, t):
            if t.isclosed:
                trades.append(t)

    c = bt.Cerebro(stdstats=False, tradehistory=True)
    c.broker.setcash(CASH)
    c.broker.addcommissioninfo(comminfo)
    if filler is not None:
        c.broker.set_filler(filler)
    c.adddata(bt.feeds.PandasData(dataname=_df(opens)))
    c.addstrategy(Scripted)
    strat = c.run()[0]
    return {
        "orders": [orders[k] for k in sorted(orders)],
        "trades": trades,
        "value": c.broker.getvalue(),
        "cash": c.broker.getcash(),
        "position": strat.getposition(strat.datas[0]).size,
    }


def _assert_flat_and_balanced(out: dict) -> None:
    """期末全部平倉時：broker 資產變化（未四捨五入）＝Σ 淨損益，現金＝資產。"""
    assert out["position"] == 0
    assert abs((out["value"] - CASH) - sum(t.pnlcomm for t in out["trades"])) <= TOL
    assert abs(out["cash"] - out["value"]) <= TOL


def _assert_trade_fees(t, want_event_fees: list[float]) -> None:
    fees = [h.event.commission for h in t.history]
    assert fees == pytest.approx(want_event_fees, abs=TOL)
    assert t.commission == pytest.approx(sum(want_event_fees), abs=TOL)
    assert t.pnlcomm == pytest.approx(t.pnl - sum(want_event_fees), abs=TOL)


def _order_fees(out: dict) -> list[float]:
    """每張委託都只成交一次時，`order.executed.comm` 就是那一次的費用。"""
    for o in out["orders"]:
        assert o["done"] and len(o["exbits"]) == 1
    return [o["cum_comm"][-1] for o in out["orders"]]


def _tw() -> TWCommission:
    return TWCommission(commission=C, tax=T)


# ── 直接跑 Cerebro ────────────────────────────────────────────────


def test_buy_and_sell_10_at_100():
    """買 10 股 @100 扣 1.425；賣 10 股 @100 扣 4.425（手續費 1.425＋稅 3）；合計 5.85。"""
    out = _cerebro([(1, "buy", 10), (2, "sell", 10)], [100.0] * 4, _tw())
    assert _order_fees(out) == pytest.approx([1.425, 4.425], abs=TOL)
    (t,) = out["trades"]
    _assert_trade_fees(t, [1.425, 4.425])
    assert t.pnl == pytest.approx(0.0, abs=TOL) and t.pnlcomm == pytest.approx(-5.85, abs=TOL)
    _assert_flat_and_balanced(out)
    assert out["value"] == pytest.approx(CASH - 5.85, abs=TOL)


def test_short_taxes_the_opening_sell_not_the_cover():
    out = _cerebro([(1, "sell", 10), (2, "buy", 10)], [100.0, 100.0, 90.0, 90.0], _tw())
    assert _order_fees(out) == pytest.approx([4.425, 10 * 90 * C], abs=TOL)
    (t,) = out["trades"]
    assert not t.long
    _assert_trade_fees(t, [4.425, 10 * 90 * C])
    assert t.pnl == pytest.approx(100.0, abs=TOL)
    _assert_flat_and_balanced(out)


def test_split_across_multiple_orders():
    """分批（多張委託）：買 6@100、買 4@110、賣 5@120、賣 5@130；每張委託各自一次成交，賣出課稅、買進⛔ 不課稅。"""
    out = _cerebro([(1, "buy", 6), (2, "buy", 4), (3, "sell", 5), (4, "sell", 5)],
                   [100.0, 100.0, 110.0, 120.0, 130.0, 130.0], _tw())
    want = [6 * 100 * C, 4 * 110 * C, 5 * 120 * (C + T), 5 * 130 * (C + T)]
    assert _order_fees(out) == pytest.approx(want, abs=TOL)
    (t,) = out["trades"]
    _assert_trade_fees(t, want)
    _assert_flat_and_balanced(out)


def test_one_order_filled_in_several_executions():
    """同一張委託多次成交（`FixedSize(4)`：4＋4＋2）。單次的費用看 execution bit（或累計差額）；
    `order.executed.comm` 是累計值，⛔ 不是單次成交的費用。"""
    out = _cerebro([(1, "buy", 10), (5, "sell", 10)], [100.0] * 10, _tw(), filler=bt.broker.fillers.FixedSize(size=4))
    buy, sell = out["orders"]
    want_buy = [4 * 100 * C, 4 * 100 * C, 2 * 100 * C]
    want_sell = [4 * 100 * (C + T), 4 * 100 * (C + T), 2 * 100 * (C + T)]
    for o, sizes, want in ((buy, [4, 4, 2], want_buy), (sell, [-4, -4, -2], want_sell)):
        assert o["done"]
        assert [s for s, _ in o["exbits"]] == sizes
        assert [c for _, c in o["exbits"]] == pytest.approx(want, abs=TOL)
        # 累計差額＝單次費用；完成時的累計值＝各次合計
        diffs = [b - a for a, b in zip([0.0] + o["cum_comm"][:-1], o["cum_comm"])]
        assert diffs == pytest.approx(want, abs=TOL)
        assert o["cum_comm"][-1] == pytest.approx(sum(want), abs=TOL)
    (t,) = out["trades"]
    assert [h.event.size for h in t.history] == [4, 4, 2, -4, -4, -2]
    _assert_trade_fees(t, want_buy + want_sell)
    assert t.commission == pytest.approx(5.85, abs=TOL)
    _assert_flat_and_balanced(out)


def test_reversal_splits_the_order_fee_into_two_trades():
    """反手：買 10、賣 20、買回 10（都 @100）。賣 20 那張委託扣 8.85，拆成兩筆交易、各自 5.85。"""
    out = _cerebro([(1, "buy", 10), (2, "sell", 20), (3, "buy", 10)], [100.0] * 5, _tw())
    assert _order_fees(out) == pytest.approx([1.425, 8.85, 1.425], abs=TOL)
    long_t, short_t = out["trades"]
    assert long_t.long and not short_t.long
    _assert_trade_fees(long_t, [1.425, 4.425])
    _assert_trade_fees(short_t, [4.425, 1.425])
    _assert_flat_and_balanced(out)


def test_custom_rates_are_used():
    out = _cerebro([(1, "buy", 10), (2, "sell", 10)], [100.0] * 4, TWCommission(commission=0.002, tax=0.004))
    assert _order_fees(out) == pytest.approx([2.0, 6.0], abs=TOL)
    _assert_flat_and_balanced(out)


def test_zero_rates_charge_nothing():
    out = _cerebro([(1, "buy", 10), (2, "sell", 10)], [100.0, 100.0, 120.0, 120.0], TWCommission(commission=0.0, tax=0.0))
    assert _order_fees(out) == [0.0, 0.0]
    (t,) = out["trades"]
    assert t.commission == 0.0 and t.pnlcomm == pytest.approx(t.pnl, abs=TOL) == pytest.approx(200.0, abs=TOL)
    _assert_flat_and_balanced(out)


# ── 走 run_backtest（engine 傳入 config 的費率） ──────────────────────


def _rows(opens: list[float]) -> list[dict]:
    return [{"timestamp": int((T0 + timedelta(days=i)).timestamp()),
             "open": o, "high": o * 1.01, "low": o * 0.99, "close": o, "volume": 1e9} for i, o in enumerate(opens)]


def _scripted(actions):
    class Scripted(bt.Strategy):
        def next(self):
            for n, kind, size in actions:
                if len(self) == n:
                    (self.buy if kind == "buy" else self.sell)(size=size)

    return Scripted


@pytest.fixture
def run_bt(monkeypatch):
    """`run_backtest` 的 backtrader 路徑；費用模型⛔ 不換，用 engine 自己建的真 `TWCommission`。可以換掉 engine 的費率。"""
    def _run(opens, actions, *, commission_rate=None, tax_rate=None):
        monkeypatch.setattr(engine, "fetch_candles", lambda symbol, tf, limit: _rows(opens))
        monkeypatch.setitem(engine.STRATEGY_MAP, "scripted", _scripted(actions))
        if commission_rate is not None:
            monkeypatch.setattr(engine, "COMMISSION_RATE", commission_rate)
        if tax_rate is not None:
            monkeypatch.setattr(engine, "TAX_RATE", tax_rate)
        return engine.run_backtest("scripted", ["T"], "1d", "2025-01-01", "2025-12-31")

    return _run


def _check_result(out: dict, net_pnls: list[float]) -> None:
    """總表依存的位數驗算：`total_return` 存六位、`avg_pnl` 存四位，⛔ 不乘回本金要求等於淨損益。
    界線是半個最小位數；剛好落在半位時（例如 79.75375 存成 79.7537）浮點雜訊會略超過，另加 1e-9。"""
    r = out["result"]
    assert r["total_trades"] == len(net_pnls)
    assert abs(r["total_return"] - sum(net_pnls) / engine.INITIAL_CASH) <= 5e-7 + 1e-9
    assert abs(r["avg_pnl"] - sum(net_pnls) / len(net_pnls)) <= 5e-5 + 1e-9


def test_run_backtest_uses_custom_config_rates(run_bt):
    """換掉 engine 的費率（⛔ 不等於 config 的值）：扣費照自訂費率算，證明費率來自 config 而且真的被傳進去。"""
    out = run_bt([100, 100, 120, 120], [(1, "buy", 10), (2, "sell", 10)], commission_rate=0.002, tax_rate=0.004)
    (t,) = out["trades"]
    fee = 10 * 100 * 0.002 + 10 * 120 * (0.002 + 0.004)  # 2 + 7.2
    assert abs(t["commission"] - fee) <= 0.005
    assert abs(t["pnl"] - (200 - fee)) <= 0.005
    _check_result(out, [200 - fee])


def test_run_backtest_zero_rates(run_bt):
    out = run_bt([100, 100, 120, 120], [(1, "buy", 10), (2, "sell", 10)], commission_rate=0.0, tax_rate=0.0)
    (t,) = out["trades"]
    assert t["commission"] == 0.0 and t["pnl"] == 200.0
    _check_result(out, [200.0])


def test_run_backtest_rows_and_totals_with_config_rates(run_bt):
    """config 的費率（engine 預設）：多單、空單各一筆、期末全部平倉；明細存兩位、總表依位數驗算。"""
    c, t = engine.COMMISSION_RATE, engine.TAX_RATE
    out = run_bt([100, 100, 110, 105, 95, 95],
                 [(1, "buy", 10), (2, "sell", 10), (3, "sell", 7), (4, "buy", 7)])
    long_fee = 10 * 100 * c + 10 * 110 * (c + t)
    short_fee = 7 * 105 * (c + t) + 7 * 95 * c
    nets = [100 - long_fee, 70 - short_fee]
    rows = sorted(out["trades"], key=lambda r: r["entry_time"])
    assert [r["direction"] for r in rows] == ["BUY", "SELL"]
    for r, fee, net in zip(rows, [long_fee, short_fee], nets):
        assert abs(r["commission"] - fee) <= 0.005
        assert abs(r["pnl"] - net) <= 0.005
    _check_result(out, nets)


# ── modular 路徑的查證（D3：只查證、⛔ 不改） ─────────────────────────


class _Stop(Exception):
    pass


def test_modular_receives_config_rates(monkeypatch):
    """`run_backtest` 把 config 的費率傳給 modular 的 `BacktestEngine`；用⛔ 不等於參數預設值的自訂費率確認。"""
    seen: list[dict] = []

    class Spy:
        def __init__(self, **kwargs):
            seen.append(kwargs)
            raise _Stop

    monkeypatch.setattr(modular_service, "BacktestEngine", Spy)
    monkeypatch.setattr(engine, "COMMISSION_RATE", 0.002)
    monkeypatch.setattr(engine, "TAX_RATE", 0.004)
    with pytest.raises(_Stop):
        engine.run_backtest(sorted(engine.MODULAR_STRATEGIES)[0], ["T"], "1d", "2025-01-01", "2025-12-31")
    assert seen[0]["commission_rate"] == 0.002 and seen[0]["tax_rate"] == 0.004


@pytest.mark.parametrize("direction,entry_fee,exit_fee", [
    (Direction.LONG, 100 * 0.002, 120 * (0.002 + 0.004)),   # 多單：出場（賣出）課稅
    (Direction.SHORT, 100 * (0.002 + 0.004), 120 * 0.002),  # 空單：進場（賣出）課稅
])
def test_modular_close_taxes_the_sell_side(direction, entry_fee, exit_fee):
    eng = BacktestEngine(initial_cash=CASH, commission_rate=0.002, tax_rate=0.004)
    pos = Position(direction=direction, entry_index=0, entry_time=None, entry_price=100.0, stop_price=90.0)
    trade, _ = eng._close("T", pos, None, 120.0, ExitReason.EOD_FORCE_CLOSE, equity_at_entry=100.0)  # 1 股
    assert trade.size == 1.0
    assert trade.commission == pytest.approx(entry_fee + exit_fee, abs=1e-4)
