#!/usr/bin/env bash
# I-100：**真的**用官方腳本把 Stage 1／2 跑完一次（小型 fixture，秒級）。
#
# 為什麼需要它：`scripts/test-replay-args.sh` 只斷言「組出來的 docker argv 對不對」，
# 那攔得住掛載與版本搞反，但證明不了：
#   ① worktree 內的 Python CLI 真的啟動得起來；
#   ② bundle 與 Stage 1 的 artifact 在容器內真的載入得了；
#   ③ Stage 1／2 在 `--network none`、無 DB 環境變數下真的跑得完並產出 artifact。
#
# 用法：scripts/smoke-replay-offline.sh
#   （需要 docker 與 git；不需要 DB、不需要正式 bundle、不需要數小時資料）
#
# ⚠️ **不在常態測試回合裡**：它要 docker build ＋ 兩次容器啟動 ＋ 兩個 git worktree。
# `python/scripts/test.sh` 只有在 `REPLAY_SMOKE=1` 時才會呼叫它。
#
# ⚠️ **tooling patch 是這支腳本的關鍵**，而且它同時驗到兩件事：
#   * 目前**工作樹**（可能尚未 commit）要能經由 patch 進到 worktree——離線腳本本來就是
#     這樣支援「還沒進版控的 tooling 變更」的；
#   * Stage 1 需要 `rr_decoupling_candidate`，而那是 issue.md I-074 Stage 0 才會補的欄位。
#     這裡用一段**明確標示為 smoke 專用**的 patch 補上它，讓 Stage 1／2 走得完全程。
#     ⛔ 這一段永遠不會進版控的產品程式碼——它只存在於本腳本產生的暫時 patch 裡。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK="$(mktemp -d)"
SCRATCH="$WORK/scratch"
BUNDLE_ROOT="$WORK/baselines"
STAGE1_OUT="$WORK/stage1"
STAGE2_OUT="$WORK/stage2"
IMAGE="${PY_IMAGE:-stock-trading-python-test:latest}"

cleanup() {
  git -C "$REPO_ROOT" worktree remove --force "$SCRATCH" >/dev/null 2>&1 || true
  git -C "$REPO_ROOT" worktree remove --force "$WORK/verify" >/dev/null 2>&1 || true
  rm -rf "$WORK"
}
trap cleanup EXIT

fails=0
pass() { echo "  ok   $1"; }
fail() { echo "  FAIL $1" >&2; fails=$((fails + 1)); }

echo "==> 建置 image：$IMAGE"
docker build -q -t "$IMAGE" "$REPO_ROOT/python" >/dev/null

# ── ① 產一份小型 bundle（用目前工作樹的程式碼，⛔ 不碰 DB） ────────────────
echo "==> 產生小型 bundle（不讀 DB）"
mkdir -p "$BUNDLE_ROOT"
docker run --rm --network none \
  --user "$(id -u):$(id -g)" \
  -e HOME=/tmp -e PYTHONDONTWRITEBYTECODE=1 \
  -v "$REPO_ROOT/python":/app \
  -v "$BUNDLE_ROOT":/out \
  -w /app "$IMAGE" python scripts/make_smoke_bundle.py /out > "$WORK/bundle_id"
BUNDLE_ID="$(tail -1 "$WORK/bundle_id")"
if [ -d "$BUNDLE_ROOT/$BUNDLE_ID" ]; then
  pass "bundle 產生成功：$BUNDLE_ID"
else
  fail "bundle 沒有產出來"
  exit 1
fi

# ── ② 把「目前工作樹 ＋ smoke 專用欄位」做成 tooling patch ────────────────
echo "==> 組 tooling patch（目前工作樹 ＋ smoke 專用的 rr_decoupling_candidate）"
git -C "$REPO_ROOT" worktree add --detach "$SCRATCH" HEAD >/dev/null 2>&1
# ⚠️ **先整個刪掉再複製**，⛔ 不要用 tar 疊上去：疊加沒有刪除語意，工作樹刪掉的檔案在
# scratch 裡會保留 HEAD 的舊版，patch 就表達不出那次刪除——smoke 會拿一份**現實中不存在
# 的程式碼組合**跑出綠燈（false pass）。
rm -rf "$SCRATCH/python"
mkdir -p "$SCRATCH/python"
# ⚠️ 排除清單要與下方 verify_patch.py 的過濾**保持一致**：這些是 gitignore 的產物
# （host 上跑過 pytest 就會有），`git add -A -N` 本來就不會收它們，patch 也帶不走。
tar -C "$REPO_ROOT/python" \
  --exclude=__pycache__ --exclude=logs --exclude=.pytest_cache -cf - . \
  | tar -C "$SCRATCH/python" -xf -
python3 - "$SCRATCH" <<'PY'
import sys
from pathlib import Path

# ⛔ smoke 專用：模擬 I-074 Stage 0 會補上的診斷欄位，讓 Stage 1／2 走得完全程。
# 這段只存在於暫時 worktree 裡，⛔ 不會進版控。
path = Path(sys.argv[1]) / "python/backtest/modular/sr_scoring/evaluation.py"
text = path.read_text(encoding="utf-8")
needle = '                "expired_event_count": expired_event_count,\n            })'
assert needle in text, "smoke patch 的錨點不見了——evaluation.py 的 row 結構改過了"
text = text.replace(needle, needle.replace(
    '            })',
    '                # [smoke-only] I-074 Stage 0 的診斷欄位替身。\n'
    '                # ⚠️ 用**決定性的子集**而不是真的 predicate：真的 predicate 在這份\n'
    '                # 合成資料上命中 0 列，cohort 與 comparison 就都是空的，\n'
    '                # Stage 2 的比較路徑等於沒被走到。\n'
    '                "rr_decoupling_candidate": bool(idx % 7 == 0),\n'
    '            })'), 1)
path.write_text(text, encoding="utf-8")
PY
git -C "$SCRATCH" add -A -N >/dev/null
git -C "$SCRATCH" diff --binary HEAD > "$WORK/tooling.patch"

# ── 2-B：patch 必須忠實表達 scratch 的內容（含**刪除**） ──────────────────
# ⚠️ 這一條是 false pass 的守門：把 patch 套到一份乾淨的 HEAD worktree 上，結果必須與
# scratch 逐檔一致。少了它，「patch 表達不出刪除」這類錯誤會安靜地讓 smoke 跑在一份
# 現實中不存在的程式碼組合上，然後給出綠燈。
cat > "$WORK/verify_patch.py" <<'VERIFYPY'
"""比對兩個 worktree 的 python/ 內容是否逐檔一致。

⚠️ **檔案清單交給 git 決定**（`ls-files --cached --others --exclude-standard`）：
那正好是「patch 帶得走的東西」——gitignore 的產物（`__pycache__`、`.pyc`、
`.pytest_cache`、log）本來就不會進 patch，自己維護排除清單只會追不完。
"""
import hashlib
import subprocess
import sys
from pathlib import Path


def listing(root):
    out = subprocess.run(
        ["git", "-C", root, "ls-files", "--cached", "--others", "--exclude-standard", "-z",
         "python"],
        capture_output=True, text=True, check=True).stdout
    files = [f for f in out.split("\0") if f]
    tree = {}
    for rel in files:
        path = Path(root) / rel
        if path.is_file():
            tree[rel] = hashlib.sha256(path.read_bytes()).hexdigest()
    return tree


expected, actual = listing(sys.argv[1]), listing(sys.argv[2])
if expected != actual:
    missing = sorted(set(expected) - set(actual))[:5]
    extra = sorted(set(actual) - set(expected))[:5]
    changed = sorted(k for k in set(expected) & set(actual) if expected[k] != actual[k])[:5]
    print(f"    patch 沒表達到：缺 {missing}、多 {extra}、內容不同 {changed}", file=sys.stderr)
    sys.exit(1)
print(f"    patch 忠實還原 {len(expected)} 個檔案（含刪除語意）")
VERIFYPY

VERIFY="$WORK/verify"
git -C "$REPO_ROOT" worktree add --detach "$VERIFY" HEAD >/dev/null 2>&1
git -C "$VERIFY" apply --index "$WORK/tooling.patch"
if python3 "$WORK/verify_patch.py" "$SCRATCH" "$VERIFY"; then
  pass "tooling patch 忠實表達工作樹（含刪除）"
else
  fail "tooling patch 沒有忠實表達工作樹——smoke 會跑在不存在的程式碼組合上"
fi
git -C "$REPO_ROOT" worktree remove --force "$VERIFY" >/dev/null 2>&1 || true
git -C "$REPO_ROOT" worktree remove --force "$SCRATCH" >/dev/null 2>&1

# ── ③ Stage 1（after 版本） ───────────────────────────────────────────────
echo "==> Stage 1（after）"
if TOOLING_PATCH="$WORK/tooling.patch" AFTER_REF=HEAD \
   "$REPO_ROOT/scripts/run-replay-offline.sh" \
     --bundle "$BUNDLE_ROOT/$BUNDLE_ID" --output-dir "$STAGE1_OUT" \
     --before-ref HEAD > "$WORK/stage1.log" 2>&1; then
  pass "Stage 1 在 --network none 下跑完"
else
  fail "Stage 1 失敗"
  tail -30 "$WORK/stage1.log" >&2
fi
for f in after_artifact.json cohort_manifest.json; do
  if [ -s "$STAGE1_OUT/$f" ]; then
    pass "Stage 1 產出 $f"
  else
    fail "Stage 1 沒有產出 $f"
  fi
done

# ── ④ Stage 2（before 版本） ──────────────────────────────────────────────
echo "==> Stage 2（before）"
if TOOLING_PATCH="$WORK/tooling.patch" AFTER_REF=HEAD \
   "$REPO_ROOT/scripts/run-replay-offline.sh" \
     --bundle "$BUNDLE_ROOT/$BUNDLE_ID" --output-dir "$STAGE2_OUT" \
     --before-ref HEAD \
     --after-artifact "$STAGE1_OUT/after_artifact.json" \
     --cohort-manifest "$STAGE1_OUT/cohort_manifest.json" > "$WORK/stage2.log" 2>&1; then
  pass "Stage 2 在 --network none 下跑完"
else
  fail "Stage 2 失敗"
  tail -30 "$WORK/stage2.log" >&2
fi
for f in comparison_artifact.json report.json; do
  if [ -s "$STAGE2_OUT/$f" ]; then
    pass "Stage 2 產出 $f"
  else
    fail "Stage 2 沒有產出 $f"
  fi
done

# ── ⑤ 內容抽驗 ───────────────────────────────────────────────────────────
if [ -s "$STAGE2_OUT/report.json" ]; then
  python3 - "$STAGE1_OUT" "$STAGE2_OUT" "$BUNDLE_ID" <<'PY' && pass "artifact 內容自洽" || fail "artifact 內容不自洽"
import hashlib, json, sys
stage1, stage2, bundle_id = sys.argv[1], sys.argv[2], sys.argv[3]
after_raw = open(f"{stage1}/after_artifact.json", "rb").read()
after = json.loads(after_raw)
cohort = json.load(open(f"{stage1}/cohort_manifest.json", encoding="utf-8"))
comparison = json.load(open(f"{stage2}/comparison_artifact.json", encoding="utf-8"))
report = json.load(open(f"{stage2}/report.json", encoding="utf-8"))
assert after["bundle_id"] == bundle_id, after["bundle_id"]
assert cohort["after_artifact_sha256"] == hashlib.sha256(after_raw).hexdigest()
assert after["rows"], "after artifact 沒有任何列"
assert all("rr_decoupling_candidate" in r for r in after["rows"])
assert cohort["keys"], "cohort 是空的——Stage 2 的比較路徑沒被走到"
assert len(comparison["rows"]) == len(cohort["keys"])
assert report["candidate_rows"] == len(cohort["keys"])
assert all(set(r) >= {"symbol", "timeframe", "as_of", "differences", "before", "after"}
           for r in comparison["rows"])
assert report["comparison_artifact_sha256"]
# provenance 要記得住執行身分（review #4）
prov = after["provenance"]
assert prov["runner_sha256"] and prov["image_digest"] and prov["base_commit"]
assert prov["project_modules_sha256"], "provenance 沒有記到任何專案模組"
print(f"    after rows={len(after['rows'])} cohort={len(cohort['keys'])} "
      f"comparison={len(comparison['rows'])}")
PY
fi

if [ "$fails" -ne 0 ]; then
  echo "==> smoke 失敗：$fails 項" >&2
  exit 1
fi
echo "==> smoke 全部通過"
