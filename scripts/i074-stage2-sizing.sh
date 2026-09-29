#!/usr/bin/env bash
# I-074 Stage 2 步驟 ④：sizing harness（issue.md I-074「Stage 2 步驟 ④：sizing harness 計畫書」v7 ＋ 差異 1）。
#
#   REPLAY_IMAGE_ID=sha256:… scripts/i074-stage2-sizing.sh --work-dir <repo 外、尚不存在的目錄> [--formal]
#
# 量出 P_B = max(P_witness, P_success, P_failure) 與階段二每個程序的記憶體峰值。
# ⛔ 不是正式的證據入口；⛔ 不含可用空間檢查（preflight 屬 ⑦，M_safety 在 ⑤ 裁定）。
#
# 演練用的故障注入（⚠️ 只給 ④ 的中止點演練，`--formal` 一律拒絕）：I074_SIZING_FAULT=twins（在 metadata twin
#   那一步中止）、copy（中止時讓「複製 S」失敗）、summary（中止時讓失敗摘要寫不出來），或 cleanup（在 work 目錄建好後
#   放一份假 cidfile 並中止——搭配移除不了的 docker 驗證容器清理失敗時 S 與 cidfile 都保留）、stuck（放一個忽略
#   TERM 的步驟再中止——驗證升級 KILL）、group-alive（模擬 KILL 之後仍有成員——驗證保留 S），或 final-check
#   （直接執行成功路徑結束前的容器檢查——搭配 ps 失敗的 docker 驗證「查不到不能當作沒有」），或 orphan-ok／
#   orphan-bad（leader 以預期／非預期的結束碼退出、背景子程序仍留在 group 裡——驗證一律收尾並中止）。
#
# 模式：預設是 ④ 的**可用性驗證**（允許未 commit，報告記錄實際執行的腳本 SHA-256）；
#       `--formal` 是 ⑤ 的**正式量測**（scripts/、python/、.gitattributes 必須 clean，harness 檔案必須等於 HEAD）。
#
# 做法重點（細節見計畫書）：
#   - 全部在 <work>/repo（`git clone --no-hardlinks` 的 HEAD）裡執行正式入口——路徑常數由腳本位置推導，
#     ⛔ 不會寫到真正的 repo；
#   - harness 自己的狀態全部放 host tmpfs 的 S＝/dev/shm/i074-sizing-<run id>/，⛔ 不落在量測的檔案系統；
#   - 正式入口經 docker shim（唯讀 rootfs、⛔ 無可寫 /tmp、cgroup 峰值、容器足跡），
#     以 TMPDIR=<work>/tmp、PYTHONDONTWRITEBYTECODE=1 執行；
#   - 每條路徑前後各記一份完整 inventory 自我檢查；metadata twin 在所有量測窗口結束後才建；
#   - 任何一步失敗：停取樣 → 以 CID 移除容器 → 結束步驟的 process group（TERM → KILL）→ 再移除一次容器 →
#     窗口標 aborted → S 複製到 <work>/raw-failed/ → failure_summary.json（⛔ 不宣稱 P_B）→
#     group 確實結束、容器清光、複製與摘要都成功才清 S。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
HELPER="$REPO_ROOT/python/scripts/i074_stage2_sizing.py"
SHIM="$REPO_ROOT/scripts/lib/i074-sizing-docker-shim.sh"
export PYTHONDONTWRITEBYTECODE=1
. "$REPO_ROOT/scripts/lib/mem-guard.sh"

die() { echo "ERROR: $*" >&2; exit 1; }

WORK_ARG=""; FORMAL=0
while [ "$#" -gt 0 ]; do
  case "$1" in
    --work-dir)
      [ "$#" -ge 2 ] || die "--work-dir 需要值。"
      [ -z "$WORK_ARG" ] || die "--work-dir 重複出現——⛔ 不靜默採用最後一個。"
      WORK_ARG="$2"; shift 2 ;;
    --formal)
      [ "$FORMAL" = "0" ] || die "--formal 重複出現。"
      FORMAL=1; shift ;;
    *) die "未知參數 $1" ;;
  esac
done
[ -n "$WORK_ARG" ] || die "用法：$0 --work-dir <repo 外、尚不存在的目錄> [--formal]"

IMAGE="${REPLAY_IMAGE_ID:-}"
[ -n "$IMAGE" ] || die "⛔ 一律要求 REPLAY_IMAGE_ID（Stage 2 identity 的 image）。"
[ -z "${PY_IMAGE:-}" ] || die "REPLAY_IMAGE_ID 與 PY_IMAGE ⛔ 不可同時設定。"
REAL_DOCKER="$(command -v docker)" || die "找不到 docker。"
case "$REAL_DOCKER" in /dev/shm/*) die "PATH 裡的 docker 已經是 shim：$REAL_DOCKER" ;; esac
"$REAL_DOCKER" image inspect "$IMAGE" >/dev/null 2>&1 || die "image $IMAGE 不在本機。"

# ── work 目錄防護：canonical path（含 parent symlink 指回 repo）、⛔ 不得已存在 ──────────────
REPO_CANON="$(realpath -- "$REPO_ROOT")"
[ ! -e "$WORK_ARG" ] && [ ! -L "$WORK_ARG" ] || die "--work-dir 已存在：$WORK_ARG（⛔ 不覆蓋）"
WORK="$(realpath -m -- "$WORK_ARG")"
case "$WORK/" in "$REPO_CANON"/*) die "--work-dir 解析後在 repo 內：$WORK（⛔ 會汙染 repo 的 inventory）" ;; esac
case "$REPO_CANON/" in "$WORK"/*) die "--work-dir 是 repo 的上層目錄：$WORK" ;; esac
[ -d "$(dirname "$WORK")" ] || die "--work-dir 的上層目錄不存在：$(dirname "$WORK")"
L0_DEV="$(stat -c %d "$(dirname "$WORK")")"

case "${I074_SIZING_FAULT:-}" in
  ""|twins|copy|summary|cleanup|stuck|group-alive|final-check|orphan-ok|orphan-bad) ;;
  *) die "I074_SIZING_FAULT 只接受 twins／copy／summary／cleanup／stuck／group-alive／final-check／orphan-ok／orphan-bad：${I074_SIZING_FAULT}" ;;
esac
if [ "$FORMAL" = "1" ]; then
  [ -z "${I074_SIZING_FAULT:-}" ] || die "--formal ⛔ 不接受 I074_SIZING_FAULT（那是演練用的故障注入）"
  for fd in 1 2; do
    if [ -f "/proc/$$/fd/$fd" ] && [ "$(stat -L -c %d "/proc/$$/fd/$fd")" = "$L0_DEV" ]; then
      die "--formal：fd $fd 被導到量測中的檔案系統上的一般檔案——harness 的輸出會混進 P_B（請導到 tmpfs 或終端機）"
    fi
  done
  dirty="$(git -C "$REPO_ROOT" status --porcelain -- scripts python .gitattributes)"
  [ -z "$dirty" ] || die "--formal：scripts/、python/、.gitattributes 有未 commit 的變更：
$dirty"
  for f in scripts/i074-stage2-sizing.sh scripts/lib/i074-sizing-docker-shim.sh python/scripts/i074_stage2_sizing.py; do
    git -C "$REPO_ROOT" ls-files --error-unmatch -- "$f" >/dev/null 2>&1 || die "--formal：$f 尚未進版控"
    [ "$(git -C "$REPO_ROOT" show "HEAD:$f" | sha256sum | cut -d' ' -f1)" = "$(sha256sum < "$REPO_ROOT/$f" | cut -d' ' -f1)" ] \
      || die "--formal：$f 與 HEAD 的內容不同"
  done
fi

RUN_ID="$(date -u +%Y%m%dT%H%M%SZ)-$$"
S="/dev/shm/i074-sizing-$RUN_ID"
mkdir -m 700 "$S" || die "建不了 S：$S"
[ "$(stat -f -c %T "$S")" = "tmpfs" ] || { rm -rf "$S"; die "S 不是 tmpfs：$S"; }
mkdir -p "$S/out" "$S/bin" "$S/phases"
LOG="$S/console.log"
say() { printf '%s %s\n' "$(date -u +%H:%M:%S)" "$*" | tee -a "$LOG" >&2; }
meta() { printf '%s\t%s\n' "$1" "$2" >> "$S/meta.tsv"; }
comp() { printf '%s\t%s\n' "$1" "$2" >> "$S/components.tsv"; }
helper() { python3 "$HELPER" "$@"; }

PHASE=""; SAMPLER_PID=""; STEP_PID=""; DONE=0
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
# ⛔ **不能先 `wait` leader**：leader 忽略 TERM 時 wait 會一直卡住、逾時與 KILL 永遠輪不到（第三輪 review 的測試抓到）。
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
#   成員時交給 on_failure() 以同一個 PGID 收尾（第四輪 review：原本 wait 之後就清空 STEP_PID，殘留程序沒有人收）。
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
}
# 成功路徑用：本次 run id 的容器必須一個都不剩。⚠️ `docker ps` 本身失敗 ⇒ 查不到就⛔ 不能當作沒有。
ensure_no_run_containers() {  # $1＝說明
  local out
  if ! out="$("$REAL_DOCKER" ps -a --filter "name=i074sz-$RUN_ID-" -q 2>>"$S/cleanup.err")"; then
    on_failure "$1：docker ps 失敗（⛔ 查不到就不能當作沒有殘留）" 1
  fi
  [ -z "$out" ] || on_failure "$1：仍有本次的容器 $(printf '%s ' $out)" 1
}
on_failure() {  # $1＝失敗的階段、$2＝結束碼。⚠️ 全部發生在量測窗口終止之後。
  local stage="$1" rc="$2" cleaned=0 copied=0 summarized=0 group_stopped=1
  trap - EXIT INT TERM
  stop_sampler                                                       # 1
  cleanup_containers || true                                         # 2：先停掉還在跑的容器
  # 2b：進行中的步驟在自己的 session 裡（setsid），整個 process group 一起結束，並**等它真的結束**
  #     （shim 的中斷處理會補寫 sidecar——⛔ 不能在它寫完之前就複製 S）。
  if [ -n "$STEP_PID" ]; then
    stop_step_group "$STEP_PID" || group_stopped=0
    STEP_PID=""
  fi
  cleanup_containers && cleaned=1                                    # 2c：group 結束後再清一次——這一次才算數
  [ -n "$PHASE" ] && [ -d "$S/phases/$PHASE" ] && touch "$S/phases/$PHASE/aborted"   # 3
  say "中止：階段 $stage、結束碼 $rc"
  if [ -d "$WORK" ] && [ "${I074_SIZING_FAULT:-}" != copy ] && cp -a "$S" "$WORK/raw-failed" 2>/dev/null; then  # 4
    copied=1
  fi
  if [ -d "$WORK" ] && [ "${I074_SIZING_FAULT:-}" != summary ] \
     && helper failure-summary --state "$S" --stage "$stage" --rc "$rc" --out "$WORK/failure_summary.json" \
          --leftover-cids "$S/leftover-cids"; then                     # 5
    summarized=1
  fi
  if [ "$cleaned" = 0 ]; then
    echo "⚠️ 有容器移除不了（CID 如下），請手動 docker rm -f：" >&2
    sort -u "$S/leftover-cids" >&2
  fi
  # 6：四者都成功才清 S——⚠️ 步驟的 process group 沒有確實結束時，仍存活的程序可能還在寫 S。
  if [ "$group_stopped" = 1 ] && [ "$cleaned" = 1 ] && [ "$copied" = 1 ] && [ "$summarized" = 1 ]; then
    rm -rf "$S"
    echo "原始量測：$WORK/raw-failed/；摘要：$WORK/failure_summary.json（⛔ 不宣稱 P_B）" >&2
  else
    echo "⚠️ 步驟結束（$group_stopped）、容器清理（$cleaned）、原始量測的複製（$copied）或失敗摘要（$summarized）" \
         "沒有全部成功——保留 S：$S" >&2
  fi
  exit 1
}
trap 'on_failure signal-INT 130' INT
trap 'on_failure signal-TERM 143' TERM
trap '[ "$DONE" = 1 ] || on_failure "未預期的結束" "$?"' EXIT

sha_of() { sha256sum < "$1" | cut -d' ' -f1; }
alloc() { helper allocated "$1" | python3 -c 'import json,sys; print(json.load(sys.stdin)["allocated"])'; }

say "==> sizing harness：run id $RUN_ID、模式 $([ "$FORMAL" = 1 ] && echo formal || echo validation)、S=$S"
meta run_id "$RUN_ID"; meta mode "$([ "$FORMAL" = 1 ] && echo formal || echo validation)"
meta work "$WORK"; meta repo "$REPO_CANON"; meta image "$IMAGE"; meta state_fstype tmpfs
meta repo_head "$(git -C "$REPO_ROOT" rev-parse HEAD)"
meta repo_dirty "$([ -n "$(git -C "$REPO_ROOT" status --porcelain)" ] && echo true || echo false)"
meta harness_sha256 "$(sha_of "$REPO_ROOT/scripts/i074-stage2-sizing.sh")"
meta shim_sha256 "$(sha_of "$SHIM")"
meta helper_sha256 "$(sha_of "$HELPER")"
IDENTITY="${XDG_DATA_HOME:-$HOME/.local/share}/stock_trading/i074_stage2/run_identity.json"
[ -f "$IDENTITY" ] && meta identity_sha256 "$(sha_of "$IDENTITY")"

# ── 0. 準備 ────────────────────────────────────────────────────────────────────
mkdir "$WORK"
case "${I074_SIZING_FAULT:-}" in
  cleanup)
    mkdir -p "$S/cid"; printf 'fault-injected-cid' > "$S/cid/o0000.cid"
    on_failure "容器清理（注入的故障）" 1 ;;
  stuck|group-alive)
    # 一個忽略 TERM 的「步驟」（⚠️ exec 之後 TERM 仍是忽略狀態）
    setsid sh -c 'trap "" TERM; exec sleep 60' &
    STEP_PID=$!
    sleep 0.5
    on_failure "步驟收不掉（注入的故障：$I074_SIZING_FAULT）" 1 ;;
  final-check)
    ensure_no_run_containers "結束前的容器檢查（注入）"
    DONE=1; rm -rf "$S"; exit 0 ;;
  orphan-ok|orphan-bad)
    # leader 自己退出（預期碼 0／非預期碼 7），背景的 sleep 留在同一個 group 裡繼續活著
    run_in_group fault_orphan 0 sh -c "sleep 60 & exit $([ "$I074_SIZING_FAULT" = orphan-ok ] && echo 0 || echo 7)"
    DONE=1; rm -rf "$S"; exit 0 ;;   # ⛔ 走到這裡＝殘留的成員沒有被擋下
esac
# ⚠️ **⛔ 不用 `--shared`**（差異 2，2026-09-24 第一次實跑被自我檢查擋下）：`--shared` 以 alternates 共用真正 repo 的
#   物件，而 `git write-tree`／`git apply --index` 寫到**已存在**的物件時會 freshen（touch）含有它的 pack——
#   改到的是真正 repo 的 pack 的 mtime。`--no-hardlinks` 完整複製物件（⛔ 也不共用 inode），git 的寫入全落在 L5。
git clone -q --no-hardlinks "$REPO_ROOT" "$WORK/repo"
CLONE="$WORK/repo"
L1="$WORK/runs"; L2="$CLONE/python/baselines/i074_stage2"; L3="$WORK/tmp"; L5="$CLONE/.git"
CACHE="$WORK/cache"
mkdir -p "$L1" "$L3" "$CACHE"
meta clone_head "$(git -C "$CLONE" rev-parse HEAD)"
meta finalizer_sha256 "$(sha_of "$CLONE/scripts/finalize-stage2-evidence.sh")"
meta claims_sha256 "$(sha_of "$CLONE/python/scripts/i074-stage2-patch-claims.py")"
meta l0_dev "$L0_DEV"
meta repo_dev "$(stat -c %d "$REPO_CANON")"
DROOT="$("$REAL_DOCKER" info --format '{{.DockerRootDir}}')"
DROOT_C="$(realpath -- "$DROOT")"
meta docker_root "$DROOT_C"; meta docker_root_dev "$(stat -c %d "$DROOT_C")"
[ "$(stat -c %d "$DROOT_C")" = "$L0_DEV" ] || on_failure "L4：Docker Root Dir 不在同一個檔案系統" 1
meta logging_default "$("$REAL_DOCKER" info --format '{{.LoggingDriver}}')"
MEM="$(mem_guard_clamp "${MEM:-700m}" 2>/dev/null)"; meta fixture_mem "$MEM"

cp -a "$L2/envcheck" "$CACHE/envcheck"      # 見證路徑要從「還沒有 envcheck/」開始
rm -rf "$L2/envcheck"
BASE="$(helper anchor-base --python-root "$CLONE/python")" || on_failure "取不到 Stage 1 after 的 base" 1
meta base_commit "$BASE"
CF_PATCH="$L2/counterfactual_e1cbbbd.patch"
meta counterfactual_sha256 "$(sha_of "$CF_PATCH")"
. "$CLONE/scripts/lib/replay-args.sh"

new_worktree() {  # $1＝ref；$2＝patched（0／1）→ 印出 worktree 路徑（在 L3）
  local wt
  wt="$(mktemp -d "$L3/tmp.XXXXXXXXXX")"; rmdir "$wt"
  replay_args_prepare_worktree "$CLONE" "$1" "$wt" >/dev/null
  if [ "$2" = 1 ]; then
    git -C "$wt" apply --index "$CF_PATCH"
    git -C "$wt" add -A -N >/dev/null 2>&1 || true
    git -C "$wt" write-tree >/dev/null
  fi
  printf '%s\n' "$wt"
}
say "==> 步驟 0：實建三種 worktree、快照與 probe 結構量 allocated bytes"
for spec in head:HEAD:0 base:$BASE:0 base_patched:$BASE:1; do
  IFS=: read -r kind ref patched <<< "$spec"
  g0="$(alloc "$L5")"
  wt="$(new_worktree "$ref" "$patched")"
  comp "wt_$kind" "$(alloc "$wt")"
  comp "git_$kind" "$(( $(alloc "$L5") - g0 ))"
  comp "index_$kind" "$(alloc "$L5/worktrees/$(basename "$wt")/index")"
  git -C "$CLONE" worktree remove --force "$wt"
done
snap="$(mktemp -d "$L3/tmp.XXXXXXXXXX")"
cp -- "$CF_PATCH" "$snap/counterfactual.patch"; : > "$snap/tooling.patch"
comp snapshot "$(alloc "$snap")"; rm -rf "$snap"
probe="$(mktemp -d "$L2/.sizing-probe.XXXXXXXX")"
mkdir "$probe/a" "$probe/b-src" "$probe/b-dst"
printf a > "$probe/a/marker"; printf src > "$probe/b-src/marker"; printf dst > "$probe/b-dst/marker"
comp probe "$(( $(alloc "$probe/a") + $(alloc "$probe/b-src") + $(alloc "$probe/b-dst") ))"
rm -rf "$probe"

# shim
ln -s "$SHIM" "$S/bin/docker"
export SIZING_REAL_DOCKER="$REAL_DOCKER" SIZING_STATE="$S" SIZING_RUN_ID="$RUN_ID" SIZING_IMAGE="$IMAGE" \
       SIZING_HELPER="$HELPER"
SHIM_PATH="$S/bin:$PATH"
LOC_JSON="$(python3 -c 'import json,sys; print(json.dumps(dict(zip(["L1","L2","L3","L5"], sys.argv[1:]))))' "$L1" "$L2" "$L3" "$L5")"
INV_JSON="$(python3 -c 'import json,sys; print(json.dumps(sys.argv[1:]))' "$WORK" "$REPO_CANON")"
SRC_JSON="$(python3 -c 'import json,sys; print(json.dumps(sys.argv[1:]))' "$CLONE")"

# 執行一個步驟並比對結束碼。$1＝步驟名、$2＝預期結束碼、$3＝role、其餘＝指令（經 shim）。
step() {
  local name="$1" want="$2" role="$3"
  shift 3
  say "    $name（預期 $want）"
  run_in_group "$name" "$want" env SIZING_PHASE="$PHASE" SIZING_ROLE="$role" \
      SIZING_INCLUDED="$([ "$PHASE" = memory_only ] && echo false || echo true)" \
      TMPDIR="$L3" PATH="$SHIM_PATH" REPLAY_IMAGE_ID="$IMAGE" "$@"
}
fixture() {  # $1＝phase
  step "fixture_$1" 0 fixture docker run --rm --network none --user "$(id -u):$(id -g)" --cpus=1 \
    --memory="$MEM" --memory-swap="$MEM" -e HOME=/tmp -e PYTHONDONTWRITEBYTECODE=1 -e PYTHONPATH=/app \
    -v "$CLONE/python":/app:ro -v "$HELPER":/sizing/i074_stage2_sizing.py:ro -v "$CACHE":/cache:ro \
    -v "$CF_PATCH":/patches/counterfactual.patch:ro -v "$L1/$1":/out -w /app "$IMAGE" \
    python /sizing/i074_stage2_sizing.py fixture --phase "$1" --python-root /app --cache /cache --out /out \
    --patch /patches/counterfactual.patch
}
freeze_patches() {  # $1＝run 目錄
  mkdir -p "$1/patches"
  cp -- "$CF_PATCH" "$1/patches/counterfactual.patch"; : > "$1/patches/tooling.patch"
}
begin_phase() {  # $1＝phase
  PHASE="$1"
  mkdir -p "$L1/$PHASE"                       # ⚠️ 允許位置的根目錄先建好，再記 inventory
  helper baseline --state "$S" --phase "$PHASE" --locations "$LOC_JSON" --fs-path "$WORK" \
    --docker-root "$DROOT_C" --inventory-roots "$INV_JSON"
  helper sample --state "$S" --phase "$PHASE" --locations "$LOC_JSON" --fs-path "$WORK" &
  SAMPLER_PID=$!
  say "==> 路徑 $PHASE：baseline 已記錄，取樣中"
}
end_phase() {  # $1＝archive 路徑
  local allow
  stop_sampler
  # ⚠️ L1 只放行**本路徑自己的** run 根目錄（⛔ 不是整個 <work>/runs——那會放過改到別條路徑的 run）。
  allow="$(python3 -c 'import json,sys; print(json.dumps(sys.argv[1:]))' "$L1/$PHASE" "$L2" "$L3" "$L5")"
  helper phase-end --state "$S" --phase "$PHASE" --run-dir "$L1/$PHASE" --archive "$1" \
    --inventory-roots "$INV_JSON" --allowed "$allow" --source-roots "$SRC_JSON" \
    || on_failure "自我檢查（$PHASE）" 1
}
FIN="$CLONE/scripts/finalize-stage2-evidence.sh"

# ── 1. witness ───────────────────────────────────────────────────────────────
begin_phase witness
new_worktree "$BASE" 0 >/dev/null
fixture witness
step envcheck 0 finalizer "$FIN" --envcheck --run-dir "$L1/witness"
end_phase "$L2/envcheck"

# ── 2. success ───────────────────────────────────────────────────────────────
begin_phase success
new_worktree "$BASE" 1 >/dev/null
fixture success
freeze_patches "$L1/success"
step finalize 0 finalizer "$FIN" --finalize --run-dir "$L1/success"
end_phase "$L2/evidence"

# ── 3. failure ───────────────────────────────────────────────────────────────
mkdir -p "$L2/failed"
ls -A "$L2/failed" > "$S/failed-before.txt"
begin_phase failure
new_worktree "$BASE" 1 >/dev/null
fixture failure
freeze_patches "$L1/failure"
step publish_failed_record 1 finalizer "$FIN" --publish-failed-record --run-dir "$L1/failure"
ls -A "$L2/failed" > "$S/failed-after.txt"
PUBLISHED="$(python3 -c 'import json,sys; print(json.loads(open(sys.argv[1]).read().strip().splitlines()[-1])["published"])' \
  "$S/out/publish_failed_record.stdout")" || on_failure "讀不到 published" 1
RECORD="$(helper record-diff --before "$S/failed-before.txt" --after "$S/failed-after.txt" \
  --published "$PUBLISHED" --failed-root "$L2/failed")" || on_failure "record 目錄的集合差" 1
end_phase "$RECORD"

# ── 4. 其餘程序（只量記憶體與耗時；⛔ 不在任何 P_path 的窗口內） ───────────────────
PHASE=memory_only
say "==> memory_only"
step recover_envcheck 0 recovery "$FIN" --recover-envcheck
step recover_durability 0 recovery "$FIN" --recover-durability
step check_failed_record 2 check "$FIN" --check-failed-record --counterfactual-patch "$L1/failure/patches/counterfactual.patch"
step recover_failed_record 1 recovery "$FIN" --recover-failed-record "$RECORD"

# ── 5. metadata twin（所有量測窗口結束之後） ───────────────────────────────────
say "==> 步驟 5：metadata twin"
[ "${I074_SIZING_FAULT:-}" != twins ] || on_failure "metadata twin（注入的故障）" 1
helper twins --state "$S" --docker "$REAL_DOCKER" --run-id "$RUN_ID" --fs-path "$WORK" || on_failure "metadata twin" 1

# ── 6. 報告 ────────────────────────────────────────────────────────────────────
say "==> 步驟 6：報告"
helper report --state "$S" --json-out "$S/sizing_report.json" --text-out "$S/sizing_report.txt" \
  || on_failure "report" 1
for wt in "$L3"/tmp.*; do
  [ -d "$wt" ] && git -C "$CLONE" worktree remove --force "$wt" >/dev/null 2>&1 || true
done
# ⚠️ 本次 run id 的容器必須一個都不剩——在寫出任何報告**之前**檢查（⛔ 不留下「有報告卻失敗」的矛盾狀態）。
ensure_no_run_containers "結束前的容器檢查"
cp "$S/sizing_report.json" "$S/sizing_report.txt" "$WORK/"
cp -a "$S" "$WORK/raw" || { echo "⚠️ 複製原始量測失敗，保留 S：$S" >&2; DONE=1; exit 1; }
DONE=1
rm -rf "$S"
echo "報告：$WORK/sizing_report.json（原始量測：$WORK/raw/）" >&2
