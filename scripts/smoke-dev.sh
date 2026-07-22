#!/usr/bin/env bash
# Dev stack smoke test: start isolated dev compose and wait for core HTTP health checks.
#
# Usage:
#   scripts/smoke-dev.sh
#
# Environment overrides:
#   COMPOSE_FILE  dev compose file (default: docker-compose.dev.yml)
#   BACKEND_URL   backend health URL (default: http://localhost:18080/health)
#   PYTHON_URL    python-server health URL (default: http://localhost:18001/health)
#   WAIT_SECONDS  max health-check wait in seconds (default: 90)
#   LOG_TAIL      log lines shown on failure (default: 120)
#   SKIP_DOWN     set to 1 to skip the pre-build stop (default: 0)
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.dev.yml}"
BACKEND_URL="${BACKEND_URL:-http://localhost:18080/health}"
PYTHON_URL="${PYTHON_URL:-http://localhost:18001/health}"
WAIT_SECONDS="${WAIT_SECONDS:-90}"
LOG_TAIL="${LOG_TAIL:-120}"
SKIP_DOWN="${SKIP_DOWN:-0}"

cd "$REPO_ROOT"

compose() {
  docker compose -f "$COMPOSE_FILE" "$@"
}

show_diagnostics() {
  echo
  echo "==> dev stack status"
  compose ps || true
  echo
  echo "==> backend logs"
  compose logs --tail="$LOG_TAIL" backend || true
  echo
  echo "==> python-server logs"
  compose logs --tail="$LOG_TAIL" python-server || true
}

wait_for_health() {
  local name="$1"
  local url="$2"
  local deadline=$((SECONDS + WAIT_SECONDS))

  echo "==> wait for $name health: $url"
  until curl -fsS "$url" >/dev/null; do
    if (( SECONDS >= deadline )); then
      echo "ERROR: $name health check timed out after ${WAIT_SECONDS}s: $url" >&2
      return 1
    fi
    sleep 2
  done
  echo "    ok: $name"
}

# build 之前先把 dev stack 停掉：這台 host 只有 2GiB RAM，實測冷 cache build 的低點
# 只剩 74 MiB available（Go compile 峰值 RSS 約 420 MiB）。上一輪留著的 dev stack
# 約占 145 MiB（postgres 26 + redis 9 + backend 9 + python-server 99），不先停就會
# 在 build 階段把記憶體壓成負數而被 OOM killer 砍掉（signal: killed）。
# 注意：這裡刻意不帶 -v，named volume（dev_postgres_data / dev_redis_data）要保留；
# 清空 dev 驗收資料是另一個明確動作，見 docs/development-workflow.md。
if [ "$SKIP_DOWN" = "1" ]; then
  echo "==> skip pre-build stop (SKIP_DOWN=1)"
else
  echo "==> stop dev stack before build (keep volumes)"
  compose down --remove-orphans
fi

echo "==> build dev images: $COMPOSE_FILE"
compose build

echo "==> start dev stack: $COMPOSE_FILE"
compose up -d

echo
compose ps

if ! wait_for_health "backend" "$BACKEND_URL"; then
  show_diagnostics
  exit 1
fi

if ! wait_for_health "python-server" "$PYTHON_URL"; then
  show_diagnostics
  exit 1
fi

echo
echo "==> dev stack smoke passed"
