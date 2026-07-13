"""SR Zone 展示層共用的字串格式化 helper。

explain_engine 與 scenario_engine 都是 Score/Evidence/Decision 之上的展示層，
共用同一套價格/百分比/角色標籤格式，集中在這裡避免各自維護一份。
"""
from __future__ import annotations

from .types import ZoneType


def fmt_price(v: float) -> str:
    return f"{v:.2f}"


def fmt_pct(v: float | None, digits: int = 1) -> str:
    if v is None:
        return "無資料"
    return f"{v * 100:.{digits}f}%"


def fmt_signed_pct(v: float | None, digits: int = 2) -> str:
    if v is None:
        return "無資料"
    sign = "+" if v > 0 else ""
    return f"{sign}{v * 100:.{digits}f}%"


def role_label(role: str) -> str:
    return {
        ZoneType.SUPPORT.value: "支撐",
        ZoneType.RESISTANCE.value: "壓力",
        ZoneType.AT_ZONE.value: "方向未定區",
    }.get(role, role)
