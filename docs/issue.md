# ISSUE：遇到的問題與已知限制

記錄實際發生過的 bug、矛盾結果、文件/程式碼不一致，以及設計上刻意接受的
限制。跟「想做的優化」無關的項目放這裡；未來想做的功能擴充記錄在
[todo.md](./todo.md)。

## 使用說明

- **狀態**：`待修復` / `修復中` / `已修復` / `已知限制（不計畫修復）`
- **嚴重度**：`高`（結果矛盾/資料錯誤）/ `中`（誤導但不影響核心功能）/
  `低`（文件或註解落後，不影響 runtime）
- 新增項目時往下加一筆，編號遞增（`I-0xx`）。修復後把「狀態」改成
  `已修復` 並補上「修復方式」欄位，不要刪除歷史紀錄。

---

### I-001：`docs/api-reference.md` 的 SR Zone API 範例仍是舊版五分量

| 欄位 | 內容 |
|---|---|
| 狀態 | 待修復 |
| 嚴重度 | 低 |
| 發現日期 | 2026-07-06 |
| 來源 | `docs/sr_zone_improve.md` Follow-up Review #1 |
| 相關檔案 | `docs/api-reference.md` |

程式與主要設計文件（`sr-zone-scoring.md`、`database-schema.md`、
`srZones.ts`）都已經改成六分量 `EV 34 + RR 17 + Trend 12.75 + Volume 12.75
+ Confidence 8.5 + Chip 15`，但 `docs/api-reference.md` 的 SR Zone response
範例仍缺 `chip`，文字也還寫「五個分量加總即為 `trading_score`」。API 使用者
會誤以為 `trading_score_breakdown` 只有五個 key。

**建議修法**：補上 `"chip": ...` 範例值，文字改成六分量，並確認範例數字
breakdown 加總等於 `trading_score`。

---

### I-002：`SRZones.svelte` 進階說明區塊仍用原始 `z.role`

| 欄位 | 內容 |
|---|---|
| 狀態 | 待修復 |
| 嚴重度 | 低 |
| 發現日期 | 2026-07-06 |
| 來源 | `docs/sr_zone_improve.md` Follow-up Review #2 |
| 相關檔案 | `frontend/src/routes/SRZones.svelte`（可信度說明那行） |

主要顯示、主要觀察區間、失效條件都已經改用 `effectiveRole(z)`，能正確處理
`resolved_role`。但進階區底部「可信度只用目前角色（...）方向的觸碰樣本
計算」那行仍用 `z.role`。若某 zone 原本是 `AT_ZONE`、後續 verifier 已解析出
方向，畫面會同時出現「已解析成支撐/壓力」跟「方向還不明確」兩種說法。

**建議修法**：改用 `effectiveRole(z)`；若想保留「confidence 實際上是用分析
當下角色計算」這個技術細節，另外補一句「分析當下角色：AT_ZONE；驗證後
角色：SUPPORT/RESISTANCE」，不要混在同一句「目前角色」文案裡。

---

### I-003：`backend/internal/store/model.go` 註解仍是舊規格

| 欄位 | 內容 |
|---|---|
| 狀態 | 待修復 |
| 嚴重度 | 低 |
| 發現日期 | 2026-07-06 |
| 來源 | `docs/sr_zone_improve.md` Follow-up Review #3 |
| 相關檔案 | `backend/internal/store/model.go` |

Runtime 欄位可以正確承接目前資料，但註解落後：`SRZoneAnalysis` 註解仍說
`GlobalExpectedValue`/`GlobalConfidence`/`GlobalRiskRewardRatio` 在 zones
為空或都沒有明確方向時可能是 `NULL`（實際上 `global_confidence` 只有
zones 是空陣列時才是 `NULL`，跟另外兩個欄位條件不同）；`TradingScoreBreakdown`
註解仍寫五個分量，缺 `chip`。不影響 API runtime，但會誤導之後改
repo/handler/DTO 的人。

**建議修法**：比照 `frontend/src/lib/api/srZones.ts` 目前的註解語意同步更新。

---

### I-004：非交易日 chip_scores 寫入過期分數（已修復）

| 欄位 | 內容 |
|---|---|
| 狀態 | 已修復 |
| 嚴重度 | 高 |
| 發現日期 | 2026-07-06 |
| 來源 | `docs/review.md` |
| 相關檔案 | `backend/internal/chip/sync.go` |
| 修復方式 | 新增 `hasDataForDate`，該日完全沒有 candle/法人/融資融券資料時直接跳過，不寫入 `chip_scores` |

`Syncer.computeAndStoreScore` 原本在非交易日（無資料）仍會用舊資料重算並
覆寫當日分數，導致非交易日出現看起來像「當日」但其實是過期的籌碼分數。

---

### I-005：回測籌碼過濾器缺資料時 fail-open（已修復）

| 欄位 | 內容 |
|---|---|
| 狀態 | 已修復 |
| 嚴重度 | 高 |
| 發現日期 | 2026-07-06 |
| 來源 | `docs/review.md` |
| 相關檔案 | `python/backtest/modular/backtester.py::_passes_chip_filter` |
| 修復方式 | 缺資料時分數視為中性值 `0.0`，而不是直接放行（`return True`） |

原本 `chip_scores` 或當日分數缺失時直接回傳 `True`（放行進場），等於過濾器
在資料缺失時完全失效，跟「有籌碼過濾」的預期行為相反。

---

### I-006：前端日期計算 UTC vs Taipei 時區 bug（已修復）

| 欄位 | 內容 |
|---|---|
| 狀態 | 已修復 |
| 嚴重度 | 中 |
| 發現日期 | 2026-07-06 |
| 來源 | `docs/review.md` |
| 相關檔案 | `frontend/src/lib/utils/date.ts`、`frontend/src/routes/Chips.svelte` |
| 修復方式 | 改用 `Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Taipei' })` 統一輸出 `YYYY-MM-DD`，並抽成共用 `date.ts` |

原本用 `new Date().toISOString()` 類的寫法會用瀏覽器 UTC 時間，在台灣時區
（UTC+8）午夜前後容易讓「今天」算成前一天或後一天。連帶發現 esbuild 會把
單行 `return <expr>` 的零參數函式消除成 `void 0`，修復時一併改成多行函式
避免這個問題再發生。

---

### I-007：SR Zone 使用最新籌碼分數而非 `before_date`（lookahead bias，已修復）

| 欄位 | 內容 |
|---|---|
| 狀態 | 已修復 |
| 嚴重度 | 中 |
| 發現日期 | 2026-07-06 |
| 來源 | `docs/review.md`、`docs/sr-zone-v3-chip-model-update.md` |
| 相關檔案 | `python/backtest/modular/sr_scoring/scoring.py::score_symbol` |
| 修復方式 | 改用 `fetch_latest_chip_score(symbol, before_date=chip_before_date)`，`before_date` 依 `analyzed_at` 換算，而非直接抓「最新」一筆 |

原本直接抓資料庫裡「最新」一筆籌碼分數，在回測情境下會偷看到 `analyzed_at`
之後才產生的籌碼資料（lookahead bias），造成回測績效虛高。

---

### I-008：SR Zone 機率模型缺乏獨立命名，`reject_count`/`rejection_count` 易混淆

| 欄位 | 內容 |
|---|---|
| 狀態 | 已知限制（不計畫修復） |
| 嚴重度 | 低 |
| 發現日期 | 2026-07-07 |
| 來源 | `docs/sr-zone-scoring.md` |
| 相關檔案 | `python/backtest/modular/sr_scoring/features.py`、`scoring.py` |

`reject_count`（拒絕次數，見 features.py）跟 `rejection_count`
（zone_features 內部欄位）命名相似，容易在閱讀程式碼時混淆；另外
`/sr-scoring/*`（Python 內部路由）跟 `/sr-zones/*`（對外 API）命名也容易
搞混。目前判斷維護成本不高，先記錄不強制重新命名（重新命名會牽動多層
API/DB/TS 契約）。

---

### I-009：停損停利同時觸發時不會自動判斷先後順序

| 欄位 | 內容 |
|---|---|
| 狀態 | 已知限制（不計畫修復） |
| 嚴重度 | 低 |
| 發現日期 | 2026-07-07 |
| 來源 | `docs/stock-analysis.md` |
| 相關檔案 | 個股分析 verifier 相關程式碼 |

同一根K棒內同時觸及停損價與停利價時，系統不會自動判斷哪個先發生，需要
人工比對 `hit_at` 決定實際結果。跟 SR Zone `labeling.py` 的 max_excursion
雙標籤問題（已於 `sr_zone_improve.md` review #3 修復，見本文件不重複列出）
是類似情境，但個股分析這邊目前選擇維持人工判斷，不自動化。

---

### I-010：Watching 進場判斷是規則式近似，非機率模型

| 欄位 | 內容 |
|---|---|
| 狀態 | 已知限制（不計畫修復） |
| 嚴重度 | 低 |
| 發現日期 | 2026-07-07 |
| 來源 | `docs/stock-analysis.md` |

「Watching」（觀察中）進場點的判斷目前是規則式近似（技術指標門檻），不是
像 SR Zone 那樣用訓練過的機率模型輸出 bounce/break probability。是否要把
這塊也升級成機率模型，目前沒有排入計畫（如果要做，應該記錄到
[todo.md](./todo.md) 而不是這裡）。
