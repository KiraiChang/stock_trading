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
#   DIST_AUTOSTAGE=1  build 後把 dist 的未追蹤檔案自動 git add（預設 0＝只檢查，
#                     有未納入版控的檔案就失敗）
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
# 2026-08-06 實測：heap 180/200/215 都 OOM、230 通過，故下限抓 225。
# 這個數字幾乎壓不下去——tsconfig 的 skipLibCheck 只省到 tsc program 那一段，
# svelte 的 language service 才是大宗；`--diagnostic-sources js,svelte`（去掉 css）
# 與 `--max-semi-space-size=2` 實測都沒有幫助。
SVELTE_CHECK_MIN_HEAP_MB="${SVELTE_CHECK_MIN_HEAP_MB:-225}"

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
  # node 撞到 heap 上限時是 SIGABRT，預設會把 1.3GB 的 core dump 丟進 repo（掛載的 workdir）。
  # 記憶體吃緊時 svelte-check 本來就可能 OOM，不該每次都附贈一顆巨大的垃圾檔。
  --ulimit core=0
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
  # svelte-check 是三步中最吃記憶體的一步，而且它的用量幾乎不隨旋鈕改變（2026-08-06 實測，
  # 數字與量法見 docs/development-workflow.md 的「frontend 三步的記憶體實測」）。heap 給不夠時
  # 它會先燒約 30 秒才吐一大段 V8 stack trace，光看訊息不容易意識到「這是 host 記憶體不夠」，
  # 所以先講清楚。這裡只警告不中止——實測下限是區間值，邊界上仍可能過。
  if [ "$NODE_HEAP_MB" -lt "$SVELTE_CHECK_MIN_HEAP_MB" ]; then
    echo >&2 "==> [warn] svelte-check 實測需要約 ${SVELTE_CHECK_MIN_HEAP_MB}MB heap，目前只有 ${NODE_HEAP_MB}MB，很可能 OOM。"
    echo >&2 "    heap 由 MEM 推導（MEM-100），MEM 由 mem-guard 依 host available 下修，"
    echo >&2 "    所以要解的是**釋放 host 記憶體**（需 available ≥ 約 $((SVELTE_CHECK_MIN_HEAP_MB + 100 + 150))MB），"
    echo >&2 "    不是調高 MEM——調高只會讓 host OOM killer 改砍呼叫端（見 docs/development-workflow.md）。"
    echo >&2 "    只想跑單元測試可用 VITEST_ARGS=... 略過這一步。"
  fi
  run_step "svelte-check" "npm run check"
  run_step "vitest" "npm run test:unit"
  run_step "vite build" "npm run build"

  # build 一定會產生新的 content hash 檔名，於是每次前端有變更都會出現
  # 「index.html 改了、但新 bundle 還是 untracked」的狀態。dist 依設計要進版控
  # （backend/internal/ui/ui.go 的 //go:embed all:dist），漏 add 會做出 index.html
  # 指向不存在檔案的 commit——SPA 整頁空白，而且所有測試都會過。
  #
  # **預設只檢查、不自動 add**：自動 add 會讓這道檢查永遠不失敗（等於沒有檢查），
  # 也會在使用者不知情下動到 git index。這一步失敗代表「工作區還不能 commit」，
  # 不是測試壞了——照訊息跑一次 git add 即可，每次前端變更只會遇到一次。
  # 明確要自動 add 時用 DIST_AUTOSTAGE=1。細節見 scripts/check-dist-assets.sh。
  if [ "${DIST_AUTOSTAGE:-0}" = "1" ]; then
    "$REPO_ROOT/scripts/check-dist-assets.sh" --fix
  else
    "$REPO_ROOT/scripts/check-dist-assets.sh"
  fi

  echo "==> frontend check + test + build 全部通過"
fi
