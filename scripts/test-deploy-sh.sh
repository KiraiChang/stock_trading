#!/bin/bash
# deploy.sh／dc.sh 範本與 compose 範本的測試（I-132 的 A1～A3；規格見 docs/development-workflow.md
# 「部署腳本：失敗即停」與「live 的設定：env_file」）。
#
#   一、流程：用假的 git／docker／sleep 放在 PATH 前面，記錄呼叫順序。每一種失敗都要回傳非 0，
#      而且在該停的地方停下（down 之前的失敗⛔ 不能呼叫 down／up；down 失敗⛔ 不能 up）。
#   二、設定解析：用真的 `docker compose config`（⛔ 不啟動任何服務），驗證 env_file 的絕對路徑、
#      從不同目錄執行結果相同、dotenv 的特殊字元、鍵不重複、不漏鍵。
#   三、dc.sh：子指令的放行／擋下。
#
# 用法：scripts/test-deploy-sh.sh（需要 docker compose；第二部分不需要 image，也不連 daemon 以外的東西）
# 結束碼：0 全部通過、1 有失敗。
# ⛔ 輸出只有案例名稱、通過／失敗與鍵名；用到的值都是測試合成的假值。
set -uo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK="$(mktemp -d "${TMPDIR:-/tmp}/test-deploy-sh.XXXXXX")"
trap 'rm -rf "$WORK"' EXIT

PASS=0
FAIL=0
ok() { PASS=$((PASS + 1)); echo "  ok   $*"; }
ng() { FAIL=$((FAIL + 1)); echo "  FAIL $*"; }

FAKE_JWT_MARKER="FAKEJWT-9f3c-not-a-secret"

# ── 假的指令 ──────────────────────────────────────────────────
FAKEBIN="$WORK/fakebin"
mkdir -p "$FAKEBIN"

cat > "$FAKEBIN/git" <<'EOF'
#!/bin/bash
# 假 git：只認 deploy.sh 會用的形式；行為由 FAKE_* 控制，每次呼叫記一行到 $CALL_LOG
echo "git $*" >> "$CALL_LOG"
[ "${1:-}" = "-C" ] && shift 2
case "$*" in
  "rev-parse --is-inside-work-tree") [ "${FAKE_NOT_GIT:-0}" = 1 ] && exit 128; echo true ;;
  "rev-parse --abbrev-ref HEAD") echo "${FAKE_BRANCH:-main}" ;;
  "status --porcelain --untracked-files=no") [ "${FAKE_DIRTY:-0}" = 1 ] && echo " M backend/main.go"; exit 0 ;;
  "pull --ff-only origin "*) exit "${FAKE_PULL_RC:-0}" ;;
  *) echo "fake git: 沒預期的呼叫：$*" >&2; exit 97 ;;
esac
EOF

cat > "$FAKEBIN/docker" <<'EOF'
#!/bin/bash
# 假 docker：compose 的子指令照 deploy.sh 的形式回應；行為由 FAKE_* 控制，每次呼叫記一行到 $CALL_LOG
# 模擬回應緩慢或卡住：被 timeout 砍掉時連同 sleep 一起結束，⛔ 不留下孤兒程序
fake_delay() {
  [ "$1" = 0 ] && return 0
  if [ "$(trap -p TERM)" = "trap -- '' SIGTERM" ]; then
    # 忽略 TERM（模擬卡死的 daemon 呼叫）：sleep 也繼承忽略，只有 timeout -k 的 KILL 才停得下來
    /bin/sleep "$1" &
    wait $!
    return 0
  fi
  /bin/sleep "$1" &
  local p=$!
  trap 'kill "$p" 2>/dev/null; exit 143' TERM INT
  wait "$p"
  trap - TERM INT
}
# 回測檢查（T-084）的第 n 次呼叫：FAKE_BT<n>_X 優先，其次 FAKE_BT_X，再其次預設值
bt() {
  local v="FAKE_BT${BT_N}_$1" d="FAKE_BT_$1"
  if [ -n "${!v+x}" ]; then printf '%s' "${!v}"; elif [ -n "${!d+x}" ]; then printf '%s' "${!d}"; else printf '%s' "$2"; fi
}
bt_count() {
  BT_N=$(($(cat "$WORK_STATE/$1" 2>/dev/null || echo 0) + 1))
  echo "$BT_N" > "$WORK_STATE/$1"
}
if [ "${1:-}" = "inspect" ]; then
  echo "docker $*" >> "$CALL_LOG"
  if [ "${2:-}" = "-f" ] && [ "${3:-}" = '{{.State.Running}}' ]; then
    bt_count bt_inspect
    [ "$(bt IGNORE_TERM 0)" = 1 ] && trap '' TERM
    fake_delay "$(bt INSPECT_DELAY 0)"
    echo "FAKE-DOCKER-STDERR-7c1e" >&2
    [ "$(bt MISSING 0)" = 1 ] && exit 1
    bt RUNNING true
    echo
    exit 0
  fi
  echo "${FAKE_RESTARTS:-0}"
  exit 0
fi
# ── T-087：試跑、備份、正式 migrator、停止確認（狀態放在 $WORK_STATE）──
DEFAULT_STATUS='migrate: pending -\nmigrate: fingerprint aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\nmigrate: instance 7712040185090818081|trading|16384|20261125093441474452\n'
DEFAULT_INST='7712040185090818081|trading|16384|20261125093441474452'
st_get() { if [ -e "$WORK_STATE/$1" ]; then cat "$WORK_STATE/$1"; else printf '%s' "${2:-}"; fi; }
st_set() { printf '%s' "$2" > "$WORK_STATE/$1"; }
case "${1:-}" in
  ps)  # ps -a --filter name=^/名稱$ --format {{.State}}
    echo "docker $*" >> "$CALL_LOG"
    [ "${FAKE_PS_RC:-0}" = 0 ] || exit "$FAKE_PS_RC"
    name="${4#name=^/}"; name="${name%\$}"
    case "$name" in
      stock_trading-migrator-*) [ "${FAKE_PS_MIGRATOR_RC:-0}" = 0 ] || exit "$FAKE_PS_MIGRATOR_RC"; st_get migrator_state ""; echo ;;
      *) if [ -e "$WORK_STATE/c_$name" ]; then echo running; else echo; fi ;;
    esac
    exit 0 ;;
  rm)  # rm -fv 名稱…
    echo "docker $*" >> "$CALL_LOG"; shift 2
    if [ "${FAKE_TRIAL_LEAK:-0}" != 1 ]; then for c in "$@"; do rm -f "$WORK_STATE/c_$c"; done; fi
    exit 0 ;;
  network)
    echo "docker $*" >> "$CALL_LOG"
    case "$2" in
      create) [ "${FAKE_NET_CREATE_RC:-0}" = 0 ] || exit "$FAKE_NET_CREATE_RC"; : > "$WORK_STATE/n_$4" ;;
      rm) [ "${FAKE_TRIAL_LEAK:-0}" = 1 ] || rm -f "$WORK_STATE/n_$3" ;;
      ls) n="${5#name=^}"; n="${n%\$}"; if [ -e "$WORK_STATE/n_$n" ]; then echo netid123; fi ;;
    esac
    exit 0 ;;
  stop) echo "docker $*" >> "$CALL_LOG"; if [ "${FAKE_STOP_WORKS:-1}" = 1 ]; then st_set migrator_state exited; fi; exit 0 ;;
  kill) echo "docker $*" >> "$CALL_LOG"; if [ "${FAKE_KILL_WORKS:-1}" = 1 ]; then st_set migrator_state exited; fi; exit 0 ;;
  run)
    echo "docker $*" >> "$CALL_LOG"
    case " $* " in
      *" run -d --name "*)
        # 臨時密碼只經環境變數（-e POSTGRES_PASSWORD 不帶值）
        case " $* " in *" -e POSTGRES_PASSWORD "*) ;; *) echo "PW_NOT_FROM_ENV" >> "$CALL_LOG" ;; esac
        [ "${#POSTGRES_PASSWORD}" -ge 16 ] || echo "PW_MISSING" >> "$CALL_LOG"
        st_set pw "${POSTGRES_PASSWORD:-}"
        : > "$WORK_STATE/c_$4"
        [ "${FAKE_TRIALPG_RC:-0}" = 0 ] || exit "$FAKE_TRIALPG_RC"
        echo cid; exit 0 ;;
      *" -e PGPASSWORD "*)
        [ "${PGPASSWORD:-}" = "$(st_get pw)" ] || echo "MARKER_PW_DIFF" >> "$CALL_LOG"
        case " $* " in *"postgres://"*) echo "MARKER_DSN_IN_ARGV" >> "$CALL_LOG" ;; esac
        st_set dsn "postgres://${PGUSER:-}:${PGPASSWORD:-}@${PGHOST:-}:${PGPORT:-}/${PGDATABASE:-}?sslmode=${PGSSLMODE:-}"
        echo "${FAKE_MARKER-t}"; exit 0 ;;
      *" -migrate=status "*|*" -migrate=up "*)
        case " $* " in *" -e DATABASE_DSN "*) ;; *) echo "DSN_NOT_FROM_ENV" >> "$CALL_LOG" ;; esac
        [ "${DATABASE_DSN:-}" = "$(st_get dsn)" ] || echo "DSN_DIFF" >> "$CALL_LOG"
        case " $* " in
          *" -migrate=status "*) printf '%b' "${FAKE_TS_OUT-${FAKE_MS1_OUT-$DEFAULT_STATUS}}"; exit "${FAKE_TS_RC:-0}" ;;
        esac
        printf 'migrate: step version=81 end=x duration=1s result=ok\nmigrate: after pending %s\n' "${FAKE_TU_AFTER:--}"; exit "${FAKE_TU_RC:-0}" ;;
    esac
    echo "fake docker: 沒預期的 run：$*" >&2; exit 97 ;;
  exec)
    case " $* " in
      *" pg_isready "*) echo "docker $*" >> "$CALL_LOG"; exit "${FAKE_PGREADY_RC:-0}" ;;
      *" --schema-only "*) echo "docker $*" >> "$CALL_LOG"; echo "-- fake schema"; exit "${FAKE_DUMP_SCHEMA_RC:-0}" ;;
      *" --data-only "*) echo "docker $*" >> "$CALL_LOG"; echo "-- fake data"; exit "${FAKE_DUMP_DATA_RC:-0}" ;;
      *" -Fc "*)
        echo "docker $*" >> "$CALL_LOG"
        [ "${FAKE_BACKUP_RC:-0}" = 0 ] || exit "$FAKE_BACKUP_RC"
        # FAKE_BACKUP_PLANT_OLD：寫備份的同時，正式檔名出現同名的東西（開頭的「已經存在」檢查之後才出現）：
        #   1＝一般檔（內容 OLD）、dir＝目錄、symlink＝指向另一個目錄的符號連結
        if [ "${FAKE_BACKUP_PLANT_OLD:-0}" != 0 ]; then
          t="$(readlink /proc/$$/fd/1)"
          case "$t" in
            *.partial)
              case "$FAKE_BACKUP_PLANT_OLD" in
                1) printf 'OLD' > "${t%.partial}" ;;
                dir) mkdir "${t%.partial}" ;;
                symlink) mkdir -p "$WORK_STATE/elsewhere" && /usr/bin/ln -s "$WORK_STATE/elsewhere" "${t%.partial}" ;;
              esac ;;
          esac
        fi
        printf 'PGDMP-fake'; exit 0 ;;
      *" pg_restore -l "*) echo "docker $*" >> "$CALL_LOG"; cat >/dev/null; exit "${FAKE_RESTORE_L_RC:-0}" ;;
      *"CREATE TABLE t087_trial_target"*) echo "docker $*" >> "$CALL_LOG"; exit "${FAKE_MARKER_CREATE_RC:-0}" ;;
      *" -U trial -d trial "*) echo "docker $*" >> "$CALL_LOG"; cat >/dev/null; exit "${FAKE_RESTORE_RC:-0}" ;;
      *"pg_control_system"*)
        # 腳本用 DB_CONTAINER／DB_NAME 查的實例識別（第 n 次：FAKE_INST<n>）
        echo "docker $*" >> "$CALL_LOG"
        BT_N=$(($(st_get inst_n 0) + 1)); st_set inst_n "$BT_N"
        v="FAKE_INST${BT_N}"
        printf '%s\n' "${!v-${FAKE_INST-$DEFAULT_INST}}"; exit 0 ;;
      *"pg_stat_activity"*)
        echo "docker $*" >> "$CALL_LOG"
        case " $* " in
          *pg_cancel_backend*) if [ "${FAKE_CANCEL_WORKS:-1}" = 1 ]; then st_set sessions 0; fi; echo 1; exit 0 ;;
          *pg_terminate_backend*) if [ "${FAKE_TERMINATE_WORKS:-1}" = 1 ]; then st_set sessions 0; fi; echo 1; exit 0 ;;
        esac
        [ "${FAKE_SESSIONS_RC:-0}" = 0 ] || exit "$FAKE_SESSIONS_RC"
        st_get sessions "${FAKE_SESSIONS:-0}"; echo; exit 0 ;;
    esac ;;
esac
if [ "${1:-}" = "exec" ]; then
  # 只認 deploy.sh 的形式：exec -e PGOPTIONS=… 容器 psql -U 帳號 -d 資料庫 -X -v ON_ERROR_STOP=1 -qtAc SQL（⛔ 不帶 -i）
  echo "docker $*" >> "$CALL_LOG"
  [ "$(readlink /proc/$$/fd/0)" = /dev/null ] || echo "STDIN_NOT_DEVNULL exec" >> "$CALL_LOG"
  if [ "$#" -ne 14 ] || [ "$2" != "-e" ] || [ "${3%%=*}" != PGOPTIONS ] || [ "$5" != psql ] || [ "$6" != -U ] || [ "$8" != -d ] \
     || [ "${10}" != -X ] || [ "${11}" != -v ] || [ "${12}" != ON_ERROR_STOP=1 ] || [ "${13}" != -qtAc ] || [ -z "${14}" ]; then
    echo "fake docker: 沒預期的 exec：$*" >&2; exit 97
  fi
  printf '%s\n' "${14}" > "$WORK_STATE/bt_sql"
  bt_count bt_exec
  [ "$(bt IGNORE_TERM 0)" = 1 ] && trap '' TERM
  fake_delay "$(bt EXEC_DELAY 0)"
  echo "FAKE-PSQL-STDERR-7c1e" >&2
  rc="$(bt RC 0)"
  [ "$rc" = 0 ] || exit "$rc"
  printf '%b' "$(bt OUT '0|0\n')"
  exit 0
fi
[ "${1:-}" = "compose" ] || { echo "fake docker: 沒預期的呼叫：$*" >&2; exit 97; }
shift
# 全域選項：-f FILE --project-directory DIR
while [ $# -gt 0 ]; do
  case "$1" in
    -f|--project-directory) shift 2 ;;
    *) break ;;
  esac
done
echo "compose $*" >> "$CALL_LOG"
# run／exec 預設會讀 stdin：deploy.sh 必須把 stdin 換成 /dev/null（在終端機執行時，timeout 底下的 docker
# 是背景程序群組，讀終端機會被 SIGTTIN 停住）。FAKE_READ_STDIN=1 時真的讀一次，模擬 compose 的行為。
case "$1" in
  run|exec)
    [ "$(readlink /proc/$$/fd/0)" = /dev/null ] || echo "STDIN_NOT_DEVNULL $1" >> "$CALL_LOG"
    if [ "${FAKE_READ_STDIN:-0}" = 1 ]; then read -r -t 5 _ || true; fi ;;
esac
case "$1" in
  config)
    [ "${FAKE_CONFIG_RC:-0}" = 0 ] || exit "$FAKE_CONFIG_RC"
    if [ "${2:-}" = "--images" ]; then
      echo "stock_trading-backend"
      exit 0
    fi
    if [ "${2:-}" = "--services" ]; then
      printf 'backend\npython-server\npython-worker\n'
      exit 0
    fi
    printf 'name: stock_trading\nservices:\n  backend:\n    environment:\n'
    printf '      A_KEY: x\n'
    if [ "${FAKE_JWT_LINE+x}" ]; then
      printf '%s\n' "$FAKE_JWT_LINE"
    else
      printf '      AUTH_JWT_SECRET: %s\n' "$FAKE_JWT_MARKER"
    fi
    printf '      REDIS_ADDR: redis:6379\n    image: x\n  python-server:\n    environment:\n'
    printf '      AUTH_JWT_SECRET: change-me\n'
    ;;
  build)
    [ "${2:-}" = "${FAKE_BUILD_FAIL:-}" ] && exit 1
    exit 0 ;;
  run)
    case " $* " in
      # 先確認 image 認得旗標（grep 執行檔，⛔ 不執行它）
      *" --entrypoint grep backend -qF check-config ./trading-backend "*)
        fake_delay "${FAKE_PROBE_DELAY:-0}"
        [ "${FAKE_NO_FLAG:-0}" = 1 ] && exit 1
        exit "${FAKE_PROBE_RC:-0}" ;;
      # T-087：確認新 image 認得 -migrate（grep 執行檔）
      *" --entrypoint grep backend -qF migrate-statement-timeout ./trading-backend "*)
        [ "${FAKE_NO_MIGRATE_FLAG:-0}" = 1 ] && exit 1
        exit 0 ;;
      # T-087：正式套用（具名容器）。外層 timeout 砍掉 CLI 時（TERM），容器的狀態留在 running（模擬 CLI 退出、容器還在跑）
      *" --name stock_trading-migrator-"*" backend ./trading-backend -migrate=up "*)
        st_set migrator_state running
        printf 'migrate: step version=81 end=x duration=1s result=ok\n'
        fake_delay "${FAKE_MUP_DELAY:-0}"
        [ "${FAKE_MUP_LEAVE_RUNNING:-0}" = 1 ] || st_set migrator_state ""
        exit "${FAKE_MUP_RC:-0}" ;;
      # T-087：-migrate=status（第 n 次：FAKE_MS<n>_OUT／RC）
      *" backend ./trading-backend -migrate=status "*)
        BT_N=$(($(st_get ms_n 0) + 1)); st_set ms_n "$BT_N"
        v="FAKE_MS${BT_N}_OUT"; r="FAKE_MS${BT_N}_RC"
        printf '%b' "${!v-${FAKE_MS_OUT-$DEFAULT_STATUS}}"
        exit "${!r:-0}" ;;
      # 一定要寫出執行檔：只帶旗標的話，真的 container 會把旗標當成程式去執行
      *" backend ./trading-backend -check-config "*)
        fake_delay "${FAKE_CHECK_DELAY:-0}"
        exit "${FAKE_CHECK_RC:-0}" ;;
      *) echo "RUN_WITHOUT_EXECUTABLE" >> "$CALL_LOG"; exit 98 ;;
    esac ;;
  down) exit "${FAKE_DOWN_RC:-0}" ;;
  up)
    case " $* " in *" --build "*) echo "UP_WITH_BUILD" >> "$CALL_LOG" ;; esac
    exit "${FAKE_UP_RC:-0}" ;;
  ps)
    fake_delay "${FAKE_PS_DELAY:-0}"
    echo "cid123" ;;
  exec)
    # wget 要帶單次逾時（-T）
    case " $* " in *" wget -q -T "*) ;; *) echo "WGET_WITHOUT_TIMEOUT" >> "$CALL_LOG" ;; esac
    fake_delay "${FAKE_HEALTH_DELAY:-0}"
    n=$(($(cat "$WORK_STATE/health" 2>/dev/null || echo 0) + 1))
    echo "$n" > "$WORK_STATE/health"
    [ "$n" -ge "${FAKE_HEALTH_AFTER:-1}" ] && [ "${FAKE_HEALTH_NEVER:-0}" = 0 ] && exit 0
    exit 1 ;;
  *) echo "fake docker: 沒預期的 compose 子指令：$*" >&2; exit 97 ;;
esac
EOF

cat > "$FAKEBIN/timeout" <<'EOF'
#!/bin/bash
# 假 timeout：記下回測檢查的 docker inspect／exec 是怎麼包的，再交給真的 timeout（行為不變）；
# 每一次的完整參數另外記在 $WORK_STATE/argv（驗臨時密碼與 DSN ⛔ 不在程序參數裡）
printf '%s\n' "timeout $*" >> "$WORK_STATE/argv"
case " $* " in
  *" docker inspect -f {{.State.Running}} "*|*" docker exec "*) echo "TIMEOUT $*" >> "$CALL_LOG" ;;
esac
exec /usr/bin/timeout "$@"
EOF

cat > "$FAKEBIN/ln" <<'EOF'
#!/bin/bash
# 假 ln：FAKE_LN_FAIL=1 時連結失敗、⛔ 不動檔案（T-087 review 的備份反例）；其他照真的 ln
echo "ln $*" >> "$CALL_LOG"
[ "${FAKE_LN_FAIL:-0}" = 1 ] && exit 1
exec /usr/bin/ln "$@"
EOF

cat > "$FAKEBIN/sleep" <<'EOF'
#!/bin/bash
# 真的等（readiness 的上限是總截止時間，時間要真的經過）；測試把間隔設成 1 秒
echo "sleep $*" >> "$CALL_LOG"
exec /bin/sleep "$@"
EOF
chmod +x "$FAKEBIN"/*

# ── 每個案例的目錄 ────────────────────────────────────────────
# $CASE/scripts/{deploy.sh,compose.yml,.env}、$CASE/redis/.env、$CASE/meminfo、$CASE/calls
setup_case() {
  CASE="$WORK/case.$1"
  rm -rf "$CASE"
  mkdir -p "$CASE/scripts" "$CASE/redis" "$CASE/build" "$CASE/state"
  # readiness 的上限縮短（總上限 ${READY_T:-4} 秒、單次 ${READY_REQ:-2} 秒、間隔 1 秒），測試才不用等 90 秒
  sed -e "s#^PROJECT_DIR=\"/path/to/trading\"#PROJECT_DIR=\"$CASE/build\"#" \
      -e "s#^REDIS_ENV_FILE=\"/path/to/redis-stack/.env\"#REDIS_ENV_FILE=\"$CASE/redis/.env\"#" \
      -e "s#^READY_TIMEOUT_SEC=90\$#READY_TIMEOUT_SEC=${READY_T:-4}#" \
      -e "s#^READY_INTERVAL_SEC=2\$#READY_INTERVAL_SEC=1#" \
      -e "s#^READY_REQUEST_TIMEOUT_SEC=5\$#READY_REQUEST_TIMEOUT_SEC=${READY_REQ:-2}#" \
      -e "s#^CHECK_CONFIG_TIMEOUT_SEC=120\$#CHECK_CONFIG_TIMEOUT_SEC=2#" \
      -e "s#^DB_CONTAINER=\"your-postgres-container\"#DB_CONTAINER=\"${DB_C-fake-pg}\"#" \
      -e "s#^DB_USER=\"your-db-user\"#DB_USER=\"${DB_U-fake_user}\"#" \
      -e "s#^BACKTEST_CHECK_TIMEOUT_SEC=15\$#BACKTEST_CHECK_TIMEOUT_SEC=1#" \
      -e "s#^BACKUP_DIR=\"/path/to/db-backups\"#BACKUP_DIR=\"${BK_DIR-$CASE/backup}\"#" \
      -e "s#^STOP_WAIT_SEC=60\$#STOP_WAIT_SEC=2#" -e "s#^CANCEL_WAIT_SEC=30\$#CANCEL_WAIT_SEC=2#" -e "s#^STOP_POLL_SEC=5\$#STOP_POLL_SEC=1#" \
      -e "s#^MIGRATE_OUTER_TIMEOUT_SEC=960\$#MIGRATE_OUTER_TIMEOUT_SEC=${OUTER_T:-960}#" \
      "$REPO/deploy.sh" > "$CASE/scripts/deploy.sh"
  mkdir -m 700 "$CASE/backup"
  for k in READY_TIMEOUT_SEC READY_INTERVAL_SEC READY_REQUEST_TIMEOUT_SEC CHECK_CONFIG_TIMEOUT_SEC BACKTEST_CHECK_TIMEOUT_SEC \
           STOP_WAIT_SEC CANCEL_WAIT_SEC STOP_POLL_SEC MIGRATE_OUTER_TIMEOUT_SEC; do
    grep -q "^$k=[0-9]*\$" "$CASE/scripts/deploy.sh" || { echo "  FAIL （測試本身）deploy.sh 找不到 $k"; exit 1; }
  done
  chmod +x "$CASE/scripts/deploy.sh"
  sed -e "s#/path/to/scripts/stock_trading/.env#$CASE/scripts/.env#" \
      -e "s#/path/to/redis-stack/.env#$CASE/redis/.env#" \
      "$REPO/docker-compose.yml" > "$CASE/scripts/compose.yml"
  printf 'AUTH_JWT_SECRET=%s\n' "$FAKE_JWT_MARKER" > "$CASE/scripts/.env"
  chmod 600 "$CASE/scripts/.env"
  printf '# redis\nREDIS_PASSWORD=fake-redis-pw\n' > "$CASE/redis/.env"
  printf 'MemTotal:        8000000 kB\nMemAvailable:    4000000 kB\n' > "$CASE/meminfo"
  : > "$CASE/calls"
}

# run_deploy：在乾淨的環境變數下執行（FAKE_* 由呼叫端用 env 帶入）。
# 外層上限 30 秒（RUN_LIMIT）：deploy.sh 的某個逾時失效時（例如拿掉 timeout -k 的反證），案例判失敗、⛔ 不讓整支測試卡住
run_deploy() {
  local t0=$SECONDS
  /usr/bin/timeout -k 2 "${RUN_LIMIT:-30}" env -i PATH="$FAKEBIN:/usr/bin:/bin" HOME="$WORK" CALL_LOG="$CASE/calls" WORK_STATE="$CASE/state" \
    FAKE_JWT_MARKER="$FAKE_JWT_MARKER" DEPLOY_MEMINFO_FILE="$CASE/meminfo" DEPLOY_RUN_ID=t087test "$@" \
    bash "$CASE/scripts/deploy.sh" ${DEPLOY_ARGS:+$DEPLOY_ARGS} < "$WORK/stdin" > "$CASE/out" 2>&1
  RC=$?
  ELAPSED=$((SECONDS - t0))
}
# stdin 給一個一般檔案（⛔ 不是 /dev/null）：測試本身從 /dev/null 執行時，才驗得出 deploy.sh 有沒有換掉 stdin
printf 'x\n' > "$WORK/stdin"

# run_deploy_tty：在偽終端機裡執行（script(1)），重現 live 的情況：操作者在終端機直接執行 ./deploy.sh
run_deploy_tty() {
  local t0=$SECONDS cmd
  cmd="$(printf '%q ' env -i PATH="$FAKEBIN:/usr/bin:/bin" HOME="$WORK" CALL_LOG="$CASE/calls" \
    WORK_STATE="$CASE/state" FAKE_JWT_MARKER="$FAKE_JWT_MARKER" DEPLOY_MEMINFO_FILE="$CASE/meminfo" "$@" \
    bash "$CASE/scripts/deploy.sh")"
  script -qec "$cmd" /dev/null > "$CASE/out" 2>&1
  RC=$?
  ELAPSED=$((SECONDS - t0))
}

called() { grep -q -- "$1" "$CASE/calls"; }
no_secret() { ! grep -q "$FAKE_JWT_MARKER" "$CASE/out" && ! grep -q "STDERR-7c1e" "$CASE/out"; }

# 應該在 down 之前停下的失敗：回傳非 0、⛔ 沒有 down／up、輸出⛔ 沒有假 JWT
expect_stop_before_down() {
  local name="$1"; shift
  if [ "$RC" -eq 0 ]; then ng "$name：應回傳非 0"; return; fi
  if called "^compose down" || called "^compose up"; then ng "$name：⛔ 不能呼叫 down／up"; return; fi
  if ! no_secret; then ng "$name：輸出帶出了 JWT"; return; fi
  for forbidden in "$@"; do
    if called "$forbidden"; then ng "$name：⛔ 不能呼叫 $forbidden"; return; fi
  done
  ok "$name（rc $RC，停在 down 之前）"
}

echo "== 一、deploy.sh 的流程（假 git／docker）"

setup_case normal
run_deploy
order="$(grep -oE '^(git -C [^ ]+ (rev-parse --is-inside-work-tree|status|pull)|compose (config$|config --services|build [a-z-]+|run|down|up|exec)|docker (inspect -f [{][{][.]State[.]Running[}][}]|exec))' "$CASE/calls" \
  | sed -E 's#^git -C [^ ]+ #git #; s#^docker inspect.*#docker inspect-db#' | uniq | tr '\n' '|')"
want="git rev-parse --is-inside-work-tree|git status|docker inspect-db|docker exec|compose config|git pull|compose config --services|compose build backend|compose build python-server|compose build python-worker|compose run|docker exec|docker inspect-db|docker exec|compose down|compose up|compose exec|"
if [ "$RC" -eq 0 ] && [ "$order" = "$want" ] && ! called UP_WITH_BUILD && ! called WGET_WITHOUT_TIMEOUT && called "^compose up -d --no-build$" \
   && called "^compose run --rm --no-deps -T --entrypoint grep backend -qF check-config ./trading-backend$" \
   && called "^compose run --rm --no-deps -T backend ./trading-backend -check-config$" && no_secret; then
  ok "正常：呼叫順序 preflight（回測檢查）→ config（JWT）→ pull --ff-only → build ×3 → 確認旗標 → check-config → 回測檢查 → down → up -d --no-build → readiness"
else
  ng "正常：rc $RC，順序 $order"
fi
if called "pull --ff-only origin main"; then ok "正常：git pull 只允許 fast-forward"; else ng "正常：git pull 沒有 --ff-only"; fi
if called "^compose run" && called "^compose exec" && ! called STDIN_NOT_DEVNULL; then
  ok "正常：compose run／exec 的 stdin 是 /dev/null"
else
  ng "正常：compose run／exec 的 stdin 不是 /dev/null（$(grep -c STDIN_NOT_DEVNULL "$CASE/calls") 次）"
fi
# 2026-11-16 live 第一次部署的重現：在終端機執行、compose run／exec 會讀 stdin → ⛔ 不能卡到逾時
if command -v script >/dev/null 2>&1; then
  setup_case tty; run_deploy_tty FAKE_READ_STDIN=1
  if [ "$RC" -eq 0 ] && [ "$ELAPSED" -le 3 ] && grep -q "部署完成" "$CASE/out"; then
    ok "在終端機執行（compose 會讀 stdin）→ 通過，${ELAPSED} 秒"
  else
    ng "在終端機執行：rc $RC、${ELAPSED} 秒（卡在讀 stdin？）"
  fi
else
  ng "找不到 script(1)，無法重現終端機的情況"
fi

setup_case notgit;   run_deploy FAKE_NOT_GIT=1;      expect_stop_before_down "PROJECT_DIR 不是 git checkout" "pull"
setup_case branch;   run_deploy FAKE_BRANCH=feature; expect_stop_before_down "build 目錄不在部署分支" "pull"
setup_case dirty;    run_deploy FAKE_DIRTY=1;        expect_stop_before_down "build 目錄有本機修改" "pull"
setup_case nocompose; rm "$CASE/scripts/compose.yml"; run_deploy; expect_stop_before_down "compose.yml 不存在" "pull"
setup_case noenv;    rm "$CASE/scripts/.env";        run_deploy; expect_stop_before_down ".env 不存在" "pull"
setup_case perm;     chmod 644 "$CASE/scripts/.env"; run_deploy; expect_stop_before_down ".env 權限不是 600" "pull"
setup_case efpath;   sed -i "s#$CASE/scripts/.env#/elsewhere/.env#" "$CASE/scripts/compose.yml"; run_deploy
expect_stop_before_down "compose.yml 的 env_file 跟 APP_ENV_FILE 不一致" "pull"
setup_case efredis;  sed -i "s#$CASE/redis/.env#/elsewhere/redis.env#" "$CASE/scripts/compose.yml"; run_deploy
expect_stop_before_down "compose.yml 的 env_file 跟 REDIS_ENV_FILE 不一致" "pull"
setup_case redisbad; printf 'REDIS_PASSWORD=a\nOTHER=b\n' > "$CASE/redis/.env"; run_deploy
expect_stop_before_down "Redis 的 .env 不只一行 REDIS_PASSWORD" "pull"
setup_case redisquote; printf 'REDIS_PASSWORD="a b"\n' > "$CASE/redis/.env"; run_deploy
expect_stop_before_down "Redis 的 .env 的值有引號" "pull"
setup_case redismissing; rm "$CASE/redis/.env"; run_deploy
expect_stop_before_down "Redis 的 .env 不存在" "pull"
setup_case pull;     run_deploy FAKE_PULL_RC=1;      expect_stop_before_down "git pull 失敗" "config --services" "^compose build"
setup_case config;   run_deploy FAKE_CONFIG_RC=1;    expect_stop_before_down "compose config 失敗" "pull" "^compose build"
setup_case jwtempty; run_deploy FAKE_JWT_LINE='      AUTH_JWT_SECRET: ""'; expect_stop_before_down "JWT 是空值" "pull" "^compose build"
setup_case jwtmissing; run_deploy FAKE_JWT_LINE='      B_KEY: y'; expect_stop_before_down "backend 沒有 JWT" "pull" "^compose build"
for v in change-me-in-production change-me dev-only-change-me '"change-me"'; do
  setup_case "jwtpub"; run_deploy FAKE_JWT_LINE="      AUTH_JWT_SECRET: $v"
  expect_stop_before_down "JWT 是公開範例值 $v" "pull" "^compose build"
done
setup_case jwtother; run_deploy FAKE_JWT_LINE='      AUTH_JWT_SECRET: change-me-too'
if [ "$RC" -eq 0 ]; then ok "JWT 只是開頭像範例值 → 通過"; else ng "JWT 只是開頭像範例值：rc $RC"; fi
setup_case mem;      printf 'MemAvailable:    1000000 kB\n' > "$CASE/meminfo"; run_deploy
expect_stop_before_down "MemAvailable 低於門檻" "^compose build"
setup_case memmissing; printf 'MemTotal: 1 kB\n' > "$CASE/meminfo"; run_deploy
expect_stop_before_down "讀不到 MemAvailable" "^compose build"
setup_case build;    run_deploy FAKE_BUILD_FAIL=python-server; expect_stop_before_down "build 失敗" "^compose run" "compose build python-worker"
setup_case check;    run_deploy FAKE_CHECK_RC=1;     expect_stop_before_down "新 image 的設定檢查失敗"
setup_case checkhang; run_deploy FAKE_CHECK_DELAY=1000
if [ "$ELAPSED" -le 4 ]; then expect_stop_before_down "新 image 的設定檢查卡住（${ELAPSED} 秒逾時）"; else ng "設定檢查卡住：${ELAPSED} 秒才結束"; fi
# 舊的 binary 會忽略 -check-config、直接啟動整個服務：⛔ 不能執行它
setup_case noflag;   run_deploy FAKE_NO_FLAG=1
expect_stop_before_down "新 image 不認得 -check-config（比 E 舊）" "backend ./trading-backend -check-config"
grep -q "不認得 -check-config" "$CASE/out" || ng "找不到旗標：訊息要說不認得 -check-config"
# 探測本身出錯或逾時：⛔ 不能說成「找不到旗標」（2026-11-16 live 的誤判）
setup_case proberr;  run_deploy FAKE_PROBE_RC=125
expect_stop_before_down "確認旗標的 compose run 出錯（rc 125）" "backend ./trading-backend -check-config"
if grep -q "結束碼 125" "$CASE/out" && ! grep -q "不認得" "$CASE/out"; then ok "compose run 出錯 → 訊息帶結束碼、⛔ 不說找不到旗標"; else ng "compose run 出錯：訊息不對"; fi
setup_case probehang; run_deploy FAKE_PROBE_DELAY=1000
expect_stop_before_down "確認旗標的步驟卡住（${ELAPSED} 秒）" "backend ./trading-backend -check-config"
if [ "$ELAPSED" -le 4 ] && grep -q "確認 -check-config 的步驟逾時" "$CASE/out"; then ok "確認旗標卡住 → 逾時訊息"; else ng "確認旗標卡住：${ELAPSED} 秒、訊息不對"; fi

setup_case down; run_deploy FAKE_DOWN_RC=1
if [ "$RC" -ne 0 ] && called "^compose down" && ! called "^compose up"; then ok "down 失敗 → ⛔ 不 up"; else ng "down 失敗：rc $RC"; fi
setup_case up; run_deploy FAKE_UP_RC=1
if [ "$RC" -ne 0 ] && called "^compose up" && ! called "^compose exec"; then ok "up 失敗 → 回傳非 0"; else ng "up 失敗：rc $RC"; fi
setup_case readylate; run_deploy FAKE_HEALTH_AFTER=3
if [ "$RC" -eq 0 ] && [ "$(grep -c '^compose exec' "$CASE/calls")" -eq 3 ]; then ok "readiness：第 3 次才 ready → 通過"; else ng "readiness 第 3 次：rc $RC"; fi
# readiness 的上限是總截止時間（review 2026-11-16）：總上限 4 秒、單次 2 秒，所有情況都要在約 5 秒內結束
expect_ready_fail_bounded() {
  local name="$1" limit="$2"
  if [ "$RC" -ne 0 ] && [ "$ELAPSED" -le "$limit" ] && grep -q "還沒 ready" "$CASE/out"; then
    ok "$name → 回傳非 0，${ELAPSED} 秒結束（上限 ${limit}）"
  else
    ng "$name：rc $RC、${ELAPSED} 秒（上限 ${limit}）"
  fi
}
setup_case readynever; run_deploy FAKE_HEALTH_NEVER=1
n_exec="$(grep -c '^compose exec' "$CASE/calls")"
if [ "$n_exec" -ge 2 ]; then expect_ready_fail_bounded "readiness 一直失敗（確認 $n_exec 次）" 5; else ng "readiness 一直失敗：只確認了 $n_exec 次"; fi
setup_case readyslow; run_deploy FAKE_HEALTH_DELAY=3
expect_ready_fail_bounded "readiness 每次回應要 3 秒（單次上限 2 秒）" 5
setup_case readyhang; run_deploy FAKE_HEALTH_DELAY=1000
expect_ready_fail_bounded "readiness 的請求卡住" 5
setup_case readypshang; run_deploy FAKE_PS_DELAY=1000
expect_ready_fail_bounded "readiness 的 compose ps 卡住" 5
# review 的重現：宣告上限 1 秒、請求要 3 秒 → ⛔ 不能回報成功
READY_T=1 READY_REQ=5 setup_case readyrepro; run_deploy FAKE_HEALTH_DELAY=3
expect_ready_fail_bounded "總上限 1 秒、請求要 3 秒（review 的重現）" 2
setup_case readyslowok; run_deploy FAKE_HEALTH_DELAY=1
if [ "$RC" -eq 0 ] && [ "$ELAPSED" -le 3 ]; then ok "readiness 回應要 1 秒（在上限內）→ 通過"; else ng "readiness 回應 1 秒：rc $RC、${ELAPSED} 秒"; fi
setup_case restarted; run_deploy FAKE_RESTARTS=1
if [ "$RC" -ne 0 ] && ! called "^compose exec"; then ok "RestartCount 不是 0 → 回傳非 0"; else ng "RestartCount 1：rc $RC"; fi

# ── 回測的 pending／running（T-084）────────────────────────
# 假 docker 的回應：FAKE_BT<n>_OUT／RC／RUNNING／MISSING／INSPECT_DELAY／EXEC_DELAY／IGNORE_TERM（n＝第幾次檢查；沒有 n 的套用到每一次）
DB_DELAY=1017   # 卡住的延遲：用少見的秒數，事後才找得出有沒有殘留的 sleep
no_leftover() { /bin/sleep 0.3; ! pgrep -f "^/bin/sleep $DB_DELAY\$" >/dev/null; }
grep_out() { grep -qF -- "$1" "$CASE/out"; }
n_calls() { grep -c -- "$1" "$CASE/calls"; }

setup_case btnormal; run_deploy
sql_deploy="$(sed -n 's/^BACKTEST_CHECK_SQL="\(.*\)"$/\1/p' "$CASE/scripts/deploy.sh")"
if [ "$RC" -eq 0 ] && [ "$(n_calls '^docker exec .*backtest_jobs')" -eq 2 ] && [ "$(n_calls '^docker inspect -f {{.State.Running}} fake-pg$')" -eq 2 ] \
   && [ "$(grep -c 'pending 0、running 0' "$CASE/out")" -eq 2 ] && no_secret; then
  ok "回測：兩次都 0|0 → 部署完成，inspect 與 exec 各兩次、訊息印出兩次"
else
  ng "回測：兩次都 0|0：rc $RC、exec $(n_calls '^docker exec .*backtest_jobs') 次"
fi
if [ "$(grep '^docker exec .*backtest_jobs' "$CASE/calls" | grep -c -- '-e PGOPTIONS=-c default_transaction_read_only=on -c statement_timeout=5000 fake-pg psql -U fake_user -d trading ')" -eq 2 ]; then
  ok "回測：兩次的 PGOPTIONS 都是唯讀加 statement_timeout，帳號與 database 照開頭的設定"
else
  ng "回測：exec 的參數不對"
fi
if [ "$(n_calls '^TIMEOUT -k 5 1 docker inspect -f {{.State.Running}} fake-pg$')" -eq 2 ] && [ "$(n_calls '^TIMEOUT -k 5 1 docker exec .*backtest_jobs')" -eq 2 ] \
   && [ "$(n_calls '^TIMEOUT ')" -eq "$(n_calls '^docker exec \|^docker inspect -f {{.State.Running}}')" ]; then
  ok "回測：每一個 docker inspect／exec 都包 timeout -k 5"
else
  ng "回測：有 docker inspect／exec 沒有包 timeout -k（TIMEOUT $(n_calls '^TIMEOUT ') 筆）"
fi
if [ -n "$sql_deploy" ] && [ "$(cat "$CASE/state/bt_sql")" = "$sql_deploy" ]; then
  ok "回測：送出的 SQL 就是 deploy.sh 的 BACKTEST_CHECK_SQL"
else
  ng "回測：送出的 SQL 跟 BACKTEST_CHECK_SQL 不同"
fi

# 超出 Bash 整數範圍的筆數（review 2026-11-25 的反例）：⛔ 不能因為整數比較出錯而放行
for v in '1|0' '0|2' '9223372036854775808|0' '0|99999999999999999999'; do
  setup_case btfirst; run_deploy FAKE_BT1_OUT="$v\n"
  expect_stop_before_down "回測第一次 $v → 停在 pull 之前" "pull" "^compose build"
  p="${v%|*}"; r="${v#*|}"
  if ! { grep_out "pending $p、running $r" && grep_out "還沒 pull"; }; then ng "回測第一次 $v：訊息要有兩個筆數與「還沒 pull」"; fi
done
setup_case btsecondblank; run_deploy 'FAKE_BT2_OUT=0|0\n\n'
if [ "$RC" -ne 0 ] && grep_out "輸出格式不對" && grep_out "新 image 已經 build 好" && ! called "^compose down"; then
  ok "回測第二次 0|0 後面多一個空行 → 停下、⛔ 沒有 down"
else
  ng "回測第二次 0|0 後面多一個空行：rc $RC"
fi
setup_case btsecond; run_deploy FAKE_BT2_OUT='0|1\n'
if [ "$RC" -ne 0 ] && called "^compose build python-worker" && called "backend ./trading-backend -check-config" \
   && ! called "^compose down" && ! called "^compose up" && grep_out "pending 0、running 1" && grep_out "新 image 已經 build 好" && no_secret; then
  ok "回測第二次 0|1 → 有 build 與設定檢查、⛔ 沒有 down／up，訊息說新 image 已經 build 好"
else
  ng "回測第二次 0|1：rc $RC"
fi

stop_case() {   # stop_case 名稱 訊息片段 FAKE_…：第一次就查不到 → 停在 pull 之前、訊息分類對
  local name="$1" msg="$2"; shift 2
  setup_case btfail; run_deploy "$@"
  expect_stop_before_down "回測查不到：$name" "pull" "^compose build"
  grep_out "$msg" || ng "回測查不到：$name 的訊息要有「$msg」"
}
stop_case "container ⛔ 沒有在跑" "找不到在跑的 DB container DB_CONTAINER=fake-pg" FAKE_BT_RUNNING=false
stop_case "container 不存在" "找不到在跑的 DB container DB_CONTAINER=fake-pg" FAKE_BT_MISSING=1
stop_case "exec 結束碼 2" "查回測狀態失敗（結束碼 2" FAKE_BT_RC=2
stop_case "輸出是空的" "輸出格式不對" FAKE_BT_OUT=
stop_case "輸出兩行" "輸出格式不對" 'FAKE_BT_OUT=0|0\n1|0\n'
stop_case "輸出是文字" "輸出格式不對" 'FAKE_BT_OUT=abc\n'
stop_case "輸出 1|x" "輸出格式不對" 'FAKE_BT_OUT=1|x\n'
# review 2026-11-25 的反例：0|0 後面多一個空行（$(…) 會吃掉結尾的換行）；另外 0|0 沒有換行也⛔ 不是 psql 的正常輸出
stop_case "0|0 後面多一個空行" "輸出格式不對" 'FAKE_BT_OUT=0|0\n\n'
stop_case "0|0 沒有換行" "輸出格式不對" 'FAKE_BT_OUT=0|0'
stop_case "inspect 逾時" "查回測狀態逾時（inspect" FAKE_BT_INSPECT_DELAY=$DB_DELAY
if ! { [ "$ELAPSED" -le 3 ] && no_leftover; }; then ng "inspect 逾時：${ELAPSED} 秒、或留下 sleep"; fi
stop_case "exec 逾時" "查回測狀態逾時（exec" FAKE_BT_EXEC_DELAY=$DB_DELAY
if ! { [ "$ELAPSED" -le 3 ] && no_leftover; }; then ng "exec 逾時：${ELAPSED} 秒、或留下 sleep"; fi

for which in INSPECT EXEC; do
  setup_case btsecondhang; run_deploy "FAKE_BT2_${which}_DELAY=$DB_DELAY"
  if [ "$RC" -ne 0 ] && called "^compose build python-worker" && ! called "^compose down" && ! called "^compose up" \
     && grep_out "查回測狀態逾時（${which,,}" && [ "$ELAPSED" -le 4 ] && no_leftover; then
    ok "回測第二次的 ${which,,} 卡住 → ${ELAPSED} 秒逾時、⛔ 沒有 down／up、⛔ 沒有殘留的 sleep"
  else
    ng "回測第二次的 ${which,,} 卡住：rc $RC、${ELAPSED} 秒"
  fi
done
# 卡死的 daemon 呼叫連 TERM 都不理：timeout -k 5 的 KILL 要讓它在「逾時＋5 秒」內結束
setup_case btkill; run_deploy FAKE_BT_IGNORE_TERM=1 FAKE_BT_EXEC_DELAY=$DB_DELAY
if [ "$RC" -ne 0 ] && grep_out "查回測狀態逾時（exec" && [ "$ELAPSED" -le 8 ] && ! called "pull" && no_leftover; then
  ok "回測的 exec 忽略 TERM → KILL 後 ${ELAPSED} 秒結束、判成逾時、⛔ 沒有殘留"
else
  ng "回測的 exec 忽略 TERM：rc $RC、${ELAPSED} 秒"
fi

# 開頭的 DB 設定：佔位值沒換、空的、有不允許的字元 → 停下，⛔ 沒有任何 git／docker 呼叫
for c in "DB_C=your-postgres-container" "DB_U=your-db-user" "DB_C=" "DB_C=a;b" "DB_U=x y"; do
  ck="${c%%=*}"; v="${c#*=}"
  if [ "$ck" = DB_C ]; then DB_C="$v" setup_case btcfg; else DB_U="$v" setup_case btcfg; fi
  run_deploy
  if [ "$RC" -ne 0 ] && [ ! -s "$CASE/calls" ] && { grep_out "先填" || grep_out "只能用英數"; }; then
    ok "DB 設定 $c → 停下、⛔ 沒有呼叫 git／docker"
  else
    ng "DB 設定 $c：rc $RC、呼叫 $(wc -l < "$CASE/calls") 筆"
  fi
done

# ── migration（T-087）────────────────────────────────────────
FPA=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
INST="7712040185090818081|trading|16384|20261125093441474452"
PEND="FAKE_MS1_OUT=migrate: pending 81\nmigrate: fingerprint $FPA\nmigrate: instance $INST\n"
AFTER="FAKE_MS2_OUT=migrate: pending -\nmigrate: fingerprint bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\nmigrate: instance $INST\n"
line_of() { grep -n -m1 -- "$1" "$CASE/calls" | cut -d: -f1; }
before() { local a b; a="$(line_of "$1")"; b="$(line_of "$2")"; [ -n "$a" ] && [ -n "$b" ] && [ "$a" -lt "$b" ]; }
trial_clean() { ! ls "$CASE/state"/c_* "$CASE/state"/n_* >/dev/null 2>&1 && called "^docker network rm stock_trading-trial-t087test"; }
# 臨時密碼⛔ 不能出現在呼叫紀錄（指令列）與輸出；DSN 都經環境變數、試跑的 migrator 跟標記表確認用同一個 DSN
no_trial_secret() {
  local pw; pw="$(cat "$CASE/state/pw" 2>/dev/null)"
  [ -n "$pw" ] && ! grep -qF -- "$pw" "$CASE/calls" && ! grep -qF -- "$pw" "$CASE/out" \
    && ! grep -qF -- "$pw" "$CASE/state/argv" && ! grep -q "postgres://trial:" "$CASE/state/argv" "$CASE/calls" \
    && ! called "PW_NOT_FROM_ENV\|PW_MISSING\|MARKER_PW_DIFF\|MARKER_DSN_IN_ARGV\|DSN_NOT_FROM_ENV\|DSN_DIFF"
}

setup_case mignone; run_deploy
if [ "$RC" -eq 0 ] && called "migrate-statement-timeout ./trading-backend" && called "backend ./trading-backend -migrate=status" \
   && ! called "^docker network" && ! called "pg_dump" && ! called "migrate=up" && before "migrate=status" "^compose down" && grep_out "沒有待套用的 migration"; then
  ok "migration：pending 空 → 確認旗標、讀狀態（down 之前）；⛔ 沒有試跑、備份、-migrate=up"
else
  ng "migration：pending 空：rc $RC"
fi

setup_case migok; run_deploy "$PEND" "$AFTER"
if [ "$RC" -eq 0 ] && before "^docker network create --internal stock_trading-trial-t087test" "^compose down" \
   && before "^docker run --rm --name stock_trading-trialmig-t087test --network stock_trading-trial-t087test .* -migrate=up -run-id=trial-t087test -expect-fingerprint=$FPA" "^compose down" \
   && before "^compose down" " -Fc" && before " -Fc" "pg_restore -l" && before "pg_restore -l" "--name stock_trading-migrator-t087test backend ./trading-backend -migrate=up -run-id=t087test -expect-fingerprint=$FPA" \
   && before "-migrate=up -run-id=t087test" "^docker ps -a --filter name=^/stock_trading-migrator-t087test" && before "^docker ps -a --filter name=^/stock_trading-migrator-t087test" "^compose up" \
   && [ "$(grep -c '^compose run .*backend ./trading-backend -migrate=status' "$CASE/calls")" -eq 2 ] && trial_clean \
   && ! called "^compose run .*trialmig" && [ -s "$CASE/backup/trading_t087test_aaaaaaaaaaaa.dump" ] && [ "$(stat -c %a "$CASE/backup/trading_t087test_aaaaaaaaaaaa.dump")" = 600 ] \
   && called "^docker rm -fv stock_trading-trialmig-t087test stock_trading-trialpg-t087test" && no_trial_secret \
   && called "-migrate=up -run-id=t087test -expect-fingerprint=$FPA -expect-instance=7712040185090818081|trading|16384|20261125093441474452 " \
   && grep_out "migrator 與腳本連到同一個 DB"; then
  ok "migration：pending 81、試跑通過 → 試跑在 down 之前（docker run --network 試跑網路），備份與正式套用（-expect-fingerprint）在 down 之後、up 之前，停止確認後才 up；試跑資源都刪了"
else
  ng "migration：pending 81 試跑通過：rc $RC：$(tail -2 "$CASE/out" | tr '\n' ' ')"
fi
if grep -q "trial_cleanup\|沒有刪乾淨" "$CASE/out"; then ng "migration：成功路徑不該有清理失敗的訊息"; fi

stop_mig() {   # stop_mig 名稱 訊息片段 FAKE_…：停在 down 之前，試跑的資源刪乾淨
  local name="$1" msg="$2"; shift 2
  setup_case migstop; run_deploy "$PEND" "$@"
  if [ "$RC" -ne 0 ] && ! called "^compose down" && ! called "^compose up" && grep_out "$msg" && grep_out "還沒 down" && no_secret; then
    if called "^docker network create"; then
      if trial_clean; then ok "migration：$name → 停在 down 之前、試跑資源刪乾淨"; else ng "migration：$name：試跑資源沒刪乾淨"; fi
    else
      ok "migration：$name → 停在 down 之前"
    fi
  else
    ng "migration：$name：rc $RC：$(tail -1 "$CASE/out")"
  fi
}
stop_mig "新 image 不認得 -migrate" "不認得 -migrate" FAKE_NO_MIGRATE_FLAG=1
stop_mig "status 結束碼 2" "沒有 goose_db_version" FAKE_MS1_RC=2
stop_mig "status 輸出格式不對" "輸出格式不對" 'FAKE_MS1_OUT=migrate: pending 81\n'
# T-087 review：版本相同、DB 不同（migrator 與腳本指向不同的實例或 database）→ 停在 down 之前、⛔ 沒有試跑
stop_mig "版本相同、DB 不同（database 不同）" "指向不同的 DB" "FAKE_INST=7712040185090818081|trading_other|16999|20261125093441474452"
called "^docker network create" && ng "migration：DB 不同時⛔ 不能開始試跑"
stop_mig "版本相同、DB 不同（實例不同）" "指向不同的 DB" "FAKE_INST=1111111111111111111|trading|16384|20261101000000000000"
stop_mig "migrator 沒印實例識別" "讀不到 migrator 連到的 DB 的實例識別" 'FAKE_MS1_OUT=migrate: pending 81\nmigrate: fingerprint aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n'
stop_mig "腳本查不到實例識別" "查不到實例識別" "FAKE_INST=garbage"
# T-087 review：試跑的名稱已經存在（⛔ 不是這一次建立的）→ 停下、⛔ 不清理它
setup_case migexists; : > "$CASE/state/n_stock_trading-trial-t087test"; run_deploy "$PEND"
if [ "$RC" -ne 0 ] && ! called "^compose down" && grep_out "已經存在" && ! called "^docker network rm" && ! called "^docker rm" && [ -e "$CASE/state/n_stock_trading-trial-t087test" ]; then
  ok "migration：試跑網路的名稱已經存在 → 停在 down 之前、⛔ 不清理別人的資源"
else
  ng "migration：試跑網路的名稱已經存在：rc $RC"
fi
stop_mig "試跑網路建不起來" "建立試跑網路失敗" FAKE_NET_CREATE_RC=1
called "^docker rm" && ng "migration：試跑容器還沒建立時⛔ 不能 docker rm"
stop_mig "試跑 PostgreSQL 起不來" "啟動試跑用 PostgreSQL 失敗" FAKE_TRIALPG_RC=125
stop_mig "匯出 schema 失敗" "匯出或還原 live 的 schema 失敗" FAKE_DUMP_SCHEMA_RC=1
stop_mig "還原失敗" "匯出或還原 live 的 schema 失敗" FAKE_RESTORE_RC=3
stop_mig "匯出版本紀錄失敗" "goose_db_version 的紀錄失敗" FAKE_DUMP_DATA_RC=1
stop_mig "目標標記表不存在" "找不到目標標記表" FAKE_MARKER=f
stop_mig "臨時庫的指紋跟 live 不同" "版本紀錄跟 live 不同" "FAKE_TS_OUT=migrate: pending 81\nmigrate: fingerprint bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\n"
stop_mig "試跑 -migrate=up 失敗" "-migrate=up 沒有通過" FAKE_TU_RC=3
stop_mig "試跑之後 pending 不是空的" "-migrate=up 沒有通過" FAKE_TU_AFTER=81
setup_case migleak; run_deploy "$PEND" FAKE_TRIAL_LEAK=1
if [ "$RC" -ne 0 ] && ! called "^compose down" && grep_out "沒有刪乾淨"; then ok "migration：試跑資源刪不掉 → 停在 down 之前、印出沒刪乾淨的名稱"; else ng "migration：試跑資源刪不掉：rc $RC"; fi

stop_after_down() {   # stop_after_down 名稱 訊息片段 FAKE_…：down 之後停下、⛔ 沒有 up
  local name="$1" msg="$2"; shift 2
  setup_case migdown; run_deploy "$PEND" "$AFTER" "$@"
  if [ "$RC" -ne 0 ] && called "^compose down" && ! called "^compose up" && ! called "goose down" && grep_out "$msg" && grep_out "服務目前是停的" && no_secret; then
    ok "migration：$name → ⛔ 沒有 up、印出下一步"
  else
    ng "migration：$name：rc $RC：$(tail -1 "$CASE/out")"
  fi
}
stop_after_down "備份失敗" "備份失敗，⛔ 不套用" FAKE_BACKUP_RC=1
if called "migrate=up -run-id=t087test"; then ng "migration：備份失敗時⛔ 不能正式套用"; fi
stop_after_down "備份讀不出來" "備份失敗，⛔ 不套用" FAKE_RESTORE_L_RC=1
# T-087 review：新備份改名失敗、同名的舊檔還在 → ⛔ 不能把舊檔當成這次的備份、⛔ 不呼叫正式 migrator
stop_after_down "備份連結失敗" "備份失敗，⛔ 不套用" FAKE_LN_FAIL=1
if called "migrate=up -run-id=t087test"; then ng "migration：備份連結失敗時⛔ 不能正式套用"; fi
setup_case migoldbk; printf 'OLD' > "$CASE/backup/trading_t087test_aaaaaaaaaaaa.dump"; run_deploy "$PEND" "$AFTER"
if [ "$RC" -ne 0 ] && ! called "migrate=up -run-id=t087test" && ! called "^compose up" && grep_out "已經存在" && [ "$(cat "$CASE/backup/trading_t087test_aaaaaaaaaaaa.dump")" = OLD ]; then
  ok "migration：同名的舊備份已經存在 → 備份失敗、⛔ 不覆蓋、⛔ 不正式套用"
else
  ng "migration：同名的舊備份：rc $RC"
fi
setup_case migoldbk2; printf 'OLD' > "$CASE/backup/trading_t087test_aaaaaaaaaaaa.dump"; run_deploy "$PEND" "$AFTER" FAKE_LN_FAIL=1
if [ "$RC" -ne 0 ] && ! called "migrate=up -run-id=t087test" && [ "$(cat "$CASE/backup/trading_t087test_aaaaaaaaaaaa.dump")" = OLD ]; then
  ok "migration：舊備份在、新備份連結又失敗 → ⛔ 不正式套用"
else
  ng "migration：舊備份在、改名失敗：rc $RC"
fi
# review 的重現：同名檔在 pg_dump 期間才出現（開頭的檢查之後）→ ⛔ 不能覆寫它、⛔ 不能把它當成這次的備份、⛔ 不正式套用
setup_case migplant; run_deploy "$PEND" "$AFTER" FAKE_BACKUP_PLANT_OLD=1
if [ "$RC" -ne 0 ] && ! called "migrate=up -run-id=t087test" && ! called "^compose up" && grep_out "已經存在（pg_dump 期間出現？）" \
   && [ "$(cat "$CASE/backup/trading_t087test_aaaaaaaaaaaa.dump")" = OLD ] && [ ! -e "$CASE/backup/trading_t087test_aaaaaaaaaaaa.dump.partial" ]; then
  ok "migration：pg_dump 期間出現同名檔 → ⛔ 不覆寫（原檔保留）、備份失敗、⛔ 不正式套用"
else
  ng "migration：pg_dump 期間出現同名檔：rc $RC"
fi
# review：晚出現的同名路徑是目錄、或指向目錄的符號連結 → ln -T 直接拒絕（⛔ 不把連結建到那個目錄裡），.partial 清掉、⛔ 不正式套用
for kind in dir symlink; do
  setup_case "migplant$kind"; run_deploy "$PEND" "$AFTER" FAKE_BACKUP_PLANT_OLD=$kind
  B="$CASE/backup/trading_t087test_aaaaaaaaaaaa.dump"
  inside="$(find -L "$B" "$CASE/state/elsewhere" -mindepth 1 2>/dev/null | wc -l)"
  if [ "$RC" -ne 0 ] && ! called "migrate=up -run-id=t087test" && ! called "^compose up" && grep_out "已經存在（pg_dump 期間出現？）" \
     && [ "$inside" = 0 ] && [ ! -e "$B.partial" ] && [ "$(find "$CASE/backup" -name '*.partial' | wc -l)" = 0 ]; then
    ok "migration：pg_dump 期間出現同名的 $kind → ln -T 拒絕、⛔ 沒有多出連結、.partial 清掉、⛔ 不正式套用"
  else
    ng "migration：pg_dump 期間出現同名的 $kind：rc $RC、裡面 $inside 個"
  fi
done
setup_case migpartial; printf 'OTHER' > "$CASE/backup/trading_t087test_aaaaaaaaaaaa.dump.partial"; run_deploy "$PEND" "$AFTER"
if [ "$RC" -ne 0 ] && ! called "migrate=up -run-id=t087test" && ! called " -Fc" && [ "$(cat "$CASE/backup/trading_t087test_aaaaaaaaaaaa.dump.partial")" = OTHER ]; then
  ok "migration：.partial 已經存在 → ⛔ 不覆寫、⛔ 不 pg_dump、⛔ 不正式套用"
else
  ng "migration：.partial 已經存在：rc $RC"
fi
# down 之後 DB 的實例變了（重啟、切換）→ ⛔ 不備份、⛔ 不套用
stop_after_down "down 之後實例變了" "實例識別變了" "FAKE_INST2=7712040185090818081|trading|16384|20261125120000000000"
if called " -Fc" || called "migrate=up -run-id=t087test"; then ng "migration：實例變了⛔ 不能備份或套用"; fi
stop_after_down "正式套用結束碼 3" "有一支失敗" FAKE_MUP_RC=3
grep_out "還原到乾淨的 DB" || ng "migration：結束碼 3 的訊息要有還原的下一步"
called "^docker ps -a --filter name=^/stock_trading-migrator-t087test" || ng "migration：結束碼 3 也要做停止確認"
stop_after_down "正式套用結束碼 4" "正式套用逾時" FAKE_MUP_RC=4
stop_after_down "正式套用結束碼 5" "沒有套用任何一支" FAKE_MUP_RC=5
grep_out "回滾標籤" || ng "migration：結束碼 5 的訊息要說用回滾標籤啟動舊 image"
stop_after_down "套用之後 pending 不是空的" "套用之後 pending 不是空的" "FAKE_MS2_OUT=migrate: pending 81\nmigrate: fingerprint bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\n"

# 外層 timeout 砍掉 docker CLI、容器與 DB 連線都還在 → 停止確認：stop 容器、cancel 連線、歸零之後才印復原訊息
OUTER_T=2 setup_case migouter; run_deploy "$PEND" "$AFTER" FAKE_MUP_DELAY=$DB_DELAY FAKE_SESSIONS=2
if [ "$RC" -ne 0 ] && grep_out "外層 timeout 到期" && called "^docker stop -t 30 stock_trading-migrator-t087test" \
   && before "^docker stop -t 30" "pg_cancel_backend" && ! called "pg_terminate_backend" && grep_out "停止確認通過" && ! called "^compose up" && no_leftover; then
  ok "migration：外層 timeout 到期（容器還在跑、DB 有 2 個連線）→ stop 容器、cancel 連線、歸零之後才印復原訊息；⛔ 沒有 up"
else
  ng "migration：外層 timeout：rc $RC：$(tail -2 "$CASE/out" | tr '\n' ' ')"
fi
if called "pg_cancel_backend" && ! grep "pg_cancel_backend" "$CASE/calls" | grep -q "application_name = 'trading-migrator-t087test' AND datname = 'trading'"; then
  ng "migration：cancel 只能對本次的 application_name 與部署目標的 database"
fi
OUTER_T=2 setup_case migkill; run_deploy "$PEND" "$AFTER" FAKE_MUP_DELAY=$DB_DELAY FAKE_STOP_WORKS=0
if [ "$RC" -ne 0 ] && before "^docker stop" "^docker kill" && grep_out "停止確認通過" && ! called "^compose up" && no_leftover; then
  ok "migration：docker stop 停不下來 → docker kill，確認退出後才印復原訊息"
else
  ng "migration：stop 停不下來：rc $RC"
fi
OUTER_T=2 setup_case migstuck; run_deploy "$PEND" "$AFTER" FAKE_MUP_DELAY=$DB_DELAY FAKE_SESSIONS=1 FAKE_CANCEL_WORKS=0 FAKE_TERMINATE_WORKS=0
if [ "$RC" -ne 0 ] && before "pg_cancel_backend" "pg_terminate_backend" && grep_out "需要人工確認 migrator 已停止" && ! grep_out "外層 timeout 到期" \
   && ! called "^compose up" && no_leftover; then
  ok "migration：cancel、terminate 之後連線還在 → 停下、印「需要人工確認」，⛔ 沒有任何復原訊息與 up"
else
  ng "migration：連線停不掉：rc $RC：$(tail -1 "$CASE/out")"
fi
OUTER_T=2 setup_case migdockerdown; run_deploy "$PEND" "$AFTER" FAKE_MUP_DELAY=$DB_DELAY FAKE_PS_MIGRATOR_RC=1
if [ "$RC" -ne 0 ] && grep_out "需要人工確認 migrator 已停止" && ! grep_out "外層 timeout 到期" && ! called "^compose up" && no_leftover; then
  ok "migration：查不到容器狀態 → 停下、印「需要人工確認」"
else
  ng "migration：查不到容器狀態：rc $RC"
fi

# T-087 review：部署互斥——同一個 scripts 目錄同時只能有一個 deploy.sh 過了 pull 之前的鎖
# 第一個：readiness 第 5 次才 ready（up 之後還要約 4 秒，期間鎖還在）
READY_T=10 setup_case lock
( run_deploy FAKE_HEALTH_AFTER=5 ) &
lock_pid=$!
for _ in $(seq 1 100); do grep -q "^compose up" "$CASE/calls" 2>/dev/null && break; /bin/sleep 0.1; done
/usr/bin/timeout -k 2 30 env -i PATH="$FAKEBIN:/usr/bin:/bin" HOME="$WORK" CALL_LOG="$CASE/calls2" WORK_STATE="$CASE/state2" \
  FAKE_JWT_MARKER="$FAKE_JWT_MARKER" DEPLOY_MEMINFO_FILE="$CASE/meminfo" DEPLOY_RUN_ID=t087second \
  bash "$CASE/scripts/deploy.sh" < "$WORK/stdin" > "$CASE/out2" 2>&1
rc2=$?
mkdir -p "$CASE/state2"
wait "$lock_pid"
if [ "$rc2" -ne 0 ] && grep -q "另一個 deploy.sh 正在部署同一個目標" "$CASE/out2" && ! grep -q "pull\|^compose build" "$CASE/calls2" && grep -q "部署完成" "$CASE/out"; then
  ok "部署互斥：第一個還在 build 之後的步驟時，第二個停在 pull 之前；第一個照常完成"
else
  ng "部署互斥：rc2 $rc2：$(tail -1 "$CASE/out2")"
fi
setup_case lock2; run_deploy
if [ "$RC" -eq 0 ] && grep_out "部署鎖："; then ok "部署互斥：前一個結束之後鎖就放掉，下一次照常部署"; else ng "部署互斥：鎖沒放掉：rc $RC"; fi

# 手動的停止確認
DEPLOY_ARGS="--confirm-migrator-stopped t087test"
setup_case migconfirm; run_deploy FAKE_SESSIONS=1
if [ "$RC" -eq 0 ] && grep_out "停止確認通過" && called "pg_cancel_backend" && ! called "^git" && ! called "^compose"; then
  ok "--confirm-migrator-stopped：只做停止確認（⛔ 沒有 git／compose）"
else
  ng "--confirm-migrator-stopped：rc $RC"
fi
DEPLOY_ARGS="--confirm-migrator-stopped a;b"
setup_case migconfirmbad; run_deploy
if [ "$RC" -ne 0 ] && ! called "^docker"; then ok "--confirm-migrator-stopped 的 RUN_ID 格式不對 → 停下"; else ng "--confirm-migrator-stopped 格式不對：rc $RC"; fi
DEPLOY_ARGS=

# BACKUP_DIR：佔位值沒換、權限不是 700 → preflight 就停下（⛔ 沒有任何 git／docker 呼叫）
BK_DIR=/path/to/db-backups setup_case bkph; run_deploy
if [ "$RC" -ne 0 ] && [ ! -s "$CASE/calls" ] && grep_out "先填 BACKUP_DIR"; then ok "BACKUP_DIR 佔位值 → 停下、⛔ 沒有呼叫"; else ng "BACKUP_DIR 佔位值：rc $RC"; fi
setup_case bkperm; chmod 755 "$CASE/backup"; run_deploy
if [ "$RC" -ne 0 ] && [ ! -s "$CASE/calls" ] && grep_out "BACKUP_DIR 的權限必須是 700"; then ok "BACKUP_DIR 權限 755 → 停下"; else ng "BACKUP_DIR 權限：rc $RC"; fi

# --preflight：只跑 preflight 與 JWT 檢查，⛔ 不 pull、不 build、不 run、不 down／up
DEPLOY_ARGS=--preflight
setup_case pre; run_deploy
if [ "$RC" -eq 0 ] && grep -q "preflight 通過" "$CASE/out" && called "^compose config$" \
   && ! called "pull" && ! called "^compose build" && ! called "^compose run" && ! called "^compose down" && ! called "^compose up"; then
  ok "--preflight：通過、⛔ 沒有 pull／build／run／down／up"
else
  ng "--preflight：rc $RC"
fi
if [ "$(grep -c '^docker exec ' "$CASE/calls")" -eq 1 ] && grep -qF "pending 0、running 0" "$CASE/out"; then
  ok "--preflight：回測只查一次、印出 pending 0、running 0"
else
  ng "--preflight：回測查了 $(grep -c '^docker exec ' "$CASE/calls") 次"
fi
setup_case prebt; run_deploy 'FAKE_BT_OUT=1|0\n'; expect_stop_before_down "--preflight：回測 1|0 → 停下" "pull"
setup_case preperm; chmod 644 "$CASE/scripts/.env"; run_deploy; expect_stop_before_down "--preflight：.env 權限不是 600" "pull"
setup_case prejwt; run_deploy FAKE_JWT_LINE='      AUTH_JWT_SECRET: change-me'; expect_stop_before_down "--preflight：JWT 是公開範例值" "pull"
DEPLOY_ARGS=--bogus
setup_case badarg; run_deploy
if [ "$RC" -ne 0 ] && [ ! -s "$CASE/calls" ]; then ok "不認得的參數 → 回傳非 0、⛔ 沒有呼叫 git／docker"; else ng "不認得的參數：rc $RC"; fi
DEPLOY_ARGS=

# 假 docker 本身要擋得住「run 只帶旗標」：把範本的執行檔拿掉，確認會失敗
setup_case noexe; sed -i 's#backend ./trading-backend -check-config#backend -check-config#' "$CASE/scripts/deploy.sh"; run_deploy
if [ "$RC" -ne 0 ] && called RUN_WITHOUT_EXECUTABLE && ! called "^compose down"; then
  ok "（測試本身）run 沒寫執行檔 → 假 docker 判失敗、⛔ 沒有 down"
else
  ng "（測試本身）run 沒寫執行檔沒有被擋：rc $RC"
fi

echo "== 二、compose 範本與 .env.example（真的 docker compose config）"

if ! docker compose version >/dev/null 2>&1; then
  ng "找不到 docker compose，第二部分無法執行"
else
  L="$WORK/layout"
  mkdir -p "$L/scripts" "$L/redis" "$L/other"
  sed -e "s#/path/to/scripts/stock_trading/.env#$L/scripts/.env#" -e "s#/path/to/redis-stack/.env#$L/redis/.env#" \
    "$REPO/docker-compose.yml" > "$L/scripts/compose.yml"
  cp "$REPO/.env.example" "$L/scripts/.env"
  chmod 600 "$L/scripts/.env"
  sed -i "s#^AUTH_JWT_SECRET=\$#AUTH_JWT_SECRET=$FAKE_JWT_MARKER#" "$L/scripts/.env"
  cat >> "$L/scripts/.env" <<'EOF'
T_SQ='a$b#c d'
T_UNQ=abc #comment
T_UNQ2=abc#kept
T_DQ="x#y"
T_SPACE='  lead'
EOF
  # 前後都有空白的值（用 printf 寫，原始碼裡才不會有行尾空白）
  printf 'T_TRIM=  trail  \n' >> "$L/scripts/.env"
  printf 'REDIS_PASSWORD=fake-redis-pw\n' > "$L/redis/.env"
  # 誘餌：執行時所在的目錄（other）放一個 .env，確認⛔ 不會被讀到（F1：compose 只找 project directory 的 .env，
  # 而那是 build 目錄；這裡的 project directory 是 repo 根目錄，所以根目錄⛔ 不能有 .env）
  printf 'GIN_MODE=debug\nLOG_RETENTION_DAYS=99\nAUTH_JWT_SECRET=decoy\n' > "$L/other/.env"
  [ -e "$REPO/.env" ] && ng "repo 根目錄有 .env，會被 compose 當成 interpolation 的來源，第二部分的結果不可信"
  benv() {
    (cd "$1" && env -u REDIS_PASSWORD -u AUTH_JWT_SECRET docker compose -f "$L/scripts/compose.yml" \
      --project-directory "$REPO" config --format json 2>/dev/null) \
      | python3 -I -c 'import json,sys; e=json.load(sys.stdin)["services"]["backend"]["environment"]; print(json.dumps(e, sort_keys=True))'
  }
  e_repo="$(benv "$REPO")"; e_scripts="$(benv "$L/scripts")"; e_other="$(benv "$L/other")"
  if [ -n "$e_repo" ] && [ "$e_repo" = "$e_scripts" ] && [ "$e_repo" = "$e_other" ]; then
    ok "從 repo、scripts、其他目錄執行，backend 最終的設定都相同"
  else
    ng "從不同目錄執行，backend 最終的設定不同（或解析失敗）"
  fi
  # shellcheck disable=SC2016  # 單引號裡是 python 程式，本來就不要 shell 展開
  printf '%s' "$e_repo" | python3 -I -c '
import json, sys
e = json.load(sys.stdin)
marker = sys.argv[1]
want = {
    "AUTH_JWT_SECRET": marker, "REDIS_PASSWORD": "fake-redis-pw",
    # config 的輸出把 $ 寫成 $$；container 拿到的是一個 $（2026-11-16 實測）
    "T_SQ": "a$$b#c d", "T_UNQ": "abc", "T_UNQ2": "abc#kept", "T_DQ": "x#y", "T_SPACE": "  lead", "T_TRIM": "trail",
    "EVALUATION_UNIVERSE_CRON": "0 16 * * 1-5", "CORPORATE_ACTION_CRON": "30 6 * * 1-5",
    "DELISTING_URL": "https://www.twse.com.tw/rwd/zh/company/suspendListing?response=csv",
    "JOB_RETRY_ENABLED": "false", "JOB_RETRY_CUTOFF": "08:45", "FINMIND_API_KEY": "",
    # 誘餌的 .env ⛔ 不能被讀到
    "GIN_MODE": "", "LOG_RETENTION_DAYS": "14",
}
bad = [k for k, v in want.items() if e.get(k) != v]
if bad:
    print("  FAIL env_file 解析的結果不符：" + "、".join(bad)); sys.exit(1)
print("  ok   env_file 的值（含 $、#、引號、空白、cron）照 .env.example 開頭的說明解析；REDIS_PASSWORD 來自 Redis 的 .env")
' "$FAKE_JWT_MARKER" && PASS=$((PASS + 1)) || FAIL=$((FAIL + 1))

  sed "s#$L/redis/.env#$L/redis/missing.env#" "$L/scripts/compose.yml" > "$L/scripts/compose.missing.yml"
  if docker compose -f "$L/scripts/compose.missing.yml" --project-directory "$REPO" config -q >/dev/null 2>&1; then
    ng "env_file 指到不存在的檔應該報錯"
  else
    ok "env_file 指到不存在的檔 → compose 報錯（required: true）"
  fi
  if docker compose -f "$REPO/docker-compose.yml" --project-directory "$REPO" config -q >/dev/null 2>&1; then
    ng "範本的 /path/to 佔位沒改時應該報錯"
  else
    ok "範本的 /path/to 佔位沒改 → compose 報錯"
  fi
fi

# 靜態檢查：compose 範本、.env.example、deploy.sh 之間的 contract
python3 -I - "$REPO" <<'EOF' && PASS=$((PASS + 1)) || FAIL=$((FAIL + 1))
import re, sys, os
repo = sys.argv[1]
compose = open(os.path.join(repo, "docker-compose.yml"), encoding="utf-8").read()
example = open(os.path.join(repo, ".env.example"), encoding="utf-8").read()
deploy = open(os.path.join(repo, "deploy.sh"), encoding="utf-8").read()
contract = open(os.path.join(repo, "backend/internal/config/contract.go"), encoding="utf-8").read()
errs = []

# backend 的 environment 鍵
m = re.search(r"^  backend:\n(.*?)(?=^  \S|\Z)", compose, re.S | re.M)
block = m.group(1) if m else ""
em = re.search(r"^    environment:\n((?:      .*\n|\s*#.*\n)*)", block, re.M)
env_keys = set(re.findall(r"^      ([A-Z0-9_]+):", em.group(1) if em else "", re.M))
if not env_keys:
    errs.append("讀不到 backend 的 environment")
if "REDIS_PASSWORD" in env_keys:
    errs.append("backend 的 environment 有 REDIS_PASSWORD（會蓋掉 env_file）")
if re.search(r"\$\{[A-Z0-9_]+:?\?", compose):
    errs.append("compose 範本有必填的 ${…:?}")
if re.search(r"^\s*[^#\s].*format:\s*raw", block, re.M):
    errs.append("backend 的 env_file 用了 format: raw")

ex_keys = re.findall(r"^([A-Z0-9_]+)=", example, re.M)
if len(ex_keys) != len(set(ex_keys)):
    errs.append(".env.example 有重複的鍵")
ex = set(ex_keys)
if re.search(r"^\s*export\s", example, re.M):
    errs.append(".env.example 有 export")
if "REDIS_PASSWORD" in ex:
    errs.append(".env.example 有 REDIS_PASSWORD")
dup = sorted(ex & env_keys)
if dup:
    errs.append(".env.example 跟 environment 重複：" + "、".join(dup))

# 舊 deploy.sh 範本 export 的鍵（I-132 之前；REDIS_PASSWORD 除外）＋ 舊 compose 範本 environment 裡的應用設定，⛔ 不能漏
old = """AUTH_JWT_SECRET FINMIND_API_KEY FINMIND_INTRADAY_ENABLED FUGLE_ENABLED FUGLE_API_KEY YAHOO_ENABLED
YAHOO_RATE_LIMIT YAHOO_BATCH_SIZE EVALUATION_UNIVERSE_ENABLED EVALUATION_UNIVERSE_CRON EVALUATION_UNIVERSE_DAYS
CANDLE_GAP_DETECTION_ENABLED CANDLE_GAP_DETECTION_AGGREGATE_RATIO CANDLE_GAP_DETECTION_AGGREGATE_MIN_SYMBOLS
CANDLE_GAP_DETECTION_CANDIDATE_CAP_PER_RUN CANDLE_GAP_DETECTION_TIMEOUT_SEC CANDLE_GAP_DETECTION_LOOKBACK_TRADING_DAYS
CANDLE_GAP_DETECTION_REQUEST_INTERVAL_MS CANDLE_GAP_DETECTION_MARKET_STALE_DAYS CANDLE_GAP_DETECTION_CALENDAR_TTL_HOURS
CANDLE_GAP_DETECTION_BREAKER_FAILURES CANDLE_GAP_DETECTION_BREAKER_COOLDOWN_MIN SR_ANALYSIS_ENABLED SR_ANALYSIS_CRON
SR_ANALYSIS_CHIP_CRON SR_ANALYSIS_TIMEFRAME SR_ANALYSIS_LIMIT CORPORATE_ACTION_CRON DELISTING_ENABLED DELISTING_CRON
DELISTING_TIMEOUT_SEC JOB_RETRY_ENABLED JOB_RETRY_INTERVAL_MIN JOB_RETRY_MAX_RERUNS JOB_RETRY_CUTOFF
SR_ZONE_VERIFY_DAYS SR_ZONE_VERIFY_MAX_ANALYSES CORPORATE_ACTION_TIMEOUT_SEC CORPORATE_ACTION_SHARD_COUNT DELISTING_URL""".split()
missing = sorted(set(old) - ex - env_keys)
if missing:
    errs.append(".env.example 漏了：" + "、".join(missing))

# 機密留空、開關都是 false
for k in ("AUTH_JWT_SECRET", "FINMIND_API_KEY", "FUGLE_API_KEY"):
    if not re.search(rf"^{k}=$", example, re.M):
        errs.append(f".env.example 的 {k} 要留空")
for k in sorted(k for k in ex if k.endswith("_ENABLED")):
    if not re.search(rf"^{k}=false$", example, re.M):
        errs.append(f".env.example 的 {k} 要是 false")

# deploy.sh 的公開範例 JWT 要跟設定契約一致
dm = re.search(r'^PUBLIC_JWT_SECRETS="([^"]*)"', deploy, re.M)
cm = re.search(r"PublicJWTSecrets\s*=\s*\[\]string\{([^}]*)\}", contract)
if not dm or not cm or sorted(dm.group(1).split()) != sorted(re.findall(r'"([^"]*)"', cm.group(1))):
    errs.append("deploy.sh 的 PUBLIC_JWT_SECRETS 跟 contract.go 的 PublicJWTSecrets 不一致")
if re.search(r"^\s*export\s", deploy, re.M):
    errs.append("deploy.sh 還有 export")
if "up --build" in deploy:
    errs.append("deploy.sh 還有 up --build")

# T-087：deploy.sh 的 INSTANCE_SQL 要跟 backend 的 database.InstanceSQL 逐字相同（兩邊的輸出直接比對）
conn = open(os.path.join(repo, "backend/internal/database/migrator_conn.go"), encoding="utf-8").read()
gm = re.search(r"const InstanceSQL = ((?:`[^`]*`\s*\+?\s*)+)", conn)
go_sql = "".join(re.findall(r"`([^`]*)`", gm.group(1))) if gm else None
sm = re.search(r'^INSTANCE_SQL="([^"]*)"$', deploy, re.M)
if not go_sql or not sm or sm.group(1) != go_sql:
    errs.append("deploy.sh 的 INSTANCE_SQL 跟 database.InstanceSQL 不一致")

if errs:
    for e in errs:
        print("  FAIL " + e)
    sys.exit(1)
print(f"  ok   範本 contract：environment {len(env_keys)} 個鍵、.env.example {len(ex)} 個鍵，不重複、不漏鍵；"
      "沒有 REDIS_PASSWORD、${…:?}、export；deploy.sh 的公開範例 JWT 跟 contract.go 一致、INSTANCE_SQL 跟 database.InstanceSQL 一致")
EOF

echo "== 三、dc.sh"
D="$WORK/dc"
mkdir -p "$D/scripts"
sed "s#^PROJECT_DIR=\"/path/to/trading\"#PROJECT_DIR=\"$D/build\"#" "$REPO/dc.sh" > "$D/scripts/dc.sh"
: > "$D/scripts/compose.yml"
run_dc() {
  : > "$D/calls"
  env -i PATH="$FAKEBIN:/usr/bin:/bin" CALL_LOG="$D/calls" WORK_STATE="$D" bash "$D/scripts/dc.sh" "$@" > "$D/out" 2>&1
  RC=$?
}
run_dc ps
if [ "$RC" -eq 0 ] && grep -qx "compose ps" "$D/calls"; then ok "dc.sh ps → 直接執行"; else ng "dc.sh ps：rc $RC"; fi
run_dc config
if [ "$RC" -eq 2 ] && [ ! -s "$D/calls" ]; then ok "dc.sh config → 擋下、⛔ 沒有呼叫 docker"; else ng "dc.sh config：rc $RC"; fi
run_dc down
if [ "$RC" -eq 2 ] && [ ! -s "$D/calls" ]; then ok "dc.sh down（沒有 --yes）→ 擋下"; else ng "dc.sh down：rc $RC"; fi
run_dc --yes down
if [ "$RC" -eq 0 ] && grep -qx "compose down" "$D/calls"; then ok "dc.sh --yes down → 執行"; else ng "dc.sh --yes down：rc $RC"; fi
run_dc -f other.yml ps
if [ "$RC" -eq 2 ] && [ ! -s "$D/calls" ]; then ok "dc.sh 的 compose 全域選項 → 擋下"; else ng "dc.sh -f：rc $RC"; fi

echo "== 四、靜態檢查"
for f in deploy.sh dc.sh scripts/test-deploy-sh.sh; do
  if bash -n "$REPO/$f"; then ok "bash -n $f"; else ng "bash -n $f"; fi
done
# 主機沒裝 shellcheck 時可以用 container 跑（2026-11-16：deploy.sh、dc.sh 0 則）：
#   docker run --rm -v "$PWD":/mnt:ro -w /mnt koalaman/shellcheck:v0.10.0 deploy.sh dc.sh scripts/test-deploy-sh.sh
if command -v shellcheck >/dev/null 2>&1; then
  if shellcheck "$REPO/deploy.sh" "$REPO/dc.sh"; then ok "shellcheck deploy.sh dc.sh"; else ng "shellcheck deploy.sh dc.sh"; fi
else
  echo "  skip shellcheck（沒有安裝）"
fi

echo "==> test-deploy-sh：通過 $PASS、失敗 $FAIL"
[ "$FAIL" -eq 0 ]
