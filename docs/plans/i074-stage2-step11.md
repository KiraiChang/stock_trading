# Stage 2 步驟 ⑪ 細部計畫 v4（2026-10-29；v3 ✅ 確認（第二輪 review 通過；D1～D5 全部採 (a)）；v4 的簡易指令 `s11`：✅ 確認（review 通過；D6、D7 採 (a)）；「十」2、3 ✅ 測試與安裝完成（⚠️ 待 review，「十二」）；歷次修正見「十一」）：B／C 判讀與關閉條件

需求確認（2026-10-29）：規劃 I-074 Stage 2 的 ⑪——以 ⑩ 晉升並已 commit（`078ec72`）的成功 archive 執行 B／C 判讀器，
依 I-074「關閉條件（2026-09-01 改為單一決策樹）」處置判讀結果並歸檔。範圍是**判讀 ＋ 關閉條件**（使用者裁決）；
⑩、⑨-1、⑨-2 計畫書的收斂**併到 I-074 收斂時**處理（使用者裁決）。本計畫確認之前⛔ 不執行判讀器、⛔ 不改任何檔案。

文中的「關閉條件」「處置（2026-09-01 定案）」「正式 scan 的計次裁決」「v29」都指 `docs/issue.md` I-074 的章節；
判讀規則的現況規格在 `sr-zone-scoring.md`「I-074 Stage 2 的晉升與 B／C 判讀器」；判讀器的用法與批次的作業規則在 `development-workflow.md`
「I-074 Stage 2 的正式執行程序」與「I-074 Stage 2 的 memory／disk acceptance harness 與 ⑩ 的 observer」的「作業規則（正式量測）」；
⑩ 的結果在 [`i074-stage2-step10.md`](./i074-stage2-step10.md)「十四」。

## 〇、唯讀盤點（2026-10-29，HEAD `078ec72`）

| 項目 | 現況 |
|---|---|
| evidence | `python/baselines/i074_stage2/evidence/` 7 檔已在 `078ec72` 進版控；檔案清單的錨 `2da23d3a…`、manifest 檔的 SHA `d5cf0629…`（⑩「十四」一）；`finalizer_provenance.base_commit` `832f678` 可達 |
| 判讀器 | `REPLAY_IMAGE_ID=sha256:2a90ad1c… scripts/judge-i074-stage2.sh`：⛔ 沒有參數、⛔ 不取鎖、⛔ 不寫真正 repo、可以重跑。流程：本腳本 ＝ HEAD → 讀 manifest 的 `base_commit` → 確認可達 → `mktemp -d` 下 `git clone --no-hardlinks` ＋ detached checkout `base_commit`（`trap … EXIT` 結束時刪除）→ 暫存複本的 `finalize-stage2-evidence.sh --verify-promotion-staging <真正 repo>/…/evidence --target evidence --judge` → stdout 一行 JSON。結束碼 0 ＝ 已判讀（B 與 C 都是 0）、1 ＝ 拒絕判讀（⛔ 不輸出 B／C）。⚠️ 驗證模式的結束碼只有 0／1：`finalize-stage2-evidence.sh` 把容器的任何非 0（含 OOM 的 137、docker 的 125）都轉成 1，所以 `judge.err` 的「驗證模式不通過（rc=1）」⛔ 不能用來分辨原因 |
| 驗證容器 | `finalize-stage2-evidence.sh` 以 `docker run --rm --network none --memory=<mem-guard 的值>` 啟動，⛔ 沒有名稱與標籤；live 的容器可能用同一個 image，所以只能以指令列含 `--verify-promotion-staging` 辨認，⛔ 不能依 image 辨認或送訊號 |
| 輸出的 contract | 封閉 key set：`kind`（evidence）、`target`、`manifest_sha256`（manifest **檔**的 SHA）、`base_commit`、`identity_sha256`（canonical identity 的 SHA，＝ P3 的 `a02bf3bd…`）、`verdict`。`verdict` 的頂層恰好 `schema`（`i074_stage2_verdict/v1`）、`verdict`（`B`／`C`）、`row_count`、`c_row_count`、`rows`；每列 `symbol`、`timeframe`、`as_of`、`class`（`NON_AVOID`／`AVOID`／`UNCLASSIFIED`）、`verdict`、`reasons`（B ⇔ 空陣列）。任一列 C → 整體 C；0 列 → 拒絕判讀 |
| 記憶體 | 判讀器的驗證與 ⑩ 晉升時的 `--verify-promotion-staging` 是同一段驗證：⑩ 的 observer 量到那個容器的 cgroup 峰值下界 328.0 MiB（`total_rss` 217.4 MiB）、執行約 29 秒。Claude 執行中 MemAvailable 約 780 MiB → mem-guard 上限 ≈ min(700, 780 − 150) ＝ 630m |
| 磁碟 | 暫存複本約 0.27 GB（真正 repo 的 `.git` 222M）；根檔案系統可用 6.3G |
| 批次的作業規則 | 當天**所有排程時間已過的批次**都要以 `/scheduler/status` 確認當天那一筆已是終態（需要 JWT）；只有整輪硬上限的（`corporate_action_sync` 45 分鐘）才可以依上限推定——`evaluation_universe_sync` 等⛔ 沒有硬上限。批次清單：平日 06:30 `stock_symbol_sync`（每日）、`corporate_action_sync`、07:00 `delisting_reconcile`（每日）、08:50 `pre_market`、09:00～13:55 每 5 分鐘 `intraday`、15:00 `daily_close`（接著 `sr_zone_verify`）、16:00 `evaluation_universe_sync`（接著 `candle_gap_detection`）、17:00 `sr_analysis`、22:00 `chip_daily_sync`、23:00 `sr_analysis_chip`（T-075 後移之後）；週末只有每日的 06:30、07:00。來源：`pre_market`、`intraday`、`daily_close` 是程式的固定註冊（T-076 部署之前）；其他都由 config 決定、可以用 env 覆寫——`stock_symbol_sync` 是 `STOCK_SYMBOLS_CRON`（config 預設 `30 6 * * *`）；`sr_evaluation` 是 `SR_EVALUATION_ENABLED`（預設 false）與 `SR_EVALUATION_CRON`（預設 `30 22 * * 1-5`），live ⛔ 沒有啟用（⑩ 的 precheck：`sr_evaluation disabled`、env ⛔ 沒有這兩個鍵）；其餘六個的 `*_CRON` 鍵 live 都有設 |
| 決策樹（「關閉條件」） | A（候選 ＝ 0）已在 Stage 1 排除（候選 156）。**B**：候選 > 0 且 before／after 如預期翻轉 → 記錄全候選的逐列證據與下游影響（含 artifact SHA）、**驗證完成、本筆關閉**。**C**：沒有翻轉或下游欄位不符逐項預期 → **⛔ 不得關閉**，另立新 issue 調查（編號依當時的下一個可用值，⛔ 不預先佔號）。判讀一律依 comparison 的 156 列逐列結果 |
| B 的預期分類（「關閉條件」的判讀矩陣，2026-09-22 以 D+1 artifact 實測） | after 側：非 AVOID **140**、AVOID **16**（分類只看 after 的 `action_state`） |
| 收斂措辭的界線（「處置」） | 定向命中只能證明該路徑**可達且下游行為符合設計**，⛔ 不能推論自然母體的發生率或整體績效影響 |
| I-074 的規模 | `issue.md` 第 332～12134 行（約 1.18 萬行，全檔約八成）；其他文件引用 I-074：`development-workflow.md` 47 處、`sr-zone-scoring.md` 27 處、`todo.md` 10 處；`docs/plans/` 有 ⑩、⑨-2 的三份計畫書 |
| I-074 的未結項 | 狀態欄「已知限制（open）①」（acceptance 成功路徑的 raw 複製失敗時頂層留下看似成功的報告）：明文「I-074 關閉之前要處置（修正，或另立一筆 issue 承接）」；I-119、I-120 是獨立的 issue（內文引用 I-074 的章節） |
| 照實揭露 | ⑩ 的分析時（2026-10-29）Claude 為了對照 ⑨-1 量測趟，對 comparison 做過結構差異比對，看過部分列的 before 欄位；⛔ 沒有做任何 B／C 判定，判讀一律以判讀器的輸出為準 |

## 一、目標、範圍與⛔ 不做

| 項目 | 內容 |
|---|---|
| 目標 | 判讀器執行一次 → 驗證輸出 → 依決策樹處置（B：驗證完成、結論歸檔、I-074 標成已完成／待 review；C：另立 issue、I-074 ⛔ 不關閉；拒絕判讀或異常中止：停下查明）→ 記錄 → review → 使用者 commit |
| 範圍 | 簡易指令 `s11`（v4；`~/.local/bin`，repo 外）、排程狀態（使用者）、判讀前置與執行（Claude）、異常中止的處理、輸出驗證、下游影響的摘要（B、C 都做，D5）、分支處置、紀錄與歸檔 |
| ⛔ 不做 | ⛔ 不改程式、判讀規則、允許清單與門檻；⛔ 不重跑 replay；⛔ 不因結果重跑判讀器或換條件（只有拒絕判讀或異常中止、而且原因是環境時才重跑，見「四」）；⛔ 不碰 live（排程狀態只由使用者查詢，JWT ⛔ 不經過 Claude）；⛔ 不改寫會讓 `832f678` 不可達的歷史；⛔ 不移除 I-074、⛔ 不刪計畫書（D3）；⛔ 不刪 ⑩、⑨-2 的 `<work>`（由使用者決定）；⛔ 不處理 T-075 的範圍 B |

## 二、前置條件（第二段的腳本**逐項自動判定，任一項不成立就停止**、⛔ 不呼叫判讀器）

| # | 條件 | 由誰、怎麼確認 |
|---|---|---|
| J1 | HEAD 包含 evidence 的 commit `078ec72`；真正 repo 的 `python/baselines/i074_stage2/` 與 `scripts/judge-i074-stage2.sh` ⛔ 沒有未 commit、未追蹤或 ignored 的變更 | 腳本（`git` 失敗⛔ 不能當作成立） |
| J2 | evidence 的檔案清單 SHA ＝ `2da23d3a…`、manifest 檔 ＝ `d5cf0629…` | 腳本 |
| J3 | `832f678` 是 HEAD 的祖先 | 腳本 |
| J4 | image `sha256:2a90ad1c…` 在本機 | 腳本 |
| J5 | MemAvailable ≥ **650 MiB**（mem-guard 上限 ≥ 500m；⑩ 的同一段驗證峰值下界 328.0 MiB） | 腳本 |
| J6 | 沒有殘留與並行：sentinel ⛔ 不存在；帶 `i074.stage2.run` 鍵的容器 ⛔ 沒有；指令列含 `--verify-promotion-staging` 的容器 ⛔ 沒有；⑩ 的入口、supervisor、observer、判讀器、`finalize-stage2-evidence.sh` 的程序 ⛔ 沒有；暫存目錄（v4：`s11 judge` 新建的 `$J/tmp`）存在而且是空的 | 腳本（任一個指令失敗⛔ 不能當作沒有） |
| J7a | live 的排程鍵 ＝ 「〇」批次清單的前提：`*_CRON` 鍵的**整組**恰好是 `CHIP_SYNC_CRON=0 22 * * 1-5`、`CORPORATE_ACTION_CRON=30 6 * * 1-5`、`DELISTING_CRON=0 7 * * *`、`EVALUATION_UNIVERSE_CRON=0 16 * * 1-5`、`SR_ANALYSIS_CHIP_CRON=0 23 * * 1-5`、`SR_ANALYSIS_CRON=0 17 * * 1-5` 這六行（多一個鍵——例如 `STOCK_SYMBOLS_CRON`、`SR_EVALUATION_CRON` 的覆寫——或任一行不同都⛔ 不成立）；`SR_EVALUATION_ENABLED` ⛔ 沒有設或是 `false`；`/scheduler/status` 的每一列與 running 的數量 | **使用者**（第一段；JWT ⛔ 不經過 Claude）：輸出存進 tmpfs 的狀態檔；⛔ 不成立就停止、重新確認批次清單 |
| J7b | 狀態檔：一般檔、mode 600、格式完整、結束時刻距今 ≤ 15 分鐘、「J7 排程鍵 ok」、「J7 running 的數量： 0」、`sr_evaluation` 那一列是 `disabled`；**當天排程時間已過的工作**（依「〇」的批次清單與狀態檔的時刻）都有當天那一筆、而且⛔ 不是 `running`；其中定時的工作（`sr_zone_verify`、`candle_gap_detection` 接在前一個之後、`intraday` 每 5 分鐘，這三個除外）的 `started_at` 時:分 ＝ 清單的時刻（以實際紀錄驗證 config 的預設值，例如 `stock_symbol_sync` 的 06:30） | 腳本 |
| J7c | 時段：從狀態檔的時刻到「現在 ＋ 20 分鐘」之間⛔ 沒有任何批次的開始時刻（平日 08:50～14:00 整段視為批次中——`pre_market` 與每 5 分鐘的 `intraday`） | 腳本 |

J7b 加上 J7c 等於作業規則的要求：開始時⛔ 沒有任何工作在執行、當天該跑的都已是終態，而判讀器執行的期間⛔ 不會有新的批次開始。

## 三、執行（D1：第一段由使用者、第二段由 Claude；v4 起兩段都用簡易指令 `s11`；輸出保存依 D2）

| 步 | 誰 | 做什麼 |
|---|---|---|
| 0 | 使用者 | 「三之二」的核對指令（一行）：`~/.local/bin/s11` ＝ HEAD 的本檔「三之一」，印出 SHA-256 與 ok；⛔ 沒有 ok 就⛔ 不用 `s11` |
| 1 | 使用者 | `s11 status`：第一段（J7a）。提示出現之後先打一個空白、再貼上 JWT、按 Enter（⛔ 不回顯、⛔ 不寫進檔案）；最後一行「ST 保存 ok：<路徑>」——把路徑告訴 Claude |
| 2 | Claude | `s11 judge <ST>`（背景執行、結束時通知；第一段結束之後 15 分鐘之內）：J1～J7c 逐項判定、任一項失敗即停止（⛔ 不呼叫判讀器）→ 判讀器一次 → 輸出原樣保存在 `~/i074_stage2_formal/judge-<UTC>/`。D1 若改成 (b)，使用者自己執行同一個指令 |

與 v3 兩段區塊的差異（v4；⛔ 沒有其他改動——兩個函式的內容逐字取自 v3 的 `i074_stage2_step11_status_001` 與 `i074_stage2_step11_judge_002`）：

* 兩段包成同一支腳本的兩個函式（`s11_status`、`s11_judge`）；第一段的 `cd` 失敗改成停止。
* 執行時記下自己的 SHA-256：狀態檔的第二行 `s11_sha256 <SHA>`、`precheck.txt` 的第一行——Claude 事後與 HEAD 的本檔對照（D6）。
* 判讀器的暫存目錄改成 `$J/tmp`：`s11 judge` 在新建的 `$J` 裡建（mode 700），所以一定是新的空目錄，異常中止時的殘留也就在 `$J` 裡；⛔ 不再需要事先建 `judge-tmp`（D7）。
* J7b、J7c 的逐項明細以 `tee -a` 同時寫進 `precheck.txt`（`pipefail` 之下，檢查失敗照樣停止）。
* `s11 judge` 以 `/dev/shm/s11.lock`（`flock -n`）防止兩個同時執行；鎖⛔ 不傳給判讀器（`9>&-`）。名稱⛔ 不是 `i074-*`，⛔ 不影響 ⑩ 類的殘留檢查。
* 以腳本檔執行，所以 J6 的 awk ⛔ 不會比對到自己（v3 要求寫成腳本檔的理由就此滿足）。

`s11 judge` 跑完（或停止）之後，Claude 把 tmpfs 的 `$ST` 與 `$J/status.txt` 以 `cmp` 確認相同，才刪 `$ST`（字面路徑）；
`s11 judge` 在 `$J/status.txt` 之前就停止（⛔ 沒有那個檔）→ 先把 `$ST` 複製進 `$J`、`cmp` 相同之後才刪。

## 三之一、腳本（`~/.local/bin/s11`）

```text id="i074_stage2_s11_script_001"
#!/bin/bash
# s11：I-074 Stage 2 ⑪ 的簡易指令（docs/plans/i074-stage2-step11.md「三之一」）
#   s11 status        第一段（J7a，使用者）：排程鍵與 /scheduler/status。提示出現之後先打一個空白、再貼上 JWT、按 Enter
#                     （⛔ 不回顯、⛔ 不寫進檔案）；最後一行是「ST 保存 ok：<路徑>」——把路徑告訴 Claude
#   s11 judge <ST>    第二段（D1(a)：Claude 在背景執行）：J1～J7c 逐項判定、任一項失敗即停止（⛔ 不呼叫判讀器）
#                     → 判讀器一次 → 輸出原樣保存在 ~/i074_stage2_formal/judge-<UTC>/
# ⛔ 不放進 repo；用之前先以計畫書「三之二」的核對指令確認與 HEAD 的計畫書相同。
set -u -o pipefail
SELF="$(readlink -f -- "$0")" && SHA="$(sha256sum < "$SELF" | cut -d' ' -f1)" && [[ "$SHA" =~ ^[0-9a-f]{64}$ ]] \
  || { echo "算不出 s11 的 SHA-256：⛔ 不執行" >&2; exit 1; }
R=/home/dev/workspace/stock_trading
IMG=sha256:2a90ad1c1dd801d59373988a4d84afe1437cbee62ccab3709b06cfa8f7da5027
usage() { sed -n '2,7s/^# \{0,1\}//p' "$SELF"; exit 2; }

s11_status() {   # 第一段（v3 的 i074_stage2_step11_status_001，逐字；只加 cd 失敗即停、狀態檔第二行的 s11_sha256）
local ST RM RT EV RE CR EX SV BIP TOKEN
cd "$R" || { echo "cd 失敗：⛔ 不執行"; return 1; }
ST="$(mktemp "/dev/shm/s11-status-$(date -u +%Y%m%dT%H%M%SZ)-XXXXXX.txt")"; RM=$?
{
echo "ST $ST"; echo "s11_sha256 $SHA"; date -u +'status 開始 %FT%TZ'
EV="$(docker inspect stock_trading-backend-1 --format '{{range .Config.Env}}{{println .}}{{end}}')"; RE=$?
grep -E '^[A-Z_]+_(CRON|ENABLED)=' <<< "$EV"                                                    # 只印排程鍵
CR="$(grep -E '^[A-Z_]+_CRON=' <<< "$EV" | LC_ALL=C sort)"                                       # *_CRON 的整組（多一個覆寫也⛔ 不成立）
EX="$(printf '%s\n' 'CHIP_SYNC_CRON=0 22 * * 1-5' 'CORPORATE_ACTION_CRON=30 6 * * 1-5' 'DELISTING_CRON=0 7 * * *' \
  'EVALUATION_UNIVERSE_CRON=0 16 * * 1-5' 'SR_ANALYSIS_CHIP_CRON=0 23 * * 1-5' 'SR_ANALYSIS_CRON=0 17 * * 1-5' | LC_ALL=C sort)"
SV="$(grep -E '^SR_EVALUATION_ENABLED=' <<< "$EV")"                                              # sr_evaluation ⛔ 不得啟用
unset EV; [ "$RE" = 0 ] && [ -n "$CR" ] && [ "$CR" = "$EX" ] && { [ -z "$SV" ] || [ "$SV" = SR_EVALUATION_ENABLED=false ]; } \
  && echo "J7 排程鍵 ok"                                                                          # EV ⛔ 不印（含金鑰）
BIP="$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}} {{end}}' stock_trading-backend-1 | cut -d' ' -f1)"
echo "J7：先打一個空白、再貼上 JWT，按 Enter（⛔ 不回顯、⛔ 不寫進檔案或紀錄）"; read -rs TOKEN
curl -fsS -H "Authorization: Bearer $TOKEN" "http://$BIP:8080/api/v1/scheduler/status" | python3 -c 'import json, sys
jobs = json.load(sys.stdin)["jobs"]
for j in sorted(jobs, key=lambda j: j["job_name"]):
    print("J7", j["job_name"], j["status"], j.get("started_at") or "-", j.get("finished_at") or "-")
print("J7 running 的數量：", sum(j["status"] == "running" for j in jobs))'; unset TOKEN
date -u +'status 結束 %FT%TZ'
} 2>&1 | tee "$ST"; RT=$?; [ "$RM" = 0 ] && [ "$RT" = 0 ] && [ -s "$ST" ] && [ "$(head -n 1 "$ST")" = "ST $ST" ] \
  && [ "$(grep -c '^status 結束 ' "$ST")" = 1 ] && grep -q '^status 結束 ' <<< "$(tail -n 1 "$ST")" && echo "ST 保存 ok：$ST"
}

s11_judge() {   # 第二段（v3 的 i074_stage2_step11_judge_002，逐字；差異見「三」的說明）
local ST="$1" J TMPD SE SEC AGE U L A C1 C2 PP P6 RC
cd "$R" || { echo "cd 失敗：⛔ 不執行"; exit 1; }
exec 9>> /dev/shm/s11.lock && flock -n 9 || { echo "另一個 s11 judge 正在執行（或鎖開不了）：⛔ 不執行"; exit 1; }   # 同時只能一個
J="$HOME/i074_stage2_formal/judge-$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -m 700 "$J" || { echo "mkdir $J 失敗：⛔ 不執行"; exit 1; }                                   # ⛔ 不碰既有目錄
say() { { printf '%s\n' "$*" >> "$J/precheck.txt"; } 2>/dev/null || { echo "寫 $J/precheck.txt 失敗：⛔ 不執行" >&2; exit 1; }; printf '%s\n' "$*"; }
die() { say "⚠️ 停止（⛔ 沒有呼叫判讀器）：$*"; exit 1; }
say "J 開始 $(date -u +%FT%TZ)；J=$J；ST=$ST；s11_sha256=$SHA"
TMPD="$J/tmp"; mkdir -m 700 "$TMPD" || die "建不了暫存目錄 $TMPD"                              # 判讀器的 TMPDIR：新的空目錄
# ── J7b：狀態檔 ──
[ -f "$ST" ] && [ ! -L "$ST" ] && [ "$(stat -c %a "$ST")" = 600 ] || die "J7b 狀態檔不存在、是 symlink 或 mode ≠ 600"
[ "$(head -n 1 "$ST")" = "ST $ST" ] && [ "$(grep -c '^status 結束 ' "$ST")" = 1 ] \
  && grep -q '^status 結束 ' <<< "$(tail -n 1 "$ST")" || die "J7b 狀態檔的格式不完整"
cp -p "$ST" "$J/status.txt" && cmp -s "$ST" "$J/status.txt" || die "J7b 狀態檔的保存失敗"
SE="$(sed -n 's/^status 結束 //p' "$ST")"; SEC="$(date -u -d "$SE" +%s)" || die "J7b 讀不到狀態檔的時刻"
AGE=$(( $(date -u +%s) - SEC )); [ "$AGE" -ge 0 ] && [ "$AGE" -le 900 ] || die "J7b 狀態檔距今 $AGE 秒（上限 900）"
grep -qxF 'J7 排程鍵 ok' "$ST" || die "J7b 排程鍵⛔ 不符預期（批次清單的前提不成立）"
grep -qxF 'J7 running 的數量： 0' "$ST" || die "J7b 有工作仍在執行"
python3 -I - "$ST" "$SEC" <<'PY' | tee -a "$J/precheck.txt" || die "J7b 當天排程時間已過的工作⛔ 不齊或⛔ 不是終態"
import datetime as dt, sys
tz = dt.timezone(dt.timedelta(hours=8))
t = dt.datetime.fromtimestamp(int(sys.argv[2]), tz)
due = [("stock_symbol_sync", "06:30"), ("delisting_reconcile", "07:00")]
if t.weekday() < 5:
    due += [("corporate_action_sync", "06:30"), ("pre_market", "08:50"), ("intraday", "09:00"), ("daily_close", "15:00"),
            ("sr_zone_verify", "15:00"), ("evaluation_universe_sync", "16:00"), ("candle_gap_detection", "16:00"),
            ("sr_analysis", "17:00"), ("chip_daily_sync", "22:00"), ("sr_analysis_chip", "23:00")]
rows = {}
for line in open(sys.argv[1], encoding="utf-8"):
    p = line.split()
    if len(p) == 5 and p[0] == "J7":
        rows[p[1]] = (p[2], p[3])
chained = {"sr_zone_verify", "candle_gap_detection", "intraday"}       # 接在前一個之後、或每 5 分鐘：⛔ 不比時:分
bad = 0
if rows.get("sr_evaluation", ("（沒有這一列）",))[0] != "disabled":
    bad += 1; print("J7b sr_evaluation ⛔ 不是 disabled：", rows.get("sr_evaluation"))
for job, hhmm in due:
    if t.strftime("%H:%M") < hhmm:
        continue
    status, started = rows.get(job, ("（沒有這一列）", "-"))
    ok = status != "running" and started.startswith(t.date().isoformat()) and (job in chained or started[11:16] == hhmm)
    bad += not ok
    print("J7b", job, hhmm, status, started, "ok" if ok else "⚠️ ⛔ 不成立")
sys.exit(1 if bad else 0)
PY
say "J7b ok（狀態檔 $AGE 秒前）"
# ── J7c：狀態檔的時刻 → 現在 ＋ 20 分鐘之間⛔ 沒有批次的開始時刻 ──
python3 -I - "$SEC" <<'PY' | tee -a "$J/precheck.txt" || die "J7c 時段⛔ 不可用"
import datetime as dt, sys, time
tz = dt.timezone(dt.timedelta(hours=8))
t = dt.datetime.fromtimestamp(int(sys.argv[1]), tz).replace(second=0, microsecond=0)
end = dt.datetime.fromtimestamp(time.time() + 20 * 60, tz)
daily, weekday = {"06:30", "07:00"}, {"06:30", "07:00", "15:00", "16:00", "17:00", "22:00", "23:00"}
while t <= end:
    hm, wd = t.strftime("%H:%M"), t.weekday() < 5
    if hm in (weekday if wd else daily) or (wd and "08:50" <= hm <= "14:00"):
        print("J7c 批次的時刻", t.isoformat()); sys.exit(1)
    t += dt.timedelta(minutes=1)
PY
say "J7c ok"
# ── J1～J6 ──
git merge-base --is-ancestor 078ec72 HEAD || die "J1 HEAD ⛔ 不含 078ec72"
U="$(git status --porcelain --untracked-files=all --ignored -- python/baselines/i074_stage2 scripts/judge-i074-stage2.sh)" && [ -z "$U" ] || die "J1 ⛔ 不乾淨：$U"
L="$(cd python/baselines/i074_stage2/evidence && find . -type f -print0 | sort -z | xargs -0 sha256sum)" || die "J2 清單失敗"
[ "$(printf '%s\n' "$L" | sha256sum | cut -d' ' -f1)" = 2da23d3aa83c73afee582b1f24e39653648783062dcb6d0f88d73d35cfdf6f9c ] \
  && [ "$(sha256sum < python/baselines/i074_stage2/evidence/evidence_manifest.json | cut -d' ' -f1)" = d5cf06298b4de0875bb00614352e5cf8b6b98450d0b72d6dec8e63458cda5e0e ] \
  || die "J2 evidence 的錨⛔ 不符"
git merge-base --is-ancestor 832f678acfa429f8e4fdba2c82eb77eae826def8 HEAD || die "J3 base_commit ⛔ 不可達"
docker image inspect "$IMG" > /dev/null || die "J4 image ⛔ 不在本機"
awk '/^MemAvailable:/ { m = int($2 / 1024); f = 1 } END { printf "J5 MemAvailable %d MiB（門檻 650）\n", m; exit !(f && m >= 650) }' /proc/meminfo \
  | tee -a "$J/precheck.txt" || die "J5 ⛔ 不成立"
A=/run/lock/i074-stage2.active; { [ -e "$A" ] || [ -L "$A" ]; } && die "J6 sentinel 存在"
C1="$(docker ps -a --filter label=i074.stage2.run -q)" || die "J6 docker ps 失敗"; [ -z "$C1" ] || die "J6 帶 i074.stage2.run 的容器：$C1"
C2="$(docker ps -a --no-trunc --format '{{.ID}} {{.Command}}')" || die "J6 docker ps 失敗"
C2="$(awk '/--verify-promotion-staging/' <<< "$C2")" || die "J6 awk 失敗"; [ -z "$C2" ] || die "J6 驗證容器仍在：$C2"
PP="$(ps -eo pid,args)" && [ -n "$PP" ] || die "J6 ps 失敗"
P6="$(awk '/run-i074-stage2\.sh|i074-stage2-supervisor\.py|i074_stage2_sizing\.py observe|judge-i074-stage2\.sh|finalize-stage2-evidence\.sh/' <<< "$PP")" || die "J6 awk 失敗"
[ -z "$P6" ] || die "J6 程序：$P6"
[ -d "$TMPD" ] && [ ! -L "$TMPD" ] && [ -z "$(ls -A "$TMPD")" ] || die "J6 暫存目錄 $TMPD 不存在、是 symlink 或⛔ 不是空的"
say "J1～J6 ok"
# ── 判讀器（一次）：輸出檔先開好——開不了就停止，⛔ 不讓重導失敗混成判讀器的 rc 1 ──
exec 3> "$J/verdict.json" 4> "$J/judge.err" || die "輸出檔開不了"
say "判讀器開始 $(date -u +%FT%TZ)"
TMPDIR="$TMPD" REPLAY_IMAGE_ID="$IMG" scripts/judge-i074-stage2.sh >&3 2>&4 9>&-; RC=$?
exec 3>&- 4>&-
printf '%s\t%s\n' "$RC" "$(date -u +%FT%TZ)" > "$J/rc.tmp" && mv "$J/rc.tmp" "$J/rc" || { echo "寫 rc 失敗（判讀器的 rc $RC）" >&2; exit 2; }
say "判讀器結束 rc $RC；暫存目錄：$(ls -A "$TMPD" | wc -l) 項"
}

case "${1:-}" in
  status) [ "$#" = 1 ] || usage; s11_status ;;
  judge)  [ "$#" = 2 ] || usage; s11_judge "$2" ;;
  *)      usage ;;
esac
```

## 三之二、核對指令（`s11 status` 之前，使用者執行；一行）

```text id="i074_stage2_s11_check_002"
cd /home/dev/workspace/stock_trading && P="$(git show HEAD:docs/plans/i074-stage2-step11.md)" && [ -n "$P" ] && [ "$(grep -cxF '```text id="i074_stage2_s11_script_001"' <<< "$P")" = 1 ] && B="$(sed -n '/^```text id="i074_stage2_s11_script_001"$/,/^```$/p' <<< "$P" | sed '1d;$d')" && [ -n "$B" ] && [ "$(head -n 1 <<< "$B")" = '#!/bin/bash' ] && [ -s ~/.local/bin/s11 ] && cmp <(printf '%s\n' "$B") ~/.local/bin/s11 && sha256sum ~/.local/bin/s11 && echo "s11 ＝ HEAD 的計畫書 ok"
```

每一步都以 `&&` 串接，任一步不成立就⛔ 不印 ok：`git show` 成功而且非空 → 開頭的那一行在計畫書裡**恰好一次** → 抽出的區塊非空、第一行是 `#!/bin/bash` → `~/.local/bin/s11` 非空 → `cmp` 相同 → 印 SHA-256 與 ok（v4 第一輪 review：只靠管線的最後一段，`git` 失敗或區塊不存在而本機是空檔時都會誤印 ok）。
⛔ 沒有印出最後的 ok → ⛔ 不用 `s11`（交給 Claude 重新安裝）。

## 四、輸出驗證與判讀（依序；任一項不符就停下回報）

| # | 檢查 | 不符時 |
|---|---|---|
| V0 | 第二段的腳本以 0 結束、`$J/rc` 存在、值是 0 或 1 | 腳本在呼叫判讀器之前停止（`precheck.txt` 有「⚠️ 停止」那一行）→ 依原因處理（時段、記憶體、殘留……），⛔ 沒有計入任何嘗試，排除之後重跑兩段。**異常中止**——`rc` 不存在（執行者被殺）或 ≥ 128（例如判讀器本身被 SIGKILL 的 137：它的 `trap` ⛔ 不會執行、暫存複本留在 `$J/tmp`，驗證容器（`--rm`）可能仍在跑到結束）：① 以腳本檔查詢，確認判讀器與 `finalize-stage2-evidence.sh` 的程序、指令列含 `--verify-promotion-staging` 的容器都已不在；仍在 → 等待（最多 30 分鐘，之後停下回報），⛔ 不對其他容器或程序送訊號。② 記下 `judge.err`、`$J/tmp` 的殘留與 `dmesg`／`journalctl -k` 的 OOM 紀錄（讀不到就照實記錄）。③ 確認都已結束之後才刪 `$J/tmp` 的殘留（字面路徑；`$J` 的其他檔原樣保存）。④ 原因是記憶體或環境 → 排除之後以新的 `s11 status`、`s11 judge`（新的 `judge-<UTC>`）重跑一次；同一個原因連續 2 次、或原因不明 → 停下回報 |
| V1 | `rc` ＝ 0 | **rc 1（拒絕判讀）**：判讀器在驗證模式失敗時一律印「驗證模式不通過（rc=1）」——`finalize-stage2-evidence.sh` 已把容器的任何非 0（含 OOM 的 137）轉成 1，**⛔ 不能只憑這一行判定原因**。另找診斷證據：`judge.err` 在那一行之前的內容（Python 的 `ERROR:` 訊息或 traceback ＝ 驗證本身拒絕；mem-guard 那一行之後就中斷、⛔ 沒有任何訊息 ＝ 疑似被殺）、`docker events --since <判讀器開始> --until <結束> --filter event=oom`（daemon 只保留有限的事件）、`dmesg`／`journalctl -k` 的 cgroup OOM 紀錄（讀不到就照實記錄）。有直接證據的環境原因（OOM、docker 暫時錯誤、磁碟）→ 排除之後重跑一次（同一個原因連續 2 次就停下回報）；完整性或信任根被拒 → 停下回報、另立 issue；**原因不明 → 停下回報**、⛔ 不自動重跑；⛔ 沒有 B／C |
| V2 | `verdict.json` 恰好一行、是 JSON；頂層 key set ＝ `kind`、`target`、`manifest_sha256`、`base_commit`、`identity_sha256`、`verdict`；`kind` ＝ evidence、`target` ＝ evidence | 停下回報 |
| V3 | `manifest_sha256` ＝ `d5cf0629…`、`base_commit` ＝ `832f678…`、`identity_sha256` ＝ `a02bf3bd…` | 停下回報 |
| V4 | `verdict.schema` ＝ `i074_stage2_verdict/v1`；`row_count` ＝ 156 ＝ `rows` 的長度；`c_row_count` ＝ `verdict` 為 C 的列數；`verdict` ＝ B ⇔ `c_row_count` ＝ 0 | 停下回報 |
| V5 | `class` 的分佈：非 AVOID 140、AVOID 16、UNCLASSIFIED 0（「關閉條件」的判讀矩陣；分類只看 after 側，after 側來自已封存的 Stage 1 D+1 artifact） | ⛔ 不論 B／C 都停下回報：分類與已記錄的事實矛盾 |

通過之後，**判讀結果以 `verdict.verdict` 為準**（⛔ 不以人工或其他統計改判）。

## 五、下游影響的摘要（D5；B、C 都做；判讀之後、只描述、⛔ 不參與 B／C 的判定）

以唯讀的一次性腳本（scratchpad）讀已 commit 的 `comparison/comparison_artifact.json.gz` 與 `verdict.json`，統計下列**預先定義**的表，連同腳本的 SHA-256 記進結果：

1. 類別（after 的 `action_state`）× before 的 `lifecycle_phase` 的列數。
2. `action_state`（＝ `position_action_condition.state`）的 before → after 轉換列數——非 AVOID 而 before 是 `TESTING` 的那一格就是持倉建議實際改變的列（「關閉條件」的 `position_impact_candidate` 定義）。
3. `market_bias`、`final_entry_state`、`rr_decoupling_candidate` 的 before → after 轉換列數。
4. top-level `position_action` 有差異的列數。
5. 標的與 as_of 的分佈（每檔列數、最早與最晚的 as_of）。
6. 判讀器每列的 `verdict` 與 `reasons` 的分佈（C 時另列前 20 列 C 的 `symbol`、`as_of`、`class`、`reasons`）。

**B 時**另做交叉核對：判讀 B 而第 4 項 > 0、或第 2、3 項出現「關閉條件」B 矩陣以外的轉換 → 停下回報（判讀器或摘要有一方錯）。
**C 時**只照實描述各表的實際值，⛔ 不預設只有 B 才成立的等式（例如第 4 項 ＝ 0），也⛔ 不據此推論 C 的成因——成因由新 issue 的調查計畫處理。

## 六、分支處置

| 結果 | 處置 |
|---|---|
| **B** | ① `sr-zone-scoring.md`「已知並接受的行為改變」：標題與內文補上定向驗證的結論（下方措辭）、數字（13,417 列、候選 156、分類與「五」的表）、證據的錨（artifact SHA、evidence 清單的錨、判讀器輸出的 SHA）；「I-074 Stage 2 的晉升與 B／C 判讀器」補 ⑪ 的正式值。② `issue.md` I-074：狀態列改成「Stage 2 完成、判讀 B：**驗證完成**（⚠️ 待 review；本筆的移除依 D3 另立收斂計畫）」、roadmap 的 ⑪ → 結果；「已知限制（open）①」依 D4 處置。③ 本檔記錄結果 |
| **C** | ⛔ 不得關閉 I-074。① 另立新 issue（當時的下一個可用編號）：判讀器的輸出摘要、「五」的表（照實描述）、證據的錨；② I-074 狀態列與 roadmap 指向它；③ 本檔記錄結果。⛔ 不寫收斂措辭、⛔ 不改判讀規則；後續調查另立計畫 |
| **拒絕判讀（rc 1）或異常中止** | 依「四」V0、V1；⛔ 沒有 B／C；I-074 狀態不變（只記錄嘗試） |

**B 的收斂措辭**（預先寫定，判讀之後⛔ 不改寫其界線）：

> 此行為改變有單元測試保護，並經一次預先界定的有界定向驗證（I-074 Stage 2，2026-10-28 執行）確認：
> 凍結 bundle 的 13,417 列中有 156 列符合完整 predicate，before（RR 條件仍在的 counterfactual）→ after 的逐列變化
> 全部符合預先定義的判讀矩陣（判讀器 B）。這只證明該路徑在真實資料上**可達、且下游行為符合設計**；
> ⛔ 不能據此推論自然母體的發生率或整體績效影響。

## 七、紀錄、歸檔與 commit

* 本檔：新增「十三、執行結果」（「十二」是「十」2、3 的測試與安裝；狀態檔與前置條件原樣、每一次嘗試、判讀器的 rc 與輸出摘要、V0～V5、「五」的表、分支處置）；標題標成「已執行／待 review」。
* `issue.md`、`sr-zone-scoring.md`：依「六」。`development-workflow.md`：判讀器一節補一行「⑪ 的執行紀錄見本檔」。
* 判讀器的完整輸出依 D2 保存；⛔ 不進 repo。
* Claude ⛔ 不 commit：停下等 review；commit 指令由使用者執行。
* 收斂：本檔與 ⑩、⑨-1、⑨-2 的計畫書依 D3 在 I-074 收斂時一起處理（刪除之前必須已經 commit 過）。

## 八、風險

| 風險 | 處置 |
|---|---|
| 前置條件不成立卻呼叫了判讀器 | 第二段逐項判定、失敗即停止；J7 也自動判定（⛔ 沒有中途人工確認）；保存失敗也停止；輸出檔先開好，重導失敗⛔ 不會混成判讀器的 rc 1 |
| 與批次重疊 | J7a（使用者查 `/scheduler/status`）＋ J7b（狀態檔 ≤ 15 分鐘、running 0、當天該跑的都已是終態）＋ J7c（到「現在 ＋ 20 分鐘」⛔ 沒有批次的開始時刻）；排程鍵⛔ 不符預期（`*_CRON` 的整組多一個或任一行不同、`sr_evaluation` 啟用）就停止（批次清單的前提不成立）；J7b 另以當天的 `started_at` 時:分驗證 config 預設的時刻與固定註冊的時刻。⚠️ 照實的界線：當天還沒跑到的工作，只能靠排程鍵與 config 的預設推定（live image 的 config 預設⛔ 無法在這裡直接讀）；固定註冊的時刻（`pre_market`、`intraday`、`daily_close`）來自程式，T-076 部署之後要重新確認 |
| 判讀器拒絕判讀 | V1：環境原因排除之後重跑一次；完整性原因停下另立 issue |
| 記憶體（2 GiB host） | J5（650 MiB）；容器有 mem-guard 上限。驗證容器被 OOM 砍掉 → 判讀器照常收尾、rc 1，訊息與其他驗證失敗相同，原因要另找證據（V1）。判讀器本身被 SIGKILL → 137、`trap` ⛔ 不執行、暫存複本與 `--rm` 容器可能還在；執行者也被殺 → ⛔ 沒有 rc 檔——兩者都走 V0 的異常中止：先確認程序與容器都已結束，才清理、才決定重跑 |
| 判讀器或規則的錯誤讓 B／C 誤判 | 規則在 ⑩ 之前已寫成程式並 review（⑦c 四輪）；V2～V5 的形狀、錨與分類檢查；B 時「五」與判讀器交叉核對，不一致就停下 |
| 看過結果之後改條件（事後合理化） | 判讀規則、允許清單、收斂措辭都在執行之前寫定；⛔ 不因結果重跑或改判 |
| 暫存複本殘留 | 判讀器的 `trap` 刪除；`TMPDIR` 指向 `s11 judge` 新建的 `$J/tmp`（v4）；結束時印出它的項目數；異常中止時依 V0 處理 |
| `s11` 與計畫書的區塊不同步、或安裝之後被改動（v4） | `s11` 的兩個函式由 v3 的兩段區塊逐字組成（差異只有「三」列出的幾項）；「三之二」的核對指令（對 HEAD 的本檔 `cmp`）；狀態檔與 `precheck.txt` 記下 `s11_sha256`，Claude 事後再對一次（D6） |
| 兩個 `s11 judge` 同時執行（v4） | `/dev/shm/s11.lock` 的 `flock -n`；J6 也查判讀器的程序 |
| JWT 外洩 | 第一段由使用者執行、`read -rs`、⛔ 不寫進檔案；狀態檔只有排程鍵與工作的狀態（`EV` ⛔ 不印） |
| 收斂措辭過度宣稱 | 措辭預先寫定、引用「處置」的界線 |
| `832f678` 變成不可達 | ⛔ 不改寫歷史（含 force push）；J3 |
| 回滾 | 只動文件與 repo 外的輸出：文件以 git 回復；判讀器可以從已 commit 的 evidence 重跑；`s11` 有問題就刪掉 `~/.local/bin/s11`（與 `/dev/shm/s11.lock`），改由 Claude 從本檔抽出腳本執行 |

## 九、決定（✅ 使用者裁決 2026-10-29：D1～D5 全部採 (a)，與兩輪 review 的建議相同）

| # | 決定 | 選項 | 理由與影響 |
|---|---|---|---|
| D1 | 誰執行 | **(a)** 第一段（排程狀態、JWT）由使用者、第二段（判讀）由 Claude（唯讀、約 1 分鐘；J5 在 Claude 執行中也成立）<br>(b) 兩段都由使用者執行、Claude 先結束（比照 ⑩） | 判讀器⛔ 沒有 sentinel 與鎖，被 OOM 砍掉也⛔ 不需要重開機（V0、V1）；JWT 在 (a)、(b) 都⛔ 不經過 Claude；(b) 較保守但多一次交接 |
| D2 | 判讀器輸出的保存 | **(a)** repo 外 `~/i074_stage2_formal/judge-<UTC>/`（`status.txt`、`precheck.txt`、`verdict.json`、`judge.err`、`rc`），SHA 與摘要記進文件，保存到 I-074 收斂<br>(b) 另 commit 進 repo（例如 `python/baselines/i074_stage2/` 下新增判讀結果檔） | 判讀器可以從已 commit 的 evidence 重跑，輸出可重現；(b) 會在 Stage 2 的目錄新增 contract 以外的檔案 |
| D3 | B 時 I-074 的移除 | **(a)** ⑪ 只做到「驗證完成、結論歸檔、I-074 標成已完成／待 review」；I-074 的移除（約 1.18 萬行的長期內容歸檔、數十處交叉引用、⑨-1／⑨-2／⑩／⑪ 計畫書的收斂）另立「I-074 收斂計畫」<br>(b) ⑪ 一併收斂 | 規模與風險都和 ⑪ 本身不同層級；(a) 讓 ⑪ 的結果先單獨 review |
| D4 | I-074「已知限制（open）①」（B 時） | **(a)** 移到新 issue 承接（B 的處置時才佔號），I-074 只留指標<br>(b) I-074 關閉之前修正程式（另立計畫） | 那是 acceptance harness 的報告行為，與判讀無關；(b) 要改 `scripts/`，⑪ 的範圍不改程式 |
| D5 | 下游影響的摘要（「五」） | **(a)** 做，B、C 都做：預先定義的表、只描述、⛔ 不參與判定（B 另做交叉核對）<br>(b) 只記判讀器的輸出 | 決策樹的 B 要求記錄「下游影響」；C 的新 issue 也需要實際值；判讀器只輸出每列的 B／C 與 reasons，⛔ 沒有轉換的統計 |

v4（`s11`）新增的決定（✅ 使用者裁決 2026-10-29：D6、D7 採 (a)——D6 以修正「三之二」的核對缺口為前提，已於 v4 第一輪 review 的修正完成）：

| # | 決定 | 選項 | 理由與影響 |
|---|---|---|---|
| D6 | `s11` 的位置、完整性與移除 | **(a)** 比照 `s92`：`~/.local/bin/s11`（repo 外、mode 755）；「三之二」的獨立核對；狀態檔與 `precheck.txt` 記下 `s11_sha256`、Claude 事後對照；安裝的 SHA-256 記進本檔；⑪ 的結果 review 通過之後由使用者決定是否刪除（連同 `/dev/shm/s11.lock`）<br>(b) 放進 repo 的 `scripts/` | ⑪ 只執行一次，工具⛔ 不需要長期維護；(b) 會動到 `scripts/`、要另外的測試與 review |
| D7 | 判讀器的暫存目錄 | **(a)** `s11 judge` 在新建的 `$J` 裡建 `$J/tmp`（mode 700）<br>(b) 照 v3：事先建 scratchpad 的 `judge-tmp` | (a) 一定是新的空目錄、⛔ 不依賴執行者的 scratchpad，D1 改成 (b) 時也⛔ 不必改指令；異常中止的殘留與輸出放在一起；暫存複本約 0.27 GB 落在根檔案系統（可用 6.3G） |

## 十、執行步驟

1. ✅ 本計畫 review（兩輪）→ 使用者定奪 D1～D5（全部 (a)）→ 確認（2026-10-29；使用者 commit 本檔）。
2. ✅（2026-10-29，結果見「十二」）兩段在執行之前，以替身在 scratchpad 測過（⛔ 不碰 live、⛔ 不執行真正的判讀器）：
   * 第一段（假的 `docker`、`curl`）：`*_CRON` 的整組多一個鍵（`STOCK_SYMBOLS_CRON`、`SR_EVALUATION_CRON`）、少一個、任一行不同、`SR_EVALUATION_ENABLED=true` 時⛔ 沒有印「J7 排程鍵 ok」；預期的六行而 `SR_EVALUATION_ENABLED` 沒設或是 `false` 時印出。
   * 第二段（假的 `docker`、判讀器、狀態檔）：J1、J4、J7b（過期、running ≠ 0、缺當天的紀錄、`sr_evaluation` ⛔ 不是 `disabled`、定時工作的時:分不符）、J7c（批次時刻）各自失敗時都停止、⛔ 沒有呼叫判讀器；`precheck.txt` 寫不進去時停止；正常時呼叫一次、`rc` 原子寫入；判讀器回 1、回 137 時 `rc` 照實記錄。
   * ✅ 2026-10-29 對 v3 的兩段區塊跑過：41 個情境全部通過（第一段 bash、zsh 各 8 個；第二段 25 個，含整合）。
   * **v4：對 `s11`（從本檔抽出「三之一」，只替換路徑與常數）重跑同一套情境**，另加：`s11` 沒有參數、參數錯 → 說明、結束碼 2；狀態檔的第二行與 `precheck.txt` 的第一行有 `s11_sha256`（＝ 測試副本的 SHA）；暫存目錄是 `$J/tmp`，正常時結束為空、137 時留下殘留；J7b、J7c 的逐項明細寫進 `precheck.txt`；`/dev/shm/s11.lock` 被持有時第二個 `s11 judge` 停止、⛔ 沒有呼叫判讀器；「三之二」的核對指令（bash、zsh 各一次）只在相同時印 ok——`git show` 失敗、區塊不存在（而本機 `s11` 是空檔）、區塊出現兩次、抽出的內容是空的或第一行⛔ 不是 `#!/bin/bash`、本機 `s11` 是空檔或⛔ 不存在、改一個位元組時都⛔ 沒有印。
3. ✅（2026-10-29，SHA-256 見「十二」；⚠️ 待 review）安裝（v4，D6）：從本檔（工作樹）抽出「三之一」→ `install -m 755` 到 `~/.local/bin/s11`；以工作樹的本檔代替 `git show HEAD:…` 執行「三之二」的核對；SHA-256 記進本檔 → ⛔ 不 commit，停下等 review；使用者 commit 本檔之後，以 HEAD 再核對一次。
4. 依 D1：使用者執行「三之二」的核對與 `s11 status` → 把 `ST` 的路徑告訴 Claude → Claude 以背景執行 `s11 judge <ST>`（第一段結束之後 15 分鐘之內）。
5. 「四」的驗證 → 「五」的摘要（B、C 都做）→ 「六」的分支處置 → 「七」的紀錄與歸檔（含 `s11_sha256` 的事後對照）→ ⛔ 不 commit，停下等 review。
6. review 通過、使用者 commit 之後：B → 依 D3 另立 I-074 收斂計畫；C → 依新 issue 另立調查計畫。

## 十一、修訂紀錄

| 版本 | 日期 | 內容 |
|---|---|---|
| v1 | 2026-10-29 | 初版 |
| v2 | 2026-10-29 | v1 第一輪 review（暫不通過；D1～D5 的方向均建議 (a)）的 4 項：<br>1. **高：前置檢查失敗仍會執行判讀器**——v1 的區塊只有註解要求確認，又要求整段寫成腳本、無法中途人工確認（review 以替身確認 J1、J4 失敗後仍到達呼叫點）。改成兩段：第一段（使用者）只查排程狀態並存檔；第二段（Claude）逐項判定、失敗即停止，J7 也自動判定；保存失敗、輸出檔開不了也停止（「二」「三」「八」）；「十」2 補上替身測試。<br>2. **中：OOM 的結束碼與清理保證寫得過滿**——v1 寫「被 OOM 砍掉 → rc 1」，只適用驗證容器被殺、判讀器照常收尾的情況；判讀器本身被 SIGKILL 會回 137、`trap` ⛔ 不執行，執行者也被殺時⛔ 沒有 rc。補「四」V0 的異常中止分支（先確認程序與容器都已結束，才清理、才決定重跑）、「〇」驗證容器的辨認方式、「八」的風險列。<br>3. **中：J7 無法保證不與批次重疊**——v1 只避開開始時刻前後 10 分鐘，而作業規則要求當天排程時間已過的批次都以 `/scheduler/status` 確認已是終態（只有整輪硬上限的才可以推定）。改成 J7a（使用者查狀態與排程鍵）、J7b（狀態檔的新鮮度、running 0、當天該跑的都已是終態）、J7c（到「現在 ＋ 20 分鐘」⛔ 沒有批次的開始時刻）（「〇」「二」「三」「八」）。<br>4. **低：C 分支的摘要要求不一致**——「六」的 C 要求「五」的表，「十」卻只在 B 時做。改成 B、C 都做；C 只照實描述實際值、⛔ 不預設 B 才成立的等式；交叉核對只在 B（「五」「六」「十」、D5） |
| v3 | 2026-10-29 | v2 第二輪 review（修正方向正確，review 的替身診斷 10 個情境通過；D1～D5 建議維持 (a)）的 2 項：<br>1. **中：J7 仍漏驗可設定的排程**——`stock_symbol_sync` 走 `STOCK_SYMBOLS_CRON`（config 預設 `30 6 * * *`），⛔ 不是固定註冊；`sr_evaluation` 啟用之後有自己的 cron（`SR_EVALUATION_CRON`）；v2 只驗六個鍵各自存在，加入 18:10 的覆寫仍會印「J7 排程鍵 ok」（review 以替身確認）。改成：J7a 要求 `*_CRON` 鍵的整組恰好是預期的六行、`SR_EVALUATION_ENABLED` 沒設或是 `false`；J7b 另要求 `sr_evaluation` 那一列是 `disabled`、當天已跑過的定時工作 `started_at` 時:分 ＝ 清單的時刻；⛔ 不成立就停止、重新確認批次清單（「〇」「二」「三」「八」；「十」2 補對應的替身情境）。<br>2. **低：V1 的 OOM 訊息不符實作**——`finalize-stage2-evidence.sh` 把驗證容器的任何非 0（含 137）轉成 1，判讀器實際印的是「驗證模式不通過（rc=1）」。改正文字，並寫明⛔ 不能只憑那一行判定 OOM：另看 `judge.err` 的內容、`docker events` 的 oom 事件、`dmesg`／`journalctl -k`；原因不明就停下回報（「〇」「四」「八」） |
| v3（確認） | 2026-10-29 | 第二輪的修正 review 通過；使用者定奪 D1～D5 全部採 (a)（「九」、「十」1） |
| v4 | 2026-10-29 | 使用者要求：把兩段包成簡易指令 `s11`（`s11 status`、`s11 judge <ST>`），併進本檔（⛔ 不另立計畫書）。「三」改成用法與差異說明、新增「三之一」（腳本）與「三之二」（核對指令）；兩個函式逐字取自 v3 的兩段區塊，差異：`cd` 失敗即停、記下 `s11_sha256`、暫存目錄改成 `$J/tmp`（D7）、`/dev/shm/s11.lock` 防止同時執行、**J7b、J7c 的逐項明細以 `tee -a` 寫進 `precheck.txt`**（「十」2 的替身測試發現：v3 只把明細印到 stdout，`precheck.txt` 只有停止的那一行，看不出是哪個工作或時刻；使用者同意一起修）。同步「二」J6、「四」V0、「八」、「九」（新增 D6、D7）、「十」2～4 |
| v4（第一輪 review） | 2026-10-29 | 裁決：D6、D7 採 (a)（D6 以修正下列缺口為前提）。修正 1 項（中）：**「三之二」的核對指令可能誤印 ok**——原指令只看管線最後的 `cmp`，`git show` 失敗不會中止；區塊不存在而本機腳本是空檔時，空的抽取結果與空檔相同，也會印 ok（review 以替身確認）。改成 `i074_stage2_s11_check_002`：先把 `git show` 的結果存進變數（失敗或空就停）→ 開頭那一行恰好一次 → 抽出的區塊非空、第一行是 `#!/bin/bash` → 本機 `s11` 非空 → `cmp` → 才印 ok；「十」2 補對應的替身情境（bash、zsh） |
| v4（確認） | 2026-10-29 | 第一輪 review 的修正 review 通過；「十」2、3 完成（「十二」；⚠️ 待 review）。照實記錄：測試時發現的問題都在測試腳本（替換次數的斷言、情境名稱含空白未加引號、`git rm` 刪了目錄），⛔ 沒有改 `s11` 與核對指令 |

## 十二、「十」2、3 的結果（v4，2026-10-29，⚠️ 待 review）

| 項目 | 內容 |
|---|---|
| v3 兩段區塊 | 2026-10-29 先對 v3 的兩段區塊跑過：41 個情境全部通過（第一段 bash、zsh 各 8 個；第二段 25 個，含整合）；當時發現 J7b、J7c 的明細沒有寫進 `precheck.txt`，併入 v4 修正 |
| 測試環境 | scratchpad：從工作樹的本檔抽出「三之一」與「三之二」，只替換路徑與常數（repo、`/dev/shm` 的狀態檔與鎖、`judge-<UTC>` 的上層、sentinel、`/proc/meminfo`、四個 SHA、J7c 的「現在」加上偏移；每一處都斷言出現次數）；替身：`docker`、`curl`、可偏移的 `date`、`mkdir`（寫不進去、`verdict.json` 是目錄）、假的 repo 與判讀器（記下 `TMPDIR` 與 fd 9、回 0／1／137）。⛔ 沒有碰 live、⛔ 沒有執行真正的判讀器 |
| 結果 | **67／67 通過**：參數 5（沒有參數、`bogus`、`status extra`、`judge`、`judge a b` → 說明、rc 2）；`s11 status` 15（bash 與 zsh（經 shebang）各 7：預期六行與 `SR_EVALUATION_ENABLED=false` 印「J7 排程鍵 ok」，多 `STOCK_SYMBOLS_CRON`／`SR_EVALUATION_CRON`、少一行、一行不同、`SR_EVALUATION_ENABLED=true` ⛔ 沒有印；另 docker inspect 失敗 1；每案都驗 ⛔ 沒有洩漏假的金鑰與 JWT、mode 600、curl 收到 token、第二行 `s11_sha256`）；`s11 judge` 29（正常、判讀器回 1、回 137；J1、J4、J5、J6 四種、J7b 八種、J7c 三種、寫不進 `precheck.txt`、輸出檔開不了、鎖被持有、鎖釋放之後、`status` → `judge` 的整合；另驗 `$J/tmp` mode 700 而正常結束為空、鎖被持有時⛔ 沒有建 `$J`、整合時 J7b 的逐項明細寫進 `precheck.txt`）；核對指令 18（bash、zsh 各 9：完全相同、恢復之後相同 → ok；差一個位元組、本機⛔ 不存在、區塊⛔ 不存在而本機是空檔、區塊出現兩次、區塊是空的而本機是空檔、第一行⛔ 不是 `#!/bin/bash`、`git show` 失敗 → ⛔ 沒有印 ok） |
| 逐項的觀察 | 失敗時判讀器一次都⛔ 沒有被呼叫、⛔ 沒有 `rc` 檔；成功與回 1／137 時恰好呼叫一次、`rc` 照實記錄；判讀器拿到的 `TMPDIR` ＝ `$J/tmp`、fd 9（鎖）⛔ 沒有傳給它；回 137 時 `$J/tmp` 留下殘留（`tmp.leftover`）、`precheck.txt` 印「暫存目錄：1 項」；J7b 失敗時 `precheck.txt` 有逐項的明細（例如 `J7b stock_symbol_sync 06:30 success 2026-10-29T06:35:00…+08:00 ⚠️ ⛔ 不成立`）、J7c 失敗時有 `J7c 批次的時刻 …` |
| 與 v3 測試的差異 | v3 的「J6：`judge-tmp` 不是空的」⛔ 不再適用（`$J/tmp` 由 `s11 judge` 新建）；改驗 `$J/tmp` 的 mode、內容與判讀器拿到的 `TMPDIR` |
| 測試腳本的問題（照實） | 第一次：替換的斷言擋下（`/dev/shm/s11.lock` 寫成 2 次，實際 1 次）；第二次：8 個 FAIL 都是測試腳本的錯（核對的情境名稱含空白而未加引號，「預期」被拆成別的字；`git rm` 把空的 `docs/plans` 刪掉、之後的 `cp` 失敗）。⛔ 沒有改 `s11` 與核對指令 |
| 安裝 | 從本檔（工作樹）抽出「三之一」（開頭那一行恰好一次、非空、與測試用的原檔 `cmp` 相同）→ `install -m 755`：`~/.local/bin/s11`，10,440 bytes、mode 755、owner `dev`；**SHA-256 `2b7b9e7316568c3562c57c1cee873f739af7e357aa697f8dfee4bb98c8312e5b`**；`command -v s11` ＝ `~/.local/bin/s11` |
| 核對 | 「三之二」以工作樹的本檔代替 `git show HEAD:…`（`cat`）執行：bash、zsh 都印出上面的 SHA-256 與「s11 ＝ HEAD 的計畫書 ok」；`s11`（沒有參數）印說明、rc 2。⚠️ 使用者 commit 本檔之後，再以 HEAD 原樣執行「三之二」一次（「十」3） |
