#!/usr/bin/env bash
# scripts/check-doc-refs.py 的迴歸測試。
#
# ⚠️ **守門工具自己要有測試**——2026-09-17 的第一版 checker 就 **fail-open**：
# 只檢查 17／142 個引用卻印「全部對得上」，識別符整份檔案都不存在時反而靜默通過。
# ⛔ 沒有這支測試的話，那種漏洞只能靠人 review 發現。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CHECK="$REPO_ROOT/scripts/check-doc-refs.py"
PASS=0; FAIL=0
pass() { PASS=$((PASS+1)); echo "  ok   $1"; }
fail() { FAIL=$((FAIL+1)); echo "  FAIL $1" >&2; }

TD="$(mktemp -d)"
trap 'rm -rf "$TD"' EXIT
mkdir -p "$TD/docs" "$TD/pkg/a" "$TD/pkg/b"

cat > "$TD/pkg/a/sample.py" <<'PY'
import os


def alpha():
    """第 4 行是定義。"""
    inner_marker = 1
    return inner_marker


    # filler 0
    # filler 1
    # filler 2
    # filler 3
    # filler 4
    # filler 5
    # filler 6
    # filler 7
    # filler 8
    # filler 9
    # filler 10
    # filler 11
    # filler 12
    # filler 13
    # filler 14
    # filler 15
    # filler 16
    # filler 17
    # filler 18
    # filler 19
    # filler 20
    # filler 21
    # filler 22
    # filler 23
    # filler 24
    # filler 25
    # filler 26
    # filler 27
    # filler 28
    # filler 29
    # filler 30
    # filler 31
    # filler 32
    # filler 33
    # filler 34
    # filler 35
    # filler 36
    # filler 37
    # filler 38
    # filler 39

def beta():
    return 2
PY
cat > "$TD/pkg/a/dup.go" <<'GO'
package a

func Dup() {}
GO
cp "$TD/pkg/a/dup.go" "$TD/pkg/b/dup.go"

# ⚠️ **訊息與結束碼都要驗**（2026-09-17 review）：初版用 `run || true` 吃掉 rc、
# 只 grep 訊息，於是「印了錯誤卻回 0」這種 fail-open ⛔ 測不出來。
# ⛔ **呼叫端⛔ 不可再包一層 `$( )`**（例如 `out="$(run)"`）：那會讓 run 跑在
# **子 shell** 裡，`RC=$?` 設的是子 shell 的變數、外層⛔ 讀不到，
# 於是「印了錯誤卻回 0」照樣測不出來——正是本測試要防的 fail-open。
# ⚠️ **run 內部**用 `OUT="$(…)"` 則是安全的（見函式內註解）。
RC=0
OUT=""
run() {
  set +e
  # ⚠️ **純賦值⛔ 不會掩蓋 rc**（`v="$(exit 7)"` → `$?` 是 7）——會掩蓋的是
  # `export v="$(…)"`，因為 `export` 這個 builtin 自己的 rc 蓋過去了。
  # ⛔ 所以這裡⛔ 不需要繞道檔案，少一次寫讀就少一個不穩定來源。
  OUT="$(python3 "$CHECK" --root "$TD" "$@" 2>&1)"
  RC=$?
  set -e
}

# fixture 的 legacy 基準線（預設空的——大多數 case 用不到）
mkdir -p "$TD/scripts"
: > "$TD/scripts/doc-refs-legacy.txt"

# ① 正確引用（定義行）
printf '`pkg/a/sample.py:4` `alpha`\n' > "$TD/docs/x.md"
run >/dev/null; [ "$RC" -eq 0 ] && pass "定義行引用 → rc=0" || fail "定義行引用 rc=$RC"

# ② 函式內部某行也算對
printf '`pkg/a/sample.py:6` `alpha`\n' > "$TD/docs/x.md"
run >/dev/null; [ "$RC" -eq 0 ] && pass "函式內部行 → rc=0" || fail "函式內部行 rc=$RC"

# ③ 漂移：指到別的函式
printf '`pkg/a/sample.py:51` `alpha`\n' > "$TD/docs/x.md"
run >/dev/null
[ "$RC" -eq 1 ] && pass "漂移引用 → rc=1" || fail "漂移引用 rc=$RC（預期 1）"

# ④ 識別符已被刪除（⛔ 初版在這裡 fail-open）
printf '`pkg/a/sample.py:4` `deleted_func`\n' > "$TD/docs/x.md"
run
{ [ "$RC" -eq 1 ] && grep -q "已不在這個檔案裡" <<< "$OUT"; } \
  && pass "識別符已刪除 → rc=1 且訊息明確" || fail "識別符已刪除：rc=$RC $OUT"

# ⑤ 同 basename 歧義 → 失敗
printf '`dup.go:3` `Dup`\n' > "$TD/docs/x.md"
run
{ [ "$RC" -eq 1 ] && grep -q "同名檔案" <<< "$OUT"; } \
  && pass "同名檔案歧義 → rc=1" || fail "同名歧義：rc=$RC $OUT"

# ⑥ repo-relative path 解掉歧義
printf '`pkg/a/dup.go:3` `Dup`\n' > "$TD/docs/x.md"
run >/dev/null; [ "$RC" -eq 0 ] && pass "帶路徑 → 歧義解除" || fail "帶路徑 rc=$RC"

# ⑦ repo 內不存在且不在 allowlist → ⛔ 失敗（⛔ 不得當成外部套件放行）
printf '`vendor/lib/date.go:164` `Parse`\n' > "$TD/docs/x.md"
run
{ [ "$RC" -eq 1 ] && grep -q "repo 內找不到這個檔案" <<< "$OUT"; } \
  && pass "repo 外檔案且不在 allowlist → rc=1" || fail "⑦ repo 外檔案：rc=$RC $OUT"

# ⑧ 行號超出檔案長度
printf '`pkg/a/sample.py:9999` `alpha`\n' > "$TD/docs/x.md"
run
{ [ "$RC" -eq 1 ] && grep -q "行號超出檔案長度" <<< "$OUT"; } \
  && pass "行號超出 → rc=1" || fail "行號超出：rc=$RC $OUT"

# ⑨ 四種合法的緊鄰格式都要被辨識
for form in '`pkg/a/sample.py:4` `alpha`' \
            '`pkg/a/sample.py:4` 的 `alpha`' \
            '`alpha`（`pkg/a/sample.py:4`）' \
            '`alpha()`（`pkg/a/sample.py:4`）'; do
  printf '%s\n' "$form" > "$TD/docs/x.md"
  run
  grep -q "識別符已驗 1" <<< "$OUT" \
    && pass "合法格式：$form" || fail "格式未被辨識：$form（$OUT）"
done

# ⑩ 未支援格式且不在基準線 → ⛔ 失敗（⛔ 不得靜默略過，也⛔ 不得只計數就放行）
printf '這一行提到 `pkg/a/sample.py:4` 但識別符 `alpha` 離得很遠很遠很遠很遠很遠很遠很遠\n' > "$TD/docs/x.md"
run
{ [ "$RC" -eq 1 ] && grep -q "新的 line-only" <<< "$OUT"; } \
  && pass "非緊鄰格式 → rc=1（⛔ 不只計數放行）" || fail "非緊鄰格式：rc=$RC $OUT"

# ⑪ 總數必須涵蓋每一個引用
printf '`pkg/a/sample.py:4` `alpha`\n`vendor/x/y.go:9` `Z`\n`pkg/a/sample.py:7`\n' > "$TD/docs/x.md"
run
grep -q "引用共 3 個" <<< "$OUT" \
  && pass "總數涵蓋每一個引用" || fail "總數不符：$OUT"

# ⑫-a repo 內找不到且不在 allowlist → ⛔ 失敗（⛔ 不得當成外部套件放行）
printf '`pkg/a/typo_name.py:3` `alpha`\n' > "$TD/docs/x.md"
run
{ [ "$RC" -eq 1 ] && grep -q "repo 內找不到這個檔案" <<< "$OUT"; } \
  && pass "未知檔案 → rc=1（⛔ 不 fail-open）" || fail "未知檔案：rc=$RC $OUT"

# ⑫-b allowlist 內的外部引用 → 略過
printf '`connection.go:262` `Foo`\n' > "$TD/docs/x.md"
run
{ [ "$RC" -eq 0 ] && grep -q "外部套件略過 1" <<< "$OUT"; } \
  && pass "allowlist 內 → 略過" || fail "allowlist：rc=$RC $OUT"

# ⑫-c 區間終點超出檔案長度 → 失敗
printf '`pkg/a/sample.py:4-9999` `alpha`\n' > "$TD/docs/x.md"
run
{ [ "$RC" -eq 1 ] && grep -q "區間終點" <<< "$OUT"; } \
  && pass "區間終點超出 → rc=1" || fail "區間終點：rc=$RC $OUT"

# ⑫-d 區間終點小於起點 → 失敗
printf '`pkg/a/sample.py:10-4` `alpha`\n' > "$TD/docs/x.md"
run
{ [ "$RC" -eq 1 ] && grep -q "小於起點" <<< "$OUT"; } \
  && pass "區間終點 < 起點 → rc=1" || fail "區間倒置：rc=$RC $OUT"

# ⑫-e 新的 line-only 引用不在基準線 → 失敗
printf '這裡只提 `pkg/a/sample.py:4` 沒有緊鄰識別符\n' > "$TD/docs/x.md"
run
{ [ "$RC" -eq 1 ] && grep -q "新的 line-only" <<< "$OUT"; } \
  && pass "新 line-only 不在基準線 → rc=1" || fail "新 line-only：rc=$RC $OUT"

# ⑫-f 在基準線內 → 通過
printf 'x.md|pkg/a/sample.py:4\n' > "$TD/scripts/doc-refs-legacy.txt"
run >/dev/null
[ "$RC" -eq 0 ] && pass "line-only 在基準線內 → rc=0" || fail "基準線內 rc=$RC"

# ⑫-g 基準線有已消失的項目 → 提示（⛔ 只准變短）
printf 'x.md|pkg/a/sample.py:4\nx.md|pkg/a/sample.py:7\n' > "$TD/scripts/doc-refs-legacy.txt"
run
{ [ "$RC" -eq 1 ] && grep -q "已不存在" <<< "$OUT"; } \
  && pass "基準線殘留 → rc=1（⛔ 不是只警告）" || fail "基準線殘留：rc=$RC $OUT"
: > "$TD/scripts/doc-refs-legacy.txt"

# ⑫-g2 同一個 key 超出基準線的**數量** → 失敗（⛔ 不能只比 key）
printf 'x.md|pkg/a/sample.py:4 x2\n' > "$TD/scripts/doc-refs-legacy.txt"
printf '只提 `pkg/a/sample.py:4` 一次\n只提 `pkg/a/sample.py:4` 兩次\n' > "$TD/docs/x.md"
run
[ "$RC" -eq 0 ] && pass "同 key 剛好用滿基準線額度 → rc=0" || fail "用滿額度 rc=$RC：$OUT"

printf '只提 `pkg/a/sample.py:4` 一次\n只提 `pkg/a/sample.py:4` 兩次\n只提 `pkg/a/sample.py:4` 三次\n' > "$TD/docs/x.md"
run
{ [ "$RC" -eq 1 ] && grep -q "第 3 筆" <<< "$OUT"; } \
  && pass "同 key 新增第 3 筆超出額度 → rc=1" || fail "超出額度：rc=$RC $OUT"
: > "$TD/scripts/doc-refs-legacy.txt"

# ⑫-g3 同一行的兩個不同區間都要各自驗（⛔ 去重不得吃掉第二筆）
printf '`pkg/a/sample.py:4-8` `alpha` 與 `pkg/a/sample.py:4-9999` `alpha`\n' > "$TD/docs/x.md"
run
{ [ "$RC" -eq 1 ] && grep -q "區間終點 9999" <<< "$OUT"; } \
  && pass "同一行兩個區間 → 第二筆仍被驗" || fail "同行區間被去重吃掉：rc=$RC $OUT"

# ⑫-g4 相同起點、不同終點、**不同識別符** → 各自綁各自的識別符
#      （⛔ 第二筆不得沿用第一筆的識別符而靜默通過）
printf '`pkg/a/sample.py:4-8` `alpha` 與 `pkg/a/sample.py:4-9` `deleted_func`\n' > "$TD/docs/x.md"
run
{ [ "$RC" -eq 1 ] && grep -q "deleted_func" <<< "$OUT"; } \
  && pass "同起點不同終點 → 識別符各自綁定" || fail "識別符被共用：rc=$RC $OUT"

# ⑫-g4b **座標完全相同、識別符不同** → 第二個識別符也要各自驗
#       （⛔ 不得沿用第一個而靜默通過；這是 (file,start,end) 當 key 仍抓不到的情況）
printf '`pkg/a/sample.py:4-8` `alpha` 與 `pkg/a/sample.py:4-8` `deleted_func`\n' > "$TD/docs/x.md"
run
{ [ "$RC" -eq 1 ] && grep -q "deleted_func" <<< "$OUT"; } \
  && pass "座標相同、識別符不同 → 各自驗" || fail "識別符被共用：rc=$RC $OUT"

# ⑫-g4c 反向：座標相同、兩個識別符都合法 → 通過
printf '`pkg/a/sample.py:4-8` `alpha` 與 `pkg/a/sample.py:4-8` `alpha`\n' > "$TD/docs/x.md"
run
[ "$RC" -eq 0 ] && pass "座標相同、識別符都合法 → rc=0" || fail "誤判：rc=$RC $OUT"

# ⑫-g5 同一行寫兩次**完全相同**的 line-only 引用 → 兩筆都要計數
printf 'x.md|pkg/a/sample.py:4\n' > "$TD/scripts/doc-refs-legacy.txt"
printf '提兩次 `pkg/a/sample.py:4` 和 `pkg/a/sample.py:4`\n' > "$TD/docs/x.md"
run
{ [ "$RC" -eq 1 ] && grep -q "第 2 筆" <<< "$OUT"; } \
  && pass "同行重複的 line-only → 各自計數（⛔ 不去重）" || fail "重複引用被去重：rc=$RC $OUT"
: > "$TD/scripts/doc-refs-legacy.txt"

# ⑫-g6 基準線額度不合法 → rc=2
printf '`pkg/a/sample.py:4` `alpha`\n' > "$TD/docs/x.md"
for bad in 'x.md|pkg/a/sample.py:4 x0' 'x.md|pkg/a/sample.py:4 x-1' 'x.md|pkg/a/sample.py:4 xabc'; do
  printf '%s\n' "$bad" > "$TD/scripts/doc-refs-legacy.txt"
  run
  [ "$RC" -eq 2 ] && pass "基準線額度不合法（$bad）→ rc=2" || fail "額度 $bad：rc=$RC $OUT"
done
: > "$TD/scripts/doc-refs-legacy.txt"

# ⑫-i walker：`.git` 必須在**遞迴之前**就被剪掉
mkdir -p "$TD/.git/worktrees/fake"
cp "$TD/pkg/a/sample.py" "$TD/.git/worktrees/fake/sample.py"
printf '`sample.py:4` `alpha`\n' > "$TD/docs/x.md"
run
# ⚠️ `.git` 底下那份若被掃到，就會變成「同名檔案有 2 個」而報 ambiguous。
# ⚠️ **rc 也要驗**：只看「沒有同名檔案」的話，checker crash 或回 missing 時⛔ 照樣印 ok。
{ [ "$RC" -eq 0 ] && ! grep -q "同名檔案" <<< "$OUT"; } \
  && pass ".git 在遞迴前被剪掉（rc=0 且無 ambiguous）" \
  || fail ".git 剪枝：rc=$RC $OUT"
rm -rf "$TD/.git"

# ⑫-j walker：⛔ 不得跟隨 symlink 走到 repo 外
OUTSIDE="$(mktemp -d)"
mkdir -p "$OUTSIDE/pkg/a"
cp "$TD/pkg/a/sample.py" "$OUTSIDE/pkg/a/sample.py"
ln -s "$OUTSIDE" "$TD/linked_out"
printf '`sample.py:4` `alpha`\n' > "$TD/docs/x.md"
run
{ [ "$RC" -eq 0 ] && ! grep -q "同名檔案" <<< "$OUT"; } \
  && pass "⛔ 不跟隨 symlink 走出 repo（rc=0 且無 ambiguous）" \
  || fail "symlink：rc=$RC $OUT"
rm -f "$TD/linked_out"; rm -rf "$OUTSIDE"

# ⑫-k walker：symlink cycle ⛔ 不得讓走訪打轉
ln -s "$TD" "$TD/pkg/loop"
printf '`pkg/a/sample.py:4` `alpha`\n' > "$TD/docs/x.md"
run
[ "$RC" -eq 0 ] && pass "symlink cycle ⛔ 不造成打轉" || fail "cycle：rc=$RC $OUT"
rm -f "$TD/pkg/loop"

# ⑫-l 直接路徑⛔ 不得繞過 walker 的防線（symlink／.git／..）
OUTSIDE2="$(mktemp -d)"; mkdir -p "$OUTSIDE2/pkg/a"
cp "$TD/pkg/a/sample.py" "$OUTSIDE2/pkg/a/sample.py"
ln -s "$OUTSIDE2" "$TD/linked2"
#   (a) 經目錄 symlink 的直接路徑 → 必須當成 repo 內找不到
printf '`linked2/pkg/a/sample.py:4` `alpha`\n' > "$TD/docs/x.md"
run
{ [ "$RC" -eq 1 ] && grep -q "repo 內找不到這個檔案" <<< "$OUT"; } \
  && pass "直接路徑經 symlink → 視為 repo 外（⛔ 不繞過防線）" \
  || fail "symlink 直接路徑被接受：rc=$RC $OUT"
rm -f "$TD/linked2"; rm -rf "$OUTSIDE2"
#   (b) `.git` 底下的直接路徑 → 必須被剪枝規則擋下
mkdir -p "$TD/.git/x"; cp "$TD/pkg/a/sample.py" "$TD/.git/x/sample.py"
printf '`.git/x/sample.py:4` `alpha`\n' > "$TD/docs/x.md"
run
{ [ "$RC" -eq 1 ] && grep -q "repo 內找不到這個檔案" <<< "$OUT"; } \
  && pass ".git 底下的直接路徑被擋" || fail ".git 直接路徑被接受：rc=$RC $OUT"
rm -rf "$TD/.git"
#   (c) `..` 指向**真實存在**的 repo 外檔案 → 必須被擋
#       ⚠️ 舊版用不存在的路徑，裸 `is_file()` 也會拒絕，**⛔ 沒有反向保護力**。
OUTSIDE3="$(mktemp -d)"
cp "$TD/pkg/a/sample.py" "$OUTSIDE3/outside_real.py"
REL_OUT="$(python3 -c "import os,sys;print(os.path.relpath(sys.argv[1], sys.argv[2]))" \
            "$OUTSIDE3/outside_real.py" "$TD")"
printf '`%s:4` `alpha`\n' "$REL_OUT" > "$TD/docs/x.md"
run
{ [ "$RC" -eq 1 ] && grep -q "repo 內找不到這個檔案" <<< "$OUT"; } \
  && pass '".." 指向真實的 repo 外檔案 → 被擋' \
  || fail "..（真實檔）未被擋：rc=$RC $OUT"
rm -rf "$OUTSIDE3"

#   (d) repo 內的**絕對路徑** → 必須被擋（⛔ 文件引用要可攜）
printf '`%s/pkg/a/sample.py:4` `alpha`\n' "$TD" > "$TD/docs/x.md"
run
{ [ "$RC" -eq 1 ] && grep -q "repo 內找不到這個檔案" <<< "$OUT"; } \
  && pass 'repo 內的絕對路徑 → 被擋' || fail "絕對路徑被接受：rc=$RC $OUT"

#   (e) repo 內但繞經 `..` → 必須被擋
printf '`pkg/../pkg/a/sample.py:4` `alpha`\n' > "$TD/docs/x.md"
run
{ [ "$RC" -eq 1 ] && grep -q "repo 內找不到這個檔案" <<< "$OUT"; } \
  && pass 'repo 內繞經 ".." → 被擋' || fail "繞經 .. 被接受：rc=$RC $OUT"

# ⑫-m walker：走訪期間路徑消失 → ⛔ 不得變成假紅
#   ⚠️ 直接刺激 `_find_by_basename` 的消失分支：mock `iterdir` 丟 FileNotFoundError。
miss_out="$(python3 - "$TD" <<'PYEOF_INNER'
import importlib.util, pathlib, sys
root = pathlib.Path(sys.argv[1])
spec = importlib.util.spec_from_file_location("chk", "scripts/check-doc-refs.py")
m = importlib.util.module_from_spec(spec); sys.modules["chk"] = m; spec.loader.exec_module(m)
m.ROOT = root
real_iterdir = pathlib.Path.iterdir
calls = {"n": 0}
def flaky(self):
    calls["n"] += 1
    if calls["n"] == 2:            # 第二次走訪時「目錄剛好被移除」
        raise FileNotFoundError(str(self))
    return real_iterdir(self)
pathlib.Path.iterdir = flaky
try:
    m._find_by_basename("sample.py", "pkg/a/sample.py")
    print("OK")
except FileNotFoundError:
    print("RAISED")
finally:
    pathlib.Path.iterdir = real_iterdir
PYEOF_INNER
)"
[ "$miss_out" = "OK" ] && pass "走訪期間目錄消失 → 跳過（⛔ 不變成假紅）" \
  || fail "消失分支未被容忍：$miss_out"

# ⑫-n walker：PermissionError ⛔ 不得被吞掉（那會漏掉候選 → fail-open）
perm_out="$(python3 - "$TD" <<'PYEOF_INNER2'
import importlib.util, pathlib, sys
root = pathlib.Path(sys.argv[1])
spec = importlib.util.spec_from_file_location("chk", "scripts/check-doc-refs.py")
m = importlib.util.module_from_spec(spec); sys.modules["chk"] = m; spec.loader.exec_module(m)
m.ROOT = root
real_iterdir = pathlib.Path.iterdir
def denied(self):
    raise PermissionError(str(self))
pathlib.Path.iterdir = denied
try:
    m._find_by_basename("sample.py", "pkg/a/sample.py")
    print("SWALLOWED")
except PermissionError:
    print("RAISED")
finally:
    pathlib.Path.iterdir = real_iterdir
PYEOF_INNER2
)"
[ "$perm_out" = "RAISED" ] && pass "PermissionError 往上拋（⛔ 不靜默漏候選）" \
  || fail "PermissionError 被吞掉：$perm_out"

# ⑫-h 識別符邊界：`alpha` ⛔ 不該被 `alphabet` 命中
cat > "$TD/pkg/a/word.py" <<'PY'
def alphabet():
    return 1
PY
printf '`pkg/a/word.py:1` `alpha`\n' > "$TD/docs/x.md"
run
[ "$RC" -eq 1 ] && pass "識別符邊界比對（alpha ≠ alphabet）" || fail "子字串誤命中：rc=$RC"
rm -f "$TD/pkg/a/word.py"

# ⑫ --list 會多印通過的引用
printf '`pkg/a/sample.py:4` `alpha`\n' > "$TD/docs/x.md"
run;         plain=$(printf '%s' "$OUT" | wc -l)
run --list;  listed=$(printf '%s' "$OUT" | wc -l)
[ "$listed" -gt "$plain" ] \
  && pass "--list 多印通過的引用（$plain → $listed 行）" || fail "--list 沒有作用（$plain vs $listed）"

# ⑬ 未知參數 → rc=2
set +e
python3 "$CHECK" --root "$TD" --bogus >/dev/null 2>&1; rc=$?
set -e
[ "$rc" -eq 2 ] && pass "未知參數 → rc=2" || fail "未知參數 rc=$rc（預期 2）"

# ⑭ --root 缺值 → rc=2
set +e
python3 "$CHECK" --root >/dev/null 2>&1; rc=$?
set -e
[ "$rc" -eq 2 ] && pass "--root 缺值 → rc=2" || fail "--root 缺值 rc=$rc"

echo
echo "==> doc-refs 測試：$PASS 通過、$FAIL 失敗"
[ "$FAIL" -eq 0 ]
