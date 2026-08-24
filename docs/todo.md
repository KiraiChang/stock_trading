# TODO：優化與待實作項目

記錄想做但還沒做的優化方向、功能擴充、架構升級。跟 bug/矛盾/限制無關的項目
放這裡；已經發生的問題或已知限制記錄在 [issue.md](./issue.md)。

## 使用說明

- **狀態**：`待規劃` / `規劃中` / `進行中` / `已完成` / `擱置`
- **優先度**：`高` / `中` / `低`（主觀評估，會隨情境調整，不是嚴格排序）
- 新增項目時往下加一筆，編號遞增（`T-0xx`），不要覆蓋舊編號。
- 項目狀態改變時直接更新該筆的「狀態」欄位，不需要搬移位置；若項目已完成
  且不需要保留歷史，可以整筆刪除或搬到文件最下方的「已完成封存」。

---

### T-002：SR Zone 機率模型自動化回測 pipeline

| 欄位 | 內容 |
|---|---|
| 狀態 | 規劃中 |
| 優先度 | 中 |
| 分類 | Python / SR Zone / 模型驗證 |
| 建立日期 | 2026-07-07 |
| 來源 | `docs/sr-zone-scoring.md` 已知限制 |
| P0 狀態 | 已實作（calibration bins 已於 2026-08-04 補齊；2026-08-05 review 通過） |
| P1 狀態 | 部分已實作（decision replay、DB 落地、API 與 UI 手動入口已完成） |
| P2 狀態 | 部分已實作（機制完整且預設關閉；正式啟用另有前置條件，見下方 review） |

目前只有 `train.py` 手動訓練 + train job metrics（time-split holdout、校準、
dataset diagnostics），沒有「模型上線後過去一段時間的訊號實際表現如何」的
自動化驗證。這項應先做成 SR Zone 專用 evaluation pipeline，不直接套用一般
`backtest/modular` 的交易策略回測；原因是 SR Zone 需要同時驗證 probability、
zone outcome、event lifecycle、daily confirmation 與 final entry state 的語意表現。

實作計畫：

- **P0：Python evaluation runner**（已實作，2026-08-05 review 通過）
  - 新增 `backtest.modular.sr_scoring.evaluation`。
  - 輸入 symbols、timeframe、limit / date range、model path、builder config。
  - 對每檔股票做 walk-forward evaluation：每個時間點只能用當下以前的 OHLCV
    建 zone、算 probability / score / decision，再用未來 N 根 K 棒產生 label。
  - 第一版輸出 JSON report，不先接 UI 或排程。
- **P0：核心驗證指標**（已實作，2026-08-05 review 通過）
  - 模型層：hold / break AUC、Brier score、log loss、calibration bins（2026-08-04 補齊，
    schema `sr_evaluation_calibration_v1`）。
  - Zone 層：support hold rate、resistance rejection rate、breakout continuation rate。
  - Decision 層：`WAIT_CONFIRMATION`、`PROBE_ENTRY`、`ENTRY_ALLOWED` 的後續勝率、
    失效率、平均報酬與 RR 分布。
  - Daily confirmation：納入隔日 / 兩日確認成效統計（已完成，現況見
    [`sr-zone-scoring.md`](./sr-zone-scoring.md)）。
- **P1：結果落 DB**（已實作起步，方向已 review；續作項目見下方剩餘工作）
  - 優先寫入既有 `stock_sr_regression_results.metrics_json`。
  - `run_id` 使用 `sr_eval_yyyymmddhhmmss`，並記錄 `model_config_hash`、
    `pipeline_version`、dataset range 與 split method。
  - 暫不新增拆欄 schema；等 report 指標穩定後再評估是否正規化。
- **P1：CLI / API**（CLI、evaluate trigger API 與 regression result list API 已實作起步）
  - 先提供 CLI：`python -m backtest.modular.sr_scoring.evaluation ...`。
  - Go API 已新增 `GET /sr-zones/regression-results` 讀取
    `stock_sr_regression_results`，支援 `limit` 與 `schema_version` 篩選。
  - Go API 已新增 `POST /sr-zones/evaluate`，轉呼叫 Python
    `POST /sr-scoring/evaluate`，可手動觸發 evaluation 或 decision replay；`write_db=true`
    時由 Python 寫入 `stock_sr_regression_results`。
- **P2：排程與模型治理**（部分已實作，方向已 review；續作項目見下方剩餘工作）
  - 已先提供可設定的 daily / weekly cron 入口，預設關閉；待 report schema review 後再決定正式啟用策略。
  - production analysis 已接入同模型最新 regression governance gate；若 evaluation
    未通過門檻，會標記模型 degraded / unreliable，並保守限制 entry gate。

驗證與風險：

- 必須避免 lookahead bias；builder、feature、decision 都只能吃當下以前資料。
- 第一版不要做交易資金模擬，先驗證訊號分類與後續 outcome，避免把 sizing /
  portfolio policy 的問題混進模型品質判斷。
- T-003 的 ATR 參數調整應依賴本項 evaluation 結果，不應單獨憑主觀調常數。

P0 已實作範圍：

- `evaluation.py` 提供 CLI / JSON report 與 `run_evaluation()`。
- 目前 report 已包含 dataset summary、zone outcome、model availability、hold/break
  AUC / Brier score / log loss 與 calibration bins（模型存在時）。
- DB 落地已支援寫入 `stock_sr_regression_results`；decision replay 分層統計已起步，
  Go evaluate background job API、result list API、前端手動入口與 scheduler 排程入口已實作。

P1 已實作範圍：

- CLI / `run_evaluation()` report 已包含 `run_id`、`pipeline_version`、`split_method`、
  `model_config_hash`。
- CLI 支援 `--write-db`、`--run-id`、`--pipeline-version`、`--passed`。
- `write_evaluation_result()` 將完整 report 存入 `metrics_json`，並投影 hold/break
  AUC / Brier score 到既有欄位。
- `write_evaluation_result()` 也可寫入 `sr_zone_decision_replay_p0` report；此類 report
  會完整存入 `metrics_json`，hold/break scalar 欄位維持 null。CLI
  `--decision-replay --write-db` 已允許；`--sweep --write-db` 仍禁止。
- Decision / daily confirmation / final entry 成效統計不能從 dataset label 假推；目前已
  改走 historical decision replay，逐日用「當下以前」OHLCV 重建 zone score 與
  decision summary，再把 decision fields 對齊未來 N 根 outcome。
- model path 可載入時，decision replay report 已輸出 `model_metadata`、
  `model_version`、`model_config_hash`、`model_trained_at`，並產生每個 symbol 的
  `replay_plan`（candidate bars、start/end as-of、min history、forward bars）。
- decision replay 已能產生 historical outcome rows：每列包含 `symbol`、`timeframe`、
  `as_of`、`current_price`、`forward_bars`、`forward_return`，並以
  `--replay-max-rows` 限制輸出量。
- replay rows 已補上後續 `build_decision_summary()` 需要的 candle context：
  `candle_open`、`candle_high`、`candle_low`、`candle_close`、
  `previous_candle_close`。同時新增 `zone_score_available`、`zone_score_error`、
  `decision_error`、`zone_score_fields_available` 與
  `outcome_summary.rows_with_zone_score / rows_with_decision_fields`，讓後續接
  historical ZoneScore 時可以清楚辨識目前可用層級。
- model path 可載入時，decision replay rows 已會用歷史 as-of slice 建立 zone、
  透過既有 `score_zone()` 產生 historical ZoneScore 摘要，並填入
  `zone_score_available`、`zone_count`、`primary_zone`、`zone_score_error`。
  report 的 `zone_score_fields_available` 與
  `outcome_summary.zone_score_error_counts` 會反映 ZoneScore replay 是否可用。
- replay rows 已補上 `build_decision_summary()` 所需的 global/chip context 起步：
  `global_trend`、`global_volatility`、`global_metrics` 與 `chip_summary`。
  `run_decision_replay(chip_scores_by_symbol=...)` 可依 as-of 日期取
  `trade_date <= as_of` 的最新 chip row，並重用 production `_build_chip_summary()` 格式；
  找不到資料時才以 `chip_summary.missing=true` 表示缺資料。
  `outcome_summary.rows_with_global_context / rows_with_chip_context`、
  `rows_with_non_missing_chip`、`chip_missing_rows` 用於追蹤前置 context 是否齊備。
- model、historical ZoneScore 與 global context 可用時，decision replay 已開始呼叫
  `build_decision_summary()`，並填入 `market_bias`、`daily_confirmation_state`、
  `final_entry_state`、`rr_context`、`decision_error`。report 會輸出
  `decision_replay_available`、`decision_fields_available`、
  `outcome_summary.rows_with_decision_fields`、`decision_error_counts`、
  `final_entry_state_counts`、`daily_confirmation_state_counts`。
- decision replay outcome summary 已新增 `by_final_entry_state`、
  `by_daily_confirmation_state`、`by_market_bias` 分層，每組輸出 rows、
  `average_forward_return`、`positive_forward_return_rate`、
  `negative_forward_return_rate`。
- decision replay row 已新增 `daily_confirmation_outcome`、`next_close_return`、
  `two_bar_close_return`，以 primary zone 角色做隔日 / 兩日確認結果標記：
  支撐 `SUPPORT_HELD` / `SUPPORT_BROKEN`、壓力
  `RESISTANCE_REJECTED` / `RESISTANCE_BROKEN`、兩日
  `SUPPORT_CONFIRMED` / `SUPPORT_FAILED` /
  `RESISTANCE_BREAKOUT_CONTINUATION` 等。
- decision replay outcome summary 已新增 `daily_confirmation_summary`，彙總支撐隔日守住率、
  支撐兩日確認率、壓力隔日壓回率、壓力突破率、壓力兩日突破延續率、隔日 /
  兩日平均報酬與依 `daily_confirmation_state`、`primary_role` 分層的 outcome counts。
- `daily_confirmation_summary` 已新增量能 / 事件 / RR gate 分層，包含
  `by_volume_context`、`by_event_sequence`、`by_market_event_types`、
  `by_event_market_state`、`by_rr_gate`、`by_rr_gate_reason_code`、`by_rr_bucket`，
  每組都輸出 counts、returns 與 failure distribution。
- `daily_confirmation_summary.failure_distribution` 已新增第一版失敗分布：
  `SUPPORT_CONFIRMATION_FAILED`、`SUPPORT_CONFIRMATION_OK`、
  `RESISTANCE_BREAKOUT_CONTINUED`、`RESISTANCE_REJECTION_OK`、
  `RESISTANCE_REJECTION_FAILED`、`RESISTANCE_UNRESOLVED` 等。
- decision replay outcome summary 已新增保守 `rr_summary`，只抽取穩定欄位
  `entry_rr`、`position_rr`、`entry_rr_source`、`position_rr_source`，輸出 count /
  average / median / source counts。RR bucket 與完整 distribution 尚未納入，避免過早
  固定 `rr_context` 統計 schema。
- decision replay 已依 symbol carry-forward previous event states，下一列會把上一列
  `event_state_summary.states` 傳入 `build_decision_summary(previous_event_states=...)`。
  report 會輸出 `event_lifecycle_replay_available`、
  `outcome_summary.rows_with_event_lifecycle` 與每列的 `event_state_count`、
  `active_event_count`、`resolved_event_count`、`expired_event_count`。
- `run_decision_replay(model_governance_by_symbol=...)` 可依 as-of 日期取
  `as_of` / `trade_date` / `created_at <= replay as_of` 的最新 governance snapshot，
  並傳入 `build_decision_summary(model_governance=...)`。row 會輸出
  `model_governance_available`、`model_governance_source_time`、
  `model_governance`；summary 會輸出 `rows_with_model_governance` 與
  `model_governance_missing_rows`。
- decision replay report 已新增 `governance_evaluation`
  (`sr_decision_replay_governance_evaluation_v1`)，依 replay rows、decision field
  coverage、error rate、model governance coverage、event lifecycle coverage 與
  `PROBE_ALLOWED` / `ENTRY_ALLOWED` 後續 outcome，輸出 `HEALTHY` / `DEGRADED` /
  `UNRELIABLE`、`passed`、`strict_passed`、`confidence_gate.allow_entry`、
  `max_entry_state` 與 reason codes。
- `write_evaluation_result()` 在呼叫端未指定 `passed` 時，會自動使用
  `governance_evaluation.passed` 投影到 `stock_sr_regression_results.passed`；
  手動指定 `passed=true/false` 時仍以呼叫端為準。
- migration `056_add_sr_regression_result_summary_columns` 已新增
  `schema_version`、`result_rows`、`source_count`、`governance_health_state`、
  `governance_strict_passed`，讓 regression result list 不必每次解析完整
  `metrics_json` 才能篩選/顯示 replay 治理狀態。
- Go repo 與 Python `write_evaluation_result()` 都會從 report 自動投影上述 summary
  欄位；API JSON 仍維持 `rows` / `sources` 命名，DB 欄位使用
  `result_rows` / `source_count` 以避開 SQL 保留字風險。
- CLI `--decision-replay` 已支援 `--chip-json` 與 `--model-governance-json`，
  JSON 可為 `{ "2330": [rows] }` 或 `[rows]`。傳單一 list 時會轉成 `__default__` key，
  語意是「這份 context 套用到所有 symbol」；per-symbol object 則嚴格比對，不會跨股票挪用
  （現況見 [`sr-zone-scoring.md`](./sr-zone-scoring.md) 的「Replay context 的股票比對規則」）。
- Go `POST /sr-zones/evaluate` 在 decision replay 模式下已會自動從 DB 補 historical
  chip context 與 model governance context；手動 request 若已提供
  `chip_scores_by_symbol` / `model_governance_by_symbol` 則保留呼叫端內容不覆蓋。
- Python CLI `--decision-replay --symbols` 在未提供 JSON context 時，已會自動從 DB 載入
  `chip_scores` 與 `stock_sr_model_governance` 歷史 context；`--chip-json` /
  `--model-governance-json` 仍可覆蓋來源。
- migration `055_create_sr_evaluation_jobs` 已新增 `sr_evaluation_jobs`，用來追蹤
  evaluation / decision replay 的 pending/running/done/failed 狀態與完成 report。
- `POST /sr-zones/evaluate` 已改為建立背景 job；前端可用
  `GET /sr-zones/evaluation-jobs/:job_id` 輪詢狀態，完成後從 job.report 顯示摘要。
- 前端 SR Zone 頁面已新增手動 evaluation / decision replay 控制、active job 狀態、
  最近 evaluation jobs 與最近 regression results 表格。
- backend 已新增 `sr_evaluation` config、scheduler cron 註冊與
  `POST /scheduler/sr-evaluation/run` 手動觸發；自動排程預設關閉，開啟後 symbols 空陣列
  會使用 watchlist，並共用 Go API 的 historical chip / model governance DB context 注入。
- 前端排程監控頁已新增 `sr_evaluation` job 狀態與手動執行按鈕。
- production analysis 已會讀取同一 `model_config_hash` 最新
  `sr_zone_decision_replay_p0` regression governance gate，並合併進
  `probability_context.health` / `decision.model_governance`。若最近 replay 為
  `UNRELIABLE`，正式 entry gate 會保守阻擋；若為 `DEGRADED`，最多限制到小量 /
  觀察。查無同模型 replay 或 DB 尚未有 summary 欄位時，分析維持原本模型治理邏輯。

#### T-002 P1 / P2 實作狀況 review（2026-08-04）

逐項對照計畫文字與實際程式碼（不只看自述）的結果：

**P1 — 與自述相符，DB 落地鏈路完整**。`write_evaluation_result()` 確實寫入 `run_id` /
`model_config_hash` / `pipeline_version` / `split_method` 與 056 的五個 summary 欄位；CLI 四個
旗標齊全；decision replay report 的 hold/break scalar 維持 null；Go list API 的
`schema_version` 篩選同時比對欄位與 `metrics_json->>'schema_version'`；背景 job 與前端輪詢都在。

**P0 遺留缺口：calibration bins 從未實作**。P0「核心驗證指標」明列「hold / break AUC、Brier
score、log loss、**calibration bins**」，但 `evaluation.py` 內沒有任何 calibration 相關程式碼，
`model_metrics` 只有 `rows` / `positive_rows` / `auc` / `brier_score` / `log_loss`。原本表頭把
P0 標成「已實作」與 P0 條列的「部分已實作」互相矛盾，已更正表頭。RR 分布只有保守版
（count / average / median / source counts），這點原本就有記錄。

**P2 — 機制完整可運作，但正式啟用有一個未寫下的前置條件**。治理鏈路端到端追過是通的
（Python 寫 `governance_health_state` → `fetch_latest_sr_regression_governance` 以
`governance_health_state <> ''` 篩選 → `_merge_regression_governance_gate` 合併；非 replay 的
evaluation report 不產生 governance verdict，因此不會被誤選中）。

當時的判斷是「預設 `replay_max_rows: 200` 搭配 watchlist（50～200 檔）時，股票覆蓋率必然遠低於
`MIN_REPLAY_SYMBOL_COVERAGE = 0.9`，每次排程都會產出 `DEGRADED`」。

**2026-08-06 更正：這個前提是錯的。** 那個「50～200 檔」是 CLAUDE.md 的 Scanner Scope
**Phase 1 規劃值**，不是現況。查 live DB 的實際數字：

| 項目 | 當時假設 | 實際 |
|---|---|---|
| `watchlists` 表 | 50～200 檔 | **11 檔** |
| 有 1d candles 的標的 | — | **11 檔**（與 watchlist 完全相同） |
| `replay_max_rows: 200` 攤到每檔 | ~1～4 列 | **~18 列** |

11 檔要達到 0.9 覆蓋率只需 10 檔產得出 rows，`MIN_GOVERNANCE_REPLAY_ROWS = 30` 也遠低於 200。
所以**現行預設很可能本來就夠用**，「必然 DEGRADED」的結論不成立。這一項因此從「需要重新設計
搭配」降級為「實跑一次 decision replay，看 `replay_coverage` 的實際數字再確認」——成本低很多。

**P1/P2 剩餘工作**：

1. 實跑一次 decision replay，用 `replay_coverage` 的實際數字確認現行 `replay_max_rows: 200`
   ＋ `symbols: []`（＝11 檔 watchlist）是否已足夠，並把結論寫進設定說明。
   （原本寫的是「需要重新設計搭配」，見上方 2026-08-06 的更正。）
2. RR 分布由保守版擴充為 bucket / distribution（原計畫的 Decision 層指標）。

（P0 遺留的 calibration bins 已於 2026-08-04 補齊、2026-08-05 review 通過，現況規格見
[`sr-zone-scoring.md`](./sr-zone-scoring.md) 的「Calibration bins」。）

#### T-002 / T-003 ＋ daily confirmation review 結論（2026-07-27 初審、2026-08-04 複審）

兩輪 review 都確認**方向正確、關鍵風險處理妥當**：無 lookahead bias、production governance
gate 只趨保守且安全降級、T-003 預設未變（`adaptive_zone_builders_enabled: false`）、排程預設
關閉（`sr_evaluation.enabled: false`）。這些性質的現況說明已歸檔到
[`sr-zone-scoring.md`](./sr-zone-scoring.md)（as-of 邊界、取樣規則、context 比對規則、
governance gate、evaluation 排程），並由 `tests/test_pipeline.py` 與 `scheduler_test.go` 鎖住。

2026-08-04 複審另外找出 9 筆問題（含一筆會讓 replay 只驗到第一檔股票的高嚴重度取樣缺陷、
一筆 MySQL 保留字），皆已修復並歸檔；review 通過後已從 `issue.md` 收斂（I-040 亦於 2026-08-18 內嵌至
`sr-zone-scoring.md` 後移除），僅留
（刻意保留的已知限制）。2026-08-05 review 通過後，I-049（context row 缺 `trade_date` 會拋
`KeyError`）也已收斂——現況行為記在 [`sr-zone-scoring.md`](./sr-zone-scoring.md) 的
「Replay context 的股票比對規則」。

F1（scheduler 測試）與 F2（前端元件互動測試）已完成並通過 review，條目已收斂。剩餘：

- **F3（低，optional，未處理）**：`evaluation.py` 單檔已超過 1900 行（replay / daily-confirmation /
  sweep / governance report 全擠一起），後續可拆模組（replay / outcomes / sweep / reporting）
  以利維護。屬大型重構，需另出計畫書。

狀態不動（T-002/T-003 P1/P2 仍「部分已實作」、daily confirmation 當時「已實作起步」，
後續已全部完成並收斂）；剩餘 P1/P2 續作與 F3
完成後再整體收斂歸檔。

---

### T-003：ATR zone 寬度乘數依個股特性調校

| 欄位 | 內容 |
|---|---|
| 狀態 | 進行中（P0/P1 已實作；門檻重定 2026-08-17 完成；P2 sweep 待跑） |
| 優先度 | 中 |
| 分類 | Python / SR Zone / Zone Builder |
| 建立日期 | 2026-07-07 |
| 來源 | `docs/sr-zone-scoring.md` 已知限制 |
| P0 狀態 | 已實作（2026-08-05 review 通過） |
| P1 狀態 | 已實作（2026-08-05 review 通過；計畫列的五個比較面向全數覆蓋） |
| P2 狀態 | **可續作**（機制完整、預設關閉）。原本擱置的兩個阻礙都已解除：標的池已擴到 131 檔（T-040 Step 3）、bucket 門檻已於 2026-08-17 重定（見下方「門檻重定」）。下一步是跑 coarse sweep |

`atr_width_multiplier`、`max_merge_width_multiple` 目前是全域固定預設值，
沒有依個股的波動特性（例如高波動的中小型股 vs 低波動的權值股）系統化調整。

實作計畫：

- **P0：抽出 builder config factory**（已實作，2026-08-05 review 通過）
  - 新增 `ZoneBuilderConfig` / `build_zone_builders(config)` 類型的集中入口。
  - `train.py`、`scoring.py` 與 T-002 evaluation runner 都改用同一個 factory。
  - 第一版保留目前預設：`atr_width_multiplier=1.5`、
    `max_merge_width_multiple=2.0`，只先移除硬編碼分散。
- **P1：個股波動 profile / bucket**（已實作起步，方向已 review；續作項目見下方剩餘工作）
  - 以歷史 OHLCV 計算 `ATR / close`、平均日內 range、價格級距、touch density。
  - 先分低波動 / 一般波動 / 高波動 bucket，不直接做 symbol-level override。
  - 每個 bucket 有一組候選 builder config。
- **P1：參數 sweep**（已實作，2026-08-05 review 通過）
  - 候選範圍先保守，例如 `atr_width_multiplier=1.0/1.25/1.5/1.75/2.0`、
    `max_merge_width_multiple=1.5/2.0/2.5`。
  - 用 T-002 evaluation 比較 touch 樣本量、hold/break calibration、
    `AT_ZONE` 比例、entry decision outcome 與 RR 分布。
- **P2：導入 bucket-based config**（部分已實作，方向已 review；續作項目見下方剩餘工作）
  - 低波動股票使用較窄 zone，高波動股票可放寬 zone，但仍限制 merge 避免過度糊成大區間。
  - 等 evaluation 樣本足夠後，才評估 symbol-level override。

相依性：

- P0 可先做，因為它只是抽參數入口。
- P1/P2 必須依賴 T-002 evaluation pipeline；沒有 evaluation 前不應直接調整正式預設值。

P0 已實作範圍：

- `zone_builder.py` 新增 `ZoneBuilderConfig` 與三個 builder 子 config。
- `build_zone_builders()` 集中建立 train/scoring/evaluation 使用的 builders。
- `train.py`、`scoring.py` 已改用 factory；正式預設值未調整。
- 尚未實作 symbol-level override 或正式預設導入。

P1 已實作範圍：

- `evaluation.py` CLI 已支援單次 evaluation 的 ATR builder 參數：
  `--atr-width-multiplier`、`--max-merge-width-multiple`、`--atr-lookback`、`--atr-period`。
- `run_builder_sweep()` / CLI `--sweep` 已支援保守 grid sweep：
  `--atr-width-grid`、`--max-merge-width-grid`，輸出每組候選的 zone outcome、
  model metrics、builder config 與 `best_by` 摘要。
- `run_evaluation()` 已輸出 `volatility_profiles`，包含 `atr_pct`、
  `average_range_pct`、`touch_density_per_100_bars` 與
  `LOW_VOLATILITY` / `NORMAL_VOLATILITY` / `HIGH_VOLATILITY` bucket。
- `zone_outcomes` 已新增 `by_volatility_bucket` 分層，sweep candidate 也會帶出同樣
  bucket outcome，供後續比較不同 ATR 參數在不同波動股票上的表現。
- `run_builder_sweep()` 已輸出 `recommended_configs_by_bucket`，依各 volatility bucket
  的 hold rate 與 average forward return 做保守排序；樣本不足時標記
  `insufficient_sample=true`，不硬給正式建議。
- sweep 目前只輸出 JSON，不寫入 `stock_sr_regression_results`；該資料表仍保留給單一
  evaluation/regression run。
- 這些參數只影響當次 evaluation，不改 train/scoring 正式預設。
- bucket recommendation 目前仍是 evaluation 參考輸出，不會改寫正式
  `build_zone_builders()` 預設；production scoring 只在明確開啟
  `SR_SCORING_ADAPTIVE_ZONE_BUILDERS_ENABLED` 時套用 bucket config。

P2 已實作範圍：

- `zone_builder.py` 新增 `volatility_bucket_from_profile()` 與
  `resolve_zone_builder_config_for_profile()`，把 LOW/NORMAL/HIGH volatility bucket
  對應到候選 ATR width / merge width config。
- `scoring.py` 新增個股 runtime profile helper，使用近 60 根 K 棒的 `ATR / close`
  與平均日內 range 作為 bucket 依據。
- `pipeline.py` 在未明確傳入 builders 時，會依
  `SR_SCORING_ADAPTIVE_ZONE_BUILDERS_ENABLED` 決定使用固定預設或 bucket config；
  目前 `python/config.yaml` 預設 `adaptive_zone_builders_enabled: false`，所以正式
  scoring 行為不會自動改變。
- analysis payload 新增 `zone_builder_runtime_config`，可檢查本次是否啟用 adaptive、
  使用哪個 bucket、實際套用的 builder config 與原因碼。
- 尚未完成：依 T-002 regression/sweep 結果決定是否預設啟用、調整 bucket 門檻、
  或加入 symbol-level override。

> 2026-07-27 review：P0（抽 config、預設未變）與 P2 adaptive flag（預設關閉、production scoring 不變）
> 方向確認無誤，見 T-002 的「review 結論」段落。

#### T-003 P1 / P2 實作狀況 review（2026-08-04 複審、2026-08-05 收斂）

已驗證完成：CLI 的 `--atr-width-multiplier` / `--max-merge-width-multiple` / `--atr-lookback` /
`--atr-period`；`--sweep` 與兩個 grid，且 `--sweep` 與 `--write-db`、`--decision-replay` 互斥；
`volatility_profiles`（`atr_pct` / `average_range_pct` / `touch_density_per_100_bars` + 三個
bucket）；`zone_outcomes.by_volatility_bucket`；`recommended_configs_by_bucket` 與
`insufficient_sample`。

2026-08-04 複審當下，P1 計畫列的五個比較面向只覆蓋 2/5：`run_builder_sweep()` 呼叫的是
`run_evaluation` 而非 `run_decision_replay`，candidate report 裡沒有 decision / RR 欄位；
calibration bins 從未實作；`AT_ZONE` 比例被判為量不到。**三個缺口都已於 2026-08-04 補齊、
2026-08-05 review 通過（python 319 passed），五個面向至此全數覆蓋。** 其中 `AT_ZONE` 的原
結論只對 evaluation dataset 成立（該處 `role` 由 approach direction 二選一決定），replay 路徑
的 `primary_zone.role` 是量得到的。現況規格見 [`sr-zone-scoring.md`](./sr-zone-scoring.md)
的「Decision Replay 的 zone builder 參數」、「Calibration bins」與「參數 sweep 的 decision
層比較」三節。

**P2 的機制齊全，但決策依據還沒取樣**。`volatility_bucket_from_profile` /
`resolve_zone_builder_config_for_profile` / `scoring.py` runtime profile / `pipeline.py` flag
gate / `zone_builder_runtime_config` 都已驗證，預設關閉也確認過。P2 的出口條件是「等
evaluation 樣本足夠後才評估是否導入」——工具現在齊了，但**還沒實際跑過一次 sweep 取樣**。

#### P2 的結論（2026-08-06 實跑 sweep 後）

sweep 已於 2026-08-06 實際跑過（11 檔 watchlist、`--limit 1500`、grid 3×2 共 6 組，
完整數據與判讀見 [`sr-zone-scoring.md`](./sr-zone-scoring.md) 的
「2026-08-06 首次實跑 sweep 的結論」）。**結論與原本的預期相反：卡住 P2 的不是「還沒跑 sweep」，
是標的池太窄。**

| 發現 | 數字 |
|---|---|
| bucket 分佈 | HIGH **9 檔**／NORMAL 2 檔／LOW **0 檔** |
| zone 層候選差異 | 支撐守住全距 1.76pp、壓力壓回 0.73pp、突破 1.08pp；**四個指標選出四個不同贏家** |
| HIGH bucket 的建議 score 全距 | **0.0056**（前三名相差 0.0004）→ 在雜訊中排序 |
| NORMAL bucket | 全距 0.0244、偏好方向與 HIGH 相反，但**只有 2 檔**，無法歸因 |

**因此三件原本要靠 sweep 決定的事，現在的答案都是「資料不足以決定」**：

1. **bucket 門檻**：不調。LOW bucket 沒有任何標的，調門檻沒有資料可驗證。
2. **是否預設啟用 adaptive builder**：**維持關閉**。9/11 落在 HIGH 表示分組對現有標的池
   幾乎不分辨；HIGH 的候選差異又在雜訊內，啟用與否的預期差異接近零。
3. **symbol-level override**：更不用談，樣本比 bucket-level 還少。

**P2 要往前走，需要的是擴標的池**（更多 NORMAL / LOW 波動的標的），不是更密的參數網格或
更多次 sweep。在 11 檔、9 檔擠在 HIGH 的情況下，再跑幾次都會得到同樣的雜訊。
因此 P2 狀態改為**擱置**——機制完整、預設關閉是安全的現況，等標的池擴大後再重跑一次 sweep 即可。

#### 門檻重定：**已實作（2026-08-17）**，P2 的前置已解除

**「等標的池擴大後再重跑 sweep」這句話不夠——擴大之後才發現真正的阻礙是門檻本身。**
2026-08-17 已依裁決重定並實作完成，現況規格歸檔在
[`sr-zone-scoring.md`](./sr-zone-scoring.md)「Volatility bucket 門檻＝凍結的全市場分位數」。

| | LOW | NORMAL | HIGH |
|---|---|---|---|
| 舊絕對門檻 | `< 1.5%` | 1.5–3.5% | `> 3.5%` |
| **重定後（凍結分位數）** | **`< 4.6089927%`** | 4.609–6.278% | **`> 6.2781977%`** |
| 選池 131 檔的分佈 | 1 → **53** | 26 → **46** | 103 → **32** |

原本的問題是**選池怎麼挑都沒用**：用分位數選出三桶均衡的 universe，但同一批標的用
pipeline 的絕對門檻分類是 103 / 26 / 1，LOW 只剩一檔，`VOLATILITY_BUCKET_ATR_CONFIGS`
的 LOW 那組永遠不會被觸發——**與 2026-08-06 sweep 時 LOW 為 0 檔沒有實質差別**。

**實作時發現的關鍵不一致（原計畫沒預見）**：`volatility_bucket_from_profile` 的判定基準是
`max(atr_pct, average_range_pct)`，但 selection report 的分位數只用 `atr_pct`。
319 檔裡 **156 檔（49%）的 `average_range_pct` 比 `atr_pct` 大**，兩種基準會讓 131 檔中的
20 檔分到不同 bucket。**只換常數不換基準，重定完仍然對不上。**
已讓 report 新增 `bucket_basis()` 與 pipeline 同源，並用同基準重算切點。

**驗收**：定案 131 檔的 `selection_bucket`（分位數重算）與 `current_bucket`（絕對門檻）
**131/131 完全一致**。python 測試 499 passed。

**既有資料不重算**（本次裁決）：`stock_sr_zone_analyses` 在 2026-08-17 之前的列，其 bucket
語意是舊門檻。沿用 `database-schema.md`「股價還原」已立的原則——分析紀錄是「當時做了什麼判斷」，
不是快取。分界日與注意事項已寫進 `sr-zone-scoring.md`。

**邊界凍結一併完成**：新常數**就是**凍結的邊界，重新取分位數＝改這兩個常數並升
`universe_version`，是一次明確的版本動作而非每日漂移。`VOLATILITY_THRESHOLD_PROVENANCE`
記下量測條件（母體、基準、分位數、工具）供重現。

**P2 的下一步**：門檻已不再是阻礙，可以跑 coarse sweep 了。三桶分別有 53 / 46 / 32 檔，
都遠超 `MIN_BUCKET_RECOMMENDATION_ROWS`。判讀時要帶上下方「HIGH bucket 天生帶半導體業偏斜」
這個前提。

**不做的事（明確記下來，避免日後誤動）**：不因這次結果調整 `build_zone_builders()` 的預設值。
score 全距 0.0056 不足以支撐任何調整；`recommended_configs_by_bucket` 的
`insufficient_sample=false` 只保證樣本數夠，不保證候選之間有可分辨的差異。

##### 判讀前提：HIGH bucket 的結論天生帶半導體業偏斜

T-040 選池定案時已裁決**接受 HIGH bucket 填不滿**（bucket 名額 30、含 watchlist 共 33，
低於 `per_bucket_min=35`），因為那是母體事實而非演算法問題：HIGH 的 90 個候選裡
**81 檔是半導體業（90%）**，其餘 14 個產業各只有 1～4 檔，產業上限 11 之下理論上限只有 36。
台股「高波動且流動性足」幾乎等於半導體。

**所以跨 bucket 比較時，HIGH 那一組的差異有多少來自「高波動」、有多少來自「半導體業」
是分不開的。** 不要把 HIGH 的勝出候選直接當成「高波動股票適用的參數」。
完整理由與被否決的兩個替代方案見
[`evaluation-universe-selection-plan.md`](./evaluation-universe-selection-plan.md)
「三個設計決定」的第二點。

##### 併入項目：bucket 邊界必須凍結進 universe artifact（2026-08-17，來自 T-040 階段 4 驗證）

**分位數邊界是相對於當下母體的，母體一動邊界就漂——bucket 不是標的的固有屬性。**

deep backfill 完成後重跑 selection report，與回補前的 `universe-v2.json` 比對，實測到：

| 觀察 | 數字 |
|---|---|
| 全體 857 檔 bucket 變動 | 9 檔 |
| 其中在 130 檔選池內 | 6 檔 |
| **其中這次完全沒重抓、`atr_pct` 一個 bit 都沒變** | **3 檔**（3530、3661、8102） |

邊界移動量：LOW/NORMAL `0.0424759 → 0.0417790`（-1.64%）、NORMAL/HIGH `0.0604097 → 0.0605084`（+0.16%）。
3530（`atr_pct` 固定 0.042139）、3661（0.060433）、8102（0.041858）三檔的值完全未變卻跳桶，
**直接證明變動來自邊界而非標的本身**。

風險放大在於邊界附近很擠：**選池 130 檔中有 18 檔（14%）距離最近邊界不到 2%**，
其中 2376、6405 的相對距離是 0.00%——就壓在線上。後果有兩層：

1. 選池刻意經營的 LOW 45 / NORMAL 45 / HIGH 29 配比會隨每日資料進來自己劣化，壓線的十幾檔來回跳。
2. 跨期比較失真：下次 evaluation 結果與這次不同時，**分不清是策略改了還是 bucket 定義改了**。

**做法**：selection report 已輸出 `quantile_edges`，選池定案時把該值凍結進 universe artifact，
下游 evaluation 依凍結邊界分桶而不是每次重算。重新取分位數改成明確的版本升級動作
（universe v2 → v3），不是每天偷偷發生的副作用。

這與上面「重定絕對門檻」是同一件事的兩面，**必須一起規劃**：絕對門檻決定 bucket 的語意，
凍結機制決定該語意在時間軸上是否穩定。只做前者，重定完的門檻一樣會被下一批資料推著跑。

> 附帶結論：階段 4 原本寫的驗證判準「5 年資料到位後 bucket 不該變」**是錯的**，已依此實測修正，
> 見 `docs/evaluation-universe-selection-plan.md` 的階段 4。

---

### T-004：籌碼分析 Phase 2 擴充指標

| 欄位 | 內容 |
|---|---|
| 狀態 | 擱置 |
| 優先度 | 低 |
| 分類 | Go / 籌碼分析 |
| 建立日期 | 2026-07-07 |
| 來源 | `docs/chip-analysis-design.md` |

股權分散表、董監持股、借券/當沖比、大戶散戶持股比例。設計文件明確標註
「Phase 2 再評估，不應阻塞 Phase 1」，目前 Phase 1（三大法人、融資融券、
分點、綜合籌碼分數）已完整上線。

---

### T-005：Fugle 即時行情盤中更新訊息格式驗證

| 欄位 | 內容 |
|---|---|
| 狀態 | 進行中 |
| 優先度 | 中 |
| 分類 | Go / 即時行情 |
| 建立日期 | 2026-07-07 |
| 來源 | `docs/fugle-integration.md` Roadmap |

目前 Fugle WebSocket 盤中更新的訊息格式解析未在實盤交易時段實際驗證過，
需要在開盤時段跑一次確認欄位、頻率、斷線重連行為符合預期。

註：盤中源相關工作暫不列入近期處理；目前先沿用既有資料流程，等後續有更合適
的盤中資料源或明確需求時再重新評估。

---

### T-006：Fugle Tier 1 REST 輪詢掃描接上排程器

| 欄位 | 內容 |
|---|---|
| 狀態 | 擱置 |
| 優先度 | 低 |
| 分類 | Go / 即時行情 / 排程 |
| 建立日期 | 2026-07-07 |
| 來源 | `docs/fugle-integration.md` Roadmap、`docs/architecture.md` |

Tier 1（非熱門股）用 REST 輪詢掃描的機制已設計但尚未實際掛上排程器
（`internal/scheduler`）自動執行。

註：Yahoo 盤中源為另一個可作為 Tier-1 廣度掃描的選項，且支援單次批次多檔，兩者擇一或並列。
Yahoo 的 client／設定／排程批次路徑已實作（見 `docs/yahoo-intraday-integration.md`），僅剩
fallback（T-031）與實盤驗證（T-032）待處理。

註：盤中源相關工作暫不列入近期處理；目前先沿用既有資料流程，等後續有更合適
的盤中資料源或明確需求時再重新評估。

---

### T-007：Fugle Tier 2 熱門股 WebSocket 訂閱管理

| 欄位 | 內容 |
|---|---|
| 狀態 | 擱置 |
| 優先度 | 低 |
| 分類 | Go / 即時行情 |
| 建立日期 | 2026-07-07 |
| 來源 | `docs/fugle-integration.md` Roadmap |

Tier 2（熱門股）動態訂閱/取消訂閱 WebSocket 頻道的管理邏輯尚未實作
（目前只有靜態 client，見 `docs/CLAUDE.md` 提到的重複連線問題修正）。

註：盤中源相關工作暫不列入近期處理；目前先沿用既有資料流程，等後續有更合適
的盤中資料源或明確需求時再重新評估。

---

### T-008：Fugle → FinMind 自動 fallback

| 欄位 | 內容 |
|---|---|
| 狀態 | 擱置 |
| 優先度 | 低 |
| 分類 | Go / 即時行情 |
| 建立日期 | 2026-07-07 |
| 來源 | `docs/fugle-integration.md` Roadmap |

Fugle 連線失敗或資料異常時，尚未實作自動切換回 FinMind 補資料的邏輯，
目前需要人工介入。

註：與 T-031（Yahoo→FinMind fallback）共用「盤中源異常時回退 FinMind」的設計，
應規劃為單一通用的盤中源 fallback 機制，而非每個源各寫一套。

盤中源相關工作暫不列入近期處理；目前先沿用既有資料流程，等後續有更合適
的盤中資料源或明確需求時再重新評估。

---

### T-009：導入 Shioaji tick-level streaming 取代批次量能計算

| 欄位 | 內容 |
|---|---|
| 狀態 | 擱置 |
| 優先度 | 低 |
| 分類 | Go / 即時行情 / Phase 2 |
| 建立日期 | 2026-07-07 |
| 來源 | `docs/architecture.md`、`CLAUDE.md` Phase 2 Roadmap |

目前量能計算是批次（K棒收盤後）而非 tick-level 即時累加，`CLAUDE.md`
Roadmap 中列為 Phase 2（Shioaji 整合）項目，非近期規劃。

---

### T-011：多檔回測改為共用資金的投資組合回測

| 欄位 | 內容 |
|---|---|
| 狀態 | 待規劃 |
| 優先度 | 中 |
| 分類 | Python / 回測引擎 |
| 建立日期 | 2026-07-07 |
| 來源 | `docs/backtest-modular-strategy.md` 已知限制 |

目前多檔股票回測是每檔獨立跑（各自有自己的模擬資金），不是真正共用同一筆
資金池、會互相排擠部位的投資組合回測。

---

### T-012：Volume Profile 改用盤中 tick 資料

| 欄位 | 內容 |
|---|---|
| 狀態 | 待規劃 |
| 優先度 | 低 |
| 分類 | Python / 回測引擎 / SR Zone |
| 建立日期 | 2026-07-07 |
| 來源 | `docs/backtest-modular-strategy.md` 已知限制 |

目前 Volume Profile 用單一 typical price `(H+L+C)/3` 近似整根K棒的成交
分布，沒有盤中 tick 資料可用時的權宜做法。Shioaji tick 資料到位後
（見 T-009）可以改成更精確的價位分布。

---

### T-013：CLAUDE.md Roadmap 長期項目彙整

| 欄位 | 內容 |
|---|---|
| 狀態 | 擱置 |
| 優先度 | 低 |
| 分類 | 架構 / 長期規劃 |
| 建立日期 | 2026-07-07 |
| 來源 | `CLAUDE.md` Roadmap Phase 2~4 |

`CLAUDE.md` 定義的長期方向，目前都還沒排入近期工作：

- Phase 2：Dashboard 升級
- Phase 3：Portfolio tracking、Position management、Strategy templates
- Phase 4：Semi-auto execution（optional）、Risk engine enhancement

（Phase 2 的 Shioaji 整合已拆成 T-009 個別追蹤。）

---

### T-017：Watching 進場點升級為機率模型

| 欄位 | 內容 |
|---|---|
| 狀態 | 待規劃 |
| 優先度 | 低 |
| 分類 | Python / stock-analysis / 模型 |
| 建立日期 | 2026-07-07 |
| 來源 | 原 `docs/issue.md` I-010，2026-07-07 分流至 todo |

個股分析的「Watching」（觀察中）進場點目前是規則式近似（`analysis.py`
`_watching_entry`）：純用「趨勢方向 + 離現價最近的支撐/壓力價位」挑一個該盯的
價位，沒有機率、期望值或風險報酬比。相對地，隔壁 SR Zone 已經是訓練過的機率
模型，會輸出 bounce/break probability。這筆是把 Watching 也升級成機率模型的規劃
（屬功能擴充，不是 bug）。

實作範式可**直接類比 `sr_scoring/` 既有管線**（兩者是獨立系統、獨立資料表，不共用
模型），需要新增對應元件：

- **Label 定義**：進場後 N 根K棒內是否達到目標／觸及停損（可複用
  `labeling.py::label_touch` 的「forward window + threshold」自動標籤範式，免人工標註）。
- **特徵工程**：以現有規則式指標（趨勢、S/R 距離、爆量比、ATR、pullback 容忍帶等）
  為基礎的特徵向量。
- **walk-forward dataset**：類比 `dataset.py`，逐根用「至今」資料算特徵 + 未來 label。
- **train/predict + 機率校準**：類比 `model.py`（time-split holdout、`CalibratedClassifierCV`）。
- **模型檔管理**：類比 SR Zone 的 joblib lazy singleton。

可用資料已具備：`candles` 歷史、`backtest_trades` 逐筆交易結果、SR Zone 的自動標籤
與 walk-forward 範式；缺的是專屬 Watching 的 label 定義與特徵 pipeline，而非資料本身。

---

### T-031：runIntradayBatch 批次失敗時的 Yahoo→FinMind fallback

| 欄位 | 內容 |
|---|---|
| 狀態 | 擱置 |
| 優先度 | 低 |
| 分類 | Go / 即時行情 / 排程 |
| 建立日期 | 2026-07-15 |
| 來源 | `docs/yahoo-intraday-integration.md` |

Yahoo 盤中源的 client、設定、main 組裝、`scheduler.runIntradayJob → runIntradayBatch` 批次路徑
**皆已實作**（現況見 `docs/yahoo-intraday-integration.md`）。**剩餘唯一工作**：批次請求失敗
（Yahoo 被限流/封鎖）時回退補資料——目前 `scheduler.go` 的 `runIntradayBatch` 只記 log 續跑其他批次
（見該處 TODO 註解），未回退 FinMind。

設計取捨（本次已確認）：

- 僅 `finmind.intraday_enabled=true` 時才回退逐檔 FinMind 分K；`intraday_enabled=false`（預設，無
  Sponsor token）時**不回退**——FinMind 分K（TaiwanStockKBar）注定 422/tier 不足，回退只會徒耗額度。
- 回退時比照現有 `ErrInsufficientTier` 邏輯：撞到 tier 不足就整輪跳過，不對每檔重打注定失敗的請求。
- 與 T-008（Fugle→FinMind fallback）共用「盤中源異常時回退」的單一通用設計，避免各源各寫一套。

註：盤中源相關工作暫不列入近期處理；目前先沿用既有資料流程，等後續有更合適
的盤中資料源或明確需求時再重新評估。

---

### T-032：Yahoo 盤中源實盤時段驗證

| 欄位 | 內容 |
|---|---|
| 狀態 | 擱置 |
| 優先度 | 低 |
| 分類 | Go / 即時行情 / 驗證 |
| 建立日期 | 2026-07-15 |
| 來源 | `docs/yahoo-intraday-integration.md` 風險與限制 |

Yahoo 為非官方 API，上線前須於台股盤中時段（09:00–13:30）用 `cmd/yahoo-check` 實測：

- minute 陣列覆蓋率：確認 `null` 僅出現在盤前/盤後，而非 ETF（如 `0050.TW`）系統性缺值——實測盤後 `0050` 陣列全為 null 但 `2330` 正常，需釐清成因。
- 延遲：`quote.refreshedTs` vs 本地時間差。
- 封鎖風險：連續批次請求是否觸發反爬/限流，據以定 `rate_limit`/`batch_size`。

註：盤中源相關工作暫不列入近期處理；目前先沿用既有資料流程，等後續有更合適
的盤中資料源或明確需求時再重新評估。

---

### T-038：`http_server` 其餘端點的測試

| 欄位 | 內容 |
|---|---|
| 狀態 | **已實作，待 review**（2026-08-18：七個端點全部補完，新增 28 支測試） |
| 優先度 | 低 |
| 分類 | Python / 測試 |
| 建立日期 | 2026-08-06 |
| 來源 | `http_server` 可測性重構（2026-08-06） |

`http_server.py` 的 DB 連線檢查已移出 import 期改到 FastAPI lifespan（見
[`development-workflow.md`](./development-workflow.md) §4），`python/tests/` 因此可以用
FastAPI TestClient，但當時只補了 `/sr-scoring/evaluate`。仍無測試的端點：

- `/analyze`（`ValueError` → 404 映射）
- `/sr-zones`（404 / 503 兩種映射、`previous_event_states` 轉發）
- `/sr-scoring/train`（`ValueError` → 400、六個參數轉發）
- `/sr-scoring/model-status`（模型不存在時**刻意回 200 + `exists: false`** 而非 503，是容易被
  「順手改成 503」破壞的設計）
- `/backtest`（`symbols` 接受 JSON string 的 `field_validator`、background task 不阻塞回應）
- `/backtest/{job_id}`（查無 job → 404；需 monkeypatch `engine`）
- `/health`

分檔慣例比照 `tests/test_http_server_sr_evaluate_*.py`：一個測試範圍一支檔，不堆進同一個檔案。

#### 實作結果（2026-08-18）

七個端點各一支檔，共 **28 支測試**，全部走 conftest 既有的 `client` fixture
（不用 `with`，所以不跑 lifespan、不連 DB）：

| 檔案 | 支數 | 鎖住的重點 |
|---|---|---|
| `test_http_server_analyze.py` | 4 | 三參數轉發、`ValueError` → 404、缺 symbol → 422 |
| `test_http_server_sr_zones.py` | 4 | 404 **與** 503 兩條分支、`previous_event_states` 省略時是**空 list 而非 None** |
| `test_http_server_train.py` | 4 | 六參數轉發（都取非預設值）、`calibration_method: null` 不被當成沒給、`ValueError` → **400** |
| `test_http_server_model_status.py` | 3 | 模型不存在回 **200 + `exists:false`**（測試明寫 `!= 503`）、其餘欄位為 None、`config_hash` 有帶出 |
| `test_http_server_backtest_submit.py` | 6 | 202、`symbols` 收 JSON string 與 list、回應只有 `{job_id, status}` |
| `test_http_server_backtest_get.py` | 5 | 查無 → 404 且不再查 results、`trigger_source` → `trigger` 改名、result 未寫入時為 null |
| `test_http_server_health.py` | 2 | `{"status":"ok"}`、**把 engine 換成一碰就爆也仍是 200**（liveness 不該依賴 DB） |

**沒能驗到的一項要說清楚**：TestClient 會在回應產生後、`client.post()` 回來前執行
background task，所以 `/backtest` 的「不阻塞」**無法用時序證明**。測試鎖的是
「工作有排進背景、回應不含執行結果」；真正的不阻塞來自 `BackgroundTasks.add_task`
而不是 `await`，那是結構上的事實。這一點寫在該檔的 docstring 裡。

---

### T-039：SR Zone 調參與決策入口沒有前端，卡住 T-002 / T-003 的收尾

| 欄位 | 內容 |
|---|---|
| 狀態 | 待規劃 |
| 優先度 | 中 |
| 分類 | Frontend / Go / Python / SR Zone |
| 建立日期 | 2026-08-06 |
| 來源 | T-002 / T-003 完成度盤點（2026-08-06） |

2026-08-06 對照程式碼盤點 T-002 / T-003 的結果：**觀察與驗證的入口已經很完整**（跑 evaluation、
看三層指標、看波動側寫、看這次用了哪組 builder、看模型治理狀態），**但調參與決策的入口幾乎都
不在前端**。這正好解釋了為什麼兩項都停在「機制齊全、就差實際跑一次取樣然後定案」——那一步
目前只能用 CLI 做。

**已有前端入口**（不需再做）：手動 evaluation / decision replay 表單（模式、symbols、limit、
`replay_max_rows`、四個 ATR 參數、寫入 DB）、evaluation job 輪詢與最近 jobs、regression results
表格、report 三層指標 / daily confirmation 摘要 / warnings / `volatility_profiles` /
`zone_builder_runtime_config`、`sr_evaluation` 排程狀態與手動執行、production 分析的模型治理區塊。

（daily confirmation 的分層渲染屬**顯示**缺口，已於 2026-08-06～08-07 完成，
現況見 [`sr-zone-scoring.md`](./sr-zone-scoring.md) 的「隔日／兩日確認的分層」，
不在本筆範圍。本筆只談**調參與決策**入口。）

**四個缺口**（依擋路程度排序）：

**A：參數 sweep 沒有 API 與 UI（最擋路）**

Python 有 `run_builder_sweep()` 與 CLI `--sweep`，但沒有 HTTP 端點、沒有 Go 路由、沒有 UI
（現況已記在 [`sr-zone-scoring.md`](./sr-zone-scoring.md) 的「參數 sweep 的 decision 層比較」）。
**T-003 P2 的最後一步就是實跑 sweep 取樣**，唯一路徑是進 container 敲 CLI。sweep 會對每組候選
各跑一次 evaluation（開 decision replay 時再加一次 replay），耗時遠超一般 API 延遲，要做就得比照
`/sr-zones/evaluate` 走背景 job（`sr_evaluation_jobs` 或另開表），不能同步回應。

決定要不要做之前，建議**先用 CLI 跑一次**——跑完才知道多久會想再跑第二次。只跑一兩次就定案的話，
API + UI 不划算。

**B：adaptive builder 開關只在 Python 設定**

`SR_SCORING_ADAPTIVE_ZONE_BUILDERS_ENABLED` 只存在於 `python/config.yaml` 與環境變數；
`backend/internal` 與 `frontend/src` 完全沒有這個概念（已 grep 確認）。要開關得改設定並重啟
python-server。T-003 P2 若決定預設啟用，這個開關的可觀測性與可切換性值得一併考慮——目前前端
只看得到「這次分析有沒有生效」（`zone_builder_runtime_config.reason_code`），看不到「系統設定是
開還是關」。

**C：排程參數不能從前端調**

`Scheduler.svelte` 只有觸發按鈕，零個輸入欄位。`sr_evaluation` 的 `enabled` / `cron` / `symbols` /
`replay_max_rows` 只能改 `backend/config.yaml` 或環境變數再重啟 backend，調參要重啟、試錯成本高。

不過**這一項的急迫性在 2026-08-06 下降了**：原本的理由是 T-002 P2 需要反覆試 `replay_max_rows`
與 `symbols` 的搭配，但查證後 watchlist 實際只有 11 檔（不是記載的 50～200 檔），現行預設很可能
本來就夠用，不需要反覆調參（見 T-002 該段的更正）。

注意這不只是 SR Zone 的問題：排程頁對所有 job 都只能觸發、不能調參。要做的話應該先決定
「排程設定要不要落 DB」這個更大的方向，不要只為 `sr_evaluation` 開特例。

**D：volatility bucket 門檻是 module 常數**

`LOW_VOLATILITY_THRESHOLD = 0.015` / `HIGH_VOLATILITY_THRESHOLD = 0.035`
（`python/backtest/modular/sr_scoring/zone_builder.py:43-44`）連 config 都不是。T-003 P2 說要
「依 sweep 結果調整 bucket 門檻」，目前只能改 code。優先度最低——門檻改動頻率本來就低，
且改了會影響 production scoring，走 code review 反而比較安全。

**相依與建議順序**

1. ~~先用 CLI 跑一次 sweep~~ **已於 2026-08-06 執行完畢**，執行方案與結果見下節。
2. **A（sweep 的 API + UI）：判定現在不做。** 實跑的體感是「6 組約 7 分鐘、跑兩次就得到結論」，
   而結論是**在標的池擴大之前不需要再跑**。為一個短期內不會再用的操作蓋背景 job ＋ UI 不划算。
   等標的池擴大、真的要反覆比較參數時再回來評估。
3. **B / D：判定現在不做。** 兩者都是為了「adaptive builder 要調整／要切換」而存在，
   而 T-003 P2 的結論是**維持關閉且短期內不會改**（見該筆的「P2 的結論」）。
4. **C（排程參數不能從前端調）：維持記錄，優先度低。** 它原本的急迫性來自 T-002 P2 需要反覆
   試參數，但那個前提也已更正（watchlist 實際 11 檔，現行預設很可能夠用）。

**本筆的現況**：四個缺口全部確認為「真實存在但現在都不急」。**本筆不刪除**——缺口本身沒有消失，
只是觸發它們的需求還沒到。等標的池擴大後，A / B / D 會一起重新變得有意義。

#### 執行方案：sweep 取樣（2026-08-06，待確認）

| 欄位 | 內容 |
|---|---|
| 狀態 | **已執行完畢（2026-08-06）**：Pass 0 ✅、Pass 1 ✅、Pass 2 依結論略過 |
| 性質 | **只跑不寫**：不改任何程式碼、不改任何預設值、不寫任何資料表 |
| 產出 | JSON report，供 T-003 P2 決定 bucket 門檻與是否預設啟用 adaptive builder |
| 結論 | **資料不足以支撐決策**——卡住的是標的池（11 檔、9 檔擠在 HIGH、LOW 空白），不是參數。見 T-003 的「P2 的結論」 |

**資料現況（2026-08-06 查 live DB，決定了整個設計）**

| 項目 | 實際 |
|---|---|
| watchlist / 有 1d candles 的標的 | **11 檔**（完全重疊） |
| 各檔根數 | `2454`／`2330`／`0050` 各 ~4,865（2006 起）；`2399`／`5490`／`3630`／`6243`／`2478` 各 ~2,400（2016 起）；`00830` 1,768；`00947` 523；`00981A` 293 |
| `chip_scores` | 9,989 列 / 11 檔 → decision replay 的 chip context 齊全 |
| `stock_sr_model_governance` | **只有 9 列 / 2 檔** → governance context 很薄，replay 的 governance 分層會大量 missing |
| 模型 | `/opt/stacks/scripts/stock_trading/python/models/sr_scoring_v4.joblib`，version `v4`、15 特徵、2026-08-06 訓練，與 `MODEL_VERSION = "v4"` 相容（已實際載入確認） |

**執行環境**

- **一次性 container**，`--network trading-net` 連 live postgres，掛本 repo 的 `python/` 與
  live 的模型檔（world-readable，已確認可讀）。
- **不用 live 的 `python-server` container 跑**：那是 production 容器、`mem_limit` 512m，
  sweep 會跟線上服務搶記憶體。
- **不用 dev project**：dev postgres 的 `candles` 是**空的**（已確認），跑不出任何東西。
  CLAUDE.md 要求用 dev project 的規則是針對「驗收開發成果／migration／測試資料」，
  這裡是唯讀的研究性查詢；而且 `--sweep` 在 CLI 層就禁止 `--write-db`
  （`evaluation.py:2167`），**結構上不可能寫到 live DB**。

**Pass 0：先診斷，不要直接 sweep**

`MIN_BUCKET_RECOMMENDATION_ROWS = 20`。11 檔很可能有 bucket 只落到 1～2 檔，touch 數不足 20，
那麼 sweep 的 `recommended_configs_by_bucket` 會全部標 `insufficient_sample`、給不出建議——
**跑 6～15 組等於白跑**。所以先花 1 次的成本確認母體：

- 單次 `run_evaluation`（**不加** `--sweep`），11 檔、`--limit 1500`、帶 `--model-path`。
- 只看兩件事：`volatility_profiles`（11 檔各落在哪個 bucket）與
  `zone_outcomes.by_volatility_bucket`（各 bucket 幾筆）。
- **決策點**：若有 bucket < 20 筆 → 現有資料量產不出 bucket 建議，該做的是**擴 watchlist**
  而不是跑 sweep，本方案就此打住。
- 順便量這一次的實際記憶體峰值，作為 pass 1 網格大小的依據。

**Pass 0 結果（2026-08-06 已執行）**

11 檔全數納入、**5,928 筆 touch**、資料區間 2020-05-31 → 2026-08-05、無 warnings。
模型 v4 可用：**hold AUC 0.842 / break AUC 0.833**。container 記憶體 350m 一次過關
（第二次重跑用 270m 也過）。

*bucket 分佈——這是主要發現*：

| bucket | 檔數 | rows |
|---|---|---|
| `HIGH_VOLATILITY` | **9 檔** | 4,676 |
| `NORMAL_VOLATILITY` | 2 檔（`0050`、`2330`） | 1,252 |
| `LOW_VOLATILITY` | **0 檔** | **完全沒有** |

門檻是 `LOW ≤ 1.5%` / `HIGH ≥ 3.5%`，而實際 ATR% 從 `2330`／`0050` 的 3.2% 一路到 `6243` 的
**11.6%**。所以 **9/11 落在 HIGH**，`0050`／`2330` 離 HIGH 門檻只差 0.3 個百分點，
**LOW bucket 永遠不會被觸發、也永遠無法用資料驗證**。

*兩個 bucket 的通過條件都滿足*：`MIN_BUCKET_RECOMMENDATION_ROWS = 20`，HIGH 4,676 / NORMAL 1,252
都遠超過，所以 sweep 產得出這兩組的建議——但**永遠產不出 LOW 的建議**。

*Pass 0 順帶抓到一個 bug*：`zone_outcomes` 三種分層的比率欄位在前端永遠顯示 `—`
（欄位名不一致），已於同日修復；現況見 [`sr-zone-scoring.md`](./sr-zone-scoring.md) 的
「Zone 層分層的欄位語意」。**修好後才有下表**，
而這正是 Pass 1 要比較的維度——若沒先跑 Pass 0，Pass 1 會在只剩 `average_forward_return`
一個維度可比的情況下跑完，結論不可靠。

修復後的 bucket 分層數字：

| bucket | rows | 支撐守住 | 壓力壓回 | 突破 |
|---|---|---|---|---|
| HIGH | 4,676 | 45.3% | 34.1% | 43.8% |
| NORMAL | 1,252 | 33.7% | 22.6% | 27.2% |

兩組差距不小，對 T-003 P2 是**正面訊號**——波動分組確實對應到不同的 zone 行為。
但**不能就此歸因於波動**：NORMAL 只有 `0050` 與 `2330` 兩檔，差異可能只是「這兩檔本來就跟
其他 9 檔不同」（都是權值股／ETF）。兩檔不足以歸因，寫結論時必須講清楚。

**Pass 1：zone 層 sweep（粗網格，6 組）**

- `--sweep --atr-width-grid 1.0,1.5,2.0 --max-merge-width-grid 1.5,2.5` → 3×2 = **6 組**，
  而不是預設的 5×3 = 15 組。先看**有沒有訊號**；若 6 組之間的差異落在雜訊內，加密網格也沒意義。
- **不開** `--sweep-decision-replay`（每組要多跑一次 replay，成本高）。

**Pass 1 結果（2026-08-06 已執行）**

6 組跑完約 **7 分鐘**、container 記憶體 **280m** 足夠、無 warnings。

| w | merge | rows | 支撐守住 | 壓力壓回 | 突破 | 平均報酬 |
|---|---|---|---|---|---|---|
| 1.0 | 1.5 | 9,493 | 41.01% | 32.17% | 39.54% | +0.041% |
| 1.0 | 2.5 | 6,144 | **42.36%** | 31.77% | **40.62%** | +0.010% |
| 1.5 | 1.5 | 8,069 | 40.60% | 32.08% | 40.18% | +0.014% |
| 1.5 | 2.5 | 4,907 | 41.58% | **32.50%** | 40.43% | +0.015% |
| 2.0 | 1.5 | 6,929 | 42.12% | 31.94% | 39.79% | **+0.059%** |
| 2.0 | 2.5 | 4,055 | 41.91% | 32.04% | 40.32% | +0.040% |

**四個指標選出四個不同的贏家**，沒有候選在多維度領先——雜訊的典型特徵。
完整判讀（含 bucket 建議的 score 全距、rows 差 2.3 倍不能當同一實驗、NORMAL 只有兩檔無法歸因）
已歸檔到 [`sr-zone-scoring.md`](./sr-zone-scoring.md) 的「2026-08-06 首次實跑 sweep 的結論」，
對 T-003 P2 的處置見該筆的「P2 的結論」。

**依計畫書自訂的準則（差異落在雜訊內就停）：不加密網格、Pass 2 不執行。**

**Pass 2：decision 層（選擇性）——2026-08-06 判定不執行**

原設計是：只在 pass 1 顯示候選之間有實質差異時才做（對勝出的 2～3 組加
`--sweep-decision-replay --model-path`）。Pass 1 的差異落在雜訊內，前提不成立，故略過。
若日後標的池擴大後重跑，這一層仍要注意 governance context 當時只有 9 列 / 2 檔，
分層會大量 missing，**不要把 missing 當成「治理不通過」**。

**與 T-002 P2 的關係：是兩件事，不要混在一起**

T-002 P2 要確認的是「排程用的 `replay_max_rows` / `symbols` 夠不夠」，那**不靠 sweep**，
靠一次普通的 `--decision-replay` 看 `replay_coverage`。而且前提已更正（watchlist 是 11 檔
不是 50～200 檔），現行預設很可能本來就夠——那是一次獨立的、更便宜的驗證。

**主要風險**

- **記憶體（最主要）**：host `MemAvailable` 約 500MB，evaluation 會載 pandas + sklearn +
  lightgbm（+shap）。順序執行時 peak 是單組的 peak 而非累加，但要在 pass 0 實測確認每組之間
  真的有釋放。`--memory` 比照 mem-guard 原則設定（不高於 available − 150MB），
  **不因為想跑快就調高**（見 `development-workflow.md`「`MEM` 是上限，不是預留」）。
- **live DB 讀取負載**：11 檔 × 1500 根 ≈ 16,500 列／次，每個 candidate 各讀一次。量極小，
  但 pass 1 是 6 次。純 SELECT。
- **輸出位置**：`--output` 要寫到 repo 外（掛一個 `/tmp` 或 scratchpad），不要落進 repo 被
  git 追蹤——sweep report 是一次性取樣結果，不是需要版控的產物。
- **判讀陷阱**：`00947`（523 根）與 `00981A`（293 根）歷史很短，`--limit 1500` 對它們是全取，
  樣本本來就少；看 per-symbol 數字時要記得這兩檔的權重不該與 2330／2454 等同。

**完成後歸檔**

跑完的結論（各 bucket 的樣本量、候選之間有無實質差異、是否足以支撐 T-003 P2 的決策）補到
[`sr-zone-scoring.md`](./sr-zone-scoring.md) 的「參數 sweep 的 decision 層比較」；
若結論是「資料量不足以下建議」，那本身就是要記下來的現況，避免下次有人再跑一次同樣的東西。

---

### T-040：擴充評估標的池（第一階段：精選 120～150 檔）

| 欄位 | 內容 |
|---|---|
| 狀態 | **已實作，待 review**（Step 0～5 全部完成；**池已匯入 live 並自主運作**，2026-08-18 盤點見下；只剩 `verify-regression-baseline.sh` 那半段驗收） |
| 優先度 | 高（同時解掉 T-002 / T-003 共同的取樣限制） |
| 分類 | Go / 資料同步 / 排程 / DB |
| 建立日期 | 2026-08-06 |
| 來源 | T-039 sweep 實跑結論：卡住的是標的池，不是參數 |
| Step 3 計畫 | 詳細流動性過濾與最終 universe 選取規格見 [`evaluation-universe-selection-plan.md`](./evaluation-universe-selection-plan.md) |
| 相依 | **T-003 的「bucket 邊界必須凍結」是本項的前置**，見下方「相依：T-003 邊界凍結」 |

**各 Step 狀態（2026-08-17）**：

| Step | 內容 | 狀態 |
|---|---|---|
| 0 | 記憶體實測 | ✅ 完成 2026-08-12（150 檔可行、200 檔不可行） |
| 1 | `ListCandidates` repo ＋ 端點 ＋ 前端頁面 | ✅ 完成 2026-08-12／13 |
| 2 | Step 1 全市場短期回補與判讀 | ✅ 完成 2026-08-13（857 檔 / 454,152 列） |
| 3 | selection report、選出最終清單 | ✅ 完成 2026-08-17（**131 檔**，計畫書階段 1～3 通過） |
| 4 | deep backfill ＋ 階段 4～6 驗證 | ✅ 完成 2026-08-17（131/131 對齊、覆蓋率 99.1%+、峰值 382MB、回歸基準已落地） |
| 5 | Phase 2：`evaluation_universe` 表與每日排程 | ✅ **已實作，待 review**（2026-08-17）。**2026-08-18 唯讀盤點：池已匯入（135 檔，非文件的 131）、排程已啟用、當日 15:06 同步 135 檔／0 失敗、池內日 K 全部到 08-18 且無手動回補**——端到端驗收的前半段成立。後半段（`run-evaluation.sh` → `verify-regression-baseline.sh`）尚未跑。詳見 [`evaluation-universe-selection-plan.md`](./evaluation-universe-selection-plan.md)「live 現況與端到端驗收」 |

#### 相依：T-003 邊界凍結

`selection_bucket` 是**對全體流動性合格股票取分位數**得到的，母體一動邊界就漂。
實測重跑 selection report 時，有 3 檔（3530、3661、8102）`atr_pct` 一個 bit 都沒變卻跳桶，
且選池 131 檔中有 18 檔距最近邊界不到 2%。

**後果是本項刻意經營的 bucket 配比會隨每日資料自己劣化**，且跨期比較會分不清
「策略改了」還是「bucket 定義改了」。處置（把 `quantile_edges` 凍結進 universe artifact）
記在 T-003 的「門檻重定 → bucket 邊界必須凍結」，**必須在 Step 5 建表前決定**——
`evaluation_universe.bucket_hint` 存的就是這個值。

**背景**：2026-08-06 實跑 sweep 後確認，SR Zone 的參數調校卡在標的池只有 **11 檔**
（9 檔落在 HIGH bucket、NORMAL 只有 `0050`／`2330`、LOW **完全空白**），候選之間的差異落在
雜訊內。要往前走需要更多橫跨波動區間的標的。完整結論見
[`sr-zone-scoring.md`](./sr-zone-scoring.md) 的「2026-08-06 首次實跑 sweep 的結論」。

#### 目標

1. 把評估用的標的池從 11 檔擴到 **120～150 檔**，且**橫跨三個波動 bucket**。
2. 這些標的每日盤後自動更新日 K，讓歷史持續累積。
3. **完全不改變現有 watchlist 的行為與成本。**

#### 不做的範圍

- **不做全市場 2,298 檔**。那是 CLAUDE.md Roadmap Phase 2 的方向，需要先改造 evaluation
  pipeline 的記憶體與時間（實測外推：2,298 檔單次 evaluation 約 4 小時、記憶體遠超這台
  2GiB host），屬另一個量級的工程，另案處理。
- **不讓新標的進入盤中掃描、籌碼同步、SR 分析、signal 掃描**（理由見下節）。
- **不動 bucket 門檻**。門檻要怎麼改，要等本項的 Step 1 量出實際分佈才有依據。
- 不改 evaluation pipeline 的批次設計——120～150 檔預期仍撐得住，但要實測確認。

#### 關鍵設計決定：新標的不能放進 `watchlists`

`watchlists` 目前驅動**六個**流程，把 200 檔塞進去會讓每一個都乘上 ~18 倍：

| 流程 | 觸發 | 每檔成本 | 200 檔的後果 |
|---|---|---|---|
| `runIntradayJob` | 盤中**每 5 分鐘** | 1 request | **完全不可行**：5 req/min 下光一輪就要 40 分鐘 |
| `runChipDailySync` | 每日 21:00 | 2 requests（法人＋融資券） | 400 requests → 80 分鐘 |
| `RunDailyClose` | 每日 15:00 | 1 request ＋ signal 評估 | 200 requests → 40 分鐘 |
| `runPreMarket` | 每日 08:50 | `BackfillHistory(5天)` | 200 requests → 40 分鐘 |
| `runSRZoneVerification` | 每日盤後 | SR zone 驗證 | 計算量 ×18 |
| SR evaluation 排程 | `symbols: []` ＝ watchlist | replay 母體 | 覆蓋率語意改變 |

所以要新增一個**與 watchlist 分離的「評估標的池」**：只維護日 K，不進盤中、不抓籌碼、
不做 SR 分析。watchlist 維持 11 檔不動。

#### 實作分兩階段

**Phase 1：選股用的一次性抓取（幾乎不需要改程式）**

查證後確認 `POST /api/v1/market/backfill`（`handler/market.go:27`）**已經支援**
`{"days": N, "symbols": [...]}`、在背景執行、並共用 `FinMindClient` 的 rate limiter。
所以 Step 1／Step 3 的抓取用現有端點就能跑。要補的是可觀測性與速度：

| 項目 | 狀態 |
|---|---|
| 進度追蹤（`market_backfill_jobs` job 紀錄 ＋ 前端輪詢） | **已完成**（Phase 1a，2026-08-07；現況見 [`database-schema.md`](./database-schema.md) 的 `market_backfill_jobs` 與 [`api-reference.md`](./api-reference.md) 的 market 章節） |
| 手動輸入代號（不再侷限 watchlist） | **已完成**（同上；`symbols` 已改必填） |
| 可續跑 | **未做**。job 紀錄讓中斷後知道跑到哪，但 backend 重啟不會接手既有任務，仍需人工用剩餘 symbols 重送 |
| 速度 | **不調整**。FinMind 註冊帳號的官方上限是 **600 requests/小時**（＝10/min），現行 5/min 已用掉一半、另一半留給重試與突發。大批量回補靠**拉長時間**而不是拉高速率；650 檔約 2.2 小時、150 檔約 30 分鐘，都是可接受的一次性成本 |
| 候選清單來源 | **已完成**（2026-08-12）。`GET /api/v1/stock-symbols/candidates` ＋ `StockSymbolRepo.ListCandidates`，支援 `security_type` / `industry` / `listed_years` / `per_industry` / `limit` / `include_delisted`，回傳可直接餵給 `POST /market/backfill` 的扁平 `symbols` 與供人工核對的 `by_industry`。**沒有撐大既有的 `Search`**——那支是 autocomplete，`limit > 100` 打回 20 是刻意設計。現況見 [`api-reference.md`](./api-reference.md) |

**Phase 2：常態維護——一個「純日 K」清單（選完標的後才需要）**

新增的 `evaluation_universe` 是**只處理日 K 的清單**，這是它與 `watchlists` 的唯一分野，
也是整個設計的重點。**明確界定它做什麼、不做什麼**，避免日後被順手接上其他流程：

| | `watchlists`（11 檔，不動） | `evaluation_universe`（新，120～150 檔） |
|---|---|---|
| 每日日 K | ✅ | ✅ **只有這一項** |
| 盤中分 K（每 5 分鐘） | ✅ | ❌ |
| 籌碼同步（法人／融資券） | ✅ | ❌ |
| signal 掃描 | ✅ | ❌ |
| SR zone 分析與驗證 | ✅ | ❌ |
| 前端 watchlist 畫面 | ✅ | ❌（研究用，不進使用者的關注清單） |

用途只有一個：**讓 evaluation / sweep 有夠寬的取樣母體**。任何要把它接上盤中或籌碼流程的
提案，都要先回頭看本筆「關鍵設計決定」那張成本表。

| 檔案 | 動作 |
|---|---|
| `migrations/{mysql,postgres,sqlite}/066_create_evaluation_universe.sql` | 新表：`symbol`、`bucket_hint`、`selected_at`、`source`、`active`、`note`。**編號是 066 不是原本寫的 058**——058 已被 `market_backfill_jobs` 用掉，mysql／postgres 目前在 065 |
| `store/evaluation_universe_repo.go` ＋ `model.go` | repo 與 model |
| `scheduler/scheduler.go` | 新 job `evaluation_universe_sync`：每日盤後（晚於 `daily_close` 的 15:00，建議 16:00）**只對池內標的跑 `FetchAndStoreDaily`**；不呼叫 `signalEng.Evaluate`、不進 SR 驗證、不進籌碼同步 |
| `config.yaml` | 新增 `evaluation_universe` 區段：`enabled`（預設 false）、`cron`、`batch_size` |
| `api/handler` ＋ `router.go` | 池的 CRUD 與手動觸發 |

**每日成本**：150 檔 × 1 request ÷ 5 req/min = **約 30 分鐘**，排在 16:00 之後的離峰時段，
與盤中排程不重疊，也遠低於 FinMind 的 600/h 上限。

#### 執行順序（2026-08-12 定案）

原計畫把記憶體實測排在 Phase 2 之後。**改成最先做**——理由見下方風險段：150 檔的
evaluation 從未實測，若跑不動，前面所有抓取與建表都是白工。順序因此是：

| # | 步驟 | 產出／決策點 | 成本 |
|---|---|---|---|
| **0** | **記憶體實測**（新增，最先做）**✅ 已完成 2026-08-12** | 見下方「Step 0 實測結果」。結論：150 檔可行、200 檔不可行，**不需要先做串流化改造**（見 T-047） | 實際只補 11 檔 |
| 1 | `ListCandidates` repo 方法與端點 **✅ 已完成 2026-08-12** | `GET /stock-symbols/candidates`，見上方 Phase 1 表與 [`api-reference.md`](./api-reference.md) | 純 backend，無外部依賴 |
| 2 | Step 1 抓取（見下） | 全市場 ATR% 分佈 | 650 requests ≈ 2.2 小時 |
| 3 | Step 2 判讀 → bucket 門檻定案 | **可能改變 T-003 的設計** | 分析，無抓取 |
| 4 | Step 3 選 120～150 檔並深抓 | 最終標的池 | 150 requests ≈ 30 分鐘 |
| 5 | Phase 2：`evaluation_universe` 表與排程 | 常態維護。詳細計畫見 [`evaluation-universe-selection-plan.md`](./evaluation-universe-selection-plan.md)「Step 5 執行計畫書」 | 見上方檔案表 |

**Step 1 與 Step 3 不合併（2026-08-12 決定）**：`FetchDailyCandles`（`market/finmind.go:182`）
**帶日期區間與單日同價**，都是 1 request/檔，所以 650 檔直接抓 5 年與抓 130 天是**同樣 650
requests**——合併可省下 Step 3 那趟 30 分鐘。**但不採用**：代價是 `candles` 從約 18 萬列變
約 78 萬列（現有 29,208 列的 27 倍），多出的 60 萬列有九成以上屬於不會入選的標的，
30 分鐘的重抓成本遠低於長期背在 `candles` 上的索引負擔。**Step 1 只抓 130 天，Step 3 才深抓。**

#### Step 0 實測結果（2026-08-12，已完成）

標的池補到 **40 檔**（新增 11 檔，見下節），三個 bucket 首次全部有標的：
**LOW 5 / NORMAL 15 / HIGH 20**。以巢狀分層子集合量成長曲線
（`MEASURE_PEAK=1 scripts/run-evaluation.sh`）：

| N | rows | 峰值 | 耗時 |
|---|---|---|---|
| 10 | 6,032 | 281 MB | — |
| 20 | 11,859 | 281 MB | 131s |
| 30 | 17,447 | 317 MB | 191s |
| 40 | 22,401 | 310 MB | 241s |

**峰值由固定的 import 開銷主導**：標的數 4 倍、rows 3.7 倍，峰值只增加約 30MB。
先前判斷「270MB 幾乎都是 pandas/sklearn/lightgbm/shap 的
import 開銷」**已由實測證實**（現況數據見 [`sr-zone-scoring.md`](./sr-zone-scoring.md)「規模上限」）。邊際成本約 **1.0 MB/檔**。
N=30（317MB）高於 N=40（310MB）的 7MB 是量測噪音（cgroup v1 峰值含 page cache），
不是真實反轉——兩者是超集關係。

**外推與結論**（只外推量到的 30MB 資料相依部分，不外推總量）：

| 規模 | 推估峰值 | 所需 available（峰值＋150MB 保留） | 判定 |
|---|---|---|---|
| 150 檔 | ~420 MB | 570 MB | ✅ **僅在不常駐 gitea 那一級服務時**（實測 630～682MB），餘裕約 60MB |
| 200 檔 | ~470 MB | 620 MB | ❌ 極度邊緣，實質不可行 |

**因此目標上限由 200 檔下修為 150 檔**，且**執行前必須確認 host available ≥ 570MB**。
實測期間 gitea（209MB）常駐時 available 只有 398MB，mem-guard 直接擋下、
連 10 檔都跑不起來——這是 `development-workflow.md`「本機同時只留一組 stack」的另一個面向。

時間約 **5.5 秒/檔**，150 檔單次約 14 分鐘（與先前估的 16 分鐘相符）；
sweep 要乘候選數，6 組候選約 1.4 小時。

**尚未驗證**：外推假設 zone building 的中間物隨標的數線性成長，這一段沒有直接量到。
實際擴到 150 檔時要再量一次，不要只依賴本次外推。

#### Step 0 的標的怎麼挑

原則是**三個 bucket 各挑幾檔**（2026-08-12 決定），但有一個必須先處理的循環相依：
**bucket 標籤要等 Step 1／Step 2 算出 ATR% 才會有，Step 0 當下不存在**。而且 LOW bucket
現在是**空的**——那正是本項要解決的問題本身（現有 11 檔的 ATR% 是 `2330`／`0050` 的 3.2%
到 `6243` 的 11.6%，門檻是 `LOW ≤ 1.5%`／`HIGH ≥ 3.5%`，見 `zone_builder.py:43-44`）。

所以 Step 0 改用**預期波動的代理指標**分層，不需要先有資料：

| 層 | 代理 | 補幾檔 |
|---|---|---|
| 預期低波動 | 電信、公用事業、大型金控、債券型 ETF | 8～10 |
| 預期中波動 | 權值股、市值型與高股息 ETF、傳產龍頭 | 8～10 |
| 預期高波動 | **不補**——現有 11 檔已有 9 檔落在 HIGH，再補只會加重既有的偏斜 | 0 |

**這樣挑不會影響 Step 0 的效力**：Step 0 量的是**記憶體峰值**，不是調參。峰值由「檔數 ×
`--limit` 列數 ＋ touch dataset 大小」決定，而 touch 數會隨波動變化（2026-08-06 的 sweep
實測不同 builder 參數的 rows 差 2.3 倍），所以**涵蓋波動範圍才量得到有代表性的峰值**。
代理猜錯某一檔的實際 bucket 不影響這個目的——那是 Step 2 要回答的問題。

計畫書「不做的範圍」寫的「硬塞債券 ETF 進 LOW 反而有害」**不適用於 Step 0**：那句話針對的是
拿它們調 zone builder 參數。Step 0 不調參，只量記憶體。但這些代理標的**不自動進入最終標的池**，
要不要留由 Step 2／Step 3 依實際 ATR% 決定。

**實際執行結果（2026-08-12）**：`candles` 原本就有 **29 檔**（不只 watchlist 的 11 檔——
先前減資／還原驗證補過），其中 27 檔深度 ≥1,587 列。所以只補了 11 檔就到 40 檔：
`00679B` `00687B` `00694B` `00695B` `00697B` `3045` `4904` `2912` `5876` `2801` `6505`
（`POST /api/v1/market/backfill`、`days=2400`，對齊既有 1,594 列那批的深度）。

**代理指標的準確度**：債券 ETF 全部落在 LOW（0.40%～0.76%）符合預期，但**個股的代理猜錯兩次**
——`6505` 台塑化 7.23%、`1301` 台塑 6.08%，兩檔「傳產／能源低 beta」實際都是 HIGH
（石化下行週期）。代理只能用來粗分，真正的 bucket 一定要等實際資料。

**交付 Step 2 的早期證據（40 檔樣本）**：LOW bucket 全部是債券 ETF，
0.76%（最高的債券 ETF）到 1.57%（最低的股票 `3045` 台灣大）之間完全是空白。
當時據此推論「台股個股端根本不存在 LOW 這個群體」。

> ⚠️ **這個推論已被 Step 1 的完整資料推翻**（2026-08-13）——那是**樣本只有 15 檔股票
> 造成的假象**。503 檔股票裡有 21 檔落在 LOW。詳見下方「Step 2 實測結果」。
> 保留這段是為了記住教訓：**小樣本的「完全空白」不是證據，只是沒抽到。**

**T-043 不是本項的前置條件**（2026-08-12 修正）。Yahoo 批次端點取不到歷史日 K，且
evaluation 硬性需要成交量與成交金額，兩者 Yahoo 都給不了或不可用。詳見 T-043 的
「與 T-040 的關係」。

#### 選股方法（三步，Step 2 的結果可能改變 T-003 的設計）

**Step 1：量分佈，不挑股票**

- 樣本：全部 354 檔 ETF ＋ 各產業分層抽樣約 300 檔股票 ≈ **650 檔**，每檔只抓最近約
  **130 天**（`VOLATILITY_PROFILE_LOOKBACK = 60` 個交易日 ＋ 假日 buffer）。
- 產出：全市場的 ATR%/close 分佈。
- 成本：650 requests，`rate_limit=5` 下約 **2.2 小時**（一次性，不需要調速率）。

**Step 2：依分佈決定策略**

現有 11 檔中最低波動的是 `0050` 的 **ATR% 3.25%**，而 HIGH 門檻是 3.5%、LOW 門檻是 1.5%
（約 `0050` 的一半）。**台灣最廣泛分散的股票型 ETF 都幾乎算不上「非高波動」**，
這強烈暗示門檻相對台股實際分佈定得太低。

- 若分佈顯示確實有一群 ≤1.5% 的**股票型**標的 → 照原計畫選它們填 LOW。
- 若幾乎沒有 → **結論是門檻要重定**，改用實際分佈的分位數（例如 P33/P67）
  切三個 bucket。硬塞幾檔債券 ETF 進 LOW 只會讓該組全是與股票行為完全不同的商品，
  拿來調 zone builder 參數**反而有害**。

##### Step 2 實測結果（2026-08-13，Step 1 抓取完成後）

抓取現況：**857 檔 / 454,152 列**（504 股票 ＋ 353 ETF）。811 檔 ≥130 列、835 檔資料到
2026-08-13。17 檔不足 60 列**幾乎全是 2026 年 6 月後新上市的 ETF**，屬正常而非抓取失敗。

以現行門檻（`LOW <1.5%` / `HIGH >3.5%`）對 840 檔（≥60 根）計算：

| 類型 | 檔數 | LOW | NORMAL | HIGH |
|---|---|---|---|---|
| 股票 | 503 | **21** | 150 | 332 |
| ETF | 337 | 183 | 114 | 40 |
| 合計 | 840 | 204 | 264 | 372 |

**先前「個股端不存在 LOW」的推論是錯的**——那是 15 檔股票樣本的假象。但**結論的方向沒變，
只是原因不同**：

**低波動與低流動性高度重疊。** 21 檔 LOW 股票的日均成交額：

| 標的 | ATR% | 日均成交額 |
|---|---|---|
| `2633` 台灣高鐵 | 1.45% | **3.34 億** |
| `4114` 健喬 | 1.31% | 4,200 萬 |
| `2459` 敦吉 | 1.46% | 1,350 萬 |
| …其餘 17 檔 | — | **多在 1,000 萬以下** |
| `3067` 全域 | 1.32% | **10 萬** |

**成交額太小的標的，ATR% 低是因為沒人交易，不是因為它穩定。** 拿這種標的調 zone builder
參數，學到的是「沒有成交所以價格不動」，與「有支撐所以守得住」是完全不同的事。

**因此流動性下限必須在 bucket 分層之前套用**，否則 LOW bucket 會被殭屍股填滿。
以日均成交額 5,000 萬粗估，21 檔裡大概只剩 `2633` 一檔——LOW bucket 仍然填不滿，
只是原因從「不存在」變成「存在但都沒有流動性」。

**待決定**：流動性門檻要訂多少。這個數字直接決定 LOW bucket 是否可用，
也決定要不要改用分位數切 bucket。

**另外三檔資料稀薄但主檔仍標 `is_listed=true`**：`4804`（停在 2026-04-13）、
`6236`、`00625K`。是成交本身稀薄（停牌或極低流動性），不是抓取失敗——
正好是流動性下限要濾掉的類型。

**Step 3：選 120～150 檔深抓 5 年** ✅ **選池完成 2026-08-17（130 檔）**

> 完整結果、三個設計決定與演算法說明見
> [`evaluation-universe-selection-plan.md`](./evaluation-universe-selection-plan.md)
> 的「Step 3 實測結果」。**最重要的發現**：pipeline 的 bucket 絕對門檻與台股實際分佈
> 差一個量級（1.5%/3.5% vs 實測分位數 4.25%/6.04%），**不重定門檻，選池怎麼挑都沒用**
> ——已列為 T-003 的前置輸入。deep backfill 尚未執行。

詳細計畫見 [`evaluation-universe-selection-plan.md`](./evaluation-universe-selection-plan.md)。
Step 3 的重點不是直接建 `evaluation_universe` 表，而是先產出 selection report，
用流動性門檻矩陣與 bucket 策略決定最終標的池，再 deep backfill 5 年。

三個 bucket 各 40～60 檔，選取規則：

- **產業分散**：半導體業有 201 檔，隨機抽會被它主導；每個 bucket 內限制單一產業佔比。
- **流動性下限**：2,298 檔裡有大量低成交量標的，OHLCV 本身就是雜訊、zone touch 沒有意義。
  用平均成交金額設門檻。
- **上市滿 5 年**：需要足夠的 walk-forward 深度，新股先排除。
- **保留現有 11 檔**：維持與 2026-08-06 那批結果的可比性。
- 成本：150 requests × 5 年，`rate_limit=5` 下約 **30 分鐘**。

#### 前端頁面（設計 2026-08-12，**已實作 2026-08-13**）

**實作結果**：新頁面 `routes/EvaluationUniverse.svelte`（route `evaluation-universe`、
側邊欄「評估標的池」），三段流程如設計稿。連同前置重構共 5 個檔案：

| 檔案 | 內容 |
|---|---|
| `lib/utils/jobPolling.ts` ＋ `.test.ts` | **先抽再用**的共用輪詢（含停滯保護），9 支測試。`Backfill.svelte` 原有兩份幾乎相同的實作，這頁會是第三份。抽出後才發現原本兩份共有的競態：慢回應會在收尾後把畫面蓋回舊狀態，一次修好三處 |
| `lib/api/stockSymbols.ts` | `fetchSymbolCandidates`；`StockSymbol` 型別自 `watchlist.ts` re-export |
| `routes/EvaluationUniverse.svelte` ＋ `.test.ts` | 頁面本體，17 支測試 |
| `routes/Backfill.svelte` | 兩處輪詢改用共用工具，行為不變（既有 11 支測試全過） |
| `router.ts` / `App.svelte` / `Sidebar.svelte` | 路由與導覽 |

**篩選選項改由 API 提供（2026-08-13 追加）**：初版把產業做成文字輸入 ＋ `datalist`，
因為當時**沒有任何端點回傳產業清單**——選項只能取自上一次查詢的 `by_industry`，
所以第一次查詢前選單是空的，使用者還是得先知道「半導體業」這五個字才打得出來。
已補上 `GET /stock-symbols/facets`（`StockSymbolRepo.Facets`，現況見
[`api-reference.md`](./api-reference.md)），證券類型與產業改為由 API 驅動的複選標籤，
各自標示**母體**筆數。三個設計決定：

- **`count` 是母體不是取樣數**：挑 `per_industry` 時要看母體才知道 9 是多是少
  （半導體業 201 檔 vs 玻璃陶瓷 5 檔）；`/candidates` 的 `by_industry` 是取樣**後**的數字。
- **`security_type` 參數只縮放 `industries`，不影響 `security_types` 清單本身**，
  否則使用者選了某個類型之後就換不回來。
- **產業清單排除 `industry = ''`**：那是「未分類」而不是一個產業（ETF 與權證全在那裡）。

**權證的處理**：選單**完整列出所有 ISIN 分類並標示筆數**，但預設只勾股票與 ETF。
看到「上市認購(售)權證 31,090」這個數字使用者自己就知道不該勾——
**用資訊而不是隱藏來防止誤選**，同時保留日後要研究特別股、創新板的彈性。

**驗收**：`frontend/scripts/test.sh` 全綠（svelte-check → 88 支 vitest → vite build）。
注意 build 產物 `backend/internal/ui/dist` **依設計要進版控**（`ui.go` 的 `//go:embed all:dist`），
新的 hash 檔名要 `scripts/check-dist-assets.sh --fix` 一併 stage，
**且舊 bundle 的刪除要在同一次 commit**，否則會做出 index.html 指向不存在檔案的前端，
而所有測試仍然會過。

以下為當初的設計稿，保留供 review 對照：

**定位**：把 Step 1／Step 3 的「產生候選清單 → 觸發回補 → 追進度」從敲 API 變成畫面操作。
**不是**要做一個新的分析頁——判讀留給既有頁面。

**分兩版，因為後端還沒到位**：

| | v1（現在可做） | v2（`evaluation_universe` 表上線後） |
|---|---|---|
| 候選清單產生 | ✅ `GET /stock-symbols/candidates` | 同左 |
| 觸發回補＋進度 | ✅ `POST /market/backfill` ＋ 既有輪詢 | 同左 |
| 標的池存檔／載入 | ❌ **沒有後端**，清單只存在當次操作 | ✅ 池的 CRUD |
| 波動分佈判讀 | 🔗 **連到既有 SR Zones 頁面**，不重做 | 同左 |

##### 為什麼新開一頁，而不是加進「歷史資料回補」

`Backfill.svelte` 已經 533 行、塞了四塊功能（股價回補／籌碼回補／手動指標／手動訊號）。
再加第五塊會讓一個日常維運頁面同時承擔研究用途。兩者的使用頻率與對象都不同：
回補頁是「今天資料缺了補一下」，標的池是「一季調整一次研究母體」。

新 route：`evaluation-universe`，Sidebar 標籤「評估標的池」（放在「歷史資料回補」之後）。

##### 畫面結構

```
┌ 評估標的池                                          [重新整理] ┐
│                                                              │
│ ┌─ ① 產生候選清單 ────────────────────────────────────────┐ │
│ │ 用途說明：Step 1 量分佈用全部 ETF ＋ 各產業分層抽樣       │ │
│ │                                                          │ │
│ │ 證券類型 [股票 ▾] 產業 [不限 ▾] 每產業上限 [9]           │ │
│ │ 上市滿   [  ] 年（留空 = 不限）   總筆數上限 [1000]      │ │
│ │                                        [產生候選清單]    │ │
│ │ ─────────────────────────────────────────────────────── │ │
│ │ 共 293 檔，涵蓋 34 個產業                                │ │
│ │ ┌ 產業分佈 ──────────────────────────────────┐          │ │
│ │ │ 電子零組件業  9 / 209  ████████░░░░░░░░░░  │          │ │
│ │ │ 半導體業      9 / 201  ████████░░░░░░░░░░  │          │ │
│ │ │ …（可摺疊，預設顯示前 10）                  │          │ │
│ │ └────────────────────────────────────────────┘          │ │
│ │ 代號預覽（可編輯的 textarea，逗號分隔）                  │ │
│ │ ┌──────────────────────────────────────────┐            │ │
│ │ │ 1101,1102,1103,…                          │            │ │
│ │ └──────────────────────────────────────────┘            │ │
│ │                          [加入下方回補清單] [複製]       │ │
│ └──────────────────────────────────────────────────────────┘ │
│                                                              │
│ ┌─ ② 回補這批標的 ────────────────────────────────────────┐ │
│ │ 代號 [textarea，承接①，可手動增刪]      天數 [130]       │ │
│ │ 預估耗時：293 檔 ÷ 5 req/min ≈ 59 分鐘   [開始回補]      │ │
│ │ ─────────────────────────────────────────────────────── │ │
│ │ bf-xxxx  [同步中]  47/293 檔，失敗 0                     │ │
│ │ ████████░░░░░░░░░░░░░░░░░░░░░░░░  16%                    │ │
│ └──────────────────────────────────────────────────────────┘ │
│                                                              │
│ ┌─ ③ 下一步 ──────────────────────────────────────────────┐ │
│ │ 回補完成後到「支撐/壓力機率」頁面跑一次 evaluation，      │ │
│ │ 在報告的「波動側寫」區看三個 bucket 的實際分佈。          │ │
│ │                              [前往支撐/壓力機率 →]       │ │
│ └──────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────┘
```

##### 各區塊規格

**① 產生候選清單**

- 證券類型：下拉，選項由 `stock_symbols.security_type` 的實際值來（**中文**：`股票`、`ETF`、
  `特別股`…），預設 `股票`。**不要寫死英文**——DB 存的是中文，寫死會查不到任何東西。
- 產業：多選，選項同樣來自實際值；預設不限。
- 每產業上限：數字，預設 9。實測 `security_type=股票,ETF&per_industry=9` **一次呼叫**
  就得到 293 檔股票 ＋ 354 檔 ETF ＝ **647 檔**，對上計畫書的「≈650 檔」——
  ETF 的 `industry` 是空字串而空字串不受上限約束，所以不用拆成兩次呼叫。
  填 0／留空 = 不限。
- 上市滿 N 年：**預設留空**。這是 Step 3 的規則，Step 1 不該帶——帶了 ETF 只剩 199 檔
  （354 檔中僅 199 檔上市滿 5 年），會漏掉四成母體，而 ETF 正是目前唯一填得進 LOW bucket 的類型。
  欄位旁要有這句提示。
- 產業分佈：`by_industry` 直接畫成「取樣數 / 母體數」的橫條，讓「有沒有被單一產業主導」
  一眼看得出來。預設顯示前 10 個產業，可展開。
- 代號預覽用 **textarea 而非唯讀清單**：實務上一定會想手動剔除幾檔（例如已知的問題標的）。

**② 回補這批標的**

- 完全重用 `Backfill.svelte` 既有的 `triggerBackfill` ＋ 輪詢流程，**包含停滯保護**
  （3 秒輪詢、連續 5 分鐘 `symbols_done` 沒推進才停止並解鎖，刻意不用固定逾時——
  650 檔在 rate limit 下本來就要跑兩個多小時）。
- **新增預估耗時**：`檔數 ÷ 5 req/min`。這是既有回補頁沒有、但這裡必要的——
  按下去要跑一小時，使用者有權在按之前知道。
- **新增進度條**：既有頁面只有「47/293 檔」文字。檔數上百時進度條的資訊密度明顯更好。
- 天數預設 130（Step 1 的 `VOLATILITY_PROFILE_LOOKBACK = 60` 個交易日 ＋ 假日 buffer）。

**③ 下一步**：只有文字與跳轉按鈕，不重做判讀。波動側寫已在
`SRZones.svelte` 的 evaluation 報告中呈現（含 bucket），重做等於維護兩份。

##### 要新增／重構的檔案

| 檔案 | 動作 |
|---|---|
| `lib/api/stockSymbols.ts` | **新增**。`fetchSymbolCandidates(opts)`。目前 `/stock-symbols/search` 借放在 `watchlist.ts`，候選清單與 watchlist 無關，不該再往那裡塞；`StockSymbol` 型別從 `watchlist.ts` 重用 |
| `lib/utils/jobPolling.ts` | **新增（重構）**。`Backfill.svelte` 裡 `pollJob` 與 `pollChipJob` 是兩份幾乎相同的程式碼，這頁會變成第三份。抽成共用的 `pollUntilTerminal({ fetch, isTerminal, progressOf, onUpdate })`，含停滯保護。**先抽再用**，不要先複製 |
| `routes/EvaluationUniverse.svelte` | 新增頁面 |
| `lib/stores/router.ts` | `Route` 加 `'evaluation-universe'` |
| `App.svelte`、`components/layout/Sidebar.svelte` | 掛路由與 nav（icon 建議 `⊞`） |

##### 狀態與錯誤處理

- 三種載入態各自獨立：產生清單、回補送出、輪詢中。**不共用一個 `loading`**——
  回補跑一小時期間，使用者應該還能重新產生候選清單。
- 候選清單為 0 筆時給明確訊息（最可能原因是證券類型填了英文值），不要只顯示空表。
- 回補的錯誤與 `failures[]` 逐檔顯示，比照既有頁面。
- **離開頁面不影響後端**，但輪詢會斷；文案要講清楚，並顯示 `job_id` 供事後查詢。

##### 不做的範圍

- 不做池的存檔／載入（等 `evaluation_universe` 表，Phase 2）。
- 不做流動性篩選——那需要 `candles.amount`，而候選標的當下還沒有 K 線。Step 3 的流動性
  下限要等回補完才算得出來，屬另一個步驟。
- 不做波動分佈的判讀畫面（見上，已存在）。
- 不動 `Backfill.svelte` 的四塊既有功能，只把輪詢邏輯抽出去共用。

##### 測試策略

比照 `Backfill.test.ts` 與 `SRZones.test.ts` 的慣例（Vitest ＋ Testing Library）：

- 候選清單：送出後正確帶上 query 參數；`by_industry` 有渲染；0 筆時顯示提示。
- 參數預設值：`listed_years` **預設為空**（這是最容易寫錯、且錯了會靜靜漏掉四成 ETF 的地方）。
- 回補：預估耗時的計算；進度條百分比；停滯保護觸發後解鎖並顯示 `job_id`。
- 抽出來的 `pollUntilTerminal` 要有自己的單元測試，涵蓋終態、停滯、fetch 失敗三條路徑。

#### 資料 contract 變化

- 新表 `evaluation_universe`，與既有表無外鍵相依；**不改 `watchlists`、`candles`、
  `stock_symbols` 的結構**。
- `candles` 只是多了更多 symbol 的資料列，schema 不變。
- API 為相容新增；既有端點行為不變。
- **仲裁順序不變**：這個池不參與任何交易決策或狀態推導，純粹是研究用的標的清單。

#### 主要風險與回滾

- **不動 `rate_limit`**：FinMind 註冊帳號上限 600 requests/小時，現行 5/min（300/h）已用一半。
  所有請求共用同一個節流器，調高會影響 live 既有排程（盤中每 5 分鐘的分 K 拉取也走同一條），
  收益（省幾小時的一次性作業）遠小於風險。**本次明確不調。**
- **DB 成長**：150 檔 × 5 年 ≈ 18 萬列（現有 29,208 列的 6 倍）。磁碟不是問題，但
  `candles` 的既有索引效能要留意。
- **evaluation 記憶體（最主要的未知數，也是本項唯一可能整個做白工的風險）**：150 檔的
  evaluation **未經實測**。先前「以 270MB 線性外推到 1.5～2GB」的估算**是錯的**——那 270MB
  幾乎都是 pandas / sklearn / lightgbm / shap 的 import 開銷，資料本身只有數 MB。重新估算後
  150 檔約落在 **350～450MB**，而這台 host 的 `MemAvailable` 常態只有 450～510MB、mem-guard
  還要再保留 150MB——**是「邊緣」而不是「大概沒問題」**。完整分析與改造方向見
  [`sr-zone-scoring.md`](./sr-zone-scoring.md)「規模上限」。
  因此**實測提前為 Step 0**（見上方執行順序），不再排到 Phase 2 之後：抓 2.2 小時、建表、
  接排程之後才發現跑不動，前面全部是白工。量完若超標，先做 T-047 的第 1、2 項
  （逐檔釋放原始 frame、只累積預測機率＋label 最後算 AUC）再往下走。
- **回滾**：Phase 1 只新增 job 表與唯讀查詢端點，`git revert` 即可；已抓下來的 candles
  留著無害（多的 symbol 不會被任何既有流程掃到）。Phase 2 的 migration 有 `-- +goose Down`。

#### 測試與驗證策略

- `backend/scripts/test.sh ./internal/market/... ./internal/store/... ./internal/api/handler/... ./internal/scheduler/...`
- migration 在 **dev project** 實跑（CLAUDE.md：不得用 live/deploy compose 驗證 migration）。
- **實際抓取由使用者在 live 環境執行**，之後由本人透過 live DB 驗證：
  1. `candles` 的 symbol 數與列數是否符合預期
  2. 各 symbol 的日期涵蓋範圍是否完整（有無缺漏交易日）
  3. 用實際資料算出 ATR% 分佈，交付 Step 2 的判斷
- **記憶體**：Step 0 就要實測（30～50 檔），不要假設線性外推成立，也不要等到 Phase 2 之後。
  量測方式比照既有腳本慣例（`scripts/` 內的 mem-guard 與 container 記憶體上限），
  記錄峰值與當下 host available，判準是「峰值 ＋ 150MB 保留 < host available」。
- **migration 編號**：新表是 `066`（058 已被 `market_backfill_jobs` 佔用）。三種 engine
  各一份，其中 mysql 版要另跑 `scripts/test-mysql-migrations.sh`
  （dev/live 是 postgres，mysql 那份沒有其他執行路徑）。

#### 完成後歸檔

- ✅ **已歸檔（2026-08-18）**：評估標的池與 watchlist 的分工、為何不合併 →
  [`architecture.md`](./architecture.md)「兩個標的清單」。含六個流程的成本表、
  現況職能對照、哪些研究該用哪一份，以及「池不加籌碼」的理由。
- ATR% 實際分佈與 bucket 門檻的最終決定，補到
  [`sr-zone-scoring.md`](./sr-zone-scoring.md) 的 zone builder 章節。
- FinMind 官方 rate limit（600 requests/小時）已於 2026-08-06 更正到
  [`finmind-integration.md`](./finmind-integration.md) 的「Rate Limit 處理」——
  該處原本寫「每分鐘約 30 requests」（＝1800/h），與官方值差 3 倍。

---

### T-041：SR Zone 決策顯示補齊 Lifecycle、Event Timeline 與 Strategy Layer

| 欄位 | 內容 |
|---|---|
| 狀態 | **Event Timeline 面向已實作／待 review**；review 8 筆發現**全數已實作待 review**（R1、R5 於 2026-08-21 各依計畫書完成，見下方「Review 發現」）；Lifecycle 與 Strategy Layer 兩個面向仍待規劃（**前置已解除**：Lifecycle Engine 已於 2026-08-18 收斂並移出本清單，狀態定義見 [`sr-zone-scoring.md`](./sr-zone-scoring.md)「分層原則：lifecycle 不看 RR」） |
| 優先度 | 中 |
| 分類 | SR Zone / Decision UI / Position Action |
| 建立日期 | 2026-08-07 |
| 來源 | 使用者需求：決策畫面需要更完整地呈現 lifecycle、事件鏈與策略層 |

目前 SR Zone 決策顯示仍偏向單一摘要，還沒有把 lifecycle、完整事件鏈與不同策略層的
position action 正式整理成可讀的前端狀態。後續應補齊下列三個面向：

- **Lifecycle 正式顯示**：在決策畫面明確呈現 `Started` / `Testing` / `Confirmed` /
  `Failed`，並定義這些狀態與既有 event lifecycle / daily confirmation / final entry state
  的對應關係。
- **Event Timeline**：改為呈現完整事件鏈，而不是只顯示兩個 Event。Timeline 應能看出事件
  何時開始、測試、確認、失敗、過期或被後續事件取代，並保留事件順序與狀態轉換脈絡。
- **Strategy Layer**：加入 `Trading`、`Swing`、`Investment` 三種策略層，讓同一組 SR Zone
  / event / confirmation 訊號能對應到不同的 `Position Action`，避免短線、波段與投資邏輯
  混在同一個建議裡。

#### 後續規劃重點

- 先盤點後端 decision summary / event state / replay report 目前已輸出的欄位，確認哪些可直接
  支援前端顯示，哪些需要補 contract。
- 若新增或改動 API contract，要同步更新 `docs/api-reference.md` 與 SR Zone 相關主題文件。
- `Position Action` 的策略差異應先文件化仲裁規則，再實作 UI，避免前端自行推導交易語意。

#### Event Timeline 面向的實作結果（2026-08-21，已實作／待 review）

三個面向裡只做了 **Event Timeline**，接的是 T-051 改讀身分層之後的
`GET /sr-zones/event-timeline`。Lifecycle 與 Strategy Layer 兩個面向**這一輪不做**，
狀態不變。

改動範圍：

| 檔案 | 內容 |
|---|---|
| `frontend/src/lib/api/srZones.ts` | 新增 `SREventTimeline*` 型別與 `getEventTimeline()`；型別註解寫明 `zone_uid` 才是鏈的身分、`seq > 1` 是新鏈、`gap_days > 1` 代表沒有觀測 |
| `frontend/src/lib/utils/eventTimeline.ts` | 語意判斷：`splitChains` / `chainEndNote` / `maxGapDays` / `chainZoneLabel` |
| `frontend/src/lib/utils/eventTimeline.test.ts` | 上述四個函式的單元測試（10 條） |
| `frontend/src/components/sr/SREventTimeline.svelte` | 預設收合的顯示元件；換 symbol／timeframe 時作廢已載入內容並重抓 |
| `frontend/src/routes/SRZones.svelte` | 掛在 Event Sequence 正下方；順手把前者標成「Event Sequence（當次分析）」 |

判讀規則與理由已歸檔到
[`sr-zone-scoring.md`](./sr-zone-scoring.md)「前端 Event Timeline 的判讀規則（現況）」。

驗證：`npm run test:unit` 20 檔 131 條全過、`svelte-check` 0 errors / 0 warnings、
`scripts/check-dist-assets.sh` 在 `git add` 前如預期報未納管（dist 已重建，
`index.html` 指向 `index-DBnXF-i3.js` / `index-DVlyP0ZY.css`）。
真實資料以 dev DB 的四檔 21 階看過：`2330` 28 條鏈全已終結（「全部終結時看起來像壞掉」
那條規則就是這樣踩出來的，已修掉）、`6182` 涵蓋其餘邊界（7 條進行中、2 條 `seq > 1`、
8 條 `ZONE_IDENTITY_ENDED`、1 條 `SYMBOL` scope）。

**不做的範圍**：Lifecycle 正式顯示、Strategy Layer 三層 position action、
timeline 的區間／筆數 UI（`max_analyses` 目前固定 60）、鏈的圖形化時間軸
（現況是文字列表）。

#### Review 發現（2026-08-21，R1～R4／R6～R8 已修，R5 待處理）

`/code-review` 對上面這批異動的發現，逐筆對照過程式碼與 `api-reference.md` 後確認成立。
**這些是本筆 review 的待辦，不是獨立 issue**——收斂 T-041 的 Event Timeline 面向之前
要逐筆處理掉；若屆時決定不修而是接受，就要搬到 [`issue.md`](./issue.md) 記成已知限制，
不能隨本筆一起消失。

**R1.（已修，2026-08-21）`decision_visible` 沒被讀出來（要動後端，contract 異動）。**
[`api-reference.md`](./api-reference.md)「GET /sr-zones/event-timeline」明寫
`SUPPORT_RETEST` 與 `RESISTANCE_BREAKOUT` 的鏈會在這個端點回傳但帶
`decision_visible=false`，「前端若要呈現，要把這個旗標一起讀出來區分，不要當成會影響
Bias 或進場的事件」。實際上 `analysis.EventTimelineChain` **沒有這個欄位**，
`SREventTimeline.svelte` 也就直接把 `event_family` 印出來——畫面會在 decision summary
正下方顯示一條 `RESISTANCE_BREAKOUT / CONFIRMED / BULLISH`，讀起來像突破買訊，
而引擎（`sr_zones.go` 的 `eventDecisionVisible`）刻意把它排除在所有決策桶之外。
修法：後端 `EventTimelineChain` 補 `decision_visible`（比照既有慣例，缺鍵視為 `true`），
前端據此把不可見的鏈標成「事實紀錄，不參與決策」。**這是對外 contract 異動，
要先出計畫書**，並同步更新 `api-reference.md`。

**R2.（已修，2026-08-21）Timeline 被 `hasDecisionDetail` 連坐隱藏。**
元件掛在 `SRZones.svelte` 的 `{#if decisionSummary && hasDecisionDetail}` 區塊內，
而 `hasDecisionDetail` 要求 `market_regime` **且** `confidence_explanation` 都存在。
事件鏈與 decision summary 無關，卻會在缺任一者的分析上整個消失（例如舊分析、
normalized decision 為 missing 的分析），身分層明明有鏈也看不到。
順帶：內層的 `{#if current}` 是死碼——`decisionSummary = current?.decision_summary`，
非 null 就代表 `current` 存在。修法：移到該 `{#if}` 之外，由 `current` 自己守衛。

**R3.（已修，2026-08-21）首次載入失敗後換股票不會重抓。**
`SREventTimeline.svelte` 的 reactive 守衛是 `if (symbol && loadedFor && …)`，
而 `loadedFor` 只在**成功**時才賦值。情境：展開 2330 → 請求 500 → `error` 有值、
`loadedFor` 仍是 `''` → 改看 6182 時守衛因 falsy 直接跳過，既不清 `error` 也不重抓，
面板持續顯示 2330 的錯誤訊息，直到手動收合再展開。
修法：失敗時也記下這次載入的目標，或把守衛改成比較「目前想看的」與「已載入的」。

**R4.（已修，2026-08-21）快取鍵只有 `symbol:timeframe`，重跑分析後不更新。**
`runAnalysis()` 直接以新分析覆寫 `current`、中途不會變 `null`，元件實例不重建；
`ensureLoaded()` 因 `loadedFor` 沒變直接 return，面板仍顯示舊的鏈與「這段期間共 N 次分析」，
畫面上沒有任何過期提示。修法：把 `current.id` 併進快取鍵。

**R5.（已實作／待 review，2026-08-21）`identity_since` 在視窗被截斷時會說謊——但錯的是後端不是文案。**
前端那句「更早的分析沒有事件鏈（刻意不回填）」忠實照著 `api-reference.md` 第 6 點寫。
落差在實作：`identity_since` 取的是**回傳鏈中最早的 `first_seen`**，而
`ListChains` 的視窗只保證**未終結**的鏈不受限制，視窗（前端固定 `max_analyses=60`）
之前就已終結的鏈會被濾掉。於是半年歷史、60 次分析只涵蓋一個月時，畫面會宣告
「身分層自 07-22 起有紀錄」，而更早的分析其實有鏈。
**這筆屬於 T-051 交付的後端行為與文件不一致**，不在前端修。
**2026-08-21 依下方「R5 計畫書」實作完成**：`identity_since` 改由
`EventIdentityRepo.GetIdentitySince` 對全歷史查出來，前端文案不動。

**R6.（已修，2026-08-21）`srZones.ts` 的註解被插隊孤立。** 新的 Event Timeline 區塊插在
「limit 預設 20；未指定 symbol 時呼叫端應給更大的值……」這段註解與它描述的
`listSRZoneAnalyses` 之間，現在那段註解讀起來像在講 Event Timeline。
修法：把新區塊移到 `listSRZoneAnalyses` 之後。

**R7.（已修，2026-08-21）`getEventTimeline` 的 `timeframe` 沒有 `encodeURIComponent`。** 同一行的 `symbol`
有編碼。目前實際值是 `1d` / `5m`，屬於同一行內不一致的防護。

**R8.（已修，2026-08-21）`splitChains` 用 `localeCompare` 比 RFC3339 字串。** Go 的 `time.Time` 在小數秒
為 0 時不輸出小數部分，比較 `…T13:30:00.123456+08:00` 與 `…T13:30:00+08:00` 會落在
`.` 對 `+` 的標點權重上；若哪天一端是 UTC、一端帶 `+08:00` 會直接錯。
影響僅止於排序，改成 `Date.parse()` 相減成本很低。

**修正結果（2026-08-21）**：純前端的 R2 / R3 / R4 / R6 / R7 / R8 六筆**已修完**；
**R1 與 R5 各依下方計畫書實作完成**（R1：後端補 `event_instances.decision_visible`
並逐條回傳，前端標記而不隱藏；R5：`identity_since` 改由身分層全歷史推導）。
**八筆全部已實作，待 review**。

修法重點：

* R2 把 `<SREventTimeline>` 移出 `{#if decisionSummary && hasDecisionDetail}`，改掛在
  該區塊之後（外層 `{#if current}` 已保證非 null，順手拿掉那個死碼守衛）。
* R3 / R4 併成同一組：元件新增 `analysisId` prop，快取鍵改成
  `symbol:timeframe:analysisId`，且 `loadedFor` 在 `finally` 裡**成功失敗都寫入**。
* R8 改用 `Date.parse()` 相減，無法解析的排到最後；新增兩條單元測試
  （小數秒／時區偏移混用、無法解析的值）鎖住行為。

驗證：`npm run test:unit` 20 檔 **133** 條全過（原 131 ＋ R8 的 2 條）、
`svelte-check` 0 errors / 0 warnings、`npm run build` 重建 dist 後
`scripts/check-dist-assets.sh` 通過（`index-ZJ7kKgIi.js` / `index-1XCtV4KW.css`）。
**尚未在真實資料上重看畫面**——R2 的位置調整與 R4 的重跑後更新要在 dev stack 上確認。

顯示端的現況規格（擺放位置、快取鍵、失敗後重抓）已同步到
[`sr-zone-scoring.md`](./sr-zone-scoring.md)「前端 Event Timeline 的判讀規則（現況）」。

#### R1 計畫書：event-timeline 的 chain 補 `decision_visible`（已實作／待 review）

跨 Go／DB／前端且動到對外 contract，依 CLAUDE.md 先留計畫書。
**2026-08-21 已依本計畫實作完成，實作結果見本節最後的「實作結果」。**

##### 修改目標

讓 `GET /sr-zones/event-timeline` 的每一條 chain 帶出 `decision_visible`，前端據此把
「只寫不讀的事實紀錄」與「會影響 Bias／進場的事件」在畫面上分開。目前
`SUPPORT_RETEST` 與 `RESISTANCE_BREAKOUT` 兩個 family 的鏈會與其他鏈長得一模一樣，
使用者會把 `RESISTANCE_BREAKOUT / CONFIRMED / BULLISH` 讀成突破買訊，
而引擎刻意把它排除在所有決策桶之外——**這正是階段 D 的隔離在顯示層漏掉的一段**。

##### 不做的範圍

* **不改 Python。** 旗標已由 `event_engine.EVENT_TYPE_META` 單一產生，本筆只是把它
  帶到顯示層。
* **不改任何決策路徑**，不動 `market_event_states` / `market_event_detections`
  的寫入與既有桶構建。
* **不把不可見的鏈藏起來。** 它們是事實紀錄，要看得到——本筆做的是**標記**不是過濾。
  藏起來會讓「這個 zone 最近有沒有被測試過」這種人工判讀失去依據。
* **不處理 R5**（`identity_since` 的視窗截斷），那是另一條線（已於 2026-08-21 另案完成，
  見下方「R5 計畫書」）。
* 不做 T-041 的 Lifecycle 與 Strategy Layer 兩個面向。

##### 受影響檔案與資料流

資料流（現況 → 目標）：

```text
Python event_engine.EVENT_TYPE_META（旗標的唯一產生者）
        ↓ state_json.decision_visible
market_event_states
        ↓ buildEventIdentityWrite（目前**沒有**把旗標帶進身分層）
event_instances  ←── 本筆要補的斷點
        ↓ BuildEventTimeline
GET /sr-zones/event-timeline → 前端
```

| 檔案 | 內容 |
|---|---|
| `backend/internal/database/migrations/{postgres,sqlite,mysql}/071_event_decision_visible.sql` | `event_instances` 新增 `decision_visible BOOLEAN NOT NULL DEFAULT TRUE` ＋ 一次性回填。編號 `071`（070 已被 `sr_identity_stats` 佔用），三份 engine 各一份 |
| `backend/internal/store/event_identity_repo.go` | `EventInstance` 加欄位；`instanceUpsertSQL()` 的 cols／VALUES／三個 engine 的 UPDATE 子句各加一行；`listEventChainsSQL` 與 `ListLive` 的 SELECT 加欄位 |
| `backend/internal/api/handler/sr_zones.go` | `buildEventIdentityWrite` 用**既有的** `eventDecisionVisible(state.StateJSON)` 填值（該函式已存在於同檔 1454 行，缺鍵回傳 `true`） |
| `backend/internal/analysis/event_timeline.go` | `EventTimelineChain` 加 ``DecisionVisible bool `json:"decision_visible"` `` |
| `frontend/src/lib/api/srZones.ts` | `SREventTimelineChain` 加 `decision_visible?: boolean` |
| `frontend/src/lib/utils/eventTimeline.ts`（＋測試） | 新增 `isDecisionVisible(chain)`：缺鍵視為 `true` |
| `frontend/src/components/sr/SREventTimeline.svelte` | 不可見的鏈加標籤「事實紀錄・不參與決策」並淡化；標題列的計數在有不可見鏈時補「（其中 N 條不參與決策）」 |

##### Contract 變化與仲裁順序

1. **旗標仍由 Python 單一產生，Go 只讀不推導。** 這條沿用 `carried_from_previous` 的
   定案：Go 或前端自己維護一份型別／family 清單時，兩份分歧不會有任何東西報錯。
   所以**不接受**「Go 依 `event_family` 判斷」或「前端硬編兩個 family 名字」這兩種寫法。
2. **缺值一律 `true`。** 與 `eventDecisionVisible` 現有語意、`api-reference.md` 的
   「缺鍵時也視為 `true`」一致；當成 `false` 會讓既有事件整批被標成不參與決策。
3. **JSON tag 不可加 `omitempty`。** `false` 會被 `omitempty` 整個吃掉，而 `false`
   正是這個欄位唯一有資訊量的值——加了等於白做。（同樣的坑 `ZoneUID` 用指標避開過。）
4. **寫入時機**：每次事件被觀測到就跟著 upsert 更新。鏈由排程收尾（沒有本次 state）時
   不會走這條路徑，欄位維持既有值——實作時要確認收尾路徑不會把它寫成預設值。

##### 回填策略

新欄位預設 `TRUE`，但 2026-08-20 階段 D 之後寫進去的 `SUPPORT_RETEST` /
`RESISTANCE_BREAKOUT` 鏈會因此被標成「參與決策」，且**已終結的鏈不會再被寫入、
不會自動修正**。所以 migration 內做一次性回填：

```sql
UPDATE event_instances SET decision_visible = FALSE
 WHERE event_family IN ('SUPPORT_RETEST', 'RESISTANCE_BREAKOUT');
```

這行**是資料修正、不是執行期推導**，要在 migration 註解裡寫明，避免下一個人把它當成
「Go 側可以照 family 判斷」的先例。不走「join `market_event_states` 的 `state_json`
回填」是因為三個 engine 的 JSON 取值語法各不相同，而回填母體小又已知。

##### 主要風險與回滾

| 風險 | 對策 |
|---|---|
| 前端把旗標讀成「要隱藏」 | 定案是**標記不隱藏**，寫進 `sr-zone-scoring.md` 的顯示規則；元件測試鎖住「不可見的鏈仍然出現在列表裡」 |
| 三份 migration 分歧（mysql 從未部署） | 比照既有慣例跑 `scripts/test-mysql-migrations.sh`；注意它只驗 DDL 不驗 CRUD（`issue.md` I-054） |
| upsert 漏改某個 engine 的 UPDATE 子句 | 新欄位在 sqlite 測試裡做「寫入 false → 重新觀測 → 仍為 false」的 round-trip |
| 舊前端搭新後端／新前端搭舊後端 | 純新增欄位；前端型別是 optional 且缺鍵視為 `true`，兩個方向都不會壞 |
| 回滾 | Down 移除欄位即可；API 欄位消失後前端走「缺鍵＝true」分支，退回本筆之前的顯示行為 |

##### 測試與驗證策略

1. Go 單元測試：`buildEventIdentityWrite` 對帶 `decision_visible=false` 的 state 寫出
   `false`、缺鍵寫出 `true`；timeline handler 的 JSON 逐欄比對含 `decision_visible`。
2. `backend/scripts/test.sh`（sqlite，含 repo 層 round-trip）。
3. `scripts/test-postgres-migrations.sh` 與 `scripts/test-mysql-migrations.sh`。
4. dev stack 的 as-of 階梯（四檔 21 階，步驟見
   [`development-workflow.md`](./development-workflow.md)）：`SUPPORT_RETEST` /
   `RESISTANCE_BREAKOUT` 的鏈全為 `false`、其餘全為 `true`；**決策輸出逐欄不變**
   仍是驗收條件（本筆不碰決策，六條門檻要全過）。
5. 前端：`npm run test:unit`＋`svelte-check`＋`npm run build`，並在 `6182` 上看畫面
   （該檔有 8 條 `ZONE_IDENTITY_ENDED`、涵蓋兩個新 family）。
6. 回填後對 dev DB 用唯讀 SQL 抽驗兩個 family 的列數與旗標值。

##### 完成後歸檔位置

* [`api-reference.md`](./api-reference.md)「GET /sr-zones/event-timeline」：欄位表補
  `decision_visible`，並把「前端若要呈現，要把這個旗標一起讀出來區分」從待辦語氣改成現況。
* [`database-schema.md`](./database-schema.md)「event_instances / event_transitions /
  zone_key_aliases」：新欄位與回填說明。
* [`sr-zone-scoring.md`](./sr-zone-scoring.md)「事件的決策可見性」補上「旗標如何流到顯示層」，
  「前端 Event Timeline 的判讀規則（現況）」補上顯示方式（標記不隱藏）。

##### 實作結果（2026-08-21）

實作與計畫一致，沒有偏離。落地內容：

| 層 | 落地 |
|---|---|
| migration | `071_event_decision_visible.sql` × 3 engine，`BOOLEAN NOT NULL DEFAULT TRUE`（sqlite 為 `1`）＋ 兩個 family 的一次性回填 |
| store | `EventInstance.DecisionVisible`；`instanceUpsertSQL()` 的 cols／VALUES ＋ 三個 engine 的 UPDATE 子句；`listLatestEventChainsSQL` 與 `listEventChainsSQL` 的 SELECT |
| handler | `buildEventIdentityWrite` 用既有的 `eventDecisionVisible(st.StateJSON)` 填值（缺鍵 `true`） |
| analysis | `EventTimelineChain.DecisionVisible`，JSON tag **無 `omitempty`** |
| 前端 | `SREventTimelineChain.decision_visible?`、`isDecisionVisible()`（缺鍵 `true`）、元件標「事實紀錄・不參與決策」並淡化、標題列補「（其中 N 條不參與決策）」 |

**已通過的驗證**：

* `backend/scripts/test.sh ./internal/store/... ./internal/analysis/... ./internal/api/...`
  全過（含新增的 upsert round-trip、`buildEventIdentityWrite` 缺鍵／`false` 兩案、
  timeline handler 的 JSON 逐欄比對）。
* `scripts/test-postgres-migrations.sh`、`scripts/test-mysql-migrations.sh` 皆通過
  （up → 驗 schema → 分段 down 到 0）。
* 前端 `npm run test:unit` 21 檔 **137** 條全過（原 133 ＋ `isDecisionVisible` 2 條
  ＋ 元件的「標記不隱藏」／「缺鍵不標記」2 條）、`svelte-check` 0 errors / 0 warnings、
  `npm run build` 重建 dist 後 `scripts/check-dist-assets.sh` 通過
  （`index-3iMGK8fu.js` / `index-1XCtV4KW.css`）。

**尚未做的驗證（review 時要補或明確放行）**：計畫第 4～6 點的 dev stack 驗收。
**2026-08-21 已把新 image 換上 dev backend，migration 071 也已套到 dev DB
（啟動 log `migrations applied version=71`）**，但回填後的抽驗、as-of 階梯的
「兩個 family 全 `false` / 其餘全 `true`」、決策輸出逐欄不變，以及在 `6182` 上
實看畫面，都還沒做。

#### R5 計畫書：`identity_since` 改由身分層全歷史推導（已實作／待 review）

動到對外 contract 的語意並跨 store／handler／analysis，依 CLAUDE.md 先留計畫書。

##### 修改目標

讓 `identity_since` 永遠等於「**身分層對這檔最早有紀錄的時間**」，不受 `max_analyses`
視窗影響，與 `api-reference.md` 判讀第 6 點、`sr-zone-scoring.md` 的敘述一致。
目前它取自「本次回傳 chains 中最早的 `first_seen`」（`event_timeline.go:206`），
而 chains 已先被視窗篩過（`sr_zones.go:974`），於是視窗之前就終結的鏈被濾掉時，
畫面會宣告「更早的分析沒有事件鏈」，實際上只是「這次沒查到」。

修好之後前端那句「身分層自 X 起有紀錄；更早的分析沒有事件鏈（刻意不回填）」才成立。

##### 不做的範圍

* **不改 `chains` / `snapshots` 的視窗語意**：`max_analyses` 仍決定回傳哪些鏈，
  判讀第 4 點不變。本筆只修 `identity_since` 這一個值。
* **不改前端**：文案、快取鍵、排序都不動。
* **不回填歷史**：`identity_since` 之前仍然沒有鏈可看，那是身分層的刻意選擇。
* **不新增 migration**：純查詢，無 DDL 變更。
* 不動任何決策路徑。
* 不處理 `scripts/verify-event-timeline.sh` 對 live schema 缺 `decision_visible` 的相依，
  那是另一條線。

##### 受影響檔案與資料流

| 層 | 修改 |
|---|---|
| store | `EventIdentityRepo` 介面新增 `GetIdentitySince(ctx, symbol, timeframe) (sql.NullTime, error)` ＋ `eventIdentityRepo` 的實作 SQL |
| handler | `SRZoneHandler.EventTimeline` 在既有兩次查詢之外多查一次全域 `identity_since`，結果傳進 `BuildEventTimeline` |
| analysis | `BuildEventTimeline` 新增 `identitySince *time.Time` 參數：非 nil 直接採用；nil 時維持既有「由 chains 推導」的降級路徑 |
| 測試 | store（sqlite 五案）、analysis（注入優先）、handler（視窗截斷回歸） |
| 文件 | `api-reference.md` 判讀第 6 點、`sr-zone-scoring.md` 對應段落 |

資料流（目標）：

```text
event_instances（全歷史，**不套視窗**）
   └─ 每條鏈的起點 = MIN(COALESCE(a.analyzed_at, t.occurred_at))
        （無 analysis_id 退回 occurred_at；完全沒有 transition 才退回 e.first_seen_at）
        └─ 全域 MIN → identity_since（K 棒軸）
```

##### 查詢

**計畫時的寫法（單一聚合查詢，已作廢）**：

```sql
SELECT MIN(x.started_at) FROM ( ... COALESCE(MIN(COALESCE(a.analyzed_at, t.occurred_at)), e.first_seen_at) ... ) x
```

**實作時的偏離（2026-08-21）**：上面那支在 sqlite 直接炸——modernc 的 driver 只對
「宣告型別是 DATETIME 的欄位」回 `time.Time`，**聚合／`COALESCE` 這類運算式沒有宣告型別、
會回字串**，掃進 `sql.NullTime` 得到
`unsupported Scan, storing driver.Value type string into type *time.Time`。
改成**兩支查詢、`SELECT` 只挑真欄位、把運算式留在 `ORDER BY`**，三個 engine 都不必解析字串
（語意、contract 與計畫完全相同，只有查詢形狀變了）：

```sql
-- 1. 全域最早的那一步（「每條鏈取最早一步再取全域最小」＝「全域最早的一步」，不需分組）
SELECT a.analyzed_at AS analyzed_at, t.occurred_at AS occurred_at
  FROM event_transitions t
  JOIN event_instances e ON e.event_uid = t.event_uid
  LEFT JOIN stock_sr_zone_analyses a ON a.id = t.analysis_id
 WHERE e.symbol = ? AND e.timeframe = ?
 ORDER BY COALESCE(a.analyzed_at, t.occurred_at) ASC
 LIMIT 1;

-- 2. 完全沒有轉換的鏈（寫入端異常，但仍要算進來）
SELECT e.first_seen_at
  FROM event_instances e
 WHERE e.symbol = ? AND e.timeframe = ?
   AND NOT EXISTS (SELECT 1 FROM event_transitions t WHERE t.event_uid = e.event_uid)
 ORDER BY e.first_seen_at ASC
 LIMIT 1;
```

兩支的較早者即 `identity_since`；都沒有列（`sql.ErrNoRows`）時回 `Valid=false`。

* **與 `BuildEventTimeline` 的 `firstSeen` 規則同構**（優先 K 棒軸 → 無 analysis 退
  wall clock → 無 transition 退 `first_seen_at`）。同一個時間點不能有兩套推導，
  否則 `identity_since` 會與 `chains[0].first_seen_at` 對不起來。
* **不能只寫 `MIN(e.first_seen_at)`**：那是 as_of 的 wall clock，與對外的 K 棒軸不同軸
  （T-045 曾把它由 `2026-08-20` 修成 `2026-07-20`，見本檔 T-045 的驗收紀錄）。
* 索引：外層走 `idx_event_instances_live (symbol, timeframe, …)`，相關子查詢走
  `idx_event_transitions_event (event_uid, occurred_at)`；單檔是數十列的量級，
  每次請求多一次聚合查詢的成本可接受。

##### Contract 變化與仲裁順序

| 項目 | 變化 | 相容性 |
|---|---|---|
| `identity_since` 值 | 由「視窗內最早鏈的起點」→「全歷史最早鏈的起點」，**只會變早不會變晚** | 鍵名、型別、時間軸都不變，前端不需改 |
| 沒有任何鏈時 | 仍為 `null` | 不變 |
| `chains` / `snapshots` | 完全不變 | 不變 |
| repo 介面 | 新增一支方法 | 只有 `eventIdentityRepo` 一個實作，測試用真 repo 打 sqlite，沒有 fake 要跟著補 |

仲裁順序：handler 查到的全域值**優先於**由 chains 推導的值；只有未注入
`eventIdentity`（值為 nil）時才退回推導，而那條路徑下 chains 必為空、推導結果也是 nil。

##### 主要風險與回滾

* **多一次查詢**：見上面的索引分析。回滾只需把 handler 那幾行拿掉、`BuildEventTimeline`
  傳 nil，行為即回到現況；無 schema 變更、不需要 down migration。
* **`BuildEventTimeline` 多一個參數**：既有 9 個呼叫點（含 live test）要補參數，
  保留 nil 降級後語意不變，編譯期就會抓到漏改。
* **值變早之後畫面會多出一段「有分析、沒有鏈」的區間**：那正是誠實的表達，
  `snapshots` 本來就照常列出，判讀第 6 點已寫明。

##### 測試與驗證策略

* store（sqlite，`event_identity_repo_test.go`）五案：
  1. 有 `analysis_id` → 取 `stock_sr_zone_analyses.analyzed_at`；
  2. transition 沒有 `analysis_id` → 退回 `occurred_at`；
  3. 鏈完全沒有 transition → 退回 `first_seen_at`；
  4. 這檔沒有任何鏈 → 回 `NULL`（`sql.NullTime.Valid=false`）；
  5. **關鍵回歸**：一條早已 `ended_at` 而會被 `ListChains(since)` 濾掉的舊鏈，
     仍要被算進 `identity_since`。
* analysis（`event_timeline_test.go`）：注入值早於 chains 推導值時要採用注入值；
  注入 nil 時維持既有推導（既有測試語意不動，只補參數）。
* handler（`sr_zones_create_test.go` 或 `sr_zones_event_identity_test.go`）：
  端點在「舊鏈被視窗濾掉」的情境下，回傳的 `identity_since` 早於 `chains[0].first_seen_at`。
* `backend/scripts/test.sh ./internal/store/... ./internal/analysis/... ./internal/api/...`
* 不跑 migration 測試（無 DDL）。前端不動，不需要重建 dist。
* dev stack 實看：待 `verify-event-timeline.sh` 的資料來源路線定案後補。

##### 完成後歸檔位置

* `docs/api-reference.md` 判讀第 6 點補「`identity_since` **不受 `max_analyses` 影響**，
  它問的是身分層何時開始有紀錄，不是這次查了多久」。
* `docs/sr-zone-scoring.md` 對應段落同步成現況說明。
* review 通過後，把 R5 連同本計畫書整筆從 `todo.md` 移除。

##### 實作結果（2026-08-21）

| 層 | 實際落點 |
|---|---|
| store | `EventIdentityRepo.GetIdentitySince`；兩支查詢見上（`identitySinceStepSQL` / `identitySinceOrphanSQL`），都沒有列時回 `Valid=false` |
| handler | `SRZoneHandler.EventTimeline` 在 `h.eventIdentity != nil` 時多查一次，結果傳進 `BuildEventTimeline` |
| analysis | `BuildEventTimeline` 新增 `identitySince *time.Time`（第 6 個參數）；非 nil 直接採用，nil 時維持由 chains 推導的降級 |
| 前端 | **未改動**，文案照舊 |
| 文件 | `api-reference.md` 判讀第 6 點補「不受 `max_analyses` 影響」與推導規則；`sr-zone-scoring.md` 的顯示規則同步 |

**與計畫的差異**：查詢形狀由「單一聚合」改成「兩支、`SELECT` 只挑真欄位」，原因見上方
「實作時的偏離」（sqlite driver 對運算式回字串）。語意、contract、仲裁順序都與計畫相同。

**已通過的驗證**：`backend/scripts/test.sh ./internal/store/... ./internal/analysis/...
./internal/api/...` 全過，含新增的 8 條測試——

* store 5 條：`analyzed_at` 優先／無 `analysis_id` 退 `occurred_at`／無轉換退
  `first_seen_at`／沒有鏈回 NULL／**視窗外的已終結舊鏈仍要算進去**（先斷言
  `ListChains` 確實濾掉它，否則測試什麼都沒證明）。
* analysis 2 條：注入值優先於推導；未注入時維持舊行為。
* handler 1 條：走完整 HTTP 路徑，`max_analyses=1` 時回傳的 `identity_since`
  早於 `chains[0].first_seen_at`。

**尚未做的驗證**：dev stack 實看畫面。`scripts/verify-event-timeline.sh` 目前連 live DB，
而 live 還沒有 migration 071 的 `decision_visible` 欄位（`ListChains` 會 42703），
要等資料來源路線定案；本筆不含 DDL，不需要 migration 測試。

---

### T-042：股價還原的收尾工作

| 欄位 | 內容 |
|---|---|
| 狀態 | 待處理（主體已完成並驗證，2026-08-11；**兩項小的已於 2026-08-18 實作，待 review**，只剩「逐檔事件的增量更新」） |
| 優先度 | 中 |
| 分類 | Go / Python / 資料正確性 |
| 建立日期 | 2026-08-11 |
| 來源 | 原 T-042 的收斂結果 |

**主體（Phase 1 分割 ＋ Phase 2 除權息）已上線並在 live 驗證通過**，現況說明見
[`database-schema.md`](./database-schema.md) 的「股價還原」與 `corporate_actions`、
[`architecture/data-pipeline.md`](./architecture/data-pipeline.md) 的「公司行動同步」、
[`api-reference.md`](./api-reference.md) 的 `/candles/:symbol` 與
`/scheduler/corporate-action-sync/run`。驗證用 `scripts/verify-adjustment.sh`（六項檢查全過）。

live 實證：0050 跨 2025-06-18 分割的價格落差由 **−74.8% 降到 +0.86%**；
2603 在 2023-06-30 的 **−39.7%** 大額配息跳空也被除權息還原修掉。

以下是**刻意留待後續**的項目（2026-08-18：兩項小的已實作／已盤查，只剩第一項）：

| 項目 | 說明 |
|---|---|
| **逐檔事件的增量更新** | 除權息與減資都是逐檔、目前每次全抓。除權息走 Yahoo（20/分）；**減資走 FinMind（5/分），與每日抓價共用節流器**——1,900 檔光減資就要 6.3 小時且會排擠行情抓取。這是 T-040 擴標的池的**前置條件**，不是日後優化 |
| ~~模型用未還原資料訓練~~ | **已驗證，不需重訓**（2026-08-11）。A/B 實測邊際分布沒有位移（`confidence` 與 `trading_score` 的中位數幾乎相同），個別決策改變 1.9%～5.4% 屬於還原修正錯誤輸入。結論與方法見 [`sr-zone-scoring.md`](./sr-zone-scoring.md) 的「模型與還原股價的相容性」 |
| ~~`corporate_action_sync` 的 cron 寫死~~ | **已實作，待 review**（2026-08-18）：改走 `corporate_action.cron`（環境變數 `CORPORATE_ACTION_CRON`），預設值等於原本的硬編碼 `"30 6 * * 1-5"`，**行為不變**。現況說明見 [`architecture/data-pipeline.md`](./architecture/data-pipeline.md)「公司行動同步」 |
| ~~Python 的 volume 變 float~~ | **已盤查，結論是不改行為，待 review**（2026-08-18）：下游全部以 float 取用、原始 volume 不跨 Python→Go 邊界，**沒有假設整數的消費者**。補 9 支測試鎖住現況，說明見 [`database-schema.md`](./database-schema.md)「股價還原」 |
| ~~減資未涵蓋~~ | **已實作並在 live 驗證通過**（2026-08-11）：7 筆減資事件，三筆假跳空（+126.8% / +109.2% / +36.3%）全部消失。合併與下市重編仍無來源，見 [`database-schema.md`](./database-schema.md)「未涵蓋的公司行動」 |

### T-043：盤後用 Yahoo 批次補日 K（價格），成交量仍走 FinMind

| 欄位 | 內容 |
|---|---|
| 狀態 | 待規劃 |
| 優先度 | 低（**不是** T-040 的前置條件，見下方「與 T-040 的關係」） |
| 分類 | Go / 資料抓取 |
| 建立日期 | 2026-08-11 |
| 來源 | 2026-08-11 對 `FinanceChartService.ApacLibraCharts` 的可行性評估；2026-08-12 與 T-040 的合併評估修正定位 |

完整評估與實測數字見
[`yahoo-intraday-integration.md`](./yahoo-intraday-integration.md) 的
「盤後聚合成日 K 的可行性評估」。摘要：

- **價格**：Yahoo 分K 聚合後與 FinMind 日 K **完全一致**（10 檔、最大差異 0.0000）。
- **成交量**：單位是張，換算後仍系統性短少 0.3%～7.8%，**不可用**。
- **批次**：至少 50 檔／請求，耗時與檔數幾乎無關。1,900 檔約 2 分鐘，
  FinMind 逐檔要 95 分鐘。

#### 與 T-040 的關係：幫不上，不要綁在一起做

本筆原本寫「價值在擴標的池、逐檔抓日 K 是主要瓶頸」。**那個前提是錯的**
（2026-08-12 對照程式碼查證後修正）：

1. **這個端點取不到歷史日 K**。它是當日分 K 專用，唯一用法是盤後把**今天**的分 K
   聚合成一根日 K。而 T-040 的兩筆抓取成本全是歷史回補——Step 1 是 650 檔 × 130 天、
   Step 3 是 150 檔 × 5 年，Yahoo 一天都補不了。
2. **日常維護那 30 分鐘也省不下來**。`FetchDailyCandles`（`market/finmind.go:182`）
   一次請求就回傳 O/H/L/C/**Volume/Amount**，且**帶日期區間與單日同價**。只要還需要成交量，
   那一個 FinMind 請求就跑不掉；從 Yahoo 另外抓價格不會減少任何 FinMind 請求。
3. **evaluation pipeline 硬性需要成交量**。`evaluation.py:92` 直接取
   `["open","high","low","close","volume"]`；`relative_volume` 是模型特徵，
   `volume_confirmation` 與 `EXTREME_VOLUME_THRESHOLD` 都是**門檻型**判斷，
   0.3%～7.8% 且與標的相關的偏差足以讓它們失真。T-040 Step 3 的流動性下限還要用
   平均成交金額，而 Yahoo 連 `Amount` 都沒有。

**更上層的結論**：卡住標的池規模上限的是**成交量**，不是價格。全市場 ＋ 完整量能
在 FinMind 現行額度（600 requests/小時）下無解，那是比 T-040 更上層的策略問題，
不是本筆能解的。

#### 那本筆的價值在哪

不在吞吐量，在**時效性**與**未來的價格層級掃描**：

1. **當日價格的前哨與交叉檢核**。`scheduler.go:116` 記錄了 FinMind 當日日 K 不會在收盤當下
   發布，曾在 14:00 拉到 `count=0` 且**靜默成功**（空陣列 BulkInsert 視為成功、`job_runs`
   顯示 success，沒有任何錯誤訊號）。Yahoo 盤後聚合可以在 13:30 收盤後立刻拿到當日價格，
   用來提前偵測「FinMind 這批是空的」並交叉檢核價格——這是 FinMind 給不了的。
2. **未來全市場的價格層級掃描**（CLAUDE.md Roadmap Phase 2 的 1,900 檔）。日常維護在
   FinMind 5/min 下要 6.3 小時／天不可行，Yahoo 約 2 分鐘。但**僅在可接受無量能確認的
   用途上成立**——突破偵測的三個條件裡有一項就是 `Volume > AvgVolume(20)`。

**要處理的問題**：

1. **一檔不存在的代號會讓整批 404**。抓取前必須用 `stock_symbols.is_listed` 過濾，
   且整批失敗時要能退成小批或逐檔以定位問題檔。
2. **價量來源不同**會讓 `candles` 的一列由兩個來源組成。要決定：
   是先寫價格後補量、還是等兩者齊備才寫？前者會讓量能指標在補齊前算錯。
3. 成交量短少的成因未確定（零股／盤後定價／取整）。若查明是可修正的，
   或許能單一來源解決。
4. 跨日穩定性與冷門股行為未驗證（目前只測過單一交易日、10 檔）。

**與既有工作的關係**：這個端點就是 `yahoo.base_url`（盤中源，`enabled: false`）。
盤中源的實盤驗證是 T-032、批次失敗的 fallback 是 T-031，兩者都擱置中；
本筆若要動，會先碰到同一組風險（非官方 API、封鎖風險）。

---

### T-045：Event Timeline——把事件狀態改成完整事件鏈

| 欄位 | 內容 |
|---|---|
| 狀態 | P1/P2 已實作，review 已收斂（2026-08-13）；前端顯示與 runtime chain 另案 |
| 優先度 | 中 |
| 分類 | Python / Go / SR Zone / 決策資料 |
| 建立日期 | 2026-08-13 |
| 來源 | 使用者需求；T-041 的 Event Timeline 面向獨立成案 |

**目標**：把「目前有哪些事件」改成「事件如何一路演進到現在」——完整事件鏈，
看得出何時開始、測試、確認、失敗、過期或被後續事件取代，並保留順序與轉換脈絡。

> **與 Lifecycle Engine 的關係**（原 T-044，**已於 2026-08-18 收斂並移出本清單**，
> 現況規格見 [`sr-zone-scoring.md`](./sr-zone-scoring.md)「分層原則：lifecycle 不看 RR」）：
> Lifecycle Engine 的職責是「依 **Event 的演進** 決定狀態」，而演進的載體就是 timeline。
> **它目前是 snapshot-based**——讀的是「當前狀態的快照」而不是演進，因為 runtime chain
> 尚未接進 Python 分析流程。要讓它真正吃到演進，缺的是本筆的 `runtime_chain`（見下方
> 「與 Lifecycle Engine（原 T-044）的接縫」）。
> **本筆內文其餘的 T-044 字樣一律指這個已完成的 Lifecycle Engine**（`lifecycle_engine.py`），
> 不是待辦項目。

#### 為什麼現在「只有兩個 event」

不是顯示層的問題，是**資料模型結構上就沒有鏈**：

1. `build_event_state_summary`（`event_engine.py:298`）以 `(zone_key, event_family)` 為鍵，
   **新事件直接整筆覆寫** `states[key] = state`。同一家族的前一個事件連同它的
   `root_event_type` 一起被蓋掉——`root_event_type` 在覆寫時被設成新事件的 type，
   **根本不是 root**。
2. 摘要只輸出**當前**分桶（`active` / `candidates` / `confirmed` / `resolved` / `expired`），
   沒有任何時間序列或轉換紀錄。前端看到的「兩個 event」通常就是
   `active` 一個 ＋ `candidates` 一個。
3. 唯一的鏈狀痕跡是 `resolved_by` 與 `latest_event_type`——只夠表達「被誰終結」，
   不足以還原順序。

#### 關鍵發現：鏈其實已經在 DB 裡

`market_event_states`（migration 042）**每一次分析都寫入一份完整狀態快照**，
欄位含 `analyzed_at`、`event_key`、`root_event_type`、`latest_event_type`、`state`、
`active`、`resolved_by`、`state_json`，並有 `(symbol, timeframe, active, analyzed_at DESC)` 索引。
另有 `market_event_detections` 記錄每次分析偵測到的原始事件。

**所以 timeline 不需要新資料表**：把同一 `(symbol, timeframe, zone_key, event_family)` 的
快照序列依 `analyzed_at` 排序，**相鄰兩份快照的差異就是一次轉換**。這個作法還有一個
新增資料表拿不到的好處——**對既有資料回溯有效**，不必等新資料累積。

#### 設計

**Timeline 的單位是「事件鏈」（chain），不是「事件」**

一條 chain ＝ 同一個 `(zone_key, event_family)` 從首次出現到終結（`RESOLVED` / `EXPIRED`）
的完整歷程。同一個 zone 可以有多條 chain（不同家族），同一家族在終結後再次觸發則是**新的一條**。

```text
chain: (zone=S-142.5, family=SUPPORT_RECLAIM)
  ├─ 2026-08-03  CANDIDATE   INTRADAY_RECLAIM      reason=[CLOSE_RECLAIM]
  ├─ 2026-08-04  CONFIRMED   INTRADAY_RECLAIM      reason=[RECLAIM_AGE_1]
  ├─ 2026-08-06  ACTIVE      INTRADAY_RECLAIM      age=2
  └─ 2026-08-07  RESOLVED    HIGH_VOLUME_BREAKDOWN resolved_by=HIGH_VOLUME_BREAKDOWN
```

**轉換的判定**：相鄰快照中 `state`、`active`、`latest_event_type`、`resolved_by` 任一改變
即產生一筆 transition；完全相同則不產生（避免每日分析都塞一筆「沒事發生」）。

**分兩階段，避免一次動太多**

| 階段 | 內容 | 風險 |
|---|---|---|
| **P1：唯讀重建** | 新增 `GET /sr-zones/event-timeline?symbol=&timeframe=`，由 Go 讀 `market_event_states` 的快照序列摺疊成 chain。**不改任何寫入路徑、不改 Python** | 低。純新增查詢，錯了不影響決策 |
| **P2：修正 root 與寫入端** ✅ **已實作 2026-08-13** | 修 `build_event_state_summary` 的覆寫問題（`root_event_type` 延續）。`first_seen_at` **不做**，理由見下 | 實測**低於預估**：目前無行為改變，見下 |

**端點路徑用 query 而不是 path param**（2026-08-13 review 修正）：`server.go:163` 已有
`GET /sr-zones/:id`，同一層再放 `:symbol` 會與它衝突（gin 不允許同位置兩個不同名的 wildcard）。
而 `evaluate`、`train-jobs`、`model-status` 等靜態同層路由都與 `:id` 並存無礙，
所以 `event-timeline` 走靜態路徑 ＋ query 參數才是與既有慣例一致的作法，
日後要加 `from` / `to` / `limit` 也自然。

**P1 需要新的 repo 查詢，不能重用既有的**：`SRZoneRepo` 目前只有
`GetMarketEventStates(analysisID)`（單次分析）與 `GetLatestMarketEventStates(symbol, timeframe)`
（只取最新一批），兩者都給不出歷史序列。要新增依 `symbol, timeframe` 取一段歷史 states 的
method，並以 `analyzed_at, analysis_id, zone_key, event_family` **穩定排序**——
摺疊邏輯依賴順序決定性，排序不穩會讓同一份資料產生不同的 chain。

**索引**：migration 042 現有的 `(symbol, timeframe, active, analyzed_at DESC)` 是為
「最新 active 快照」設計的，`active` 卡在中間，不適合 timeline 依
`(symbol, timeframe, zone_key, event_family, analyzed_at)` 摺疊。三種 engine 都要補。
**但不必在 P1 就加**——live 目前整張表只有 76 列，索引的寫入成本此刻大於收益；
應等 SR 分析的執行頻率提高、資料量起來後再補，並在計畫裡記下這個判斷依據。

P1 先做的理由：**它能立刻回答「鏈長什麼樣子」**，而那正是設計 P2 與 T-044 輸入形狀所需要的
實證。先改寫入端等於在還沒看過真實鏈的情況下決定資料形狀。

#### P1 review 修正（2026-08-13）

實作後的 review 找到四個問題與一個交付完整性缺口，都已修正並補上回歸測試：

**一、MySQL 拒絕 `IN (… LIMIT ?)`**（`sr_zone_repo.go`）。MySQL 至今仍不支援
`IN/ALL/ANY/SOME` 子查詢裡直接用 `LIMIT`（ERROR 1235），該端點在 mysql 部署上會全數 500。
修法是多包一層 derived table。**這是三種 engine 都要記住的通用陷阱**——
同一個檔案的 `GetLatestMarketEventStates` 用的是純量形式 `= (SELECT … LIMIT 1)`，
那個 MySQL 允許，所以它不需要這層；正確範例就在旁邊卻選了會失敗的寫法。

**二、墓碑分支吞掉終結後的真實變化**（`event_timeline.go`）。原本只判斷「是不是終結狀態」，
所以 `RESOLVED → EXPIRED` 的老化（`_normalize_previous_event_state` 在 `age_bars` 達門檻時
會翻）會被整個吞掉，鏈的 `final_state` 永遠停在 `RESOLVED`，與 DB 最新狀態矛盾。
改為**只有與前一步完全相同才算墓碑**，有變化就接在同一條鏈上。

**三、`snapshots` 漏掉沒有事件的分析**（影響最大）。快照原本由事件狀態列推導，
但**一次沒偵測到任何事件的分析不會留下任何 state 列**。實測 0050 有 14 次分析、
只有 11 次產生事件列——漏了 21%，而 `snapshots` 正是文件宣稱「誠實揭露觀測缺口」的唯一依據。

修正後 live 實測的差異不只是多三個點：

```
修正前： 07-20(+0) 07-21(+1) 07-22(+1) 07-23(+1) 08-03(+11) …
修正後： 07-13(+0) 07-14(+1) 07-15(+1) 07-20(+5) 07-21(+1) … 08-03(+11) …
```

觀測起點從 07-20 修正為 07-13，並多出一個 `07-20(+5)` 的缺口——**漏掉的分析不只讓計數變少，
還會讓相鄰的 gap 被錯誤地合併或消失**。新增 `SRZoneRepo.ListAnalysisSnapshots` 從
`stock_sr_zone_analyses` 取所有分析，handler 一併查詢帶入。

**四、有分析但沒有任何事件時，`snapshots` 被 early return 整個丟掉**（`event_timeline.go`）。
P1 修正三之後，handler 會把所有分析傳進 `BuildEventTimeline`，但函式一開始若 `rows == 0`
就直接回傳，導致「這段期間有分析、只是沒有事件」被輸出成完全沒有觀測。修法是讓
`snapshots` 先根據 `analyses` 建出來，再回傳空 `chains`。回歸測試
`TestBuildEventTimelineKeepsAnalysisSnapshotsWhenNoEvents` 鎖住這個形狀。

**交付完整性缺口**：P1/P2 新增的 `event_timeline.go`、單元測試、live 驗證測試與
`scripts/verify-event-timeline.sh` 一度還是 untracked；本機測試會讀工作目錄，所以不會抓到
「只提交 tracked diff 後 clone/CI 找不到 `analysis.BuildEventTimeline`」這種錯。四個檔案已納入
version control（`A` 狀態）。若未來常發生類似問題，應補一個類似 `scripts/check-dist-assets.sh`
的交付檢查。

**一個過程上的觀察**：這批的單元測試從頭到尾**沒有自己抓到任何一個真 bug**——
墓碑重複問題是 live 實跑抓到的，其餘邊界與交付缺口是 review 抓到的。測試驗證的是
「我以為的行為」，而錯的正是那個「我以為」。這也是為什麼計畫裡的 live 實跑驗收項目不能省。

**收斂驗證（2026-08-13）**：`backend/scripts/test.sh ./internal/analysis ./internal/store ./internal/api/handler`
通過，`bash -n scripts/verify-event-timeline.sh` 通過，
`python/scripts/test.sh backtest/modular/sr_scoring/tests/test_event_engine.py` 通過（27 passed）。
原先記在 `docs/issue.md` 的兩筆 T-045 相關 issue（當時編號 `I-070` / `I-071`）已確認修復並移除，
保留結論改歸檔於本節。**`I-070` 這個編號後來被回收再發過一次**（給了 T-040 selection report
的 `keep_symbols` 靜默丟棄，該筆也已收斂，規格歸檔在
[`evaluation-universe-selection-plan.md`](./evaluation-universe-selection-plan.md)
「watchlist 的分級保留」）。兩者都與本節無關；編號不重複發放是原則，這裡是既成事實，
靠這段註記避免日後把三件事混在一起。

#### P2 實作結果（2026-08-13）

**修了什麼**：merge 迴圈的 `states[key] = state` 是整筆覆寫，把 `root_event_type` 設成
新偵測的 type——欄位名叫 root 卻永遠等於 latest，鏈的起點無法還原。
改為**前一狀態未終結時延續 root**，已 `RESOLVED`／`EXPIRED` 才視為新鏈。
這個邊界規則與 Go 端摺疊 timeline 的 `isClosedEventState` **刻意對稱**，
兩邊的註解互相指向對方，避免日後規則漂移。

**風險實測低於計畫預估——目前沒有任何行為改變。**
`EVENT_TYPE_META` 的四種事件類型各自對應一個獨立 family
（`EXTREME_VOLUME`→`VOLUME_CONTEXT`、`HIGH_VOLUME_BREAKDOWN`→`SUPPORT_BREAKDOWN`、
`INTRADAY_RECLAIM`→`SUPPORT_RECLAIM`、`REVERSAL_CANDIDATE`→`SUPPORT_REVERSAL`），
**一個 family 只有一種 type**，所以同一個 `(zone_key, event_family)` 的 root 與新偵測的
type 永遠相同，覆寫從來沒有真的遺失過資訊。

這一點很重要，因為 `root_event_type` **確實被 decision 消費**
（`decision_engine.py:864` 的 `_event_state_types` 與 `:877` 的 `_event_state_max_age`
都把它併進事件類型集合，而該集合直接決定 `event_signal` → `lifecycle_phase`）。
若 root 真的會與 latest 不同，這次修改就會改變決策；正因為兩者恆等，**本次修改是純粹的
正確性補強**，不需要 decision 對照測試。

**所以 P2 的價值是擋住未來**：哪一天某個 family 多出第二種 type，覆寫就會開始默默吃掉鏈的
起點並連帶影響 `event_signal`，而那時不會有任何東西報錯。兩支新測試鎖住這個規則
（其中用到的 `INTRABAR_BREAKDOWN` 是**虛構型別**，repo 裡不存在，僅用來構造該情境）。

**`first_seen_at` 刻意不做**（縮減計畫範圍）：

- Python 端的事件狀態**沒有任何時間欄位**，以 K 棒為單位運作（`age_bars`），拿不到 `analyzed_at`。
- 要持久化就得在 `market_event_states` 加欄位＝一支 migration ＋ 三種 engine 同步。
- 而 **P1 的 Go timeline 已從快照序列推導出 `first_seen_at`**，這個欄位目前**沒有消費者**，
  加了只會多一份可能與推導值不一致的資料。
- 真正需要它的是 `runtime_chain`（讓 Python 的 Lifecycle Engine 知道鏈跑多久），已列為另案。

**驗證**：`python/scripts/test.sh` 428 passed / 1 skipped，既有的事件狀態機測試
（含 `test_fresh_detection_resets_carried_event_age`）全數未受影響。
`age_bars` 的重置行為**沒有動**——`_normalize_previous_event_state` 的註解明說那是刻意設計。

#### 與 Lifecycle Engine（原 T-044）的接縫

**原 T-044 已完成並移出清單**；以下保留當時的接縫分析，其中的 `[T-044]` 指的是現行的
`lifecycle_engine.py`。仍未完成的是本筆的 `runtime_chain` 那一半。

```
market_event_states 快照序列
        ↓（P1 摺疊）
   Event Timeline（chain[]）
        ↓
[T-044] Lifecycle Engine ── lifecycle + reason_codes
        ↓
   Decision Engine（＋RR Gate、策略模式）
```

**但 P1 的 chain 不會自動成為 T-044 的輸入**（2026-08-13 review 修正）。P1 只在 Go 端新增
唯讀 API，Python 分析流程完全沒動；Lifecycle Engine 若要吃 chain，必須把 chain contract
一路送進 Python scoring / replay runtime。因此 chain 要分成兩種，**不要混為一談**：

| | `display_chain[]` | `runtime_chain[]` |
|---|---|---|
| 產生者 | Go 由 DB 快照重建（P1） | Go→Python request contract（未規劃） |
| 消費者 | T-041 前端 timeline、人工檢查 | Lifecycle Engine 的權威輸入 |
| 需要動到 | 新增唯讀查詢 | analysis client mapping、replay previous-state 管線、對照測試 |
| 本次範圍 | ✅ P1 | ❌ 另案 |

所以 **T-044 現階段是 snapshot-based**，等 `runtime_chain` 到位後才換輸入形狀。
換成 chain 之後才可能實作現在做不到的規則，例如「同一個 zone 第三次測試」與「第一次測試」
判成不同狀態——現在的模型看不出次數。**那些新規則不在 T-044 也不在 T-045 的範圍內。**

#### 既有 `event_sequence` 的去留

`decision_engine._event_sequence()`（`decision_engine.py:1826`）目前輸出的「Event Sequence」
只是把**當根偵測到的** `market_events` 依固定優先序排序去重，欄位是 `type` / `label` /
`direction`，沒有 `analyzed_at`、沒有 state transition、沒有 chain 邊界。
**它不是事件鏈，也不該被當成事件鏈**。

Timeline 上線後兩者會同時存在且名字近似，容易誤用。建議：P1 完成後在
`api-reference.md` 明確區分兩者語意（「當次事件摘要」vs「跨分析事件鏈」），
待 T-041 前端改用 timeline 顯示後，再評估 `event_sequence` 是否退場——
**本次不刪**，因為它仍是 decision summary 的既有欄位。

#### 不做的範圍

- **不新增資料表**（除非 P1 實證顯示快照序列真的不足以還原鏈）。
- **不改前端**——那是 T-041 的 Event Timeline 面向，本筆只提供 API。
- 不改事件偵測邏輯（`detect_market_events`）與事件家族定義。
- P1 不碰 Python；P2 才動。

#### 前置條件：目前沒有任何排程會產生 SR zone 分析

**timeline 的解析度等於分析頻率，不是 K 棒頻率。** 而分析頻率目前是——沒有頻率。

2026-08-13 對照程式碼與 live 資料確認：

| 事實 | 依據 |
|---|---|
| 9 個排程 job 沒有任何一個建立 SR zone 分析 | `scheduler.go` 的 `startRun` 呼叫點：`pre_market`／`intraday`／`daily_close`／`chip_daily_sync`／`stock_symbol_sync`／`sr_zone_verify`／`sr_evaluation`／`corporate_action_sync` |
| `sr_zone_verify` **不建立分析**，只重驗既有分析的 zone 有沒有被突破 | `scheduler.go:409-412` 的說明 |
| 唯一的建立路徑是手動／API | `POST /sr-zones` → `handler/sr_zones.go:524` |
| live 實際只有 **20 次分析 / 4 檔標的**（2026-07-13～08-12） | `stock_sr_zone_analyses`：0050 十四次、2330 四次、`00981A` 與 `00947` 各一次 |
| 連帶 `market_event_states` 只有 **76 列 / 13 次分析 / 2 檔標的** | 同上，事件狀態只在有分析時才寫入 |

**所以 P1 做出來，多數標的的 timeline 會是空的，0050 也只有十幾個點。**
這不影響 P1 的正確性，但**決定它的當下價值**：沒有穩定的分析節奏，事件鏈就沒有內容可看。

要讓 timeline 真的有東西，需要一個**定期對 watchlist 產生 SR zone 分析的排程**——
那是本筆之外的獨立工作，且成本不低：每檔分析都會呼叫 Python scoring，
而這台 host 的記憶體限制已在 [`sr-zone-scoring.md`](./sr-zone-scoring.md)「規模上限」量化過。

**已決定（2026-08-13）：採第 2 案——P1 照做，接受初期資料稀疏。**
理由是 P1 是純新增的唯讀查詢、風險最低，而摺疊邏輯本身需要真實資料驗證形狀，
0050 現有的十幾個分析點已足以驗證鏈能否還原。補分析排程牽涉每檔都要跑 Python scoring，
在這台 2GiB host 上需要單獨評估（見 `sr-zone-scoring.md`「規模上限」），不綁進 timeline 的範圍。

當初評估的三個選項（保留供日後回顧）：

1. **先補分析排程再做 P1** — timeline 一上線就有內容，但要先付排程與記憶體的成本。
2. **P1 照做，接受初期資料稀疏** — 端點與摺疊邏輯先就緒，資料隨分析累積自然變厚。
   前提是輸出必須誠實標示 snapshot gap（見下方風險），否則空白會被誤讀成「沒有事件」。
3. **兩者同批** — 範圍最大，但避免做出一個沒有資料可展示的功能。

#### 主要風險

- **鏈會有洞**，P1 的輸出必須誠實標示「這是分析快照序列，不是逐日事件史」，
  否則會被誤讀成那段期間沒有事件發生（成因見上方前置條件）。
- 分析被取代時 `sr_zone_repo.go:558` 會 `DELETE FROM market_event_states WHERE analysis_id=?`，
  重跑同一天的分析會覆蓋當天快照。鏈因此是「每次分析的最後一版」，不含當日中間狀態。
- P2 修 `root_event_type` 會改變 `market_event_states` 的寫入內容，**既有列不會回填**，
  所以鏈在修正前後的語意不同。要在文件標明分界日期。

#### 測試與驗證策略

- P1：摺疊邏輯的表格驅動測試（同狀態不產生 transition、resolved_by 產生、
  跨 analysis 缺漏時的行為），以及對 live 唯讀資料實跑一次確認 0050 的鏈可還原。
- P2：`build_event_state_summary` 的既有測試要擴充「同家族第二次事件不得抹掉 root」。
- 與 T-044 同批時，decision summary 的逐欄對照測試同樣適用。

#### 完成後歸檔

- 事件鏈的定義、chain 的邊界（何時算新的一條）→ [`sr-zone-scoring.md`](./sr-zone-scoring.md)。
- 新端點 → [`api-reference.md`](./api-reference.md)。
- 「timeline 解析度 ＝ 分析頻率」這個限制 → [`issue.md`](./issue.md) 或 `sr-zone-scoring.md`，
  它會長期影響判讀。

#### Review 決策紀錄（2026-08-13，已採納並整合進上文）

**方向正確：不要再把 `event_sequence` 當 Event Timeline。** 目前
`decision_engine._event_sequence()` 只是把當根偵測到的 `market_events` 依固定優先序排序並去重，
輸出 `type` / `label` / `direction` 等欄位，沒有 `analyzed_at`、state transition 或 chain
邊界；前端現在顯示的 Event Sequence 因此只是當次事件摘要，不是完整事件鏈。

P1「不新增資料表、由既有 `market_event_states` 快照序列重建」可行，但要補三個設計細節：

1. **端點路徑要改。** 目前 router 已有 `GET /sr-zones/:id`，`GET /sr-zones/:symbol/event-timeline`
   容易和同層 wildcard 路由衝突或讓 symbol/id 語意混雜。建議改為
   `GET /sr-zones/event-timeline?symbol=2330&timeframe=1d`；若未來要支援區間或筆數限制，
   也自然能加 `from` / `to` / `limit`。
2. **需要新的 repo 查詢 contract。** 現有 `SRZoneRepo` 只有
   `GetMarketEventStates(analysisID)` 與 `GetLatestMarketEventStates(symbol,timeframe)`；
   P1 需要的是依 `symbol,timeframe` 取一段歷史 states，並按
   `analyzed_at, analysis_id, zone_key, event_family` 穩定排序。這應明確列為新增 method，
   不能假設現有 latest snapshot 查詢能重用。
3. **需要補索引規劃。** migration 042 目前的索引偏向
   `(symbol, timeframe, active, analyzed_at DESC)`，適合最新 active snapshot，不適合 timeline
   依 `(symbol,timeframe,zone_key,event_family,analyzed_at)` 摺疊。P1 若要在資料量增加後仍可用，
   應補一個歷史 timeline 查詢用索引；三種 engine（postgres/sqlite/mysql）都要同步。

P1 產出的 `chain[]` 要先定位為**展示與實證資料**，不是自動等於 T-044 的 runtime input。
原因是 P1 按目前規劃只在 Go 端新增唯讀 API，不改 Python 分析流程；但 T-044 的
Lifecycle Engine 若要吃 `chain[]`，必須把 chain contract 傳進 Python scoring/replay runtime。
因此 T-045 與 T-044 的接縫建議拆成兩層：

- `display_chain[]`：Go 由 DB 快照重建，供 T-041 前端 timeline 顯示與人工檢查。
- `runtime_chain[]`：若 Lifecycle Engine 要以 chain 為權威輸入，另案或同批補 Go→Python
  request contract、analysis client mapping、replay previous-state 管線與對照測試。

測試除原本 P1/P2 項目外，還要補路由與查詢層測試：endpoint 不被 `/sr-zones/:id` 吃掉、
同狀態快照不產生 transition、`resolved_by` / `latest_event_type` 改變會產生 transition、
跨 analysis 缺口會在輸出標示 snapshot gap，而不是假裝逐日連續。

---

### T-047：SR evaluation 串流化，解除全市場的記憶體上限

| 欄位 | 內容 |
|---|---|
| 狀態 | 待規劃（**全市場路線的前置條件**） |
| 優先度 | 低（150 檔以內不需要） |
| 分類 | Python / SR Zone / 效能 |
| 建立日期 | 2026-08-18（原 `issue.md` I-056 的「可行的改造方向」，該筆收斂時移入） |
| 來源 | T-039 sweep 實跑 ＋ T-040 Step 0 記憶體實測 |

現況上限與實測數據見 [`sr-zone-scoring.md`](./sr-zone-scoring.md)
「規模上限：`sources` 與 `dataset` 必須同時常駐記憶體」。
**150 檔以內不需要做這一項**——2026-08-17 實測 131 檔峰值 382MB，跑得完。

要往 CLAUDE.md Roadmap 的全市場（2,298 檔）方向走才需要：

1. **串流化**：逐檔建完 dataset 後立刻釋放該檔的原始 frame。前提是把
   `_volatility_profiles` 改成**逐檔算好 profile 再丟掉 frame**，而不是最後才一次算。
   這是最有效的一刀——原始 frames 是全市場情境下最大的一塊（約 220MB）。
2. **指標可以串流，但不能分批平均**：AUC 是非線性的排序統計量，
   **把各批的 AUC 平均是錯的**。正確做法是只累積「預測機率 ＋ label」兩個一維陣列
   （全市場約 124 萬列 × 2 × 8B ≈ **20MB**），最後一次算 AUC / Brier / log loss。
   這條路可行且便宜。
3. 降 `--limit`（每檔取較少 K 棒）或對標的抽樣——最省事，但**直接犧牲樣本量**，
   而樣本量正是擴標的池要解決的問題，只適合當臨時手段。

**另一個獨立的前置條件**：T-042 的「逐檔事件的增量更新」。減資走 FinMind（5/分）
且與每日抓價共用節流器，1,900 檔光減資就要 6.3 小時並排擠行情抓取。
記憶體解決了，抓取節流仍會擋住。

---

### T-049：Market State 與所有下游改讀同一套 state

| 欄位 | 內容 |
|---|---|
| 狀態 | 待規劃（**前置未完成**） |
| 優先度 | 中 |
| 分類 | Python / SR Zone / 決策邏輯 |
| 建立日期 | 2026-08-18 |
| 來源 | 同 T-048（SR Zone 狀態持久化，已於 2026-08-20 完成並收斂），階段 5～6 |

**範圍**：Market State 改讀 Lifecycle 而不再自己重判 Event；接著 Bias /
Daily Confirmation / Reclaim / Event Sequence / Final Entry 全部改讀同一套 state。

**依使用者確認，本筆先只列方向與驗證門檻，細節等看過真實資料形狀再寫。**

#### 從 T-048 交接過來的項目（2026-08-20 收斂時移入）

T-048 已完成並收斂，身分層／事件鏈的現況規格見
[`sr-zone-scoring.md`](./sr-zone-scoring.md)「Zone 身分與 ZoneMatcher」與「事件層：鏈的
身分與三段關聯決策」，schema 見 [`database-schema.md`](./database-schema.md)。
它明確延後、指名由本筆承接的有四項；另有一個 T-048 驗收踩出的 issue
（I-077）也必須在本筆一起看，因為它會直接改 Market State：

1. **`ZoneScore.zone_uid` 仍未接上**（Python 端在分析當下拿不到身分）。要餵得動 matcher
   就得給它「上一次的 zone 清單」，而 Python 目前唯一的跨次狀態通道是 Go 傳進來的
   `previous_event_states`，沒有 `previous_zones`——**要改 `/sr-zones` 的 request contract**。
   T-048 階段 E 定案延後的理由是「它現在沒有讀者」：唯一的候選消費者是把
   `event_engine._zone_key()` 換成 uid，而那會改變 `market_event_states.event_key` /
   `zone_key` 與 carry-forward 的比對面，與本筆的三段關聯決策重疊，是獨立的一階。
   **本筆若要動 `_zone_key()`，這兩件事必須一起決定。**
2. **兩個新事件（`SUPPORT_RETEST_HELD` / `RESISTANCE_BREAKOUT`）的 `resolves` 是空的。**
   `resolves` 會把既有 family 改成 `RESOLVED`／`active=False`，那是決策可見的改變。
   「壓力突破是否 resolve 支撐側事件」本身是個真問題（現行
   `EVENT_FAMILY_LIFECYCLE_RULES` 全是 `SUPPORT_*` 與 `VOLUME_CONTEXT`，沒有壓力側先例），
   但只有在事件真的接進決策之後才有意義。
3. **`evaluation.py` 的 `market_event_types` 分層鍵會多出新型別**，影響 replay／
   evaluation 的分層可比性，不影響 `stock_sr_decisions`。非阻斷，但做分佈比較前要先處理，
   否則新舊兩批的分層對不起來。
4. **新舊兩套並行比對還沒做**（見下方前置①）。
5. **I-077：同一個交易日重複分析會讓事件提早老化到 `EXPIRED`。**
   目前 `age_bars` 是「被 carry 的分析次數」而不是「K 棒推進次數」；
   T-049 一旦讓 Market State 與下游全部改讀同一套 state，就不能再把這個問題留在旁邊。
   規劃時要一起決定是否把老化改成依最新 K 棒 timestamp 推進，而不是依分析次數推進。

#### 兩個前置條件，缺一不可

1. **新舊兩套狀態並行比對過一段時間。** T-048 本身已完成，但它的驗收做的是**回歸比對**
   （改動前後決策逐欄相同、身分層數字逐項重現），**不是**計畫書要求的並行比對
   （逐日比對新舊兩套的 active 事件集合，差異只能來自「分裂被正確合併」）。
   起點資料是 [`issue.md`](./issue.md) I-080 的落差表：同一份 84 次分析裡，
   `event_instances` 的鏈數與 timeline 端點的 `(zone_key, family)` 組合數**雙向都對不上**
   （多出來的是 key 漂移拆開的鏈，少的是身分終止後的重生鏈）。
   這件事要等前置②給出母體才做得起來——21 個交易日湊不出「一段時間」。
2. **補分析排程**——「定期對 watchlist 產生 SR zone 分析」。
   這是 `issue.md` **I-074** 的關閉條件，也是本筆唯一可行的驗證來源：
   目前 production live DB 的 `stock_sr_zone_analyses` 只有 **4 檔 / 20 次分析**
   （2026-08-18 再次確認未增加），
   而本筆會同時改動 Bias、進場、事件序列——**比 T-044 那次影響面大一個量級**，
   不能再用「428 支測試全綠」當證據交付。

   注意這個 **4 檔 / 20 次** 是 production live DB 的自然母體；T-048 收斂時使用的
   **4 檔 / 84 次** 是 isolated/as-of 階梯驗證 fixture，用來證明回歸與身分層寫入，
   不能替代本筆需要的 production 分佈比較母體。

   這個排程已於 2026-08-20 獨立成
   [T-052](#t-052定期對-watchlist-產生-sr-zone-分析分析排程)（在那之前沒有任何 todo 在追，
   只散見於 T-045 與本筆的討論段落）。它本身有成本（每檔都要跑一次 Python scoring，
   記憶體限制見 `sr-zone-scoring.md`「規模上限」），要獨立評估。

#### 驗證門檻（現在就定，避免事後放寬）

`MODE=replay scripts/run-evaluation.sh` 對真實資料比對 `final_entry_state` /
`lifecycle_phase` / `market_bias` 的分佈變化，且**母體要足以做分佈比較**。
在達到這個門檻之前，本筆不應開始實作階段 6。

**這一條是從 T-044 的教訓來的**：那次的行為改變至今只有單元測試層級的證據
（`issue.md` I-074），因為驗證母體始終沒有補上。同樣的缺口不應該在影響面更大的
這一筆再發生一次。

---

### T-050：身分追蹤的可觀測性——把關聯決策計數從 log 升級成可查詢的 metric

| 欄位 | 內容 |
|---|---|
| 狀態 | **已實作／待 review**（2026-08-21） |
| 優先度 | 中（T-048 階段 C 的後續；T-048 已於 2026-08-20 完成並收斂，本筆不擋任何人） |
| 分類 | Go / 可觀測性 |
| 建立日期 | 2026-08-19 |
| 來源 | T-048 階段 C「P1 frozen chain 可觀測性」的取捨：以結構化 log 交付，metric 另立一筆（現況見 [`sr-zone-scoring.md`](./sr-zone-scoring.md)「可觀測性：一筆結構化 log 拆出關聯決策」） |
| 關聯 | T-048 階段 C（`eventIdentityStats`）、階段 B（`zone_identity` 的 warn） |

#### 問題

T-048 階段 C 的 `eventIdentityStats` 會把每次分析的關聯決策拆成
`MatchedByChain` / `MatchedByCurrent` / `MatchedByAlias` / `UnmatchedKeys` /
`CarriedNoop` / `ZoneEndedSkipped` / `ChainConflicts` / `ChainKeyAmbiguous` /
`CarriedParseFail` / `Invariant`，以**單筆結構化 log** 輸出。

log 能回答「這一次分析發生了什麼」，但答不出**趨勢**問題，而 F1／F2 這類缺陷正是
趨勢型的——「alias 命中率從 2% 爬到 30%」代表 zone 邊界漂移在惡化，
「`ChainConflicts` 開始非零」代表身分匹配出了新的成因。這些在 log 裡要靠人去 grep
才看得到，而沒有人會每天 grep。**F1 在 live 存在了兩週沒人發現**，就是這個缺口。

`backend/go.mod` 目前**沒有 prometheus 或任何 metrics 依賴**，整個 backend
也沒有 `/metrics` 端點——引入是跨系統決策（要決定 exporter、抓取端、儀表板、
保留期），不該夾帶在一個只寫不讀的功能裡交付。

**2026-08-21 的實測讓這個缺口變得具體。** T-052 上線後的第一輪排程（8/20 22:00）跑完
11 檔全部 skip，隔天早上想查發生什麼事時：

* `job_runs` **查不到任何 `sr_analysis` 紀錄**——因為 `runPreMarket`（每天 08:50）會
  `DeleteBefore(TodayTaipei())`，**只保留當天**。前一晚的紀錄在早上 08:50 被例行清掉。
* 唯一還原得出真相的只有 `docker logs`，而且要一路往回翻才看得到
  `sr analysis done ... skipped=11` 與每檔的 skip 原因。

也就是說**現有的兩個機制都答不出「昨晚那輪做了什麼」**：log 沒人翻，job_runs 隔天就沒了。
本筆要建的表**因此不能沿用 job_runs 的當日清除策略**——那正是要解的問題。

#### 計畫書（**已實作／待 review**，2026-08-21）

##### 修改目標與不做的範圍

* **目標**：讓「身分關聯決策的組成」變成**可查詢、且能跨日比較**的資料，回答
  「alias 命中率是不是在爬」「`ChainConflicts` 是不是開始非零」這類趨勢問題。
* **不做**：不改任何決策邏輯、不改身分層的判準、不改事件偵測。
* **不做**：不做告警規則（要先有抓取端），不引入 metrics 依賴（見 V1）。
* **不做**：不回填歷史——過去的 `eventIdentityStats` 只存在於 log，重建不可能可靠。

##### 四個定案

**V1：寫進 DB 的一張輕量表 ＋ 查詢端點，不引入 prometheus。**

| 方案 | 判斷 |
|---|---|
| **DB 表 ＋ 端點**（採用） | 不新增依賴，與 `job_runs` / `stock_sr_regression_results` 的既有模式一致；查詢端點可直接餵給既有前端 |
| prometheus client ＋ `/metrics` | 否決。要一併決定 exporter／抓取端／儀表板／保留期，是跨系統決策；而這台 host 只有 2GiB，再擺 Prometheus ＋ Grafana 不現實 |

**決定性的理由是資料頻率**：這個 metric **一次分析產生一筆**，T-052 上線後是
**每天約 22 筆**。Prometheus 是為了高頻抓取設計的，22 點/天本質上是一張**表**而不是
time series。用它反而要處理「抓取間隔內沒有變化」這種與問題無關的複雜度。

告警之後仍可加：`Invariant` 非零是唯一真正需要即時告警的訊號，而它現在已經是 Error 級 log。

**V2：一次分析一列，母體寫在資料裡而不是靠讀者記得。**

`eventIdentityStats` 只在 `reuse_existing=false` 那條路徑產生（身分層的既有已知限制），
所以它統計的是分析的**子集**。**每列都帶 `analysis_id`**，讓「分母是哪些分析」可以直接
join `stock_sr_zone_analyses` 算出來，而不是靠讀者記得這條限制。

T-052 上線後這條限制的實務影響變小了——排程走的正是 `reuse_existing=false`——但
`portfolio/analyzer` 那條重用路徑仍然不產生統計，欄位語意必須寫明。

**V3：存原始計數，比率在查詢時算。**

表裡只存**該次分析的原始計數**（`matched_by_chain` / `matched_by_current` /
`matched_by_alias` …），不存比率。理由：比率的分母會隨「要看哪個區間」而變，先算好等於
把一個決定寫死。查詢端點負責聚合。

**`Invariant` 與其他欄位語意不同**，要分開：其他是「這樣的分佈正不正常」，而它是
**必須恆為零**的不變式違反。表裡存計數，端點另外把它拉成獨立欄位，不要混進同一組比率裡。

**V4：一併涵蓋 zone 側。**

階段 B 的 zone 身分比對失敗目前只有 `zone identity: match failed` 這類 warn，同樣沒有趨勢。
成本很低——同一列多幾個欄位即可（比對是否降級、`ListLive` 撈到幾個候選、終止了幾個身分），
不必另開一張表。

##### 受影響的檔案與資料流

```text
migrations/{postgres,sqlite,mysql}/0XX_sr_identity_stats.sql   **三份都要**
Go store/model.go                    ＋ SRIdentityStats
Go store/sr_identity_stats_repo.go   新增：Insert ＋ List（供端點查詢與聚合）
Go api/handler/sr_zones.go           persistEventIdentity 算完 stats 後多寫一筆
                                     （**fail-open**：寫入失敗只記 log，不影響分析）
Go api/handler/sr_zones.go           ＋ GET /sr-zones/identity-stats
Go api/server.go                     路由 ＋ repo 注入
```

* Python：**不改**。前端：不改（要接是 T-041／另案）。

##### 資料 contract 變化

| 變更 | 型態 | 相容性 |
|---|---|---|
| 新表 `sr_identity_stats` | 純新增 | 沒有任何既有查詢會碰到 |
| `GET /sr-zones/identity-stats` | 純新增端點 | — |
| 既有 API／決策欄位 | **不變** | 本筆只多寫一張表 |

##### 主要風險與回滾

| 風險 | 對策 |
|---|---|
| 在分析路徑上多一次寫入，失敗會拖垮分析 | **fail-open**，比照身分層既有語意：寫入失敗只記 log，分析照常回 201。統計缺一列比分析失敗好 |
| 表無限成長 | 22 列/天 ≈ 8000 列/年，欄位都是整數。**刻意不套 job_runs 的當日清除**——那正是要解的問題。真的要清也該是「保留 N 個月」而不是「保留當天」 |
| 統計與 log 不一致（兩份事實） | 同一個 `eventIdentityStats` 值同時餵給 log 與表，不各算一次 |
| 母體被誤讀成「所有分析」 | V2 的 `analysis_id` ＋ 欄位註解 ＋ 歸檔說明 |
| 回滾 | 純新增表與端點，寫入是 fail-open。`git revert` ＋ `-- +goose Down` |

##### 測試與驗證策略

* **單元（Go）**：`eventIdentityStats` → 資料列的映射逐欄；寫入失敗時分析仍成立
  （fail-open）；`Invariant` 非空時該列看得出來。
* **Migration**：`scripts/test-postgres-migrations.sh`；sqlite 由 `backend/scripts/test.sh` 覆蓋；
  mysql 依 I-054 既有處置。
* **端到端（dev 階梯）**：重跑四檔 21 階後
  1. `sr_identity_stats` 恰好 **84 列**（每次分析一列）；
  2. **從表裡算得出目前只能靠 grep log 得到的數字**——`alias_ambiguous` 合計為 0
     （F5 修法後的實測值），`MatchedByAlias` 合計為 0，`Invariant` 合計為 0；
  3. 決策路徑逐欄不變。

##### 實作結果（2026-08-21）

| 門檻 | 結果 |
|---|---|
| Go 全量 `test.sh` | 全綠（+6 支） |
| `scripts/test-postgres-migrations.sh` | 全綠（含 070 up/down） |
| 一次分析一列 | **84 列 / 84 次分析** |
| **表能重現只能靠 grep log 得到的數字** | `alias_ambiguous` **0**、`matched_by_alias` **0**、`invariant_violations` **0**、`unmatched_keys` **0**、降級 **0**——與 F5 修法後及階段 C 記錄的實測值一致 |
| 端點 summary | `matched_total=382`（既有鏈 250 ／本次 map 132 ／alias 0）、`alias_hit_rate=0`；`symbol=2330` 篩選得 21 次分析、`matched_total=92` |

**新拿到的數字**：`matched_by_chain` 250 與 `matched_by_current` 132 這兩個以前只在 log 裡的值，
現在查得到也比得了——第一段（既有鏈命中）吃掉約 65%，這正是 `matched_by_alias` 一直是 0 的原因
（見 `issue.md` I-078）。

實作與計畫書的一處差異：

* **寫入點放在 `RunAnalysis` 而不是 `persistEventIdentity` 內。** 後者在 zone 身分降級時會直接
  return，而「這次降級了」正是最該留下的一列。改成 `persistEventIdentity` 回傳 stats、由
  `RunAnalysis` 統一寫，降級時寫一列帶 `zone_identity_degraded=true`。
  若那時候不寫，趨勢圖上會看到「這天很乾淨」，而真相是「這天什麼都沒算」。

##### 完成後歸檔

* 「可觀測性：一筆結構化 log 拆出關聯決策」改寫成 log ＋ 表兩層，並寫明**表才答得出趨勢**
  → [`sr-zone-scoring.md`](./sr-zone-scoring.md)。
* 新表 schema、保留策略（**不沿用 job_runs 的當日清除**）與母體語意 →
  [`database-schema.md`](./database-schema.md)。
* 新端點 → [`api-reference.md`](./api-reference.md)。

#### 前置

**已滿足**：T-048 全案已完成、review 通過並收斂（2026-08-20），`eventIdentityStats`
的欄位定義已穩定，metric 的 label 可以照它定。

#### 不做的範圍

* 不做告警規則本身（那要先有抓取端）。
* 不改任何決策邏輯。

---

### T-051：Event Timeline 改讀身分層，讓修好的分裂真的看得到

| 欄位 | 內容 |
|---|---|
| 狀態 | **已實作／待 review**（2026-08-20） |
| 優先度 | 中（T-048 的價值兌現點、T-041 的前置；**不在 T-049 的路徑上**，服務的是 display_chain 而非決策，與下游 T-041 同級） |
| 分類 | Go / SR Zone / 事件鏈 / 對外 API |
| 建立日期 | 2026-08-20 |
| 來源 | T-048 全案 review：身分層四張表目前**沒有任何讀者** |
| 關聯 | 修的是 [`issue.md`](./issue.md) I-080；下游是 T-041 的 Event Timeline 面向 |

#### 問題

T-048 已經把「同一個 zone 跨交易日的身分」算出來也存下來了，但
`GET /sr-zones/event-timeline` 仍以 `(zone_key, event_family)` 摺疊
`market_event_states`——**T-048 要修的分裂，在唯一會顯示鏈的端點上原封不動**。
實測落差與成因見 I-080。

更根本的一點：`zone_instances` / `event_instances` / `zone_relations` /
`event_transitions` 四張表在 Go 端只有兩處 `SELECT`（`zone_identity_repo.go` 的
`ListLive`、`event_identity_repo.go` 找活鏈），**兩處都是 matcher 自己餵自己**。
對外唯一看得到的身分資料是階段 E 加的 `zones[].data.zone_uid`。身分層目前仍是
「只寫不讀」，本筆是它的第一個真正讀者。

#### 計畫書（**已實作／待 review**，2026-08-20）

##### 修改目標與不做的範圍

* **目標**：`GET /sr-zones/event-timeline` 改以**身分層**為來源，讓同一個 zone 的鏈不再
  因 `zone_key` 漂移而被拆開（[`issue.md`](./issue.md) I-080）。
* **不做**：不改任何決策邏輯、不改事件偵測、不改 Python、不動身分層的寫入端。
* **不做**：不接血緣（`zone_relations`）——把 `SPLIT`/`MERGE`/`RESHAPE` 前後的鏈串成一條
  是獨立的一階，且寫入端刻意決定「parent 的事件不傳給 child」，讀取端不該偷偷接回去。
* **不做**：不改前端（T-041 另案）。

##### 四個定案

**U1：換來源，不是「維持摺疊 ＋ 多帶 `zone_uid`」。**

盤點後這其實不是偏好問題——**併行方案做不出來**：

* `market_event_states` **沒有 `zone_uid` 欄位**，摺疊時拿不到身分。
* 想在讀取時補上這個對應，只能拿 `zone_key` 去查 `zone_key_aliases`，而那是**有損的**：
  每個身分只留最近 8 筆 alias，實測已有 **23 個身分撞頂**（I-079），超出的舊 key 永遠查不回來。
* `stock_sr_zones` 雖然有 `zone_uid`，但**沒有存 `zone_key`**（`db:"-"`），
  兩邊接不起來——除非用價格邊界回推，而那正是 T-048 要消滅的模式。

反過來看，**寫入端在三段關聯決策裡已經把這個對應做對了**（既有鏈優先 → carried 護欄 →
key 解析／alias 備援），答案就存在 `event_instances`。讀取時重算一次不但更差，還會與寫入端
變成兩份會漂移的事實。

**U2：三項現行輸出身分層沒有，逐項處置。**

| 現行輸出 | 身分層有沒有 | 處置 |
|---|---|---|
| 每一步的 `state` / `reason_codes` / `event_type` | ✅ `event_transitions` 的 `to_state` / `reason_codes` / `trigger_event_type` | 直接對應 |
| 每一步的 `active` | ❌ 只有鏈層的當前 `active` | **不再逐步輸出**。要重建它得把 family 的 `gating_states` 規則複製一份到 Go——那會是第二份判準，正是 T-048 一路在避免的東西。鏈層仍輸出 `active` |
| `changed[]`（相鄰快照的差異欄位） | ❌ 摺疊時才算得出來 | **不需要了**。`from_state → to_state` ＋ `trigger_event_type` 本身就說明了這一步改了什麼，而且是存下來的事實而非推導 |

`snapshots`（分析次數與 gap 揭露）**保留**——它查的是 `stock_sr_zone_analyses`，與事件無關，
不受換來源影響。

> 順帶：`event_timeline.go` 上 `Snapshots` 的註解寫著「目前沒有任何排程會產生分析」，
> **T-052 之後這句已經過期**，本筆一併更新。

**U3：鏈的鍵改成 `zone_uid`，`zone_key` 降級但不刪。**

回應的 chain 物件：

| 欄位 | 變化 |
|---|---|
| `zone_uid` | **新增**，鏈的身分（`event_scope='SYMBOL'` 時為 `null`） |
| `event_uid` | **新增**，這條鏈自己的 id，供前端穩定 key 與後續下鑽 |
| `seq` | **新增**，同一個 (zone, family) 的第幾條鏈 |
| `end_reason` | **新增**，`RESOLVED` / `EXPIRED` / `ZONE_IDENTITY_ENDED` |
| `zone_key` | **保留但語意改變**：從「鏈的身分」變成 `last_zone_key`（最近一次觀測到時事件帶的 key），只供人工比對 |
| `closed` / `final_state` / `direction` / `root_event_type` / `first_seen_at` / `last_seen_at` | 不變 |

**`seq > 1` 是新的一條鏈，不與前一條合併**——這與寫入端的語意一致（前一條 `RESOLVED`／
`EXPIRED` 之後再出現同家族事件，是新的一條而不是舊鏈復活）。實測 10 條。
`end_reason = ZONE_IDENTITY_ENDED`（實測 8 條）要在回應裡看得出來：那是「zone 身分終止所以
鏈收攤」，不是自然結束，前端若把它畫成一般結束會誤導。

**U4：涵蓋範圍要誠實揭露。**

身分層是從 migration 068 才開始寫的，**更早的分析沒有事件鏈資料**，換來源後那段期間會
從 timeline 上消失。回應新增 `identity_since`（身分層最早的 `first_seen_at`）；
早於它的 `snapshots` 照常列出，讓「這段沒有鏈」與「這段沒有分析」在畫面上分得開。
**不回填**——理由與 `stock_sr_zones.zone_uid` 相同，回填要解的正是「兩個舊 key 是不是同一個
zone」，那是身分層本身要建的能力。

##### 受影響的檔案與資料流

```text
Go store/event_identity_repo.go   ＋ ListChains(symbol, timeframe, opts)
                                  ＋ ListTransitions(eventUIDs)
                                  （目前只有 matcher 自己用的「找活鏈」，沒有任何列表查詢）
Go analysis/event_timeline.go     BuildEventTimeline 改吃 chains ＋ transitions，
                                  不再摺疊 market_event_states；移除 changed[] 的 diff 邏輯
Go api/handler/sr_zones.go        EventTimeline handler 改叫新的 repo 方法
```

* Python：**不改**。前端：**不改**（T-041 另案）。DB：**不新增表也不新增欄位**。

##### 資料 contract 變化

| 變更 | 型態 | 相容性 |
|---|---|---|
| chain ＋ `zone_uid` / `event_uid` / `seq` / `end_reason` | 純新增鍵 | 前端目前沒有任何呼叫端（`frontend/src` 查無 `event-timeline`） |
| chain 的 `zone_key` 語意改為 `last_zone_key` | **語意變更** | 值的形狀不變；沒有消費者，但要寫進 `api-reference.md` |
| transition 移除 `active` / `changed` | **移除欄位** | 同上，目前無消費者 |
| 回應 ＋ `identity_since` | 純新增鍵 | — |
| 鏈的數量與邊界 | **會變**（這正是目的） | 實測 I-080 的落差表 |

##### 主要風險與回滾

| 風險 | 對策 |
|---|---|
| 身分層是「只寫不讀」，本筆是**第一個真正的讀者**，寫入端的缺陷會第一次被看見 | 這是好事也是風險。驗收要求端點鏈數與 `event_instances` **逐檔相同**——對不上就是讀取端寫錯，不是資料壞 |
| `event_transitions` 沒有 `active`，前端若已依賴它會壞 | 前端沒有任何呼叫端，現在改是最便宜的時機 |
| 舊分析沒有鏈，看起來像「資料不見了」 | U4 的 `identity_since` ＋ 保留 `snapshots` |
| 回滾 | 純讀取端改動，DB 與寫入端都沒動。`git revert` 即可 |

##### 測試與驗證策略

* **單元（Go）**：`event_instances` ＋ `event_transitions` 組成 chain 的映射；
  `seq > 1` 不與前一條合併；`ZONE_IDENTITY_ENDED` 的 `end_reason` 有輸出；
  `event_scope='SYMBOL'` 的鏈 `zone_uid` 為 null 不 panic；沒有身分層資料時回空鏈但仍有
  `snapshots`。
* **端到端（dev 階梯）**：重跑四檔 21 階（`ladder.sh`）後
  1. 端點回傳的鏈數與 `event_instances` 的鏈數**逐檔相同**（2330 28／3105 38／6182 37／8150 25）；
  2. 至少一個「漂移過 `zone_key` 的身分」在新端點上是**一條**鏈——舊實作會是多條；
  3. 決策路徑逐欄不變（本筆只動讀取端，`stock_sr_decisions` 應完全相同）。

##### Review findings（2026-08-20，**已修正／待複審**）

* **P2：`max_analyses` 只限制了 `snapshots`，`chains` 仍撈全歷史。** 語意錯位，且 T-052
  每日累積後回應會愈來愈大。**已修正**：先查分析、取最舊那次的時間當視窗起點再查鏈。
  納入規則是「有一步落在視窗內，**或這條鏈還沒結束**」——後半是必要的，一條長壽而這段
  期間沒有變化的鏈正是最該看到的。被選中的鏈一律回完整歷史（切一半會失去誕生那一步）。
* **P3：`zone_uid` 用 `string + omitempty`，SYMBOL scope 時鍵會直接消失。** 與計畫書寫的
  「為 null」不符，消費端寫「欄位存在但為 null」的判斷會靜默走到 undefined 分支。
  **已修正**：`zone_uid` 與 `zone_key` 都改成 `*string`，並補一支**序列化層**的測試
  （Go struct 上 `*string(nil)` 與 omitempty 分不出來，只有 marshal 才驗得到）。

##### 修正 P2 時抓到的回歸（2026-08-20）

**第一版的 P2 修正是空操作，而且它掩蓋著一個更大的問題：兩個時間軸混用。**

| 欄位 | 時間基準 | 實測 |
|---|---|---|
| `stock_sr_zone_analyses.analyzed_at` | **K 棒日期** | 2026-07-20 ~ 08-17 |
| `event_instances.first_seen_at` / `event_transitions.occurred_at` | **wall clock** | 2026-08-20 09:36 |

過濾條件寫成 `last_seen_at >= since`，拿 wall-clock 欄位比 K 棒日期——**條件恆真**，
`max_analyses=1` 照樣回 28 條鏈。單元測試抓不到，因為它測的是 Go 的映射而不是 SQL 的跨軸比較。

同一個根因也造成**顯示回歸**：2330 的 28 條鏈會全部顯示成在同一秒內發生，而 snapshots
橫跨一個月。舊實作用 `market_event_states.analyzed_at`（K 棒日期）所以內部一致，
**這是換來源引入的新問題**，而門檻①（鏈數相同）驗不到它。

**修法**：`event_transitions.analysis_id` join 回 `stock_sr_zone_analyses.analyzed_at`，
對外一律用 K 棒軸；視窗過濾改用 `EXISTS(join analyses)`。沒有 `analysis_id`（排程收尾）
才退回 wall clock。新增兩支測試釘住，其中 helper 刻意讓 `occurred_at` 與 K 棒時間差好幾天，
任何把兩者搞混的實作都會炸開。

##### 實作結果（2026-08-20）

| 門檻 | 結果 |
|---|---|
| Go 全量 `test.sh` | 全綠 |
| ① 端點鏈數 vs `event_instances` | **28 / 38 / 37 / 25 逐檔相同** |
| ② 漂移過 key 的身分 | 用過 5 個 `zone_key` 的身分在新端點上是**一條**鏈（`seq=1`）；舊實作會是 5 條 |
| ③ `max_analyses` 真的限制 chains | 1→0、3→4、10→10、500→28，且鏈的時間範圍與 snapshots 視窗一致 |
| 時間軸 | `identity_since` 由 wall clock 的 `2026-08-20` 變成 K 棒軸的 `2026-07-20` |
| 決策路徑 | 本筆只動讀取端，沒有任何寫入 |

`max_analyses=1` 回 0 條鏈是**正確**的：最後一次分析沒有產生任何轉換，且 2330 的 28 條鏈
全部已終結；`snapshots=1` 仍然揭露「那天有跑分析」。

##### 完成後歸檔

* timeline 改讀身分層、`display_chain` 的新語意與 `identity_since` →
  [`api-reference.md`](./api-reference.md)「GET /sr-zones/event-timeline」。
* 「身分層的第一個讀者」與 U1 的取捨（為什麼不能在讀取時重算 key → uid）→
  [`sr-zone-scoring.md`](./sr-zone-scoring.md)「事件層：鏈的身分與三段關聯決策」。
* I-080 修復後改狀態；I-079 的 alias 上限在此成為「讀取端不依賴 alias」的佐證，於該筆補一句。

#### 驗收門檻

* 同一組四檔 21 階 as-of 階梯，端點回傳的鏈數與 `event_instances` 的鏈數**逐檔相同**。
* 至少一個「漂移過 `zone_key` 的身分」在新端點上是**一條**鏈（現行會是多條）。
* 決策路徑逐欄不變——本筆只動讀取端。

---

### T-052：定期對 watchlist 產生 SR zone 分析（分析排程）

| 欄位 | 內容 |
|---|---|
| 狀態 | **已實作／待 review**（2026-08-20；上線＝把 `sr_analysis.enabled` 設成 true） |
| 優先度 | 高（同時是 T-049 的硬前置與三筆驗證缺口的唯一解） |
| 分類 | Go / 排程 / SR Zone / 驗證母體 |
| 建立日期 | 2026-08-20 |
| 來源 | T-048 全案 review。T-049 已列它為前置，但**先前沒有任何 todo 在追**，只散見於 T-045 與 T-049 的討論段落 |
| 關聯 | [`issue.md`](./issue.md) I-074（Lifecycle Engine RR 解耦的 replay 驗證無法執行）、I-078（身分層兩條路徑從未被執行）；T-049 的前置條件② |

#### 問題

`stock_sr_zone_analyses` 的 production live DB 母體長期停在極小規模
（2026-08-18 盤點：4 檔 / 20 次分析），
所有需要「分佈比較」的驗證因此都做不了。目前卡在這一點的至少有三筆：

* **I-074**：Lifecycle Engine 的 RR 解耦只有單元測試層級的證據，`MODE=replay` 的 decision replay
  跑不起來。
* **I-078**：T-048 身分層的 `EXPIRED` 收攤與 alias 備援兩條路徑，在 84 次分析的母體裡
  一次都沒被觸發——身分還來不及缺席到失格。
* **T-049**：本身就把「母體要足以做分佈比較」寫成不可放寬的前置門檻。

as-of 階梯可以造出深度（同一檔多個時間點），但造不出廣度，也造不出「真實使用節奏下
身分會不會失格」這種只有時間會給的答案。

T-048 收斂時的 **4 檔 / 84 次** 是 isolated/as-of 階梯驗證 fixture，不是 production
自然母體；它能證明「改動前後逐欄相同」與「身分層數字重現」，但不能關閉 I-074 / T-049
要求的 production 分佈比較。

#### 計畫書（**已實作／待 review**，2026-08-20）

##### 修改目標與不做的範圍

* **目標**：每個交易日收盤後，對 **watchlist** 的每一檔跑一次帶身分追蹤的 SR zone 分析，
  讓 `stock_sr_zone_analyses` 開始自然累積 production 母體。
* **不做**：不改任何分析／決策／身分層邏輯——本筆只是**多一個呼叫端**。
* **不做**：不動 `evaluation_universe`（理由見下方範圍定案）、不改 watchlist 內容、
  不動前端、不新增 API 端點（除了既有 scheduler status 會多一個 job 名稱）。
* **不做**：不做補歷史（不回頭幫過去的日子補分析）——母體靠往前累積，不靠回填。

##### 六個定案

**S1：範圍是 watchlist，不是 `evaluation_universe`。**

[`architecture.md`](./architecture.md)「兩個標的清單」的分工表已經把這件事定死了：
`evaluation_universe`（135 檔）**唯一職能是日 K 維護**，「不做任何分析，也不參與任何交易
決策或狀態推導」，那是 T-040 的核心約束；`watchlists`（11 檔）才是做 SR zone 驗證與
production 分析的那一份。本筆補的正是那張表裡「production 分析」這一格**目前其實不存在的
排程**——現有的 `runSRZoneVerification` 只驗證既有 zone，不產生新分析。

母體規模：11 檔 × 每交易日 1 次 ≈ **每月 220 次分析**，對照現況 20 次（累積數月）。

**S2：排程必須走 handler 那條帶身分追蹤的路徑，且只能有一份實作。**

身分追蹤（`matchZoneIdentity` → `applyZoneUIDs` → `repo.Create` → `persistZoneIdentity`
→ `persistEventIdentity`）目前**只存在於 `SRZoneHandler.Create`**。
`analysis.SRAnalysisProvider` 那條（`reuse_existing=true`）**不寫 `zone_uid`、不追身分**，
所以不能用——用了就當不成 I-078 的關閉條件。

**不能讓 scheduler 直接 import handler**：`api/handler` 已經 import `scheduler`
（`SchedulerHandler`），反向會是 import cycle。

作法：**scheduler 自己宣告一個窄介面，由 `main.go` 注入 handler**（Go 的慣例：介面由消費端
定義）。handler 把 `Create` 的核心抽成一個可重用的方法，`Create` 與排程**呼叫同一份**：

```go
// scheduler 端
type SRAnalysisRunner interface {
    RunAnalysis(ctx context.Context, symbol, timeframe string, limit int) (uint64, error)
}
// handler 端：Create 與排程都呼叫它，確保永遠只有一份身分追蹤邏輯
func (h *SRZoneHandler) RunAnalysis(ctx context.Context, symbol, timeframe string, limit int) (uint64, error)
```

考慮過但否決：

| 方案 | 否決理由 |
|---|---|
| 把整包身分邏輯搬到 `internal/analysis` | 正解但是大重構（`sr_zones.go` 的身分相關函式與測試整批搬家），風險與本筆的目標不成比例。**列為之後的獨立重構**，不夾帶在這裡 |
| 排程內部打自己的 HTTP `/sr-zones` | 該路由是 protected，排程要自製 JWT；而且為了呼叫自己繞一趟網路 |
| 在 scheduler 複製一份身分追蹤 | 兩份會漂移，而漂移時**沒有任何東西會報錯**——那正是 T-048 一路在解的那類問題 |

**S3：獨立 cron，而且是兩段——17:00 不含當日籌碼、22:00 含當日籌碼。**

先講為什麼獨立而不掛在 `RunDailyClose` 尾端（`sr_zone_verify` 是掛尾端的先例）：

* daily_close 失敗（FinMind 抓空是實際發生過的事）不該連帶讓分析整天不跑。
* 獨立 cron 才有獨立的 `job_runs` 與獨立的手動觸發入口，出事時看得出是誰失敗。

**為什麼要兩段**：SR 分析吃籌碼（`trading_score` 的 Chip 佔 15%，`chip_summary` 也進決策），
而 FinMind 的法人／融資券要傍晚到晚間才發布——`chip.sync.cron` 因此是 `0 21 * * 1-5`。
17:00 跑到的必然是**前一日的籌碼**。所以：

| 時段 | 內容 | 為什麼是這個時間 |
|---|---|---|
| **17:00** | 當日 K 棒 ＋ **前一日籌碼** | 收盤後盡快有一份可看的分析。已晚於 `daily_close`（15:00）與 `evaluation_universe_sync`（16:00） |
| **22:00** | 當日 K 棒 ＋ **當日籌碼** | 籌碼排程 21:00 **開始**，排 21:00 會與它對跑。既有先例 `sr_evaluation` 排 22:30 並註明「預設晚於 chip sync」，本筆插在中間 |

兩段各自有**前置檢查，不符就 skip 該檔而不是失敗**：

* 兩段共同：該檔最新 candle 的交易日必須是今天（一次處理掉假日、停牌、daily_close 未完成）。
* 22:00 專屬：該檔籌碼的 `trade_date` 必須是今天。**籌碼沒進來就跳過**——那一輪算出來的東西
  會與 17:00 那份一模一樣，白跑一次還多推一次 `observed_absences`（見 S4）。

**S4：一天兩次會讓 `observed_absences` 前進兩次，這是刻意接受的。**

`next_observed_absences` 的規則是「配到歸零、沒配到 +1」，而它是 **per 分析**不是 per 日
（`zone_matcher.py`；已知限制「`as_of` 取的是 wall clock，不是資料日期」的同一個根源）。
一天兩次之後，`MAX_OBSERVED_ABSENCES = 3` 的實質意義從「約 3 個交易日」變成
**「約 1.5 個交易日」**——zone 身分會比現在早一倍失格。時間軸
（`MAX_ABSENCE_TRADING_DAYS = 20`，用交易日）不受影響。

**定案是接受並記錄，不動常數**，理由有二：

1. 次數軸的語意本來就是「我們看了幾次都沒看到」，不是「過了幾天」——兩個軸並存正是為了
   分辨這兩件事。看得更頻繁、更早判失格，符合它的原意。
2. 它會讓 [`issue.md`](./issue.md) **I-078 的 `EXPIRED` 收攤更快被觸發**，而那正是本筆要
   幫忙關閉的缺口之一。

若之後想維持「約 3 個交易日」的等價語意，選項是把 `MAX_OBSERVED_ABSENCES` 調成 6——
但那是 matcher 常數、屬身分層可見改變，**要另案並附實測**，不在本筆範圍。

**S5：守衛是「每時段一次」，不是「每日一次」。**

S3 改成兩段之後，「當日已分析過就跳過」會直接把 22:00 那輪整個擋掉，所以守衛的粒度是
**(交易日, 時段)**：同一個時段今天已經跑過才跳過，17:00 與 22:00 互不影響。

I-077 修完後（**必須先上線**，見下方前置），同時段重跑不再影響老化，所以這個守衛只是省一次
Python scoring ＋ 少推一次 `observed_absences`（S4）。

**跳過就是完全不跑 matcher，因此 `observed_absences` 不會增加**——這是對的：那個計數的
語意是「我們看了幾次都沒看到這個 zone」，而跳過的那次我們根本沒看。

**S6：序列執行、單檔失敗不中斷、`enabled` 預設 false。**

* **序列**（一檔跑完再跑下一檔）。11 檔 × 一次 scoring，這台 2GiB host 撐得住——
  **注意 `sr-zone-scoring.md`「規模上限」講的是 `run_evaluation()` 把所有標的 DataFrame
  一次全載，與本筆逐檔 scoring 是兩回事**，本筆的峰值就是單檔的 frame，
  與使用者現在手動點一次分析完全相同。
* 單檔失敗只記 warn 並繼續，最後由 `job_runs` 記 `total` / `failed` / `last_err`。
* 行程內 `atomic.Bool` 擋重複觸發（比照 `universeSyncRunning`）。
* `enabled` 預設 **false**，比照 `evaluation_universe` / `sr_evaluation` 的既有 pattern；
  live 上線是一個明確的 config 動作。

##### 受影響檔案與資料流

```text
config           ＋ sr_analysis: {enabled, cron, chip_cron, timeframe, limit}
Go api/handler/sr_zones.go   Create 的核心抽成 RunAnalysis(ctx, symbol, timeframe, limit)
                             ——**行為不變**，Create 改為呼叫它
Go scheduler/scheduler.go    ＋ SRAnalysisRunner 介面、runSRAnalysis(slot)、兩個 cron 註冊、
                             markRegistered("sr_analysis" / "sr_analysis_chip")、
                             atomic.Bool 重複觸發守衛（兩個時段各一個）
Go cmd/server（或 api/server.go）  把 handler 注入 scheduler（接線）
Go api/handler/scheduler.go  ＋ 手動觸發入口（比照 RunDailyClose）
```

* Python：**不改**。
* 前端：不改（`/scheduler/status` 會多一個 job 名稱，是純新增）。
* DB：**不新增表也不新增欄位**。寫入的都是既有的 `stock_sr_zone_analyses` /
  `stock_sr_zones` / 身分層四張表 / `job_runs`。

##### 資料 contract 變化

| 變更 | 型態 | 相容性 |
|---|---|---|
| config ＋ `sr_analysis` 區塊 | 純新增，預設關閉 | 沒設＝行為與現在完全相同 |
| `/scheduler/status` 多 `sr_analysis` / `sr_analysis_chip` | 純新增鍵 | 啟用時才出現，比照既有 job |
| API／DB schema | **不變** | — |

##### 主要風險與回滾

| 風險 | 對策 |
|---|---|
| **I-077 未上線就開排程**，排程與人工同日各跑一次，老化一天前進 2，**污染的正是要累積的母體** | I-077 已於 2026-08-20 修復（待 review）。本筆上線前先確認該修法已在 live |
| 抽 `RunAnalysis` 時不慎改到 `Create` 的行為 | 純抽取、不改順序；`Create` 既有測試全綠即為證據，另加一支「兩個呼叫端走同一份邏輯」的測試 |
| 排程在 candles 還沒到位時跑，產生一批基於舊 K 棒的分析 | S3 的「最新 candle 交易日必須是今天」前置檢查，不符就 skip 並記 `job_runs` |
| 22:00 那輪在籌碼還沒進來時跑，等於白跑一次又多推一次 `observed_absences` | S3 的籌碼 `trade_date` 前置檢查，不是今天就 skip |
| 一天兩次讓 `observed_absences` 前進兩次，`MAX_OBSERVED_ABSENCES=3` 的實質意義縮成約 1.5 個交易日 | S4：刻意接受並記錄。它同時讓 I-078 的 `EXPIRED` 收攤更快被觸發。要維持日數等價得調常數，屬另案 |
| 11 檔逐檔 scoring 拖累這台 2GiB host | 序列執行；峰值等同使用者手動點一次分析。若實測不行，先降頻（改隔日）而不是加併發 |
| 母體開始累積後，`zone_key_aliases` 撞頂比例上升（I-079） | 那正是本筆要量測的東西之一，見驗收門檻 |
| 回滾 | `enabled: false` 即可停止；已寫入的分析留著無害（沒有任何決策讀身分層）。程式面 `git revert` |

##### 測試與驗證策略

* **單元（Go）**：`enabled=false` 或 runner 未注入時**不註冊** cron（比照 adjuster /
  evaluationUniverse 的既有測試）；cron 字串壞掉時只記 log 不中止；單檔失敗時其餘照跑且
  `job_runs` 的 `failed` 計數正確；candle 不是今天時 skip 而非 fail；重複觸發被 atomic 擋掉。
* **不變式（Go）**：`Create` 與 `RunAnalysis` 走同一份邏輯——抽取後 handler 既有測試全綠。
* **端到端（dev stack）**：手動觸發，確認
  1. 每檔各產生一筆 `stock_sr_zone_analyses`；
  2. 該次的 `stock_sr_zones.zone_uid` **非空**（證明走的是身分追蹤那條路，不是 provider）；
  3. `job_runs` 有對應紀錄；
  4. **同一時段**再觸發一次 → 全部 skip，不新增分析；
  5. **另一時段**觸發 → 照常產生分析（守衛是 per 時段，不是 per 日）；
  6. 22:00 那輪在籌碼未進來時 skip；籌碼進來後跑出的分析，其
     `data_quality.updated_at.chip` 是**今天**、且 `chip_summary` 與 17:00 那輪不同。
* **單元（Go）**：兩個時段的守衛互不干擾（跑過 17:00 不會擋掉 22:00）。
* **回歸**：`backend/scripts/test.sh` 全綠。Python 未改，不需重跑（但抽取若動到 client 呼叫，仍跑一次）。

##### 完成後歸檔

* 排程職能新增一列 → [`architecture.md`](./architecture.md)「兩個標的清單」的分工表
  （「SR zone 驗證 / production 分析」那一格從願景變成現況）。
* **兩段式排程的理由**（17:00 不含當日籌碼、22:00 含；以及 `observed_absences` 因此
  一天前進兩次）→ [`sr-zone-scoring.md`](./sr-zone-scoring.md)「資格閘門」與
  「四個已知限制」那兩段——次數軸的實質日數等價值改變了，那裡必須寫明。
* cron 預設值、`enabled` 語意、skip 條件與 `job_runs` 判讀 →
  [`development-workflow.md`](./development-workflow.md) 或 `architecture/data-pipeline.md`
  的排程段（比照 `corporate_action_sync` 的寫法）。
* 「排程走 handler 的 `RunAnalysis`、與 `Create` 同一份身分追蹤」→
  [`sr-zone-scoring.md`](./sr-zone-scoring.md)「Zone 身分與 ZoneMatcher → 接線」，
  並更新那裡「只接在 `reuse_existing=false` 那條路徑」的已知限制描述。

##### 實作結果（2026-08-20）

程式已完成，**但排程預設關閉，等同尚未上線**——實際開始累積母體是把
`sr_analysis.enabled` 設成 true 的那一刻。

**上線步驟**（2026-08-20 補齊）：改 `deploy.sh` 的 `SR_ANALYSIS_ENABLED="true"` 再重新部署。
環境變數已同時拉進 `docker-compose.yml`（正式）、`docker-compose.dev.yml` 與 `deploy.sh`
三處——**compose 的 `environment:` 是白名單，沒列的變數不會進 container**，第一版只加了
dev 那份，等於正式環境根本開不起來。**前置：I-077 的修法必須先在 live 生效。**

| 驗證 | 結果 |
|---|---|
| Go 全量 `test.sh` | 全綠（+6 支排程測試） |
| 註冊 | `enabled=true` 時多 **2** 個 cron entry；runner 未注入時 **0**（等同導入前） |
| 端到端① 產生分析 | 觸發一次 → watchlist 兩檔各一筆（`total=2 analyzed=2 skipped=0 failed=0`） |
| 端到端② **走的是身分追蹤那條路** | 新分析的 zone 全部帶 `zone_uid`（11/11、16/16） |
| 端到端③ `job_runs` | `sr_analysis` success、`symbols_total=2`、`failed=0` |
| 端到端④ 同時段重觸發 | 兩檔都 skip（`已分析過今日 K 棒`），不新增分析 |
| 端到端⑤ **籌碼守衛**（P1 修正後複驗） | dev DB 的 `chip_scores` 是 0 筆 → `with_chip=true` **兩檔都 skip**（`沒有任何籌碼資料`），分析數 88→88。**同一個觸發在 P1 修正前會產生 2 筆**（86→88），這個前後差異就是修正生效的直接證據 |
| 端到端⑥ **守衛是 per 時段** | 只在 P1 修正**前**實走過（17:00 跳過後，`with_chip=true` 仍照跑並新增 2 筆，86→88）。修正後 22:00 那輪多了「當日籌碼已入庫」的前提，而 dev DB 沒有籌碼資料，**無法在 dev 上重現這個組合**；改由單元測試 `TestSRAnalysisChipSlotRunsWhenTodayChipArrived` 覆蓋（17:00 已分析過今日 K 棒 ＋ 當日籌碼已入庫 → 22:00 照跑） |
| 端到端⑦ timeframe 隔離（P2） | 未在 dev 實走（要先造一筆同日的 5m 分析）；由 `TestSRAnalysisIgnoresAnalysesFromOtherTimeframe` 覆蓋 |

##### Review findings（2026-08-20，**已修正／待複審**）

* **P1：`sr_analysis_chip` 沒有真的確認「當日籌碼已入庫」就會跑。** 原實作只看最新分析的
  `chip_summary.trade_date` 是不是今天——但 21:00 的 chip sync 失敗或還沒寫完時，那個條件
  同樣成立（最新分析用的是昨日籌碼），於是 22:00 會拿昨日籌碼再產生一筆內容相同的分析，
  白算一次還多推一次 `observed_absences`，污染的正是 T-049 要用的 production 母體。
  與同檔註解「當日籌碼必須已入庫」直接矛盾。
  **已修正**：改為先查 `ChipScoreRepo.GetLatest`，`trade_date` 不是今天就跳過
  （`當日籌碼尚未入庫` / `沒有任何籌碼資料`），再套原本的「已用今日籌碼分析過」冪等檢查。
  原測試把錯誤行為固定成預期，已改寫並補上「籌碼停在昨天」「完全沒有籌碼」兩個案例。
* **P2：skip 判斷沒有帶 `timeframe`。** 原本用 `srZoneRepo.List(symbol, 1)`，而 `List`
  只按 symbol 過濾——使用者今天手動跑過一次 5m 分析，就會讓 1d 的排程誤判「今天已經分析過」
  而整批跳過，`sr_analysis.timeframe` 這個設定形同失效。
  **已修正**：新增 `SRZoneRepo.GetLatestByTimeframe`（`WHERE symbol=? AND timeframe=?`）
  並改用它；補一支測試鎖住「5m 的分析不擋 1d 的排程」。
* **P3：`docker-compose.dev.yml` 漏了 `SR_ANALYSIS_TIMEFRAME`。** compose 的
  `environment:` 是白名單，沒列的變數不會進 container，於是 dev stack 無法覆寫 timeframe。
  **已修正**。

實作與計畫書的差異：

* **多注入了一個 `store.CandleRepo`**。計畫書寫「job 開頭檢查最新 candle 的交易日是不是
  今天」，但 scheduler 原本沒有任何 candle 查詢管道。改成由 `SetSRAnalysis` 一起注入，
  比照既有選填相依的處理。
* **跳過的判斷全部從資料推導，沒有行程內狀態**（最新 K 棒的交易日、最新分析的 K 棒交易日、
  該筆分析用的籌碼日）。這比原本設想的「行程內記住今天跑過沒」好：重啟後行為一致，
  而且兩輪的規則本來就必須不同。
* **`docker-compose.dev.yml` 也要補環境變數**。第一次驗證時 `/scheduler/status` 回
  `disabled`，原因是 compose 的 `environment:` 是白名單，沒列的變數不會進 container。

##### 前置

**I-077 必須先在 live 生效**（老化單位改為 K 棒推進）。理由見上方風險表第一列。

#### 驗收門檻

* 排程連續運行一段時間後，production `stock_sr_zone_analyses` 的母體足以跑
  `MODE=replay scripts/run-evaluation.sh` 做分佈比較（I-074 的關閉條件）。
  **母體要按 (symbol, 交易日) 去重再計數**：兩段式一天會產生兩筆分析，兩筆站在**同一根
  K 棒**、只有籌碼不同，當成兩個獨立樣本會高估母體。做分佈比較時取當日**含籌碼那一筆**
  （22:00 那輪；當日籌碼沒進來時只會有 17:00 那筆）。
* `zone_instances` 出現 `EXPIRED`，且 EXPIRED 收攤行為與單元測試一致（I-078 的第一個關閉條件）。
* `MatchedByAlias` 不能假設一定會因排程自然變成非零：T-048 實測中第一段既有鏈命中會吃掉
  多數情況。排程上線後需設定觀察期限；若仍為 0，改用 targeted integration/live fixture
  或 T-050 metric 證明 alias 備援路徑，而不是把 T-052 卡死在不可控的自然觸發上。

#### 與其他條目的先後（2026-08-20 定）

**本筆是整條鏈的時鐘起點**，母體累積要數週，所以要最早啟動；其餘工作填滿等待期：

本筆於 **2026-08-20 在 live 啟用**（`SR_ANALYSIS_ENABLED=true`）。

**同一個交易日的兩輪分析在多數欄位上完全相同**（`analyzed_at` 是 K 棒時間、`current_price`
是該根收盤，兩者都一樣；只有 `created_at` 與籌碼日不同），判讀規則與前端的呈現方式見
[`sr-zone-scoring.md`](./sr-zone-scoring.md)「`trade_date`：這份籌碼是哪一天的」。

| 時序 | 項目 | 理由 |
|---|---|---|
| 上線前 | I-077 老化單位修法在 live 生效 | 見上方計畫書的「前置」與風險表第一列，不先修就會一邊累積一邊污染母體 |
| 與本筆並行 | [T-051](#t-051event-timeline-改讀身分層讓修好的分裂真的看得到) | 零依賴，且是 T-048 唯一使用者看得到的成果 |
| 累積期**當中** | [T-050](#t-050身分追蹤的可觀測性把關聯決策計數從-log-升級成可查詢的-metric) | 要趕在累積期內上線才看得到 alias 命中率與撞頂比例的**趨勢**，事後補等於白等一輪 |
| 母體足夠後 | 並行比對 → I-074 關閉 → I-078／I-079 重新量測 | 純驗證，不寫功能 |
| 全綠後 | [T-049](#t-049market-state-與所有下游改讀同一套-state) | 動決策邏輯，另需計畫書 |


---

### T-054：用 Redis 降低 Python 端 SR Zone 分析記憶體？——評估結論：不採用

| 欄位 | 內容 |
|---|---|
| 狀態 | 待規劃（評估已完成，結論為**不走 Redis**；下方「可行的槓桿」待決定要不要做） |
| 優先度 | 低 |
| 分類 | Python / SR Zone / 效能 |
| 建立日期 | 2026-08-21 |
| 來源 | 使用者觀察「每次分析記憶體用量會增加」，要求評估以 Redis 外移資料 |
| 相關 | [T-047](#t-047sr-evaluation-串流化解除全市場的記憶體上限)（批次路徑的記憶體上限） |

#### 結論

**Redis 在 `/sr-zones` 這條路徑上沒有可外移的東西，不採用。**
實測 `python-server` 穩態 RSS 約 **254MB，其中 253MB 是直譯器＋套件＋模型**，
單次分析的資料只有約 **16KB**（400 根 K × 5 欄 × 8B）。Redis 搬得走資料，
搬不走 import。

另外，「每次分析記憶體用量會增加」**不是洩漏，是暖機**：前 3 次分析從 140MB 爬到
255MB 之後就持平，連續 75 次分析後反而微降。

#### 量測一：`/sr-zones` 連續 75 次（dev stack，postgres，`limit=400`，4 檔輪流）

RSS 取容器內 PID 1 的 `VmRSS`（不含 page cache），每次分析後取樣。

| 分析次數 | RSS | 說明 |
|---|---|---|
| 0（idle，模型已載入） | 139.6 MB | 呼叫過 `/sr-scoring/model-status` 後的基線 |
| 1 | 166.8 MB | +27 MB |
| 2 | 167.2 MB | 幾乎不動 |
| 3 | 259.2 MB | +92 MB，暖機完成 |
| 4～75 | 253.7～259.2 MB | **持平**，末段 254.1 MB（比峰值低） |

單次分析耗時 6.9～10.7 秒。**瓶頸是 CPU 不是記憶體**，與
[`sr-zone-scoring.md`](./sr-zone-scoring.md)「規模上限」對 evaluation 的結論一致。

RSS 不會退回 140MB，是因為 CPython／glibc 不把釋放的 arena 還給 OS，加上
`shap` 是**延遲 import**（`evidence.py` 的 `_shap_available()` / `_build_explainers()`
都在函式內 `import shap`），所以它的成本要等第一次真的產 evidence 才付。
這就是使用者觀察到的「每次分析都在增加」——它有上界，且上界就是下面那張表的總和。

#### 量測二：記憶體到底花在哪（同一個 image，拋棄式 container，`--memory=320m`）

| 階段 | 累積 RSS | 增量 |
|---|---|---|
| bare interpreter | 7 MB | — |
| + numpy | 23 MB | +16 |
| + pandas | 64 MB | +41 |
| + sqlalchemy | 79 MB | +15 |
| + sklearn | 158 MB | **+79** |
| + lightgbm | 161 MB | +3 |
| + shap | 221 MB | **+60** |
| + fastapi / uvicorn | 238 MB | +17 |
| + `joblib.load(sr_scoring_v4.joblib)` | 253 MB | +15 |

同一支探針**跳過 `import shap`** 時停在 **181 MB**——所以 shap 連同它拉進來的
相依套件實際佔 **72 MB**。

模型本身很小：檔案 4.2MB，`explanation_background` 只有 `(32, 15)` = 3,840 bytes。
**SHAP 的成本在 import，不在資料。**

#### 為什麼 Redis 幫不上忙

1. **沒有資料規模可以外移**。254MB 裡真正屬於「這次分析的資料」的部分是 16KB 等級。
   把它放進 Redis，Python 端省下的量小到量不出來。
2. **省不掉的那 253MB 依定義不能外移**。pandas / sklearn / shap 的 module 物件與
   原生庫必須留在會用到它們的行程裡；模型要拿來 `predict_proba` 就得是反序列化後的
   Python 物件。存進 Redis 只是多一份序列化拷貝，用的時候還是要載回來。
3. **Redis 與 Python 在同一台 host**。這台只有 2GiB，`redis` 是 `docker-compose.dev.yml`
   的服務、跟 `python-server` 共用同一份實體記憶體。把 X MB 從 Python 搬到 Redis，
   host 總用量不變（還多了 redis-py client 與序列化緩衝）。
   Redis 要能真的降低本機壓力，前提是它跑在**別台機器**上——目前不是這個架構。

批次路徑（`run_evaluation`）也一樣：全市場情境下最大的一塊確實是原始 frames（約 220MB），
但 T-047 已經給出對的解法——**逐檔算完 profile 就釋放 frame** 的串流化。
Redis 版本要付出同樣的峰值（用的時候還是要載回 pandas），外加序列化成本與 Redis 自己的 RSS。
串流化嚴格優於 Redis。

#### 可行的槓桿（真的想降記憶體時做這些）

| 手段 | 省下 | 代價 | 需要改程式嗎 |
|---|---|---|---|
| **A. 關掉 SHAP evidence** | **72 MB（253→181，−28%）** | evidence 降級為 rules only，前端 badge 顯示「rules only」；規則式分數與機率不受影響 | **不用**。`config.py` 已有 `SR_SCORING_EVIDENCE_ENABLED` 環境變數覆寫，compose 加一行即可，可逆 |
| B. 把 SHAP 隔到獨立子行程，用完回收 | 同 A，但保留 evidence 功能 | 每次分析多一次 fork ＋ import 成本（shap import 本身就要數秒），熱路徑延遲會明顯變差 | 要，且動到 `evidence.py` |
| C. 批次路徑串流化 | 見 T-047 | 見 T-047 | 見 T-047 |

A 是唯一「零程式碼、立刻可逆、省下四分之一」的選項。
`build_evidence()` 的 `shap_ready = bool(evidence_enabled) and _shap_available() and ...`
會短路，關掉時 `_shap_available()` 根本不會執行，shap 因此永遠不被 import。

**本評估不建議現在就關**：dev 穩態 254MB 對 512m 的 `mem_limit` 還有一倍餘裕，
沒有實際壓力就沒有理由犧牲 evidence。這一列是「真的撞到上限時先拉這根桿」的備案。

#### 順帶發現（待確認，不屬於本評估範圍）

`docker stats` 顯示 **live stack 的 `stock_trading-python-server-1` 啟動 2 分鐘就到 323MB**，
比 dev 的穩態 254MB 高約 70MB。live 開了 `SR_ANALYSIS_ENABLED`（T-052），
可能是排程分析的併發或 `SR_ANALYSIS_LIMIT=400` 下 zone 數較多所致，但沒有查證。
若之後要收 `mem_limit`，要先量 live 的實際穩態，不能拿 dev 的數字外推。

#### 驗證方式

重跑量測一即可（容器外對 `http://127.0.0.1:18001/sr-zones` 連續打，
每次後 `docker exec … grep VmRSS /proc/1/status`）。
判準是**穩態是否收斂**，不是單次峰值——單看前 3 次一定會看到成長。

---

### T-055：RR 語意分層——Setup RR 與 Executable RR 必須是兩個數字，門檻必須具名分層

| 欄位 | 內容 |
|---|---|
| 狀態 | 規劃中（**計畫書第 3 版：已納入 2026-08-21 review R1／R2／R3／R5、同日「裁決前建議」與 F1 定案；契約全部定案、無待決項，尚未實作**） |
| 優先度 | 中 |
| 分類 | Python / SR Zone / 決策語意 |
| 建立日期 | 2026-08-21 |
| 來源 | live 0050 分析（`analyzed_at=2026-08-20`）出現「Entry RR 1.87R 通過」與「RR 未達完整買進門檻」並存 |
| 相關 | 壓力分層（原 T-056，**已完成並移出本清單**）——本筆要引用的 `blocking_resistance_zone` 由它落地，欄位分工見 [`sr-zone-scoring.md`](./sr-zone-scoring.md)「壓力分層」 |

#### 核實結果：現象屬實，但成因有兩層，且與原始推測不完全相同

live 資料（`stock_sr_decisions`，0050 / 1d / `analyzed_at=2026-08-20T16:00Z`）：

```json
"rr_gate":    { "actual_rr": 1.8664, "minimum_rr": 1.5, "qualified": true, "reason_code": "RR_QUALIFIED" }
"risk_notes": [ "風險報酬比未達完整買進門檻，Final Entry 需保守觀察。", ... ]
"rr_context": { "entry_rr": 1.8664, "execution_rr": 1.8664, "execution_rr_source": "PRIMARY_ZONE",
                "entry_price": 104.1114, "stop_price": 103.4886, "target_price": 105.2738,
                "target_basis": "PRIMARY_ZONE_RR", "price_basis": "PRIMARY_SUPPORT_UPPER",
                "executable_now": false, "entry_executability_reason_code": "ENTRY_ZONE_OVERSHOT" }
```

**成因一：同一個數字被兩套門檻判定（這是「通過 vs 未達門檻」的直接來源）**

| 判定點 | 位置 | 門檻 | 對 1.8664 的結果 |
|---|---|---|---|
| `rr_gate` | `decision_engine.py::_rr_gate` → `_minimum_rr` | 動態 **1.5 / 1.8 / 2.0**（依 `entry_action_state` / tier / role；本例 `PROBE_ENTRY` → 1.5） | `qualified=true` |
| `risk_notes` 階梯 | `decision_engine.py:496-502` | **寫死 1.5 / 2.0** | `1.5 ≤ rr < 2.0` → `RR_BELOW_FULL_ENTRY` |

兩句話各自為真，但讀起來互相矛盾。**注意：就算 `_minimum_rr` 走的是預設 1.8 分支，
1.8664 仍會同時「通過」與「未達完整買進門檻」**——所以這不是 `PROBE_ENTRY` 特例，是門檻本身有兩套。

**成因二：這條路徑根本沒有真正的 Executable RR（原始推測命中的是這一層）**

`execution_rr` 與 `entry_rr` 完全相同、`execution_rr_source=PRIMARY_ZONE`，因為
`_rr_context` 只有在 **市價型 entry**（`price_basis ∈ {RECLAIM_CLOSE, CONTINUATION_MARKET_PRICE}`）
才會走 `NEAREST_RESISTANCE_TARGET` 算真 RR；本例 `price_basis=PRIMARY_SUPPORT_UPPER`，
落到 `elif entry_rr is not None` 分支，直接**把 setup RR 抄成 execution RR，再反推 target**：

```
target_price = entry_price + risk × entry_rr = 104.1114 + 0.6228 × 1.8664 = 105.2738
```

這個 target 是**由 RR 反推出來的產物，不是任何真實價位**，而且它落在
`entry_blocking_zone` 的壓力區 **105.1862 ~ 110.9983 之內**（超出 0.0876）——
同一份決策一邊說「前方壓力擋住進場」，一邊把獲利目標設在那道壓力後面。

用同一組已存數字換算，真正可執行的 RR 是（以下為本計畫書的推算，非系統輸出）：

| 情境 | entry | stop | target | RR |
|---|---|---|---|---|
| 系統目前顯示 | 104.1114 | 103.4886 | 105.2738（反推） | **1.87** |
| target 封頂在擋路壓力 | 104.1114 | 103.4886 | 105.1862 | **1.73** |
| 以現價進場（實際只能這樣） | **104.65** | 103.4886 | 105.1862 | **0.46** |

`executable_now=false` / `ENTRY_ZONE_OVERSHOT` 已正確指出「104.1114 進不去了」，
但畫面上的 RR 仍是用那個進不去的價位算的。**系統的擋單決策是對的，錯的是它展示的數字。**

#### 修改目標

1. **門檻具名分層（R1 修正，原「收斂成一套」作廢）**：不是把所有判斷折成 `_minimum_rr()` 一個值——
   那會吃掉 Full Entry 語意。改成兩個**具名**門檻，各自有明確受眾：

   | 門檻 | 目前值 | 用途 | 對應現況 |
   |---|---|---|---|
   | `probe_min_rr` | `_minimum_rr()`（1.5 / 1.8 / 2.0，依 state / tier / role） | 這次提議的動作能不能放行 | `rr_gate.minimum_rr` |
   | `full_entry_min_rr` | 2.0 | 夠不夠格做完整部位 | ladder 的 `< 2.0` 與 `strong` 的 `rr >= 2.0` |

   **為什麼 0050 會踩到**：2.0 在 `_minimum_rr()` 只出現在 Tier-1／Resistance 分支
   （`decision_engine.py:1204-1207`）。Tier-1 zone 的 gate 與 full entry 剛好同值，折成一個不會出事；
   0050 的 primary 是 **TIER_3 support**，gate 1.5（PROBE_ENTRY）而 full entry 2.0，**兩者必然分岔**。

   **定案（2026-08-21 裁決）**：保留兩個 gate，`rr_gate` 加 `gate_kind`；
   前端主顯示一個 authoritative gate，輔助顯示另一個。

   ```jsonc
   "rr_gate": {
     "gate_kind":   "PROBE",                 // 只有 PROBE / FULL_ENTRY 兩值，見 F1 定案
     "minimum_rr":  1.5,                     // 這一層的門檻
     "actual_rr":   1.7257,                  // 被測試的 RR（封頂後的 executable_rr）
     "gate_basis":  "ENTRY_STOP_TARGET",     // actual_rr 是怎麼算出來的，見 F1 定案
     "qualified":   true,
     "reason_code": "RR_QUALIFIED",
     "zone_actual_rr": 1.8664,               // setup RR，保留供對照（既有欄位）
     "secondary_gate": { "gate_kind": "FULL_ENTRY", "minimum_rr": 2.0, "qualified": false }
   }
   ```

   **兩層 gate 測的是同一個 `actual_rr`，只有門檻不同**——這正是讓「通過」與「未達門檻」
   不再矛盾的關鍵性質，所以 `secondary_gate` **不帶自己的 `actual_rr`**。

   * `probe_min_rr` / `full_entry_min_rr` 是**內部命名與文件術語**，不是對外欄位名；
     對外要能看出的是「目前用哪個 gate 仲裁」與「另一層 gate 的狀態」。
   * `risk_notes` 的 `RR_BELOW_FULL_ENTRY` **保留原名**（既有測試以 reason code 驅動，見
     `test_decision_engine.py:543-553`），但語意綁定到 `secondary_gate`。
   * 前端主卡只顯示 authoritative `rr_gate`，`secondary_gate` 以小字或次要列呈現，
     **不放兩張等權重卡片**——等權重會製造新的矛盾感，正是本筆要消滅的東西。

   **驗收語意**：修完後「通過」與「未達門檻」仍可並存，但**必須各自標明是哪個 gate**；
   不得出現不指明 gate 的裸「通過」。
2. **Executable RR 在所有 entry 路徑都要是真的**：zone 限價路徑也要有 target 來源，
   不得用 RR 反推 target；沒有可量化 target 時明確輸出 unknown
   （`gate_basis=TARGET_UNAVAILABLE` ＋ `rr_formula_available=false`，語意沿用既有的
   `MARKET_ENTRY_TARGET_UNAVAILABLE`，見 F1 定案的值域相容處置），而不是抄 setup RR。
3. **target 不得穿越擋路壓力**：`target_price` 需與 `entry_blocking_zone` 一致，
   封頂在擋路壓力的 `price_low`。
4. **`executable_now=false` 時，RR 顯示要跟著失效**：不可用進不去的價位算出的 RR 當主顯示。

#### 不做的範圍

* **不改 `_minimum_rr()` 的 1.5 / 1.8 / 2.0 分級本身**，也**不改 `strong` 的 `rr >= 2.0`**——
  兩者都是既有調校結果。本筆只替它們命名並讓對外顯示指明是哪一個，不動數值。
* **不改 `entry_blocking_zone` 的擋單門檻**（`max(zone_width×0.5, price×0.005)`，見
  [`sr-zone-scoring.md`](./sr-zone-scoring.md)）。
* 不改 `position_rr` 維持 `null` 的設計（SR Zone 不讀持倉成本）。
* **不把 `SUPPORT_RETEST_HELD` / `RESISTANCE_BREAKOUT` 接進 Decision**——見下方共同約束。

#### 受影響檔案與資料流

```
decision_engine.py::_decision_action:496-502        ← 門檻具名分層（成因一）
decision_engine.py::_rr_context:1237-1330           ← Executable RR 與 target 來源（成因二）
decision_engine.py::_execution_rr_gate:1332         ← 解除「只有市價型才算」的限制
decision_engine.py::_nearest_resistance_above_entry:1388  ← target 候選來源之一
decision_engine.py::_entry_blocking_zone_detail:1935 ← target 封頂來源（唯讀，不改門檻）
        ↓ decision_summary.rr_context / rr_gate / risk_notes
backend/internal/analysis/client.go → stock_sr_decisions.rr_context_json / rr_gate_json
        ↓
frontend/src/lib/api/srZones.ts（型別）→ SRZones.svelte:2309-2340（RR Gate ＋ RR 區塊）
```

**target 封頂的 arbitration（R2 補充，實作前必須定案）**

現況：`_rr_context()` 的簽章是 `(primary_zone, position_zone, entry_executability, defense_lines,
target_zone)`——**沒有 `entry_blocking_zone` 參數**，呼叫端只傳
`target_zone=_nearest_resistance_above_entry(...)`（`decision_engine.py:2481-2489`）。

**但呼叫順序已經是對的**：`entry_blocking_zone = _entry_blocking_zone_detail(...)` 在 **2434** 就算完，
`_rr_context(...)` 在 **2481** 才呼叫。所以不需要重排流程，只要把已算好的值傳進去——
這降低了本項的實作風險。

**定案（2026-08-21 裁決）**：target 取**所有前方壓力中最低的可用 `price_low`**；
`blocked=false` 時也封頂；由**呼叫端先算好 `execution_target` 再傳入** `_rr_context()`。

| 問題 | 定案 |
|---|---|
| target 原始來源 | candidates = `_nearest_resistance_above_entry()` ∪ `entry_blocking_zone.blocking_zone` ∪ `blocking_resistance_zone`（後兩者共用 `_blocking_resistance_zone`，恆指同一個 zone） |
| 過濾條件 | 只納入 `price_low > entry_price` 的 candidate |
| 衝突時誰優先 | **取最低 `price_low`**——Executable RR 問的是「entry 到第一道可量化前方阻力還有多少空間」，target 不得穿越任何前方壓力 |
| `blocked=false` 但前方仍有 resistance | **仍封頂**。`blocked` 只代表「近到足以擋單」，不代表「可以把 target 設在更後面的壓力」 |
| 傳參方式 | `_rr_context()` **不自己找 zone**；呼叫端算好 `execution_target` 物件再傳入 |

```jsonc
"execution_target": { "price": 105.1862, "basis": "FIRST_RESISTANCE_CAP", "source": "blocking_resistance_zone" }
```

**實作風險比預期低——正確的值早就算出來了，只是被丟掉。** 對 0050 這筆用
`entry_price=104.1114` 跑 `_nearest_resistance_above_entry()`：符合「`price_low > entry_price`
＋非 EXPIRED ＋非 LOW confidence」的 resistance 只有 **105.19~111.00**（105.1862）與
**107.18~107.82**（107.1775），取最近者即 **105.1862**——**正是定案要的 cap**。
它在 `:2485` 已經被算出來當 `target_zone` 傳進 `_rr_context()`，
卻因為 `market_price_entry=false` 而**整個分支沒被執行**，最後走 `PRIMARY_ZONE_RR` 反推出 105.2738。
所以主要工作是「讓非市價路徑也用這個值」，不是新建一套選擇邏輯。

**前置相依已解除且已完成**：結構／擋路壓力採 `blocking_resistance_zone`。該欄位已由原 T-056
（壓力分層）於 2026-08-24 完成並歸檔，Python／Go 投影／API 展開／前端皆已落地，**本筆可直接引用**。
語意與選法見 [`sr-zone-scoring.md`](./sr-zone-scoring.md)「壓力分層」——特別注意它**沒有 tier 過濾**，
不保證是結構性壓力。

#### 契約變化（**定案：方案 C 折衷，2026-08-21 裁決**）

不一次移除 `entry_rr` / `execution_rr`——歷史資料、evaluation 統計與前端都已消費它們。

| 欄位 | 處置 |
|---|---|
| `rr_context.entry_rr` | **保留**，文件改標為 setup RR 的 legacy alias |
| `rr_context.setup_rr` | **新增**，語意＝zone 歷史統計 RR（與 `entry_rr` 同值） |
| `rr_context.executable_rr` | **新增**，語意＝從 `entry_price` 到封頂 target 的實際 RR |
| `rr_context.execution_rr` | **保留**為 `executable_rr` 的 alias，**一版後再評估 deprecate** |
| `rr_context.execution_target` | **新增**（`price` / `basis` / `source`），見上方 arbitration |
| `rr_gate.gate_kind` | **新增**：`PROBE` / `FULL_ENTRY`（**只有兩值**，見 F1 定案） |
| `rr_gate.secondary_gate` | **新增**：另一層 gate 的 `gate_kind` / `minimum_rr` / `qualified`（**不含 `actual_rr`**） |
| `rr_gate.gate_basis` | **既有欄位，改為一律輸出**，值域擴為三值（見 F1 定案） |
| ~~`rr_gate.actual_rr_source`~~ | **不新增**——與 `gate_basis` 一對一重複，見 F1 定案 |
| `rr_gate.zone_actual_rr` | **既有欄位保留**，語意＝`setup_rr`；改為一律輸出 |

沿用既有語意不變的部分：`target_known=false` 時 `actual_rr=null` 但 **`qualified=true`**
（target 未知 ≠ RR 不合格，見 `_execution_rr_gate:1344-1352` 與 `sr-zone-scoring.md`），
`position_rr` 維持 `null`。

#### 風險與回滾

| 風險 | 對策 |
|---|---|
| **會改變決策輸出** | target 封頂會讓 RR 普遍下修，被擋掉的樣本變多——方向偏保守，但仍需 decision replay 前後比對 |
| `execution_rr` 語意改變破壞既有 evaluation 統計 | `execution_rr_distribution` 的歷史值不可跨版本比較，report 需帶 `pipeline_version` |
| 舊分析沒有新欄位 | 前端沿用「缺欄位安全降級」慣例（見 `sr-zone-scoring.md`） |
| 回滾 | 純 Python 決策層改動，無 migration；回滾＝還原 `decision_engine.py` |

#### 測試與驗證

1. 單元測試（`python/scripts/test.sh backtest/modular/sr_scoring/tests/test_decision_engine.py`）：
   * **迴歸案例直接用 0050 這組數字**（rr=1.8664 / probe min=1.5 / full entry min=2.0）：
     `rr_gate.gate_kind="PROBE"` 且 `qualified=true`，同時
     `rr_gate.secondary_gate={gate_kind:"FULL_ENTRY", minimum_rr:2.0, qualified:false}`；
     `RR_BELOW_FULL_ENTRY` 仍可出現，但**不得出現不指明 gate 的裸「通過」**。
   * `execution_target` 對 0050 必須是 **105.1862**（`basis=FIRST_RESISTANCE_CAP`），
     而非現況反推的 105.2738；對應 `executable_rr` ≈ **1.726**。
   * `blocked=false` 但前方仍有 `price_low > entry_price` 的 resistance 時，target 仍須封頂。
   * `price_basis=PRIMARY_SUPPORT_UPPER` 時 `execution_rr` 不得等於 `entry_rr`（除非兩者本來就相等且 target 為真實價位）。
   * `target_price` 不得 ≥ `entry_blocking_zone.blocking_zone.price_low`。
   * `executable_now=false` 時 `rr_gate.actual_rr` 的處置需有明確斷言。
   * **F1 契約**：`gate_kind` 值域只有 `PROBE` / `FULL_ENTRY`（出現 `EXECUTION` 即失敗）；
     `gate_basis` 一律輸出且僅三值；`secondary_gate` **不得**含 `actual_rr`；
     **不得**存在 `actual_rr_source` 欄位。
   * **F1 迴歸保護**：`reason_code` 值域不得變動；特別斷言
     `_final_entry_risk_notes` 對 `EXECUTION_RR_UNAVAILABLE` 的比對仍成立（`:602`）、
     `_final_entry_permission` 對 `qualified` 的判斷仍成立（`:848`）。
   * **F1 連帶後果**：`strong` 與 `secondary_gate` 必須讀同一個 `actual_rr`——
     建構一組 `executable_rr < 2.0 ≤ setup_rr` 的 fixture，斷言
     **不會同時出現 `action=Buy` 與 `secondary_gate.qualified=false`**。
2. **前端測試（R3 補充，原計畫缺漏）**——`frontend/src/routes/SRZones.test.ts`：
   * `executable_now=false` 時，主顯示 RR **不得**使用 setup／`entry_rr`；
   * `execution_rr` 有值時要顯示 executable RR 及其 `execution_rr_source` / `gate_basis`；
   * RR Gate 卡片必須同時顯示 `actual_rr` 與 `gate_kind`，且
     `secondary_gate` 以次要樣式呈現（不得等權重）；
   * `execution_rr` 為 unknown 時要明確顯示 unknown／target unavailable，不得留白或退回 setup RR；
   * 舊分析缺新欄位時安全降級（沿用既有慣例）。

   **為什麼一定要有這條**：現行 `SRZones.svelte:2326-2340` 的 RR 區塊只有「進場 RR (Entry)」
   （`entry_rr`）與「持股 RR (Position)」（`position_rr`），**`execution_rr` 根本沒有顯示位置**。
   後端就算修出正確的 executable RR，畫面仍會繼續把 setup RR 當主 RR——等於白修。

   另一個現況問題一併處理：`rr_gate` 區塊（`:2309-2324`）顯示 `qualified` / `minimum_rr` /
   `reason_code`，但**不顯示 `actual_rr`**。所以使用者看到的「1.87」來自 RR 區塊、「通過」來自
   Gate 區塊，**兩個數字分屬不同區塊、沒有任何標示說它們同源**——這放大了本筆要修的矛盾感。
3. decision replay（`POST /sr-scoring/evaluate`，`decision_replay=true`）比對改動前後
   `by_rr_gate` / `by_rr_gate_reason_code` / `by_entry_executability` 分佈。
4. **驗收一律走 dev compose**，不得用 live 做測試資料。

#### 完成後歸檔

`docs/sr-zone-scoring.md`：更新「`rr_context` 將新進場 RR 與既有部位 RR 拆開」與
「`rr_gate` 是 final/execution gate 的對外結果」兩段，補上 Setup／Executable 的定義、
target 封頂規則，以及 `probe_min_rr` / `full_entry_min_rr` 兩層具名門檻的定義與顯示規則；
並補上 F1 定案的兩個正交軸（`gate_kind` 門檻層 / `gate_basis` RR 推導方式）與
`MARKET_ENTRY_TARGET_UNAVAILABLE` → `TARGET_UNAVAILABLE` 的值域遷移說明。

**本筆同時負責關閉 I-081 與 I-082 的文件項（2026-08-21 定，非選配）。** 兩者要改的
「Legacy action pipeline」第 3、4 條，正好是本筆重寫 RR 門檻敘述時會動到的同一段——
分兩次改等於同一段寫兩遍，且中間那版必然又對不上實作。

| Issue | T-055 必須完成的文件動作 | 完成後 |
|---|---|---|
| **I-081** | 第 3 條的 `risk_reward_ratio < 1.0` 改成正確門檻。**不是把 1.0 改成 1.5 了事**——本筆會把門檻改成 `probe_min_rr` / `full_entry_min_rr` 兩層，該條要照新語意整條重寫 | I-081 整筆可移除 |
| **I-082** | 第 4 條改寫：EXPIRED 的 `Buy` 守門**不在第 4 步**，而在第 5～6 步的 `structure_broken` / `bearish_setup` 提前 return（`_structure_state:2242` 的 `EXPIRED → BREAKDOWN`）。敘述要指明擋在哪裡 | I-082 整筆可移除（迴歸測試已於 2026-08-21 先行補完） |

**T-055 的驗收條件因此多一條**：`sr-zone-scoring.md`「Legacy action pipeline」第 3、4 條
與實作一致，且 I-081 / I-082 可同步結案。漏掉這一條就不算完成。

**review 通過後才把本筆從 todo.md 移除。**（R5 的兩筆搬移已於 2026-08-21 完成，
即 I-081 / I-082 的建立，不再是移除時的待辦。）

#### 核實過程中的附帶發現（**R5 已結案：2026-08-21 搬入 `docs/issue.md`**）

兩筆都已建立 issue 條目，本筆不再保留內文，避免 `docs/todo.md` 變成 issue 暫存區：

| Issue | 內容 | 與本筆的關係 |
|---|---|---|
| [`issue.md`](./issue.md) **I-081** | `sr-zone-scoring.md` legacy action pipeline 第 3 條門檻寫 `< 1.0`，實作是 `< 1.5`（`decision_engine.py:499` 與 `:516`） | **已定案由本筆一併改**（2026-08-21）：本筆會把該段整條重寫成兩層具名門檻，見「完成後歸檔」 |
| [`issue.md`](./issue.md) **I-082** | ~~EXPIRED 的 primary zone 仍可能升級到 `Buy`~~ → **2026-08-21 重現實驗推翻**：行為正確，守門在 `_structure_state:2242`（`EXPIRED → BREAKDOWN`）而非 `strong`。剩下的只有文件錯位與缺迴歸保護 | **修法衝突已解除**（本筆不動 `strong`）。迴歸測試「EXPIRED primary 不得輸出 `Buy`」**已於 2026-08-21 先行補完**（3 條測試＋變異驗證），是本筆改 `_decision_action` 時的安全網。**剩下的文件改寫已定案由本筆一併處理**，見「完成後歸檔」 |

**I-082 的兩次判斷都被推翻，過程留在該筆**：先是誤判「`strong` 沒排除 EXPIRED 所以會發 Buy」，
接著又錯誤地把嚴重度上調。實際窮舉（SUPPORT／RESISTANCE × 三種 regime ＋ 對照組）顯示
**EXPIRED primary 走不到 `strong`**：SUPPORT 被 `structure_broken` 提前 return 擋下、
RESISTANCE 被 `bearish_setup` 擋下，而兩條 `_pick_primary_zone` 清單都排除 `AT_ZONE`。
嚴重度已由「中」下修為「低」，範圍縮小為**文件改寫 ＋ 補迴歸測試**。詳見 I-082。

---

#### 原始 Review findings（2026-08-21，**已由第 2／3 版處理，保留作決策沿革**）

> **本區塊（`原始 Review findings` 到 `裁決納入紀錄`）原為 T-055／T-056 共同的 review 沿革。
> T-056 已於 2026-08-24 完成、review 通過並歸檔，整筆移出本清單**，現況規格見
> [`sr-zone-scoring.md`](./sr-zone-scoring.md) 的「壓力分層：tactical / blocking / structural
> 是三個不同欄位」。本區塊自此**只服務仍在規劃中的 T-055**；文中所有 T-056 字樣都是當時的
> 決策沿革，`tactical_resistance_zone` / `blocking_resistance_zone` 的**現行契約以
> `sr-zone-scoring.md` 為準**，不要回頭找已移除的 T-056 小節。

> 文件核實補記（2026-08-21）：本節是原始 review findings，
> R1～R5 已於下方「Review 回應」與「裁決納入紀錄」全數處理並回寫進計畫本文。
> 本節與「裁決前建議」**只作決策沿革**，被第 3 版覆寫的欄位值已逐處加註。
> **實作一律以 T-055／T-056 本文的契約表與「裁決納入紀錄」（第 3 版）為準**，
> 不要拿本節或「裁決前建議」當實作契約。

以下是 T-055 / T-056 計畫書 review 當時的待修正點。這些 findings 在當時修完前，
兩筆都還不應進入實作；目前處理狀態以後續「Review 回應」與「裁決納入紀錄」為準。

**R1.（P1）T-055 把「唯一門檻」直接等同 `_minimum_rr()`，會吃掉 Full Entry 語意。**

目前計畫寫 `risk_notes` 階梯改讀 `_minimum_rr()` 的同一個值，讓「通過」與「未達門檻」在數學上
不可能同時成立。但 `_minimum_rr()` 回的是**當前 gate 門檻**：`PROBE_ENTRY=1.5`、
Tier-1／Resistance 為 `2.0`、其他為 `1.8`。如果 0050 這筆是 probe gate，`1.8664`
通過 probe 是合理的；同時未達 full-size entry 也可能是合理語意。

計畫應改成「**一套具名門檻／語意**」，而不是把所有判斷都折成 `_minimum_rr()` 一個值。實作前
至少要定義清楚：

* `probe_min_rr` 與 `full_entry_min_rr` 是否是兩個具名門檻；
* `rr_gate.minimum_rr` 指的是哪個 gate；
* `risk_notes` 要用哪個 gate 產生哪個 reason code；
* 前端要顯示「Probe 通過但 Full Entry 未通過」還是只顯示一個主 gate。

否則修完會把原本合理的「只允許試單，不是完整部位」語意消掉。

**R2.（P2）T-055 的 target 封頂資料流還沒定義完整。**

計畫要求 `target_price` 不得穿越 `entry_blocking_zone`，要封頂在
`entry_blocking_zone.blocking_zone.price_low`。但目前受影響資料流只列 `_rr_context()` 與
`_nearest_resistance_above_entry()`；現行 `_rr_context()` 沒拿 `entry_blocking_zone` 參數，
呼叫端也只傳 `target_zone=_nearest_resistance_above_entry(...)`。

實作前要補上 arbitration：

* target 原始來源是 `nearest_resistance_above_entry`、`entry_blocking_zone`，還是 T-056 定義出的
  structural／blocking zone；
* 當「最近可當目標的 tactical resistance」與「擋路 structural resistance」不同時，誰優先；
* `entry_blocking_zone.blocked=false` 但前方仍有 resistance 時是否封頂；
* 要把 `entry_blocking_zone` 傳進 `_rr_context()`，還是呼叫端先算好 capped target 再傳入。

**R3.（P2）T-055 缺前端驗收，可能修完後畫面仍顯示錯的 RR。**

計畫目標寫 `executable_now=false` 時 RR 顯示要跟著失效，但測試策略只有 Python 單元測試與
decision replay。現行前端 RR 區塊只顯示 `entry_rr` 與 `position_rr`，沒有主顯示
`execution_rr`。也就是說後端即使修出正確 executable RR，畫面仍可能繼續把 setup RR 當主 RR。

測試策略應補 `frontend/src/routes/SRZones.test.ts`：

* 不可執行時主顯示不得使用 setup／entry RR；
* `execution_rr` 有值時要顯示 executable RR 與 source／gate_basis；
* `execution_rr` unknown 時要明確顯示 unknown／target unavailable；
* 舊分析缺新欄位時要安全降級。

**R4.（P3）T-056 的 affected data flow 有一個不存在／不精準的落點。**

計畫寫 `decision_engine.py::_zone_summaries / _price_path`，但 Python 端目前沒有 `_zone_summaries`
函式。實際資料流是 `build_decision_summary` 產生 top-level `nearest_*` /
`primary_structural_zone`，Go 的 `buildDecisionZoneSummariesJSON()` 再把這些欄位投影進
`stock_sr_decisions.zone_summaries_json`。

請把 T-056 的受影響檔案與資料流改成精準落點，避免實作時找錯抽象層。

**R5.（P3）附帶發現已判定屬 issue，但尚未進 `docs/issue.md`。**

T-055 的「核實過程中的附帶發現」已寫明兩筆都是文件與實作不一致，依規則應進
`docs/issue.md`。若本次只是計畫書草案，短暫留在 T-055 內可以接受；若要提交，應同步建立 issue
或明確標成「待 review 後搬移」，避免 `docs/todo.md` 變成 issue 暫存區。

##### 再核實 findings（2026-08-21）

**F3.（P3）文件一致性修正（原記於 `issue.md` I-083，已於 2026-08-24 收斂移除）的補記本身
仍有小型殘字，容易讓歷史區塊讀起來不乾淨。**

`原始 Review findings` 的補記已經說「實作一律以 T-055／T-056 本文契約表與
裁決納入紀錄（第 3 版）為準」，但下一行又重複寫「不要直接引用本節或『建議裁決』作為
實作契約，應以 T-055/T-056 本文契約表與『裁決納入紀錄』為準」。這段有三個小問題：

* 同一個「以第 3 版為準」語意重複兩次；
* `不要直接引用` 被硬斷行，讀起來像殘留拼接；
* `建議裁決` 已改名為 `裁決前建議`，但這裡與下方「仍未定案」收斂句仍保留舊標題。

建議後續修正時把補記收成一句，統一稱呼 `裁決前建議`，並刪掉重複的
「以契約表與裁決納入紀錄為準」句子。

**處理結果（2026-08-21）：三項全部核實成立，已修正。**

成因是該次文件一致性修正的**字串替換只匹配到句子前半**，把原句尾巴留在原地，
接出「不要直接引用 ／ 本節或『建議裁決』作為實作契約，應以…為準」這段拼接殘句——
所以三個小問題其實是同一個成因的三種表徵。

| 子項 | 處置 |
|---|---|
| 語意重複兩次 | 補記末句收成一句：「**不要拿本節或「裁決前建議」當實作契約**」，刪掉重複的「應以…為準」 |
| 硬斷行拼接殘字 | 殘句整段移除，補記現為五行完整句 |
| 舊標題 `建議裁決` | todo.md 內四處全部改為 `裁決前建議`：補記、「仍未定案」收斂句、T-055／T-056 兩個狀態欄（後兩處加引號，明示是小節名而非泛稱） |

**命名慣例（2026-08-21 定，避免再漂移）**：本區段內文引用小節時，
**引號內只寫小節名本身**（`原始 Review findings` / `再核實 findings` / `Review 回應` /
`裁決前建議` / `裁決納入紀錄`），**不帶日期與版次**；需要標示版次時寫在引號外，
例如「裁決納入紀錄」（第 3 版）。小節標題本身統一為 `小節名（日期，版次）`。
這樣標題補日期或改版次時，內文引用不會跟著失效。

**這條命名慣例已歸檔**到 [`development-workflow.md`](./development-workflow.md)
「文件收斂規則」，本節保留作沿革。原本承載這件事的 `issue.md` I-083 已於 2026-08-24
review 通過後整筆收斂移除。


#### Review 回應（2026-08-21，第 2 版）

> **本節記錄的是第 2 版的處理狀態**：當時 R1／R2 只把問題列成「待定案表」，尚未定案。
> 那些待定案項目已於**第 3 版全數關閉**（見下方「裁決前建議」與「裁決納入紀錄」）。
> 表格內「四個待定案問題」一類的描述屬當時狀態，**不是現況**。

五項 findings **全部核實成立**，計畫已據此修改。逐項對照：

| # | 核實結果 | 計畫變更 |
|---|---|---|
| **R1** | **成立，且比 finding 更尖銳**。`2.0` 在三處同時出現：`_minimum_rr()` 的 Tier-1／Resistance 分支（`:1204-1207`）、ladder 的 `< 2.0`（`:501`）、`strong` 的 `rr >= 2.0`（`:524`）。Tier-1 zone 的 gate 與 full entry 剛好同值，折成一個看不出問題；**0050 的 primary 是 TIER_3 support，gate 1.5 而 full entry 2.0，必然分岔**——原方案會在這裡吃掉 Full Entry 語意 | 標題「門檻也必須只有一套」→「**門檻必須具名分層**」；修改目標第 1 條整條重寫為 `probe_min_rr` / `full_entry_min_rr` 兩層具名門檻＋四個待定案問題；不做範圍補上「`strong` 的 2.0 也不動」 |
| **R2** | **成立**。`_rr_context()` 簽章確實沒有 `entry_blocking_zone`，呼叫端只傳 `target_zone=_nearest_resistance_above_entry(...)`（`:2481-2489`）。**另查到一個 finding 未提、對計畫有利的事實**：`_entry_blocking_zone_detail` 在 `:2434` 就算完，`_rr_context` 在 `:2481` 才呼叫——**呼叫順序已經是對的**，不需重排流程 | 受影響資料流補上精確行號；新增「target 封頂的 arbitration」小節，把 review 的四個問題逐條列成待定案表；標注與 T-056 的順序相依 |
| **R3** | **成立**。`SRZones.svelte:2326-2340` 的 RR 區塊只有 `entry_rr` 與 `position_rr`，`execution_rr` 沒有顯示位置。**另查到**：`rr_gate` 區塊（`:2309-2324`）顯示 `qualified` / `minimum_rr` / `reason_code` 但**不顯示 `actual_rr`**——所以「1.87」與「通過」分屬兩個區塊、沒有任何標示說它們同源，這正是矛盾感被放大的原因 | 測試與驗證新增第 2 條前端測試（四個斷言），並把 `rr_gate` 缺 `actual_rr` 一併納入本筆處理範圍 |
| **R4** | **成立**。Python 端**沒有** `_zone_summaries` 函式；是 `build_decision_summary()`（`:2331`）在 `:2637-2655` 直接組 top-level 欄位，`zone_summaries_json` 這個集合概念只存在於 Go 的 `buildDecisionZoneSummariesJSON()`（`client.go:807-832`）。原計畫另把 `_price_path` 與 summary 併在一行也不精準——兩者是不同來源 | T-056 受影響資料流整段重寫為精準落點，並加註修正說明 |
| **R5** | **成立** | 第 2 版標為待搬移；**2026-08-21 已結案**——兩筆搬入 `docs/issue.md` 成為 **I-081** / **I-082**，T-055 內只留指標表 |

**~~仍未定案、需要你裁決的項目~~ → 已於下方「裁決前建議」全數關閉，並回寫進兩筆計畫本文（第 3 版）。**

1. ~~R1：門檻命名／`gate_kind`／前端顯示幾個 gate~~ → 定案見 T-055 修改目標第 1 條。
2. ~~R2：target 來源／優先順序／`blocked=false` 是否封頂／傳參方式~~ → 定案見 T-055「target 封頂的 arbitration」。
3. ~~契約命名 A/B/C 與「結構壓力」指哪一個 zone~~ → T-055 採 C、T-056 採 B+C，見兩筆的「契約變化」。

**相依關係更新**：原本「若 target 來源選 (c) 則 T-056 必須先定案」的**不確定性已解除**——
結構／擋路壓力確定採 `blocking_resistance_zone`。但**實作順序不變**：
`blocking_resistance_zone` 欄位要先在 T-056 落地，T-055 才能引用它當 target cap。

##### 裁決前建議（2026-08-21，**已由第 3 版裁決修正，保留作決策沿革**）

> 文件核實補記（2026-08-21）：本節保留裁決前建議，已被下方「裁決納入紀錄」（第 3 版）修正。
> 其中 `gate_kind=EXECUTION` 與新增 `actual_rr_source` 兩點已被第 3 版否決，逐處已加註。
> 實作時以 T-055 本文契約表與第 3 版裁決為準。

以下是對上方三組未定案問題的裁決前建議方向；第 3 版已採用其中方向並修正部分欄位契約，
本節保留作決策沿革，實作時不要直接採用未經第 3 版覆寫的欄位值。

1. **R1：保留兩個 gate，`rr_gate` 加 `gate_kind`；前端主顯示一個 authoritative gate，
   輔助顯示另一個 gate。**

   不建議把「唯一門檻」解讀成只有 `_minimum_rr()` 一個數字。0050 這類情境應表達成
   「Probe RR 通過，但 Full Entry 未通過」，而不是把其中一句消掉。建議 contract：

   * ~~`rr_gate.gate_kind`：`PROBE` / `FULL_ENTRY` / `EXECUTION`~~
     → **已被第 3 版否決**：`EXECUTION` 是 RR 來源不是門檻層，值域收斂為
     `PROBE` / `FULL_ENTRY`，執行性資訊改由 `gate_basis` 承載（見 F1 定案「偏離 1」）；
   * `rr_gate.minimum_rr`：目前實際仲裁用的門檻；
   * `rr_gate.actual_rr`：目前實際仲裁用的 RR；
   * `rr_gate.secondary_gate`：另一層 gate，例如
     `{ "gate_kind": "FULL_ENTRY", "minimum_rr": 2.0, "qualified": false }`。

   `probe_min_rr` / `full_entry_min_rr` 可作為內部命名或文件術語；對外回應重點是能看出
   「目前用哪個 gate 仲裁」與「另一層 gate 的狀態」。前端主卡只顯示 `rr_gate` 這個
   authoritative gate，旁邊用小字或次要列顯示 `secondary_gate`，避免同時放兩張等權重卡片造成
   新的矛盾感。

2. **R2：target 取所有前方壓力中的最低可用 `price_low`；`blocked=false` 時也封頂；
   呼叫端先算好 `execution_target` 再傳入 `_rr_context()`。**

   Executable RR 問的是「從 entry 到第一道可量化前方阻力還有多少空間」，所以 target 不得穿越
   任何前方壓力。建議 arbitration：

   * candidates 包含 `_nearest_resistance_above_entry()`、`entry_blocking_zone.blocking_zone`，
     以及 T-056 定義出的 `blocking_resistance_zone`；
   * 只納入 `price_low > entry_price` 的 candidate；
   * target = 最低 `price_low`；
   * `entry_blocking_zone.blocked=false` 仍要封頂，因為「沒有近到足以擋單」不等於
     「可以把 target 設在更後面的壓力」；
   * `_rr_context()` 不自己找 zone。呼叫端先算 `execution_target`，例如
     `{ "price": 105.1862, "basis": "FIRST_RESISTANCE_CAP", "source": "blocking_resistance_zone" }`，
     再交給 `_rr_context()` 算 RR。

   這會讓 RR 普遍下修，方向偏保守；必須用 decision replay 檢查 `by_rr_gate`、
   `by_rr_gate_reason_code`、`by_entry_executability` 與 final entry 分佈。

3. **T-055 契約命名採折衷 C：保留舊欄位作 alias，新增語意清楚的新欄位。**

   不建議一次移除 `entry_rr` / `execution_rr`，因為歷史資料、evaluation 與前端都已消費它們。
   建議：

   * 保留 `entry_rr`，文件改成 setup RR alias；
   * 新增 `setup_rr`；
   * 新增 `executable_rr`；
   * 保留 `execution_rr` 作 `executable_rr` alias，一版後再評估是否 deprecate；
   * ~~`rr_gate` 補 `actual_rr_source`~~ → **已被第 3 版否決**：與 `gate_basis` 一對一重複，
     不新增（見 F1 定案「偏離 2」）；`gate_kind` / `gate_basis` 照補。

4. **T-056 契約命名採 B + C：新增清楚欄位，同時保留 legacy alias。**

   建議新增：

   * `tactical_resistance_zone`：品質加權後最相關的戰術壓力；
   * `blocking_resistance_zone`：第一道前方擋路壓力，供顯示與 executable RR target cap 使用。

   `nearest_resistance_zone` 暫時保留為 `tactical_resistance_zone` 的 legacy alias，文件標明
   「不是價格最近」。~~`structural_resistance_zone` 不要直接等同 `primary_structural_zone`~~
   → **第 3 版收得更緊：`structural_resistance_zone` 直接不建立**（比本建議更嚴格，
   理由見 T-056 契約表）。原文如下：`structural_resistance_zone` 不要直接等同 `primary_structural_zone`，
   除非先確認語意；目前 `primary_structural_zone` 是 Tier-1 品質最高的結構參考，
   `entry_blocking_zone.blocking_zone` 是最近擋路壓力，兩者不保證相同。

   「結構壓力」在交易顯示與 RR 計算裡建議採 `blocking_resistance_zone`，不是
   `primary_structural_zone`。後者保留為大結構參考；executable RR 與進場擋路應看第一道
   前方壓力，也就是 blocking / nearest-by-price。


##### 裁決納入紀錄（2026-08-21，第 3 版）

四項裁決**全數採用並回寫進計畫本文**，本節只記錄「納入時另外查到、需要在實作前關掉」的兩個旗標。

**F1（P1）→ 已定案（2026-08-21）：兩個正交軸，`actual_rr_source` 不新增。**

`gate_basis` **已經存在**於 `_execution_rr_gate`（`decision_engine.py:1332-1362`），
現有值是 `ENTRY_STOP_TARGET` 與 `MARKET_ENTRY_TARGET_UNAVAILABLE`。定案如下：

| 欄位 | 回答的問題 | 值域 | 正交性 |
|---|---|---|---|
| `gate_kind` | **這一層 gate 在問什麼**（門檻層） | `PROBE` / `FULL_ENTRY` | 只跟門檻有關，與 RR 怎麼來的無關 |
| `gate_basis` | **`actual_rr` 是怎麼算出來的**（推導方式） | `ENTRY_STOP_TARGET` / `PRIMARY_ZONE_STATISTIC` / `TARGET_UNAVAILABLE` | 只跟 RR 來源有關，與門檻無關 |

**⚠ 本定案偏離裁決原文兩處，理由如下（依 CLAUDE.md 需明確回報差異）：**

**偏離 1：`gate_kind` 拿掉 `EXECUTION`，只留 `PROBE` / `FULL_ENTRY`。**
`EXECUTION` 描述的是「這個 RR 是用執行價位算的」，那是 **RR 的來源**，不是**門檻層**。
把它放進 `gate_kind` 會讓該欄位同時承載兩個軸——正是本計畫要消滅的「同名不同義」。
執行性資訊改由 `gate_basis=ENTRY_STOP_TARGET` 承載。

**偏離 2：不新增 `actual_rr_source`。**
它與 `gate_basis` 是一對一映射（`EXECUTABLE_RR ⟺ ENTRY_STOP_TARGET`、
`SETUP_RR ⟺ PRIMARY_ZONE_STATISTIC`、`無 ⟺ TARGET_UNAVAILABLE`），
同時輸出就是在修「同名不同義」的計畫裡再造一組「**異名同義**」。
保留既有的 `gate_basis`（已被 evaluation 的 `by_rr_gate_reason_code` 一類統計消費），
不新造欄位。

**值域相容處置**：`MARKET_ENTRY_TARGET_UNAVAILABLE` 泛化為 `TARGET_UNAVAILABLE`
——T-055 後所有路徑都會算 target，`MARKET_ENTRY_` 前綴會變成謊話。
舊值標為 legacy、**改動後不再產生**；前端與 evaluation 需在一個版本內把兩者視為同一桶，
report 以 `pipeline_version` 區隔，不得跨版本混計。

**明確不動的部分（避免有人順手「整理」而弄壞）**：

* `reason_code` 的既有值域（`RR_QUALIFIED` / `RR_INSUFFICIENT` / `EXECUTION_RR_INSUFFICIENT` /
  `EXECUTION_RR_UNAVAILABLE` / `RR_UNAVAILABLE` / `NO_PRIMARY_ZONE`）**一律不動**。
  `_final_entry_permission:848` 讀 `qualified`、`_final_entry_risk_notes:602` 逐字比對
  `"EXECUTION_RR_UNAVAILABLE"`——改了會靜默改變 final entry 行為。
  新資訊由 `gate_kind` / `gate_basis` 承載，不靠改 reason code。
* `target_known=false` 時 `actual_rr=null` 但 **`qualified=true`** 的語意不變。

**連帶後果（必須一併處理，否則會產生新的自相矛盾）**：
若 `full_entry_min_rr` 改成對 `executable_rr` 判定，則 `_decision_action` 的 `strong`
（`decision_engine.py:520-527`）也必須讀**同一個** `actual_rr`，否則
`action=Buy` 可能與 `secondary_gate.qualified=false` 並存——又是一組矛盾。
**`strong` 的門檻值 2.0 仍不動**（不做範圍），改的只是它讀哪個 RR。
`executable_rr ≤ setup_rr` 通常成立，所以方向偏保守，**必須用 decision replay 佐證**
`by_rr_gate` 與 final entry 分佈的變化幅度。

**F2（P2）`blocking_resistance_zone` 不保證是結構性的，UI 標籤不可寫「結構壓力」。**

裁決把「結構壓力」在交易顯示與 RR 計算裡定為 `blocking_resistance_zone`。但它的來源
`_entry_blocking_zone_detail`（`:1935-1955`）的過濾條件只有「role=RESISTANCE ＋ 非 EXPIRED ＋
`price_high >= current_price`」，**沒有任何 tier 條件**——所以第一道擋路壓力完全可能是 Tier-3 短期壓力。

0050 這筆它剛好是 Tier-1 主結構，屬巧合。若把 UI 標籤寫成「結構壓力」，
遇到 Tier-3 擋路時就會出現「標著結構壓力的短期壓力」——與本筆要修的錯配同一類。
**已在 T-056 契約表寫入標籤警告：UI 用「前方擋路壓力」，不用「結構壓力」。**

**F1 已定案（含兩處偏離裁決原文的說明），F2 的標籤用詞已寫進 T-056 契約表。**
兩者都不改變裁決的方向，只是把欄位分工與用詞收斂到不會再造成同名／異名混淆的程度。
T-055 / T-056 的契約至此**全部定案，無待決項**；剩下的是實作與 replay 佐證。

---

#### T-055 實作約束：`SUPPORT_RETEST_HELD` 與 `RESISTANCE_BREAKOUT` 維持只寫不讀

（原為 T-055／T-056 共同約束；T-056 已完成且**未**接入這兩個事件，約束對它已驗收通過。）

**本筆計畫不得順手把這兩個事件接進 Decision。**

核實現況（2026-08-21）：隔離是完整且顯式的。

| 層 | 機制 | 位置 |
|---|---|---|
| Python 事件桶 | `decision_visible=False`，`build_event_state_summary` 只收 visible | `event_engine.py:104-118, 499-516` |
| Python 決策 | `is_decision_visible` 過濾 | `decision_engine.py:1794, 2296` |
| Python 截斷 | `_truncate_events` 優先保留 visible，shadow 事件擠不掉正式事件 | `event_engine.py:314-330` |
| Go carry-forward | `eventDecisionVisible` 跳過 | `backend/internal/api/handler/sr_zones.go:400-403` |
| 持久化 | `event_instances.decision_visible`（migration 071） | — |

**注意型別名**：事件型別是 `SUPPORT_RETEST_HELD`（family 才是 `SUPPORT_RETEST`）與
`RESISTANCE_BREAKOUT`（family 與型別同名）。

**接 Decision 的前提，不是單日行情看起來吻合**，而是母體累積後的驗證通過
（見 [T-052](#t-052定期對-watchlist-產生-sr-zone-分析分析排程) 的累積、
[T-049](#t-049market-state-與所有下游改讀同一套-state) 的 state 收斂）。
`RESISTANCE_BREAKOUT` 的 `resolves` 刻意留空、方向為 BULLISH——一旦可見，
`active_bullish_events` 只看 direction 就會改變 lifecycle 判讀，**不需要任何人「認識」它就會改決策**
（見 `event_engine.py:61-72` 的說明）。這是它必須留在 shadow 的技術理由，與今天行情走勢無關。

---

### T-058：除權息端點無法在 test 以外覆寫，dev 驗不了公司行動同步

| 欄位 | 內容 |
|---|---|
| 狀態 | 待規劃 |
| 優先度 | 低 |
| 分類 | Go / 可測試性 / 驗收流程 |
| 建立日期 | 2026-08-24 |
| 來源 | 公司行動同步分片改造的 dev 驗收（2026-08-24） |

`YahooDividendClient` 的端點是檔案內的 const `yahooDividendURL`
（`internal/market/yahoo_dividend.go:21`），只有一個**未匯出**的 `baseURLForTest`
可以覆寫，給單元測試用。`yahoo.base_url` / `YAHOO_BASE_URL` 管的是**另一個端點**
（盤中報價的 `FinanceChartService.ApacLibraCharts`），不是這一個。

後果：**在 dev stack 上驗 `corporate_action_sync` 一定會打到真實 Yahoo。**
2026-08-24 驗公司行動同步分片時，FinMind 可以用 `FINMIND_BASE_URL` 指到本地 stub，
Yahoo 那半只能照打真實 API（實際寫進 dev 的 35 筆 `2330` 除權息就是真的）。
規模一大就變成「驗收動作本身在對外部服務施壓」。

可能作法（擇一，實作前要先確認）：

* 加 `yahoo.dividend_base_url`（預設空＝走現行 const），只在 dev/測試環境設。
* 或把 `baseURLForTest` 改成匯出的設定入口，由 `main.go` 依 config 注入。

**不做的理由也要一併評估**：多一個設定就多一個「線上被設錯就靜默打到別的地方」的面。
現況的 const 至少不可能被誤設。所以這筆的價值取決於「dev 驗收這條路徑」有多常做——
若公司行動同步之後不再需要反覆驗，可以直接關掉這筆。

**相關**：[`issue.md`](./issue.md) I-085（同一段設定的另一個文件缺口）。

---

### T-059：`job_runs` 只保留當天，排程健康史無法回溯

| 欄位 | 內容 |
|---|---|
| 狀態 | 待規劃 |
| 優先度 | 中 |
| 分類 | Go / Scheduler / 可觀測性 |
| 建立日期 | 2026-08-24 |
| 來源 | SR zone 分析排程驗收（live 唯讀查詢時實際踩到） |

`runPreMarket` 每天開盤前把前一天以前的執行紀錄整批刪掉（`scheduler.go:337-341`）：

```go
// 只保留當天的排程執行紀錄，開盤前先清掉前幾天的舊資料
if n, err := s.jobRuns.DeleteBefore(ctx, timeutil.TodayTaipei()); err != nil {
```

`DeleteBefore` 的註解也寫明「用於只保留當天資料」，所以這是刻意設計，不是 bug。
但它與「靠 `job_runs` 發現排程問題」這個既定作法直接衝突：

* **2026-08-24 的驗收就踩到了**：想查 2026-08-21（排程啟用後第一個完整交易日）那兩輪
  `sr_analysis` / `sr_analysis_chip` 的 `status` 與 `symbols_total`，紀錄已經沒了，
  只能改從 `stock_sr_zone_analyses` 的 `created_at` 分佈反推「兩輪都有跑、各 11 檔」。
  現況佐證：全表 62 筆但 `id` 已經到 1763。
* **I-084 正是靠 `job_runs` 的 `failed=808 / total=857` 發現的。** 那是當天查才看得到；
  隔一天再看，`corporate_action_sync` 那筆就被清掉了，證據不存在。T-057 修完之後
  「跑不完會持續顯示 `partial`」這個可觀察訊號，也只在**當天**成立。
* `/scheduler/status` 的 `stale` 判斷本身不受影響（它看的是最後一次執行時間），
  受影響的是**趨勢**：無法回答「這支這週失敗過幾次」「是哪一天開始變 partial 的」。

**可能作法（擇一，實作前要先確認）**：

* 改成保留 N 天（例如 14 或 30），`DeleteBefore(now - N days)`。改動最小，
  但要先估資料量——目前一個交易日約 60 筆（`intraday` 就占 55 筆），30 天約 1800 筆，
  對 PostgreSQL 完全不是問題。
* 或分級保留：`intraday` 這種高頻的只留當天，其餘每日排程留 N 天。實作較複雜，
  但避免高頻 job 洗掉低頻 job 的可見度。

**不做的理由也要一併評估**：當初「只留當天」大概是為了讓 `/scheduler/status` 的查詢
永遠很快、資料表不成長。若改成保留 N 天，要確認 `List(limit)` 那條查詢
（`ORDER BY started_at DESC LIMIT ?`）在資料變多後仍走得到索引。

**相關**：[`issue.md`](./issue.md) I-086 / I-087（同一次驗收發現的狀態誠實缺陷）——
那兩筆修好之後，失敗才會正確反映在 `status` 上，而本筆決定那個 `status` 留多久。

---

### T-060：`srZoneVerifyLimit` 固定 50，驗證覆蓋窗口會隨資料成長縮短

| 欄位 | 內容 |
|---|---|
| 狀態 | 待規劃 |
| 優先度 | 低 |
| 分類 | Go / Scheduler / SR Zone |
| 建立日期 | 2026-08-24 |
| 來源 | SR zone 分析排程驗收 |

`runSRZoneVerification` 每次只驗最近 `srZoneVerifyLimit = 50` 筆分析
（`scheduler.go:32-34`、`:557`），常數註解寫明是「避免隨著資料成長無限制掃描」，
所以這是刻意的取捨，不是缺陷。但那個數字是在**分析還沒排程化**的年代訂的：

| 時期 | 每交易日新增分析 | 50 筆涵蓋 |
|---|---|---|
| 排程啟用前（手動觸發） | 1～3 筆 | 約 20 個交易日 |
| **現在**（watchlist 11 檔 × 2 輪） | **22 筆** | **約 2.3 個交易日** |
| watchlist 若擴到 30 檔 | 60 筆 | **不到 1 個交易日** |

2026-08-24 那輪實測驗了 45 筆——正好是當時全表分析數（15:02 執行時 56 − 今天 17:00
那輪的 11 筆），還沒撞到 50 的上限。**下一個交易日起就會開始撞到。**

後果不是資料錯誤，而是**「zone 有沒有被突破」的驗證只回溯得到最近兩天**，
更早的分析裡那些 `PENDING` 的 zone 會永遠停在 `PENDING`。

**可能作法（擇一，實作前要先確認）**：

* 改成「驗最近 N 個交易日的分析」而不是「最近 N 筆」，讓覆蓋窗口與 watchlist
  大小脫鉤。
* 或把 50 改成可設定，並讓預設值從 watchlist 檔數推導（例如 `檔數 × 2 輪 × N 天`）。
* 或維持固定上限，但改成只驗 `status='PENDING'` 的 zone——已經驗過並定案的不必重驗，
  同樣的預算能回溯更久。**這條要先確認 `Verify` 是不是冪等、以及已定案的 zone
  會不會因為後續行情再次變化而需要重驗。**

**不做的理由**：這台 host 只有 2GiB，驗證是逐筆跑的，放大窗口等於拉長排程佔用時間。
真的要放大，要先量一筆驗證的實際成本。

