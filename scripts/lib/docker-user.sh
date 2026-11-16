#!/usr/bin/env bash
# container 以哪個 uid 執行：依 Docker 是 rootless 還是 rootful 決定（I-133）。
#
# 由測試與評估腳本以 source 引入，不直接執行。
#
# 為什麼需要這個：
#   這些腳本把 repo、cache 或輸出目錄以讀寫掛進 container，要讓 container 寫出的檔案在主機上
#   是執行者（例如 dev）所有。
#   * rootful Docker：container 的 uid 就是主機的 uid，所以要帶 `--user "$(id -u):$(id -g)"`，
#     否則檔案會是 root 所有。
#   * rootless Docker：container 在 user namespace 裡，**container 的 uid 0 對應主機的執行者**，
#     其他 uid 對應 /etc/subuid 的範圍。這時帶 `--user 1001:1001` 反而會變成主機上的 subuid
#     （例如 166536），寫不進執行者擁有、權限 755 的目錄（2026-11-13 在新主機上
#     `mkdir /gocache/00: permission denied`）。所以 rootless 時⛔ 不帶 `--user`：container 的
#     root 對主機來說就是執行者，權限沒有變大。
#   * 判斷不出來（`docker info` 失敗）：照 rootful 處理，也就是原本的行為，⛔ 不靜默改成 root；
#     之後的 `docker run` 本來就會因為連不到 daemon 而失敗。
#
# 用法：
#   . "$REPO_ROOT/scripts/lib/docker-user.sh"
#   …參數檢查、selftest 等會提前結束的路徑…
#   docker_user_args                     # 設定 DOCKER_USER_ARGS 陣列（rootless 時是空的）
#   docker run ${DOCKER_USER_ARGS[@]+"${DOCKER_USER_ARGS[@]}"} …
#
# ⚠️ source 只載入函式、⛔ 沒有副作用；docker_user_args 會呼叫 docker info，一定要排在參數檢查與
#    selftest 提前結束之後、緊接著啟動 container 的那一段之前（review 2026-11-16：排在前面時，
#    只驗參數的 selftest 也會去問 daemon，daemon 沒回應就卡住、偽造的參數也無法立即被拒絕）。
#    scripts/test-docker-user.sh 會檢查呼叫的位置。
#
# 可覆寫的環境變數（測試與除錯用）：
#   DOCKER_USER_MODE        auto（預設：問 docker info）、user（一律帶 --user）、none（一律不帶）
#   DOCKER_INFO_TIMEOUT_SEC docker info 的上限秒數（預設 10；有 coreutils 的 timeout 才生效），逾時照 rootful 處理
#
# ⛔ 其他腳本不要再直接寫 `--user "$(id -u):$(id -g)"`：scripts/test-docker-user.sh 會擋。

# docker_user_args：設定全域陣列 DOCKER_USER_ARGS。同一支腳本呼叫多次只問一次 docker info。
# DOCKER_USER_ARGS 由引入它的腳本使用
# shellcheck disable=SC2034
docker_user_args() {
  local mode="${DOCKER_USER_MODE:-auto}"
  if [ "$mode" = auto ]; then
    if [ -z "${_DOCKER_USER_DETECTED:-}" ]; then
      # daemon 沒回應時⛔ 不能卡住：有 timeout（coreutils）就限制時間，逾時當成判斷不出來。
      local opts t="${DOCKER_INFO_TIMEOUT_SEC:-10}" to=()
      command -v timeout >/dev/null 2>&1 && to=(timeout "$t")
      if opts="$(${to[@]+"${to[@]}"} docker info --format '{{.SecurityOptions}}' 2>/dev/null </dev/null)"; then
        case "$opts" in
          *name=rootless*) _DOCKER_USER_DETECTED=none ;;
          *) _DOCKER_USER_DETECTED=user ;;
        esac
      else
        _DOCKER_USER_DETECTED=user
      fi
    fi
    mode="$_DOCKER_USER_DETECTED"
  fi
  case "$mode" in
    none) DOCKER_USER_ARGS=() ;;
    user) DOCKER_USER_ARGS=(--user "$(id -u):$(id -g)") ;;
    *)
      echo "ERROR: DOCKER_USER_MODE 只能是 auto、user 或 none（目前：$mode）" >&2
      return 2
      ;;
  esac
}
