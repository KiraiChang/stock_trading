#!/usr/bin/env bash
# I-074 Stage 2：證據的發布與修復入口（issue.md I-074 ③ evidence contract「七之四：CLI matrix」）。
#
# ⚠️ **③b 第一包只實作環境見證的兩種模式**；其餘五種（normal／recover-durability／
# publish-failed-record／check-failed-record／recover-failed-record）屬 ③d。
#
#   # 發布：見證趟的 operational 輸出 → python/baselines/i074_stage2/envcheck/
#   #       ⚠️ EQUIVALENT（0）與 NOT_EQUIVALENT（7）**都會發布證據**
#   REPLAY_IMAGE_ID=sha256:… scripts/finalize-stage2-evidence.sh --envcheck --run-dir <run 目錄>
#
#   # 修復：⛔ 不讀 operational 輸入、⛔ 不重跑 replay，只重新驗證既有 envcheck/ 並 fsync
#   REPLAY_IMAGE_ID=sha256:… scripts/finalize-stage2-evidence.sh --recover-envcheck
#
# `<run 目錄>` 底下必須有 `witness/after_artifact.json` 與 `witness/cohort_manifest.json`
# （after' 見證趟以 Stage 1 模式、`--output-dir <run 目錄>/witness` 產出）。
#
# 結束碼：0 ＝ EQUIVALENT；**7 ＝ NOT_EQUIVALENT**（證據已發布，Stage 2 停在 before 之前）；
#         **3 ＝ durability 未確認**（⛔ 不刪除，用 --recover-envcheck 修復）；1 ＝ 其他。
#
# 設計重點（與 Stage 1 的 `finalize-evidence.sh` 同一套紀律）：
#   - ⚠️ 路徑一律**寫死常數或由 `--run-dir` 固定推導**，⛔ 使用者不得逐檔覆寫來源；
#   - ⚠️ Python 段在 **Stage 2 identity 的 image** 內執行（`REPLAY_IMAGE_ID` 必須與它相符）；
#   - ⚠️ Stage 1 證據**唯讀**掛載，只有 `python/baselines/i074_stage2/` 可寫；
#   - 程式碼來自 `--source-ref`（預設 HEAD）的 detached worktree，唯讀掛在 `/app`。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PYTHON_DIR="$REPO_ROOT/python"
. "$REPO_ROOT/scripts/lib/replay-args.sh"
. "$REPO_ROOT/scripts/lib/mem-guard.sh"

# ⚠️ 注入參數只能由本腳本給——**精確比對**（⛔ 不共用前綴清單，理由見 finalize-evidence.sh）。
STAGE2_INJECTED=(--run-identity --python-root --image-digest --base-commit --tooling-patch-sha256 --source-root --runner-sha256)
for _arg in "$@"; do
  for _inj in "${STAGE2_INJECTED[@]}"; do
    if [ "$_arg" = "$_inj" ] || [ "${_arg%%=*}" = "$_inj" ]; then
      echo "ERROR: $_inj 只能由官方腳本注入，⛔ 不接受使用者傳入。" >&2
      exit 1
    fi
  done
done

MODE=""; RUN_DIR=""; SOURCE_REF="HEAD"; SOURCE_REF_SEEN=0
while [ "$#" -gt 0 ]; do
  case "$1" in
    --envcheck|--recover-envcheck)
      [ -z "$MODE" ] || { echo "ERROR: 模式旗標互斥：已有 --$MODE，又給了 $1。" >&2; exit 1; }
      MODE="${1#--}"; shift ;;
    --run-dir)
      [ "$#" -ge 2 ] || { echo "ERROR: --run-dir 需要值。" >&2; exit 1; }
      [ -z "$RUN_DIR" ] || { echo "ERROR: --run-dir 重複出現——⛔ 不靜默採用最後一個。" >&2; exit 1; }
      RUN_DIR="$2"; shift 2 ;;
    --source-ref)
      [ "$#" -ge 2 ] || { echo "ERROR: --source-ref 需要值。" >&2; exit 1; }
      # ⚠️ ⛔ 不靜默採用最後一個：它決定跑哪一份程式碼（CLI matrix：重複參數一律中止）。
      [ "$SOURCE_REF_SEEN" = "0" ] || { echo "ERROR: --source-ref 重複出現——⛔ 不靜默採用最後一個。" >&2; exit 1; }
      SOURCE_REF_SEEN=1; SOURCE_REF="$2"; shift 2 ;;
    *) echo "ERROR: 未知參數 $1" >&2; exit 1 ;;
  esac
done
case "$MODE" in
  envcheck)
    [ -n "$RUN_DIR" ] || { echo "ERROR: --envcheck 需要 --run-dir。" >&2; exit 1; } ;;
  recover-envcheck)
    [ -z "$RUN_DIR" ] || { echo "ERROR: --recover-envcheck ⛔ 不接受 --run-dir：它只依賴既有的 envcheck/。" >&2; exit 1; } ;;
  *)
    echo "用法: $0 --envcheck --run-dir <dir> | --recover-envcheck" >&2; exit 1 ;;
esac

IMAGE_ID="${REPLAY_IMAGE_ID:-}"
[ -n "$IMAGE_ID" ] || { echo "ERROR: ⛔ 一律要求 REPLAY_IMAGE_ID（Stage 2 identity 的 image）。" >&2; exit 1; }
[ -z "${PY_IMAGE:-}" ] || { echo "ERROR: REPLAY_IMAGE_ID 與 PY_IMAGE ⛔ 不可同時設定。" >&2; exit 1; }
docker image inspect "$IMAGE_ID" >/dev/null 2>&1 || {
  echo "ERROR: image $IMAGE_ID 不在本機——用 scripts/restore-replay-image.sh --stage 2 <bundle> 還原。" >&2
  exit 1
}

STAGE1_BASE="$PYTHON_DIR/baselines/i074_stage1"
STAGE2_BASE="$PYTHON_DIR/baselines/i074_stage2"
[ -d "$STAGE1_BASE" ] || { echo "ERROR: 找不到 Stage 1 證據：$STAGE1_BASE（⚠️ 必須與 Stage 2 一起保存）" >&2; exit 1; }
mkdir -p "$STAGE2_BASE"
MOUNTS=(-v "$STAGE1_BASE":"$STAGE1_BASE":ro -v "$STAGE2_BASE":"$STAGE2_BASE")

if [ "$MODE" = "envcheck" ]; then
  # ⚠️ ⛔ 固定由 XDG 推導 Stage 2 的 identity，⛔ 沒有覆寫參數。
  IDENTITY="${XDG_DATA_HOME:-$HOME/.local/share}/stock_trading/i074_stage2/run_identity.json"
  [ -f "$IDENTITY" ] || { echo "ERROR: 找不到 Stage 2 的 run identity：$IDENTITY" >&2; exit 1; }
  ID_ABS="$(replay_args_abs_path "$IDENTITY" "run identity")"
  python3 "$PYTHON_DIR/scripts/validate-i074-run-identity.py" "$ID_ABS" \
      --expect-image-id "$IMAGE_ID" >/dev/null
  [ -d "$RUN_DIR" ] || { echo "ERROR: --run-dir 不存在：$RUN_DIR" >&2; exit 1; }
  RUN_ABS="$(cd "$RUN_DIR" && pwd)"
  # ⚠️ 兩份都是**輸入**，⛔ 進 Docker 前必須確定是既有檔案。
  replay_args_require_file "$RUN_ABS/witness/after_artifact.json" "--run-dir 的 witness/after_artifact.json"
  replay_args_require_file "$RUN_ABS/witness/cohort_manifest.json" "--run-dir 的 witness/cohort_manifest.json"
  MOUNTS+=(-v "$RUN_ABS/witness":"$RUN_ABS/witness":ro -v "$ID_ABS":"$ID_ABS":ro)
  CMD_EXTRA=(--envcheck --run-dir "$RUN_ABS" --run-identity "$ID_ABS")
else
  # ⚠️ **recovery ⛔ 不掛載也⛔ 不注入外部 identity**——它只讀 envcheck 內封存的那一份。
  ARCHIVED="$STAGE2_BASE/envcheck/identity/run_identity.json.gz"
  [ -f "$ARCHIVED" ] || { echo "ERROR: 找不到封存的 identity：$ARCHIVED" >&2; exit 1; }
  python3 "$PYTHON_DIR/scripts/validate-i074-run-identity.py" "$ARCHIVED" \
      --expect-image-id "$IMAGE_ID" >/dev/null
  CMD_EXTRA=(--recover-envcheck)
fi

WORKTREE="$(mktemp -d)"
rmdir "$WORKTREE"   # worktree add 要求目標不存在
cleanup() {
  git -C "$REPO_ROOT" worktree remove --force "$WORKTREE" >/dev/null 2>&1 || true
  rm -rf "$WORKTREE"
}
trap cleanup EXIT
BASE_COMMIT="$(replay_args_prepare_worktree "$REPO_ROOT" "$SOURCE_REF" "$WORKTREE")"
TOOLING_PATCH_SHA256="$(replay_args_tooling_patch_sha256 "$WORKTREE" "$BASE_COMMIT" "${TOOLING_PATCH:-}")"
RUNNER_SHA256="$(replay_args_runner_sha256 "${BASH_SOURCE[0]}")"

MEM="$(mem_guard_clamp "${MEM:-700m}")"
DOCKER_ARGS=(
  --rm --network none --user "$(id -u):$(id -g)"
  --cpus="${CPUS:-1}" --memory="$MEM" --memory-swap="$MEM"
  -e HOME=/tmp -e PYTHONDONTWRITEBYTECODE=1
  -v "$WORKTREE/python":/app:ro "${MOUNTS[@]}" -w /app
)
CMD=(python -m backtest.modular.sr_scoring.replay_bundle.envcheck
     "${CMD_EXTRA[@]}"
     --python-root "$PYTHON_DIR"
     --image-digest "$IMAGE_ID" --base-commit "$BASE_COMMIT"
     --tooling-patch-sha256 "$TOOLING_PATCH_SHA256"
     --source-root /app --runner-sha256 "$RUNNER_SHA256")

if [ "${REPLAY_DRY_RUN:-0}" = "1" ]; then
  printf '%s\n' docker run "${DOCKER_ARGS[@]}" "$IMAGE_ID" "${CMD[@]}"
  exit 0
fi
# ⚠️ 前景執行、⛔ 不用 exec：保留 trap 讓 worktree 一定被清掉；原樣傳出 0／7／3／1。
set +e
docker run "${DOCKER_ARGS[@]}" "$IMAGE_ID" "${CMD[@]}"
RC=$?
set -e
exit "$RC"
