#!/usr/bin/env bash
# I-074 Stage 1：跨日逐列比對 D 與 D+1 的兩份 after artifact。
#
# 用法：
#   REPLAY_IMAGE_ID=sha256:… scripts/compare-replay-crossday.sh \
#       --d <D 的 after_artifact.json> --d1 <D+1 的 after_artifact.json> \
#       --output-dir <全新目錄>
#
# ⚠️ **結束碼原樣傳出**：0 ＝ MATCH；**5 ＝ 不一致（證據已完整發布，⛔ 不是失敗殘骸）**；
# 1 ＝ 輸入不合法或 validator 失敗（⛔ 此時不留下任何 artifact）。
#
# 設計重點：
#   - **same-path bind mount**：兩份輸入與 identity 都掛在**與 host 相同的絕對路徑**上
#     （`run-replay-offline.sh` 對 bundle／output dir 就是這樣做的）——參數不用改寫，
#     也就不會改寫錯，⛔ 不存在「host path vs container path」兩種值。
#   - **`--run-identity` 由本腳本注入**：⚠️ 兩份 after 只能互比，若**兩份都帶同一個錯誤
#     `bundle_id`**，沒有 identity 當獨立第二來源就沒有任何東西能發現。
#   - ⛔ `--run-identity` **不進 `REPLAY_INJECTED_ARGS`**：那份清單會連唯一前綴縮寫一起擋，
#     而 `--run-id` 正好是它的前綴——共用會讓 Stage 1／2 的 `--run-id` 直接壞掉（實測過）。
#     crossday 的 parser 自己 `allow_abbrev=False` ＋ 自查重複。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PYTHON_DIR="$REPO_ROOT/python"
. "$REPO_ROOT/scripts/lib/replay-args.sh"
replay_args_reject_injected "$@"
replay_args_reject_duplicates "$@"

D=""; D1=""; OUT=""; BEFORE_REF=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --d) D="$2"; shift 2 ;;
    --d1) D1="$2"; shift 2 ;;
    --output-dir) OUT="$2"; shift 2 ;;
    --source-ref) BEFORE_REF="$2"; shift 2 ;;
    *) echo "ERROR: 未知參數 $1" >&2; exit 1 ;;
  esac
done
[ -n "$D" ] && [ -n "$D1" ] && [ -n "$OUT" ] || { echo "用法: $0 --d <f> --d1 <f> --output-dir <dir>" >&2; exit 1; }

IMAGE_ID="${REPLAY_IMAGE_ID:-}"
if [ -z "$IMAGE_ID" ]; then
  echo "ERROR: comparator ⛔ 一律要求預先提供 REPLAY_IMAGE_ID（由 pin-replay-image.sh 取得）。" >&2
  exit 1
fi
if [ -n "${PY_IMAGE:-}" ]; then
  echo "ERROR: REPLAY_IMAGE_ID 與 PY_IMAGE ⛔ 不可同時設定。" >&2
  exit 1
fi
docker image inspect "$IMAGE_ID" >/dev/null 2>&1 || {
  echo "ERROR: image $IMAGE_ID 不在本機。" >&2; exit 1; }

# ⚠️ ⛔ 固定由 XDG 推導，⛔ 沒有覆寫參數（測試改覆寫 `XDG_DATA_HOME`）。
IDENTITY="${XDG_DATA_HOME:-$HOME/.local/share}/stock_trading/i074_stage1/run_identity.json"
[ -f "$IDENTITY" ] || { echo "ERROR: 找不到 run identity：$IDENTITY" >&2; exit 1; }

# ⚠️ **Docker 之前**先用 host validator 驗 identity（⛔ 不傳 --bundle：comparator 手上
# 沒有可信的 bundle 第二來源，那層關係留到容器內、crossday artifact 發布之前）。
python3 "$PYTHON_DIR/scripts/validate-i074-run-identity.py" "$IDENTITY" \
    --expect-image-id "$IMAGE_ID" >/dev/null

D_ABS="$(cd "$(dirname "$D")" && pwd)/$(basename "$D")"
D1_ABS="$(cd "$(dirname "$D1")" && pwd)/$(basename "$D1")"
mkdir -p "$OUT"; OUT_ABS="$(cd "$OUT" && pwd)"
ID_ABS="$(cd "$(dirname "$IDENTITY")" && pwd)/$(basename "$IDENTITY")"

WORKTREE="$(mktemp -d)"
trap 'rm -rf "$WORKTREE"' EXIT
BASE_COMMIT="$(replay_args_prepare_worktree "$REPO_ROOT" "${BEFORE_REF:-HEAD}" "$WORKTREE")"
TOOLING_PATCH_SHA256="$(replay_args_tooling_patch_sha256 "$WORKTREE" "$BASE_COMMIT" "${TOOLING_PATCH:-}")"
RUNNER_SHA256="$(replay_args_runner_sha256 "${BASH_SOURCE[0]}")"

DOCKER_ARGS=(
  --rm --network none --user "$(id -u):$(id -g)"
  --cpus="${CPUS:-1}" --memory="${MEM:-700m}" --memory-swap="${MEM:-700m}"
  -e HOME=/tmp -e PYTHONDONTWRITEBYTECODE=1
  -v "$WORKTREE/python":/app:ro
  # ⚠️ same-path：⛔ 不改寫路徑
  -v "$D_ABS":"$D_ABS":ro -v "$D1_ABS":"$D1_ABS":ro -v "$ID_ABS":"$ID_ABS":ro
  -v "$OUT_ABS":"$OUT_ABS"
  -w /app
)
CMD=(python -m backtest.modular.sr_scoring.replay_bundle.crossday
     --d "$D_ABS" --d1 "$D1_ABS" --output-dir "$OUT_ABS"
     --run-identity "$ID_ABS"
     --image-digest "$IMAGE_ID" --base-commit "$BASE_COMMIT"
     --tooling-patch-sha256 "$TOOLING_PATCH_SHA256"
     --source-root /app --runner-sha256 "$RUNNER_SHA256")

if [ "${REPLAY_DRY_RUN:-0}" = "1" ]; then
  printf '%s\n' docker run "${DOCKER_ARGS[@]}" "$IMAGE_ID" "${CMD[@]}"
  exit 0
fi
# ⚠️ **以 image ID 執行**，⛔ 不用 tag（tag 會移動）。結束碼原樣傳出（含 5）。
exec docker run "${DOCKER_ARGS[@]}" "$IMAGE_ID" "${CMD[@]}"
