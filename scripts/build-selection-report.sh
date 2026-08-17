#!/usr/bin/env bash
# Evaluation Universe Selection Report（T-040 Step 3）。**唯讀，只跑 SELECT。**
#
# 產出選池分析報告：每檔的波動、流動性、bucket 與排除原因，外加多組流動性門檻的比較矩陣。
# 完整規格見 docs/evaluation-universe-selection-plan.md。
#
# 用法：
#   scripts/build-selection-report.sh                          # 印到 stdout
#   OUTPUT=/tmp/report.json scripts/build-selection-report.sh   # 落檔
#   MIN_AMOUNT=50000000 scripts/build-selection-report.sh       # 換流動性門檻
#
# 可覆寫的環境變數：
#   OUTPUT          輸出 JSON 路徑（container 內掛成 /out）；未指定則印到 stdout
#   MIN_AMOUNT      日均成交金額下限（預設 20000000，即 2000 萬）
#   MIN_TRADED_DAYS 近 60 個市場交易日內至少幾天有成交（預設 45）
#   SECURITY_TYPE   逗號分隔，預設 `股票,ETF`
#   KEEP_SYMBOLS    無條件保留的代號（逗號分隔），預設為 watchlist 基準的 11 檔。
#                   **重跑比對時一定要與定案那次相同**，否則 universe 會不一樣：
#                   watchlist 是靠這個參數才留住的，漏傳會讓 00830/00947/00981A/6243
#                   這類「靠保留才進池」的標的整批掉出，看起來像資料出問題（實測踩過）。
#   DB_DSN          SQLAlchemy DSN；未指定時從 live python-server container 讀
#   DSN_FROM        要讀 DSN 的 container（預設 stock_trading-python-server-1）
#   NETWORK         要接的 docker 網路（預設 trading-net）
#   MEM / CPUS / PY_IMAGE / MEM_RESERVE_MB …  同 python/scripts/test.sh
#
# 設計重點：
#   - **報告是研究輸出，不是 production config**。它不決定任何事，只提供決策依據；
#     門檻與最終清單由人看過報告後決定（見計畫書「風險」段的最後一列）。
#   - **唯讀**：只有 SELECT。CLAUDE.md 禁止的是拿 live 做測試資料、migration 驗證與
#     清空資料；唯讀讀取不在此列（理由同 scripts/run-evaluation.sh）。
#   - **DSN 從 live container 讀**，不寫進 repo：密碼不進版控，live 改密碼時不用改腳本。
#   - 走 python/scripts/test.sh 的 image 與 mem-guard 慣例；這台 host 只有 2GiB。
#     報告本身很輕（857 檔 × 60 根），但仍照規矩夾記憶體上限。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PYTHON_DIR="$REPO_ROOT/python"
IMAGE="${PY_IMAGE:-stock-trading-python-test:latest}"
MEM="${MEM:-500m}"
CPUS="${CPUS:-1}"
NETWORK="${NETWORK:-trading-net}"
DSN_FROM="${DSN_FROM:-stock_trading-python-server-1}"
OUTPUT="${OUTPUT:-}"
MIN_AMOUNT="${MIN_AMOUNT:-20000000}"
MIN_TRADED_DAYS="${MIN_TRADED_DAYS:-45}"
SECURITY_TYPE="${SECURITY_TYPE:-股票,ETF}"
# 定案 universe（universe-v2）用的 watchlist；預設寫死在這裡才能讓重跑可重現。
KEEP_SYMBOLS="${KEEP_SYMBOLS:-0050,00830,00947,00981A,2330,2399,2454,2478,3630,5490,6243}"

cd "$REPO_ROOT"
# shellcheck source=lib/mem-guard.sh
. "$REPO_ROOT/scripts/lib/mem-guard.sh"
MEM="$(mem_guard_clamp "$MEM")"

if [ -z "${DB_DSN:-}" ]; then
  DB_DSN="$(docker inspect "$DSN_FROM" -f '{{range .Config.Env}}{{println .}}{{end}}' 2>/dev/null \
    | sed -n 's/^DATABASE_DSN=//p' | head -1)"
  if [ -z "$DB_DSN" ]; then
    echo "ERROR: 從 $DSN_FROM 讀不到 DATABASE_DSN（live stack 沒起來？）。" >&2
    echo "       可用 DB_DSN=... 直接指定，或 DSN_FROM=<container> 換來源。" >&2
    exit 1
  fi
fi

if ! docker network inspect "$NETWORK" >/dev/null 2>&1; then
  echo "ERROR: 找不到 docker 網路 $NETWORK——live stack 沒起來？（NETWORK= 可覆寫）" >&2
  exit 1
fi

DOCKER_ARGS=(
  --rm
  --user "$(id -u):$(id -g)"
  --network "$NETWORK"
  --cpus="$CPUS"
  --memory="$MEM"
  --memory-swap="$MEM"
  --pids-limit=200
  -e HOME=/tmp
  -e PYTHONDONTWRITEBYTECODE=1
  -e DATABASE_DRIVER=postgres
  -e DATABASE_DSN="$DB_DSN"
  -v "$PYTHON_DIR":/app
  -w /app
)
CMD_ARGS=(
  python -m selection_report
  --min-amount "$MIN_AMOUNT"
  --min-traded-days "$MIN_TRADED_DAYS"
  --security-type "$SECURITY_TYPE"
  --keep-symbols "$KEEP_SYMBOLS"
)

if [ -n "$OUTPUT" ]; then
  OUT_DIR="$(cd "$(dirname "$OUTPUT")" && pwd)"
  DOCKER_ARGS+=(-v "$OUT_DIR":/out)
  CMD_ARGS+=(--output "/out/$(basename "$OUTPUT")")
fi

echo "==> 建置 image：$IMAGE"
docker build -t "$IMAGE" "$PYTHON_DIR" >/dev/null

echo "==> selection report（唯讀）：min_amount=$MIN_AMOUNT min_traded_days=$MIN_TRADED_DAYS mem=$MEM"
exec docker run "${DOCKER_ARGS[@]}" "$IMAGE" "${CMD_ARGS[@]}"
