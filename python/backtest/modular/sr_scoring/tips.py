"""SR Zone report reading tips.

Tips are intentionally framed as technical-analysis reading guidance, not UI
instructions or implementation notes. The public response still returns a flat
list of strings to keep the API compatible with existing frontend clients.
"""
from __future__ import annotations

from typing import Optional

from .formatting import fmt_price

CHIP_SIGNAL_WEAK_THRESHOLD = 10.0
CHIP_SIGNAL_STRONG_THRESHOLD = 30.0


def _moving_average_tip(current_price: float, ma5: Optional[float]) -> str:
    if ma5 is None:
        return "判讀提醒｜均線：5日均線資料不足時，先以區間邊界與收盤確認判讀。"
    diff = (current_price - ma5) / ma5 if ma5 else 0.0
    if diff > 0.01:
        return f"判讀提醒｜均線：收盤站上5日均線（{fmt_price(ma5)}），短線動能偏穩，但仍需搭配量能確認。"
    if diff < -0.01:
        return f"判讀提醒｜均線：收盤跌破5日均線（{fmt_price(ma5)}），短線動能偏弱，靠近支撐也不代表可直接買進。"
    return f"判讀提醒｜均線：收盤接近5日均線（{fmt_price(ma5)}），方向仍在整理，應等待突破或跌破確認。"


def _chip_tip(chip_score: Optional[float]) -> str:
    if chip_score is None:
        return "判讀提醒｜籌碼：尚無籌碼分數時，籌碼項先以中性看待，不代表偏多或偏空。"
    if chip_score >= CHIP_SIGNAL_STRONG_THRESHOLD:
        return "判讀提醒｜籌碼：籌碼偏多可增加支撐參考性，但支撐仍不是買點，仍需價格與量能確認。"
    if chip_score >= CHIP_SIGNAL_WEAK_THRESHOLD:
        return "判讀提醒｜籌碼：籌碼略偏多只是輔助加分，不能取代區間突破、反彈或停損規劃。"
    if chip_score <= -CHIP_SIGNAL_STRONG_THRESHOLD:
        return "判讀提醒｜籌碼：籌碼偏空時，支撐被測試需要更嚴格確認，壓力也可能更容易形成壓制。"
    if chip_score <= -CHIP_SIGNAL_WEAK_THRESHOLD:
        return "判讀提醒｜籌碼：籌碼略偏空代表支撐需要更多確認，不等於所有壓力都適合放空。"
    return "判讀提醒｜籌碼：籌碼分數接近中性時，不應把它解讀成明確方向訊號。"


def _build_analysis_tips(
    period_summaries: list[dict], current_price: float, ma5: Optional[float], chip_score: Optional[float]
) -> list[str]:
    return [
        "指標小辭典｜RR：Risk/Reward，衡量預期報酬相對預期風險；RR 高不等於勝率高。",
        "指標小辭典｜EV：Expected Value，表示機率加權後的期望報酬，會同時考慮反彈與跌破情境。",
        "指標小辭典｜Confidence：信心分數反映樣本數、近期性與穩定度；低信心不是看空，而是需要更多確認。",
        "指標小辭典｜Trading Score：綜合 EV、RR、趨勢、量能、信心與籌碼後的交易參考分數，不是單一買賣訊號。",
        "指標小辭典｜Confluence：多方法共振，代表不同證據族群指向相近價位，但仍需後續 K 棒確認。",
        "價位語意｜Support：支撐是現價下方可能出現承接的區間，不是自動買點。",
        "價位語意｜Resistance：壓力是現價上方可能遇到賣壓的區間，不是自動放空點。",
        "價位語意｜AT_ZONE：價格正在區間內，應優先觀察收盤站回、跌破或量能變化。",
        "價位語意｜Primary Zone：主區是目前最具決策參考性的區間，但仍要搭配風險報酬與停損位置。",
        "事件語意｜Break：Break 指收盤有效穿越區間邊界；單根刺穿不等於有效突破或跌破。",
        "事件語意｜Bounce：Bounce 指價格測試支撐或壓力後出現反彈或壓回，需要收盤與量能確認。",
        "事件語意｜Reclaim：Reclaim 指跌破後重新站回關鍵區間，常用來觀察失而復得的結構修復。",
        "事件語意｜Invalidated：Invalidated 代表原區間參考性被破壞，應重新評估支撐壓力與停損。",
        "事件語意｜Pullback：Pullback 是趨勢中的回測，不等於反轉；要看是否守住關鍵區間。",
        _moving_average_tip(current_price, ma5),
        _chip_tip(chip_score),
    ]
