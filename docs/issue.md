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

### I-014：突破/跌破訊號未依現價位置過濾 S/R，對已跨越價位發假訊號

| 欄位 | 內容 |
|---|---|
| 狀態 | 待修復 |
| 嚴重度 | 中 |
| 分類 | Go / 訊號引擎 |
| 建立日期 | 2026-07-14 |
| 來源 | review `internal/signal`（fugle_stream 與 signal 關係檢視） |
| 位置 | `internal/signal/support_resistance.go:36-49`、`internal/signal/breakout.go:29-64` |

`CalcSupportResistance` 把 60 根視窗內所有 local high 當阻力、所有 local low 當支撐，不論其在
現價之上或之下。`CheckBreakout` 只用 `price > r.Price`／`price < s.Price` 比大小：上升段裡
多數「阻力」已在現價下方，`price > r.Price` 恆真，只要多頭＋量比 ≥ 2 就對很久以前已跨過的
價位發 BREAKOUT（阻力條件近乎失效）；下跌段同理，對已跌破的舊支撐 `price < s.Price` 恆真、
頻繁發 BREAKDOWN。

修復方向：阻力只保留現價之上、支撐只保留現價之下，並改為偵測「前一根仍在另一側、當根收盤
跨越」的跨越事件，而非單純比大小。與 I-015 相關。

---

### I-015：BREAKDOWN 缺量能與「1~2 根未收回」確認，與 stop_001 不符

| 欄位 | 內容 |
|---|---|
| 狀態 | 待修復 |
| 嚴重度 | 中 |
| 分類 | Go / 訊號引擎 |
| 建立日期 | 2026-07-14 |
| 來源 | review `internal/signal` |
| 位置 | `internal/signal/breakout.go:47-64` |

`CLAUDE.md` `stop_001` 定義跌破需「Close < Support ＋ 結構破壞 ＋ 1~2 根內未收回」。實作只判
`price < support && trend==Bearish`，當根即發訊、無量能門檻、無收回確認；配合 I-014，在跌勢中
容易反覆誤發。

修復方向：加入收回確認窗（1~2 根），並評估是否比照 BREAKOUT 要求量能配合。
