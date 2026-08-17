# Evaluation Universe Selection Plan

建立日期：2026-08-17

關聯任務：[`todo.md`](./todo.md) T-040 Step 3

## 目的

本計畫書定義 T-040 Step 3「流動性過濾與最終 evaluation universe」的落地方式。
它承接 T-040 Step 1 / Step 2 已完成的全市場短期回補與波動分佈判讀，用來選出可支撐
T-002 / T-003 後續 evaluation、sweep 與 decision replay 的研究標的池。

核心目標：

1. 從已回補的候選標的中選出 120～150 檔，而不是回到原本 200 檔上限。
2. 在 bucket 分層之前先排除低流動性標的，避免 LOW bucket 被「沒人交易所以價格不動」的標的填滿。
3. 保留現有 11 檔 watchlist 的可比性，但不讓新標的進入 watchlist、盤中掃描、籌碼同步、signal 掃描或 production SR 分析。
4. 等最終清單與 deep backfill 驗證後，再決定是否建立 `evaluation_universe` 表與每日純日 K 排程。

## 現況

T-040 已完成的前置能力：

- Step 0 記憶體實測完成：150 檔可行，200 檔過於邊緣；執行 150 檔 evaluation 前應確認 host available >= 570MB。
- Step 1 候選清單 API / 前端完成：`GET /stock-symbols/candidates`、`GET /stock-symbols/facets` 與 `EvaluationUniverse.svelte` 可產生候選並觸發回補。
- Step 2 全市場短期資料已回補並判讀：857 檔 / 454,152 列，840 檔具備 >=60 根可計算波動 profile。
- 現行 LOW / NORMAL / HIGH 門檻下，股票 LOW 確實存在，但與低流動性高度重疊。

重要限制：

- `EvaluationUniverse.svelte` 不做流動性篩選是正確的；候選產生時還沒有 `candles.amount` 可用。
- `fetch_candles()` 已回傳 `amount`，但 `evaluation._load_db_sources()` 目前只保留 OHLCV，因此既有 `volatility_profiles` 還沒有日均成交金額。
- `evaluation_universe` 表是維護「已選定標的池」的機制，不是用來決定選誰的工具。

## 不做事項

- 不直接調整 T-003 的 `build_zone_builders()` 預設值。
- 不預設啟用 adaptive builder。
- 不把新標的塞進 `watchlists`。
- 不用債券 ETF 補足股票 LOW bucket 來決定股票用的 builder config。
- 不先做 sweep API / UI；目前 CLI 與 JSON report 足以支撐一次性研究。
- 不先做完整 `evaluation_universe` CRUD；最終標的池確認後再做 Phase 2。

## 流動性分析輸出

Step 3 第一個交付物應是一份 selection report，而不是直接建表或深抓 5 年。
報告可以由 Python CLI 或小型分析工具輸出 JSON / CSV，資料來源為已回補的候選 K 線與
`stock_symbols` 主檔。

每檔至少輸出：

| 欄位 | 說明 |
|---|---|
| `symbol` | 代號 |
| `name` | 名稱 |
| `security_type` | TWSE ISIN 分類，例如 `股票` / `ETF` |
| `industry` | 產業，ETF 多為空字串 |
| `candle_count` | 已有日 K 根數 |
| `last_candle_at` | 最新日 K 日期 |
| `atr_pct` | 近 60 根 ATR / close。**資料不新鮮者必須輸出 `null`**，不可照算 |
| `average_range_pct` | 近 60 根平均日內振幅 |
| `current_bucket` | 依現行 LOW / NORMAL / HIGH 絕對門檻計算的 bucket。`atr_pct` 為 `null` 時應為 `stale` |
| `avg_amount_60` | 近 60 根平均成交金額 |
| `median_amount_60` | 近 60 根成交金額中位數 |
| `traded_days_60` | **該標的在最近 60 個「市場交易日」內的 K 線根數**（分母取全庫 distinct 日期）。**不可定義成「自己近 60 根裡 `amount > 0` 的天數」**——見下方「已知缺陷」 |
| `liquidity_tier` | 依成交金額切出的流動性層級 |
| `universe_role` | `primary`（參與股票 builder 決策）／`supplemental`（僅交叉觀察）。依 **TWSE 代號後綴**判定：ETF 代號結尾為字母者（`B` 債券／`L` 槓桿／`R` 反向／`U` 期貨／`A` 主動／`K` 特殊級別）一律 `supplemental`，純數字代號的股票型 ETF 才是 `primary`。**規則刻意保守**——誤標 supplemental 只是少一檔觀察，誤標 primary 會讓債券或槓桿商品影響股票參數 |
| `selection_bucket` | 最終選池用 bucket，可能等於 current bucket，也可能是分位數 bucket |
| `selection_status` | `selected` / `excluded` / `review` |
| `exclusion_reason` | 排除原因，例如 `low_liquidity`、`stale_candle`、`short_history` |

建議硬性排除條件：

- `traded_days_60 < 45`（依上表的**市場交易日**定義）
- `last_candle_at` 明顯落後全體最新交易日
- `avg_amount_60` 或 `median_amount_60` 幾乎為 0
- 5 年 deep backfill 前，上市未滿 5 年者不進最終主池；可保留在觀察清單但不參與 T-003 正式調參。

### 已知缺陷與修正（2026-08-17 review）

**一、`traded_days_60` 的原定義（近 60 根中 `amount > 0` 的天數）永遠不會觸發排除。**

實測全庫 454,164 列裡，`amount = 0` 與 `volume = 0` 的**都是 0 筆**。原因是 FinMind 的日 K
**只回傳有成交的交易日**——沒成交的日子根本沒有那一列。所以該定義對任何有 60 根的標的
恆等於 60，`traded_days_60 < 45` 這條「硬性排除條件」抓不到任何東西。

而它要抓的問題是真實存在的。改用市場交易日當分母後：

| 近 60 個市場交易日的有成交天數 | 檔數 |
|---|---|
| 58–60（幾乎每日成交） | 793 |
| 45–57 | 40 |
| 20–44 | 18 |
| **< 20（多數日無成交）** | **5** |

`6236 中湛` 在 60 個市場交易日裡只成交 **7 天**，但它有 38 根 K 線——用原定義完全看不出來。

**二、`atr_pct` 的樣本可能來自很久以前。** 「近 60 根」對停止交易的標的可能橫跨數月：
`4804 大略-KY` 停在 2026-04-13，它的近 60 根是 2025-12 到 2026-04，算出來的波動率描述的是
四個月前的狀態。原計畫的 `last_candle_at` 排除發生在**報告產出之後**，而 `current_bucket`
已經用髒資料算完了——只看 bucket 分佈統計的人會被污染。因此改為在報告產出時就標 `stale`。

## 門檻矩陣

不要先把流動性門檻寫死為 5,000 萬。T-040 Step 2 已顯示 5,000 萬可能讓 LOW 股票只剩極少數，
因此應先比較多組門檻後再決策。

股票門檻矩陣：

| 門檻 | 用途 |
|---|---|
| 1,000 萬 | 最低可交易性門檻，避免明顯無交易標的 |
| 2,000 萬 | 折衷門檻，觀察 LOW bucket 是否仍有足夠樣本 |
| 5,000 萬 | 較嚴格門檻，接近原文件粗估點 |
| 1 億 | 高流動性版本，用來看穩健樣本是否仍足夠 |

ETF 應分開統計：

- 股票型 ETF 可作為市場代表樣本，但不應主導股票 bucket 門檻。
- 債券 ETF 可保留為補充觀察，但不拿來決定股票 zone builder config。
- 若 ETF 進入最終 universe，應在 selection report 中標示 `security_type=ETF` 與用途。

每個門檻都要輸出：

- 合格股票數
- 合格 ETF 數
- current LOW / NORMAL / HIGH 分佈
- 依股票分位數切 bucket 後的 LOW / NORMAL / HIGH 分佈
- 各 bucket 的產業分佈
- 被排除的主要原因統計

## Bucket 決策規則

Step 3 不應先假設一定沿用目前絕對門檻。決策順序如下：

1. 先套用流動性與資料完整度過濾。
2. 檢查現行絕對門檻下，LOW / NORMAL / HIGH 是否都有足夠股票樣本。
3. 若三組樣本足夠且產業不過度偏斜，`selection_bucket` 可沿用 current bucket。
4. 若 LOW 不足或集中在少數低流動性 / 特定產業標的，改用流動性合格股票內的 ATR% 分位數切 bucket，例如 P33 / P67。
5. ETF 不與股票混在同一個分位數母體；需要時可在 report 中提供 ETF 自己的 bucket 分佈。

建議第一版採用：

- 股票主池使用「流動性合格後的分位數 bucket」作為 `selection_bucket`。
- 保留 current bucket 欄位，讓結果能回推到 T-003 現有 LOW / HIGH 門檻是否需要重定。
- T-003 builder 調參以股票主池為主；ETF 結果作為交叉觀察，不直接覆寫股票參數。

## 最終 Universe 選取規則

目標規模：120～150 檔。

選取原則：

1. 保留現有 11 檔，作為**歷史連續性與回歸檢查基準**。
   **注意不是「sweep 結果可比」**：2026-08-06 那次 sweep 的母體就是這 11 檔，
   而新 universe 是 120～150 檔、bucket 組成完全不同，兩次 sweep 的 score **不可直接對比**。
   能比的是**同樣 11 檔的個別 zone 統計與 `volatility_profiles`**（資料沒變就該完全相同），
   那是回歸檢查，不是效果比較。

   **保留是分級的，不是無條件**——規格見下方「watchlist 的分級保留」。
2. 股票為主，依 `selection_bucket` 盡量平均分配。
3. 每個 bucket 目標 35～45 檔；不足時不要用低流動性股票硬補。
4. 限制單一產業在單一 bucket 內的占比，避免半導體、電子零組件等大型產業主導。
5. 優先選上市滿 5 年且 K 線可深抓的標的。
6. ETF 只保留代表性股票型 ETF；債券 ETF 若保留，標記為 `supplemental`，不參與股票 builder 決策。

### watchlist 的分級保留（2026-08-17 裁決）

**設計原則：真正要避免的是「靜默」，不是「保留與否」。** 早期實作只在 `selection_status ==
"selected"` 時才收 keep symbol，於是 watchlist 標的一旦被判不合格就**從 universe 消失且不留紀錄**；
主檔裡找不到的（無 K 線被跳過、`security_type` 不在查詢範圍、已從 `stock_symbols` 移除）
同樣靜默忽略。重跑時 universe 少一檔，看起來會像資料異常或選池決策改變。

**一律無條件保留也不對。** `stale_candle` 的標的 `atr_pct` 為 None、`selection_bucket` 為
`BUCKET_STALE`：拿不到 ATR config、也產不出 `volatility_profiles`——而「當回歸基準」正是保留
watchlist 的唯一理由。強塞進 universe 只會多一個無桶標的，並讓 `per_bucket` 的配額統計分母對不上。

所以按**能不能算出 bucket 與 profile** 分級：

| 情形 | 處置 | 輸出欄位 |
|---|---|---|
| 合格 | 留在 universe，列入回歸基準 | `regression_baseline_symbols` |
| `short_history` / `thin_trading` / `low_liquidity` | 留在 universe，**不**列入基準 | `baseline_excluded` |
| 深度撐不起 walk-forward | 留在 universe，**不**列入基準 | `insufficient_depth` ＋ `baseline_excluded` |
| `stale_candle`（`KEEP_FATAL_EXCLUSIONS`） | **不留**，顯式報告 | `keep_symbols_dropped` |
| 主檔／K 線裡找不到 | **不留**，顯式報告 | `keep_symbols_missing` |

三個欄位語意刻意分開，不要合併：

* `insufficient_depth` 只講**深度**，並帶 `kind: backfillable / listing_age`（見階段 4）。
  把流動性與新鮮度混進去會讓它變成雜物袋。
* `baseline_excluded` 是「留在池內但不當基準」的**總表**，`insufficient_depth` 是它的來源之一。
* `selection_reason` 區分 `watchlist_baseline`（是基準）與 `watchlist_kept`（只是留著）——
  後者若也叫 baseline，會與 `baseline_excluded` 自我矛盾。

分級保留下來的不合格標的**一律歸 `supplemental`**（第 6 點的機制，輸出在 `supplemental_symbols`）：
留在 universe 供觀察，但不得影響股票 builder 決策——否則等於用低流動性／稀疏成交的標的去調參，
正是第 3 點要避免的事。

**現況（2026-08-17 實測）**：11 檔 watchlist 全數 `selection_status=selected`，
`keep_symbols_missing` 與 `keep_symbols_dropped` 都是空的，
`baseline_excluded` 只有 `{'00947': 'insufficient_depth', '00981A': 'insufficient_depth'}`，
`supplemental_symbols` 只有 `{'00981A': 'etf_suffix'}`，回歸基準 **9 檔**。
分級機制目前沒有被實際觸發，但母體裡有 13 檔 `stale_candle`、10 檔 `thin_trading`、
16 檔 `short_history`，watchlist 未來落進去是合理的預期。

最終 report 應包含三份清單：

| 清單 | 用途 |
|---|---|
| `selected_symbols` | 要 deep backfill 的最終清單 |
| `review_symbols` | 邊界案例，需人工確認 |
| `excluded_symbols` | 被排除者與原因 |

**`review_symbols` 必須有收斂條件，否則會變成永久待辦。** 規則：

- review 項目**必須在 deep backfill 前清空**——每一筆要嘛升為 `selected`、要嘛降為 `excluded`。
- 判定依據與判定者記在 report 或 `todo.md`，不要只留在對話。
- **未決者一律歸 `excluded`**：寧可少幾檔，也不要讓沒被判斷過的標的混進調參母體。

## 執行流程

1. 產出 Step 3 selection report。
2. 依 report 決定流動性門檻與 bucket 策略。
3. 人工確認 `selected_symbols`，目標 120～150 檔。
4. 用既有 EvaluationUniverse 頁面或 `POST /market/backfill` 對 `selected_symbols` 執行 5 年 deep backfill，建議 `days=2400`。
5. 跑 baseline evaluation，確認 150 檔規模下記憶體、時間與 bucket 分佈。
6. 跑 T-003 coarse sweep。
7. 若 zone 層候選差異明確，再選少數候選跑 decision replay / RR 比較。
8. 只有在 deep backfill 與 evaluation/sweep 都確認可行後，才實作 Phase 2 `evaluation_universe` 表與每日純日 K 排程。

## Phase 2：正式 Evaluation Universe

Phase 2 的目的不是選股，而是維護已確認的標的池。

建議資料表仍沿用 T-040 原規格：

| 欄位 | 說明 |
|---|---|
| `symbol` | 代號 |
| `bucket_hint` | selection report 決定的 bucket |
| `selected_at` | 入池時間 |
| `source` | 入池來源，例如 `T-040_STEP3` |
| `active` | 是否仍在每日日 K 維護 |
| `note` | 流動性門檻、ETF supplemental 等備註 |

排程仍應只做一件事：每日盤後對 active universe 跑日 K 更新。

不得接入：

- 盤中分 K
- 籌碼同步
- signal 掃描
- production SR zone 分析與驗證
- watchlist UI

## 測試與驗證策略

分清**哪些唯讀、哪些寫入 live**：階段 0～3 與 5～6 全程唯讀，只有階段 4 的 deep backfill
會寫入，依 `development-workflow.md` 的「要動 live 資料時的做法」需要單獨授權並由使用者執行。

### 前提：selection report 要是可重複執行的腳本

依 `development-workflow.md` 的「測試腳本優先」，這支**不該是一次性指令**。
建議 `scripts/build-selection-report.sh`，比照 `scripts/verify-event-timeline.sh`：
DSN 從 live container 讀（密碼不進版控）、唯讀、走 mem-guard、輸出 JSON / CSV。

### 階段 0：固定資料快照（唯讀）

報告開頭必須輸出資料狀態，否則數字無法重現也無法歸因：

```
symbols=857  rows=454,164  market_days_60=[…～2026-08-13]  host_available=xxxMB
```

**這不是形式**——日後有人重跑得到不同數字時，沒有快照就無法判斷是資料變了還是程式錯了。

### 階段 1：已知答案測試（唯讀，**最重要**）

**這是唯一能抓到「對資料長相誤解」的方法。** T-045 的經驗：13 支單元測試全綠，
但真正的 bug（終結狀態被重複回報）是 live 實跑才發現的——因為測試驗的是「我以為的行為」。

用實測確認過的真實標的當斷言，六筆各蓋一種失效模式：

| 標的 | 實際狀況 | report 必須判為 |
|---|---|---|
| `6236` 中湛 | 60 個市場交易日只成交 7 天（K 線僅 7 根） | 排除，原因 `short_history`（根數不足以算波動，優先於 `thin_trading`） |
| `4804` 大略-KY | 停在 2026-04-13 | 排除 `stale_candle`，且 `atr_pct` 為 `null` **不是照算** |
| `3067` 全域 | 60 個市場交易日只成交 **44 天**、日均 13 萬 | 排除，原因 `thin_trading`（成交天數不足優先於低流動性） |
| `2633` 台灣高鐵 | ATR 1.10%、日均 3.34 億 | LOW 且流動性合格 → 選入 |
| `00679B` 元大美債20年 | 債券 ETF（代號 `B` 結尾） | `universe_role=supplemental`，**不進股票分位數母體** |
| `2330` 台積電 | ATR **2.80%** | **NORMAL**——不是 HIGH |
| `2454` 聯發科 | ATR 6%+ | HIGH |

任一筆不符即為 bug。

> **`2330` 是 NORMAL 不是 HIGH。** 本表初版把它寫成 HIGH，是把「半導體業 195/201 落在
> HIGH」的統計套到個股上——**台積電正是那 6 檔 NORMAL 之一**，大型權值股波動低於產業
> 平均本來就合理。2026-08-13 與 08-17 兩次實測都是 2.8%。
> 這類「預期值本身就寫錯」的情況，正是已知答案測試要用**實測數字**而非印象的原因。

> **排除原因的優先序**：`stale_candle` → `short_history` → `thin_trading` → `low_liquidity`。
> 「資料不可用」優先於「條件不合格」，因為前者代表那筆數字本身不可信。
> 每檔只給一個原因，否則排除統計無法加總。

### 階段 2：獨立重算交叉驗證（唯讀）

比照 `scripts/verify-adjustment.sh` 的既有模式（用 SQL 獨立重算一次再比對）：
抽 20～30 檔，以**純 SQL** 重算 `atr_pct`、`avg_amount_60`、`traded_days_60`，
與 report 逐欄比對，容忍度 1e-6。

會抓到的是：**時區處理**（`ts` 存 UTC，用 `ts::date` 會差一天——本專案已踩過）、
`adj_factor` 有沒有套、視窗邊界差一根。

### 階段 3：性質檢查（唯讀）

不論資料如何都必須成立，違反即為邏輯錯誤：

- **門檻單調性**：1,000 萬 → 2,000 萬 → 5,000 萬 → 1 億，合格檔數遞減且**後者是前者的子集合**
- **分位數母體純度**：斷言計算 P33/P67 的**輸入集合本身**等於「流動性合格的股票」，
  不能只看輸出的 bucket 數量
- **三份清單互斥且完整**：`selected ∪ review ∪ excluded` ＝ 全體，兩兩無交集
- **每檔恰有一個排除原因**，或明確定義多重原因的優先序

### 階段 4：deep backfill（**寫入 live，需授權，由使用者執行**）

之後以唯讀驗證三件事：

1. **筆數與涵蓋**：每檔 ≥1,500 列，最後交易日與全體一致
2. **缺漏交易日**：以市場交易日為分母，覆蓋率 ≥95%（低於此代表本來就不該入選）
3. **bucket 穩定性**：重跑 selection report 比對 bucket 變動
   —— 判準見下方，**不是「不該變」**

#### 階段 4 實測與判準修正（2026-08-17）

79 檔淺標的完成 `days=2400` 回補後實跑，三項結果：

| 驗證 | 結果 |
|---|---|
| ① 筆數與涵蓋 | **通過**：127/130 ≥1,500 列，3 檔例外全部查明非回補失敗 |
| ② 缺漏交易日 | **通過**：99.1%～100%，全數高於 95% |
| ③ bucket 穩定性 | **原判準錯誤**，見下 |

**第 3 項原本寫「理論上 bucket 不該變，變了就代表資料有問題」——這個判準是錯的。**
bucket 由**分位數**決定，分位數是相對於當下母體的，母體一動邊界就跟著移。實測 9 檔變動中
有 3 檔（3530、3661、8102）這次根本沒重抓、`atr_pct` 一個 bit 都沒變，照樣跳桶。
詳細數據與後續處置記在 `docs/todo.md` 的 T-003「門檻重定 → bucket 邊界必須凍結」。

**修正後的判準**：比對重跑前後的 `quantile_edges` 與 bucket 變動集合，逐檔確認每一筆變動
都能歸因到「`atr_pct` 改變」或「邊界移動」二者之一。**無法歸因的才是資料問題。**
真正該當成通過條件的是 `selection_status`——實測**零變動**，代表流動性與歷史長度篩選是穩的。

#### 重跑比對的前提：`--keep-symbols` 必須一致

`build-selection-report.sh` 原本沒有把 `--keep-symbols` 傳給 `selection_report`，
重跑時 watchlist 不會被保留，00830 / 00947 / 00981A / 6243 這批「靠保留才進池」的標的
會整批掉出，看起來像資料出問題。已把定案用的 11 檔設為腳本預設值（`KEEP_SYMBOLS` 可覆寫），
**重跑比對前先確認這個值與定案那次相同**。

**預設值是刻意凍結的**——凍結才能重現定案那次的選池。代價是它不會跟著 watchlist 變動，
所以報告會比對 DB `watchlists` 並輸出 `keep_symbols_drift`，不一致時 stderr 警告。
沒有這個比對，日後在前端加一檔 watchlist 後重跑，那檔不進池且不會有任何提示。

`watchlists` 的成員資格**就是「表裡有這一列」**，不要用 `watched` 過濾：那個布林是
「要不要即時監聽」的開關且有併發上限，實測 11 檔裡只有 2 檔為 true。

#### 深度不足的標的：留在池內，但不算回歸基準

watchlist 的保留不受年限篩選約束（見「watchlist 的分級保留」），所以 `min_listed_years=5`
擋不住它——00947（529 根）、00981A（299 根）就是這樣進池的，而 `--limit 1500` 的
walk-forward 撐不起來。處置是**留在 universe 照跑 evaluation，但從回歸基準扣掉**：
`select_universe` 輸出 `insufficient_depth` 與 `regression_baseline_symbols`，
實測回歸基準是 **9 檔**而不是 11 檔（見階段 6）。

這個檢查另外抓到 `min_listed_years` **結構上抓不到**的一類標的：1569 濱川（2005 年上市）、
1617 榮星（2000 年上市）——上市二十年，庫內卻只有 158 列、起點 2025-12-16，從未深補過。
年限篩選看的是 `listed_date`，看不到「我們有沒有抓」。它們是靠 bucket 名額正常選進來的，
若直接跑 evaluation 會靜靜產出退化結果。所以 `insufficient_depth_detail` 另外標 `kind`：

* `backfillable`——上市夠久、庫裡沒資料，**深補就會好**（1569、1617）
* `listing_age`——庫裡已是全部歷史，再抓也不會變多（00877 1,476 列即其完整 6 年、00947、00981A）

**`backfillable` 的標的要在進入階段 5 之前補完。**

### 階段 5：baseline evaluation（唯讀，但吃記憶體）

```bash
MEASURE_PEAK=1 scripts/run-evaluation.sh --symbols <selected>
```

判準沿用 T-040 Step 0 實測的那一套：**峰值 ＋ 150MB 保留 < host available**。
執行前必須確認**沒有 gitea 那一級的服務常駐**——實測那會讓 available 掉到 398MB，
mem-guard 直接擋下、連 10 檔都跑不起來（見 `development-workflow.md`「container 上限的總和也要顧」）。
150 檔推估 ~420MB，需要 570MB available。

同時比對三項與 Step 0 外推值的落差：實際峰值 vs 420MB、耗時 vs 14 分鐘、
report 的 `volatility_profiles` bucket 分佈 vs selection report 的預期。

### 階段 6：回歸基準（唯讀）

watchlist 的資料沒變，它們的 `volatility_profiles` **應與 2026-08-06 那次完全相同**。
不同即代表 pipeline 在這段期間被改動而未被發現。

這是「保留 watchlist」的正確用法——比對的是**個別標的的統計**，不是 sweep 的結論。

**基準是 9 檔不是 11 檔**：00947、00981A 深度撐不起 walk-forward（見階段 4），
以 `select_universe` 輸出的 `regression_baseline_symbols` 為準，不要手寫 watchlist 清單。
實測該欄位為 `0050, 00830, 2330, 2399, 2454, 2478, 3630, 5490, 6243`。

### T-003 後續

- 第一輪只跑 coarse sweep。
- 不因單一 bucket 的小幅勝出直接改 production default。
- 需要看到跨 bucket、跨指標的穩定差異，才考慮調整 `VOLATILITY_BUCKET_ATR_CONFIGS`。
- adaptive builder 預設仍維持關閉，直到 decision replay / RR 比較也支持啟用。

## Step 3 實測結果（2026-08-17）

### 工具

`scripts/build-selection-report.sh`（唯讀）→ `python/selection_report.py`。
核心是純函數，測試不需要 DB；已知答案測試用 live 實測的真實標的形狀當 fixture。

### 選池結果：130 檔

| bucket（分位數） | 候選 | 選入 | 其中 ETF | 產業數 |
|---|---|---|---|---|
| `LOW_VOLATILITY` | 128 | 45 | 2 | 20 |
| `NORMAL_VOLATILITY` | 85 | 45 | 2 | 21 |
| `HIGH_VOLATILITY` | 90 | **29**（未達 35） | 0 | 14 |

流動性門檻 2,000 萬、產業上限 11 檔/bucket、上市滿 5 年、watchlist 11 檔無條件保留。

### 三個設計決定（2026-08-17）

**一、維持分位數選取，並把「重定絕對門檻」列為 T-003 的前置輸入。**

這是本次最重要的發現：**pipeline 用的絕對門檻（1.5% / 3.5%）與台股實際分佈差一個量級**，
流動性合格股票的分位數切點是 **4.25% / 6.04%**。所以用分位數選出的「三個 bucket 平均分佈」
在 pipeline 眼中仍是 **103 / 26 / 1**——LOW 只剩一檔。

**不重定門檻，選池怎麼挑都沒用。** 報告因此輸出 `threshold_gap` 欄位並列兩組數字，
讓 T-003 有可執行的依據而不是印象。與其扭曲選池去遷就過時的門檻，
不如把門檻重定正式納入 T-003（見 `todo.md` T-003「門檻重定」）。

**二、接受 HIGH bucket 只有 29 檔，不放寬產業上限。**

這不是演算法的問題，是**母體的事實**：HIGH bucket 的 90 個候選裡 **82 檔是半導體業**（91%）。
產業上限把它壓到 11 檔後，其餘 13 個產業總共只湊得出 18 檔。
**「產業分散的高波動 bucket」在台股資料上不成立**——放寬上限只會讓 HIGH 變成半導體專場，
那對調參沒有幫助。這一點應直接寫進 T-003 的判讀前提。

**三、ETF 給獨立配額（每 bucket 2 檔），不佔產業名額。**

股票型 ETF 的 `industry` 是空字串，與股票共用配額時會被歸成「(未分類)」這一個**假產業**，
實測吃掉 LOW bucket 整整 11 個名額。ETF 不是一個產業，是不同的商品類別。
修正後 LOW bucket 的產業數由 19 增為 20。

### 選取演算法的排序依據

**日均成交金額由高到低**，同額以代號打破平手（決定性，同輸入必得同輸出）。

這與 Step 1 候選抽樣**刻意避開**的「取代號最小的前 N」不同：那裡的偏斜會扭曲**分佈量測**，
這裡流動性高低本身就是**資料品質**的代理——zone 的觸價統計在成交稀疏的標的上不可信。
但它仍是一種偏斜（偏大型股），所以用產業上限壓住。

### 已驗證的已知答案（階段 1）

八筆全數通過，包含兩筆**本計畫書初版預期值寫錯**的案例（`2330` 應為 NORMAL、
`3067` 應為 `thin_trading`）。這正是已知答案測試要用實測數字而非印象的理由。

## 風險

| 風險 | 處理 |
|---|---|
| LOW 股票在合理流動性門檻後仍不足 | 改用分位數 bucket；不要硬補低流動性股票 |
| ETF 主導 LOW bucket | 股票與 ETF 分開統計，ETF 不直接決定股票 builder |
| 150 檔 evaluation 記憶體接近上限 | 降到 120～130 檔；不先做大規模 pipeline 重構 |
| 深抓 5 年後部分標的資料不足 | 從 review pool 補替代標的 |
| selection report 被誤當 production 設定 | 明確標記為研究輸出，不寫 production config |

## 完成條件

Step 3 可視為完成的條件：

1. 已產出包含流動性、bucket、產業分佈與排除原因的 selection report，
   且**通過階段 1～3**（已知答案測試、獨立重算交叉驗證、性質檢查）。
2. 已確認最終 120～150 檔 `selected_symbols`，且 `review_symbols` **已清空**。
3. 已完成 5 年 deep backfill，並**通過階段 4** 的三項檢查（筆數涵蓋、缺漏交易日、bucket 變動可歸因），
   且 `insufficient_depth` 內沒有 `kind=backfillable` 的標的殘留。
4. baseline evaluation 可在資源限制內完成（**階段 5** 的判準：峰值 ＋ 150MB < host available）。
5. **階段 6** 的回歸基準通過：`regression_baseline_symbols`（實測 9 檔）的
   `volatility_profiles` 與 2026-08-06 相同。
6. T-003 sweep 已用新 universe 跑過，並把是否足以調參的結論回寫到 `todo.md` 或 `sr-zone-scoring.md`。

Phase 2 `evaluation_universe` 表與排程不是 Step 3 的完成前置；它是 Step 3 選池與驗證完成後的維護機制。
