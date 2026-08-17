#!/usr/bin/env bash
# SR volatility 回歸基準的比對（計畫書階段 6）。**唯讀，不碰 DB。**
#
# 吃一份 evaluation report（由 scripts/run-evaluation.sh 產出）與版本控管的基準檔，
# 比對**序數性質**。不比絕對值——`atr_pct` 取近 60 根，時間前進窗口就滾動，
# `adj_factor` 重算也會改變還原價。實測 6243 在 11 天內從 11.6% 掉到 8.60%，那是預期。
# 原本階段 6 寫「應與 2026-08-06 完全相同」，那是不可檢驗的命題，已修正。
#
# 用法：
#   # 1. 先產出 evaluation report（約 12 分鐘 / 131 檔）
#   OUTPUT=/tmp/eval.json scripts/run-evaluation.sh --symbols <選池> --limit 1500
#   # 2. 比對
#   scripts/verify-regression-baseline.sh /tmp/eval.json
#
#   # 重建基準（pipeline 有預期中的行為改變時才做，並在 PR 說明為什麼）
#   REBUILD=1 SNAPSHOT='{"generated_at":"2026-09-01",...}' \
#     scripts/verify-regression-baseline.sh /tmp/eval.json
#
# 可覆寫的環境變數：
#   BASELINE      基準檔路徑（預設 python/baselines/sr_volatility_baseline.json）
#   SYMBOLS       重建時的標的清單，逗號分隔（預設取基準檔現有的 symbols）
#   SNAPSHOT      重建時的資料快照 JSON（**必填**，見下）
#   MIN_SPEARMAN  排名相關下限（預設 0.9）
#   REBUILD=1     重建基準而不是比對
#   PY_IMAGE / MEM  同 python/scripts/test.sh
#
# 設計重點：
#   - **基準放 git 而不是 DB**：基準的價值在於「改變時要被看見並被 review」，那正是
#     git diff 的語意。寫 `stock_sr_regression_results` 需要 --write-db（計畫書全程避免動
#     live），而 DB 裡的一列不會出現在 code review 上。
#   - **快照是必填**：基準檔要記下產生當時的最後交易日、每檔列數與 adj_factor 狀態，
#     否則下次比對無法判斷差異來自 pipeline 還是來自資料——那就是原定義踩到的坑。
#   - 這支不自己跑 evaluation：那要 12 分鐘與近 400MB，跟「比對」是兩件事，
#     分開才能重複比對同一份 report。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PYTHON_DIR="$REPO_ROOT/python"
IMAGE="${PY_IMAGE:-stock-trading-python-test:latest}"
MEM="${MEM:-300m}"
BASELINE="${BASELINE:-$PYTHON_DIR/baselines/sr_volatility_baseline.json}"
MIN_SPEARMAN="${MIN_SPEARMAN:-0.9}"

if [ $# -lt 1 ]; then
  echo "用法：$0 <evaluation-report.json>" >&2
  exit 2
fi
REPORT="$1"
[ -f "$REPORT" ] || { echo "ERROR: 找不到 report $REPORT" >&2; exit 1; }

cd "$REPO_ROOT"
# shellcheck source=lib/mem-guard.sh
. "$REPO_ROOT/scripts/lib/mem-guard.sh"
MEM="$(mem_guard_clamp "$MEM")"

REPORT_DIR="$(cd "$(dirname "$REPORT")" && pwd)"
DOCKER_ARGS=(
  --rm
  --user "$(id -u):$(id -g)"
  --memory="$MEM"
  --memory-swap="$MEM"
  --pids-limit=100
  -e HOME=/tmp
  -e PYTHONDONTWRITEBYTECODE=1
  -v "$PYTHON_DIR":/app
  -v "$REPORT_DIR":/report:ro
  -w /app
)
IN="/report/$(basename "$REPORT")"
REL_BASELINE="/app/baselines/$(basename "$BASELINE")"

echo "==> 建置 image：$IMAGE"
docker build -t "$IMAGE" "$PYTHON_DIR" >/dev/null

if [ "${REBUILD:-0}" = "1" ]; then
  if [ -z "${SNAPSHOT:-}" ]; then
    echo "ERROR: 重建基準必須提供 SNAPSHOT（資料快照 JSON）。" >&2
    echo "       少了它，下次比對無法判斷差異來自 pipeline 還是資料。" >&2
    exit 1
  fi
  SYMBOLS="${SYMBOLS:-$(python3 -c "import json,sys;print(','.join(json.load(open('$BASELINE'))['symbols']))")}"
  echo "==> 重建基準：$SYMBOLS"
  exec docker run "${DOCKER_ARGS[@]}" "$IMAGE" \
    python -m baseline_check build --report "$IN" \
      --symbols "$SYMBOLS" --snapshot "$SNAPSHOT" --output "$REL_BASELINE"
fi

echo "==> 比對回歸基準（唯讀）：min_spearman=$MIN_SPEARMAN mem=$MEM"
exec docker run "${DOCKER_ARGS[@]}" "$IMAGE" \
  python -m baseline_check compare --report "$IN" \
    --baseline "$REL_BASELINE" --min-spearman "$MIN_SPEARMAN"
