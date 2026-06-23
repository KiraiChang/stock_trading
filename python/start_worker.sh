#!/usr/bin/env bash
# 透過 venv 啟動 DB polling worker（Method A）
# 使用方式：bash start_worker.sh

set -e
cd "$(dirname "$0")"

if [ ! -d ".venv" ]; then
    echo "找不到虛擬環境，請先執行 bash setup.sh" >&2
    exit 1
fi

echo "啟動 worker（Method A - DB polling）..."
.venv/bin/python worker.py
