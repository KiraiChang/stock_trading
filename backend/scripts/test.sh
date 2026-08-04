#!/usr/bin/env bash
# Go 驗證腳本：在 docker 內依序執行 go vet → go test → build 編譯檢查。
#
# 用法：
#   backend/scripts/test.sh                        # 全部套件（./...）
#   backend/scripts/test.sh ./internal/market/...  # 只驗指定套件
#   TEST_FLAGS="-count=1 -v" backend/scripts/test.sh ./internal/market/...
#
# 可覆寫的環境變數：
#   TEST_FLAGS  傳給 go test 的旗標（預設空）
#   MEM         container 記憶體上限（預設 1800m）
#   MEMSWAP     memory+swap 上限（預設不設，交由 docker 預設值）
#   CPUS        CPU 上限（預設 1）
#   GO_IMAGE    使用的 golang image（預設 golang:1.25-alpine）
#   CACHE_DIR   build/module 快取根目錄（預設 ~/.cache/stock_trading）
#   GO_MEMLIMIT 單一 go 子行程的 heap 軟上限（預設 250MiB，見下方設計重點）
#
# 設計重點：
#   - GOMAXPROCS=1 + GOFLAGS=-p=1：本機只有 2GiB RAM，平行編譯會 OOM，必須序列化。
#   - GOGC=off + GOMEMLIMIT：序列化只限制「併發數」，不限制單一行程的 heap，container
#     的 --memory 也擋不住 host 層級的 OOM killer。`modernc.org/sqlite/lib`（C 轉譯的
#     巨大 generated package）的 vet／compile 子行程會直接吃爆記憶體，出現
#     `vet: signal: killed` 而讓整條驗證中止。比照 backend/Dockerfile builder stage
#     壓到 250MiB 才穩定。只影響編譯過程，不影響產出的執行檔與測試結果。
#   - --user：container 內以本機 uid/gid 執行，避免產出 root 所有的檔案。
#   - build 產物寫到 container 內的 /tmp，不落在 repo（曾誤產出 backend/server 並被 commit）。
#   - 快取放 repo 外的 CACHE_DIR，跨次重用且不會被誤加進版控。
set -euo pipefail

BACKEND_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CACHE_DIR="${CACHE_DIR:-$HOME/.cache/stock_trading}"
GO_IMAGE="${GO_IMAGE:-golang:1.25-alpine}"
TEST_FLAGS="${TEST_FLAGS:-}"
MEM="${MEM:-1800m}"
CPUS="${CPUS:-1}"
GO_MEMLIMIT="${GO_MEMLIMIT:-250MiB}"

PKGS="$*"
[ -n "$PKGS" ] || PKGS="./..."

mkdir -p "$CACHE_DIR/gocache" "$CACHE_DIR/gomodcache"

# vet/test 針對指定套件；build 檢查固定涵蓋所有 cmd 進入點，確保執行檔仍編得起來。
CMD="set -e
go vet $PKGS
go test $TEST_FLAGS $PKGS
mkdir -p /tmp/gobuild
go build -o /tmp/gobuild/ ./cmd/..."

DOCKER_ARGS=(
  --rm
  --user "$(id -u):$(id -g)"
  --cpus="$CPUS"
  --memory="$MEM"
  --pids-limit=200
  -e HOME=/tmp
  -e GOMAXPROCS=1
  -e GOFLAGS=-p=1
  -e GOGC=off
  -e GOMEMLIMIT="$GO_MEMLIMIT"
  -e CGO_ENABLED=0
  -e GOCACHE=/gocache
  -e GOMODCACHE=/gomodcache
  -v "$BACKEND_DIR":/app
  -w /app
  -v "$CACHE_DIR/gocache":/gocache
  -v "$CACHE_DIR/gomodcache":/gomodcache
)
[ -n "${MEMSWAP:-}" ] && DOCKER_ARGS+=(--memory-swap="$MEMSWAP")

echo "==> go vet/test/build：packages=$PKGS image=$GO_IMAGE mem=$MEM gomemlimit=$GO_MEMLIMIT"
exec docker run "${DOCKER_ARGS[@]}" "$GO_IMAGE" sh -c "$CMD"
