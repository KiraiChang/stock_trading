#!/usr/bin/env bash
# Go 驗證腳本：在 docker 內依序執行 go vet → go test → build 編譯檢查。
#
# 用法：
#   backend/scripts/test.sh                        # 全部套件（./...）
#   backend/scripts/test.sh ./internal/market/...  # 只驗指定套件
#   TEST_FLAGS="-count=1 -v" backend/scripts/test.sh ./internal/market/...
#   RACE=1 backend/scripts/test.sh ./internal/scheduler/...  # 開 race detector
#
# 可覆寫的環境變數：
#   TEST_FLAGS  傳給 go test 的旗標（預設空）
#   RACE=1      開 race detector。**預設關閉**，原因見下方設計重點。
#   MEM         container 記憶體上限（預設 700m；會再經 mem-guard 依 host 實況下修）
#   MEMSWAP     memory+swap 上限（預設等於 MEM，即關掉 container swap）
#   CPUS        CPU 上限（預設 1）
#   GO_IMAGE    使用的 golang image（預設 golang:1.25-alpine）
#   CACHE_DIR   build/module 快取根目錄（預設 ~/.cache/stock_trading）
#   GO_MEMLIMIT 單一 go 子行程的 heap 軟上限（預設 250MiB，見下方設計重點）
#   MEM_RESERVE_MB / MEM_STRICT / MEM_FORCE  見 scripts/lib/mem-guard.sh
#
# 設計重點：
#   - GOMAXPROCS=1 + GOFLAGS=-p=1：本機只有 2GiB RAM，平行編譯會 OOM，必須序列化。
#   - GOGC=off + GOMEMLIMIT：序列化只限制「併發數」，不限制單一行程的 heap，container
#     的 --memory 也擋不住 host 層級的 OOM killer。`modernc.org/sqlite/lib`（C 轉譯的
#     巨大 generated package）的 vet／compile 子行程會直接吃爆記憶體，出現
#     `vet: signal: killed` 而讓整條驗證中止。比照 backend/Dockerfile builder stage
#     壓到 250MiB 才穩定。只影響編譯過程，不影響產出的執行檔與測試結果。
#   - MEM 經 scripts/lib/mem-guard.sh 下修：--memory 高於 host 供得起的量時，host 層級的
#     OOM killer 會改砍呼叫端而不是 container（見 docs/development-workflow.md 的
#     「`MEM` 是上限，不是預留」）。原本的 1800m 遠
#     高於這台 host 的可用量，而實際用量早就被 GOMEMLIMIT 壓住，等於一個永遠用不到卻會
#     害死呼叫端的數字，因此下修為 700m。
#   - --user：container 內以本機 uid/gid 執行，避免產出 root 所有的檔案。
#   - build 產物寫到 container 內的 /tmp，不落在 repo（曾誤產出 backend/server 並被 commit）。
#   - 快取放 repo 外的 CACHE_DIR，跨次重用且不會被誤加進版控。
#   - **RACE=1 為什麼不是預設**：`-race` 需要 cgo（`-race requires cgo`），而預設的
#     alpine image 沒有 C toolchain、CGO 也刻意關閉。開啟時會換成 debian 版 golang image
#     並放寬 GOMEMLIMIT——race detector 的 shadow memory 是原本用量的數倍，
#     250MiB 會直接把測試壓死。在這台 2GiB host 上**只適合針對單一套件跑**，
#     不要對 ./... 開。
#
#     沒有這個開關時，並發 bug 在本專案是抓不到的：2026-08-18 曾把一個
#     `map` 並發讀寫（Start() 寫、/scheduler/status 讀）交付出去，所有測試全綠——
#     那種 bug 會是 `fatal error: concurrent map read and map write`，不可 recover。
#
#   - **race 模式的記憶體是這台 host 的極限，數字都是實測（2026-08-18）**：
#     瓶頸是 `modernc.org/sqlite/lib`——`internal/store/sqlite.go` 的 blank import 把它拖進
#     幾乎每個套件的相依樹，race 版的它是全 repo 最貴的一次 compile。
#
#       * `GO_MEMLIMIT=600MiB`（本開關的初版預設）：單一 compile 長到 **anon-rss 545MB**，
#         host 記憶體見底，**OOM killer 砍的是呼叫端（claude 工作階段），不是 container**。
#         這就是把預設降到 400MiB 的原因——別再調回去。
#       * `GO_MEMLIMIT=400MiB`：compile 峰值仍有 **約 510MB**（這個 package 的 live heap
#         本身就超過軟上限，GOMEMLIMIT 只能逼 GC、壓不到 450MB）。
#       * `MEM` 被 mem-guard 下修到 **582m 時不夠**：
#         `modernc.org/sqlite/lib: compile: signal: killed`（container 撞 cgroup 上限，
#         這是**正確**的失敗方式——死的是 container，錯誤訊息讀得到）。**700m 才跑得完。**
#
#     推論：跑 race 前 host 的 `MemAvailable` 要有 **850MB 以上**（700m 上限 ＋ mem-guard
#     的 150MB 保留），否則 MEM 會被下修到跑不完。實務上得先停掉 dev stack 的
#     python 兩支（約 140MB）才騰得出來。
set -euo pipefail

BACKEND_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REPO_ROOT="$(cd "$BACKEND_DIR/.." && pwd)"
CACHE_DIR="${CACHE_DIR:-$HOME/.cache/stock_trading}"
RACE="${RACE:-0}"
if [ "$RACE" = "1" ]; then
  # alpine 沒有 C toolchain，race 需要 cgo，所以換 debian 版。
  GO_IMAGE="${GO_IMAGE:-golang:1.25}"
  # race detector 的 shadow memory 是數倍開銷，250MiB 會直接壓死測試。
  # 但**不能放到 600MiB**：實測會讓單一 compile 長到 545MB 而害 host OOM killer 砍掉
  # 呼叫端（見檔頭「race 模式的記憶體」）。400MiB 是實測跑得完又不會拖垮 host 的值。
  GO_MEMLIMIT="${GO_MEMLIMIT:-400MiB}"
  CGO="1"
else
  GO_IMAGE="${GO_IMAGE:-golang:1.25-alpine}"
  GO_MEMLIMIT="${GO_MEMLIMIT:-250MiB}"
  CGO="0"
fi
TEST_FLAGS="${TEST_FLAGS:-}"
MEM="${MEM:-700m}"
CPUS="${CPUS:-1}"

# shellcheck source=../../scripts/lib/mem-guard.sh
. "$REPO_ROOT/scripts/lib/mem-guard.sh"
MEM="$(mem_guard_clamp "$MEM")"
MEMSWAP="${MEMSWAP:-$MEM}"

PKGS="$*"
[ -n "$PKGS" ] || PKGS="./..."

mkdir -p "$CACHE_DIR/gocache" "$CACHE_DIR/gomodcache"

# vet/test 針對指定套件；build 檢查固定涵蓋所有 cmd 進入點，確保執行檔仍編得起來。
if [ "$RACE" = "1" ]; then
  # race 模式只跑 test：vet 與 build 在這個模式下沒有額外資訊，
  # 但 cgo 編譯很慢又吃記憶體，全跑會顯著拉長且更容易撞上限。
  CMD="set -e
go test -race $TEST_FLAGS $PKGS"
else
  CMD="set -e
go vet $PKGS
go test $TEST_FLAGS $PKGS
mkdir -p /tmp/gobuild
go build -o /tmp/gobuild/ ./cmd/..."
fi

DOCKER_ARGS=(
  --rm
  --user "$(id -u):$(id -g)"
  --cpus="$CPUS"
  --memory="$MEM"
  --memory-swap="$MEMSWAP"
  --pids-limit=200
  -e HOME=/tmp
  -e GOMAXPROCS=1
  -e GOFLAGS=-p=1
  -e GOGC=off
  -e GOMEMLIMIT="$GO_MEMLIMIT"
  -e CGO_ENABLED="$CGO"
  -e GOCACHE=/gocache
  -e GOMODCACHE=/gomodcache
  -v "$BACKEND_DIR":/app
  -w /app
  -v "$CACHE_DIR/gocache":/gocache
  -v "$CACHE_DIR/gomodcache":/gomodcache
)

if [ "$RACE" = "1" ]; then
  echo "==> go test -race：packages=$PKGS image=$GO_IMAGE mem=$MEM gomemlimit=$GO_MEMLIMIT"
  echo "    race detector 記憶體開銷是數倍，**只適合針對單一套件跑**，不要對 ./... 開。"
else
  echo "==> go vet/test/build：packages=$PKGS image=$GO_IMAGE mem=$MEM gomemlimit=$GO_MEMLIMIT"
fi
exec docker run "${DOCKER_ARGS[@]}" "$GO_IMAGE" sh -c "$CMD"
