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
| K 棒同步 | `market.Fetcher` | 歷史日 K、盤中資料、回補流程 |
| 排程資料工作 | `backend/internal/scheduler` | pre-market、intraday、daily-close 中的資料更新階段 |
| K 棒資料表 | `candles` | 下游指標、訊號、SR Zone、驗證使用 |
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
- 使用者維護的 watchlist。
- 使用者新增的 BUY / SELL / ADJUSTMENT transaction。

## 輸出契約

- `candles`
- `institutional_trades`
- `margin_trades`
- `broker_trades`
- `watchlist`
- `positions`
- `position_transactions`
- `job_runs`
- `chip_sync_jobs`

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
