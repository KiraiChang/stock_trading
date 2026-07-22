#!/usr/bin/env bash
# Python 驗證腳本：在 docker 內跑 pytest。
#
# 用法：
#   python/scripts/test.sh                                  # backtest/ 與 tests/
#   python/scripts/test.sh backtest/modular/sr_scoring/tests # 只跑指定目錄
#   python/scripts/test.sh -k event_engine backtest/         # 也可直接帶 pytest 參數
#
# 可覆寫的環境變數：
#   MEM        container 記憶體上限（預設 1024m）
#   CPUS       CPU 上限（預設 1）
#   PY_IMAGE   測試用 image tag（預設 stock-trading-python-test:latest）
#
# 設計重點：
#   - 直接用 python/Dockerfile 建測試 image：裡面已裝好 requirements.txt（含 pytest）
#     與 lightgbm 需要的 libgomp1；靠 docker layer cache，requirements 沒改時幾乎不花時間。
#   - --user：以本機 uid/gid 執行，測試若產生檔案不會變成 root 所有。
#   - PYTHONDONTWRITEBYTECODE + -p no:cacheprovider：不在 repo 留 __pycache__ / .pytest_cache。
#   - db.py 已改成 lazy 連線，這段不需要任何 DB 環境變數即可跑。
set -euo pipefail

PYTHON_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE="${PY_IMAGE:-stock-trading-python-test:latest}"
MEM="${MEM:-1024m}"
CPUS="${CPUS:-1}"

# 參數原樣轉交 pytest：用 "$@" 而不是把參數併成字串，否則帶空白的參數
# （例如 -k "a or b"）會被 word splitting 拆開，pytest 會把 "or" 當成路徑。
if [ "$#" -eq 0 ]; then
  set -- backtest/ tests/
fi

echo "==> 建置測試 image：$IMAGE"
docker build -t "$IMAGE" "$PYTHON_DIR"

echo "==> pytest：$* image=$IMAGE mem=$MEM"
exec docker run --rm \
  --user "$(id -u):$(id -g)" \
  --cpus="$CPUS" \
  --memory="$MEM" \
  --pids-limit=200 \
  -e HOME=/tmp \
  -e PYTHONDONTWRITEBYTECODE=1 \
  -v "$PYTHON_DIR":/app \
  -w /app \
  "$IMAGE" \
  pytest -p no:cacheprovider "$@"
