#!/usr/bin/env bash
# 對真實 DB 執行 SR Zone evaluation / decision replay，**預設唯讀、不寫任何一張表**。
#
# 用途：daily confirmation 驗證、T-039 的 sweep 取樣、T-040 選股的分佈量測——
# 這些都要拿真實資料跑，不是單元測試能取代的（單元測試用的是合成資料）。
#
# 用法：
#   scripts/run-evaluation.sh --symbols 2330,2454,...            # evaluation
#   MODE=replay scripts/run-evaluation.sh --symbols 2330,2454    # decision replay（daily confirmation 在這裡）
#   MODE=sweep  scripts/run-evaluation.sh --symbols 2330,2454    # ATR builder 參數 sweep
#   OUTPUT=/tmp/report.json scripts/run-evaluation.sh --symbols … # 結果落檔（預設印到 stdout）
#
# 額外參數會原樣轉交 evaluation.py，例如 --limit 1200、--atr-width-grid 0.8,1.0。
#
# 可覆寫的環境變數：
#   MODE         evaluate（預設）/ replay / sweep
#   OUTPUT       輸出 JSON 路徑（container 內會掛成 /out）；未指定時印到 stdout
#   DB_DSN       SQLAlchemy DSN。**預設從 live 的 python-server container 讀它的
#                DATABASE_DSN**——不要把密碼寫進 repo，也不會因為 live 改密碼而失效。
#   DSN_FROM     要讀 DSN 的 container 名（預設 stock_trading-python-server-1）
#   NETWORK      要接的 docker 網路（預設 trading-net，live postgres 所在）
#   MODELS_DIR   模型目錄（預設 /opt/stacks/scripts/stock_trading/python/models）
#   MODEL_PATH   模型檔在 container 內的路徑（預設 /app/models/sr_scoring_v4.joblib）
#   MEM / CPUS / PY_IMAGE / MEM_RESERVE_MB …  同 python/scripts/test.sh
#   WRITE_DB=1   **才會**加上 --write-db（預設不加，見下方安全預設）
#
# 設計重點：
#   - **預設唯讀**：不帶 --write-db。CLAUDE.md 規定驗收不得動 live 資料，而這支腳本的
#     資料來源就是 live，所以把「不寫」設成預設、要寫必須明確 WRITE_DB=1 並看到警告。
#   - **資料來源為什麼是 live**：evaluation 需要數千根真實日 K，dev project 沒有這些資料。
#     唯讀讀取不在 CLAUDE.md 禁止之列（該條針對測試資料、migration 驗證與清空資料），
#     todo.md T-040 也明訂「實際抓取由使用者在 live 環境執行，之後透過 live DB 驗證」。
#   - **模型檔在 live 主機上**，repo 的 python/models/ 是空的，所以要掛 MODELS_DIR（唯讀）。
#   - 沿用 python/scripts/test.sh 的 image 與資源約束（mem-guard、--user、不留 __pycache__）。
#   - 記憶體：evaluation 的 sources 與 dataset 必須同時常駐（見 issue.md I-056），
#     用量隨標的數成長且**不可線性外推**。跑大批標的前先看 free -m。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PYTHON_DIR="$REPO_ROOT/python"
IMAGE="${PY_IMAGE:-stock-trading-python-test:latest}"
MEM="${MEM:-700m}"
CPUS="${CPUS:-1}"
MODE="${MODE:-evaluate}"
NETWORK="${NETWORK:-trading-net}"
MODELS_DIR="${MODELS_DIR:-/opt/stacks/scripts/stock_trading/python/models}"
MODEL_PATH="${MODEL_PATH:-/app/models/sr_scoring_v4.joblib}"
DSN_FROM="${DSN_FROM:-stock_trading-python-server-1}"
OUTPUT="${OUTPUT:-}"

# DSN 從 live container 的環境變數讀，不寫進 repo：
#   1. 密碼不該進版控；
#   2. live 改密碼時這支腳本不用跟著改；
#   3. 密碼含 `@` 之類的字元時，手抄進 DSN 很容易寫出解析錯誤的字串。
if [ -z "${DB_DSN:-}" ]; then
  DB_DSN="$(docker inspect "$DSN_FROM" -f '{{range .Config.Env}}{{println .}}{{end}}' 2>/dev/null \
    | sed -n 's/^DATABASE_DSN=//p' | head -1)"
  if [ -z "$DB_DSN" ]; then
    echo "ERROR: 從 $DSN_FROM 讀不到 DATABASE_DSN（live stack 沒起來？）。" >&2
    echo "       可用 DB_DSN=... 直接指定，或用 DSN_FROM=<container> 換一個來源。" >&2
    exit 1
  fi
fi

# shellcheck source=lib/mem-guard.sh
. "$REPO_ROOT/scripts/lib/mem-guard.sh"
MEM="$(mem_guard_clamp "$MEM")"
MEMSWAP="${MEMSWAP:-$MEM}"

MODE_ARGS=()
case "$MODE" in
  evaluate) ;;
  replay)   MODE_ARGS+=(--decision-replay) ;;
  sweep)    MODE_ARGS+=(--sweep) ;;
  *) echo "ERROR: 未知的 MODE=$MODE（可用 evaluate / replay / sweep）" >&2; exit 1 ;;
esac

if [ "${WRITE_DB:-0}" = "1" ]; then
  cat >&2 <<'EOF'
==> [warn] WRITE_DB=1：這次會**寫入** stock_sr_regression_results。
    資料來源是 live DB，寫入即是動到 live 資料。確定是有意為之再繼續。
EOF
  MODE_ARGS+=(--write-db)
fi

if ! docker network inspect "$NETWORK" >/dev/null 2>&1; then
  echo "ERROR: 找不到 docker 網路 $NETWORK——live stack 沒起來？（NETWORK= 可覆寫）" >&2
  exit 1
fi
if [ ! -d "$MODELS_DIR" ]; then
  echo "ERROR: 模型目錄不存在：$MODELS_DIR（MODELS_DIR= 可覆寫）" >&2
  exit 1
fi

DOCKER_ARGS=(
  --rm
  --user "$(id -u):$(id -g)"
  --network "$NETWORK"
  --cpus="$CPUS"
  --memory="$MEM"
  --memory-swap="$MEMSWAP"
  --pids-limit=200
  -e HOME=/tmp
  -e PYTHONDONTWRITEBYTECODE=1
  -e DATABASE_DRIVER=postgres
  -e DATABASE_DSN="$DB_DSN"
  -e SR_SCORING_MODEL_PATH="$MODEL_PATH"
  -v "$PYTHON_DIR":/app
  -v "$MODELS_DIR":/app/models:ro
  -w /app
)

CMD_ARGS=(python -m backtest.modular.sr_scoring.evaluation "${MODE_ARGS[@]}" --model-path "$MODEL_PATH" "$@")

if [ -n "$OUTPUT" ]; then
  OUT_DIR="$(cd "$(dirname "$OUTPUT")" && pwd)"
  OUT_FILE="$(basename "$OUTPUT")"
  DOCKER_ARGS+=(-v "$OUT_DIR":/out)
  CMD_ARGS+=(--output "/out/$OUT_FILE")
fi

echo "==> 建置 image：$IMAGE"
docker build -t "$IMAGE" "$PYTHON_DIR"

echo "==> evaluation：mode=$MODE network=$NETWORK mem=$MEM write_db=${WRITE_DB:-0}"
echo "    args: $*"
exec docker run "${DOCKER_ARGS[@]}" "$IMAGE" "${CMD_ARGS[@]}"
