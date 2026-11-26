# T-089 計畫書 v1.10：可用首版——台股盤後交易計畫

> 狀態：v1.10（2026-11-26：v1.5 複審通過；程式在 `t089` 分支實作完成，實作 review 兩輪通過；第六節的 dev 實跑驗收完成，紀錄在第十節）。第八節的 live 唯讀盤點已完成（重用期限 24 小時、計畫 0 筆）。對應：`docs/todo.md` T-089。
> 需求：照 T-089 的三項交付（接通入口、顯示資料是否可用、用實際流程驗收）與「計畫書要優先定清的兩件事」；
> 功能檢討的「下一期回看」（5 個交易日試用）⛔ 不在本案，首版可操作後另外處理（使用者 2026-11-25 裁決）。
> 依據：[`feature-reviews/2026-11-19-first-usable-release.md`](../feature-reviews/2026-11-19-first-usable-release.md)。
> 時程：程式在 `t089` 分支實作；I-139（11-28）與 I-141（11-30）上 live 之前⛔ 不合併進 init。

## 一、目標與不做的範圍

**目標**

1. **接通入口**：首頁的觀察清單每一列可以直接開啟那一檔的交易計畫（交易分析頁），⛔ 不用再重填股票代號。
   已有投組時沿用目前選的投組；沒有投組時引導建立，建立後直接回到原本那一檔。
2. **顯示資料是否可用**：
   * 分開標示**行情日期**（這次分析實際用到的最後一根日 K）、**分析紀錄時間**（這筆計畫寫入的時間）與**歷史快照**。
   * 「最新」要同時符合三件事：行情日期是**當下應有的行情日**、持倉**版本沒有變**、來源**確認是日 K**（D2、D3）。
   * 判斷結果（例如「等待」）與資料狀態（等待當日資料、行情過期、資料不足、行情狀態無法確認、更新失敗）分開顯示，⛔ 不混在同一個標籤。
   * 更新失敗時，畫面上的既有計畫標成歷史，仍可回看，⛔ 不被誤認成最新的判斷。
3. **行情日期以分析實際用到的日 K 為準**：SR 快照只在「用的是目前最新那根日 K」時才重用；新日 K 入庫後一定重算。
4. **切換與競爭**：每條請求鏈從開始就固定投組與標的；切換後，較晚返回的舊回應（載入或分析，成功或失敗）⛔ 不能改掉目前選擇的任何畫面狀態。

**不做**

* ⛔ 不改判斷邏輯：SR 評分、進場許可、持有決策、部位計算全部照舊（Python 與 `portfolio` 的決策程式⛔ 不動）。
* ⛔ 不做 5 個交易日的試用回看（另外處理）。
* ⛔ 不做進階警報、擴大標的池、整套風控與配置（功能檢討的取捨；T-081 另案）。
* ⛔ 不重排交易分析頁的進階區塊：首屏只加一張「計畫摘要」卡，既有的細節區塊照原樣放在下方（Q3）。
* ⛔ 不做多使用者的資料隔離與權限（功能檢討「共用環境與權限」；首版自用）。
* ⛔ 不收窄 API：兩支交易分析端點照舊接受 `timeframe`、歷史照舊依投組與標的回全部紀錄；只有「確認是日 K」的計畫能進入摘要的「最新」判定（D2）。
* ⛔ 沒有 migration：需要的日期與版本都已經存在（第二節）；來源 timeframe 新紀錄寫進既有的 `evidence` JSON。

## 二、現況（2026-11-25 讀碼）

**入口**

* 首頁的觀察清單點一列只會把 `selectedSymbol` 設成那一檔、切換右邊的 K 線圖；沒有到交易分析的入口。
* 交易分析頁（route `positions`，側欄「交易分析」）的股票代號是自己的輸入框，⛔ 不讀 `selectedSymbol`（籌碼頁有讀，是既有慣例）。
* 投組選擇存在 `selectedPortfolioID`（localStorage）；沒有投組時頁面只顯示「請先建立 Portfolio」，建立後若輸入框有代號才會重新載入。

**行情日期**

* Python 的 SR 評分把 `analyzed_at` 設成**最後一根 K 棒的時間**（`frame.index[-1]`），⛔ 不是牆上時間；Go 原樣存進 `stock_sr_zone_analyses.analyzed_at`。
  日 K 的時間是台北當日 00:00（FinMind 的日期）。
* 現行分析器（`rule_version` = `position_sr_zone_v1`）建立計畫時，`position_analyses.analyzed_at` 直接複製 SR 快照的 `analyzed_at`，
  `sr_zone_analysis_id` 記快照 id，`position_version` 記當時的持倉版本；`created_at` 是寫入時間。
* **來源 timeframe 沒有記在計畫上**：分析 API 接受 `timeframe`（前端固定送 `1d`），計畫表沒有 timeframe 欄，`evidence.decision_context` 也沒有。
  要確認來源只能回查 SR 快照的 `timeframe`，而 SR 快照可以從 SR 頁刪除（`DELETE /sr-zones/:id`）。
* migration 038 從舊表 `holding_analyses` 搬過來的紀錄：`rule_version` = `holding_sr_zone_v1_legacy`、`position_version` = 0，
  `sr_zone_analysis_id` 是**原樣搬移**的舊值（⛔ 不能拿「有 SR id」當成來源可確認），`analyzed_at` 的語意無法確認。
* 前端目前只顯示 `created_at`（分析紀錄時間），⛔ 沒有顯示行情日期；歷史清單也只有 `created_at`。

**SR 快照的重用**

* `SRAnalysisProvider.Analyze` 在沒有強制刷新時，取同一檔最近的快照（依 `created_at` 由新到舊），
  第一筆 timeframe 相同、`now − analyzed_at` 在 0 到 `POSITION_ANALYSIS_SR_REUSE_MAX_AGE_HOURS`（預設 24）小時之間的就重用。
* 因為 `analyzed_at` 是日 K 日期的 00:00，預設 24 小時時重用窗口實際上是「那根日 K 的當天」；**這是設定值的副作用**：
  設定超過 24 小時，新日 K 入庫後仍會重用舊快照，產生一筆「分析紀錄時間較新、行情日期卻是舊的」計畫。live 的實際設定值在實作前唯讀確認（第八節）。
* 同一個 provider 也被 SR 頁的 `reuse_existing=true` 使用。

**K 棒查詢與交易日**

* `CandleRepo.GetLatest` 查無資料時回 `sql.ErrNoRows`，DB 故障時回其他錯誤，兩者分得開。
* I-139 的共用判斷 `tradingday.Judge.On(ctx, day)` 可以判斷任何一天，`Result.Source` 是 `weekend`／`manual`／`calendar`／`fallback`；
  日曆取不到時照平日（`fallback`，fail-open）。**讀手動設定失敗或逾時**時只記 Warn、照年度日曆判斷，回的仍是 `calendar`——
  呼叫端看不出那一天的手動設定其實沒有讀到。

**失敗、資料不足與持倉版本**

* `POST /trade-analysis/analyze`：Python 回 404（找不到日 K，或日 K 根數不足）→ 404；503（機率模型尚未訓練）→ 503；其他（逾時、Python 連不上）→ 500 或 502。
  前端一律顯示錯誤字串，畫面上的舊計畫照樣顯示、⛔ 沒有任何標記。
* 持倉（`positions`）每次買賣或更正 `version` 加一；沒有持倉列的標的版本是 0。同一天先分析空手、再買進，舊的空手計畫行情日期相同，現在看不出它已經不適用。
* 「等待」是判斷結果：`action` 為 `WAIT`（`action_label`「等待」）；SR 決策裡的 `WAIT_CONFIRMATION` 顯示為「等待確認」。

**競爭**

* `select()` 依序 await 持倉、交易流水、歷史三個請求，每一步都重新讀全域的 `symbol` 與 `$selectedPortfolioID`，⛔ 沒有請求序號；
  切換股票或投組時，較慢的舊回應會蓋掉新的，甚至同一條鏈的後半段讀到新的選擇。
* `runAnalysis()` 回來後無條件設定「最新分析」並呼叫 `select()`；`busy`、`error` 是整頁共用的，⛔ 沒有綁定標的。
* `saveTrade()`、`saveAdjustment()` 同樣：回應回來就直接改 `current`、清表單，再用當下的 `symbol` 重新載入；表單內容是整頁共用的。

**權限**

* 唯讀投組可以讀持倉與歷史，⛔ 不能執行分析（後端 `requirePortfolioAccess(…, true)`、前端停用按鈕）。本案照舊（Q1）。

## 三、設計

### D1：SR 快照只在「同一根日 K」時重用

* `SRAnalysisProvider` 取候選快照之前，先查這一檔、這個 timeframe 最新一根 K 棒（`CandleRepo.GetLatest`）。
* 重用條件改成**兩個都成立**：
  1. 快照的 `analyzed_at` **等於**最新 K 棒的時間（同一根）；
  2. 既有的 timeframe 與年齡條件照舊。
* 查不到最新 K 棒（查無資料或查詢失敗）→ ⛔ 不重用、照常呼叫 Python（Python 會自己回 404 或算出結果），⛔ 不因為這次查詢失敗而讓分析失敗。
* 只會讓重用**變少**、⛔ 不會變多。SR 頁的 `reuse_existing=true` 走同一個 provider，一起適用。
* 同一根日 K 的數值事後被改寫（例如同日重抓）時，仍可能重用改寫前的快照；照舊靠「強制刷新 SR」，記為已知限制。

### D2：API 回應補上行情日期、來源與資料狀態（只加欄位）

**新計畫多記來源**：分析器在 `evidence.decision_context` 加 `timeframe`（SR 快照的 timeframe）與 `market_data_at`（SR 快照的 `analyzed_at`）。只加 JSON 鍵、⛔ 不改既有鍵。

**每一筆計畫的衍生欄位**（`POST /trade-analysis/analyze` 與 `GET /trade-analysis/:symbol/history`，⛔ 不存 DB）：

* `market_date`：`analyzed_at` 的台北日期 `YYYY-MM-DD`；來源無法確認時是 `null`。
* `source_confirmed_1d`：布林。依序判定，第一個成立的決定結果：
  1. `rule_version` ⛔ 不是 `position_sr_zone_v1`（例如 038 的 `holding_sr_zone_v1_legacy`）→ `false`；
  2. `evidence.decision_context.timeframe` 有值 → 等於 `1d` 才是 `true`（新紀錄）；
  3. 回查 `sr_zone_analysis_id` 的 SR 快照：存在、`timeframe` 是 `1d`、`analyzed_at` 等於計畫的 `analyzed_at` → `true`（本案之前的現行紀錄）；
  4. 其他（快照已刪、對不上）→ `false`。
  回查用一句 `IN` 查詢一次取完；回查失敗時這一批都當 `false`，並在 `data_status.source_lookup` 記 `error`。

**共用判斷補一個內部旗標**：`tradingday.Result` 加 `OverrideUnavailable`（讀手動設定失敗或逾時時為 true；`json:"-"`，⛔ 不出現在既有的
`/market-day-overrides`、`/intraday/status` 回應）。判斷結果與排程的降級行為⛔ 不變（照舊用日曆判斷），只是讓本案的呼叫端知道那一天沒有確認過手動設定。

**回應頂層的 `data_status`**（兩支端點都有，針對這一個投組與標的）：

| 欄位 | 內容 |
|---|---|
| `latest_candle` | `ok`（有）／`no_data`（查無日 K）／`error`（查詢失敗） |
| `latest_candle_date` | `ok` 時是最新一根日 K 的台北日期，其餘 `null` |
| `expected_date` | 當下**應有的行情日**（見下） |
| `settled_date` | 今天之前最近一個交易日（盤前、休市日、收盤後都應該已經有它的日 K） |
| `awaiting_today` | 今天是交易日、台北時間已過 13:30、`latest_candle_date` 早於今天 → `true` |
| `calendar` | `confirmed`：推算用到的每一天都是 `weekend`、`manual`，或 `calendar` 且手動設定有讀到；`unconfirmed`：有任何一天是 `fallback`、有任何一天 `OverrideUnavailable`，或往回 15 天找不到交易日 |
| `position_version` | 目前持倉的版本（沒有持倉列是 0；讀取失敗是 `null`） |
| `source_lookup` | `ok`／`error`（D2 的 SR 快照回查） |
| `as_of` | 伺服器產生這份狀態的時間 |

**應有的行情日**（用 `Judge.On` 由今天往回推）：

* 今天是交易日、而且台北時間已過 13:30 → `expected_date` 是**今天**；
* 其他（盤前與盤中、休市日、週末）→ `expected_date` 是 `settled_date`（今天之前最近一個交易日）。
* 13:30 是等待當日日 K 的起點（Q2）：13:30 到當天日 K 入庫之間，用 `settled_date` 的計畫仍算「最新」，另外提示「等待今日日 K」。

### D3：前端狀態推導（集中在一個工具函式）

**摘要卡用哪一筆**：這個投組、這一檔的歷史裡，`source_confirmed_1d` 為 true 的最新一筆；一筆都沒有時才用最新的一筆（主狀態會是「來源無法確認」）。

新增 `frontend/src/lib/utils/tradePlanStatus.ts`，輸入計畫、`data_status`、這個鍵最近一次分析的結果，輸出**一個主狀態**加上可以並存的提示，各自有單元測試。
主狀態依下表**由上往下**取第一個符合的：

| 主狀態 | 條件 | 畫面 |
|---|---|---|
| 資料不足 | 這個鍵最近一次分析回 404，或 `latest_candle` 是 `no_data` | 「資料不足：日 K 不夠，無法產生計畫」，⛔ 不顯示判斷；既有計畫只在歷史清單 |
| 行情狀態無法確認 | `latest_candle` 是 `error`、`calendar` 是 `unconfirmed`、`position_version` 是 `null`，或 `source_lookup` 是 `error` | 「行情狀態無法確認」＋原因；有計畫的話照樣顯示、標成歷史 |
| 尚無計畫 | 這個投組、這一檔沒有任何計畫 | 「尚無計畫」＋「分析」按鈕；`latest_candle_date` 早於 `settled_date` 時另外提示「最新日 K 只到 11-20（應有 11-24），分析結果會是過期的行情」 |
| 來源無法確認 | `source_confirmed_1d` 是 false | 「舊紀錄或非日 K：無法確認行情日期」，一律當歷史 |
| 歷史：日 K 已變動 | `market_date` **晚於** `latest_candle_date`（計畫用的那根日 K 已不在庫內，例如被刪除或重建） | 「計畫用的 11-25 日 K 已不在庫內（最新只到 11-24），請重新分析」 |
| 歷史：更新失敗 | 這個鍵最近一次分析失敗（非 404） | 「更新失敗（16:05）：顯示的是 11-24 的歷史計畫」 |
| 歷史：持倉已變動 | 計畫的 `position_version` ⛔ 不等於 `data_status.position_version` | 「持倉已變動（買賣或更正後），請重新分析」 |
| 歷史：有較新的日 K | `market_date` 早於 `latest_candle_date` | 「用 11-24 的日 K · 已有 11-25 的日 K，請重新分析」 |
| 行情過期 | `market_date` 等於 `latest_candle_date`，但早於 `settled_date`（日 K 停更多天） | 「行情過期：最新日 K 只到 11-20（應有 11-24）」 |
| 歷史：回看 | 從分析歷史點選的非最新一筆 | 「歷史計畫（11-21 分析）」 |
| 最新 | `market_date` **等於** `latest_candle_date`，而且等於 `expected_date`，或（`awaiting_today` 且等於 `settled_date`） | 「行情日期 11-25 · 分析於 11-25 16:02」 |
| 行情狀態無法確認（其他） | 以上都不符合（例如庫內日 K 比應有行情日還新） | 「行情狀態無法確認：庫內日 K 11-26 晚於應有行情日 11-25」，標成歷史 |

最後一列是兜底：「最新」只能由倒數第二列的條件產生，⛔ 不能因為沒有命中其他狀態就變成最新。

可以並存的提示：

* `awaiting_today`：「今日日 K 尚未入庫（收盤批次 15:00），目前最新是 11-24」。
* 判斷結果照 `action_label` 顯示；「等待」類的判斷用判斷的樣式，旁邊列出 `trigger_conditions`（下一個觀察條件），跟資料狀態分開。
* 主狀態不是「最新」時，摘要卡直接放「分析」按鈕（唯讀投組照既有規則停用並說明）。

### D4：入口與首屏摘要

* **觀察清單**：每一列加一個「計畫」按鈕（點列本身仍是切換 K 線圖）。按下去：`selectedSymbol` 設成那一檔，route 切到 `positions`。
* **交易分析頁**：
  * 開頁時讀 `selectedSymbol`，有值就帶進代號並自動載入持倉與分析歷史（**只讀、⛔ 不自動分析**，Q1）。
  * 從觀察清單**兩步內**看到計畫：按「計畫」→（主狀態不是「最新」或沒有計畫時）按「分析」。
  * 沒有投組：摘要卡位置顯示建立投組的欄位與「建立後會開啟 2330 的計畫」；建立成功後自動載入原本那一檔。
* **計畫摘要卡**（新，頁面最上方）：判斷結果、主要理由（`reason` 第一條）、下一個觀察條件、行情日期、分析紀錄時間、D3 的主狀態與提示。
  既有區塊照原樣在下方；分析歷史每一列加上行情日期。

### D5：請求鏈固定選擇

* 每條請求鏈（載入：持倉 → 流水 → 歷史；分析：分析 → 重新載入）在**開始時**取一份「選擇」（投組 id＋代號）與遞增的序號，
  鏈裡每一步都用這份選擇發請求，⛔ 不再讀全域的 `symbol`／`$selectedPortfolioID`。
* 回應要套用到畫面之前，先比對：這份選擇等於目前的選擇、而且序號是這個選擇最新的。⛔ 不符合就丟掉（不改計畫、持倉、流水、摘要、錯誤、載入中狀態）。
* 分析的結果另外記在發出請求的那個鍵上（成功：下次載入那個鍵時歷史自然有新的一筆；失敗：記下「更新失敗」）；回到那個鍵時才顯示。
* **買賣與更正也走同一套**：`saveTrade`、`saveAdjustment` 在開始時取選擇與序號，`expected_version` 用那份選擇當時的持倉版本；
  回應回來時選擇已經換掉，就⛔ 不改 `current`、⛔ 不清表單、⛔ 不顯示錯誤、⛔ 不用當下的代號重新載入；結果記在發出請求的那個鍵上
  （成功：回到那個鍵時重新載入就看得到；失敗：回到那個鍵時顯示「買進寫入失敗：…」）。切換股票或投組時，交易與更正的表單一律清空。
* `busy`（載入中、分析中、寫入中）與錯誤訊息改成依鍵記錄：A 檔分析中切到 B 檔，B 的按鈕⛔ 不被 A 的分析鎖住，A 回來的錯誤⛔ 不出現在 B。
* 切換股票或投組時，先清掉畫面上的計畫、持倉、流水，再載入新的；載入中顯示「載入 2330…」，⛔ 不保留上一檔的內容。

## 四、受影響的檔案、模組與資料流

* backend
  * `internal/analysis/sr_analysis_provider.go`：D1 的重用條件（注入 candle repo）；呼叫端（`portfolio.NewAnalyzer`、`api/server.go`）跟著傳入。
  * `internal/portfolio/analyzer.go`：`evidence.decision_context` 加 `timeframe`、`market_data_at`（只加鍵，⛔ 不動判斷）。
  * `internal/tradingday/judge.go`：`Result` 加 `OverrideUnavailable`（`json:"-"`）；判斷結果與排程行為⛔ 不變。
  * `internal/api/handler/trade_analysis.go`：D2 的衍生欄位與 `data_status`（注入 candle repo、SR repo 的批次讀取、`tradingday.Judge`）。
  * `internal/store`：SR 快照依 id 批次讀 `id, timeframe, analyzed_at` 的查詢（三種 DB 都要）。
  * 測試：provider、analyzer 的 evidence、handler 的欄位與邊界。
* frontend
  * `src/components/market/WatchlistTable.svelte`：「計畫」按鈕。
  * `src/routes/Positions.svelte`：讀 `selectedSymbol`、摘要卡、D5 的請求鏈、沒有投組的引導。
  * `src/lib/api/tradeAnalysis.ts`：回應型別加 `market_date`、`source_confirmed_1d`、`data_status`。
  * `src/lib/utils/tradePlanStatus.ts`（新）與測試；`src/routes/Positions.test.ts`（新）。
* 資料流：⛔ 沒有新的資料表或欄位；分析仍是一次寫一筆 `position_analyses`（`evidence` 多兩個鍵）；SR 快照的重用變少時 Python 的評分呼叫會略增（只在新日 K 入庫後的第一次分析）。
* contract：兩支交易分析端點的回應**只加欄位**；`evidence` 只加鍵；⛔ 沒有 schema、設定鍵或 migration 的變更。

## 五、主要風險與回滾

| 風險 | 處理 |
|---|---|
| D1 讓重用變少，分析變慢 | 只有快照不是最新日 K 時才重算，正是要的；查最新日 K 是單筆索引查詢 |
| 查最新日 K 失敗讓分析失敗 | D1 退回「不重用、照常呼叫 Python」，⛔ 不回錯；D2 顯示「行情狀態無法確認」，⛔ 不顯示成資料不足 |
| 應有行情日依賴共用休市判斷 | 用到 `fallback` 的那一天會讓 `calendar` 變 `unconfirmed`，畫面顯示「行情狀態無法確認」，⛔ 不誤判成過期或最新 |
| 共用判斷加旗標影響 I-139 | 只加一個 `json:"-"` 的欄位，判斷結果、排程的閘門與降級⛔ 不變；I-139 的既有測試全部照舊通過 |
| 往回推交易日多打日曆 | 共用判斷的日曆是逐年快取、失敗共用 5 分鐘；最多 15 天、跨年最多兩個年份 |
| 舊紀錄的行情日期被誤認 | 只有 `position_sr_zone_v1` 且確認是 `1d` 的計畫能進「最新」；038 的舊紀錄、SR 快照已刪的一律「來源無法確認」 |
| 前端自動載入多打 API | 開頁只讀三個既有的 GET；⛔ 不自動分析 |
| 回滾 | 退回舊 image；回應多出的欄位與 `evidence` 多出的鍵舊版都⛔ 不讀，⛔ 沒有 DB 狀態要還原 |

## 六、測試與驗證

**backend（`backend/scripts/test.sh`）**

* provider：快照是最新日 K 且在年齡內 → 重用；最新日 K 比快照新 → 重算（模擬設定 72 小時，仍在年齡內）；查無 K 棒、查詢失敗 → 重算且⛔ 不回錯；強制刷新照舊。
* analyzer：新計畫的 `evidence.decision_context` 有 `timeframe`、`market_data_at`，其他鍵⛔ 沒有變。
* handler 的 `source_confirmed_1d`：四條判定各一個案例（038 舊紀錄帶著舊 SR id → false；新紀錄 evidence 是 `1d` → true、是 `1m` → false；
  沒有 evidence 鍵、回查快照是 `1d` 且時間相同 → true；快照已刪或時間對不上 → false；回查失敗 → 全部 false 且 `source_lookup` = `error`）。
* handler 的 `data_status`：
  * `latest_candle`：有 → `ok`；`sql.ErrNoRows` → `no_data`；其他錯誤 → `error`（⛔ 不是 `no_data`）。
  * 應有行情日：交易日 09:00 → 前一交易日；交易日 13:29 → 前一交易日；13:30 → 今天；週一盤前 → 上週五；連假中 → 連假前最後一個交易日；
    往回推碰到 `fallback` → `unconfirmed`；往回推的某一天讀手動設定逾時（回 `calendar`、`OverrideUnavailable`）→ `unconfirmed`；跨年（1 月初往回推到前一年）。
  * `tradingday`：讀手動設定失敗、逾時 → `Source` 仍是 `calendar`、`IsTrading` 照日曆、`OverrideUnavailable` 為 true；讀到（含沒有設定）→ false；
    `/market-day-overrides` 與 `/intraday/status` 的回應⛔ 沒有新欄位。
  * `awaiting_today`：交易日 13:30 且最新是昨天 → true；最新是今天 → false；休市日 → false。
  * `position_version`：有持倉列、沒有持倉列（0）、讀取失敗（`null`）。
* 反證：拿掉 D1 的同一根日 K 條件、把 `no_data` 與 `error` 合併、拿掉 `rule_version` 判定、`calendar` 只看 `fallback`，新測試要失敗。

**frontend（`frontend/scripts/test.sh`：svelte-check、vitest、build）**

* `tradePlanStatus`：D3 主狀態表的每一列各一個案例，加上「排在前面的優先」的組合（例如持倉已變動＋有較新的日 K → 持倉已變動），以及兩個提示。
  特別案例：日 K 停更多天、計畫日期等於最新入庫日期 → 「行情過期」、⛔ 不是「最新」；同一天空手計畫、之後買進 → 「持倉已變動」；
  計畫用 11-25、庫內最新退回 11-24、`expected_date` 是 11-25 → 「日 K 已變動」、⛔ 不是「最新」；沒有計畫 → 「尚無計畫」（停更時帶提示）；
  庫內日 K 晚於應有行情日 → 兜底的「無法確認」。
* `Positions.test.ts`（請求的完成順序由測試控制）：
  * 有 `selectedSymbol` 時開頁就載入那一檔；⛔ 不呼叫分析。
  * 沒有投組 → 建立 → 自動載入原本那一檔。
  * 載入鏈：A 的持倉回來前切到 B → A 的流水與歷史請求仍用 A 的選擇送出，回應⛔ 不套用到畫面。
  * **分析回應**：在 A 送出分析、切到 B，A 才回**成功** → B 的摘要、歷史、載入狀態⛔ 不變；回到 A 看得到新的一筆。
  * **分析回應**：在 A 送出分析、切到 B，A 才回**失敗**（500）→ B ⛔ 沒有錯誤訊息、⛔ 沒有「更新失敗」；回到 A 才顯示「更新失敗」。
  * A 分析中切到 B：B 的「分析」按鈕可以按（⛔ 不被 A 鎖住）。
  * **寫入回應**：在 A 送出買進、切到 B，A 才回**成功** → B 的持倉、交易表單、更正表單、錯誤⛔ 不變，⛔ 沒有用 B 的代號重新載入；回到 A 看得到新的持倉。
  * **寫入回應**：在 A 送出更正、切到 B，A 才回**失敗**（409 版本衝突）→ B ⛔ 沒有錯誤訊息；回到 A 才顯示更正失敗。
  * 切到 B 時交易與更正的表單是空的（⛔ 不帶 A 填到一半的內容）。
  * 切換投組（同一檔）同樣的載入、分析、寫入案例。
  * 分析回 404 → 「資料不足」、⛔ 不顯示判斷；`latest_candle` 是 `error` → 「行情狀態無法確認」、⛔ 不是資料不足。
* `WatchlistTable`：按「計畫」設定 `selectedSymbol` 並切換 route；點列本身⛔ 不切換 route。
* 反證：拿掉選擇比對或序號比對，競爭的案例（載入、分析、寫入）要失敗；把 `busy` 改回整頁共用，「B 的按鈕可以按」要失敗。

**dev 實跑（實際流程驗收，`docker-compose.dev.yml`，Q4）**

由 Claude 在 dev 用瀏覽器（無頭）走完整流程，每一步留截圖與畫面文字給使用者確認；較晚回應的時序⛔ 不在瀏覽器裡驗（由前端測試控制）。
無頭瀏覽器在 dev 跑不起來時，改由使用者經通道操作（臨時加金鑰、驗收完移除），先回報再換。

**驗收環境：可丟棄的 dev DB**（v1.4，取代 v1.3 的逐筆清理）

合成日 K 一旦存在，dev 的排程（SR 分析的身分追蹤、SR 驗證、指標與訊號）就可能用它更新**既有**的區間、事件與狀態；
逐筆刪快照復原不了這些改動，事後查殘留的 `analysis_id` 也驗不出來。所以驗收整段在一個**可丟棄的 database** 上做，結束後整個丟掉，⛔ 不逐筆刪除：

* **切換前**（服務還在跑、連著 `trading` 時）：
  * **回測雙零檢查**：`trading` 的 `backtest_jobs` 裡 `pending` 與 `running` 都是 0（沿用 `development-workflow.md`「部署前的回測雙零檢查」）；⛔ 不是 0 就等它結束，⛔ 不切換。
    停 Python 會中斷執行中的回測，服務恢復後會把留在 `trading` 的 `running` 回收成失敗。驗收期間⛔ 不送回測。
  * **記下服務狀態**（review 2026-11-26 修正：⛔ 不用 `ps`，它只列在跑的）：`docker compose -f docker-compose.dev.yml --profile worker ps -a --format json`
    （含 `worker` profile 的 `python-worker`、含已停止的容器）存進作業目錄，整理成三份清單：
    * **原本在跑**：容器存在而且 `State` 是 `running` 的服務；
    * **原本停著**：容器存在、⛔ 不是 `running`（`exited`、`created`…）的服務；
    * **原本沒有容器**：`--profile worker config --services` 列得出、但⛔ 沒有容器的服務（2026-11-26 查：`python-worker` 目前就是這種）。

    `ps -a` 也會列出同一個 project、但⛔ 不在這份 compose 裡的容器（2026-11-26 查：別份 compose 留下的 `fake-yahoo`）：只記錄、⛔ 不動，復原後的核對狀態要一樣。
    `python-worker` 驗收用不到（瀏覽器流程與強制刷新 SR 走 python-server），⛔ 不啟動；開跑前用 `ps -a` 再確認它原本沒有容器，驗收後確認它仍然沒有容器。
  * **image 的回復點**（v1.8）：dev compose 的 project 固定是 `stock_trading_dev`，換 worktree 仍共用同一組預設標籤（`stock_trading_dev-<服務>:latest`），
    在 t089 worktree build 會把 t089 的程式蓋進這個標籤。build 之前逐一服務記下兩個值（兩者可能不同）：
    * **容器實際用的 image ID**（`docker inspect -f '{{.Image}}'` 容器）——用它打回復標籤 `stock_trading_dev-<服務>:pre-t089`；
    * **預設標籤原本指向的 ID**（`docker image inspect -f '{{.Id}}'` `stock_trading_dev-<服務>:latest`）——記進作業目錄，復原時把 `latest` 標回它。

    沒有容器的服務（`python-worker`）只記預設標籤的 ID。照 `development-workflow.md`「部署失敗時」的做法：⛔ 不靠 ID 或「沒改程式所以一樣」認舊版。
  * **Redis DB 5 要能獨占**：開跑前確認 dev Redis 的 DB 5 是空的（`DBSIZE` 是 0）、沒有其他作業在用；驗收 backend 啟動後核對它的有效設定是 `REDIS_DB=5`。
* **建立**：dev 的服務全部停下（`stop`）後，只啟動 dev postgres，用 `CREATE DATABASE t089_accept TEMPLATE trading` 複製一份 dev 的 `trading`
  （範本複製要求來源⛔ 沒有連線；先確認磁碟空間夠）。dev 原本的 `trading` 在驗收期間⛔ 沒有任何服務連著，內容⛔ 不會變。
  ⚠️ dev postgres 要掛**一般的 dev volume**：用作業隔離 override（掛 `stock_trading_dev_i131_pgdata`，I-123 判讀與 I-140 重播在用）的作業要先停下，兩者⛔ 不能同時跑。
* **連線**：用 repo 外的驗收 override（放在 dev 作業目錄，權限 700）把 backend、python-server、python-worker 的 `DATABASE_DSN` 改指 `t089_accept`，
  backend 的 `REDIS_DB` 改用另一個編號（⛔ 不跟一般 dev 共用快取）。啟動用 `docker compose -f docker-compose.dev.yml -f 驗收 override up -d`。
* **合成日 K、計畫、SR 快照、排程的任何寫入都只進 `t089_accept`**；排程照常跑也沒關係（跑過哪些照實記錄）。
* **專用帳號**：在 `t089_accept` 用新的驗收帳號建專用投組，步驟 1～6、8 在這個投組做；步驟 7 另用一個沒有投組的新帳號。
  `development-workflow.md`「dev 取得 JWT：每次用新的驗收帳號」的 `dev_token` 把啟用帳號的 SQL 固定送到 `trading`，切換後註冊寫進 `t089_accept`、啟用卻會 `UPDATE 0`；
  驗收改用一份把 `-d` 換成 `t089_accept` 的版本，⛔ 不改共用的 `dev_token`。
* **所有直接寫 DB 的 SQL 都明確指向 `t089_accept`**（啟用帳號、補合成日 K）：每一次寫入前先查 `SELECT current_database()`，⛔ 不是 `t089_accept` 就中止；
  寫入後核對筆數（啟用 `UPDATE 1`、補日 K 的筆數等於要補的日期數），並確認 `trading` 裡⛔ 沒有那個驗收帳號。

**驗收前準備**（dev 的 FinMind token 無效，日 K 停在失效前，⛔ 不能靠排程抓新資料）：

* 開始時先讀一次 `data_status`，記下當下的 `expected_date`、`settled_date`（驗收跨過 13:30 或跨日就重讀，依新的值調整）。
* **新鮮檔 X**：在 `t089_accept` 為 X 補日 K 到 `settled_date` 的**前一個交易日**（合成的日 K：沿用最後收盤、小幅波動）。
* **過期檔 Y**：⛔ 不補，日 K 停在失效前，留給「行情過期」。

**復原與丟棄**（成功或中途失敗都要做；**服務復原與資料丟棄是兩件獨立的事**，前一件失敗⛔ 不擋後一件）：

1. **服務復原**（先做）：用 `docker compose -f docker-compose.dev.yml -f 驗收 override --profile worker stop` 停掉驗收期間的所有服務；
   **image 分兩段標回去**（v1.8；v1.10 修正順序）：compose 重建容器時用的是 `latest` 當下指向的 image，而「容器原本用的 ID」與「`latest` 原本指向的 ID」可能不同，所以：
   * **重建之前**：每個服務的 `latest` 先指向 `:pre-t089`（也就是**容器原本用的 image**）；
   * 接著**依切換前記下的三份清單**用一般的設定、一律帶 `--no-build` 重建（DSN 回到 `trading`、Redis 回到原本的編號），容器就回到原本的 image；
   * **重建之後**：再把 `latest` 標回切換前記下的「預設標籤原本指向的 ID」（已經建好的容器⛔ 不受標籤影響）。兩個 ID 相同時這一步不會改變任何東西。
   * 沒有容器的服務（`python-worker`）沒有「容器原本用的 ID」，只做重建之後那一步。

   ⛔ 不用不指名的 `up -d`：那會啟動切換前停著的服務（review 2026-11-26）。
   * **原本在跑**：`docker compose -f docker-compose.dev.yml up -d --no-build --no-deps` 後面逐一列出這些服務名稱；清單裡有 `python-worker` 時加 `--profile worker`。
   * **原本停著**：`docker compose -f docker-compose.dev.yml up --no-start --no-build --no-deps` 列出這些服務名稱，用一般設定重建、⛔ 不啟動
     （⛔ 不留下指向 `t089_accept` 的驗收設定容器；`python-worker` 在清單裡時同樣加 `--profile worker`）。
   * **原本沒有容器**：驗收期間建立的這些容器用 `docker compose -f docker-compose.dev.yml rm -f` 列出服務名稱刪掉（只刪容器、⛔ 不帶 `-v`）。

   核對：再跑一次同樣的 `ps -a --format json`，逐一服務比對跟切換前相同（在跑／停著／沒有容器）；**每個容器的 image ID 等於切換前那個容器實際用的 ID**、
   每個 `latest` 等於切換前記下的 ID；在跑的 backend、python-server、python-worker 連的是 `trading`（只看 database 名稱），backend 的 `REDIS_DB` 回到原本的值；
   `trading` 的 `backtest_jobs` 仍是雙零。⛔ 不相同就停下回報，⛔ 不自己補啟動或停止。
   驗收中途停下時（例如步驟 6 停了 python-server，或 build 之後任何一步失敗）也一樣從這一步開始，**標回 image 也要做**，⛔ 不需要先處理任何資料。
   核對通過之後才刪 `:pre-t089` 標籤。
2. **資料丟棄**（服務都已經不連 `t089_accept` 之後）：`DROP DATABASE t089_accept`；清掉驗收用的 Redis 編號（只清 DB 5：`redis-cli -n 5 FLUSHDB`，⛔ 不用 `FLUSHALL`）。
   這兩步要 dev 的 postgres、redis 在跑：**原本停著**或**原本沒有容器**的話，只為這一步用 `up -d --no-deps postgres redis` 暫時啟動，
   做完照它原本的狀態處理（停著的 `stop`、沒有容器的 `rm -f`），再跑一次第 1 步的核對。
3. **核對**：`t089_accept` 已不存在；`trading` 裡 X、Y 的最新日 K、SR 快照與計畫的最大 id 跟建立前記下的相同（驗收期間沒有東西寫進 `trading`）。
4. 驗收 override 與作業目錄的截圖、紀錄保留到 review 結束；override 裡⛔ 沒有密碼以外的機密，跟作業目錄一起刪。

**驗收中的兩項裁決**（2026-11-26 使用者採建議）：

* **交易日曆取不到時**：先回報並留證——用一檔已有日 K 的股票，保留 `calendar` = `unconfirmed`、畫面的「行情狀態無法確認」與 backend 的日曆錯誤紀錄；
  若是暫時故障，等 5 分鐘的降級期過後重試一次。仍然失敗，就把依賴日曆的判讀（「最新」「行情過期」「更新失敗」）標成待補驗，
  先完成入口、沒有投組、唯讀與復原的檢查。⛔ 不用手動設定「照常開市」代替（會讓那一天直接走 manual，也可能改變應有行情日）。
* **找不到判斷是「等待」的標的時**：先查 X、Y，再查 dev 的自選清單；仍然沒有，驗收紀錄寫「元件測試通過，dev 無實際樣本」。
  元件測試（`Positions.test.ts`「判斷『等待』與資料狀態分開顯示」）斷言同一張摘要卡裡 `plan-judgement` 顯示「等待」、`plan-status` 是另一個獨立的元素、
  也列出下一個觀察條件。合成日 K 維持原本的用途，⛔ 不調整走勢去做出「等待」。

**步驟**（每一步留截圖與畫面文字）：

1. 登入 → 觀察清單按 X 的「計畫」→「尚無計畫」→ 按「分析」→ 兩步內看到計畫（空手），主狀態「行情過期」（還差一天）。
2. **新日 K**：為 X 補 `settled_date` 那一根 → 「有較新的日 K」→ 按「分析」→ 行情日期變成 `settled_date`、SR 快照有重算、主狀態「最新」
   （13:30 之後的交易日另有「等待今日日 K」提示）。
3. **買進後、還沒重新分析**：新增一筆買進 → 「持倉已變動，請重新分析」、⛔ 不是最新 → 重新分析 → 持有的計畫、「最新」。
4. 找一檔判斷是「等待」的（可以是 X 或 Y），確認判斷跟資料狀態分開顯示、列出下一個觀察條件。
5. **行情過期**：Y 按「計畫」→ 分析 → 「行情過期：最新日 K 只到…（應有…）」，⛔ 不能顯示成「最新」。
6. **刷新失敗**：停掉驗收環境的 python-server → 在 X 按「**強制刷新 SR**」（一般「分析」在同一根日 K 時會重用快照、Python 停了也會成功，驗不到失敗）→
   「更新失敗」、舊計畫標成歷史；換一檔再換回來，標記仍在；啟動 python-server 後恢復。
7. 沒有投組的帳號（新註冊的驗收帳號）→ 從觀察清單按「計畫」→ 建立投組 → 回到原本那一檔。
8. 唯讀投組：「分析」按鈕停用並說明，歷史照樣可看。
9. 做上面的「復原與丟棄」，核對結果附在驗收紀錄。

## 七、完成後要歸檔的位置

* `docs/api-reference.md`：交易分析兩支端點的 `market_date`、`source_confirmed_1d`、`data_status`（含應有行情日的規則）。
* `docs/architecture.md`：交易分析的流程補「SR 快照只在同一根日 K 時重用」、畫面的資料狀態與「最新」的三個條件；同日改寫是已知限制。
* `docs/database-schema.md`：`position_analyses.analyzed_at` 是行情日期（複製自 SR 快照）、`created_at` 是分析紀錄時間、`position_version` 用來判斷持倉是否變動；
  038 的舊紀錄（`holding_sr_zone_v1_legacy`）來源無法確認；`evidence.decision_context` 的 `timeframe`、`market_data_at`。
* `docs/todo.md` T-089 改成「已實作／待 review」；review 確認後收斂，回看另外處理。

## 八、上版與實作前的確認

* 實作前請 live 專人唯讀盤點（指令另給）：
  * `POSITION_ANALYSIS_SR_REUSE_MAX_AGE_HOURS` 的實際值；
  * `position_analyses` 依 `rule_version` 分組的筆數，以及每組裡「`sr_zone_analysis_id` 是 NULL」「回查快照不存在」「快照 timeframe 不是 `1d`」「快照 `analyzed_at` 跟計畫不同」各幾筆。
* **盤點結果**（2026-11-25 10:31，live 專人執行）：
  * 重用期限：backend 的環境變數⛔ 沒有設定 `POSITION_ANALYSIS_SR_REUSE_MAX_AGE_HOURS`，用映像檔 `config.yaml` 的 `sr_reuse_max_age_hours: 24`。
    所以 live 目前**⛔ 沒有**「新日 K 入庫後仍重用舊快照」的情況（第二節）；D1 照做，讓這件事⛔ 不再依賴設定值。
  * `position_analyses` 共 **0 筆**（沒有任何計畫，也⛔ 沒有 038 的舊紀錄）。跟 T-081 的盤點（交易事件 0 筆）一致：交易分析在 live 還沒用過。
  * 影響：上版後 live 的計畫全部由新版產生，`source_confirmed_1d` 都走第 2 條（`evidence` 的 `timeframe`）；第 1、3、4 條的判定照做（dev 與日後的資料仍可能用到），
    ⛔ 不因為 live 目前沒有就拿掉。上版後的 live 確認從「尚無計畫」開始。
* 照一般部署規範上 live；⛔ 沒有 migration、⛔ 沒有設定鍵。上版前的回測雙零檢查照舊。
* 上版後在 live 用瀏覽器走一次：觀察清單按「計畫」→ 看到計畫或「尚無計畫」、行情日期與主狀態合理、判斷與資料狀態分開
  （⛔ 不在 live 補日 K、⛔ 不停服務、⛔ 不寫交易）。

## 九、裁決

* **Q1**（2026-11-25 採建議）：開頁只讀，分析由使用者觸發；唯讀投組沿用既有的停用規則。
* **Q2**（採建議並補充）：13:30 是等待當日日 K 的起點；另補應有行情日（D2）與查詢失敗的判定（`latest_candle` = `error`、`calendar` = `unconfirmed` → 行情狀態無法確認）。
* **Q3**（採建議）：首版只加摘要卡。
* **Q4**（採建議）：dev 用瀏覽器走完整流程並留畫面證據；較晚回應的時序由前端測試控制。

## 十、實作備註（2026-11-26，`t089` 分支、未合併）

**後端**

* B1 `tradingday`：`Result` 加 `OverrideUnavailable`（`json:"-"`）；讀手動設定失敗或逾時時是 true，判斷結果、`Source`、排程的降級⛔ 不變。
  日曆也取不到時是 fallback 加旗標。
* B2 `store`：`SRZoneRepo.GetSourceRefs(ids)` 只 SELECT `id, timeframe, analyzed_at`，一句 `IN`（`sqlx.In`＋`Rebind`，跟既有的 `CandleDatesInRange` 同一個寫法）；
  空的 ids ⛔ 不查詢。新型別 `SRZoneAnalysisSourceRef`。實作 `SRZoneRepo` 的兩個測試替身補上這個方法。
* B3 D1：`NewSRAnalysisProvider` 多一個 `LatestCandleReader`（`store.CandleRepo` 滿足它），`portfolio.NewAnalyzer` 跟著多一個參數；
  兩處呼叫端（`api/server.go`）傳 `candleRepo`。沒有注入（nil）時一律⛔ 不重用。既有的重用測試改成提供「最新 K 棒就是快照那一根」。
* B4：`decision_context` 加 `timeframe`、`market_data_at`。
* B5 D2：新檔 `api/handler/trade_plan_status.go`。
  * 共用判斷照 intraday 的做法，**請求時才**從 scheduler 取（handler 建立時 scheduler 可能還沒注入；開工前已說明）；沒有時用 `noJudge`，平日是 fallback → `calendar` = `unconfirmed`。
    建構函式收 `func() *tradingday.Judge`，nil 指標轉成 nil 介面（⛔ 不讓「裝著 nil 指標的介面」被當成有判斷）。
  * 計畫的回應用內嵌 `*store.PositionAnalysis` 的 struct，JSON 攤平、只多兩個欄位。
  * 回查失敗時只有「需要回查」的那幾筆當 false；evidence 帶 timeframe 的新紀錄照常判定（`source_lookup` = `error` 時畫面一律「行情狀態無法確認」）。
  * 持倉讀到 `(nil, nil)` 也當 0（測試替身的行為；正式的 repo 是 `sql.ErrNoRows`）。
* gofmt：只格式化這次新增的檔；既有檔案裡原本就沒格式化的部分（例如 `store/model.go` 的其他 struct）⛔ 沒有動。

**前端**

* F1：新型別 `TradePlan`（`PositionAnalysis` 加兩個欄位）、`TradeDataStatus`；`listTradeAnalyses` 改回 `{ analyses, data_status }`。
* F2：`lib/utils/tradePlanStatus.ts`：主狀態表、兩個提示、`pickSummaryPlan`。時間用 `formatToParts` 自己組字串（locale 的 `format()` 會插入不換行空白，畫面與測試都不穩定）。
* F3：觀察清單每列在代號旁加「計畫」按鈕（一直顯示，⛔ 不放在 hover 才出現的操作欄）。
* F4：`Positions.svelte` 重寫 script 與上半部；既有的細節區塊（原「最新分析」，改名「計畫細節」）逐字保留。
  * **錯誤分兩種**：載入錯誤（重新載入成功就清掉）與動作錯誤（分析、買賣、更正；同一檔下一次動作開始時才清掉）。
    第一版只有一種，回到 A 重新載入時把 A 的「更正失敗」清掉了，由前端測試抓到後改正。
  * 資料不足時摘要卡與細節區塊都⛔ 不顯示判斷；使用者從歷史點選時細節區塊照樣顯示那一筆。
* `vite build` 會換掉 `backend/internal/ui/dist` 的 bundle 檔名，commit 時要一起 `git add backend/internal/ui/dist`。

**測試與反證**（2026-11-26）

* 後端：`backend/scripts/test.sh` 全部套件 rc 0（16 個套件）。新測試：D1 四組、analyzer 的 evidence 鍵、`GetSourceRefs`（SQLite）、
  `OverrideUnavailable` 六種情境與 JSON、handler 的來源判定（八筆）、回查失敗、`latest_candle` 三種加未注入、應有行情日十二種（含連假、跨年、
  fallback、讀不到手動設定、15 天找不到、沒有共用判斷）、`awaiting_today` 四種、`position_version` 三種、Analyze 的回應。
  **反證五項都失敗**：拿掉同一根日 K 條件、`no_data` 與 `error` 合併、拿掉 `rule_version` 判定、`calendar` 只看 fallback、B1 不設旗標。
* 前端：`frontend/scripts/test.sh` svelte-check 0 錯誤 0 警告、vitest 29 個檔 258 項全過、build 成功。新測試：`tradePlanStatus` 33 項、
  `WatchlistTable` 2 項、`Positions` 17 項（入口、404、`latest_candle` = `error`、唯讀投組、載入鏈三項、分析三項、寫入三項、切換投組三項）。
  **反證六項都失敗**：拿掉選擇比對、拿掉序號比對、`busy` 改回整頁共用、分析回來用當下的選擇重新載入、寫入回來不比對選擇、錯誤不依鍵。
* `GetSourceRefs` 的 `IN` 只在 SQLite 上測；PostgreSQL 上同樣寫法的 `CandleDatesInRange` 已在 live 使用，dev 實跑驗收時會經過 PostgreSQL。

**實作 review（2026-11-26）的修正**

* [P1] 買賣或更正寫入成功、重新載入失敗時，摘要仍拿寫入前的持倉版本比對而標「最新」。改成：寫入成功時在那一檔記一個序號，
  `tradePlanStatus` 多一個輸入 `positionWrittenSinceLoad`，有它就是「持倉已變動」；**寫入之後才開始**的載入成功時才清掉（寫入前就在飛的載入⛔ 不算）。
* [P2] 從投組 1 的 2330 切到投組 2，持倉清單還在載入時選 2454，清單一回來就跳回 2330。改成：投組切換立即確定選擇並開始載入，跟持倉清單並行；
  持倉清單也帶投組與序號，晚回來的舊清單⛔ 不蓋掉新的；開頁時從觀察清單帶入的代號，在使用者已經自己選了別檔時⛔ 不覆蓋。建立投組後回到原本那一檔也用同一個做法。
* [P2] dev 復原：改用 `ps -a` 記三份清單、逐一指名復原（第六節）；指令在 dev 上唯讀試過，確認列得出已停止的容器與 profile 服務。
* 測試：`Positions` 加 5 項（買進、更正各一項的「寫入成功、重新載入失敗」，投組切換三項），`tradePlanStatus` 加 2 項；
  五項新反證都失敗（買進、更正成功不標待確認、狀態不看待確認、投組切換改回先等持倉清單、持倉清單不比對序號）。

**實作 review 第二輪（2026-11-26）**：前輪三項複審通過。[P3] 換投組時，新投組的個股持倉已經載入、持倉清單還沒回來的期間，
頁尾「現有 LONG」仍顯示舊投組的股票。改成：持倉清單記住屬於哪一個投組，換投組時立即清空（投組切換與建立投組都經過同一個地方）。
`Positions` 加 1 項，反證（拿掉換投組時清空）失敗。

### dev 實跑驗收紀錄（2026-11-26 16:50～17:15，`t089` 分支 `b7cb1a3`）

**結論**：步驟 1～3、5、6、8 照預期通過；步驟 4、7 依使用者裁決由元件測試補足（理由見下表）；步驟 9 的復原與丟棄全部核對一致。

**環境**：
* 瀏覽器：Playwright 的 `mcr.microsoft.com/playwright/python:v1.48.0-jammy`（這個 image 沒有 Python 的 `playwright` 套件，另外 build 一個本機用的 `t089-pw:local`，只 `pip install playwright==1.48.0`）。
  容器接 `stock_trading_dev_net`、打 `http://backend:8080`；每一步注入 JWT 與投組選擇、從首頁觀察清單按「計畫」進入；截圖與畫面文字在 repo 外的 `~/t089_accept/shots/`（29 張，保留到 review 結束）。
* 交易日曆取得成功（`calendar` = `confirmed`）。dev 的手動設定留著 I-139 dev 驗收時的 **11-25 休市**，照裁決⛔ 沒有改，所以當時（11-26 16:55，交易日、13:30 之後）
  `expected_date` = 11-26、`settled_date` = 11-24、`awaiting_today` = true。
* **機率模型**：dev 的 `python/models` 是空的，分析一律 503「機率模型尚未訓練」（畫面正確地當成「更新失敗」類、⛔ 不是資料不足）。
  改由 live 專人從 python-server 容器複製 `sr_scoring_v4.joblib` 交給 dev（sha256 `2a91fd8d…`，三處一致），驗收 override 唯讀掛進 python-server。
  這顆**跟 repo 的 I-074 baseline（`python/baselines/b1_20260901_…/model.joblib`）是同一個檔**。
* **標的**：dev 的日 K 超過 250 根的只有 2330（最後 11-12）；自選清單的 0050 完全沒有日 K。依使用者同意，Y 也用 2330，放在另一個投組、在補合成日 K **之前**先做步驟 5；
  0050 加驗「資料不足」。X（2330，專用投組）補合成日 K 11-13～11-23（7 根），步驟 2 再補 11-24。

| 步驟 | 結果 |
|---|---|
| 5 行情過期（Y） | 開頁「尚無計畫：最新日 K 只到 11-12（應有 11-24），分析結果會是過期的行情」＋「今日日 K 尚未入庫」提示；分析後「行情過期：最新日 K 只到 11-12（應有 11-24）」，⛔ 不是最新；判斷「避開」另外顯示；歷史列有行情 11-12 |
| 加驗：資料不足（0050） | 開頁就是「資料不足」（`latest_candle` = `no_data`）；分析回 404 後仍是「資料不足」，⛔ 不顯示判斷 |
| 1 兩步內看到計畫 | 按「計畫」→「尚無計畫：最新日 K 只到 11-23（應有 11-24）」→ 按「分析」→「行情過期」（差一天） |
| 2 新日 K | 補 11-24 後「用 11-23 的日 K · 已有 11-24 的日 K，請重新分析」；重新分析 SR 快照重算（新 id，11-24），主狀態「最新」（等待今日日 K 時用 `settled_date`）＋提示 |
| 3 買進後、還沒重新分析 | 買進 1000 股後立刻「持倉已變動（買賣或更正後），請重新分析」；重新分析得到 LONG、持倉版本 1 的計畫（「減碼」），回到「最新」 |
| 4 判斷「等待」 | dev 沒有實際樣本（2330 是「避開」與「減碼」，0050 沒有日 K）；依裁決記「元件測試通過，dev 無實際樣本」。截圖仍可看到判斷是摘要卡裡獨立的元素、旁邊列出下一個觀察條件 |
| 6 刷新失敗 | 停驗收的 python-server、按「強制刷新 SR」→「更新失敗（17:05）：顯示的是 11-24 的歷史計畫」，錯誤只在 2330；切到 0050 再切回，標記仍在；啟動 python-server 後重新分析回到「最新」（同一個瀏覽器分頁） |
| 7 沒有投組 | **前提不成立**：註冊時後端就自動建立「Personal Portfolio」（`ensureDefaultUserPortfolio`），正常流程碰不到「沒有投組」。新帳號從觀察清單按「計畫」直接載入 2330（尚無計畫）。依裁決記「正常流程碰不到；畫面路徑由元件測試（沒有投組 → 建立 → 自動載入原本那一檔）涵蓋」 |
| 8 唯讀投組 | 群組投組（驗收帳號是 VIEWER）：投組選單標 READ、上方說明「目前 Portfolio 為唯讀…歷史照樣可看」、「分析」停用；API 上 OWNER 分析 201、VIEWER 403；歷史照樣顯示 OWNER 的那一筆 |

**順帶確認**：
* D1：補日 K 之後，舊快照（11-12、11-23）都⛔ 沒有被重用。步驟 3 又重算一份，⛔ 沒有重用 11-24 那份——它是最新那根日 K，但 `analyzed_at`（11-24 00:00）距今超過 24 小時，被原本的年齡條件排除；跟設計一致（D1 只會讓重用變少）。
* 帳號啟用、補日 K 的每一次寫入前都查 `current_database()` 是 `t089_accept`，寫入後核對筆數；`trading` 裡⛔ 沒有驗收帳號。

**步驟 9：復原與丟棄**（照第六節 v1.8）：
* 切換前：三份清單——停著 `postgres`、`python-server`、`redis`、`backend`；沒有容器 `python-worker`；另記 `fake-yahoo`（exited）。
  image：backend `3a4acd5d67de`、python-server `d37c5a8d92dd`（容器實際用的與 `latest` 相同）、python-worker 只有 `latest` `b51cf6f2cd28`；打 `:pre-t089`。
  `trading` 回測雙零；Redis DB 5 是空的；驗收 backend 啟動後的有效設定 `REDIS_DB=5`、應用服務的連線全部在 `t089_accept`。
* 收尾：停驗收服務 → 三個 `latest` 標回原本的 ID → 在主 repo 目錄用一般設定 `up --no-start --no-build --no-deps` 重建停著的四個服務
  （backend、python-server 重建後是 `created`，切換前是 `exited`，同屬「停著」）→ 暫時啟動 postgres、redis：`DROP DATABASE t089_accept`、`redis-cli -n 5 FLUSHDB` → 停回去。
  這次每個服務的「容器原本用的 ID」與「`latest` 原本指向的 ID」都相同，所以先標 `latest` 再重建的結果正確；兩者不同時的順序已改正在第六節（v1.10：重建前 `latest` 指向 `:pre-t089`、重建後才標回原本的 ID）。
* 清理的查詢輸出（2026-11-26 17:13，當時終端機逐字；另存在 `~/t089_accept/cleanup_output.txt`）：

  ```
  conns_on_accept|0          ← DROP 之前，t089_accept 上沒有連線
  dropped
  exists|0                   ← DROP 之後查 pg_database：t089_accept 不存在
  db5 before flush: 0        ← redis-cli -n 5 DBSIZE
  OK                         ← redis-cli -n 5 FLUSHDB
  db5 after: 0               ← redis-cli -n 5 DBSIZE
  # Keyspace
  db0:keys=1,expires=0,avg_ttl=0,subexpiry=0   ← 只有 db0（切換前就是這 1 個 key），⛔ 沒有 db5
  baseline-identical         ← trading 的基準檔前後逐字相同
  backtests|0|0
  ```
* 核對：`ps -a` 逐一服務的類別、容器實際用的 image ID、`latest` 的 ID 全部跟切換前相同；`python-worker` 仍然沒有容器、`fake-yahoo` 照舊；
  backend 的 DSN 回到 `trading`、⛔ 沒有 `REDIS_DB`；`trading` 的基準（X、Y 的日 K、SR 快照／計畫／日 K／使用者的最大 id、goose 版本）逐字相同；回測雙零。核對通過後刪掉 `:pre-t089`。
* 還留著：`~/t089_accept/`（截圖、紀錄、override、模型副本），保留到 review 結束再刪；live `app` 家目錄裡的模型副本請專人刪除；
  驗收用的兩個 Playwright image（約 2.8 GB）留著供之後的驗收用，不需要時再刪。

## 版本紀錄

| 版本 | 日期 | 內容 |
|---|---|---|
| v1 | 2026-11-25 | 初版：範圍照 T-089（回看另外處理）；讀碼確認行情日期已存在 `position_analyses.analyzed_at`、重用條件改成同一根日 K；四項待裁決 |
| v1.1 | 2026-11-25 | 併入 v1 review：[P1] 「最新」改看當下應有的行情日（`expected_date`／`settled_date`，由共用休市判斷往回推），日 K 停更時標「行情過期」，查詢失敗與查無 K 棒分開（`latest_candle` = `error` → 行情狀態無法確認）；[P1] 核對持倉版本（`position_version` 不同 → 持倉已變動），驗收加「買進後、還沒重新分析」；[P2] 來源確認改成 `rule_version` ＋ `1d`（新紀錄寫進 `evidence`，舊紀錄回查快照），⛔ 不收窄 API，live 盤點改依 `rule_version` 與來源分組；[P2] 請求鏈從開始固定選擇，驗收加分析回應較晚回來（成功／失敗），`busy` 與錯誤依鍵記錄；Q1～Q4 依建議裁決 |
| v1.2 | 2026-11-25 | 併入 v1.1 review：[P1] 讀手動設定失敗或逾時也算 `calendar` = `unconfirmed`（共用判斷加 `json:"-"` 的 `OverrideUnavailable`，排程行為⛔ 不變）；[P1] 「最新」要求 `market_date` 等於 `latest_candle_date`，補「日 K 已變動」（計畫比庫內新）、「尚無計畫」與兜底的「無法確認」；[P2] D5 套用到買賣與更正（選擇、序號、表單、錯誤），補寫入回應晚到的案例；[P2] dev 驗收先準備新鮮檔與過期檔（合成日 K、驗收後刪除），刷新失敗改按「強制刷新 SR」 |
| v1.3 | 2026-11-25 | v1.2 複審通過；依複審補 dev 驗收收尾：專用帳號與投組、開始前記基準、驗收中記計畫與 SR 快照 id，成功或中途失敗都依 id 清 SR 快照（含驗收期間排程產生的）、計畫、合成日 K 與衍生的指標與訊號，並核對所有指向快照的表；避開會動到日 K 的排程時段 |
| v1.3（live 盤點） | 2026-11-25 | 第八節記 live 唯讀盤點：重用期限用映像檔的 24 小時（環境變數沒有設定）；`position_analyses` 0 筆；設計⛔ 不變 |
| v1.4 | 2026-11-25 | 併入 v1.3 review（功能設計可收斂）：dev 驗收改在可丟棄的 database `t089_accept`（範本複製 dev 的 `trading`，驗收 override 改 DSN 與 Redis 編號）上做，結束後整個丟掉，⛔ 不再逐筆刪快照（重用的快照 id 會選到驗收前的資料、排程的身分追蹤改過的既有狀態復原不了）；服務復原與資料丟棄分開、前者先做且⛔ 不依賴後者 |
| v1.5 | 2026-11-25 | 併入 v1.4 review：驗收帳號的啟用與補合成日 K 的 SQL 都明確指向 `t089_accept`、寫入前查 `current_database()`（共用的 `dev_token` 固定寫 `trading`，⛔ 不改它，驗收用換了 `-d` 的版本）；切換前做回測雙零檢查並記下服務狀態，復原照切換前的狀態（`python-worker` 屬於 `worker` profile，有在跑才用 `--profile worker` 帶回） |
| v1.6 | 2026-11-26 | v1.5 複審通過；在 `t089` 分支實作 B1～B6、F1～F5（第十節）；共用判斷改成請求時從 scheduler 取（開工前說明）；錯誤拆成載入與動作兩種；後端反證五項、前端反證六項都失敗；dev 實跑驗收未做 |
| v1.10 | 2026-11-26 | 併入驗收 review（驗收通過、同意步驟 4、7 以元件測試補驗）：[1] image 復原順序改正——重建前 `latest` 先指向 `:pre-t089`（容器原本的 image）、重建後才標回 `latest` 原本的 ID（兩者不同時，舊順序會讓容器用到原標籤的 ID）；這次兩者相同，結果正確；[2] 步驟 9 補清理的查詢輸出（`t089_accept` 不存在、DB 5 `DBSIZE` 0） |
| v1.9 | 2026-11-26 | dev 實跑驗收完成（紀錄在第十節）：步驟 1～3、5、6、8 通過；Y 用 2330 的另一個投組、加驗 0050 資料不足；機率模型由 live 交接（跟 repo baseline 同一個檔）；步驟 4、7 依裁決由元件測試補足（步驟 7 的前提不成立：註冊時自動建立 Personal Portfolio）；補「等待」的元件斷言；復原與丟棄全部核對一致 |
| v1.8 | 2026-11-26 | dev 驗收的具體化（使用者同意三項）：image 回復點——build 前記下容器實際用的 image ID（打 `:pre-t089`）與預設標籤原本指向的 ID，復原時先停驗收服務、標回 `latest`、`--no-build` 依原狀態重建、比對 image ID，中途失敗也照做；`python-worker` ⛔ 不啟動（開跑前與驗收後確認沒有容器）；Redis 用 DB 5（開跑前確認是空的、驗收 backend 的有效設定是 5、結束只清 DB 5） |
| v1.7（第二輪） | 2026-11-26 | 前輪三項複審通過；[P3] 換投組時頁尾「現有 LONG」立即清空，⛔ 不暫時顯示舊投組的股票 |
| v1.7 | 2026-11-26 | 併入實作 review：[P1] 買賣或更正寫入成功後、重新載入成功之前，摘要⛔ 不能是「最新」（標持倉已變動）；[P2] 投組切換立即確定選擇、持倉清單帶投組與序號，晚到的回應⛔ 不跳回原本那一檔；[P2] dev 復原改用 `ps -a` 記三份清單（在跑／停著／沒有容器），逐一指名復原，⛔ 不用不指名的 `up -d` |
