#!/usr/bin/env bash
# 記憶體護欄：讓 container 的 --memory 上限不可能高於 host 實際供得起的量。
#
# 由 backend/frontend/python 三支 scripts/test.sh 以 source 引入，不直接執行。
#
# 為什麼需要這個：
#   docker --memory 是 cgroup 的「上限」，不是「預留」。把上限設得比 host 的 MemAvailable
#   還高時，container 根本撞不到自己的 cgroup 限制，會先耗盡 host 實體記憶體 + swap，
#   由 **host 層級的 OOM killer** 出手，砍掉 badness 分數最高的行程——在這台機器上就是
#   呼叫測試的那個工作階段本身（claude CLI，RSS 隨 transcript 成長到 400~500MB）。
#   2026-08-03 與 2026-08-05 各發生一次，現況說明見 docs/development-workflow.md 的
#   「`MEM` 是上限，不是預留——不可高於 host 可用量」。
#
#   所以「測試不夠記憶體就把 MEM 調大」是錯的：那是把「container 自己被 cgroup 砍」
#   （可回收、錯誤訊息讀得到）換成「host 砍掉呼叫端」（工作階段直接消失）。
#   要降的是實際用量（拆步驟、限制 runtime heap），不是上限。
#
# 用法：
#   source "$REPO_ROOT/scripts/lib/mem-guard.sh"
#   MEM="$(mem_guard_clamp "$MEM")"
#
# 可覆寫的環境變數：
#   MEM_RESERVE_MB  保留給呼叫端執行期間「還會再長大」的量，預設 150。注意 MemAvailable
#                   已經扣掉所有行程當下的 RSS，所以這裡不必再涵蓋 claude 的既有佔用，
#                   只需涵蓋跑測試那幾分鐘內的成長與安全邊際，設太大會讓上限低到無法執行。
#   MEM_MIN_MB      clamp 後的下限，低於此值直接 abort，預設 256
#   MEM_STRICT=1    超過上限時不 clamp，直接 abort（CI 或想明確發現設定錯誤時用）
#   MEM_FORCE=1     完全略過護欄（明確知道自己在做什麼時才用）

# 把 docker 記憶體字串（512m / 2g / 1024 / 1g）換算成 MB。無法解析時回傳空字串。
mem_guard_to_mb() {
  local value="${1:-}"
  local num unit
  num="${value%%[a-zA-Z]*}"
  unit="${value#"$num"}"
  [ -n "$num" ] || return 0
  case "$num" in
    *[!0-9]*) return 0 ;;
  esac
  case "$(printf '%s' "$unit" | tr '[:upper:]' '[:lower:]')" in
    ''|b)   echo $((num / 1024 / 1024)) ;;
    k|kb)   echo $((num / 1024)) ;;
    m|mb)   echo "$num" ;;
    g|gb)   echo $((num * 1024)) ;;
    *)      return 0 ;;
  esac
}

# host 目前真正可用的記憶體（MB）。讀 MemAvailable 而非 MemTotal：
# 這台機器常駐 dev stack 與其他 container，MemTotal 沒有參考價值。
mem_guard_available_mb() {
  local kb
  kb="$(awk '/^MemAvailable:/ {print $2; exit}' /proc/meminfo 2>/dev/null || true)"
  [ -n "$kb" ] || return 1
  echo $((kb / 1024))
}

# 主要入口：把請求的記憶體上限壓進 host 供得起的範圍，回傳實際該用的值（docker 格式）。
# 所有訊息都印到 stderr，stdout 只留回傳值，方便 MEM="$(mem_guard_clamp "$MEM")"。
mem_guard_clamp() {
  local requested="${1:-}"
  local reserve="${MEM_RESERVE_MB:-150}"
  local floor="${MEM_MIN_MB:-256}"
  local requested_mb available_mb max_mb

  if [ "${MEM_FORCE:-0}" = "1" ]; then
    echo >&2 "==> [mem-guard] MEM_FORCE=1，略過記憶體護欄（MEM=$requested）"
    printf '%s\n' "$requested"
    return 0
  fi

  requested_mb="$(mem_guard_to_mb "$requested")"
  if [ -z "$requested_mb" ]; then
    echo >&2 "==> [mem-guard] 無法解析 MEM=$requested，略過護欄"
    printf '%s\n' "$requested"
    return 0
  fi

  if ! available_mb="$(mem_guard_available_mb)"; then
    echo >&2 "==> [mem-guard] 讀不到 /proc/meminfo 的 MemAvailable，略過護欄"
    printf '%s\n' "$requested"
    return 0
  fi

  max_mb=$((available_mb - reserve))

  if [ "$max_mb" -lt "$floor" ]; then
    cat >&2 <<EOF
==> [mem-guard] 記憶體不足，中止。
    host available=${available_mb}MB、保留 ${reserve}MB 後只剩 ${max_mb}MB，低於下限 ${floor}MB。
    先關掉不需要的 container（dev stack 常駐約 145MiB）再重跑，或調低 MEM_RESERVE_MB／MEM_MIN_MB。
EOF
    return 1
  fi

  if [ "$requested_mb" -le "$max_mb" ]; then
    echo >&2 "==> [mem-guard] MEM=$requested（host available=${available_mb}MB、上限 ${max_mb}MB）"
    printf '%s\n' "$requested"
    return 0
  fi

  if [ "${MEM_STRICT:-0}" = "1" ]; then
    cat >&2 <<EOF
==> [mem-guard] MEM_STRICT=1，中止。
    MEM=${requested}（${requested_mb}MB）高於 host 供得起的 ${max_mb}MB
    （available=${available_mb}MB - reserve=${reserve}MB）。
    --memory 是上限不是預留，設得比 host 高只會讓 host OOM killer 改砍呼叫端。
EOF
    return 1
  fi

  cat >&2 <<EOF
==> [mem-guard] MEM 由 ${requested} 下修為 ${max_mb}m。
    host available=${available_mb}MB、保留 ${reserve}MB 給呼叫端。--memory 是 cgroup 上限而非
    預留，設得比 host 供得起的量高時，host OOM killer 會先砍掉呼叫測試的行程而不是 container。
    若這步真的塞不進 ${max_mb}MB，要降的是實際用量（拆步驟、限制 runtime heap），不是上限。
EOF
  printf '%sm\n' "$max_mb"
}
