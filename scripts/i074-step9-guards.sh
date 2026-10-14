#!/usr/bin/env bash
# I-074 Stage 2 步驟 ⑨ 的執行入口（docs/issue.md「Stage 2 步驟 ⑨ 細部計畫 v1」「六」）。
#
#   scripts/i074-step9-guards.sh --source-tree <40 碼 tree> --out <repo 外的新目錄>              # S2～S6
#   scripts/i074-step9-guards.sh --source-tree <40 碼 tree> --out <repo 外的新目錄> --s7 smoke   # S7 的 smoke
#   scripts/i074-step9-guards.sh --source-tree <40 碼 tree> --out <repo 外的新目錄> --s7 full    # S7 的完整驗證
#
# ⚠️ **執行邊界**：從正式工作樹啟動的這一支只做 bootstrap——⛔ 不 source 任何 helper，只用 git、coreutils 與 find：
#   ① `--source-tree` 是 40 碼的 tree；② **自身的 bytes ＝ `<tree>:scripts/i074-step9-guards.sh`**；③ 可用空間
#   （執行期間的下限 ＋ 隔離 clone 本身的大小）；④ 建綁定 `<tree>` 的隔離 clone（`--no-hardlinks`、`HEAD^{tree}` ＝
#   `<tree>`、工作樹乾淨）；⑤ 以**子程序**執行 clone 裡的同一支（⛔ 不 exec——要保留 EXIT trap 收掉 clone）。之後執行的
#   每一支程式都是 `<tree>` 的內容；worktree 登記只落在 clone 的 `.git`。正式 repo 只做明列的唯讀 inventory（HEAD、
#   index 的 digest、status、worktree 清單，以及物件庫每個檔的 inode／大小／mtime／link 數）。
# ⚠️ **clone ⛔ 不用 hardlink**（⑨ 實作第一輪 review）：本地 clone 預設以 hardlink 共用物件檔，clone 裡的 git 寫入
#   （`add`、`write-tree`、`apply --index`）遇到已存在的物件會 freshen（改 mtime）——改到的是正式 repo 的同一個 inode。
# ⚠️ **image 契約**：會 build 的步驟（S3 的 `e1cbbbd` 既有測試、`--s7 smoke`、`--s7 full`）各用本次執行唯一的 tag，
#   以 `scripts/i074-step9-image.sh` 在 build 之前 reserve、之後 release ＋ verify（成功、失敗、INT、TERM 都一樣）。
#   **結束碼**：主流程 0 而 release／verify／`cleanup_status.json`／worktree 的清理／正式 repo 物件庫的比對任一失敗
#   → 1；主流程非零或收到 INT（130）／TERM（143）→ 保留原碼，清理失敗照樣印到 stderr。
# ⚠️ 整支包在函式裡、最後一行才呼叫並 exit（bash 逐行讀檔，避免執行途中被改到）。
set -euo pipefail

readonly STEP9_BASE=e1cbbbdab44f8cf2d152e6ade9235d844f590d7f
readonly STEP9_T1=a1649733b910ae90f08efc5f215f31f898619f52
readonly STEP9_CF_SHA=fa7f5ba840a3106978ad1ea1661eae7c97cb77710db2a7e042bb6f1b3a6fea14
readonly STEP9_SEM_SHA=5b851e1eef4081af5ba6d4ad6d5ffcfaa3344b051d9594f6d2578853404ec9e8
readonly STEP9_EMPTY_SHA=e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
readonly STEP9_CF_REL=python/baselines/i074_stage2/counterfactual_e1cbbbd.patch
readonly STEP9_TOOL_REL=python/baselines/i074_stage2/tooling_e1cbbbd.patch
readonly STEP9_FORMAL_BUNDLE=b1_20260901_1d_74350966_5d7ecb10
readonly STEP9_SLICE_ID=b1_20260901_1d_de3ab843_a7c9ffb4
readonly STEP9_E1_TESTS_SUMMARY="1223 passed, 1 skipped"
# 隔離 clone 建好之後、執行任何步驟之前必須還有的可用空間。依據（2026-10-12 實測）：`--s7 full` 裡 `test-i074-stage2.sh`
# 的 preflight 6 需要 ≥ 1,241,513,984 bytes（T-072），而從 clone 建好到 `test-i074-stage2.sh` 結束，完整驗證本身最多吃掉
# 197,275,648 bytes（兩趟 194～197 MB；之後的 pytest 段最多 315 MB，但它⛔ 不受 preflight 6 約束）；兩者相加約 1.44 GB，
# 再留約 60 MB 餘裕。bootstrap 另外加上 clone 本身的大小（正式 repo 的物件庫 ＋ `<tree>` 的工作樹），⛔ 不靠 hardlink 省空間。
readonly STEP9_MIN_FREE_BYTES=1500000000
readonly STEP9_SELF_REL=scripts/i074-step9-guards.sh
readonly STEP9_IDENTITY_REL=stock_trading/i074_stage2/run_identity.json
# 會執行或讀取的 repo 檔案（`step9_manifest.json` 的明列紀錄；結構上的封閉由 clone 保證）。
readonly STEP9_MANIFEST_PATHS=(
  scripts/i074-step9-guards.sh scripts/i074-step9-image.sh scripts/lib/replay-args.sh scripts/lib/mem-guard.sh
  scripts/make-i074-tooling-patch.sh scripts/run-replay-offline.sh scripts/smoke-replay-offline.sh
  scripts/test-replay-args.sh scripts/test-i074-stage2.sh scripts/test-i074-step9.sh
  python/scripts/test.sh python/scripts/_i074_bootstrap.py python/scripts/validate-i074-run-identity.py
  python/scripts/make_counterfactual_smoke_bundle.py python/scripts/i074_decision_summary_producer.py
  python/scripts/i074_differential_guard.py python/scripts/i074_tooling_guard.py python/scripts/i074_step9_runs.py
  python/scripts/fixtures/i074_differential_guard_cases.json python/scripts/fixtures/i074_step9/step8_full_run3.log
  python/scripts/fixtures/i074_step9/step8_full_run3.meta.json
  python/backtest/modular/sr_scoring/evaluation.py python/backtest/modular/sr_scoring/replay_bundle
  "$STEP9_CF_REL" "$STEP9_TOOL_REL"
)

die() { echo "ERROR: i074-step9-guards：$*" >&2; exit 1; }
say() { echo "==> [step9] $*"; }

# ─────────────────────────────── bootstrap（正式工作樹） ───────────────────────────────

STEP9_WORK=""
STEP9_INNER_PID=""
STEP9_OBJDIR=""
STEP9_BOOT_OUT=""
step9_bootstrap_cleanup() {
  [ -n "$STEP9_WORK" ] || return 0
  chmod -R u+w "$STEP9_WORK" 2>/dev/null || true
  rm -rf -- "$STEP9_WORK"
}

step9_objects_dir() {  # $1＝repo → 物件庫的絕對路徑（git 2.30 沒有 --path-format）
  local common
  common="$(cd "$1" && cd "$(git rev-parse --git-common-dir)" && pwd -P)" || return 1
  printf '%s/objects\n' "$common"
}

# 物件庫每個檔的相對路徑、inode、大小、mtime、link 數（⛔ 不讀內容）。
step9_objects_inventory() { find "$1" -type f -printf '%P %i %s %T@ %n\n' | LC_ALL=C sort; }

step9_objects_verify() {  # 正式 repo 的物件庫前後完全相同 → 0；不同或量不到 → 1（訊息印到 stderr）
  [ -n "$STEP9_OBJDIR" ] && [ -f "$STEP9_BOOT_OUT/formal_objects_before.txt" ] || return 0
  if ! step9_objects_inventory "$STEP9_OBJDIR" > "$STEP9_BOOT_OUT/formal_objects_after.txt"; then
    echo "ERROR: i074-step9-guards：正式 repo 物件庫的 inventory 失敗" >&2
    return 1
  fi
  if ! cmp -s "$STEP9_BOOT_OUT/formal_objects_before.txt" "$STEP9_BOOT_OUT/formal_objects_after.txt"; then
    diff "$STEP9_BOOT_OUT/formal_objects_before.txt" "$STEP9_BOOT_OUT/formal_objects_after.txt" | head -n 20 >&2 || true
    echo "ERROR: i074-step9-guards：正式 repo 的物件庫前後不同（見 formal_objects_before.txt／formal_objects_after.txt）——⛔ 不是嚴格為零" >&2
    return 1
  fi
}

step9_bootstrap_exit() {  # bootstrap 中途 die 的收尾（結束碼照舊）
  local rc=$?
  trap - EXIT INT TERM
  step9_bootstrap_cleanup || echo "ERROR: i074-step9-guards：收不掉工作目錄 $STEP9_WORK" >&2
  step9_objects_verify || true
  exit "$rc"
}

step9_bootstrap_signal() {  # $1＝signal 編號
  local sig="$1" rc=$((128 + $1))
  trap - INT TERM
  if [ -n "$STEP9_INNER_PID" ]; then
    kill -TERM "$STEP9_INNER_PID" 2>/dev/null || true
    wait "$STEP9_INNER_PID" 2>/dev/null || true
  fi
  step9_bootstrap_cleanup || echo "ERROR: i074-step9-guards：收不掉工作目錄 $STEP9_WORK" >&2
  step9_objects_verify || true
  trap - EXIT
  exit "$rc"
}

step9_make_clone() {  # $1＝正式 repo $2＝tree $3＝clone 目的地
  local repo="$1" tree="$2" clone="$3" got commit linked
  git -c core.hooksPath=/dev/null clone -q --no-hardlinks --no-checkout --template= "$repo" "$clone" \
    || die "建不了隔離 clone"
  # ⛔ 不與正式 repo 共用任何物件檔：在 clone 裡做任何 git 寫入之前先驗。
  linked="$(find "$clone/.git/objects" -type f -links +1 -print -quit)" || die "量不到隔離 clone 物件檔的 link 數"
  [ -z "$linked" ] || die "隔離 clone 的物件檔與其他路徑共用 inode（$linked）——⛔ 不在 hardlink 的 clone 上執行"
  [ ! -e "$clone/.git/objects/info/alternates" ] || die "隔離 clone 有 alternates——⛔ 不與正式 repo 共用物件庫"
  git -C "$repo" archive "$tree" | tar -x -C "$clone" || die "展開不了 $tree"
  git -C "$clone" -c core.hooksPath=/dev/null add -A || die "隔離 clone 的 git add 失敗"
  got="$(git -C "$clone" write-tree)" || die "隔離 clone 的 write-tree 失敗"
  [ "$got" = "$tree" ] || die "隔離 clone 的 tree $got ≠ $tree（⛔ 不在不完整的 checkout 上執行）"
  commit="$(GIT_AUTHOR_NAME=i074-step9 GIT_AUTHOR_EMAIL=i074-step9@invalid GIT_COMMITTER_NAME=i074-step9 \
            GIT_COMMITTER_EMAIL=i074-step9@invalid git -C "$clone" commit-tree "$tree" -p HEAD -m "i074 step9: $tree")" \
    || die "隔離 clone 的 commit-tree 失敗"
  git -C "$clone" update-ref HEAD "$commit" || die "隔離 clone 的 update-ref 失敗"
  [ "$(git -C "$clone" rev-parse 'HEAD^{tree}')" = "$tree" ] || die "隔離 clone 的 HEAD^{tree} ≠ $tree"
  [ -z "$(git --no-optional-locks -C "$clone" status --porcelain=v1 --untracked-files=all)" ] \
    || die "隔離 clone 不乾淨"
  git -C "$clone" cat-file -e "${STEP9_BASE}^{commit}" || die "隔離 clone 裡沒有 ${STEP9_BASE:0:7}"
}

step9_bootstrap() {
  local tree="" out="" s7="" self repo expected avail objdir objsize treesize need rc
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --source-tree) [ "$#" -ge 2 ] || die "--source-tree 需要值"; [ -z "$tree" ] || die "--source-tree 重複"; tree="$2"; shift 2 ;;
      --out) [ "$#" -ge 2 ] || die "--out 需要值"; [ -z "$out" ] || die "--out 重複"; out="$2"; shift 2 ;;
      --s7) [ "$#" -ge 2 ] || die "--s7 需要值"; [ -z "$s7" ] || die "--s7 重複"; s7="$2"; shift 2 ;;
      --in-clone) die "--in-clone 只能由 bootstrap 在隔離 clone 內使用" ;;
      *) die "未知參數：$1" ;;
    esac
  done
  [ -n "$tree" ] && [ -n "$out" ] || die "用法：--source-tree <40 碼 tree> --out <新目錄> [--s7 smoke|full]"
  case "$s7" in ""|smoke|full) ;; *) die "--s7 只接受 smoke 或 full" ;; esac
  self="$(realpath "${BASH_SOURCE[0]}")"
  repo="$(cd "$(dirname "$self")/.." && pwd -P)"
  [[ "$tree" =~ ^[0-9a-f]{40}$ ]] && [ "$(git -C "$repo" cat-file -t "$tree" 2>/dev/null)" = tree ] \
    || die "--source-tree 只接受 40 碼的 tree OID（⛔ 不接受 commit、ref）：$tree"
  # ② 自身 ＝ T_src 的 blob（⛔ 不 source 任何 helper 之前先驗）。
  expected="$(mktemp)"
  git -C "$repo" cat-file blob "$tree:$STEP9_SELF_REL" > "$expected" 2>/dev/null || { rm -f "$expected"; die "$tree 裡沒有 $STEP9_SELF_REL"; }
  if ! cmp -s "$expected" "$self"; then
    rm -f "$expected"
    die "執行中的入口 ≠ $tree 裡的 $STEP9_SELF_REL——受測的不是要封存的內容（⛔ 不執行）"
  fi
  rm -f "$expected"
  [ ! -e "$out" ] || die "--out 已存在（⛔ 不覆寫）：$out"
  mkdir -p -- "$(dirname "$out")"
  out="$(cd "$(dirname "$out")" && pwd -P)/$(basename "$out")"
  case "$out/" in "$repo"/*) die "--out 必須在 repo 之外：$out" ;; esac
  # ③ 可用空間 ＝ 執行期間的下限 ＋ clone 本身（`--no-hardlinks` 複製整個物件庫 ＋ `<tree>` 的工作樹）；不足即停（T-072）。
  objdir="$(step9_objects_dir "$repo")" || die "找不到正式 repo 的物件庫"
  objsize="$(du -sb "$objdir" | cut -f1)" || die "量不到物件庫的大小"
  treesize="$(git -C "$repo" archive "$tree" | wc -c)" || die "量不到 $tree 的大小"
  need=$((STEP9_MIN_FREE_BYTES + objsize + treesize))
  avail="$(df -B1 --output=avail "${TMPDIR:-/tmp}" | tail -n1 | tr -d ' ')"
  [ "$avail" -ge "$need" ] || die "${TMPDIR:-/tmp} 可用空間 $avail bytes < $need（執行期間的下限 $STEP9_MIN_FREE_BYTES ＋ 隔離 clone 的物件庫 $objsize ＋ 工作樹 $treesize；T-072）——⛔ 不降低隔離換取空間"
  mkdir -- "$out"
  STEP9_BOOT_OUT="$out"
  step9_objects_inventory "$objdir" > "$out/formal_objects_before.txt" || die "正式 repo 物件庫的 inventory 失敗"
  STEP9_OBJDIR="$objdir"
  STEP9_WORK="$(mktemp -d "${TMPDIR:-/tmp}/i074-step9.XXXXXXXX")"
  trap step9_bootstrap_exit EXIT
  trap 'step9_bootstrap_signal 2' INT
  trap 'step9_bootstrap_signal 15' TERM
  say "隔離 clone：$STEP9_WORK/clone（tree $tree；可用空間 $avail bytes ≥ $need）"
  step9_make_clone "$repo" "$tree" "$STEP9_WORK/clone"
  rc=0
  bash "$STEP9_WORK/clone/$STEP9_SELF_REL" --in-clone --source-tree "$tree" --out "$out" --work "$STEP9_WORK" \
    --formal-repo "$repo" ${s7:+--s7 "$s7"} &
  STEP9_INNER_PID=$!
  wait "$STEP9_INNER_PID" || rc=$?
  STEP9_INNER_PID=""
  trap - INT TERM
  if ! step9_bootstrap_cleanup; then
    echo "ERROR: i074-step9-guards：收不掉工作目錄 $STEP9_WORK" >&2
    [ "$rc" -ne 0 ] || rc=1
  fi
  if ! step9_objects_verify; then
    [ "$rc" -ne 0 ] || rc=1
  fi
  trap - EXIT
  return "$rc"
}

# ─────────────────────────────── clone 內 ───────────────────────────────

REPO_ROOT=""
STEP9_OUT=""
STEP9_FORMAL=""
STEP9_TREE=""
STEP9_RUN_ID=""
STEP9_CHILD_PGID=""
# image 契約的狀態（trap 依它決定要不要 release）
STEP9_IMG_TAG=""
STEP9_IMG_INV=""
STEP9_IMG_SHA=""
STEP9_IMG_ALLOW=""

step9_formal_inventory() {  # 正式 repo 與 host 的唯讀 inventory（⛔ 不寫入任何東西）
  local repo="$STEP9_FORMAL"
  echo "## head"; git -C "$repo" rev-parse HEAD
  echo "## index"; git -C "$repo" ls-files --stage | sha256sum | cut -d' ' -f1
  echo "## status"; git --no-optional-locks -C "$repo" status --porcelain=v1 --untracked-files=all
  echo "## worktrees"; git -C "$repo" worktree list --porcelain
  echo "## containers"; docker ps -aq --no-trunc | LC_ALL=C sort
  echo "## run-lock"; ls -A /run/lock 2>/dev/null | { grep '^i074-stage2' || true; }
  echo "## dev-shm"; ls -A /dev/shm 2>/dev/null | { grep '^i074-' || true; }
}

# `/tmp` 是 host 共用的（同一個 uid 的其他代理程式也會寫）：⛔ 不列進上面那份「不符即失敗」的 inventory，
# 改成前後各記一份、把差異寫進 tmp_diff.txt，由結果逐一歸因（本輪的程式一律跟隨 TMPDIR，應為零）。
step9_tmp_listing() { ls -A /tmp | { grep -vxF "$(basename "$STEP9_WORK_DIR")" || true; } | LC_ALL=C sort; }

step9_image_release() {  # release ＋ verify（release 失敗也照樣 verify），結果寫進 cleanup_status.json
  [ -n "$STEP9_IMG_TAG" ] || return 0
  local helper="$REPO_ROOT/scripts/i074-step9-image.sh" rel=ok ver=ok rc=0
  "$helper" release "$STEP9_IMG_TAG" "$STEP9_IMG_INV" "$STEP9_IMG_SHA" "$STEP9_IMG_ALLOW" || { rel=failed; rc=1; }
  "$helper" verify "$STEP9_IMG_TAG" "$STEP9_IMG_INV" "$STEP9_IMG_SHA" "$STEP9_IMG_ALLOW" || { ver=failed; rc=1; }
  if ! python3 -I -B - "$STEP9_OUT/cleanup_status.json" "$STEP9_IMG_TAG" "$rel" "$ver" "$STEP9_IMG_ALLOW" <<'PY'
import json, os, sys
path, tag, rel, ver, allow = sys.argv[1:6]
entries = []
if os.path.exists(allow):
    entries = [l.strip() for l in open(allow, encoding="utf-8") if l.strip()]
status = {}
if os.path.exists(path):
    status = json.load(open(path, encoding="utf-8"))
status[tag] = {"release": rel, "verify": ver, "allowance": entries}
tmp = path + ".tmp"
with open(tmp, "w", encoding="utf-8") as f:
    json.dump(status, f, ensure_ascii=False, sort_keys=True, indent=1)
    f.write("\n")
os.replace(tmp, path)
PY
  then
    echo "ERROR: i074-step9-guards：cleanup_status.json 寫入失敗（$STEP9_IMG_TAG：release=$rel verify=$ver）" >&2
    rc=1
  fi
  [ "$rc" -eq 0 ] || echo "ERROR: i074-step9-guards：image 契約的收尾失敗（$STEP9_IMG_TAG：release=$rel verify=$ver）" >&2
  STEP9_IMG_TAG=""
  return "$rc"
}

step9_inner_signal() {  # $1＝signal 編號
  local rc=$((128 + $1))
  trap - INT TERM
  if [ -n "$STEP9_CHILD_PGID" ]; then
    kill -TERM -- "-$STEP9_CHILD_PGID" 2>/dev/null || true
    wait 2>/dev/null || true
  fi
  step9_image_release || true
  trap - EXIT
  exit "$rc"
}

step9_inner_exit() {
  local rc=$?
  trap - EXIT INT TERM
  if ! step9_image_release; then
    [ "$rc" -ne 0 ] || rc=1
  fi
  exit "$rc"
}

# 長時間的子步驟：放進自己的 process group（收到 INT／TERM 時整組收掉），stdout／stderr 寫進 $1。
step9_run_child() {  # $1＝log；其餘＝指令
  local log="$1" rc=0; shift
  setsid "$@" > "$log" 2>&1 &
  STEP9_CHILD_PGID=$!
  wait "$STEP9_CHILD_PGID" || rc=$?
  STEP9_CHILD_PGID=""
  return "$rc"
}

step9_image_reserve() {  # $1＝步驟名
  local tag="stock-trading-i074-step9:${STEP9_RUN_ID}-$1"
  STEP9_IMG_INV="$STEP9_OUT/image_$1.inventory"
  STEP9_IMG_ALLOW="$STEP9_OUT/image_$1.allowance"
  STEP9_IMG_SHA="$("$REPO_ROOT/scripts/i074-step9-image.sh" reserve "$tag" "$STEP9_IMG_INV")" \
    || die "image 契約：reserve $tag 被拒絕"
  STEP9_IMG_TAG="$tag"
}

step9_sha() { sha256sum < "$1" | cut -d' ' -f1; }

step9_resolve_image() {  # 設 STEP9_IMAGE_ID、STEP9_IDENTITY_SHA（唯讀讀取 identity；不在本機就停）。⛔ 不在 $(…) 裡呼叫
  local identity id
  identity="${XDG_DATA_HOME:-$HOME/.local/share}/$STEP9_IDENTITY_REL"
  [ -f "$identity" ] || die "找不到 Stage 2 identity：$identity（⛔ 不自行 pin）"
  id="$(python3 -I -B -c 'import json,sys; print(json.load(open(sys.argv[1]))["expected_image_id"])' "$identity")" \
    || die "讀不到 identity 的 expected_image_id"
  python3 -B "$REPO_ROOT/python/scripts/validate-i074-run-identity.py" "$identity" --expect-image-id "$id" \
    --bundle "$REPO_ROOT/python/baselines/$STEP9_FORMAL_BUNDLE" > /dev/null || die "Stage 2 identity 驗證不過"
  [ "$(docker image inspect "$id" -f '{{.Id}}' 2>/dev/null || true)" = "$id" ] \
    || die "Stage 2 pinned image $id 不在本機（⛔ 不自行 build 替代）"
  STEP9_IDENTITY_SHA="$(step9_sha "$identity")"
  STEP9_IMAGE_ID="$id"
}

step9_manifest() {  # step9_manifest.json：明列的 repo 檔案 → git 物件 OID，與 repo 外的輸入
  local list="$STEP9_WORK_DIR/manifest.list" path
  : > "$list"
  for path in "${STEP9_MANIFEST_PATHS[@]}"; do
    git -C "$REPO_ROOT" ls-tree "$STEP9_TREE" -- "$path" >> "$list" || die "ls-tree $path 失敗"
  done
  python3 -I -B - "$STEP9_OUT/step9_manifest.json" "$list" "$STEP9_TREE" "$STEP9_RUN_ID" "$1" \
    "${STEP9_IMAGE_ID:-}" "${STEP9_IDENTITY_SHA:-}" "$(git --version)" "$(python3 --version 2>&1)" \
    "$(docker version -f '{{.Server.Version}}' 2>/dev/null || echo unknown)" <<'PY'
import json, sys
out, listing, tree, run_id, mode, image, identity, gitv, pyv, dockerv = sys.argv[1:11]
files = {}
for line in open(listing, encoding="utf-8").read().splitlines():
    meta, path = line.split("\t", 1)
    _mode, kind, oid = meta.split(" ")
    files[path] = {"type": kind, "oid": oid}
doc = {"schema": "i074_step9_manifest_v1", "source_tree": tree, "run_id": run_id, "mode": mode, "files": files,
       "external": {"image_id": image or None, "stage2_identity_sha256": identity or None,
                    "git": gitv, "python3": pyv, "docker_server": dockerv}}
with open(out, "w", encoding="utf-8") as f:
    json.dump(doc, f, ensure_ascii=False, sort_keys=True, indent=1)
    f.write("\n")
PY
}

step9_s2_compose() {
  local base_tree cf="$REPO_ROOT/$STEP9_CF_REL" tool="$REPO_ROOT/$STEP9_TOOL_REL" line tool2 comp2
  say "S2：兩份 patch 沒有被 ⑨ 改動、產生器 --verify、合成與三方 SHA"
  git -C "$REPO_ROOT" diff --quiet HEAD~1 HEAD -- "$STEP9_CF_REL" "$STEP9_TOOL_REL" \
    || die "⑨ 改到了兩份 patch（相對啟動當下的正式 HEAD）——⑨ ⛔ 不得改 patch"
  "$REPO_ROOT/scripts/make-i074-tooling-patch.sh" --verify "$STEP9_TREE" > "$STEP9_OUT/tooling_verify.log" 2>&1 \
    || die "make-i074-tooling-patch.sh --verify 不通過（見 tooling_verify.log）"
  base_tree="$(git -C "$REPO_ROOT" rev-parse "${STEP9_BASE}^{tree}")"
  STEP9_WT_O="$STEP9_WORK_DIR/wt_original"; STEP9_WT_P="$STEP9_WORK_DIR/wt_patched"; local wt_k="$STEP9_WORK_DIR/wt_tooling_only"
  replay_args_prepare_worktree "$REPO_ROOT" "$STEP9_BASE" "$STEP9_WT_O" > /dev/null || die "建不了原始側的 worktree"
  [ "$(git -C "$STEP9_WT_O" write-tree)" = "$base_tree" ] || die "原始側的 worktree ≠ base tree"
  replay_args_prepare_worktree "$REPO_ROOT" "$STEP9_BASE" "$STEP9_WT_P" > /dev/null || die "建不了 patched 側的 worktree"
  line="$(replay_args_compose "$STEP9_WT_P" "$STEP9_BASE" "$cf" "$tool")" || die "合成失敗"
  read -r STEP9_T1_GOT STEP9_T2 STEP9_CF_GOT STEP9_TOOL_GOT STEP9_COMP STEP9_SEM_GOT <<< "$line"
  replay_args_prepare_worktree "$REPO_ROOT" "$STEP9_BASE" "$wt_k" > /dev/null || die "建不了只套 tooling 的 worktree"
  line="$(replay_args_compose "$wt_k" "$STEP9_BASE" "" "$tool")" || die "只套 tooling 的合成失敗"
  read -r _ _ _ tool2 comp2 _ <<< "$line"
  STEP9_BASE_TREE="$base_tree"
  STEP9_CF_RAW="$(step9_sha "$cf")"; STEP9_TOOL_RAW="$(step9_sha "$tool")"
  [ "$STEP9_T1_GOT" = "$STEP9_T1" ] || die "T1 $STEP9_T1_GOT ≠ $STEP9_T1"
  [ "$STEP9_CF_GOT" = "$STEP9_CF_SHA" ] && [ "$STEP9_CF_RAW" = "$STEP9_CF_SHA" ] \
    || die "counterfactual 的 canonical／raw SHA ≠ $STEP9_CF_SHA（$STEP9_CF_GOT／$STEP9_CF_RAW）"
  [ "$STEP9_SEM_GOT" = "$STEP9_SEM_SHA" ] || die "語意 SHA $STEP9_SEM_GOT ≠ $STEP9_SEM_SHA"
  [ "$STEP9_TOOL_GOT" = "$STEP9_TOOL_RAW" ] || die "tooling 的 raw SHA ≠ canonical（$STEP9_TOOL_RAW／$STEP9_TOOL_GOT）"
  [ "$STEP9_TOOL_GOT" != "$STEP9_EMPTY_SHA" ] || die "tooling patch 是空的"
  [ "$tool2" = "$STEP9_TOOL_GOT" ] && [ "$comp2" = "$STEP9_TOOL_GOT" ] \
    || die "只套 tooling 重算的 SHA（$tool2／$comp2）≠ $STEP9_TOOL_GOT——兩份 patch 的 bytes 並⛔ 不獨立"
}

step9_s3_e1cbbbd_tests() {
  local rc=0 summary
  say "S3：e1cbbbd ＋ 兩份 patch 的既有測試（它自己的 python/scripts/test.sh）"
  step9_image_reserve e1cbbbd-test
  step9_run_child "$STEP9_OUT/e1cbbbd_tests.log" env PY_IMAGE="$STEP9_IMG_TAG" SKIP_SHELL_TESTS=1 \
    bash "$STEP9_WT_P/python/scripts/test.sh" || rc=$?
  STEP9_E1_IMAGE="$(docker image inspect "$STEP9_IMG_TAG" -f '{{.Id}}' 2>/dev/null || echo -)"
  step9_image_release || die "S3 的 image 契約收尾失敗"
  [ "$rc" -eq 0 ] || die "e1cbbbd 既有測試失敗（rc=$rc，見 e1cbbbd_tests.log）"
  summary="$(grep -E '^=+ .* in [0-9.]+s' "$STEP9_OUT/e1cbbbd_tests.log" | tail -n1)"
  case "$summary" in *" $STEP9_E1_TESTS_SUMMARY "*|*" $STEP9_E1_TESTS_SUMMARY,"*) ;;
    *) die "e1cbbbd 既有測試的摘要 ≠「$STEP9_E1_TESTS_SUMMARY」：$summary" ;; esac
  STEP9_E1_SUMMARY="$summary"
}

step9_docker_run() {  # 其餘＝docker run 的參數（固定的隔離選項由這裡加）
  docker run --rm --network none --user "$(id -u):$(id -g)" --memory="$STEP9_MEM" --memory-swap="$STEP9_MEM" \
    -e HOME=/tmp -e PYTHONDONTWRITEBYTECODE=1 "$@"
}

step9_s4_differential() {
  local dir="$STEP9_OUT/differential" side wt
  say "S4：differential guard（兩個獨立的容器 ＋ 第三方 comparator ＋ tamper 自我檢查）"
  mkdir -p "$dir/producer_out"
  for side in original patched; do
    if [ "$side" = original ]; then wt="$STEP9_WT_O"; else wt="$STEP9_WT_P"; fi
    step9_docker_run -v "$wt/python":/app:ro \
      -v "$REPO_ROOT/python/scripts/i074_decision_summary_producer.py":/guard/i074_decision_summary_producer.py:ro \
      -v "$REPO_ROOT/python/scripts/i074_differential_guard.py":/guard/i074_differential_guard.py:ro \
      -v "$REPO_ROOT/python/scripts/fixtures/i074_differential_guard_cases.json":/guard/fixture.json:ro \
      -v "$dir/producer_out":/out "$STEP9_IMAGE_ID" \
      python -I -B /guard/i074_decision_summary_producer.py --fixture /guard/fixture.json --source-root /app \
      --side "$side" --out "/out/$side.json" > "$dir/producer_$side.log" 2>&1 \
      || die "producer（$side）失敗（見 differential/producer_$side.log）"
  done
  python3 -I -B "$REPO_ROOT/python/scripts/i074_differential_guard.py" \
    --fixture "$REPO_ROOT/python/scripts/fixtures/i074_differential_guard_cases.json" \
    --before-patched "$dir/producer_out/patched.json" --after-original "$dir/producer_out/original.json" \
    --repo "$REPO_ROOT" --patched-tree "$STEP9_T2" --original-tree "$STEP9_BASE_TREE" \
    --self-test-tampers --out "$dir/report.json" > "$dir/comparator.log" 2>&1 \
    || die "differential guard 不通過（見 differential/report.json）"
}

step9_runner() {  # $1＝名稱 $2＝tooling patch（或空）；其餘＝runner 參數
  local name="$1" tool="$2" tmp="$STEP9_WORK_DIR/tmp_runner_$1" rc=0; shift 2
  mkdir -p "$tmp"
  env -u PY_IMAGE TMPDIR="$tmp" REPLAY_IMAGE_ID="$STEP9_IMAGE_ID" AFTER_REF="$STEP9_BASE" ${tool:+TOOLING_PATCH="$tool"} \
    "$REPO_ROOT/scripts/run-replay-offline.sh" "$@" > "$STEP9_OUT/tooling/$name.log" 2>&1 || rc=$?
  replay_args_remove_worktrees_under "$REPO_ROOT" "$tmp" || die "收不掉 $name 的 worktree"
  [ "$rc" -eq 0 ] || die "runner（$name）失敗（rc=$rc，見 tooling/$name.log）"
}

step9_s5_tooling() {
  local dir="$STEP9_OUT/tooling" slice tool="$REPO_ROOT/$STEP9_TOOL_REL" id
  say "S5：tooling 非語意 guard（切片 bundle、Stage 1／2 的 A／B 成對比較）"
  mkdir -p "$dir/slice"
  step9_docker_run -v "$REPO_ROOT/python":/app:ro -v "$dir/slice":/out -w /app "$STEP9_IMAGE_ID" \
    python scripts/make_counterfactual_smoke_bundle.py "/app/baselines/$STEP9_FORMAL_BUNDLE" /out \
    > "$dir/slice.out" 2> "$dir/slice.log" || die "切片 bundle 產生失敗（見 tooling/slice.log）"
  id="$(tail -n1 "$dir/slice.out")"
  [ "$id" = "$STEP9_SLICE_ID" ] && [ "$(ls -A "$dir/slice")" = "$STEP9_SLICE_ID" ] \
    || die "切片 bundle 的 ID 不符（'$id'）——⛔ 不跑任何 replay"
  slice="$dir/slice/$STEP9_SLICE_ID"
  step9_runner stage1_a "" --bundle "$slice" --output-dir "$dir/stage1_a" --before-ref "$STEP9_BASE"
  step9_runner stage2_a "" --bundle "$slice" --output-dir "$dir/stage2_a" --before-ref "$STEP9_BASE" \
    --after-artifact "$dir/stage1_a/after_artifact.json" --cohort-manifest "$dir/stage1_a/cohort_manifest.json"
  step9_runner stage2_b "$tool" --bundle "$slice" --output-dir "$dir/stage2_b" --before-ref "$STEP9_BASE" \
    --after-artifact "$dir/stage1_a/after_artifact.json" --cohort-manifest "$dir/stage1_a/cohort_manifest.json"
  step9_runner stage1_b "$tool" --bundle "$slice" --output-dir "$dir/stage1_b" --before-ref "$STEP9_BASE"
  python3 -I -B "$REPO_ROOT/python/scripts/i074_tooling_guard.py" \
    --stage1-a "$dir/stage1_a" --stage1-b "$dir/stage1_b" --stage2-a "$dir/stage2_a" --stage2-b "$dir/stage2_b" \
    --expected-tooling-sha256 "$STEP9_TOOL_GOT" --base-commit "$STEP9_BASE" --out "$dir/report.json" \
    > "$dir/comparator.log" 2>&1 || die "tooling 非語意 guard 不通過（見 tooling/report.json）"
}

step9_write_results() {  # $1＝mode
  python3 -I -B - "$STEP9_OUT/results.json" "$1" "$STEP9_TREE" "$STEP9_RUN_ID" \
    "T1=${STEP9_T1_GOT:-}" "T2=${STEP9_T2:-}" "base_tree=${STEP9_BASE_TREE:-}" \
    "counterfactual_sha256=${STEP9_CF_RAW:-}" "counterfactual_canonical_sha256=${STEP9_CF_GOT:-}" \
    "tooling_sha256=${STEP9_TOOL_RAW:-}" "tooling_canonical_sha256=${STEP9_TOOL_GOT:-}" \
    "composed_sha256=${STEP9_COMP:-}" "counterfactual_semantic_sha256=${STEP9_SEM_GOT:-}" \
    "e1cbbbd_tests_summary=${STEP9_E1_SUMMARY:-}" "e1cbbbd_tests_image_id=${STEP9_E1_IMAGE:-}" \
    "image_id=${STEP9_IMAGE_ID:-}" <<'PY'
import json, sys
out, mode, tree, run_id = sys.argv[1:5]
values = dict(arg.split("=", 1) for arg in sys.argv[5:])
doc = {"schema": "i074_step9_results_v1", "mode": mode, "source_tree": tree, "run_id": run_id,
       "values": {k: v for k, v in values.items() if v}}
with open(out, "w", encoding="utf-8") as f:
    json.dump(doc, f, ensure_ascii=False, sort_keys=True, indent=1)
    f.write("\n")
PY
}

step9_check_formal_unchanged() {
  step9_formal_inventory > "$STEP9_OUT/formal_after.txt" 2>&1 || die "正式 repo 的 inventory 失敗"
  step9_tmp_listing > "$STEP9_OUT/tmp_after.txt" || die "/tmp 的 inventory 失敗"
  LC_ALL=C diff "$STEP9_OUT/tmp_before.txt" "$STEP9_OUT/tmp_after.txt" > "$STEP9_OUT/tmp_diff.txt" \
    || echo "WARN: i074-step9-guards：/tmp 的頂層項目前後不同（見 tmp_diff.txt）——要逐一歸因，對不到本輪才算殘留" >&2
  if ! cmp -s "$STEP9_OUT/formal_before.txt" "$STEP9_OUT/formal_after.txt"; then
    diff "$STEP9_OUT/formal_before.txt" "$STEP9_OUT/formal_after.txt" | head -n 20 >&2 || true
    die "正式 repo／host 的 inventory 前後不同（見 formal_before.txt／formal_after.txt）——⛔ 不是嚴格為零"
  fi
}

step9_s7_smoke() {
  local rc=0 tmp="$STEP9_WORK_DIR/tmp_smoke"
  say "S7：smoke（REPLAY_SMOKE=1，隔離 clone 內）"
  mkdir -p "$tmp"
  step9_image_reserve smoke
  step9_run_child "$STEP9_OUT/smoke.log" env REPLAY_SMOKE=1 PY_IMAGE="$STEP9_IMG_TAG" TMPDIR="$tmp" \
    bash "$REPO_ROOT/scripts/smoke-replay-offline.sh" || rc=$?
  if ! replay_args_remove_worktrees_under "$REPO_ROOT" "$tmp"; then
    echo "ERROR: i074-step9-guards：收不掉 smoke 的 worktree（$tmp）" >&2
    [ "$rc" -ne 0 ] || rc=1
  fi
  step9_image_release || { [ "$rc" -ne 0 ] || rc=1; }
  if [ "$rc" -ne 0 ]; then
    echo "ERROR: i074-step9-guards：smoke 失敗（rc=$rc，見 smoke.log）——結束碼原樣傳出" >&2
    exit "$rc"
  fi
}

step9_s7_full() {
  local rc=0 tmp="$STEP9_WORK_DIR/tmp_full" before after added count outside
  say "S7：完整驗證（python/scripts/test.sh -v backtest/ tests/，隔離 clone 內）"
  mkdir -p "$tmp"
  before="$(git -C "$REPO_ROOT" worktree list --porcelain | awk '/^worktree /{ print $2 }' | LC_ALL=C sort)"
  step9_image_reserve full
  step9_run_child "$STEP9_OUT/full.log" env TMPDIR="$tmp" PY_IMAGE="$STEP9_IMG_TAG" \
    bash "$REPO_ROOT/python/scripts/test.sh" -v backtest/ tests/ || rc=$?
  after="$(git -C "$REPO_ROOT" worktree list --porcelain | awk '/^worktree /{ print $2 }' | LC_ALL=C sort)"
  added="$(LC_ALL=C comm -13 <(printf '%s\n' "$before") <(printf '%s\n' "$after"))"
  printf '%s\n' "$added" > "$STEP9_OUT/full_added_worktrees.txt"
  step9_image_release || { [ "$rc" -ne 0 ] || rc=1; }
  if [ "$rc" -ne 0 ]; then
    echo "ERROR: i074-step9-guards：完整驗證失敗（rc=$rc，見 full.log）——結束碼原樣傳出" >&2
    exit "$rc"
  fi
  # I-118 的 ＋4：恰好 4 筆、全部在專屬 TMPDIR 之下（一致性檢查；之後連同 clone 收掉）
  count="$(grep -c . <<< "$added" || true)"
  outside="$(grep -v "^$tmp/" <<< "$added" || true)"
  [ "$count" -eq 4 ] && [ -z "$outside" ] \
    || die "完整驗證新增的 worktree 登記⛔ 不是恰好 4 筆、全部在 $tmp 之下（見 full_added_worktrees.txt）"
  replay_args_remove_worktrees_under "$REPO_ROOT" "$tmp" || die "收不掉完整驗證的 worktree"
  python3 -I -B "$REPO_ROOT/python/scripts/i074_step9_runs.py" --new-log "$STEP9_OUT/full.log" \
    --out "$STEP9_OUT/full_compare.json" > "$STEP9_OUT/full_compare.log" 2>&1 \
    || die "與 ⑧ baseline 的逐項比對不通過（見 full_compare.json）"
}

step9_inner() {
  local mode="" work="" self avail
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --in-clone) shift ;;
      --source-tree) STEP9_TREE="$2"; shift 2 ;;
      --out) STEP9_OUT="$2"; shift 2 ;;
      --work) work="$2"; shift 2 ;;
      --formal-repo) STEP9_FORMAL="$2"; shift 2 ;;
      --s7) mode="$2"; shift 2 ;;
      *) die "未知參數：$1" ;;
    esac
  done
  self="$(realpath "${BASH_SOURCE[0]}")"
  REPO_ROOT="$(cd "$(dirname "$self")/.." && pwd -P)"
  # 內部參數只在 bootstrap 建的隔離 clone 裡有效。
  [ -n "$work" ] && [ "$REPO_ROOT" = "$(cd "$work" && pwd -P)/clone" ] \
    || die "--in-clone 只能在 bootstrap 建的隔離 clone 裡執行"
  [ "$REPO_ROOT" != "$(cd "$STEP9_FORMAL" && pwd -P)" ] || die "--in-clone ⛔ 不得在正式 repo 執行"
  [ "$(git -C "$REPO_ROOT" rev-parse 'HEAD^{tree}')" = "$STEP9_TREE" ] || die "clone 的 HEAD^{tree} ≠ $STEP9_TREE"
  [ -z "$(git --no-optional-locks -C "$REPO_ROOT" status --porcelain=v1 --untracked-files=all)" ] || die "clone 不乾淨"
  STEP9_WORK_DIR="$(cd "$work" && pwd -P)"
  export TMPDIR="$STEP9_WORK_DIR/tmp"
  mkdir -p "$TMPDIR"
  # shellcheck source=lib/replay-args.sh
  . "$REPO_ROOT/scripts/lib/replay-args.sh"
  # shellcheck source=lib/mem-guard.sh
  . "$REPO_ROOT/scripts/lib/mem-guard.sh"
  STEP9_RUN_ID="$(date -u +%Y%m%dt%H%M%Sz)-$(od -An -N4 -tx1 /dev/urandom | tr -d ' \n')"
  trap step9_inner_exit EXIT
  trap 'step9_inner_signal 2' INT
  trap 'step9_inner_signal 15' TERM
  # 隔離 clone 建好之後的可用空間（bootstrap 量的是建 clone 之前；這裡才是執行期間的起點，T-072）
  avail="$(df -B1 --output=avail "$TMPDIR" | tail -n1 | tr -d ' ')"
  [ "$avail" -ge "$STEP9_MIN_FREE_BYTES" ] \
    || die "建好隔離 clone 之後 $TMPDIR 可用空間 $avail bytes < $STEP9_MIN_FREE_BYTES（T-072）——⛔ 不降低隔離換取空間"
  say "建好隔離 clone 之後的可用空間：$avail bytes"
  step9_formal_inventory > "$STEP9_OUT/formal_before.txt" 2>&1 || die "正式 repo 的 inventory 失敗"
  step9_tmp_listing > "$STEP9_OUT/tmp_before.txt" || die "/tmp 的 inventory 失敗"
  say "run id：$STEP9_RUN_ID；模式：${mode:-guards}"
  case "$mode" in
    "")
      STEP9_MEM="$(mem_guard_clamp "${STEP9_MEM:-400m}")"
      step9_s2_compose
      step9_s3_e1cbbbd_tests
      step9_resolve_image
      step9_s4_differential
      step9_s5_tooling
      ;;
    smoke) step9_s7_smoke ;;
    full) step9_s7_full ;;
  esac
  step9_manifest "${mode:-guards}"
  step9_write_results "${mode:-guards}"
  step9_check_formal_unchanged
  say "完成（${mode:-guards}）：結果在 $STEP9_OUT"
}

step9_main() {
  if [ "${1:-}" = "--in-clone" ]; then
    step9_inner "$@"
  else
    step9_bootstrap "$@"
  fi
}

step9_main "$@"
exit $?
