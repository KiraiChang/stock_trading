#!/usr/bin/env bash
# scripts/smoke-dev-worker.sh 的測試（T-083 計畫書 3.5）。⛔ 不需要真的 docker：用假的 docker 記錄呼叫，
# 驗證每一種失敗的停止點、等待上限，以及「從啟動 worker 起，失敗也一定收尾、只收本次的 worker」。
#
# 用法：scripts/test-smoke-dev-worker.sh；結束碼 0 全部通過、1 有失敗。
# 單引號裡的 $ 是刻意的（要原樣寫進假腳本），ok／ng 不會失敗
# shellcheck disable=SC2015,SC2016
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO_ROOT/scripts/smoke-dev-worker.sh"
WORK="$(mktemp -d "${TMPDIR:-/tmp}/test-smoke-dev-worker.XXXXXX")"
trap 'rm -rf "$WORK"' EXIT

PASS=0
FAIL=0
ok() { PASS=$((PASS + 1)); echo "  ok   $*"; }
ng() { FAIL=$((FAIL + 1)); echo "  FAIL $*"; }

FAKEBIN="$WORK/bin"
mkdir -p "$FAKEBIN"
cat > "$FAKEBIN/docker" <<'EOF'
#!/bin/bash
# 假 docker：照 smoke-dev-worker.sh 的呼叫形式回應；行為由 FAKE_* 控制，每次呼叫記一行到 $CALL_LOG
echo "docker $*" >> "$CALL_LOG"
# FAKE_HANG=<子指令>：那個子指令卡 30 秒（模擬沒有回應的 daemon）
# FAKE_IGNORE_TERM=1：卡住的那個還會忽略 SIGTERM（exec 之後 sleep 也繼承），只有 SIGKILL 停得了；
#   改睡 31 秒，測試才能確認它有被砍掉、⛔ 沒有留成孤兒
hang() {
  [ "${FAKE_HANG:-}" = "$1" ] || return 0
  if [ "${FAKE_IGNORE_TERM:-0}" = 1 ]; then trap '' TERM; exec /bin/sleep 31; fi
  exec /bin/sleep 30
}
GOOD=sha256:1111111111111111111111111111111111111111111111111111111111111111
case "$1" in
  image)
    hang image; [ "${FAKE_NO_IMAGE:-0}" = 1 ] && exit 1
    # postgres 的 image 給另一個 ID：取錯 image 時，啟動後的 .Image 比對就對不上
    case "$*" in *postgres*) echo sha256:3333333333333333333333333333333333333333333333333333333333333333 ;; *) echo "$GOOD" ;; esac
    exit 0 ;;
  run)
    [ "${FAKE_PIPCHECK_FAIL:-0}" = 1 ] && exit 1
    echo "${FAKE_MD5:-0123456789abcdef0123456789abcdef}"; exit 0 ;;
  inspect)
    case "$*" in *State*) hang inspect ;; *) hang inspectimg ;; esac
    case "$*" in
      *'{{.Image}}'*) echo "${FAKE_USED_IMAGE:-$GOOD}" ;;
      *'{{.State.Status}}'*) echo "running ${FAKE_RESTARTS:-0}" ;;
    esac
    exit 0 ;;
  compose) ;;
  *) echo "fake docker: 沒預期的呼叫：$*" >&2; exit 97 ;;
esac
shift
while [ $# -gt 0 ]; do
  case "$1" in -f|--profile) shift 2 ;; *) break ;; esac
done
hang "$1"
case "$1" in
  ps)
    if [ "${2:-}" = -a ]; then
      if [ -f "$STATE/up" ]; then
        # 收尾之後的確認：rm 成功就查不到（FAKE_RM_LEAVES=1 時仍查得到）
        if [ ! -f "$STATE/rm" ] || [ "${FAKE_RM_LEAVES:-0}" = 1 ]; then echo "cid0000000000001"; fi
      else
        [ "${FAKE_EXISTING:-0}" = 1 ] && echo "abcdef1234567890"
      fi
    elif [ -f "$STATE/up" ]; then
      echo "cid0000000000001"
    fi
    exit 0 ;;
  up)
    case " $* " in
      *" postgres "*) exit "${FAKE_PG_RC:-0}" ;;
      *" --no-deps python-worker "*) touch "$STATE/up"; exit 0 ;;
    esac
    exit 98 ;;
  exec)
    [ "${FAKE_PSQL_FAIL:-0}" = 1 ] && exit 1
    echo "${FAKE_PENDING:-0}"; exit 0 ;;
  build) exit "${FAKE_BUILD_RC:-0}" ;;
  config)
    if [ "${2:-}" = --images ]; then
      # 真的 compose 會連 depends_on 的 postgres 一起列、順序不固定：預設把 postgres 排在前面
      case "${FAKE_IMAGES:-depsfirst}" in
        depsfirst) printf 'postgres:16-alpine\nstock_trading_dev-python-worker\n' ;;
        svcfirst) printf 'stock_trading_dev-python-worker\npostgres:16-alpine\n' ;;
        nosvc) printf 'postgres:16-alpine\n' ;;
      esac
      exit 0
    fi
    printf 'name: %s\nservices:\n  postgres:\n    image: x\n' "${FAKE_PROJECT-stock_trading_dev}"
    exit 0 ;;
  logs)
    n=$(($(cat "$STATE/logs" 2>/dev/null || echo 0) + 1))
    echo "$n" > "$STATE/logs"
    echo "python-worker-1  | INFO worker started — poll_interval=10s"
    if [ "${FAKE_POLL_NEVER:-0}" = 0 ]; then
      # 第 n 次讀 log 時已經輪詢了 n+1 次：第 2 次讀就看得到 poll #3，正常的案例離 WAIT_SECONDS=3 有餘裕
      for i in $(seq "$((n + 1))"); do echo "python-worker-1  | INFO poll #$i: no pending jobs, sleeping 10s"; done
    fi
    [ "${FAKE_TRACEBACK:-0}" = 1 ] && echo "python-worker-1  | Traceback (most recent call last):"
    exit 0 ;;
  rm) [ "${FAKE_RM_RC:-0}" = 0 ] && touch "$STATE/rm"; exit "${FAKE_RM_RC:-0}" ;;
  *) echo "fake docker: 沒預期的 compose 子指令：$*" >&2; exit 97 ;;
esac
EOF
chmod +x "$FAKEBIN/docker"

# run CASE [ENV=VAL…]：在乾淨的環境下跑（WAIT_SECONDS 縮短、檢查間隔 1 秒）
run() {
  CASE="$WORK/$1"; shift
  rm -rf "$CASE"; mkdir -p "$CASE/state"; : > "$CASE/calls"
  local t0=$SECONDS
  env -i PATH="$FAKEBIN:/usr/bin:/bin" HOME="$WORK" CALL_LOG="$CASE/calls" STATE="$CASE/state" \
    WAIT_SECONDS=3 CHECK_INTERVAL=1 CALL_TIMEOUT=2 CLEANUP_TIMEOUT=2 KILL_AFTER=1 "$@" bash "$SCRIPT" > "$CASE/out" 2>&1
  RC=$?
  ELAPSED=$((SECONDS - t0))
}
called() { grep -q -- "$1" "$CASE/calls"; }
UP='compose .* up -d --no-build --no-deps python-worker'
RM='compose .* rm -sf python-worker'

# 失敗而且有收尾：rc 1、有 up、有 rm（只針對 python-worker）、⛔ 沒有 down／stop
expect_fail_cleaned() {
  local name="$1"
  if [ "$RC" -eq 1 ] && called "$UP" && called "$RM" && ! called ' down' && ! called ' stop' \
     && [ "$(grep -c ' rm ' "$CASE/calls")" -eq 1 ]; then
    ok "$name → rc 1、有收尾（只移除 python-worker）"
  else
    ng "$name：rc $RC、up=$(grep -c "$UP" "$CASE/calls") rm=$(grep -c "$RM" "$CASE/calls")"
  fi
}
# 在啟動之前就停下：⛔ 沒有 up、⛔ 沒有 rm
expect_stop_before_up() {
  local name="$1" want="$2"
  if [ "$RC" -eq "$want" ] && ! called "$UP" && ! called ' rm '; then
    ok "$name → rc $RC、⛔ 沒有啟動也沒有移除"
  else
    ng "$name：rc $RC（預期 $want）、up=$(grep -c "$UP" "$CASE/calls") rm=$(grep -c ' rm ' "$CASE/calls")"
  fi
}

echo "== smoke-dev-worker.sh（假 docker）"
run normal
order="$(grep -oE 'compose .*(config$|ps -a -q|up -d --wait|exec -T postgres|build python-worker|config --images|up -d --no-build|logs --no-color python-worker|rm -sf)' "$CASE/calls" \
  | sed -E 's/^compose -f [^ ]+ --profile worker //' | uniq | tr '\n' '|')"
want="config|ps -a -q|up -d --wait|exec -T postgres|build python-worker|config --images|up -d --no-build|logs --no-color python-worker|rm -sf|ps -a -q|"
if [ "$RC" -eq 0 ] && [ "$order" = "$want" ] && grep -q '通過' "$CASE/out" && ! called '^docker run' \
   && grep -q 'image：stock_trading_dev-python-worker' "$CASE/out"; then
  ok "正常（config --images 把 postgres 列在前面）→ rc 0、取到 worker 的 image；順序 project → ps → postgres → pending → build → image → up → logs → 收尾 → 確認已移除；沒設 md5 就⛔ 不跑 docker run"
else
  ng "正常：rc $RC、順序 $order"
fi
run md5ok EXPECT_FREEZE_MD5=0123456789abcdef0123456789abcdef
if [ "$RC" -eq 0 ] && called '^docker run --rm --network none --entrypoint sh'; then ok "md5 相符 → 通過（檢查用 --network none、入口換成 sh）"; else ng "md5 相符：rc $RC"; fi
run svcfirst FAKE_IMAGES=svcfirst
[ "$RC" -eq 0 ] && grep -q 'image：stock_trading_dev-python-worker' "$CASE/out" \
  && ok "config --images 的順序不同（worker 在前）也取到 worker 的 image" || ng "worker 在前：rc $RC"
run nosvcimg FAKE_IMAGES=nosvc; expect_stop_before_up "config --images 沒有 worker 的 image（只有 postgres）" 1
run nobuild NO_BUILD=1
if [ "$RC" -eq 0 ] && ! called ' build '; then ok "NO_BUILD=1 → ⛔ 不 build"; else ng "NO_BUILD=1：rc $RC"; fi

run existing FAKE_EXISTING=1; expect_stop_before_up "開始前已經有 python-worker" 2
run pending FAKE_PENDING=2;   expect_stop_before_up "dev DB 有 pending 的 backtest_jobs" 2
run psqlfail FAKE_PSQL_FAIL=1; expect_stop_before_up "查不到 backtest_jobs" 2
run pgfail FAKE_PG_RC=1;      expect_stop_before_up "postgres 沒有 healthy" 1
run buildfail FAKE_BUILD_RC=1; expect_stop_before_up "build 失敗" 1
run noimage FAKE_NO_IMAGE=1;  expect_stop_before_up "找不到 image" 1
run md5bad EXPECT_FREEZE_MD5=ffffffffffffffffffffffffffffffff; expect_stop_before_up "pip freeze 的 md5 不符" 1
run pipcheck EXPECT_FREEZE_MD5=0123456789abcdef0123456789abcdef FAKE_PIPCHECK_FAIL=1; expect_stop_before_up "pip check 沒通過" 1

run wrongimage FAKE_USED_IMAGE=sha256:2222222222222222222222222222222222222222222222222222222222222222
expect_fail_cleaned "container 用的不是本次的 image"
run restarted FAKE_RESTARTS=1; expect_fail_cleaned "RestartCount 不是 0"
run traceback FAKE_TRACEBACK=1; expect_fail_cleaned "log 有 Traceback"
run pollnever FAKE_POLL_NEVER=1
expect_fail_cleaned "等不到 poll #3"
if [ "$ELAPSED" -le 6 ]; then ok "等不到 poll #3：${ELAPSED} 秒結束（上限 3 秒＋間隔）"; else ng "等不到 poll #3：${ELAPSED} 秒才結束"; fi
if grep -q 'Traceback\|worker started' "$CASE/out"; then ok "失敗時印出 worker 的 log"; else ng "失敗時沒有印出 worker 的 log"; fi

# review 2026-11-16 ①：只操作 dev project
run otherproj FAKE_PROJECT=stock_trading
expect_stop_before_up "compose 解析出來的 project 是 stock_trading（live 的名稱）" 2
if ! called ' ps ' && grep -q 'stock_trading' "$CASE/out"; then ok "project 不對 → 連 ps 都⛔ 不跑"; else ng "project 不對：還跑了 ps"; fi
run noproj FAKE_PROJECT=
expect_stop_before_up "compose 解析不出 project 名稱" 2
run cfghang FAKE_HANG=config
expect_stop_before_up "查 project 時 docker 卡住（${ELAPSED} 秒）" 2
[ "$ELAPSED" -le 4 ] && ok "查 project 卡住：${ELAPSED} 秒結束" || ng "查 project 卡住：${ELAPSED} 秒才結束"
if COMPOSE_FILE=/elsewhere.yml run envfile && [ "$RC" -eq 0 ] && grep -q -- '-f docker-compose.dev.yml' "$CASE/calls" \
   && ! grep -q 'elsewhere' "$CASE/calls"; then
  ok "COMPOSE_FILE 環境變數⛔ 不能換掉 dev 的 compose 檔"
else
  ng "COMPOSE_FILE 環境變數：rc $RC"
fi

# review ②：收尾失敗 → 非 0（⛔ 不能回報成功卻留下 worker）
run rmfail FAKE_RM_RC=1
if [ "$RC" -eq 1 ] && grep -q '移除 python-worker 失敗' "$CASE/out" && grep -q '通過' "$CASE/out"; then
  ok "驗收通過但移除失敗 → rc 1"
else
  ng "移除失敗：rc $RC"
fi
run rmleaves FAKE_RM_LEAVES=1
if [ "$RC" -eq 1 ] && grep -q '移除之後仍查得到' "$CASE/out"; then ok "移除之後仍查得到 → rc 1"; else ng "移除之後仍查得到：rc $RC"; fi
run rmhang FAKE_HANG=rm
if [ "$RC" -eq 1 ] && [ "$ELAPSED" -le 4 ]; then ok "收尾卡住 → ${ELAPSED} 秒逾時、rc 1"; else ng "收尾卡住：rc $RC、${ELAPSED} 秒"; fi
run rmfail2 FAKE_RM_RC=1 FAKE_TRACEBACK=1
[ "$RC" -eq 1 ] && ok "驗收失敗又移除失敗 → 仍是 rc 1（保留原本的失敗）" || ng "驗收失敗又移除失敗：rc $RC"

# review ③：等待期間 docker 卡住也要在上限內結束、而且收尾
for h in inspect logs; do
  run "hang$h" FAKE_HANG="$h" WAIT_SECONDS=1 CALL_TIMEOUT=30
  if [ "$RC" -eq 1 ] && [ "$ELAPSED" -le 4 ] && called "$RM"; then
    ok "等待期間 docker $h 卡住（WAIT_SECONDS=1）→ ${ELAPSED} 秒結束、rc 1、有收尾"
  else
    ng "等待期間 docker $h 卡住：rc $RC、${ELAPSED} 秒、rm=$(grep -c "$RM" "$CASE/calls")"
  fi
done
run hangimg FAKE_HANG=inspectimg
if [ "$RC" -eq 1 ] && [ "$ELAPSED" -le 6 ] && called "$RM"; then
  ok "啟動之後確認 image 時 docker 卡住 → ${ELAPSED} 秒結束（CALL_TIMEOUT＋收尾）、rc 1、有收尾"
else
  ng "確認 image 時卡住：rc $RC、${ELAPSED} 秒、rm=$(grep -c "$RM" "$CASE/calls")"
fi
for h in ps exec image; do
  run "pre$h" FAKE_HANG="$h"
  if [ "$RC" -ne 0 ] && [ "$ELAPSED" -le 4 ] && ! called "$UP"; then
    ok "啟動之前 docker $h 卡住 → ${ELAPSED} 秒結束、⛔ 沒有啟動"
  else
    ng "啟動之前 docker $h 卡住：rc $RC、${ELAPSED} 秒"
  fi
done

# review 2026-11-18：卡住的 docker 忽略 SIGTERM → 要靠 timeout -k 的 SIGKILL 結束，仍在上限內並進入收尾
orphans() { pgrep -fx '/bin/sleep 31' | wc -l; }
for h in inspect logs; do
  run "ign$h" FAKE_HANG="$h" FAKE_IGNORE_TERM=1 WAIT_SECONDS=1 CALL_TIMEOUT=1
  # 上限：WAIT_SECONDS 1＋KILL_AFTER 1＋收尾
  if [ "$RC" -eq 1 ] && [ "$ELAPSED" -le 5 ] && called "$RM" && [ "$(orphans)" -eq 0 ]; then
    ok "等待期間 docker $h 卡住又忽略 SIGTERM → ${ELAPSED} 秒結束、rc 1、有收尾、⛔ 沒有留下卡住的程序"
  else
    ng "等待期間 docker $h 忽略 SIGTERM：rc $RC、${ELAPSED} 秒、rm=$(grep -c "$RM" "$CASE/calls")、殘留 $(orphans)"
  fi
done
run ignrm FAKE_HANG=rm FAKE_IGNORE_TERM=1
# 上限：正常流程約 2 秒＋CLEANUP_TIMEOUT 2＋KILL_AFTER 1
if [ "$RC" -eq 1 ] && [ "$ELAPSED" -le 6 ] && grep -q '移除 python-worker 失敗' "$CASE/out" && [ "$(orphans)" -eq 0 ]; then
  ok "收尾卡住又忽略 SIGTERM → ${ELAPSED} 秒結束、rc 1、⛔ 沒有留下卡住的程序"
else
  ng "收尾忽略 SIGTERM：rc $RC、${ELAPSED} 秒、殘留 $(orphans)"
fi
run ignconfig FAKE_HANG=config FAKE_IGNORE_TERM=1
if [ "$RC" -eq 2 ] && [ "$ELAPSED" -le 5 ] && ! called "$UP" && [ "$(orphans)" -eq 0 ]; then
  ok "查 project 時卡住又忽略 SIGTERM → ${ELAPSED} 秒結束、rc 2、⛔ 沒有啟動"
else
  ng "查 project 忽略 SIGTERM：rc $RC、${ELAPSED} 秒、殘留 $(orphans)"
fi

# 中途被砍（Ctrl+C／TERM）也要收尾
CASE="$WORK/term"; rm -rf "$CASE"; mkdir -p "$CASE/state"; : > "$CASE/calls"
env -i PATH="$FAKEBIN:/usr/bin:/bin" HOME="$WORK" CALL_LOG="$CASE/calls" STATE="$CASE/state" \
  WAIT_SECONDS=60 CHECK_INTERVAL=1 CALL_TIMEOUT=2 CLEANUP_TIMEOUT=2 KILL_AFTER=1 FAKE_POLL_NEVER=1 bash "$SCRIPT" > "$CASE/out" 2>&1 &
pid=$!
for _ in $(seq 50); do called "$UP" && break; /bin/sleep 0.1; done
/bin/sleep 1
kill -TERM "$pid"
wait "$pid"; RC=$?
if [ "$RC" -eq 143 ] && called "$RM"; then ok "等待中被 TERM → rc 143、有收尾"; else ng "被 TERM：rc $RC、rm=$(grep -c "$RM" "$CASE/calls")"; fi

# 參數不對：任何 docker 呼叫之前就擋下
run projname COMPOSE_PROJECT_NAME=other
if [ "$RC" -eq 2 ] && [ ! -s "$CASE/calls" ]; then ok "COMPOSE_PROJECT_NAME 有設定 → rc 2、docker 呼叫 0 次"; else ng "COMPOSE_PROJECT_NAME：rc $RC"; fi
run projempty COMPOSE_PROJECT_NAME=
if [ "$RC" -eq 2 ] && [ ! -s "$CASE/calls" ]; then ok "COMPOSE_PROJECT_NAME 是空字串也擋"; else ng "COMPOSE_PROJECT_NAME 空字串：rc $RC"; fi
for bad in "POLLS=0" "POLLS=x" "WAIT_SECONDS=-1" "NO_BUILD=2" "EXPECT_FREEZE_MD5=abc" "CALL_TIMEOUT=0" "CLEANUP_TIMEOUT=x" "KILL_AFTER=0"; do
  run badarg "$bad"
  if [ "$RC" -eq 2 ] && [ ! -s "$CASE/calls" ]; then ok "$bad → rc 2、docker 呼叫 0 次"; else ng "$bad：rc $RC"; fi
done

echo "==> test-smoke-dev-worker：通過 $PASS、失敗 $FAIL"
[ "$FAIL" -eq 0 ]
