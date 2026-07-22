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

### I-002：對外標籤各自平行推導，狀態未同步時會互相矛盾

| 欄位 | 內容 |
|---|---|
| 狀態 | 待修復 |
| 嚴重度 | 中 |
| 分類 | Python / SR Zone / decision_summary |
| 發現日期 | 2026-07-22 |
| 來源 | 使用者觀察 ＋ 2026-07-22 code review |

同一份分析中，多組對外欄位對同一件事給出互相矛盾的敘述：

- `Bullish Trend` vs `market_bias=REVERSAL_BIAS`（反轉觀察）
- RR 通過、`entry_action_state=PROBE_ALLOWED` vs 持有建議「條件式持有」
- event 判定「連兩日收高」 vs 價格路徑敘述「價格未延續」

成因不是模型判斷錯，而是這些標籤**各自從不同輸入平行推導**：有的看 `structure_state`、
有的看當下 `market_events`、有的看 carried `event_state_summary`、有的看 `market_action`。
事件在生命週期中前進一步（產生 → ACTIVE → 延續 → 失效／過期）時，沒被同步到的那一邊
就停在舊敘述，外顯為互相打架。三組矛盾是同一個結構缺陷的三個投影。

已知的具體實例（`decision_engine.py`）：

1. **新事件被標成延續**：line 270 規定「出現 `INTRADAY_RECLAIM` 事件即設
   `short_term_regime=RECLAIM_ATTEMPT`」，而 line 580 已把 `RECLAIM_ATTEMPT` 納入
   `BULLISH_CONTINUATION` 分支。因此一筆**剛發生、尚未確認**的收復也會被標成「多頭延續」，
   但其 entry permission 常仍是「條件式持有」——等於把第一組矛盾轉移成第二組。
   line 582-583 的 `REVERSAL_BIAS` 分支對 `INTRADAY_RECLAIM` 因此幾乎不可達，且無測試覆蓋。
2. **AVOID 時 bias 仍可為反轉觀察**：事件分支（line 582-583）排在 `market_action == "AVOID"`
   分支（line 584-585）之前，故 AVOID ＋ 反轉／收復事件會輸出 `REVERSAL_BIAS` 而非偏空標籤，
   與 `docs/sr-zone-scoring.md` 第 645-646 行宣稱的「market_bias、market_action、
   final_entry_permission 三者語意一致」不符。
3. **文件落後於實作**：`sr-zone-scoring.md` 第 641-643 行的 market_bias 規格仍只列
   `RECOVERY` / `EARLY_TREND`，未含 `RECLAIM_ATTEMPT`。**刻意先不改文件**——該行為本身尚待
   設計確認（見 1.），現在改文件等於替未定案的行為背書。

修法方向不是逐個分支補特例（那正是造成 1. 的做法），而是統一由 Event Lifecycle 推導，
見 [todo.md 的 T-034](./todo.md)。在架構收斂前，新增 `market_bias` 之類的分支特例要留意
是否只是把矛盾搬到另一組欄位。

#### 建議處理方向（2026-07-22 code review，待與 T-034 一併處理）

**實例 2（AVOID 仍輸出 REVERSAL_BIAS）：先不要單獨調分支順序。**
把 `market_action == "AVOID"` 分支往上搬看似一行就能解，但那又是「在平行推導上疊順序
特例」，與造成實例 1 的做法同型，只會把矛盾推到下一組欄位。真正要定的是語意問題：
AVOID ＋ 反轉／收復事件時，對外應說「偏空」還是「反轉觀察但不可進場」？這屬於 T-034
要一次定義的 derived view 規則，處理時應**產出 `market_bias` 的完整真值表**
（short_term_regime × event 狀態 × market_action → bias），而不是再補一個 `if`。

**實例 3（規格未含 RECLAIM_ATTEMPT）：等行為定案再一次更新文件。**
現在把 `RECLAIM_ATTEMPT` 補進 `sr-zone-scoring.md` 第 641-643 行，等於替尚在爭議中的
行為（實例 1）背書；若 T-034 收斂後語意改變，文件與依規格撰寫的契約測試要再翻一次。
故先在此列管，待 T-034 定案後**同一次**更新主題文件規格、補上依規格獨立推導的契約測試，
再把本筆從 issue 清單移除。若在 T-034 之前就需要文件與現況一致，替代做法是補上
`RECLAIM_ATTEMPT` 並明確標註「暫行，待 T-034 收斂」，不要無註記地寫成正式規格。
