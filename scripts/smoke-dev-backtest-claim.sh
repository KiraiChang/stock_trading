#!/usr/bin/env bash
# 回測任務的認領與持久化的 dev 驗收（I-134 計畫書第七節第 4 點）：兩條路徑的競爭、python-server／python-worker
# 各自重啟、HTTP 回應遺失、schema 時序。
#
# 範圍：直接用 psql 寫入測試任務、直接呼叫 python-server 的 POST /backtest，⛔ 不經過 backend。這裡驗的是
#   python 的認領與持久化；backend 的條件式取消、HTTP 觸發與 log 由 Go 測試負責。⚠️ 完整的 backend 提交流程
#   （backend 收到請求 → INSERT → 觸發 HTTP → python 認領）這支腳本⛔ 沒有驗。
#
# Usage:
#   scripts/smoke-dev-backtest-claim.sh
#   SCENARIOS=race,lost-response NO_BUILD=1 scripts/smoke-dev-backtest-claim.sh
#
# 前置：dev stack 的 postgres、backend（已套用 migration 077）、python-server 在跑（例如先跑 scripts/smoke-dev.sh）；
#   dev DB ⛔ 沒有任何 pending／running 的回測；測試標的在日期範圍內有日 K，而且回測至少產生一筆交易。
#
# Environment overrides:
#   SCENARIOS        要跑的情境，逗號或空白分隔（default: race,server-restart,worker-restart,lost-response,schema）。
#                    另有 trades（I-135：backtrader 路徑的交易明細；要搭配 SMOKE_STRATEGY=breakout_v1，⛔ 不在預設裡）：
#                      SCENARIOS=trades SMOKE_STRATEGY=breakout_v1 scripts/smoke-dev-backtest-claim.sh
#   NO_BUILD         1 = 不 build，用現有的 image（default: 0）
#   SMOKE_STRATEGY   回測策略（default: breakout_swing_atr_v1；2330 的 2024～2025 日 K 會產生交易）
#   SMOKE_SYMBOL     回測標的（default: 2330）
#   SMOKE_START／SMOKE_END  回測日期（default: 2024-01-01／2025-12-31）
#   SERVER_URL       python-server（dev compose 發布在 127.0.0.1:18001）（default: http://127.0.0.1:18001）
#   TOTAL_TIMEOUT    整支腳本（收尾除外）的上限秒數（default: 1800）
#   BUILD_TIMEOUT    build 的上限秒數；實際取它與總剩餘時間的較小值（default: 1200）
#   STEP_TIMEOUT     每個等待迴圈的上限秒數；實際取它與總剩餘時間的較小值（default: 120）
#   LOCK_TIMEOUT     持鎖連線（pg_sleep）的上限秒數；實際取它與總剩餘時間的較小值（default: 300）
#   CALL_TIMEOUT     單一 docker／psql／curl 呼叫的上限秒數；也受總剩餘時間限制（default: 30）
#   CLEANUP_TIMEOUT  收尾的上限秒數（逾時之後仍要能收尾，⛔ 不受總剩餘時間限制）（default: 180）
#   KILL_AFTER       逾時送出 SIGTERM 之後，再等幾秒就送 SIGKILL（default: 5）
#   CHECK_INTERVAL   等待迴圈的間隔秒數（default: 1）
#
# 結束碼：0 全部通過；1 驗收失敗或收尾失敗；2 參數或前置條件不對（⛔ 沒有改任何東西）。
#
# ⛔ 只操作 dev project：compose 檔固定是 docker-compose.dev.yml（⛔ 不接受覆寫），任何會改變狀態的指令之前先確認
#    解析出來的 project 是 stock_trading_dev；COMPOSE_PROJECT_NAME 有設定（連空字串）也停下。
# 測試資料：job_id 一律是 i134smoke_<執行時間>_<情境>_<序號>；驗證與清理的 SQL 用 starts_with（字面比對，
#    ⛔ 不用 LIKE：`_` 會被當成萬用字元），前綴用 psql 變數傳入。臨時 DB 是 i134smoke_<執行時間>。
# 收尾（trap；成功、失敗、逾時、Ctrl+C 都走同一段）：
#   1. 本次前綴下仍是 pending 的改成 failed（i134smoke_cleanup），再停掉 python 兩個服務（暫停中的先 unpause）。
#   2. 確認執行路徑已停止：兩個 container 都不在執行、⛔ 不是 paused；結束來自它們的殘留 DB 連線（被鎖擋住的
#      backend 不會自己發現 client 已經斷線），再確認 pg_stat_activity 沒有來自它們的連線。
#   3. 結束 socket client 與持鎖的連線（⛔ 不論第 2 步的結果）。
#   4. 第 2 步確認通過才做：依前綴刪除本次的交易明細、結果、任務。
#   5. 臨時 DB：先停掉臨時 server 並確認、結束連到臨時 DB 的連線，確認之後才刪除。
#   6. 第 2 步確認通過才做，最後才做：依開始時的紀錄恢復服務（worker 原本在跑就起回來；原本存在但停止就維持停止、
#      ⛔ 不移除；原本不存在才移除本次建立的）。
#   第 2 步沒有通過：⛔ 不刪測試資料、⛔ 不恢復服務，印出前綴、各服務狀態與手動清理的 SQL，回傳非 0。
# ⛔ 輸出不印 DSN。
# A && B || fail 的 C 都是結束腳本的 fail／die2，不會有「A 真、B 假之外也跑 C」的問題
# shellcheck disable=SC2015
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="docker-compose.dev.yml"
DEV_PROJECT="stock_trading_dev"
DEFAULT_SCENARIOS="race server-restart worker-restart lost-response schema"
ALL_SCENARIOS="$DEFAULT_SCENARIOS trades"
SCENARIOS="${SCENARIOS:-$DEFAULT_SCENARIOS}"
NO_BUILD="${NO_BUILD:-0}"
SMOKE_STRATEGY="${SMOKE_STRATEGY:-breakout_swing_atr_v1}"
SMOKE_SYMBOL="${SMOKE_SYMBOL:-2330}"
SMOKE_START="${SMOKE_START:-2024-01-01}"
SMOKE_END="${SMOKE_END:-2025-12-31}"
SERVER_URL="${SERVER_URL:-http://127.0.0.1:18001}"
TOTAL_TIMEOUT="${TOTAL_TIMEOUT:-1800}"
BUILD_TIMEOUT="${BUILD_TIMEOUT:-1200}"
STEP_TIMEOUT="${STEP_TIMEOUT:-120}"
LOCK_TIMEOUT="${LOCK_TIMEOUT:-300}"
CALL_TIMEOUT="${CALL_TIMEOUT:-30}"
CLEANUP_TIMEOUT="${CLEANUP_TIMEOUT:-180}"
KILL_AFTER="${KILL_AFTER:-5}"
CHECK_INTERVAL="${CHECK_INTERVAL:-1}"
SRV=python-server
WRK=python-worker

die2() { echo "ERROR: $*" >&2; exit 2; }

# ── 參數：任何 docker 指令之前 ──
if [ -n "${COMPOSE_PROJECT_NAME+x}" ]; then
  die2 "COMPOSE_PROJECT_NAME is set (\"$COMPOSE_PROJECT_NAME\"); it would override the project name in $COMPOSE_FILE. Unset it and rerun."
fi
for v in TOTAL_TIMEOUT BUILD_TIMEOUT STEP_TIMEOUT LOCK_TIMEOUT CALL_TIMEOUT CLEANUP_TIMEOUT KILL_AFTER CHECK_INTERVAL; do
  case "${!v}" in ''|*[!0-9]*) die2 "$v 要是正整數（目前：${!v}）" ;; esac
  [ "${!v}" -gt 0 ] || die2 "$v 要是正整數（目前：${!v}）"
done
case "$NO_BUILD" in 0|1) ;; *) die2 "NO_BUILD 只能是 0 或 1" ;; esac
[[ "$SMOKE_STRATEGY" =~ ^[a-z0-9_]+$ ]] || die2 "SMOKE_STRATEGY 只能有 [a-z0-9_]"
[[ "$SMOKE_SYMBOL" =~ ^[0-9A-Z.]+$ ]] || die2 "SMOKE_SYMBOL 只能有 [0-9A-Z.]"
for v in SMOKE_START SMOKE_END; do
  [[ "${!v}" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]] || die2 "$v 要是 YYYY-MM-DD"
done
[[ "$SERVER_URL" =~ ^http://127\.0\.0\.1:[0-9]+$ ]] || die2 "SERVER_URL 只接受 http://127.0.0.1:<port>（dev 發布的位址）"
SCEN_LIST=()
for s in ${SCENARIOS//,/ }; do
  case " $ALL_SCENARIOS " in *" $s "*) SCEN_LIST+=("$s") ;; *) die2 "不認得的情境：$s（可用：$ALL_SCENARIOS）" ;; esac
done
[ "${#SCEN_LIST[@]}" -gt 0 ] || die2 "SCENARIOS 是空的"
# trades 的精度界線是照 backtrader 路徑的欄位定義訂的（I-135），只用在 breakout_v1
if [[ " ${SCEN_LIST[*]} " == *" trades "* ]] && [ "$SMOKE_STRATEGY" != breakout_v1 ]; then
  die2 "trades 情境要搭配 SMOKE_STRATEGY=breakout_v1（目前：$SMOKE_STRATEGY）"
fi
# trades 的明細查詢跟 Go 的 PostgreSQL 測試共用同一份檔案（參數 $1＝job_id、$2＝前綴；⛔ 不在這裡另寫一份）
TRADES_CHECK_SQL="$REPO_ROOT/backend/internal/database/testdata/backtest_trades_check.sql"
# 費用核對（I-136）也一樣：參數 $1＝job_id、$2＝前綴、$3＝手續費率、$4＝稅率
TRADES_FEE_SQL="$REPO_ROOT/backend/internal/database/testdata/backtest_trades_fee_check.sql"
if [[ " ${SCEN_LIST[*]} " == *" trades "* ]]; then
  [ -r "$TRADES_CHECK_SQL" ] || die2 "讀不到明細查詢：$TRADES_CHECK_SQL"
  [ -r "$TRADES_FEE_SQL" ] || die2 "讀不到費用查詢：$TRADES_FEE_SQL"
fi
for c in timeout curl python3; do
  command -v "$c" >/dev/null 2>&1 || die2 "找不到 $c"
done

RUN_ID="$(date -u +%Y%m%d%H%M%S)"
PREFIX="i134smoke_${RUN_ID}_"
TMPDB="i134smoke_${RUN_ID}"
LOCK_APP="${PREFIX}lock"
TMP_SRV_NAME="${PREFIX}schema"
[[ "$PREFIX" =~ ^[a-z0-9_]+$ ]] || die2 "前綴只能有 [a-z0-9_]（$PREFIX）"
WORK="$(mktemp -d "${TMPDIR:-/tmp}/smoke-dev-backtest-claim.XXXXXX")"

cd "$REPO_ROOT"
DEADLINE=$((SECONDS + TOTAL_TIMEOUT))
IN_CLEANUP=0
CDL=0

# rem CAP：本次呼叫可用的秒數＝CAP 與「剩餘時間扣掉 KILL_AFTER」的較小值（收尾期間看收尾的截止時間 CDL，
#   正常流程看 DEADLINE）。扣掉 KILL_AFTER：逾時之後還有 KILL_AFTER 秒才送 SIGKILL，這段也要算在期限內。
#   時間用完就輸出 0——⛔ 不再給新的預算（review 2026-11-18：原本耗盡後固定給 1 秒，每個呼叫又多 KILL_AFTER，
#   CLEANUP_TIMEOUT=1 實際花了 10 秒）。0 由 t／ti／wait_until 擋下、⛔ 不執行（timeout 0 等於不設上限）。
rem() {
  local cap="$1" r
  if [ "$IN_CLEANUP" = 1 ]; then r=$((CDL - SECONDS - KILL_AFTER)); else r=$((DEADLINE - SECONDS - KILL_AFTER)); fi
  (( r < 1 )) && r=0
  (( r > cap )) && r=$cap
  echo "$r"
}
no_budget() {
  if [ "$IN_CLEANUP" = 1 ]; then
    echo "ERROR（收尾）：收尾時限 ${CLEANUP_TIMEOUT} 秒用完，未執行：$*" >&2
    : >"$WORK/cleanup_failed" 2>/dev/null || true
  else
    echo "ERROR: 總時限 ${TOTAL_TIMEOUT} 秒用完，未執行：$*" >&2
  fi
  return 124
}
# t SEC CMD…：有上限的執行（stdin 是 /dev/null）。⚠️ 一定要 -k：忽略 SIGTERM 的 docker／plugin 才停得了。
#   SEC 不是正整數（時間用完）就⛔ 不執行、回傳 124。
t() { local sec="$1"; shift; [[ "$sec" =~ ^[1-9][0-9]*$ ]] || { no_budget "$*"; return 124; }; timeout -k "$KILL_AFTER" "$sec" "$@" </dev/null; }
# ti SEC CMD…：同上，但保留 stdin（psql 的 SQL 從 stdin 進來）
ti() { local sec="$1"; shift; [[ "$sec" =~ ^[1-9][0-9]*$ ]] || { no_budget "$*"; return 124; }; timeout -k "$KILL_AFTER" "$sec" "$@"; }
compose() { local sec="$1"; shift; t "$sec" docker compose -f "$COMPOSE_FILE" --profile worker "$@"; }
dc() { compose "$(rem "$CALL_TIMEOUT")" "$@"; }

# q NAME [var=val…]：SQL 從 stdin 進來，開頭加上 `-- q:NAME`（假 docker 測試靠它分辨查詢）。
#   前綴一律用 psql 變數 :'prefix' 傳入；其他變數由參數帶入。SQL_DB 指定資料庫（default: trading）。
q() {
  local name="$1"; shift
  local args=(-v ON_ERROR_STOP=1 -v "prefix=$PREFIX" -v "lockapp=$LOCK_APP")
  local kv
  for kv in "$@"; do args+=(-v "$kv"); done
  { printf -- '-- q:%s\n' "$name"; cat; } | ti "$(rem "$CALL_TIMEOUT")" docker compose -f "$COMPOSE_FILE" --profile worker \
    exec -T postgres psql -U trading -d "${SQL_DB:-trading}" -X -q -tA "${args[@]}" -f -
}

log() { echo "==> $*"; }
fail() { echo "ERROR: $*" >&2; exit 1; }

# read_rates → "c t"：python-server container 裡 config 載入的**原始**費率（I-136；`python/config.yaml` 的
#   backtest.commission_rate／tax_rate）。⛔ 不讀 TWCommission 的參數（那是被測的對象，換算錯了會兩邊一起錯）。
#   只接受有限的非負小數（nan、inf、負數都不是）；讀不到或不合格就回傳非 0。
valid_rate() { [[ "$1" =~ ^[0-9]+(\.[0-9]+)?([eE][-+]?[0-9]+)?$ ]]; }
read_rates() {
  local out c t
  out="$(dc exec -T "$SRV" python -c 'from config import COMMISSION_RATE as c, TAX_RATE as t; print(repr(float(c)), repr(float(t)))')" || return 1
  out="${out##*$'\n'}"
  read -r c t _ <<<"$out"
  valid_rate "$c" && valid_rate "$t" || return 1
  echo "$c $t"
}

# ── 服務的查詢 ──
# cid_of SVC：用 label 找（⛔ 含 compose run 的臨時 container，oneoff=False 排除它）；超過一個就當成錯誤
cid_of() {
  local out
  out="$(t "$(rem "$CALL_TIMEOUT")" docker ps -a -q --no-trunc \
    --filter "label=com.docker.compose.project=$DEV_PROJECT" \
    --filter "label=com.docker.compose.service=$1" \
    --filter "label=com.docker.compose.oneoff=False")" || return 1
  [ "$(grep -c . <<<"$out")" -le 1 ] || { echo "ERROR: $1 有不只一個 container" >&2; return 1; }
  echo "$out"
}
state_of() { t "$(rem "$CALL_TIMEOUT")" docker inspect -f '{{.State.Status}}' "$1"; }
ip_of() { t "$(rem "$CALL_TIMEOUT")" docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}} {{end}}' "$1" | awk '{print $1}'; }
KNOWN_IPS=""
# remember_ip VAR CID：把 CID 的 IP 存進 VAR，並記進 KNOWN_IPS（收尾確認「沒有來自它們的連線」用）。
#   ⚠️ 不能寫成 VAR="$(remember_ip …)"：command substitution 是 subshell，KNOWN_IPS 的更新會丟掉。
remember_ip() {
  local ip; ip="$(ip_of "$2")" || ip=""
  printf -v "$1" '%s' "$ip"
  if [ -n "$ip" ]; then KNOWN_IPS="${KNOWN_IPS:+$KNOWN_IPS,}$ip"; fi
}
now_ts() { date -u +%Y-%m-%dT%H:%M:%SZ; }
logs_since() { local ts="$1"; shift; dc logs --no-color --since "$ts" "$@" 2>&1; }

# wait_until DESC CMD…：CMD 成功就回傳；STEP_TIMEOUT（與總剩餘時間）內等不到就失敗
wait_until() {
  local desc="$1" budget; shift
  budget="$(rem "$STEP_TIMEOUT")"
  (( budget >= 1 )) || { no_budget "等待：$desc" || true; fail "等不到（時限用完）：$desc"; }
  local dl=$((SECONDS + budget))
  while :; do
    if "$@"; then return 0; fi
    (( SECONDS + CHECK_INTERVAL <= dl )) || fail "等不到：$desc"
    sleep "$CHECK_INTERVAL"
  done
}

# ── 狀態紀錄（收尾用）──
SRV_CID0=""; WRK_CID0=""; WRK_STATE0=""   # 開始時的紀錄；WRK_STATE0：absent／stopped／running
WRK_CREATED=0                               # worker 是本次建立的
STATE_CHANGED=0                             # 已經開始改變狀態（之後失敗都要收尾）
LOCK_BG=""; CLIENT_BG=""; TMP_SRV_ID=""; TMP_SRV_STARTED=0; TMPDB_CREATED=0

# ── 持鎖（重啟兩個情境與 lost-response 共用）──
LOCK_PID=""
lock_ready() {
  LOCK_PID="$(q lock_pid <<'SQL'
SELECT a.pid FROM pg_stat_activity a
WHERE a.application_name = :'lockapp'
  AND EXISTS (SELECT 1 FROM pg_locks l WHERE l.pid = a.pid AND l.granted
              AND l.relation = 'backtest_trades'::regclass AND l.mode = 'ShareLock');
SQL
)" || return 1
  LOCK_PID="${LOCK_PID//[[:space:]]/}"
  [[ "$LOCK_PID" =~ ^[0-9]+$ ]]
}
start_lock() {
  local sec; sec="$(rem "$LOCK_TIMEOUT")"
  (( sec >= 1 )) || fail "總時限用完，⛔ 不開始持鎖"
  log "持鎖：LOCK TABLE backtest_trades IN SHARE MODE（最多 ${sec} 秒）"
  printf -- "-- q:lock_hold\nBEGIN;\nLOCK TABLE backtest_trades IN SHARE MODE;\nSELECT pg_sleep(%s);\nCOMMIT;\n" "$sec" \
    | timeout -k "$KILL_AFTER" "$sec" docker compose -f "$COMPOSE_FILE" --profile worker \
        exec -T -e "PGAPPNAME=$LOCK_APP" postgres psql -U trading -d trading -X -q -tA -f - >"$WORK/lock.out" 2>&1 &
  LOCK_BG=$!
  wait_until "持鎖連線取得鎖" lock_ready
}
release_lock() {
  q lock_release >/dev/null <<'SQL' || true
SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE application_name = :'lockapp';
SQL
  if [ -n "$LOCK_BG" ]; then kill "$LOCK_BG" 2>/dev/null || true; wait "$LOCK_BG" 2>/dev/null || true; LOCK_BG=""; fi
  local left
  left="$(q lock_left <<'SQL'
SELECT count(*) FROM pg_stat_activity WHERE application_name = :'lockapp';
SQL
)" || return 1
  [ "${left//[[:space:]]/}" = 0 ]
}
# blocked_by_lock IP：來自 IP 的連線被本輪持鎖的 PID 擋住，而且已在同一個交易裡持有 backtest_results 的鎖
#   （⛔ 只看 wait_event_type='Lock' 不算）
blocked_by_lock() {
  local n
  n="$(q blocked "ip=$1" "lockpid=$LOCK_PID" <<'SQL'
SELECT count(*) FROM pg_stat_activity a
WHERE host(a.client_addr) = :'ip'
  AND :lockpid = ANY(pg_blocking_pids(a.pid))
  AND EXISTS (SELECT 1 FROM pg_locks l WHERE l.pid = a.pid AND l.granted
              AND l.relation = 'backtest_results'::regclass);
SQL
)" || return 1
  [ "${n//[[:space:]]/}" -ge 1 ] 2>/dev/null
}
# 砍掉 container 之後，它被鎖擋住的 backend 不會自己發現 client 已經斷線：明確結束，並等到消失
terminate_from_ip() {
  q terminate_ip "ip=$1" >/dev/null <<'SQL'
SELECT pg_terminate_backend(pid) FROM pg_stat_activity
WHERE host(client_addr) = :'ip' AND application_name <> :'lockapp';
SQL
}
# 情境裡砍掉服務之後用：只結束「來自 IP、被本輪持鎖 PID 擋住」的 backend（新起來的 container 可能拿到同一個 IP）
terminate_blocked_from_ip() {
  q terminate_blocked "ip=$1" "lockpid=$LOCK_PID" >/dev/null <<'SQL'
SELECT pg_terminate_backend(pid) FROM pg_stat_activity
WHERE host(client_addr) = :'ip' AND :lockpid = ANY(pg_blocking_pids(pid));
SQL
}
no_blocked_from_ip() {
  local n
  n="$(q blocked_left "ip=$1" "lockpid=$LOCK_PID" <<'SQL'
SELECT count(*) FROM pg_stat_activity WHERE host(client_addr) = :'ip' AND :lockpid = ANY(pg_blocking_pids(pid));
SQL
)" || return 1
  [ "${n//[[:space:]]/}" = 0 ]
}
no_conn_from_ip() {
  local n
  n="$(q conn_from_ip "ip=$1" <<'SQL'
SELECT count(*) FROM pg_stat_activity WHERE host(client_addr) = :'ip' AND application_name <> :'lockapp';
SQL
)" || return 1
  [ "${n//[[:space:]]/}" = 0 ]
}

# ── 測試任務 ──
insert_jobs() {  # insert_jobs ID…（同一個交易）
  local ids; ids="$(IFS=,; echo "$*")"
  q insert_jobs "ids=$ids" "strategy=$SMOKE_STRATEGY" "symbols=[\"$SMOKE_SYMBOL\"]" "start=$SMOKE_START" "end=$SMOKE_END" >/dev/null <<'SQL'
INSERT INTO backtest_jobs (job_id, type, strategy, symbols, timeframe, start_date, end_date, status, trigger_source)
SELECT id, 'backtest', :'strategy', CAST(:'symbols' AS jsonb), '1d', CAST(:'start' AS date), CAST(:'end' AS date),
       'pending', 'manual'
FROM unnest(string_to_array(:'ids', ',')) AS id
WHERE starts_with(id, :'prefix');
SQL
}
job_state() {  # job_state ID → status|owner|error|finished(t/f)|results|trades|dup_trades|total_trades
  q job_state "id=$1" <<'SQL'
SELECT j.status || '|' || coalesce(j.execution_owner, '') || '|' || coalesce(j.error, '') || '|'
       || (j.finished_at IS NOT NULL)::text || '|'
       || (SELECT count(*) FROM backtest_results r WHERE r.job_id = j.job_id) || '|'
       || (SELECT count(*) FROM backtest_trades t WHERE t.job_id = j.job_id) || '|'
       || (SELECT count(*) - count(DISTINCT (t.symbol, t.direction, t.entry_time, t.exit_time))
           FROM backtest_trades t WHERE t.job_id = j.job_id) || '|'
       || coalesce((SELECT r.total_trades::text FROM backtest_results r WHERE r.job_id = j.job_id), '')
FROM backtest_jobs j
WHERE starts_with(j.job_id, :'prefix') AND j.job_id = :'id';
SQL
}
field() { cut -d'|' -f"$2" <<<"$1"; }
job_final() {  # job_final ID：done 或 failed
  local s; s="$(job_state "$1")" || return 1
  case "$(field "$s" 1)" in done|failed) return 0 ;; *) return 1 ;; esac
}
post_body() {
  printf '{"job_id":"%s","strategy":"%s","symbols":["%s"],"timeframe":"1d","start_date":"%s","end_date":"%s"}' \
    "$1" "$SMOKE_STRATEGY" "$SMOKE_SYMBOL" "$SMOKE_START" "$SMOKE_END"
}
post_job() {  # post_job ID → HTTP 狀態碼
  t "$(rem "$CALL_TIMEOUT")" curl -sS -o /dev/null -w '%{http_code}' -X POST -H 'Content-Type: application/json' \
    --data "$(post_body "$1")" "$SERVER_URL/backtest"
}
server_healthy() { t "$(rem "$CALL_TIMEOUT")" curl -fsS -o /dev/null "$SERVER_URL/health" 2>/dev/null; }
log_has() {  # log_has TS PATTERN SVC…
  local ts="$1" pat="$2" out; shift 2
  out="$(logs_since "$ts" "$@")" || return 1
  grep -qF -- "$pat" <<<"$out"
}
expect_log_lacks() {  # expect_log_lacks TS PATTERN SVC…：讀不到 log 也算失敗（⛔ 不能當成「沒有出現」）
  local ts="$1" pat="$2" out; shift 2
  out="$(logs_since "$ts" "$@")" || fail "讀不到 $* 的 log"
  ! grep -qF -- "$pat" <<<"$out" || fail "$* 的 log 出現了 $pat"
}
polls_since() {  # polls_since TS → worker 的 poll 次數
  local out; out="$(logs_since "$1" "$WRK")" || return 1
  grep -c 'poll #' <<<"$out" || true
}
pause_worker() { dc pause "$WRK" >/dev/null; }
unpause_worker() { dc unpause "$WRK" >/dev/null; }
start_svc() { compose "$(rem $((CALL_TIMEOUT * 2)))" up -d --no-build --no-deps "$1" >/dev/null; }
kill_svc() { dc kill -s KILL "$1" >/dev/null; }

# expect_job ID STATUS OWNER [ERROR]：最終狀態
expect_job() {
  local s; s="$(job_state "$1")" || fail "查不到 $1 的狀態"
  [ "$(field "$s" 1)" = "$2" ] || fail "$1 的狀態是 $(field "$s" 1)，預期 $2（$s）"
  [ "$(field "$s" 2)" = "$3" ] || fail "$1 的 execution_owner 是 $(field "$s" 2)，預期 $3"
  if [ -n "${4:-}" ]; then [ "$(field "$s" 3)" = "$4" ] || fail "$1 的 error 是 $(field "$s" 3)，預期 $4"; fi
}
expect_job_status() {
  local s; s="$(job_state "$1")" || fail "查不到 $1 的狀態"
  [ "$(field "$s" 1)" = "$2" ] || fail "$1 的狀態是 $(field "$s" 1)，預期 $2（error：$(field "$s" 3)）"
}
expect_done_once() {  # 結果 1 列、明細沒有重複、筆數等於 total_trades
  local s; s="$(job_state "$1")" || fail "查不到 $1 的狀態"
  [ "$(field "$s" 5)" = 1 ] || fail "$1 的結果有 $(field "$s" 5) 列，預期 1"
  [ "$(field "$s" 7)" = 0 ] || fail "$1 的交易明細有 $(field "$s" 7) 筆重複"
  [ "$(field "$s" 6)" = "$(field "$s" 8)" ] || fail "$1 的明細 $(field "$s" 6) 筆，結果的 total_trades 是 $(field "$s" 8)"
}
expect_rolled_back() {  # 結果與明細都 0 列、有 finished_at
  local s; s="$(job_state "$1")" || fail "查不到 $1 的狀態"
  [ "$(field "$s" 4)" = true ] || fail "$1 沒有 finished_at"
  [ "$(field "$s" 5)" = 0 ] && [ "$(field "$s" 6)" = 0 ] || fail "$1 的結果／明細沒有回滾（$(field "$s" 5)／$(field "$s" 6)）"
}

# ── 收尾 ──
CLEANUP_RC=0
# 收尾的各步驟在 subshell 裡跑（裡面的 fail／exit 只結束那一步），所以失敗另外留一個標記檔
cfail() { echo "ERROR（收尾）：$*" >&2; CLEANUP_RC=1; : >"$WORK/cleanup_failed" 2>/dev/null || true; }
stop_paths() {
  q cleanup_fail_pending >/dev/null <<'SQL' || cfail "把本次 pending 的任務改成 failed 失敗"
UPDATE backtest_jobs SET status = 'failed', error = 'i134smoke_cleanup', finished_at = CURRENT_TIMESTAMP
WHERE starts_with(job_id, :'prefix') AND status = 'pending';
SQL
  local svc cid st
  for svc in "$SRV" "$WRK"; do
    cid="$(cid_of "$svc")" || { cfail "查不到 $svc"; return 1; }
    [ -n "$cid" ] || continue
    st="$(state_of "$cid")" || { cfail "查不到 $svc 的狀態"; return 1; }
    if [ "$st" = paused ]; then dc unpause "$svc" >/dev/null || { cfail "unpause $svc 失敗"; return 1; }; fi
    dc stop "$svc" >/dev/null || { cfail "停止 $svc 失敗"; return 1; }
  done
}
paths_stopped() {
  local svc cid st ip
  for svc in "$SRV" "$WRK"; do
    cid="$(cid_of "$svc")" || return 1
    [ -n "$cid" ] || continue
    st="$(state_of "$cid")" || return 1
    case "$st" in running|paused|restarting) echo "    $svc 仍是 $st" >&2; return 1 ;; esac
  done
  for ip in ${KNOWN_IPS//,/ }; do
    terminate_from_ip "$ip" || return 1
  done
  for ip in ${KNOWN_IPS//,/ }; do
    wait_until "來自 $ip 的連線結束" no_conn_from_ip "$ip" || return 1
  done
}
delete_data() {
  q cleanup_delete >/dev/null <<'SQL'
DELETE FROM backtest_trades WHERE starts_with(job_id, :'prefix');
DELETE FROM backtest_results WHERE starts_with(job_id, :'prefix');
DELETE FROM backtest_jobs WHERE starts_with(job_id, :'prefix');
SQL
}
# 臨時 server 用**事先決定的名稱**移除與確認（review 2026-11-18：原本用 ID，`run` 之後取 ID 失敗時收尾就跳過移除、
#   仍然刪 DB）。確認不了已經不在（查詢失敗或仍查得到）就⛔ 不刪臨時 DB。
tmp_srv_left() {
  t "$(rem "$CALL_TIMEOUT")" docker ps -a -q --filter "name=^/${TMP_SRV_NAME}\$"
}
drop_tmpdb() {
  local left
  if [ "$TMP_SRV_STARTED" = 1 ]; then
    t "$(rem "$CALL_TIMEOUT")" docker rm -f "$TMP_SRV_NAME" >/dev/null 2>&1 || true
    if ! left="$(tmp_srv_left)"; then
      cfail "查不到臨時 server（$TMP_SRV_NAME）的狀態；⛔ 不刪臨時 DB $TMPDB"
      return 1
    fi
    if [ -n "$left" ]; then
      cfail "臨時 server（$TMP_SRV_NAME）還在；⛔ 不刪臨時 DB $TMPDB"
      return 1
    fi
  fi
  [ "$TMPDB_CREATED" = 1 ] || return 0
  q tmpdb_terminate "tmpdb=$TMPDB" >/dev/null <<'SQL' || { cfail "結束臨時 DB 的連線失敗；⛔ 不刪臨時 DB"; return 1; }
SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = :'tmpdb';
SQL
  q tmpdb_drop "tmpdb=$TMPDB" >/dev/null <<'SQL' || { cfail "刪除臨時 DB $TMPDB 失敗"; return 1; }
DROP DATABASE IF EXISTS :"tmpdb";
SQL
}
restore_services() {
  start_svc "$SRV" || cfail "起回 $SRV 失敗"
  case "$WRK_STATE0" in
    running) start_svc "$WRK" || cfail "起回 $WRK 失敗" ;;
    stopped) : ;;  # 原本存在但停止：維持停止，⛔ 不移除
    absent)
      if [ "$WRK_CREATED" = 1 ]; then
        compose "$(rem "$CALL_TIMEOUT")" rm -sf "$WRK" >/dev/null || cfail "移除本次建立的 $WRK 失敗"
      fi ;;
  esac
}
cleanup() {
  local rc=$?
  trap - EXIT INT TERM
  if [ "$STATE_CHANGED" = 1 ]; then
    IN_CLEANUP=1
    CDL=$((SECONDS + CLEANUP_TIMEOUT))
    log "收尾（最多 ${CLEANUP_TIMEOUT} 秒；時限用完就停下、列出沒做完的項目）"
    local stopped=0
    if ( stop_paths ) && ( paths_stopped ); then stopped=1; else cfail "無法確認 python 兩個服務已停止"; fi
    [ -n "$CLIENT_BG" ] && { kill "$CLIENT_BG" 2>/dev/null || true; wait "$CLIENT_BG" 2>/dev/null || true; }
    ( release_lock ) || cfail "持鎖連線沒有結束（application_name=$LOCK_APP）"
    if [ "$stopped" = 1 ]; then
      ( delete_data ) || cfail "刪除本次的測試資料失敗（前綴 $PREFIX）"
    fi
    ( drop_tmpdb ) || true
    if [ "$stopped" = 1 ]; then
      ( restore_services ) || cfail "恢復服務失敗"
      echo "    已恢復服務：$SRV 在跑；$WRK 依開始時的狀態（$WRK_STATE0）"
    else
      echo "ERROR：⛔ 沒有刪除測試資料、⛔ 沒有恢復服務。前綴：$PREFIX" >&2
      echo "  確認 python 兩個服務都已停止之後，手動清理（psql，dev DB）：" >&2
      echo "    DELETE FROM backtest_trades  WHERE starts_with(job_id, '$PREFIX');" >&2
      echo "    DELETE FROM backtest_results WHERE starts_with(job_id, '$PREFIX');" >&2
      echo "    DELETE FROM backtest_jobs    WHERE starts_with(job_id, '$PREFIX');" >&2
      dc ps -a "$SRV" "$WRK" >&2 2>&1 || true
    fi
    [ -e "$WORK/cleanup_failed" ] && CLEANUP_RC=1
    [ "$CLEANUP_RC" = 0 ] || { [ "$rc" = 0 ] && rc=1; }
  fi
  rm -rf "$WORK"
  exit "$rc"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

# ═══ 0. 只操作 dev project ═══
project="$(dc config 2>/dev/null | awk '/^name:/{print $2; exit}')" || project=""
[ "$project" = "$DEV_PROJECT" ] || die2 "$COMPOSE_FILE 解析出來的 project 是「${project:-（查不到）}」，不是 $DEV_PROJECT；⛔ 不操作"

# ═══ 1. 開始前的檢查（⛔ 不改任何東西）═══
log "開始前的檢查（前綴 $PREFIX）"
pg_cid="$(cid_of postgres)" || die2 "查不到 postgres"
[ -n "$pg_cid" ] && [ "$(state_of "$pg_cid")" = running ] || die2 "postgres 沒有在跑"
SRV_CID0="$(cid_of "$SRV")" || die2 "查不到 $SRV"
[ -n "$SRV_CID0" ] && [ "$(state_of "$SRV_CID0")" = running ] || die2 "$SRV 沒有在跑（先起 dev stack）"
WRK_CID0="$(cid_of "$WRK")" || die2 "查不到 $WRK"
if [ -z "$WRK_CID0" ]; then
  WRK_STATE0=absent
else
  case "$(state_of "$WRK_CID0")" in
    running) WRK_STATE0=running ;;
    exited|created|dead) WRK_STATE0=stopped ;;
    paused) die2 "$WRK 處於 paused；⛔ 不操作（先確認是誰暫停的）" ;;
    *) die2 "$WRK 的狀態不明" ;;
  esac
fi
pre="$(q precheck "symbol=$SMOKE_SYMBOL" "start=$SMOKE_START" "end=$SMOKE_END" <<'SQL'
SELECT (SELECT count(*) FROM information_schema.columns
        WHERE table_name = 'backtest_jobs' AND column_name = 'execution_owner') || '|'
    || (SELECT count(*) FROM backtest_jobs WHERE status IN ('pending', 'running')) || '|'
    || (SELECT count(*) FROM backtest_jobs WHERE starts_with(job_id, 'i134smoke_')) || '|'
    || (SELECT count(*) FROM candles WHERE symbol = :'symbol' AND timeframe = '1d'
        AND ts >= CAST(:'start' AS date) AND ts < CAST(:'end' AS date) + 1);
SQL
)" || die2 "查不到 dev DB（連不上、逾時或表不存在）"
pre="${pre//[[:space:]]/}"
[ "$(field "$pre" 1)" = 1 ] || die2 "dev DB 沒有 backtest_jobs.execution_owner（backend 還沒套用 migration 077）"
[ "$(field "$pre" 2)" = 0 ] || die2 "dev DB 有 $(field "$pre" 2) 筆 pending／running 的回測；⛔ 不操作（會被一起執行或誤判）"
[ "$(field "$pre" 3)" = 0 ] || die2 "dev DB 還有之前留下的 i134smoke_ 任務（$(field "$pre" 3) 筆）；先確認、清掉再重跑"
[ "$(field "$pre" 4)" -gt 0 ] 2>/dev/null || die2 "$SMOKE_SYMBOL 在 $SMOKE_START～$SMOKE_END 沒有日 K"
# trades 的費用核對要用 config 的原始費率：開始前先確認讀得到（⛔ 還沒改任何東西，讀不到就結束碼 2）。
#   config 是 build 時複製進 image 的，實際用的值在換上本次的 image 之後、寫入任務之前再讀一次（scenario_trades）。
if [[ " ${SCEN_LIST[*]} " == *" trades "* ]]; then
  RATES0="$(read_rates)" || die2 "讀不到 $SRV 的 config 費率（COMMISSION_RATE、TAX_RATE），或不是有限的非負數"
  echo "    $SRV 目前的費率（config）：手續費 ${RATES0% *}、交易稅 ${RATES0#* }"
fi
echo "    worker 開始時：$WRK_STATE0；$SMOKE_SYMBOL 日 K $(field "$pre" 4) 根"

# ═══ 2. build 與換上本次的 image ═══
STATE_CHANGED=1
if [ "$NO_BUILD" = 1 ]; then
  log "略過 build（NO_BUILD=1）"
else
  sec="$(rem "$BUILD_TIMEOUT")"
  log "build $SRV、$WRK（最多 ${sec} 秒）"
  compose "$sec" build "$SRV" "$WRK" >/dev/null || fail "build 失敗或逾時"
fi
start_svc "$SRV" || fail "起 $SRV 失敗"
if [ "$WRK_STATE0" = absent ]; then WRK_CREATED=1; fi
if [ "$WRK_STATE0" = stopped ]; then
  # 原本存在但停止的 worker：⛔ 不重建、⛔ 不移除。它的 image 不是本次的就停下。
  want="$(t "$(rem "$CALL_TIMEOUT")" docker image inspect -f '{{.Id}}' "${DEV_PROJECT}-${WRK}")" || want=""
  have="$(t "$(rem "$CALL_TIMEOUT")" docker inspect -f '{{.Image}}' "$WRK_CID0")" || have=""
  [ -n "$want" ] && [ "$want" = "$have" ] \
    || fail "$WRK 原本存在但停止，而且不是本次的 image；⛔ 不重建它。確認之後自己移除或啟動再重跑"
  t "$(rem "$CALL_TIMEOUT")" docker start "$WRK_CID0" >/dev/null || fail "啟動原本停止的 $WRK 失敗"
else
  start_svc "$WRK" || fail "起 $WRK 失敗"
fi
wait_until "$SRV /health" server_healthy
SRV_CID="$(cid_of "$SRV")"; WRK_CID="$(cid_of "$WRK")"
remember_ip SRV_IP "$SRV_CID"; remember_ip WRK_IP "$WRK_CID"
[ -n "$SRV_IP" ] && [ -n "$WRK_IP" ] || fail "查不到 python 服務的 IP"
wait_until "$WRK 開始輪詢" log_has 1970-01-01T00:00:00Z "worker started" "$WRK"

# ═══ 情境 ═══
scenario_race() {
  local ts ids=() pids=() i id codes s lines both=0 wins_w=0 wins_s=0
  ts="$(now_ts)"
  for i in 1 2 3 4 5; do ids+=("${PREFIX}race_$i"); done
  log "race：一次寫入 5 筆 pending，等 worker 撈到這一批之後並行送出 5 個 POST"
  insert_jobs "${ids[@]}"
  wait_until "worker 撈到 5 筆" log_has "$ts" "found 5 pending job(s)" "$WRK"
  for id in "${ids[@]}"; do
    ( post_job "$id" >"$WORK/code_$id" 2>/dev/null || echo err >"$WORK/code_$id" ) &
    pids+=($!)
  done
  for i in "${pids[@]}"; do wait "$i" || true; done
  for id in "${ids[@]}"; do
    codes="$(cat "$WORK/code_$id")"
    case "$codes" in 202|409) ;; *) fail "POST $id 回 $codes（預期 202 或 409）" ;; esac
    wait_until "$id 結束" job_final "$id"
    expect_job_status "$id" "done"
  done
  lines="$(logs_since "$ts" "$SRV" "$WRK")" || fail "讀不到 log"
  for id in "${ids[@]}"; do
    s="$(job_state "$id")"
    local okw oks lw ls owner
    okw="$(grep -cF "claim ok job=$id owner=worker" <<<"$lines" || true)"
    oks="$(grep -cF "claim ok job=$id owner=server" <<<"$lines" || true)"
    lw="$(grep -cF "claim lost job=$id owner=worker" <<<"$lines" || true)"
    ls="$(grep -cF "claim lost job=$id owner=server" <<<"$lines" || true)"
    owner="$(field "$s" 2)"
    [ $((okw + oks)) = 1 ] || fail "$id 的 claim ok 有 $((okw + oks)) 次（預期 1）"
    if [ "$okw" = 1 ]; then [ "$owner" = worker ] || fail "$id 由 worker 認領，owner 卻是 $owner"; wins_w=$((wins_w + 1)); fi
    if [ "$oks" = 1 ]; then [ "$owner" = server ] || fail "$id 由 server 認領，owner 卻是 $owner"; wins_s=$((wins_s + 1)); fi
    expect_done_once "$id"
    if [ $((okw + lw)) -ge 1 ] && [ $((oks + ls)) -ge 1 ]; then both=$((both + 1)); fi
  done
  [ "$both" -ge 1 ] || fail "race 沒有覆蓋競爭：沒有任何一筆同時有 worker 與 server 的認領紀錄（⛔ 不算通過）"
  echo "    通過：5 筆都只執行一次；兩邊都嘗試過的有 $both 筆；worker 贏 $wins_w、server 贏 $wins_s（只做記錄）"
}

restart_case() {  # restart_case VICTIM：VICTIM 是被砍的那一個（server 或 worker）
  local victim="$1" w s vic_svc vic_ip vic_id other_id other_owner ts old_ip
  w="${PREFIX}${victim}restart_w"; s="${PREFIX}${victim}restart_s"
  start_lock
  if [ "$victim" = server ]; then
    insert_jobs "$w"
    wait_until "worker 認領 $w 並卡在明細（回測要至少一筆交易）" blocked_by_lock "$WRK_IP"
    expect_job "$w" running worker
    pause_worker
    insert_jobs "$s"
    [ "$(post_job "$s")" = 202 ] || fail "POST $s 沒有回 202"
    wait_until "server 認領 $s 並卡在明細" blocked_by_lock "$SRV_IP"
    vic_svc="$SRV"; vic_ip="$SRV_IP"; vic_id="$s"; other_id="$w"; other_owner=worker
  else
    pause_worker
    insert_jobs "$s"
    [ "$(post_job "$s")" = 202 ] || fail "POST $s 沒有回 202"
    wait_until "server 認領 $s 並卡在明細" blocked_by_lock "$SRV_IP"
    insert_jobs "$w"
    unpause_worker
    wait_until "worker 認領 $w 並卡在明細" blocked_by_lock "$WRK_IP"
    vic_svc="$WRK"; vic_ip="$WRK_IP"; vic_id="$w"; other_id="$s"; other_owner=server
  fi
  log "$victim-restart：kill -s KILL $vic_svc（$vic_id 已寫入結果、卡在明細）"
  ts="$(now_ts)"
  kill_svc "$vic_svc"
  old_ip="$vic_ip"
  terminate_blocked_from_ip "$old_ip" || fail "結束 $vic_svc 的殘留連線失敗"
  wait_until "$vic_svc 的殘留連線結束" no_blocked_from_ip "$old_ip"
  start_svc "$vic_svc" || fail "起回 $vic_svc 失敗"
  if [ "$victim" = server ]; then
    wait_until "$SRV /health" server_healthy
    SRV_CID="$(cid_of "$SRV")"; remember_ip SRV_IP "$SRV_CID"
  else
    WRK_CID="$(cid_of "$WRK")"; remember_ip WRK_IP "$WRK_CID"
  fi
  wait_until "$vic_svc 收掉 $vic_id" log_has "$ts" "owner=$victim: $vic_id" "$vic_svc"
  expect_job "$vic_id" failed "$victim" interrupted_on_restart
  expect_rolled_back "$vic_id"
  expect_job "$other_id" running "$other_owner"
  release_lock || fail "放鎖失敗"
  [ "$victim" = server ] && unpause_worker
  wait_until "$other_id 完成" job_final "$other_id"
  expect_job "$other_id" "done" "$other_owner"
  expect_done_once "$other_id"
  echo "    通過：$vic_id → failed／interrupted_on_restart、結果與明細都回滾；$other_id 不受影響、最後 done"
}

scenario_lost_response() {
  local id="${PREFIX}lost_1" ts_all ts_resume
  ts_all="$(now_ts)"
  pause_worker
  start_lock
  insert_jobs "$id"
  log "lost-response：不讀回應的 socket client 送出 POST，認領提交、卡在明細之後直接 RST"
  # 連線、送出完整的請求、印 sent，之後⛔ 不讀；被 TERM 時 SO_LINGER 0 關閉（RST，丟掉沒讀的回應）
  # ⚠️ 直接呼叫 timeout（⛔ 不經過 t 函式）：背景執行函式時 $! 是 subshell，kill 它會留下 timeout 與 python3
  local csec; csec="$(rem "$LOCK_TIMEOUT")"
  (( csec >= 1 )) || fail "總時限用完，⛔ 不啟動 socket client"
  timeout -k "$KILL_AFTER" "$csec" python3 -I -c '
import signal, socket, struct, sys, time
host, port, body = sys.argv[1], int(sys.argv[2]), sys.argv[3].encode()
s = socket.create_connection((host, port), timeout=10)
req = (f"POST /backtest HTTP/1.1\r\nHost: {host}:{port}\r\nContent-Type: application/json\r\n"
       f"Content-Length: {len(body)}\r\nConnection: close\r\n\r\n").encode() + body
s.sendall(req)
print("sent", flush=True)
def bye(*_):
    s.setsockopt(socket.SOL_SOCKET, socket.SO_LINGER, struct.pack("ii", 1, 0))
    s.close()
    print("closed", flush=True)
    sys.exit(0)
signal.signal(signal.SIGTERM, bye)
while True:
    time.sleep(1)
' 127.0.0.1 "${SERVER_URL##*:}" "$(post_body "$id")" </dev/null >"$WORK/client.out" 2>&1 &
  CLIENT_BG=$!
  wait_until "client 送出請求" grep -qs '^sent$' "$WORK/client.out"
  wait_until "server 認領 $id 並卡在明細" blocked_by_lock "$SRV_IP"
  expect_job "$id" running server
  kill "$CLIENT_BG" 2>/dev/null || true; wait "$CLIENT_BG" 2>/dev/null || true; CLIENT_BG=""
  grep -qs '^closed$' "$WORK/client.out" || fail "socket client 沒有照預期以 RST 關閉連線"
  ts_resume="$(now_ts)"
  unpause_worker
  wait_until "worker 恢復後輪詢 3 輪" polls_ge3 "$ts_resume"
  expect_job "$id" running server
  expect_log_lacks "$ts_all" "$id" "$WRK"
  release_lock || fail "放鎖失敗"
  wait_until "$id 完成" job_final "$id"
  expect_job "$id" "done" server
  expect_log_lacks "$ts_all" "$id" "$WRK"
  expect_done_once "$id"
  echo "    通過：回應遺失之後 worker 三輪都沒有接手；$id 由 server 完成、明細只有一份"
}
polls_ge3() { local n; n="$(polls_since "$1")" || return 1; [ "$n" -ge 3 ]; }

scenario_schema() {
  local ts
  log "schema：臨時 DB 沒有 execution_owner → 臨時 server 等待 → ALTER 之後繼續啟動（077 本身由 migration 測試負責）"
  q tmpdb_create "tmpdb=$TMPDB" >/dev/null <<'SQL' || fail "建立臨時 DB 失敗"
CREATE DATABASE :"tmpdb";
SQL
  TMPDB_CREATED=1
  t "$(rem "$CALL_TIMEOUT")" docker compose -f "$COMPOSE_FILE" --profile worker exec -T postgres sh -c \
    "pg_dump -U trading -d trading -s -t 'public.backtest_*' | psql -U trading -d $TMPDB -X -q -v ON_ERROR_STOP=1 -f -" \
    >/dev/null || fail "複製回測三張表的結構失敗"
  SQL_DB="$TMPDB" q tmpdb_drop_col >/dev/null <<'SQL' || fail "拿掉臨時 DB 的 execution_owner 失敗"
ALTER TABLE backtest_jobs DROP COLUMN execution_owner;
SQL
  ts="$(now_ts)"
  # ⛔ 不發布 port（compose run 預設不發布；⛔ 不加 --service-ports）。
  # 先標記「已起」再執行：run 失敗或逾時也可能已經建立了 container，收尾一律依名稱移除。
  TMP_SRV_STARTED=1
  dc run -d --rm --no-deps --name "$TMP_SRV_NAME" -e LOG_DIR=/tmp/i134smoke-logs \
    -e "DATABASE_DSN=postgresql+psycopg2://trading:trading@postgres:5432/$TMPDB" "$SRV" >/dev/null || fail "起臨時 server 失敗"
  TMP_SRV_ID="$(t "$(rem "$CALL_TIMEOUT")" docker inspect -f '{{.Id}}' "$TMP_SRV_NAME")" || fail "查不到臨時 server"
  wait_until "臨時 server 等 schema" tmp_log_has "waiting for schema"
  SQL_DB="$TMPDB" q tmpdb_add_col >/dev/null <<'SQL' || fail "臨時 DB 加回 execution_owner 失敗"
ALTER TABLE backtest_jobs ADD COLUMN execution_owner VARCHAR(16);
SQL
  wait_until "臨時 server 繼續啟動" tmp_log_has "schema ready"
  wait_until "臨時 server 完成啟動收尾" tmp_log_has "no interrupted jobs to reap owner=server"
  [ "$(t "$(rem "$CALL_TIMEOUT")" docker inspect -f '{{.State.Status}}' "$TMP_SRV_ID")" = running ] || fail "臨時 server 沒有在跑"
  ( drop_tmpdb ) || fail "清掉臨時 server 與臨時 DB 失敗"
  TMP_SRV_ID=""; TMP_SRV_STARTED=0; TMPDB_CREATED=0
  echo "    通過：欄位出現之後臨時 server 繼續啟動；臨時 server 與臨時 DB 已清掉"
}
tmp_log_has() { local out; out="$(t "$(rem "$CALL_TIMEOUT")" docker logs "$TMP_SRV_ID" 2>&1)" || return 1; grep -qF -- "$1" <<<"$out"; }

# trades（I-135）：backtrader 路徑寫進 backtest_trades 的明細——筆數、價格與股數、帶方向的精度界線、台北時間。
#   認領的路徑誰都可以（202 或 409 都接受），驗的是寫進去的內容。
scenario_trades() {
  local id="${PREFIX}trades_1" code s bad rates c tx
  log "trades：$SMOKE_STRATEGY 的交易明細（筆數、價格與股數、帶方向的精度界線、台北時間、費用）"
  # 預期費用用執行這次回測的 image 載入的 config 原始費率（寫入任務之前讀）
  rates="$(read_rates)" || fail "讀不到 $SRV 的 config 費率（COMMISSION_RATE、TAX_RATE），或不是有限的非負數"
  c="${rates% *}"; tx="${rates#* }"
  echo "    費率（本次的 image 的 config）：手續費 $c、交易稅 $tx"
  insert_jobs "$id"
  code="$(post_job "$id")" || code=err
  case "$code" in 202|409) ;; *) fail "POST $id 回 $code（預期 202 或 409）" ;; esac
  wait_until "$id 結束" job_final "$id"
  expect_job_status "$id" "done"
  expect_done_once "$id"
  s="$(job_state "$id")" || fail "查不到 $id 的狀態"
  [ "$(field "$s" 6)" -ge 1 ] 2>/dev/null || fail "$id 沒有任何交易明細（這組資料應該要有交易）"
  # 共用查詢包進 PREPARE，再用 psql 變數 EXECUTE（$1＝job_id、$2＝前綴）
  bad="$( { printf 'PREPARE trades_check(text, text) AS\n'; cat "$TRADES_CHECK_SQL"
            printf ";\nEXECUTE trades_check(:'id', :'prefix');\n"; } | q trades_check "id=$id")" || fail "查不到 $id 的交易明細"
  bad="${bad//[[:space:]]/}"
  [ "$bad" = 0 ] || fail "$id 有 $bad 筆明細不符合規格（必要欄位是 NULL 或 NaN、價格／股數不是正數、超出精度界線，或時間不是台北的日 K 時刻）"
  # 費用核對（I-136）：commission 跟 config 原始費率算出的模擬費用（手續費＋賣出的交易稅）差在界線內
  bad="$( { printf 'PREPARE fee_check(text, text, numeric, numeric) AS\n'; cat "$TRADES_FEE_SQL"
            printf ";\nEXECUTE fee_check(:'id', :'prefix', :'comm_rate', :'tax_rate');\n"; } \
          | q trades_fee_check "id=$id" "comm_rate=$c" "tax_rate=$tx")" || fail "查不到 $id 的費用核對"
  bad="${bad//[[:space:]]/}"
  [ "$bad" = 0 ] || fail "$id 有 $bad 筆明細的 commission 跟 config 費率（手續費 $c、交易稅 $tx）算出的模擬費用對不上"
  echo "    通過：$(field "$s" 6) 筆交易明細，價格與股數都是正數、符合帶方向的精度界線、時間是台北時間、費用符合 config 費率"
}

for sc in "${SCEN_LIST[@]}"; do
  case "$sc" in
    race) scenario_race ;;
    server-restart) restart_case server ;;
    worker-restart) restart_case worker ;;
    lost-response) scenario_lost_response ;;
    schema) scenario_schema ;;
    trades) scenario_trades ;;
  esac
done
log "全部通過：${SCEN_LIST[*]}"
