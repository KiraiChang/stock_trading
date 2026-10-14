#!/usr/bin/env bash
# I-074 Stage 2 步驟 ⑨：image 契約（docs/issue.md「Stage 2 步驟 ⑨ 細部計畫 v1」「六」的 image 契約）。
#
# ⑨ 會 `docker build` 的三個步驟（S3 的 `e1cbbbd` 既有測試、S7 的 smoke 與完整驗證）各用一個**本次執行唯一**的
# 專屬 tag，由入口（`scripts/i074-step9-guards.sh`）在 build 之前 `reserve`、之後（成功、失敗、中斷都一樣）
# `release` 與 `verify`：
#
#   i074-step9-image.sh reserve <tag> <inventory>                           # stdout 印 inventory 的 SHA-256
#   i074-step9-image.sh release <tag> <inventory> <inventory_sha256> <allowance>
#   i074-step9-image.sh verify  <tag> <inventory> <inventory_sha256> <allowance>
#
# ⚠️ **命名空間**：只接受 `stock-trading-i074-step9:<UTC 時間>z-<8 碼 hex>-<步驟>`——`release` 永遠刪不到
#   `stock-trading-python-test:latest` 這類共用或別人的 tag。
# ⚠️ **inventory** 的第一行是本次的 tag（受 SHA 保護），其後是建立前 `docker image ls -a` 的「image ID ＋ tag」；
#   `release`／`verify` 傳入的 tag 必須等於它，inventory 被改即拒絕。
# ⚠️ **release（冪等）**：tag 不存在 → 沒有要收的；本輪新建的 image → 刪除；原本就存在、還有別的 tag → 只解除
#   專屬 tag；原本就存在而且建立前**沒有任何 tag**（cache 命中原本的 dangling image）→ ⛔ 不刪（刪 tag 等於刪掉
#   原本存在的 image），保留專屬 tag，寫一筆 allowance；`docker image rm` 失敗 → 非零、⛔ 不寫 allowance。
# ⚠️ **verify**：⛔ 不信任 allowance 檔——只能是零筆或「本次 tag 的唯一一筆」，而且由 inventory 與目前狀態重算成立；
#   image ID 集合必須與建立前完全相同、tag 集合只多出成立的 allowance。
#
# 結束碼：0 ＝ 成功；1 ＝ 拒絕或失敗（訊息在 stderr）。
set -euo pipefail

STEP9_TAG_RE='^stock-trading-i074-step9:[0-9]{8}t[0-9]{6}z-[0-9a-f]{8}-(e1cbbbd-test|smoke|full)$'
NONE_REF='<none>:<none>'

die() { echo "ERROR: i074-step9-image：$*" >&2; exit 1; }

img_check_tag() {
  [[ "$1" =~ $STEP9_TAG_RE ]] || die "tag ⛔ 不在 ⑨ 的命名空間：$1"
}

# 目前的 image 清單：「ID REF」一行一筆，C locale 排序、去重。docker 失敗 → 非零（⛔ 不當成空清單）。
img_list() {
  local out
  out="$(docker image ls -a --no-trunc --format '{{.ID}} {{.Repository}}:{{.Tag}}')" || return 1
  [ -z "$out" ] || LC_ALL=C sort -u <<< "$out"
}

img_check_inventory() {  # $1＝tag $2＝inventory $3＝sha
  [ -f "$2" ] || die "inventory 不存在：$2"
  [[ "$3" =~ ^[0-9a-f]{64}$ ]] || die "inventory 的 SHA-256 必須是 64 碼 hex"
  [ "$(sha256sum < "$2" | cut -d' ' -f1)" = "$3" ] || die "inventory 在 reserve 之後被改過（SHA 不符）——⛔ 不依它刪除任何東西"
  [ "$(head -n1 "$2")" = "tag $1" ] || die "傳入的 tag ≠ inventory 記錄的本次 tag"
}

cmd_reserve() {
  local tag="$1" inv="$2" now tmp
  img_check_tag "$tag"
  [ ! -e "$inv" ] || die "inventory 已存在（⛔ 不覆寫）：$inv"
  now="$(img_list)" || die "docker image ls 失敗——⛔ 無法盤點"
  if awk -v t="$tag" '$2 == t { found = 1 } END { exit found ? 0 : 1 }' <<< "$now"; then
    die "tag 已存在（⛔ 不覆寫）：$tag"
  fi
  tmp="$inv.tmp.$$"
  { printf 'tag %s\n' "$tag"; [ -z "$now" ] || printf '%s\n' "$now"; } > "$tmp"
  mv -- "$tmp" "$inv"
  # 前次執行留下的 ⑨ tag：列出來（⛔ 不擋執行；結果裡要逐一對到前次的 allowance）。
  awk '$2 ~ /^stock-trading-i074-step9:/ { print "  note 既有的 ⑨ tag：" $2 " → " $1 }' <<< "$now" >&2
  sha256sum < "$inv" | cut -d' ' -f1
}

cmd_release() {
  local tag="$1" inv="$2" sha="$3" allow="$4" now id others before_tags
  img_check_tag "$tag"
  img_check_inventory "$tag" "$inv" "$sha"
  now="$(img_list)" || die "docker image ls 失敗"
  id="$(awk -v t="$tag" '$2 == t { print $1 }' <<< "$now")"
  if [ -z "$id" ]; then
    return 0                       # build 之前就失敗、或已經 release 過
  fi
  if ! awk -v i="$id" 'NR > 1 && $1 == i { found = 1 } END { exit found ? 0 : 1 }' "$inv"; then
    docker image rm "$tag" > /dev/null || die "刪不掉本輪新建的 image（$tag → $id）"
    return 0
  fi
  others="$(awk -v i="$id" -v t="$tag" -v n="$NONE_REF" '$1 == i && $2 != t && $2 != n' <<< "$now")"
  if [ -n "$others" ]; then
    docker image rm "$tag" > /dev/null || die "解除不了專屬 tag（$tag）"
    return 0
  fi
  before_tags="$(awk -v i="$id" -v n="$NONE_REF" 'NR > 1 && $1 == i && $2 != n' "$inv")"
  if [ -n "$before_tags" ]; then
    die "image $id 建立前帶有別的 tag、現在只剩專屬 tag——⛔ 不刪（會刪掉原本存在的 image），請人工確認"
  fi
  touch "$allow"
  grep -qxF "$tag $id" "$allow" || printf '%s %s\n' "$tag" "$id" >> "$allow"
}

cmd_verify() {
  local tag="$1" inv="$2" sha="$3" allow="$4" now
  img_check_tag "$tag"
  img_check_inventory "$tag" "$inv" "$sha"
  now="$(img_list)" || die "docker image ls 失敗"
  printf '%s\n' "$now" | python3 -I -B -c '
import sys
tag, inv, allow, none = sys.argv[1:5]
def parse(lines):
    m = {}
    for line in lines:
        line = line.rstrip("\n")
        if not line:
            continue
        i, ref = line.split(" ", 1)
        m.setdefault(i, set())
        if ref != none:
            m[i].add(ref)
    return m
before = parse(open(inv, encoding="utf-8").readlines()[1:])
now = parse(sys.stdin.readlines())
try:
    entries = [l.rstrip("\n") for l in open(allow, encoding="utf-8") if l.strip()]
except FileNotFoundError:
    entries = []
problems = []
allowed_id = None
if len(entries) > 1:
    problems.append(f"allowance 只能是零筆或本次 tag 的唯一一筆（實際 {len(entries)} 筆）")
elif entries:
    parts = entries[0].split(" ")
    if len(parts) != 2 or parts[0] != tag:
        problems.append(f"allowance 的 tag ⛔ 不是本次的 {tag}：{entries[0]!r}")
    elif parts[1] not in before or before[parts[1]]:
        problems.append(f"allowance 的 image {parts[1]} 建立前⛔ 不是沒有 tag 的既有 image")
    elif now.get(parts[1]) != {tag}:
        problems.append(f"allowance 的 image {parts[1]} 現在的 tag ≠ {{{tag}}}")
    else:
        allowed_id = parts[1]
if set(before) != set(now):
    problems.append(f"image ID 集合與建立前不同：多 {sorted(set(now) - set(before))}、少 {sorted(set(before) - set(now))}")
for i in sorted(set(before) & set(now)):
    want = {tag} if i == allowed_id else before[i]
    if now[i] != want:
        problems.append(f"image {i} 的 tag 與建立前不同：{sorted(before[i])} → {sorted(now[i])}")
for p in problems:
    print("ERROR: i074-step9-image：verify：" + p, file=sys.stderr)
sys.exit(1 if problems else 0)
' "$tag" "$inv" "$allow" "$NONE_REF"
}

main() {
  [ "$#" -ge 1 ] || die "用法：reserve|release|verify …"
  local cmd="$1"; shift
  case "$cmd" in
    reserve) [ "$#" -eq 2 ] || die "用法：reserve <tag> <inventory>"; cmd_reserve "$@" ;;
    release) [ "$#" -eq 4 ] || die "用法：release <tag> <inventory> <inventory_sha256> <allowance>"; cmd_release "$@" ;;
    verify)  [ "$#" -eq 4 ] || die "用法：verify <tag> <inventory> <inventory_sha256> <allowance>"; cmd_verify "$@" ;;
    *) die "未知命令：$cmd" ;;
  esac
}

main "$@"
exit $?
