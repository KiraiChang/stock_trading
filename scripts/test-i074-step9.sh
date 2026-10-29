#!/usr/bin/env bash
# I-074 Stage 2 步驟 ⑨ 的 shell 測試（git show 20eacb2:docs/issue.md 的 I-074「Stage 2 步驟 ⑨ 細部計畫 v1」「八」#5、#5b、#5c）。
#
# 由 `scripts/test-replay-args.sh` 呼叫（也可以單獨執行——反向驗證就是這樣跑的）。標籤一律以 `step9：` 開頭
# （`python/scripts/i074_step9_runs.py` 據此辨認 ⑨ 新增的項目）。
#
# ⚠️ 全部在本腳本自己的暫存目錄裡：fake docker（以狀態檔模擬 image 庫）、隔離 repo（入口以「複製 ＋ sed 常數」
#   換成隔離 repo 的值，並斷言只差常數行）；⛔ 不碰真正的 docker、⛔ 不碰真正 repo 的 worktree 登記。
# ⚠️ 長存的 stub 一律帶 token（`exec -a step9-stub-<token>`），斷言之後確認⛔ 沒有存活；⛔ 不用 pkill -f。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fails=0
pass() { echo "  ok   step9：$1"; }
fail() { echo "  FAIL step9：$1" >&2; fails=$((fails + 1)); }

TD="$(mktemp -d)"
trap 'chmod -R u+w "$TD" 2>/dev/null || true; rm -rf "$TD"' EXIT
FAKE="$TD/bin"
mkdir -p "$FAKE"

# ── fake docker：狀態檔一行一筆「ID REF」（沒有 tag 的 image ＝「ID <none>:<none>」）────────────────
cat > "$FAKE/docker" <<'FAKEEOF'
#!/usr/bin/env bash
set -euo pipefail
state="${FAKE_DOCKER_STATE:?}"
echo "$*" >> "${FAKE_DOCKER_LOG:-/dev/null}"
touch "$state"
none='<none>:<none>'
case "${1:-}" in
  image)
    case "${2:-}" in
      ls) cat "$state" ;;
      rm)
        [ "${FAKE_RM_FAIL:-0}" != 1 ] || { echo "fake: image rm 失敗" >&2; exit 1; }
        tag="$3"; id="$(awk -v t="$tag" '$2 == t { print $1 }' "$state")"
        [ -n "$id" ] || { echo "fake: no such image $tag" >&2; exit 1; }
        if [ -n "$(awk -v i="$id" -v t="$tag" -v n="$none" '$1 == i && $2 != t && $2 != n' "$state")" ]; then
          awk -v t="$tag" '$2 != t' "$state" > "$state.new"
        else
          awk -v i="$id" '$1 != i' "$state" > "$state.new"
        fi
        mv "$state.new" "$state" ;;
      inspect) awk -v t="$3" '$2 == t { print $1 }' "$state" ;;
      *) exit 0 ;;
    esac ;;
  build)
    tag=""; shift
    while [ "$#" -gt 0 ]; do [ "$1" = -t ] && { tag="$2"; shift; }; shift; done
    id="${FAKE_BUILD_ID:-sha256:$(od -An -N32 -tx1 /dev/urandom | tr -d ' \n')}"
    old="$(awk -v t="$tag" '$2 == t { print $1 }' "$state")"
    awk -v t="$tag" '$2 != t' "$state" > "$state.new"; mv "$state.new" "$state"
    if [ -n "$old" ] && [ -z "$(awk -v i="$old" '$1 == i' "$state")" ]; then echo "$old $none" >> "$state"; fi
    awk -v i="$id" -v n="$none" '!($1 == i && $2 == n)' "$state" > "$state.new"; mv "$state.new" "$state"
    echo "$id $tag" >> "$state" ;;
  ps) exit 0 ;;
  version) echo fake ;;
  *) exit 0 ;;
esac
FAKEEOF
chmod +x "$FAKE/docker"

HELPER="$REPO_ROOT/scripts/i074-step9-image.sh"
RUN_A=20261012t000000z-0a1b2c3d
RUN_B=20261012t000100z-1b2c3d4e
TAG1="stock-trading-i074-step9:$RUN_A-smoke"
TAG2="stock-trading-i074-step9:$RUN_B-smoke"
INITIAL=("sha256:aaa stock-trading-python-test:latest" "sha256:ddd <none>:<none>" "sha256:fff other:1")

h_case() {  # $1＝名稱：建立新的狀態檔（INITIAL）與路徑變數
  H_DIR="$TD/h_$1"; mkdir -p "$H_DIR"
  STATE="$H_DIR/state"; LOG="$H_DIR/docker.log"; INV="$H_DIR/inv"; ALLOW="$H_DIR/allow"
  printf '%s\n' "${INITIAL[@]}" > "$STATE"; : > "$LOG"
}
hx() { PATH="$FAKE:$PATH" FAKE_DOCKER_STATE="$STATE" FAKE_DOCKER_LOG="$LOG" "$HELPER" "$@"; }
hbuild() { PATH="$FAKE:$PATH" FAKE_DOCKER_STATE="$STATE" FAKE_DOCKER_LOG="$LOG" FAKE_BUILD_ID="${2:-}" docker build -t "$1" . > /dev/null; }
state_sorted() { LC_ALL=C sort "$STATE"; }
INITIAL_SORTED="$(printf '%s\n' "${INITIAL[@]}" | LC_ALL=C sort)"

echo "==> step9：image helper（h1～h12）"
# h1
h_case h1; printf '%s\n' "sha256:abc $TAG1" >> "$STATE"
if hx reserve "$TAG1" "$INV" 2> "$H_DIR/err" > /dev/null; then fail "h1：tag 已存在卻 reserve 成功"
elif [ ! -e "$INV" ] && grep -q 'tag 已存在' "$H_DIR/err"; then pass "h1：tag 已存在 → reserve 拒絕、⛔ 沒有寫 inventory"
else fail "h1：拒絕的方式不符"; fi
# h2
h_case h2
bad=0
for t in stock-trading-python-test:latest "stock-trading-i074-step9:smoke" "stock-trading-i074-step9:$RUN_A-other"; do
  hx reserve "$t" "$INV.$RANDOM" 2>/dev/null && bad=1
done
SHA="$(hx reserve "$TAG1" "$INV")"
hx release stock-trading-python-test:latest "$INV" "$SHA" "$ALLOW" 2>/dev/null && bad=1
if [ "$bad" = 0 ] && ! grep -q 'image rm' "$LOG"; then pass "h2：命名空間外的 tag → reserve／release 都拒絕、⛔ 沒有 image rm"
else fail "h2：命名空間外的 tag 沒有全部被拒絕"; fi
# h3
h_case h3; SHA="$(hx reserve "$TAG1" "$INV")"; hbuild "$TAG1"
if hx release "$TAG1" "$INV" "$SHA" "$ALLOW" && hx verify "$TAG1" "$INV" "$SHA" "$ALLOW" \
   && [ "$(state_sorted)" = "$INITIAL_SORTED" ] && grep -q "image rm $TAG1" "$LOG" && [ ! -s "$ALLOW" ]; then
  pass "h3：新 image → release 刪除 tag 與 image、verify 通過"
else fail "h3：新 image 的收尾不符"; fi
# h4
h_case h4; SHA="$(hx reserve "$TAG1" "$INV")"; hbuild "$TAG1" sha256:aaa
if hx release "$TAG1" "$INV" "$SHA" "$ALLOW" && hx verify "$TAG1" "$INV" "$SHA" "$ALLOW" \
   && [ "$(state_sorted)" = "$INITIAL_SORTED" ]; then
  pass "h4：cache 命中既有、帶別的 tag 的 image → 只解除專屬 tag、verify 通過"
else fail "h4：既有帶 tag 的 image 的收尾不符"; fi
# h5
h_case h5; SHA="$(hx reserve "$TAG1" "$INV")"; hbuild "$TAG1" sha256:ddd
if hx release "$TAG1" "$INV" "$SHA" "$ALLOW" && hx verify "$TAG1" "$INV" "$SHA" "$ALLOW" \
   && ! grep -q 'image rm' "$LOG" && [ "$(cat "$ALLOW")" = "$TAG1 sha256:ddd" ]; then
  pass "h5：cache 命中既有的 dangling image → ⛔ 沒有 image rm、allowance 一筆、verify 通過"
else fail "h5：既有 dangling image 的收尾不符"; fi
H5_STATE="$STATE"; H5_INV="$INV"; H5_SHA="$SHA"; H5_ALLOW="$ALLOW"; H5_LOG="$LOG"
# h6
h_case h6; SHA="$(hx reserve "$TAG1" "$INV")"
if hx release "$TAG1" "$INV" "$SHA" "$ALLOW" && hx verify "$TAG1" "$INV" "$SHA" "$ALLOW" && ! grep -q 'image rm' "$LOG"; then
  pass "h6：build 之前就失敗（tag 不存在）→ release 不做事、verify 通過"
else fail "h6：tag 不存在時的收尾不符"; fi
# h7
h_case h7; SHA="$(hx reserve "$TAG1" "$INV")"; hbuild "$TAG1"
rc=0; FAKE_RM_FAIL=1 hx release "$TAG1" "$INV" "$SHA" "$ALLOW" 2>/dev/null || rc=$?
if [ "$rc" -ne 0 ] && [ ! -s "$ALLOW" ] && ! hx verify "$TAG1" "$INV" "$SHA" "$ALLOW" 2>/dev/null; then
  pass "h7：image rm 失敗 → release 非零、⛔ 沒有寫 allowance、verify 失敗（多一個 ID）"
else fail "h7：清理失敗的處理不符"; fi
# h8
h_case h8; SHA="$(hx reserve "$TAG1" "$INV")"; hbuild "$TAG1"
id="$(awk -v t="$TAG1" '$2 == t { print $1 }' "$STATE")"; echo "$id <none>:<none>" >> "$INV"
if ! hx release "$TAG1" "$INV" "$SHA" "$ALLOW" 2>/dev/null && ! hx verify "$TAG1" "$INV" "$SHA" "$ALLOW" 2>/dev/null \
   && ! grep -q 'image rm' "$LOG"; then
  pass "h8：inventory 在 reserve 之後被改（新 ID 冒充既有）→ release、verify 以 SHA 拒絕"
else fail "h8：被竄改的 inventory 沒有被拒絕"; fi
# h9（以 h5 的合法狀態為底，逐項竄改 allowance）
h9_check() {  # $1＝說明 $2＝allowance 內容 $3＝verify 用的 tag（預設 TAG1）
  local allow="$TD/h9_$RANDOM.allow"
  printf '%s' "$2" > "$allow"
  if STATE="$H5_STATE" LOG="$H5_LOG" hx verify "${3:-$TAG1}" "$H5_INV" "$H5_SHA" "$allow" 2>/dev/null; then
    fail "h9：$1 → verify 竟然通過"
  else pass "h9：$1 → verify 失敗"; fi
}
h9_check "allowance 多一筆" "$TAG1 sha256:ddd"$'\n'"$TAG1 sha256:ddd"$'\n'
h9_check "格式正確、同樣指向既有 dangling image 的另一個 ⑨ tag" "$TAG2 sha256:ddd"$'\n'
h9_check "本次的 tag 但指向帶 tag 的 ID" "$TAG1 sha256:aaa"$'\n'
h9_check "命名空間外的 tag" "other:1 sha256:ddd"$'\n'
h9_check "verify 收到的 tag ≠ inventory 記錄的 tag" "" "$TAG2"
# h10
h10_ok=1
for setup in new tagged dangling none; do
  h_case "h10_$setup"; SHA="$(hx reserve "$TAG1" "$INV")"
  case "$setup" in new) hbuild "$TAG1" ;; tagged) hbuild "$TAG1" sha256:aaa ;; dangling) hbuild "$TAG1" sha256:ddd ;; esac
  hx release "$TAG1" "$INV" "$SHA" "$ALLOW" && hx release "$TAG1" "$INV" "$SHA" "$ALLOW" \
    && hx verify "$TAG1" "$INV" "$SHA" "$ALLOW" || h10_ok=0
  [ ! -e "$ALLOW" ] || [ "$(grep -c . "$ALLOW")" -le 1 ] || h10_ok=0
done
[ "$h10_ok" = 1 ] && pass "h10：重複 release（四種情況）→ 結果與一次相同、allowance ⛔ 沒有重複" || fail "h10：重複 release ⛔ 不是冪等"
# h11（h5 留下 allowance 的 tag 之後再執行一次）
STATE="$H5_STATE"; LOG="$H5_LOG"; INV2="$TD/h11.inv"; ALLOW2="$TD/h11.allow"
if SHA2="$(hx reserve "$TAG2" "$INV2" 2> "$TD/h11.err")" && grep -qF "$TAG1" "$TD/h11.err" \
   && hbuild "$TAG2" && hx release "$TAG2" "$INV2" "$SHA2" "$ALLOW2" && hx verify "$TAG2" "$INV2" "$SHA2" "$ALLOW2"; then
  pass "h11：前次 allowance 留下的 tag ⛔ 不會卡住下一輪（reserve 列出它、第二輪 verify 通過）"
else fail "h11：成功之後再次執行失敗"; fi
# h12
h12_check() {  # $1＝說明；$2＝對狀態檔的改動（awk 以外的 shell 片段）
  h_case "h12_$RANDOM"; local sha
  sha="$(hx reserve "$TAG1" "$INV")"
  eval "$2"
  if hx verify "$TAG1" "$INV" "$sha" "$ALLOW" 2>/dev/null; then fail "h12：$1 → verify 竟然通過"
  else pass "h12：$1 → verify 失敗"; fi
}
h12_check "多一個 image ID" 'echo "sha256:eee <none>:<none>" >> "$STATE"'
h12_check "少一個原本存在的 ID" 'grep -v "^sha256:fff " "$STATE" > "$STATE.n"; mv "$STATE.n" "$STATE"'
h12_check "別的 tag 被移動" 'sed -i "s/^sha256:aaa stock-trading-python-test:latest/sha256:aaa <none>:<none>/" "$STATE"; echo "sha256:fff stock-trading-python-test:latest" >> "$STATE"'

# ── 隔離 repo ───────────────────────────────────────────────────────────────
ENTRY_REAL="$REPO_ROOT/scripts/i074-step9-guards.sh"
STUB_BODY='#!/usr/bin/env bash
set -euo pipefail
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
root="$(git -C "$here" rev-parse --show-toplevel)"
if [ "${STUB_BUILD:-1}" = 1 ]; then FAKE_BUILD_ID="${STUB_BUILD_ID:-}" docker build -t "$PY_IMAGE" "$root" > /dev/null; fi
[ -z "${STUB_EXTRA_IMAGE:-}" ] || FAKE_BUILD_ID="$STUB_EXTRA_IMAGE" docker build -t step9-extra:x "$root" > /dev/null
i=0; while [ "$i" -lt "${STUB_WORKTREES:-0}" ]; do i=$((i + 1)); git -C "$root" worktree add -q --detach "$TMPDIR/wt$i" HEAD; done
[ -z "${STUB_LOCK_WT:-}" ] || git -C "$root" worktree lock "$TMPDIR/wt1"
[ -z "${STUB_OUTSIDE_WT:-}" ] || git -C "$root" worktree add -q --detach "$(dirname "$TMPDIR")/outside_wt" HEAD
[ -z "${STUB_TOUCH_FORMAL:-}" ] || touch -m "$STUB_TOUCH_FORMAL"
[ -z "${STUB_STATUS_BLOCK:-}" ] || mkdir -p "$STUB_STATUS_BLOCK"
if [ -n "${STUB_MODIFY_FORMAL:-}" ]; then
  cp "$STUB_MODIFY_FORMAL" "$STUB_MODIFY_FORMAL.bak"
  printf "#!/usr/bin/env bash\ntouch %s\n" "$STUB_SENTINEL" > "$STUB_MODIFY_FORMAL"
  sleep 0.3
  mv "$STUB_MODIFY_FORMAL.bak" "$STUB_MODIFY_FORMAL"
fi
echo "STEP9_STUB_RAN $here/$(basename "$0")"
if [ -n "${STUB_STARTED:-}" ]; then touch "$STUB_STARTED"; exec -a "step9-stub-$STUB_TOKEN" sleep 30; fi
exit "${STUB_RC:-0}"
'

mk_ir() {  # $1＝目錄 $2＝MIN_FREE 常數 $3＝variant（normal／exportignore／hardlink）→ 設定 IR、IR_TREE
  IR="$1"; mkdir -p "$IR"
  git -C "$IR" init -q
  git -C "$IR" config user.email t@example.com; git -C "$IR" config user.name test
  echo base > "$IR/README"; git -C "$IR" add README; git -C "$IR" commit -qm base
  local base; base="$(git -C "$IR" rev-parse HEAD)"
  mkdir -p "$IR/scripts/lib" "$IR/python/scripts"
  sed -e "s/^readonly STEP9_BASE=.*/readonly STEP9_BASE=$base/" \
      -e "s/^readonly STEP9_MIN_FREE_BYTES=.*/readonly STEP9_MIN_FREE_BYTES=$2/" \
      "$ENTRY_REAL" > "$IR/scripts/i074-step9-guards.sh"
  if [ "${3:-normal}" = hardlink ]; then  # 模擬「clone 用了預設的 hardlink」：拿掉 --no-hardlinks（必須恰好一處）
    [ "$(grep -c -- ' clone -q --no-hardlinks ' "$IR/scripts/i074-step9-guards.sh")" = 1 ] \
      || { echo "mk_ir：入口裡找不到恰好一處 clone --no-hardlinks" >&2; return 1; }
    sed -i 's/ clone -q --no-hardlinks / clone -q /' "$IR/scripts/i074-step9-guards.sh"
  fi
  cp "$REPO_ROOT/scripts/i074-step9-image.sh" "$REPO_ROOT/scripts/lib/mem-guard.sh" "$IR/scripts/" 2>/dev/null
  mv "$IR/scripts/mem-guard.sh" "$IR/scripts/lib/mem-guard.sh"
  { cat "$REPO_ROOT/scripts/lib/replay-args.sh"; echo 'echo STEP9_TSRC_REPLAY_ARGS >&2'; } > "$IR/scripts/lib/replay-args.sh"
  printf '#!/usr/bin/env bash\necho STEP9_TSRC_GENERATOR\nexit 1\n' > "$IR/scripts/make-i074-tooling-patch.sh"
  printf '%s' "$STUB_BODY" > "$IR/scripts/smoke-replay-offline.sh"
  printf '%s' "$STUB_BODY" > "$IR/python/scripts/test.sh"
  cp "$REPO_ROOT/python/scripts/i074_step9_runs.py" "$IR/python/scripts/"
  chmod +x "$IR"/scripts/*.sh "$IR/python/scripts/test.sh"
  if [ "${3:-normal}" = exportignore ]; then
    echo "ignored.txt export-ignore" > "$IR/.gitattributes"; echo x > "$IR/ignored.txt"
  fi
  git -C "$IR" add -A
  IR_TREE="$(git -C "$IR" write-tree)"
}

echo "==> step9：入口的守門與信任邊界（n1～n9）"
mk_ir "$TD/ir" 1 normal
# 複製的入口只差兩行常數
diff_lines="$(diff "$ENTRY_REAL" "$IR/scripts/i074-step9-guards.sh" || true)"
diff_lines="$(grep '^[<>]' <<< "$diff_lines" || true)"
const_diff="$(grep -c . <<< "$diff_lines" || true)"
other_lines="$(grep -v -e 'readonly STEP9_BASE=' -e 'readonly STEP9_MIN_FREE_BYTES=' <<< "$diff_lines" || true)"
if [ -n "$other_lines" ]; then
  fail "隔離 repo 的入口與正式入口的差異⛔ 不只常數行"
elif [ "$const_diff" = 4 ]; then
  pass "隔離 repo 的入口與正式入口只差 STEP9_BASE、STEP9_MIN_FREE_BYTES 兩行常數"
else fail "隔離 repo 的入口差異行數不符（$const_diff）"; fi

ENV_BASE=(env PATH="$FAKE:$PATH" XDG_DATA_HOME="$TD/xdg")
run_entry() {  # $1＝名稱；其餘＝入口參數。stdout／stderr → $TD/<名稱>.out／.err；回傳結束碼；TMPDIR 專屬
  local name="$1" rc=0; shift
  mkdir -p "$TD/tmp_$name"
  "${ENV_BASE[@]}" FAKE_DOCKER_STATE="$STATE" FAKE_DOCKER_LOG="$LOG" TMPDIR="$TD/tmp_$name" \
    bash "$IR/scripts/i074-step9-guards.sh" "$@" > "$TD/$name.out" 2> "$TD/$name.err" || rc=$?
  return "$rc"
}
h_case entry
WT_BEFORE="$(git -C "$IR" worktree list --porcelain)"

# 參數：只接受 40 碼 tree、--out 不得已存在
rc=0; run_entry argref --source-tree HEAD --out "$TD/o_ref" || rc=$?
rc2=0; run_entry argcommit --source-tree "$(git -C "$IR" rev-parse HEAD)" --out "$TD/o_commit" || rc2=$?
mkdir -p "$TD/o_exists"; rc3=0; run_entry argexists --source-tree "$IR_TREE" --out "$TD/o_exists" || rc3=$?
if [ "$rc" = 1 ] && [ "$rc2" = 1 ] && [ "$rc3" = 1 ] && grep -q '40 碼' "$TD/argref.err" && grep -q '40 碼' "$TD/argcommit.err" \
   && grep -q '已存在' "$TD/argexists.err"; then
  pass "入口只接受 40 碼的 tree（⛔ 不接受 ref、commit）、--out 已存在即拒絕"
else fail "入口的參數守門不符"; fi

# n1：正式工作樹的入口 ≠ T_src 的 blob → 在建 clone 之前拒絕
cp "$IR/scripts/i074-step9-guards.sh" "$TD/entry.bak"; echo '# tampered' >> "$IR/scripts/i074-step9-guards.sh"
rc=0; run_entry n1 --source-tree "$IR_TREE" --out "$TD/o_n1" || rc=$?
cp "$TD/entry.bak" "$IR/scripts/i074-step9-guards.sh"
if [ "$rc" = 1 ] && grep -q '執行中的入口 ≠' "$TD/n1.err" && [ -z "$(ls -A "$TD/tmp_n1")" ] && [ ! -e "$TD/o_n1" ]; then
  pass "n1：正式工作樹的入口 ≠ T_src 的 blob → bootstrap 在建 clone 之前拒絕"
else fail "n1：被改過的入口沒有在 bootstrap 被擋下"; fi

# n2～n5、n6：正式工作樹的 helper／產生器／runner／producer／comparator／fixture 被改成寫 sentinel → ⛔ 不被執行；
#           T_src 的 helper 帶標記 → 標記出現（跑的是 T_src）
SENT="$TD/sentinel_n2"
for f in scripts/lib/replay-args.sh scripts/lib/mem-guard.sh scripts/make-i074-tooling-patch.sh; do
  cp "$IR/$f" "$TD/$(basename "$f").bak"
  printf '#!/usr/bin/env bash\ntouch %s\n' "$SENT" > "$IR/$f"
done
for f in scripts/run-replay-offline.sh python/scripts/i074_decision_summary_producer.py \
         python/scripts/i074_differential_guard.py python/scripts/i074_tooling_guard.py; do
  printf '#!/usr/bin/env bash\ntouch %s\n' "$SENT" > "$IR/$f"
done
echo '{broken' > "$IR/python/scripts/fixtures_dummy.json"
rc=0; run_entry n2 --source-tree "$IR_TREE" --out "$TD/o_n2" || rc=$?
for f in scripts/lib/replay-args.sh scripts/lib/mem-guard.sh scripts/make-i074-tooling-patch.sh; do
  cp "$TD/$(basename "$f").bak" "$IR/$f"
done
rm -f "$IR/scripts/run-replay-offline.sh" "$IR/python/scripts/i074_decision_summary_producer.py" \
      "$IR/python/scripts/i074_differential_guard.py" "$IR/python/scripts/i074_tooling_guard.py" "$IR/python/scripts/fixtures_dummy.json"
if [ ! -e "$SENT" ] && grep -q 'STEP9_TSRC_GENERATOR' "$TD/o_n2/tooling_verify.log" 2>/dev/null; then
  pass "n2～n5：正式工作樹的 replay-args／mem-guard／產生器（與 runner、producer、comparator）被改 → ⛔ 沒有被執行，跑的是 clone 裡的 T_src"
else fail "n2～n5：正式工作樹被改過的檔案被執行了，或沒有跑到 clone 的產生器（rc=$rc）"; fi
if grep -q 'STEP9_TSRC_REPLAY_ARGS' "$TD/n2.err"; then
  pass "n6：T_src 的 lib/replay-args.sh 帶的標記出現（證明 source 的是 T_src 的內容）"
else fail "n6：沒有看到 T_src 的 helper 標記"; fi

# n7：內部參數在 clone 以外出現 → 拒絕
rc=0; run_entry n7 --in-clone --source-tree "$IR_TREE" --out "$TD/o_n7" --work "$TD" --formal-repo "$IR" || rc=$?
if [ "$rc" = 1 ] && grep -q '只能在 bootstrap 建的隔離 clone' "$TD/n7.err"; then
  pass "n7：--in-clone 在 clone 以外 → 拒絕"
else fail "n7：--in-clone 在 clone 以外竟然被接受（rc=$rc）"; fi
rc=0; run_entry n7b --source-tree "$IR_TREE" --in-clone --out "$TD/o_n7b" || rc=$?
[ "$rc" = 1 ] && grep -q -- '--in-clone 只能由 bootstrap' "$TD/n7b.err" \
  && pass "n7：--in-clone 混在一般參數裡 → bootstrap 拒絕" || fail "n7：--in-clone 混在一般參數裡竟然被接受"

# 磁碟不足 → 在建 clone 之前拒絕（另一個以大門檻 sed 的隔離 repo）
IR_SAVE="$IR"; IR_TREE_SAVE="$IR_TREE"
mk_ir "$TD/ir_disk" 999999999999999 normal
rc=0; run_entry disk --source-tree "$IR_TREE" --out "$TD/o_disk" || rc=$?
if [ "$rc" = 1 ] && grep -q '可用空間' "$TD/disk.err" && [ -z "$(ls -A "$TD/tmp_disk")" ]; then
  pass "可用空間不足 → 在建 clone 之前拒絕（T-072）"
else fail "可用空間不足的守門不符（rc=$rc）"; fi

# n8：clone 的 tree ≠ T_src（export-ignore 讓 archive 少一個檔）→ 拒絕
mk_ir "$TD/ir_export" 1 exportignore
rc=0; run_entry n8 --source-tree "$IR_TREE" --out "$TD/o_n8" || rc=$?
if [ "$rc" = 1 ] && grep -q '隔離 clone 的 tree' "$TD/n8.err"; then
  pass "n8：clone 的 tree ≠ T_src（checkout 不完整）→ 拒絕"
else fail "n8：不完整的 clone 竟然被接受（rc=$rc）"; fi
IR="$IR_SAVE"; IR_TREE="$IR_TREE_SAVE"

if [ "$(git -C "$IR" worktree list --porcelain)" = "$WT_BEFORE" ] && ! grep -q 'i074.stage2.run' "$LOG"; then
  pass "隔離 repo 的 worktree 登記前後不變、fake docker 的紀錄裡⛔ 沒有 i074.stage2.run label"
else fail "隔離 repo 留下了 worktree 登記，或出現了正式 label"; fi

echo "==> step9：--s7 兩個模式（s1～s15）"
s7_run() {  # $1＝名稱 $2＝模式；其餘＝環境變數（KEY=VALUE）。設定 S7_RC、S7_OUT
  local name="$1" mode="$2"; shift 2
  h_case "s7_$name"
  S7_OUT="$TD/o_$name"; S7_RC=0
  mkdir -p "$TD/tmp_$name"
  env PATH="$FAKE:$PATH" XDG_DATA_HOME="$TD/xdg" FAKE_DOCKER_STATE="$STATE" FAKE_DOCKER_LOG="$LOG" \
    TMPDIR="$TD/tmp_$name" "$@" bash "$IR/scripts/i074-step9-guards.sh" --source-tree "$IR_TREE" --out "$S7_OUT" \
    --s7 "$mode" > "$TD/$name.out" 2> "$TD/$name.err" || S7_RC=$?
}
status_of() {  # $1＝欄位（release／verify）→ cleanup_status.json 裡唯一一筆 tag 的值
  python3 -I -B -c 'import json,sys; d=json.load(open(sys.argv[1])); print(next(iter(d.values()))[sys.argv[2]])' \
    "$S7_OUT/cleanup_status.json" "$1" 2>/dev/null || echo "-"
}
image_back() { [ "$(state_sorted)" = "$INITIAL_SORTED" ]; }

s7_run s1 smoke STUB_RC=0
if [ "$S7_RC" = 0 ] && image_back && [ "$(status_of release)" = ok ] && [ "$(status_of verify)" = ok ] \
   && [ -f "$S7_OUT/step9_manifest.json" ] && grep -q 'STEP9_STUB_RAN' "$S7_OUT/smoke.log"; then
  pass "s1：成功 → release、verify 都執行、rc 0、image 庫回到建立前"
else fail "s1：成功路徑不符（rc=$S7_RC）"; tail -5 "$TD/s1.err" >&2; fi
s7_run s2 smoke STUB_RC=3
[ "$S7_RC" = 3 ] && image_back && [ "$(status_of verify)" = ok ] \
  && pass "s2：被執行的腳本失敗 → release、verify 照樣執行、rc 原樣傳出（3）" || fail "s2：失敗路徑不符（rc=$S7_RC）"
s7_run s5 smoke STUB_BUILD=0
[ "$S7_RC" = 0 ] && ! grep -q 'image rm' "$LOG" && [ "$(status_of release)" = ok ] \
  && pass "s5：build 之前就失敗（tag 不存在）→ release 不做事" || fail "s5：tag 不存在時不符（rc=$S7_RC）"

s7_signal() {  # $1＝名稱 $2＝signal（INT／TERM）；其餘＝環境變數
  local name="$1" sig="$2" token="tok$RANDOM$RANDOM" started="$TD/started_$1" pid i=0; shift 2
  h_case "s7_$name"
  S7_OUT="$TD/o_$name"; S7_RC=0; S7_TOKEN="$token"
  mkdir -p "$TD/tmp_$name"
  env PATH="$FAKE:$PATH" XDG_DATA_HOME="$TD/xdg" FAKE_DOCKER_STATE="$STATE" FAKE_DOCKER_LOG="$LOG" \
    TMPDIR="$TD/tmp_$name" STUB_STARTED="$started" STUB_TOKEN="$token" "$@" \
    python3 -I -c 'import os, signal, sys; signal.signal(signal.SIGINT, signal.SIG_DFL); os.execvp(sys.argv[1], sys.argv[1:])' \
    bash "$IR/scripts/i074-step9-guards.sh" --source-tree "$IR_TREE" --out "$S7_OUT" --s7 smoke \
    > "$TD/$name.out" 2> "$TD/$name.err" &
  pid=$!
  while [ ! -e "$started" ] && [ "$i" -lt 150 ]; do sleep 0.2; i=$((i + 1)); done
  # 入口可能在 stub 啟動之前就結束（例如被注回的缺陷讓它提早失敗）：kill 失敗⛔ 不中止整支腳本，交給斷言回報 FAIL
  kill "-$sig" "$pid" 2>/dev/null || true
  wait "$pid" || S7_RC=$?
}
no_token_alive() { local ps_out; ps_out="$(ps -eo args)"; ! grep -qF "step9-stub-$S7_TOKEN" <<< "$ps_out"; }
s7_signal s3 INT
[ "$S7_RC" = 130 ] && image_back && [ "$(status_of verify)" = ok ] && no_token_alive \
  && pass "s3：執行途中 INT → trap 執行 release、verify，rc 130、image 庫回到建立前、⛔ 沒有存活的 stub" \
  || fail "s3：INT 的處理不符（rc=$S7_RC）"
s7_signal s4 TERM
[ "$S7_RC" = 143 ] && image_back && [ "$(status_of verify)" = ok ] && no_token_alive \
  && pass "s4：執行途中 TERM → trap 執行 release、verify，rc 143、⛔ 沒有存活的 stub" \
  || fail "s4：TERM 的處理不符（rc=$S7_RC）"

# s6、s7：--s7 full 執行的是 clone 裡的 test.sh（正式工作樹被改，執行前或執行途中都一樣）
SENT6="$TD/sentinel_s6"
cp "$IR/python/scripts/test.sh" "$TD/testsh.bak"; printf '#!/usr/bin/env bash\ntouch %s\n' "$SENT6" > "$IR/python/scripts/test.sh"
s7_run s6 full STUB_RC=0
cp "$TD/testsh.bak" "$IR/python/scripts/test.sh"
TD_REAL="$(cd "$TD" && pwd -P)"
ran_in_clone() {  # $1＝名稱：實際執行的 test.sh 在該次入口的隔離 clone 之內（⛔ 不是正式工作樹的那一份）
  grep -q "^STEP9_STUB_RAN $TD_REAL/tmp_$1/i074-step9\.[^/]*/clone/python/scripts/test.sh\$" "$S7_OUT/full.log"
}
[ ! -e "$SENT6" ] && ran_in_clone s6 \
  && pass "s6：--s7 full 開始之前改了正式工作樹的 test.sh → ⛔ 沒有被執行（執行的是 clone 裡的 T_src）" \
  || fail "s6：正式工作樹被改過的 test.sh 被執行了"
SENT7="$TD/sentinel_s7"
s7_run s7 full STUB_RC=0 STUB_MODIFY_FORMAL="$IR/python/scripts/test.sh" STUB_SENTINEL="$SENT7"
[ ! -e "$SENT7" ] && cmp -s "$TD/testsh.bak" "$IR/python/scripts/test.sh" && ran_in_clone s7 \
  && pass "s7：--s7 full 執行途中改了正式工作樹的 test.sh、結束前恢復 → ⛔ 沒有被執行、結果不受影響" \
  || fail "s7：執行途中被改的正式工作樹檔案被執行了"

# s8：I-118 的 ＋4 歸因（之後的逐項比對在隔離 repo 沒有 baseline，必然失敗——這裡只看歸因那一步）
s7_run s8a full STUB_WORKTREES=4
if [ "$S7_RC" = 1 ] && ! grep -q '恰好 4 筆' "$TD/s8a.err" && grep -q '逐項比對' "$TD/s8a.err" \
   && [ "$(grep -c . "$S7_OUT/full_added_worktrees.txt")" = 4 ]; then
  pass "s8：clone 的專屬 TMPDIR 下恰好 4 個 worktree → 歸因通過（之後才到逐項比對）"
else fail "s8：4 個 worktree 的歸因不符"; fi
s7_run s8b full STUB_WORKTREES=5
grep -q '恰好 4 筆' "$TD/s8b.err" && pass "s8：5 個 worktree → 歸因失敗" || fail "s8：5 個 worktree 竟然歸因通過"
s7_run s8c full STUB_WORKTREES=3 STUB_OUTSIDE_WT=1
grep -q '恰好 4 筆' "$TD/s8c.err" && pass "s8：其中一個在專屬 TMPDIR 以外 → 歸因失敗" || fail "s8：TMPDIR 以外的 worktree 竟然歸因通過"
if [ "$(git -C "$IR" worktree list --porcelain)" = "$WT_BEFORE" ]; then
  pass "s8：正式 repo（隔離 repo）的 worktree 登記⛔ 沒有改變（＋4 落在 clone 內）"
else fail "s8：正式 repo 的 worktree 登記改變了"; fi

# s9／n9：正式工作樹的 image helper 與 smoke 腳本被改 → ⛔ 沒有被執行
SENT9="$TD/sentinel_s9"
cp "$IR/scripts/i074-step9-image.sh" "$TD/image.bak"; cp "$IR/scripts/smoke-replay-offline.sh" "$TD/smoke.bak"
printf '#!/usr/bin/env bash\ntouch %s\n' "$SENT9" | tee "$IR/scripts/i074-step9-image.sh" > "$IR/scripts/smoke-replay-offline.sh"
s7_run s9 smoke STUB_RC=0
cp "$TD/image.bak" "$IR/scripts/i074-step9-image.sh"; cp "$TD/smoke.bak" "$IR/scripts/smoke-replay-offline.sh"
[ ! -e "$SENT9" ] && [ "$S7_RC" = 0 ] \
  && pass "s9／n9：正式工作樹的 i074-step9-image.sh 與 smoke 腳本被改 → ⛔ 沒有被執行（reserve／release／verify 照樣成功）" \
  || fail "s9／n9：正式工作樹被改過的 helper 或 smoke 腳本被執行了（rc=$S7_RC）"

# s10～s14：清理失敗的結束碼仲裁
s7_run s10 smoke STUB_RC=0 FAKE_RM_FAIL=1
[ "$S7_RC" = 1 ] && [ "$(status_of release)" = failed ] && [ "$(status_of verify)" = failed ] \
  && pass "s10：主流程成功但 release 失敗 → 回 1、verify 仍有執行" || fail "s10：release 失敗的仲裁不符（rc=$S7_RC）"
s7_run s11 smoke STUB_RC=0 STUB_EXTRA_IMAGE=sha256:eee
[ "$S7_RC" = 1 ] && [ "$(status_of release)" = ok ] && [ "$(status_of verify)" = failed ] \
  && pass "s11：主流程成功但 verify 失敗（多一個未登記的 image）→ 回 1" || fail "s11：verify 失敗的仲裁不符（rc=$S7_RC）"
s7_run s12 smoke STUB_RC=3 FAKE_RM_FAIL=1
[ "$S7_RC" = 3 ] && [ "$(status_of release)" = failed ] \
  && pass "s12：主流程失敗（3）且 release 失敗 → 保留 3、清理失敗有紀錄" || fail "s12：仲裁不符（rc=$S7_RC）"
s7_signal s13 TERM FAKE_RM_FAIL=1
[ "$S7_RC" = 143 ] && [ "$(status_of release)" = failed ] && [ "$(status_of verify)" = failed ] && no_token_alive \
  && pass "s13：TERM 且 release 失敗 → 保留 143、verify 仍有執行" || fail "s13：仲裁不符（rc=$S7_RC）"
s7_run s14a smoke STUB_RC=0 STUB_STATUS_BLOCK="$TD/o_s14a/cleanup_status.json"
[ "$S7_RC" = 1 ] && grep -q 'cleanup_status.json 寫入失敗' "$TD/s14a.err" \
  && pass "s14：主流程與清理都成功、但 cleanup_status.json 寫不進去 → 回 1、stderr 有訊息" \
  || fail "s14：狀態檔寫入失敗的仲裁不符（rc=$S7_RC）"
s7_run s14b smoke STUB_RC=3 STUB_STATUS_BLOCK="$TD/o_s14b/cleanup_status.json"
[ "$S7_RC" = 3 ] && grep -q 'cleanup_status.json 寫入失敗' "$TD/s14b.err" \
  && pass "s14：同上但主流程失敗（3）→ 保留 3、stderr 有訊息" || fail "s14：仲裁不符（rc=$S7_RC）"
# s15：smoke 的 worktree 清理失敗（stub 在專屬 TMPDIR 建一個 worktree 並 lock，`worktree remove --force` 移除不了）
s7_run s15a smoke STUB_RC=0 STUB_WORKTREES=1 STUB_LOCK_WT=1
[ "$S7_RC" = 1 ] && grep -q '移除 worktree 登記失敗' "$TD/s15a.err" \
  && [ "$(status_of release)" = ok ] && [ "$(status_of verify)" = ok ] \
  && pass "s15：smoke 成功但 worktree 清理失敗 → 回 1（image 的 release、verify 照樣執行）" \
  || fail "s15：smoke 成功但 worktree 清理失敗的仲裁不符（rc=$S7_RC）"
s7_run s15b smoke STUB_RC=3 STUB_WORKTREES=1 STUB_LOCK_WT=1
[ "$S7_RC" = 3 ] && grep -q '移除 worktree 登記失敗' "$TD/s15b.err" \
  && pass "s15：smoke 失敗（3）且 worktree 清理失敗 → 保留 3、stderr 有訊息" || fail "s15：仲裁不符（rc=$S7_RC）"

echo "==> step9：隔離 clone 與正式 repo 的物件庫（n10～n14）"
# ⚠️ hardlink 的 clone 會讓 clone 裡的 git 寫入（add、write-tree）freshen 到正式 repo 的物件檔（只改 mtime）。
objects_inv() { find "$IR/.git/objects" -type f -printf '%P %i %s %T@ %n\n' | LC_ALL=C sort; }
OBJ_BEFORE="$(objects_inv)"
s7_run n10 smoke STUB_RC=0
OBJ_AFTER="$(objects_inv)"
if [ "$S7_RC" = 0 ] && [ "$OBJ_BEFORE" = "$OBJ_AFTER" ]; then
  pass "n10：正式 repo 物件庫的 inode／大小／mtime／link 數前後完全相同（隔離 clone ⛔ 不共用物件檔）"
else fail "n10：正式 repo 的物件庫前後改變了（rc=$S7_RC）"; fi
if [ -s "$S7_OUT/formal_objects_before.txt" ] && cmp -s "$S7_OUT/formal_objects_before.txt" "$S7_OUT/formal_objects_after.txt"; then
  pass "n10：入口留下正式 repo 物件庫前後的 inventory，而且兩份相同"
else fail "n10：入口沒有留下物件庫的 inventory，或兩份不同"; fi

IR_SAVE="$IR"; IR_TREE_SAVE="$IR_TREE"
if mk_ir "$TD/ir_hl" 1 hardlink; then
  rc=0; run_entry n11 --source-tree "$IR_TREE" --out "$TD/o_n11" || rc=$?
  if [ "$rc" = 1 ] && grep -q '共用 inode' "$TD/n11.err" && [ ! -e "$TD/o_n11/formal_before.txt" ] \
     && [ -z "$(ls -A "$TD/tmp_n11")" ]; then
    pass "n11：clone 的物件檔與正式 repo 共用 inode（hardlink）→ 在執行任何步驟之前拒絕"
  else fail "n11：hardlink 的 clone 竟然被接受（rc=$rc）"; fi
else fail "n11：建不了 hardlink 版的隔離 repo"; fi
IR="$IR_SAVE"; IR_TREE="$IR_TREE_SAVE"

OBJ_FILE="$(find "$IR/.git/objects" -type f -path '*/objects/[0-9a-f][0-9a-f]/*' -print -quit)"
s7_run n12a smoke STUB_RC=0 STUB_TOUCH_FORMAL="$OBJ_FILE"
[ "$S7_RC" = 1 ] && grep -q '物件庫前後不同' "$TD/n12a.err" \
  && pass "n12：執行途中正式 repo 的物件檔被 touch（模擬 freshen）→ 回 1" || fail "n12：物件庫的改變沒有被抓到（rc=$S7_RC）"
s7_run n12b smoke STUB_RC=3 STUB_TOUCH_FORMAL="$OBJ_FILE"
[ "$S7_RC" = 3 ] && grep -q '物件庫前後不同' "$TD/n12b.err" \
  && pass "n12：同上但主流程失敗（3）→ 保留 3、stderr 有訊息" || fail "n12：仲裁不符（rc=$S7_RC）"

# fake df：第一次（bootstrap）回報 FAKE_DF_FIRST（預設很大）、之後（clone 建好之後）回報 0；隔離 repo 的門檻 sed 成 1
mkdir -p "$TD/bin_df"
cat > "$TD/bin_df/df" <<'DFEOF'
#!/usr/bin/env bash
if [ -e "$FAKE_DF_MARK" ]; then n=0; else n="${FAKE_DF_FIRST:-999999999999999}"; : > "$FAKE_DF_MARK"; fi
printf '     Avail\n%s\n' "$n"
DFEOF
chmod +x "$TD/bin_df/df"
# n13：建 clone 之前夠、建好之後⛔ 不夠 → 在執行任何步驟之前拒絕
s7_run n13 smoke STUB_RC=0 PATH="$TD/bin_df:$FAKE:$PATH" FAKE_DF_MARK="$TD/df_mark_n13"
if [ "$S7_RC" = 1 ] && grep -q '建好隔離 clone 之後' "$TD/n13.err" && [ ! -e "$S7_OUT/smoke.log" ] \
   && [ -z "$(ls -A "$TD/tmp_n13")" ]; then
  pass "n13：建好隔離 clone 之後可用空間不足 → 在執行任何步驟之前拒絕（T-072）"
else fail "n13：建好 clone 之後的空間檢查不符（rc=$S7_RC）"; fi
# n14：只夠執行期間的下限（門檻 1 ＋ 1 byte）、⛔ 不夠 clone 本身 → bootstrap 在建 clone 之前拒絕
s7_run n14 smoke STUB_RC=0 PATH="$TD/bin_df:$FAKE:$PATH" FAKE_DF_MARK="$TD/df_mark_n14" FAKE_DF_FIRST=2
if [ "$S7_RC" = 1 ] && grep -q '隔離 clone 的物件庫' "$TD/n14.err" && [ ! -e "$S7_OUT" ] \
   && [ -z "$(ls -A "$TD/tmp_n14")" ]; then
  pass "n14：可用空間只夠執行期間的下限、⛔ 不夠隔離 clone 本身 → bootstrap 在建 clone 之前拒絕"
else fail "n14：bootstrap 的門檻沒有把隔離 clone 的大小算進去（rc=$S7_RC）"; fi

if [ -z "$(ls -A "$TD/xdg" 2>/dev/null)" ]; then pass "⛔ 沒有寫進 XDG（identity 只在 S4 唯讀讀取）"; else fail "測試寫進了 XDG"; fi

if [ "$fails" -ne 0 ]; then
  echo "==> step9 測試失敗：$fails 項" >&2
  exit 1
fi
echo "==> step9 測試全部通過"
