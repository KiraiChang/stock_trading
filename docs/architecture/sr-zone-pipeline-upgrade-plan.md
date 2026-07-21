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
| P2-B | P2 | AI | Calibration / walk-forward / dataset diagnostics JSON contract | 否 |
| P2-C-1 | P2 | Data / Schema | Decision / Event normalized tables 第一批 | 是 |
| P2-C-2A | P2 | Data / Schema | Daily candidate normalized table | 是 |
| P2-C-2B | P2 | Data / Schema | Model metrics / model governance normalized tables | 是 |
| P2-C-2C | P2 | API / Frontend | 已正規化區塊由 normalized rows 組 response | 否 |

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

- [x] event detection DTO
- [x] event state DTO
- [x] transition function
- [x] active / resolved 判斷
- [x] 0050 fixture
- [x] Active Risk Gate 不再被歷史事件永久阻擋

P1 已落地為無資料表 lifecycle 摘要：`decision_summary.market_events` 保留 raw event
chain，`decision_summary.event_state_summary` 回報 active / resolved state。Decision
hard gate 只讀 active bearish event；已被 reclaim/reversal resolve 的 breakdown 不再強制
`EXIT`。`price_path.path_state` 新增 `EVENT_RISK` 表示仍有 active bearish event 阻擋。

P1 當時先暫緩 `market_event_detections` / `market_event_states`，原因是事件狀態語意尚未由
fixture 驗證前，不應先承諾資料表。後續已在 P2-C 正規化批次補上 event detection/state
tables，並由 API response snapshot 使用。

## P2：AI Governance 與 DB 正規化

P2 才處理模型治理與 schema 正規化：

- [x] model health gate（先落在 `probability_context.health` / `decision_summary.model_governance`）
- [x] confidence factors（先以 `confidence_gate` 表達 AI Pipeline 對 entry 的上限）
- [x] calibration report（先落在 `probability_context.model_reports`）
- [x] walk-forward output（先落在 `probability_context.model_reports`）
- [x] `stock_sr_decisions`（第一批，不回填舊資料）
- [x] `market_event_detections`（第一批，不回填舊資料）
- [x] `market_event_states`（第一批，不回填舊資料）
- [x] `stock_sr_daily_candidates`（P2-C-2A，不回填舊資料）
- [x] `stock_sr_model_metrics`（P2-C-2B，不回填舊資料）
- [x] `stock_sr_model_governance`（P2-C-2B，不回填舊資料）
- [x] `stock_sr_regression_results`（P2-C-3，不回填舊資料）
- [x] JSON text → JSONB（P2-C-4，PostgreSQL only）
- [x] `stock_sr_decisions` detail JSON projection（P2-C-5，先保留 legacy fallback）
- [x] API / Frontend normalized-only response（P2-C-6，舊資料以 missing/null 呈現）

P2-A/P2-B 已先以無 DB 契約落地：AI Pipeline 輸出 model governance、calibration
report、walk-forward report 與 dataset diagnostics；Decision Pipeline 只消費
`model_governance.health_state` / `confidence_gate`，不直接讀 raw model metrics。

P2-C 第一批已採「不考慮舊資料」落地：新增 `stock_sr_decisions`、
`market_event_detections`、`market_event_states`，Go `ToStore()` 從
`decision_summary` 解析 normalized projection，`SRZoneRepo.Create()` 在同一個 transaction
寫入 analysis、zones 與第一批 normalized tables。舊 JSON 欄位暫保留作為 raw/debug 與前端
相容來源，不做舊快照 backfill。

P2-C-2A 已新增 `stock_sr_daily_candidates`，Go `ToStore()` 從
`decision_summary.daily_candidate_zones` 解析 projection，`SRZoneRepo.Create()` 在同一個
transaction 寫入 daily candidate normalized rows。舊 JSON 欄位暫保留作為 raw/debug 與前端
相容來源，不做舊快照 backfill。

P2-C-2B 已新增兩張 AI Pipeline 正規化表：`stock_sr_model_metrics` 保存訓練任務完成時
的 hold/break 模型品質 projection，`stock_sr_model_governance` 保存每次 analysis 套用模型後
的 health/gate/report projection。兩者分開，避免把「模型訓練品質」與「單次分析決策可信度」
混為同一層資料。

P2-C-2C 已新增 SR Zone response snapshot 組裝層：`Create` / `Get` / `Verify`
都先載入 normalized rows，再把 decision authority fields、market events、event state summary、
daily candidates、model governance/model reports 組回既有前端相容 response shape。這是
P2-C-6 normalized snapshot primary 的過渡步驟。

P2-C-3 已新增 `stock_sr_regression_results` 作為 regression fixture、walk-forward
與 calibration 回歸驗證結果的 normalized 紀錄。這張表保存跨模型設定或 pipeline 版本的驗收結果，
和 `stock_sr_model_metrics` 的「單次 train job 品質 projection」分開，避免訓練任務清理影響
長期 regression 驗收紀錄。

P2-C-4 已新增 PostgreSQL-only JSONB migration，將 SR Zone analysis / zone raw JSON、
decision/event projection、daily candidate、model quality 與 regression result JSON 欄位轉成
JSONB。SQLite / MySQL 維持 TEXT / LONGTEXT 儲存 JSON 字串，Go `RawJSON` 仍使用 string 綁定以
維持三種資料庫相容。

P2-C-5 已在 `stock_sr_decisions` 補齊 decision detail JSON projection，包括 market regime、
data quality、price path detail、daily confirmation、defense lines、RR gate/context、
position action condition、market context、confidence explanation、risk notes 與 zone summary
集合，為 P2-C-6 將 API / Frontend response 切成 normalized snapshot primary 鋪路。

P2-C-6 已將 API / Frontend 讀取面切成 normalized snapshot primary：`decision` 不再以
`stock_sr_zone_analyses.decision_summary` 作 base，`probability_context` 不再以
`stock_sr_zone_analyses.probability_context` 作 base。舊 analysis 若缺 normalized rows，response
回 `null`，並以 `normalized_status` 標示 `missing`；前端據此顯示缺 normalized snapshot 狀態。

## 不建議先做

- 不先做 model registry。
- 不先做大型 DB 正規化。
- 不在 Decision 邊界未穩前擴充更多 AI 模型判斷。

## 驗收方式

P0 完成時，應滿足：

- `docs/sr-zone-scoring.md` 可清楚分辨 Analysis / AI / Decision 欄位。
- `docs/sr-zone-improve.md` 的 P0 項目已有對應 pipeline 歸屬。
- 後續實作不會再把模型機率、分析 score、交易 action 混成同一層欄位。
- Decision / Event / Daily Candidate / Model Governance 已由 normalized tables 承接。
- API / Frontend 不再把 legacy JSON 當正常 response source；舊 analysis 缺 normalized rows 時
  以 `normalized_status=missing` 與 `null` 區塊呈現。
