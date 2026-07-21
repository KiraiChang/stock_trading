"""SR Zone tier / role 顯示標籤共用定義。

`scoring.py`（zone 評分序列化）與 `decision_engine.py`（decision snapshot 序列化）
都需要把 zone 的 tier / role 轉成中文顯示標籤。抽到這裡單一來源，避免兩邊各自
定義一份、修改時漏改一邊造成 drift（見 T-035）。

只依賴 `types.py`；`scoring` 依賴 `decision_engine`，兩者再依賴本模組，無循環。
"""
from __future__ import annotations

from typing import Optional

from .types import ZoneTier, ZoneType

TIER_LABEL_TEXT = {
    ZoneTier.TIER_1_MAIN_STRUCTURE.value: "主結構",
    ZoneTier.TIER_2_TRADING_ZONE.value: "交易區",
    ZoneTier.TIER_3_SHORT_TERM.value: "短期",
}

ROLE_LABEL_TEXT = {
    ZoneType.SUPPORT.value: "支撐",
    ZoneType.RESISTANCE.value: "壓力",
    ZoneType.AT_ZONE.value: "區間內",
}


def role_label(role: str) -> str:
    return ROLE_LABEL_TEXT.get(role, role)


def display_label(tier: Optional[str], role: str, fallback_tier_label: Optional[str] = None) -> str:
    tier_label = TIER_LABEL_TEXT.get(tier or "", fallback_tier_label or tier or "")
    return f"{tier_label}{role_label(role)}"
