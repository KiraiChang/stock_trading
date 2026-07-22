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
#
# 設計重點：
#   - GOMAXPROCS=1 + GOFLAGS=-p=1：本機只有 2GiB RAM，平行編譯會 OOM，必須序列化。
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
  -e CGO_ENABLED=0
  -e GOCACHE=/gocache
  -e GOMODCACHE=/gomodcache
  -v "$BACKEND_DIR":/app
  -w /app
  -v "$CACHE_DIR/gocache":/gocache
  -v "$CACHE_DIR/gomodcache":/gomodcache
)
[ -n "${MEMSWAP:-}" ] && DOCKER_ARGS+=(--memory-swap="$MEMSWAP")

echo "==> go vet/test/build：packages=$PKGS image=$GO_IMAGE mem=$MEM"
exec docker run "${DOCKER_ARGS[@]}" "$GO_IMAGE" sh -c "$CMD"
