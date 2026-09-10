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

echo "==> 建置 image：$IMAGE"
docker build -t "$IMAGE" "$PYTHON_DIR"
IMAGE_DIGEST="$(docker image inspect "$IMAGE" -f '{{.Id}}' 2>/dev/null || true)"
if [ -z "$IMAGE_DIGEST" ]; then
  echo "ERROR: 取不到 $IMAGE 的 image digest——⛔ 不產出說不清出處的 artifact。" >&2
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
  printf '%s\n' docker run "${DOCKER_ARGS[@]}" "$IMAGE" "${CMD_ARGS[@]}"
  exit 0
fi

exec docker run "${DOCKER_ARGS[@]}" "$IMAGE" "${CMD_ARGS[@]}"
