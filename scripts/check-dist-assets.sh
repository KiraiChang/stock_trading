#!/usr/bin/env bash
# 確認 backend/internal/ui/dist 沒有未納入版控的檔案，且 index.html 引用的 asset 都存在。
#
# 用法：
#   scripts/check-dist-assets.sh          # 檢查，有問題回非零（預設）
#   scripts/check-dist-assets.sh --fix    # 直接 git add dist（明確要求時才用）
#
# 為什麼需要這道檢查（2026-08-10；規則本身見 docs/development-workflow.md 的提交前檢查清單）：
#   `backend/internal/ui/ui.go` 用 `//go:embed all:dist` 把整個 dist 打包進執行檔，所以
#   dist 是**刻意納入版控**的。而 vite 每次 build 都會產生新的 content hash 檔名，
#   於是每次前端有變更都會出現「index.html 改了、但新 bundle 還是 untracked」的狀態；
#   此時 `git commit -a` 只會帶走 index.html 與舊檔的刪除，做出一個 index.html 指向
#   不存在檔案的 commit——SPA 整頁空白，而且 build 與測試全都會過，沒有任何東西會失敗。
#
#   `development-workflow.md` 的提交前檢查清單**早就有**「重新 build dist 並 git add」這一條，
#   但 2026-08-10 還是漏了一次。這說明問題不在缺少規則，而在漏掉時沒有東西會失敗。
#
#   **注意：這件事沒辦法用 Go 測試檢查。** `//go:embed` 讀的是磁碟，不是 git——
#   本機磁碟上檔案存在，embed 就成功，測試照樣綠。必須是 git-aware 的檢查。
#
# 為什麼檢查「整個 dist 有無 untracked」而不是只比對 index.html 的引用：
#   目前 app 沒有動態 `import()`，所有 chunk 都被 index.html 的 modulepreload 列出，
#   兩種做法等價。但只要有人加了 route-level 的 lazy import，那個 chunk **不會**出現在
#   index.html 裡，只比對引用就會放行一個未納入版控的 lazy chunk——正是本腳本要防的 bug。
#   直接看「dist 底下有沒有 untracked」嚴格更強，也順便涵蓋舊 bundle 的刪除是否已被 git 記錄。
#
# 為什麼預設是檢查而不是自動修（2026-08-10 review 後改）：
#   自動 `git add` 會讓這支腳本永遠不失敗，等於沒有檢查；而且它會在使用者不知情的狀況下
#   動到 git index——例如跑完測試後放棄該次前端改動（`git checkout -- frontend/src`）卻沒有
#   重 build，那批 dist 會留在 staged 狀態，之後一個不相干的 commit 就會夾帶對不上原始碼的
#   bundle，且因為不再顯示為 `??` 而更難在 `git status` 察覺。所以預設只檢查、明確要求才修。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_REL="backend/internal/ui/dist"
DIST_DIR="$REPO_ROOT/$DIST_REL"
INDEX_HTML="$DIST_DIR/index.html"
FIX=0
[ "${1:-}" = "--fix" ] && FIX=1

if [ ! -f "$INDEX_HTML" ]; then
  echo "ERROR: 找不到 $INDEX_HTML——dist 還沒 build？" >&2
  exit 1
fi

if ! git -C "$REPO_ROOT" rev-parse --git-dir >/dev/null 2>&1; then
  echo "==> [dist] 不在 git repo 內，略過版控檢查"
  exit 0
fi

# ── 1. index.html 引用的 asset 必須存在於磁碟（build 不完整會在這裡被抓到）──────
mapfile -t ASSETS < <(grep -oE '(src|href)="/assets/[^"]+"' "$INDEX_HTML" \
  | sed -E 's/.*"\/assets\/([^"]+)"/\1/' | sort -u)

if [ "${#ASSETS[@]}" -eq 0 ]; then
  echo "ERROR: index.html 沒有引用任何 /assets/ 檔案，格式可能變了，請檢查此腳本的解析" >&2
  exit 1
fi

missing_on_disk=()
for asset in "${ASSETS[@]}"; do
  [ -f "$DIST_DIR/assets/$asset" ] || missing_on_disk+=("assets/$asset")
done
if [ "${#missing_on_disk[@]}" -gt 0 ]; then
  echo "ERROR: index.html 引用的檔案不存在於磁碟（dist 不一致，請重新 build）：" >&2
  printf '    %s\n' "${missing_on_disk[@]}" >&2
  exit 1
fi

# ── 2. dist 底下不得有未納入版控的檔案 ──────────────────────────────────
mapfile -t UNTRACKED < <(git -C "$REPO_ROOT" ls-files --others --exclude-standard -- "$DIST_REL")

if [ "${#UNTRACKED[@]}" -eq 0 ]; then
  echo "==> [dist] 無未納入版控的檔案（index.html 引用的 ${#ASSETS[@]} 個 asset 皆存在）"
  exit 0
fi

if [ "$FIX" = "1" ]; then
  echo "==> [dist] 以下檔案尚未納入版控，依要求自動 git add："
  printf '    %s\n' "${UNTRACKED[@]}"
  git -C "$REPO_ROOT" add -- "$DIST_REL"
  echo "==> [dist] 已 stage。提交前請確認舊 bundle 的刪除也在這次 commit 裡。"
  exit 0
fi

cat >&2 <<EOF
ERROR: $DIST_REL 底下有未納入版控的檔案：
$(printf '    %s\n' "${UNTRACKED[@]}")

    dist 依設計要進版控（ui.go 的 //go:embed all:dist）。這樣 commit 會做出
    index.html 指向不存在檔案的前端，而所有測試仍然會過。

    修正：git add $DIST_REL
    （或 scripts/check-dist-assets.sh --fix）
EOF
exit 1
