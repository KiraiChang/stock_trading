#!/bin/bash
# deploy.sh 範本（I-132）：部署 stock_trading 的 backend、python-worker、python-server。
#
# 目錄的配置（跟 live 相同）：
#   * scripts 目錄：這支 deploy.sh、compose.yml（repo 的 docker-compose.yml 複製過去）、.env（.env.example 複製
#     過去，權限 600）。這支腳本只認自己所在的目錄。
#   * build 目錄（PROJECT_DIR）：專門給 build 用的 git checkout，⛔ 不在裡面改檔案。compose 的 project directory
#     指向它，build context（`context: .`）才會對。
#   * Redis stack 的 .env（REDIS_ENV_FILE）：只有一行 REDIS_PASSWORD，compose.yml 用 env_file 直接讀。
#
# 流程（任何一步失敗都回傳非 0，而且在 down 之前就停下；錯誤訊息⛔ 不帶機密）：
#   preflight（最後查回測的 pending／running）→ compose config（JWT）→ git pull --ff-only → 記憶體檢查
#   → build（逐一服務，舊服務還在跑）→ 確認新 image 認得 -check-config → 用新 image 檢查完整設定
#   → migration（T-087）：確認新 image 認得 -migrate → -migrate=status 讀 live 的狀態（唯讀）
#     → 有待套用的 migration 時，在拋棄式 PostgreSQL 上試跑（失敗就停下，原服務照常運作）
#   → 再查一次回測 → down
#   → 有待套用的 migration 時：備份 → 具名的一次性容器正式套用 → 停止確認 → 再讀一次狀態（pending 要是空的）
#   → up -d --no-build → readiness（/health 與 RestartCount，總上限 90 秒）
# migration 失敗時⛔ 不 up、⛔ 不自動 goose down：印出下一步，照部署手冊的復原分支處理。
# 手動做停止確認：./deploy.sh --confirm-migrator-stopped RUN_ID
# 回測（T-084）：backtest_jobs 有任何 pending 或 running 就停下；查不到（container 不在、逾時、權限、輸出格式不對）
# 也算失敗、停下。⛔ 不留跳過檢查的選項：真的要在有回測時部署，改用人工流程（手動 build、down、up）。
# `./deploy.sh --preflight` 只跑 preflight（含第一次回測檢查）與 JWT 檢查（⛔ 不 pull、不 build、不 down），換上新版之前試跑用。
# ⚠️ 部署失敗時⛔ 沒有自動回滾：只會回報失敗，由人判斷要不要換回前一版的 image。
# ⛔ 這裡不再 export 任何設定：應用設定在 .env、Redis 密碼在 Redis stack 的 .env，都由 compose 的 env_file 注入。
set -euo pipefail
# ⛔ 不讀 stdin（整支腳本都不需要輸入）：在終端機執行時，timeout 會把它底下的 docker 放進新的程序群組
# （對終端機來說是背景）；compose run／exec 預設會讀 stdin，一讀終端機就被 SIGTTIN 停住，等到逾時才結束。
# 2026-11-16 live 第一次部署就停在「確認新 image 支援 -check-config」：grep 本身沒問題，是 compose run 被停住、
# 120 秒後被 timeout 砍掉（dev 用 script 模擬終端機重現；readiness 的 compose exec 也會卡住，而且是在 down 之後）。
exec </dev/null

# ── 要填的值 ──────────────────────────────────────────────────
# build 用的 git checkout 的絕對路徑
PROJECT_DIR="/path/to/trading"
# 要部署的分支（build 目錄必須已經 checkout 在這個分支）
DEPLOY_BRANCH="main"
# Redis stack 的 .env；必須跟 compose.yml 裡 env_file 寫的路徑相同
REDIS_ENV_FILE="/path/to/redis-stack/.env"
# build 前 MemAvailable 的下限（MiB）。低於它就⛔ 不 build、⛔ 不 down：不停機 build 時，build 和正在跑的
# 服務會同時佔記憶體。依據：2026-11-16 在這台主機量的冷 cache build（記在 docs/development-workflow.md
# 「部署腳本：失敗即停」）。
MIN_MEM_AVAILABLE_MB=1536
# readiness 的總上限（秒）：從 up 之後開始算，包含每次請求本身花的時間
READY_TIMEOUT_SEC=90
# readiness 每次確認之間的間隔，與單次請求的上限（秒；單次上限還會再被剩餘時間截短）
READY_INTERVAL_SEC=2
READY_REQUEST_TIMEOUT_SEC=5
# 用新 image 檢查設定的上限（秒）：正常只要幾秒；超過就當成失敗（⛔ 不 down）
CHECK_CONFIG_TIMEOUT_SEC=120
# 查回測狀態用的 DB（T-084）：live 的 DB 在另一個 stack 的 container，用 docker exec 在 container 內以本機連線查（⛔ 不需要密碼）。
# 只允許英數與 _.-；佔位值沒換掉就停下（⛔ 不呼叫 docker）
DB_CONTAINER="your-postgres-container"
DB_USER="your-db-user"
DB_NAME="trading"
# 查回測狀態的硬逾時（秒）：docker inspect 與 docker exec 各自包 timeout -k 5（到期送 TERM，再過 5 秒還沒結束就 KILL）
BACKTEST_CHECK_TIMEOUT_SEC=15
# ── migration（T-087）──
# 停機前試跑用的 PostgreSQL image：要跟 live 的 PostgreSQL 同一個主版本，而且主機上已經有這個 image（⛔ 不在部署中拉 image）
TRIAL_PG_IMAGE="postgres:16-alpine"
# 停機中備份的目錄（權限必須是 700；有待套用的 migration 時才會寫入）
BACKUP_DIR="/path/to/db-backups"
# 正式套用的上限：每條連線的 lock_timeout 與 statement_timeout、整體期限（秒）
MIGRATE_LOCK_TIMEOUT="5s"
MIGRATE_STATEMENT_TIMEOUT="10m"
MIGRATE_TIMEOUT_SEC=900
# 外層 timeout（管的是 docker CLI，⛔ 不是容器）：比整體期限多 60 秒。到期之後一定先做停止確認
MIGRATE_OUTER_TIMEOUT_SEC=960
# 試跑每一步（匯出還原、status、up）與備份的上限（秒）
TRIAL_STEP_TIMEOUT_SEC=600
BACKUP_TIMEOUT_SEC=1800
# 停止確認：等 migrator 的連線自己結束最多幾秒、cancel 之後再等幾秒、每次確認的間隔
STOP_WAIT_SEC=60
CANCEL_WAIT_SEC=30
STOP_POLL_SEC=5

# ── 由腳本位置推出來的路徑（⛔ 不要改）────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STACK_COMPOSE_FILE="$SCRIPT_DIR/compose.yml"
APP_ENV_FILE="$SCRIPT_DIR/.env"
# 測試用（scripts/test-deploy-sh.sh）；正式使用⛔ 不要設
MEMINFO_FILE="${DEPLOY_MEMINFO_FILE:-/proc/meminfo}"

# 查回測狀態的 SQL（T-084）：只讀、一行兩個數字（pending|running）。⛔ 不要改：測試會從這裡取出字串、在拋棄式 PostgreSQL 上實測
BACKTEST_CHECK_SQL="SELECT count(*) FILTER (WHERE status = 'pending'), count(*) FILTER (WHERE status = 'running') FROM backtest_jobs"
BACKTEST_PGOPTIONS="-c default_transaction_read_only=on -c statement_timeout=5000"
# pg_dump 用的唯讀選項（⛔ 不帶 statement_timeout：pg_dump 自己會關掉它，大表要很久）
DUMP_PGOPTIONS="-c default_transaction_read_only=on"
# T-087：本次執行的識別（UTC 時間戳＋6 位亂數，同一秒啟動的兩次也⛔ 不會撞名）。正式 migrator 的容器名稱與 application_name、
# 試跑的容器與網路、備份檔名都用它
RUN_ID="${DEPLOY_RUN_ID:-$(date -u +%Y%m%dT%H%M%SZ)-$(od -An -N3 -tx1 /dev/urandom | tr -d ' \n')}"
# 實例識別（system_identifier|database|oid|postmaster 啟動時間）：跟 backend 的 database.InstanceSQL 逐字相同（Go 測試會比對）。
# migrator（compose 的 DSN）與這支腳本（DB_CONTAINER／DB_NAME）各查一次、逐字比對，確認備份、正式套用、停止確認都是同一個 DB
INSTANCE_SQL="SELECT s.system_identifier::text || '|' || current_database() || '|' || d.oid::text || '|' || to_char(pg_postmaster_start_time() AT TIME ZONE 'UTC', 'YYYYMMDDHH24MISSUS') FROM pg_control_system() s, pg_database d WHERE d.datname = current_database()"
# 部署互斥（T-087 review）：同一個 scripts 目錄（同一個部署目標）同時只能有一個 deploy.sh 在 pull 之後的步驟
LOCK_FILE="$SCRIPT_DIR/.deploy.lock"

# 跟 backend 設定契約的 PublicJWTSecrets 相同（backend/internal/config/contract.go；測試會比對）
PUBLIC_JWT_SECRETS="change-me-in-production change-me dev-only-change-me"

die() {
  echo "deploy.sh: $*" >&2
  exit 1
}

PREFLIGHT_ONLY=0
CONFIRM_ONLY=""
case "${1:-}" in
  "") [ $# -le 1 ] || die "參數太多（只有 --preflight 或 --confirm-migrator-stopped RUN_ID）" ;;
  --preflight) PREFLIGHT_ONLY=1; [ $# -le 1 ] || die "參數太多（只有 --preflight）" ;;
  --confirm-migrator-stopped)
    [ $# -eq 2 ] || die "用法：./deploy.sh --confirm-migrator-stopped RUN_ID"
    CONFIRM_ONLY="$2" ;;
  *) die "不認得的參數：$1（只有 --preflight 或 --confirm-migrator-stopped RUN_ID）" ;;
esac

COMPOSE=(docker compose -f "$STACK_COMPOSE_FILE" --project-directory "$PROJECT_DIR")
compose() {
  "${COMPOSE[@]}" "$@"
}

# check_backtests 階段 [補充說明]：查 backtest_jobs 的 pending／running（唯讀、有硬逾時）。兩個都是 0 才通過；
# 有任何一筆、或查不到（container 不在、逾時、其他錯誤、輸出格式不對）都停下。psql／docker 的原始錯誤⛔ 不顯示（stderr 丟掉）。
check_backtests() {
  local where="$1" extra="${2:-}" rc out running pending
  rc=0
  running="$(timeout -k 5 "$BACKTEST_CHECK_TIMEOUT_SEC" docker inspect -f '{{.State.Running}}' "$DB_CONTAINER" 2>/dev/null)" || rc=$?
  case "$rc" in
    0) ;;
    124|137) die "查回測狀態逾時（inspect，${BACKTEST_CHECK_TIMEOUT_SEC} 秒），停止部署（$where）$extra" ;;
    *) die "找不到在跑的 DB container DB_CONTAINER=$DB_CONTAINER，停止部署（$where）$extra" ;;
  esac
  [ "$running" = true ] || die "找不到在跑的 DB container DB_CONTAINER=$DB_CONTAINER，停止部署（$where）$extra"
  rc=0
  # 成功時在輸出後面補一個 #：$(…) 會去掉結尾所有的換行，補了才看得到原始輸出（例如 0|0 後面多一個空行）
  out="$(timeout -k 5 "$BACKTEST_CHECK_TIMEOUT_SEC" docker exec -e PGOPTIONS="$BACKTEST_PGOPTIONS" "$DB_CONTAINER" \
    psql -U "$DB_USER" -d "$DB_NAME" -X -v ON_ERROR_STOP=1 -qtAc "$BACKTEST_CHECK_SQL" 2>/dev/null && printf '#')" || rc=$?
  case "$rc" in
    0) ;;
    124|137) die "查回測狀態逾時（exec，${BACKTEST_CHECK_TIMEOUT_SEC} 秒），停止部署（$where）$extra" ;;
    *) die "查回測狀態失敗（結束碼 $rc；⛔ 不顯示原始錯誤，請手動用唯讀查詢確認），停止部署（$where）$extra" ;;
  esac
  out="${out%#}"
  # 原始輸出必須剛好一行「數字|數字」加一個換行；多行、空行、其他文字都⛔ 不符合
  [[ "$out" =~ ^([0-9]+)[|]([0-9]+)$'\n'$ ]] || die "查回測狀態的輸出格式不對，停止部署（$where）$extra"
  pending="${BASH_REMATCH[1]}"
  running="${BASH_REMATCH[2]}"
  # 只有兩個都剛好是字串 0 才放行；⛔ 不做整數比較（超出 Bash 整數範圍的數字，比較會出錯而被當成「不大於 0」）
  if [ "$pending" != 0 ] || [ "$running" != 0 ]; then
    die "backtest_jobs 有 pending $pending、running $running，等回測結束再部署（⛔ 不留跳過的選項），停止部署（$where）$extra"
  fi
  echo "==> 回測：pending 0、running 0"
}

# ── migration 的函式（T-087；規格見 docs/development-workflow.md「部署腳本：失敗即停」）──

# status_field 輸出 前綴：-migrate 的輸出裡「migrate: 前綴 值」要剛好一行，印出值；沒有或不只一行就回 1
status_field() {
  local n
  n="$(printf '%s\n' "$1" | grep -c "^migrate: $2 ")" || true
  [ "$n" = 1 ] || return 1
  printf '%s\n' "$1" | sed -n "s/^migrate: $2 //p"
}
valid_fp() { [[ "$1" =~ ^[0-9a-f]{64}$ ]]; }
valid_pending() { [[ "$1" =~ ^(-|[0-9]+(,[0-9]+)*)$ ]]; }

# container_state 名稱：印出容器的狀態（不存在就印空字串）；docker 沒回應就回非 0
container_state() {
  timeout -k 5 15 docker ps -a --filter "name=^/$1\$" --format '{{.State}}' 2>/dev/null
}

# migrator_sessions：部署目標的 database 上，application_name 是本次 migrator 的連線數（唯讀；只回筆數）
migrator_sessions() {
  local out
  out="$(timeout -k 5 "$BACKTEST_CHECK_TIMEOUT_SEC" docker exec -e PGOPTIONS="$BACKTEST_PGOPTIONS" "$DB_CONTAINER" \
    psql -U "$DB_USER" -d "$DB_NAME" -X -v ON_ERROR_STOP=1 -qtAc \
    "SELECT count(*) FROM pg_stat_activity WHERE application_name = '$1' AND datname = '$DB_NAME'" 2>/dev/null && printf '#')" || return 1
  out="${out%#}"
  [[ "$out" =~ ^([0-9]+)$'\n'$ ]] || return 1
  printf '%s\n' "${BASH_REMATCH[1]}"
}

# signal_migrator_sessions 函式名稱 application_name：只對本次 migrator、部署目標 database 的 pid 呼叫 pg_cancel_backend／pg_terminate_backend
signal_migrator_sessions() {
  timeout -k 5 "$BACKTEST_CHECK_TIMEOUT_SEC" docker exec "$DB_CONTAINER" \
    psql -U "$DB_USER" -d "$DB_NAME" -X -v ON_ERROR_STOP=1 -qtAc \
    "SELECT count($1(pid)) FROM pg_stat_activity WHERE application_name = '$2' AND datname = '$DB_NAME'" >/dev/null 2>&1
}

# wait_sessions_zero application_name 秒數：每 STOP_POLL_SEC 秒確認一次，歸零就印 0；時間到了印最後的筆數；查不到回 1
wait_sessions_zero() {
  local n waited=0
  n="$(migrator_sessions "$1")" || return 1
  while [ "$n" != 0 ] && [ "$waited" -lt "$2" ]; do
    sleep "$STOP_POLL_SEC"
    waited=$((waited + STOP_POLL_SEC))
    n="$(migrator_sessions "$1")" || return 1
  done
  printf '%s\n' "$n"
}

# confirm_migrator_stopped RUN_ID：正式 migrator 的停止確認（成功、失敗、逾時都做；任何復原動作之前也要先做）。
#   1. 容器：還在跑就 docker stop -t 30，再不停就 docker kill；最後要不存在或已退出。
#   2. DB：本次 application_name 在部署目標 database 的連線數；不是 0 → 等 → pg_cancel_backend → 等 → pg_terminate_backend → 要是 0。
#   ⛔ 不對其他 application_name、其他 database 的連線做任何事。任何一步確認不了就停下、⛔ 不往下做任何復原動作。
confirm_migrator_stopped() {
  local id="$1" name app st step n
  name="stock_trading-migrator-$id"
  app="trading-migrator-$id"
  local manual="需要人工確認 migrator 已停止（容器 $name、application_name $app）；確認之前⛔ 不做任何復原動作"
  echo "==> 停止確認：容器 $name、application_name $app"
  for step in check stop kill; do
    st="$(container_state "$name")" || die "查不到容器狀態（docker 沒有回應），$manual"
    case "$st" in ""|exited|dead) break ;; esac
    case "$step" in
      check) echo "==> 容器還在（$st）：docker stop -t 30"; timeout -k 5 45 docker stop -t 30 "$name" >/dev/null 2>&1 || true ;;
      stop) echo "==> 容器還在（$st）：docker kill"; timeout -k 5 15 docker kill "$name" >/dev/null 2>&1 || true ;;
      kill) die "容器 kill 之後還在（$st），$manual" ;;
    esac
  done
  n="$(wait_sessions_zero "$app" "$STOP_WAIT_SEC")" || die "查不到 migrator 的連線數，$manual"
  if [ "$n" != 0 ]; then
    echo "==> $DB_NAME 上還有 $n 個 $app 的連線：pg_cancel_backend"
    signal_migrator_sessions pg_cancel_backend "$app" || die "pg_cancel_backend 失敗，$manual"
    n="$(wait_sessions_zero "$app" "$CANCEL_WAIT_SEC")" || die "查不到 migrator 的連線數，$manual"
  fi
  if [ "$n" != 0 ]; then
    echo "==> 還有 $n 個：pg_terminate_backend"
    signal_migrator_sessions pg_terminate_backend "$app" || die "pg_terminate_backend 失敗，$manual"
    n="$(wait_sessions_zero "$app" "$STOP_POLL_SEC")" || die "查不到 migrator 的連線數，$manual"
  fi
  [ "$n" = 0 ] || die "terminate 之後還有 $n 個連線，$manual"
  echo "==> 停止確認通過：容器已退出、$DB_NAME 上⛔ 沒有 $app 的連線"
}

# 試跑用的資源（名稱固定帶 RUN_ID）：成功、失敗都刪，刪完確認三者都不存在
TRIAL_NET="stock_trading-trial-$RUN_ID"
TRIAL_PG="stock_trading-trialpg-$RUN_ID"
TRIAL_MIG="stock_trading-trialmig-$RUN_ID"
# 本次建立的資源（T-087 review：只清理本次建立的）。run_trial 開頭先確認三個名稱都⛔ 不存在，之後才註冊 trap、才開始建立；
# 名稱帶本次的 RUN_ID、而且有部署鎖，所以確認之後出現的同名資源就是這一次建立的（建立到一半失敗的也算）。
TRIAL_NET_MADE=0
TRIAL_PG_MADE=0
TRIAL_MIG_MADE=0
trial_cleanup() {
  local left="" c out rm_list=()
  [ "$TRIAL_MIG_MADE" = 1 ] && rm_list+=("$TRIAL_MIG")
  [ "$TRIAL_PG_MADE" = 1 ] && rm_list+=("$TRIAL_PG")
  if [ "${#rm_list[@]}" -gt 0 ]; then
    timeout -k 5 60 docker rm -fv "${rm_list[@]}" >/dev/null 2>&1 || true
  fi
  if [ "$TRIAL_NET_MADE" = 1 ]; then
    timeout -k 5 30 docker network rm "$TRIAL_NET" >/dev/null 2>&1 || true
  fi
  for c in ${rm_list[@]+"${rm_list[@]}"}; do
    if ! out="$(container_state "$c")"; then left="$left $c（查不到）"; elif [ -n "$out" ]; then left="$left $c"; fi
  done
  if [ "$TRIAL_NET_MADE" = 1 ]; then
    if ! out="$(timeout -k 5 15 docker network ls -q --filter "name=^$TRIAL_NET\$" 2>/dev/null)"; then
      left="$left $TRIAL_NET（查不到）"
    elif [ -n "$out" ]; then
      left="$left $TRIAL_NET"
    fi
  fi
  if [ -n "$left" ]; then
    echo "deploy.sh: ⚠️ 試跑的資源沒有刪乾淨：$left（請手動 docker rm -fv／docker network rm）" >&2
    return 1
  fi
  echo "==> 試跑的 container 與網路已刪除"
}
# shellcheck disable=SC2317  # 由 trap EXIT 呼叫
trial_exit_trap() {
  local rc=$?
  trial_cleanup || true
  exit "$rc"
}

# run_trial：停機前試跑（D5）。live 的 schema 與 goose_db_version 的紀錄匯到拋棄式 PostgreSQL（獨立的 internal 網路、⛔ 不開 port、
# 結束就刪），用新 image 在上面跑 -migrate=up。失敗就停下（還沒 down、原服務照常運作）。
run_trial() {
  local stop="停止部署（還沒 down，舊服務照常運作）" dsn out rc tfp tpend i pw c thost tport tuser tdb tssl
  echo "==> 停機前試跑：網路 $TRIAL_NET、PostgreSQL $TRIAL_PG（$TRIAL_PG_IMAGE）"
  # 先確認三個名稱都不存在（存在就停下、⛔ 不清理——那不是這一次建立的）
  for c in "$TRIAL_MIG" "$TRIAL_PG"; do
    out="$(container_state "$c")" || die "查不到試跑容器的狀態（docker 沒有回應），$stop"
    [ -z "$out" ] || die "試跑容器 $c 已經存在（⛔ 不是這一次建立的，⛔ 不清理），$stop"
  done
  out="$(timeout -k 5 15 docker network ls -q --filter "name=^$TRIAL_NET\$" 2>/dev/null)" || die "查不到 docker 網路（docker 沒有回應），$stop"
  [ -z "$out" ] || die "試跑網路 $TRIAL_NET 已經存在（⛔ 不是這一次建立的，⛔ 不清理），$stop"
  trap trial_exit_trap EXIT
  TRIAL_NET_MADE=1
  timeout -k 5 30 docker network create --internal "$TRIAL_NET" >/dev/null 2>&1 || die "建立試跑網路失敗，$stop"
  # 臨時帳密（原裁決）：密碼是這一次產生的亂數，只經環境變數傳給 docker——在子 shell 裡 export（shell 內建、⛔ 不產生程序）
  # 再呼叫 timeout／docker，⛔ 不用 `env KEY=值`（會留在 env 與 timeout 的程序參數裡，/proc 讀得到；T-087 review）。⛔ 不印出來。
  # --internal 網路只擋對外的連線，主機上的程序仍連得到容器，所以⛔ 不用 trust。
  pw="$(od -An -N16 -tx1 /dev/urandom | tr -d ' \n')"
  [ "${#pw}" = 32 ] || die "產生試跑的臨時密碼失敗，$stop"
  TRIAL_PG_MADE=1
  if ! ( export POSTGRES_PASSWORD="$pw"
      timeout -k 5 60 docker run -d --name "$TRIAL_PG" --network "$TRIAL_NET" -e POSTGRES_USER=trial -e POSTGRES_DB=trial \
        -e POSTGRES_PASSWORD "$TRIAL_PG_IMAGE" >/dev/null 2>&1 ); then
    die "啟動試跑用 PostgreSQL 失敗（TRIAL_PG_IMAGE=$TRIAL_PG_IMAGE；主機上要先有這個 image），$stop"
  fi
  # 用 TCP 確認（初始化期間的暫時伺服器只聽 unix socket，用 socket 會太早判成 ready）
  for i in $(seq 1 60); do
    if timeout -k 5 10 docker exec "$TRIAL_PG" pg_isready -q -h 127.0.0.1 -U trial -d trial >/dev/null 2>&1; then break; fi
    [ "$i" -lt 60 ] || die "試跑用 PostgreSQL 60 秒內沒有 ready，$stop"
    sleep 1
  done
  # live 的 schema 與版本紀錄（唯讀匯出、直接接到拋棄式 container 還原，⛔ 不落地成檔案）
  if ! timeout -k 5 "$TRIAL_STEP_TIMEOUT_SEC" docker exec -e PGOPTIONS="$DUMP_PGOPTIONS" "$DB_CONTAINER" \
      pg_dump -U "$DB_USER" -d "$DB_NAME" --schema-only --no-owner --no-acl 2>/dev/null \
      | timeout -k 5 "$TRIAL_STEP_TIMEOUT_SEC" docker exec -i "$TRIAL_PG" psql -X -q -v ON_ERROR_STOP=1 -U trial -d trial >/dev/null 2>&1; then
    die "試跑：匯出或還原 live 的 schema 失敗（⛔ 不顯示原始錯誤），$stop"
  fi
  if ! timeout -k 5 "$TRIAL_STEP_TIMEOUT_SEC" docker exec -e PGOPTIONS="$DUMP_PGOPTIONS" "$DB_CONTAINER" \
      pg_dump -U "$DB_USER" -d "$DB_NAME" --data-only --no-owner --no-acl -t public.goose_db_version 2>/dev/null \
      | timeout -k 5 "$TRIAL_STEP_TIMEOUT_SEC" docker exec -i "$TRIAL_PG" psql -X -q -v ON_ERROR_STOP=1 -U trial -d trial >/dev/null 2>&1; then
    die "試跑：匯出或還原 goose_db_version 的紀錄失敗，$stop"
  fi
  timeout -k 5 30 docker exec "$TRIAL_PG" psql -X -q -v ON_ERROR_STOP=1 -U trial -d trial -c "CREATE TABLE t087_trial_target (id int)" >/dev/null 2>&1 \
    || die "試跑：建立目標標記表失敗，$stop"
  # 試跑的 migrator 只帶臨時庫的 DSN；先用**同一組連線參數**確認標記表存在（DSN 指錯就停下）。DSN 由下面這組參數組成；
  # 標記表的確認用 libpq 的 PG* 環境變數（⛔ 不把 DSN 放進容器裡 psql 的程序參數），migrator 用 DATABASE_DSN 環境變數。
  local thost="$TRIAL_PG" tport=5432 tuser=trial tdb=trial tssl=disable
  dsn="postgres://$tuser:$pw@$thost:$tport/$tdb?sslmode=$tssl"
  out="$( export PGHOST="$thost" PGPORT="$tport" PGUSER="$tuser" PGPASSWORD="$pw" PGDATABASE="$tdb" PGSSLMODE="$tssl"
    timeout -k 5 60 docker run --rm --network "$TRIAL_NET" -e PGHOST -e PGPORT -e PGUSER -e PGPASSWORD -e PGDATABASE -e PGSSLMODE \
      "$TRIAL_PG_IMAGE" psql -X -qtAc "SELECT to_regclass('t087_trial_target') IS NOT NULL" 2>/dev/null )" || out=""
  [ "$out" = t ] || die "試跑：用試跑的 DSN 找不到目標標記表 t087_trial_target（DSN 指錯？），$stop"
  TRIAL_MIG_MADE=1
  rc=0
  # shellcheck disable=SC2030,SC2031  # 刻意只在子 shell 裡 export（DSN 含臨時密碼，⛔ 不留在這支腳本的環境）
  out="$( export DATABASE_DSN="$dsn"
    timeout -k 5 "$TRIAL_STEP_TIMEOUT_SEC" docker run --rm --name "$TRIAL_MIG" --network "$TRIAL_NET" -e DATABASE_DRIVER=postgres -e DATABASE_DSN \
      "$NEW_IMAGE" ./trading-backend -migrate=status 2>/dev/null )" || rc=$?
  [ "$rc" = 0 ] || die "試跑：臨時庫的 -migrate=status 失敗（結束碼 $rc），$stop"
  tfp="$(status_field "$out" fingerprint)" || tfp=""
  tpend="$(status_field "$out" pending)" || tpend=""
  if ! { [ "$tfp" = "$BASE_FP" ] && [ "$tpend" = "$BASE_PENDING" ]; }; then
    die "試跑：臨時庫的版本紀錄跟 live 不同（fingerprint ${tfp:0:12}…／${BASE_FP:0:12}…、pending $tpend／$BASE_PENDING；匯出期間有變動？），$stop"
  fi
  rc=0
  # shellcheck disable=SC2030,SC2031  # 同上
  out="$( export DATABASE_DSN="$dsn"
    timeout -k 5 "$TRIAL_STEP_TIMEOUT_SEC" docker run --rm --name "$TRIAL_MIG" --network "$TRIAL_NET" -e DATABASE_DRIVER=postgres -e DATABASE_DSN \
      "$NEW_IMAGE" ./trading-backend -migrate=up -run-id="trial-$RUN_ID" -expect-fingerprint="$BASE_FP" \
      -migrate-lock-timeout="$MIGRATE_LOCK_TIMEOUT" -migrate-statement-timeout="$MIGRATE_STATEMENT_TIMEOUT" -migrate-timeout="${MIGRATE_TIMEOUT_SEC}s" 2>/dev/null )" || rc=$?
  printf '%s\n' "$out" | grep '^migrate: step ' | sed 's/^/  試跑 /' || true
  tpend="$(status_field "$out" "after pending")" || tpend=""
  if ! { [ "$rc" = 0 ] && [ "$tpend" = "-" ]; }; then die "試跑：-migrate=up 沒有通過（結束碼 $rc、之後的 pending ${tpend:-?}），$stop"; fi
  trap - EXIT
  trial_cleanup || die "試跑通過，但試跑的資源沒有刪乾淨，$stop"
  echo "==> 停機前試跑通過：$BASE_PENDING"
}

# backup_db：停機中的備份（D6）。pg_dump -Fc 到 BACKUP_DIR（700），pg_restore -l 確認檔案讀得出來（能不能還原由演練驗證）
# 呼叫端是 `backup_db || die`，set -e 在這裡⛔ 不生效：每一步都要明確檢查。檔名帶本次的 RUN_ID；**原子地拒絕覆寫**（T-087 review）：
#   * .partial 用 noclobber（set -C，O_EXCL）建立：已經存在就失敗、⛔ 不動它；
#   * 正式檔名用 ln -T（hard link；目的檔已經存在時 link(2) 一定失敗、⛔ 不覆寫），⛔ 不用 mv（rename 會覆寫 pg_dump 期間才出現的同名檔）；
#     -T 讓目的路徑是目錄或指向目錄的符號連結時也直接失敗（⛔ 不會把連結建到那個目錄裡；T-087 review）；
#     連結之後確認正式檔就是這次的 .partial（同一個 inode），才刪掉 .partial。
backup_db() {
  local f
  f="$BACKUP_DIR/trading_${RUN_ID}_${BASE_FP:0:12}.dump"
  echo "==> 備份 live DB：$f"
  if [ -e "$f" ] || [ -e "$f.partial" ]; then
    echo "deploy.sh: 備份檔 $f（或 .partial）已經存在，⛔ 不覆蓋" >&2
    return 1
  fi
  if ! ( set -C; umask 077; : > "$f.partial" ) 2>/dev/null; then
    echo "deploy.sh: 備份檔 $f.partial 已經存在或建立不了，⛔ 不覆寫" >&2
    return 1
  fi
  if ! timeout -k 10 "$BACKUP_TIMEOUT_SEC" docker exec -e PGOPTIONS="$DUMP_PGOPTIONS" "$DB_CONTAINER" \
      pg_dump -U "$DB_USER" -d "$DB_NAME" -Fc 2>/dev/null >> "$f.partial"; then
    rm -f -- "$f.partial"
    return 1
  fi
  if [ ! -s "$f.partial" ]; then
    rm -f -- "$f.partial"
    return 1
  fi
  if ! ln -T -- "$f.partial" "$f" 2>/dev/null; then
    echo "deploy.sh: 備份檔 $f 已經存在（pg_dump 期間出現？）或連結失敗，⛔ 不覆寫" >&2
    rm -f -- "$f.partial"
    return 1
  fi
  if ! [ "$f" -ef "$f.partial" ]; then
    echo "deploy.sh: 備份檔 $f 不是這次寫的那一份" >&2
    rm -f -- "$f.partial"
    return 1
  fi
  if ! rm -f -- "$f.partial" || [ -e "$f.partial" ] || [ ! -s "$f" ]; then
    echo "deploy.sh: 整理備份檔失敗（$f）" >&2
    return 1
  fi
  if ! timeout -k 5 300 docker exec -i "$DB_CONTAINER" pg_restore -l < "$f" >/dev/null 2>&1; then
    echo "deploy.sh: 備份檔讀不出來（pg_restore -l）" >&2
    return 1
  fi
  BACKUP_FILE="$f"
  echo "==> 備份完成：$(stat -c %s "$f") bytes，pg_restore -l 讀得出來"
}

# script_instance：用 DB_CONTAINER／DB_NAME 查實例識別（唯讀；跟 migrator 的 -migrate=status 印的 instance 逐字比對）
script_instance() {
  local out
  out="$(timeout -k 5 "$BACKTEST_CHECK_TIMEOUT_SEC" docker exec -e PGOPTIONS="$BACKTEST_PGOPTIONS" "$DB_CONTAINER" \
    psql -U "$DB_USER" -d "$DB_NAME" -X -v ON_ERROR_STOP=1 -qtAc "$INSTANCE_SQL" 2>/dev/null && printf '#')" || return 1
  out="${out%#}"
  [[ "$out" =~ ^([0-9]+[|][A-Za-z0-9_.-]+[|][0-9]+[|][0-9]{20})$'\n'$ ]] || return 1
  printf '%s\n' "${BASH_REMATCH[1]}"
}
valid_instance() { [[ "$1" =~ ^[0-9]+[|][A-Za-z0-9_.-]+[|][0-9]+[|][0-9]{20}$ ]]; }

# ── 1. preflight（都在 git pull 之前）────────────────────────
echo "==> preflight"
# 查回測用的 DB 設定：佔位值或空字串、不允許的字元 → 停下（⛔ 不呼叫 docker；值會拼進 docker 的參數）
for kv in "DB_CONTAINER=$DB_CONTAINER" "DB_USER=$DB_USER" "DB_NAME=$DB_NAME"; do
  k="${kv%%=*}"
  v="${kv#*=}"
  case "$v" in
    ""|your-postgres-container|your-db-user) die "先填 $k（deploy.sh 開頭的「要填的值」），停止部署（還沒 pull、還沒 down）" ;;
  esac
  [[ "$v" =~ ^[A-Za-z0-9_.-]+$ ]] || die "$k 只能用英數與 _.-，停止部署（還沒 pull、還沒 down）"
done
[[ "$BACKTEST_CHECK_TIMEOUT_SEC" =~ ^[1-9][0-9]*$ ]] || die "BACKTEST_CHECK_TIMEOUT_SEC 要是正整數，停止部署（還沒 pull、還沒 down）"
for kv in "MIGRATE_TIMEOUT_SEC=$MIGRATE_TIMEOUT_SEC" "MIGRATE_OUTER_TIMEOUT_SEC=$MIGRATE_OUTER_TIMEOUT_SEC" "TRIAL_STEP_TIMEOUT_SEC=$TRIAL_STEP_TIMEOUT_SEC" \
  "BACKUP_TIMEOUT_SEC=$BACKUP_TIMEOUT_SEC" "STOP_WAIT_SEC=$STOP_WAIT_SEC" "CANCEL_WAIT_SEC=$CANCEL_WAIT_SEC" "STOP_POLL_SEC=$STOP_POLL_SEC"; do
  [[ "${kv#*=}" =~ ^[1-9][0-9]*$ ]] || die "${kv%%=*} 要是正整數，停止部署（還沒 pull、還沒 down）"
done
if ! { [[ "$MIGRATE_LOCK_TIMEOUT" =~ ^[1-9][0-9]*(ms|s|m)$ ]] && [[ "$MIGRATE_STATEMENT_TIMEOUT" =~ ^[1-9][0-9]*(ms|s|m|h)$ ]]; }; then
  die "MIGRATE_LOCK_TIMEOUT／MIGRATE_STATEMENT_TIMEOUT 的格式不對（例如 5s、10m），停止部署（還沒 pull、還沒 down）"
fi
[[ "$TRIAL_PG_IMAGE" =~ ^[A-Za-z0-9_.:/@-]+$ ]] || die "TRIAL_PG_IMAGE 的格式不對，停止部署（還沒 pull、還沒 down）"
[[ "$RUN_ID" =~ ^[A-Za-z0-9_.-]{1,30}$ ]] || die "RUN_ID 的格式不對，停止部署（還沒 pull、還沒 down）"
if [ -n "$CONFIRM_ONLY" ]; then
  [[ "$CONFIRM_ONLY" =~ ^[A-Za-z0-9_.-]{1,30}$ ]] || die "RUN_ID 的格式不對"
  command -v timeout >/dev/null 2>&1 || die "找不到 timeout 指令（coreutils）"
  confirm_migrator_stopped "$CONFIRM_ONLY"
  exit 0
fi
case "$BACKUP_DIR" in /path/to/*|"") die "先填 BACKUP_DIR（deploy.sh 開頭的「要填的值」），停止部署（還沒 pull、還沒 down）" ;; esac
if ! { [ -d "$BACKUP_DIR" ] && [ -w "$BACKUP_DIR" ]; }; then die "BACKUP_DIR=$BACKUP_DIR 不存在或不能寫入，停止部署（還沒 pull、還沒 down）"; fi
[ "$(stat -c %a "$BACKUP_DIR")" = 700 ] || die "BACKUP_DIR 的權限必須是 700，停止部署（還沒 pull、還沒 down）"
# readiness 用 timeout 限制每次請求（coreutils；⛔ 不能缺，否則一個卡住的請求就沒有上限）
command -v timeout >/dev/null 2>&1 || die "找不到 timeout 指令（coreutils），停止部署（還沒 down）"
command -v flock >/dev/null 2>&1 || die "找不到 flock 指令（util-linux；部署互斥要用），停止部署（還沒 down）"
if ! git -C "$PROJECT_DIR" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  die "$PROJECT_DIR 不是 git checkout，停止部署（還沒 down）"
fi
if ! branch="$(git -C "$PROJECT_DIR" rev-parse --abbrev-ref HEAD)"; then
  die "讀不到 $PROJECT_DIR 的目前分支，停止部署（還沒 down）"
fi
if [ "$branch" != "$DEPLOY_BRANCH" ]; then
  die "$PROJECT_DIR 在分支 $branch，不是 $DEPLOY_BRANCH，停止部署（還沒 down）"
fi
# 只看追蹤中的檔案有沒有本機修改；沒有追蹤的檔案如果會跟 pull 衝突，pull 本身會失敗
if ! dirty="$(git -C "$PROJECT_DIR" status --porcelain --untracked-files=no)"; then
  die "讀不到 $PROJECT_DIR 的 git status，停止部署（還沒 down）"
fi
if [ -n "$dirty" ]; then
  die "$PROJECT_DIR 有本機修改（build 目錄⛔ 不能改檔案），停止部署（還沒 down）"
fi

[ -f "$STACK_COMPOSE_FILE" ] || die "找不到 $STACK_COMPOSE_FILE，停止部署（還沒 down）"

if [ ! -f "$APP_ENV_FILE" ] || [ ! -r "$APP_ENV_FILE" ]; then
  die "讀不到 $APP_ENV_FILE，停止部署（還沒 down）"
fi
mode="$(stat -c %a "$APP_ENV_FILE")"
if [ "$mode" != "600" ]; then
  die "$APP_ENV_FILE 的權限是 $mode，必須是 600，停止部署（還沒 down）"
fi

# compose.yml 裡 backend 的 env_file 要剛好指到這兩個檔（只比對路徑字串，⛔ 不讀檔案內容）。
# 認得兩種寫法：`- path: /abs/file` 與 `- /abs/file`；行尾的 # 註解忽略。
envfiles="$(awk '
  /^  [^ #]/ { in_backend = ($0 ~ /^  backend:[ \t]*$/); in_ef = 0; next }
  in_backend && /^    [^ #]/ { in_ef = ($0 ~ /^    env_file:[ \t]*$/); next }
  in_backend && in_ef && /^      - / {
    line = $0
    sub(/[ \t]+#.*$/, "", line)
    sub(/^      - (path:[ \t]*)?/, "", line)
    gsub(/^["\047]|["\047][ \t]*$/, "", line)
    sub(/[ \t]+$/, "", line)
    print line
  }' "$STACK_COMPOSE_FILE")"
for f in "$APP_ENV_FILE" "$REDIS_ENV_FILE"; do
  if ! printf '%s\n' "$envfiles" | grep -qxF -- "$f"; then
    die "$STACK_COMPOSE_FILE 的 backend env_file 沒有 $f（路徑要跟 deploy.sh 相同），停止部署（還沒 down）"
  fi
done

# Redis stack 的 .env 走白名單，⛔ 不模仿 compose 的解析：compose 的 .env 語法很多（export、= 前後的空白、
# 引號、$ 展開…），只認得其中幾種、其他行忽略，就會漏掉沒認出來的寫法。所以只允許三種行：空行、從第一個字元
# 就是 # 的註解、剛好一行 REDIS_PASSWORD=值（值只用英數與 ._+/=-，這種寫法 compose 照字面讀）。
# 這裡只檢查、⛔ 不讀出值（compose 自己會讀）。
if awk '
  /^[ \t]*$/ || /^#/ { next }
  /^REDIS_PASSWORD=[A-Za-z0-9._+\/=-]+$/ { n++; next }
  { bad++ }
  END { if (bad || n != 1) exit 3 }' "$REDIS_ENV_FILE" 2>/dev/null; then rc=0; else rc=$?; fi
case $rc in
  0) ;;
  3) die "$REDIS_ENV_FILE 只能有一行 REDIS_PASSWORD=值（值只用英數與 ._+/=-），加上空行與 # 開頭的註解，停止部署（還沒 down）" ;;
  *) die "讀不到 $REDIS_ENV_FILE，停止部署（還沒 down）" ;;
esac

# 回測的 pending／running（第一次）：preflight 的最後、JWT 檢查之前，所以 --preflight 也會跑到。最早擋下，什麼都還沒動。
check_backtests "還沒 pull、還沒 down"

# ── 2. 最終設定裡的 JWT（在 git pull 之前就能先擋）──────────
# 用跟部署相同的參數跑 compose config，**在記憶體裡**用 awk 取出 backend 最終的 AUTH_JWT_SECRET。
# ⛔ 不把 config 寫到檔案或印到畫面；config 的錯誤訊息也⛔ 不顯示（可能帶出 env_file 的內容）。
# 完整的設定契約由第 5 步用新 image 檢查，這一步只涵蓋 JWT。設定都在 scripts 目錄（compose.yml 與兩個
# env_file），跟 build 目錄 pull 到什麼無關，所以放在 pull 之前。
echo "==> 檢查最終設定的 AUTH_JWT_SECRET"
set +e
compose config 2>/dev/null | awk -v public="$PUBLIC_JWT_SECRETS" '
  BEGIN { n = split(public, p, " "); for (i = 1; i <= n; i++) bad[p[i]] = 1 }
  /^  [^ #]/ { in_backend = ($0 ~ /^  backend:[ \t]*$/); in_env = 0; next }
  in_backend && /^    [^ #]/ { in_env = ($0 ~ /^    environment:[ \t]*$/); next }
  in_backend && in_env && /^      AUTH_JWT_SECRET:/ {
    v = $0
    sub(/^      AUTH_JWT_SECRET:[ \t]*/, "", v)
    if (v ~ /^".*"$/ || v ~ /^\047.*\047$/) v = substr(v, 2, length(v) - 2)
    found = 1
  }
  END {
    if (!found || v == "") exit 3
    if (v in bad) exit 4
  }'
st=("${PIPESTATUS[@]}")
set -e
if [ "${st[0]}" -ne 0 ]; then
  die "docker compose config 失敗（為了不帶出機密，⛔ 不顯示原始錯誤；請檢查 compose.yml 與兩個 env_file），停止部署（還沒 down）"
fi
case "${st[1]}" in
  0) ;;
  3) die "backend 最終的 AUTH_JWT_SECRET 是空值，停止部署（還沒 down）" ;;
  4) die "backend 最終的 AUTH_JWT_SECRET 是公開的範例值，停止部署（還沒 down）" ;;
  *) die "檢查 AUTH_JWT_SECRET 時發生錯誤，停止部署（還沒 down）" ;;
esac

# --preflight：只跑上面兩步（含第一次回測檢查，加上讀 MemAvailable），⛔ 不 pull、⛔ 不 build、⛔ 不 down。
# 用途：換上新的 deploy.sh 之前，先用暫存檔名放在 scripts 目錄試跑（SCRIPT_DIR 由腳本位置決定）。
if [ "$PREFLIGHT_ONLY" = 1 ]; then
  avail="$(awk '/^MemAvailable:/ { print int($2 / 1024) }' "$MEMINFO_FILE" 2>/dev/null || true)"
  echo "==> preflight 通過（⛔ 沒有 pull、build、down）；MemAvailable ${avail:-?}MiB，build 的下限 ${MIN_MEM_AVAILABLE_MB}MiB"
  exit 0
fi

# ── 3. 更新 build 目錄 ────────────────────────────────────────
# 部署互斥（T-087 review）：pull／build 之前取得同一個部署目標（scripts 目錄）的鎖；拿不到就停下（⛔ 不等）。鎖跟著程序，結束就放掉
exec 9>>"$LOCK_FILE" || die "開不了鎖檔 $LOCK_FILE，停止部署（還沒 pull、還沒 down）"
flock -n 9 || die "另一個 deploy.sh 正在部署同一個目標（$LOCK_FILE），停止部署（還沒 pull、還沒 down）"
echo "==> 部署鎖：$LOCK_FILE（run_id $RUN_ID）"
echo "==> git pull --ff-only origin $DEPLOY_BRANCH"
if ! git -C "$PROJECT_DIR" pull --ff-only origin "$DEPLOY_BRANCH"; then
  die "git pull 失敗（只允許 fast-forward），停止部署（還沒 down）"
fi

# ── 4. build（舊的服務還在跑）────────────────────────────────
# 一次 build 一個服務（避免兩個 image 的記憶體峰值疊加），每個服務 build 之前都檢查 MemAvailable。
if ! services="$(compose config --services 2>/dev/null)"; then
  die "列不出 compose.yml 的服務，停止部署（還沒 down）"
fi
[ -n "$services" ] || die "compose.yml 沒有任何服務，停止部署（還沒 down）"
for svc in $services; do
  if ! avail="$(awk '/^MemAvailable:/ { print int($2 / 1024); found = 1 } END { if (!found) exit 1 }' "$MEMINFO_FILE")"; then
    die "讀不到 $MEMINFO_FILE 的 MemAvailable，停止部署（還沒 build $svc、還沒 down）"
  fi
  if [ "$avail" -lt "$MIN_MEM_AVAILABLE_MB" ]; then
    die "MemAvailable ${avail}MiB 低於 ${MIN_MEM_AVAILABLE_MB}MiB，⛔ 不在服務運作中 build（還沒 build $svc、還沒 down）。請改在低負載時段，或改用先 down 再 build 的舊順序"
  fi
  echo "==> build $svc（MemAvailable ${avail}MiB）"
  if ! compose build "$svc"; then
    die "build $svc 失敗，停止部署（還沒 down，舊服務照常運作）"
  fi
done

# ── 5. 用新 image 檢查完整設定（backend 的設定契約）─────────
# ⚠️ 一定要寫出執行檔：Dockerfile 只有 CMD、沒有 ENTRYPOINT，compose run 後面的參數會整個取代 CMD。
# 只載入設定、跑契約檢查，⛔ 不連 DB 或任何外部服務；輸出只有鍵名和規則代號。
# ⛔ 先確認新 image 的 backend 認得 -check-config：I-132 E 之前的 binary ⛔ 不解析參數，會忽略旗標、
# 直接啟動一個完整的 backend（連 live DB、回收執行中的 job_runs、啟動排程器）。所以先用 grep 在執行檔裡
# 找旗標名稱（⛔ 不執行它）；找不到就停下（例如 build 目錄的程式比 E 舊、或為了回滾 checkout 了舊版）。
echo "==> 確認新 image 支援 -check-config"
# 結束碼分開判讀：1 才是「執行檔裡找不到」；124 是逾時；其他是 compose run 本身出錯（⛔ 不是旗標的問題）。
rc=0
timeout "$CHECK_CONFIG_TIMEOUT_SEC" "${COMPOSE[@]}" run --rm --no-deps -T --entrypoint grep backend -qF check-config ./trading-backend || rc=$?
case "$rc" in
  0) ;;
  1) die "新 image 的 backend 不認得 -check-config（程式比 I-132 E 舊？），⛔ 不執行設定檢查（舊的 binary 會直接啟動整個服務），停止部署（還沒 down）" ;;
  124) die "確認 -check-config 的步驟逾時（${CHECK_CONFIG_TIMEOUT_SEC} 秒），⛔ 不執行設定檢查，停止部署（還沒 down）" ;;
  *) die "確認 -check-config 的步驟失敗（compose run 的結束碼 $rc，⛔ 不是「找不到旗標」），⛔ 不執行設定檢查，停止部署（還沒 down）" ;;
esac
echo "==> 用新 image 檢查設定"
rc=0
timeout "$CHECK_CONFIG_TIMEOUT_SEC" "${COMPOSE[@]}" run --rm --no-deps -T backend ./trading-backend -check-config || rc=$?
case "$rc" in
  0) ;;
  124) die "新 image 的設定檢查逾時（${CHECK_CONFIG_TIMEOUT_SEC} 秒），停止部署（還沒 down，舊服務照常運作）" ;;
  *) die "新 image 的設定檢查沒有通過（結束碼 $rc；違反的鍵見上方），停止部署（還沒 down，舊服務照常運作）" ;;
esac

# ── 5a～5c. migration（停機前；T-087）───────────────────────
# 5a 先確認新 image 認得 -migrate（grep 執行檔裡的旗標名稱，⛔ 不執行；比照 -check-config）
echo "==> 確認新 image 支援 -migrate"
rc=0
timeout "$CHECK_CONFIG_TIMEOUT_SEC" "${COMPOSE[@]}" run --rm --no-deps -T --entrypoint grep backend -qF migrate-statement-timeout ./trading-backend || rc=$?
case "$rc" in
  0) ;;
  1) die "新 image 的 backend 不認得 -migrate（程式比 T-087 舊？），停止部署（還沒 down）" ;;
  124) die "確認 -migrate 的步驟逾時（${CHECK_CONFIG_TIMEOUT_SEC} 秒），停止部署（還沒 down）" ;;
  *) die "確認 -migrate 的步驟失敗（compose run 的結束碼 $rc），停止部署（還沒 down）" ;;
esac
if ! NEW_IMAGE="$(compose config --images backend 2>/dev/null)" || ! [[ "$NEW_IMAGE" =~ ^[A-Za-z0-9_.:/@-]+$ ]]; then
  die "讀不到 backend 的 image 名稱，停止部署（還沒 down）"
fi
# 5b 用新 image 唯讀讀 live 的 migration 狀態，記下基準
echo "==> 用新 image 讀 live 的 migration 狀態（唯讀）"
rc=0
out="$(timeout "$CHECK_CONFIG_TIMEOUT_SEC" "${COMPOSE[@]}" run --rm --no-deps -T backend ./trading-backend -migrate=status 2>/dev/null)" || rc=$?
case "$rc" in
  0) ;;
  2) die "live 的 DB 沒有 goose_db_version（從來沒有套用過 migration？），停止部署（還沒 down）" ;;
  124) die "讀 migration 狀態逾時（${CHECK_CONFIG_TIMEOUT_SEC} 秒），停止部署（還沒 down）" ;;
  *) die "讀 migration 狀態失敗（結束碼 $rc），停止部署（還沒 down）" ;;
esac
printf '%s\n' "$out" | grep '^migrate: ' | sed 's/^migrate: /  /' || true
BASE_FP="$(status_field "$out" fingerprint)" || BASE_FP=""
BASE_PENDING="$(status_field "$out" pending)" || BASE_PENDING=""
if ! { valid_fp "$BASE_FP" && valid_pending "$BASE_PENDING"; }; then die "migration 狀態的輸出格式不對，停止部署（還沒 down）"; fi
# 核對 migrator（compose 的 DSN）與這支腳本（DB_CONTAINER／DB_NAME）指向同一個 PostgreSQL 實例與 database（T-087 review）：
# 版本指紋只涵蓋已套用的版本，兩個不同的 DB 可能相同；備份、正式套用、停止確認必須是同一個 DB
MIG_INSTANCE="$(status_field "$out" instance)" || MIG_INSTANCE=""
valid_instance "$MIG_INSTANCE" || die "讀不到 migrator 連到的 DB 的實例識別，停止部署（還沒 down）"
SCRIPT_INSTANCE="$(script_instance)" || die "用 DB_CONTAINER／DB_NAME 查不到實例識別，停止部署（還沒 down）"
if [ "$MIG_INSTANCE" != "$SCRIPT_INSTANCE" ]; then
  die "backend 的 DSN 與 DB_CONTAINER=$DB_CONTAINER／DB_NAME=$DB_NAME 指向不同的 DB（migrator：$MIG_INSTANCE、腳本：$SCRIPT_INSTANCE），停止部署（還沒 down）"
fi
echo "==> migrator 與腳本連到同一個 DB：$MIG_INSTANCE"
# 5c 有待套用的 migration：停機前試跑
if [ "$BASE_PENDING" != "-" ]; then
  echo "==> 待套用的 migration：$BASE_PENDING（基準 ${BASE_FP:0:12}…）"
  run_trial
else
  echo "==> ⛔ 沒有待套用的 migration"
fi

# 回測的 pending／running（第二次，down 之前）：build 要好幾分鐘，這段期間舊的 worker 還在跑、會接走新送出的回測。
check_backtests "還沒 down" "；新 image 已經 build 好、舊服務照常運作，回測結束後重跑 ./deploy.sh"

# ── 6. 換版 ──────────────────────────────────────────────────
echo "==> down"
if ! compose down; then
  die "down 失敗，⛔ 不進 up；請手動確認服務狀態"
fi

# ── 6a～6b. 停機中套用 migration（T-087）──────────────────────
# 失敗時⛔ 不 up（build 之後 latest 已經指向新 image，自動 up 會啟動新版）、⛔ 不 goose down：印出下一步，照部署手冊的復原分支處理。
if [ "$BASE_PENDING" != "-" ]; then
  untouched="⛔ 沒有碰 schema：用部署前的回滾標籤把舊 image 換回 latest，再 up -d --no-build（部署手冊 R1、R2）"
  BACKUP_FILE=""
  now_instance="$(script_instance)" || now_instance=""
  [ "$now_instance" = "$MIG_INSTANCE" ] || die "down 之後 DB 的實例識別變了（${now_instance:-查不到}），⛔ 不備份、⛔ 不套用；服務目前是停的。$untouched"
  backup_db || die "備份失敗，⛔ 不套用；服務目前是停的。$untouched"
  echo "==> 正式套用：run_id $RUN_ID（容器 stock_trading-migrator-$RUN_ID、application_name trading-migrator-$RUN_ID）"
  rc=0
  timeout -k 10 "$MIGRATE_OUTER_TIMEOUT_SEC" "${COMPOSE[@]}" run --rm --no-deps -T --name "stock_trading-migrator-$RUN_ID" backend \
    ./trading-backend -migrate=up -run-id="$RUN_ID" -expect-fingerprint="$BASE_FP" -expect-instance="$MIG_INSTANCE" \
    -migrate-lock-timeout="$MIGRATE_LOCK_TIMEOUT" -migrate-statement-timeout="$MIGRATE_STATEMENT_TIMEOUT" -migrate-timeout="${MIGRATE_TIMEOUT_SEC}s" || rc=$?
  # 不論成功失敗都先確認 migrator 已經停下（外層 timeout 只砍得掉 docker CLI，⛔ 不代表容器與 DB 上的工作已經結束）
  confirm_migrator_stopped "$RUN_ID"
  partial="套用到一半：照部署手冊的復原分支——用 -migrate=status 查最後成功的版本並對照實際 schema；已提交的都是「先加」的變更、舊 image 確認相容才換回舊 image 啟動，否則從備份 $BACKUP_FILE 還原到乾淨的 DB 再啟動舊 image。服務目前是停的"
  case "$rc" in
    0) ;;
    5) die "正式套用前 live 的版本紀錄或實例跟基準不同（-expect-fingerprint／-expect-instance 不符），⛔ 沒有套用任何一支；服務目前是停的。$untouched" ;;
    3) die "正式套用有一支失敗（結束碼 3；前面的已提交）。$partial" ;;
    4) die "正式套用逾時（結束碼 4，整體上限 ${MIGRATE_TIMEOUT_SEC} 秒）。$partial" ;;
    124|137) die "正式套用的外層 timeout 到期（${MIGRATE_OUTER_TIMEOUT_SEC} 秒；停止確認已通過）。$partial" ;;
    *) die "正式套用失敗（結束碼 $rc）。$partial" ;;
  esac
  # 6b 再讀一次狀態：pending 必須是空的
  rc=0
  out="$(timeout "$CHECK_CONFIG_TIMEOUT_SEC" "${COMPOSE[@]}" run --rm --no-deps -T backend ./trading-backend -migrate=status 2>/dev/null)" || rc=$?
  after_pending="$(status_field "$out" pending)" || after_pending=""
  if ! { [ "$rc" = 0 ] && [ "$after_pending" = "-" ]; }; then die "套用之後 pending 不是空的（結束碼 $rc、pending ${after_pending:-?}）。$partial"; fi
  echo "==> migration 套用完成：$BASE_PENDING"
fi

echo "==> up -d --no-build"
if ! compose up -d --no-build; then
  die "up 失敗；服務目前是停的，請手動處理（⛔ 沒有自動回滾）"
fi

# ── 7. readiness ─────────────────────────────────────────────
# up -d 成功只代表 Docker 收下了啟動要求，⛔ 不代表 backend 通過了設定和 Redis 的檢查。
# 要同時符合「container 內打 /health 拿到成功回應」和「RestartCount 是 0」才算通過。
# 上限是**總截止時間**（READY_TIMEOUT_SEC），⛔ 不是檢查次數：每個 docker 呼叫都用 timeout 包住，
# 單次上限取 READY_REQUEST_TIMEOUT_SEC 與剩餘時間較小的那個；wget 自己也帶 -T。
# 請求卡住或回應緩慢時，最多超過總上限約 1 秒（SECONDS 的精度），⛔ 不會在截止之後才回報成功。
echo "==> 等 backend ready（最多 ${READY_TIMEOUT_SEC} 秒）"
deadline=$((SECONDS + READY_TIMEOUT_SEC))
remaining() {
  local r=$((deadline - SECONDS))
  [ "$r" -gt 0 ] || r=0
  if [ "$r" -gt "$1" ]; then echo "$1"; else echo "$r"; fi
}
checks=0
while :; do
  t="$(remaining "$READY_REQUEST_TIMEOUT_SEC")"
  [ "$t" -gt 0 ] || break
  checks=$((checks + 1))
  cid="$(timeout "$t" "${COMPOSE[@]}" ps -q backend 2>/dev/null || true)"
  if [ -n "$cid" ]; then
    t="$(remaining "$READY_REQUEST_TIMEOUT_SEC")"
    [ "$t" -gt 0 ] || break
    restarts="$(timeout "$t" docker inspect -f '{{.RestartCount}}' "$cid" 2>/dev/null || echo "?")"
    if [ "$restarts" != "0" ] && [ "$restarts" != "?" ]; then
      die "backend 重啟過（RestartCount $restarts），部署失敗；看 log：docker compose logs --tail 200 backend（⛔ 沒有自動回滾）"
    fi
    t="$(remaining "$READY_REQUEST_TIMEOUT_SEC")"
    [ "$t" -gt 0 ] || break
    if timeout "$t" "${COMPOSE[@]}" exec -T backend wget -q -T "$t" -O /dev/null http://127.0.0.1:8080/health >/dev/null 2>&1; then
      # 成功回應之後再確認一次：沒有重啟過、而且還在截止時間之內
      t="$(remaining "$READY_REQUEST_TIMEOUT_SEC")"
      [ "$t" -gt 0 ] || break
      restarts="$(timeout "$t" docker inspect -f '{{.RestartCount}}' "$cid" 2>/dev/null || echo "?")"
      if [ "$restarts" = "0" ] && [ "$SECONDS" -le "$deadline" ]; then
        echo "==> 部署完成：backend /health 正常、RestartCount 0（第 $checks 次確認）"
        exit 0
      fi
      [ "$restarts" = "?" ] || die "backend 重啟過（RestartCount $restarts），部署失敗（⛔ 沒有自動回滾）"
    fi
  fi
  t="$(remaining "$READY_INTERVAL_SEC")"
  [ "$t" -gt 0 ] || break
  sleep "$t"
done
die "等了 ${READY_TIMEOUT_SEC} 秒 backend 還沒 ready（確認 $checks 次），部署失敗；看 log：docker compose logs --tail 200 backend（⛔ 沒有自動回滾）"
