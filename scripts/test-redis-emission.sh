#!/usr/bin/env bash
# Redis emission 操作的整合測試：對**隔離的 dev Redis** 驗 Lua compare-and-delete
# 等「只有真實 Redis 才驗得到」的語意（見 docs/issue.md I-102）。
#
# 用法：
#   scripts/test-redis-emission.sh            # 起一個拋棄式 Redis，跑完即刪
#   REDIS_TEST_ADDR=host:port scripts/test-redis-emission.sh   # 用既有的（自負隔離責任）
#
# ⛔ **不要指向 live 的 redis-redis-1／trading-net。** 依 CLAUDE.md，驗收一律走 dev；
#    這支測試會寫入 key，指到 live 就是拿正式環境當測試資料。
#    2026-09-02 第一版的註解範例就是指向 live，已修正。
#
# 為什麼另開腳本而不是塞進 backend/scripts/test.sh：
#   那支跑在沒有網路相依的容器裡，是**每次都要跑**的基本驗證。把外部服務加進去
#   會讓整條驗證需要 Redis 才跑得動——I-102 的「測試接縫」那節明確要避免這件事。
#   所以整合測試預設 skip，只有這支腳本會設 REDIS_TEST_ADDR。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GO_IMAGE="${GO_IMAGE:-golang:1.25-alpine}"
CACHE_DIR="${CACHE_DIR:-$HOME/.cache/stock_trading}"
NET_NAME="stock-trading-redis-emission-test"
REDIS_NAME="stock-trading-redis-emission-test-redis"

cleanup() {
  if [[ -n "${STARTED_REDIS:-}" ]]; then
    docker rm -f "$REDIS_NAME" >/dev/null 2>&1 || true
    docker network rm "$NET_NAME" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

if [[ -n "${REDIS_TEST_ADDR:-}" ]]; then
  echo "==> 使用既有 Redis：$REDIS_TEST_ADDR（請自行確認它不是 live）"
  NETWORK_ARGS=()
else
  echo "==> 起一個拋棄式 Redis（跑完即刪，與 live 及 dev 都不共用資料卷）"
  docker network create "$NET_NAME" >/dev/null 2>&1 || true
  docker rm -f "$REDIS_NAME" >/dev/null 2>&1 || true
  # 不掛 volume：資料只活在容器裡，刪掉就沒了。
  docker run -d --rm --name "$REDIS_NAME" --network "$NET_NAME" \
    --memory 64m redis:7-alpine >/dev/null
  STARTED_REDIS=1
  REDIS_TEST_ADDR="$REDIS_NAME:6379"
  NETWORK_ARGS=(--network "$NET_NAME")

  echo "==> 等 Redis 就緒"
  for _ in $(seq 1 30); do
    if docker exec "$REDIS_NAME" redis-cli ping >/dev/null 2>&1; then break; fi
    sleep 1
  done
fi

mkdir -p "$CACHE_DIR/gocache" "$CACHE_DIR/gomodcache"

echo "==> go test ./internal/store/ -run TestLuaCompareAndDelete|TestFirstReadOnly"
exec docker run --rm \
  "${NETWORK_ARGS[@]}" \
  --user "$(id -u):$(id -g)" \
  --memory 500m --cpus=1 \
  -e HOME=/tmp -e GOMAXPROCS=1 -e GOFLAGS=-p=1 -e GOGC=off -e GOMEMLIMIT=250MiB \
  -e GOCACHE=/gocache -e GOMODCACHE=/gomodcache \
  -e REDIS_TEST_ADDR="$REDIS_TEST_ADDR" \
  -v "$REPO_ROOT/backend":/app -w /app \
  -v "$CACHE_DIR/gocache":/gocache \
  -v "$CACHE_DIR/gomodcache":/gomodcache \
  "$GO_IMAGE" go test ./internal/store/ -run 'TestLuaCompareAndDelete|TestFirstReadOnly' -count=1 -v
