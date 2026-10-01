#!/bin/bash -p
# I-074 Stage 2 ⑦b：⑩ 的正式入口（issue.md I-074 v29「八之一」～「八之二」＋「Stage 2 步驟 ⑦b 細部計畫 v1」）。
#
#   REPLAY_IMAGE_ID=sha256:… scripts/run-i074-stage2.sh --freeze-record <path> --work-dir <repo 外、尚不存在的目錄>
#   REPLAY_IMAGE_ID=sha256:… scripts/run-i074-stage2.sh --resume  --work-dir <dir>   # 只在 finalize／publish 回 1 之後
#   REPLAY_IMAGE_ID=sha256:… scripts/run-i074-stage2.sh --promote --work-dir <dir>   # ⚠️ ⑦b：晉升是 stub，一律回 9
#
# ⚠️ **正式命令一律直接執行本檔**（shebang 是 `#!/bin/bash -p`：⛔ 處理 BASH_ENV／ENV、⛔ 匯入環境裡的函式）；
#    以 `bash <本檔>` 啟動會被拒絕（`$-` 不含 `p`）。
#
# 同一個檔案三種角色（依第一個參數分派）：
#   入口（公開 argv）→ `exec /usr/bin/python3 -I scripts/lib/i074-stage2-supervisor.py`（取鎖、sentinel、subreaper）
#   → 持鎖階段（`--internal-locked-stage`；真正 repo 的 HEAD 版）：建立執行目錄、複製 freeze record、clone 複本
#   → 複本內 orchestrator（`--internal-in-clone`；複本 `repo_head` 版）：preflight 0～7 → replay → 分流 → finalize／publish
#   → 依磁碟事實判終態 → 晉升（⑦b：stub，回 9）。
#
# 端到端結束碼（⑦b 的範圍；完整的表見計畫書「二之十」）：
#   1 ＝ 還沒有任何終態（含 preflight、replay 失敗；finalize／publish 回 1 時可以 --resume）；
#   2 ＝ preflight 的 failed-record lookup 命中；8 ＝ --promote 模式在取得 sentinel 之前失敗（排除原因後可重跑）；
#   9 ＝ --promote 模式遇到 sentinel，或已有終態而晉升尚未實作（stub）；128＋N ＝ 被訊號 N 中斷；
#   137 ＝ supervisor 被 SIGKILL（sentinel 留下，**需要重開機**）。
#
# ⚠️ 測試**⛔ 不碰 `/run/lock`**：scripts/test-i074-stage2.sh 把本檔複製到隔離的 repo、以 sed 改掉下方「常數」區塊，
#    並斷言與正式檔案只差那幾行——⛔ 沒有執行期的覆寫口。
set -euo pipefail

# ── 常數（⚠️ 測試以 sed 改的只有這一段：scripts/test-i074-stage2.sh） ──────────────────
I074_LOCK_PATH=/run/lock/i074-stage2.lock
I074_SENTINEL_PATH=/run/lock/i074-stage2.active
I074_LABEL_KEY=i074.stage2.run
I074_TRUSTED_PATH=/usr/bin:/bin
I074_GIT=/usr/bin/git
I074_PYTHON=/usr/bin/python3
I074_BASH=/bin/bash
# ── 常數結束 ────────────────────────────────────────────────────────────────────────

# 結束碼（鏡像 supervisor；⑦c 才把 8、9 放進 replay_bundle/publish.py——「三」#2）
readonly EXIT_ABORT=1 EXIT_LOOKUP_HIT=2 EXIT_PROMOTION_FAILED=8 EXIT_PROMOTION_BLOCKED=9

readonly SELF_REL=scripts/run-i074-stage2.sh
readonly SUPERVISOR_REL=scripts/lib/i074-stage2-supervisor.py
readonly SHIM_REL=scripts/lib/i074-stage2-docker-label-shim.sh
readonly STAGE2_REL=python/baselines/i074_stage2
readonly CF_PATCH_REL=$STAGE2_REL/counterfactual_e1cbbbd.patch
readonly TOOLING_PATCH_REL=$STAGE2_REL/tooling_e1cbbbd.patch
# 凍結副本（相對 <work>/run；鏡像 stage2_archive 的 FROZEN_*，由測試斷言相等）
readonly FROZEN_CF=patches/counterfactual.patch
readonly FROZEN_TOOLING=patches/tooling.patch
# 入口清單：入口與持鎖階段會執行的每一個檔案（「入口清單的投影 ＝ HEAD」就是入口的 cleanliness 的全部）。
readonly ENTRY_FILES=(scripts/run-i074-stage2.sh scripts/lib/i074-stage2-supervisor.py
                      python/scripts/i074_stage2_freeze_record.py python/scripts/i074_stage2_preflight.py
                      python/scripts/_i074_bootstrap.py python/backtest/modular/sr_scoring/replay_bundle/canonical.py)

case "${1:-}" in
  --internal-locked-stage) ROLE=locked; shift ;;
  --internal-in-clone) ROLE=clone; shift ;;
  *) ROLE=entry ;;
esac

say() { printf '%s %s\n' "$(date -u +%H:%M:%S)" "$*" >&2; }
err() { printf 'ERROR: %s\n' "$*" >&2; }

# ═════════════════════════════════════════════════════════════════════════════════
# 入口
# ═════════════════════════════════════════════════════════════════════════════════
entry_main() {
  local pre=$EXIT_ABORT a v mode="" freeze="" work_arg="" repo work freeze_abs="" f want got
  for a in "$@"; do [ "$a" != --promote ] || pre=$EXIT_PROMOTION_FAILED; done
  # ⚠️ 以下到 PATH 為止**只用 bash 內建**（第二輪 review）：bash -p 已經⛔ 處理 BASH_ENV／ENV；
  #    這裡再清掉會讓固定路徑的程式載入呼叫者程式碼的變數，⛔ 不傳給 supervisor 與之後的 workload。
  case "$-" in
    *p*) ;;
    *) err "必須直接執行 scripts/run-i074-stage2.sh（#!/bin/bash -p）——⛔ 不接受 bash <script>（BASH_ENV 可能已經執行）"
       exit "$pre" ;;
  esac
  unset BASH_ENV ENV
  for v in $(compgen -e); do
    case "$v" in LD_*|PYTHON*|GIT_*) unset "$v" ;; esac
  done
  PATH="$I074_TRUSTED_PATH"
  export PATH
  for v in $(compgen -e); do
    case "$v" in I074_STAGE2_*) err "環境已帶 $v——協定變數只能由 supervisor 產生"; exit "$pre" ;; esac
  done
  if ! [[ "${REPLAY_IMAGE_ID:-}" =~ ^sha256:[0-9a-f]{64}$ ]]; then
    err "REPLAY_IMAGE_ID 必須是 sha256: ＋ 64 位小寫 hex（⛔ tag、⛔ 選項）"
    exit "$pre"
  fi

  while [ "$#" -gt 0 ]; do
    case "$1" in
      --freeze-record|--work-dir)
        if [ "$#" -lt 2 ] || [ -z "$2" ] || [ "${2#-}" != "$2" ]; then err "$1 需要值"; exit "$pre"; fi
        if [ "$1" = --freeze-record ]; then
          [ -z "$freeze" ] || { err "--freeze-record 重複出現"; exit "$pre"; }
          freeze="$2"
        else
          [ -z "$work_arg" ] || { err "--work-dir 重複出現"; exit "$pre"; }
          work_arg="$2"
        fi
        shift 2 ;;
      --resume|--promote)
        [ -z "$mode" ] || { err "模式旗標互斥：已有 --$mode，又給了 $1"; exit "$pre"; }
        mode="${1#--}"; shift ;;
      *) err "未知參數 $1（⛔ 沒有路徑覆寫、⛔ 沒有 --repo-head）"; exit "$pre" ;;
    esac
  done
  if [ -z "$mode" ]; then
    mode=run
    [ -n "$freeze" ] || { err "用法：$SELF_REL --freeze-record <path> --work-dir <dir> | --resume --work-dir <dir> | --promote --work-dir <dir>"; exit "$pre"; }
  else
    [ -z "$freeze" ] || { err "--freeze-record 只屬於完整執行"; exit "$pre"; }
  fi
  [ -n "$work_arg" ] || { err "需要 --work-dir"; exit "$pre"; }

  case "${BASH_SOURCE[0]}" in */*) repo="${BASH_SOURCE[0]%/*}/.." ;; *) repo=.. ;; esac
  repo="$(cd "$repo" && pwd -P)" || { err "找不到 repo"; exit "$pre"; }
  for f in "${ENTRY_FILES[@]}"; do
    if ! git -C "$repo" ls-files --error-unmatch -- "$f" >/dev/null 2>&1; then
      err "$f 沒有被追蹤——⛔ 不執行未 commit 的程式"; exit "$pre"
    fi
    want="$(git -C "$repo" cat-file blob "HEAD:$f" | sha256sum)" || { err "HEAD 裡沒有 $f"; exit "$pre"; }
    got="$(sha256sum < "$repo/$f")"
    [ "$want" = "$got" ] || { err "$f 的內容 ≠ HEAD 中的版本——⛔ 不執行未 commit 的程式"; exit "$pre"; }
  done

  if [ "$mode" = run ]; then
    if [ -e "$work_arg" ] || [ -L "$work_arg" ]; then err "--work-dir 已存在：$work_arg（⛔ 不覆蓋）"; exit "$pre"; fi
    work="$(realpath -m -- "$work_arg")"
    [ -d "${work%/*}" ] || { err "--work-dir 的上層目錄不存在"; exit "$pre"; }
    freeze_abs="$(realpath -e -- "$freeze")" && [ -f "$freeze_abs" ] || { err "freeze record 不存在：$freeze"; exit "$pre"; }
  else
    [ -d "$work_arg" ] && [ ! -L "$work_arg" ] || { err "--work-dir 不存在：$work_arg"; exit "$pre"; }
    work="$(realpath -e -- "$work_arg")"
  fi
  case "$work/" in "$repo"/*) err "--work-dir 在 repo 內：$work"; exit "$pre" ;; esac
  case "$repo/" in "$work"/*) err "--work-dir 是 repo 的上層目錄：$work"; exit "$pre" ;; esac

  local args=("$mode" --work-dir "$work")
  [ -z "$freeze_abs" ] || args+=(--freeze-record "$freeze_abs")
  exec "$I074_PYTHON" -I "$repo/$SUPERVISOR_REL" "${args[@]}"
}

# ═════════════════════════════════════════════════════════════════════════════════
# 持鎖階段與複本共用的守門
# ═════════════════════════════════════════════════════════════════════════════════
MODE=""; WORK=""; FREEZE_ARG=""
parse_internal() {  # $1＝角色（locked／clone）；其餘：<mode> --work-dir <dir> [--freeze-record <path>]
  local role="$1"
  shift
  MODE="${1:-}"; shift || true
  case "$MODE" in run|resume|promote) ;; *) err "內部模式不符：'$MODE'"; return 1 ;; esac
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --work-dir) [ "$#" -ge 2 ] && [ -z "$WORK" ] || return 1; WORK="$2"; shift 2 ;;
      --freeze-record) [ "$#" -ge 2 ] && [ -z "$FREEZE_ARG" ] || return 1; FREEZE_ARG="$2"; shift 2 ;;
      *) err "內部參數不符：$1"; return 1 ;;
    esac
  done
  [ -n "$WORK" ] || return 1
  # --freeze-record 只屬於持鎖階段的 run（複本內一律讀 <work>/freeze/ 的副本）。
  if [ "$role" = locked ] && [ "$MODE" = run ]; then [ -n "$FREEZE_ARG" ] || return 1; else [ -z "$FREEZE_ARG" ] || return 1; fi
}

# 協定 v1 ＋ 信任根的 PATH（「二之三之一」）。⚠️ 名稱或語意要改就是協定 v2。
require_protocol() {
  local want=orchestrator
  [ "$MODE" != promote ] || want=promote
  [[ "${I074_STAGE2_TOKEN:-}" =~ ^[0-9a-f]{64}$ ]] || { err "I074_STAGE2_TOKEN 不符"; return 1; }
  [[ "${I074_STAGE2_SUPERVISOR_PID:-}" =~ ^[1-9][0-9]*$ ]] || { err "I074_STAGE2_SUPERVISOR_PID 不符"; return 1; }
  [[ "${I074_STAGE2_SUPERVISOR_START:-}" =~ ^[1-9][0-9]*$ ]] || { err "I074_STAGE2_SUPERVISOR_START 不符"; return 1; }
  case "${I074_STAGE2_REAL_REPO:-}" in /*) ;; *) err "I074_STAGE2_REAL_REPO 不符"; return 1 ;; esac
  case "${I074_STAGE2_REAL_DOCKER:-}" in /*) ;; *) err "I074_STAGE2_REAL_DOCKER 不符"; return 1 ;; esac
  [ "${I074_STAGE2_MODE:-}" = "$want" ] || { err "I074_STAGE2_MODE 與模式 $MODE 不一致"; return 1; }
  [[ "${REPLAY_IMAGE_ID:-}" =~ ^sha256:[0-9a-f]{64}$ ]] || { err "REPLAY_IMAGE_ID 格式不符"; return 1; }
  [ "$PATH" = "$WORK/bin:$I074_TRUSTED_PATH" ] || { err "PATH 不是 <work>/bin:$I074_TRUSTED_PATH"; return 1; }
  hash -r
  [ "$(command -v git)" = "$I074_GIT" ] || { err "git 不是 $I074_GIT"; return 1; }
  [ "$(command -v python3)" = "$I074_PYTHON" ] || { err "python3 不是 $I074_PYTHON"; return 1; }
}

# 「八之一之二」第 10 列：/proc/locks 上有 supervisor 持有的 FLOCK、starttime 相符、它在自己的 ppid 鏈上、
# sentinel 與協定變數相同。⛔ 不用「再 flock 一次」。⚠️ 只用標準庫（⛔ 不 import repo 模組）。
VERIFY_LOCKS_PY='
import json, os, stat, sys
lock, sentinel, pid, start, token, mode = sys.argv[1:7]
pid = int(pid)
def fail(msg):
    print("ERROR: 持鎖驗證：" + msg, file=sys.stderr)
    sys.exit(1)
def fields(p):
    with open("/proc/%d/stat" % p) as fh:
        d = fh.read()
    return d[d.rfind(")") + 2:].split()
try:
    st = os.lstat(lock)
except OSError as exc:
    fail("鎖檔讀不到：%s" % exc)
if not stat.S_ISREG(st.st_mode):
    fail("鎖檔不是一般檔案")
want = "%02x:%02x:%d" % (os.major(st.st_dev), os.minor(st.st_dev), st.st_ino)
held = False
with open("/proc/locks") as fh:
    for line in fh:
        f = line.split()
        if len(f) >= 6 and f[1] == "FLOCK" and f[5] == want and f[4] == str(pid):
            held = True
if not held:
    fail("鎖檔（%s）上沒有 supervisor（pid %d）持有的 FLOCK" % (want, pid))
try:
    if fields(pid)[19] != start:
        fail("supervisor 的 starttime 不符")
except OSError:
    fail("supervisor 不在了")
p, seen = os.getppid(), set()
while p != pid:
    if p <= 1 or p in seen:
        fail("supervisor 不在自己的 ppid 鏈上")
    seen.add(p)
    try:
        p = int(fields(p)[1])
    except OSError:
        fail("ppid 鏈讀不到")
try:
    fd = os.open(sentinel, os.O_RDONLY | os.O_NOFOLLOW)
    with os.fdopen(fd, "rb") as fh:
        obj = json.loads(fh.read(4096))
    with open("/proc/sys/kernel/random/boot_id") as fh:
        boot = fh.read().strip()
except (OSError, ValueError) as exc:
    fail("sentinel 讀不到或讀不懂：%s" % exc)
if not isinstance(obj, dict) or obj.get("token") != token or obj.get("supervisor_pid") != pid \
        or obj.get("supervisor_start_time") != int(start) or obj.get("boot_id") != boot or obj.get("mode") != mode:
    fail("sentinel 與協定變數不符")
'
verify_locks() {
  "$I074_PYTHON" -I -B -c "$VERIFY_LOCKS_PY" "$I074_LOCK_PATH" "$I074_SENTINEL_PATH" "$I074_STAGE2_SUPERVISOR_PID" \
    "$I074_STAGE2_SUPERVISOR_START" "$I074_STAGE2_TOKEN" "$I074_STAGE2_MODE"
}

verify_head_files() {  # $1＝repo；其餘＝檔案（相對路徑）→ 每一個都已追蹤且內容 ＝ HEAD
  local repo="$1" f
  shift
  for f in "$@"; do
    git -C "$repo" ls-files --error-unmatch -- "$f" >/dev/null 2>&1 || { err "$f 沒有被追蹤"; return 1; }
    [ "$(git -C "$repo" cat-file blob "HEAD:$f" | sha256sum)" = "$(sha256sum < "$repo/$f")" ] \
      || { err "$f 的內容 ≠ HEAD 中的版本"; return 1; }
  done
}

# ═════════════════════════════════════════════════════════════════════════════════
# 持鎖階段（真正 repo 的 HEAD 版）
# ═════════════════════════════════════════════════════════════════════════════════
locked_main() {
  local code=$EXIT_ABORT icode=$EXIT_ABORT repo work_canon repo_head freeze_dir report run_head run_freeze
  case " $* " in *" promote "*) code=$EXIT_PROMOTION_FAILED ;; esac
  case "$-" in *p*) ;; *) err "持鎖階段必須以 /bin/bash -p 執行"; exit "$code" ;; esac
  parse_internal locked "$@" || { err "持鎖階段的參數不符"; exit "$code"; }
  # code：參數、layout 之類「排除原因後可重跑」的失敗（promote → 8）；icode：複本完整性（promote → 9，同「八之三」步驟 1）。
  [ "$MODE" != promote ] || { code=$EXIT_PROMOTION_FAILED; icode=$EXIT_PROMOTION_BLOCKED; }
  require_protocol || exit "$code"
  verify_locks || exit "$code"
  repo="$(cd "${BASH_SOURCE[0]%/*}/.." && pwd -P)"
  [ "$repo" = "$I074_STAGE2_REAL_REPO" ] || { err "持鎖階段不在 I074_STAGE2_REAL_REPO 裡執行"; exit "$code"; }
  verify_head_files "$repo" "${ENTRY_FILES[@]}" || exit "$code"

  if [ "$MODE" = run ]; then
    [ "$(realpath -m -- "$WORK")" = "$WORK" ] || { err "--work-dir 不是 canonical"; exit "$code"; }
    mkdir -- "$WORK" || { err "建不了執行目錄（已存在？）：$WORK"; exit "$code"; }
    mkdir -- "$WORK/freeze" "$WORK/logs" "$WORK/state" "$WORK/tmp" "$WORK/bin" "$WORK/run" || exit "$code"
    # 「三」#4：先複製、再驗副本（⛔ 先驗再複製：驗到的與之後用的可能不是同一份）。
    freeze_dir="${FREEZE_ARG%/*}"
    report="$freeze_dir/sizing_report.json"
    [ -f "$FREEZE_ARG" ] && [ ! -L "$FREEZE_ARG" ] && [ -f "$report" ] && [ ! -L "$report" ] \
      || { err "freeze record 或同目錄的 sizing_report.json 不存在"; exit "$code"; }
    cp -- "$FREEZE_ARG" "$WORK/freeze/freeze_record.json" && cp -- "$report" "$WORK/freeze/sizing_report.json" \
      || { err "複製 freeze record／報告失敗"; exit "$code"; }
    repo_head="$("$I074_PYTHON" -B "$repo/python/scripts/i074_stage2_freeze_record.py" check-basic \
                 --record "$WORK/freeze/freeze_record.json")" || { err "freeze record 的基本驗證不過"; exit "$code"; }
    git -C "$repo" cat-file -e "${repo_head}^{commit}" 2>/dev/null \
      || { err "repo_head $repo_head 不在真正 repo 裡"; exit "$code"; }
    say "==> 建立隔離複本：$WORK/repo（detached @ $repo_head）"
    git clone -q --no-hardlinks --no-checkout --template= -- "$repo" "$WORK/repo" \
      || { err "clone 失敗"; exit "$code"; }
    git -C "$WORK/repo" -c core.hooksPath=/dev/null checkout -q --detach "$repo_head" \
      || { err "checkout $repo_head 失敗"; exit "$code"; }
    [ "$(git -C "$WORK/repo" rev-parse HEAD)" = "$repo_head" ] || { err "複本的 HEAD ≠ repo_head"; exit "$code"; }
    [ ! -e "$WORK/repo/.git/objects/info/alternates" ] || { err "複本有 alternates"; exit "$code"; }
    [ "$(git -C "$WORK/repo" remote get-url origin)" = "$repo" ] || { err "複本的 origin ≠ 真正 repo"; exit "$code"; }
  else
    work_canon="$(realpath -e -- "$WORK" 2>/dev/null)" && [ "$work_canon" = "$WORK" ] \
      || { err "--work-dir 不存在或不是 canonical"; exit "$code"; }
    [ -d "$WORK/repo/.git" ] && [ -d "$WORK/freeze" ] && [ -f "$WORK/state/run.json" ] \
      || { err "執行目錄的 layout 不符（repo／freeze／state/run.json）"; exit "$code"; }
    [ "$("$I074_PYTHON" -I -B -c 'import json,sys; print(json.load(open(sys.argv[1]))["work_dir"])' \
          "$WORK/state/run.json" 2>/dev/null)" = "$WORK" ] || { err "state/run.json 記的 work_dir ≠ $WORK"; exit "$code"; }
    # 凍結的 repo_head：freeze record 副本（以 HEAD 版的驗證器驗）＝ state/run.json 記的，副本的 SHA 也 ＝ 記錄值。
    repo_head="$("$I074_PYTHON" -B "$repo/python/scripts/i074_stage2_freeze_record.py" check-basic \
                 --record "$WORK/freeze/freeze_record.json")" || { err "freeze record 副本的基本驗證不過"; exit "$icode"; }
    read -r run_head run_freeze < <("$I074_PYTHON" -I -B -c '
import json, sys
p = json.load(open(sys.argv[1]))
print(p.get("repo_head", ""), p.get("freeze_record_sha256", ""))' "$WORK/state/run.json" 2>/dev/null) || true
    [ "$run_head" = "$repo_head" ] && [ "$run_freeze" = "$(sha256sum < "$WORK/freeze/freeze_record.json" | cut -d' ' -f1)" ] \
      || { err "freeze record 副本或 repo_head 與 state/run.json 不符"; exit "$icode"; }
    git -C "$repo" cat-file -e "${repo_head}^{commit}" 2>/dev/null || { err "repo_head $repo_head 不在真正 repo 裡"; exit "$icode"; }
  fi
  # ⚠️ ⑦b 實作第一輪 review（中）：⛔ 讓複本的 orchestrator「先執行、再自己驗自己」——持鎖階段（HEAD 版）在 exec 之前，
  #    以凍結的 repo_head 驗：複本 HEAD ＝ repo_head，且要執行的檔案內容 ＝ **真正 repo 的物件**裡那個 commit 的 blob。
  [ "$(git -C "$WORK/repo" rev-parse --verify HEAD 2>/dev/null)" = "$repo_head" ] \
    || { err "複本的 HEAD ≠ freeze record 的 repo_head"; exit "$icode"; }
  [ -f "$WORK/repo/$SELF_REL" ] && [ ! -L "$WORK/repo/$SELF_REL" ] \
    || { err "複本裡沒有 $SELF_REL"; exit "$icode"; }
  [ "$(git -C "$repo" cat-file blob "$repo_head:$SELF_REL" 2>/dev/null | sha256sum)" = "$(sha256sum < "$WORK/repo/$SELF_REL")" ] \
    || { err "複本的 $SELF_REL ≠ repo_head 中的版本（以真正 repo 的物件比對）——⛔ 執行"; exit "$icode"; }
  exec "$I074_BASH" -p "$WORK/repo/$SELF_REL" --internal-in-clone "$MODE" --work-dir "$WORK"
}

# ═════════════════════════════════════════════════════════════════════════════════
# 複本內 orchestrator（複本 repo_head 版）
# ═════════════════════════════════════════════════════════════════════════════════
CLONE=""; STATE=""; REPO_HEAD=""; SHIM_READY=0; STEP_PID=""; FAIL=$EXIT_ABORT
IDENTITY=""; BASE=""; BUNDLE_ID=""; SEM=""; REPLAY_RC=""

pf() { "$I074_PYTHON" -B "$CLONE/python/scripts/i074_stage2_preflight.py" "$@"; }
fr() { "$I074_PYTHON" -B "$CLONE/python/scripts/i074_stage2_freeze_record.py" "$@"; }
sha() { sha256sum < "$1" | cut -d' ' -f1; }
blob_sha() { git -C "$CLONE" cat-file blob "$REPO_HEAD:$1" | sha256sum | cut -d' ' -f1; }
json_field() {  # $1＝檔案、$2＝鍵（⚠️ 只用標準庫）
  "$I074_PYTHON" -I -B -c 'import json,sys; v=json.load(open(sys.argv[1])).get(sys.argv[2]); print(v if isinstance(v,str) else "")' "$1" "$2"
}
abort() { err "$*"; exit "$FAIL"; }

# preflight 0：複本自身完整性。⚠️ **只用 git、shell 與內嵌標準庫**（⛔ 不 import repo 模組——它們本身可能被改過）。
integrity() {
  local self out entry path listing
  self="$(realpath -e -- "${BASH_SOURCE[0]}")" || return 1
  [ "$self" = "$CLONE/$SELF_REL" ] || { err "orchestrator 不在複本內執行：$self"; return 1; }
  [ ! -e "$CLONE/.git/objects/info/alternates" ] || { err "複本有 alternates"; return 1; }
  [ "$(git -C "$CLONE" remote get-url origin)" = "$I074_STAGE2_REAL_REPO" ] || { err "複本的 origin ≠ 真正 repo"; return 1; }
  REPO_HEAD="$("$I074_PYTHON" -I -B -c '
import json, re, sys
v = json.load(open(sys.argv[1])).get("repo_head")
sys.exit(1) if not (isinstance(v, str) and re.fullmatch("[0-9a-f]{40}", v)) else print(v)' \
    "$WORK/freeze/freeze_record.json" 2>/dev/null)" || { err "freeze record 副本讀不到 repo_head"; return 1; }
  [ "$(git -C "$CLONE" rev-parse --verify HEAD)" = "$REPO_HEAD" ] || { err "複本的 HEAD ≠ freeze record 的 repo_head"; return 1; }
  out="$(git -C "$CLONE" status --porcelain --untracked-files=no)" || { err "git status 失敗"; return 1; }
  [ -z "$out" ] || { err "複本的已追蹤檔被修改：$out"; return 1; }
  listing="$WORK/tmp/.integrity-status"
  git -C "$CLONE" status --porcelain=v1 -z --untracked-files=all --ignored > "$listing" \
    || { rm -f -- "$listing"; err "git status 失敗"; return 1; }
  while IFS= read -r -d '' entry; do
    case "$entry" in
      '?? '*|'!! '*) path="${entry:3}"
        case "$path" in "$STAGE2_REL"/*) ;; *) rm -f -- "$listing"; err "複本在 $STAGE2_REL 以外有未追蹤（或 ignored）的項目：$path"; return 1 ;; esac ;;
    esac
  done < "$listing"
  rm -f -- "$listing"
  [ "$(git -C "$CLONE" cat-file blob "$REPO_HEAD:$SELF_REL" | sha256sum)" = "$(sha256sum < "$self")" ] \
    || { err "orchestrator 自身的內容 ≠ repo_head 中的版本"; return 1; }
  if [ -f "$STATE/run.json" ]; then
    [ "$(json_field "$STATE/run.json" freeze_record_sha256)" = "$(sha "$WORK/freeze/freeze_record.json")" ] \
      || { err "freeze record 副本的 SHA ≠ state/run.json"; return 1; }
    [ "$(json_field "$STATE/run.json" report_sha256)" = "$(sha "$WORK/freeze/sizing_report.json")" ] \
      || { err "報告副本的 SHA ≠ state/run.json"; return 1; }
  fi
  if [ "$SHIM_READY" = 1 ]; then
    [ "$(ls -A -- "$WORK/bin")" = docker ] || { err "$WORK/bin 底下不是恰好只有 docker"; return 1; }
    [ "$(blob_sha "$SHIM_REL")" = "$(sha "$WORK/bin/docker")" ] || { err "$WORK/bin/docker ≠ 複本的 label shim"; return 1; }
    hash -r
    [ "$(command -v docker)" = "$WORK/bin/docker" ] || { err "docker 沒有解析到 label shim"; return 1; }
  else
    [ -z "$(ls -A -- "$WORK/bin")" ] || { err "label shim 安裝之前 $WORK/bin 就不是空的"; return 1; }
  fi
}

# 檢查點：/proc/locks → preflight 0 →（有 state 時）state 的鏈與交叉條件、目前的 image 與 identity、凍結 patch 與輸出。
checkpoint() {  # $1＝必須存在的 state（逗號分隔）
  verify_locks || return 1
  integrity || return 1
  pf state-check --state-dir "$STATE" --work-dir "$WORK" --real-repo "$I074_STAGE2_REAL_REPO" --require "$1" \
    --image "$REPLAY_IMAGE_ID" --identity "$IDENTITY" --verify-files > "$WORK/tmp/.state-check" || return 1
  local k v
  while IFS='=' read -r k v; do
    case "$k" in bundle_id) BUNDLE_ID="$v" ;; semantic) SEM="$v" ;; rc) REPLAY_RC="$v" ;; esac
  done < "$WORK/tmp/.state-check"
}

# 長步驟一律背景 ＋ wait（bash 要等前景命令結束才處理 trap）。$1＝log 名稱；其餘＝指令。
run_step() {
  local name="$1" rc=0
  shift
  "$@" >"$WORK/logs/$name.out" 2>"$WORK/logs/$name.err" &
  STEP_PID=$!
  wait "$STEP_PID" || rc=$?
  STEP_PID=""
  return "$rc"
}

on_signal() {  # $1＝訊號編號
  local ids i
  trap - TERM INT HUP
  if [ -n "$STEP_PID" ]; then
    kill -TERM "$STEP_PID" 2>/dev/null || true
    for i in $(seq 1 50); do kill -0 "$STEP_PID" 2>/dev/null || break; sleep 0.1; done
  fi
  if [ "$SHIM_READY" = 1 ]; then
    ids="$(docker ps -a -q --filter "label=$I074_LABEL_KEY=$I074_STAGE2_TOKEN" 2>/dev/null)" || ids=""
    [ -z "$ids" ] || docker rm -f $ids >/dev/null 2>&1 || true
  fi
  err "被訊號 $1 中斷（其餘的後代與容器由 supervisor 收尾）"
  exit $((128 + $1))
}

install_shim() {
  cp -- "$CLONE/$SHIM_REL" "$WORK/bin/docker" && chmod 0555 "$WORK/bin/docker" || return 1
  SHIM_READY=1
  hash -r
  [ "$(command -v docker)" = "$WORK/bin/docker" ] || { err "docker 沒有解析到 label shim"; return 1; }
  integrity
}

freeze_patches() {  # preflight 1
  local name rel dst
  mkdir -p -- "$WORK/run/patches" || return 1
  [ ! -e "$WORK/run/stage2" ] && [ ! -L "$WORK/run/stage2" ] || { err "$WORK/run/stage2 ⛔ 不得預先存在"; return 1; }
  for name in counterfactual tooling; do
    if [ "$name" = counterfactual ]; then rel=$CF_PATCH_REL; dst="$WORK/run/$FROZEN_CF"; else rel=$TOOLING_PATCH_REL; dst="$WORK/run/$FROZEN_TOOLING"; fi
    [ ! -e "$dst" ] || { err "凍結副本已存在：$dst"; return 1; }
    cp -- "$CLONE/$rel" "$dst" || return 1
    [ "$(sha "$dst")" = "$(blob_sha "$rel")" ] || { err "$rel 的凍結副本 ≠ repo_head 中的版本"; return 1; }
  done
  [ -s "$WORK/run/$FROZEN_TOOLING" ] || { err "tooling patch ⛔ 不得為空（ba 的 orchestrator 層）"; return 1; }
}

compose_and_check_freeze_record() {  # preflight 2
  local wt line rc=0
  BASE="$(json_field "$WORK/freeze/freeze_record.json" base_commit)"
  [[ "$BASE" =~ ^[0-9a-f]{40}$ ]] || { err "freeze record 的 base_commit 讀不到"; return 1; }
  # shellcheck source=lib/replay-args.sh
  . "$CLONE/scripts/lib/replay-args.sh"
  wt="$(mktemp -d "$WORK/tmp/compose.XXXXXX")" && rmdir -- "$wt" || return 1
  if ! replay_args_prepare_worktree "$CLONE" "$BASE" "$wt" >/dev/null; then
    err "建不了 preflight 的暫時 worktree"; rc=1
  elif ! line="$(replay_args_compose "$wt" "$BASE" "$WORK/run/$FROZEN_CF" "$WORK/run/$FROZEN_TOOLING")"; then
    err "兩份凍結 patch 的合成失敗"; rc=1
  else
    read -r C_T1 C_T2 C_CF C_TOOL C_COMP C_SEM <<< "$line"
    fr check-full --record "$WORK/freeze/freeze_record.json" --report "$WORK/freeze/sizing_report.json" --clone "$CLONE" \
      --frozen-counterfactual "$WORK/run/$FROZEN_CF" --frozen-tooling "$WORK/run/$FROZEN_TOOLING" \
      --counterfactual-canonical "$C_CF" --tooling-canonical "$C_TOOL" --identity "$IDENTITY" --image "$REPLAY_IMAGE_ID" \
      || rc=1
  fi
  # ⚠️ replay 之前唯一的清理：**成功或失敗都移除**（⑦b 實作第一輪 review），移除失敗本身也是 1；
  #    replay 開始之後⛔ 不做任何 worktree 清理。
  if [ -e "$wt" ] || grep -qxF "worktree $wt" <<< "$(git -C "$CLONE" worktree list --porcelain)"; then
    git -C "$CLONE" worktree remove --force "$wt" || { err "移除 preflight 的暫時 worktree 失敗"; rc=1; }
  fi
  [ "$rc" != 0 ] || SEM="$C_SEM"
  return "$rc"
}

anchors() {  # preflight 3（容器內，目前的 identity）
  local mem
  # shellcheck source=lib/mem-guard.sh
  . "$CLONE/scripts/lib/mem-guard.sh"
  mem="$(mem_guard_clamp 700m)"
  run_step anchors docker run --rm --network none --read-only --user "$(id -u):$(id -g)" --cpus=1 \
    --memory="$mem" --memory-swap="$mem" --pids-limit=200 -e HOME=/tmp -e PYTHONDONTWRITEBYTECODE=1 -e PYTHONPATH=/app \
    -v "$CLONE/python":/app:ro -v "$IDENTITY":/identity/run_identity.json:ro -w /app "$REPLAY_IMAGE_ID" \
    python scripts/i074_stage2_preflight.py anchors --python-root /app --run-identity /identity/run_identity.json \
    --image-digest "$REPLAY_IMAGE_ID" || { err "信任錨（anchors）不通過：$(tail -3 "$WORK/logs/anchors.err")"; return 1; }
  [ "$(json_field "$WORK/logs/anchors.out" after_base_commit)" = "$BASE" ] \
    || { err "Stage 1 after 的 base_commit ≠ freeze record 的 base_commit"; return 1; }
  BUNDLE_ID="$(json_field "$WORK/logs/anchors.out" bundle_id)"
  [ -n "$BUNDLE_ID" ] && [ -d "$CLONE/python/baselines/$BUNDLE_ID" ] || { err "bundle 目錄不在複本裡：$BUNDLE_ID"; return 1; }
}

real_repo_checks() {  # preflight 4 的真正 repo 兩條（「八之一」的例外）
  local out real="$I074_STAGE2_REAL_REPO" line
  out="$(git --no-optional-locks -C "$real" status --porcelain --untracked-files=all --ignored -- "$STAGE2_REL/")" \
    || { err "讀不到真正 repo 的 git status"; return 1; }
  [ -z "$out" ] || { err "真正 repo 的 $STAGE2_REL/ 底下有未追蹤（或 ignored）的項目——先 commit 已晉升的 failed record：$out"; return 1; }
  local real_failed clone_failed
  real_failed="$(git --no-optional-locks -C "$real" ls-tree -r HEAD -- "$STAGE2_REL/failed/")" || return 1
  clone_failed="$(git -C "$CLONE" ls-tree -r "$REPO_HEAD" -- "$STAGE2_REL/failed/")" || return 1
  while IFS= read -r line; do
    [ -n "$line" ] || continue
    grep -qxF -- "$line" <<< "$clone_failed" \
      || { err "真正 repo HEAD 的 failed record 不在複本的 repo_head 裡（freeze record 太舊）：$line"; return 1; }
  done <<< "$real_failed"
}

lookup() {  # preflight 4、5
  local rc=0
  run_step check_failed_record "$CLONE/scripts/finalize-stage2-evidence.sh" --check-failed-record \
    --counterfactual-patch "$WORK/run/$FROZEN_CF" || rc=$?
  case "$rc" in
    0) return 0 ;;
    2) err "failed-record lookup 命中：同一個語意 SHA 已有失敗紀錄——⛔ 不得重跑"; exit "$EXIT_LOOKUP_HIT" ;;
    *) err "failed-record 的檢查失敗（rc=$rc）：$(tail -3 "$WORK/logs/check_failed_record.err")"; return 1 ;;
  esac
}

disk() {  # preflight 6
  local root
  root="$(docker info --format '{{.DockerRootDir}}' 2>/dev/null)" && [ -n "$root" ] \
    || { err "取不到 Docker Root Dir——⛔ fail-closed"; return 1; }
  pf disk --location "run=$WORK/run" --location "tmp=$WORK/tmp" --location "baselines=$CLONE/$STAGE2_REL" \
    --location "git=$CLONE/.git" --docker-root "$root" >/dev/null
}

terminal_exists() {
  local d
  case "$REPLAY_RC" in
    0) d="$CLONE/$STAGE2_REL/evidence" ;;
    6) d="$CLONE/$STAGE2_REL/failed/$BUNDLE_ID-$SEM" ;;
    *) return 1 ;;
  esac
  [ -d "$d" ] && [ ! -L "$d" ]
}

promote_stub() {
  say "==> ⚠️ 晉升尚未實作（⑦c）：終態留在複本 $CLONE 內，⛔ 沒有搬進真正 repo"
  exit "$EXIT_PROMOTION_BLOCKED"
}

finalize_or_publish() {  # 依 replay 的 rc；之後依磁碟事實判終態
  local rc=0 kind name
  if [ "$REPLAY_RC" = 0 ]; then
    kind=finalize; name=finalize
    run_step finalize "$CLONE/scripts/finalize-stage2-evidence.sh" --finalize --run-dir "$WORK/run" || rc=$?
  else
    kind=publish_failed_record; name=publish_failed_record
    run_step publish_failed_record "$CLONE/scripts/finalize-stage2-evidence.sh" --publish-failed-record \
      --run-dir "$WORK/run" || rc=$?
  fi
  say "    $name 結束（rc=$rc）——⚠️ 終態依磁碟事實判定，⛔ 不看結束碼"
  if terminal_exists; then
    checkpoint run,preflight,replay_started,replay_done || abort "晉升之前的檢查點不符"
    promote_stub
  fi
  if [ "$rc" = 1 ]; then
    pf state-write-attempt --state-dir "$STATE" --work-dir "$WORK" --real-repo "$I074_STAGE2_REAL_REPO" --kind "$kind" \
      || abort "寫不了 state/attempt.json"
    abort "$name 回 1 且沒有終態：log 在 $WORK/logs/$name.err；排除原因後可以 --resume"
  fi
  abort "$name 回 $rc 但磁碟上沒有終態（矛盾）——⛔ 不寫 attempt"
}

clone_main() {
  local code rc=0 argv
  case " $* " in *" promote "*) FAIL=$EXIT_PROMOTION_BLOCKED ;; esac
  case "$-" in *p*) ;; *) abort "複本內 orchestrator 必須以 /bin/bash -p 執行" ;; esac
  parse_internal clone "$@" || abort "複本內 orchestrator 的參數不符"
  [ "$MODE" != promote ] || FAIL=$EXIT_PROMOTION_BLOCKED
  require_protocol || abort "協定或 PATH 不符"
  verify_locks || abort "持鎖驗證不過——⛔ 不執行任何步驟"
  CLONE="$WORK/repo"; STATE="$WORK/state"
  IDENTITY="${XDG_DATA_HOME:-$HOME/.local/share}/stock_trading/i074_stage2/run_identity.json"
  [ -f "$IDENTITY" ] && [ ! -L "$IDENTITY" ] || abort "找不到 Stage 2 的 run identity：$IDENTITY"
  export TMPDIR="$WORK/tmp"
  trap 'on_signal 15' TERM
  trap 'on_signal 2' INT
  trap 'on_signal 1' HUP
  [ "$MODE" = run ] || SHIM_READY=1
  integrity || abort "preflight 0：複本完整性不符——⛔ 不執行任何 validator、⛔ 不進 replay"

  case "$MODE" in
    promote)
      checkpoint run || abort "晉升之前的檢查點不符"
      promote_stub ;;
    resume)
      checkpoint run,preflight,replay_started,replay_done,attempt \
        || abort "--resume 的 state 不符（缺少、被改、或 image／identity 變了）——⛔ 不呼叫 finalizer；請改用 --promote"
      if terminal_exists; then promote_stub; fi
      say "==> --resume：沿用同一份凍結 patch 與 operational 輸出（⛔ 不 replay）"
      finalize_or_publish ;;
  esac

  # ── run ──────────────────────────────────────────────────────────────────────
  pf state-write-run --state-dir "$STATE" --work-dir "$WORK" --real-repo "$I074_STAGE2_REAL_REPO" \
    || abort "寫不了 state/run.json（freeze record 的封閉 schema？）"
  install_shim || abort "安裝 label shim 失敗"
  say "==> preflight 1：凍結兩份 patch"
  freeze_patches || abort "preflight 1 不通過"
  say "==> preflight 2：freeze record 完整驗證（合成兩份凍結 patch）"
  compose_and_check_freeze_record || abort "preflight 2 不通過"
  say "==> preflight 3：信任錨（目前的 Stage 2 identity）"
  anchors || abort "preflight 3 不通過"
  say "==> preflight 4：真正 repo 的 failed record ＋ lookup"
  real_repo_checks || abort "preflight 4 不通過"
  lookup || abort "preflight 4 不通過"
  say "==> preflight 6：磁碟"
  disk || abort "preflight 6（磁碟）不通過"
  pf state-write-preflight --state-dir "$STATE" --work-dir "$WORK" --real-repo "$I074_STAGE2_REAL_REPO" \
    --counterfactual-sha256 "$C_CF" --tooling-sha256 "$C_TOOL" --composed-sha256 "$C_COMP" --semantic-sha256 "$C_SEM" \
    --identity "$IDENTITY" --image "$REPLAY_IMAGE_ID" --anchors "$WORK/logs/anchors.out" \
    || abort "寫不了 state/preflight.json"

  mapfile -d '' argv < <(pf state-write-replay-started --state-dir "$STATE" --work-dir "$WORK" \
                           --real-repo "$I074_STAGE2_REAL_REPO")
  [ -f "$STATE/replay_started.json" ] && [ "${#argv[@]}" -gt 0 ] || abort "寫不了 state/replay_started.json"
  say "==> replay（log：$WORK/logs/replay.{out,err}）"
  # ⚠️ 兩份 patch 的環境變數**只**出現在這一次呼叫（finalizer 會對 HEAD worktree 套 TOOLING_PATCH）。
  run_step replay env I074_STAGE=2 COUNTERFACTUAL_PATCH="$WORK/run/$FROZEN_CF" TOOLING_PATCH="$WORK/run/$FROZEN_TOOLING" \
    "${argv[@]}" || rc=$?
  say "    replay 結束（rc=$rc）"
  case "$rc" in
    0|6) ;;
    *) abort "replay 回 $rc（⛔ 沒有終態、⛔ 不能 --resume）：$(tail -3 "$WORK/logs/replay.err")" ;;
  esac
  pf state-write-replay-done --state-dir "$STATE" --work-dir "$WORK" --real-repo "$I074_STAGE2_REAL_REPO" --rc "$rc" \
    || abort "replay rc=$rc 的輸出形狀不符（⛔ 沒有終態、⛔ 不能 --resume）"
  # ⚠️ replay 開始之後⛔ 不做任何 worktree 清理：殘留只在複本的 .git 與 <work>/tmp，保留到整個 <work> 被刪除。
  checkpoint run,preflight,replay_started,replay_done \
    || abort "replay 之後的檢查點不符——⛔ 不發布、⛔ 不能 --resume；停下、另立 issue"
  finalize_or_publish
}

case "$ROLE" in
  entry) entry_main "$@" ;;
  locked) locked_main "$@" ;;
  clone) clone_main "$@" ;;
esac
