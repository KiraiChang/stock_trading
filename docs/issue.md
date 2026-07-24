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

目前無待處理項目。

已結案並歸檔：

- **I-019 ~ I-032**（SR Zone Final Entry 仲裁鏈的 context-aware entry executability、market-price
  RR / target / best_trade_zone 語意、entry blocking gate、final entry state 正規化、risk_notes
  reason-code 驅動改寫、market-price target 選取等一整組問題）已於 2026-07-24 review 通過並修正完成，
  現況規格歸檔於 [`sr-zone-scoring.md`](./sr-zone-scoring.md) 的 Final Entry Arbitration 與
  rr_context / rr_gate 段落。
- **I-033 ~ I-035**（T-020 Position owner scope 的 review 發現：group `AddMember` 角色保護、
  group membership 與 tenant membership 一致性、前端 `canWritePortfolio` 預設）已於 2026-07-24
  review 通過並修正完成，現況歸檔於 [`database-schema.md`](./database-schema.md)、
  [`architecture.md`](./architecture.md) 與 [`api-reference.md`](./api-reference.md)。

下一筆新問題從 `I-036` 起編。
