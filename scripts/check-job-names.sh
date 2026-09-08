#!/usr/bin/env bash
# 前端的 job 清單是否與後端的 knownSchedulerJobs **完全一致**（雙向）。
#
# ⚠️ **要解的問題**：這是**每加一支排程就會再發生一次**的漂移（原 docs/issue.md I-110）。
# 2026-09-08 發現前端的 `JobName` union 少了 candle_gap_detection / sr_analysis /
# sr_analysis_chip，其中兩支在 live 是開著的——`jobLabel` 也少了後兩支，
# 所以排程頁直接把 `sr_analysis` 這種原始字串渲染出來。
#
# ⛔ **必須是雙向集合比較，不能只查「後端每一項有沒有出現在前端」**（2026-09-08 review）：
#   * 單向查漏掉「前端留著後端已經移除的舊 job」——那會在畫面上留一列永遠 disabled 的殭屍；
#   * 而且「有沒有出現」要比對**實際的 union 成員與 object key**，不能整份檔案 grep：
#     job 名稱也會出現在**註解**裡（例如「那是 stock_symbol_sync 的權責」），
#     從 union 移除之後照樣能通過。
#
# ⛔ **兩份都要驗**：union 少了只是型別騙人（label 表是 Record<string, string>，
# runtime 照常渲染），但 `jobLabel` 少了**畫面上看得到**。
#
# **為什麼是 shell 而不是單元測試**：這道檢查跨兩個語言的原始碼，而
# backend/scripts/test.sh 只掛載 backend/（讀不到 frontend/）；前端沒有 @types/node，
# vitest 裡讀檔要多裝相依。前端腳本掛的是 repo root，兩邊都讀得到。
# 比照 check-dist-assets.sh，由 frontend/scripts/test.sh 在最後呼叫。
#
# 真正的修法是讓型別從後端產生，但那需要一套產生流程；在那之前用這道檢查擋住。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GO_SRC="$REPO_ROOT/backend/internal/api/handler/scheduler.go"
TS_SRC="$REPO_ROOT/frontend/src/lib/api/scheduler.ts"
SVELTE_SRC="$REPO_ROOT/frontend/src/routes/Scheduler.svelte"

for f in "$GO_SRC" "$TS_SRC" "$SVELTE_SRC"; do
  if [ ! -f "$f" ]; then
    echo "ERROR: 找不到 $f（檔案搬家的話要同步改這支腳本）" >&2
    exit 1
  fi
done

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# ── 後端：knownSchedulerJobs 的字面值 ──────────────────────────────────
grep -oP 'var knownSchedulerJobs = \[\]string\{\K[^}]+' "$GO_SRC" \
  | grep -oP '"\K[a-z_]+' | sort -u > "$tmp/backend"

# ── 前端①：JobName union 的**實際成員**（`  | 'name'` 那種行）───────────
# ⛔ 只取 `export type JobName =` 到第一個空行為止，且只認 union 行——
# 這樣註解裡的 job 名稱不會被算成成員。
awk '/^export type JobName =/{f=1; next} f && /^$/{exit} f' "$TS_SRC" \
  | grep -oP "^\s*\|\s*'\K[a-z_]+" | sort -u > "$tmp/union"

# ── 前端②：jobLabel 的**實際 object key**（`    name: '…'`）────────────
# ⛔ 同樣只取那個物件字面值的區塊：觸發按鈕的 `job.job_name === '…'`
# 與區塊內的註解都不算 key。
awk '/const jobLabel/{f=1; next} f && /^  \}/{exit} f' "$SVELTE_SRC" \
  | grep -oP "^\s{4}\K[a-z_]+(?=:)" | sort -u > "$tmp/labels"

for f in backend union labels; do
  if [ ! -s "$tmp/$f" ]; then
    echo "ERROR: 解析不出 $f 的清單（宣告形式變了？）" >&2
    exit 1
  fi
done

fail=0
report() { # report <來源> <缺少的檔案> <多出的檔案>
  local what="$1" missing extra
  missing="$(comm -23 "$tmp/backend" "$tmp/$2" || true)"
  extra="$(comm -13 "$tmp/backend" "$tmp/$2" || true)"
  if [ -n "$missing" ]; then
    echo "ERROR: 前端 $what 少了：$(echo $missing)" >&2
    fail=1
  fi
  if [ -n "$extra" ]; then
    echo "ERROR: 前端 $what 多出後端沒有的：$(echo $extra)" >&2
    echo "       （後端移除排程時前端也要跟著移，否則畫面會留一列永遠 disabled 的殭屍）" >&2
    fail=1
  fi
}

report "JobName union（frontend/src/lib/api/scheduler.ts）" union
report "jobLabel（frontend/src/routes/Scheduler.svelte）" labels

if [ "$fail" = "1" ]; then
  echo "" >&2
  echo "後端 knownSchedulerJobs 目前是：" >&2
  sed 's/^/  /' "$tmp/backend" >&2
  exit 1
fi

echo "==> [job-names] 前端 union 與 label 與後端的 $(wc -l < "$tmp/backend") 個 job 完全一致"
