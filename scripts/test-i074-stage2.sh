#!/usr/bin/env bash
# I-074 Stage 2 ⑦b：supervisor／orchestrator／label shim 的整合測試（host 端；issue.md I-074「Stage 2 步驟 ⑦b 細部計畫 v1」「五」）。
#
#   scripts/test-i074-stage2.sh        # 由 python/scripts/test.sh 在 SKIP_SHELL_TESTS 那一段呼叫
#
# ⚠️ 隔離（⑦ 總綱 v1「五」）：
#   - ⛔ **絕不碰 `/run/lock`**、⛔ **絕不建立帶正式鍵的真實容器**：正式腳本複製到**合成的最小 repo**，以 sed 改掉常數區塊
#     （鎖檔、sentinel、uid、label 鍵、信任根、base OID），並斷言與正式檔案**只差那幾行**——正式檔案⛔ 沒有執行期覆寫口；
#   - docker、runner、finalizer 都是 fake（記錄每一次呼叫與環境）；git 是轉交真 git 的包裝（可注入情境）；
#   - 結束時斷言 `/run/lock` 的 i074-stage2 項目、真正 repo 的 worktree 登記數都⛔ 沒有變化。
# ⚠️ 測試程序的收尾一律以 pid／process group 為準（⛔ `pkill -f`：會比對到自己的命令列）。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
export PYTHONDONTWRITEBYTECODE=1
fails=0
pass() { echo "  ok   $1"; }
fail() { echo "  FAIL $1" >&2; fails=$((fails + 1)); }
check() { local label="$1"; shift; if "$@"; then pass "$label"; else fail "$label"; fi; }

lock_listing() { ls -1A /run/lock 2>/dev/null | grep -F i074-stage2 || true; }
LOCK_BEFORE="$(lock_listing)"
WT_BEFORE="$(git -C "$REPO_ROOT" worktree list --porcelain | grep -c '^worktree ' || true)"

TD="$(mktemp -d)"
chmod 700 "$TD"
PIDS=()
cleanup() {
  local p
  for p in "${PIDS[@]:-}"; do
    [ -n "$p" ] || continue
    kill -KILL -- "-$p" 2>/dev/null || kill -KILL "$p" 2>/dev/null || true
  done
  for p in $(cat "$TD"/*/orphan.pid "$TD"/*/runner.pid 2>/dev/null || true); do kill -KILL "$p" 2>/dev/null || true; done
  rm -rf "$TD"
}
trap cleanup EXIT

LABEL=i074.stage2.testrun
LOCKDIR="$TD/lock"
mkdir -m 700 "$LOCKDIR"
TRUSTED="$TD/trusted"
mkdir -m 755 "$TRUSTED"
TEST_PATH="$TRUSTED:/usr/bin:/bin"
ME="$(id -u)"
IMG="sha256:$(printf 'i074-stage2-test-image' | sha256sum | cut -d' ' -f1)"
IMG2="sha256:$(printf 'another-image' | sha256sum | cut -d' ' -f1)"
XDG="$TD/xdg"
IDENTITY="$XDG/stock_trading/i074_stage2/run_identity.json"
DOCKER_ROOT="$TD/dockerroot"
mkdir -p "$DOCKER_ROOT" "$(dirname "$IDENTITY")"

# ── fake docker（以 sed 改過的 DOCKER 常數指向它；記錄每一次呼叫） ──────────────────
cat > "$TRUSTED/docker" <<'FAKEDOCKER'
#!/bin/bash
S="${I074_TEST_SCENARIO:?}"; D="$S/docker"
mkdir -p "$D/containers"
printf '%s\n' "$*" >> "$D/calls.log"
labels_of() {  # 印出 argv 裡每一個 --label 的值
  local prev=""
  for a in "$@"; do [ "$prev" = --label ] && printf '%s\n' "$a"; prev="$a"; done
}
sub="$1"
[ "$sub" = container ] && { shift; sub="$1"; }
case "$sub" in
  image)
    [ ! -f "$S/docker_inspect_fail" ] || exit 1
    cat "$S/docker_labels" 2>/dev/null || echo null ;;
  ps)
    [ ! -f "$S/docker_ps_fail" ] || exit 1
    filter=""; prev=""
    for a in "$@"; do [ "$prev" = --filter ] && filter="${a#label=}"; prev="$a"; done
    for c in "$D"/containers/*; do
      [ -f "$c" ] || continue
      case "$filter" in
        *=*) grep -qxF -- "$filter" "$c" && basename "$c" ;;
        *) grep -q "^$filter=" "$c" && basename "$c" ;;
      esac
    done
    exit 0 ;;
  rm)
    shift; [ "${1:-}" = -f ] && shift
    [ ! -f "$S/docker_rm_fail" ] || exit 1
    for id in "$@"; do rm -f "$D/containers/$id"; done ;;
  info)
    [ ! -f "$S/docker_info_fail" ] || exit 1
    cat "$S/docker_root" ;;
  run|create)
    n="$(ls "$D" | grep -c '^run\.' || true)"
    printf '%s\n' "$*" > "$D/run.$n"
    if [ "$sub" = create ]; then labels_of "$@" > "$D/containers/c$n"; fi
    case "$*" in
      *i074_stage2_preflight.py\ anchors*)
        [ ! -f "$S/anchors_fail" ] || { echo "fake anchors：拒絕" >&2; exit 1; }
        [ ! -f "$S/anchors_hook" ] || bash "$S/anchors_hook"
        cat "$S/anchors.json" ;;
    esac ;;
  *) : ;;
esac
FAKEDOCKER

# ── git 包裝（sed 過的 GIT 常數與 TRUSTED_PATH 指向這裡；其餘轉交真的 git） ─────────────
cat > "$TRUSTED/git" <<'FAKEGIT'
#!/bin/bash
S="${I074_TEST_SCENARIO:-}"
[ -z "$S" ] || printf '%s\n' "$*" >> "$S/git.log"
if [ -n "$S" ] && [ -f "$S/git_fail_worktree_remove" ]; then
  case " $* " in *" worktree remove "*) echo "fake git：worktree remove 失敗" >&2; exit 1 ;; esac
fi
/usr/bin/git "$@"; rc=$?
if [ "$rc" = 0 ] && [ -n "$S" ] && [ -f "$S/git_after_checkout" ]; then
  case " $* " in *" checkout -q --detach "*) clone=""; prev=""
    for a in "$@"; do [ "$prev" = -C ] && clone="$a"; prev="$a"; done
    bash "$S/git_after_checkout" "$clone" ;;
  esac
fi
exit "$rc"
FAKEGIT
chmod 755 "$TRUSTED/docker" "$TRUSTED/git"

# ── 合成的「真正 repo」 ─────────────────────────────────────────────────────────────
# shellcheck source=lib/replay-args.sh
. "$REPO_ROOT/scripts/lib/replay-args.sh"
SR="$TD/real"
SRCDIR=python/backtest/modular/sr_scoring
git init -q "$SR"
git -C "$SR" config user.name t; git -C "$SR" config user.email t@t; git -C "$SR" config commit.gpgsign false
for f in decision_engine.py lifecycle_engine.py tests/test_i074_diagnostics.py tests/test_lifecycle_engine.py evaluation.py; do
  mkdir -p "$(dirname "$SR/$SRCDIR/$f")"
  printf 'base %s\n' "$f" > "$SR/$SRCDIR/$f"
done
git -C "$SR" add -A && git -C "$SR" commit -qm base
BASE_OID="$(git -C "$SR" rev-parse HEAD)"
BASE_TREE="$(git -C "$SR" rev-parse 'HEAD^{tree}')"
WT1="$TD/wt1"
git -C "$SR" worktree add -q --detach "$WT1" "$BASE_OID"
for f in decision_engine.py lifecycle_engine.py tests/test_i074_diagnostics.py tests/test_lifecycle_engine.py; do
  printf 'counterfactual %s\n' "$f" >> "$WT1/$SRCDIR/$f"
done
git -C "$WT1" add -A; T1="$(git -C "$WT1" write-tree)"
printf 'tooling\n' >> "$WT1/$SRCDIR/evaluation.py"
git -C "$WT1" add -A; T2="$(git -C "$WT1" write-tree)"
git -C "$SR" worktree remove --force "$WT1"
mkdir -p "$SR/python/baselines/i074_stage2" "$SR/python/baselines/b1_test" "$SR/scripts/lib" "$SR/python/scripts" \
         "$SR/$SRCDIR/replay_bundle"
replay_args_canonical_diff "$SR" "$BASE_TREE" "$T1" > "$SR/python/baselines/i074_stage2/counterfactual_e1cbbbd.patch"
replay_args_canonical_diff "$SR" "$T1" "$T2" > "$SR/python/baselines/i074_stage2/tooling_e1cbbbd.patch"
printf 'bundle\n' > "$SR/python/baselines/b1_test/manifest.json"
# ⑧ 補齊（v29 n9）：Stage 1 錨點與 envcheck/ 的佔位——fake 流程⛔ 不讀它們；n9 的 (ii)(iii) 改它們，證明⛔ 不影響 ⑩。
mkdir -p "$SR/python/baselines/i074_stage1" "$SR/python/baselines/i074_stage2/envcheck"
printf '{"placeholder":"stage1"}\n' > "$SR/python/baselines/i074_stage1/evidence_manifest.json"
printf '{"placeholder":"envcheck"}\n' > "$SR/python/baselines/i074_stage2/envcheck/evidence_manifest.json"
printf 'sizing harness\n' > "$SR/scripts/i074-stage2-sizing.sh"
printf 'sizing shim\n' > "$SR/scripts/lib/i074-sizing-docker-shim.sh"
printf 'sizing helper\n' > "$SR/python/scripts/i074_stage2_sizing.py"
cp "$REPO_ROOT/scripts/lib/replay-args.sh" "$REPO_ROOT/scripts/lib/mem-guard.sh" "$SR/scripts/lib/"
cp "$REPO_ROOT/python/scripts/_i074_bootstrap.py" "$REPO_ROOT/python/scripts/i074_stage2_preflight.py" "$SR/python/scripts/"
cp "$REPO_ROOT/$SRCDIR/replay_bundle/canonical.py" "$REPO_ROOT/$SRCDIR/replay_bundle/publish.py" "$SR/$SRCDIR/replay_bundle/"
cp "$REPO_ROOT/python/scripts/i074_stage2_promote.py" "$SR/python/scripts/"
# ⑦c：晉升的 staging 規則（與正式 repo 的 .gitignore 同一行）——晉升的 ignore 守門以真 git 查它。
grep -x 'python/baselines/i074_stage2/.promote-staging-\*' "$REPO_ROOT/.gitignore" > "$SR/.gitignore"

# 正式腳本 → sed 改常數（⚠️ 只改這些行；下方斷言 diff 只落在這些行）
sedcopy() {  # $1＝正式檔（repo 相對）；$2＝目的地；其餘＝sed 表達式
  local src="$1" dst="$2"
  shift 2
  sed "$@" "$REPO_ROOT/$src" > "$dst"
  chmod --reference="$REPO_ROOT/$src" "$dst"
}
SED_ORCH=(-e "s|^I074_LOCK_PATH=.*|I074_LOCK_PATH=$LOCKDIR/i074-stage2.lock|"
          -e "s|^I074_SENTINEL_PATH=.*|I074_SENTINEL_PATH=$LOCKDIR/i074-stage2.active|"
          -e "s|^I074_LABEL_KEY=.*|I074_LABEL_KEY=$LABEL|"
          -e "s|^I074_TRUSTED_PATH=.*|I074_TRUSTED_PATH=$TEST_PATH|"
          -e "s|^I074_GIT=.*|I074_GIT=$TRUSTED/git|")
SED_SUP=(-e "s|^LOCK_PATH = .*|LOCK_PATH = \"$LOCKDIR/i074-stage2.lock\"|"
         -e "s|^SENTINEL_PATH = .*|SENTINEL_PATH = \"$LOCKDIR/i074-stage2.active\"|"
         -e "s|^RUN_UID = .*|RUN_UID = $ME|"
         -e "s|^LABEL_KEY = .*|LABEL_KEY = \"$LABEL\"|"
         -e "s|^TRUSTED_PATH = .*|TRUSTED_PATH = \"$TEST_PATH\"|"
         -e "s|^GIT = .*|GIT = \"$TRUSTED/git\"|"
         -e "s|^DOCKER = .*|DOCKER = \"$TRUSTED/docker\"|"
         -e "s|^TRUSTED_OWNER_UID = .*|TRUSTED_OWNER_UID = $ME|")
SED_SHIM=(-e "s|^I074_LABEL_KEY=.*|I074_LABEL_KEY=$LABEL|"
          -e "s|^I074_SENTINEL_PATH=.*|I074_SENTINEL_PATH=$LOCKDIR/i074-stage2.active|"
          -e "s|^I074_DOCKER=.*|I074_DOCKER=$TRUSTED/docker|"
          -e "s|^I074_TRUSTED_OWNER_UID=.*|I074_TRUSTED_OWNER_UID=$ME|")
SED_FR=(-e "s|^BASE_COMMIT = .*|BASE_COMMIT = \"$BASE_OID\"|")
sedcopy scripts/run-i074-stage2.sh "$SR/scripts/run-i074-stage2.sh" "${SED_ORCH[@]}"
sedcopy scripts/lib/i074-stage2-supervisor.py "$SR/scripts/lib/i074-stage2-supervisor.py" "${SED_SUP[@]}"
sedcopy scripts/lib/i074-stage2-docker-label-shim.sh "$SR/scripts/lib/i074-stage2-docker-label-shim.sh" "${SED_SHIM[@]}"
sedcopy python/scripts/i074_stage2_freeze_record.py "$SR/python/scripts/i074_stage2_freeze_record.py" "${SED_FR[@]}"
SED_JUDGE=(-e "s|^I074_TRUSTED_PATH=.*|I074_TRUSTED_PATH=$TEST_PATH|" -e "s|^I074_GIT=.*|I074_GIT=$TRUSTED/git|")
sedcopy scripts/judge-i074-stage2.sh "$SR/scripts/judge-i074-stage2.sh" "${SED_JUDGE[@]}"

# fake runner（記錄 argv 與環境；依情境產出 operational 輸出、停在 barrier、改動複本）
cat > "$SR/scripts/run-replay-offline.sh" <<'FAKERUNNER'
#!/usr/bin/env bash
S="${I074_TEST_SCENARIO:?}"
mkdir -p "$S/spy"
{ printf 'ARGV'; printf ' %s' "$@"; echo; env | sort | sed 's/^/ENV /'
  for v in COUNTERFACTUAL_PATCH TOOLING_PATCH; do [ -f "${!v:-/nonexistent}" ] && echo "SHA $v $(sha256sum < "${!v}" | cut -d' ' -f1)"; done
} >> "$S/spy/runner.log"
echo "=== runner ===" >> "$S/git.log"
# ⚠️ 像真的 runner 一樣在 TMPDIR 留下一個 worktree（`exec docker run` 不會跑 EXIT trap）——orchestrator ⛔ 不得清它。
_clone="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
_wt="$(mktemp -d)" && rmdir "$_wt" && git -C "$_clone" worktree add -q --detach "$_wt" HEAD && echo "$_wt" > "$S/runner_worktree"
out=""; prev=""
for a in "$@"; do [ "$prev" = --output-dir ] && out="$a"; prev="$a"; done
if [ -f "$S/runner_orphan" ]; then setsid sleep 60 </dev/null >/dev/null 2>&1 & echo $! > "$S/orphan.pid"; fi
if [ -p "$S/barrier" ]; then
  echo $$ > "$S/runner.pid"
  echo "$PPID" > "$S/orch.pid"          # 父程序 ＝ 複本內的 orchestrator（run_step 的背景工作直接 exec 到這裡）
  [ ! -f "$S/runner_ignore_term" ] || trap '' TERM
  [ ! -f "$S/runner_create_container" ] || docker create --name keep "sha256:0000" true >/dev/null
  touch "$S/at_barrier"
  read -r _ < "$S/barrier"
  if [ -f "$S/runner_after_barrier" ]; then bash "$S/runner_after_barrier"; exit 0; fi
fi
rc="$(cat "$S/runner_rc" 2>/dev/null || echo 0)"
shape="$(cat "$S/runner_shape" 2>/dev/null || echo auto)"
mkdir -p "$out"
case "$rc:$shape" in
  0:auto) for f in before_source_artifact.json comparison_artifact.json report.json; do echo "$f" > "$out/$f"; done ;;
  0:extra) for f in before_source_artifact.json comparison_artifact.json report.json extra.json; do echo "$f" > "$out/$f"; done ;;
  0:missing) for f in before_source_artifact.json comparison_artifact.json; do echo "$f" > "$out/$f"; done ;;
  6:auto) echo diag > "$out/bounded_diagnostics.json" ;;
  6:both) echo diag > "$out/bounded_diagnostics.json"; echo c > "$out/comparison_artifact.json" ;;
esac
[ ! -f "$S/runner_hook" ] || bash "$S/runner_hook"
exit "$rc"
FAKERUNNER

# fake finalizer（check／finalize／publish 的結果依情境；終態建在複本裡）。
# ⚠️ ⑦c：終態帶**真的** identity（evidence：XDG identity 的 gzip；failed：record 內嵌它）——晉升的第 3c 步以它綁定；
#    `--verify-promotion-staging` 回報**它讀到的** manifest／record 的 SHA（晉升以它綁定經 fd 寫入的那一份）。
cat > "$SR/scripts/finalize-stage2-evidence.sh" <<'FAKEFIN'
#!/usr/bin/env bash
S="${I074_TEST_SCENARIO:?}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
mkdir -p "$S/spy"
{ printf 'ARGV'; printf ' %s' "$@"; echo; env | sort | sed 's/^/ENV /'; } >> "$S/spy/finalizer.log"
IDF="${XDG_DATA_HOME:-$HOME/.local/share}/stock_trading/i074_stage2/run_identity.json"
B="$REPO_ROOT/python/baselines/i074_stage2"
case "$1" in
  --check-failed-record) exit "$(cat "$S/check_rc" 2>/dev/null || echo 0)" ;;
  --finalize|--publish-failed-record)
    n="$(( $(cat "$S/fin_count" 2>/dev/null || echo 0) + 1 ))"; echo "$n" > "$S/fin_count"
    plan="$(sed -n "${n}p" "$S/fin_plan" 2>/dev/null)"; [ -n "$plan" ] || plan="0 yes"
    rc="${plan% *}"; term="${plan#* }"
    if [ "$term" = yes ]; then
      if [ "$1" = --finalize ]; then
        d="$B/evidence"; mkdir -p "$d/identity" "$d/patch"
        printf '{"finalizer_provenance":{"base_commit":"%s"},"n":%s}\n' "$(git -C "$REPO_ROOT" rev-parse HEAD)" "$n" > "$d/evidence_manifest.json"
        gzip -n -c "$IDF" > "$d/identity/run_identity.json.gz"
        cp "$B/counterfactual_e1cbbbd.patch" "$d/patch/counterfactual.patch"
      else
        work="$(cd "$2/.." 2>/dev/null && pwd)"; [ -n "$work" ] || work="$(cd "$3/.." && pwd)"
        name="$(python3 -c 'import json,sys; p=json.load(open(sys.argv[1])); print(p["bundle_id"]+"-"+p["counterfactual_semantic_sha256"])' "$work/state/preflight.json")"
        d="$B/failed/$name"; mkdir -p "$d/patch"
        python3 -c 'import json,sys; json.dump({"run_identity": json.load(open(sys.argv[1])), "x": 1}, open(sys.argv[2], "w"))' "$IDF" "$d/failure_record.json"
        cp "$B/counterfactual_e1cbbbd.patch" "$d/patch/counterfactual.patch"
      fi
    fi
    exit "$rc" ;;
  --verify-promotion-staging)
    path="$2"; target="$4"; judge="${5:-}"
    printf 'VERIFY %s %s %s\n' "$path" "$target" "$judge" >> "$S/spy/verify.log"
    [ ! -f "$S/verify_hook" ] || bash "$S/verify_hook" "$path" || exit 1
    rc="$(cat "$S/verify_rc" 2>/dev/null || echo 0)"; [ "$rc" = 0 ] || exit "$rc"
    [ ! -f "$S/verify_needs" ] || [ -e "$REPO_ROOT/$(cat "$S/verify_needs")" ] || { echo "fake verify：缺錨點" >&2; exit 1; }
    if [ "$target" = evidence ]; then kind=evidence; key=manifest_sha256; f="$path/evidence_manifest.json"
    else kind=failed_record; key=record_sha256; f="$path/failure_record.json"; fi
    extra=""
    [ "$judge" != --judge ] || extra=',"verdict":{"schema":"i074_stage2_verdict/v1","verdict":"B"}'
    printf '{"base_commit":"%s","identity_sha256":"%s","kind":"%s","%s":"%s","target":"%s"%s}\n' \
      "$(git -C "$REPO_ROOT" rev-parse HEAD)" "$(sha256sum < "$IDF" | cut -d' ' -f1)" "$kind" "$key" \
      "$(sha256sum < "$f" | cut -d' ' -f1)" "$target" "$extra"
    exit 0 ;;
esac
exit 99
FAKEFIN
chmod 755 "$SR/scripts/run-replay-offline.sh" "$SR/scripts/finalize-stage2-evidence.sh"
git -C "$SR" add -A && git -C "$SR" commit -qm "i074 stage2 synthetic"
HEAD_OID="$(git -C "$SR" rev-parse HEAD)"

# identity（XDG）與 freeze record ＋ 報告（以正式的 build 寫出——同時驗寫入端）
write_identity() {  # $1＝created_at；$2＝image
  python3 -c '
import sys
sys.path.insert(0, sys.argv[1] + "/python/scripts")
from _i074_bootstrap import load_replay_bundle
c = load_replay_bundle(__import__("pathlib").Path(sys.argv[1]) / "python", ("canonical",))["canonical"]
payload = {"schema_version": 1, "kind": "sr_zone_run_identity", "bundle_id": "b1_test",
           "expected_image_id": sys.argv[4], "created_at": sys.argv[3]}
open(sys.argv[2], "wb").write(c.canonical_json_bytes(payload))' "$SR" "$IDENTITY" "$1" "$2"
}
write_identity "2026-09-30T00:00:00+00:00" "$IMG"
make_freeze() {  # $1＝輸出目錄；$2＝repo_head；$3（可選）＝tooling raw SHA 的覆寫
  local out="$1" head="$2" cf tool
  mkdir -p "$out"
  cf="$(sha256sum < "$SR/python/baselines/i074_stage2/counterfactual_e1cbbbd.patch" | cut -d' ' -f1)"
  tool="${3:-$(sha256sum < "$SR/python/baselines/i074_stage2/tooling_e1cbbbd.patch" | cut -d' ' -f1)}"
  python3 -c '
import hashlib, sys
sys.path.insert(0, sys.argv[1] + "/python/scripts")
from _i074_bootstrap import load_replay_bundle
from pathlib import Path
c = load_replay_bundle(Path(sys.argv[1]) / "python", ("canonical",))["canonical"]
sr, out, head, base, image, identity, cf, tool = sys.argv[1:9]
def fsha(rel): return hashlib.sha256(__import__("subprocess").run(["git", "-C", sr, "cat-file", "blob", head + ":" + rel], stdout=-1, check=True).stdout).hexdigest()
meta = {"run_id": "20260930T000000Z-1", "mode": "formal", "repo_head": head, "clone_head": head, "base_commit": base,
        "harness_sha256": fsha("scripts/i074-stage2-sizing.sh"), "shim_sha256": fsha("scripts/lib/i074-sizing-docker-shim.sh"),
        "helper_sha256": fsha("python/scripts/i074_stage2_sizing.py"), "image": image,
        "identity_sha256": hashlib.sha256(open(identity, "rb").read()).hexdigest(),
        "counterfactual_sha256": cf, "counterfactual_canonical_sha256": cf, "tooling_sha256": tool, "tooling_canonical_sha256": tool}
report = {"schema": "i074_stage2_sizing_report_v1", "mode": "formal", "status": "ok", "P_B": 1000, "meta": meta}
open(out + "/sizing_report.json", "wb").write(c.canonical_json_bytes(report))' "$SR" "$out" "$head" "$BASE_OID" "$IMG" "$IDENTITY" "$cf" "$tool"
  python3 "$SR/python/scripts/i074_stage2_freeze_record.py" build --report "$out/sizing_report.json" --repo "$SR" \
    --identity "$IDENTITY" --out "$out/freeze_record.json" 2>"$out/build.err"
}
FR_DIR="$TD/freeze"
make_freeze "$FR_DIR" "$HEAD_OID"
FR="$FR_DIR/freeze_record.json"
if [ -f "$FR" ]; then pass "寫入端：formal ＋ ok ＋ P_B ≤ 預算 → build 寫出 freeze record（host 3.9）"; else
  fail "build 沒有寫出 freeze record"; exit 1; fi

# ── 情境 ─────────────────────────────────────────────────────────────────────────
NS=0
clean_real() {  # 清掉晉升進真正 repo 的終態與 orphan staging（⛔ 不動已追蹤的檔）
  rm -rf "$SR/python/baselines/i074_stage2/evidence" "$SR/python/baselines/i074_stage2"/.promote-staging-*
  git -C "$SR" clean -fdq -- python/baselines/i074_stage2/failed/ >/dev/null 2>&1 || true
}
new_scenario() {  # → 設 S；預設：anchors 回合成的 bundle／base、docker root 在同一個檔案系統（KEEP_REAL=1：⛔ 不清真正 repo）
  [ "${KEEP_REAL:-0}" = 1 ] || clean_real
  NS=$((NS + 1))
  S="$TD/sc$NS"
  mkdir -p "$S/docker/containers" "$S/spy"
  printf '%s\n' "$DOCKER_ROOT" > "$S/docker_root"
  printf '{"after_artifact":"python/baselines/i074_stage1/d1/after_artifact.json.gz","after_base_commit":"%s","bundle_id":"b1_test","cohort_manifest":"python/baselines/i074_stage1/d1/cohort_manifest.json.gz"}\n' \
    "$BASE_OID" > "$S/anchors.json"
  : > "$S/git.log"
}
# 以正式命令（直接執行）跑一次。$1＝repo；其餘＝參數。環境只帶這幾個（外加 EXTRA_ENV）。
RC=0
run_entry() {
  local repo="$1"
  shift
  RC=0
  env -i HOME="$HOME" PATH="${CALLER_PATH:-$TEST_PATH}" XDG_DATA_HOME="$XDG" REPLAY_IMAGE_ID="${IMAGE_OVERRIDE-$IMG}" \
    I074_TEST_SCENARIO="$S" ${EXTRA_ENV:-} "$repo/scripts/run-i074-stage2.sh" "$@" > "$S/out" 2> "$S/err" || RC=$?
}
lock_free() { python3 -c 'import fcntl,os,sys; fd=os.open(sys.argv[1], os.O_RDWR); fcntl.flock(fd, fcntl.LOCK_EX|fcntl.LOCK_NB)' \
                "$LOCKDIR/i074-stage2.lock" 2>/dev/null; }
no_sentinel() { [ ! -e "$LOCKDIR/i074-stage2.active" ]; }
runner_called() { [ -s "$S/spy/runner.log" ]; }
finalizer_called() { grep -q -- "$1" "$S/spy/finalizer.log" 2>/dev/null; }
docker_ran() { ls "$S/docker" | grep -q '^run\.'; }
expect_rc() { [ "$RC" = "$1" ] || { echo "    （rc=$RC，預期 $1）" >&2; tail -5 "$S/err" >&2; return 1; }; }
wait_for() { local i; for i in $(seq 1 200); do [ -e "$1" ] && return 0; sleep 0.1; done; return 1; }
reset_sentinel() { rm -f "$LOCKDIR/i074-stage2.active"; }   # 模擬重開機（隔離的鎖目錄）
promoted_ok() {  # $1＝執行目錄；$2＝evidence 或 failed/<name> → 真正 repo 的目的地 ≡ 複本的終態、⛔ 沒有 staging
  diff -r "$1/repo/python/baselines/i074_stage2/$2" "$SR/python/baselines/i074_stage2/$2" >/dev/null \
    && ! ls -d "$SR/python/baselines/i074_stage2"/.promote-staging-* >/dev/null 2>&1
}
failed_name() { python3 -c 'import json,sys; p=json.load(open(sys.argv[1])); print("failed/"+p["bundle_id"]+"-"+p["counterfactual_semantic_sha256"])' "$1/state/preflight.json"; }

echo "==> i074 Stage 2 ⑦b：靜態檢查"
only_const_diff() {  # $1＝正式檔；$2＝副本；$3＝允許差異的行首正規式；$4＝預期差異行數
  local d n
  d="$(diff "$REPO_ROOT/$1" "$2" | grep '^[<>]' || true)"
  n="$(grep -c . <<< "$d" || true)"
  [ "$n" = "$4" ] && ! grep -vqE "^[<>] ($3)" <<< "$d"
}
check "sed 副本只差常數行：run-i074-stage2.sh" only_const_diff scripts/run-i074-stage2.sh "$SR/scripts/run-i074-stage2.sh" \
  'I074_(LOCK_PATH|SENTINEL_PATH|LABEL_KEY|TRUSTED_PATH|GIT)=' 10
check "sed 副本只差常數行：supervisor" only_const_diff scripts/lib/i074-stage2-supervisor.py "$SR/scripts/lib/i074-stage2-supervisor.py" \
  '(LOCK_PATH|SENTINEL_PATH|RUN_UID|LABEL_KEY|TRUSTED_PATH|GIT|DOCKER|TRUSTED_OWNER_UID) = ' "$( [ "$ME" = 1001 ] && echo 14 || echo 16)"
check "sed 副本只差常數行：label shim" only_const_diff scripts/lib/i074-stage2-docker-label-shim.sh "$SR/scripts/lib/i074-stage2-docker-label-shim.sh" \
  'I074_(LABEL_KEY|SENTINEL_PATH|DOCKER|TRUSTED_OWNER_UID)=' 8
check "sed 副本只差常數行：freeze record 的 base OID" only_const_diff python/scripts/i074_stage2_freeze_record.py \
  "$SR/python/scripts/i074_stage2_freeze_record.py" 'BASE_COMMIT = ' 2
check "sed 副本只差常數行：判讀器（⑦c）" only_const_diff scripts/judge-i074-stage2.sh "$SR/scripts/judge-i074-stage2.sh" \
  'I074_(TRUSTED_PATH|GIT)=' 4
check "shebang：run-i074-stage2.sh、label shim 與判讀器的第一行恰好是 #!/bin/bash -p" \
  bash -c 'for f in "$@"; do [ "$(head -n1 "$f")" = "#!/bin/bash -p" ] || exit 1; done' _ \
  "$REPO_ROOT/scripts/run-i074-stage2.sh" "$REPO_ROOT/scripts/lib/i074-stage2-docker-label-shim.sh" \
  "$REPO_ROOT/scripts/judge-i074-stage2.sh"
static_rules() {
  python3 - "$REPO_ROOT" <<'PY'
import re, sys
root = sys.argv[1]
files = ["scripts/run-i074-stage2.sh", "scripts/run-replay-offline.sh", "scripts/finalize-stage2-evidence.sh",
         "scripts/lib/replay-args.sh", "scripts/lib/mem-guard.sh", "scripts/judge-i074-stage2.sh"]
entries = ("scripts/run-i074-stage2.sh", "scripts/judge-i074-stage2.sh")   # 只有這兩個入口可以（恰好一次）指派 PATH
cmd_docker = re.compile(r'(^|[;&|(]|\b(?:then|do|else|exec|!)\s)\s*("?\$\{?\w*DOCKER\w*\}?"?|\S*/docker)(\s|$)')
bad = []
path_assign = []
for rel in files:
    for n, line in enumerate(open(f"{root}/{rel}", encoding="utf-8"), 1):
        code = line.split("#", 1)[0] if not line.lstrip().startswith("#") else ""
        if not code.strip():
            continue
        if cmd_docker.search(code):
            bad.append(f"{rel}:{n} 以絕對路徑或變數呼叫 docker：{line.strip()}")
        if "command -p" in code:
            bad.append(f"{rel}:{n} command -p")
        for m in re.finditer(r'(?<![\w$])PATH=(\S*)', code):
            path_assign.append((rel, n, m.group(1)))
        if re.search(r'\bexport\s+PATH\b', code):
            path_assign.append((rel, n, "export"))
allowed = {("scripts/lib/replay-args.sh", '"$PATH"')}
for entry in entries:
    mine = [(r, n, v) for r, n, v in path_assign if r == entry]
    if sorted(v for _r, _n, v in mine) != sorted(['"$I074_TRUSTED_PATH"', "export"]):
        bad.append(f"{entry} 的 PATH 指派不是恰好一處常數：{mine}")
for r, n, v in path_assign:
    if r not in entries and (r, v) not in allowed:
        bad.append(f"{r}:{n} 改寫 PATH：{v}")
for b in bad:
    print(b, file=sys.stderr)
sys.exit(1 if bad else 0)
PY
}
check "⑩ 呼叫圖：⛔ 不以絕對路徑／變數呼叫 docker、⛔ 沒有 command -p、PATH 只在兩個入口各指派一次常數" static_rules
mirror_constants() {
  python3 - "$REPO_ROOT" <<'PY'
import re, sys
root = sys.argv[1]
sys.path.insert(0, f"{root}/python/scripts")
from _i074_bootstrap import STAGE2_ARCHIVE_MODULES, load_replay_bundle
from pathlib import Path
mods = load_replay_bundle(Path(root) / "python", STAGE2_ARCHIVE_MODULES)
sa = mods["stage2_archive"]
orch = open(f"{root}/scripts/run-i074-stage2.sh", encoding="utf-8").read()
sup = open(f"{root}/scripts/lib/i074-stage2-supervisor.py", encoding="utf-8").read()
shim = open(f"{root}/scripts/lib/i074-stage2-docker-label-shim.sh", encoding="utf-8").read()
tp = open(f"{root}/scripts/make-i074-tooling-patch.sh", encoding="utf-8").read()
fr = open(f"{root}/python/scripts/i074_stage2_freeze_record.py", encoding="utf-8").read()
judge = open(f"{root}/scripts/judge-i074-stage2.sh", encoding="utf-8").read()
import i074_stage2_preflight as pf
import i074_stage2_promote as pm
pub = mods["publish"]
def sh(name, text=orch): return re.search(rf"^{name}=(\S+)", text, re.M).group(1)
def py(name): return re.search(rf'^{name} = "?([^"\n]+)"?', sup, re.M).group(1)
problems = []
codes = dict(re.findall(r"(EXIT_\w+)=(\d+)", re.search(r"^readonly EXIT_ABORT=.*$", orch, re.M).group(0)))
want = {"EXIT_ABORT": "1", "EXIT_LOOKUP_HIT": "2", "EXIT_PROMOTION_FAILED": "8", "EXIT_PROMOTION_BLOCKED": "9"}
if codes != want: problems.append(f"orchestrator 結束碼 {codes}")
for k, v in want.items():
    if py(k) != v: problems.append(f"supervisor {k}={py(k)}")
# ⑦c：8、9 的唯一定義在 replay_bundle/publish.py；晉升模組取自它，orchestrator 與 supervisor 鏡像。
if (pub.EXIT_PROMOTION_FAILED, pub.EXIT_PROMOTION_BLOCKED) != (8, 9):
    problems.append("publish.py 的 8／9 不是 8、9")
if (pm.EXIT_PROMOTION_FAILED, pm.EXIT_PROMOTION_BLOCKED, pm.EXIT_DURABILITY_UNCONFIRMED, pm.EXIT_FAILED_RECORD) != (8, 9, 3, 6):
    problems.append("晉升模組的結束碼 ≠ publish.py")
for s in ("I074_TRUSTED_PATH", "I074_GIT", "I074_PYTHON"):
    if sh(s, judge) != sh(s): problems.append(f"判讀器 {s} ≠ orchestrator")
if pm.GIT != sh("I074_GIT"): problems.append("晉升模組的 GIT ≠ orchestrator 的 I074_GIT")
for s, p in (("I074_LOCK_PATH", "LOCK_PATH"), ("I074_SENTINEL_PATH", "SENTINEL_PATH"), ("I074_LABEL_KEY", "LABEL_KEY"),
             ("I074_TRUSTED_PATH", "TRUSTED_PATH"), ("I074_GIT", "GIT"), ("I074_PYTHON", "PYTHON"), ("I074_BASH", "BASH")):
    if sh(s) != py(p): problems.append(f"{s} ≠ supervisor {p}")
for s, p in (("I074_LABEL_KEY", "LABEL_KEY"), ("I074_SENTINEL_PATH", "SENTINEL_PATH"), ("I074_DOCKER", "DOCKER"),
             ("I074_TRUSTED_OWNER_UID", "TRUSTED_OWNER_UID")):
    if sh(s, shim) != py(p): problems.append(f"shim {s} ≠ supervisor {p}")
if sh("readonly FROZEN_CF") != sa.FROZEN_COUNTERFACTUAL_PATCH or sh("readonly FROZEN_TOOLING") != sa.FROZEN_TOOLING_PATCH:
    problems.append("FROZEN_* ≠ stage2_archive")
ops = tuple(Path(x).name for x in (sa.OPERATIONAL_BEFORE_SOURCE, sa.OPERATIONAL_COMPARISON, sa.OPERATIONAL_REPORT))
if tuple(sorted(pf.OPERATIONAL_SUCCESS)) != tuple(sorted(ops)) or pf.OPERATIONAL_FAILURE != (Path(sa.OPERATIONAL_FAILURE).name,):
    problems.append("preflight 的 OPERATIONAL_* ≠ stage2_archive")
if {Path(x).parent.name for x in (sa.OPERATIONAL_BEFORE_SOURCE, sa.OPERATIONAL_FAILURE)} != {"stage2"}:
    problems.append("operational 輸出不在 stage2/ 底下")
if re.search(r'^BASE_COMMIT = "(\w+)"', fr, re.M).group(1) != re.search(r"^I074_TOOLING_BASE=(\w+)", tp, re.M).group(1):
    problems.append("freeze record 的 BASE_COMMIT ≠ make-i074-tooling-patch.sh 的 I074_TOOLING_BASE")
for p in problems:
    print(p, file=sys.stderr)
sys.exit(1 if problems else 0)
PY
}
check "鏡像常數：結束碼（1／2／8／9；⑦c 的 8、9 ＝ publish.py ＝ 晉升模組）、信任根（含判讀器與晉升模組）、鎖與 label、FROZEN_*／OPERATIONAL_*、base OID 各處相等" mirror_constants

echo "==> i074 Stage 2 ⑦b：label shim（單元）"
SHIM_T="$TD/shimtest"
mkdir -p "$SHIM_T"
cp "$SR/scripts/lib/i074-stage2-docker-label-shim.sh" "$SHIM_T/docker"
chmod 755 "$SHIM_T/docker"
TOKEN_T="$(printf t | sha256sum | cut -d' ' -f1)"
printf '{"mode":"orchestrator","token":"%s"}' "$TOKEN_T" > "$LOCKDIR/i074-stage2.active"
shim_run() {  # 其餘＝參數；以 SHIM_ENV 覆寫
  RC=0
  env -i PATH="$TEST_PATH" I074_TEST_SCENARIO="$S" I074_STAGE2_TOKEN="${SHIM_TOKEN-$TOKEN_T}" \
    I074_STAGE2_REAL_DOCKER="${SHIM_REAL-$TRUSTED/docker}" ${SHIM_ENV:-} "$SHIM_T/docker" "$@" > "$S/out" 2> "$S/err" || RC=$?
}
shim_case() {  # $1＝說明；$2＝預期 rc；其餘＝參數
  local label="$1" want="$2"
  shift 2
  new_scenario
  shim_run "$@"
  if [ "$RC" = "$want" ]; then pass "$label"; else fail "$label（rc=$RC）"; cat "$S/err" >&2; fi
}
for sub in "run" "create" "container run" "container create"; do
  new_scenario
  # shellcheck disable=SC2086
  shim_run $sub --rm img cmd
  line="$(cat "$S/docker/calls.log" 2>/dev/null)"
  n="$(grep -o -- "--label $LABEL=$TOKEN_T" <<< "$line" | wc -l)"
  check "shim：$sub → 子指令之後恰好一個 label" test "$RC:$n:${line#"$sub --label $LABEL=$TOKEN_T "}" = "0:1:--rm img cmd"
done
new_scenario; shim_run ps -a
check "shim：其他子指令原樣放行" test "$RC:$(cat "$S/docker/calls.log")" = "0:ps -a"
new_scenario; printf 'labels-out' > "$S/docker_labels"; shim_run image inspect x
check "shim：I/O 透明（stdout 原樣）" test "$(cat "$S/out")" = "labels-out"
shim_case "shim：全域選項在子指令之前（-H … run）→ 125" 125 -H tcp://x run img
shim_case "shim：全域選項在任何子指令之前（--config ps）→ 125" 125 --config /x ps
shim_case "shim：--label-file → 125" 125 run --label-file f img
shim_case "shim：--label-file=… → 125" 125 run --label-file=f img
shim_case "shim：-l 自帶鍵 → 125" 125 run -l "$LABEL=x" img
shim_case "shim：--label= 自帶鍵 → 125" 125 run "--label=$LABEL=$TOKEN_T" img
shim_case "shim：值裡含鍵 → 125" 125 run -e "X=$LABEL" img
SHIM_TOKEN="ABC" shim_case "shim：token 不是 hex64 → 125" 125 run img
SHIM_TOKEN="$(printf u | sha256sum | cut -d' ' -f1)" shim_case "shim：token ≠ sentinel → 125" 125 run img
SHIM_REAL="$SHIM_T/docker" shim_case "shim：real docker 指向自己 → 125" 125 ps
cp "$REPO_ROOT/scripts/lib/i074-sizing-docker-shim.sh" "$SHIM_T/sizing-shim"; chmod 755 "$SHIM_T/sizing-shim"
SHIM_REAL="$SHIM_T/sizing-shim" shim_case "shim：real docker 是 sizing shim → 125" 125 ps
SHIM_REAL="relative/docker" shim_case "shim：real docker 不是絕對路徑 → 125" 125 ps
SHIM_ENV="SIZING_STATE=/x" shim_case "shim：環境帶 SIZING_* → 125" 125 ps
SHIM_ENV="SIZING_PROFILE=acceptance" shim_case "shim：環境帶 SIZING_PROFILE=acceptance（⑦d 的 acceptance harness）→ 125" 125 ps
chmod g+w "$TRUSTED/docker"
shim_case "shim：real docker 不符合信任條件（group 可寫）→ 125" 125 ps
chmod 755 "$TRUSTED/docker"
new_scenario; RC=0
env -i PATH="$TEST_PATH" I074_TEST_SCENARIO="$S" I074_STAGE2_TOKEN="$TOKEN_T" I074_STAGE2_REAL_DOCKER="$TRUSTED/docker" \
  bash "$SHIM_T/docker" ps > /dev/null 2>&1 || RC=$?
check "shim：以 bash <shim>（沒有 -p）執行 → 125" test "$RC" = 125
rm -f "$LOCKDIR/i074-stage2.active"

echo "==> i074 Stage 2 ⑦b：sizing shim／harness 與 label shim 互斥"
new_scenario
RC=0
env -i PATH="$TEST_PATH" I074_TEST_SCENARIO="$S" SIZING_REAL_DOCKER="$TRUSTED/docker" SIZING_STATE="$S" \
  I074_STAGE2_TOKEN="$TOKEN_T" "$REPO_ROOT/scripts/lib/i074-sizing-docker-shim.sh" ps >/dev/null 2>&1 || RC=$?
check "sizing shim：環境帶 I074_STAGE2_* → 125、real docker 未被呼叫" test "$RC:$(cat "$S/docker/calls.log" 2>/dev/null)" = "125:"
RC=0
env -i PATH="$TEST_PATH" I074_TEST_SCENARIO="$S" SIZING_REAL_DOCKER="$SHIM_T/docker" SIZING_STATE="$S" \
  "$REPO_ROOT/scripts/lib/i074-sizing-docker-shim.sh" ps >/dev/null 2>&1 || RC=$?
check "sizing shim：SIZING_REAL_DOCKER 是 label shim → 125" test "$RC" = 125
RC=0
env -i PATH="$TEST_PATH" HOME="$HOME" REPLAY_IMAGE_ID="$IMG" I074_STAGE2_TOKEN="$TOKEN_T" \
  "$REPO_ROOT/scripts/i074-stage2-sizing.sh" --work-dir "$TD/sizing-x" > /dev/null 2> "$S/err" || RC=$?
check "sizing harness：環境帶 I074_STAGE2_* → 中止" bash -c '[ "$1" != 0 ] && grep -q "互斥" "$2" && [ ! -e "$3" ]' _ "$RC" "$S/err" "$TD/sizing-x"
mkdir -p "$TD/lsbin"; cp "$SHIM_T/docker" "$TD/lsbin/docker"
RC=0
env -i PATH="$TD/lsbin:$TEST_PATH" HOME="$HOME" REPLAY_IMAGE_ID="$IMG" \
  "$REPO_ROOT/scripts/i074-stage2-sizing.sh" --work-dir "$TD/sizing-y" > /dev/null 2> "$S/err" || RC=$?
check "sizing harness：PATH 上的 docker 是 label shim → 中止" bash -c '[ "$1" != 0 ] && grep -q "label shim" "$2"' _ "$RC" "$S/err"

echo "==> i074 Stage 2 ⑦b：入口（正式命令、環境、argv、入口清單）"
# BASH_ENV：bash <script> 啟動被拒絕、supervisor ⛔ 沒有啟動（正式命令⛔ 處理 BASH_ENV 的那一支在完整流程裡驗）。
new_scenario
RC=0
env -i HOME="$HOME" PATH="$TEST_PATH" XDG_DATA_HOME="$XDG" REPLAY_IMAGE_ID="$IMG" I074_TEST_SCENARIO="$S" \
  bash "$SR/scripts/run-i074-stage2.sh" --freeze-record "$FR" --work-dir "$S/work" > "$S/out" 2> "$S/err" || RC=$?
check "bash <script> 啟動 → 1、supervisor ⛔ 沒有啟動（沒有鎖檔、沒有 docker 呼叫、沒有執行目錄）" \
  bash -c '[ "$1" = 1 ] && [ ! -e "$2/i074-stage2.lock" ] && [ ! -s "$3/docker/calls.log" ] && [ ! -e "$3/work" ]' _ "$RC" "$LOCKDIR" "$S"
RC=0
env -i HOME="$HOME" PATH="$TEST_PATH" XDG_DATA_HOME="$XDG" REPLAY_IMAGE_ID="$IMG" I074_TEST_SCENARIO="$S" \
  bash "$SR/scripts/run-i074-stage2.sh" --promote --work-dir "$S" > /dev/null 2>&1 || RC=$?
check "bash <script> --promote → 8" test "$RC" = 8
for bad in "" "--help" "-f" "stock-trading-python:latest" "sha256:$(printf x | sha256sum | cut -d' ' -f1 | tr a-f A-F)" \
           "sha256:$(printf x | sha256sum | cut -c1-63)" "$IMG " "$IMG
"; do
  new_scenario
  IMAGE_OVERRIDE="$bad" run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
  check "REPLAY_IMAGE_ID='$(printf %q "$bad")' → 1、入口就擋下（supervisor ⛔ 啟動）、docker ⛔ 一次都沒被呼叫" \
    bash -c '[ "$1" = 1 ] && [ ! -s "$2/docker/calls.log" ] && [ ! -e "$2/work" ] && grep -q "REPLAY_IMAGE_ID 必須是" "$2/err" \
             && ! grep -q "i074-stage2-supervisor:" "$2/err"' _ "$RC" "$S"
done
new_scenario; IMAGE_OVERRIDE="--help" run_entry "$SR" --promote --work-dir "$S"
check "REPLAY_IMAGE_ID 格式錯 ＋ --promote → 8" test "$RC" = 8
usage_case() {  # $1＝說明；$2＝預期；其餘＝參數
  local label="$1" want="$2"
  shift 2
  new_scenario
  run_entry "$SR" "$@"
  check "$label" bash -c '[ "$1" = "$2" ] && [ ! -e "$3/work" ] && [ ! -e "$4/i074-stage2.active" ]' _ "$RC" "$want" "$S" "$LOCKDIR"
}
usage_case "argv：未知參數 → 1" 1 --freeze-record "$FR" --work-dir "$TD/w-u1" --bogus
usage_case "argv：⛔ 沒有 --repo-head（裸 OID 的入口）→ 1" 1 --repo-head "$HEAD_OID" --work-dir "$TD/w-u2"
usage_case "argv：--x=value 寫法 → 1" 1 --freeze-record="$FR" --work-dir "$TD/w-u3"
usage_case "argv：--work-dir 重複 → 1" 1 --freeze-record "$FR" --work-dir "$TD/w-u4" --work-dir "$TD/w-u5"
usage_case "argv：缺值 → 1" 1 --freeze-record "$FR" --work-dir
usage_case "argv：模式互斥（--resume --promote）→ 8" 8 --resume --promote --work-dir "$TD"
for override in --after-artifact --bundle --output-dir --before-ref --cohort-manifest; do
  usage_case "ad：入口帶 $override（覆寫階段一的輸出）→ 1、沒有建複本" 1 --freeze-record "$FR" --work-dir "$TD/w-ad" "$override" /x
done
usage_case "work 目錄已存在 → 1" 1 --freeze-record "$FR" --work-dir "$TD"
usage_case "work 目錄在 repo 內 → 1" 1 --freeze-record "$FR" --work-dir "$SR/inside"
new_scenario; EXTRA_ENV="I074_STAGE2_TOKEN=$TOKEN_T" run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
check "環境已帶 I074_STAGE2_* → 1" bash -c '[ "$1" = 1 ] && [ ! -e "$2/work" ]' _ "$RC" "$S"
for f in scripts/run-i074-stage2.sh scripts/lib/i074-stage2-supervisor.py python/scripts/i074_stage2_freeze_record.py \
         python/scripts/i074_stage2_preflight.py python/scripts/_i074_bootstrap.py "$SRCDIR/replay_bundle/canonical.py"; do
  cp "$SR/$f" "$TD/saved"
  printf '\n# dirty\n' >> "$SR/$f"
  new_scenario; rm -f "$LOCKDIR/i074-stage2.lock"; run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
  check "入口清單：$f 的內容 ≠ HEAD → 1、沒有取鎖" bash -c '[ "$1" = 1 ] && [ ! -e "$2/i074-stage2.lock" ] && [ ! -e "$3/work" ]' _ "$RC" "$LOCKDIR" "$S"
  cp "$TD/saved" "$SR/$f"
  git -C "$SR" rm -q --cached -- "$f"
  new_scenario; run_entry "$SR" --promote --work-dir "$TD"
  check "入口清單：$f 未追蹤（git rm --cached）→ 8（promote）、沒有取鎖" bash -c '[ "$1" = 8 ] && [ ! -e "$2/i074-stage2.lock" ]' _ "$RC" "$LOCKDIR"
  git -C "$SR" add -- "$f"
done

echo "==> i074 Stage 2 ⑦b：完整流程（fake replay）"
# 對照組：清單以外的檔案 dirty 照常通過；呼叫者 PATH 前面放 fake git／docker／python3（⛔ 從未被執行）；外層環境被污染。
mkdir -p "$TD/evil"
for p in git docker python3; do printf '#!/bin/sh\ntouch "%s/evil_%s_ran"\nexit 1\n' "$TD" "$p" > "$TD/evil/$p"; chmod 755 "$TD/evil/$p"; done
printf 'dirty\n' >> "$SR/python/baselines/b1_test/manifest.json"
new_scenario
printf 'touch "%s/bash_env_ran"\n' "$S" > "$S/benv.sh"
printf 'import pathlib; pathlib.Path("%s/sitecustomize_ran").touch()\n' "$S" > "$S/sitecustomize.py"
CALLER_PATH="$TD/evil:$TEST_PATH" \
EXTRA_ENV="GIT_DIR=/nonexistent DOCKER_HOST=tcp://evil TOOLING_PATCH=/evil COUNTERFACTUAL_PATCH=/evil MEM=9g SIZING_X=1 LD_PRELOAD=/nonexistent/evil.so PYTHONPATH=$S BASH_ENV=$S/benv.sh ENV=$S/benv.sh" \
  run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
git -C "$SR" checkout -q -- python/baselines/b1_test/manifest.json
W="$S/work"; SUCCESS_S="$S"
check "成功路徑：終態（evidence/）→ 晉升 → 0（⑦c）" expect_rc 0
check "成功路徑：清單以外的檔案 dirty ⛔ 不擋（入口只驗清單的投影）、晉升完成" bash -c 'grep -q "晉升完成" "$1"' _ "$S/err"
check "⑦c 成功路徑：真正 repo 的 evidence/ 逐位元等於複本的終態、⛔ 沒有留下 staging" promoted_ok "$W" evidence
check "⑦c 成功路徑：驗證模式驗的是真正 repo 裡的 staging（直屬 i074_stage2）" bash -c '
  grep -qE "^VERIFY $2/python/baselines/i074_stage2/\.promote-staging-[0-9a-f]{16} evidence $" "$1/spy/verify.log"' _ "$S" "$SR"
check "⑦c 成功路徑：發布之後的目的地照常出現在真正 repo 的未追蹤清單（⛔ 不被忽略）" bash -c '
  git -C "$1" status --porcelain --untracked-files=all | grep -qF "?? python/baselines/i074_stage2/evidence/evidence_manifest.json"' _ "$SR"
check "信任根：呼叫者 PATH 前置的 fake git／docker／python3 ⛔ 從未被執行" bash -c '! ls "$1"/evil_*_ran >/dev/null 2>&1' _ "$TD"
check "LD_PRELOAD 只影響第一個程序（入口的 bash）：supervisor 與之後的程序⛔ 再載入" \
  test "$(grep -c "evil.so.*cannot be preloaded" "$S/err" || true)" = 1
new_scenario; RC=0
env -i HOME="$HOME" PATH="$TD/evil:$TEST_PATH" XDG_DATA_HOME="$XDG" REPLAY_IMAGE_ID="$IMG" I074_TEST_SCENARIO="$S" \
  /usr/bin/python3 -I "$SR/scripts/lib/i074-stage2-supervisor.py" run --work-dir "$S/work" --freeze-record "$FR" \
  > "$S/out" 2> "$S/err" || RC=$?
check "信任根（supervisor 那一層）：繞過入口、以呼叫者的 PATH 直接啟動 supervisor → fake git／docker／python3 仍⛔ 被執行" \
  bash -c '[ "$1" = 0 ] && ! ls "$2"/evil_*_ran >/dev/null 2>&1' _ "$RC" "$TD"
S="$SUCCESS_S"
check "正式命令直接執行：BASH_ENV／ENV ⛔ 沒有執行、PYTHONPATH 的 sitecustomize ⛔ 沒有執行（整條流程）" \
  bash -c '[ ! -e "$1/bash_env_ran" ] && [ ! -e "$1/sitecustomize_ran" ]' _ "$S"
check "成功路徑：五份中的四份 state（沒有 attempt）" bash -c 'cd "$1/state" && [ -f run.json ] && [ -f preflight.json ] && [ -f replay_started.json ] && [ -f replay_done.json ] && [ ! -e attempt.json ]' _ "$W"
check "成功路徑：sentinel 已刪、鎖已放" bash -c '[ ! -e "$1/i074-stage2.active" ]' _ "$LOCKDIR"
check "成功路徑：鎖已放（獨立 fd 立刻拿得到）" lock_free
CFSHA="$(sha256sum < "$SR/python/baselines/i074_stage2/counterfactual_e1cbbbd.patch" | cut -d' ' -f1)"
TOOLSHA="$(sha256sum < "$SR/python/baselines/i074_stage2/tooling_e1cbbbd.patch" | cut -d' ' -f1)"
check "環境：runner 只收到凍結副本的兩個 patch（路徑與 SHA）" bash -c '
  grep -qxF "ENV COUNTERFACTUAL_PATCH=$2/run/patches/counterfactual.patch" "$1" && grep -qxF "ENV TOOLING_PATCH=$2/run/patches/tooling.patch" "$1" \
  && grep -qxF "SHA COUNTERFACTUAL_PATCH $3" "$1" && grep -qxF "SHA TOOLING_PATCH $4" "$1"' _ "$S/spy/runner.log" "$W" "$CFSHA" "$TOOLSHA"
check "環境：finalizer 與 check ⛔ 收不到兩份 patch 的變數" bash -c '! grep -qE "^ENV (TOOLING_PATCH|COUNTERFACTUAL_PATCH)=" "$1"' _ "$S/spy/finalizer.log"
check "環境：GIT_*／DOCKER_*／MEM／SIZING_*／LD_PRELOAD／PYTHONPATH／BASH_ENV 誰都收不到" bash -c '
  ! grep -hqE "^ENV (GIT_DIR|DOCKER_HOST|MEM|SIZING_X|LD_PRELOAD|PYTHONPATH|BASH_ENV|ENV)=" "$1" "$2"' _ "$S/spy/runner.log" "$S/spy/finalizer.log"
check "環境：runner 的 PATH ＝ <work>/bin:TRUSTED_PATH、PYTHONDONTWRITEBYTECODE=1" bash -c '
  grep -qxF "ENV PATH=$2/bin:$3" "$1" && grep -qxF "ENV PYTHONDONTWRITEBYTECODE=1" "$1"' _ "$S/spy/runner.log" "$W" "$TEST_PATH"
check "replay argv：bundle／after／cohort 取自 anchors、帶 --i074-counterfactual" bash -c '
  grep -qF -- "--bundle $2/repo/python/baselines/b1_test --output-dir $2/run/stage2 --before-ref $3 --after-artifact $2/repo/python/baselines/i074_stage1/d1/after_artifact.json.gz --cohort-manifest $2/repo/python/baselines/i074_stage1/d1/cohort_manifest.json.gz --i074-counterfactual" "$1"' \
  _ "$S/spy/runner.log" "$W" "$BASE_OID"
label_ok() {  # 每一個 run／create 恰好一個本趟的 label（在子指令之後）；⛔ 任何呼叫帶正式鍵
  local f tok n bad=0
  tok="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["token"])' "$1" 2>/dev/null || true)"
  for f in "$S"/docker/run.*; do
    [ -f "$f" ] || continue
    n="$(grep -o -- "--label $LABEL=[0-9a-f]\{64\}" "$f" | wc -l)"
    [ "$n" = 1 ] || bad=1
  done
  ! grep -q 'i074\.stage2\.run[=]' "$S/docker/calls.log" && [ "$bad" = 0 ]
}
check "label：每個 docker run 恰好一個本趟的 label；⛔ 沒有任何帶正式鍵的呼叫" label_ok /dev/null
check "anchors：在 Stage 2 image 內以唯讀方式執行（--network none、--read-only、/app:ro）" bash -c '
  grep -qE "run --label [^ ]+ --rm --network none --read-only .* -v [^ ]+/repo/python:/app:ro .*i074_stage2_preflight.py anchors" "$1"' _ "$S/docker/calls.log"
check "worktree：replay 開始之後 orchestrator ⛔ 沒有呼叫 worktree remove／prune" bash -c '
  ! sed -n "/=== runner ===/,\$p" "$1" | grep -qE "worktree (remove|prune)"' _ "$S/git.log"
check "worktree：runner 留下的 worktree（實體目錄與登記）保留到 <work> 被刪除" bash -c '
  wt="$(cat "$2/runner_worktree")"; case "$wt" in "$1/tmp/"*) ;; *) exit 1 ;; esac
  [ -d "$wt" ] && git -C "$1/repo" worktree list --porcelain | grep -qxF "worktree $wt"' _ "$W" "$S"
check "worktree：複本的登記只在 <work>/tmp 底下" bash -c '
  git -C "$1/repo" worktree list --porcelain | sed -n "s/^worktree //p" | grep -vxF "$1/repo" | grep -vq "^$1/tmp/" && exit 1 || exit 0' _ "$W"

new_scenario; echo "1 no" > "$S/fin_plan"
run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
W="$S/work"
check "ae：finalize 回 1 且沒有終態 → 1 ＋ state/attempt.json" bash -c '[ "$1" = 1 ] && [ -f "$2/state/attempt.json" ]' _ "$RC" "$W"
printf '1 no\n0 yes\n' > "$S/fin_plan"
: > "$S/spy/runner.log"
run_entry "$SR" --resume --work-dir "$W"
check "ae：--resume → runner ⛔ 未被呼叫、finalize 以同一個 --run-dir 呼叫 → 終態 → 晉升 → 0" bash -c '
  [ "$1" = 0 ] && [ ! -s "$2/spy/runner.log" ] && [ "$(grep -c -- "--finalize --run-dir $3/run" "$2/spy/finalizer.log")" = 2 ]' _ "$RC" "$S" "$W"
INODE_BEFORE="$(stat -c %i "$SR/python/baselines/i074_stage2/evidence")"
run_entry "$SR" --promote --work-dir "$W"
check "⑦c --promote：已晉升 → 6a（冪等，只補 fsync）→ 0、目的地沒有被重建" bash -c '
  [ "$1" = 0 ] && [ "$(stat -c %i "$2/python/baselines/i074_stage2/evidence")" = "$3" ]' _ "$RC" "$SR" "$INODE_BEFORE"

resume_case() {  # $1＝說明；$2＝在 resume 之前執行的 shell（可用 $W）；預期 1 且 finalizer ⛔ 未被再呼叫
  local label="$1" mutate="$2" before after
  new_scenario; echo "1 no" > "$S/fin_plan"
  run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
  W="$S/work"
  before="$(grep -c . "$S/spy/finalizer.log" 2>/dev/null || true)"
  W="$W" S="$S" bash -c "$mutate"
  run_entry "$SR" --resume --work-dir "$W"
  after="$(grep -c . "$S/spy/finalizer.log" 2>/dev/null || true)"
  check "$label" bash -c '[ "$1" = 1 ] && [ "$2" = "$3" ]' _ "$RC" "$before" "$after"
}
resume_case "resume：沒有 attempt.json → 1、finalizer 未被呼叫" 'rm "$W/state/attempt.json"'
resume_case "resume：operational 輸出被改 → 1" 'echo x >> "$W/run/stage2/report.json"'
resume_case "resume：凍結 patch 被改 → 1" 'chmod u+w "$W/run/patches/tooling.patch"; echo x >> "$W/run/patches/tooling.patch"'
resume_case "resume：state 被刪（replay_done）→ 1" 'rm "$W/state/replay_done.json"'
resume_case "resume：state 多一欄（非封閉）→ 1" 'python3 -c "import json,sys; p=json.load(open(sys.argv[1])); p[\"x\"]=1; open(sys.argv[1],\"w\").write(json.dumps(p,sort_keys=True,separators=(\",\",\":\")))" "$W/state/run.json"'
resume_case "resume：複本的 HEAD 被移動（同一個 tree 的新 commit）→ 1" \
  'git -C "$W/repo" -c user.name=t -c user.email=t@t -c commit.gpgsign=false commit -q --allow-empty -m moved'
resume_case "resume：複本的 HEAD 被移到沒有 orchestrator 的 commit → 1（持鎖階段 fail-closed）" 'git -C "$W/repo" checkout -q --detach HEAD~1'
new_scenario; echo "1 no" > "$S/fin_plan"
run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"; W="$S/work"
sed -i "2i touch \"$S/tampered_orch_ran\"" "$W/repo/scripts/run-i074-stage2.sh"
git -C "$W/repo" update-index --assume-unchanged scripts/run-i074-stage2.sh
run_entry "$SR" --resume --work-dir "$W"
check "⑦b 實作第一輪 review（中）：resume 時複本的 orchestrator 被改（assume-unchanged 掩蓋）→ 1、⛔ 被執行、finalizer 未被再呼叫" \
  bash -c '[ "$1" = 1 ] && [ ! -e "$2/tampered_orch_ran" ] && [ "$(grep -c -- --finalize "$2/spy/finalizer.log")" = 1 ]' _ "$RC" "$S"
run_entry "$SR" --promote --work-dir "$W"
check "⑦b 實作第一輪 review（中）：promote 時同上 → 9（複本完整性）、⛔ 被執行" \
  bash -c '[ "$1" = 9 ] && [ ! -e "$2/tampered_orch_ran" ]' _ "$RC" "$S"
git -C "$W/repo" update-index --no-assume-unchanged scripts/run-i074-stage2.sh
git -C "$W/repo" checkout -q -- scripts/run-i074-stage2.sh
IMAGE_OVERRIDE="$IMG2" run_entry "$SR" --resume --work-dir "$W"
check "resume：REPLAY_IMAGE_ID 改變 → 1、finalizer 未被再呼叫" bash -c '[ "$1" = 1 ] && [ "$(grep -c -- --finalize "$2")" = 1 ]' _ "$RC" "$S/spy/finalizer.log"
cp "$IDENTITY" "$TD/identity.saved"
write_identity "2026-09-30T00:00:01+00:00" "$IMG"
run_entry "$SR" --resume --work-dir "$W"
check "resume：XDG identity 只改 created_at → 1、finalizer 未被再呼叫" bash -c '[ "$1" = 1 ] && [ "$(grep -c -- --finalize "$2")" = 1 ]' _ "$RC" "$S/spy/finalizer.log"
cp "$TD/identity.saved" "$IDENTITY"

new_scenario; echo 6 > "$S/runner_rc"; echo "1 yes" > "$S/fin_plan"
run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
check "ad／z：replay 回 6 → publish-failed-record；它回 1 但 record 已在磁碟 → 晉升 → 6（依磁碟事實、⛔ 不寫 attempt）" bash -c '
  [ "$1" = 6 ] && grep -q -- "--publish-failed-record --run-dir" "$2/spy/finalizer.log" && [ ! -e "$2/work/state/attempt.json" ]' _ "$RC" "$S"
check "⑦c：failed record 晉升到真正 repo 的 failed/<bundle>-<語意 SHA>（failed/ 由晉升建立）" promoted_ok "$S/work" "$(failed_name "$S/work")"
new_scenario; echo 6 > "$S/runner_rc"; printf '1 no\n1 yes\n' > "$S/fin_plan"
run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
rc1="$RC"
run_entry "$SR" --resume --work-dir "$S/work"
check "publish 回 1 且沒有 record → attempt → --resume 再 publish → 晉升 → 6" test "$rc1:$RC" = "1:6"
new_scenario; echo "3 yes" > "$S/fin_plan"
run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
check "finalize 回 3 ＋ evidence/ 存在 → 依磁碟事實晉升 → 0（⛔ 不呼叫 ③ 的 recovery）" bash -c '
  [ "$1" = 0 ] && ! grep -q -- "--recover-durability" "$2/spy/finalizer.log"' _ "$RC" "$S"
run_entry "$SR" --promote --work-dir "$S/work"
check "n11：finalize 回 3 之後直接 --promote → 0" expect_rc 0
new_scenario; echo "3 no" > "$S/fin_plan"
run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
check "finalize 回 3 但沒有終態（矛盾）→ 1、⛔ 不寫 attempt" bash -c '[ "$1" = 1 ] && [ ! -e "$2/work/state/attempt.json" ]' _ "$RC" "$S"

for rc in 1 2 4 137; do                       # ⑧ 補齊：137（OOM）
  new_scenario; echo "$rc" > "$S/runner_rc"
  run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
  check "ad：replay 回 $rc → 1、finalize／publish 都未被呼叫、⛔ 沒有 replay_done" bash -c '
    [ "$1" = 1 ] && ! grep -qE -- "--(finalize|publish-failed-record)" "$2/spy/finalizer.log" && [ ! -e "$2/work/state/replay_done.json" ]' _ "$RC" "$S"
done
for spec in "0 extra" "0 missing" "6 both"; do
  new_scenario; echo "${spec% *}" > "$S/runner_rc"; echo "${spec#* }" > "$S/runner_shape"
  run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
  check "ad：replay rc=${spec% *} 但輸出形狀是 ${spec#* } → 1、未發布" bash -c '
    [ "$1" = 1 ] && ! grep -qE -- "--(finalize|publish-failed-record)" "$2/spy/finalizer.log"' _ "$RC" "$S"
done

echo "==> i074 Stage 2 ⑦b：preflight 的每一步（失敗 → 1，之後各步⛔ 不執行）"
pre_case() {  # $1＝說明；$2＝預期 rc；$3＝情境設定（shell，可用 $S）
  local label="$1" want="$2"
  new_scenario
  S="$S" bash -c "$3"
  run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
  check "$label" bash -c '[ "$1" = "$2" ] && [ ! -s "$3/spy/runner.log" ] && [ ! -e "$3/work/state/preflight.json" ]' _ "$RC" "$want" "$S"
}
pre_case "n：check-failed-record 回 2（同語意 SHA 已記錄）→ 2、runner 未被呼叫" 2 'echo 2 > "$S/check_rc"'
pre_case "check-failed-record 回 1（record 損壞）→ 1" 1 'echo 1 > "$S/check_rc"'
pre_case "ai／ay：anchors 不通過 → 1、check 與 runner 都未被呼叫" 1 'touch "$S/anchors_fail"'
check "ai／ay：anchors 不通過時 check-failed-record ⛔ 未被呼叫" bash -c '! grep -q -- --check-failed-record "$1/spy/finalizer.log" 2>/dev/null' _ "$S"
pre_case "anchors 的 after_base_commit ≠ freeze record 的 base → 1" 1 \
  'sed -i "s/\"after_base_commit\":\"[0-9a-f]*\"/\"after_base_commit\":\"'"$HEAD_OID"'\"/" "$S/anchors.json"'
pre_case "anchors 的 bundle 不在複本裡 → 1" 1 'sed -i "s/\"bundle_id\":\"b1_test\"/\"bundle_id\":\"b1_missing\"/" "$S/anchors.json"'
pre_case "n4：docker info 失敗 → 1" 1 'touch "$S/docker_info_fail"'
pre_case "n4：Docker Root Dir 是空的 → 1" 1 ': > "$S/docker_root"'
pre_case "n3／n5：Docker Root Dir 在另一個裝置（/dev/shm）→ 1" 1 'echo /dev/shm > "$S/docker_root"'
pre_case "worktree：preflight 第 2 步的暫時 worktree 移除失敗 → 1" 1 'touch "$S/git_fail_worktree_remove"'
cp "$IDENTITY" "$TD/identity.saved3"
write_identity "2026-09-30T00:00:09+00:00" "$IMG"
new_scenario
run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
check "⑦b 實作第一輪 review（低）：preflight 2 驗證失敗（identity 不符）→ 1，暫時 worktree 照樣移除（登記與實體目錄）" bash -c '
  [ "$1" = 1 ] && [ "$(git -C "$2/work/repo" worktree list --porcelain | grep -c "^worktree ")" = 1 ] \
  && ! ls -d "$2"/work/tmp/compose.* >/dev/null 2>&1 && grep -q "Stage 2 identity" "$2/err"' _ "$RC" "$S"
cp "$TD/identity.saved3" "$IDENTITY"
new_scenario
printf 'echo tampered >> "%s/work/repo/python/baselines/i074_stage2/counterfactual_e1cbbbd.patch"\n' "$S" > "$S/anchors_hook"
run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
check "ax：凍結之後改掉複本常數路徑上的 counterfactual → runner 收到的是凍結副本（路徑與 SHA 皆為原值）" bash -c '
  grep -qxF "ENV COUNTERFACTUAL_PATCH=$2/work/run/patches/counterfactual.patch" "$1" && grep -qxF "SHA COUNTERFACTUAL_PATCH $3" "$1"' \
  _ "$S/spy/runner.log" "$S" "$CFSHA"
check "ax：之後的檢查點因複本 dirty 中止 → 1、finalize 未被呼叫" bash -c '
  [ "$1" = 1 ] && ! grep -q -- --finalize "$2/spy/finalizer.log" 2>/dev/null' _ "$RC" "$S"
first_run_integrity() {  # $1＝說明；$2＝fake git 在 checkout 之後執行的改動（$1＝複本）
  new_scenario
  printf '%s\n' "$2" > "$S/git_after_checkout"
  run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
  check "$1 → 1、anchors／check／runner ⛔ 都未被呼叫" bash -c '
    [ "$1" = 1 ] && ! grep -q anchors "$2/docker/calls.log" 2>/dev/null && [ ! -s "$2/spy/finalizer.log" ] && [ ! -s "$2/spy/runner.log" ]' _ "$RC" "$S"
}
first_run_integrity "n7：複本的已追蹤檔被修改" 'echo x >> "$1/python/scripts/i074_stage2_preflight.py"'
first_run_integrity "n7：複本在 i074_stage2/ 以外有未追蹤檔" 'touch "$1/python/stray.txt"'
first_run_integrity "n7：複本在 i074_stage2/ 以外有 ignored 檔" 'printf "*.ign\n" > "$1/.git/info/exclude"; touch "$1/python/x.ign"'
first_run_integrity "n7：複本有 alternates" 'mkdir -p "$1/.git/objects/info"; echo /nonexistent > "$1/.git/objects/info/alternates"'
first_run_integrity "n7：orchestrator 自身內容 ≠ repo_head（assume-unchanged 掩蓋）" \
  "sed -i '2i touch \"$TD/tampered_orch_ran\"' \"\$1/scripts/run-i074-stage2.sh\"; git -C \"\$1\" update-index --assume-unchanged scripts/run-i074-stage2.sh"
check "⑦b 實作第一輪 review（中）：被改過的複本 orchestrator ⛔ 被執行（持鎖階段在 exec 之前以真正 repo 的物件比對）" \
  bash -c '[ ! -e "$1/tampered_orch_ran" ] && grep -q "repo_head 中的版本" "$2/err"' _ "$TD" "$S"
new_scenario; mkdir -p "$SR/python/baselines/i074_stage2/failed/b1_test-x"; echo r > "$SR/python/baselines/i074_stage2/failed/b1_test-x/r"
run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
check "n12：真正 repo 的 i074_stage2/ 底下有未追蹤的 failed record → 1、runner 未被呼叫" bash -c '[ "$1" = 1 ] && [ ! -s "$2/spy/runner.log" ]' _ "$RC" "$S"
git -C "$SR" add -A python/baselines/i074_stage2/failed && git -C "$SR" commit -qm "promoted failed record"
new_scenario
run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
check "n12：真正 repo HEAD 的 failed/ ⊄ 複本（舊的 freeze record）→ 1、runner 未被呼叫" bash -c '[ "$1" = 1 ] && [ ! -s "$2/spy/runner.log" ] && grep -q "freeze record 太舊" "$2/err"' _ "$RC" "$S"
NEW_HEAD="$(git -C "$SR" rev-parse HEAD)"
make_freeze "$TD/freeze2" "$NEW_HEAD"
new_scenario
run_entry "$SR" --freeze-record "$TD/freeze2/freeze_record.json" --work-dir "$S/work"
check "n12：commit 之後用新的 freeze record → 通過 preflight（進 replay）" bash -c '[ "$1" = 0 ] && [ -s "$2/spy/runner.log" ]' _ "$RC" "$S"
git -C "$SR" reset -q --hard "$HEAD_OID"
# ba（orchestrator 層）：repo_head 的 tooling patch 是 0 bytes
: > "$SR/python/baselines/i074_stage2/tooling_e1cbbbd.patch"
git -C "$SR" commit -qam "empty tooling"
make_freeze "$TD/freeze3" "$(git -C "$SR" rev-parse HEAD)" "$TOOLSHA"
new_scenario
run_entry "$SR" --freeze-record "$TD/freeze3/freeze_record.json" --work-dir "$S/work"
check "ba：repo_head 的 tooling patch 是 0 bytes → 1、runner 未被呼叫" bash -c '[ "$1" = 1 ] && [ ! -s "$2/spy/runner.log" ] && grep -q "不得為空" "$2/err"' _ "$RC" "$S"
git -C "$SR" reset -q --hard "$HEAD_OID"
# freeze record 的違反（shell 兩支；逐條在 pytest）
new_scenario; cp "$FR_DIR/sizing_report.json" "$TD/fr-bad-dir.json" 2>/dev/null || true
mkdir -p "$TD/frbad"; cp "$FR_DIR/sizing_report.json" "$TD/frbad/"
python3 -c 'import json,sys; p=json.load(open(sys.argv[1])); p["p_b_bytes"]=True; open(sys.argv[2],"w").write(json.dumps(p,sort_keys=True,separators=(",",":")))' "$FR" "$TD/frbad/freeze_record.json"
run_entry "$SR" --freeze-record "$TD/frbad/freeze_record.json" --work-dir "$S/work"
check "n7：freeze record 的 p_b_bytes 是 bool → 1、⛔ 沒有建複本" bash -c '[ "$1" = 1 ] && [ ! -e "$2/work/repo" ]' _ "$RC" "$S"
new_scenario; mkdir -p "$TD/frbad2"; cp "$FR" "$TD/frbad2/"; printf '{}' > "$TD/frbad2/sizing_report.json"
run_entry "$SR" --freeze-record "$TD/frbad2/freeze_record.json" --work-dir "$S/work"
check "n7：報告與 freeze record 的 report_sha256 不符 → 1、runner 未被呼叫" bash -c '[ "$1" = 1 ] && [ ! -s "$2/spy/runner.log" ]' _ "$RC" "$S"

echo "==> i074 Stage 2 ⑦b：replay 之後的檢查點（n8）"
post_case() {  # $1＝說明；$2＝runner_hook 的內容（$W 可用）；$3（可選）＝replay 的結束碼
  new_scenario
  [ -z "${3:-}" ] || echo "$3" > "$S/runner_rc"
  printf 'W=%s\n%s\n' "$S/work" "$2" > "$S/runner_hook"
  run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
  check "$1 → 1、finalize／publish 未被呼叫、⛔ 沒有 attempt" bash -c '
    [ "$1" = 1 ] && ! grep -qE -- "--(finalize|publish-failed-record)" "$2/spy/finalizer.log" && [ ! -e "$2/work/state/attempt.json" ]' _ "$RC" "$S"
}
post_case "n8：replay 期間改掉複本的已追蹤檔" 'echo x >> "$W/repo/python/scripts/i074_stage2_preflight.py"'
post_case "n8：replay 期間換掉 freeze record 副本" 'echo " " >> "$W/freeze/freeze_record.json"'
post_case "n8：replay 期間改掉 <work>/bin/docker" 'chmod u+w "$W/bin/docker"; echo "#" >> "$W/bin/docker"'
post_case "n8：replay 期間在 <work>/bin 多放一個檔" 'touch "$W/bin/git"'
# ⚠️ ⑧ 補齊：上面「改掉複本的已追蹤檔」改的是 orchestrator 自己會執行的 helper——它在檢查點之前就因語法錯誤失敗，
#   證明不了檢查點本身。下面兩支改**⛔ 不會被執行**的已追蹤檔（bundle 的 manifest），finalize 與 publish 兩條路徑各一支。
post_case "n8（⑧ 補齊）：replay 期間改掉複本裡⛔ 不會被執行的已追蹤檔（bundle manifest）" 'echo x >> "$W/repo/python/baselines/b1_test/manifest.json"'
post_case "n8（⑧ 補齊）：replay 回 6（publish 之前）時改掉同一個已追蹤檔" 'echo x >> "$W/repo/python/baselines/b1_test/manifest.json"' 6
post_case "state：第一次執行時 replay 期間 XDG identity 被改（只改 created_at）" \
  "cp \"$IDENTITY\" \"$TD/identity.saved2\"; python3 -c 'import sys; p=open(sys.argv[1]).read().replace(\"00:00:00+00:00\",\"00:00:02+00:00\"); open(sys.argv[1],\"w\").write(p)' \"$IDENTITY\""
cp "$TD/identity.saved2" "$IDENTITY"
new_scenario; echo "1 no" > "$S/fin_plan"
run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
echo x >> "$S/work/repo/python/scripts/i074_stage2_preflight.py"
run_entry "$SR" --promote --work-dir "$S/work"
check "n8：--promote 之前改動複本 → 9、晉升⛔ 未開始" bash -c '[ "$1" = 9 ] && ! grep -q "==> 晉升" "$2"' _ "$RC" "$S/err"
new_scenario; echo "1 no" > "$S/fin_plan"                  # ⑧ 補齊：同上，但改⛔ 不會被執行的已追蹤檔
run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
echo x >> "$S/work/repo/python/baselines/b1_test/manifest.json"
run_entry "$SR" --promote --work-dir "$S/work"
check "n8（⑧ 補齊）：--promote 之前改動複本裡⛔ 不會被執行的已追蹤檔 → 9、晉升⛔ 未開始" bash -c '[ "$1" = 9 ] && ! grep -q "==> 晉升" "$2"' _ "$RC" "$S/err"

echo "==> i074 Stage 2 ⑦c：晉升（真 git 的 ignore 守門、staging 與 git add -A、n9、n12 的晉升那一層）"
GI="$SR/.gitignore"
cp "$GI" "$TD/gitignore.saved"
restore_gi() { cp "$TD/gitignore.saved" "$GI"; rm -f "$SR/python/baselines/i074_stage2/.gitignore"; }
no_staging() { ! ls -d "$SR/python/baselines/i074_stage2"/.promote-staging-* >/dev/null 2>&1; }
verify_count() { grep -c . "$S/spy/verify.log" 2>/dev/null || true; }

# ⑩ 開始之前就把 staging 規則刪掉（工作樹；入口清單⛔ 不含 .gitignore）→ 晉升的 staging 守門 → 9、⛔ 不建 staging。
new_scenario
: > "$GI"
run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
check "ignore 守門：⑩ 開始之前刪掉 staging 規則 → 晉升 9、目的地不存在" bash -c '
  [ "$1" = 9 ] && [ ! -e "$2/python/baselines/i074_stage2/evidence" ] && grep -q "staging 的 ignore 守門" "$3/work/logs/promote.err"' _ "$RC" "$SR" "$S"
check "ignore 守門：⛔ 沒有建立任何 .promote-staging-*" no_staging
restore_gi
run_entry "$SR" --promote --work-dir "$S/work"
check "ignore 守門：修好規則之後重跑 --promote → 0" expect_rc 0

# 以 --promote 重跑同一個執行目錄（⛔ 不再跑 preflight）：先讓第一次的晉升在驗證模式失敗，終態留在複本、真正 repo 沒有東西。
new_scenario; echo 1 > "$S/verify_rc"
run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
GW="$S/work"; GS="$S"
check "ignore 守門（準備）：驗證模式失敗 → 9、目的地不存在、staging 已清" bash -c '
  [ "$1" = 9 ] && [ ! -e "$2/python/baselines/i074_stage2/evidence" ]' _ "$RC" "$SR"
rm -f "$GS/verify_rc"
guard_case() {  # $1＝說明；$2＝在真正 repo 執行的改動（shell）
  clean_real          # ⚠️ 每一支都從「目的地不存在」開始（⛔ 不讓前一支的結果改走 6a——反向驗證抓到的遮蔽）
  restore_gi
  (cd "$SR" && eval "$2")
  run_entry "$SR" --promote --work-dir "$GW"
  check "$1 → 9、⛔ 沒有 staging、目的地不存在" bash -c '
    [ "$1" = 9 ] && [ ! -e "$2/python/baselines/i074_stage2/evidence" ]' _ "$RC" "$SR"
  check "$1：⛔ 沒有建立任何 .promote-staging-*" no_staging
}
guard_case "ignore 守門：過廣規則 python/baselines/i074_stage2/*" 'printf "python/baselines/i074_stage2/*\n" >> .gitignore'
guard_case "ignore 守門：過廣規則 python/baselines/" 'printf "python/baselines/\n" >> .gitignore'
guard_case "ignore 守門：i074_stage2/.gitignore 的否定規則 !.promote-staging-*" \
  'printf "!.promote-staging-*\n" > python/baselines/i074_stage2/.gitignore'
clean_real
restore_gi
printf 'python/baselines/i074_stage2/.promote-staging-*/\n' > "$GI"
run_entry "$SR" --promote --work-dir "$GW"
check "ignore 守門（對照組）：只對目錄生效的規則 …/.promote-staging-*/ → 照常晉升 0" bash -c '
  [ "$1" = 0 ] && diff -r "$2/repo/python/baselines/i074_stage2/evidence" "$3/python/baselines/i074_stage2/evidence" >/dev/null' \
  _ "$RC" "$GW" "$SR"
# 第六輪 review 的 recovery：目的地已與來源逐位元相同（6b rename 之後、parent fsync 之前的狀態）＋ 過廣規則 → 6a → 9、目的地未被動。
restore_gi
printf 'python/baselines/\n' >> "$GI"
INODE_R="$(stat -c %i "$SR/python/baselines/i074_stage2/evidence")"
N_VERIFY="$(verify_count)"
run_entry "$SR" --promote --work-dir "$GW"
N_VERIFY_AFTER="$(verify_count)"
check "ignore 守門（recovery）：目的地已存在 ＋ 過廣規則 → 6a → 9、⛔ 不呼叫驗證模式、目的地未被動" bash -c '
  [ "$1" = 9 ] && [ "$2" = "$3" ] && [ "$(stat -c %i "$4/python/baselines/i074_stage2/evidence")" = "$5" ]' \
  _ "$RC" "$N_VERIFY" "$N_VERIFY_AFTER" "$SR" "$INODE_R"
restore_gi
run_entry "$SR" --promote --work-dir "$GW"
check "ignore 守門（recovery）：修好規則之後 → 0" expect_rc 0

# staging 與 git add -A（第一輪 review）：staging 已建好、rename 之前（fake 驗證模式那一刻）在真正 repo 執行 git add -A。
new_scenario
cat > "$S/verify_hook" <<EOF
case "\$1" in
  */.promote-staging-*)
    git -C "$SR" add -A >/dev/null 2>&1
    git -C "$SR" diff --cached --name-only > "$S/index_names"
    git -C "$SR" status --porcelain --untracked-files=all > "$S/status_at_barrier" ;;
esac
EOF
run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
check "staging 與 git add -A：barrier 有執行、晉升照常 0" bash -c '[ "$1" = 0 ] && [ -f "$2/index_names" ]' _ "$RC" "$S"
check "staging 與 git add -A：staging ⛔ 不進 index、⛔ 不出現在未追蹤清單" bash -c '
  ! grep -q promote-staging "$1/index_names" && ! grep -q promote-staging "$1/status_at_barrier"' _ "$S"
check "staging 與 git add -A：發布之後的目的地照常出現在未追蹤清單" bash -c '
  git -C "$1" status --porcelain --untracked-files=all | grep -qF "?? python/baselines/i074_stage2/evidence/"' _ "$SR"
git -C "$SR" reset -q

# ⑧ 補齊（v29 n9）：(i)(ii)(iii) 各自一個情境；晉升的 evidence/ 與 runner 收到的兩份 patch，都與「⛔ 不動真正 repo」的
#   對照組逐位元相同。錨點改的是 Stage 1 錨點與 envcheck/（規格寫的那兩處）。下面原本的合併情境照舊（判讀器接在它之後）。
new_scenario
run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
check "n9 對照組：⛔ 不動真正 repo → 晉升 0" expect_rc 0
rm -rf "$TD/n9_control"; cp -a "$SR/python/baselines/i074_stage2/evidence" "$TD/n9_control"
grep '^SHA ' "$S/spy/runner.log" > "$TD/n9_control.sha"
n9_case() {  # $1＝說明；$2＝runner_hook（⑩ 期間在真正 repo 做的事）
  new_scenario
  printf '%s\n' "$2" > "$S/runner_hook"
  run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
  check "$1 → 晉升 0；evidence/ 與 runner 收到的兩份 patch 都與對照組逐位元相同" bash -c '
    [ "$1" = 0 ] && [ -s "$2/n9_control.sha" ] && diff -r "$2/n9_control" "$3/python/baselines/i074_stage2/evidence" >/dev/null \
    && diff <(grep "^SHA " "$4/spy/runner.log") "$2/n9_control.sha" >/dev/null' _ "$RC" "$TD" "$SR" "$S"
  git -C "$SR" reset -q --hard "$HEAD_OID"
}
n9_case "n9 (i)：⑩ 期間在真正 repo commit 一般的檔案" "
git -C '$SR' commit -q --allow-empty -m 'n9 (i)'
printf 'n9\n' > '$SR/python/notes.txt'; git -C '$SR' add python/notes.txt; git -C '$SR' commit -qm 'n9 (i) file'"
n9_case "n9 (ii)：⑩ 期間在真正 repo 的工作樹改 Stage 1 錨點與 envcheck/" "
printf 'tampered\n' >> '$SR/python/baselines/i074_stage1/evidence_manifest.json'
printf 'tampered\n' >> '$SR/python/baselines/i074_stage2/envcheck/evidence_manifest.json'"
n9_case "n9 (iii)：⑩ 期間在真正 repo commit 改 Stage 1 錨點與 envcheck/（連驗證入口一起改壞）" "
printf 'tampered\n' >> '$SR/python/baselines/i074_stage1/evidence_manifest.json'
printf 'tampered\n' >> '$SR/python/baselines/i074_stage2/envcheck/evidence_manifest.json'
printf 'exit 1\n' > '$SR/scripts/finalize-stage2-evidence.sh'
git -C '$SR' commit -qam 'n9 (iii)'"

# n9：⑩ 執行期間在真正 repo (i) commit 一般檔案、(ii) 工作樹改錨點、(iii) commit 改錨點（連驗證入口一起改壞）——
#     晉升的證據與複本的終態逐位元相同；判讀器（之後）照樣判讀 base_commit 那一版。
new_scenario
cat > "$S/runner_hook" <<EOF
git -C "$SR" -c user.name=t -c user.email=t@t -c commit.gpgsign=false commit -q --allow-empty -m "n9 (i)"
printf 'n9\n' > "$SR/python/notes.txt"; git -C "$SR" add python/notes.txt
git -C "$SR" -c user.name=t -c user.email=t@t -c commit.gpgsign=false commit -q -m "n9 (i) file"
printf 'tampered anchor\n' >> "$SR/python/baselines/i074_stage2/tooling_e1cbbbd.patch"
printf 'exit 1\n' > "$SR/scripts/finalize-stage2-evidence.sh"
git -C "$SR" -c user.name=t -c user.email=t@t -c commit.gpgsign=false commit -qam "n9 (iii)"
printf 'tampered again\n' >> "$SR/python/baselines/i074_stage2/counterfactual_e1cbbbd.patch"
EOF
run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
N9_S="$S"; N9_W="$S/work"
check "n9：⑩ 期間真正 repo 照常 commit 與編輯 → 晉升 0、證據 ≡ 複本的終態" bash -c '[ "$1" = 0 ]' _ "$RC"
check "n9：晉升的證據逐位元等於複本的終態（對照組）" promoted_ok "$N9_W" evidence

echo "==> i074 Stage 2 ⑦c：判讀器（bootstrap 與流程；接在 n9 之後：真正 repo 的 HEAD 與工作樹都已改掉錨點與驗證入口）"
judge_run() {  # 其餘＝參數
  RC=0
  env -i HOME="$HOME" PATH="${CALLER_PATH:-$TEST_PATH}" XDG_DATA_HOME="$XDG" REPLAY_IMAGE_ID="${IMAGE_OVERRIDE-$IMG}" \
    I074_TEST_SCENARIO="$S" ${EXTRA_ENV:-} "$SR/scripts/judge-i074-stage2.sh" "$@" > "$S/out" 2> "$S/err" || RC=$?
}
judge_ok() { [ "$RC" = 0 ] && [ "$(grep -c . "$S/out")" = 1 ] && grep -q '"verdict":{' "$S/out"; }
judge_refused() { [ "$RC" = 1 ] && [ ! -s "$S/out" ]; }
S="$N9_S"
CALLER_PATH="$TD/evil:$TEST_PATH" judge_run
check "判讀器 n9 (ii)(iii)：真正 repo 的工作樹與 HEAD 都改了錨點、HEAD 的驗證入口已壞 → 照樣判讀（base_commit 那一版）" judge_ok
check "判讀器：驗證模式在 base_commit 的暫存複本裡、以 --judge 驗真正 repo 的 evidence/" bash -c '
  grep -qxF "VERIFY $2/python/baselines/i074_stage2/evidence evidence --judge" "$1/spy/verify.log"' _ "$S" "$SR"
check "判讀器：呼叫者 PATH 前置的 fake git／docker／python3 ⛔ 從未被執行" bash -c '! ls "$1"/evil_*_ran >/dev/null 2>&1' _ "$TD"
RC=0
env -i HOME="$HOME" PATH="$TEST_PATH" XDG_DATA_HOME="$XDG" REPLAY_IMAGE_ID="$IMG" I074_TEST_SCENARIO="$S" \
  bash "$SR/scripts/judge-i074-stage2.sh" > "$S/out" 2> "$S/err" || RC=$?
check "判讀器：bash <script> → 1、⛔ 不輸出 B／C" judge_refused
judge_run --target evidence
check "判讀器：⛔ 沒有參數（帶了 → 1）" judge_refused
IMAGE_OVERRIDE="sha256:$(printf x | sha256sum | cut -c1-63)" judge_run
check "判讀器：REPLAY_IMAGE_ID 格式錯 → 1" judge_refused
cp "$SR/scripts/judge-i074-stage2.sh" "$TD/judge.saved"
printf '\n# dirty\n' >> "$SR/scripts/judge-i074-stage2.sh"
judge_run
check "判讀器：腳本的內容 ≠ HEAD → 1" judge_refused
cp "$TD/judge.saved" "$SR/scripts/judge-i074-stage2.sh"
MANIFEST="$SR/python/baselines/i074_stage2/evidence/evidence_manifest.json"
cp "$MANIFEST" "$TD/manifest.saved"
set_judge_base() { python3 -c 'import json,sys; p=json.load(open(sys.argv[1])); p["finalizer_provenance"]["base_commit"]=sys.argv[2]; open(sys.argv[1],"w").write(json.dumps(p))' "$MANIFEST" "$1"; }
set_judge_base "$(printf 'f%.0s' {1..40})"
judge_run
check "判讀器 m：base_commit 不可達 → 1、⛔ 不輸出 B／C" judge_refused
LACK_IDX="$TD/lack.index"
GIT_INDEX_FILE="$LACK_IDX" git -C "$SR" read-tree "$HEAD_OID"
GIT_INDEX_FILE="$LACK_IDX" git -C "$SR" rm -q --cached python/baselines/i074_stage2/counterfactual_e1cbbbd.patch
LACK="$(git -C "$SR" -c user.name=t -c user.email=t@t commit-tree -p "$HEAD_OID" -m "lack anchor" "$(GIT_INDEX_FILE="$LACK_IDX" git -C "$SR" write-tree)")"
git -C "$SR" update-ref refs/heads/lack-anchor "$LACK"
set_judge_base "$LACK"
echo "python/baselines/i074_stage2/counterfactual_e1cbbbd.patch" > "$S/verify_needs"
judge_run
check "判讀器 m：base_commit 的樹裡缺錨點 → 驗證模式不通過 → 1、⛔ 不輸出 B／C" judge_refused
rm -f "$S/verify_needs"
cp "$TD/manifest.saved" "$MANIFEST"
echo 1 > "$S/verify_rc"
judge_run
check "判讀器：驗證模式不通過 → 1、⛔ 不輸出 B／C" judge_refused
rm -f "$S/verify_rc"
judge_run
check "判讀器：同一份 evidence 再判一次 → 0（唯讀、可重跑）" judge_ok
git -C "$SR" update-ref -d refs/heads/lack-anchor
git -C "$SR" reset -q --hard "$HEAD_OID"

# n12（晉升那一層）：failed record 晉升之後還沒 commit → 下一次 ⑩ 的 preflight 中止；commit 之後換新的 freeze record → 通過。
new_scenario; echo 6 > "$S/runner_rc"; echo "1 yes" > "$S/fin_plan"
run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
N12_NAME="$(failed_name "$S/work")"
check "n12：replay 回 6 → failed record 晉升到真正 repo → 6" bash -c '[ "$1" = 6 ] && [ -d "$2/python/baselines/i074_stage2/$3" ]' \
  _ "$RC" "$SR" "$N12_NAME"
KEEP_REAL=1 new_scenario
run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
check "n12：晉升的 failed record 還沒 commit → 下一次 ⑩ 在 preflight 中止（1）、runner 未被呼叫" bash -c '
  [ "$1" = 1 ] && [ ! -s "$2/spy/runner.log" ] && grep -q "先 commit 已晉升的 failed record" "$2/err"' _ "$RC" "$S"
git -C "$SR" add -A "python/baselines/i074_stage2/$N12_NAME"
git -C "$SR" -c user.name=t -c user.email=t@t -c commit.gpgsign=false commit -qm "n12 promoted failed record"
make_freeze "$TD/freeze_n12" "$(git -C "$SR" rev-parse HEAD)"
KEEP_REAL=1 new_scenario
run_entry "$SR" --freeze-record "$TD/freeze_n12/freeze_record.json" --work-dir "$S/work"
check "n12：commit 之後換新的 freeze record → 通過 preflight、進 replay、晉升 0" bash -c '
  [ "$1" = 0 ] && [ -s "$2/spy/runner.log" ]' _ "$RC" "$S"
git -C "$SR" reset -q --hard "$HEAD_OID"

echo "==> i074 Stage 2 ⑦b：supervisor（並行、sentinel、訊號、殘留）"
start_bg() {  # 背景執行正式命令（setsid：收尾以 process group 為準）；$1＝repo；其餘＝參數 → BG_PID
  local repo="$1"
  shift
  env -i HOME="$HOME" PATH="$TEST_PATH" XDG_DATA_HOME="${BG_XDG:-$XDG}" REPLAY_IMAGE_ID="$IMG" I074_TEST_SCENARIO="$S" \
    setsid "$repo/scripts/run-i074-stage2.sh" "$@" > "$S/out" 2> "$S/err" &
  BG_PID=$!
  PIDS+=("$BG_PID")
}
# 執行中拿不到鎖；不同 XDG_DATA_HOME、不同 repo 同時啟動 → 後到的 1；結束後立刻拿得到。
new_scenario; mkfifo "$S/barrier"; S1="$S"
start_bg "$SR" --freeze-record "$FR" --work-dir "$S/work"
wait_for "$S1/at_barrier" || fail "barrier 沒有到達"
if lock_free; then fail "n7b：執行期間鎖竟然是空的"; else pass "n7b：orchestrator 執行期間，獨立 fd 拿不到鎖"; fi
new_scenario; BG_XDG="$TD/xdg-other" run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
check "n7b：不同 XDG_DATA_HOME 同時啟動 → 1（同一把鎖）" bash -c '[ "$1" = 1 ] && [ ! -e "$2/work" ]' _ "$RC" "$S"
SR2="$TD/real2"; git clone -q "$SR" "$SR2"; git -C "$SR2" checkout -q "$HEAD_OID"
new_scenario; run_entry "$SR2" --freeze-record "$FR" --work-dir "$S/work"
check "n7b：不同 repo 同時啟動 → 1" bash -c '[ "$1" = 1 ] && [ ! -e "$2/work" ]' _ "$RC" "$S"
new_scenario; run_entry "$SR" --promote --work-dir "$S1/work"
check "n7b：鎖衝突時 --promote → 8" expect_rc 8
# 非後代的程序拿到**完全正確**的協定變數（取自 runner 記下的環境）也⛔ 通過持鎖驗證（ppid 鏈）
new_scenario; RC=0
mapfile -t PROTO < <(sed -n 's/^ENV \(I074_STAGE2_[A-Z_]*=.*\)$/\1/p' "$S1/spy/runner.log")
env -i PATH="$S1/work/bin:$TEST_PATH" HOME="$HOME" REPLAY_IMAGE_ID="$IMG" "${PROTO[@]}" \
  "$S1/work/repo/scripts/run-i074-stage2.sh" --internal-in-clone resume --work-dir "$S1/work" > /dev/null 2> "$S/err" || RC=$?
check "O1：協定變數完全正確、supervisor 活著且持鎖，但呼叫者不是它的後代 → 1（ppid 鏈）" \
  bash -c '[ "$1" = 1 ] && [ "$2" = 6 ] && grep -q "ppid 鏈" "$3"' _ "$RC" "${#PROTO[@]}" "$S/err"
S="$S1"; echo go > "$S1/barrier"; RC=0; wait "$BG_PID" || RC=$?
check "n7b：先到的照常完成（晉升 → 0）" expect_rc 0
check "n7b：正常結束之後 sentinel 已刪" bash -c '[ ! -e "$1/i074-stage2.active" ]' _ "$LOCKDIR"
check "n7b：鎖檔從未被 unlink（仍在）" test -f "$LOCKDIR/i074-stage2.lock"
check "n7b：鎖在正常結束後可取得" lock_free

# 外部 helper ⛔ 延長鎖：supervisor 外部開著鎖檔的程序不影響釋放。
python3 -c 'import os,sys,time; fd=os.open(sys.argv[1], os.O_RDWR); time.sleep(30)' "$LOCKDIR/i074-stage2.lock" & HELPER_PID=$!; PIDS+=("$HELPER_PID")
new_scenario
run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
check "n7b：外部 helper 開著鎖檔也⛔ 不延長鎖（結束後立刻拿得到）" bash -c '[ "$1" = 0 ]' _ "$RC"
check "n7b：外部 helper 仍在時鎖已可取得" lock_free
kill "$HELPER_PID" 2>/dev/null || true
lock_free || fail "外部 helper 之後鎖沒放"

# setsid 的孤兒回到 subreaper：它消失之前⛔ 放鎖（supervisor 結束時它已經不在）。
new_scenario; touch "$S/runner_orphan"
run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
ORPHAN="$(cat "$S/orphan.pid" 2>/dev/null || echo 0)"
check "n7b：setsid 的孤兒在 supervisor 結束之前已被收掉" bash -c '[ "$1" = 0 ] && ! kill -0 "$2" 2>/dev/null' _ "$RC" "$ORPHAN"

# TERM：移除容器 → TERM／KILL → 都消失之前拿不到鎖 → 128＋15。
new_scenario; mkfifo "$S/barrier"; touch "$S/runner_create_container"
start_bg "$SR" --freeze-record "$FR" --work-dir "$S/work"
wait_for "$S/at_barrier" || fail "barrier 沒有到達"
RUNNER_PID="$(cat "$S/runner.pid")"
kill -TERM "$BG_PID"; RC=0; wait "$BG_PID" || RC=$?
check "G1：supervisor 收到 TERM → 143" expect_rc 143
check "n7b：TERM 之後本趟的容器已移除、runner 已結束、sentinel 已刪、鎖已放" bash -c '
  [ -z "$(ls "$1/docker/containers")" ] && ! kill -0 "$2" 2>/dev/null && [ ! -e "$3/i074-stage2.active" ]' _ "$S" "$RUNNER_PID" "$LOCKDIR"
lock_free || fail "TERM 之後鎖沒放"

# 清不空：docker ps 失敗 → ⛔ 不刪 sentinel、⛔ 不放鎖（supervisor 持續回報、不結束）。
new_scenario; mkfifo "$S/barrier"
start_bg "$SR" --freeze-record "$FR" --work-dir "$S/work"
wait_for "$S/at_barrier" || fail "barrier 沒有到達"
touch "$S/docker_ps_fail"
echo go > "$S/barrier"
sleep 3
check "G3：清不空 → supervisor 不結束、sentinel 留下" bash -c 'kill -0 "$1" 2>/dev/null && [ -e "$2/i074-stage2.active" ]' _ "$BG_PID" "$LOCKDIR"
if lock_free; then fail "G3：清不空時鎖竟然放了"; else pass "G3：清不空 → 鎖不放"; fi
check "G3：持續回報" bash -c 'grep -q "不刪 sentinel" "$1"' _ "$S/err"
kill -KILL -- "-$BG_PID" 2>/dev/null || true; wait "$BG_PID" 2>/dev/null || true
reset_sentinel

# SIGKILL ＋ deterministic barrier（「三」#11）。
new_scenario; mkfifo "$S/barrier"; touch "$S/runner_ignore_term"; SK="$S"
cat > "$S/runner_after_barrier" <<EOF
# ⚠️ fixture 的設定失敗要立刻中止（⛔ 讓後面的步驟掩蓋它）：docker create 失敗 → 記下結束碼、⛔ 繼續。
docker create --name old sha256:0000 true >/dev/null || { echo "\$?" > "$S/after_barrier_create_failed"; exit 1; }
"$S/work/repo/scripts/run-i074-stage2.sh" --internal-in-clone resume --work-dir "$S/work" > "$S/old_resume.out" 2> "$S/old_resume.err"
echo \$? > "$S/old_resume.rc"
EOF
start_bg "$SR" --freeze-record "$FR" --work-dir "$S/work"
wait_for "$SK/at_barrier" || fail "barrier 沒有到達"
kill -KILL "$BG_PID"; RC=0; wait "$BG_PID" || RC=$?
check "G2：supervisor 被 SIGKILL → 137、sentinel 留下" bash -c '[ "$1" = 137 ] && [ -e "$2/i074-stage2.active" ]' _ "$RC" "$LOCKDIR"
# ⚠️ 確定的順序（⑦b 實作第二輪 review 重現的競爭）：舊 orchestrator 收到 parent-death TERM 之後，最多等 5 秒、再以 label 移除
#    本趟的容器——barrier 若在這段期間放開，舊流程建立的容器會被它刪掉。所以先等舊 orchestrator 確實結束，才放開 barrier。
OLD_ORCH="$(cat "$SK/orch.pid")"
for _i in $(seq 1 200); do kill -0 "$OLD_ORCH" 2>/dev/null || break; sleep 0.1; done
check "deterministic barrier：舊 orchestrator 已因 parent-death TERM 結束（之後才放開 barrier）" \
  bash -c '! kill -0 "$1" 2>/dev/null' _ "$OLD_ORCH"
new_scenario; run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
check "n7b：SIGKILL 之後新的 supervisor → 1、提示重開機、⛔ 建立複本" bash -c '
  [ "$1" = 1 ] && grep -q "需要重開機" "$2/err" && [ ! -e "$2/work" ]' _ "$RC" "$S"
new_scenario; run_entry "$SR" --promote --work-dir "$SK/work"
check "n7b：SIGKILL 之後 --promote → 9" expect_rc 9
S="$SK"; echo go > "$SK/barrier"
wait_for "$SK/old_resume.rc" || fail "舊流程沒有走到下一個檢查點"
check "deterministic barrier：舊流程的 docker create 成功（fixture 設定沒有失敗）" test ! -e "$SK/after_barrier_create_failed"
check "deterministic barrier：舊流程的下一個檢查點 /proc/locks 驗證失敗、finalize 未被呼叫" bash -c '
  [ "$(cat "$1/old_resume.rc")" != 0 ] && grep -q "持鎖驗證" "$1/old_resume.err" && ! grep -q -- --finalize "$1/spy/finalizer.log" 2>/dev/null' _ "$SK"
check "deterministic barrier：舊流程建立的容器帶本趟的 label（經 shim）" bash -c 'grep -qx "$2=[0-9a-f]\{64\}" "$1"/docker/containers/*' _ "$SK" "$LABEL"
reset_sentinel
cp -r "$SK/docker/containers/." "$TD/residual.saved" 2>/dev/null || true
new_scenario; cp -r "$SK/docker/containers/." "$S/docker/containers/"
run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
check "n7b：模擬重開機之後仍有帶鍵的容器 → 1、⛔ 留下 sentinel" bash -c '[ "$1" = 1 ] && [ ! -e "$2/i074-stage2.active" ] && [ ! -e "$3/work" ]' _ "$RC" "$LOCKDIR" "$S"
new_scenario; cp -r "$SK/docker/containers/." "$S/docker/containers/"
run_entry "$SR" --promote --work-dir "$SK/work"
check "n7b：殘留容器時 --promote → 8" expect_rc 8
new_scenario
run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
check "n7b：移除殘留容器之後直接重跑成功（⛔ 不需要重開機）" expect_rc 0
new_scenario; touch "$S/docker_ps_fail"
run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
check "S7：docker ps 失敗 → 1、⛔ 留下 sentinel" bash -c '[ "$1" = 1 ] && [ ! -e "$2/i074-stage2.active" ]' _ "$RC" "$LOCKDIR"
new_scenario; printf '{"%s":"x"}\n' "$LABEL" > "$S/docker_labels"
run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
check "S6：image 的 Config.Labels 含鍵 → 1" bash -c '[ "$1" = 1 ] && [ ! -e "$2/work" ]' _ "$RC" "$S"
new_scenario; touch "$S/docker_inspect_fail"
run_entry "$SR" --promote --work-dir "$SK/work"
check "S6：image inspect 失敗 → 8（promote）" expect_rc 8
# sentinel 讀不懂 → 照樣拒絕；⛔ 沒有解除入口。
printf 'garbage' > "$LOCKDIR/i074-stage2.active"
new_scenario; run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
check "S5：sentinel 讀不懂 → 照樣拒絕（1）、內容⛔ 被改" bash -c '[ "$1" = 1 ] && [ "$(cat "$2/i074-stage2.active")" = garbage ]' _ "$RC" "$LOCKDIR"
RC=0; env -i PATH="$TEST_PATH" HOME="$HOME" REPLAY_IMAGE_ID="$IMG" /usr/bin/python3 -I "$SR/scripts/lib/i074-stage2-supervisor.py" unlock \
  > /dev/null 2>&1 || RC=$?
check "n7b：⛔ 沒有解除入口（unlock 之類的模式一律拒絕、sentinel 不動）" bash -c '[ "$1" = 1 ] && [ "$(cat "$2/i074-stage2.active")" = garbage ]' _ "$RC" "$LOCKDIR"
reset_sentinel
# 直接執行持鎖階段／複本 orchestrator → 在完整性檢查之前中止。
new_scenario; RC=0
env -i PATH="$SK/work/bin:$TEST_PATH" HOME="$HOME" REPLAY_IMAGE_ID="$IMG" "$SK/work/repo/scripts/run-i074-stage2.sh" \
  --internal-in-clone resume --work-dir "$SK/work" > /dev/null 2> "$S/err" || RC=$?
check "O1：直接執行複本 orchestrator（沒有協定變數）→ 1" bash -c '[ "$1" = 1 ] && grep -q "I074_STAGE2_TOKEN" "$2"' _ "$RC" "$S/err"
sleep 30 & FAKE_SUP=$!; PIDS+=("$FAKE_SUP")
new_scenario; RC=0
env -i PATH="$SK/work/bin:$TEST_PATH" HOME="$HOME" REPLAY_IMAGE_ID="$IMG" I074_STAGE2_TOKEN="$TOKEN_T" \
  I074_STAGE2_SUPERVISOR_PID="$FAKE_SUP" I074_STAGE2_SUPERVISOR_START=1 I074_STAGE2_REAL_REPO="$SR" \
  I074_STAGE2_REAL_DOCKER="$TRUSTED/docker" I074_STAGE2_MODE=orchestrator "$SK/work/repo/scripts/run-i074-stage2.sh" \
  --internal-in-clone resume --work-dir "$SK/work" > /dev/null 2> "$S/err" || RC=$?
check "O1：協定變數齊全、但該 pid 沒有持鎖 → 1、在完整性檢查之前中止" bash -c '[ "$1" = 1 ] && grep -q "持鎖驗證" "$2"' _ "$RC" "$S/err"
kill "$FAKE_SUP" 2>/dev/null || true
new_scenario; RC=0
env -i PATH="$TEST_PATH" HOME="$HOME" REPLAY_IMAGE_ID="$IMG" "$SR/scripts/run-i074-stage2.sh" \
  --internal-locked-stage promote --work-dir "$SK/work" > /dev/null 2>&1 || RC=$?
check "L1：直接執行持鎖階段（promote）→ 8" expect_rc 8
# 信任條件不符 → 1／8、沒有取鎖。
rm -f "$LOCKDIR/i074-stage2.lock"
chmod g+w "$TRUSTED/git"
new_scenario; run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
check "S0：固定的 git 群組可寫 → 1、沒有取鎖" bash -c '[ "$1" = 1 ] && [ ! -e "$2/i074-stage2.lock" ]' _ "$RC" "$LOCKDIR"
chmod 755 "$TRUSTED/git"
chmod g+w "$TRUSTED"
new_scenario; run_entry "$SR" --promote --work-dir "$SK/work"
check "S0：信任根的上層目錄群組可寫 → 8（promote）、沒有取鎖" bash -c '[ "$1" = 8 ] && [ ! -e "$2/i074-stage2.lock" ]' _ "$RC" "$LOCKDIR"
chmod 755 "$TRUSTED"

echo "==> i074 Stage 2（⑧ 補齊）：兩個執行目錄同時 --promote、殘留的 worktree 登記之下重跑"
pending_work() {  # 走完 ⑩、第一次晉升時驗證模式失敗（9）→ 終態留在複本裡等晉升；→ 設 PW_S、PW_W
  KEEP_REAL=1 new_scenario; echo 1 > "$S/verify_rc"
  run_entry "$SR" --freeze-record "$FR" --work-dir "$S/work"
  [ "$RC" = 9 ] || fail "準備待晉升的終態：rc=$RC（預期 9）"
  rm -f "$S/verify_rc"; PW_S="$S"; PW_W="$S/work"
}
new_scenario                                                    # 清掉真正 repo 先前晉升的終態
pending_work; P1_S="$PW_S"; P1_W="$PW_W"
pending_work; P2_W="$PW_W"
S="$P1_S"; mkfifo "$S/vbarrier"
cat > "$S/verify_hook" <<EOF
case "\$1" in */.promote-staging-*) touch "$S/at_vbarrier"; cat "$S/vbarrier" > /dev/null ;; esac
EOF
start_bg "$SR" --promote --work-dir "$P1_W"
wait_for "$P1_S/at_vbarrier" || fail "第一個 --promote 沒有走到驗證模式的 barrier"
P1_STAGING="$(ls -d "$SR/python/baselines/i074_stage2"/.promote-staging-* 2>/dev/null || true)"
ln -sfn "$SR" "$TD/real_alias"
KEEP_REAL=1 new_scenario
run_entry "$TD/real_alias" --promote --work-dir "$P2_W"
check "n11：兩個執行目錄同時 --promote（後到的以 symlink 別名寫真正 repo）→ 後到的 8、原因是鎖衝突" bash -c '
  [ "$1" = 8 ] && grep -q "鎖已被另一個 supervisor 持有" "$2/err"' _ "$RC" "$S"
check "n11：後到的⛔ 沒有清掉先到的 staging" bash -c '[ -n "$1" ] && [ -d "$1" ]' _ "$P1_STAGING"
S="$P1_S"; echo go > "$P1_S/vbarrier"; RC=0; wait "$BG_PID" || RC=$?
check "n11：先到的照常完成（晉升 → 0）、⛔ 沒有留下 staging" bash -c '
  [ "$1" = 0 ] && ! ls -d "$2"/.promote-staging-* >/dev/null 2>&1' _ "$RC" "$SR/python/baselines/i074_stage2"
# 合成守門中途被殺留下的 worktree 登記（實體目錄已刪／仍在）→ 下一次 --promote 照樣成功（這一次走 6a）；
# prune 只清實體目錄已消失的那一筆（⑦c 細部計畫 v1 的改寫）。
git -C "$P2_W/repo" worktree add -q --detach "$P2_W/tmp/killed-gone" HEAD
git -C "$P2_W/repo" worktree add -q --detach "$P2_W/tmp/killed-kept" HEAD
rm -rf "$P2_W/tmp/killed-gone"
KEEP_REAL=1 new_scenario
run_entry "$SR" --promote --work-dir "$P2_W"
check "n11：殘留的 worktree 登記之下重跑 --promote → 0（6a）" expect_rc 0
check "n11：實體目錄已消失的登記被 prune、仍在的保留" bash -c '
  l="$(git -C "$1/repo" worktree list --porcelain)"
  ! grep -qxF "worktree $1/tmp/killed-gone" <<< "$l" && grep -qxF "worktree $1/tmp/killed-kept" <<< "$l"' _ "$P2_W"

echo "==> i074 Stage 2 ⑦b：host unittest（Python $(python3 -c 'import sys; print("%d.%d" % sys.version_info[:2])')）"
if PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s "$REPO_ROOT/scripts/tests" -p 'test_i074_stage2_host.py' \
     > "$TD/unittest.log" 2>&1; then
  pass "host unittest：$(grep -E '^Ran [0-9]+ tests' "$TD/unittest.log")"
else
  fail "host unittest"; tail -30 "$TD/unittest.log" >&2
fi

echo "==> i074 Stage 2 ⑦b：隔離"
check "隔離：/run/lock 的 i074-stage2 項目前後不變" test "$(lock_listing)" = "$LOCK_BEFORE"
check "隔離：真正 repo 的 worktree 登記數不變" test "$(git -C "$REPO_ROOT" worktree list --porcelain | grep -c '^worktree ' || true)" = "$WT_BEFORE"

echo
if [ "$fails" -gt 0 ]; then
  echo "==> test-i074-stage2：$fails 項失敗" >&2
  exit 1
fi
echo "==> test-i074-stage2：全部通過"
