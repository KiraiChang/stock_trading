# 晚間批次後移一小時 計畫 v1（2026-10-28，✅ 決定 D1～D5 已定案（2026-10-28）；A1～A5 已執行（2026-10-28 14:13，A5 全部 ok，「九」）、⚠️ 待觀察當晚 22:00／23:00）

需求（✅ 使用者裁決 2026-10-28）：為了讓 I-074 Stage 2 的 ⑩ 能在平日 17:00 開跑、18:00 之後⛔ 不需要人力介入
（[`i074-stage2-step10.md`](./i074-stage2-step10.md) 的 D2），把 live 的兩個晚間批次**永久**往後移一小時：
`chip_daily_sync` 21:00 → **22:00**、`sr_analysis_chip` 22:00 → **23:00**（平日）。之後每個平日 17:05～22:00 有近 5 小時沒有批次。
評估過、⛔ 不採用的做法：「⑩ 還在跑才自動延後」——live 的排程⛔ 沒有暫停或延後的 API，要做到得改 backend 程式並部署（高影響異動，
而且 ⑩ 之前 repo 只能 commit 文件）；「⑩ 當天手動改、隔天改回」——碰 live 兩次、容易忘記改回。
決定 D1～D5 已由使用者定案（2026-10-28）；由使用者在 D4 的時段執行，Claude ⛔ 不動 live、⛔ 不改程式。

## 〇、唯讀盤點（2026-10-28）

| 項目 | 現況 |
|---|---|
| 排程機制 | backend（Go）程序內的 cron（`internal/scheduler`），時間在**啟動時**讀設定（viper：`config.yaml` → 環境變數覆寫，鍵的 `.` 換成 `_`）。執行中⛔ 沒有暫停或改時間的 API；手動補跑的端點有 7 個（daily-close、stock-symbol-sync、sr-evaluation、corporate-action-sync、evaluation-universe-sync、sr-analysis、delisting-reconcile），⛔ 沒有 `chip_daily_sync`。cron ⛔ 不補跑錯過的觸發 |
| `chip_daily_sync` | 設定鍵 `chip.sync.cron`（環境變數 `CHIP_SYNC_CRON`）；程式預設與 image 內的 `config.yaml` 都是 `0 21 * * 1-5`。⚠️ **live 的 compose 與 repo 的 `docker-compose.yml` 的 backend environment 都⛔ 沒有這個鍵**——只設 export 帶不進容器，要先加鍵。對 watchlist 抓法人與融資融券（30 天 lookback，漏一天下次自動補）；實測約 4 分鐘（10-23、10-26） |
| `sr_analysis_chip` | 設定鍵 `sr_analysis.chip_cron`（環境變數 `SR_ANALYSIS_CHIP_CRON`，live 與 repo 的 compose 都有這個鍵）；live 目前 `0 22 * * 1-5`。這一輪要求當日籌碼（籌碼不是今天的標的跳過）→ **必須晚於 `chip_daily_sync`**；實測不到 0.1 秒 |
| `sr_evaluation` | 預設 `30 22 * * 1-5`（設計註解：晚於 chip sync、晚於 `sr_analysis_chip`）；live 是 disabled |
| `/scheduler/status` 的 stale 門檻 | 寫死在程式（⛔ 不隨 cron 調整）：`chip_daily_sync` 72 小時——後移一小時，週五 → 週一的間隔仍是 72 小時，與現在相同 |
| live 的部署 | compose 檔在 repo 外（`/opt/stacks/scripts/stock_trading/compose.yml`，使用者的帳號擁有，Claude 的帳號⛔ 不可寫），project directory 是另一份部署用的 clone；環境變數（含金鑰）由同目錄的 `deploy.sh` export。⚠️ **那份 `deploy.sh` 會 `git pull`、`down`、`up --build -d`——等於整套重新部署最新的程式**，本計畫⛔ 不得用它來套用。⚠️ 它⛔ 不是有效的 UTF-8（`file` 判成 extended-ASCII）：GNU grep 預設當成二進位檔、⛔ 不輸出任何行（「三」A4） |
| live backend 的 image | 容器的 image ＝ `stock_trading-backend` 這個 tag ＝ `sha256:ba964c44…`（2026-09-17 build、09-18 啟動）。`up --no-build` 會用 tag 指到的 image——兩者相同才⛔ 不會順便換程式（執行當下再驗） |
| 與 ⑩ 的關係 | I-074 ⑨-2 之後到 ⑩ 之前，真正 repo 只能 commit 文件（⑩ 計畫的 P1）→ repo 的 `docker-compose.yml`、`deploy.sh` 範本、`backend/config.yaml`、`config.go` ⑩ 之前⛔ 不能改；live 的檔案在 repo 外，不受影響 |

## 一、目標、範圍與⛔ 不做

| 項目 | 內容 |
|---|---|
| 目標 | live 的平日 `chip_daily_sync` 22:00、`sr_analysis_chip` 23:00，永久生效；⛔ 不重新部署程式 |
| 範圍 A（⑩ 之前） | 使用者修改 live 的 compose（加 `CHIP_SYNC_CRON` 鍵）與 live 的 `deploy.sh`（兩個新值，讓之後的部署保留）→ **只重建 backend 容器**（同一個 image）→ 驗證 → 觀察一個平日晚上；文件同步現況（只動 `docs/`） |
| 範圍 B（⑩ 的晉升結果 commit 之後） | repo 的 `docker-compose.yml` 加 `CHIP_SYNC_CRON` 鍵、`deploy.sh` 範本加 export 與新值、`backend/config.yaml` 與 `config.go` 的預設值與註解（D3）；另行實作與 review |
| ⛔ 不做 | ⛔ 不跑 live 的 `deploy.sh`；⛔ 不 build image；⛔ 不動 backend 以外的服務；⛔ 不改其他排程；Claude ⛔ 不讀 live `deploy.sh` 的內容（含金鑰）、⛔ 不碰 live 的檔案（帳號也⛔ 沒有權限）——由使用者執行，Claude 提供指令與判讀 |

## 二、資料流與順序

* 晚間的順序不變：17:00 `sr_analysis`（前一日籌碼）→ **22:00** `chip_daily_sync`（當日籌碼）→ **23:00** `sr_analysis_chip`（當日 K ＋ 當日籌碼）；兩者仍隔 1 小時（籌碼採集約 4 分鐘）。
* 下游：當日籌碼與含籌碼的 SR 分析晚一小時產生；隔天 06:30 之前都已完成 → 盤前與盤中⛔ 沒有影響。FinMind 的法人與融資券晚間才發布，晚一小時只會更穩。
* `sr_evaluation`（live disabled）：預設 22:30 會變成早於 `sr_analysis_chip`（23:00），與設計註解的順序不符 → D2。
* ⛔ 沒有資料表、API 或前端 contract 的變化；`/scheduler/status` 的欄位與 stale 判定不變。

## 三、執行程序（使用者在自己的終端機執行；D4 的時段）

```text id="evening_batch_shift_apply_001"
# A1 precheck（唯讀）：image 一致、記下現況（只印排程鍵與雜湊，⛔ 不印其他 env 的值）
IMG_C="$(docker inspect stock_trading-backend-1 --format '{{.Image}}')"; IMG_T="$(docker image inspect stock_trading-backend --format '{{.Id}}')"
echo "容器 $IMG_C"; echo "tag  $IMG_T"; [ -n "$IMG_C" ] && [ "$IMG_C" = "$IMG_T" ] && echo "A1 image 一致 ok"   # ⛔ 不一致就停：--no-build 會換程式
docker inspect stock_trading-backend-1 --format '{{range .Config.Env}}{{println .}}{{end}}' | grep -E '^[A-Z_]+_(CRON|ENABLED)=' | sort > /dev/shm/shift-sched-before.txt
docker inspect stock_trading-backend-1 --format '{{range .Config.Env}}{{println .}}{{end}}' | grep -vE '^(CHIP_SYNC_CRON|SR_ANALYSIS_CHIP_CRON)=' | sort | sha256sum > /dev/shm/shift-rest-before.sha
cat /dev/shm/shift-sched-before.txt /dev/shm/shift-rest-before.sha; date +'A1 %F %T（⛔ 不在任何排程前後 10 分鐘內重建）'

# A2、A3：使用者用編輯器修改（先備份；⛔ 不要把檔案內容貼進對話）
#   cd /opt/stacks/scripts/stock_trading && cp -p compose.yml compose.yml.bak-<日期> && cp -p deploy.sh deploy.sh.bak-<日期>
#   compose.yml：backend 的 environment 加一行（放在 SR_ANALYSIS_CHIP_CRON 旁邊）：
#       CHIP_SYNC_CRON: ${CHIP_SYNC_CRON:-0 21 * * 1-5}
#   deploy.sh：加 export CHIP_SYNC_CRON="0 22 * * 1-5"；SR_ANALYSIS_CHIP_CRON 改成 "0 23 * * 1-5"

# A4 只重建 backend（同一個 image；⛔ 不跑 deploy.sh）：在子 shell 只載入 deploy.sh 的 export 與 PROJECT_DIR
#   ⚠️ live 的 deploy.sh 不是有效的 UTF-8：一般的 grep 會當成二進位檔、⛔ 不輸出任何行（eval 拿到空的 → 金鑰全空）——一定要 LC_ALL=C grep -a；
#   載入之後先驗（只驗「有沒有值」與兩個排程值，⛔ 不印金鑰），不成立就⛔ 不呼叫 compose
cd /opt/stacks/scripts/stock_trading && ( set -a; eval "$(LC_ALL=C grep -aE '^(export [A-Z_]+|PROJECT_DIR)=' deploy.sh)"; \
  [ -n "$PROJECT_DIR" ] && [ -n "$AUTH_JWT_SECRET" ] && [ -n "$FINMIND_API_KEY" ] \
    && [ "$CHIP_SYNC_CRON" = "0 22 * * 1-5" ] && [ "$SR_ANALYSIS_CHIP_CRON" = "0 23 * * 1-5" ] \
    || { echo "A4 載入不完整：⛔ 不重建"; exit 1; }; \
  docker compose -f ./compose.yml --project-directory "$PROJECT_DIR" up -d --no-build --no-deps backend ); echo "A4 rc $?"

# A5 驗證
docker inspect stock_trading-backend-1 --format '{{.Image}} {{.State.Status}} {{.State.StartedAt}}'          # image ＝ A1、running、剛剛啟動
docker inspect stock_trading-backend-1 --format '{{range .Config.Env}}{{println .}}{{end}}' | grep -E '^[A-Z_]+_(CRON|ENABLED)=' | sort | diff /dev/shm/shift-sched-before.txt -
#   預期只有：＋ CHIP_SYNC_CRON=0 22 * * 1-5；SR_ANALYSIS_CHIP_CRON 0 22 → 0 23；其他排程鍵⛔ 沒有變化
docker inspect stock_trading-backend-1 --format '{{range .Config.Env}}{{println .}}{{end}}' | grep -vE '^(CHIP_SYNC_CRON|SR_ANALYSIS_CHIP_CRON)=' | sort | sha256sum \
  | diff /dev/shm/shift-rest-before.sha - && echo "A5 其餘 env（含金鑰）與重建前完全相同 ok"   # ⛔ 不同 → A6（例如金鑰變空、開關被 deploy.sh 的新值改掉）
docker logs --since 10m stock_trading-backend-1 2>&1 | grep -i 'cron register failed' || echo "A5 cron 註冊 ok"
BIP="$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}} {{end}}' stock_trading-backend-1 | cut -d' ' -f1)"
printf 'A5 API 回應（⛔ 沒帶 JWT，預期 401）：%s\n' "$(curl -s -o /dev/null -w '%{http_code}' "http://$BIP:8080/api/v1/scheduler/status")"
#   另外前端登入一次。⚠️ /scheduler/status 只在 job 從未執行時才顯示有沒有註冊（有紀錄就顯示上一輪的結果），
#   所以註冊以「cron register failed ⛔ 沒有出現」與當晚 22:00、23:00 的實際執行為準（實作時修正，2026-10-28）
#   全部 ok 之後才刪基準檔（⛔ 不 ok → A6，基準留著給回滾之後的比對）：
#   rm -f /dev/shm/shift-sched-before.txt /dev/shm/shift-rest-before.sha

# A6 回滾（A5 任一項不符）：還原兩個備份檔，同 A4 重建；CHIP_SYNC_CRON 鍵消失 → 回到 image 內的 21:00
#   2026-10-28 的 A2、A3 由使用者直接編輯、⛔ 沒有備份 → 改用 sed 改回原值，再同 A4 重建：
#   sed -i '/^      CHIP_SYNC_CRON: /d' compose.yml && LC_ALL=C sed -i -e '/^export CHIP_SYNC_CRON=/d' \
#     -e 's|^export SR_ANALYSIS_CHIP_CRON="0 23 \* \* 1-5"|export SR_ANALYSIS_CHIP_CRON="0 22 * * 1-5"|' deploy.sh
```

當晚與隔天：`/scheduler/status` 的 `chip_daily_sync` 在 22:00、`sr_analysis_chip` 在 23:00 開始而且是終態；隔天 08:50 `pre_market` 照常。

## 四、風險與回滾

| 風險 | 處置 |
|---|---|
| 用 live 的 `deploy.sh` 套用 → 整套停下、拉最新程式重新 build | ⛔ 不用；A4 只重建 backend |
| `up --no-build` 用到不同的 image（tag 被移動過）→ 順便換了程式 | A1 驗容器的 image ＝ tag 的 image；不同就停、另行判斷 |
| 環境變數沒載入 → 容器以預設值建立（金鑰變空、開關回到 false） | ⚠️ live 的 `deploy.sh` 不是有效的 UTF-8，GNU grep 預設把它當二進位檔、⛔ 不輸出行（2026-10-28 只計數驗證：預設讀法拿不到內容，`LC_ALL=C grep -a` 才有 18 行）→ A4 一律 `LC_ALL=C grep -a`，載入之後先驗金鑰有值、兩個排程值正確才呼叫 compose；A5 比對「兩個排程鍵以外的整份 env」的雜湊與重建前相同、前端登入；不符就 A6 |
| live 的 `deploy.sh` 的值與目前容器不同（之前改過但沒重新部署） | A5 的雜湊比對會抓到 → A6 回滾、另行判斷（⛔ 不順便套用其他變更） |
| 重建時剛好遇到排程時刻 → cron ⛔ 不補跑 | D4 的時段；任何排程前後 10 分鐘內⛔ 不重建 |
| backend 重建的幾秒內 API 無法使用 | 選⛔ 沒有人在用、⛔ 沒有批次的時段（D4） |
| 之後有人用 live 的 `deploy.sh` 重新部署 | A3 已把新值寫進 live 的 `deploy.sh`，會保留 |
| repo 與 live 不一致（repo 的預設仍是 21:00／22:00） | ⑩ 之前只能改文件：文件寫明「live 以 env 覆寫」；範圍 B 在 ⑩ 之後同步 |
| ⑩ 依賴這兩個值 | ⑩ 的 precheck 驗兩個鍵（「P4 晚間批次已後移 ok」），⛔ 沒有就⛔ 不用平日 17:00 時段 |

## 五、驗證策略

* A1、A5（上面的區塊）；當晚與隔天的實際執行時間。
* 範圍 B（⑩ 之後）：`backend/scripts/test.sh`；dev stack 上確認新預設值的註冊（`development-workflow.md`「在 dev stack 上驗排程類功能」）。

## 六、決定

| # | 決定 | 裁決 |
|---|---|---|
| D1 | 新時間 | ✅ 使用者裁決（2026-10-28）：`chip_daily_sync` 22:00、`sr_analysis_chip` 23:00（平日） |
| D2 | `sr_evaluation`（live disabled，預設 22:30） | ✅ 使用者裁決（2026-10-28，照建議）：範圍 B 把預設一併改成 23:30，維持「晚於 `sr_analysis_chip`」的設計順序；live ⛔ 不動（disabled） |
| D3 | repo 的預設值 | ✅ 使用者裁決（2026-10-28，照建議）：範圍 B 把 `config.yaml`、`config.go` 的預設也改成 22:00／23:00（dev 與 live 一致），repo 的 compose 加 `CHIP_SYNC_CRON` 鍵、`deploy.sh` 範本加 export |
| D4 | 執行時段 | ✅ 使用者裁決（2026-10-28，照建議）：平日 18:00～20:30 或週末白天（⛔ 不在任何排程前後 10 分鐘內）；完成之後至少觀察**一個平日晚上**（22:00、23:00 確實執行）才安排 ⑩ 的平日 17:00 時段 |
| D5 | 誰執行 | ✅ 使用者裁決（2026-10-28，照建議）：使用者（live 的檔案屬於使用者的帳號）；Claude 提供指令、事後判讀，⛔ 不讀 live `deploy.sh` 的內容 |

## 七、執行步驟

1. ✅ 本計畫 review → 決定 D1～D5 定案（2026-10-28，D1 使用者指定、D2～D5 照建議）。
2. ✅ 使用者執行 A1～A5（2026-10-28；A4 在 14:13，⚠️ 不在 D4 的時段，見「九」），A5 全部 ok、⛔ 不需要 A6。
3. 當晚與隔天確認實際執行時間 → Claude 同步文件（只動 `docs/`）：`development-workflow.md` 的 live 批次表、`architecture.md`、`chip-analysis-design.md`、`api-reference.md`、`sr-zone-scoring.md` 等提到 21:00／22:00 的地方，寫成「live 以 env 覆寫為 22:00／23:00；repo 預設待範圍 B」→ ⛔ 不 commit，停下等 review。
4. review 通過 → 使用者 commit（只動 `docs/`）→ ⑩ 才可以用平日 17:00 時段。
5. ⑩ 的晉升結果 commit 之後：範圍 B 另行實作、review、commit；完成後收斂 `todo.md` T-075 與本計畫。

## 八、歸檔

* `development-workflow.md`：live 批次表（作業規則）、「只重建 backend、⛔ 不跑 live `deploy.sh`」的程序。
* `chip-analysis-design.md`（籌碼採集的時間）、`architecture.md`（排程表）。
* 收斂：範圍 B 完成並 review 之後，刪除本計畫與 T-075（刪除之前本檔必須已經 commit 過）。

## 九、執行紀錄（2026-10-28，週三）

| 步驟 | 結果 |
|---|---|
| A1 | 11:48 執行：image 一致 ok（`ba964c44…`）；排程鍵 12 個（`SR_ANALYSIS_CHIP_CRON=0 22 * * 1-5`、⛔ 沒有 `CHIP_SYNC_CRON`）；其餘 env 的雜湊 `9aa96f53…`。⚠️ A4 之前⛔ 沒有重跑（基準檔的時間仍是 11:48）——A5 比對的是 11:48 的基準，差異只有預期的兩個鍵、其餘 env 雜湊相同、image 相同，已涵蓋 11:48 到重建之間的變化 |
| A2、A3 | 11:08～11:09 使用者直接編輯 live 的兩個檔、⛔ 沒有備份（回滾改用 A6 的 sed）；Claude 事後以大小、`bash -n`、compose 的 dry-run 驗證（dry-run 的 env 45 個鍵，與容器只差兩個排程鍵） |
| A4 | **14:13:08（+08:00）重建**（容器的 `StartedAt` `2026-10-28T06:13:08Z`）。⚠️ **偏離 D4**（平日 18:00～20:30）：使用者 11:55 詢問能否在盤中執行，Claude 建議 18:00～20:30，另列 14:05～14:50 為技術上可行、但⛔ 不是 D4 的時段（最後一次 `intraday` 13:55、下一個排程 15:00，距離 18 分鐘與 47 分鐘，符合「⛔ 不在任何排程前後 10 分鐘內」）；使用者在 14:13 執行。A4 的輸出⛔ 沒有貼上，以 A5 判定成功 |
| A5 | 使用者貼上的輸出（原樣）：<br>`1a2`<br>`> CHIP_SYNC_CRON=0 22 * * 1-5`<br>`9c10`<br>`< SR_ANALYSIS_CHIP_CRON=0 22 * * 1-5`<br>`---`<br>`> SR_ANALYSIS_CHIP_CRON=0 23 * * 1-5`<br>`A5 其餘 env（含金鑰）與重建前完全相同 ok`<br>`A5 cron 註冊 ok`<br>`A5 API 回應（⛔ 沒帶 JWT，預期 401）：401`<br>第一行（image、狀態、啟動時間）⛔ 沒有貼上，Claude 14:15 自行唯讀核對：image `ba964c44…`（＝ A1）、`running`、啟動 `06:13:08Z`、重啟次數 0；排程鍵 `CHIP_SYNC_CRON=0 22 * * 1-5`、`SR_ANALYSIS_CHIP_CRON=0 23 * * 1-5`、`SR_ANALYSIS_CRON=0 17 * * 1-5` |
| 判定 | ✅ A5 全部 ok、⛔ 不需要 A6；基準檔（`/dev/shm/shift-*`）由使用者刪除 |
| 當晚觀察 | ⚠️ 待確認：`chip_daily_sync` 22:00、`sr_analysis_chip` 23:00 的實際執行時間與結果（`docker logs` 只取 `chip daily sync job completed`、`sr analysis done` 兩種行，或 `/scheduler/status`）→ 「七」3 |
