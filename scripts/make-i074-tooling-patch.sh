#!/usr/bin/env bash
# I-074 Stage 2：產生 tooling patch（`python/baselines/i074_stage2/tooling_e1cbbbd.patch`）。
#
# 為什麼需要它（issue.md I-074「Stage 2 步驟 ⑦ 總綱 v1」的「一之一」「二」）：⑩ 的 replay 容器掛的是
# **`e1cbbbd` worktree** 的 `python/`，⛔ 不是 HEAD——HEAD 對 `evaluation.py` 與 `replay_bundle/` 的改動
# （反事實路徑、串流 loader、`CounterfactualEffectCheck`…）只能經由 tooling patch 進入 replay。
# tooling patch ＝ `T1..T2` 的 canonical diff：
#   T1 ＝ `e1cbbbd` ＋ counterfactual（取自**來源 tree** 裡的那一份）
#   T2 ＝ T1，但 tooling 路徑（`evaluation.py`、`replay_bundle/`）換成**來源 tree** 的內容
#
# 用法（⛔ 不讀 HEAD、⛔ 不讀工作樹——來源一律是明確的 40 碼 tree OID）：
#   scripts/make-i074-tooling-patch.sh --source-tree <tree>   # 成功才把 patch bytes 印到 stdout
#   scripts/make-i074-tooling-patch.sh --verify <tree>        # 0＝該 tree 內的 patch 與重新產生的逐位元相同；1＝不同或缺檔
#
# 版控流程（⑦a～⑦d 只要動到 tooling 路徑就照做；見 docs/development-workflow.md）：
#   git add <程式>  →  T=$(git write-tree)  →  --source-tree "$T" > 暫存檔  →  放到上面的路徑並 git add
#   → review → commit → --verify "$(git rev-parse 'HEAD^{tree}')"
#
# 四條不變條件（⛔ 任一不成立即中止、stdout ⛔ 無輸出）：
#   ① counterfactual 的改動檔案與 tooling 路徑**交集為空**；
#   ② T2 在 tooling 路徑上 ＝ 來源 tree；
#   ③ T1..T2 只動到 tooling 路徑；
#   ④ **產品碼不變**：`python/` 扣掉 baselines／scripts／sr_scoring/tests／replay_bundle／evaluation.py 之後，
#      `e1cbbbd` → 來源 tree **沒有差異**——⛔ 否則 ⑩ 會靜默跑到舊版的產品碼。
# 最後以**唯一的合成函式** `replay_args_compose()` 重建一次自我驗證（T2 相同、raw ＝ canonical）。
#
# ⚠️ 被 source 時只定義常數與函式、⛔ 不執行（測試以刻意做壞的 tree 逐條驗不變條件）。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=lib/replay-args.sh
. "$REPO_ROOT/scripts/lib/replay-args.sh"

I074_TOOLING_BASE=e1cbbbdab44f8cf2d152e6ade9235d844f590d7f
I074_CF_PATCH_PATH=python/baselines/i074_stage2/counterfactual_e1cbbbd.patch
I074_TOOLING_PATCH_PATH=python/baselines/i074_stage2/tooling_e1cbbbd.patch
I074_TOOLING_EVAL=python/backtest/modular/sr_scoring/evaluation.py
I074_TOOLING_DIR=python/backtest/modular/sr_scoring/replay_bundle
# 產品碼不變條件要排除的路徑（其餘的 `python/` 必須與 base 完全相同）。
I074_PRODUCT_EXCLUDES=(python/baselines python/scripts python/backtest/modular/sr_scoring/tests
                       "$I074_TOOLING_DIR" "$I074_TOOLING_EVAL")

_mk_is_tooling_path() {
  [ "$1" = "$I074_TOOLING_EVAL" ] || [ "${1#"$I074_TOOLING_DIR"/}" != "$1" ]
}

_mk_is_tree() {
  [[ "$1" =~ ^[0-9a-f]{40}$ ]] && [ "$(git -C "$REPO_ROOT" cat-file -t "$1" 2>/dev/null)" = tree ]
}

# 某個 tree 在某路徑上的物件 OID（不存在時印空字串）。
_mk_oid_at() {
  git -C "$REPO_ROOT" rev-parse --verify --quiet "$1:$2" 2>/dev/null || true
}

# 四條不變條件。用法：mk_check_invariants <base_tree> <source_tree> <T1> <T2>
mk_check_invariants() {
  local base="$1" src="$2" t1="$3" t2="$4" names path excludes=() e
  # ①
  names="$(_replay_args_canonical names "$REPO_ROOT" "$base" "$t1")" || return 1
  while IFS= read -r path; do
    [ -n "$path" ] || continue
    if _mk_is_tooling_path "$path"; then
      echo "ERROR: ①：counterfactual 改到了 tooling 路徑 $path——兩份 patch 的範圍必須不相交。" >&2
      return 1
    fi
  done <<< "$names"
  # ②
  for path in "$I074_TOOLING_DIR" "$I074_TOOLING_EVAL"; do
    if [ "$(_mk_oid_at "$t2" "$path")" != "$(_mk_oid_at "$src" "$path")" ]; then
      echo "ERROR: ②：T2 在 $path 上 ≠ 來源 tree。" >&2
      return 1
    fi
  done
  # ③
  names="$(_replay_args_canonical names "$REPO_ROOT" "$t1" "$t2")" || return 1
  while IFS= read -r path; do
    [ -n "$path" ] || continue
    if ! _mk_is_tooling_path "$path"; then
      echo "ERROR: ③：T1..T2 改到了 tooling 路徑以外的 $path。" >&2
      return 1
    fi
  done <<< "$names"
  # ④
  for e in "${I074_PRODUCT_EXCLUDES[@]}"; do excludes+=(":(exclude)$e"); done
  names="$(_replay_args_canonical names "$REPO_ROOT" "$base" "$src" python "${excludes[@]}")" || return 1
  if [ -n "$names" ]; then
    echo "ERROR: ④：來源 tree 的產品碼與 ${I074_TOOLING_BASE:0:7} 不同——⑩ 會靜默跑到舊版（tooling patch 只帶得進" \
         "evaluation.py 與 replay_bundle/）：" >&2
    sed 's/^/  /' <<< "$names" >&2
    return 1
  fi
}

MK_WORKTREES=()
MK_TMP=""
_mk_cleanup() {
  local wt
  for wt in "${MK_WORKTREES[@]}"; do
    git -C "$REPO_ROOT" worktree remove --force "$wt" >/dev/null 2>&1 || true
    rm -rf "$wt"
  done
  [ -z "$MK_TMP" ] || rm -rf "$MK_TMP"
}

# ⚠️ ⛔ 不在 $(…) 裡呼叫：subshell 內登記的 worktree 不會回到清理清單。結果放在 MK_WT。
_mk_new_worktree() {
  MK_WT="$(mktemp -d)" && rmdir "$MK_WT" || return 1
  MK_WORKTREES+=("$MK_WT")
  replay_args_prepare_worktree "$REPO_ROOT" "$I074_TOOLING_BASE" "$MK_WT" >/dev/null || {
    echo "ERROR: 開不了 ${I074_TOOLING_BASE:0:7} 的 worktree。" >&2; return 1; }
}

# 對來源 tree 產生 tooling patch，寫到 $2。
mk_generate() {
  local src="$1" out="$2" base_tree cf empty compose t1 t2 line mode oid t1b t2b tool_sha
  base_tree="$(git -C "$REPO_ROOT" rev-parse "${I074_TOOLING_BASE}^{tree}")" || return 1
  cf="$MK_TMP/counterfactual.patch"; empty="$MK_TMP/empty.patch"
  git -C "$REPO_ROOT" cat-file blob "$src:$I074_CF_PATCH_PATH" > "$cf" 2>/dev/null || {
    echo "ERROR: 來源 tree 裡沒有 $I074_CF_PATCH_PATH。" >&2; return 1; }
  : > "$empty"

  _mk_new_worktree || return 1
  compose="$(replay_args_compose "$MK_WT" "$I074_TOOLING_BASE" "$cf" "$empty")" || return 1
  read -r t1 _ _ _ _ _ <<< "$compose"
  # tooling 路徑換成來源 tree 的內容（⚠️ 只動 index；工作樹與這一步無關）。
  git -C "$MK_WT" rm -r -q --cached --ignore-unmatch -- "$I074_TOOLING_EVAL" "$I074_TOOLING_DIR" || return 1
  if [ -n "$(_mk_oid_at "$src" "$I074_TOOLING_DIR")" ]; then
    git -C "$MK_WT" read-tree --prefix="$I074_TOOLING_DIR/" "$src:$I074_TOOLING_DIR" || return 1
  fi
  line="$(git -C "$REPO_ROOT" ls-tree "$src" -- "$I074_TOOLING_EVAL")" || return 1
  if [ -n "$line" ]; then
    read -r mode _ oid _ <<< "$line"
    git -C "$MK_WT" update-index --add --cacheinfo "$mode,$oid,$I074_TOOLING_EVAL" || return 1
  fi
  t2="$(git -C "$MK_WT" write-tree)" || return 1
  mk_check_invariants "$base_tree" "$src" "$t1" "$t2" || return 1
  replay_args_canonical_diff "$REPO_ROOT" "$t1" "$t2" > "$out" || return 1

  # 自我驗證：以唯一的合成函式從頭重建，T1／T2 必須相同、產出的 bytes 必須就是 canonical diff。
  _mk_new_worktree || return 1
  compose="$(replay_args_compose "$MK_WT" "$I074_TOOLING_BASE" "$cf" "$out")" || {
    echo "ERROR: 自我驗證：產出的 tooling patch 套不回去。" >&2; return 1; }
  read -r t1b t2b _ tool_sha _ _ <<< "$compose"
  if [ "$t1b" != "$t1" ] || [ "$t2b" != "$t2" ] || [ "$(sha256sum < "$out" | cut -d' ' -f1)" != "$tool_sha" ]; then
    echo "ERROR: 自我驗證：重建出的 tree 或 SHA 與產生時不同（T1 $t1b／$t1、T2 $t2b／$t2）。" >&2
    return 1
  fi
}

mk_main() {
  local mode="" tree=""
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --source-tree|--verify)
        [ -z "$mode" ] || { echo "ERROR: --source-tree 與 --verify 互斥、⛔ 不得重複。" >&2; return 1; }
        [ "$#" -ge 2 ] || { echo "ERROR: $1 需要 40 碼的 tree OID。" >&2; return 1; }
        mode="${1#--}"; tree="$2"; shift 2 ;;
      *) echo "ERROR: 未知參數 $1" >&2; return 1 ;;
    esac
  done
  [ -n "$mode" ] || { echo "用法: $0 --source-tree <tree> | --verify <tree>" >&2; return 1; }
  _mk_is_tree "$tree" || {
    echo "ERROR: 只接受 40 碼的 tree OID（⛔ commit、ref、HEAD）：$tree" >&2; return 1; }
  MK_TMP="$(mktemp -d)"
  mk_generate "$tree" "$MK_TMP/tooling.patch" || return 1
  if [ "$mode" = source-tree ]; then
    cat -- "$MK_TMP/tooling.patch"
    return 0
  fi
  git -C "$REPO_ROOT" cat-file blob "$tree:$I074_TOOLING_PATCH_PATH" > "$MK_TMP/committed.patch" 2>/dev/null || {
    echo "ERROR: --verify：tree $tree 裡沒有 $I074_TOOLING_PATCH_PATH。" >&2; return 1; }
  if cmp -s "$MK_TMP/committed.patch" "$MK_TMP/tooling.patch"; then
    echo "OK: $I074_TOOLING_PATCH_PATH 與對 $tree 重新產生的結果逐位元相同。"
    return 0
  fi
  echo "ERROR: --verify：$I074_TOOLING_PATCH_PATH 與對 $tree 重新產生的結果不同——tooling patch 漂移了，" \
       "要依版控流程重新產生並 stage。" >&2
  return 1
}

if [ "${BASH_SOURCE[0]}" = "$0" ]; then
  trap _mk_cleanup EXIT
  mk_main "$@"
fi
