# Database Schema

支援三種資料庫：**SQLite**（開發）、**MySQL 8.0+**、**PostgreSQL 14+**。  
Migration 由 goose 在啟動時自動執行，不需手動跑 SQL。

---

## candles

主要 OHLCV 資料表。

| 欄位 | 類型 | 說明 |
|------|------|------|
| id | BIGSERIAL / BIGINT AUTO_INCREMENT | PK |
| symbol | VARCHAR(10) | 股票代號，e.g. `2330` |
| timeframe | VARCHAR(5) | `1m`, `5m`, `1d` |
| open / high / low / close | DECIMAL(10,2) | 價格 |
| volume | BIGINT | 成交量 |
| amount | DECIMAL(18,2) | 成交金額 |
| ts | TIMESTAMPTZ / DATETIME(0) | K 棒時間 |
| adj_factor | DECIMAL(18,10) | **價格**的累積還原係數（migration 061），預設 1 |
| vol_factor | DECIMAL(18,10) | **成交量**的累積還原係數（migration 062），預設 1 |

**Index：**
- `UNIQUE(symbol, timeframe, ts)`：防止 FinMind 重複寫入
- `INDEX(symbol, timeframe, ts DESC)`：支援 `ORDER BY ts DESC LIMIT N`

**Constraint：**
- `ck_candles_positive_price`（migration 060）：`open/high/low/close` 全部必須 `> 0`。
  背景：live 曾出現 4 根 OHLCV 全為 0 的日 K（2026-08-10 已清除並完整驗證），
  **無成交的日子應該是「沒有那筆資料」，不是一根價格為 0 的 K 棒**。
  留著它會污染 MA / ATR / zone 建構且不會有任何東西報錯。
  **不約束 `volume`**：成交量為 0 在盤中分K 是正常的。
  寫入端另有一層（`market/fetcher.go` 的 `toStoreCandles`），兩層各補對方的不足——
  Go guard 擋不住手動 SQL，DB 約束不會告訴你哪一檔哪一天被丟掉。
- postgres 版寫成 **`NOT VALID`** 是刻意的：加上約束的當下 live 仍有那 4 列髒資料，
  一個會驗證既有列的約束會讓 migration 失敗、連帶擋住整個部署。`NOT VALID` 不管資料
  當下乾不乾淨都套得上，把「驗證既有列」留成部署後的獨立動作。
  **live 已於 2026-08-10 清完資料並執行 `VALIDATE CONSTRAINT`**（`convalidated = t`，
  postgres 為此掃過全表）。日後若有新環境重跑這條 migration，同樣要在資料清乾淨後補跑：
  `ALTER TABLE candles VALIDATE CONSTRAINT ck_candles_positive_price;`。

> **`ts` 存 UTC**。Taipei 00:00 = 前一日 16:00+00，所以查詢一定要
> `(ts at time zone 'Asia/Taipei')::date` 再比對日期，直接 `ts::date` 會整整差一天。

### 股價還原（`adj_factor` / `vol_factor`）

**`open/high/low/close/volume` 永遠是原始成交價量**，還原用係數相乘而不是改寫原始值：

```
還原價 = close * adj_factor
還原量 = volume / vol_factor      ← 注意是除
amount 不調整                      ← 成交金額是錢，不隨股數重新定義
```

**為什麼價乘量除**：分割讓股數變多，所以歷史價要縮小、歷史量要放大，方向相反。

**為什麼價與量用不同係數**：現金股利讓價格下修，但**股數沒有改變**，
所以成交量不可以跟著調整（`vol_factor = 1`）。只有分割與配股會改變股數。
因此 `還原價 × 還原量 == close × volume` **只在兩個係數相等時成立**——
現金股利發生時錢真的離開公司，乘積本來就該變小。

**為什麼不直接改寫原始價**：每次有新的公司行動就要回頭改寫該檔的全部歷史列，
而所有從舊價算出來的衍生資料（`indicator_snapshots`、`stock_analyses`、
`stock_sr_zone_analyses`）不會跟著更新，也不會報錯。

**還原後的 volume 是 float，這是刻意的**：`volume / vol_factor` 是除法，Python 端
（`db.fetch_candles`，還原的唯一進入點）不會把它截回整數——截整數會無聲丟掉還原的精度，
而且連 `vol_factor = 1` 的常見情形也會被牽動。**下游全部以 float 取用**
（`.astype(float)` / `to_numpy(dtype=float)`），且原始 volume 不跨 Python→Go 邊界
（Go 收的是 `relative_volume` 這類 float 欄位），所以沒有「`1234.0` 打進 int64 欄位」
的解析風險。行為由 `python/tests/test_db_fetch_candles.py` 鎖住——
把它改成回整數是**行為改變**，不是修 bug。要原始整數量時傳 `adjusted=False`。

**係數是 `corporate_actions` 的純函數**，重算永遠整段覆寫、不讀舊值，所以**冪等**：
跑一次跟跑十次結果相同。實作在 `market/adjuster.go`，
驗證用 `scripts/verify-adjustment.sh`（唯讀，用 SQL 獨立重算一次再比對）。

> **既有衍生資料的基準不一致**：`indicator_snapshots`、`stock_analyses`、
> `stock_sr_zone_analyses` 裡在還原上線**之前**產生的列，是用原始價算的。
> 刻意不回頭重算——後兩者是「當時做了什麼判斷」的紀錄，不是快取；
> `indicator_snapshots` 會在下次計算時自然被覆蓋。做跨期比較時要記得這件事。

---

## corporate_actions

公司行動（分割／除權息），`candles` 還原係數的唯一來源。

| 欄位 | 類型 | 說明 |
|------|------|------|
| symbol | VARCHAR(10) | 股票代號 |
| event_date | DATE | **新價的第一個交易日**。套用範圍是 `ts < event_date`，**當日不套** |
| action_type | VARCHAR(32) | 分割／反分割／面額變更（來源原文）、`DIVIDEND_CASH` / `DIVIDEND_STOCK` / `DIVIDEND_BOTH`、`CAPITAL_REDUCTION`、`UNKNOWN`。migration 064 由 16 放寬——`CAPITAL_REDUCTION` 是 17 字元 |
| before_price / after_price | DECIMAL(10,2) | 事件前後的參考價 |
| factor | DECIMAL(18,10) | 價格係數 = `after_price / before_price` |
| volume_factor | DECIMAL(18,10) | 股數係數。**純現金股利為 1** |
| source | VARCHAR(255) | `TaiwanStockSplitPrice`、`YahooDividendsByYear`、`TaiwanStockCapitalReductionReferencePrice`。migration 065 由 32 放寬——最長的是 41 字元，且外部 dataset 名稱不受我們控制，所以取 255 而非「剛好夠用」 |

**Constraint：**
- `UNIQUE(symbol, event_date, action_type)`：冪等性的第一道保證。重複抓取若產生第二筆，
  同一次事件會被乘兩次。
- `ck_corporate_actions_prices`：`before_price`、`after_price`、`factor` 皆 `> 0`。
- `ck_corporate_actions_volume_factor`：`volume_factor > 0`
  （驗證腳本用 `LN(volume_factor)` 連乘，0 會變 `-inf`）。

**資料來源**（見 [`architecture/data-pipeline.md`](./architecture/data-pipeline.md)）：

| 類型 | 來源 | 取得方式 |
|---|---|---|
| 分割／反分割／面額變更 | FinMind `TaiwanStockSplitPrice` | **一次批次請求抓全市場**（2015～2026 共 33 筆） |
| 除權息 | Yahoo `dividendsByYear` | **逐檔查詢**，沒有批次端點 |
| 減資 | FinMind `TaiwanStockCapitalReductionReferencePrice` | **逐檔查詢**（整批需 Sponsor tier） |

> **減資與反分割在數學上是同一件事**：股數變少、價格變高，所以係數 **> 1**，
> 且 `volume_factor` 等於價格係數。成交量的調整方向是**縮小**，與分割相反。
>
> **仍不涵蓋合併與下市重編**——見下方「未涵蓋的公司行動」。

### 未涵蓋的公司行動：合併與下市重編

股價還原目前涵蓋**分割、反分割、面額變更、除權息、減資**，
但**不涵蓋合併與下市重編**，而且**沒有資料源**：

* FinMind 的完整 dataset 目錄（104 個，台股 61 個）裡沒有這類資料。
* TWSE 的 `exchangeReport/TWTAUU` 欄位齊全但**只服務當年度**——跨年區間會被靜默截斷成當年；
  純過去的區間回一個與實際原因不符的錯誤（「查詢結束日期小於查詢開始日期」）。

**目前標的中沒有觀察到實例**：2026-08-11 減資上線後，全庫已無未解釋的假跳空。

#### 偵測方法與它的兩個例外

原理是「台股單日漲跌幅上限 ±10%，還原後仍超過就代表有未處理的公司行動」：

```sql
with adj as (
  select symbol, ts, close*adj_factor as p,
         lag(close*adj_factor) over (partition by symbol order by ts) as prev,
         lag(ts) over (partition by symbol order by ts) as prev_ts
  from candles where timeframe='1d')
select symbol, (prev_ts at time zone 'Asia/Taipei')::date, (ts at time zone 'Asia/Taipei')::date,
       round((p/prev-1)*100,1) as pct, (ts::date - prev_ts::date) as gap_days
from adj where prev > 0 and abs(p/prev-1) > 0.15 order by abs(p/prev-1) desc;
```

**兩個例外不先排除就會追到不存在的問題**：

1. **國外成分證券 ETF 不受 ±10% 限制。** 實測 `00830` 在 2025-04-07 為 −20.6%、
   04-10 為 +19.1%，但**同日 28 檔全部同向**（04-07 平均 −10.0%、04-10 平均 +9.7%），
   是 2025 年 4 月關稅衝擊的市場性事件，不是資料問題。
   **判別方法：看同一天其他標的動了沒。**
2. **門檻本身會漏掉真實事件。** 當初用 25% 只找到 3 筆減資，權威來源實際有 **7 筆**
   （多出 2412、2478 的 2016 那筆、2609、2317）。
   **門檻只能當異常偵測，不能當事件清單。**

---

## indicator_snapshots

技術指標快照，每次計算後 upsert。

| 欄位 | 類型 | 說明 |
|------|------|------|
| ma5/10/20/60 | DECIMAL(10,4) | 移動平均 |
| rsi14 | DECIMAL(7,4) | RSI（0～100）。⚠️ **3 位整數位是必要的**：`avgLoss = 0` 時 RSI 就是 100，`DECIMAL(6,4)` 容不下它——2026-09-01 因此讓 2454 的指標整支寫不進去（原記於 `issue.md` I-101） |
| macd / macd_signal / macd_hist | DECIMAL(10,4) | MACD 三線 |
| bb_upper / middle / lower | DECIMAL(10,4) | 布林通道 |
| atr14 | DECIMAL(10,4) | 平均真實波幅 |
| vwap | DECIMAL(10,4) | 成交量加權均價 |
| vol_ma20 | BIGINT | 20 日均量 |
| vol_ratio | DECIMAL(23,4) | 量比（當日量 / MA20）。**沒有業務上界**：MA20 走整數除法且有 `ma > 0` 守門，分母最小是 1，所以 `ratio ≤ current_volume`，而 volume 是 `BIGINT`。19 位整數位是為了涵蓋非負 `int64` 的完整值域，讓「不會溢位」在建構上成立，而不是依實測值估一個夠用的上限 |

---

## signals

訊號記錄，永久保存供回測分析。

| 欄位 | 說明 |
|------|------|
| signal_type | `BREAKOUT`, `BREAKDOWN`, `VOLUME_SPIKE` |
| direction | `BUY`, `SELL`, `WATCH` |
| vol_ratio | 觸發時的量比。`DECIMAL(23,4)`，與 `indicator_snapshots.vol_ratio` **同一個來源、同一個型別**——只放寬其中一張表等於沒放寬 |
| resistance / support | 觸發的阻力 / 支撐價位 |
| trend | 當時趨勢狀態（`BULLISH`, `BEARISH`, `SIDEWAYS`） |
| strength | 訊號強度，預設 1.0；有籌碼資料時會依 `chip_scores.signal` 上修或下修 |
| chip_signal | 評估當下使用的籌碼訊號；空值代表查無籌碼資料 |
| **ts** | **觸發當下的 K 棒時間戳，⛔ 不是寫入時間** |
| created_at | 實際寫入時間 |

⚠️ **統計每日訊號量要用 `created_at`，不是 `ts`**（2026-09-08 的 live 觀察時踩到，原記於 `todo.md` T-070，已收斂；現況見 `architecture.md`「live 運作觀察：四個交易日」）：
盤前那輪（08:50）產生的訊號帶的是**前一個交易時段**的 K 棒時間戳，用 `ts` 分組會把
它們算到前一天，做出「log 比資料表多幾筆」的假象——那個差額看起來很像
「訊號有產生但沒落盤」，實際上一筆都沒少。

---

## watchlists

監控清單，簡單的 symbol 清單。股票是否仍在交易所名單內由 `stock_symbols.is_listed`
判斷，不直接寫在 watchlist。

| 欄位 | 說明 |
|------|------|
| symbol | 股票代號（UNIQUE） |
| name | 股票名稱 |
| sector | 產業別（可空） |
| watched | 是否透過 WebSocket 即時監聽；同時最多 3 檔為 `true`（`store.MaxWatchedSymbols`），由 `PATCH /watchlist/:symbol/watch` 設定，超過上限回 409 |

---

## stock_symbols

股票主檔。`stock_symbol_sync` 每天從 TWSE ISIN 上市（strMode=2）＋上櫃（strMode=4）
清單同步有價證券資料；本次清單有出現的 symbol 會 upsert 並設 `is_listed=true`，原本
已上市但本次沒出現的 symbol 會以 `last_seen_at` 浮水印設 `is_listed=false`，用來簡化
watchlist 維護與下架標的判斷。任一來源抓取失敗、或快照涵蓋數明顯少於現有上市數（疑似
來源截斷）時整體略過本次同步，避免誤將大量個股標記下市。抓取遇到暫時性失敗時由排程整輪
重跑（`job_retry`，預設關閉），其餘由人工觸發手動同步；timeout、來源間隔與重跑的規則見
[data-pipeline.md 的 TWSE ISIN 同步策略](./architecture/data-pipeline.md#twse-isin-同步策略)。

| 欄位 | 說明 |
|------|------|
| symbol | 股票或有價證券代號（UNIQUE） |
| name | 名稱 |
| isin_code | ISIN Code |
| market | 市場別，例如 TWSE LISTED / 上市 |
| security_type | TWSE ISIN 頁面的分類列。**值是中文**：`股票`（1,945）、`ETF`（354）、`上市認購(售)權證`（31,090）、`上櫃認購(售)權證`（9,568）等 |
| industry | 產業別 |
| cfi_code | CFI Code |
| remarks | 備註 |
| listed_date | 上市日 |
| is_listed | 是否仍出現在最近一次 TWSE ISIN 同步清單 |
| delisted_date | 終止上市日期。**由 `delisting_reconcile` 投影或人工填入，⛔ 不參與 `is_listed` 的判定**，見下方 `delisting_events` |
| delisted_event_id | 上面那個日期的 provenance：非 NULL＝本 job 投影的（可被替換／撤銷）、NULL 但 `delisted_date` 非 NULL＝**人工填的，永不自動改動** |
| last_seen_at | 最近一次在來源清單看到的時間 |
| created_at / updated_at | 建立與更新時間 |

⚠️ **`delisted_date` 是敘述欄位，不是判定欄位。** 「還在不在交易」只看 `is_listed`
（ISIN 清冊缺席），⛔ 不要改成查 `delisted_date IS NOT NULL`——上櫃標的沒有第二來源
可補日期，那樣會把它們全部誤判成仍在交易。

⛔ **重新上市時兩個欄位必須一起清成 `NULL`**（`upsert` 的 `ON CONFLICT DO UPDATE` 已經這麼做）：
ISIN 清冊是 `is_listed` 的權威來源，它確認重新上市時留著終止日就會做出
「`is_listed = true` ＋ 有終止日」的不可能狀態。**人工值也要清**——
「人工值永不自動改動」只約束 `delisting_reconcile` 這支 job，不約束 current-state 不變量。

---

## delisting_events / delisting_source_snapshots

終止上市名單的**事件層**與來源快照（migration 076，T-071）。
排程行為與「為什麼不驅動 `is_listed`」見
[architecture.md 的終止上市對帳](./architecture.md)。

### 為什麼分兩層

`delisting_events` 記的是**歷史事實**——「來源 S 說代號 X 在日期 D 終止上市」，
同一代號可以有多代紀錄；`stock_symbols` 是 **current-state 主檔**，一個代號只有一列。
混在一起正是誤判的成因：TWSE 名單裡有 2002 年的「光寶電子(2301)」，
而今天的 2301 是「光寶科」，直接寫主檔就會在光寶科將來下市時回填上一代證券的日期。
分層之後，**證不出身分的事件仍被保存**供人工判讀。

| `delisting_events` 欄位 | 說明 |
|------|------|
| source | 來源識別，目前只有 `twse_suspend_listing` |
| symbol / company_name / delisted_date | 來源給的三個欄位（CSV 沒有 ISIN） |
| first_seen_at / last_seen_at | 首次與最近一次在來源看到 |
| missing_from_source_at | 非 NULL＝已從來源消失，**不參與投影**但保留 |
| — | 身分鍵 `UNIQUE (source, symbol, delisted_date)` |

**身分鍵含日期**，所以同一代號可以有多筆不同日期的事件（代號重用時必然如此）。
這也是 `job_runs.symbols_total` 記**distinct 代號數**而不是事件筆數的原因——
那個欄位的單位由全域契約定死是標的數。

⛔ **事件永不 `DELETE`，消失只標記**（`missing_from_source_at`），而且只在
**NULL → timestamp 的轉換當下**寫入。持續缺席不得重複改寫它，否則「來源更正」
計數每輪都 > 0，job 會永久 `partial`。**來源更正計數是邊緣觸發的**，
單位是 distinct event 數。

⛔ **改名／重現的計數不能用 upsert 的 `RowsAffected`**：正常路徑每輪都會更新
`last_seen_at`，一筆都沒更正也會被算成「全部都變了」；三種 engine 的 affected-row
語意還各不相同（MySQL 值未變回 0、更新回 2；postgres 一律 1），拿它當語意來源
本身就不可移植。實作改成在記憶體裡比對更新前後的狀態。
**首次缺席可以用 affected rows**——那句 SQL 帶 `WHERE missing_from_source_at IS NULL`，
predicate 自己保證了只有真的發生轉換的列才會被更新。

### `delisting_source_snapshots`：縮水防護的基準

每次**通過驗證並寫入**時記一列（`row_count` 是去重後的 canonical event 數，
⛔ 不是 CSV 原始列數，否則去重會被誤判成縮水）。下一輪比的就是上一筆 `accepted`。

**同一批 CSV 內的重複 key 在解析階段就處理完**，⛔ 不丟給 DB——postgres 的批次 upsert 在同一批內重複命中同一個
conflict target 會整批失敗（`ON CONFLICT DO UPDATE command cannot affect row a second time`）：
同 `(symbol, delisted_date)`、名稱相同 → 去重保留一筆（Info，來源重複列）；名稱不同 → 來源自相矛盾，**整輪解析失敗**
（名稱是投影的輸入，猜錯就是誤投影）。2026-09-04 實測 265 筆⛔ 沒有重複代號，但那是觀察、⛔ 不是保證。

⛔ **「上一筆」一律 `ORDER BY id DESC LIMIT 1`**：兩次連續執行可能落在同一個
`fetched_at`（秒級精度），只靠時間戳排序就變成未定義，基準會不確定。

⚠️ **`job_run_id` 的生命週期比快照短**：`job_runs` 只保留 30 天，而快照是縮水基準、
必須長期保存。所以它是 nullable ＋ `ON DELETE SET NULL`——一般 FK 會讓 `DeleteBefore`
失敗，`CASCADE` 更糟：會把縮水基準一起刪掉。

**快照必須與事件寫入在同一個 transaction**：它是下一輪的判斷基準，
「事件更新了但快照沒寫」會讓下一輪拿舊基準比新事件。

### 投影的 provenance：`delisted_event_id` 的所有權是單向的

| 狀態 | 意義 | 本 job 可否改動 |
|---|---|---|
| 兩欄皆 NULL | 沒有終止日 | 可投影（首次投影） |
| 兩欄皆非 NULL | job-owned | 可替換、可撤銷 |
| `delisted_date` 非 NULL、`delisted_event_id` NULL | **人工填的** | ⛔ 不可，只能告警 |
| `delisted_date` NULL、`delisted_event_id` 非 NULL | 不可能狀態 | 由 `CHECK` 擋掉 |

⛔ **人工值不得被靜默升級成 job-owned**：日期與來源完全相同時記 `manual_matched`
（Info，不是 degradation），**不補 `delisted_event_id`**。補了之後那筆人工值
就會在來源事件消失時被本 job 撤銷刪掉。

⛔ **FK 用 `RESTRICT`，與鄰近 `job_run_id` 的 `SET NULL` 相反，不可照抄**：
`delisted_event_id` 是**所有權標記**，被清成 NULL 會讓 job-owned 的投影被誤認成
人工值，從此不參與重算、**永久無法收斂**。事件本來就永不 DELETE，
所以 `RESTRICT` 不會擋到任何正常路徑。

`CHECK (delisted_event_id IS NULL OR delisted_date IS NOT NULL)` 擋的是半套狀態——
撤銷或重新上市時只清一欄的實作錯誤訊號。

### 投影規則：每輪重算，不列舉觸發條件

規則 1～5 是**候選過濾**（`evaluateCandidateFilters` 是唯一實作，
一筆事件只會落在其中一種結果）：

| 規則 | 條件 | 判定 |
|---|---|---|
| 1 | `is_listed = true` | 目前還在交易的**一律不碰** |
| 2／3 | 市場別非上市、`security_type` 非「股票」，或 `listed_date IS NULL` | 無法解析（`unresolvable`） |
| 4 | 終止日**早於**上市日 | 必然是上一代證券（`generation_gap`） |
| 5 | 正規化後的公司名稱不符 | 代號重用（`identity_doubt`） |
| — | 以上全數通過 | 成為候選 |

規則 6 是**數量仲裁**（同一代號有多筆候選＝歧義，不投影）；
規則 7 是 **ownership 四態仲裁**（上面那張表）。

**名稱正規化是封閉的**（`NormalizeCompanyName`），固定順序：
①全半形統一（NFKC）→ ②去除所有空白 → ③**只從字串尾端**移除 allowlist
（`-創櫃`、`-創`、`-DR`、`-KY`，由長到短、最多移除一次）。
⛔ `-DR` 只在結尾移除，出現在中間不動；不得用「切到第一個 `-`」這類規則。
⚠️ **名稱比對是啟發式，不是身分證明**（公司會改名，CSV 沒有 ISIN），
定位是**只准不放**：不符就不寫入、等人工，⛔ 不會因為名稱相符就放寬其他任何一條。

**每輪重算而不是列舉觸發條件**：改名後不再相符、兩筆候選的歧義、
替代事件不合格、主檔自己變了（改名／`listed_date`／`market`／`security_type`）
——這些都會讓既有投影失去資格，而「列舉觸發條件」的寫法會全部漏掉。

### DATE 欄位的比較：⛔ 不做時區換算

`delisted_date` 是 **DATE**——沒有時刻也沒有時區，但各 driver 交還的 `time.Time`
帶的 location 不同：

| engine | 交還的值 |
|---|---|
| postgres（pgx） | `2026-09-01 00:00:00 UTC` |
| mysql（`parseTime=true` ＋ `loc=Asia/Taipei`，見 `config.yaml` 的 DSN 範例） | `2026-09-01 00:00:00 +08:00` |

⛔ **先 `.UTC()` 再取日期會讓後者變成 `2026-08-31`**：事件身分鍵、事件 id 查找與
「同一天」比對整批差一天，而且**不會有任何東西報錯**。
一律用 `store.DateKey` / `store.SameCalendarDay`（取掛鐘日期）。
（2026-09-08 review 修正；回歸測試 `TestDateKeyUsesCalendarDateNotUTC` 與
`TestMySQLMigrationsDelistingDateKeyUnderTaipeiLoc`——後者刻意另開一個帶 `loc` 的連線，
因為 `scripts/test-mysql-migrations.sh` 的 DSN 沒帶它。）

⚠️ `stock_symbols.delisted_event_id` 在 Go 端用 **`store.NullInt64`**，不是
`sql.NullInt64`：後者會序列化成 `{"Int64":123,"Valid":true}`，而 `omitempty`
對 struct 無效。這個欄位會經 `GET /stocks` 系列送到前端。

### 寫入的 CAS 契約

投影與撤銷都是 compare-and-swap，條件是**判定當下讀到的那份 snapshot**
（含 `name` / `market` / `security_type` / `listed_date` / `delisted_date` /
`delisted_event_id`）。落空就記 `concurrency_conflict`（投影）或
`revocation_conflict`（撤銷），該輪 `partial`，⛔ 不重試。

⛔ **撤銷的 CAS 條件必須與投影完全相同**：只比 ownership 的話，
「因名稱不符而準備撤銷、期間名稱被修正成相符」仍會清除那筆已經重新成立的投影。

三個 engine 的分歧點，都已在實作處理：

* ⛔ **snapshot 讀取用一般 `SELECT`，不可 `FOR UPDATE`**——鎖住之後並行更新只能等到
  本交易結束，CAS 就永遠不會落空，衝突路徑變成不可達（連測都測不到）。
  鎖只出現在**最終寫入點**的 guarded read（pg/mysql 是 `FOR UPDATE`；
  SQLite 沒有 `FOR UPDATE`，靠的是同一交易內的單 writer 契約）。
* ⚠️ **SQLite 的交易用 `BEGIN IMMEDIATE`，不是預設的 deferred**（2026-09-08，原記於 `issue.md` I-111，已收斂）：
  本交易先讀後寫，而 SQLite 在「已經讀過、要升級成 writer」時**不呼叫 busy handler**，
  會直接回 `SQLITE_BUSY`——DSN 上的 `busy_timeout=5000` 對它形同不存在。
  `BEGIN IMMEDIATE` 把等待移到交易開頭，外部 writer 短暫持鎖時才等得到。
  ⛔ 實作必須綁在**單一 `*sqlx.Conn`** 上，且 rollback 失敗時要丟棄該連線
  （`Raw` 回 `driver.ErrBadConn`），不能把還開著交易的連線放回 pool。
  ℹ️ 只有 SQLite 分支這樣做，PostgreSQL／MySQL 維持 `BeginTxx`。
  **行為**：外部 writer 持鎖短於 `busy_timeout`（5 秒）→ 等待後成功；超過 → 整輪失敗
  （那筆 `job_runs` 仍寫得回去，`finishRunStatus` 是單句寫入）。
  由 `TestWithImmediateTx*`（四條：等待成功／逾時失敗／rollback 後連線可重用／
  ctx 已取消）與排程層的 `TestDelistingReconcileWaitsOutShortExternalWriter`、
  `TestDelistingReconcileExternalWriterBeyondBusyTimeoutWritesFailed` 釘住。
* ⚠️ **`busy_timeout` 與 `foreign_keys` 是 connection-local 的 pragma，一律由 DSN 帶**（`store/sqlite.go` 的 `enforcedSQLitePragmas`）：
  用一次 `Exec` 設定只對 pool 裡的那一條連線有效，其他連線照樣是預設值（FK 預設是關的）。
  兩個都是強制政策——DSN 裡原本同名的 `_pragma` 先移除、再加入唯一一份固定值，⛔ 不能被外部設定覆寫成 0；其他 `_pragma` 照留。
  `journal_mode=WAL` 是寫進 DB 檔的持久設定，⛔ 不是 connection-local，可以用 `Exec` 設。
* ⛔ **nullable 欄位要 null-safe**：以 `(col IS NULL AND ? IS NULL) OR col = ?`
  的形式組出，不用 engine 專屬的 `IS NOT DISTINCT FROM` / `<=>`。
  寫成 `col = NULL` 永遠不為真，那些列就永遠撤銷不掉。
* ⛔ **MySQL 的字串比較要 binary**（`BINARY col = BINARY ?`）：預設 collation
  不分大小寫，CAS 會在名稱只有大小寫差異時誤判成沒變，於是拿過期的仲裁結果寫入。
* ⛔ **DB error 不是 CAS 落空**：`Project` / `ConfirmProjection` / `Revoke` 回 error 時
  **整輪 rollback**，⛔ 不得記成 X／RX 再繼續寫 accepted 快照——MySQL／SQLite 的單句
  錯誤不會中止整個 transaction，那會 commit「事件寫了、快照寫了、投影沒寫」的半套狀態。
* ⛔ **不可用 no-op `UPDATE` 再看 affected rows 判斷「值相同」**：MySQL 把欄位更新成
  相同值時通常回報 0 affected rows，一個穩定且完全正確的投影會**每天被誤判成衝突**。
  「值相同」走的是帶同一組 CAS 條件的 current read。

## users

使用者帳號，密碼以 bcrypt 雜湊儲存。

| 欄位 | 說明 |
|------|------|
| email | 唯一識別，作為登入帳號 |
| password_hash | bcrypt hash，cost=10 |
| status | `active`（可登入）或 `inactive`（預設，需管理員啟用） |
| created_at | 建立時間 |

> 新帳號預設 `inactive`，需要透過 `PATCH /users/:id/status` 或前端使用者管理頁面手動啟用後才能登入。

---

## backtest_jobs

回測任務佇列，Go 寫入後由 Python 消費。

| 欄位 | 說明 |
|------|------|
| job_id | UUID 字串，唯一識別 |
| strategy | 策略名稱，e.g. `breakout_v1` |
| symbols | JSON 陣列，e.g. `["2330","2454"]`（PostgreSQL 為 JSONB） |
| timeframe | K 棒週期 |
| start_date / end_date | 回測區間 |
| status | `pending` → `running` → `done` / `failed`（轉移條件見下表） |
| execution_owner | 認領這一筆的 python 服務：`worker` 或 `server`；NULL＝還沒被認領，或 migration 077 之前的舊資料。⛔ 不對外（Go 的 `BacktestJob` 與 python 的 `GET /backtest/{job_id}` 都明列欄位、不含它） |
| error | 失敗原因；`pending`／`running` 時是 NULL（Go 的 `GetJob`／`ListJobs` 用 `COALESCE(error, '')` 讀，對外 `error,omitempty`）。取消是 `cancelled by user`，重啟收掉的是 `interrupted_on_restart`。I-137 起 python 只寫封閉值域的代碼（形式見 `architecture.md`「寫入失敗的一致性契約」的合法形式表），三個讀出端點回傳前再比對同一份規則；I-137 之前留下的原始例外文字⛔ 不改 DB，對外改回 `internal_error` |
| trigger_source | `manual`（API 觸發）。**DB 欄位名是 `trigger_source`，JSON／API 欄位名仍是 `trigger`**——`trigger` 是 MySQL 保留字，migration 059 改名（見 issue.md I-054） |
| use_chip_filter | 是否在模組化回測中用 `chip_scores.total_score` 過濾進場 |
| chip_min_score | 籌碼 filter 門檻（-100～100）；缺資料視為 0 |
| started_at / finished_at | 執行時間戳；一律由狀態轉移的 SQL 寫 DB 的 `CURRENT_TIMESTAMP`（⛔ 不由 python 組時間字串）。migration 077 之前的舊資料是 python 用 `datetime.utcnow().isoformat()` 寫的不帶時區字串，照寫入當時連線的 TimeZone 解讀；⛔ 沒有改舊資料 |

**狀態轉移**（I-134；每一個都是條件式的 UPDATE，影響 0 列就代表這一筆已經不是自己的，⛔ 不再往下寫）：

| 轉移 | 誰 | 條件 | 影響 0 列時 |
|---|---|---|---|
| 建立 → `pending` | backend | （INSERT） | — |
| `pending` → `running` | worker 或 server（認領） | `job_id = ? AND status = 'pending'`，同時寫 `execution_owner`、`started_at` | 認領失敗：⛔ 不執行（server 回 409） |
| `pending` → `failed`（取消） | backend `DELETE /backtest/:job_id` | `job_id = ? AND status = 'pending'` | 再查一次：不存在 404、存在 409 |
| `running` → `done` | 認領者 | `job_id = ? AND status = 'running' AND execution_owner = 自己`；跟結果、交易明細**同一個交易** | 整個交易 rollback（結果與明細都⛔ 不留） |
| `running` → `failed`（執行出錯） | 認領者 | 同上 | 只記 log |
| `running` → `failed`（`interrupted_on_restart`） | 同一種服務的下一個程序，啟動時 | `status = 'running' AND execution_owner = 自己` | — |

* 兩條路徑都在用：backend 寫入 `pending` 之後呼叫 python-server 的 `POST /backtest`（`PYTHON_SERVICE_URL`），python-worker 也每 10 秒輪詢 `pending`。
  server 在**回應之前同步認領**，所以回應在路上遺失時任務已經是 `running`／`server`，worker 撈不到；同一筆兩邊同時認領時只有一方成功。
* **重啟收尾的前提：每種服務只有一個程序**。所有 `execution_owner = 自己` 的 `running` 都是上一個程序留下的，才能直接收掉。會破壞這個前提的變更：

  | 服務 | 現況 | 會破壞前提的變更 |
  |---|---|---|
  | python-worker | compose 一個 service、`python worker.py` 單一程序、依序執行 | `--scale`、多副本、在 container 裡另開執行緒或程序跑任務 |
  | python-server | compose 一個 service、`python http_server.py`（`uvicorn.run`，沒有 `workers`、沒有 `reload`）；回測在 FastAPI 的背景工作裡，與 HTTP 同一個程序 | uvicorn `workers>1`、`--reload`、gunicorn 多 worker、多副本 |
  | 新舊重疊 | compose 的重建是先停舊的再起新的；`deploy.sh` 是 `down` → `up` | 「先起新的再停舊的」的滾動更新 |
  | 本機的 `python/start_server.sh` | `uvicorn … --reload`，只給本機開發 | ⚠️ 不在保證範圍內：reload 時新程序若比舊程序先起，會誤收舊程序的任務。⛔ 不用它連 dev 或 live 的 DB |

  要改成上面任何一種，先把 `execution_owner` 換成實例識別（並重新設計收尾），⛔ 不能直接改。
* 重啟收掉的任務⛔ 不自動重跑，要的話重新送出。
* **schema 時序**：compose 裡 python ⛔ 不等 backend、migration 由 backend 啟動時套用，所以 python 兩個服務啟動時先等 `execution_owner` 與 `backtest_results.data_coverage`（078；I-137）都出現
  （每 2 秒查一次，整體最多 120 秒；每次探測本身也只等剩餘時間——DB 卡住時探測在 daemon thread 裡被放棄，探測回來時已過期也算逾時；
  逾時以非 0 結束、交給 compose 重啟）。⛔ 不在 python 端建立欄位。
* migration 077 之前的 `running`（`execution_owner` 是 NULL）兩種服務都⛔ 不會收，要由人確認後處理。live 首次上版（2026-11-18）時，部署前與
  舊程序退出後各查一次，這種列都是 0 筆，⛔ 沒有需要處理的。之後新程式認領時一定會寫 `execution_owner`，不會再產生新的 NULL `running`。
  當時的流程（停止新提交 → 盤點 → 記下舊程序 → 部署 → 確認舊程序退出 → 只處理清單內仍是 NULL 的列）見
  `git show 85849b9:docs/plans/i134-backtest-claim.md` 的 6.2。
* **程式註解裡的 I-134**：I-134 在 2026-11-19 收斂。程式、測試、腳本與三份 migration 077 檔頭裡的「（I-134）」「`issue.md` I-134」是**由來標記**，
  記的是這段為什麼這樣寫，規格以本節與 `architecture.md` 的回測流程為準。⛔ 不為了改這些標記去動 migration 檔。
  I-134 的內容是 python 服務重啟後，執行中的回測一直停在 `running`。修正包括：原子認領與 `execution_owner`、結果與明細同一個交易提交、
  重啟時收掉自己留下的 `running`、server 在認領失敗時回 409，以及一併修掉的兩個既有問題：Go 讀不到 `error` 是 NULL 的任務、
  worker 讀到 PostgreSQL 原生型別的 `symbols`／日期。

---

## job_runs

排程執行紀錄，由 Go scheduler 寫入。`pre_market`、`intraday`、`daily_close`、
`sr_zone_verify`、`chip_daily_sync`、`stock_symbol_sync` 都使用這張表；
manual/backfill 籌碼同步另用 `chip_sync_jobs`。

| 欄位 | 說明 |
|------|------|
| job_name | 排程名稱（`VARCHAR(64)`，migration 063 由 20 放寬——`corporate_action_sync` 是 21 字元，原本的上限裝不下且**失敗時只記 log 不中斷排程**，症狀是「job 有跑但狀態頁看不到」） |
| status | `running` / `success` / `partial` / `failed` / `skipped` |
| symbols_total / symbols_failed | 本輪處理與失敗數 |
| error | 最後一筆錯誤或摘要訊息 |
| started_at / finished_at | 執行時間戳 |

---

## intraday_rounds

盤中一輪一列（migration 079；T-088＋I-103）。`run_id` 是 `job_runs` 的 id（⛔ 不設外鍵：保留期清理在開盤前跟 `job_runs` 同一步刪）。

| 欄位 | 說明 |
|------|------|
| run_id | `job_runs.id`（主鍵） |
| source | `scheduled`（盤中排程）／`refill`（手動補抓） |
| scheduled_at | 排定的時點 |
| lock_acquired_at | 拿到鎖、實際開始執行的時刻；等鎖逾時是 NULL |
| exec_finished_at | 實際結束的時刻；還在跑、中斷或沒有執行是 NULL |
| lock_wait_ms | 等鎖的時間 |
| outcome | `executed`／`skipped_lock_timeout`（等鎖逾時、⛔ 不執行）／`failed`（在逐檔結果之前就失敗，例如讀不到自選清單；⛔ 沒有逐檔的列） |
| evidence_failures | 逐輪存證寫失敗的次數；> 0 的輪次是「對帳不完整」 |

## intraday_symbol_runs

盤中每一輪、每一檔一列，主鍵 `(run_id, symbol)`（migration 079）。行情段在抓取與寫入之後 insert，評估段在那一檔評估完成之後 update；
手動補抓（`source=refill`）的列沒有評估段。保留期跟 `job_runs` 相同（約 600 列／天）。

| 欄位 | 說明 |
|------|------|
| run_id／symbol／source | 同 `intraday_rounds` |
| data_at | 寫行情段的時刻 |
| data_status | `written`／`no_data`／`store_failed`／`fetch_failed` |
| data_cause | 安全原因碼（`market.TransientCause` 的值域，另加 `retry_budget`、`refill_deadline`）；written／no_data 是 NULL |
| last_bar_ts | 這次寫入的最後一根的時間；written 以外是 NULL |
| fingerprint | 寫入之後 DB 裡最近 120 根 1 分 K 的 SHA-256（輸入指紋）；written 以外是 NULL |
| fetch_attempts | 這一檔所在那一批的嘗試次數（1～3） |
| http_recovered | 第一次失敗、後來 HTTP 成功（⛔ 不等於寫入成功） |
| eval_at／eval_status／eval_cause | 評估段。`eval_status`：`no_signal`／`signal_sent`／`signal_suppressed`／`withheld_stale`／`degraded`／`evaluate_failed`／`not_evaluated`；`not_evaluated` 的 `eval_cause` 是 `fetch_failed`／`store_failed`／`no_data`／`unchanged` |

* 回滾：兩張都是新表，退回舊 image 即可、⛔ 不用退 migration（舊程式不讀不寫它們；goose 遇到較新的 schema ⛔ 不報錯，見 `backtest_results` 的 078 說明）。

---

## backtest_results

回測績效摘要，Python 寫入後由 Go API 讀取。

| 欄位 | 說明 |
|------|------|
| total_return | 總報酬率 |
| annual_return | 年化報酬率 |
| win_rate | 勝率 |
| max_drawdown | 最大回撤 |
| sharpe_ratio | 夏普比率 |
| total_trades / win_trades / loss_trades | 交易統計 |
| avg_pnl | 平均損益 |
| data_coverage | 每一檔實際取到的資料範圍，JSON 陣列存成 TEXT（migration 078；I-137）。NULL＝未記錄覆蓋範圍（I-137 上版之前，以及回滾到舊 image 期間寫入的結果），⛔ 不回補 |

backtrader 路徑（`STRATEGY_MAP`）在 I-136 之前的結果，費用偏低、報酬被高估；新舊界線見 `backtest_trades` 的「費用的新舊界線」。

**`data_coverage` 的格式**（I-137）：陣列，一檔一筆，順序跟請求的 `symbols` 相同（含被略過的）：

| 鍵 | 說明 |
|---|---|
| `symbol` | 股票代號 |
| `first_bar`、`last_bar` | 範圍內實際取到的第一根、最後一根的台北日期（`YYYY-MM-DD`）；被略過時是 null |
| `bars` | 範圍內的根數 |
| `warmup_bars` | 實際取到的暖身根數（起點之前，最多 `warmup_required`） |
| `warmup_required` | 需要的暖身根數（config 的 `backtest.warmup_bars`，目前 250） |
| `skipped` | 範圍內 0 根、這一檔被略過時是 true（這時⛔ 不查暖身段，`warmup_bars` 是 0） |

* **首尾日期與暖身是否足夠分開記**，⛔ 不合成一個「完整／不完整」的狀態：`first_bar` 晚於 `start_date` 可能只是起點遇到週末或休市，
  也可能那一檔後來才有資料，⛔ 不判定是哪一種；暖身不足（例如 DB 的第一根剛好在起點當天）是另一件事。
* **範圍內的缺漏⛔ 不偵測**（例如中間少了一個月）：要判定漏了交易日需要獨立的交易日依據，這裡沒有。
* 全部被略過時任務 `failed`（`no_data_in_range`），不會寫結果。
* **新舊界線**：I-137 部署之前的回測都先取「最近 N 根」再切（backtrader 1000 根、modular 2000 根），範圍的前段可能被靜默截掉，
  而且沒有暖身（指標從範圍首根冷啟動）、modular 的統計從第 `min_bars` 根起算。⛔ 不改、⛔ 不回補；要新的結果就另建 job 重跑。
  live 的界線：commit `31e5566`，backend image `bc503920bb75`、python-server `f166cc4e58c5`、python-worker `4303b51148a2`，
  界線時間 2026-11-20 14:10:35（台北），是兩個 python 服務中較晚啟動的時間（worker 14:10:34）；migration 078 由同一次部署的 backend 套用。
  部署時 `backtest_results` 7 筆，`data_coverage` 全是 NULL（⛔ 沒有回補）。上版後的實跑 `bt_20261120_061311_000`（`breakout_v1`、2330、
  2024-01-01～2025-12-31）：`bars` 485，等於 DB 在這段台北日期的日 K 根數，暖身 250／250。
  * **回滾不用退 078**：欄位可為 NULL。舊 python 的寫入、舊 Go 的結果查詢都明確列欄位，舊 python 的 schema 等待也不看它；
    goose（v3.21.1）在 DB 版本高於舊 binary 認得的最大版本時⛔ 不報錯、也不執行任何 migration。
    ⚠️ 舊 python-server 的 `GET /backtest/{job_id}` 讀結果用 `SELECT *`，回滾期間會把保留下來的 `data_coverage` 以**原始的 JSON 字串**帶出
    （新版才解析成陣列）；Go 的 API 不受影響。退回舊 image 之後新寫的結果 `data_coverage` 是 NULL。
  * 部署紀錄（部署前後的核對）原文見 `git show 64019c4:docs/plans/i137-backtest-date-range.md` 的「七之二」。
  * **部署前的唯讀盤點**（2026-11-20 10:08，live 當時是 `03e7028`）：以每筆任務的 `started_at`（沒有就 `created_at`）為準，重建當時看得到的最近 N 根
    （K 棒的 `ts` 與 `created_at` 都早於那個時間），數範圍內（台北日期）落在 N 根之外的根數。`done`／`failed` 共 9 筆任務、41 組任務×標的，
    **被截掉的都是 0**（範圍都在當時的最近 N 根之內）；pending／running 0 筆。⚠️ 這是估算：K 棒被刪掉、或事後被改寫（⛔ 不改 `created_at`）都無法還原。
  * **`breakout_v1` 送了多檔的舊任務**：1 筆，`bt_20260701_073955_000`（`done`，5 檔 2330、0050、00947、00830、00981A），只交易了第一檔
    2330（舊版只看第一檔成功載入的標的），結果⛔ 不代表其他四檔。I-137 起 `breakout_v1` 限單檔（真正的多檔見 `issue.md` I-138）。

---

## backtest_trades

每筆回測交易明細（只列已平倉的交易）。兩條回測路徑寫同一組欄位；**backtrader 路徑**（`STRATEGY_MAP`，例如 `breakout_v1`）的定義如下，
全部從 backtrader 的成交紀錄（`tradehistory=True` 的 `Trade.history`）算（I-135）：

| 欄位 | 說明 |
|------|------|
| job_id | FK → backtest_jobs.job_id |
| symbol | 股票代號 |
| direction | 方向：多方 `BUY`、空方 `SELL`（第一筆成交的正負號） |
| entry_time / exit_time | 第一筆進場成交、最後一筆出場成交的時刻。寫成帶 `+08:00` 的台北時間（由 UTC 時刻換算）；日 K 的成交時刻是台北的 `00:00:00` |
| entry_price / exit_price | 進場成交、出場成交各自依股數加權的平均價（2 位）。⛔ 不用 backtrader 的 `Trade.price`：減碼後再加碼時它是剩餘部位的成本，⛔ 不是全部進場成交的均價 |
| size | 進場成交的股數合計（正數） |
| pnl | 淨損益（毛損益扣掉費用，2 位） |
| pnl_pct | 淨損益 ÷ **未四捨五入**的進場成交金額，比例值（`0.01`＝1%；`DECIMAL(8,4)`）。modular 路徑是「淨損益 ÷ 進場時該標的的 equity」，目前全額投入，分母數值相同；口徑統一見 `todo.md` T-085 |
| commission | 分攤到這筆交易的**模擬總費用**＝手續費＋賣出的交易稅（＝這筆交易各成交事件的費用合計；反手的委託會拆成兩筆交易、費用依股數分攤）。費用模型是「**設定費率的模擬模型**」（I-136）：config 的 `commission_rate`（買賣都收）與 `tax_rate`（賣出收，空單的賣出在進場），都是比例值；⛔ 沒有最低手續費、⛔ 不取整數、⛔ 沒有商品別稅率（ETF、當沖等），見 `todo.md` T-085。新舊界線見本表下方 |

* **寫入前的核對**（不符就 raise，讓任務 `failed`，⛔ 不寫出看起來正常的錯誤數字）：
  * 成交的股數、價格、費用、時間，以及 backtrader 的損益與費用，都要是**有限數**（NaN、正負無限大一律報錯）。這一步在任何運算之前做：
    NaN 的大小比較一律不成立，後面的核對擋不住它。
  * 部位在同一筆交易裡⛔ 不變號、最後回到 0；未四捨五入的毛損益等於 backtrader 的 `Trade.pnl`；`Trade.commission` 等於各成交事件的費用合計。
  * 這些核對採用**絕對容許值 `1e-6`**（⛔ 不乘上金額）。這是採用的值，⛔ 不是對任意金額的保證；測試以大額（單筆成交金額約
    1e8～4e8）、多筆加減碼交錯的多空交易支持它目前夠用。
* **精度界線**：欄位是四捨五入過的，⛔ 不能拿來精確反算。毛損益在**未四捨五入**的成交資料上等於 backtrader 的損益（見上一點）；
  存下來的值符合 `|d × (exit_price − entry_price) × size − (pnl + commission)| ≤ 0.01 × size + 0.01`，`d` 是方向（`BUY` +1、`SELL` −1）。
  例：買 3@102、賣 1@104、賣 2@105，毛損益 8；出場均價存成 104.67，反算是 8.01。
* **費用的新舊界線**（I-136）：修正之前 backtrader 路徑的 broker 扣的費用**偏低**（手續費只有設定值的 1/100、完全沒有交易稅），
  所以 `commission` 偏低、`pnl` 與 `backtest_results` 的報酬、平均損益被高估，勝率等統計也可能不同。既有資料⛔ 不改、⛔ 不回補；
  要新的結果就另建 job 重跑。live 的界線：commit `03e7028`，python-server image `40702c240772`、python-worker `c58a5cce2c69`，
  界線時間 2026-11-19 14:22:38（台北），是兩個 python 服務中較晚啟動的時間（worker 14:22:37）。
  部署期間沒有排隊或執行中的回測，所以這之後建立的 backtrader 任務才是新的費用。界線之前 live 只有 1 筆 `breakout_v1`
  （2026-07-01，`done`、0 筆交易），沒有受影響的數字。modular 路徑⛔ 不受影響
  （它用自己的費用計算，費率同樣由 config 傳入）。
* **時間與資料庫**：帶 offset 的字串寫進 PostgreSQL 的 `TIMESTAMPTZ`，存的時刻跟連線的 TimeZone 無關（`scripts/test-postgres-migrations.sh` 會跑的測試驗證）。
  ⚠️ 只保證 PostgreSQL：MySQL 的 `DATETIME` 收到帶 offset 的值仍會換成連線的時區，SQLite 存的是字串本身。
* **程式註解裡的 I-135／I-136**：兩筆都在 2026-11-19 收斂。程式、測試與腳本註解裡的「（I-135）」「（I-136）」是**由來標記**，記的是這段為什麼這樣寫，
  規格以本節為準：
  * **I-135**：`breakout_v1` 只要有交易就失敗——`strategy._trades` 是兩層（data → tradeid → 交易的 list），原本只走了一層。
    修正時一併改成從成交紀錄算明細、時間帶 `+08:00`（當時的裁決 D2）、寫入前核對，以及共用的明細查詢。
  * **I-136**：broker 的手續費被除以 100、實際扣費沒有交易稅 → `TWCommission` 設 `percabs=True`、⛔ 不覆寫 `getcommission()`、
    賣出（含開空單）課稅，engine 明確傳入 config 費率；`commission` 定義為含稅的模擬總費用，以及共用的費用查詢。

---

## stock_analyses

個股現況分析快照，Go 呼叫 Python 計算後寫入；`trade_verification`/
`verified_at` 由 `POST /api/v1/analysis/:id/verify` 更新（可重複執行，每次
重新計算，非一次性狀態機）。詳見 [stock-analysis.md](./stock-analysis.md)。

| 欄位 | 說明 |
|------|------|
| symbol / timeframe | 分析標的與週期 |
| analyzed_at | 分析當下最後一根K棒的時間（驗證時只看嚴格晚於此時間的資料） |
| current_price | 分析當下收盤價 |
| trend | `BULLISH` / `BEARISH` / `SIDEWAYS` |
| entry_status | `ACTIVE`（已觸發真正進場條件）/ `WATCHING`（觀察中的觸發價位） |
| entry_direction | `LONG` / `SHORT` / `NONE` |
| entry_price | 已觸發：實際進場價；觀察中：觸發價位 |
| entry_reason | 人類可讀的判斷依據 |
| stop_loss_atr / stop_loss_structural / stop_loss_composite | 三種停損價位 |
| take_profit_next_level / take_profit_risk_reward / take_profit_atr | 三種停利價位 |
| trade_verification | JSON：每個停損/停利方法各自「有沒有被觸及、何時、什麼價位」；`entry_status=WATCHING` 時為 `{"applicable": false}` |
| verified_at | 最近一次驗證時間，`NULL` 代表尚未驗證過 |

**Index：** `INDEX(symbol, created_at DESC)`，支援查某檔股票的歷史分析列表。

---

## stock_analysis_levels

`stock_analyses` 底下的支撐/壓力位清單（一對多），驗證時逐筆更新。

| 欄位 | 說明 |
|------|------|
| analysis_id | FK → stock_analyses.id |
| price | 價位 |
| type | `SUPPORT` / `RESISTANCE` |
| strength | 強度（0~1，越高代表訊號越強/越多方法認同） |
| method | 產生此 level 的演算法：`swing` / `atr_channel` / `volume_profile_poc` / `volume_profile_vah` / `volume_profile_val` |
| status | `PENDING`（尚未驗證）/ `HELD_SO_FAR`（目前為止沒被突破）/ `BROKEN`（已被突破） |
| broken_at / broken_price | 第一次被突破的時間與收盤價（`status=BROKEN` 時才有值） |

---

## stock_sr_zone_analyses

SR Zone Scoring 分析快照（機構級版本，見
[sr-zone-scoring.md](./sr-zone-scoring.md)），Go 呼叫 Python `POST /sr-zones`
計算後寫入。驗證機制跟 `stock_analyses` 一樣是純 Go（`SRZoneVerifier`，見
sr-zone-scoring.md「十四」），差異在 zone 是價格區間而非單一價位。
JSON 欄位在 PostgreSQL 為 `JSONB`；SQLite / MySQL 以文字 JSON 儲存，Go 端統一使用
`store.RawJSON` 讀寫。

| 欄位 | 說明 |
|------|------|
| symbol / timeframe | 分析標的與週期 |
| analyzed_at | 分析當下最後一根K棒的時間 |
| current_price | 分析當下收盤價 |
| global_trend | 股票層級趨勢（`trend_slope`），同一次分析所有 zone 共用，只存一次 |
| global_volatility | 股票層級波動率（`ATR / close`），同一次分析所有 zone 共用，只存一次 |
| global_expected_value | 所有「有明確方向」的 zone 依 confidence 加權平均的 EV，是唯一收斂的權威數字；只有完全沒有 zone 解析出明確方向（zones 為空或全部 `AT_ZONE`）時才是 `NULL` |
| global_confidence | 所有 zone confidence 的簡單平均（不論 zone 有沒有明確方向都計入）；只有 zones 陣列本身是空的時候才是 `NULL`，跟 global_expected_value 的 `NULL` 條件不同 |
| global_risk_reward_ratio | 所有「有明確方向」的 zone 依 confidence 加權平均的 RR；`NULL` 條件同 global_expected_value |
| model_version | 產生這筆分析所用的模型版本（目前為 `"v4"`）；Python 端萬一沒回傳則寫 `"unknown"` |
| pipeline_version | 分層 API/持久化契約版本，目前為 `"v2"` |
| evidence | 分析層全局 Evidence JSON，包含 SHAP explainer metadata、global metrics 與籌碼快照。evidence 可降級：shap 未安裝／模型缺 v4 background／`evidence_enabled=false` 時 `model.explainer=null`、`model.evidence_available=false`（見 [sr-zone-scoring.md](./sr-zone-scoring.md)「十九」） |
| explanation | JSON：分析層白話解釋（`schema_version`/`summary`/`action_reason`/`market_drivers`/`risk_notes`/`model_context`），由 Python explain engine 依 Score/Evidence/Decision 產生的純展示層，不影響決策；`schema_version` 目前為 `sr_explain_v1`，舊資料為 JSON `null` |
| scenario | JSON：分析層結構化情境（`schema_version`/`state`/`title`/`summary`/`trigger_conditions`/`invalidation_conditions`/`market_regime`/`primary_zone`/`global_confidence`），由 Python scenario engine 依 decision/score/explanation 產生的純展示層，不改機率/分數/決策；`schema_version` 目前為 `sr_scenario_v1`，舊資料為 JSON `null` |
| model_config_hash | 訓練這個模型時的 `DatasetConfig`/zone builder 參數/`model_type`/`calibration_method` 快照的短 hash（比 `model_version` 更細），見 [sr-zone-scoring.md](./sr-zone-scoring.md)「十六」；比這個欄位還舊的分析為空字串 |
| period_summaries | JSON：短/中/長期摘要卡的支撐/壓力摘要 |
| analysis_tips | JSON：前端顯示的白話提示陣列 |
| chip_summary | JSON：整檔層級籌碼拆解；查無資料時為 `{"missing": true, ...}`，舊資料可能為 JSON `null` |
| decision_summary | JSON：Semantic Pipeline、Market Regime、Primary Zone、風險提示等前端預設閱讀層 |

**Index：** `INDEX(symbol, created_at DESC)`。

---

## stock_sr_zones

`stock_sr_zone_analyses` 底下的 zone 清單（一對多）。跟 `stock_analysis_levels`
不同：每個 zone 是一段**價格區間**（`price_low`~`price_high`），不是單一
價位，且欄位數量遠多於 Level（含機率、EV、RR、量能確認等 ML 產出的數字）。
JSON 欄位在 PostgreSQL 為 `JSONB`；SQLite / MySQL 以文字 JSON 儲存。

| 欄位 | 說明 |
|------|------|
| analysis_id | FK → stock_sr_zone_analyses.id（Go 端手動 2 步刪除，非 DB `ON DELETE CASCADE`） |
| zone_uid | 這個 zone 的跨交易日身分 → `zone_instances.zone_uid`。**可空、刻意不加外鍵**，語意見下方 |
| price_low / price_high | 區間上下緣 |
| method | `atr` / `volume_profile` |
| role | `SUPPORT` / `RESISTANCE` / `AT_ZONE`（分析當下依現價判斷，寫入後不會再變動） |
| resolved_role | `SUPPORT` / `RESISTANCE` / `NULL`。只有 `role=AT_ZONE` 的 zone 在後續驗證（`POST /sr-zones/:id/verify`）時，價格真正收盤離開區間後才會被解析並寫入；`role != AT_ZONE` 的 zone 永遠是 `NULL`（角色從分析當下就已明確，不需要另外解析）。前端判斷「這個 zone 現在算支撐還是壓力」應優先看 resolved_role，沒有值再退回 role，見 [sr-zone-scoring.md](./sr-zone-scoring.md)「十四」 |
| tier / tier_label | 依區間寬度分三層：`TIER_1_MAIN_STRUCTURE`（主結構）/ `TIER_2_TRADING_ZONE`（交易區）/ `TIER_3_SHORT_TERM`（短期支撐） |
| support_score / resistance_score | 依機率貝式收縮而來的強度分數（0~1） |
| net_score / net_score_label | `support_score - resistance_score`；`STRONG_SUPPORT` / `NEUTRAL` / `STRONG_RESISTANCE` |
| confidence / confidence_level | 多因子可信度（樣本數/時間衰減/歷史穩定度）；`LOW`/`MEDIUM`/`HIGH`/`VERY_HIGH` |
| bounce_probability / break_probability | 反彈/跌破機率（`role=AT_ZONE` 時為 `NULL`） |
| expected_gain / expected_loss / expected_value | 角色解析後的平均反彈/跌破報酬、加權期望值 |
| risk_reward_ratio / reward_risk_percentile | `|expected_gain/expected_loss|`；此比值在訓練資料歷史分佈中的百分位 |
| relative_volume / volume_confirmation | 角色解析後的相對量能；`CONFIRMED`/`WEAK`/`NEUTRAL`/`FAILED` |
| touch_count / reject_count / break_count | 觸碰/拒絕/突破次數（`touch_count` 是兩個方向加總；`reject_count`/`break_count` 是角色解析後方向的次數） |
| support_touch_count / resistance_touch_count | `touch_count` 依方向拆分（兩者相加等於 `touch_count`），讓「作為支撐」跟「作為壓力」各自的歷史樣本數可以被診斷；confidence 依角色只用其中一個方向計算，見 [sr-zone-scoring.md](./sr-zone-scoring.md)「六」 |
| overlap_group / confluence_count | 跨方法（ATR/volume_profile）重疊分群：不同方法都指向同一價位帶的 zone 有相同的 `overlap_group`；`confluence_count` 是群組內 zone 數（恆 >= 1，單獨的 zone 沒有群組，`overlap_group` 為 `NULL`）。不合併/刪除任何 zone，見 [sr-zone-scoring.md](./sr-zone-scoring.md)「十七」 |
| zone_momentum / zone_direction | 這個 zone 自己的歷史觸碰動能（逐 zone 不同，非股票層級量）；`UP`/`DOWN`/`FLAT` |
| recent_validation | `VALIDATED_RECENTLY` / `PENDING_VALIDATION` / `NOT_TESTED_RECENTLY` / `EXPIRED` |
| trading_score | 可拆解的綜合交易分數（0~100） = EV(34%) + RR(17%) + Trend(12.75%) + Volume(12.75%) + Confidence(8.5%) + Chip(15%)（【2026-07 籌碼分析整合】新增 chip 分量後原五個分量權重等比例縮小） |
| trading_score_breakdown | JSON：`trading_score` 六個分量各自的加權貢獻值，加總即為 `trading_score` |
| trading_recommendation | `STRONG_BUY`/`BUY`/`WATCH`/`NEUTRAL`/`AVOID`/`STRONG_SELL` |
| features | JSON：同一 zone 的 support/resistance typed 特徵快照 |
| evidence | JSON：兩方向 hold/break 的 SHAP baseline、最終機率、特徵值、貢獻與風險旗標。evidence 降級或此 zone 不在 `evidence_max_zones` 前 N 名時，`support`/`resistance` 為 `null`、僅保留 `risk_flags`（見 [sr-zone-scoring.md](./sr-zone-scoring.md)「十九」） |
| explanation | JSON：單一 zone 的白話解釋（`schema_version`/`role_summary`/`score_reason`/`probability_reason`/`confidence_reason`/`positive_factors`/`negative_factors`/`watch_conditions`），純展示層；`schema_version` 目前為 `sr_explain_v1`，舊資料為 JSON `null` |
| scenario | JSON：單一 zone 的結構化情境（`schema_version`/`state`/`title`/`summary`/`trigger_conditions`/`invalidation_conditions`），純展示層；`state` 為機器標記（如 `SUPPORT_RETEST`/`RESISTANCE_REJECTION`/`WAIT_FOR_DIRECTION`/`RETEST_REQUIRED`）；`schema_version` 目前為 `sr_scenario_v1`，舊資料為 JSON `null` |
| status / broken_at / broken_price | `PENDING`（尚未驗證或 `AT_ZONE` 方向未定）/ `HELD_SO_FAR`（曾被觸碰但未被突破）/ `BROKEN`（已被突破，`broken_at`/`broken_price` 是連續確認突破的第一根K棒）。由 `POST /sr-zones/:id/verify` 或 `daily_close` 排程更新，見 [sr-zone-scoring.md](./sr-zone-scoring.md)「十四」 |

**Index：** `INDEX(analysis_id)`、`INDEX(zone_uid)`；查詢時額外依 `tier` 排序（`CASE tier WHEN
'TIER_1_MAIN_STRUCTURE' THEN 1 ...`）後再依 `trading_score DESC`。

**`zone_uid`：分析快照 ↔ zone 身分的 join 路徑**（migration 069）。在此之前，「這次分析的
第 N 個 zone 是哪個身分」只活在 Go handler 的記憶體裡，DB 兩邊接不起來，下游只能靠價格
邊界回推。現在 zones 寫入 `stock_sr_zones` 時就帶著身分——身分比對排在 `repo.Create`
**之前**，見 [sr-zone-scoring.md](./sr-zone-scoring.md)「Zone 身分與 ZoneMatcher → 接線」。

* **可空且沒有外鍵，是刻意的。** `zone_instances` 的寫入（`Apply`）在 `stock_sr_zones`
  之後、且是另一個交易。加外鍵會讓「zones 已寫入、身分寫入失敗」這個既有的可容忍降級
  （handler 只記 warn、API 照樣回 201）直接違反約束，把降級升級成整筆分析失敗。
* **`NULL` 有三種語意，都不代表「這個 zone 沒有身分」**：① 該次分析早於 migration 069；
  ② 當次身分比對或寫入降級了（要查 log，API 回 201 看不出來）；③ 由
  `analysis/sr_analysis_provider.go` 這條不做身分追蹤的路徑建立（signal／排程／
  `reuse_existing=true`，與 `/sr-zones` 的新分析是兩條路）。
* **既有列不回填。** 回填要解的正是「兩個舊 `zone_key` 是不是同一個 zone」，而那正是
  身分層本身要建的能力——沒有它的當下，回填不可能做對。

---

## sr_identity_stats

身分關聯決策的**逐次分析**統計（migration 070）。一次分析一列。
查詢端點見 [`api-reference.md`](./api-reference.md) 的 `GET /sr-zones/identity-stats`；
為什麼是一張表而不是 prometheus，見 [`architecture.md`](./architecture.md)
「可觀測性：為什麼沒有 metrics 依賴」。

**為什麼要有這張表**：同一組數字已經有結構化 log（見
[sr-zone-scoring.md](./sr-zone-scoring.md)「可觀測性」），但 log 答不出趨勢問題。
**立表當時**（2026-08-21）唯一記錄排程行為的 `job_runs` 只保留當天
（`runPreMarket` 每天 08:50 `DeleteBefore(TodayTaipei())`）——實測前一晚排程的紀錄
早上就查不到了，所以**本表刻意不沿用那個保留策略**。

**`job_runs` 的保留期已於 2026-08-25 改成 30 天**（`jobRunRetentionDays`，
見 [`api-reference.md`](./api-reference.md) 的「`job_runs` 保留 30 天」），
上面那個「只保留當天」的前提已不成立。但本表仍有存在價值、也仍然不設保留期：
`job_runs` 記的是**每輪排程的成敗與標的數**，本表記的是**每次分析的身分關聯計數**，
兩者的粒度與問題域不同——`job_runs` 答不出「alias 命中率這個月往下掉了嗎」。

| 欄位 | 說明 |
|------|------|
| analysis_id | 對應的 `stock_sr_zone_analyses.id`。**沒有外鍵**，理由同 `stock_sr_zones.zone_uid`：統計寫入是 fail-open 的降級路徑，不該讓它反過來擋住主資料 |
| symbol / timeframe | 便於直接篩選，不必每次 join |
| matched_by_chain / matched_by_current / matched_by_alias | 三段關聯決策各自的命中數。優先序不可調換 |
| unmatched_keys / carried_noop / zone_ended_skipped / chain_conflicts / chain_key_ambiguous / alias_ambiguous / carried_parse_fail | 各類異常的**筆數**（明細留在 log，表只要計數） |
| invariant_violations | **必須恆為零**。與上面那些「分佈正不正常」的欄位語意不同，這一欄問的是不變式有沒有被違反，不要併進同一組比率 |
| zone_identity_degraded | 該次 zone 身分比對整段沒跑成。**此時其餘計數全為 0**——別讀成「這次很乾淨」 |
| event_identity_degraded | 事件層整段沒跑（多半是前者連帶） |
| zone_live_candidates | 該次進入 matcher 的既有身分數，供比率當分母參考 |
| zone_ended | 該次因 `SPLIT` / `MERGE` / `RESHAPE` 終止的身分數 |

**Index：** `INDEX(symbol, created_at DESC)`、`INDEX(analysis_id)`。

**母體是分析的子集**：統計只在 `reuse_existing=false` 那條路徑產生
（`analysis/sr_analysis_provider.go` 的重用路徑不追身分，也就沒有統計）。
算比率時分母要 join `stock_sr_zone_analyses`，不要拿本表列數當「所有分析」。

**只存原始計數，不存比率**：比率的分母隨查詢區間而變，先算好等於把一個決定寫死在資料裡。
聚合由 `GET /sr-zones/identity-stats` 負責，**走獨立的 SQL aggregate、不受該端點的
`limit` 影響**（`SRIdentityStatsRepo.Summarize`）——`limit` 只截斷明細。
明細與聚合共用同一個 WHERE builder，兩邊的過濾條件不會分岔。

量級：一次分析一列，所以**每交易日約「watchlist 檔數 × 2」列**（每日兩輪）。
以 2026-08-31 production 的 11 檔估算是每交易日約 22 列 ≈ 5.5k~5.7k 列/年，欄位幾乎都是整數。
**檔數會變，要重估就套上面那條公式**，不要沿用 22 這個數字；也**別拿它去乘 365**——
只有交易日會產出。

---

## stock_sr_decisions

每筆 SR Zone analysis 的 Decision Pipeline normalized snapshot。主欄位保存可查詢的
authority fields；detail JSON 欄位保存前端決策面仍需要、但不適合拆成大量細表的展示/解釋資料。
JSON 欄位在 PostgreSQL 為 `JSONB`；SQLite / MySQL 以文字 JSON 儲存。

| 欄位 | 說明 |
|------|------|
| analysis_id | FK → `stock_sr_zone_analyses.id`，每筆 analysis 一筆 decision |
| market_bias / entry_permission_state / position_action | Decision authority fields；P3 後分別對應 `semantic_pipeline.bias_state`、`entry_permission_state` 與 `action_state` 的對外摘要。`entry_permission_state` 自 `decision-semantic-pipeline-p5`（I-129）起已套上 execution gate 的封頂：RR 不合格是 `WAIT_CONFIRMATION`、⛔ 不是 `BLOCKED`（見 `sr-zone-scoring.md`「進場許可的 RR 判斷看 execution gate」） |
| price_path_state / model_health_state / event_market_state | Price path、AI health 與 event state 的查詢欄位 |
| reason_codes | JSON 陣列，彙整 final entry、price path、model governance 與 active bearish event reason codes |
| market_regime_json / data_quality_json / decision_derived_view_json | Decision market regime、資料品質與 derived view 權威語意 detail |
| event_sequence_json / daily_price_action_json | 事件序列與 daily price action detail |
| price_path_json / daily_confirmation_json | Price path 完整 detail 與日 K 確認狀態 |
| defense_lines_json | tactical / swing / strategic 防守線 |
| rr_context_json / rr_gate_json | Entry RR、position RR 來源與 RR gate 判斷（`rr_gate` 是 execution gate；I-129 起多 `setup_qualified`，是 setup gate 的判定，舊快照沒有這個鍵） |
| position_action_condition_json | 部位操作條件；`state` 由 `semantic_pipeline.action_state` 推導，防守價仍由 primary zone 計算 |
| market_context_json / confidence_explanation_json / risk_notes_json | 前端決策說明所需的 context、confidence factor 與風險提示 |
| zone_summaries_json | `nearest_decision_zone`、`nearest_support_zone`、`nearest_resistance_zone`、`primary_structural_zone`、`best_trade_zone`、`primary_zone`、`secondary_zones` |
| decision_summary | 原始 decision JSON snapshot，保留 debug 與舊相容用途 |

**Index：** `UNIQUE(analysis_id)`、`INDEX(symbol, timeframe, analyzed_at DESC)`。

---

## market_event_detections

每筆 SR Zone analysis 偵測到的市場事件逐筆 raw 紀錄（`detect_market_events` 的完整 event chain），
一筆 analysis 對應 0..N 列。保留完整偵測鏈供對外呈現與稽核；Decision gating 只消費
`market_event_states` 的 active 集合（見 [sr-zone-scoring.md](./sr-zone-scoring.md)「十八」）。

| 欄位 | 說明 |
|------|------|
| analysis_id | FK → `stock_sr_zone_analyses.id` |
| event_key | 事件唯一鍵（同一 zone 的事件鏈以此關聯） |
| event_type / event_family / event_scope | 事件類型、族群與範圍（例如 `HIGH_VOLUME_BREAKDOWN`） |
| zone_key | 事件對應的 zone 鍵 |
| direction | 事件方向（bullish / bearish 等） |
| state | 事件狀態 |
| active | 是否為 active 事件（`0`/`1`，預設 `0`） |
| confidence / price_level | 事件信心與價位；可為 `NULL` |
| reason_codes | JSON 陣列，事件 reason codes |
| event_json | 事件完整 detail JSON（PostgreSQL 為 `JSONB`；SQLite/MySQL 文字 JSON） |
| created_at | 建立時間 |

**Index：** `INDEX(analysis_id)`、`INDEX(symbol, timeframe, analyzed_at DESC)`。

---

## market_event_states

由 raw event chain 收斂出的每個事件狀態（`build_event_state_summary`），一筆 analysis 對應
0..N 列。同一 zone 的 `HIGH_VOLUME_BREAKDOWN → INTRADAY_RECLAIM → REVERSAL_CANDIDATE` 會收斂為
一列並標示 active/resolved，讓已被 reclaim/reversal 收復的 breakdown 不再作為 active bearish gate。

| 欄位 | 說明 |
|------|------|
| analysis_id | FK → `stock_sr_zone_analyses.id` |
| event_key | 事件狀態唯一鍵 |
| event_type / event_family / event_scope | 事件類型、族群與範圍 |
| zone_key | 事件對應的 zone 鍵 |
| root_event_type / latest_event_type | 事件鏈的起始與最新事件類型 |
| direction | 事件方向 |
| state | 事件狀態 |
| active | 是否仍為 active（`0`/`1`，預設 `0`）；resolved 後為 `0` |
| resolved_by | 解除該事件的事件類型；可為 `NULL` |
| confidence / price_level | 信心與價位；可為 `NULL` |
| reason_codes | JSON 陣列，狀態 reason codes |
| state_json | 事件狀態完整 detail JSON（PostgreSQL 為 `JSONB`；SQLite/MySQL 文字 JSON）。內含 `carried_from_previous` 與 `decision_visible` 兩個由 Python 單一產生、Go 只讀的旗標，見下方「event_instances / event_transitions / zone_key_aliases」 |
| created_at | 建立時間 |

**Index：** `INDEX(analysis_id)`、`INDEX(symbol, timeframe, active, analyzed_at DESC)`。

---

## stock_sr_daily_candidates

每筆 SR Zone analysis 的 `decision_summary.daily_candidate_zones` normalized projection——當現價
離既有 zone 過遠、或發生盤中收復/反轉事件時，用日 K OHLC 產生的短線支撐/壓力候選區，一筆 analysis
對應 0..N 列。

| 欄位 | 說明 |
|------|------|
| analysis_id | FK → `stock_sr_zone_analyses.id` |
| price_low / price_high | 候選區價格區間 |
| label / role | 顯示標籤與角色（`SUPPORT`/`RESISTANCE`） |
| source / lifecycle / decision_role | 候選來源、zone 健康度與決策角色。**`lifecycle` 欄存的是 zone 本身的健康度**（`CANDIDATE` / `VALIDATED` / `CONFIRMED` / `WEAKENING` / `BROKEN` / `INVALIDATED`），**不是事件演進**；API / `decision_summary` 層對應的鍵已更名為 `zone_health_state`，同物件的 `lifecycle` 鍵是 deprecated alias（值相同）。欄位名維持 `lifecycle` 不改。四套同名不同義的 lifecycle 對照見 [`sr-zone-scoring.md`](./sr-zone-scoring.md)「分層原則：lifecycle 不看 RR」 |
| distance_pct / distance_label | 與現價距離百分比與標籤；`distance_pct` 可為 `NULL` |
| reason | 候選成因說明文字 |
| event_refs | JSON 陣列，關聯事件參照 |
| candidate_json | 候選區完整 detail JSON（PostgreSQL 為 `JSONB`；SQLite/MySQL 文字 JSON） |
| created_at | 建立時間 |

**Index：** `INDEX(analysis_id)`、`INDEX(symbol, timeframe, analyzed_at DESC)`。

---

## sr_scoring_train_jobs

SR Zone Scoring 機率模型的訓練任務紀錄（見
[sr-zone-scoring.md](./sr-zone-scoring.md)「訓練任務可觀測化」）。訓練本身在
Go 背景 goroutine 呼叫 Python 同步執行，這張表讓 `POST /sr-zones/train`
可以立即回傳 `job_id`，前端輪詢 `GET /sr-zones/train-jobs/:job_id` 查詢進度，
不用只靠伺服器 log。這張表是 job history，不是 model registry；目前系統只維持
一個現行模型，訓練成功會覆蓋 active model path。舊的 `done` / `failed` 紀錄可由
`DELETE /sr-zones/train-jobs?keep=20` 清理，`pending` / `running` 不會被刪除。

| 欄位 | 說明 |
|------|------|
| job_id | 任務識別碼（`sr_train_<時間戳>` 格式），API 查詢用這個而不是 `id` |
| status | `pending`（已建立，尚未開始）/ `running`（訓練中）/ `done`（成功）/ `failed`（失敗） |
| symbols | JSON 陣列字串，這次訓練用的股票代號清單 |
| timeframe / fetch_limit / model_type | 訓練參數（K棒週期、每檔股票抓取根數、`gradient_boosting`/`hist_gradient_boosting`/`lightgbm`/`logistic_regression`） |
| row_count / sources | 訓練資料筆數、來源股票數；只有 `status=done` 才有值。**DB 欄位名是 `row_count`，JSON 欄位名仍是 `rows`**（`rows` 是 MySQL 保留字，migration 059 改名） |
| metrics | JSON：`{"hold": {...}, "break": {...}}`，兩個模型各自的 accuracy/precision/recall/auc/brier_score/log_loss/train_rows/test_rows/positive_rate_train/positive_rate_test/calibrated；只有 `status=done` 才有值。DB 欄位 `NOT NULL DEFAULT ''`（用 `store.RawJSON` 讀寫，不能是 SQL `NULL`，空字串在 API 回應會序列化成 `null`） |
| model_path / model_version | 訓練完成後寫入的模型檔路徑與版本；只有 `status=done` 才有值 |
| dataset_summary | JSON：`summarize_training_dataset()` 的診斷摘要（見 sr-zone-scoring.md「四」），只有 `status=done` 才有值。DB 欄位同樣 `NOT NULL DEFAULT ''` |
| error | 失敗原因；只有 `status=failed` 才有值 |
| started_at / finished_at | 開始/結束時間；`status=pending` 時兩者皆為 `NULL` |
| created_at | 任務建立時間（等同呼叫 `POST /sr-zones/train` 的時間） |

**Index：** `INDEX(created_at DESC)`。

---

## stock_sr_model_metrics

train job 完成時的 hold/break 模型品質 projection，一筆成功 train job 對應一列（`UNIQUE(job_id)`）。
與 `sr_scoring_train_jobs.metrics` 的差別：這張是拆欄可查詢的品質快照，供 model governance 與品質
追蹤使用，不是 job history 本身。

| 欄位 | 說明 |
|------|------|
| train_job_id | FK → `sr_scoring_train_jobs.id` |
| job_id | 對應 train job 的 `job_id`，唯一 |
| model_version / model_type / split_method / timeframe | 模型版本、類型、切分方式與 K 棒週期 |
| row_count / sources | 訓練資料筆數與來源股票數；可為 `NULL`。**DB 欄位名是 `row_count`，JSON 欄位名仍是 `rows`**（理由同上） |
| hold_auc / hold_brier_score / hold_log_loss / hold_calibrated / hold_test_rows | hold/bounce 方向品質指標；可為 `NULL` |
| break_auc / break_brier_score / break_log_loss / break_calibrated / break_test_rows | break 方向品質指標；可為 `NULL` |
| metrics_json / dataset_summary_json | 完整 metrics 與 dataset 摘要 JSON（PostgreSQL 為 `JSONB`；SQLite/MySQL 文字 JSON） |
| created_at | 建立時間 |

**Index：** `INDEX(model_version, created_at DESC)`。

---

## stock_sr_model_governance

每次 SR analysis 套用模型後的 AI health / confidence gate / model report projection，一筆 analysis
對應一列（`UNIQUE(analysis_id)`）。Decision 只消費此表的 health/gate 結果，不直接讀 raw model metrics。

| 欄位 | 說明 |
|------|------|
| analysis_id | FK → `stock_sr_zone_analyses.id` |
| model_version / model_config_hash | 套用的模型版本與設定 hash |
| health_state | 模型健康度（例如 `HEALTHY`/`DEGRADED`/`UNRELIABLE`） |
| average_edge_pp / directional_zone_count / zone_count | 平均 edge（百分點）、有方向 zone 數與總 zone 數；可為 `NULL` |
| allow_entry / max_entry_state | 是否允許依模型進場、最高可達的進場狀態 |
| quality_flags / warning_flags / blocking_flags | JSON 陣列，品質、警告與阻擋旗標 |
| confidence_gate_json | confidence gate 判斷 detail JSON |
| calibration_report_json / walk_forward_report_json / dataset_diagnostics_json | 校準、walk-forward 與 dataset 診斷報告 JSON |
| governance_json | model governance 完整 detail JSON |
| created_at | 建立時間 |

JSON 欄位在 PostgreSQL 為 `JSONB`；SQLite/MySQL 以文字 JSON 儲存。

**Index：** `INDEX(symbol, timeframe, analyzed_at DESC)`。

---

## stock_sr_regression_results

SR Zone regression fixture、walk-forward 與 calibration 回歸驗收結果。這張表保存跨
`model_config_hash` / `pipeline_version` 的驗收紀錄，用來追蹤模型或 pipeline 改動後是否仍通過
既定門檻；它不是 train job history，也不隨 `sr_scoring_train_jobs` pruning 刪除。
`metrics_json` 在 PostgreSQL 為 `JSONB`；SQLite / MySQL 以文字 JSON 儲存。

| 欄位 | 說明 |
|------|------|
| run_id | 回歸驗收 run 識別碼，唯一 |
| model_config_hash / pipeline_version | 本次驗收對應的模型設定與 pipeline 版本 |
| dataset_from / dataset_to | 驗收資料範圍；可為 `NULL` |
| split_method | 驗收切分方式，例如 `time` |
| hold_auc / hold_brier_score | hold/bounce 方向主要品質指標 |
| break_auc / break_brier_score | break 方向主要品質指標 |
| passed | 是否通過當次門檻；可為 `NULL` 表示尚未判定 |
| metrics_json | 完整驗收報告 JSON，保留門檻、fixture 名稱與其他指標 |
| created_at | 建立時間 |

**Index：** `INDEX(model_config_hash, created_at DESC)`、`INDEX(passed, created_at DESC)`。

---

## institutional_trades

三大法人買賣超 raw table，由 `chip.Syncer` upsert。

| 欄位 | 說明 |
|------|------|
| symbol / trade_date | 股票代號與交易日，組成唯一鍵 |
| foreign_net_buy | 外資買賣超股數 |
| investment_trust_net_buy | 投信買賣超股數 |
| dealer_net_buy | 自營商買賣超股數 |
| total_net_buy | 三大法人合計買賣超股數 |
| created_at / updated_at | 建立與更新時間 |

---

## margin_trades

融資融券 raw table，由 `chip.Syncer` upsert。

| 欄位 | 說明 |
|------|------|
| symbol / trade_date | 股票代號與交易日，組成唯一鍵 |
| margin_balance / margin_change | 融資餘額與增減 |
| short_balance / short_change | 融券餘額與增減 |
| margin_usage_rate / short_usage_rate | 資券使用率，可為 `NULL` |
| created_at / updated_at | 建立與更新時間 |

---

## broker_trades

券商分點買賣超 raw table，由 `chip.Syncer` upsert。FinMind 目前不支援券商分點時，
`broker_score` 會 fallback 為中性，不阻止其他籌碼分數計算。

| 欄位 | 說明 |
|------|------|
| symbol / trade_date / broker_name / branch_name | 唯一鍵 |
| buy_volume / sell_volume / net_buy | 分點買進、賣出與買賣超股數 |
| created_at | 建立時間 |

---

## chip_scores

每日籌碼分析結果快照，供 API、訊號、回測與 SR Zone v3 模型讀取。

| 欄位 | 說明 |
|------|------|
| symbol / trade_date | 股票代號與交易日，組成唯一鍵 |
| institutional_score | 法人分數（-100～100） |
| margin_score | 融資融券分數（-100～100） |
| broker_score | 券商分點分數（-100～100） |
| concentration_score | 集中度分數（0～100） |
| total_score | 籌碼總分（-100～100） |
| signal_type | `BULLISH` / `BEARISH` / `NEUTRAL` / `RISK`。**DB 欄位名是 `signal_type`，JSON 欄位名仍是 `signal`**（`signal` 是 MySQL 保留字，migration 059 改名） |
| reason | JSON：產生此分數的人類可讀原因 |
| created_at / updated_at | 建立與更新時間 |

---

## chip_sync_jobs

手動或 backfill 籌碼同步任務紀錄。日結同步不寫這張表，而是寫
`job_runs.job_name=chip_daily_sync`。

| 欄位 | 說明 |
|------|------|
| job_id | 任務識別碼（`chip_<時間戳到毫秒>_<4 位隨機碼>`；隨機碼是為了避免同毫秒的兩個請求撞上 UNIQUE） |
| mode | `manual` / `backfill` |
| symbols | JSON 陣列字串 |
| data_types | JSON 陣列字串；空陣列代表使用同步器預設資料類型 |
| from_date / to_date | 同步日期區間 |
| force_sync | API 接受並保存；目前 upsert 已冪等，尚未實作跳過既有資料的特殊行為。**DB 欄位名是 `force_sync`，JSON 欄位名仍是 `force`**（`force` 是 MySQL 保留字，migration 059 改名） |
| status | `pending` / `running` / `done` / `partial` / `failed` |
| symbols_total / symbols_done / symbols_failed | 任務進度 |
| failures | JSON：逐 symbol 失敗原因 |
| error | 任務層級錯誤摘要 |
| started_at / finished_at / created_at | 任務時間戳 |

**Index：** `INDEX(created_at DESC)`。

---

## market_backfill_jobs

股價（日K）手動回補任務紀錄，對應 `POST /api/v1/market/backfill`。結構刻意比照
`chip_sync_jobs`——同一個「歷史資料回補」頁面上兩塊 UI 走同一套輪詢流程；差別只在
回補範圍的表達方式：籌碼用 `from_date`/`to_date`，股價用 `days`（往前幾天）。

排程的每日盤前回補（`runPreMarket`）**不寫這張表**，它走的是既有的
`job_runs` 紀錄。這張表只記錄使用者手動觸發的回補。

| 欄位 | 說明 |
|------|------|
| job_id | 任務識別碼（`bf_<時間戳到毫秒>_<4 位隨機碼>` 格式） |
| symbols | JSON 陣列字串，要回補的股票代號。**API 層必填**，不會自動代入 watchlist |
| days | 往前回補幾天 |
| status | `pending` / `running` / `done` / `partial`（部分失敗） / `failed`（全部失敗，或背景執行 panic） |
| symbols_total / symbols_done / symbols_failed | 任務進度；每回補完一檔就更新一次，所以進度是逐檔推進的 |
| failures | JSON 物件陣列 `[{"symbol":…,"error":…}]`；`NOT NULL DEFAULT '[]'`，用 `store.RawJSON` 讀寫，不能是 SQL `NULL` |
| error | 任務層級錯誤摘要（`all symbols failed` / `some symbols failed` / `internal error`） |
| started_at / finished_at / created_at | 任務時間戳。`started_at` 在第一次進度更新時以 `COALESCE` 寫入 |

**Index：** `INDEX(created_at DESC)`、`job_id` UNIQUE。

**不做 job 續跑**：backend 重啟時進行中的任務不會被接手，會永遠停在 `running`。
前端的輪詢靠「進度停滯」偵測而非固定逾時來收斂（見
[`api-reference.md`](./api-reference.md) 的 market 章節）。

---

## evaluation_universe

評估標的池（migration 066，T-040 Step 5）。**與 `watchlists` 分離**：`watchlists` 驅動盤中
掃描、籌碼同步、日結掃描、signal 與 production SR 分析五＋一個流程，把 131 檔塞進去會讓
每一個都乘上約 12 倍。本表只驅動一件事——每日盤後更新這批標的的日 K，讓歷史持續累積供
T-002 / T-003 研究使用。**不參與任何交易決策或狀態推導。**

| 欄位 | 類型 | 說明 |
|------|------|------|
| symbol | VARCHAR(10) UNIQUE | 股票代號 |
| bucket_hint | VARCHAR(32) | 入池時的 `selection_bucket` |
| bucket_edge_low / bucket_edge_high | DECIMAL(18,10) | **入池時實際使用的分位數邊界**，等於當下的 `LOW/HIGH_VOLATILITY_THRESHOLD` |
| universe_version | VARCHAR(32) | 例如 `v2`；重新取分位數就升版 |
| universe_role | VARCHAR(16) | `primary` 參與股票 builder 決策／`supplemental` 僅交叉觀察。**沒有 CHECK 約束**，合法值由 `store.AllUniverseRoles()` 與欄寬回歸測試把住 |
| selected_at | TIMESTAMPTZ | 入池時間，由伺服器決定（不接受呼叫端指定） |
| source | VARCHAR(64) | 入池來源，例如 `T-040_STEP3` |
| active | BOOLEAN | 是否仍納入每日維護 |
| note | TEXT（mysql 為 VARCHAR(1024)） | 流動性門檻、`insufficient_depth` 等備註 |

**為什麼邊界存在每一列**（刻意的反正規化）：`bucket_hint` 單獨存在無法回答「這個 bucket
是用哪組邊界判的」。分位數是相對於當下母體的——實測 2026-08-17 有 3 檔（3530、3661、8102）
`atr_pct` 一個 bit 都沒變卻換桶，只因母體變了邊界移動。131 列的重複成本可忽略，
換來的是每一列自我描述。門檻的凍結機制見
[`sr-zone-scoring.md`](./sr-zone-scoring.md)「Volatility bucket 門檻」。

**`active=false` 不刪除列**：入池與退池的歷史本身是研究紀錄。重新匯入 selection report
（upsert）**不會動 `active`**——停用是獨立的人工決定。

**mysql 的 `note` 用 `VARCHAR(1024)` 而非 `TEXT`**：MySQL 的 TEXT 不能有 `DEFAULT`，
用 TEXT 會重演 `057` 那種「省略該欄位的 INSERT 在 mysql 失敗、另兩個 engine 成功」的
不對稱（見 [`issue.md`](./issue.md) I-054 第 3 項）。

---

## candle_verification_state

日 K 缺漏偵測的逐標的驗證簿記（migration 074，原記於 `issue.md` I-091）。
現況規格見 [`architecture.md`](./architecture.md)「日 K 缺漏偵測」。

**要解的問題**：`evaluation_universe_sync` 的 `success` 只代表「請求沒失敗」，
不代表「拿到了該有的資料」。2026-08-25 那輪 135 檔全成功，其中 `2867` 只回了 3 根
（視窗內有 7 個交易日），沒有任何東西報錯。本表存的是**偵測自己的進度**，
不是缺口本身。

| 欄位 | 類型 | 說明 |
|------|------|------|
| symbol / timeframe | VARCHAR(10) / VARCHAR(5)（sqlite 為 TEXT） | 複合主鍵 |
| last_attempted_at | TIMESTAMPTZ / DATETIME(0) / DATETIME，NULL | **公平排序鍵**。每次實際嘗試後更新，**不論成功或失敗** |
| last_verified_at | TIMESTAMPTZ / DATETIME(0) / DATETIME，NULL | 只在**驗證成功**時更新。「確認有缺口」也算成功——那是一次有結論的驗證 |
| last_result | VARCHAR(12)（sqlite 為 TEXT）NOT NULL | `verified` / `gap` / `deferred` / `unavailable`，有 CHECK 約束 |
| consecutive_failures | INTEGER NOT NULL DEFAULT 0 | **逐 symbol** 的連續失敗數 |

**時間欄位的三種型別依序為 PostgreSQL / MySQL / SQLite。**
MySQL 用 `DATETIME(0)` 而非 `TIMESTAMPTZ`——本 schema 一律存 UTC，時區由應用層負責。

**`last_result` 沒有 DEFAULT，寫入時必填**：給 `DEFAULT ''` 等於偷偷引入第五種狀態——
宣告只有四個值卻讓空字串合法。值域靠 `CHECK` 把住（mysql 8.0.16 起才強制執行，
更早的版本只解析不執行，所以寫入端本來就要保證）。

⚠️ **`deferred` 必須在值域裡**：規格要求 `deferred` 也更新 `last_attempted_at`，
而 `last_result` 是 NOT NULL ＋ CHECK——**首次就 deferred 的 symbol 沒有舊列可保留**，
漏掉它會讓「對照源還沒發布到那天」這種正常情況變成寫入失敗。

**`gap` 與 `unavailable` 是不同的軸，不是嚴重度階梯的兩格**：`gap` 是「驗成功了，
結論是有缺口」，`unavailable` 是「根本沒驗成」。把兩者混為一談會讓「驗不了」被記成
「驗過了沒問題」——那是這整個機制最該避免的失效。

**刻意不建額外索引**：查詢一律是 `WHERE timeframe = ? AND symbol IN (…)`，由主鍵支撐；
排序在 Go 端做（見下）。多一個沒人用的索引只是寫入成本。

### 為什麼是獨立的表，而不是加欄位到 `evaluation_universe`

1. 那張表的 `Upsert` 是「重新匯入 selection report」的**常態動作**，把驗證簿記混進去
   會讓研究決策與運維狀態互相干擾；
2. 候選來源日後可能不只池成員（`watchlists` 有同樣問題），綁在池表上會限制擴充；
3. 入退池是研究決策、驗證進度是運維狀態，兩者生命週期不同。

### ⛔ 排序不能下放到 SQL

公平排序的第一順位是「**沒有列者優先**」，而兩個常見的寫法都不成立：

* **沒有列 ≠ 欄位為 NULL**。`WHERE symbol IN (…)` 對首次出現的候選**根本不會回傳任何列**，
  它不會變成排在最前面的 NULL，而是**直接消失**。
* **`NULLS FIRST` 不是跨 driver 可用的語法**：MySQL 不支援，而且本 repo 至今從未用過它
  （見 `issue.md` I-054：mysql 的 CRUD 從未被驗證）。

所以 `LoadStates` **不帶 limit、也不排序**，只負責把已有的 state 查回來；
候選清單與這份 map 的 LEFT-merge 與排序一律在 Go 端做。
**repo 若先截斷，沒被回傳的既有 state 會被呼叫端誤認成「從未出現」而排到最前面**——
排序直接壞掉，而且壞得很安靜。

### `RecordAttempts` 的批次不得有重複鍵

同一個 symbol 可能同時落在多個 aggregate 分組（它在兩個不同的缺漏日期都缺 K 棒）。
PostgreSQL 的批次 `INSERT … ON CONFLICT DO UPDATE`
**不允許同一個 statement 更新同一列兩次**，那會直接報錯而讓**整批**寫入失敗。

去重是呼叫端的責任（它要做的本來就是「彙整成這個 symbol 的整體結論」），
repo 仍會擋下重複鍵並回傳帶 symbol 的可讀錯誤。

**`last_verified_at` 的 upsert 用 `COALESCE` 保護既有值**：本輪沒有任何成功驗證時傳 NULL
進來，不能覆寫掉先前的成功時間——那會讓「上次驗成功是什麼時候」在一次失敗之後永遠遺失。

---

## zone_instances / zone_role_incarnations / zone_transitions / zone_relations

Zone 的跨交易日身分與生命週期（T-048 階段 B，migration 067）。**目前只寫不讀**——
沒有任何決策路徑查詢它們，既有的 `market_event_states` / `market_event_detections`
繼續並行寫入。

**要解的問題**：舊的 zone 身分是 `event_engine._zone_key()`，也就是
`role:price_low:price_high`——綁在浮點邊界與角色上。2026-08-18 對 live 的盤點抓到兩個後果：

* **同一個支撐分裂成兩條事件鏈**：0050 從 2026-08-05 起，`SUPPORT:102.4916:103.1084`
  與 `SUPPORT:102.5414:103.1585` 每天並存，價格區間重疊 99%，各自帶一條 reclaim 鏈。
  下游重複計數且沒有任何東西會報錯。
* **角色翻轉必然斷鏈**：實測有 `IoU = 1.000` 的翻轉——邊界一動也沒動，
  只因為 role 從 RESISTANCE 變 SUPPORT，key 就完全不同。

### 三層結構

| 表 | 語意 | 終態 |
|---|---|---|
| `zone_instances` | **身分**。跨越失效與角色翻轉 | `SPLIT` / `MERGED` / `RESHAPED` |
| `zone_role_incarnations` | **一世**。同一價位失效後又有效＝下一世（`seq + 1`） | `INVALIDATED` / `EXPIRED` |
| `zone_transitions` | 狀態與角色轉換的 append-only 流水 | — |
| `zone_relations` | 分裂／合併的血緣邊 | — |

**`INVALIDATED` 是「這一世」的終態，不是身分的終態。** 這樣「這個價位長期是不是關鍵」
與「這一世活了多久」兩個問題都答得出來。

### 資格閘門：`observed_absences` 與交易日

`zone_instances.observed_absences` 是「連續幾次觀測到它不存在」。它與 `last_seen_at`
是**兩個獨立的軸**，一起決定某個身分還有沒有資格進入 matcher 的候選集合：

| 軸 | 欄位 | 量什麼 | 在哪裡擋 |
|---|---|---|---|
| 次數 | `observed_absences` | 「我們看了幾次都沒看到」 | `ListLive` 的 SQL（`< 3`） |
| 時間 | `last_seen_at` | wall-clock 陳舊度 | matcher（**用交易日算**） |

**為什麼時間軸不在 SQL 擋**：SQL 裡沒有交易日的概念，硬算會退回日曆天，
而週五→週一會被算成 3 天。`ListLive` 只用 `last_seen_at` 做寬鬆的下界過濾，
精確距離由 matcher 用注入的交易日曆算。

**為什麼兩個軸都要**：單一時間軸分不出「zone 消失了」與「我們根本沒看」——
實測 2330 全期只有 4 次分析、橫跨 5 週，任何兩次之間都隔很久。

**`as_of` 取的是 wall clock，不是資料日期。** `persistZoneIdentity`
（`backend/internal/api/handler/sr_zones.go`）用 `time.Now().In(timeutil.TaipeiTZ)`
當基準日，所以：

* `observed_absences` 量的是**分析次數**，不是時間。回補歷史、或同一天內對同一檔重跑
  多次分析，缺席次數都會以與市場無關的速度累加。
* 同一天內跑完的整串分析，`as_of` 全部相同，**交易日缺席距離恆為 0**——時間軸
  在單日內不可能自然觸發。2026-08-19 的階梯驗收只能用 fixture（直接改 `last_seen_at`）
  證明那條路徑會動。

判讀這兩個欄位前要先知道這件事，特別是拿它們回答「這個 zone 沉寂多久了」的時候。

**`EXPIRED` 與 `INVALIDATED` 的差別是誰造成的**：`INVALIDATED` 是市場事件
（被跌破／突破），`EXPIRED` 是長期缺席、我們不再認得它。收攤時同時寫
`expired_at` 與一筆 `end_reason='EXPIRED_BY_ABSENCE'` 的 transition。
`expired_at` 與 `ended_at` 分開存是刻意的：`ended_at` 回答「這一世何時結束」，
`expired_at` 回答「何時被判定為不再認得」，後者是資格閘門的稽核依據。

### 幾個容易寫錯的地方

* **`zone_uid` 是 opaque UUID**，不可把價格或 role 編進去——那正是舊 `_zone_key()`
  的問題成因。`price_low` / `price_high` 只是最近一次觀測值，**不是身分**。
* **`zone_role_incarnations.role` 只收 `SUPPORT` / `RESISTANCE`。** `AT_ZONE` 是
  「方向暫時無法解析」不是角色；live 有一條連續 16 次分析都是 `AT_ZONE` 的鏈，
  讓它開一世會產生沒有語意的紀錄。`AT_ZONE` 期間沿用這一世原本的角色。
* **`zone_relations` 沒有 `CONTINUE`。** 身分延續由 `zone_uid` 不變表達；寫成
  `parent = child` 的自環會讓沿 parent 遞迴回溯祖先的查詢無法終止
  （`WITH RECURSIVE` 沒有 cycle 偵測會直接失敗）。schema 有 CHECK 擋住。
* **`RESHAPE`（N→M）不猜血緣**：所有 parent 終止、所有 child 新生，
  只記錄實際匹配上的邊。誠實記錄一次無法解析的重整，好過編一組看起來合理的父子關係。
* **`from_state IS NULL` 的 `STATE_CHANGE` 恰好等於「身分誕生」**（`to_state='ACTIVE'`、
  `reason_codes=["IDENTITY_CREATED"]`）。失格與終態都從 `ACTIVE` 出發，不留白；
  純 role 轉換（`ROLE_*`）則是 `from_state` / `to_state` 都 NULL，靠 `transition_kind` 分辨。
  誕生的 `incarnation_uid` 在 `AT_ZONE` 誕生時是 NULL——那是因為 `AT_ZONE` 不開一世，
  不是漏帶。**誕生時間問這張表就好，不必再去查 `zone_instances.first_seen_at`**；
  這條不變式 2026-08-19 才補齊，在那之前誕生完全不寫 transition。
* **`zone_transitions.is_illegal`**：不合法的轉換照樣寫入，只標記不擋。
  判讀時記得過濾——這是刻意的取捨，目的是先看清楚現實會發生什麼。
* **`reason_codes` 是 `TEXT DEFAULT '[]'` 而不是 JSON 型別**：mysql 的 JSON 欄位
  給不起 DEFAULT，會造成三個 engine 不對稱（見下方「欄位命名規範」與 I-054 第 3 項）。

`transition_kind` 分四種，**role 的三種變化必須分開**——混為一談會讓真正的翻轉被雜訊
淹沒。實測 161 個匹配配對裡，`AT_ZONE` 的進出有 15 筆、真正的 `SUPPORT ↔ RESISTANCE`
翻轉只有 3 筆：

| `transition_kind` | 語意 |
|---|---|
| `STATE_CHANGE` | 一世或身分的狀態變化 |
| `ROLE_RESOLVED` | `AT_ZONE` → 有向：方向被解析出來 |
| `ROLE_UNRESOLVED` | 有向 → `AT_ZONE`：價格進入 zone，方向暫時無法解析 |
| `ROLE_FLIPPED` | `SUPPORT` ↔ `RESISTANCE`：真正的翻轉，結束當前這一世並開下一世 |

---

## event_instances / event_transitions / zone_key_aliases

事件鏈的跨交易日身分與生命週期，以及 zone 身分的 key 別名歷史
（T-048 階段 C，migration 068）。**目前只寫不讀**——與 067 那批一樣，
沒有任何決策路徑查詢它們，`market_event_states` / `market_event_detections`
繼續並行寫入，逐欄相同是驗收條件。

**要解的問題**：事件鏈原本的身分是 `market_event_states.zone_key`
（`role:price_low:price_high`），與 067 解掉的 zone 身分問題同源；而且事件鏈
**沒有被存下來**——`internal/analysis/event_timeline.go` 是回讀歷史快照、在讀取時
摺疊出 timeline，那份摺疊規則與 Python 的 `build_event_state_summary` 是兩份必須保持
對稱的實作，分歧時沒有任何東西會報錯。068 把生命週期變成存下來的事實。

### 三張表

| 表 | 語意 | 終態 |
|---|---|---|
| `event_instances` | **一條事件鏈**（一個 zone × 一個 family × 一個 `seq`） | `RESOLVED` / `EXPIRED` / `ZONE_IDENTITY_ENDED` |
| `event_transitions` | 鏈的狀態轉換 append-only 流水 | — |
| `zone_key_aliases` | zone 身分歷來用過的 `zone_key` | — |

### 幾個容易踩到的設計點

* **`zone_uid` 可為 NULL，但唯一鍵用 `zone_scope_key`**：`event_scope='SYMBOL'` 的事件
  （爆量這類）不屬於任何 zone。三個 engine 的 UNIQUE 都不擋多個 NULL，直接把
  可空的 `zone_uid` 放進唯一鍵等於不設限制，所以另存一個 NOT NULL 的投影鍵：
  ZONE scope 時等於 `zone_uid`，SYMBOL scope 時是字串 `'SYMBOL'`。
* **`seq` 表達同一個 (zone, family) 的先後鏈**：前一條 `RESOLVED` / `EXPIRED` 之後
  再出現同家族事件，那是新的一條鏈而不是舊鏈復活。規則與 Python 的
  `build_event_state_summary` 對稱，寫法比照 `zone_role_incarnations`。
* **`root_event_type` 不可被新偵測蓋掉**（T-045 踩過）：欄位名叫 root 卻永遠等於
  latest 的話，鏈的起點無法還原。
* **`active` 不等於 `state`**：還要通過該 family 的 `gating_states` 才算數。
* **`last_zone_key` 是鏈延續的第一把鑰匙**，見下一節。**每次觀測都要更新**，
  停在誕生那天的值會讓這把鑰匙永遠 miss。
* **`event_transitions.from_state IS NULL` 恰好等於「鏈誕生」**。這條不變式沿用 067
  的定案（zone 那邊曾因誕生不寫 transition ＋ 失格也留白，導致 `from_state IS NULL`
  什麼都問不出來），事件表不再踩第二次：所有非誕生的轉換都必須明寫 `from_state`。
* **`zone_key_aliases` 每個 `zone_uid` 只留最新 8 筆**，prune 在寫入的同一個交易內做。
  邊界每次由 ATR 重算，不設上限這張表會隨分析次數單調成長。

### 事件關聯到 zone 身分的三段決策

事件身上帶的 `zone_key` 是「上次那個 zone 長什麼樣」，本次分析的 `zone_key` 由這次的
ATR 邊界與 role 算出來——**兩者對不上是常態而非例外**（2026-08-19 實測 41 筆 ZONE scope
事件有 26 筆對不上）。所以關聯是三段的，優先序不可調換：

| 段 | 判準 | 為什麼在這個位置 |
|---|---|---|
| ① 既有鏈 | `(last_zone_key, event_family)` 命中活鏈 → 直接沿用 `instance.zone_uid` | 連 key 都不必解析。這是「鏈靜默凍結」的根治 |
| ② carried 護欄 | 沒有活鏈 ＋ `carried_from_previous=true` → **不建立新 occurrence** | carried 代表「重報」不代表「發生」。少了這條，每次分析都會讓終態的鏈重生一條新鏈 |
| ③ key 解析 | 本次分析的 `zone_key → zone_uid` map，miss 再退到 `zone_key_aliases` | 只服務**新**關聯 |

`carried_from_previous` 由 Python 在 `build_event_state_summary`（新偵測寫 `false`）與
`_normalize_previous_event_state`（carry forward 無條件寫 `true`）兩條路徑都寫進
`state_json`；Go 只讀不推導。**缺鍵是異常**，不是「不是 carried」。

`decision_visible`（2026-08-20 起）同樣寫在 `state_json`、同樣由 Python 單一產生
（`event_engine.EVENT_TYPE_META`）、Go 只讀不推導；**但缺鍵的處理刻意相反：缺鍵一律當
`true`**。既有四個事件型別都是決策可見的，而這個鍵出現之前寫下的列根本不會有它，
當成 `false` 會讓既有事件整批從決策桶消失。它標示這個事件能不能被決策看到——
`SUPPORT_RETEST_HELD` 與 `RESISTANCE_BREAKOUT` 是 `false`，只寫進
`market_event_states` / `market_event_detections` / `event_instances`，不進任何決策桶。
語意與消費端見 [`sr-zone-scoring.md`](./sr-zone-scoring.md)「事件的決策可見性」。

**`event_instances.decision_visible`（migration 071，2026-08-21 起）** 把這個旗標從
`state_json` 帶進身分層，讓 `GET /sr-zones/event-timeline` 能逐條回傳，顯示端才分得出
「事實紀錄」與「會影響決策的事件」。三個 engine 都是
`BOOLEAN NOT NULL DEFAULT TRUE`（sqlite 存 0/1）。要點：

* **值由 `buildEventIdentityWrite` 從 `state_json` 搬過來**（`eventDecisionVisible`，
  缺鍵回傳 `true`），Go **不依 `event_family` 推導**——那會變成第二份型別清單。
* **每次觀測跟著 upsert 更新**（三個 engine 的 UPDATE 子句都要有這一欄）。鏈由排程收尾、
  沒有本次 state 時不走這條路徑，所以已終結的鏈不會被寫回預設值。
* **migration 內有一次性回填**
  （`UPDATE ... SET decision_visible = FALSE WHERE event_family IN
  ('SUPPORT_RETEST','RESISTANCE_BREAKOUT')`）。階段 D 上線（2026-08-20）到本次
  migration 之間寫進去的鏈會落在預設 `TRUE`，而**已終結的鏈不會再被寫入、不會自動修正**。
  這行**是資料修正、不是執行期推導**——不要把它當成「Go 側可以照 family 判斷」的先例。
  不走「join `state_json` 回填」是因為三個 engine 的 JSON 取值語法各不相同，
  而回填母體小又已知。

**zone 身分因 `SPLIT` / `MERGE` / `RESHAPE` 終止時，parent 身上的事件不傳給 child**，
而是以 `end_reason='ZONE_IDENTITY_ENDED'` 收攤。那條鏈的前提（那個 zone 存在）已經消失，
接到 child 等於宣稱鏈延續了，而 `RESHAPE` 的定義正是「血緣無法解析」。血緣留在
`zone_relations`，之後要沿 parent 回溯補得回來；反過來先接了再拆拆不回來。

### 終態的不變式

`ended_at` 有值 ⇔ `state ∈ {RESOLVED, EXPIRED}` 且 `active = false`。三個欄位是**一起**
寫的（Go 側只有一個終態寫入點），曾經發生過只設 `ended_at` / `end_reason`、
`state` 與 `active` 沿用舊值的矛盾快照。查核 SQL 見
[`development-workflow.md`](./development-workflow.md)「as-of 階梯驗收的六條門檻」。

---

## 欄位命名規範：避開 MySQL 保留字

新增 migration 時，**欄位名不可使用 MySQL 保留字**（`trigger`、`signal`、`force`、
`rows`、`interval`、`range`、`rank`、`groups`、`system`、`condition`… 完整清單見
MySQL 官方文件）。

理由是這個專案同時維護 mysql / postgres / sqlite 三份 migration，但 **repo 的查詢語句
三個 engine 共用同一份字串**。保留字在 DDL 可以用反引號迴避，但反引號是 MySQL 專屬
語法，放進共用的查詢字串會讓 postgres / sqlite 直接語法錯誤——等於沒有可行的迴避方式。

已經踩過一次：`trigger`／`signal`／`force`／`rows` 四個欄位在 2026-08-07 由 migration 059
改名為 `trigger_source`／`signal_type`／`force_sync`／`row_count`。postgres 與 sqlite 都
容許這些字裸寫，所以問題潛伏了 57 個 migration 才在第一次真的跑 MySQL 時爆出來。

**改名時 JSON／API 欄位名維持原樣**（Go struct 的 `db` tag 與 `json` tag 刻意不同），
對外契約不受影響。`SELECT *` 直接當 API 回應的地方要記得手動轉回來（見
`python/http_server.py` 的 `_backtest_job_payload`）。

改到 `migrations/mysql/` 之後要跑 `scripts/test-mysql-migrations.sh` 實際驗證，
見 [`development-workflow.md`](./development-workflow.md)。

---

## position_transactions / positions

`position_transactions` 是不可變事件帳；支援 `OPENING_BALANCE`、`BUY`、`SELL`、
`ADJUSTMENT`。BUY/SELL 保存股數、價格、費用與稅；ADJUSTMENT 保存更正後股數、
AVG 成本及原因。API 不提供 update/delete。ADJUSTMENT 代表無現金流的帳務校正，
不改變 `realized_pnl`；有實際成交價與現金流的增減股必須使用 BUY/SELL。

`tenants` / `tenant_members` / `portfolio_groups` / `group_members` / `portfolios`
是 Position owner scope（migration 051 / 052 導入）。
`portfolio` 是真正持有 position 的帳本；`tenant` 是資料隔離邊界。Migration 051
建立初始 tenant / portfolio scope，並把既有全域持倉暫存到 `portfolio_id=1` 的 Legacy
Shared Portfolio；migration 053 改為每個 user 一個 `is_default` 的 Personal Portfolio，
移除 legacy shared default portfolio 與 position 相關表的 `portfolio_id DEFAULT 1`（API 必須
明確指定 `portfolio_id`）。**注意：053 刻意捨棄 `portfolio_id=1` 的舊全域持倉、不搬遷、不可逆**
——舊全域資料無使用者歸屬，三方言分別以 `DELETE` 或 rebuild only-copy（`portfolio_id<>1`）實作，
`-- +goose Down` 只還原空的 Legacy portfolio row，無法還原被刪的持倉列。Migration 052 新增
`portfolio_groups` 與 `group_members`；API 對外仍稱 groups，DB 表名避開 MySQL `GROUPS` 關鍵字風險。

`portfolios.owner_type` 支援 `TENANT` / `USER` / `GROUP`。`GROUP` portfolio 的
`owner_id` 指向 `portfolio_groups.id`；group `VIEWER` 可讀不可寫，`OWNER` / `ADMIN`
可寫入部位與分析快照。每個 `(owner_type, owner_id)` 至多一個 `is_default` portfolio，由
migration 054 的唯一約束保證：PG / SQLite 用 partial unique index（`WHERE is_default`），
MySQL 無 partial index 改用 functional key part `(IF(is_default, owner_id, NULL))`（unique 視多個
NULL 互異，需 MySQL 8.0.13+）。另注意 MySQL 不允許在 `INSERT INTO portfolios ... SELECT` 的子查詢
直接引用 `portfolios`（error 1093），涉及此表的 `NOT EXISTS` 子查詢需包一層 derived table 強制物化。

`tenant_members.role` 目前不參與授權判斷（`CanAccess` 只看 tenant membership 是否
存在），所有 tenant membership 一律預設 `MEMBER`（migration 051 搬入的既有 users 與
新註冊 user 皆為 `MEMBER`）；實際讀寫權限由 portfolio owner scope 與 `group_members.role`
決定。加入 group 成員要求其已是 group tenant 的成員，否則拒絕（不自動補 tenant membership）。

`positions` 是每個 `portfolio_id + symbol` 唯一的 AVG projection：

| 欄位 | 說明 |
|------|------|
| portfolio_id / symbol | 帳本 scope 與股票代號；`UNIQUE(portfolio_id, symbol)`，不可再把 symbol 視為全域唯一 |
| shares / avg_cost | 目前股數與移動加權平均成本 |
| realized_pnl | SELL 累積已實現損益 |
| version | optimistic version；事件 request 必須帶目前版本 |
| last_event_id / updated_at | projection 對應的最後事件與更新時間；`last_event_id` 以 FK 指向 immutable ledger |

## position_analyses

FLAT 與 LONG 共用的不可變分析快照。owner scope 導入後快照同樣保存 `portfolio_id`，表示
該次分析使用哪個 portfolio 的股數、AVG 成本與 version。包含 Position version、SR Zone reference、
Action、目前／目標／調整股數、調整金額、進場／停損／停利價、風險金額、
預期報酬、RR、損益、設定快照、Evidence、觸發與失效條件。

`sr_zone_analysis_id` 是 nullable 的 best-effort reference，沒有 DB FK 約束。刪除
被引用的 SR Zone 快照時，`position_analyses` 不會被刪除或阻擋；歷史分析仍保留
自身的決策快照欄位，但無法再透過該 id 回查完整 SR zones。

Migration 038 將同 symbol 的舊 holdings 依股數加權合併為一筆
`OPENING_BALANCE`，並把舊 `holding_analyses` 搬為
`rule_version=holding_sr_zone_v1_legacy` 後移除舊表。
