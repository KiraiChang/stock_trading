#!/usr/bin/env bash
# I-140 重播工具（backend/internal/bouncereplay）的 PostgreSQL 整合測試：起一個拋棄式的 postgres（跟
# test-postgres-migrations.sh 共用 docker-compose.postgres.yml 的服務定義），在裡面建 i140_t_* 三個 database，
# 跑 TestPostgresBounceReplay（來源庫的讀取 SQL、外部訊號照原樣注入、NUMERIC 精度、完整重播），最後收乾淨。
#
# ⛔ 不連 dev 或 live 的資料庫；真正的重播用隔離的 dev PostgreSQL（docs/plans/i140-intraday-bounce-dedup.md 第 7.3 節）。
#
# **每次執行都用自己的資源**（複審 2026-11-24）：project、網路、volume 的名稱都帶這一次的執行序號，⛔ 不發布 port；
# 收尾只清理這一次建立的資源——⛔ 不會碰到 migration 驗證（stock_trading_pgtest）或其他同時在跑的測試。
#
# 用法：scripts/test-bouncereplay-pg.sh（可覆寫的環境變數與記憶體的做法同 test-postgres-migrations.sh；KEEP_UP=1 保留這一次的 postgres）
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# container 以哪個 uid 執行：rootless 時不帶 --user（I-133，見 scripts/lib/docker-user.sh）。
# ⚠️ 這裡只載入函式；docker_user_args（會問 docker info）放在參數檢查之後、啟動 container 之前。
. "$REPO_ROOT/scripts/lib/docker-user.sh"
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

RUN_ID="$(date +%s)-$$"
PROJECT="stock_trading_brtest_${RUN_ID//-/_}"
OVERRIDE=""
STARTED=0

compose() {
  docker compose -p "$PROJECT" -f "$COMPOSE_FILE" -f "$OVERRIDE" "$@"
}

BIN_DIR=""
cleanup() {
  local status=$?
  if [ "$STARTED" = "1" ]; then
    if [ "$KEEP_UP" = "1" ]; then
      echo "==> KEEP_UP=1，保留這一次的 postgres（project $PROJECT；override 在 $OVERRIDE，收的時候要一起帶）"
    else
      echo "==> 收掉這一次的 postgres（project $PROJECT，只刪它自己的網路與 volume）"
      compose down -v --remove-orphans >/dev/null 2>&1 || true
    fi
  fi
  if [ "$KEEP_UP" != "1" ] && [ -n "$BIN_DIR" ] && [ -d "$BIN_DIR" ]; then
    rm -rf "$BIN_DIR"
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
# 這一次專用的網路與 volume 名稱（compose 檔裡寫死了 name，⛔ 不改它就會跟 migration 驗證共用），並撤掉 port。
OVERRIDE="$BIN_DIR/brtest.override.yml"
printf '%s\n' \
  'services:' \
  '  postgres:' \
  '    ports: !reset []' \
  'networks:' \
  '  pgtest_net:' \
  "    name: ${PROJECT}_net" \
  'volumes:' \
  '  pgtest_data:' \
  "    name: ${PROJECT}_data" > "$OVERRIDE"

echo "==> [1/4] 編譯重播工具的測試執行檔（image=$GO_IMAGE mem=$MEM gomemlimit=$GO_MEMLIMIT）"
mkdir -p "$CACHE_DIR/gocache" "$CACHE_DIR/gomodcache"
docker_user_args   # 問 docker info；⚠️ 排在參數檢查之後（I-133 review）
docker run --rm \
  ${DOCKER_USER_ARGS[@]+"${DOCKER_USER_ARGS[@]}"} \
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
  go test -c -o /out/bouncereplay-pg.test ./internal/bouncereplay

# ── 階段 B：起 postgres（編譯器已退場）──────────────────────────────
echo "==> [2/4] 啟動這一次專用的 postgres（project $PROJECT）"
STARTED=1
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
echo "==> [3/4] 跑重播工具的 PostgreSQL 整合測試"
docker run --rm \
  --network "$NETWORK_NAME" \
  --cpus="$CPUS" \
  --memory=256m \
  --memory-swap=256m \
  --pids-limit=100 \
  -e BOUNCEREPLAY_PG_DSN="$POSTGRES_DSN" \
  -v "$BIN_DIR":/out:ro \
  "$RUN_IMAGE" \
  /out/bouncereplay-pg.test -test.v -test.run 'TestPostgresBounceReplay'
echo "==> [4/4] 重播工具的 PostgreSQL 整合測試通過"
