#!/usr/bin/env bash
# python-worker 的 dev 驗收：build（可略過）→ 啟動 → 確認用的是本次的 image、持續運行、啟動 log 沒有錯誤
# → 收尾（停止並移除本次建立的 worker）。T-083 的計畫書 3.5。
#
# 為什麼另外一支：scripts/smoke-dev.sh ⛔ 不支援 python-worker（它在 compose 的 `worker` profile、
# 有自己的 build 與 image），而且 smoke-dev.sh 預設會先 down，語意不同。
#
# Usage:
#   scripts/smoke-dev-worker.sh
#   NO_BUILD=1 POLLS=5 scripts/smoke-dev-worker.sh
#   EXPECT_FREEZE_MD5=39c5d9d45667ea4d872b9e746ac99600 scripts/smoke-dev-worker.sh
#
# Environment overrides:
#   NO_BUILD           1 = 不 build，用現有的 image（不存在就失敗）（default: 0）
#   EXPECT_FREEZE_MD5  有設定時，用本次的 image 檢查 `pip check` 與 `pip freeze --all | LC_ALL=C sort | md5sum`
#                      （⚠️ 一定要 LC_ALL=C：排序跟 locale 有關，en_US 與 C 排出來的順序不同、md5 就不同）
#                      （不連網路、⛔ 不啟動 worker 程式），不相符就停下（default: 不檢查）
#   POLLS              要等到的輪詢次數：log 出現 `poll #N` 才算持續運行（default: 3）
#   WAIT_SECONDS       等 postgres healthy 與等 `poll #N` 各自的上限秒數（default: 90）
#   CHECK_INTERVAL     確認 container 狀態的間隔秒數（default: 2）
#   CALL_TIMEOUT       單一 docker 查詢／啟動指令的上限秒數（default: 30；等待期間還會再被剩餘時間截短）
#   BUILD_TIMEOUT      build 的上限秒數（default: 1200；冷 build 要從網路裝套件）
#   CLEANUP_TIMEOUT    收尾（移除 worker 與確認已移除）的上限秒數（default: 60）
#   KILL_AFTER         逾時送出 SIGTERM 之後，再等幾秒就送 SIGKILL（default: 5；docker／plugin 忽略 SIGTERM 時
#                      才用得到）
#   LOG_TAIL           失敗時印出的 worker log 行數（default: 120）
#
# 結束碼：0 通過；1 驗收失敗或收尾失敗；2 參數或前置條件不對（⛔ 沒有啟動 worker）。
#
# ⛔ 只操作 dev project：compose 檔固定是 repo 的 docker-compose.dev.yml（⛔ 不接受覆寫），而且在任何會
#    改變狀態的指令之前，先確認解析出來的 project 名稱是 stock_trading_dev；不是就停下。
#    COMPOSE_PROJECT_NAME 有設定（連空字串）也停下：它會蓋過 compose 檔的 name:。
# 收尾：從啟動 worker 那一步起註冊 trap，成功、失敗、逾時、Ctrl+C 都會停止並移除**本次建立的**
#       python-worker（⛔ 不動 postgres 等其他服務），並確認它真的不在了；移除失敗或逾時 → 結束碼 1
#       （⛔ 不能回報成功卻留下一個還在輪詢的 worker）。開始前就有 python-worker 的 container 時，
#       ⛔ 不啟動、⛔ 不移除它（不是本次建立的）。
# 每個 docker 指令都用 coreutils 的 timeout 包住；等待期間單次查詢的上限取 CALL_TIMEOUT 與剩餘時間較小的
#   那個；逾時先送 SIGTERM，KILL_AFTER 秒後還沒結束就送 SIGKILL（`timeout -k`，整個 process group）。
#   所以 docker 卡住、甚至忽略 SIGTERM 時，等待也會在 WAIT_SECONDS＋KILL_AFTER（＋約 1 秒）內結束，
#   收尾在 CLEANUP_TIMEOUT＋KILL_AFTER 內結束。
# ⛔ 輸出只有步驟、image ID、md5 與 log 的摘要，不印 DSN。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="docker-compose.dev.yml"
DEV_PROJECT="stock_trading_dev"
NO_BUILD="${NO_BUILD:-0}"
EXPECT_FREEZE_MD5="${EXPECT_FREEZE_MD5:-}"
POLLS="${POLLS:-3}"
WAIT_SECONDS="${WAIT_SECONDS:-90}"
CHECK_INTERVAL="${CHECK_INTERVAL:-2}"
CALL_TIMEOUT="${CALL_TIMEOUT:-30}"
BUILD_TIMEOUT="${BUILD_TIMEOUT:-1200}"
CLEANUP_TIMEOUT="${CLEANUP_TIMEOUT:-60}"
KILL_AFTER="${KILL_AFTER:-5}"
LOG_TAIL="${LOG_TAIL:-120}"
SVC=python-worker

# ── 先驗參數：任何 docker 指令之前 ──
if [ -n "${COMPOSE_PROJECT_NAME+x}" ]; then
  echo "ERROR: COMPOSE_PROJECT_NAME is set (\"$COMPOSE_PROJECT_NAME\"); it would override the project" >&2
  echo "       name in $COMPOSE_FILE. Unset it and rerun." >&2
  exit 2
fi
for v in POLLS WAIT_SECONDS CHECK_INTERVAL CALL_TIMEOUT BUILD_TIMEOUT CLEANUP_TIMEOUT KILL_AFTER LOG_TAIL; do
  case "${!v}" in
    ''|*[!0-9]*) echo "ERROR: $v 要是正整數（目前：${!v}）" >&2; exit 2 ;;
  esac
  [ "${!v}" -gt 0 ] || { echo "ERROR: $v 要是正整數（目前：${!v}）" >&2; exit 2; }
done
case "$NO_BUILD" in 0|1) ;; *) echo "ERROR: NO_BUILD 只能是 0 或 1" >&2; exit 2 ;; esac
if [ -n "$EXPECT_FREEZE_MD5" ] && ! [[ "$EXPECT_FREEZE_MD5" =~ ^[0-9a-f]{32}$ ]]; then
  echo "ERROR: EXPECT_FREEZE_MD5 要是 md5（32 個小寫 16 進位字元）" >&2
  exit 2
fi
command -v timeout >/dev/null 2>&1 || { echo "ERROR: 找不到 timeout（coreutils）；沒有它 docker 卡住就沒有上限" >&2; exit 2; }

cd "$REPO_ROOT"
# t SEC CMD…：有上限的執行（stdin 是 /dev/null，在終端機執行時才不會被 SIGTTIN 停住）。
# 逾時先送 SIGTERM；⚠️ 一定要 -k：只送 SIGTERM 時，忽略它的 docker／plugin 會一直卡著（review 2026-11-18）。
t() {
  local sec="$1"; shift
  timeout -k "$KILL_AFTER" "$sec" "$@" </dev/null
}
compose() {
  local sec="$1"; shift
  t "$sec" docker compose -f "$COMPOSE_FILE" --profile worker "$@"
}

STARTED=0
cleanup() {
  local rc=$? left cdl
  trap - EXIT INT TERM
  if [ "$STARTED" = 1 ]; then
    # 整段收尾共用一個上限（CLEANUP_TIMEOUT）：印 log、移除、確認，每一步都只拿剩下的時間
    cdl=$((SECONDS + CLEANUP_TIMEOUT))
    crem() { local r=$((cdl - SECONDS)); (( r < 1 )) && r=1; echo "$r"; }
    if [ "$rc" -ne 0 ]; then
      echo
      echo "==> $SVC logs（最後 $LOG_TAIL 行）"
      # 印 log 最多用一半的收尾時間，留時間給移除
      compose $(( (CLEANUP_TIMEOUT + 1) / 2 )) logs --no-color --tail="$LOG_TAIL" "$SVC" 2>&1 || true
    fi
    echo "==> 收尾：停止並移除本次建立的 $SVC（最多約 $((CLEANUP_TIMEOUT + KILL_AFTER)) 秒）"
    if ! compose "$(crem)" rm -sf "$SVC" >/dev/null 2>&1; then
      echo "ERROR: 移除 $SVC 失敗或逾時；它可能還在輪詢，請手動確認（docker compose -f $COMPOSE_FILE --profile worker ps -a）" >&2
      [ "$rc" -eq 0 ] && rc=1
    elif ! left="$(compose "$(crem)" ps -a -q "$SVC" 2>/dev/null)" || [ -n "$left" ]; then
      echo "ERROR: 移除之後仍查得到 $SVC（或查詢失敗）；請手動確認" >&2
      [ "$rc" -eq 0 ] && rc=1
    else
      echo "    已移除"
    fi
  fi
  exit "$rc"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

# 0. 只操作 dev project（任何會改變狀態的指令之前）
project="$(compose "$CALL_TIMEOUT" config 2>/dev/null | awk '/^name:/{print $2; exit}')" || project=""
if [ "$project" != "$DEV_PROJECT" ]; then
  echo "ERROR: $COMPOSE_FILE 解析出來的 project 是「${project:-（查不到）}」，不是 $DEV_PROJECT；⛔ 不操作" >&2
  exit 2
fi

# 1. 開始前就有 worker → 不是本次建立的，⛔ 不動它
if ! existing="$(compose "$CALL_TIMEOUT" ps -a -q "$SVC")"; then
  echo "ERROR: 查不到 $SVC 的狀態（失敗或逾時）" >&2
  exit 1
fi
if [ -n "$existing" ]; then
  echo "ERROR: 已經有 $SVC 的 container（${existing:0:12}），不是本次建立的；⛔ 不啟動、⛔ 不移除。" >&2
  echo "       確認沒人在用之後，自己停掉它再重跑。" >&2
  exit 2
fi

# 2. postgres 起到 healthy
echo "==> postgres：起到 healthy（最多 ${WAIT_SECONDS} 秒）"
if ! compose $((WAIT_SECONDS + CALL_TIMEOUT)) up -d --wait --wait-timeout "$WAIT_SECONDS" postgres; then
  echo "ERROR: postgres 沒有在 ${WAIT_SECONDS} 秒內 healthy" >&2
  exit 1
fi

# 3. 有 pending 的回測任務 → worker 一起來就會執行它們，⛔ 不啟動
if ! pending="$(compose "$CALL_TIMEOUT" exec -T postgres psql -U trading -d trading -tAc \
    "select count(*) from backtest_jobs where status='pending'" 2>/dev/null)"; then
  echo "ERROR: 查不到 dev DB 的 backtest_jobs（表不存在、連不上或逾時），⛔ 不啟動 $SVC" >&2
  exit 2
fi
pending="${pending//[[:space:]]/}"
if [ "$pending" != 0 ]; then
  echo "ERROR: dev DB 有 ${pending} 筆 pending 的 backtest_jobs；$SVC 一啟動就會執行它們，⛔ 不啟動" >&2
  exit 2
fi
echo "==> backtest_jobs：pending 0 筆"

# 4. build
if [ "$NO_BUILD" = 1 ]; then
  echo "==> 略過 build（NO_BUILD=1）"
else
  echo "==> build $SVC（最多 ${BUILD_TIMEOUT} 秒）"
  compose "$BUILD_TIMEOUT" build "$SVC" || { echo "ERROR: build 失敗或逾時" >&2; exit 1; }
fi
# ⚠️ `config --images python-worker` 也會列出 depends_on 的 postgres，而且順序不固定（2026-11-18 dev 實跑：
#    `head -1` 取到 postgres:16-alpine）。python-worker 沒有 image: 鍵，名稱是 compose 預設的
#    <project>-<service>；project 已在第 0 步驗過，所以只認這個名稱、要剛好出現一次。
want_image="${DEV_PROJECT}-${SVC}"
images="$(compose "$CALL_TIMEOUT" config --images "$SVC")" || images=""
if [ "$(grep -cFx "$want_image" <<<"$images")" = 1 ]; then image="$want_image"; else image=""; fi
image_id="$(t "$CALL_TIMEOUT" docker image inspect -f '{{.Id}}' "$image" 2>/dev/null)" || image_id=""
if [ -z "$image" ] || [ -z "$image_id" ]; then
  echo "ERROR: 找不到 $SVC 的 image（${image:-查不到名稱}）" >&2
  exit 1
fi
echo "==> image：$image ${image_id:7:12}"

# 5. 套件檢查（選用）：不連網路、⛔ 不啟動 worker 程式
if [ -n "$EXPECT_FREEZE_MD5" ]; then
  echo "==> pip check 與 pip freeze 的 md5"
  got="$(t $((CALL_TIMEOUT * 4)) docker run --rm --network none --entrypoint sh "$image_id" -c \
    'pip check >&2 && pip freeze --all 2>/dev/null | LC_ALL=C sort | md5sum | cut -d" " -f1')" || {
    echo "ERROR: pip check 沒有通過（或逾時）" >&2
    exit 1
  }
  if [ "$got" != "$EXPECT_FREEZE_MD5" ]; then
    echo "ERROR: pip freeze 的 md5 是 $got，預期 $EXPECT_FREEZE_MD5" >&2
    exit 1
  fi
  echo "    ok：$got"
fi

# 6. 啟動（從這裡起，失敗也會收尾）
echo "==> 啟動 $SVC（--no-deps）"
STARTED=1
compose $((CALL_TIMEOUT * 2)) up -d --no-build --no-deps "$SVC" || { echo "ERROR: 啟動 $SVC 失敗或逾時" >&2; exit 1; }

# 7. 用的是本次的 image
cid="$(compose "$CALL_TIMEOUT" ps -q "$SVC")" || cid=""
[ -n "$cid" ] || { echo "ERROR: 找不到剛啟動的 $SVC container" >&2; exit 1; }
used="$(t "$CALL_TIMEOUT" docker inspect -f '{{.Image}}' "$cid")" || used=""
if [ "$used" != "$image_id" ]; then
  echo "ERROR: $SVC 用的 image 是 ${used:7:12}，不是本次的 ${image_id:7:12}" >&2
  exit 1
fi
echo "==> container ${cid:0:12} 用的是本次的 image"

# 8. 持續運行到 poll #POLLS（總上限 WAIT_SECONDS；單次查詢也受剩餘時間限制）
echo "==> 等 log 出現 poll #$POLLS（最多 ${WAIT_SECONDS} 秒）"
deadline=$((SECONDS + WAIT_SECONDS))
left_sec() {
  local r=$((deadline - SECONDS))
  (( r > CALL_TIMEOUT )) && r=$CALL_TIMEOUT
  (( r < 1 )) && r=1
  echo "$r"
}
timed_out() {
  echo "ERROR: ${WAIT_SECONDS} 秒內沒有等到 poll #$POLLS（$1）" >&2
  exit 1
}
while :; do
  (( SECONDS < deadline )) || timed_out "時間到"
  if ! state="$(t "$(left_sec)" docker inspect -f '{{.State.Status}} {{.RestartCount}}' "$cid")"; then
    (( SECONDS < deadline )) || timed_out "查 container 狀態卡住"
    echo "ERROR: 查不到 $SVC 的狀態" >&2
    exit 1
  fi
  if [ "$state" != "running 0" ]; then
    echo "ERROR: $SVC 沒有持續運行（狀態、RestartCount：$state）" >&2
    exit 1
  fi
  (( SECONDS < deadline )) || timed_out "時間到"
  # 先存成變數再比對：管線裡 grep -q 提早結束會讓 compose logs 收到 SIGPIPE，pipefail 下整條判成失敗
  if ! logs="$(compose "$(left_sec)" logs --no-color "$SVC" 2>&1)"; then
    (( SECONDS < deadline )) || timed_out "讀 log 卡住"
    echo "ERROR: 讀不到 $SVC 的 log" >&2
    exit 1
  fi
  if grep -q "poll #${POLLS}[^0-9]" <<<"$logs"; then
    break
  fi
  (( SECONDS + CHECK_INTERVAL <= deadline )) || timed_out "時間到"
  sleep "$CHECK_INTERVAL"
done

# 9. 啟動 log（用上一步最後讀到的那一份）
if ! grep -q 'worker started' <<<"$logs"; then
  echo "ERROR: log 沒有 worker started" >&2
  exit 1
fi
if grep -qE 'Traceback|poll #[0-9]+ error' <<<"$logs"; then
  echo "ERROR: log 有 Traceback 或輪詢錯誤" >&2
  exit 1
fi
echo "==> 通過：持續運行到 poll #$POLLS、RestartCount 0、log 沒有 Traceback／輪詢錯誤"
