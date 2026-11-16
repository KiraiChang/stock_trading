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
#   preflight → git pull --ff-only → compose config（JWT）→ 記憶體檢查 → build（逐一服務，舊服務還在跑）
#   → 用新 image 檢查完整設定 → down → up -d --no-build → readiness（/health 與 RestartCount）
# ⚠️ 部署失敗時⛔ 沒有自動回滾：只會回報失敗，由人判斷要不要換回前一版的 image。
# ⛔ 這裡不再 export 任何設定：應用設定在 .env、Redis 密碼在 Redis stack 的 .env，都由 compose 的 env_file 注入。
set -euo pipefail

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

# ── 由腳本位置推出來的路徑（⛔ 不要改）────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STACK_COMPOSE_FILE="$SCRIPT_DIR/compose.yml"
APP_ENV_FILE="$SCRIPT_DIR/.env"
# 測試用（scripts/test-deploy-sh.sh）；正式使用⛔ 不要設
MEMINFO_FILE="${DEPLOY_MEMINFO_FILE:-/proc/meminfo}"

# 跟 backend 設定契約的 PublicJWTSecrets 相同（backend/internal/config/contract.go；測試會比對）
PUBLIC_JWT_SECRETS="change-me-in-production change-me dev-only-change-me"

die() {
  echo "deploy.sh: $*" >&2
  exit 1
}

COMPOSE=(docker compose -f "$STACK_COMPOSE_FILE" --project-directory "$PROJECT_DIR")
compose() {
  "${COMPOSE[@]}" "$@"
}

# ── 1. preflight（都在 git pull 之前）────────────────────────
echo "==> preflight"
# readiness 用 timeout 限制每次請求（coreutils；⛔ 不能缺，否則一個卡住的請求就沒有上限）
command -v timeout >/dev/null 2>&1 || die "找不到 timeout 指令（coreutils），停止部署（還沒 down）"
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

# ── 2. 更新 build 目錄 ────────────────────────────────────────
echo "==> git pull --ff-only origin $DEPLOY_BRANCH"
if ! git -C "$PROJECT_DIR" pull --ff-only origin "$DEPLOY_BRANCH"; then
  die "git pull 失敗（只允許 fast-forward），停止部署（還沒 down）"
fi

# ── 3. 最終設定裡的 JWT（在 build 之前就能先擋）──────────────
# 用跟部署相同的參數跑 compose config，**在記憶體裡**用 awk 取出 backend 最終的 AUTH_JWT_SECRET。
# ⛔ 不把 config 寫到檔案或印到畫面；config 的錯誤訊息也⛔ 不顯示（可能帶出 env_file 的內容）。
# 完整的設定契約由第 5 步用新 image 檢查，這一步只涵蓋 JWT。
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
echo "==> 用新 image 檢查設定"
if ! compose run --rm --no-deps backend ./trading-backend -check-config; then
  die "新 image 的設定檢查沒有通過（違反的鍵見上方），停止部署（還沒 down，舊服務照常運作）"
fi

# ── 6. 換版 ──────────────────────────────────────────────────
echo "==> down"
if ! compose down; then
  die "down 失敗，⛔ 不進 up；請手動確認服務狀態"
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
