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
#   TOOLING_PATCH  要套進 worktree 的 patch 檔（本階段的 tooling 變更）
#   REPLAY_DRY_RUN=1  只印出最終的 docker argv，不真的執行（給測試與人工核對用）
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
# ⚠️ `tooling_patch_sha256` 是**worktree 實際 `git diff --binary` 的輸出** hash，
# ⛔ 不是「傳進來那個 patch 檔的 hash」。`git diff --binary` 預設不含 untracked，所以用
# `git apply --index` 套用、補 `git add -A -N`，取完 diff 再斷言沒有 `??` 行。
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
for _arg in "$@"; do
  case "$_arg" in
    --i074-preflight|--i074-capacity-probe) I074_MODE=1 ;;
  esac
done

IMAGE="${PY_IMAGE:-stock-trading-python-test:latest}"
MEM="${MEM:-700m}"
CPUS="${CPUS:-1}"
AFTER_REF="${AFTER_REF:-HEAD}"
TOOLING_PATCH="${TOOLING_PATCH:-}"

STAGE="$(replay_args_offline_stage "$@")"
if [ "$STAGE" = "3" ]; then
  echo "ERROR: --after-artifact 與 --cohort-manifest 必須成對出現："                 >&2
  echo "       兩個都沒給是 Stage 1，兩個都給是 Stage 2，只給其中一個是不完整的模式。" >&2
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
  I074_IDENTITY_PATH="${XDG_DATA_HOME:-$HOME/.local/share}/stock_trading/i074_stage1/run_identity.json"
  if [ ! -f "$I074_IDENTITY_PATH" ]; then
    echo "ERROR: 找不到 run identity：$I074_IDENTITY_PATH" >&2
    echo "       先執行 scripts/pin-replay-image.sh <bundle> 建立它。" >&2
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

WORKTREE="$(mktemp -d)"
rmdir "$WORKTREE"   # worktree add 要求目標不存在
cleanup() {
  git -C "$REPO_ROOT" worktree remove --force "$WORKTREE" >/dev/null 2>&1 || true
  rm -rf "$WORKTREE"
}
trap cleanup EXIT

BASE_COMMIT="$(replay_args_prepare_worktree "$REPO_ROOT" "$SOURCE_REF" "$WORKTREE")"
TOOLING_PATCH_SHA256="$(replay_args_tooling_patch_sha256 "$WORKTREE" "$BASE_COMMIT" "$TOOLING_PATCH")"
RUNNER_SHA256="$(replay_args_runner_sha256 "${BASH_SOURCE[0]}")"

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
  "$IMAGE_DIGEST" /app "$BASE_COMMIT" "$TOOLING_PATCH_SHA256" "$RUNNER_SHA256" "$@")

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
