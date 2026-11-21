# I-139 計畫書 v1.2：scheduler 依交易日曆判斷休市日（休市日不跑只在交易日有意義的排程）

> 狀態：v1.2，⚠️ 待使用者確認（v1、v1.1 的 review 已併入；第六節 D1～D6 依 review 改寫，仍待裁決）。對應：`docs/issue.md` I-139。
> 需求：2026-11-20 使用者要求寫計畫書（I-139 的「可能的解法」）。

## 一、目標與不做的範圍

**目標**

1. **休市日不跑只在交易日有意義的排程步驟**：盤中、盤前的回補與預熱、收盤的當日取數與掃描，在休市日記 `skipped`（原因 `market_closed`），⛔ 不再照跑、⛔ 不再記成 success。
   會回補或核對過去幾天資料的排程（評估標的池同步、日 K 缺漏偵測、籌碼同步、SR 驗證）與盤前的清理照原排程（D1）。
2. **判斷依據用既有的 TWSE 年度日曆**（`market.TradingCalendar`，`holidaySchedule` 逐列分類，日 K 缺漏偵測已經在用），⛔ 不自己維護假日清單。
3. **排程與 API 共用同一份判斷**（3.1）：日曆查詢有短逾時；取不到時照平日跑（fail-open），而且降級結果短暫共用，⛔ 不讓每個呼叫端各自重新等待、在相近時間得到不同判斷。
4. **臨時休市（颱風假）**：年度日曆⛔ 不含當天才宣布的臨時休市，提供手動設定（D3），儲存與取消都留下操作者、時間與原因。
5. **畫面與 API 說得出「今日休市」**：`GET /intraday/status` 的 `market_open` 改用同一個判斷，自選清單在休市日顯示「今日休市」，⛔ 不再把前一個交易日的資料說成「已更新」。
6. **I-123 的判讀先知道哪些日期預期開市**（D5）：判讀器依開窗前固定的預定交易日清單核對每一天，⛔ 不只看資料裡出現的日期。

**不做的範圍**

* **補行交易日（週六交易）**：週末在共用判斷中一律回報休市，手動「照常開市」只允許平日（D6）；要支援得讀日曆的「補行交易」列並放寬 cron，列為已知限制。
* 不屬於「當日行情」的排程（股票主檔、公司行動、終止上市對帳、SR 評估）與 D1 列為照原排程的項目照舊每天跑，見 3.2 的表。
* 手動觸發的端點與重跑（`daily-close/run`、`intraday-refill/run` 等）⛔ 不加閘門：閘門只放在 cron 入口。
* 排程時間本身（`todo.md` T-076）⛔ 不在本案。
* 「開盤後所有標的都沒有新 K 棒」這種資料旁證⛔ 不用來自動停排程（D4 只提示）。

## 二、現況（2026-11-20～21 讀碼）

* cron 只看星期幾：`pre_market` `50 8 * * 1-5`、`intraday` `*/5 9-13 * * 1-5`、`daily_close` `0 15 * * 1-5`；其他每日排程的 cron 由設定決定，同樣不看休市日。
* `timeutil.IsMarketOpen` 只看週末與 09:00～13:30。`intraday` 的 cron 觸發 09:00～13:55，入口用它擋掉 13:30 之後（⛔ 不建立 `job_runs`），所以一天 55 輪；`GET /intraday/status` 的 `market_open` 也用它。
* **`pre_market`** 開頭先清理 `job_runs` 與逐輪存證（保留期），再建立 `job_runs`、回補 5 天日 K、預熱日線指標。
* **`RunDailyClose`** 供 cron 與手動端點共用；收尾接著跑 `sr_zone_verify`（獨立的 `job_runs`，驗的是最近 N 天的分析）。
* **`evaluation_universe_sync`** 回補最近 N 天（預設 10）日 K，接著跑 `candle_gap_detection`（用年度日曆判定預期交易日）；**`chip_daily_sync`** 的法人與融資融券以區間 API 抓、帶回看緩衝。這幾個都會回補或核對過去幾天的資料。
* SR 分析有「今天的 K 棒已入庫」的前置，休市日會自己跳過。
* **日曆**：`market.ExchangeReference.TradingCalendar(ctx, year)`（TWSE `holidaySchedule`、驗年份、逐列分類）；有 TTL 快取，**只快取成功結果**，失敗⛔ 不寫快取——日曆故障時每個呼叫端都會重新請求、重新等待。
  `exchangeReference` 由 `NewCandleGapDependencies` 建立，main 裡無條件注入 scheduler（`SetCandleGapDetection`），跟缺漏偵測共用同一個 client、節流與 breaker。
* **排程狀態頁**：`/scheduler/status` 取每個 job 最新一筆，`intraday` 的 stale 門檻 10 分鐘、⛔ 不看是否盤中。
* **T-088 的存證**：migration 079（`intraday_rounds.outcome` 的值域 `executed`／`skipped_lock_timeout`／`failed`）預計 2026-11-21 上 live。
* **觀察工具的新版判讀**（`v2_judge.py`，repo 外）：只逐一檢查資料裡出現的日期。用合成資料實測（v1 review，2026-11-21）：盤中紀錄全空、或只剩一天 55 輪，**都回傳「資料足夠」**。
  整天沒有紀錄的日子⛔ 不會被抓到——這跟 I-139 有沒有實作無關，11-23 起的新版窗口就會受影響（見 D5 與第九節）。

## 三、設計

### 3.1 共用的交易日判斷（`tradingday.Judge`）

* 新的小元件（暫定 `backend/internal/tradingday`），scheduler 與 API handler **共用同一個實例**（main 建立、兩邊注入）。介面：
  `Today(ctx) Result`／`On(ctx, day) Result`，`Result{IsTrading bool, Source string, Override *Override}`。`Source` 是封閉值域：
  * `weekend`：週六、週日，**一律休市**，⛔ 不查日曆也⛔ 不看手動設定（D6；跟現有 cron 一致，日曆取不到時也一樣）。
  * `manual`：當天有生效中的手動設定（3.4），以手動為準（「休市」或「照常開市」）。
  * `calendar`：日曆取得成功，依 `IsTradingDay` 判斷。
  * `fallback`：日曆取不到（逾時、請求失敗、breaker 開啟、年份驗證失敗），**一律當成交易日**（fail-open）。
* **判斷順序**：週末 → 手動設定 → 日曆 → fallback。
* **日曆查詢**：
  * 每次真的發請求時帶 **5 秒逾時**（`context.WithTimeout`），⛔ 不沿用呼叫端的 context 等到底。
  * 同一時間只有一個請求在飛（singleflight），其他呼叫端等它的結果，⛔ 不並行重打。
  * **共用的請求跟發起它的呼叫端脫鉤**：請求用自己的 context（`context.WithoutCancel` 再加 5 秒逾時），⛔ 不用發起者的 context。
    呼叫端只用自己的 context 等結果：某個呼叫端（例如畫面的 API 請求）先被取消，只有它自己提早回 `fallback`，共用的請求照樣跑完，其他呼叫端（例如排程）照樣拿到結果。
    **呼叫端取消⛔ 不算日曆失敗**，⛔ 不寫入下面的 5 分鐘降級結果；只有共用請求本身失敗或逾時才寫。
  * **失敗結果共用 5 分鐘**：取不到時記下 `fallback` 與時間，5 分鐘內所有呼叫端直接用這個降級結果（⛔ 不再請求、⛔ 不再等待），過了才重試。
    成功結果沿用 `exchangeReference` 的 TTL 快取。
  * `fallback` 第一次出現時記一次 Warn `trading calendar unavailable`（帶原因碼），恢復時記 Info；⛔ 不是每次判斷都記。
* **手動設定每次判斷都重讀**（一筆索引查詢），⛔ 不快取，所以設定與取消在**下一次判斷**生效（排程的下一輪、畫面的下一次輪詢）。
  讀不到手動設定（DB 錯誤）時照日曆判斷並記 Warn——DB 讀不到時排程本身大多也跑不了，⛔ 不另外設計。
* 跨年時取當年度的日曆。

### 3.2 哪些排程要看日曆（D1）

閘門**只放在 cron 入口**（`AddFunc` 註冊的那一層），手動端點與重跑走原本的函式，⛔ 不經過閘門。

| 排程 | 休市日 | 理由 |
|---|---|---|
| `intraday`（每 5 分鐘） | 09:00～13:30 的**每一輪**記 `skipped`（D2），⛔ 不抓取、⛔ 不評估 | 沒有盤中行情 |
| `pre_market`（08:50） | **清理照跑**（`job_runs` 與逐輪存證的保留期）；回補 5 天日 K 與預熱⛔ 不跑，`pre_market` 記 `skipped` | 清理跟當天有沒有交易無關；回補與預熱是為了當天的盤中 |
| `daily_close`（15:00） | 當日取數與掃描⛔ 不跑，`daily_close` 記 `skipped`；**尾端的 `sr_zone_verify` 照跑** | 當天沒有日 K；SR 驗證核對的是最近 N 天的分析 |
| `evaluation_universe_sync`（含尾端的 `candle_gap_detection`） | **照原排程** | 回補最近 N 天的日 K；缺漏偵測本來就用日曆 |
| `chip_daily_sync` | **照原排程** | 法人與融資融券是區間抓、帶回看緩衝，休市日跑一次也會回補前幾天 |
| `sr_analysis`、`sr_analysis_chip` | 照舊（已有「今天的 K 棒已入庫」前置，自己跳過） | ⛔ 不改 |
| `stock_symbol_sync`、`corporate_action_sync`、`delisting_reconcile`、`sr_evaluation` | **照跑** | 跟當天有沒有交易無關；公司行動是依星期分片，跳過會讓那一片當週沒有覆蓋 |
| 手動端點與重跑 | **照跑** | 手動代表人決定要跑；手動補抓在休市日會回「沒有當日 K 棒」，本來就正確 |

* 記錄的內容：`job_runs` `status=skipped`、`symbols_total=0`、`error` 是 `market_closed (calendar)` 或 `market_closed (manual)`，⛔ 不含其他文字（`weekend` ⛔ 不會出現：週末本來就不在 cron 裡）。
* `fallback` 時照跑，`job_runs` 照常。
* `daily_close` 是 `RunDailyClose` 與 `sr_zone_verify` 兩段：cron 入口在休市日只記 `daily_close` 的 `skipped`，再直接呼叫 `sr_zone_verify`；手動端點照舊呼叫完整的 `RunDailyClose`。

### 3.3 盤中一輪的記錄方式（D2）

* 休市日 09:00～13:30 的 **55 個時點各記一筆** `skipped`＋`market_closed (…)`，**只寫 `job_runs`**：⛔ 不拿鎖、⛔ 不寫 `intraday_rounds`／`intraday_symbol_runs`（079 的值域不必擴充）。
  有紀錄才分得出「排程有觸發、確認休市」和「排程漏跑」。
* 閘門放在入口現有的兩個檢查之後：先 `IsMarketOpen`（13:30 之後⛔ 不建立紀錄，13:35 起照舊沒有紀錄）、再「沒有盤中資料源」（照舊⛔ 不建立紀錄），然後才判斷休市。
* `/scheduler/status` 的 `intraday` 在盤中時段持續是最新的 `skipped`，⛔ 不會在 09:10 就亮 stale。量很小（一年約 15 個平日休市日 × 55 筆，保留 30 天）。
* `docs/issue.md` I-139「可能的解法」原本寫「一天只記一筆」，已同步改成每輪一筆。

### 3.4 臨時休市的手動設定（D3）

* **新表 `market_day_overrides`**（migration 080，三份 DB），**只新增、⛔ 不改寫**，每次儲存與取消各一列：
  `id`（主鍵）、`trade_date`、`action`（封閉值域：`closed` 手動休市／`open` 手動照常開市／`cleared` 取消）、`reason`（必填文字，例如「證交所公告 11-22 颱風全日休市」，上限 200 字）、`user_id`、`created_at`。
  索引 `(trade_date, id)`。某一天目前的狀態＝該日最新一列；最新一列是 `cleared` 或沒有任何列 → 依日曆。
* **API**（權限沿用現有的手動端點：任何登入的使用者；跟 T-088 的 D6 同一個「多人使用之前要調整」）：
  * `PUT /api/v1/market-day-overrides/{date}`：body `{"action": "closed"|"open", "reason": "…"}`。
  * `DELETE /api/v1/market-day-overrides/{date}`：body `{"reason": "…"}`，寫一列 `cleared`。
  * `GET /api/v1/market-day-overrides?from=…`：每天目前的狀態，以及完整的操作紀錄（誰、何時、做什麼、原因）。
  * 只接受**今天或未來的平日**（過去的日期：`job_runs` 已經發生；週末：D6），其他回 422；`reason` 空白回 422。
  * 生效時間：下一次判斷（3.1）。⛔ 不中斷正在跑的那一輪；盤中才設定的話，之前已執行的輪次照實保留。
* **前端（排程頁）**：顯示今天的判斷與來源，明列三種狀態——**依日曆**（含日曆說的開市／休市，或「日曆取不到，照平日執行」）、**手動休市**、**手動照常開市**；
  可以設定或取消，都要填原因；下方列出操作紀錄。
* **文案要讓使用者確認的是「市場全日休市」**：「依交易所公告設定。停班範圍（全日、上午、下午）對交易的處理不同，以公告為準；本設定代表**全日休市**，只有部分時段停班而照常交易時⛔ 不要設定。」
  操作依據是交易所公告（證交所的天然災害休市規則：https://www.twse.com.tw/zh/clearing/suspended.html）。

### 3.5 畫面與 API

* `GET /intraday/status` 加 `trading_day`（`{is_trading, source}`，`source` 同 3.1），`market_open` 改成「交易日且 09:00～13:30」。
* 自選清單的盤中狀態：`is_trading=false` 時每一列顯示「今日休市（日曆）」或「今日休市（手動設定）」，⛔ 不顯示上一輪的「已更新」；
  `fallback` 時照常顯示，另在清單標題列提示「交易日曆取不到，照平日執行」。
* 前端輪詢（T-088 的 `intradayPolling`）依 `market_open`，休市日自然用非盤中的 5 分鐘間隔。
* `/scheduler/status` ⛔ 不改計算方式；休市日 `skipped` 的 `error` 帶 `market_closed (…)`，排程頁照原樣顯示。

### 3.6 （D4）沒有盤中資料的提示

* ⛔ 不自動停排程，只在 `GET /intraday/status` 加 `session_data`，封閉值域。各狀態分開，⛔ 不讓同一個提示同時代表資料源故障與市場休市：
  * `ok`：今天已有 1 分 K。
  * `pending`：交易日 09:30 之前、還沒有今天的 1 分 K（⛔ 不提示）。
  * `none_yet`：交易日 09:30 之後、所有自選標的今天都沒有 1 分 K，**而且最近一輪沒有已知的抓取失敗** → 提示「截至 HH:MM 尚未收到今日盤中資料，請核對行情來源與交易所公告」（HH:MM 是現在時間）。
  * `fetch_failing`：同上但最近一輪有 `fetch_failed` 或輪次 `failed` → 沿用 T-088 的抓取失敗顯示，⛔ 不出「尚未收到」的提示。
  * `store_failing`：同上但最近一輪已有回應、K 棒全部寫入失敗（`store_failed`）→ 沿用 T-088 的寫入失敗提示，⛔ 不出「尚未收到」的提示。
  * `empty_watchlist`：自選清單是空的 →「自選清單是空的」。
  * `unavailable`：查詢 DB 失敗 →「讀不到盤中資料狀態」。
  * `closed`：今天判斷為休市 → 只顯示「今日休市」。
* 由人看了提示、核對公告後決定要不要設定手動休市。

## 四、受影響的檔案、模組與資料流

| 位置 | 改動 |
|---|---|
| `backend/internal/tradingday/`（新） | 共用判斷：週末、手動、日曆（逾時、單一請求、失敗共用 5 分鐘）、fallback |
| `backend/internal/scheduler/` | cron 入口的閘門（3.2）、`pre_market` 清理與回補分開、`daily_close` 休市日只跑 `sr_zone_verify`、`skipped` 紀錄 |
| `backend/internal/store/` | `MarketDayOverrideRepo`（新增一列、讀某日最新一列、列操作紀錄） |
| migration 080（postgres、sqlite、mysql） | `market_day_overrides` |
| `backend/internal/api/` | `market-day-overrides` 三支端點；`GET /intraday/status` 的 `trading_day`、`market_open`、`session_data` |
| `backend/cmd/server/main.go` | 建立判斷元件、注入 scheduler 與 handler |
| `backend/pkg/timeutil` | `IsMarketOpen` 保留（純時間判斷） |
| `frontend/` | 盤中狀態的「今日休市」與 `session_data` 提示、排程頁的手動設定與操作紀錄 |
| repo 外的觀察工具 | 預定交易日清單、缺天檢查、休市日排除（D5；其中缺天檢查是先行項目，見第九節） |
| 文件 | 見第八節 |

## 五、主要風險與回滾

| 風險 | 處理 |
|---|---|
| 日曆判錯，把交易日當成休市日 → 整天沒有盤中訊號 | 日曆逐列分類、驗年份（缺漏偵測已在用）；手動「照常開市」可以推翻；取不到一律 fail-open |
| 日曆端點故障或很慢，排程與畫面重複等待 | 5 秒逾時、單一請求、失敗共用 5 分鐘 |
| 相近時間的呼叫端得到不同判斷 | 排程與 API 共用同一個實例與快取 |
| 跟缺漏偵測共用 client、節流與 breaker | 有 TTL 快取，一年只會真的打幾次；breaker 開啟時 fail-open |
| 手動設定打錯日期或誤設 | 只接受今天或未來的平日；可以取消；每次操作都留紀錄（誰、何時、原因）；畫面顯示目前的判斷與來源 |
| 盤中才設定手動休市，當天紀錄一半執行、一半 `skipped` | 照實保留；I-123 判讀依公告核對（D5） |
| 休市日 55 筆 `skipped` 讓 `job_runs` 變多 | 一年約 800 筆，保留 30 天 |
| 回滾 | 退回舊 image（排程照舊每天跑）；`market_day_overrides` 是新表，⛔ 不用退 migration |

## 六、待裁決（v1.1 依 review 改寫，建議附在後面）

* **D1 排程範圍**：建議照 3.2——盤中、盤前的回補與預熱、收盤的當日取數與掃描加閘門；盤前清理、`evaluation_universe_sync`、`candle_gap_detection`、`chip_daily_sync`、`sr_zone_verify` 照原排程；
  閘門只放在 cron 入口，手動端點與重跑照跑。（v1 原本連評估標的池、籌碼與 SR 驗證都擋；review 指出它們會回補或核對過去幾天，改為照跑。）
* **D2 盤中休市日的記錄**：建議 09:00～13:30 每一輪記一筆 `skipped`（55 筆），只寫 `job_runs`；測試鎖住 13:30 最後一輪會記、13:35 不再記；`issue.md` 的「一天一筆」同步改掉。
* **D3 臨時休市**：建議做手動設定，畫面明列依日曆／手動休市／手動照常開市三種狀態；儲存與取消都留操作者、時間與原因（只新增的紀錄表）；下一次判斷生效；依交易所公告，文案確認的是全日休市。
* **D4 沒有盤中資料的提示**：建議只提示、⛔ 不自動停排程；文案「截至 HH:MM 尚未收到今日盤中資料，請核對行情來源與交易所公告」；自選清單為空、DB 查詢失敗、已知抓取失敗各自有狀態（3.6）。
* **D5 觀察工具**：建議如下，並寫進 I-123 計畫書（規則要在看到新版窗口資料之前定下）：
  1. **開窗前固定預定交易日清單**（依 TWSE 年度日曆）與判讀規則；確認休市的日期另外記在休市清單，附交易所公告的出處。
     T-088 照計畫 11-21（週六）切版時，新版窗口是 **11-23（一）～11-27（五）**（2026 年日曆這五天都是交易日）；實際切版時間有變，就從切版後第一個預定交易日重算。
  2. 判讀器**逐一檢查清單上的每一天**：每個預定交易日要**恰好 55 輪**（保留現行的 `!= 55` 判定）——0 輪（整天沒有紀錄）、少於 55 輪、多於 55 輪（例如重複觸發）都是**資料不足**；資料裡出現、但⛔ 不在清單上的日期另外列出、⛔ 不計入。
  3. **確認休市的日期**另列，從盤中「5 個交易日」的分母扣除，窗口順延一個交易日。
  4. 預定交易日整天 55 輪都是 `market_closed`、但⛔ 沒有在休市清單上 → 資料不足，列出來核對（日曆或手動設定跟清單不一致）。
  5. **盤中才補設休市**：保留已執行與跳過的紀錄，按公告核對當日是否全日休市——是 → 改列休市日（同第 3 點）；否 → 當天資料不足。
  6. 主檔、對帳、Redis 等原本跨日觀察的項目照舊納入所有日子（含休市日與週末）。
  7. 至少補測：整天零筆、只剩一天 55 輪（現況會誤判的兩個案例）、某一天 56 輪（重複觸發⛔ 不能通過）、只有一筆 `market_closed`、確認休市且 55 輪、盤中才設定。
* **D6 週六補行交易**：建議列為已知限制；週末在共用判斷中一律回報休市（日曆取不到時也是），手動「照常開市」只允許平日。

## 七、測試與驗證

* **共用判斷**：
  * 週末 → `false/weekend`（日曆取不到、有手動設定時都一樣）。
  * 日曆說休市 → `false/calendar`；日曆說交易日 → `true/calendar`；「開始交易」「最後交易」這類列照常是交易日（沿用日曆既有的測試，補一個判斷層的案例）。
  * 手動休市推翻日曆的交易日、手動開市推翻日曆的休市日 → `manual`；最新一列是 `cleared` → 回到日曆。
  * 日曆逾時（假 client 卡住）→ 5 秒內回 `true/fallback`；5 分鐘內再判斷⛔ 不再請求、結果相同；過了 5 分鐘才重試；同時多個呼叫端只發一個請求。
  * 發起共用查詢的呼叫端（API 請求）先取消：它自己回 `fallback`；同時在等的其他呼叫端（排程）照樣拿到日曆的結果；⛔ 沒有寫入 5 分鐘的降級結果，下一次判斷用的是成功的快取。
  * 回錯誤年份 → `fallback`。Warn 只在進入 fallback 時記一次。
  * 設定或取消手動之後，下一次判斷就反映（⛔ 不等快取）。
* **排程閘門**：
  * 休市日 `intraday`：09:00 與 13:30 兩輪都記 `skipped`＋`market_closed (…)`、一天共 55 筆；13:35 ⛔ 不記；⛔ 不拿鎖、⛔ 不寫 `intraday_rounds`／`intraday_symbol_runs`、⛔ 不抓取、⛔ 不評估。
  * 休市日 `pre_market`：清理照跑（`DeleteBefore` 有被呼叫）、⛔ 不回補、⛔ 不預熱、記 `skipped`。
  * 休市日 `daily_close`：⛔ 不取數、⛔ 不掃描、記 `skipped`；`sr_zone_verify` 照跑。
  * `evaluation_universe_sync`、`candle_gap_detection`、`chip_daily_sync`、`stock_symbol_sync`、`corporate_action_sync`、`delisting_reconcile` 照跑。
  * 手動端點（`daily-close/run`、`intraday-refill/run`）在休市日照跑。
* **fail-open**：日曆取不到時所有排程照跑，`job_runs` 照常；交易日的行為與 T-088 相同（既有測試全部通過）。
* **手動設定**：只接受今天或未來的平日、`reason` 必填；儲存與取消各寫一列（含 `user_id`）；`GET` 列出目前狀態與操作紀錄；migration 080 三份 DB（`test-postgres-migrations.sh`、`test-mysql-migrations.sh`、SQLite）。
* **API 與畫面**：`trading_day`、`market_open`、`session_data` 各值的 JSON（含「已有回應、K 棒全部寫入失敗」→ `store_failing`、沿用 T-088 的寫入失敗提示）；自選清單的「今日休市（日曆／手動）」、日曆取不到的提示、`session_data` 的各種文案；排程頁的三種狀態、設定與取消、操作紀錄。
* **觀察工具**：D5 第 7 點的案例；`accept_v2.sh` 既有案例照舊通過。
* **dev 實跑**：用手動休市（或假的日曆）在 dev 走一個「休市日」：盤中與收盤記 `skipped`、`sr_zone_verify` 照跑、畫面顯示今日休市；取消手動設定後下一輪恢復。
* 完整回歸：`backend/scripts/test.sh`、`frontend/scripts/test.sh`、`python/scripts/test.sh`。

## 八、完成後要歸檔的位置

* `docs/architecture.md`：排程一覽補「休市日的行為」（哪些排程看日曆、共用判斷、fail-open、手動設定）。
* `docs/architecture/data-pipeline.md`：每日排程的前置條件、cron 入口的閘門。
* `docs/yahoo-intraday-integration.md`：盤中狀態在休市日的顯示、`session_data`。
* `docs/database-schema.md`：`market_day_overrides`；`job_runs` 的 `skipped`＋`market_closed`。
* `docs/api-reference.md`：`market-day-overrides` 三支端點、`/intraday/status` 的新欄位。
* `docs/development-workflow.md`：颱風假的手動設定程序（依公告、全日休市才設定、填原因）；部署時段在休市日仍照平日（照跑的排程還在）。
* `docs/issue.md` I-139 改成「已實作／待 review」；review 確認後收斂。

## 九、上版與觀察

* 本案改的只有休市日的行為，交易日⛔ 不變，⛔ 不影響 I-123 新版窗口的交易日計數；新版窗口內若有休市日，判讀照 D5。
* **先行項目（跟 I-139 實作無關，建議本週處理）**：判讀器的缺天檢查（D5 第 1、2、7 點：預定交易日清單 11-23～11-27、每個預定交易日恰好 55 輪，含 0 輪與 56 輪的案例）。
  現況整天沒有紀錄的日子會被判成「資料足夠」，11-23～11-27 的新版窗口就會用到；要在看到新版窗口的資料之前改好並驗收。
  休市日相關的第 3～5 點要等 I-139 上版後才會出現 `market_closed`，可以跟本案一起做。
* 上版時段照部署規範；休市日照樣只能用平日的時段（照跑的排程還在）。

## 版本紀錄

| 版本 | 日期 | 內容 |
|---|---|---|
| v1 | 2026-11-20 | 初版。 |
| v1.1 | 2026-11-21 | 併入 v1 review：D1 改成只擋盤中、盤前回補與預熱、收盤取數與掃描，盤前清理、評估標的池、缺漏偵測、籌碼、SR 驗證照原排程，閘門只放 cron 入口；D2 鎖住 13:30／13:35 的邊界並同步改 issue.md；D3 改成只新增的操作紀錄表 `market_day_overrides`（三種狀態、操作者、原因），文案確認全日休市；D4 拆出 `session_data` 各狀態；D5 改成依預定交易日清單逐日核對（現況判讀器漏檢缺天），缺天檢查列為先行項目；D6 週末在共用判斷一律休市、手動開市只限平日；新增 3.1 的共用判斷（5 秒逾時、單一請求、失敗共用 5 分鐘、手動設定每次重讀）。 |
| v1.2 | 2026-11-21 | 併入 v1.1 review：[P2] 新版窗口改成 11-23（一）～11-27（五）（v1.1 少算 11-23、多算週六 11-28；切版時間有變就從切版後第一個預定交易日重算），issue.md 同步；[P2] D5 保留每個預定交易日恰好 55 輪（`!= 55`），補 56 輪的案例；實作驗收補兩個案例：已有回應但 K 棒全部寫入失敗 → `store_failing` 沿用 T-088 的寫入失敗提示，發起共用日曆查詢的請求先取消時其他呼叫端照樣拿到結果、⛔ 不觸發 5 分鐘降級（3.1 改成共用請求跟發起者的 context 脫鉤）。 |
