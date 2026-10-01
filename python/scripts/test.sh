#!/usr/bin/env bash
# Python 驗證腳本：在 docker 內跑 pytest。
#
# 用法：
#   python/scripts/test.sh                                  # backtest/ 與 tests/
#   python/scripts/test.sh backtest/modular/sr_scoring/tests # 只跑指定目錄
#   python/scripts/test.sh -k event_engine backtest/         # 也可直接帶 pytest 參數
#   PY_ENV="SR_EXCURSION_BENCH=1" python/scripts/test.sh -s backtest/…/test_excursion_cost.py
#
# 可覆寫的環境變數：
#   MEM        container 記憶體上限（預設 700m；會再經 mem-guard 依 host 實況下修）
#   MEMSWAP    memory+swap 上限（預設等於 MEM，即關掉 container swap）
#   CPUS       CPU 上限（預設 1）
#   PY_IMAGE   測試用 image tag（預設 stock-trading-python-test:latest）
#   MEM_RESERVE_MB / MEM_STRICT / MEM_FORCE  見 scripts/lib/mem-guard.sh
#   SKIP_SHELL_TESTS=1  跳過 scripts/test-replay-args.sh 與 scripts/test-i074-stage2.sh（預設會先跑）
#   REPLAY_SMOKE=1      另外跑 scripts/smoke-replay-offline.sh（真的跑完 Stage 1／2；預設不跑）
#   PY_ENV     以空白分隔的 NAME=VALUE，原樣傳進 container。給「預設 skip、明確要求
#              才跑」的測試用（例如成本量測 SR_EXCURSION_BENCH=1）——這類測試不該
#              進常態回合，但也不該退化成一次性 docker 指令繞過腳本。
#
# 設計重點：
#   - 直接用 python/Dockerfile 建測試 image：裡面已裝好 requirements.txt（含 pytest）
#     與 lightgbm 需要的 libgomp1；靠 docker layer cache，requirements 沒改時幾乎不花時間。
#   - --user：以本機 uid/gid 執行，測試若產生檔案不會變成 root 所有。
#   - PYTHONDONTWRITEBYTECODE + -p no:cacheprovider：不在 repo 留 __pycache__ / .pytest_cache。
#   - db.py 已改成 lazy 連線，這段不需要任何 DB 環境變數即可跑。
#   - MEM 經 scripts/lib/mem-guard.sh 下修：--memory 高於 host 供得起的量時，host 層級的
#     OOM killer 會改砍呼叫端而不是 container（見 docs/development-workflow.md 的
#     「`MEM` 是上限，不是預留」）。
set -euo pipefail

PYTHON_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REPO_ROOT="$(cd "$PYTHON_DIR/.." && pwd)"
IMAGE="${PY_IMAGE:-stock-trading-python-test:latest}"
MEM="${MEM:-700m}"
CPUS="${CPUS:-1}"

# shellcheck source=../../scripts/lib/mem-guard.sh
. "$REPO_ROOT/scripts/lib/mem-guard.sh"
MEM="$(mem_guard_clamp "$MEM")"
MEMSWAP="${MEMSWAP:-$MEM}"

# 參數原樣轉交 pytest：用 "$@" 而不是把參數併成字串，否則帶空白的參數
# （例如 -k "a or b"）會被 word splitting 拆開，pytest 會把 "or" 當成路徑。
if [ "$#" -eq 0 ]; then
  set -- backtest/ tests/
fi

# 逐個展開成 -e 參數；用陣列而非字串，值含空白時才不會被 word splitting 拆開。
ENV_ARGS=()
for pair in ${PY_ENV:-}; do
  ENV_ARGS+=(-e "$pair")
done

# ── I-100：先跑 shell 層的參數所有權測試 ───────────────────────────────────
# 為什麼放這裡：python 測試容器只掛 python/（見下方 -v），讀不到 scripts/。
# 那一側驗「兩支官方腳本拒絕使用者傳入 provenance 參數」與「實際組出的 argv == 版控
# fixture」，python 這一側再用同一份 fixture 跑 CLI 衝突矩陣。
# ⚠️ 不掛進常態回合的話，它會變成沒人固定執行的手動項目。

# 端到端 smoke（真的用官方腳本跑完 Stage 1／2）——⚠️ 要 docker build ＋ 兩次容器啟動
# ＋ 兩個 git worktree，所以**預設不跑**，明確要求才跑。

if [ "${REPLAY_SMOKE:-0}" = "1" ]; then
  "$REPO_ROOT/scripts/smoke-replay-offline.sh"
fi

# ⚠️ **文件裡的原始碼行號引用**——改完程式碼忘了更新文件是**每次都會再發生**的漂移
# （2026-09-17 一次掃描就抓到 16 處指錯）。
# ⛔ **刻意放在 docker build 之前、也刻意不放進 `SKIP_SHELL_TESTS` 區塊**：
# 它⛔ 不需要 image，幾秒就跑完，文件錯誤應該**最快失敗**；而 `SKIP_SHELL_TESTS`
# 的語意是「跳過 replay 的 shell 測試」，⛔ 不該順手把文件檢查也關掉。
"$REPO_ROOT/scripts/test-doc-refs.sh"   # ⚠️ 守門工具自己的迴歸測試（初版曾 fail-open）
python3 "$REPO_ROOT/scripts/check-doc-refs.py"

echo "==> 建置測試 image：$IMAGE"
docker build -t "$IMAGE" "$PYTHON_DIR"

# ⚠️ **shell 測試排在 build 之後**（⛔ 不是之前）：comparator 與 Stage 1 的 argv 測試
# 需要 image 才跑得起來，放在前面的話**乾淨環境第一次執行會整段跳過**——
# 那正是「evidence 錯綁」這類問題沒有被測試抓到的原因之一。
if [ "${SKIP_SHELL_TESTS:-0}" != "1" ]; then
  # ⚠️ `IMAGE_REQUIRED=1`：image 剛建好，需要它的那幾段⛔ 不得靜默 skip。
  IMAGE_REQUIRED=1 PY_IMAGE="$IMAGE" "$REPO_ROOT/scripts/test-replay-args.sh"
  # I-074 Stage 2 ⑦b：supervisor／orchestrator／label shim 的整合測試 ＋ host unittest（⛔ 不碰 /run/lock、⛔ 真實容器）。
  "$REPO_ROOT/scripts/test-i074-stage2.sh"
fi


echo "==> pytest：$* image=$IMAGE mem=$MEM"
exec docker run --rm \
  --user "$(id -u):$(id -g)" \
  --cpus="$CPUS" \
  --memory="$MEM" \
  --memory-swap="$MEMSWAP" \
  --pids-limit=200 \
  -e HOME=/tmp \
  -e PYTHONDONTWRITEBYTECODE=1 \
  "${ENV_ARGS[@]+"${ENV_ARGS[@]}"}" \
  -v "$PYTHON_DIR":/app \
  -w /app \
  "$IMAGE" \
  pytest -p no:cacheprovider "$@"
