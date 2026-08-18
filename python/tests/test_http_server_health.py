"""GET /health——liveness 檢查。

**刻意不碰 DB**：`client` fixture 不跑 lifespan，這條測試能過就代表 /health 沒有
偷偷加上依賴。這正是它該有的樣子——liveness 探針如果會因為 DB 短暫不可用而失敗，
compose 會把一個其實還活著的 container 重啟掉。要驗 DB 連得上是 readiness 的事，
啟動期的 fail-fast 行為由 test_http_server_startup.py 守。
"""
from __future__ import annotations

ENDPOINT = "/health"


def test_health_returns_ok(client):
    response = client.get(ENDPOINT)

    assert response.status_code == 200
    assert response.json() == {"status": "ok"}


def test_health_does_not_require_database(client, monkeypatch):
    """把 engine 換成一碰就爆的物件，/health 仍須是 200。"""

    class _Exploding:
        def connect(self):
            raise AssertionError("/health 不應該碰 DB")

    import http_server

    monkeypatch.setattr(http_server, "engine", _Exploding())

    response = client.get(ENDPOINT)

    assert response.status_code == 200
