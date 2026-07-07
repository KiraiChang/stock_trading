# ISSUE：遇到的問題與已知限制

記錄實際發生過的 bug、矛盾結果、文件/程式碼不一致，以及設計上刻意接受的
限制。跟「想做的優化」無關的項目放這裡；未來想做的功能擴充記錄在
[todo.md](./todo.md)。

## 使用說明

- **狀態**：`待修復` / `修復中` / `已修復` / `已知限制（不計畫修復）`
- **嚴重度**：`高`（結果矛盾/資料錯誤）/ `中`（誤導但不影響核心功能）/
  `低`（文件或註解落後，不影響 runtime）
- 新增項目時往下加一筆，編號遞增（`I-0xx`）。修復後若仍需短期追蹤，先把「狀態」改成
  `已修復` 並補上「修復方式」；若修復紀錄已移到對應主題文件或 review 文件，
  則可從本清單移除。

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

### I-011：SR Zone 支撐壓力摘要看不到明確籌碼分析

| 欄位 | 內容 |
|---|---|
| 狀態 | 待修復 |
| 嚴重度 | 中 |
| 發現日期 | 2026-07-07 |
| 來源 | 支撐壓力頁面檢視 |
| 相關檔案 | `python/backtest/modular/sr_scoring/scoring.py`、`frontend/src/routes/SRZones.svelte`、`frontend/src/lib/api/srZones.ts` |

目前籌碼已經進入 SR Zone 評分：v3 模型把 `chip_features` 納入 hold/break probability model，`trading_score_breakdown.chip` 也以 15% 權重直接影響總分。後端短中長摘要的 `reasons` 會放入籌碼理由，例如籌碼偏多時對支撐有利、壓力可能較容易被挑戰。

但前端摘要目前只把 `reasons.slice(0, 3)` 串成一句文字，籌碼沒有獨立欄位、徽章或分數，也沒有依支撐/壓力角色分開顯示解讀。使用者在支撐壓力摘要層容易看不出籌碼已經影響模型與 trading score。

**建議修法**：把籌碼從一般 reasons 中拉出來，至少在短 / 中 / 長支撐與壓力卡片中顯示「籌碼：偏多 / 中性 / 偏空 / 無資料」與一句角色化解讀；進階區可顯示 `trading_score_breakdown.chip` 對總分貢獻了多少分。
