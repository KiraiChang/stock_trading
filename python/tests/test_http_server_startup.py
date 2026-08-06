"""http_server 的啟動路徑：DB 連線檢查必須在 lifespan，不在 module import 期。

見 docs/development-workflow.md §4「模組 import 不得有連線等副作用」。這組測試鎖的是
「檢查還在、但只在啟動時跑」——兩邊都要，只鎖其中一邊都會讓另一半悄悄壞掉。
"""
from __future__ import annotations

import importlib

from fastapi.testclient import TestClient

import http_server


def test_lifespan_runs_db_connection_check(monkeypatch):
    """服務啟動時仍要 fail-fast 地檢查 DB——這個檢查不能因為要好測就被整個拿掉。"""
    calls = []
    monkeypatch.setattr(http_server, "check_connection", lambda: calls.append("checked"))

    with TestClient(http_server.app):
        pass

    assert calls == ["checked"]


def test_client_without_lifespan_does_not_touch_db(monkeypatch):
    """不進 lifespan 的 TestClient 完全不碰 DB——端點測試因此不需要任何 DB 環境。"""
    calls = []
    monkeypatch.setattr(http_server, "check_connection", lambda: calls.append("checked"))

    response = TestClient(http_server.app).get("/health")

    assert response.status_code == 200
    assert calls == []


def test_importing_module_does_not_check_connection(monkeypatch):
    """import http_server 不得有連線副作用。

    用 reload 重跑一次整個 module body：哪天有人把 check_connection() 放回頂層，這條會紅。
    reload 會重建 http_server.app 並把 check_connection 綁到這裡的假物件上，所以最後要
    先 undo 再 reload 一次，把 module 還原成綁著真正 check_connection 的狀態。
    """
    import db

    calls = []
    monkeypatch.setattr(db, "check_connection", lambda: calls.append("checked"))
    try:
        importlib.reload(http_server)
        assert calls == []
    finally:
        monkeypatch.undo()
        importlib.reload(http_server)
