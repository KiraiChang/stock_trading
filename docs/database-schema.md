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
| rsi14 | DECIMAL(6,4) | RSI（0～100） |
| macd / macd_signal / macd_hist | DECIMAL(10,4) | MACD 三線 |
| bb_upper / middle / lower | DECIMAL(10,4) | 布林通道 |
| atr14 | DECIMAL(10,4) | 平均真實波幅 |
| vwap | DECIMAL(10,4) | 成交量加權均價 |
| vol_ma20 | BIGINT | 20 日均量 |
| vol_ratio | DECIMAL(6,4) | 量比（當日量 / MA20） |

---

## signals

訊號記錄，永久保存供回測分析。

| 欄位 | 說明 |
|------|------|
| signal_type | `BREAKOUT`, `BREAKDOWN`, `VOLUME_SPIKE` |
| direction | `BUY`, `SELL`, `WATCH` |
| vol_ratio | 觸發時的量比 |
| resistance / support | 觸發的阻力 / 支撐價位 |
| trend | 當時趨勢狀態（`BULLISH`, `BEARISH`, `SIDEWAYS`） |
| strength | 訊號強度，預設 1.0；有籌碼資料時會依 `chip_scores.signal` 上修或下修 |
| chip_signal | 評估當下使用的籌碼訊號；空值代表查無籌碼資料 |

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
來源截斷）時整體略過本次同步，避免誤將大量個股標記下市。抓取失敗時不自動重試，改由人工
觸發手動同步；timeout、來源間隔與這個決策的理由見
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
| last_seen_at | 最近一次在來源清單看到的時間 |
| created_at / updated_at | 建立與更新時間 |

---

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
| status | `pending` → `running` → `done` / `failed` |
| trigger_source | `manual`（API 觸發）。**DB 欄位名是 `trigger_source`，JSON／API 欄位名仍是 `trigger`**——`trigger` 是 MySQL 保留字，migration 059 改名（見 issue.md I-054） |
| use_chip_filter | 是否在模組化回測中用 `chip_scores.total_score` 過濾進場 |
| chip_min_score | 籌碼 filter 門檻（-100～100）；缺資料視為 0 |
| started_at / finished_at | 執行時間戳 |

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

---

## backtest_trades

每筆回測交易明細。

| 欄位 | 說明 |
|------|------|
| job_id | FK → backtest_jobs.job_id |
| symbol | 股票代號 |
| direction | `BUY` / `SELL` |
| entry_time / exit_time | 進出場時間 |
| entry_price / exit_price | 進出場價格 |
| size | 交易股數 |
| pnl / pnl_pct | 損益（絕對值 / 百分比） |
| commission | 手續費 |

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

**Index：** `INDEX(analysis_id)`；查詢時額外依 `tier` 排序（`CASE tier WHEN
'TIER_1_MAIN_STRUCTURE' THEN 1 ...`）後再依 `trading_score DESC`。

---

## stock_sr_decisions

每筆 SR Zone analysis 的 Decision Pipeline normalized snapshot。主欄位保存可查詢的
authority fields；detail JSON 欄位保存前端決策面仍需要、但不適合拆成大量細表的展示/解釋資料。
JSON 欄位在 PostgreSQL 為 `JSONB`；SQLite / MySQL 以文字 JSON 儲存。

| 欄位 | 說明 |
|------|------|
| analysis_id | FK → `stock_sr_zone_analyses.id`，每筆 analysis 一筆 decision |
| market_bias / entry_permission_state / position_action | Decision authority fields；P3 後分別對應 `semantic_pipeline.bias_state`、`entry_permission_state` 與 `action_state` 的對外摘要 |
| price_path_state / model_health_state / event_market_state | Price path、AI health 與 event state 的查詢欄位 |
| reason_codes | JSON 陣列，彙整 final entry、price path、model governance 與 active bearish event reason codes |
| market_regime_json / data_quality_json / decision_derived_view_json | Decision market regime、資料品質與 derived view 權威語意 detail |
| event_sequence_json / daily_price_action_json | 事件序列與 daily price action detail |
| price_path_json / daily_confirmation_json | Price path 完整 detail 與日 K 確認狀態 |
| defense_lines_json | tactical / swing / strategic 防守線 |
| rr_context_json / rr_gate_json | Entry RR、position RR 來源與 RR gate 判斷 |
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
| state_json | 事件狀態完整 detail JSON（PostgreSQL 為 `JSONB`；SQLite/MySQL 文字 JSON） |
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
| source / lifecycle / decision_role | 候選來源、生命週期與決策角色 |
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
