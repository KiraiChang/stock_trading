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
#   orphan-bad（leader 以預期／非預期的結束碼退出、背景子程序仍留在 group 裡——驗證一律收尾並中止），或 fs-noise
#   （success 窗口期間在量測範圍外寫 2 MiB——有效性條件必須把量測判成無效；issue.md I-074「Stage 2 量測的有效性條件與
#   ⑨-1 fail-fast 計畫」）。
#
# 模式：預設是 ④ 的**可用性驗證**（允許未 commit，報告記錄實際執行的腳本 SHA-256）；
#       `--formal` 是 ⑤ 的**正式量測**（scripts/、python/、.gitattributes 必須 clean，harness 檔案必須等於 HEAD）。
#
# ⚠️ I-074 ⑦b（「Stage 2 步驟 ⑦b 細部計畫 v1」「二之九」）：
#   - 一律用**真實的兩份 patch**（複本常數路徑的 counterfactual ＋ tooling），patched worktree 以唯一的合成函式
#     `replay_args_compose()` 建立，並斷言兩份的 raw SHA 各自 ＝ 增量 canonical SHA；
#   - `--formal` 且報告 `status = ok`、`P_B ≤ P_B_BUDGET` 時寫出 `<work>/freeze_record.json`（v29「八之二」）——
#     validation 模式、`assumption_violated`、`P_B` 超過預算、任何失敗路徑都⛔ 不寫；
#   - 與 ⑩ 的 label shim **互斥**：環境帶 `I074_STAGE2_*`、或 PATH 上的 docker 是 label shim → 中止。
#
# ⚠️ I-074 ⑦d（「Stage 2 步驟 ⑦d 細部計畫 v1」）：
#   - 開頭的 bootstrap 先把 harness 自己的檔案（「快照與來源的清單」①）凍結成 S 裡的快照、再 `exec` 快照版本；共用原語
#     （scripts/lib/i074-stage2-measure.sh）、shim、helper、mem-guard 一律從快照載入；
#   - ⑩ 的正式程式從工作複本（HEAD）執行——freeze record 的 `build`／`check-pair` 也是（`--repo <工作複本>`）；
#   - 啟動 harness 的 repo（SIZING_ORIGIN_REPO）只用於 git clone 的來源、--formal 的檢查與 inventory 自我檢查；
#   - 第一輪 review：bootstrap 只解析一次 HEAD 的 commit OID（BOOT_HEAD）——快照、工作複本（clone 後 checkout 它）、報告的
#     repo_head 與 freeze record 都綁它，報告驗 clone_head ＝ repo_head；--formal 另要求啟動 repo 的 HEAD 仍是它；
#   - accounted 多一項 runner_frozen_patches（runner 以 exec docker run 結束，凍結副本留在 L3）；報告綁住完整的 harness_manifest。
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

I074_BOOT_PROFILE=sizing
I074_BOOT_SPREFIX=i074-sizing
I074_BOOT_SELF=scripts/i074-stage2-sizing.sh
I074_BOOT_FILES=(scripts/i074-stage2-sizing.sh scripts/lib/i074-stage2-measure.sh scripts/lib/i074-sizing-docker-shim.sh
                 scripts/lib/mem-guard.sh python/scripts/i074_stage2_sizing.py python/scripts/i074_stage2_preflight.py)
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
. "$S/harness/scripts/lib/mem-guard.sh"
# shellcheck source=lib/i074-stage2-measure.sh
. "$S/harness/scripts/lib/i074-stage2-measure.sh"           # ⚠️ 共用原語：從快照載入（只定義函式）

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

measure_start_guards "sizing harness"   # 設 IMAGE、REAL_DOCKER（與 ⑩ 的 label shim 互斥）
measure_work_dir_guard "$WORK_ARG"      # 設 REPO_CANON、WORK、L0_DEV

case "${I074_SIZING_FAULT:-}" in
  ""|twins|copy|summary|cleanup|stuck|group-alive|final-check|orphan-ok|orphan-bad) ;;
  bootstrap-copy|bootstrap-manifest|bootstrap-stall) ;;   # ⑦d：只在 snapshot pass 生效
  orphan-setsid|subreaper-stall|guard-noident|guard-badident|guard-pgleader) ;;   # ⑦d 增補
  fs-noise) ;;                                                                     # 有效性條件計畫
  *) die "I074_SIZING_FAULT 只接受 twins／copy／summary／cleanup／stuck／group-alive／final-check／orphan-ok／orphan-bad／orphan-setsid／bootstrap-*／subreaper-stall／guard-*／fs-noise：${I074_SIZING_FAULT}" ;;
esac
if [ "$FORMAL" = "1" ]; then
  [ -z "${I074_SIZING_FAULT:-}" ] || die "--formal ⛔ 不接受 I074_SIZING_FAULT（那是演練用的故障注入）"
  measure_formal_fd_guard
  dirty="$(git -C "$REPO_ROOT" status --porcelain -- scripts python .gitattributes)"
  [ -z "$dirty" ] || die "--formal：scripts/、python/、.gitattributes 有未 commit 的變更：
$dirty"
  # ⚠️ 第一輪 review：上面的 clean 檢查是相對於「現在的」HEAD——它必須仍是 bootstrap 解析的那一個。
  [ "$(git -C "$REPO_ROOT" rev-parse HEAD)" = "$BOOT_HEAD" ] \
    || die "--formal：HEAD 在 bootstrap 之後移動了（bootstrap 解析的是 $BOOT_HEAD）——快照、工作複本與報告要綁同一個 commit"
  # ⚠️ ⑦d：harness 自己的檔案（清單 ①）已由 bootstrap 在快照前後驗過 ＝ BOOT_HEAD；freeze record 的寫入端與它 import 的常數
  #   改從工作複本（BOOT_HEAD）執行（清單 ②），所以這裡驗它們在 BOOT_HEAD 裡（⛔ 不讀原始 repo 的工作樹版本）。
  for f in python/scripts/i074_stage2_freeze_record.py python/scripts/i074_stage2_preflight.py; do
    git -C "$REPO_ROOT" cat-file -e "$BOOT_HEAD:$f" 2>/dev/null || die "--formal：$f 尚未進版控（freeze record 由工作複本執行）"
  done
fi

RUN_ID="$BOOT_RUN_ID"                                         # S 由 bootstrap 建好（/dev/shm/i074-sizing-<run id>）
mkdir -p "$S/out" "$S/bin" "$S/phases"
LOG="$S/console.log"
measure_install_traps

say "==> sizing harness：run id $RUN_ID、模式 $([ "$FORMAL" = 1 ] && echo formal || echo validation)、S=$S"
meta run_id "$RUN_ID"; meta mode "$([ "$FORMAL" = 1 ] && echo formal || echo validation)"
meta work "$WORK"; meta repo "$REPO_CANON"; meta image "$IMAGE"; meta state_fstype tmpfs
meta repo_head "$BOOT_HEAD"                                  # ⚠️ 第一輪 review：bootstrap 解析的那一個（⛔ 不再讀 HEAD）
meta repo_dirty "$([ -n "$(git -C "$REPO_ROOT" status --porcelain)" ] && echo true || echo false)"
meta harness_sha256 "$(sha_of "$S/harness/scripts/i074-stage2-sizing.sh")"   # ⑦d：快照裡的（＝ 實際執行的）
meta shim_sha256 "$(sha_of "$SHIM")"
meta helper_sha256 "$(sha_of "$HELPER")"
IDENTITY="${XDG_DATA_HOME:-$HOME/.local/share}/stock_trading/i074_stage2/run_identity.json"
if [ -f "$IDENTITY" ]; then
  meta identity_sha256 "$(sha_of "$IDENTITY")"
elif [ "$FORMAL" = "1" ]; then
  on_failure "--formal 需要 Stage 2 的 run identity（freeze record 要綁它）：$IDENTITY" 1
fi

# ── 0. 準備 ────────────────────────────────────────────────────────────────────
mkdir "$WORK"
measure_subreaper_guard                 # ⑦d 增補：harness 自己是 subreaper（行為驗證）
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
  orphan-setsid)
    # ⑦d 增補：步驟留下一個逃出 group 的程序（setsid）——group 的檢查看不到它，收養的檢查必須擋下並清掉
    run_in_group fault_orphan_setsid 0 sh -c 'setsid sleep 60 </dev/null >/dev/null 2>&1 & echo "$!" > "$0/orphan.pid"; exit 0' "$S"
    DONE=1; rm -rf "$S"; exit 0 ;;   # ⚠️ 走到這裡＝收養的程序⛔ 沒有被擋下
esac
# ⚠️ **⛔ 不用 `--shared`**（差異 2，2026-09-24 第一次實跑被自我檢查擋下）：`--shared` 以 alternates 共用真正 repo 的
#   物件，而 `git write-tree`／`git apply --index` 寫到**已存在**的物件時會 freshen（touch）含有它的 pack——
#   改到的是真正 repo 的 pack 的 mtime。`--no-hardlinks` 完整複製物件（⛔ 也不共用 inode），git 的寫入全落在 L5。
# ⚠️ 第一輪 review：工作複本 checkout bootstrap 解析的 OID（⑩ 的工作複本同樣是 detached 在 repo_head）——clone 期間原始 repo
#   的 HEAD 移動也⛔ 不影響本次；本機 clone 複製整個物件庫，所以這個 OID 一定在。報告與 freeze record 都驗 clone_head ＝ repo_head。
git clone -q --no-hardlinks "$REPO_ROOT" "$WORK/repo"
CLONE="$WORK/repo"
git -C "$CLONE" checkout -q --detach "$BOOT_HEAD" || on_failure "工作複本 checkout 不了 $BOOT_HEAD" 1
L1="$WORK/runs"; L2="$CLONE/python/baselines/i074_stage2"; L3="$WORK/tmp"; L5="$CLONE/.git"
CACHE="$WORK/cache"
mkdir -p "$L1" "$L3" "$CACHE"
CLONE_HEAD="$(git -C "$CLONE" rev-parse HEAD)"
meta clone_head "$CLONE_HEAD"
[ "$CLONE_HEAD" = "$BOOT_HEAD" ] || on_failure "工作複本的 HEAD $CLONE_HEAD ≠ 記下的 repo_head $BOOT_HEAD" 1
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
TOOL_PATCH="$L2/tooling_e1cbbbd.patch"          # ⚠️ ⑦b：真實的 tooling（⑩ 的 tooling ⛔ 不得為空）
[ -s "$TOOL_PATCH" ] || on_failure "tooling patch 不存在或是空的：$TOOL_PATCH" 1
meta counterfactual_sha256 "$(sha_of "$CF_PATCH")"
meta tooling_sha256 "$(sha_of "$TOOL_PATCH")"
. "$CLONE/scripts/lib/replay-args.sh"

new_worktree() {  # $1＝ref；$2＝patched（0／1）→ 印出 worktree 路徑（在 L3）
  local wt
  wt="$(mktemp -d "$L3/tmp.XXXXXXXXXX")" && rmdir "$wt" || return 1
  replay_args_prepare_worktree "$CLONE" "$1" "$wt" >/dev/null || return 1
  if [ "$2" = 1 ]; then
    # ⚠️ ⑦b：與 runner 同一個合成函式（counterfactual → tooling）；結果必須 ＝ 步驟 0 之前記下的那一次。
    [ "$(replay_args_compose "$wt" "$1" "$CF_PATCH" "$TOOL_PATCH")" = "$COMPOSE" ] \
      || { echo "ERROR: 合成結果與第一次不同（或合成失敗）" >&2; return 1; }
  fi
  printf '%s\n' "$wt"
}
# ⚠️ ⑦b：先以唯一的合成函式算一次兩份**真實** patch 的增量 canonical SHA（量測窗口之外），並斷言 raw ＝ canonical。
cwt="$(mktemp -d "$L3/tmp.XXXXXXXXXX")"; rmdir "$cwt"
replay_args_prepare_worktree "$CLONE" "$BASE" "$cwt" >/dev/null || on_failure "合成用的 worktree" 1
COMPOSE="$(replay_args_compose "$cwt" "$BASE" "$CF_PATCH" "$TOOL_PATCH")" || on_failure "兩份 patch 的合成失敗" 1
git -C "$CLONE" worktree remove --force "$cwt" || on_failure "移除合成用的 worktree" 1
read -r _T1 _T2 C_CF C_TOOL C_COMP C_SEM <<< "$COMPOSE"
[ "$C_CF" = "$(sha_of "$CF_PATCH")" ] || on_failure "counterfactual 的 raw SHA ≠ 增量 canonical SHA" 1
[ "$C_TOOL" = "$(sha_of "$TOOL_PATCH")" ] || on_failure "tooling 的 raw SHA ≠ 增量 canonical SHA" 1
meta counterfactual_canonical_sha256 "$C_CF"; meta tooling_canonical_sha256 "$C_TOOL"
meta composed_sha256 "$C_COMP"; meta counterfactual_semantic_sha256 "$C_SEM"
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
cp -- "$CF_PATCH" "$snap/counterfactual.patch"; cp -- "$TOOL_PATCH" "$snap/tooling.patch"
comp snapshot "$(alloc "$snap")"; rm -rf "$snap"
# ⚠️ ⑦d（「三」#15）：runner 把兩份 patch 凍結在私有的 mktemp -d，但它以 exec docker run 結束、EXIT trap ⛔ 不執行——
#   凍結副本留在 L3。以同形狀的目錄量：witness（③c 的見證趟：⛔ 沒有 counterfactual、tooling 是 0-byte）與兩份 patch。
frz="$(mktemp -d "$L3/tmp.XXXXXXXXXX")"
: > "$frz/tooling.patch"
comp frozen_witness "$(alloc "$frz")"; rm -rf "$frz"
frz="$(mktemp -d "$L3/tmp.XXXXXXXXXX")"
cp -- "$CF_PATCH" "$frz/counterfactual.patch"; cp -- "$TOOL_PATCH" "$frz/tooling.patch"
comp frozen_patched "$(alloc "$frz")"; rm -rf "$frz"
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
    -v "$L1/$1":/out -w /app "$IMAGE" \
    python /sizing/i074_stage2_sizing.py fixture --phase "$1" --python-root /app --cache /cache --out /out \
    --composed-sha256 "$C_COMP"
}
freeze_patches() {  # $1＝run 目錄
  mkdir -p "$1/patches"
  cp -- "$CF_PATCH" "$1/patches/counterfactual.patch"; cp -- "$TOOL_PATCH" "$1/patches/tooling.patch"
}
begin_phase() {  # $1＝phase
  mkdir -p "$L1/$1"                           # ⚠️ 允許位置的根目錄先建好，再記 inventory
  measure_begin_phase "$1" "$LOC_JSON" "$WORK" "$DROOT_C" "$INV_JSON"
}
end_phase() {  # $1＝archive 路徑
  local allow
  # ⚠️ L1 只放行**本路徑自己的** run 根目錄（⛔ 不是整個 <work>/runs——那會放過改到別條路徑的 run）。
  allow="$(python3 -c 'import json,sys; print(json.dumps(sys.argv[1:]))' "$L1/$PHASE" "$L2" "$L3" "$L5")"
  measure_end_phase "$L1/$PHASE" "$1" "$INV_JSON" "$allow" "$SRC_JSON"
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
# ⚠️ 有效性條件計畫「二」：P_B_BUDGET 取自 S 的快照（並斷言與工作複本逐位元相同）；沒有確定的超標、但有有效性問題
#   → 量測無效（rc＝1，⛔ 不是判定，可以重跑）。
helper report --state "$S" --clone "$CLONE" --json-out "$S/sizing_report.json" --text-out "$S/sizing_report.txt" \
  || on_failure "report" 1
# ⚠️ ⑦b：freeze record 在**任何檔案複製到 <work> 之前**於 S 建好並自我驗證；只有 formal ＋ ok ＋ P_B ≤ 預算才寫。
# ⚠️ ⑦d：freeze record 的寫入端從**工作複本**（HEAD）執行（「快照與來源的清單」②；⛔ 不從原始 repo 的活路徑）——它只以
#   git 物件讀 repo_head 中的檔案內容，工作複本有同一份物件；它 import 的 i074_stage2_preflight.py 也是工作複本的。
FREEZE="$CLONE/python/scripts/i074_stage2_freeze_record.py"
if [ "$FORMAL" = "1" ]; then
  python3 "$FREEZE" build --report "$S/sizing_report.json" --repo "$CLONE" --identity "$IDENTITY" \
    --out "$S/freeze_record.json" || on_failure "freeze record" 1
fi
for wt in "$L3"/tmp.*; do
  [ -d "$wt" ] && git -C "$CLONE" worktree remove --force "$wt" >/dev/null 2>&1 || true
done
# ⚠️ 本次 run id 的容器必須一個都不剩——在寫出任何報告**之前**檢查（⛔ 不留下「有報告卻失敗」的矛盾狀態）。
ensure_no_run_containers "結束前的容器檢查"
measure_check_adopted "結束前的收養檢查"     # ⑦d 增補
cp "$S/sizing_report.json" "$S/sizing_report.txt" "$WORK/"
if [ -f "$S/freeze_record.json" ]; then
  cp "$S/freeze_record.json" "$WORK/"
  # 複製之後對兩個副本再驗一次；不符就刪掉 freeze record（⛔ 不留下可以進 ⑩ 的錯誤紀錄）。
  if ! python3 "$FREEZE" check-pair --record "$WORK/freeze_record.json" --report "$WORK/sizing_report.json"; then
    rm -f -- "$WORK/freeze_record.json"
    on_failure "freeze record 的副本驗證" 1
  fi
  say "==> freeze record：$WORK/freeze_record.json"
fi
cp -a "$S" "$WORK/raw" || { echo "⚠️ 複製原始量測失敗，保留 S：$S" >&2; DONE=1; exit 1; }
DONE=1
rm -rf "$S"
echo "報告：$WORK/sizing_report.json（原始量測：$WORK/raw/）" >&2
