#!/bin/bash -p
# I-074 Stage 2 ⑦c：B／C 判讀器的入口（issue.md I-074「Stage 2 步驟 ⑦c 細部計畫 v1」「二之四」）。
#
#   REPLAY_IMAGE_ID=sha256:… scripts/judge-i074-stage2.sh        # ⛔ 沒有參數
#
# 讀真正 repo 中**已晉升**的成功 archive（python/baselines/i074_stage2/evidence/）：以它的
# `finalizer_provenance.base_commit` 建暫存複本，在複本裡執行
# `finalize-stage2-evidence.sh --verify-promotion-staging <真正 repo>/…/evidence --target evidence --judge`——
# 判讀規則與錨點在結構上一定是 ⑩ 之前寫好的那一版（⛔ 不讀真正 repo 的工作樹或目前的 HEAD）；
# 驗證與判讀在**同一個 Python 程序**（⛔ 沒有第二次讀取）；信任根的綁定（manifest 的 base_commit ＝ 暫存複本的 HEAD）
# 也在那裡驗，所以第 1 步讀到之後 manifest 若被換掉，就在第 3 步擋下。
#
# 結束碼：0 ＝ 已判讀（B 與 C **都是 0**，結果在 stdout 那一行 JSON 的 `verdict`）；1 ＝ 拒絕判讀（⛔ 不輸出 B／C）。
# ⚠️ ⛔ 不取鎖、⛔ 不寫真正 repo（唯讀、在晉升之後由人執行）；與 ⑩ 的入口同一套 bootstrap（「三」#10）：
#    正式命令一律直接執行本檔（`#!/bin/bash -p`）；以 `bash <本檔>` 啟動會被拒絕。
set -euo pipefail

# ── 常數（⚠️ 測試以 sed 改的只有這一段：scripts/test-i074-stage2.sh） ──────────────────
I074_TRUSTED_PATH=/usr/bin:/bin
I074_GIT=/usr/bin/git
I074_PYTHON=/usr/bin/python3
# ── 常數結束 ────────────────────────────────────────────────────────────────────────

readonly SELF_REL=scripts/judge-i074-stage2.sh
readonly EVIDENCE_REL=python/baselines/i074_stage2/evidence

err() { printf 'ERROR: %s\n' "$*" >&2; }

# ⚠️ 以下到 PATH 為止**只用 bash 內建**：bash -p 已經⛔ 不處理 BASH_ENV／ENV；這裡再清掉會讓固定路徑的程式
#    載入呼叫者程式碼的變數。
case "$-" in
  *p*) ;;
  *) err "必須直接執行 $SELF_REL（#!/bin/bash -p）——⛔ 不接受 bash <script>（BASH_ENV 可能已經執行）"; exit 1 ;;
esac
unset BASH_ENV ENV
for v in $(compgen -e); do
  case "$v" in LD_*|PYTHON*|GIT_*) unset "$v" ;; esac
done
PATH="$I074_TRUSTED_PATH"
export PATH
[ "$#" = 0 ] || { err "⛔ 沒有參數（判讀的對象固定是真正 repo 的 $EVIDENCE_REL）"; exit 1; }
if ! [[ "${REPLAY_IMAGE_ID:-}" =~ ^sha256:[0-9a-f]{64}$ ]]; then
  err "REPLAY_IMAGE_ID 必須是 sha256: ＋ 64 位小寫 hex（⛔ 不接受 tag 或選項）"
  exit 1
fi
hash -r
[ "$(command -v git)" = "$I074_GIT" ] && [ "$(command -v python3)" = "$I074_PYTHON" ] \
  || { err "git／python3 不是信任根（$I074_GIT、$I074_PYTHON）"; exit 1; }

# ── 0：本腳本的內容 ＝ HEAD ───────────────────────────────────────────────────────
case "${BASH_SOURCE[0]}" in */*) repo="${BASH_SOURCE[0]%/*}/.." ;; *) repo=.. ;; esac
repo="$(cd "$repo" && pwd -P)" || { err "找不到 repo"; exit 1; }
git -C "$repo" ls-files --error-unmatch -- "$SELF_REL" >/dev/null 2>&1 || { err "$SELF_REL 沒有被追蹤"; exit 1; }
[ "$(git -C "$repo" cat-file blob "HEAD:$SELF_REL" | sha256sum)" = "$(sha256sum < "$repo/$SELF_REL")" ] \
  || { err "$SELF_REL 的內容 ≠ HEAD 中的版本——⛔ 不執行未 commit 的程式"; exit 1; }

# ── 1：讀 manifest 的 finalizer_provenance.base_commit（⚠️ 只用來決定跑哪一版；真正的驗證在第 3 步） ────
base="$("$I074_PYTHON" -I -B -c '
import json, re, sys
v = (json.load(open(sys.argv[1], encoding="utf-8")).get("finalizer_provenance") or {}).get("base_commit")
sys.exit(1) if not (isinstance(v, str) and re.fullmatch("[0-9a-f]{40}", v)) else print(v)' \
  "$repo/$EVIDENCE_REL/evidence_manifest.json" 2>/dev/null)" \
  || { err "讀不到 $EVIDENCE_REL/evidence_manifest.json 的 finalizer_provenance.base_commit——拒絕判讀"; exit 1; }

# ── 2：base_commit 可達 → 暫存複本、detached checkout、HEAD ＝ base（「三」#1） ──────────
git -C "$repo" cat-file -e "${base}^{commit}" 2>/dev/null \
  || { err "base_commit $base 在真正 repo 裡不可達——拒絕判讀"; exit 1; }
tmp="$(mktemp -d)" || { err "建不了暫存目錄"; exit 1; }
trap 'rm -rf -- "$tmp"' EXIT
git clone -q --no-hardlinks --no-checkout --template= -- "$repo" "$tmp/repo" || { err "clone 失敗"; exit 1; }
git -C "$tmp/repo" -c core.hooksPath=/dev/null checkout -q --detach "$base" || { err "checkout $base 失敗"; exit 1; }
[ "$(git -C "$tmp/repo" rev-parse HEAD)" = "$base" ] || { err "暫存複本的 HEAD ≠ base_commit"; exit 1; }
[ "$(git -C "$tmp/repo" remote get-url origin)" = "$repo" ] || { err "暫存複本的 origin ≠ 真正 repo"; exit 1; }
[ -f "$tmp/repo/scripts/finalize-stage2-evidence.sh" ] && [ ! -L "$tmp/repo/scripts/finalize-stage2-evidence.sh" ] \
  || { err "base_commit $base 裡沒有驗證入口——拒絕判讀"; exit 1; }

# ── 3：驗證 ＋ 判讀（同一個 Python 程序）；錨點取自暫存複本（＝ base_commit 的樹） ─────────────
set +e
out="$(TMPDIR="$tmp" "$tmp/repo/scripts/finalize-stage2-evidence.sh" --verify-promotion-staging "$repo/$EVIDENCE_REL" \
         --target evidence --judge)"
rc=$?
set -e
[ "$rc" = 0 ] || { err "驗證模式不通過（rc=$rc）——拒絕判讀、⛔ 不輸出 B／C"; exit 1; }

# ── 4：stdout 只印驗證模式那一行 JSON（含 verdict、base_commit、manifest SHA） ─────────────────
printf '%s\n' "$out"
