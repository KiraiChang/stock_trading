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

### I-040：production regression governance gate 在該模型首次 decision-replay 寫入前為 no-op（by-design）

| 欄位 | 內容 |
|---|---|
| 狀態 | 已知限制（刻意，by-design） |
| 嚴重度 | 低 |
| 分類 | Python / SR Zone / 模型治理 |
| 發現日期 | 2026-07-27 |
| 來源 | T-002 P2 review |

`pipeline._merge_regression_governance_gate` 只有在 `fetch_latest_sr_regression_governance(model_config_hash)`
查得同模型、`schema_version=sr_zone_decision_replay_p0` 的最新 replay 結果時才會作用。若該
`model_config_hash` 尚未跑過任何 `--write-db` 的 decision replay（新訓練模型、或 scheduler 關閉且從未手動
執行），fetch 回 None → gate 為 no-op，分析維持原本模型治理。這是刻意的安全預設（gate 只趨保守、
不因缺資料而誤擋），但意味著**這層 P2 安全網要等該模型至少跑過一次 evaluation 並寫入 DB 後才生效**。
上線流程若倚賴此 gate，需確保新模型部署後有排入一次 decision replay。屬營運相依，非 bug。

---

### I-049：`_chip_row_for_as_of` 遇到缺 `trade_date` 的 context row 會拋 KeyError

| 欄位 | 內容 |
|---|---|
| 狀態 | 已修復（待 review） |
| 嚴重度 | 中 |
| 分類 | Python / SR Zone / 模型驗證 |
| 發現日期 | 2026-08-04 |
| 來源 | T-002 A 區 review 修復後的自我 review |

`evaluation._chip_row_for_as_of` 原本直接 `row["trade_date"]`，只要 chip context 裡有一列
缺這個欄位就 `KeyError`，**整個 decision replay job 直接失敗**。姊妹函式
`_snapshot_for_as_of` 本來就有 `if raw_time is None: continue` 的防護，兩者不一致。

可達性：`POST /sr-zones/evaluate` 允許呼叫端自帶 `chip_scores_by_symbol`（見
[`sr-zone-scoring.md`](./sr-zone-scoring.md) 的「Replay context 的股票比對規則」），
外部給的資料不保證每列都有 `trade_date`。這是既有問題，不是同批 review 的修復引入的，
在補「context row 缺時間欄位」的邊界測試時才浮現。

**修復方式（2026-08-04，已實作）**：改用 `row.get("trade_date")`，`None` 或
`pd.Timestamp` 解析失敗都只 `continue` 跳過該列，與 `_snapshot_for_as_of` 行為一致。
新增測試：context 混入一列沒有 `trade_date` 的資料時，replay 仍完成且 as-of 命中有時間戳
的那一列。

---

下一筆新問題從 `I-050` 起編。
