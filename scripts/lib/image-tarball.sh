# I-074 Stage 2：image tarball 的 SHA 驗證（pin-replay-image.sh 與 restore-replay-image.sh 共用）。
#
# ⚠️ **⛔ 不可用 `sha256sum -c <sidecar>`**（2026-09-23 review）：`-c` 驗的是 sidecar **裡寫的那個檔名**，
# 而 sidecar 可以被改成指向同目錄的另一個合法檔案（decoy）——decoy 驗過之後，呼叫端卻去 load
# 固定的 `<hex>.tar`，於是**被竄改的 tarball 照樣被信任**。所以這裡：
#
#   ① sidecar **恰好一行**；
#   ② 格式恰好是 `<64 字元小寫 hex>␠␠<hex>.tar`，檔名⛔ 不得是別的；
#   ③ 讀出期望的 digest，**對 `<hex>.tar` 本身重算**比較。
#
#   用法：image_tarball_verify <images 目錄> <image hex>   → 0 ＝ 相符；非 0 ＝ 不符（訊息走 stderr）
image_tarball_verify() {
  local dir="$1" hex="$2" tar sum lines line expected actual
  tar="$dir/$hex.tar"
  sum="$dir/$hex.tar.sha256"
  if [ ! -f "$tar" ] || [ ! -f "$sum" ]; then
    echo "ERROR: 找不到 tarball 或它的 .sha256：$tar" >&2
    return 1
  fi
  lines="$(wc -l < "$sum")"
  if [ "$lines" -ne 1 ]; then
    echo "ERROR: $sum 必須恰好一行，實際 $lines 行——⛔ 不猜哪一行才算數。" >&2
    return 1
  fi
  line="$(cat "$sum")"
  if ! [[ "$line" =~ ^([0-9a-f]{64})\ \ (.+)$ ]]; then
    echo "ERROR: $sum 的格式不是「<64 hex>␠␠<檔名>」。" >&2
    return 1
  fi
  expected="${BASH_REMATCH[1]}"
  if [ "${BASH_REMATCH[2]}" != "$hex.tar" ]; then
    echo "ERROR: $sum 指向的是「${BASH_REMATCH[2]}」，⛔ 不是 $hex.tar——⛔ 不接受指向其他檔案的 checksum。" >&2
    return 1
  fi
  actual="$(sha256sum "$tar" | cut -d' ' -f1)"
  if [ "$actual" != "$expected" ]; then
    echo "ERROR: $tar 的 SHA-256 與記錄不符（記錄 $expected、實際 $actual）。" >&2
    return 1
  fi
  return 0
}
