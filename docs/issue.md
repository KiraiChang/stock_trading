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

---

### I-011：`scoring.py::TRADING_SCORE_WEIGHTS` 註解跟 v3 模型行為不符

| 欄位 | 內容 |
|---|---|
| 狀態 | 待修復 |
| 嚴重度 | 中 |
| 發現日期 | 2026-07-07 |
| 來源 | 審視 commit `07da5c2`「調整模型以及納入籌碼分析到模型內」時發現 |
| 相關檔案 | `python/backtest/modular/sr_scoring/scoring.py`（`TRADING_SCORE_WEIGHTS` 上方註解） |

`TRADING_SCORE_WEIGHTS` 上方註解寫「chip_score 是固定加權公式的輸入，
**不影響 hold/break 機率模型本身**，因此不需要重新訓練模型」。這句話在
commit `07da5c2` 之前是對的，但該 commit 已經把 `chip_features`（`chip_total
_score`/`chip_institutional_score`/`chip_margin_score`/`chip_broker_score`
/`chip_concentration_score`/`chip_missing`）加進 `FEATURE_COLUMNS`，
`predict_hold_probability`/`predict_break_probability` 現在都會吃 chip
特徵、`MODEL_VERSION` 也因此 bump 到 `v3`。註解沒有跟著更新，現在會讓人以為
籌碼只影響 `trading_score` 的固定 15% 權重，實際上籌碼還會透過模型影響
`bounce_probability`/`break_probability`，進而影響 `expected_value`（34%）
與 `support_score`/`resistance_score`。

**建議修法**：更新註解，明確說明 chip 現在有兩條路徑影響最終分數：(1) 模型
特徵（影響機率、進而影響 EV/support/resistance score）(2) 獨立的
`trading_score` 加權分量（15%，直接用原始 `chip_score`）。是否要因此調整
權重或拿掉其中一條路徑，記錄在 [todo.md](./todo.md) T-014，這裡只修正
文字本身的錯誤描述。

---

### I-012：`db.py::fetch_latest_chip_score` docstring 跟 `score_symbol()` 實際行為不符

| 欄位 | 內容 |
|---|---|
| 狀態 | 待修復 |
| 嚴重度 | 低 |
| 發現日期 | 2026-07-07 |
| 來源 | 審視 commit `07da5c2` 時，順帶檢查籌碼分數查詢路徑發現 |
| 相關檔案 | `python/db.py::fetch_latest_chip_score` |

Docstring 寫「即時評分（`score_symbol`）不帶這個參數，直接取全庫最新一筆」。
這句話描述的是 `docs/review.md` 那筆 lookahead bias 修復**之前**的行為。
修復後（見 [issue.md](./issue.md) I-007）`score_symbol()` 一律會算出
`chip_before_date`（依 `analyzed_at` 換算）並傳入 `before_date`，不會再直接
取全庫最新一筆。docstring 沒有跟著更新，內容跟現在的程式碼相反，容易誤導
之後改動這個函式或呼叫端的人，以為即時評分本來就该看全庫最新一筆而重新
把 lookahead bug 引入。

**建議修法**：把 docstring 改成「`score_symbol()` 一律會傳入
`before_date`（依分析當下的 `analyzed_at` 換算），確保不會看到分析當下
之後才產生的籌碼資料；只有測試或診斷用途才會省略 `before_date` 直接取
全庫最新一筆」。
