#!/usr/bin/env bash
# I-129 的離線影響量測：在 python image 裡跑逐列比較工具（python/scripts/compare-i129-replay.py）。
#
# 用法（路徑一律 **host 絕對路徑**，容器內掛在同一個路徑上）：
#   scripts/compare-i129-replay.sh --bundle <bundle 目錄> \
#       --a <commit A 的 after_artifact.json> --b <commit B 的 after_artifact.json> \
#       --a-commit <A 的完整 commit> --b-commit <B 的完整 commit> \
#       [--mode probe|fallback] --output <report.json>
#   --mode probe（預設）量試單的設計；--mode fallback 驗收退回版（⛔ 不判績效）。
#
# 結束碼：同 compare-i129-replay.py（0 PASS／FALLBACK_ACCEPTED、3 退回「不放行」、4 不變式不成立、5 無法歸因）；1 ＝ 參數錯誤。
# 判準：docs/sr-zone-scoring.md「I-129 離線影響量測的判準」；流程：docs/development-workflow.md「I-129 的離線影響量測工具」。
#
# 設計重點：
#   - `--network none`、輸入一律唯讀掛載，只有 output 所在的目錄可寫；⛔ 不覆寫既有報告。
#   - 記憶體經 scripts/lib/mem-guard.sh 下修，OOM 時先犧牲這個 container（同 python/scripts/test.sh）。
#     比較工具是串流的（⛔ 不整份載入兩份 artifact），預設 600m 足夠。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE="${PY_IMAGE:-stock-trading-python-test:latest}"
MEM="${MEM:-600m}"

usage() { sed -n '4,10p' "$0" >&2; exit 1; }

BUNDLE="" A="" B="" OUTPUT="" A_COMMIT="" B_COMMIT="" MODE="probe"
while [ "$#" -gt 0 ]; do
  case "$1" in
    --bundle) BUNDLE="${2:-}"; shift 2 ;;
    --a) A="${2:-}"; shift 2 ;;
    --b) B="${2:-}"; shift 2 ;;
    --output) OUTPUT="${2:-}"; shift 2 ;;
    --a-commit) A_COMMIT="${2:-}"; shift 2 ;;
    --b-commit) B_COMMIT="${2:-}"; shift 2 ;;
    --mode) MODE="${2:-}"; shift 2 ;;
    *) echo "ERROR: 不認得的參數 $1" >&2; usage ;;
  esac
done
for pair in "--bundle:$BUNDLE" "--a:$A" "--b:$B" "--output:$OUTPUT"; do
  name="${pair%%:*}"; value="${pair#*:}"
  [ -n "$value" ] || { echo "ERROR: 缺少 $name" >&2; usage; }
  case "$value" in /*) ;; *) echo "ERROR: $name 要用絕對路徑：$value" >&2; exit 1 ;; esac
done
for pair in "--a-commit:$A_COMMIT" "--b-commit:$B_COMMIT"; do
  name="${pair%%:*}"; value="${pair#*:}"
  [[ "$value" =~ ^[0-9a-f]{40}$ ]] || { echo "ERROR: $name 要是完整的 40 字元 commit：'$value'" >&2; exit 1; }
done
case "$MODE" in probe|fallback) ;; *) echo "ERROR: --mode 只接受 probe／fallback：'$MODE'" >&2; exit 1 ;; esac
[ -d "$BUNDLE" ] || { echo "ERROR: 找不到 bundle 目錄 $BUNDLE" >&2; exit 1; }
[ -f "$A" ] || { echo "ERROR: 找不到 $A" >&2; exit 1; }
[ -f "$B" ] || { echo "ERROR: 找不到 $B" >&2; exit 1; }
[ ! -e "$OUTPUT" ] || { echo "ERROR: $OUTPUT 已存在（⛔ 不覆寫）" >&2; exit 1; }
OUT_DIR="$(dirname "$OUTPUT")"
[ -d "$OUT_DIR" ] || { echo "ERROR: 輸出目錄 $OUT_DIR 不存在" >&2; exit 1; }

# shellcheck source=lib/mem-guard.sh
. "$REPO_ROOT/scripts/lib/mem-guard.sh"
MEM="$(mem_guard_clamp "$MEM")"
OOM_ARG="$(mem_guard_oom_arg)" || exit 1

# 掛載：同一個目錄只掛一次（docker ⛔ 不接受重複的掛載點）；output 目錄可寫，其餘唯讀。
MOUNTS=(-v "$REPO_ROOT/python":/app:ro -v "$BUNDLE":"$BUNDLE":ro -v "$OUT_DIR":"$OUT_DIR")
for dir in "$(dirname "$A")" "$(dirname "$B")"; do
  case " $OUT_DIR $BUNDLE ${SEEN_DIRS:-} " in
    *" $dir "*) ;;
    *) MOUNTS+=(-v "$dir":"$dir":ro); SEEN_DIRS="${SEEN_DIRS:-} $dir" ;;
  esac
done

echo "==> 建置 image：$IMAGE" >&2
docker build -q -t "$IMAGE" "$REPO_ROOT/python" >/dev/null
echo "==> 比較（mem=$MEM、--network none）" >&2
exec docker run --rm --network none \
  --user "$(id -u):$(id -g)" \
  --memory="$MEM" --memory-swap="$MEM" \
  ${OOM_ARG:+"$OOM_ARG"} \
  -e HOME=/tmp -e PYTHONDONTWRITEBYTECODE=1 \
  "${MOUNTS[@]}" \
  -w /app "$IMAGE" \
  python scripts/compare-i129-replay.py --bundle "$BUNDLE" --a "$A" --b "$B" \
    --a-commit "$A_COMMIT" --b-commit "$B_COMMIT" --mode "$MODE" --output "$OUTPUT"
