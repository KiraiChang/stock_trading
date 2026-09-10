#!/usr/bin/env bash
# I-100：Stage 0 與離線 replay 的**共用 argv builder ＋ 參數所有權驗證**。
#
# 為什麼要抽出來：計畫書要求「CLI 衝突測試必須用**官方腳本實際組出的完整 argv** 跑一次」。
# 兩支腳本各自拼一份的話，測試驗到的就不是實際組法。所以**唯一的組裝入口在這裡**，
# `run-evaluation.sh`、`run-replay-offline.sh` 與 `scripts/test-replay-args.sh` 都呼叫它。
#
# ⛔ 這五個參數只能由官方腳本注入，使用者傳入即拒絕：
#   --image-digest / --base-commit / --tooling-patch-sha256 / --source-root / --runner-sha256
#   （--runner-sha256 於 v23 第一輪 review 補上：runner 是容器外的 shell 腳本，
#     容器內的 Python 讀不到它，hash 只能由這一側算好後注入。）
# 理由：它們是 provenance 的事實來源。使用者能傳同名參數，就能寫出與實際執行不符的
# provenance。⚠️ **驗證只能發生在 shell 層**——容器內的 Python 無法自己
# `docker image inspect` 得知自己跑在哪個 image；CLI 那一端只負責對**重複值**中止。

REPLAY_INJECTED_ARGS=(--image-digest --base-commit --tooling-patch-sha256 --source-root --runner-sha256)

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
#   用法：replay_args_offline <image_digest> <source_root> <base_commit> <tooling_patch_sha256> [使用者參數...]
replay_args_offline() {
  local image_digest="$1"; shift
  local source_root="$1"; shift
  local base_commit="$1"; shift
  local tooling_patch="$1"; shift
  local runner_sha256="$1"; shift
  printf '%s\n' python -m backtest.modular.sr_scoring.evaluation \
    --image-digest "$image_digest" --source-root "$source_root" \
    --base-commit "$base_commit" --tooling-patch-sha256 "$tooling_patch" \
    --runner-sha256 "$runner_sha256" "$@"
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

# tooling patch 的 hash 一律取自 **worktree 實際 `git diff --binary` 的輸出**，
# ⛔ 不是「傳進來那個 patch 檔的 hash」——傳錯 patch 檔就會算出不同的值，那正是要抓的。
#
# ⚠️ `git diff --binary` **預設不含 untracked**，而本計畫明確會新增檔案。所以用
# `git apply --index` 套用（新增檔直接進 index）、套用後補 `git add -A -N`，
# 取完 diff 再斷言 `git status --porcelain` 沒有 `??` 行——還有 untracked 就代表
# 仍有東西漏在 hash 外。
#   用法：replay_args_tooling_patch_sha256 <worktree> <base_commit> [patch_file]
replay_args_tooling_patch_sha256() {
  local worktree="$1" base_commit="$2" patch_file="${3:-}"
  if [ -n "$patch_file" ]; then
    git -C "$worktree" apply --index "$patch_file" || return 1
  fi
  git -C "$worktree" add -A -N >/dev/null 2>&1 || true
  if git -C "$worktree" status --porcelain | grep -q '^??'; then
    echo "ERROR: worktree 仍有 untracked 檔案——代表還有東西漏在 tooling patch hash 外，⛔ 中止。" >&2
    git -C "$worktree" status --porcelain | grep '^??' >&2
    return 1
  fi
  git -C "$worktree" diff --binary "$base_commit" | sha256sum | cut -d' ' -f1
}
