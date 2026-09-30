#!/usr/bin/env bash
# I-100 Stage 1／2：**完全離線**地從凍結 bundle 跑 decision replay。
#
# 用法（`--bundle` 與 `--output-dir` 一律用 **host 絕對路徑**，容器內掛在同一個路徑上）：
#
#   # Stage 1（掃描）：用 **after** 版本跑全候選，產出候選 manifest 與完整逐列輸出
#   scripts/run-replay-offline.sh --bundle "$PWD/python/baselines/<bundle_id>" \
#       --output-dir /tmp/stage1 --before-ref ecbc141^
#
#   # Stage 2（比對）：用 **before** 版本跑同一個範圍，再與 Stage 1 的輸出逐列對照
#   scripts/run-replay-offline.sh --bundle "$PWD/python/baselines/<bundle_id>" \
#       --output-dir /tmp/stage2 --before-ref ecbc141^ \
#       --after-artifact /tmp/stage1/after_artifact.json \
#       --cohort-manifest /tmp/stage1/cohort_manifest.json
#
# ⚠️ **哪一個 stage 跑哪一份程式碼，是這支腳本最容易搞反的地方**：
#
#   | stage | 跑的版本 | worktree 建在 |
#   |---|---|---|
#   | 1 | **after**（要驗的新版） | `--after-ref`（預設 HEAD） |
#   | 2 | **before**（對照組） | `--before-ref` |
#
#   把 Stage 1 跑在 before 版上，產出的 `after_artifact.json` 裡裝的其實是 before 的結果，
#   而且舊版本根本沒有 bundle CLI——這種錯誤不會在任何單元測試裡出現，
#   所以 `scripts/test-replay-args.sh` 用 `REPLAY_DRY_RUN=1` 直接斷言掛載與 worktree。
#
# 可覆寫的環境變數：
#   AFTER_REF      Stage 1 要跑的版本（預設 HEAD）
#   TOOLING_PATCH  要套進 worktree 的 patch 檔（本階段的 tooling 變更；可空）
#   COUNTERFACTUAL_PATCH  ⚠️ I-074 Stage 2 反事實模式的語意 patch（**只**在帶 `--i074-counterfactual` 時
#                  必須非空；沒帶 flag 時⛔ 必須是空的）。固定套用順序 counterfactual → tooling。
#   REPLAY_DRY_RUN=1  只印出最終的 docker argv，不真的執行（給測試與人工核對用）
#   I074_STAGE     I-074 正式流程要用哪一份 run identity（封閉列舉 1／2，預設 1）。
#                  ⚠️ 只在 I-074 正式流程有意義；Stage 2 的環境見證趟（after'）用 2——
#                  見 issue.md I-074 Stage 2 計畫書「二、⑤」。⛔ 它不是路徑，⛔ 不開放任意覆寫。
#   MEM / CPUS / PY_IMAGE  同 python/scripts/test.sh
#
# 設計重點：
#   - **結構性離線**：`--network none`、**不注入任何 DB 環境變數**、程式碼唯讀掛載，
#     只有 `--output-dir` 可寫。⛔ 也不注入 `--model-path`——模型是 bundle 的一部分。
#   - **bundle 與 Stage 1 的 artifact 是「現在的」檔案，⛔ 不在版本 worktree 裡**：
#     bundle 是之後才進版控的，舊的 before worktree 不會有它。所以兩者各自唯讀掛載，
#     且掛在**與 host 相同的絕對路徑**上——這樣參數不用改寫，也就不會改寫錯。
#   - ⛔ `--image-digest`／`--base-commit`／`--tooling-patch-sha256`／`--source-root`／
#     `--runner-sha256` 只能由本腳本注入（見 scripts/lib/replay-args.sh）。
#
# ⚠️ `base_commit` 的取得順序是**固定的**，否則有 TOCTOU（見 replay_args_prepare_worktree）：
#   ① ref → immutable OID  ② 用該 OID 建 detached worktree  ③ 讀 worktree HEAD  ④ 斷言相同
#
# ⚠️ `tooling_patch_sha256` 是 base 到**實際套用後的 tree** 的 **canonical diff** hash（唯一的合成函式
# `replay_args_compose()`，見 scripts/lib/replay-args.sh），⛔ 不是「傳進來那個 patch 檔的 hash」。
#
# ⚠️ **I-074 Stage 2 反事實模式**（`--i074-counterfactual`，⑦a）：
#   - 兩份 patch 在一開始各讀一次、**凍結**到私有目錄（空的 TOOLING_PATCH 建 0-byte 副本），之後只用副本；
#   - 依固定順序 counterfactual → tooling 套用，得三個增量 canonical SHA 與語意 SHA（四檔集合由合成函式驗）；
#   - **docker 之前**斷言兩份 patch 的 raw bytes SHA 各自 ＝ 增量 canonical SHA（⛔ 不讓非 canonical 的
#     patch 跑完約 180 分鐘才在 finalize 被擋）；
#   - 注入 `--counterfactual-patch-sha256`；flag 納入 `I074_MODE`（⛔ 不自動 pin、必須有 identity）。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PYTHON_DIR="$REPO_ROOT/python"

# shellcheck source=lib/replay-args.sh
. "$REPO_ROOT/scripts/lib/replay-args.sh"
replay_args_reject_injected "$@"
# ⚠️ 必須排在任何「取值」之前：腳本取第一個、argparse 取最後一個，重複會讓實際執行的
# 版本與報告宣稱的版本不同（見 replay_args_reject_duplicates）。
replay_args_reject_duplicates "$@"

# ⚠️ **I-074 正式流程的偵測**：帶這兩個 flag 之一即視為正式流程——
# 那時⛔ 不自動 pin，**必須**預先提供 `REPLAY_IMAGE_ID`（三趟要跑在同一個 image 上）。
I074_MODE=0
CF_FLAG=0
for _arg in "$@"; do
  case "$_arg" in
    --i074-preflight|--i074-capacity-probe) I074_MODE=1 ;;
    --i074-counterfactual) I074_MODE=1; CF_FLAG=1 ;;
    --i074-counterfactual=*)
      echo "ERROR: --i074-counterfactual 是旗標，⛔ 不接受 =value 的寫法。" >&2; exit 1 ;;
  esac
done

I074_STAGE="${I074_STAGE:-1}"
case "$I074_STAGE" in
  1|2) ;;
  *) echo "ERROR: I074_STAGE 只接受 1 或 2（封閉列舉），實際 '$I074_STAGE'。" >&2; exit 1 ;;
esac
if [ "$I074_STAGE" != "1" ] && [ "$I074_MODE" = "0" ]; then
  echo "ERROR: I074_STAGE=$I074_STAGE 只在 I-074 正式流程（--i074-preflight／--i074-capacity-probe／--i074-counterfactual）有意義。" >&2
  exit 1
fi

IMAGE="${PY_IMAGE:-stock-trading-python-test:latest}"
MEM="${MEM:-700m}"
CPUS="${CPUS:-1}"
AFTER_REF="${AFTER_REF:-HEAD}"
TOOLING_PATCH="${TOOLING_PATCH:-}"
COUNTERFACTUAL_PATCH="${COUNTERFACTUAL_PATCH:-}"

STAGE="$(replay_args_offline_stage "$@")"
if [ "$STAGE" = "3" ]; then
  echo "ERROR: --after-artifact 與 --cohort-manifest 必須成對出現："                 >&2
  echo "       兩個都沒給是 Stage 1，兩個都給是 Stage 2，只給其中一個是不完整的模式。" >&2
  exit 1
fi

# ⚠️ **反事實模式的 truth table**（Stage 2 計畫書「二、④」）——一律在建 worktree、跑 docker 之前：
#   flag ＋ patch 空 → 中止（沒有 patch 的反事實沒有意義）；無 flag ＋ patch 非空 → 中止（⛔ 不得在一般路徑
#   偷偷帶語意 patch）；flag 只限 Stage 2；flag 必須搭 Stage 2 的 identity（F2-a，明示而⛔ 不推斷）。
if [ "$CF_FLAG" = "1" ]; then
  [ -n "$COUNTERFACTUAL_PATCH" ] || {
    echo "ERROR: --i074-counterfactual 需要非空的 COUNTERFACTUAL_PATCH——沒有 patch 的反事實沒有意義。" >&2; exit 1; }
  [ "$STAGE" = "2" ] || {
    echo "ERROR: --i074-counterfactual 只限 Stage 2（需要成對的 --after-artifact 與 --cohort-manifest）。" >&2; exit 1; }
  [ "$I074_STAGE" = "2" ] || {
    echo "ERROR: --i074-counterfactual 必須搭配 I074_STAGE=2（before 要用唯一固定的 Stage 2 identity）。" >&2; exit 1; }
elif [ -n "$COUNTERFACTUAL_PATCH" ]; then
  echo "ERROR: 沒有 --i074-counterfactual 卻給了 COUNTERFACTUAL_PATCH——⛔ 不得在一般路徑帶語意 patch。" >&2
  exit 1
fi

BUNDLE="$(replay_args_value_of --bundle "$@" || true)"
OUTPUT_DIR="$(replay_args_value_of --output-dir "$@" || true)"
BEFORE_REF="$(replay_args_value_of --before-ref "$@" || true)"
AFTER_ARTIFACT="$(replay_args_value_of --after-artifact "$@" || true)"
COHORT_MANIFEST="$(replay_args_value_of --cohort-manifest "$@" || true)"

for required in BUNDLE:--bundle OUTPUT_DIR:--output-dir BEFORE_REF:--before-ref; do
  name="${required%%:*}"
  flag="${required#*:}"
  if [ -z "${!name}" ]; then
    echo "ERROR: 需要 $flag。" >&2
    exit 1
  fi
done

# Stage 1 跑 after、Stage 2 跑 before——⚠️ 這一行是本腳本的核心語意。
if [ "$STAGE" = "1" ]; then
  SOURCE_REF="$AFTER_REF"
else
  SOURCE_REF="$BEFORE_REF"
fi

if [ "${REPLAY_ARGS_SELFTEST:-0}" = "1" ]; then
  echo "==> replay-args selftest：參數驗證通過（stage=$STAGE source_ref=$SOURCE_REF）"
  exit 0
fi

# shellcheck source=lib/mem-guard.sh
. "$REPO_ROOT/scripts/lib/mem-guard.sh"
MEM="$(mem_guard_clamp "$MEM")"
MEMSWAP="${MEMSWAP:-$MEM}"

if [ ! -d "$BUNDLE" ]; then
  echo "ERROR: bundle 目錄不存在：$BUNDLE（要用 host 絕對路徑）" >&2
  exit 1
fi
BUNDLE_ABS="$(cd "$BUNDLE" && pwd)"
mkdir -p "$OUTPUT_DIR"
OUT_ABS="$(cd "$OUTPUT_DIR" && pwd)"

MOUNTS=(-v "$BUNDLE_ABS":"$BUNDLE_ABS":ro -v "$OUT_ABS":"$OUT_ABS")

# ── I-074：**Docker 之前**的 run identity 守門 ─────────────────────────────
#
# ⚠️ runner 是四類角色中**唯一手上有 bundle path** 的，所以它的 `bundle_id` 比對
# 可以在 Docker 前完成——validator 自己呼叫 `load_bundle()`（三方相等 ＋ 完整 hash 驗證）
# 取得 ID 再比對，⛔ shell 不解析 identity，也就沒有雙真相源與兩次讀取的 TOCTOU。
if [ "$I074_MODE" = "1" ]; then
  # ⚠️ ⛔ **沒有路徑覆寫參數**：v25 已裁決「正式流程固定由 XDG 推導，測試改覆寫
  # `XDG_DATA_HOME`」——⛔ 換個名字加回來等於把移除掉的後門原樣放回去。
  # ⚠️ `I074_STAGE` 只決定推導出哪一份（封閉列舉），⛔ 不是路徑。
  I074_IDENTITY_PATH="${XDG_DATA_HOME:-$HOME/.local/share}/stock_trading/i074_stage${I074_STAGE}/run_identity.json"
  if [ ! -f "$I074_IDENTITY_PATH" ]; then
    echo "ERROR: 找不到 run identity：$I074_IDENTITY_PATH" >&2
    echo "       先執行 scripts/pin-replay-image.sh --stage $I074_STAGE <bundle> 建立它。" >&2
    exit 1
  fi
  python3 "$PYTHON_DIR/scripts/validate-i074-run-identity.py" "$I074_IDENTITY_PATH" \
      --expect-image-id "${REPLAY_IMAGE_ID:-}" --bundle "$BUNDLE_ABS" >/dev/null
fi

# Stage 1 的兩份 artifact 也不在版本 worktree 裡——各自唯讀掛載其所在目錄。
declare -A SEEN_DIRS=()
for artifact in "$AFTER_ARTIFACT" "$COHORT_MANIFEST"; do
  [ -n "$artifact" ] || continue
  if [ ! -f "$artifact" ]; then
    echo "ERROR: artifact 不存在：$artifact（要用 host 絕對路徑）" >&2
    exit 1
  fi
  dir="$(cd "$(dirname "$artifact")" && pwd)"
  if [ -z "${SEEN_DIRS[$dir]:-}" ] && [ "$dir" != "$OUT_ABS" ] && [ "$dir" != "$BUNDLE_ABS" ]; then
    SEEN_DIRS[$dir]=1
    MOUNTS+=(-v "$dir":"$dir":ro)
  fi
done

WORKTREE=""
FROZEN_DIR=""
cleanup() {
  if [ -n "$WORKTREE" ]; then
    git -C "$REPO_ROOT" worktree remove --force "$WORKTREE" >/dev/null 2>&1 || true
    rm -rf "$WORKTREE"
  fi
  [ -z "$FROZEN_DIR" ] || rm -rf "$FROZEN_DIR"
}
trap cleanup EXIT

# ── 兩份 patch 先**凍結**（③ evidence contract「七之三」的 runner 層）──────────────────────
# ⚠️ 各讀一次到私有目錄，之後的套用與 SHA 一律只用副本——⛔ 不再讀原始路徑。空的 TOOLING_PATCH 直接建
# **0-byte 副本**（⛔ 不是「跳過」），於是 tooling 的 SHA 自然是空字串的 SHA。
FROZEN_DIR="$(mktemp -d)"
FROZEN_CF=""
FROZEN_TOOL="$FROZEN_DIR/tooling.patch"
freeze_patch() {  # $1＝來源；$2＝目的地；$3＝說明
  if [ -L "$1" ] || [ ! -f "$1" ]; then
    echo "ERROR: $3 必須是一般檔案（⛔ symlink、⛔ 目錄）：$1" >&2; exit 1
  fi
  cp -- "$1" "$2" || { echo "ERROR: 讀不到 $3：$1" >&2; exit 1; }
}
if [ "$CF_FLAG" = "1" ]; then
  FROZEN_CF="$FROZEN_DIR/counterfactual.patch"
  freeze_patch "$COUNTERFACTUAL_PATCH" "$FROZEN_CF" COUNTERFACTUAL_PATCH
  [ -s "$FROZEN_CF" ] || { echo "ERROR: COUNTERFACTUAL_PATCH 是 0 bytes——空的反事實＝沒有反事實。" >&2; exit 1; }
fi
if [ -n "$TOOLING_PATCH" ]; then
  freeze_patch "$TOOLING_PATCH" "$FROZEN_TOOL" TOOLING_PATCH
else
  : > "$FROZEN_TOOL"
fi

WORKTREE="$(mktemp -d)"
rmdir "$WORKTREE"   # worktree add 要求目標不存在
BASE_COMMIT="$(replay_args_prepare_worktree "$REPO_ROOT" "$SOURCE_REF" "$WORKTREE")"
# ⚠️ 唯一的合成函式：固定順序 counterfactual → tooling，三個增量 canonical SHA ＋ 語意 SHA。
COMPOSE_OUT="$(replay_args_compose "$WORKTREE" "$BASE_COMMIT" "$FROZEN_CF" "$FROZEN_TOOL")"
read -r _T1 _T2 CF_PATCH_SHA256 TOOL_INCR_SHA256 TOOLING_PATCH_SHA256 _SEM <<< "$COMPOSE_OUT"
CF_INJECT=""
if [ "$CF_FLAG" = "1" ]; then
  # ⚠️ **runner 的前置守門**：兩份 patch 的 raw bytes 必須**就是** canonical diff——⛔ 不讓非 canonical 的
  # patch 燒掉一整趟 replay 才在 finalize 被擋（正式 archive 無條件要求 raw ＝ canonical）。
  if [ "$(sha256sum < "$FROZEN_CF" | cut -d' ' -f1)" != "$CF_PATCH_SHA256" ]; then
    echo "ERROR: COUNTERFACTUAL_PATCH ⛔ 不是 canonical diff 的輸出（raw bytes 的 SHA ≠ 重建的 $CF_PATCH_SHA256）" \
         "——⛔ 在 replay 之前中止。" >&2
    exit 1
  fi
  if [ "$(sha256sum < "$FROZEN_TOOL" | cut -d' ' -f1)" != "$TOOL_INCR_SHA256" ]; then
    echo "ERROR: TOOLING_PATCH ⛔ 不是 canonical diff 的輸出（raw bytes 的 SHA ≠ 重建的增量 SHA $TOOL_INCR_SHA256）" \
         "——⛔ 在 replay 之前中止。" >&2
    exit 1
  fi
  CF_INJECT="$CF_PATCH_SHA256"
fi
RUNNER_SHA256="$(replay_args_runner_sha256 "${BASH_SOURCE[0]}")"

# ── I-074 Stage 2 的 after' 見證趟：E7 **在 replay 之前**就驗 ──────────────────
#
# ⚠️ Stage 2 identity ＋ Stage 1 模式＝環境見證趟。它必須跑**原始的 Stage 1 base**、⛔ 不套 patch
# （E7）；⛔ 不能等到 envcheck 發布才擋——那時已燒完約 180 分鐘，而 after' 只有一趟。
# 兩個值都已由上面算好（worktree 實際的 HEAD 與 diff），這裡用 Stage 1 信任錨比對。
if [ "$I074_MODE" = "1" ] && [ "$I074_STAGE" = "2" ] && [ "$STAGE" = "1" ]; then
  python3 "$PYTHON_DIR/scripts/check-i074-witness-run.py" \
      --base-commit "$BASE_COMMIT" --tooling-patch-sha256 "$TOOLING_PATCH_SHA256"
fi

# ── image：⚠️ **一律以 image ID 執行**，⛔ 不用 tag ──────────────────────────
#
# ⛔ 舊版是 `docker build` → inspect 出 digest 只寫進 provenance → **`docker run` 仍用 tag**，
# build 與 run 之間留下 **tag 移動窗口**。
#
# ⚠️ 無 `REPLAY_IMAGE_ID` 時**呼叫 `pin-replay-image.sh`**（⛔ 不在這裡自己 build）——
# 那是**唯一**的 build 實作，既保留「不設環境變數也能跑」的既有用法，又不會有第二套 build。
if [ -n "${REPLAY_IMAGE_ID:-}" ]; then
  if [ -n "${PY_IMAGE:-}" ]; then
    echo "ERROR: REPLAY_IMAGE_ID 與 PY_IMAGE ⛔ 不可同時設定（會有兩個 image 來源）。" >&2
    exit 1
  fi
  ACTUAL_ID="$(docker image inspect "$REPLAY_IMAGE_ID" -f '{{.Id}}' 2>/dev/null || true)"
  if [ "$ACTUAL_ID" != "$REPLAY_IMAGE_ID" ]; then
    echo "ERROR: REPLAY_IMAGE_ID=$REPLAY_IMAGE_ID 在本機找不到（實際 '$ACTUAL_ID'）。" >&2
    echo "       ⛔ 不得改為重建：跨日的三趟必須跑在同一個 image 上。" >&2
    exit 1
  fi
  echo "==> 使用釘死的 image ID（⛔ 未 build）：$REPLAY_IMAGE_ID"
  IMAGE_DIGEST="$REPLAY_IMAGE_ID"
else
  if [ "$I074_MODE" = "1" ]; then
    echo "ERROR: I-074 正式流程（--i074-preflight／--i074-capacity-probe）⛔ 要求預先提供" >&2
    echo "       REPLAY_IMAGE_ID（由 scripts/pin-replay-image.sh 取得）——⛔ 不自動 pin。" >&2
    exit 1
  fi
  # ⚠️ `--no-identity`：一般 Stage 1／2 只需要「單一 build 實作 ＋ 以 ID 執行」，
  # ⛔ 不該順手在使用者家目錄建立 I-074 的協調檔。
  IMAGE_DIGEST="$("$REPO_ROOT/scripts/pin-replay-image.sh" --no-identity)"
fi
if [ -z "$IMAGE_DIGEST" ]; then
  echo "ERROR: 取不到 image ID——⛔ 不產出說不清出處的 artifact。" >&2
  exit 1
fi

# `--source-root` 是容器內的唯讀掛載點（/app），⛔ 不是 host 路徑。
mapfile -t CMD_ARGS < <(replay_args_offline \
  "$IMAGE_DIGEST" /app "$BASE_COMMIT" "$TOOLING_PATCH_SHA256" "$RUNNER_SHA256" "$CF_INJECT" "$@")

DOCKER_ARGS=(
  --rm
  --user "$(id -u):$(id -g)"
  --network none
  --cpus="$CPUS"
  --memory="$MEM"
  --memory-swap="$MEMSWAP"
  --pids-limit=200
  -e HOME=/tmp
  -e PYTHONDONTWRITEBYTECODE=1
  "${MOUNTS[@]}"
  -v "$WORKTREE/python":/app:ro
  -w /app
)

echo "==> offline replay：stage=$STAGE source_ref=$SOURCE_REF base_commit=$BASE_COMMIT mem=$MEM"
echo "==> patches: counterfactual=${CF_INJECT:--} tooling=$TOOL_INCR_SHA256 composed=$TOOLING_PATCH_SHA256"

if [ "${REPLAY_DRY_RUN:-0}" = "1" ]; then
  printf '%s\n' docker run "${DOCKER_ARGS[@]}" "$IMAGE_DIGEST" "${CMD_ARGS[@]}"
  exit 0
fi

# ── MEASURE_PEAK=1：量峰值並寫成 probe 的 measurement ＋ completion ─────────
#
# ⚠️ **峰值由容器自己在指令結束後、退出前寫進 `/peak/peak`**，⛔ 不從外面輪詢——
# `run-evaluation.sh:51` 記過那個實測教訓（輪詢會漏掉真正的峰值）。
# ⚠️ 這裡用**前景**執行（⛔ 不 detached）：log 仍是即時串流，跑三小時的過程看得見。
# ⛔ **讀不到就 fail-closed**：⛔ 不得把「量不到」寫成 0 bytes 歸檔。
if [ "${MEASURE_PEAK:-0}" = "1" ]; then
  PEAK_DIR="$(mktemp -d)"
  HOST_LOW_FILE="$PEAK_DIR/host_low"
  # ⚠️ ⛔ **不得覆蓋既有的 worktree cleanup trap**——那樣每次 probe 都會留下
  # detached worktree。這裡把兩者串起來。
  cleanup_peak() {
    rm -rf "$PEAK_DIR"
    [ -n "${LOW_PID:-}" ] && kill "$LOW_PID" 2>/dev/null || true
    cleanup
  }
  trap cleanup_peak EXIT

  # host 的 MemAvailable 低點：容器跑的期間每 2 秒取一次最小值。
  ( low=""
    while :; do
      cur="$(awk '/^MemAvailable:/ {print $2 * 1024; exit}' /proc/meminfo 2>/dev/null || echo "")"
      if [ -n "$cur" ] && { [ -z "$low" ] || [ "$cur" -lt "$low" ]; }; then
        low="$cur"; printf '%s' "$low" > "$HOST_LOW_FILE"
      fi
      sleep 2
    done ) &
  LOW_PID=$!

  # 包一層 sh：跑完原本的指令後、退出前把 cgroup 峰值寫出來，並**保留原指令的 exit code**。
  # 兩種 cgroup 版本都試（v1 在 4.19 kernel 上是 memory.max_usage_in_bytes）。
  PEAK_WRAPPER='"$@"; rc=$?;
{ cat /sys/fs/cgroup/memory/memory.max_usage_in_bytes 2>/dev/null \
  || cat /sys/fs/cgroup/memory.peak 2>/dev/null; } > /peak/peak || true
exit $rc'

  set +e
  docker run "${DOCKER_ARGS[@]}" -v "$PEAK_DIR":/peak "$IMAGE_DIGEST" \
    sh -c "$PEAK_WRAPPER" _ "${CMD_ARGS[@]}"
  REPLAY_RC=$?
  set -e
  kill "$LOW_PID" 2>/dev/null || true

  if [ "$REPLAY_RC" -ne 0 ]; then
    echo "==> replay 以 rc=$REPLAY_RC 結束——⛔ 不寫 measurement。" >&2
    exit "$REPLAY_RC"
  fi

  PEAK_BYTES="$(tr -dc '0-9' < "$PEAK_DIR/peak" 2>/dev/null || true)"
  HOST_LOW_BYTES="$(tr -dc '0-9' < "$HOST_LOW_FILE" 2>/dev/null || true)"
  CGROUP_LIMIT_BYTES="$(( $(mem_guard_to_mb "$MEM") * 1024 * 1024 ))"
  if [ -z "$PEAK_BYTES" ] || [ "$PEAK_BYTES" = "0" ] || [ -z "$HOST_LOW_BYTES" ]; then
    echo "ERROR: 量測缺失（peak='$PEAK_BYTES' host_low='$HOST_LOW_BYTES'）——⛔ fail-closed。" >&2
    echo "       ⛔ 不得把「量不到」歸檔成 0 bytes：那會讓後續判讀以為峰值極低。" >&2
    echo "       ⛔ 因此也不寫 completion——⛔ 不留下一份看起來完整的 probe。" >&2
    exit 1
  fi

  # ⚠️ measurement 與 completion 由 **Python** 寫（canonical ＋ schema validator），
  # ⛔ shell 不自己組 JSON。provenance 從 computation 沿用——三份必須來自同一次執行。
  python3 "$PYTHON_DIR/scripts/write-i074-probe-measurement.py" \
    --computation "$OUT_ABS/capacity_probe_computation.json" \
    --output-dir "$OUT_ABS" \
    --peak-rss-bytes "$PEAK_BYTES" \
    --host-low-bytes "$HOST_LOW_BYTES" \
    --cgroup-limit-bytes "$CGROUP_LIMIT_BYTES"
  exit 0
fi

# ⚠️ **以 image ID 執行**，⛔ 不是 tag。
exec docker run "${DOCKER_ARGS[@]}" "$IMAGE_DIGEST" "${CMD_ARGS[@]}"
