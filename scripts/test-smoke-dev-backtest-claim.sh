#!/usr/bin/env bash
# scripts/smoke-dev-backtest-claim.sh 的測試（由來：I-134）。⛔ 不需要真的 docker：假的 docker／curl／python3
# 記錄呼叫並照 FAKE_* 回應，驗證參數與前置檢查的停止點、測試資料只用本次前綴（字面比對）、收尾的順序與條件、
# worker 三種開始狀態的恢復、持鎖連線與 socket client 一定被結束、卡住又忽略 SIGTERM 的呼叫被 -k 砍掉。
#
# 用法：scripts/test-smoke-dev-backtest-claim.sh；結束碼 0 全部通過、1 有失敗。
#   SMOKE_SCRIPT=/path/to/copy.sh 可以改測另一份（突變測試用）。
# shellcheck disable=SC2015,SC2016
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="${SMOKE_SCRIPT:-$REPO_ROOT/scripts/smoke-dev-backtest-claim.sh}"
WORK="$(mktemp -d "${TMPDIR:-/tmp}/test-smoke-dev-backtest-claim.XXXXXX")"
trap 'rm -rf "$WORK"' EXIT

PASS=0
FAIL=0
ok() { PASS=$((PASS + 1)); echo "  ok   $*"; }
ng() { FAIL=$((FAIL + 1)); echo "  FAIL $*"; }

FAKEBIN="$WORK/bin"
mkdir -p "$FAKEBIN"

cat > "$FAKEBIN/docker" <<'EOF'
#!/bin/bash
# 假 docker：照 smoke-dev-backtest-claim.sh 的呼叫形式回應。狀態放在 $STATE，每次呼叫記一行到 $CALL_LOG。
echo "docker $*" >> "$CALL_LOG"
GOOD=sha256:1111111111111111111111111111111111111111111111111111111111111111
st() { cat "$STATE/$1" 2>/dev/null || echo absent; }
set_st() { echo "$2" > "$STATE/$1"; }
# FAKE_HANG=<子指令>：那個子指令卡住；FAKE_IGNORE_TERM=1 時還會忽略 SIGTERM（只有 SIGKILL 停得了）
hang() {
  [ "${FAKE_HANG:-}" = "$1" ] || return 0
  if [ "${FAKE_IGNORE_TERM:-0}" = 1 ]; then trap '' TERM; exec /bin/sleep 31; fi
  exec /bin/sleep 30
}
P="$(cat "$STATE/prefix" 2>/dev/null)"
# FAKE_HANG_AFTER_CLEANUP=1：收尾開始之後（看到 cleanup_fail_pending），每一個 docker 呼叫都卡住而且忽略 SIGTERM
if [ "${FAKE_HANG_AFTER_CLEANUP:-0}" = 1 ] && [ -f "$STATE/cleanup_started" ]; then trap '' TERM; exec /bin/sleep 31; fi

case "$1" in
  ps)
    hang ps
    case "$*" in
      *name=*) [ "${FAKE_TMP_PS_FAIL:-0}" = 1 ] && exit 1; [ "$(st tmp)" != absent ] && echo tmpcid ;;
      *service=postgres*) echo pgcid ;;
      *service=python-server*) [ "$(st srv)" != absent ] && echo srvcid ;;
      *service=python-worker*) [ "$(st wrk)" != absent ] && cat "$STATE/wrkcid" ;;
    esac
    exit 0 ;;
  inspect)
    hang inspect
    fmt="$3"; cid="$4"
    case "$fmt" in
      *State.Status*)
        case "$cid" in
          pgcid) echo running ;;
          srvcid) st srv ;;
          wrk*) st wrk ;;
          tmpcid) [ "$(st tmp)" = absent ] && exit 1; st tmp ;;
          *) exit 1 ;;
        esac ;;
      *NetworkSettings*)
        case "$cid" in srvcid) echo "172.18.0.5 " ;; wrk*) echo "172.18.0.6 " ;; esac ;;
      *'{{.Image}}'*)
        case "$cid" in wrkold) echo "${FAKE_WRKOLD_IMAGE:-$GOOD}" ;; *) echo "$GOOD" ;; esac ;;
      *'{{.Id}}'*)
        [ "${FAKE_TMP_ID_FAIL:-0}" = 1 ] && exit 1
        [ "$(st tmp)" = absent ] && exit 1; echo tmpcid ;;
    esac
    exit 0 ;;
  image) echo "$GOOD"; exit 0 ;;
  start) set_st wrk running; exit 0 ;;
  rm) [ "${FAKE_TMP_STAYS:-0}" = 1 ] || set_st tmp absent; exit 0 ;;
  logs)
    if [ "${FAKE_TMP_NOWAIT:-0}" = 0 ]; then
      # 每呼叫一次 logs 就多一輪探測；077 與 078 都補回之後才就緒（FAKE_READY_WITH_077=1：只補 077 就就緒＝探測漏了 078）
      n=$(( $(cat "$STATE/waits" 2>/dev/null || echo 0) + 1 )); echo "$n" > "$STATE/waits"
      for _ in $(seq "$n"); do
        echo "INFO backtest.db_writer — waiting for schema (backtest_jobs.execution_owner (077), backtest_results.data_coverage (078)), retry in 2s"
      done
      if [ -f "$STATE/addcol" ] && { [ -f "$STATE/addcov" ] || [ "${FAKE_READY_WITH_077:-0}" = 1 ]; }; then
        echo "INFO backtest.db_writer — schema ready: backtest_jobs.execution_owner (077), backtest_results.data_coverage (078) exist"
        echo "INFO backtest.db_writer — no interrupted jobs to reap owner=server"
      fi
    fi
    exit 0 ;;
  compose) ;;
  *) echo "fake docker: 沒預期的呼叫：$*" >&2; exit 97 ;;
esac

shift
while [ $# -gt 0 ]; do
  case "$1" in -f|--profile) shift 2 ;; *) break ;; esac
done
sub="$1"; shift
hang "$sub"
svc="${*: -1}"
key() { case "$1" in python-server) echo srv ;; python-worker) echo wrk ;; esac; }
case "$sub" in
  config) printf 'name: %s\nservices:\n  postgres:\n    image: x\n' "${FAKE_PROJECT-stock_trading_dev}"; exit 0 ;;
  build) exit "${FAKE_BUILD_RC:-0}" ;;
  up)
    k="$(key "$svc")"
    if [ "$k" = wrk ] && [ "$(st wrk)" = absent ]; then echo wrknew > "$STATE/wrkcid"; fi
    set_st "$k" running; exit 0 ;;
  stop)
    k="$(key "$svc")"
    [ "${FAKE_STOP_FAIL:-0}" = 1 ] && exit 1
    [ "${FAKE_STOP_NOOP:-0}" = 1 ] && exit 0
    if [ "$(st "$k")" = paused ]; then echo "STOP_WHILE_PAUSED $svc" >> "$CALL_LOG"; exit 0; fi
    set_st "$k" exited; exit 0 ;;
  kill) set_st "$(key "$svc")" exited; exit 0 ;;
  pause) set_st "$(key "$svc")" paused; exit 0 ;;
  unpause) set_st "$(key "$svc")" running; exit 0 ;;
  rm) set_st wrk absent; rm -f "$STATE/wrkcid"; exit 0 ;;
  run) set_st tmp running; exit 0 ;;
  ps) echo "fake ps"; exit 0 ;;
  logs)
    case " $* " in
      *" python-worker "*)
        echo "python-worker-1  | INFO worker started — poll_interval=10s"
        echo "python-worker-1  | INFO poll #1: found 5 pending job(s)"
        for i in 2 3 4 5; do echo "python-worker-1  | INFO poll #$i: no pending jobs, sleeping 10s"; done ;;
    esac
    for i in 1 2 3 4 5; do
      id="${P}race_$i"
      if [ "$i" = 1 ]; then
        case " $* " in *" python-worker "*) echo "python-worker-1  | INFO claim ok job=$id owner=worker" ;; esac
        [ "${FAKE_RACE_NO_OVERLAP:-0}" = 1 ] || case " $* " in *" python-server "*) echo "python-server-1  | INFO claim lost job=$id owner=server (not pending)" ;; esac
      else
        case " $* " in *" python-server "*) echo "python-server-1  | INFO claim ok job=$id owner=server" ;; esac
        [ "${FAKE_RACE_NO_OVERLAP:-0}" = 1 ] || case " $* " in *" python-worker "*) echo "python-worker-1  | INFO claim lost job=$id owner=worker (not pending)" ;; esac
      fi
    done
    exit 0 ;;
  exec)
    # exec -T python-server python -c …：讀 config 的費率（I-136）。第幾次讀記在 $STATE/rates_n
    if [[ " $* " == *" python-server python "* ]]; then
      n=$(( $(cat "$STATE/rates_n" 2>/dev/null || echo 0) + 1 )); echo "$n" > "$STATE/rates_n"
      echo "RATES_READ $n" >> "$CALL_LOG"
      [ "${FAKE_RATES_FAIL:-0}" = 1 ] && exit 1
      [ "${FAKE_RATES_FAIL_AFTER:-0}" = 1 ] && [ "$n" -ge 2 ] && exit 1
      echo "${FAKE_RATES-0.001425 0.003}"; exit 0
    fi ;;
  *) echo "fake docker: 沒預期的 compose 子指令：$sub $*" >&2; exit 97 ;;
esac

# exec [-T] [-e K=V] postgres CMD…
args=("$@")
i=0
while [ "${args[$i]}" != postgres ]; do i=$((i + 1)); done
cmd="${args[$((i + 1))]}"
[ "$cmd" = psql ] || exit 0   # pg_dump | psql（schema 情境）
id=""
for ((j = i + 1; j < ${#args[@]}; j++)); do
  if [ "${args[$j]}" = -v ]; then
    kv="${args[$((j + 1))]}"
    case "$kv" in
      prefix=*) echo "${kv#prefix=}" > "$STATE/prefix"; echo "PSQL_PREFIX ${kv#prefix=}" >> "$CALL_LOG" ;;
      id=*) id="${kv#id=}" ;;
    esac
  fi
done
sql="$(cat)"
name="$(head -1 <<<"$sql" | sed -n 's/^-- q:\(.*\)$/\1/p')"
echo "SQL $name $(tr '\n' ' ' <<<"$sql")" >> "$CALL_LOG"
hang "q:$name"
case "$name" in
  precheck) echo "${FAKE_PRECHECK:-1|0|0|485|1}" ;;
  lock_hold) exec /bin/sleep 61 ;;
  lock_pid) echo 4242 ;;
  lock_left) echo "${FAKE_LOCK_LEFT:-0}" ;;
  blocked) [ "${FAKE_NEVER_BLOCKED:-0}" = 1 ] && echo 0 || echo 1 ;;
  conn_from_ip|blocked_left) echo "${FAKE_CONN_LEFT:-0}" ;;
  job_state)
    case "$id" in
      *race_1) echo "done|worker||true|1|4|0|4" ;;
      *race_*) echo "done|server||true|1|4|0|4" ;;
      *trades_*) [ "${FAKE_TRADES_FAILED:-0}" = 1 ] && echo "failed|server|boom|true|0|0|0|" || echo "done|server||true|1|3|0|3" ;;
      *) echo "running|server||false|0|0|0|" ;;
    esac ;;
  cleanup_fail_pending) touch "$STATE/cleanup_started" ;;
  trades_check) [ "${FAKE_TRADES_CHECK_FAIL:-0}" = 1 ] && exit 1; echo "${FAKE_TRADES_BAD:-0}" ;;
  trades_fee_check) [ "${FAKE_FEE_CHECK_FAIL:-0}" = 1 ] && exit 1; echo "${FAKE_FEE_BAD:-0}" ;;
  cleanup_delete) [ "${FAKE_DELETE_FAIL:-0}" = 1 ] && exit 1 ;;
  tmpdb_add_col) touch "$STATE/addcol" ;;
  tmpdb_add_cov) touch "$STATE/addcov" ;;
  trades_coverage) [ "${FAKE_COVERAGE_FAIL:-0}" = 1 ] && exit 1; echo "${FAKE_COVERAGE-487|2024-01-02|2025-12-31|250|false|487}" ;;
esac
exit 0
EOF

cat > "$FAKEBIN/curl" <<'EOF'
#!/bin/bash
echo "curl $*" >> "$CALL_LOG"
case "$*" in
  */health*) exit "${FAKE_HEALTH_RC:-0}" ;;
  *) printf '%s' "${FAKE_POST_CODE:-202}" ;;
esac
EOF

cat > "$FAKEBIN/python3" <<'EOF'
#!/bin/bash
# 假的 socket client：送出之後就等著被結束；收到 TERM 時跟真的一樣印 closed
echo "python3-client" >> "$CALL_LOG"
/bin/sleep 62 &
SP=$!
trap 'kill "$SP"; echo closed; exit 0' TERM
echo sent
wait "$SP"
EOF
chmod +x "$FAKEBIN/docker" "$FAKEBIN/curl" "$FAKEBIN/python3"

# run CASE [ENV=VAL…]：時限都縮短；FAKE_SRV0／FAKE_WRK0 是開始時的服務狀態
run() {
  CASE="$WORK/$1"; shift
  rm -rf "$CASE"; mkdir -p "$CASE/state"; : > "$CASE/calls"
  local kv srv0=running wrk0=absent
  for kv in "$@"; do
    case "$kv" in FAKE_SRV0=*) srv0="${kv#*=}" ;; FAKE_WRK0=*) wrk0="${kv#*=}" ;; esac
  done
  echo "$srv0" > "$CASE/state/srv"
  echo "$wrk0" > "$CASE/state/wrk"
  [ "$wrk0" != absent ] && echo wrkold > "$CASE/state/wrkcid"
  local t0=$SECONDS
  env -i PATH="$FAKEBIN:/usr/bin:/bin" HOME="$WORK" TMPDIR="$CASE" CALL_LOG="$CASE/calls" STATE="$CASE/state" \
    NO_BUILD=1 SCENARIOS=race TOTAL_TIMEOUT=30 STEP_TIMEOUT=3 CALL_TIMEOUT=2 LOCK_TIMEOUT=10 \
    CLEANUP_TIMEOUT=15 KILL_AFTER=1 CHECK_INTERVAL=1 "$@" bash "$SCRIPT" > "$CASE/out" 2>&1
  RC=$?
  ELAPSED=$((SECONDS - t0))
}
called() { grep -q -- "$1" "$CASE/calls"; }
count() { grep -c -- "$1" "$CASE/calls"; }
first() { grep -n -- "$1" "$CASE/calls" | head -1 | cut -d: -f1; }
last() { grep -n -- "$1" "$CASE/calls" | tail -1 | cut -d: -f1; }
srv_state() { cat "$CASE/state/srv"; }
wrk_state() { cat "$CASE/state/wrk"; }

# 會改變狀態的呼叫
CHANGING='compose .* \(up\|stop\|kill\|pause\|unpause\|rm\|run\|build\) \|^docker start \|^docker rm \|^SQL \(insert_jobs\|cleanup\|lock_hold\|tmpdb\)'
expect_untouched() {
  local name="$1" want="$2"
  if [ "$RC" -eq "$want" ] && ! grep -q "$CHANGING" "$CASE/calls"; then
    ok "$name → rc $RC、⛔ 沒有改任何東西"
  else
    ng "$name：rc $RC（預期 $want）、改變狀態的呼叫：$(grep -c "$CHANGING" "$CASE/calls")"; sed 's/^/      /' "$CASE/out" | tail -5
  fi
}
no_orphans() { ! pgrep -fx "/bin/sleep $1" >/dev/null 2>&1; }

STOP_SRV='compose .* stop python-server'
DELETE='^SQL cleanup_delete'
RELEASE='^SQL lock_release'
CONFIRM='^SQL conn_from_ip'
UP_SRV='compose .* up -d --no-build --no-deps python-server'
UP_WRK='compose .* up -d --no-build --no-deps python-worker'
RM_WRK='compose .* rm -sf python-worker'

echo "== 參數與 project"
run param_project_name COMPOSE_PROJECT_NAME=
expect_untouched "COMPOSE_PROJECT_NAME 設成空字串" 2
[ "$(wc -l < "$CASE/calls")" -eq 0 ] && ok "  連一個 docker 呼叫都沒有" || ng "  有 docker 呼叫"
run param_scenario SCENARIOS=race,bogus
expect_untouched "不認得的情境" 2
run param_timeout TOTAL_TIMEOUT=0
expect_untouched "TOTAL_TIMEOUT=0" 2
run param_url SERVER_URL=http://example.com:8001
expect_untouched "SERVER_URL 不是 127.0.0.1" 2
run param_strategy "SMOKE_STRATEGY=x';drop"
expect_untouched "SMOKE_STRATEGY 有引號" 2
run wrong_project FAKE_PROJECT=stock_trading
expect_untouched "project 不是 stock_trading_dev" 2

echo "== 開始前的檢查"
run pre_no_owner "FAKE_PRECHECK=0|0|0|485|1"
expect_untouched "沒有 execution_owner（077 還沒套用）" 2
run pre_pending "FAKE_PRECHECK=1|2|0|485|1"
expect_untouched "有 pending／running" 2
run pre_leftover "FAKE_PRECHECK=1|0|3|485|1"
expect_untouched "有之前留下的 i134smoke_ 任務" 2
run pre_nocandle "FAKE_PRECHECK=1|0|0|0|1"
expect_untouched "沒有日 K" 2
run pre_no_coverage "FAKE_PRECHECK=1|0|0|485|0"
expect_untouched "沒有 data_coverage（078 還沒套用；I-137）" 2
run pre_wrk_paused FAKE_WRK0=paused
expect_untouched "worker 是 paused" 2
run pre_srv_down FAKE_SRV0=exited
expect_untouched "python-server 沒有在跑" 2

echo "== race 通過；收尾的順序；只用本次前綴"
run race_ok
if [ "$RC" -eq 0 ] && grep -q '全部通過' "$CASE/out"; then ok "race 通過 → rc 0"; else ng "race：rc $RC"; sed 's/^/      /' "$CASE/out" | tail -8; fi
p="$(sed -n 's/^PSQL_PREFIX //p' "$CASE/calls" | sort -u)"
[[ "$p" =~ ^i134smoke_[0-9]{14}_$ ]] && [ "$(wc -l <<<"$p")" -eq 1 ] && ok "psql 一律帶 -v prefix=$p" || ng "prefix 不對：$p"
del="$(grep "$DELETE" "$CASE/calls")"
if [ "$(grep -o "starts_with(job_id, :'prefix')" <<<"$del" | wc -l)" -eq 3 ] && ! grep -qi 'like' <<<"$del" \
   && ! grep -q 'i134smoke_' <<<"$del"; then
  ok "清理的 DELETE：三張表都是 starts_with(job_id, :'prefix')、⛔ 沒有 LIKE、前綴⛔ 不寫死在 SQL 裡"
else
  ng "清理的 DELETE 不對：$del"
fi
if ! grep '^SQL ' "$CASE/calls" | grep -v '^SQL precheck' | grep -qi ' like '; then ok "所有查詢都⛔ 沒有 LIKE"; else ng "有查詢用了 LIKE"; fi
grep '^SQL cleanup_fail_pending' "$CASE/calls" | grep -q "starts_with(job_id, :'prefix') AND status = 'pending'" \
  && ok "收尾先把本次的 pending 改成 failed（只限本次前綴）" || ng "收尾沒有先處理本次的 pending"
o_pend=$(first '^SQL cleanup_fail_pending'); o_stop=$(first "$STOP_SRV"); o_conf=$(first "$CONFIRM")
o_rel=$(last "$RELEASE"); o_del=$(first "$DELETE"); o_up=$(last "$UP_SRV")
if [ -n "$o_pend" ] && [ -n "$o_stop" ] && [ -n "$o_conf" ] && [ -n "$o_rel" ] && [ -n "$o_del" ] && [ -n "$o_up" ] \
   && [ "$o_pend" -lt "$o_stop" ] && [ "$o_stop" -lt "$o_conf" ] && [ "$o_conf" -lt "$o_rel" ] \
   && [ "$o_rel" -lt "$o_del" ] && [ "$o_del" -lt "$o_up" ]; then
  ok "收尾順序：pending→failed → 停止 → 確認停止 → 放鎖 → 刪資料 → 恢復服務"
else
  ng "收尾順序不對：pending=$o_pend stop=$o_stop confirm=$o_conf release=$o_rel delete=$o_del up=$o_up"
fi
[ "$(wrk_state)" = absent ] && called "$RM_WRK" && [ "$(srv_state)" = running ] \
  && ok "worker 原本不存在 → 移除本次建立的；python-server 起回來" || ng "worker absent 的恢復不對（wrk=$(wrk_state) srv=$(srv_state)）"

echo "== worker 的開始狀態"
run wrk_running FAKE_WRK0=running
if [ "$RC" -eq 0 ] && [ "$(wrk_state)" = running ] && ! called "$RM_WRK" && [ "$(last "$UP_WRK")" -gt "$(first "$DELETE")" ]; then
  ok "worker 原本在跑 → 刪資料之後起回來、⛔ 沒有移除"
else
  ng "worker running：rc $RC wrk=$(wrk_state) rm=$(count "$RM_WRK")"
fi
run wrk_stopped FAKE_WRK0=exited
if [ "$RC" -eq 0 ] && [ "$(wrk_state)" = exited ] && ! called "$RM_WRK" && ! called "$UP_WRK" && called '^docker start wrkold'; then
  ok "worker 原本存在但停止 → 用原本的 container 跑、最後維持停止、⛔ 沒有移除也⛔ 沒有重建"
else
  ng "worker stopped：rc $RC wrk=$(wrk_state) rm=$(count "$RM_WRK") up=$(count "$UP_WRK")"
fi
run wrk_stopped_old FAKE_WRK0=exited FAKE_WRKOLD_IMAGE=sha256:9999
if [ "$RC" -eq 1 ] && [ "$(wrk_state)" = exited ] && ! called "$RM_WRK" && ! called "$UP_WRK" && ! called '^docker start'; then
  ok "停止的 worker 不是本次的 image → rc 1、⛔ 不重建、⛔ 不移除、⛔ 不啟動"
else
  ng "worker stopped（舊 image）：rc $RC wrk=$(wrk_state)"
fi

echo "== race 沒有雙方嘗試的證據"
run race_no_overlap FAKE_RACE_NO_OVERLAP=1
[ "$RC" -eq 1 ] && grep -q '沒有覆蓋競爭' "$CASE/out" && ok "沒有任何一筆兩邊都嘗試過 → rc 1（⛔ 不算通過）" || ng "race 沒有證據卻是 rc $RC"

echo "== 收尾失敗與確認不通過"
run stop_fail FAKE_STOP_FAIL=1
if [ "$RC" -eq 1 ] && ! called "$DELETE" && [ "$(count "$UP_SRV")" -eq 1 ] && called "$RELEASE" && grep -q '手動清理' "$CASE/out"; then
  ok "停止失敗 → rc 1、⛔ 沒有刪資料、⛔ 沒有恢復服務、仍然放鎖、印出手動清理的 SQL"
else
  ng "停止失敗：rc $RC delete=$(count "$DELETE") up=$(count "$UP_SRV") release=$(count "$RELEASE")"
fi
run conn_left FAKE_CONN_LEFT=1
if [ "$RC" -eq 1 ] && ! called "$DELETE" && [ "$(count "$UP_SRV")" -eq 1 ] && called "$RELEASE"; then
  ok "停止後仍有來自它們的連線 → rc 1、⛔ 沒有刪資料、⛔ 沒有恢復服務、仍然放鎖"
else
  ng "連線沒清乾淨：rc $RC delete=$(count "$DELETE") up=$(count "$UP_SRV")"
fi
run stop_noop FAKE_STOP_NOOP=1
if [ "$RC" -eq 1 ] && ! called "$DELETE" && [ "$(count "$UP_SRV")" -eq 1 ]; then
  ok "stop 回傳 0 但 container 還在跑 → rc 1、⛔ 沒有刪資料、⛔ 沒有恢復服務"
else
  ng "stop 沒生效：rc $RC delete=$(count "$DELETE") up=$(count "$UP_SRV")"
fi
run delete_fail FAKE_DELETE_FAIL=1
[ "$RC" -eq 1 ] && [ "$(count "$UP_SRV")" -eq 2 ] && ok "刪資料失敗 → rc 1（收尾失敗⛔ 不回報成功），服務仍照紀錄恢復" || ng "刪資料失敗：rc $RC up=$(count "$UP_SRV")"
run lock_left SCENARIOS=race FAKE_LOCK_LEFT=1
[ "$RC" -eq 1 ] && grep -q '持鎖連線沒有結束' "$CASE/out" && ok "持鎖連線結束不了 → rc 1" || ng "持鎖連線殘留：rc $RC"

echo "== 中途失敗：暫停的 worker、持鎖連線、socket client"
run worker_restart_stuck SCENARIOS=worker-restart FAKE_NEVER_BLOCKED=1
o_unp=$(last 'compose .* unpause python-worker'); o_wstop=$(first 'compose .* stop python-worker')
if [ "$RC" -eq 1 ] && called 'compose .* pause python-worker' && [ -n "$o_unp" ] && [ -n "$o_wstop" ] \
   && [ "$o_unp" -lt "$o_wstop" ] && ! called STOP_WHILE_PAUSED && [ "$(wrk_state)" = absent ]; then
  ok "worker 暫停中失敗 → 收尾先 unpause 再 stop，最後照紀錄移除"
else
  ng "暫停的 worker：rc $RC unpause=$o_unp stop=$o_wstop wrk=$(wrk_state)"
fi
called "$RELEASE" && no_orphans 61 && ok "  持鎖連線被結束（⛔ 沒有留下持鎖的程序）" || ng "  持鎖連線沒有被結束"
run lost_stuck SCENARIOS=lost-response FAKE_NEVER_BLOCKED=1
if [ "$RC" -eq 1 ] && called python3-client && no_orphans 62 && no_orphans 61 && called "$RELEASE"; then
  ok "lost-response 卡在同步點 → rc 1、socket client 與持鎖連線都被結束"
else
  ng "lost-response 失敗的收尾：rc $RC client=$(count python3-client) orphan62=$(pgrep -fxc '/bin/sleep 62') orphan61=$(pgrep -fxc '/bin/sleep 61') release=$(count "$RELEASE")"
  sed 's/^/      /' "$CASE/out" | tail -6
fi

echo "== schema 情境"
run schema_ok SCENARIOS=schema
run_line="$(grep 'compose .* run ' "$CASE/calls" | head -1)"
o_rm=$(first '^docker rm -f i134smoke_[0-9]*_schema'); o_drop=$(first '^SQL tmpdb_drop ')
if [ "$RC" -eq 0 ] && [ -n "$o_rm" ] && [ -n "$o_drop" ] && [ "$o_rm" -lt "$o_drop" ] \
   && [[ "$run_line" == *"--no-deps"* ]] && [[ "$run_line" != *"--service-ports"* ]] && [[ "$run_line" != *" -p "* ]]; then
  ok "schema 通過 → 臨時 server ⛔ 不發布 port；先停臨時 server、再刪臨時 DB"
else
  ng "schema：rc $RC rm=$o_rm drop=$o_drop run=$run_line"
fi
grep '^SQL tmpdb_drop ' "$CASE/calls" | grep -q 'DROP DATABASE IF EXISTS :"tmpdb"' && ok "  DROP DATABASE 用 psql 變數" || ng "  DROP DATABASE 的寫法不對"
o_cov=$(first '^SQL tmpdb_add_cov '); o_col=$(first '^SQL tmpdb_add_col ')
if [ -n "$o_col" ] && [ -n "$o_cov" ] && [ "$o_col" -lt "$o_cov" ] \
   && grep '^SQL tmpdb_drop_col ' "$CASE/calls" | grep -qF 'DROP COLUMN data_coverage'; then
  ok "  臨時 DB 拿掉 077、078 兩個欄位；先補 077、確認仍在等，再補 078（I-137）"
else
  ng "  schema 的 077／078 順序不對：add_col=$o_col add_cov=$o_cov"
fi
run schema_ready_with_077 SCENARIOS=schema FAKE_READY_WITH_077=1
if [ "$RC" -eq 1 ] && grep -q '只有 077、還沒有 078 時臨時 server 就繼續啟動了' "$CASE/out" && called '^SQL tmpdb_drop '; then
  ok "只補 077 就就緒（探測漏了 078）→ rc 1、照常收尾（I-137）"
else
  ng "只補 077 就就緒沒被抓到：rc $RC"; sed 's/^/      /' "$CASE/out" | tail -4
fi
run schema_tmp_stays SCENARIOS=schema FAKE_TMP_NOWAIT=1 FAKE_TMP_STAYS=1
if [ "$RC" -eq 1 ] && ! called '^SQL tmpdb_drop ' && grep -q '不刪臨時 DB' "$CASE/out"; then
  ok "臨時 server 沒有確認停止 → rc 1、⛔ 不刪臨時 DB"
else
  ng "臨時 server 還在卻刪了 DB：rc $RC drop=$(count '^SQL tmpdb_drop ')"
fi

run schema_id_fail_stays SCENARIOS=schema FAKE_TMP_ID_FAIL=1 FAKE_TMP_STAYS=1
if [ "$RC" -eq 1 ] && called '^docker rm -f i134smoke_[0-9]*_schema' && ! called '^SQL tmpdb_drop '; then
  ok "起臨時 server 之後取不到 ID、server 又移不掉 → 仍依名稱移除、⛔ 不刪臨時 DB（review P2）"
else
  ng "取不到 ID：rc $RC rm=$(count '^docker rm -f i134smoke_') drop=$(count '^SQL tmpdb_drop ')"
fi
run schema_id_fail SCENARIOS=schema FAKE_TMP_ID_FAIL=1
if [ "$RC" -eq 1 ] && called '^docker rm -f i134smoke_[0-9]*_schema' && called '^SQL tmpdb_drop ' \
   && [ "$(cat "$CASE/state/tmp")" = absent ]; then
  ok "取不到 ID、但依名稱確認已移除 → 才刪臨時 DB"
else
  ng "取不到 ID（可移除）：rc $RC drop=$(count '^SQL tmpdb_drop ') tmp=$(cat "$CASE/state/tmp")"
fi
run schema_ps_fail SCENARIOS=schema FAKE_TMP_NOWAIT=1 FAKE_TMP_PS_FAIL=1
[ "$RC" -eq 1 ] && ! called '^SQL tmpdb_drop ' && ok "查不到臨時 server 的狀態 → ⛔ 不刪臨時 DB" || ng "查詢失敗卻刪了 DB：rc $RC"

echo "== trades 情境（I-135）"
run trades_ok SCENARIOS=trades SMOKE_STRATEGY=breakout_v1
if [ "$RC" -eq 0 ] && grep -q '3 筆交易明細' "$CASE/out" && called "$DELETE" \
   && grep '^SQL trades_check' "$CASE/calls" | grep -qF 'starts_with(t.job_id, $2)' \
   && grep '^SQL trades_check' "$CASE/calls" | grep -qF "EXECUTE trades_check(:'id', :'prefix');"; then
  ok "trades 通過 → rc 0、明細查詢只帶本次前綴（EXECUTE 綁 :'prefix'）、照常收尾"
else
  ng "trades：rc $RC"; sed 's/^/      /' "$CASE/out" | tail -5
fi
grep '^SQL trades_check' "$CASE/calls" | grep -q "CASE t.direction WHEN 'BUY' THEN 1 ELSE -1 END" \
  && ok "  精度界線帶方向（空方 −1）" || ng "  精度界線沒有帶方向"
# 送出的查詢就是共用檔案本身（PREPARE … AS <檔案內容>;），⛔ 不是腳本裡另一份
SHARED_SQL="$REPO_ROOT/backend/internal/database/testdata/backtest_trades_check.sql"
WANT="PREPARE trades_check(text, text) AS $(tr '\n' ' ' <"$SHARED_SQL");"
grep '^SQL trades_check' "$CASE/calls" | grep -qF -- "$WANT" \
  && ok "  明細查詢是共用檔案的內容（包進 PREPARE）" || ng "  明細查詢跟共用檔案不一致"
grep '^SQL trades_check' "$CASE/calls" | grep -qF 'IS NOT TRUE' \
  && ok "  壞列用 IS NOT TRUE 判（NULL 也算壞）" || ng "  明細查詢沒有用 IS NOT TRUE"
run trades_failed SCENARIOS=trades SMOKE_STRATEGY=breakout_v1 FAKE_TRADES_FAILED=1
[ "$RC" -eq 1 ] && called "$DELETE" && grep -q '預期 done' "$CASE/out" && ok "任務 failed → rc 1、照常收尾" || ng "任務 failed：rc $RC"
run trades_bad SCENARIOS=trades SMOKE_STRATEGY=breakout_v1 FAKE_TRADES_BAD=2
[ "$RC" -eq 1 ] && called "$DELETE" && grep -q '2 筆明細不符合規格' "$CASE/out" && ok "明細不符合規格 → rc 1、照常收尾" || ng "明細不符合：rc $RC"
run trades_check_fail SCENARIOS=trades SMOKE_STRATEGY=breakout_v1 FAKE_TRADES_CHECK_FAIL=1
[ "$RC" -eq 1 ] && called "$DELETE" && grep -q '查不到 .* 的交易明細' "$CASE/out" && ok "明細查詢失敗 → rc 1、照常收尾" || ng "明細查詢失敗：rc $RC"
# 費用核對（I-136）：送出的是共用檔的內容、費率是 container 裡 config 的原始值、在寫入任務之前讀
run trades_fee_ok SCENARIOS=trades SMOKE_STRATEGY=breakout_v1 FAKE_RATES="0.002 0.004"
FEE_SQL="$REPO_ROOT/backend/internal/database/testdata/backtest_trades_fee_check.sql"
WANT="PREPARE fee_check(text, text, numeric, numeric) AS $(tr '\n' ' ' <"$FEE_SQL");"
if [ "$RC" -eq 0 ] && grep '^SQL trades_fee_check' "$CASE/calls" | grep -qF -- "$WANT" \
   && grep '^SQL trades_fee_check' "$CASE/calls" | grep -qF "EXECUTE fee_check(:'id', :'prefix', :'comm_rate', :'tax_rate');" \
   && grep -qF -- "-v comm_rate=0.002 -v tax_rate=0.004" "$CASE/calls"; then
  ok "費用核對：共用檔的內容（包進 PREPARE）、費率用 container 讀到的值（0.002／0.004）"
else
  ng "費用核對：rc $RC"; sed 's/^/      /' "$CASE/out" | tail -5
fi
[ "$(count '^RATES_READ ')" -eq 2 ] && [ "$(first '^RATES_READ 2')" -lt "$(first '^SQL insert_jobs')" ] \
  && ok "  費率讀兩次（開始前、換上本次的 image 之後），第二次在寫入任務之前" || ng "  費率讀取的次數或順序不對"
run trades_fee_bad SCENARIOS=trades SMOKE_STRATEGY=breakout_v1 FAKE_FEE_BAD=1
[ "$RC" -eq 1 ] && called "$DELETE" && grep -q '1 筆明細的 commission 跟 config 費率' "$CASE/out" && ok "費用不符 → rc 1、照常收尾" || ng "費用不符：rc $RC"
run trades_fee_check_fail SCENARIOS=trades SMOKE_STRATEGY=breakout_v1 FAKE_FEE_CHECK_FAIL=1
[ "$RC" -eq 1 ] && called "$DELETE" && grep -q '查不到 .* 的費用核對' "$CASE/out" && ok "費用查詢失敗 → rc 1、照常收尾" || ng "費用查詢失敗：rc $RC"
run trades_rates_fail SCENARIOS=trades SMOKE_STRATEGY=breakout_v1 FAKE_RATES_FAIL=1
expect_untouched "開始前讀不到 config 費率" 2
for bad in "nan 0.003" "0.001425 inf" "-0.001 0.003" "0.001425"; do
  run trades_rates_invalid SCENARIOS=trades SMOKE_STRATEGY=breakout_v1 FAKE_RATES="$bad"
  expect_untouched "費率不是有限的非負數（$bad）" 2
done
run trades_rates_fail_after SCENARIOS=trades SMOKE_STRATEGY=breakout_v1 FAKE_RATES_FAIL_AFTER=1
[ "$RC" -eq 1 ] && called "$DELETE" && ! called '^SQL insert_jobs' && ok "換上本次的 image 之後讀不到費率 → rc 1、⛔ 沒寫入任務、照常收尾" || ng "之後讀不到費率：rc $RC"
run trades_wrong_strategy SCENARIOS=trades
expect_untouched "trades 沒搭配 breakout_v1" 2

echo "== trades 的覆蓋範圍（I-137）"
run trades_cov_ok SCENARIOS=trades SMOKE_STRATEGY=breakout_v1
if [ "$RC" -eq 0 ] && grep -q '覆蓋範圍：2024-01-02～2025-12-31，487 根（等於 DB 的根數），暖身 250 根' "$CASE/out" \
   && grep '^SQL trades_coverage' "$CASE/calls" | grep -qF "AT TIME ZONE 'Asia/Taipei'"; then
  ok "覆蓋範圍的根數等於 DB 裡這段台北日期的根數 → 通過；DB 的根數用台北日期算"
else
  ng "覆蓋範圍：rc $RC"; sed 's/^/      /' "$CASE/out" | tail -4
fi
run trades_cov_truncated SCENARIOS=trades SMOKE_STRATEGY=breakout_v1 "FAKE_COVERAGE=200|2025-03-01|2025-12-31|250|false|487"
[ "$RC" -eq 1 ] && called "$DELETE" && grep -q '範圍內根數 200 不等於 DB 裡' "$CASE/out" && ok "範圍內根數少於 DB（被截斷）→ rc 1、照常收尾" || ng "截斷沒被抓到：rc $RC"
run trades_cov_null SCENARIOS=trades SMOKE_STRATEGY=breakout_v1 "FAKE_COVERAGE=|||||487"
[ "$RC" -eq 1 ] && called "$DELETE" && grep -q '的結果沒有 data_coverage' "$CASE/out" && ok "結果沒有 data_coverage → rc 1、照常收尾" || ng "沒有 data_coverage：rc $RC"
run trades_cov_out_of_range SCENARIOS=trades SMOKE_STRATEGY=breakout_v1 "FAKE_COVERAGE=487|2023-12-29|2025-12-31|250|false|487"
[ "$RC" -eq 1 ] && grep -q '超出 2024-01-01～2025-12-31' "$CASE/out" && ok "首尾日期超出請求的範圍 → rc 1" || ng "首尾超出範圍：rc $RC"
run trades_cov_fail SCENARIOS=trades SMOKE_STRATEGY=breakout_v1 FAKE_COVERAGE_FAIL=1
[ "$RC" -eq 1 ] && called "$DELETE" && grep -q '查不到 .* 的覆蓋範圍' "$CASE/out" && ok "覆蓋範圍查詢失敗 → rc 1、照常收尾" || ng "覆蓋範圍查詢失敗：rc $RC"

echo "== 收尾的時限"
run race_base
BASE=$ELAPSED
run cleanup_budget FAKE_HANG_AFTER_CLEANUP=1 FAKE_IGNORE_TERM=1 CLEANUP_TIMEOUT=3
if [ "$RC" -eq 1 ] && [ "$ELAPSED" -le $((BASE + 3 + 1)) ] && grep -q '收尾時限 3 秒用完，未執行' "$CASE/out" && no_orphans 31 \
   && ! called "$DELETE"; then
  ok "收尾期間 docker 一直卡住 → ${ELAPSED} 秒結束（正常 ${BASE} 秒＋CLEANUP_TIMEOUT 3）、列出沒做的項目、⛔ 沒有刪資料（review P2）"
else
  ng "收尾超時：rc $RC、${ELAPSED} 秒（正常 ${BASE}）、未執行訊息 $(grep -c '未執行' "$CASE/out")"
fi

echo "== 卡住又忽略 SIGTERM"
run hang_logs FAKE_HANG=logs FAKE_IGNORE_TERM=1
[ "$RC" -eq 1 ] && no_orphans 31 && [ "$ELAPSED" -le 40 ] && ok "讀 log 卡住 → rc 1、${ELAPSED} 秒結束、卡住的程序被 -k 砍掉" \
  || ng "讀 log 卡住：rc $RC、${ELAPSED} 秒、orphan=$(pgrep -fxc '/bin/sleep 31')"
run hang_stop FAKE_HANG=stop FAKE_IGNORE_TERM=1
if [ "$RC" -eq 1 ] && no_orphans 31 && ! called "$DELETE" && [ "$(count "$UP_SRV")" -eq 1 ]; then
  ok "收尾的 stop 卡住 → rc 1、⛔ 沒有刪資料、⛔ 沒有恢復服務、卡住的程序被砍掉（${ELAPSED} 秒）"
else
  ng "收尾的 stop 卡住：rc $RC delete=$(count "$DELETE") up=$(count "$UP_SRV")"
fi
run hang_precheck "FAKE_HANG=q:precheck" FAKE_IGNORE_TERM=1
[ "$RC" -eq 2 ] && no_orphans 31 && ! grep -q "$CHANGING" "$CASE/calls" && ok "開始前的查詢卡住 → rc 2、⛔ 沒有改任何東西" || ng "precheck 卡住：rc $RC"

echo
echo "通過 $PASS、失敗 $FAIL"
[ "$FAIL" -eq 0 ]
