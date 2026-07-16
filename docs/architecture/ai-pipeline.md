# AI Pipeline

AI Pipeline 負責模型訓練、模型狀態、模型推論與模型評估。它不直接抓外部資料，也不輸出交易行動；
它只提供模型產物、機率、metadata 與 metrics 給 Analysis / Decision Pipeline 消費。

## 職責

- 建立訓練資料集。
- 訓練與保存模型 artifact。
- 提供模型狀態與可追溯 metadata。
- 執行模型推論。
- 保存訓練 job 與 metrics，讓模型品質可檢查。

## 現有模組歸位

| 類別 | 現有位置 | 說明 |
|------|----------|------|
| SR scoring 訓練 | `python/backtest/modular/sr_scoring/train.py` | 建立 dataset 並訓練 hold/break 模型 |
| 模型封裝 | `python/backtest/modular/sr_scoring/model.py` | `ModelBundle`、feature schema、metrics、config hash |
| 模型 artifact | `SR_SCORING_MODEL_PATH` | 目前唯一 active model 檔案 |
| 模型狀態 | Python `/sr-scoring/model-status` / Go `/sr-zones/model-status` | 回傳模型是否存在、版本、訓練時間、metrics |
| 訓練入口 | Python `/sr-scoring/train` / Go `/sr-zones/train` | 非同步訓練 job |
| 訓練任務表 | `sr_scoring_train_jobs` | job status、metrics、dataset summary |
| 模型推論 | SR Zone scoring | 輸出 bounce/break probability |
| 設定追溯 | `model_config_hash` | 訓練設定 hash，分析快照可追溯 |

## 目前模型策略

目前系統只維持一個 active model，不是 model registry。

- 前端可選 `model_type` / `split_method` / `calibration_method`。
- 每次訓練成功都會覆蓋 `SR_SCORING_MODEL_PATH`。
- `sr_scoring_train_jobs` 是 job history，不代表可切換的模型清單。
- 舊 job 可清理；active model artifact 不因清理 job 而刪除。

## 輸入

- Analysis Pipeline 建立的訓練特徵資料集。
- Data Pipeline 已落地的 candles 與 chip_scores。
- 使用者或前端提交的訓練參數。

## 輸出契約

- active model artifact
- `model_version`
- `model_config_hash`
- `training_config`
- train job status
- train metrics
- dataset summary
- `bounce_probability`
- `break_probability`

## 不負責事項

- 不直接決定買賣。
- 不管理 position sizing。
- 不自行同步市場資料。
- 不保存交易決策快照。

## SR Zone P0 契約

P0 先固定 AI Pipeline 在 SR Zone 中只提供模型與統計可信度，不輸出交易 action。

| 類型 | 欄位 / 結構 | P0 要求 |
|------|-------------|---------|
| 模型機率 / likelihood | `bounce_probability` / `break_probability` | 只代表模型輸出；若模型健康度不足，後續應標示不可宣稱精確勝率 |
| 模型追溯 | `model_version` / `model_config_hash` / `training_config` | 每筆分析快照需能知道使用哪個 active model 設定 |
| 模型狀態 | model status / train job metrics | 提供模型是否存在、訓練時間、metrics、dataset summary |
| 模型健康度 | `model_health`（後續 P2） | P0 先保留契約位置，實作延後到 AI governance 階段 |

AI Pipeline 不得輸出 `BUY`、`SELL`、`HOLD`、`final_entry_permission` 或
`position_action`。這些交易語意只屬於 Decision Pipeline。

## 後續升級方向

- model registry：允許多模型並存、回滾與指定模型推論。
- 自動化模型回測：對照訓練 metrics 與真實交易表現。
- 模型上線門檻：依 AUC、Brier score、樣本數、資料覆蓋率 gate active model。
- 模型退役策略：清理舊 artifact 與 job history。
