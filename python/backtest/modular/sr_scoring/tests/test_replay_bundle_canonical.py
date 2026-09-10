"""canonical 序列化與內容指紋（I-100 Phase A）。

這一檔鎖住的是**bundle 身分**：同一份輸入必須逐位元重現，而任何「看起來無害」的差異
（多一個空白、gzip 的時間戳、陣列順序）都會換一個 `bundle_id`。
"""
from __future__ import annotations

import gzip
from datetime import datetime, timezone
from decimal import Decimal

import pytest

from ..replay_bundle import canonical_gzip_bytes, canonical_json_bytes, content_hash8, symbols_hash8
from ..replay_bundle.canonical import CanonicalError, gunzip_bytes, sha256_hex


def test_json_is_sorted_compact_and_has_no_trailing_newline():
    out = canonical_json_bytes({"b": 1, "a": {"d": 2, "c": 3}})
    assert out == b'{"a":{"c":3,"d":2},"b":1}'


def test_json_keeps_unicode_raw():
    """`ensure_ascii=False`：中文不轉義。轉義與否會改變 bytes，所以必須釘住。"""
    assert canonical_json_bytes({"k": "中文"}) == '{"k":"中文"}'.encode("utf-8")


def test_json_rejects_nan_and_infinity_with_path():
    with pytest.raises(CanonicalError) as exc:
        canonical_json_bytes({"rows": [{"close": float("nan")}]})
    assert "$.rows[0].close" in str(exc.value)
    with pytest.raises(CanonicalError):
        canonical_json_bytes([float("inf")])


def test_json_datetime_normalized_to_utc_iso():
    naive = datetime(2026, 9, 1, 13, 30, 0)
    aware = datetime(2026, 9, 1, 21, 30, 0, tzinfo=timezone(offset=__import__("datetime").timedelta(hours=8)))
    # tz-naive 當 UTC；tz-aware 換算到 UTC——兩者必須落在同一個字串。
    assert canonical_json_bytes(naive) == canonical_json_bytes(aware)
    assert b"2026-09-01T13:30:00+00:00" in canonical_json_bytes(naive)


def test_json_decimal_becomes_string_not_float():
    """⛔ Decimal→float 會把 DB 的精確十進位值換成最近的二進位近似。"""
    assert canonical_json_bytes(Decimal("0.1")) == b'"0.1"'


def test_json_rejects_non_string_keys_and_bytes():
    with pytest.raises(CanonicalError):
        canonical_json_bytes({1: "x"})
    with pytest.raises(CanonicalError):
        canonical_json_bytes({"k": b"raw"})


def test_gzip_is_deterministic_and_writes_no_mtime_or_filename():
    payload = b"x" * 200
    first = canonical_gzip_bytes(payload)
    second = canonical_gzip_bytes(payload)
    assert first == second
    # gzip header：bytes 4-7 是 mtime，FNAME flag 是 FLG 的 bit 3。
    assert first[4:8] == b"\x00\x00\x00\x00"
    assert first[3] & 0x08 == 0
    assert gunzip_bytes(first) == payload


def test_gzip_differs_from_stdlib_default_mtime():
    """對照組：stdlib 預設會寫入當下時間，所以同內容會產出不同 bytes。"""
    payload = b"y" * 50
    with_time = gzip.compress(payload, mtime=12345)
    assert with_time != canonical_gzip_bytes(payload)


def test_content_hash8_binds_filename_to_content():
    """⚠️ 兩份 payload 內容互換時**必須**得到不同的 hash。"""
    a = sha256_hex(b"candles")
    b = sha256_hex(b"chip")
    first = content_hash8({"candles.json.gz": a, "chip.json.gz": b})
    swapped = content_hash8({"candles.json.gz": b, "chip.json.gz": a})
    assert first != swapped


def test_content_hash8_is_order_independent_and_8_hex():
    a = sha256_hex(b"1")
    b = sha256_hex(b"2")
    assert content_hash8({"x": a, "y": b}) == content_hash8({"y": b, "x": a})
    assert len(content_hash8({"x": a})) == 8


def test_content_hash8_rejects_malformed_hash():
    with pytest.raises(CanonicalError):
        content_hash8({"x": "ABCDEF"})
    with pytest.raises(CanonicalError):
        content_hash8({})


def test_symbols_hash8_dedupes_and_ignores_order():
    assert symbols_hash8(["2330", "0050", "2330"]) == symbols_hash8(["0050", "2330"])
    assert symbols_hash8(["2330"]) != symbols_hash8(["2454"])
    with pytest.raises(CanonicalError):
        symbols_hash8([" ", ""])
