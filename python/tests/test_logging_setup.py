import logging
import re
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from logging_setup import configure_logging


def test_configure_logging_writes_iso8601_file(tmp_path, monkeypatch):
    monkeypatch.setenv("LOG_DIR", str(tmp_path))
    monkeypatch.setenv("LOG_RETENTION_DAYS", "3")

    configure_logging("python-test")
    logging.getLogger("example").info("hello")
    for handler in logging.getLogger().handlers:
        handler.flush()

    content = (tmp_path / "python-test.log").read_text(encoding="utf-8")
    assert re.search(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z", content)
    assert "[python-test] INFO" in content
    assert "hello" in content


def test_configure_logging_replaces_existing_handlers(tmp_path, monkeypatch):
    monkeypatch.setenv("LOG_DIR", str(tmp_path))

    configure_logging("python-test")
    configure_logging("python-test")

    assert len(logging.getLogger().handlers) == 2
