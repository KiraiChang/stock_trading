"""POST /zone-identity/match（T-048 階段 B 接線）。

**這支端點刻意獨立於 /sr-zones**：階段 B 是「只寫不讀」，沒有任何決策依賴它的輸出。
掛進 /sr-zones 就得動 scoring.py / pipeline.py，為一個還沒有讀者的功能去動決策核心，
風險與收益不成比例。

端點本身是 `zone_matcher.match_zones` 的純函數包裝，matcher 的行為由
`backtest/modular/sr_scoring/tests/test_zone_matcher.py` 涵蓋（35 支）。
這裡只驗**接線**：欄位有沒有正確轉進去、轉出來。
"""
from __future__ import annotations

ENDPOINT = "/zone-identity/match"

# 真實形狀：0050 recent_pivot [104.73,105.37]，live 那筆邊界完全沒動卻翻轉的 zone。
PIVOT = {"price_low": 104.73, "price_high": 105.37, "method": "recent_pivot"}


def test_continuation_returns_existing_identity(client):
    response = client.post(ENDPOINT, json={
        "previous": [{**PIVOT, "role": "SUPPORT", "zone_uid": "Z-P"}],
        "current": [{**PIVOT, "role": "SUPPORT"}],
    })

    assert response.status_code == 200
    body = response.json()
    assert body["zone_uids"] == ["Z-P"]
    assert body["relations"] == []          # 延續不是血緣邊
    assert body["next_observed_absences"] == {"Z-P": 0}


def test_role_flip_is_reported(client):
    """邊界一動也沒動、只有角色翻轉——現行 `_zone_key()` 會判成兩個不同 zone。"""
    response = client.post(ENDPOINT, json={
        "previous": [{**PIVOT, "role": "SUPPORT", "zone_uid": "Z-P",
                      "incarnation_role": "SUPPORT"}],
        "current": [{**PIVOT, "role": "RESISTANCE"}],
    })

    body = response.json()
    assert body["zone_uids"] == ["Z-P"]
    assert body["role_transitions"] == [
        {"zone_uid": "Z-P", "kind": "ROLE_FLIPPED",
         "from_role": "SUPPORT", "to_role": "RESISTANCE"}
    ]
    assert body["incarnation_roles"] == ["RESISTANCE"]


def test_trading_days_may_be_descending(client):
    """`db.fetch_market_trading_days()` 是 `ORDER BY d DESC`——端點要吃得下去。

    直接把降冪清單丟給 `TradingCalendar()` 會 raise，所以這裡走 `from_iterable`。
    這條測的就是那個轉接有沒有做對。
    """
    response = client.post(ENDPOINT, json={
        "as_of": "2026-08-18",
        "trading_days": ["2026-08-18", "2026-08-17", "2026-08-14"],   # 降冪
        "previous": [{**PIVOT, "role": "SUPPORT", "zone_uid": "Z-P",
                      "last_seen_at": "2026-08-14"}],
        "current": [{**PIVOT, "role": "SUPPORT"}],
    })

    assert response.status_code == 200
    assert response.json()["zone_uids"] == ["Z-P"]


def test_absence_gate_expires_identity(client):
    """缺席次數達上限的身分要出現在 expired_previous，並把次數 +1。

    +1 是與 repo 端的握手：`ListLive` 用 `<= 上限` 才撈得到剛達上限的身分，
    收攤後 +1 越過上限，下次才不會被重複收攤。
    """
    response = client.post(ENDPOINT, json={
        "previous": [{**PIVOT, "role": "SUPPORT", "zone_uid": "Z-EXP",
                      "observed_absences": 3}],
        "current": [],
    })

    body = response.json()
    assert body["expired_previous"] == ["Z-EXP"]
    assert body["next_observed_absences"] == {"Z-EXP": 4}


def test_split_reports_lineage(client):
    response = client.post(ENDPOINT, json={
        "previous": [{"price_low": 100.0, "price_high": 110.0, "method": "atr",
                      "role": "SUPPORT", "zone_uid": "Z-P"}],
        "current": [
            {"price_low": 100.10, "price_high": 109.90, "method": "atr", "role": "SUPPORT"},
            {"price_low": 100.20, "price_high": 110.10, "method": "atr", "role": "SUPPORT"},
        ],
    })

    body = response.json()
    assert body["terminated_previous"] == ["Z-P"]
    assert {r["relation"] for r in body["relations"]} == {"SPLIT"}
    assert len(body["relations"]) == 2
    # 終止的 parent 不該拿到缺席計數——呼叫端照著存回去會讓它復活
    assert "Z-P" not in body["next_observed_absences"]


def test_empty_request_is_valid(client):
    """全空是合法請求：這檔第一次分析時 previous 就是空的。"""
    response = client.post(ENDPOINT, json={})

    assert response.status_code == 200
    assert response.json()["zone_uids"] == []


def test_endpoint_does_not_touch_the_database(client, monkeypatch):
    """純函數包裝，不該碰 DB——`client` fixture 本來就沒有可用連線。"""
    import http_server

    class _Exploding:
        def connect(self):
            raise AssertionError("/zone-identity/match 不該碰 DB")

    monkeypatch.setattr(http_server, "engine", _Exploding())

    response = client.post(ENDPOINT, json={
        "previous": [{**PIVOT, "role": "SUPPORT", "zone_uid": "Z-P"}],
        "current": [{**PIVOT, "role": "SUPPORT"}],
    })

    assert response.status_code == 200
