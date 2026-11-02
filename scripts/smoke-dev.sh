#!/usr/bin/env bash
# Dev stack smoke test: start isolated dev compose and wait for core HTTP health checks.
#
# Usage:
#   scripts/smoke-dev.sh
#   NO_BUILD=1 SERVICES=backend OVERRIDE=/path/to/override.yml scripts/smoke-dev.sh
#
# Environment overrides:
#   COMPOSE_FILE  dev compose file (default: docker-compose.dev.yml)
#   BACKEND_URL   backend health URL (default: http://localhost:18080/health)
#   PYTHON_URL    python-server health URL (default: http://localhost:18001/health)
#   WAIT_SECONDS  max wait in seconds for dependency readiness and for each health
#                 check (default: 90)
#   LOG_TAIL      log lines shown on failure (default: 120)
#   SKIP_DOWN     set to 1 to skip the pre-build stop (default: 0)
#   NO_BUILD      set to 1 to skip both the pre-build stop and the build, e.g. when the
#                 images already match the code and only environment values changed.
#                 `up` gets --no-build, so a missing image fails instead of being built.
#                 (default: 0)
#   SERVICES      app services to start: backend, python-server, or both, separated by
#                 spaces or commas (default: both). Only the selected services are built
#                 and health-checked. postgres/redis are started as their dependencies;
#                 python-worker (profile "worker") is not an option.
#   OVERRIDE      one extra compose file, passed as a second -f (e.g. acceptance-only
#                 settings kept outside the repo). A relative path resolves against the
#                 directory you run the script from; paths inside it follow compose's rule
#                 (relative to the first -f, i.e. the repo root).
#
# COMPOSE_PROJECT_NAME must not be set (not even empty): compose lets it override the
# project name in the compose file, which would point `down` at another project.
set -euo pipefail

CALLER_PWD="$PWD"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.dev.yml}"
BACKEND_URL="${BACKEND_URL:-http://localhost:18080/health}"
PYTHON_URL="${PYTHON_URL:-http://localhost:18001/health}"
WAIT_SECONDS="${WAIT_SECONDS:-90}"
LOG_TAIL="${LOG_TAIL:-120}"
SKIP_DOWN="${SKIP_DOWN:-0}"
NO_BUILD="${NO_BUILD:-0}"
SERVICES="${SERVICES:-}"
OVERRIDE="${OVERRIDE:-}"

# ── 先驗參數：任何 docker 指令之前就擋下錯的值 ──

# COMPOSE_PROJECT_NAME 會蓋過 compose 檔的 name:，down --remove-orphans 就會打到別的 project。
# 設了它通常代表這個 shell 正在操作別的 project，所以中止、⛔ 不默默清掉。
if [ -n "${COMPOSE_PROJECT_NAME+x}" ]; then
  echo "ERROR: COMPOSE_PROJECT_NAME is set (\"$COMPOSE_PROJECT_NAME\"); it would override the project" >&2
  echo "       name in $COMPOSE_FILE. Unset it and rerun." >&2
  exit 2
fi

# SERVICES：空白或逗號分隔；重複的名稱忽略；空字串等於預設（兩個都起）。
want_backend=0
want_python=0
read -r -a requested <<< "${SERVICES//,/ }"
for svc in ${requested[@]+"${requested[@]}"}; do
  case "$svc" in
    backend) want_backend=1 ;;
    python-server) want_python=1 ;;
    *)
      echo "ERROR: SERVICES accepts only backend and python-server (got: $svc)." >&2
      echo "       postgres/redis are started as dependencies; python-worker is not an option." >&2
      exit 2
      ;;
  esac
done
if (( want_backend == 0 && want_python == 0 )); then
  want_backend=1
  want_python=1
fi

selected=()
if (( want_backend )); then selected+=(backend); fi
if (( want_python )); then selected+=(python-server); fi

if (( want_backend && want_python )); then
  mode="all"
  deps=()
elif (( want_backend )); then
  mode="partial"
  deps=(postgres redis)
else
  mode="partial"
  deps=(postgres)
fi

compose_files=(-f "$COMPOSE_FILE")
if [ -n "$OVERRIDE" ]; then
  # 腳本下面會 cd 到 repo 根目錄，相對路徑要先以呼叫時的目錄解析。
  case "$OVERRIDE" in
    /*) ;;
    *) OVERRIDE="$CALLER_PWD/$OVERRIDE" ;;
  esac
  if [ ! -f "$OVERRIDE" ]; then
    echo "ERROR: OVERRIDE file not found: $OVERRIDE" >&2
    exit 2
  fi
  compose_files+=(-f "$OVERRIDE")
fi

cd "$REPO_ROOT"

compose() {
  docker compose "${compose_files[@]}" "$@"
}

show_diagnostics() {
  local svc
  echo
  echo "==> dev stack status"
  compose ps || true
  for svc in "$@"; do
    echo
    echo "==> $svc logs"
    compose logs --tail="$LOG_TAIL" "$svc" || true
  done
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

echo "==> mode: services=${selected[*]} no_build=$NO_BUILD override=${OVERRIDE:-none}"

up_flags=(-d)
if [ "$NO_BUILD" = "1" ]; then
  # 缺 image 的 build 服務，compose 會在 up 時自己 build；--no-build 讓它直接失敗。
  up_flags+=(--no-build)
  echo "==> skip pre-build stop and build (NO_BUILD=1)"
else
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

  # 一次 build 一個：compose 預設會並行 build，兩個 image 的峰值會疊加。
  # 部分服務模式只 build 選了的服務。
  for svc in "${selected[@]}"; do
    echo "==> build dev image: $svc ($COMPOSE_FILE)"
    compose build "$svc"
  done
fi

if [ "$mode" = "all" ]; then
  echo "==> start dev stack: $COMPOSE_FILE"
  compose up "${up_flags[@]}"
else
  # 部分服務模式：先用 --wait 把依賴起到 healthy（dev 已經 down 時也會把它們起來；
  # 已經在跑時立刻返回），確認之後才用 --no-deps 起選了的服務。
  echo "==> start dependencies and wait until healthy: ${deps[*]}"
  if ! compose up "${up_flags[@]}" --wait --wait-timeout "$WAIT_SECONDS" "${deps[@]}"; then
    echo "ERROR: dependencies not healthy within ${WAIT_SECONDS}s: ${deps[*]}" >&2
    show_diagnostics "${deps[@]}"
    exit 1
  fi
  echo "==> start ${selected[*]} without deps"
  compose up "${up_flags[@]}" --no-deps "${selected[@]}"
  if (( ! want_python )); then
    echo "NOTE: python-server is not started. backend /health does not need it, but jobs that"
    echo "      call python (e.g. SR analysis) will fail; use this mode only for paths without python."
  fi
fi

echo
compose ps

if (( want_backend )); then
  if ! wait_for_health "backend" "$BACKEND_URL"; then
    show_diagnostics "${selected[@]}"
    exit 1
  fi
fi

if (( want_python )); then
  if ! wait_for_health "python-server" "$PYTHON_URL"; then
    show_diagnostics "${selected[@]}"
    exit 1
  fi
fi

echo
echo "==> dev stack smoke passed"
