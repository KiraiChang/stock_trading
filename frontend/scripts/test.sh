#!/usr/bin/env bash
# Frontend 驗證腳本：在 docker 內依序跑 svelte-check（型別）→ vitest（單元）→ vite build。
# 任一步失敗即中止（fail-fast），型別錯誤或測試失敗不會被 build 蓋過。
#
# 用法：
#   frontend/scripts/test.sh             # check → test:unit → build（沿用現有 node_modules）
#   frontend/scripts/test.sh --install   # 先 npm ci 再跑上述三步
#   VITEST_ARGS="src/routes/Scheduler.test.ts" frontend/scripts/test.sh   # 開發迭代：只跑指定測試
#
# 可覆寫的環境變數：
#   MEM         container 記憶體上限（預設 1024m；vitest+jsdom 較吃資源，必要時上調）
#   CPUS        CPU 上限（預設 1）
#   NODE_IMAGE  使用的 node image（預設 node:20-alpine）
#   CACHE_DIR   npm 快取根目錄（預設 ~/.cache/stock_trading）
#   VITEST_ARGS 傳給 vitest 的額外參數（預設空）。設了就只跑 vitest（略過 check 與 build），
#               供開發迭代單一測試檔用；驗收仍要跑不帶此變數的完整三步。
#
# 設計重點：
#   - 掛載 repo root 而非 frontend/：vite.config.ts 的 outDir 是
#     ../backend/internal/ui/dist，只掛 frontend/ 會讓產物寫進 container 內憑空消失。
#   - --user：dist 產物以本機 uid/gid 寫入 backend/internal/ui/dist，不會變成 root 所有。
#   - 三層測試框架（svelte-check / vitest / @testing-library/svelte）的現況說明見
#     docs/development-workflow.md；vitest 用獨立的 vitest.config.ts，2GiB host 下以
#     single fork 限制併發避免 OOM。
set -euo pipefail

FRONTEND_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REPO_ROOT="$(cd "$FRONTEND_DIR/.." && pwd)"
CACHE_DIR="${CACHE_DIR:-$HOME/.cache/stock_trading}"
NODE_IMAGE="${NODE_IMAGE:-node:20-alpine}"
MEM="${MEM:-1024m}"
CPUS="${CPUS:-1}"
VITEST_ARGS="${VITEST_ARGS:-}"

INSTALL=0
[ "${1:-}" = "--install" ] && INSTALL=1
[ -d "$FRONTEND_DIR/node_modules" ] || INSTALL=1

mkdir -p "$CACHE_DIR/npm"

CMD="set -e"
[ "$INSTALL" = "1" ] && CMD="$CMD
npm ci"
if [ -n "$VITEST_ARGS" ]; then
  # 開發迭代模式：只跑 vitest，省掉 svelte-check 與 vite build 的等待。
  CMD="$CMD
npx vitest run $VITEST_ARGS"
  echo "==> frontend vitest only：args=$VITEST_ARGS install=$INSTALL image=$NODE_IMAGE mem=$MEM"
else
  CMD="$CMD
npm run check
npm run test:unit
npm run build"
  echo "==> frontend check+test+build：install=$INSTALL image=$NODE_IMAGE mem=$MEM"
fi

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
