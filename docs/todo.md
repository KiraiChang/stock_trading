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
| 狀態 | 待規劃（**前置已解除**：T-044 已於 2026-08-18 收斂，Lifecycle 狀態定義已定案） |
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

### T-044：抽出獨立的 Lifecycle Engine

| 欄位 | 內容 |
|---|---|
| 狀態 | **P0 已實作，已收斂**（2026-08-13 實作、2026-08-18 review 確認）。驗證缺口記為 `issue.md` I-074 |
| 優先度 | 中 |
| 分類 | Python / SR Zone / 決策邏輯 |
| 建立日期 | 2026-08-13 |
| 來源 | 使用者需求：lifecycle 判定應與建議產出分離 |

**目標**：新增一個獨立的 Lifecycle Engine，職責只有一件事——**依 Event 的演進決定當前
處於哪一個生命週期狀態**。Decision Engine 改為消費這個狀態，再疊上 RR Gate、策略模式等
條件才輸出最終建議。同一個 lifecycle 狀態因此可以在不同策略立場下得到不同建議，
而狀態判定本身只有一套。

#### 現況盤點（2026-08-13 對照程式碼）

**一、lifecycle 不是不存在，而是存在四套不同的詞彙**

| 來源 | 位置 | 狀態集合 | 語意 |
|---|---|---|---|
| **單一事件** | `event_engine.py:13-17` | `CANDIDATE` / `CONFIRMED` / `ACTIVE` / `RESOLVED` / `EXPIRED` | **一個事件自己的生老病死**——timeline 的基本單位 |
| pipeline 層 | `decision_engine.py:957-975`（`_decision_semantic_pipeline` 內） | `NORMAL` / `TESTING` / `CONFIRMED` / `CONTINUATION` / `BREAKDOWN` / `INVALIDATED` / `NO_PRIMARY_ZONE` | **整體事件演進**——本次要抽出的就是這個 |
| zone 層 | `decision_engine.py:162`（`_zone_lifecycle`） | `CANDIDATE` / `VALIDATED` / `CONFIRMED` / `WEAKENING` / `BROKEN` / `INVALIDATED` | **zone 本身的健康度**，與事件演進是不同軸 |
| 規劃中 | [T-041](#t-041sr-zone-決策顯示補齊-lifecycleevent-timeline-與-strategy-layer) | `Started` / `Testing` / `Confirmed` / `Failed` | 前端顯示用，尚未實作 |

**四套裡有三套共用 `CANDIDATE` / `CONFIRMED` 這兩個字但意思都不同**，這是這一塊難讀的主因。
`_zone_lifecycle` 的輸出還以 `"lifecycle"` 為鍵放進 decision summary
（`decision_engine.py:156`），與 pipeline 的 `lifecycle_phase` 在同一份報告裡並存。

**分層的正確說法**：單一事件的狀態機（第 1 列）是**輸入**，pipeline lifecycle（第 2 列）是
**輸出**。本筆要抽出的是後者，前者維持不動——但兩者都叫 lifecycle 會讓人以為是同一件事。

**二、目前的 lifecycle 判定會讀 RR Gate——正是要拆掉的耦合**

`_decision_semantic_pipeline` 的 `CONTINUATION` 分支條件包含 `rr_qualified`
（`rr_gate.qualified`）。**風險報酬比是策略條件，不是事件事實**：同一段價格行為，
在 RR 不合格時被判成 `CONFIRMED`、合格時才變 `CONTINUATION`。這讓「現在處於什麼階段」
無法獨立回答，也是本次抽離最實質的一項。

**三、Trading / Investment 策略模式目前不存在**

全 repo 沒有任何 strategy mode 的實作（`swing` 只出現在防守線的價位計算，是不同概念）。
T-041 規劃了 `Trading` / `Swing` / `Investment` 三層但仍是待規劃。

#### 修改目標（2026-08-13 review 後定案為 P0）

**採 P0：snapshot-compatible engine。** 輸入維持現有的
`event_state_summary` / `daily_price_action` / `structure_state`，**不等 chain contract**。

理由：T-045 P1 是 Go 端唯讀 API，產出的 chain **不會流進 Python 分析流程**——
Python 目前只透過 `previous_event_states`（`analysis/client.go:952`）拿到最新快照。
要讓 Lifecycle Engine 吃 chain 必須另補 Go→Python request contract、analysis client mapping
與 replay 管線，那是獨立工程（見 T-045 的 `runtime_chain`）。**等 T-045 對本筆沒有幫助**，
兩筆因此各自獨立、可並行。

1. 新增 `lifecycle_engine.py`：**純函數**，輸出 lifecycle 狀態 ＋ reason codes。
   不 import RR、不 import 策略模式。**文件要明說它現階段是 snapshot-based 而非 chain-based**，
   避免日後誤以為它已經看得到事件演進的完整歷程。
2. `decision_engine.py` 改為呼叫它，移除內嵌的 lifecycle 推導與 RR 耦合。
3. `_zone_lifecycle` **改為增量式更名，不做破壞性改名**：新增語意清楚的鍵
   （`zone_health_state`），舊的 `"lifecycle"` 鍵保留並標記 deprecated。
   **不用 `zone_state`**：`scenario_engine.py` 已有一個同名但語意不同的函式（場景判定）。
   原因是 `SRZones.svelte` 有 5 處消費它（`best_trade_zone` / `nearest_support_zone` /
   `nearest_resistance_zone` / `primary_structural_zone`），破壞性改名會把「引擎抽離」
   與「API／前端 contract 遷移」綁成同一批，兩件事的風險性質完全不同。
   顯示名稱的收斂留給 T-041。

#### 不做的範圍

- **不實作 Trading / Investment 策略模式**。那是 T-041 的 Strategy Layer，範圍與風險都是
  另一個量級。本次只**留出接縫**：Decision Engine 取得 lifecycle 之後的那一段，改寫成
  可以依模式分岔的形狀，但只實作現有的單一模式。
- **不新增第四套狀態詞彙**。T-041 的 `Started/Testing/Confirmed/Failed` 應改為直接渲染
  本引擎的狀態集合，而不是再定義一組並維護對應表。
- 不改 zone builder、probability、scoring 的任何邏輯。
- 不改前端（T-041 另案）。

#### 狀態機提案

沿用 pipeline 現有的七個狀態，**不重新命名**——它們已經寫進 DB（`sr_zone_decision_events`）、
API 與 replay 報告，改名的漣漪遠大於收益。判定順序即優先序：

| # | 狀態 | 進入條件（抽出後） |
|---|---|---|
| 1 | `INVALIDATED` | `structure_state == SUPPORT_RECLAIM_INVALIDATED` |
| 2 | `BREAKDOWN` | 有 active bearish event，或 `structure_state == BREAKDOWN` |
| 3 | `CONTINUATION` | `CLOSE_RECLAIM` ＋ 價格延續 ＋ 動能確認 ＋ 明確突破 zone（**移除 `rr_qualified`**） |
| 4 | `CONFIRMED` | `CLOSE_RECLAIM` ＋（`SUPPORT_RECLAIM_CONFIRMED` 或 reclaim 已滿一日） |
| 5 | `TESTING` | `event_signal ∈ {CLOSE_RECLAIM, SUPPORT_TEST}` |
| 6 | `NO_PRIMARY_ZONE` | 沒有 primary zone |
| 7 | `NORMAL` | 其餘 |

`event_signal` 的推導（`decision_engine.py:919-943`）一併移入本引擎——它本來就是純粹的
事件分類，留在 decision engine 沒有理由。

#### 預期的行為改變（使用者已同意重構可伴隨行為改變，此處逐項列出）

**唯一的來源改變是 `CONTINUATION` 不再要求 `rr_qualified`**，但它的**影響面不只 lifecycle 欄位**
——這點初版計畫低估了**兩次**：第一次漏了下游推導鏈，第二次（2026-08-13 review）
漏了「原本落到 `TESTING` 的樣本也會變成 `CONTINUATION`」這一整條路徑。
完整的行為改變清單與已接受的結論已歸檔到
[`sr-zone-scoring.md`](./sr-zone-scoring.md) 的「已知並接受的行為改變」。`lifecycle_phase` 是下游一連串推導的輸入
（`decision_engine.py:977-1035`），所以「RR 不合格但價格延續成立」的樣本會沿著整條鏈改變：

```
lifecycle_phase  CONFIRMED          → CONTINUATION
market_state     BULLISH_RECOVERY   → BULLISH_CONTINUATION
bias_state       BULLISH_BIAS       → BULLISH_CONTINUATION
        ↓
market_bias / position_action_condition / final_entry_permission / 前端顯示
```

因此**驗證不能只比對 `lifecycle_phase`**，必須涵蓋：
`decision_derived_view.semantic_pipeline`、`market_bias`、`position_action_condition`、
`final_entry_permission`，以及 replay 的 `final_entry_state` / `lifecycle_phase` /
`market_bias` 分佈。

`_zone_lifecycle` 改為**增量新增鍵**（見上方修改目標），因此不構成破壞性 contract 變更。

其餘狀態的判定條件完全不變。

#### 受影響檔案與資料流

```
event_engine.detect_market_events ─┐
zone/structure state ──────────────┼→ [新] lifecycle_engine.resolve(...) → lifecycle + reason_codes
daily_price_action ────────────────┘                                              │
                                                                                  ▼
rr_gate ──────────────────────────────────────────────→ decision_engine（建議產出）
strategy mode（T-041，尚未存在）───────────────────────↗
```

| 檔案 | 動作 |
|---|---|
| `python/backtest/modular/sr_scoring/lifecycle_engine.py` | 新增：純函數 ＋ 狀態常數 |
| `python/backtest/modular/sr_scoring/decision_engine.py` | 移除內嵌推導，改呼叫；`_zone_lifecycle` 更名 |
| `python/backtest/modular/sr_scoring/tests/` | 新增 lifecycle 狀態機的單元測試 |
| `docs/api-reference.md`、`docs/sr-zone-scoring.md` | contract 與現況說明 |

#### 主要風險與回滾

- **最大風險是「重構」變成「悄悄改變決策」**。`decision_engine.py` 有 2,747 行，
  lifecycle 的下游消費點分散（`market_state`、`bias_state`、`action_state`、entry gate）。
  對策：先寫**行為對照測試**——抽離前後對同一組輸入產出的完整 decision summary 必須逐欄一致，
  只有上表列出的那一項可以不同。
- RR 解耦後若下游沒有補上對應的 gate，會讓進場建議變寬鬆。上線前要用 decision replay
  比對 `final_entry_state` 的分佈，確認沒有整體放寬。
- 回滾：純 Python 變更，`git revert` 即可；沒有 migration、沒有資料寫入格式變更
  （除非 zone `"lifecycle"` 鍵更名，那一項要與前端同批進退）。

#### 測試與驗證策略

- `python/scripts/test.sh`：lifecycle 狀態機的表格驅動測試，涵蓋七個狀態與優先序邊界
  （例如 bearish event 與 CLOSE_RECLAIM 同時成立時必須是 `BREAKDOWN`）。
- **行為對照**：抽離前先錄一組 decision summary 快照，抽離後逐欄比對。
- `scripts/run-evaluation.sh MODE=replay`：對真實資料跑 decision replay，比對
  `final_entry_state` 與 `lifecycle_phase` 的分佈變化，量化 RR 解耦的實際影響。
- 記憶體：replay 走既有腳本，注意這台 host 的限制（見 `sr-zone-scoring.md`「規模上限」）。

#### 完成後歸檔

- 狀態機定義、優先序與各狀態語意 → [`sr-zone-scoring.md`](./sr-zone-scoring.md)。
- 若 zone `"lifecycle"` 鍵更名 → [`api-reference.md`](./api-reference.md) 與
  [`database-schema.md`](./database-schema.md)。
- 「lifecycle 不看 RR」這條分層原則 → `sr-zone-scoring.md`，避免日後又被加回去。

#### P0 實作結果（2026-08-13）

| 項目 | 內容 |
|---|---|
| `lifecycle_engine.py`（新） | `resolve_lifecycle()` 純函數，**簽章裡沒有 `rr_gate`**。`event_state_types` / `event_state_max_age` 兩個純事件 helper 一併移入 |
| `decision_engine.py` | 改為呼叫，移除內嵌的 `event_signal` ＋ `lifecycle_phase` 推導與 RR 耦合 |
| `_zone_lifecycle` → `_zone_health_state` | **增量新增** `zone_health_state` 鍵，舊的 `lifecycle` 保留並標 deprecated |
| 測試 | `test_lifecycle_engine.py` 12 個 test function（其中一支 parametrized 成 5 個 case，pytest 實收 16 個 lifecycle cases），涵蓋七個狀態、優先序、延續三條件的每一項缺失 |

**差點自己製造出第五套同名詞彙**：原本要把 `_zone_lifecycle` 改名為 `_zone_state`，
但 `scenario_engine.py` **已經有一個 `_zone_state`**，回傳的是場景判定
（`WAIT_FOR_DIRECTION` / `RETEST_REQUIRED` / `SUPPORT_RETEST` / `RESISTANCE_REJECTION`），
概念與值域都不同。同一個 package 內兩個同名不同義的函式，正是本筆開頭診斷的那個問題。
改用 `_zone_health_state`，並在 docstring 同時標明它與 `lifecycle_phase`、
`scenario_engine._zone_state` 三者的區別——**三者都在描述 zone，但問的是三個不同的問題**。

#### 驗證現況與**尚未關閉的缺口**

抽離後 428 支既有測試全數通過。**但這不是「沒有行為改變」的證據**——
它是「沒有任何既有測試涵蓋 RR 解耦那條路徑」的證據。兩者結論完全不同，不能混為一談。

行為改變確實存在且可由結構證明：舊條件要求 `rr_qualified`，不滿足時會落到下一個分支
（`CONFIRMED` 或 `TESTING`）；移除後同樣輸入得到 `CONTINUATION`。
兩支測試分工：`test_continuation_only_needs_price_evidence` 鎖住「延續只看三項價格證據」，
`test_widened_path_previously_testing_now_continuation` 鎖住**真正變寬的那條路徑**
（收復未確認 ＋ `age_bars=0`，舊碼落 `TESTING`、新碼是 `CONTINUATION`）。
注意前者**無法**防守「RR 被加回來」——`resolve_lifecycle` 簽章裡沒有 `rr_gate`，
真要加回來會是新增參數，那支測試照樣綠燈。

**計畫要求的完整驗證尚未執行**：decision replay 對真實資料比對
`final_entry_state` / `lifecycle_phase` / `market_bias` 的分佈變化。
現實限制是 live 只有 **4 檔標的 / 20 次分析**（2026-08-18 重新確認，一筆都沒增加），
replay 的統計意義有限。因此目前這個行為改變**只有單元測試層級的證據**。

**2026-08-18 決定：接受現狀並收斂本筆。** 缺口不會消失，但它的本質是
「production 分析資料太少」——那是獨立的問題，不該讓 T-044 無限期掛著。
缺口已轉記為 [`issue.md`](./issue.md) **I-074**（含關閉條件）；
現況規格早已歸檔在 [`sr-zone-scoring.md`](./sr-zone-scoring.md)
「分層原則：lifecycle 不看 RR」與「已知並接受的行為改變」。

#### 與 T-041 的關係

T-041 的三個面向裡，**Lifecycle 正式顯示**與**Strategy Layer** 都依賴本筆先把狀態定義收斂。
建議順序：T-044（本筆，後端分層）→ T-041 的 Lifecycle 顯示 → T-041 的 Strategy Layer。
T-041 原訂的 `Started/Testing/Confirmed/Failed` 應改為直接使用本引擎的狀態集合。

#### Review 決策紀錄（2026-08-13，已採納並整合進上文）

**方向正確，但 T-044 / T-045 的接縫要先收斂再開工。** 現有程式碼確認
`_decision_semantic_pipeline` 的 `CONTINUATION` 分支確實讀 `rr_gate.qualified`
（`decision_engine.py:958-963`），所以「lifecycle 不應看 RR」這個拆分方向是對的：
RR 是進場與策略條件，不是事件演進事實。

需要修正的是輸入契約：T-045 規劃 Lifecycle Engine 應吃 `chain[]`，但本筆的修改目標目前寫成
直接從事件狀態與價格結構抽出 `lifecycle_engine.resolve(...)`。現有 runtime 只把**最新 snapshot**
透過 `previous_event_states` 傳回 Python，並沒有完整 chain。因此開工前要明確二選一：

1. **T-044 P0：snapshot-compatible engine**。先抽出目前 `_decision_semantic_pipeline`
   裡的 lifecycle 判定，輸入仍是 `event_state_summary` / `daily_price_action` / `structure_state`，
   行為維持等價，只移除 RR 耦合。這條路可以先做，但文件要承認它還不是 chain-based engine。
2. **T-045 先補 runtime chain contract**。先讓 Go / Python 在分析流程裡能傳遞 `chain[]`，
   再讓 T-044 的 Lifecycle Engine 直接吃 chain。這條路設計較完整，但範圍明顯跨 Go API、
   Python request contract、replay 與測試。

RR 解耦的影響也不能只記成 `lifecycle_phase` 的變化。現有下游會接著用 `lifecycle_phase`
推導 `market_state`、`bias_state`、`action_state`、`entry_permission_state`
（`decision_engine.py:977-1035`）；因此 RR 不合格但價格延續成立的樣本，可能從
`CONFIRMED / BULLISH_RECOVERY` 變成 `CONTINUATION / BULLISH_CONTINUATION`，再影響
`market_bias`、`position_action_condition` 與前端顯示。驗證策略要擴成完整比對：

- `decision_derived_view.semantic_pipeline`
- `market_bias`
- `position_action_condition`
- `final_entry_permission`
- replay 的 `final_entry_state` / `lifecycle_phase` / `market_bias` 分佈

`_zone_lifecycle` 更名是正確方向，但不建議列為 T-044 必要同批。`"lifecycle"` 目前已在
decision zone summary、frontend 型別與 `SRZones.svelte` 顯示中被消費；若同批破壞性改名，
會把 lifecycle engine 抽離與 API/front-end contract migration 綁在一起。較穩的做法是：
先保留舊 `"lifecycle"`，新增語意清楚的新鍵（例如 `zone_state` 或 `zone_health_state`），
文件標記舊鍵 deprecated，等 T-041 前端整理時再收斂顯示名稱。

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

> **與 [T-044](#t-044抽出獨立的-lifecycle-engine) 必須一起設計**：Lifecycle Engine 的職責是
> 「依 **Event 的演進** 決定狀態」，而演進的載體就是 timeline。先做 timeline，
> Lifecycle Engine 才有正確的輸入形狀；否則它只能繼續讀「當前狀態的快照」，
> 那不是演進，是切片。**建議 T-045 先於或與 T-044 同批進行。**

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

#### 與 T-044 的接縫

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

### T-048：SR Zone 狀態持久化——Zone 身分、Event Lifecycle 與轉換紀錄

| 欄位 | 內容 |
|---|---|
| 狀態 | 進行中（階段 A／B 已完成並 review 通過，2026-08-19；階段 C **修法已實作、七階階梯復驗全數通過，待 review**——F1／F2／F4 三條門檻都歸零，F3 的真實資料覆蓋與 alias 路徑的實測覆蓋仍缺；階段 D 待規劃） |
| 優先度 | 高（T-049 與 T-041 的共同前置） |
| 分類 | Python / Go / DB / SR Zone / 決策資料 |
| 建立日期 | 2026-08-18 |
| 來源 | 使用者需求：把「每次重判」改成「可追蹤的演進」 |
| 關聯 | 接手 T-044（Lifecycle Engine P0 已實作）與 T-045（事件鏈 P1/P2 已實作）；T-049 是它的下游 |

#### 問題：現在的事件鏈會分裂，而且沒有東西會報錯

事件鏈**已經有持久化**（T-045 做的 `market_event_states` / `market_event_detections`），
但它的身分是 `_zone_key()`（`event_engine.py:199`）：

```python
return f"{role}:{price_low:.4f}:{price_high:.4f}"
```

**身分綁在浮點邊界上，而 zone 邊界每次分析都由 ATR 重算。** 2026-08-18 對 live
`market_event_states` 的唯讀盤點抓到兩個實證：

**一、同一個支撐分裂成兩條鏈。** 0050：

| 日期 | zone_key | event |
|---|---|---|
| 07-23 ～ 08-04 | `SUPPORT:102.4916:103.1084` | INTRADAY_RECLAIM |
| **08-05 起** | `SUPPORT:102.4916:103.1084` | INTRADAY_RECLAIM |
| **08-05 起** | `SUPPORT:102.5414:103.1585` | INTRADAY_RECLAIM |

08-05 之後兩個 key **每天並存**，各自帶一條 reclaim 鏈。價格區間重疊 99%，
實際上是同一個支撐。下游看到的是兩個獨立 zone、兩次獨立事件——**這會重複計數**，
而且沒有任何檢查會發現。

**二、`role` 在 key 裡，所以 `ROLE_FLIPPED` 在現行模型中無法表達。** 支撐翻壓力必然
產生一個全新身分、舊鏈直接斷掉。這不是 bug，是資料模型的結構限制。

#### 目標

1. Zone 有**跨交易日穩定的身分**，邊界漂移與角色翻轉都不改變身分。
2. Event 與 Zone 的生命週期是**存下來的事實**，不是每次分析重新推導的結果。
3. 狀態轉換**留痕**：看得出何時、因為什麼從哪個狀態到哪個狀態。

#### 不做的範圍

* **不改任何下游決策邏輯**。Market State / Bias / Final Entry 這批全部留在 T-049。
  本筆做完，決策行為必須與現在**逐欄相同**——這是它的驗收條件之一。
* 不做前端顯示（T-041）。
* 不擴充事件種類到 4 個以外。
* 不重算歷史：既有 `market_event_states` 的 86 列不回填 zone 身分（理由見「相容策略」）。

#### 執行順序（2026-08-18 定案：**3 → 4 → 1 → 2**）

需求原本給的順序是「1 事件表 → 2 Lifecycle Engine → 3 Zone ID → 4 zone 表」。**但 Zone
身分是事件表的外鍵前提**：先建 `event_instances` 就只能沿用會漂移的 `zone_key`，等階段 3
做出穩定 ID 後，階段 1 的資料要整批 migration ＋ 回填，而回填正是「無法可靠判斷兩個
舊 key 是不是同一個 zone」的那件事——**那個回填不可能做對，因為它要解的正是本筆要建的能力**。

先把身分釘死，後面兩張表一開始就長在正確的鍵上：

| 新序 | 原編號 | 內容 |
|---|---|---|
| **A** | 3 | Zone ID + ZoneMatcher（純 Python，不碰 DB） |
| **B** | 4 | `zone_instances` + `zone_transitions` |
| **C** | 1 | `event_instances` + `event_transitions`（外鍵指向 `zone_instances`） |
| **D** | 2 | Lifecycle Engine 支援 4 個事件 |

**A 是唯一沒有現成東西可接手的部分，也是其餘三階段的地基**——它做錯，後面三個階段
會長在錯誤的鍵上，而那時已經有資料了。所以 A 的參數必須用 live 資料決定，
不能在程式裡先寫死一組猜測值（見下）。

---

#### 階段 A：Zone ID + ZoneMatcher —— **已完成（2026-08-19 review 通過）**

#### 階段 B：`zone_instances` + `zone_transitions` —— **已完成（2026-08-19 review 通過）**

身分層（`zone_matcher.py` ＋ 四張表 ＋ `/zone-identity/match` 接線）已實作、已驗收。
**現況規格見 [`sr-zone-scoring.md`](./sr-zone-scoring.md)「Zone 身分與 ZoneMatcher」**
（三層模型、matcher 判準與門檻來源、資格閘門、接線、四個已知限制、實測特性），
schema 與 `from_state` 不變式見 [`database-schema.md`](./database-schema.md)，
重跑驗收的步驟見 [`development-workflow.md`](./development-workflow.md)
「在 dev stack 上做『as-of 階梯』驗收」。

階段 C／D 要接的兩件事：

* **身分層只寫不讀。** 沒有任何決策依賴 `zone_instances`；階段 C 是第一個讀者。
* **`ZoneScore.zone_uid` 還沒接上。** matcher 需要「上一次的 zone 清單」當輸入，
  而 Python 端目前唯一的跨次狀態通道是 Go 傳進來的 `previous_event_states`，
  沒有 `previous_zones`——要餵得動它就得改 contract，那是階段 C 的工作。

#### 階段 C：`event_instances` + `event_transitions`（**修法已實作、階梯復驗通過，待 review**，2026-08-19）

##### 修改目標

把事件的**身分與生命週期**從「每次分析重新推導」變成「存下來的事實」，並讓它的鍵落在
階段 A／B 做出來的穩定 `zone_uid` 上。現行 `market_event_states` 的鍵是
`_zone_key()`（`role:price_low:price_high`），邊界每次由 ATR 重算，同一個支撐會分裂成
兩條並存的鏈——那正是 T-048 要解的原始問題，只是還沒套用到事件層。

事件鏈目前也**沒有被存下來**：`internal/analysis/event_timeline.go` 是靠回讀
`market_event_states` 的歷史快照、在讀取時摺疊出 timeline。摺疊規則與 Python 的
`build_event_state_summary` 是兩份必須保持對稱的實作，而沒有任何東西會在它們分歧時報錯。

##### 不做的範圍

* **不新增事件型別。** `SUPPORT_RETEST` / `RESISTANCE_BREAKOUT` 目前不存在，要新增偵測
  邏輯、會產生新的交易訊號，那是階段 D，這輪不碰。
* **不刪舊表、不改任何決策。** `market_event_states` / `market_event_detections` 繼續
  並行寫入，決策路徑一行不動。本階段**只寫不讀**。
* **不動 `scoring.py` / `pipeline.py`。** 理由與階段 B 相同：那是有決策責任的路徑，
  為一個還沒有讀者的功能去動它，風險與收益不成比例。
* 不動前端。

##### 五個設計決定

**D1：`zone_uid` 從哪裡來 —— Go handler 端關聯，不把匹配搬進 Python。**

事件在 Python 分析當中算出來（`event_engine.py` 用 `_zone_key()`），而身分匹配在 Go、
分析之後（`/zone-identity/match`）。要讓 `event_instances.zone_uid` 指得到
`zone_instances`，兩者得在某處會合。**handler 裡已經同時握有兩邊**：
`result.ToStore()` 回傳的 `zones`（與 `m.ZoneUIDs` 索引對齊）與
`projections.EventStates` / `EventDetections`（帶 `ZoneKey`）。所以關聯完全是本地的，
不需要改 Python 的分析路徑。

**但關聯鍵要由單一 authority 產生。** Go 若自己用 `fmt.Sprintf("%s:%.4f:%.4f", …)`
重建 key，就成了 `_zone_key()` 的平行實作——兩份浮點格式化哪天分歧，關聯會**靜默**失敗
（事件掛不到 zone，看起來就像「這次沒有 zone 事件」）。**改成讓 Python 在 zone 的序列化
裡多輸出一個 `zone_key`**（呼叫同一個 `_zone_key()`），Go 直接字串比對。這是**純新增
欄位的 contract 變更**，不影響既有欄位。

**D2：`event_instances.zone_uid` 必須可為 NULL。** 階段 C 的原始草圖寫 `zone_uid (FK)`
沒說可否為空，但 **live 的 86 筆 `market_event_states` 裡有 12 筆是 `event_scope='SYMBOL'`**
（`zone_key` 就是字串 `"SYMBOL"`），它們本來就不屬於任何 zone。設成 NOT NULL 會讓這
12 筆無處可去，或被迫塞一個假的 zone。

**D3：事件身分＝`(zone_uid, event_family)`**，對應現行的 `(zone_key, event_family)`
（`build_event_state_summary` 的 `states` dict 就是用這個 key）。`SYMBOL` scope 的
`zone_uid` 為 NULL，唯一索引要用 engine 中立的寫法處理 NULL（postgres 的
`UNIQUE` 不擋多個 NULL，三個 engine 行為不一致——比照 067 的做法，用
`event_scope` ＋ 一個 `zone_scope_key` 的生成欄位或明確的 sentinel 值，實作時定案並寫進註解）。

**D4：`SPLIT` / `MERGE` / `RESHAPE` 時，parent 身上的事件不傳給 child。**

這是階段 A 明確留給階段 C 的問題（原文：「parent 身上的 event 要不要跟著傳給 child？
關聯表讓這件事可以被**明確決定**（沿 parent 走），而不是像現在這樣隱性地斷掉」）。

**本計畫的建議是「不傳」**，理由：parent 的身分已經終止，它身上那條事件鏈的前提
（那個 zone 存在）也隨之消失；把它接到 child 等於宣稱「這條鏈延續了」，而那件事
**我們還沒有證據支持**——`RESHAPE` 在實測裡就是「血緣無法解析」的意思。
事件在 parent 終止時一併收掉，並寫一筆帶 `ZONE_IDENTITY_ENDED` 的 transition
把原因留痕。`zone_relations` 的血緣留著，之後若真有需要沿 parent 回溯，資料補得回來；
反過來（先接了再想拆開）拆不回來。

**D5：`event_transitions` 由 Go 端 diff 產生**，比照 `zone_transitions`：handler 已經
在呼叫 Python 之前載入 `previousEventStates`（`GetLatestMarketEventStates`），
新舊狀態的比較所需資料都在同一個地方。**`from_state IS NULL` 的語意沿用 067 定案的
不變式**：誕生是唯一 `from_state` 留白的狀態轉換（見 `database-schema.md`）——
這條規則已經在 zone 那邊踩過一次坑（I-076），事件表不要再踩第二次。

##### as-of 階梯實跑發現（2026-08-19，dev stack 七階 0050）

**以下四個 F 是問題證據，各段裡的「修法」是發現當下的初版想法；
實際要做的以底下「階段 C 修法計畫書」為準**（2026-08-19 使用者確認需求後改寫）。

上面五個決定照原樣實作完成後，在 dev stack 跑 as-of 階梯：0050 日 K，七階
`2026-07-21 / 07-28 / 08-04 / 08-05 / 08-06 / 08-12 / 08-18`（刻意在 08-05 前後連三階，
那是「同一支撐分裂成兩條鏈」的起點），每階 `POST /sr-zones` 帶 `reuse_existing=false`。
**七階全部 201，migration 68 在 dev postgres 套用成功。**

通過的部分：`from_state` 不變式 **0 違反**（誕生以外沒有留白）；`SYMBOL` scope 事件
走 NULL `zone_uid` 正確；`market_event_*` 與決策路徑逐行未動（Go diff 只換掉一行
`persistZoneIdentity` 呼叫改接回傳值，Python diff 是純新增函數）。

以下 F1／F2 是**單元測試看不到、只有跨交易日才會現形**的缺陷；F4 是
2026-08-19 code review 補出的資料一致性缺陷。

**F1：63% 的 ZONE scope 事件關聯不到 zone 身分而被跳過（26 / 41 筆，第七階 8 筆全滅）。**

D1 的假設是「事件與 zone 在同一次分析算出來，key 一定對得上」。**這對新偵測到的事件成立，
對延續的事件不成立**——`market_event_states` 會把事件 carry forward，而那個 zone 不一定
還在這次分析的 zone 清單裡，或還在但 role 已經變了：

| 事件 key | 這次分析的 zone 清單 | zone 身分 | 失敗成因 |
|---|---|---|---|
| `SUPPORT:104.4357:105.0643` | 同區間仍在，role 變成 `AT_ZONE` | `0c2bbbe4` ACTIVE | **role 在 key 裡** |
| `SUPPORT:101.3267:101.9364` | 邊界漂走，不在清單 | 仍 ACTIVE（缺席容忍） | zone 暫時缺席 |

兩個成因、同一個後果：**身分還活著，只是 key 到不了它**。而後果不是「事件掛不上」這麼明顯——
`0c2bbbe4` 的 `SUPPORT_RECLAIM` 鏈停在第六階的 `CONFIRMED`、第七階沒有更新，**鏈靜默凍結**，
資料表面上完全正常。這比斷掉更難查。

**D1 修正**：`zone_instances` 增加 `last_zone_key`（每次分析觀測到時寫入，值仍由 Python 的
`zone_identity_key()` 產生——**單一 authority 的性質不變**，Go 依然只做字串比對）。
關聯順序改成兩段：

1. 本次分析的 `zone_key → zone_uid` map（現行行為）
2. miss 時退到「`state='ACTIVE'` 的身分中 `last_zone_key` 相同者」

兩個活著的身分共用同一個 `last_zone_key` 時（同區間不同 method 並非不可能），沿用
`zoneUIDByZoneKey` 既有規則**保留第一個**並計入 warn，不讓 map 寫入順序決定。
欄位併進**尚未 commit 的 068**，不另開 069。

**F2：終態每次分析重生一條新鏈。**

`660a04ba | SUPPORT_BREAKDOWN` 在階梯裡出現 seq 1（`RESOLVED`）、seq 2（`EXPIRED`）、
seq 3（`EXPIRED`），**seq 2／3 誕生即終態**。原因是 `buildEventIdentityWrite` 的
「上一條鏈已終結 → 開新鏈（seq + 1）」把 Python 每次重報的**同一筆**終態當成了新事件。
「這次看到」不等於「這次發生」。

判別資訊 Python 其實已經給了：`state_json.carried_from_previous`（實測值為 `true`）。

**D5 修正**：開新鏈的條件收緊成「`carried_from_previous == false`，**或**狀態非終態
（不是 `RESOLVED` / `EXPIRED`）」；carried 的終態若找不到活著的鏈就**不寫**，不重生。
旗標從 `state_json` 讀出，**不動 `market_event_states` 的欄位**——「既有欄位逐欄相同」
仍然是本階段的驗收條件。

**F3（不是缺陷，是覆蓋率缺口）**：這輪 45 個 zone 身分**全部 ACTIVE、沒有任何終止**，
所以 **D4（parent 終止時事件收攤、`ZONE_IDENTITY_ENDED`）的端到端路徑一次都沒被觸發**，
目前只有單元測試證據。要補的話得挑一組實測會 `SPLIT` / `RESHAPE` 的 symbol 再跑一次階梯。

**F4：D4 收掉 parent event 時，`event_instances` 的狀態沒有同步終態。**

`buildEventIdentityWrite` 的 D4 分支在 parent zone 因 `SPLIT` / `MERGE` / `RESHAPE` 終止時，
目前只設定：

* `ended_at = now`
* `end_reason = "ZONE_IDENTITY_ENDED"`
* `event_transitions.to_state = "EXPIRED"`
* `reason_codes = ["ZONE_IDENTITY_ENDED"]`

但 instance 本身沿用 `latest` 裡的舊值，沒有同步把 `state` 改成 `EXPIRED`，也沒有把
`active` 改成 `false`。結果會出現「`ended_at` 已有值、transition 說已 `EXPIRED`，
但 `event_instances.state` 仍是 `ACTIVE` / `CONFIRMED` 且 `active=true`」的自相矛盾資料。
這不會影響目前決策（本階段只寫不讀），但會讓後續 T-049 / T-041 讀新表時誤判 parent
事件仍是可用事件，或在比對 `event_instances` 與 `event_transitions` 時看到兩套終態。

**D4 修正**：parent zone 身分終止時，收掉的 event instance 要同步：

* `state = "EXPIRED"`（與 transition `to_state` 對齊）
* `active = false`
* `ended_at = now`
* `end_reason = "ZONE_IDENTITY_ENDED"`
* transition 維持 `from_state = 舊 state`、`to_state = "EXPIRED"`、
  `reason_codes = ["ZONE_IDENTITY_ENDED"]`

補測試：`TestBuildEventIdentityWriteEndsChainWhenZoneIdentityEnds` 不只檢查
`EndedAt` / `EndReason`，也要檢查 parent instance 的 `State == "EXPIRED"`、
`Active == false`，且 transition 的 `FromState` 是原狀態、`ToState == "EXPIRED"`。

**兩點補充（2026-08-19，確認 F4 屬實時追加）：**

* **矛盾在 `event_instances` 單表內就成立**，不必 join `event_transitions`：這個分支的
  進入條件是 `!e.EndedAt.Valid`（鏈還活著），所以沿用的舊值必然是 `ACTIVE` / `CONFIRMED`
  且 `active=true`，寫出來就是「`ended_at` 有值卻 `active=true`」。因此階梯驗收門檻的
  第 ④ 條（見「測試與驗證策略」）是**單表可驗**的，成本很低，應該當常態檢查。
* **F4 與 F3 是同一件事的兩面。** F4 是純靠讀碼發現的——這輪階梯 45 個身分全部 ACTIVE，
  **D4 分支一次都沒被執行過**。所以修完 F4 之後若不挑一組實測會 `SPLIT` / `RESHAPE`
  的 symbol 重跑階梯，F4 的修正一樣**只有單元測試證據**，與 F3 的缺口完全重疊，
  兩者要一起補、不要分兩次跑。

##### 階段 C 修法計畫書（2026-08-19 使用者確認，**已依此實作**；實作中的兩處偏離見下方「修法後的階梯復驗」）

**本節取代上面 F1／F2／F4 各自的「修法」段落**（那些是發現當下的初版想法）。
上面的 F1／F2／F3／F4 敘述保留為**問題證據**，修法一律以本節為準。與初版的三處差異
在文末「與初版修法的差異」列出。

###### 修法範圍與優先序

| 優先 | 問題 | 修法 |
|---|---|---|
| P0 | F4 snapshot / transition 不一致 | terminal write 原子化 |
| P0 | F2 terminal rebirth | `carried_from_previous=true` 禁止建立新 occurrence |
| P0 | F1 chain silent freeze | 既有 instance 優先，直接沿用 `instance.zone_uid` |
| P1 | F1 key 漂移 | zone key **alias history**（不只 `last_zone_key`） |
| P1 | frozen chain 可觀測性 | 關聯決策計數 ＋ warning ＋ 終態不變式 |
| P1 | F3 E2E 覆蓋缺口 | synthetic `SPLIT` / `MERGE` / `RESHAPE` integration test |
| P2 | 真實資料覆蓋 | 挑會 `SPLIT` / `RESHAPE` 的 historical symbol 重跑 as-of 階梯 |

###### 不做的範圍（沿用階段 C 原本的界線）

* 不改任何下游決策；`market_event_states` / `market_event_detections`
  的既有欄位**逐欄相同**仍是驗收條件。
* 不新增事件型別（階段 D）、不動 `scoring.py` / `pipeline.py`、不動前端。
* **不引入 metrics 套件**：`backend/go.mod` 目前沒有 prometheus 或任何 metrics 依賴，
  為一個只寫不讀的功能引進整套 metrics pipeline 是跨系統決策，超出本輪。
  P1 的「metric」以**結構化 log 欄位 ＋ 階梯驗收 SQL 不變式**交付，
  真正的 metrics endpoint 另開 todo（見文末）。
* 不回填既有列：`event_instances.last_zone_key` 與 `zone_key_aliases` 都由下次分析補上。

###### 最終關聯決策（本輪的核心，取代現行的單段 map 查找）

```text id="event_zone_resolution_001"
Incoming market_event_state
             │
             ▼
Does previous event instance exist?        ← 以 (last_zone_key, event_family) 查活鏈
        /                 \
      YES                  NO
       │                    │
reuse instance.zone_uid     │
       │                    ▼
       │             carried_from_previous?   ← 從 state_json 讀
       │                 /        \
       │               YES         NO
       │                │           │
       │           WARN / NOOP      │           ← 不建立新 occurrence，這條路到此為止
       │                            ▼
       │                    resolve current zone
       │                            │
       │                    current key map     ← 本次分析的 zone_key → zone_uid
       │                            │ miss
       │                    key alias history   ← ACTIVE 身分歷來用過的 zone_key
       │
       └───────────────┬────────────┘
                       ▼
             continue-or-create        ← 匯流點：zone_uid 到這裡才第一次已知
                       ▼
                Lifecycle transition
                       │
                       ▼
         snapshot + transition atomically
```

順序的含義：**instance 延續性 > carried 護欄 > key 解析（本次 map → alias history）**。
alias history 只服務「全新關聯」，不用來救既有鏈條——既有鏈條由第一段直接接住，
連 key 都不必解析，這才是 F1 chain silent freeze 的根治。

**匯流點不是無條件開新鏈——活鏈要用兩把鑰匙查（2026-08-19 定案）。**

第一段的查找鍵是 `(last_zone_key, event_family)`，也就是**事件身上帶的 key**；
而 resolve 段產出的是 `zone_uid`。兩者不等價，中間漏一格：

```text id="event_chain_two_keys_001"
鏈 C 活著，掛在 zone Z，last_zone_key = "SUPPORT:102.4916:103.1084"
今天 Z 的邊界被 ATR 重算 → 本次 zone_key = "SUPPORT:102.5414:103.1585"
今天在 Z 上有一個真的新偵測（carried = false）

  ① (last_zone_key, family) 查 → miss（C 記的是舊 key）
  ② carried == false          → 護欄放行
  ③ resolve → current key map → Z
  ④ 若此時無條件開新鏈 → Z 上出現第二條活鏈，而 C 還活著
```

④ 的後果**正是本輪要根治的那個**：`ListLatestChains` 每組只回最大 `seq`，
C 從此撈不出來、`last_seen_at` 不再更新——F1 的靜默凍結原樣重現，
只是成因從「key 到不了身分」換成「身分到不了鏈」。

所以匯流點是 continue-or-create：

```go id="event_chain_resolve_001"
// 第一把鑰匙：事件身上的 key
if chain := liveChainByZoneKey(st.ZoneKey, st.EventFamily); chain != nil {
    update(chain)                    // 沿用 chain.ZoneUID，不解析 key
    return
}
if carriedFromPrevious(st) {
    warnOrphanCarriedEvent()         // carried 不得建立新 occurrence
    return
}
zoneUID, ok := resolveCurrentZone(st) // current key map → key alias history
if !ok {
    recordUnmatched(st)
    return
}
// 第二把鑰匙：同一個身分上這個 family 有沒有活鏈
if chain := liveChainByZone(zoneUID, st.EventFamily); chain != nil {
    update(chain)                    // 並把 last_zone_key 更新成本次的 key
    return
}
createNewChain(zoneUID)
```

兩把鑰匙互補，缺一不可：

| 查找鍵 | 接住的情況 |
|---|---|
| `(last_zone_key, family)` | zone **解析不出來**（key 漂走、role 翻轉、zone 本次缺席）——鏈仍要延續 |
| `(zone_uid, family)` | zone 解析得出來，但鏈記的 key 是舊的——**不可開第二條** |

`update()` 每次都要把 `last_zone_key` 更新成本次事件帶的 key，
否則第一把鑰匙會停在鏈誕生那天的值，往後永遠 miss。

**三個守門條件**（漏掉會把 F1 的修法變成新的資料污染來源）：

1. **第一段命中的鏈，其 `zone_uid` 若落在本次 `EndedZoneUIDs`（`SPLIT`/`MERGE`/`RESHAPE`）
   就不延續**，交給 D4 收攤。否則會出現「事件在已終止的身分上繼續活著」，
   而 D4 的迴圈又因為 `written` 去重而跳過它——鏈永遠收不掉。
2. **alias history 只指向 `state='ACTIVE'` 且 `ended_at IS NULL` 的身分**（在 SQL 就過濾）。
   不能把事件掛到收攤過的身分上。
3. **第一段命中、但本次 map 對同一個 `zone_key` 給出不同 `zone_uid` 時，以 instance 為準
   並記 `chain_conflict` 計數**。使用者的決策樹指定 instance 優先；衝突本身要看得見。

`SYMBOL` scope 事件不走 zone 解析，`zone_scope_key` 直接是 `'SYMBOL'`，
但第一段與第二段（carried 護欄）照樣適用。

###### P0-F4：terminal write 原子化

抽出唯一的終態寫入點：

```go id="event_terminal_write_001"
func applyEventTerminalState(inst *store.EventInstance, state, reason string, now time.Time)
// 一次設定四項：State = state、Active = false、EndedAt = now、EndReason = reason
```

兩個呼叫點都改走它：

* `st.State ∈ {RESOLVED, EXPIRED}` → `applyEventTerminalState(&inst, st.State, st.State, now)`
* D4（zone 身分終止）→ `applyEventTerminalState(&inst, "EXPIRED", "ZONE_IDENTITY_ENDED", now)`，
  transition 維持 `from_state = 舊 state`、`to_state = "EXPIRED"`、
  `reason_codes = ["ZONE_IDENTITY_ENDED"]`

**`Active` 強制 `false` 需要先驗證不改變既有欄位**：`EVENT_FAMILY_LIFECYCLE_RULES` 的
`gating_states` 是 `(CONFIRMED, ACTIVE)`，`RESOLVED` / `EXPIRED` 都不在其中，
所以 Python 端終態的 `active` 本來就是 `False`。實作時用單元測試把這件事釘住，
而不是靠讀規則推論。

**DB 層再補一道**：`event_instances` 的 upsert 目前 `ended_at` / `end_reason` 有
`COALESCE` 保護，但 `state` / `active` 是無條件覆寫——已終結的鏈若被寫入非終態，
`ended_at` 保住了、`state` 卻退回 `ACTIVE`，又是同一種自相矛盾。改成：

```sql id="event_terminal_upsert_001"
state  = CASE WHEN event_instances.ended_at IS NOT NULL THEN event_instances.state  ELSE excluded.state  END,
active = CASE WHEN event_instances.ended_at IS NOT NULL THEN FALSE                  ELSE excluded.active END
```

三個 engine 都支援 `CASE`（mysql 用 `VALUES(...)` 與表名限定的既有寫法）。
純函數已經不會產出這種列，這道是**不變式的最後一關**，成本很低。

###### P0-F2：carried 事件不建立新 occurrence

`state_json.carried_from_previous`（Python 在 `_normalize_previous_event_state`
無條件設 `True`）是「這次沒有新偵測，只是把上次的狀態抄過來」的唯一判別資訊。

**規則（2026-08-19 使用者定案）：`carried_from_previous == false` 是建立新 event
occurrence 的必要條件。** 狀態是不是終態**不參與**這個判斷。

決策樹第二段：**沒有活的既有 instance ＋ `carried_from_previous == true` → WARN + NOOP**，
不開新鏈、不寫 instance、不寫 transition。

初版修法寫的是「`carried_from_previous == false`，**或**狀態非終態」——那個 `或`
只堵住階梯上實際看到的症狀，留下一格漏洞：

| `carried_from_previous` | 狀態 | 初版（OR） | 定案規則 |
|---|---|---|---|
| `false` | 終態 | 可開新鏈 | 可開新鏈 |
| `false` | 非終態 | 可開新鏈 | 可開新鏈 |
| `true` | 終態 | 不可開（F2 的症狀） | 不可開 |
| `true` | **非終態** | **可開新鏈**（漏洞） | **不可開** |

最後一格是同一種重生，只是新鏈掛在 `ACTIVE` 而不是 `EXPIRED`——資料表面上更像正常事件、
更難查。理由與 F2 本身同一條：carried 代表「重報」不代表「發生」；找不到活鏈就表示這條鏈
已經被收掉或從未建立，此時開新鏈是把重報寫成新事件。todo 原文把「carried 的非終態找不到
活鏈時的行為」列為待定案，定案就是這一條。

旗標從 `StateJSON` 解析（`store.MarketEventState.StateJSON` 已是 `RawJSON`），
**不動 `market_event_states` 的任何欄位**。解析失敗（欄位缺席／型別不符）視為
`false`，並計入 `carried_parse_failed`——把解析問題當成 carried 會靜默吃掉真實新事件。

###### P0-F1：既有 instance 優先

`event_instances` 新增 `last_zone_key VARCHAR(96) NULL`：**這條鏈最近一次被觀測到時，
事件身上帶的 `zone_key`**。每次寫入 instance 時一併更新（upsert 要更新它）。

`ListLatestChains` 一併回傳，`buildEventIdentityWrite` 建第二個索引：

```text id="event_chain_index_001"
byScopeKey[zone_scope_key|event_family]  → 現行（seq 計算、開新鏈仍靠它）
byZoneKey[last_zone_key|event_family]    → 新增，**只收 ended_at IS NULL 的活鏈**
```

`byZoneKey` 同一組出現兩條活鏈時（不同 `zone_uid` 曾用過同一個 key），
沿用 `zoneUIDByZoneKey` 的既有規則**保留第一個並計入 `chain_key_ambiguous`**，
順序以 `ListLatestChains` 既有的 `ORDER BY zone_scope_key, event_family` 決定，
不讓 map 寫入順序決定。

`SYMBOL` scope 的鏈 `last_zone_key` 寫 `'SYMBOL'`，與 `zone_scope_key` 一致。

###### P1-F1：zone key alias history

新表併進**尚未 commit 的 068**（三份 engine 都要），不另開 069：

```sql id="zone_key_aliases_001"
CREATE TABLE IF NOT EXISTS zone_key_aliases (
    zone_uid      VARCHAR(64) NOT NULL REFERENCES zone_instances(zone_uid),
    zone_key      VARCHAR(96) NOT NULL,
    first_seen_at TIMESTAMPTZ NOT NULL,
    last_seen_at  TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (zone_uid, zone_key)
);
CREATE INDEX IF NOT EXISTS idx_zone_key_aliases_key
    ON zone_key_aliases (zone_key, last_seen_at);
```

* **寫入**：`buildZoneIdentityWrite` 為「這次觀測到的每個 zone」產一筆
  `(zone_uid, 本次 zone_key)` upsert（命中就只推進 `last_seen_at`）。
  `ZoneIdentityWrite` 加 `KeyAliases []ZoneKeyAlias`，在**同一個既有交易**內寫。
* **上限**：每個 `zone_uid` 只保留最新 **8** 筆，在同一交易內對本次觸及的 `zone_uid`
  做 prune。邊界每次由 ATR 重算，不設上限這張表會隨分析次數單調成長；
  8 筆足以涵蓋「缺席容忍 3 次 ＋ 角色翻轉前後」的回溯需求。
* **讀取**：`ZoneIdentityRepo.ListKeyAliases(ctx, symbol, timeframe)` →
  `map[zone_key]zone_uid`，SQL 內 join `zone_instances` 過濾
  `state='ACTIVE' AND ended_at IS NULL`，同一個 `zone_key` 對到多個活身分時
  取 `last_seen_at` 最新者並回報衝突數。
* **`zone_instances.last_zone_key` 不加**。alias 表最新的一列就是它，加了會有兩份事實，
  而兩份事實分歧時沒有任何東西會報錯——那正是本筆一路在解的那類問題。

alias 覆蓋 F1 兩個成因：role 從 `SUPPORT` 變 `AT_ZONE`（舊 key 仍在 alias 裡）、
邊界漂走（舊邊界的 key 仍在 alias 裡）。

###### P1：frozen chain 可觀測性

`buildEventIdentityWrite` 的第二個回傳值從 `[]string` 換成結構：

```go id="event_identity_stats_001"
type eventIdentityStats struct {
    MatchedByChain    int      // 第一段：既有 instance 直接沿用 zone_uid
    MatchedByCurrent  int      // 第三段：本次分析的 key map
    MatchedByAlias    int      // 第三段：退到 alias history
    UnmatchedKeys     []string // 三段都到不了身分
    CarriedNoop       []string // carried 且無活鏈 → 不建立 occurrence
    ZoneEndedSkipped  []string // 身分本次終止，交給 D4
    ChainConflicts    []string // instance.zone_uid 與本次 map 不一致
    ChainKeyAmbiguous []string // 同一個 last_zone_key 有兩條活鏈
    CarriedParseFail  int      // state_json 解析不出旗標
    Invariant         []string // 終態不一致，**恆應為空**
}
```

* `persistEventIdentity` 以**單筆結構化 log** 輸出全部欄位（zap field，
  可被日誌聚合直接數）。
* `UnmatchedKeys` / `ChainConflicts` / `ChainKeyAmbiguous` / `CarriedParseFail`
  非零 → `Warn`。
* `Invariant` 非空 → **`Error`**。它掃 `w.Instances`：
  `EndedAt.Valid` 但 `State ∉ {RESOLVED, EXPIRED}` 或 `Active == true`。
  這是 F4 在**寫入前**的攔截，與 DB 層的 `CASE` 與階梯驗收的 SQL 是同一條不變式的三道。

###### P1-F3：synthetic 血緣 integration test

新檔 `backend/internal/store/zone_event_identity_lifecycle_test.go`（sqlite ＋ 真 migration）。
**不經 HTTP**：`h.client` 是具體型別 `*analysis.Client`，為此改成介面是跨模組重構，
與本輪範圍不成比例。改為直接組 `analysis.ZoneIdentityMatchResult`
（含 `SPLIT` / `MERGE` / `RESHAPE` 與 `TerminatedPrevious`）餵進
`buildZoneIdentityWrite` → `ZoneIdentityRepo.Apply` → `buildEventIdentityWrite` →
`EventIdentityRepo.Apply`，逐階推進三～四階，再用 SQL 驗不變式。

**要驗的是純函數測不到的東西**：兩個 repo 的交易互動、外鍵、upsert 的
`COALESCE` / `CASE` 保護、以及 D4 收攤在真的有資料時是否落地。
HTTP 那一跳仍只由 as-of 階梯覆蓋，這點寫進歸檔。

`buildZoneIdentityWrite` / `buildEventIdentityWrite` 目前在 `handler` package，
要在 `store` 的測試裡用得把它們**匯出或搬家**。取捨：把測試放在
`backend/internal/api/handler/` 下、由測試自己開 sqlite ＋ 跑 migration
（`store` 已有現成寫法可抄），**不搬動生產程式碼**。實作時採這個方向。

###### P2：真實資料覆蓋

P0／P1 完成、0050 七階重跑通過**之後**才做，順序不可對調——階梯要驗的四條門檻
在修法未完成前必然失敗，先跑只會浪費一輪。

1. 從 live 唯讀盤點，找出 `zone_instances` 曾出現 `SPLIT` / `MERGE` / `RESHAPED`
   的 symbol；live 目前沒有這類資料時，改從 `stock_sr_zone_analyses` 找
   zone 邊界跨日變動最大的 symbol 當候選。
2. 對候選跑一次 as-of 階梯（步驟見 `development-workflow.md`），
   驗收門檻與 0050 同六條。
3. **找不到真的會終止身分的 symbol 時**，D4 以 synthetic integration test 交付，
   並把「D4 的真實資料覆蓋仍缺」寫進 `sr-zone-scoring.md` 的已知限制，
   不當作已驗證。這是明確的降級路徑，不是默默跳過。

###### 受影響的檔案（本修法新增／變更的部分）

| 檔案 | 變更 |
|---|---|
| `migrations/{postgres,sqlite,mysql}/068_event_identity.sql` | ＋`event_instances.last_zone_key`、＋`zone_key_aliases` 表（三份同步） |
| `internal/store/model.go` | — |
| `internal/store/event_identity_repo.go` | `EventInstance.LastZoneKey`；upsert 更新 `last_zone_key`、`state`/`active` 加 `CASE` 保護；`ListLatestChains` 帶回新欄位 |
| `internal/store/zone_identity_repo.go` | `ZoneKeyAlias` 型別、`ZoneIdentityWrite.KeyAliases`、`Apply` 內 upsert ＋ prune、新增 `ListKeyAliases` |
| `internal/api/handler/sr_zones.go` | `zoneIdentityOutcome` ＋`AliasUIDByZoneKey`；`buildEventIdentityWrite` 改三段決策 ＋ `applyEventTerminalState` ＋ `eventIdentityStats`；`persistZoneIdentity` 產生 alias 寫入並讀 `ListKeyAliases`；`persistEventIdentity` 結構化 log |
| `internal/api/handler/sr_zones_event_identity_test.go` | 補齊下列單元測試 |
| `internal/api/handler/sr_zones_identity_lifecycle_test.go`（新） | synthetic `SPLIT`/`MERGE`/`RESHAPE` integration test |
| Python | **不動**（`zone_key` 已在未 commit 的 diff 裡，本修法不再改 Python） |
| 前端 | 不動 |

###### 資料 contract 變化

| 變更 | 型態 | 相容性 |
|---|---|---|
| `event_instances.last_zone_key` | 純新增欄位（可空） | 068 尚未 commit，無既有資料 |
| `zone_key_aliases` | 新表 | 沒有讀者以外的依賴；空表時 alias 段等同 miss，行為退回現行 |
| `market_event_*` / 決策路徑 | **不變** | 逐欄相同仍是驗收條件 |

###### 主要風險與回滾

| 風險 | 對策 |
|---|---|
| instance 優先把事件掛到**已終止**的身分上 | 守門條件 1（`EndedZoneUIDs` 檢查）＋ `ZoneEndedSkipped` 計數 ＋ 階梯門檻⑤ |
| 匯流點無條件開新鏈 → 同一 `(zone_uid, family)` 兩條活鏈，舊鏈被 `ListLatestChains` 的 `MAX(seq)` 蓋掉而靜默凍結 | 匯流點做 continue-or-create（第二把鑰匙）＋ 階梯新增門檻⑦ |
| carried 護欄過嚴，吃掉真實的新事件 | 新事件的 `carried_from_previous` 為 `false`（Python 只在 `_normalize_previous_event_state` 設 `True`），單元測試釘住；解析失敗一律當 `false` 並計數 |
| alias history 讓事件掛到**錯的**身分 | 只指向 ACTIVE 身分；衝突取最新並計數；上限 8 筆限制回溯深度 |
| `zone_key_aliases` 無上限成長 | 每 `zone_uid` prune 到最新 8 筆，與寫入同一交易 |
| 三段決策讓 `buildEventIdentityWrite` 變複雜、錯了看不出來 | 維持純函數；每一段各有單元測試；`eventIdentityStats` 讓每一段的命中數在 log 裡可數 |
| 回滾 | 仍是純新增（新欄位、新表、新分支）。068 尚未 commit，`git revert` ＋ `-- +goose Down` 即可；已寫入的新表留著無害 |

###### 測試與驗證策略

單元（`buildEventIdentityWrite`，在既有測試檔擴充）：

* **第一段**：既有活鏈以 `last_zone_key` 命中 → 沿用 `instance.zone_uid`，
  即使本次 map 完全沒有這個 key（**這一條就是 F1 的回歸測試**）；
  命中但 `zone_uid ∈ EndedZoneUIDs` → 不延續、交給 D4；
  同一 `last_zone_key` 兩條活鏈 → 保留第一個並計入 `ChainKeyAmbiguous`；
  instance 與本次 map 不一致 → 以 instance 為準並計入 `ChainConflicts`。
* **第二段**：carried ＋ 無活鏈 ＋ **終態** → 不寫（F2 回歸）；
  carried ＋ 無活鏈 ＋ **非終態** → 一樣不寫（本計畫定案）；
  非 carried ＋ 無活鏈 → 照常開新鏈（`seq + 1`）；
  `state_json` 解析不出旗標 → 當 `false` 並計入 `CarriedParseFail`。
* **第三段**：本次 map 命中；map miss ／ alias 命中（role 已翻轉、邊界漂走各一組）；
  三段全 miss → 進 `UnmatchedKeys` 且**不**寫 NULL `zone_uid`。
* **匯流點（第二把鑰匙）**：第一把鑰匙 miss、carried=false、resolve 命中，
  且該 `zone_uid` 上**已有活鏈** → 沿用該鏈（`event_uid` 不變、`seq` 不變），
  **不得開第二條**；同一情境下該 `zone_uid` 上沒有活鏈才 `seq + 1` 開新鏈。
  沿用時 `last_zone_key` 要更新成本次事件帶的 key（否則第一把鑰匙往後永遠 miss，
  要有獨立一條測試釘住）。
* **F4**：`RESOLVED` / `EXPIRED` 與 D4 兩條路徑都產出
  `State`＝終態、`Active=false`、`EndedAt` 有值、`EndReason` 正確，
  且 transition 的 `FromState` 保留舊狀態、`ToState='EXPIRED'`；
  `Invariant` 掃描在正常輸入下恆為空。
* `SYMBOL` scope 走 NULL `zone_uid`、`zone_scope_key='SYMBOL'`（既有測試不得退化）。

repo 層（sqlite）：

* `zone_key_aliases` upsert 命中只推進 `last_seen_at`；prune 保留最新 8 筆；
  `ListKeyAliases` 過濾非 ACTIVE 身分、衝突取最新。
* `event_instances` upsert：已終結的鏈被寫入非終態時 `state` / `active` 不退回。
* synthetic `SPLIT` / `MERGE` / `RESHAPE` 多階 integration test（P1-F3）。

migration：三個 engine 都實跑——`scripts/test-postgres-migrations.sh`、
`scripts/test-mysql-migrations.sh`、`migrate_sqlite_test.go`。

**as-of 階梯重跑**（0050，七階 `2026-07-21` ～ `2026-08-18`，dev project）。
驗收門檻在原四條上加三條：

1. 「could not be matched」warn **歸零**。
2. 沒有任何鏈誕生即終態：`from_state IS NULL AND to_state IN ('RESOLVED','EXPIRED')` 為 0 列。
3. 每階的 ZONE scope 事件狀態數與寫入 `event_instances` 的數量一致。
4. 不存在 `ended_at IS NOT NULL` 但 `state NOT IN ('RESOLVED','EXPIRED')` 或 `active=true` 的列。
5. **不存在指向非 ACTIVE `zone_instances` 的未終結 `event_instances`**（守門條件 1／2 的成果）。
6. **每階的 `MatchedByChain` ＋ `MatchedByCurrent` ＋ `MatchedByAlias` 等於該階
   ZONE scope 的事件數**，且第七階不再出現靜默凍結的鏈
   （`0c2bbbe4` 的 `SUPPORT_RECLAIM` 要在第七階有 `last_seen_at` 更新）。
7. **同一個 `(symbol, timeframe, zone_scope_key, event_family)` 不存在兩條未終結的鏈**
   （`ended_at IS NULL` 分組後 `COUNT(*) > 1` 為 0 列）。這是匯流點 continue-or-create
   的驗收，也是「舊鏈被 `MAX(seq)` 蓋掉」的唯一可觀測訊號。

決策不變的證明：`stock_sr_zone_analyses` 與 `market_event_*` 的既有欄位
在本修法前後**逐欄相同**。

###### 與初版修法的差異（需要記錄的偏離）

1. **F1 的主修法從「`zone_instances.last_zone_key` 兩段查找」改成「既有 instance 優先
   ＋ alias history」**。初版仍要求事件先解析出 key 才找得到鏈，鏈的延續依然被 key
   綁架；新版讓既有鏈直接以 `(last_zone_key, family)` 接住，key 解析只服務新關聯。
   `zone_instances.last_zone_key` 因此**不加**，改為 `zone_key_aliases` 表。
2. **F2 的條件從初版的「`carried == false` **或** 狀態非終態」收斂成
   「`carried == false` 是必要條件」**（使用者定案）。差別只在「carried ＋ 非終態 ＋
   無活鏈」那一格：初版允許開新鏈，定案不允許。同時把 todo 原本列為待定案的
   「carried 的非終態找不到活鏈」定案在同一條規則上。
3. **P1「metric」以結構化 log ＋ 階梯 SQL 交付，不引入 metrics 套件**。
   若要真正的 metrics endpoint，另立一筆 todo（**待使用者裁示是否現在就開**）。

###### 完成後歸檔

現況規格併進 [`sr-zone-scoring.md`](./sr-zone-scoring.md)「Zone 身分與 ZoneMatcher」
（擴成涵蓋事件層與三段關聯決策），schema（`zone_key_aliases`、
`event_instances.last_zone_key`）進 [`database-schema.md`](./database-schema.md)，
階梯驗收的六條門檻補進 [`development-workflow.md`](./development-workflow.md)。


##### 修法後的階梯復驗（2026-08-19，dev stack 七階 0050，**全數通過**）

把 dev DB 清空重建（含把 goose 68 退掉重套，才會有後加進 068 的 `zone_key_aliases`），
用同一組七階日期 `07-21 / 07-28 / 08-04 / 08-05 / 08-06 / 08-12 / 08-18` 重跑。
**七階全部 201。** 逐階的 `event identity` 拆解與收尾的不變式 SQL：

| 門檻 | 修法前 | 修法後 |
|---|---|---|
| `unmatched_zone_keys`（F1） | 26 / 41 掛空，第七階 8 筆全滅 | **每一階都是空的** |
| 終態重生（F2） | 每次分析重開新 occurrence | `event_instances` 收斂到 10 條（修法前 14 且持續長） |
| `invariant_violations`（F4） | D4 路徑寫出矛盾快照 | **每一階都是空的** |
| `from_state` 留白（誕生除外） | 0 | 0 |
| ZONE scope 事件掛空 zone | 26 筆 | 0 |
| alias 外鍵／每 zone 8 筆上限 | — | 0 違反 |

收尾狀態：`zone_instances` 45（全 ACTIVE）、`zone_transitions` 74、`zone_key_aliases` 75、
`event_instances` 10（8 筆掛 zone、2 筆 SYMBOL scope）、`event_transitions` 16、
`market_event_states` 48——**zone 身分層的三個數字與修法前逐一相同**，
證明本輪只動事件層。

`carried_noop` 收斂到 5 筆，逐筆比對 `event_instances` 後確認**全部是已終態的鏈**
（1 筆 `RESOLVED`、4 筆 `EXPIRED`）被 Python 的 carry forward 重報，護欄擋下重生——
這正是 F2 要的行為。

###### 復驗過程中改掉的兩件事（偏離計畫，已實作）

1. **Python 兩條路徑都要寫 `carried_from_previous`**（`event_engine.py`
   `build_event_state_summary`）。原本只有 `_normalize_previous_event_state` 無條件寫
   `True`，新偵測的事件**根本沒有這個鍵**，於是 Go 端把每一筆新事件都算成
   `carried_parse_failed`——第一階七筆事件就 5 筆「解析失敗」，那個計數等於沒有訊號。
   現在新偵測寫 `False`，**缺鍵才是異常**。副作用：`market_event_states.state_json`
   多一個布林鍵（既有鍵一個都沒動）。
2. **`CarriedNoop` 從 `hasWarning()` 移除**（`sr_zones.go`）。計畫書列的 Warn 條件本來就
   只有 `UnmatchedKeys` / `ChainConflicts` / `ChainKeyAmbiguous` / `CarriedParseFail`，
   實作多算了它。終態被 carry forward 是常態，每一條走完的鏈此後每次分析都貢獻一筆——
   實測單調累積到 5 筆、階階觸發，warn 因此永遠不會歸零，真正的異常會被淹掉。
   它仍然出現在 log 欄位裡，只是不再自己決定級別。`AliasAmbiguous` 則**刻意留著**
   （性質與 `ChainKeyAmbiguous` 相同，計畫書列舉時漏了）。
   移除後重跑第七階：**warn 一筆都沒有**。

###### 仍未被實測覆蓋的兩塊（不是缺陷，是覆蓋缺口）

* **alias history 一次都沒被走到**：七階的 `matched_by_alias` 全部是 0。既有鏈優先
  （`matched_by_chain`）已經把 F1 的兩個成因都接住了，alias 是它的備援。
  目前只有單元測試（`TestBuildEventIdentityWriteFallsBackToAliasWhen*`）覆蓋。
  **四檔 21 日的每日階梯重跑後仍然是 0**，見下方 P2。
* **F3 原樣還在**：這輪 45 個 zone 身分仍然全部 ACTIVE，`zone_ended_skipped` 全空，
  D4 收攤路徑（F4 修的那條）在真實資料上**一次都沒被執行**。
  **已由下方 P2 的每日階梯補上**（6182 執行了 4 次）。

順帶發現一筆與本 todo 無關的問題：同一個交易日重複分析會讓事件提早老化到 `EXPIRED`
（`age_bars` 的計數單位是分析次數不是 K 棒），記為 [`issue.md`](./issue.md) I-077。

##### P2 真實資料覆蓋：每日階梯 × 四檔（2026-08-19，**F3 缺口已補上**）

0050 那組七階跑不出血緣邊的原因不是 symbol 選錯，是**階距**：週距下 zone 早就漂到不重疊，
直接走「缺席→失格」，2→2 的元件根本組不起來。改成**每日**階梯（2026-07-21～08-18 共
21 個交易日）並換成會 churn 的標的：`2330`、`3105`、`6182`、`8150`
（後三者近 60 根日均振幅 7.4～7.8%、量能 24M～60M）。**84 次分析全部 201。**

| symbol | 身分數 | SPLIT | MERGE | RESHAPE | 血緣邊 |
|---|---|---|---|---|---|
| 2330 | 82 | 2 | 4 | 6 | 19 |
| 6182 | 108 | 2 | 6 | 10 | 29 |
| 8150 | 68 | 0 | 2 | 4 | 9 |
| 3105 | 71 | 0 | 0 | 0 | 0 |

（中間三欄是 `zone_instances.state` 的終態分布，最右是 `zone_relations` 的邊數。）

**D4 收攤路徑在真實資料上執行了 4 次**：6182 的四條 `SUPPORT_RECLAIM` 鏈，parent 都是
`RESHAPED` 的身分，寫出來的快照是 `state=EXPIRED`／`active=false`／
`end_reason='ZONE_IDENTITY_ENDED'`——**三個欄位一致**，正是 F4 修的那條路徑。
修法前這條路只設 `ended_at` / `end_reason`，會留下 `state` 仍是 `ACTIVE` 的矛盾快照。

六條門檻在這組資料上全數通過（84 次分析、329 個身分、57 條血緣邊、59 條事件鏈）：
`unmatched_zone_keys` 0、`invariant_violations` 0、`carried_parse_failed` 0、
`from_state` 留白 0、ZONE scope 事件掛空 zone 0、alias 外鍵與每 zone 8 筆上限 0 違反；
另加一條 D4 專屬檢查「`ZONE_IDENTITY_ENDED` 的 parent 仍是 ACTIVE」也是 0。
三段關聯決策的命中分布：`matched_by_chain` 49、`matched_by_current` 18、
`matched_by_alias` **0**——即使在 57 條血緣邊的高 churn 資料上，alias 也沒有輪到它
決定關聯，它仍然只有單元測試覆蓋。

##### F5：alias 索引與 matcher 對「還活著」用了兩套定義（**已實作並端到端復驗／待 review**）

上面那輪出現 **85 筆 `alias_ambiguous`**（37 筆 warn 全部由它觸發）——
同一個 `zone_key` 對到多個仍算「活著」的身分。逐筆查下去，成因不是 key 撞號：

```
uid       symbol method        last_role   price_low   price_high  state   observed_absences
b9cb296b  8150   recent_pivot  RESISTANCE  118.144500  118.855500  ACTIVE  4
7c64eabe  8150   recent_pivot  RESISTANCE  118.144500  118.855500  ACTIVE  4
```

兩個身分的 role／method／邊界**逐位元相同**，而且都還是 `ACTIVE`。真正的成因是
**兩個模組對「還活著」的判準不同**：

* matcher 的資格閘門是 `observed_absences <= 3` ＋ `last_seen_at` 在 20 個交易日內
  （`zone_matcher.py` 的 `MAX_OBSERVED_ABSENCES` / `MAX_ABSENCE_TRADING_DAYS`）。
  失格的身分列進 `expired_previous`，**不再進候選集合**。
* alias 索引的查詢是 `z.state = 'ACTIVE' AND z.ended_at IS NULL`
  （`listZoneKeyAliasesSQL`）。而階段 B 的定案是**失格只收掉「這一世」，
  身分本身仍是 `ACTIVE`**——於是 matcher 早就放棄的身分，在 alias 索引裡照樣是候選。

量化：20 個撞號的 key、涉及 32 個身分，其中 **25 個（78%）的 `observed_absences` 已經
超過 3**。也就是說 `alias_ambiguous` 主要是在數殭屍身分。

**這輪沒有寫出錯誤資料**——`matched_by_alias` 是 0，alias 一次都沒有真的決定過關聯。
但它是潛伏的：只要既有鏈那一段 miss 一次，事件就可能掛到一個 matcher 早已放棄的身分上。
而且它讓 warn 又變成常態噪音（37/37），與 `carried_noop` 當初的毛病同一類。

**修法方向**（尚未實作，兩個選項）：

1. **用 matcher 的裁決當唯一權威**：`persistZoneIdentity` 手上已經有
   `ZoneIdentityMatchResult.expired_previous`，把這批從本次 alias 索引排除。
   門檻的定義維持 Python 一份，不必在 Go 複製 `MAX_OBSERVED_ABSENCES`。
2. **在 SQL 加 `z.observed_absences <= N`**：改動最小，但把 Python 的常數複製到 Go，
   兩邊分歧時沒有東西會報錯——正是 T-048 一路在消滅的那種模式。**不建議**。

順帶一提，`observed_absences` 對失格身分仍會持續累加（實測到 4），因為它們每輪都
不在候選集合裡。目前無害，但那個計數是無上界的。

###### 選項 1 已實作並復驗：只擋得住「失格發生的那一輪」（2026-08-19）

`aliasUIDByZoneKey(refs, matched.ExpiredPrevious)` 已實作，image 重建後把同一組階梯
（同樣四檔 × 21 個交易日，2026-07-21～08-18）在退乾淨的 dev DB 上重跑一次，
**84 次分析全部 201**。

**行為零回歸**：zone 身分 329、血緣邊 57、事件鏈 59、transitions 116、alias 685、
`ZONE_IDENTITY_ENDED` 4 次——與修法前逐項相同；六條門檻＋D4 專屬檢查全部 0，
`unmatched_zone_keys`／`chain_conflicts`／`chain_key_ambiguous`／`carried_parse_failed`／
`invariant_violations` 全程 0，`matched_by_alias` 仍是 0。

**但主目標幾乎沒動**：`alias_ambiguous` **85 → 77**、由它觸發的 warn **37 → 36**。
最終 DB 裡仍有 **16 個 `zone_key` 對到多個 `state='ACTIVE'` 的身分**（修法前 20 個），
涉及 32 個身分，其中 **25 個的 `observed_absences` 是 4**——與修法前同一批殭屍。

成因是 `expired_previous` 是**單輪快照**：`ListLive` 的次數軸
（`observed_absences <= zoneIdentityMaxAbsences`）讓失格身分**下一輪就不再進 matcher**
（`zone_identity_repo.go` 的介面註解本來就寫明這個設計——放它進來一次、收攤把次數推過
上限、下次就擋在 SQL）。所以一個身分一生只會出現在 `expired_previous` **一次**；
那一輪之後它永遠沉在 alias 索引裡，選項 1 再也碰不到。8 筆的減幅正好就是
「剛好在這一輪失格」的那些。

**選項 2 當初的反對理由不成立**：`zoneIdentityMaxAbsences = 3` **早就存在於 Go**
（`sr_zones.go:1062`，註解寫著「與 zone_matcher.MAX_OBSERVED_ABSENCES 同值」），
而且它就是餵給 `ListLive` 的那個參數。常數已經複製過去了，alias 索引改用**同一個**值
不新增分歧來源，反而讓 alias 索引與 matcher 的候選集合由同一個地方定義。
對本輪資料試算：alias 索引加上 `z.observed_absences <= 3` 後，撞號的 key **16 → 0**。

**已實作（2026-08-19）**：`listZoneKeyAliasesSQL` 加上 `AND z.observed_absences <= ?`，
`ListKeyAliases` 多收一個 `maxObservedAbsences`，呼叫端傳的就是餵給 `ListLive` 的
`zoneIdentityMaxAbsences`——alias 索引與 matcher 的候選集合自此由**同一個地方**定義。
選項 1 的 `expired_previous` 排除保留，兩道過濾各管一段：SQL 擋「已經沉下去的」，
`expired_previous` 擋「這一輪剛失格、次數還沒推過上限的」。
新增 `TestZoneKeyAliasListExcludesZonesOverAbsenceLimit` 同時釘住「超過上限要排除」與
「剛好等於上限要留著」（用 `<` 會讓收攤流程變成不可達的死碼）。`backend/scripts/test.sh` 全綠。

**端到端實測（2026-08-19，image 重建後重跑）**：dev DB 退乾淨（`candles`、身分／事件／alias
與 `stock_sr_zone_analyses` 及其六個參照方一起 TRUNCATE，保留 `stg_candles`）後，
同一組四檔 × 21 個交易日重跑，**84 次分析全部 201**。

* **`alias_ambiguous` 77 → 0**：這一輪 backend log **一筆 warn 都沒有**
  （84 行全是 `level=info` 的 request log；唯一的 warn 是啟動時的 FinMind api_key 提醒，
  在階梯開始之前）。上一輪是 36 筆 warn、全部由 `alias_ambiguous` 觸發。
  試算的「16 → 0」到此有了端到端證據。
* **行為零回歸**：zone 身分 329、血緣邊 57（2330 19／6182 29／8150 9／3105 0）、
  事件鏈 59、transitions 116、alias 685、`ZONE_IDENTITY_ENDED` 4 次——與修法前逐項相同；
  終態分布也相同（ACTIVE 293／RESHAPED 20／MERGED 12／SPLIT 4）。
  六條門檻＋D4 專屬檢查（`ZONE_IDENTITY_ENDED` 的 parent 仍是 ACTIVE）全部 0。

**判讀陷阱：直接對 DB 數撞號會是 16，不是 0。** F5 改的是**查詢路徑**
（`listZoneKeyAliasesSQL` 的候選條件），不是刪 alias 資料——`zone_key_aliases`
裡那些殭屍身分的列還在，所以不帶 `observed_absences <= 3` 去 group 一樣會數到 16 個
撞號 key（涉及 31 個身分：25 個 `observed_absences=4`，其餘 4／1／1 個是 0／1／3）。
帶上與 `ListLive` 相同的次數上限再數才是 0。要驗這條修法，看的是
`alias_ambiguous` 的 warn 有沒有消失，不是 DB 的原始撞號數。

**復驗方式的限制**：`logging.NewWithConfig` 把 level 寫死 `zap.InfoLevel`
（`backend/internal/logging/logger.go`），所以 Debug 版的 `event identity: zone association`
**任何環境都不會輸出**——每階的逐段命中數只有在該次分析觸發 warn 時才看得到。
上面的 `matched_by_chain` / `matched_by_alias` 數字都是「warn 樣本內」的值，不是全 84 次的總和。
要全量觀測得等 T-050 的 metric。


##### 受影響的檔案與資料流

```
Python: serialization.py（+zone_key，純新增）
            │
            ▼
Go handler: result.ToStore() ─► zones ─┐
                                       ├─► zoneKey → zone_uid 對照
            projections.EventStates ───┘        │（miss 時退到 F1 修法）
                                                │
            zone_instances.last_zone_key ───────┤（ACTIVE 身分的最後一次 key）
                                                ▼
                              event_instances / event_transitions（新）
            projections.EventStates ─────► market_event_states（既有，照舊寫）
```

* Python：`backtest/modular/sr_scoring/serialization.py`（＋`zone_key` 輸出）、
  對應測試。**`event_engine.py` / `lifecycle_engine.py` 不改。**
* Go：`internal/store/`（新增 `EventIdentityRepo`，比照 `zone_identity_repo.go` 的
  `ListLive` ＋ 單一交易 `Apply`）、`internal/api/handler/sr_zones.go`
  （`persistEventIdentity` ＋ `buildEventIdentityWrite` 純函數）、
  `internal/analysis/client.go`（`SRZone` 多接一個 `zone_key` 欄位）
* Go（**F1／F2／F4 修法追加**）：`internal/store/zone_identity_repo.go`
  （寫入 `last_zone_key`，`ListLive` 一併回傳）、`internal/store/model.go`
  （`ZoneInstance.LastZoneKey`）、`sr_zones.go`（關聯改兩段查找、開鏈條件讀
  `state_json.carried_from_previous`；D4 收 parent event 時同步 `state=EXPIRED` /
  `active=false`）
* Migration：`{postgres,sqlite,mysql}/068_event_identity.sql`（**三份都要**；
  **F1 修法的 `zone_instances.last_zone_key` 併進這三份，不另開 069**）
* 前端：不動

##### 資料 contract 變化

| 變更 | 型態 | 相容性 |
|---|---|---|
| Python zone 序列化 ＋ `zone_key` | 純新增欄位 | 舊 Go 忽略它即可 |
| `event_instances` / `event_transitions` | 新表 | 沒有讀者 |
| `market_event_*` | **不變** | 逐欄相同是驗收條件 |
| `zone_instances` ＋ `last_zone_key`（F1 修法） | 純新增欄位 | 067 已部署，欄位可為空；舊列由下次分析補上 |

##### 主要風險與回滾

| 風險 | 對策 |
|---|---|
| 事件關聯不到 zone（key 對不上）而**靜默**掛空 | D1 讓 key 由單一 authority 產生；再加一條「ZONE scope 的事件關聯失敗要記 warn 並計數」，不能只是寫進 NULL。**這條風險實測發生了**（F1，26／41 筆），warn 是唯一抓到它的東西——修法見「as-of 階梯實跑發現」 |
| 修完 F1 後仍有 key 到不了身分的殘量 | 階梯重跑時 warn 計數必須降到 0；非 0 就是還有第三種成因沒找出來，不能當已知限制帶過 |
| `zone_uid` 只在 `reuse_existing=false` 那條路徑產生 | 與階段 B 同一個已知限制：`event_instances` 也只在該路徑寫，統計的是分析的子集。**沿用不擴大**，記進主題文件 |
| 同 symbol 併發分析 | 與階段 B 相同的已知限制，本階段不處理（沒有決策讀它） |
| 新舊兩套事件狀態不一致 | 並行寫入 ＋ 逐日逐欄比對 SQL；差異只能來自「分裂被正確合併」，其他來源都算 bug |
| 回滾 | 純新增（新表、新欄位、新模組），既有路徑不改。`git revert` ＋ `-- +goose Down`；已寫入的新表留著無害 |

##### 測試與驗證策略

* **`buildEventIdentityWrite` 拆成純函數**，比照 `buildZoneIdentityWrite`——這段的錯誤
  （事件掛錯 zone、鏈斷掉、誕生沒留痕）在資料裡看起來都很正常。
* 單元：事件誕生／延續／resolve／expire 各一組；`from_state` 不變式；
  parent 終止時事件收掉（D4）；SYMBOL scope 的事件走 NULL `zone_uid`。
* 單元（**F1 修法**）：本次分析 map 命中；map miss ／ `last_zone_key` 命中（role 已翻轉、
  zone 暫時缺席各一組）；兩個 ACTIVE 身分共用同一個 `last_zone_key` 時保留第一個並計入 warn；
  身分已終止時**不**回退（不能把事件掛到收攤過的身分上）。
* 單元（**F2 修法**）：carried 的終態找不到活鏈時不開新鏈；非 carried 的新事件照常開新鏈
  （seq + 1）；carried 的**非**終態找不到活鏈時的行為要明確定案並測。
* 單元（**F4 修法**）：parent zone 身分終止時，event instance 與 transition 的終態一致：
  `event_instances.state='EXPIRED'`、`active=false`、`end_reason='ZONE_IDENTITY_ENDED'`，
  且 transition `from_state` 保留舊狀態、`to_state='EXPIRED'`。
* **三個 engine 的 migration 都要實跑**：`scripts/test-postgres-migrations.sh`、
  `scripts/test-mysql-migrations.sh`、`migrate_sqlite_test.go`。
* **as-of 階梯**（工具已現成，見 `development-workflow.md`）：跑完後比對
  `event_instances` 與 `market_event_states` 的每日 active 集合。
  **修法完成後要重跑同一組七階**（0050，07-21 ～ 08-18），驗收門檻四條：
  ①「could not be matched」warn 歸零；② 沒有任何鏈誕生即終態
  （`from_state IS NULL AND to_state IN ('RESOLVED','EXPIRED')` 為 0 列）；
  ③ 每階的 ZONE scope 事件狀態數與寫入 `event_instances` 的數量一致；
  ④ 不存在 `ended_at IS NOT NULL` 但 `state NOT IN ('RESOLVED','EXPIRED')` 或 `active=true` 的列。
* **D4 端到端仍缺覆蓋**（F3）：這輪沒有任何 zone 身分終止。要補得挑一組實測會
  `SPLIT` / `RESHAPE` 的 symbol 再跑一次，否則 D4 只能靠單元測試交付，這件事要寫進歸檔。
* **決策不變的證明**：`stock_sr_zone_analyses` 與 `market_event_*` 的既有欄位
  在本階段前後必須**逐欄相同**。

##### 完成後歸檔

現況規格併進 [`sr-zone-scoring.md`](./sr-zone-scoring.md)「Zone 身分與 ZoneMatcher」
（擴成涵蓋事件層），schema 進 [`database-schema.md`](./database-schema.md)。

#### 階段 D：Lifecycle Engine 支援 4 個事件

要支援 `SUPPORT_BREAKDOWN` / `SUPPORT_RECLAIM` / `SUPPORT_RETEST` / `RESISTANCE_BREAKOUT`。
**盤點後發現這四個的現況差很多，不是同一種工作：**

| 事件 | 現況 |
|---|---|
| `SUPPORT_BREAKDOWN` | ✅ 已是 event family（`HIGH_VOLUME_BREAKDOWN`） |
| `SUPPORT_RECLAIM` | ✅ 已是 event family（`INTRADAY_RECLAIM`） |
| `SUPPORT_RETEST` | ❌ **不存在**。只在 `scenario_engine._zone_state` 當**場景字串**，不是事件 |
| `RESISTANCE_BREAKOUT` | ❌ **不存在**。只在 `evaluation.py` 當 replay 的兩根 K 結果標籤 |

所以後兩個**是新增偵測邏輯**，不是把既有東西存下來。它們要各自定義：觸發條件、
`EVENT_TYPE_META`（family / direction / default_state / resolves）、
`EVENT_FAMILY_LIFECYCLE_RULES`（gating_states / expires_after_bars）、以及 `EVENT_ORDER`。

**`RESISTANCE_BREAKOUT` 另外有一個結構性問題**：現行 `EVENT_FAMILY_LIFECYCLE_RULES`
全部是 `SUPPORT_*` 與 `VOLUME_CONTEXT`，沒有任何壓力側的 family。加它進來會是第一個
壓力側事件，`resolves` 的語意（壓力突破是否 resolve 支撐事件）要先想清楚。

#### 受影響的檔案與資料流

```
zone_builder.py ──► ZoneMatcher（新）──► ZoneScore.zone_uid
                                            │
scoring.py / pipeline.py ───────────────────┤
                                            ▼
event_engine.py（zone_key → zone_uid）  lifecycle_engine.py（＋2 個新事件）
                                            │
                                            ▼
              Go: analysis/client.go contract ＋ store repo ＋ migration 067
```

* Python：`zone_builder.py`、`types.py`、`event_engine.py`、`lifecycle_engine.py`、
  `scoring.py`、`pipeline.py`、`serialization.py`、新增 `zone_matcher.py`
* Go：`internal/analysis/client.go`（contract）、`internal/store/`（新 repo）、
  `internal/database/migrations/{postgres,sqlite,mysql}/067_*`（**三份都要**）
* 前端：本筆不動

#### 風險與回滾／相容策略

| 風險 | 對策 |
|---|---|
| ZoneMatcher 誤配把兩個真實 zone 併成一個 | 門檻先用 live 資料掃描決定；`zone_transitions` 留痕讓誤配看得出來 |
| 新舊兩套事件狀態不一致 | **並行寫入一段時間**，新表不餵給任何決策；用 SQL 逐日比對兩邊的 active 集合 |
| mysql migration 從未部署（I-054） | 三份 migration 都寫，但只有 postgres/sqlite 有執行證明，照 I-054 的既有處置 |
| 回滾 | 三個階段都是**純新增**（新表、新欄位、新模組），既有路徑不改。`git revert` ＋ `-- +goose Down` 即可；已寫入的新表留著無害，因為沒有任何決策讀它 |

**相容策略的核心：本筆全程「只寫不讀」。** 新表寫入、舊路徑照跑、決策完全不變。
這讓階段 A～D 可以逐個交付而不需要一次性切換，也讓 T-049 有真實資料可以比對。

#### 測試與驗證策略

* ZoneMatcher：純函數，用 live 抓下來的真實 zone 形狀當 fixture（比照
  `selection_report.py` 的已知答案測試）。**必測 0050 那組 IoU > 0.9 的分裂案例**。
* 狀態機：合法/非法轉換各一組；`ROLE_FLIPPED` 後身分不變。
* 事件：4 個事件各自的觸發與過期；`RESISTANCE_BREAKOUT` 的 `resolves` 語意。
* **並行比對**：新舊兩套同時寫入後，用 SQL 比對每日的 active 事件集合，
  差異只能來自「分裂被正確合併」，不能有其他來源。
* 決策不變的證明：`stock_sr_zone_analyses` 的既有欄位在本筆前後必須逐欄相同。

#### 完成後歸檔

現況規格寫進 [`sr-zone-scoring.md`](./sr-zone-scoring.md)（Zone 身分與 ZoneMatcher、
狀態機、四個事件的定義），schema 寫進 [`database-schema.md`](./database-schema.md)，
contract 變更寫進 [`api-reference.md`](./api-reference.md)。

**階段 A／B 的部分已歸檔**（2026-08-19）：`sr-zone-scoring.md`「Zone 身分與 ZoneMatcher」、
`database-schema.md` 的四張表與 `from_state` 不變式、`development-workflow.md` 的
as-of 階梯驗收步驟。階段 C／D 完成後補上事件層的部分。

---

### T-049：Market State 與所有下游改讀同一套 state

| 欄位 | 內容 |
|---|---|
| 狀態 | 待規劃（**前置未完成**） |
| 優先度 | 中 |
| 分類 | Python / SR Zone / 決策邏輯 |
| 建立日期 | 2026-08-18 |
| 來源 | 同 T-048，階段 5～6 |

**範圍**：Market State 改讀 Lifecycle 而不再自己重判 Event；接著 Bias /
Daily Confirmation / Reclaim / Event Sequence / Final Entry 全部改讀同一套 state。

**依使用者確認，本筆先只列方向與驗證門檻，細節等 T-048 落地、看過真實資料形狀再寫。**

#### 兩個前置條件，缺一不可

1. **T-048 完成**，且新舊兩套狀態已並行比對過一段時間。
2. **補分析排程**——「定期對 watchlist 產生 SR zone 分析」。
   這是 `issue.md` **I-074** 的關閉條件，也是本筆唯一可行的驗證來源：
   目前 `stock_sr_zone_analyses` 只有 **4 檔 / 20 次分析**（2026-08-18 再次確認未增加），
   而本筆會同時改動 Bias、進場、事件序列——**比 T-044 那次影響面大一個量級**，
   不能再用「428 支測試全綠」當證據交付。

   這個排程目前**沒有任何 todo 在追**，只散見於 T-045 的討論段落。它本身有成本
   （每檔都要跑一次 Python scoring，記憶體限制見 `sr-zone-scoring.md`「規模上限」），
   要獨立評估。

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
| 狀態 | 待規劃 |
| 優先度 | 中（T-048 階段 C 的後續，不擋階段 C 交付） |
| 分類 | Go / 可觀測性 |
| 建立日期 | 2026-08-19 |
| 來源 | T-048 階段 C 修法計畫書「P1 frozen chain 可觀測性」的取捨：以結構化 log 交付，metric 另立一筆 |
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

#### 要決定的事（規劃時定案，先不寫死）

* **要不要引入 prometheus client**，還是先用「寫進 DB 的一張輕量統計表 ＋ 一個查詢端點」。
  後者不新增依賴、與現有 repo 模式一致，但沒有現成的告警管道。
* **母體問題**：`zone_uid` 只在 `reuse_existing=false` 那條路徑產生
  （T-048 階段 B／C 的既有已知限制），metric 統計的是分析的**子集**。
  指標定義要把這件事寫進去，不然分母會被誤讀。
* **哪些是 gauge、哪些是 counter**：命中率是比例（每次分析算一次），
  `Invariant` 違反是**必須為零**的 counter，兩者的告警語意完全不同。
* 是否一併涵蓋階段 B 的 zone 側（`zone identity: match failed` 等目前也只有 warn）。

#### 前置

T-048 階段 C 的修法完成並 review 通過——`eventIdentityStats` 的欄位定義要先穩定下來，
否則 metric 的 label 會跟著改。

#### 不做的範圍

* 不做告警規則本身（那要先有抓取端）。
* 不改任何決策邏輯。
