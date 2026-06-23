#!/usr/bin/env bash
# 建立 Python 虛擬環境並安裝依賴
# 使用方式：bash setup.sh

set -e
cd "$(dirname "$0")"

echo ""
echo "=== Trading Python Setup ==="
echo "目錄：$(pwd)"
echo ""

# 確認 python3 可用
echo "[1/3] Python 版本：$(python3 --version)"

# 建立 venv
if [ -d ".venv" ]; then
    echo "[2/3] 虛擬環境已存在，跳過建立"
else
    echo "[2/3] 建立虛擬環境 .venv ..."
    python3 -m venv .venv
    echo "      完成"
fi

# 安裝套件
echo "[3/3] 升級 pip ..."
.venv/bin/pip install --upgrade pip

echo "[3/3] 安裝 requirements.txt ..."
.venv/bin/pip install -r requirements.txt

echo ""
echo "=== 設定完成 ==="
echo "啟動服務請執行："
echo "  bash start_worker.sh      # Method A：DB polling worker"
echo "  bash start_server.sh      # Method B：FastAPI HTTP server"
echo ""
