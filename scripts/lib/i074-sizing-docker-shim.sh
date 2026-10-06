#!/usr/bin/env bash
# I074-SIZING-DOCKER-SHIM
# I-074 Stage 2 步驟 ④：sizing harness 的 docker shim（issue.md I-074「Stage 2 步驟 ④：sizing harness 計畫書」
# 的「三、docker shim」表）。harness 把它以 `docker` 的名字放在 PATH 最前面；⛔ 不是正式入口。
#
# 只改寫 `docker run`，其餘子指令原樣 exec 真正的 docker。改寫內容：
#   在 image 前加 --cidfile <S>/cid/<ID>.cid、--name i074sz-<run id>-<ID>、--read-only、-v <S>/peak/<ID>:/peak，
#   拿掉 --rm（量完足跡後由本 shim `docker rm`）；容器指令包進 PEAK_WRAPPER；原本的 option 與指令逐 token 不變。
#   ⛔ 不提供任何可寫的 /tmp（封閉寫入模型）。
#
# ⚠️ **I/O 透明**：本 shim 自己⛔ 不向 stdout／stderr 寫任何 byte——容器的 stdout／stderr 原樣直通
#   （`--check-failed-record` 把 stdout 當 JSON 解析）；計量結果只寫 S（tmpfs）裡的 sidecar；
#   自己的錯誤寫 <S>/shim-errors.log。
#
# 由 harness 設定的環境：SIZING_REAL_DOCKER、SIZING_STATE（S）、SIZING_RUN_ID、SIZING_IMAGE、SIZING_PHASE、
#   SIZING_ROLE、SIZING_INCLUDED（true／false）、SIZING_HELPER（python/scripts/i074_stage2_sizing.py）。
#
# ⚠️ I-074 ⑦d（「Stage 2 步驟 ⑦d 細部計畫 v1」「二之四」）：`SIZING_PROFILE`——未設定 ＝ sizing（上面的行為⛔ 不變）；
#   `acceptance`：⛔ 不加 `--read-only`（⑩ 不加；SizeRw 由報告照實計入）；role `replay` 的容器指令開頭必須恰好是
#   `python -m backtest.modular.sr_scoring.evaluation`，換成 `python /acceptance/replay_stub.py`（其後的參數逐 token 不變），
#   並加唯讀掛載 `<S>/harness/python/scripts/i074_stage2_replay_stub.py`（**快照**的路徑，由 S 推導、⛔ 不接受其他來源）與
#   兩個模式變數（SIZING_REPLAY_MODE、SIZING_REPLAY_COMPUTE）；`full` 另把恰好一個 `-v <x>:/app:ro` 的來源換成
#   SIZING_NOCF_PYTHON（`e1cbbbd` ＋ tooling、⛔ 不套 counterfactual 的 worktree）。其他值 → 125、⛔ 不執行。
# ⚠️ ⑦d 實作第一輪 review：兩個 profile 的 sidecar 都另記 daemon 實際套用的記憶體上限（`docker inspect` 的
#   `HostConfig.Memory`／`MemorySwap`；mem-guard 下修後的 --memory），由 acceptance 的報告逐 invocation 比對封存的 argv。
# ⚠️ ⑦d 增補（「Stage 2 步驟 ⑦d 增補計畫：容器記憶體改以 RSS 判定」「二」①）：acceptance 的**每一個** role 都以唯讀掛載
#   `<S>/harness/python/scripts/i074_stage2_rss_wrapper.py`（**快照**的路徑，由 S 推導）並把容器指令包成
#   `python -I /acceptance/rss_wrapper.py <原指令>`（取代 sizing 的 `sh -c "$PEAK_WRAPPER"`）；快照裡沒有它 → 125。
#   sizing 照舊（⛔ 不掛 wrapper）。
set -uo pipefail

REAL="${SIZING_REAL_DOCKER:?}"
# ⚠️ I-074 ⑦b：與 ⑩ 的 label shim **互斥**（「Stage 2 步驟 ⑦b 細部計畫 v1」「二之五」）——在 ⑩ 的流程裡（環境帶
#   `I074_STAGE2_*`）、或「真正的 docker」其實是 label shim，一律 125、⛔ 不執行；錯誤只寫 S（I/O 透明）。
_shim_reject() {
  [ -n "${SIZING_STATE:-}" ] && printf 'shim：%s\n' "$1" >>"$SIZING_STATE/shim-errors.log" 2>/dev/null
  exit 125
}
for _v in $(compgen -e); do
  case "$_v" in I074_STAGE2_*) _shim_reject "環境帶 $_v（與 label shim 互斥）" ;; esac
done
if [ "$(head -c 2 -- "$REAL" 2>/dev/null)" = '#!' ] \
   && grep -q 'I074-STAGE2-LABEL-SHIM' <<< "$(head -n 5 -- "$REAL")"; then
  _shim_reject "SIZING_REAL_DOCKER 是 label shim：$REAL（兩個 shim 互斥）"
fi
if [ "${1:-}" != "run" ]; then
  exec "$REAL" "$@"
fi
shift

S="${SIZING_STATE:?}"
IMAGE="${SIZING_IMAGE:?}"
ERRLOG="$S/shim-errors.log"
py() { PYTHONDONTWRITEBYTECODE=1 python3 "$SIZING_HELPER" "$@" 2>>"$ERRLOG"; }
PROFILE="${SIZING_PROFILE:-sizing}"
case "$PROFILE" in
  sizing|acceptance) ;;
  *) printf 'shim：未知的 SIZING_PROFILE %s\n' "$PROFILE" >>"$ERRLOG"; exit 125 ;;
esac

# ⚠️ 與 `run-replay-offline.sh` 的 MEASURE_PEAK **同一套**候選路徑與順序：v1 → v2；讀不到時 peak 檔為空
#   （report 會 fail-closed）；⚠️ 保留原指令的結束碼。`SIZING_CGROUP_ROOT`／`SIZING_PEAK_DIR` 只給 host 上的測試覆寫。
PEAK_WRAPPER='"$@"; rc=$?;
{ cat "${SIZING_CGROUP_ROOT:-/sys/fs/cgroup}/memory/memory.max_usage_in_bytes" 2>/dev/null \
  || cat "${SIZING_CGROUP_ROOT:-/sys/fs/cgroup}/memory.peak" 2>/dev/null; } > "${SIZING_PEAK_DIR:-/peak}/peak" || true
exit $rc'

args=("$@")
pos=-1
for i in "${!args[@]}"; do
  if [ "${args[$i]}" = "$IMAGE" ]; then pos=$i; break; fi
done
if [ "$pos" -lt 0 ]; then
  printf 'shim：docker run 的參數裡找不到 image %s\n' "$IMAGE" >>"$ERRLOG"
  exit 125
fi
opts=()
for tok in "${args[@]:0:pos}"; do
  [ "$tok" = "--rm" ] || opts+=("$tok")
done
cmd=("${args[@]:pos+1}")
READ_ONLY=(--read-only)
if [ "$PROFILE" = acceptance ]; then
  READ_ONLY=()
  if [ "${SIZING_ROLE:-}" = replay ]; then
    STUB="$S/harness/python/scripts/i074_stage2_replay_stub.py"
    if [ "${#cmd[@]}" -lt 3 ] || [ "${cmd[0]}" != python ] || [ "${cmd[1]}" != -m ] \
       || [ "${cmd[2]}" != backtest.modular.sr_scoring.evaluation ]; then
      printf 'shim：replay 的容器指令開頭不是 python -m backtest.modular.sr_scoring.evaluation：%s\n' "${cmd[*]:0:3}" >>"$ERRLOG"
      exit 125
    fi
    case "${SIZING_REPLAY_MODE:-}:${SIZING_REPLAY_COMPUTE:-}" in
      success:stub|failure:stub|success:full) ;;
      *) printf 'shim：SIZING_REPLAY_MODE／SIZING_REPLAY_COMPUTE 不符：%s\n' "${SIZING_REPLAY_MODE:-}:${SIZING_REPLAY_COMPUTE:-}" >>"$ERRLOG"
         exit 125 ;;
    esac
    [ -f "$STUB" ] || { printf 'shim：快照裡沒有 launcher：%s\n' "$STUB" >>"$ERRLOG"; exit 125; }
    if [ "$SIZING_REPLAY_COMPUTE" = full ]; then
      [ -n "${SIZING_NOCF_PYTHON:-}" ] || { printf 'shim：full 沒有 SIZING_NOCF_PYTHON\n' >>"$ERRLOG"; exit 125; }
      n=0
      for i in "${!opts[@]}"; do
        if [ "${opts[$i]}" = -v ] && [[ "${opts[$((i + 1))]:-}" == *:/app:ro ]]; then
          opts[$((i + 1))]="$SIZING_NOCF_PYTHON:/app:ro"; n=$((n + 1))
        fi
      done
      [ "$n" = 1 ] || { printf 'shim：full 要恰好一個 /app 的掛載，實際 %s 個\n' "$n" >>"$ERRLOG"; exit 125; }
    fi
    opts+=(-v "$STUB:/acceptance/replay_stub.py:ro" -e "I074_ACCEPTANCE_REPLAY=$SIZING_REPLAY_MODE"
           -e "I074_ACCEPTANCE_COMPUTE=$SIZING_REPLAY_COMPUTE")
    cmd=(python /acceptance/replay_stub.py "${cmd[@]:3}")
  fi
fi

WRAP=(sh -c "$PEAK_WRAPPER" _)
if [ "$PROFILE" = acceptance ]; then
  RSS_WRAPPER="$S/harness/python/scripts/i074_stage2_rss_wrapper.py"
  [ -f "$RSS_WRAPPER" ] || { printf 'shim：快照裡沒有 RSS wrapper：%s\n' "$RSS_WRAPPER" >>"$ERRLOG"; exit 125; }
  opts+=(-v "$RSS_WRAPPER:/acceptance/rss_wrapper.py:ro")
  WRAP=(python -I /acceptance/rss_wrapper.py)
fi

SEQ="$(py seq --state "$S")" || exit 125
ID="$(printf 'o%03d0' "$SEQ")"
CIDFILE="$S/cid/$ID.cid"
PEAKDIR="$S/peak/$ID"
mkdir -p "$S/cid" "$PEAKDIR" "$S/logs" 2>>"$ERRLOG" || exit 125
spec=("${opts[@]}" --cidfile "$CIDFILE" --name "i074sz-${SIZING_RUN_ID:?}-$ID" "${READ_ONLY[@]}"
      -v "$PEAKDIR:/peak" "$IMAGE" "${WRAP[@]}" "${cmd[@]}")

# ⚠️ 不可變索引在**執行前**寫（exclusive create）；寫不了就⛔ 不執行。
py index --state "$S" --sequence "$SEQ" --phase "${SIZING_PHASE:?}" --role "${SIZING_ROLE:?}" \
  --included "${SIZING_INCLUDED:?}" --image "$IMAGE" --profile "$PROFILE" -- "${spec[@]}" >/dev/null || exit 125

CHILD=""
on_signal() {  # $1＝訊號名、$2＝結束碼。⚠️ 以 cidfile 記錄的 CID 強制移除（⛔ 不以名稱猜）。
  if [ -s "$CIDFILE" ]; then "$REAL" rm -f "$(cat "$CIDFILE")" >/dev/null 2>>"$ERRLOG" || true; fi
  [ -n "$CHILD" ] && kill "$CHILD" 2>/dev/null || true
  py sidecar --state "$S" --sequence "$SEQ" --rc "$2" --container "$(cat "$CIDFILE" 2>/dev/null)" \
    --size-rw "" --log-config "" --stdout-log /dev/null --stderr-log /dev/null \
    --peak-file "$PEAKDIR/peak" --memory-limit "" --memory-swap-limit "" --failure "被 $1 中斷" >/dev/null || true
  exit "$2"
}
trap 'on_signal INT 130' INT
trap 'on_signal TERM 143' TERM

py event --state "$S" --id "$ID" --kind create_begin >/dev/null || exit 125
# ⚠️ 背景執行 ＋ wait：bash 要等前景指令結束才處理 trap，那樣中斷時容器來不及移除。
#   stdout／stderr 原樣繼承（I/O 透明）；正式入口⛔ 不用 stdin（沒有 -i）。
"$REAL" run "${spec[@]}" &
CHILD=$!
wait "$CHILD"
RC=$?
CHILD=""

failures=()
CID="$(cat "$CIDFILE" 2>/dev/null || true)"
SIZE_RW=""; LOG_CONFIG=""; MEM_LIMIT=""; SWAP_LIMIT=""
if [ -z "$CID" ]; then
  failures+=(--failure "cidfile 讀不到（容器可能沒有建立）")
else
  SIZE_RW="$("$REAL" inspect --size -f '{{.SizeRw}}' "$CID" 2>>"$ERRLOG")" || failures+=(--failure "inspect --size 失敗")
  LOG_CONFIG="$("$REAL" inspect -f '{{json .HostConfig.LogConfig}}' "$CID" 2>>"$ERRLOG")" \
    || failures+=(--failure "inspect LogConfig 失敗")
  # ⚠️ 兩個值以一個空白分隔；多出的 token 會落進 SWAP_LIMIT、讓它不是整數 → sidecar 記成量測失敗。
  MEM_LIMITS="$("$REAL" inspect -f '{{.HostConfig.Memory}} {{.HostConfig.MemorySwap}}' "$CID" 2>>"$ERRLOG")" \
    || failures+=(--failure "inspect HostConfig.Memory 失敗")
  read -r MEM_LIMIT SWAP_LIMIT <<< "${MEM_LIMITS:-}"
  # ⚠️ docker logs 只導進 S 的檔案給 sidecar 計數，⛔ 不重新輸出。
  "$REAL" logs "$CID" >"$S/logs/$ID.stdout" 2>"$S/logs/$ID.stderr" || failures+=(--failure "docker logs 失敗")
  if "$REAL" rm "$CID" >/dev/null 2>>"$ERRLOG"; then
    py event --state "$S" --id "$ID" --kind rm_done >/dev/null || failures+=(--failure "rm_done 事件寫不進去")
  else
    failures+=(--failure "docker rm 失敗")
  fi
fi
touch "$S/logs/$ID.stdout" "$S/logs/$ID.stderr" 2>>"$ERRLOG" || true
py sidecar --state "$S" --sequence "$SEQ" --rc "$RC" --container "$CID" --size-rw "$SIZE_RW" \
  --log-config "$LOG_CONFIG" --stdout-log "$S/logs/$ID.stdout" --stderr-log "$S/logs/$ID.stderr" \
  --peak-file "$PEAKDIR/peak" --memory-limit "$MEM_LIMIT" --memory-swap-limit "$SWAP_LIMIT" "${failures[@]}" >/dev/null || true
# ⚠️ 傳回原指令的結束碼——量測失敗由 report fail-closed，⛔ 不改寫結束碼。
exit "$RC"
