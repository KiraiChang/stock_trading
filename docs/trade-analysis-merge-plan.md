# SR Zone 與 Position Analysis 合併計畫書

> 暫存文件：本文件用來追蹤「SR Zone + Position Analysis 統一成交易分析入口」的功能實作計畫。等全部功能完成、必要設計內容搬入正式文件後，刪除此文件，並移除 `docs/todo.md` 對應項目。

## 目標

把目前分散的「沒有持股時的 SR Zone 分析」與「有持股時的 Position Analysis」整合成同一個使用者入口：使用者直接輸入股票代碼，系統依資料庫是否存在持股資料決定分析情境；若沒有持股資料，視為空手，仍提供可操作的交易分析建議。

內部架構不把兩個核心模型硬合併：

- SR Zone 保持市場結構分析層，負責 Data → Features → Score → Evidence。
- Position Analysis 保持決策層，負責把持股狀態、成本、停損、目標價、風險報酬比與 SR Zone 結果整合成 Decision。
- 對外提供統一的「交易分析 / 股票決策」功能與 API facade。

## 現況摘要

目前已有可合併的基礎：

- SR Zone 已有獨立分析流程與 API，能產生支撐壓力區、分數、信心、摘要與 evidence。
- Position Analyzer 已會讀取持股、建立或重用 SR Zone，並輸出 Position Analysis。
- Position Analysis 的核心資料模型已保留未來調整空間，例如目標價計算、停損、風險報酬比、ADJUSTMENT 類型等。
- 使用者已確認初期只處理單一持股，等系統穩定後再納入整體投資組合。

主要缺口：

- 前端入口仍偏向 SR Zone 與 Position Analysis 分離。
- API 語意尚未形成「輸入股票代碼 → 自動判斷持股狀態 → 回傳統一交易分析」。
- SR Zone 建立 / 重用邏輯需要抽出共用層，避免不同 handler 重複實作。
- SR Zone 刪除、Position Analysis 歷史參照、資料完整性邊界仍需整理。

## 建議架構

```text
Stock Code
  │
  ▼
Unified Trade Analysis API
  │
  ├─ Load Position
  │    ├─ 有持股：使用現有 position / avg cost / quantity
  │    └─ 無持股：建立空手分析 context
  │
  ├─ SR Analysis Provider
  │    ├─ 重用近期有效 SR Zone
  │    └─ 必要時重新產生 SR Zone
  │
  ├─ Position / Watch Decision Engine
  │    ├─ 有持股：續抱、調整、停損、停利、減碼等建議
  │    └─ 空手：觀察、等待、突破、回測支撐等建議
  │
  ▼
Unified Trade Analysis Result
```

## 分層邊界

### Data

資料來源包含：

- 股票代碼與基本行情。
- 持股資料；若資料庫沒有持股資料，建立空手 context。
- 歷史 K 線與 SR Zone 計算所需資料。
- 既有 SR Zone 分析結果與 Position Analysis 歷史。

### Features

特徵層保留兩類輸入：

- SR Zone features：支撐壓力距離、區間寬度、confluence、volume profile、ATR、bounce / break 相關特徵。
- Position features：AVG 成本、未實現損益、距離停損、距離目標價、持股數量、風險報酬比。

### Score

SR Zone score 維持目前模型 / 規則輸出；Position score 則建立在：

- SR Zone 的市場結構分數。
- 持股或空手情境。
- 成本基準使用 AVG。
- 初期參數固定，但保留後續調整空間。

### Evidence

Evidence 應能說明決策來源：

- 哪些支撐 / 壓力區影響判斷。
- 哪些持股數據影響判斷。
- 若沒有持股，明確標示這是空手分析，而不是資料缺漏錯誤。

### Decision

Decision 是對外最終輸出，初期支援：

- 有持股：HOLD、REDUCE、EXIT、ADJUSTMENT 等。
- 空手：WATCH、WAIT、BREAKOUT_ENTRY、PULLBACK_ENTRY 等。
- `ADJUSTMENT` 不直接刪除，保留作為調整型決策。

## 實作階段

### Phase 1：統一前端入口

目標：使用者直接輸入股票代碼，照系統建議走。

任務：

- 建立或調整單一「交易分析」入口。
- 前端不要求使用者先選「有持股 / 沒持股」。
- 顯示目前分析情境：有持股或空手。
- 保留 SR Zone 詳細資料展開區，避免喪失市場結構資訊。

驗收：

- 輸入有持股股票代碼時，看到持股導向決策。
- 輸入無持股股票代碼時，看到空手導向建議。
- 使用者仍能看到主要支撐 / 壓力與 evidence。

### Phase 2：統一 API facade

目標：提供單一 API 給前端使用。

建議 endpoint：

- `POST /api/trade-analysis/analyze`
- `GET /api/trade-analysis/:stock_code/latest`
- `GET /api/trade-analysis/:stock_code/history`

任務：

- 後端依股票代碼讀取 position。
- 找不到 position 時建立空手 context，不回傳錯誤。
- 回應格式包含 `context`、`decision`、`sr_zone_analysis`、`position_analysis` 或等價欄位。
- 保留既有 SR Zone / Position API 的相容性，避免一次性破壞既有頁面。

驗收：

- 同一 endpoint 可處理有持股與無持股。
- 無持股不產生 404；語意上是空手分析。
- 既有 SR Zone / Position API 測試仍通過。

### Phase 3：抽出 SRAnalysisProvider

目標：避免各 handler / analyzer 重複處理 SR Zone 建立、重用與錯誤語意。

狀態：進行中。已先抽出後端共用 provider，讓 Trade Analysis / Position
Analysis 使用同一套 SR Zone 重用與重算策略；`/sr-zones` endpoint 保留預設
「直接建立新分析」語意，但新增 `reuse_existing=true` 選項，讓 SR Zone 專頁
可明確選擇重用近期快照。

任務：

- 將「讀取近期 SR Zone → 不足時重算 → 回傳 analysis + zones」封裝為共用服務。
- Position Analyzer 與 Trade Analysis API 改用同一 provider。
- 定義 SR Zone 分析失敗時的錯誤分類：資料不足、模型不可用、系統錯誤。

驗收：

- 重用 / 重算規則只有一個主要實作。
- SR Zone 端點與交易分析端點得到一致的 SR 結果。
- 錯誤訊息能區分可恢復的資料不足與真正系統錯誤。

### Phase 4：資料完整性與歷史參照

目標：整理 SR Zone 與 Position Analysis 的關聯邊界。

任務：

- 確認刪除 SR Zone 時是否會影響 Position Analysis 歷史。
- 若歷史只需要 snapshot，確保 Position Analysis 儲存必要摘要。
- 若歷史需要可回查原始 SR Zone，補上 reference policy。
- 明確定義分析紀錄的保留策略。

驗收：

- 刪除或重建 SR Zone 不會讓既有交易分析歷史變成不可讀。
- 文件描述清楚 snapshot 與 reference 的責任邊界。

### Phase 5：相容性清理與正式文件更新

目標：功能穩定後收斂舊入口與暫存文件。

任務：

- 評估舊 SR Zone / Position Analysis 頁面是否保留為進階頁或導向新入口。
- 更新 `docs/architecture.md`、`docs/stock-analysis.md`、`docs/sr-zone-scoring.md` 等正式文件。
- 移除完成後不再需要的 `docs/todo.md` 追蹤項。
- 刪除此計畫書。

驗收：

- 正式文件反映目前架構。
- `docs/todo.md` 不保留已完成項目。
- `docs/trade-analysis-merge-plan.md` 已刪除。

## 測試策略

- 後端 unit test：有持股、無持股、SR Zone 重用、SR Zone 重算、資料不足。
- API handler test：統一 endpoint 的回應格式與錯誤語意。
- 前端 build：確認統一入口可編譯。
- Docker dev stack smoke test：使用 `docker-compose.dev.yml` 驗證 migration、API 與基本頁面流程。

## 完成定義

此計畫視為完成需同時滿足：

- 使用者可由單一入口輸入股票代碼取得交易分析。
- 資料庫無持股時，系統以空手分析處理。
- 有持股時，系統使用 AVG 成本與持股資訊產生決策。
- SR Zone 的 Data → Features → Score → Evidence 與 Position 的 Decision 層邊界清楚。
- `ADJUSTMENT` 決策保留，未被直接刪除。
- 正式文件已更新，暫時計畫書已刪除。
