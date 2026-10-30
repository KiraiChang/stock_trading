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
# ——真正的原因卻因為 `2>&1 >/dev/null` 完全看不到。實例見 git show 20eacb2:docs/issue.md 的 I-074「✅ 根因已定位並修正（2026-09-18）」。
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

# ── I-074 Stage 2：下面各段共用的暫存目錄、image 與 run identity ──
#
# T-077 刪除 finalizer（`finalize-stage2-evidence.sh`）與它的測試之後，原本定義在那一段、後面各段也會用到的
# 變數搬到這裡（finalizer 測試的最後版本見 c6c2481 的本檔）。`S2_TD` 由「Stage 2 反事實 argv」那一段結束時刪除。
echo "==> i074 Stage 2：共用的暫存目錄與 run identity"
S2_TD="$(mktemp -d)"
S2_IMG="$(docker image inspect "${PY_IMAGE:-stock-trading-python-test:latest}" -f '{{.Id}}' 2>/dev/null || true)"
S2_BUNDLE="$REPO_ROOT/python/baselines/b1_20260901_1d_74350966_5d7ecb10"
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
elif [ "${IMAGE_REQUIRED:-0}" = "1" ]; then
  fail "Stage 2 的共用 run identity：找不到 image 或正式 bundle（IMAGE_REQUIRED=1）"
else
  echo "  skip 找不到 image 或正式 bundle，略過 Stage 2 的共用 run identity" >&2
fi

# ── I-074 Stage 2（⑦a）：runner 反事實模式的隔離 repo、patch fixture 與語意鍵 ──
#
# T-077 刪除 finalizer 的測試時，原本夾在那一段裡、測**保留的** `run-replay-offline.sh` 與 `lib/replay-args.sh` 的項目
# 連同它們的建置原樣搬到這裡（v4）。隔離 repo ⛔ 不再複製被刪的 finalizer 與 `i074-stage2-patch-claims.py`。
S2_REPO="$S2_TD/repo"
mkdir -p "$S2_REPO/scripts/lib" "$S2_REPO/python/scripts" \
         "$S2_REPO/python/backtest/modular/sr_scoring" "$S2_REPO/python/baselines/i074_stage1"
cp "$REPO_ROOT/scripts/run-replay-offline.sh" "$S2_REPO/scripts/"
cp "$REPO_ROOT"/scripts/lib/*.sh "$S2_REPO/scripts/lib/"
cp "$REPO_ROOT/python/scripts/validate-i074-run-identity.py" "$REPO_ROOT/python/scripts/_i074_bootstrap.py" \
   "$S2_REPO/python/scripts/"
cp -r "$REPO_ROOT/python/backtest/modular/sr_scoring/replay_bundle" "$S2_REPO/python/backtest/modular/sr_scoring/"
find "$S2_REPO" -name __pycache__ -prune -exec rm -rf {} +
: > "$S2_REPO/python/baselines/i074_stage1/.keep"
# 反事實 patch 的合成要一個真的 base。⚠️ ⑦a 起反事實 patch 改動的檔案集合**恰好**是固定的四個檔
# （`I074_CF_FILES`，兩個產品檔 ＋ 兩個測試檔），所以 base 放這四個檔的佔位內容；tooling 另外新增檔案。
mkdir -p "$S2_REPO/python/backtest/modular/sr_scoring/tests" "$S2_REPO/docs"
for _f in "${I074_CF_FILES[@]}"; do printf 'base\n' > "$S2_REPO/$_f"; done
printf 'docs\n' > "$S2_REPO/docs/notes.md"
git -C "$S2_REPO" init -q
git -C "$S2_REPO" add -A
git -C "$S2_REPO" -c user.name=t -c user.email=t@t -c commit.gpgsign=false commit -qm fixture

if [ -n "$S2_IMG" ] && [ -d "$S2_BUNDLE" ]; then
  echo "==> i074 Stage 2（⑦a）：runner 反事實模式的 patch fixture 與語意鍵（隔離 repo）"
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
  S2_P="$S2_TD/p_main"; s2_make_patches "$S2_P" b b ""           # 反事實 ＋ 空 tooling
  S2_PT="$S2_TD/p_tool"; s2_make_patches "$S2_PT" b b x          # 同一份反事實 ＋ 非空 tooling
  S2_P2="$S2_TD/p_other"; s2_make_patches "$S2_P2" c c ""        # 產品檔也不同的另一份反事實（語意 SHA 不同）
  S2_PTEST="$S2_TD/p_tests"; s2_make_patches "$S2_PTEST" b z ""  # ⚠️ 只有測試檔不同：完整 SHA 不同、語意 SHA 相同
  S2_PEXTRA="$S2_TD/p_extra"; s2_make_patches "$S2_PEXTRA" b b "" docs/notes.md   # 多一個非白名單檔
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

  s2_after_flag() {  # $1＝檔案；$2＝旗標 → 印出它後面那個 token
    awk -v f="$2" 'prev==f{print; exit} {prev=$0}' "$1"
  }

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
  # ⚠️ ⑧ 補齊：看訊息。擋下它的是 run identity 的驗證（identity 記的 image ≠ 空的 REPLAY_IMAGE_ID）——那一步**只在 I074_MODE**
  #   執行、而且在自動 pin 的分支之前就結束，所以這個訊息同時證明 flag 納入了 I074_MODE、也⛔ 沒有走到自動 pin。
  S7_IMG="" s7_run s7_x "$S2_P/counterfactual.patch" "" "${S7_ARGS[@]}" --i074-counterfactual \
    && fail "x：缺 REPLAY_IMAGE_ID 竟通過（⛔ 不得自動 pin）" || \
    s7_blocked "x：flag 納入 I074_MODE → 缺 REPLAY_IMAGE_ID 即拒絕、⛔ 不自動 pin" s7_x "三趟必須用同一個 image"
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
  fail "runner 反事實模式的測試：找不到 image 或正式 bundle（IMAGE_REQUIRED=1）"
else
  echo "  skip 找不到 image 或正式 bundle，略過 runner 反事實模式的測試" >&2
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

# ── I-074 Stage 2：封存證據的固定值（T-077；取代漂移測試順帶提供的「patch 的 bytes 沒有變」） ──
#
# 只驗三個檔的 SHA-256 等於封存值（與 docs/sr-zone-scoring.md 的紀錄相同）；⛔ 不讀產品碼。
# ⚠️ ⛔ 不驗 manifest 列出的 artifact 的實際內容，也⛔ 不驗 manifest 自己的 SHA 鏈——那是 B／C 判讀器
# （scripts/judge-i074-stage2.sh）驗證模式的工作。這三個檔是 I-074 的證據：⛔ 不改、⛔ 不重新產生。
echo "==> i074 Stage 2：封存證據的固定值（兩份 patch 與 evidence manifest）"
while read -r sealed_sha sealed_rel; do
  sealed_got="$({ sha256sum < "$REPO_ROOT/$sealed_rel"; } 2>/dev/null | cut -d' ' -f1 || true)"
  if [ "$sealed_got" = "$sealed_sha" ]; then
    pass "封存值相同：$sealed_rel"
  else
    fail "封存證據被改動或不存在：$sealed_rel（預期 $sealed_sha、實際 '$sealed_got'）"
  fi
done <<'SEALED'
fa7f5ba840a3106978ad1ea1661eae7c97cb77710db2a7e042bb6f1b3a6fea14 python/baselines/i074_stage2/counterfactual_e1cbbbd.patch
5ff9a90fe33113ee0dcece3d121910daeadc3a3d5eeb6135f4b7939eeccf0b98 python/baselines/i074_stage2/tooling_e1cbbbd.patch
d5cf06298b4de0875bb00614352e5cf8b6b98450d0b72d6dec8e63458cda5e0e python/baselines/i074_stage2/evidence/evidence_manifest.json
SEALED

# ── pin --stage 2：專用 tag、tarball、Stage 1 行為不變 ─────────────────────
# ── v29 ag（⑧ 補齊；第一、二輪 review 改寫）：pin 的專用 tag ⛔ 不等於任何 build 腳本的預設 tag——`python/scripts/test.sh`
#    等腳本只 build 自己的預設 tag（或 PY_IMAGE），而 `pin --stage 2` ⛔ 不接受 PY_IMAGE（下方），所以 test.sh 跑完之後釘住的 image
#    仍帶專用 tag。掃描 scripts/、scripts/lib/、python/scripts/ 的全部 .sh，**只**排除會自我比對的本腳本（它的文字會被抽成假值）；
#    第二輪 review：⛔ 不以名稱整批排除 test-*.sh——否則 test-foo.sh 寫死專用 tag 會 fail-open。抽三種寫法：非空的
#    `${PY_IMAGE:-<tag>}`、字面值的 `IMAGE="<tag>"`、字面值的 `build … -t <tag>`；抽到的每一個值都必須像 image reference，
#    專用 tag 只准出現在 pin 與 restore 兩支。⚠️ 靜態檢查，⛔ 不放在需要正式 bundle 的段落裡。
echo "==> i074 Stage 2（⑧）：pin 的專用 tag 與 build 腳本的預設 tag（ag）"
ag_scan() {  # $1＝repo root → 印出 FOUND 一行與每個 PROBLEM 一行
  local root="$1" dedicated f
  local -a files=()
  dedicated="$(sed -n 's/^[[:space:]]*IMAGE="\(stock-trading-python-replay:[^"]*\)"$/\1/p' "$root/scripts/pin-replay-image.sh")"
  for f in "$root"/scripts/*.sh "$root"/scripts/lib/*.sh "$root"/python/scripts/*.sh; do
    [ "$f" = "$root/scripts/test-replay-args.sh" ] || files+=("$f")
  done
  python3 - "$root" "$dedicated" "${files[@]}" <<'PY' || echo "PROBLEM 抽取程式失敗"
import re, sys
root, dedicated, files = sys.argv[1], sys.argv[2], sys.argv[3:]
pats = [re.compile(r'\$\{PY_IMAGE:-([^}]+)\}'), re.compile(r'^\s*(?:local\s+)?IMAGE="([^"$]*)"', re.M),
        re.compile(r'\bbuild\b[^\n]*?\s-t\s+"?([^"\s$]+)"?')]
ref = re.compile(r'[a-z0-9][a-z0-9._/-]*(?::[A-Za-z0-9._-]+)?')
allowed = {"scripts/pin-replay-image.sh", "scripts/restore-replay-image.sh"}
found, problems = [], []
if dedicated != "stock-trading-python-replay:i074-stage2":
    problems.append(f"pin-replay-image.sh 的專用 tag 抽不到或不符：{dedicated!r}")
for f in files:
    rel = f[len(root) + 1:]
    text = open(f, encoding="utf-8").read()
    for pat in pats:
        for m in pat.finditer(text):
            tag = m.group(1)
            found.append((rel, tag))
            if not ref.fullmatch(tag):
                problems.append(f"不像 image reference：{rel}：{tag!r}")
            elif tag == dedicated and rel not in allowed:
                problems.append(f"專用 tag 出現在 build 腳本：{rel}")
if not any(rel == "python/scripts/test.sh" for rel, _ in found):
    problems.append("python/scripts/test.sh 抽不到預設 tag")
print("FOUND " + " ".join(sorted({f"{rel}={tag}" for rel, tag in found})))
for problem in problems:
    print("PROBLEM " + problem)
PY
}
AG_OUT="$(ag_scan "$REPO_ROOT")"
if ! grep -q '^PROBLEM ' <<< "$AG_OUT" \
   && grep -qF 'scripts/pin-replay-image.sh=stock-trading-python-replay:i074-stage2' <<< "$AG_OUT"; then
  pass "ag：專用 tag 只在 pin／restore，⛔ 不等於任何 build 腳本的預設 tag"
else
  fail "ag：專用 tag 與 build 腳本的預設 tag 撞名、抽到假值或抽不到"; printf '%s\n' "$AG_OUT" >&2
fi
# 反向案例（第二輪 review）：複本裡多一支 test-*.sh 以字面值 build 專用 tag → 必須被抓到（⛔ 不因名稱被略過）
AG_NEG="$S2_TD/ag_neg"
mkdir -p "$AG_NEG/scripts/lib" "$AG_NEG/python/scripts"
cp "$REPO_ROOT"/scripts/*.sh "$AG_NEG/scripts/"
cp "$REPO_ROOT"/scripts/lib/*.sh "$AG_NEG/scripts/lib/"
cp "$REPO_ROOT"/python/scripts/*.sh "$AG_NEG/python/scripts/"
printf '#!/usr/bin/env bash\ndocker build -q -t stock-trading-python-replay:i074-stage2 "$1"\n' > "$AG_NEG/scripts/test-foo.sh"
AG_NEG_OUT="$(ag_scan "$AG_NEG")"
if grep -qxF "PROBLEM 專用 tag 出現在 build 腳本：scripts/test-foo.sh" <<< "$AG_NEG_OUT"; then
  pass "ag（反向案例）：另一支 test-*.sh 寫死專用 tag → 被抓到"
else
  fail "ag（反向案例）：test-*.sh 寫死專用 tag 竟沒被抓到"; printf '%s\n' "$AG_NEG_OUT" >&2
fi
rm -rf -- "$AG_NEG"

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
    fail "找不到已封存的 $S7R_TOOL——它是 I-074 的證據，⛔ 不重新產生（產生器已於 T-077 刪除）"
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

# （smoke 的「來源 bundle 必須乾淨」前置檢查只為反事實段存在，已隨該段於 T-077 移除；它的兩項測試一併刪除，最後版本見 c6c2481。）

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

# ⚠️ **結尾要再檢查一次**：`$fails` 的第一次檢查在上面的 I-100 段落結束處，
# ⛔ 之後新增的 I-074 測試若呼叫 `fail`，沒有這一段就會照樣印「全部通過」並回 0，
# 連帶讓 `python/scripts/test.sh` 誤報成功。
if [ "$fails" -ne 0 ]; then
  echo "==> replay-args 測試失敗：$fails 項" >&2
  exit 1
fi
echo "==> replay-args 測試全部通過"
