"""TWSE 交易日曆——`--as-of` 的「資料是否到齊」要靠它才判得準。

**為什麼不能用「candles 裡有哪些日期」反推**：`architecture.md` 已寫明**實際日期集合回答
不了「少了哪些天」**，休市日要靠交易日曆判斷，⛔ 不能把「缺列的日子」都當成休市。
readiness 的判準因此是：

```
expected_latest = max(trading_day <= as_of)          -- 依權威交易日曆
market_latest   = MAX(ts) over candles WHERE timeframe = :tf   -- 不分 symbol
market_latest < expected_latest → 中止（資料還沒到齊）
```

⚠️ **request 契約不是小事**：Go 端（`exchange_reference.go:296` 的註解與 `:334` 的實作）
已經踩過並記錄——參數名一定是 **`date=<YYYY>0101`**，用 `queryYear` 會被**完全忽略**，
端點照樣回 HTTP 200、格式正常，但回的是**當年**資料。所以①只送 `date` ＋ `response`、
②**逐列驗年**、③⛔ 不用筆數當完整性門檻（2026 是 27 筆、2025 是 24 筆，逐年本來就不同）。

⚠️ **本檔存的是「正規化後的逐日分類結果」**，⛔ 不是 TWSE 原始列、也不是「例外日清單」：
後兩者都要求 loader 自己重跑一次分類，那就無法保證與 Stage 0 得到相同答案。

⚠️ **與 Go 端 `IsTradingDay` 的一個刻意差異**：Go 那邊「週末一律不是交易日」是寫死在
判定函式裡的；這裡改成把結論**存進表**，並在 builder 就擋下「被分類成交易日、卻落在週末」
的列（fail-closed）。TWSE 真的排出週末交易日時，這裡會中止而不是靜默給出與 Go 不同的答案。
"""
from __future__ import annotations

import json
import re
from datetime import date, datetime, timedelta, timezone
from pathlib import Path
from typing import Any, Callable

from .canonical import canonical_json_bytes, sha256_hex

CALENDAR_SCHEMA_VERSION = 1
CALENDAR_SOURCE = "twse_holidaySchedule"
TWSE_CALENDAR_URL = "https://www.twse.com.tw/rwd/zh/holidaySchedule/holidaySchedule"

# 封閉 enum。⛔ 未知值即中止。
ROW_TYPES = ("trading", "weekend", "holiday", "settlement_only", "trading_marked")

# `row_type` 與 `is_trading_day` 的固定映射——builder 與 loader **兩端都要驗**。
# 同時存兩個能表達交易狀態的欄位卻沒有映射規則的話，
# `{"row_type":"holiday","is_trading_day":true}` 不會被任何東西擋下來。
_TRADING_ROW_TYPES = frozenset({"trading", "trading_marked"})

_ISO_RE = re.compile(r"^\d{4}-\d{2}-\d{2}$")
_COMPACT_ROC_RE = re.compile(r"^\d{7}$")


class CalendarError(ValueError):
    """交易日曆取得、解析或驗證失敗。⛔ 一律中止，不猜。"""


# ── 解析（移植自 Go 的 parseCalendarDate ＋ newStrictDate） ──────────────────

def _new_strict_date(year: int, month: int, day: int, raw: str) -> date:
    """⚠️ 嚴格：`1150231` ⛔ 不得被正規化成 3/3。"""
    if not (1 <= month <= 12 and 1 <= day <= 31):
        raise CalendarError(f"日期超出範圍 {raw!r}")
    try:
        return date(year, month, day)
    except ValueError as exc:
        raise CalendarError(f"日期不存在 {raw!r}") from exc


def parse_calendar_date(raw: str) -> date:
    """holidaySchedule 的日期是 **ISO（`2026-01-01`）或 compact 民國（`1150101`）**。

    ⛔ 這裡要移植的是 `parseCalendarDate` ＋ `newStrictDate`，**不是 `parseROCDate`**——
    後者吃的是 `115/01/01`，照它實作會把 TWSE 的合法格式全部拒絕。
    """
    text = str(raw).strip()
    if _ISO_RE.match(text):
        y, m, d = text.split("-")
        return _new_strict_date(int(y), int(m), int(d), text)
    if _COMPACT_ROC_RE.match(text):
        return _new_strict_date(int(text[:3]) + 1911, int(text[3:5]), int(text[5:7]), text)
    raise CalendarError(f"無法解析日期 {raw!r}")


def classify_calendar_row(day: date, name: str, desc: str) -> str:
    """逐列分類，**不是集合相減**。順序沿用 Go 端 `classifyCalendarRow`。

    ⚠️ **順序不能對調**：交易日語意（開始／最後交易日）必須排在「放假」之前，
    某列同時被兩者命中時交易日優先。

    ⚠️ 這是**中文名稱的字串比對，本質脆弱**——TWSE 改字就會落到最後一條而中止。
    **那個方向是刻意的**：讓改字壞在明處，而不是被猜過去。
    """
    joined = f"{name} {desc}"
    if "開始交易" in joined or "最後交易" in joined:
        return "trading_marked"
    # 實測這類列的「說明」是空字串，所以條件要看**名稱**。
    if "市場無交易" in name:
        return "settlement_only"
    if "放假" in desc or "補假" in desc:
        return "holiday"
    if day.weekday() >= 5:
        return "weekend"
    raise CalendarError(f"未知的日曆列型別 date={day.isoformat()} name={name!r} desc={desc!r}")


def _normalize_text(value: object) -> str:
    return re.sub(r"\s+", " ", str(value or "")).strip()


# ── 線上取得 ────────────────────────────────────────────────────────────────

def fetch_year_rows(year: int, *, url: str = TWSE_CALENDAR_URL, http_get: Callable | None = None):
    """回傳 `(overrides, raw_row_count, fetched_at)`。

    `overrides` 是 `{date: row_type}`——只有 TWSE 明確列出的那些日子。
    """
    params = {"date": f"{year:04d}0101", "response": "json"}
    fetched_at = datetime.now(timezone.utc).isoformat()
    try:
        payload = (http_get or _default_http_get)(url, params)
    except Exception as exc:  # noqa: BLE001 - 網路層的例外型別很多，統一 fail-closed。
        raise CalendarError(f"交易日曆取得失敗（{year}）：{exc}") from exc

    if not isinstance(payload, dict):
        raise CalendarError(f"交易日曆回應格式異常（{year}）：不是 JSON object")
    rows = payload.get("data")
    # ⛔ 不用筆數當完整性門檻——只驗「非空 ＋ 年份相符」。
    if not isinstance(rows, list) or not rows:
        raise CalendarError(f"交易日曆 {year} 回傳 0 筆")

    overrides: dict[date, str] = {}
    for row in rows:
        if not isinstance(row, list) or len(row) < 2:
            raise CalendarError(f"交易日曆 {year} 欄位數異常：{row!r}")
        day = parse_calendar_date(row[0])
        # **每一列都要驗年份**：`queryYear` 的行為證明 TWSE 會對無效參數回傳看似正常的資料。
        if day.year != year:
            raise CalendarError(f"交易日曆請求 {year} 卻回傳 {day.year}（{row[0]!r}）")
        name = _normalize_text(row[1])
        desc = _normalize_text(row[2]) if len(row) >= 3 else ""
        row_type = classify_calendar_row(day, name, desc)
        # ⛔ **重複日期一律中止**，分類相同也不放行：TWSE 同一天出現兩列本身就是
        # 「回應與既有假設不符」的訊號，靜默折疊會讓我們對輸入的理解與實際脫節。
        if day in overrides:
            raise CalendarError(
                f"交易日曆 {year} 有重複日期：{day.isoformat()}"
                f"（既有分類 {overrides[day]}、這一列 {row_type}）"
            )
        overrides[day] = row_type
    return overrides, len(rows), fetched_at


def _default_http_get(url: str, params: dict[str, str]) -> Any:
    import httpx

    resp = httpx.get(url, params=params, timeout=30.0)
    resp.raise_for_status()
    return resp.json()


# ── 正規化成 payload ────────────────────────────────────────────────────────

def _days_of_year(year: int):
    day = date(year, 1, 1)
    while day.year == year:
        yield day
        day += timedelta(days=1)


def build_calendar_payload(years, overrides_by_year: dict[int, dict[date, str]]) -> dict[str, Any]:
    """把逐年的 override 展開成**每一天恰好一列**的分類表。

    ⛔ 不是「例外日清單」：`days[]` 一定涵蓋每個 covered year 的 1/1～12/31。
    """
    covered = sorted({int(y) for y in years})
    if not covered:
        raise CalendarError("covered_years 不得為空")
    days: list[dict[str, Any]] = []
    for year in covered:
        overrides = overrides_by_year.get(year, {})
        for day in _days_of_year(year):
            row_type = overrides.get(day)
            if row_type is None:
                row_type = "weekend" if day.weekday() >= 5 else "trading"
            elif row_type in _TRADING_ROW_TYPES and day.weekday() >= 5:
                # fail-closed：見模組說明的「與 Go 端的一個刻意差異」。
                raise CalendarError(
                    f"{day.isoformat()} 被分類成 {row_type} 卻落在週末——"
                    "TWSE 的排程與既有假設不符，這裡中止而不是猜一個答案。"
                )
            days.append({
                "date": day.isoformat(),
                "is_trading_day": row_type in _TRADING_ROW_TYPES,
                "row_type": row_type,
            })
    payload = {
        "schema_version": CALENDAR_SCHEMA_VERSION,
        "source": CALENDAR_SOURCE,
        "covered_years": covered,
        "days": days,
    }
    validate_calendar_payload(payload)
    return payload


def build_calendar_online(years, *, url: str = TWSE_CALENDAR_URL, http_get: Callable | None = None):
    """線上模式：回傳 `(payload, provenance)`。

    ⛔ `fetched_at`／`raw_row_count` **一律不在 payload 裡**：它們進 payload 就會進
    `content_hash8`——日曆內容完全相同、只是抓取時間不同，bundle ID 就會變，
    與「同輸入重產只有 `captured_at` 不同」直接矛盾。兩者放 manifest 的 provenance 區。
    """
    overrides_by_year: dict[int, dict[date, str]] = {}
    provenance_years: list[dict[str, Any]] = []
    for year in sorted({int(y) for y in years}):
        overrides, raw_row_count, fetched_at = fetch_year_rows(year, url=url, http_get=http_get)
        overrides_by_year[year] = overrides
        provenance_years.append({
            "year": year, "fetched_at": fetched_at, "raw_row_count": raw_row_count,
        })
    payload = build_calendar_payload(sorted(overrides_by_year), overrides_by_year)
    return payload, {"mode": "online", "years": provenance_years}


def load_frozen_calendar(path: Path):
    """`--trading-calendar <file>` 的替代入口。回傳 `(payload, provenance)`。

    ⛔ **只接受一種格式：與 bundle 內完全相同的 canonical `trading_calendar.json`**。
    不接受 TWSE 原始 response、不接受單年度片段、不接受多份 response 的集合——
    那些都要求 loader 自己重跑一次正規化，就回到「無法保證與 Stage 0 得到相同答案」的問題。

    ⚠️ **收下之後要重做一次 canonical 序列化，與輸入 bytes 逐位元比對**。這條是**故意嚴格**
    的：語意相同但縮排、鍵序或多餘空白不同的檔案也會被擋，因為它要求的就是「與 bundle 內
    那一份完全相同」。
    """
    path = Path(path)
    try:
        raw = path.read_bytes()
    except OSError as exc:
        raise CalendarError(f"讀不到 frozen 交易日曆 {path}：{exc}") from exc
    try:
        payload = json.loads(raw.decode("utf-8"))
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise CalendarError(f"frozen 交易日曆不是合法 JSON：{exc}") from exc
    validate_calendar_payload(payload)
    if canonical_json_bytes(payload) != raw:
        raise CalendarError(
            f"frozen 交易日曆 {path} 不是 canonical 形式（縮排／鍵序／多餘空白都算）——"
            "⛔ 它必須與 bundle 內的 trading_calendar.json 逐位元相同。"
        )
    provenance = {
        "mode": "frozen",
        "frozen_sha256": sha256_hex(raw),
        "loaded_at": datetime.now(timezone.utc).isoformat(),
        # ⚠️ frozen 輸入裡**根本沒有**原始抓取時間與原始列數，明確標為不可得，⛔ 不假造。
        "raw_fetch_info": "unavailable_in_frozen_mode",
    }
    return payload, provenance


# ── 驗證（exact-field） ─────────────────────────────────────────────────────

def _require_int(value: object, field: str) -> int:
    """⚠️ 整數一律用 `type(x) is int`，⛔ 不用 `isinstance`。

    `isinstance(True, int)` 是 `True`，而且 `True == 1`——所以 `{"schema_version": true}`
    連「值必須等於 1」那道檢查都會一起通過。
    """
    if type(value) is not int:
        raise CalendarError(f"{field} 必須是 int：{value!r}")
    return value


def validate_calendar_payload(payload: object) -> None:
    """exact-field validation：多一個未知欄位、少一個必要欄位，一律中止（兩層都是）。

    ⚠️ 沒有 exact-field 規則時，**frozen 檔多一個 loader 會忽略的欄位，語意不變卻換一個
    `bundle_id`**（多的欄位仍進 payload bytes → 進 `content_hash8`）。
    """
    if not isinstance(payload, dict):
        raise CalendarError("trading_calendar 必須是 object")
    expected = {"schema_version", "source", "covered_years", "days"}
    unknown, missing = set(payload) - expected, expected - set(payload)
    if unknown or missing:
        raise CalendarError(f"trading_calendar 欄位不符：缺 {sorted(missing)}，多 {sorted(unknown)}")

    version = _require_int(payload["schema_version"], "schema_version")
    if version != CALENDAR_SCHEMA_VERSION:
        raise CalendarError(f"未知的 trading_calendar schema_version={version}")
    if payload["source"] != CALENDAR_SOURCE:
        raise CalendarError(f"trading_calendar.source 必須是 {CALENDAR_SOURCE!r}")

    covered = payload["covered_years"]
    if not isinstance(covered, list) or not covered:
        raise CalendarError("covered_years 必須是非空陣列")
    years = [_require_int(y, "covered_years[]") for y in covered]
    # ⛔ 嚴格升冪、不得重複：canonical JSON 的 `sort_keys` **只排物件的鍵，不排陣列**，
    # `[2025, 2026]` 與 `[2026, 2025]` 語意相同卻會得到不同的 payload hash。
    if years != sorted(set(years)):
        raise CalendarError(f"covered_years 必須嚴格升冪且不重複：{years}")

    days = payload["days"]
    if not isinstance(days, list) or not days:
        raise CalendarError("days 必須是非空陣列")
    seen: set[str] = set()
    day_fields = {"date", "is_trading_day", "row_type"}
    for item in days:
        if not isinstance(item, dict):
            raise CalendarError("days[] 的每一列都必須是 object")
        unknown, missing = set(item) - day_fields, day_fields - set(item)
        if unknown or missing:
            raise CalendarError(f"days[] 欄位不符：缺 {sorted(missing)}，多 {sorted(unknown)}")
        raw_date = item["date"]
        if not isinstance(raw_date, str) or not _ISO_RE.match(raw_date):
            raise CalendarError(f"days[].date 必須是 YYYY-MM-DD：{raw_date!r}")
        y, m, d = raw_date.split("-")
        parsed = _new_strict_date(int(y), int(m), int(d), raw_date)  # ⛔ 2026-02-30 拒絕
        if raw_date in seen:
            raise CalendarError(f"days[] 有重複日期：{raw_date}")
        seen.add(raw_date)
        if parsed.year not in set(years):
            raise CalendarError(f"days[].date {raw_date} 落在 covered_years {years} 之外")
        flag = item["is_trading_day"]
        # ⚠️ 驗 bool 要用 isinstance，⛔ 不能用 `isinstance(x, int)`——bool 是 int 的子類，
        # 後者會讓 `1` 通過。
        if not isinstance(flag, bool):
            raise CalendarError(f"days[].is_trading_day 必須是 JSON true/false：{flag!r}")
        row_type = item["row_type"]
        if row_type not in ROW_TYPES:
            raise CalendarError(f"未知的 row_type：{row_type!r}")
        if flag != (row_type in _TRADING_ROW_TYPES):
            raise CalendarError(
                f"{raw_date} 的 row_type={row_type!r} 與 is_trading_day={flag!r} 矛盾"
            )

    # **完整年度不變條件**：每個 covered year 的 1/1～12/31 每一天恰好一列。
    for year in years:
        expected_days = 366 if _is_leap(year) else 365
        actual = sum(1 for d in seen if d.startswith(f"{year:04d}-"))
        if actual != expected_days:
            raise CalendarError(
                f"{year} 年應有 {expected_days} 列，實際 {actual} 列——"
                "⛔ 年度內少任何一天即中止，不得把「查不到」當成交易日或非交易日。"
            )
    # `covered_years` 必須與 `days[].date` 的年度集合完全相等。
    day_years = {int(d[:4]) for d in seen}
    if day_years != set(years):
        raise CalendarError(f"covered_years {years} 與 days[] 的年度集合 {sorted(day_years)} 不符")


def _is_leap(year: int) -> bool:
    return year % 4 == 0 and (year % 100 != 0 or year % 400 == 0)


# ── 查表 ────────────────────────────────────────────────────────────────────

class TradingCalendar:
    """已驗證的日曆。**loader 直接查表**得到 `is_trading_day`，⛔ 不重新分類。"""

    def __init__(self, payload: dict[str, Any]) -> None:
        validate_calendar_payload(payload)
        self.payload = payload
        self.covered_years = set(payload["covered_years"])
        self._table = {row["date"]: row["is_trading_day"] for row in payload["days"]}

    def is_trading_day(self, day: date) -> bool:
        """⚠️ 查詢落在 `covered_years` 之外一律中止。"""
        if day.year not in self.covered_years:
            raise CalendarError(
                f"{day.isoformat()} 落在 covered_years {sorted(self.covered_years)} 之外"
            )
        try:
            return self._table[day.isoformat()]
        except KeyError as exc:  # pragma: no cover - 完整年度不變條件已擋住
            raise CalendarError(f"日曆缺少 {day.isoformat()}") from exc

    def expected_latest(self, as_of: date) -> date:
        """`max(trading_day <= as_of)`。⛔ 算不出任何一天即中止。"""
        day = as_of
        while True:
            if day.year not in self.covered_years:
                raise CalendarError(
                    f"從 {as_of.isoformat()} 往前找不到任何交易日——"
                    f"已退到 {day.isoformat()}，超出 covered_years {sorted(self.covered_years)}。"
                    "⚠️ `as_of` 落在年初時前一個交易日在去年，日曆必須同時涵蓋前一年度。"
                )
            if self.is_trading_day(day):
                return day
            day -= timedelta(days=1)
