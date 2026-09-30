#!/usr/bin/env bash
# I-100：Stage 0 與離線 replay 的**共用 argv builder ＋ 參數所有權驗證**。
#
# 為什麼要抽出來：計畫書要求「CLI 衝突測試必須用**官方腳本實際組出的完整 argv** 跑一次」。
# 兩支腳本各自拼一份的話，測試驗到的就不是實際組法。所以**唯一的組裝入口在這裡**，
# `run-evaluation.sh`、`run-replay-offline.sh` 與 `scripts/test-replay-args.sh` 都呼叫它。
#
# ⛔ 這六個參數只能由官方腳本注入，使用者傳入即拒絕：
#   --image-digest / --base-commit / --tooling-patch-sha256 / --source-root / --runner-sha256 /
#   --counterfactual-patch-sha256
#   （--runner-sha256 於 v23 第一輪 review 補上：runner 是容器外的 shell 腳本，
#     容器內的 Python 讀不到它，hash 只能由這一側算好後注入。
#     --counterfactual-patch-sha256 是 I-074 Stage 2 ⑦a 加的：反事實 patch 的 canonical SHA，
#     由 runner 從凍結副本推導；⚠️ 模式旗標 --i074-counterfactual 是使用者 opt-in，⛔ 不在這裡。）
# 理由：它們是 provenance 的事實來源。使用者能傳同名參數，就能寫出與實際執行不符的
# provenance。⚠️ **驗證只能發生在 shell 層**——容器內的 Python 無法自己
# `docker image inspect` 得知自己跑在哪個 image；CLI 那一端只負責對**重複值**中止。

REPLAY_INJECTED_ARGS=(--image-digest --base-commit --tooling-patch-sha256 --source-root --runner-sha256
                      --counterfactual-patch-sha256)

# 使用者參數裡不得出現任何注入參數。
#
# ⛔ **連「唯一前綴縮寫」都要擋**：argparse 預設接受縮寫，`--image-d` 會被展開成
# `--image-digest`。只比完整名稱與 `--name=value` 的話，縮寫形式會整條穿過去，
# 而官方注入的值排在使用者參數之前——argparse 取最後一個，使用者傳的那個就贏了。
# （Python 端另外用 `allow_abbrev=False` 把縮寫關掉，兩層都要有：這一層給出的是
# 「⛔ 不接受使用者傳入」這個明確理由，而不是一句 unrecognized arguments。）
replay_args_reject_injected() {
  local arg name injected
  for arg in "$@"; do
    # 只看長選項；`--name=value` 先切掉值的部分。
    case "$arg" in --*) name="${arg%%=*}" ;; *) continue ;; esac
    [ ${#name} -ge 3 ] || continue
    for injected in "${REPLAY_INJECTED_ARGS[@]}"; do
      # name 是 injected 的前綴（含完全相同）即拒絕。
      if [ "${injected#"$name"}" != "$injected" ] || [ "$name" = "$injected" ]; then
        echo "ERROR: $name 只能由官方腳本注入（會被展開成 $injected），⛔ 不接受使用者傳入。" >&2
        echo "       它是 provenance 的事實來源；讓使用者填等於允許偽造執行身分。" >&2
        return 1
      fi
    done
  done
  return 0
}

# 官方腳本自己的內容指紋：**腳本本身 ＋ 這個 lib**。
# ⚠️ 容器內的 Python 讀不到 scripts/，所以只能由這一側算好後注入。
replay_args_runner_sha256() {
  local script="$1"
  local lib="${BASH_SOURCE[0]}"
  cat "$script" "$lib" | sha256sum | cut -d' ' -f1
}

# Stage 0 的完整指令 token（一行一個）。
#   用法：replay_args_stage0 <model_path> <image_digest> [使用者參數...]
replay_args_stage0() {
  local model_path="$1"; shift
  local image_digest="$1"; shift
  local runner_sha256="$1"; shift
  printf '%s\n' python -m backtest.modular.sr_scoring.evaluation \
    --model-path "$model_path" --image-digest "$image_digest" \
    --runner-sha256 "$runner_sha256" "$@"
}

# Stage 1／2（離線）的完整指令 token（一行一個）。
# ⛔ 這裡**不注入 --model-path**：模型是 bundle 的一部分，從 bundle 載入。
# ⚠️ 第 6 個參數是反事實 patch 的 canonical SHA：**空字串＝不注入**（一般路徑）；非空時排在
# `--runner-sha256` 之後注入 `--counterfactual-patch-sha256`（I-074 Stage 2 ⑦a）。
#   用法：replay_args_offline <image_digest> <source_root> <base_commit> <tooling_patch_sha256>
#                             <runner_sha256> <counterfactual_sha256|""> [使用者參數...]
replay_args_offline() {
  local image_digest="$1"; shift
  local source_root="$1"; shift
  local base_commit="$1"; shift
  local tooling_patch="$1"; shift
  local runner_sha256="$1"; shift
  local counterfactual_sha256="$1"; shift
  local injected=(--image-digest "$image_digest" --source-root "$source_root"
                  --base-commit "$base_commit" --tooling-patch-sha256 "$tooling_patch"
                  --runner-sha256 "$runner_sha256")
  if [ -n "$counterfactual_sha256" ]; then
    injected+=(--counterfactual-patch-sha256 "$counterfactual_sha256")
  fi
  printf '%s\n' python -m backtest.modular.sr_scoring.evaluation "${injected[@]}" "$@"
}

# 使用者參數裡有沒有成對的 --after-artifact ＋ --cohort-manifest（＝Stage 2）。
# ⚠️ 只給其中一個時回 3——腳本要在建 worktree、跑 docker 之前就中止。
replay_args_offline_stage() {
  local arg has_after=0 has_cohort=0
  for arg in "$@"; do
    case "${arg%%=*}" in
      --after-artifact) has_after=1 ;;
      --cohort-manifest) has_cohort=1 ;;
    esac
  done
  if [ "$has_after" = 1 ] && [ "$has_cohort" = 1 ]; then
    printf '2\n'
  elif [ "$has_after" = 0 ] && [ "$has_cohort" = 0 ]; then
    printf '1\n'
  else
    printf '3\n'
  fi
}

# ⛔ **會影響 checkout 或掛載的參數不得重複**。
#
# ⚠️ 這不是潔癖，是一個真的會產生「報告與實際不符」的洞：本檔的 `replay_args_value_of`
# 取**第一個**值（腳本拿它 checkout、拿它決定掛哪個目錄），而 argparse 取**最後一個**
# （Python 拿它寫進 artifact）。`--before-ref A --before-ref B` 於是變成
# 「實際跑 A、報告寫 B」——正好破壞 I-100 要保護的版本身分。
REPLAY_SINGLE_VALUE_ARGS=(--before-ref --bundle --output-dir --after-artifact --cohort-manifest)

replay_args_reject_duplicates() {
  local arg name candidate count
  for candidate in "${REPLAY_SINGLE_VALUE_ARGS[@]}"; do
    count=0
    for arg in "$@"; do
      case "$arg" in --*) name="${arg%%=*}" ;; *) continue ;; esac
      [ "$name" = "$candidate" ] && count=$((count + 1))
    done
    if [ "$count" -gt 1 ]; then
      echo "ERROR: $candidate 出現 $count 次——⛔ 不接受重複。" >&2
      echo "       腳本取第一個值去 checkout／掛載，argparse 取最後一個值寫進 artifact，" >&2
      echo "       重複會讓「實際執行的版本」與「報告宣稱的版本」不同。" >&2
      return 1
    fi
  done
  return 0
}

# 取出某個長選項的值（支援 `--name value` 與 `--name=value`）。
replay_args_value_of() {
  local wanted="$1"; shift
  local i args=("$@")
  for ((i = 0; i < ${#args[@]}; i++)); do
    case "${args[$i]}" in
      "$wanted") printf '%s\n' "${args[$((i + 1))]:-}"; return 0 ;;
      "$wanted"=*) printf '%s\n' "${args[$i]#*=}"; return 0 ;;
    esac
  done
  return 1
}

# 使用者參數裡有沒有 --emit-bundle（＝Stage 0）。
replay_args_is_stage0() {
  local arg
  for arg in "$@"; do
    if [ "$arg" = "--emit-bundle" ] || [ "${arg#--emit-bundle=}" != "$arg" ]; then
      return 0
    fi
  done
  return 1
}

# ── before worktree 與 tooling patch（I-100 八） ─────────────────────────────
#
# ⛔ `base_commit` 的取得順序必須固定，否則有 TOCTOU：
#   ① ref → immutable OID  ② 用該 OID 建 detached worktree（⛔ 不是用 branch 名建）
#   ③ 從 worktree 讀 HEAD   ④ 斷言 head == oid，不符即中止
# 少了 ①③④，branch 在 worktree 建立後被移動時，記進 manifest 的 commit 可能已經不是
# 實際執行的那份程式碼。
#   用法：replay_args_prepare_worktree <repo_root> <before_ref> <worktree_path>  → 印出 OID
replay_args_prepare_worktree() {
  local repo_root="$1" before_ref="$2" worktree="$3" oid head_oid
  oid="$(git -C "$repo_root" rev-parse "${before_ref}^{commit}")" || return 1
  git -C "$repo_root" worktree add --detach "$worktree" "$oid" >/dev/null || return 1
  head_oid="$(git -C "$worktree" rev-parse HEAD)" || return 1
  if [ "$head_oid" != "$oid" ]; then
    echo "ERROR: worktree 的 HEAD（$head_oid）與解析出的 OID（$oid）不符——⛔ 中止。" >&2
    return 1
  fi
  printf '%s\n' "$oid"
}

# ⚠️ **只移除「登記路徑解析後確實位於 <dir> 之內」的 worktree**，<dir> 以外⛔ 一律不動。
#   用法：replay_args_remove_worktrees_under <repo_root> <dir>   → 0＝範圍內的都移除了；1＝任何一步失敗
# 給 smoke 與測試用：runner 走到 `exec docker run` 時 EXIT trap 不會執行，worktree 會留下（I-118 的既有成因）。
# ⛔ **不可改用「執行前後的登記差集」**（⑦a 實作第一輪 review）：同一段時間內使用者或其他程序建立的
# worktree 也會出現在差集裡而被誤刪。呼叫端要讓 runner 以**本次專屬、新建**的 `TMPDIR` 執行（runner 的
# worktree 來自 `mktemp -d`），再只清這個目錄底下的登記。
# ⚠️ **⛔ 不吞錯誤**（第二輪 review）：`worktree list` 失敗、`worktree remove` 失敗都回 1；remove 失敗時
# ⛔ 不再 `rm -rf` 實體目錄（否則會留下 stale 的登記，卻回報成功）。
replay_args_remove_worktrees_under() {
  local repo="$1" dir listing wt parent real rc=0
  dir="$(cd "$2" 2>/dev/null && pwd -P)" || { echo "ERROR: 清理範圍 $2 不存在。" >&2; return 1; }
  # ⚠️ 先取成變數再檢查結束碼——⛔ 不放在 process substitution 裡（失敗時 while 只會收到空輸入）。
  if ! listing="$(git -C "$repo" worktree list --porcelain)"; then
    echo "ERROR: 讀不到 $repo 的 worktree 清單——⛔ 無法確認要清哪些。" >&2
    return 1
  fi
  while IFS= read -r wt; do
    [ -n "$wt" ] || continue
    parent="$(cd "$(dirname "$wt")" 2>/dev/null && pwd -P)" || continue   # 上層已不存在 → ⛔ 不可能在範圍內
    real="$parent/$(basename "$wt")"
    case "$real" in
      "$dir"/*) ;;
      *) continue ;;                                                     # 範圍外：⛔ 不動
    esac
    if git -C "$repo" worktree remove --force "$wt" >/dev/null; then
      rm -rf -- "$wt"
    else
      echo "ERROR: 移除 worktree 登記失敗：$wt——⛔ 不刪實體目錄（避免留下 stale 登記卻回報成功）。" >&2
      rc=1
    fi
  done < <(sed -n 's/^worktree //p' <<< "$listing")
  return "$rc"
}

# 某個路徑在 git 裡是否乾淨。
#   用法：replay_args_path_clean <repo_root> <path>   → 0＝乾淨；1＝有未提交的改動或 untracked；2＝`git status` 本身失敗
# ⚠️ **⛔ 不可寫成 `[ -z "$(git status …)" ]`**（⑦a 實作第二輪 review）：`git status` 失敗且沒有 stdout 時，
# 那個條件照樣成立——讀不到狀態被當成「乾淨」。這裡把「指令失敗」與「輸出非空」分開處理。
replay_args_path_clean() {
  local repo="$1" path="$2" out
  if ! out="$(git -C "$repo" status --porcelain=v1 --untracked-files=all -- "$path")"; then
    echo "ERROR: 讀不到 $path 的 git status——⛔ 不當成乾淨。" >&2
    return 2
  fi
  if [ -n "$out" ]; then
    echo "ERROR: $path 有未提交的改動或 untracked 檔案：" >&2
    printf '%s\n' "$out" >&2
    return 1
  fi
  return 0
}

# ── canonical diff 與唯一的合成函式（I-074 Stage 2 ⑦ 總綱 v1「二」、⑦a 細部計畫 B1） ────────
#
# ⚠️ **這裡是全 repo 唯一的定義**：runner、`finalize-stage2-evidence.sh` 的合成守門與
# `--check-failed-record`、tooling patch 產生器、Stage 1 的 finalizer／comparator 自己的 provenance
# 都呼叫這幾支，⛔ 不各寫一份（兩套推導＝兩套語意）。

# 反事實 patch 的**語意鍵**只涵蓋這兩個產品檔（正向白名單，⛔ 不用排除式 pathspec）。
I074_CF_PRODUCT_PATHS=(
  python/backtest/modular/sr_scoring/decision_engine.py
  python/backtest/modular/sr_scoring/lifecycle_engine.py
)
# 反事實 patch 改動的檔案集合**恰好**是這四個（C locale 排序）；多一個、少一個一律拒絕。
# ⚠️ 集合是常數——要改就改計畫（⑦ 總綱 v1 決策表第 9 列）。
I074_CF_FILES=(
  python/backtest/modular/sr_scoring/decision_engine.py
  python/backtest/modular/sr_scoring/lifecycle_engine.py
  python/backtest/modular/sr_scoring/tests/test_i074_diagnostics.py
  python/backtest/modular/sr_scoring/tests/test_lifecycle_engine.py
)
EMPTY_SHA256=e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855

# canonical diff 的核心。$1＝模式（diff／sha／names）；$2＝來源 repo 的任一目錄（worktree 或 repo 本身）；
# $3、$4＝兩個 **40 碼 tree OID**；其餘＝固定 pathspec（可省略）。
#
# ⚠️ **為什麼在暫存 bare repo 算**：`--full-index` 之外，git config（`diff.noprefix`、`diff.context`、
# `diff.renames`、`diff.orderFile`、`core.quotePath`）與屬性（`diff=<driver>`、`-diff`）都會改變 bytes，
# 而且工作樹的 `.gitattributes` 與 `info/attributes` ⛔ 無法以 `-c` 關掉；host 的 git 2.30 也⛔ 沒有
# `GIT_CONFIG_GLOBAL`。暫存 bare repo 只以 alternates 借來源 repo 的物件：沒有工作樹、沒有 `info/`，
# local config 是它自己的；`env -i` 清掉全部 `GIT_*` 與 `GIT_CONFIG_PARAMETERS`，HOME／XDG 指向空目錄、
# 關掉 system config 與 system attributes。下面釘死的參數是第二層防線。
# ⚠️ 用 subshell ＋ EXIT trap 收掉暫存目錄；⛔ 不依賴 `set -e`（在 `if`／`$(…)` 裡會被關掉）。
_replay_args_canonical() {
  local mode="$1" src="$2" a="$3" b="$4"; shift 4
  (
    tmp="$(mktemp -d)" || exit 1
    trap 'rm -rf "$tmp"' EXIT
    mkdir "$tmp/home" || exit 1
    iso() {
      env -i PATH="$PATH" HOME="$tmp/home" XDG_CONFIG_HOME="$tmp/home" \
        GIT_CONFIG_NOSYSTEM=1 GIT_ATTR_NOSYSTEM=1 git "$@"
    }
    local common_rel common oid
    common_rel="$(cd "$src" && iso rev-parse --git-common-dir)" || {
      echo "ERROR: canonical diff：$src 不是 git repo。" >&2; exit 1; }
    # ⚠️ 2.30 ⛔ 沒有 `rev-parse --path-format`：相對路徑以 `cd` ＋ `pwd -P` 轉成絕對路徑。
    common="$(cd "$src" && cd "$common_rel" && pwd -P)" || exit 1
    iso init -q --bare --template= "$tmp/repo.git" || exit 1
    mkdir -p "$tmp/repo.git/objects/info" || exit 1
    printf '%s\n' "$common/objects" > "$tmp/repo.git/objects/info/alternates" || exit 1
    for oid in "$a" "$b"; do
      if ! [[ "$oid" =~ ^[0-9a-f]{40}$ ]] || [ "$(iso --git-dir="$tmp/repo.git" cat-file -t "$oid" 2>/dev/null)" != tree ]; then
        echo "ERROR: canonical diff 只接受 40 碼的 tree OID（⛔ commit、ref、工作樹）：$oid" >&2
        exit 1
      fi
    done
    local pathspec=()
    [ "$#" -eq 0 ] || pathspec=(-- "$@")
    local base_args=(--git-dir="$tmp/repo.git" -c core.quotePath=true -c diff.suppressBlankEmpty=false
                     -c core.attributesFile=/dev/null)
    case "$mode" in
      diff|sha)
        local out="$tmp/diff"
        # ⚠️ **唯一定義**（⑦ 總綱 v1「二」的 canonical diff 列）——參數一個都不得增減。
        iso "${base_args[@]}" diff --binary --full-index --no-ext-diff --no-textconv --no-color \
          --src-prefix=a/ --dst-prefix=b/ -U3 --inter-hunk-context=0 --diff-algorithm=myers --no-renames \
          --indent-heuristic -O/dev/null --no-relative "$a" "$b" "${pathspec[@]}" > "$out" || {
          echo "ERROR: canonical diff 失敗（$a..$b）。" >&2; exit 1; }
        if [ "$mode" = diff ]; then
          cat -- "$out"
        else
          sha256sum < "$out" | cut -d' ' -f1
        fi ;;
      names)
        # 改動的檔案集合：`-z` 之後 quotePath 無關；換行分隔、C locale 排序。
        iso "${base_args[@]}" diff --name-only -z --no-renames --no-relative "$a" "$b" "${pathspec[@]}" \
          > "$tmp/names" || { echo "ERROR: 取不到 $a..$b 的改動檔案。" >&2; exit 1; }
        tr '\0' '\n' < "$tmp/names" | LC_ALL=C sort ;;
      *) echo "ERROR: 未知的 canonical 模式 $mode" >&2; exit 1 ;;
    esac
  )
}

# canonical diff 的 raw bytes（stdout）。用法：replay_args_canonical_diff <repo 內任一目錄> <treeA> <treeB> [path...]
replay_args_canonical_diff() { _replay_args_canonical diff "$@"; }
# canonical diff 的 SHA-256。用法同上。
replay_args_canonical_sha256() { _replay_args_canonical sha "$@"; }

# **唯一的合成函式**：在 base 的 worktree 依固定順序套用兩份 patch，算出三個 canonical SHA 與語意 SHA。
#
#   用法：replay_args_compose <worktree> <base_commit> <counterfactual 檔|""> <tooling 檔|"">
#   成功時 stdout 印一行：`<T1> <T2> <cf_sha|-> <tooling_sha> <composed_sha> <semantic_sha|->`
#
# ⚠️ 前提：worktree 由 `replay_args_prepare_worktree()` 建在 base、HEAD ＝ base、index ＝ base 的 tree。
# ⚠️ 固定順序 counterfactual → tooling；中繼 tree 用 `git write-tree`（⛔ 不建中繼 commit——會牴觸
#   `replay_args_prepare_worktree()` 的 HEAD 斷言）。空字串＝沒有該份；tooling 檔為 0 bytes 時不套用。
# ⚠️ 有 counterfactual 時：改動檔案集合必須**恰好**是 `I074_CF_FILES`，語意 diff（只含
#   `I074_CF_PRODUCT_PATHS`）必須非空——⛔ 任一不符即中止、⛔ 不輸出語意 SHA。
# ⚠️ 套用端（`git apply`）仍讀來源 repo 的 config；被扭曲時 canonical SHA 會 ≠ patch 的 raw bytes SHA，
#   由呼叫端的「raw ＝ canonical」fail-closed 擋下。
replay_args_compose() {
  local wt="$1" base="$2" cf="$3" tool="$4"
  local base_oid base_tree head idx t1 t2 status_out cf_sha="-" tool_sha comp_sha sem="-" names f
  base_oid="$(git -C "$wt" rev-parse --verify --quiet "${base}^{commit}")" || {
    echo "ERROR: 合成：解析不了 base $base。" >&2; return 1; }
  base_tree="$(git -C "$wt" rev-parse --verify --quiet "${base}^{tree}")" || {
    echo "ERROR: 合成：解析不了 base 的 tree。" >&2; return 1; }
  head="$(git -C "$wt" rev-parse HEAD)" || return 1
  if [ "$head" != "$base_oid" ]; then
    echo "ERROR: 合成：worktree 的 HEAD（$head）≠ base（$base_oid）。" >&2; return 1
  fi
  idx="$(git -C "$wt" write-tree)" || return 1
  if [ "$idx" != "$base_tree" ]; then
    echo "ERROR: 合成：worktree 的 index 不是乾淨的 base（$idx ≠ $base_tree）。" >&2; return 1
  fi
  for f in "$cf" "$tool"; do
    [ -n "$f" ] || continue
    if [ -L "$f" ] || [ ! -f "$f" ]; then
      echo "ERROR: 合成：patch 必須是一般檔案（⛔ symlink）：$f" >&2; return 1
    fi
    case "$f" in /*) ;; *) echo "ERROR: 合成：patch 必須是絕對路徑：$f" >&2; return 1 ;; esac
  done
  if [ -n "$cf" ]; then
    [ -s "$cf" ] || { echo "ERROR: 合成：counterfactual patch ⛔ 不得為 0 bytes（空的反事實＝沒有反事實）。" >&2; return 1; }
    git -C "$wt" apply --index "$cf" || { echo "ERROR: 合成：counterfactual patch 套不上 base。" >&2; return 1; }
    t1="$(git -C "$wt" write-tree)" || return 1
  else
    t1="$base_tree"
  fi
  if [ -n "$tool" ] && [ -s "$tool" ]; then
    git -C "$wt" apply --index "$tool" || {
      echo "ERROR: 合成：tooling patch 套不上 counterfactual 之後的 tree。" >&2; return 1; }
  fi
  t2="$(git -C "$wt" write-tree)" || return 1
  # ⚠️ **乾淨檢查**：`write-tree` 只看 index，容器掛的卻是工作樹——兩者必須恰好相同：
  #   ⛔ 不得有 untracked（`??`）、⛔ 不得有 unstaged 的改動（第二欄非空白）。
  # ⚠️ **⛔ 不可寫成 `git status | grep -q`**：`grep -q` 一找到就退出，`git` 收到 SIGPIPE 後在
  #   `set -o pipefail` 下讓整條 pipeline 回非零——於是 `if` 走不進去，**這道 fail-closed 守門反而被繞過**。
  #   先把 status 取成變數（順便讓 git 自己的失敗能被看見），再用 here-string 比對。
  # ⚠️ `--untracked-files=all` 明示，⛔ 不讓 `status.showUntrackedFiles=no` 之類的 config 藏掉 untracked。
  if ! status_out="$(git -C "$wt" status --porcelain=v1 --untracked-files=all)"; then
    echo "ERROR: 讀不到 worktree 的 git status——⛔ 無法確認有沒有漏檔，中止。" >&2
    return 1
  fi
  if grep -q '^??' <<< "$status_out"; then
    echo "ERROR: worktree 仍有 untracked 檔案——代表還有東西漏在 patch hash 外，⛔ 中止。" >&2
    grep '^??' <<< "$status_out" >&2
    return 1
  fi
  if grep -q '^.[^ ]' <<< "$status_out"; then
    echo "ERROR: worktree 有未進 index 的改動——容器會跑到 hash 外的內容，⛔ 中止。" >&2
    grep '^.[^ ]' <<< "$status_out" >&2
    return 1
  fi
  head="$(git -C "$wt" rev-parse HEAD)" || return 1
  [ "$head" = "$base_oid" ] || { echo "ERROR: 合成：HEAD 被動到了（$head）。" >&2; return 1; }

  if [ -n "$cf" ]; then
    names="$(_replay_args_canonical names "$wt" "$base_tree" "$t1")" || return 1
    if [ "$names" != "$(printf '%s\n' "${I074_CF_FILES[@]}" | LC_ALL=C sort)" ]; then
      echo "ERROR: 合成：counterfactual 改動的檔案集合 ⛔ 不是固定的四個檔（兩個產品檔 ＋ 兩個測試檔）：" >&2
      sed 's/^/  /' <<< "$names" >&2
      return 1
    fi
    cf_sha="$(replay_args_canonical_sha256 "$wt" "$base_tree" "$t1")" || return 1
    sem="$(replay_args_canonical_sha256 "$wt" "$base_tree" "$t1" "${I074_CF_PRODUCT_PATHS[@]}")" || return 1
    if [ "$sem" = "$EMPTY_SHA256" ]; then
      echo "ERROR: 合成：counterfactual 在兩個產品檔上沒有任何改動（語意 diff 為空）。" >&2; return 1
    fi
  fi
  tool_sha="$(replay_args_canonical_sha256 "$wt" "$t1" "$t2")" || return 1
  comp_sha="$(replay_args_canonical_sha256 "$wt" "$base_tree" "$t2")" || return 1
  printf '%s %s %s %s %s %s\n' "$t1" "$t2" "$cf_sha" "$tool_sha" "$comp_sha" "$sem"
}

# 一般路徑（Stage 1／2、finalizer／comparator 自己的 provenance）的 `tooling_patch_sha256`：
# 套用（可省略的）單一 patch 之後，base 到最終 tree 的 **canonical** diff SHA。
# ⛔ 不是「傳進來那個 patch 檔的 hash」——傳錯 patch 檔就會算出不同的值，那正是要抓的。
# ⚠️ ⑦a 起改寫在 `replay_args_compose()` 上（⛔ 不再 `git diff --binary <commit>`、⛔ 不再 `add -A -N`）；
#   空 patch 的值不變（空字串的 SHA）。
#   用法：replay_args_tooling_patch_sha256 <worktree> <base_commit> [patch_file]
replay_args_tooling_patch_sha256() {
  local worktree="$1" base_commit="$2" patch_file="${3:-}" out comp
  if [ -n "$patch_file" ]; then
    case "$patch_file" in /*) ;; *) patch_file="$PWD/$patch_file" ;; esac
  fi
  out="$(replay_args_compose "$worktree" "$base_commit" "" "$patch_file")" || return 1
  read -r _ _ _ _ comp _ <<< "$out"
  printf '%s\n' "$comp"
}

# ⚠️ **把 host 路徑轉成絕對路徑，⛔ 目錄不存在時硬失敗**。
# ⛔ 不可再用裸的 `$(cd "$(dirname "$p")" && pwd)/$(basename "$p")`：`cd` 失敗時
# command substitution 只是回空字串，於是 `/run_identity.json` 這種**看起來完全合法**
# 的絕對路徑會一路傳進 `-v` 掛載與容器 argv，最後的錯誤訊息完全指不到真正的原因。
# ⚠️ **本函式⛔ 只保證「父目錄存在」，⛔ 不保證「檔案存在」**——那是刻意的：
# evidence root 這類**輸出**路徑本來就允許尚未存在。**必要的輸入檔要由 caller 自己 `-f` 驗**
# （見 `replay_args_require_file`）。⛔ 漏驗的後果不是「早點報錯」而已：實測 `docker -v`
# 會把不存在的來源**建成 root 擁有的目錄**留在 host 上，錯誤延後到容器內才爆。
replay_args_abs_path() {
  local p="$1" label="${2:-路徑}" dir
  dir="$(cd "$(dirname "$p")" 2>/dev/null && pwd)" || dir=""
  if [ -z "$dir" ]; then
    echo "ERROR: $label 的所在目錄不存在：$(dirname "$p")" >&2
    return 1
  fi
  printf '%s/%s\n' "$dir" "$(basename "$p")"
}

# ⚠️ **必要輸入檔的 fail-closed 檢查**：在組 `docker -v` **之前**呼叫。
# ⛔ 不可省略——`docker -v` 對不存在的來源會自己建目錄（實測是 root 擁有，host 上還刪不掉），
# 於是「檔名打錯」會變成容器內「不是檔案」這種指不到原因的錯誤。
replay_args_require_file() {
  local p="$1" label="${2:-輸入檔}"
  if [ ! -f "$p" ]; then
    echo "ERROR: $label 必須是既有檔案：$p" >&2
    return 1
  fi
}
