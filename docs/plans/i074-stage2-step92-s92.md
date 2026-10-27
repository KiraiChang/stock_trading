# Stage 2 步驟 ⑨-2 的簡易指令 `s92` v1（2026-10-27，✅ 確認（D1～D4 照建議）；實作 ✅ review 通過並 commit `832f678`；2026-10-27 用於 ⑨-2）

需求確認（2026-10-27）：比照 ⑨-1 的 `s91`，把 ⑨-2 要貼的長區塊（`docs/issue.md` I-074「Stage 2 步驟 ⑨-2 細部計畫 v4」的「二」precheck、「三」的 ①②③）
包成 **repo 外**的子指令腳本 `s92`（`pre`／`go`／`st`／`stop`）。開不開跑仍由使用者看輸出決定（⛔ 不把開跑的條件改成程式判斷）。`s92` 是可選的：照舊貼區塊也可以。
✅ 2026-10-27 使用者確認本計畫，D1～D4 全部照建議。依 CLAUDE.md，Claude ⛔ 不 commit。文中的「⑨-2「二」」「⑨-1 的 `s91`」等都指 `docs/issue.md` I-074 的章節。

## 〇、唯讀盤點（2026-10-27，HEAD `eb855b0`）

| 項目 | 現況 |
|---|---|
| 要包的區塊 | ⑨-2「二」`i074_stage2_step92_precheck_003`（P1～P8；`R91`／`E91` 是佔位）、「三」`i074_stage2_step92_launch_002`（① 失敗即停的啟動、② 狀態、③ 安全中止） |
| 基礎 | ⑨-1 的 `s91` v5（`i074_stage2_s91_script_003`，五輪 review；`~/.local/bin/s91`，2026-10-24 實際用於 ⑨-1、事後對照 ok） |
| 與 ⑨-1 的差異 | 入口 `scripts/i074-stage2-sizing.sh --formal`（⛔ 沒有 `--replay-compute`）；wrapper 以固定的環境啟動（`env -u MEM …`、`MEM_MIN_MB=447` 等，⑨-2「三」的 D3）；work 在 `~/i074_stage2_sizing/formal-92-<UTC>`、控制目錄 `/dev/shm/s92-<NAME>`；sizing 的 commit point（發布 `<work>/freeze_record.json`）之後忽略 INT／TERM、⛔ 不中止（⑨-2「三」） |
| ⑨-2 precheck 的保存 | ⛔ 沒有：區塊只印在終端機，註解寫「輸出貼進紀錄」；P4 要求「輸出原樣記進本次紀錄」。⑨-1 的 precheck（執行紀錄格式 v2）有 tmpfs 保存、btime、D1 的證據與「五」7 的基準 |
| ⑨-2「五」6 | 要求「真正 repo 的 worktree 登記數不變；`/tmp` 的新項目逐一歸因」——⑨-2 的 precheck ⛔ 沒有開跑之前的基準 |
| 安裝的位置 | `~/.local/bin` 已有 `s91`（mode 755）；`s92` 尚未存在；`/usr/bin/flock` 可用 |
| ⑨-2 的時序 | ⑨-2 要在「⑨-1 之後的最後一次 commit」之後、以 ⑩ 要用的 HEAD 執行；本計畫的文件 commit 只動 `docs/`（P1 照樣成立），必須在 ⑨-2 **之前** commit——核對指令要從 HEAD 的本檔抽出腳本 |
| 磁碟的餘裕 | 本檔約 30 KB，`code_worktree` 隨 HEAD 的 tree 增加同一個量級；⑨-1 實測的餘裕 4.6 MiB |

## 一、目標、範圍與⛔ 不做

| 項目 | 內容 |
|---|---|
| 目標 | 四個短指令取代貼區塊：`s92 pre`、`s92 go`、`s92 st`、`s92 stop`；從檔案執行，⛔ 不再有貼上的風險（`R91`／`E91` 的替換、折行、`read` 讀走下一行、JWT 被當成指令）；`go`、`stop` 的每一步都 fail-closed，並行的兩個也⛔ 不會雙啟動、⛔ 不會送兩次；precheck 的輸出自動保存；執行之前可以獨立核對腳本與 HEAD 的本檔相同 |
| 範圍 | `~/.local/bin/s92`（新檔，mode 755）；本檔（腳本、核對指令與測試）；`docs/issue.md` ⑨-2「二」「三」各加一行指標、I-074 狀態列與 roadmap 的指標 |
| ⛔ 不做 | ⛔ 不改 ⑨-2 的 precheck、wrapper、②③、開跑的條件與判讀（⑨-2 的區塊⛔ 不動）；⛔ 不自動判斷開跑的條件；⛔ 不改 repo 的程式；⛔ 不在真正的 repo 跑 `s92 go`（測試用只把 `cd` 換成假 repo 的副本）；⛔ 不動 `s91` |

## 二、設計（`s91` v5 的骨架，換成 ⑨-2 的區塊）

| 子指令 | 內容 |
|---|---|
| `s92 pre` | ⑨-2「二」的區塊（`_003`，**逐字**；`R91` ＝ `cbde340…`、`E91` ＝ `d3df284…` 已代入）。前後加上 ⑨-1「二」的四段（**逐字**取自 ⑨-1 的 `_002`，D1）：`mktemp` 的 `/dev/shm/s92-pre-<UTC>-XXXXXX.txt` 與 `tee`、開始／結束標記與 `btime`、D1 的證據（`claude` 的程序數 0 才印 ok；`codex` 只記錄）、「五」6 的基準（worktree 登記數、`/tmp` 的清單）；最後驗證保存成功才印「PRE 保存 ok」 |
| `s92 go` | 與 `s91 go` 相同：全域的啟動鎖 `/dev/shm/s92.lock`（`flock -n`、⛔ 不傳給 wrapper）；回答 `yes` 前後各一次與 P7 相同的程序檢查；「三」的 ① 每一步失敗即停；`run.tsv` 多一欄 `s92_sha256`；wrapper 的內容**逐字**取自 ⑨-2「三」的 ①（固定的 `MEM*` 環境）；啟動之後最多 15 秒驗證 5 個身分欄位、`s92_sha256`、wrapper 與 harness 都還在 |
| `s92 st [NAME]` | 控制目錄的來源驗證（`^formal-92-[0-9]{8}T[0-9]{6}Z$`、⛔ 不是 symlink、owner、mode 700、`run.tsv` 的四欄與目錄一致、`work` ＝ `$HOME/i074_stage2_sizing/<name>`）→「三」的 ② 的函式與顯示（**逐字**）；另顯示 freeze record 是否已發布（D2）與中止的紀錄 |
| `s92 stop [NAME]` | 與 `s91 stop` 相同（claim ＝ `set -C` 建立 `stop_attempt`、PID／starttime 只取一次、PID ＋ starttime ＋ cmdline 含 `i074-stage2-sizing.sh`、`kill` 的結束碼、`stop_sent`／`stop_failed`、每個控制目錄只嘗試一次）；另外 **`<work>/freeze_record.json` 已存在 → 拒絕**（commit point 已過，⑨-2「三」：⛔ 不中止；D2） |
| 核對（獨立） | 「三之二」的一行指令：從 `git show HEAD:docs/plans/i074-stage2-step92-s92.md` 抽出「三之一」的腳本，與 `~/.local/bin/s92` `cmp`，相同才印 SHA-256 與 ok；跑完之後 Claude 再以 `run.tsv` 的 `s92_sha256` 對照 HEAD 的本檔 |

`s91` 與 `s92` 的差異只有上表的四處（`pre` 的內容、wrapper、名稱與路徑、`st`／`stop` 的 freeze record）；其他每一行（程序檢查、鎖、claim、來源驗證的寫法）都與 `s91` v5 相同（「七」以 `diff` 確認）。

## 三之一、腳本（`~/.local/bin/s92`）

```text id="i074_stage2_s92_script_001"
#!/usr/bin/env bash
# s92：I-074 Stage 2 ⑨-2 的簡易指令（docs/plans/i074-stage2-step92-s92.md）
#   s92 pre          precheck（＝ ⑨-2「二」的區塊，逐字；另存進 tmpfs）：P4 的提示出現之後，先打一個空白、再貼上 JWT
#   s92 go           啟動（＝「三」的 ①，每一步失敗即停、啟動後驗證）：precheck 剛跑完、開跑的條件都成立才用
#   s92 st [NAME]    狀態（＝「三」的 ②，逐字；另顯示 freeze record）：/dev/shm 只有一個控制目錄時可以省略 NAME
#   s92 stop [NAME]  安全中止（與「三」的 ③ 相同的身分條件）：只在必要時；每個控制目錄只能嘗試一次；freeze record 發布之後⛔ 不送
# ⛔ 不放進 repo（P1 的 clean、HEAD 的 tree 大小）；執行之前先用計畫書裡的核對指令確認與 HEAD 的文件相同。

usage() { sed -n '2,6p' "$0" | sed 's/^# \{0,1\}//'; }
NAME_RE='^formal-92-[0-9]{8}T[0-9]{6}Z$'
LOCK=/dev/shm/s92.lock                                    # 啟動鎖（空檔；名稱⛔ 不是 i074-*、⛔ 不是控制目錄的形狀）

# ── ② 的函式（逐字）
v()     { awk -F'\t' -v k="$1" '$1 == k { print $2 }' "$CTL/run.tsv"; }
st()    { awk '{ sub(/.*\) /, ""); print $20 }' "/proc/$1/stat" 2>/dev/null; }
alive() { [ -n "$1" ] && [ -n "$2" ] && [ "$(st "$1")" = "$2" ]; }      # pid ＋ starttime 都相符才算同一個程序
one() { [ "$(awk -F'\t' -v k="$1" '$1 == k' "$CTL/run.tsv" | wc -l)" = 1 ]; }   # run.tsv 的欄位恰好一行

# 控制目錄：給了 NAME 就用它，否則 /dev/shm 必須恰好一個 s92-formal-92-*（⛔ 不猜）；再驗證來源
s92_ctl() {
  local d n b k
  if [ -n "${1:-}" ]; then
    [[ "$1" =~ $NAME_RE ]] || { echo "NAME 的格式不對：「$1」（formal-92-YYYYMMDDTHHMMSSZ）" >&2; return 1; }
    CTL="/dev/shm/s92-$1"
  else
    d="$(find /dev/shm -maxdepth 1 -name 's92-formal-92-*')" || { echo "find 失敗：⛔ 不猜控制目錄" >&2; return 1; }
    n="$(grep -c . <<< "$d")"
    [ "$n" = 1 ] || { echo "/dev/shm 的控制目錄有 $n 個：請指定 NAME" >&2; [ -n "$d" ] && sed 's|^/dev/shm/s92-|  |' <<< "$d" >&2; return 1; }
    CTL="$d"
  fi
  b="${CTL#/dev/shm/s92-}"
  [[ "$b" =~ $NAME_RE ]] || { echo "控制目錄的名稱不對：$CTL" >&2; return 1; }
  [ -d "$CTL" ] && [ ! -L "$CTL" ] || { echo "控制目錄不存在或是 symlink：$CTL" >&2; return 1; }
  [ "$(stat -c '%u %a' "$CTL")" = "$(id -u) 700" ] || { echo "控制目錄的 owner／mode 不對（$(stat -c '%U %a' "$CTL")，要 $(id -un) 700）：$CTL" >&2; return 1; }
  [ -f "$CTL/run.tsv" ] && [ ! -L "$CTL/run.tsv" ] || { echo "run.tsv 不存在、不是一般檔或是 symlink：$CTL" >&2; return 1; }
  for k in name expected work launched_utc; do one "$k" || { echo "run.tsv 的 $k 缺失或重複：$CTL" >&2; return 1; }; done
  [ "$(v name)" = "$b" ] && [[ "$(v expected)" =~ ^[0-9a-f]{40}$ ]] && [ "$(v work)" = "$HOME/i074_stage2_sizing/$b" ] \
    && [[ "$(v launched_utc)" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$ ]] \
    || { echo "run.tsv 的 name／expected／work／launched_utc 與控制目錄不符或格式不對：$CTL" >&2; return 1; }
}

s92_pre() {
# ⑨-2「二」的區塊（_003，逐字；R91／E91 已代入）。前後加上 ⑨-1「二」的保存（tmpfs 的 $PRE）、btime、D1 的證據與「五」6 的基準——都逐字取自 ⑨-1 的 _002
PRE="$(mktemp "/dev/shm/s92-pre-$(date -u +%Y%m%dT%H%M%SZ)-XXXXXX.txt")"; RM=$?
{
echo "PRE $PRE"; date -u +'precheck 開始 %FT%TZ'; BT="$(awk '$1 == "btime" { print $2 }' /proc/stat)"; RBT=$?; echo "btime $BT"   # 判斷之後有沒有重開機
# P1～P10 的確認（使用者在自己的終端機執行；**只貼這一個區塊**——P4 的 read 會等你貼上 JWT（前端登入後取得，或 POST /api/v1/auth/login）；輸出貼進紀錄）：
#   整段包在 { } 裡——shell 先讀完整段才執行，read 執行時輸入佇列裡⛔ 沒有下一行可以被讀走（2026-10-16 修改）
{
cd /home/dev/workspace/stock_trading
IMG=sha256:2a90ad1c1dd801d59373988a4d84afe1437cbee62ccab3709b06cfa8f7da5027
R91=cbde34098c893dbf958ce9ab5a5fab26e63fb34e; E91=d3df284f691d05ff9468f43b7f40397f5cd15df8                    # 都取自「⑨-1 正式驗收結果」一節（P1、P10）
git merge-base --is-ancestor 53538a3 HEAD && git merge-base --is-ancestor c0cf0de HEAD \
  && git merge-base --is-ancestor ec23e4a HEAD && git merge-base --is-ancestor "$R91" HEAD \
  && git merge-base --is-ancestor "$E91" HEAD && echo "P1 祖先 ok"                                      # E91 也必須是 HEAD 的祖先
GS="$(git status --porcelain -- scripts python .gitattributes)" && [ -z "$GS" ] && echo "P1 clean ok"        # git 失敗 ⛔ 不能當作 clean
DD="$(git diff --name-only "$E91" HEAD -- . ':(exclude)docs' ':(exclude)CLAUDE.md')"; RDD=$?
printf 'P1 ⑨-1 之後文件（docs/ 與根目錄的 CLAUDE.md）以外的變更：%s（git diff 的 rc %s）\n' "${DD:-無}" "$RDD"
[ "$RDD" = 0 ] && [ -z "$DD" ] && echo "P1 ⑨-1 之後只動文件 ok"                                              # E91 錯 ⛔ 不能當作只動文件
git diff --quiet 53538a3 HEAD -- python/baselines/i074_stage2/counterfactual_e1cbbbd.patch \
  python/baselines/i074_stage2/tooling_e1cbbbd.patch && echo "P2 ok"
sha256sum python/baselines/i074_stage2/counterfactual_e1cbbbd.patch python/baselines/i074_stage2/tooling_e1cbbbd.patch
docker image inspect "$IMG" > /dev/null && echo "P3 image ok"
ID="${XDG_DATA_HOME:-$HOME/.local/share}/stock_trading/i074_stage2/run_identity.json"
python3 -c 'import json,sys; sys.exit(0 if json.load(open(sys.argv[1]))["expected_image_id"] == sys.argv[2] else 1)' "$ID" "$IMG" \
  && echo "P3 identity ok"                                                                                   # 第二輪 review：以結束碼判定
sha256sum "$ID"                                                                                              # P3：a02bf3bd…
docker inspect stock_trading-backend-1 --format '{{range .Config.Env}}{{println .}}{{end}}' | grep -E '^[A-Z_]+_(CRON|ENABLED)='   # P4
BIP="$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}} {{end}}' stock_trading-backend-1 | cut -d' ' -f1)"
echo "P4：先打一個空白、再貼上 JWT，按 Enter（⛔ 不回顯、⛔ 不寫進檔案或紀錄）"; read -rs TOKEN   # 空白：萬一沒跑到 read，那一行⛔ 不進 history
curl -fsS -H "Authorization: Bearer $TOKEN" "http://$BIP:8080/api/v1/scheduler/status" | python3 -c 'import json, sys
jobs = json.load(sys.stdin)["jobs"]
for j in sorted(jobs, key=lambda j: j["job_name"]):
    print("P4", j["job_name"], j["status"], j.get("started_at", "-"), j.get("finished_at", "-"), "⚠️ 仍在執行" if j["status"] == "running" else "")
print("P4 running 的數量：", sum(j["status"] == "running" for j in jobs))'; unset TOKEN       # 輸出原樣記進本次紀錄
awk '/^MemAvailable:/ { m = int($2 / 1024); f = 1 } END { printf "P5 MemAvailable %d MiB（門檻 700）\n", m; exit !(f && m >= 700) }' \
  /proc/meminfo && echo "P5 ok"                                                                              # 第二輪 review：以結束碼判定
df -h / /dev/shm                                                                                             # P6
S7="$(find /dev/shm -maxdepth 1 -name 'i074-*')"; RS=$?; C7="$(docker ps -a --filter name=i074sz- -q)"; RC=$?  # P7：find ⛔ 不受 zsh 的 glob 影響
PP="$(ps -eo pid,args)"; RP=$?; P7="$(awk '/i074-stage2-(acceptance|sizing)\.sh/' <<< "$PP")"; RA7=$?         # ps 先完整取值，再交給 awk
printf 'P7 S：%s\nP7 容器：%s\nP7 程序：%s\nP7 rc：find %s、docker ps %s、ps %s、awk %s\n' "${S7:-無}" "${C7:-無}" "${P7:-無}" "$RS" "$RC" "$RP" "$RA7"
[ "$RS" = 0 ] && [ "$RC" = 0 ] && [ "$RP" = 0 ] && [ "$RA7" = 0 ] && [ -n "$PP" ] && [ -z "$S7" ] && [ -z "$C7" ] && [ -z "$P7" ] \
  && echo "P7 ok"                                                                                            # 任一個指令失敗 ⛔ 不能當作沒有殘留
git rev-parse HEAD                                                                                           # P8
}
PD="$(ps -eo pid,rss,comm)"; RD=$?                                                                           # D1：ps 先完整取值，再交給 awk
DL="$(awk '$3 ~ /^(claude|codex)/ { print "D1", $0 }' <<< "$PD")"; RA=$?; NC="$(awk '$3 == "claude" { n++ } END { print n + 0 }' <<< "$PD")"; RN=$?
printf '%s\nD1 rc：ps %s、awk %s／%s；claude 的程序數 %s\n' "${DL:-D1 （⛔ 沒有 claude／codex 的程序）}" "$RD" "$RA" "$RN" "${NC:-無}"   # codex 只記錄（可以存在、⛔ 不使用）
[ "$RD" = 0 ] && [ "$RA" = 0 ] && [ "$RN" = 0 ] && [ -n "$PD" ] && [ "$NC" = 0 ] && echo "D1 Claude 已結束 ok"  # 任一個指令失敗 ⛔ 不能當作已結束
WL="$(git worktree list --porcelain)"; RW=$?; NW="$(grep -c '^worktree ' <<< "$WL")"; RG=$?                   # 「五」7 的基準
TL="$(ls -A /tmp)"; RL=$?; TB="$(sed 's|^|基準 /tmp：|' <<< "$TL")"; RE=$?
printf '基準 worktree 登記數 %s\n%s\n基準 rc：git %s、grep %s、ls %s、sed %s、btime 的 awk %s\n' "${NW:-無}" "$TB" "$RW" "$RG" "$RL" "$RE" "$RBT"
[ "$RW" = 0 ] && [ "$RG" = 0 ] && [ "$RL" = 0 ] && [ "$RE" = 0 ] && [ "$RBT" = 0 ] && [[ "$NW" =~ ^[1-9][0-9]*$ ]] && [[ "$BT" =~ ^[1-9][0-9]*$ ]] \
  && [ -n "$TB" ] && echo "基準 ok"                                                                          # 任一個指令失敗 ⛔ 不能當作有基準
date -u +'precheck 結束 %FT%TZ'
} 2>&1 | tee "$PRE"; RT=$?; [ "$RM" = 0 ] && [ "$RT" = 0 ] && [ -s "$PRE" ] && [ "$(head -n 1 "$PRE")" = "PRE $PRE" ] \
  && [ "$(grep -c '^precheck 開始 ' "$PRE")" = 1 ] && [ "$(grep -c '^precheck 結束 ' "$PRE")" = 1 ] && grep -q '^precheck 結束 ' <<< "$(tail -n 1 "$PRE")" && echo "PRE 保存 ok：$PRE"   # ⛔ 沒有這一行就⛔ 不開跑
}

# 與 P7 相同的程序檢查：⛔ 沒有 acceptance／sizing 的程序、而且 ps 與 awk 都成功，才回 0
s92_procs() {
  local PP RP P7 RA7
  PP="$(ps -eo pid,args)"; RP=$?; P7="$(awk '/i074-stage2-(acceptance|sizing)\.sh/' <<< "$PP")"; RA7=$?
  [ "$RP" = 0 ] && [ "$RA7" = 0 ] && [ -n "$PP" ] && [ -z "$P7" ] && return 0
  echo "已經有 acceptance／sizing 的程序，或檢查本身失敗（ps $RP、awk $RA7）：⛔ 不啟動" >&2; [ -n "$P7" ] && printf '%s\n' "$P7" >&2; return 1
}

s92_go() {
  local ans SELF SHA wp i k f
  exec 9>> "$LOCK" && /usr/bin/flock -n 9 || { echo "另一個 s92 go 正在執行（$LOCK）：⛔ 不啟動" >&2; return 1; }   # 持有到驗證完成；⛔ 不傳給 wrapper
  s92_procs || return 1
  read -r -p "precheck 剛跑完、開跑的條件都成立了嗎（每一個 ok、P4 的每一列、P6 的 df 都看過）？輸入 yes 開跑：" ans
  [ "$ans" = yes ] || { echo "⛔ 沒有開跑" >&2; return 1; }
  s92_procs || return 1                                                                # 回答之前可能有人另外啟動
  SELF="$(readlink -f -- "$0")" && SHA="$(sha256sum < "$SELF" | cut -d' ' -f1)" && [[ "$SHA" =~ ^[0-9a-f]{64}$ ]] \
    || { echo "算不出 s92 的 SHA-256：⛔ 不啟動" >&2; return 1; }
  # ── 「三」的 ①：每一步失敗即停；wrapper 的內容逐字
  cd /home/dev/workspace/stock_trading || { echo "cd 失敗：⛔ 不啟動" >&2; return 1; }
  EXPECTED="$(git rev-parse --verify HEAD)" && [[ "$EXPECTED" =~ ^[0-9a-f]{40}$ ]] || { echo "git rev-parse 失敗：⛔ 不啟動" >&2; return 1; }
  NAME="formal-92-$(date -u +%Y%m%dT%H%M%SZ)" && [[ "$NAME" =~ $NAME_RE ]] || { echo "NAME 不對（「$NAME」）：⛔ 不啟動" >&2; return 1; }
  WORK="$HOME/i074_stage2_sizing/$NAME"; CTL="/dev/shm/s92-$NAME"
  [ ! -e "$WORK" ] && [ ! -L "$WORK" ] || { echo "work 目錄已存在：$WORK：⛔ 不啟動" >&2; return 1; }
  mkdir -m 700 "$CTL" || { echo "mkdir 失敗：⛔ 不碰既有的 $CTL、⛔ 不啟動" >&2; return 1; }
  printf 'expected\t%s\nname\t%s\nwork\t%s\nlaunched_utc\t%s\ns92_sha256\t%s\n' "$EXPECTED" "$NAME" "$WORK" "$(date -u +%FT%TZ)" "$SHA" > "$CTL/run.tsv" \
    || { echo "寫 run.tsv 失敗：$CTL 只建了一半、⛔ 沒有啟動" >&2; return 1; }
  cat > "$CTL/wrapper.sh" <<'WRAP' || { echo "寫 wrapper.sh 失敗：$CTL 只建了一半、⛔ 沒有啟動" >&2; return 1; }
ctl="$1"; work="$2"
st() { awk '{ sub(/.*\) /, ""); print $20 }' "/proc/$1/stat" 2>/dev/null; }   # /proc/<pid>/stat 的 starttime（第 22 欄）
printf 'wrapper_pid\t%s\nwrapper_start\t%s\nwrapper_pgid\t%s\n' "$$" "$(st $$)" "$(ps -o pgid= -p $$ | tr -d ' ')" >> "$ctl/run.tsv"
env -u MEM -u CPUS -u TOOLING_PATCH -u REPLAY_DRY_RUN -u PY_IMAGE -u I074_SIZING_FAULT \
    -u SIZING_SNAPSHOT -u SIZING_PROFILE -u SIZING_REPLAY_MODE -u SIZING_REPLAY_COMPUTE \
    MEM_MIN_MB=447 MEM_RESERVE_MB=150 MEM_FORCE=0 MEM_STRICT=0 \
    REPLAY_IMAGE_ID=sha256:2a90ad1c1dd801d59373988a4d84afe1437cbee62ccab3709b06cfa8f7da5027 \
  scripts/i074-stage2-sizing.sh --formal --work-dir "$work" < /dev/null &      # 固定的環境（D3；下表「啟動的環境」）
hp=$!
printf 'harness_pid\t%s\nharness_start\t%s\n' "$hp" "$(st "$hp" || true)" >> "$ctl/run.tsv"
wait "$hp"; rc=$?
printf '%s\t%s\n' "$rc" "$(date -u +%FT%TZ)" > "$ctl/rc.tmp" && mv "$ctl/rc.tmp" "$ctl/rc"   # rc marker（原子寫入）
WRAP
  setsid nohup bash "$CTL/wrapper.sh" "$CTL" "$WORK" > "$CTL/log" 2>&1 < /dev/null 9>&- &
  wp=$!
  # ── 啟動之後的驗證（最多 15 秒）：5 個身分欄位各恰好一行、都是正整數；wrapper_pid ＝ 這次啟動的 PID；wrapper 與 harness 都還在
  for i in $(seq 1 15); do
    sleep 1
    [ -e "$CTL/rc" ] && { cat "$CTL/run.tsv"; echo "harness 已經結束（rc $(cut -f1 "$CTL/rc")）：開跑失敗——交給 Claude 判讀，⛔ 不要再啟動" >&2; return 1; }
    f=0; for k in wrapper_pid wrapper_start wrapper_pgid harness_pid harness_start; do one "$k" && [[ "$(v "$k")" =~ ^[1-9][0-9]*$ ]] || f=1; done
    [ "$f" = 0 ] && break
  done
  cat "$CTL/run.tsv"
  [ "$f" = 0 ] || { echo "run.tsv 在 15 秒內⛔ 沒有寫齊：可能仍有程序在跑——⛔ 不要再啟動，用 s92 st 查看、交給 Claude" >&2; return 1; }
  s92_ctl "$NAME" && one s92_sha256 && [ "$(v s92_sha256)" = "$SHA" ] \
    && [ "$(v wrapper_pid)" = "$wp" ] && alive "$(v wrapper_pid)" "$(v wrapper_start)" && alive "$(v harness_pid)" "$(v harness_start)" \
    || { echo "控制目錄、s92_sha256 或 wrapper／harness 的身分無法確認：⛔ 不要再啟動，用 s92 st 查看、交給 Claude" >&2; return 1; }
  echo "已啟動：NAME＝$NAME（之後用 s92 st 查看）" >&2
}

s92_st() {
  s92_ctl "${1:-}" || return 1
  echo "CTL：$CTL"
echo "harness：$(alive "$(v harness_pid)" "$(v harness_start)" && echo 執行中 || echo 已結束)" \
     "wrapper：$(alive "$(v wrapper_pid)" "$(v wrapper_start)" && echo 執行中 || echo 已結束)" \
     "rc：$(cut -f1 "$CTL/rc" 2>/dev/null || echo 無)"
tail -3 "$CTL/log"
  echo "freeze record：$( [ -e "$(v work)/freeze_record.json" ] && echo "已發布（commit point 已過：⛔ 不中止）" || echo 尚未發布)"
  if [ -e "$CTL/stop_attempt" ]; then
    echo "中止：嘗試 $(cut -f1 "$CTL/stop_attempt")；$( [ -e "$CTL/stop_sent" ] && echo "已送出 $(cat "$CTL/stop_sent")" || echo "未送出（$(cut -f2 "$CTL/stop_failed" 2>/dev/null || echo 結果不明)）")"
  fi
}

s92_stop() {
  local k hp hs ans kr now
  s92_ctl "${1:-}" || return 1
  echo "CTL：$CTL"
  [ -e "$CTL/rc" ] && { echo "已有 rc marker（$(cut -f1 "$CTL/rc")）：harness 已結束，⛔ 不送訊號" >&2; return 1; }
  [ -e "$(v work)/freeze_record.json" ] && { echo "freeze record 已發布（commit point 已過）：⛔ 不中止，等它自己結束" >&2; return 1; }
  [ -e "$CTL/stop_attempt" ] && { echo "這個控制目錄已經嘗試過一次（$(cut -f1 "$CTL/stop_attempt")）：⛔ 不再送" >&2; return 1; }
  for k in harness_pid harness_start; do
    one "$k" && [[ "$(v "$k")" =~ ^[1-9][0-9]*$ ]] || { echo "run.tsv 的 $k 缺失、重複或不是正整數：⛔ 不送訊號" >&2; return 1; }
  done
  hp="$(v harness_pid)"; hs="$(v harness_start)"                                      # 只取一次，之後都用這兩個值
  read -r -p "確定要對 harness（PID $hp）送一次 TERM 嗎（這次嘗試會失敗，依「四」判讀）？輸入 yes：" ans
  [ "$ans" = yes ] || { echo "⛔ 沒有送訊號" >&2; return 1; }
  ( set -C; printf '%s\t%s\t%s\n' "$(date -u +%FT%TZ)" "$hp" "$hs" > "$CTL/stop_attempt" ) 2> /dev/null \
    || { echo "取不到這次中止的 claim（另一個 s92 stop 已經在做，或寫入失敗）：⛔ 不送訊號" >&2; return 1; }   # set -C：原子的建立、⛔ 不覆寫
  if ! { alive "$hp" "$hs" && grep -qa 'i074-stage2-sizing.sh' "/proc/$hp/cmdline"; }; then                # 與 ③ 相同的身分條件
    printf '%s\t身分不符或已結束\n' "$(date -u +%FT%TZ)" > "$CTL/stop_failed"; echo "身分不符或已結束：⛔ 不送訊號" >&2; return 1
  fi
  kill -TERM "$hp"; kr=$?; now="$(date -u +%FT%TZ)"
  if [ "$kr" != 0 ]; then
    printf '%s\tkill 的結束碼 %s\n' "$now" "$kr" > "$CTL/stop_failed"; echo "kill 失敗（結束碼 $kr）：交給 Claude，⛔ 不要再送" >&2; return 1
  fi
  printf '%s\n' "$now" > "$CTL/stop_sent" \
    || { echo "TERM 已送出（$now），但 stop_sent 寫入失敗：交給 Claude；stop_attempt 已擋下重送，⛔ 不要再送" >&2; return 1; }
  echo "已送出一次 TERM（$now）；之後用 s92 st 查看，等 harness 結束（最多約 5 分鐘）；⛔ 不要再送" >&2
}

case "${1:-}" in
  pre)  [ "$#" = 1 ] || { usage; exit 2; }; s92_pre ;;
  go)   [ "$#" = 1 ] || { usage; exit 2; }; s92_go ;;
  st)   [ "$#" -le 2 ] || { usage; exit 2; }; s92_st "${2:-}" ;;
  stop) [ "$#" -le 2 ] || { usage; exit 2; }; s92_stop "${2:-}" ;;
  *)    usage; exit 2 ;;
esac
```

## 三之二、核對指令（`s92 pre` 之前，使用者執行；一行）

```text id="i074_stage2_s92_check_001"
cd /home/dev/workspace/stock_trading && git show HEAD:docs/plans/i074-stage2-step92-s92.md | sed -n '/^```text id="i074_stage2_s92_script_001"$/,/^```$/p' | sed '1d;$d' | cmp - ~/.local/bin/s92 && sha256sum ~/.local/bin/s92 && echo "s92 ＝ HEAD 的計畫書 ok"
```

⛔ 沒有印出最後的 ok → ⛔ 不用 `s92`（照舊貼區塊，或交給 Claude 重新安裝）。

## 四、決定

| # | 決定 | 建議 |
|---|---|---|
| D1 | `s92 pre` 要不要加上 ⑨-1 的保存、btime、D1 的證據與基準 | **建議 (a) 加**：⑨-2 的 P4 要求「輸出原樣記進紀錄」、「五」6 要求 worktree 與 `/tmp` 的前後比對，現行的區塊都⛔ 沒有留下證據；加上的四段逐字取自 ⑨-1 已 review 的 `_002`（fail-closed），⑨-2 的區塊本身⛔ 不動；`$PRE` 在 tmpfs、名稱⛔ 不是 `i074-*`，⛔ 不影響 P6、P7、P9。(b) 只包 ⑨-2 的區塊（與 `s91` 的「逐字」相同），輸出照舊由使用者手動保存 |
| D2 | freeze record 發布之後的 `stop` | **建議拒絕**（⑨-2「三」：commit point 之後⛔ 不中止，harness 也忽略 TERM——送了只會留下誤導的 `stop_sent`）；`st` 同時顯示「已發布（commit point 已過：⛔ 不中止）」。⚠️ 檢查與送出之間 commit point 仍可能剛好通過：那時 TERM 會被忽略、⛔ 沒有傷害——`stop_sent` 只代表**已送出訊號**、⛔ 不代表**已中止**；是否中止一律看 rc marker 與 `<work>` 的內容（⑨-2「四」；第一輪 review 補，「七」的 X2 實測） |
| D3 | 位置與完整性 | 比照 `s91`：`~/.local/bin/s92`（repo 外）、「三之二」的獨立核對、`run.tsv` 的 `s92_sha256`；安裝的 SHA-256 記進本檔 |
| D4 | 移除的時機 | 比照 `s91` 的 D6：⑨-2 的結果 review 通過之後，由使用者決定是否 `rm ~/.local/bin/s92`（與 `/dev/shm/s92.lock`） |

## 五、受影響的檔案與歸檔

| 檔案 | 改動 |
|---|---|
| `~/.local/bin/s92`（repo 外） | 新檔、mode 755；內容 ＝「三之一」（逐位元相同） |
| `/dev/shm/s92.lock`、`/dev/shm/s92-pre-*.txt`（repo 外、tmpfs） | 第一次 `s92 go` 建立的空檔（啟動鎖）；`s92 pre` 的輸出（跑完由 Claude 複製進 `<NAME>.ctl/precheck.txt`、重驗之後刪除） |
| `docs/issue.md` ⑨-2「二」「三」 | 各加一行指標：「也可以用 `s92 pre`／`s92 go`、`s92 st`、`s92 stop`（先執行 `plans/i074-stage2-step92-s92.md` 的核對指令）」；區塊⛔ 不動 |
| `docs/issue.md` I-074 狀態列、roadmap | 一句指標 |
| 本檔 | 計畫、腳本、結果（含安裝的 SHA-256）；保留到 ⑨-2 的結果 review 通過，之後隨 ⑨-2 的計畫一起收斂（工具只服務這一次，⛔ 不搬進主題文件） |

## 六、風險與回滾

| 風險 | 處置 |
|---|---|
| 腳本與 ⑨-2 的區塊不同步 | 腳本由 HEAD 的區塊組成；安裝前後都確認 ⑨-2 的 precheck（`_003`）、wrapper、② 的函式與顯示都逐字包含在腳本裡 |
| 安裝之後、執行之前腳本被改動 | 「三之二」的核對（對 HEAD 的本檔，`cmp`）；`run.tsv` 的 `s92_sha256` 讓 Claude 事後再對一次 |
| `go` 雙啟動或半途失敗、`stop` 送兩次 | 與 `s91` 相同的鎖、兩次程序檢查、失敗即停、啟動之後的驗證、claim |
| 選錯控制目錄 | 來源驗證；`stop` 另驗 ③ 的身分條件 |
| 加上的 D1 證據要求 `claude` 的程序數 0 | 照 ⑨-2 的 D1（Claude 先結束）；Codex 只記錄，等待期間⛔ 不使用 |
| 本檔的 commit 讓 `code_worktree` 變大 | KB 級；⑨-2 照實判定 |
| 回滾 | 刪掉 `~/.local/bin/s92`（與 `/dev/shm/s92.lock`），照舊貼區塊；⛔ 不影響 repo |

## 七、測試與驗證（確認之後；scratchpad 的測試副本，`HOME` 指到 scratchpad，⛔ 不在真正的 repo 跑 `go`）

* 逐字：⑨-2 的 precheck（`_003`，代入 `R91`／`E91`）、wrapper（12 行；第一輪 review 訂正：原寫 9 行）、② 的函式與顯示、⑨-1 的四段都以完整的子字串出現在腳本裡；與 `s91` v5 的 `diff` 只有「二」最後一段列的四處；`bash -n`。
* `s92 pre`：pty（`HISTFILE` 指到 scratchpad）裡 `read` 真的在等、假 curl 收到的 header 與 token 完全相同、token 在輸出、`$PRE` 與 history 都是 0 次、「PRE 保存 ok」；非互動：結束碼 0、`$PRE` ＝ 終端機的輸出（最後一行除外）。
* `s92 go`（測試副本：`cd` 換成假 git repo、假的 `scripts/i074-stage2-sizing.sh` 依環境變數立刻失敗或等 20～25 秒，並把自己的環境記下來）：比照 `s91` 的 14 案（已有程序、回答 `no`、回答之後才出現程序、鎖被持有、兩個同時、`git rev-parse`／`cd` 失敗、`mkdir` 失敗、work 已存在、wrapper ⛔ 沒有起來、harness 立刻失敗、正常、鎖⛔ 沒有傳下去、第一次的 harness 還在時再 `go`）；另驗**假 harness 看到的環境**：呼叫端先 export `MEM=300m`、`MEM_MIN_MB=1`、`TOOLING_PATCH=x` 等 → harness 看到的是 `MEM` 未設定、`MEM_MIN_MB=447`、`MEM_RESERVE_MB=150`、`MEM_FORCE=0`、`MEM_STRICT=0`、`REPLAY_IMAGE_ID` 固定、其他移除。
* `s92 st`（7 案）與來源驗證（10 案，`st`、`stop` 各跑一次）：比照 `s91`；另加「freeze record 已發布」的顯示。
* `s92 stop`（14 案）：比照 `s91`；另加 **`<work>/freeze_record.json` 存在 → 拒絕、⛔ 沒有 claim**；**X2（第一輪 review 補）：等待回答時 freeze record 才發布** → 照樣送 TERM、寫 `stop_sent`（已送訊號）；假 harness 在發布之後忽略 TERM、照常以 0 結束、freeze record 保留（⛔ 沒有中止）；`st` 同時顯示「已送出」、`rc：0` 與「freeze record：已發布」。
* 參數（6 案）：說明、結束碼 2。
* 安裝之後：核對指令（以工作樹的本檔代替 `git show HEAD:…`）印 ok；mode 755；`s92`（說明）；`s92 st` 在沒有控制目錄時回 1；doc-refs、`git diff --check`、禁止符號後面的否定詞。測試留下的控制目錄、鎖檔與假程序都清掉（`/dev/shm` 只剩既有的 `libpod_lock`、`s91.lock`）。

## 八、執行步驟

1. 本計畫確認（含 D1～D4）。
2. 安裝（`install -m 755`）、「七」的測試、`docs/issue.md` 的指標 → ⛔ 不 commit，停下等 review。
3. review 通過 → 使用者 commit（只動 `docs/`）→ 核對指令對 HEAD 印 ok（Claude 確認）。
4. 在選定的時段（⑨-2 的 D2）：結束 Claude → 核對指令 → `s92 pre` → 確認開跑的條件 → `s92 go`（輸入 `yes`）→ 等待期間偶爾 `s92 st` → 必要時 `s92 stop`（只一次）→ 完成之後重新啟動 Claude。

## 九、實作結果（2026-10-27，待 review）

| 項目 | 結果 |
|---|---|
| `~/.local/bin/s92` | 從本檔（工作樹）抽出「三之一」→ `install -m 755`：17,955 bytes、mode 755、owner `dev`；**SHA-256 `f9004a96efd518399bfa790df5d4d69a35d13d2f50460c12018449b754d98b94`**；`command -v s92` ＝ `~/.local/bin/s92` |
| 核對指令（「三之二」） | 以工作樹的本檔代替 `git show HEAD:…` 原樣執行：印出上面的 SHA-256 與「s92 ＝ HEAD 的計畫書 ok」。本檔 commit 之後 HEAD 會變、「三之一」⛔ 沒有變；commit 之後 Claude 再對 HEAD 跑一次 |
| 腳本的來源 | 以腳本從 HEAD（`eb855b0`）的 `docs/issue.md` 組成：`s91` v5 的骨架 ＋ ⑨-2 的區塊；逐字包含 ⑨-2 的 precheck（`_003`，代入 `R91`／`E91`）、wrapper（12 行）、② 的函式與顯示（7 行）、⑨-1 的保存／btime／D1／基準；與 `s91` v5 的 `diff` 只有「二」最後一段列的四處；`bash -n` 通過 |
| `docs/issue.md` | ⑨-2「二」「三」的區塊之前各加一行指標（區塊⛔ 沒有動）；I-074 狀態列與 roadmap 的 ⑨-2 改成「P1 的修正已 commit `eb855b0`；簡易指令 `s92`（⚠️ 待 review）」 |

測試（scratchpad；⛔ 沒有在真正的 repo 跑 `go`）——**A：31 案、B：54 案（第一輪 review 補 X2 的 3 案之後重跑整份 B），全部符合**：

* A（安裝的 `s92`；`curl` 換成假的，其他是真的唯讀指令）：參數 6 案（說明、結束碼 2）；沒有控制目錄時 `st`、`stop` → 「控制目錄有 0 個」、1；`st formal-92-bogus` → 格式錯、1。`pre` 非互動：rc 0、「PRE 保存 ok」、`$PRE` ＝ 終端機的輸出（最後一行除外）、mode 600、假 curl 收到的 header ＝ token（前後空白已去掉）、token 在輸出與 `$PRE` 都是 0 次；P1 三項、P2、P3 兩項、P4 的 running 數量、P7、基準都有 ok；Claude 在執行 → ⛔ 沒有 D1 的 ok（預期）、D1 的 rc 行與 btime、worktree 登記數都有。`pre` pty：`read` 真的在等（提示行之後⛔ 沒有其他輸出才送 token）、「PRE 保存 ok」、header ＝ token、token 在 pty 輸出與 `$PRE` 都是 0 次。測試留下的 2 個 `$PRE` 已移出 `/dev/shm`。
* B（測試副本：`cd` 換成假 git repo、`/dev/shm` 換成 scratchpad 的目錄，共 13 行差異；假 harness 記下自己的環境、收到 TERM 時收掉子程序並以 143 結束）：
  * `go` 14 案＋環境：已有程序、回答 `no`、回答之後才出現程序、鎖被持有、兩個同時、`git rev-parse`／`cd` 失敗、**`mkdir` 失敗（既有的 `run.tsv`、`wrapper.sh` ⛔ 沒有被改、⛔ 沒有 wrapper）**、work 已存在、wrapper ⛔ 沒有起來（15 秒後回 1）、harness 立刻失敗、正常（回 0、10 個欄位各一行、`s92_sha256` ＝ 腳本、wrapper 與 harness 都在、`harness_pid` 是 harness 本身）、鎖⛔ 沒有傳給 wrapper／harness、第一次的 harness 還在時再 `go`。**環境**：呼叫端先 export `MEM=300m`、`MEM_MIN_MB=1`、`MEM_RESERVE_MB=9`、`MEM_FORCE=1`、`MEM_STRICT=1`、`CPUS`、`TOOLING_PATCH`、`REPLAY_DRY_RUN`、`PY_IMAGE`、`I074_SIZING_FAULT`、`SIZING_*`、`REPLAY_IMAGE_ID=bogus` → harness 看到的是 `MEM` 未設定、`MEM_MIN_MB=447`、`MEM_RESERVE_MB=150`、`MEM_FORCE=0`、`MEM_STRICT=0`、`REPLAY_IMAGE_ID` ＝ 封存的 image、其他都已移除。
  * `st` 7 案＋1：省略 NAME、指定 NAME、格式錯、不存在、兩個時列出並回 1、**freeze record 已發布的顯示**、harness 結束之後顯示 `rc：143` 與中止的紀錄。
  * `stop` 15 案＋X2：**freeze record 已發布 → 拒絕、⛔ 沒有 claim、harness 還在**；**X2：`stop` 停在 `yes` 的提示時才發布 → `stop` 回 0、`stop_attempt` 與 `stop_sent` 都有（已送訊號）；假 harness 忽略 TERM、照常以 rc 0 結束、freeze record 保留（⛔ 沒有中止）；`st` 同時顯示「中止：…已送出」、`rc：0`、「freeze record：已發布（commit point 已過：⛔ 不中止）」**；回答 `no`；正確 → 一次 TERM、`stop_attempt` 與 `stop_sent`、回 0；之後再 `stop` → 拒絕；缺 `harness_pid`、`harness_pid` 重複／⛔ 不是數字、缺 `harness_start`、已有 rc marker → 拒絕、⛔ 沒有 claim；`harness_start` 不符 → 身分不符、`stop_attempt` 與 `stop_failed`、回 1，之後再 `stop` 拒絕、`st` 顯示「未送出（身分不符或已結束）」；cmdline ⛔ 不是 harness → ⛔ 不送；`kill` 失敗（關掉內建的 `kill`、換成回 1 的假指令）→ 回 1、`stop_failed` 記下結束碼；**兩個同時 → 只有一個送了 TERM**、另一個取不到 claim；`stop_sent` 寫入失敗 → 回 1、TERM 已送出，之後拒絕重送。
  * 來源驗證 10 案（`st`、`stop` 各一次，都拒絕、⛔ 沒有 claim）：名稱不合格式、控制目錄是 symlink、mode 755、`run.tsv` 是 symlink、沒有 `run.tsv`、`name` 不符、`name` 重複、`expected`／`work`／`launched_utc` 不對。
* 收尾：本測試的程序都已結束（PID ＋ starttime 釘住才收）、⛔ 沒有殘留的假程序；`/dev/shm` 只剩既有的 `libpod_lock`、`s91.lock`；真正 repo 的 worktree 登記數 401 ⛔ 沒有變、⛔ 沒有建任何 commit、`~/i074_stage2_sizing` ⛔ 沒有 `formal-92-*`。

與計畫的差異：⛔ 沒有（「七」列的案例都做了；`st` 的 freeze record 顯示與 `stop` 的拒絕依 D2）。

## 第一輪 review 的修正（2026-10-27）

裁決：計畫整體合理，⛔ 沒有阻擋問題；補 2 項（P3）。另外確認了 `_003`、固定的環境、狀態的顯示、安裝版的 SHA、並行與重複的 `stop`、紀錄寫入失敗的阻擋都符合預期，語法與文件檢查通過。

| # | review 的意見 | 查證 | 處置 |
|---|---|---|---|
| 1 | （P3）補「等待回答期間 freeze record 才發布」的測試：此時仍會送 TERM 並寫 `stop_sent`（D2 已揭露的窗口），要以假 harness 驗證 TERM 被忽略、freeze record 保留，並明確區分「已送訊號」與「已中止」 | ✅ 成立：原本只測「執行 `stop` 之前已發布」；D2 已寫明這個窗口、⛔ 不改腳本 | 測試 B 加 X2（3 案）：假 harness 的 `commitwait` 模式在旗標出現時發布 `<work>/freeze_record.json`、之後忽略 TERM；`stop` 停在提示時才發布 → `stop` 回 0、`stop_sent` 存在，harness 在 TERM 之後仍在、以 rc 0 結束、freeze record 保留，`st` 同時顯示三者。整份 B 重跑：54 案全部符合。D2 補「`stop_sent` ＝ 已送出訊號、⛔ 不代表已中止；是否中止看 rc marker 與 `<work>`」。腳本⛔ 沒有變（SHA 仍是 `f9004a96…98b94`） |
| 2 | （P3）wrapper 的行數應是 12 行 | ✅ 成立：⑨-2 的 wrapper 是 12 行（`env -u …` 折成 5 行），9 行是 ⑨-1 的 | 「七」與「九」改成 12 行 |

## 十、實際使用（2026-10-27，⑨-2；⚠️ 待 review）

| 項目 | 結果 |
|---|---|
| 使用的子指令 | 使用者回報：核對指令（「三之二」）印出 ok → `s92 pre`（「PRE 保存 ok」）→ `s92 go`（輸入 `yes`，印出「已啟動」）→ 等待期間只用 `s92 st`；⛔ 沒有 `s92 stop`（控制目錄⛔ 沒有 `stop_attempt`／`stop_sent`／`stop_failed`） |
| 事後對照（D3） | `run.tsv` 的 `s92_sha256` `f9004a96efd518399bfa790df5d4d69a35d13d2f50460c12018449b754d98b94` ＝ 從 HEAD（`832f678`）的本檔抽出的「三之一」＝ `~/.local/bin/s92`（`cmp` 相同） |
| 啟動之後的驗證 | `run.tsv` 的 10 個欄位各恰好一行；`wrapper_pid` 1514982、`harness_pid` 1514990（＝ 報告的 run id 結尾） |
| 保存 | `s92 pre` 的 `$PRE`（`/dev/shm/s92-pre-20261027T090423Z-S3pHMR.txt`）與控制目錄的 4 個檔已由 Claude 複製進 `~/i074_stage2_sizing/formal-92-20261027T090656Z.ctl/`（`precheck.txt` mode 600）、`cmp` 相同、重驗之後才從 `/dev/shm` 刪除 |
| 結果 | ⑨-2 通過——見 `docs/issue.md` I-074「Stage 2 步驟 ⑨-2 正式確認重跑結果」 |
| D4（移除的時機） | 待使用者決定：⑨-2 的結果 review 通過之後，是否 `rm ~/.local/bin/s92 /dev/shm/s92.lock`（`/dev/shm/s92.lock` 仍在） |
