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
#   * ⚠️ `rr_decoupling_candidate` 現在由**產品端**產出（I-074 Stage 0 已實作），
#     ⛔ 本腳本不再注入假值——注入會覆蓋真欄位而造成 false pass。
#
# ⚠️ **I-074 Stage 2 的反事實路徑**（⑦a 細部計畫 B7，最後一段）：合成 bundle 的單調 K 棒⛔ 不會產生
# candidate，所以另外從已進版控的正式 bundle 切出 6243 最後 250 根（`make_counterfactual_smoke_bundle.py`），
# 在 `e1cbbbd` ＋ 真正的 counterfactual ＋ 工作樹現行的 tooling patch 上跑一次真的反事實 Stage 2，
# 斷言「after 有候選 → before 消掉 → comparison 非空且逐列翻轉」。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=lib/replay-args.sh
. "$REPO_ROOT/scripts/lib/replay-args.sh"   # `replay_args_path_clean()`、`replay_args_remove_worktrees_under()`
WORK="$(mktemp -d)"
SCRATCH="$WORK/scratch"
BUNDLE_ROOT="$WORK/baselines"
STAGE1_OUT="$WORK/stage1"
STAGE2_OUT="$WORK/stage2"
IMAGE="${PY_IMAGE:-stock-trading-python-test:latest}"

cleanup() {
  git -C "$REPO_ROOT" worktree remove --force "$SCRATCH" >/dev/null 2>&1 || true
  git -C "$REPO_ROOT" worktree remove --force "$WORK/verify" >/dev/null 2>&1 || true
  # 反事實段中途失敗時，專屬 TMPDIR 底下的 worktree 登記也要收掉（⛔ 範圍外一律不動）。
  # ⚠️ 失敗時函式自己印 ERROR（⛔ 不靜默）；trap 裡⛔ 不改寫結束碼。
  [ ! -d "$WORK/cf_tmp" ] || replay_args_remove_worktrees_under "$REPO_ROOT" "$WORK/cf_tmp" || true
  rm -rf "$WORK"
}
trap cleanup EXIT

fails=0
pass() { echo "  ok   $1"; }
fail() { echo "  FAIL $1" >&2; fails=$((fails + 1)); }

# ── ⓪ 前置：反事實段的來源 bundle 必須乾淨（⚠️ 在 docker build 與**任何 replay 之前**） ─────────────
# ⚠️ `replay_args_path_clean()` 把「`git status` 失敗」與「有改動」分開，兩者都⛔ 不當成乾淨（⑦a 實作第二輪
# review：`[ -z "$(git status …)" ]` 在 status 失敗時照樣成立）。⛔ 不通過就整支中止，⛔ 不啟動任何 replay。
CF_SOURCE_ID=b1_20260901_1d_74350966_5d7ecb10
CF_SOURCE="$REPO_ROOT/python/baselines/$CF_SOURCE_ID"
echo "==> 前置：反事實段的來源 bundle 必須乾淨"
if ! replay_args_path_clean "$REPO_ROOT" "$CF_SOURCE"; then
  echo "==> smoke 中止：來源 bundle 不乾淨或讀不到 git status——⛔ 不啟動任何 replay" >&2
  exit 1
fi
pass "來源 bundle 在 replay 之前是乾淨的（git status 成功且為空）"
CF_SRC_SHA="$(cat "$CF_SOURCE/manifest.sha256")"

echo "==> 建置 image：$IMAGE"
docker build -q -t "$IMAGE" "$REPO_ROOT/python" >/dev/null

# ── ① 產一份小型 bundle（用目前工作樹的程式碼，⛔ 不碰 DB） ────────────────
echo "==> 產生小型 bundle（不讀 DB）"
mkdir -p "$BUNDLE_ROOT"
# ⚠️ `/app` 唯讀、只有 `/out` 可寫（產生器只寫 `/out`）。
docker run --rm --network none \
  --user "$(id -u):$(id -g)" \
  -e HOME=/tmp -e PYTHONDONTWRITEBYTECODE=1 \
  -v "$REPO_ROOT/python":/app:ro \
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
echo "==> 組 tooling patch（目前工作樹，⛔ 不再注入任何假欄位）"
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
# ⚠️ **2026-09-14：這裡原本會注入一個假的 `rr_decoupling_candidate`**（I-074 Stage 0
# 完成前產品端還沒有那個欄位）。⛔ **已移除**——產品端現在會自己產出真欄位，而同名鍵
# 在同一個 dict literal 裡排在後面會**覆蓋掉它**，於是正式資料流壞掉也照樣綠燈。
# 現在 smoke 驗的是**真實 candidate**；合成資料自然零命中時，**空 cohort 是合法結果**。
git -C "$SCRATCH" add -A -N >/dev/null
git -C "$SCRATCH" diff --binary HEAD > "$WORK/tooling.patch"
# ⚠️ **工作樹乾淨時 patch 是空的**（剛 commit 完就會這樣），而 `git apply` 對空輸入會報
# `error: unrecognized input`——⛔ 那不是失敗，是「這一輪沒有 tooling 變更」。
# `run-replay-offline.sh` 本來就用 `if [ -n "$patch_file" ]` 處理這件事
# （空 patch 時 `tooling_patch_sha256` 就是空字串的 SHA-256），smoke 要跟它一致。
if [ -s "$WORK/tooling.patch" ]; then
  TOOLING_PATCH_ARG="$WORK/tooling.patch"
else
  echo "  note 工作樹與 HEAD 相同——本輪沒有 tooling patch（⛔ 不是錯誤）" >&2
  TOOLING_PATCH_ARG=""
fi

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
[ -n "$TOOLING_PATCH_ARG" ] && git -C "$VERIFY" apply --index "$TOOLING_PATCH_ARG"
if python3 "$WORK/verify_patch.py" "$SCRATCH" "$VERIFY"; then
  pass "tooling patch 忠實表達工作樹（含刪除）"
else
  fail "tooling patch 沒有忠實表達工作樹——smoke 會跑在不存在的程式碼組合上"
fi
git -C "$REPO_ROOT" worktree remove --force "$VERIFY" >/dev/null 2>&1 || true
git -C "$REPO_ROOT" worktree remove --force "$SCRATCH" >/dev/null 2>&1

# ── ③ Stage 1（after 版本） ───────────────────────────────────────────────
echo "==> Stage 1（after）"
if TOOLING_PATCH="$TOOLING_PATCH_ARG" AFTER_REF=HEAD \
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
if TOOLING_PATCH="$TOOLING_PATCH_ARG" AFTER_REF=HEAD \
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
# ⚠️ 這一條現在驗的是**產品端真的產出了欄位**，⛔ 不再是 smoke 自己注入的假值。
assert all("rr_decoupling_candidate" in r for r in after["rows"])
# ⚠️ **⛔ 不再斷言 cohort 非空**（2026-09-14）：移除假 candidate 之後，合成資料很可能
# 自然零命中，而**空 cohort 是合法結果**——Stage 1／2 都成功、artifact 都產出來了。
# 非空 cohort 的比較路徑改由 artifact-layer 的 pytest 涵蓋
# （`test_replay_bundle_stages.py` 的 stub，⛔ 不經過產品資料流）。
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

# ── ⑥ I-074 Stage 2 的反事實路徑（⑦a 細部計畫 B7） ──────────────────────────
echo "==> I-074 Stage 2 反事實路徑（正式 bundle 的切片 ＋ 真正的 counterfactual ＋ 工作樹現行的 tooling）"
CF_BASE=e1cbbbdab44f8cf2d152e6ade9235d844f590d7f
CF_EXPECTED_ID=b1_20260901_1d_de3ab843_a7c9ffb4
CF_PATCH="$REPO_ROOT/python/baselines/i074_stage2/counterfactual_e1cbbbd.patch"
CF_OUT="$WORK/cf_baselines"
CF_S1="$WORK/cf_stage1"
CF_S2="$WORK/cf_stage2"
# ⚠️ runner 走到 `exec docker run` 時 EXIT trap 不會執行，worktree 會留下（I-118 的既有成因）——這一段收掉自己
# 造成的登記。⛔ **不用「前後差集」**（⑦a 實作第一輪 review）：smoke 跑數分鐘，同時段別人建立的 worktree 也會
# 出現在差集裡而被誤刪。runner 以**本次專屬、新建**的 TMPDIR 執行，只清這個目錄底下的登記。
CF_TMP="$WORK/cf_tmp"
mkdir -p "$CF_OUT" "$CF_TMP"
cf_ok=1
# ⚠️ 來源 bundle 是否乾淨已在最前面（⓪）驗過——不乾淨或讀不到狀態時 smoke 根本不會走到這裡。

# 切片 fixture：⚠️ `/app` 唯讀、只有 `/out`（smoke 的暫存目錄）可寫；產生器自己驗 ID、只有 6243、恰好 250 根。
if [ "$cf_ok" = 1 ] && docker run --rm --network none --user "$(id -u):$(id -g)" -e HOME=/tmp -e PYTHONDONTWRITEBYTECODE=1 \
     -v "$REPO_ROOT/python":/app:ro -v "$CF_OUT":/out -w /app "$IMAGE" \
     python scripts/make_counterfactual_smoke_bundle.py "/app/baselines/$CF_SOURCE_ID" /out > "$WORK/cf_bundle_id"; then
  CF_BUNDLE_ID="$(tail -1 "$WORK/cf_bundle_id")"
else
  CF_BUNDLE_ID=""
fi
# ⚠️ **replay 之前**再驗一次：ID 與實際目錄名都 ＝ 記錄的常數，否則⛔ 不跑任何 replay。
if [ "$cf_ok" = 1 ]; then
  if [ "$CF_BUNDLE_ID" = "$CF_EXPECTED_ID" ] && [ -d "$CF_OUT/$CF_EXPECTED_ID" ] \
     && [ "$(ls "$CF_OUT")" = "$CF_EXPECTED_ID" ]; then
    pass "切片 bundle：$CF_BUNDLE_ID（只有 6243、250 根）"
  else
    fail "切片 bundle 的 ID 不符（'$CF_BUNDLE_ID'）——⛔ 不跑反事實的 replay"
    cf_ok=0
  fi
fi

if [ "$cf_ok" = 1 ]; then
  # tooling patch：工作樹現行的 python/ → 暫存 index 的 tree → 產生器（⛔ 不動真正的 index）。
  CF_INDEX="$WORK/cf_index"
  GIT_INDEX_FILE="$CF_INDEX" git -C "$REPO_ROOT" read-tree HEAD
  GIT_INDEX_FILE="$CF_INDEX" git -C "$REPO_ROOT" add -A python
  CF_TREE="$(GIT_INDEX_FILE="$CF_INDEX" git -C "$REPO_ROOT" write-tree)"
  if "$REPO_ROOT/scripts/make-i074-tooling-patch.sh" --source-tree "$CF_TREE" > "$WORK/cf_tooling.patch"; then
    pass "tooling patch 由工作樹現行內容產生（$(wc -c < "$WORK/cf_tooling.patch") bytes）"
  else
    fail "產生不了 tooling patch"; cf_ok=0
  fi
fi

if [ "$cf_ok" = 1 ]; then
  IMAGE_ID="$(docker image inspect "$IMAGE" -f '{{.Id}}')"
  XDG_DATA_HOME="$WORK/xdg" python3 "$REPO_ROOT/python/scripts/ensure-i074-run-identity.py" \
    --stage 2 --bundle "$CF_OUT/$CF_EXPECTED_ID" --image-id "$IMAGE_ID" >/dev/null
  # Stage 1（after 側）：`e1cbbbd` ＋ tooling（⛔ 沒有 counterfactual——RR 已解耦的正式 after 語意）。
  if env -u PY_IMAGE TMPDIR="$CF_TMP" REPLAY_IMAGE_ID="$IMAGE_ID" TOOLING_PATCH="$WORK/cf_tooling.patch" AFTER_REF="$CF_BASE" \
       "$REPO_ROOT/scripts/run-replay-offline.sh" --bundle "$CF_OUT/$CF_EXPECTED_ID" --output-dir "$CF_S1" \
       --before-ref "$CF_BASE" > "$WORK/cf_stage1.log" 2>&1; then
    pass "反事實 smoke 的 Stage 1（e1cbbbd ＋ tooling）跑完"
  else
    fail "反事實 smoke 的 Stage 1 失敗"; tail -30 "$WORK/cf_stage1.log" >&2; cf_ok=0
  fi
fi

if [ "$cf_ok" = 1 ]; then
  # Stage 2（before 側）：`e1cbbbd` ＋ counterfactual ＋ 同一份 tooling，⚠️ 反事實模式。
  set +e
  env -u PY_IMAGE TMPDIR="$CF_TMP" REPLAY_IMAGE_ID="$IMAGE_ID" XDG_DATA_HOME="$WORK/xdg" I074_STAGE=2 \
    COUNTERFACTUAL_PATCH="$CF_PATCH" TOOLING_PATCH="$WORK/cf_tooling.patch" \
    "$REPO_ROOT/scripts/run-replay-offline.sh" --bundle "$CF_OUT/$CF_EXPECTED_ID" --output-dir "$CF_S2" \
    --before-ref "$CF_BASE" --after-artifact "$CF_S1/after_artifact.json" \
    --cohort-manifest "$CF_S1/cohort_manifest.json" --i074-counterfactual > "$WORK/cf_stage2.log" 2>&1
  cf_rc=$?
  set -e
  if [ "$cf_rc" -eq 0 ]; then
    pass "反事實 Stage 2 回 0（counterfactual 生效）"
  else
    fail "反事實 Stage 2 回 $cf_rc（6＝反事實沒有生效）"; tail -30 "$WORK/cf_stage2.log" >&2; cf_ok=0
  fi
fi

if [ "$cf_ok" = 1 ]; then
  python3 - "$REPO_ROOT/python" "$CF_S1" "$CF_S2" "$CF_OUT/$CF_EXPECTED_ID" <<'CFPY' && pass "反事實輸出：恰好三檔、cohort ≥ 1、comparison 非空且逐列翻轉" || fail "反事實輸出不符"
import hashlib, json, sys
from pathlib import Path
python, s1, s2, bundle = (Path(a) for a in sys.argv[1:])
sys.path.insert(0, str(python / "scripts"))
from _i074_bootstrap import STAGE2_ARCHIVE_MODULES, load_replay_bundle
mods = load_replay_bundle(python, STAGE2_ARCHIVE_MODULES)
art, sa = mods["artifacts"], mods["stage2_archive"]
assert {p.name for p in s2.iterdir()} == {"before_source_artifact.json", "comparison_artifact.json", "report.json"}
keys = [tuple(k) for k in json.loads((s1 / "cohort_manifest.json").read_text(encoding="utf-8"))["keys"]]
assert len(keys) >= 1, "cohort 是空的——這一段就驗不到非空翻轉"
raw = (s2 / "comparison_artifact.json").read_bytes()
comparison = json.loads(raw)
assert art.validate_comparison_artifact(comparison) == sorted(keys)
report_max = json.loads((bundle / "manifest.json").read_text(encoding="utf-8"))["report_max_rows"]
art.validate_report(json.loads((s2 / "report.json").read_text(encoding="utf-8")), comparison=comparison,
                    comparison_sha256=hashlib.sha256(raw).hexdigest(), report_max_rows=report_max)
for row in comparison["rows"]:
    assert row["after"]["rr_decoupling_candidate"] is True and row["before"]["rr_decoupling_candidate"] is False
    assert "lifecycle_phase" in row["differences"] and row["before"]["lifecycle_phase"] != "CONTINUATION"
before = sa.stream_before_source(s2 / "before_source_artifact.json", keep_keys=set(keys))
assert before.effect is None and before.candidate_keys == []
print(f"    before rows={len(before.keys)} cohort={len(keys)} comparison={len(comparison['rows'])}")
CFPY
fi

if [ "$(cat "$CF_SOURCE/manifest.sha256")" = "$CF_SRC_SHA" ] && replay_args_path_clean "$REPO_ROOT" "$CF_SOURCE"; then
  pass "來源 bundle 未被修改（manifest.sha256 前後相同、結束時 git status 成功且仍為空）"
else
  fail "來源 bundle 在 smoke 期間被修改了（或結束時 git status 失敗／不為空）"
fi
# 只清本次專屬 TMPDIR 底下的 worktree 登記（⛔ 範圍外一律不動；list 或 remove 失敗 → 記為失敗）。
replay_args_remove_worktrees_under "$REPO_ROOT" "$CF_TMP" \
  && pass "反事實段的 worktree 登記（專屬 TMPDIR 底下）已收掉" || fail "反事實段的 worktree 清理失敗"

if [ "$fails" -ne 0 ]; then
  echo "==> smoke 失敗：$fails 項" >&2
  exit 1
fi
echo "==> smoke 全部通過"
