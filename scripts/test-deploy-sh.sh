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
  /bin/sleep "$1" &
  local p=$!
  trap 'kill "$p" 2>/dev/null; exit 143' TERM INT
  wait "$p"
  trap - TERM INT
}
if [ "${1:-}" = "inspect" ]; then
  echo "docker $*" >> "$CALL_LOG"
  echo "${FAKE_RESTARTS:-0}"
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
case "$1" in
  config)
    [ "${FAKE_CONFIG_RC:-0}" = 0 ] || exit "$FAKE_CONFIG_RC"
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
    # 一定要寫出執行檔：只帶旗標的話，真的 container 會把旗標當成程式去執行
    case " $* " in
      *" backend ./trading-backend -check-config "*) exit "${FAKE_CHECK_RC:-0}" ;;
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
      "$REPO/deploy.sh" > "$CASE/scripts/deploy.sh"
  for k in READY_TIMEOUT_SEC READY_INTERVAL_SEC READY_REQUEST_TIMEOUT_SEC; do
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

# run_deploy：在乾淨的環境變數下執行（FAKE_* 由呼叫端用 env 帶入）
run_deploy() {
  local t0=$SECONDS
  env -i PATH="$FAKEBIN:/usr/bin:/bin" HOME="$WORK" CALL_LOG="$CASE/calls" WORK_STATE="$CASE/state" \
    FAKE_JWT_MARKER="$FAKE_JWT_MARKER" DEPLOY_MEMINFO_FILE="$CASE/meminfo" "$@" \
    bash "$CASE/scripts/deploy.sh" > "$CASE/out" 2>&1
  RC=$?
  ELAPSED=$((SECONDS - t0))
}

called() { grep -q -- "$1" "$CASE/calls"; }
no_secret() { ! grep -q "$FAKE_JWT_MARKER" "$CASE/out"; }

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
order="$(grep -oE '^(git -C [^ ]+ (rev-parse --is-inside-work-tree|status|pull)|compose (config$|config --services|build [a-z-]+|run|down|up|exec))' "$CASE/calls" \
  | sed -E 's#^git -C [^ ]+ #git #' | uniq | tr '\n' '|')"
want="git rev-parse --is-inside-work-tree|git status|git pull|compose config|compose config --services|compose build backend|compose build python-server|compose build python-worker|compose run|compose down|compose up|compose exec|"
if [ "$RC" -eq 0 ] && [ "$order" = "$want" ] && ! called UP_WITH_BUILD && ! called WGET_WITHOUT_TIMEOUT && called "^compose up -d --no-build$" \
   && called "^compose run --rm --no-deps backend ./trading-backend -check-config$" && no_secret; then
  ok "正常：呼叫順序 preflight → pull --ff-only → config → build ×3 → check-config → down → up -d --no-build → readiness"
else
  ng "正常：rc $RC，順序 $order"
fi
if called "pull --ff-only origin main"; then ok "正常：git pull 只允許 fast-forward"; else ng "正常：git pull 沒有 --ff-only"; fi

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
setup_case pull;     run_deploy FAKE_PULL_RC=1;      expect_stop_before_down "git pull 失敗" "^compose config" "^compose build"
setup_case config;   run_deploy FAKE_CONFIG_RC=1;    expect_stop_before_down "compose config 失敗" "^compose build"
setup_case jwtempty; run_deploy FAKE_JWT_LINE='      AUTH_JWT_SECRET: ""'; expect_stop_before_down "JWT 是空值" "^compose build"
setup_case jwtmissing; run_deploy FAKE_JWT_LINE='      B_KEY: y'; expect_stop_before_down "backend 沒有 JWT" "^compose build"
for v in change-me-in-production change-me dev-only-change-me '"change-me"'; do
  setup_case "jwtpub"; run_deploy FAKE_JWT_LINE="      AUTH_JWT_SECRET: $v"
  expect_stop_before_down "JWT 是公開範例值 $v" "^compose build"
done
setup_case jwtother; run_deploy FAKE_JWT_LINE='      AUTH_JWT_SECRET: change-me-too'
if [ "$RC" -eq 0 ]; then ok "JWT 只是開頭像範例值 → 通過"; else ng "JWT 只是開頭像範例值：rc $RC"; fi
setup_case mem;      printf 'MemAvailable:    1000000 kB\n' > "$CASE/meminfo"; run_deploy
expect_stop_before_down "MemAvailable 低於門檻" "^compose build"
setup_case memmissing; printf 'MemTotal: 1 kB\n' > "$CASE/meminfo"; run_deploy
expect_stop_before_down "讀不到 MemAvailable" "^compose build"
setup_case build;    run_deploy FAKE_BUILD_FAIL=python-server; expect_stop_before_down "build 失敗" "^compose run" "compose build python-worker"
setup_case check;    run_deploy FAKE_CHECK_RC=1;     expect_stop_before_down "新 image 的設定檢查失敗"

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

if errs:
    for e in errs:
        print("  FAIL " + e)
    sys.exit(1)
print(f"  ok   範本 contract：environment {len(env_keys)} 個鍵、.env.example {len(ex)} 個鍵，不重複、不漏鍵；"
      "沒有 REDIS_PASSWORD、${…:?}、export；deploy.sh 的公開範例 JWT 跟 contract.go 一致")
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
