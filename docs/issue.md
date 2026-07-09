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

### I-005：ZoneScore.UnmarshalJSON 遇到 nested 且 score 為 null 時靜默解成全零 zone

| 欄位 | 內容 |
|---|---|
| 狀態 | 待修復 |
| 嚴重度 | 低 |
| 分類 | Go / SR Zone |
| 建立日期 | 2026-07-09 |
| 來源 | review「SR Zone v2 pipeline evidence」變更（working tree，未提交）時發現 |

`backend/internal/analysis/client.go` 的 `ZoneScore.UnmarshalJSON`（約 line 303）
用 `if nested.Score != nil` 判別 nested/flat。若 zone 是 wrapper 形
`{"data":..,"features":..,"score":null,"evidence":..,"lifecycle":..}`，JSON `null`
會讓 `nested.Score == nil` → 落到 direct 分支，把整個 wrapper 當扁平 ZoneScore 解，
但 wrapper 的鍵（data/score/lifecycle…）都對不上扁平 tag（price_low/method…），
結果 zone 被解成 `price_low=0, method="", trading_score=0`、丟掉 features/evidence、
`ConfluenceCount` 被強制成 1、`Status=PENDING`，且**不報錯**。目前 Python 一定輸出
非 null 的 score，屬潛在/脆弱問題，但 null-vs-flat 的判別方式不穩健。

修法：改用更明確的判別（例如偵測 wrapper 專屬鍵，或當 nested 且 score 為 null 時
回傳明確錯誤）。
