"""交易日曆：request 契約、解析器、schema 與 readiness 查表（I-100 Phase B）。

對應計畫書「四、`--as-of` 的『資料是否到齊』怎麼判」與「十二」的日曆兩列。
"""
from __future__ import annotations

import json
from datetime import date

import pytest

from ..replay_bundle import (
    CalendarError,
    TradingCalendar,
    build_calendar_online,
    build_calendar_payload,
    classify_calendar_row,
    load_frozen_calendar,
    parse_calendar_date,
    validate_calendar_payload,
)
from ..replay_bundle.calendar import fetch_year_rows
from ..replay_bundle.canonical import canonical_json_bytes


# ── 解析器 ──────────────────────────────────────────────────────────────────

def test_parses_iso_and_compact_roc():
    assert parse_calendar_date("2026-01-01") == date(2026, 1, 1)
    assert parse_calendar_date(" 1150101 ") == date(2026, 1, 1)


def test_parses_leap_day():
    assert parse_calendar_date("1130229") == date(2024, 2, 29)
    assert parse_calendar_date("2024-02-29") == date(2024, 2, 29)


@pytest.mark.parametrize("raw", ["1150231", "2026-02-30", "1151301", "115/01/01", "", "20260101"])
def test_rejects_invalid_dates(raw):
    """⚠️ `1150231` ⛔ 不得被正規化成 3/3；`115/01/01` 是 `parseROCDate` 的格式，這裡不收。"""
    with pytest.raises(CalendarError):
        parse_calendar_date(raw)


# ── 列型分類 ────────────────────────────────────────────────────────────────

@pytest.mark.parametrize("name,desc,expected", [
    ("開始交易日", "", "trading_marked"),
    ("最後交易日", "", "trading_marked"),
    ("春節", "放假", "holiday"),
    ("補假", "補假", "holiday"),
    ("市場無交易，僅辦理結算交割作業", "", "settlement_only"),
])
def test_classify_weekday_rows(name, desc, expected):
    assert classify_calendar_row(date(2026, 2, 12), name, desc) == expected


def test_classify_weekend_row():
    assert classify_calendar_row(date(2026, 2, 28), "和平紀念日", "") == "weekend"


def test_classify_prefers_trading_over_holiday():
    """⚠️ 順序不能對調：同時命中時交易日語意優先。"""
    assert classify_calendar_row(date(2026, 2, 11), "最後交易日", "放假") == "trading_marked"


def test_unknown_row_type_aborts():
    with pytest.raises(CalendarError) as exc:
        classify_calendar_row(date(2026, 3, 2), "TWSE 改了字", "誰知道")
    assert "未知的日曆列型別" in str(exc.value)


# ── request 契約 ────────────────────────────────────────────────────────────

def _fake_http(rows, *, seen=None, year_override=None):
    def http_get(url, params):
        if seen is not None:
            seen.append((url, dict(params)))
        return {"stat": "OK", "data": rows if year_override is None else year_override}

    return http_get


def test_request_uses_date_param_and_never_queryyear():
    """⚠️ v13 的「錯誤年份測試」擋不住用錯參數名——它驗回應內容，沒有驗**實際送出的 query**。"""
    seen: list = []
    fetch_year_rows(2026, http_get=_fake_http([["1150101", "開國紀念日", "放假"]], seen=seen))
    _url, params = seen[0]
    assert params == {"date": "20260101", "response": "json"}
    assert "queryYear" not in params


def test_response_from_another_year_aborts():
    with pytest.raises(CalendarError) as exc:
        fetch_year_rows(2025, http_get=_fake_http([["1150101", "開國紀念日", "放假"]]))
    assert "卻回傳 2026" in str(exc.value)


def test_empty_data_aborts():
    with pytest.raises(CalendarError):
        fetch_year_rows(2026, http_get=_fake_http([]))


def test_fetch_failure_aborts_fail_closed():
    def boom(url, params):
        raise RuntimeError("connection reset")

    with pytest.raises(CalendarError) as exc:
        fetch_year_rows(2026, http_get=boom)
    assert "取得失敗" in str(exc.value)


def test_row_count_is_not_a_completeness_threshold():
    """⛔ 不用筆數當門檻——2026 是 27 筆、2025 是 24 筆，逐年本來就不同。"""
    overrides, raw_count, _ = fetch_year_rows(
        2026, http_get=_fake_http([["1150101", "開國紀念日", "放假"]])
    )
    assert raw_count == 1 and len(overrides) == 1


@pytest.mark.parametrize("second", ["開始交易日", "開國紀念日"])
def test_duplicate_date_aborts_even_when_the_classification_matches(second):
    """⛔ **重複日期一律中止**，分類相同也不放行。

    TWSE 同一天出現兩列本身就是「回應與既有假設不符」的訊號，靜默折疊會讓我們對輸入的
    理解與實際脫節——而這份日曆是 readiness 判斷的依據。
    """
    desc = "" if second == "開始交易日" else "放假"
    rows = [["1150101", "開國紀念日", "放假"], ["1150101", second, desc]]
    with pytest.raises(CalendarError) as exc:
        fetch_year_rows(2026, http_get=_fake_http(rows))
    assert "重複日期" in str(exc.value)


# ── payload 正規化 ──────────────────────────────────────────────────────────

def _online(years, rows_by_year):
    def http_get(url, params):
        year = int(params["date"][:4])
        return {"data": rows_by_year[year]}

    return build_calendar_online(years, http_get=http_get)


def test_payload_covers_every_day_of_every_covered_year():
    payload, provenance = _online([2025, 2026], {
        2025: [["1140101", "開國紀念日", "放假"]],
        2026: [["1150101", "開國紀念日", "放假"]],
    })
    assert payload["covered_years"] == [2025, 2026]
    assert len(payload["days"]) == 365 + 365
    assert provenance["mode"] == "online"
    assert [y["year"] for y in provenance["years"]] == [2025, 2026]
    assert all("fetched_at" in y and "raw_row_count" in y for y in provenance["years"])


def test_payload_has_no_fetch_time_or_raw_row_count():
    """⛔ 它們進 payload 就會進 content_hash8——同內容不同抓取時間就換 bundle ID。"""
    payload, _ = _online([2026], {2026: [["1150101", "開國紀念日", "放假"]]})
    text = json.dumps(payload, ensure_ascii=False)
    assert "fetched_at" not in text and "raw_row_count" not in text


def test_default_classification_for_days_not_listed():
    payload, _ = _online([2026], {2026: [["1150101", "開國紀念日", "放假"]]})
    table = {row["date"]: row for row in payload["days"]}
    assert table["2026-01-01"] == {"date": "2026-01-01", "is_trading_day": False,
                                   "row_type": "holiday"}
    # 2026-01-02 是週五：沒被列出就是普通交易日
    assert table["2026-01-02"]["row_type"] == "trading"
    assert table["2026-01-02"]["is_trading_day"] is True
    # 2026-01-03 是週六
    assert table["2026-01-03"]["row_type"] == "weekend"


def test_settlement_only_day_is_not_a_trading_day():
    """⚠️ 只扣「放假」會漏掉這一類——它們是平日、不是放假日，但市場無交易。"""
    payload, _ = _online([2026], {2026: [["1150212", "市場無交易，僅辦理結算交割作業", ""]]})
    row = next(r for r in payload["days"] if r["date"] == "2026-02-12")
    assert row["row_type"] == "settlement_only" and row["is_trading_day"] is False


def test_trading_marked_day_stays_a_trading_day():
    """⚠️ 全部扣除會讓「開始交易日」這種真正的交易日被排除。"""
    payload, _ = _online([2026], {2026: [["1150102", "開始交易日", ""]]})
    row = next(r for r in payload["days"] if r["date"] == "2026-01-02")
    assert row["row_type"] == "trading_marked" and row["is_trading_day"] is True


def test_trading_classification_on_weekend_is_fail_closed():
    with pytest.raises(CalendarError) as exc:
        _online([2026], {2026: [["1150103", "開始交易日", ""]]})
    assert "落在週末" in str(exc.value)


# ── schema 驗證 ─────────────────────────────────────────────────────────────

def _valid_payload():
    payload, _ = _online([2026], {2026: [["1150101", "開國紀念日", "放假"]]})
    return payload


def test_valid_payload_passes():
    validate_calendar_payload(_valid_payload())


@pytest.mark.parametrize("mutate,reason", [
    (lambda p: p.__setitem__("surprise", 1), "多欄位"),
    (lambda p: p.pop("source"), "缺欄位"),
    (lambda p: p.__setitem__("schema_version", True), "schema_version 是 bool"),
    (lambda p: p.__setitem__("schema_version", "1"), "schema_version 是字串"),
    (lambda p: p.__setitem__("schema_version", 2), "未知版本"),
    (lambda p: p.__setitem__("covered_years", [True]), "covered_years 是 bool"),
    (lambda p: p.__setitem__("covered_years", [2026, 2025]), "年度倒序"),
    (lambda p: p.__setitem__("covered_years", [2026, 2026]), "年度重複"),
    (lambda p: p.__setitem__("covered_years", [2025, 2026]), "年度集合與 days 不符"),
    (lambda p: p.__setitem__("source", "guessed"), "來源不符"),
    (lambda p: p["days"][0].__setitem__("is_trading_day", 0), "is_trading_day 是 0"),
    (lambda p: p["days"][0].__setitem__("is_trading_day", "true"), "is_trading_day 是字串"),
    (lambda p: p["days"][0].__setitem__("row_type", "unknown"), "未知 row_type"),
    (lambda p: p["days"][0].__setitem__("date", "2026-02-30"), "不存在的日期"),
    (lambda p: p["days"][0].__setitem__("extra", 1), "days 多欄位"),
    (lambda p: p["days"][0].pop("row_type"), "days 缺欄位"),
    (lambda p: p["days"].pop(5), "年度內少一天"),
])
def test_invalid_payloads_are_rejected(mutate, reason):
    payload = _valid_payload()
    mutate(payload)
    with pytest.raises(CalendarError):
        validate_calendar_payload(payload)


def test_row_type_and_flag_contradiction_is_rejected():
    """v12 補的守門：`{"row_type":"holiday","is_trading_day":true}` 必須被擋。"""
    payload = _valid_payload()
    payload["days"][0]["is_trading_day"] = True
    with pytest.raises(CalendarError) as exc:
        validate_calendar_payload(payload)
    assert "矛盾" in str(exc.value)


def test_duplicate_date_in_days_is_rejected():
    payload = _valid_payload()
    payload["days"][1] = dict(payload["days"][0])
    with pytest.raises(CalendarError) as exc:
        validate_calendar_payload(payload)
    assert "重複日期" in str(exc.value)


# ── frozen override ─────────────────────────────────────────────────────────

def test_frozen_and_online_produce_bitwise_identical_payload(tmp_path):
    """⚠️ 比的是 payload，**manifest 的 calendar provenance 本來就會不同**（模式不同）。"""
    online_payload, online_prov = _online([2026], {2026: [["1150101", "開國紀念日", "放假"]]})
    path = tmp_path / "trading_calendar.json"
    path.write_bytes(canonical_json_bytes(online_payload))

    frozen_payload, frozen_prov = load_frozen_calendar(path)
    assert canonical_json_bytes(frozen_payload) == canonical_json_bytes(online_payload)
    assert online_prov["mode"] == "online" and frozen_prov["mode"] == "frozen"
    assert frozen_prov["raw_fetch_info"] == "unavailable_in_frozen_mode"
    assert "fetched_at" not in frozen_prov and "raw_row_count" not in frozen_prov


def test_frozen_rejects_non_canonical_layout(tmp_path):
    """語意相同但縮排／鍵序不同的檔案也要被擋——它就是要求「與 bundle 內完全相同」。"""
    payload = _valid_payload()
    path = tmp_path / "pretty.json"
    path.write_text(json.dumps(payload, indent=2, ensure_ascii=False), encoding="utf-8")
    with pytest.raises(CalendarError) as exc:
        load_frozen_calendar(path)
    assert "canonical" in str(exc.value)


def test_frozen_rejects_unknown_field(tmp_path):
    payload = _valid_payload()
    payload["surprise"] = 1
    path = tmp_path / "extra.json"
    path.write_bytes(canonical_json_bytes(payload))
    with pytest.raises(CalendarError):
        load_frozen_calendar(path)


def test_frozen_rejects_raw_twse_response(tmp_path):
    """⛔ 不接受 TWSE 原始 response、單年度片段或多份 response 的集合。"""
    path = tmp_path / "raw.json"
    path.write_bytes(canonical_json_bytes({"stat": "OK", "data": [["1150101", "開國紀念日", "放假"]]}))
    with pytest.raises(CalendarError):
        load_frozen_calendar(path)


def test_frozen_sha256_is_stable_for_same_file(tmp_path):
    """⛔ `frozen_sha256` **不是** volatile 欄位：同一份檔案必須得到相同值。"""
    payload = _valid_payload()
    path = tmp_path / "c.json"
    path.write_bytes(canonical_json_bytes(payload))
    first = load_frozen_calendar(path)[1]["frozen_sha256"]
    second = load_frozen_calendar(path)[1]["frozen_sha256"]
    assert first == second


# ── 查表與 expected_latest ──────────────────────────────────────────────────

def _calendar(years, rows_by_year):
    payload, _ = _online(years, rows_by_year)
    return TradingCalendar(payload)


def test_expected_latest_on_weekend_falls_back_to_friday():
    cal = _calendar([2026], {2026: [["1150101", "開國紀念日", "放假"]]})
    # 2026-09-05 是週六 → 應退回 09-04（週五）
    assert cal.expected_latest(date(2026, 9, 5)) == date(2026, 9, 4)


def test_expected_latest_on_holiday_falls_back():
    cal = _calendar([2026], {2026: [["1150101", "開國紀念日", "放假"]]})
    # 2026-01-01 是假日（週四）→ 應退回 2025-12-31，但日曆只涵蓋 2026 → 中止
    with pytest.raises(CalendarError) as exc:
        cal.expected_latest(date(2026, 1, 1))
    assert "covered_years" in str(exc.value)


def test_expected_latest_crosses_year_boundary_when_previous_year_covered():
    """⚠️ `as_of` 落在年初時，前一個交易日在去年——只抓當年會算錯。"""
    cal = _calendar([2025, 2026], {
        2025: [["1141231", "最後交易日", ""]],
        2026: [["1150101", "開國紀念日", "放假"]],
    })
    assert cal.expected_latest(date(2026, 1, 1)) == date(2025, 12, 31)


def test_lookup_outside_covered_years_aborts():
    cal = _calendar([2026], {2026: [["1150101", "開國紀念日", "放假"]]})
    with pytest.raises(CalendarError):
        cal.is_trading_day(date(2025, 12, 31))


def test_lookup_is_table_driven_not_reclassified():
    """loader 直接查表：把表裡的值改掉，查詢結果就要跟著改（證明沒有重新分類）。"""
    payload, _ = _online([2026], {2026: [["1150101", "開國紀念日", "放假"]]})
    row = next(r for r in payload["days"] if r["date"] == "2026-01-02")
    row["is_trading_day"] = False
    row["row_type"] = "holiday"
    cal = TradingCalendar(payload)
    assert cal.is_trading_day(date(2026, 1, 2)) is False
