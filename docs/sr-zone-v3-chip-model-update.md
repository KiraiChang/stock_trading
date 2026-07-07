# SR Zone v3 籌碼特徵與模型更新紀錄

本文整理 SR Zone Scoring 這次從 v2 升級到 v3 的異動內容，作為後續重訓、驗證與維護的參考。

## 變更摘要

- `MODEL_VERSION` 從 `v2` 升級為 `v3`，模型檔預設路徑改為 `models/sr_scoring_v3.joblib`。
- ML 訓練特徵新增籌碼欄位，籌碼不再只影響 `trading_score`，也會進入 hold/break probability model。
- 模型載入時會檢查 `feature_names` 是否等於目前 `FEATURE_COLUMNS`，舊模型檔會明確失敗，避免用錯 schema 靜默產生錯誤分析。
- 訓練模型類型新增 `hist_gradient_boosting` 與 `lightgbm`，保留既有 `gradient_boosting` 與 `logistic_regression`。
- Go 訓練 API/client 新增傳遞 `split_method` 與 `calibration_method`，目前預設仍是 `time` + `sigmoid`。
- 前端 SR Zones 訓練 UI 新增 `Hist Gradient Boosting` 與 `LightGBM` 選項。

## v3 Feature Schema

v3 的 `FEATURE_COLUMNS`：

```python
[
    "touch_count",
    "rejection_count",
    "breakout_count",
    "average_bounce_return",
    "average_break_return",
    "relative_volume",
    "volatility",
    "trend_strength",
    "is_support",
    "chip_total_score",
    "chip_institutional_score",
    "chip_margin_score",
    "chip_broker_score",
    "chip_concentration_score",
    "chip_missing",
]
```

籌碼欄位來源是 `chip_scores`：

- `chip_total_score` 對應 `total_score`
- `chip_institutional_score` 對應 `institutional_score`
- `chip_margin_score` 對應 `margin_score`
- `chip_broker_score` 對應 `broker_score`
- `chip_concentration_score` 對應 `concentration_score`
- `chip_missing`：找不到籌碼資料時為 `1.0`，有資料時為 `0.0`

## Lookahead Safety

訓練資料產生時，會依每筆 touch event 的 `touch_time` 查詢該股票當下以前最新的 `chip_scores.trade_date`，不使用 touch 之後的籌碼資料。

推論時，`score_symbol()` 也會用最後一根 K 棒日期作為 `before_date` 查詢籌碼資料，避免歷史重算時拿到資料庫中較新的籌碼分數。

缺少籌碼資料時不阻斷訓練或分析：籌碼數值欄位填 `0.0`，`chip_missing=1.0`，讓模型能學到「籌碼缺資料」這個狀態。

## 後續驗證

v3 後籌碼訊號同時透過兩條路徑影響最終 `trading_score`：ML 特徵會影響 `bounce_probability`/`break_probability`，獨立 `chip` 分量則直接以 15% 權重加進分數。後續需要用相同資料集比較「含 chip 特徵」與「不含 chip 特徵」模型的 AUC、brier、calibration 與回測結果，確認雙路徑沒有讓籌碼訊號被不透明地放大；在實驗完成前，不預設要移除模型特徵或獨立權重任一方。

## 模型選項

目前支援：

- `gradient_boosting`：既有預設，使用 scikit-learn `GradientBoostingClassifier`。
- `hist_gradient_boosting`：使用 scikit-learn `HistGradientBoostingClassifier`，適合較大 tabular dataset。
- `lightgbm`：使用 `LGBMClassifier`，作為高效率 GBDT 候選；需要安裝 `lightgbm>=4.0.0`。
- `logistic_regression`：保留作為較可解釋的 baseline。

`lightgbm` 是 requirements 內的正式依賴；若環境尚未安裝，選用該模型時會回明確錯誤。

## 重訓與相容性

v3 與 v1/v2 模型檔不相容：

- v1 使用舊的單一 `avg_return_after_touch` schema。
- v2 已拆成 `average_bounce_return` / `average_break_return`，但沒有籌碼訓練特徵。
- v3 新增籌碼特徵，因此必須重新訓練。

重訓範例：

```powershell
cd python
.venv/Scripts/python.exe -m backtest.modular.sr_scoring.train --symbols 2330,2454,0050 --timeframe 1d --limit 1500
```

或透過 Go API：

```bash
curl -X POST http://localhost:8080/api/v1/sr-zones/train \
  -H "Content-Type: application/json" \
  -d '{"symbols":["2330","2454"],"limit":1500,"model_type":"gradient_boosting","split_method":"time","calibration_method":"sigmoid"}'
```

建議週期：日線模型以每週或每累積約 20 個交易日重訓一次為主；若新增大量股票、修改 feature schema、修改 zone builder、或 metrics 明顯退化，應立即重訓。

## 驗證紀錄

本次變更後已執行：

```powershell
cd python
.venv/Scripts/python.exe -m pytest backtest/modular/sr_scoring/tests -v
# 120 passed
```

```powershell
cd backend
go test ./...
# passed
```

```powershell
cd frontend
npm run build
# passed
```

## 主要異動檔案

- Python：`python/backtest/modular/sr_scoring/model.py`、`dataset.py`、`train.py`、`scoring.py`、`types.py`
- Python DB：`python/db.py`
- Go：`backend/internal/analysis/client.go`、`backend/internal/api/handler/sr_zones.go`
- Frontend：`frontend/src/lib/api/srZones.ts`、`frontend/src/routes/SRZones.svelte`
- Config：`python/config.yaml`
- Requirements：`python/requirements.txt`
