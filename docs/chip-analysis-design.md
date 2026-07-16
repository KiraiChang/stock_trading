# 籌碼分析設計與目前實作

本文件記錄籌碼分析的設計與目前實作狀態。籌碼分析用於補強既有技術分析與回測系統，協助判斷法人、融資融券、主力與券商分點等資金行為，不執行自動交易。

目前 Phase 1 已落地：Go backend 已有 `internal/chip`、chip repositories、
`handler.Chip`、`chip.Syncer`、`chip_sync_jobs`、傍晚獨立排程的
`chip_daily_sync`，前端頁面為 `frontend/src/routes/Chips.svelte`。Phase 2 項目仍追蹤在
[todo.md](./todo.md)。

在四條 pipeline 架構中，三大法人／融資融券／券商分點 raw data 同步與落地屬於
[Data Pipeline](./architecture/data-pipeline.md)；`chip_scores` 計算與籌碼摘要屬於
[Analysis Pipeline](./architecture/analysis-pipeline.md)。Decision Pipeline 只消費
已落地的籌碼分析結果，不應重新同步 raw data。

## 1. 目標與範圍

### 目標

- 建立台股籌碼資料的匯入、儲存、查詢與分析流程。
- 將籌碼狀態轉換成可被前端展示、訊號引擎引用、回測模組驗證的結構化資料。
- 與既有 K 線、技術指標、支撐壓力與回測結果整合，提供多維度交易輔助。

### 目前範圍

Phase 1 已支援：

- 三大法人買賣超：外資、投信、自營商。
- 融資融券：融資餘額、融券餘額、增減、使用率。
- 主力買賣超：依券商分點彙總估算。
- 分點券商買賣超排行。
- 籌碼集中度與連續買賣超天數。

Phase 2 再評估：

- 股權分散表。
- 董監持股。
- 借券、當沖比、現股當沖統計。
- 大戶、散戶持股比例。

## 2. 資料來源

優先沿用既有 market data 架構，新增 chip data fetcher。資料來源需分層處理：官方來源作為最終可信資料，API 服務作為開發效率與補資料來源，第三方或券商資料作為延伸來源。

### 建議來源優先順序

1. TWSE / TPEx 官方公開資訊：作為法人買賣超、融資融券、分點資料的主要來源。優點是權威且可驗證；缺點是格式可能變動、更新時間不完全一致，需要 parser 與重試機制。
2. FinMind：作為 Phase 1 首選開發來源，若既有 integration 已可取得資料，優先用於快速建立資料流程與測試資料。優點是 API 結構穩定；缺點是部分資料可能受方案、頻率限制或欄位涵蓋度影響。
3. Fugle：作為既有市場資料整合的補充來源，適合補足個股基本行情、成交量與可能的籌碼輔助欄位。若需付費或授權資料，必須在 config 中明確切換。
4. 券商或第三方分點資料：可用於主力買賣超與分點排行，但資料授權與穩定性需個別確認。不得把非官方資料設為唯一來源。
5. 本地快取與歷史資料庫：API 暫時失敗時，使用已落地的 raw chip tables 或匯入檔重算 `chip_scores`。

### 各資料類型建議來源

| 資料類型 | 優先來源 | 備援來源 | 說明 |
| --- | --- | --- | --- |
| 三大法人買賣超 | TWSE / TPEx | FinMind | 需拆分外資、投信、自營商，並保存合計值。 |
| 融資融券 | TWSE / TPEx | FinMind | 注意交易所、櫃買資料格式不同，統一成股數與日期。 |
| 券商分點 | TWSE / TPEx 或授權資料 | 第三方資料 | 分點名稱需正規化，避免同一分點多種名稱。 |
| 成交量與價格 | 既有 candles | Fugle / FinMind | 籌碼分數需要成交量比例與價格趨勢輔助判斷。 |
| 股權分散、董監持股 | 公開資訊觀測站 | 第三方資料 | Phase 2 再實作，更新頻率較低，不應阻塞 Phase 1。 |

### Fallback 與資料選擇策略

- 每個 provider 需標示 `source_name`、`fetched_at`、`source_trade_date`，方便稽核與重抓。
- 同一股票、同一日期、同一資料類型有多個來源時，依 `official > configured_api > third_party > cache` 選擇有效資料。
- 官方資料尚未更新時，不應用空資料覆蓋既有資料；需標記為 `pending` 或保留前次狀態。
- 若來源回傳單位為「張」，進入 store 前必須轉成「股」。前端顯示時再轉回「張」。
- 若缺少券商分點資料，`broker_score` 應 fallback 為 0 或 neutral，不應阻止法人與融資融券分數計算。

### 開發建議

目前 Phase 1 使用既有 FinMind client 實作籌碼同步。三大法人與融資融券可同步；
券商分點在 FinMind 不支援時會回傳 unsupported，`broker_score` fallback 為中性，不阻止
法人與融資融券分數計算。官方 TWSE / TPEx parser 與更多來源仍屬後續擴充。

資料來源需抽象成 interface，避免分析邏輯綁定單一 provider。

```go
type ChipDataSource interface {
    FetchInstitutionalTrades(symbol string, date time.Time) ([]InstitutionalTrade, error)
    FetchMarginTrades(symbol string, date time.Time) (*MarginTrade, error)
    FetchBrokerTrades(symbol string, date time.Time) ([]BrokerTrade, error)
}
```

## 3. 後端模組設計

目前 Go backend 已包含以下模組：

```text
backend/internal/chip/          # 籌碼分析核心邏輯
backend/internal/market/        # 資料來源 fetcher，可放 provider 實作
backend/internal/store/         # 籌碼資料 repository
backend/internal/api/handler/chip.go   # 籌碼 API handler
backend/internal/scheduler/            # chip_daily_sync（傍晚獨立 cron，見 chip.sync.cron）
```

核心職責：

- `market`: 負責外部資料取得與原始格式轉換。
- `store`: 負責資料庫讀寫，不包含分析規則。
- `chip`: 負責計算集中度、連續買賣超、法人趨勢與籌碼分數。
- `handler`: 提供 REST API 給前端與其他服務使用。
- `scheduler`: 收盤後或指定時間同步籌碼資料。

## 4. 資料模型

### institutional_trades

儲存三大法人買賣超。

```sql
CREATE TABLE institutional_trades (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    symbol VARCHAR(20) NOT NULL,
    trade_date DATE NOT NULL,
    foreign_net_buy BIGINT NOT NULL DEFAULT 0,
    investment_trust_net_buy BIGINT NOT NULL DEFAULT 0,
    dealer_net_buy BIGINT NOT NULL DEFAULT 0,
    total_net_buy BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_institutional_symbol_date (symbol, trade_date)
);
```

### margin_trades

儲存融資融券與資券變化。

```sql
CREATE TABLE margin_trades (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    symbol VARCHAR(20) NOT NULL,
    trade_date DATE NOT NULL,
    margin_balance BIGINT NOT NULL DEFAULT 0,
    margin_change BIGINT NOT NULL DEFAULT 0,
    short_balance BIGINT NOT NULL DEFAULT 0,
    short_change BIGINT NOT NULL DEFAULT 0,
    margin_usage_rate DECIMAL(10,4) NULL,
    short_usage_rate DECIMAL(10,4) NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_margin_symbol_date (symbol, trade_date)
);
```

### broker_trades

儲存券商分點買賣超。

```sql
CREATE TABLE broker_trades (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    symbol VARCHAR(20) NOT NULL,
    trade_date DATE NOT NULL,
    broker_name VARCHAR(100) NOT NULL,
    branch_name VARCHAR(100) NOT NULL,
    buy_volume BIGINT NOT NULL DEFAULT 0,
    sell_volume BIGINT NOT NULL DEFAULT 0,
    net_buy BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_broker_symbol_date_branch (symbol, trade_date, broker_name, branch_name)
);
```

### chip_scores

儲存每日籌碼分析結果，供 API、訊號與回測快速查詢。

```sql
CREATE TABLE chip_scores (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    symbol VARCHAR(20) NOT NULL,
    trade_date DATE NOT NULL,
    institutional_score DECIMAL(10,4) NOT NULL DEFAULT 0,
    margin_score DECIMAL(10,4) NOT NULL DEFAULT 0,
    broker_score DECIMAL(10,4) NOT NULL DEFAULT 0,
    concentration_score DECIMAL(10,4) NOT NULL DEFAULT 0,
    total_score DECIMAL(10,4) NOT NULL DEFAULT 0,
    signal VARCHAR(20) NOT NULL DEFAULT 'NEUTRAL',
    reason TEXT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_chip_score_symbol_date (symbol, trade_date)
);
```

遷移需同時支援既有 MySQL、PostgreSQL、SQLite migration 目錄。

## 5. 分析規則

### 法人趨勢

計算項目：

- 外資連續買賣超天數。
- 投信連續買賣超天數。
- 三大法人合計買賣超占成交量比例。
- 5 日、20 日累積買賣超。

建議分數：

```text
institutional_score =
  foreign_trend_score * 0.4 +
  investment_trust_trend_score * 0.35 +
  dealer_trend_score * 0.1 +
  total_net_buy_ratio_score * 0.15
```

### 融資融券

常見解讀：

- 融資增加且價格下跌：偏弱。
- 融資減少且價格上漲：籌碼沉澱，偏強。
- 融券增加且價格突破：可能軋空，偏強。
- 融資使用率過高：風險升高。

### 主力與分點

計算項目：

- Top N 分點買超合計。
- Top N 分點賣超合計。
- 主力買超占成交量比例。
- 買方集中度與賣方集中度。
- 主力連續買超天數。

集中度範例：

```text
concentration = abs(top_10_net_buy) / daily_volume
```

### 籌碼總分

```text
total_score =
  institutional_score * 0.35 +
  margin_score * 0.20 +
  broker_score * 0.30 +
  concentration_score * 0.15
```

訊號分類：

- `BULLISH`: 籌碼偏多。
- `BEARISH`: 籌碼偏空。
- `NEUTRAL`: 無明顯方向。
- `RISK`: 籌碼過熱或融資風險升高。

## 6. API 設計與目前端點

### 查詢單一股票籌碼摘要

```http
GET /api/v1/chips/:symbol/summary?date=2026-07-03
```

回應：

```json
{
  "symbol": "2330",
  "date": "2026-07-03",
  "signal": "BULLISH",
  "totalScore": 72.5,
  "institutional": {
    "foreignNetBuy": 12000,
    "investmentTrustNetBuy": 3000,
    "dealerNetBuy": -500,
    "consecutiveDays": 4
  },
  "margin": {
    "marginBalance": 23000,
    "marginChange": -1200,
    "shortBalance": 4200,
    "shortChange": 800
  },
  "broker": {
    "topNetBuy": 9000,
    "concentration": 0.18
  },
  "reason": [
    "外資連續買超 4 日",
    "融資下降且價格走強",
    "主力買超集中度提高"
  ]
}
```

### 查詢歷史籌碼分數

```http
GET /api/v1/chips/:symbol/scores?from=2026-01-01&to=2026-07-03
```

### 查詢券商分點排行

```http
GET /api/v1/chips/:symbol/brokers?date=2026-07-03&limit=20
```

### 手動同步籌碼資料

```http
POST /api/v1/chips/sync
```

Payload：

```json
{
  "mode": "manual",
  "symbols": ["2330", "2317"],
  "from": "2026-07-01",
  "to": "2026-07-03",
  "dataTypes": ["institutional", "margin", "broker", "scores"],
  "force": false
}
```

這支端點會立即建立 `chip_sync_jobs` 紀錄並背景執行，回傳 `{ "job": ... }`。
用 `GET /api/v1/chips/sync/:job_id` 查詢 manual/backfill 同步進度。`mode` 省略時為
`manual`；`mode=backfill` 且未指定 `from` 時使用 `chip.sync.history_trading_days`
往回推。`force` 目前只保存，upsert 本身已具冪等性。

## 7. 前端設計

目前前端頁面與元件：

```text
frontend/src/routes/Chips.svelte
frontend/src/lib/api/chips.ts
frontend/src/components/chip/ChipScorePanel.svelte
frontend/src/components/chip/InstitutionalTrend.svelte
frontend/src/components/chip/MarginTrend.svelte
frontend/src/components/chip/BrokerRankingTable.svelte
```

頁面區塊：

- 籌碼總分與訊號狀態。
- 三大法人買賣超趨勢圖。
- 融資融券變化圖。
- 主力集中度與分點排行。
- 籌碼理由列表。
- 與 K 線日期同步的籌碼明細。

UI 原則：

- 分數用清楚的 badge 或 progress bar，但避免過度裝飾。
- 買超使用紅色系、賣超使用綠色系，符合台股習慣。
- 表格需支援排序：買超、賣超、集中度、分點名稱。
- 日期切換需與既有股票選擇與市場資料頁面一致。

## 8. 排程與資料同步

籌碼資料同步已由既有 scheduler 統一協調，但不與日 K 或技術指標 job 綁成同一個不可分割流程。日 K 與技術指標通常可在收盤後較快完成，籌碼資料可能延後發布；籌碼 job 失敗時不會回滾 K 線或技術指標結果。

### 觸發模式

支援三種觸發模式：

- `daily`：交易日日結同步。**與 15:00 收盤掃描（`daily_close`）解耦，改由傍晚獨立 cron 觸發**（見下方「排程時間與資料延遲」）。
- `backfill`：歷史資料回補。不可混在日結 job 中執行，應由背景 job 依日期區間與 symbol 分批回補。
- `manual`：手動驅動。透過 API 或 CLI 補特定股票、日期區間或資料類型，適合修正 provider 資料或重算 `chip_scores`。

### 排程時間與資料延遲（現況）

FinMind 的法人資料在收盤（13:30）後傍晚才發布、融資融券更要晚間才由 TWSE 更新，比日 K 晚。若沿用 15:00 日 K 那批的時間點採集當日籌碼，多半會抓到空陣列，而空資料被視為成功、只能靠隔天 lookback 區間回補「昨天」，導致資料庫永遠落後一天。因此現況為：

- **獨立傍晚排程**：日結籌碼採集（`runChipDailySync`）由 `Scheduler.Start()` 用獨立 cron 觸發，時間由設定 `chip.sync.cron` 控制，**預設 `0 21 * * 1-5`**（台北時區）。需要自動重試時可設多時段，例如 `0 18,20,22 * * 1-5`；upsert 天生冪等，重跑安全。
- **空資料不再靜默成功**：當目標日確認是交易日（該日已有日 K）卻抓不到當日法人／融資融券時，`Syncer.SyncRange` 會回報 `ErrChipDataNotPublished`，讓 `chip_daily_sync`（`job_runs`）或 `chip_sync_jobs` 記為失敗、留下可見訊號，而非誤判成功。非交易日（該日無日 K，如週末／國定假日）不會被誤判為失敗。
- **`chip_scores` 自我修復回算**：`daily` 單日模式除了計算目標日，另會向前回算最近數個交易日（`scoreRecomputeWindow`，預設 5 個日曆天）。當某天的原始籌碼在自己排程當下尚未發布、隔天才被 lookback 補進 DB 時，回算窗口會替那天重算分數，消除分數落後一天。`computeAndStoreScore` 具冪等性且會跳過無原始資料的非交易日。

日結建議順序（K 線／指標／SR 驗證在 15:00 `daily_close`，籌碼採集在傍晚獨立排程）：

```text
[15:00 daily_close]
Sync daily candles / volume
  ↓
Calculate technical indicators and evaluate signals
  ↓
Run SR zone verification

[傍晚 chip.sync.cron，預設 21:00]
Sync raw chip data (含 lookback 區間回補)
  ↓
Calculate / 回算 chip_scores（目標日 + 近 scoreRecomputeWindow 天）
  ↓
Refresh signal strength / UI cache
```

日結籌碼 job 的失敗狀態獨立記錄在 `job_runs`（`job_name=chip_daily_sync`）。manual/backfill
同步則記錄在 `chip_sync_jobs`。K 線與訊號已完成時，籌碼同步失敗（含 `ErrChipDataNotPublished`）
只標記籌碼 job failed，後續可用 `POST /api/v1/chips/sync` 手動重跑或等下一個排程時段補上。

手動同步參數建議：

```json
{
  "mode": "manual",
  "symbols": ["2330", "2317"],
  "from": "2026-07-01",
  "to": "2026-07-03",
  "dataTypes": ["institutional", "margin", "broker", "scores"],
  "force": false
}
```

Job 流程：

```text
Load symbols
  ↓
Fetch institutional trades
  ↓
Fetch margin trades
  ↓
Fetch broker trades
  ↓
Upsert raw chip tables
  ↓
Calculate chip score
  ↓
Store chip_scores
  ↓
Notify frontend through existing API/WebSocket if needed
```

同步必須可重跑，資料寫入使用 upsert，避免重複資料。

### 歷史回補與批次寫入

歷史籌碼資料建議採分階段回補，避免第一次同步時間過長或資料來源被限流。

建議回補區間：

- 最小可用：`250` 個交易日，約 1 年，足以支援 20 日、60 日累積買賣超與基本籌碼趨勢。
- 建議預設：`500` 個交易日，約 2 年，較適合觀察法人長週期、融資水位與主力分點變化。
- 回測或模型訓練：`1000` 個交易日以上，約 4 年，應以背景 job 慢慢補，不阻塞日常同步。

DB 寫入必須分批執行，不建議一次大量 insert 或 upsert。初期沿用保守批次大小：

```yaml
chip:
  sync:
    history_trading_days: 500
    batch_size: 50
```

`batch_size: 50` 適合作為安全預設，尤其在同時寫入法人、融資融券、券商分點與 `chip_scores` 時較容易控制失敗範圍。若實測 MySQL / PostgreSQL / SQLite 的 upsert 效能穩定，可再調整為 `200` 或 `500`。

批次寫入規則：

- 每批以同一資料類型為單位，例如 institutional、margin、broker 分開批次。
- 每批失敗需記錄 symbol、date range、provider、錯誤原因。
- job 可從失敗批次重跑，不應整段回補重來。
- upsert key 必須使用 symbol + trade_date；券商分點需再加 broker / branch。
- 寫入 raw chip tables 成功後，再分批計算並 upsert `chip_scores`。

## 9. 與既有訊號和回測整合

籌碼分數不直接產生交易指令，只作為訊號引擎加權因子。

整合方式：

- 技術突破成立，但籌碼 `BEARISH`：降低訊號強度。
- 支撐附近反彈，籌碼 `BULLISH`：提高觀察優先度。
- 融資過高且價格跌破支撐：標記 `RISK`。
- 回測時允許策略使用 `chip_scores.total_score` 作為 filter。

Python 回測可透過 API 或資料庫查詢籌碼分數，避免重寫 Go 分析規則。若 Python 需要離線回測，另建匯出檔或 read-only repository。

## 10. 錯誤處理與資料品質

需處理：

- 交易日無資料。
- 個股停牌或下市。
- 資料來源延遲更新。
- 分點名稱不一致。
- 法人資料單位不同，例如股數、張數。

所有外部資料進入 store 前需轉成一致單位。建議內部統一使用「股」作為 volume 單位，前端再顯示為「張」。

## 11. 測試建議

Go 測試：

- `backend/internal/chip`: 分數計算、連續買賣超、集中度。
- `backend/internal/store`: upsert、日期區間查詢、缺資料處理。
- `backend/internal/api/handler`: API response shape 與錯誤碼。

Python 測試：

- 回測策略引用籌碼分數時的 filter 行為。
- 缺少籌碼資料時需 fallback 為 neutral，不應中斷回測。

前端測試或手動驗證：

- 籌碼頁面可切換股票與日期。
- 表格排序正確。
- 無資料時顯示空狀態，不顯示錯誤堆疊。

## 12. 開發順序建議

1. 新增資料表 migration 與 repository。
2. 實作籌碼資料 source interface 與一個 provider。
3. 實作 chip score 計算邏輯與單元測試。
4. 建立 API handler。
5. 建立排程同步 job。（已完成，`chip_daily_sync` 由傍晚獨立 cron 觸發，見 §8「排程時間與資料延遲」）
6. 新增前端籌碼分析頁面與 API client。（已完成，`Chips.svelte` / `lib/api/chips.ts`）
7. 將籌碼分數接入訊號引擎與回測 filter。（已完成，訊號強度與模組化回測 filter）
8. 補齊文件與開發指南。（持續維護）

## 13. 非目標

- 不做自動下單。
- 不把籌碼分數視為單一買賣依據。
- 不在前端硬編碼分析規則。
- 不讓 Python 與 Go 各自維護不同版本的籌碼計分邏輯。
