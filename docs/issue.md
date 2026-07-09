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

---

### I-006：Position handler 把 ApplyEvent 的 infra 錯誤一律回 400、且不記 log

| 欄位 | 內容 |
|---|---|
| 狀態 | 待修復 |
| 嚴重度 | 中 |
| 分類 | Go / API / Position |
| 建立日期 | 2026-07-09 |
| 來源 | review「Position Analysis」功能（working tree）時發現 |

`backend/internal/api/handler/position.go`（約 line 110、149）在 `ApplyEvent` 之後只
特判 `ErrPositionVersionConflict`（回 409），其餘所有錯誤都走
`c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})`。但 `ApplyEvent` 也會回
`BeginTxx`／`FOR UPDATE`／`insertEvent`／replay `SelectContext`／`upsertPosition`／
`Commit` 的 DB/infra 錯誤——這些不是 client 的錯。結果：MySQL 連線中斷、deadlock、
Postgres FK 違規等會回 `400 {"error":"driver: bad connection"}`（洩漏內部錯誤、且暗示是
輸入問題），又因為跳過 `serverError()`，伺服器端沒有 log 可查。

修法：把 `applyPositionEvent` 的業務驗證錯誤（SELL 超賣、BUY 需正數、ADJUSTMENT 需理由）
包成 sentinel error（例如 `ErrPositionInvalidEvent`），handler 對它回 400，其餘走
`serverError`（500 + log）。

---

### I-007：ADJUSTMENT 直接覆寫 shares/avg 但不記 realized PnL，減股時去同步

| 欄位 | 內容 |
|---|---|
| 狀態 | 待修復 |
| 嚴重度 | 中 |
| 分類 | Go / Position / 會計 |
| 建立日期 | 2026-07-09 |
| 來源 | review「Position Analysis」功能（working tree）時發現 |

`backend/internal/store/position_repo.go::applyPositionEvent` 的 `ADJUSTMENT` 分支直接把
`position.Shares`/`AvgCost` 設成 target，完全不動 `RealizedPnL`。當調整是「減少」股數時，
那些股數的成本基礎憑空消失、沒有對應的已實現損益，`avg_cost*shares + realized_pnl` 不再
能對回實際現金流。可能是刻意的手動覆寫（有強制 reason），但目前對「減股調整」會靜默讓
realized PnL 與 ledger 對不上。需確認語意：若允許覆寫，應在減股時補記一筆推導的 realized
PnL，或在文件明確說明「ADJUSTMENT 不影響 realized PnL、僅供帳務對齊」。

---

### I-008：突破（Buy 且在所有壓力之上）永遠無法 ENTER/ADD

| 欄位 | 內容 |
|---|---|
| 狀態 | 待修復 |
| 嚴重度 | 中 |
| 分類 | Go / Position / Sizing |
| 建立日期 | 2026-07-09 |
| 來源 | review「Position Analysis」功能（working tree）時發現 |

`backend/internal/portfolio/analyzer.go::buildSnapshot` 的 `canIncrease` 需要 `rr.Valid`，
而 `rr` 需要 `takeProfit.Valid`、`takeProfit` 又需要 `resistance != nil &&
resistance.PriceLow > current`。當價格突破所有上方壓力（`pickResistanceZone` 回 nil，或
現價已在壓力帶內使 `PriceLow <= current`）時，`takeProfit`/`rr` 皆無效、`canIncrease=false`：
FLAT+`Buy` 變成 WAIT、LONG+`Buy` 落到 default 變 HOLD，永遠不會 ENTER/ADD。與系統「突破→
BUY」的定位相衝。需決定：突破時是否用其他方式定義停利/風險（例如 ATR 倍數目標）以便量化
RR，或明確接受「沒有上方壓力當目標時不進場」為刻意保守設計並文件化。

---

### I-009：migration 038 三套 DB schema 不等價（FK 與預設值）

| 欄位 | 內容 |
|---|---|
| 狀態 | 待修復 |
| 嚴重度 | 中 |
| 分類 | DB / Migration / Position |
| 建立日期 | 2026-07-09 |
| 來源 | review「Position Analysis」功能（working tree）時發現 |

`038_position_analysis.sql` 三套不等價：Postgres 的 `positions.last_event_id` 有
`REFERENCES position_transactions(id)` 外鍵，MySQL/SQLite 沒有；`note` 與 `position_analyses`
的多個文字/JSON 欄位在 pg/sqlite 有 `DEFAULT ''`/`'{}'`/`'[]'`，MySQL 則是 NOT NULL 無預設。
happy path 不會壞（repo 一律帶值），但參照完整性與預設行為依 DB 而異：刪除/重編被
`positions` 引用的 `position_transactions` 列，Postgres 會擋、MySQL/SQLite 放行；而 Postgres
的 FK 違規又會被 I-006 誤報成 400。若三套本應等價，需對齊（統一加/去 FK，統一預設值）。
