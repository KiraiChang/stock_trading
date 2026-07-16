from __future__ import annotations

import logging
import os
import sys
import time
from logging.handlers import TimedRotatingFileHandler

DEFAULT_RETENTION_DAYS = 14


class ISO8601Formatter(logging.Formatter):
    converter = time.gmtime

    def formatTime(self, record: logging.LogRecord, datefmt: str | None = None) -> str:
        return time.strftime("%Y-%m-%dT%H:%M:%SZ", self.converter(record.created))


class ServiceFilter(logging.Filter):
    def __init__(self, service: str) -> None:
        super().__init__()
        self.service = service

    def filter(self, record: logging.LogRecord) -> bool:
        record.service = self.service
        return True


def configure_logging(service: str, default_log_dir: str | None = None) -> None:
    retention_days = _env_int("LOG_RETENTION_DAYS", DEFAULT_RETENTION_DAYS)

    formatter = ISO8601Formatter("%(asctime)s [%(service)s] %(levelname)-8s %(name)s — %(message)s")
    service_filter = ServiceFilter(service)

    stdout_handler = logging.StreamHandler(sys.stdout)
    stdout_handler.setFormatter(formatter)
    stdout_handler.addFilter(service_filter)

    root = logging.getLogger()
    for handler in root.handlers[:]:
        handler.close()
        root.removeHandler(handler)
    root.setLevel(logging.INFO)
    root.addHandler(stdout_handler)

    # 檔案 log 為 best-effort：log 目錄不可寫（例如本機非 Docker、未設 LOG_DIR 時預設的
    # /app/logs/... 無法建立）時只警告並退回 stdout-only，不讓 logging 設定在 import 期整個崩掉。
    log_dir = os.getenv("LOG_DIR") or default_log_dir or f"/app/logs/{service}"
    try:
        os.makedirs(log_dir, exist_ok=True)
        # when="midnight" 的預設 suffix 已是 "%Y-%m-%d" 且與 extMatch 成對；不另外覆寫 suffix，
        # 避免日後改 when 時 extMatch 沒同步、造成 backupCount 刪檔靜默失效。
        file_handler = TimedRotatingFileHandler(
            os.path.join(log_dir, f"{service}.log"),
            when="midnight",
            backupCount=retention_days,
            utc=True,
            encoding="utf-8",
        )
        file_handler.setFormatter(formatter)
        file_handler.addFilter(service_filter)
        root.addHandler(file_handler)
    except OSError as exc:
        root.warning("file logging disabled (log dir %s not writable): %s", log_dir, exc)


def _env_int(key: str, fallback: int) -> int:
    try:
        value = int(os.getenv(key, ""))
    except ValueError:
        return fallback
    return value if value > 0 else fallback
