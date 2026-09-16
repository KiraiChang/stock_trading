#!/usr/bin/env bash
# I-074 Stage 1：**唯一**的 build／pin 入口，同時是 run identity 的 producer。
#
# 用法：
#   scripts/pin-replay-image.sh <bundle 絕對路徑>
#
# **stdout 只印 `sha256:…` 一行**（machine-readable），其餘訊息一律走 stderr。
# ⚠️ **非零結果時 stdout ⛔ 無輸出**——probe／D／D+1／comparator／finalizer 都是靠 stdout
# 取這個 ID 的，失敗卻照印，下游會拿著一個沒驗過的值繼續跑。
#
# ⚠️ **依「identity 存不存在」分流**，⛔ 不比對 `created_at`（它每次都變，
# 「所有欄位相同才 no-op」永遠不成立；而且重新 build 也⛔ 不保證得到相同的 image ID）：
#
#   | identity | 行為 |
#   |---|---|
#   | 已存在 | 驗 bundle 相符 ＋ **確認該 image 仍在本機** → 直接輸出既有 ID；⛔ 不 build、⛔ 不產新 created_at、⛔ 不重寫；⚠️ 但**重新 fsync**（durability 的唯一修復路徑） |
#   | 不存在 | 才 build → inspect → 建立並發布 |
#   | 已存在但 image 已不在本機 | ⚠️ **fail-closed**，⛔ 不得重建後換一個 ID——那會讓跨日的三趟跑在不同 image 上 |
#
# 結束碼：0 ＝ 成功；3 ＝ durability 未確認（檔案有效，重跑會走 no-op 並重新 fsync）；1 ＝ 其他。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PYTHON_DIR="$REPO_ROOT/python"
IMAGE="${PY_IMAGE:-stock-trading-python-test:latest}"
ENSURE="$PYTHON_DIR/scripts/ensure-i074-run-identity.py"

# ⚠️ `--no-identity`：**只 build 並印出 image ID**，⛔ 不建立／不讀 run identity。
# 給**非 I-074** 的一般 Stage 1／2 用——它們需要的只是「單一 build 實作 ＋ 以 ID 執行」，
# ⛔ 不該因此在使用者家目錄產生 I-074 的協調檔。
NO_IDENTITY=0
if [ "${1:-}" = "--no-identity" ]; then
  NO_IDENTITY=1
  shift
fi

BUNDLE="${1:-}"
if [ "$NO_IDENTITY" = "0" ] && [ -z "$BUNDLE" ]; then
  echo "用法: $0 [--no-identity] <bundle 絕對路徑>" >&2
  exit 1
fi
BUNDLE_ABS=""
[ -n "$BUNDLE" ] && BUNDLE_ABS="$(cd "$BUNDLE" && pwd)"

if [ "$NO_IDENTITY" = "1" ]; then
  echo "==> 建置 image：$IMAGE（--no-identity：⛔ 不碰 run identity）" >&2
  docker build -t "$IMAGE" "$PYTHON_DIR" >&2
  ONLY_ID="$(docker image inspect "$IMAGE" -f '{{.Id}}' 2>/dev/null || true)"
  if [ -z "$ONLY_ID" ]; then
    echo "ERROR: 取不到 $IMAGE 的 image ID。" >&2
    exit 1
  fi
  printf '%s\n' "$ONLY_ID"
  exit 0
fi

# ── ① 已有 identity？ ───────────────────────────────────────────────────────
# `--peek` 的語意：存在 → 印既有 ID 並**重新 fsync**；不存在 → **exit 2**（⛔ 不是失敗）。
set +e
EXISTING="$(python3 "$ENSURE" --bundle "$BUNDLE_ABS" --peek 2>/dev/null)"
PEEK_RC=$?
set -e

if [ "$PEEK_RC" -eq 0 ]; then
  # ⚠️ **image 必須仍在本機**，否則後面三趟會跑在別的 image 上。
  if ! docker image inspect "$EXISTING" >/dev/null 2>&1; then
    echo "ERROR: identity 記的 image $EXISTING 已不在本機——⛔ fail-closed。" >&2
    echo "       ⛔ 不得重建後換一個 ID：那會讓跨日的三趟跑在不同 image 上。" >&2
    echo "       要換 image 等於整組 Stage 1 重來（連同已產出的證據）。" >&2
    exit 1
  fi
  echo "==> 沿用既有的 run identity（⛔ 未 build、未改 created_at）" >&2
  printf '%s\n' "$EXISTING"
  exit 0
fi
if [ "$PEEK_RC" -ne 2 ]; then
  echo "ERROR: 讀取既有 run identity 失敗（rc=$PEEK_RC）。" >&2
  exit "$PEEK_RC"
fi

# ── ② 沒有 identity：build 一次並釘死 ──────────────────────────────────────
echo "==> 建置 image：$IMAGE" >&2
docker build -t "$IMAGE" "$PYTHON_DIR" >&2
IMAGE_ID="$(docker image inspect "$IMAGE" -f '{{.Id}}' 2>/dev/null || true)"
if [ -z "$IMAGE_ID" ]; then
  echo "ERROR: 取不到 $IMAGE 的 image ID。" >&2
  exit 1
fi

# ⚠️ identity 由 Python 寫（canonical ＋ 原子 rename ＋ 兩次 fsync），
# ⛔ shell 不自己組 JSON——schema 只能有一份。
set +e
OUT="$(python3 "$ENSURE" --bundle "$BUNDLE_ABS" --image-id "$IMAGE_ID")"
RC=$?
set -e
if [ "$RC" -ne 0 ]; then
  exit "$RC"
fi
echo "==> 已釘死 image ID 與 bundle 身分" >&2
printf '%s\n' "$OUT"
