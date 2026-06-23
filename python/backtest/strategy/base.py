"""
Base backtrader strategy。
內建台灣股市手續費佣金模型，子策略只需實作 next()。
"""
import backtrader as bt
from config import COMMISSION_RATE, TAX_RATE


class TWCommission(bt.CommInfoBase):
    """台灣股市手續費：買賣 0.1425%，賣出交易稅 0.3%。"""
    params = (
        ("commission", COMMISSION_RATE),
        ("tax", TAX_RATE),
        ("stocklike", True),
        ("commtype", bt.CommInfoBase.COMM_PERC),
    )

    def getcommission(self, size: float, price: float) -> float:
        return abs(size) * price * self.p.commission

    def _getcommission(self, size: float, price: float, pseudoexec: bool) -> float:
        comm = abs(size) * price * self.p.commission
        # 賣出加交易稅
        if size < 0:
            comm += abs(size) * price * self.p.tax
        return comm


class BaseTWStrategy(bt.Strategy):
    """所有 Taiwan 策略的基底，統一設定 Log 和指標參數。"""

    def log(self, msg: str, dt=None) -> None:
        dt = dt or self.datas[0].datetime.date(0)
        print(f"[{dt}] {msg}")

    def notify_order(self, order: bt.Order) -> None:
        if order.status in [order.Submitted, order.Accepted]:
            return
        if order.status == order.Completed:
            direction = "BUY" if order.isbuy() else "SELL"
            self.log(
                f"{direction} {order.data._name}: "
                f"price={order.executed.price:.2f}, "
                f"size={order.executed.size:.0f}, "
                f"comm={order.executed.comm:.2f}"
            )
        elif order.status in [order.Canceled, order.Margin, order.Rejected]:
            self.log(f"Order {order.Status[order.status]}: {order.data._name}")
