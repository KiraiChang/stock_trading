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
  - Daily confirmation：納入 T-028 的隔日 / 兩日確認成效統計。
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

但 **預設 `replay_max_rows: 200` 搭配 watchlist（50～200 檔）時，股票覆蓋率必然遠低於
`MIN_REPLAY_SYMBOL_COVERAGE = 0.9`，每次排程都會產出 `DEGRADED`**，production 進場上限被壓到
`SMALL_ENTRY`。所以 P2 要正式啟用，得先決定 `replay_max_rows` 與 `sr_evaluation.symbols` 的
搭配（例如縮小 symbols 到重點觀察名單，或把 `replay_max_rows` 調到
`檔數 × 足夠樣本`），不只是「等 report schema review」。

**P1/P2 剩餘工作**：

1. 決定 P2 正式啟用的 `replay_max_rows` / `symbols` 搭配並寫進設定說明。
2. RR 分布由保守版擴充為 bucket / distribution（原計畫的 Decision 層指標）。

（P0 遺留的 calibration bins 已於 2026-08-04 補齊、2026-08-05 review 通過，現況規格見
[`sr-zone-scoring.md`](./sr-zone-scoring.md) 的「Calibration bins」。）

#### T-002 / T-003 / T-028 review 結論（2026-07-27 初審、2026-08-04 複審）

兩輪 review 都確認**方向正確、關鍵風險處理妥當**：無 lookahead bias、production governance
gate 只趨保守且安全降級、T-003 預設未變（`adaptive_zone_builders_enabled: false`）、排程預設
關閉（`sr_evaluation.enabled: false`）。這些性質的現況說明已歸檔到
[`sr-zone-scoring.md`](./sr-zone-scoring.md)（as-of 邊界、取樣規則、context 比對規則、
governance gate、evaluation 排程），並由 `tests/test_pipeline.py` 與 `scheduler_test.go` 鎖住。

2026-08-04 複審另外找出 9 筆問題（含一筆會讓 replay 只驗到第一檔股票的高嚴重度取樣缺陷、
一筆 MySQL 保留字），皆已修復並歸檔；review 通過後已從 `issue.md` 收斂，僅留 I-040
（刻意保留的已知限制）。2026-08-05 review 通過後，I-049（context row 缺 `trade_date` 會拋
`KeyError`）也已收斂——現況行為記在 [`sr-zone-scoring.md`](./sr-zone-scoring.md) 的
「Replay context 的股票比對規則」。

F1（scheduler 測試）與 F2（前端元件互動測試）已完成並通過 review，條目已收斂。剩餘：

- **F3（低，optional，未處理）**：`evaluation.py` 單檔已超過 1900 行（replay / daily-confirmation /
  sweep / governance report 全擠一起），後續可拆模組（replay / outcomes / sweep / reporting）
  以利維護。屬大型重構，需另出計畫書。

狀態不動（T-002/T-003 P1/P2 仍「部分已實作」、T-028「已實作起步」）；剩餘 P1/P2 續作與 F3
完成後再整體收斂歸檔。

---

### T-003：ATR zone 寬度乘數依個股特性調校

| 欄位 | 內容 |
|---|---|
| 狀態 | 規劃中 |
| 優先度 | 中 |
| 分類 | Python / SR Zone / Zone Builder |
| 建立日期 | 2026-07-07 |
| 來源 | `docs/sr-zone-scoring.md` 已知限制 |
| P0 狀態 | 已實作（2026-08-05 review 通過） |
| P1 狀態 | 已實作（2026-08-05 review 通過；計畫列的五個比較面向全數覆蓋） |
| P2 狀態 | 部分已實作（flag + runtime metadata 已有、預設關閉；決策依據已具備，待實跑 sweep 取樣） |

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

**P1/P2 剩餘工作**：

- **仍待辦**：實際跑一次 sweep 取樣，依結果決定 bucket 門檻、是否預設啟用 adaptive
  builder，以及是否需要 symbol-level override。這是 P2 收斂的最後一步。

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

### T-028：SR Zone Daily Confirmation 回測與評估

| 欄位 | 內容 |
|---|---|
| 狀態 | 待規劃 |
| 優先度 | 中 |
| 分類 | Python / SR Zone / 模型驗證 |
| 建立日期 | 2026-07-15 |
| 來源 | SR Zone Decision Engine P2 後續限制；T-002 的子任務 |

`decision_summary.daily_confirmation` 是單筆 EOD runtime 判讀，不能代表規則已完成歷史驗證。
後續需在 SR Zone evaluation/backtest pipeline（T-002）中加入 daily confirmation label 與成效統計：

- 候選支撐隔日守住率。（已實作起步，方向已 review；續作項目見下方剩餘工作）
- 候選壓力隔日壓回率或突破延續率。（已實作起步，方向已 review；續作項目見下方剩餘工作）
- 兩日確認後的勝率、風險報酬分布與失效率。（已實作起步：兩日方向 / 報酬統計；完整 RR distribution 待補）
- 不同量能條件、event sequence、RR gate 下的分層表現。（已實作起步，方向已 review；續作項目見下方剩餘工作）

已實作範圍：

- `run_decision_replay()` 的每列 replay row 新增 `daily_confirmation_outcome`、
  `next_close_return`、`two_bar_close_return`。
- `outcome_summary.daily_confirmation_summary` 會輸出支撐 / 壓力隔日 zone 結果與兩日結果，
  並提供 `by_state`、`by_primary_role` 分層。
- `outcome_summary.daily_confirmation_summary` 已補上量能條件、event sequence、market event、
  market state、RR gate、RR reason code、RR bucket 分層。
- `outcome_summary.daily_confirmation_summary.failure_distribution` 已提供第一版失敗分布。
- 尚未完成：更完整的 RR distribution（例如 percentiles / drawdown-like failure window）、
  以及依量能強弱數值、event sequence 順序細節與 RR gate 原始基礎值做更細分層。

> 2026-07-27 review：daily confirmation outcome 標記與分層統計以未來 idx+1 / idx+2 label 計算、
> 無 lookahead，方向確認無誤，見 T-002 的「review 結論」段落。

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

### T-037：T-002 / T-003 的 evaluation 產出接上前端畫面

| 欄位 | 內容 |
|---|---|
| 狀態 | 進行中（A、B 已實作待 review；C 未開始） |
| 優先度 | 中 |
| 分類 | Frontend / Go / SR Zone / 可觀測性 |
| 建立日期 | 2026-08-05 |
| 來源 | T-002 / T-003 前端畫面補齊狀況檢查（2026-08-05） |

2026-08-05 對照程式碼檢查的結果：**T-002 的操作入口齊全，但 P0 計畫列的三層核心指標在 UI 上
一個都沒顯示；T-003 則是前端完全沒有任何呈現**（`grep` 整個 `frontend/src` 對
`volatility_profile` / `zone_builder_runtime_config` / `atr_width_multiplier` 零命中）。

**最實際的後果**：Zone Evaluation 模式跑完等於白跑。該模式的 report 依設計沒有
`governance_evaluation`（治理區塊不出現），而 `model_metrics` / `zone_outcomes` 又沒渲染，
畫面上只剩 `run_id` 與 `rows` / `sources`。要看 AUC 就只能勾「寫入結果」去查 regression
results 表格——正是 2026-08-04 那批修正想解掉的兩難，當時只解了 decision replay 那一半。

**A：report 面板補上核心指標**（前端為主，無 API 變更；欄位早就在 report 裡）

| 層級 | report 欄位 | 現況 |
|---|---|---|
| 模型層 | `model_metrics.{hold,break}`：AUC / Brier / log loss / calibration bins + ECE | 未顯示 |
| Zone 層 | `zone_outcomes`：hold rate / rejection rate / breakout continuation | 未顯示 |
| Decision 層 | `outcome_summary.by_final_entry_state` / `by_market_bias` / `rr_summary` / `at_zone_rate` | 未顯示 |
| Daily confirmation | `outcome_summary.daily_confirmation_summary`（T-028） | 未顯示 |
| 警告 | `warnings` | 只有 regression results 表格有，live report 面板沒有 |

面板一次塞太多會難讀，需先決定分區與預設收合策略；calibration bins 這種 10 列的資料要考慮
是否只顯示 ECE / max error 摘要，細節走 raw JSON。

**B：T-003 的 builder 參數與 runtime config**

- evaluation 表單補上四個 ATR 參數輸入（`atr_width_multiplier` / `max_merge_width_multiple` /
  `atr_lookback` / `atr_period`）。**Go API 與 Python 都已支援**，evaluation 與 decision replay
  兩種模式都會生效（見 [`sr-zone-scoring.md`](./sr-zone-scoring.md) 的「Decision Replay 的
  zone builder 參數」），純粹是前端沒開欄位。
- 顯示 `volatility_profiles` 與 `zone_outcomes.by_volatility_bucket`。
- 顯示分析用的 `zone_builder_runtime_config`（adaptive 是否啟用、用了哪個 bucket、原因碼）。
  **這條要先補 Go**：`analysis/client.go` 目前沒有對應欄位，Python payload 的這塊在 Go 端就被
  丟掉了，前端拿不到。屬 T-003 P2 的可觀測性缺口。

**C：`http_server` 端點的可測性**

`/sr-scoring/evaluate` 目前沒有任何測試，且 `http_server.py` 在 import 時就呼叫
`check_connection()`，測試環境沒有 DB 會直接失敗，無法用 FastAPI TestClient 補。
2026-08-05 就實際發生過一次「參數收下卻沒生效」的 wiring bug（replay 分支漏傳
`builder_config`），這類問題目前只能靠結構避免、無法靠測試鎖住。需要的是把連線檢查移出
import 期（或提供可注入的 DB stub），屬小型重構。

**不在本項範圍**：參數 sweep 沒有 API 與 UI（Go 無路由、Python 無端點），只能走 CLI——這是
已記載的現況（見 [`sr-zone-scoring.md`](./sr-zone-scoring.md) 的「參數 sweep 的 decision 層
比較」），要不要做成背景 job 另案評估，不在這裡順手加。

**相依**：B 的第三點需先改 Go；A 與 B 前兩點是純前端。實作前需依規模決定是否另出計畫書
（跨 Go + 前端即屬跨模組異動）。

#### 實作計畫：T-037 A —— report 面板補上三層核心指標（2026-08-05，已實作待 review）

| 欄位 | 內容 |
|---|---|
| 狀態 | 已實作（待 review） |
| 範圍 | frontend only（SR Zone 頁 evaluation 面板）；本次只做 A，B / C 不動 |
| 影響 runtime | 只有 frontend；不動 Go、Python、API contract 與 DB |

**實作結果（2026-08-05）**：四個區塊與 warnings 全部完成，`svelte-check` 0 errors、
14 files / 42 tests（新增 5 筆）、build 綠。現況說明已歸檔到
[`sr-zone-scoring.md`](./sr-zone-scoring.md) 的「前端手動 evaluation 入口的判讀」（含區塊與
schema 的對應表、`—` 不等於 0、ECE 樣本不足兩個判讀陷阱）。

實作中修掉一個計畫沒預見的問題：**Svelte 模板不能寫明確泛型參數**。
`sortedEntries<SRZoneOutcomeGroup>(...)` 會被模板當成 `<` / `>` 比較運算子，編譯過得去但
runtime 拋 `ReferenceError: SRZoneOutcomeGroup is not defined`，且只在有分層資料的 report
才觸發——靠型別檢查與 build 都抓不到，是新增的元件測試先攔下來的。改用型別推導即可。

**先講一個會決定整個設計的前提：兩種 report 的欄位幾乎是互斥的**

查證 `evaluation.py` 的兩處 report 組裝後確認：

| 欄位 | Zone Evaluation<br>`sr_zone_evaluation_p0` | Decision Replay<br>`sr_zone_decision_replay_p0` |
|---|---|---|
| `model_metrics`（AUC / Brier / log loss / calibration） | ✅ | ❌ |
| `zone_outcomes`（hold rate / rejection rate / …） | ✅ | ❌ |
| `dataset_summary` | ✅ | ❌ |
| `outcome_summary`（decision 分層 / RR / daily confirmation） | ❌ | ✅ |
| `governance_evaluation` / `replay_coverage` / `replay_rows` | ❌ | ✅ |
| `volatility_profiles` / `builder_config` / `warnings` | ✅ | ✅ |

這正好解釋了「Zone Evaluation 跑完畫面一片空白」：現有面板只渲染治理區塊，而那是 replay
專屬欄位。所以**所有區塊一律 by-presence 渲染（`{#if report.xxx}`），不要用模式旗標硬寫**
——用模式判斷會在 schema 演進時再度失準，而 by-presence 天然容錯。

**目標**

1. Zone Evaluation 模式跑完就看得到模型層與 Zone 層指標，不必勾「寫入結果」去查表格。
2. Decision Replay 模式補上 decision 分層、RR 摘要與 daily confirmation 成效。
3. 兩種模式都顯示 `warnings`。

**不做的範圍**

- 不做 B（ATR 參數輸入、`volatility_profiles` 明細、`zone_builder_runtime_config`）與 C（端點可測性）。
- 不顯示 `replay_rows` 原始列、不加「完整 report JSON」區塊。
- 不改後端與 Python：這些欄位早就在 report 裡，純粹是前端沒渲染。

**受影響檔案**

| 檔案 | 動作 |
|---|---|
| `frontend/src/lib/api/srZones.ts` | 新增具名 optional 型別掛進 `SREvaluationReport`（比照既有 `SRDecisionReplayGovernance` 的做法，不靠 index signature） |
| `frontend/src/routes/SRZones.svelte` | evaluation report 區下新增四個 `<details>` 分區 + warnings 列 |
| `frontend/src/routes/SRZones.test.ts` | 新增兩種 schema 的渲染測試與邊界測試 |

**型別**（欄位名以 `evaluation.py` 實際輸出為準）

- `SRBinaryMetrics`：`rows` / `positive_rows` / `auc` / `brier_score` / `log_loss` / `calibration`
- `SRCalibration`：`bin_count` / `rows` / `binned_rows` / `bins[]` / `expected_calibration_error` /
  `max_calibration_error` / `insufficient_sample`；bin 為 `lower` / `upper` / `rows` /
  `mean_predicted` / `observed_rate` / `gap`
- `SRModelMetrics`：`model_available` / `model_version` / `model_trained_at` /
  `model_config_hash` / `hold` / `break`（**`hold` 與 `break` 在無模型時是 `null`**）
- `SRZoneOutcomes`：`rows` / `support_hold_rate` / `resistance_rejection_rate` /
  `break_positive_rate` / `average_forward_return` / `by_method` / `by_role` / `by_volatility_bucket`
- `SRDecisionOutcomeGroup`：`rows` / `rows_with_forward_return` / `average_forward_return` /
  `positive_forward_return_rate` / `negative_forward_return_rate`
- `SRRRSummary`：`rows_with_entry_rr` / `average_entry_rr` / `median_entry_rr` /
  `rows_with_position_rr` / `average_position_rr` / `median_position_rr` /
  `entry_rr_source_counts` / `position_rr_source_counts`
- `SRDailyConfirmationSummary`：`rows` / 五個 rate（`support_next_hold_rate`、
  `support_two_bar_confirm_rate`、`resistance_next_rejection_rate`、
  `resistance_next_breakout_rate`、`resistance_two_bar_breakout_continuation_rate`）/
  `average_next_close_return` / `average_two_bar_close_return` / `failure_distribution` / `by_state`
- `SROutcomeSummary`：`at_zone_rate` / `primary_zone_role_counts` / `rr_summary` /
  `daily_confirmation_summary` / `by_final_entry_state` / `by_daily_confirmation_state` /
  `by_market_bias`（其餘 `rows_with_*` 計數本次不渲染，靠既有索引簽章吸收）

**UI（四個分區，各用 `<details>`，沿用檔案既有樣式）**

| 分區 | 摘要列（預設可見） | 展開內容 |
|---|---|---|
| 模型層 | hold / break 的 AUC、Brier、log loss、ECE | calibration 10 個 bin 的表格（區間、rows、mean_predicted、observed_rate、gap） |
| Zone 層 | support hold rate、resistance rejection rate、break positive rate、平均 forward return | `by_role` / `by_method` / `by_volatility_bucket` 三張小表 |
| Decision 層 | `at_zone_rate`、平均 entry RR / position RR | `by_final_entry_state` / `by_market_bias` / `by_daily_confirmation_state` 表格（rows、平均報酬、正負報酬率） |
| Daily confirmation | 五個 rate + 隔日／兩日平均報酬 | `failure_distribution` 與 `by_state` |

`warnings` 不折疊，直接列在分區之上。

**顏色**（依 [`development-workflow.md`](./development-workflow.md) 的三類規則，避免重蹈 `fall` 是綠色的坑）

- `warnings` 屬錯誤訊息文字 → `text-rise`（紅）。
- 報酬率屬行情語意 → 沿用既有 `fmtSignedPct()` 與 `signedClass()`。
- **不得**用 `text-fall` 標示任何「不好的」指標，那是綠色。

**主要風險**

- **null 與空值**：無模型時 `model_metrics.hold` / `break` 是 `null`（不是缺鍵），`calibration`
  在 `rows=0` 時也是 `null`，空 bin 的 `mean_predicted` / `observed_rate` / `gap` 全是 null。
  必須顯示 `—` 而非 `0`——把 null 印成 0 會讓「沒資料」看起來像「完美校準」。
- **ECE 誤用**：`insufficient_sample=true`（樣本 < 50）時 bin 內 observed_rate 抖動極大，UI 必須
  明確標示「樣本不足，不可用於調參」，否則使用者會拿雜訊做參數決策。這是主題文件明講的陷阱。
- **面板長度**：四區全展開會很長，故預設全部收合、摘要只放關鍵數字。
- **回滾**：純前端、無契約變更，`git checkout` 即可還原。既有測試不會紅（純新增）。

**測試（`SRZones.test.ts`）**

- Zone Evaluation report（有 `model_metrics` / `zone_outcomes`、無 `governance_evaluation`）→
  模型層與 Zone 層區塊出現、治理區塊不出現。**這條直接鎖住本次要解的痛點。**
- Decision Replay report → decision 層與 daily confirmation 區塊出現、模型層區塊不出現。
- `model_available: false`（`hold` / `break` 為 `null`）→ 不拋錯，數值顯示 `—`。
- `calibration.insufficient_sample: true` → 顯示樣本不足標示。
- `warnings` 非空 → 內容顯示且帶 `text-rise` **class 斷言**（只斷言文字抓不到配色錯誤）。

**驗證**：`frontend/scripts/test.sh`（svelte-check → vitest → build 三步全綠）。後端與 Python
無改動，不需重跑。

**完成後歸檔**：各區塊對應哪個 schema、為何 by-presence 渲染、ECE 樣本不足的判讀方式，補到
[`sr-zone-scoring.md`](./sr-zone-scoring.md) 的「前端手動 evaluation 入口的判讀」。

#### 實作計畫：T-037 B —— builder 參數輸入與 runtime config 可觀測性（2026-08-05，待確認）

| 欄位 | 內容 |
|---|---|
| 狀態 | 已實作（待 review） |
| 範圍 | B①②（純前端）＋ B③（Python→Go→DB→API→前端全鏈路）；C 不在本次 |
| 影響 runtime | 前端、Go backend、DB schema；Python 不改 |

**實作結果（2026-08-05）**：兩階段都完成。

- 階段 1（B①②）：`frontend/scripts/test.sh` 三步全綠（svelte-check 0 errors、
  14 files / 49 tests、build 通過）。
- 階段 2（B③）：`backend/scripts/test.sh ./internal/store/... ./internal/analysis/...
  ./internal/api/handler/...` 全綠；前端再跑一次為 14 files / **51 tests**、build 通過。
- 現況說明已歸檔到 [`sr-zone-scoring.md`](./sr-zone-scoring.md)：新增
  「Adaptive builder 選用與 `zone_builder_runtime_config`」（reason_code 五種值、
  舊資料 JSON null ≠ adaptive 未啟用、為何用 NOT NULL），並在
  「Decision Replay 的 zone builder 參數」補前端入口的「留白＝不送鍵」語意、
  「前端手動 evaluation 入口的判讀」補波動側寫區塊與單位陷阱。

**實作中修掉一個計畫沒預見的問題**：Svelte 模板**不能寫 TS 型別註記**。原本想用
`{#each [...] as field}` 把四個參數輸入收成一個迴圈，裡面的
`set: (v: number | null) => ...` 讓 svelte-check 直接 `Unexpected token`，且錯誤會外溢成
「SRZones.svelte has no default export」把 App.svelte 與測試一起弄紅。改成四個明確的
`<label>` + `bind:value` 即可——這與 A 那次的「模板不能寫明確泛型參數」是同一類坑。

**自我 review 後修掉的兩處（2026-08-05）**：

1. **前端接受 `0` 但 Go 會靜默丟掉**：`optionalBuilderParams` 原本刻意保留 0（理由是
   「0 是合法設定值」），但 `SREvaluationRequest` 這四個欄位是 `omitempty`，
   `json.Marshal` 轉發給 Python 前就把 0 丟掉了——使用者填 0 會得到「參數收下卻無效果」的
   靜默失效，正是 T-037 C 要解的那類 bug。已改成**只送正數**，`min` 由 `0` 收緊到 `0.1`，
   測試改為 `drops non-positive and NaN`（同時斷言正數要照送），並修正
   `sr-zone-scoring.md` 中「0 是合法的參數值」的錯誤敘述。要支援 0 得先拿掉 Go 的
   `omitempty`，不是前端硬送。
2. **`client.go` 不合 gofmt**：新欄位插在 `ChipSummary` 與 `Model` 之間，中間的註解會中斷
   gofmt 的對齊區塊，讓 `Model` 的舊 padding 變成不合格式。本機沒有 go toolchain、
   `go vet` 也不檢查格式，現有流程擋不下來。已把新欄位移到 struct 最後。

**尚未處理（提交前要注意）**：`backend/internal/ui/dist/` 的建置產物每次 build 都會換檔名——
`index.html` 已 track 且指向新檔名，但新的 `assets/index-*.js` / `.css` 是 untracked。
用 `git commit -am` 會只提交 index.html 而漏掉資產，Go embed 的 dist 就缺檔。
**提交時要 `git add backend/internal/ui/dist/` 整個目錄。**

**驗證涵蓋範圍**：

| 項目 | 驗證方式 | 結果 |
|---|---|---|
| sqlite migration | `internal/store` 測試實跑 goose migration 後做 round-trip | ✅ |
| postgres migration | `scripts/smoke-dev.sh` 起 dev stack，backend 啟動時由 goose 實際套用 | ✅ `goose_db_version` = 57，欄位為 `jsonb NOT NULL DEFAULT 'null'::jsonb` |
| mysql migration | **未實跑**——本機沒有 MySQL 實例，dev compose 用的是 postgres | ⚠ 僅比照 037 既有寫法（ADD NULL → backfill → MODIFY NOT NULL） |
| dev stack 端到端 | `scripts/smoke-dev.sh`（先停 → build → 起 → health check） | ✅ backend / python-server 皆 healthy，`dev stack smoke passed` |
| API 回應含新欄位 | `TestSRZoneGetExposesZoneBuilderRuntimeConfig` | ✅ |

`smoke-dev.sh` 是在 live project 的 9 個 container 仍常駐、`MemAvailable` 約 930MB 的情況下
跑的，Go 編譯階段 available 一度掉到 275MB 但沒有觸發 OOM killer；後續 handler 測試被
mem-guard 由 700m 自動下修到 540m 仍通過。這次沒重演 I-053，但邊際很窄。

**先回報一項與 T-037 原描述的落差**

原文把 B③ 寫成「這條要先補 Go：`analysis/client.go` 目前沒有對應欄位」，實際查證後**範圍比
這句話大**：SR 分析的 API 回應是**繞經 DB 再讀回來**的（`sr_zones.go:578` `ToStore()` →
`repo.Create()` → `loadSRZonePipelineSnapshot()` → `srZonePipelineResponse()`；即使是走
`provider.Analyze` 的分支，第 558 行一樣是從 DB 讀 snapshot）。所以只在 `client.go` 補欄位，
資料仍會在 `ToStore()` 這關被丟掉，前端拿不到。**要讓前端看得到就必須落地成 DB 欄位**，
連帶三個 DB engine 的 migration。這已是資料庫 contract 變更，故單獨出此計畫書。

反過來也有好消息，B① 與 B② 都比原描述更輕：

| 子項 | 原描述 | 實際查證 |
|---|---|---|
| B① 四個 ATR 參數輸入 | 「Go API 與 Python 都已支援，純粹前端沒開欄位」 | ✅ 正確。`sr_regression_results.go:63` 直接 bind 進 `analysis.SREvaluationRequest`，該 struct 的四個欄位（`client.go:1167-1170`）json tag 齊全 → **純前端** |
| B② `volatility_profiles` 顯示 | 未言明 | **純前端**。`RunSREvaluation` 回傳 `map[string]any`（原樣穿透），report 欄位早就到得了前端。且 `zone_outcomes.by_volatility_bucket` 已由 A 做掉（`SRZones.svelte:1510`），本項只剩 `volatility_profiles` 本體 |
| B③ `zone_builder_runtime_config` | 「先補 Go client.go」 | **全鏈路**：Go struct ＋ **DB migration ×3** ＋ store model ＋ repo 兩處欄位清單 ＋ handler 回應 ＋ 前端 |

**目標**

1. evaluation 表單可以直接調四個 ATR builder 參數，不必改 code 或走 CLI。
2. evaluation report 看得到 `volatility_profiles`（各 symbol 落在哪個波動 bucket、touch count），
   讓 A 已做好的 `by_volatility_bucket` 分層數字有母體可對照。
3. SR 分析畫面看得到 `zone_builder_runtime_config`：adaptive 是否啟用、用了哪個 bucket 的
   config、以及 `reason_code`（`EXPLICIT_BUILDERS` / `ADAPTIVE_ZONE_BUILDERS_DISABLED` /
   `ADAPTIVE_ZONE_BUILDERS_ERROR`）——目前這段在 Go 端被靜默丟棄，是 T-003 P2 的可觀測性缺口。

**不做的範圍**

- 不做 C（`http_server.py` import 期 `check_connection()` 重構與 `/sr-scoring/evaluate` 測試）。
- 不做參數 sweep 的 API／UI（已記載為現況限制，只能走 CLI）。
- 不把四個 ATR 參數存進 evaluation job 記錄（不加 job 表欄位）：實際生效值已由 report 的
  `builder_config` 回聲，夠用且不需要再一次 migration。
- 不改 Python：`pipeline.py:99/395` 早就把 runtime config 放進 `analysis` payload 了。

**分兩階段實作（可分別驗收）**

**階段 1：B①＋B②（純前端，無 contract 變更）**

| 檔案 | 動作 |
|---|---|
| `frontend/src/lib/api/srZones.ts` | `SREvaluationOptions` 加四個 optional 參數；`runSREvaluation` body 帶上 `atr_width_multiplier` / `max_merge_width_multiple` / `atr_lookback` / `atr_period`（**未填就不送**，讓 Go 的 `omitempty` 與 Python 預設值接手）；新增 `SRVolatilityProfile` 型別掛進 `SREvaluationReport`（比照 A 的具名 optional 型別做法） |
| `frontend/src/routes/SRZones.svelte` | 表單新增四個 optional 數字輸入（放在既有 `evaluationLimit`／`evaluationReplayMaxRows` 附近，約 1310-1330 行區塊），預設空白＝沿用後端預設；report 區新增 `volatility_profiles` 分區（`<details>`，by-presence 渲染） |
| `frontend/src/routes/SRZones.test.ts` | 「四個參數留白時 body 不含該鍵」「填了才送出且型別為數字」「`volatility_profiles` 有資料時渲染、缺鍵時不渲染」 |

**階段 2：B③（全鏈路）**

| 檔案 | 動作 |
|---|---|
| `backend/internal/database/migrations/{mysql,postgres,sqlite}/057_add_sr_zone_builder_runtime_config.sql` | `stock_sr_zone_analyses` 加 `zone_builder_runtime_config` JSON/TEXT 欄位，`ADD COLUMN IF NOT EXISTS` + `-- +goose Up/Down`，比照 056 的寫法；三個 engine 的型別依既有 RawJSON 欄位（如 `period_summaries`）在該 engine 的宣告方式對齊 |
| `backend/internal/analysis/client.go` | `zonePipelineAnalysis`（303 行）加 `ZoneBuilderRuntimeConfig json.RawMessage \`json:"zone_builder_runtime_config"\``；`ToStore()` 帶進 store 型別 |
| `backend/internal/store/model.go` | `SRZoneAnalysis` 加 `ZoneBuilderRuntimeConfig RawJSON`（比照 `PeriodSummaries` 的 db/json tag 寫法） |
| `backend/internal/store/sr_zone_repo.go` | **兩處欄位清單都要改**（第 74 行、第 337 行），以及對應的 INSERT 參數 |
| `backend/internal/api/handler/sr_zones.go` | `srZonePipelineResponse` 的 `analysis` 區塊（186-192 行）加上該鍵 |
| `frontend/src/lib/api/srZones.ts` / `SRZones.svelte` | 型別 ＋ 分析頁顯示（adaptive 啟用與否、`reason_code`、bucket、`config` 快照） |

**資料 contract 變化**

- 新增 DB 欄位（nullable，無預設值語意）；**舊資料為 NULL**，前端必須 by-presence 渲染，
  NULL 時整個分區不出現——不可顯示成「adaptive 未啟用」，那是把「沒有紀錄」誤讀成「有紀錄且為關閉」。
- API 回應新增一個 optional 鍵，屬**相容新增**，既有前端不受影響。
- 仲裁順序不變：這個欄位純粹是**紀錄**（Python 端已經決定好了才回傳），不參與任何決策或狀態推導。

**主要風險與回滾**

- **三個 engine 的 migration 漏改**：目前 mysql 55 / postgres 56 / sqlite 55 個檔，數量本就不一致，
  容易只補一個。→ 三個都補，並在 dev project（postgres）實跑 migration 驗證。
- **repo 欄位清單有兩處**：只改一處會出現「寫得進去、讀不出來」或 scan 欄位數不符的 runtime error。
  → 兩處都改，並靠既有 repo 測試覆蓋。
- **NULL 判讀**：如上，舊分析沒有這個值，UI 必須顯示為「無紀錄」而非「未啟用」。
- **回滾**：migration 有 `-- +goose Down`；Go／前端變更皆為相容新增，`git revert` 即可。
  已寫入的欄位留著不影響舊版程式（多餘欄位不會被 scan）。

**測試與驗證策略**

- 階段 1：`frontend/scripts/test.sh`（svelte-check → vitest → build 三步全綠）。
- 階段 2：
  - `backend/scripts/test.sh ./internal/store/... ./internal/analysis/... ./internal/api/handler/...`
  - migration 在 **dev project**（`docker-compose.dev.yml`）實跑，確認 up / down 都過（CLAUDE.md：
    不得用 live/deploy compose project 做 migration 驗證）。
  - `scripts/smoke-dev.sh` 跑一次分析，確認新欄位在 API 回應中出現且值正確。
  - 前端測試補「NULL/缺鍵時不渲染」與「有值時顯示 reason_code」。
- 記憶體：依 `development-workflow.md`，開跑前先確認沒有其他 stack 常駐（見 issue.md I-053）。

**完成後歸檔**

- `zone_builder_runtime_config` 的欄位語意、`reason_code` 三種值的意義、NULL 代表舊資料，補到
  [`sr-zone-scoring.md`](./sr-zone-scoring.md) 的 zone builder 章節。
- 四個 ATR 參數在 UI 的位置與「留白＝沿用預設」的行為，補到同文件的「Decision Replay 的
  zone builder 參數」。
