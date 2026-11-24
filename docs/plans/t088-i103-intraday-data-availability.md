# T-088＋I-103 計畫書 v1.5：盤中逐檔行情可用性（逐檔抓取與寫入結果、能否評估、畫面顯示、矯正手段）

> 狀態：v1.5（✅ 2026-11-20 使用者確認）；**已實作、未 commit，待 review**（實作備註與偏差見第十節；v1、v1.2、v1.3、v1.4 的 review 已併入；2026-11-20 使用者裁決：上版採第九節的 C、D6 沿用現有權限）。
> 對應：`docs/todo.md` T-088、`docs/issue.md` I-103。
> 需求確認：2026-11-20（使用者：T-088 與 I-103 合併成一份計畫書，先於 T-084／T-087 進行）。

## 一、目標與不做的範圍

**目標**

1. **逐檔的抓取與寫入結果（I-103）**：Yahoo 批次路徑每一輪、每一檔都要分得出「寫入成功」「回應裡沒有這檔的 K 棒」「這檔寫入失敗」「整批抓取失敗」，
   並讓「整批失敗」與「逐檔寫入失敗」在 `job_runs` **分開**可辨識（I-103 的關閉條件）。
2. **能不能評估由這一輪的資料決定（T-088）**：抓取失敗、寫入失敗、沒有資料的標的⛔ 不評估；引擎實際讀到的 K 棒跟上一次**成功評估**時完全相同才跳過（D1）。
   不評估的記成「本輪未評估」與原因，⛔ 不再跟「評估過、未觸發」混在一起。
3. **系統自己的判斷依據**（使用者 2026-11-20 的原則：⛔ 不全面依靠第三方資訊決定系統是否正常）：每一檔記下最後一根有效 K 棒的時間與落後程度，
   跟抓取、寫入、評估的結果**分開記**；「Yahoo 有回應」⛔ 不等於「資料可用」。
4. **畫面**：自選清單每一檔分開顯示**行情狀態**與**評估結果**，例如「09:15 行情更新失敗（伺服器錯誤 5xx）｜最後有效 K 棒 09:10｜本輪未評估」。
5. **矯正手段**（使用者原則：保留第三方資料異常的矯正手段）：下一輪自動補回要看得見；另有手動補抓，結果要能說明補了什麼、還缺什麼。
6. **可以逐輪對帳**：每一輪、每一檔的結果都留下來（保留期跟 `job_runs` 相同），I-123 的分段判讀（第九節）能逐輪重算。

**不做的範圍**

* 訊號邏輯本身（突破、跌破、爆量的條件、cooldown、判重的規則）⛔ 不改；訊號引擎只加「不送出」的選項與回報判重結果（3.3）。
* 日 K 路徑（`pre_market`、`daily_close`）⛔ 不改；日 K 交易分析的資料日期顯示屬於功能檢討「可用首版」第 2 項，另案。
* 備援資料源（Yahoo→FinMind fallback，`todo.md` T-031、T-008）⛔ 不做。
* 「事後補判」（空窗期間的時點回頭評估）⛔ 不在第一版：訊號引擎只看最新一根，補判要另外設計。
* `signal:queue` 沒有 consumer 是既有狀態（`architecture.md` 已記），⛔ 不處理。

## 二、現況（2026-11-20 讀碼）

* **抓取**：`FetchAndStoreIntradayBatch` 回傳 `(stored int, err error)`。整批請求失敗才回 error；回應裡沒有某一檔（或全是 null 棒）就跳過、
  某一檔 `BulkInsert` 失敗只記 log——兩者都只反映在 `stored` 少了，看不出是哪一檔、也分不開（I-103）。
  log `fetched intraday batch candles` 只有 `requested`／`stored`。
* **排程**：`runIntradayBatch` 先逐批抓取（暫時性失敗在同一輪內重試，層二；重試預算 120 秒只約束重試階段），整批最後仍失敗時 `tally.addOpaqueFetchFailures(len(batch))`
  ——只記筆數、不記是哪幾檔，雖然這一批的代號其實是知道的。**接著不論抓取結果，所有標的都跑 `evaluateSymbol(…, "1m")`**，用 DB 裡既有的 K 棒評估。
* **`job_runs` 的摘要**：`fetch_failed:N (代號:原因, …); evaluate_failed:…; degraded:…`；`symbols_failed` 是失敗標的的聯集（opaque 時取下界）。
* **FinMind 逐檔路徑**（live 沒有啟用）：`FetchAndStoreMinute` 回應是空的也回成功，接著照樣用舊 K 棒評估。
* **評估讀的 K 棒**：指標讀最近 120 根（`indicator` 的 `lookback`），訊號引擎讀最近 100 根；其他輸入（籌碼分數）是日資料，盤中⛔ 不會變。
* **訊號引擎**：沒有觸發時直接返回，⛔ 沒有逐檔寫入任何狀態；被判重擋下時也只記 debug log。所以「抓取失敗、用舊 K 棒評估、沒有訊號」「評估失敗」
  「觸發了但被判重擋下」在外面看起來都跟「有新資料、評估後未觸發」一樣。
  * 訊號的時間戳是最新一根 K 棒的時間；判重看「同一個訊號身分、時間差小於 15 分鐘」。所以抓取失敗時用**同一批舊 K 棒**重評，
    產生的會是同一個時間戳的訊號，通常被判重擋下。⚠️ 但上一輪如果沒有評估完成（評估失敗、重啟、判重本身失敗），舊 K 棒仍可能產生並推送訊號；
    逐檔寫入失敗時那一檔也是用舊 K 棒評估。
* **寫入是 upsert**：同一個時間戳的 K 棒會被後來的回應更新（收盤價、成交量會變，`intraday_refill_test.go` 有測），較早缺的棒也會被補回，較早的棒也可能被修訂。
* **補回**：Yahoo 批次端點每次回傳**當日全部**的 1 分 K，所以某一輪失敗時缺的棒，下一輪成功時會補回（`yahoo-intraday-integration.md`）。
  ⚠️ 13:30 是最後一輪，那一輪失敗就沒有下一輪；補回的是 K 棒，空窗期間**沒有被評估的時點⛔ 不會回頭評估**。
* **兩輪可能重疊**：重試最多讓一輪晚約 120 秒，⛔ 不保證兩輪⛔ 不重疊（`yahoo-intraday-integration.md`）。較舊的請求晚回來時，會用較舊的值覆寫同一分鐘的 K 棒。
* **畫面**：Dashboard 的自選清單用日 K 與日 K 指標填值，盤中只在產生訊號時經 WebSocket 推播；⛔ 沒有任何盤中資料新鮮度的顯示。
* **權限**：使用者帳號沒有角色區分，現有的手動排程端點（`/scheduler/*/run`）任何登入的使用者都能觸發（功能檢討「共用環境與權限」也提到）。
* **觀察工具的相依**：I-123 的觀察工具（repo 外）依 log 訊息名稱過濾：`intraday batch fetch failed`、`intraday batch fetch succeeded after retry`、
  `intraday batch retry failed`、`fetched intraday batch candles` 等；intraday 的輪數與最終失敗輪數由 `job_runs` 的狀態計數（含 `skipped` 一欄）。
* 2026-11-18 的實例：09:15、09:45 兩輪 Yahoo 回 HTTP 500、重試用完，那兩輪仍以舊 K 棒評估全部 11 檔；下一輪 11 檔都恢復寫入。
  每天 09:00 那一輪都是 `requested 11`、`stored 0`（開盤第一輪還沒有分 K）。

## 三、設計

### 3.1 逐檔結果與 `job_runs`（I-103）

* `FetchAndStoreIntradayBatch` 改回傳逐檔結果（新型別，例如每一檔一筆 `SymbolResult{Symbol, Data, LastBarTS, Err}`），`err` 仍只代表整批請求失敗：
  * `written`：回應裡有這檔的 K 棒、`BulkInsert` 成功；`LastBarTS` 是回應裡最後一根的時間。
  * `no_data`：回應裡沒有這檔（或全是 null 棒、或 key 對不上）。⛔ 不是失敗（開盤第一輪、停牌、當下無成交都會發生；比照 `candle_gap_detection`
    把「沒有資料」與「失敗」分開）。
  * `store_failed`：`BulkInsert` 失敗，`Err` 留給呼叫端分類（原文只進 log）。
* 整批失敗時，這一批的每一檔都標 `fetch_failed`（代號已知），⛔ 不再只記筆數。
* **重試要把逐檔結果一路傳回排程**（v1.4）：現在的 `retryIntradayBatch` 只回傳布林值，會丟掉逐檔結果。改成回傳**最後一次嘗試的逐檔結果**與嘗試次數：
  * 例：第一次 HTTP 500、重試得到 200 之後，A 寫入成功、B 寫入失敗、C 沒有資料 → 三檔分別記 `written`／`store_failed`／`no_data`，⛔ 不再整批當成成功或失敗。
  * 逐輪紀錄另外記這一批的 `fetch_attempts`（1～3）與 `http_recovered`（第一次失敗、後來 HTTP 成功），所以「HTTP 救回」跟「寫入成功」分得開
    ——救回的那一批裡仍可能有寫入失敗的檔。
  * **重試預算到期時，依實際停在哪個階段分類**（v1.5 改）：
    * 已經 `BulkInsert` 提交的檔 → `written`（K 棒確實在 DB 裡，可以評估）；
    * HTTP 200 已經拿到這檔的 K 棒、卻在**寫入階段**逾時（還沒寫或寫到一半被取消）→ `store_failed`，原因碼 `retry_budget`；
    * 回應已確認沒有這檔 → `no_data`（維持）；
    * HTTP 回應本身沒拿到（請求或節流等待逾時）→ 這一批還沒分類的檔 `fetch_failed`，原因碼 `retry_budget`。
    ⚠️ 以前這種情況整批計入失敗，改成逐檔照實記。
  * 層二的 log（`intraday batch fetch succeeded after retry`、`intraday batch retry failed`、`intraday batch retry stopped: retry budget exhausted`、
    `intraday batch fetch failed`）的觸發條件與欄位⛔ 不變，只新增逐檔的計數欄位。
* **`job_runs` 分開記抓取與寫入失敗**：
  * tally 新增 `addStoreFailure(symbol, err)`，跟 `addFetchFailure` 分開存；摘要沿用現有格式、新增一類：
    `fetch_failed:N (…); store_failed:M (…); evaluate_failed:…; degraded:…`。
  * `symbols_failed` ＝ fetch、store、evaluate 三類失敗標的的**聯集**；`no_data` ⛔ 不記失敗。
  * 狀態推導不變：全部失敗 `failed`、部分失敗 `partial`。⚠️ 以前看不到的逐檔寫入失敗會讓 success 變成 partial，全部寫入失敗會變成 failed——這是 I-103 要的，
    但**會改變 I-123 觀察的計數口徑**（第九節）。
* log：既有的訊息名稱與欄位⛔ 不改（觀察工具在用），只**新增**欄位與訊息：`fetched intraday batch candles` 加 `no_data`、`store_failed` 的數量；
  每一檔寫入失敗記 `intraday batch store failed`（既有）加 `run_id` 與安全原因碼。
* **FinMind 逐檔路徑（D4）**：`FetchAndStoreMinute` 回傳這一檔寫入的根數；抓取失敗記 fetch、寫入失敗記 store、空回應記 `no_data`，三者都⛔ 不評估。

### 3.2 能不能評估（T-088；D1、D2）

**輸入指紋**（v1.3 改）：寫入之後，從 DB 讀**評估實際會用到的 K 棒**——這一檔最近 120 根 1 分 K（指標的 120 根涵蓋訊號引擎的 100 根），
依時間排序，對每一根的時間與 OHLCV 取 SHA-256。同時間戳的棒被更新、較早的棒被補回或**被修訂**（例如某一根的高低價或收盤價改了），指紋都會變。
⚠️ 窗口大小跟著指標的 `lookback` 走（同一個常數或共用的取值，⛔ 不各寫一份）。

在 `evaluateSymbol(…, "1m")` **之前**，依這一輪的逐檔結果決定：

| 這一輪的資料 | 評估 |
|---|---|
| `written`，指紋跟**上一次成功評估**記下的不同（或沒有成功評估的紀錄） | ✅ 評估 |
| `written`，指紋跟上一次成功評估**相同** | ⛔ 不評估（`unchanged`）：引擎讀到的 K 棒完全相同，結果也相同 |
| `no_data`、`fetch_failed`、`store_failed` | ⛔ 不評估 |

* 「上一次成功評估」＝這一檔最近一筆評估成功（`EvaluateWithResult` 沒有回 error）的紀錄（3.3 的表）。評估失敗、手動補抓、中斷的輪次⛔ 不算，
  所以下一輪取得資料就會重試。
* 同一輪其他寫入成功的標的照常評估。
* ⛔ 不評估的標的不呼叫訊號引擎，所以⛔ 不會從舊 K 棒產生或推送訊號，也⛔ 不會被當成「未觸發」。
* **延遲（D2）**：`lag` ＝ 這一輪的時刻 − 最後一根有效 K 棒的時間，**跟抓取、寫入、評估的結果分開記**。
  * 首版門檻 **10 分鐘**（兩輪），超過就標「行情延遲」。抓取失敗、盤中停止更新時，延遲照樣隨時間增加、照樣顯示（API 依當下時間算）。
  * 開盤第一輪（09:00）還沒有分 K 是 `no_data`，延遲從第一根出現之後才算。
  * 收到比上一次成功評估新、但仍落後超過 10 分鐘的 K 棒：**照常計算，但⛔ 不當成即時訊號送出**（3.3 的不送出選項），評估結果記 `withheld_stale`。
    資料追上之後，條件仍成立就會在那一輪正常送出。

### 3.3 每一輪、每一檔的紀錄（v1.3 改：逐輪保存，取代「只存最新一輪」）

* 新表 `intraday_symbol_runs`（migration 079，三份 DB 都加），主鍵 `(run_id, symbol)`，**每一輪、每一檔一列**；`run_id` 是 `job_runs` 的 id，
  所以逐輪都對得起來，也⛔ 沒有「較舊的一輪蓋掉較新狀態」的問題。
* 新表 `intraday_rounds`（同一個 migration 079），一輪一列、主鍵 `run_id`（v1.4）：`source`（`scheduled`／`refill`）、`scheduled_at`（排定的時點）、
  `lock_acquired_at`（實際開始執行）、`exec_finished_at`、`lock_wait_ms`、`outcome`（`executed`／`skipped_lock_timeout`）、`evidence_failures`（逐輪存證寫失敗的次數）。
  **排定的時點與實際執行區間分開記**（3.5、第九節的重疊判定用）。
* 欄位分兩段：
  * **行情段**（抓取、寫入之後 insert）：`source`（`scheduled`／`refill`）、`data_at`、`data_status`（`written`／`no_data`／`fetch_failed`／`store_failed`）、
    `data_cause`（安全原因碼，可為 NULL）、`last_bar_ts`、`fingerprint`。
  * **評估段**（那一檔評估完成之後 update；補抓的列永遠是 NULL）：`eval_at`、`eval_status`、`eval_cause`。
* `eval_status` 的值域：

  | 值 | 意思 |
  |---|---|
  | `no_signal` | 評估完成，沒有觸發 |
  | `signal_sent` | 觸發並送出（寫入 DB、推播） |
  | `signal_suppressed` | 觸發了，但被判重擋下（DB 判重或 reservation） |
  | `withheld_stale` | 觸發了，但資料延遲，⛔ 不當即時訊號送出 |
  | `degraded` | 評估完成但有階段降級（照現有的 degraded 定義） |
  | `evaluate_failed` | 評估失敗（例如指標計算失敗） |
  | `not_evaluated` | 這一輪⛔ 不評估；原因在 `eval_cause`：`fetch_failed`、`store_failed`、`no_data`、`unchanged` |

  「評估成功」＝ `no_signal`、`signal_sent`、`signal_suppressed`、`withheld_stale`、`degraded`；這些列的 `fingerprint` 就是下一輪比對的「上一次成功評估」。
* 訊號引擎要多回報兩件事：**被判重擋下**（`EvaluateResult` 加一個欄位，⛔ 不改判重規則）；**不送出的選項**
  （`withheld_stale` 用：照常算到判重為止，⛔ 不 reserve、⛔ 不寫 `signals`、⛔ 不推播）。
* **⛔ 不放 Redis**：Redis 停用時（2026-11-13 上午就發生過）紀錄也會跟著消失。
* **保留期**跟 `job_runs` 相同，在開盤前清理 `job_runs` 的同一步一起刪（量：11 檔 × 55 輪 ≈ 600 列／天）。
* **存證本身寫失敗時**（v1.4）：
  * **建立 run 失敗**（`startRun` 回 0；現在的行為是照跑、只是沒有紀錄）：這一輪照樣抓取、照 3.2 判定與評估（⛔ 不因為存證失敗就停掉盤中訊號），
    但寫不了逐輪紀錄；記 log `intraday evidence unavailable: run not created`（帶排定時點）。對 I-123 而言，那一輪在 `job_runs` 不存在，就是**資料不足**（當天不到 55 輪）。
  * **逐檔的行情段或評估段寫失敗**：記 log `intraday evidence write failed`（`run_id`、`symbol`、哪一段、安全原因碼），計入那一輪的 `evidence_failures`，
    並在 `job_runs` 摘要加 `evidence_failed:N`、該輪至少記成 partial——⛔ 不讓「整輪 success、卻缺逐檔證據」出現。
    `intraday_rounds` 本身也寫不進去時，`job_runs` 的摘要仍然帶得到（兩者分開寫）。
  * **失敗的方向是「多評估」**：評估段沒寫進去，下一輪查不到這次的成功評估 → 會再評估一次；同一個訊號由判重擋下（`signal_suppressed`），⛔ 不會少評估。
  * **對帳不完整的判定**：`evidence_failures > 0`、或 `job_runs` 摘要有 `evidence_failed` 的輪次，標「對帳不完整」，逐輪對帳時另外列出、⛔ 不當成一致。
* **逐輪對帳**：每一輪的 `fetch_failed`、`store_failed` 檔數、可評估與未評估的檔數，都能從這張表重算，並且要等於那一輪 `job_runs` 的摘要與 `symbols_failed`
  （測試鎖住；I-123 的封存只要多帶這張表）。

### 3.4 API 與畫面

* `GET /api/v1/intraday/status`：回傳**最近一輪排程的輪次紀錄**（`intraday_rounds`：排定時點、結果、`run_id`），以及自選清單每一檔的
  **最近一筆逐檔紀錄**、**最近一次手動補抓**（如果比排程的新）、**上一次成功評估**的時間、**最後有效 K 棒**，與依當下時間算的 `lag`；沒有紀錄的標的回 `null`（例如還沒開盤）。
* **最近一輪被跳過時**（v1.5）：skipped 的輪次沒有逐檔列，所以畫面如果只看逐檔列，會停在上一輪的結果。最近一輪排程的輪次紀錄比該檔最近一筆逐檔列新時，
  以輪次紀錄為準，例如 09:15 等鎖逾時：「09:15 本輪未執行（等鎖逾時）｜本輪未評估｜最後有效 K 棒 09:10｜上一次評估 09:10」——
  保留 09:10 的最後有效 K 棒與上一次評估時間，但⛔ 不把 09:10 的評估結果當成 09:15 的。
* **刷新方式**：前端在盤中每 60 秒重抓一次（排程是 5 分鐘一輪，60 秒內看得到新一輪）；非盤中每 5 分鐘一次（實作 review 後改：原本非盤中只抓一次，開盤前開著的頁面開盤後不會恢復）。
  每一列顯示 `data_at`／`eval_at`，所以畫面停在舊資料時看得出來。⛔ 不靠 WebSocket（它只在產生訊號時推播）。
* 自選清單每一列顯示兩段：
  * **行情**：「HH:MM 已更新｜最後 K 棒 HH:MM」、「本輪未取得新 K 棒」、「行情更新失敗（原因）」；延遲超過門檻另加「行情延遲 N 分鐘」。
    最近一次手動補抓比排程的新時，顯示「HH:MM 已補抓，本次未評估（上一次評估 HH:MM）」。
  * **評估**：「未觸發」只在 `no_signal` 時出現；`signal_sent` 顯示訊號、`signal_suppressed` 顯示「已觸發（與先前的訊號相同，未重送）」、
    `withheld_stale` 顯示「資料延遲，未即時送出」、`evaluate_failed` 顯示「評估失敗」、`not_evaluated` 顯示「本輪未評估（原因）」。
  * **評估段還沒寫的排程列**：那一輪的 `job_runs` 還是 `running` → 「本輪評估中」；已經結束（含重啟回收的 `aborted`）→「本輪評估未完成（中斷）」。
    兩種情況「上一次成功評估」都不變。
  * 原因只顯示原因碼對照的中文（例如 `http_5xx` →「伺服器錯誤（5xx）」）。⚠️ 原因碼是類別，⛔ 不顯示確切的狀態碼（log 才有）。
* 非盤中時段照樣顯示最後一輪的狀態與時間，⛔ 不假裝是即時的。

### 3.5 矯正手段與互斥（D3）

* **下一輪自動補回（看得見）**：下一輪成功時補回整天的分 K（指紋改變 → 照常評估），畫面看得到恢復時間。
* **手動補抓**：`POST /api/v1/scheduler/intraday-refill/run`。
  * **範圍**：只限**當日**、自選清單（可指定其中幾檔）。
  * **誰能觸發**：沿用現有手動排程端點的權限（任何登入的使用者；系統目前沒有角色）。使用者裁決（D6）：目前只有一人使用，⚠️ **多人使用之前要調整**
    （見功能檢討「共用環境與權限」），這一點要寫進 `api-reference.md`。
  * **只寫 K 棒、⛔ 不評估訊號**；收盤後也可以執行（13:30 最後一輪失敗時用）。結果記 `job_runs`（新的 job 名稱 `intraday_refill`），逐檔寫 `source=refill` 的列。
  * ⛔ **不推進「上一次成功評估」**：補抓的列沒有評估段，下一輪指紋不同就會照常評估。
  * **回報補了什麼**：補抓前先讀目標時段（當日 09:00 到現在或 13:30）DB 裡的 K 棒，跟回應比對，逐檔回報**新增**幾根、**更新**幾根（同時間戳、值不同）、
    **不變**幾根，以及**仍無法確認的缺口**：交易時段內 DB 仍沒有 K 棒的分鐘（可能是當下無成交，也可能是資料缺漏，⛔ 不判定是哪一種）。
    最後一根時間與總根數只證明寫入進度，⛔ 不當成「已補齊」的證據。
* **互斥與上限**（v1.3 補上限）：盤中一輪與手動補抓共用一把鎖，依序執行（兩輪盤中排程之間也是），⛔ 不再重疊，較舊的請求⛔ 不會晚回來覆寫較新的 K 棒。

  | 對象 | 上限 | 逾時怎麼辦 |
  |---|---|---|
  | 盤中一輪等鎖 | 60 秒 | ⛔ 不執行這一輪：建一筆 `job_runs`，狀態 `skipped`、摘要 `lock_wait_timeout`，`intraday_rounds.outcome=skipped_lock_timeout`；log `intraday round skipped: lock wait timeout`（帶 `run_id` 與排定時點）；⛔ 不寫逐檔的列。I-123 看得出「本輪未執行」跟「行情抓取失敗」（`failed`）不同 |
  | 手動補抓等鎖 | 30 秒 | 回 409，⛔ 不建 `job_runs` |
  | 手動補抓整次 | 45 秒（含抓取與寫入；context 到期就停） | `job_runs` 記 `failed`、摘要 `refill_deadline`，已寫入的檔照實回報，其餘標「未完成」 |

  * 補抓的整次上限（45 秒）小於盤中等鎖的上限（60 秒），所以正常情況下盤中輪次⛔ 不會因為補抓被跳過。
  * 盤中一輪本身的執行時間仍由現有的重試預算（120 秒）與抓取、評估的時間決定；一輪拖過下一輪的等鎖上限時，下一輪照上表記成 `skipped`。
  * **先拿鎖、再建 `job_runs`**（v1.4 改）：`job_runs.started_at` 因此是**實際開始執行**的時刻，依序執行的兩輪⛔ 不會被既有的重疊查詢誤判成重疊。
    排定的時點記在 `intraday_rounds.scheduled_at`；等鎖的時間記在 `lock_wait_ms` 與 log。
  * `skipped` 的 `job_runs` 在等鎖逾時的那一刻建立、立刻結束；它⛔ 沒有執行，重疊判定要排除它（第九節）。
* **事後補判**、**備援來源**：⛔ 不在本案。

## 四、受影響的檔案、模組與資料流

| 位置 | 改動 |
|---|---|
| `backend/internal/market/fetcher.go` | 批次與 FinMind 逐檔回傳逐檔結果；補抓用的方法（只寫 K 棒、回報新增／更新／不變／缺口） |
| `backend/internal/scheduler/scheduler.go`、`job_error.go` | tally 分開 fetch／store；評估前判定；逐檔紀錄；鎖與上限；`intraday_refill` job；保留期清理 |
| `backend/internal/indicator/`、`backend/internal/signal/` | 指紋窗口共用 `lookback`；`EvaluateResult` 回報判重擋下；不送出的選項 |
| `backend/internal/store/` | `IntradaySymbolRunRepo`（insert 行情段、update 評估段、最近一筆、上一次成功評估、依 run 列出、清理）、`IntradayRoundRepo` |
| migration 079（postgres、sqlite、mysql） | `intraday_symbol_runs`、`intraday_rounds` |
| `backend/internal/api/` | `GET /intraday/status`、`POST /scheduler/intraday-refill/run` |
| `frontend/src/components/market/WatchlistTable.svelte`、`lib/api/` | 行情與評估兩段狀態、60 秒刷新、補抓按鈕與結果 |
| 文件 | 見第八節 |

資料流（一輪）：等鎖（≤ 60 秒）→ 建 run 與 `intraday_rounds` → Yahoo 批次（重試回傳逐檔結果） → 逐檔結果 → 寫 K 棒 → 讀最近 120 根算指紋 → insert 行情段 →
只對可評估的標的呼叫訊號引擎 → update 評估段 →（`job_runs` 摘要與 `symbols_failed`）→ 放鎖 → 前端每 60 秒讀狀態 API。

## 五、主要風險與回滾

| 風險 | 處理 |
|---|---|
| I-123 第二輪觀察中途改變計數口徑 | 使用者裁決採第九節的 C：日終判定、換版前封存、記下切換證據、新版本另取完整 5 個交易日 |
| `job_runs` 的狀態變了（以前看不到的逐檔寫入失敗會變 partial／failed） | 這是 I-103 要的；摘要分開列 fetch 與 store，文件寫明 |
| 指紋相同就不評估，漏掉訊號 | 指紋取引擎實際讀的 120 根的完整內容；評估失敗、補抓、中斷都不算成功評估；測試鎖住「只修訂較早一根」 |
| 每輪多讀 120 根算指紋 | 11 檔 × 每 5 分鐘一次、走既有索引；跟評估本身讀的量相同 |
| 延遲門檻誤判（Yahoo 的延遲本來就未知，T-032） | 第一版只在「新但延遲」時暫緩送出，其他情況只標示 |
| 鎖讓一輪晚開始或被跳過 | 等鎖上限 60 秒，跳過時記 `skipped`＋`lock_wait_timeout`；補抓上限 45 秒小於它 |
| 逐輪紀錄的量 | 約 600 列／天，保留期跟 `job_runs` 相同 |
| migration 079 | 新表；回滾時退回舊 image，⛔ 不用退 migration（舊程式不讀不寫它；比照 078 的分析） |

## 六、裁決

* **D1 沒有新 K 棒時**：只有**引擎實際讀到的 K 棒跟上一次成功評估相同**才跳過；同時間戳更新、補回或修訂較早的棒、上一次評估失敗，都要評估（3.2）。
* **D2 延遲**：首版門檻 10 分鐘；延遲跟抓取、寫入、評估結果分開記，停止更新時也顯示；「新但延遲超過門檻」照常計算、⛔ 不當即時訊號送出（3.2、3.3）。
* **D3 矯正手段**：下一輪自動補回（看得見）＋手動補抓（當日、自選清單；回報新增／更新／不變與仍無法確認的缺口；不推進評估進度；與排程互斥、有上限）（3.5）。
* **D4 FinMind 逐檔路徑**：比照；抓取或寫入失敗、空回應都⛔ 不評估；補測試「空回應時⛔ 不會用舊 K 棒評估」（3.1）。
* **D5 上版時間**（✅ 使用者 2026-11-20 裁決採第九節的 C）：實作、review、dev 驗收完成就上 live，⛔ 不等 11-27；換版前後照第九節處理 I-123。
* **D6 手動補抓的權限**（✅ 使用者 2026-11-20 裁決）：沿用現有手動排程端點的權限（任何登入的使用者）；目前只有一人使用，**多人使用之前要調整**，寫進 `api-reference.md`。

## 七、測試與驗證

* **fetcher**：回應有 K 棒 → `written`＋最後一根時間；回應沒有這檔、全 null → `no_data`；`BulkInsert` 失敗 → `store_failed`；整批 HTTP 500 → error。
* **指紋**：同時間戳的棒值改變 → 改變；補回較早的棒 → 改變；**只修訂較早一根**（根數、最後一根、成交量合計都不變，只改中間某一根的高低價或收盤價）→ 改變；
  完全相同 → 不變；窗口跟指標的 `lookback` 同源。
* **排程（首版最重要的一條）**：DB 裡的舊 K 棒**符合突破條件**，模擬 HTTP 500 且重試用完 → 該檔⛔ 不呼叫訊號引擎、⛔ 不產生或推送訊號、
  行情段 `fetch_failed`（`http_5xx`）、評估段 `not_evaluated`（`fetch_failed`），⛔ 不顯示「未觸發」；同一輪另一批成功的標的照常評估。
* **`job_runs`**：逐檔寫入失敗 → `store_failed:1 (…)`、partial、`symbols_failed` 帶那一檔；整批失敗 → `fetch_failed:N (…)`；兩者同時有 → 分開列、`symbols_failed` 是聯集；
  `no_data` ⛔ 不計失敗。**逐輪對帳**：從 `intraday_symbol_runs` 重算的檔數等於那一輪 `job_runs` 的摘要與 `symbols_failed`。
* 指紋相同 → `not_evaluated`（`unchanged`）；評估失敗後同一份資料再來 → 會重試評估。
* **重試的逐檔結果**：第一次 500、重試 200 之後 A 寫入成功、B 寫入失敗、C 沒有資料 → 三檔分別記錄、`http_recovered=true`、`fetch_attempts=2`；
  **重試預算到期**（同一個案例驗三種結果）：A 已提交 → `written` 並評估；B 的 K 棒已在 HTTP 200 裡、寫入階段逾時 → `store_failed`（`retry_budget`）；
  C 回應確認沒有 → `no_data`；另驗 HTTP 回應本身逾時 → 該批 `fetch_failed`（`retry_budget`）。層二 log 的觸發條件與欄位不變。
* **最近一輪被跳過**：09:10 正常評估、09:15 等鎖逾時 → API 帶出 09:15 的輪次紀錄，畫面顯示「本輪未執行（等鎖逾時）｜本輪未評估」，最後有效 K 棒與上一次評估仍是 09:10。
* **存證失敗**：`startRun` 失敗 → 照樣抓取與評估、log `intraday evidence unavailable`；逐檔紀錄寫失敗 → `evidence_failed:N`、該輪至少 partial、
  `intraday_rounds.evidence_failures` 計數；評估段寫失敗 → 下一輪再評估、訊號由判重擋下。
* **排定與執行分開**：兩輪依序執行時，`job_runs` 的區間⛔ 不重疊（用既有的重疊查詢核對）；`intraday_rounds` 的 `scheduled_at` 與 `lock_acquired_at` 各自正確。
* 觸發但被判重擋下 → `signal_suppressed`；新但延遲超過門檻、觸發 → `withheld_stale`、`signals` 沒有新列、沒有推播。
* **鎖與上限**：補抓與盤中一輪依序執行；盤中一輪等鎖超過 60 秒 → `skipped`＋`lock_wait_timeout`、沒有逐檔的列；補抓等鎖超過 30 秒 → 409；
  補抓整次超過 45 秒 → `failed`＋`refill_deadline`、已寫的照實回報；兩輪盤中排程⛔ 不重疊。
* **手動補抓**：收盤後可執行、只寫 K 棒、⛔ 不呼叫訊號引擎、⛔ 不推進上一次成功評估（補抓後下一輪照常評估）、回報新增／更新／不變與缺口、`job_runs` 有紀錄；
  畫面顯示「已補抓，本次未評估」，⛔ 不顯示「本輪評估中」。
* **畫面**：評估段還沒寫、`job_runs` 還在跑 →「本輪評估中」；已結束或 `aborted` →「本輪評估未完成（中斷）」。
* FinMind 路徑：空回應 → `no_data`、⛔ 不評估。
* 保留期清理會一起刪逐輪紀錄。
* API 的 JSON 形狀（有紀錄、沒有紀錄回 null）；前端各狀態的文字、60 秒刷新、⛔ 不顯示原始錯誤。
* migration 079：`scripts/test-postgres-migrations.sh`、`scripts/test-mysql-migrations.sh`、SQLite 的 migration 測試。
* log 相容：觀察工具用的訊息名稱與欄位都還在（測試核對 log 欄位）。
* dev 實跑：用假的 Yahoo 回應走「成功、500、恢復」三輪與一次手動補抓，前端看得到各狀態。
* 完整回歸：`backend/scripts/test.sh`、`frontend/scripts/test.sh`、`python/scripts/test.sh`。

## 八、完成後要歸檔的位置

* `docs/architecture.md`「寫入失敗的一致性契約」：批次路徑的 `symbols_failed` 改成逐檔粒度、摘要分開列 fetch 與 store，`no_data` 不算失敗。
* `docs/yahoo-intraday-integration.md`：逐檔結果、能不能評估的規則與指紋、延遲、手動補抓、互斥與上限、13:30 最後一輪的處理。
* `docs/architecture/data-pipeline.md`：盤中的資料流與逐輪紀錄。
* `docs/database-schema.md`：`intraday_symbol_runs`、`intraday_rounds`。
* `docs/api-reference.md`：兩支新端點（手動補抓註明：任何登入的使用者都能觸發，多人使用之前要調整）。
* `docs/issue.md` I-103、`docs/todo.md` T-088 改成「已實作／待 review」；review 確認後收斂。

## 九、上版時機與 I-123 的分段判讀（使用者 2026-11-20 裁決：採 C）

**為什麼不能直接換**：I-123 第二輪要跟第一輪用**同一套口徑**判讀。T-088 會改變兩件事：`job_runs` 的計數（逐檔寫入失敗會讓 success 變 partial、全部失敗變 failed），
以及盤中評估的行為（跳過、暫緩送出）。log 訊息名稱保留，工具讀得到，但**計數口徑會變**。

**評估過的做法**（v1.1）：A 等 11-27 交接後才上；B 開關分段上版（行為改變的部分放在預設關閉的開關後面）；**C 現在就上完整版、分段判讀**；
D 改用 log 計數判讀。使用者裁決採 **C**：實作、review、dev 驗收完成就上，⛔ 不等 11-27。

**C 的做法**（⚠️ 以下判讀規則在看到 11-20 之後的資料**之前**定下來，⛔ 事後不改）：

1. **日終判定**（v1.3）：以 **2026-11-26 台北日終（11-27 00:00）** 判定舊版是否完整覆蓋原窗口（11-20～11-26）。
   * 換版發生在 11-27 00:00 **之後**（含 11-27 08:00 交接之後才換）：舊版完整覆蓋，照 I-123 的原安排判讀與裁決，⛔ 不分段。
   * 換版發生在 11-27 00:00 **之前**（含 11-26 收盤後到午夜之間）：照下面第 2～6 點分段。⚠️ 11-26 當天即使 55 輪都齊了，晚間的排程與 Redis 的觀察還沒結束，
     ⛔ 不算完整覆蓋。
2. **換版前封存**：上 live 之前，照 I-123 計畫書「新主機觀察的做法」第 1 點交接一次——`job_runs` 快照（只取這張表）與 UTC 11-19 到換版當天的 backend log，
   兩端核對 SHA-256。⛔ 沒有封存成功就不換版。
3. **切版證據**（v1.3）：分別記下三件事，寫進 I-123 計畫書與本計畫書——
   ①**舊版資料截止**：舊 backend 收到 shutdown 的時間，以及舊版最後一筆 `job_runs` 的 id 與時間；
   ②**新 backend 啟動**：commit、backend image、`server starting` 的時間；
   ③**新版第一筆 intraday**：新版本跑的第一個 intraday 輪次的 `run_id` 與時間。
   換版要在收盤之後（14:05 之後，照部署規範），切換點落在兩個交易日之間，⛔ 不把一個交易日切成兩半。
4. **切換前那一段**（11-20 到換版當天）：照原口徑、原門檻判讀並記錄。它⛔ 不足 5 個交易日，所以⛔ 不能據此「通過」；
   但如果已經觸發 I-123 第 4 點的「另立問題」條件（例如最終失敗 > 5 輪、同一個原因碼分散 3 天以上），就照「另立問題」處理。
5. **新版本的完整窗口**（v1.4 改）：從**換版後的第一個排定交易日**起，另取完整的 5 個交易日，當作 I-123 的延長那一輪（延長只做一次的規則不變）。
   ⛔ 不以「第一筆實際 intraday」起算：首日如果整天漏跑，那正是要抓的異常（那一天照 I-123 的「資料不足」處理）。新版第一筆 intraday 的 `run_id` 只當切版證據。
   * **數字門檻照原樣**；結果**標明「採新版計數口徑」**。
   * 最終失敗輪數照原定義，用 `job_runs` 的 `failed` 計。新版本下 `failed` 也可能因為**全部標的寫入失敗**（`store_failed`）而出現：判讀時把這種輪次**另外列出**、
     說明原因，但⛔ 不從計數中扣除（扣除等於事後改門檻）。
   * **`skipped`（等鎖逾時）怎麼算**（v1.4）：每天要有 55 輪**排定**的紀錄（`job_runs` 含 `skipped`；不到 55 照「資料不足」）；
     `skipped` 的輪次那一輪⛔ 沒有新資料、也⛔ 沒有評估，對使用者的結果跟抓取失敗相同，所以**計入最終失敗輪數**（從嚴），判讀時另外列出原因 `lock_wait_timeout`。
   * **重疊**：只看**實際執行**的輪次（排除 `skipped`），用 `job_runs` 的起訖判定（先拿鎖再建 run，起訖就是執行區間），也可以用 `intraday_rounds` 的執行區間交叉核對。
     ⚠️ （實作 review 2026-11-20）**手動補抓**在放鎖之後才寫完 `job_runs`，它的 `job_runs` 起訖⛔ 不等於持鎖區間：新版查詢若把補抓納入重疊判定，
     一律用 `intraday_rounds` 的 `lock_acquired_at`～`exec_finished_at`（依序執行的補抓按 `job_runs` 起訖會報重疊 1、按持鎖區間是 0）。
   * **對帳不完整**的輪次（3.3）另外列出。
   * **逐輪證據缺失時的資料不足**（v1.5）：
     * 某交易日 intraday 的 `job_runs`（含 `skipped`）不是 55 輪 → 資料不足（建立 run 失敗的輪次就是這樣）；
     * 實際執行的輪次缺 `intraday_rounds` 的列、或逐檔列數不等於那一輪的 `symbols_total`，**而且那一輪沒有 `evidence_failed` 標記**（證據無故缺失）→ 資料不足；
     * 有 `evidence_failed` 標記的輪次（3.3）→ ⛔ 不算資料不足（`job_runs` 的狀態仍在、失敗已被記下），列為「對帳不完整」；
     * 封存的 `job_runs`、兩張逐輪表、log 任一份讀不到或不完整 → 資料不足（沿用既有規則）。
   * **新版的封存查詢與驗收案例**（v1.5）：觀察工具（repo 外）的封存版要加一份**新版查詢**，⛔ 不改舊版查詢（切換前那一段照舊版判讀）：
     * 最終失敗輪數 ＝ `failed` ＋ `skipped`（原因 `lock_wait_timeout`），兩者分開列；
     * 重疊只取實際執行的輪次（排除 `skipped`）；
     * 依上一點判資料不足與對帳不完整；全部寫入失敗造成的 `failed` 另外列出。
     * **驗收案例**（判讀前必須先跑過，結果寫進 I-123 計畫書）：模擬資料「一輪執行中、另一輪等鎖逾時」——**舊查詢**報「重疊 1、失敗 0」，
       **新版查詢**必須報「重疊 0、失敗 1」；另加「執行的輪次缺逐檔列、沒有 `evidence_failed`」→ 資料不足，「有 `evidence_failed`」→ 對帳不完整、⛔ 不是資料不足。
   * 層二的救回、原因碼、Redis 等其他項目照原做法；觀察工具讀的 log 訊息名稱⛔ 不變。
   * 新版本另外有逐輪紀錄（`intraday_symbol_runs`、`intraday_rounds`），封存時一起帶走，判讀時可以拿來逐輪對帳，⛔ 不改變門檻。
6. **新版查詢的實作與驗收**（2026-11-20，repo 外的 `~/.cache/stock_trading/obs/archive/`）：
   * `run-obs-archive.sh` 加 `OBS_QUERY_SET=v2`（預設 v1，行為⛔ 不變——用 11-13～11-19 的資料重跑，v1 的 12 個輸出檔跟修改前逐位元組相同；原檔備份在 `run-obs-archive.sh.v1-20261120`）。
   * 新版查詢獨立在 `queries_v2.tsv`（工具與驗收腳本讀同一份）：`v2_intraday_daily`（排定輪數、最終失敗＝failed＋等鎖逾時的 skipped、全部寫入失敗與讀不到清單另外列）、
     `v2_rounds`（連續兩輪判定用）、`v2_overlap`（盤中與補抓用 `intraday_rounds` 的持鎖區間、排除沒有執行的；其他工作照舊）、`v2_evidence`（逐輪對帳）；
     `v2_judge.py` 印出判讀用的數字，資料不足時結束碼 3（⛔ 不在工具裡下通過／延長的結論）。
   * 驗收 `accept_v2.sh`：在 dev postgres 建臨時 DB（表結構取自 repo 的 migration 010／063／079、只有合成資料、用完刪掉），全部符合：
     「一輪執行中、另一輪等鎖逾時」舊查詢報重疊 1／failed 0，新版報**重疊 0、最終失敗 1**；補抓按 `job_runs` 起訖會報重疊 1，按持鎖區間 0；
     有 `evidence_failed` 的輪次是對帳不完整（結束碼 0）；已執行卻缺逐檔列、沒有 `evidence_failed` → 資料不足（結束碼 3）；每日不到 55 輪 → 資料不足。
   * 檔案指紋（SHA-256 前 12 碼）：`run-obs-archive.sh` `3054d5671db0`、`queries_v2.tsv` `f601d260afdd`、`v2_judge.py` `9effa36a37ae`、`accept_v2.sh` `5fd8c84f65aa`。
   * ⚠️ 新版窗口的交接要多帶兩張表：`pg_dump -t public.job_runs -t public.intraday_rounds -t public.intraday_symbol_runs`。
   * 2026-11-21 更新（I-139 計畫書第九節的先行項目）：判讀改依預定交易日清單逐日核對、每個預定交易日恰好 55 輪，上面的指紋已過時，現行的指紋與驗收紀錄見 I-123 計畫書「新版窗口的預定交易日」。
7. **上版順序**（使用者 2026-11-20 裁決）：11-21（假日）準備並上版，下禮拜 live 與 dev 同步驗證（dev 的「成功 → 500 → 恢復」實跑與畫面核對改在上版之後、跟 live 第一個交易日同步做）。
   ⚠️ 這表示上版前⛔ 沒有 dev 的排程輪次實跑；風險由單元與整合測試（真的 SQLite、真的訊號引擎）與 dev 的手動補抓實跑承擔，出問題照回滾（退回 `pre-t088` 的 image、⛔ 不用退 migration 079）。
   ✅ 2026-11-24 補做完成：dev 的「成功 → 500 → 恢復」實跑與畫面、live 新版前兩個交易日的輪次與逐檔紀錄都符合（見第十節）。
   ✅ **live 上版紀錄**（2026-11-21 週六 15:12～15:26；平日的 cron 在週六⛔ 不觸發，live 週末只有 06:30 主檔同步與 07:00 終止上市對帳，都已跑完）：`31e5566` → `941f11e`；image backend `bc503920bb75` → `cc9c64c1e37e`、python-server `f166cc4e58c5` → `75922cb6672b`、python-worker `4303b51148a2` → `7fea4e0b4679`（python 程式沒改，重新 build 出新的 ID）；回滾標籤 `pre-t088` 指向舊的三個；`Config.Env` 50 行 `a4eccbc0aa79` 不變；backend 執行檔 `90539d61bf71` → `639c279ba1cf`；migration 79、兩張新表都在、0 筆；前端入口 `index-C6VWJ-Bz.js` → `index-BC4dYoER.js`；`/intraday/status` 未帶 token 回 401（換版前 404）；log 的 `redis connected`＋`scheduler started` 2 行、設定錯誤 0、panic 0；python `/health` 正常。**新 backend 啟動 2026-11-21 15:21:59（台北）**。上版後的前端確認（E-1）：使用者 2026-11-21 回報正常。
   ✅ **切版證據**（2026-11-21）：① 舊版資料截止：`job_runs` 最大 id `6078`（11-21 07:00 `delisting_reconcile`）、共 1541 筆，dump 前後一致；舊版最後一筆 intraday 是 id `6069`（11-20 13:30）。② 新 backend 啟動 2026-11-21 15:21:59（台北）。③ 新版第一筆 intraday 的 `run_id`：`6085`（2026-11-23 09:00:00 台北，`success`；2026-11-24 10:17 左右 live 唯讀查詢，`intraday_rounds` 最小 `run_id` 也是 `6085`、當時共 71 列＝11-23 的 55 輪加 11-24 09:00～10:15 的 16 輪，兩邊一致）。換版前那一段的封存在 `~/i131_merge/live/obs1121/`（`job_runs` dump 32,398 bytes＋UTC 11-19～11-21 三個 log），`SHA256SUMS` 的雜湊 `8737224ae96d…`，live 端、admin 搬檔後與 dev 端三次核對一致；UTC 11-21 的 log 只複製到 15:15，換版前最後幾分鐘與換版後的內容留給新版窗口的交接。換版前那一段只有 11-20 一個交易日，照第九節的 C 用舊口徑判讀並記錄（⛔ 不能據此通過）。
8. **I-123、I-122 的收斂**跟著延後到新窗口判讀完成；dev 作業 volume、交接目錄照 I-123 的保留條件繼續保留。

**代價**：I-123 的收斂比 A 晚（新窗口 5 個交易日從換版後才開始）；好處是盤中逐檔的可見性與「⛔ 不用舊 K 棒評估」提早上線。

## 十、實作備註（2026-11-20；已改好、未 commit，待 review）

**改了哪些**

* market：`IntradaySymbolResult`（`written`／`no_data`／`store_failed`／`fetch_failed`）；`FetchAndStoreIntradayBatch(ctx, runID, symbols)` 回逐檔結果，
  拆出 `FetchIntradayBatchCandles`（只抓）與 `StoreIntradayBatch`（依請求順序逐檔寫，ctx 到期時還沒寫的記 `store_failed`）；`FetchAndStoreMinute` 回逐檔結果（空回應 `no_data`）。
* store：`IntradayEvidenceRepo`（兩張表、最近一筆與上一次成功評估各一句查詢、清理同一個交易）；migration 079 三份。
* signal：`EvaluateWithOptions`（`Withhold`：停在 reservation 之前）、`EvaluateResult.Suppressed`（DB 判重與 reservation 都算）、`Withheld`／`WithheldSignal`。
  `indicator.Lookback` 匯出給指紋用。
* scheduler：新檔 `intraday.go`（鎖、一輪、兩條路徑、判定、指紋、存證、重試回傳逐檔結果）與 `intraday_refill.go`（手動補抓）；
  `job_error.go` 的 tally 加 `storeFailed`、`evidenceFailed`，**移除只知筆數的 opaque 計數**（唯一的使用者是舊的批次路徑）；`pre_market` 清理逐輪紀錄。
* api：`GET /intraday/status`、`POST /scheduler/intraday-refill/run`（`IntradayHandler`）；main 注入 `SetIntradayEvidence`。
* frontend：`lib/api/intraday.ts`、`lib/utils/intradayStatus.ts`（判讀文字的純函式）、`lib/stores/intraday.ts`；自選清單每一列下方一行盤中狀態、
  標題列「補抓當日分 K」按鈕與結果；Dashboard 盤中每 60 秒重抓、非盤中抓一次就停。dist 重新 build。
* 文件：`architecture.md`「寫入失敗的一致性契約」、`yahoo-intraday-integration.md`「逐檔結果、能不能評估與存證」、`architecture/data-pipeline.md`
  「盤中的逐輪存證」、`database-schema.md` 兩張表、`api-reference.md` 兩支端點。

**跟計畫的差異**（都不改變規格的方向，review 時請確認）

1. **「本輪評估中」與「中斷」的判定**：計畫寫看那一輪 `job_runs` 是否 `running`；實作改用 scheduler 行程內「正在執行的 run_id」
   （`IntradayRunningRunID`）——等價（重啟回收的 `aborted` 一定不是行程內正在跑的那一輪），而且⛔ 不用為了這件事擴充 `JobRunRepo` 介面。
2. **FinMind 路徑的「帳號等級不足」**：以前逐檔抓、逐檔評估，撞到等級不足時前面的檔已經評估過；現在先抓完再評估，撞到時整輪 `skipped`、前面的檔也不評估。
   live 沒有啟用這條路徑。
3. **評估結果的優先順序**：同時有降級與送出時記 `degraded`（原因是第一個降級的階段），⛔ 不記 `signal_sent`。
4. **缺口的範圍**：13:26～13:29 是收盤集合競價（13:25～13:30 不撮合），⛔ 不列為缺口；09:00 照列（第一根是否標成 09:00 依來源而定，列為「仍無法確認」）。
5. **`intraday_refill` 不在 `/scheduler/status` 的排程清單**：它只有手動觸發，列進去會被當成 `never_run`／stale；結果查 `job_runs` 與補抓的回應。
6. **新增一行 log** `intraday round started`（`run_id`、排定時點、等鎖時間）；既有的訊息名稱與欄位都⛔ 不變。
**實作 review（2026-11-20）的修正**：

* [P2] 手動補抓失敗或沒有當日 K 棒時，畫面仍顯示「已補抓」→ 依補抓的逐檔結果顯示（「已補抓」「補抓：沒有當日 K 棒」「補抓失敗（原因）」），都保留最後有效 K 棒。
* [P2] 開盤前開著 Dashboard、開盤後不會自動刷新（非盤中的回應會清掉計時器）→ 改成 `setTimeout` 串接：盤中 60 秒、非盤中 5 分鐘，每次依最新的 `market_open` 決定下一次；
  補跨開盤的驗收（fake timers：非盤中 → 開盤 → 每 60 秒、停止之後不再抓）。
* [P2] 補抓的 45 秒沒有涵蓋整段持鎖時間（收尾重試最壞約 133 秒仍持鎖）→ 期限從拿到鎖開始算，建 run 改用期限內的 ctx，K 棒寫完就放鎖，存證與 `job_runs` 收尾在鎖外；
  補測試「`job_runs` 收尾卡住時，鎖已經放掉、K 棒已寫入」。`intraday_rounds.exec_finished_at` 記的是放鎖的時刻。
* [P2] 讀不到自選清單時兩條路徑都以 total=0、failed=0 收尾、推成 success → 明確記成 `failed`（`watchlist_fetch`），補兩條路徑的測試。

**實作 review 第二輪（2026-11-20）的修正**：

* [P2] 讀不到自選清單後畫面仍沿用上一輪的「未觸發」→ 輪次紀錄記 `outcome=failed`（migration 079 的 CHECK 加 `failed`；079 尚未上 live，直接改）；
  狀態 API 的 `round_skipped` 改成一般化的 `round_state`（`skipped`／`failed`／`running`／`incomplete`），補抓與輪次紀錄都比逐檔列新時取 run_id 較新的；
  畫面顯示「本輪執行失敗｜本輪未評估」並保留上一次評估時間。
* [P2] 初次請求未完成就切頁，舊頁面仍會啟動輪詢與 WebSocket → 銷毀之後⛔ 不再啟動；補 Dashboard 的元件測試（拿掉那道檢查時測試會失敗）。
* [P3] 持鎖測試可能在補抓真正開始前就拿到鎖 → 先用通道確認已進入 `job_runs` 的收尾，再驗證拿得到鎖。

7. **`job_runs` 摘要的原因碼**沿用既有的 joberr 分類器（`safeJobErrorReason`）：Yahoo 回 HTTP 500 時摘要是 `fetch_failed:2 (0050:internal_error, …)`，
   逐檔紀錄的 `data_cause` 才是 `http_5xx`（`market.TransientCause`）。joberr 認不得 HTTP 狀態碼是既有的分類範圍，本案⛔ 不改；review 時請判斷要不要另案。

**驗證**（2026-11-20）

| 項目 | 結果 |
|---|---|
| backend 完整測試（`backend/scripts/test.sh`） | 全部通過 |
| scheduler 的盤中測試開 race detector（`RACE=1`） | 通過 |
| postgres、mysql 的 migration 驗證（up → 驗 schema → 分段 down 到 0） | 都 rc 0 |
| frontend（svelte-check、vitest 186 項、build） | 通過；dist 有新檔要一起加入版控 |
| python 完整測試 | 2154 項通過、1 項跳過（這次⛔ 沒有改 python） |
| dev 實跑：手動補抓 | ✅ 見下方 |
| dev 實跑：排程輪次（成功、500、恢復）與畫面 | ✅ 2026-11-24（上版之後做，第九節第 7 點；見下方） |
| 實作 review 的四項修正（2026-11-20） | backend scheduler 測試、frontend（svelte-check 0、vitest 189 項、build）通過 |
| 實作 review 第二輪的三項修正（2026-11-20） | backend 完整測試、postgres／mysql migration 驗證（079 的 CHECK 加 `failed`）、frontend（svelte-check 0、vitest 192 項、build）通過；⚠️ dev DB 已套用舊版 079，下次 dev 實跑前要讓它重新套用（刪兩張表與 `goose_db_version` 的 79 那一列，只在 dev） |
| C 切版前：觀察工具的新版查詢與驗收案例（重疊 0、失敗 1） | ✅ 2026-11-20（見第九節「新版查詢的實作與驗收」） |

* **dev 實跑：手動補抓**（2026-11-20 收盤後；dev compose 加暫存 override：一個假的 Yahoo 批次端點（TLS、自簽憑證只給這個 container 信任），
  `YAHOO_ENABLED=true`；postgres 掛的是一般的 dev volume）：自選清單 2330、0050。
  ①第一次：當日 09:00～13:30 共 271 分鐘、假服務刻意缺 09:30 → 每檔新增 270、缺口只列 09:30（13:26～13:29 假服務有回，⛔ 不是缺口）；
  ②第二次（同樣的資料）→ 不變 270；③假服務回 500 → 兩檔 `fetch_failed`／`http_5xx`，`job_runs` 是 `failed`、2 檔失敗。
  `intraday_rounds` 4 筆 `source=refill`、都有結束時間；狀態 API 在收盤後 `market_open=false`、延遲依當下時間算（219 分鐘）但⛔ 不標 stale、沒有排程輪次時 `latest_round` 是 null。
  實跑發現兩處小問題，已修並補測試：自選清單是空的時補抓回應的 `symbols` 是 null（改成空陣列）；抓取失敗那幾列的 `gaps` 是 null（改成空陣列）。
* **dev 實跑：排程輪次與畫面**（2026-11-24 盤中；同一個 dev compose 與假 Yahoo，自選清單 2330、0050；dev 的 `JOB_RETRY_ENABLED=false`，所以每一檔 `fetch_attempts=1`、同一輪內⛔ 不重試，是預期行為）：
  ①成功：10:00 那一輪 `job_runs` id 82 `success`、`intraday_rounds.outcome=executed`，兩檔 `written`／`no_signal`；
  ②假服務改成回 500（10:00:17 切換）→ 10:05、10:10 兩輪（id 83、84）`job_runs` 是 `failed`、2 檔失敗，`intraday_rounds` 仍是 `executed`，逐檔 `fetch_failed`／`http_5xx`／`not_evaluated`；
  畫面（首頁自選清單，使用者確認）：「10:05 行情更新失敗（伺服器錯誤（5xx））｜最後有效 K 棒 10:00」、「評估：本輪未評估（行情更新失敗）」，最後有效 K 棒沿用上一輪、⛔ 沒有被空值蓋掉；
  `session_data` 維持 `ok`（當天已經有 K 棒，抓取失敗只在每一列顯示，照 I-139 的規格⛔ 不另出橫幅）；
  ③改回 ok（10:10:10）→ 10:15 那一輪（id 85）`success`，兩檔 `written`、最後 K 棒 10:15，畫面恢復「10:15 已更新」。
  `job_runs.error` 的摘要只寫粗分類（`internal_error`），細的原因碼在 `intraday_symbol_runs.data_cause`（`http_5xx`）與 log 的 `intraday batch fetch failed`（`cause`）。
  ⚠️ 驗收環境的插曲：假服務的自簽憑證只給了 2 天效期，2026-11-22 17:07 過期，11-23 08:50 的 pre_market 與 09:00～09:30 的 7 輪（id 8～15）都因為 TLS 驗證失敗記 `failed`（逐檔原因 `not_transient`）；09:31 換 30 天的憑證、重啟假服務與 backend 後，09:35 起恢復。這是 dev 驗收環境的問題，⛔ 不是程式問題。
* **live 同步驗證**（2026-11-24 10:25 左右，使用者在 live 跑唯讀查詢；新版第一個交易日 11-23 與 11-24 盤中到當時）：
  `intraday_rounds` 11-23 是 55 輪 `scheduled`／`executed`、11-24 到 10:25 是 18 輪，存證寫失敗 0、等鎖 0 ms、排定到實際開始最多 0.04 秒；
  `job_runs` 的 intraday 兩天都是 `success`（55、18 筆），跟輪次紀錄一致；`intraday_symbol_runs` 11-23 共 605 列＝55 輪 × 11 檔、11-24 共 198 列＝18 × 11，逐輪逐檔都有紀錄；
  兩天都⛔ 沒有 `fetch_failed`／`store_failed`（`no_data` 每天 11 列、`written` 其餘），評估結果有 `no_signal`、`not_evaluated`、`signal_sent`、`signal_suppressed`，⛔ 沒有 `evaluate_failed`／`withheld_stale`／`degraded`；
  換版後的其他作業（pre_market、daily_close、sr_zone_verify、sr_analysis 等）都正常，只有 `candle_gap_detection` 與 `corporate_action_sync` 各一筆 `partial`（單一標的驗不了／股利取不到），跟 T-088 無關。
  live 首頁顯示「盤中：10:25 已更新｜最後 K 棒 10:25」「評估：未觸發」。
  追查（同日稍後）：`no_data` 那 11 列都是每天 09:00 那一輪（開盤當下還沒有 K 棒）；`written` 但 `not_evaluated` 的原因碼全是 `unchanged`（指紋相同、⛔ 不重評）。
  11-23 的 `signal_sent` 145 次跟 `signals` 當天 145 筆一致，換版前 11-16～11-20 每天 122～161 筆，⛔ 沒有因換版變多（`SUPPORT_BOUNCE` 每天約 105～120 筆，是既有的分佈）。
* 新增的測試：抓取失敗時舊 K 棒符合突破條件也⛔ 不評估（對照組證明那批 K 棒確實會觸發）；重試的逐檔結果（A 寫入成功、B 寫入失敗、C 沒有資料，`http_recovered`）；
  重試預算在寫入途中與請求途中到期；指紋相同才跳過、評估失敗會重試；只修訂較早一根指紋就變；延遲的資料⛔ 不送出；等鎖逾時 `skipped`；依序執行時排定與實際開始分開記；
  存證寫失敗 `evidence_failed`＋partial；建立 run 失敗照樣評估；手動補抓的新增／更新／不變與缺口、⛔ 不評估、⛔ 不推進上一次成功評估；補抓 busy／不在清單／沒有來源；
  補抓逾時；FinMind 空回應⛔ 不評估；repo 的最近一筆、上一次成功評估與清理；狀態 API 的 round_skipped、eval_pending、refill、延遲；
  `fetched intraday batch candles` 的既有欄位；前端判讀文字（「未觸發」只對應 no_signal、抓取失敗顯示「本輪未評估」）。

## 版本紀錄

| 版本 | 日期 | 內容 |
|---|---|---|
| v1 | 2026-11-20 | 初版。 |
| v1.1 | 2026-11-20 | review：D1 改成「輸入指紋跟上一次成功評估相同才跳過」（同時間戳更新、補回、評估失敗都要評估）；D2 延遲分開記、停止更新也顯示、「新但延遲」照常計算但⛔ 不即時送出；D3 手動補抓限當日與自選清單、回報新增／更新／不變與缺口；D4 補空回應的測試；D5 交接後即可上版；[P2] 行情狀態與評估結果分兩段寫（評估完成才寫評估段、判重擋下與暫緩送出分開）、定義 60 秒刷新；[P2] 手動補抓⛔ 不推進評估進度、與盤中排程用同一把鎖依序執行、狀態寫入帶單調順序。新增 D6（手動補抓的權限）與第九節（提前上版的評估，建議 B 開關分段上版）。 |
| v1.2 | 2026-11-20 | 使用者裁決：上版採 C（實作、review、dev 驗收完成就上；換版前封存、記下切換時間、切換前那一段照原口徑判讀並記錄、新版本另取完整 5 個交易日當延長那一輪、`store_failed` 造成的 failed 另外列出但⛔ 不扣除）；D6 沿用現有權限，多人使用之前要調整。第九節改寫成 C 的做法。 |
| v1.3 | 2026-11-20 | review：[P2] C 的切版以 11-26 台北日終判定（之後換版照原安排、之前換版就分段），切版證據分成舊版資料截止、新 backend 啟動、新版第一筆 intraday 的 run_id，新版沿用數字門檻並標明採新版計數口徑；[P2] 指紋改成對引擎實際讀的最近 120 根完整內容取 SHA-256（補「只修訂較早一根」的測試）；[P2] `job_runs` 分開記 fetch 與 store 失敗、`symbols_failed` 為聯集，逐檔狀態改成逐輪保存的 `intraday_symbol_runs`（主鍵 run_id＋symbol，保留期同 `job_runs`），可逐輪對帳；[P2] 手動補抓顯示「已補抓，本次未評估」，評估段未寫時分「評估中」與「中斷」；[P2] 共用鎖加上限：盤中等鎖 60 秒（逾時記 `skipped`＋`lock_wait_timeout`）、補抓等鎖 30 秒（409）、補抓整次 45 秒。 |
| v1.4 | 2026-11-20 | review：[P2] 先拿鎖再建 `job_runs`（起訖就是執行區間，依序執行⛔ 不被誤判重疊），排定時點與實際執行區間記在新表 `intraday_rounds`；`skipped` 計入最終失敗輪數（從嚴）、每天仍要 55 輪排定紀錄、重疊只看實際執行的輪次；[P2] 新版窗口從換版後第一個排定交易日起算，第一筆 run_id 只當切版證據；[P2] 重試回傳最後一次嘗試的逐檔結果，記 `fetch_attempts`／`http_recovered` 分開「HTTP 救回」與「寫入成功」，預算到期前已寫的檔照實記；[P2] 存證寫失敗時的處理：建立 run 失敗照跑但當天資料不足、逐檔紀錄寫失敗記 `evidence_failed` 且至少 partial、失敗方向是多評估、標對帳不完整。 |
| v1.5 | 2026-11-20 | review：[P2] 新版判讀規則納入封存驗收：觀察工具加新版查詢（最終失敗＝failed＋skipped、重疊排除 skipped），驗收案例「一輪執行中、另一輪等鎖逾時」舊查詢報重疊 1／失敗 0、新版必須報重疊 0／失敗 1，並明訂逐輪證據缺失時何者算資料不足、何者算對帳不完整；[P2] 最近一輪被跳過時，狀態 API 帶出輪次紀錄，畫面顯示本輪未執行／未評估並保留上一輪的最後有效 K 棒與評估時間；[P2] 重試預算到期依實際階段分類（已提交 written、寫入階段逾時 store_failed、確認沒有 no_data、HTTP 逾時 fetch_failed），同一個案例驗三種結果。 |
