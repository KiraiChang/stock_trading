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

if [ "$fails" -ne 0 ]; then
  echo "==> replay-args 測試失敗：$fails 項" >&2
  exit 1
fi
echo "==> replay-args 測試全部通過"
