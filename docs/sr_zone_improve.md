# SR Zone Scoring 改善計畫

本文件供 Claude Code 依序執行。目標不是重寫 `sr_scoring`，而是在現有
「Zone Builder → Features/Labeling → ML Model → Scoring → Go 持久化 → UI」
架構上補齊可追蹤性、模型驗證、資料品質與營運可用性。

## 讀取範圍

開始實作前先讀這些檔案，避免改壞既有契約：

- `docs/sr-zone-scoring.md`
- `docs/architecture.md`
- `docs/api-reference.md` 的 SR Zone 區段
- `docs/database-schema.md` 的 `stock_sr_zone_analyses` / `stock_sr_zones`
- `python/backtest/modular/sr_scoring/`
- `python/http_server.py`
- `backend/internal/analysis/client.go`
- `backend/internal/api/handler/sr_zones.go`
- `backend/internal/store/model.go`
- `backend/internal/store/sr_zone_repo.go`
- `frontend/src/lib/api/srZones.ts`
- `frontend/src/routes/SRZones.svelte`

## 現況摘要

目前 SR Zone Scoring 已完成核心閉環：

- Python 端可建立 ATR / Volume Profile zones，計算 touch/rejection/breakout、分離 bounce/break return，並用已訓練模型輸出機率。
- `score_symbol()` 已輸出 global metrics、tier、trading score breakdown 與 recommendation。
- Go 端可呼叫 Python `/sr-zones`，將結果寫入 `stock_sr_zone_analyses` 與 `stock_sr_zones`。
- 前端已有分析、歷史紀錄、模型訓練觸發與 zone 分層顯示。

主要缺口集中在四類：

- 可追蹤性不足：`model_version` 已有 DB 欄位，但分析結果固定寫空字串。
- 模型品質不可驗證：訓練只回傳簡單 metrics，沒有時間切分、校準、混淆矩陣、資料分佈與產出報告。
- 生命週期不完整：zone 有 `status` / `broken_at` / `broken_price`，但沒有 verifier 或排程更新。
- 營運狀態不可見：訓練是背景 goroutine，API 立即回 202，但沒有 job id、進度、最後訓練結果或前端輪詢。

## 改善原則

1. 保持 Python 是唯一量化邏輯來源，Go 只做呼叫、持久化、API 與輕量驗證。
2. 所有新欄位要同步 Python response、Go DTO、store model、DB migration、TypeScript type、UI。
3. 不改動現有 API 欄位語意；新增欄位優先採 optional / nullable，避免破壞歷史資料。
4. 每個任務都要補測試，最少覆蓋 Python unit tests 與 Go DTO/repo/handler tests。
5. 改善模型品質時要保留 deterministic 行為，固定 `random_state`，訓練摘要需可重現。

## Phase 1：模型版本與分析可追蹤性

### 任務 1.1：讓 `/sr-zones` 回傳模型 metadata

目前 `ModelBundle.version` / `trained_at` / `metrics` 只存在模型檔中，`score_symbol()`
沒有輸出，導致 Go `ToStore()` 只能把 `model_version` 寫成空字串。

實作：

- 在 `python/backtest/modular/sr_scoring/scoring.py::score_symbol()` 回傳：
  - `model_version`
  - `model_trained_at`
  - `model_feature_names`
- `model_feature_names` 可先只回傳，不一定入 DB；主要用於 API 診斷與測試。
- 更新 `python/http_server.py` 註解，說明 `/sr-zones` 會回傳模型 metadata。

驗收：

- `POST /sr-zones` JSON 頂層包含 `model_version: "v2"`。
- `model_trained_at` 來自 `ModelBundle.trained_at`。
- 未訓練模型仍維持 503 fail-fast。

### 任務 1.2：Go DTO 與 DB 寫入 `model_version`

實作：

- 在 `backend/internal/analysis/client.go::ZoneScoreResult` 新增：
  - `ModelVersion string`
  - `ModelTrainedAt *string` 或 `store.NullString` 對應前先用 pointer DTO
- `ToStore()` 寫入 `SRZoneAnalysis.ModelVersion = r.ModelVersion`。
- 若 Python 回傳空值，明確寫 `"unknown"` 或維持空字串需二選一。建議寫 `"unknown"`，並更新文件。
- 補 `backend/internal/analysis/client_test.go`，確認 `model_version` 會進 store model。

驗收：

- 新建分析後 `stock_sr_zone_analyses.model_version` 不再是空字串。
- `GET /api/v1/sr-zones/:id` 回傳 `analysis.model_version`。

### 任務 1.3：文件同步

更新：

- `docs/sr-zone-scoring.md` 移除「`model_version` 未被寫入」已知限制，改寫成已支援。
- `docs/api-reference.md` 範例中的 `"model_version": ""` 改為 `"v2"`。
- `docs/database-schema.md` 更新 `model_version` 說明。

## Phase 2：訓練任務可觀測化

### 任務 2.1：新增 SR scoring train job 狀態儲存

目前 Go 背景 goroutine 只寫 log，前端無法知道訓練成功、失敗、metrics 或模型路徑。

建議新增資料表 `sr_scoring_train_jobs`：

欄位：

- `id`
- `job_id`
- `status`：`queued` / `running` / `success` / `failed`
- `symbols`：JSON array string
- `timeframe`
- `limit`
- `model_type`
- `rows`
- `sources`
- `metrics`：JSON
- `model_path`
- `model_version`
- `error`
- `started_at`
- `finished_at`
- `created_at`

實作：

- 新增 migrations：SQLite / MySQL / PostgreSQL 都要有。
- 新增 `store.SRScoringTrainJob` 與 repo。
- `SRZoneHandler.Train` 建立 job 後回 `202 { job_id, status }`。
- goroutine 開始時更新 `running`，完成更新 `success`，失敗更新 `failed`。

驗收：

- 觸發訓練後立即拿到 `job_id`。
- 訓練完成後可從 DB 查到 rows、metrics、model_version 或 error。
- 失敗時不只寫 log，API 可查得到錯誤。

### 任務 2.2：新增查詢訓練任務 API

API：

- `GET /api/v1/sr-zones/train-jobs?limit=20`
- `GET /api/v1/sr-zones/train-jobs/:job_id`

前端：

- `frontend/src/lib/api/srZones.ts` 新增 type 與 fetch function。
- `SRZones.svelte` 在訓練區塊顯示最近訓練紀錄：
  - status
  - rows / sources
  - model type
  - model version
  - hold/break AUC、precision、recall
  - error
- 觸發訓練後輪詢目前 job，直到 `success` / `failed`。

驗收：

- 使用者不需要看 server log 就知道訓練是否成功。
- 失敗原因會顯示在前端。

## Phase 3：模型驗證與校準

### 任務 3.1：改用時間序列 holdout 評估

目前 `model.py::train_model()` 使用 `train_test_split` 隨機切分，對金融時間序列容易高估表現。

實作：

- 保留隨機切分為可選模式，但新增預設 `split_method="time"`。
- `dataset` 已有 `touch_time`，依時間排序後用最後 `test_size` 比例當 holdout。
- metrics 同時回傳：
  - train rows / test rows
  - positive rate train / test
  - accuracy / precision / recall / auc
  - brier score
  - log loss
- `run_training()` 回傳 `split_method`。

驗收：

- 預設訓練結果不再使用隨機切分。
- 測試確認時間較晚的 rows 只出現在 test set。

### 任務 3.2：機率校準

目前 Gradient Boosting 的 `predict_proba` 未校準，`bounce_probability` / `break_probability`
可能排序有用但機率值不準。

實作選項：

- 使用 `CalibratedClassifierCV`，支援 `method="isotonic"` / `"sigmoid"`。
- 訓練參數新增 `calibration_method`，預設 `"sigmoid"`。
- 若資料量太少，降級為不校準並在 metrics 標記 `calibrated=false`。

驗收：

- `ModelBundle.metrics` 包含 calibration 設定與 brier score。
- 預測仍符合 `_normalize_probabilities()` 後 `hold + break <= 1`。

### 任務 3.3：訓練資料診斷報告

新增 dataset diagnostics，幫助判斷模型是否可信。

回傳：

- 每個 symbol 的 rows 數
- support/resistance rows 分佈
- hold/break positive rate
- no-outcome touch 比例（若有統計）
- feature missing/zero rate
- RR reference 樣本數

實作位置：

- `python/backtest/modular/sr_scoring/dataset.py` 可新增 `summarize_training_dataset()`。
- `train.py::run_training()` 把 `dataset_summary` 放進結果。
- Go train job metrics JSON 原樣保存。

驗收：

- 訓練後可知道模型是由哪些股票、多少事件、什麼標籤分佈訓練出來。
- 若 rows 集中在少數股票，前端可看出風險。

## Phase 4：Zone 生命週期驗證

### 任務 4.1：新增 SR zone verifier

現況 `stock_sr_zones.status` 永遠是 `PENDING`。需要用後續 candles 更新 zone 是否仍有效。

建議語意：

- `HELD_SO_FAR`：分析後價格曾觸碰 zone，且尚未連續突破不利邊界。
- `BROKEN`：依 role 判斷：
  - SUPPORT：收盤連續 `confirmation_bars` 根低於 `price_low`
  - RESISTANCE：收盤連續 `confirmation_bars` 根高於 `price_high`
  - AT_ZONE：先等價格離開 zone 後解析角色，或保持 `PENDING`
- `broken_at` / `broken_price`：第一次確認突破的 K 棒。

實作位置：

- 參考個股分析 verifier：`internal/analysis/verifier.go`。
- 新增 `internal/analysis/sr_zone_verifier.go`。
- repo 新增更新方法，例如 `UpdateZoneStatus()`。

驗收：

- 對 SUPPORT zone，後續 candles 連續跌破會標記 `BROKEN`。
- 尚未跌破但有觸碰會標記 `HELD_SO_FAR`。
- 無後續資料不改狀態。

### 任務 4.2：新增手動驗證 API

API：

- `POST /api/v1/sr-zones/:id/verify`

行為：

- 讀取 analysis 與 zones。
- 讀取 `analyzed_at` 之後的 candles。
- 更新 zones 狀態。
- 回傳更新後 analysis + zones。

前端：

- 分析詳情旁新增「重新驗證」按鈕。
- zone 狀態 badge 改成真實狀態，不再只是預留欄位。

驗收：

- 手動驗證可重複執行且 idempotent。
- `BROKEN` zone 不會被後續反彈改回 `HELD_SO_FAR`，除非明確設計重置 API。

### 任務 4.3：排程整合

將 SR zone verifier 接到 daily close job，或新增獨立 job：

- 每日收盤後驗證最近 N 天的 SR analyses。
- 寫入 scheduler job run 記錄。
- 失敗時不影響現有 indicator/signal 排程。

驗收：

- 不需要手動點擊也會更新近期 zone 狀態。

## Phase 5：Zone Builder 與評分品質改善

### 任務 5.1：跨方法重疊 zone 去重或群組

目前 ATR 與 Volume Profile 不跨方法合併，可能輸出高度重疊的 zones，前端看起來像重複訊號。

建議不要直接刪除資訊，先做「群組」：

- 新增 `zone_cluster_id` 或 response-only `overlap_group`。
- 同群組定義：IoU 或 overlap ratio 超過門檻，例如 `overlap / min(width_a, width_b) >= 0.6`。
- 群組內保留所有 zone，但 UI 可收合或標註「多方法共振」。

實作順序：

1. Python scoring 階段對 zones 分群。
2. `ZoneScore` 新增 `overlap_group` / `confluence_count`。
3. Go / DB / TS / UI 同步。

驗收：

- 重疊 ATR + volume profile zone 會顯示同一 group。
- 不改變原本排序規則，只在同 tier 內可用 confluence 作次排序或顯示輔助資訊。

### 任務 5.2：把 builder / feature 參數納入模型 metadata

目前 `atr_width_multiplier`、`merge_pct`、`forward_bars`、`threshold_pct` 等散落在預設值。

實作：

- `ModelBundle` 新增 `training_config`：
  - DatasetConfig
  - builder configs
  - feature columns
  - split/calibration/model type
- `/sr-zones` 回傳 `model_config_hash`。
- 分析快照可儲存 `model_config_hash` 或在 `model_version` 後附加短 hash。

驗收：

- 看到一筆分析時，可以知道它用哪組訓練/label/builder 設定產生。
- 重訓改參數後，舊分析可被辨識出來。

### 任務 5.3：合理化 `touch_count` 與 confidence

目前 `confidence` 使用 `features_as_support.touch_count`，而 `touch_count` 是所有方向聚合值；語意可行但命名容易誤解。

改善：

- 明確拆分：
  - `touch_count_total`
  - `touch_count_role`
  - `support_touch_count`
  - `resistance_touch_count`
- confidence 可改用 role-specific touch count，global confidence 再用 total 作輔助。
- 若改 API 欄位風險太高，先在 Python 內部拆分並保留舊 `touch_count` 為 total。

驗收：

- 支撐/壓力兩種角色的歷史樣本數可被診斷。
- 文件明確說明 confidence 使用哪個 count。

## Phase 6：前端決策可用性

### 任務 6.0：改成新手優先的閱讀層級

目前 `frontend/src/routes/SRZones.svelte` 同時顯示模型、機率、EV/RR、量能、
驗證狀態、touch 統計與 score breakdown。這些細節對驗證邏輯有用，但對投資
新手來說資訊密度太高，容易不知道第一眼該看哪裡、下一步該做什麼。

目標：

- 預設畫面讓新手只需要讀「結論、理由、下一步」。
- 保留所有細節，但收合在「進階指標」或「驗證細節」區塊，供之後 debug、
  模型驗證與進階使用者檢查。
- 不刪 API 欄位、不刪現有計算，這是 UI 資訊架構調整，不是演算法調整。

建議 UI 分層：

1. 頂部總結卡：只顯示股票、現價、整體判斷、整體信心、主要觀察區間。
2. 新手操作區：用白話呈現：
   - 「目前比較接近支撐 / 壓力 / 區間內」
   - 「可以觀察價格是否回到 X ~ Y」
   - 「若跌破/突破此區間，這個判斷失效」
   - 「信心：低/中/高」
3. Zone 列表預設簡化：
   - 價格區間
   - 支撐/壓力角色
   - 建議：觀察 / 避開 / 買進候選
   - 信心
   - 失效條件
4. 每個 zone 加上「進階」展開區：
   - support_score / resistance_score / net_score
   - bounce_probability / break_probability
   - expected_gain / expected_loss / expected_value
   - risk_reward_ratio / reward_risk_percentile
   - volume_confirmation / recent_validation
   - trading_score_breakdown
   - touch_count / reject_count / break_count / zone_momentum

文案規則：

- 避免讓新手直接面對 `EV`、`RR`、`net_score`、`confidence` 這類術語；預設顯示
  中文白話，術語放 tooltip 或進階區。
- 所有建議都要保持「輔助判斷」語氣，不寫成保證獲利或自動交易指令。
- 對 `LOW` confidence 要明確提示「樣本少或太久沒測試，先觀察」。
- 對 `AT_ZONE` 要明確提示「現在在區間內，方向還不明確」，不要給新手看起來
  像確定買賣訊號。

驗收：

- 使用者不展開進階區，也能知道：
  - 哪個價格區間最重要。
  - 目前應該觀察支撐還是壓力。
  - 什麼條件代表判斷失效。
  - 這個判斷可信度高不高。
- 展開進階區後，現有所有細節仍可看到，供後續 verifier、模型校準與問題追蹤使用。

### 任務 6.1：新增模型狀態區塊

前端目前只有「開始訓練」與文字訊息，缺少模型是否可用的明確狀態。

建議 API：

- Python：`GET /sr-scoring/model-status`
- Go proxy：`GET /api/v1/sr-zones/model-status`

回傳：

- model exists
- version
- trained_at
- model_path
- metrics summary
- feature_names

前端：

- 在訓練區塊顯示目前模型版本、訓練時間、AUC/Brier、可用/不可用。
- 若模型不存在，分析按鈕旁顯示明確狀態。

驗收：

- 使用者不用先按分析失敗才知道模型沒訓練。

### 任務 6.2：改善錯誤訊息

目前 `SRZones.svelte` catch 後固定顯示通用錯誤。

實作：

- `frontend/src/lib/api/client.ts` 保留後端 error/detail message。
- SRZones 根據 HTTP status / detail 顯示：
  - 503：模型未訓練
  - 404：沒有 candles 或資料不足
  - 502：Python service 未啟動或失敗
  - 400：輸入錯誤

驗收：

- 使用者能從 UI 判斷下一步是「補資料」、「啟動 Python」、「訓練模型」或「改股票代號」。

## 建議執行順序

1. Phase 1：模型版本寫入。低風險、高價值，先解決追蹤性。
2. Phase 2：訓練 job 狀態。解決目前操作黑盒問題。
3. Phase 3：模型驗證與校準。提高機率輸出的可信度。
4. Phase 4：zone verifier。讓 `status` 欄位變成真實功能。
5. Phase 6：前端新手模式、狀態與錯誤。先讓使用者看得懂怎麼操作，再逐步顯示後端能力。
6. Phase 5：builder/score 品質優化。影響模型與輸出語意，等前面可觀測性完成後再做。

## 測試建議

Python：

```bash
cd python
python -m pytest backtest/modular/sr_scoring/tests
python -m backtest.modular.sr_scoring.train --symbols 2330,2454,0050 --timeframe 1d --limit 1500
```

Go：

```bash
cd backend
go test ./internal/analysis ./internal/api/handler ./internal/store
```

Frontend：

```bash
cd frontend
npm run build
```

端到端手動驗收：

1. 啟動 Python service 與 Go server。
2. 在前端觸發 SR 模型訓練。
3. 從 train job API 看到 running → success。
4. 對一檔有足夠 candles 的股票執行 SR analysis。
5. 確認 DB `model_version` 非空。
6. 開啟歷史分析，zones 排序、global metrics、trading score breakdown 正常。
7. 若已完成 verifier，補入後續 candles 後手動驗證，確認 zone status 更新。

## 注意事項

- 不要把個股分析的 `stock_analysis_levels` verifier 直接套到 zone；zone 是區間，且 role 可能是 `AT_ZONE`。
- 不要在模型不存在時回傳中性機率；維持目前 503 fail-fast。
- 不要把 `global_trend` / `global_volatility` 放回每個 zone，現有設計刻意避免重複。
- 新增 DB 欄位時三種 migration 都要同步：SQLite、MySQL、PostgreSQL。
- 若新增 response 欄位但暫不入 DB，仍要更新 TypeScript type，避免前端隱性 any。
