#!/bin/bash
# dc.sh 範本：手動操作 live compose 的小幫手（I-132；放在 scripts 目錄，跟 deploy.sh、compose.yml 一起）。
#
# compose.yml 裡已經沒有必填的 ${…:?}（Redis 密碼與應用設定都由 env_file 注入），在 scripts 目錄直接執行
# `docker compose ps`／`logs` 也可以。這支腳本多做三件事：
#   * 固定帶跟 deploy.sh 相同的 compose 參數（compose.yml、build 目錄當 project directory），
#     所以 project 名稱、build context 都跟 deploy.sh 一致；
#   * ⛔ 擋下 config：它會把 JWT、API key 等機密整份印出來；
#   * 會改變狀態（或可能印出機密）的子指令要加 --yes。
#
# 用法：
#   ./dc.sh ps
#   ./dc.sh logs --tail 200 backend
#   ./dc.sh logs -f --since 10m backend
#   ./dc.sh --yes restart backend
#
# 子指令的規則：
#   * ps、logs、top、images、events、version：唯讀，直接執行。
#   * config：⛔ 一律擋下。
#   * 其他（restart、stop、up、down、exec、run…）：要加 --yes。⚠️ 部署請用 deploy.sh，這裡的 up 不會做
#     deploy.sh 的檢查，也不會 git pull；⛔ 不要用這裡 up --build。
#   * compose 的全域選項⛔ 不開放。
set -euo pipefail

# build 用的 git checkout 的絕對路徑（跟 deploy.sh 的 PROJECT_DIR 相同）
PROJECT_DIR="/path/to/trading"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STACK_COMPOSE_FILE="$SCRIPT_DIR/compose.yml"

usage() {
  echo "用法：./dc.sh [--yes] 子指令 [參數…]" >&2
  echo "  唯讀（直接執行）：ps、logs、top、images、events、version" >&2
  echo "  config：不提供（會印出機密）" >&2
  echo "  其他子指令：要加 --yes" >&2
}

YES=0
if [ "${1:-}" = "--yes" ]; then
  YES=1
  shift
fi
if [ $# -lt 1 ]; then
  usage
  exit 2
fi

sub="$1"
case "$sub" in
  ps|logs|top|images|events|version) ;;
  config)
    echo "dc.sh: 不提供 config：它會把 JWT、API key 等機密整份印出來。" >&2
    exit 2 ;;
  -*)
    echo "dc.sh: 第一個參數要是子指令（--yes 除外）；compose 的全域選項⛔ 不開放，需要的話改這支腳本。" >&2
    usage
    exit 2 ;;
  *)
    if [ "$YES" -ne 1 ]; then
      echo "dc.sh: '$sub' 會改變狀態或可能印出機密，確定要執行請加 --yes，例如：./dc.sh --yes $sub ..." >&2
      exit 2
    fi ;;
esac

if [ ! -f "$STACK_COMPOSE_FILE" ]; then
  echo "dc.sh: 找不到 $STACK_COMPOSE_FILE" >&2
  exit 1
fi

exec docker compose -f "$STACK_COMPOSE_FILE" --project-directory "$PROJECT_DIR" "$@"
