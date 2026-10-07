#!/usr/bin/env bash
# I-074 Stage 2 ⑦d：memory／disk acceptance harness（issue.md I-074「Stage 2 步驟 ⑦d 細部計畫 v1」）。
#
#   REPLAY_IMAGE_ID=sha256:… scripts/i074-stage2-acceptance.sh --work-dir <repo 外、尚不存在的目錄> \
#       [--replay-compute stub|full] [--formal]
#
# 在 repo 外的兩層隔離複本（原始 repo → <work>/real（模擬的真正 repo）→ <work>/repo（工作複本，origin ＝ <work>/real））
# 以**真實的** runner、`evaluation.py`（launcher 只替換 `_decision_replay_rows`）、finalizer 與晉升跑 success／failure 兩條實際流程：
#   - 每一個容器程序的 cgroup 峰值、每一步 host 端的最大單一程序 RSS 都 < 450 MiB（取樣的程序群組總和只作單向警報）；
#   - success、failure 的磁碟峰值 P_path ≤ P_B_BUDGET（量法與 P_B 相同；P_B 的起訖⛔ 不變）；兩次晉升的磁碟只列資訊值。
# 結束碼：0＝報告已寫且 ok；2＝報告已寫但 threshold_exceeded；1＝harness 失敗（⛔ 不產報告）。
#
# ⚠️ ⛔ 不是正式的證據入口。正式驗收是 ⑨-1（`--formal --replay-compute full`，用 ⑨ 封存的兩份 patch）；
#   `--replay-compute full` 是 ⑨-1 唯一的量測趟（約 3 小時，⛔ 不套 counterfactual）——⑦d 的開發驗證只用 stub。
# ⚠️ 做法（細節見計畫書）：開頭的 bootstrap 解析一次 HEAD 的 commit OID、把 harness 自己的檔案凍結成 S 裡的快照、`exec`
#   快照版本；⑩ 的正式程式一律從工作複本（checkout 那個 OID）執行，報告的 repo_head／clone_head 也是它；每一步以 `env -i` ＋ supervisor 的 `clean_env()` ＋ 固定 PATH 經 `host-run` 執行，容器經 sizing 的
#   shim（profile acceptance：⛔ 不加 --read-only）；兩份 patch 的環境變數只給 replay 那一步；失敗清理沿用共用原語的六步。
#   演練用的故障注入：I074_SIZING_FAULT＝prepare（work 目錄建好之後立刻中止）／twins／copy／summary／bootstrap-*／
#   fs-noise（success 窗口期間在量測範圍外寫 2 MiB——有效性條件必須判成無效）（--formal 一律拒絕）。
#   ⚠️ full 模式（⑨-1）：量測趟之前先建 twin、以 `acceptance-precheck` 判定已量到的全部門檻與有效性——不是 ok 就⛔ 不跑量測趟
#   （額度⛔ 不消耗）；判讀一律經 `precheck-verdict`（issue.md I-074「Stage 2 量測的有效性條件與 ⑨-1 fail-fast 計畫」）。
set -euo pipefail

I074_BOOT_PROFILE=acceptance
I074_BOOT_SPREFIX=i074-accept
I074_BOOT_SELF=scripts/i074-stage2-acceptance.sh
I074_BOOT_FILES=(scripts/i074-stage2-acceptance.sh scripts/lib/i074-stage2-measure.sh scripts/lib/i074-sizing-docker-shim.sh
                 python/scripts/i074_stage2_sizing.py python/scripts/i074_stage2_replay_stub.py
                 python/scripts/i074_stage2_rss_wrapper.py python/scripts/i074_stage2_preflight.py
                 scripts/lib/i074-stage2-supervisor.py)
# >>> I074-STAGE2-BOOTSTRAP ────────────────────────────────────────────────────────────────────────────
# ⚠️ I-074 ⑦d（「Stage 2 步驟 ⑦d 細部計畫 v1」「二之三」的「harness 的快照」「bootstrap 的失敗與中斷」與「快照與來源的清單」）：
#   harness 自己的檔案先凍結成 S 裡的快照，主腳本再 `exec` 快照裡的自己——之後每一行都從快照執行（⛔ 不讀活路徑；bash 是
#   邊讀邊執行腳本，所以這一段放在最前面、盡早換掉）。
#   - snapshot pass（`SIZING_SNAPSHOT` 未設定）：**只解析一次** HEAD 的 commit OID（BOOT_HEAD）→ exclusive 建 S → 裝 bootstrap
#     trap → 驗 tmpfs／owner／路徑 → 複製「清單 ①」→ 寫 MANIFEST →（--formal：複製前驗原檔 ＝ BOOT_HEAD、複製後再驗快照
#     ＝ BOOT_HEAD）→ exec 快照裡的主腳本（BOOT_HEAD 以 SIZING_BOOT_HEAD 交給它）；
#   - re-exec pass（`SIZING_SNAPSHOT` 已設定）：先裝 bootstrap trap，再驗 realpath／S 的路徑與屬性／BOOT_HEAD 是啟動 repo 裡的
#     commit／**整個 S 的形狀**（恰好 harness/、清單 ① 的檔案與它們的目錄、MANIFEST——⛔ 沒有其他檔案、目錄、symlink 或
#     特殊檔案）／MANIFEST（每個 SHA 相符；--formal 再驗一次 ＝ BOOT_HEAD）；全部通過之後的失敗才會刪 S。
#   ⚠️ bootstrap 階段（以上兩個 pass 的驗證完成之前）⛔ 不刪任何東西：失敗或訊號只印出 S 的位置（第六輪 review 的保守方案）。
#   ⚠️ ⑦d 增補：snapshot pass 經 python3 啟動器設 subreaper 之後才 exec 快照裡的主腳本（harness 自己收養步驟留下的程序）。
#   ⚠️ ⑦d 實作第一輪 review：快照、工作複本（clone 之後 checkout 這個 OID）、報告的 repo_head 與 freeze record 全部綁
#     BOOT_HEAD——之後⛔ 不再讀會移動的 HEAD（--formal 另要求啟動 repo 的 HEAD 仍是它）。
#   ⚠️ 本段在 sizing 與 acceptance 兩個入口各一份，除了開頭的四個常數之外**逐字相同**（測試釘住）；⛔ 不 source 任何檔案。
boot_die() { echo "ERROR: $*" >&2; exit 1; }
boot_formal=0
for _boot_a in "$@"; do [ "$_boot_a" != --formal ] || boot_formal=1; done
if [ -z "${SIZING_SNAPSHOT:-}" ]; then
  BOOT_ORIGIN="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)" || boot_die "找不到啟動 harness 的 repo"
  [ "$(realpath -- "${BASH_SOURCE[0]}")" = "$BOOT_ORIGIN/$I074_BOOT_SELF" ] || boot_die "入口不在預期的位置：${BASH_SOURCE[0]}"
  BOOT_HEAD="$(git -C "$BOOT_ORIGIN" rev-parse -q --verify 'HEAD^{commit}')" && [[ "$BOOT_HEAD" =~ ^[0-9a-f]{40}$ ]] \
    || boot_die "解析不到啟動 repo 的 HEAD commit：$BOOT_ORIGIN"
  if [ "$boot_formal" = 1 ]; then
    [ -z "${I074_SIZING_FAULT:-}" ] || boot_die "--formal ⛔ 不接受 I074_SIZING_FAULT（那是演練用的故障注入）"
    for _boot_f in "${I074_BOOT_FILES[@]}"; do
      git -C "$BOOT_ORIGIN" cat-file -e "$BOOT_HEAD:$_boot_f" 2>/dev/null || boot_die "--formal：$_boot_f 尚未進版控（不在 HEAD $BOOT_HEAD 裡）"
      [ "$(git -C "$BOOT_ORIGIN" show "$BOOT_HEAD:$_boot_f" | sha256sum | cut -d' ' -f1)" = "$(sha256sum < "$BOOT_ORIGIN/$_boot_f" | cut -d' ' -f1)" ] \
        || boot_die "--formal：$_boot_f 與 HEAD 的內容不同（HEAD ＝ $BOOT_HEAD）"
    done
  fi
  BOOT_RUN_ID="$(date -u +%Y%m%dT%H%M%SZ)-$$"
  S="/dev/shm/$I074_BOOT_SPREFIX-$BOOT_RUN_ID"
  mkdir -m 700 "$S" 2>/dev/null || boot_die "建不了 S（已存在或無法建立；⛔ 不碰既有的目錄）：$S"
  trap 'echo "⚠️ bootstrap 沒有完成——⛔ 不刪任何東西，S 保留在：$S" >&2' EXIT
  trap 'exit 130' INT
  trap 'exit 143' TERM
  [ "$(stat -f -c %T "$S")" = tmpfs ] || boot_die "S 不是 tmpfs：$S"
  [ "$(stat -c %u "$S")" = "$(id -u)" ] && [ "$(realpath -- "$S")" = "$S" ] || boot_die "S 的 owner 或路徑不符：$S"
  if [ "${I074_SIZING_FAULT:-}" = bootstrap-stall ]; then sleep 60 & wait "$!"; fi
  for _boot_f in "${I074_BOOT_FILES[@]}"; do
    mkdir -p "$S/harness/$(dirname "$_boot_f")" || boot_die "建不了快照的目錄：$_boot_f"
    [ "${I074_SIZING_FAULT:-}" != bootstrap-copy ] || boot_die "（注入的故障）複製失敗：$_boot_f"
    cp -- "$BOOT_ORIGIN/$_boot_f" "$S/harness/$_boot_f" || boot_die "複製失敗：$_boot_f"
  done
  [ "${I074_SIZING_FAULT:-}" != bootstrap-manifest ] || boot_die "（注入的故障）寫不了 MANIFEST"
  (cd "$S/harness" && sha256sum -- "${I074_BOOT_FILES[@]}") > "$S/harness/MANIFEST" || boot_die "寫不了 MANIFEST"
  if [ "$boot_formal" = 1 ]; then
    for _boot_f in "${I074_BOOT_FILES[@]}"; do
      [ "$(git -C "$BOOT_ORIGIN" show "$BOOT_HEAD:$_boot_f" | sha256sum | cut -d' ' -f1)" = "$(sha256sum < "$S/harness/$_boot_f" | cut -d' ' -f1)" ] \
        || boot_die "--formal：快照的 $_boot_f 與 HEAD 不同（複製前後被改過；HEAD ＝ $BOOT_HEAD）"
    done
  fi
  find "$S/harness" -type f -exec chmod a-w {} + || boot_die "快照設不了唯讀"
  # ⚠️ ⑦d 增補（「二」④）：經 python3 啟動器設 PR_SET_CHILD_SUBREAPER 再 execv bash——harness 自己成為 subreaper（屬性跨
  #   execve 保留），步驟以任何方式結束留下的程序都由它收養；之後由 measure_subreaper_guard 以行為驗證。
  exec env SIZING_SNAPSHOT="$S" SIZING_ORIGIN_REPO="$BOOT_ORIGIN" SIZING_BOOT_RUN_ID="$BOOT_RUN_ID" SIZING_BOOT_HEAD="$BOOT_HEAD" \
    python3 -c 'import ctypes, os, sys
if ctypes.CDLL(None, use_errno=True).prctl(36, 1, 0, 0, 0) != 0:
    sys.exit("ERROR: 設不了 PR_SET_CHILD_SUBREAPER（errno %d）——⛔ 不刪任何東西，S 保留在：%s"
             % (ctypes.get_errno(), os.environ["SIZING_SNAPSHOT"]))
os.execv("/bin/bash", ["/bin/bash"] + sys.argv[1:])' "$S/harness/$I074_BOOT_SELF" "$@"
fi
S="$SIZING_SNAPSHOT"
trap 'echo "⚠️ 快照的驗證沒有完成——⛔ 不刪任何東西，S 保留在：$S" >&2' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
BOOT_RUN_ID="${SIZING_BOOT_RUN_ID:-}"
BOOT_ORIGIN="${SIZING_ORIGIN_REPO:-}"
BOOT_HEAD="${SIZING_BOOT_HEAD:-}"
[[ "$BOOT_RUN_ID" =~ ^[0-9]{8}T[0-9]{6}Z-[0-9]+$ ]] && [ "$S" = "/dev/shm/$I074_BOOT_SPREFIX-$BOOT_RUN_ID" ] \
  || boot_die "SIZING_SNAPSHOT 不是本 harness 的快照路徑：$S"
[ "$(realpath -- "${BASH_SOURCE[0]}")" = "$S/harness/$I074_BOOT_SELF" ] \
  || boot_die "主腳本不在快照裡（⛔ 不能以環境變數跳過快照）：${BASH_SOURCE[0]}"
[ "$(stat -f -c %T "$S")" = tmpfs ] && [ "$(stat -c %u "$S")" = "$(id -u)" ] && [ "$(stat -c %a "$S")" = 700 ] \
  || boot_die "S 不是 tmpfs、owner 不是自己、或 mode 不是 0700：$S"
[ -n "$BOOT_ORIGIN" ] && git -C "$BOOT_ORIGIN" rev-parse --git-dir >/dev/null 2>&1 || boot_die "SIZING_ORIGIN_REPO 不是 repo：$BOOT_ORIGIN"
[[ "$BOOT_HEAD" =~ ^[0-9a-f]{40}$ ]] \
  && [ "$(git -C "$BOOT_ORIGIN" rev-parse -q --verify "$BOOT_HEAD^{commit}" 2>/dev/null)" = "$BOOT_HEAD" ] \
  || boot_die "SIZING_BOOT_HEAD 不是啟動 repo 裡的 commit：$BOOT_HEAD"
# ⚠️ ⑦d 實作第一輪 review：取得刪除權之前，封閉驗證**整個 S** 的形狀——每一項以「型別 相對路徑」NUL 分隔、排序後比 SHA
#   （檔名含換行也⛔ 不會混淆）；find 讀不了任何目錄 → 失敗。放在讀任何檔案內容之前（FIFO 之類的特殊檔案⛔ 不會被讀到而卡住）；
#   ⛔ 不在 S 裡寫任何東西（寫了就改變了形狀）。
_boot_want="$({
  printf '%s\0' 'd harness' 'f harness/MANIFEST'
  for _boot_f in "${I074_BOOT_FILES[@]}"; do
    printf 'f harness/%s\0' "$_boot_f"
    _boot_d="$(dirname "$_boot_f")"
    while [ "$_boot_d" != . ]; do printf 'd harness/%s\0' "$_boot_d"; _boot_d="$(dirname "$_boot_d")"; done
  done; } | LC_ALL=C sort -zu | sha256sum)"
_boot_have="$(find "$S" -mindepth 1 -printf '%y %P\0' | LC_ALL=C sort -z | sha256sum)" || boot_die "列不出 S 的內容：$S"
[ "$_boot_have" = "$_boot_want" ] \
  || boot_die "S 的形狀 ≠ 快照（只能有 harness/、清單裡的檔案與它們的目錄、MANIFEST——⛔ 沒有其他檔案、目錄、symlink 或特殊檔案）：$S"
mapfile -t _boot_lines < "$S/harness/MANIFEST" || boot_die "讀不到 MANIFEST"
[ "${#_boot_lines[@]}" = "${#I074_BOOT_FILES[@]}" ] || boot_die "MANIFEST 的檔案數 ${#_boot_lines[@]} ≠ 清單的 ${#I074_BOOT_FILES[@]}"
for _boot_i in "${!I074_BOOT_FILES[@]}"; do
  _boot_f="${I074_BOOT_FILES[$_boot_i]}"
  [[ "${_boot_lines[$_boot_i]}" =~ ^([0-9a-f]{64})\ \ (.+)$ ]] && [ "${BASH_REMATCH[2]}" = "$_boot_f" ] \
    || boot_die "MANIFEST 第 $((_boot_i + 1)) 行不是 $_boot_f"
  _boot_sha="${BASH_REMATCH[1]}"
  [ "$(sha256sum < "$S/harness/$_boot_f" | cut -d' ' -f1)" = "$_boot_sha" ] || boot_die "快照的 $_boot_f 與 MANIFEST 不符"
  if [ "$boot_formal" = 1 ]; then
    [ "$(git -C "$BOOT_ORIGIN" show "$BOOT_HEAD:$_boot_f" | sha256sum | cut -d' ' -f1)" = "$_boot_sha" ] \
      || boot_die "--formal：快照的 $_boot_f 與 HEAD 不同（HEAD ＝ $BOOT_HEAD）"
  fi
done
# 驗證完成：之後到正式流程換成六步清理之前，任何失敗都刪掉 S（它已確認是本次的快照）。
trap 'rm -rf -- "$S"' EXIT
# <<< I074-STAGE2-BOOTSTRAP ────────────────────────────────────────────────────────────────────────────

REPO_ROOT="$BOOT_ORIGIN"                                    # ⚠️ 只用於 git 與 inventory（「快照與來源的清單」③）
HELPER="$S/harness/python/scripts/i074_stage2_sizing.py"
SHIM="$S/harness/scripts/lib/i074-sizing-docker-shim.sh"
export PYTHONDONTWRITEBYTECODE=1
# shellcheck source=lib/i074-stage2-measure.sh
. "$S/harness/scripts/lib/i074-stage2-measure.sh"           # ⚠️ 共用原語：從快照載入（只定義函式）

die() { echo "ERROR: $*" >&2; exit 1; }

WORK_ARG=""; FORMAL=0; COMPUTE=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --work-dir)
      [ "$#" -ge 2 ] || die "--work-dir 需要值。"
      [ -z "$WORK_ARG" ] || die "--work-dir 重複出現——⛔ 不靜默採用最後一個。"
      WORK_ARG="$2"; shift 2 ;;
    --replay-compute)
      [ "$#" -ge 2 ] || die "--replay-compute 需要值。"
      [ -z "$COMPUTE" ] || die "--replay-compute 重複出現。"
      case "$2" in stub|full) COMPUTE="$2" ;; *) die "--replay-compute 只接受 stub／full：$2" ;; esac
      shift 2 ;;
    --formal)
      [ "$FORMAL" = "0" ] || die "--formal 重複出現。"
      FORMAL=1; shift ;;
    *) die "未知參數 $1" ;;
  esac
done
[ -n "$WORK_ARG" ] || die "用法：$0 --work-dir <repo 外、尚不存在的目錄> [--replay-compute stub|full] [--formal]"
COMPUTE="${COMPUTE:-stub}"

measure_start_guards "acceptance harness"   # 設 IMAGE、REAL_DOCKER（與 ⑩ 的 label shim 互斥）
measure_work_dir_guard "$WORK_ARG"          # 設 REPO_CANON、WORK、L0_DEV

case "${I074_SIZING_FAULT:-}" in
  ""|prepare|twins|copy|summary|bootstrap-copy|bootstrap-manifest|bootstrap-stall) ;;
  probe-timeout|subreaper-stall|guard-noident|guard-badident|guard-pgleader) ;;   # ⑦d 增補
  fs-noise) ;;                                                                     # 有效性條件計畫
  *) die "I074_SIZING_FAULT 只接受 prepare／twins／copy／summary／bootstrap-*／probe-timeout／subreaper-stall／guard-*／fs-noise：${I074_SIZING_FAULT}" ;;
esac
# ⚠️ 「快照與來源的清單」②：⑩ 的正式程式從工作複本（BOOT_HEAD）執行——BOOT_HEAD 裡必須有它們。
PROD_FILES=(scripts/run-replay-offline.sh scripts/finalize-stage2-evidence.sh scripts/lib/replay-args.sh
            scripts/lib/i074-stage2-supervisor.py python/scripts/i074_stage2_preflight.py python/scripts/i074_stage2_promote.py
            python/scripts/_i074_bootstrap.py)
for f in "${PROD_FILES[@]}"; do
  git -C "$REPO_ROOT" cat-file -e "$BOOT_HEAD:$f" 2>/dev/null || die "$f 不在 HEAD $BOOT_HEAD 裡（⑩ 的正式程式由工作複本執行）"
done
if [ "$FORMAL" = "1" ]; then
  [ -z "${I074_SIZING_FAULT:-}" ] || die "--formal ⛔ 不接受 I074_SIZING_FAULT（那是演練用的故障注入）"
  [ "$COMPUTE" = full ] || die "--formal 必須帶 --replay-compute full（stub ⛔ 不涵蓋計算工作集，⛔ 不得當成 ⑨-1 的正式驗收）"
  measure_formal_fd_guard
  dirty="$(git -C "$REPO_ROOT" status --porcelain -- scripts python .gitattributes)"
  [ -z "$dirty" ] || die "--formal：scripts/、python/、.gitattributes 有未 commit 的變更：
$dirty"
  # ⚠️ 第一輪 review：上面的 clean 檢查是相對於「現在的」HEAD——它必須仍是 bootstrap 解析的那一個。
  [ "$(git -C "$REPO_ROOT" rev-parse HEAD)" = "$BOOT_HEAD" ] \
    || die "--formal：HEAD 在 bootstrap 之後移動了（bootstrap 解析的是 $BOOT_HEAD）——快照、工作複本與報告要綁同一個 commit"
fi
IDENTITY="${XDG_DATA_HOME:-$HOME/.local/share}/stock_trading/i074_stage2/run_identity.json"
[ -f "$IDENTITY" ] && [ ! -L "$IDENTITY" ] || die "找不到 Stage 2 的 run identity：$IDENTITY"

RUN_ID="$BOOT_RUN_ID"                                         # S 由 bootstrap 建好（/dev/shm/i074-accept-<run id>）
mkdir -p "$S/out" "$S/bin" "$S/phases" "$S/host"
LOG="$S/console.log"
measure_install_traps

say "==> acceptance harness：run id $RUN_ID、模式 $([ "$FORMAL" = 1 ] && echo formal || echo validation)、replay_compute $COMPUTE、S=$S"
meta run_id "$RUN_ID"; meta mode "$([ "$FORMAL" = 1 ] && echo formal || echo validation)"; meta replay_compute "$COMPUTE"
meta work "$WORK"; meta repo "$REPO_CANON"; meta image "$IMAGE"; meta state_fstype tmpfs
REPO_HEAD="$BOOT_HEAD"                                      # ⚠️ 第一輪 review：bootstrap 解析的那一個（⛔ 不再讀 HEAD）
meta repo_head "$REPO_HEAD"
meta repo_dirty "$([ -n "$(git -C "$REPO_ROOT" status --porcelain)" ] && echo true || echo false)"
IDENTITY_SHA0="$(sha_of "$IDENTITY")"
meta identity_sha256 "$IDENTITY_SHA0"

# ── 0. 準備 ────────────────────────────────────────────────────────────────────
mkdir "$WORK"
[ "${I074_SIZING_FAULT:-}" != prepare ] || on_failure "準備（注入的故障）" 1
measure_subreaper_guard                                     # ⑦d 增補：harness 自己是 subreaper（行為驗證）
# ⑦d 增補（「二」⑦）：能力檢查——clone 之前、任何量測之前。真正的 docker（⛔ 不經 shim、⛔ 不進索引）、同一個 image、
#   同一種掛載；cidfile 與名稱都在 cleanup_containers 的範圍內，docker client 記成 STEP_PID（被中斷時由六步清理收尾）。
probe_capability() {
  local pdir="$S/probe" rc=0 tmo=120 cid
  [ "${I074_SIZING_FAULT:-}" != probe-timeout ] || tmo=2
  mkdir -p "$pdir/peak" "$S/cid"
  setsid timeout -s KILL "$tmo" "$REAL_DOCKER" run --rm --network none --user "$(id -u):$(id -g)" \
    --cidfile "$S/cid/probe.cid" --name "i074sz-$RUN_ID-probe" \
    -v "$S/harness/python/scripts/i074_stage2_rss_wrapper.py:/acceptance/rss_wrapper.py:ro" -v "$pdir/peak:/peak" \
    "$IMAGE" python -I /acceptance/rss_wrapper.py python -c pass >"$pdir/stdout" 2>"$pdir/stderr" &
  STEP_PID=$!
  wait "$STEP_PID" || rc=$?
  STEP_PID=""
  [ "$rc" = 0 ] || on_failure "能力檢查：docker run 失敗或逾時（結束碼 $rc）" 1
  helper rss-capability --file "$pdir/peak/rss.json" 2>>"$pdir/stderr" \
    || on_failure "能力檢查：rss.json 不符（cgroup v1 的 total_rss、reaper＝pid1；見 $pdir/stderr）" 1
  cid="$(cat "$S/cid/probe.cid" 2>/dev/null)" || cid=""
  if [ -n "$cid" ] && "$REAL_DOCKER" inspect "$cid" >/dev/null 2>&1; then
    on_failure "能力檢查：--rm 之後容器仍在（$cid）" 1
  fi
  rm -f "$S/cid/probe.cid"
  meta rss_capability "v1:total_rss"
}
probe_capability
# 兩層複本（「三」#18）：原始 repo → <work>/real（模擬的真正 repo；晉升的目的地）→ <work>/repo（工作複本，origin ＝ <work>/real）。
# ⚠️ ⛔ 不用 --shared（④ 差異 2：會 freshen 原始 repo 的 pack）。
# ⚠️ 第一輪 review：兩層都 checkout bootstrap 解析的 OID（⑩ 的工作複本同樣是 detached 在 repo_head）——clone 期間原始 repo 的
#   HEAD 移動也⛔ 不影響本次；本機 clone 複製整個物件庫，所以這個 OID 一定在。
REAL="$WORK/real"; CLONE="$WORK/repo"
git clone -q --no-hardlinks "$REPO_ROOT" "$REAL"
git -C "$REAL" checkout -q --detach "$REPO_HEAD" || on_failure "模擬的真正 repo checkout 不了 $REPO_HEAD" 1
git clone -q --no-hardlinks "$REAL" "$CLONE"
git -C "$CLONE" checkout -q --detach "$REPO_HEAD" || on_failure "工作複本 checkout 不了 $REPO_HEAD" 1
CLONE_HEAD="$(git -C "$CLONE" rev-parse HEAD)"
meta clone_head "$CLONE_HEAD"
[ "$CLONE_HEAD" = "$REPO_HEAD" ] && [ "$(git -C "$REAL" rev-parse HEAD)" = "$REPO_HEAD" ] \
  || on_failure "工作複本的 HEAD $CLONE_HEAD（或模擬的真正 repo）≠ 記下的 repo_head $REPO_HEAD" 1
L1="$WORK/runs"; L2="$CLONE/python/baselines/i074_stage2"; L3="$WORK/tmp"; L5="$CLONE/.git"
L6="$REAL/python/baselines/i074_stage2"
mkdir -p "$L1" "$L3" "$WORK/aside"
meta l0_dev "$L0_DEV"
meta repo_dev "$(stat -c %d "$REPO_CANON")"
DROOT="$("$REAL_DOCKER" info --format '{{.DockerRootDir}}')"
DROOT_C="$(realpath -- "$DROOT")"
meta docker_root "$DROOT_C"; meta docker_root_dev "$(stat -c %d "$DROOT_C")"
[ "$(stat -c %d "$DROOT_C")" = "$L0_DEV" ] || on_failure "L4：Docker Root Dir 不在同一個檔案系統" 1
meta logging_default "$("$REAL_DOCKER" info --format '{{.LoggingDriver}}')"
meta runner_sha256 "$(sha_of "$CLONE/scripts/run-replay-offline.sh")"
meta finalizer_sha256 "$(sha_of "$CLONE/scripts/finalize-stage2-evidence.sh")"
meta promote_sha256 "$(sha_of "$CLONE/python/scripts/i074_stage2_promote.py")"

helper acceptance-anchors --python-root "$CLONE/python" > "$S/anchors.json" || on_failure "取不到 Stage 1 的信任錨" 1
read -r BUNDLE_ID BASE < <(python3 -c 'import json,sys; a=json.load(open(sys.argv[1])); print(a["bundle_id"], a["after_base_commit"])' "$S/anchors.json")
meta bundle_id "$BUNDLE_ID"; meta base_commit "$BASE"
CF_PATCH="$L2/counterfactual_e1cbbbd.patch"
TOOL_PATCH="$L2/tooling_e1cbbbd.patch"
[ -s "$CF_PATCH" ] && [ -s "$TOOL_PATCH" ] || on_failure "兩份 patch 不存在或是空的（⑩ 的 tooling ⛔ 不得為空）" 1
meta counterfactual_sha256 "$(sha_of "$CF_PATCH")"; meta tooling_sha256 "$(sha_of "$TOOL_PATCH")"
# shellcheck source=lib/replay-args.sh
. "$CLONE/scripts/lib/replay-args.sh"                       # ⚠️ 正式程式：從工作複本
new_worktree() {  # $1＝ref；$2＝patched（0／1）→ 印出 worktree 路徑（在 L3）
  local wt
  wt="$(mktemp -d "$L3/tmp.XXXXXXXXXX")" && rmdir "$wt" || return 1
  replay_args_prepare_worktree "$CLONE" "$1" "$wt" >/dev/null || return 1
  if [ "$2" = 1 ]; then
    [ "$(replay_args_compose "$wt" "$1" "$CF_PATCH" "$TOOL_PATCH")" = "$COMPOSE" ] \
      || { echo "ERROR: 合成結果與第一次不同（或合成失敗）" >&2; return 1; }
  fi
  printf '%s\n' "$wt"
}
cwt="$(mktemp -d "$L3/tmp.XXXXXXXXXX")"; rmdir "$cwt"
replay_args_prepare_worktree "$CLONE" "$BASE" "$cwt" >/dev/null || on_failure "合成用的 worktree" 1
COMPOSE="$(replay_args_compose "$cwt" "$BASE" "$CF_PATCH" "$TOOL_PATCH")" || on_failure "兩份 patch 的合成失敗" 1
git -C "$CLONE" worktree remove --force "$cwt" || on_failure "移除合成用的 worktree" 1
read -r _T1 _T2 C_CF C_TOOL C_COMP C_SEM <<< "$COMPOSE"
[ "$C_CF" = "$(sha_of "$CF_PATCH")" ] || on_failure "counterfactual 的 raw SHA ≠ 增量 canonical SHA" 1
[ "$C_TOOL" = "$(sha_of "$TOOL_PATCH")" ] || on_failure "tooling 的 raw SHA ≠ 增量 canonical SHA" 1
meta counterfactual_canonical_sha256 "$C_CF"; meta tooling_canonical_sha256 "$C_TOOL"
meta composed_sha256 "$C_COMP"; meta counterfactual_semantic_sha256 "$C_SEM"
say "==> 步驟 0：實建 worktree、快照、runner 的凍結副本與 probe 結構量 allocated bytes"
for spec in head:HEAD:0 base_patched:$BASE:1; do
  IFS=: read -r kind ref patched <<< "$spec"
  g0="$(alloc "$L5")"
  wt="$(new_worktree "$ref" "$patched")"
  comp "wt_$kind" "$(alloc "$wt")"
  comp "git_$kind" "$(( $(alloc "$L5") - g0 ))"
  comp "index_$kind" "$(alloc "$L5/worktrees/$(basename "$wt")/index")"
  git -C "$CLONE" worktree remove --force "$wt"
done
snap="$(mktemp -d "$L3/tmp.XXXXXXXXXX")"
cp -- "$CF_PATCH" "$snap/counterfactual.patch"; cp -- "$TOOL_PATCH" "$snap/tooling.patch"
comp snapshot "$(alloc "$snap")"; rm -rf "$snap"
frz="$(mktemp -d "$L3/tmp.XXXXXXXXXX")"
cp -- "$CF_PATCH" "$frz/counterfactual.patch"; cp -- "$TOOL_PATCH" "$frz/tooling.patch"
comp frozen_patched "$(alloc "$frz")"; rm -rf "$frz"
probe="$(mktemp -d "$L2/.sizing-probe.XXXXXXXX")"
mkdir "$probe/a" "$probe/b-src" "$probe/b-dst"
printf a > "$probe/a/marker"; printf src > "$probe/b-src/marker"; printf dst > "$probe/b-dst/marker"
comp probe "$(( $(alloc "$probe/a") + $(alloc "$probe/b-src") + $(alloc "$probe/b-dst") ))"
rm -rf "$probe"
NOCF=""
if [ "$COMPUTE" = full ]; then
  # ⚠️ 量測趟的程式碼（「二之二」）：e1cbbbd ＋ tooling、⛔ 不套 counterfactual；在任何 baseline 之前建好，⛔ 不計入 P_path。
  NOCF="$WORK/nocf"
  replay_args_prepare_worktree "$CLONE" "$BASE" "$NOCF" >/dev/null || on_failure "量測趟的 worktree" 1
  line="$(replay_args_compose "$NOCF" "$BASE" "" "$TOOL_PATCH")" || on_failure "量測趟的合成（⛔ 不套 counterfactual）" 1
  read -r _ _ _ nocf_tool _ _ <<< "$line"
  [ "$nocf_tool" = "$C_TOOL" ] || on_failure "量測趟的 tooling 增量 SHA ≠ 正式合成的" 1
fi
helper clean-env --clone "$CLONE" > "$S/clean.env" || on_failure "clean-env" 1
mapfile -d '' CLEAN < "$S/clean.env"

# shim（profile acceptance）
ln -s "$SHIM" "$S/bin/docker"
LOC_JSON="$(python3 -c 'import json,sys; print(json.dumps(dict(zip(["L1","L2","L3","L5"], sys.argv[1:]))))' "$L1" "$L2" "$L3" "$L5")"
PROMO_LOC_JSON="$(python3 -c 'import json,sys; print(json.dumps(dict(zip(["L3","L5","L6"], sys.argv[1:]))))' "$L3" "$L5" "$L6")"
INV_JSON="$(python3 -c 'import json,sys; print(json.dumps(sys.argv[1:]))' "$WORK" "$REPO_CANON")"
SRC_JSON="$(python3 -c 'import json,sys; print(json.dumps(sys.argv[1:]))' "$CLONE" "$REAL")"

# 執行一個步驟並比對結束碼。$1＝步驟名、$2＝預期結束碼、$3＝role、其餘＝指令。⚠️ `env -i` ＋ `clean_env()` ＋ 固定 PATH，
# 經 `host-run`（host 端程序樹的記憶體）；容器經 shim。
step() {
  local name="$1" want="$2" role="$3" included=false
  shift 3
  case "$PHASE" in success|failure|promote_success|promote_failure) included=true ;; esac
  say "    $name（預期 $want）"
  run_in_group "$name" "$want" env -i "${CLEAN[@]}" PATH="$S/bin:/usr/bin:/bin" PYTHONDONTWRITEBYTECODE=1 TMPDIR="$L3" \
      REPLAY_IMAGE_ID="$IMAGE" SIZING_REAL_DOCKER="$REAL_DOCKER" SIZING_STATE="$S" SIZING_RUN_ID="$RUN_ID" \
      SIZING_IMAGE="$IMAGE" SIZING_HELPER="$HELPER" SIZING_PROFILE=acceptance SIZING_PHASE="$PHASE" SIZING_ROLE="$role" \
      SIZING_INCLUDED="$included" python3 "$HELPER" host-run --state "$S" --step "$name" -- "$@"
  # ⑦d 增補（「二」⑦）：逐步檢查——這一步的 host 紀錄與目前為止的每一份 sidecar；不符就立刻中止（⛔ 不拖到最後的報告）。
  helper check-step --state "$S" --step "$name" 2>>"$S/check-step.err" \
    || { tail -5 "$S/check-step.err" >&2; on_failure "量測檢查（$name）" 1; }
}
# replay：⚠️ 兩份 patch 的環境變數**只**在這一步（總綱「四」）。$1＝步驟名、$2＝預期、$3＝run 目錄、$4＝模式、$5＝計算
replay() {
  local name="$1" want="$2" run="$3" mode="$4" compute="$5" argv
  helper replay-argv --clone "$CLONE" --run-dir "$run" --anchors "$S/anchors.json" > "$S/argv.$name" \
    || on_failure "replay-argv（$name）" 1
  mapfile -d '' argv < "$S/argv.$name"
  # ⚠️ replay_argv() 的第一個 token 就是 runner 本身（⑩ 照原樣執行）——⛔ 不再另外加。
  [ "${#argv[@]}" -gt 1 ] && [ "${argv[0]}" = "$CLONE/scripts/run-replay-offline.sh" ] \
    || on_failure "replay-argv（$name）的第一個 token 不是工作複本的 runner" 1
  step "$name" "$want" replay env I074_STAGE=2 COUNTERFACTUAL_PATCH="$L1/${PATCH_RUN:-$(basename "$run")}/patches/counterfactual.patch" \
    TOOLING_PATCH="$L1/${PATCH_RUN:-$(basename "$run")}/patches/tooling.patch" SIZING_REPLAY_MODE="$mode" \
    SIZING_REPLAY_COMPUTE="$compute" ${NOCF:+SIZING_NOCF_PYTHON="$NOCF/python"} "${argv[@]}"
}
promote() {  # $1＝步驟名、$2＝預期、$3＝replay 的結束碼（0／6）
  step "$1" "$2" promotion python3 "$HELPER" promote-measure --clone "$CLONE" --work "$WORK" --real "$REAL" \
    --identity "$IDENTITY" --bundle-id "$BUNDLE_ID" --semantic "$C_SEM" --repo-head "$REPO_HEAD" --rc "$3"
}
freeze_patches() {  # $1＝run 目錄（⑩ 的 layout：<run>/patches/{counterfactual,tooling}.patch）
  mkdir -p "$1/patches"
  cp -- "$CF_PATCH" "$1/patches/counterfactual.patch"; cp -- "$TOOL_PATCH" "$1/patches/tooling.patch"
}
begin_disk() { mkdir -p "$L1/$1"; measure_begin_phase "$1" "$LOC_JSON" "$WORK" "$DROOT_C" "$INV_JSON"; }
end_disk() {  # $1＝archive
  local allow
  allow="$(python3 -c 'import json,sys; print(json.dumps(sys.argv[1:]))' "$L1/$PHASE" "$L2" "$L3" "$L5")"
  measure_end_phase "$L1/$PHASE" "$1" "$INV_JSON" "$allow" "$SRC_JSON"
}
begin_promo() { measure_begin_phase "$1" "$PROMO_LOC_JSON" "$WORK" "$DROOT_C" "$INV_JSON"; }
end_promo() {  # $1＝晉升的目的地
  local allow
  allow="$(python3 -c 'import json,sys; print(json.dumps(sys.argv[1:]))' "$L3" "$L5" "$L6")"
  measure_end_phase "$L6" "$1" "$INV_JSON" "$allow" "$SRC_JSON"
}
FIN="$CLONE/scripts/finalize-stage2-evidence.sh"

# ── 1. preflight（只量記憶體） ──────────────────────────────────────────────────
PHASE=preflight
say "==> preflight"
step check_failed_record_preflight 0 check "$FIN" --check-failed-record --counterfactual-patch "$CF_PATCH"

# ── 2. success ───────────────────────────────────────────────────────────────
begin_disk success
freeze_patches "$L1/success"
replay replay_success 0 "$L1/success" success stub
step finalize 0 finalizer "$FIN" --finalize --run-dir "$L1/success"
end_disk "$L2/evidence"

# ── 3. 晉升 success（資訊值的窗口） ──────────────────────────────────────────────
begin_promo promote_success
promote promote_success 0 0
end_promo "$L6/evidence"

# ── 4. failure ───────────────────────────────────────────────────────────────
mkdir -p "$L2/failed"
ls -A "$L2/failed" > "$S/failed-before.txt"
begin_disk failure
freeze_patches "$L1/failure"
replay replay_failure 6 "$L1/failure" failure stub
step publish_failed_record 1 finalizer "$FIN" --publish-failed-record --run-dir "$L1/failure"
ls -A "$L2/failed" > "$S/failed-after.txt"
PUBLISHED="$(python3 -c 'import json,sys; print(json.loads(open(sys.argv[1]).read().strip().splitlines()[-1])["published"])' \
  "$S/out/publish_failed_record.stdout")" || on_failure "讀不到 published" 1
RECORD="$(helper record-diff --before "$S/failed-before.txt" --after "$S/failed-after.txt" \
  --published "$PUBLISHED" --failed-root "$L2/failed")" || on_failure "record 目錄的集合差" 1
end_disk "$RECORD"

# ── 5～6. 晉升 failure：晉升的第 3 步要求終態恰好一組，evidence/ 暫時搬開（同一個檔案系統的 rename） ────────────
PHASE=""
mv -- "$L2/evidence" "$WORK/aside/evidence" || on_failure "evidence/ 搬不開" 1
begin_promo promote_failure
promote promote_failure 6 6
end_promo "$L6/failed/$(basename "$RECORD")"
PHASE=""
mv -- "$WORK/aside/evidence" "$L2/evidence" || on_failure "evidence/ 搬不回來" 1

# ── 7. 其餘程序（只量記憶體） ───────────────────────────────────────────────────
PHASE=memory_only
say "==> memory_only"
step check_failed_record 2 check "$FIN" --check-failed-record --counterfactual-patch "$L1/failure/patches/counterfactual.patch"
step recover_durability 0 recovery "$FIN" --recover-durability
step recover_failed_record 1 recovery "$FIN" --recover-failed-record "$RECORD"
# i（環境等價比對程序）：⚠️ 封存的 envcheck/ 是 ③c 當時的 HEAD 發布的，--recover-envcheck 要求執行身分 ＝ 封存的
#   finalizer_provenance.base_commit——所以先把它搬開，以封存的見證輸出（after'／cohort'；gzip 解壓即原本的 canonical bytes）
#   重新發布一份（--envcheck：E3a 全量比對 ＋ 發布，就是 ③c 執行過的程式），再量 --recover-envcheck，最後把原本的 envcheck/ 搬回來。
WIT="$L1/witness"
mkdir -p "$WIT/witness"
for f in after_artifact cohort_manifest; do
  gzip -dc "$L2/envcheck/witness/$f.json.gz" > "$WIT/witness/$f.json" || on_failure "解壓封存的 $f" 1
done
mv -- "$L2/envcheck" "$WORK/aside/envcheck" || on_failure "envcheck/ 搬不開" 1
step envcheck 0 finalizer "$FIN" --envcheck --run-dir "$WIT"
step recover_envcheck 0 recovery "$FIN" --recover-envcheck
rm -rf -- "$L2/envcheck" && mv -- "$WORK/aside/envcheck" "$L2/envcheck" || on_failure "envcheck/ 搬不回來" 1
if [ "$COMPUTE" = full ]; then
  # ⚠️ 有效性條件計畫「二」的「⑨-1 的 fail-fast」：所有磁碟窗口都已結束（量測趟本身⛔ 沒有磁碟窗口），先建 twin、
  #   再以 precheck 判定已量到的全部門檻與有效性——⛔ 不是 ok 就⛔ 不跑量測趟（額度⛔ 不消耗）。precheck.json 隨 S 保存；
  #   判讀一律經 `precheck-verdict --raw <dir> --repo <受信任的 repo> --expected-repo-head <開跑前記下的 HEAD>`。
  say "==> metadata twin（量測趟之前的 invocation）"
  [ "${I074_SIZING_FAULT:-}" != twins ] || on_failure "metadata twin（注入的故障）" 1
  helper twins --state "$S" --docker "$REAL_DOCKER" --run-id "$RUN_ID" --fs-path "$WORK" || on_failure "metadata twin" 1
  say "==> precheck（量測趟之前的判定）"
  PRE_RC=0
  helper acceptance-precheck --state "$S" --clone "$CLONE" || PRE_RC=$?
  case "$PRE_RC" in
    0) ;;
    2) on_failure "precheck：threshold_exceeded（確定的違反，⛔ 不得重跑）——量測趟未執行、額度未消耗；判讀見 precheck-verdict" 1 ;;
    3) on_failure "precheck：invalid（量測無效，可以重跑）——量測趟未執行、額度未消耗；判讀見 precheck-verdict" 1 ;;
    *) on_failure "precheck 失敗（rc=$PRE_RC）——量測趟未執行、額度未消耗" 1 ;;
  esac
  say "==> ⑨-1 的量測趟：完整計算（⛔ 不套 counterfactual；約 3 小時）"
  mkdir -p "$L1/full"
  PATCH_RUN=success replay replay_full 0 "$L1/full" success full
fi

# ── 8. metadata twin（所有量測窗口結束之後；full 模式只補量測趟那一個——run_twins 跳過已有的） ─────────────────
PHASE=""
say "==> metadata twin"
[ "${I074_SIZING_FAULT:-}" != twins ] || on_failure "metadata twin（注入的故障）" 1
helper twins --state "$S" --docker "$REAL_DOCKER" --run-id "$RUN_ID" --fs-path "$WORK" || on_failure "metadata twin" 1

# ── 9. 報告 ────────────────────────────────────────────────────────────────────
say "==> 報告"
REPORT_RC=0
helper acceptance-report --state "$S" --clone "$CLONE" --json-out "$S/acceptance_report.json" \
  --text-out "$S/acceptance_report.txt" || REPORT_RC=$?
case "$REPORT_RC" in 0|2) ;; *) on_failure "report" 1 ;; esac
for wt in "$L3"/tmp.* ${NOCF:+"$NOCF"}; do
  [ -d "$wt" ] && git -C "$CLONE" worktree remove --force "$wt" >/dev/null 2>&1 || true
done
ensure_no_run_containers "結束前的容器檢查"
measure_check_adopted "結束前的收養檢查"                       # ⑦d 增補
[ "$(sha_of "$IDENTITY")" = "$IDENTITY_SHA0" ] || on_failure "Stage 2 identity 檔在執行期間被改了" 1
cp "$S/acceptance_report.json" "$S/acceptance_report.txt" "$WORK/"
cp -a "$S" "$WORK/raw" || { echo "⚠️ 複製原始量測失敗，保留 S：$S" >&2; DONE=1; exit 1; }
DONE=1
rm -rf "$S"
echo "報告：$WORK/acceptance_report.json（原始量測：$WORK/raw/；status：$([ "$REPORT_RC" = 0 ] && echo ok || echo threshold_exceeded)）" >&2
exit "$REPORT_RC"
