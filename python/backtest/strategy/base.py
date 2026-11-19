"""
Base backtrader strategy。
內建台灣股市手續費佣金模型，子策略只需實作 next()。
"""
import backtrader as bt
from config import COMMISSION_RATE, TAX_RATE


class TWCommission(bt.CommInfoBase):
    """台股費用的**設定費率模擬模型**（I-136）：買賣都收手續費 `commission`，賣出另收交易稅 `tax`。

    * 兩個參數都是**比例值**（`0.001425`＝0.1425%）：`percabs=True`，⛔ 不能拿掉（backtrader 預設會把 `COMM_PERC`
      的 `commission` 除以 100）。
    * 費率由呼叫端傳入（`engine.py` 傳 config 的 `COMMISSION_RATE`、`TAX_RATE`）；params 的預設值只是後備。
    * ⛔ 不覆寫 `getcommission()`：父類別的 `getcommission()` 委派給 `_getcommission()`，broker 實際扣款走這條，
      手續費與交易稅都只在 `_getcommission()` 算。
    * 賣出＝`size < 0`：broker 傳入的成交股數帶委託的正負號，平多單與開空單的賣出都課稅。
    * ⛔ 沒有最低手續費、⛔ 不取整數、⛔ 沒有商品別稅率（ETF、當沖等），見 docs/todo.md T-085。
    """
    params = (
        ("commission", COMMISSION_RATE),
        ("tax", TAX_RATE),
        ("stocklike", True),
        ("commtype", bt.CommInfoBase.COMM_PERC),
        ("percabs", True),
    )

    def _getcommission(self, size: float, price: float, pseudoexec: bool) -> float:
        notional = abs(size) * price
        comm = notional * self.p.commission
        if size < 0:  # 賣出（含開空單）加交易稅
            comm += notional * self.p.tax
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
