#!/bin/bash -p
# I074-STAGE2-LABEL-SHIM
# I-074 Stage 2 ⑦b：⑩ 的容器 label 的**唯一注入點**（issue.md I-074 ⑦ 總綱 v1「四」的 label 列＋「Stage 2 步驟 ⑦b 細部計畫 v1」「二之五」）。
#
# 複本內的 orchestrator 把**複本的**這個檔案複製成 `<work>/bin/docker`；supervisor 把 workload 的 PATH 設成
# `<work>/bin:/usr/bin:/bin`，所以 ⑩ 呼叫圖裡以名字呼叫的 `docker` 都經過這裡。⛔ 不是給人直接用的入口。
#
#   - `run`／`create`／`container run`／`container create`：在子指令之後插入**恰好一個**
#     `--label i074.stage2.run=<token>`，再 exec 真正的 docker；其餘子指令原樣 exec（I/O 透明：本檔⛔ 不寫 stdout）。
#   - 拒絕（結束碼 125、⛔ 不執行 docker）：子指令之前有全域選項（任何子指令都拒絕——否則 `docker -H x run` 不會被
#     認成 run）；run／create 的任一 token 含 label 鍵或以 `--label-file` 開頭；token 不是 64 位小寫 hex 或 ≠ sentinel；
#     真正的 docker 不是固定的 `/usr/bin/docker`、指向 shim、或不符合信任條件；環境帶 `SIZING_*`（與 sizing shim 互斥）。
#   - ⚠️ 已知限制：直接呼叫 `/usr/bin/docker` 攔不到（由靜態測試禁止 ⑩ 呼叫圖這樣做）。
set -uo pipefail

# ── 常數（⚠️ 測試以 sed 改的只有這一段：scripts/test-i074-stage2.sh） ──────────────────
I074_LABEL_KEY=i074.stage2.run
I074_SENTINEL_PATH=/run/lock/i074-stage2.active
I074_DOCKER=/usr/bin/docker
I074_TRUSTED_OWNER_UID=0
# ── 常數結束 ────────────────────────────────────────────────────────────────────────

die() { printf 'i074-stage2 label shim：%s——⛔ 不執行 docker\n' "$*" >&2; exit 125; }

case "$-" in *p*) ;; *) die "必須以 #!/bin/bash -p 執行" ;; esac
for _v in $(compgen -e); do
  case "$_v" in SIZING_*) die "環境帶 $_v（sizing shim 與 label shim 互斥）" ;; esac
done

# 目錄可信：owner 是信任 uid 或 root，且⛔ group／other 可寫（root 擁有且設了 sticky bit 的共用目錄除外）。
trusted_dir() {
  local owner mode
  read -r owner mode < <(stat -L -c '%u %a' -- "$1") || return 1
  [ "$owner" = "$I074_TRUSTED_OWNER_UID" ] || [ "$owner" = 0 ] || return 1
  (( (8#$mode & 8#022) == 0 )) && return 0
  [ "$owner" = 0 ] && (( (8#$mode & 8#1000) != 0 ))
}
trusted_program() {  # $1＝realpath
  local owner mode dir
  [ -f "$1" ] && [ -x "$1" ] || return 1
  read -r owner mode < <(stat -L -c '%u %a' -- "$1") || return 1
  { [ "$owner" = "$I074_TRUSTED_OWNER_UID" ] || [ "$owner" = 0 ]; } && (( (8#$mode & 8#022) == 0 )) || return 1
  dir="${1%/*}"; [ -n "$dir" ] || dir=/
  while :; do
    trusted_dir "$dir" || return 1
    [ "$dir" = / ] && return 0
    dir="${dir%/*}"; [ -n "$dir" ] || dir=/
  done
}

REAL="${I074_STAGE2_REAL_DOCKER:-}"
case "$REAL" in /*) ;; *) die "I074_STAGE2_REAL_DOCKER 必須是絕對路徑：'$REAL'" ;; esac
REAL_CANON="$(realpath -e -- "$REAL" 2>/dev/null)" || die "I074_STAGE2_REAL_DOCKER 不存在：$REAL"
SELF_CANON="$(realpath -e -- "${BASH_SOURCE[0]}" 2>/dev/null)" || die "找不到 shim 自己"
[ "$REAL_CANON" != "$SELF_CANON" ] || die "真正的 docker 指向 shim 自己"
[ "$REAL_CANON" = "$(realpath -e -- "$I074_DOCKER" 2>/dev/null)" ] || die "真正的 docker 不是固定的 $I074_DOCKER：$REAL_CANON"
# ⚠️ here-string（⛔ `head | grep -q`：pipefail 下 grep 提早結束會讓 head 收到 SIGPIPE 而誤判成「不是 shim」）。
if [ "$(head -c 2 -- "$REAL_CANON" 2>/dev/null)" = '#!' ] \
   && grep -qE 'I074-SIZING-DOCKER-SHIM|I074-STAGE2-LABEL-SHIM' <<< "$(head -n 5 -- "$REAL_CANON")"; then
  die "真正的 docker 是 shim：$REAL_CANON"
fi
trusted_program "$REAL_CANON" || die "真正的 docker 不符合信任條件：$REAL_CANON"

TOKEN="${I074_STAGE2_TOKEN:-}"
[[ "$TOKEN" =~ ^[0-9a-f]{64}$ ]] || die "I074_STAGE2_TOKEN 不是 64 位小寫 hex"
SENTINEL=""
{ IFS= read -r -d '' SENTINEL < "$I074_SENTINEL_PATH"; } 2>/dev/null || [ -n "$SENTINEL" ] || die "讀不到 sentinel"
[[ "$SENTINEL" =~ \"token\":\"([0-9a-f]{64})\" ]] && [ "${BASH_REMATCH[1]}" = "$TOKEN" ] \
  || die "I074_STAGE2_TOKEN 與 sentinel 內的不符"

[ "$#" -ge 1 ] || exec "$REAL_CANON"
case "$1" in -*) die "子指令之前有全域選項 '$1'" ;; esac

SUB=0
case "$1" in
  run|create) SUB=1 ;;
  container) [ "$#" -ge 2 ] && case "$2" in run|create) SUB=2 ;; esac ;;
esac
if [ "$SUB" -gt 0 ]; then
  for _tok in "$@"; do
    case "$_tok" in
      *"$I074_LABEL_KEY"*) die "參數裡已經有 label 鍵 $I074_LABEL_KEY（重複或偽造）" ;;
      --label-file*) die "⛔ 不接受 --label-file" ;;
    esac
  done
  exec "$REAL_CANON" "${@:1:SUB}" --label "$I074_LABEL_KEY=$TOKEN" "${@:SUB+1}"
fi
exec "$REAL_CANON" "$@"
