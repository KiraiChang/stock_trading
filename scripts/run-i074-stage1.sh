#!/usr/bin/env bash
# I-074 Stage 1 的 **orchestrator**：跑 crossday，然後**無論結果都**跑 finalizer。
#
# ⚠️ **為什麼需要它**：`compare-replay-crossday.sh` 一回 5，普通的 `set -e` 流程就停了，
# finalizer 永遠不會執行——而 **mismatch 正是最需要保存證據的情況**。
#
# 流程定死：
#   1. 捕捉 crossday 的 exit code（⛔ 不讓 set -e 直接中斷）
#   2. **只接受 0 或 5**——其餘一律視為失敗，⛔ 不執行 finalizer
#   3. 執行 finalizer
#   4. finalizer 成功 → **回傳原始的 0 或 5**
#   5. finalizer 一般失敗 → 回 1
#   6. ⚠️ finalizer 回 **3（durability 未確認）→ 回 3**，**優先於**原始的 0／5——
#      證據已發布但落盤未確認是**必須被看見**的狀態，⛔ 不得被 MATCH 的 0 蓋掉
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
EXIT_DURABILITY_UNCONFIRMED=3
EXIT_CROSSDAY_MISMATCH=5

D=""; D1=""; CROSSDAY_OUT=""; ROOT=""; SOURCES=()
while [ "$#" -gt 0 ]; do
  case "$1" in
    --d) D="$2"; shift 2 ;;
    --d1) D1="$2"; shift 2 ;;
    --crossday-output-dir) CROSSDAY_OUT="$2"; shift 2 ;;
    --evidence-root) ROOT="$2"; shift 2 ;;
    --source) SOURCES+=("$2"); shift 2 ;;
    *) echo "ERROR: 未知參數 $1" >&2; exit 1 ;;
  esac
done
[ -n "$D" ] && [ -n "$D1" ] && [ -n "$CROSSDAY_OUT" ] && [ -n "$ROOT" ] || {
  echo "用法: $0 --d <f> --d1 <f> --crossday-output-dir <dir> --evidence-root <dir> --source rel=path …" >&2
  exit 1; }

# ── ⓪ **先**擋掉被綁定來源的覆寫 ───────────────────────────────────────────
#
# ⚠️ 這一關⛔ **必須排在 comparator 之前**：放在後面的話，錯誤的呼叫會**先產出 crossday
# artifact**，而 comparator 若回 5，流程又因為覆寫參數回 1 → **跳過 finalizer**，
# ⛔ 那就掩蓋掉了 mismatch，直接違反「mismatch 必須封存」這條核心政策。
for item in "${SOURCES[@]}"; do
  case "${item%%=*}" in
    d/after_artifact.json.gz|d1/after_artifact.json.gz|crossday/crossday_artifact.json.gz)
      echo "ERROR: ${item%%=*} 由 orchestrator 綁定本次執行的產物，⛔ 不接受 --source 覆寫。" >&2
      echo "       ⚠️ 此時⛔ 尚未執行 comparator——⛔ 不會留下任何產物。" >&2
      exit 1 ;;
  esac
done

# ── ① crossday（⛔ 不讓 set -e 中斷）────────────────────────────────────────
set +e
"$REPO_ROOT/scripts/compare-replay-crossday.sh" --d "$D" --d1 "$D1" --output-dir "$CROSSDAY_OUT"
CROSSDAY_RC=$?
set -e

# ── ② 只接受 0 或 5 ────────────────────────────────────────────────────────
if [ "$CROSSDAY_RC" -ne 0 ] && [ "$CROSSDAY_RC" -ne "$EXIT_CROSSDAY_MISMATCH" ]; then
  echo "ERROR: crossday 以 rc=$CROSSDAY_RC 結束（⛔ 不是 0 或 5）——⛔ 不執行 finalizer。" >&2
  exit "$CROSSDAY_RC"
fi
echo "==> crossday rc=$CROSSDAY_RC；⚠️ mismatch 也是證據，照樣 finalize。" >&2

# ── ③ finalizer ───────────────────────────────────────────────────────────
# ⚠️ **這三份由 orchestrator 自己綁**，⛔ 不接受使用者提供：
# ⛔ 否則可以「comparator 比較本次 D／D+1、finalizer 卻封存另一組合法但**舊**的產物」，
# 最終 exit code 與 evidence 包描述的⛔ 不是同一次比較——而全圖 validator 只證明得了
# `--source` 內部自洽，⛔ 無從知道它是不是剛剛那一次的結果。
BOUND_SOURCES=(
  "d/after_artifact.json.gz=$D"
  "d1/after_artifact.json.gz=$D1"
  "crossday/crossday_artifact.json.gz=$CROSSDAY_OUT/crossday_artifact.json"
)
FINAL_ARGS=(--evidence-root "$ROOT")
for item in "${BOUND_SOURCES[@]}"; do FINAL_ARGS+=(--source "$item"); done
# ⚠️ 覆寫的檢查已在 ⓪ 做完（⛔ 必須在 comparator 之前），這裡只組參數。
for item in "${SOURCES[@]}"; do FINAL_ARGS+=(--source "$item"); done
set +e
"$REPO_ROOT/scripts/finalize-evidence.sh" "${FINAL_ARGS[@]}"
FINAL_RC=$?
set -e

# ── ④⑤⑥ 結束碼仲裁 ────────────────────────────────────────────────────────
if [ "$FINAL_RC" -eq "$EXIT_DURABILITY_UNCONFIRMED" ]; then
  # ⚠️ **優先於原始的 0／5**：⛔ 不得被 MATCH 的 0 蓋掉。
  echo "ERROR: evidence 已發布但 durability 未確認——⛔ 不刪除，用 --recover-durability 修復。" >&2
  exit "$EXIT_DURABILITY_UNCONFIRMED"
fi
if [ "$FINAL_RC" -ne 0 ]; then
  echo "ERROR: finalizer 失敗（rc=$FINAL_RC）。" >&2
  exit 1
fi
exit "$CROSSDAY_RC"
