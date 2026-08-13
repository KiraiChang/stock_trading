#!/usr/bin/env bash
# Event Timeline 對 live 資料的驗證（todo.md T-045 P1 驗收）。**唯讀，只跑 SELECT。**
#
# 用法：
#   scripts/verify-event-timeline.sh              # 預設驗 0050
#   SYMBOL=2330 scripts/verify-event-timeline.sh  # 換一檔
#
# 可覆寫的環境變數：
#   SYMBOL     要驗的標的（預設 0050，目前只有它有足夠的分析次數）
#   DB_DSN     直接指定 DSN；未指定時從 live backend container 讀
#   DSN_FROM   要讀 DSN 的 container（預設 stock_trading-backend-1）
#   NETWORK    要接的 docker 網路（預設 trading-net，live postgres 所在）
#   GO_IMAGE / MEM / CPUS / CACHE_DIR  同 backend/scripts/test.sh
#
# 設計重點：
#   - **為什麼需要對 live 跑**：摺疊邏輯的單元測試建立在「事件終結後不會再出現」這個
#     假設上，而實跑證明假設是錯的——狀態表會把 EXPIRED／RESOLVED 一直帶在後續快照裡，
#     導致每份快照各開一條垃圾鏈。這類「對資料長相的誤解」只有實跑抓得到。
#   - **DSN 從 live container 讀**，不寫進 repo：密碼不進版控，live 改密碼時腳本不用跟著改。
#   - 唯讀讀取不在 CLAUDE.md 的禁令之列（該條針對測試資料、migration 驗證與清空資料），
#     理由與 scripts/run-evaluation.sh 相同。
#   - 走 backend/scripts/test.sh 的 image 與 mem-guard 慣例；這台 host 只有 2GiB。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="$REPO_ROOT/backend"
SYMBOL="${SYMBOL:-0050}"
NETWORK="${NETWORK:-trading-net}"
DSN_FROM="${DSN_FROM:-stock_trading-backend-1}"
GO_IMAGE="${GO_IMAGE:-golang:1.25-alpine}"
CACHE_DIR="${CACHE_DIR:-$HOME/.cache/stock_trading}"
MEM="${MEM:-700m}"
CPUS="${CPUS:-1}"

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

mkdir -p "$CACHE_DIR/gocache" "$CACHE_DIR/gomodcache"

echo "==> Event Timeline live 驗證：symbol=$SYMBOL network=$NETWORK mem=$MEM（唯讀）"
exec docker run --rm \
  --user "$(id -u):$(id -g)" \
  --network "$NETWORK" \
  --cpus="$CPUS" \
  --memory="$MEM" \
  --memory-swap="$MEM" \
  --pids-limit=200 \
  -e HOME=/tmp \
  -e GOMAXPROCS=1 \
  -e GOFLAGS=-p=1 \
  -e GOGC=off \
  -e GOMEMLIMIT=250MiB \
  -e CGO_ENABLED=0 \
  -e GOCACHE=/gocache \
  -e GOMODCACHE=/gomodcache \
  -e SR_TIMELINE_LIVE_DSN="$DB_DSN" \
  -e SR_TIMELINE_SYMBOL="$SYMBOL" \
  -v "$BACKEND_DIR":/app \
  -w /app \
  -v "$CACHE_DIR/gocache":/gocache \
  -v "$CACHE_DIR/gomodcache":/gomodcache \
  "$GO_IMAGE" \
  go test ./internal/analysis/ -run TestEventTimelineAgainstLiveData -v
