# shellcheck shell=bash
# I074-STAGE2-MEASURE-LIB
# I-074 Stage 2 ⑦d：sizing harness（scripts/i074-stage2-sizing.sh）與 memory／disk acceptance harness
# （scripts/i074-stage2-acceptance.sh）共用的 shell 原語（git show 20eacb2:docs/issue.md 的 I-074「Stage 2 步驟 ⑦d 細部計畫 v1」「二之一」）。
#
# ⚠️ 只負責 **re-exec 之後**的正式流程：由**快照裡**的主腳本、在快照的完整驗證通過之後 source（⛔ 不從活路徑 source）。
#   建 S、快照與 re-exec 是兩個入口各自的 minimal bootstrap（「二之三」的「bootstrap 的失敗與中斷」列）。
# ⚠️ 本檔只定義函式與狀態變數的初值，⛔ 不執行任何動作。呼叫端在 source 之前要設好 S、HELPER、REAL_DOCKER、RUN_ID、LOG；
#   之後再設 WORK（失敗清理要複製 S 到 <work>/raw-failed/）。
# ⚠️ 內容自 ④ 的 sizing harness 原樣抽出（共用原語，⛔ 不另寫一份）；演練用的故障注入仍是 `I074_SIZING_FAULT`。
# ⚠️ ⑦d 增補（git show 20eacb2:docs/issue.md 的 I-074「Stage 2 步驟 ⑦d 增補計畫：容器記憶體改以 RSS 判定」「二」④）：harness 自己是 subreaper（bootstrap 以
#   python3 啟動器設定）——`measure_subreaper_guard` 以行為驗證；每一步之後、正常結束之前以 `reap-adopted --check-only` 確認
#   沒有被收養的程序；`on_failure` 以 `reap-adopted` 清掉它們（結束碼 ≠ 0 → 保留 S）。

# ── 啟動守門（⚠️ 自 ④ 的 sizing harness 抽出；呼叫端要先定義 die、設 REPO_ROOT） ─────────────────────────
# REPLAY_IMAGE_ID、與 ⑩ 的 label shim 互斥（環境帶 I074_STAGE2_*、或 PATH 上的 docker 是 label shim → 中止）。設 IMAGE、REAL_DOCKER。
measure_start_guards() {  # $1＝harness 的名稱（訊息用）
  local _v
  IMAGE="${REPLAY_IMAGE_ID:-}"
  [ -n "$IMAGE" ] || die "⛔ 一律要求 REPLAY_IMAGE_ID（Stage 2 identity 的 image）。"
  [ -z "${PY_IMAGE:-}" ] || die "REPLAY_IMAGE_ID 與 PY_IMAGE ⛔ 不可同時設定。"
  for _v in $(compgen -e); do
    case "$_v" in I074_STAGE2_*) die "環境帶 $_v——$1 與 ⑩ 的 label shim 互斥（⛔ 在 ⑩ 的流程裡執行）" ;; esac
  done
  REAL_DOCKER="$(command -v docker)" || die "找不到 docker。"
  case "$REAL_DOCKER" in /dev/shm/*) die "PATH 裡的 docker 已經是 shim：$REAL_DOCKER" ;; esac
  # ⚠️ here-string（⛔ `head | grep -q`：pipefail 下會因 SIGPIPE 誤判）。
  if [ "$(head -c 2 -- "$REAL_DOCKER" 2>/dev/null)" = '#!' ] \
     && grep -q 'I074-STAGE2-LABEL-SHIM' <<< "$(head -n 5 -- "$REAL_DOCKER")"; then
    die "PATH 上的 docker 是 ⑩ 的 label shim：$REAL_DOCKER（兩個 shim 互斥）"
  fi
  "$REAL_DOCKER" image inspect "$IMAGE" >/dev/null 2>&1 || die "image $IMAGE 不在本機。"
}
# work 目錄防護：canonical path（含 parent symlink 指回 repo）、⛔ 不得已存在。設 REPO_CANON、WORK、L0_DEV。
measure_work_dir_guard() {  # $1＝--work-dir 的值
  REPO_CANON="$(realpath -- "$REPO_ROOT")"
  [ ! -e "$1" ] && [ ! -L "$1" ] || die "--work-dir 已存在：$1（⛔ 不覆蓋）"
  WORK="$(realpath -m -- "$1")"
  case "$WORK/" in "$REPO_CANON"/*) die "--work-dir 解析後在 repo 內：$WORK（⛔ 會汙染 repo 的 inventory）" ;; esac
  case "$REPO_CANON/" in "$WORK"/*) die "--work-dir 是 repo 的上層目錄：$WORK" ;; esac
  [ -d "$(dirname "$WORK")" ] || die "--work-dir 的上層目錄不存在：$(dirname "$WORK")"
  L0_DEV="$(stat -c %d "$(dirname "$WORK")")"
}
# --formal：stdout／stderr ⛔ 不得導到量測中的檔案系統上的一般檔案（harness 的輸出會混進峰值）。
measure_formal_fd_guard() {
  local fd
  for fd in 1 2; do
    if [ -f "/proc/$$/fd/$fd" ] && [ "$(stat -L -c %d "/proc/$$/fd/$fd")" = "$L0_DEV" ]; then
      die "--formal：fd $fd 被導到量測中的檔案系統上的一般檔案——harness 的輸出會混進 P_B（請導到 tmpfs 或終端機）"
    fi
  done
}

say() { printf '%s %s\n' "$(date -u +%H:%M:%S)" "$*" | tee -a "$LOG" >&2; }
meta() { printf '%s\t%s\n' "$1" "$2" >> "$S/meta.tsv"; }
comp() { printf '%s\t%s\n' "$1" "$2" >> "$S/components.tsv"; }
helper() { python3 "$HELPER" "$@"; }
sha_of() { sha256sum < "$1" | cut -d' ' -f1; }
alloc() { helper allocated "$1" | python3 -c 'import json,sys; print(json.load(sys.stdin)["allocated"])'; }

PHASE=""; SAMPLER_PID=""; STEP_PID=""; DONE=0
HARNESS_STARTTIME=""; GUARD_PROBE=""; GUARD_SHELL_PID=""; GUARD_LEFTOVER=0
# ⚠️ git show 20eacb2:docs/issue.md 的 I-074「Stage 2 raw-failed freeze record 修正計畫 v1」「三」：作廢清單由入口登記（sizing 登記 freeze record 與報告的
#   所有位置；acceptance ⛔ 不登記——on_failure ⛔ 不進臨界區、⛔ 不呼叫作廢，行為與修正前相同）。MEASURE_VOIDED＝最近一次作廢的結果。
MEASURE_VOID_PATHS=(); MEASURE_VOIDED=1
# 本程序（$$）在 /proc/<pid>/stat 的 starttime（第 22 欄）。$1＝pid（預設 $$）。
proc_starttime() {
  local s
  s="$(< "/proc/${1:-$$}/stat")" || return 1
  s="${s##*) }"
  set -- $s
  printf '%s\n' "${20}"
}
# harness 收養的程序：`--check-only`（$1＝說明）。⚠️ helper 必須是本 shell 的直接子程序（⛔ 不在 $( ) 裡呼叫）——
#   它以 getppid() 確認 harness 的身分。有收養的程序 ＝ 步驟把程序留在 host → on_failure（它會清掉它們）。
measure_check_adopted() {
  local rc=0
  helper reap-adopted --state "$S" --parent "$$" --parent-starttime "$HARNESS_STARTTIME" \
    ${SAMPLER_PID:+--exclude "$SAMPLER_PID"} --check-only 2>>"$S/adopted.err" || rc=$?
  [ "$rc" = 0 ] || on_failure "$1：harness 收養了程序（步驟把程序留在 host）或檢查失敗（reap-adopted 結束碼 $rc）" 1
}
# guard 的測試程序以 kill-pinned 收掉（身分可信時）；結果附加到 <S>/kill-pinned.jsonl。回傳 kill-pinned 的結束碼。
measure_kill_guard_probe() {
  local rc=0 out
  [ -n "$GUARD_PROBE" ] || return 0
  set -- $GUARD_PROBE
  out="$(helper kill-pinned --pid "$1" --starttime "$2")" || rc=$?
  [ -z "$out" ] || printf '%s\n' "$out" >> "$S/kill-pinned.jsonl"
  GUARD_PROBE=""
  [ "$rc" = 0 ] || GUARD_LEFTOVER=1
  return "$rc"
}
# 測試程序：自己 os.setsid()（失敗就立刻結束、⛔ 不寫身分）→ 讀自己的 starttime → 寫 <S>/guard-probe.json → 睡 5 秒。
#   失敗出口另寫只供診斷的 <S>/guard-probe-status.json（⛔ 不是身分來源）。$1＝S、$2＝故障注入。
GUARD_PROBE_CODE='import errno, json, os, sys, time
S, fault = sys.argv[1], sys.argv[2]
def write(name, obj):
    tmp = os.path.join(S, "." + name + ".tmp")
    with open(tmp, "w") as fh:
        fh.write(json.dumps(obj, sort_keys=True, separators=(",", ":")))
    os.replace(tmp, os.path.join(S, name))
def fail(stage, code):
    try:
        write("guard-probe-status.json", {"schema": "i074_stage2_guard_probe_status_v1", "pid": os.getpid(),
                                          "stage": stage, "errno": errno.errorcode.get(code, "EINVAL")})
    except OSError:
        pass
    sys.exit(3)
try:
    if fault == "guard-pgleader":
        os.setpgid(0, 0)
    os.setsid()
except OSError as exc:
    fail("setsid", exc.errno)
try:
    with open("/proc/self/stat") as fh:
        starttime = int(fh.read().rsplit(")", 1)[1].split()[19])
except (OSError, ValueError, IndexError) as exc:
    fail("self_stat", getattr(exc, "errno", None) or errno.EINVAL)
if fault != "guard-noident":
    write("guard-probe.json", {"pid": os.getpid() + (1 if fault == "guard-badident" else 0), "starttime": starttime})
time.sleep(5)
'
# ⑦d 增補：以行為驗證 harness 是 subreaper。⚠️ 在六步清理的 trap 裝好、建好 work 目錄之後呼叫；所有出口都收掉測試程序。
measure_subreaper_guard() {
  local ident ppid s
  rm -f "$S/guard-probe.json" "$S/guard-probe-status.json"
  # ① 子 shell **直接**背景啟動 Python（⛔ 不經外部的 setsid 指令），以 command substitution 回報 $!（恰好一行整數）。
  GUARD_SHELL_PID="$( ( python3 -c "$GUARD_PROBE_CODE" "$S" "${I074_SIZING_FAULT:-}" </dev/null >/dev/null 2>&1 & echo "$!" ) )"
  [[ "$GUARD_SHELL_PID" =~ ^[0-9]+$ ]] || GUARD_SHELL_PID=""
  for _ in $(seq 1 40); do
    [ -s "$S/guard-probe.json" ] && break
    sleep 0.05
  done
  # ③ 身分可信 ⟺ JSON 格式正確、pid ＝ $!、starttime 與 /proc 相符（helper 驗；不符 → 結束碼 1）。
  if [ -n "$GUARD_SHELL_PID" ] && ident="$(helper guard-identity --state "$S" --shell-pid "$GUARD_SHELL_PID" 2>>"$S/guard.err")"; then
    GUARD_PROBE="$ident"
    [ "${I074_SIZING_FAULT:-}" != subreaper-stall ] || { sleep 60 & wait "$!"; }
    s="$(< "/proc/${GUARD_PROBE%% *}/stat")" && s="${s##*) }" && set -- $s && ppid="$2" || ppid=""
    if [ "$ppid" != "$$" ]; then
      on_failure "harness ⛔ 不是 subreaper（測試程序 ${GUARD_PROBE% *} 的 ppid＝${ppid:-讀不到}、harness＝$$）" 1
    fi
    measure_kill_guard_probe || on_failure "收不掉 guard 的測試程序（見 kill-pinned.jsonl）" 1
    say "==> harness 是 subreaper（guard 通過）"
    return 0
  fi
  # ④ 身分不可信：⛔ 不送任何訊號，只以 $! 等最多 6 秒、確認它已消失或是 zombie；之後中止（subreaper 沒有被驗證）。
  local status=""
  [ -s "$S/guard-probe-status.json" ] && status="$(< "$S/guard-probe-status.json")"
  if [ -n "$GUARD_SHELL_PID" ]; then
    for _ in $(seq 1 120); do
      s="$(cat "/proc/$GUARD_SHELL_PID/stat" 2>/dev/null)" || break
      s="${s##*) }"; set -- $s
      case "$1" in Z|X) break ;; esac
      sleep 0.05
    done
    if s="$(cat "/proc/$GUARD_SHELL_PID/stat" 2>/dev/null)"; then
      s="${s##*) }"; set -- $s
      case "$1" in Z|X) ;; *) GUARD_LEFTOVER=1 ;; esac
    fi
  else
    GUARD_LEFTOVER=1
  fi
  on_failure "guard 無法確認測試程序的身分（\$!＝${GUARD_SHELL_PID:-沒有}；狀態檔：${status:-沒有}）——⛔ 不送訊號，subreaper 沒有被驗證" 1
}
stop_sampler() {
  if [ -n "$SAMPLER_PID" ]; then
    touch "$S/phases/$PHASE/sampler.stop" 2>/dev/null || true
    wait "$SAMPLER_PID" 2>/dev/null || true
    SAMPLER_PID=""
  fi
}
# 以 cidfile 記錄的 CID（含 twin）docker rm -f；helper 失敗時退回 bash 迴圈，**逐一累積**移除不了的 CID（⛔ 不吞掉）。
# 另以名稱**偵測**本次 run id 的殘留容器（⛔ 只用來偵測、⛔ 不用來移除）。回傳 0＝全部清掉；1＝有殘留（寫在 leftover-cids）。
cleanup_containers() {
  local f cid named left=0
  : > "$S/leftover-cids"
  if ! helper cleanup-cids --state "$S" --docker "$REAL_DOCKER" 2>>"$S/cleanup.err"; then
    for f in "$S"/cid/*.cid; do
      [ -s "$f" ] || continue
      cid="$(cat "$f")"
      if "$REAL_DOCKER" rm -f "$cid" >/dev/null 2>>"$S/cleanup.err" \
         || ! "$REAL_DOCKER" inspect "$cid" >/dev/null 2>&1; then
        rm -f "$f"
      else
        printf '%s\n' "$cid" >> "$S/leftover-cids"; left=1
      fi
    done
  fi
  named="$("$REAL_DOCKER" ps -a --filter "name=i074sz-$RUN_ID-" -q 2>>"$S/cleanup.err")" || { named=""; left=1; }
  if [ -n "$named" ]; then printf '%s\n' $named >> "$S/leftover-cids"; left=1; fi
  return "$left"
}
# process group 的成員（pid(stat)）。⚠️ zombie ⛔ 不算——它已經不能再寫任何東西；leader 在被 wait 回收之前就是 zombie。
group_members() {
  ps -e -o pid=,pgid=,stat= | awk -v g="$1" '$2 == g && $3 !~ /^Z/ { printf "%s(%s) ", $1, $3 }'
}
# process group 還有沒有活著的成員。⚠️ `ps` 本身失敗 ⇒ 當作仍存活（⛔ 查不到不能當作沒有；pipefail 讓 ps 的失敗傳得出來）。
# ⚠️ `I074_SIZING_FAULT=group-alive` 只給演練：模擬「連 KILL 都收不掉」的情況。
group_alive() {
  local members
  [ "${I074_SIZING_FAULT:-}" != group-alive ] || return 0
  members="$(group_members "$1")" || return 0
  [ -n "$members" ]
}
# 結束進行中的步驟：TERM → 最多等 20 秒 → 仍有成員就升級 KILL → 最多再等 5 秒 → **再確認一次**。
# 回傳 0＝整個 group 確實結束（並回收 leader）；1＝仍有成員（呼叫端必須保留 S——它們可能還在寫 S）。
# ⛔ **不能先 `wait` leader**：leader 忽略 TERM 時 wait 會一直卡住、逾時與 KILL 永遠輪不到（④ 第三輪 review 的測試抓到）。
#   ⚠️ 也⛔ 不能只 wait leader：shim 是孫程序、還在補寫 sidecar——所以輪詢整個 group。
stop_step_group() {
  local pgid="$1" term_polls=100 kill_polls=25
  # 演練（stuck／group-alive）只縮短等待時間（2 秒 ＋ 1 秒），邏輯相同。
  case "${I074_SIZING_FAULT:-}" in stuck|group-alive) term_polls=10; kill_polls=5 ;; esac
  say "    收尾：process group $pgid（成員：$(group_members "$pgid")）"
  kill -TERM -- "-$pgid" 2>/dev/null || true
  for _ in $(seq 1 "$term_polls"); do
    group_alive "$pgid" || { wait "$pgid" 2>/dev/null || true; return 0; }
    sleep 0.2
  done
  say "⚠️ process group $pgid 在 TERM 之後仍有成員（$(group_members "$pgid")）——升級 KILL"
  kill -KILL -- "-$pgid" 2>/dev/null || true
  for _ in $(seq 1 "$kill_polls"); do
    group_alive "$pgid" || { wait "$pgid" 2>/dev/null || true; return 0; }
    sleep 0.2
  done
  say "⚠️ process group $pgid 在 KILL 之後仍有成員（$(group_members "$pgid")）"
  return 1
}
# 在自己的 process group（setsid）執行一個步驟、等 leader 結束並比對結束碼。$1＝步驟名、$2＝預期結束碼、其餘＝指令。
# ⚠️ **leader 結束 ≠ 步驟結束**：同一個 group 裡可能還有背景子程序（leader 沒等它們就退出）仍在寫 S。
#   ⛔ 不論結束碼是否符合預期，都要確認 group 已無活著的成員；`STEP_PID` 保留到確認完——結束碼不符或 group 仍有
#   成員時交給 on_failure() 以同一個 PGID 收尾（④ 第四輪 review）。
# ⚠️ 背景 ＋ wait：bash 要等前景指令結束才處理 trap。
run_in_group() {
  local name="$1" want="$2" rc=0
  shift 2
  setsid "$@" >"$S/out/$name.stdout" 2>"$S/out/$name.stderr" &
  STEP_PID=$!
  wait "$STEP_PID" || rc=$?
  printf '%s\t%s\t%s\n' "$name" "$want" "$rc" >> "$S/rc.tsv"
  [ "$rc" = "$want" ] || { tail -5 "$S/out/$name.stderr" >&2; on_failure "$name" "$rc"; }
  if group_alive "$STEP_PID"; then
    on_failure "$name：leader 已結束（結束碼 $rc）但 process group $STEP_PID 仍有成員" 1
  fi
  STEP_PID=""
  measure_check_adopted "$name"                                      # ⑦d 增補：逃出 group 的程序也⛔ 不得留下
}
# 成功路徑用：本次 run id 的容器必須一個都不剩。⚠️ `docker ps` 本身失敗 ⇒ 查不到就⛔ 不能當作沒有。
ensure_no_run_containers() {  # $1＝說明
  local out
  if ! out="$("$REAL_DOCKER" ps -a --filter "name=i074sz-$RUN_ID-" -q 2>>"$S/cleanup.err")"; then
    on_failure "$1：docker ps 失敗（⛔ 查不到就不能當作沒有殘留）" 1
  fi
  [ -z "$out" ] || on_failure "$1：仍有本次的容器 $(printf '%s ' $out)" 1
}
# 作廢 MEASURE_VOID_PATHS（raw-failed freeze record 修正計畫「三」、D1；no-follow）：symlink 只移除 link（⛔ 不跟隨）；一般檔案先把
#   SHA-256 記進 console.log 再刪；其他型別（目錄、FIFO……）只記型別、⛔ 不讀內容，嘗試刪除。最後逐一確認都不存在 → 0；
#   否則 1，並把仍存在的路徑與型別印到 stderr。⚠️ 每個指令都容許失敗（⛔ 不會在作廢到一半時中止；呼叫端一律 guarded）。
measure_void_outputs() {
  local p sha kind left=0
  for p in "${MEASURE_VOID_PATHS[@]}"; do
    if [ -L "$p" ]; then
      say "作廢 $p：symlink（⛔ 不跟隨）" || true
      rm -f -- "$p" || true
    elif [ -f "$p" ]; then
      sha="$(sha256sum < "$p" | cut -d' ' -f1)" || sha="讀不到"
      say "作廢 $p：一般檔案 sha256=$sha" || true
      rm -f -- "$p" || true
    elif [ -e "$p" ]; then
      kind="$(stat -c %F -- "$p" 2>/dev/null)" || kind="型別不明"
      say "作廢 $p：$kind（⛔ 不讀內容）" || true
      rm -f -- "$p" || true
    fi
  done
  for p in "${MEASURE_VOID_PATHS[@]}"; do
    if [ -e "$p" ] || [ -L "$p" ]; then
      kind="$(stat -c %F -- "$p" 2>/dev/null)" || kind="型別不明"
      echo "⚠️ 作廢失敗，仍存在：$p（$kind）" >&2 || true
      left=1
    fi
  done
  return "$left"
}
# ⚠️ ⑨-1 細部計畫「十」（「〇之一」(i)）：S/bin/docker 是 shim 在 PATH 上的入口（指向 S/harness 裡本體的 symlink）。raw/、raw-failed/
#   是整個 S 的複本，帶著它就產不出 raw manifest（契約遇到 symlink 一律拒絕）——複製之前只移除這一個 symlink：先把 readlink 的結果
#   記進 console.log，⛔ 不跟隨、⛔ 不動其他檔案（本體在 harness/、有 MANIFEST 的 SHA）。不存在 → 0；存在但不是 symlink、或移除
#   之後仍存在 → 1（呼叫端⛔ 不複製）。之後的收尾只用 REAL_DOCKER（⛔ 不再經 PATH 找 docker）。
measure_drop_shim_link() {
  local link="$S/bin/docker" target kind
  if [ -L "$link" ]; then
    target="$(readlink -- "$link")" || target="（readlink 失敗）"
    say "shim 的入口：bin/docker → $target（移除這個 symlink，⛔ 不跟隨）" || true
    rm -f -- "$link" || true
  elif [ -e "$link" ]; then
    kind="$(stat -c %F -- "$link" 2>/dev/null)" || kind="型別不明"
    echo "⚠️ shim 的入口 $link 存在但不是 symlink（$kind）——⛔ 不移除、⛔ 不複製" >&2 || true
    return 1
  fi
  if [ -e "$link" ] || [ -L "$link" ]; then
    echo "⚠️ 移除不了 shim 的入口：$link" >&2 || true
    return 1
  fi
}
on_failure() {  # $1＝失敗的階段、$2＝結束碼。⚠️ 全部發生在量測窗口終止之後。
  # ⚠️ raw-failed freeze record 修正計畫「三」（D7）：作廢清單非空時，一開始就忽略 INT／TERM（在 trap - EXIT 之前——⛔ 不在兩個
  #   builtin 之間留下原本的 handler），作廢完才恢復預設（之後與修正前相同：清理期間再收到訊號就中止）。作廢放在最前面：
  #   之後的 say、touch 等在 set -e 下失敗會讓 on_failure 提早中止，⛔ 不得擋在作廢之前。清單為空（acceptance）時⛔ 不進臨界區。
  if [ "${#MEASURE_VOID_PATHS[@]}" -gt 0 ]; then
    trap '' INT TERM
    trap - EXIT
    MEASURE_VOIDED=1
    measure_void_outputs || MEASURE_VOIDED=0
    trap - INT TERM
  else
    trap - EXIT INT TERM
  fi
  local stage="$1" rc="$2" cleaned=0 copied=0 summarized=0 group_stopped=1 adopted_ok=1 rrc=0
  stop_sampler                                                       # 1
  [ -z "${FS_NOISE:-}" ] || rm -f -- "$FS_NOISE"                     # 演練的 fs-noise（量測範圍外的暫存檔）
  cleanup_containers || true                                         # 2：先停掉還在跑的容器
  # 2b：進行中的步驟在自己的 session 裡（setsid），整個 process group 一起結束，並**等它真的結束**
  #     （shim 的中斷處理會補寫 sidecar——⛔ 不能在它寫完之前就複製 S）。
  if [ -n "$STEP_PID" ]; then
    stop_step_group "$STEP_PID" || group_stopped=0
    STEP_PID=""
  fi
  # 2b'（⑦d 增補）：guard 的測試程序（kill-pinned）與 harness 收養的程序（reap-adopted；含 host-run 被 KILL 留下的）
  measure_kill_guard_probe || adopted_ok=0
  [ "$GUARD_LEFTOVER" = 0 ] || adopted_ok=0
  if [ -n "$HARNESS_STARTTIME" ]; then
    helper reap-adopted --state "$S" --parent "$$" --parent-starttime "$HARNESS_STARTTIME" 2>>"$S/adopted.err" || rrc=$?
    [ "$rrc" = 0 ] || adopted_ok=0
  fi
  cleanup_containers && cleaned=1                                    # 2c：group 結束後再清一次——這一次才算數
  [ -n "$PHASE" ] && [ -d "$S/phases/$PHASE" ] && touch "$S/phases/$PHASE/aborted"   # 3
  say "中止：階段 $stage、結束碼 $rc"
  # 4：⚠️ 作廢沒有全部成功 → ⛔ 不複製（raw-failed/ ⛔ 不得含移除不了的 freeze record）、保留 S（第 6 步）。
  #   ⚠️ ⑨-1 細部計畫「十」：三項清理（步驟的 group、收養的程序、容器）也要都成功——任一失敗時寫入者可能還在改 S，複本會是混合
  #   時間點的內容（只保留 S、只當診斷資料）。發布是原子的：raw-failed 與 staging 兩個名稱都⛔ 不存在（symlink 也算：mv -T 會直接
  #   取代空目錄、cp -a 會複製到既有目錄的裡面）→ 移除 shim 的入口 → 完整複製到 staging → 同目錄的 mv -T（rename(2)）。任何一段
  #   失敗或中途被終止 → ⛔ 沒有 raw-failed/（最多留下 staging，⛔ 不刪、⛔ 不是證據）；rename 之後 raw-failed/ 就是正式的原始量測
  #   （之後才被終止、摘要缺失也一樣——rename 是唯一的分界）。
  if [ "$MEASURE_VOIDED" = 1 ] && [ "$group_stopped" = 1 ] && [ "$adopted_ok" = 1 ] && [ "$cleaned" = 1 ] \
     && [ -d "${WORK:-/nonexistent}" ] && [ "${I074_SIZING_FAULT:-}" != copy ] \
     && [ ! -e "$WORK/raw-failed" ] && [ ! -L "$WORK/raw-failed" ] \
     && [ ! -e "$WORK/.raw-failed.staging" ] && [ ! -L "$WORK/.raw-failed.staging" ] \
     && measure_drop_shim_link \
     && cp -a "$S" "$WORK/.raw-failed.staging" 2>/dev/null \
     && mv -T -- "$WORK/.raw-failed.staging" "$WORK/raw-failed"; then  # 4
    copied=1
  elif [ -d "${WORK:-/nonexistent}" ]; then
    { echo "⚠️ raw-failed/ 沒有發布（作廢、三項清理、名稱檢查、shim 的入口、複製或 rename 有一項沒有成功）"
      if [ -e "$WORK/.raw-failed.staging" ] || [ -L "$WORK/.raw-failed.staging" ]; then
        echo "   留下的 staging（⛔ 不是證據）：$WORK/.raw-failed.staging"
      fi; } >&2 || true
  fi
  # 5：⑨-1 細部計畫「十」（D9）：摘要的欄位與 note 固定指向 raw-failed/——只在它確實發布（copied ＝ 1）之後才寫。寫入失敗（含 summary
  #   注入）時 raw-failed/ 仍是正式的原始量測（D10），⛔ 不刪、⛔ 不回滾，S 照舊保留；helper 以原子發布寫（正式檔只會是不存在或完整的
  #   canonical JSON）。
  if [ "$copied" = 1 ] && [ "${I074_SIZING_FAULT:-}" != summary ] \
     && helper failure-summary --state "$S" --stage "$stage" --rc "$rc" --out "$WORK/failure_summary.json" \
          --leftover-cids "$S/leftover-cids"; then                     # 5
    summarized=1
  fi
  if [ "$cleaned" = 0 ]; then
    echo "⚠️ 有容器移除不了（CID 如下），請手動 docker rm -f：" >&2
    sort -u "$S/leftover-cids" >&2
  fi
  if [ "$adopted_ok" = 0 ]; then
    echo "⚠️ 收不掉的 host 程序（reap-adopted 結束碼 $rrc；guard 的測試程序：$GUARD_LEFTOVER）：" >&2
    cat "$S/leftover-pids.json" "$S/kill-pinned.jsonl" "$S/adopted.err" 2>/dev/null >&2 || true
  fi
  # 6：作廢與五者都成功才清 S——⚠️ 步驟的 process group 沒有確實結束、或還有收不掉的程序時，它們可能還在寫 S。
  if [ "$MEASURE_VOIDED" = 1 ] && [ "$group_stopped" = 1 ] && [ "$adopted_ok" = 1 ] && [ "$cleaned" = 1 ] && [ "$copied" = 1 ] \
     && [ "$summarized" = 1 ]; then
    rm -rf "$S"
    echo "原始量測：$WORK/raw-failed/；摘要：$WORK/failure_summary.json（⛔ 不宣稱 P_B）" >&2
  else
    echo "⚠️ 作廢（$MEASURE_VOIDED）、步驟結束（$group_stopped）、收養的程序（$adopted_ok）、容器清理（$cleaned）、原始量測的複製（$copied）" \
         "或失敗摘要（$summarized）沒有全部成功——保留 S：$S" >&2
    [ "$copied" = 0 ] || echo "   raw-failed/ 已發布（正式的原始量測；摘要缺失）：$WORK/raw-failed/" >&2
  fi
  exit 1
}
# 正式流程的 trap（六步清理）。⚠️ 呼叫之前，入口的 bootstrap trap 仍有效。
measure_install_traps() {
  HARNESS_STARTTIME="$(proc_starttime)" || HARNESS_STARTTIME=""
  trap 'on_failure signal-INT 130' INT
  trap 'on_failure signal-TERM 143' TERM
  trap '[ "$DONE" = 1 ] || on_failure "未預期的結束" "$?"' EXIT
}
# 開一個量測窗口：$1＝phase、$2＝locations（JSON）、$3＝L0 的路徑、$4＝Docker Root Dir、$5＝inventory roots（JSON）。
# ⚠️ 允許位置的根目錄要由呼叫端**先**建好，再呼叫本函式（否則第一次建檔會改到不在 allowlist 的父目錄 mtime）。
measure_begin_phase() {
  PHASE="$1"
  helper baseline --state "$S" --phase "$PHASE" --locations "$2" --fs-path "$3" --docker-root "$4" --inventory-roots "$5"
  helper sample --state "$S" --phase "$PHASE" --locations "$2" --fs-path "$3" &
  SAMPLER_PID=$!
  say "==> 路徑 $PHASE：baseline 已記錄，取樣中"
  measure_fs_noise_begin "$PHASE"
}
# ⚠️ git show 20eacb2:docs/issue.md 的 I-074「Stage 2 量測的有效性條件與 ⑨-1 fail-fast 計畫」「六」：故障注入 fs-noise（⛔ 不用在正式量測、只在演練；--formal 一律拒絕
#   故障注入）——success 窗口期間在量測範圍外（work 目錄的父目錄，同一個檔案系統；⛔ 不在 inventory 的根目錄內）寫 2 MiB、
#   窗口結束之後刪除。有效性條件 ①（未解釋的淨成長 ≤ 1 MiB）必須把這次量測判成無效。
FS_NOISE=""
measure_fs_noise_begin() {
  [ "${I074_SIZING_FAULT:-}" = fs-noise ] && [ "$1" = success ] || return 0
  local parent
  parent="$(dirname "$WORK")"
  [ "$(stat -c %d "$parent")" = "$(stat -c %d "$WORK")" ] || on_failure "fs-noise：work 目錄的父目錄⛔ 不在同一個檔案系統" 1
  FS_NOISE="$parent/.i074-fs-noise-$RUN_ID"
  { head -c 2097152 /dev/urandom > "$FS_NOISE" && sync -- "$FS_NOISE"; } || on_failure "fs-noise：寫不了 $FS_NOISE" 1
  say "==> 故障注入 fs-noise：量測範圍外寫了 2 MiB（$FS_NOISE）"
}
measure_fs_noise_end() {
  [ -n "$FS_NOISE" ] || return 0
  rm -f -- "$FS_NOISE"
  FS_NOISE=""
}
# 關一個量測窗口：$1＝run 目錄、$2＝archive 路徑、$3＝inventory roots（JSON）、$4＝允許位置（JSON）、$5＝source roots（JSON）。
measure_end_phase() {
  stop_sampler
  helper phase-end --state "$S" --phase "$PHASE" --run-dir "$1" --archive "$2" \
    --inventory-roots "$3" --allowed "$4" --source-roots "$5" \
    || on_failure "自我檢查（$PHASE）" 1
  measure_fs_noise_end
}
