# Decision Pipeline

Decision Pipeline 負責把 Data / Analysis / AI 的輸出收斂為交易行動、部位調整與風控結果。
它是唯一能輸出「買、賣、加碼、減碼、等待、避免」等交易語意的 pipeline。

## 職責

- 消費分析快照、模型機率、部位 projection 與風險設定。
- 產生 action、target shares、adjustment shares。
- 建立停損、停利、RR、風險預算與 reason codes。
- 保存不可變 decision snapshot。
- 讓前端與使用者可以追溯「為什麼是這個決策」。

## 現有模組歸位

| 類別 | 現有位置 | 說明 |
|------|----------|------|
| Trade Analysis facade | `/trade-analysis/*` | 前端與 API 的統一決策入口 |
| Position Analysis | `backend/internal/portfolio` / `position_analyses` | FLAT/LONG 情境、風險、目標股數 |
| Position projection | `positions` | Decision 的部位上下文來源 |
| SR Decision Summary | SR Zone `decision_summary` | SR 結果收斂成 entry permission、RR context 等 |
| Signal final decision | `backend/internal/signal` | 最終 signal event 與方向 |
| 風控設定 | `position_analysis` config | risk budget、add-on ratio、minimum RR 等 |
| 交易理由 | reason / evidence JSON | 可追溯決策說明 |

## 輸入

- Data Pipeline 的 position projection 與交易流水。
- Analysis Pipeline 的 indicators、chip_scores、SR Zone snapshots、stock analysis snapshots。
- AI Pipeline 的模型機率與 model metadata。
- 系統設定中的風險參數。

## 輸出契約

- `position_analyses`
- action / action label
- target shares
- adjustment shares
- defense price
- structural stop
- take profit
- market RR
- position RR
- reason codes
- decision evidence
- final entry permission

## 不負責事項

- 不重新抓 K 棒或籌碼 raw data。
- 不重新訓練模型。
- 不直接修改歷史分析快照。
- 不改寫 immutable transaction；資料更正應新增 `ADJUSTMENT`。

## 決策快照原則

- 每次 decision 都保存不可變快照，不覆蓋舊結果。
- Decision 應引用當下使用的 analysis snapshot、model version/config hash 與 position state。
- 若上游資料缺漏，Decision 應降級、阻擋或標示原因，而不是自行補資料。
- FLAT 與 LONG 情境要清楚分開：空手看進場，持股看持有、加碼、減碼、停利、停損。

## SR Zone P0 契約

P0 先固定 Decision Pipeline 的唯一權威輸出，避免同一份 SR Zone 分析同時出現互斥交易語意。

| 類型 | 欄位 / 結構 | P0 要求 |
|------|-------------|---------|
| 市場偏向 | `market_bias` | 只回答市場結構偏多/偏空/中性，不等於可進場 |
| 進場權限 | `final_entry_permission` | 未持有者能否進場的唯一輸出；不得再由 `market_action` 或 `action` 取代 |
| 持股動作 | `position_action` | 已持有者如何處理的唯一輸出；需與 entry permission 分離 |
| Hard gates | active event、RR、EV、blocking zone、confidence | 必須有優先序，且 reason codes 可追溯 |
| Reason codes | `reason_codes` | 前端顯示與測試斷言應依 reason codes，而非自由文字解析 |

SR Zone P3 後，對外語意由 `decision_derived_view.semantic_pipeline` 串成
`Event -> Lifecycle -> Market State -> Bias -> Action -> Entry`。`market_bias`、
`position_action_condition.state` 與 `final_entry_permission.state` 皆由此鏈路推導；
legacy `market_action` / `action` / `entry_action_state` 僅保留為相容與明細，不得作為最終決策來源。

P0 也明確停用或降級舊重複欄位：

- `market_action`
- `action`
- `action_label`

這些欄位若仍需相容舊前端或舊快照，只能作為 deprecated alias，不能作為最終決策來源。

## 近期整理方向

- 將 `decision_summary` 與 analysis score breakdown 的文件界線再拉清楚。
- 將 Signal Engine 中的「分析前置」與「最終 signal decision」逐步分開。
- 後續程式重構時，Decision Pipeline 應只依賴明確 input DTO，不直接依賴上游內部 repo 細節。
