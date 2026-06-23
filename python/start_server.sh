#!/usr/bin/env bash
# 透過 venv 啟動 FastAPI HTTP server（Method B）
# 使用方式：bash start_server.sh

set -e
cd "$(dirname "$0")"

if [ ! -d ".venv" ]; then
    echo "找不到虛擬環境，請先執行 bash setup.sh" >&2
    exit 1
fi

echo "啟動 FastAPI server（Method B - HTTP）on port 8001 ..."
.venv/bin/uvicorn http_server:app --port 8001 --reload
