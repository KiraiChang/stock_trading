"""`python/tests/` 共用設定與 fixture。

module 頂層只做「必須在任何測試模組 import 之前生效」的兩件事（`sys.path` 與 `LOG_DIR`）。

**刻意不在這裡 import http_server。** conftest 的 ImportError 對 pytest 是致命的：整個 session
直接放棄，連 `--continue-on-collection-errors` 都救不回來，而且錯誤只說「載入 conftest 失敗」，
跟 test_logging_setup 這種毫不相關的測試綁在一起。改由需要的測試模組自己 import 之後，錯誤會
明確歸屬到那三支檔案，加上 `--continue-on-collection-errors` 其餘測試就照跑。
（注意：預設仍會中斷 session——收集階段有 error 時 pytest 一律 Interrupted，這是 pytest 的行為，
不是這裡能改的。這個分法換到的是錯誤歸屬與「可以選擇繼續」，不是無條件隔離。）
"""
from __future__ import annotations

import copy
import os
import sys
import tempfile
from pathlib import Path

# 讓 `import http_server` / `import config` 找得到 python/ 底下的模組（比照 test_logging_setup.py）。
sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

# http_server 在 import 期會 configure_logging("python-server")，其預設 LOG_DIR 是
# /app/logs/python-server——在測試容器裡那正好是掛載進來的 repo 目錄。導到 tmp，
# 別讓跑測試這件事在 repo 留下檔案。用固定路徑而不是 mkdtemp()：mkdtemp 每個 session
# 都會留一個新目錄，在容器裡無所謂，但有人用 venv 直接在 host 跑 pytest 時會累積。
os.environ.setdefault("LOG_DIR", str(Path(tempfile.gettempdir()) / "stock-trading-test-logs"))

import pytest
from fastapi.testclient import TestClient

_FAKE_REPORT = {
    "run_id": "test-run-id",
    "schema_version": "sr_zone_evaluation_p0",
    "model_metrics": {"hold": None, "break": None},
}


def make_fake_report() -> dict:
    """每次回傳一份獨立的 report。

    deepcopy 而不是 dict()：淺拷貝會讓巢狀的 model_metrics 在所有測試間共用同一個物件，
    日後某條測試改了一個欄位就會害另一條莫名其妙紅掉。
    """
    return copy.deepcopy(_FAKE_REPORT)


@pytest.fixture
def client() -> TestClient:
    """**刻意不寫成 `with TestClient(...)`。**

    starlette 的 TestClient 只有被當成 context manager 使用時才會跑 lifespan，而 lifespan
    會呼叫 check_connection() 真的連 DB。不用 `with` ＝ 這些端點測試完全不需要 DB。
    要驗證啟動行為的測試請自己開 context manager（見 test_http_server_startup.py）。
    """
    import http_server

    return TestClient(http_server.app)


@pytest.fixture
def evaluate_payload():
    """/sr-scoring/evaluate 的最小合法 request body，可用 kwargs 覆寫任何欄位。"""

    def _build(**overrides) -> dict:
        return {"symbols": ["2330"], **overrides}

    return _build


class RunnerCalls:
    """記錄 evaluation / decision replay / 寫 DB 三個下游被呼叫時收到什麼。"""

    def __init__(self) -> None:
        self.evaluation: list[dict] = []
        self.replay: list[dict] = []
        self.write_db: list[tuple[dict, object]] = []


@pytest.fixture
def runners(monkeypatch) -> RunnerCalls:
    """把端點的三個下游換成記錄器。

    三個名字都是 http_server 的 module global（從 evaluation 模組 import 進來），
    端點以 module global 解析，所以 setattr 在 http_server 上即可生效。
    """
    import http_server

    calls = RunnerCalls()

    def fake_run_evaluation(**kwargs):
        calls.evaluation.append(kwargs)
        return make_fake_report()

    def fake_run_decision_replay(**kwargs):
        calls.replay.append(kwargs)
        return make_fake_report()

    def fake_write_evaluation_result(report, passed=None):
        calls.write_db.append((report, passed))

    monkeypatch.setattr(http_server, "run_evaluation", fake_run_evaluation)
    monkeypatch.setattr(http_server, "run_decision_replay", fake_run_decision_replay)
    monkeypatch.setattr(http_server, "write_evaluation_result", fake_write_evaluation_result)
    return calls
