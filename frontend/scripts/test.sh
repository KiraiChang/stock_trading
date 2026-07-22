#!/usr/bin/env bash
# Frontend 驗證腳本：在 docker 內跑 vite build（含 Svelte/TS 編譯檢查）。
#
# 用法：
#   frontend/scripts/test.sh             # 直接 build（沿用現有 node_modules）
#   frontend/scripts/test.sh --install   # 先 npm ci 再 build
#
# 可覆寫的環境變數：
#   MEM         container 記憶體上限（預設 1024m）
#   CPUS        CPU 上限（預設 1）
#   NODE_IMAGE  使用的 node image（預設 node:20-alpine）
#   CACHE_DIR   npm 快取根目錄（預設 ~/.cache/stock_trading）
#
# 設計重點：
#   - 掛載 repo root 而非 frontend/：vite.config.ts 的 outDir 是
#     ../backend/internal/ui/dist，只掛 frontend/ 會讓產物寫進 container 內憑空消失。
#   - --user：dist 產物以本機 uid/gid 寫入 backend/internal/ui/dist，不會變成 root 所有。
#   - 目前前端沒有測試框架（無 vitest / svelte-check），build 就是唯一的自動化檢查，
#     見 docs/todo.md 的「frontend 測試框架」項目。
set -euo pipefail

FRONTEND_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REPO_ROOT="$(cd "$FRONTEND_DIR/.." && pwd)"
CACHE_DIR="${CACHE_DIR:-$HOME/.cache/stock_trading}"
NODE_IMAGE="${NODE_IMAGE:-node:20-alpine}"
MEM="${MEM:-1024m}"
CPUS="${CPUS:-1}"

INSTALL=0
[ "${1:-}" = "--install" ] && INSTALL=1
[ -d "$FRONTEND_DIR/node_modules" ] || INSTALL=1

mkdir -p "$CACHE_DIR/npm"

CMD="set -e"
[ "$INSTALL" = "1" ] && CMD="$CMD
npm ci"
CMD="$CMD
npm run build"

echo "==> frontend build：install=$INSTALL image=$NODE_IMAGE mem=$MEM"
exec docker run --rm \
  --user "$(id -u):$(id -g)" \
  --cpus="$CPUS" \
  --memory="$MEM" \
  --pids-limit=200 \
  -e HOME=/tmp \
  -e npm_config_cache=/npmcache \
  -v "$REPO_ROOT":/app \
  -w /app/frontend \
  -v "$CACHE_DIR/npm":/npmcache \
  "$NODE_IMAGE" \
  sh -c "$CMD"
