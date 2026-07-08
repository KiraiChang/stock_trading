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

### I-001：持股分析多筆寫入未包 transaction，失敗會留下孤兒 SR 分析

| 欄位 | 內容 |
|---|---|
| 狀態 | 待修復 |
| 嚴重度 | 中 |
| 分類 | Go / Portfolio |
| 建立日期 | 2026-07-08 |
| 來源 | 審視 commit `37b6b4f`「加入持股操作分析」時發現 |

`backend/internal/portfolio/analyzer.go` 的 `Analyze` 依序做多筆寫入：先
`srZoneRepo.Create`（寫入 `sr_zone_analyses` + zones），再 `holdings.CreateAnalysis`
（寫入 `holding_analyses`）。整段沒有包在同一個 DB transaction 裡，若後段
`CreateAnalysis` 失敗，前段已寫入的 SR 分析與 zones 會成為孤兒資料（沒有任何
`holding_analyses` 引用、卻仍出現在 SR Zone 歷史）。

建議把 SR 分析建立與 holding 分析建立包進同一個 transaction，或在後段失敗時補償
刪除前段寫入的 SR 分析。

---

### I-003：handler 存在性檢查把所有錯誤當 404，掩蓋 500

| 欄位 | 內容 |
|---|---|
| 狀態 | 待修復 |
| 嚴重度 | 中 |
| 分類 | Go / API |
| 建立日期 | 2026-07-08 |
| 來源 | 審視 commit `37b6b4f`「加入持股操作分析」時發現 |

`backend/internal/api/handler/holding.go` 的 Update / Delete / Analyze / ListAnalyses
都用 `if _, err := h.repo.Get(ctx, id); err != nil { 回 404 }` 做存在性檢查，
`GetAnalysis` 也把任何錯誤一律回 404。DB 連線中斷等暫時性錯誤會被誤報成
「holding not found」，掩蓋真正的 500，妨礙除錯與監控。

建議只在 `errors.Is(err, sql.ErrNoRows)` 時回 404，其餘走既有的 `serverError`
（回 500 並記 log）。

---

### I-004：持股分析重用 SR 快照缺新鮮度窗，「分析」可能回傳過期資料

| 欄位 | 內容 |
|---|---|
| 狀態 | 待修復 |
| 嚴重度 | 中 |
| 分類 | Go / Portfolio / SR Zone |
| 建立日期 | 2026-07-08 |
| 來源 | 審視 commit `1b999bd`「持股分析對於 sr_zone 的分析去重處理」時發現 |

commit `1b999bd` 為了解決舊 I-002（持股分析每次都新寫一筆 SR 快照、污染 SR Zone
歷史）加入去重：`analyzer.go` 的 `findExistingSRSnapshot` 用
`srZoneRepo.List(symbol, 200)`（`ORDER BY created_at DESC`）找出該 symbol+timeframe
**最新一筆** SR 快照就直接重用，找不到才呼叫 `ScoreZones` 新建。污染/累積面已解決，
但重用**完全沒有新鮮度或時間窗限制**（測試 `TestAnalyzeReusesExistingSRZoneSnapshot`
刻意用 7/1 的快照、於 7/8 重用，證實這是預期行為）。衍生問題：

1. **「分析」可能回傳過期資料**：重用舊快照時，`current_price`（前端顯示為「現價」）、
   未實現損益、停損/停利價都來自舊快照。使用者今天按「分析」卻看到數天前的價位與
   PnL，對 human-in-the-loop 交易輔助工具是誤導。
2. **可能永遠凍結**：若持股標的沒有其他來源（watchlist／排程器）重算 SR，第一次分析
   建立的快照會被無限重用，之後每次「分析」的現價/損益都不再更新。

建議把重用限制在新鮮度窗內（例如 `analyzed_at` 為同一交易日，或加可設定 TTL），
超出範圍就 fall through 去跑一次新的 `ScoreZones`——即舊 I-002 原本「複用**當日**既有
分析」的意圖，而非無限期重用。
