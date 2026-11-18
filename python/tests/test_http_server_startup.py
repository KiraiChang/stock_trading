"""http_server 的啟動路徑：DB 連線檢查必須在 lifespan，不在 module import 期。

見 docs/development-workflow.md §4「模組 import 不得有連線等副作用」。這組測試鎖的是
「檢查還在、但只在啟動時跑」——兩邊都要，只鎖其中一邊都會讓另一半悄悄壞掉。
"""
from __future__ import annotations

import importlib

import pytest
from fastapi.testclient import TestClient

import http_server


def _stub_backtest_startup(monkeypatch, calls: list) -> None:
    """lifespan 在連線檢查之後還會等 schema、收掉遺留的 running（I-134）；端點以外的測試把它們換成記錄器。"""
    monkeypatch.setattr(http_server, "wait_for_schema", lambda: calls.append("schema"))
    monkeypatch.setattr(http_server, "reap_interrupted", lambda owner: calls.append(("reap", owner)) or [])


def test_lifespan_runs_db_connection_check(monkeypatch):
    """服務啟動時仍要 fail-fast 地檢查 DB——這個檢查不能因為要好測就被整個拿掉。

    順序：連線檢查 → 等 schema（migration 077）→ 收掉上一個 server 程序留下的 running。
    """
    calls = []
    monkeypatch.setattr(http_server, "check_connection", lambda: calls.append("checked"))
    _stub_backtest_startup(monkeypatch, calls)

    with TestClient(http_server.app):
        pass

    assert calls == ["checked", "schema", ("reap", "server")]


def test_lifespan_schema_timeout_fails_startup_without_reap(monkeypatch):
    """等不到 schema 就讓啟動失敗（uvicorn 以非 0 結束、交給 compose 重啟），⛔ 不收尾。"""
    from backtest.db_writer import SchemaNotReady

    calls = []
    monkeypatch.setattr(http_server, "check_connection", lambda: None)

    def not_ready():
        raise SchemaNotReady("missing")

    monkeypatch.setattr(http_server, "wait_for_schema", not_ready)
    monkeypatch.setattr(http_server, "reap_interrupted", lambda owner: calls.append("reap"))

    with pytest.raises(SchemaNotReady):
        with TestClient(http_server.app):
            pass

    assert calls == []


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
