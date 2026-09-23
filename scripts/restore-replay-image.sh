#!/usr/bin/env bash
# I-074 Stage 2：從 repo 外的 tarball 還原釘住的 image（issue.md I-074 Stage 2 計畫書「二、⑤」）。
#
# 用法：
#   scripts/restore-replay-image.sh --stage 2 <bundle 絕對路徑>
#
# **stdout 只印 `sha256:…` 一行**，其餘訊息走 stderr；⚠️ 非零結果時 stdout ⛔ 無輸出。
#
# ⚠️ **為什麼需要它**：Stage 1 釘住的 image 已不在本機，而當時⛔ 沒有任何備份——凍結 bundle 的
# 可重現性只靠「image 還在」（issue.md I-116）。Stage 2 在 pin 時就 `docker save` 一份，
# 這支是對應的還原程序。順序寫死，任一步不符即中止：
#
#   ① 讀 Stage 2 identity 記的 image ID（經 `ensure-i074-run-identity.py --peek`：
#      bundle 相符檢查 ＋ 重新 fsync）
#   ② image 還在本機 → 只重新掛上專用 tag，⛔ 不 load
#   ③ **先驗 tarball 的 SHA-256**：sidecar 恰好一行、檔名恰好是 `<hex>.tar`，再對 tarball 本身重算
#      ——⛔ 不驗就 load 等於信任一份可能被換掉的檔案；⛔ 也⛔ 不用 `sha256sum -c`（可被 decoy 繞過）
#   ④ `docker load`
#   ⑤ **再驗 image ID 等於 identity 的 `expected_image_id`**——`docker save`／`load` 保留 image ID，
#      ⛔ 不相等代表 tarball 裝的不是那一個 image
#
# ⚠️ 只支援 `--stage 2`：Stage 1 當時沒有 tarball。
# 結束碼：0 ＝ image 已就位；1 ＝ 其他（⛔ 不自動重建）。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENSURE="$REPO_ROOT/python/scripts/ensure-i074-run-identity.py"
# shellcheck source=lib/image-tarball.sh
. "$REPO_ROOT/scripts/lib/image-tarball.sh"
IMAGE="stock-trading-python-replay:i074-stage2"
IMAGES_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/stock_trading/i074_stage2/images"

if [ "${1:-}" != "--stage" ] || [ "${2:-}" != "2" ] || [ -z "${3:-}" ] || [ "$#" -ne 3 ]; then
  echo "用法: $0 --stage 2 <bundle 絕對路徑>（⚠️ 只支援 Stage 2）" >&2
  exit 1
fi
BUNDLE_ABS="$(cd "$3" && pwd)"

# ① identity 記的 image ID
set +e
EXPECTED="$(python3 "$ENSURE" --stage 2 --bundle "$BUNDLE_ABS" --peek 2>/dev/null)"
PEEK_RC=$?
set -e
if [ "$PEEK_RC" -ne 0 ] || [ -z "$EXPECTED" ]; then
  echo "ERROR: 讀不到 Stage 2 的 run identity（rc=$PEEK_RC）——⛔ 沒有 identity 就沒有要還原的對象。" >&2
  exit 1
fi

# ② 已在本機
if docker image inspect "$EXPECTED" >/dev/null 2>&1; then
  docker tag "$EXPECTED" "$IMAGE" >&2
  echo "==> image 已在本機，只重新掛上專用 tag（⛔ 未 load）" >&2
  printf '%s\n' "$EXPECTED"
  exit 0
fi

HEX="${EXPECTED#sha256:}"
TAR="$IMAGES_DIR/$HEX.tar"
SUM="$IMAGES_DIR/$HEX.tar.sha256"
if [ ! -f "$TAR" ] || [ ! -f "$SUM" ]; then
  echo "ERROR: 找不到 tarball 或它的 .sha256：$TAR——⛔ 無法還原，⛔ 也⛔ 不得改為重建。" >&2
  exit 1
fi

# ③ 先驗 SHA——⚠️ ⛔ 不用 `sha256sum -c`：sidecar 可以改指向同目錄的 decoy，驗過的就不是
#    要 load 的那一份（見 lib/image-tarball.sh）。
if ! image_tarball_verify "$IMAGES_DIR" "$HEX"; then
  echo "ERROR: tarball 驗證不過：$TAR——⛔ 不 load。" >&2
  exit 1
fi
echo "==> tarball SHA-256 驗證通過" >&2

# ④ load
docker load -i "$TAR" >&2

# ⑤ 再驗 image ID
ACTUAL="$(docker image inspect "$EXPECTED" -f '{{.Id}}' 2>/dev/null || true)"
if [ "$ACTUAL" != "$EXPECTED" ]; then
  echo "ERROR: load 之後找不到 identity 記的 image（預期 $EXPECTED，實際 '$ACTUAL'）。" >&2
  exit 1
fi
docker tag "$EXPECTED" "$IMAGE" >&2
echo "==> 已從 tarball 還原並重新掛上專用 tag" >&2
printf '%s\n' "$EXPECTED"
