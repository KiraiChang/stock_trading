# Stage 2 步驟 ⑩ 細部計畫 v3（2026-10-28，✅ 確認（第三輪 review 通過、D1～D9 已定案）；「十」2 實作 ✅ review 通過並 commit `a9cb41d`（「十三」）；⚠️ 2026-10-28 17:00 執行（使用者裁決，「十」4）；歷次 review 的修正見「十一」，review 的收斂條件見「十二」）

需求確認（2026-10-27）：規劃 I-074 Stage 2 的 ⑩——**唯一一次**正式 Stage 2（before 全量 replay ＋ finalize ＋ 晉升，在隔離複本執行）。
範圍：⑩ 從前置條件一直到晉升、驗證與保存，加上交給 ⑪ 的規則；⑪ 的 B／C 判讀另立計畫。像 `s91`／`s92` 那樣的簡易指令
**之後另立計畫**，本計畫只寫貼上版的區塊。本計畫確認之前⛔ 不執行 ⑩、⛔ 不改程式（⑨-2 之後只能 commit 文件）。
文中的「v29」「八之一」～「八之三」「正式 scan 的計次裁決」「關閉條件」「⑨-1 細部計畫」「⑨-2 細部計畫」「⑨-2 結果」都指
`docs/issue.md` I-074 的章節；操作程序的現況規格在 `docs/development-workflow.md`「I-074 Stage 2 的正式執行程序」。

依據：v29「八」的 ⑩、「八之一」（隔離複本、真正 repo 不凍結、操作契約）、「八之一之二」（鎖、sentinel、本趟的範圍、重開機才解除）、
「八之二」（freeze record）、「八之三」（晉升與端到端結束碼）、「磁碟檢查（v29）」、「正式 scan 的計次裁決」的 v26／v27 表、
⑦b／⑦c 的實作（`development-workflow.md`「I-074 Stage 2 的正式執行程序」）、⑦d 的 observer、⑨-1 細部計畫 v8 的程序（P1～P9、wrapper、
控制目錄、安全中止）與 ⑨-2 結果（freeze record 的位置與錨）。

## 〇、唯讀盤點（2026-10-27，HEAD `62e1773`）

| 項目 | 現況 |
|---|---|
| 入口 | `REPLAY_IMAGE_ID=sha256:2a90ad1c… scripts/run-i074-stage2.sh --freeze-record <⑨-2 的 <work>/freeze_record.json> --work-dir <repo 外、尚不存在>`。**直接執行**（shebang `#!/bin/bash -p`；`bash <script>` 會被拒絕）。入口只用 bash 內建，驗完公開 argv 與「入口清單的投影 ＝ HEAD」之後 `exec /usr/bin/python3 -I scripts/lib/i074-stage2-supervisor.py`——**PID 不變**，所以 wrapper 記下的 `harness_pid` 之後就是 supervisor。`--work-dir` 的上層目錄必須已存在、`--work-dir` 本身⛔ 不得存在、⛔ 不得在 repo 內 |
| freeze record | `/home/dev/i074_stage2_sizing/formal-92-20261027T090656Z/freeze_record.json`：SHA-256 `098a39363d7bf40507d137fee43e70fe39dc7966ed38651beaadbbfc8cb4bed3`、`repo_head` `832f678acfa429f8e4fdba2c82eb77eae826def8`（＝ `E92`）、`P_B` 163,024,896；同目錄 `raw/` 的 manifest `sha256` `e4b1ead7a653c68677877d3263e22fb2728d0ba2e98b943f648fadc6f56b6765`（⑨-2 結果）。持鎖階段只接受「成功結束的 sizing `<work>`、在原位置的頂層」——⛔ 不得搬移、複製、改名 |
| HEAD 與 ⑨-2 之後的 commit | HEAD ＝ `62e17739e7ed6e5df8e8a9ee1cf48f4704be1e4d`（⑨-2 結果，＝ `R92`）；`832f678..62e1773` 只動 `docs/`。複本釘在 `832f678`；入口、supervisor、持鎖階段則是真正 repo 的 HEAD 版——所以 ⑨-2 之後到 ⑩ 開跑之前仍只能 commit 文件（⑨-2 細部計畫「六」） |
| 流程與耗時 | supervisor（驗帳號 → 取 `/run/lock/i074-stage2.lock` → 確認沒有 sentinel → 查殘留的 label 容器 → 建 sentinel → subreaper）→ 持鎖階段（來源檢查、複製 freeze record 與報告、`git clone --no-hardlinks`、detached checkout `repo_head`、驗要執行的腳本 ＝ `repo_head` 的 blob）→ 複本內 orchestrator：preflight 0（完整性）→ 1（凍結兩份 patch）→ 2（freeze record 完整驗證）→ 3（`anchors`，容器）→ 4（真正 repo 的兩條檢查 ＋ `--check-failed-record`，容器）→ 6（磁碟）→ `state/replay_started.json` → **replay**（容器）→ finalize（容器）或 publish-failed-record → 依磁碟事實判終態 → **晉升**（`--verify-promotion-staging`，容器）。耗時以 ⑨-1 估：量測趟（full compute）185.8 分鐘、finalize 與晉升的容器各約 1 分鐘 → **整段約 3 小時 15 分～3 小時 30 分**；**保守估計約 3 小時 52 分**（replay 取「正式 scan 的計次裁決」用的 3.7 小時上界 ＋ 其餘步驟約 10 分鐘）——P4 與 D2 以保守估計計算（v3） |
| 結束碼 | 0 成功 archive 已在真正 repo durable；6 failed record 已在真正 repo durable；2 failed-record lookup 命中；1 還沒有任何終態；3 某一層的 durability 未確認 → 重跑 `--promote`；8 本次沒有確認晉升完成、可以安全重跑 `--promote`；9 需要人工判斷；128＋N 被訊號 N 中斷（supervisor 清空本趟之後回傳）；137 有兩種：**持鎖階段的子程序**被 SIGKILL（supervisor 照常清空本趟、刪 sentinel、放鎖之後才回 137——⛔ 不需要重開機），或 **supervisor 自己**被 SIGKILL（sentinel 留下 → **只能重開機**）。⚠️ 收回（rollback）失敗時回 1 也會留下 sentinel——所以判讀一律**先看 sentinel**（「四」） |
| `state/` | `run.json`（preflight 0 之後）→ `preflight.json`（preflight 6 之後）→ `replay_started.json`（replay 之前）→ `replay_done.json`（replay 回 0／6 且輸出形狀通過）→ `attempt.json`（finalize／publish 回 1 而且沒有終態；欄位 `kind`、`rc`、`count`（每失敗一次 ＋1）、`replay_done_sha256`；**`--resume` 成功之後⛔ 不刪**）。⚠️ **`replay_started.json` 寫在呼叫 runner 之前，只代表「準備啟動」**：runner（`run-replay-offline.sh`）在 `docker run` 之前還有記憶體下限（mem-guard）、run identity、artifact、patch 是否 canonical、image 的檢查，全部通過之後才在 `logs/replay.out` 印 `==> offline replay：…`、隨即 `exec docker run`（label shim 拒絕時回 125）；orchestrator 在 `log` 記 `replay 結束（rc=N）`。計次依「四」的三分（v2）。複本內的 `i074_stage2_preflight.py state-check --require <清單>` 驗每一份現存 state 的 schema 與 SHA 鏈、印出 `present=…`（唯讀） |
| 容器的記憶體上限 | ⑩ 的四種容器（`anchors`、`check_failed_record`、replay、finalize／晉升的驗證）各在啟動當下以 `mem_guard_clamp(700m)` 決定上限。⚠️ **supervisor 以清單刪除 `MEM*`**（連同 `SIZING_*`、`DOCKER_*` 等），所以 ⑩ ⛔ 無法像 ⑨-2 那樣固定 `MEM_MIN_MB=447`：預設 `MEM_RESERVE_MB=150`、`MEM_MIN_MB=256` → 上限 ＝ min(700, MemAvailable − 150)，低於 256 才拒絕。⑨-1 的量測趟上限 700m、最大單一 RSS 394.3 MiB、cgroup 476.1 MiB |
| 現在的記憶體與磁碟 | host 2 GiB、2 核；MemAvailable 757 MiB（Claude 執行中）、swap 511／512 MiB 已用；⑨-2 precheck（Claude 已結束）時 1,177 MiB。根檔案系統可用 4.9G、`/dev/shm` 1.0G 幾乎全空。真正 repo 的 `.git` 222M（複本約 0.27 GB） |
| 殘留 | `/run/lock` ⛔ 沒有 i074-stage2 的鎖檔與 sentinel（第一次執行時由 supervisor 建鎖檔，**永不 unlink**）；真正 repo 的 `python/baselines/i074_stage2/` ⛔ 沒有未追蹤或 ignored 的項目、HEAD ⛔ 沒有 `failed/`；`.gitignore` 有 `python/baselines/i074_stage2/.promote-staging-*` |
| observer | `python3 python/scripts/i074_stage2_sizing.py observe --work-dir <⑩ 的 work> --out <新目錄>`：⛔ 不要求 `--work-dir` 已存在；狀態在 `/dev/shm/i074-observe-<run id>/`（名稱符合 `i074-*`）；SIGINT、SIGTERM 都會讓它寫出報告後結束，**正常結束才刪自己的狀態目錄**（被 KILL 或當掉會留下，而 P7 要求 `/dev/shm/i074-*` 為空——「五」7）；啟動時在 stderr 印出狀態目錄的路徑；每 0.5 秒讀容器的 cgroup 峰值、每 2 秒 `docker ps` 與 MemAvailable、每 10 秒量 `--work-dir`。環境帶 `I074_STAGE2_*`／`SIZING_*` 或 PATH 上的 docker 是 shim → 拒絕。⛔ 不是 ⑩ 的前置、⛔ 不為量測而重跑（v29「八」） |
| 計次（「正式 scan 的計次裁決」v27） | 上限 ①（跑完並留下 artifact 的 replay，最多 5）：已用 3（D、D+1、after'）；上限 ②（物理啟動，最多 8）：已用 3。**before 正式趟：0／1、它的崩潰重試：0／1**；before 重跑（只限 rc 6 之後、語意 SHA 不同）：0／1。⑨-1 的量測趟另計、⛔ 不佔上限 |
| live 的批次（`development-workflow.md` 的作業規則） | 平日 06:30 `stock_symbol_sync`、`corporate_action_sync`（上限 45 分鐘）、07:00 `delisting_reconcile`、08:50 `pre_market`、09:00～13:55 `intraday`、15:00 `daily_close`（＋ `sr_zone_verify`）、16:00 `evaluation_universe_sync`（＋ `candle_gap_detection`）、17:00 `sr_analysis`、21:00 `chip_daily_sync`、22:00 `sr_analysis_chip`；週末只有 06:30、07:00。實測（⑨-1、⑨-2 的 precheck）：17:00 `sr_analysis` 0～1.7 分鐘、21:00 `chip_daily_sync` 約 4 分鐘、22:00 `sr_analysis_chip` 不到 0.1 秒。⚠️ v3：D2 的首選時段以**晚間兩個批次後移**為前提（`chip_daily_sync` → 22:00、`sr_analysis_chip` → 23:00；另立計畫 [`evening-batch-shift.md`](./evening-batch-shift.md)） |

## 一、目標、範圍與⛔ 不做

| 項目 | 內容 |
|---|---|
| 目標 | 以 ⑨-2 的 freeze record 跑**唯一一次**正式 Stage 2：成功 → 成功 archive 晉升進真正 repo 的 `python/baselines/i074_stage2/evidence/`（結束碼 0）→ 驗證、保存、記錄 → review → 使用者 commit（evidence ＋ 文件）→ ⑪。每一次嘗試都記錄並保存 |
| 範圍 | 前置條件、時段、啟動與程序身分（沿用 ⑨-1 的 wrapper，加上 observer）、狀態、安全中止、完成判定、依結束碼與 `state/` 的判讀與分支、`--resume`／`--promote` 的使用時機、驗證與保存、交給 ⑪ 的規則、紀錄與歸檔 |
| ⛔ 不做 | ⛔ 不改程式、門檻、預算與 schema；⛔ 不跑 ⑪ 的判讀器；⛔ 不碰 live（⛔ 不停排程、⛔ 不停 live 的容器，只避開時段）；⛔ 不調大 `MEM`、⛔ 不用 `MEM_FORCE`；⛔ 不手動刪除 sentinel、⛔ 不手動搬移晉升的檔案；⛔ 不搬移、⛔ 不複製、⛔ 不改名 ⑨-2 與 ⑩ 的 `<work>`；⛔ 不用 `pkill -f`／`pgrep -f` 找程序；⛔ 不另跑演練（⑦b／⑦c 的測試已涵蓋流程，正式趟只有一次） |

## 二、前置條件（每一次開跑——含重跑——都逐項確認、記進結果；任一項不成立就⛔ 不開跑）

| # | 條件 | 怎麼確認 |
|---|---|---|
| P1 | HEAD 包含 `E92`（`832f678`）與 `R92`（`62e1773`）；`scripts/`、`python/`、`.gitattributes` 沒有未 commit 的變更；**`E92` 到 HEAD 之間只動了文件**（`docs/` 與根目錄的 `CLAUDE.md`）——入口、supervisor、持鎖階段（真正 repo 的 HEAD 版）＝ ⑨-2 量到的那一版 | 「P1 祖先 ok」「P1 clean ok」「P1 ⑨-2 之後只動文件 ok」；`git` 失敗⛔ 不能當作成立 |
| P2 | freeze record 的來源完好：在原位置、形狀符合持鎖階段的來源規則、SHA ＝ `098a3936…`、`check-basic` 印出 `E92`、`check-pair` rc 0、`raw/` 的 manifest 仍是 `e4b1ead7…` | 「P2 來源形狀 ok」「P2 freeze record 的 SHA ok」「P2 check-basic ok」「P2 check-pair ok」「P2 raw manifest ok」 |
| P3 | image 在本機、identity 的 `expected_image_id` ＝ `REPLAY_IMAGE_ID`、identity 檔的 SHA ＝ `a02bf3bd…`（與 ⑨-1、⑨-2 相同） | 「P3 image ok」「P3 identity ok」「P3 identity SHA ok」（v3 第四輪 review：SHA 由人眼比對改成自動判定） |
| P4 | 整段執行⛔ 不重疊任何批次（D2）：**當天所有排程時間已過的工作**都已有當天那一筆、而且是終態（⛔ 不是 `running`）；以**保守估計（3 小時 52 分）**算出的結束時間距離下一個批次 ≥ 30 分鐘（v3）；D2 的平日 17:00 時段另要 live backend 的晚間批次已後移 | `docker inspect` 只看排程鍵 ＋ `/scheduler/status` 的每一列，**輸出原樣記進紀錄**；平日 17:00 時段另要「P4 晚間批次已後移 ok」 |
| P5 | 開跑當下 **MemAvailable ≥ 950 MiB**（D3）：扣掉 100 MiB 的漂移（含 observer），replay 啟動當下仍 ≥ 850 MiB → 上限 ＝ min(700, 850 − 150) ＝ 完整的 700m | 「P5 ok」（結束碼）。不成立 → 結束 Codex 等非必要的程序（⛔ 不動 live），仍不成立就⛔ 不開跑 |
| P6 | 根檔案系統可用 **≥ 2,000,000,000 bytes**（preflight 6 在複本建立之後要求 1,241,513,984；複本約 0.27 GB；晉升另要來源的 2 倍；observer 與 log 很小）、`/dev/shm` 可用 ≥ 256 MB | 「P6 ok」（結束碼）＋ `df -h` 原樣 |
| P7 | 沒有殘留：`/run/lock/i074-stage2.active` ⛔ 不存在；帶 `i074.stage2.run` 鍵的容器、`i074sz-` 容器都沒有；`/dev/shm/i074-*` 為空（含 observer 的狀態）；⛔ 沒有 ⑩ 的入口、supervisor、acceptance、sizing、observer 的程序 | 「P7 ok」（`find`、`docker ps` ×2、`ps`、`awk` 的結束碼都是 0 才算） |
| P8 | 真正 repo 的 `python/baselines/i074_stage2/` ⛔ 沒有未追蹤或 ignored 的項目（preflight 4 也會擋，先在這裡確認以免開跑才失敗）；記下 HEAD 的 `failed/` 筆數（預期 0） | 「P8 ok」 |
| P9 | 外部的 commit 錨點：開跑之前記下 `EXPECTED=$(git rev-parse HEAD)`（寫進控制目錄）；work 目錄 `~/i074_stage2_formal/formal-10-<UTC>`、observer 的 `--out` `~/i074_stage2_observe/<同一個名稱>`、控制目錄 `/dev/shm/s10-<同一個名稱>/` 都尚不存在 | 「三」① 自己檢查；入口也拒絕已存在的 work 目錄 |
| P10 | **⑨-2 已通過**、結果已 review 並 commit（`62e1773`；D9） | 讀 ⑨-2 結果一節 |
| D1 | Claude 已結束（⛔ 沒有 `claude` 的程序）；Codex 只記錄、等待期間⛔ 不使用 | 「D1 Claude 已結束 ok」 |
| 基準 | 真正 repo 的 worktree 登記數、`/tmp` 的清單、`btime`——「五」6 的前後比對 | 「基準 ok」 |

```text id="i074_stage2_step10_precheck_001"
# P1～P10、D1 與基準（使用者在自己的終端機執行；**只貼這一個區塊**——P4 的 read 會等你貼上 JWT（前端登入後取得，或 POST /api/v1/auth/login））：
#   整段的輸出同時存進 tmpfs 的 $PRE（mktemp：名稱不碰撞、mode 600），跑完由 Claude 複製進 <NAME>.ctl/precheck.txt；⛔ 沒有「PRE 保存 ok」就⛔ 不開跑
cd /home/dev/workspace/stock_trading
PRE="$(mktemp "/dev/shm/s10-pre-$(date -u +%Y%m%dT%H%M%SZ)-XXXXXX.txt")"; RM=$?
{
echo "PRE $PRE"; date -u +'precheck 開始 %FT%TZ'; BT="$(awk '$1 == "btime" { print $2 }' /proc/stat)"; RBT=$?; echo "btime $BT"   # 判斷之後有沒有重開機
IMG=sha256:2a90ad1c1dd801d59373988a4d84afe1437cbee62ccab3709b06cfa8f7da5027
E92=832f678acfa429f8e4fdba2c82eb77eae826def8; R92=62e17739e7ed6e5df8e8a9ee1cf48f4704be1e4d   # ⑨-2 的 EXPECTED 與結果的 commit
F92=/home/dev/i074_stage2_sizing/formal-92-20261027T090656Z                                     # ⑨-2 的 <work>（原位置）
git merge-base --is-ancestor "$E92" HEAD && git merge-base --is-ancestor "$R92" HEAD && echo "P1 祖先 ok"
GS="$(git status --porcelain -- scripts python .gitattributes)" && [ -z "$GS" ] && echo "P1 clean ok"        # git 失敗 ⛔ 不能當作 clean
DD="$(git diff --name-only "$E92" HEAD -- . ':(exclude)docs' ':(exclude)CLAUDE.md')"; RDD=$?
printf 'P1 ⑨-2 之後文件（docs/ 與根目錄的 CLAUDE.md）以外的變更：%s（git diff 的 rc %s）\n' "${DD:-無}" "$RDD"
[ "$RDD" = 0 ] && [ -z "$DD" ] && echo "P1 ⑨-2 之後只動文件 ok"
[ -f "$F92/freeze_record.json" ] && [ ! -L "$F92/freeze_record.json" ] && [ -d "$F92/raw" ] && [ ! -L "$F92/raw" ] \
  && [ ! -e "$F92/failure_summary.json" ] && [ ! -e "$F92/raw-failed" ] && [ ! -L "$F92/raw-failed" ] && echo "P2 來源形狀 ok"
FS="$(sha256sum < "$F92/freeze_record.json" | cut -d' ' -f1)"; echo "P2 freeze record 的 SHA $FS"
[ "$FS" = 098a39363d7bf40507d137fee43e70fe39dc7966ed38651beaadbbfc8cb4bed3 ] && echo "P2 freeze record 的 SHA ok"
H2="$(python3 -B python/scripts/i074_stage2_freeze_record.py check-basic --record "$F92/freeze_record.json")" \
  && [ "$H2" = "$E92" ] && echo "P2 check-basic ok（repo_head $H2）"
python3 -B python/scripts/i074_stage2_freeze_record.py check-pair --record "$F92/freeze_record.json" \
  --report "$F92/sizing_report.json" && echo "P2 check-pair ok"
python3 -B python/scripts/i074_stage2_sizing.py raw-manifest --raw "$F92/raw" --repo . --expected-repo-head "$E92" \
  --check-sha256 e4b1ead7a653c68677877d3263e22fb2728d0ba2e98b943f648fadc6f56b6765 && echo "P2 raw manifest ok"
docker image inspect "$IMG" > /dev/null && echo "P3 image ok"
ID="${XDG_DATA_HOME:-$HOME/.local/share}/stock_trading/i074_stage2/run_identity.json"
python3 -c 'import json,sys; sys.exit(0 if json.load(open(sys.argv[1]))["expected_image_id"] == sys.argv[2] else 1)' "$ID" "$IMG" \
  && echo "P3 identity ok"
IS="$(sha256sum < "$ID" | cut -d' ' -f1)"; echo "P3 identity 的 SHA ${IS:-讀不到}"
[ "$IS" = a02bf3bd3f44cadb8d9df2e5e9d7d6b89d92cca2130e2e85526cdc8b139d9f4d ] && echo "P3 identity SHA ok"   # 與 ⑨-1、⑨-2 相同
docker inspect stock_trading-backend-1 --format '{{range .Config.Env}}{{println .}}{{end}}' | grep -E '^[A-Z_]+_(CRON|ENABLED)='   # P4
EV="$(docker inspect stock_trading-backend-1 --format '{{range .Config.Env}}{{println .}}{{end}}')" && grep -qxF 'CHIP_SYNC_CRON=0 22 * * 1-5' <<< "$EV" \
  && grep -qxF 'SR_ANALYSIS_CHIP_CRON=0 23 * * 1-5' <<< "$EV" && echo "P4 晚間批次已後移 ok（D2 的平日 17:00 時段可用）"; unset EV   # EV ⛔ 不印（含金鑰）
BIP="$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}} {{end}}' stock_trading-backend-1 | cut -d' ' -f1)"
echo "P4：先打一個空白、再貼上 JWT，按 Enter（⛔ 不回顯、⛔ 不寫進檔案或紀錄）"; read -rs TOKEN   # 空白：萬一沒跑到 read，那一行⛔ 不進 history
curl -fsS -H "Authorization: Bearer $TOKEN" "http://$BIP:8080/api/v1/scheduler/status" | python3 -c 'import json, sys
jobs = json.load(sys.stdin)["jobs"]
for j in sorted(jobs, key=lambda j: j["job_name"]):
    print("P4", j["job_name"], j["status"], j.get("started_at", "-"), j.get("finished_at", "-"), "⚠️ 仍在執行" if j["status"] == "running" else "")
print("P4 running 的數量：", sum(j["status"] == "running" for j in jobs))'; unset TOKEN
awk '/^MemAvailable:/ { m = int($2 / 1024); f = 1 } END { printf "P5 MemAvailable %d MiB（門檻 950）\n", m; exit !(f && m >= 950) }' \
  /proc/meminfo && echo "P5 ok"
df -h / /dev/shm                                                                                             # P6
df -B1 --output=avail / /dev/shm | awk 'NR == 2 { r = $1 } NR == 3 { s = $1 } END { printf "P6 / 可用 %d bytes（門檻 2000000000）、/dev/shm 可用 %d bytes（門檻 268435456）\n", r, s; exit !(r >= 2000000000 && s >= 268435456) }' \
  && echo "P6 ok"
A7=/run/lock/i074-stage2.active; [ ! -e "$A7" ] && [ ! -L "$A7" ]; RA0=$?                                     # P7：sentinel ⛔ 不得存在
S7="$(find /dev/shm -maxdepth 1 -name 'i074-*')"; RS=$?
C7="$(docker ps -a --filter name=i074sz- -q)"; RC=$?; L7="$(docker ps -a --filter label=i074.stage2.run -q)"; RL7=$?
PP="$(ps -eo pid,args)"; RP=$?
P7="$(awk '/run-i074-stage2\.sh|i074-stage2-supervisor\.py|i074-stage2-(acceptance|sizing)\.sh|i074_stage2_sizing\.py observe/' <<< "$PP")"; RA7=$?
printf 'P7 sentinel：%s\nP7 S：%s\nP7 容器（i074sz-）：%s\nP7 容器（label）：%s\nP7 程序：%s\nP7 rc：find %s、docker ps %s／%s、ps %s、awk %s\n' \
  "$([ "$RA0" = 0 ] && echo 無 || echo "⚠️ 存在（需要重開機，⛔ 不得手動刪除）")" "${S7:-無}" "${C7:-無}" "${L7:-無}" "${P7:-無}" "$RS" "$RC" "$RL7" "$RP" "$RA7"
[ "$RA0" = 0 ] && [ "$RS" = 0 ] && [ "$RC" = 0 ] && [ "$RL7" = 0 ] && [ "$RP" = 0 ] && [ "$RA7" = 0 ] && [ -n "$PP" ] \
  && [ -z "$S7" ] && [ -z "$C7" ] && [ -z "$L7" ] && [ -z "$P7" ] && echo "P7 ok"                         # 任一個指令失敗 ⛔ 不能當作沒有殘留
U8="$(git status --porcelain --untracked-files=all --ignored -- python/baselines/i074_stage2/)" && [ -z "$U8" ] && echo "P8 ok"
printf 'P8 HEAD 的 failed record 檔數：%s\n' "$(git ls-tree -r HEAD -- python/baselines/i074_stage2/failed/ | wc -l)"
H9="$(git rev-parse --verify HEAD)" && echo "P9 $H9"
PD="$(ps -eo pid,rss,comm)"; RD=$?                                                                           # D1：ps 先完整取值，再交給 awk
DL="$(awk '$3 ~ /^(claude|codex)/ { print "D1", $0 }' <<< "$PD")"; RA=$?; NC="$(awk '$3 == "claude" { n++ } END { print n + 0 }' <<< "$PD")"; RN=$?
printf '%s\nD1 rc：ps %s、awk %s／%s；claude 的程序數 %s\n' "${DL:-D1 （⛔ 沒有 claude／codex 的程序）}" "$RD" "$RA" "$RN" "${NC:-無}"   # codex 只記錄（可以存在、⛔ 不使用）
[ "$RD" = 0 ] && [ "$RA" = 0 ] && [ "$RN" = 0 ] && [ -n "$PD" ] && [ "$NC" = 0 ] && echo "D1 Claude 已結束 ok"  # 任一個指令失敗 ⛔ 不能當作已結束
WL="$(git worktree list --porcelain)"; RW=$?; NW="$(grep -c '^worktree ' <<< "$WL")"; RG=$?                   # 「五」6 的基準
TL="$(ls -A /tmp)"; RL=$?; TB="$(sed 's|^|基準 /tmp：|' <<< "$TL")"; RE=$?
printf '基準 worktree 登記數 %s\n%s\n基準 rc：git %s、grep %s、ls %s、sed %s、btime 的 awk %s\n' "${NW:-無}" "$TB" "$RW" "$RG" "$RL" "$RE" "$RBT"
[ "$RW" = 0 ] && [ "$RG" = 0 ] && [ "$RL" = 0 ] && [ "$RE" = 0 ] && [ "$RBT" = 0 ] && [[ "$NW" =~ ^[1-9][0-9]*$ ]] && [[ "$BT" =~ ^[1-9][0-9]*$ ]] \
  && [ -n "$TB" ] && echo "基準 ok"                                                                          # 任一個指令失敗 ⛔ 不能當作有基準
date -u +'precheck 結束 %FT%TZ'
} 2>&1 | tee "$PRE"; RT=$?; [ "$RM" = 0 ] && [ "$RT" = 0 ] && [ -s "$PRE" ] && [ "$(head -n 1 "$PRE")" = "PRE $PRE" ] \
  && [ "$(grep -c '^precheck 開始 ' "$PRE")" = 1 ] && [ "$(grep -c '^precheck 結束 ' "$PRE")" = 1 ] && grep -q '^precheck 結束 ' <<< "$(tail -n 1 "$PRE")" && echo "PRE 保存 ok：$PRE"
```

⚠️ **開跑的條件**：P1～P9（P1 三行、P2 五行、P3 三行、P5、P6、P7、P8）、D1、基準都印出 ok，P4 的每一列都看過（當天應確認的工作都是終態、running 0；平日 17:00 時段另要「P4 晚間批次已後移 ok」），
最後一行是「PRE 保存 ok」；而且 precheck 結束之後**立刻**接「三」①（間隔越短越好，⑨-1 是 55 秒、⑨-2 是 80 秒）。
⚠️ 只貼這一個區塊；看到「P4：先打一個空白、再貼上 JWT…」這一行的**實際輸出**之後，先打一個空白、再貼上 JWT、按 Enter（與 ⑨-1、⑨-2 相同）。

## 三、啟動、程序身分、狀態、安全中止與完成判定（D1：沿用 ⑨-1 的 wrapper，由使用者在 tmux／自己的終端機執行）

| 項目 | 規則 |
|---|---|
| 控制目錄 | `/dev/shm/s10-<NAME>/`（tmpfs、mode 700；名稱⛔ 不是 `i074-*`）：`run.tsv`（身分）、`wrapper.sh`、`log`（⑩ 入口的 stdout／stderr——每一步的輸出在 `<work>/logs/`，這裡只有幾 KB）、`observe.log`、`rc`（rc marker，原子寫入） |
| 啟動的環境 | `REPLAY_IMAGE_ID` 固定成封存的 image；其他都由入口與 supervisor 清理（入口清 `BASH_ENV`／`ENV`／`LD_*`／`PYTHON*`／`GIT_*`、PATH 固定；supervisor 以清單刪除 `MEM*`、`SIZING_*`、`DOCKER_*`、`I074_STAGE2_*` 等）。⚠️ 呼叫端帶 `I074_STAGE2_*` 時入口直接拒絕；`HOME`、`XDG_DATA_HOME`（identity 的位置）照傳 |
| observer（D4） | wrapper **在 ⑩ 之前**以背景啟動 observer（`--work-dir` ＝ 本次的 work、`--out` ＝ `~/i074_stage2_observe/<NAME>`，輸出到 `observe.log`）、記下它的 PID ＋ starttime；⑩ 結束之後以 TERM 停止（PID ＋ starttime 相符才送），最多等 120 秒、記下結束碼——**observer 啟動失敗或沒有結束都⛔ 不影響 ⑩ 與 rc marker**。它在 supervisor 之外啟動，不屬於「本趟」（「八之一之二」第 5 列）。⚠️ **啟動驗證只看主流程**（v2）：observer 的 starttime 讀不到（例如它立刻結束）就記成 `-`，⛔ 不算啟動失敗；observer 的狀態另印一行。報告缺失時的處理見「五」6、7 |
| 程序身分 | `harness_pid` ＝ ⑩ 入口的 PID；入口 `exec` 成 supervisor 之後 PID 與 starttime 都不變，所以 `/proc/<pid>/cmdline` 會從 `run-i074-stage2.sh` 變成 `i074-stage2-supervisor.py`。判斷存活一律用 PID ＋ starttime |
| 完成 | **rc marker 存在 ⟺ ⑩ 已結束**（wrapper 在 `wait` 之後、停止 observer 之後才寫）；② 的最後一行是判斷（v3，依序）：查詢失敗 → 等待；⑩ 的程序或帶 `i074.stage2.run` 的容器仍在 → 等待（容器停了卻不消失 → 人工判讀）；有 rc marker → 可以啟動 Claude；⛔ 沒有 rc marker 而 wrapper 與 ⑩ 都確定已結束、程序、容器、sentinel 都不在 →「四」的「不明」列（wrapper 被殺或 host 重開），可以啟動 Claude；⛔ 沒有 rc marker 而 sentinel 仍在或身分不明 → 人工判讀（⛔ 不自動放行） |
| ⚠️ 等待期間 | ⛔ 不啟動 Claude（D1：直到 rc marker 出現——finalize 與晉升的容器在最後才依當下的 MemAvailable 決定上限）、⛔ 不使用 Codex、⛔ 不跑測試、⛔ 不 build image；真正 repo 遵守操作契約（⛔ 不搬移、刪除或替換 `python/`、`python/baselines/`、`i074_stage2/`、`failed/` 本身，⛔ 不動 `.promote-staging-*` 與 `.gitignore` 的 staging 規則；其他編輯與 commit 照常——但 Claude 結束時本來就⛔ 不會有）。只用「②」查看；① 的驗證沒有通過時也一樣（⑩ 可能仍在執行）：**依 ② 最後一行的判斷，判斷是「可以啟動 Claude」之前⛔ 不啟動 Claude** |

```text id="i074_stage2_step10_launch_001"
# ① 啟動：控制目錄（tmpfs）、wrapper 腳本、身分紀錄——每一步失敗即停（整段是兩個函式（啟動與收尾），貼上之後立刻執行、執行完就移除）
s10_launch() {
local i k f o
cd /home/dev/workspace/stock_trading || { echo "cd 失敗：⛔ 不啟動"; return 1; }
EXPECTED="$(git rev-parse --verify HEAD)" && [[ "$EXPECTED" =~ ^[0-9a-f]{40}$ ]] || { echo "git rev-parse 失敗：⛔ 不啟動"; return 1; }
NAME="formal-10-$(date -u +%Y%m%dT%H%M%SZ)" && [[ "$NAME" =~ ^formal-10-[0-9]{8}T[0-9]{6}Z$ ]] || { echo "NAME 不對（「$NAME」）：⛔ 不啟動"; return 1; }
WORK="$HOME/i074_stage2_formal/$NAME"; OBS="$HOME/i074_stage2_observe/$NAME"; CTL="/dev/shm/s10-$NAME"
mkdir -p "$HOME/i074_stage2_formal" "$HOME/i074_stage2_observe" || { echo "建上層目錄失敗：⛔ 不啟動"; return 1; }
[ ! -e "$WORK" ] && [ ! -L "$WORK" ] && [ ! -e "$OBS" ] && [ ! -L "$OBS" ] || { echo "work 或 observer 的目錄已存在：⛔ 不啟動"; return 1; }
mkdir -m 700 "$CTL" || { echo "mkdir 失敗：⛔ 不碰既有的 $CTL、⛔ 不啟動"; return 1; }                    # 原子的排他：同一秒的第二次啟動停在這裡
printf 'expected\t%s\nname\t%s\nwork\t%s\nobserve\t%s\nlaunched_utc\t%s\n' "$EXPECTED" "$NAME" "$WORK" "$OBS" "$(date -u +%FT%TZ)" > "$CTL/run.tsv" \
  || { echo "寫 run.tsv 失敗：$CTL 只建了一半、⛔ 沒有啟動"; return 1; }
cat > "$CTL/wrapper.sh" <<'WRAP' || { echo "寫 wrapper.sh 失敗：$CTL 只建了一半、⛔ 沒有啟動"; return 1; }
ctl="$1"; work="$2"; obs="$3"
st()  { awk '{ sub(/.*\) /, ""); print $20 }' "/proc/$1/stat" 2>/dev/null; }   # /proc/<pid>/stat 的 starttime（第 22 欄）
run() { local s; s="$(awk '{ sub(/.*\) /, ""); print $1 }' "/proc/$1/stat" 2>/dev/null)"; [ -n "$s" ] && [ "$s" != Z ]; }   # 存在而且⛔ 不是 zombie
printf 'wrapper_pid\t%s\nwrapper_start\t%s\nwrapper_pgid\t%s\n' "$$" "$(st $$)" "$(ps -o pgid= -p $$ | tr -d ' ')" >> "$ctl/run.tsv"
python3 -B python/scripts/i074_stage2_sizing.py observe --work-dir "$work" --out "$obs" < /dev/null > "$ctl/observe.log" 2>&1 &
op=$!; os="$(st "$op")"; [ -n "$os" ] || os=-                                  # 讀不到（例如它立刻結束）記成 -：⛔ 不算 ⑩ 的啟動失敗
printf 'observer_pid\t%s\nobserver_start\t%s\n' "$op" "$os" >> "$ctl/run.tsv"
sleep 2                                                                       # observer 先開始取樣；它失敗⛔ 不影響 ⑩
REPLAY_IMAGE_ID=sha256:2a90ad1c1dd801d59373988a4d84afe1437cbee62ccab3709b06cfa8f7da5027 \
  scripts/run-i074-stage2.sh --freeze-record /home/dev/i074_stage2_sizing/formal-92-20261027T090656Z/freeze_record.json \
  --work-dir "$work" < /dev/null &
hp=$!
printf 'harness_pid\t%s\nharness_start\t%s\n' "$hp" "$(st "$hp" || true)" >> "$ctl/run.tsv"
wait "$hp"; rc=$?
if [ "$os" != - ] && [ "$(st "$op")" = "$os" ]; then kill -TERM "$op" 2>/dev/null; fi   # PID ＋ starttime 相符才送
for i in $(seq 1 120); do run "$op" || break; sleep 1; done                  # observer 寫報告最多等 120 秒
if run "$op"; then orc=未結束; else wait "$op" 2>/dev/null; orc=$?; fi
printf 'observer_rc\t%s\n' "$orc" >> "$ctl/run.tsv"
printf '%s\t%s\n' "$rc" "$(date -u +%FT%TZ)" > "$ctl/rc.tmp" && mv "$ctl/rc.tmp" "$ctl/rc"   # rc marker（原子寫入）
WRAP
setsid nohup bash "$CTL/wrapper.sh" "$CTL" "$WORK" "$OBS" > "$CTL/log" 2>&1 < /dev/null &
for i in $(seq 1 15); do                                  # 啟動之後的驗證（最多 15 秒）：7 個身分欄位各恰好一行；主流程的 5 個必須是正整數
  sleep 1
  [ -e "$CTL/rc" ] && { cat "$CTL/run.tsv"; echo "⑩ 已經結束（rc $(cut -f1 "$CTL/rc")）：開跑失敗——⛔ 不要再啟動；先執行 ②，照它最後一行的判斷"; return 1; }
  f=0; for k in wrapper_pid wrapper_start wrapper_pgid harness_pid harness_start observer_pid observer_start; do
    [ "$(awk -F'\t' -v k="$k" '$1 == k' "$CTL/run.tsv" | wc -l)" = 1 ] || f=1
    case "$k" in
      observer_*) ;;                                      # observer ⛔ 不是前置：⛔ 不驗值（可以是 -）
      *) [[ "$(awk -F'\t' -v k="$k" '$1 == k { print $2 }' "$CTL/run.tsv")" =~ ^[1-9][0-9]*$ ]] || f=1 ;;
    esac
  done
  [ "$f" = 0 ] && break
done
cat "$CTL/run.tsv"
[ "$f" = 0 ] || { echo "run.tsv 在 15 秒內⛔ 沒有寫齊：⑩ 可能仍在執行——⛔ 不要再啟動、⛔ 不啟動 Claude；用 ② 查看，依它最後一行的判斷"; return 1; }
for k in wrapper harness; do                               # PID ＋ starttime 都相符：wrapper 與 ⑩ 都還在
  [ "$(awk '{ sub(/.*\) /, ""); print $20 }' "/proc/$(awk -F'\t' -v k="${k}_pid" '$1 == k { print $2 }' "$CTL/run.tsv")/stat" 2>/dev/null)" \
    = "$(awk -F'\t' -v k="${k}_start" '$1 == k { print $2 }' "$CTL/run.tsv")" ] \
    || { echo "$k 的身分無法確認（或已結束）：⛔ 不要再啟動、⛔ 不啟動 Claude；用 ② 查看，依它最後一行的判斷"; return 1; }
done
o="$(awk -F'\t' '$1 == "observer_start" { print $2 }' "$CTL/run.tsv")"     # observer 只報告、⛔ 不擋 ⑩
if [ "$o" != - ] && [ "$(awk '{ sub(/.*\) /, ""); if ($1 != "Z") print $20 }' "/proc/$(awk -F'\t' '$1 == "observer_pid" { print $2 }' "$CTL/run.tsv")/stat" 2>/dev/null)" = "$o" ]; then
  echo "observer：執行中"
else
  echo "observer：⚠️ 沒有在執行——⑩ 照常（只影響觀測；看 $CTL/observe.log，結果照實記錄）"
fi
echo "已啟動：NAME＝$NAME（CTL＝$CTL；之後用 ② 查看）"
}
s10_done() { unset -f s10_launch s10_done; echo "① 的結束碼：$1（0 以外：照上一行的訊息處理）"; return "$1"; }   # 收尾：移除兩個函式、印出並回傳結束碼
s10_launch; s10_done "$?"

# ② 狀態（任何時候都可以執行；CTL 換成實際的目錄）——最後一行是判斷：「可以啟動 Claude」之前⛔ 不啟動 Claude
CTL=/dev/shm/s10-<NAME>
v()     { awk -F'\t' -v k="$1" '$1 == k { print $2 }' "$CTL/run.tsv" 2>/dev/null; }
st()    { awk '{ sub(/.*\) /, ""); if ($1 != "Z") print $20 }' "/proc/$1/stat" 2>/dev/null; }   # 存在而且⛔ 不是 zombie 才印 starttime
alive() { [ -n "$1" ] && [ -n "$2" ] && [ "$2" != - ] && [ "$(st "$1")" = "$2" ]; }    # pid ＋ starttime 都相符才算同一個程序
idn()   { local p s; p="$(v "${1}_pid")"; s="$(v "${1}_start")"                       # 欄位缺失 → 不明（⛔ 不能當作已結束）
          if [ -z "$p" ] || [ -z "$s" ] || [ "$s" = - ]; then echo 不明; elif alive "$p" "$s"; then echo 執行中; else echo 已結束; fi; }
RC="$(cut -f1 "$CTL/rc" 2>/dev/null)"; A=/run/lock/i074-stage2.active; SE="$({ [ -e "$A" ] || [ -L "$A" ]; } && echo 有 || echo 無)"
PL="$(ps -eo pid,lstart,args)"; RPL=$?                                        # ps 先完整取值，再交給 awk
SP="$(awk -v w="$CTL/wrapper.sh" 'index($0, w) || /run-i074-stage2\.sh|i074-stage2-supervisor\.py/' <<< "$PL")"; RSP=$?
CN="$(docker ps -a --filter label=i074.stage2.run --format '{{.ID}} {{.Status}} {{.Names}}')"; RCN=$?   # 帶 ⑩ 標籤的容器（含已停止的）
echo "⑩：$(idn harness)；wrapper：$(idn wrapper)；observer：$(idn observer)；rc：${RC:-無}；sentinel：$SE"
printf 'wrapper／⑩ 的入口、supervisor、orchestrator 的程序：%s\n帶 i074.stage2.run 的容器：%s\n（rc：ps %s、awk %s、docker ps %s）\n' "${SP:-無}" "${CN:-無}" "$RPL" "$RSP" "$RCN"
echo "state：$(ls "$(v work)/state" 2>/dev/null | tr '\n' ' ')"
tail -3 "$CTL/log"
if [ "$RPL" != 0 ] || [ "$RSP" != 0 ] || [ -z "$PL" ] || [ "$RCN" != 0 ]; then    # 判斷依序（v3）：查詢失敗 → 程序或容器 → rc marker → 不明
  echo "判斷：查詢失敗——繼續等待、稍後再執行 ②（⛔ 不啟動 Claude）"
elif [ -n "$SP" ] || [ -n "$CN" ]; then
  echo "判斷：⑩ 的程序或容器仍在——繼續等待（⛔ 不啟動 Claude、⛔ 不要再啟動；容器已停止卻一直不消失 → 人工判讀）"
elif [ -n "$RC" ]; then
  echo "判斷：⑩ 已結束（rc $RC；sentinel：$SE），程序與容器都不在——可以啟動 Claude，依「四」判讀"
elif [ "$SE" = 無 ] && [ "$(idn wrapper)" = 已結束 ] && [ "$(idn harness)" = 已結束 ]; then
  echo "判斷：⛔ 沒有 rc marker，而 wrapper 與 ⑩ 都已結束，程序、容器、sentinel 都不在——「四」的「不明」列：可以啟動 Claude"
else
  echo "判斷：⛔ 沒有 rc marker，而 sentinel 仍在、或身分不明——人工判讀（⛔ 不自動放行；看上面各行，再依「四」的「sentinel 仍存在」或「不明」列）"
fi

# ③ 安全中止（只在必要時——replay 的容器啟動之後中止會佔物理啟動的額度，見「四」；延用 ② 的函式）：身分相符才對 supervisor 送一次 TERM
hp="$(v harness_pid)"; hs="$(v harness_start)"
if alive "$hp" "$hs" && grep -qa 'i074-stage2-supervisor.py' "/proc/$hp/cmdline"; then kill -TERM "$hp"; else echo "身分不符或已結束：⛔ 不送訊號"; fi
```

⚠️ ③ 之後 supervisor 依「八之一之二」第 8 列正常釋放（後代 TERM 20 秒 → KILL 5 秒、移除本趟的容器、刪 sentinel、放鎖），回 143；
⛔ 不要再送第二次、⛔ 不要對其他程序送訊號。之後依「四」判讀。

④ `--resume`／`--promote`（**只在「四」指示時**才用；同樣由使用者在自己的終端機執行、Claude 結束）：先把第二行的兩個值改成實際的，再整段貼上。
前置條件（P3 的 image、identity 與它的 SHA，P5，P7 的 sentinel、容器與程序）的輸出存進控制目錄的 `precheck.txt`；之後的狀態一樣用 ②（CTL 換成 ④ 印出的目錄）。

```text id="i074_stage2_step10_resume_001"
# ④ --resume／--promote：MODE 是 resume 或 promote；N 是 ① 印出的 NAME（⛔ 不改就會被拒絕）
MODE="resume"; N="formal-10-<UTC>"
s10_rp() {
local mode="$1" name="$2" i k f WORK CTL PCO RPC
cd /home/dev/workspace/stock_trading || { echo "cd 失敗：⛔ 不啟動"; return 1; }
case "$mode" in resume|promote) ;; *) echo "MODE 只能是 resume 或 promote（「$mode」）：⛔ 不啟動"; return 1 ;; esac
[[ "$name" =~ ^formal-10-[0-9]{8}T[0-9]{6}Z$ ]] || { echo "N 不對（「$name」）：⛔ 不啟動"; return 1; }
WORK="$HOME/i074_stage2_formal/$name"
[ -d "$WORK" ] && [ ! -L "$WORK" ] || { echo "work 不存在或是 symlink（$WORK）：⛔ 不啟動"; return 1; }
# 前置條件（「四」末段）：任一項⛔ 不成立就⛔ 不啟動；在子 shell 裡跑，輸出之後存進控制目錄
PCO="$(
  date -u +'④ precheck %FT%TZ'
  IMG=sha256:2a90ad1c1dd801d59373988a4d84afe1437cbee62ccab3709b06cfa8f7da5027
  ID="${XDG_DATA_HOME:-$HOME/.local/share}/stock_trading/i074_stage2/run_identity.json"
  docker image inspect "$IMG" > /dev/null || { echo "P3 image ⛔ 不在本機"; exit 1; }; echo "P3 image ok"
  python3 -c 'import json,sys; sys.exit(0 if json.load(open(sys.argv[1]))["expected_image_id"] == sys.argv[2] else 1)' "$ID" "$IMG" \
    || { echo "P3 identity ⛔ 不符"; exit 1; }; echo "P3 identity ok"
  IS="$(sha256sum < "$ID" | cut -d' ' -f1)"
  [ "$IS" = a02bf3bd3f44cadb8d9df2e5e9d7d6b89d92cca2130e2e85526cdc8b139d9f4d ] || { echo "P3 identity 的 SHA ⛔ 不符（${IS:-讀不到}）"; exit 1; }; echo "P3 identity SHA ok"
  awk '/^MemAvailable:/ { m = int($2 / 1024); f = 1 } END { printf "P5 MemAvailable %d MiB（門檻 950）\n", m; exit !(f && m >= 950) }' /proc/meminfo \
    || { echo "P5 ⛔ 不成立"; exit 1; }; echo "P5 ok"
  A7=/run/lock/i074-stage2.active; [ ! -e "$A7" ] && [ ! -L "$A7" ] || { echo "P7 sentinel 存在（只能重開機解除，⛔ 不得手動刪除）"; exit 1; }
  L7="$(docker ps -a --filter label=i074.stage2.run -q)" || { echo "P7 docker ps 失敗"; exit 1; }
  [ -z "$L7" ] || { echo "P7 帶 i074.stage2.run 的容器：$L7"; exit 1; }
  PP="$(ps -eo pid,args)" && [ -n "$PP" ] || { echo "P7 ps 失敗"; exit 1; }
  P7="$(awk '/run-i074-stage2\.sh|i074-stage2-supervisor\.py|i074-stage2-(acceptance|sizing)\.sh|i074_stage2_sizing\.py observe/' <<< "$PP")" \
    || { echo "P7 awk 失敗"; exit 1; }
  [ -z "$P7" ] || { echo "P7 程序：$P7"; exit 1; }
  echo "P7 ok（sentinel、容器、程序）"
)"; RPC=$?
printf '%s\n' "$PCO"
[ "$RPC" = 0 ] || { echo "④ 的前置條件⛔ 不成立：⛔ 不啟動"; return 1; }
CTL="/dev/shm/s10-$name-$mode-$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -m 700 "$CTL" || { echo "mkdir 失敗：⛔ 不碰既有的 $CTL、⛔ 不啟動"; return 1; }
printf '%s\n' "$PCO" > "$CTL/precheck.txt" \
  && printf 'expected\t%s\nname\t%s\nwork\t%s\nmode\t%s\nlaunched_utc\t%s\n' "$(git rev-parse --verify HEAD)" "$name" "$WORK" "$mode" "$(date -u +%FT%TZ)" > "$CTL/run.tsv" \
  || { echo "寫控制目錄失敗：$CTL 只建了一半、⛔ 沒有啟動"; return 1; }
cat > "$CTL/wrapper.sh" <<'WRAP' || { echo "寫 wrapper.sh 失敗：$CTL 只建了一半、⛔ 沒有啟動"; return 1; }
ctl="$1"; work="$2"; mode="$3"
st()  { awk '{ sub(/.*\) /, ""); print $20 }' "/proc/$1/stat" 2>/dev/null; }   # /proc/<pid>/stat 的 starttime（第 22 欄）
printf 'wrapper_pid\t%s\nwrapper_start\t%s\nwrapper_pgid\t%s\n' "$$" "$(st $$)" "$(ps -o pgid= -p $$ | tr -d ' ')" >> "$ctl/run.tsv"
REPLAY_IMAGE_ID=sha256:2a90ad1c1dd801d59373988a4d84afe1437cbee62ccab3709b06cfa8f7da5027 \
  scripts/run-i074-stage2.sh "--$mode" --work-dir "$work" < /dev/null &
hp=$!
printf 'harness_pid\t%s\nharness_start\t%s\n' "$hp" "$(st "$hp" || true)" >> "$ctl/run.tsv"
wait "$hp"; rc=$?
printf '%s\t%s\n' "$rc" "$(date -u +%FT%TZ)" > "$ctl/rc.tmp" && mv "$ctl/rc.tmp" "$ctl/rc"   # rc marker（原子寫入）
WRAP
setsid nohup bash "$CTL/wrapper.sh" "$CTL" "$WORK" "$mode" > "$CTL/log" 2>&1 < /dev/null &
for i in $(seq 1 15); do                                  # 啟動之後的驗證（最多 15 秒）：5 個身分欄位各恰好一行、都是正整數
  sleep 1
  [ -e "$CTL/rc" ] && { cat "$CTL/run.tsv"; echo "已經結束（rc $(cut -f1 "$CTL/rc")）：⛔ 不要再啟動；先執行 ②（CTL＝$CTL），照它最後一行的判斷"; return 1; }
  f=0; for k in wrapper_pid wrapper_start wrapper_pgid harness_pid harness_start; do
    [ "$(awk -F'\t' -v k="$k" '$1 == k' "$CTL/run.tsv" | wc -l)" = 1 ] \
      && [[ "$(awk -F'\t' -v k="$k" '$1 == k { print $2 }' "$CTL/run.tsv")" =~ ^[1-9][0-9]*$ ]] || f=1
  done
  [ "$f" = 0 ] && break
done
cat "$CTL/run.tsv"
[ "$f" = 0 ] || { echo "run.tsv 在 15 秒內⛔ 沒有寫齊：可能仍在執行——⛔ 不要再啟動、⛔ 不啟動 Claude；用 ②（CTL＝$CTL）查看，依它最後一行的判斷"; return 1; }
for k in wrapper harness; do                               # PID ＋ starttime 都相符：wrapper 與 supervisor 都還在
  [ "$(awk '{ sub(/.*\) /, ""); print $20 }' "/proc/$(awk -F'\t' -v k="${k}_pid" '$1 == k { print $2 }' "$CTL/run.tsv")/stat" 2>/dev/null)" \
    = "$(awk -F'\t' -v k="${k}_start" '$1 == k { print $2 }' "$CTL/run.tsv")" ] \
    || { echo "$k 的身分無法確認（或已結束）：⛔ 不要再啟動、⛔ 不啟動 Claude；用 ②（CTL＝$CTL）查看，依它最後一行的判斷"; return 1; }
done
echo "已啟動：CTL＝$CTL（之後用 ② 查看，把 ② 的 CTL 換成這個目錄）"
}
s10_rp_done() { unset -f s10_rp s10_rp_done; echo "④ 的結束碼：$1（0 以外：照上一行的訊息處理）"; return "$1"; }   # 收尾：移除兩個函式、印出並回傳結束碼
s10_rp "$MODE" "$N"; s10_rp_done "$?"
```

## 四、判讀與分支（一律依 rc ＋ `<work>/state/` ＋ 真正 repo 的磁碟事實；每一次嘗試都記進結果）

**先看 sentinel**（v2）：rc marker 出現之後，⛔ 不論 rc，`/run/lock/i074-stage2.active` 仍存在（`-e` 或 `-L`）→ 一律走下表的「sentinel 仍存在」列；
sentinel ⛔ 不存在才依 rc 對照其他列。

**計次的分界**（v2）：留下 artifact（成功 archive 或 failed record）才佔上限 ①。`state/replay_started.json` 寫在呼叫 runner **之前**，只代表「準備啟動」，
⛔ 不能單獨當作物理啟動。它不存在 → ⛔ 不計入任何上限（「正式 scan 的計次裁決」：preflight 與指紋檢查失敗⛔ 不計入）。它存在而
`replay_done.json` ⛔ 不存在時，依 `<work>/logs/replay.{out,err}`、observer 的報告與 `btime` **依序**分成三種（v3：只認容器啟動的直接證據）：

| 分類（依序判斷） | 證據 | 計次 |
|---|---|---|
| **已啟動** | 有**容器內程序的直接證據**，任一：observer 報告的 `replay_seen` 是 `true`；`logs/replay.err` 有只有容器內的 Python 才會印的路徑——`/usr/local/lib/python3.11/` 或 `File "/app/`（host 是 Python 3.9.2，兩個路徑都不存在。⑨-1 實測：量測趟與 failure 趟的 `replay.err` 都在 mem-guard 那一行之後、開跑約 1 分鐘內出現 3 個 sklearn 的 `InconsistentVersionWarning`，是容器載入模型時印的）。⚠️ ⛔ 不用 replay 的 rc 推論：runner 印完 `==> offline replay：`、還沒 `exec docker run` 就被訊號終止，也會留下 137／143 | 佔上限 ② 一格、⛔ 不佔 ① |
| **沒有進到 Docker** | 全部成立：`logs/replay.out` 是一般檔（⛔ 不是 symlink）而且⛔ 沒有以 `==> offline replay：` 開頭的行（runner 在 `docker run` 之前的檢查——mem-guard 下限、run identity、artifact、patch canonical、image——就結束了）；host 自 `<NAME>` 的 UTC 時間之後⛔ 沒有重開機（`/proc/stat` 的 `btime` 早於它——重開機可能讓還沒寫回磁碟的 log 缺行，缺行⛔ 不能證明沒有啟動）；也⛔ 沒有上一列的直接證據 | ⛔ 不計入（同 preflight 與指紋檢查失敗） |
| **無法確認** | 其他全部，例如：有 `==> offline replay：` 的行但沒有直接證據（⛔ 不論 rc）；重開機過；`replay.out` 不存在或不是一般檔 | **停下、由使用者裁決**是否計入 ② |

⚠️ observer 的 `replay_seen` 是 `false` 或沒有報告，⛔ 不能當作「沒有啟動」（observer ⛔ 不是前置，可能沒在跑）。分類與依據（引用的那幾行原樣）記進結果。
**降級**：host 在「五」完成之前重開過（`btime` 晚於 `<NAME>` 的 UTC 時間）→ 控制目錄與 observer 的狀態消失、sentinel 也隨 `/run/lock` 消失；
`<work>` 與真正 repo 的檔案仍在（durability 由 ⑩ 自己的 fsync 與 `--promote` 處理）——先停下回報，由「三」的 ② 改看 `<work>/state/` 與真正 repo，再依下表。

| rc ／ 狀態 | 判讀 | 處置 |
|---|---|---|
| **0** | 成功 archive 已在真正 repo durable（佔上限 ① 與 ② 各一格） | 「五」的驗證全做 → **⑩ 成功** → 記錄 → review → 使用者 commit（`python/baselines/i074_stage2/evidence/` ＋ 文件）→ ⑪ |
| **6** | 反事實失效：failed record 已在真正 repo durable（佔 ① 與 ② 各一格） | 「五」適用的部分 → 記錄 → **使用者必須先 commit 這筆 failed record**（下一次 ⑩ 的 lookup 才看得到）→ ⛔ 不得重跑同一個語意 SHA；before 重跑的資格（不同的 counterfactual 語意 SHA、只有白名單產品檔的 diff 改變）牽涉新的 patch → ⑨、⑨-1、⑨-2 全部重做——**停下回報、另立計畫** |
| **2** | preflight 的 lookup 命中（⛔ 不計入） | 停下回報（目前 HEAD ⛔ 沒有任何 failed record，⛔ 不應發生） |
| **1**，`replay_started.json` ⛔ 不存在 | preflight 或之前的步驟失敗、還沒有終態（⛔ 不計入） | 依 `log` 的 `ERROR:` 與 `<work>/logs/` 查明：⛔ 不需要改程式（暫時性的 docker 錯誤、批次撞期、磁碟、記憶體、殘留）→ 重新確認 P1～P10 之後以**新的** work 與控制目錄重跑；**同一個原因連續 2 次就停下回報**；需要改程式 → 停下、另立計畫（改了 `scripts/`、`python/` → ⑨-2 重跑） |
| **1**，`replay_started.json` 存在、`replay_done.json` ⛔ 不存在 | runner 在 Docker 之前就結束、replay 回 0／6 以外的值、或輸出形狀不符——⛔ 沒有 artifact、⛔ 不佔 ①；⛔ 不能 `--resume`。依上面的三分 | **沒有進到 Docker** → 同上一列（⛔ 不計入；原因多半是記憶體下限或環境，看 `<work>/logs/replay.err`）。**已啟動**或**無法確認** → **停下回報**（D5）：記事故紀錄（`<work>/logs/replay.{out,err}`、observer 的峰值與 MemAvailable、容器上限、dmesg 的 OOM）；已啟動 → 是否用掉 before 正式趟**唯一一次的崩潰重試**由使用者裁決；無法確認 → 是否計入 ② 也由使用者裁決；⛔ 不自動重跑 |
| **1**，`replay_done.json` 與 `attempt.json` 都存在 | finalize／publish 回 1 而且沒有終態（⛔ 不佔任何上限：finalize 失敗只重跑 finalize） | 查明原因 → `scripts/run-i074-stage2.sh --resume --work-dir <work>`（同一個控制目錄的「三」④ 指令）；⛔ 不 replay。同一個原因連續 2 次就停下回報 |
| **1**，`replay_done.json` 存在、`attempt.json` ⛔ 不存在、⛔ 沒有終態 | replay 之後的檢查點不符 | 停下回報、另立 issue（⛔ 不發布、⛔ 不能 `--resume`） |
| **3** | 某一層的 durability 未確認 | `--promote`（冪等）；直到 0／6；同一個原因連續 2 次就停下回報 |
| **8** | 晉升沒有確認完成、可以安全重跑 | 排除原因（空間、另一個 `--promote`）之後 `--promote` |
| **9** | 需要人工判斷 | 停下回報：看 `<work>/logs/promote.err`；⛔ 不手動搬檔、⛔ 不自動重跑 |
| **128＋N**（例如 ③ 之後的 143；137 見下一列） | 被訊號中斷、supervisor 已清空本趟並刪 sentinel | 先依 `state/` 與上面的三分判斷 replay 有沒有啟動（無法確認 → 由使用者裁決）→ `--promote`，由它依磁碟事實裁決（有終態 → 0／6；沒有 → 9，交人工依計次政策處理） |
| **137**，sentinel ⛔ 不存在 | 持鎖階段的子程序被 SIGKILL（常見是 host 的 OOM killer）；supervisor 已清空本趟、刪 sentinel 之後才回傳——**⛔ 不需要重開機** | 先確認沒有殘留（帶 `i074.stage2.run` 鍵的容器、⑩ 的程序——P7 的那兩項；任一個指令失敗⛔ 不能當作沒有）、記下 `dmesg` 的 OOM 紀錄 → 同「128＋N」列。⚠️ sentinel ⛔ 不存在卻有殘留 ＝ 異常：**停下回報**（⛔ 不重開機、⛔ 不自己清） |
| **sentinel 仍存在**（⛔ 不論 rc；常見是 137——supervisor 自己被 SIGKILL；收回失敗時是 1） | supervisor 的清理沒有完成 | **停下回報**：sentinel 只能靠**重開機**解除（⛔ 不得手動刪除；重開機會中斷 live 服務——v29 決策 9，使用者已知並接受）。重開機由使用者決定時機；之後先查 `docker ps -a --filter label=i074.stage2.run`，有殘留就 `docker rm -f`，再 `--promote` 依磁碟事實裁決 |
| ⛔ 沒有 rc marker（wrapper 被殺或 host 重開；② 的判斷是「不明」列） | 不明 | 停下回報：以 ② 與 `state/`、真正 repo 的 `evidence/`／`failed/`、sentinel 判斷，再對照上面各列 |

⚠️ `--resume`／`--promote` 也要經 supervisor 取同一把鎖、同一套 sentinel；同樣由使用者在自己的終端機執行（Claude 結束），前置條件只需 P3、P5、P7 的
sentinel、容器與程序（⛔ 不需要 P4 的時段——晉升只要幾分鐘，但仍避開批次的開始時刻；容器是實作時加上的：supervisor 本來就拒絕殘留的 label 容器，提早在 precheck 擋下），
指令在「三」④（與 ① 同一套 wrapper、⛔ 沒有 observer；控制目錄 `/dev/shm/s10-<NAME>-<resume|promote>-<UTC>/`；狀態一樣用 ②，把 CTL 換成這個目錄）。

## 五、驗證與保存（rc 0 時全做；其他結果做適用的部分）

1. **真正 repo**（v2：`--untracked-files=all` 會逐檔列出）：
   * `git status --porcelain --untracked-files=all --ignored -- python/baselines/i074_stage2/` 成功、輸出非空，而且**每一行**都以 `?? python/baselines/i074_stage2/evidence/` 開頭（rc 6 時是 `?? python/baselines/i074_stage2/failed/<bundle>-<語意 SHA>/`，`<bundle>`、語意 SHA 取自 `state/preflight.json`）；⛔ 沒有 `!!`（含 `.promote-staging-*`）與其他狀態；行數 ＝「五」2 清單的檔案數。
   * `git status --porcelain --untracked-files=all -- python scripts .gitattributes` 去掉上面那些行之後為空（其他檔⛔ 沒有變化）；`git diff --cached --name-only` 為空（整個 repo ⛔ 沒有 staged 的變更——evidence 由使用者自己 stage）。
   * 任一個 `git` 指令失敗⛔ 不能當作成立。
2. **晉升 ＝ 來源**：真正 repo 的 `evidence/` 與複本內的 `<work>/repo/python/baselines/i074_stage2/evidence/` 逐檔相同（檔案清單與每檔 SHA-256 的排序清單相同）；記下清單本身的 SHA-256 當作錨。
3. **信任根**：`evidence_manifest.json` 的 `finalizer_provenance.base_commit` ＝ freeze record 的 `repo_head` ＝ 複本的 `HEAD` ＝ `E92`（晉升已驗，這裡從檔案再讀一次記錄）；manifest 的兩份 patch SHA、composed、語意 SHA ＝ 封存值；identity 與 image ＝ P3。
4. **`state/`**（v2：分開驗首次成功與恢復成功）：先以複本內的 `python3 -B <work>/repo/python/scripts/i074_stage2_preflight.py state-check --state-dir <work>/state --work-dir <work> --real-repo <真正 repo> --require run,preflight,replay_started,replay_done` 驗 schema 與 SHA 鏈（rc 0；它也會驗現存的 `attempt.json`），印出的 `rc` ＝ 0（rc 6 時 ＝ 6）。
   * **首次就成功**（「四」⛔ 沒有記下 finalize／publish 回 1 的嘗試）：`present` 恰好是 `run,preflight,replay_started,replay_done`（`attempt.json` ⛔ 不在）。
   * **恢復後成功**（有 rc 1 → `--resume`，或之後經 `--promote` 完成）：`present` 多了 `attempt`；它的 `kind` ＝ 失敗那一步、`rc` ＝ 1、`count` ＝「四」記下的 finalize／publish 失敗次數、`replay_done_sha256` ＝ 現在 `replay_done.json` 的 SHA-256。
   * `state/` 的每一份都原樣保存（⛔ 不刪、⛔ 不改——`attempt.json` 是合法的失敗沿革）；兩種之外的組合 → 停下回報。
5. **殘留**：sentinel ⛔ 不存在（鎖檔 `/run/lock/i074-stage2.lock` 會留著，永不 unlink——⛔ 不是殘留）；帶 `i074.stage2.run` 鍵與 `i074sz-` 的容器、⑩ 與 observer 的程序都沒有；`/dev/shm/i074-*` 為空；真正 repo 的 worktree 登記數不變（⑩ ⛔ 不寫真正 repo 的 `.git`）；`/tmp` 的新項目逐一歸因。
6. **數字與 observer**：`launched_utc` → rc marker 的耗時；`<work>/logs/` 各步驟的時間（replay 的開始與結束）；observer 的 `observation.json`（`observation_complete`、`replay_seen`、`missing_expected_containers`、每個容器的 cgroup 峰值下界與 `total_rss` 取樣最大值、MemAvailable 與 L0 的低點）——**只記錄、⛔ 不判定**；`<work>` 的大小與根檔案系統的可用空間。
   **報告缺失或不完整**（v2：observer 起不來、`observer_rc` 非 0 或「未結束」、`--out` 不存在或少了 `observation.json`、`observation_complete` 是 `false`）：照實記錄 `observer_start`、`observer_rc`、`observe.log`（全文）與 `--out` 的現況，結果寫「觀測報告缺失／不完整」；⛔ 不補量、⛔ 不為量測重跑 ⑩（v29「八」）。
7. **保存**：控制目錄 `/dev/shm/s10-<NAME>/` 的內容（含 `observe.log`）與 `$PRE` 先複製進 `~/i074_stage2_formal/<NAME>.ctl/`（`precheck.txt` mode 600），`cmp` 相同、重驗 `$PRE` 的三項之後才刪；用過「三」④ 的，每個 `/dev/shm/s10-<NAME>-<resume|promote>-<UTC>/` 也複製進 `<NAME>.ctl/<resume|promote>-<UTC>/`（含它的 `precheck.txt`），`cmp` 相同之後才刪；observer 的 `--out` 原位置保存（不存在就照實記錄）。
   **observer 沒有正常結束**（v2）：
   * `observer_rc` 是「未結束」→ Claude 確認同一個程序（PID ＋ starttime 相符、而且⛔ 不是 zombie，`/proc/<pid>/cmdline` 含 `i074_stage2_sizing.py` 與 `observe`）之後送一次 TERM、最多等 120 秒；仍在 → 同樣確認之後送 KILL。確認不了 → ⛔ 不送訊號、停下回報。
   * 它的狀態目錄留下（`observe.log` 的 `==> observer：…；狀態 <路徑>；…` 那一行印出的路徑，⛔ 不用 glob 找）→ 確認 observer 已結束（PID ＋ starttime ⛔ 不相符，`ps` 也⛔ 沒有 `i074_stage2_sizing.py observe` 的程序；任一個指令失敗⛔ 不能當作已結束）之後，`cp -a` 進 `.ctl/observe-state/`、`diff -r` 相同才刪——否則它會擋住之後 `--resume`／`--promote`／重跑的 P7。任何一項確認不了 → ⛔ 不刪、停下回報。
   `<work>` 與 ⑨-2 的保存：⑩ 的 `<work>`（複本、凍結 patch、runner 的輸出、logs、state）**在原位置保存到晉升結果的 commit 之後**（「八之三」）；⑨-2 的 `<work>` 也保存到這個 commit 之後（⑨-2 細部計畫「五」7）。`.ctl/`、observer 的報告與兩個 manifest 保存到 I-074 收斂。

## 六、交給 ⑪ 的規則

* ⑪ 只在 **rc 0 而且「五」全部成立**、使用者已 commit `python/baselines/i074_stage2/evidence/` 之後進行；判讀器是 `scripts/judge-i074-stage2.sh`（⛔ 沒有參數、⛔ 不寫真正 repo、可以重跑），規則見 `sr-zone-scoring.md`「I-074 Stage 2 的晉升與 B／C 判讀器」與「關閉條件」。
* 判讀器的錨點取自 `finalizer_provenance.base_commit`（＝ `E92`）的 git 物件：⛔ 不得改寫會讓 `832f678` 不可達的歷史（含 force push）。
* ⑩ 之後真正 repo 照常可以 commit（複本與晉升已結束）；evidence 的 commit 之後才可以刪 ⑩ 與 ⑨-2 的 `<work>`（由使用者決定）。
* rc 6 → ⛔ 沒有 ⑪（failed record ⛔ 不進 B／C），依「四」停下另立計畫。

## 七、歸檔與 commit

* `docs/issue.md` I-074：新節「Stage 2 步驟 ⑩ 正式執行結果」（結構比照 ⑨-1、⑨-2 的結果一節：結論與交給 ⑪ 的值、嘗試一覽、前置條件與 precheck 原樣、`run.tsv`、判讀、「五」的驗證與數字、observer 的摘要、計次的更新）；狀態列、roadmap（⑩ → 結果、⑪ →「現在在這裡」）；「正式 scan 的計次裁決」的計數只在結果一節記錄（⛔ 不改政策本身）。
* `sr-zone-scoring.md`「I-074 Stage 2 的晉升與 B／C 判讀器」：補 ⑩ 的正式值（evidence 的錨、`base_commit`、耗時）。
* `development-workflow.md`「I-074 Stage 2 的正式執行程序」：補 ⑩ 的 precheck、啟動（含 observer 由 wrapper 管理）、狀態、安全中止與 `--resume`／`--promote` 的區塊，以及「四」的判讀表（現況規格，⛔ 不只留在計畫書）；observer 一節補「由 wrapper 啟動與停止」。
* 本檔：結果之後標成「已執行／待 review」；review 確認、現況歸檔之後連同 I-074 的對應指標一起收斂（刪除之前必須已經 commit 過）。
* Claude ⛔ 不 commit：停下等 review；commit 指令由使用者執行（含 `python/baselines/i074_stage2/evidence/`）。

## 八、風險

| 風險 | 處置 |
|---|---|
| **supervisor 被 SIGKILL（host 的 OOM killer）→ sentinel 留下 → 只能重開機、中斷 live** | P5（950 MiB）、D1（Claude 結束、Codex ⛔ 不使用、等待期間⛔ 不啟動 Claude）、D2（避開批次）；observer 很小。發生時依「四」的「sentinel 仍存在」列停下回報（v2：137 而 sentinel ⛔ 不存在 ＝ 子程序被 SIGKILL，⛔ 不需要重開機） |
| replay 容器的上限偏低而在容器內被 OOM 砍掉（佔物理啟動的額度） | ⑩ ⛔ 無法固定 `MEM_MIN_MB`（supervisor 刪除 `MEM*`，照實記成已知限制）；以 P5 讓 replay 啟動時拿到完整的 700m（⑨-1：最大單一 RSS 394.3 MiB）。發生時依「四」停下回報，崩潰重試由使用者裁決。runner 的 mem-guard 在 Docker 之前就拒絕 → 「四」的「沒有進到 Docker」（⛔ 不計入） |
| 計次判讀錯誤（把「準備啟動」當成物理啟動而誤扣崩潰重試，或反過來） | v2：`replay_started.json` 只代表準備啟動。v3：「已啟動」只認容器內程序的直接證據（observer 的 `replay_seen`、`replay.err` 裡容器才有的路徑），⛔ 不用 rc 推論；「沒有進到 Docker」限 host ⛔ 沒有重開機；其他一律**停下由使用者裁決** |
| 批次撞期：記憶體與 IO 的競爭 | P4 ＋ D2（v3：以保守估計 3 小時 52 分計算、距下一個批次 ≥ 30 分鐘；平日 17:00 時段以晚間批次後移為前提，precheck 驗 live 的兩個 cron 鍵） |
| 晚間批次後移之後又被改回（例如 live 重新部署覆蓋了 env） | precheck 的「P4 晚間批次已後移 ok」⛔ 沒有印出 → 平日 17:00 時段⛔ 不開跑，改用 D2 的備選時段 |
| 磁碟不足 | P6（2 GB）；preflight 6 自己再驗 1,241,513,984；晉升另有 2 倍的預檢（不足 → 8，可以重跑 `--promote`） |
| 等待期間有人改了真正 repo 的 `python/baselines/i074_stage2/` | 操作契約（「三」）；違反時晉升回 9（⛔ 無法撤銷），依「四」停下 |
| 等待期間重新啟動 Claude，讓 finalize／晉升的容器上限偏低 | D1：直到 rc marker 出現都⛔ 不啟動 |
| host 在 ⑩ 期間重開機 | sentinel 隨 tmpfs 消失；replay 中斷 → 依「四」的三分判斷是否佔物理啟動；依「四」的降級與 `--promote` |
| wrapper 被殺、終端機關閉 | `setsid nohup`；rc marker 缺失 → ② 的判斷 →「四」的不明列 |
| observer 失敗、拖慢或沒結束 | 它⛔ 不是前置：wrapper ⛔ 不等它開始、最多等它結束 120 秒；① 的啟動驗證⛔ 不看它的值（v2）；報告缺失或不完整就照實記錄（「五」6）；沒有正常結束而留下的狀態目錄依「五」7 保存之後才刪（否則擋住下一次的 P7） |
| ① 的驗證沒有通過、或 wrapper 與 supervisor 都被殺，而使用者提早啟動 Claude（⑩ 的容器其實仍在跑） | v2：① 的失敗訊息改成「⛔ 不啟動 Claude、用 ② 查看」。v3：② 依序判斷——查詢失敗、程序或帶 `i074.stage2.run` 的容器仍在 → 等待；⛔ 沒有 rc marker 而 sentinel 仍在或身分不明 → 人工判讀（⛔ 不自動放行）；只有程序、容器都不在才印「可以啟動 Claude」 |
| 貼上版的區塊在 zsh 與 bash 的行為不同（例如 `"$VAR:…"` 的修飾符） | 實作時以假的入口與假的 observer，在 bash 與 zsh 各測一次（「九」） |
| ⑨-2 的 `<work>` 被動到 | P2（形狀、SHA、`check-basic`、`check-pair`、raw manifest） |

## 九、決定（✅ D1～D9 已定案，2026-10-28）

每一項的選項以 **(a)** 為建議。✅ 使用者於 2026-10-28 定奪：D1、D3～D9 全部採 (a)（與第三輪 review 的建議相同）；D2 另行裁決。

| # | 決定 | 選項 | 理由與影響 | 裁決 |
|---|---|---|---|---|
| D1 | 誰啟動、Claude 在不在 | **(a)** 與 ⑨-1、⑨-2 相同：使用者在 tmux／自己的終端機以「三」的 wrapper 啟動，Claude 先結束，直到 ② 的判斷是「可以啟動 Claude」才重新啟動（Codex 只記錄、⛔ 不使用）<br>(b) Claude 留著，由 Claude 啟動與監看 | 2 GiB 的 host 上 Claude 約 360～380 MiB，(b) 時 P5（950 MiB）不成立；最怕的是 supervisor 被 OOM killer 砍掉（sentinel → 只能重開機、中斷 live） | ✅ 使用者裁決（2026-10-28）：(a) |
| D2 | 時段 | 平日 17:00 時段（前提：晚間批次後移）／週末白天／平日深夜 | 見「裁決」 | ✅ **使用者裁決（2026-10-28）：首選平日 17:00 時段，前提是晚間兩個批次永久後移一小時**（`chip_daily_sync` 21:00 → 22:00、`sr_analysis_chip` 22:00 → 23:00；另立計畫 [`evening-batch-shift.md`](./evening-batch-shift.md)，完成並驗證之後才可用）。17:00 的 `sr_analysis` 已是終態之後做 precheck、開跑，**最晚 17:30 開跑、盡量靠近 17:00**（第三輪 review 的建議；17:30 ＋ 保守估計 3 小時 52 分 ＝ 21:22，距 22:00 ≥ 30 分鐘；一般情況約 20:30 結束）。使用者只需在 17:00～17:30 在場，之後到 rc marker 都⛔ 不需要人；Claude 關到 ② 判斷可以啟動為止。備選（後移完成之前、或 precheck ⛔ 沒有印出「P4 晚間批次已後移 ok」）：週末 06:30、07:00 的批次都已是終態之後開跑；或平日最後一個晚間批次（後移前 22:00、後移後 23:00）已是終態之後開跑；兩者都最晚 01:00 開跑（01:00 ＋ 3 小時 52 分 ＝ 04:52，距 06:30 ≥ 30 分鐘）。⛔ 不採用：後移完成之前的平日 17:00 時段（保守估計會撞到 21:00 的 `chip_daily_sync`）、平日白天。2026-10-28 補：「後移完成」以 T-075 的 A5 全部 ok 為準、⛔ 不等觀察一個平日晚上（使用者裁決，「十」4） |
| D3 | 記憶體門檻（P5） | **(a)** MemAvailable ≥ 950 MiB（與 ⑨-1 相同）<br>(b) 更高，例如 1,050 MiB<br>(c) 更低，例如 850 MiB | ⑩ 的容器上限只能靠開跑當下的 MemAvailable（`MEM*` 會被 supervisor 刪除）：950 − 漂移 100 − 預留 150 ＝ 完整的 700m。(b) 多留 100 MiB 的漂移，但較常開不了跑；(c) replay 只拿得到約 600m（⑨-1 的量測趟：最大單一 RSS 394.3 MiB、cgroup 476.1 MiB），餘裕變小 | ✅ 使用者裁決（2026-10-28）：(a) |
| D4 | observer | **(a)** 跑，由 wrapper 管理（「三」①：⑩ 之前啟動、結束之後 TERM；失敗⛔ 不影響 ⑩，① 只驗主流程）<br>(b) 照 `development-workflow.md` 在另一個終端機手動啟動、⑩ 結束之後 Ctrl-C<br>(c) 不跑 | v29「八」：⑩ 期間記錄實際峰值（⛔ 不作為前置）。(a) 有 PID ＋ starttime 的紀錄、⛔ 不需要第二個終端機；(b) 名稱要等 ① 印出之後才知道，⑩ 結束時要有人在場按 Ctrl-C（與 D2「18:00 之後⛔ 不需要人」衝突）；(c) 少一份峰值紀錄，「四」的計次也少一種直接證據（`replay_seen`） | ✅ 使用者裁決（2026-10-28）：(a) |
| D5 | `replay_started.json` 存在之後失敗（沒有 artifact） | **(a)** 依「四」的三分：沒有進到 Docker → ⛔ 不計入，同 preflight 失敗處理；已啟動 → 停下回報，由使用者裁決是否用掉唯一一次的崩潰重試；無法確認 → 停下回報，是否計入 ② 也由使用者裁決<br>(b) 一律停下由使用者裁決（連「沒有進到 Docker」也由使用者確認不計入） | 已啟動佔上限 ②，而且 before 正式趟只剩這一次重試，⛔ 不自動重跑。(b) 更保守，代價是 runner 在 Docker 之前就失敗（例如 mem-guard 下限）也要等使用者確認；兩者都由使用者重跑 | ✅ 使用者裁決（2026-10-28）：(a) |
| D6 | 安全中止的時機 | **(a)** 只在必要時（例如 live 出事、記憶體告急）由使用者執行「三」③<br>(b) 另訂自動中止的條件（例如 MemAvailable 低於某個值就中止） | replay 的容器啟動之前中止⛔ 不計入，之後中止 ＝ 物理啟動一次，分不出來由使用者裁決（「四」）。(b) 要另寫監看程式，而且自動中止在 replay 啟動之後會用掉物理啟動的額度；⑩ 只有一次正式趟 | ✅ 使用者裁決（2026-10-28）：(a) |
| D7 | 位置 | **(a)** work：`~/i074_stage2_formal/<NAME>`；observer：`~/i074_stage2_observe/<NAME>`；控制目錄：`/dev/shm/s10-<NAME>/`；`<NAME>` ＝ `formal-10-<UTC>`<br>(b) 其他位置（work ⛔ 不得在 repo 內、上層目錄必須已存在） | 比照 ⑨-1、⑨-2 的命名（`~/i074_stage2_acceptance`、`~/i074_stage2_sizing`）；控制目錄在 tmpfs、名稱⛔ 不是 `i074-*`（P7 的殘留檢查） | ✅ 使用者裁決（2026-10-28）：(a) |
| D8 | 成功之後的 commit | **(a)** Claude 記錄結果與歸檔 → review → 使用者一次 commit `python/baselines/i074_stage2/evidence/` 與文件 → ⑪<br>(b) evidence 先單獨 commit（讓 ⑪ 可以先跑），文件之後再 commit | (a) 一個 commit 同時有證據與它的紀錄，review 一次完成；(b) ⑪ 較早開始，但 evidence 的 commit 當下沒有對應的紀錄 | ✅ 使用者裁決（2026-10-28）：(a) |
| D9 | ⑨-2 結果的 review 狀態（P10 的依據） | **(a)** 依 2026-10-27 的 review 與 commit `62e1773`，在本計畫的文件修改裡把 ⑨-2 結果一節、狀態列與 roadmap 的「⚠️ 待 review」改成「✅ review 通過並 commit `62e1773`」<br>(b) 維持「⚠️ 待 review」，另做一次 ⑨-2 結果的 review | P10 要求 ⑨-2 已通過並 review。那次 review 的結論是「⑨-2 通過有原始證據支持，未發現需要重跑的疏失」，指出的兩項文件缺漏（正式 `<work>` 的保存規則、s92 的 review 狀態）已修正並併入 `62e1773` | ✅ 使用者裁決（2026-10-28）：(a) |

## 十、執行步驟

1. ✅ 本計畫 review（依「十二」的收斂條件；第三輪通過，2026-10-28）→ 使用者定奪 D1～D9（D1、D3～D9 全部 (a)，D2 另行裁決）→ 確認。
2. ✅ 實作（2026-10-28 已完成，第四輪 review 通過，結果見「十三」；只動文件與 scratchpad）：「三」④（`--resume`／`--promote` 的 wrapper 版）補進本檔；「二」「三」的區塊以假的入口、假的 observer 在 scratchpad 測過
   （bash 與 zsh 各一次：正常、入口立刻失敗、observer 起不來、observer 立刻結束（starttime 讀不到 → `-`，① 仍通過）、observer 忽略 TERM、work 或控制目錄已存在、`mkdir` 失敗、
   ③ 的身分不符、② 的每一個判斷分支（v3：查詢失敗、程序仍在、容器仍在、rc marker 已出現、⛔ 沒有 rc marker 而 sentinel 仍在、⛔ 沒有 rc marker 而身分不明——含 review 的最小案例「身分不明 ＋ sentinel 仍在」、全部不在）——
   容器以 PATH 前置的假 `docker` 模擬（⛔ 不建立真的容器），sentinel 的路徑在測試複本裡換成 scratchpad 的檔（⛔ 不碰 `/run/lock`）；precheck 以真的唯讀指令跑一次，
   Claude 執行中所以 D1 ⛔ 不成立是預期）；⛔ 不碰 `/run/lock`、⛔ 不建立帶 `i074.stage2.run` 的容器、⛔ 不執行真正的 ⑩ 入口（D9 的狀態更新已在確認時完成）；
   `docs/issue.md` 的指標 → ⛔ 不 commit，停下等 review。
3. ✅ review 通過（第四輪）→ 使用者 commit `a9cb41d`。
4. 在選定的時段（D2；平日 17:00 時段要等晚間批次後移的計畫完成並驗證）：使用者結束 Claude → 「二」的 precheck → 「三」① → 等待（約 3.5 小時；只用 ②）→ 完成判定。
   ⚠️ **使用者裁決（2026-10-28 下午）：當天（週三）17:00 執行。** T-075 的 A1～A5 已在 14:13 完成、A5 全部 ok（容器的 env 已是 22:00／23:00、⛔ 沒有 `cron register failed`），
   放行 T-075 的 D4「完成之後至少觀察一個平日晚上」與 T-075「七」4「文件同步 commit 之後」兩項；precheck 仍要印出「P4 晚間批次已後移 ok」。
   殘餘風險：後移⛔ 沒有實際生效（缺 22:00 實際執行的直接證據）→ 21:00 的 `chip_daily_sync` 碰到 ⑩ 的尾段。耗時都是估計：17:08 開跑 ＋ 保守估計 3 小時 52 分 ＝ 21:00，對舊排程**沒有緩衝**（一般約 20:20～20:40 結束）；
   若排程仍在 21:00，或執行超出估計，仍可能重疊。依新排程 22:00 計算，17:30 開跑的保守估計 21:22 結束、緩衝 38 分鐘，符合 P4（≥ 30 分鐘）。
   開跑仍依 D2：最晚 17:30、盡量靠近 17:00（盡量在 17:08 之前）。
5. 重新啟動 Claude：依「四」判讀 → 「五」的驗證與保存 → 「七」的紀錄與歸檔 → ⛔ 不 commit，停下等 review。
6. review 通過、使用者 commit（evidence ＋ 文件）之後，才規劃 ⑪（另行規劃與確認）。

## 十一、修訂紀錄

| 版本 | 日期 | 內容 |
|---|---|---|
| v1 | 2026-10-27 | 初版 |
| v2 | 2026-10-28 | v1 第一輪 review（暫不通過）的 5 項，全檔傳播（〇、「三」、「四」、「五」、「八」、D1、D4～D6、「十」2）：<br>1. **高：計次的分界**——`replay_started.json` 寫在呼叫 runner 之前，runner 還可能在 Docker 之前的檢查退出，照 v1 會誤扣唯一的崩潰重試。改成「準備啟動」與實際啟動的證據分開：沒有進到 Docker／已啟動／無法確認的三分，無法確認就停下由使用者裁決。<br>2. **中：137**——持鎖階段的子程序被 SIGKILL 時，supervisor 清理完才回 137。判讀改成先看 sentinel；137 而 sentinel ⛔ 不存在⛔ 不需要重開機。<br>3. **中：「五」1 的 Git 狀態**——`--untracked-files=all` 會逐檔列出，v1 的「恰好一列」不會成立。改成逐行檢查前綴與行數，另查其他檔與 staging。<br>4. **中：「五」4 的 `attempt.json`**——`--resume` 成功之後仍保留。改成以 `state-check` 驗鏈結，分開驗首次成功與恢復成功，保留合法的失敗沿革。<br>5. **中：observer 的非必要性**——v1 的 ① 要求 observer 的 starttime 是正整數，observer 立刻結束會被報成啟動失敗並叫使用者交給 Claude（與 D1 衝突）。改成 ① 只驗主流程、observer 讀不到記成 `-`；失敗訊息改成用 ② 等待；② 加上程序清單與判斷行；「五」6、7 明定報告缺失與狀態目錄殘留的處理 |
| v3 | 2026-10-28 | v2 第二輪 review（暫不通過）的 2 項，加上使用者的兩個裁決：<br>1. **高：計次「已啟動」的證據**——v2 的「有 `==> offline replay：` ＋ rc ⛔ 不是 125／126／127」證明不了容器已啟動（runner 印完那一行、`exec docker run` 之前被訊號終止也會留下 137／143）。改成只認容器內程序的直接證據（observer 的 `replay_seen`、`replay.err` 裡只有容器才有的 Python 路徑；⑨-1 實測可用）；「沒有進到 Docker」另限 host ⛔ 沒有重開機（重開機後缺行⛔ 不能證明沒有啟動）；其他歸「無法確認」（「四」、「八」）。<br>2. **高：② 的「可以啟動 Claude」**——v2 在⛔ 沒有 rc marker、找不到程序時就放行，⛔ 沒有要求 sentinel 已清除、⛔ 沒有查容器，身分「不明」也會通過（review 已在 bash、zsh 的最小案例重現）。改成依序判斷：查詢失敗 → 等待；程序或帶 `i074.stage2.run` 的容器仍在 → 等待；有 rc marker → 可以啟動；⛔ 沒有 rc marker 而 sentinel 仍在或身分不明 → 人工判讀；① 看到 rc marker 也改成先執行 ②（「三」、「八」、「十」2 的測試項目）。<br>3. **使用者裁決：D2 改成平日 17:00 時段為首選**，前提是晚間兩個批次永久後移一小時（另立計畫）；P4 與 D2 改以保守估計 3 小時 52 分計算，precheck 新增「P4 晚間批次已後移 ok」（〇、「二」、「八」、D2、「十」4）。<br>4. **使用者裁決：採用 review 的收斂條件**（「十二」）。<br>5. 「九」改成逐項定奪的格式：每項列選項、理由與「裁決」欄（使用者要求，2026-10-28） |
| v3（確認） | 2026-10-28 | 第三輪 review 通過（⛔ 沒有新的阻擋項目；review 以 bash、zsh 驗證 ② 的 16 個狀態分支案例——身分不明、sentinel 殘留、容器仍在、查詢失敗等——結果都符合預期）。使用者定奪 D1、D3～D9 全部採 (a)；D2 補上「開跑盡量靠近 17:00」（review 的建議）；D9 的文件修改已完成（⑨-2 結果在 `issue.md`、`sr-zone-scoring.md`、`development-workflow.md`、`plans/i074-stage2-step92-s92.md` 的「⚠️ 待 review」改成「✅ review 通過並 commit `62e1773`」） |
| v3（第四輪 review） | 2026-10-28 | 「十」2 實作的 review（暫不通過）1 項：④ 的 P3 只比對 `expected_image_id`、漏驗 identity 檔的完整 SHA（P3 要求 ＝ `a02bf3bd…`），只改 `created_at` 的 identity 會被誤報成 ok（後面的 checkpoint 仍會拒絕）。修正：④ 補驗 SHA、⛔ 不符就在建控制目錄與呼叫入口之前停下；使用者裁決一併修「二」的主 precheck（SHA 由人眼比對改成自動判定、開跑條件改成「P3 三行」）；補測 R13（bash、zsh 都拒絕）與主 precheck 的正反兩次實跑（「十三」） |
| v3（執行時段） | 2026-10-28 | 第四輪的修正 review 通過（使用者確認），「十」2 ＝ commit `a9cb41d`；使用者裁決當天 17:00 執行，「後移完成並驗證」以 T-075 的 A5 為準、⛔ 不等觀察一個平日晚上（「十」4、D2）；review 的非阻擋文字建議：殘餘風險改寫成「對舊排程沒有緩衝、仍可能重疊」，並補上新排程下的緩衝（「十」4） |

## 十二、review 的收斂條件（v3，✅ 使用者裁決 2026-10-28）

之後的 review 只有下列三類**擋住確認**；其他發現記進下面的「已知限制」，發生時再由使用者裁決（本計畫所有⛔ 沒有對應列的狀態一律「停下回報」，所以不會因為沒寫到而自動往下走）：

1. **影響成功路徑**（precheck → ① → 等待 → rc 0 → 「五」→ 交給 ⑪）。
2. **會自動觸發不可逆、或會用掉額度的動作**：用掉正式趟或崩潰重試、刪除或搬移檔案、重開機、在工作負載仍在時放行 Claude。
3. **計畫與程式或規格不符**（以程式與 `docs/` 的現況規格為準）。

已知限制（不擋確認；照實記錄）：

* 目前沒有。

## 十三、實作結果（「十」2，2026-10-28，✅ review 通過（第四輪），commit `a9cb41d`）

| 項目 | 內容 |
|---|---|
| 「三」④ | 補上區塊 `i074_stage2_step10_resume_001`：`MODE`（resume／promote）與 `N`（① 的 NAME）兩個值；格式驗證 → work 必須存在 → 前置條件（P3 的 image、identity 與它的完整 SHA-256、P5、P7 的 sentinel、容器與程序）在子 shell 執行、輸出存進控制目錄的 `precheck.txt` → 控制目錄 `/dev/shm/s10-<NAME>-<mode>-<UTC>/` → 與 ① 同一套 wrapper（⛔ 沒有 observer）→ 5 個身分欄位的驗證與 PID ＋ starttime 的確認。⚠️ **與計畫的差異**：P7 另查 label 容器（計畫原寫「sentinel 與程序兩項」）——supervisor 本來就拒絕殘留的容器，提早在 precheck 擋下，較嚴；「四」末段已同步。「五」7 補上 ④ 控制目錄的保存。第四輪 review：④ 原本只比對 `expected_image_id`，漏驗 P3 要求的 identity SHA（`a02bf3bd…`）→ 補上；「二」的主 precheck 原本只印出 SHA、靠人眼比對 → 一併改成自動判定「P3 identity SHA ok」，開跑條件改成「P3 三行」 |
| 測試環境 | scratchpad 的假 repo：假入口（記下 argv 與 `REPLAY_IMAGE_ID`，`exec` 假 supervisor——PID 不變、cmdline 變成 supervisor）、假 supervisor（正常、立刻失敗、TERM → 143）、假 observer（正常、立刻結束、忽略 TERM）、假 `docker`（`image inspect`、`ps` 的容器清單與失敗）、假 `meminfo`、假 identity。從本檔抽出區塊、只替換路徑（`/dev/shm`、`/run/lock`、`/proc/meminfo`、repo 與 `HOME` 都換成 scratchpad）、observer 的等待 120 → 4 秒、NAME 的時間可固定、identity 的預期 SHA 換成假 identity 的 SHA；⛔ 沒有碰 `/run/lock`、⛔ 沒有建立容器、⛔ 沒有執行真正的 ⑩ 入口 |
| 結果 | **bash 39／39、zsh 39／39 通過**（第四輪 review 補 R13：image 相同、identity 只多 `created_at` → SHA 不同 → 拒絕、⛔ 沒有建控制目錄；R1 另驗「P3 identity SHA ok」）。① 正常（rc 0、observer_rc 0、報告寫出、入口的 argv 與 `REPLAY_IMAGE_ID`）、入口立刻失敗、observer 立刻結束（① 仍通過、observer_rc 2）、observer 忽略 TERM（rc marker 照寫、observer_rc「未結束」）、work 已存在、控制目錄已存在（既有內容⛔ 沒有被動）；③ 身分相符送 TERM（rc 143、observer 也停）與已結束（⛔ 不送）；② 的每個分支（查詢失敗、程序仍在、容器仍在、有 rc marker、rc 137 ＋ sentinel、⛔ 沒有 rc marker 而身分不明 ＋ sentinel——review 的最小案例——身分不明、已結束 ＋ sentinel、全部不在）；④ 的 resume、promote、入口立刻失敗，以及 MODE 錯、N 沒改、work 不存在、P5、sentinel、容器、程序、image、identity、identity 的 SHA 各自拒絕（⛔ 沒有建控制目錄）；結束時本測試的程序都已結束 |
| 觀察 | 入口立刻失敗時，① 的驗證可能早於 rc marker：依時序印「⑩ 已經結束（rc 1）」或「harness／wrapper 的身分無法確認」——三種都是「⛔ 不啟動 Claude、用 ②」，接著的 ② 判斷「已結束（rc 1）、可以啟動 Claude」（已測）。照現行設計，⛔ 不改 |
| 互動 shell | 靜態檢查四個區塊的 `!`：只有 `! `、`!=`、`$!` 與單引號內的 awk——bash、zsh 的歷史展開都⛔ 不會觸發 |
| 真的 precheck | zsh、唯讀、stdin 接 `/dev/null`（2026-10-28T03:21:57Z；第四輪 review 之後 03:38:16Z 再跑一次）：P1 三行、P2 五行、P3 三行（含「P3 identity SHA ok」，SHA `a02bf3bd…`）、P6、P7、P8、基準、「PRE 保存 ok」都成立；另以 `XDG_DATA_HOME` 指向 scratchpad 的 identity 複本（image 相同、只多一個欄位）再跑一次：「P3 identity ok」照常印出、SHA `dd3ba447…`、「P3 identity SHA ok」⛔ 沒有印出（判斷正確）；預期不成立的：P4（⛔ 沒有 JWT → 401；「P4 晚間批次已後移 ok」⛔ 沒有印出——後移還沒做，判斷正確）、P5（767 MiB，Claude 執行中）、D1（claude 的程序）。測試產生的 PRE 檔（三個）都已刪除 |
