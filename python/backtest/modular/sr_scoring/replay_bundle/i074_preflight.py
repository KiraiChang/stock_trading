"""I-074 Stage 1 的**專屬** preflight（⛔ 不屬於通用 evaluation contract）。

⚠️ **為什麼是專屬的**：bundle ID 與 13417 這兩個值是**這一筆**的執行參數，⛔ 不得硬編進
通用路徑——smoke bundle 只有幾十列，通用守門若要求 13417 會讓它一律失敗。

⚠️ **拆成兩個時點**：v4 要求 preflight 排在 `_decision_replay_rows()` **之前**（跑完 3.2
小時才發現就不叫 preflight），但「13417 列」是 replay 的**輸出**、只有跑完才知道。所以：

* **pre**——用**輸入**推導的配額（replay context 推導完成後、replay 之前）；
* **post**——用**輸出**的實際列數（replay 之後、發布之前）。

⛔ 兩個都要：只留 post 的話輸入錯了要 3.2 小時後才知道；只留 pre 的話 replay 中途少產列
不會被發現。

⛔ **本模組不重算 candidate range**：`bars − min_history_bars − forward_bars` 的唯一真相源是
`evaluation.py` 的 `_candidate_bar_range()`，而本 package ⛔ 不得反向 import 它。
呼叫端先推導好 replay context，這裡**只比數字與集合**。
"""
from __future__ import annotations

from datetime import datetime, timedelta, timezone
from typing import Any, Iterable, Mapping, Sequence

from .artifacts import ArtifactError, row_key

# ── 這一筆的固定參數（⛔ 只在這裡出現，⛔ 不進通用 contract）─────────────────
I074_BUNDLE_ID = "b1_20260901_1d_74350966_5d7ecb10"

# 2026-09-01 baseline 的 11 檔，取自 `replay_cohort_2026-09-01.json` 的
# `runs[*].rows[*].symbol` distinct 集合。
I074_SYMBOLS = frozenset(
    {"0050", "00830", "00947", "00981A", "2330", "2399", "2454", "2478", "3630", "5490", "6243"}
)

# ⚠️ **台北交易日**，⛔ 不是 UTC 日期：末根 epoch 1788192000 的 UTC 日期是 2026-08-31，
# 轉 Asia/Taipei 才是 2026-09-01。直接比 UTC 會把正確的 bundle 擋掉。
I074_AS_OF_TAIPEI = "2026-09-01"

# 14352 根 candles − 11 檔 × (min_history_bars 80 ＋ forward_bars 5) ＝ 13417。
# ⚠️ 這是 **11 檔合計**的候選列數，⛔ 不是每檔。
I074_EXPECTED_ROWS = 13417

# ⛔ 不依賴 zoneinfo／tzdata：台北是固定 UTC+8（⛔ 無日光節約），
# 而這個模組要能在 host 的最小環境跑。
_TAIPEI = timezone(timedelta(hours=8))


def taipei_date(epoch_seconds: int) -> str:
    """epoch → UTC → Asia/Taipei → local date（`YYYY-MM-DD`）。"""
    return datetime.fromtimestamp(int(epoch_seconds), timezone.utc).astimezone(_TAIPEI).date().isoformat()


def preflight_pre(
    *,
    bundle_id: str,
    universe: Sequence[tuple[str, str, str]],
    quota: Mapping[str, int],
    last_timestamps: Mapping[str, int],
) -> None:
    """replay **之前**的四項檢查。

    `universe`／`quota` 由呼叫端以既有的 `_all_candidates_quota()` 推導，
    `last_timestamps` 是每檔最後一根 candle 的 epoch 秒。
    """
    if bundle_id != I074_BUNDLE_ID:
        raise ArtifactError(
            f"I-074 Stage 1 的 bundle_id 必須是 {I074_BUNDLE_ID}，實際 {bundle_id!r}——"
            "⛔ bundle 一旦選定就不得更換（換了等於整個 I-074 重來）。"
        )

    symbols = {key[0] for key in universe}
    if symbols != set(I074_SYMBOLS):
        raise ArtifactError(
            f"symbols 集合不符：多={sorted(symbols - set(I074_SYMBOLS))}、"
            f"缺={sorted(set(I074_SYMBOLS) - symbols)}"
        )

    bad_dates = {
        symbol: taipei_date(ts)
        for symbol, ts in sorted(last_timestamps.items())
        if taipei_date(ts) != I074_AS_OF_TAIPEI
    }
    if bad_dates:
        raise ArtifactError(
            f"下列標的的末根**台北日期**不是 {I074_AS_OF_TAIPEI}：{bad_dates}"
            "（⚠️ 比的是台北交易日，⛔ 不是 UTC 日期）"
        )

    quota_total = sum(quota.values())
    if quota_total != I074_EXPECTED_ROWS or len(universe) != I074_EXPECTED_ROWS:
        raise ArtifactError(
            f"候選列數不符：sum(quota)={quota_total}、len(universe)={len(universe)}，"
            f"預期都是 {I074_EXPECTED_ROWS}（11 檔合計）"
        )


def preflight_post(
    *,
    rows: Iterable[dict[str, Any]],
    universe: Sequence[tuple[str, str, str]],
) -> None:
    """replay **之後**、發布之前的檢查：實際列數與 key 集合。

    ⚠️ 這⛔ 不是 pre 的重複——pre 驗的是**輸入**推導出的配額，這裡驗的是**輸出**。
    兩個都等於 13417 才代表「載入範圍對」且「replay 真的走完了那個範圍」。
    """
    rows = list(rows)
    if len(rows) != I074_EXPECTED_ROWS:
        raise ArtifactError(
            f"replay 實際產出 {len(rows)} 列，預期 {I074_EXPECTED_ROWS}——"
            "⛔ 不發布不完整的正式 artifact。"
        )
    actual, expected = {row_key(row) for row in rows}, set(universe)
    if actual != expected:
        raise ArtifactError(
            f"replay 輸出的 key 集合與 universe 不符："
            f"多={sorted(actual - expected)[:5]}、缺={sorted(expected - actual)[:5]}"
        )
