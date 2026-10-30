#!/usr/bin/env bash
# I-074 Stage 2 B／C 判讀器（scripts/judge-i074-stage2.sh）的 bootstrap 測試（host 端）。
# T-077 自 scripts/test-i074-stage2.sh 抽出（該檔已刪除；最後版本見 c6c2481）；案例與期望值照原樣。
#
#   scripts/test-judge-i074-stage2.sh   # 由 python/scripts/test.sh 在 SKIP_SHELL_TESTS 那一段呼叫
#
# ⚠️ 隔離：
#   - 判讀器複製到 mktemp 底下的合成 repo，以 sed 只改常數區塊的 `I074_TRUSTED_PATH`、`I074_GIT` 兩行，
#     並斷言與正式檔案**只差那幾行**——正式檔案⛔ 沒有執行期覆寫口；
#   - git 是轉交真 git 的包裝；`base_commit` 裡的驗證入口是 fake（記錄呼叫、依情境回傳）；
#   - ⛔ 不需要 docker 與 image；⛔ 不碰真正 repo 的工作樹、index 與 refs；結束時斷言真正 repo 的 worktree 登記數不變。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
JUDGE_REL=scripts/judge-i074-stage2.sh
EVIDENCE_REL=python/baselines/i074_stage2/evidence
fails=0
pass() { echo "  ok   $1"; }
fail() { echo "  FAIL $1" >&2; fails=$((fails + 1)); }
check() { local label="$1"; shift; if "$@"; then pass "$label"; else fail "$label"; fi; }

WT_BEFORE="$(git -C "$REPO_ROOT" worktree list --porcelain | grep -c '^worktree ' || true)"
TD="$(mktemp -d)"
chmod 700 "$TD"
trap 'rm -rf -- "$TD"' EXIT

TRUSTED="$TD/trusted"
mkdir -m 755 "$TRUSTED"
TEST_PATH="$TRUSTED:/usr/bin:/bin"
IMG="sha256:$(printf 'i074-judge-test-image' | sha256sum | cut -d' ' -f1)"
JTMP="$TD/jtmp"          # 判讀器的 TMPDIR：暫存複本都建在這裡，每一次執行之後必須清空
S="$TD/s"
mkdir -p "$JTMP" "$S"

# ── git 包裝（sed 過的 I074_GIT 與 I074_TRUSTED_PATH 指向這裡；其餘轉交真的 git） ─────────────
cat > "$TRUSTED/git" <<'FAKEGIT'
#!/bin/bash
exec /usr/bin/git "$@"
FAKEGIT
chmod 755 "$TRUSTED/git"

# ── 呼叫者 PATH 前置的 fake 程式：一旦被執行就留下記號 ───────────────────────────
mkdir -p "$TD/evil"
for p in git docker python3; do
  printf '#!/bin/sh\ntouch "%s/evil_%s_ran"\nexit 1\n' "$TD" "$p" > "$TD/evil/$p"
  chmod 755 "$TD/evil/$p"
done

# ── 合成的「真正 repo」 ─────────────────────────────────────────────────────────────
SR="$TD/real"
git init -q "$SR"
git -C "$SR" config user.name t; git -C "$SR" config user.email t@t; git -C "$SR" config commit.gpgsign false
mkdir -p "$SR/scripts" "$SR/$EVIDENCE_REL"
printf 'counterfactual anchor\n' > "$SR/python/baselines/i074_stage2/counterfactual_e1cbbbd.patch"
printf 'tooling anchor\n' > "$SR/python/baselines/i074_stage2/tooling_e1cbbbd.patch"

# 正式判讀器 → sed 改常數（⚠️ 只改這兩行；下方斷言 diff 只落在這兩行）
sed -e "s|^I074_TRUSTED_PATH=.*|I074_TRUSTED_PATH=$TEST_PATH|" -e "s|^I074_GIT=.*|I074_GIT=$TRUSTED/git|" \
  "$REPO_ROOT/$JUDGE_REL" > "$SR/$JUDGE_REL"
chmod --reference="$REPO_ROOT/$JUDGE_REL" "$SR/$JUDGE_REL"

# fake 驗證入口（只實作 --verify-promotion-staging）。⚠️ 它在判讀器的暫存複本裡執行，
#   REPO_ROOT 是複本；錨點⛔ 不是 base_commit 那一版就拒絕（證明判讀器跑的是 base_commit 的樹）。
cat > "$SR/scripts/finalize-stage2-evidence.sh" <<'FAKEFIN'
#!/usr/bin/env bash
S="${I074_TEST_SCENARIO:?}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
mkdir -p "$S/spy"
case "$1" in
  --verify-promotion-staging)
    path="$2"; target="$4"; judge="${5:-}"
    printf 'VERIFY %s %s %s\n' "$path" "$target" "$judge" >> "$S/spy/verify.log"
    rc="$(cat "$S/verify_rc" 2>/dev/null || echo 0)"; [ "$rc" = 0 ] || exit "$rc"
    [ ! -f "$S/verify_needs" ] || [ -e "$REPO_ROOT/$(cat "$S/verify_needs")" ] || { echo "fake verify：缺錨點" >&2; exit 1; }
    [ "$(cat "$REPO_ROOT/python/baselines/i074_stage2/tooling_e1cbbbd.patch")" = "tooling anchor" ] \
      || { echo "fake verify：錨點⛔ 不是 base_commit 那一版" >&2; exit 1; }
    extra=""
    [ "$judge" != --judge ] || extra=',"verdict":{"schema":"i074_stage2_verdict/v1","verdict":"B"}'
    printf '{"base_commit":"%s","kind":"evidence","manifest_sha256":"%s","target":"%s"%s}\n' \
      "$(git -C "$REPO_ROOT" rev-parse HEAD)" "$(sha256sum < "$path/evidence_manifest.json" | cut -d' ' -f1)" \
      "$target" "$extra"
    exit 0 ;;
esac
exit 99
FAKEFIN
chmod 755 "$SR/scripts/finalize-stage2-evidence.sh"
git -C "$SR" add -A && git -C "$SR" commit -qm "base（⑩ 之前寫好的錨點與驗證入口）"
BASE_OID="$(git -C "$SR" rev-parse HEAD)"

# 晉升之後的 evidence（manifest 指向 base_commit），之後再 commit
MANIFEST="$SR/$EVIDENCE_REL/evidence_manifest.json"
set_judge_base() {
  python3 -c 'import json,sys; json.dump({"finalizer_provenance": {"base_commit": sys.argv[2]}}, open(sys.argv[1], "w"))' \
    "$MANIFEST" "$1"
}
set_judge_base "$BASE_OID"
git -C "$SR" add -A && git -C "$SR" commit -qm "evidence"
# 對應原本的 n9 (ii)(iii)：HEAD 改掉錨點並把驗證入口改壞（commit），工作樹再改一次錨點（⛔ 不 commit）
printf 'tampered anchor\n' >> "$SR/python/baselines/i074_stage2/tooling_e1cbbbd.patch"
printf 'exit 1\n' > "$SR/scripts/finalize-stage2-evidence.sh"
git -C "$SR" commit -qam "HEAD：錨點與驗證入口都改壞"
printf 'tampered again\n' >> "$SR/python/baselines/i074_stage2/counterfactual_e1cbbbd.patch"
HEAD_OID="$(git -C "$SR" rev-parse HEAD)"

judge_run() {  # 其餘＝參數
  RC=0
  env -i HOME="$HOME" PATH="${CALLER_PATH:-$TEST_PATH}" TMPDIR="$JTMP" REPLAY_IMAGE_ID="${IMAGE_OVERRIDE-$IMG}" \
    I074_TEST_SCENARIO="$S" "$SR/$JUDGE_REL" "$@" > "$S/out" 2> "$S/err" || RC=$?
}
judge_ok() { [ "$RC" = 0 ] && [ "$(grep -c . "$S/out")" = 1 ] && grep -q '"verdict":{' "$S/out"; }
judge_refused() { [ "$RC" = 1 ] && [ ! -s "$S/out" ]; }
# ⚠️ 拒絕要是**預期的那一道**守門擋下的（⛔ 不能只看結束碼：別的守門的拒絕也會被當成通過）
judge_refused_with() { judge_refused && grep -qF -- "$1" "$S/err"; }
jtmp_empty() { [ -z "$(ls -A "$JTMP")" ]; }

echo "==> 判讀器：靜態檢查"
only_const_diff() {  # $1＝正式檔；$2＝副本；$3＝允許差異的行首正規式；$4＝預期差異行數
  local d n
  d="$(diff "$REPO_ROOT/$1" "$2" | grep '^[<>]' || true)"
  n="$(grep -c . <<< "$d" || true)"
  [ "$n" = "$4" ] && ! grep -vqE "^[<>] ($3)" <<< "$d"
}
check "sed 副本只差常數行（I074_TRUSTED_PATH、I074_GIT）" only_const_diff "$JUDGE_REL" "$SR/$JUDGE_REL" 'I074_(TRUSTED_PATH|GIT)=' 4
check "shebang：第一行恰好是 #!/bin/bash -p" bash -c '[ "$(head -n1 "$1")" = "#!/bin/bash -p" ]' _ "$REPO_ROOT/$JUDGE_REL"
static_rules() {
  python3 - "$REPO_ROOT/$JUDGE_REL" <<'PY'
import re, sys
rel = sys.argv[1]
cmd_docker = re.compile(r'(^|[;&|(]|\b(?:then|do|else|exec|!)\s)\s*("?\$\{?\w*DOCKER\w*\}?"?|\S*/docker)(\s|$)')
bad, path_assign = [], []
for n, line in enumerate(open(rel, encoding="utf-8"), 1):
    code = line.split("#", 1)[0] if not line.lstrip().startswith("#") else ""
    if not code.strip():
        continue
    if cmd_docker.search(code):
        bad.append(f"第 {n} 行以絕對路徑或變數呼叫 docker：{line.strip()}")
    if "command -p" in code:
        bad.append(f"第 {n} 行 command -p")
    for m in re.finditer(r'(?<![\w$])PATH=(\S*)', code):
        path_assign.append(m.group(1))
    if re.search(r'\bexport\s+PATH\b', code):
        path_assign.append("export")
if sorted(path_assign) != sorted(['"$I074_TRUSTED_PATH"', "export"]):
    bad.append(f"PATH 的指派不是恰好一處常數：{path_assign}")
for b in bad:
    print(b, file=sys.stderr)
sys.exit(1 if bad else 0)
PY
}
check "⛔ 不以絕對路徑／變數呼叫 docker、⛔ 沒有 command -p、PATH 恰好一處常數指派 ＋ 一次 export" static_rules
trust_roots() {
  python3 - "$REPO_ROOT/$JUDGE_REL" <<'PY'
import re, sys
text = open(sys.argv[1], encoding="utf-8").read()
want = {"I074_TRUSTED_PATH": "/usr/bin:/bin", "I074_GIT": "/usr/bin/git", "I074_PYTHON": "/usr/bin/python3"}
got = {k: (re.findall(rf"^{k}=(\S+)$", text, re.M) or [None]) for k in want}
bad = [f"{k}={v}（預期恰好一行 {want[k]}）" for k, v in got.items() if v != [want[k]]]
for b in bad:
    print(b, file=sys.stderr)
sys.exit(1 if bad else 0)
PY
}
check "信任根常數：I074_TRUSTED_PATH=/usr/bin:/bin、I074_GIT=/usr/bin/git、I074_PYTHON=/usr/bin/python3" trust_roots

echo "==> 判讀器：bootstrap 與流程（真正 repo 的 HEAD 與工作樹都已改掉錨點，HEAD 的驗證入口已壞）"
CALLER_PATH="$TD/evil:$TEST_PATH" judge_run
check "HEAD 與工作樹都改了錨點、HEAD 的驗證入口已壞、呼叫者 PATH 前置 fake 程式 → 照樣判讀（base_commit 那一版）" judge_ok
check "stdout 那一行 JSON 的 base_commit ＝ manifest 的 base_commit（暫存複本的 HEAD）" \
  grep -qF "\"base_commit\":\"$BASE_OID\"" "$S/out"
check "驗證模式在 base_commit 的暫存複本裡、以 --judge 驗真正 repo 的 evidence/" \
  grep -qxF "VERIFY $SR/$EVIDENCE_REL evidence --judge" "$S/spy/verify.log"
check "呼叫者 PATH 前置的 fake git／docker／python3 ⛔ 沒有被執行" bash -c '! ls "$1"/evil_*_ran >/dev/null 2>&1' _ "$TD"
RC=0
env -i HOME="$HOME" PATH="$TEST_PATH" TMPDIR="$JTMP" REPLAY_IMAGE_ID="$IMG" I074_TEST_SCENARIO="$S" \
  bash "$SR/$JUDGE_REL" > "$S/out" 2> "$S/err" || RC=$?
check "bash <script> → 1、⛔ 不輸出 B／C" judge_refused_with "必須直接執行"
judge_run --target evidence
check "⛔ 沒有參數（帶了 → 1）" judge_refused_with "沒有參數"
IMAGE_OVERRIDE="sha256:$(printf x | sha256sum | cut -c1-63)" judge_run
check "REPLAY_IMAGE_ID 格式錯 → 1" judge_refused_with "REPLAY_IMAGE_ID 必須是"
cp "$SR/$JUDGE_REL" "$TD/judge.saved"
printf '\n# dirty\n' >> "$SR/$JUDGE_REL"
judge_run
check "腳本的內容 ≠ HEAD → 1" judge_refused_with "≠ HEAD 中的版本"
cp "$TD/judge.saved" "$SR/$JUDGE_REL"
cp "$MANIFEST" "$TD/manifest.saved"
set_judge_base "$(printf 'f%.0s' {1..40})"
judge_run
check "base_commit 不可達 → 1、⛔ 不輸出 B／C" judge_refused_with "不可達"
LACK_IDX="$TD/lack.index"
GIT_INDEX_FILE="$LACK_IDX" git -C "$SR" read-tree "$BASE_OID"
GIT_INDEX_FILE="$LACK_IDX" git -C "$SR" rm -q --cached python/baselines/i074_stage2/counterfactual_e1cbbbd.patch
LACK="$(git -C "$SR" commit-tree -p "$BASE_OID" -m "lack anchor" "$(GIT_INDEX_FILE="$LACK_IDX" git -C "$SR" write-tree)")"
git -C "$SR" update-ref refs/heads/lack-anchor "$LACK"
set_judge_base "$LACK"
echo "python/baselines/i074_stage2/counterfactual_e1cbbbd.patch" > "$S/verify_needs"
judge_run
check "base_commit 的樹裡缺錨點 → 驗證模式不通過 → 1、⛔ 不輸出 B／C" bash -c '[ "$1" = 1 ] && [ ! -s "$2/out" ] && grep -qF "fake verify：缺錨點" "$2/err" && grep -qF "驗證模式不通過" "$2/err"' _ "$RC" "$S"
rm -f "$S/verify_needs"
cp "$TD/manifest.saved" "$MANIFEST"
echo 1 > "$S/verify_rc"
judge_run
check "驗證模式不通過 → 1、⛔ 不輸出 B／C" judge_refused_with "驗證模式不通過（rc=1）"
rm -f "$S/verify_rc"
judge_run
check "同一份 evidence 再判一次 → 0（唯讀、可重跑）" judge_ok
check "判讀器的暫存複本每一次都清掉（TMPDIR 底下沒有殘留）" jtmp_empty
check "判讀器⛔ 沒有改動真正 repo 的 HEAD" bash -c '[ "$(git -C "$1" rev-parse HEAD)" = "$2" ]' _ "$SR" "$HEAD_OID"

echo "==> 判讀器：隔離"
WT_AFTER="$(git -C "$REPO_ROOT" worktree list --porcelain | grep -c '^worktree ' || true)"
check "真正 repo 的 worktree 登記數⛔ 沒有變化（$WT_BEFORE）" bash -c '[ "$1" = "$2" ]' _ "$WT_BEFORE" "$WT_AFTER"

if [ "$fails" -ne 0 ]; then
  echo "==> test-judge-i074-stage2：失敗 $fails 項" >&2
  exit 1
fi
echo "==> test-judge-i074-stage2：全部通過"
