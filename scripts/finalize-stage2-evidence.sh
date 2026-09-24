#!/usr/bin/env bash
# I-074 Stage 2：證據的發布與修復入口（issue.md I-074 ③ evidence contract「七之四：CLI matrix」）。
#
# 七種模式（互斥、必選其一；路徑一律寫死常數或由 --run-dir 固定推導，⛔ 不開逐檔覆寫）：
#
#   # ③b 環境見證：見證趟的 operational 輸出 → python/baselines/i074_stage2/envcheck/
#   #   ⚠️ EQUIVALENT（0）與 NOT_EQUIVALENT（7）**都會發布證據**
#   REPLAY_IMAGE_ID=sha256:… scripts/finalize-stage2-evidence.sh --envcheck --run-dir <見證趟的 run 目錄>
#   REPLAY_IMAGE_ID=sha256:… scripts/finalize-stage2-evidence.sh --recover-envcheck
#
#   # ③d 成功 archive：run 目錄的 before source ＋ comparison ＋ report ＋ 凍結 patch → …/i074_stage2/evidence/
#   REPLAY_IMAGE_ID=sha256:… scripts/finalize-stage2-evidence.sh --finalize --run-dir <Stage 2 的 run 目錄>
#   REPLAY_IMAGE_ID=sha256:… scripts/finalize-stage2-evidence.sh --recover-durability
#
#   # ③d failed-attempt record（反事實沒有生效，replay 回 6 時）→ …/i074_stage2/failed/<bundle>-<sha>/
#   REPLAY_IMAGE_ID=sha256:… scripts/finalize-stage2-evidence.sh --publish-failed-record --run-dir <run 目錄>
#   REPLAY_IMAGE_ID=sha256:… scripts/finalize-stage2-evidence.sh --check-failed-record --counterfactual-patch <patch>
#   REPLAY_IMAGE_ID=sha256:… scripts/finalize-stage2-evidence.sh --recover-failed-record <record 目錄>
#
# run 目錄的固定佈局（⚠️ 由 ⑦ 的 orchestrator 產出）：
#   witness/after_artifact.json、witness/cohort_manifest.json              ← --envcheck
#   stage2/before_source_artifact.json、stage2/comparison_artifact.json、
#   stage2/report.json                                                     ← --finalize
#   stage2/bounded_diagnostics.json                                        ← --publish-failed-record
#   patches/counterfactual.patch、patches/tooling.patch（凍結副本；tooling 可為 0 bytes）
#
# 結束碼：
#   --envcheck／--recover-envcheck：0＝EQUIVALENT；7＝NOT_EQUIVALENT；3＝durability 未確認；1＝其他
#   --finalize：0；3（⛔ 不刪除，用 --recover-durability）；1
#   --recover-durability：manifest 記的 terminal_outcome；3；1
#   --publish-failed-record：⚠️ **1**（完整發布——那一次執行本來就是失敗的）；3（rename 成功、fsync 失敗）；
#                            1（rename 之前失敗：⚠️ 事故紀錄遺失，允許以同一 SHA 重跑）
#   --check-failed-record：0＝無命中、放行；**2＝命中**（已記錄的壞 patch）；1＝record 損壞、驗證失敗、
#                          patch 套用失敗，或輸入⛔ 不是 canonical 的 `git diff --binary` 輸出
#   --recover-failed-record：⚠️ **1**（成功也是 1）；3
#
# 設計重點：
#   - ⚠️ Python 段在 **Stage 2 identity 的 image** 內執行（`REPLAY_IMAGE_ID` 必須與它相符）；
#   - ⚠️ Stage 1 證據**唯讀**掛載，只有 `python/baselines/i074_stage2/` 可寫（check 模式連它也唯讀）；
#   - 程式碼來自 `--source-ref`（預設 HEAD）的 detached worktree，唯讀掛在 `/app`；
#   - ⚠️ **合成守門**（「四之二」、F8-a）在 host 的隔離 worktree 執行：依 ordered components 套用兩份
#     patch、`git write-tree` 取中繼 tree、重算三個 SHA——⛔ 不建中繼 commit、HEAD 全程不動；
#     ⛔ 任一不符即中止、⛔ 不呼叫 Python。check 模式則是 Python 段先過才跑（失敗點要唯一）。
#   - ⚠️ **合成守門與封存之間的交接**（2026-09-24 review 高 1）：Python 之後會**重新讀**來源，兩段之間
#     來源被一致地換掉的話，shell 證明的是 A、封存的卻是 B（`:ro` 只擋容器寫入，⛔ 凍結不了 host）。
#     所以驗過的 patch base 與三個 SHA 以 `--verified-*` 注入，Python 拿**實際要封存的內容**比對，
#     相符才允許 commit point／fsync。⛔ 不沿用 `--base-commit`——那是 finalizer 程式碼的來源 commit。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PYTHON_DIR="$REPO_ROOT/python"
. "$REPO_ROOT/scripts/lib/replay-args.sh"
. "$REPO_ROOT/scripts/lib/mem-guard.sh"

# ⚠️ 注入參數只能由本腳本給——**精確比對**（⛔ 不共用前綴清單，理由見 finalize-evidence.sh）。
STAGE2_INJECTED=(--run-identity --python-root --image-digest --base-commit --tooling-patch-sha256 --source-root --runner-sha256
                 --verified-patch-base --verified-counterfactual-sha256 --verified-tooling-sha256 --verified-composed-sha256)
for _arg in "$@"; do
  for _inj in "${STAGE2_INJECTED[@]}"; do
    if [ "$_arg" = "$_inj" ] || [ "${_arg%%=*}" = "$_inj" ]; then
      echo "ERROR: $_inj 只能由官方腳本注入，⛔ 不接受使用者傳入。" >&2
      exit 1
    fi
  done
done

MODE=""; RUN_DIR=""; RECORD_DIR=""; CF_PATCH=""; SOURCE_REF="HEAD"; SOURCE_REF_SEEN=0
set_mode() {
  [ -z "$MODE" ] || { echo "ERROR: 模式旗標互斥：已有 --$MODE，又給了 --$1。" >&2; exit 1; }
  MODE="$1"
}
while [ "$#" -gt 0 ]; do
  case "$1" in
    --envcheck|--recover-envcheck|--finalize|--recover-durability|--publish-failed-record|--check-failed-record)
      set_mode "${1#--}"; shift ;;
    --recover-failed-record)
      [ "$#" -ge 2 ] || { echo "ERROR: --recover-failed-record 需要 record 目錄。" >&2; exit 1; }
      set_mode recover-failed-record; RECORD_DIR="$2"; shift 2 ;;
    --run-dir)
      [ "$#" -ge 2 ] || { echo "ERROR: --run-dir 需要值。" >&2; exit 1; }
      [ -z "$RUN_DIR" ] || { echo "ERROR: --run-dir 重複出現——⛔ 不靜默採用最後一個。" >&2; exit 1; }
      RUN_DIR="$2"; shift 2 ;;
    --counterfactual-patch)
      [ "$#" -ge 2 ] || { echo "ERROR: --counterfactual-patch 需要值。" >&2; exit 1; }
      [ -z "$CF_PATCH" ] || { echo "ERROR: --counterfactual-patch 重複出現——⛔ 不靜默採用最後一個。" >&2; exit 1; }
      CF_PATCH="$2"; shift 2 ;;
    --source-ref)
      [ "$#" -ge 2 ] || { echo "ERROR: --source-ref 需要值。" >&2; exit 1; }
      # ⚠️ ⛔ 不靜默採用最後一個：它決定跑哪一份程式碼（CLI matrix：重複參數一律中止）。
      [ "$SOURCE_REF_SEEN" = "0" ] || { echo "ERROR: --source-ref 重複出現——⛔ 不靜默採用最後一個。" >&2; exit 1; }
      SOURCE_REF_SEEN=1; SOURCE_REF="$2"; shift 2 ;;
    *) echo "ERROR: 未知參數 $1（⛔ 不接受逐檔覆寫來源）" >&2; exit 1 ;;
  esac
done
case "$MODE" in
  envcheck|finalize|publish-failed-record)
    [ -n "$RUN_DIR" ] || { echo "ERROR: --$MODE 需要 --run-dir。" >&2; exit 1; } ;;
  recover-envcheck|recover-durability|recover-failed-record|check-failed-record)
    [ -z "$RUN_DIR" ] || { echo "ERROR: --$MODE ⛔ 不接受 --run-dir：它只依賴既有的證據與寫死常數。" >&2; exit 1; } ;;
  *)
    echo "用法: $0 --envcheck --run-dir <dir> | --recover-envcheck | --finalize --run-dir <dir> |" \
         "--recover-durability | --publish-failed-record --run-dir <dir> |" \
         "--check-failed-record --counterfactual-patch <patch> | --recover-failed-record <dir>" >&2
    exit 1 ;;
esac
if [ "$MODE" = "check-failed-record" ]; then
  # ⚠️ **只吃 patch 路徑，⛔ 不吃 SHA**——從介面層杜絕 spoof（「七之一」）。
  [ -n "$CF_PATCH" ] || { echo "ERROR: --check-failed-record 需要 --counterfactual-patch <patch 路徑>。" >&2; exit 1; }
elif [ -n "$CF_PATCH" ]; then
  echo "ERROR: --counterfactual-patch 只屬於 --check-failed-record。" >&2; exit 1
fi

IMAGE_ID="${REPLAY_IMAGE_ID:-}"
[ -n "$IMAGE_ID" ] || { echo "ERROR: ⛔ 一律要求 REPLAY_IMAGE_ID（Stage 2 identity 的 image）。" >&2; exit 1; }
[ -z "${PY_IMAGE:-}" ] || { echo "ERROR: REPLAY_IMAGE_ID 與 PY_IMAGE ⛔ 不可同時設定。" >&2; exit 1; }
docker image inspect "$IMAGE_ID" >/dev/null 2>&1 || {
  echo "ERROR: image $IMAGE_ID 不在本機——用 scripts/restore-replay-image.sh --stage 2 <bundle> 還原。" >&2
  exit 1
}

STAGE1_BASE="$PYTHON_DIR/baselines/i074_stage1"
STAGE2_BASE="$PYTHON_DIR/baselines/i074_stage2"
ENVCHECK_ARCHIVE="$STAGE2_BASE/envcheck"
EVIDENCE_ARCHIVE="$STAGE2_BASE/evidence"
FAILED_ROOT="$STAGE2_BASE/failed"
[ -d "$STAGE1_BASE" ] || { echo "ERROR: 找不到 Stage 1 證據：$STAGE1_BASE（⚠️ 必須與 Stage 2 一起保存）" >&2; exit 1; }
mkdir -p "$STAGE2_BASE"
if [ "$MODE" = "check-failed-record" ]; then
  # ⚠️ lookup 是唯讀的——⛔ 連 i074_stage2 也不給寫。
  MOUNTS=(-v "$STAGE1_BASE":"$STAGE1_BASE":ro -v "$STAGE2_BASE":"$STAGE2_BASE":ro)
else
  MOUNTS=(-v "$STAGE1_BASE":"$STAGE1_BASE":ro -v "$STAGE2_BASE":"$STAGE2_BASE")
fi

validate_identity() {  # $1＝identity 檔（.json 或 .json.gz）
  python3 "$PYTHON_DIR/scripts/validate-i074-run-identity.py" "$1" --expect-image-id "$IMAGE_ID" >/dev/null
}
# ⚠️ ⛔ 固定由 XDG 推導 Stage 2 的 identity，⛔ 沒有覆寫參數。
use_xdg_identity() {
  local identity="${XDG_DATA_HOME:-$HOME/.local/share}/stock_trading/i074_stage2/run_identity.json"
  [ -f "$identity" ] || { echo "ERROR: 找不到 Stage 2 的 run identity：$identity" >&2; exit 1; }
  ID_ABS="$(replay_args_abs_path "$identity" "run identity")"
  validate_identity "$ID_ABS"
  MOUNTS+=(-v "$ID_ABS":"$ID_ABS":ro)
}
# ⚠️ recovery／lookup ⛔ 不掛載也⛔ 不注入外部 identity——只驗封存在 envcheck 的那一份（唯一固定的 Stage 2 identity）。
use_archived_identity() {  # $1＝封存的 identity
  [ -f "$1" ] || { echo "ERROR: 找不到封存的 identity：$1" >&2; exit 1; }
  validate_identity "$1"
}
use_run_dir() {  # 其餘參數＝run 目錄內必須存在的輸入（相對路徑）
  [ -d "$RUN_DIR" ] || { echo "ERROR: --run-dir 不存在：$RUN_DIR" >&2; exit 1; }
  RUN_ABS="$(cd "$RUN_DIR" && pwd)"
  local rel
  for rel in "$@"; do
    # ⚠️ 全部都是**輸入**，⛔ 進 Docker 前必須確定是既有檔案。
    replay_args_require_file "$RUN_ABS/$rel" "--run-dir 的 $rel"
  done
}

CF_FILE=""; TOOL_FILE=""; CLAIM_ARGS=(); VERIFY_ARGS=()
case "$MODE" in
  envcheck)
    use_xdg_identity
    use_run_dir witness/after_artifact.json witness/cohort_manifest.json
    MOUNTS+=(-v "$RUN_ABS/witness":"$RUN_ABS/witness":ro)
    CMD_EXTRA=(--envcheck --run-dir "$RUN_ABS" --run-identity "$ID_ABS") ;;
  recover-envcheck)
    use_archived_identity "$ENVCHECK_ARCHIVE/identity/run_identity.json.gz"
    CMD_EXTRA=(--recover-envcheck) ;;
  finalize)
    use_xdg_identity
    use_run_dir stage2/before_source_artifact.json stage2/comparison_artifact.json stage2/report.json \
                patches/counterfactual.patch patches/tooling.patch
    MOUNTS+=(-v "$RUN_ABS/stage2":"$RUN_ABS/stage2":ro -v "$RUN_ABS/patches":"$RUN_ABS/patches":ro)
    CF_FILE="$RUN_ABS/patches/counterfactual.patch"; TOOL_FILE="$RUN_ABS/patches/tooling.patch"
    CLAIM_ARGS=(--finalize-run-dir "$RUN_ABS")
    CMD_EXTRA=(--finalize --run-dir "$RUN_ABS" --run-identity "$ID_ABS") ;;
  recover-durability)
    [ -f "$EVIDENCE_ARCHIVE/evidence_manifest.json" ] || {
      echo "ERROR: 找不到成功 archive：$EVIDENCE_ARCHIVE（--recover-durability ⛔ 不接受 failed root）" >&2; exit 1; }
    use_archived_identity "$EVIDENCE_ARCHIVE/identity/run_identity.json.gz"
    CF_FILE="$EVIDENCE_ARCHIVE/patch/counterfactual.patch"; TOOL_FILE="$EVIDENCE_ARCHIVE/patch/tooling.patch"
    CLAIM_ARGS=(--archive)
    CMD_EXTRA=(--recover-durability) ;;
  publish-failed-record)
    use_xdg_identity
    use_run_dir stage2/bounded_diagnostics.json patches/counterfactual.patch patches/tooling.patch
    MOUNTS+=(-v "$RUN_ABS/stage2":"$RUN_ABS/stage2":ro -v "$RUN_ABS/patches":"$RUN_ABS/patches":ro)
    CF_FILE="$RUN_ABS/patches/counterfactual.patch"; TOOL_FILE="$RUN_ABS/patches/tooling.patch"
    CLAIM_ARGS=(--failure-run-dir "$RUN_ABS")
    CMD_EXTRA=(--publish-failed-record --run-dir "$RUN_ABS" --run-identity "$ID_ABS") ;;
  check-failed-record)
    replay_args_require_file "$CF_PATCH" "--counterfactual-patch"
    CF_ABS="$(replay_args_abs_path "$CF_PATCH" "--counterfactual-patch")"
    use_archived_identity "$ENVCHECK_ARCHIVE/identity/run_identity.json.gz"
    CMD_EXTRA=(--check-failed-record) ;;
  recover-failed-record)
    if [ -L "$RECORD_DIR" ] || [ ! -d "$RECORD_DIR" ]; then
      echo "ERROR: --recover-failed-record 的 record 目錄不存在或是 symlink：$RECORD_DIR" >&2; exit 1
    fi
    RECORD_ABS="$(cd "$RECORD_DIR" && pwd)"
    [ "$(dirname "$RECORD_ABS")" = "$FAILED_ROOT" ] || {
      echo "ERROR: --recover-failed-record 只接受 $FAILED_ROOT/ 底下的 record 目錄：$RECORD_ABS" >&2; exit 1; }
    use_archived_identity "$ENVCHECK_ARCHIVE/identity/run_identity.json.gz"
    CF_FILE="$RECORD_ABS/patch/counterfactual.patch"; TOOL_FILE="$RECORD_ABS/patch/tooling.patch"
    CLAIM_ARGS=(--failed-record "$RECORD_ABS")
    CMD_EXTRA=(--recover-failed-record "$RECORD_ABS") ;;
esac

WORKTREE=""
COMPOSE_TREES=()
cleanup() {
  local wt
  for wt in "$WORKTREE" "${COMPOSE_TREES[@]}"; do
    [ -n "$wt" ] || continue
    git -C "$REPO_ROOT" worktree remove --force "$wt" >/dev/null 2>&1 || true
    rm -rf "$wt"
  done
}
trap cleanup EXIT

sha_of() { sha256sum < "$1" | cut -d' ' -f1; }

# 在 <base> 開一個隔離 worktree（⚠️ HEAD 必須等於 base，見 replay_args_prepare_worktree）；結果放在 NEW_TREE。
# ⚠️ ⛔ 不在 $(…) 裡呼叫：subshell 內登記的 worktree 不會回到 cleanup 的清單。
new_compose_tree() {
  NEW_TREE="$(mktemp -d)" || return 1
  rmdir "$NEW_TREE" || return 1
  COMPOSE_TREES+=("$NEW_TREE")
  replay_args_prepare_worktree "$REPO_ROOT" "$1" "$NEW_TREE" >/dev/null || {
    echo "ERROR: 開不了 base $1 的隔離 worktree。" >&2; return 1; }
}

# 合成守門（「四之二」、F8-a）：依 ordered components（counterfactual → tooling）套用兩份 patch，
# 每一步 `git write-tree` 取中繼 tree，重算三個 SHA 並與宣告值比對。
# 用法：compose_check <說明> <base> <cf 檔> <tooling 檔> <cf SHA> <tooling SHA> <composed SHA>
# ⚠️ 失敗一律回 1（⛔ 不讓 git 的 128 之類漏成結束碼）；呼叫端一律 `|| exit 1`。
compose_check() {
  local label="$1" base="$2" src_cf="$3" src_tool="$4" want_cf="$5" want_tool="$6" want_comp="$7"
  local t1 t2 got_cf got_tool got_comp head base_oid snap cf tool
  # ⚠️ 先把兩份 patch **各讀一次**到私有目錄，之後的 SHA 與 `git apply` 都用這份副本——
  # ⛔ 不讓「算 SHA 的那次」與「套用的那次」讀到不同的內容。
  snap="$(mktemp -d)" || return 1
  COMPOSE_TREES+=("$snap")
  cf="$snap/counterfactual.patch"; tool="$snap/tooling.patch"
  cp -- "$src_cf" "$cf" && cp -- "$src_tool" "$tool" || {
    echo "ERROR: 合成守門（$label）：讀不到 patch。" >&2; return 1; }
  if [ "$(sha_of "$cf")" != "$want_cf" ] || [ "$(sha_of "$tool")" != "$want_tool" ]; then
    echo "ERROR: 合成守門（$label）：patch 檔的 bytes 與宣告的 SHA 不符。" >&2; return 1
  fi
  [ -s "$cf" ] || { echo "ERROR: 合成守門（$label）：counterfactual patch ⛔ 不得為 0 bytes。" >&2; return 1; }
  new_compose_tree "$base" || return 1
  git -C "$NEW_TREE" apply --index "$cf" || {
    echo "ERROR: 合成守門（$label）：counterfactual patch 套不上 base $base。" >&2; return 1; }
  t1="$(git -C "$NEW_TREE" write-tree)" || return 1
  if [ -s "$tool" ]; then
    git -C "$NEW_TREE" apply --index "$tool" || {
      echo "ERROR: 合成守門（$label）：tooling patch 套不上 counterfactual 之後的 tree。" >&2; return 1; }
  fi
  t2="$(git -C "$NEW_TREE" write-tree)" || return 1
  got_cf="$(git -C "$NEW_TREE" diff --binary "$base" "$t1" | sha256sum | cut -d' ' -f1)" || return 1
  got_tool="$(git -C "$NEW_TREE" diff --binary "$t1" "$t2" | sha256sum | cut -d' ' -f1)" || return 1
  got_comp="$(git -C "$NEW_TREE" diff --binary "$base" "$t2" | sha256sum | cut -d' ' -f1)" || return 1
  head="$(git -C "$NEW_TREE" rev-parse HEAD)" || return 1
  base_oid="$(git -C "$REPO_ROOT" rev-parse "${base}^{commit}")" || return 1
  [ "$head" = "$base_oid" ] || { echo "ERROR: 合成守門（$label）：HEAD 被動到了（$head）。" >&2; return 1; }
  if [ "$got_cf" != "$want_cf" ]; then
    echo "ERROR: 合成守門（$label）：counterfactual 的重建 SHA $got_cf ≠ 宣告的 $want_cf" \
         "（patch 不是 canonical 的 git diff --binary 輸出）。" >&2; return 1
  fi
  if [ "$got_tool" != "$want_tool" ]; then
    echo "ERROR: 合成守門（$label）：tooling 的重建 SHA $got_tool ≠ 宣告的 $want_tool。" >&2; return 1
  fi
  if [ "$got_comp" != "$want_comp" ]; then
    echo "ERROR: 合成守門（$label）：合成 SHA $got_comp ≠ 宣告的 composed $want_comp" \
         "——⛔ 兩份 patch 合成不出 before 實際跑的那一份。" >&2; return 1
  fi
  return 0
}

if [ "${#CLAIM_ARGS[@]}" -gt 0 ]; then
  # ⚠️ 宣告值由 host 端小工具取出（只讀小檔）；⛔ 取不到就中止、⛔ 不呼叫 Python finalizer。
  CLAIMS="$(python3 "$PYTHON_DIR/scripts/i074-stage2-patch-claims.py" "${CLAIM_ARGS[@]}")" || exit 1
  read -r C_BASE C_CF C_TOOL C_COMP <<< "$CLAIMS"
  compose_check "$MODE" "$C_BASE" "$CF_FILE" "$TOOL_FILE" "$C_CF" "$C_TOOL" "$C_COMP" || exit 1
  # ⚠️ 交接：Python 必須證明它**實際要封存的**就是這一份（見檔頭「合成守門與封存之間的交接」）。
  VERIFY_ARGS=(--verified-patch-base "$C_BASE" --verified-counterfactual-sha256 "$C_CF"
               --verified-tooling-sha256 "$C_TOOL" --verified-composed-sha256 "$C_COMP")
fi

WORKTREE="$(mktemp -d)"
rmdir "$WORKTREE"   # worktree add 要求目標不存在
BASE_COMMIT="$(replay_args_prepare_worktree "$REPO_ROOT" "$SOURCE_REF" "$WORKTREE")"
TOOLING_PATCH_SHA256="$(replay_args_tooling_patch_sha256 "$WORKTREE" "$BASE_COMMIT" "${TOOLING_PATCH:-}")"
RUNNER_SHA256="$(replay_args_runner_sha256 "${BASH_SOURCE[0]}")"

case "$MODE" in
  envcheck|recover-envcheck) PY_MODULE=backtest.modular.sr_scoring.replay_bundle.envcheck ;;
  *) PY_MODULE=backtest.modular.sr_scoring.replay_bundle.stage2_archive ;;
esac
MEM="$(mem_guard_clamp "${MEM:-700m}")"
DOCKER_ARGS=(
  --rm --network none --user "$(id -u):$(id -g)"
  --cpus="${CPUS:-1}" --memory="$MEM" --memory-swap="$MEM"
  -e HOME=/tmp -e PYTHONDONTWRITEBYTECODE=1
  -v "$WORKTREE/python":/app:ro "${MOUNTS[@]}" -w /app
)
CMD=(python -m "$PY_MODULE"
     "${CMD_EXTRA[@]}"
     --python-root "$PYTHON_DIR"
     --image-digest "$IMAGE_ID" --base-commit "$BASE_COMMIT"
     --tooling-patch-sha256 "$TOOLING_PATCH_SHA256"
     --source-root /app --runner-sha256 "$RUNNER_SHA256"
     "${VERIFY_ARGS[@]}")

if [ "${REPLAY_DRY_RUN:-0}" = "1" ]; then
  printf '%s\n' docker run "${DOCKER_ARGS[@]}" "$IMAGE_ID" "${CMD[@]}"
  exit 0
fi

if [ "$MODE" != "check-failed-record" ]; then
  # ⚠️ 前景執行、⛔ 不用 exec：保留 trap 讓 worktree 一定被清掉；原樣傳出 Python 的結束碼。
  set +e
  docker run "${DOCKER_ARGS[@]}" "$IMAGE_ID" "${CMD[@]}"
  RC=$?
  set -e
  exit "$RC"
fi

# ── --check-failed-record：① Python 段 → ② shell 段（F8-a 與本次的 canonical 比對鍵） ────────
# ⚠️ Python 段失敗時 shell 段⛔ 不得執行（失敗點要唯一）；任一段失敗一律 fail-closed（rc=1）。
set +e
CHECK_JSON="$(docker run "${DOCKER_ARGS[@]}" "$IMAGE_ID" "${CMD[@]}")"
RC=$?
set -e
if [ "$RC" -ne 0 ]; then
  echo "ERROR: --check-failed-record 的 Python 段失敗（rc=$RC）——⛔ fail-closed，⛔ 不執行 shell 合成段。" >&2
  exit 1
fi
PARSED="$(python3 -c '
import json, sys
doc = json.loads(sys.stdin.read())
print(doc["base_commit"])
for r in doc["records"]:
    print(r["dir"], r["base_commit"], r["counterfactual_patch_sha256"], r["tooling_patch_sha256"], r["composed_sha256"])
' <<< "$CHECK_JSON")" || { echo "ERROR: 讀不懂 --check-failed-record 的 Python 段輸出——⛔ fail-closed。" >&2; exit 1; }
CHECK_BASE="$(head -n 1 <<< "$PARSED")"
RECORDS="$(tail -n +2 <<< "$PARSED")"

# 本次的比對鍵：⚠️ 與 runner 同一套推導——在 Stage 1 after 的 base 套用輸入 patch，`write-tree` 得 T1，
# 鍵 ＝ sha256(git diff --binary <base> <T1>)。⛔ 不直接雜湊檔案：同一份變更可以有不同的文字表示。
new_compose_tree "$CHECK_BASE" || exit 1
git -C "$NEW_TREE" apply --index "$CF_ABS" || {
  echo "ERROR: --counterfactual-patch 套不上 base $CHECK_BASE——⛔ 在 replay 之前拒絕。" >&2; exit 1; }
T1="$(git -C "$NEW_TREE" write-tree)" || exit 1
CANON_KEY="$(git -C "$NEW_TREE" diff --binary "$CHECK_BASE" "$T1" | sha256sum | cut -d' ' -f1)" || exit 1

HIT=""
while read -r R_DIR R_BASE R_CF R_TOOL R_COMP; do
  [ -n "$R_DIR" ] || continue
  [ "$(dirname "$R_DIR")" = "$FAILED_ROOT" ] || {
    echo "ERROR: Python 段回報的 record 不在 $FAILED_ROOT/ 底下：$R_DIR——⛔ fail-closed。" >&2; exit 1; }
  # F8-a：每份 record 的兩份 patch 都要合成得出它宣告的 composed。
  compose_check "failed record $(basename "$R_DIR")" "$R_BASE" "$R_DIR/patch/counterfactual.patch" \
    "$R_DIR/patch/tooling.patch" "$R_CF" "$R_TOOL" "$R_COMP" || exit 1
  if [ "$R_CF" = "$CANON_KEY" ]; then HIT="$R_DIR"; fi
done <<< "$RECORDS"

# ⚠️ 順序固定：① 先用重建出的 canonical SHA 查找——命中 → 2；② 未命中但輸入 bytes ≠ canonical diff → 1。
if [ -n "$HIT" ]; then
  echo "ERROR: 命中已記錄的失敗：$HIT——同一份反事實 patch ⛔ 不得重跑（改 patch 後才可）。" >&2
  exit 2
fi
if [ "$(sha_of "$CF_ABS")" != "$CANON_KEY" ]; then
  echo "ERROR: --counterfactual-patch ⛔ 不是 canonical 的 git diff --binary 輸出（bytes 的 SHA ≠ 重建的 $CANON_KEY）" \
       "——正式 archive 無條件要求兩者相等，⛔ 在 replay 之前拒絕。" >&2
  exit 1
fi
echo "OK: 沒有命中任何 failed record（比對鍵 $CANON_KEY）。"
exit 0
