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
offline="$(replay_args_offline "sha256:x" "/app" "abc" "def" "ghi" --bundle /b --output-dir /o --before-ref main)"
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
# ⛔ **⛔ 不能用「在 worktree 裡放一堆檔案」來測**：`replay_args_tooling_patch_sha256()`
# 會先跑 `git add -A -N`，把 untracked **全部吸收成 intent-to-add**，`??` 根本不會出現
# ——那樣測到的是「沒有 untracked」，⛔ 不是守門本身。所以這裡直接驗**比對模式**：
# 大量輸出下仍要找得到 `??`，且結束碼正確。
# ⚠️ **要讓 helper 本身收到大量 `??`**（⛔ 不是只測旁邊的 grep）：
# 用 PATH 注入假 `git`，讓 `status --porcelain` 吐 5000 行 untracked，
# 其餘子命令（`add`／`diff`／`rev-parse`）一律成功且無輸出。
FAKE_GIT_DIR="$TMP_REPO/../fakegit-$$"
mkdir -p "$FAKE_GIT_DIR"
cat > "$FAKE_GIT_DIR/git" <<'FAKEGIT'
#!/usr/bin/env bash
# ⚠️ 只為這條測試存在：讓 `status --porcelain` 產生大量 ?? 行。
for a in "$@"; do
  if [ "$a" = "status" ]; then
    for i in $(seq 1 5000); do printf '?? many/file_%s.txt\n' "$i"; done
    exit 0
  fi
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

# ⚠️ **結尾要再檢查一次**：`$fails` 的第一次檢查在上面的 I-100 段落結束處，
# ⛔ 之後新增的 I-074 測試若呼叫 `fail`，沒有這一段就會照樣印「全部通過」並回 0，
# 連帶讓 `python/scripts/test.sh` 誤報成功。
if [ "$fails" -ne 0 ]; then
  echo "==> replay-args 測試失敗：$fails 項" >&2
  exit 1
fi
echo "==> replay-args 測試全部通過"
