#!/usr/bin/env bash
# scripts/lib/docker-user.sh 的測試與守門（I-133）。⛔ 不需要真的 docker：用假的 docker 模擬
# rootless／rootful／daemon 連不到三種情況。
#
#   一、判斷：rootless → 不帶 --user；rootful → --user 本機 uid/gid；docker info 失敗 → --user；
#      DOCKER_USER_MODE 覆寫；同一支腳本只問一次 docker info。
#   三、呼叫的位置（review 2026-11-16）：docker_user_args 要緊接在啟動 container 的那一段之前；
#      run-evaluation.sh／run-replay-offline.sh 的 selftest 與偽造參數的拒絕路徑⛔ 不能呼叫 docker
#      （daemon 沒回應時也要立刻結束）。
#   二、守門：repo 追蹤中的 *.sh ⛔ 不能直接寫 `--user "$(id -u)…"`（lib 與本測試除外）；
#      用到 DOCKER_USER_ARGS 的腳本必須 source lib 並呼叫 docker_user_args。
#
# 用法：scripts/test-docker-user.sh；結束碼 0 全部通過、1 有失敗。
# 單引號裡的 $ 是刻意的（要原樣寫進假腳本或交給子 shell），ok／ng 不會失敗
# shellcheck disable=SC2015,SC2016
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LIB="$REPO_ROOT/scripts/lib/docker-user.sh"
WORK="$(mktemp -d "${TMPDIR:-/tmp}/test-docker-user.XXXXXX")"
trap 'rm -rf "$WORK"' EXIT

PASS=0
FAIL=0
ok() { PASS=$((PASS + 1)); echo "  ok   $*"; }
ng() { FAIL=$((FAIL + 1)); echo "  FAIL $*"; }

FAKEBIN="$WORK/bin"
mkdir -p "$FAKEBIN"
cat > "$FAKEBIN/docker" <<'EOF'
#!/bin/bash
# 假 docker：只認 `docker info --format …`；FAKE_INFO=rootless|rootful|fail|hang
echo "docker $*" >> "$CALL_LOG"
[ "${1:-}" = info ] || { echo "fake docker: 沒預期的呼叫：$*" >&2; exit 97; }
case "${FAKE_INFO:-rootful}" in
  rootless) echo '[name=seccomp,profile=builtin name=rootless name=cgroupns]' ;;
  rootful) echo '[name=apparmor name=seccomp,profile=builtin name=cgroupns]' ;;
  fail) echo "Cannot connect to the Docker daemon" >&2; exit 1 ;;
  hang) exec /bin/sleep 30 ;;
esac
EOF
# 卡住的 docker：任何呼叫都記一行然後睡 30 秒（模擬沒有回應的 daemon）
HANGBIN="$WORK/hangbin"
mkdir -p "$HANGBIN"
printf '#!/bin/bash\necho "docker $*" >> "$CALL_LOG"\nexec /bin/sleep 30\n' > "$HANGBIN/docker"
chmod +x "$HANGBIN/docker"
chmod +x "$FAKEBIN/docker"

# probe：在乾淨的子 shell 裡 source lib、呼叫 N 次，印出 rc 與陣列內容（以 | 分隔）
probe() {
  local calls="$1"; shift
  : > "$WORK/calls"
  env -i PATH="$FAKEBIN:/usr/bin:/bin" CALL_LOG="$WORK/calls" "$@" bash -c '
    set -u
    . "$0"
    rc=0
    for _ in $(seq "$1"); do docker_user_args || rc=$?; done
    printf "rc=%s args=%s\n" "$rc" "${DOCKER_USER_ARGS[*]-}"
  ' "$LIB" "$calls" 2>/dev/null
}
ME="--user $(id -u):$(id -g)"

echo "== 一、判斷"
out="$(probe 1 FAKE_INFO=rootless)"
[ "$out" = "rc=0 args=" ] && ok "rootless → 不帶 --user" || ng "rootless：$out"
out="$(probe 1 FAKE_INFO=rootful)"
[ "$out" = "rc=0 args=$ME" ] && ok "rootful → $ME" || ng "rootful：$out"
out="$(probe 1 FAKE_INFO=fail)"
[ "$out" = "rc=0 args=$ME" ] && ok "docker info 失敗 → 照 rootful（$ME，⛔ 不靜默改成 root）" || ng "docker info 失敗：$out"
out="$(probe 1 FAKE_INFO=rootless DOCKER_USER_MODE=user)"
[ "$out" = "rc=0 args=$ME" ] && ok "DOCKER_USER_MODE=user 蓋過偵測" || ng "MODE=user：$out"
out="$(probe 1 FAKE_INFO=rootful DOCKER_USER_MODE=none)"
[ "$out" = "rc=0 args=" ] && ok "DOCKER_USER_MODE=none 蓋過偵測" || ng "MODE=none：$out"
out="$(probe 1 FAKE_INFO=rootful DOCKER_USER_MODE=bogus)"
case "$out" in rc=2*) ok "DOCKER_USER_MODE 不認得的值 → rc 2" ;; *) ng "MODE=bogus：$out" ;; esac
out="$(probe 1 FAKE_INFO=rootless DOCKER_USER_MODE=user)"
if [ ! -s "$WORK/calls" ]; then ok "有覆寫時⛔ 不問 docker info"; else ng "有覆寫仍呼叫了 docker"; fi
probe 3 FAKE_INFO=rootless >/dev/null
n="$(grep -c '^docker info' "$WORK/calls")"
[ "$n" = 1 ] && ok "呼叫 3 次只問 1 次 docker info" || ng "docker info 被問了 $n 次"
t0=$SECONDS
out="$(probe 1 FAKE_INFO=hang DOCKER_INFO_TIMEOUT_SEC=1)"
el=$((SECONDS - t0))
if [ "$out" = "rc=0 args=$ME" ] && [ "$el" -le 3 ]; then
  ok "docker info 沒回應 → ${el} 秒逾時、照 rootful（$ME）"
else
  ng "docker info 沒回應：$out，${el} 秒"
fi
# 空陣列在 set -u 下要能展開（各腳本的寫法）
out="$(env -i PATH="$FAKEBIN:/usr/bin:/bin" CALL_LOG="$WORK/calls" FAKE_INFO=rootless bash -c '
  set -euo pipefail; . "$0"; docker_user_args
  printf "[%s]" ${DOCKER_USER_ARGS[@]+"${DOCKER_USER_ARGS[@]}"} x' "$LIB" 2>&1)"
[ "$out" = "[x]" ] && ok "set -u 下空陣列的展開不報錯、不多出空字串參數" || ng "空陣列展開：$out"

echo "== 二、守門"
# guard DIR：列出違規（直接寫 --user "$(id -u)…"；或用了 DOCKER_USER_ARGS 卻沒 source／呼叫 lib）
guard() {
  local dir="$1" f
  while IFS= read -r f; do
    case "$f" in scripts/lib/docker-user.sh|scripts/test-docker-user.sh) continue ;; esac
    if grep -qF -- '--user "$(id -u)' "$dir/$f"; then echo "直接寫 --user：$f"; fi
    if grep -q 'DOCKER_USER_ARGS' "$dir/$f"; then
      grep -q 'scripts/lib/docker-user.sh"' "$dir/$f" || echo "沒 source lib：$f"
      grep -qE '^[[:space:]]*docker_user_args([[:space:]]|$)' "$dir/$f" || echo "沒呼叫 docker_user_args：$f"
    fi
  done < <(cd "$dir" && { git ls-files --cached --others --exclude-standard '*.sh' 2>/dev/null || find . -name '*.sh' | sed 's#^\./##'; })
}
v="$(guard "$REPO_ROOT")"
if [ -z "$v" ]; then ok "repo 的 *.sh：沒有直接寫的 --user、用到 DOCKER_USER_ARGS 的都有 source 並呼叫 lib"; else ng "守門：$v"; fi
n="$(cd "$REPO_ROOT" && git ls-files '*.sh' | xargs grep -l 'docker_user_args' | grep -vc '^scripts/lib/docker-user.sh$')"
[ "$n" -ge 15 ] && ok "改用 lib 的腳本 $n 支（I-133 盤點的 15 支）" || ng "改用 lib 的腳本只有 $n 支"
# 守門本身：違規的假 repo 要被抓到
F="$WORK/fakerepo"
mkdir -p "$F/scripts"
git -C "$F" init -q
printf '#!/bin/bash\ndocker run --user "$(id -u):$(id -g)" x\n' > "$F/scripts/a.sh"
printf '#!/bin/bash\ndocker run ${DOCKER_USER_ARGS[@]+"${DOCKER_USER_ARGS[@]}"} x\n' > "$F/scripts/b.sh"
git -C "$F" add scripts
v="$(guard "$F")"
if grep -q '直接寫 --user：scripts/a.sh' <<<"$v" && grep -q '沒 source lib：scripts/b.sh' <<<"$v" \
   && grep -q '沒呼叫 docker_user_args：scripts/b.sh' <<<"$v"; then
  ok "（測試本身）守門抓得到直接寫的 --user、沒 source、沒呼叫"
else
  ng "（測試本身）守門沒抓到：$v"
fi

echo "== 三、呼叫的位置"
# placement FILE：每個 docker_user_args 呼叫之後（略過註解與空行）的那一段必須用到 DOCKER_USER_ARGS——
# 陣列 `X=(` 到 `)` 之內，或 `\` 續行的同一個指令之內。印出不符合的行號。
placement() {
  awk '
    function flush() { if (pending && !found) print "第 " callno " 行的呼叫後面沒有緊接著用到 DOCKER_USER_ARGS"; pending=0; mode="" }
    /^[[:space:]]*docker_user_args([[:space:]]|$)/ { flush(); pending=1; found=0; callno=NR; next }
    pending && mode=="" {
      if ($0 ~ /^[[:space:]]*(#|$)/) next
      if ($0 ~ /^[[:space:]]*[A-Za-z_]+=\($/) { mode="array"; next }
      mode="cont"
    }
    pending && mode=="array" { if ($0 ~ /DOCKER_USER_ARGS/) found=1; if ($0 ~ /^[[:space:]]*\)/) flush(); next }
    pending && mode=="cont" { if ($0 ~ /DOCKER_USER_ARGS/) found=1; if ($0 !~ /\\$/) flush(); next }
    END { flush() }
  ' "$1"
}
bad=""
n=0
while IFS= read -r f; do
  case "$f" in scripts/lib/docker-user.sh|scripts/test-docker-user.sh) continue ;; esac
  grep -qE '^[[:space:]]*docker_user_args([[:space:]]|$)' "$REPO_ROOT/$f" || continue
  n=$((n + 1))
  r="$(placement "$REPO_ROOT/$f")"
  [ -z "$r" ] || bad="$bad $f（$r）"
done < <(cd "$REPO_ROOT" && git ls-files --cached --others --exclude-standard '*.sh')
if [ -z "$bad" ] && [ "$n" -ge 15 ]; then ok "$n 支腳本的 docker_user_args 都緊接在啟動 container 之前"; else ng "呼叫位置：$n 支；$bad"; fi
# placement 本身：呼叫排在前面、中間隔著別的指令 → 要抓到；緊接的陣列 → 放過
printf 'docker_user_args\nif [ -n "${X:-}" ]; then exit 0; fi\ndocker run ${DOCKER_USER_ARGS[@]+"${DOCKER_USER_ARGS[@]}"} x\n' > "$WORK/early.sh"
printf 'docker_user_args   # x\n# c\nA=(\n  --rm\n  ${DOCKER_USER_ARGS[@]+"${DOCKER_USER_ARGS[@]}"}\n)\n' > "$WORK/arr.sh"
if [ -n "$(placement "$WORK/early.sh")" ] && [ -z "$(placement "$WORK/arr.sh")" ]; then
  ok "（測試本身）位置檢查抓得到排在前面的呼叫、放過緊接的陣列"
else
  ng "（測試本身）位置檢查不對：early=$(placement "$WORK/early.sh") arr=$(placement "$WORK/arr.sh")"
fi
# 實測：daemon 沒回應時，只驗參數的路徑要立刻結束、⛔ 不呼叫 docker
nodocker() {
  local name="$1" want="$2"; shift 2
  : > "$WORK/hangcalls"
  local t0=$SECONDS rc=0 el c good=0
  env -i PATH="$HANGBIN:/usr/bin:/bin" HOME="$WORK" CALL_LOG="$WORK/hangcalls" REPLAY_ARGS_SELFTEST=1 \
    timeout 20 "$@" >/dev/null 2>&1 || rc=$?
  el=$((SECONDS - t0))
  c="$(wc -l < "$WORK/hangcalls")"
  if [ "$want" = ok ] && [ "$rc" -eq 0 ]; then good=1; fi
  if [ "$want" = reject ] && [ "$rc" -ne 0 ] && [ "$rc" -ne 124 ]; then good=1; fi
  if [ "$good" = 1 ] && [ "$c" -eq 0 ] && [ "$el" -le 5 ]; then
    ok "$name：rc $rc、${el} 秒、docker 呼叫 0 次"
  else
    ng "$name：rc $rc、${el} 秒、docker 呼叫 $c 次"
  fi
}
nodocker "run-evaluation.sh 的 selftest" ok "$REPO_ROOT/scripts/run-evaluation.sh" --symbols 2330
nodocker "run-evaluation.sh 拒絕偽造的 --image-digest" reject "$REPO_ROOT/scripts/run-evaluation.sh" --symbols 2330 --image-digest sha256:evil
nodocker "run-replay-offline.sh 的 selftest" ok "$REPO_ROOT/scripts/run-replay-offline.sh" --bundle /b --output-dir /o --before-ref HEAD
nodocker "run-replay-offline.sh 拒絕偽造的 --source-root" reject "$REPO_ROOT/scripts/run-replay-offline.sh" --bundle /b --output-dir /o --before-ref HEAD --source-root /elsewhere

echo "==> test-docker-user：通過 $PASS、失敗 $FAIL"
[ "$FAIL" -eq 0 ]
