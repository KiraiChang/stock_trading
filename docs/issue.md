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

### I-002：每次「分析」都往共用 SR 歷史表寫一筆、無去重

| 欄位 | 內容 |
|---|---|
| 狀態 | 待修復 |
| 嚴重度 | 中 |
| 分類 | Go / Portfolio / SR Zone |
| 建立日期 | 2026-07-08 |
| 來源 | 審視 commit `37b6b4f`「加入持股操作分析」時發現 |

持股頁每按一次「分析」，`Analyze` 都會呼叫 `client.ScoreZones` 並 `srZoneRepo.Create`
寫入一筆全新的 `sr_zone_analyses`。這些記錄跟從 SR Zone 頁手動分析產生的記錄混在
同一張表，會污染 SR Zone 頁面的歷史清單，且同一檔同一天重複按會不斷累積、沒有
任何去重或來源標記。

需先確認是否為預期行為（快照不可變設計本身需要一份當次 SR 分析）。若不希望污染
SR 歷史，可考慮：複用當日既有 SR 分析、或替持股觸發的 SR 分析加上來源標記／獨立
過濾，讓 SR Zone 頁面只顯示手動分析。屬設計層面，方向確定後再展開。

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
