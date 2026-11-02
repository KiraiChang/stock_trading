# Data Pipeline

Data Pipeline 負責取得、清洗、時間對齊與持久化資料。它是所有分析、AI 與決策的上游，
但本身不做交易判斷、不訓練模型、不輸出買賣建議。

## 職責

- 連接外部資料源並處理 API 限制、錯誤、fallback 與重試策略。
- 將資料正規化後寫入 DB。
- 保留同步任務狀態，讓資料缺口可以追蹤。
- 對齊交易日期、K 棒時間、籌碼資料日期與部位資料時間。
- 提供下游穩定讀取契約。

## 現有模組歸位

| 類別 | 現有位置 | 說明 |
|------|----------|------|
| 市場資料來源 | `backend/internal/market` | FinMind、Fugle、Yahoo client 與資料源抽象 |
| 股票主檔同步 | `market.TWSEISINClient` / `market.StockSymbolSyncer` | 從 TWSE ISIN 清單同步有價證券主檔與 `is_listed` 狀態 |
| K 棒同步 | `market.Fetcher` | 歷史日 K、盤中資料、回補流程 |
| 排程資料工作 | `backend/internal/scheduler` | pre-market、intraday、daily-close、stock-symbol-sync 中的資料更新階段 |
| K 棒資料表 | `candles` | 下游指標、訊號、SR Zone、驗證使用 |
| 股票主檔資料表 | `stock_symbols` | symbol、名稱、ISIN、分類、產業與是否仍上市 |
| 籌碼 raw data | `institutional_trades` / `margin_trades` / `broker_trades` | 三大法人、融資融券、券商分點原始資料 |
| 籌碼同步任務 | `chip_sync_jobs` | 手動/回補籌碼任務進度 |
| 排程任務紀錄 | `job_runs` | daily/intraday 類任務狀態 |
| 監控清單 | `watchlist` | 決定排程與 UI 預設處理範圍 |
| 交易流水 | position transactions | immutable ledger，提供 Decision Pipeline 使用 |
| 部位 projection | `positions` | 由交易流水投影出的目前持股狀態 |

## 輸入

- FinMind 歷史日 K、分 K、三大法人、融資融券、券商分點。
- Fugle REST / WebSocket 行情。
- Yahoo 盤中行情。
- TWSE ISIN 上市（strMode=2）＋上櫃（strMode=4）有價證券清單，抓取行為見
  「[TWSE ISIN 同步策略](#twse-isin-同步策略)」。
- 使用者維護的 watchlist。
- 使用者新增的 BUY / SELL / ADJUSTMENT transaction。

## 輸出契約

- `candles`
- `stock_symbols`
- `institutional_trades`
- `margin_trades`
- `broker_trades`
- `watchlist`
- `positions`
- `position_transactions`
- `job_runs`
- `chip_sync_jobs`

## TWSE ISIN 同步策略

`stock_symbol_sync`（預設每日 06:30，`stock_symbols.cron`）依序抓上市（strMode=2）與
上櫃（strMode=4）兩個來源，合併去重後寫入 `stock_symbols`。

| 行為 | 現況 | 可調參數 |
|------|------|----------|
| 單次請求上限（含讀 body） | 300 秒 | `stock_symbols.timeout_sec` / `STOCK_SYMBOLS_TIMEOUT_SEC` |
| 來源之間的間隔 | 3 秒 | `stock_symbols.fetch_delay_sec` / `STOCK_SYMBOLS_FETCH_DELAY_SEC` |
| 整個 job 的上限 | 20 分鐘（`scheduler.stockSymbolSyncTimeout`） | 程式常數 |
| 失敗處理 | all-or-nothing；暫時性失敗時整輪自動重跑（預設關閉，見「失敗後的自動重跑」） | `job_retry.*` / `JOB_RETRY_*` |

### 來源實測值（2026-07-22）

直接對 `strMode=2`（上市）發一次請求量到：**HTTP 200、7.5 MB、耗時 251 秒**。這解釋了
原本的失敗——30 秒與後來調到的 90 秒都遠低於實際下載時間，才會出現
`context deadline exceeded (... while reading body)`：連線與 header 都正常，是 body
讀到一半被 client timeout 中斷。timeout 因此設為 300 秒（量測值加約兩成餘裕），
job 上限同步放大到 20 分鐘（2 來源 × 300 秒 + 間隔 + 寫入）。

判斷是否需要再調整時，看 log 的 `elapsed` 欄位，不要憑感覺加秒數。

兩個秒數參數留 `0` 或不設定時，沿用 `internal/market/twse_isin.go` 的預設常數；預設值只
定義在程式裡一處，config 不重複寫死數字。request 會帶可辨識來源的 `User-Agent`
（`stock-trading/1.0 (personal market data sync)`）與 `Accept` / `Accept-Language`，
**刻意不偽裝成瀏覽器**——TWSE ISIN 是公開清單，抓取本身正當，站方要限流或聯絡時也才有依據。

### 失敗之後

抓取失敗時整批放棄本次同步（all-or-nothing）。**請求層⛔ 不重試**；暫時性失敗由排程**整輪重跑**
（`job_retry.enabled` 開啟時，規則見下面的「失敗後的自動重跑」）——兩個來源整批重抓，⛔ 不會只拿到半個市場。
開關關閉、或失敗不是暫時性時，由人工觸發：前端的手動同步按鈕（`POST
/api/v1/scheduler/stock-symbol-sync/run`），與 cron 走同一段邏輯、在背景執行；已有一輪在跑時回 409。

診斷依據：每個來源成功或失敗都會記 `elapsed`（`twse isin source fetched` /
`twse isin source fetch failed`），要判斷 timeout 是否需要調整時看這個欄位；錯誤本身由
scheduler 記一次（`stock symbol sync failed`），不重複記錄。

## 失敗後的自動重跑（`job_retry`）

2026-11-02 起（I-123～I-126；計畫書 [`plans/i123-i126-job-retry.md`](../plans/i123-i126-job-retry.md)）。
**預設關閉**：`job_retry.enabled`／`JOB_RETRY_ENABLED`。I-122 在 live 看到的失敗——DNS 查詢逾時、IPv6 不可達、
postgres 短暫進入 recovery mode——都是網路波動或第三方維修造成的暫時性失敗，⛔ 不追查根因，改由重跑吸收。

### 什麼算暫時性

`market.IsTransient` **只認型別、⛔ 不比對訊息文字**：

| 算暫時性 | 依據 |
|---|---|
| DNS 查詢逾時或暫時性失敗 | `*net.DNSError` 的 `IsTimeout`／`IsTemporary` |
| 連線層錯誤（拒絕、重設、網路不可達、讀寫逾時） | `*net.OpError` |
| HTTP client 逾時 | `url.Error` 的 `Timeout()` |
| 連線中途斷掉 | `io.ErrUnexpectedEOF` |
| 上游 5xx（含維修常見的 503） | `market.HTTPStatusError` 的 `Code >= 500`（TWSE ISIN、TWSE 終止上市名單、Yahoo 報價、Yahoo 配息、FinMind 的非 200 分支） |
| DB 連線類錯誤 | 錯誤鏈上的 `SQLState()` 屬於 class `08`、`57P01`～`57P03`、`53300` |

⛔ **不算**：4xx（含 404，以及 429——限流時縮短間隔只會更糟）、FinMind 的 `api_status`、解析錯誤、主檔縮水防護、
空快照、前置條件不成立、DNS 的 NXDOMAIN，以及**這一輪自己的預算用完**——http client 被 ctx 取消時回的
`url.Error` 也帶 `Timeout()`，所以呼叫端一律先看自己的 `ctx.Err()`。FinMind client 既有的請求層重試
（網路錯誤、5xx、429、402 退避 3 次）⛔ 不變。

### 層一：每日工作整輪重跑

適用 `stock_symbol_sync`、`corporate_action_sync`（**只限整輪早退**：分割批次、列標的失敗）、`delisting_reconcile`。

* **觸發**：一輪以 `failed` 結束、原因是暫時性、而且是 cron 或自動重跑發起的。**手動觸發的失敗⛔ 不排**。
* **時間**：失敗結束後隔 `interval_min`（預設 10 分鐘）重跑，每個工作每天最多 `max_reruns`（預設 3）次。
* **整輪的截止時刻** `cutoff`（預設 08:45，台北）：重跑的**整輪**——寫開始紀錄、業務、寫結束紀錄——都要在它之前結束，
  ⛔ 不壓到 08:50 的 `pre_market`。**最晚起跑 ＝ 截止 − 業務預算 − 3 分鐘**（3 分鐘涵蓋 `startRun` 的 10 秒上限與結束寫入重試最多 133 秒）：
  預設下 `corporate_action_sync` 07:57（預算 45 分鐘，隨 `corporate_action.timeout_sec` 變）、`stock_symbol_sync` 08:22（20 分鐘）、
  `delisting_reconcile` 08:37（5 分鐘）。重跑時刻晚於它、或跨到隔天，就⛔ 不排。
* **同一天、同一個工作只有一條重跑鏈**（計數與至多一個待跑的重跑），換日重置。**一輪算哪一天看它的起跑時刻**：前一天起跑、今天才結束的一輪
  ⛔ 不動今天的狀態——成功⛔ 不讓今天的鏈失效、失敗⛔ 不排重跑（重跑時刻跨到起跑的隔天，`error` 附「已停止」與起跑那天的最晚起跑）、
  主檔的 success ⛔ 不算數給今天的連帶重跑（對帳的前置同樣要求起跑於今天）。
* **任一入口成功，舊鏈就失效**：cron、手動、重跑、連帶觸發，只要當天這個工作有一輪⛔ 不是 `failed`（success，或整輪跑完的 partial），
  就取消待跑的重跑、當天⛔ 不再排。例：06:30 失敗、06:35 手動成功 → 06:40 的重跑⛔ 不執行。
* **計時器到期時**先確認還是同一天、還沒成功、還沒超過最晚起跑時刻，再取得執行所有權；取不到（手動那輪還在跑）→ 這次重跑取消、記 Warn，鏈結束。
* **每次重跑都是一輪完整的執行**：新的一筆 `job_runs`；TWSE 的兩個來源整批重抓，all-or-nothing 不變。
  排了重跑的那一筆，`error` 結尾附 `; 自動重跑 n/m（HH:MM）`、用完附 `; 自動重跑已用完（m/m）`、
  因最晚起跑時刻而⛔ 不排附 `; 自動重跑已停止（最晚起跑 HH:MM）`（語意見 [`api-reference.md`](../api-reference.md)「自動重跑：`error` 結尾的固定文字」）。
* ⚠️ 重跑鏈**只在記憶體裡**：backend 重啟後⛔ 不保留，`Stop()` 時全部取消。

**連帶重跑**：`delisting_reconcile` 的前置是「當日最新一筆 `stock_symbol_sync` 是 success」。對帳因前置不成立而失敗時
（cron 或自動重跑發起的）⛔ 不排自己的計時器，而是**等主檔成功落盤**之後連帶觸發一次，計入對帳自己的每日上限：

* 判斷依據是「**主檔最新一輪已成功落盤**」——⛔ 不是「主檔今天曾成功」：任何一輪主檔開始時設為假，結束紀錄以 success
  **寫入成功之後**才設為真。例：06:30 主檔成功 → 06:55 手動同步開始並失敗 → 07:00 對帳被擋 → ⛔ 不觸發。
* **一次主檔成功最多帶動一次對帳**（每次成功一個代號，對帳記下用掉的代號），⛔ 不會因為對帳又被擋下而反覆觸發、耗盡額度。
* 狀態都在同一把鎖下判斷：對帳登記等待時重新確認（擋住「對帳讀到主檔還在 running → 主檔成功並通知 → 對帳才登記等待」），
  觸發時對帳還持有所有權就留著「待觸發」、等它釋放所有權時再試一次。超過對帳的最晚起跑時刻或額度用完就⛔ 不觸發。

**執行所有權**：`stock_symbol_sync`、`corporate_action_sync`、`delisting_reconcile` 的 cron、手動、自動重跑（含連帶觸發）
**共用同一個執行所有權**——兩輪主檔同步並行時，較舊的快照可能較晚寫入。手動撞上回 409、cron 撞上只記 Warn
（⛔ 不寫 `job_runs`，避免蓋掉進行中那輪的結果）、重跑撞上取消。

**對原本「不自動重試」三個理由的回應**：① 漏同步一天的代價——I-122 看到它會連帶擋下終止上市對帳，比原先估的大；
② all-or-nothing 不變，重跑是整批重抓；③「連續打同一站台」——間隔 10 分鐘、一天最多 3 次、盤前截止，
而且只在暫時性錯誤時才跑；I-122 的三次失敗都在 DNS 階段，請求根本沒有送到 TWSE。

### 層二、層三

* **`intraday` 的批次**：同一輪內重試，見 [`yahoo-intraday-integration.md`](../yahoo-intraday-integration.md)「批次失敗的同一輪重試」。
* **`corporate_action_sync` 的失敗標的**：第一輪跑完之後，挑出**所有失敗階段都是暫時性**的標的，剩餘預算超過 60 秒時先等 60 秒、
  再對它們跑一次逐檔同步（仍在同一個整輪預算內）。第二輪成功的從失敗清單移除；第二輪**仍失敗**（含做到一半預算用完）的記**第二輪**的原因——它反映資料現在的狀態，例如事件已寫入、係數重算失敗；
  第二輪沒跑到（預算用完）的保留第一輪的原因，
  `symbols_failed` 與 `error` 依合併後的結果寫。404 之類的永久失敗⛔ 不重試；重算是冪等的，重試⛔ 不會累積誤差。

### 不受開關控制的部分

結束寫入的重試（退避 1／2／4／8／16／32 秒，最壞 133 秒）、`startRun` 的 10 秒上限，以及 `stock_symbol_sync` 整輪失敗記 `failed`、
對帳前置不成立的專屬原因，都是缺陷修正，**一律生效**（見 [`api-reference.md`](../api-reference.md) 的 `/scheduler/status`）。

### 部署與回滾

四個環境變數（`JOB_RETRY_ENABLED`、`JOB_RETRY_INTERVAL_MIN`、`JOB_RETRY_MAX_RERUNS`、`JOB_RETRY_CUTOFF`）要在
repo 的 `docker-compose.yml`、`docker-compose.dev.yml`、`deploy.sh` 範本，以及 live 的 `compose.yml` 都有，否則靜默失效
（[`development-workflow.md`](../development-workflow.md)「新增排程開關要改三個地方」）。backend 啟動時記一行 Info
`job retry config`：開關、間隔、上限、截止時刻與三個工作的最晚起跑時刻，部署後看它確認。
回滾：`JOB_RETRY_ENABLED=false` 重新部署，確認那一行顯示 `enabled=false`。

## 不負責事項

- 不計算 `chip_scores`，那屬於 Analysis Pipeline。
- 不計算指標與訊號。
- 不建立 SR Zone。
- 不訓練或選擇模型。
- 不產生買賣建議。

## 近期整理方向

- 將文件中的「資料同步」與「分析計算」描述分開。
- 將 K 棒回補、籌碼回補、盤中資料同步標記為 Data Pipeline job。
- 後續若做程式重構，可先抽出 data job interface，避免 scheduler 直接混入分析與決策邏輯。

---

## 公司行動同步（`corporate_action_sync`）

維護 `corporate_actions` 與 `candles` 的還原係數。**預設平日 06:30** 執行
（`corporate_action.cron`，環境變數 `CORPORATE_ACTION_CRON`，台北時區；
06:30 早於 08:50 的 `pre_market`，讓當天開盤前的分析已吃到最新係數），
或 `POST /api/v1/scheduler/corporate-action-sync/run` 手動觸發
（前端「排程狀態」頁有按鈕）。**重算是冪等的**，重複觸發不會累積誤差，
所以需要多跑幾次時直接設多時段即可（例如 `"30 6,12 * * 1-5"`）。

**這支排程沒有 `enabled` 開關**，與 `stock_symbols` / `sr_evaluation` /
`evaluation_universe` 不同：是否啟用取決於程式有沒有注入 adjuster
（`Scheduler.SetAdjuster`）。漏跑一次就會讓該檔整段歷史出現假跳空，而重算是冪等的，
所以沒有「刻意關掉」的情境。config 的 `cron` 留白時會退回程式內的預設值而不是靜默不註冊。

兩個來源的取得成本差很多，處理方式因此不同：

| 類型 | 來源 | 方式 | 規模 |
|---|---|---|---|
| 分割／反分割／面額變更 | FinMind `TaiwanStockSplitPrice` | **一次批次請求抓全市場**，每次整段重抓 2015 年起 | 全市場 11 年僅 33 筆 |
| 除權息 | Yahoo `dividendsByYear` | **逐檔**，標的來源是 `candles` 內所有相異 symbol | 每檔約 12～34 筆 |
| 減資 | FinMind `TaiwanStockCapitalReductionReferencePrice` | **逐檔**（整批需 Sponsor tier），與除權息在同一個迴圈 | 全市場稀少 |

**分割為什麼整段重抓而不做增量**：一次請求就抓得完，而「增量」需要維護游標、
漏一次就永久缺一筆。事件表是 upsert、重算是冪等的，整段重抓沒有副作用。

**除權息的標的來源刻意不是 watchlist**：評估標的池（見 todo.md T-040）的標的不在
watchlist 裡，只跑 watchlist 會讓它們「分割有還原、除權息沒有」，而且不會報錯。

**除權息與減資合併在同一個迴圈**（`Adjuster.SyncPerSymbolEvents`）：重算要 UPDATE 該檔的
整段歷史，分開跑會做兩次。兩者打的是不同 host（Yahoo／FinMind），各有各的節流器，
合併不會互相排擠。

### 規模限制：除權息與減資都是逐檔查詢

Yahoo 的 symbol 在 URL path 裡，**沒有批次端點**。受 `yahoo.rate_limit`
（每分鐘 20，與盤中報價**共用同一個節流器**——同一個 host，各自節流會讓實際速率加倍）節制：

- 目前約 28 檔：約 1.5 分鐘。
- 擴到 1,900 檔（T-040 的終局）：**數小時**。

因此**增量更新是擴標的池的前置條件**，不是日後優化。目前是每次全抓。

### 回補之後會立即重算

回補可能插入**比公司行動更早**的 K 棒，而 `BulkInsert` 寫入的係數是預設值 1。
`fetcher.BackfillHistory` 因此在成功寫入後呼叫 `Adjuster.RecomputeAffected`，
只重算**有事件**的標的（全市場 33 筆事件只涵蓋 31 檔，無腦全算會變成大量無謂的全表 UPDATE）。

重算失敗**不計入回補的 failed 數**：K 棒已經寫進去了，算成「回補失敗」會誤導呼叫端去重抓。
改記 Error log，靠隔天排程與 `scripts/verify-adjustment.sh` 補救。

### 每日抓取不需要重算

每日抓取寫入的是最新一根 K 棒，位置在所有公司行動之後，係數本來就是 1。
只有回補會插入比事件更早的 K 棒。
