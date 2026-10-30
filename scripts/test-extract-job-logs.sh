#!/usr/bin/env bash
# scripts/extract-job-logs.py 的迴歸測試（I-122 D1；規格見 docs/plans/i122-live-batch-failures.md「八」）。
#
# ⚠️ 以 fake docker 執行（PATH 最前面放 $TD/bin/docker），⛔ 不碰 live、⛔ 不需要 image。
# ⚠️ 失敗的案例一律檢查三件事：結束碼非零、輸出目錄裡⛔ 沒有任何檔（含暫存檔）、stderr ⛔ 不含 fixture 裡的假 secret 與 log 原文。
#
# 反向驗證時用 EXTRACT_TOOL 指向改壞的副本：拿掉任一層遮罩或任一個結束碼檢查，對應的案例要變紅。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TOOL="${EXTRACT_TOOL:-$REPO_ROOT/scripts/extract-job-logs.py}"
PASS=0; FAIL=0
pass() { PASS=$((PASS+1)); echo "  ok   $1"; }
fail() { FAIL=$((FAIL+1)); echo "  FAIL $1" >&2; }

TD="$(mktemp -d)"
trap 'rm -rf "$TD"' EXIT
mkdir -p "$TD/bin" "$TD/base/logs"

# ── fake docker：只認工具會用到的兩種呼叫，呼叫紀錄寫進 $FAKE_ROOT/calls.log ──
cat > "$TD/bin/docker" <<'SH'
#!/usr/bin/env bash
echo "$*" >> "$FAKE_ROOT/calls.log"
if [ "$#" -eq 4 ] && [ "$1" = exec ] && [ "$3" = cat ]; then
  f="$FAKE_ROOT/logs/$(basename "$4")"
  [ -f "$f" ] || { echo "cat: can't open '$4': No such file or directory" >&2; exit 1; }
  cat "$f"
  # FAKE_EXEC_FAIL：內容照樣輸出之後才回非零（工具⛔ 不得採用已讀到的部分）
  [ "${FAKE_EXEC_FAIL:-}" = "$(basename "$4")" ] && exit 1
  exit 0
fi
if [ "$#" -eq 5 ] && [ "$1 $2 $3" = "container inspect --format" ] && [ "$4" = '{{json .Config.Env}}' ]; then
  [ "${FAKE_INSPECT_FAIL:-0}" = 1 ] && { echo "Error: No such container: $5" >&2; exit 1; }
  cat "$FAKE_ROOT/env.json"
  exit 0
fi
echo "fake docker：未預期的呼叫" >&2
exit 98
SH
chmod +x "$TD/bin/docker"

# ── 基本 fixture ──
# 時間窗（台北）→ UTC：100 ＝ 10-27 22:30Z；101／106 ＝ 10-28 22:30Z 起；102 ＝ 10-28 23:00Z；
# 107 ＝ 10-29 02:00Z（沒有 log 行）；103 ＝ 10-29 05:30Z；104 ＝ 05:25Z（intraday 的對照）；105 ⛔ 不是對照。
cat > "$TD/base/windows.csv" <<'CSV'
id,job_name,status,symbols_total,symbols_failed,started_tpe,finished_tpe,error
100,stock_symbol_sync,success,2,0,2026-10-28T06:30:00.000000,2026-10-28T06:31:31.000000,
101,stock_symbol_sync,partial,0,1,2026-10-29T06:30:00.000000,2026-10-29T06:30:10.000000,symbol_sync:timeout
106,corporate_action_sync,partial,100,3,2026-10-29T06:30:00.000000,2026-10-29T07:05:00.000000,"yahoo:3 (2330:timeout, 2317:timeout, 1101:upstream_error)"
102,delisting_reconcile,failed,0,0,2026-10-29T07:00:00.000000,2026-10-29T07:00:00.008000,delisting_reconcile:internal_error
107,intraday,aborted,0,0,2026-10-29T10:00:00.000000,2026-10-29T10:00:01.000000,process 在此 job 完成前結束
104,intraday,success,10,0,2026-10-29T13:25:00.000000,2026-10-29T13:25:08.000000,
103,intraday,failed,10,10,2026-10-29T13:30:00.000000,2026-10-29T13:30:10.000000,fetch_failed:10
105,intraday,success,10,0,2026-10-29T13:45:00.000000,2026-10-29T13:45:08.000000,
CSV
cat > "$TD/base/logs/backend-2026-10-27.log" <<'LOG'
{"level":"info","ts":"2026-10-27T22:30:30.000Z","caller":"scheduler/scheduler.go:1","msg":"stock symbol sync done CONTROL100-MARK"}
{"level":"info","ts":"2026-10-27T23:59:59.000Z","msg":"late OUTSIDE27-MARK"}
LOG
cat > "$TD/base/logs/backend-2026-10-28.log" <<'LOG'
{"level":"info","ts":"2026-10-28T22:29:50.000Z","msg":"before window OUTSIDE-MARK"}
{"level":"error","ts":"2026-10-28T22:30:09.500Z","caller":"scheduler/scheduler.go:1","msg":"stock symbol sync failed","error":"Get \"https://isin.twse.com.tw/isin/C_public.jsp?strMode=2&q=L1ONLYFAKE&token=QTOKENFAKE1\": context deadline exceeded"}
{"level":"error","ts":"2026-10-29T06:30:10.000+0800","msg":"kv api_key=KVFAKE77 and secret_key: KVFAKE88 then password authentication failed for user x"}
{"level":"warn","ts":"2026-10-28T22:30:11.000Z","msg":"shapes jwt eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.c2lnZmFrZQ conn postgres://CONNFAKE@db:5432/x userinfo https://ui:UIFAKE9@example.com/p hdr Authorization: Bearer BEARERFAKE1 long Zz9Yy8Xx7Ww6Vv5Uu4Tt3Ss2Rr1Qq0Pp9O short Ab3Cd4Ef5Gh6Ij7Kl8Mn9Op0Qr1St2U letters abcdefghijabcdefghijabcdefghijabcdefghij ver v1.2.3 plain https://example.org/plain/path glued base_https://example.net/x?k=GLUEDFAKE and x_postgres://GLUEDCONN@db/x","fields":{"token":"STRUCTFAKE5","count":3}}
{"level":"error","ts":"2026-10-28T23:00:00.004Z","caller":"scheduler/delisting_reconcile.go:1","msg":"delisting reconcile failed","error":"stock_symbol_sync has not succeeded today"}
LOG
cat > "$TD/base/logs/backend-2026-10-29.log" <<'LOG'
{"level":"info","ts":"2026-10-29T05:25:03.000Z","msg":"intraday batch done CONTROL104-MARK"}
{"level":"error","ts":"2026-10-29T05:30:05.000Z","caller":"scheduler/scheduler.go:1","msg":"intraday batch fetch failed","error":"yahoo: unexpected status 503"}
{"level":"info","ts":"2026-10-29T05:45:03.000Z","msg":"intraday batch done NOT-CONTROL-MARK"}
LOG
cat > "$TD/base/env.json" <<'JSON'
["PATH=/usr/local/bin:/usr/bin","AUTH_JWT_SECRET=jwt-secret-FAKE-1","DATABASE_DSN=postgres://trading_username:dsnpw-FAKE-2@postgres:5432/trading?sslmode=disable","FINMIND_API_KEY=finmind-FAKE-3","FUGLE_API_KEY=fugle-FAKE-4","REDIS_PASSWORD=","LOG_RETENTION_DAYS=14"]
JSON

# fixture 裡所有⛔ 不得出現在 stderr 的字串（假 secret、log 原文的標記）
LEAKS=(QTOKENFAKE1 L1ONLYFAKE GLUEDFAKE GLUEDCONN TAIL_CANARY TAILSQ TAILESC 'TAIL QUOTED' 31415926 KVFAKE77 KVFAKE88 CONNFAKE UIFAKE9 BEARERFAKE1 STRUCTFAKE5 Zz9Yy8Xx7Ww6
       LEAKFAKE-BAD-1 LEAKFAKE-TS-2 jwt-secret-FAKE-1 dsnpw-FAKE-2 finmind-FAKE-3 fugle-FAKE-4
       'j/w+t' 'j%2Fw' 'r"q' MARK 'stock symbol sync' 'context deadline' '"msg"' Traceback)

N=0
new_case() {   # 以基本 fixture 建一個乾淨的案例目錄 $C
  N=$((N+1)); C="$TD/case$N"
  mkdir -p "$C/out"
  cp -r "$TD/base/logs" "$C/logs"
  cp "$TD/base/windows.csv" "$TD/base/env.json" "$C/"
}
run_tool() {   # 結束碼放 $RC；stdout／stderr 放檔案
  set +e
  FAKE_ROOT="$C" PATH="$TD/bin:$PATH" python3 -I "$TOOL" --windows "$C/windows.csv" \
    --out "$C/out/e2.jsonl" --report "$C/out/e2_report.txt" > "$C/stdout" 2> "$C/stderr"
  RC=$?
  set -e
}
no_leak() {    # $1 檔案；額外的字串放在 $2…
  local f="$1" s; shift
  for s in "${LEAKS[@]}" "$@"; do
    if grep -qF -- "$s" "$f"; then return 1; fi
  done
  return 0
}
expect_abort() {   # $1 案例名；$2 stderr 要含的類別字串；$3… 額外⛔ 不得出現的字串
  local name="$1" want="$2"; shift 2
  if [ "$RC" -ne 0 ] && [ -z "$(ls -A "$C/out")" ] && [ ! -s "$C/stdout" ] \
     && grep -qF -- "中止" "$C/stderr" && grep -qF -- "$want" "$C/stderr" && no_leak "$C/stderr" "$@"; then
    pass "$name"
  else
    fail "$name（rc=$RC，輸出目錄：$(ls -A "$C/out" | tr '\n' ' ')）"
    sed 's/^/       /' "$C/stderr" >&2
  fi
}
has()   { grep -qF -- "$2" "$1"; }
line_has_both() { awk -v a="$2" -v b="$3" 'index($0, a) && index($0, b) { f = 1 } END { exit !f }' "$1"; }
check() {      # $1 案例名；其餘是要成立的指令
  local name="$1"; shift
  if "$@"; then pass "$name"; else fail "$name"; fi
}
lacks() { ! grep -qF -- "$2" "$1"; }
append_28() {  # 在 10-28 的檔（窗 101 之內）加一行
  printf '%s\n' "$1" >> "$C/logs/backend-2026-10-28.log"
}

echo "==> extract-job-logs：正常路徑"
new_case; run_tool
O="$C/out/e2.jsonl"; R="$C/out/e2_report.txt"
check "正常：結束碼 0" [ "$RC" -eq 0 ]
check "正常：輸出目錄只有兩個檔（⛔ 沒有殘留的暫存檔）" [ "$(ls -A "$C/out" | sort | tr '\n' ' ')" = "e2.jsonl e2_report.txt " ]
check "正常：兩個檔都是 mode 600" [ "$(stat -c %a "$O") $(stat -c %a "$R")" = "600 600" ]
check "正常：輸出 7 行" [ "$(wc -l < "$O")" -eq 7 ]
check "來源：依序讀 10-27、10-28、10-29 三個 UTC 日期" \
  [ "$(grep '^exec ' "$C/calls.log" | awk '{print $4}' | tr '\n' ' ')" = \
    "/app/logs/backend/backend-2026-10-27.log /app/logs/backend/backend-2026-10-28.log /app/logs/backend/backend-2026-10-29.log " ]
check "順序：L3 的 inspect 在讀完全部來源之後才呼叫" [ "$(tail -n 1 "$C/calls.log" | cut -d' ' -f1-2)" = "container inspect" ]
check "過濾：窗外的行⛔ 不在輸出" lacks "$O" "OUTSIDE"
check "對照：intraday 取最接近失敗的 104，⛔ 不取 105" line_has_both "$O" "CONTROL104-MARK" '"windows":[104]'
check "對照：105 的行⛔ 不在輸出" lacks "$O" "NOT-CONTROL-MARK"
check "對照：當天沒有失敗時取當天最早的 success（100）" line_has_both "$O" "CONTROL100-MARK" '"windows":[100]'
check "時間窗：06:30 同時起跑的兩個工作都記在同一行" line_has_both "$O" "delisting reconcile failed" '"windows":[102,106]'
check "時間：+0800 的 ts 換算正確" line_has_both "$O" "password authentication failed" '"windows":[101,106]'
check "L1：URL 的 query 換成 <query removed>" has "$O" 'C_public.jsp?<query removed>'
check "L1：只靠 L1 擋的 query 參數已拿掉" lacks "$O" "L1ONLYFAKE"
check "L1：query 裡的其他參數也拿掉" lacks "$O" "strMode=2"
check "遮罩反向：值裡剛好有 token 時，⛔ 不把後面的文字當成鍵值" has "$O" '\": context deadline exceeded"'
check "L1 反向：沒有 query 的 URL 原樣保留" has "$O" "https://example.org/plain/path"
check "L1：前面緊接 _ 的 URL 也拿掉 query" lacks "$O" "GLUEDFAKE"
check "L2 鍵值：api_key= 的值遮掉" lacks "$O" "KVFAKE77"
check "L2 鍵值：帶後綴的鍵名（secret_key:）也遮掉" lacks "$O" "KVFAKE88"
check "L2 鍵值反向：password 後面⛔ 沒有 = 或 : 時原樣保留" has "$O" "password authentication failed for user x"
check "L2 JWT：遮掉" lacks "$O" "eyJhbGciOiJIUzI1NiJ9"
check "L2 連線字串：遮掉" lacks "$O" "CONNFAKE"
check "L2 連線字串：前面緊接 _ 也遮掉" lacks "$O" "GLUEDCONN"
check "L2 URL 帳密：遮掉、保留主機" has "$O" "https://<masked>@example.com/p"
check "L2 URL 帳密：密碼⛔ 不在輸出" lacks "$O" "UIFAKE9"
check "L2 bearer：遮掉" lacks "$O" "BEARERFAKE1"
check "L2 長字串：34 字的英數混合字串遮掉" lacks "$O" "Zz9Yy8Xx7Ww6Vv5Uu4Tt3Ss2Rr1Qq0Pp9O"
check "L2 長字串反向：31 字原樣保留" has "$O" "short Ab3Cd4Ef5Gh6Ij7Kl8Mn9Op0Qr1St2U letters"
check "L2 長字串反向：只有英文字母的 40 字原樣保留" has "$O" "abcdefghijabcdefghijabcdefghijabcdefghij"
check "L2 敏感鍵：結構化欄位 token 的值遮掉" lacks "$O" "STRUCTFAKE5"
check "L2 敏感鍵反向：同層的其他欄位原樣保留" has "$O" '"count":3'
check "遮罩反向：一般錯誤訊息原樣保留" has "$O" "stock_symbol_sync has not succeeded today"
check "報告：L3 通過" has "$R" "結果：通過"
check "報告：空值的鍵列為未比對" has "$R" "空值、未比對：REDIS_PASSWORD"
check "報告：DSN 的密碼有比對" has "$R" "DATABASE_DSN 的密碼"
check "報告：沒有 log 行的窗列出來" has "$R" "沒有任何 log 行的時間窗：id=107"
check "報告：輸出的 SHA-256 相符" has "$R" "SHA-256 $(sha256sum "$O" | cut -d' ' -f1)"
check "報告：每個來源的計數" has "$R" "backend-2026-10-28.log：總行數 5、取用 4、末段未換行：完整 0、不完整而丟棄 0"
check "報告：L1 的替換次數" has "$R" "L1 URL query：2"
check "報告與 stdout ⛔ 不含假 secret" no_leak "$R"
check "stdout ⛔ 不含假 secret" no_leak "$C/stdout"

echo "==> extract-job-logs：跨 UTC 午夜的窗"
new_case
cat > "$C/windows.csv" <<'CSV'
id,job_name,status,symbols_total,symbols_failed,started_tpe,finished_tpe,error
200,intraday,failed,1,1,2026-10-29T07:59:58.000000,2026-10-29T08:00:03.000000,fetch_failed:1
CSV
run_tool
check "跨日：結束碼 0" [ "$RC" -eq 0 ]
check "跨日：讀 10-28 與 10-29 兩個檔、⛔ 不讀 10-27" \
  [ "$(grep '^exec ' "$C/calls.log" | awk '{print $4}' | tr '\n' ' ')" = \
    "/app/logs/backend/backend-2026-10-28.log /app/logs/backend/backend-2026-10-29.log " ]

echo "==> extract-job-logs：末段未換行（正在寫入的當天檔 vs. 歷史檔；review 2026-10-30）"
# ⚠️ 「正在寫入」以工具讀取當下的 UTC 日期判斷，所以當天檔的 fixture 用今天的 UTC 日期產生
# （剛好跨 UTC 午夜執行時會誤判成歷史檔而中止，屬已知限制）。
TODAY="$(date -u +%F)"
new_case
cat > "$C/windows.csv" <<CSV
id,job_name,status,symbols_total,symbols_failed,started_tpe,finished_tpe,error
300,intraday,failed,1,1,${TODAY}T09:00:00.000000,${TODAY}T09:00:05.000000,fetch_failed:1
CSV
printf '%s\n' "{\"level\":\"error\",\"ts\":\"${TODAY}T01:00:02.000Z\",\"msg\":\"today fetch failed TODAY-MARK\"}" \
  > "$C/logs/backend-$TODAY.log"
printf '%s' "{\"level\":\"info\",\"ts\":\"${TODAY}T01:00:03" >> "$C/logs/backend-$TODAY.log"
run_tool
check "當天檔的末段不完整：結束碼 0" [ "$RC" -eq 0 ]
check "當天檔的末段不完整：丟掉並計數" \
  has "$C/out/e2_report.txt" "backend-$TODAY.log：總行數 1、取用 1、末段未換行：完整 0、不完整而丟棄 1"
check "當天檔：完整的行照常保留" has "$C/out/e2.jsonl" "TODAY-MARK"
new_case
printf '%s' '{"level":"error","ts":"2026-10-29T05:30:06.000Z","msg":"LAST-LINE-ERROR-MARK"}' >> "$C/logs/backend-2026-10-29.log"
run_tool
check "歷史檔（查詢範圍的最後一天）末段完整但沒換行：結束碼 0" [ "$RC" -eq 0 ]
check "歷史檔的末段完整：照常保留、⛔ 不丟掉" line_has_both "$C/out/e2.jsonl" "LAST-LINE-ERROR-MARK" '"windows":[103]'
check "歷史檔的末段完整：報告計數" \
  has "$C/out/e2_report.txt" "backend-2026-10-29.log：總行數 4、取用 3、末段未換行：完整 1、不完整而丟棄 0"
new_case
printf '%s' '{"level":"error","ts":"2026-10-29T05:30:07' >> "$C/logs/backend-2026-10-29.log"; run_tool
expect_abort "歷史檔（查詢範圍的最後一天）末段不完整 → 中止" "沒有換行結尾而且不完整"
new_case
printf '%s' '{"level":"error","ts":"2026-10-28T22:30:12' >> "$C/logs/backend-2026-10-28.log"; run_tool
expect_abort "歷史檔（中間的日期）末段不完整 → 中止" "沒有換行結尾而且不完整"

echo "==> extract-job-logs：來源讀取失敗"
new_case; rm "$C/logs/backend-2026-10-27.log"; run_tool
expect_abort "來源缺檔 → 中止、⛔ 沒有輸出" "讀取失敗"
check "來源缺檔：⛔ 沒有呼叫 L3 的 inspect" lacks "$C/calls.log" "container inspect"
new_case; FAKE_EXEC_FAIL=backend-2026-10-28.log run_tool
expect_abort "內容讀完才回非零 → 中止、⛔ 不採用已讀到的部分" "讀取失敗"

echo "==> extract-job-logs：壞資料（錯誤訊息⛔ 不洩漏）"
new_case; append_28 'not-json LEAKFAKE-BAD-1 {"msg":"stock symbol sync MARK"'; run_tool
expect_abort "不是 JSON 的行（含假 secret）→ 中止、訊息⛔ 不帶原文" "不是 UTF-8 的 JSON"
new_case; append_28 '{"ts":"bogus LEAKFAKE-TS-2","msg":"x MARK"}'; run_tool
expect_abort "ts 無法解析（含假 secret）→ 中止、訊息⛔ 不帶原文" "ts 欄位無法解析"
new_case; printf '\xff\xfe LEAKFAKE-BAD-1\n' >> "$C/logs/backend-2026-10-27.log"; run_tool
expect_abort "不是 UTF-8 的行 → 中止" "不是 UTF-8 的 JSON"

echo "==> extract-job-logs：L3（遮罩漏掉的已知值）"
new_case; append_28 '{"level":"error","ts":"2026-10-28T22:30:12.000Z","msg":"upstream echoed finmind-FAKE-3 here"}'; run_tool
expect_abort "L3 原值 → 中止、⛔ 沒有輸出（連暫存檔也沒有）" "FINMIND_API_KEY"
new_case
sed -i 's#AUTH_JWT_SECRET=jwt-secret-FAKE-1#AUTH_JWT_SECRET=j/w+t s=1\&x#' "$C/env.json"
append_28 '{"level":"error","ts":"2026-10-28T22:30:12.000Z","msg":"saw j%2Fw%2Bt%20s%3D1%26x"}'; run_tool
expect_abort "L3 URL 編碼值 → 中止" "AUTH_JWT_SECRET"
new_case
sed -i 's#"REDIS_PASSWORD="#"REDIS_PASSWORD=r\\"q\\\\p"#' "$C/env.json"
append_28 '{"level":"error","ts":"2026-10-28T22:30:12.000Z","msg":"auth r\\\"q\\\\p failed"}'; run_tool
expect_abort "L3 JSON 跳脫後的值（log 文字裡內嵌的 JSON）→ 中止" "REDIS_PASSWORD"
new_case
sed -i 's#"REDIS_PASSWORD="#"REDIS_PASSWORD=31415926"#' "$C/env.json"
append_28 '{"level":"error","ts":"2026-10-28T22:30:12.000Z","msg":"retry","attempt_id":31415926}'; run_tool
expect_abort "L3 非字串欄位裡的值（只有輸出比對擋得到）→ 中止" "遮罩後的輸出裡仍有 REDIS_PASSWORD"
new_case; append_28 '{"level":"error","ts":"2026-10-28T22:30:12.000Z","msg":"login dsnpw-FAKE-2 rejected"}'; run_tool
expect_abort "L3 DSN 拆出的密碼 → 中止" "DATABASE_DSN 的密碼"
new_case; sed -i 's#FUGLE_API_KEY=fugle-FAKE-4#FUGLE_API_KEY=partial#' "$C/env.json"; run_tool
expect_abort "L3 也檢查報告（值只出現在報告裡）→ 中止" "報告裡出現 FUGLE_API_KEY" partial

echo "==> extract-job-logs：含空白與分隔符的值（review 2026-10-30）"
new_case
append_28 '{"level":"error","ts":"2026-10-28T22:30:13.000Z","msg":"quoted api_key=\"qv9&TAIL QUOTED,x;y\" single password='"'"'sq pass&TAILSQ'"'"' esc {\\\"token\\\":\\\"esc v&TAILESC\\\"} done"}'
run_tool
O="$C/out/e2.jsonl"
check "引號：結束碼 0" [ "$RC" -eq 0 ]
check "雙引號內的值（含空白、&、, ;）整段遮掉" lacks "$O" "TAIL QUOTED"
check "雙引號：收尾引號之後原樣保留" has "$O" 'api_key=\"<masked>\" single'
check "單引號內的值整段遮掉" lacks "$O" "TAILSQ"
check "Go %q 的跳脫引號內的值整段遮掉" lacks "$O" "TAILESC"
check "引號：後面的文字原樣保留" has "$O" "} done"
new_case
sed -i 's#FINMIND_API_KEY=finmind-FAKE-3#FINMIND_API_KEY=prefix9\&TAIL_CANARY_7#' "$C/env.json"
append_28 '{"level":"error","ts":"2026-10-28T22:30:12.000Z","msg":"req api_key=prefix9&TAIL_CANARY_7 failed"}'; run_tool
expect_abort "已知值只被遮住一部分（未加引號、含 &）→ L3 中止" "沒有完整蓋住原始 log 裡的 FINMIND_API_KEY"
new_case
sed -i 's#FINMIND_API_KEY=finmind-FAKE-3#FINMIND_API_KEY=pre fix9\&TAIL_CANARY_7#' "$C/env.json"
append_28 '{"level":"error","ts":"2026-10-28T22:30:12.000Z","msg":"req api_key=pre fix9&TAIL_CANARY_7 failed"}'; run_tool
expect_abort "已知值只被遮住一部分（未加引號、含空白）→ L3 中止" "沒有完整蓋住原始 log 裡的 FINMIND_API_KEY"
new_case
sed -i 's#FINMIND_API_KEY=finmind-FAKE-3#FINMIND_API_KEY=prefix9\&TAIL_CANARY_7#' "$C/env.json"
append_28 '{"level":"error","ts":"2026-10-28T22:30:12.000Z","msg":"req api_key=\"prefix9&TAIL_CANARY_7\" failed"}'; run_tool
check "已知值在引號內：整段遮掉、L3 通過（⛔ 不誤判）" [ "$RC" -eq 0 ]
check "已知值在引號內：輸出⛔ 不含尾段" lacks "$C/out/e2.jsonl" "TAIL_CANARY"

echo "==> extract-job-logs：取不到 secret"
new_case; sed -i 's#,"FUGLE_API_KEY=fugle-FAKE-4"##' "$C/env.json"; run_tool
expect_abort "缺少 secret 環境變數 → 中止" "缺少 FUGLE_API_KEY"
new_case; FAKE_INSPECT_FAIL=1 run_tool
expect_abort "inspect 失敗 → 中止" "讀不到容器的環境變數"
new_case; sed -i 's#postgres://trading_username:dsnpw-FAKE-2@#postgres://trading_username@#' "$C/env.json"; run_tool
expect_abort "DSN 拆不出密碼 → 中止" "拆不出密碼"

echo "==> extract-job-logs：輸入與輸出檔"
new_case; printf '%s\n' '300,intraday,running,0,0,2026-10-29T11:00:00.000000,,' >> "$C/windows.csv"; run_tool
expect_abort "running 的紀錄 → 中止" "沒有結束時間"
check "running：⛔ 沒有呼叫 docker" [ ! -e "$C/calls.log" ]
new_case; sed -i '1s/^id,/run_id,/' "$C/windows.csv"; run_tool
expect_abort "表頭不符 → 中止" "表頭"
new_case; echo keep > "$C/out/e2.jsonl"; run_tool
kept_existing() {
  [ "$RC" -ne 0 ] && [ "$(cat "$C/out/e2.jsonl")" = keep ] && [ "$(ls -A "$C/out")" = e2.jsonl ] && [ ! -e "$C/calls.log" ]
}
check "輸出檔已存在 → 中止、原檔不變、⛔ 沒有呼叫 docker" kept_existing

echo "==> test-extract-job-logs：ok $PASS、FAIL $FAIL"
[ "$FAIL" -eq 0 ]
