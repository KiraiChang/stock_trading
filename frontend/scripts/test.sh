#!/usr/bin/env bash
# Frontend 驗證腳本：依序跑 svelte-check（型別）→ vitest（單元）→ vite build。
# 任一步失敗即中止（fail-fast），型別錯誤或測試失敗不會被 build 蓋過。
#
# 用法：
#   frontend/scripts/test.sh             # check → test:unit → build（沿用現有 node_modules）
#   frontend/scripts/test.sh --install   # 先 npm ci 再跑上述三步
#   VITEST_ARGS="src/routes/Scheduler.test.ts" frontend/scripts/test.sh   # 開發迭代：只跑指定測試
#
# 可覆寫的環境變數：
#   MEM          每個 container 的記憶體上限（預設 440m；會再經 mem-guard 依 host 實況下修）
#   NODE_HEAP_MB node 的 old-space 上限（預設 320）
#   CPUS         CPU 上限（預設 1）
#   NODE_IMAGE   使用的 node image（預設 node:20-alpine）
#   CACHE_DIR    npm 快取根目錄（預設 ~/.cache/stock_trading）
#   VITEST_ARGS  傳給 vitest 的額外參數（預設空）。設了就只跑 vitest（略過 check 與 build），
#                供開發迭代單一測試檔用；驗收仍要跑不帶此變數的完整三步。
#   MEM_RESERVE_MB / MEM_STRICT / MEM_FORCE  見 scripts/lib/mem-guard.sh
#
# 設計重點：
#   - 掛載 repo root 而非 frontend/：vite.config.ts 的 outDir 是
#     ../backend/internal/ui/dist，只掛 frontend/ 會讓產物寫進 container 內憑空消失。
#   - --user：dist 產物以本機 uid/gid 寫入 backend/internal/ui/dist，不會變成 root 所有。
#   - **三步各起一個 container**：跑完就退出、記憶體立刻歸還 host，峰值是三者的 max 而非
#     sum，而且哪一步爆掉一眼可辨。這台 host 只有 2GiB 且 swap 常態用滿，差別是關鍵的。
#   - NODE_OPTIONS=--max-old-space-size：node 的預設 old-space 由可用記憶體推導，不明確
#     指定就會一路漲到接近 cgroup 上限才認真 GC。這是把「實際用量」壓下來的主要槓桿。
#   - --memory-swap 等於 --memory：關掉 container 的 swap。host swap 常態 100% 用滿，
#     放任 container 換頁只會拖垮整台機器，不如讓它乾脆撞 cgroup 上限。
#   - 記憶體上限一律經 scripts/lib/mem-guard.sh 下修，避免 host OOM killer 改砍呼叫端
#     （實際發生過兩次，見 docs/development-workflow.md 的「`MEM` 是上限，不是預留」）。
#   - 三層測試框架（svelte-check / vitest / @testing-library/svelte）的現況說明見
#     docs/development-workflow.md；vitest 用獨立的 vitest.config.ts，以 single fork 限制併發。
set -euo pipefail

FRONTEND_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REPO_ROOT="$(cd "$FRONTEND_DIR/.." && pwd)"
CACHE_DIR="${CACHE_DIR:-$HOME/.cache/stock_trading}"
NODE_IMAGE="${NODE_IMAGE:-node:20-alpine}"
MEM="${MEM:-440m}"
NODE_HEAP_MB="${NODE_HEAP_MB:-320}"
CPUS="${CPUS:-1}"
VITEST_ARGS="${VITEST_ARGS:-}"

# shellcheck source=../../scripts/lib/mem-guard.sh
. "$REPO_ROOT/scripts/lib/mem-guard.sh"
MEM="$(mem_guard_clamp "$MEM")"
MEMSWAP="${MEMSWAP:-$MEM}"

# node heap 要留空間給 container 內的其他用量（node 自身的 RSS 開銷、esbuild／svelte-check
# 子行程、page cache）。護欄把 MEM 下修後若沒跟著調，heap 上限會逼近甚至超過 container
# 上限，等於形同虛設——node 會先撞 cgroup 被殺，而不是提早 GC。
MEM_MB="$(mem_guard_to_mb "$MEM")"
if [ -n "$MEM_MB" ] && [ "$NODE_HEAP_MB" -gt $((MEM_MB - 100)) ]; then
  NODE_HEAP_MB=$((MEM_MB - 100))
  echo >&2 "==> [mem-guard] NODE_HEAP_MB 隨 MEM=$MEM 下修為 ${NODE_HEAP_MB}m"
fi

INSTALL=0
[ "${1:-}" = "--install" ] && INSTALL=1
[ -d "$FRONTEND_DIR/node_modules" ] || INSTALL=1

mkdir -p "$CACHE_DIR/npm"

DOCKER_ARGS=(
  --rm
  --user "$(id -u):$(id -g)"
  --cpus="$CPUS"
  --memory="$MEM"
  --memory-swap="$MEMSWAP"
  --pids-limit=200
  -e HOME=/tmp
  -e npm_config_cache=/npmcache
  -e NODE_OPTIONS="--max-old-space-size=$NODE_HEAP_MB"
  -v "$REPO_ROOT":/app
  -w /app/frontend
  -v "$CACHE_DIR/npm":/npmcache
)

# 每一步一個 container；set -e 讓任一步非零 exit 就中止整條驗證。
run_step() {
  local label="$1"
  shift
  echo "==> frontend $label（mem=$MEM node-heap=${NODE_HEAP_MB}m image=$NODE_IMAGE）"
  docker run "${DOCKER_ARGS[@]}" "$NODE_IMAGE" sh -c "$*"
}

if [ "$INSTALL" = "1" ]; then
  run_step "npm ci" "npm ci"
fi

if [ -n "$VITEST_ARGS" ]; then
  # 開發迭代模式：只跑 vitest，省掉 svelte-check 與 vite build 的等待。
  run_step "vitest（args=$VITEST_ARGS）" "npx vitest run $VITEST_ARGS"
else
  run_step "svelte-check" "npm run check"
  run_step "vitest" "npm run test:unit"
  run_step "vite build" "npm run build"
  echo "==> frontend check + test + build 全部通過"
fi
