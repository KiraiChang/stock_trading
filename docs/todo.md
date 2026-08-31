# TODO：優化與待實作項目

記錄想做但還沒做的優化方向、功能擴充、架構升級。跟 bug/矛盾/限制無關的項目
放這裡；已經發生的問題或已知限制記錄在 [issue.md](./issue.md)。

## 使用說明

- **狀態**：`待規劃` / `規劃中` / `進行中` / `已完成` / `擱置`
- **優先度**：`高` / `中` / `低`（主觀評估，會隨情境調整，不是嚴格排序）
- 新增項目時往下加一筆，編號遞增（`T-0xx`），不要覆蓋舊編號。
- 項目狀態改變時直接更新該筆的「狀態」欄位，不需要搬移位置；若項目已完成
  且不需要保留歷史，可以整筆刪除或搬到文件最下方的「已完成封存」。
- **編號只增不重用，`下一個新編號從 T-068 起算`。**（T-062 於 2026-08-25 收斂；T-063 於 2026-08-26 發出、2026-08-27 收斂；T-064 於 2026-08-27 發出，內容由 `issue.md` I-097 改列；T-065 於 2026-08-27 發出，由 T-055 的 F1 裁決分出；T-055 於 2026-08-27 收斂；T-066 於 2026-08-27 發出，承接 T-055 未完成的 decision replay 驗證；T-067 於 2026-08-28 發出，承接 `issue.md` I-091 未完成的驗收。）
  **發出新編號時記得把這一行一起往前推**——比照 [`issue.md`](./issue.md) 的同名規則，
  那邊漏推過一次，差點重用編號（`I-070` 已經真的重用過一次）。
- **不要用「檔案裡最大值 + 1」決定編號。** 已收斂的項目會整筆移除，但它們的編號
  **仍然被佔用**：程式碼註解、主題文件與 git log 會留著舊 ID，重用會讓兩件無關的事
  共用一個代號。本檔目前看得到的最大是 `T-064`，而 `T-062` 已經發出並收斂，
  更早也有多筆被移除（例如 `T-044` / `T-046` / `T-053` / `T-056` / `T-057` /
  `T-059` / `T-060`）。判斷可用編號時翻 git log 或本節，不要數檔案。
- **移除條目前要先反轉依賴**（與 [`issue.md`](./issue.md) 同一條規則）：先把要長期保留的
  行為、設計與限制寫進 `docs/` 的對應主題文件，再把程式碼與文件裡「見 todo.md T-0xx」
  這類**活指標**改成指向該主題文件，只保留「原記於…，已收斂」形式的歷史引用。
  收斂後用這條檢查沒有殘留：

  ```bash
  comm -13 <(grep -o '^### T-0[0-9][0-9]' docs/todo.md | sed 's/### //' | sort -u) \
           <(rg --no-filename --only-matching --no-messages \
                --glob '!**/node_modules/**' --glob '!**/dist/**' \
                --glob '*.{md,go,ts,svelte,py,sh,yml,yaml,sql}' \
                'T-0[0-9][0-9]' . | sort -u)
  ```

  副檔名清單**每一種都是踩過才加的**，不要精簡它（完整理由見
  [`issue.md`](./issue.md) 的同一條規則）：`.ts` / `.svelte` 是前端、
  **`.yml` / `.yaml` 是 compose 與 config、`.sql` 是 migration 檔頭**。
  另外**不要換回 `grep -rho … | grep -v node_modules`**——`-h` 去掉檔名之後那個
  `grep -v` 濾不掉任何東西。

  列出的 ID 必須**只剩明確標為歷史沿革的引用**（「原記於…」「已收斂」），
  不能有任何「見 T-0xx」形式的活指標。**本節自己會出現在輸出裡**，那是預期的。

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
| 狀態 | **部分通過／主項保留**（2026-08-31 review：程式碼方向通過，但文件明列的 `verify-regression-baseline.sh` regression baseline 驗收**仍未完成**，不宜整筆移除。Step 0～5 已完成，池已匯入 live 並自主運作） |
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
| 5 | Phase 2：`evaluation_universe` 表與每日排程 | **實作 review 通過（2026-08-31）／regression baseline 驗收未完成**（2026-08-17 實作）。**2026-08-18 唯讀盤點：池已匯入（135 檔，非文件的 131）、排程已啟用、當日 15:06 同步 135 檔／0 失敗、池內日 K 全部到 08-18 且無手動回補**——端到端驗收的前半段成立。後半段（`run-evaluation.sh` → `verify-regression-baseline.sh`）尚未跑。詳見 [`evaluation-universe-selection-plan.md`](./evaluation-universe-selection-plan.md)「live 現況與端到端驗收」 |

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
| 狀態 | **部分通過／主項保留**（2026-08-31 review：**Event Timeline 面向的後端與 8 筆 review 發現通過**，含當日對 live 重跑 `verify-event-timeline.sh` 通過；⚠️ **四項 dev stack 驗收仍未做**，見下方「尚未做的驗收（T-041 的單一權威清單）」。Lifecycle 與 Strategy Layer 兩個面向尚未完成，主項保留不移除）；那兩個面向仍待規劃（**前置已解除**：Lifecycle Engine 已於 2026-08-18 收斂並移出本清單，狀態定義見 [`sr-zone-scoring.md`](./sr-zone-scoring.md)「分層原則：lifecycle 不看 RR」） |
| 優先度 | 中 |
| 分類 | SR Zone / Decision UI / Position Action |
| 建立日期 | 2026-08-07 |
| 來源 | 使用者需求：決策畫面需要更完整地呈現 lifecycle、事件鏈與策略層 |

目前 SR Zone 決策顯示仍偏向單一摘要，還沒有把 lifecycle 與不同策略層的
position action 正式整理成可讀的前端狀態。下列三個面向中，**Event Timeline 已於
2026-08-21 交付**（後端與 review 發現已於 2026-08-31 通過），另兩個仍待規劃：

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

#### Event Timeline 面向的實作結果（2026-08-21 實作；後端於 2026-08-31 review 通過）

三個面向裡只做了 **Event Timeline**，接的是 2026-08-20 改讀身分層之後的
`GET /sr-zones/event-timeline`（原記於 `todo.md` T-051，已收斂）。Lifecycle 與 Strategy Layer 兩個面向**這一輪不做**，
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

#### Review 發現（2026-08-21 提出，八筆全數已修；2026-08-31 review 通過）

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

**R5.（2026-08-21 實作，2026-08-31 review 通過）`identity_since` 在視窗被截斷時會說謊——但錯的是後端不是文案。**
前端那句「更早的分析沒有事件鏈（刻意不回填）」忠實照著 `api-reference.md` 第 6 點寫。
落差在實作：`identity_since` 取的是**回傳鏈中最早的 `first_seen`**，而
`ListChains` 的視窗只保證**未終結**的鏈不受限制，視窗（前端固定 `max_analyses=60`）
之前就已終結的鏈會被濾掉。於是半年歷史、60 次分析只涵蓋一個月時，畫面會宣告
「身分層自 07-22 起有紀錄」，而更早的分析其實有鏈。
**這筆屬於當時後端交付的行為與文件不一致**（原記於 `todo.md` T-051，已收斂），不在前端修。
**2026-08-21 依計畫書實作完成（計畫書已隨 review 通過移除）**：`identity_since` 改由
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
**R1 與 R5 各依計畫書實作完成**（R1：後端補 `event_instances.decision_visible`
並逐條回傳，前端標記而不隱藏；R5：`identity_since` 改由身分層全歷史推導——
R5 的計畫書已隨 review 通過移除，現況見 [`api-reference.md`](./api-reference.md) 判讀第 6 點）。
**八筆全部已實作，並於 2026-08-31 review 通過**（前端畫面驗證另計，見「尚未做的驗證」）。

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
⚠️ **尚未在真實資料上重看畫面**——R2 的位置調整與 R4 的重跑後更新要在 dev stack 上以肉眼確認。
**完整的未完成清單見本筆最後的「尚未做的驗收」**，那裡是單一權威來源（這一項只是其中之一）。

顯示端的現況規格（擺放位置、快取鍵、失敗後重抓）已同步到
[`sr-zone-scoring.md`](./sr-zone-scoring.md)「前端 Event Timeline 的判讀規則（現況）」。

#### R1 計畫書：event-timeline 的 chain 補 `decision_visible`（2026-08-31 review 通過）

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
  現況見 [`api-reference.md`](./api-reference.md) event-timeline 判讀第 6 點）。
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

**尚未做的驗證——以下是 2026-08-21 當下的狀態，逐項現況見本筆最後的
「尚未做的驗收」**（那裡是單一權威來源，不要在這裡各自更新）：

| 計畫第 4～6 點 | 2026-08-21 當下 | 現況（2026-08-31） |
|---|---|---|
| 4. as-of 階梯 | 未做 | **仍未做**（權威清單第 1 項） |
| 5. 前端測試 ＋ 在 `6182` 看畫面 | 測試已過、畫面未看 | 測試維持通過；**看畫面仍未做**（第 3 項） |
| 6. 回填後 dev DB 抽驗 | 未做 | **仍未做**（第 2 項） |
| 決策輸出逐欄不變 | 未做 | **已由後端側涵蓋**：2026-08-31 對 live 跑 `verify-event-timeline.sh` PASS |

**2026-08-21 已把新 image 換上 dev backend，migration 071 也已套到 dev DB
（啟動 log `migrations applied version=71`）**——環境前置當時就備好了，
卡住的是這四項都需要**人工在 dev stack 上確認**——形式包含人工操作、log／SQL 檢查與畫面確認。

#### 尚未做的驗收（T-041 的單一權威清單）

⚠️ **前一版寫「唯一未完成的是前端畫面」是錯的**——R1 計畫書的驗收第 4、6 點與第 5 點的
「看畫面」那半也都還沒做。全部未完成項集中在這裡，其他段落不要再各自宣稱進度：

| # | 項目 | 出處 | 狀態 |
|---|---|---|---|
| 1 | dev stack 的 as-of 階梯（四檔 21 階，兩個 family 全 `false` / 其餘全 `true`） | R1 計畫書驗收第 4 點 | **未做** |
| 2 | 回填後對 dev DB 唯讀 SQL 抽驗兩個 family 的列數與旗標值 | R1 計畫書驗收第 6 點 | **未做** |
| 3 | 在 `6182` 上看畫面（`decision_visible` 標記而不隱藏） | R1 計畫書驗收第 5 點的後半 | **未做**（前半的 `test:unit` / `svelte-check` / `build` 已過） |
| 4 | R2 的位置調整、R4 的重跑後更新，在 dev stack 上以肉眼確認 | Review 發現 R2 / R4 | **未做** |

四項都要 **dev stack**，且**沒有腳本可以代勞**——形式各不相同：

* **人工操作**：第 1 項要把 live 日 K 唯讀複製到 staging，再依交易日**逐階釋出**到
  `candles`，每釋出一階打一次分析（步驟見 [`development-workflow.md`](./development-workflow.md)
  「在 dev stack 上做 as-of 階梯驗收」）。
* **log 檢查**：第 1 項每一階要看 backend 的 `event identity: zone association`
  結構化 log，對六條門檻逐條判（`unmatched_zone_keys` 空、`invariant_violations` 恆空、
  `carried_parse_failed` 為 0…）。
* **SQL 檢查**：第 1 項跑完要對 DB 跑不變式 SQL；第 2 項是回填後的唯讀抽驗。
* **畫面確認**：第 3、4 項是肉眼看前端。

這是它們一直留著的原因，不是被遺忘。

**後端側已經驗過，不在缺口內**：`scripts/verify-event-timeline.sh` 於 2026-08-31
對 live 重跑 **PASS**（`TestEventTimelineAgainstLiveData`）。

⚠️ **原本記的阻塞已於 2026-08-31 解除**：當時該腳本連 live DB，而 live 還沒有
migration 071 的 `decision_visible` 欄位（`ListChains` 會 42703）。live 現在已在
**migration 74**、`event_instances.decision_visible` 存在。
**但那驗的是後端鏈資料，不是畫面**——上表四項與它無關，仍未做。
本筆不含 DDL，不需要 migration 測試。

---

### T-042：股價還原的收尾工作

| 欄位 | 內容 |
|---|---|
| 狀態 | **部分通過／主項保留**（2026-08-31 review：cron config 與 volume float 兩個子項**通過**；「逐檔事件的增量更新」仍未完成，主項保留。主體已於 2026-08-11 完成並驗證） |
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
| **逐檔事件的增量更新** | 除權息與減資都是逐檔、目前每次全抓。除權息走 Yahoo（20/分）；**減資走 FinMind（5/分），與每日抓價共用節流器**——1,900 檔光減資就要 6.3 小時且會排擠行情抓取。這是**日後把池擴到約 1,900 檔的前置條件**，不是日後優化；⚠️ **不擋 T-040 已完成的第一階段（120～150 檔，實際 135 檔）**——那個規模下全抓仍可接受 |
| ~~模型用未還原資料訓練~~ | **已驗證，不需重訓**（2026-08-11）。A/B 實測邊際分布沒有位移（`confidence` 與 `trading_score` 的中位數幾乎相同），個別決策改變 1.9%～5.4% 屬於還原修正錯誤輸入。結論與方法見 [`sr-zone-scoring.md`](./sr-zone-scoring.md) 的「模型與還原股價的相容性」 |
| ~~`corporate_action_sync` 的 cron 寫死~~ | **review 通過（2026-08-31）**，2026-08-18 實作：改走 `corporate_action.cron`（環境變數 `CORPORATE_ACTION_CRON`），預設值等於原本的硬編碼 `"30 6 * * 1-5"`，**行為不變**。現況說明見 [`architecture/data-pipeline.md`](./architecture/data-pipeline.md)「公司行動同步」 |
| ~~Python 的 volume 變 float~~ | **review 通過（2026-08-31）**，結論是不改行為，2026-08-18 盤查：下游全部以 float 取用、原始 volume 不跨 Python→Go 邊界，**沒有假設整數的消費者**。補 9 支測試鎖住現況，說明見 [`database-schema.md`](./database-schema.md)「股價還原」 |
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
它明確延後、指名由本筆承接的有四項（第 5 項的老化單位**已於 2026-08-20 修好並收斂**，
保留在此供閱讀脈絡）：

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
5. ~~同一個交易日重複分析會讓事件提早老化到 `EXPIRED`。~~ **已解決**：
   `age_bars` 已改成依「K 棒推進」而不是「被 carry 的分析次數」（2026-08-20 上線）。
   現況規格見 [`sr-zone-scoring.md`](./sr-zone-scoring.md)「老化的單位是『K 棒推進』」
   （原記於 `issue.md` I-077，已收斂）。**本筆不必再處理這一項。**

#### 兩個前置條件，缺一不可

1. **新舊兩套狀態並行比對過一段時間。** T-048 本身已完成，但它的驗收做的是**回歸比對**
   （改動前後決策逐欄相同、身分層數字逐項重現），**不是**計畫書要求的並行比對
   （逐日比對新舊兩套的 active 事件集合，差異只能來自「分裂被正確合併」）。
   起點資料是 2026-08-19 那份落差量測（原記於 `issue.md` I-080，已收斂）：同一份 84 次
   分析裡，`event_instances` 的鏈數與當時 timeline 端點的 `(zone_key, family)` 組合數
   **雙向都對不上**——多出來的是 key 漂移拆開的鏈，少的是身分終止後的重生鏈。
   （端點已於 2026-08-20 改讀身分層，現況規格見
   [`sr-zone-scoring.md`](./sr-zone-scoring.md)「事件層：鏈的身分與三段關聯決策」。）
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
| 狀態 | **review 不通過／待修復**（2026-08-31：`summary` 被明細的 `limit` 截斷，區間語意失真，見下方「review 發現」） |
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

* `job_runs` **查不到任何 `sr_analysis` 紀錄**——當時 `runPreMarket`（每天 08:50）會
  `DeleteBefore(TodayTaipei())`，**只保留當天**，前一晚的紀錄在早上 08:50 被例行清掉。
  （保留期已於 2026-08-25 改成 30 天，這一點不再成立；但下面那條「唯一線索只剩 log」
  的結論在當時是真的，也是本筆立案的背景。）
* 唯一還原得出真相的只有 `docker logs`，而且要一路往回翻才看得到
  `sr analysis done ... skipped=11` 與每檔的 skip 原因。

也就是說**當時的兩個機制都答不出「昨晚那輪做了什麼」**：log 沒人翻，
`job_runs` 隔天就沒了。本筆要建的表**因此不沿用 `job_runs` 當時的當日清除策略**
——那正是要解的問題。（`job_runs` 已於 2026-08-25 改成保留 30 天，這段是立案背景。）

#### 計畫書（**已實作；2026-08-31 review 不通過**——見上方「review 發現」，2026-08-21）

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

#### review 發現（2026-08-31）——**不通過：summary 不是完整 days 區間的聚合**

端點宣稱「回溯 `days` 天的區間聚合」（`sr_zones.go:2120-2127` 的註解：
`days` 預設 30、`limit` 預設 200），但實際資料流是**先過濾日期、再截斷筆數、
然後對截斷後的結果算 summary**：

| 步驟 | 位置 | 行為 |
|---|---|---|
| 1 | `sr_identity_stats_repo.go:108-136` | `WHERE created_at >= From` ＋ `ORDER BY created_at DESC` ＋ **`LIMIT 200`**（`q.Limit <= 0` 時的預設） |
| 2 | `sr_zones.go:2159-2161` | `summarizeIdentityStats(rows)` —— 直接對**那 200 列**聚合 |

`sr_analysis` 排程上線後每交易日產出約 22 列（11 檔 × 兩輪，見 T-052「上線實績」），
所以**預設的「30 天 summary」實際只涵蓋約 9 個交易日**。

而且 response 只有 `rows` 與 `summary` 兩個鍵，**沒有 `truncated` 或總列數提示**，
呼叫端無從得知自己拿到的是截斷結果。本端點最主要的趨勢指標 `alias_hit_rate`
是個比率，分母被無聲截斷正好讓它失真——而該欄位的註解自己寫著
「**比率在這裡才算，不存進表**：分母隨『要看哪個區間』而變」，
現在的實作等於讓分母由 `limit` 決定而不是由 `days` 決定。

**修法方向（擇一，待決策）**：

* **A**：`summary` 改用**未套 `limit` 的 SQL aggregate**（`COUNT` / `SUM` 走
  `WHERE` 但不走 `LIMIT`），`limit` 只限制明細 `rows`。語意與文件一致，需多一次查詢。
* **B**：維持單次查詢，但回傳 `total_rows` / `truncated`，並把文件與欄位註解
  明確改成「**本批次**聚合」而不是「區間聚合」。

⚠️ 兩個方向都會改 API response 的語意或形狀，屬 contract 修改，實作前要先寫計畫書。

---

### T-052：定期對 watchlist 產生 SR zone 分析（分析排程）

| 欄位 | 內容 |
|---|---|
| 狀態 | **review 不通過／待修復**（2026-08-31：兩個時段可以同時執行，違反文件寫明的序列與記憶體假設，見下方「review 發現」。上線與資料累積本身通過，見「上線實績」） |
| 優先度 | 高（同時是 T-049 的硬前置與三筆驗證缺口的唯一解） |
| 分類 | Go / 排程 / SR Zone / 驗證母體 |
| 建立日期 | 2026-08-20 |
| 來源 | T-048 全案 review。T-049 已列它為前置，但**先前沒有任何 todo 在追**，只散見於 T-045 與 T-049 的討論段落 |
| 關聯 | [`issue.md`](./issue.md) I-074（Lifecycle Engine RR 解耦的 replay 驗證無法執行）、I-078（身分層兩條路徑從未被執行）；T-049 的前置條件② |

#### 上線實績（2026-08-28 核實）

⚠️ **不要再從 `backend/config.yaml` 的 `sr_analysis.enabled: false` 推論它沒開。**
那是 repo 預設值；live 由環境變數覆寫，實際狀態要查容器：

```bash
docker inspect stock_trading-backend-1 --format '{{range .Config.Env}}{{println .}}{{end}}' | grep SR_ANALYSIS
# SR_ANALYSIS_ENABLED=true
```

（`candle_gap_detection` 同一個模式——config 預設 false，live 靠 env 開，見 T-067。）

**排程執行**（`job_runs`）：

| job | 輪數 | 狀態 | 每輪檔數 |
|---|---|---|---|
| `sr_analysis`（17:00） | 4 | 全 `success` | 11 |
| `sr_analysis_chip`（22:00） | 3 | 全 `success` | 11 |

**資料累積**（`stock_sr_zone_analyses`）：

| 日期 | 分析筆數 | 檔數 |
|---|---|---|
| 2026-08-21 / 24 / 25 / 26 / 27 | 各 22 | 各 11 |
| 2026-08-28 | 11（22:00 那輪尚未跑） | 11 |

⚠️ **`job_runs` 只涵蓋 2026-08-25 起，那是該表的保留範圍、不是 sr_analysis 特有**——
表裡**每一個** job 的最早紀錄都是 08-25。分析資料本身從 08-21 就已經是每日 22 筆，
所以排程實際上線的時間早於 `job_runs` 能證明的範圍。

**對下游的影響**：母體已從「4 檔 / 20 次」長到 144 次分析（2026-08-28 核實），
I-074 / I-078 / T-049 的「母體太小」那一項前置**已經解除**。
⚠️ 但 T-049 與 T-066 另有 **model bundle** 的前置沒解（`model_available: false`），
那與本筆無關，不要把「母體夠了」誤讀成「replay 可以跑了」。

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

#### 計畫書（**已實作；2026-08-31 review 不通過**——見上方「review 發現」，2026-08-20）

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

老化單位修好之後（2026-08-20 已上線），同時段重跑不再影響老化，所以這個守衛只是省一次
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
| ~~老化單位未上線就開排程~~，排程與人工同日各跑一次，老化一天前進 2 | **已解除**：老化單位修法於 2026-08-20 上線（原記於 `issue.md` I-077，已收斂） |
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
dev 那份，等於正式環境根本開不起來。**前置：老化單位修法必須先在 live 生效——2026-08-20 已滿足。**

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

**老化單位改為 K 棒推進必須先在 live 生效——2026-08-20 已滿足**（原記於 `issue.md` I-077，已收斂）。理由見上方風險表第一列。

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
| 上線前 | ~~老化單位修法在 live 生效~~ **已於 2026-08-20 滿足** | 見上方計畫書的「前置」與風險表第一列 |
| 與本筆並行 | ~~Event Timeline 改讀身分層~~ **已於 2026-08-20 完成**（原記於 `todo.md` T-051，已收斂） | 零依賴，且是 T-048 唯一使用者看得到的成果 |
| 累積期**當中** | [T-050](#t-050身分追蹤的可觀測性把關聯決策計數從-log-升級成可查詢的-metric) | 要趕在累積期內上線才看得到 alias 命中率與撞頂比例的**趨勢**，事後補等於白等一輪 |
| 母體足夠後 | 並行比對 → I-074 關閉 → I-078／I-079 重新量測 | 純驗證，不寫功能 |
| 全綠後 | [T-049](#t-049market-state-與所有下游改讀同一套-state) | 動決策邏輯，另需計畫書 |


---

#### review 發現（2026-08-31）——**不通過：兩個時段可以同時跑**

`runSRAnalysis`（`scheduler.go:1362-1376`）對兩個時段用**兩個不同的 atomic guard**，
而且註解明講這是刻意的：

```go
jobName, guard := "sr_analysis", &s.srAnalysisRunning
if withChip {
    jobName, guard = "sr_analysis_chip", &s.srAnalysisChipRunning
}
// 併發守衛只擋同一輪自己；兩輪之間互不影響。
```

**但同一個函式上方的 doc comment 寫的是相反的假設**：

> **序列執行**：這台 host 只有 2GiB。逐檔的峰值等同使用者手動點一次分析；
> 真的撐不住時要降頻而不是加併發。

兩句話直接衝突。而手動觸發端點（`handler/scheduler.go:69`）可以分別帶或不帶
`with_chip=true`，所以**兩個請求可以同時對同一份 watchlist 跑分析**。後果有三個：

1. **並行兩個 Python scoring**——直接違反 2GiB host 的序列假設（見
   [`development-workflow.md`](./development-workflow.md) 的記憶體限制）。
2. **同一 symbol 的身分匹配與事件鏈寫入並行**——`zone_matcher` 與
   `event_instances` 的三段關聯決策不是為並行設計的。
3. **skip 判斷與分析落地之間沒有鎖**——兩輪都可能在對方寫入前通過
   「最新 K 棒不是今天就跳過」的判斷，於是同一根 K 棒被分析兩次。

⚠️ **cron 排程本身撞不到**（17:00 與 22:00 相隔五小時），所以 live 至今沒有實例；
可觸發的路徑是**手動端點**，以及未來任一輪耗時超過五小時。

**修法方向**：資料層的「兩個時段各自冪等」規則保留不動，
**執行層改成共用一個全域 guard／mutex**。
「允許不同時段各跑一次」不等於「允許兩輪同時跑」——現行實作把前者實作成了後者。

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

**相關**：`yahoo.rate_limit` 同時節流除權息逐檔同步這件事已寫進 `backend/config.yaml`
的 `yahoo:` 段與 [`architecture.md`](./architecture.md) 的公司行動同步節
（原記於 `issue.md` I-085，已收斂）。

---

### T-061：`sr_zone_verify` 的排程層驗收補不齊——verifier 是具體型別，無法 stub

| 欄位 | 內容 |
|---|---|
| 狀態 | 待規劃 |
| 優先度 | 低 |
| 分類 | Go / Scheduler / 測試 |
| 建立日期 | 2026-08-25 |
| 來源 | 收盤驗證窗口改成「時間窗口」那輪（2026-08-25 收斂）的 code review；**承接該輪計畫書「測試與驗證策略」第 2 點**，那一點當時未完成、也未計入該輪的完成範圍 |

`runSRZoneVerification` 取到清單之後會逐筆呼叫 `s.srZoneVerifier.Verify(...)`，
再把 `len(analyses)` 當成 `symbols_total` 寫進 `job_runs`。**這兩件事目前沒有測試。**

現有的 `TestSRZoneVerifyUsesConfiguredWindow` / `TestSRZoneVerifyFallsBackToDefaults` /
`TestSRZoneVerifyClampsMaxAnalysesToHardLimit` 都讓 stub 回**空清單**，只驗窗口參數
（`since` / `limit`）算得對不對，走不到迴圈。測試註解已寫明卡點：

```go
// analyses 回空，所以不會走到 Verify（srZoneVerifier 是具體型別、無法 stub）
```

`Scheduler.srZoneVerifier` 的型別是 `*analysis.SRZoneVerifier`（具體 struct），
沒有介面可以替換，所以測不到：

* **窗口內每一筆都真的被 `Verify` 到**（少驗一筆不會有任何跡象）。
* **`job_runs.symbols_total` 等於實際處理筆數**——這個數字正是
  [`architecture.md`](./architecture.md) 教人用來判斷「窗口有沒有被 `max_analyses`
  截斷」的依據，它本身沒被釘住。（`sr_zone_verify_devbench_test.go` 有斷言它，
  但那條要真的 postgres、預設 skip，不算日常保護。）
* **單筆 `Verify` 失敗只記錄、不中斷其他筆**（這條規則寫在函式註解裡，同樣沒有測試）。

**這段邏輯不是那輪改動的**：2026-08-25 把收盤驗證從「最近 N 筆」改成「最近 N 天」
那一輪，只換了清單來源（`List` → `ListRefsSince`）與 log 欄位，迴圈與 `finishRun`
一行都沒動（`git diff` 確認）。這是更早就存在的覆蓋缺口。

但那一輪的計畫書把「每一筆都被 `Verify`、`symbols_total` 等於實際處理筆數」列為
第 2 條驗收條件，是它自己承諾要驗、最後沒做到的。**本條目是把該承諾移交，不是註銷。**
（該輪已於 2026-08-25 收斂，現況規格見 [`architecture.md`](./architecture.md) 的
SR Zone Scoring 排程說明段。）

**可能作法（擇一，實作前要先確認）**：

* 在 `scheduler` 端定義最小介面（只含 `Verify`），`Scheduler` 欄位改吃介面，
  `main.go` 注入既有的 `*analysis.SRZoneVerifier`。比照 `SRAnalysisRunner` 的既有慣例。
* 或改用可跑的整合 fixture：真的建一份分析與 zone，跑完之後斷言 zone 的
  `status` / `broken_at` 有被更新。成本較高但連 `Verify` 的實際效果一起驗到。

**不做的理由 / 風險**：動 `Scheduler` 的相依型別屬於排程修改，照 CLAUDE.md 要先有
計畫書；而且 `sr_zone_verify` 是無條件註冊、每個交易日都會跑的 job，重構失手的代價
是整支排程靜默失效。優先度低是因為這段邏輯本身很久沒變過，不是因為它不重要。

---

### T-064：`structure_state` 中文標籤在前後端各定義一次，headline／badge 契約未定

| 欄位 | 內容 |
|---|---|
| 狀態 | 待規劃 |
| 優先度 | 低 |
| 分類 | 前端 / Python / SSOT 整理 |
| 建立日期 | 2026-08-26（**原為 `issue.md` I-097，2026-08-27 依分類移到本清單**——沒有證據造成錯誤呈現或交易語意改變，實質是 SSOT 整併與 dead-key 清理，屬維護性改善。**編號 I-097 不回收**） |
| 來源 | 0050 `2026-08-26` 分析內容的逐項核實 |

#### 現象

同一組 `structure_state` 的中文對照表**存在兩份**：

| key | Python `decision_engine.py:305` | 前端 `SRZones.svelte:596` |
|---|---|---|
| `NORMAL` | **`""`（空字串）** | **「結構正常」** |
| `SUPPORT_RECLAIM_CANDIDATE` | 支撐收復候選 | 支撐收復候選 |
| `SUPPORT_RECLAIM_CONFIRMED` | 支撐收復確認 | 支撐收復確認 |
| `SUPPORT_RECLAIM_INVALIDATED` | 支撐收復失效 | 支撐收復失效 |
| `BREAKDOWN` | 短線結構跌破 | 短線結構跌破 |
| `RECOVERY_CANDIDATE` | **沒有** | 短線測試支撐 |
| `RECOVERY` | **沒有** | 短線收回支撐 |
| `RECOVERY_INVALIDATED` | **沒有** | 短線結構轉弱 |

**兩份字典確實不同，但前一版直接稱為「已漂移」證據不足**（2026-08-27 再核實）：

1. `NORMAL=""` 用在 Python 組合 **Market Regime headline** 時省略冗字；前端
   badge 顯示「結構正常」。兩個位置目的不同，可能是刻意的摘要層次，不宜未定契約就判 bug。
2. 前端多三個 `RECOVERY_*` key，而目前 live `market_regime.structure_state` 的全歷史
   分佈確實是 0 筆；但刪除前仍要確認舊 API payload／fixture／保存報告的相容需求，
   不能只因 live 為 0 就直接稱為死 key。

#### 兩份各自流向不同的畫面位置

* Python 的 `structure_label` 進 **`market_regime.label` headline**
  （`:329～337` 組合 label），**不是** `confidence_explanation`；前一版資料流寫錯。
* 前端的 `structureStateText` 用在 **`market_regime.structure_state` 的 badge**
  （`SRZones.svelte:2154`）。

所以同一頁會同時看到 Python headline 與前端 badge。對 `NORMAL` 而言，headline 省略
「結構正常」、badge 明示它；這是兩層呈現，但目前沒有證據證明使用者會讀成互相矛盾。

#### 處理方向（**擇一，未定案**）

**A. 以 Python 為單一來源**：Python 額外輸出 `structure_state_label` 欄位，前端 badge 只搬值；
`market_regime.label` 仍可保留「NORMAL 時省略」的 headline 組合規則。

* 與既有慣例一致——`role_label` / `tier_label` / `market_bias_label` / `action_label`
  等都已經是「Python 產生標籤、前端只搬」的形狀。
* 需要處理 `NORMAL` 的空字串：要嘛給它一個實際的詞，要嘛明確定義「空字串＝不顯示」。

**B. 以前端為單一來源**：Python 不再組中文，說明文字改用結構化欄位由前端組裝。

* 改動面較大，且與上述既有慣例相反。

**傾向 A，但先補契約測試**：同一 state 的 label 只能在一處定義；headline 是否省略
NORMAL 是另一條 presentation rule，不要混成字典值。`RECOVERY_*` 三個 key 則先搜尋
歷史 payload／fixture 的消費情況，確認不需相容後再刪除。

#### 核實（2026-08-27）：相容性查核已結案，`RECOVERY_*` 可以安全刪除

上面要求的「先搜尋歷史 payload／fixture 的消費情況」已做完，三個方向都查了：

| 查核 | 結果 |
|---|---|
| `_structure_state()` 的**全部**回傳值（窮舉） | 只有 `BREAKDOWN` / `NORMAL` / `SUPPORT_RECLAIM_{CANDIDATE,CONFIRMED,INVALIDATED}` 五個——**產不出 `RECOVERY_*`** |
| 原始碼／fixture 的消費者 | 只有 `SRZones.svelte:598-600` 的**定義本身**（三個 key 分別在 598／599／600），以及 `backend/internal/ui/dist` 的**編譯產物**（同一份原始碼）。**沒有任何測試或 fixture 消費它們** |
| live 歷史 payload（**兩張表都查，2026-08-27 擷取**） | 見下方查詢與數字；**兩張表的 `RECOVERY_*` 都是零筆** |

**live 統計的出處與可重現查詢**（2026-08-27 擷取，**兩張表數字不同是正常的**——
並非每筆分析都會寫出 decision 列）：

```sql
-- 分析表（111 列）
SELECT decision_summary->'market_regime'->>'structure_state', count(*)
FROM stock_sr_zone_analyses GROUP BY 1;
--  NORMAL 55 / SUPPORT_RECLAIM_CANDIDATE 40 / SUPPORT_RECLAIM_CONFIRMED 16

-- 決策表（104 列）
SELECT decision_summary->'market_regime'->>'structure_state', count(*)
FROM stock_sr_decisions GROUP BY 1;
--  NORMAL 53 / SUPPORT_RECLAIM_CANDIDATE 37 / SUPPORT_RECLAIM_CONFIRMED 14
```

**兩張表的 `RECOVERY_CANDIDATE` / `RECOVERY` / `RECOVERY_INVALIDATED` 都是 0 筆。**
（原本只寫「全部 111 筆分析」而沒寫表名與欄位，導致無法重現——
review 時用 `stock_sr_decisions` 查得 104 筆，兩個數字其實都對。）

⚠️ **刪除前要確認的是「哪一個欄位」，不是「哪一個字串」**：Python 確實有
`short_term_regime = "RECOVERY"`（`decision_engine.py:315`），前端也另有
`shortTermRegimeText` 在處理它。**兩者名字像、欄位不同**——
grep 到 `"RECOVERY"` 不代表 `structureStateText` 的那三個 key 還活著。
這正是雙真相源最容易誤判的地方，也是本筆值得修的理由之一。

**順帶一提**：`BREAKDOWN` 與 `SUPPORT_RECLAIM_INVALIDATED` 這兩個**產得出來但至今沒出現過**
的值，其 label 兩邊是一致的，不在本筆的漂移範圍內。

---

### T-065：進場強度判定仍讀 setup RR，與 execution gate 反方向矛盾

| 欄位 | 內容 |
|---|---|
| 狀態 | 待規劃 |
| 優先度 | 中高（會產出**無法診斷**的 `BLOCKED`：兩層 gate 都通過、RR 11.0，卻禁止進場且沒有任何欄位說明理由） |
| 分類 | Python / SR Zone / 決策語意 / 資料契約 |
| 發現日期 | 2026-08-27 |
| 來源 | 原記於 `todo.md` T-055 第三輪 review 的 F1 裁決（T-055 已於 2026-08-27 收斂）——封頂做法只涵蓋單向，反向由本筆承接。現況說明見 [`sr-zone-scoring.md`](./sr-zone-scoring.md)「已知的單向性」 |

#### 成因：封頂是單調遞減運算

現行實作在 `secondary_gate` 不合格時把 `decision_derived_view.semantic_pipeline`
與 `final_entry_permission` **一起封頂**到 `PROBE_ALLOWED`。那解決的是
**高估**：`setup_rr ≥ 2.0 > executable_rr`，畫面說 Buy 但 gate 說不夠。

**封頂只能降不能升**，所以反方向完全碰不到：`executable_rr` 高於 `setup_rr` 時，
判定在更上游就提前 return 了，下游拿不到控制權。

#### 重現（2026-08-27，`_summary` 的 CONTINUATION fixture，support TIER_3、壓力放 122.0）

| `setup_rr` | `rr_gate` | `semantic` / `final` | `action` | `risk_notes` |
|---|---|---|---|---|
| 1.4 | `qualified=True` `secondary=True` **`actual_rr=11.0`** | **`BLOCKED`** | `Hold` | 「主交易區風險報酬比不足。」＋「Final Entry 禁止進場…」 |
| 1.7 | 同上 | **`BLOCKED`** | `Hold` | **只有**「Final Entry 禁止進場…」 |
| 2.8（對照組） | `actual_rr=2.429` | `ENTRY_ALLOWED` | `BuySmall` | 正常 |

`setup_rr=1.7` 那列最嚴重：**BLOCKED 但沒有任何欄位解釋為什麼**——
`rr_gate` 顯示 11.0 兩層皆過，`risk_notes` 也沒提 RR。使用者與 API 消費者都無從診斷。

#### 三處要改的上游提前 return

| 位置 | 現況 | 問題 |
|---|---|---|
| `decision_engine.py:519` | `if rr is None or rr < 1.5: return "WATCH"` | `rr` 是 setup RR。setup 1.4 / executable 11.0 → 直接 WATCH ＋「風險報酬比不足」 |
| `decision_engine.py:1042` | `elif not rr_qualified: entry_permission_state = "BLOCKED"` | `rr_gate` 是**更早的 setup-RR gate**。setup 1.7 < TIER_3 的 1.8 → BLOCKED |
| `decision_engine.py:527` | `strong` 的 `rr >= 2.0` | 同樣讀 setup RR，方向 B 會少發 Buy |

`constructive` 不看 rr，不在範圍內。

#### 真正的工作：解資料流的環

```
execution gate ──需要──> entry_executability ──需要──> decision_derived_view ──需要──> execution gate
```

`_execution_target` 要 `entry_price`（來自 `entry_executability`），而 derived view
只要 `rr_qualified` 一個布林。看起來可以兩階段（先用 setup gate 建 derived view 供
executability 用，算出 execution gate 後重建 derived view），**但重建時
`entry_executability` 不會跟著重算**——那正是殘留的來源。要真正一致，得處理這個定點問題
或重新設計相依方向。

⚠️ 這會同時動到 `_decision_action`、`_decision_semantic_pipeline`、`_entry_executability`
三個 authoritative 欄位的產生路徑，屬 CLAUDE.md 的**大規模／高影響異動**，
**實作前要先寫計畫書**，不能直接改。

#### 驗收準則

* 上表 `setup_rr=1.4` 與 `1.7` 兩組必須輸出與 `rr_gate` 一致的結果，
  且 `BLOCKED` 時必須有欄位指明理由。
* `setup_rr=2.8` 對照組行為不得改變。
* 既有的封頂測試（`test_full_entry_gate_caps_authoritative_permission_end_to_end`
  與其對照組）必須續存並通過——本筆是**擴充**單向為雙向，不是取代。

#### 沿革：本筆是怎麼分出來的

單向封頂原記於 `todo.md` T-055（已於 2026-08-27 收斂）。該筆的 F1 條目原本要求
「`strong` 與 `secondary_gate` 讀同一個 `actual_rr`」，那是**雙向**不變式的一種實作手段；
2026-08-27 裁決把範圍收在單向封頂，雙向一致轉由本筆承接。
**本筆不阻擋既有的單向封頂**——它已獨立驗收通過。

現況（含單向性的長期說明、實測形狀與端到端測試）在
[`sr-zone-scoring.md`](./sr-zone-scoring.md)「RR 語意分層／已知的單向性」。
**動手前先讀那一節**，本節只保留分家的理由。

---

### T-066：補跑 RR 語意分層（原 T-055）的 decision replay 前後比對

| 欄位 | 內容 |
|---|---|
| 狀態 | 待規劃（**前置未滿足：dev 沒有 model bundle**） |
| 優先度 | 中（RR 分層已上版控且行為有測試釘住，缺的是分佈影響的實測證據） |
| 分類 | 驗證 / Python / SR Zone / 決策語意 |
| 建立日期 | 2026-08-27 |
| 來源 | T-055 收斂時未完成的驗收第 3 項——**條目移除後這件事會沒有清單追蹤**（同 T-063 的教訓） |

#### 要驗什麼

T-055 把 executable RR 的 target 改成「entry 前方第一道壓力封頂」，預期讓 RR 普遍下修、
被擋掉的樣本變多。**這個幅度至今沒有量過**，只有單元／端到端測試證明行為正確。

比對 `POST /sr-scoring/evaluate`（`decision_replay=true`）的三個分佈：
`by_rr_gate` / `by_rr_gate_reason_code` / `by_entry_executability`。

#### ⚠️ 那 84 筆**不是**合格的 before

dev `stock_sr_decisions` 現有 84 筆確實是改動前的產物，分佈已擷取備查（見
[`sr-zone-scoring.md`](./sr-zone-scoring.md)「分佈影響尚未 decision replay 驗證」）。
**但它不能拿來跟新跑的 replay 直接相減**：

| 面向 | 那 84 筆 | 新跑的 replay |
|---|---|---|
| symbols | 當時累積下來的，未受控 | 本次指定的清單 |
| as-of dates | 當時的分析日 | 本次 replay 的取樣列 |
| 模型 | 當時的 bundle（或根本沒有） | 本次新訓練的 bundle |
| 設定 | 當時的 `replay_max_rows` 等 | 本次的參數 |

四項都不同，**樣本差異會被誤讀成 T-055 的因果影響**——而那正是這一筆要量的東西。
所以那 84 筆只能當**描述性參考**（「改動前大致長這樣」），
**不得寫成前後因果比較**，也不得用它算「被擋掉的樣本多了幾 %」。

⚠️ **更根本的一點：`stock_sr_decisions` 根本不是 replay cohort 的來源。**
decision replay 是拿 OHLCV **重算**決策（見下一節），不讀既有的 `stock_sr_decisions`。
所以那 84 筆與新 replay 連「同一個母體的前後兩次觀察」都談不上——
它們是兩組不同東西，不只是取樣不同。

#### 唯一合格的做法：同一 cohort 跑兩次

**先搞清楚 cohort 是什麼決定的**：`_decision_replay_rows`（`evaluation.py:1393`）的 as-of
完全來自傳進去的 OHLCV DataFrame——`_candidate_bar_range` 取
`[min_history_bars, len(df) - forward_bars - 1]`，再由
`window_start = max(first_idx, last_idx - quota + 1)` 取**尾端** quota 根。
**window 錨在資料尾端**，所以來源多一根 K 棒，整段 as-of 就位移。

before 與 after 必須是**同一組輸入**，只差在程式碼版本：

| 必須固定 | 為什麼 | 怎麼核對 |
|---|---|---|
| **OHLCV source snapshot**（含 `symbols`） | as-of 是 df 尾端；多一根 K 棒 window 就位移 | `replay_plan` 逐檔比 `candle_count` / `candidate_bars` / `start_as_of` / `end_as_of` |
| **`dataset_config`**：`min_history_bars`、`forward_bars_support` / `forward_bars_resistance` | 直接決定 `_candidate_bar_range` 的頭尾 | `replay_plan` 已帶 `min_history_bars` / `forward_bars` |
| **`replay_max_rows`** | 決定 `quota_by_symbol` → `window_start`（見「Decision Replay 的取樣規則」） | `replay_coverage.quota_by_symbol` |
| **chip-score ／ model-governance context snapshot** | `_load_db_replay_chip_context` / `_load_db_replay_model_governance_context` 依 `[dataset_from, dataset_to]` **當下從 DB 撈**，DB 內容變了輸入就變 | `outcome_summary` 的 `rows_with_non_missing_chip` / `rows_with_model_governance` |
| **model bundle** | 同一個 `model_path`，**不得各訓練一次** | `model_metadata` |
| **`builder_config`** | zone 建構參數會改變 zone 本身 | 兩次設定相同 |

⚠️ **凍結來源資料是前置動作，不是注意事項**：兩次 replay 之間
**不得再匯入或補任何 K 棒**（含 `daily_close`、`evaluation_universe_sync`），
也不得重跑籌碼或模型治理同步。做不到就先把 dev 的相關排程停掉。

**before 怎麼取得**：T-055 的實作 commit 的 **parent** 就是乾淨的改動前基準，
用 `git worktree` 取出該 commit 跑一次即可，**不需要手動回退檔案**。

⚠️ **若最後無法重跑舊版**（例如環境湊不出同一個 bundle），本筆的產出就只能標成
**描述性參考**，並在 `sr-zone-scoring.md` 明寫「未經因果驗證」——
**不得**在任何文件裡把它敘述成 T-055 的前後影響。

#### 為什麼現在跑不了（2026-08-27 實測）

端點成功回應、產出 200 列 replay rows，但：

```
decision_replay_available: false
decision_fields_available: false
model_available:           false      ← 根因
by_rr_gate / by_rr_gate_reason_code / by_entry_executability：不存在
```

`evaluation.py` 的 `if bundle is not None` 讓整段 `build_decision_summary(...)` 從未被執行，
`rr_gate` / `entry_executability` / `final_entry_state` 全是 `None`。
與 [`issue.md`](./issue.md) **I-074** 同一類阻塞（那筆缺 production 分析資料，這裡缺 dev 模型）。

#### 前置步驟

1. 在 dev 跑 `POST /sr-scoring/train` 產生 model bundle。
2. ⚠️ **需要一個記憶體窗口，且需先徵得同意**：本機 host 只有 2GiB，起 dev python-server
   之後只剩約 400MB available，LightGBM 訓練會再往上衝。依
   [`development-workflow.md`](./development-workflow.md)「本機同時只留一組 stack」，
   **應先停掉 live stack 再做**——停 live 不能自作主張。
3. 訓練完再跑一次 evaluate，比對三個分佈。

#### 完成條件

1. **同一 replay cohort 跑兩次**（上表六項輸入全部固定，只差程式碼版本），
   三個分佈都有實測數字。
2. 報告要保留**兩次**的 `replay_plan`、`replay_coverage.quota_by_symbol`，以及
   `rows_with_non_missing_chip` / `rows_with_model_governance` 兩個 context 覆蓋計數，
   並**逐檔核對相同**——這是 cohort 一致的證據。**只貼分佈數字不算完成。**
3. 結果寫進 [`sr-zone-scoring.md`](./sr-zone-scoring.md)「分佈影響尚未 decision replay 驗證」
   那一節，改寫成實測結論並移除「尚未驗證」的措辭。
4. 若走了上面的降級路徑（無法重跑舊版），第 3 步改成寫入**描述性參考 ＋ 未經因果驗證**的
   說明，且本筆**不視為完成**，狀態改為「擱置」並記下缺什麼。

**驗收一律走 dev compose**，不得用 live 做測試資料。

---

### T-067：日 K 缺漏偵測的驗收——造缺口實測與 live 運作觀察

| 欄位 | 內容 |
|---|---|
| 狀態 | **A 已完成（2026-08-28），B 觀察中**——四項情境全部有實測結果，結果已歸檔到 [`architecture.md`](./architecture.md)「日 K 缺漏偵測 › 驗收實測（2026-08-28）」；B 還差兩個交易日（下週一、二）。**本筆保留供 review 與 B 收尾。** |
| 優先度 | 中（**基本正向與負向路徑已驗證**：正向以 dev 造一個人工缺口驗到 `upstream_data_gap`，負向以 `2867` 的停止買賣在 live 與 dev 各驗一次判 `verified` 不告警。**這不等於誤報漏報已全面排除**——live 端正向、`deferred`、陳舊升級、breaker 四條路徑都還沒被觸發，見下方「B 的進度」） |
| 分類 | 驗證 / Go / 排程 / 市場資料 |
| 建立日期 | 2026-08-28 |
| 來源 | `issue.md` I-091 收斂時未完成的驗收——**條目移除後這件事會沒有清單追蹤**（同 T-063 的教訓） |

#### 背景

`candle_gap_detection` 的實作、測試與文件歸檔都完成了（現況規格見
[`architecture.md`](./architecture.md)「日 K 缺漏偵測」），並於 **2026-08-28 14:37** 隨
`CANDLE_GAP_DETECTION_ENABLED=true` 上線。

> 📌 **原始的驗收步驟原文保留供 review 對照，未改寫。**
> 其中**帶日期標註的段落（「2026-08-28 補」「（驗收前基線）」等）是執行後才加的**——
> 執行中發現原文缺了什麼，就地補在該步驟旁邊，比另開一節更好用。
> **整體現況以「A 的實作結果」與「B 的進度」兩節為準**，那兩節在本節之後。

**（驗收前基線）** 當時它的正確性只有單元測試層級的證據：

* 沒有跑過任何一輪（它跟著 `evaluation_universe_sync` 在平日 16:00 執行）；
* **從來沒有用真實的缺口驗證過它會不會報**。

⚠️ **`2867` 只能驗一半**（已向 TWSE 三層查證：它是合法的停止買賣，沒有任何資料遺失）：

* **不能驗「正向缺口偵測」**——它沒有真的漏資料，在這個設計下本來就不該告警，
  而且它不會復牌。要驗這一半得**自己造缺口**（下方 A 的第 1、2 步）。
* **可以當「休市／停止買賣不誤報」的天然負向案例**——它的缺列是真實的、
  交易所那邊也真的沒有成交（下方 A 的第 4 步）。

兩半都要驗：只驗正向會漏掉誤報，只驗負向會漏掉漏報。

#### A. 造缺口實測（dev compose）

**必須走 dev**，不得在 live 刪資料。

##### 前置：dev 不保證有可用的資料

`docker-compose.dev.yml` 用的是**獨立的資料卷**，與 live 完全不共用。所以不能假設
dev 裡有池成員、證券主檔或足夠的歷史 K 棒——**沒有這些，偵測會早退或算出空的候選集合，
而那看起來會像「一切正常」**。

開始前先確認四件事，缺哪一項就補哪一項（**第四項是 2026-08-28 補的**，見表格下方）：

| 檢查 | 查法 | 不足時 |
|---|---|---|
| 池裡有成員 | `SELECT COUNT(*) FROM evaluation_universe WHERE active;` | 從前端「評估標的池 → ③ 已入池」匯入 selection report；**空池時偵測記 `success` / `total=0`，什麼都驗不到** |
| 主檔有那幾檔且 `is_listed` | `SELECT symbol, is_listed, market FROM stock_symbols WHERE symbol IN (…);` | 跑一次 `stock_symbol_sync`（前端排程頁可手動觸發）。**查無主檔的標的會 fail-open 保留但沒有 `market`**，偵測會把它落成 `verification_unavailable` |
| 視窗內有足夠日 K | `SELECT symbol, COUNT(*) FROM candles WHERE timeframe='1d' AND ts >= now() - interval '20 days' GROUP BY 1;` | 手動觸發 `evaluation_universe_sync` 回補（FinMind 5 req/min，135 檔約 26 分鐘；**只驗少數幾檔的話先把池縮小**） |
| **dev image 夠不夠新** | `docker exec <dev-pg> psql -tAc "SELECT max(version_id) FROM goose_db_version;"` 對照 `backend/internal/database/migrations/postgres/` 的最大編號 | **重建 image**（`docker compose -f docker-compose.dev.yml build backend`），不是只重建容器 |

⛔ **第四項是 2026-08-28 實作時補的，原本沒有，而它的失敗模式最隱蔽。**
migration 是 **embed 進 binary** 的，所以 image 的新舊決定了 schema 的新舊：
當天 dev image 建於 08-25 17:28，而 `074_candle_verification_state.sql` 是 08-28 14:22 才加入，
於是 `candle_verification_state` **整張表不存在**。這件事**三個地方都不會報**：

* backend 照樣記 `migrations applied version=73` ——對舊 binary 而言 73 確實是最新，不是錯誤；
* 偵測的**四項依賴檢查照樣通過**（repo 物件非 nil，nil 檢查看不出表在不在）；
* 原本的三項前置（池成員／主檔／日 K）**沒有任何一項會碰到它**。

**但實際執行時會明確降級，不會假裝正常**：`LoadStates` 炸在 `relation does not exist` 之後，
`candle_gap_detection.go:203` 會記 `verification_state_read_failed` 並設 `degraded = true`，
該輪收在 **`partial`**。所以危險不在於「結果看起來正常」，而在於
**啟動與前置檢查都不會告訴你原因**——你會拿到一個 `partial`，然後從三項前置查不出所以然。

**`docker compose up -d` 只重建容器、不重建 image**——T-067 原文只寫了「重建 backend 容器」，
那對環境變數夠、對 migration 不夠。

⚠️ **`lookback_trading_days` 預設 10**，所以視窗內至少要有 10 個交易日的資料才驗得出
「中段缺口」與「完整標的」的差別。資料太少時整池都會是候選，變成 aggregate 短路，
驗不到逐檔那條路徑。

##### 前置：dev 的**兩個開關**預設都是關的

`docker-compose.dev.yml` 裡 `EVALUATION_UNIVERSE_ENABLED` 與
`CANDLE_GAP_DETECTION_ENABLED` **都**預設 `false`。

⚠️ **兩個開關是巢狀的，不是並列的**：偵測的註冊寫在 parent 的
`if evaluationUniverse != nil && Enabled` 區塊裡（`scheduler.go`），所以

* **parent 沒啟用 → 偵測不會被註冊，而且不會有任何 Error log**。
  「candle gap detection 已啟用但依賴不齊，不註冊」那條**只在 parent 已註冊、
  偵測自身 enabled、但四項依賴缺一時**才出現——只開偵測那個開關的話，
  照那條訊息去排錯會一無所獲。
* 未啟用時 `runCandleGapDetection` 在第一行就早退，**不執行也不寫任何 `job_runs`**——
  照步驟跑會什麼都沒發生而且**沒有痕跡**，很容易被誤讀成「沒有缺口」。

1. **兩個都設 `true`**：`EVALUATION_UNIVERSE_ENABLED=true` ＋
   `CANDLE_GAP_DETECTION_ENABLED=true`。
2. **重建 backend 容器**——環境變數只在容器建立時帶入，改了設定不重建等於沒改。
3. 確認 `/scheduler/status` 的 `candle_gap_detection` **不是 `disabled`**。
   （**不要要求它是 `never_run`**——dev 若已有歷史執行紀錄，狀態會是上一輪的結果。）
   仍是 `disabled` 時依序看：parent 有沒有註冊 → 偵測的 `enabled` 有沒有生效 →
   啟動 log 有沒有「依賴不齊」的 Error。

##### 步驟

**上面兩節的前置都確認過再開始**——資料不足或偵測未啟用時，下面每一步的結果都不能採信。

⚠️ **這個流程會改動 dev 的持久化資料**（刪真實 K 棒、寫 verification state、可能改池成員），
而 dev 用的是**具名 volume，不會自己還原**。**開始前先做 0. 備份，結束後一定要跑「還原」那一節。**

0. **決定測試池要留哪幾檔，再備份要動的東西。**

   **建議把池縮成三檔**：`<目標>`、`<對照>`、`<負向案例>`。
   ⚠️ **不要縮成只剩 `<目標>`**——第 3、4 步需要另外兩檔**在池裡**才會被偵測碰到；
   池裡只剩一檔的話，第 3 步的「對照組不出現在告警裡」會變成**永遠會過的空集合測試**，
   第 4 步的負向案例也根本不會產生 `verified` state。

   縮池的目的是**把回補從 26 分鐘縮到幾秒**，不是為了讓 `skipped` 變成某個數字
   （那個數字不可靠，見第 2 步）。

   ✅ **首選：三檔都從「原本就 `active`」的成員裡挑。** 這樣測試期間只會發生
   「停用其他成員」一種變更，還原時把它們設回 `true` 就完整了。
   ⚠️ **`<負向案例>` 最容易破這條**——dev 的池不一定有 `2867` 這類停止買賣的標的。
   真的湊不出來、必須啟用一檔原本 inactive 的成員或新匯入一檔時，
   **備份 (c) 與還原都要用下面的完整版**，否則還原不回去。

   三檔各自要滿足的前提**不一樣**，不要以為都要有當日 K：

   | 標的 | 前提 |
   |---|---|
   | `<目標>` | **必須有當日 K**——才會被跳過，人工缺口才不會在偵測前被補回 |
   | `<對照>` | 視窗內完整即可；有沒有當日 K 都不影響（它本來就不會成為候選） |
   | `<負向案例>` | 本來就沒有那些日子的 K——**那正是要驗的形狀**，不要幫它補資料 |

   接著備份三份，缺一份就有東西還原不了：

   ```sql
   -- (a) 要刪掉的那根中段 K 棒：把整列存下來（最保險的一份）
   --     ⛔ 日期比對一律用台北日，理由與寫法見下方第 1 步的 ⚠️
   SELECT * FROM candles
   WHERE symbol = '<目標>' AND timeframe = '1d'
     AND (ts AT TIME ZONE 'Asia/Taipei')::date = DATE '<中段日期>';

   -- (b) 測試涉及標的的既有 verification state（可能本來就有）
   --     **三檔都要**：負向案例那一檔在第 4 步會被寫成 verified，漏了就還原不了
   SELECT * FROM candle_verification_state
   WHERE symbol IN ('<目標>', '<對照>', '<負向案例>');

   -- (c) 池的完整快照。⛔ **不要只記 active 的那些**——測試期間可能會「啟用」
   --     原本 inactive 的成員，只記 active 清單的話還原時不會把它設回 false；
   --     新匯入的 row 也認不出來（備份裡沒有 = 測試期間新增的）。
   SELECT symbol, active FROM evaluation_universe ORDER BY symbol;
   ```

1. 在 dev 挑一檔池內標的，**而且那一檔必須已經有「今天」的日 K**（理由見下方 ⚠️），
   刪掉它**視窗中段**的一根日 K（不是最新那根——尾端缺口會自癒，
   中段才是 T-062 跳過最佳化之後的真正盲點）。

   ```sql
   DELETE FROM candles
   WHERE symbol = '<目標>' AND timeframe = '1d'
     AND (ts AT TIME ZONE 'Asia/Taipei')::date = DATE '<中段日期>';
   ```

   ⛔ **不要寫成 `ts = '<中段日期>'`**。`ts` 是 timestamptz，右側的日期字面值會用
   **session timezone**（dev 是 UTC）解讀成 `00:00+00`，而日 K 存的是**台北 00:00**
   ＝前一日 `16:00+00`——**刪不到任何東西，而你會以為已經刪了**。
   備份、刪除、還原三處**用同一個 predicate**，日界線才不會各自為政。

   ⚠️ **目標標的一定要落在 `skipped` 那批，否則正向驗收必然失敗。**
   `runEvaluationUniverseSync` 的順序是
   `dropSymbolsSyncedToday` → `BackfillHistory(days=10)` → **偵測**，
   而 `BackfillHistory` 抓的是 `today-10 ~ today` 且走 upsert——
   **沒被跳過的標的，人工刪掉的中段 K 棒會在偵測跑到之前先被補回來**，
   偵測因此看不到缺口。那不是實作壞了，是驗收步驟設計錯了。

   跳過的條件是「該檔有當日日 K」。**執行前直接查那一檔**，不要事後看 `skipped` 數字：

   ```sql
   SELECT symbol, MAX(ts) FROM candles
   WHERE symbol = '<目標>' AND timeframe = '1d'
     AND (ts AT TIME ZONE 'Asia/Taipei')::date = (now() AT TIME ZONE 'Asia/Taipei')::date
   GROUP BY symbol;
   ```

   ⛔ **不要寫成 `ts >= (now() AT TIME ZONE 'Asia/Taipei')::date`**。`ts` 是 timestamptz，
   右側的 `date` 會用 **session timezone** 轉回時間；dev postgres 的 `TimeZone` 是 **UTC**，
   於是右側變成 `2026-08-28 00:00+00`——而系統把日 K 存成**台北 00:00**
   （`market/finmind.go`），也就是 `2026-08-27 16:00+00`。兩者差 8 小時，
   **真的存在的當日 K 一筆都比對不到**。

   2026-08-28 在 live 實測：`ts >= …::date` 命中 **0** 筆，
   `(ts AT TIME ZONE 'Asia/Taipei')::date = …` 命中 **9** 筆。
   後果不對稱——誤判成「查無」會讓你走進 fixture 分支，替一檔**本來就有**當日 K 的標的
   插一根假資料，忘了清就會讓之後每一輪都跳過它。

   （跳過判定本身用的是 `timeutil.TodayTaipei()` 的台北日界，見 `scheduler.go`
   的 `dropSymbolsSyncedToday`。這條查詢要對齊的就是它。）

   查得到就會被跳過。查不到的兩條路：

   * **正解——在交易日、上游資料發布後執行**（日 K 約 15:00 後才有），
     先手動觸發一次 `evaluation_universe_sync` 讓它抓到當日 K。
   * **退路——非交易日只能用人工 fixture**。⚠️ **非交易日回補拿不到當日 K**
     （上游根本沒有那天的資料），所以那時**只能自己插一根**：

     ```sql
     -- 純粹為了觸發「今天已抓過」的跳過機制，不是真實行情。
     -- ts 要寫成**台北當日 00:00 的那個時間點**，與系統存日 K 的方式一致。
     INSERT INTO candles (symbol, timeframe, open, high, low, close, volume, amount, ts)
     VALUES ('<目標>', '1d', 1, 1, 1, 1, 1, 1,
             (((now() AT TIME ZONE 'Asia/Taipei')::date)::timestamp AT TIME ZONE 'Asia/Taipei'))
     ON CONFLICT (symbol, timeframe, ts) DO NOTHING;
     ```

     ⛔ **`::timestamp` 不能省。** 少了它，`date` 會被隱式轉成 **timestamptz**，
     `AT TIME ZONE` 就變成**反向**轉換，算出來的是台北當日 **16:00** 而不是 00:00。
     2026-08-28 實測：`((…)::date) AT TIME ZONE 'Asia/Taipei'` → `2026-08-28 08:00`，
     加了 `::timestamp` → `2026-08-27 16:00+00`，**後者才等於真實日 K 的存法**。
     插錯時間點的 fixture 觸發不了跳過，而且下面那條 DELETE 也刪不到它。

     **驗完一定要刪掉**——留著會讓之後每一輪都跳過它：

     ```sql
     DELETE FROM candles
     WHERE symbol = '<目標>' AND timeframe = '1d'
       AND (ts AT TIME ZONE 'Asia/Taipei')::date = (now() AT TIME ZONE 'Asia/Taipei')::date;
     ```

     **兩段都用同一種台北日期表示**——同一份流程裡混用兩種日界線，遲早會插進去一根
     刪不掉的假 K 棒。
2. 手動觸發 `evaluation_universe_sync`，再看偵測有沒有把那一天報成 `upstream_data_gap`。

   ⚠️ **不要拿完成 log 的 `skipped` 當證據**：那行只記**數量**不記 symbol
   （`evaluation universe sync done`），別的標的被跳過也會讓 `skipped > 0`，
   證明不了目標那一檔。**目標有沒有被跳過，一律看第 1 步那條逐檔查詢**——
   `skipped` 這個數字在任何池大小下都不可靠，不要試圖用它推論。
3. **對照組（完整標的）**：另挑一檔不動它，確認它**不出現在告警裡**。
   ⚠️ **不要期待它產生 `verified` state**——完整標的沒有缺漏日期，就不會進候選集合，
   `verifyCandidates` 根本不會碰到它。那是設計如此，不是漏寫。
4. **反向驗證（無成交，天然負向案例）**：**不要自己造**——交易所那天沒成交，
   我們本來就沒有那根 K 棒可刪。用 **`2867` 在 2026-08-20 之後的日期**：
   那些日子日曆說是交易日、我們沒有 K 棒，所以它**會**成為候選；
   核對後交易所那邊也沒有成交，應判 **`verified` 且不告警**。
   這證明的是「合法停止買賣不得誤報」——與第 1、2 步的正向驗證合起來才算完整。
   ⚠️ **dev 不一定有 `2867` 的資料**（見上方前置）。沒有的話改用其他有停止買賣紀錄的
   標的，或**標為無法執行**——不要為了湊這一步硬造一筆假的停止買賣。

**要記錄的是「報了什麼」而不只是「有沒有報」**：`job_runs.error` 的前綴要是
`upstream_data_gap`，不是 `verification_unavailable`——後者代表根本沒驗成，
那不算驗證通過。

⚠️ **需要記憶體窗口**：這台 host 只有 2GiB，起 dev stack 前**應先停掉 live stack**
（見 [`development-workflow.md`](./development-workflow.md)「本機同時只留一組 stack」）。
那是需要你同意的動作，不能自作主張。

##### 還原（**驗完必做**）

dev 是持久化 volume，上面每一項人工變更都會留著。**不還原的後果不是「資料髒了」而是
「之後每一輪都產生假告警」**——而誤報訓練使用者忽略告警，正是本筆自己列的第一條風險。

**1. 還原中段 K 棒——優先走自然路徑**

先讓目標**不再被跳過**（刪掉當日 K，若當初插了 fixture 就是刪它），再觸發一次
`evaluation_universe_sync`：`BackfillHistory` 抓 10 天視窗且走 upsert，會把中段那根補回來。

```sql
-- 補完確認日期集合完整（比對備份 (a) 的那一天有沒有回來）
SELECT ts FROM candles
WHERE symbol = '<目標>' AND timeframe = '1d'
  AND ts >= now() - interval '20 days'
ORDER BY ts;
```

上游拿不到時（非交易日、標的已停止買賣）才用備份 (a) 的值手動 `INSERT` 還原。

**2. 清掉人工產生的 verification state**

```sql
DELETE FROM candle_verification_state
WHERE symbol IN ('<目標>', '<對照>', '<負向案例>');
```

**三檔都要**——負向案例那一檔在第 4 步會被寫成 `verified`，漏掉它等於留下一筆人工 state。

那是純簿記，刪掉之後下一輪會把它們當成**首次出現**——正是我們要的乾淨起點。
備份 (b) 若顯示它們**本來就有** state，改成把原值寫回去，不要留空。

**3. 恢復測試池——兩個方向都要，還要處理新增的 row**

⛔ **不是「把備份裡的成員設回 true」就好**。測試期間可能發生三種變更，
只還原第一種會讓池悄悄變成另一個樣子：

| 測試期間做了什麼 | 還原要做什麼 |
|---|---|
| 停用其他成員（縮池） | 設回 `active = true` |
| **啟用**原本 inactive 的成員 | **設回 `active = false`** |
| **新匯入**一檔（備份裡沒有） | **刪掉那一列** |

```sql
-- (1) 依備份逐檔寫回原值，true / false 都要。
--     把備份 (c) 的內容填成 VALUES 清單。
UPDATE evaluation_universe AS e
SET active = b.active
FROM (VALUES ('<symbol1>', true), ('<symbol2>', false) /* …備份 (c) 全部… */)
     AS b(symbol, active)
WHERE e.symbol = b.symbol;

-- (2) 刪掉測試期間新增的列（備份 (c) 裡沒有的就是新增的）
DELETE FROM evaluation_universe
WHERE symbol NOT IN (/* …備份 (c) 的全部 symbol… */);
```

⚠️ **(2) 執行前先用 `SELECT` 看一次結果**——備份清單貼漏一個，這條會把真實成員刪掉。

**4. 收尾檢查——逐項比對備份，不要看整輪狀態**

驗收結束時 dev 應該回到：**沒有任何人工缺口、沒有任何人工 state、池成員與縮池前相同**。
逐項確認：

| 檢查 | 判準 |
|---|---|
| 人工刪掉的那根 K 棒 | 用備份 (a) 的 `(symbol, 台北日期)` 查得回來 |
| 三檔的 verification state | 與備份 (b) **逐檔相同**——本來沒有的要沒有，本來有的要是原值 |
| 池 | **整張表的 `(symbol, active)` 集合等於備份 (c)**——只比對 active 集合看不出多留下來的 inactive 列 |

⛔ **不要用「再跑一次應該是 `success`」當判準**。dev 本來就可能有**其他真實的缺口或
驗證不可用**（歷史資料不完整、上游當初就沒給），那時整輪本來就會是 `partial`。
拿它當還原乾淨的標準，會反過來訓練人把真實缺口當成「自己沒清乾淨」而去刪資料——
方向完全相反。

整輪的 `success` / `partial` **只當輔助觀察**：若出現 `upstream_data_gap`，
先確認它報的是不是**你動過的那個 (symbol, 日期)**，不是就與本次驗收無關。

#### B. live 首輪運作觀察

第一輪會在**上線後的第一個平日 16:00** 跟著池同步跑（parent 約 25 分鐘，所以結果約 16:25）。

```bash
docker exec postgres-postgres-1 psql -U trading_username -d trading -c \
  "SELECT job_name, status, symbols_total, symbols_failed, started_at, error
   FROM job_runs WHERE job_name='candle_gap_detection' ORDER BY id DESC LIMIT 3;"

docker logs stock_trading-backend-1 2>&1 | grep 'candle gap detection done'

docker exec postgres-postgres-1 psql -U trading_username -d trading -c \
  "SELECT last_result, COUNT(*) FROM candle_verification_state GROUP BY 1;"
```

要確認的：

| 觀察 | 判讀 |
|---|---|
| `/scheduler/status` 顯示 `candle_gap_detection` | 應為 `never_run` 而**不是 `disabled`**；顯示 `disabled` 代表四項依賴有缺，看啟動 log |
| 首輪 `status` | `success` 代表沒有候選缺口；`partial` 要**看 `error` 前綴**分辨是真缺口還是驗不了 |
| `candle_verification_state` 有沒有列 | 全空代表根本沒有候選缺口（正常），或偵測沒跑到 |
| 對交易所的請求量 | 確認沒有異常暴增（aggregate 短路 ＋ cap 20 ＋ 500ms 間隔應該壓得住） |

⚠️ **首輪若是 `partial`，在 A 完成之前你分不出是真的抓到缺口還是實作有問題**——
這正是 A 要先做的理由。

#### 完成條件

A 的四項情境都有實測結果、B 至少觀察三個交易日且請求量正常，並把結論補進
[`architecture.md`](./architecture.md)「日 K 缺漏偵測」那一節。

**進度（2026-08-28）**：A 四項已完成，結論已寫進該節的「驗收實測（2026-08-28）」子節；
**B 只完成第一個交易日**，還差兩個（見下方「B 的進度」）。所以本筆**尚未達成完成條件**。

**（已失效，2026-08-28）** 原文警語是「在 A 完成之前，不要把 live 的任何 `partial`
當成已證實的上游缺漏」。A 已完成，該限制解除：live 若出現 `partial`，照
`architecture.md`「三種結論必須分得開」用 `error` 前綴判讀即可。

#### 順帶未補的一條測試

原計畫的測試矩陣第 24 條要求「`StatesBySymbols` 整體失敗時**一次斷言**回補與偵測兩邊的
收斂」。目前是**分成兩支**測的（`TestEvaluationUniverseSyncFallsBackToFullPoolWhenMasterLookupFails`
與 `TestCandleGapDetectionUnavailableWhenSymbolStatesFail`），兩邊的行為都有守到，
但**分開測會讓「其中一邊忘了處理」看不出來**——那正是原文要求合成一支的理由。
做 A 的時候順手補一支端到端的。

✅ **已補（2026-08-28）**：`TestEvaluationUniverseSyncAndGapDetectionDivergeWhenMasterLookupFails`
（`backend/internal/scheduler/candle_gap_detection_test.go`）。它跑的是
`runEvaluationUniverseSync` 而不是 `runCandleGapDetection`，所以走的是
`scheduler.go` 尾端那條真正的傳遞路徑（`poolSymbols, states, statesErr`），
一次斷言兩個**方向相反**的收斂：回補退回**全量**、偵測**不猜端點**記
`partial` ＋ `verification_unavailable`。順帶釘住兩筆 `job_runs` 是分開的
（parent 維持 `success`，偵測的 `partial` 不污染它），以及偵測寫在 parent 自己的紀錄關閉之後。

#### A 的實作結果（2026-08-28）

**四項情境全部有實測結果，數字與判讀已歸檔到**
[`architecture.md`](./architecture.md)「日 K 缺漏偵測 › 驗收實測（2026-08-28）」。
以下只留這份清單要追蹤的東西。

**負向那半改由 live 首輪承接，比原計畫更強。** T-067 原本擔心「dev 不一定有 `2867` 的資料」，
結果 live 首輪（16:25）就自然涵蓋了：6 個候選全部來自 `2867`，對真實交易所核對後判
`verified` 不告警。dev 那輪又獨立重現一次。

**偏離計畫的一步：dev 的歷史 K 棒改用「從 live postgres 複製」而不是 FinMind 回補。**
原因是 dev 沒有 `FINMIND_API_KEY`（`deploy.sh` 進版控的是佔位值，金鑰在
`/opt/stacks/scripts/stock_trading/`），回補一定失敗。**這不影響驗收效力**——
受測的偵測讀的是「DB 實際日期集合 ＋ TWSE 年度日曆 ＋ 交易所逐檔核對」，
三者都不經過 FinMind。代價是還原不能走「自然路徑 upsert 補回」，改用備份手動處理
（原文本來就留了這條退路）。**下次要在 dev 驗任何吃 FinMind 的東西之前，先確認金鑰。**

**還原已逐項比對基準完成**：`2330` 537 列止於 08-18、`1101`/`2867` 各 0 列、
`candle_verification_state` 0 列、池 0 列、暫存表已移除。
⚠️ **dev 的基準比原文假設的乾淨**——池本來就是空的、`1101`/`2867` 本來就沒有 K 棒，
所以還原是「移除我加的東西」而不是「把刪掉的補回去」。下次基準不同時不能照抄這段。

#### B 的進度

| 交易日 | 狀態 |
|---|---|
| 2026-08-28（五） | ✅ 首輪 `success`，數字見 `architecture.md` |
| 2026-08-31（一） | ⬜ 待觀察 |
| 2026-09-01（二） | ⬜ 待觀察 |

**live 端的正向告警至今沒有發生過**，所以 `partial` ＋ `upstream_data_gap` 在 live 的
表現目前只有 dev 的等價證據；`deferred`、陳舊升級、breaker 三條路徑也還沒被真實流量觸發。
B 收完之後，若這幾條仍未觸發，要在 `architecture.md` 留一筆「未驗到」而不是當成已驗。
