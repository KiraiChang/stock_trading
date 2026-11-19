# I-137 計畫書 v1.4：回測改成依日期範圍取 K 棒（暖身、覆蓋範圍標示、任務上限、日期驗證、breakout_v1 限單檔）

> 狀態：v1.4 草稿，待使用者確認。對應：`docs/issue.md` I-137。
> 需求確認：2026-11-19。使用者裁決（四項都採建議）：D1 往前多取暖身根數、D2 資料不足時照跑並標示、D3 設上限且超過就明確拒絕、
> D4 既有資料不改、記新舊界線。v1 的第九節 Q1～Q4 已裁決（見第三節第 6、7 點與第八節），v1 review 的 5 個 P2 與 API 契約、v1.1 review 的 4 個 P2 與 2 個 P3、v1.2 review 的 2 個 P2 與日期上界、v1.3 review 的 1 個 P2 已併入（見版本紀錄）。

## 一、目標與不做的範圍

**目標**

1. 兩條回測路徑（backtrader 的 `run_backtest`、modular 的 `run_modular_backtest`）都改成**依台北日期**取 `start_date`～`end_date` 的 K 棒，
   再另外取起點之前的暖身資料。⛔ 不再先取「最近 N 根」（1000／2000）再切。
2. **暖身（D1）**：起點之前固定多取 250 根，只用來算指標。**交易起點**與**統計起點**分開定義，兩者都是範圍內第一根（第三節第 2 點）。
3. **覆蓋範圍標示（D2）**：每一檔實際取到的首尾日期、根數、暖身是否足夠、是否被略過，寫進結果並顯示在前端，⛔ 不再只記 log。
4. **任務上限（D3）**：每個任務的標的數與總根數（含暖身）有上限；超過就讓任務失敗，錯誤訊息寫明數字，⛔ 不截斷。
5. **日期驗證（Q1）**：Go 建任務前、python 兩條執行路徑的共用入口，都檢查日期格式、日期存在、開始日 ≤ 結束日。
6. **`breakout_v1` 限單檔（Q4）**：前端、Go、python 都擋下 backtrader 策略的多檔請求；真正的多檔支援另立 `issue.md` I-138。
7. **既有資料（D4）**：⛔ 不改、⛔ 不回補；部署前在 live 唯讀估算哪些任務被截掉過、哪些 `breakout_v1` 任務送了多檔，記下部署的版本與時間當界線。

**不做的範圍**

* 部位大小、`pnl_pct` 口徑、費用細節：`todo.md` T-085。多檔共用資金的組合回測：`todo.md` T-011。
* `breakout_v1` 真正的多檔回測（每檔各自一個 Cerebro、彙總口徑）：`issue.md` I-138，跟本案分開 review。
* 策略邏輯本身（訊號、停損）⛔ 不改；只改「餵進去的資料範圍」與「從哪一根開始允許交易、開始計算統計」。
* **範圍內的缺漏（例如中間少了一個月）⛔ 不偵測**：要判定漏了交易日，需要獨立的交易日依據，本案沒有。前端只列實際的首尾日期與根數（第三節第 3 點）。
* candles 的讀取仍只走 `python/db.py`（Python 端唯一的 candles 進入點）。

## 二、現況（2026-11-19 讀碼）

* **backtrader 路徑**：`fetch_candles(symbol, timeframe, limit=1000)` 取最近 1000 根，`_to_dataframe` 用 `df.loc[start_date:end_date]` 切，
  切完才餵給 Cerebro。**目前沒有暖身**：指標從範圍內第一根開始冷啟動。`BreakoutV1` 的 SMA(5／20)、RSI(14)、量的 SMA(20) 由 minperiod 擋住前幾根；
  `next()` 另外要求近 60 根（`SR_LOOKBACK`）才判斷。多檔時全部載入同一個 Cerebro，但策略只看 `self.data`（第一檔），其他檔⛔ 不會交易。
* **modular 路徑**：`limit=2000`，同樣先切再跑，而且是一檔取完就跑、再取下一檔。`BacktestEngine.run` 在資料不到 `min_bars + 2` 根時直接回空報告；
  否則從第 `min_bars` 根開始迴圈，權益曲線在迴圈裡寫入，所以範圍的前 `min_bars` 根⛔ 不在權益曲線裡。S/R 元件看最近 `lookback` 根（20～60），
  Wilder 平滑是遞迴的，結果跟歷史從哪一根開始有關。
* **統計**：backtrader 的 `total_return` 用 broker 期初期末資產；`annual_return` 用 `Returns`、`sharpe_ratio` 用 `SharpeRatio`（它內部再用
  `TimeReturn` 子分析器算每期報酬）、`max_drawdown` 用 `DrawDown`，三個都從第一根資料開始算。modular 的三個統計由權益曲線算。
* **舊的截斷不只發生在「範圍內超過 N 根」**：最近 N 根是從整段歷史算的。例如範圍內有 600 根、範圍結束之後還有 800 根，backtrader 只留得下範圍內最後 200 根。
* **沒資料的處理**：某一檔整段沒資料就略過並記 warning；全部沒資料就丟 `No data loaded for any symbol in the given date range`。
* **輸入驗證**：Go 的 `POST /backtest` 只檢查 `start_date`、`end_date` 不是空字串；python 端沒有檢查。
* **schema 等待**：python 兩個服務啟動時用 `_schema_ready` 探測 `backtest_jobs.execution_owner`（I-134），等到之後才收尾、才接任務。
* **結果的寫入與讀取**：python 的 `db_writer` 寫 `backtest_results`（結果、明細與 `done` 同一個交易）。Go 的 `UpsertResult` 已經沒有正式的呼叫端；
  `GET /api/v1/backtest/:job_id` 回 `{"job", "result", …}`，`result` 是 `GetResult` 讀的那一列（還沒完成時是 null）。`GET /api/v1/backtest` 只回任務清單，⛔ 沒有結果。
* **前端**：`Backtest.svelte` 開啟時**預設勾選整個自選清單**，送出時 timeframe 固定 `1d`；策略清單在 `lib/api/backtest.ts` 的 `BACKTEST_STRATEGIES`。
  Go 端沒有策略清單（策略名稱照收，由 python 判斷）。

## 三、修法

### 1. 取資料：`db.py` 新增依日期範圍、有上限的讀法

* 新增 `fetch_candles_range(symbol, timeframe, start_date, end_date, warmup_bars, max_range_bars, adjusted=True, conn=None)`，回傳暖身段與範圍段：
  * **參數 `budget`**：這一檔還能用的根數（任務的總額度扣掉前面各檔已經用掉的，第 4 點）。
  * **先取範圍段**：`ts` 在台北 `start_date` 00:00（含）到 `end_date` 隔天 00:00（不含）之間，依時間排序，`LIMIT budget + 1`。
    上界沿用 `as_of_cutoff_epoch`，下界用同一個表示式（跟回傳的 `timestamp` 同源，比照 I-100）。
    取到 `budget + 1` 根就代表超過，⛔ 不再查暖身段。
    **範圍段是 0 根**：這一檔直接回「略過」（`skipped=true`、`warmup_bars=0`），⛔ 不查暖身段、⛔ 不扣額度，接著處理下一檔。
    否則前一檔剛好用完額度時，下一檔即使範圍內完全沒資料，也會因為暖身段查到一根舊 K 棒而讓整筆任務失敗。
  * **再取暖身段**：用範圍段之後**剩下的額度**限制。`ts` 在台北 `start_date` 00:00 之前，取最近 `min(warmup_bars, budget − 範圍段根數 + 1)` 根
    （`ORDER BY ts DESC LIMIT`，再反轉）。取到的根數比剩下的額度多 1，就是「範圍段沒超過、加上暖身才超過」，同樣超過。
  * 回傳兩段與「是否超過」；超過時呼叫端讓任務失敗（第 4 點）。**上限由實際讀取本身擋**，⛔ 不先 `COUNT` 再讀，
    兩次查詢之間補進來的資料也就⛔ 突破不了上限；每一檔最多讀 `budget + 1` 根。
  * 還原價的處理跟 `fetch_candles` 共用（抽成內部函式），⛔ 不複製一份。`fetch_candles` 本身⛔ 不改。

### 2. 暖身、交易起點與統計起點（D1）

* **暖身根數 `backtest.warmup_bars = 250`**（config，兩條路徑共用；是現有策略的預設值）。理由：
  * 目前最長的回看是 60 根（`SR_LOOKBACK`、modular 的 S/R 與結構停損 `lookback`），250 根留了很大的餘裕；測試核對每個策略的最長回看 ≤ 250。
  * Wilder 平滑（期數 14）經過 250 根，起點的權重剩 (13/14)^250 ≈ 1e-8。測試比較範圍首根的指標跟「用更長歷史算」的值，差距要在容許值內。
  * 固定根數讓同一個請求的結果⛔ 不會因為 DB 補了更早的資料而改變。
* **兩個起點**，都是「範圍內第一根」（以下稱 `r0`）：
  * **交易起點**：`r0` 之前⛔ 不產生任何訊號或委託。
  * **統計起點**：權益曲線、年化報酬、夏普、最大回撤都從 `r0` 開始算。
* **modular**：
  * 迴圈從 `r0` 開始。在「能評估訊號」之前（`t < min_bars`，只有暖身不足時才會發生），每一根都記入**初始權益**（空手、⛔ 不評估訊號）；
    之後照現在的邏輯評估。所以權益曲線的第一個點一定是 `r0`。
  * 範圍內一直到最後都達不到 `min_bars`：⛔ 沒有交易，權益曲線是範圍內每一根的初始權益，三個統計都是 0（跟現在的空報告數值相同），
    覆蓋範圍標成暖身不足。⛔ 不再因為「資料太短」回一個沒有權益曲線的空報告。
  * 迴圈裡的 `df.iloc[: t + 1]` 包含暖身段，所以範圍首根的指標已經暖身。
* **backtrader**：
  * 整段（暖身＋範圍）餵給 Cerebro。`BaseTWStrategy` 加參數 `trade_start`（`r0` 的台北日期），`BreakoutV1.next()` 在 bar 的**台北日期**早於
    `trade_start` 時直接返回、⛔ 不下單。⚠️ backtrader 的 bar 時間是不帶時區的 UTC（日 K 的台北 00:00 會變成前一天 16:00），
    比較前要先換成台北日期；測試用「首根在台北 00:00」驗證首根當天算在範圍內。
  * 委託在下一根開盤成交，暖身段的 `next()` 不下單，所以暖身段最後一根的形態⛔ 不會在 `r0` 成交。
  * **統計自己算，⛔ 不用 `Returns`、`SharpeRatio`、`DrawDown` 三個分析器**：它們（含 `SharpeRatio` 內部的 `TimeReturn` 子分析器）都從第一根開始算，
    沒有排除暖身的參數。改成用**獨立的分析器**記錄範圍內每一根收盤後的 broker 資產，再用跟三個分析器**相同的公式**算
    （年化的期數、無風險利率與換算、標準差的樣本設定，照 backtrader 原始碼）。
    * ⛔ 不在策略的 `next()` 記：指標還沒達到 minperiod 時，backtrader 呼叫的是 `prenext()`／`nextstart()`，`next()` 不會被呼叫；
      暖身是 0 或很短時，範圍前段會漏記，範圍太短時甚至一根都記不到。分析器的 `prenext`、`nextstart`、`next` 三個都要記
      （只記台北日期 ≥ `trade_start` 的根）。
    * `total_return` 仍是期初期末資產（暖身段沒有交易，結果相同）。

### 3. 覆蓋範圍寫進結果（D2、Q3）

* **新欄位** `backtest_results.data_coverage`（文字，存 JSON 陣列；可為 NULL＝I-137 之前的結果，覆蓋範圍不明）。migration 078，三份 DB 都加。
* 每一檔一筆，**實際首尾日期與暖身是否足夠分開記**，⛔ 不合成一個「完整／不完整」的狀態：

  | 欄位 | 說明 |
  |---|---|
  | `symbol` | 股票代號 |
  | `first_bar`、`last_bar` | 範圍內實際取到的第一根、最後一根的台北日期（`YYYY-MM-DD`）；被略過時是 null |
  | `bars` | 範圍內的根數 |
  | `warmup_bars` | 實際取到的暖身根數（0～250） |
  | `warmup_required` | 需要的暖身根數（250） |
  | `skipped` | 範圍內 0 根、這一檔被略過時是 true（這時 `warmup_bars` 是 0：⛔ 不查暖身段） |

  * `first_bar` 晚於 `start_date`，可能只是起點遇到週末或休市，也可能是那一檔後來才有資料。⛔ 不判定是哪一種，只如實記下日期。
  * 暖身不足（`warmup_bars < warmup_required`）是獨立的事實：例如 DB 的第一根剛好在起點當天，首尾日期完整，但暖身是 0。
* **全部被略過**時照現在丟錯（`No data loaded for any symbol in the given date range`），任務 `failed`。
* **寫入**：python 的 `db_writer` 在同一個交易寫進新欄位。Go 的 `BacktestResult` 加欄位，`GetResult` 讀出來；`UpsertResult` 只為測試補上欄位。
* **API 契約**：`GET /api/v1/backtest/:job_id` 的 `result.data_coverage` 是上表的**陣列**；I-137 之前的結果是 **null**；任務還沒完成時 `result` 本身是 null（不變）。
  python-server 的 `GET /backtest/{job_id}` 同樣回陣列或 null：它用 `SELECT *` 讀結果，⛔ 不能把 DB 裡的 JSON 字串原樣回傳，要先解析。
  `GET /api/v1/backtest` 的任務清單⛔ 不變。Go 的 handler 測試要驗**實際回傳的 JSON**（陣列、null 兩種）。
* **前端**：結果卡片的總表上方，一檔一行，列出實際的首尾日期與根數，例如「2330：2024-01-02～2025-12-31，487 根」。
  暖身不足時在同一行加「暖身不足（120／250 根）」；被略過時寫「區間內沒有資料，已略過」。`data_coverage` 是 null 時不顯示這一區。

### 4. 任務上限（D3）

* **兩個上限**：
  * `max_symbols`：每個任務的標的數。Go 建任務時與 python 都要判斷，所以要有**兩邊共用的設定來源**（見下）。
  * `max_task_bars`：每個任務所有標的的**範圍段＋暖身段**根數合計。只有 python 判斷，放在 python config 的 `backtest` 區段。
  * **數值待實測決定**（第六節第 5 點），寫回本節給使用者確認後才定案。決定前要先在 live 唯讀查自選清單的檔數與歷史任務的最多檔數：
    前端預設勾選整個自選清單，上限太小會擋掉平常的用法。
* **`max_symbols` 的設定來源**：環境變數 `BACKTEST_MAX_SYMBOLS`，Go 與 python 用**同一個預設值**。
  * Go：backend config 的 `backtest.max_symbols`（viper 對應 `BACKTEST_MAX_SYMBOLS`），`SetDefault` 給預設值，設定契約加一條 ≥ 1。
  * python：`config.py` 讀 `BACKTEST_MAX_SYMBOLS`，沒設定（或空字串）就用 `config.yaml` 的 `backtest.max_symbols`；不是 ≥ 1 的整數就啟動失敗。
  * **預設值一致**由 repo 層的檢查腳本比對 Go 的 `SetDefault` 與 python 的 `config.yaml`（第 6 點的同一支腳本）。
  * live 目前**只用預設值**：live 的 compose 裡，python 兩個服務⛔ 沒有 `env_file`，只有固定的 `environment:`。要改值時，backend 的 `.env` 與
    python 兩個服務的 `environment:` 要一起改，這個規則寫進 `development-workflow.md`。dev compose 讓三個服務都能從同一個變數帶入，供非預設值的驗收。
* **標的數**：Go 建任務前與 python 共用入口都檢查，超過時 Go 回 400、python 讓任務 `failed`。
* **總根數**：python 先依序取完**所有**標的的資料再開始跑（modular 改成先取後跑）。每一檔呼叫 `fetch_candles_range` 時帶「剩下的額度」，
  範圍段先扣、再用剩下的額度限制暖身段（第 1 點）。任何一檔超過就讓任務 `failed`，⛔ 不截斷、⛔ 不跑一半。
  * 錯誤訊息只說得出**下限**：查詢在 `額度 + 1` 根就停了，實際總數可能更多。寫成
    `task too large: at least <n> bars (range + warmup) by symbol <symbol>, max <m>`，⛔ 不寫成「共 n 根」。
* 上限⛔ 不在 Go 算根數（要查 DB，留在 python 一處）。

### 5. 日期驗證（Q1）

* **Go**：`POST /backtest` 在建任務之前檢查 `start_date`、`end_date` 是 `YYYY-MM-DD`、是存在的日期（`2025-02-30` 不行）、`start_date` ≤ `end_date`
  （同一天可以），而且兩個日期都⛔ 不能晚於 **2099-12-31**。不符回 400，訊息寫明哪一個欄位。
  * **最大日期 2099-12-31**：取數的上界是「結束日的隔天 00:00（台北）」，`9999-12-31` 是合法日期，但加一天會溢位。
    Go 與 python 共用同一個常數，由第 6 點的 repo 層腳本比對兩邊的值。
* **python**：兩條執行路徑（server 與 worker）共用的入口做同樣的檢查，不符就讓任務 `failed`：繞過 Go 直接寫進 DB 的、或修正之前已經排隊的任務，都會明確失敗。

### 6. `breakout_v1` 限單檔（Q4）

* backtrader 策略（`STRATEGY_MAP`，目前只有 `breakout_v1`）只接受 1 檔：
  * **前端**：見第 9 點。
  * **Go**：`POST /backtest` 的策略是 backtrader 策略、而且標的超過 1 檔時回 400。Go 需要一份 backtrader 策略的名單。
  * **python**：共用入口做同樣的檢查（python 的 `STRATEGY_MAP` 是準）。
  * **名單一致**：新增 repo 層的檢查腳本（比照 `scripts/check-job-names.sh`），比對 python 的 `STRATEGY_MAP`、Go 的 backtrader 名單與
    前端的單選名單，順便比對 `max_symbols` 的兩個預設值；不一致就失敗。⛔ 不放在 pytest：pytest 的 container 只掛載 `python/`，讀不到 Go 與前端。
    腳本掛進 `frontend/scripts/test.sh`（`check-job-names.sh` 掛在那裡）。
* modular 繼續支援多檔。

### 7. schema 等待加上 migration 078

* python 兩個服務的 `_schema_ready` 除了 `backtest_jobs.execution_owner`，再探測 `backtest_results.data_coverage`。兩個都在才算就緒，
  之後才收尾、才接任務（server 在就緒前⛔ 不接 `POST /backtest`，沿用 I-134 的時序）。⛔ 不在 python 端建欄位，⛔ 不改成「寫入時才報錯」
  （那會讓正常任務跑完整段才因為上版時序變成 `failed`）。
* 驗收要涵蓋「077 已在、078 還沒套用」：兩個服務都要等，078 套用之後才接任務。

### 8. 既有資料與新舊界線（D4）

* ⛔ 不改、⛔ 不回補既有結果；新欄位是 NULL。
* **部署前在 live 唯讀估算**，給每一筆任務（`done` 與 `failed`）、每一檔：
  * 以任務的 `started_at` 為準（沒有就用 `created_at`），重建當時看得到的最近 N 根：K 棒的 `ts`（行情時間）**而且** `created_at`（入庫時間）
    都早於那個時間，取最近 1000／2000 根（依策略路徑）。再數**範圍內**有幾根落在這 N 根之外（比 N 根裡最早的一根還早）。
    大於 0 就是可能被截掉過，記下被排除的根數。
    * 只看 `ts` 會誤判：行情時間早、但晚幾天才補進 DB 的 K 棒，當時其實還不在。
  * ⚠️ 這仍是估算：K 棒被刪掉、或事後被改寫（upsert 只更新價量、⛔ 不改 `created_at`）都無法完整還原當時的狀態。
  * 另外列出 `breakout_v1` 送了多檔的任務：只有「**第一檔成功載入的標的**」會交易（排在前面但範圍內沒資料的會先被略過），結果⛔ 不代表其他檔。
* 部署後記下 commit、python image、啟動時間當界線，歸檔到 `database-schema.md`（同 I-136 的做法）。

### 9. 前端：選檔與明確拒絕的顯示

* **選檔**：
  * 首次載入時，預設策略是 `breakout_v1`（backtrader）→ ⛔ 不再預設全選，標的是空的，提示「backtrader 版一次只能回測一檔，請選一檔」。
    預設策略是 modular 時才照現在全選自選清單。
  * 從 modular 切到 backtrader 策略、而且已選超過 1 檔時：清空選擇並顯示同一個提示，讓使用者重新選，⛔ 不自動挑一檔。
  * backtrader 策略下，勾選改成單選（選新的一檔就取代舊的）。從 backtrader 切回 modular 時保留目前的選擇、恢復多選。
* **送出被拒絕（400）**：顯示後端回的原因（日期格式、日期不存在、開始日晚於結束日、標的數超過、backtrader 多檔），
  ⛔ 不再一律顯示「送出回測失敗，請稍後再試」。其他錯誤（例如 5xx、連不上）維持原本的訊息。
* **背景任務失敗**：選到 `failed` 的任務時，結果區顯示「回測失敗」與**可公開的原因**（第 10 點的代碼對照成中文），
  ⛔ 不再顯示「尚未完成，稍後會自動刷新」。`pending`／`running` 才顯示「尚未完成」。
  非預期的錯誤、以及 I-137 之前留下的未分類紀錄，顯示通用訊息「回測失敗（內部錯誤）」，⛔ 不顯示原始文字。

### 10. 回測任務的錯誤一律先分類

* **現況的問題**：server 與 worker 失敗時都把 `str(exc)` 寫進 `backtest_jobs.error`，`GET /api/v1/backtest` 與 `/:job_id` 原樣回傳。
  原始例外可能帶 SQL、參數或其他內部細節。`architecture.md`「寫入失敗的一致性契約」已經規定「外來錯誤一律先分類、分類不出來一律 `internal_error`」，
  但回測任務的 `error` 不在它目前的適用範圍裡。本案把它納入。
* **寫入（python）**：`backtest_jobs.error` 只寫封閉值域的代碼，可以帶本專案自己組的安全參數：

  | 代碼 | 什麼時候 |
  |---|---|
  | `invalid_date:<欄位>` | 日期格式錯、日期不存在、晚於 2099-12-31 |
  | `start_after_end` | 開始日晚於結束日 |
  | `too_many_symbols:<n>:<max>` | 標的數超過上限 |
  | `backtrader_single_symbol:<n>` | backtrader 策略送了多檔 |
  | `task_too_large:<symbol>:<至少 n>:<max>` | 總根數（含暖身）超過上限 |
  | `no_data_in_range` | 所有標的在範圍內都沒有資料 |
  | `unknown_strategy` | 策略名稱不認得 |
  | `interrupted_on_restart` | 既有（I-134），不變 |
  | `internal_error` | 其他所有例外（含交易明細核對失敗等） |

  * 已知的錯誤用專屬的例外類別帶出代碼；其他例外一律寫 `internal_error`，原始例外只進 log（`exc_info`）。server 與 worker 用同一個轉換函式。
  * 參數只能是本專案自己組的值（欄位名、整數、股票代號），⛔ 不放使用者輸入的原始字串或例外的文字。
  * Go 取消任務寫的 `cancelled by user` 是既有的固定文字，照舊。
* **讀出**：會把 `error` 帶進回應的端點有三個，都在回傳之前比對**同一份允許規則**（上表的格式與 `cancelled by user`）；
  不符合的（I-137 之前留下的原始文字）一律改回 `internal_error`。DB 的舊資料⛔ 不改。
  * Go：`GET /api/v1/backtest`（任務清單）與 `GET /api/v1/backtest/:job_id`。
  * python：python-server 的 `GET /backtest/{job_id}`（`_backtest_job_payload` 組回應的地方）。
  * **兩份規則一致**：Go 與 python 各有一份測試案例檔（輸入的 `error` → 預期的輸出，含合法代碼、`cancelled by user`、`interrupted_on_restart`、
    含 SQL 的舊字串、空值），內容必須相同，由第 6 點的 repo 層腳本比對兩個檔案；兩邊的單元測試各自跑全部案例。
* **顯示（前端）**：代碼對照成中文（例如 `task_too_large` →「範圍太大：2330 至少 6,001 根（含暖身），上限 6,000 根」）；
  `internal_error` 與不認得的代碼顯示通用訊息。

## 四、受影響的檔案、資料流與 contract

| 位置 | 改動 |
|---|---|
| `python/db.py` | 新增 `fetch_candles_range`；還原價處理抽成共用的內部函式 |
| `python/config.py`、`python/config.yaml` | `backtest.warmup_bars`（250）、`backtest.max_symbols`、`backtest.max_task_bars`（待實測） |
| `python/backtest/engine.py` | 共用入口的驗證（日期、標的數、backtrader 單檔）、新讀法、上限、`trade_start`、自己算的統計、覆蓋範圍 |
| `python/backtest/strategy/base.py`、`breakout_v1.py` | `trade_start` 與範圍外不下單、記錄範圍內的資產 |
| `python/backtest/modular/service.py`、`backtester.py` | 先取後跑、新讀法、上限、迴圈起點與初始權益、覆蓋範圍 |
| `python/backtest/db_writer.py` | 寫 `data_coverage`；`_schema_ready` 加探測 078 |
| `python/http_server.py`、`python/worker.py` | 失敗時寫分類過的代碼（共用的轉換函式），原始例外只進 log；python-server 的 GET 回傳前套用允許規則、解析 `data_coverage` |
| 兩份錯誤案例檔（Go 與 python 各一份） | 允許規則的共同案例，repo 層腳本比對內容相同 |
| migration 078（postgres／sqlite／mysql） | `backtest_results.data_coverage` |
| `backend/internal/api/handler`、`backend/internal/backtest`、`backend/internal/store` | 日期（含最大日期）、標的數與 backtrader 單檔的 400；backtrader 名單；`BacktestResult` 加欄位、讀寫；回傳 `error` 前比對允許清單 |
| `backend/internal/config`、`backend/config.yaml`、`.env.example` | `backtest.max_symbols`（`BACKTEST_MAX_SYMBOLS`）與設定契約 |
| `docker-compose.dev.yml` | 三個服務都能帶入 `BACKTEST_MAX_SYMBOLS`（非預設值的驗收） |
| `frontend`（`lib/api/backtest.ts`、`routes/Backtest.svelte`） | 型別、覆蓋範圍的顯示、選檔規則、400 原因、失敗任務的原因 |
| repo 層的檢查腳本（新增）、`frontend/scripts/test.sh` | backtrader 名單與 `max_symbols` 預設值的一致性 |
| `scripts/smoke-dev-backtest-claim.sh` | schema 時序情境加上 078；範圍的用法（第六節第 11 點） |
| 測試、文件 | 見第六、七節 |

**資料流**：前端送出 → Go 驗證（日期、標的數、backtrader 單檔）後建任務 → server／worker（schema 含 078 才就緒）認領 → python 共用入口再驗證一次 →
依序取所有標的的暖身＋範圍（總根數受上限）→ 跑回測（`r0` 之前不交易、統計從 `r0` 起）→ 結果＋明細＋覆蓋範圍同一個交易寫入 → Go 讀結果 → 前端顯示。

**contract 的變化**

* DB：`backtest_results` 多一個可為 NULL 的欄位 `data_coverage`。
* API：`GET /api/v1/backtest/:job_id` 的 `result.data_coverage`（陣列或 null）；`POST /api/v1/backtest` 多幾種 400（日期、標的數、backtrader 多檔）。
* 結果的數字：**所有回測的結果都可能跟以前不同**（D1 的代價）——有暖身之後，範圍前段的指標不再冷啟動、訊號可能提早出現；
  統計從範圍首根起算（modular 原本略過前 `min_bars` 根）；原本被截掉的，現在從起點開始跑。

## 五、主要風險與回滾

| 風險 | 處理 |
|---|---|
| 範圍很長、標的很多時記憶體或時間暴增 | 任務的標的數與總根數上限（含暖身），用實測決定；超過明確失敗 |
| 計數與讀取之間補了資料 | 上限由讀取本身的 `LIMIT max + 1` 擋，⛔ 不先 `COUNT` |
| backtrader 的統計改成自己算時公式變了 | 測試鎖住「沒有暖身時等於原本三個分析器」 |
| 暖身段的訊號漏進範圍內；或首根因為 UTC 被判成範圍外 | 範圍外不下單；交易門檻用台北日期，測試包含首根在台北 00:00 |
| 暖身不足時權益曲線少了範圍前段 | 能評估訊號之前逐根記初始權益；測試整段都達不到 `min_bars` 的情況 |
| 上版時序：python 比 migration 078 先起來 | schema 等待加上 078，就緒前不接任務 |
| Go、前端、python 的 backtrader 名單不一致；Go 與 python 的 `max_symbols` 不一致 | repo 層的檢查腳本比對名單與預設值；dev 用非預設值驗收三個服務的判定一致 |
| backtrader 的權益只在 `next()` 記，漏掉範圍前段 | 獨立分析器的 `prenext`、`nextstart`、`next` 都記；測試暖身 0、範圍比 minperiod 短 |
| 暖身段吃掉額度後突破總根數上限 | 範圍段先扣，暖身段用剩下的額度限制；測試「範圍段沒超過、加上暖身才超過」 |
| 前端預設勾選整個自選清單，上限擋掉平常的用法 | 上限決定前先查 live 的自選清單檔數與歷史任務的最多檔數 |
| 失敗原因把原始例外（SQL、參數）帶到 API 與畫面 | python 只寫分類過的代碼；Go 回傳前比對允許清單，舊資料改回 `internal_error`；測試原始例外不出現在 API 與畫面 |
| 前一檔用完額度，下一檔範圍內沒資料卻因暖身段讓任務失敗 | 範圍段 0 根就略過、⛔ 不查暖身段 |
| 回滾 | 三個服務退回 `pre-i137` 的 image；migration 只加可為 NULL 的欄位，⛔ 不用退（舊程式不讀不寫它；舊 python 的 schema 等待也不看它） |

## 六、測試與驗證

1. **`db.py`**（SQLite）：暖身段最多 250 根且都在起點之前；範圍段含頭含尾（台北日期，含起點當天 00:00 與終點當天）；起點是假日時從下一個交易日開始；
   範圍段超過上限時回傳 `max + 1` 根（呼叫端據此拒絕）；還原價跟 `fetch_candles` 一致。
2. **截斷的回歸（兩條路徑都測；issue 要求的驗收）**：合成的日 K 超過 1000（modular 超過 2000）根。
   * **部分被截掉**：起點早於最近 N 根、終點在 N 根之內 → `first_bar` 是起點後的第一個交易日、範圍內根數等於全部。
   * **範圍之後還有很多根**：範圍內 600 根、之後 800 根（backtrader）→ 範圍內 600 根全部用到。
   * **整段在最近 N 根之前** → 正常跑完。
   * 三個案例在舊寫法（`limit=1000`／`2000`）放回去時都要失敗。
3. **暖身與兩個起點**：
   * 範圍外⛔ 沒有任何成交；暖身段最後一根放一個會觸發的形態，⛔ 不能在 `r0` 成交。
   * backtrader：首根在台北 00:00 時，首根當天可以產生訊號（⛔ 被當成前一天）。
   * 範圍首根的指標跟「用更長歷史算」的值差在容許值內，而且⛔ 等於冷啟動的值（證明暖身有用到）；每個策略的最長回看 ≤ 250。
   * modular：權益曲線的第一個點是 `r0`；暖身不足時，達到 `min_bars` 之前的點都是初始權益；整段都達不到 `min_bars` 時⛔ 沒有交易、
     權益曲線涵蓋範圍內每一根、三個統計是 0、覆蓋範圍標暖身不足。
4. **統計**（暖身本身可能改變訊號與成交，所以比較新舊統計時要讓**成交與權益序列相同**）：
   * 用照腳本下單的策略（下單時機不看指標），同一組成交：(a) 只餵範圍段、掛原本三個分析器；(b) 餵暖身＋範圍、用新的分析器與 `trade_start`。
     兩者的 `annual_return`、`sharpe_ratio`、`max_drawdown` 相同。
   * 同一次執行同時掛原本三個分析器與新的分析器（暖身 0）：數值相同。
   * 暖身 0、範圍比 `BreakoutV1` 的 minperiod 還短：新的分析器仍記到範圍內每一根（走的是 `prenext`），三個統計是 0。
5. **上限**：
   * 標的數等於上限通過、多 1 檔在 Go 回 400、在 python 讓任務 `failed`。
   * 總根數（含暖身）等於上限通過、多 1 根失敗，訊息寫「至少 n 根」、上限與代號；超過時⛔ 沒有開始跑任何一檔。
   * **範圍段沒超過、加上暖身才超過**（例如剩 10 根額度、範圍段 10 根、暖身有 250 根）→ 失敗；暖身段的查詢最多取剩下的額度 + 1 根。
   * 多檔時第二檔才超過 → 失敗，第一檔⛔ 沒有開始跑。
   * **額度剛好用完，下一檔範圍內沒有資料、只有暖身段的歷史** → 那一檔是 `skipped`（`warmup_bars` 0）、任務照常完成；暖身段的查詢⛔ 沒有執行。
   * **`max_symbols` 用非預設值**（dev 的三個服務都設成 2）：3 檔的請求 Go 回 400；直接寫進 DB 的 3 檔任務，server 與 worker 都讓它 `failed`；2 檔照常。
   * **實測決定數值**：先在 live 唯讀查自選清單的檔數與歷史任務的最多檔數；dev 用 2330 的完整日 K（4,930 根）與合成的多檔資料，
     在 mem-guard 下量兩條路徑的執行時間與記憶體高點（單檔長範圍、多檔、候選上限的 1.5 倍）；數值與量測結果寫回第三節第 4 點給使用者確認。
6. **覆蓋範圍**：首尾日期與根數正確；起點是週末時 `first_bar` 是下一個交易日、⛔ 不算暖身不足；DB 第一根剛好在起點當天時首尾完整但暖身是 0；
   暖身足夠但範圍中間缺一段時照樣只記首尾（⛔ 不偵測缺漏，測試確認這是設計）；被略過的一檔；全部被略過時丟錯。
   JSON 寫進 DB 再由 Go 讀回，handler 測試驗實際的 API JSON（陣列、null）。
7. **日期驗證**：格式錯、日期不存在、開始日晚於結束日、晚於 2099-12-31（例如 `9999-12-31`）→ Go 回 400、python 讓任務 `failed`；
   同一天、`2099-12-31` 通過；repo 層腳本比對兩邊的最大日期。
7-1. **錯誤分類**：每一種已知錯誤寫出上表的代碼；非預期的例外（例如訊息裡帶 SQL 的假例外）寫 `internal_error`，原始文字只在 log。
   三個讀出端點（Go 的清單與 `/:job_id`、python-server 的 `GET /backtest/{job_id}`）對不符合允許規則的舊 `error`（例如含 SQL 的字串）都回傳 `internal_error`，
   合法代碼、`cancelled by user`、`interrupted_on_restart` 原樣回傳。python 端沿用 `test_http_server_backtest_get.py` 的既有測試加案例；
   同一個測試也驗 `result.data_coverage` 是陣列或 null（⛔ 不是字串）。兩份案例檔不一致時 repo 層腳本失敗。
   API 的回應與前端畫面都⛔ 不能出現原始例外的文字（測試直接搜尋那段文字）。
8. **backtrader 單檔**：`breakout_v1` 兩檔 → Go 回 400、python 讓任務 `failed`；modular 兩檔照常；repo 層腳本比對三份名單與 `max_symbols` 的預設值
   （突變：任一份名單少一個或預設值不同 → 腳本失敗）。
9. **前端**（vitest）：首次載入、預設策略是 `breakout_v1` 時沒有預選；modular 多選後切到 `breakout_v1` 會清空並提示；backtrader 下選新的一檔會取代舊的；
   切回 modular 保留選擇；400 顯示後端的原因；5xx 顯示原本的訊息；`failed` 的任務顯示代碼對照的中文，`internal_error` 與不認得的代碼顯示通用訊息，
   `pending`／`running` 顯示「尚未完成」；
   `data_coverage` 的一檔一行（首尾日期、暖身不足、已略過）與 null 時不顯示。
10. **schema 等待**：077 在、078 還沒套用 → 兩個服務都不就緒、⛔ 不接任務；078 套用後就緒（單元測試，加上 `smoke-dev-backtest-claim.sh` 的 schema 時序情境）。
11. **migration**：`scripts/test-postgres-migrations.sh`、`scripts/test-mysql-migrations.sh`（只驗 DDL）、SQLite 的單元測試。
12. **突變**：放回 `limit=1000`／`2000`、拿掉範圍外不下單、交易門檻改用 UTC 日期、統計改回三個分析器、modular 的權益曲線改回從 `min_bars` 起記、
    拿掉 schema 探測的 078、拿掉讀取的 `LIMIT`、暖身段改回固定取 250 根（不扣額度）、分析器只在 `next` 記，各自要有測試失敗。
13. **回歸**：`python/scripts/test.sh`、`backend/scripts/test.sh`、前端的測試腳本全部通過。
14. **dev 實跑**：`scripts/smoke-dev-backtest-claim.sh` 的 `trades` 情境，範圍改成比 1000 根長（例如 2330 的 2015-01-01～2025-12-31），
    確認範圍內根數超過 1000、`data_coverage` 寫入、暖身 250；再跑一次起點早於 2006-10-18 的範圍，確認 `first_bar` 是 2006-10-18、暖身是 0。
    要不要把範圍做成腳本的參數、或加新情境，實作時決定並寫進實作備註。

## 七、完成後要歸檔的位置

* `docs/architecture.md`「寫入失敗的一致性契約」：適用範圍加上 `backtest_jobs.error`，合法形式表加一列（第三節第 10 點的代碼）。
* `docs/architecture.md` 的回測流程：依日期範圍取資料、暖身 250 根、交易起點與統計起點、任務上限、日期驗證、backtrader 限單檔、schema 等待含 078。
* `docs/database-schema.md`：`backtest_results.data_coverage` 的格式；範圍內缺漏不偵測；新舊界線（部署後補上）。
* `docs/api-reference.md`：`GET /api/v1/backtest/:job_id` 的 `result.data_coverage`；`POST /api/v1/backtest` 的各種 400；Go 的兩個 GET 與 python-server 的 GET 的 `error` 只會是分類過的代碼。
* `docs/development-workflow.md`：smoke 腳本的範圍用法與 078 的 schema 時序（若有改）；`BACKTEST_MAX_SYMBOLS` 要三個服務一起改；新的檢查腳本。
* `python/config.yaml`：新設定的註解。
* `docs/issue.md` I-137 改成「已實作／待 review」；review 確認後收斂。I-138 另行處理。

## 八、v1 的 Q1～Q4 裁決（2026-11-19）

* **Q1** 日期驗證一起做：Go 檢查 `YYYY-MM-DD`、日期存在、開始日 ≤ 結束日（同一天可以），失敗回 400；python 在兩條執行路徑的共用入口做同樣的檢查。→ 第三節第 5 點。
* **Q2** 暖身預設 250 根、兩條路徑共用；測試核對最長回看，並比較範圍首根的指標與更長歷史的結果。→ 第三節第 2 點。
* **Q3** 提示放在總表上方、一檔一行，列實際的首尾日期，分開說明暖身不足與「區間內沒有資料、已略過」；首根晚於開始日只顯示實際日期，⛔ 不判成缺漏。→ 第三節第 3 點。
* **Q4** I-137 先把 `breakout_v1` 限成單檔（前端單選與提示、Go 400、python 檢查），modular 繼續多檔；真正的多檔回測另立 I-138、分開 review。→ 第三節第 6 點。

## 版本紀錄

| 版本 | 日期 | 內容 |
|---|---|---|
| v1 | 2026-11-19 | 初版。使用者裁決 D1～D4（都採建議）。 |
| v1.1 | 2026-11-19 | review [P2]×5：①歷史盤點改成以 `started_at` 重建當時的最近 N 根，再數範圍內被排除的根數（範圍之後的 K 棒也會擠掉範圍內的）；②覆蓋範圍拿掉合成的狀態，首尾日期與暖身是否足夠分開記，範圍內缺漏⛔ 不偵測（沒有獨立的交易日依據）；③交易起點與統計起點分開定義：modular 在能評估訊號之前逐根記初始權益、整段達不到 `min_bars` 時也有權益曲線，backtrader 統計自己算（`SharpeRatio` 的 `TimeReturn` 也會算到暖身），交易門檻用台北日期；④上限改成每任務的標的數與總根數（含暖身），由讀取本身的 `LIMIT` 擋、先取後跑；⑤migration 078 列入兩個 python 服務的 schema 等待。API 契約寫明 `result.data_coverage` 是陣列或 null、列表不變。Q1～Q4 裁決併入（日期驗證、暖身 250、前端顯示、`breakout_v1` 限單檔並另立 I-138）。 |
| v1.2 | 2026-11-19 | review [P2]×4：①backtrader 的範圍內權益改由獨立分析器在 `prenext`、`nextstart`、`next` 都記（策略的 `next()` 在指標達到 minperiod 前不會被呼叫），統計的比較改成成交與權益序列相同的對照；②總根數上限：範圍段先扣，暖身段用剩下的額度限制，測「範圍段沒超過、加上暖身才超過」，錯誤訊息寫「至少 n 根」；③`max_symbols` 的共用設定來源是 `BACKTEST_MAX_SYMBOLS`、Go 與 python 同一個預設值（live 的 python 服務沒有 `env_file`，只用預設值），dev 用非預設值驗收三個服務；④前端：首次載入與切到 backtrader 策略時不預選、要求重新選檔，顯示 400 的原因與失敗任務的 `error`。[P3]×2：名單與預設值的比對改成 repo 層的檢查腳本（pytest 讀不到 Go 與前端），掛進 `frontend/scripts/test.sh`；歷史盤點加上 `candles.created_at` 的條件，舊版 `breakout_v1` 寫成只交易「第一檔成功載入的標的」。 |
| v1.3 | 2026-11-19 | review [P2]×2：①範圍段 0 根就直接略過（`skipped`、`warmup_bars` 0），⛔ 不查暖身段、⛔ 不扣額度，測「額度剛好用完，下一檔只有暖身歷史」；②失敗原因只顯示可公開的內容：python 寫封閉值域的代碼（其他例外一律 `internal_error`，原始例外只進 log），Go 回傳前比對允許清單、舊的原始文字改回 `internal_error`，前端代碼對照中文，測試原始例外不出現在 API 與畫面；`backtest_jobs.error` 納入 `architecture.md` 的錯誤分類契約。另外補上日期上界：最大日期 2099-12-31，Go 與 python 共用（`9999-12-31` 加一天會溢位）。 |
| v1.4 | 2026-11-19 | review [P2]×1：python-server 的 `GET /backtest/{job_id}` 也會回傳 `error`，同樣套用允許規則；Go 與 python 的規則用兩份內容相同的案例檔鎖住（repo 層腳本比對）；python-server 的 GET 回傳的 `data_coverage` 要先解析成陣列。 |
