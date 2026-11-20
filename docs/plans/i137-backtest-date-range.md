# I-137 計畫書 v1.7：回測改成依日期範圍取 K 棒（暖身、覆蓋範圍標示、任務上限、日期驗證、breakout_v1 限單檔）

> 狀態：v1.7（v1.4 已確認並實作；v1.5 是實作中的裁決：每一檔的上限與資料不足的代碼；v1.6 是實作 review 的修正；v1.7 是 review 定下 `max_task_bars` 35,000）。已實作／待 review。對應：`docs/issue.md` I-137。
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
    * （v1.6）**年化報酬與夏普以日為期**：原本的 `Returns`／`SharpeRatio` 都是 `timeframe=Days`，先取每一天最後一根的資產再算；
      最大回撤（`DrawDown`）是逐根算。分 K 一天多根時，⛔ 不能把每一根當成一天、照 252 年化（review 2026-11-19：那樣即使沒有暖身、成交完全相同，
      1m 的結果也會變）。對照測試加上一天多根的分 K。

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

* **三個上限**（v1.5 加上每一檔的上限）：
  * `max_symbols`：每個任務的標的數。Go 建任務時與 python 都要判斷，所以要有**兩邊共用的設定來源**（見下）。
  * `max_symbol_bars`（v1.5）：**每一檔**的範圍段＋暖身段根數（＝實際送進引擎的資料量；暖身取滿 250 根時，範圍最多 4,750 根）。只有 python 判斷。
  * `max_task_bars`：每個任務所有標的的**範圍段＋暖身段**根數合計，限制多檔的總量。只有 python 判斷。
* **數值**（2026-11-19 實測與 live 唯讀查詢）：
  * live：自選清單 11 檔；歷史任務每個策略最多送過 5 檔；自選清單的日 K 每檔最深 4,935 根、11 檔合計 29,978 根；另有 `1m`，每檔最深 22,652 根、合計 199,356 根
    （前端只送 `1d`，但 API 接受其他 timeframe）。
  * dev 量測（合成日 K、`--memory=700m --cpus=1`）：記憶體高點都在 111～121MB，⛔ 不是瓶頸；**時間是瓶頸**，modular 跟每一檔根數的平方成正比
    （`pullback_atrchannel_structural_v1` 單檔 1,500／2,750／5,250／10,250 根為 5.6／19.5／76／268 秒，另兩種 composite 5,250 根約 60 秒），
    backtrader 的 `breakout_v1` 10,250 根 1.7 秒。只靠合計擋不住單一檔很長（例如 6 萬根全給一檔，modular 估計超過 2 小時），所以加上每一檔的上限。
  * **`max_symbols` = 20**（自選清單全選 11 檔不會被擋）；**`max_symbol_bars` = 5,000**（使用者裁決；日 K 的整段歷史放得下，modular 單檔約 60～76 秒）；
    **`max_task_bars` = 35,000**（v1.7，review 的建議、使用者採用；原本的候選值 60,000）：11 檔日 K 整段歷史 29,978 根放得下、還有約 5,000 根餘裕；
    最壞情況 7 檔 × 5,000 根，依下面的實測（耗時跟檔數成正比）約 8 分鐘。等 T-086 改善 modular 效能後再評估提高。
    （v1.6 實測多檔，取代原本用單檔外推的「約 15 分鐘」）`pullback_atrchannel_structural_v1`：4 檔 × 2,750 根（含暖身）82 秒（單檔 19.5 秒 × 4）；
    **12 檔 × 5,000 根＝60,000 根（原候選值的最壞情況）830 秒（約 13.8 分鐘）、記憶體高點 144MB**——耗時跟檔數成正比。
  * modular 的平方成長另列 `todo.md` T-086。
* **`max_symbols` 的設定來源**：環境變數 `BACKTEST_MAX_SYMBOLS`，Go 與 python 用**同一個預設值**。
  * Go：backend config 的 `backtest.max_symbols`（viper 對應 `BACKTEST_MAX_SYMBOLS`），`SetDefault` 給預設值，設定契約加一條 ≥ 1。
  * python：`config.py` 讀 `BACKTEST_MAX_SYMBOLS`，沒設定（或空字串）就用 `config.yaml` 的 `backtest.max_symbols`；不是 ≥ 1 的整數就啟動失敗。
  * **預設值一致**由 repo 層的檢查腳本比對 Go 的 `SetDefault` 與 python 的 `config.yaml`（第 6 點的同一支腳本）。
  * live 目前**只用預設值**：live 的 compose 裡，python 兩個服務⛔ 沒有 `env_file`，只有固定的 `environment:`。要改值時，backend 的 `.env` 與
    python 兩個服務的 `environment:` 要一起改，這個規則寫進 `development-workflow.md`。dev compose 讓三個服務都能從同一個變數帶入，供非預設值的驗收。
* **標的數**：Go 建任務前與 python 共用入口都檢查，超過時 Go 回 400、python 讓任務 `failed`。
* **總根數與每一檔**：python 先依序取完**所有**標的的資料再開始跑（modular 改成先取後跑）。每一檔呼叫 `fetch_candles_range` 時帶
  `min(每一檔的上限, 任務剩下的額度)`，範圍段先扣、再用剩下的額度限制暖身段（第 1 點）。任何一檔超過就讓任務 `failed`，⛔ 不截斷、⛔ 不跑一半。
  * **分開回報**（v1.5）：每一檔的上限先擋到 → `symbol_too_large:<代號>:<至少 n>:<max>`（要縮短日期範圍，減少檔數沒有用）；
    任務剩下的額度先擋到 → `task_too_large:<代號>:<至少 n>:<max>`（**先碰到任務的總額度**：要減少檔數或縮短範圍）。
    ⚠️ 查詢在 `額度 + 1` 根就停，所以只知道先碰到哪一個上限：只剩 1,000 根額度、下一檔有 6,000 根時，查到第 1,001 根就回 `task_too_large`，
    ⛔ 不能宣稱「每一檔都合格」（review 2026-11-19）。
  * 錯誤訊息只說得出**下限**：查詢在 `額度 + 1` 根就停了，實際總數可能更多。寫成
    `task too large: at least <n> bars (range + warmup) by symbol <symbol>, max <m>`，⛔ 不寫成「共 n 根」。
* 上限⛔ 不在 Go 算根數（要查 DB，留在 python 一處）。

### 4-1. 資料不足（v1.5；只用於 backtrader）

* 資料（範圍段＋暖身段）比策略指標的 minperiod 還少時，backtrader 預設的 runonce 模式會丟 `IndexError`（實測：`BreakoutV1` 19 根丟錯、20 根可以跑），
  I-137 之前就是這樣，任務以 `internal_error` 失敗。
* 改成**執行前檢查**：實際載入的範圍段＋暖身段根數，對照策略的 `required_bars()`（`BreakoutV1` 是 `max(ma_fast, ma_slow, rsi_period + 1, vol_period)` ＝ 20，
  測試核對它等於 backtrader 實際的 minperiod）。不夠就讓任務失敗：`insufficient_data:<代號>:<取得 n>:<需要 m>`，
  畫面顯示「資料不足：取得 19 根，至少需要 20 根」。
* 暖身沒滿 250 根、但總資料仍夠初始化：照常跑，覆蓋範圍標暖身不足。
* modular ⛔ 不用這個代碼：它在資料不到 `min_bars` 時本來就不評估訊號（第 2 點），不會丟錯。

### 5. 日期驗證（Q1）

* **Go**：`POST /backtest` 在建任務之前檢查 `start_date`、`end_date` 是 `YYYY-MM-DD`、是存在的日期（`2025-02-30` 不行）、`start_date` ≤ `end_date`
  （同一天可以），而且兩個日期都⛔ 不能晚於 **2099-12-31**、⛔ 不能早於 **0001-01-01**（v1.6：Go 的 `time.Parse` 收年份 0000，python 不收、
  PostgreSQL 也會報錯而回 500）。不符回 400，訊息寫明哪一個欄位。
  * （v1.6）Go 與 python 的請求驗證由**同一份案例檔**鎖住（兩邊各一份、內容相同，由 repo 層腳本比對），跟錯誤允許規則的案例檔同一套做法。
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
* **盤點結果**（2026-11-20 10:08 在 live 唯讀執行，live 當時是 `03e7028`；SQL 先在 dev 以合成資料驗過：一般截斷、晚補的 K 棒、1m 與 1d 分開、
  範圍在窗口內、多檔第一檔無資料）：`done`／`failed` 共 9 筆任務、41 組任務×標的，被截掉的 0 組、0 根；`breakout_v1` 多檔 1 筆
  （`bt_20260701_073955_000`，5 檔，實際交易 2330）；pending／running 0 筆。已歸檔到 `database-schema.md` 的 `data_coverage` 新舊界線。

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
  | `task_too_large:<symbol>:<至少 n>:<max>` | 先碰到任務的總額度（所有標的合計，含暖身） |
  | `symbol_too_large:<symbol>:<至少 n>:<max>` | 單一檔（含暖身）超過每一檔的上限（v1.5） |
  | `insufficient_data:<symbol>:<取得 n>:<需要 m>` | backtrader：資料不夠初始化指標（v1.5） |
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
| `python/config.py`、`python/config.yaml` | `backtest.warmup_bars`（250）、`backtest.max_symbols`、`backtest.max_task_bars`（35,000） |
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
   * （v1.5 修改）**總資料足夠初始化、但範圍首根仍在 `prenext`**：例如暖身 5 根＋範圍 30 根（首根是第 6 根，minperiod 20），新的分析器從範圍首根起
     記到每一根；暖身 0、範圍 30 根也一樣（前 19 根走 `prenext`）。v1.4 原本寫的「範圍比 minperiod 還短」現在會是 `insufficient_data`（第三節第 4-1 點）。
5. **上限**：
   * 標的數等於上限通過、多 1 檔在 Go 回 400、在 python 讓任務 `failed`。
   * 總根數（含暖身）等於上限通過、多 1 根失敗，訊息寫「至少 n 根」、上限與代號；超過時⛔ 沒有開始跑任何一檔。
   * **範圍段沒超過、加上暖身才超過**（例如剩 10 根額度、範圍段 10 根、暖身有 250 根）→ 失敗；暖身段的查詢最多取剩下的額度 + 1 根。
   * 多檔時第二檔才超過 → 失敗，第一檔⛔ 沒有開始跑。
   * **額度剛好用完，下一檔範圍內沒有資料、只有暖身段的歷史** → 那一檔是 `skipped`（`warmup_bars` 0）、任務照常完成；暖身段的查詢⛔ 沒有執行。
   * （v1.5）**單一檔超過每一檔的上限** → `symbol_too_large`；**先碰到任務的總額度** → `task_too_large`；**兩種上限都會超過、但先碰到總額度** →
     `task_too_large`；單一檔剛好等於上限可以執行。
   * （v1.5）**資料不足**：minperiod − 1 根 → `insufficient_data`；剛好等於 minperiod 可以執行；範圍很短（例如 3 根）但暖身足夠時照常通過；
     `required_bars()` 等於 backtrader 實際的 minperiod。
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

## 七之後：實作備註（2026-11-19；已改好、未 commit，待 review）

**改了什麼**

* python：`db.fetch_candles_range`（範圍段 `LIMIT 額度 + 1`、範圍段 0 根⛔ 不查暖身、暖身段用剩下的額度限制；還原價抽成 `_apply_adjustment`）；
  `backtest/data_range.py`（兩條路徑共用的取資料：先取後跑、額度、覆蓋範圍）；`backtest/job_errors.py`（請求驗證、對外代碼、允許規則、`MAX_DATE`）；
  `backtest/range_stats.py`（`RangeValue` 分析器與照抄 backtrader 公式的範圍內統計）；`strategy/base.py` 的 `trade_start` 與 `bar_taipei_date`；
  `BreakoutV1.next()` 範圍外不下單；modular 的 `BacktestEngine.run(trade_start_index=…)`；`db_writer` 寫 `data_coverage`、schema 探測加 078；
  server／worker 失敗時寫 `job_error_code(exc)`；python-server 的 GET 套用允許規則、解析 `data_coverage`；config 的 `warmup_bars`／`max_symbols`
  （`BACKTEST_MAX_SYMBOLS` 可覆寫）／`max_task_bars`。
* migration 078（三份）：`backtest_results.data_coverage TEXT`。
* Go：`config.Backtest.MaxSymbols`（`BACKTEST_MAX_SYMBOLS`，預設 20，設定契約 ≥ 1）；`backtest.ValidateRequest`、`PublicJobError`、`MaxDate`、
  `BacktraderStrategies`；`Manager` 帶標的數上限；handler 的 400、兩個 GET 清洗 `error`；`BacktestResult.DataCoverage`（`store.NullJSONText`：
  輸出原本的 JSON 值或 null）。
* 前端：`BACKTRADER_STRATEGIES`／`isBacktraderStrategy`、`DataCoverage` 型別；`lib/backtestErrors.ts`（代碼 → 中文）；`Backtest.svelte`
  的選檔規則、400 原因、失敗任務的原因、覆蓋範圍一檔一行。
* 腳本：`scripts/check-backtest-contract.py`（掛進 `frontend/scripts/test.sh`）；`smoke-dev-backtest-claim.sh`（前置檢查要 078、schema 情境
  先補 077 確認仍在等再補 078、trades 情境核對覆蓋範圍的根數等於 DB 的根數）與它的假 docker 測試；`docker-compose.dev.yml` 三個服務都能帶入
  `BACKTEST_MAX_SYMBOLS`；`.env.example` 加註解說明（平常保持註解）。

**跟計畫的差異**

1. **允許規則用整串比對**：python 的 `re.match` 加 `$` 會放過「代碼後面多一個換行」（`$` 可以匹配結尾換行之前），Go 的 `^…$` 不會。
   python 改用 `fullmatch`，案例檔加兩筆結尾換行的案例鎖住。
2. **既有測試配合「backtrader 限單檔」改寫**：I-135 的兩個多檔測試（`test_multiple_trades_and_symbols`、大額多筆成交）原本走 `run_backtest`
   送兩檔，現在會被擋。前者拆成「單檔多筆」走 `run_backtest`、「`_extract_trades` 走過每一檔」直接用 Cerebro 載兩檔驗；後者多方、空方分兩次跑。
   驗的內容⛔ 不變。
3. **資料比指標期數還短**：backtrader 預設的 runonce 模式直接丟 `IndexError`（`runonce=False` 不會），I-137 之前就是這樣。回報使用者後裁決採 (c)：
   新增 `insufficient_data`（第三節第 4-1 點，v1.5）。v1.4 的「範圍比 minperiod 還短」驗收因此改成「總資料足夠、範圍首根仍在 prenext」。
4. **上限**：量測發現時間（modular 平方成長）才是瓶頸，回報使用者後裁決加上每一檔 5,000 根的上限（第三節第 4 點，v1.5）；
   數值依 live 唯讀查詢與 dev 量測寫回第三節第 4 點，`max_task_bars` 由 v1.7 的 review 定為 35,000。modular 效能另列 `todo.md` T-086。
5. modular 的 `trade_start_index` 預設是 `None`（維持原本的行為），只有 `run_modular_backtest` 會帶值；直接呼叫 `BacktestEngine.run` 的測試與工具不受影響。
6. 前端測試：jsdom 裡結束日期輸入框的初始值是空的，送出前的日期檢查會先擋下，所以送出相關的測試先明確填好兩個日期（跟本案無關的測試環境現象）。

**驗證**

| 項目 | 結果 |
|---|---|
| `python/scripts/test.sh -q`（完整） | rc 0：2154 passed、1 skipped（v1.6 之後）（新增 `test_db_fetch_candles_range.py` 9 項、`test_backtest_date_range.py` 等） |
| `backend/scripts/test.sh`（完整） | 全部 ok |
| `scripts/test-postgres-migrations.sh`、`scripts/test-mysql-migrations.sh` | rc 0、rc 0 |
| `frontend/scripts/test.sh` | svelte-check 0 錯誤、vitest 174 passed、build 成功；dist 有新檔要 `git add`（`check-dist-assets.sh` 的設計） |
| `scripts/check-backtest-contract.py`、`scripts/check-job-names.sh` | 一致 |
| `scripts/test-smoke-dev-backtest-claim.sh` | 68 項通過（原本 60 項） |
| 突變（python） | 12 組都有測試失敗：放回最近 1000 根 6、拿掉範圍外不下單 2、交易門檻改 UTC 1、統計算到暖身段 1、分析器只在 next 記 1、modular 權益曲線從 min_bars 起 2、schema 探測拿掉 078 2、拿掉範圍段的 LIMIT 1、暖身不扣額度 1、空範圍仍查暖身 1、失敗時寫 `str(exc)` 3、python GET 不清洗 error 11。⚠️ 第一次寫的「拿掉 LIMIT」只是停用第一道超過判斷，第二道照樣判超過（等價突變，9 項全過），改成真的拿掉 SQL 的 `LIMIT` 才是有效突變 |
| 突變（v1.5） | 拿掉 `insufficient_data` 檢查 1、每檔上限一律回 `task_too_large` 2、拿掉每檔上限 2 項失敗；`required_bars()` 少算 RSI 的 +1 是等價突變（最大值仍由 SMA(20) 決定），由「等於 backtrader 實際 minperiod」的測試把關 |
| 突變（Go） | GET 不清洗 1、清單不清洗 1、POST 不檢查 1、最大日期不檢查 1、允許規則放行全部 1、`data_coverage` 輸出成字串 1 |
| v1.6 的修正 | 分 K 一天 4 根的新舊對照（同一次執行、有暖身段）都等於原本的分析器；反向測試：把每一根當成一天就不同。突變：拿掉以日分組 → 2 項失敗；拿掉 Go 的 `MinDate` → 共用案例與 handler 的 400 各 1 項失敗。兩種上限都會超過、先碰到總額度 → `task_too_large` |
| 突變（`check-backtest-contract.py`） | 前端名單、Go 名單、`max_symbols` 預設值、最大日期、案例檔，各改一處都 rc 1 |
| 上限實測 | 見第三節第 4 點（live 唯讀查詢 2026-11-19 17:38；dev 合成資料量測） |
| dev 實跑 `smoke-dev-backtest-claim.sh`（預設五個情境） | rc 0：race、server-restart、worker-restart、lost-response、schema（只補 077 時臨時 server 仍在等，補 078 之後才繼續啟動） |
| dev 實跑 `trades`，2330、2015-01-01～2025-12-31 | rc 0：範圍內 2,682 根＝DB 的根數（遠超過舊的 1,000 根）、首根 2015-01-05、暖身 250；17 筆明細與費用核對通過 |
| dev 實跑 `trades`，2330、2005-01-01～2012-12-31 | rc 0：首根 2006-10-19（DB 的第一根）、1,547 根＝DB 的根數、暖身 0；6 筆明細通過 |
| dev 非預設值 `BACKTEST_MAX_SYMBOLS=2`（三個服務都帶入） | Go：3 檔 400 `too_many_symbols:3:2`、`breakout_v1` 2 檔 400 `backtrader_single_symbol:2`、`9999-12-31` 400 `invalid_date:end_date`、modular 2 檔 201 → `done`（2317 在 dev 沒有日 K → `skipped`）；直接寫進 DB 的 3 檔任務，server 與 worker 都 `failed`／`too_many_symbols:3:2`，Go 讀出相同代碼。測試任務已刪除，dev stack 已 `down`（⛔ 沒有 `-v`） |

## 七之二：live 上版（2026-11-20）

* 14:04 部署前檢查：live `03e7028`、`job_runs` running 0、回測 pending／running 0、migration 77、`deploy.sh` `abcf1c019b25`、`--preflight` 通過；
  image backend `f3a5ae6b9c9c`、python-server `40702c240772`、python-worker `c58a5cce2c69`（已打 `pre-i137` 回滾標籤）。
* `deploy.sh`：`03e7028..31e5566`，rc=0。
* 14:11 部署後：三個服務 `running`、RestartCount 0；image backend `bc503920bb75`、python-server `f166cc4e58c5`、python-worker `4303b51148a2`；
  backend 執行檔 `90539d61bf71`、`Config.Env` 50 行與雜湊不變；migration 78、`data_coverage` 欄位存在；前端入口 `index-C6VWJ-Bz.js`；
  python 兩個服務的設定 `250 20 5000 35000`、費率不變、`schema ready`（077、078）、沒有 Traceback；`/health` 正常、worker 有在 poll；
  既有 7 筆結果的 `data_coverage` 都是 NULL，任務的分布與 2 筆 `failed` 跟部署前相同。
* 新舊界線：2026-11-20 14:10:35（台北，python-server 啟動；worker 14:10:34），歸檔在 `database-schema.md` 的 `data_coverage`。
* 上版後的實跑核對（E-2）：`breakout_v1`、2330、2024-01-01～2025-12-31（`bt_20261120_061311_000`，14:13:11 建立、不到 1 秒 `done`）。
  `data_coverage` 是一筆 2330：`first_bar` 2024-01-02、`last_bar` 2025-12-31、`bars` 485（等於 DB 在這段台北日期的日 K 根數）、
  `warmup_bars`／`warmup_required` 250／250、`skipped` false。明細 3 筆、`total_trades` 3；I-135 的明細查詢與 I-136 的費用查詢都 0 筆不符（rc 0）。
  `sharpe_ratio` -50.6 是 `breakout_v1` 固定買 1 股、報酬極小（`total_return` 0.0003）而無風險利率 1% 的結果，公式跟原本的分析器相同，
  屬於 `todo.md` T-085（部位大小），⛔ 不是 I-137 的問題。前端的檢查（E-1）：使用者 2026-11-20 在已登入的 live 畫面確認正常——切到 `breakout_v1` 時是單選、不預選；
  兩筆舊的 `failed`（`bt_20260701_074009_000`、`bt_20260701_075122_000`）各自顯示「回測失敗（內部錯誤）」。
* E-2 輸出裡的 `zsh: command not found: status` 是把說明文字一起貼進終端機，出現在查任務狀態那一行之後；部署的 rc=0 是 C 段緊接
  `deploy.sh` 取得，兩者無關。

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
| v1.5 | 2026-11-19 | 實作中的裁決（使用者）：①加每一檔的上限 5,000 根（範圍段＋暖身段；量測發現 modular 的時間跟每一檔根數的平方成正比，只靠合計擋不住單一檔很長），單一檔超過回 `symbol_too_large`、每一檔都合格但合計超過回 `task_too_large`；modular 效能另列 T-086；②資料不夠初始化 backtrader 指標時，執行前檢查並回 `insufficient_data`（取代原本丟 `IndexError` 變成 `internal_error`），v1.4 的「範圍比 minperiod 還短」驗收改成「總資料足夠、範圍首根仍在 prenext」。上限數值依 live 唯讀查詢與 dev 量測寫回第三節第 4 點。 |
| v1.6 | 2026-11-19 | 實作 review：[P2] backtrader 的年化報酬與夏普改回以日為期（取每一天最後一根的資產，跟原本 `timeframe=Days` 的分析器相同），最大回撤維持逐根，補一天多根的分 K 新舊對照；[P3] Go 擋掉年份 0000（`MinDate` 0001-01-01，回 400 `invalid_date`），請求驗證改用 Go 與 python 共用的案例檔；[P3] `task_too_large` 的描述改成「先碰到任務的總額度」（查詢在額度 + 1 根就停，⛔ 不能宣稱每一檔都合格），補兩種上限都會超過的案例。`max_task_bars` 的 60,000 仍是候選值，多檔的整體耗時見實測。 |
| v1.7 | 2026-11-20 | 實作 review 通過 v1.6 的三項修正；`max_task_bars` 依 review 的建議定為 **35,000**（含暖身；11 檔日 K 整段歷史 29,978 根放得下，60,000 的多檔實測單筆 830 秒，最壞情況改為 7 檔 × 5,000 根、約 8 分鐘），等 T-086 改善 modular 效能後再評估提高。 |
