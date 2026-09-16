#!/usr/bin/env bash
# I-074 Stage 1：把 operational 產物封存成**一次發布的整包證據**，或修復 durability。
#
# 兩種模式（⚠️ CLI matrix 見計畫書十三-C）：
#
#   # normal：9 份 operational → archived evidence（⚠️ root 事前必須完全不存在）
#   REPLAY_IMAGE_ID=sha256:… scripts/finalize-evidence.sh \
#       --evidence-root <repo 內路徑> --source <rel>=<path> …（9 份）
#
#   # recovery：⛔ 不讀 operational inputs、⛔ 不重新壓縮，只重新驗證既有 root 並 fsync
#   REPLAY_IMAGE_ID=sha256:… scripts/finalize-evidence.sh \
#       --evidence-root <同一路徑> --recover-durability
#
# 結束碼：0／5 ＝ 依 crossday 的 `outcome` 還原的終端結果；
#         **3 ＝ durability 未確認**（證據已發布，⛔ 不刪除，用 --recover-durability 修復）；
#         1 ＝ 其他失敗。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PYTHON_DIR="$REPO_ROOT/python"
. "$REPO_ROOT/scripts/lib/replay-args.sh"

# ⚠️ ⛔ **不用 `replay_args_reject_injected`**：它會連「唯一前綴縮寫」一起擋，而
# **`--source` 正好是 `--source-root` 的前綴**——共用那份清單會讓 finalizer 的
# `--source` **完全不能用**（實測踩到過；與 `--run-identity` vs `--run-id` 同一類錯）。
# finalizer 自己做**精確比對**的所有權檢查。
FINALIZER_INJECTED=(--run-identity --image-digest --base-commit --tooling-patch-sha256 --source-root --runner-sha256)
for _arg in "$@"; do
  for _inj in "${FINALIZER_INJECTED[@]}"; do
    if [ "$_arg" = "$_inj" ] || [ "${_arg%%=*}" = "$_inj" ]; then
      echo "ERROR: $_inj 只能由官方腳本注入，⛔ 不接受使用者傳入。" >&2
      exit 1
    fi
  done
done

ROOT=""; RECOVER=0; SOURCES=(); SOURCE_REF="HEAD"
while [ "$#" -gt 0 ]; do
  case "$1" in
    --evidence-root) ROOT="$2"; shift 2 ;;
    --source) SOURCES+=("$2"); shift 2 ;;
    --recover-durability) RECOVER=1; shift ;;
    --source-ref) SOURCE_REF="$2"; shift 2 ;;
    *) echo "ERROR: 未知參數 $1" >&2; exit 1 ;;
  esac
done
[ -n "$ROOT" ] || { echo "用法: $0 --evidence-root <dir> [--source rel=path …|--recover-durability]" >&2; exit 1; }

IMAGE_ID="${REPLAY_IMAGE_ID:-}"
[ -n "$IMAGE_ID" ] || { echo "ERROR: finalizer ⛔ 一律要求 REPLAY_IMAGE_ID。" >&2; exit 1; }
[ -z "${PY_IMAGE:-}" ] || { echo "ERROR: REPLAY_IMAGE_ID 與 PY_IMAGE ⛔ 不可同時設定。" >&2; exit 1; }
docker image inspect "$IMAGE_ID" >/dev/null 2>&1 || { echo "ERROR: image $IMAGE_ID 不在本機。" >&2; exit 1; }

mkdir -p "$(dirname "$ROOT")"; ROOT_ABS="$(cd "$(dirname "$ROOT")" && pwd)/$(basename "$ROOT")"
MOUNTS=(-v "$(dirname "$ROOT_ABS")":"$(dirname "$ROOT_ABS")")
CMD_EXTRA=()

if [ "$RECOVER" = "1" ]; then
  [ "${#SOURCES[@]}" -eq 0 ] || { echo "ERROR: --recover-durability ⛔ 不接受 --source。" >&2; exit 1; }
  # ⚠️ **recovery ⛔ 不掛載也⛔ 不注入外部 identity**——它只讀 evidence 內的 archived copy。
  # Docker 之前先用 host validator 驗那一份（⛔ 不驗 bundle_id：recovery 手上沒有 bundle
  # 第二來源，那層關係留到容器內的全圖驗證、fsync 之前）。
  ARCHIVED="$ROOT_ABS/identity/run_identity.json.gz"
  [ -f "$ARCHIVED" ] || { echo "ERROR: 找不到 archived identity：$ARCHIVED" >&2; exit 1; }
  python3 "$PYTHON_DIR/scripts/validate-i074-run-identity.py" "$ARCHIVED" \
      --expect-image-id "$IMAGE_ID" >/dev/null
  CMD_EXTRA=(--recover-durability)
else
  [ "${#SOURCES[@]}" -gt 0 ] || { echo "ERROR: normal finalization 需要 --source。" >&2; exit 1; }
  # ⚠️ ⛔ 固定由 XDG 推導，⛔ 沒有覆寫參數。
  IDENTITY="${XDG_DATA_HOME:-$HOME/.local/share}/stock_trading/i074_stage1/run_identity.json"
  [ -f "$IDENTITY" ] || { echo "ERROR: 找不到 run identity：$IDENTITY" >&2; exit 1; }
  ID_ABS="$(cd "$(dirname "$IDENTITY")" && pwd)/$(basename "$IDENTITY")"
  python3 "$PYTHON_DIR/scripts/validate-i074-run-identity.py" "$ID_ABS" \
      --expect-image-id "$IMAGE_ID" >/dev/null
  # ⚠️ **same-path 唯讀掛載** ＋ 由本腳本注入 `--run-identity`。
  MOUNTS+=(-v "$ID_ABS":"$ID_ABS":ro)
  CMD_EXTRA=(--run-identity "$ID_ABS")
  for item in "${SOURCES[@]}"; do
    path="${item#*=}"
    abs="$(cd "$(dirname "$path")" && pwd)/$(basename "$path")"
    MOUNTS+=(-v "$abs":"$abs":ro)
    CMD_EXTRA+=(--source "${item%%=*}=$abs")
  done
fi

WORKTREE="$(mktemp -d)"
trap 'rm -rf "$WORKTREE"' EXIT
BASE_COMMIT="$(replay_args_prepare_worktree "$REPO_ROOT" "$SOURCE_REF" "$WORKTREE")"
TOOLING_PATCH_SHA256="$(replay_args_tooling_patch_sha256 "$WORKTREE" "$BASE_COMMIT" "${TOOLING_PATCH:-}")"
RUNNER_SHA256="$(replay_args_runner_sha256 "${BASH_SOURCE[0]}")"

DOCKER_ARGS=(
  --rm --network none --user "$(id -u):$(id -g)"
  --cpus="${CPUS:-1}" --memory="${MEM:-700m}" --memory-swap="${MEM:-700m}"
  -e HOME=/tmp -e PYTHONDONTWRITEBYTECODE=1
  -v "$WORKTREE/python":/app:ro "${MOUNTS[@]}" -w /app
)
CMD=(python -m backtest.modular.sr_scoring.replay_bundle.evidence
     --evidence-root "$ROOT_ABS" "${CMD_EXTRA[@]}"
     --image-digest "$IMAGE_ID" --base-commit "$BASE_COMMIT"
     --tooling-patch-sha256 "$TOOLING_PATCH_SHA256"
     --source-root /app --runner-sha256 "$RUNNER_SHA256")

if [ "${REPLAY_DRY_RUN:-0}" = "1" ]; then
  printf '%s\n' docker run "${DOCKER_ARGS[@]}" "$IMAGE_ID" "${CMD[@]}"
  exit 0
fi
exec docker run "${DOCKER_ARGS[@]}" "$IMAGE_ID" "${CMD[@]}"
