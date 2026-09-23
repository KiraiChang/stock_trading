#!/usr/bin/env bash
# I-074 Stage 1：**唯一**的 build／pin 入口，同時是 run identity 的 producer。
#
# 用法：
#   scripts/pin-replay-image.sh <bundle 絕對路徑>
#   scripts/pin-replay-image.sh --stage 2 <bundle 絕對路徑>   # I-074 Stage 2（見下方）
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
#
# ⚠️ **`--stage 2`（I-074 Stage 2，issue.md I-074 Stage 2 計畫書「二、⑤」）**：Stage 1 釘住的
# image 已不在本機（2026-09-23 查證）。可能成因是 pin 用的 tag 與 `python/scripts/test.sh`
# 等腳本**共用**——任何一次重新 build 都會讓釘住的 ID 失去 tag、之後被 prune 清掉。所以 Stage 2：
#
#   | 項目 | 做法 |
#   |---|---|
#   | tag | ⚠️ **專用 tag** `stock-trading-python-replay:i074-stage2`，⛔ 不與任何 build 腳本共用；⛔ 不接受 `PY_IMAGE` |
#   | identity | Stage 2 自己的一份（`…/stock_trading/i074_stage2/run_identity.json`，依 stage 固定推導） |
#   | tarball | pin 之後 `docker save` 到 repo 外的 `…/stock_trading/i074_stage2/images/<hex>.tar`，旁邊一份 `<hex>.tar.sha256` |
#   | image 已不在本機 | ⚠️ 仍 fail-closed；⚠️ 改用 `scripts/restore-replay-image.sh --stage 2 <bundle>` 從 tarball 還原（先驗 SHA、`docker load` 後再驗 image ID） |
#   | `--adopt-image <完整 image ID>` | ⚠️ **採用既有 image、⛔ 不 build**（2026-09-23 使用者裁決，見下方） |
#
# ⚠️ **`--adopt-image`（只限 `--stage 2`、只在 identity 還不存在時）**：2026-09-23 實測
# `stock_trading-python-server:latest`（與遺失的 image 同一份 Dockerfile、只晚 16 分鐘 build）的
# `pip_freeze_sha256` 與 Stage 1 **完全相同**，而今天重新 build 會裝到不同的套件（layer cache 已更新）。
# 所以採用它。⚠️ 採用前**必須證明環境相同**：容器內算出的 `{pip_freeze_sha256, python_version}`
# 要**逐字等於**已封存 Stage 1 after 的（`print-i074-environment.py`，經 Stage 1 信任錨），⛔ 不符即中止。
#
# ⚠️ **Stage 1 的行為逐項不變**（它的 fail-closed 是正確的）。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PYTHON_DIR="$REPO_ROOT/python"
IMAGE="${PY_IMAGE:-stock-trading-python-test:latest}"
ENSURE="$PYTHON_DIR/scripts/ensure-i074-run-identity.py"
# shellcheck source=lib/image-tarball.sh
. "$REPO_ROOT/scripts/lib/image-tarball.sh"

# ⚠️ `--no-identity`：**只 build 並印出 image ID**，⛔ 不建立／不讀 run identity。
# 給**非 I-074** 的一般 Stage 1／2 用——它們需要的只是「單一 build 實作 ＋ 以 ID 執行」，
# ⛔ 不該因此在使用者家目錄產生 I-074 的協調檔。
NO_IDENTITY=0
STAGE=1
STAGE_SEEN=0
ADOPT_IMAGE=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --adopt-image)
      [ "$#" -ge 2 ] || { echo "ERROR: --adopt-image 需要完整的 image ID。" >&2; exit 1; }
      [ -z "$ADOPT_IMAGE" ] || { echo "ERROR: --adopt-image 重複出現——⛔ 不靜默採用最後一個。" >&2; exit 1; }
      ADOPT_IMAGE="$2"; shift 2 ;;
    --no-identity)
      [ "$NO_IDENTITY" = "0" ] || { echo "ERROR: --no-identity 重複出現。" >&2; exit 1; }
      NO_IDENTITY=1; shift ;;
    --stage)
      [ "$#" -ge 2 ] || { echo "ERROR: --stage 需要值（1 或 2）。" >&2; exit 1; }
      # ⚠️ ⛔ 不靜默採用最後一個（CLI matrix：重複參數一律中止）。
      [ "$STAGE_SEEN" = "0" ] || { echo "ERROR: --stage 重複出現——⛔ 不靜默採用最後一個。" >&2; exit 1; }
      STAGE_SEEN=1; STAGE="$2"; shift 2 ;;
    --*) echo "ERROR: 未知參數 $1" >&2; exit 1 ;;
    *) break ;;
  esac
done
case "$STAGE" in
  1|2) ;;
  *) echo "ERROR: --stage 只接受 1 或 2（封閉列舉），實際 '$STAGE'。" >&2; exit 1 ;;
esac
if [ "$NO_IDENTITY" = "1" ] && [ "$STAGE" != "1" ]; then
  echo "ERROR: --no-identity ⛔ 不可與 --stage 2 並用——Stage 2 一定要有自己的 identity。" >&2
  exit 1
fi
if [ -n "$ADOPT_IMAGE" ]; then
  [ "$STAGE" = "2" ] || { echo "ERROR: --adopt-image 只限 --stage 2。" >&2; exit 1; }
  # ⚠️ 只收**完整 image ID**：⛔ 不收 tag（tag 會移動，採用的必須是當下確認過的那一個）。
  if ! [[ "$ADOPT_IMAGE" =~ ^sha256:[0-9a-f]{64}$ ]]; then
    echo "ERROR: --adopt-image 必須是完整的 image ID（sha256: ＋ 64 hex），⛔ 不收 tag：'$ADOPT_IMAGE'" >&2
    exit 1
  fi
fi
if [ "$STAGE" = "2" ]; then
  # ⚠️ 專用 tag：⛔ 與 build 腳本共用 tag 正是 Stage 1 的 image 遺失的可能成因。
  if [ -n "${PY_IMAGE:-}" ]; then
    echo "ERROR: --stage 2 ⛔ 不接受 PY_IMAGE——它一律使用專用 tag，⛔ 不與任何 build 腳本共用。" >&2
    exit 1
  fi
  IMAGE="stock-trading-python-replay:i074-stage2"
  IMAGES_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/stock_trading/i074_stage2/images"
fi

BUNDLE="${1:-}"
if [ "$NO_IDENTITY" = "0" ] && [ -z "$BUNDLE" ]; then
  echo "用法: $0 [--no-identity] [--stage 1|2 [--adopt-image <image ID>]] <bundle 絕對路徑>" >&2
  exit 1
fi
# ⚠️ bundle 之後⛔ 不得還有別的參數（⛔ 不靜默忽略）；`--no-identity` 不吃 bundle。
if [ "$NO_IDENTITY" = "1" ] && [ "$#" -gt 0 ] || [ "$#" -gt 1 ]; then
  echo "ERROR: 多餘的參數：$*——⛔ 不靜默忽略。" >&2
  exit 1
fi

# ── Stage 2：repo 外的 tarball（`docker save`）───────────────────────────────
#
# ⚠️ 已有 tarball 時**先驗 SHA**，⛔ 不覆寫；沒有才 save（temp → sha256 → rename）。
# ⚠️ save 失敗時 identity 仍在、tarball 沒有——重跑本腳本會走 no-op 分支並補做這一步。
save_stage2_tarball() {
  local image_id="$1" hex tar sum tmp digest
  hex="${image_id#sha256:}"
  tar="$IMAGES_DIR/$hex.tar"
  sum="$IMAGES_DIR/$hex.tar.sha256"
  mkdir -p "$IMAGES_DIR"
  if [ -f "$tar" ] || [ -f "$sum" ]; then
    if [ ! -f "$tar" ] || [ ! -f "$sum" ]; then
      echo "ERROR: tarball 與它的 .sha256 只剩其中一個（$IMAGES_DIR）——⛔ 不猜，人工處理。" >&2
      return 1
    fi
    # ⚠️ ⛔ 不用 `sha256sum -c`：sidecar 可以改指向同目錄的 decoy（見 lib/image-tarball.sh）。
    if ! image_tarball_verify "$IMAGES_DIR" "$hex"; then
      echo "ERROR: 既有 tarball 驗證不過：$tar——⛔ 不覆寫，人工處理。" >&2
      return 1
    fi
    echo "==> 既有 tarball 驗證通過：$tar" >&2
    return 0
  fi
  tmp="$IMAGES_DIR/.$hex.tar.tmp.$$"
  echo "==> docker save → $tar" >&2
  if ! docker save -o "$tmp" "$image_id" >&2; then
    rm -f "$tmp"
    echo "ERROR: docker save 失敗。" >&2
    return 1
  fi
  sync "$tmp"
  digest="$(sha256sum "$tmp" | cut -d' ' -f1)"
  mv "$tmp" "$tar"
  printf '%s  %s\n' "$digest" "$hex.tar" > "$sum.tmp.$$"
  sync "$sum.tmp.$$"
  mv "$sum.tmp.$$" "$sum"
  sync "$IMAGES_DIR"
  echo "==> tarball SHA-256：$digest（記錄在 $sum）" >&2
}
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

# ── Stage 2 的 --adopt-image：採用前證明環境與 Stage 1 相同 ──────────────────
adopt_stage2_image() {
  local image_id="$1" actual expected current
  actual="$(docker image inspect "$image_id" -f '{{.Id}}' 2>/dev/null || true)"
  if [ "$actual" != "$image_id" ]; then
    echo "ERROR: 本機找不到 image $image_id（實際 '$actual'）——⛔ 無法採用。" >&2
    return 1
  fi
  # ⚠️ 預期值取自**已驗證的** Stage 1 信任錨（⛔ 不是直接讀檔）。
  expected="$(python3 "$PYTHON_DIR/scripts/print-i074-environment.py" --stage1)" || return 1
  # ⚠️ 實際值**在要採用的那個 image 裡**算（⛔ 不是 host 的環境）；程式碼唯讀掛入、⛔ 不連網。
  current="$(docker run --rm --network none --memory=300m --memory-swap=300m \
      -v "$PYTHON_DIR":/repo-python:ro "$image_id" \
      python /repo-python/scripts/print-i074-environment.py --current)" || return 1
  if [ -z "$expected" ] || [ "$expected" != "$current" ]; then
    echo "ERROR: $image_id 的環境與 Stage 1 當時⛔ 不同——⛔ 不採用。" >&2
    echo "       Stage 1 ：$expected" >&2
    echo "       本 image：$current" >&2
    return 1
  fi
  echo "==> 環境與 Stage 1 相同：$current" >&2
}

# ── ① 已有 identity？ ───────────────────────────────────────────────────────
# `--peek` 的語意：存在 → 印既有 ID 並**重新 fsync**；不存在 → **exit 2**（⛔ 不是失敗）。
set +e
EXISTING="$(python3 "$ENSURE" --stage "$STAGE" --bundle "$BUNDLE_ABS" --peek 2>/dev/null)"
PEEK_RC=$?
set -e

if [ "$PEEK_RC" -eq 0 ]; then
  # ⚠️ 已有 identity 時 `--adopt-image` 只能指向**同一個** image（⛔ 不得藉此換掉已釘死的 image）。
  if [ -n "$ADOPT_IMAGE" ] && [ "$ADOPT_IMAGE" != "$EXISTING" ]; then
    echo "ERROR: Stage 2 identity 已釘死 $EXISTING，--adopt-image 卻指向 $ADOPT_IMAGE——⛔ 不換。" >&2
    exit 1
  fi
  # ⚠️ **image 必須仍在本機**，否則後面三趟會跑在別的 image 上。
  if ! docker image inspect "$EXISTING" >/dev/null 2>&1; then
    echo "ERROR: identity 記的 image $EXISTING 已不在本機——⛔ fail-closed。" >&2
    echo "       ⛔ 不得重建後換一個 ID：那會讓跨日的三趟跑在不同 image 上。" >&2
    if [ "$STAGE" = "2" ]; then
      echo "       改用 scripts/restore-replay-image.sh --stage 2 <bundle> 從 tarball 還原。" >&2
    else
      echo "       要換 image 等於整組 Stage 1 重來（連同已產出的證據）。" >&2
    fi
    exit 1
  fi
  if [ "$STAGE" = "2" ]; then
    # ⚠️ 重新掛上專用 tag（被 prune 保護的前提），並補齊／驗證 tarball。
    docker tag "$EXISTING" "$IMAGE" >&2
    save_stage2_tarball "$EXISTING"
  fi
  echo "==> 沿用既有的 run identity（⛔ 未 build、未改 created_at）" >&2
  printf '%s\n' "$EXISTING"
  exit 0
fi
if [ "$PEEK_RC" -ne 2 ]; then
  echo "ERROR: 讀取既有 run identity 失敗（rc=$PEEK_RC）。" >&2
  exit "$PEEK_RC"
fi

# ── ② 沒有 identity：build（或採用）一次並釘死 ──────────────────────────────
if [ -n "$ADOPT_IMAGE" ]; then
  echo "==> 採用既有 image（⛔ 不 build）：$ADOPT_IMAGE" >&2
  adopt_stage2_image "$ADOPT_IMAGE"
  IMAGE_ID="$ADOPT_IMAGE"
  # ⚠️ 先掛上專用 tag（被 prune 保護的前提），再建立 identity。
  docker tag "$IMAGE_ID" "$IMAGE" >&2
else
  echo "==> 建置 image：$IMAGE" >&2
  docker build -t "$IMAGE" "$PYTHON_DIR" >&2
  IMAGE_ID="$(docker image inspect "$IMAGE" -f '{{.Id}}' 2>/dev/null || true)"
  if [ -z "$IMAGE_ID" ]; then
    echo "ERROR: 取不到 $IMAGE 的 image ID。" >&2
    exit 1
  fi
fi

# ⚠️ identity 由 Python 寫（canonical ＋ 原子 rename ＋ 兩次 fsync），
# ⛔ shell 不自己組 JSON——schema 只能有一份。
set +e
OUT="$(python3 "$ENSURE" --stage "$STAGE" --bundle "$BUNDLE_ABS" --image-id "$IMAGE_ID")"
RC=$?
set -e
if [ "$RC" -ne 0 ]; then
  exit "$RC"
fi
if [ "$STAGE" = "2" ]; then
  save_stage2_tarball "$IMAGE_ID"
fi
echo "==> 已釘死 image ID 與 bundle 身分" >&2
printf '%s\n' "$OUT"
