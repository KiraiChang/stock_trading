# SR Zone Pipeline Upgrade Plan

本文件記錄 `docs/sr-zone-improve.md` 與四條 pipeline 架構的實作先後順序。目標是先讓
SR Zone 改進項目被 Data / Analysis / AI / Decision Pipeline 邊界承接，再進入功能與
schema 實作，避免在契約尚未穩定時先建立大型事件表或 DB 正規化。

## 核心判斷

`docs/sr-zone-improve.md` 內含多種不同層級的工作：

- 語意 bug 修正：欄位名稱、顯示語意、重複 action 欄位。
- Analysis Pipeline：zone、features、score breakdown、confluence、evidence。
- AI Pipeline：模型健康度、機率可信度、walk-forward、calibration。
- Decision Pipeline：market bias、entry permission、position action、hard gates。
- Data / Schema：event tables、JSONB、decision tables、regression result tables。

因此不建議直接照該文件 Sprint 1~5 從大型 Event Lifecycle schema 開始做。正確順序是先處理
pipeline 邊界與語意收斂，再做事件生命週期，最後才做 AI governance 與 DB 正規化。

## 推薦順序

| 階段 | 優先級 | Pipeline | 內容 | 是否先改 DB |
|------|--------|----------|------|-------------|
| P0-A | P0 | Analysis / AI / Decision | SR Zone 輸出契約分層 | 否 |
| P0-B | P0 | Analysis / Decision | 語意 bug 與欄位矛盾修正計畫 | 否 |
| P0-C | P0 | Decision | Decision Arbitration 單一出口前置規格 | 否 |
| P1-A | P1 | Analysis / Decision | Event Lifecycle 純函數與 fixture | 否 |
| P1-B | P1 | Analysis | Market State Engine | 否 |
| P1-C | P1 | Analysis / Decision | Best Trade Zone 與 Price Path schema 穩定化 | 否 |
| P2-A | P2 | AI | Model Health、Confidence、Walk-forward | 否 |
| P2-B | P2 | Data / Schema | JSONB、event tables、decision tables | 是 |

## P0-A：SR Zone Pipeline 邊界契約

先把 SR Zone 的輸出分清楚，不急著搬程式碼。

### Analysis Pipeline

Analysis Pipeline 負責產生可解釋分析快照與 evidence：

- zone detection
- feature engineering
- score breakdown
- `confluence_family_count`
- `confluence_families`
- `chip_summary`
- `entry_relevance_breakdown`
- `price_path`
- `defense_lines`
- `evidence`
- `explanation`

Analysis Pipeline 可以消費 AI Pipeline 的機率輸出，但不得把最終交易行動混在 score breakdown 裡。

### AI Pipeline

AI Pipeline 負責模型產物、模型狀態與模型可信度：

- `bounce_probability` / `break_probability` 或後續改名後的 likelihood score
- `model_version`
- `model_config_hash`
- `training_config`
- train job metrics
- model health
- calibration report
- walk-forward report

AI Pipeline 不輸出 `BUY` / `SELL` / `HOLD`，也不決定 position sizing。

### Decision Pipeline

Decision Pipeline 是唯一能輸出交易語意的 pipeline：

- `market_bias`
- `final_entry_permission`
- `position_action`
- hard gates
- reason codes
- entry precedence
- position action rules
- blocking zone decision

Decision Pipeline 只消費 Analysis / AI / Data 的輸出，不重新抓 K 棒、不重新同步籌碼、不重新訓練模型。

## P0-B：語意修正計畫拆分

此階段先規劃並拆小項，再進入程式實作。

| 項目 | Pipeline | 目的 |
|------|----------|------|
| `tier_label` / `role_label` / `display_label` | Analysis | 避免 `RESISTANCE` 顯示成「短期支撐」 |
| chip signal 門檻與 UI 顯示 | Analysis | 避免只依正負號著色，讓弱多/弱空有一致語意 |
| 停用 `market_action` / `action` / `action_label` | Decision | 避免與 `final_entry_permission`、`position_action` 矛盾 |
| confluence family 顯示 | Analysis | UI 顯示證據族群，不把 raw method count 當獨立證據數 |
| BUY + WAIT_CONFIRMATION 衝突 | Decision | 最終 entry permission 必須有單一權威輸出 |

完成 P0-B 後，文件與 API 契約應能回答：

- 哪些欄位是分析結果。
- 哪些欄位是模型輸出。
- 哪些欄位是交易決策。
- 哪些舊欄位已被取代、僅保留相容。

## P0-C：Decision Arbitration 前置規格

Decision Arbitration 實作前，先固定規格：

- 單一 `final_entry_permission`
- 單一 `position_action`
- hard gate priority
- entry precedence
- position action precedence
- reason code schema
- blocked / wait / probe / small entry / accumulate 的排序
- exit / reduce / conditional hold / hold / add 的排序

P0-C 不要求完整 Event Lifecycle DB；可以先以現有 `decision_summary`、daily confirmation、
price path、RR/EV gate、chip state、blocking zone 等欄位作為輸入。

## P1：事件與市場狀態

P1 才開始承接 `docs/sr-zone-improve.md` 的 Event Lifecycle 與 Market State。

建議先做純函數與 fixture：

- event detection DTO
- event state DTO
- transition function
- active / resolved / failed / expired 判斷
- 0050 fixture
- Active Risk Gate 不再被歷史事件永久阻擋

暫緩：

- `market_event_detections`
- `market_event_states`

原因：事件狀態語意尚未由 fixture 驗證前，不應先承諾資料表。

## P2：AI Governance 與 DB 正規化

P2 才處理模型治理與 schema 正規化：

- model health gate
- confidence factors
- calibration report
- walk-forward output
- JSON text → JSONB
- `stock_sr_decisions`
- `stock_sr_daily_candidates`
- `market_event_detections`
- `market_event_states`
- `stock_sr_model_metrics`
- `stock_sr_regression_results`

這些項目需要穩定的 P0/P1 契約支撐，否則會產生 migration 連鎖修改。

## 不建議先做

- 不先建 event tables。
- 不先做 JSONB migration。
- 不先拆 `stock_sr_decisions`。
- 不先做 model registry。
- 不先做大型 DB 正規化。
- 不在 Decision 邊界未穩前擴充更多 AI 模型判斷。

## 驗收方式

P0 完成時，應滿足：

- `docs/sr-zone-scoring.md` 可清楚分辨 Analysis / AI / Decision 欄位。
- `docs/sr-zone-improve.md` 的 P0 項目已有對應 pipeline 歸屬。
- 後續實作不會再把模型機率、分析 score、交易 action 混成同一層欄位。
- 明確列出先不做 event table / JSONB / DB 正規化。
