#!/usr/bin/env bash
# Postgres migration 實跑驗證：起一個專用的 postgres，跑完整 goose up → 驗 schema →
# 分段 down 到 0，最後收乾淨。
#
# 為什麼需要這支（dev stack 就有 postgres）：dev 的 backend 啟動時只跑 **Up**，
# **Down 那一半沒有任何執行路徑**——017／018 的回滾鏈斷掉就是這樣一直沒被發現的。
# 要驗 Down 就得 down 到 0，那會清光 dev 的資料，是另一個明確動作，不該混進驗收流程。
# postgres 一直是三份 migration 裡唯一沒有自動化驗證的（mysql 有本腳本的 mysql 版，
# sqlite 走 migrate_sqlite_test.go），本腳本補上這個缺口。
#
# 用法：
#   scripts/test-postgres-migrations.sh              # 完整驗證（跑完會 down -v）
#   KEEP_UP=1 scripts/test-postgres-migrations.sh    # 跑完保留 postgres，方便連進去看 schema
#
# 可覆寫的環境變數：
#   MEM         編譯階段 container 的記憶體上限（預設 700m；會再經 mem-guard 依 host 實況下修）
#   CPUS        CPU 上限（預設 1）
#   GO_IMAGE    編譯用的 golang image（預設 golang:1.25-alpine）
#   RUN_IMAGE   執行測試執行檔用的 image（預設 alpine:3.20）
#   CACHE_DIR   build/module 快取根目錄（預設 ~/.cache/stock_trading）
#   GO_MEMLIMIT 單一 go 子行程的 heap 軟上限（預設 250MiB）
#   WAIT_SECONDS  等 postgres healthy 的上限秒數（預設 120）
#   KEEP_UP     設 1 則跑完不收 postgres（預設 0）
#   MEM_RESERVE_MB / MEM_STRICT / MEM_FORCE  見 scripts/lib/mem-guard.sh
#
# 設計重點（與 test-mysql-migrations.sh 相同的取捨，理由見該檔）：
#   - **兩階段錯開記憶體**：先在 postgres 還沒起來時把測試編成執行檔，編譯器退場後才起
#     postgres，再用輕量 container 跑編好的 binary。峰值是 max 而不是 sum（見 docs/development-workflow.md「container 上限的總和也要顧」）。
#   - 驗證邏輯在 Go 測試裡（backend/internal/database/migrate_postgres_test.go）。
#     migration 是 //go:embed 打包的，goose CLI 從磁碟讀到的是另一份檔案，驗了不算數。
#   - 測試 container 附掛到 compose 網路、用 service name 連線，不依賴 host port。
#   - 每次都從空 DB 開始：開頭與結尾都 `down -v`。這是**專用 project**（stock_trading_pgtest），
#     CLAUDE.md 禁止的是對 live/dev project 做這件事。
#   - CGO_ENABLED=0：編出來的是靜態執行檔，才能在 alpine 上直接跑。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="$REPO_ROOT/backend"
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.postgres.yml}"
CACHE_DIR="${CACHE_DIR:-$HOME/.cache/stock_trading}"
GO_IMAGE="${GO_IMAGE:-golang:1.25-alpine}"
RUN_IMAGE="${RUN_IMAGE:-alpine:3.20}"
MEM="${MEM:-700m}"
CPUS="${CPUS:-1}"
GO_MEMLIMIT="${GO_MEMLIMIT:-250MiB}"
WAIT_SECONDS="${WAIT_SECONDS:-120}"
KEEP_UP="${KEEP_UP:-0}"

# NETWORK_NAME 不寫死：compose 檔改了網路名字時，寫死的值只會讓測試以
# 「network not found」這種與 migration 無關的錯誤失敗。改成起來之後從 container 問。
NETWORK_NAME=""
POSTGRES_DSN="postgres://trading:trading@postgres:5432/trading?sslmode=disable"

cd "$REPO_ROOT"

# shellcheck source=lib/mem-guard.sh
. "$REPO_ROOT/scripts/lib/mem-guard.sh"

compose() {
  docker compose -f "$COMPOSE_FILE" "$@"
}

BIN_DIR=""
cleanup() {
  local status=$?
  if [ -n "$BIN_DIR" ] && [ -d "$BIN_DIR" ]; then
    rm -rf "$BIN_DIR"
  fi
  if [ "$KEEP_UP" = "1" ]; then
    echo "==> KEEP_UP=1，保留 postgres（手動收：docker compose -f $COMPOSE_FILE down -v）"
  else
    echo "==> 收掉 postgres（含 volume，下次從空 DB 開始）"
    compose down -v --remove-orphans >/dev/null 2>&1 || true
  fi
  exit "$status"
}
trap cleanup EXIT

# postgres 調瘦後比 MySQL 省，但仍要留給執行 binary 的 container 與 dockerd。
# 低於這個水位就不要開始——與其中途被 host OOM killer 砍掉呼叫端，不如一開始就講清楚。
MIN_AVAILABLE_MB="${MIN_AVAILABLE_MB:-500}"
if available_mb="$(mem_guard_available_mb)"; then
  echo "==> host available=${available_mb}MB"
  if [ "$available_mb" -lt "$MIN_AVAILABLE_MB" ]; then
    cat >&2 <<EOF
==> 記憶體不足，中止。
    host available=${available_mb}MB，低於 ${MIN_AVAILABLE_MB}MB。
    先關掉不需要的 container（docker ps 清場）再重跑。
    不要改大 postgres 的 mem_limit——那是 cgroup 上限不是預留，只會讓 host OOM killer 改砍呼叫端。
EOF
    exit 1
  fi
fi

# ── 階段 A：編譯測試執行檔（postgres 還沒起來，記憶體全給編譯器）────────
MEM="$(mem_guard_clamp "$MEM")"
MEMSWAP="${MEMSWAP:-$MEM}"

BIN_DIR="$(mktemp -d)"

echo "==> [1/4] 編譯 migration 測試執行檔（image=$GO_IMAGE mem=$MEM gomemlimit=$GO_MEMLIMIT）"
mkdir -p "$CACHE_DIR/gocache" "$CACHE_DIR/gomodcache"
docker run --rm \
  --user "$(id -u):$(id -g)" \
  --cpus="$CPUS" \
  --memory="$MEM" \
  --memory-swap="$MEMSWAP" \
  --pids-limit=200 \
  -e HOME=/tmp \
  -e GOMAXPROCS=1 \
  -e GOFLAGS=-p=1 \
  -e GOGC=off \
  -e GOMEMLIMIT="$GO_MEMLIMIT" \
  -e CGO_ENABLED=0 \
  -e GOCACHE=/gocache \
  -e GOMODCACHE=/gomodcache \
  -v "$BACKEND_DIR":/app \
  -w /app \
  -v "$CACHE_DIR/gocache":/gocache \
  -v "$CACHE_DIR/gomodcache":/gomodcache \
  -v "$BIN_DIR":/out \
  "$GO_IMAGE" \
  go test -c -o /out/migrate-postgres.test ./internal/database

# ── 階段 B：起 postgres（編譯器已退場）──────────────────────────────
echo "==> [2/4] 啟動 postgres：$COMPOSE_FILE"
compose down -v --remove-orphans >/dev/null 2>&1 || true
compose up -d

echo "==> 等 postgres healthy（上限 ${WAIT_SECONDS}s）"
deadline=$((SECONDS + WAIT_SECONDS))
while true; do
  cid="$(compose ps -q postgres)"
  if [ -n "$cid" ]; then
    health="$(docker inspect -f '{{.State.Health.Status}}' "$cid" 2>/dev/null || echo unknown)"
    [ "$health" = "healthy" ] && break
  fi
  if (( SECONDS >= deadline )); then
    echo "ERROR: postgres 在 ${WAIT_SECONDS}s 內沒有變成 healthy（目前 ${health:-unknown}）" >&2
    compose logs --tail=80 postgres || true
    exit 1
  fi
  sleep 3
done
echo "    ok: postgres healthy"

# 網路名字從 container 問，不寫死（compose 檔改名時不會變成難懂的 network not found）。
NETWORK_NAME="$(docker inspect -f '{{range $k, $v := .NetworkSettings.Networks}}{{$k}}{{end}}' "$cid")"
if [ -z "$NETWORK_NAME" ]; then
  echo "ERROR: 取不到 postgres container 的網路名稱" >&2
  exit 1
fi

# ── 階段 C：執行已編好的測試執行檔 ──────────────────────────────────
echo "==> [3/4] 跑 migration 驗證（up → 驗 schema → 分段 down 到 0 → NOT VALID 約束情境）"
docker run --rm \
  --network "$NETWORK_NAME" \
  --cpus="$CPUS" \
  --memory=128m \
  --memory-swap=128m \
  --pids-limit=100 \
  -e POSTGRES_MIGRATION_DSN="$POSTGRES_DSN" \
  -v "$BIN_DIR":/out:ro \
  "$RUN_IMAGE" \
  /out/migrate-postgres.test -test.v -test.run 'TestPostgresMigrations'

# ── 階段 D：收尾由 trap 處理 ────────────────────────────────────────
echo "==> [4/4] postgres migration 驗證通過"
