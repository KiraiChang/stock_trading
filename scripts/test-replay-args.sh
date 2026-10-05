#!/usr/bin/env bash
# I-100：兩支官方腳本的**參數所有權**與 **argv 組法**驗證。
#
# 為什麼要有這一支：python 測試容器只掛 `python/`（見 python/scripts/test.sh），
# 讀不到 `scripts/`。所以 shell 這一側驗「腳本拒絕使用者傳入注入參數」與
# 「實際組出的 argv == 版控 fixture」，python 那一側再用同一份 fixture 跑 CLI 衝突矩陣。
# ⚠️ 由 `python/scripts/test.sh` 在啟動 pytest 之前呼叫——否則它會變成沒人固定執行的手動項目。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FIXTURE="$REPO_ROOT/python/scripts/fixtures/stage0_argv.json"
# shellcheck source=lib/replay-args.sh
. "$REPO_ROOT/scripts/lib/replay-args.sh"

fails=0
pass() { echo "  ok   $1"; }
fail() { echo "  FAIL $1" >&2; fails=$((fails + 1)); }

echo "==> replay-args：使用者不得傳入注入參數"
for arg in --image-digest --base-commit --tooling-patch-sha256 --source-root --runner-sha256; do
  if replay_args_reject_injected --symbols 2330 "$arg" value 2>/dev/null; then
    fail "$arg 應該被拒絕（空白分隔形式）"
  else
    pass "$arg 被拒絕（空白分隔形式）"
  fi
  if replay_args_reject_injected --symbols 2330 "$arg=value" 2>/dev/null; then
    fail "$arg= 應該被拒絕（等號形式）"
  else
    pass "$arg 被拒絕（等號形式）"
  fi
done

if replay_args_reject_injected --symbols 2330 --limit 1500 2>/dev/null; then
  pass "一般參數照常通過"
else
  fail "一般參數不該被拒絕"
fi

echo "==> replay-args：⛔ 連唯一前綴縮寫都要擋（argparse 會把它展開）"
for abbrev in --image-d --base-c --tooling-p --source-r --runner-s; do
  if replay_args_reject_injected --symbols 2330 "$abbrev" evil 2>/dev/null; then
    fail "$abbrev 應該被拒絕（argparse 會展開成受保護參數）"
  else
    pass "$abbrev 被拒絕"
  fi
  if replay_args_reject_injected --symbols 2330 "$abbrev=evil" 2>/dev/null; then
    fail "$abbrev= 應該被拒絕"
  else
    pass "$abbrev= 被拒絕"
  fi
done

echo "==> replay-args：Stage 0 的 argv 必須與版控 fixture 逐 token 相同"
actual="$(python3 - "$FIXTURE" <<'PY'
import json, subprocess, sys
doc = json.load(open(sys.argv[1], encoding="utf-8"))
i = doc["inputs"]
out = subprocess.run(
    ["bash", "-c", '. scripts/lib/replay-args.sh; replay_args_stage0 "$@"', "_",
     i["model_path"], i["image_digest"], i["runner_sha256"], *i["user_args"]],
    capture_output=True, text=True, check=True).stdout
print(json.dumps([line for line in out.split("\n") if line != ""], ensure_ascii=False))
PY
)"
expected="$(python3 -c 'import json,sys; print(json.dumps(json.load(open(sys.argv[1],encoding="utf-8"))["stage0_argv"], ensure_ascii=False))' "$FIXTURE")"
if [ "$actual" = "$expected" ]; then
  pass "Stage 0 argv 與 fixture 相同"
else
  fail "Stage 0 argv 與 fixture 不同"
  echo "    實際：$actual" >&2
  echo "    fixture：$expected" >&2
fi

echo "==> replay-args：離線模式⛔ 不注入 --model-path"
offline="$(replay_args_offline "sha256:x" "/app" "abc" "def" "ghi" "" --bundle /b --output-dir /o --before-ref main)"
if grep -q -- "--model-path" <<<"$offline"; then
  fail "離線模式不該注入 --model-path（模型在 bundle 裡）"
else
  pass "離線模式沒有 --model-path"
fi
for required in --image-digest --source-root --base-commit --tooling-patch-sha256 --runner-sha256; do
  if grep -q -- "$required" <<<"$offline"; then
    pass "離線模式注入了 $required"
  else
    fail "離線模式少了 $required"
  fi
done
# ⚠️ I-074 Stage 2 ⑦a：第 6 個參數＝反事實 patch 的 SHA；空字串⛔ 不注入，非空時緊接在 --runner-sha256 之後。
grep -qx -- "--counterfactual-patch-sha256" <<<"$offline" \
  && fail "一般路徑（第 6 參數空）竟注入了 --counterfactual-patch-sha256" \
  || pass "一般路徑⛔ 不注入 --counterfactual-patch-sha256"
cf_offline="$(replay_args_offline "sha256:x" "/app" "abc" "def" "ghi" "$(printf 'f%.0s' {1..64})" --bundle /b)"
cf_expected="$(printf '%s\n' python -m backtest.modular.sr_scoring.evaluation --image-digest sha256:x \
  --source-root /app --base-commit abc --tooling-patch-sha256 def --runner-sha256 ghi \
  --counterfactual-patch-sha256 "$(printf 'f%.0s' {1..64})" --bundle /b)"
[ "$cf_offline" = "$cf_expected" ] \
  && pass "反事實 SHA 注入在 --runner-sha256 之後、使用者參數之前" \
  || { fail "反事實 SHA 的注入位置不符"; diff <(printf '%s\n' "$cf_expected") <(printf '%s\n' "$cf_offline") >&2 || true; }
for spoof in --counterfactual-patch-sha256 --counterfactual-patch-sha256=x --counterfactual-p; do
  if replay_args_reject_injected --bundle /b "$spoof" >/dev/null 2>&1; then
    fail "使用者傳入 $spoof 竟未被拒（s：只能由 runner 注入）"
  else
    pass "使用者傳入 $spoof → 拒絕（含縮寫與 =value）"
  fi
done

echo "==> 兩支腳本：使用者傳入注入參數時必須拒絕（spoof）"
for script in run-evaluation.sh run-replay-offline.sh; do
  path="$REPO_ROOT/scripts/$script"
  if [ ! -x "$path" ]; then
    fail "$script 不存在或不可執行"
    continue
  fi
  if ! grep -q "replay_args_reject_injected" "$path"; then
    fail "$script 沒有呼叫 replay_args_reject_injected"
  else
    pass "$script 會拒絕使用者傳入的注入參數"
  fi
  if ! grep -q "replay_args_stage0\|replay_args_offline" "$path"; then
    fail "$script 沒有用共用的 argv builder"
  else
    pass "$script 用的是共用 argv builder"
  fi
done

echo "==> replay-args：離線模式的 stage 判定"
[ "$(replay_args_offline_stage --bundle /b --output-dir /o)" = "1" ] \
  && pass "兩個都沒給 → Stage 1" || fail "兩個都沒給應該是 Stage 1"
[ "$(replay_args_offline_stage --bundle /b --after-artifact /a --cohort-manifest /c)" = "2" ] \
  && pass "兩個都給 → Stage 2" || fail "兩個都給應該是 Stage 2"
[ "$(replay_args_offline_stage --bundle /b --after-artifact /a)" = "3" ] \
  && pass "只給其中一個 → 不完整" || fail "只給其中一個應該回不完整"

echo "==> run-evaluation.sh：spoof 實測（不需要 docker）"
spoof_out="$(REPLAY_ARGS_SELFTEST=1 "$REPO_ROOT/scripts/run-evaluation.sh" \
  --symbols 2330 --image-digest sha256:evil 2>&1 || true)"
if grep -q "只能由官方腳本注入" <<<"$spoof_out"; then
  pass "run-evaluation.sh 拒絕了偽造的 --image-digest"
else
  fail "run-evaluation.sh 沒有拒絕偽造的 --image-digest：$spoof_out"
fi

spoof_out="$(REPLAY_ARGS_SELFTEST=1 "$REPO_ROOT/scripts/run-replay-offline.sh" \
  --bundle /b --output-dir /o --before-ref main --source-root /elsewhere 2>&1 || true)"
if grep -q "只能由官方腳本注入" <<<"$spoof_out"; then
  pass "run-replay-offline.sh 拒絕了偽造的 --source-root"
else
  fail "run-replay-offline.sh 沒有拒絕偽造的 --source-root：$spoof_out"
fi

echo "==> run-evaluation.sh：Stage 0 與 WRITE_DB／MODE 互斥"
out="$(REPLAY_ARGS_SELFTEST=1 WRITE_DB=1 "$REPO_ROOT/scripts/run-evaluation.sh" \
  --emit-bundle /b --symbols 2330 2>&1 || true)"
if grep -q "WRITE_DB" <<<"$out"; then
  pass "Stage 0 與 WRITE_DB=1 互斥"
else
  fail "Stage 0 沒有擋下 WRITE_DB=1：$out"
fi
out="$(REPLAY_ARGS_SELFTEST=1 MODE=sweep "$REPO_ROOT/scripts/run-evaluation.sh" \
  --emit-bundle /b --symbols 2330 2>&1 || true)"
if grep -q "MODE" <<<"$out"; then
  pass "Stage 0 與 MODE=sweep 互斥"
else
  fail "Stage 0 沒有擋下 MODE=sweep：$out"
fi

echo "==> before worktree：TOCTOU 與 tooling patch hash"
TMP_REPO="$(mktemp -d)"
cleanup_repo() { rm -rf "$TMP_REPO"; }
trap cleanup_repo EXIT
(
  cd "$TMP_REPO"
  git init -q .
  git config user.email t@example.com
  git config user.name test
  echo base > a.txt
  git add a.txt
  git commit -qm base
  git branch feature
)
BASE_OID="$(git -C "$TMP_REPO" rev-parse feature)"

# 情境一：解析出 OID 後**移動 branch** → worktree 仍建在原 OID → **應通過**
WT1="$TMP_REPO/../wt1-$$"
(
  cd "$TMP_REPO"
  echo more >> a.txt
  git commit -qam second
  git branch -f feature HEAD
) >/dev/null
if oid="$(replay_args_prepare_worktree "$TMP_REPO" "$BASE_OID" "$WT1" 2>/dev/null)" \
   && [ "$oid" = "$BASE_OID" ]; then
  pass "解析後移動 branch：worktree 仍在原 OID"
else
  fail "解析後移動 branch 應該通過"
fi

# 情境二：人為改動 detached worktree 的 HEAD → `head != oid` → **中止**
git -C "$TMP_REPO" worktree remove --force "$WT1" >/dev/null 2>&1 || true
WT2="$TMP_REPO/../wt2-$$"
git -C "$TMP_REPO" worktree add --detach "$WT2" "$BASE_OID" >/dev/null 2>&1
git -C "$WT2" checkout -q "$(git -C "$TMP_REPO" rev-parse feature)" 2>/dev/null || true
if replay_args_prepare_worktree "$TMP_REPO" "$BASE_OID" "$WT2" >/dev/null 2>&1; then
  fail "worktree 目標已存在／HEAD 被改動時應該中止"
else
  pass "worktree HEAD 被改動時中止"
fi
git -C "$TMP_REPO" worktree remove --force "$WT2" >/dev/null 2>&1 || true

# tooling patch：只**新增一個檔案**的 patch 也必須改變 hash
WT3="$TMP_REPO/../wt3-$$"
replay_args_prepare_worktree "$TMP_REPO" "$BASE_OID" "$WT3" >/dev/null
clean_hash="$(replay_args_tooling_patch_sha256 "$WT3" "$BASE_OID")"
git -C "$TMP_REPO" worktree remove --force "$WT3" >/dev/null 2>&1 || true

PATCH="$TMP_REPO/add-file.patch"
cat > "$PATCH" <<'PATCHEOF'
diff --git a/new_tool.py b/new_tool.py
new file mode 100644
index 0000000..7898192
--- /dev/null
+++ b/new_tool.py
@@ -0,0 +1 @@
+a
PATCHEOF
WT4="$TMP_REPO/../wt4-$$"
replay_args_prepare_worktree "$TMP_REPO" "$BASE_OID" "$WT4" >/dev/null
patched_hash="$(replay_args_tooling_patch_sha256 "$WT4" "$BASE_OID" "$PATCH")"
git -C "$TMP_REPO" worktree remove --force "$WT4" >/dev/null 2>&1 || true
if [ "$clean_hash" != "$patched_hash" ]; then
  pass "只新增一個檔案的 patch 也改變了 tooling_patch_sha256"
else
  fail "新增檔案沒有進 diff——tooling patch 沒被完整涵蓋"
fi

# ⚠️ **untracked 守門的 SIGPIPE 迴歸**（2026-09-21 review）：舊寫法
# `git status --porcelain | grep -q '^??'` 在 `pipefail` 下有競爭——`grep -q` 一找到就退出、
# `git` 收到 SIGPIPE，整條 pipeline 回非零，於是 `if` 走不進去、**守門反而被繞過**。
#
# ⛔ **⛔ 不能用「在 worktree 裡放一堆檔案」來測**：那樣測到的只是 git 的行為，⛔ 不是守門的比對模式。
# ⚠️ **要讓 helper 本身收到大量 `??`**（⛔ 不是只測旁邊的 grep）：用 PATH 注入假 `git`，讓
# `status --porcelain` 吐 5000 行 untracked。⚠️ ⑦a 起守門在 `replay_args_compose()` 裡，它先驗
# 「HEAD ＝ base、index ＝ base 的 tree」——所以假 `git` 對 `rev-parse`／`write-tree` 一律回同一個 OID，
# 讓流程**真的走到** status 那一道；其餘子命令成功且無輸出。
FAKE_GIT_DIR="$TMP_REPO/../fakegit-$$"
mkdir -p "$FAKE_GIT_DIR"
cat > "$FAKE_GIT_DIR/git" <<'FAKEGIT'
#!/usr/bin/env bash
# ⚠️ 只為這條測試存在：讓 `status --porcelain` 產生大量 ?? 行。
for a in "$@"; do
  case "$a" in
    status) for i in $(seq 1 5000); do printf '?? many/file_%s.txt\n' "$i"; done; exit 0 ;;
    rev-parse|write-tree) printf '%040d\n' 7; exit 0 ;;
  esac
done
exit 0
FAKEGIT
chmod +x "$FAKE_GIT_DIR/git"
WT5="$TMP_REPO/../wt5-$$"
mkdir -p "$WT5"
set +e
many_out="$(PATH="$FAKE_GIT_DIR:$PATH" replay_args_tooling_patch_sha256 "$WT5" "$BASE_OID" 2>&1)"
many_rc=$?
set -e
rm -rf "$WT5" "$FAKE_GIT_DIR"
if [ "$many_rc" -ne 0 ] && grep -q "仍有 untracked" <<< "$many_out"; then
  pass "helper 收到 5000 行 ?? 時確實回非零並報漏檔（⛔ 無 SIGPIPE fail-open）"
else
  fail "大量 untracked 未被 helper 擋下：rc=$many_rc"
fi

# ⚠️ 靜態確認產品端⛔ 不是用 pipe——用 `-F` 字面比對，
# ⛔ 不要用 ERE：`\|` 在不同 grep 實作下語意不一致（本機是 ugrep），
# pattern 不匹配會讓這條斷言**永遠是綠的**（2026-09-21 反向驗證抓到）。
if grep -qF 'status --porcelain | grep -q' "$REPO_ROOT/scripts/lib/replay-args.sh"; then
  fail "lib 仍用 git status | grep -q（⛔ pipefail 下的 SIGPIPE 競爭）"
else
  pass "lib 的 untracked 守門⛔ 不經 pipe"
fi

# ⚠️ `git status` 自己失敗時也⛔ 不得靜默放行。
WT6="$TMP_REPO/../wt6-$$"
mkdir -p "$WT6"                      # ⛔ 不是 git worktree，status 會失敗
set +e
bad_out="$(replay_args_tooling_patch_sha256 "$WT6" "$BASE_OID" 2>&1)"; bad_rc=$?
set -e
rm -rf "$WT6"
[ "$bad_rc" -ne 0 ] && pass "git status 失敗 → 非零（⛔ 不靜默放行）" \
  || fail "git status 失敗竟然回 0"

echo "==> run-replay-offline.sh：結構性離線"
offline_script="$REPO_ROOT/scripts/run-replay-offline.sh"
if grep -q -- "--network none" "$offline_script"; then
  pass "離線容器用 --network none"
else
  fail "離線容器沒有 --network none"
fi
if grep -qE '^\s*-e (DATABASE_DSN|DATABASE_DRIVER)' "$offline_script"; then
  fail "離線容器不該注入任何 DB 環境變數"
else
  pass "離線容器沒有 DB 環境變數"
fi
if grep -q ':/app:ro' "$offline_script"; then
  pass "程式碼是唯讀掛載"
else
  fail "程式碼不是唯讀掛載"
fi

echo "==> run-replay-offline.sh：stage 決定跑哪一份程式碼、bundle 與 artifact 要掛得進去"
# ⚠️ 這一組是 findings 1／2 的回歸測試：只 grep 腳本內容抓不到「Stage 1 跑成 before」
# 或「bundle 沒掛進容器」，一定要看**實際組出來的 docker argv**。
DRY_BUNDLE="$(mktemp -d)"
DRY_STAGE1="$(mktemp -d)"
DRY_OUT="$(mktemp -d)"
cleanup_dry() { rm -rf "$DRY_BUNDLE" "$DRY_STAGE1" "$DRY_OUT"; }
trap 'cleanup_repo; cleanup_dry' EXIT
: > "$DRY_STAGE1/after_artifact.json"
: > "$DRY_STAGE1/cohort_manifest.json"

HEAD_OID="$(git -C "$REPO_ROOT" rev-parse HEAD)"
PARENT_OID="$(git -C "$REPO_ROOT" rev-parse HEAD~1)"

stage1_argv="$(REPLAY_DRY_RUN=1 AFTER_REF="$HEAD_OID" "$REPO_ROOT/scripts/run-replay-offline.sh" \
  --bundle "$DRY_BUNDLE" --output-dir "$DRY_OUT" --before-ref "$PARENT_OID" 2>/dev/null | tr '\n' ' ' || true)"
if grep -q -- "--base-commit $HEAD_OID" <<<"$stage1_argv"; then
  pass "Stage 1 跑的是 after 版本（base_commit = AFTER_REF）"
else
  fail "Stage 1 沒有跑 after 版本：$(grep -o -- '--base-commit [0-9a-f]*' <<<"$stage1_argv")"
fi
if grep -q "$DRY_BUNDLE:$DRY_BUNDLE:ro" <<<"$stage1_argv"; then
  pass "Stage 1 把 bundle 唯讀掛進容器"
else
  fail "Stage 1 沒有掛載 bundle"
fi
if grep -q -- "--runner-sha256" <<<"$stage1_argv"; then
  pass "Stage 1 注入了 runner_sha256"
else
  fail "Stage 1 少了 runner_sha256"
fi

stage2_argv="$(REPLAY_DRY_RUN=1 AFTER_REF="$HEAD_OID" "$REPO_ROOT/scripts/run-replay-offline.sh" \
  --bundle "$DRY_BUNDLE" --output-dir "$DRY_OUT" --before-ref "$PARENT_OID" \
  --after-artifact "$DRY_STAGE1/after_artifact.json" \
  --cohort-manifest "$DRY_STAGE1/cohort_manifest.json" 2>/dev/null | tr '\n' ' ' || true)"
if grep -q -- "--base-commit $PARENT_OID" <<<"$stage2_argv"; then
  pass "Stage 2 跑的是 before 版本（base_commit = BEFORE_REF）"
else
  fail "Stage 2 沒有跑 before 版本"
fi
if grep -q "$DRY_STAGE1:$DRY_STAGE1:ro" <<<"$stage2_argv"; then
  pass "Stage 2 把 Stage 1 的 artifact 目錄唯讀掛進容器"
else
  fail "Stage 2 沒有掛載 Stage 1 的 artifact"
fi
if grep -q "$DRY_BUNDLE:$DRY_BUNDLE:ro" <<<"$stage2_argv"; then
  pass "Stage 2 把 bundle 唯讀掛進容器"
else
  fail "Stage 2 沒有掛載 bundle"
fi
if [ "$(grep -c -- "--network none" <<<"$stage2_argv")" -ge 1 ]; then
  pass "離線容器實際 argv 含 --network none"
else
  fail "離線容器實際 argv 沒有 --network none"
fi
if grep -qE -- "-e +(DATABASE_DSN|DATABASE_DRIVER)" <<<"$stage2_argv"; then
  fail "離線容器實際 argv 出現 DB 環境變數"
else
  pass "離線容器實際 argv 沒有 DB 環境變數"
fi

echo "==> run-replay-offline.sh：⛔ 會影響 checkout／掛載的參數不得重複"
BASE_ARGS=(--bundle "$DRY_BUNDLE" --output-dir "$DRY_OUT" --before-ref A)
STAGE2_ARGS=("${BASE_ARGS[@]}"
  --after-artifact "$DRY_STAGE1/after_artifact.json"
  --cohort-manifest "$DRY_STAGE1/cohort_manifest.json")
for dup in --before-ref --bundle --output-dir --after-artifact --cohort-manifest; do
  case "$dup" in
    --before-ref)      extra=("${BASE_ARGS[@]}" --before-ref B) ;;
    --bundle)          extra=("${BASE_ARGS[@]}" --bundle /other) ;;
    --output-dir)      extra=("${BASE_ARGS[@]}" --output-dir /other) ;;
    --after-artifact)  extra=("${STAGE2_ARGS[@]}" --after-artifact /other.json) ;;
    --cohort-manifest) extra=("${STAGE2_ARGS[@]}" --cohort-manifest /other.json) ;;
  esac
  if REPLAY_DRY_RUN=1 "$REPO_ROOT/scripts/run-replay-offline.sh" "${extra[@]}" >/dev/null 2>&1; then
    fail "$dup 重複時應該中止（腳本取第一個、argparse 取最後一個）"
  else
    pass "$dup 重複時中止"
  fi
done

echo "==> run-replay-offline.sh：只給其中一個 artifact 要在建 worktree 之前中止"
if REPLAY_DRY_RUN=1 "$REPO_ROOT/scripts/run-replay-offline.sh" \
     --bundle "$DRY_BUNDLE" --output-dir "$DRY_OUT" --before-ref "$PARENT_OID" \
     --after-artifact "$DRY_STAGE1/after_artifact.json" >/dev/null 2>&1; then
  fail "只給 --after-artifact 應該中止"
else
  pass "只給 --after-artifact 會中止"
fi

echo "==> run-evaluation.sh：driver 可覆寫（手動驗 mysql 快照要用）"
eval_argv="$(REPLAY_DRY_RUN=1 DB_DRIVER=mysql DB_DSN="mysql+pymysql://u:p@h/db" \
  MODELS_DIR="$REPO_ROOT/python" NETWORK=bridge \
  "$REPO_ROOT/scripts/run-evaluation.sh" --symbols 2330 2>/dev/null | tr '\n' ' ' || true)"
if grep -q "DATABASE_DRIVER=mysql" <<<"$eval_argv"; then
  pass "DB_DRIVER=mysql 有傳進容器"
else
  fail "DB_DRIVER 沒有生效（手動驗 mysql 會跑不起來）"
fi

echo "==> run-replay-offline.sh：專屬結束碼 4 必須原樣傳出"
# ⚠️ smoke 兩側都跑同一個 HEAD，正常⛔ 不會 mismatch，驗不到這條——所以用 fake docker。
# ⛔ **fake 不能「一律 exit 4」**：腳本的順序是 build → image inspect → exec run，
# 一律回 4 的話**在 build 就退出**，測試看到 4 卻完全沒驗到 passthrough（false pass）。
FAKE_BIN="$(mktemp -d)"
FAKE_LOG="$FAKE_BIN/calls.log"
cat > "$FAKE_BIN/docker" <<'FAKEEOF'
#!/usr/bin/env bash
echo "$1 $2" >> "$FAKE_DOCKER_LOG"
case "$1" in
  build) exit 0 ;;
  image)
    # image inspect → 印出合法 digest 並回 0
    printf 'sha256:%064d\n' 0
    exit 0 ;;
  run) exit 4 ;;   # ← 模擬 CLI 回 EXIT_CANDIDATE_MISMATCH
  *) exit 0 ;;
esac
FAKEEOF
chmod +x "$FAKE_BIN/docker"

DRY_BUNDLE2="$(mktemp -d)"
DRY_OUT2="$(mktemp -d)"
set +e
FAKE_DOCKER_LOG="$FAKE_LOG" PATH="$FAKE_BIN:$PATH" AFTER_REF="$HEAD_OID" \
  "$REPO_ROOT/scripts/run-replay-offline.sh" \
    --bundle "$DRY_BUNDLE2" --output-dir "$DRY_OUT2" --before-ref "$PARENT_OID" >/dev/null 2>&1
exit4_code=$?
set -e

if [ "$exit4_code" -eq 4 ]; then
  pass "run-replay-offline.sh 原樣傳出 exit 4"
else
  fail "exit code 被吞掉了：預期 4，實際 $exit4_code"
fi
# ⚠️ ⛔ 只看結束碼不夠——要確認真的走到 docker run，否則 build 階段就退出也會看到 4。
if grep -q "^run " "$FAKE_LOG" 2>/dev/null; then
  pass "確實執行到 docker run（⛔ 不是在 build 階段就退出）"
else
  fail "沒有執行到 docker run——這個 4 是別的階段回的"
fi
rm -rf "$FAKE_BIN" "$DRY_BUNDLE2" "$DRY_OUT2"

echo "==> run-replay-offline.sh：I-074 Stage 2 的結束碼 6（反事實沒有生效）也必須原樣傳出"
# ⚠️ 同上：fake docker 只在 `run` 回 6；另驗真的走到 docker run。
# ⚠️ 走到 `exec docker run` 時 runner 的 EXIT trap 不會執行，worktree 會留下（I-118 的既有成因）——這支⑦a 新增的
# 測試自己收掉它造成的登記。⛔ **不用「前後差集」**（⑦a 實作第一輪 review）：同時段別人建立的 worktree 也會被誤刪。
# runner 以本次專屬的 TMPDIR 執行，只清這個目錄底下的登記（`replay_args_remove_worktrees_under()`）。
# 為了證明這一點，fake docker 在 runner **執行途中**另建一個無關的 worktree（放在 TMPDIR 以外），結束後它必須還在。
FAKE_BIN="$(mktemp -d)"
FAKE_LOG="$FAKE_BIN/calls.log"
cat > "$FAKE_BIN/docker" <<'FAKEEOF'
#!/usr/bin/env bash
echo "$1 $2" >> "$FAKE_DOCKER_LOG"
case "$1" in
  build) exit 0 ;;
  image) printf 'sha256:%064d\n' 0; exit 0 ;;
  run)
    # 模擬「同一段時間內的另一個程序」建立 worktree（⚠️ 在 runner 的 TMPDIR 以外）。
    git -C "$FAKE_REPO" worktree add -q --detach "$FAKE_OTHER_WT" HEAD >/dev/null 2>&1
    exit 6 ;;   # ← 模擬 CLI 回 EXIT_COUNTERFACTUAL_INEFFECTIVE
  *) exit 0 ;;
esac
FAKEEOF
chmod +x "$FAKE_BIN/docker"
DRY_BUNDLE2="$(mktemp -d)"
DRY_OUT2="$(mktemp -d)"
EXIT6_TMP="$(mktemp -d)"                       # runner 本次專屬的 TMPDIR（⚠️ 新建、只給這一次）
EXIT6_OTHER="$(mktemp -d)/unrelated-wt"        # 無關的 worktree（TMPDIR 以外）
set +e
FAKE_DOCKER_LOG="$FAKE_LOG" FAKE_REPO="$REPO_ROOT" FAKE_OTHER_WT="$EXIT6_OTHER" PATH="$FAKE_BIN:$PATH" \
  TMPDIR="$EXIT6_TMP" AFTER_REF="$HEAD_OID" "$REPO_ROOT/scripts/run-replay-offline.sh" \
    --bundle "$DRY_BUNDLE2" --output-dir "$DRY_OUT2" --before-ref "$PARENT_OID" >/dev/null 2>&1
exit6_code=$?
set -e
[ "$exit6_code" -eq 6 ] && grep -q "^run " "$FAKE_LOG" 2>/dev/null \
  && pass "run-replay-offline.sh 原樣傳出 exit 6（且確實走到 docker run）" \
  || fail "exit 6 被吞掉或沒走到 docker run：實際 $exit6_code"
exit6_list() { git -C "$REPO_ROOT" worktree list --porcelain | sed -n 's/^worktree //p'; }
grep -qF -- "$EXIT6_TMP/" <<< "$(exit6_list)" \
  && pass "前提：runner 確實在專屬的 TMPDIR 底下留下了 worktree（⛔ 不是空測）" \
  || fail "前提不成立：專屬 TMPDIR 底下沒有 worktree 登記"
replay_args_remove_worktrees_under "$REPO_ROOT" "$EXIT6_TMP" \
  && pass "清理函式回 0（範圍內的登記都移除了）" || fail "清理函式回報失敗"
if grep -qF -- "$EXIT6_TMP/" <<< "$(exit6_list)"; then
  fail "exit 6 的測試沒有收掉專屬 TMPDIR 底下的 worktree"
else
  pass "exit 6 的測試收掉了自己（專屬 TMPDIR 底下）的 worktree 登記"
fi
if grep -qxF -- "$EXIT6_OTHER" <<< "$(exit6_list)" && [ -d "$EXIT6_OTHER" ]; then
  pass "執行期間另建的無關 worktree（範圍外）⛔ 沒有被刪"
else
  fail "清理誤刪了範圍外的 worktree"
fi
git -C "$REPO_ROOT" worktree remove --force "$EXIT6_OTHER" >/dev/null 2>&1 || true
rm -rf "$FAKE_BIN" "$DRY_BUNDLE2" "$DRY_OUT2" "$EXIT6_TMP" "$(dirname "$EXIT6_OTHER")"

if [ "$fails" -ne 0 ]; then
  echo "==> replay-args 測試失敗：$fails 項" >&2
  exit 1
fi
# ── 12-B：host 端 run identity validator ───────────────────────────────────
#
# ⚠️ 它在 **host**（沒有 pandas）以 Docker **之前**的守門身分執行，所以這一組要證明：
# 兩種格式都收、契約（stdout／exit code）成立、**⛔ 不依賴 site-packages**，
# 而且 `--bundle` 這條新路徑（v23 加的 `load_bundle()`）真的跑得動。
echo "==> i074：host 端 run identity validator"
I074_TD="$(mktemp -d)"
I074_ID="$I074_TD/xdg/stock_trading/i074_stage1/run_identity.json"
I074_IMG="sha256:$(printf 'a%.0s' $(seq 64))"
I074_BUNDLE="$REPO_ROOT/python/baselines/b1_20260901_1d_74350966_5d7ecb10"
ENSURE="$REPO_ROOT/python/scripts/ensure-i074-run-identity.py"
VALIDATE="$REPO_ROOT/python/scripts/validate-i074-run-identity.py"

if [ -d "$I074_BUNDLE" ]; then
  XDG_DATA_HOME="$I074_TD/xdg" python3 "$ENSURE" --bundle "$I074_BUNDLE" --image-id "$I074_IMG" >/dev/null 2>&1 \
    && pass "ensure 建立 run identity" || fail "ensure 建立 run identity"

  # ⚠️ **`python3 -S`**：site-packages ⛔ 不在 sys.path，證明整條路徑 dependency-light。
  # ⚠️ 且**必須同時帶 `--bundle`**——只測不帶的路徑會全綠，卻測不到 v23 才加進閉包的
  # `bundle`／`calendar` 這兩個 import。
  out="$(python3 -S "$VALIDATE" "$I074_ID" --expect-image-id "$I074_IMG" --bundle "$I074_BUNDLE" 2>/dev/null)"
  if [ "$out" = "$I074_IMG" ]; then
    pass "python3 -S ＋ --bundle：通過且 stdout 只有 image ID"
  else
    fail "python3 -S ＋ --bundle 的 stdout 是 '$out'"
  fi

  # ⛔ 不傳 --bundle 也不得失敗（comparator／finalizer／recovery 的用法）。
  python3 -S "$VALIDATE" "$I074_ID" --expect-image-id "$I074_IMG" >/dev/null 2>&1 \
    && pass "不傳 --bundle 仍通過" || fail "不傳 --bundle 竟然失敗"

  # image id 不符 → 非零且 **stdout 無輸出**。
  set +e
  out="$(python3 -S "$VALIDATE" "$I074_ID" --expect-image-id "sha256:$(printf 'b%.0s' $(seq 64))" 2>/dev/null)"; rc=$?
  set -e
  if [ "$rc" -ne 0 ] && [ -z "$out" ]; then
    pass "image id 不符 → 非零且 stdout 無輸出"
  else
    fail "image id 不符：rc=$rc stdout='$out'"
  fi

  # --bundle 不符 → 非零（validator 自己 load_bundle 取 ID 再比對）。
  set +e
  out="$(python3 -S "$VALIDATE" "$I074_ID" --expect-image-id "$I074_IMG" --bundle "$I074_TD" 2>/dev/null)"; rc=$?
  set -e
  if [ "$rc" -ne 0 ] && [ -z "$out" ]; then
    pass "--bundle 不符 → 非零且 stdout 無輸出"
  else
    fail "--bundle 不符：rc=$rc"
  fi

  # 壞 schema → 非零。
  echo '{"bad":1}' > "$I074_TD/broken.json"
  set +e
  out="$(python3 -S "$VALIDATE" "$I074_TD/broken.json" --expect-image-id "$I074_IMG" 2>/dev/null)"; rc=$?
  set -e
  if [ "$rc" -ne 0 ] && [ -z "$out" ]; then
    pass "壞 schema → 非零且 stdout 無輸出"
  else
    fail "壞 schema：rc=$rc"
  fi

  # ⚠️ **peek 的三條分支**：不存在 → exit 2（⛔ 不是失敗，是「還沒 pin」）。
  set +e
  XDG_DATA_HOME="$I074_TD/empty" python3 "$ENSURE" --bundle "$I074_BUNDLE" --peek >/dev/null 2>&1; rc=$?
  set -e
  [ "$rc" -eq 2 ] && pass "peek 於 identity 不存在時回 exit 2" || fail "peek 不存在時 rc=$rc（預期 2）"
else
  echo "  skip 找不到正式 bundle，略過 host validator 測試" >&2
fi
rm -rf "$I074_TD"

# ⚠️ **建立 run identity 的失敗⛔ 不得被吞掉**。
# 這一步失敗（例如剛 docker build 完、host 記憶體吃緊）而 rc 沒被檢查時，identity
# 不會存在，後續被測腳本整支 exit 非零，於是**下游出現一整排看似隨機的斷言失敗**
# ——真正的原因卻因為 `2>&1 >/dev/null` 完全看不到。實例見 docs/issue.md I-074。
ensure_identity() {
  local xdg="$1" bundle="$2" image="$3" label="$4" err rc
  err="$(XDG_DATA_HOME="$xdg" python3 "$REPO_ROOT/python/scripts/ensure-i074-run-identity.py" \
      --bundle "$bundle" --image-id "$image" 2>&1 >/dev/null)"; rc=$?
  if [ "$rc" -ne 0 ]; then
    fail "$label：建立 run identity 失敗（rc=$rc）——以下斷言的失敗都源自這裡"
    printf '%s\n' "$err" | sed 's/^/    /' >&2
    return 1
  fi
  if [ ! -f "$xdg/stock_trading/i074_stage1/run_identity.json" ]; then
    fail "$label：ensure 回 0 但 identity 檔不存在"
    return 1
  fi
  return 0
}

# ── 12-A：comparator 的 argv／mount／ownership ──────────────────────────────
# ⚠️ **呼叫被測腳本時要 `env -u PY_IMAGE`**：`PY_IMAGE` 只用來查剛建好的 image ID，
# 而被測腳本**刻意禁止** `PY_IMAGE` 與 `REPLAY_IMAGE_ID` 併用（兩個 image 來源）。
echo "==> i074：comparator 的 argv／mount／所有權"
CMP_TD="$(mktemp -d)"
CMP_FIXTURE="$REPO_ROOT/python/scripts/fixtures/comparator_argv.json"
CMP_IMG="$(docker image inspect "${PY_IMAGE:-stock-trading-python-test:latest}" -f '{{.Id}}' 2>/dev/null || true)"
CMP_BUNDLE="$REPO_ROOT/python/baselines/b1_20260901_1d_74350966_5d7ecb10"

if [ -n "$CMP_IMG" ] && [ -d "$CMP_BUNDLE" ]; then
  CMP_ID="$CMP_TD/xdg/stock_trading/i074_stage1/run_identity.json"
  # ⚠️ 覆寫 `XDG_DATA_HOME`——⛔ 沒有「只給測試用」的路徑參數（那是強制不了的後門）。
  ensure_identity "$CMP_TD/xdg" "$CMP_BUNDLE" "$CMP_IMG" comparator || true
  : > "$CMP_TD/d.json"; : > "$CMP_TD/d1.json"

  set +e
  CMP_OUT="$(env -u PY_IMAGE REPLAY_DRY_RUN=1 REPLAY_IMAGE_ID="$CMP_IMG" XDG_DATA_HOME="$CMP_TD/xdg" \
      "$REPO_ROOT/scripts/compare-replay-crossday.sh" \
      --d "$CMP_TD/d.json" --d1 "$CMP_TD/d1.json" --output-dir "$CMP_TD/out" 2>/dev/null)"
  set -e

  # ⚠️ **argv 逐 token 比對**：把動態值換成 placeholder 再和 fixture 比。
  # ⛔ 只驗行為而不比對 argv，等於沒有釘住兩端的契約。
  ACTUAL="$(printf '%s\n' "$CMP_OUT" | sed -n '/^python$/,$p' \
    | sed -e "s|^$CMP_TD/d\.json\$|<D>|" -e "s|^$CMP_TD/d1\.json\$|<D1>|" \
          -e "s|^$CMP_TD/out\$|<OUT>|" -e "s|^$CMP_ID\$|<IDENTITY>|" \
          -e "s|^sha256:[0-9a-f]\{64\}\$|<IMAGE_ID>|" \
          -e "s|^[0-9a-f]\{40\}\$|<BASE_COMMIT>|" \
    | python3 -c 'import sys;print(chr(10).join(l.rstrip(chr(10)) for l in sys.stdin))')"
  EXPECTED="$(python3 -c '
import json,sys,re
fx=json.load(open(sys.argv[1],encoding="utf-8"))["comparator_argv"]
print("\n".join(fx))' "$CMP_FIXTURE")"
  # 兩個 64-hex（tooling patch／runner）在正規化後仍是實際值，逐行換成 placeholder。
  ACTUAL="$(printf '%s\n' "$ACTUAL" | awk '
    prev=="--tooling-patch-sha256"{print "<TOOLING_PATCH_SHA256>"; prev=$0; next}
    prev=="--runner-sha256"{print "<RUNNER_SHA256>"; prev=$0; next}
    {print; prev=$0}')"
  if [ "$ACTUAL" = "$EXPECTED" ]; then
    pass "comparator argv 與 fixture 逐 token 相同"
  else
    fail "comparator argv 與 fixture 不符"
    diff <(printf '%s\n' "$EXPECTED") <(printf '%s\n' "$ACTUAL") >&2 || true
  fi

  # ⚠️ **same-path 且 `:ro`** 的掛載。
  for target in "$CMP_TD/d.json" "$CMP_TD/d1.json" "$CMP_ID"; do
    if grep -qx -- "$target:$target:ro" <<< "$CMP_OUT"; then
      pass "same-path :ro 掛載 $(basename "$target")"
    else
      fail "$(basename "$target") 不是 same-path :ro 掛載"
      # ⚠️ **失敗要印出可比對的證據**——⛔ 不能只靠重跑就宣告收斂：
      # 這條曾出現過「完整跑失敗、單獨重跑通過」的非決定性結果。
      echo "    預期 mount : $target:$target:ro" >&2
      echo "    實際 -v    :" >&2
      grep -A1 -x -- '-v' <<< "$CMP_OUT" | grep -v '^-v$\|^--$' | sed 's/^/      /' >&2
      echo "    dry-run 輸出行數: $(printf '%s\n' "$CMP_OUT" | wc -l)" >&2
      echo "    CMP_IMG=$CMP_IMG CMP_ID 存在=$( [ -f "$CMP_ID" ] && echo yes || echo NO )" >&2
    fi
  done

  # ⛔ 使用者⛔ 不得自己傳 `--run-identity`（它只能由官方腳本注入）。
  set +e
  env -u PY_IMAGE REPLAY_DRY_RUN=1 REPLAY_IMAGE_ID="$CMP_IMG" XDG_DATA_HOME="$CMP_TD/xdg" \
    "$REPO_ROOT/scripts/compare-replay-crossday.sh" \
    --d "$CMP_TD/d.json" --d1 "$CMP_TD/d1.json" --output-dir "$CMP_TD/out2" \
    --run-identity /evil >/dev/null 2>&1
  rc=$?
  set -e
  [ "$rc" -ne 0 ] && pass "使用者傳入 --run-identity 被拒絕" || fail "使用者傳入 --run-identity 竟然通過"

  # ⚠️ **路徑轉絕對值⛔ 不得靜默退化**：舊寫法在目錄不存在時會算出 `/d.json` 這種
  # 看似合法的路徑並照樣掛載。⚠️ 這裡**直接測 helper**——caller 端的 `-f` 會更早攔下，
  # 所以⛔ 不能透過 caller 驗這條。
  set +e
  ABS_OUT="$(bash -c '. "$1/scripts/lib/replay-args.sh"; replay_args_abs_path "$2" --demo' \
      _ "$REPO_ROOT" "$CMP_TD/no-such-dir/d.json" 2>"$CMP_TD/abs.err")"
  rc=$?
  set -e
  if [ "$rc" -ne 0 ] && grep -q "所在目錄不存在" "$CMP_TD/abs.err" && [ -z "$ABS_OUT" ]; then
    pass "abs_path：目錄不存在 → 非零且⛔ 不吐出 /d.json"
  else
    fail "abs_path 目錄不存在：rc=$rc stdout=[$ABS_OUT] 首行=$(head -1 "$CMP_TD/abs.err")"
  fi

  # ⚠️ **父目錄存在但檔案不存在**也要 fail-closed。⛔ 不擋的話 `docker -v` 會把來源
  # **建成 root 擁有的目錄**（實測）留在 host 上，錯誤延後到容器內才爆。
  set +e
  MISS_ERR="$(env -u PY_IMAGE REPLAY_DRY_RUN=1 REPLAY_IMAGE_ID="$CMP_IMG" XDG_DATA_HOME="$CMP_TD/xdg" \
      "$REPO_ROOT/scripts/compare-replay-crossday.sh" \
      --d "$CMP_TD/typo.json" --d1 "$CMP_TD/d1.json" --output-dir "$CMP_TD/out4" 2>&1)"
  rc=$?
  set -e
  if [ "$rc" -ne 0 ] && grep -q "必須是既有檔案" <<< "$MISS_ERR"; then
    pass "--d 檔名打錯（父目錄存在）→ 明確錯誤"
  else
    fail "--d 檔名打錯：rc=$rc 首行=$(head -1 <<< "$MISS_ERR")"
  fi
  [ ! -e "$CMP_TD/typo.json" ] && pass "被拒後 host 上⛔ 沒有被 docker 建出殘留" \
    || fail "host 上出現殘留：$CMP_TD/typo.json"
elif [ "${IMAGE_REQUIRED:-0}" = "1" ]; then
  # ⚠️ 由 `python/scripts/test.sh` 呼叫時 image **剛建好**——找不到代表環境有問題，
  # ⛔ 不得靜默 skip（那正是這幾段測試長期沒被執行到的原因）。
  fail "comparator argv 測試：找不到 image 或正式 bundle（IMAGE_REQUIRED=1）"
else
  echo "  skip 找不到 image 或正式 bundle，略過 comparator argv 測試" >&2
fi
rm -rf "$CMP_TD"

# ── Stage 1 的 argv fixture（含 --i074-preflight 的透傳）────────────────────
echo "==> i074：Stage 1 argv 與 fixture 相同"
S1_TD="$(mktemp -d)"
S1_FIXTURE="$REPO_ROOT/python/scripts/fixtures/stage1_argv.json"
S1_IMG="$(docker image inspect "${PY_IMAGE:-stock-trading-python-test:latest}" -f '{{.Id}}' 2>/dev/null || true)"
S1_BUNDLE="$REPO_ROOT/python/baselines/b1_20260901_1d_74350966_5d7ecb10"

if [ -n "$S1_IMG" ] && [ -d "$S1_BUNDLE" ]; then
  S1_ID="$S1_TD/xdg/stock_trading/i074_stage1/run_identity.json"
  ensure_identity "$S1_TD/xdg" "$S1_BUNDLE" "$S1_IMG" "stage 1" || true
  set +e
  S1_OUT="$(env -u PY_IMAGE REPLAY_DRY_RUN=1 REPLAY_IMAGE_ID="$S1_IMG" XDG_DATA_HOME="$S1_TD/xdg" \
      "$REPO_ROOT/scripts/run-replay-offline.sh" --bundle "$S1_BUNDLE" \
      --output-dir "$S1_TD/out" --before-ref HEAD --i074-preflight 2>/dev/null)"
  set -e

  S1_ACTUAL="$(printf '%s\n' "$S1_OUT" | sed -n '/^python$/,$p' \
    | sed -e "s|^$S1_BUNDLE\$|<BUNDLE>|" -e "s|^$S1_TD/out\$|<OUT>|" \
          -e "s|^sha256:[0-9a-f]\{64\}\$|<IMAGE_ID>|" \
          -e "s|^[0-9a-f]\{40\}\$|<BASE_COMMIT>|" \
    | awk '
        prev=="--tooling-patch-sha256"{print "<TOOLING_PATCH_SHA256>"; prev=$0; next}
        prev=="--runner-sha256"{print "<RUNNER_SHA256>"; prev=$0; next}
        {print; prev=$0}')"
  S1_EXPECTED="$(python3 -c '
import json,sys
print("\n".join(json.load(open(sys.argv[1],encoding="utf-8"))["stage1_argv"]))' "$S1_FIXTURE")"
  if [ "$S1_ACTUAL" = "$S1_EXPECTED" ]; then
    pass "Stage 1 argv 與 fixture 逐 token 相同"
  else
    fail "Stage 1 argv 與 fixture 不符"
    diff <(printf '%s\n' "$S1_EXPECTED") <(printf '%s\n' "$S1_ACTUAL") >&2 || true
  fi
  # ⚠️ flag ⛔ 不得被 shell 吃掉——它要原樣出現在容器內的 CLI。
  grep -qx -- "--i074-preflight" <<< "$S1_OUT" \
    && pass "--i074-preflight 透傳進容器內的 CLI" || fail "--i074-preflight 沒有透傳"

  # ⛔ I-074 正式流程缺 REPLAY_IMAGE_ID → **立即拒絕**，⛔ 不自動 pin。
  set +e
  env -u PY_IMAGE XDG_DATA_HOME="$S1_TD/xdg" "$REPO_ROOT/scripts/run-replay-offline.sh" --bundle "$S1_BUNDLE" \
      --output-dir "$S1_TD/out2" --before-ref HEAD --i074-preflight >/dev/null 2>&1
  rc=$?
  set -e
  [ "$rc" -ne 0 ] && pass "正式流程缺 REPLAY_IMAGE_ID 被拒絕" || fail "缺 REPLAY_IMAGE_ID 竟然通過"
elif [ "${IMAGE_REQUIRED:-0}" = "1" ]; then
  fail "Stage 1 argv 測試：找不到 image 或正式 bundle（IMAGE_REQUIRED=1）"
else
  echo "  skip 找不到 image 或正式 bundle，略過 Stage 1 argv 測試" >&2
fi
rm -rf "$S1_TD"

# ── pin-replay-image.sh 的三條 producer 分支與 stdout contract ─────────────
#
# ⚠️ 用 **fake docker**：⛔ 不能真的 build（那要好幾分鐘，也會動到本機 image）。
echo "==> i074：pin-replay-image.sh 的三條分支"
PIN_TD="$(mktemp -d)"
PIN_BUNDLE="$REPO_ROOT/python/baselines/b1_20260901_1d_74350966_5d7ecb10"
mkdir -p "$PIN_TD/bin" "$PIN_TD/xdg"
cat > "$PIN_TD/bin/docker" <<'FAKE'
#!/usr/bin/env bash
printf '%s\n' "$1" >> "$FAKE_DOCKER_LOG"
case "$1" in
  build) exit 0 ;;
  image)
    # $3 是 tag 或 image id
    if [ -n "${FAKE_MISSING:-}" ] && [ "$3" = "$FAKE_MISSING" ]; then exit 1; fi
    printf 'sha256:%064d\n' 1
    exit 0 ;;
esac
exit 0
FAKE
chmod +x "$PIN_TD/bin/docker"
PIN_ID="sha256:$(printf '%064d' 1)"

if [ -d "$PIN_BUNDLE" ]; then
  # ① identity 不存在 → 會 build
  : > "$PIN_TD/log1"
  set +e
  out="$(PATH="$PIN_TD/bin:$PATH" XDG_DATA_HOME="$PIN_TD/xdg" FAKE_DOCKER_LOG="$PIN_TD/log1" \
      "$REPO_ROOT/scripts/pin-replay-image.sh" "$PIN_BUNDLE" 2>/dev/null)"
  rc=$?
  set -e
  if [ "$rc" -eq 0 ] && [ "$out" = "$PIN_ID" ] && grep -qx build "$PIN_TD/log1"; then
    pass "identity 不存在 → build 並印出 image ID"
  else
    fail "identity 不存在的分支：rc=$rc out='$out'"
  fi

  # ② identity 已存在 → ⛔ 不 build
  : > "$PIN_TD/log2"
  set +e
  out2="$(PATH="$PIN_TD/bin:$PATH" XDG_DATA_HOME="$PIN_TD/xdg" FAKE_DOCKER_LOG="$PIN_TD/log2" \
      "$REPO_ROOT/scripts/pin-replay-image.sh" "$PIN_BUNDLE" 2>/dev/null)"
  rc2=$?
  set -e
  if [ "$rc2" -eq 0 ] && [ "$out2" = "$PIN_ID" ] && ! grep -qx build "$PIN_TD/log2"; then
    pass "identity 已存在 → ⛔ 不 build，直接沿用既有 ID"
  else
    fail "identity 已存在的分支：rc=$rc2 out='$out2' build=$(grep -cx build "$PIN_TD/log2")"
  fi

  # ③ image 已不在本機 → **fail-closed**，且 stdout ⛔ 無輸出
  : > "$PIN_TD/log3"
  set +e
  out3="$(PATH="$PIN_TD/bin:$PATH" XDG_DATA_HOME="$PIN_TD/xdg" FAKE_DOCKER_LOG="$PIN_TD/log3" \
      FAKE_MISSING="$PIN_ID" "$REPO_ROOT/scripts/pin-replay-image.sh" "$PIN_BUNDLE" 2>/dev/null)"
  rc3=$?
  set -e
  if [ "$rc3" -ne 0 ] && [ -z "$out3" ] && ! grep -qx build "$PIN_TD/log3"; then
    pass "image 已不在本機 → fail-closed、stdout 無輸出、⛔ 不重建"
  else
    fail "image 消失的分支：rc=$rc3 out='$out3'"
  fi

  # ④ --no-identity：只 build，⛔ 不碰 identity
  rm -rf "$PIN_TD/xdg2"; : > "$PIN_TD/log4"
  set +e
  out4="$(PATH="$PIN_TD/bin:$PATH" XDG_DATA_HOME="$PIN_TD/xdg2" FAKE_DOCKER_LOG="$PIN_TD/log4" \
      "$REPO_ROOT/scripts/pin-replay-image.sh" --no-identity 2>/dev/null)"
  rc4=$?
  set -e
  # ⚠️ 要**確認真的有 build**——⛔ 只看 rc／stdout 的話，日後誤刪 build 也會通過
  # （fake 的 `image inspect` 仍會回 ID）。
  if [ "$rc4" -eq 0 ] && [ "$out4" = "$PIN_ID" ] && [ ! -d "$PIN_TD/xdg2" ] \
     && grep -qx build "$PIN_TD/log4"; then
    pass "--no-identity → **確實 build**，⛔ 不建立 run identity"
  else
    fail "--no-identity 分支：rc=$rc4 out='$out4'"
  fi
elif [ "${IMAGE_REQUIRED:-0}" = "1" ]; then
  fail "pin-replay-image.sh 測試：找不到正式 bundle（IMAGE_REQUIRED=1）"
else
  echo "  skip 找不到正式 bundle，略過 pin 測試" >&2
fi
rm -rf "$PIN_TD"

# ── finalizer 的 argv（normal／recovery 兩組）與模式衝突 ───────────────────
#
# ⚠️ fixture 沒有任何測試讀取的話，它就只是一份沒人維護的檔案——⛔ 釘不住任何契約。
echo "==> i074：finalizer 的 argv 與模式衝突"
FIN_TD="$(mktemp -d)"
FIN_FIXTURE="$REPO_ROOT/python/scripts/fixtures/finalizer_argv.json"
FIN_IMG="$(docker image inspect "${PY_IMAGE:-stock-trading-python-test:latest}" -f '{{.Id}}' 2>/dev/null || true)"
FIN_BUNDLE="$REPO_ROOT/python/baselines/b1_20260901_1d_74350966_5d7ecb10"

if [ -n "$FIN_IMG" ] && [ -d "$FIN_BUNDLE" ]; then
  ensure_identity "$FIN_TD/xdg" "$FIN_BUNDLE" "$FIN_IMG" finalizer || true
  FIN_ID="$FIN_TD/xdg/stock_trading/i074_stage1/run_identity.json"
  mkdir -p "$FIN_TD/src"
  FIN_SRC_ARGS=()
  for rel in d/after_artifact.json.gz d/cohort_manifest.json.gz \
             d1/after_artifact.json.gz d1/cohort_manifest.json.gz \
             crossday/crossday_artifact.json.gz \
             probe/capacity_probe_computation.json.gz \
             probe/capacity_probe_measurement.json.gz probe/capacity_probe.json.gz \
             identity/run_identity.json.gz; do
    f="$FIN_TD/src/$(printf '%s' "$rel" | tr '/' '_')"
    : > "$f"
    FIN_SRC_ARGS+=(--source "$rel=$f")
  done

  set +e
  FIN_OUT="$(env -u PY_IMAGE REPLAY_DRY_RUN=1 REPLAY_IMAGE_ID="$FIN_IMG" XDG_DATA_HOME="$FIN_TD/xdg" \
      "$REPO_ROOT/scripts/finalize-evidence.sh" --evidence-root "$FIN_TD/ev" \
      "${FIN_SRC_ARGS[@]}" 2>/dev/null)"
  set -e
  FIN_ACTUAL="$(printf '%s\n' "$FIN_OUT" | sed -n '/^python$/,$p' \
    | sed -e "s|^$FIN_TD/ev\$|<ROOT>|" -e "s|^$FIN_ID\$|<IDENTITY>|" \
          -e "s|^sha256:[0-9a-f]\{64\}\$|<IMAGE_ID>|" -e "s|^[0-9a-f]\{40\}\$|<BASE_COMMIT>|" \
          -e "s|^\(.*\)=$FIN_TD/src/.*\$|\1=<SRC:\1>|" \
    | awk '
        prev=="--tooling-patch-sha256"{print "<TOOLING_PATCH_SHA256>"; prev=$0; next}
        prev=="--runner-sha256"{print "<RUNNER_SHA256>"; prev=$0; next}
        {print; prev=$0}')"
  FIN_EXPECTED="$(python3 -c '
import json,sys
print("\n".join(json.load(open(sys.argv[1],encoding="utf-8"))["normal_argv"]))' "$FIN_FIXTURE")"
  # ⚠️ **mount 也要驗**——只從 `python` token 開始比 argv 的話，前面的 Docker mounts
  # 全被丟掉，「identity 是否 :ro 掛載」這條計畫書明列的 contract 就沒人守。
  if grep -qx -- "$FIN_ID:$FIN_ID:ro" <<< "$FIN_OUT"; then
    pass "finalizer normal：identity 以 same-path :ro 掛載"
  else
    fail "finalizer normal 的 identity 不是 same-path :ro 掛載"
    # ⚠️ **失敗要印出可比對的證據**——⛔ 不能只靠重跑收斂。
    echo "    預期 mount : $FIN_ID:$FIN_ID:ro" >&2
    echo "    實際 -v 值 :" >&2
    grep -- ':ro$' <<< "$FIN_OUT" | sed 's/^/      /' >&2
    echo "    identity 檔存在=$( [ -f "$FIN_ID" ] && echo yes || echo NO )" >&2
    # ⚠️ **完整輸出落地**——這條斷言已經非決定性失敗兩次（2026-09-17 兩次都在完整
    # `python/scripts/test.sh` 流程中、單獨重跑都通過）。⛔ 現場只有一次，要留得下來。
    _dump="${TMPDIR:-/tmp}/i074-finalizer-fail.$$.txt"
    { echo "=== env ==="; echo "TMPDIR=${TMPDIR:-}"; echo "PWD=$PWD"; echo "FIN_TD=$FIN_TD";
       echo "readlink -f FIN_TD=$(readlink -f "$FIN_TD")"; echo "=== 完整 dry-run 輸出 ===";
       printf '%s\n' "$FIN_OUT"; } > "$_dump" 2>&1
    echo "    ⚠️ 完整輸出已存到：$_dump" >&2
    echo "    realpath(FIN_ID)=$(readlink -f "$FIN_ID" 2>/dev/null || echo n/a)" >&2
    echo "    argv 裡的 --run-identity：$(grep -A1 -x -- '--run-identity' <<< "$FIN_OUT" | tail -1)" >&2
  fi
  if [ "$FIN_ACTUAL" = "$FIN_EXPECTED" ]; then
    pass "finalizer normal argv 與 fixture 逐 token 相同"
  else
    fail "finalizer normal argv 與 fixture 不符"
    diff <(printf '%s\n' "$FIN_EXPECTED") <(printf '%s\n' "$FIN_ACTUAL") >&2 || true
  fi

  # ⚠️ 九份 `--source` 任一不存在就要 fail-closed（⛔ 同樣不能讓 docker 建出目錄）。
  FIN_MISS_ARGS=(); FIN_MISS_PATH=""
  for a in "${FIN_SRC_ARGS[@]}"; do
    case "$a" in
      d1/cohort_manifest.json.gz=*) FIN_MISS_PATH="$FIN_TD/src/typo_missing"
                                    FIN_MISS_ARGS+=("d1/cohort_manifest.json.gz=$FIN_MISS_PATH") ;;
      *) FIN_MISS_ARGS+=("$a") ;;
    esac
  done
  set +e
  FIN_MISS_ERR="$(env -u PY_IMAGE REPLAY_DRY_RUN=1 REPLAY_IMAGE_ID="$FIN_IMG" XDG_DATA_HOME="$FIN_TD/xdg" \
      "$REPO_ROOT/scripts/finalize-evidence.sh" --evidence-root "$FIN_TD/root2" \
      "${FIN_MISS_ARGS[@]}" 2>&1)"
  rc=$?
  set -e
  if [ "$rc" -ne 0 ] && grep -q "必須是既有檔案" <<< "$FIN_MISS_ERR"; then
    pass "finalizer：--source 指向不存在的檔案 → 明確錯誤"
  else
    fail "finalizer --source 不存在：rc=$rc 首行=$(head -1 <<< "$FIN_MISS_ERR")"
  fi
  [ ! -e "$FIN_MISS_PATH" ] && pass "finalizer 被拒後 host 上⛔ 沒有殘留" \
    || fail "host 上出現殘留：$FIN_MISS_PATH"

  # ⛔ **模式衝突**：recovery ⛔ 不接受 --source。
  set +e
  env -u PY_IMAGE REPLAY_IMAGE_ID="$FIN_IMG" XDG_DATA_HOME="$FIN_TD/xdg" \
    "$REPO_ROOT/scripts/finalize-evidence.sh" --evidence-root "$FIN_TD/ev" \
    --recover-durability "${FIN_SRC_ARGS[@]}" >/dev/null 2>&1
  rc=$?
  # ⛔ normal ⛔ 不接受空的 --source。
  env -u PY_IMAGE REPLAY_IMAGE_ID="$FIN_IMG" XDG_DATA_HOME="$FIN_TD/xdg" \
    "$REPO_ROOT/scripts/finalize-evidence.sh" --evidence-root "$FIN_TD/ev2" >/dev/null 2>&1
  rc2=$?
  set -e
  [ "$rc" -ne 0 ] && pass "recovery ⛔ 不接受 --source" || fail "recovery 竟接受 --source"

  # ⚠️ **recovery_argv 也要有人讀**——fixture 沒有測試使用等於沒釘住契約。
  mkdir -p "$FIN_TD/ev/identity"
  python3 - "$FIN_ID" "$FIN_TD/ev/identity/run_identity.json.gz" <<'EMBED'
import gzip, pathlib, sys
raw = pathlib.Path(sys.argv[1]).read_bytes()
import io
buf = io.BytesIO()
with gzip.GzipFile(filename="", mode="wb", fileobj=buf, compresslevel=9, mtime=0) as fh:
    fh.write(raw)
pathlib.Path(sys.argv[2]).write_bytes(buf.getvalue())
EMBED
  set +e
  FIN_REC="$(env -u PY_IMAGE REPLAY_DRY_RUN=1 REPLAY_IMAGE_ID="$FIN_IMG" XDG_DATA_HOME="$FIN_TD/xdg" \
      "$REPO_ROOT/scripts/finalize-evidence.sh" --evidence-root "$FIN_TD/ev" \
      --recover-durability 2>/dev/null)"
  set -e
  FIN_REC_ACTUAL="$(printf '%s\n' "$FIN_REC" | sed -n '/^python$/,$p' \
    | sed -e "s|^$FIN_TD/ev\$|<ROOT>|" -e "s|^sha256:[0-9a-f]\{64\}\$|<IMAGE_ID>|" \
          -e "s|^[0-9a-f]\{40\}\$|<BASE_COMMIT>|" \
    | awk '
        prev=="--tooling-patch-sha256"{print "<TOOLING_PATCH_SHA256>"; prev=$0; next}
        prev=="--runner-sha256"{print "<RUNNER_SHA256>"; prev=$0; next}
        {print; prev=$0}')"
  FIN_REC_EXPECTED="$(python3 -c '
import json,sys
print("\n".join(json.load(open(sys.argv[1],encoding="utf-8"))["recovery_argv"]))' "$FIN_FIXTURE")"
  # ⚠️ **recovery ⛔ 不得以任何形式碰外部 identity**——它只讀 evidence 內的 archived copy。
  # ⛔ 只比對 `$FIN_ID:$FIN_ID:ro` 這一個精確字串是不夠的：回歸成 **RW mount** 或掛到
  # **另一個 container path** 時仍會通過。斷言**整份輸出完全不含該路徑**。
  if grep -qF -- "$FIN_ID" <<< "$FIN_REC"; then
    fail "recovery 的指令出現了外部 identity 路徑"
  else
    pass "finalizer recovery：整份指令⛔ 完全不含外部 identity 路徑"
  fi
  if [ "$FIN_REC_ACTUAL" = "$FIN_REC_EXPECTED" ]; then
    pass "finalizer recovery argv 與 fixture 逐 token 相同"
  else
    fail "finalizer recovery argv 與 fixture 不符"
    diff <(printf '%s\n' "$FIN_REC_EXPECTED") <(printf '%s\n' "$FIN_REC_ACTUAL") >&2 || true
  fi
  [ "$rc2" -ne 0 ] && pass "normal 缺 --source 被拒絕" || fail "normal 缺 --source 竟通過"
elif [ "${IMAGE_REQUIRED:-0}" = "1" ]; then
  fail "finalizer argv 測試：找不到 image 或正式 bundle（IMAGE_REQUIRED=1）"
else
  echo "  skip 找不到 image 或正式 bundle，略過 finalizer argv 測試" >&2
fi
rm -rf "$FIN_TD"

# ── orchestrator 的結束碼仲裁與 source 綁定 ────────────────────────────────
#
# ⚠️ 用 **fake 的 compare／finalize** 測仲裁：把 orchestrator 複製到臨時目錄後，
# 它的 `REPO_ROOT` 會跟著變，於是 `$REPO_ROOT/scripts/*.sh` 指到同目錄的 fake。
echo "==> i074：orchestrator 的 0／5／3 仲裁"
ORC_TD="$(mktemp -d)"
mkdir -p "$ORC_TD/scripts"
cp "$REPO_ROOT/scripts/run-i074-stage1.sh" "$ORC_TD/scripts/"
cat > "$ORC_TD/scripts/compare-replay-crossday.sh" <<'FAKE'
#!/usr/bin/env bash
exit "${FAKE_CROSSDAY_RC:-0}"
FAKE
cat > "$ORC_TD/scripts/finalize-evidence.sh" <<'FAKE'
#!/usr/bin/env bash
printf '%s\n' "$@" > "$FAKE_FINAL_ARGS_OUT"
exit "${FAKE_FINAL_RC:-0}"
FAKE
chmod +x "$ORC_TD/scripts/"*.sh
: > "$ORC_TD/d.json"; : > "$ORC_TD/d1.json"

orc() {  # $1=crossday rc  $2=finalizer rc
  set +e
  FAKE_CROSSDAY_RC="$1" FAKE_FINAL_RC="$2" FAKE_FINAL_ARGS_OUT="$ORC_TD/final_args" \
    "$ORC_TD/scripts/run-i074-stage1.sh" --d "$ORC_TD/d.json" --d1 "$ORC_TD/d1.json" \
    --crossday-output-dir "$ORC_TD/cd" --evidence-root "$ORC_TD/ev" \
    --source "probe/capacity_probe.json.gz=$ORC_TD/d.json" >/dev/null 2>&1
  local rc=$?
  set -e
  printf '%s' "$rc"
}

[ "$(orc 0 0)" = "0" ] && pass "crossday 0 ＋ finalizer 0 → 0" || fail "crossday 0 ＋ finalizer 0"
[ "$(orc 5 0)" = "5" ] && pass "crossday 5 ＋ finalizer 0 → **5**（mismatch 也 finalize）" \
  || fail "crossday 5 的原始碼沒有保留"
[ "$(orc 5 3)" = "3" ] && pass "finalizer 3 **優先於**原始的 5（durability 未確認要被看見）" \
  || fail "durability 未確認沒有優先"
[ "$(orc 0 3)" = "3" ] && pass "finalizer 3 優先於原始的 0" || fail "durability 未確認沒有優先"
[ "$(orc 0 1)" = "1" ] && pass "finalizer 一般失敗 → 1" || fail "finalizer 一般失敗"
[ "$(orc 2 0)" = "2" ] && pass "crossday 非 0／5 → ⛔ 不跑 finalizer" || fail "crossday 非 0／5"

# ⚠️ **本次 comparator 的輸入與輸出由 orchestrator 綁定**——⛔ 否則可以
# 「比較本次 D／D+1、卻封存另一組舊產物」，最終 exit code 與 evidence 包不是同一次比較。
orc 0 0 >/dev/null
for rel in d/after_artifact.json.gz d1/after_artifact.json.gz crossday/crossday_artifact.json.gz; do
  if grep -q "^--source$" "$ORC_TD/final_args" && grep -q "^$rel=" "$ORC_TD/final_args"; then
    pass "orchestrator 綁定 $rel"
  else
    fail "orchestrator 沒有綁定 $rel"
  fi
done
set +e
FAKE_CROSSDAY_RC=0 FAKE_FINAL_RC=0 FAKE_FINAL_ARGS_OUT="$ORC_TD/x" \
  "$ORC_TD/scripts/run-i074-stage1.sh" --d "$ORC_TD/d.json" --d1 "$ORC_TD/d1.json" \
  --crossday-output-dir "$ORC_TD/cd" --evidence-root "$ORC_TD/ev" \
  --source "d/after_artifact.json.gz=/evil" >/dev/null 2>&1
rc=$?
set -e
[ "$rc" -ne 0 ] && pass "使用者⛔ 不得用 --source 覆寫被綁定的三份" || fail "被綁定的來源竟可覆寫"

# ⚠️ **而且要在 comparator 之前擋**：放在後面的話，錯誤呼叫會先產出 crossday artifact，
# comparator 回 5 之後流程又因覆寫回 1 → **跳過 finalizer**，⛔ 掩蓋掉 mismatch。
cat > "$ORC_TD/scripts/compare-replay-crossday.sh" <<'FAKE'
#!/usr/bin/env bash
: > "$FAKE_COMPARATOR_MARKER"
exit "${FAKE_CROSSDAY_RC:-0}"
FAKE
chmod +x "$ORC_TD/scripts/compare-replay-crossday.sh"
rm -f "$ORC_TD/comparator_ran"
set +e
FAKE_CROSSDAY_RC=5 FAKE_COMPARATOR_MARKER="$ORC_TD/comparator_ran" \
  "$ORC_TD/scripts/run-i074-stage1.sh" --d "$ORC_TD/d.json" --d1 "$ORC_TD/d1.json" \
  --crossday-output-dir "$ORC_TD/cd" --evidence-root "$ORC_TD/ev" \
  --source "crossday/crossday_artifact.json.gz=/evil" >/dev/null 2>&1
set -e
[ ! -f "$ORC_TD/comparator_ran" ] \
  && pass "覆寫在 **comparator 之前**就被擋（⛔ 不留下任何產物）" \
  || fail "comparator 竟已執行——mismatch 可能被掩蓋"
rm -rf "$ORC_TD"

# ── I-074 Stage 2（③b）：環境見證的 finalizer argv／mount／模式、pin --stage 2、restore、I074_STAGE ──
#
# 對應 issue.md I-074 ③ evidence contract「七之四」與測試 bg；Stage 2 計畫書「二、⑤」。
#
# ⚠️ **finalizer 的測試一律在隔離的最小 repo 裡跑**（2026-09-23 review）：它的路徑是寫死常數
# （`python/baselines/i074_stage2/envcheck/`），⛔ 沒有覆寫參數。先前在真正的 repo 裡暫放 fixture，
# 正式 `envcheck/` 一進版控，recover 的 argv 測試就會**永久 skip**——shell 的 mount、注入參數與
# fixture 就沒人守了。現在把**工作樹現行的**腳本與模組複製進一個 `git init` 的暫存 repo，
# 所有斷言都對它做，⛔ 完全不碰真正的 repo，也⛔ 不因正式證據存在而略過。
echo "==> i074 Stage 2（⑦c）：.gitignore 的 staging 規則（靜態；執行期的守門在晉升的第 5b 步與 6b）"
# ⚠️ 只排除晉升的 staging；正式目的地（evidence/、failed/）⛔ 不排除——發布之後要進版控。
gi_ignored() { git -C "$REPO_ROOT" check-ignore --no-index -q -- "$1"; }
gi_ignored python/baselines/i074_stage2/.promote-staging-0123456789abcdef \
  && gi_ignored python/baselines/i074_stage2/.promote-staging-0123456789abcdef/evidence_manifest.json \
  && pass "staging（.promote-staging-<16 hex> 與它的成員）被 .gitignore 排除" || fail ".gitignore 沒有排除晉升的 staging"
if gi_ignored python/baselines/i074_stage2/evidence/x || gi_ignored "python/baselines/i074_stage2/failed/x/failure_record.json"; then
  fail ".gitignore 竟然排除了正式目的地（evidence/ 或 failed/）"
else
  pass "正式目的地（evidence/、failed/）⛔ 不被排除"
fi

echo "==> i074 Stage 2：finalize-stage2-evidence.sh 的 argv、mount 與模式（隔離 repo）"
S2_TD="$(mktemp -d)"
S2_FIXTURE="$REPO_ROOT/python/scripts/fixtures/stage2_finalizer_argv.json"
S2_IMG="$(docker image inspect "${PY_IMAGE:-stock-trading-python-test:latest}" -f '{{.Id}}' 2>/dev/null || true)"
S2_BUNDLE="$REPO_ROOT/python/baselines/b1_20260901_1d_74350966_5d7ecb10"

S2_REPO="$S2_TD/repo"
mkdir -p "$S2_REPO/scripts/lib" "$S2_REPO/python/scripts" \
         "$S2_REPO/python/backtest/modular/sr_scoring" "$S2_REPO/python/baselines/i074_stage1"
cp "$REPO_ROOT/scripts/finalize-stage2-evidence.sh" "$REPO_ROOT/scripts/run-replay-offline.sh" "$S2_REPO/scripts/"
cp "$REPO_ROOT"/scripts/lib/*.sh "$S2_REPO/scripts/lib/"
cp "$REPO_ROOT/python/scripts/validate-i074-run-identity.py" "$REPO_ROOT/python/scripts/_i074_bootstrap.py" \
   "$REPO_ROOT/python/scripts/i074-stage2-patch-claims.py" "$S2_REPO/python/scripts/"
cp -r "$REPO_ROOT/python/backtest/modular/sr_scoring/replay_bundle" "$S2_REPO/python/backtest/modular/sr_scoring/"
find "$S2_REPO" -name __pycache__ -prune -exec rm -rf {} +
: > "$S2_REPO/python/baselines/i074_stage1/.keep"
# ③d 的合成守門要一個真的 base。⚠️ ⑦a 起反事實 patch 改動的檔案集合**恰好**是固定的四個檔
# （`I074_CF_FILES`，兩個產品檔 ＋ 兩個測試檔），所以 base 放這四個檔的佔位內容；tooling 另外新增檔案。
mkdir -p "$S2_REPO/python/backtest/modular/sr_scoring/tests" "$S2_REPO/docs"
for _f in "${I074_CF_FILES[@]}"; do printf 'base\n' > "$S2_REPO/$_f"; done
printf 'docs\n' > "$S2_REPO/docs/notes.md"
git -C "$S2_REPO" init -q
git -C "$S2_REPO" add -A
git -C "$S2_REPO" -c user.name=t -c user.email=t@t -c commit.gpgsign=false commit -qm fixture
S2_FIN="$S2_REPO/scripts/finalize-stage2-evidence.sh"
S2_PYROOT="$S2_REPO/python"
S2_ENVCHECK="$S2_PYROOT/baselines/i074_stage2/envcheck"

s2_normalize() {
  # $1＝dry-run 輸出。⚠️ 從 `python` token 開始比；mount 另外驗。
  sed -n '/^python$/,$p' <<< "$1" \
    | sed -e "s|^$S2_TD/[a-z0-9_]*run\$|<RUN_DIR>|" -e "s|^$S2_ID\$|<IDENTITY>|" \
          -e "s|^$S2_PYROOT/baselines/i074_stage2/failed/[^/]*\$|<RECORD_DIR>|" \
          -e "s|^${S2_REAL:-/nonexistent}/python/baselines/i074_stage2/.*\$|<PROMOTION_PATH>|" \
          -e "s|^failed/[^/]*-[0-9a-f]\{64\}\$|<FAILED_TARGET>|" \
          -e "s|^$S2_PYROOT\$|<PYTHON_ROOT>|" \
          -e "s|^sha256:[0-9a-f]\{64\}\$|<IMAGE_ID>|" -e "s|^[0-9a-f]\{40\}\$|<BASE_COMMIT>|" \
    | awk '
        prev=="--tooling-patch-sha256"{print "<TOOLING_PATCH_SHA256>"; prev=$0; next}
        prev=="--runner-sha256"{print "<RUNNER_SHA256>"; prev=$0; next}
        prev=="--verified-patch-base"{print "<VERIFIED_PATCH_BASE>"; prev=$0; next}
        prev=="--verified-counterfactual-sha256"{print "<VERIFIED_COUNTERFACTUAL_SHA256>"; prev=$0; next}
        prev=="--verified-tooling-sha256"{print "<VERIFIED_TOOLING_SHA256>"; prev=$0; next}
        prev=="--verified-composed-sha256"{print "<VERIFIED_COMPOSED_SHA256>"; prev=$0; next}
        prev=="--verified-counterfactual-semantic-sha256"{print "<VERIFIED_COUNTERFACTUAL_SEMANTIC_SHA256>"; prev=$0; next}
        {print; prev=$0}'
}
s2_expected() {
  python3 -c 'import json,sys; print("\n".join(json.load(open(sys.argv[1],encoding="utf-8"))[sys.argv[2]]))' \
    "$S2_FIXTURE" "$1"
}
s2_fin() {  # 在隔離 repo 裡跑 finalizer（dry-run）；stdout＝docker argv
  env -u PY_IMAGE REPLAY_DRY_RUN=1 REPLAY_IMAGE_ID="${S2_IMAGE_OVERRIDE:-$S2_IMG}" \
    XDG_DATA_HOME="$S2_TD/xdg" "$S2_FIN" "$@"
}

if [ -n "$S2_IMG" ] && [ -d "$S2_BUNDLE" ]; then
  set +e
  XDG_DATA_HOME="$S2_TD/xdg" python3 "$REPO_ROOT/python/scripts/ensure-i074-run-identity.py" \
      --stage 2 --bundle "$S2_BUNDLE" --image-id "$S2_IMG" >/dev/null 2>&1
  rc=$?
  set -e
  S2_ID="$S2_TD/xdg/stock_trading/i074_stage2/run_identity.json"
  if [ "$rc" -eq 0 ] && [ -f "$S2_ID" ] && [ ! -e "$S2_TD/xdg/stock_trading/i074_stage1" ]; then
    pass "ensure --stage 2 → 只建立 i074_stage2 的 identity（⛔ 不碰 Stage 1 那份）"
  else
    fail "ensure --stage 2：rc=$rc"
  fi
  mkdir -p "$S2_TD/run/witness"
  : > "$S2_TD/run/witness/after_artifact.json"
  : > "$S2_TD/run/witness/cohort_manifest.json"

  set +e
  S2_OUT="$(s2_fin --envcheck --run-dir "$S2_TD/run" 2>/dev/null)"
  set -e
  if [ "$(s2_normalize "$S2_OUT")" = "$(s2_expected envcheck_argv)" ]; then
    pass "envcheck argv 與 fixture 逐 token 相同"
  else
    fail "envcheck argv 與 fixture 不符"
    diff <(s2_expected envcheck_argv) <(s2_normalize "$S2_OUT") >&2 || true
  fi
  # ⚠️ mount 的四條契約（⛔ 只比 argv 會把 mount 全丟掉）。⚠️ 用 here-string，⛔ 不用 `| grep -q`。
  grep -qx -- "$S2_ID:$S2_ID:ro" <<< "$S2_OUT" \
    && pass "envcheck：Stage 2 identity 以 same-path :ro 掛載" || fail "envcheck：identity 不是 same-path :ro"
  grep -qx -- "$S2_TD/run/witness:$S2_TD/run/witness:ro" <<< "$S2_OUT" \
    && pass "envcheck：witness 目錄以 same-path :ro 掛載" || fail "envcheck：witness 不是 same-path :ro"
  grep -qx -- "$S2_PYROOT/baselines/i074_stage1:$S2_PYROOT/baselines/i074_stage1:ro" <<< "$S2_OUT" \
    && pass "envcheck：Stage 1 證據⛔ 只讀" || fail "envcheck：Stage 1 證據不是 :ro"
  grep -qx -- "$S2_PYROOT/baselines/i074_stage2:$S2_PYROOT/baselines/i074_stage2" <<< "$S2_OUT" \
    && pass "envcheck：只有 baselines/i074_stage2 可寫" || fail "envcheck：i074_stage2 的掛載不符"
  if grep -q -- '/stock_trading/i074_stage1/run_identity.json' <<< "$S2_OUT"; then
    fail "envcheck 的指令出現了 Stage 1 的 identity"
  else
    pass "envcheck：⛔ 完全不碰 Stage 1 的 identity"
  fi
  if grep -qF -- "$REPO_ROOT/python/baselines" <<< "$S2_OUT"; then
    fail "隔離失效：指令裡出現了真正 repo 的 baselines 路徑"
  else
    pass "隔離：指令⛔ 完全不含真正 repo 的 baselines 路徑"
  fi

  # 模式衝突與輸入檢查
  s2_rejects() {  # $1＝說明；其餘＝參數
    local label="$1"; shift
    set +e
    s2_fin "$@" >/dev/null 2>&1
    local rc=$?
    set -e
    [ "$rc" -ne 0 ] && pass "$label" || fail "$label（竟然 rc=0）"
  }
  s2_rejects "兩個模式旗標互斥" --envcheck --recover-envcheck --run-dir "$S2_TD/run"
  s2_rejects "沒有模式旗標 → 拒絕" --run-dir "$S2_TD/run"
  s2_rejects "--envcheck 缺 --run-dir → 拒絕" --envcheck
  s2_rejects "--recover-envcheck ⛔ 不接受 --run-dir" --recover-envcheck --run-dir "$S2_TD/run"
  s2_rejects "--run-dir 重複 → 拒絕" --envcheck --run-dir "$S2_TD/run" --run-dir "$S2_TD/run"
  s2_rejects "--source-ref 重複 → 拒絕（⛔ 不靜默採用最後一個）" --envcheck --run-dir "$S2_TD/run" \
    --source-ref HEAD --source-ref HEAD
  s2_rejects "使用者注入 --run-identity → 拒絕" --envcheck --run-dir "$S2_TD/run" --run-identity "$S2_ID"
  s2_rejects "使用者注入 --python-root → 拒絕" --envcheck --run-dir "$S2_TD/run" --python-root /x
  S2_IMAGE_OVERRIDE="sha256:$(printf '%064d' 7)" s2_rejects "REPLAY_IMAGE_ID 不在本機 → 拒絕" \
    --envcheck --run-dir "$S2_TD/run"
  mv "$S2_TD/run/witness/cohort_manifest.json" "$S2_TD/run/cohort.bak"
  s2_rejects "witness 輸入缺檔 → Docker 之前就拒絕" --envcheck --run-dir "$S2_TD/run"
  mv "$S2_TD/run/cohort.bak" "$S2_TD/run/witness/cohort_manifest.json"
  s2_rejects "--recover-envcheck 找不到封存的 identity → 拒絕" --recover-envcheck

  # recover-envcheck：在隔離 repo 裡放一份封存 identity（⛔ 不碰真正的證據，⛔ 也不因它存在而 skip）。
  mkdir -p "$S2_ENVCHECK/identity"
  python3 - "$S2_ID" "$S2_ENVCHECK/identity/run_identity.json.gz" <<'EMBED'
import gzip, io, pathlib, sys
buf = io.BytesIO()
with gzip.GzipFile(filename="", mode="wb", fileobj=buf, compresslevel=9, mtime=0) as fh:
    fh.write(pathlib.Path(sys.argv[1]).read_bytes())
pathlib.Path(sys.argv[2]).write_bytes(buf.getvalue())
EMBED
  set +e
  S2_REC="$(s2_fin --recover-envcheck 2>/dev/null)"
  set -e
  if [ "$(s2_normalize "$S2_REC")" = "$(s2_expected recover_envcheck_argv)" ]; then
    pass "recover-envcheck argv 與 fixture 逐 token 相同"
  else
    fail "recover-envcheck argv 與 fixture 不符"
    diff <(s2_expected recover_envcheck_argv) <(s2_normalize "$S2_REC") >&2 || true
  fi
  if grep -qF -- "$S2_ID" <<< "$S2_REC"; then
    fail "recover-envcheck 的指令出現了外部 identity 路徑"
  else
    pass "recover-envcheck：整份指令⛔ 完全不含外部 identity 路徑"
  fi
  grep -qx -- "$S2_PYROOT/baselines/i074_stage1:$S2_PYROOT/baselines/i074_stage1:ro" <<< "$S2_REC" \
    && pass "recover-envcheck：Stage 1 證據⛔ 只讀" || fail "recover-envcheck：Stage 1 證據不是 :ro"

  # ── ③d：其餘五種模式、合成守門與 check 的決策 ─────────────────────────────
  #
  # 對應 ③ evidence contract「四之二」「七之一」「七之四」與測試 y、bg、bh、bi、ah3、ak～av。
  # ⚠️ 合成守門跑的是**真的 git**：隔離 repo 裡的 base ＋ 用 tree diff 產生的真 canonical patch。
  # Python 段在 dry-run 下⛔ 不執行；check 的決策改用 fake docker 回傳固定的 Python 段輸出
  # （Python 段本身的 F1～F10 在 pytest 的 test_replay_stage2_archive.py）。
  echo "==> i074 Stage 2（③d）：其餘五種模式、合成守門與 check 的決策（隔離 repo）"
  S2_BASE_OID="$(git -C "$S2_REPO" rev-parse HEAD)"
  s2_sha() { sha256sum < "$1" | cut -d' ' -f1; }
  S2_BASE_TREE="$(git -C "$S2_REPO" rev-parse "$S2_BASE_OID^{tree}")"
  # $1＝輸出目錄；$2＝反事實附加到兩個**產品檔**的內容（空＝不動）；$3＝附加到兩個**測試檔**的內容（空＝不動）；
  # $4＝tooling 新增檔的內容（空＝0-byte）；其餘＝另外要改的檔（⚠️ 用來造「四檔集合不符」的反事實）。
  # ⚠️ ⑦a 起 stored patch 一律是**共用函式**產生的 canonical diff；另存一份預設 `git diff --binary`
  # （縮寫的 index 行）當「同一個 T1、不同文字表示」的非 canonical 對照。
  s2_make_patches() {
    local out="$1" prod="$2" tests="$3" tool="$4" wt="$S2_TD/gen.$RANDOM$RANDOM" t1 t2 f
    shift 4
    mkdir -p "$out"
    git -C "$S2_REPO" worktree add -q --detach "$wt" "$S2_BASE_OID"
    if [ -n "$prod" ]; then for f in "${I074_CF_PRODUCT_PATHS[@]}"; do printf '%s\n' "$prod" >> "$wt/$f"; done; fi
    if [ -n "$tests" ]; then
      for f in "${I074_CF_FILES[@]}"; do
        case " ${I074_CF_PRODUCT_PATHS[*]} " in *" $f "*) ;; *) printf '%s\n' "$tests" >> "$wt/$f" ;; esac
      done
    fi
    for f in "$@"; do mkdir -p "$(dirname "$wt/$f")"; printf 'extra\n' >> "$wt/$f"; done
    git -C "$wt" add -A
    t1="$(git -C "$wt" write-tree)"
    replay_args_canonical_diff "$S2_REPO" "$S2_BASE_TREE" "$t1" > "$out/counterfactual.patch"
    git -C "$wt" diff --binary "$S2_BASE_OID" "$t1" > "$out/counterfactual.noncanon.patch"
    replay_args_canonical_sha256 "$S2_REPO" "$S2_BASE_TREE" "$t1" "${I074_CF_PRODUCT_PATHS[@]}" > "$out/semantic.sha256"
    if [ -n "$tool" ]; then
      printf '%s\n' "$tool" > "$wt/tool_target.txt"
      git -C "$wt" add -A
    fi
    t2="$(git -C "$wt" write-tree)"
    replay_args_canonical_diff "$S2_REPO" "$t1" "$t2" > "$out/tooling.patch"
    git -C "$wt" diff --binary "$t1" "$t2" > "$out/tooling.noncanon.patch"
    replay_args_canonical_sha256 "$S2_REPO" "$S2_BASE_TREE" "$t2" > "$out/composed.sha256"
    git -C "$S2_REPO" worktree remove --force "$wt"
  }
  s2_json() {  # $1＝輸出（.json 或 .json.gz，canonical）；$2＝內容（⚠️ 只放 claims 小工具要讀的欄位）
    python3 - "$REPO_ROOT/python" "$1" "$2" <<'PY'
import json, sys
from pathlib import Path
sys.path.insert(0, sys.argv[1] + "/scripts")
from _i074_bootstrap import load_replay_bundle
c = load_replay_bundle(Path(sys.argv[1]), ("canonical",))["canonical"]
raw = c.canonical_json_bytes(json.loads(sys.argv[3]))
out = Path(sys.argv[2])
out.parent.mkdir(parents=True, exist_ok=True)
out.write_bytes(c.canonical_gzip_bytes(raw) if out.name.endswith(".gz") else raw)
PY
  }
  s2_patches_json() {  # $1＝cf SHA；$2＝tooling SHA；$3＝composed SHA
    printf '{"counterfactual_patch_sha256":"%s","tooling_patch_sha256":"%s","composed_sha256":"%s","ordered_components":["counterfactual","tooling"]}' "$1" "$2" "$3"
  }
  s2_prov_json() {  # $1＝composed SHA
    printf '{"base_commit":"%s","tooling_patch_sha256":"%s"}' "$S2_BASE_OID" "$1"
  }
  S2_P="$S2_TD/p_main"; s2_make_patches "$S2_P" b b ""           # 反事實 ＋ 空 tooling
  S2_PT="$S2_TD/p_tool"; s2_make_patches "$S2_PT" b b x          # 同一份反事實 ＋ 非空 tooling
  S2_P2="$S2_TD/p_other"; s2_make_patches "$S2_P2" c c ""        # 產品檔也不同的另一份反事實（語意 SHA 不同）
  S2_PX="$S2_TD/p_swap"; s2_make_patches "$S2_PX" b b y          # tooling 換成另一份**合法**的 patch
  S2_PTEST="$S2_TD/p_tests"; s2_make_patches "$S2_PTEST" b z ""  # ⚠️ 只有測試檔不同：完整 SHA 不同、語意 SHA 相同
  S2_PEXTRA="$S2_TD/p_extra"; s2_make_patches "$S2_PEXTRA" b b "" docs/notes.md   # 多一個非白名單檔
  S2_PMISS="$S2_TD/p_miss"; s2_make_patches "$S2_PMISS" "" b ""                    # 只改測試檔（少兩個產品檔）
  S2_CF="$(s2_sha "$S2_P/counterfactual.patch")"
  S2_SEM="$(cat "$S2_P/semantic.sha256")"
  [ "$(cat "$S2_PTEST/semantic.sha256")" = "$S2_SEM" ] && [ "$(s2_sha "$S2_PTEST/counterfactual.patch")" != "$S2_CF" ] \
    && [ "$(cat "$S2_P2/semantic.sha256")" != "$S2_SEM" ] \
    && pass "語意鍵：只改測試檔 → 完整 SHA 不同、語意 SHA 相同；產品檔不同 → 語意 SHA 不同" \
    || fail "語意鍵的 fixture 不符預期"
  S2_COMP_T="$(cat "$S2_PT/composed.sha256")"
  S2_TOOL_T="$(s2_sha "$S2_PT/tooling.patch")"
  [ "$(cat "$S2_P/composed.sha256")" = "$S2_CF" ] && [ "$(s2_sha "$S2_P/tooling.patch")" = "$(sha256sum < /dev/null | cut -d' ' -f1)" ] \
    && pass "fixture：空 tooling 的 SHA 是空字串的 SHA、composed ＝ counterfactual" || fail "fixture 的 patch 不符預期"

  s2_dry() {  # $1＝輸出檔前綴；其餘＝參數。回傳 rc，stdout／stderr 分別存檔
    local tag="$1"; shift
    set +e
    s2_fin "$@" > "$S2_TD/$tag.out" 2> "$S2_TD/$tag.err"
    local rc=$?
    set -e
    return "$rc"
  }
  s2_argv_ok() {  # $1＝說明；$2＝fixture key；$3＝輸出檔前綴
    if [ "$(s2_normalize "$(cat "$S2_TD/$3.out")")" = "$(s2_expected "$2")" ]; then
      pass "$1 argv 與 fixture 逐 token 相同"
    else
      fail "$1 argv 與 fixture 不符"
      diff <(s2_expected "$2") <(s2_normalize "$(cat "$S2_TD/$3.out")") >&2 || true
      cat "$S2_TD/$3.err" >&2
    fi
  }
  s2_mount_ok() {  # $1＝說明；$2＝輸出檔前綴；$3＝完整的 mount 字串
    grep -qx -- "$3" "$S2_TD/$2.out" && pass "$1" || fail "$1（找不到 $3）"
  }
  s2_blocked() {  # $1＝說明；$2＝輸出檔前綴；$3＝stderr 必須含的字串。⚠️ 合成守門擋下時⛔ 不得印出 docker 指令
    if [ ! -s "$S2_TD/$2.out" ] && grep -q -- "$3" "$S2_TD/$2.err"; then
      pass "$1"
    else
      fail "$1"; cat "$S2_TD/$2.err" >&2
    fi
  }

  # ── normal（--finalize） ──
  S2_FRUN="$S2_TD/s2run"
  mkdir -p "$S2_FRUN/stage2" "$S2_FRUN/patches"
  : > "$S2_FRUN/stage2/before_source_artifact.json"
  : > "$S2_FRUN/stage2/report.json"
  s2_json "$S2_FRUN/stage2/comparison_artifact.json" \
    "{\"schema_version\":1,\"kind\":\"sr_zone_replay_comparison\",\"provenance\":$(s2_prov_json "$S2_COMP_T")}"
  cp "$S2_PT/counterfactual.patch" "$S2_PT/tooling.patch" "$S2_FRUN/patches/"
  s2_dry fin --finalize --run-dir "$S2_FRUN" || true
  s2_argv_ok "finalize" finalize_argv fin
  s2_mount_ok "finalize：run 目錄的 stage2/ 以 same-path :ro 掛載" fin "$S2_FRUN/stage2:$S2_FRUN/stage2:ro"
  s2_mount_ok "finalize：凍結 patch 以 same-path :ro 掛載" fin "$S2_FRUN/patches:$S2_FRUN/patches:ro"
  s2_mount_ok "finalize：Stage 2 identity 以 same-path :ro 掛載" fin "$S2_ID:$S2_ID:ro"
  s2_mount_ok "finalize：Stage 1 證據⛔ 只讀" fin "$S2_PYROOT/baselines/i074_stage1:$S2_PYROOT/baselines/i074_stage1:ro"
  # y（成功 archive）：tooling 換成另一份**合法** patch → 各自的 SHA 都對得上檔案，⛔ 但合成不出宣告的 composed。
  cp "$S2_PX/tooling.patch" "$S2_FRUN/patches/tooling.patch"
  s2_dry fin_y --finalize --run-dir "$S2_FRUN" && fail "y：換掉 tooling patch 竟通過" || \
    s2_blocked "y：finalize 換掉 tooling patch → 合成守門中止、⛔ 不呼叫 Python" fin_y "合成 SHA"
  cp "$S2_PT/tooling.patch" "$S2_FRUN/patches/tooling.patch"
  # 非 canonical 的反事實 patch（同一個 T1、不同的文字表示）→ 正式 archive 無條件拒絕
  cp "$S2_PT/counterfactual.noncanon.patch" "$S2_FRUN/patches/counterfactual.patch"
  s2_dry fin_nc --finalize --run-dir "$S2_FRUN" && fail "非 canonical 的反事實 patch 竟通過" || \
    s2_blocked "finalize：反事實 patch ⛔ 不是 canonical diff（預設縮寫的 index 行）→ 中止" fin_nc "canonical"
  # ⑦a：四檔不變條件在共用合成函式裡，所以成功 archive 也拒絕多出非白名單檔的反事實。
  cp "$S2_PEXTRA/counterfactual.patch" "$S2_FRUN/patches/counterfactual.patch"
  s2_dry fin_extra --finalize --run-dir "$S2_FRUN" && fail "多出非白名單檔的反事實竟通過 finalize" || \
    s2_blocked "finalize：反事實改到固定四檔以外的檔案 → 合成守門中止" fin_extra "四個"
  cp "$S2_PT/counterfactual.patch" "$S2_FRUN/patches/counterfactual.patch"
  mv "$S2_FRUN/stage2/report.json" "$S2_TD/report.bak"
  s2_dry fin_miss --finalize --run-dir "$S2_FRUN" && fail "缺 report 竟通過" || \
    s2_blocked "finalize：run 目錄缺輸入 → Docker 之前就拒絕" fin_miss "report.json"
  mv "$S2_TD/report.bak" "$S2_FRUN/stage2/report.json"
  s2_json "$S2_TD/badkind.json" "{\"schema_version\":1,\"kind\":\"sr_zone_replay_report\",\"provenance\":$(s2_prov_json "$S2_COMP_T")}"
  cp "$S2_FRUN/stage2/comparison_artifact.json" "$S2_TD/comparison.bak"
  cp "$S2_TD/badkind.json" "$S2_FRUN/stage2/comparison_artifact.json"
  s2_dry fin_claims --finalize --run-dir "$S2_FRUN" && fail "取不到宣告值竟通過" || \
    s2_blocked "finalize：取不到合成守門的宣告值 → 中止、⛔ 不呼叫 Python" fin_claims "宣告值"
  cp "$S2_TD/comparison.bak" "$S2_FRUN/stage2/comparison_artifact.json"

  # ── 高 1（2026-09-24 review）：合成守門與封存之間的交接 ──
  # ⚠️ shell 驗過的四個值以 `--verified-*` 交給 Python；Python 拿實際要封存的內容比對（pytest 的
  # test_toctou_*）。這裡守 shell 那一半：交出去的就是**合成守門驗過的**值，⛔ 不是事後重讀的。
  s2_after_flag() {  # $1＝檔案；$2＝旗標 → 印出它後面那個 token
    awk -v f="$2" 'prev==f{print; exit} {prev=$0}' "$1"
  }
  if [ "$(s2_after_flag "$S2_TD/fin.out" --verified-patch-base)" = "$S2_BASE_OID" ] \
     && [ "$(s2_after_flag "$S2_TD/fin.out" --verified-counterfactual-sha256)" = "$S2_CF" ] \
     && [ "$(s2_after_flag "$S2_TD/fin.out" --verified-tooling-sha256)" = "$S2_TOOL_T" ] \
     && [ "$(s2_after_flag "$S2_TD/fin.out" --verified-composed-sha256)" = "$S2_COMP_T" ]; then
    pass "高 1：finalize 把合成守門驗過的 base 與三個 SHA 交給 Python"
  else
    fail "高 1：交接值與合成守門驗過的值不符"
  fi
  s2_rejects "高 1：使用者自帶 --verified-composed-sha256 → 拒絕（只能由腳本注入）" \
    --finalize --run-dir "$S2_FRUN" --verified-composed-sha256 "$S2_COMP_T"
  # 合成守門之後、Docker 之前把 tooling patch 換成另一份合法 patch（fake docker 在 `run` 的第一步換檔）：
  # ⚠️ 交出去的必須仍是 A 的值——Python 讀到的是 B，比對不符即中止（見 pytest）。
  mkdir -p "$S2_TD/swapbin"
  cat > "$S2_TD/swapbin/docker" <<'FAKE'
#!/usr/bin/env bash
case "$1" in
  image) printf '%s\n' "$3"; exit 0 ;;
  run) cp "$SWAP_FROM" "$SWAP_TO"; shift; printf '%s\n' "$@" > "$FAKE_ARGV"; exit 0 ;;
esac
exit 0
FAKE
  chmod +x "$S2_TD/swapbin/docker"
  set +e
  env -u PY_IMAGE PATH="$S2_TD/swapbin:$PATH" SWAP_FROM="$S2_PX/tooling.patch" SWAP_TO="$S2_FRUN/patches/tooling.patch" \
    FAKE_ARGV="$S2_TD/swap.argv" REPLAY_IMAGE_ID="$S2_IMG" XDG_DATA_HOME="$S2_TD/xdg" \
    "$S2_FIN" --finalize --run-dir "$S2_FRUN" >/dev/null 2>"$S2_TD/swap.err"
  rc=$?
  set -e
  if [ "$rc" -eq 0 ] && [ "$(s2_sha "$S2_FRUN/patches/tooling.patch")" != "$S2_TOOL_T" ] \
     && [ "$(s2_after_flag "$S2_TD/swap.argv" --verified-tooling-sha256)" = "$S2_TOOL_T" ] \
     && [ "$(s2_after_flag "$S2_TD/swap.argv" --verified-composed-sha256)" = "$S2_COMP_T" ]; then
    pass "高 1：合成守門之後換掉輸入 → 交給 Python 的仍是驗過的 A（Python 讀到 B 即中止）"
  else
    fail "高 1：換檔回歸測試不符（rc=$rc）"; cat "$S2_TD/swap.err" >&2
  fi
  cp "$S2_PT/tooling.patch" "$S2_FRUN/patches/tooling.patch"

  # ── recover-durability ──
  S2_EVID="$S2_PYROOT/baselines/i074_stage2/evidence"
  mkdir -p "$S2_EVID/patch" "$S2_EVID/identity"
  cp "$S2_PT/counterfactual.patch" "$S2_PT/tooling.patch" "$S2_EVID/patch/"
  cp "$S2_ENVCHECK/identity/run_identity.json.gz" "$S2_EVID/identity/"
  s2_json "$S2_EVID/evidence_manifest.json" \
    "{\"schema_version\":1,\"kind\":\"sr_zone_stage2_evidence_manifest\",\"patches\":$(s2_patches_json "$S2_CF" "$S2_TOOL_T" "$S2_COMP_T")}"
  s2_json "$S2_EVID/comparison/comparison_artifact.json.gz" \
    "{\"schema_version\":1,\"kind\":\"sr_zone_replay_comparison\",\"provenance\":$(s2_prov_json "$S2_COMP_T")}"
  s2_dry rd --recover-durability || true
  s2_argv_ok "recover-durability" recover_durability_argv rd
  if grep -qF -- "$S2_ID" "$S2_TD/rd.out"; then
    fail "recover-durability 的指令出現了外部 identity 路徑"
  else
    pass "recover-durability：⛔ 不掛載也⛔ 不注入外部 identity"
  fi
  # bi：換掉 archive 內的 tooling patch 並**同步改 manifest**——metadata 全對，⛔ 只有合成守門擋得下。
  cp "$S2_PX/tooling.patch" "$S2_EVID/patch/tooling.patch"
  s2_json "$S2_EVID/evidence_manifest.json" \
    "{\"schema_version\":1,\"kind\":\"sr_zone_stage2_evidence_manifest\",\"patches\":$(s2_patches_json "$S2_CF" "$(s2_sha "$S2_PX/tooling.patch")" "$S2_COMP_T")}"
  s2_dry rd_bi --recover-durability && fail "bi：recovery 竟放行換過的 patch" || \
    s2_blocked "bi：recover-durability 重做合成守門 → 中止、⛔ 不呼叫 Python" rd_bi "合成 SHA"
  cp "$S2_PT/tooling.patch" "$S2_EVID/patch/tooling.patch"

  # ── publish-failed-record ──
  S2_XRUN="$S2_TD/failrun"
  mkdir -p "$S2_XRUN/stage2" "$S2_XRUN/patches"
  s2_json "$S2_XRUN/stage2/bounded_diagnostics.json" \
    "{\"schema_version\":1,\"kind\":\"sr_zone_stage2_counterfactual_failure\",\"provenance\":$(s2_prov_json "$S2_CF")}"
  cp "$S2_P/counterfactual.patch" "$S2_P/tooling.patch" "$S2_XRUN/patches/"
  s2_dry pf --publish-failed-record --run-dir "$S2_XRUN" || true
  s2_argv_ok "publish-failed-record" publish_failed_record_argv pf
  [ "$(s2_after_flag "$S2_TD/pf.out" --verified-counterfactual-semantic-sha256)" = "$S2_SEM" ] \
    && pass "publish-failed-record：交給 Python 的語意 SHA ＝ 由凍結 patch 重算的值（failed record 的目錄鍵）" \
    || fail "publish-failed-record：語意 SHA 的交接值不符"
  grep -qx -- "--verified-counterfactual-semantic-sha256" "$S2_TD/fin.out" \
    && fail "finalize 竟帶了語意 SHA（成功 archive ⛔ 不存語意 SHA）" \
    || pass "finalize：⛔ 不帶語意 SHA 的交接"
  s2_rejects "使用者自帶 --verified-counterfactual-semantic-sha256 → 拒絕（只能由腳本注入）" \
    --publish-failed-record --run-dir "$S2_XRUN" --verified-counterfactual-semantic-sha256 "$S2_SEM"
  # TOCTOU（⑦a）：合成守門之後、Docker 之前把反事實換成「只改測試檔」的那一份（語意 SHA 相同、完整 SHA 不同）。
  # ⚠️ 交出去的必須仍是 A 的兩個值——Python 讀到 B 的完整 SHA 不符即中止（pytest 的 test_toctou_*）。
  set +e
  env -u PY_IMAGE PATH="$S2_TD/swapbin:$PATH" SWAP_FROM="$S2_PTEST/counterfactual.patch" \
    SWAP_TO="$S2_XRUN/patches/counterfactual.patch" FAKE_ARGV="$S2_TD/swap_pf.argv" REPLAY_IMAGE_ID="$S2_IMG" \
    XDG_DATA_HOME="$S2_TD/xdg" "$S2_FIN" --publish-failed-record --run-dir "$S2_XRUN" >/dev/null 2>"$S2_TD/swap_pf.err"
  rc=$?
  set -e
  if [ "$rc" -eq 0 ] && [ "$(s2_sha "$S2_XRUN/patches/counterfactual.patch")" != "$S2_CF" ] \
     && [ "$(s2_after_flag "$S2_TD/swap_pf.argv" --verified-counterfactual-sha256)" = "$S2_CF" ] \
     && [ "$(s2_after_flag "$S2_TD/swap_pf.argv" --verified-counterfactual-semantic-sha256)" = "$S2_SEM" ]; then
    pass "TOCTOU：驗完之後換掉反事實 → 交給 Python 的仍是驗過的 A（完整 SHA 與語意 SHA）"
  else
    fail "TOCTOU（publish）回歸測試不符（rc=$rc）"; cat "$S2_TD/swap_pf.err" >&2
  fi
  cp "$S2_P/counterfactual.patch" "$S2_XRUN/patches/counterfactual.patch"
  s2_mount_ok "publish-failed-record：中繼檔目錄以 same-path :ro 掛載" pf "$S2_XRUN/stage2:$S2_XRUN/stage2:ro"
  # y／bh（failed record）：反事實換成另一份合法 patch → 合成不出中繼檔記的 composed → ⛔ 不發布。
  cp "$S2_P2/counterfactual.patch" "$S2_XRUN/patches/counterfactual.patch"
  s2_dry pf_y --publish-failed-record --run-dir "$S2_XRUN" && fail "bh：換掉 patch 竟發布" || \
    s2_blocked "y／bh：publish-failed-record 的 F8-a 不過 → ⛔ 不發布、⛔ 不呼叫 Python" pf_y "合成 SHA"
  cp "$S2_P/counterfactual.patch" "$S2_XRUN/patches/counterfactual.patch"

  # ── recover-failed-record ──
  S2_FAILED="$S2_PYROOT/baselines/i074_stage2/failed"
  s2_record() {  # $1＝record 目錄；$2＝patch 目錄；$3＝宣告的 composed（空＝用 $2 的）；$4＝宣告的語意 SHA（空＝用 $2 的）
    mkdir -p "$1/patch"
    cp "$2/counterfactual.patch" "$2/tooling.patch" "$1/patch/"
    local comp="${3:-$(cat "$2/composed.sha256")}" sem="${4:-$(cat "$2/semantic.sha256")}"
    s2_json "$1/failure_record.json" \
      "{\"schema_version\":1,\"kind\":\"sr_zone_stage2_failed_attempt\",\"patches\":$(s2_patches_json "$(s2_sha "$2/counterfactual.patch")" "$(s2_sha "$2/tooling.patch")" "$comp"),\"provenance\":$(s2_prov_json "$comp"),\"counterfactual_semantic_sha256\":\"$sem\"}"
  }
  S2_REC1="$S2_FAILED/b1_fixture-$S2_SEM"
  s2_record "$S2_REC1" "$S2_P"
  s2_dry rf --recover-failed-record "$S2_REC1" || true
  s2_argv_ok "recover-failed-record" recover_failed_record_argv rf
  [ "$(s2_after_flag "$S2_TD/rf.out" --verified-counterfactual-semantic-sha256)" = "$S2_SEM" ] \
    && pass "recover-failed-record：宣告的語意 SHA ＝ 由 record 內實際 patch 重算的值 → 才注入" \
    || fail "recover-failed-record：語意 SHA 的交接值不符"
  # F4 的 shell 層：record 宣告的語意 SHA ≠ 由封存 patch 重算的值 → ⛔ 不呼叫 Python、⛔ 不 fsync。
  S2_REC_SEMBAD="$S2_FAILED/b1_fixture-$(printf 'd%.0s' {1..64})"
  s2_record "$S2_REC_SEMBAD" "$S2_P2" "" "$(printf 'd%.0s' {1..64})"
  s2_dry rf_sem --recover-failed-record "$S2_REC_SEMBAD" && fail "宣告的語意 SHA 與重算值不符竟被接受" || \
    s2_blocked "recover-failed-record：宣告的語意 SHA ≠ 重算值 → 中止、⛔ 不呼叫 Python" rf_sem "counterfactual_semantic_sha256"
  mkdir -p "$S2_TD/elsewhere"
  cp -r "$S2_REC1" "$S2_TD/elsewhere/"
  s2_dry rf_out --recover-failed-record "$S2_TD/elsewhere/$(basename "$S2_REC1")" && fail "failed root 外的目錄竟被接受" || \
    s2_blocked "recover-failed-record：只接受 failed/ 的直接子目錄" rf_out "只接受"
  ln -s "$S2_REC1" "$S2_TD/reclink"
  s2_dry rf_ln --recover-failed-record "$S2_TD/reclink" && fail "symlink 竟被接受" || \
    s2_blocked "recover-failed-record：⛔ 不接受 symlink" rf_ln "symlink"
  S2_REC_BAD="$S2_FAILED/b1_fixture-$(printf '%064d' 1)"
  s2_record "$S2_REC_BAD" "$S2_PX" "$S2_CF"         # tooling 非空、卻宣告「composed ＝ 反事實」
  s2_dry rf_y --recover-failed-record "$S2_REC_BAD" && fail "F8-a：recovery 竟放行合成不出的 record" || \
    s2_blocked "F8-a：recover-failed-record 重做合成守門 → 中止" rf_y "合成 SHA"

  # ── check-failed-record：argv 與「只吃路徑」 ──
  s2_dry chk --check-failed-record --counterfactual-patch "$S2_P/counterfactual.patch" || true
  s2_argv_ok "check-failed-record" check_failed_record_argv chk
  s2_mount_ok "check-failed-record：lookup 唯讀，連 i074_stage2 也 :ro" chk \
    "$S2_PYROOT/baselines/i074_stage2:$S2_PYROOT/baselines/i074_stage2:ro"
  if grep -qF -- "$S2_P/counterfactual.patch" "$S2_TD/chk.out"; then
    fail "check-failed-record：使用者的 patch 路徑進了容器"
  else
    pass "check-failed-record：使用者的 patch ⛔ 不進容器（比對鍵只在 host 推導）"
  fi
  s2_rejects "ar：--counterfactual-patch 給 SHA（⛔ 不是檔案）→ 拒絕" --check-failed-record --counterfactual-patch "$S2_CF"
  s2_rejects "ar：⛔ 沒有吃 SHA 的參數" --check-failed-record --counterfactual-patch-sha256 "$S2_CF"

  # ── bg：模式互斥、重複、⛔ 逐檔覆寫 ──
  s2_rejects "bg：--finalize 與 --recover-durability 互斥" --finalize --recover-durability --run-dir "$S2_FRUN"
  s2_rejects "bg：--check-failed-record 缺 --counterfactual-patch → 拒絕" --check-failed-record
  s2_rejects "bg：--counterfactual-patch 只屬於 check" --finalize --run-dir "$S2_FRUN" --counterfactual-patch "$S2_P/counterfactual.patch"
  s2_rejects "bg：--counterfactual-patch 重複 → 拒絕" --check-failed-record \
    --counterfactual-patch "$S2_P/counterfactual.patch" --counterfactual-patch "$S2_P/counterfactual.patch"
  s2_rejects "bg：--recover-failed-record 重複 → 拒絕" --recover-failed-record "$S2_REC1" --recover-failed-record "$S2_REC1"
  s2_rejects "bg：--recover-durability ⛔ 不接受 --run-dir" --recover-durability --run-dir "$S2_FRUN"
  s2_rejects "bg：--publish-failed-record 缺 --run-dir → 拒絕" --publish-failed-record
  s2_rejects "bg：⛔ 不接受 --source 逐檔覆寫" --finalize --run-dir "$S2_FRUN" \
    --source "stage2/report.json=$S2_TD/other.json"

  # ── check-failed-record 的決策（fake docker 回傳 Python 段的輸出） ──
  mkdir -p "$S2_TD/chkbin"
  cat > "$S2_TD/chkbin/docker" <<'FAKE'
#!/usr/bin/env bash
case "$1" in
  image) printf '%s\n' "$3"; exit 0 ;;
  run) printf 'run\n' >> "$FAKE_DOCKER_LOG"
       [ "${FAKE_CHECK_RC:-0}" = "0" ] || exit "$FAKE_CHECK_RC"
       cat "$FAKE_CHECK_JSON"; exit 0 ;;
esac
exit 0
FAKE
  chmod +x "$S2_TD/chkbin/docker"
  s2_check_json() {  # $1＝輸出；其餘＝record 目錄（宣告值讀自各自的 failure_record.json）
    python3 - "$S2_BASE_OID" "$@" <<'PY'
import json, sys
base, out, dirs = sys.argv[1], sys.argv[2], sys.argv[3:]
records = []
for d in dirs:
    r = json.load(open(d + "/failure_record.json"))
    records.append({"dir": d, "base_commit": r["provenance"]["base_commit"],
                    **{k: r["patches"][k] for k in ("counterfactual_patch_sha256", "tooling_patch_sha256", "composed_sha256")},
                    "counterfactual_semantic_sha256": r["counterfactual_semantic_sha256"]})
json.dump({"mode": "check_failed_record", "base_commit": base, "records": records}, open(out, "w"))
PY
  }
  s2_check() {  # $1＝輸出檔前綴；$2＝canned JSON；$3＝輸入 patch；回傳 finalizer 的 rc
    local tag="$1"
    : > "$S2_TD/$tag.log"
    set +e
    env -u PY_IMAGE PATH="$S2_TD/chkbin:$PATH" FAKE_DOCKER_LOG="$S2_TD/$tag.log" FAKE_CHECK_JSON="$2" \
      FAKE_CHECK_RC="${S2_FAKE_RC:-0}" REPLAY_IMAGE_ID="$S2_IMG" XDG_DATA_HOME="$S2_TD/xdg" \
      "$S2_FIN" --check-failed-record --counterfactual-patch "$3" > "$S2_TD/$tag.out" 2> "$S2_TD/$tag.err"
    local rc=$?
    set -e
    return "$rc"
  }
  s2_check_rc() {  # $1＝說明；$2＝預期 rc；$3＝輸出檔前綴；其餘＝s2_check 的參數
    local label="$1" want="$2" tag="$3" rc=0
    shift 3
    s2_check "$tag" "$@" || rc=$?
    if [ "$rc" = "$want" ]; then pass "$label"; else fail "$label（rc=$rc，預期 $want）"; cat "$S2_TD/$tag.err" >&2; fi
  }
  s2_check_json "$S2_TD/none.json"
  s2_check_json "$S2_TD/hit.json" "$S2_REC1"
  S2_REC2="$S2_FAILED/b1_fixture-$(cat "$S2_P2/semantic.sha256")"
  s2_record "$S2_REC2" "$S2_P2"
  s2_check_json "$S2_TD/other.json" "$S2_REC2"
  s2_check_json "$S2_TD/bad.json" "$S2_REC_BAD"
  s2_check_json "$S2_TD/sembad.json" "$S2_REC_SEMBAD"
  _cf0="${I074_CF_PRODUCT_PATHS[0]}"
  printf '%s\n' "diff --git a/$_cf0 b/$_cf0" "--- a/$_cf0" "+++ b/$_cf0" \
    '@@ -1 +1 @@' '-zzz' '+b' > "$S2_TD/conflict.patch"
  s2_check_json "$S2_TD/outside.json" "$S2_TD/elsewhere/$(basename "$S2_REC1")"

  s2_check_rc "ak：沒有任何 record → rc=0 放行" 0 ck_none "$S2_TD/none.json" "$S2_P/counterfactual.patch"
  s2_check_rc "al：records 都有效、語意 SHA 都不同 → rc=0" 0 ck_other "$S2_TD/other.json" "$S2_P/counterfactual.patch"
  s2_check_rc "am／n：語意 SHA 命中（完整 SHA 也相同）→ rc=2" 2 ck_hit "$S2_TD/hit.json" "$S2_P/counterfactual.patch"
  s2_check_rc "o2／am：只改測試檔（完整 SHA 不同、語意 SHA 相同）→ 仍命中 rc=2" 2 ck_o2 "$S2_TD/hit.json" \
    "$S2_PTEST/counterfactual.patch"
  s2_check_rc "o：白名單產品檔的 diff 改變（語意 SHA 不同）→ rc=0 放行" 0 ck_o "$S2_TD/hit.json" \
    "$S2_P2/counterfactual.patch"
  s2_check_rc "ab：recover 之後同語意 SHA 仍⛔ 不得重跑 → rc=2" 2 ck_ab "$S2_TD/hit.json" "$S2_P/counterfactual.patch"
  s2_check_rc "F4（shell 層）：record 宣告的語意 SHA ≠ 由封存 patch 重算的值 → rc=1" 1 ck_sem "$S2_TD/sembad.json" \
    "$S2_P/counterfactual.patch"
  grep -q "重算" "$S2_TD/ck_sem.err" && pass "F4：錯誤訊息指出宣告值 ≠ 重算值" || fail "F4 的錯誤訊息不符"
  s2_check_rc "語意鍵：反事實多出非白名單檔 → rc=1（replay 之前拒絕）" 1 ck_extra "$S2_TD/none.json" \
    "$S2_PEXTRA/counterfactual.patch"
  s2_check_rc "語意鍵：只改測試檔、沒改產品檔的反事實 → rc=1" 1 ck_miss "$S2_TD/none.json" \
    "$S2_PMISS/counterfactual.patch"
  s2_check_rc "as／av：bytes 不同但 T1 相同（非 canonical）→ 仍命中 rc=2（命中優先）" 2 ck_as "$S2_TD/hit.json" \
    "$S2_P/counterfactual.noncanon.patch"
  s2_check_rc "au：非 canonical 且未命中 → rc=1（replay 之前拒絕）" 1 ck_au "$S2_TD/none.json" \
    "$S2_P/counterfactual.noncanon.patch"
  grep -q "canonical" "$S2_TD/ck_au.err" && pass "au：錯誤訊息指出不是 canonical" || fail "au 的錯誤訊息不符"
  s2_check_rc "at：patch 套用失敗 → rc=1" 1 ck_at "$S2_TD/none.json" "$S2_TD/conflict.patch"
  s2_check_rc "ap／ah3：Python 段過、F8-a 失敗 → rc=1" 1 ck_ap "$S2_TD/bad.json" "$S2_P/counterfactual.patch"
  grep -q "合成守門" "$S2_TD/ck_ap.err" && pass "ap：失敗點是合成守門" || fail "ap 的失敗點不符"
  S2_FAKE_RC=1 s2_check_rc "an／aq／ah3：Python 段失敗（record 損壞、信任錨失敗）→ rc=1" 1 ck_ao "$S2_TD/bad.json" \
    "$S2_P/counterfactual.patch"
  if grep -q "Python 段失敗" "$S2_TD/ck_ao.err" && ! grep -q "合成守門" "$S2_TD/ck_ao.err"; then
    pass "ao：Python 段失敗時 shell 合成段⛔ 不執行（失敗點唯一）"
  else
    fail "ao：Python 段失敗後 shell 段仍被執行"; cat "$S2_TD/ck_ao.err" >&2
  fi
  s2_check_rc "Python 段回報 failed root 外的 record → rc=1" 1 ck_out "$S2_TD/outside.json" "$S2_P/counterfactual.patch"
  grep -qx run "$S2_TD/ck_hit.log" && pass "check：Python 段在容器內執行（fake docker 收到 run）" || fail "check：沒有執行 Python 段"


  # ── ⑦c：唯讀的驗證入口 --verify-promotion-staging（「Stage 2 步驟 ⑦c 細部計畫 v1」「二之二」） ──────────
  #
  # 真正 repo 由**本 repo 的 origin** 推導：隔離 repo 加一個指向合成「真正 repo」的 origin。⚠️ 全部 dry-run（⛔ 不真的
  # 跑 docker）；Python 段（完整驗證、信任根、--judge 的同程序判讀）在 pytest 的 test_replay_stage2_archive.py。
  echo "==> i074 Stage 2（⑦c）：--verify-promotion-staging 的路徑規則、掛載與合成守門（隔離 repo）"
  mkdir -p "$S2_TD/realrepo/python/baselines/i074_stage2"
  S2_REAL="$(cd "$S2_TD/realrepo" && pwd -P)"
  S2_RI="$S2_REAL/python/baselines/i074_stage2"
  git -C "$S2_REPO" remote add origin "$S2_REAL"
  s2_evidence_at() {  # $1＝目錄；$2＝patch 目錄（manifest／comparison 只放 claims 要讀的欄位）
    local comp; comp="$(cat "$2/composed.sha256")"
    mkdir -p "$1/patch" "$1/identity"
    cp "$2/counterfactual.patch" "$2/tooling.patch" "$1/patch/"
    cp "$S2_ENVCHECK/identity/run_identity.json.gz" "$1/identity/"
    s2_json "$1/evidence_manifest.json" \
      "{\"schema_version\":1,\"kind\":\"sr_zone_stage2_evidence_manifest\",\"patches\":$(s2_patches_json "$(s2_sha "$2/counterfactual.patch")" "$(s2_sha "$2/tooling.patch")" "$comp")}"
    s2_json "$1/comparison/comparison_artifact.json.gz" \
      "{\"schema_version\":1,\"kind\":\"sr_zone_replay_comparison\",\"provenance\":$(s2_prov_json "$comp")}"
  }
  s2_evidence_at "$S2_RI/evidence" "$S2_PT"
  S2_VSTG="$S2_RI/.promote-staging-0123456789abcdef"
  cp -r "$S2_RI/evidence" "$S2_VSTG"
  S2_VFAIL="failed/b1_fixture-$S2_SEM"
  s2_record "$S2_RI/$S2_VFAIL" "$S2_P"
  s2_vp() {  # $1＝輸出檔前綴；其餘＝參數（⚠️ 一律加 --verify-promotion-staging 由呼叫端給）
    local tag="$1"; shift
    s2_dry "$tag" "$@"
  }
  s2_vp_rejects() {  # $1＝說明；$2＝輸出檔前綴；其餘＝參數 → 結束碼必須是 1、⛔ 不印出 docker 指令
    local label="$1" tag="$2" rc=0
    shift 2
    s2_dry "$tag" "$@" || rc=$?
    if [ "$rc" = 1 ] && [ ! -s "$S2_TD/$tag.out" ]; then pass "$label"; else fail "$label（rc=$rc）"; cat "$S2_TD/$tag.err" >&2; fi
  }
  s2_vp_ro() {  # $1＝輸出檔前綴 → 每一個 -v 都是 :ro，而且⛔ 不掛整個 i074_stage2（⛔ 沒有任何可寫的證據目錄）
    awk 'prev=="-v"{print} {prev=$0}' "$S2_TD/$1.out" > "$S2_TD/$1.mounts"
    [ -s "$S2_TD/$1.mounts" ] && ! grep -qv ':ro$' "$S2_TD/$1.mounts" \
      && ! grep -q -- "^$S2_PYROOT/baselines/i074_stage2:" "$S2_TD/$1.mounts"
  }

  s2_real_snapshot() { find "$S2_REAL" -printf '%p %y %s %T@\n' | sort; }
  S2_REAL_BEFORE="$(s2_real_snapshot)"
  s2_vp vp_ev --verify-promotion-staging "$S2_RI/evidence" --target evidence || true
  s2_argv_ok "verify-promotion-staging（evidence 目的地）" verify_promotion_staging_evidence_argv vp_ev
  s2_mount_ok "verify：Stage 1 錨點 same-path :ro" vp_ev "$S2_PYROOT/baselines/i074_stage1:$S2_PYROOT/baselines/i074_stage1:ro"
  s2_mount_ok "verify：envcheck 錨點 same-path :ro" vp_ev "$S2_ENVCHECK:$S2_ENVCHECK:ro"
  s2_mount_ok "verify：<path> same-path :ro" vp_ev "$S2_RI/evidence:$S2_RI/evidence:ro"
  s2_vp_ro vp_ev && pass "verify：全部掛載唯讀、⛔ 不掛整個 i074_stage2" || fail "verify：出現可寫的掛載"
  [ "$(s2_after_flag "$S2_TD/vp_ev.out" --verified-composed-sha256)" = "$S2_COMP_T" ] \
    && ! grep -qx -- --verified-counterfactual-semantic-sha256 "$S2_TD/vp_ev.out" \
    && pass "verify（evidence）：交接合成守門驗過的值、⛔ 不帶語意 SHA" || fail "verify（evidence）：交接值不符"
  s2_vp vp_judge --verify-promotion-staging "$S2_RI/evidence" --target evidence --judge || true
  s2_argv_ok "verify-promotion-staging（evidence ＋ --judge）" verify_promotion_staging_evidence_judge_argv vp_judge
  s2_vp vp_stg --verify-promotion-staging "$S2_VSTG" --target evidence || true
  s2_argv_ok "verify-promotion-staging（staging，直屬 i074_stage2）" verify_promotion_staging_evidence_argv vp_stg
  s2_vp vp_fail --verify-promotion-staging "$S2_RI/$S2_VFAIL" --target "$S2_VFAIL" || true
  s2_argv_ok "verify-promotion-staging（failed record 目的地）" verify_promotion_staging_failed_argv vp_fail
  [ "$(s2_after_flag "$S2_TD/vp_fail.out" --verified-counterfactual-semantic-sha256)" = "$S2_SEM" ] \
    && pass "verify（failed）：重算的語意 SHA ＝ 宣告值 ＝ target 名稱 → 才注入" || fail "verify（failed）：語意 SHA 的交接值不符"
  s2_mount_ok "verify（failed）：<path> same-path :ro" vp_fail "$S2_RI/$S2_VFAIL:$S2_RI/$S2_VFAIL:ro"
  s2_vp_ro vp_fail && pass "verify（failed）：全部掛載唯讀" || fail "verify（failed）：出現可寫的掛載"
  [ "$(s2_real_snapshot)" = "$S2_REAL_BEFORE" ] && pass "verify：四次執行之後真正 repo ⛔ 不被寫入（檔案集合、大小、mtime 都不變）" \
    || fail "verify：真正 repo 被寫入了"

  # 路徑規則：只接受真正 repo（本 repo 的 origin）的 staging（直屬）或目的地本身。
  cp -r "$S2_RI/evidence" "$S2_TD/elsewhere-evidence"
  s2_vp_rejects "verify：真正 repo 外的路徑 → 1" vp_out --verify-promotion-staging "$S2_TD/elsewhere-evidence" --target evidence
  # ⚠️ 本 repo 內放一份**合法**、名稱也對的 staging（⛔ 不借用前面測試改過的 $S2_EVID：它的合成守門本來就過不了，
  #    擋下它的會是合成守門而⛔ 不是路徑規則——反向驗證抓到的）。
  S2_VCLONE="$S2_PYROOT/baselines/i074_stage2/.promote-staging-0123456789abcdef"
  s2_evidence_at "$S2_VCLONE" "$S2_PT"
  s2_vp_rejects "verify：本 repo（複本）內的路徑（合法的內容、名稱也對）→ 1" vp_clone --verify-promotion-staging "$S2_VCLONE" --target evidence
  ln -s "$S2_RI/evidence" "$S2_TD/evlink"
  s2_vp_rejects "verify：路徑本身是 symlink → 1" vp_ln --verify-promotion-staging "$S2_TD/evlink" --target evidence
  ln -s "$S2_REAL" "$S2_TD/reallink"
  s2_vp_rejects "verify：路徑有 symlink 成分（⛔ 不是 canonical）→ 1" vp_lnc \
    --verify-promotion-staging "$S2_TD/reallink/python/baselines/i074_stage2/evidence" --target evidence
  mkdir -p "$S2_RI/sub"; cp -r "$S2_VSTG" "$S2_RI/sub/"
  s2_vp_rejects "verify：staging 不是 i074_stage2 的直屬 → 1" vp_deep \
    --verify-promotion-staging "$S2_RI/sub/.promote-staging-0123456789abcdef" --target evidence
  cp -r "$S2_RI/evidence" "$S2_RI/.promote-staging-xyz"; cp -r "$S2_RI/evidence" "$S2_RI/evidence2"
  s2_vp_rejects "verify：staging 名稱不符 → 1" vp_name --verify-promotion-staging "$S2_RI/.promote-staging-xyz" --target evidence
  s2_vp_rejects "verify：目的地名稱 ≠ target → 1" vp_name2 --verify-promotion-staging "$S2_RI/evidence2" --target evidence
  s2_vp_rejects "verify：target 與目的地不符（evidence 路徑 ＋ failed target）→ 1" vp_tgt \
    --verify-promotion-staging "$S2_RI/evidence" --target "$S2_VFAIL"
  s2_vp_rejects "verify：target 不在封閉集合 → 1" vp_tgt2 --verify-promotion-staging "$S2_RI/evidence" --target failed
  s2_vp_rejects "verify：缺 --target → 1" vp_not --verify-promotion-staging "$S2_RI/evidence"
  s2_vp_rejects "verify：--target 重複 → 1" vp_tt --verify-promotion-staging "$S2_RI/evidence" --target evidence --target evidence
  s2_vp_rejects "verify：--judge 搭 failed target → 1" vp_jf --verify-promotion-staging "$S2_RI/$S2_VFAIL" --target "$S2_VFAIL" --judge
  s2_vp_rejects "verify：--judge 重複 → 1" vp_jj --verify-promotion-staging "$S2_RI/evidence" --target evidence --judge --judge
  s2_vp_rejects "verify：--target 只屬於驗證模式 → 1" vp_tonly --recover-durability --target evidence
  s2_vp_rejects "verify：⛔ 不接受 --run-dir → 1" vp_rd --verify-promotion-staging "$S2_RI/evidence" --target evidence --run-dir "$S2_FRUN"
  s2_vp_rejects "verify：⛔ 不接受 --source-ref → 1" vp_sr --verify-promotion-staging "$S2_RI/evidence" --target evidence --source-ref HEAD
  set +e
  env -u PY_IMAGE REPLAY_DRY_RUN=1 REPLAY_IMAGE_ID="$S2_IMG" XDG_DATA_HOME="$S2_TD/xdg" TOOLING_PATCH="$S2_PT/tooling.patch" \
    "$S2_FIN" --verify-promotion-staging "$S2_RI/evidence" --target evidence > "$S2_TD/vp_tp.out" 2> "$S2_TD/vp_tp.err"
  rc=$?
  set -e
  [ "$rc" = 1 ] && [ ! -s "$S2_TD/vp_tp.out" ] && pass "verify：TOOLING_PATCH 有值 → 1（驗的必須是 HEAD 本身）" \
    || fail "verify：TOOLING_PATCH 竟被接受（rc=$rc）"
  s2_vp_rejects "verify：使用者自帶 --verified-composed-sha256 → 1" vp_inj \
    --verify-promotion-staging "$S2_RI/evidence" --target evidence --verified-composed-sha256 "$S2_COMP_T"
  # 合成守門：failed 的 target 名稱裡的 SHA ≠ 重算的語意 SHA（record 宣告值本身正確）→ ⛔ 不呼叫 Python。
  S2_VBADNAME="failed/b1_fixture-$(printf 'e%.0s' {1..64})"
  s2_record "$S2_RI/$S2_VBADNAME" "$S2_P"
  s2_vp vp_badname --verify-promotion-staging "$S2_RI/$S2_VBADNAME" --target "$S2_VBADNAME" && fail "target 名稱的 SHA 不符竟通過" || \
    s2_blocked "verify：failed 的 target 名稱裡的 SHA ≠ 重算的語意 SHA → 1、⛔ 不呼叫 Python" vp_badname "target 名稱"
  # 合成守門：換掉 tooling patch（manifest 不動）→ 合成 SHA 不符。
  cp "$S2_PX/tooling.patch" "$S2_VSTG/patch/tooling.patch"
  s2_vp vp_swap --verify-promotion-staging "$S2_VSTG" --target evidence && fail "換過 tooling 的 staging 竟通過" || \
    s2_blocked "verify：staging 的 tooling patch 被換 → 合成守門中止、⛔ 不呼叫 Python" vp_swap "SHA"
  cp "$S2_PT/tooling.patch" "$S2_VSTG/patch/tooling.patch"
  # 真正 repo 的推導：origin 不是本機絕對路徑、或指向本 repo 自己 → 1。
  git -C "$S2_REPO" remote set-url origin "relative/real"
  s2_vp_rejects "verify：origin 不是本機絕對路徑 → 1" vp_orig --verify-promotion-staging "$S2_RI/evidence" --target evidence
  git -C "$S2_REPO" remote set-url origin "$S2_REPO"
  s2_vp_rejects "verify：origin 是本 repo 自己 → 1" vp_self --verify-promotion-staging "$S2_VCLONE" --target evidence
  git -C "$S2_REPO" remote set-url origin "$S2_REAL"

  # ── ⑦a：runner 的反事實模式（Stage 2 計畫書「六、2」的 s～x、ba 的 runner 層；隔離 repo） ──────
  #
  # ⚠️ 全部用 dry-run（⛔ 不真的跑 docker）；runner 在印出 docker argv 之前就完成凍結、合成與前置守門。
  echo "==> i074 Stage 2（⑦a）：run-replay-offline.sh 的反事實模式（隔離 repo）"
  mkdir -p "$S2_TD/s7a"
  : > "$S2_TD/s7a/after_artifact.json"
  : > "$S2_TD/s7a/cohort_manifest.json"
  S7_ARGS=(--bundle "$S2_BUNDLE" --output-dir "$S2_TD/s7out" --before-ref "$S2_BASE_OID"
           --after-artifact "$S2_TD/s7a/after_artifact.json" --cohort-manifest "$S2_TD/s7a/cohort_manifest.json")
  s7_run() {  # $1＝輸出檔前綴；$2＝COUNTERFACTUAL_PATCH；$3＝TOOLING_PATCH；其餘＝runner 參數
    local tag="$1" cf="$2" tool="$3" rc
    shift 3
    set +e
    env -u PY_IMAGE REPLAY_DRY_RUN=1 REPLAY_IMAGE_ID="${S7_IMG-$S2_IMG}" XDG_DATA_HOME="$S2_TD/xdg" \
      I074_STAGE="${S7_STAGE:-2}" COUNTERFACTUAL_PATCH="$cf" TOOLING_PATCH="$tool" \
      "$S2_REPO/scripts/run-replay-offline.sh" "$@" > "$S2_TD/$tag.out" 2> "$S2_TD/$tag.err"
    rc=$?
    set -e
    return "$rc"
  }
  s7_blocked() {  # $1＝說明；$2＝輸出檔前綴；$3＝stderr 必須含的字串（空＝不看訊息）。⚠️ ⛔ 不得印出 docker 指令
    if ! grep -qx run "$S2_TD/$2.out" && { [ -z "$3" ] || grep -q -- "$3" "$S2_TD/$2.err"; }; then
      pass "$1"
    else
      fail "$1"; cat "$S2_TD/$2.err" >&2
    fi
  }
  S2_TOOL_EMPTY="$(sha256sum < /dev/null | cut -d' ' -f1)"
  # 正向：兩份 canonical patch → 注入反事實 SHA；三個增量 SHA 各自正確（t：⛔ 不靠「hash 必然不同」）。
  if s7_run s7_ok "$S2_PT/counterfactual.patch" "$S2_PT/tooling.patch" "${S7_ARGS[@]}" --i074-counterfactual; then
    if [ "$(s2_after_flag "$S2_TD/s7_ok.out" --counterfactual-patch-sha256)" = "$S2_CF" ] \
       && [ "$(s2_after_flag "$S2_TD/s7_ok.out" --tooling-patch-sha256)" = "$S2_COMP_T" ] \
       && grep -qx -- "==> patches: counterfactual=$S2_CF tooling=$S2_TOOL_T composed=$S2_COMP_T" "$S2_TD/s7_ok.out"; then
      pass "t：反事實 SHA、增量 tooling SHA 與合成 SHA 各自等於獨立推導的值（固定順序 counterfactual → tooling）"
    else
      fail "t：runner 的三個 SHA 不符"; cat "$S2_TD/s7_ok.out" >&2
    fi
    grep -qx -- "--i074-counterfactual" "$S2_TD/s7_ok.out" \
      && pass "--i074-counterfactual 透傳進容器內的 CLI" || fail "--i074-counterfactual 沒有透傳"
    [ "$(awk 'prev=="--runner-sha256"{getline; print; exit} {prev=$0}' "$S2_TD/s7_ok.out")" = "--counterfactual-patch-sha256" ] \
      && pass "--counterfactual-patch-sha256 注入在 --runner-sha256 之後" || fail "--counterfactual-patch-sha256 的位置不符"
  else
    fail "反事實模式的正向 dry-run 失敗"; cat "$S2_TD/s7_ok.err" >&2
  fi
  # u／ba（runner 層）：TOOLING_PATCH 空 → 0-byte 凍結副本，兩個 SHA 都有明確值、合成 ＝ 反事實。
  if s7_run s7_u "$S2_P/counterfactual.patch" "" "${S7_ARGS[@]}" --i074-counterfactual \
     && grep -q -- "==> patches: counterfactual=$S2_CF tooling=$S2_TOOL_EMPTY composed=$S2_CF" "$S2_TD/s7_u.out" \
     && [ "$(s2_after_flag "$S2_TD/s7_u.out" --tooling-patch-sha256)" = "$S2_CF" ]; then
    pass "u／ba：空的 TOOLING_PATCH → 0-byte 凍結副本（tooling ＝ 空字串的 SHA、composed ＝ 反事實）"
  else
    fail "u／ba：空 TOOLING_PATCH 的處理不符"; cat "$S2_TD/s7_u.err" >&2
  fi
  s7_run s7_v "" "" "${S7_ARGS[@]}" --i074-counterfactual && fail "v：flag 開啟但沒有 patch 竟通過" || \
    s7_blocked "v：flag 開啟但 COUNTERFACTUAL_PATCH 空 → 中止" s7_v "COUNTERFACTUAL_PATCH"
  S7_STAGE=1 s7_run s7_w "$S2_P/counterfactual.patch" "" "${S7_ARGS[@]}" && fail "w：沒帶 flag 卻帶語意 patch 竟通過" || \
    s7_blocked "w：flag 關閉但 COUNTERFACTUAL_PATCH 非空 → 中止" s7_w "一般路徑"
  s7_run s7_s1 "$S2_P/counterfactual.patch" "" --bundle "$S2_BUNDLE" --output-dir "$S2_TD/s7out" \
    --before-ref "$S2_BASE_OID" --i074-counterfactual && fail "flag 帶在 Stage 1 竟通過" || \
    s7_blocked "p（runner）：flag 帶在 Stage 1 → 中止" s7_s1 "只限 Stage 2"
  S7_STAGE=1 s7_run s7_id1 "$S2_P/counterfactual.patch" "" "${S7_ARGS[@]}" --i074-counterfactual \
    && fail "flag 搭 I074_STAGE=1 竟通過" || \
    s7_blocked "flag 必須搭 I074_STAGE=2（Stage 2 identity）→ 否則中止" s7_id1 "I074_STAGE=2"
  s7_run s7_eq "$S2_P/counterfactual.patch" "" "${S7_ARGS[@]}" --i074-counterfactual=1 \
    && fail "--i074-counterfactual=1 竟被接受" || s7_blocked "--i074-counterfactual=value → 中止" s7_eq "=value"
  S7_IMG="" s7_run s7_x "$S2_P/counterfactual.patch" "" "${S7_ARGS[@]}" --i074-counterfactual \
    && fail "x：缺 REPLAY_IMAGE_ID 竟通過（⛔ 不得自動 pin）" || \
    s7_blocked "x：flag 納入 I074_MODE → 缺 REPLAY_IMAGE_ID 即拒絕、⛔ 不自動 pin" s7_x ""
  s7_run s7_spoof "$S2_P/counterfactual.patch" "" "${S7_ARGS[@]}" --i074-counterfactual \
    --counterfactual-patch-sha256 "$S2_CF" && fail "s：使用者自帶反事實 SHA 竟通過" || \
    s7_blocked "s：使用者自帶 --counterfactual-patch-sha256 → 拒絕（只能由 runner 注入）" s7_spoof "只能由官方腳本注入"
  s7_run s7_nc "$S2_PT/counterfactual.noncanon.patch" "$S2_PT/tooling.patch" "${S7_ARGS[@]}" --i074-counterfactual \
    && fail "非 canonical 的反事實 patch 竟通過 runner" || \
    s7_blocked "runner 前置守門：反事實 patch 的 raw bytes ≠ canonical → docker 之前中止" s7_nc "不是 canonical"
  s7_run s7_nct "$S2_PT/counterfactual.patch" "$S2_PT/tooling.noncanon.patch" "${S7_ARGS[@]}" --i074-counterfactual \
    && fail "非 canonical 的 tooling patch 竟通過 runner" || \
    s7_blocked "runner 前置守門：tooling patch 的 raw bytes ≠ canonical → docker 之前中止" s7_nct "TOOLING_PATCH"
  s7_run s7_extra "$S2_PEXTRA/counterfactual.patch" "" "${S7_ARGS[@]}" --i074-counterfactual \
    && fail "多出非白名單檔的反事實竟通過 runner" || \
    s7_blocked "語意鍵（runner）：反事實改動⛔ 不是固定的四個檔 → 中止" s7_extra "四個"
  ln -s "$S2_P/counterfactual.patch" "$S2_TD/cf_link.patch"
  s7_run s7_link "$S2_TD/cf_link.patch" "" "${S7_ARGS[@]}" --i074-counterfactual \
    && fail "symlink 的反事實 patch 竟被凍結" || s7_blocked "凍結：COUNTERFACTUAL_PATCH ⛔ 不得是 symlink" s7_link "一般檔案"
  # 一般路徑（⛔ 沒有 flag）：只有 tooling，⛔ 不注入反事實 SHA。
  if S7_STAGE=1 s7_run s7_gen "" "$S2_PT/tooling.patch" "${S7_ARGS[@]}" \
     && ! grep -qx -- "--counterfactual-patch-sha256" "$S2_TD/s7_gen.out" \
     && grep -q -- "==> patches: counterfactual=- " "$S2_TD/s7_gen.out"; then
    pass "一般路徑：⛔ 不注入 --counterfactual-patch-sha256（逐項不變）"
  else
    fail "一般路徑的 runner 行為不符"; cat "$S2_TD/s7_gen.err" >&2
  fi

  # ⚠️ 合成守門與程式碼 worktree 都要被清掉（dry-run、中止、fake docker 三種路徑）。
  S2_WT_COUNT="$(git -C "$S2_REPO" worktree list | wc -l)"
  [ "$S2_WT_COUNT" = "1" ] && pass "③d 的每一條路徑都清掉了暫時 worktree" \
    || { fail "③d 留下了 worktree（$S2_WT_COUNT 條）"; git -C "$S2_REPO" worktree list >&2; }
elif [ "${IMAGE_REQUIRED:-0}" = "1" ]; then
  fail "Stage 2 finalizer 測試：找不到 image 或正式 bundle（IMAGE_REQUIRED=1）"
else
  echo "  skip 找不到 image 或正式 bundle，略過 Stage 2 finalizer 測試" >&2
fi

# ── aj：`.gitattributes` 對巢狀的 patch／log 生效（③ evidence contract「三之零」） ──
echo "==> i074 Stage 2：.gitattributes 的巢狀規則"
for s2_attr_path in python/baselines/i074_stage2/evidence/patch/counterfactual.patch \
                    python/baselines/i074_stage2/failed/b1_x-0/patch/tooling.patch \
                    python/baselines/i074_stage2/counterfactual_e1cbbbd.patch \
                    python/baselines/i074_stage2/failed/b1_x-0/run.log; do
  if [ "$(git -C "$REPO_ROOT" check-attr text -- "$s2_attr_path")" = "$s2_attr_path: text: unset" ]; then
    pass "aj：$s2_attr_path 的 text 是 unset（逐位元保存）"
  else
    fail "aj：$s2_attr_path 的 text ⛔ 不是 unset"
  fi
done
if [ "$(git -C "$REPO_ROOT" check-attr text -- python/baselines/i074_stage2/evidence/evidence_manifest.json)" \
     = "python/baselines/i074_stage2/evidence/evidence_manifest.json: text: unspecified" ]; then
  pass "aj：JSON artifact ⛔ 不跟著繞過 whitespace 檢查"
else
  fail "aj：JSON artifact 被 patch 的規則波及"
fi

# ── pin --stage 2：專用 tag、tarball、Stage 1 行為不變 ─────────────────────
echo "==> i074 Stage 2：pin-replay-image.sh --stage 2"
mkdir -p "$S2_TD/bin" "$S2_TD/state"
cat > "$S2_TD/bin/docker" <<'FAKE'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "$FAKE_DOCKER_LOG"
ID="sha256:$(printf '%064d' 2)"
case "$1" in
  build) exit 0 ;;
  tag) exit 0 ;;
  save)  # save -o <file> <id>
    printf 'fake-image-%s' "$4" > "$3"; exit 0 ;;
  load) : > "$FAKE_STATE/loaded"; exit 0 ;;
  image)  # image inspect <ref> [-f …]
    if [ -n "${FAKE_MISSING:-}" ] && [ "$3" = "$FAKE_MISSING" ] && [ ! -f "$FAKE_STATE/loaded" ]; then exit 1; fi
    printf '%s\n' "$ID"; exit 0 ;;
esac
exit 0
FAKE
chmod +x "$S2_TD/bin/docker"
S2_PIN_ID="sha256:$(printf '%064d' 2)"
S2_IMAGES="$S2_TD/pinxdg/stock_trading/i074_stage2/images"
s2_pin() {  # $1＝log 檔；其餘＝參數
  local log="$1"; shift
  : > "$log"
  env -u PY_IMAGE PATH="$S2_TD/bin:$PATH" XDG_DATA_HOME="$S2_TD/pinxdg" FAKE_DOCKER_LOG="$log" \
    FAKE_STATE="$S2_TD/state" "$REPO_ROOT/scripts/pin-replay-image.sh" "$@" 2>/dev/null
}

if [ -d "$S2_BUNDLE" ]; then
  set +e
  out="$(s2_pin "$S2_TD/p1" --stage 2 "$S2_BUNDLE")"; rc=$?
  set -e
  if [ "$rc" -eq 0 ] && [ "$out" = "$S2_PIN_ID" ] \
     && grep -qx "build -t stock-trading-python-replay:i074-stage2 $REPO_ROOT/python" "$S2_TD/p1" \
     && [ -f "$S2_TD/pinxdg/stock_trading/i074_stage2/run_identity.json" ] \
     && [ ! -e "$S2_TD/pinxdg/stock_trading/i074_stage1" ]; then
    pass "--stage 2 → 以**專用 tag** build、建立 Stage 2 identity（⛔ 不碰 Stage 1）"
  else
    fail "--stage 2 的建立分支：rc=$rc out='$out'"
    sed 's/^/    /' "$S2_TD/p1" >&2
  fi
  S2_HEX="$(printf '%064d' 2)"
  if [ -f "$S2_IMAGES/$S2_HEX.tar" ] && (cd "$S2_IMAGES" && sha256sum -c --status "$S2_HEX.tar.sha256"); then
    pass "--stage 2 → tarball 與 .sha256 已落地且可驗證"
  else
    fail "--stage 2 → 找不到可驗證的 tarball"
  fi

  set +e
  out="$(s2_pin "$S2_TD/p2" --stage 2 "$S2_BUNDLE")"; rc=$?
  set -e
  if [ "$rc" -eq 0 ] && [ "$out" = "$S2_PIN_ID" ] && ! grep -q '^build' "$S2_TD/p2" \
     && ! grep -q '^save' "$S2_TD/p2" && grep -qx "tag $S2_PIN_ID stock-trading-python-replay:i074-stage2" "$S2_TD/p2"; then
    pass "identity 已存在 → ⛔ 不 build、⛔ 不重存 tarball，只重新掛上專用 tag"
  else
    fail "--stage 2 no-op 分支：rc=$rc out='$out'"
  fi

  cp "$S2_IMAGES/$S2_HEX.tar.sha256" "$S2_TD/sidecar.orig"
  printf 'x' >> "$S2_IMAGES/$S2_HEX.tar"
  set +e
  out="$(s2_pin "$S2_TD/p3" --stage 2 "$S2_BUNDLE")"; rc=$?
  set -e
  [ "$rc" -ne 0 ] && [ -z "$out" ] && pass "既有 tarball 的 SHA 不符 → fail-closed、stdout 無輸出" \
    || fail "tarball 被改過竟通過：rc=$rc out='$out'"

  set +e
  out="$(FAKE_MISSING="$S2_PIN_ID" s2_pin "$S2_TD/p4" --stage 2 "$S2_BUNDLE")"; rc=$?
  set -e
  [ "$rc" -ne 0 ] && [ -z "$out" ] && ! grep -q '^build' "$S2_TD/p4" \
    && pass "image 已不在本機 → fail-closed、⛔ 不重建" || fail "image 消失分支：rc=$rc out='$out'"

  set +e
  PY_IMAGE=foo:bar PATH="$S2_TD/bin:$PATH" XDG_DATA_HOME="$S2_TD/pinxdg" FAKE_DOCKER_LOG="$S2_TD/p5" \
    FAKE_STATE="$S2_TD/state" "$REPO_ROOT/scripts/pin-replay-image.sh" --stage 2 "$S2_BUNDLE" >/dev/null 2>&1
  rc=$?
  out="$(s2_pin "$S2_TD/p6" --no-identity --stage 2)"; rc2=$?
  out3="$(s2_pin "$S2_TD/p7" --stage 3 "$S2_BUNDLE")"; rc3=$?
  set -e
  [ "$rc" -ne 0 ] && pass "--stage 2 ⛔ 不接受 PY_IMAGE（⛔ 不與 build 腳本共用 tag）" || fail "--stage 2 竟接受 PY_IMAGE"
  [ "$rc2" -ne 0 ] && pass "--no-identity ⛔ 不可與 --stage 2 並用" || fail "--no-identity --stage 2 竟通過"
  [ "$rc3" -ne 0 ] && pass "--stage 只接受 1／2" || fail "--stage 3 竟通過"

  # ⚠️ 重複／多餘參數一律中止（⛔ 不靜默採用最後一個、⛔ 不靜默忽略）。
  set +e
  out="$(s2_pin "$S2_TD/p8" --stage 2 --stage 2 "$S2_BUNDLE")"; rc8=$?
  out="$(s2_pin "$S2_TD/p9" --stage 2 "$S2_BUNDLE" extra)"; rc9=$?
  out="$(s2_pin "$S2_TD/p10" --no-identity extra)"; rc10=$?
  set -e
  [ "$rc8" -ne 0 ] && ! grep -q '^build' "$S2_TD/p8" \
    && pass "--stage 重複 → 拒絕（⛔ 不靜默採用最後一個）" || fail "--stage 重複竟通過"
  [ "$rc9" -ne 0 ] && ! grep -q '^build' "$S2_TD/p9" \
    && pass "bundle 之後的多餘參數 → 拒絕" || fail "多餘參數竟被忽略"
  [ "$rc10" -ne 0 ] && ! grep -q '^build' "$S2_TD/p10" \
    && pass "--no-identity 之後的多餘參數 → 拒絕" || fail "--no-identity 的多餘參數竟被忽略"

  # ── restore：先驗 SHA → load → 再驗 image ID ──
  echo "==> i074 Stage 2：restore-replay-image.sh"
  s2_restore() {
    local log="$1"; shift
    : > "$log"
    env -u PY_IMAGE PATH="$S2_TD/bin:$PATH" XDG_DATA_HOME="$S2_TD/pinxdg" FAKE_DOCKER_LOG="$log" \
      FAKE_STATE="$S2_TD/state" FAKE_MISSING="$S2_PIN_ID" \
      "$REPO_ROOT/scripts/restore-replay-image.sh" "$@" 2>/dev/null
  }
  set +e
  out="$(s2_restore "$S2_TD/r1" --stage 2 "$S2_BUNDLE")"; rc=$?   # tarball 已被改過（上面）
  set -e
  [ "$rc" -ne 0 ] && [ -z "$out" ] && ! grep -q '^load' "$S2_TD/r1" \
    && pass "tarball 的 SHA 不符 → ⛔ 不 load" || fail "SHA 不符竟 load：rc=$rc"
  # ⚠️ **decoy**（2026-09-23 review）：tar 仍是被改過的，但 sidecar 改指向同目錄一份**合法的**
  # decoy——`sha256sum -c` 會通過，⛔ 但要 load 的仍是那份被改過的 tar。必須被擋下。
  printf 'decoy' > "$S2_IMAGES/decoy.tar"
  printf '%s  decoy.tar\n' "$(sha256sum "$S2_IMAGES/decoy.tar" | cut -d' ' -f1)" > "$S2_IMAGES/$S2_HEX.tar.sha256"
  set +e
  out="$(s2_restore "$S2_TD/r_decoy" --stage 2 "$S2_BUNDLE")"; rc=$?
  out2="$(s2_pin "$S2_TD/p_decoy" --stage 2 "$S2_BUNDLE")"; rc2=$?
  set -e
  [ "$rc" -ne 0 ] && [ -z "$out" ] && ! grep -q '^load' "$S2_TD/r_decoy" \
    && pass "restore：sidecar 指向 decoy → ⛔ 不 load" || fail "restore 被 decoy 繞過：rc=$rc"
  [ "$rc2" -ne 0 ] && [ -z "$out2" ] \
    && pass "pin：sidecar 指向 decoy → fail-closed" || fail "pin 被 decoy 繞過：rc=$rc2"
  rm -f "$S2_IMAGES/decoy.tar"
  # 修回原本的 tarball（重算它原本記錄的內容）——⚠️ 下一支要**只剩**「sidecar 不是一行」這個缺陷
  printf 'fake-image-%s' "$S2_PIN_ID" > "$S2_IMAGES/$S2_HEX.tar"
  # 兩行的 sidecar（tar 本身合法、digest 也對）也⛔ 不接受——⛔ 不猜哪一行才算數
  { cat "$S2_TD/sidecar.orig"; cat "$S2_TD/sidecar.orig"; } > "$S2_IMAGES/$S2_HEX.tar.sha256"
  set +e
  out="$(s2_restore "$S2_TD/r_two" --stage 2 "$S2_BUNDLE")"; rc=$?
  set -e
  [ "$rc" -ne 0 ] && ! grep -q '^load' "$S2_TD/r_two" \
    && pass "restore：sidecar 不是恰好一行 → ⛔ 不 load" || fail "兩行 sidecar 竟通過"
  cp "$S2_TD/sidecar.orig" "$S2_IMAGES/$S2_HEX.tar.sha256"
  set +e
  out="$(s2_restore "$S2_TD/r2" --stage 2 "$S2_BUNDLE")"; rc=$?
  set -e
  if [ "$rc" -eq 0 ] && [ "$out" = "$S2_PIN_ID" ] && grep -q '^load' "$S2_TD/r2" \
     && grep -qx "tag $S2_PIN_ID stock-trading-python-replay:i074-stage2" "$S2_TD/r2"; then
    pass "SHA 相符 → load、再驗 image ID、重新掛上專用 tag"
  else
    fail "restore 正常分支：rc=$rc out='$out'"
  fi
  rm -f "$S2_TD/state/loaded"
  rm -f "$S2_IMAGES/$S2_HEX.tar"
  set +e
  out="$(s2_restore "$S2_TD/r3" --stage 2 "$S2_BUNDLE")"; rc=$?
  out4="$(s2_restore "$S2_TD/r4" --stage 1 "$S2_BUNDLE")"; rc4=$?
  set -e
  [ "$rc" -ne 0 ] && [ -z "$out" ] && pass "找不到 tarball → fail-closed（⛔ 不改為重建）" || fail "沒有 tarball 竟通過"
  [ "$rc4" -ne 0 ] && pass "restore 只支援 --stage 2" || fail "restore --stage 1 竟通過"

  # ── --adopt-image：採用既有 image、⛔ 不 build（2026-09-23 使用者裁決）────────
  echo "==> i074 Stage 2：pin-replay-image.sh --stage 2 --adopt-image"
  mkdir -p "$S2_TD/abin"
  cat > "$S2_TD/abin/docker" <<'FAKE'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "$FAKE_DOCKER_LOG"
case "$1" in
  image) [ "$3" = "$FAKE_KNOWN" ] && { printf '%s\n' "$3"; exit 0; }; exit 1 ;;
  run) printf '%s\n' "$FAKE_ENV_JSON"; exit 0 ;;          # 容器內算出的環境
  save) printf 'fake-image-%s' "$4" > "$3"; exit 0 ;;
esac
exit 0
FAKE
  chmod +x "$S2_TD/abin/docker"
  A_ID="sha256:$(printf '%064d' 3)"
  A_XDG="$S2_TD/adoptxdg"
  # ⚠️ 預期值取自**已驗證的** Stage 1 信任錨——與 pin 用的是同一支。
  A_ENV="$(python3 "$REPO_ROOT/python/scripts/print-i074-environment.py" --stage1 2>/dev/null || true)"
  s2_adopt() {  # $1＝log；$2＝容器內要回報的環境 JSON；其餘＝參數
    local log="$1" env_json="$2"; shift 2
    : > "$log"
    env -u PY_IMAGE PATH="$S2_TD/abin:$PATH" XDG_DATA_HOME="$A_XDG" FAKE_DOCKER_LOG="$log" \
      FAKE_KNOWN="$A_ID" FAKE_ENV_JSON="$env_json" \
      "$REPO_ROOT/scripts/pin-replay-image.sh" "$@" 2>/dev/null
  }
  A_IDENTITY="$A_XDG/stock_trading/i074_stage2/run_identity.json"
  set +e
  out="$(s2_adopt "$S2_TD/a1" "$A_ENV" --adopt-image "$A_ID" "$S2_BUNDLE")"; rc1=$?
  out2="$(s2_adopt "$S2_TD/a2" "$A_ENV" --stage 2 --adopt-image stock_trading-python-server:latest "$S2_BUNDLE")"; rc2=$?
  out3="$(s2_adopt "$S2_TD/a3" "$A_ENV" --stage 2 --adopt-image "$A_ID" --adopt-image "$A_ID" "$S2_BUNDLE")"; rc3=$?
  out4="$(s2_adopt "$S2_TD/a4" "$A_ENV" --stage 2 --adopt-image "sha256:$(printf '%064d' 9)" "$S2_BUNDLE")"; rc4=$?
  set -e
  [ "$rc1" -ne 0 ] && pass "--adopt-image 只限 --stage 2" || fail "--adopt-image 竟可用於 Stage 1"
  [ "$rc2" -ne 0 ] && pass "--adopt-image ⛔ 不收 tag，只收完整 image ID" || fail "--adopt-image 竟收了 tag"
  [ "$rc3" -ne 0 ] && pass "--adopt-image 重複 → 拒絕" || fail "--adopt-image 重複竟通過"
  [ "$rc4" -ne 0 ] && [ ! -e "$A_IDENTITY" ] \
    && pass "本機找不到要採用的 image → 拒絕、⛔ 不建立 identity" || fail "採用不存在的 image 竟通過"

  if [ -n "$A_ENV" ]; then
    A_BAD="$(sed 's/"python_version":"[^"]*"/"python_version":"3.11.99"/' <<< "$A_ENV")"
    set +e
    out="$(s2_adopt "$S2_TD/a5" "$A_BAD" --stage 2 --adopt-image "$A_ID" "$S2_BUNDLE")"; rc=$?
    set -e
    if [ "$rc" -ne 0 ] && [ -z "$out" ] && [ ! -e "$A_IDENTITY" ] \
       && ! grep -q '^tag\|^save\|^build' "$S2_TD/a5"; then
      pass "環境與 Stage 1 不同 → ⛔ 不採用（⛔ 沒有 tag、identity、tarball）"
    else
      fail "環境不同竟被採用：rc=$rc out='$out'"
    fi
    set +e
    out="$(s2_adopt "$S2_TD/a6" "$A_ENV" --stage 2 --adopt-image "$A_ID" "$S2_BUNDLE")"; rc=$?
    set -e
    A_HEX="$(printf '%064d' 3)"
    if [ "$rc" -eq 0 ] && [ "$out" = "$A_ID" ] && ! grep -q '^build' "$S2_TD/a6" \
       && grep -qx "tag $A_ID stock-trading-python-replay:i074-stage2" "$S2_TD/a6" \
       && grep -qF "\"expected_image_id\":\"$A_ID\"" "$A_IDENTITY" \
       && ( . "$REPO_ROOT/scripts/lib/image-tarball.sh"; image_tarball_verify \
              "$A_XDG/stock_trading/i074_stage2/images" "$A_HEX" ); then
      pass "環境相同 → 採用：⛔ 不 build、掛上專用 tag、identity 記的是它、tarball 可驗證"
    else
      fail "採用的正常分支：rc=$rc out='$out'"
      sed 's/^/    /' "$S2_TD/a6" >&2
    fi
    set +e
    out="$(s2_adopt "$S2_TD/a7" "$A_ENV" --stage 2 --adopt-image "sha256:$(printf '%064d' 4)" "$S2_BUNDLE")"; rc7=$?
    out8="$(s2_adopt "$S2_TD/a8" "$A_ENV" --stage 2 --adopt-image "$A_ID" "$S2_BUNDLE")"; rc8=$?
    set -e
    [ "$rc7" -ne 0 ] && pass "identity 已釘死後 --adopt-image 指向別的 image → 拒絕（⛔ 不換）" \
      || fail "identity 已釘死卻換了 image"
    [ "$rc8" -eq 0 ] && [ "$out8" = "$A_ID" ] && ! grep -q '^run\|^build' "$S2_TD/a8" \
      && pass "identity 已釘死、--adopt-image 同一個 → no-op（⛔ 不重跑環境比對）" \
      || fail "同一個 image 的 no-op 分支：rc=$rc8 out='$out8'"
  elif [ "${IMAGE_REQUIRED:-0}" = "1" ]; then
    fail "--adopt-image 測試：讀不到 Stage 1 的環境指紋（IMAGE_REQUIRED=1）"
  fi
elif [ "${IMAGE_REQUIRED:-0}" = "1" ]; then
  fail "pin --stage 2 測試：找不到正式 bundle（IMAGE_REQUIRED=1）"
fi

# ── run-replay-offline.sh 的 I074_STAGE ──────────────────────────────────
echo "==> i074 Stage 2：run-replay-offline.sh 的 I074_STAGE"
set +e
I074_STAGE=3 REPLAY_ARGS_SELFTEST=1 "$REPO_ROOT/scripts/run-replay-offline.sh" --bundle /b --output-dir /o \
  --before-ref HEAD --i074-preflight >/dev/null 2>&1; rc=$?
I074_STAGE=2 REPLAY_ARGS_SELFTEST=1 "$REPO_ROOT/scripts/run-replay-offline.sh" --bundle /b --output-dir /o \
  --before-ref HEAD >/dev/null 2>&1; rc2=$?
set -e
[ "$rc" -ne 0 ] && pass "I074_STAGE 只接受 1／2" || fail "I074_STAGE=3 竟通過"
[ "$rc2" -ne 0 ] && pass "I074_STAGE=2 ⛔ 只在 I-074 正式流程有意義" || fail "一般流程竟接受 I074_STAGE=2"
if [ -d "$S2_BUNDLE" ]; then
  set +e
  err="$(env -u PY_IMAGE I074_STAGE=2 REPLAY_DRY_RUN=1 REPLAY_IMAGE_ID="sha256:$(printf '%064d' 2)" \
      XDG_DATA_HOME="$S2_TD/emptyxdg" "$REPO_ROOT/scripts/run-replay-offline.sh" --bundle "$S2_BUNDLE" \
      --output-dir "$S2_TD/witness_out" --before-ref HEAD --i074-preflight 2>&1)"; rc=$?
  set -e
  [ "$rc" -ne 0 ] && grep -q "i074_stage2/run_identity.json" <<< "$err" \
    && pass "I074_STAGE=2 → 推導的是 Stage 2 的 identity 路徑" \
    || fail "I074_STAGE=2 的 identity 路徑不符：rc=$rc $(head -1 <<< "$err")"
fi

# ── 見證趟的 E7：**在 replay 之前**就擋（2026-09-23 review 高 1） ─────────────
#
# ⚠️ 錯設 AFTER_REF 或殘留 TOOLING_PATCH ⛔ 不得燒完一整趟 replay 才被拒——after' 只有一趟。
# 用 fake docker 跑**非 dry-run**：失敗時 log 裡⛔ 不得出現 `run`，正確時才出現。
echo "==> i074 Stage 2：見證趟的 E7 前置守門"
if [ -d "$S2_BUNDLE" ] && [ -n "$S2_IMG" ] && [ -f "${S2_ID:-/nonexistent}" ]; then
  mkdir -p "$S2_TD/e7bin"
  cat > "$S2_TD/e7bin/docker" <<'FAKE'
#!/usr/bin/env bash
printf '%s\n' "$1" >> "$FAKE_DOCKER_LOG"
case "$1" in
  image) printf '%s\n' "$3"; exit 0 ;;   # inspect：回傳被問的那個 ID
esac
exit 0
FAKE
  chmod +x "$S2_TD/e7bin/docker"
  # Stage 1 after 的 base commit（⚠️ 只讀檔頭，⛔ 不用 `zcat | head`——pipefail 下會 SIGPIPE 假失敗）
  S1_BASE="$(python3 -c '
import gzip, re, sys
head = gzip.open(sys.argv[1]).read(65536).decode("utf-8", "replace")
print(re.search(r"\"base_commit\":\"([0-9a-f]{40})\"", head).group(1))' \
    "$REPO_ROOT/python/baselines/i074_stage1/d1/after_artifact.json.gz")"
  printf '%s\n' 'diff --git a/zz_e7_probe.txt b/zz_e7_probe.txt' 'new file mode 100644' \
    '--- /dev/null' '+++ b/zz_e7_probe.txt' '@@ -0,0 +1 @@' '+e7' > "$S2_TD/e7.patch"
  e7_run() {  # $1＝log；$2＝AFTER_REF；$3＝TOOLING_PATCH（可空）
    : > "$1"
    env -u PY_IMAGE PATH="$S2_TD/e7bin:$PATH" FAKE_DOCKER_LOG="$1" I074_STAGE=2 \
      REPLAY_IMAGE_ID="$S2_IMG" XDG_DATA_HOME="$S2_TD/xdg" AFTER_REF="$2" TOOLING_PATCH="$3" \
      "$REPO_ROOT/scripts/run-replay-offline.sh" --bundle "$S2_BUNDLE" \
      --output-dir "$S2_TD/e7out.$(basename "$1")" --before-ref 'ecbc141^' --i074-preflight
  }
  set +e
  err="$(e7_run "$S2_TD/e7a" HEAD "" 2>&1)"; rca=$?
  errb="$(e7_run "$S2_TD/e7b" "$S1_BASE" "$S2_TD/e7.patch" 2>&1)"; rcb=$?
  # ⚠️ 放行的那一支用 dry-run：真的走到 `exec docker run` 時 runner 的 EXIT trap 不會執行，
  # worktree 會留在 /tmp。dry-run 印出的 `docker run` 就證明守門已放行。
  outc="$(REPLAY_DRY_RUN=1 e7_run "$S2_TD/e7c" "$S1_BASE" "" 2>/dev/null)"; rcc=$?
  set -e
  if [ "$(git -C "$REPO_ROOT" rev-parse HEAD)" != "$S1_BASE" ]; then
    [ "$rca" -ne 0 ] && grep -q "E7" <<< "$err" && ! grep -qx run "$S2_TD/e7a" \
      && pass "AFTER_REF 不是 Stage 1 base → replay 之前中止、⛔ 沒有 docker run" \
      || fail "錯的 AFTER_REF 沒有在 replay 之前被擋：rc=$rca"
  fi
  [ "$rcb" -ne 0 ] && grep -q "E7" <<< "$errb" && ! grep -qx run "$S2_TD/e7b" \
    && pass "見證趟套了 TOOLING_PATCH → replay 之前中止、⛔ 沒有 docker run" \
    || fail "殘留的 TOOLING_PATCH 沒有在 replay 之前被擋：rc=$rcb"
  [ "$rcc" -eq 0 ] && grep -qx run <<< "$outc" \
    && pass "正確的 base、無 patch → 守門放行，才走到 docker run" \
    || fail "正確的見證趟竟被擋下：rc=$rcc"
elif [ "${IMAGE_REQUIRED:-0}" = "1" ]; then
  fail "E7 前置守門測試：找不到 image、正式 bundle 或 Stage 2 identity（IMAGE_REQUIRED=1）"
fi
# ── I-074 Stage 2 步驟 ④：sizing harness 的 shim、wrapper 與參數檢查 ─────────────────
#
# 對應 issue.md I-074「Stage 2 步驟 ④：sizing harness 計畫書」（v7 ＋ 差異）的測試 a、a2～a7、b。
# ⚠️ ⛔ 任何一條都不得真的啟動 harness 的量測（那要跑數分鐘並建 clone）——harness 只測「在動手之前就拒絕」。
echo "==> i074 Stage 2 步驟 ④：sizing harness 的 docker shim"
SZ_TD="$(mktemp -d)"
SZ_SHIM="$REPO_ROOT/scripts/lib/i074-sizing-docker-shim.sh"
SZ_HELPER="$REPO_ROOT/python/scripts/i074_stage2_sizing.py"
SZ_IMG="sha256:$(printf '%064d' 5)"
mkdir -p "$SZ_TD/real" "$SZ_TD/cg1/memory" "$SZ_TD/cg2" "$SZ_TD/cgnone" "$SZ_TD/cg0/memory"
echo 123456789 > "$SZ_TD/cg1/memory/memory.max_usage_in_bytes"
echo 987654321 > "$SZ_TD/cg2/memory.peak"
echo 0 > "$SZ_TD/cg0/memory/memory.max_usage_in_bytes"
# fake 的「真正 docker」：記錄參數；run 時依 --cidfile 寫 CID，並在 host 上執行容器指令（含 wrapper，
# /peak 由 SIZING_PEAK_DIR 指到 -v 的來源、cgroup 由 SIZING_CGROUP_ROOT 指到假的 cgroup 樹）。
cat > "$SZ_TD/real/docker" <<'FAKE'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "$FAKE_LOG"
case "$1" in
  run)
    shift; args=("$@"); cidfile=""; peak=""; i=0
    [ -n "${FAKE_RUNARGS:-}" ] && printf '%s\0' "$@" > "$FAKE_RUNARGS"   # ⚠️ NUL 分隔：wrapper 內含換行
    while [ "$i" -lt "${#args[@]}" ]; do
      case "${args[$i]}" in
        --cidfile) cidfile="${args[$((i+1))]}"; i=$((i+2)) ;;
        -v) case "${args[$((i+1))]}" in *:/peak) peak="${args[$((i+1))]%:/peak}" ;; esac; i=$((i+2)) ;;
        "$FAKE_IMAGE") i=$((i+1)); break ;;
        *) i=$((i+1)) ;;
      esac
    done
    [ -n "$cidfile" ] && printf 'cid-%s' "$(basename "$cidfile" .cid)" > "$cidfile"
    [ -n "${FAKE_SLEEP:-}" ] && sleep "$FAKE_SLEEP"
    SIZING_PEAK_DIR="${peak:-$FAKE_NOPEAK}" SIZING_CGROUP_ROOT="${FAKE_CGROUP:-/nonexistent}" "${args[@]:i}"
    exit $? ;;
  inspect)
    [ -z "${FAKE_INSPECT_FAIL:-}" ] || exit 1
    case "$*" in
      *SizeRw*) echo "${FAKE_SIZE_RW:-0}" ;;
      *LogConfig*) echo "${FAKE_LOGCONFIG:-{\"Type\":\"json-file\",\"Config\":{}}}" ;;
      *HostConfig.Memory*) echo "${FAKE_MEMORY-465567744 465567744}" ;;   # ⚠️ `-`：空字串也照給（測「讀到空的」）
      *) echo '[{}]' ;;
    esac ;;
  logs) printf 'out\n'; printf 'err\n' >&2 ;;
esac
exit 0
FAKE
chmod +x "$SZ_TD/real/docker"
mkdir -p "$SZ_TD/bin"
ln -s "$SZ_SHIM" "$SZ_TD/bin/docker"
sz_run() {  # $1＝S 目錄；其餘＝docker 參數。stdout／stderr 分別寫進 $1.out／$1.err，回傳 shim 的結束碼
  local st="$1"; shift
  mkdir -p "$st"
  set +e
  env PATH="$SZ_TD/bin:$PATH" SIZING_REAL_DOCKER="$SZ_TD/real/docker" SIZING_STATE="$st" SIZING_RUN_ID=t \
      SIZING_IMAGE="$SZ_IMG" SIZING_PHASE="${SZ_PHASE:-success}" SIZING_ROLE="${SZ_ROLE:-finalizer}" \
      SIZING_INCLUDED="${SZ_INCLUDED:-true}" SIZING_HELPER="$SZ_HELPER" FAKE_LOG="$st.log" FAKE_IMAGE="$SZ_IMG" \
      FAKE_RUNARGS="$st.runargs" \
      FAKE_CGROUP="${SZ_CG:-$SZ_TD/cg1}" docker "$@" > "$st.out" 2> "$st.err"
  local rc=$?
  set -e
  return "$rc"
}
sz_sidecar() {  # $1＝S；$2＝python 運算式（變數 s＝sidecar）
  python3 -c 'import json,sys,glob; s=json.load(open(glob.glob(sys.argv[1]+"/containers/*.json")[0])); print(eval(sys.argv[2]))' "$1" "$2"
}
SZ_CMD=(sh -c 'printf "OUT\n"; printf "ERR\n" >&2; exit 3')

# a：改寫與 I/O 透明
rc=0; sz_run "$SZ_TD/sa" run --rm --network none -v /x:/y:ro "$SZ_IMG" "${SZ_CMD[@]}" || rc=$?
direct_out="$("${SZ_CMD[@]}" 2>/dev/null)" || true
[ "$rc" = 3 ] && pass "a：shim 原樣傳回容器的結束碼（3）" || fail "a：結束碼不符（rc=$rc）"
if [ "$(cat "$SZ_TD/sa.out")" = "$direct_out" ] && [ "$(cat "$SZ_TD/sa.err")" = "ERR" ] \
   && [ "$(wc -c < "$SZ_TD/sa.out")" = "4" ]; then
  pass "a3：經 shim 的 stdout 與未包裝的逐 byte 相同、stderr ⛔ 沒有被重複或加料"
else
  fail "a3：shim 汙染了 stdout／stderr"; od -c "$SZ_TD/sa.out" | head -3 >&2
fi
if python3 - "$SZ_TD/sa.runargs" "$SZ_TD/sa" "$SZ_IMG" <<'PY'
import sys
args = open(sys.argv[1], "rb").read().decode().split("\0")[:-1]
st, img = sys.argv[2], sys.argv[3]
pos = args.index(img)
want_opts = ["--network", "none", "-v", "/x:/y:ro", "--cidfile", f"{st}/cid/o0010.cid", "--name", "i074sz-t-o0010",
             "--read-only", "-v", f"{st}/peak/o0010:/peak"]
assert args[:pos] == want_opts, args[:pos]
assert args[pos + 1:pos + 3] == ["sh", "-c"] and "memory.max_usage_in_bytes" in args[pos + 3], args[pos + 1:pos + 4]
assert args[pos + 4:] == ["_", "sh", "-c", r'printf "OUT\n"; printf "ERR\n" >&2; exit 3'], args[pos + 4:]
PY
then
  pass "a：改寫＝原 option 逐 token 保留 ＋ cidfile／name／--read-only／/peak、⛔ --rm、指令逐 token 包進 wrapper"
else
  fail "a：改寫後的 argv 不符"
fi
if python3 - "$SZ_TD/sa.runargs" "$SZ_IMG" <<'PY'
import sys
args = open(sys.argv[1], "rb").read().decode().split("\0")[:-1]
opts = args[:args.index(sys.argv[2])]
targets = [opts[i + 1].split(":")[1] for i, t in enumerate(opts) if t == "-v"]
assert "/tmp" not in targets and "--tmpfs" not in opts and not any(t.startswith("--mount") for t in opts), opts
PY
then
  pass "a6：shim ⛔ 沒有提供任何 /tmp 掛載（bind 或 tmpfs）"
else
  fail "a6：shim 提供了可寫的 /tmp"
fi
[ "$(sz_sidecar "$SZ_TD/sa" 's["status"], s["rc"], s["peak_bytes"], s["log_bound"] > 0')" = "('ok', 3, 123456789, True)" ] \
  && pass "a4：sidecar 記錄結束碼、cgroup v1 峰值與 log 上界" || fail "a4：sidecar 內容不符：$(sz_sidecar "$SZ_TD/sa" 's')"
set +e
env PATH="$SZ_TD/bin:$PATH" SIZING_REAL_DOCKER="$SZ_TD/real/docker" FAKE_LOG="$SZ_TD/pass.log" \
  docker image inspect whatever >/dev/null 2>&1
set -e
grep -qx 'image inspect whatever' "$SZ_TD/pass.log" && pass "a：非 run 的子指令原樣轉給真正的 docker" \
  || fail "a：非 run 的子指令沒有原樣轉交"

# a2：wrapper 的 v1 → v2 → 缺 → 0
rc=0; SZ_CG="$SZ_TD/cg2" sz_run "$SZ_TD/sv2" run "$SZ_IMG" true || rc=$?
[ "$rc" = 0 ] && [ "$(sz_sidecar "$SZ_TD/sv2" 's["peak_bytes"]')" = 987654321 ] \
  && pass "a2：v1 缺、v2 存在 → 讀 memory.peak" || fail "a2：v2 fallback 不符"
rc=0; SZ_CG="$SZ_TD/cgnone" sz_run "$SZ_TD/svn" run "$SZ_IMG" sh -c 'exit 7' || rc=$?
[ "$rc" = 7 ] && [ "$(sz_sidecar "$SZ_TD/svn" 's["status"]')" = measure_failed ] \
  && pass "a2：兩者皆缺 → 量測失敗、原結束碼（7）保留" || fail "a2：兩者皆缺時不符（rc=$rc）"
rc=0; SZ_CG="$SZ_TD/cg0" sz_run "$SZ_TD/sv0" run "$SZ_IMG" true || rc=$?
[ "$(sz_sidecar "$SZ_TD/sv0" 's["status"]')" = measure_failed ] && pass "a2：峰值為 0 → 量測失敗（⛔ 不當成 0）" \
  || fail "a2：峰值為 0 竟被接受"
if grep -q 'memory/memory.max_usage_in_bytes' "$SZ_SHIM" && grep -q 'memory.peak' "$SZ_SHIM" \
   && grep -q 'memory/memory.max_usage_in_bytes' "$REPO_ROOT/scripts/run-replay-offline.sh" \
   && grep -q 'memory.peak' "$REPO_ROOT/scripts/run-replay-offline.sh"; then
  pass "a2：wrapper 的候選路徑與 run-replay-offline.sh 的 MEASURE_PEAK 相同"
else
  fail "a2：wrapper 的候選路徑與 runner 不同"
fi

# a4：inspect 失敗 → 量測失敗，但結束碼照樣傳回
rc=0; FAKE_INSPECT_FAIL=1 sz_run "$SZ_TD/sif" run "$SZ_IMG" sh -c 'exit 4' || rc=$?
[ "$rc" = 4 ] && [ "$(sz_sidecar "$SZ_TD/sif" 's["status"]')" = measure_failed ] \
  && pass "a4：inspect 失敗 → sidecar 標量測失敗、shim 仍傳回原結束碼" || fail "a4：inspect 失敗時不符（rc=$rc）"
rc=0; FAKE_LOGCONFIG='{"Type":"local","Config":{}}' sz_run "$SZ_TD/slc" run "$SZ_IMG" true || rc=$?
[ "$(sz_sidecar "$SZ_TD/slc" 's["status"]')" = measure_failed ] && pass "a8：log driver 不是 json-file → 量測失敗" \
  || fail "a8：非 json-file 竟被接受"
rc=0; FAKE_LOGCONFIG='{"Type":"json-file","Config":{"max-size":"10m"}}' sz_run "$SZ_TD/slr" run "$SZ_IMG" true || rc=$?
[ "$(sz_sidecar "$SZ_TD/slr" 's["status"]')" = measure_failed ] && pass "a8：log 有 rotation → 量測失敗" \
  || fail "a8：log rotation 竟被接受"
rc=0; FAKE_SIZE_RW=4096 sz_run "$SZ_TD/srw" run "$SZ_IMG" true || rc=$?
[ "$(sz_sidecar "$SZ_TD/srw" 's["size_rw"]')" = 4096 ] && pass "a4：SizeRw 原樣記錄（report 對非 0 fail-closed）" \
  || fail "a4：SizeRw 沒有記錄"
# ⑦d 實作第一輪 review #2：daemon 實際套用的記憶體上限（HostConfig.Memory／MemorySwap）照實記錄；讀不懂 → 量測失敗
[ "$(sz_sidecar "$SZ_TD/srw" '(s["status"], s["memory_limit_bytes"], s["memory_swap_limit_bytes"])')" = "('ok', 465567744, 465567744)" ] \
  && pass "a4：sidecar 記下容器實際的記憶體上限（inspect 的 Memory／MemorySwap）" || fail "a4：sidecar 沒有記下記憶體上限"
rc=0; FAKE_MEMORY="465567744 465567744 1" sz_run "$SZ_TD/smem3" run "$SZ_IMG" true || rc=$?
[ "$rc" = 0 ] && [ "$(sz_sidecar "$SZ_TD/smem3" 's["status"]')" = measure_failed ] \
  && grep -q 上限讀不到 <<< "$(sz_sidecar "$SZ_TD/smem3" 's["failures"]')" \
  && pass "a4：記憶體上限讀不懂（多出 token）→ 量測失敗、結束碼照傳" || fail "a4：讀不懂的記憶體上限竟被接受（rc=$rc）"
rc=0; FAKE_MEMORY="" sz_run "$SZ_TD/smem0" run "$SZ_IMG" true || rc=$?
[ "$rc" = 0 ] && [ "$(sz_sidecar "$SZ_TD/smem0" 's["status"]')" = measure_failed ] \
  && pass "a4：記憶體上限是空的 → 量測失敗（⛔ 不寫成 0）" || fail "a4：空的記憶體上限竟被接受（rc=$rc）"

# a5：中斷——容器還在跑時送 SIGTERM，shim 以 cidfile 記錄的 CID docker rm -f
# ⚠️ 以 setsid 放進自己的 process group：shim 就是 group leader（$sz_pid），收尾時只對這個 group 送 KILL，
#   ⛔ 不用 pkill -f——以指令字串比對會連呼叫端（例如命令列裡含同一段字串的 shell）一起殺掉。
mkdir -p "$SZ_TD/sint"
setsid env PATH="$SZ_TD/bin:$PATH" SIZING_REAL_DOCKER="$SZ_TD/real/docker" SIZING_STATE="$SZ_TD/sint" SIZING_RUN_ID=t \
    SIZING_IMAGE="$SZ_IMG" SIZING_PHASE=success SIZING_ROLE=finalizer SIZING_INCLUDED=true SIZING_HELPER="$SZ_HELPER" \
    FAKE_LOG="$SZ_TD/sint.log" FAKE_IMAGE="$SZ_IMG" FAKE_CGROUP="$SZ_TD/cg1" FAKE_SLEEP=30 \
    docker run "$SZ_IMG" true > /dev/null 2>&1 &
sz_pid=$!
for _ in $(seq 1 100); do [ -s "$SZ_TD/sint/cid/o0010.cid" ] && break; sleep 0.1; done
kill -TERM "$sz_pid" 2>/dev/null || true
set +e; wait "$sz_pid"; set -e
if grep -qx 'rm -f cid-o0010' "$SZ_TD/sint.log" && [ "$(sz_sidecar "$SZ_TD/sint" '"TERM" in " ".join(s["failures"])')" = True ]; then
  pass "a5：SIGTERM → 以 cidfile 的 CID docker rm -f、sidecar 標中斷"
else
  fail "a5：中斷時沒有以 CID 移除容器"; cat "$SZ_TD/sint.log" >&2
fi
kill -KILL -- "-$sz_pid" 2>/dev/null || true

# a7：metadata twin（真的 Docker；長 argv 與長 mount path）
if [ -n "$S2_IMG" ]; then
  st7="$SZ_TD/s7"
  long_arg="$(python3 -c 'print("a" * 33000)')"
  long_src="$SZ_TD/$(python3 -c 'print("/".join(["d" * 200] * 10))')"
  python3 "$SZ_HELPER" index --state "$st7" --sequence 1 --phase witness --role fixture --included true \
    --image "$S2_IMG" -- --cidfile "$st7/cid/o0010.cid" --name i074sz-t-o0010 --read-only -v "$long_src:/x" \
    "$S2_IMG" true "$long_arg"
  if python3 "$SZ_HELPER" twins --state "$st7" --docker "$(command -v docker)" --run-id t --fs-path "$SZ_TD" \
     && python3 -c '
import json,sys
t=json.load(open(sys.argv[1]))
assert t["status"]=="ok", t
assert t["adopted_bytes"] >= max(t["raw_bytes"]) and t["adopted_bytes"] >= 2*max(t["inspect_lengths"]) >= 66000, t
' "$st7/twins/o0010.json"; then
    pass "a7：長 argv／長 mount path 的 twin——採用值 ≥ 原始值、≥ 2 × inspect 長度"
  else
    fail "a7：metadata twin 不符"
  fi
  [ -z "$(docker ps -a --filter name=i074sz-t- -q)" ] && pass "a7：twin 全部移除" || fail "a7：留下了 twin 容器"

  # a6：封閉寫入（真的 Docker）：寫 /tmp、寫 /、tempfile 都失敗；一般指令照常
  sz_real() {  # $1＝S；其餘＝容器指令
    local st="$1"; shift
    mkdir -p "$st"
    set +e
    env PATH="$SZ_TD/bin:$PATH" SIZING_REAL_DOCKER="$(command -v docker)" SIZING_STATE="$st" SIZING_RUN_ID=t \
        SIZING_IMAGE="$S2_IMG" SIZING_PHASE=success SIZING_ROLE=finalizer SIZING_INCLUDED=true SIZING_HELPER="$SZ_HELPER" \
        docker run --rm --network none --user "$(id -u):$(id -g)" "$S2_IMG" "$@" >/dev/null 2>&1
    local rc=$?
    set -e
    return "$rc"
  }
  sz_real "$SZ_TD/r1" sh -c 'echo ok' && pass "a6：對照組——一般指令照常成功" || fail "a6：對照組失敗"
  sz_real "$SZ_TD/r2" sh -c 'touch /tmp/x' && fail "a6：竟然寫得了 /tmp" || pass "a6：寫 /tmp → 失敗（唯讀）"
  sz_real "$SZ_TD/r3" sh -c 'touch /x' && fail "a6：竟然寫得了 /" || pass "a6：寫 / → 失敗（唯讀）"
  sz_real "$SZ_TD/r4" python -c 'import tempfile; tempfile.gettempdir()' && fail "a6：tempfile 竟可用" \
    || pass "a6：tempfile.gettempdir() → 失敗（⛔ 沒有可寫的暫存位置）"
elif [ "${IMAGE_REQUIRED:-0}" = "1" ]; then
  fail "sizing 的真 Docker 測試：找不到 image（IMAGE_REQUIRED=1）"
fi

# b：harness 在動手之前就拒絕
echo "==> i074 Stage 2 步驟 ④：sizing harness 的參數與 work 目錄防護"
SZ_H="$REPO_ROOT/scripts/i074-stage2-sizing.sh"
sz_rejects() {  # $1＝說明；$2＝stderr 必須含的字串；其餘＝參數（⚠️ 一律帶 REPLAY_IMAGE_ID，除非另外清空）
  local label="$1" want="$2" rc=0; shift 2
  set +e
  env -u PY_IMAGE REPLAY_IMAGE_ID="${SZ_IMAGE_OVERRIDE-$S2_IMG}" "$SZ_H" "$@" > /dev/null 2> "$SZ_TD/b.err"; rc=$?
  set -e
  if [ "$rc" -ne 0 ] && grep -q -- "$want" "$SZ_TD/b.err"; then pass "$label"; else fail "$label（rc=$rc）"; cat "$SZ_TD/b.err" >&2; fi
}
ln -s "$REPO_ROOT" "$SZ_TD/repo-link"
mkdir "$SZ_TD/exists"
SZ_IMAGE_OVERRIDE="" sz_rejects "b：沒有 REPLAY_IMAGE_ID → 拒絕" "REPLAY_IMAGE_ID" --work-dir "$SZ_TD/w1"
if [ -n "$S2_IMG" ]; then
  sz_rejects "b：work 目錄在 repo 內 → 拒絕" "repo 內" --work-dir "$REPO_ROOT/sizing-should-not-exist"
  sz_rejects "b：parent symlink 指回 repo → 拒絕" "repo 內" --work-dir "$SZ_TD/repo-link/sizing-x"
  sz_rejects "b：work 目錄已存在 → 拒絕（⛔ 不覆蓋）" "已存在" --work-dir "$SZ_TD/exists"
  sz_rejects "b：--work-dir 重複 → 拒絕" "重複" --work-dir "$SZ_TD/w2" --work-dir "$SZ_TD/w3"
  sz_rejects "b：未知參數 → 拒絕" "未知參數" --work-dir "$SZ_TD/w4" --bogus
  I074_SIZING_FAULT=bogus sz_rejects "b：I074_SIZING_FAULT 不認得的值 → 拒絕" "只接受" --work-dir "$SZ_TD/w5"
  [ ! -e "$REPO_ROOT/sizing-should-not-exist" ] && [ ! -e "$SZ_TD/w2" ] && [ ! -e "$SZ_TD/w4" ] && [ ! -e "$SZ_TD/w5" ] \
    && pass "b：被拒絕時⛔ 沒有建立任何 work 目錄" || fail "b：被拒絕後仍留下 work 目錄"
  # --formal：在隔離的最小 repo 裡測（⛔ 不碰真正的 repo，也⛔ 不讓測試真的開始量測）
  SZ_REPO="$SZ_TD/frepo"
  mkdir -p "$SZ_REPO/scripts/lib" "$SZ_REPO/python/scripts"
  cp "$SZ_H" "$SZ_REPO/scripts/"; cp "$SZ_SHIM" "$REPO_ROOT/scripts/lib/mem-guard.sh" "$SZ_REPO/scripts/lib/"
  cp "$REPO_ROOT/scripts/lib/i074-stage2-measure.sh" "$SZ_REPO/scripts/lib/"      # ⑦d：共用原語（快照清單 ①）
  cp "$SZ_HELPER" "$SZ_REPO/python/scripts/"
  # ⚠️ ⑦b：freeze record 的寫入端與它 import 的常數也在 `--formal` 的「檔案 ＝ HEAD」清單裡（n10）。
  cp "$REPO_ROOT/python/scripts/i074_stage2_freeze_record.py" "$REPO_ROOT/python/scripts/i074_stage2_preflight.py" \
     "$SZ_REPO/python/scripts/"
  git -C "$SZ_REPO" init -q
  git -C "$SZ_REPO" add -A
  git -C "$SZ_REPO" -c user.name=t -c user.email=t@t -c commit.gpgsign=false commit -qm fixture
  sz_formal() {  # $1＝說明；$2＝stderr 必須含的字串；stdout 導到 /dev/null、stderr 導到 tmpfs
    local label="$1" want="$2" rc=0 err="/dev/shm/sz-formal-$$.err"
    set +e
    env -u PY_IMAGE REPLAY_IMAGE_ID="$S2_IMG" "$SZ_REPO/scripts/i074-stage2-sizing.sh" --formal \
      --work-dir "$SZ_TD/fw" > /dev/null 2> "$err"; rc=$?
    set -e
    if [ "$rc" -ne 0 ] && grep -q -- "$want" "$err" && [ ! -e "$SZ_TD/fw" ]; then pass "$label"
    else fail "$label（rc=$rc）"; cat "$err" >&2; fi
    rm -f "$err"
  }
  printf 'dirty\n' >> "$SZ_REPO/python/scripts/i074_stage2_sizing.py"
  # ⚠️ ⑦d：helper 在快照清單 ① 內——bootstrap 在建 S 之前就以「與 HEAD 的內容不同」拒絕（n10 的下一支驗 python/ 的 clean 檢查）
  sz_formal "b：--formal 時 harness 的檔案有未 commit 的變更 → 在建 S 之前拒絕" "與 HEAD 的內容不同"
  git -C "$SZ_REPO" checkout -q -- python/scripts/i074_stage2_sizing.py
  printf 'dirty\n' >> "$SZ_REPO/python/scripts/i074_stage2_freeze_record.py"
  sz_formal "n10：--formal 時 freeze record 的寫入端有未 commit 的變更 → 拒絕" "未 commit"
  git -C "$SZ_REPO" checkout -q -- python/scripts/i074_stage2_freeze_record.py
  git -C "$SZ_REPO" rm -q --cached python/scripts/i074_stage2_preflight.py
  git -C "$SZ_REPO" -c user.name=t -c user.email=t@t -c commit.gpgsign=false commit -qm untrack-preflight
  printf 'python/scripts/i074_stage2_preflight.py\n' > "$SZ_REPO/.git/info/exclude"
  sz_formal "n10：--formal 時 freeze record 依賴的 preflight 常數模組未進版控 → 拒絕" "尚未進版控"
  : > "$SZ_REPO/.git/info/exclude"
  git -C "$SZ_REPO" add python/scripts/i074_stage2_preflight.py
  git -C "$SZ_REPO" -c user.name=t -c user.email=t@t -c commit.gpgsign=false commit -qm retrack-preflight
  git -C "$SZ_REPO" rm -q --cached scripts/lib/i074-sizing-docker-shim.sh
  git -C "$SZ_REPO" -c user.name=t -c user.email=t@t -c commit.gpgsign=false commit -qm untrack
  printf 'scripts/lib/i074-sizing-docker-shim.sh\n' > "$SZ_REPO/.git/info/exclude"
  sz_formal "b：--formal 時 harness 檔案未進版控 → 拒絕" "尚未進版控"
  I074_SIZING_FAULT=twins sz_formal "b：--formal ⛔ 不接受演練用的故障注入" "故障注入"
  # ⚠️ ⑦d：下一支要打到 fd 的守門——先把 shim 重新納入版控（否則 bootstrap 的「清單 ① 未進版控」會先擋下、遮蔽它）。
  : > "$SZ_REPO/.git/info/exclude"
  git -C "$SZ_REPO" add scripts/lib/i074-sizing-docker-shim.sh
  git -C "$SZ_REPO" -c user.name=t -c user.email=t@t -c commit.gpgsign=false commit -qm retrack-shim
  set +e
  env -u PY_IMAGE REPLAY_IMAGE_ID="$S2_IMG" "$SZ_REPO/scripts/i074-stage2-sizing.sh" --formal \
    --work-dir "$SZ_TD/fw2" > "$SZ_TD/formal.out" 2> "/dev/shm/sz-fd-$$.err"; rc=$?
  set -e
  [ "$rc" -ne 0 ] && [ ! -e "$SZ_TD/fw2" ] && grep -q "fd 1 被導到" "/dev/shm/sz-fd-$$.err" \
    && pass "b：--formal 時 stdout 導到量測中的檔案系統 → 拒絕（打到 fd 的守門）" \
    || { fail "b：--formal 沒有擋下導到 L0 的輸出（rc=$rc）"; cat "/dev/shm/sz-fd-$$.err" >&2; }
  rm -f "/dev/shm/sz-fd-$$.err"
fi
# 容器清理失敗（review）：helper 與 fallback 的 rm -f 都失敗 → S 與 cidfile 保留，摘要與 stderr 列出 CID。
# ⚠️ fake docker：image inspect 成功（⛔ 不需要真的 image）、rm 一律失敗、inspect 顯示容器仍在、ps 沒有具名容器。
mkdir -p "$SZ_TD/nodel"
cat > "$SZ_TD/nodel/docker" <<'FAKE'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "$NODEL_LOG"
case "$1" in
  image|inspect|ps) exit 0 ;;
  rm) exit 1 ;;
esac
exit 0
FAKE
chmod +x "$SZ_TD/nodel/docker"
set +e
env -u PY_IMAGE PATH="$SZ_TD/nodel:$PATH" NODEL_LOG="$SZ_TD/nodel.log" I074_SIZING_FAULT=cleanup \
  REPLAY_IMAGE_ID="sha256:$(printf '%064d' 6)" "$SZ_H" --work-dir "$SZ_TD/wclean" > /dev/null 2> "$SZ_TD/clean.err"
rc=$?
set -e
kept="$(grep -o '/dev/shm/i074-sizing-[^ ）]*' "$SZ_TD/clean.err" | tail -1)"
if [ "$rc" -ne 0 ] && [ -n "$kept" ] && [ "$(cat "$kept/cid/o0000.cid" 2>/dev/null)" = "fault-injected-cid" ] \
   && grep -qx 'fault-injected-cid' "$SZ_TD/clean.err" \
   && [ "$(grep -c '^rm -f fault-injected-cid$' "$SZ_TD/nodel.log")" -ge 2 ] \
   && python3 -c 'import json,sys; assert json.load(open(sys.argv[1]))["leftover_containers"]==["fault-injected-cid"]' \
        "$SZ_TD/wclean/failure_summary.json"; then
  pass "a12：容器移除不了 → S 與 cidfile 保留、摘要與 stderr 列出 CID（helper 與 fallback 都試過 rm -f）"
else
  fail "a12：容器清理失敗時沒有保留 S（rc=$rc）"; cat "$SZ_TD/clean.err" >&2
fi
[ -n "$kept" ] && [ -d "$kept" ] && rm -rf "$kept"

# 步驟收不掉與成功路徑的 docker ps（review）：fake docker 的 ps 由 FAKE_PS_RC 控制，其餘一律成功、容器都已不在。
mkdir -p "$SZ_TD/okd"
cat > "$SZ_TD/okd/docker" <<'FAKE'
#!/usr/bin/env bash
case "$1" in
  image|rm) exit 0 ;;
  inspect) exit 1 ;;
  ps) exit "${FAKE_PS_RC:-0}" ;;
esac
exit 0
FAKE
chmod +x "$SZ_TD/okd/docker"
sz_fault() {  # $1＝故障；$2＝work 目錄名；stderr 寫到 $SZ_TD/<名>.err，回傳 harness 的結束碼
  local rc=0
  set +e
  env -u PY_IMAGE PATH="${SZ_FAULT_PATH:+$SZ_FAULT_PATH:}$SZ_TD/okd:$PATH" I074_SIZING_FAULT="$1" \
    REPLAY_IMAGE_ID="sha256:$(printf '%064d' 6)" \
    "$SZ_H" --work-dir "$SZ_TD/$2" > /dev/null 2> "$SZ_TD/$2.err"; rc=$?
  set -e
  return "$rc"
}
sz_pgid() {  # harness 在 stderr 印出的 process group（⛔ 不用 pgrep -f／pkill -f 以指令字串找程序）
  grep -o 'process group [0-9]*' "$1" | head -1 | grep -o '[0-9]*$' || true
}
# ⚠️ 耗時上限：注入的步驟是 `sleep 60`、演練的窗口是 2 ＋ 1 秒——超過 30 秒代表收尾又被 `wait` leader 卡住
#   （第三輪修正時實測過：先 wait 會一直等到 sleep 自己結束，逾時與 KILL 永遠輪不到）。
t0=$SECONDS; rc=0; sz_fault stuck wstuck || rc=$?; el=$((SECONDS - t0))
pg="$(sz_pgid "$SZ_TD/wstuck.err")"
if [ "$rc" = 1 ] && [ -n "$pg" ] && [ "$el" -lt 30 ] && grep -q "升級 KILL" "$SZ_TD/wstuck.err" \
   && grep -q "原始量測：" "$SZ_TD/wstuck.err" && ! pgrep -g "$pg" >/dev/null 2>&1 \
   && [ -f "$SZ_TD/wstuck/failure_summary.json" ]; then
  pass "a12：忽略 TERM 的步驟 → 升級 KILL、group 確實結束後才照常清 S（${el} 秒）"
else
  fail "a12：忽略 TERM 的步驟沒有被收掉（rc=$rc、${el} 秒）"; cat "$SZ_TD/wstuck.err" >&2
fi
[ -n "$pg" ] && kill -KILL -- "-$pg" 2>/dev/null || true
t0=$SECONDS; rc=0; sz_fault group-alive walive || rc=$?; el=$((SECONDS - t0))
pg="$(sz_pgid "$SZ_TD/walive.err")"
kept="$(grep -o '/dev/shm/i074-sizing-[^ ）]*' "$SZ_TD/walive.err" | tail -1)"
if [ "$rc" = 1 ] && [ "$el" -lt 30 ] && [ -n "$kept" ] && [ -d "$kept" ] && grep -q "步驟結束（0）" "$SZ_TD/walive.err"; then
  pass "a12：KILL 之後 group 仍有成員 → 保留 S（仍存活的程序可能還在寫它）"
else
  fail "a12：group 沒結束卻清掉了 S（rc=$rc）"; cat "$SZ_TD/walive.err" >&2
fi
[ -n "$kept" ] && [ -d "$kept" ] && rm -rf "$kept"
[ -n "$pg" ] && kill -KILL -- "-$pg" 2>/dev/null || true
# host 的 ps 失敗 → 查不到 group 的成員 ⇒ ⛔ 不能當作已結束：S 保留
mkdir -p "$SZ_TD/psfail"; printf '#!/bin/sh\nexit 1\n' > "$SZ_TD/psfail/ps"; chmod +x "$SZ_TD/psfail/ps"
rc=0; SZ_FAULT_PATH="$SZ_TD/psfail" sz_fault stuck wpsx || rc=$?
pg="$(sz_pgid "$SZ_TD/wpsx.err")"
kept="$(grep -o '/dev/shm/i074-sizing-[^ ）]*' "$SZ_TD/wpsx.err" | tail -1)"
if [ "$rc" = 1 ] && [ -n "$kept" ] && [ -d "$kept" ] && grep -q "步驟結束（0）" "$SZ_TD/wpsx.err"; then
  pass "a12：host 的 ps 失敗 → 不當作 group 已結束、保留 S"
else
  fail "a12：ps 失敗竟被當成 group 已結束（rc=$rc）"; cat "$SZ_TD/wpsx.err" >&2
fi
[ -n "$kept" ] && [ -d "$kept" ] && rm -rf "$kept"
[ -n "$pg" ] && kill -KILL -- "-$pg" 2>/dev/null || true
# leader 自己退出、背景子程序仍在同一個 group 裡（第四輪 review）：不論結束碼符不符合預期，都要以同一個 PGID 收尾並中止
for kind in ok bad; do
  rc=0; sz_fault "orphan-$kind" "worph$kind" || rc=$?
  pg="$(sz_pgid "$SZ_TD/worph$kind.err")"
  if [ "$kind" = ok ]; then want_stage='fault_orphan：leader 已結束（結束碼 0）但 process group'; want_rc=1
  else want_stage='fault_orphan'; want_rc=7; fi
  if [ "$rc" = 1 ] && [ -n "$pg" ] && grep -q "收尾：process group $pg（成員：[0-9]" "$SZ_TD/worph$kind.err" \
     && ! pgrep -g "$pg" >/dev/null 2>&1 \
     && python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); assert d["failed_stage"].startswith(sys.argv[2]) and d["rc"]==int(sys.argv[3]), d' \
          "$SZ_TD/worph$kind/failure_summary.json" "$want_stage" "$want_rc"; then
    pass "a12：leader 以$([ "$kind" = ok ] && echo 預期 || echo 非預期)的結束碼退出、背景子程序仍存活 → 以同一個 PGID 收尾並中止"
  else
    fail "a12：leader 退出後殘留的 group 成員沒有被收尾（orphan-$kind，rc=$rc）"; cat "$SZ_TD/worph$kind.err" >&2
  fi
  [ -n "$pg" ] && kill -KILL -- "-$pg" 2>/dev/null || true
done
rc=0; FAKE_PS_RC=1 sz_fault final-check wps || rc=$?
if [ "$rc" = 1 ] && grep -q "docker ps 失敗" "$SZ_TD/wps/failure_summary.json" && [ ! -e "$SZ_TD/wps/sizing_report.json" ]; then
  pass "成功路徑：docker ps 失敗 → fail-closed（⛔ 查不到不能當作沒有殘留）"
else
  fail "成功路徑：docker ps 失敗竟被當成沒有殘留（rc=$rc）"; cat "$SZ_TD/wps.err" >&2
fi
rc=0; sz_fault final-check wps0 || rc=$?
[ "$rc" = 0 ] && pass "成功路徑：ps 成功且沒有本次的容器 → 通過（對照組）" || fail "對照組失敗（rc=$rc）"
for k in $(ls /dev/shm | grep '^i074-sizing-' || true); do
  grep -qF "/dev/shm/$k" "$SZ_TD"/*.err 2>/dev/null && rm -rf "/dev/shm/$k"
done

rm -rf "$SZ_TD"

# ── I-074 Stage 2 ⑦d：acceptance harness（shim 的 profile、守門、互斥、快照與 bootstrap） ─────────────────────
#
# 對應 issue.md I-074「Stage 2 步驟 ⑦d 細部計畫 v1」「五」的 ac4、ac7、ac8、ac11、ac17、ac18。
# ⚠️ 任何一條都⛔ 不得真的開始量測（⛔ 不建 clone、⛔ 不跑容器）：harness 只測「在動手之前就拒絕」、bootstrap 與注入的中止點；
#   需要 git HEAD 的情境在隔離的最小 repo 裡跑（⛔ 不改真正的 repo）。測試自己建的 S（/dev/shm/i074-*）由測試自己收拾。
echo "==> i074 Stage 2 ⑦d：sizing shim 的 acceptance profile"
AC_TD="$(mktemp -d)"
AC_SHIM="$REPO_ROOT/scripts/lib/i074-sizing-docker-shim.sh"
AC_HELPER="$REPO_ROOT/python/scripts/i074_stage2_sizing.py"
AC_IMG="sha256:$(printf '%064d' 5)"
mkdir -p "$AC_TD/real"
# fake 的「真正 docker」：run 只錄下參數（NUL 分隔）、寫 CID 與 /peak，⛔ 不執行容器指令；inspect／logs／rm 都成功。
cat > "$AC_TD/real/docker" <<'FAKE'
#!/usr/bin/env bash
case "$1" in
  run)
    shift; printf '%s\0' "$@" > "$AC_RUNARGS"; args=("$@")
    for i in "${!args[@]}"; do
      case "${args[$i]}" in
        --cidfile) printf 'cid-x' > "${args[$((i + 1))]}" ;;
        -v) case "${args[$((i + 1))]}" in *:/peak) echo 123456789 > "${args[$((i + 1))]%:/peak}/peak" ;; esac ;;
      esac
    done
    exit 0 ;;
  inspect) case "$*" in *SizeRw*) echo 4096 ;; *LogConfig*) echo '{"Type":"json-file","Config":{}}' ;;
                        *HostConfig.Memory*) echo '465567744 465567744' ;; *) echo '[{}]' ;; esac ;;
esac
exit 0
FAKE
chmod +x "$AC_TD/real/docker"
ac_shim() {  # $1＝S；其餘＝docker 參數。回傳 shim 的結束碼；runargs 在 $1.runargs
  local st="$1" rc=0; shift
  mkdir -p "$st/harness/python/scripts"
  : > "$st/harness/python/scripts/i074_stage2_replay_stub.py"
  set +e
  env SIZING_REAL_DOCKER="$AC_TD/real/docker" SIZING_STATE="$st" SIZING_RUN_ID=t SIZING_IMAGE="$AC_IMG" \
      SIZING_PHASE="${AC_PHASE:-success}" SIZING_ROLE="${AC_ROLE:-replay}" SIZING_INCLUDED="${AC_INCLUDED:-true}" \
      SIZING_HELPER="$AC_HELPER" SIZING_PROFILE="${AC_PROFILE-acceptance}" SIZING_REPLAY_MODE="${AC_MODE-success}" \
      SIZING_REPLAY_COMPUTE="${AC_COMPUTE-stub}" ${AC_NOCF:+SIZING_NOCF_PYTHON="$AC_NOCF"} AC_RUNARGS="$st.runargs" \
      "$AC_SHIM" "$@" > /dev/null 2>&1
  rc=$?
  set -e
  return "$rc"
}
AC_EVAL=(python -m backtest.modular.sr_scoring.evaluation --x 1 --y)
rc=0; ac_shim "$AC_TD/s1" run --rm --network none -v /wt/python:/app:ro -w /app "$AC_IMG" "${AC_EVAL[@]}" || rc=$?
if [ "$rc" = 0 ] && python3 - "$AC_TD/s1" "$AC_IMG" <<'PY'
import json, sys
st, img = sys.argv[1], sys.argv[2]
args = open(st + ".runargs", "rb").read().decode().split("\0")[:-1]
pos = args.index(img)
want = ["--network", "none", "-v", "/wt/python:/app:ro", "-w", "/app",
        "-v", f"{st}/harness/python/scripts/i074_stage2_replay_stub.py:/acceptance/replay_stub.py:ro",
        "-e", "I074_ACCEPTANCE_REPLAY=success", "-e", "I074_ACCEPTANCE_COMPUTE=stub",
        "--cidfile", f"{st}/cid/o0010.cid", "--name", "i074sz-t-o0010", "-v", f"{st}/peak/o0010:/peak"]
assert args[:pos] == want, args[:pos]
assert args[pos + 1:pos + 3] == ["sh", "-c"] and args[pos + 4:] == ["_", "python", "/acceptance/replay_stub.py", "--x", "1", "--y"], args[pos:]
assert json.load(open(st + "/index/0001.json"))["profile"] == "acceptance"
PY
then
  pass "ac4：acceptance 的 replay → ⛔ 沒有 --read-only、指令換成快照裡的 launcher（加唯讀掛載與兩個模式變數）、其餘逐 token 不變"
else
  fail "ac4：acceptance 的 replay 改寫不符（rc=$rc）"
fi
rc=0; AC_ROLE=finalizer ac_shim "$AC_TD/s2" run --rm -v /wt/python:/app:ro "$AC_IMG" python -m x --finalize || rc=$?
if [ "$rc" = 0 ] && python3 - "$AC_TD/s2" "$AC_IMG" <<'PY'
import sys
st, img = sys.argv[1], sys.argv[2]
args = open(st + ".runargs", "rb").read().decode().split("\0")[:-1]
pos = args.index(img)
assert "--read-only" not in args[:pos] and args[pos + 4:] == ["_", "python", "-m", "x", "--finalize"], args
PY
then pass "ac4：acceptance 的其他 role → ⛔ 沒有 --read-only、容器指令逐 token 不變"; else fail "ac4：acceptance 的 finalizer 改寫不符（rc=$rc）"; fi
rc=0; AC_COMPUTE=full AC_NOCF=/nocf/python ac_shim "$AC_TD/s3" run -v /wt/python:/app:ro "$AC_IMG" "${AC_EVAL[@]}" || rc=$?
if [ "$rc" = 0 ] && python3 - "$AC_TD/s3" "$AC_IMG" <<'PY'
import sys
st, img = sys.argv[1], sys.argv[2]
args = open(st + ".runargs", "rb").read().decode().split("\0")[:-1]
opts = args[:args.index(img)]
assert "/nocf/python:/app:ro" in opts and "/wt/python:/app:ro" not in opts and "I074_ACCEPTANCE_COMPUTE=full" in opts, opts
PY
then pass "ac4：full → 恰好換掉 /app 的來源（不套 counterfactual 的 worktree）"; else fail "ac4：full 的 /app 換錯（rc=$rc）"; fi
ac_rejects() {  # $1＝說明；其餘＝docker 參數；預期 125、⛔ 沒有執行 docker run
  local label="$1" st rc=0; shift
  st="$AC_TD/r$RANDOM"
  ac_shim "$st" "$@" || rc=$?
  if [ "$rc" = 125 ] && [ ! -e "$st.runargs" ]; then pass "$label"; else fail "$label（rc=$rc）"; fi
}
AC_COMPUTE=full AC_NOCF=/nocf/python ac_rejects "ac4：full 但沒有 /app 的掛載 → 125" run "$AC_IMG" "${AC_EVAL[@]}"
AC_COMPUTE=full AC_NOCF=/nocf/python ac_rejects "ac4：full 但有兩個 /app 的掛載 → 125" run -v /a:/app:ro -v /b:/app:ro "$AC_IMG" "${AC_EVAL[@]}"
AC_COMPUTE=full ac_rejects "ac4：full 但沒有 SIZING_NOCF_PYTHON → 125" run -v /a:/app:ro "$AC_IMG" "${AC_EVAL[@]}"
ac_rejects "ac4：replay 的指令開頭不是 evaluation → 125" run "$AC_IMG" python -m backtest.modular.sr_scoring.other
AC_MODE=bogus ac_rejects "ac4：SIZING_REPLAY_MODE 非法 → 125" run "$AC_IMG" "${AC_EVAL[@]}"
AC_MODE=failure AC_COMPUTE=full AC_NOCF=/n ac_rejects "ac4：full 只限 success → 125" run -v /a:/app:ro "$AC_IMG" "${AC_EVAL[@]}"
AC_PROFILE=bogus ac_rejects "ac4：未知的 SIZING_PROFILE → 125" run "$AC_IMG" "${AC_EVAL[@]}"
AC_PROFILE="" AC_ROLE=finalizer AC_INCLUDED=true ac_shim "$AC_TD/s4" run -v /x:/y "$AC_IMG" python -m x || true
python3 - "$AC_TD/s4" "$AC_IMG" <<'PY' && pass "ac4：profile 未設定 ＝ sizing（照舊加 --read-only）" || fail "ac4：未設定 profile 的行為變了"
import sys
st, img = sys.argv[1], sys.argv[2]
args = open(st + ".runargs", "rb").read().decode().split("\0")[:-1]
assert "--read-only" in args[:args.index(img)], args
PY

# ac7、ac8：harness 在動手之前就拒絕（fake docker：image inspect 成功；identity 放在測試自己的 XDG）
echo "==> i074 Stage 2 ⑦d：acceptance harness 的參數、守門與互斥"
ACC_H="$REPO_ROOT/scripts/i074-stage2-acceptance.sh"
mkdir -p "$AC_TD/okd" "$AC_TD/xdg/stock_trading/i074_stage2" "$AC_TD/empty-xdg"
printf '{}\n' > "$AC_TD/xdg/stock_trading/i074_stage2/run_identity.json"
cat > "$AC_TD/okd/docker" <<'FAKE'
#!/usr/bin/env bash
case "$1" in image|rm) exit 0 ;; inspect) exit 1 ;; ps) exit 0 ;; info) echo /var/lib/docker ;; esac
exit 0
FAKE
chmod +x "$AC_TD/okd/docker"
ac_shm_count() { ls /dev/shm | grep -c '^i074-accept-' || true; }
acc_rejects() {  # $1＝說明；$2＝stderr 必須含的字串；其餘＝參數
  local label="$1" want="$2" rc=0 before after; shift 2
  before="$(ac_shm_count)"
  set +e
  env -u PY_IMAGE PATH="${AC_PATH:-$AC_TD/okd:$PATH}" XDG_DATA_HOME="${AC_XDG:-$AC_TD/xdg}" \
    REPLAY_IMAGE_ID="${AC_IMAGE_OVERRIDE-$AC_IMG}" ${AC_ENV:-} "$ACC_H" "$@" > /dev/null 2> "$AC_TD/acc.err"; rc=$?
  set -e
  after="$(ac_shm_count)"
  if [ "$rc" -ne 0 ] && grep -q -- "$want" "$AC_TD/acc.err" && [ "$before" = "$after" ]; then pass "$label"
  else fail "$label（rc=$rc、S $before→$after）"; cat "$AC_TD/acc.err" >&2; fi
}
ln -s "$REPO_ROOT" "$AC_TD/repo-link"
mkdir "$AC_TD/exists"
AC_IMAGE_OVERRIDE="" acc_rejects "ac7：沒有 REPLAY_IMAGE_ID → 拒絕" "REPLAY_IMAGE_ID" --work-dir "$AC_TD/w1"
acc_rejects "ac7：work 目錄在 repo 內 → 拒絕" "repo 內" --work-dir "$REPO_ROOT/acceptance-should-not-exist"
acc_rejects "ac7：parent symlink 指回 repo → 拒絕" "repo 內" --work-dir "$AC_TD/repo-link/acc-x"
acc_rejects "ac7：work 目錄已存在 → 拒絕（⛔ 不覆蓋）" "已存在" --work-dir "$AC_TD/exists"
acc_rejects "ac7：--work-dir 重複 → 拒絕" "重複" --work-dir "$AC_TD/w2" --work-dir "$AC_TD/w3"
acc_rejects "ac7：未知參數 → 拒絕" "未知參數" --work-dir "$AC_TD/w4" --bogus
acc_rejects "ac7：--replay-compute 非法 → 拒絕" "只接受 stub" --work-dir "$AC_TD/w5" --replay-compute fast
acc_rejects "ac7：--replay-compute 重複 → 拒絕" "重複" --work-dir "$AC_TD/w6" --replay-compute stub --replay-compute full
I074_SIZING_FAULT=bogus acc_rejects "ac7：I074_SIZING_FAULT 不認得的值 → 拒絕" "只接受" --work-dir "$AC_TD/w7"
AC_XDG="$AC_TD/empty-xdg" acc_rejects "ac7：找不到 Stage 2 的 run identity → 拒絕" "run identity" --work-dir "$AC_TD/w8"
AC_ENV="I074_STAGE2_TOKEN=x" acc_rejects "ac8：環境帶 I074_STAGE2_* → 中止（與 ⑩ 的 label shim 互斥）" "互斥" --work-dir "$AC_TD/w9"
mkdir -p "$AC_TD/lsbin"
printf '#!/bin/bash -p\n# I074-STAGE2-LABEL-SHIM\nexit 0\n' > "$AC_TD/lsbin/docker"; chmod +x "$AC_TD/lsbin/docker"
AC_PATH="$AC_TD/lsbin:$PATH" acc_rejects "ac8：PATH 上的 docker 是 label shim → 中止" "label shim" --work-dir "$AC_TD/w10"
[ ! -e "$REPO_ROOT/acceptance-should-not-exist" ] && ! ls -d "$AC_TD"/w[0-9]* >/dev/null 2>&1 \
  && pass "ac7：被拒絕時⛔ 沒有建立任何 work 目錄" || fail "ac7：被拒絕後仍留下 work 目錄"
rc=0
env SIZING_REAL_DOCKER="$AC_TD/real/docker" SIZING_STATE="$AC_TD/s5" SIZING_PROFILE=acceptance I074_STAGE2_TOKEN=x \
  "$AC_SHIM" ps > /dev/null 2>&1 || rc=$?
[ "$rc" = 125 ] && pass "ac8：acceptance profile 的 shim 在環境帶 I074_STAGE2_* 時 125" || fail "ac8：shim 沒有拒絕（rc=$rc）"

# ac11、ac17、ac18：隔離的最小 repo（HEAD 有 harness 與 ⑩ 的正式程式）；⛔ 不碰真正的 repo
echo "==> i074 Stage 2 ⑦d：快照、bootstrap 與注入的中止點（隔離的最小 repo）"
AC_REPO="$AC_TD/arepo"
for f in scripts/i074-stage2-acceptance.sh scripts/i074-stage2-sizing.sh scripts/lib/i074-stage2-measure.sh \
         scripts/lib/i074-sizing-docker-shim.sh scripts/lib/mem-guard.sh python/scripts/i074_stage2_sizing.py \
         python/scripts/i074_stage2_replay_stub.py scripts/run-replay-offline.sh scripts/finalize-stage2-evidence.sh \
         scripts/lib/replay-args.sh scripts/lib/i074-stage2-supervisor.py python/scripts/i074_stage2_preflight.py \
         python/scripts/i074_stage2_promote.py python/scripts/_i074_bootstrap.py python/scripts/i074_stage2_freeze_record.py; do
  mkdir -p "$AC_REPO/$(dirname "$f")"; cp -p "$REPO_ROOT/$f" "$AC_REPO/$f"
done
git -C "$AC_REPO" init -q && git -C "$AC_REPO" add -A
git -C "$AC_REPO" -c user.name=t -c user.email=t@t -c commit.gpgsign=false commit -qm fixture
AC_FIX="$(git -C "$AC_REPO" rev-parse HEAD)"
# PATH 上包裝的 cp：$AC_CP_MODE＝after（複製之後改掉／刪掉原檔）、before（複製之前改掉原檔，給 --formal 的後驗）或
# commit（複製之前在隔離 repo 加一個空 commit——bootstrap 解析完 HEAD 之後 HEAD 才移動；內容⛔ 不變）。
# ⚠️ after 的改法是「會讓程式壞掉」的（python 與被 source 的 shell 都在第二行插入 raise SystemExit(99)／return 99）——
#   harness 只要有一處用到活路徑的檔案就會失敗，測試因此證明得了「用的是快照」。
mkdir -p "$AC_TD/cpw"
cat > "$AC_TD/cpw/cp" <<'WRAP'
#!/usr/bin/env bash
src="${@: -2:1}"
case "$src" in
  */python/scripts/i074_stage2_replay_stub.py|*/python/scripts/i074_stage2_sizing.py|*/scripts/lib/i074-sizing-docker-shim.sh|*/scripts/lib/mem-guard.sh)
    case "$src" in "$AC_CP_REPO"/*) ;; *) exec /bin/cp "$@" ;; esac
    if [ "$AC_CP_MODE" = before ]; then printf '\n# tampered\n' >> "$src"; exec /bin/cp "$@"; fi
    if [ "$AC_CP_MODE" = commit ]; then
      git -C "$AC_CP_REPO" -c user.name=t -c user.email=t@t -c commit.gpgsign=false commit -q --allow-empty -m moved || exit 98
      exec /bin/cp "$@"
    fi
    /bin/cp "$@" || exit $?
    case "$src" in
      *replay_stub.py) rm -f -- "$src" ;;
      *.py) sed -i '2i raise SystemExit(99)  # tampered' "$src" ;;   # ⚠️ 放在開頭：結尾的 raise SystemExit(main()) 之後永遠執行不到
      *) sed -i '2i return 99 2>/dev/null || exit 99  # tampered' "$src" ;;
    esac
    exit 0 ;;
esac
exec /bin/cp "$@"
WRAP
chmod +x "$AC_TD/cpw/cp"
ac_isolated() {  # $1＝入口（相對 AC_REPO）；$2＝work 目錄名；其餘＝參數。stderr → $AC_TD/<名>.err
  local entry="$1" name="$2" rc=0; shift 2
  set +e
  env -u PY_IMAGE PATH="${AC_ISO_PATH:-$AC_TD/okd:$PATH}" XDG_DATA_HOME="$AC_TD/xdg" REPLAY_IMAGE_ID="$AC_IMG" \
    AC_CP_REPO="$AC_REPO" AC_CP_MODE="${AC_CP_MODE:-after}" "$AC_REPO/$entry" --work-dir "$AC_TD/$name" "$@" \
    > /dev/null 2> "/dev/shm/ac-$$-$name.err"; rc=$?
  set -e
  mv "/dev/shm/ac-$$-$name.err" "$AC_TD/$name.err"
  return "$rc"
}
ac_kept() { grep '保留' "$1" | grep -o '/dev/shm/i074-[a-z]*-[0-9TZ-]*' | tail -1 || true; }   # 只認「保留」的訊息
ac_started() { grep -o 'S=/dev/shm/i074-[a-z]*-[0-9TZ-]*' "$1" | head -1 | cut -d= -f2 || true; }
ac_head_sha() { git -C "$AC_REPO" show "HEAD:$1" | sha256sum | cut -d' ' -f1; }
# ac11 ＋ ac18：快照建立之後改掉／刪掉原始的 launcher、helper、shim → 中止點的清理、原始量測與 MANIFEST 都是快照的
rc=0; I074_SIZING_FAULT=prepare AC_ISO_PATH="$AC_TD/cpw:$AC_TD/okd:$PATH" ac_isolated scripts/i074-stage2-acceptance.sh wprep || rc=$?
raw="$AC_TD/wprep/raw-failed/harness"
s_path="$(ac_started "$AC_TD/wprep.err")"
if [ "$rc" = 1 ] && [ -f "$AC_TD/wprep/failure_summary.json" ] && [ -z "$(ac_kept "$AC_TD/wprep.err")" ] \
   && [ -n "$s_path" ] && [ ! -e "$s_path" ] \
   && [ ! -e "$AC_REPO/python/scripts/i074_stage2_replay_stub.py" ] \
   && grep -q tampered "$AC_REPO/python/scripts/i074_stage2_sizing.py" \
   && [ "$(sha256sum < "$raw/python/scripts/i074_stage2_replay_stub.py" | cut -d' ' -f1)" = "$(ac_head_sha python/scripts/i074_stage2_replay_stub.py)" ] \
   && [ "$(sha256sum < "$raw/python/scripts/i074_stage2_sizing.py" | cut -d' ' -f1)" = "$(ac_head_sha python/scripts/i074_stage2_sizing.py)" ] \
   && grep -q "$(ac_head_sha scripts/lib/i074-sizing-docker-shim.sh)  scripts/lib/i074-sizing-docker-shim.sh" "$raw/MANIFEST"; then
  pass "ac11／ac18：注入的中止點 → raw-failed 與 failure_summary、S 已清；快照之後改掉或刪掉原檔⛔ 不影響本次（用的是快照）"
else
  fail "ac11／ac18：中止點或快照不符（rc=$rc）"; cat "$AC_TD/wprep.err" >&2
fi
git -C "$AC_REPO" checkout -q -- . && git -C "$AC_REPO" status --porcelain | grep -q . && fail "隔離 repo 沒還原" || true
# sizing：同一支（另驗 mem-guard 的快照）；`cleanup` 中止點在 work 目錄建好之後
rc=0; I074_SIZING_FAULT=cleanup AC_ISO_PATH="$AC_TD/cpw:$AC_TD/okd:$PATH" ac_isolated scripts/i074-stage2-sizing.sh szprep || rc=$?
raw="$AC_TD/szprep/raw-failed/harness"
if [ "$rc" = 1 ] && [ -f "$AC_TD/szprep/failure_summary.json" ] && grep -q tampered "$AC_REPO/scripts/lib/mem-guard.sh" \
   && [ "$(sha256sum < "$raw/scripts/lib/mem-guard.sh" | cut -d' ' -f1)" = "$(ac_head_sha scripts/lib/mem-guard.sh)" ]; then
  pass "ac18：sizing 的快照（含 mem-guard）——快照之後改掉原檔⛔ 不影響本次"
else
  fail "ac18：sizing 的快照不符（rc=$rc）"; cat "$AC_TD/szprep.err" >&2
fi
git -C "$AC_REPO" checkout -q -- .
if grep -q 'FREEZE="\$CLONE/python/scripts/i074_stage2_freeze_record.py"' "$REPO_ROOT/scripts/i074-stage2-sizing.sh" \
   && grep -q -- '--repo "\$CLONE"' "$REPO_ROOT/scripts/i074-stage2-sizing.sh" \
   && ! grep -q 'REPO_ROOT/python/scripts/i074_stage2_freeze_record' "$REPO_ROOT/scripts/i074-stage2-sizing.sh"; then
  pass "ac18：sizing 的 freeze record 由工作複本執行（⛔ 不從原始 repo 的活路徑）"
else
  fail "ac18：sizing 的 freeze record 仍取自活路徑"
fi
# ⑦d 實作第一輪 review #1：bootstrap 解析完 HEAD 之後 HEAD 才移動 → 快照、兩層工作複本與 repo_head 全部綁 bootstrap 的 OID
for entry in acceptance sizing; do
  rc=0; AC_CP_MODE=commit AC_ISO_PATH="$AC_TD/cpw:$AC_TD/okd:$PATH" ac_isolated "scripts/i074-stage2-$entry.sh" "whead-$entry" || rc=$?
  meta_f="$AC_TD/whead-$entry/raw-failed/meta.tsv"
  real_ok=1
  [ "$entry" = sizing ] || [ "$(git -C "$AC_TD/whead-$entry/real" rev-parse HEAD 2>/dev/null)" = "$AC_FIX" ] || real_ok=0
  if [ "$rc" = 1 ] && [ "$(git -C "$AC_REPO" rev-parse HEAD)" != "$AC_FIX" ] && [ -f "$meta_f" ] \
     && grep -qx "repo_head	$AC_FIX" "$meta_f" && grep -qx "clone_head	$AC_FIX" "$meta_f" \
     && [ "$(git -C "$AC_TD/whead-$entry/repo" rev-parse HEAD 2>/dev/null)" = "$AC_FIX" ] && [ "$real_ok" = 1 ]; then
    pass "review1：$entry——bootstrap 之後 HEAD 移動 → repo_head、clone_head 與工作複本都是 bootstrap 解析的 OID（⛔ 不是移動後的 HEAD）"
  else
    fail "review1：$entry 沒有綁住 bootstrap 的 OID（rc=$rc）"; cat "$AC_TD/whead-$entry.err" >&2; cat "$meta_f" >&2 2>/dev/null || true
  fi
  git -C "$AC_REPO" reset -q --hard "$AC_FIX"
done
shm0="$(ac_shm_count)"
rc=0; AC_CP_MODE=commit AC_ISO_PATH="$AC_TD/cpw:$AC_TD/okd:$PATH" ac_isolated scripts/i074-stage2-acceptance.sh wfh1 --formal --replay-compute full || rc=$?
if [ "$rc" = 1 ] && grep -q "HEAD 在 bootstrap 之後移動了" "$AC_TD/wfh1.err" && [ -z "$(ac_kept "$AC_TD/wfh1.err")" ] \
   && [ "$(ac_shm_count)" = "$shm0" ] && [ ! -e "$AC_TD/wfh1" ]; then
  pass "review1：--formal 時 HEAD 在 bootstrap 之後移動 → 拒絕（建 work 目錄之前；S 已確認是本次的快照、已清）"
else
  fail "review1：--formal 沒擋下 bootstrap 之後移動的 HEAD（rc=$rc）"; cat "$AC_TD/wfh1.err" >&2
fi
git -C "$AC_REPO" reset -q --hard "$AC_FIX"
rc=0; AC_CP_MODE=commit AC_ISO_PATH="$AC_TD/cpw:$AC_TD/okd:$PATH" ac_isolated scripts/i074-stage2-sizing.sh wfh2 --formal || rc=$?
[ "$rc" = 1 ] && grep -q "HEAD 在 bootstrap 之後移動了" "$AC_TD/wfh2.err" && [ ! -e "$AC_TD/wfh2" ] \
  && pass "review1：sizing --formal 時 HEAD 在 bootstrap 之後移動 → 拒絕" \
  || { fail "review1：sizing --formal 沒擋下移動的 HEAD（rc=$rc）"; cat "$AC_TD/wfh2.err" >&2; }
git -C "$AC_REPO" reset -q --hard "$AC_FIX"
# 對照組：HEAD 沒有移動的 --formal 通過這一道（之後在隔離 repo 缺的東西上失敗），repo_head ＝ 原本的 HEAD
rc=0; ac_isolated scripts/i074-stage2-acceptance.sh wfh3 --formal --replay-compute full || rc=$?
[ "$rc" = 1 ] && ! grep -q "HEAD 在 bootstrap 之後移動了" "$AC_TD/wfh3.err" \
  && grep -qx "mode	formal" "$AC_TD/wfh3/raw-failed/meta.tsv" && grep -qx "repo_head	$AC_FIX" "$AC_TD/wfh3/raw-failed/meta.tsv" \
  && pass "review1：對照組——HEAD 沒有移動的 --formal 通過這一道" \
  || { fail "review1：對照組不符（rc=$rc）"; cat "$AC_TD/wfh3.err" >&2; }
# --formal：清單 ① 在快照前後都驗 ＝ HEAD；必須帶 full；⛔ 不接受故障注入
rc=0; ac_isolated scripts/i074-stage2-acceptance.sh wf1 --formal || rc=$?
[ "$rc" = 1 ] && grep -q "必須帶 --replay-compute full" "$AC_TD/wf1.err" && [ -z "$(ac_kept "$AC_TD/wf1.err")" ] \
  && pass "ac7：--formal 沒帶 --replay-compute full → 拒絕" || { fail "ac7：--formal 沒帶 full 卻沒拒絕（rc=$rc）"; cat "$AC_TD/wf1.err" >&2; }
rc=0; I074_SIZING_FAULT=twins ac_isolated scripts/i074-stage2-acceptance.sh wf2 --formal --replay-compute full || rc=$?
[ "$rc" = 1 ] && grep -q "故障注入" "$AC_TD/wf2.err" && [ -z "$(ac_kept "$AC_TD/wf2.err")" ] \
  && pass "ac7：--formal ⛔ 不接受故障注入（在建 S 之前就拒絕）" || fail "ac7：--formal 接受了故障注入（rc=$rc）"
printf '\n# dirty\n' >> "$AC_REPO/python/scripts/i074_stage2_replay_stub.py"
rc=0; ac_isolated scripts/i074-stage2-acceptance.sh wf3 --formal --replay-compute full || rc=$?
[ "$rc" = 1 ] && grep -q "與 HEAD 的內容不同" "$AC_TD/wf3.err" && [ -z "$(ac_kept "$AC_TD/wf3.err")" ] \
  && pass "ac7：--formal 時清單內的檔案 ≠ HEAD → 在建 S 之前就拒絕" || fail "ac7：--formal 沒擋下 ≠ HEAD 的檔案（rc=$rc）"
git -C "$AC_REPO" checkout -q -- .
rc=0; AC_CP_MODE=before AC_ISO_PATH="$AC_TD/cpw:$AC_TD/okd:$PATH" ac_isolated scripts/i074-stage2-acceptance.sh wf4 --formal --replay-compute full || rc=$?
kept="$(ac_kept "$AC_TD/wf4.err")"
if [ "$rc" = 1 ] && grep -q "複製前後被改過" "$AC_TD/wf4.err" && [ -n "$kept" ] && [ -f "$kept/harness/python/scripts/i074_stage2_sizing.py" ]; then
  pass "ac18：--formal 的後驗（複製前後被改過）→ 1、bootstrap ⛔ 不刪任何東西（S 保留並印出位置）"
else
  fail "ac18：--formal 的後驗不符（rc=$rc）"; cat "$AC_TD/wf4.err" >&2
fi
[ -n "$kept" ] && rm -rf -- "$kept"
git -C "$AC_REPO" checkout -q -- .
# bootstrap 的失敗：複製、MANIFEST（validation 的故障注入）→ 1、S 與已寫的檔案保留、印出位置
for fault in bootstrap-copy bootstrap-manifest; do
  rc=0; I074_SIZING_FAULT="$fault" ac_isolated scripts/i074-stage2-acceptance.sh "wb-$fault" || rc=$?
  kept="$(ac_kept "$AC_TD/wb-$fault.err")"
  if [ "$rc" = 1 ] && [ -n "$kept" ] && [ -d "$kept/harness" ] && grep -q "不刪任何東西" "$AC_TD/wb-$fault.err"; then
    pass "ac18：bootstrap 失敗（$fault）→ 1、⛔ 不刪任何東西（S 保留並印出位置）"
  else
    fail "ac18：bootstrap 失敗（$fault）的處置不符（rc=$rc）"; cat "$AC_TD/wb-$fault.err" >&2
  fi
  [ -n "$kept" ] && rm -rf -- "$kept"
done
# bootstrap 期間收到 INT／TERM（故障注入讓 bootstrap 停在建好 S 之後）→ 130／143、S 保留
for sig in INT TERM; do
  want=$([ "$sig" = INT ] && echo 130 || echo 143)
  before="$(ls /dev/shm | grep '^i074-accept-' || true)"
  # ⚠️ 非互動的 shell 裡，背景工作的 SIGINT 一開始就是 ignore（bash 也無法 trap 進場時被 ignore 的訊號）——以 python 把 SIGINT
  #   還原成預設、setsid 之後再 exec harness，訊號才送得到（⑩ 的操作者在終端機按 Ctrl-C 時本來就是預設）。
  python3 -c 'import os, signal, sys; signal.signal(signal.SIGINT, signal.SIG_DFL); os.setsid(); os.execvp(sys.argv[1], sys.argv[1:])' \
    env -u PY_IMAGE PATH="$AC_TD/okd:$PATH" XDG_DATA_HOME="$AC_TD/xdg" REPLAY_IMAGE_ID="$AC_IMG" I074_SIZING_FAULT=bootstrap-stall \
    "$AC_REPO/scripts/i074-stage2-acceptance.sh" --work-dir "$AC_TD/wsig$sig" > /dev/null 2> "$AC_TD/wsig$sig.err" &
  bpid=$!
  new=""
  for _ in $(seq 1 100); do
    new="$(comm -13 <(printf '%s\n' $before | sort) <(ls /dev/shm | grep '^i074-accept-' | sort) | head -1)"
    [ -n "$new" ] && break
    sleep 0.1
  done
  sleep 0.3
  kill "-$sig" -- "-$bpid" 2>/dev/null || true
  rc=0; wait "$bpid" || rc=$?
  if [ "$rc" = "$want" ] && [ -n "$new" ] && [ -d "/dev/shm/$new" ] && grep -q "不刪任何東西" "$AC_TD/wsig$sig.err"; then
    pass "ac18：bootstrap 期間收到 $sig → $want、S 保留並印出位置"
  else
    fail "ac18：bootstrap 期間收到 $sig 的處置不符（rc=$rc、S=$new）"; cat "$AC_TD/wsig$sig.err" >&2
  fi
  [ -n "$new" ] && rm -rf -- "/dev/shm/$new"
done
# 手動設定 SIZING_SNAPSHOT：①指向既有目錄（主腳本不在它底下）；②內容完整合法的快照、之後的驗證才失敗；③MANIFEST 的 SHA 不符
mkdir -p "$AC_TD/manual"; printf keep > "$AC_TD/manual/f"; ino="$(stat -c %i "$AC_TD/manual/f")"
rc=0; env SIZING_SNAPSHOT="$AC_TD/manual" "$ACC_H" --work-dir "$AC_TD/wm1" > /dev/null 2> "$AC_TD/wm1.err" || rc=$?
[ "$rc" = 1 ] && [ "$(cat "$AC_TD/manual/f")" = keep ] && [ "$(stat -c %i "$AC_TD/manual/f")" = "$ino" ] \
  && pass "ac18：手動設定 SIZING_SNAPSHOT 指向既有目錄 → 1、該目錄⛔ 不變（⛔ 不能以環境變數跳過快照）" \
  || fail "ac18：手動設定的 SIZING_SNAPSHOT 沒有被擋下或被改了（rc=$rc）"
ac_manual_snapshot() {  # $1＝毀損（見下面的 case；ok／head ＝ 不毀損）→ 印出 S 與 run id
  local id s f
  id="$(date -u +%Y%m%dT%H%M%SZ)-9$RANDOM"
  s="/dev/shm/i074-accept-$id"
  mkdir -m 700 "$s"
  for f in scripts/i074-stage2-acceptance.sh scripts/lib/i074-stage2-measure.sh scripts/lib/i074-sizing-docker-shim.sh \
           python/scripts/i074_stage2_sizing.py python/scripts/i074_stage2_replay_stub.py; do
    mkdir -p "$s/harness/$(dirname "$f")"; cp "$REPO_ROOT/$f" "$s/harness/$f"
  done
  (cd "$s/harness" && sha256sum -- scripts/i074-stage2-acceptance.sh scripts/lib/i074-stage2-measure.sh \
     scripts/lib/i074-sizing-docker-shim.sh python/scripts/i074_stage2_sizing.py python/scripts/i074_stage2_replay_stub.py) \
    > "$s/harness/MANIFEST"
  case "$1" in
    extra) printf x > "$s/harness/scripts/extra.sh" ;;
    sha) local c; c="$(head -c 1 "$s/harness/MANIFEST")"; sed -i "1s/^./$([ "$c" = a ] && echo b || echo a)/" "$s/harness/MANIFEST" ;;
    # ⑦d 實作第一輪 review #3：S 的形狀——根目錄的兄弟檔案、額外的空目錄（根目錄與 harness 底下）、FIFO、symlink
    sibling) printf x > "$s/sibling.txt" ;;
    rootdir) mkdir "$s/other" ;;
    emptydir) mkdir "$s/harness/python/extra.d" ;;
    fifo) mkfifo "$s/harness/scripts/pipe" ;;
    symlink) ln -s "$REPO_ROOT/CLAUDE.md" "$s/harness/scripts/link" ;;
    # 形狀檢查必須在讀任何檔案內容之前：MANIFEST 換成 FIFO——先讀它的話會卡住（由 timeout 收掉、結束碼 137）
    manifest-fifo) rm -f "$s/harness/MANIFEST"; mkfifo "$s/harness/MANIFEST" ;;
  esac
  printf '%s %s\n' "$s" "$id"
}
AC_REAL_HEAD="$(git -C "$REPO_ROOT" rev-parse HEAD)"
for spec in "extra:S 的形狀" "sha:與 MANIFEST 不符" "sibling:S 的形狀" "rootdir:S 的形狀" "emptydir:S 的形狀" \
            "fifo:S 的形狀" "symlink:S 的形狀" "manifest-fifo:S 的形狀" "head:SIZING_BOOT_HEAD"; do
  dmg="${spec%%:*}"; want="${spec#*:}"
  read -r s id < <(ac_manual_snapshot "$dmg")
  before="$(find "$s" -printf '%y %i %s %p\n' | sort)"
  rc=0
  env -u REPLAY_IMAGE_ID SIZING_SNAPSHOT="$s" SIZING_ORIGIN_REPO="$REPO_ROOT" SIZING_BOOT_RUN_ID="$id" \
    SIZING_BOOT_HEAD="$([ "$dmg" = head ] && printf '%040d' 0 || echo "$AC_REAL_HEAD")" \
    timeout -s KILL 30 /bin/bash "$s/harness/scripts/i074-stage2-acceptance.sh" \
    --work-dir "$AC_TD/wm-$dmg" > /dev/null 2> "$AC_TD/wm-$dmg.err" || rc=$?
  if [ "$rc" = 1 ] && [ "$(find "$s" -printf '%y %i %s %p\n' | sort)" = "$before" ] && grep -q "不刪任何東西" "$AC_TD/wm-$dmg.err" \
     && grep -q -- "$want" "$AC_TD/wm-$dmg.err"; then
    pass "ac18：手動準備的快照（$dmg）→ re-exec 的驗證失敗（$want）、⛔ 不刪任何東西（S 的每一項、inode 不變）"
  else
    fail "ac18：手動準備的快照（$dmg）的處置不符（rc=$rc）"; cat "$AC_TD/wm-$dmg.err" >&2
  fi
  rm -rf -- "$s"
done
# 對照組：形狀與內容都合法的手動快照通過 bootstrap 的驗證（之後在 REPLAY_IMAGE_ID 失敗）→ S 由 trap 清掉（已裁決的界線）
read -r s id < <(ac_manual_snapshot ok)
rc=0
env -u REPLAY_IMAGE_ID SIZING_SNAPSHOT="$s" SIZING_ORIGIN_REPO="$REPO_ROOT" SIZING_BOOT_RUN_ID="$id" SIZING_BOOT_HEAD="$AC_REAL_HEAD" \
  timeout -s KILL 60 /bin/bash "$s/harness/scripts/i074-stage2-acceptance.sh" \
  --work-dir "$AC_TD/wm-ok" > /dev/null 2> "$AC_TD/wm-ok.err" || rc=$?
if [ "$rc" = 1 ] && [ ! -e "$s" ] && grep -q REPLAY_IMAGE_ID "$AC_TD/wm-ok.err" && ! grep -q "不刪任何東西" "$AC_TD/wm-ok.err"; then
  pass "ac18：對照組——合法的手動快照通過形狀與內容的驗證（之後的失敗才清掉 S）"
else
  fail "ac18：合法的手動快照竟沒通過 bootstrap 的驗證（rc=$rc）"; cat "$AC_TD/wm-ok.err" >&2
fi
rm -rf -- "$s"
rm -rf "$AC_TD"

# ── I-074 Stage 2 ⑦a：Stage 2 反事實 argv（真正 repo、真正的兩份 patch、dry-run） ─────────────
#
# ⚠️ 真的在 `e1cbbbd` 建 worktree、套上**已封存的** counterfactual 與**版控中的** tooling patch：
# 同時證明兩份 patch 能依固定順序合成、raw ＝ canonical，而 argv 與 fixture 逐 token 相同。
echo "==> i074 Stage 2（⑦a）：Stage 2 反事實 argv 與 fixture 相同（真正 repo）"
S7R_FIXTURE="$REPO_ROOT/python/scripts/fixtures/stage2_argv.json"
S7R_CF="$REPO_ROOT/python/baselines/i074_stage2/counterfactual_e1cbbbd.patch"
S7R_TOOL="$REPO_ROOT/python/baselines/i074_stage2/tooling_e1cbbbd.patch"
S7R_BASE=e1cbbbdab44f8cf2d152e6ade9235d844f590d7f
if [ -n "$S2_IMG" ] && [ -d "$S2_BUNDLE" ] && [ -f "${S2_ID:-/nonexistent}" ]; then
  if [ ! -f "$S7R_TOOL" ]; then
    fail "找不到 $S7R_TOOL——要依版控流程（scripts/make-i074-tooling-patch.sh）產生並 stage"
  else
    mkdir -p "$S2_TD/s7r"
    : > "$S2_TD/s7r/after_artifact.json"
    : > "$S2_TD/s7r/cohort_manifest.json"
    S7R_WT0="$(git -C "$REPO_ROOT" worktree list | wc -l)"
    set +e
    S7R_OUT="$(env -u PY_IMAGE REPLAY_DRY_RUN=1 REPLAY_IMAGE_ID="$S2_IMG" XDG_DATA_HOME="$S2_TD/xdg" I074_STAGE=2 \
      COUNTERFACTUAL_PATCH="$S7R_CF" TOOLING_PATCH="$S7R_TOOL" "$REPO_ROOT/scripts/run-replay-offline.sh" \
      --bundle "$S2_BUNDLE" --output-dir "$S2_TD/s7r/out" --before-ref "$S7R_BASE" \
      --after-artifact "$S2_TD/s7r/after_artifact.json" --cohort-manifest "$S2_TD/s7r/cohort_manifest.json" \
      --i074-counterfactual 2>"$S2_TD/s7r.err")"
    rc=$?
    set -e
    S7R_ACTUAL="$(sed -n '/^python$/,$p' <<< "$S7R_OUT" \
      | sed -e "s|^$S2_BUNDLE\$|<BUNDLE>|" -e "s|^$S2_TD/s7r/out\$|<OUT>|" \
            -e "s|^$S2_TD/s7r/after_artifact.json\$|<AFTER_ARTIFACT>|" \
            -e "s|^$S2_TD/s7r/cohort_manifest.json\$|<COHORT_MANIFEST>|" \
            -e "s|^sha256:[0-9a-f]\{64\}\$|<IMAGE_ID>|" -e "s|^[0-9a-f]\{40\}\$|<BASE_COMMIT>|" \
      | awk '
          prev=="--tooling-patch-sha256"{print "<TOOLING_PATCH_SHA256>"; prev=$0; next}
          prev=="--runner-sha256"{print "<RUNNER_SHA256>"; prev=$0; next}
          prev=="--counterfactual-patch-sha256"{print "<COUNTERFACTUAL_PATCH_SHA256>"; prev=$0; next}
          {print; prev=$0}')"
    S7R_EXPECTED="$(python3 -c 'import json,sys; print("\n".join(json.load(open(sys.argv[1],encoding="utf-8"))["stage2_counterfactual_argv"]))' "$S7R_FIXTURE")"
    if [ "$rc" -eq 0 ] && [ "$S7R_ACTUAL" = "$S7R_EXPECTED" ]; then
      pass "Stage 2 反事實 argv 與 fixture 逐 token 相同"
    else
      fail "Stage 2 反事實 argv 與 fixture 不符（rc=$rc）"; cat "$S2_TD/s7r.err" >&2
      diff <(printf '%s\n' "$S7R_EXPECTED") <(printf '%s\n' "$S7R_ACTUAL") >&2 || true
    fi
    S7R_CF_SHA="$(sha256sum < "$S7R_CF" | cut -d' ' -f1)"
    [ "$(awk 'prev=="--counterfactual-patch-sha256"{print; exit} {prev=$0}' <<< "$S7R_OUT")" = "$S7R_CF_SHA" ] \
      && [ "$(awk 'prev=="--before-ref"{print; exit} {prev=$0}' <<< "$S7R_OUT")" = "$S7R_BASE" ] \
      && [ "$(awk 'prev=="--base-commit"{print; exit} {prev=$0}' <<< "$S7R_OUT")" = "$S7R_BASE" ] \
      && pass "真正的兩份 patch：raw ＝ canonical、注入的反事實 SHA ＝ 封存 patch 的 SHA、before-ref ＝ base ＝ e1cbbbd" \
      || fail "真正的兩份 patch 的 SHA 或 base 不符"
    [ "$(git -C "$REPO_ROOT" worktree list | wc -l)" = "$S7R_WT0" ] \
      && pass "runner 的 worktree 已清掉（真正 repo 的登記數不變）" || fail "runner 在真正 repo 留下了 worktree"
  fi
elif [ "${IMAGE_REQUIRED:-0}" = "1" ]; then
  fail "Stage 2 反事實 argv 測試：找不到 image、正式 bundle 或 Stage 2 identity（IMAGE_REQUIRED=1）"
fi

rm -rf "$S2_TD"

# ── I-074 Stage 2 ⑦a：canonical diff 在惡意 git config／屬性下 bytes 不變（⑦ 總綱 v1「二」） ─────
#
# ⚠️ 每一層單獨、再全部同時；另有對照組證明這些設定**真的會**改變一般 `git diff` 的 bytes（⛔ 不是空測）。
echo "==> i074 Stage 2（⑦a）：canonical diff 在惡意 git config／屬性下 bytes 不變"
MC_TD="$(mktemp -d)"
MC_REPO="$MC_TD/repo"
git init -q "$MC_REPO"
(
  cd "$MC_REPO"
  printf 'def foo():\n    x = 1\n\n    y = 2\n    return x + y\n' > a.py          # funcname 行 ＋ 空白 context 行
  printf 'rename me please\nline2\nline3\nline4\n' > old_name.txt              # rename（⛔ 內容不變）
  printf 'first\n' > z_first.txt
  printf 'last\n' > a_last.txt                                                  # 多檔案（順序）
  mkdir 資料 && printf '中文\n' > 資料/檔案.txt                                  # 非 ASCII 檔名
  printf 'bin\000\001\002' > bin.dat                                            # binary
  git add -A && git -c user.name=t -c user.email=t@t -c commit.gpgsign=false commit -qm A
  printf 'def foo():\n    x = 1\n\n    y = 3\n    return x + y\n' > a.py
  git mv old_name.txt new_name.txt
  printf 'first changed\n' > z_first.txt
  printf 'last changed\n' > a_last.txt
  printf '中文改\n' > 資料/檔案.txt
  printf 'bin\000\003\004' > bin.dat
  git add -A && git -c user.name=t -c user.email=t@t -c commit.gpgsign=false commit -qm B
)
MC_A="$(git -C "$MC_REPO" rev-parse 'HEAD~1^{tree}')"
MC_B="$(git -C "$MC_REPO" rev-parse 'HEAD^{tree}')"
replay_args_canonical_diff "$MC_REPO" "$MC_A" "$MC_B" > "$MC_TD/ref.patch"
printf 'z_first.txt\n' > "$MC_TD/order"
printf '* diff=evil\n*.txt -diff\n*.dat diff\n' > "$MC_TD/evil_attrs"
mc_evil_config() {  # $1＝config 檔
  local kv
  for kv in diff.noprefix=true diff.context=9 diff.renames=copies "diff.orderFile=$MC_TD/order" \
            core.quotePath=false diff.mnemonicPrefix=true diff.algorithm=patience diff.indentHeuristic=false \
            diff.suppressBlankEmpty=true diff.interHunkContext=9 core.abbrev=12 color.diff=always \
            'diff.evil.xfuncname=^(.*)$'; do
    git config -f "$1" "${kv%%=*}" "${kv#*=}"
  done
}
mc_same() {  # 在目前的環境下算一次 canonical diff，bytes ＝ 參考值才回 0
  replay_args_canonical_diff "$MC_REPO" "$MC_A" "$MC_B" > "$MC_TD/out.patch" && cmp -s "$MC_TD/out.patch" "$MC_TD/ref.patch"
}
mc_check() {  # $1＝說明；$2＝0／1（上一步 mc_same 的結果）
  # ⚠️ pass／fail 一律在主 shell 呼叫——subshell 裡的 `fails` 計數傳不回來。
  if [ "$2" = 0 ]; then
    pass "惡意 config：$1 → bytes ＝ 乾淨環境的參考值"
  else
    fail "惡意 config：$1 改變了 canonical diff 的 bytes"
  fi
}
# ⚠️ 完整 index 行要看**文字檔**那一段：`--binary` 對 binary 檔本來就印完整 OID（反向驗證抓到的空測）。
grep -q '^diff --git a/new_name.txt b/new_name.txt$' "$MC_TD/ref.patch" && ! grep -q '^rename from' "$MC_TD/ref.patch" \
  && grep -q '^index [0-9a-f]\{40\}\.\.[0-9a-f]\{40\} ' \
       <<< "$(awk '/^diff --git a\/a.py b\/a.py$/{f=1; next} f && /^index /{print; exit}' "$MC_TD/ref.patch")" \
  && grep -q '^GIT binary patch$' "$MC_TD/ref.patch" \
  && grep -q '"a/\\350' "$MC_TD/ref.patch" \
  && pass "參考值：--no-renames、--full-index、binary、非 ASCII 以 quotePath 跳脫" || fail "參考值的形狀不符"
cp "$MC_REPO/.git/config" "$MC_TD/config.orig"
mc_evil_config "$MC_REPO/.git/config"
# 對照組：同一組惡意設定下，一般 `git diff` 的 bytes 確實不同。
git -C "$MC_REPO" diff --binary "$MC_A" "$MC_B" > "$MC_TD/plain.patch" 2>/dev/null || true
cmp -s "$MC_TD/plain.patch" "$MC_TD/ref.patch" && fail "對照組：惡意 config 竟沒有改變一般 git diff（測試是空的）" \
  || pass "對照組：惡意 config 確實改變一般 git diff 的 bytes"
rc=0; mc_same || rc=1; mc_check "來源 repo 的 .git/config" "$rc"
cp "$MC_TD/config.orig" "$MC_REPO/.git/config"
mkdir -p "$MC_TD/home"
mc_evil_config "$MC_TD/home/.gitconfig"
rc=0; ( export HOME="$MC_TD/home" XDG_CONFIG_HOME="$MC_TD/home"; mc_same ) || rc=1
mc_check "呼叫端 HOME 的 .gitconfig" "$rc"
mkdir -p "$MC_REPO/.git/info"
cp "$MC_TD/evil_attrs" "$MC_REPO/.git/info/attributes"
rc=0; mc_same || rc=1; mc_check "info/attributes" "$rc"
rm -f "$MC_REPO/.git/info/attributes"
cp "$MC_TD/evil_attrs" "$MC_REPO/.gitattributes"
rc=0; mc_same || rc=1; mc_check "工作樹的 .gitattributes" "$rc"
rm -f "$MC_REPO/.gitattributes"
git -C "$MC_REPO" config core.attributesFile "$MC_TD/evil_attrs"
rc=0; mc_same || rc=1; mc_check "core.attributesFile" "$rc"
cp "$MC_TD/config.orig" "$MC_REPO/.git/config"
rc=0
( export GIT_CONFIG_PARAMETERS="'diff.noprefix'='true' 'diff.context'='0' 'core.quotepath'='false'" \
         GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=diff.renames GIT_CONFIG_VALUE_0=copies
  mc_same ) || rc=1
mc_check "GIT_CONFIG_PARAMETERS／GIT_CONFIG_COUNT" "$rc"
mc_evil_config "$MC_REPO/.git/config"
git -C "$MC_REPO" config core.attributesFile "$MC_TD/evil_attrs"
cp "$MC_TD/evil_attrs" "$MC_REPO/.git/info/attributes"
cp "$MC_TD/evil_attrs" "$MC_REPO/.gitattributes"
rc=0
( export HOME="$MC_TD/home" XDG_CONFIG_HOME="$MC_TD/home" \
         GIT_CONFIG_PARAMETERS="'diff.noprefix'='true' 'diff.context'='0'" \
         GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=diff.renames GIT_CONFIG_VALUE_0=copies
  mc_same ) || rc=1
mc_check "全部同時" "$rc"
replay_args_canonical_diff "$MC_REPO" "$(git -C "$MC_REPO" rev-parse HEAD)" "$MC_B" >/dev/null 2>&1 \
  && fail "canonical diff 竟接受 commit OID" || pass "canonical diff ⛔ 接受 commit OID（只收 tree）"
replay_args_canonical_diff "$MC_REPO" HEAD "$MC_B" >/dev/null 2>&1 \
  && fail "canonical diff 竟接受 ref" || pass "canonical diff ⛔ 接受 ref（只收 40 碼 tree OID）"
rm -rf "$MC_TD"

# ── I-074 Stage 2 ⑦a：tooling patch 產生器（隔離 repo；複製腳本 ＋ sed 常數） ─────────────────
echo "==> i074 Stage 2（⑦a）：tooling patch 產生器的四條不變條件（隔離 repo）"
GEN_TD="$(mktemp -d)"
GEN="$GEN_TD/repo"
GEN_SR=python/backtest/modular/sr_scoring
mkdir -p "$GEN/scripts/lib" "$GEN/$GEN_SR/tests" "$GEN/$GEN_SR/replay_bundle" "$GEN/python/scripts"
cp "$REPO_ROOT/scripts/make-i074-tooling-patch.sh" "$GEN/scripts/"
cp "$REPO_ROOT/scripts/lib/replay-args.sh" "$GEN/scripts/lib/"
for _f in "${I074_CF_FILES[@]}"; do printf 'base\n' > "$GEN/$_f"; done
printf 'eval0\n' > "$GEN/$GEN_SR/evaluation.py"
printf 'rb0\n' > "$GEN/$GEN_SR/replay_bundle/a.py"
printf 'other0\n' > "$GEN/$GEN_SR/other.py"
printf 'x0\n' > "$GEN/python/scripts/x.py"
git -C "$GEN" init -q
git -C "$GEN" add -A
git -C "$GEN" -c user.name=t -c user.email=t@t -c commit.gpgsign=false commit -qm base
GEN_BASE="$(git -C "$GEN" rev-parse HEAD)"
GEN_BT="$(git -C "$GEN" rev-parse 'HEAD^{tree}')"
sed -i "s/^I074_TOOLING_BASE=[0-9a-f]\{40\}\$/I074_TOOLING_BASE=$GEN_BASE/" "$GEN/scripts/make-i074-tooling-patch.sh"
GEN_DIFF="$(diff "$REPO_ROOT/scripts/make-i074-tooling-patch.sh" "$GEN/scripts/make-i074-tooling-patch.sh" | grep -c '^[<>]' || true)"
[ "$GEN_DIFF" = 2 ] && pass "產生器的複本與正式檔案只差 base 常數那一行" || fail "產生器的複本與正式檔案差了 $GEN_DIFF 行"
gen_tree() {  # $1＝底的 tree；其餘＝「路徑=內容」或「路徑=@檔案」→ 印出新 tree（⚠️ 暫存 index，⛔ 不建 worktree）
  local idx="$GEN_TD/idx.$RANDOM$RANDOM" kv path val oid
  local base="$1"; shift
  GIT_INDEX_FILE="$idx" git -C "$GEN" read-tree "$base"
  for kv in "$@"; do
    path="${kv%%=*}"; val="${kv#*=}"
    if [ "${val#@}" != "$val" ]; then
      oid="$(git -C "$GEN" hash-object -w "${val#@}")"
    else
      oid="$(printf '%s\n' "$val" | git -C "$GEN" hash-object -w --stdin)"
    fi
    GIT_INDEX_FILE="$idx" git -C "$GEN" update-index --add --cacheinfo "100644,$oid,$path"
  done
  GIT_INDEX_FILE="$idx" git -C "$GEN" write-tree
  rm -f "$idx"
}
GEN_CFSPEC=(); for _f in "${I074_CF_FILES[@]}"; do GEN_CFSPEC+=("$_f=cf"); done
GEN_T1="$(gen_tree "$GEN_BT" "${GEN_CFSPEC[@]}")"
replay_args_canonical_diff "$GEN" "$GEN_BT" "$GEN_T1" > "$GEN_TD/cf.patch"
GEN_CFPATH=python/baselines/i074_stage2/counterfactual_e1cbbbd.patch
GEN_TOOLPATH=python/baselines/i074_stage2/tooling_e1cbbbd.patch
GEN_S1="$(gen_tree "$GEN_BT" "$GEN_CFPATH=@$GEN_TD/cf.patch" "$GEN_SR/evaluation.py=eval1" \
  "$GEN_SR/replay_bundle/a.py=rb1" "$GEN_SR/replay_bundle/new.py=new" "python/scripts/x.py=x1" \
  "$GEN_SR/tests/test_new.py=t1")"
GEN_MK="$GEN/scripts/make-i074-tooling-patch.sh"
if "$GEN_MK" --source-tree "$GEN_S1" > "$GEN_TD/tool.patch" 2> "$GEN_TD/tool.err"; then
  GEN_NAMES="$(grep '^diff --git' "$GEN_TD/tool.patch" | sed 's|^diff --git a/||;s| b/.*||' | LC_ALL=C sort | tr '\n' ' ')"
  [ "$GEN_NAMES" = "$GEN_SR/evaluation.py $GEN_SR/replay_bundle/a.py $GEN_SR/replay_bundle/new.py " ] \
    && pass "產生器：只含 evaluation.py 與 replay_bundle/（⛔ 不含測試、腳本與 baselines）" \
    || fail "產生器的路徑集合不符：$GEN_NAMES"
else
  fail "產生器的正向路徑失敗"; cat "$GEN_TD/tool.err" >&2
fi
GEN_S1V="$(gen_tree "$GEN_S1" "$GEN_TOOLPATH=@$GEN_TD/tool.patch")"
"$GEN_MK" --verify "$GEN_S1V" >/dev/null 2>&1 && pass "--verify：tree 內的 patch 與重新產生的逐位元相同 → 0" \
  || fail "--verify 對一致的 tree 竟不回 0"
: > "$GEN_TD/stale.patch"
GEN_S1S="$(gen_tree "$GEN_S1" "$GEN_TOOLPATH=@$GEN_TD/stale.patch")"
"$GEN_MK" --verify "$GEN_S1S" >/dev/null 2>&1 && fail "--verify 對漂移的 patch 竟回 0" || pass "--verify：patch 漂移 → 1"
"$GEN_MK" --verify "$GEN_S1" >/dev/null 2>&1 && fail "--verify 對缺 patch 的 tree 竟回 0" || pass "--verify：tree 裡沒有 patch → 1"
GEN_S4="$(gen_tree "$GEN_S1" "$GEN_SR/other.py=other1")"
set +e
"$GEN_MK" --source-tree "$GEN_S4" > "$GEN_TD/s4.out" 2> "$GEN_TD/s4.err"; rc=$?
set -e
[ "$rc" -ne 0 ] && [ ! -s "$GEN_TD/s4.out" ] && grep -q "④" "$GEN_TD/s4.err" \
  && pass "④ 產品碼不變條件：來源 tree 改到其他產品檔 → 中止、stdout ⛔ 無輸出" || fail "④ 沒有擋下（rc=$rc）"
"$GEN_MK" --source-tree "$GEN_BT" >/dev/null 2>&1 && fail "來源 tree 沒有 counterfactual 竟通過" \
  || pass "來源 tree 裡沒有 counterfactual patch → 中止"
"$GEN_MK" --source-tree HEAD >/dev/null 2>&1 && fail "產生器竟接受 ref" || pass "產生器 ⛔ 接受 ref（⛔ 不讀 HEAD）"
"$GEN_MK" --source-tree "$GEN_BASE" >/dev/null 2>&1 && fail "產生器竟接受 commit OID" || pass "產生器只接受 tree OID"
gen_inv() { ( . "$GEN_MK"; mk_check_invariants "$@" ) }
GEN_T2="$(gen_tree "$GEN_T1" "$GEN_SR/evaluation.py=eval1" "$GEN_SR/replay_bundle/a.py=rb1" "$GEN_SR/replay_bundle/new.py=new")"
gen_inv "$GEN_BT" "$GEN_S1" "$GEN_T1" "$GEN_T2" 2>/dev/null && pass "不變條件：合法的 T1／T2 → 通過（對照組）" \
  || fail "不變條件的對照組竟失敗"
GEN_T1BAD="$(gen_tree "$GEN_T1" "$GEN_SR/evaluation.py=evalX")"
gen_inv "$GEN_BT" "$GEN_S1" "$GEN_T1BAD" "$GEN_T2" 2> "$GEN_TD/i1.err" && fail "① 沒有擋下" \
  || { grep -q "①" "$GEN_TD/i1.err" && pass "①：counterfactual 改到 tooling 路徑 → 中止" || fail "① 的失敗點不符"; }
GEN_T2BAD="$(gen_tree "$GEN_T1" "$GEN_SR/evaluation.py=evalZ")"
gen_inv "$GEN_BT" "$GEN_S1" "$GEN_T1" "$GEN_T2BAD" 2> "$GEN_TD/i2.err" && fail "② 沒有擋下" \
  || { grep -q "②" "$GEN_TD/i2.err" && pass "②：T2 在 tooling 路徑上 ≠ 來源 → 中止" || fail "② 的失敗點不符"; }
GEN_T2X="$(gen_tree "$GEN_T2" "$GEN_SR/other.py=other1")"
gen_inv "$GEN_BT" "$GEN_S1" "$GEN_T1" "$GEN_T2X" 2> "$GEN_TD/i3.err" && fail "③ 沒有擋下" \
  || { grep -q "③" "$GEN_TD/i3.err" && pass "③：T1..T2 改到 tooling 以外 → 中止" || fail "③ 的失敗點不符"; }
gen_inv "$GEN_BT" "$GEN_S4" "$GEN_T1" "$GEN_T2" 2> "$GEN_TD/i4.err" && fail "④ 沒有擋下" \
  || { grep -q "④" "$GEN_TD/i4.err" && pass "④：來源 tree 的產品碼 ≠ base → 中止" || fail "④ 的失敗點不符"; }
# 自我驗證：讓產出在自我驗證之前被竄改（另一份複本注入一行）→ 必須中止、stdout ⛔ 無輸出。
# ⚠️ 用 Python 做字串替換（⛔ 不用 sed／grep 的 regex：這台的 grep 是 ugrep，`||` 的語意不同）。
python3 - "$GEN_MK" "$GEN/scripts/mk_tampered.sh" <<'TAMPER'
import sys
src, dst = sys.argv[1], sys.argv[2]
text = open(src, encoding="utf-8").read()
anchor = '  replay_args_canonical_diff "$REPO_ROOT" "$t1" "$t2" > "$out" || return 1\n'
assert text.count(anchor) == 1
open(dst, "w", encoding="utf-8").write(text.replace(anchor, anchor + '  printf "# tampered\\n" >> "$out"\n'))
TAMPER
chmod +x "$GEN/scripts/mk_tampered.sh"
set +e
"$GEN/scripts/mk_tampered.sh" --source-tree "$GEN_S1" > "$GEN_TD/tamper.out" 2> "$GEN_TD/tamper.err"; rc=$?
set -e
[ "$rc" -ne 0 ] && [ ! -s "$GEN_TD/tamper.out" ] && grep -q "自我驗證" "$GEN_TD/tamper.err" \
  && pass "自我驗證：產出寫出之後被改動 → 中止、stdout ⛔ 無輸出" || fail "自我驗證沒有擋下被竄改的產出（rc=$rc）"
rm -f "$GEN/scripts/mk_tampered.sh"
[ "$(git -C "$GEN" worktree list | wc -l)" = 1 ] && pass "產生器的暫時 worktree 全部清掉" \
  || { fail "產生器留下了 worktree"; git -C "$GEN" worktree list >&2; }

# ── I-074 Stage 2 ⑦a：乾淨檢查與 worktree 清理的錯誤處理（⑦a 實作第二輪 review：⛔ fail-open） ──────
echo "==> i074 Stage 2（⑦a）：replay_args_path_clean／replay_args_remove_worktrees_under 的錯誤處理"
FC_TD="$(mktemp -d)"
FC_REPO="$FC_TD/repo"
mkdir -p "$FC_REPO/bundle"
printf 'x\n' > "$FC_REPO/bundle/a.txt"
git -C "$FC_REPO" init -q
git -C "$FC_REPO" add -A
git -C "$FC_REPO" -c user.name=t -c user.email=t@t -c commit.gpgsign=false commit -qm base
REAL_GIT="$(command -v git)"
mkdir -p "$FC_TD/bin"
cat > "$FC_TD/bin/git" <<'FAKEGIT'
#!/usr/bin/env bash
# ⚠️ 只攔 `status`（FAKE_STATUS＝fail：失敗且沒有 stdout；dirty：成功但輸出一行），其餘交給真的 git。
for a in "$@"; do
  if [ "$a" = status ]; then
    case "${FAKE_STATUS:-}" in
      fail)  echo "fatal: simulated status failure" >&2; exit 128 ;;
      dirty) printf ' M python/baselines/x/manifest.json\n'; exit 0 ;;
    esac
  fi
done
exec "$REAL_GIT" "$@"
FAKEGIT
chmod +x "$FC_TD/bin/git"
fc_clean() {  # $1＝預期 rc；$2＝說明；其餘＝環境變數
  local want="$1" label="$2" rc=0
  shift 2
  env "$@" PATH="$FC_TD/bin:$PATH" REAL_GIT="$REAL_GIT" bash -c \
    '. "$1/scripts/lib/replay-args.sh"; replay_args_path_clean "$2" "$2/bundle"' _ "$REPO_ROOT" "$FC_REPO" \
    >/dev/null 2>&1 || rc=$?
  [ "$rc" = "$want" ] && pass "$label" || fail "$label（rc=$rc，預期 $want）"
}
fc_clean 0 "path_clean：乾淨 → 0"
fc_clean 2 "path_clean：git status 失敗（沒有 stdout）→ 2（⛔ 不當成乾淨）" FAKE_STATUS=fail
fc_clean 1 "path_clean：status 成功但有輸出 → 1" FAKE_STATUS=dirty
printf 'y\n' >> "$FC_REPO/bundle/a.txt"
fc_clean 1 "path_clean：已追蹤檔被改 → 1"
git -C "$FC_REPO" checkout -q -- bundle/a.txt
printf 'z\n' > "$FC_REPO/bundle/new.txt"
fc_clean 1 "path_clean：多一個 untracked 檔 → 1"
rm -f "$FC_REPO/bundle/new.txt"
fc_clean 0 "path_clean：還原之後 → 0（對照組）"

# smoke：來源檢查在 docker build 與任何 replay 之前——status 失敗或 dirty 都⛔ 不得呼叫 docker。
cat > "$FC_TD/bin/docker" <<'FAKEDOCKER'
#!/usr/bin/env bash
echo "$*" >> "$FAKE_DOCKER_LOG"
exit 1
FAKEDOCKER
chmod +x "$FC_TD/bin/docker"
for fc_mode in fail dirty; do
  : > "$FC_TD/docker_$fc_mode.log"
  set +e
  env FAKE_STATUS="$fc_mode" REAL_GIT="$REAL_GIT" FAKE_DOCKER_LOG="$FC_TD/docker_$fc_mode.log" \
    PATH="$FC_TD/bin:$PATH" "$REPO_ROOT/scripts/smoke-replay-offline.sh" > "$FC_TD/smoke_$fc_mode.out" 2>&1
  rc=$?
  set -e
  if [ "$rc" -ne 0 ] && [ ! -s "$FC_TD/docker_$fc_mode.log" ] && grep -q "不啟動任何 replay" "$FC_TD/smoke_$fc_mode.out"; then
    pass "smoke：來源的 git status $([ "$fc_mode" = fail ] && echo 失敗 || echo 不為空) → 中止、docker ⛔ 一次都沒被呼叫"
  else
    fail "smoke：來源的 git status $fc_mode 沒有在 replay 之前擋下（rc=$rc）"; cat "$FC_TD/smoke_$fc_mode.out" >&2
  fi
done

# 清理函式：list 失敗 → 非零；remove 失敗 → 非零、⛔ 不刪實體目錄、登記仍在。
mkdir -p "$FC_TD/scope"
set +e
replay_args_remove_worktrees_under "$FC_TD/not-a-repo" "$FC_TD/scope" >/dev/null 2>&1; rc=$?
set -e
[ "$rc" -ne 0 ] && pass "remove_worktrees_under：worktree list 失敗 → 非零（⛔ 不當成沒有東西要清）" \
  || fail "worktree list 失敗竟回 0"
git -C "$FC_REPO" worktree add -q --detach "$FC_TD/scope/locked" HEAD
git -C "$FC_REPO" worktree lock "$FC_TD/scope/locked"
set +e
replay_args_remove_worktrees_under "$FC_REPO" "$FC_TD/scope" >/dev/null 2>&1; rc=$?
set -e
if [ "$rc" -ne 0 ] && [ -d "$FC_TD/scope/locked" ] \
   && grep -qxF -- "worktree $FC_TD/scope/locked" <<< "$(git -C "$FC_REPO" worktree list --porcelain)"; then
  pass "remove_worktrees_under：worktree remove 失敗 → 非零、⛔ 不刪實體目錄、登記仍在"
else
  fail "worktree remove 失敗時的處理不符（rc=$rc）"
fi
git -C "$FC_REPO" worktree unlock "$FC_TD/scope/locked"
replay_args_remove_worktrees_under "$FC_REPO" "$FC_TD/scope" && [ ! -e "$FC_TD/scope/locked" ] \
  && pass "remove_worktrees_under：解鎖之後 → 0 且已移除（對照組）" || fail "解鎖之後仍移除不了"
rm -rf "$FC_TD"

# ⚠️ **漂移測試**（⑦ 總綱 v1「二」的版控列）：對「目前 index 的 tree」重新產生，必須與 index 中的 patch 逐位元相同。
# 開發途中（index 還沒有新 patch）會失敗是預期行為——依版控流程 stage 之後才會通過。
echo "==> i074 Stage 2（⑦a）：tooling patch 漂移測試（真正 repo 的 index）"
DRIFT_WT0="$(git -C "$REPO_ROOT" worktree list | wc -l)"
if DRIFT_TREE="$(git -C "$REPO_ROOT" write-tree)" \
   && "$REPO_ROOT/scripts/make-i074-tooling-patch.sh" --verify "$DRIFT_TREE" > "$GEN_TD/drift.out" 2> "$GEN_TD/drift.err"; then
  pass "tooling patch：index 的 tree 重新產生的結果 ＝ index 中的 patch（沒有漂移）"
else
  fail "tooling patch 漂移了（或 index 還沒有它）——依版控流程重新產生並 stage"; cat "$GEN_TD/drift.err" >&2
fi
[ "$(git -C "$REPO_ROOT" worktree list | wc -l)" = "$DRIFT_WT0" ] \
  && pass "漂移測試沒有在真正 repo 留下 worktree" || fail "漂移測試在真正 repo 留下了 worktree"
rm -rf "$GEN_TD"


# ⚠️ **結尾要再檢查一次**：`$fails` 的第一次檢查在上面的 I-100 段落結束處，
# ⛔ 之後新增的 I-074 測試若呼叫 `fail`，沒有這一段就會照樣印「全部通過」並回 0，
# 連帶讓 `python/scripts/test.sh` 誤報成功。
if [ "$fails" -ne 0 ]; then
  echo "==> replay-args 測試失敗：$fails 項" >&2
  exit 1
fi
echo "==> replay-args 測試全部通過"
