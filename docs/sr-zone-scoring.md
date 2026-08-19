# 支撐/壓力機率評分（SR Zone Scoring）

機構級的支撐/壓力分析：輸入一檔股票的歷史 OHLCV，輸出一組「價格區間
（zone）」，每個 zone 都帶有支撐/壓力強度、可信度、反彈/跌破機率、期望值、
風險報酬比、量能確認、驗證狀態、可拆解的交易分數與交易建議——目標是把系統
從「描述市場」提升到「指導交易」，同時所有指標都要可解釋、不互相矛盾、
可回測驗證（詳見文末「設計原則」）。

跟 [stock-analysis.md](./stock-analysis.md)（個股分析）的差異：個股分析用
`SwingHighLowSR`/`ATRChannelSR`/`VolumeProfileSR` 算出的是**單一價位**
（Level），規則式判斷進場/停損/停利；SR Zone Scoring 算出的是**價格區間**
（Zone），用機器學習模型預測「觸碰後反彈 vs 跌破」的機率，兩者是完全獨立
的兩套系統，共用一小部分底層工具函式（`calc_atr`、`find_swing_highs/lows`），
不要混淆。

實作位置：
- 核心演算法：`python/backtest/modular/sr_scoring/`（`types.py`、
  `zone_builder.py`、`features.py`、`labeling.py`、`dataset.py`、
  `model.py`、`scoring.py`、`train.py`）
- Python HTTP 端點：`python/http_server.py`（`POST /sr-zones`、
  `POST /sr-scoring/train`、`GET /sr-scoring/model-status`）
- Go 持久化：`backend/internal/analysis/client.go`（`ScoreZones`/
  `TrainModel`/`GetModelStatus`、`UpstreamStatusError`）、
  `backend/internal/store/sr_zone_repo.go`、
  `backend/internal/store/sr_scoring_train_job_repo.go`（訓練任務追蹤）
- Go 驗證：`backend/internal/analysis/sr_zone_verifier.go`（`SRZoneVerifier`，
  見「十四」），排程整合見 `backend/internal/scheduler/scheduler.go`
- API：`POST /api/v1/sr-zones`、`GET /api/v1/sr-zones`、
  `GET /api/v1/sr-zones/:id`、`POST /api/v1/sr-zones/:id/verify`、
  `POST /api/v1/sr-zones/train`、`GET /api/v1/sr-zones/train-jobs`、
  `GET /api/v1/sr-zones/train-jobs/:job_id`、
  `GET /api/v1/sr-zones/model-status`、
  `DELETE /api/v1/sr-zones/:id`（見 api-reference.md）
- 資料表：`stock_sr_zone_analyses`、`stock_sr_zones`、
  `sr_scoring_train_jobs`（見 database-schema.md）
- 前端：「支撐/壓力機率分析」頁面（`SRZones.svelte`，新手優先閱讀層級見
  「十五」）
- v3 籌碼特徵與模型升級紀錄：[sr-zone-v3-chip-model-update.md](./sr-zone-v3-chip-model-update.md)

---

## 整體流程

```
score_symbol(symbol, timeframe)
    ↓ fetch_candles（重用 db.py）
    ↓ ATRZoneBuilder().build(df) + VolumeProfileZoneBuilder().build(df)
      + RecentMicrostructureZoneBuilder().build(df)
    ↓ 依寬度分 3 個 Tier（見「六、Zone Tier」）
    ↓ 對每個 zone：
    │     compute_zone_features()（角色解析前後各算一次：support 視角 / resistance 視角）
    │     confidence()（多因子，跟角色無關，兩個視角共用同一個值）
    │     get_model() 預測 hold/break 機率 → 正規化 → 依 confidence 收縮成 support_score/resistance_score
    │     net_score = support_score - resistance_score
    │     依現價判斷 role（SUPPORT / RESISTANCE / AT_ZONE）
    │     role != AT_ZONE 才算：bounce/break probability、EV、RR、reward/risk percentile、volume_confirmation
    │     recent_validation、zone_momentum/zone_direction
    │     trading_score_breakdown → trading_score / zone_quality_score
    │     entry_relevance_breakdown → entry_relevance_score
    ↓ 依 Tier 排序，同層內依 trading_score 排序
    ↓ 用所有 zone 算出唯一的 Global Model（見「七、Global Model」）
    ↓ 回傳 {symbol, current_price, global_*, zones: [...]}
```

模型未訓練時 `get_model()` 會直接拋 `RuntimeError`（fail-fast），`/sr-zones`
回 `503`，不會靜默回傳中性機率——寧可讓呼叫端明確知道「還沒訓練」，也不要
給一個看起來正常、實際上沒有意義的數字。

---

## 一、Zone 建立（Zone Builder）

`zone_builder.py` 提供多種獨立方法，各自產生候選 zone，方法不同，不跨方法
合併（`method` 欄位標註來源）：

### ATRZoneBuilder

以 `trend.find_swing_highs`/`find_swing_lows` 找 pivot 當候選中心價，
`zone_width = atr_width_multiplier × ATR(atr_period)`（預設
`atr_width_multiplier=1.5`、`atr_period=14`）。同一批 pivot-high／pivot-low
候選各自依距現價遠近排序，取前 `max_zones_per_type`（預設 5）個。

**合併演算法與寬度上限保護**：原始邏輯（single-linkage：只比較新候選跟
「目前已經合併到多大」的邊界夠不夠近）有鏈式擴張的 bug——一串前後緊鄰的
pivot（例如同一段整理區間裡好幾個小波段高低點）會像滾雪球一樣被合併成一個
涵蓋過大範圍的區間，把本該是幾個獨立關鍵價位的資訊揉成一坨模糊區間，還可能
把現價一起吃進去，讓角色被誤判成 `AT_ZONE`、損失掉方向性的機率/EV/RR 輸出。

修正後（`_merge_zone_candidates`）：每個 cluster 記住「開啟它的第一個候選」
的原始寬度當固定基準（不會因為合併越滾越大而跟著鬆綁），合併後總寬度超過
`max_merge_width_multiple × 基準寬度`（預設 2.0 倍）就停止繼續吃，讓後面的
候選各自獨立輸出：

```
gap_threshold = merge_pct × max(現有候選中心價, 新候選中心價)
可以合併  ⟺  新候選.price_low <= 現有候選.price_high + gap_threshold
          且  合併後寬度 <= max_merge_width_multiple × 該 cluster 的基準寬度
```

### VolumeProfileZoneBuilder

`typical price = (H+L+C)/3` 分箱（預設 `num_bins=24`），高成交量 bin
（`>= quantile(bin_volume, high_volume_percentile)`，預設 0.7）允許中間夾雜
最多 `max_gap_bins`（預設 1）個未達標的 bin 也合併成一段連續區間，依總成交量
排序取前 `max_zones_per_type` 個（現價上下各自獨立排序取前 N）。

角色（SUPPORT/RESISTANCE）不在建立階段決定——同一個 zone 在價格穿越後，
支撐/壓力角色本來就會互換，實際角色由 `scoring.py` 依「當下價格」動態判斷
（`_resolve_role`：現價 > zone 上緣 → SUPPORT；現價 < zone 下緣 →
RESISTANCE；否則 AT_ZONE）。

### RecentMicrostructureZoneBuilder

短線戰術 zone 來源，用來補足 ATR / Volume Profile 偏結構性的區間：

- `recent_pivot`：最近 swing high / swing low。
- `breakdown_reclaim`：前收盤價被最新 K 棒盤中穿越後又收回或跌回，作為 breakdown / reclaim 參考線。
- `vwap_reclaim`：若資料含 `vwap` 則使用最新 VWAP，否則以 5 日收盤均價作為平均成本 reclaim 參考。

這些 zone 寬度較窄，預設視為短線戰術層，不取代主結構 zone；Decision Engine
會把它們納入 `market_events`、`defense_lines.tactical` 與 primary-zone ranking。

### Adaptive builder 選用與 `zone_builder_runtime_config`

`pipeline._resolve_runtime_builders` 決定這次分析要用哪組 builder 設定，並把決策過程原樣
記錄在 `analysis.zone_builder_runtime_config`。**這是純紀錄，不參與任何仲裁或狀態推導**，
但少了它就無從回答「這次分析為什麼用這組 zone 寬度」。

| `reason_code` | 意義 | `enabled` |
|---|---|---|
| `VOLATILITY_BUCKET_CONFIG` | adaptive 生效，依波動 bucket 套用 `VOLATILITY_BUCKET_ATR_CONFIGS` 的覆寫 | `true` |
| `EXPLICIT_BUILDERS` | 呼叫端自帶 builder，不做 adaptive 解析 | `false` |
| `ADAPTIVE_ZONE_BUILDERS_DISABLED` | adaptive 開關關閉，用 baseline 預設 | `false` |
| `UNKNOWN_VOLATILITY_BUCKET` | 分不出 bucket（資料不足），退回 base config | `false` |
| `ADAPTIVE_ZONE_BUILDERS_ERROR` | 解析過程拋例外，已回退預設 builder，另帶 `error` 字串 | `false` |

`config` 是三個 builder 的參數快照（`ATRZoneBuilder` / `VolumeProfileZoneBuilder` /
`RecentMicrostructureZoneBuilder`），形狀依 builder 而異。

**落地與相容性**（2026-08-05，T-037 B）：這個欄位先前在 Go 端沒有承接欄位，Python 送了也會
在 `ToStore()` 被丟掉，前端拿不到。現在 `stock_sr_zone_analyses` 有同名欄位（migration 057，
mysql / postgres / sqlite 三份），由 API 的 `analysis` 區塊回傳，SR Zone 頁的分析卡下方以
摺疊區顯示。

- **舊分析（057 之前）的值是 JSON `null`**，代表「沒有這項紀錄」——**不等於 adaptive 未啟用**
  （那是 `enabled:false` 且有 `reason_code`）。前端據此整區隱藏，不可顯示成「未啟用」。
- 欄位是 `NOT NULL`：`store.RawJSON` 是純 string、沒有實作 `sql.Scanner`，SQL NULL 會讓
  scan 直接失敗，所以三份 migration 都把舊列 backfill 成 JSON `null` 而非留 SQL NULL。

### Volatility bucket 門檻＝凍結的全市場分位數（2026-08-17 重定）

`LOW_VOLATILITY_THRESHOLD` / `HIGH_VOLATILITY_THRESHOLD`（`zone_builder.py`）**不是手選的整數**，
是一次全市場量測的凍結結果：

| | 舊值（2026-08-17 之前） | 現值 |
|---|---|---|
| LOW | `< 1.5%` | **`< 4.6089927430152715%`** |
| HIGH | `> 3.5%` | **`> 6.278197721225691%`** |

量測條件記在同檔的 `VOLATILITY_THRESHOLD_PROVENANCE`：319 檔流動性合格股票
（`security_type=股票`、日均成交 ≥ 2,000 萬）的 P33 / P67，工具是
`scripts/build-selection-report.sh`。

**為什麼要重定**：舊值與台股實際分佈差一個量級。用它分類 T-040 選出的 131 檔會得到
**103 / 26 / 1**——LOW 只剩一檔，`VOLATILITY_BUCKET_ATR_CONFIGS` 的 LOW 那組 config
永遠不會被觸發、也永遠無法用資料驗證，T-003 的 sweep 因此卡住。重定後是 **53 / 46 / 32**。

**判定基準是 `max(atr_pct, average_range_pct)`，門檻必須用同一個基準量。**
這是重定時踩出來的：selection report 一開始只取 `atr_pct` 的分位數，但 319 檔裡有
**156 檔（49%）的 `average_range_pct` 比 `atr_pct` 大**，兩種基準會讓 131 檔中的 20 檔
分到不同 bucket。**門檻、切點、判定基準三者同源**是硬性要求，否則重定門檻也修不好
「選池說 LOW、runtime 說 NORMAL」這類對不上的問題。

**值刻意不四捨五入。** 選池的 `bucket_hint` 就是用這組數字判的，取整會讓貼在邊界的
十幾檔與它不一致（實測 131 檔中有 18 檔距最近邊界不到 2%）。

**要重新取分位數就改這兩個常數並升 `universe_version`**——那是一次明確的版本動作。
不這樣做的後果已實證：分位數是相對於當下母體的，2026-08-17 有 3 檔（3530、3661、8102）
`atr_pct` 一個 bit 都沒變卻換桶，只因母體變了邊界移動。凍結機制與選池的關係見
[`evaluation-universe-selection-plan.md`](./evaluation-universe-selection-plan.md)。

**既有資料不重算。** `stock_sr_zone_analyses` 裡 2026-08-17 之前的列，其 bucket 語意是舊門檻。
這沿用 [`database-schema.md`](./database-schema.md)「股價還原」段已立的原則——
分析紀錄是「當時做了什麼判斷」，不是快取。做跨期比較時要記得這條分界。

---

## 二、特徵工程（Features）

`features.py::compute_zone_features(df, zone, as_of_index, approach, ...)`
是訓練資料與即時評分共用的唯一入口，語意是「以 `as_of_index` 當下累積至今
的歷史表現」，避免訓練/評分兩套邏輯不一致：

| 特徵 | 說明 |
|---|---|
| `touch_count` | 這個 zone 被觸碰的次數（`find_touches`，看 K 棒範圍是否與 zone 相交） |
| `rejection_count` | 觸碰後在確認窗口內反向遠離的次數 |
| `breakout_count` | 連續收盤突破邊界達確認根數視為一次突破（state machine，避免同一段行情重複計數） |
| `average_bounce_return` | 觸碰後被分類為「守住/反彈」的那些歷史觸碰，其 forward_return 平均值（恆為正或 0） |
| `average_break_return` | 被分類為「跌破/突破」的那些歷史觸碰，其 forward_return 平均值（恆為負或 0） |
| `relative_volume` | 觸碰量 / 觸碰前 MA volume |
| `volatility` | `ATR / close`（股票層級量，見「七、Global Model」） |
| `trend_strength` | MA20 序列線性回歸斜率正規化值（股票層級量） |

`average_bounce_return`/`average_break_return`（`average_bounce_break_returns`）
是這次重新設計的關鍵修正：舊版只有一個 `avg_return_after_touch`，把「反彈時
漲多少」跟「跌破時虧多少」混在一起平均，數學上沒有意義（一個恆正、一個恆負，
混合平均會互相抵消）。新版重用 `labeling.py::label_touch` 的判定邏輯，把每次
觸碰依結果分類後，各自獨立平均。

`zone_momentum(df, touches, lookback=5)` 是真正**逐 zone 不同**的量——平均這
個 zone 每次被觸碰前的價格動能（`(close[i] - close[i-lookback]) / close[i-lookback]`），
不是股票層級的 `trend_strength`。

---

## 三、Labeling（訓練資料標記）

`labeling.py::label_touch(df, touch, forward_bars, threshold_pct, method)`
決定一次觸碰事件「未來到底是守住還是跌破」：

```
role=SUPPORT：
    hold  = 未來 forward_bars 根內最高點漲幅 > threshold_pct（反彈成立）
    break = 未來 forward_bars 根內最低點跌幅 < -threshold_pct（支撐失守）
role=RESISTANCE（鏡射）：
    hold  = 未來 forward_bars 根內最低點跌幅 < -threshold_pct（壓力有效）
    break = 未來 forward_bars 根內最高點漲幅 > threshold_pct（壓力突破）
```

預設 `forward_bars=5`、`threshold_pct=0.03`，`method="max_excursion"`。資料
不足以判斷未來窗口時回傳 `None`（不強行標記）。**No-lookahead 保證**：所有
呼叫端在傳入 `as_of_index` 前，都會先檢查 `touch_index + forward_bars <=
as_of_index` 才呼叫 `label_touch`（`label_touch` 內部只用 `len(df)` 當邊界，
不知道「現在」是哪一根，這個檢查必須由呼叫端負責，訓練與評分共用同一套
`_classify_touches` 輔助函式，見 `scoring.py`）。

---

## 四、機率模型（ML Model）

`dataset.py::build_training_dataset()` 用上述 labeling 邏輯做 walk-forward
組裝（每隔幾根重建 zone、偵測新觸碰事件才產生一筆訓練列），彙整成扁平
DataFrame，欄位跟 `FEATURE_COLUMNS` 完全對齊：

```python
FEATURE_COLUMNS = [
    "touch_count", "rejection_count", "breakout_count",
    "average_bounce_return", "average_break_return",
    "relative_volume", "volatility", "trend_strength",
    "is_support",  # 角色 one-hot；support/resistance 樣本 pooled 一起訓練，泛化性較好
    "chip_total_score", "chip_institutional_score", "chip_margin_score",
    "chip_broker_score", "chip_concentration_score", "chip_missing",
]
```

`model.py::train_model(dataset, model_type)` 訓練**兩個獨立分類器**：
`hold_model`（預測反彈/支撐延續）與 `break_model`（預測跌破/突破延續），
`model_type` 可選 `gradient_boosting`（預設）、`hist_gradient_boosting`、`lightgbm` 或 `logistic_regression`。
訓練同時算出 `rr_reference`——所有 `abs(bounce/break)` 非零樣本的排序陣列，
存進 `ModelBundle`，供即時評分時查百分位（見「八」）。

`MODEL_VERSION = "v3"`：v1 的 feature schema 用單一 `avg_return_after_touch`，
v2 缺少籌碼訓練特徵，兩者都跟 v3 不相容。
**舊模型檔不能直接套用在新版程式碼上，需要重新訓練**（`python -m
backtest.modular.sr_scoring.train ...` 或 `POST /sr-zones/train`）。

`get_model()` 是 lazy singleton——一次分析（`score_symbol`）只載入一次已訓練
好的模型，全部 zone 共用同一份，這就是「只有一個 Global Model」的架構基礎
（見「七」）。

**模型可追蹤性**：`score_symbol()` 回傳頂層帶 `model_version`/
`model_trained_at`/`model_feature_names`（直接來自載入的 `ModelBundle`），
Go 端寫入 `stock_sr_zone_analyses.model_version`——每一筆分析都能回答「這是
哪個模型版本、什麼時候訓練的算出來的」。

**訓練任務可觀測化**：`POST /sr-zones/train` 不再只是觸發背景 goroutine、
回應完就沒有下文——Go 端會先建立一筆 `sr_scoring_train_jobs` 紀錄
（`status=pending`）並回傳 `job_id`，goroutine 依序更新
`running → done`/`failed`（含 rows/sources/metrics/model_path/model_version
或 error），前端可用 `GET /sr-zones/train-jobs/:job_id` 輪詢進度，不需要只靠
伺服器 log 或重新呼叫 `POST /sr-zones` 猜測新模型是否已生效。

**時間序列 holdout 切分**：`train_model()` 預設 `split_method="time"`——每檔
股票各自依 `touch_time` 排序後，取最後一段（預設 20%）當 test set，其餘當
train set，再合併所有股票的結果。不用「對整個 pooled 資料集做一次全域時間
排序」，因為跨股票 pooled 的資料集裡，不同股票的歷史時間範圍可能差很多
（例如有的股票剛上市、資料比較新），全域切分容易讓 test set 集中在少數
幾檔股票；逐股票各自切分則保證每檔股票都對 train/test 有貢獻，同時仍然
確保「test 一定比 train 時間晚」，不會用未來資料驗證過去的模型（金融時間
序列用隨機切分容易高估表現）。舊的隨機切分行為保留為 `split_method="random"`
供比較，不建議當作正式評估依據。

**機率校準**：`gradient_boosting`（`GradientBoostingClassifier`）的
`predict_proba` 預設不是良好校準的機率——排序可能有鑑別力，但機率的絕對值
不準。`calibration_method="sigmoid"`（預設，可選 `"isotonic"` 或 `"none"`）
用 `CalibratedClassifierCV` 包一層校準。訓練集太小（< 40 筆）或任一類別樣本
太少（< 10 筆）時自動降級為不校準，並在 `metrics.calibrated` 標記
`0.0`——校準本身需要在訓練集內部再切一次 CV，樣本不夠時這個切分不可靠，
寧可不校準也不要用一個看似有校準、實際上是雜訊的結果。

**擴充的訓練 metrics**：`hold`/`break` 兩個模型的 metrics 現在包含
`train_rows`/`test_rows`、`positive_rate_train`/`positive_rate_test`
（取代舊版單一、且其實是用全資料集算的 `positive_rate`）、`brier_score`、
`log_loss`、`calibrated`，加上原有的 `accuracy`/`precision`/`recall`/`auc`。

**訓練資料診斷報告**：`dataset.py::summarize_training_dataset()` 產生
`{rows, rows_by_symbol, role_counts, hold_positive_rate, break_positive_rate,
feature_zero_rate, rr_reference_count}`，`run_training()` 把它放進結果的
`dataset_summary` 欄位，原樣存進 `sr_scoring_train_jobs.dataset_summary`。
這是判斷「這次訓練出來的模型可不可信」用的——例如樣本集中在少數幾檔股票、
或某個特徵幾乎永遠是 0，光看 accuracy/AUC 這類整體指標看不出這件事。

---

## 五、機率正規化與分數推導

`hold_model`／`break_model` 是兩個獨立訓練的二元分類器，個別預測不保證
`hold_p + break_p <= 1`（理論上可能同時輸出高機率，邏輯上矛盾）。
`_normalize_probabilities`：加總超過 100% 時等比例縮小，讓兩者維持「至多
其中一個發生」的合理上限；`1 - hold_p - break_p` 隱含為「兩者皆未發生（盤整/
不明確）」的機率。

`support_score`/`resistance_score` 不是獨立於機率之外的規則式公式，而是由
（正規化後的）hold 機率**依 confidence 貝式收縮**而來：

```
score = confidence × hold_probability + (1 − confidence) × 0.5
```

confidence 高時 score 趨近模型機率本身，confidence 低時往中性值 0.5 收縮。
這個設計是為了解決舊版「支撐強度分數」與「反彈機率」可能互相矛盾的問題——
score 現在是機率的單調函式，不會再出現「強度分數很高但機率很低」這種自相
矛盾的輸出。

### 五之一、Probability Context 機率解讀層

`probability_context` 是機率的結構化解讀與品質標記層，不重新訓練模型、不改
`bounce_probability` / `break_probability`、不改 EV/RR、score 或 decision 門檻。
它把既有正規化後的機率補成前端可穩定顯示的三分解：

```
neutral_probability = max(0, 1 - bounce_probability - break_probability)
edge_pp = abs(bounce_probability - break_probability) × 100
```

**頂層 `probability_context` contract**：

| 欄位 | 說明 |
|---|---|
| `schema_version` | Probability context schema 版本，目前為 `sr_probability_context_v1` |
| `model_metrics.hold/break` | 訓練時保存的 AUC、Brier、log loss、calibrated、test rows 摘要 |
| `health.quality_flags` | 模型層品質提示，例如未校準或 test rows 偏少 |
| `health.warning_flags` / `blocking_flags` | P2 model governance 的警示與阻擋原因 |
| `health.health_state` | `HEALTHY` / `DEGRADED` / `UNRELIABLE`，供 Decision gate 消費 |
| `health.confidence_gate` | AI Pipeline 對 entry 的上限建議，例如 `max_entry_state=SMALL_ENTRY` 或 `WAIT_CONFIRMATION` |
| `health.average_edge_pp` | 具方向性 zone 的平均 hold/break 機率差距 |
| `health.directional_zone_count` / `zone_count` | 有方向機率的 zone 數與總 zone 數 |
| `model_reports.calibration_report` | 校準狀態、方法、Brier/log loss 的穩定報告 schema |
| `model_reports.walk_forward_report` | time split / walk-forward 測試列數與正例率報告 schema |
| `model_reports.dataset_diagnostics` | 訓練 dataset config、zone builders、split method 與測試列數摘要 |

Decision Pipeline 不直接讀 raw model metrics 決定交易行動；`build_decision_from_evidence()`
會先把 `AnalysisScores` 轉成 `model_governance`，再由 Decision 消費
`health_state` / `confidence_gate`。`UNRELIABLE` 會阻擋依機率模型進場；
`DEGRADED` 不直接產生 action，但會讓強買條件降級，最多保留小量或觀察語意。

**`zones[].probability_context` contract**：

| 欄位 | 說明 |
|---|---|
| `schema_version` | Probability context schema 版本，目前為 `sr_probability_context_v1` |
| `bounce_probability` / `break_probability` | 與 score 欄位同值，方便此 contract 自足顯示 |
| `neutral_probability` | 盤整/不明確的隱含機率；`AT_ZONE` 為 `null` |
| `dominant_outcome` | `BOUNCE` / `BREAK` / `NEUTRAL` / `NO_DIRECTION` |
| `edge_pp` | bounce 與 break 的百分點差距；越小代表方向優勢越不明顯 |
| `quality_flags` | zone 層品質提示，例如 `LOW_CONFIDENCE`、`LOW_PROBABILITY_EDGE`、`NO_DIRECTION` |

舊分析可能沒有 `probability_context` 或值為 JSON `null`，前端應隱藏此區塊並
繼續使用既有 probability / explanation 顯示。`AT_ZONE` 不給方向機率，會標記
`NO_DIRECTION` 與 `MISSING_DIRECTIONAL_PROBABILITY`，避免把區間內震盪硬解讀成
支撐或壓力。

---

## 六、Confidence（多因子可信度）

```
confidence = (sample_factor + recency_factor + stability_factor) / 3
```

| 因子 | 公式 | 意義 |
|---|---|---|
| `sample_factor` | `touch_count / (touch_count + 5)` | 貝式收縮：樣本數（觸碰次數）越少越保守，`touch_count=5` 時剛好 0.5 |
| `recency_factor` | `0.5 ^ (距最近一次觸碰的根數 / 40)` | 時間衰減：每過 40 根減半；從未被觸碰過回傳 0 |
| `stability_factor` | `max(守住次數, 跌破次數) / 總次數` | 歷史結果一致性；沒有可判定的歷史結果回傳中性值 0.5（無資訊，不直接判 0） |

三個因子等權重平均——任一因子偏低都會拖低整體 confidence，避免「觸碰次數
夠多但都是很久以前」或「次數多但結果很不穩定」被誤判成高可信度。`confidence
_level` 是分級結果：`< 30%` LOW、`30~60%` MEDIUM、`60~80%` HIGH、`>= 80%`
VERY_HIGH。

**touch_count 依方向拆分**：`touch_count`（API/DB 欄位）恆為兩個方向
（`FROM_ABOVE`/支撐方向、`FROM_BELOW`/壓力方向）觸碰次數加總，反映 zone
整體活躍度；`support_touch_count`/`resistance_touch_count` 分開統計，讓
「作為支撐」跟「作為壓力」各自的歷史樣本數可以被診斷。**confidence 依
角色只用其中一個方向的樣本數/穩定度計算**：`role=SUPPORT` 只用
`support_touch_count` 方向的觸碰算 `sample_factor`/`recency_factor`/
`stability_factor`，不會被壓力方向的（可能完全不同的）表現稀釋或拉抬；
`role=RESISTANCE` 同理只用壓力方向；`role=AT_ZONE`（方向還沒解析出來）
才用兩個方向合計計算，作為方向未定時的保守估計。這是 2026-07 加入
`support_touch_count`/`resistance_touch_count` 時一併修正的：修正前
`confidence` 一律用兩個方向合計的樣本/穩定度，同一個 zone 若「作為支撐」
表現很穩定但「作為壓力」表現很差（或反之），算出來的 confidence 會被另一
個不相關方向的表現拖累或拉抬，不是這個角色本身應有的可信度。

---

## 七、Support/Resistance Score 與 Net Score

`net_score = support_score - resistance_score`，`net_score_label` 依門檻
（`±0.15`）分類成 `STRONG_SUPPORT` / `NEUTRAL` / `STRONG_RESISTANCE`——避免
只看單一分數判斷，要求使用者同時比較兩邊的強度。

`role`（現價位置下目前扮演的角色）跟 `net_score_label`（這個價位帶過去
更像支撐還是壓力）是兩個不同概念，可能方向相反（例如 `role=SUPPORT` 但
`net_score_label=STRONG_RESISTANCE`）——這不一定是演算法錯誤，但前端需要
明確解釋兩者語意差異，並在方向相反時提示「角色與歷史強弱不一致，建議
降低信心」，避免使用者覺得同一張卡片自相矛盾（前端實作見
`frontend/src/routes/SRZones.svelte::roleNetScoreConflicts`）。

---

## 八、Expected Value / Risk Reward / Reward-Risk Percentile

只有 `role != AT_ZONE`（已解析出明確方向）才會算這些數字：

```
expected_gain  = 角色解析後的 average_bounce_return（>= 0）
expected_loss  = 角色解析後的 average_break_return（<= 0）
expected_value = bounce_probability × expected_gain + break_probability × expected_loss
risk_reward_ratio = |expected_gain / expected_loss|（expected_loss=0 時為 None，不硬除）
reward_risk_percentile = 這個 risk_reward_ratio 在訓練資料集歷史 RR 分佈（ModelBundle.rr_reference）中的百分位（bisect 查表）
```

`expected_value` 的公式是這次重新設計修正的核心問題：舊版用
`hold機率 × reward - break機率 × risk`，其中 reward/risk 是「zone 寬度」這種
結構性估計，跟「反彈時實際漲多少」脫鉤，會算出反直覺的結果（例如
Bounce=65%/Break=35%/Average Return=-1.6% 這種輸入卻得到負的 EV）。新版直接
用歷史上「真的反彈」與「真的跌破」的實際報酬分開加權，不再混用單一
`average_return`。

---

## 九、Volume Confirmation

依角色解析後的 `relative_volume` 與 `recent_validation`（見下）分類：

```
recent_validation == EXPIRED           且 relative_volume >= 1.2 → FAILED（高量但破了，量能確認了失敗）
recent_validation == VALIDATED_RECENTLY 且 relative_volume >= 1.2 → CONFIRMED（高量且守住）
relative_volume < 0.8                                             → WEAK（量能不足）
其餘                                                                → NEUTRAL
```

---

## 十、Zone Momentum / Zone Direction / Recent Validation

- `zone_direction`：`zone_momentum > 0.01` → UP；`< -0.01` → DOWN；否則 FLAT。
- `recent_validation`（取代舊版單純的 `Pending Validation`）：依「最近一次
  觸碰的時間」與「有沒有守住」判斷，狀態機大致如下：

```
從未被觸碰過                              → PENDING_VALIDATION
最後一次觸碰太新、還無法判定結果             → PENDING_VALIDATION
最後一次觸碰結果是跌破                       → EXPIRED
守住 且 距今 <= 20 根                       → VALIDATED_RECENTLY
守住 但 距今 > 60 根                        → NOT_TESTED_RECENTLY
守住 且介於 20~60 根之間                     → VALIDATED_RECENTLY（仍在有效窗口內）
```

---

## 十一、Zone Tier（可排序）

zone 依寬度（`price_high - price_low`）在**同一次分析**裡的相對排名分三層
（tercile），讓 zone 清單「可排序」成有意義的階層，而不是一堆平行、看不出
主次的價格區間：

| Tier | 中文 | 意義 |
|---|---|---|
| `TIER_1_MAIN_STRUCTURE` | 主結構 | 最寬的 1/3，宏觀主結構 |
| `TIER_2_TRADING_ZONE` | 交易區 | 中間 1/3，適合作為進出場的操作區間 |
| `TIER_3_SHORT_TERM` | 短期支撐 | 最窄的 1/3，貼近盤中操作的精確價位 |

`score_symbol()` 回傳的 `zones` 陣列依 Tier 由粗到細排序，同一層內再依
`trading_score` 由高到低排序（`_assign_tiers`/`_sort_zone_scores`，
`scoring.py`）。

`period_summaries` 的短／中／長摘要不直接沿用完整清單排序，而是用摘要專用
混合分數挑選各 tier 內最適合閱讀的支撐與壓力：`trading_score` 50%、
`confidence` 20%、距離現價 20%、多方法共振 10%。支撐候選仍必須在現價下方，
壓力候選仍必須在現價上方；完整 `zones` 排序不受摘要排序影響。

---

## 十二、Trading Score（可拆解）與 Trading Recommendation

```
Trading Score = EV(34%) + RR(17%) + Trend(12.75%) + Volume(12.75%) + Confidence(8.5%) + Chip(15%)
```

【2026-07 籌碼分析整合】新增 `chip` 分量後，原本 EV(40%)/RR(20%)/Trend(15%)/
Volume(15%)/Confidence(10%) 五個分量依原比例縮小（乘以 0.85），合計仍為
100，見 `scoring.py::TRADING_SCORE_WEIGHTS`。v3 模型也把 `chip_features` 納入
hold/break probability model，因此籌碼會透過兩條路徑影響最終分數：一是模型
機率進而影響 EV/support/resistance score，二是下表獨立的 `chip` 加權分量。

每個分量先正規化到 `[0,1]`（`_normalize_signed`：0.5 為中性，±cap 為 0/1
分），再乘上對應權重，回傳值就是「這個分量對總分的實際貢獻」，直接存在
`trading_score_breakdown`（六個分量加總 = `trading_score`），使用者可以逐項
檢視分數怎麼來的，不用自己再乘一次權重：

| 分量 | 權重 | 正規化方式 |
|---|---|---|
| `expected_value` | 34% | `expected_value` 正規化（cap=0.05）；`role=AT_ZONE` 或缺值時用中性值 0.5 |
| `risk_reward` | 17% | `min(1, risk_reward_ratio / 3.0)`；缺值時用中性值 0.5 |
| `trend` | 12.75% | `overall_trend` 正規化（cap=0.1），`role=RESISTANCE` 時取負號對齊方向 |
| `volume` | 12.75% | `volume_confirmation` 查表（CONFIRMED=1.0／NEUTRAL=0.5／WEAK=0.3／FAILED=0.0）；缺值時用中性值 0.5 |
| `confidence` | 8.5% | 直接用 `confidence`（本身已經是 0~1） |
| `chip` | 15% | 籌碼分數（`chip_scores.total_score`，-100~100）正規化（cap=100），`role=RESISTANCE` 時取負號對齊方向；`score_symbol()` 會依 `analyzed_at` 換算 `before_date` 查詢，避免歷史分析拿到未來籌碼資料；查無籌碼資料時用中性值 0.5，見 `fetch_latest_chip_score` |

`role=AT_ZONE` 或角色相關數值缺值時，對應分量用中性值 0.5 計算，不直接給
0 分——沒有方向不代表這個 zone「不好」，只是還沒有可以評分的方向性資料。

`trading_recommendation` 依角色跟分數映射（6 級，非對稱門檻——只有
`STRONG_SELL`、沒有單獨的 `SELL`，這是需求文件的原始定義）：

```
role=SUPPORT：    >=80 STRONG_BUY / >=60 BUY / >=40 WATCH / >=20 NEUTRAL / 其餘 AVOID
role=RESISTANCE： >=80 STRONG_SELL / >=60 AVOID / >=40 NEUTRAL / >=20 WATCH / 其餘 NEUTRAL
role=AT_ZONE：    >=50 WATCH / 其餘 NEUTRAL
```

---

## 十二之一、籌碼數字化輸出（摘要層）

籌碼不只影響分數，也在摘要層以**可拆解的數字**呈現，而不是壓成一句文字。輸出
分兩個層級，都對齊分析快照當下的籌碼（`before_date`，非即時最新值）：

**整檔層級 `chip_summary`**（`_build_chip_summary`，response top-level，存於
`stock_sr_zone_analyses.chip_summary` JSON 欄位，migration 034）：供前端「共用
籌碼面板」一次顯示，不逐 zone 重複。

| 欄位 | 範圍 | 說明 |
|---|---|---|
| `missing` | bool | `true`=查無籌碼資料（跟「分數接近 0 的中性」不同，前端分開顯示） |
| `score` | -100~100 | 籌碼總分（`chip_scores.total_score`） |
| `signal` | 類別 | `BULLISH`/`BEARISH`/`NEUTRAL`/`RISK` |
| `institutional_score`/`margin_score`/`broker_score` | -100~100 | 法人／融資／分點子分數 |
| `concentration_score` | 0~100 | 集中度子分數 |

**每張支撐/壓力摘要卡 `period_summaries[].support/resistance.chip`**
（`_zone_summary`，角色化；隨 `period_summaries` JSON passthrough，不佔 zones 表
欄位）：

| 欄位 | 說明 |
|---|---|
| `direction` | 整檔原始方向 `bullish`/`bearish`/`neutral`/`none`（未翻號；`none`=無資料） |
| `contribution` | 籌碼對這個角色 `trading_score` 的**直接加權貢獻**（0~15，已依支撐/壓力翻號，即 `trading_score_breakdown.chip`） |
| `bounce_delta_pp`/`break_delta_pp` | 籌碼對本 zone 反彈/跌破機率的**模型邊際貢獻**（百分點）；查無籌碼資料時為 `null` |

**機率邊際貢獻（反事實）怎麼算**：在 `score_zone` 裡，對該 zone 角色用同一組
zone 特徵各推論兩次——一次用實際 `chip_features`、一次用中性籌碼基準
（`neutral_chip_features()`：四子分數與總分皆 0、`chip_missing=0`），兩者正規化後
機率相減再乘 100 就是 `*_delta_pp`。這是啟發式解釋量（模型有交互作用，delta 會
因 zone 而異），不是嚴格的 Shapley 值；查無籌碼資料（`chip_missing`）時不計算。

**兩條路徑，不是重複計分**：`contribution`（直接加權，15%）與 `*_delta_pp`
（v3 模型特徵）是籌碼影響分數的**兩條獨立路徑**，摘要把兩者攤開正是為了讓使用
者看得到各自的效果，而非同一個效果被算兩次。這兩條路徑是否讓籌碼被實際放大
超過設計權重，需用 shadow policy 比較，而非直接從權重常數推論。

**籌碼雙路徑評估基準**：production scoring 目前保留直接 `chip` 分量；同時在
`scoring_rules.py` 提供 `_trading_score_breakdown_no_direct_chip` 作為離線比較用 shadow
policy。該 policy 移除直接 `chip` 分量，並把其餘分量恢復為 EV 40% / RR 20% /
Trend 15% / Volume 15% / Confidence 10%。後續若要調整 production 權重，應先比較
現況與 shadow policy 的 top1/top3 zone 排名、摘要支撐/壓力選擇、分數差異分布，
再決定是否移除或調低直接 `chip` 權重。

**摘要 `reasons` 不再含籌碼句**：籌碼從 `_zone_summary` 的 `reasons[]` 拉出改成上述
結構化 `chip` 欄位，`reasons` 只保留均線、驗證、量能、信心、共振等非籌碼理由，
避免同一件事在文字與數字兩處重複。整檔跑馬燈 `analysis_tips` 改由 `tips.py`
輸出固定分類的閱讀指南；籌碼只作為「判讀提醒」之一，不再混入產品操作說明。
偏多/偏空門檻統一為 `CHIP_SIGNAL_THRESHOLD`（±10，對齊
`internal/chip` 的 `signalThreshold`）。

---

## 十三、Global Model（只有一個）

同一次分析只有**一個**權威的整體評估區塊，不在每個 zone 重複輸出：

| 欄位 | 計算方式 |
|---|---|
| `global_trend` | `trend_slope(df)` 只算一次（股票層級量） |
| `global_volatility` | `zone_volatility(df)` 只算一次（股票層級量） |
| `global_expected_value` | 對所有「有明確方向」（role != AT_ZONE 且 `expected_value` 非 None）的 zone，用 confidence 加權平均：`Σ(zone_EV × zone.confidence) / Σ(zone.confidence)` |
| `global_confidence` | 所有 zone confidence 的**簡單平均**（不分角色，不用 confidence 加權自己，避免循環） |
| `global_risk_reward_ratio` | 比照 `global_expected_value`，用同一套 confidence 加權平均 `risk_reward_ratio` |

`global_expected_value`/`global_risk_reward_ratio` 只有在「完全沒有 zone
解析出明確方向」（zones 為空，或全部是 `AT_ZONE`/沒有 `expected_value`/
`risk_reward_ratio`）時才是 `null`。`global_confidence` 的 null 條件不同：
只要 zones 陣列本身不是空的，就算全部都是 `AT_ZONE`，`global_confidence`
仍然會是所有 zone confidence 的簡單平均，不會是 `null`——只有 zones 完全
是空陣列時 `global_confidence` 才會是 `null`。這是為了讓「Final EV」唯一
收斂：不再需要使用者自己判斷「要看哪個 zone 才代表這檔股票」。

---

## 十四、SR Zone Decision Engine（T-019）

SR Zone Decision Engine 是 position-agnostic 的市場決策層，輸出欄位仍是
`decision_summary`。第一階段只回答「這份 SR Zone 分析本身目前應如何閱讀」：
目前盤勢前提、唯一操作結論、主交易區、共用背景與風險。它不處理持有成本、
股數、未實現損益、停損金額、停利金額或加碼金額；這些由 Position Analysis
Decision Engine 處理。

它不取代既有 `global_*`、`period_summaries`、`analysis_tips`、`zones`、
EV/RR、機率、score breakdown 或籌碼拆解，而是把這些既有資料整理成單一決策
入口，讓使用者不用自行從多張 zone 卡片拼湊結論。

### Engine 邊界

實作上應把目前 `scoring.py` 內 `_build_decision_summary` 相關 helper 視為
Decision Engine 的 v1 內核，後續可抽到獨立模組
`backtest/modular/sr_scoring/decision_engine.py`。第一版邊界如下：

| 項目 | 說明 |
|---|---|
| 輸入 | `zone_scores`、`current_price`、`global_trend`、`global_volatility`、`global_metrics`、`chip_summary`、`ModelBundle` metadata |
| 輸出 | top-level `decision_summary` JSON |
| 持久化 | Go 端 passthrough 寫入 `stock_sr_zone_analyses.decision_summary` |
| 前端 | `/sr-zones` 頁面優先顯示 decision summary panel，再展開 zones 細節 |
| 不做 | 不做持股個人化、不下單、不輸出部位大小、不覆寫 zone 原始分數 |

Decision Engine 必須 deterministic：同一份 SR Zone scoring 結果、同一份模型
metadata 與同一份籌碼摘要，必須產生完全相同的 `decision_summary`。

`score_symbol()` top-level response 會增加：

```json
{
  "decision_summary": {
    "market_regime": {
      "primary": "TREND_UP",
      "flags": ["HIGH_VOLATILITY"],
      "label": "多頭趨勢但波動偏高",
      "reasons": ["整體趨勢向上", "波動高於近期常態"]
    },
    "action": "BuySmall",
    "action_label": "小量試單",
    "position_action": "HOLD",
    "position_action_condition": {
      "state": "SUPPORT_RECLAIM_CANDIDATE",
      "invalidation_price": 960.0,
      "recovery_price": 970.0,
      "reason_codes": ["PRIMARY_SUPPORT", "SUPPORT_RECLAIM_AWAIT_CONFIRMATION"]
    },
    "primary_zone": { "zone_id": 16, "role": "SUPPORT", "reason": "高信心支撐且風險報酬仍可接受" },
    "market_context": [],
    "confidence_explanation": {},
    "risk_notes": [],
    "secondary_zones": []
  }
}
```

### Market Regime

Market Regime 是所有解讀的最高優先共同前提，先用股票層級與摘要層資料判斷「這份分析應該用什麼市場狀態閱讀」，再決定 action 與 zone 文案語氣。採「一個相容 primary regime + structural/short-term 拆分 + 多個 flags」：

| 類別 | 用途 | 可能輸入 |
|---|---|---|
| `TREND_UP` | 偏多趨勢盤，支撐回測較值得關注 | `global_trend`、MA 位置、primary support、籌碼方向 |
| `TREND_DOWN` | 偏空趨勢盤，壓力與跌破風險優先 | `global_trend`、MA 位置、primary resistance、籌碼方向 |
| `RANGE_BOUND` | 區間盤，靠近支撐/壓力才有操作意義 | `global_trend` 接近中性、支撐壓力距離、EV/RR |
| `LOW_CONFIDENCE` | 樣本或近期驗證不足，所有 action 需降級 | `global_confidence`、primary zone confidence |
| `HIGH_VOLATILITY` | 波動偏高，倉位與停損要求提高 | `global_volatility`、ATR/close、zone width |

`LOW_CONFIDENCE`、`HIGH_VOLATILITY` 較適合作為 flags，不一定取代趨勢方向。例如 `primary=TREND_UP` 且 `flags=[HIGH_VOLATILITY]`，前端應呈現「偏多但波動高，不適合重倉追價」。

目前 `market_regime` 也會輸出：

- `structural_trend`：由 `global_trend` 判斷的中長線結構，值與相容欄位 `trend_regime` 一致。
- `short_term_regime`：由最新 market events / structure state 判斷的短線狀態，例如 `BREAKDOWN_RISK`、`RECLAIM_ATTEMPT`、`REVERSAL_CANDIDATE`、`RECOVERY`、`EARLY_TREND`、`NORMAL`。`RECOVERY` 對應 `structure_state=SUPPORT_RECLAIM_CONFIRMED`；`EARLY_TREND` 對應區間盤但 `global_trend>0` 且 `global_confidence>=0.55` 的早期趨勢。
- `tactical_regime`：短線戰術 regime，現階段與 `short_term_regime` 同值，作為 UI 與後續規則演進的明確欄位。
- `recovery_state`：收復/失效狀態；`structure_state=SUPPORT_RECLAIM_CONFIRMED` 時為 `RECOVERY`，其餘與 `structure_state` 同值，避免 Structural Trend、Tactical Regime、Recovery State 混在同一欄位解讀。
- `primary`：保留給舊前端/舊資料讀取的相容欄位，仍表示主要趨勢 regime。

`decision_summary.decision_derived_view` 是 Event Lifecycle 與對外語意欄位之間的權威轉接層。
推導順序固定為：`market_events`（本根 raw 偵測）→ `event_state_summary`（lifecycle）→
`decision_derived_view`（對外語意）→ 對外標籤。除候選區產生與防守線展示外，對外結論不得
各自直接讀 raw `market_events` 再推導一次狀態。

各對外標籤的接線現況（全部已由 derived view 推導；此為現況規格，非待辦）：

| 對外標籤 | 接線現況 |
|---|---|
| `market_bias` | ✅ 已接線（P3-B 後由 `semantic_pipeline.bias_state` 推導，hard blocker 優先） |
| `daily_confirmation` | ✅ 已接線（`daily_reason_codes`） |
| `final_entry_permission` | ✅ P3-C 後 state 由 `semantic_pipeline.entry_permission_state` 推導；daily invalidated / blocked / chasing risk 仍保守優先 |
| `price_path` | ✅ `path_state` 由 `path_gate_state` 推導；價位仍由 price path 函式計算 |
| `position_action_condition` | ✅ P3-C 後 `state` 由 `semantic_pipeline.action_state` 推導；防守價仍由 primary zone 計算，`structure_state` 僅供 debug |

P0–P2 已把語意 gate 接進 derived view，價位型欄位仍由各自函式計算實際價格，避免把 price
math 塞進 lifecycle layer。`decision_derived_view.version=decision-derived-view-p2` 是目前
收斂後的 payload：不再輸出 production 空轉 echo 的 `final_entry_gate_state`，也不在
`price_path` / `position_action_condition` 內輸出與 legacy state 平行的 gate 欄位。此段是
T-034 完成後的現況規格；後續維護以本文件為準，不再於 `docs/todo.md` 追蹤完成封存。

P3 開始新增 `decision_derived_view.semantic_pipeline` contract，用來明確呈現單向語意推導鏈：

```text
Event -> Lifecycle -> Market State -> Bias -> Action -> Entry
```

`semantic_pipeline.version` **目前為 `decision-semantic-pipeline-p4`**
（P3 起新增這個 contract，T-044 於 2026-08-13 因 lifecycle 語意改變升為 p4，
見下方「分層原則：lifecycle 不看 RR」）。標準欄位包含：
`event_signal`、`lifecycle_phase`、`market_state`、`bias_state`、`action_state`、
`entry_permission_state`、`reason_codes` 與 `source_order`。P3-A 先用 fixture 鎖住
Close Reclaim 的 `TESTING` / `CONFIRMED` / `CONTINUATION` 語意；P3-B 已讓
`market_bias` 改由 `semantic_pipeline.bias_state` 推導，`market_action=AVOID` 等 hard
blocker 會在 semantic pipeline 內統一覆蓋為 `BEARISH_BIAS` / `AVOID` / `BLOCKED`。
P3-C 已讓 `position_action_condition.state` 與
`final_entry_permission.state` 分別改由 `semantic_pipeline.action_state` /
`semantic_pipeline.entry_permission_state` 推導；daily invalidated / blocked / chasing risk
這類 hard gate 仍保守優先。

`decision_summary.market_bias` 是對外的多空傾向標籤（`BULLISH_BIAS` / `BEARISH_BIAS` /
`NEUTRAL_BIAS` / `REVERSAL_BIAS` / `BULLISH_CONTINUATION`），由
`decision_derived_view.semantic_pipeline.bias_state` 轉出。`market_action=AVOID` 是 semantic
hard blocker，會同步輸出 `BEARISH_BIAS`、`action_state=AVOID` 與
`entry_permission_state=BLOCKED`；若非 `AVOID`，`TESTING` / `CONFIRMED` 的收復修復語境輸出
`BULLISH_BIAS`，只有 `CONTINUATION` 才輸出 `BULLISH_CONTINUATION`（多頭延續）。
這確保 `market_bias`、`market_action`、`final_entry_permission` 三者語意一致，不會出現
「多頭延續 bias + 避開 action」或「active reclaim 還被標成反轉觀察」的矛盾輸出。

Regime 預設門檻：

| 條件 | 門檻 |
|---|---|
| `TREND_UP` | `global_trend >= 0.015` |
| `TREND_DOWN` | `global_trend <= -0.015` |
| `RANGE_BOUND` | 介於上述兩者之間 |
| `HIGH_VOLATILITY` | `global_volatility >= 0.035` |
| `LOW_CONFIDENCE` | `global_confidence < 0.45` |

這些門檻是 Decision Engine v1 的規則，不是模型訓練參數；調整時必須同步更新本文件、
Python 測試與 API 範例。

### Market Events

Decision Engine 會在 action 前先偵測 `decision_summary.market_events`：

| Event | 語意 | Action 影響 |
|---|---|---|
| `EXTREME_VOLUME` | 最新量能達極端放大門檻 | context event，不單獨決定 action；需搭配 breakdown/reclaim/reversal 解讀 |
| `HIGH_VOLUME_BREAKDOWN` | 支撐區被收盤跌破，且相對量放大或量能狀態為失敗 | 依破線 zone 嚴重度降風險；primary/main-structure 或高相關破線可強制 `EXIT`，短線非 primary 破線只降為 `REDUCE_ON_BREAKDOWN` 或 risk note |
| `INTRADAY_RECLAIM` | 日 K 支撐測試後收盤收回區間上緣 | 提升內部 event-aware entry relevance，但對外分數不混入事件修正；內部 type 保留 `INTRADAY_RECLAIM` 作相容名稱，對外 label/reason 使用 close-based 語意避免 EOD 模式誤讀為即時盤中訊號 |
| `REVERSAL_CANDIDATE` | 支撐測試未失守，且 EV / confidence 未轉弱 | 提升內部 event-aware entry relevance，作為候選反轉訊號 |

對外回傳的 `entry_relevance_score` 是不含事件修正的 base relevance，與 `zones[]` 同名欄位保持同義；事件影響另由 `market_events`、`event_state_summary`、`short_term_regime` 與 action/risk notes 呈現。

`market_events` 保留 latest candle 偵測到的完整事件序列；`event_state_summary` 則是
P2 的無資料表 lifecycle 摘要，用來區分 `CANDIDATE` / `CONFIRMED` / `ACTIVE` /
`RESOLVED` / `EXPIRED` event：

- `states`：每個 event family / zone key 的最新狀態。
- `candidates`：已偵測但尚未完成確認的事件，例如盤中跌破但收盤未跌破、或反轉候選。
- `confirmed`：已完成確認且仍有效的事件，例如收盤跌破支撐或收盤收回支撐上緣。
- `active`：目前仍有效且可進入 decision gating 的事件狀態（`CONFIRMED` / `ACTIVE` 且 `active=true`）。
- `resolved`：已被後續事件解除的狀態。
- `expired`：超過有效期或來源 zone 已失效的狀態；保留歷史但不得進入 gating。
- `active_bearish_events`：Decision hard gate 會使用的 active bearish risk。
- `market_state`：由 active event state 推導的短線市場狀態。

Lifecycle gating 與 aging 規則由 event family 統一定義，不得在 decision 欄位各自重算：

| Event family | 可進入 gating 的 state | `expires_after_bars` | resolve 條件 |
|---|---|---:|---|
| `VOLUME_CONTEXT` | `ACTIVE` | 1 | 不 resolve 其他事件，只作當根量能 context |
| `SUPPORT_BREAKDOWN` | `CONFIRMED` / `ACTIVE` | 2 | 同 zone 出現 confirmed/active `INTRADAY_RECLAIM` |
| `SUPPORT_RECLAIM` | `CONFIRMED` / `ACTIVE` | 2 | 目前不 resolve bullish event；未延續則 aging 後 `EXPIRED` |
| `SUPPORT_REVERSAL` | `ACTIVE` | 2 | candidate/confirmed 階段只作觀察，未升級前不進 gating |

同一 zone 若出現 `HIGH_VOLUME_BREAKDOWN → INTRADAY_RECLAIM → REVERSAL_CANDIDATE`，
raw `market_events` 仍保留完整鏈，但 `HIGH_VOLUME_BREAKDOWN` 在
`event_state_summary` 會變成 `RESOLVED`，不得再作為 active bearish gate 永久強制
`EXIT`。未被收復且收盤確認的 active breakdown 仍會讓
`price_path.path_state=EVENT_RISK` 並降風險；只有盤中跌破但收盤未跌破時會保留為
`CANDIDATE`，不進 `active_bearish_events`。

`REVERSAL_CANDIDATE` 只代表候選反轉，不會直接進 active bullish gate，也不會單獨解除
breakdown；必須由收盤收回上緣的 `INTRADAY_RECLAIM` 這類 confirmed/active event
觸發 resolve。

Decision gating 一律只消費 `event_state_summary.active` 與 `decision_derived_view`。P3 後，
`market_bias`、`position_action_condition.state` 與 `final_entry_permission.state` 的權威來源是
`decision_derived_view.semantic_pipeline`；primary zone 選擇、legacy market action / entry action state
與 event-aware entry relevance 仍吃 active lifecycle / derived state 作為相容與明細來源。已被
resolve 的 breakdown 不會再懲罰 relevance 或翻空 bias。完整 raw event chain 保留給對外呈現
（`market_events` / `event_sequence` / `event_state_summary`）。

跨分析延續由 Go backend 在建立新分析前讀取同 `symbol/timeframe` 最近一筆 analysis 的
完整 `market_event_states` snapshot（包含 active / resolved / expired），透過 Python
`/sr-zones` request 的 `previous_event_states` 傳入。Python 會先把 previous lifecycle states
放入 lifecycle map，再套用本次 latest candle 事件：若本次沒有新事件，前次 active breakdown
會繼續進入 `active_bearish_events`；若本次出現 confirmed/active `INTRADAY_RECLAIM`，同 zone
的 previous breakdown 會轉為 `RESOLVED`。resolved / expired state round-trip 回 Python 後
不得重新變 active，只能維持 resolved、進一步 aging 成 expired，或被同 family fresh detection
覆蓋為新的 age=0 state。

**Aging → `EXPIRED`（避免事件無限期停留在 active）**：每個 event state 帶 `age_bars`
存活計數，隨 state JSON 經 Go round-trip 累積——每被 carry 一次（一根 K 棒/分析）+1，被
當根新偵測覆蓋時歸零。carried state 的 `age_bars` 達到自身 `expires_after_bars` 即轉
`EXPIRED`、`active=false`，退出 gating；resolved state 也會 aging 成 expired，避免已解除事件
永久停留在 lifecycle snapshot。未定義 family 規則的事件套
`DEFAULT_EVENT_EXPIRES_AFTER_BARS`（預設 3），確保沒有任何 carried 事件永生。
`expires_after_bars` 與 `age_bars` 由 Go `scoreZonesPreviousEventStates` 從 `state_json`
帶回（缺 `expires_after_bars` 時送 `null` 讓 Python 套預設，而非誤傳 0 造成立即過期）。

例外（刻意）：`_daily_candidate_zones` 與 `_defense_lines` 仍消費完整 raw `market_events`——
前者用「歷史上出現過 `INTRADAY_RECLAIM` / `REVERSAL_CANDIDATE`」決定是否補日 K 候選區，後者用最近
微結構事件的 zone_ref 定位戰術防守線；兩者是「候選區產生」與「防守線呈現」，不是進場 gating，需要
完整事件脈絡才完整，故與「gating 只吃 active」並存而不矛盾。

Daily candidate zone 若 `price_low == price_high`，不再以 `2405.00 ~ 2405.00` 這類零寬區間呈現，
而改成 trigger 語意：resistance 端輸出 `zone_kind=BREAKOUT_TRIGGER`、support 端輸出
`zone_kind=BREAKDOWN_TRIGGER`，並填 `trigger_price` 與 `BREAKOUT_TRIGGER 2405.00` /
`BREAKDOWN_TRIGGER 2405.00` label。`price_low` / `price_high` 保留作舊 payload 相容；trigger
不可直接成為 `best_trade_zone`。

### Daily Price Action / Data Quality

`decision_summary.daily_price_action` 使用最新日 K OHLC 與前一日收盤建立 EOD 判讀。現階段會輸出
`close_location`、`range_pct`、`gap_state`、`follow_through_state`、
`price_follow_through_state`、`momentum_confirmation_state`、
`reclaim_rejection_state`、`lower_wick_ratio`、`upper_wick_ratio`，以及
`body_proxy_ratio`、`body_ratio`、`body_ratio_source`。`body_proxy_ratio` 固定代表「前一日收盤
到當日收盤」相對當日 high/low range 的近似值；`body_ratio` 在 evidence/frame 傳入
daily open 時使用 `abs(close - open) / (high - low)`，且 `body_ratio_source="DAILY_OPEN"`。
若呼叫端未傳入 daily open，`body_ratio` 會退回 `body_proxy_ratio`，並標示
`body_ratio_source="PREVIOUS_CLOSE_PROXY"`。

`follow_through_state` 保留 legacy 相容欄位；新判讀應優先看
`price_follow_through_state` 與 `momentum_confirmation_state`，用來區分「價格延續」與
「動能是否確認」。當 active reclaim 已存在但 `price_follow_through_state=NO_PRICE_FOLLOW_THROUGH`
時，`decision_derived_view.daily_reason_codes` 會輸出 `WAIT_PRICE_FOLLOW_THROUGH`；
`daily_confirmation` 與 `final_entry_permission` 可維持 `PROBE_ALLOWED`，但 reason code
必須清楚表示「RR 通過，仍等待價格延續／動能確認」。

`decision_summary.data_quality.features` 會把缺資料、中性資料與負向資料分開，不把 missing 視為
neutral，也不把 neutral 視為 bearish。籌碼 `chip_summary.score` 使用 `chip_scores.total_score`
的 `-100~100` 值域，門檻對齊 `CHIP_SIGNAL_THRESHOLD=20`：

| Chip score | interpretation |
|---|---|
| `< -20` | `NEGATIVE` |
| `-20..20` | `NEUTRAL` |
| `> 20` | `POSITIVE` |

feature status 可判定 `AVAILABLE` / `MISSING` / `STALE` / `INVALID`。`STALE` 依
`data_quality_metadata.updated_at` 與 `analysis_as_of` 判斷，預設容忍 1 天；`INVALID` 來自
`data_quality_metadata.validation_errors` 或 OHLC 基本值域檢查（例如 high < low、close 不在
high/low 區間）。每個 feature 會保留 `updated_at` 與 `reason_codes`，前端會將
`stale_features`、`invalid_features` 與 missing/neutral/negative 分開顯示。
另外 `market_data_completeness`、`rr_completeness`、`trade_qualification_completeness`
會拆開市場資料完整度、RR 資料完整度與交易資格完整度；legacy `overall_completeness`
仍保留為市場資料完整度相容欄位。

Chip missingness 會拆成 raw score 與 effective impact：`chip_summary.score` / `raw_score`
表示依可用分量重新正規化後的籌碼方向（present 分量的加權平均 `weighted_sum / available_weight`），
`coverage` / `confidence` 表示可用分量權重（四子分量權重和為 1.0，`coverage` = 有值分量的權重和），
`effective_score` 表示 `raw_score * coverage`（實作即 `weighted_sum`）後的實際影響強度。Decision Engine 的
`data_quality.chip_coverage` 讀 `coverage`，前端文案應呈現「方向」與「覆蓋率」兩件事，
避免把低覆蓋率下的 effective impact 誤寫成「籌碼中性」。

**`effective_score` 一律依 coverage 降權，不採用 DB `total_score`**：DB `total_score` 未依覆蓋率降權，
低覆蓋率時直接回傳會高估籌碼實際影響（`effective` 與滿覆蓋的 `score` 重複、低覆蓋時誤導），故
`_build_chip_summary` 只用 `weighted_sum`。邊界情況：chip row 存在但四子分數全為 None（`total_score`
仍可能有值）時，`raw_score` / `score` / `effective_score` 皆為 `None`、`coverage=0.0`、`confidence_level=NONE`，
但 `missing=False`（保守：無可用分量時不回報未降權的 `total_score`，也與「查無 chip 資料」的 `missing=True`
區分）。前端須把此情況視為「有籌碼資料但分量不足以評分」，不可當成中性訊號。

### Decision Zone Scores / Lifecycle

`decision_summary.*_zone` 會保留 legacy score 欄位，並新增語意拆分欄位：

| 欄位 | 語意 | 目前來源 |
|---|---|---|
| `structural_score` | 區間本身品質 | `zone_quality_score` |
| `decision_relevance_score` | 當下決策相關性，不含 market event 修正 | `entry_relevance_score` |
| `tradability_score` | legacy 可交易綜合分 | `trading_score` |

Zone lifecycle 目前由 deterministic EOD rule 產生，可能值包含 `CANDIDATE`、`CONFIRMED`、
`VALIDATED`、`WEAKENING`、`BROKEN`、`INVALIDATED`。這是 decision summary 的解讀欄位，
不改寫原始 `zones[]` 的驗證結果。

每個 decision zone 另輸出 `zone_width_pct`（區間寬度佔現價比例）與 `zone_width_penalty`
（寬區間的距離評分懲罰）。`zone_width_penalty` 會併入 nearest decision 的距離評分（`price_path`
的 `nearest_support_zone` / `nearest_resistance_zone` 選擇），避免過寬、模糊的區間僅因中心價較近
就勝過較窄、較精確的關鍵價位。

### Semantic Pipeline 與 Legacy Action

P3 後，對外交易語意的權威鏈路是：

```text
Event -> Lifecycle -> Market State -> Bias -> Action -> Entry
```

對應欄位為 `decision_derived_view.semantic_pipeline`：

| 欄位 | 語意 | 典型值 |
|---|---|---|
| `event_signal` | 最新事件語意 | `CLOSE_RECLAIM`、`SUPPORT_TEST`、`CLOSE_BREAKDOWN` |
| `lifecycle_phase` | 事件成熟度 | `TESTING`、`CONFIRMED`、`CONTINUATION`、`BREAKDOWN` |
| `market_state` | 市場狀態 | `BULLISH_RECOVERY`、`BULLISH_CONTINUATION`、`REVERSAL_CANDIDATE`、`BREAKDOWN_RISK` |
| `bias_state` | 對外多空傾向來源 | `BULLISH_BIAS`、`BULLISH_CONTINUATION`、`REVERSAL_BIAS`、`BEARISH_BIAS` |
| `action_state` | 持有者語意來源 | `CONDITIONAL_HOLD`、`HOLD`、`DEFEND_BREAKDOWN`、`AVOID`、`WATCH` |
| `entry_permission_state` | 未持有者進場權限來源 | `PROBE_ALLOWED`、`ENTRY_ALLOWED`、`WAIT_CONFIRMATION`、`BLOCKED` |

#### 分層原則：lifecycle 不看 RR（2026-08-13 起）

`lifecycle_phase` 由獨立的 **Lifecycle Engine**（`lifecycle_engine.py`）判定，
它的職責只有一件事——**依 Event 的演進決定目前處於哪一個階段**。

**它的函式簽章裡沒有 `rr_gate`，這是刻意的，請不要加回去。**
風險報酬比是進場與策略條件，不是事件事實。原本 `CONTINUATION` 的判定含
`rr_gate.qualified`，導致同一段價格行為在 RR 不合格時被判成 `CONFIRMED`、
合格時才是 `CONTINUATION`——**策略條件改寫了事件事實**，於是「現在處於什麼階段」
無法被獨立回答。要維持保守度應該由 Decision Engine 用 RR Gate 去擋。

`CONTINUATION` 現在只要求三個**價格證據**同時成立：價格跟進、動能確認、
明確突破 zone 上緣（×1.03）。

**注意這裡有四套同名不同義的「lifecycle」**，判讀時要先確認在講哪一個：

| 名稱 | 位置 | 語意 |
|---|---|---|
| `LIFECYCLE_*` | `event_engine.py` | **單一事件**的生老病死（`CANDIDATE`/`ACTIVE`/`RESOLVED`/`EXPIRED`） |
| `lifecycle_phase` | `lifecycle_engine.py` | **整體事件演進**（本表格這個） |
| `zone_health_state` | `decision_engine.py` | **zone 本身**的健康度（舊鍵 `lifecycle` 已 deprecated） |
| `_zone_state` | `scenario_engine.py` | **場景判定**（`SUPPORT_RETEST`/`RETEST_REQUIRED`…） |

#### 已知並接受的行為改變（待 replay 評估）

RR 解耦讓 `CONTINUATION` 的涵蓋範圍變寬，**其中一條會放寬持有建議**：

| 情境 | 舊 | 新 |
|---|---|---|
| 收復已確認（structure `SUPPORT_RECLAIM_CONFIRMED` 或撐過一根 K 棒）＋ 三項價格證據 ＋ RR 不合格 | `CONFIRMED` → `HOLD` | `CONTINUATION` → `HOLD`（不變） |
| **收復尚未確認**（`SUPPORT_RECLAIM_CANDIDATE`、`age_bars=0`）＋ 三項價格證據 ＋ RR 不合格 | `TESTING` → `CONDITIONAL_HOLD` | **`CONTINUATION` → `HOLD`** |

第二列的 `action_state` 會被 `_position_action_condition` 原樣採用，
所以 `position_action_condition.state` 由「條件性續抱」變成「續抱」。
**進場那條線沒有放寬**——`entry_permission_state` 的 `elif not rr_qualified: BLOCKED`
排在 `ENTRY_ALLOWED` 之前，RR 不合格時進場一律仍是 `BLOCKED`。

**目前刻意接受這個放寬**（2026-08-13 決定），理由是沒有乾淨的還原方式：
舊行為的保守度來自「RR 失敗時掉回確認層級判斷」這個**順序副作用**，而抽離後那個區分
已被收斂掉；硬加 RR gate 會讓第一列原本就該是 `HOLD` 的樣本變成 `CONDITIONAL_HOLD`，
成為另一個方向的回歸。**待 decision replay 累積足夠資料後再評估**——
而那依賴「有排程定期產生 SR 分析」，見 [`todo.md`](./todo.md) T-045 的前置條件。

`semantic_pipeline.version` 已由 `decision-semantic-pipeline-p3` 升為 `p4`，
所以 `stock_sr_decisions` 裡的資料可以從版本字串分辨是改前還是改後產生的。

標準 Close Reclaim 閱讀路徑：

| 情境 | Semantic path |
|---|---|
| 收盤收復當日 | `Close Reclaim -> TESTING -> BULLISH_RECOVERY -> CONDITIONAL_HOLD -> PROBE_ALLOWED` |
| 隔日仍守住 | `Close Reclaim -> CONFIRMED -> BULLISH_RECOVERY -> HOLD -> PROBE_ALLOWED` |
| 明確突破延續 | `Close Reclaim -> CONTINUATION -> BULLISH_CONTINUATION -> HOLD -> ENTRY_ALLOWED` |

`position_action_condition.state` 讀 `semantic_pipeline.action_state`；legacy
`decision_derived_view.position_gate_state` 僅為相容 alias，也等於 `semantic_pipeline.action_state`，
不得再作為獨立推導來源（前端型別 `SRDecisionDerivedView.position_gate_state` 已標 `@deprecated`，
請改讀 `semantic_pipeline.action_state`）。若無 semantic pipeline（理論上只會出現在手動呼叫或舊資料
相容路徑），`position_action_condition.state` 保守回 `WATCH`。`final_entry_permission.state` 讀
`semantic_pipeline.entry_permission_state`，但 daily `INVALIDATED` / `BLOCKED` /
`CHASING_RISK` 仍保守優先。`market_bias` 讀 `semantic_pipeline.bias_state`。

`entry_permission_state` 的前方壓力判定使用 entry 層 `blocking_zone_ahead`，其計算刻意排除
daily candidate zones（只看真實 scored zones），避免較弱的日 K 候選區擋住進場；path 層
`path_gate_state=BLOCKING_ZONE_AHEAD` 則仍含 daily candidate zones。兩者是不同層級、可各自成立。

Legacy `action` 是整份分析的相容操作結論，避免舊前端自行從多張 zone 卡片拼湊結果。目前限定四種：

| Action | 語意 | 典型條件 |
|---|---|---|
| `Buy` | 條件完整，可正常部位進場 | regime 偏多或區間下緣、primary support 高信心、EV/RR 正向、波動與風險可控 |
| `BuySmall` | 條件偏正向但仍有明顯保留 | 偏多但波動高、confidence 未達很高、EV/RR 普通、或距離主支撐略遠 |
| `Hold` | 不追價，等待更好的價格或確認 | 方向不差但現價不在合理風險報酬位置、primary zone 不夠近、或支撐/壓力訊號混合 |
| `Avoid` | 不建議操作 | regime 偏空、primary zone 失效、低信心且高波動、EV/RR 明顯不佳、或價格接近強壓但缺乏突破證據 |

Legacy action 應由 Market Regime、primary zone、`entry_relevance_score`、market events、
`zone_quality_score`、`confidence`、`expected_value`、`risk_reward_ratio`、`chip_summary` 與風險條件共同決定。
`trading_score` 保留為 legacy quality score，不得單獨直接決定 `Buy` / `BuySmall`。若任一核心資料缺失，
預設應保守降級，例如 `Buy` 降為 `BuySmall`，`BuySmall` 降為 `Hold`。

`position_action_condition.state` 是持有者語意來源：`CONDITIONAL_HOLD` 表示條件式持有，
`HOLD` 表示事件已確認但仍需搭配防守線管理，`DEFEND_BREAKDOWN` 表示優先防守。前端應列出
`invalidation_price`（防守線）、`recovery_price`（回穩線）與 `reason_codes`，不可只看 legacy
`position_action=HOLD`。
`decision_derived_view.position_reason_codes` 是部位防守/價位背景 context，例如
`POSITION_SUPPORT_DEFENSE`、`POSITION_RESISTANCE_OVERHEAD`、`POSITION_RECLAIM_DEFENSE`；
它們不是另一套 action state，也不得覆蓋 `semantic_pipeline.action_state`。

`entry_action_state` 是 legacy 進場階段明細，不取代 `final_entry_permission`：

| State | 語意 |
|---|---|
| `BLOCKED` | 禁止進場；硬性風險或條件失效 |
| `WAIT_CONFIRMATION` | 等待確認，不進場 |
| `PROBE_ENTRY` | 仍在待確認語境，只能視為觀察性試探 |
| `SMALL_ENTRY` | 條件普通但已確認，可小量進場 |
| `ACCUMULATE` | 條件完整但仍適合分批累積 |
| `BUY` | 條件完整，可正常買進 |

若 zone 是 `PENDING_VALIDATION` 或 position reason 含 `SUPPORT_RECLAIM_AWAIT_CONFIRMATION`，
即使 legacy `action=BuySmall`，`entry_action_state` 也不得高於 `PROBE_ENTRY`。P3 後，
`final_entry_permission.state` 可由 semantic pipeline 輸出 `PROBE_ALLOWED`，代表未持有者僅允許觀察性試探，
不是正式進場。

`final_entry_permission` 是 `semantic_pipeline.entry_permission_state` 的對外進場權限輸出，
並合併 `decision_derived_view.final_entry_reason_codes` 作為可追溯理由；前端若要顯示
「是否允許進場」應優先讀此欄位；legacy `entry_action_state` / `daily_entry_state` 保留給明細與相容。
`final_entry_permission.state` 不再輸出 `NO_SETUP` 或空語意；若 daily confirmation 為 `INVALIDATED`
或 `BLOCKED`，final permission 會降為 `BLOCKED`；若 daily confirmation 為 `CHASING_RISK`，
final permission 會降為 `WAIT_CONFIRMATION`。`ENTRY_ALLOWED` 只在 semantic pipeline 判定
`CONTINUATION`、RR 合格且前方沒有既有 SR 壓力區擋道時輸出；若前方有既有 SR 壓力區，
未持有者進場權限降為 `WAIT_CONFIRMATION` 並附上 `BLOCKING_ZONE_AHEAD`。`TESTING` /
`CONFIRMED` 的收復修復語境通常只輸出 `PROBE_ALLOWED`，避免把持有者的 `HOLD` 誤解成
未持有者可正式進場。

Final Entry Arbitration P0/P1 後，對外進場仲裁順序固定為：

```text
Final Entry
> Executability
> Blocking Zone
> Model Health
> Daily Entry State
> Historical Zone RR
```

`final_entry_permission.state` 對外只輸出四種權限：`BLOCKED`、`WAIT_CONFIRMATION`、
`PROBE_ALLOWED`、`ENTRY_ALLOWED`。Legacy `entry_action_state` 的 `BUY` / `ACCUMULATE` /
`SMALL_ENTRY` / `PROBE_ENTRY` 只作為內部輸入階段，進入 final entry 後會正規化成上述四種權限。

`entry_executability` 會明確輸出 `entry_price`、`entry_zone_lower`、`entry_zone_upper`、
`tolerance`、`executable_now`、`reason_code` 與 `price_basis`。`tolerance` 目前為
`max(current_price * 0.002, zone_width * 0.1)`。回測支撐語境使用
`price_basis=PRIMARY_SUPPORT_UPPER`，現價高於 support entry zone upper + tolerance 時
`executable_now=false`，`final_entry_permission.state` 降為 `WAIT_CONFIRMATION` 並附上
`ENTRY_ZONE_OVERSHOT`，`best_trade_zone` 不可輸出。收復／修復／延續語境不再用 primary support
upper 判斷追價，而是改用 `price_basis=RECLAIM_CLOSE` 或 `CONTINUATION_MARKET_PRICE`；
這類語境的 chasing risk 由 daily confirmation 的 `CHASING_RISK` 處理。若現價低於 support
entry zone lower - tolerance，`executable_now=false` 並輸出 `ENTRY_ZONE_UNDERSHOT`。

`entry_executability.reason_code` 目前包含：`NO_ENTRY_ZONE`、`ENTRY_ZONE_NOT_SUPPORT`、
`EXECUTABLE_NOW`、`ENTRY_ZONE_OVERSHOT`、`ENTRY_ZONE_UNDERSHOT`。Final entry 另可能因模型健康度
輸出 `MODEL_ENTRY_BLOCKED`、`MODEL_ENTRY_CAPPED`。

`entry_blocking_zone` 是進場層前方壓力 gate，只看尚未失效的 scored resistance zone，不使用
daily candidate zone。若最近 scored resistance 距離小於 proxy 門檻（目前
`max(zone_width * 0.5, current_price * 0.005)`，`threshold_basis=ZONE_WIDTH_OR_0_5_PERCENT_PROXY`），
`blocked=true`，`final_entry_permission.state` 降為 `WAIT_CONFIRMATION` 並附上
`NEAR_RESISTANCE_BLOCKING_ENTRY`。輸出欄位同時包含價格單位與比例單位：
`distance_price` / `threshold_price` 是價格差，`distance_pct` / `threshold_pct` 是除以現價後的比例。
舊欄位 `distance_to_nearest_resistance` 與 `threshold` 暫時保留為比例值相容 alias，前端新顯示應改讀
`distance_pct` / `threshold_pct` 或價格欄位。`price_path.blocking_zone` 仍是路徑提示，可包含 daily
candidate；semantic entry gate 的 `BLOCKING_ZONE_AHEAD` 使用 entry 層近壓力判斷，path 層則仍可用
完整路徑提示描述前方壓力。

`action` / `market_action` 是 final entry 對齊後的相容輸出，不得高於
`final_entry_permission`。若 final entry 為 `WAIT_CONFIRMATION`，即使 legacy
`entry_action_state` 是 `SMALL_ENTRY` / `ACCUMULATE`，對外也輸出 `market_action=WATCH` 與
`action=Hold`；`best_trade_zone` 只在 final entry 為 `PROBE_ALLOWED` / `ENTRY_ALLOWED`、
`executable_now=true`、entry blocking 未阻擋、RR 合格，且 entry 不是
`RECLAIM_CLOSE` / `CONTINUATION_MARKET_PRICE` 這類市價型語境時輸出。市價型語境不得把
historical support zone 當成 `best_trade_zone`，避免 `best_trade_zone` 與 `entry_price` 互相矛盾；
前端應改讀 `entry_executability` 與 `rr_context`。`risk_notes` 需在 final entry 降級後改寫舊的
進場語氣註記並保留原因，避免 legacy action 與對外操作語氣不一致。改寫由 **reason code 驅動**，
不以中文文案子字串比對：`_decision_action` 對會被改寫的註記帶結構化 code（`MODEL_DEGRADED_ENTRY_TONE`、
`RR_BELOW_FULL_ENTRY`），final entry 為 `WAIT_CONFIRMATION` / `BLOCKED` 時依 code 換成保留原因的
保守句（例如「風險報酬比未達完整買進門檻，Final Entry 需保守觀察。」），最後才統一轉回純字串輸出；
因此改文案不影響改寫邏輯，對外 `risk_notes` 仍是 `string[]`。

Legacy action pipeline：

1. 若沒有 primary zone：`Hold`，並加入「沒有足夠明確主交易區」風險註記。
2. 若 primary zone 距離現價超過 8%：保留 action 判斷，但加上不追價風險註記。
3. 若 primary zone `risk_reward_ratio < 1.0`：加上風險報酬不足註記。
4. 若 primary zone `recent_validation=EXPIRED`：加上近期驗證失效註記，且不應升級到 `Buy`。
5. 若出現 `HIGH_VOLUME_BREAKDOWN`，依破線 zone 的 tier / 是否 primary / entry relevance / 距離分級：主結構或高相關破線可 `Avoid` + `EXIT`；短線非 primary 破線降為 `Watch` + `REDUCE_ON_BREAKDOWN` 或只加 risk note。若同一 primary zone 已進入 `SUPPORT_RECLAIM_CONFIRMED`，舊 breakdown 不再強制 `EXIT`。
6. 若 regime 偏空，或 primary zone 是 resistance 且沒有 bullish setup：`Avoid`。
7. 若符合 strong setup：`Buy`。
8. 若符合 constructive setup：`BuySmall`。
9. 其他情況：`Hold`。

Setup 定義：

| Setup | 條件 |
|---|---|
| bullish setup | `market_regime.primary in (TREND_UP, RANGE_BOUND)` 且 primary zone 是 `SUPPORT` |
| bearish setup | `market_regime.primary == TREND_DOWN` 或 primary zone 是 `RESISTANCE` |
| strong setup | primary zone event-aware entry relevance `>= 75`、`confidence >= 0.65`、`expected_value > 0`、`risk_reward_ratio >= 2.0`、距離現價 `<= 5%`、且沒有 regime flags |
| constructive setup | bullish setup 且 primary zone event-aware entry relevance `>= 55`、`confidence >= 0.45`、`expected_value >= 0` |

若 `expected_value` 或 `risk_reward_ratio` 缺失，不得判定 strong setup；缺失值只允許進入
`Hold`、`BuySmall` 的保守觀察語境，除非之後另有明確規則。

### Reclaim 狀態

支撐收復不得用單根 K 直接宣告確認：

| State | 條件 |
|---|---|
| `SUPPORT_RECLAIM_CANDIDATE` | 最新 K 的 low 進入支撐 zone，且 close 收回 `zone.high` 上方；這只代表候選，不代表確認 |
| `SUPPORT_RECLAIM_CONFIRMED` | 前一根 K 已收回 `zone.high` 上方，且最新 K 沒有重新收破 `zone.low` |
| `SUPPORT_RECLAIM_INVALIDATED` | 最新 K 收破 `zone.low`，支撐收復失效 |
| `BREAKDOWN` | primary zone 近期驗證已失效或結構性跌破 |

這些狀態只影響 Decision/Scenario 與 Position Action 的條件呈現，不改寫 zone 的原始機率、EV/RR 或 score。
原始 `recent_validation` 仍由 scoring / zone builder 的 touch classification 與 validation window
決定；Decision Summary 只輸出 reclaim evidence，不應把 `PENDING_VALIDATION` 直接改寫成已驗證。
`validation_debug` 會揭露 `analysis_date`、`zone_generation_end_date`、`validation_start_date`、
`validation_end_date`、`latest_touch_bar_date` 與 `latest_validation_bar_date`。**此
`zone.formed_at_index + 1` 過濾只作用於 validation window（`recent_validation` 與 `validation_debug`
統計）**，確保 validation window 符合 `zone_generation_end_date < validation_bar_date <= analysis_date`；
`_filter_validation_touches` 只過濾餵給 `recent_validation` 的觸碰。ML 特徵層的 `touch_count`、
`breakout_count`、`rejection_count` 與 `confidence`（sample/recency/stability）仍使用完整 lookback
窗口（`as_of_index - lookback_bars + 1 .. as_of_index`，含 zone 形成前的觸碰），這是為了讓訓練與即時
評分的特徵定義一致，屬刻意設計，不套用 `formed_at_index + 1` 過濾。

### Defense Lines

`decision_summary.defense_lines` 提供三層防守線：

| 層級 | 來源 | 用途 |
|---|---|---|
| `tactical` | 最近 market event 對應 zone，或最近 `TIER_3_SHORT_TERM` microstructure zone | 短線觀察、reclaim/breakdown 風險提示 |
| `swing` | `primary_zone` | 預設交易防守線，`position_action_condition.invalidation_price` / `recovery_price` 仍以此層為相容 alias |
| `strategic` | `TIER_1_MAIN_STRUCTURE` 中 quality / confidence 較高的主結構 zone | 中長線結構防守與風險定位 |

`position_action_condition` 保留舊欄位，前端可優先顯示 `defense_lines`，舊資料或舊前端則繼續讀
`invalidation_price` / `recovery_price`。

`rr_context` 將新進場 RR 與既有部位 RR 拆開：`entry_rr` 仍用於 `rr_gate` / trade candidate，
`position_rr` 保留為持股語境欄位，但 SR Zone 是市場結構層，不讀使用者持倉成本；因此
SR `decision_summary.rr_context.position_rr` 維持 `null` 且
`position_rr_source=UNAVAILABLE`，避免把 entry RR 誤讀成既有部位 RR。實際持倉
Position RR 由 Go Position Engine 在 `position_analyses.evidence.position_decision.position_rr`
輸出，來源標示為 `POSITION_AVG_COST`。

P0/P1 後，`rr_context` 也輸出 entry RR 的顯示基礎：`entry_price`、`entry_zone_lower`、
`entry_zone_upper`、`stop_price`、`target_price`、`price_basis`、`stop_basis`、`target_basis`、
`structural_stop_price`、`risk_price`、`reward_price`、`stop_distance_pct`、`execution_rr`、
`execution_rr_source`、`executable_now`、`entry_executability_reason_code` 與
`rr_formula_available`。試單 RR 的 stop 優先使用 `defense_lines.tactical` 的 support invalidation
price（`stop_basis=TACTICAL_STOP`）；若沒有 tactical support，且 primary / swing line 是 support，
才 fallback 到 primary zone stop。Resistance primary 不得輸出 long entry 的反向停損。
`structural_stop_price` 僅作結構防守參考，不作為試單 stop。若 `price_basis` 是 `RECLAIM_CLOSE`
或 `CONTINUATION_MARKET_PRICE`，`entry_rr` 僅是 historical zone statistic；實際 final RR gate 改讀
`execution_rr`。市價型 entry 的 target 取 **entry price 之上最近的有效 resistance** 的 `price_low`
（`target_basis=NEAREST_RESISTANCE_TARGET`）：先以「`price_low > entry_price` ＋ 排除 `EXPIRED` ＋
排除 LOW confidence」嚴格挑最近者，落空再退到「排除 `EXPIRED`、允許 LOW confidence」，但**不回退到
含 `EXPIRED`**（與 `entry_blocking_zone` 的 EXPIRED 過濾一致）。不採「取最近 resistance 再事後過濾
方向」，以免被 entry 之下、已被跨越但仍標為 resistance 的區擋掉、錯過上方真正壓力。若沒有可量化 target，
`target_price=null`、
`target_basis=MARKET_ENTRY_TARGET_UNAVAILABLE`、`rr_formula_available=false`、`target_known=false`；
這代表 target unknown，不代表 RR 不合格，因此不會單獨把 final entry 降為 `WAIT_CONFIRMATION`。
只有 target 已知且 `execution_rr < minimum_rr`（`EXECUTION_RR_INSUFFICIENT`）才會降級。

`rr_gate` 是 final/execution gate 的對外結果。市價型 entry 會輸出 `gate_basis`、
`zone_actual_rr` 與 `target_known`：`zone_actual_rr` 保留 historical zone statistic 供對照，
`actual_rr` 則是可量化時的 execution RR；若 `target_known=false`，`actual_rr=null` 但
`qualified=true`，由 `risk_notes` 提醒 target 尚未量化。

`price_path.next_decision_source` 只描述下一個決策價位來源。拆分 nearest support / resistance 後，
有效值為 `nearest_support_zone`、`nearest_resistance_zone` 或 `daily_candidate_zone`；
`nearest_decision_zone` 欄位仍保留為摘要相容欄位，但不再作為 `next_decision_source`。
`price_path.blocking_zone` 可能來自完整 zone pool，而不一定存在於前端 selected summary zones。
因此會輸出 `source_scope`、`method`、`timeframe`、`tier`、`tier_label`、`confidence`、
`confidence_level`、`distance_pct`、`zone_id` 與 `selected_summary_zone` 等 metadata，讓使用者能追溯
blocking zone 的來源。**目前限制**：`zone_id` 恆為 `null`（`ZoneScore` 尚無 id 欄位可對應），`timeframe`
只有 daily candidate 路徑會填 `"1d"`、`ZONE_SCORE_POOL` 路徑恆為 `null`；`tier`/`tier_label`/`confidence`/
`confidence_level` 在 daily candidate 路徑也為 `null`（daily 候選無 tier/confidence）。前端應把這些欄位
視為 nullable，不可假設一定有值。

### Primary Zone 與 Secondary Zones

目前 `zones` 主要依 tier 與 `trading_score` 排序，適合完整清單，但不一定等於「現在最值得操作的主交易區」。`decision_summary.primary_zone` 應只挑一個目前最具決策意義的 zone，其他放入 `secondary_zones` 或前端展開明細。

篩選與排序：

1. 先排除 `role=AT_ZONE`、`recent_validation=EXPIRED`、`confidence_level=LOW`、距離現價過遠、EV/RR 缺失或不合理的 zone；若全部被排除，允許退而選擇最接近且風險可解釋的觀察區，action 通常不應高於 `Hold`。
2. 候選 zone 依摘要專用分數排序，而不是完全沿用 `trading_score`。摘要分數以 event-aware entry relevance 為主，搭配 zone quality、RR、confluence 與 regime/role alignment。
3. 優先挑與 action 方向一致的 zone。`Buy`/`BuySmall` 應優先挑 support；`Avoid` 在偏空情境可挑主要 resistance 或失效支撐作為風險來源；`Hold` 可挑最近的等待區。
4. `secondary_zones` 只保留摘要必要資訊，例如 `zone_id`、角色、價位區間、距離現價、簡短原因與是否可展開，不重複整份 market context。

Primary zone 候選池：

- 預設排除 `role=AT_ZONE`。
- 預設排除 `recent_validation=EXPIRED`。
- 預設排除 `confidence_level=LOW`。
- 預設排除 `expected_value is None`。
- 若全部被排除，退回只排除 `AT_ZONE` 的保守候選池；此時 action 通常不應高於
  `Hold`，除非後續規則明確放寬。

目前 primary-zone ranking 分數：

```
summary_score =
  event_aware_entry_relevance/100 * 0.65
  + zone_quality_score/100 * 0.15
  + rr_score * 0.07
  + confluence_family_score * 0.08
  + role_alignment * 0.05
```

`event_aware_entry_relevance` 是 base `entry_relevance_score` 加上 market event 內部修正後的分數，只用於 decision gating / primary-zone ranking；對外仍回傳 base relevance。`role_alignment=1.0` 僅在 regime 與角色方向一致時成立：`TREND_UP/RANGE_BOUND + SUPPORT` 或 `TREND_DOWN + RESISTANCE`。Distance、EV、confidence 已包含在 entry relevance 裡，不再額外重複加權；`rr_score` 以 3.0 為滿額上限；`confluence_family_score` 使用去重後的獨立 evidence family count，而不是 raw zone count。

### Market Context 去重

Market Context 放股票層級或整份分析共用的理由，zone 卡片只保留 zone 獨有理由。

Market Context 建議包含：

- 整體趨勢：`global_trend`、MA 位置、趨勢方向。
- 整體波動：`global_volatility`、是否高波動、區間寬度是否偏大。
- 整體區間辨識信心：`global_confidence` 與資料完整度。
- 籌碼摘要：`chip_summary` 的方向、分數、缺漏狀態。
- 模型狀態：`model_version`、`model_config_hash`、訓練時間或資料不足提示。
- 共同風險：例如高波動、低信心、價格離主交易區太遠、籌碼缺漏。

Zone 卡片只保留：價位區間、距離現價、role、bounce/break 機率、EV/RR、recent validation、volume confirmation、confluence、該 zone 的 Zone Confidence（區間辨識信心）與該 zone 獨有原因。這能避免每張卡片重複描述趨勢、籌碼或模型狀態。

### Zone Confidence 透明化

`confidence` 的產品名稱是 Zone Confidence／區間辨識信心，代表這個價格區間被辨識為有效支撐/壓力的可靠度；它不是反彈勝率。反彈/守住機率與跌破/突破機率必須以 `bounce_probability`、`break_probability` 分開顯示。

前端預設顯示 `79%（高）` 這類人可讀格式，展開後顯示因子貢獻。第一版應先忠實揭露目前既有公式內因子：

```json
"confidence_explanation": {
  "value": 0.79,
  "level": "HIGH",
  "label": "79%（高）",
  "formula_factors": [
    { "key": "sample_factor", "value": 0.71, "label": "樣本數", "description": "觸碰次數越多越可靠" },
    { "key": "recency_factor", "value": 0.84, "label": "近期性", "description": "越近期驗證過越可靠" },
    { "key": "stability_factor", "value": 0.82, "label": "穩定度", "description": "歷史守住/跌破結果越一致越可靠" }
  ],
  "context_factors": [
    { "key": "volume_confirmation", "effect": "supportive", "label": "量能確認" },
    { "key": "confluence", "effect": "supportive", "label": "多方法共振" },
    { "key": "chip_missing", "effect": "warning", "label": "籌碼資料缺漏" }
  ]
}
```

`formula_factors` 必須只放真正進入 confidence 公式的因子；`context_factors` 只是輔助解釋，不得讓使用者誤以為量能、籌碼或 confluence 已經直接改變 `confidence`。若未來要把這些因素納入 confidence 公式，必須同步更新公式、文件、API 範例與測試。

### Python / Go / Frontend 合約

| 層 | 責任 |
|---|---|
| Python | Decision Engine 只在 Python 端計算，產生 `decision_summary`；不得依賴 Go 或前端再補決策 |
| Go | `analysis.Client` 與 `SRZoneRepo` 只做 JSON passthrough、保存與回傳；不得重新解釋 action |
| Frontend | 前端只負責呈現與空值保護；不得在 UI 端重新挑 primary zone 或覆寫 action |

舊分析可能沒有 `decision_summary` 或值為 `null`，前端必須安全降級為只顯示 zones 與
period summaries。新分析若 `decision_summary` 缺欄位，應視為 Python contract 失敗，
由 Python 單元測試先攔下。

前端現況：`SRZones.svelte` 已呈現 `final_entry_permission`（權威進場閘，優先於 legacy
`entry_action_state` / `daily_entry_state`）、`rr_context`（`entry_rr` / `position_rr`，
`position_rr` 為 null 時顯示「—」並標示尚未接入持股防守區）、daily price action 的
`price_follow_through_state` / `momentum_confirmation_state`、`nearest_support_zone` /
`nearest_resistance_zone`（取代舊單一 `nearest_decision_zone` 的顯示位置）與
`short_term_regime`；別名欄位 `tactical_regime` / `structural_trend` 不另外呈現，避免重複。

### 測試與驗收

Decision Engine v1 至少需要以下測試：

| 測試 | 預期 |
|---|---|
| 多頭 + 高品質近距離支撐 | action=`Buy`，primary zone 為該支撐 |
| 多頭 + 高波動 | action 不高於 `BuySmall`，risk_notes 含高波動 |
| 區間盤 + 支撐條件普通 | action=`BuySmall` 或 `Hold`，不可直接 `Buy` |
| 偏空 + resistance primary | action=`Avoid` |
| 無合格 primary zone | action=`Hold`，primary_zone 為 null 或保守觀察區 |
| low confidence / expired zone | 不得成為第一候選，除非所有嚴格候選都被排除 |
| EV/RR 缺失 | 不得輸出 `Buy` |
| chip missing | market_context 或 regime reasons 必須揭露籌碼缺漏 |

Go 測試需確認 `decision_summary` 從 Python response 到 DB/API 完整 round-trip。
Frontend 驗收需確認 `decision_summary=null` 的舊資料不會造成頁面錯誤。

---
## 設計原則

1. **可解釋**：每個指標都要有明確公式與資料來源（本文件即是依此原則撰寫）。
2. **不互相矛盾**：score 是機率的單調函式（見「五」），EV/RR 用同一套
   `average_bounce_return`/`average_break_return`，Trading Score 的每個分量
   都用同一份 zone 的其他欄位算出，不會有互相打架的結論。
3. **可回測**：所有機率來自 `sklearn` 訓練的模型，`rr_reference` 存在
   `ModelBundle` 裡，理論上可以用歷史資料重新驗證每個決策指標的準確度
   （目前尚未實作自動化回測，見「已知限制」）。
4. **Deterministic**：同一份 candles + 同一份已訓練模型，`score_symbol()`
   永遠回傳相同結果，沒有隨機性（訓練階段的 `train_test_split` 有固定
   `random_state`）。
5. **API 與 UI 一致**：`ZoneScore`（Python）↔ `store.SRZone`（Go）↔
   `SRZone`（TypeScript）三處欄位一一對應，改動任一層都要同步其他兩層。
6. **避免重複資訊**：股票層級的量（`global_trend`/`global_volatility`）只在
   Global Model 出現一次，不逐 zone 重複；zone 層級的量
   （`zone_momentum`/`zone_direction`）才逐 zone 各自不同。

---

## 十五、Zone 生命週期驗證（Verifier）

`internal/analysis/sr_zone_verifier.go::SRZoneVerifier` 重新比對已存的
zone 跟後續實際走勢，更新 `stock_sr_zones.status`/`broken_at`/
`broken_price`。跟個股分析的 `Verifier`（`verifier.go`）同樣可重複呼叫、
每次都用目前為止最新的 candles 重新計算，不是一次性判定；差異在於 zone
是一段價格區間，且角色可能是 `AT_ZONE`（分析當下現價落在區間內，方向
未定），需要額外處理：

```
role=AT_ZONE：
    先找「收盤真正離開區間」的第一根K棒決定方向
    （收在上方 → 之後視為 SUPPORT；收在下方 → 之後視為 RESISTANCE）
    離開之前維持 PENDING（現價還在區間內，沒有方向可以驗證）

role=SUPPORT（或 AT_ZONE 離開後解析出來的）：
    收盤連續 confirmation_bars（預設 2，跟
    features.py::count_breakouts 的訓練期特徵定義一致）根低於
    price_low → BROKEN，broken_at/broken_price 取這段連續突破的第一根

role=RESISTANCE：
    收盤連續 confirmation_bars 根高於 price_high → BROKEN

其餘：
    K棒範圍曾與區間相交（觸碰過）但未被突破 → HELD_SO_FAR
    從未被觸碰 → 維持 PENDING
```

每次都是從候選 candles 的開頭重新掃描（不是從上次驗證結果繼續），所以一旦
某次驗證判定 `BROKEN`，之後不管價格如何反彈，重新驗證永遠會在同一根K棒
判定 `BROKEN`——不會被後續反彈改回 `HELD_SO_FAR`（沒有另外設計「重置」
API）。

觸發方式：
- 手動：`POST /api/v1/sr-zones/:id/verify`（見 api-reference.md）
- 自動：`daily_close` 排程（收盤後）跑完主要的拉 K 棒/掃描流程後，接著對
  最近 `srZoneVerifyLimit`（預設 50）筆 SR zone 分析各自重新驗證一次，
  寫入獨立的 `sr_zone_verify` job_run 紀錄，失敗不影響 `daily_close` 本身
  的結果。

---

## 十六、模型狀態查詢與錯誤訊息分級

**模型狀態**：`GET /sr-scoring/model-status`（Python）／
`GET /api/v1/sr-zones/model-status`（Go）讓前端在觸發分析前就能知道模型
準備好了沒——不像 `/sr-zones` 那樣在模型不存在時回 503，這支端點永遠回
200，用 `exists` 欄位表示狀態，`exists=true` 時同時回傳
`version`/`trained_at`/`split_method`/`metrics`/`feature_names`。前端「支撐/
壓力機率分析」頁面頂部會顯示這個狀態（模型可用/尚未訓練）。

**錯誤訊息分級**：`internal/analysis.UpstreamStatusError` 保留 Python
`/sr-zones` 回應的實際 HTTP 狀態碼，讓 Go handler（`mapScoreZonesError`）
依狀態碼回不同的通用訊息給前端，而不是把所有非 200 回應都壓成同一句
「Python service 錯誤」：

```
404（沒有 candles）    → 「找不到歷史資料，請確認股票代號是否正確，或先用「歷史資料回補」補資料」
503（模型未訓練）      → 「機率模型尚未訓練，請先在下方「訓練/更新機率模型」區塊訓練」
其他（連線失敗等）     → 502「Python 服務無法連線，請確認服務是否已啟動」
```

原始錯誤文字（可能含內部路徑、堆疊資訊）只寫伺服器 log，不會出現在前端
回應裡——這個設計延續了先前「伺服器錯誤只寫 log、不回原始文字給前端」的
原則（見 development-guide.md 常見問題）。

**前端新手優先閱讀層級**：「支撐/壓力機率分析」頁面預設只顯示「哪個區間
最重要、目前該觀察支撐還是壓力、什麼條件代表判斷失效、可信度高不高」這
幾件事，用白話文字取代 EV/RR/net_score 等術語；所有原始數字（機率、EV/
RR、score breakdown、觸碰統計、Global Model 原始數字）都還在，收在「展開
進階細節」裡，不刪除任何既有欄位或計算。`AT_ZONE` 角色會明確提示「方向還
不明確，不是確定的買賣訊號」；`confidence_level=LOW` 會提示「樣本少或太久
沒測試，先觀察」。

---

## 十七、模型設定可追溯性（training_config / model_config_hash）

SR Zone Scoring 橫跨四條 pipeline：zone 建立、features、score breakdown 與
evidence 屬於 [Analysis Pipeline](./architecture/analysis-pipeline.md)；bounce/break
機率模型與訓練任務屬於 [AI Pipeline](./architecture/ai-pipeline.md)；
`decision_summary` 與交易語意屬於 [Decision Pipeline](./architecture/decision-pipeline.md)。

目前系統只維持**一個現行機率模型**，不是 model registry。前端可選
`model_type` / `split_method` / `calibration_method`，但每次訓練成功都會寫入
同一個 `SR_SCORING_MODEL_PATH`，讓新的 `hold_model` / `break_model` 成為現行
模型；`sr_scoring_train_jobs` 保存的是訓練任務紀錄與 metrics，不代表有多個
模型可切換。若未來要支援多模型並存，需要另外設計 model registry、模型檔路徑、
分析快照關聯與前端選模流程。

訓練選項取捨：

| 選項 | 建議用途 | 優點 | 代價 / 風險 |
|------|----------|------|-------------|
| `gradient_boosting` | 預設正式模型 | 小到中型資料穩定，能處理非線性 | 訓練速度中等，仍需用 holdout metrics 檢查 |
| `hist_gradient_boosting` | 資料量變大時比較 | 訓練較快，適合較多 rows | 小資料不一定比預設穩 |
| `logistic_regression` | 基準模型、診斷過擬合 | 可解釋、輸出保守 | 非線性捕捉能力弱 |
| `lightgbm` | 大量資料實驗 | 大資料常有較好速度/表現 | Python 環境需安裝 `lightgbm`，否則訓練會失敗 |

| 選項 | 建議用途 | 說明 |
|------|----------|------|
| `split_method=time` | 預設正式評估 | 每檔股票用較新的 touch 事件當 holdout，避免未來資料混入訓練 |
| `split_method=random` | 舊結果比較 | 金融時間序列容易高估表現，不建議作為正式評估 |
| `calibration_method=sigmoid` | 預設校準 | 較穩；樣本不足時後端會自動降級為未校準 |
| `calibration_method=isotonic` | 大樣本實驗 | 彈性較高；小資料容易過擬合 |
| `calibration_method=none` | 診斷 estimator 原始輸出 | 不做機率校準，不建議直接當最終機率模型 |

`sr_scoring_train_jobs` 只保留任務狀態與診斷資料。前端可呼叫
`DELETE /api/v1/sr-zones/train-jobs?keep=20` 清理舊紀錄；後端只刪除
`done` / `failed`，不會刪 `pending` / `running`。

`model_version`（`v1`/`v2`/`v3`）只到 feature schema 這種粗粒度，同一個版本底下
換過幾次 `DatasetConfig`（`forward_bars`/`threshold_pct`/`label_method`
等）、zone builder 參數（`atr_width_multiplier`/`merge_pct`/
`high_volume_percentile` 等）、`model_type`、`calibration_method`，光看
`model_version` 完全無法分辨。

`ModelBundle.training_config`（`model.py`）在 `train.py::run_training()`
組裝，內容：
```json
{
  "dataset_config": { "forward_bars_support": 5, "threshold_pct_support": 0.03, "...": "..." },
  "zone_builders": {
    "ATRZoneBuilder": { "lookback": 60, "atr_width_multiplier": 1.5, "...": "..." },
    "VolumeProfileZoneBuilder": { "lookback": 60, "num_bins": 24, "...": "..." }
  },
  "model_type": "gradient_boosting",
  "split_method": "time",
  "calibration_method": "sigmoid"
}
```
`model.py::compute_config_hash()` 對這個 dict 算 sha256（`sort_keys=True`
確保建構順序不影響結果），取前 12 碼十六進位存成 `ModelBundle.config_hash`。

`POST /sr-zones` 回傳的 `model_config_hash` 就是產生這次評分的模型的
`config_hash`，Go `ToStore()` 寫入 `stock_sr_zone_analyses.model_config_hash`
——每筆分析快照都記錄了它是用哪組訓練設定產生的，重訓改參數後舊分析可以
靠這個值被辨識出來（新分析會有不同的 hash）。`GET /sr-scoring/model-status`
／`GET /api/v1/sr-zones/model-status` 也回傳目前模型的 `config_hash`／
`training_config`，供訓練前後比對用。

## 十八、Decision / Event 正規化儲存

P2-C 第一批先把 Decision Pipeline 與 Event Lifecycle 的可查詢欄位從
`decision_summary` JSON projection 到 normalized tables：

| Table | 用途 |
|---|---|
| `stock_sr_decisions` | 每筆 SR analysis 的決策主欄位與 detail projection：authority fields、RR gate/context、price path detail、defense lines、confidence explanation、risk notes、market context 與 zone summary 集合 |
| `market_event_detections` | `decision_summary.market_events` 的逐事件 detection projection |
| `market_event_states` | `decision_summary.event_state_summary.states` 的 lifecycle state projection |
| `stock_sr_daily_candidates` | `decision_summary.daily_candidate_zones` 的日 K 戰術候選區 projection |
| `stock_sr_model_metrics` | `sr_scoring_train_jobs.metrics` 的訓練模型品質 projection |
| `stock_sr_model_governance` | `probability_context.health` / `model_reports` 的單次分析模型治理 projection |
| `stock_sr_regression_results` | regression fixture、walk-forward 與 calibration 回歸驗收結果 |

這批採「不考慮舊資料」策略：不回填舊 analysis，也不保證舊快照有 normalized rows。Go
`ZoneScoreResult.ToStore()` 從 Python 回傳的 `decision` / legacy `decision_summary`
解析 projection，`SRZoneRepo.Create()` 在同一個 transaction 寫入
`stock_sr_zone_analyses`、`stock_sr_zones` 與 analysis-scoped normalized tables。

API / 前端 response 已切成 normalized snapshot primary；legacy raw JSON 欄位保留為
raw/debug snapshot，不再作正常 response source。`stock_sr_daily_candidates` 保存
`price_low`/`price_high`、`role`、`source`、`lifecycle`（deprecated，等同 `zone_health_state`）、`decision_role`、
`distance_pct`、`reason`、`event_refs` 與完整 `candidate_json`。

P2-C-5 起，`stock_sr_decisions` 也保存尚未拆成獨立表、但前端決策面需要的 detail JSON：
`market_regime_json`、`data_quality_json`、`decision_derived_view_json`、`event_sequence_json`、
`daily_price_action_json`、
`price_path_json`、`daily_confirmation_json`、`defense_lines_json`、`rr_context_json`、
`rr_gate_json`、`position_action_condition_json`、`market_context_json`、
`confidence_explanation_json`、`risk_notes_json` 與 `zone_summaries_json`。
`zone_summaries_json` 內含 `nearest_decision_zone`、`nearest_support_zone`、
`nearest_resistance_zone`、`primary_structural_zone`、`best_trade_zone`、`primary_zone`
與 `secondary_zones`。

AI Pipeline 的正規化分成兩層，不混用：

- `stock_sr_model_metrics`：訓練任務完成時寫入，保存 hold/break 的 AUC、Brier score、
  log loss、calibrated、test rows 與 raw `metrics_json` / `dataset_summary_json`。
- `stock_sr_model_governance`：每次 SR analysis 建立時寫入，保存當次模型健康度
  `health_state`、`average_edge_pp`、zone count、`confidence_gate`、flags，以及
  calibration / walk-forward / dataset diagnostics raw report JSON。
- `stock_sr_regression_results`：保存 regression fixture 或 walk-forward/calibration
  驗收 run 的結果，欄位包含 `run_id`、`model_config_hash`、`pipeline_version`、dataset range、
  split method、hold/break AUC 與 Brier score、`passed` 與 raw `metrics_json`。這張表是長期
  回歸驗收紀錄，不隨 train job pruning 被刪除。

P2-C-4 起，PostgreSQL 的 SR Zone JSON 欄位使用 JSONB，涵蓋 analysis / zone raw JSON、
decision/event projection、daily candidate、model quality 與 regression result JSON 欄位。
SQLite / MySQL 維持文字 JSON 型別；Go `RawJSON` 仍以 string 讀寫，確保三種資料庫方言共用同一套
store code。

API response 目前採 normalized snapshot 組裝。`decision` 由 `stock_sr_decisions` authority
fields / detail JSON、`market_event_detections`、`market_event_states` 與
`stock_sr_daily_candidates` 組出；`probability_context` 由 `stock_sr_model_governance`
組出。`stock_sr_zone_analyses.decision_summary` 與 `probability_context` legacy JSON 欄位仍保留
作 raw/debug snapshot，但不再作正常 response source。舊 analysis 若缺 normalized rows，
response 對應區塊回 `null`，並以 `normalized_status` 標示 `missing`。

---

## 十九、跨方法重疊分群（Confluence）

ATR 法（swing pivot + ATR 通道）跟成交量分布法各自獨立建立 zone，計算基礎
完全不同，即使各自 builder 內部已經合併過同方法的重疊 zone（見「一、Zone
建立」），仍可能出現「兩種方法各自建出、但實際上指向同一個價位帶」的
情形——這種殘餘重疊不代表資料有誤，反而是「多方法都認同」的正面訊號，
不應該被直接刪除或靜默合併掉（會丟失這個交叉驗證資訊）。

`ranking.py::_group_overlapping_zones()`（union-find）只比較**不同 method**
的 zone pair：
```
overlap 比例 = overlap 寬度 / min(zone_a 寬度, zone_b 寬度)
overlap 比例 >= 0.6（OVERLAP_GROUP_THRESHOLD）→ 同一群組
```
透過中介 zone 傳遞相連的 pair（A-B 重疊、B-C 重疊，A-C 本身不重疊）仍會被
歸為同一群組——這是 union-find 的標準行為。**不合併、不刪除任何 zone**，
只標記 `overlap_group`（群組 id，只有群組內 zone 數 > 1 才有值，單獨的
zone 為 `null`）、`confluence_count`（群組內 raw zone 數，恆 >= 1）、
`confluence_family_count` 與 `confluence_families`（去重後的獨立 evidence family）。

Evidence family 用來避免 correlated evidence inflation：多個短線微結構 zone
雖然可能都重疊在同一價位，但不應把每個 zone 都當成完全獨立證據。現行 family：

| Family | 包含 method |
|---|---|
| `STRUCTURAL_ATR` | `atr` |
| `VOLUME_PROFILE` | `volume_profile` |
| `RECENT_MICROSTRUCTURE` | `recent_pivot`、`breakdown_reclaim` |
| `VWAP_OR_AVERAGE_RECLAIM` | `vwap_reclaim` |

排序規則不變：仍是 tier 由粗到細、同層內 `trading_score` 由高到低；
`confluence_family_count` 只當第三順位的 tie-breaker（`trading_score` 幾乎不會
真的相等，實務上很少真正影響排序結果），不會改變既有排序邏輯。前端在
zone 卡片標題列仍可顯示 legacy「多方法共振 ×N」，但 primary-zone ranking
與新文案應優先使用 `confluence_family_count` / `confluence_families`；
`overlap_group` 原始 id 在「展開進階細節」裡。

---

## 命名對照（欄位與路由前綴）

系統裡有幾組名稱相似、容易誤認為不同東西的命名，實際上是「同一個量／同一個
功能」在不同層的不同叫法。整理成對照表避免閱讀時混淆（因為重新命名會橫跨
Python/Go/DB/TS/Svelte 五層並牽動已訓練模型，投報比過低，決定不重命名，改用
註解與本表消除混淆）。

### 欄位：特徵層 vs 輸出層

| 層 | 欄位名 | 定義位置 | 說明 |
|---|---|---|---|
| ML 特徵層 | `rejection_count` | `types.py` `ZoneFeatures` | 餵給模型訓練/推論的原始特徵 |
| API 輸出層 | `reject_count` | `types.py` `ZoneScore` | **值直接複製自** `rejection_count` |
| 同上（跨語言） | `reject_count` | Go `client.go`/`store` → DB 欄位 → TS `srZones.ts` → Svelte | 一路沿用輸出層名稱 |

**關鍵**：`reject_count` 與 `rejection_count` 是**同一個數值**，不是兩個獨立的量，
唯一連接點在 `scoring.py` 的 `reject_count = role_features.rejection_count`（該行有
「跨層改名點」註解）。同理 `break_count`（輸出層）＝ `breakout_count`（特徵層）。

### 路由前綴：`/sr-scoring` vs `/sr-zones`

| 前綴 | 位置 | 負責 |
|---|---|---|
| `/sr-scoring/*` | Python FastAPI（`http_server.py`）內部 | **只有** `train`、`model-status` |
| `/sr-zones`（Python） | Python FastAPI 內部 | 評分主端點（`score_symbol`），前綴本身就與 train 分裂 |
| `/sr-zones/*` | Go Gin（`server.go`）對外 API | 前端看到的全部端點（評分、驗證、train、model-status…） |
| `sr_scoring_train_jobs` | DB 表／Go `SRScoringTrainJob` | 訓練 job 實體用 `sr_scoring` 前綴 |

對外一律 `/sr-zones/*`（`/sr-scoring` 對前端完全不可見，由 Go client 轉換）。
撰寫新程式碼時要分清楚當下是哪一層，避免把 Python 內部路由跟對外 API 搞混。

---

## Evaluation Pipeline 與 Builder Config 規劃

T-002 / T-003 的後續實作方向固定為：

1. **先建立 SR Zone 專用 evaluation pipeline**：不要直接把一般
   `backtest/modular` 交易策略回測拿來當模型驗證。SR Zone evaluation 第一版應專注在
   probability、zone outcome、event lifecycle、daily confirmation 與 final entry state
   的 walk-forward 表現，避免把資金配置、position sizing 或 portfolio policy 的問題混進模型品質。
2. **evaluation 結果先寫 `stock_sr_regression_results.metrics_json`**：這張表已用來保存
   regression fixture、walk-forward 與 calibration 驗收紀錄。T-002 第一版不新增拆欄 schema；
   等 report 內的指標穩定後，再決定哪些欄位值得正規化。
3. **ATR zone 調參先抽 config，不先改預設值**：`train.py`、`scoring.py` 與 evaluation runner
   應共用同一個 `ZoneBuilderConfig` / builder factory。`atr_width_multiplier=1.5`、
   `max_merge_width_multiple=2.0` 先保留為 baseline，後續用 evaluation 比較不同 bucket config。
4. **T-003 的正式調參依賴 T-002**：低波動 / 一般波動 / 高波動 bucket 可以作為第一階段，
   但不應在缺乏 walk-forward evaluation 前直接導入 symbol-level override，避免過擬合。

### 規模上限：`sources` 與 `dataset` 必須同時常駐記憶體

`run_evaluation()` 的結構決定了它的規模上限：

```python
sources = _load_db_sources(symbols, timeframe, limit)     # 所有標的的 DataFrame 一次全載
dataset = build_training_dataset(sources, builders, ...)  # 全部 touch 併成一張表
...
volatility_profiles = _volatility_profiles(sources, dataset)   # ← 同時要 sources 和 dataset
```

**`sources` 無法在建完 `dataset` 後釋放**，因為 `_volatility_profiles` 還要用它。
峰值 = 全部原始 K 線 ＋ 全部 touch dataset ＋ 模型 ＋ 中間物，四者同時存在。

#### 實測數據

| 標的數 | touch rows | 峰值 | 單次耗時 |
|---|---|---|---|
| 10 | 6,032 | 281 MB | — |
| 20 | 11,859 | 281 MB | 131 秒 |
| 30 | 17,447 | 317 MB | 191 秒 |
| 40 | 22,401 | 310 MB | 241 秒 |
| **131** | **72,083** | **382 MB** | **約 12 分鐘** |

**峰值由固定的 import 開銷主導**（pandas / numpy / sklearn / lightgbm / shap），
資料本身只有數 MB。標的數 10→40（4 倍）、rows 3.7 倍，峰值只增加約 30MB——
**邊際成本約 1.0 MB/檔**。

**所以正確的外推方式是「固定基線 ＋ 線性邊際」**，只外推量到的那 ~30MB 資料相依部分。
把 270MB 乘以標的倍數會高估一個量級。131 檔實測 382MB 低於外推的 401MB，
**模型成立且偏保守**。

量測噪音約 ±7MB（N=30 的 317MB 高於 N=40 的 310MB，但兩者是超集關係）——
cgroup v1 的峰值含 page cache。

#### 量測方式

`MEASURE_PEAK=1 scripts/run-evaluation.sh`。峰值由**容器在退出前自報** cgroup 單調最大值，
**不能從外面輪詢**——`_volatility_profiles`（就是「sources 與 dataset 同時常駐」那一段）
跑在流程最後，輪詢會系統性低估且偏差不固定。

#### 現況上限

**時間是硬性的線性成長**（walk-forward 逐檔跑），sweep 還要再乘候選數。
這台 host 的 `MemAvailable` 常態只有 450～510MB、mem-guard 再保留 150MB：

* **150 檔可行**（推估 ~420MB，需 570MB available），且**要求執行當下不常駐額外服務**
* **200 檔不可行**（推估 470MB，需 620MB available）
* **全市場（2,298 檔）給不起**——推估 0.8～1.2GB、單次約 4 小時、6 組 sweep 約 24 小時

擴標的池的改造方向見 [`todo.md`](./todo.md) T-047。

### Decision Replay 的 as-of 邊界（無 lookahead）

replay 的每一列都只能看到 as-of 當下為止的資訊，這是整條驗證管線的第一風險，已兩次
review 逐項確認：

- zone 用 `df.iloc[: as_of_index + 1]` 重建（`_historical_zone_score_summary`），
  `current_price` 取 as-of 的 close。
- 籌碼與模型治理 context 只取 `<= as_of` 的最新一列
  （`_chip_row_for_as_of` / `_snapshot_for_as_of`）。
- 未來資料**只**用於 label：`forward_return` / `next_close_return` / `two_bar_close_return`。
- 每檔的 as-of 區間上界固定留 `forward_bars` 根（`_candidate_bar_range`），確保 label 算得出來。

### Decision Replay 的取樣規則（`replay_max_rows`）

`replay_max_rows` 是**所有股票加總**的預算，不是每檔的配額。分配規則在
`evaluation._allocate_replay_quota`：

- 預算跨股票**均分**；配額超過該檔可用 as-of 根數（`candidate_bars`）的部分會被收回，
  重新分給還有餘裕的股票，重複到收斂。短天期個股不會白白佔用預算。
- 常數 `MIN_ROWS_PER_SYMBOL = 5`：預算不足以讓每檔都拿到下限時，只覆蓋前
  `replay_max_rows // MIN_ROWS_PER_SYMBOL` 檔（依來源順序，決定性），其餘列入
  `replay_coverage.symbols_skipped`。用意是避免「200 檔各分到 1 列」這種算不出統計量的樣本。
- 每檔取的是**最新**的 N 根（`range(last_idx - quota + 1, last_idx + 1)`），不是最舊的：
  模型健康度 gate 用來限制當下進場，要用近期盤勢驗證。維持**連續**區間而非等間距抽樣，
  是因為 event lifecycle 的 `previous_event_states` 需要連續的前一根狀態。

report 的 `symbols` / `sources` / `replay_plan` 描述的是「要求驗證的範圍」，新增的
`replay_coverage`（`symbols_requested` / `symbols_covered` / `symbols_skipped` /
`coverage_ratio` / `quota_by_symbol` / `window_mode`）描述的是「實際驗證到的範圍」。預算不足
時兩者會不一致，有 skipped 時 `warnings` 也會多一筆。

覆蓋率低於 `MIN_REPLAY_SYMBOL_COVERAGE = 0.9` 時，`_decision_replay_governance_evaluation`
會加上 **warning** flag `REPLAY_SYMBOL_COVERAGE_PARTIAL` → `health_state=DEGRADED` →
production 進場上限降到 `SMALL_ENTRY`。刻意用 warning 而非 blocking：覆蓋不完整代表信心度
下降，但不該讓 watchlist 裡一檔上市不久的個股害全體停單。

> 注意：以 scheduler 預設 `replay_max_rows: 200` 搭配 50～200 檔的 watchlist，覆蓋率必然
> 低於門檻而落在 DEGRADED。這是誠實的訊號（200 列本來就驗證不了 200 檔），要完整覆蓋就得
> 調高 `replay_max_rows` 或縮小 `sr_evaluation.symbols`。

**做分層統計時，`replay_max_rows` 才是決定樣本量的旋鈕，不是標的數**（2026-08-07 實測）：
11 檔 × `replay_max_rows=5000` 得到 4,998 筆 outcome rows，九個分層裡除兩組外都有數百到
數千筆。但 `by_state` 的分布極度偏斜——`BLOCKED` 一組就佔 78%，`ENTRY_READY` 只有 13 筆。
**那是決策引擎本身的分布特性，不是取樣不足**：加標的只會等比放大各組，稀有狀態仍然稀有，
要補強只能拉高總預算。預設 200 拿來做九個分層等於每組個位數，無法產生統計量。
（2026-08-07 實測：11 檔 × `replay_max_rows=5000` → 4,998 列，九個分層裡除兩組外
都有數百到數千筆；`by_state` 的 `BLOCKED` 一組就佔 78%，`ENTRY_READY` 僅 13 筆。）

`pipeline_version` 因此從 `sr_zone_decision_replay_p0` 升為 `..._p1`，讓新舊取樣方式的
report 可區分。**`schema_version` 維持 `sr_zone_decision_replay_p0` 不變**——
`fetch_latest_sr_regression_governance` 是用 schema_version 過濾的，改了會讓 production gate
查不到資料而靜默失效。

### Replay context 的股票比對規則

decision replay 的籌碼與模型治理 context 是 `{symbol: [rows...]}`，由
`evaluation._rows_for_symbol` 依序比對：

1. **精確 symbol key**。
2. **`__default__`**：CLI 傳入單一 list（而非 per-symbol object）時
   `_load_symbol_rows_json` 會產生這個 key，語意是「這份資料套用到所有股票」，多股票也有效。
3. **單一來源時**才容忍 key 命名不一致（例如 `2330` vs `2330.TW`）。

第 3 點刻意限制在 `len(sources) == 1`：Go 端 `analysis/sr_evaluation_context.go` 只把
**查得到資料**的股票寫進 map，所以「多檔 replay、只有一檔有籌碼資料」時 dict 會剛好只剩
一組 key；若不限制，其他股票就會靜默套用別檔的籌碼分數與治理快照，污染 replay 特徵。

查無 context 的股票維持 `chip_summary.missing=True` / `model_governance_available=False`，
覆蓋率由 `outcome_summary` 的 `rows_with_non_missing_chip` / `rows_with_model_governance`
呈現，不會靜默補值。

context 在進 replay 迴圈前會由 `_sorted_context` **統一排成時間升冪**（只排一次）。
`_chip_row_for_as_of` / `_snapshot_for_as_of` 是掃到第一筆超過 as_of 就 `break`，依賴升冪
輸入；排程路徑的 Go repo 雖然都是 `ORDER BY ... ASC`，但 `POST /sr-zones/evaluate` 允許
呼叫端自帶這兩份 context，因此不能假設順序。取不到或解析不了時間的 row 會被排到最前面
（等同視為最舊），不會拋例外。

### Decision Replay 的 zone builder 參數

`run_decision_replay(builder_config=...)` 會把 `ZoneBuilderConfig` 一路傳到
`_decision_replay_rows` → `_historical_zone_score_summary`，replay 才會用指定的 ATR 參數重建
zone。未指定時走 baseline 預設，行為與過去相同。

這條路徑先前是斷的（`_historical_zone_score_summary` 早就有這個參數，但沒有呼叫端傳），
造成兩個後果：CLI 的 `--atr-width-multiplier` 等四個參數在 `--decision-replay` 模式下**靜默
無效**；而且參數 sweep 若對每組候選跑 replay，會得到一模一樣的 decision 指標。現在 replay
report 會帶出 `builder_config` snapshot，可追溯該次用了哪組參數。

**CLI 與 HTTP 兩條路徑現在都會套用**。`POST /sr-scoring/evaluate`（以及轉呼叫它的 Go
`POST /sr-zones/evaluate`）的 `atr_width_multiplier` / `max_merge_width_multiple` /
`atr_lookback` / `atr_period` 對 evaluation 與 decision replay 都生效——2026-08-05 前只有
evaluation 分支組 `builder_config`，replay 分支漏傳，是同一個陷阱的 HTTP 版。現在 config 在
分支外組好共用。這四個欄位的預設值與 `ATRZoneBuilderConfig` 相同（1.5 / 2.0 / 60 / 14），
呼叫端沒指定時等同於不傳，排程與前端的既有行為不變。

**前端入口**（2026-08-05 補）：SR Zone 頁「模型驗證 / Decision Replay」面板的
「zone builder 參數（留白 = 沿用後端預設）」摺疊區有這四個欄位。語意是**留白＝整個鍵不送**，
由後端沿用預設值——不是送 0，也不是送 null：

- **只有正數會送出**。留白、`NaN`、`0` 與負數一律不送該鍵。
- **0 為什麼也要擋**：Go 的 `SREvaluationRequest` 對這四個欄位用 `omitempty`，
  `json.Marshal` 在轉發給 Python 前就會把 0 丟掉。前端若照送 0，使用者會看到「參數收下了」
  但完全沒有效果——這種「參數收下卻沒生效」的靜默 wiring 失效現已由
  `python/tests/test_http_server_sr_evaluate_wiring.py` 鎖住。這四個參數本來也沒有 0 的
  合理語意（zone 寬度 0、ATR 期數 0）。**要支援 0 得先拿掉 Go 那邊的 `omitempty`**，
  不是在前端硬送。
- 前端 state 用 `number | null` 而非 `number`：`<input type="number">` 清空時
  `bind:value` 給的是 `null`，用 0 當預設會分不出「沒填」與「填了 0」。
- 實際生效值由 report 的 `builder_config` 回聲，要確認參數有沒有吃到就看那裡；四個參數
  **不會**存進 evaluation job 記錄。

這條路徑的規則由 `frontend/src/lib/api/srZones.test.ts` 鎖住（留白不送鍵、非正數與 NaN
要丟掉、正數要照送）。

### Calibration bins（`model_metrics.{hold,break}.calibration`）

`sr_evaluation_calibration_v1`：把預測機率等寬切 `CALIBRATION_BIN_COUNT = 10` 個 bin，
逐 bin 輸出 `lower` / `upper` / `rows` / `mean_predicted` / `observed_rate` /
`gap`（`observed_rate - mean_predicted`），並彙總 `expected_calibration_error`
（樣本加權 `|gap|` 平均）與 `max_calibration_error`。

- **空 bin 會保留**（`rows=0`、其餘 null）：sweep 要跨 candidate 對齊比較，schema 必須穩定。
- 最後一個 bin 含右端點，`proba=1.0` 不會被邊界條件漏掉。
- 總樣本低於 `MIN_CALIBRATION_ROWS = 50` 時標記 `insufficient_sample=true`——bin 內的
  `observed_rate` 抖動過大，ECE 不應拿來挑參數。

與 `probability_engine._calibration_report` 是不同東西：那支描述「訓練時有沒有做校準」，
這裡是拿 holdout 資料實際量出來的 reliability。

### 參數 sweep 的 decision 層比較

`run_builder_sweep(decision_replay=True, model_path=...)` 會讓每組候選在 `run_evaluation()`
之外再跑一次 `run_decision_replay()`（帶該候選的 builder config），candidate 摘要新增
`decision_outcomes`：`by_final_entry_state`、`rr_summary`、`replay_coverage`、
`decision_fields_available`。`best_by` 同步新增 `entry_average_forward_return`。

- **預設關閉**。未提供 `model_path` 時只記 warning 並略過 replay，zone 層比較照常完成。
- chip / governance context 在 sweep 開頭載入一次後傳給所有候選，不會每組各查一次 DB。
- 排名只計入 `ENTRY_ALLOWED` / `PROBE_ALLOWED`（`WAIT_CONFIRMATION` 的後續報酬不代表進場
  品質），且樣本數需 `>= MIN_ENTRY_OUTCOME_ROWS`，避免用個位數樣本挑參數。
- **成本**：實測 2 檔股票 × 400 根 K、4 組候選、`replay_max_rows=50` 約 73 秒，peak RSS
  177MB——記憶體不是瓶頸，時間才是。預設 5×3 grid（15 組）外推約 4～5 分鐘。因此
  `SWEEP_DEFAULT_REPLAY_MAX_ROWS = 50`（單次 replay 是 200），建議搭配較小的 symbol 集合。
- **`AT_ZONE` 比例**：`decision_outcomes.at_zone_rate` 與 `primary_zone_role_counts` 來自
  replay 每列的 `primary_zone.role`，分母只計有 primary zone 的列。比例偏高代表現價一直落在
  區間內、方向解析不出來，通常是 zone 畫得太寬——正是 ATR 寬度調校要看的訊號。
  這個指標**只有 replay 路徑量得到**：evaluation dataset 的 `role` 由 approach direction
  二選一決定（見 `features.py`），永遠不會是 `AT_ZONE`。
- **decision 層不一定能分出勝負**：若取樣區間內沒有任何列走到進場狀態（實測合成資料時
  所有列都落在 `BLOCKED`），各候選的 `by_final_entry_state` 會完全相同，
  `best_by.entry_average_forward_return` 也會是 `None`。這代表**資料本身沒有進場訊號**，
  不是參數沒生效——此時要靠 zone 層指標比較，或擴大取樣區間 / 換一段行情再跑。
  「builder config 有沒有真的生效」不要靠 decision 指標判斷，該由 replay 的
  `builder_config` snapshot 與 `zone_count` 差異來確認。

### 2026-08-06 首次實跑 sweep 的結論：卡住的是標的池，不是參數

這是第一次拿真實資料跑 sweep（11 檔 watchlist、`--limit 1500`、5,928～9,493 筆 touch、
grid 3×2）。結論與原本的預期相反，**值得記下來避免重跑一次同樣的東西**。

**一、bucket 分佈本身就是最大的限制**

| bucket | 檔數 | touch rows |
|---|---|---|
| `HIGH_VOLATILITY` | **9 檔** | 4,676 |
| `NORMAL_VOLATILITY` | 2 檔（`0050`、`2330`） | 1,252 |
| `LOW_VOLATILITY` | **0 檔** | **0** |

門檻是 `LOW ≤ 1.5%` / `HIGH ≥ 3.5%`（ATR/close），實際 ATR% 從 `2330`／`0050` 的 3.2%
到 `6243` 的 11.6%。所以 **LOW bucket 的 config 永遠不會被觸發、也永遠無法用資料驗證**，
而 `0050`／`2330` 離 HIGH 門檻只差 0.3 個百分點。

**二、zone 層候選之間的差異落在雜訊內**

6 組候選的頂層指標全距：支撐守住 1.76pp、壓力壓回 0.73pp、突破 1.08pp。
**四個指標選出四個不同的贏家**，沒有任何候選在多維度上領先——這是雜訊的典型特徵。
粗估 n≈6,000、p≈0.42 的標準誤約 0.63pp，而各候選的樣本高度重疊（同一段價格序列的不同切法），
有效樣本遠小於名目值，實際不確定度更大。

`recommended_configs_by_bucket` 雖然 `insufficient_sample=false`（兩個 bucket 都遠超過
`MIN_BUCKET_RECOMMENDATION_ROWS = 20`），但 **HIGH bucket 六組的 score 全距只有 0.0056、
前三名相差 0.0004**——這個「建議」實質上是在雜訊中排序，**不足以作為調參依據**。
`insufficient_sample=false` 只保證樣本數夠，不保證候選之間有可分辨的差異，判讀時要自己看全距。

**三、判讀 rows 的陷阱**

各候選的 `rows` 從 4,055 到 9,493 差 2.3 倍——不同 builder 參數產生的 zone 數不同，
touch 母體**根本不是同一群**，不能當成同一個實驗的重複測量。
反過來說，「窄 zone 產生 2 倍以上的觸價，但守住率幾乎一樣」本身就是資訊：
這幾組參數對 zone 品質的影響很小。

**四、唯一像訊號的東西，卻無法歸因**

NORMAL bucket 的 score 全距 0.0244（HIGH 的 4 倍），而且**偏好方向與 HIGH 相反**
（HIGH 偏好窄 zone＋少合併，NORMAL 偏好寬 zone＋多合併）。這看起來像真訊號，
但 NORMAL 只有 `0050` 與 `2330` **兩檔**，無法把「波動較低」與「這兩檔剛好是權值股／ETF」
分開，而且該組排名第一的候選只有 849 rows。

**所以要驗證 bucket-based config，缺的是更多 NORMAL / LOW 波動的標的，不是更密的參數網格。**
在標的池只有 11 檔、9 檔擠在 HIGH 的情況下，再跑幾次 sweep 都會得到同樣的雜訊。

decision 層（Pass 2）依此結論**未執行**：它的前提是 zone 層先顯示候選之間有實質差異。

### 前端手動 evaluation 入口的判讀

SR Zone 頁的「模型驗證 / Decision Replay」面板：

- **「寫入結果」預設不勾**。勾選代表把結果寫進 `stock_sr_regression_results`，而該表的最新一筆
  就是這個 `model_config_hash` 的 production entry gate 依據——**第一次寫入會讓原本 no-op 的
  gate 開始生效**（見「gate 在該模型首次 decision-replay 寫入前是 no-op」）。勾選時 UI 會顯示這段因果的警語。
- **治理判定不需要寫 DB 就看得到**：job 完成後，report 摘要下方會顯示
  `governance_evaluation` 的 `health_state`（HEALTHY 綠 / DEGRADED 黃 / UNRELIABLE 紅）、
  `confidence_gate.allow_entry`、`max_entry_state` 與 blocking/warning flags，以及
  `replay_coverage` 的覆蓋率與被略過的股票。因此標準流程是**先不勾寫入跑一次、確認判定合理，
  再決定要不要寫入**。
- `allow_entry` 與 `max_entry_state` 才是真正會限制 production 進場的值，`health_state`
  只是它們的摘要——判讀時看前兩者。
- Zone Evaluation 模式的 report 沒有 `governance_evaluation`，治理區塊不會出現。

**面板區塊與 schema 的對應**：兩種 report 的欄位幾乎互斥，面板一律以「欄位在不在」決定顯示
（`{#if report.xxx}`），不用模式旗標判斷——模式判斷會在 schema 演進時失準，by-presence 天然容錯。

| 區塊 | 依據欄位 | Zone Evaluation<br>`sr_zone_evaluation_p0` | Decision Replay<br>`sr_zone_decision_replay_p0` |
|---|---|---|---|
| 模型層（AUC / Brier / log loss / calibration bins） | `model_metrics` | ✅ | ❌ |
| Zone 層（守住率 / 壓回率 / 突破率 + 角色·方法·波動分層） | `zone_outcomes` | ✅ | ❌ |
| Decision 層（`at_zone_rate`、RR 摘要、進場狀態分層） | `outcome_summary` | ❌ | ✅ |
| 隔日／兩日確認成效 | `outcome_summary.daily_confirmation_summary` | ❌ | ✅ |
| 模型治理 / 覆蓋率 | `governance_evaluation`、`replay_coverage` | ❌ | ✅ |
| 波動側寫 | `volatility_profiles` | ✅ | ✅ |
| 警告 | `warnings` | ✅ | ✅ |

2026-08-05 之前面板只渲染治理區塊，而那是 replay 專屬欄位，所以**Zone Evaluation 模式跑完
畫面上只剩 run_id 與 rows/sources**，要看 AUC 就得勾「寫入結果」去查 regression results
表格——與「先 dry run 再決定寫不寫」的設計初衷互相矛盾。四個指標區塊補上後才真正解掉。

**判讀時的兩個陷阱**：

- **`—` 不等於 0**：模型載不到時 `model_metrics.hold` / `break` 是 `null`（不是缺鍵），
  `calibration` 在無樣本時是 `null`，空 bin 的 `mean_predicted` / `observed_rate` / `gap`
  也都是 null。UI 一律顯示 `—`；把 null 當 0 讀會把「沒資料」誤判成「完美校準」。
- **ECE 樣本不足**：`calibration.insufficient_sample=true`（樣本 < `MIN_CALIBRATION_ROWS = 50`）
  時 bin 內 `observed_rate` 抖動極大，面板會標紅「樣本不足，ECE 抖動大，不可用於調參」。
  看到這行就不要拿該次 ECE 做參數決策。

**波動側寫（`volatility_profiles`）**（2026-08-05 補）：逐檔列出 `bucket`、`atr_pct`、
`average_range_pct`、觸價次數與每百根觸價密度，並顯示分組門檻（低波動 ≤ 1.5%、高波動 ≥ 3.5%，
即 `LOW_VOLATILITY_THRESHOLD` / `HIGH_VOLATILITY_THRESHOLD`）。這一區是 Zone 層
「依波動 bucket」分層的**母體**：那裡是各 bucket 的成效，這裡是哪幾檔落在該 bucket、樣本
夠不夠。兩者要一起看，否則會拿只有一兩檔的 bucket 去下調參結論。`atr_pct` 與
`average_range_pct` 是比例（0.042 = 4.2%），`touch_density_per_100_bars` 已經是每百根的
次數、不是比率。

**Zone 層分層的欄位語意**（2026-08-06 補）：`zone_outcomes` 的三種分層
（`by_role` / `by_method` / `by_volatility_bucket`）**與頂層使用同名、同算法的比率欄位**——
`support_hold_rate`（只取組內 `is_support==1`）、`resistance_rejection_rate`（只取 `is_support==0`）、
`break_positive_rate`（整組）。這是刻意的：分層若另立一套 key，前端就無法用同一組欄位渲染
分層與頂層，也無法直接對照「這一組比整體好還是差」。

分層另外多一個 `hold_rate`，**不要跟 `support_hold_rate` 搞混**：

- `hold_rate` 是整組（支撐與壓力混在一起）的 `hold_label` 平均，即「不分方向的 zone 守住率」。
  `_bucket_candidate_score` 以 0.7 的權重用它排序 sweep 的 bucket 建議。
- `support_hold_rate` / `resistance_rejection_rate` 是同一份 `hold_label` 依角色拆開看。
  在 `by_role` 分層裡兩者**必有一個是 `null`**（該組只有一種角色），這是正常的，不是缺資料。

這組欄位在 2026-08-06 之前是錯的（分層只回 `hold_rate`/`break_rate`，前端讀三個不存在的 key，
比率欄位永遠顯示 `—`）。當時前後端測試各自對著虛構的形狀互相印證都沒發現，教訓與具體要求
見 [`development-workflow.md`](./development-workflow.md) 的「測試 fixture 必須是後端真的會產生的形狀」。

**隔日／兩日確認的分層**（2026-08-06 建立九個，2026-08-07 增為十五個）：
`daily_confirmation_summary` 除了摘要的五個 rate 與兩日正負報酬率之外，還有十五個分層。
面板依語意分三群，各自一個預設收合的 `<details>`（**粗體為 2026-08-07 新增**，
桶定義與由來見下方「再細的六個分層」）：

| 群 | 分層欄位 | 回答什麼 |
|---|---|---|
| 結果面 | `by_state`、`by_primary_role` | 這批 outcome 本身怎麼分布 |
| 條件面 | `by_volume_context`、**`by_volume_strength`**、`by_event_sequence`、**`by_primary_market_event`**、**`by_market_event_count`**、`by_market_event_types`、`by_event_market_state` | 當時的量能與事件條件下表現差多少 |
| RR 面 | `by_rr_gate`、`by_rr_gate_reason_code`、`by_rr_bucket`、**`by_stop_distance_bucket`**、**`by_entry_executability`**、**`by_rr_formula_state`** | RR gate 的判斷後來對不對、被擋住的原因是什麼 |

「量能不足時的隔日守住表現如何」這類問題只能靠分層回答，摘要的五個 rate 答不了。十幾張表一次
攤開太長，所以分三群並預設全收合；空的分層整塊不出現（by-presence，與其他區塊一致）。

判讀這一區有兩件事要知道：

- **分層裡只有原始 counts，沒有 hold rate 這種現成比率——這是刻意的。** 每組給的是
  `next_zone_result_counts` / `two_bar_result_counts`（例如 `SUPPORT_HELD: 12`）與平均報酬、
  正負報酬率。摘要那五個 rate 是 Python `_outcome_rate` 算的，帶 `primary_role` 過濾語意；
  前端若自行把 counts 相除得出「分層版 hold rate」，必然會跟 Python 的定義悄悄分岔，而且不會有
  任何測試發現。所以 UI 只照 counts 顯示，要比率請看摘要或改 Python。三個 Record 欄位
  （隔日結果 / 兩日結果 / 失敗分布）以 `隔日/` `兩日/` `失敗/` 前綴攤成 chip 列放在該組下方，
  前綴是必要的——`SUPPORT_HELD` 這種值在三個 Record 裡都可能出現。
- **`rows` 少於 20 的組會標紅「樣本不足」，但不會被隱藏。** 分層一細，單組可能只剩一兩列，
  此時正負報酬率只會是 0% 或 100%，是純雜訊。**標示而非過濾**：靜默隱藏會讓人分不出
  「這組本來就沒資料」與「這組被藏起來了」。門檻 20 是借用 Python 的
  `MIN_BUCKET_RECOMMENDATION_ROWS`（sweep 判定 bucket 樣本夠不夠下建議用的同一個數字），
  刻意不在前端自創一個沒有來歷的門檻。

面板配色沿用 [`development-workflow.md`](./development-workflow.md) 的三類規則：warnings 與
「樣本不足」屬錯誤／警示文字用 `text-rise`（紅），報酬率屬行情語意走既有
`fmtSignedPct()` / `signedClass()`。這些顏色由 `SRZones.test.ts` 的 class 斷言鎖住。

**再細的六個分層與分桶邊界的由來**（2026-08-07 補）：上表九個分層之外另有六個，
併入原本的三群顯示（條件面 +3、RR 面 +3）：

| 分層欄位 | 桶 | 邊界從哪來 |
|---|---|---|
| `by_volume_strength` | `VOL_LT_0_8` / `VOL_0_8_TO_1_2` / `VOL_1_2_TO_2_5` / `VOL_GTE_2_5` | **沿用既有常數**：`scoring.VOLUME_CONFIRMATION_LOW/HIGH`（判定 WEAK/NEUTRAL/CONFIRMED）與 `event_engine.EXTREME_VOLUME_THRESHOLD`（判定爆量事件） |
| `by_stop_distance_bucket` | `<1%` / `1–3%` / `3–6%` / `6–10%` / `≥10%` | 取自 2026-08-07 的 4,998 筆真實 report 分位數（p50≈1.8%、p75≈6.6%、p90≈9.2%） |
| `by_entry_executability` | `rr_context.entry_executability_reason_code` 原值 | 不分桶（基數本來就低） |
| `by_rr_formula_state` | `RR_FORMULA_COMPLETE` / `REWARD_MISSING` / `RISK_NOT_POSITIVE` / `ENTRY_OR_STOP_MISSING` | 依**上游成因**，不是依欄位有無（見下方警告） |
| `by_primary_market_event` | 事件的**固定優先序**代表值 | 不分桶（見下方警告） |
| `by_market_event_count` | `0` / `1` / `2` / `3+` | 事件數 |

四個設計決定：

- **量能分桶不自訂邊界。** 分層若另訂一套門檻，同一筆資料在「分類」（`volume_context`）與
  「分層」（`volume_strength`）會講出不一致的故事——例如分類說 CONFIRMED、分層卻落在偏弱的桶。

  > **但門檻相同不代表主體相同——這兩欄仍然可能不一致**（2026-08-10 更正，先前這裡宣稱
  > 沿用常數就能避免不一致，是錯的）：`volume_strength` 吃的是 **primary zone 的**
  > `relative_volume`；而 `volume_context` 在偵測到 `EXTREME_VOLUME` 事件時會被覆寫成該值，
  > 那個事件是 `event_engine.detect_market_events()` 用**全體 zone 的最大**
  > `relative_volume` 判定的。所以「primary zone 量能很弱、但別的 zone 爆量」時，同一列會
  > 同時出現 `volume_context=EXTREME_VOLUME` 與 `volume_strength=VOL_LT_0_8`——
  > **這不是 bug，是兩個不同主體**。比較兩者時要記得；replay row 只帶 primary zone，
  > 拿不到全體 zone 的最大值，要對齊得先擴充 row projection。
- **`by_rr_formula_state` 是全域維度，但在 `RR_UNAVAILABLE` 子集內是乾淨的分割。**
  真實資料上 `by_rr_gate_reason_code` 的 `RR_UNAVAILABLE` 是最大的一組（4,998 筆裡佔 62%），
  但看不出 RR 為何算不出來。加上這個分層後才分得開三種處置完全不同的情況：
  **`REWARD_MISSING`**（有停損、沒有目標價，zone 上方沒有可用壓力區——實測是最大宗）、
  **`RISK_NOT_POSITIVE`**（entry 與 stop 都有，但 `entry - stop <= 0`，停損價在進場價之上或同價）、
  **`ENTRY_OR_STOP_MISSING`**（連 entry 或 stop 都沒有，通常是根本沒有 primary zone）。

  > **不要改回「risk / reward 各自有無」的四象限**（2026-08-10 更正）：
  > `decision_engine._rr_context()` 的 `reward_price` 只在 `if risk > 0:` 內賦值，
  > 所以 **reward 有值必然蘊含 risk 有值**，「只有 reward」那一格永遠是空的。
  > 初版就開了那個空桶，真實資料跑出 0 筆卻可能被誤讀成「風險側從不缺」，
  > 而真正的 `entry - stop <= 0` 案例被靜靜併進「兩邊都缺」。
  > 這條不變式由 `test_rr_formula_state_has_no_unreachable_bucket` 對真實生產者鎖住。

  **實測分布**（2026-08-10，4 檔 × `limit=600` × `replay_max_rows=400`，400 列）：
  `REWARD_MISSING` 138（34.5%）／`ENTRY_OR_STOP_MISSING` 123（30.8%）／
  `RR_FORMULA_COMPLETE` 81（20.3%）／`RISK_NOT_POSITIVE` 58（14.5%），
  最小桶 58 筆遠高於樣本不足門檻 20。其中 `RISK_NOT_POSITIVE` 那 58 筆在初版分桶下
  會被併進「兩邊都缺」，**等於 14.5% 的列被貼上錯誤標籤**——這是重新設計的實據。

  **這個維度不是 `RR_UNAVAILABLE` 的子分類，桶內也含其他 reason code 的列**
  （2026-08-10 交叉表更正）。在 `RR_UNAVAILABLE`(235) 內它確實乾淨分割成
  127／70／38；但另有 33 列是 `RR_QUALIFIED` 卻 `ENTRY_OR_STOP_MISSING`、
  14 列是 `RR_QUALIFIED` 卻 `RISK_NOT_POSITIVE`——**gate 說 RR 合格，卻算不出可執行的
  風險距離**。原因是 `_rr_gate()` 用 zone 層預先算好的 `primary_zone.risk_reward_ratio`，
  而 `rr_context` 是另外由 entry/stop/target 推導的，兩者來源不同。這種矛盾只有交叉看
  才會浮現，是這個維度真正的價值所在。
  只看 `by_stop_distance_bucket` 會把停損距離當成唯一的原始基礎值，完全看不到 reward 側的缺口。
- **`by_primary_market_event` 不是時間順序，別照字面理解**（2026-08-07 更正，見
  本節）。這個欄位原名 `by_first_market_event`，文件也曾寫成
  「保留事件的先後」「先發生什麼」——**那是錯的**：
  - `decision_engine._event_sequence()` 是用**固定優先序**排序的
    （`EXTREME_VOLUME` 10 → `HIGH_VOLUME_BREAKDOWN` 20 → `INTRADAY_RECLAIM` 30 →
    `REVERSAL_CANDIDATE` 40），不是偵測時間。
  - 同一列的 `market_events` 全部來自**同一根 K 棒**，`normalize_market_event()` 也沒有任何
    時間欄位——**單根 K 棒內「誰先發生」根本沒有定義**。
  - 因此本欄位是**事件類型集合的確定性函數**，也就是 `by_market_event_types` 的低基數粗化，
    資訊量不會超過它。留著的唯一價值是組數少、樣本不會被切碎（實測 4 組 vs 7 組）。
  - 要問「哪個事件先發生」得靠 event state 的 `age_bars`（跨 K 棒的存活根數），
    那是另一個維度，目前沒有帶進 decision replay 的 row。
- **邊界先用真實分布試算過再定。** 上述分桶在 4,998 筆上的最小組是 87 筆，全部高於前端的
  樣本不足門檻 20——分層切太細會讓每組都標成「樣本不足」，等於白做。

**RR 分布**（2026-08-07 補）：`rr_summary` 除了既有的平均與中位數，另有
`entry_rr_distribution` / `execution_rr_distribution` / `position_rr_distribution`，
各含 `count` / `average` / `stddev` / `min` / `p10` / `p25` / `median` / `p75` / `p90` / `max`。

**為什麼一定要看分位數**：2026-08-07 的真實 report 上 `average_entry_rr = 6.45`、
`median_entry_rr = 2.34`、`max = 1032`——**平均是中位數的 2.75 倍**。只報平均會系統性
高估這套規則的風險報酬。UI 因此把 p10/中位數/p90 一起攤開，並以中位數為主要判讀依據。

`execution_rr` 同時補進統計。它一直存在於 `rr_context` 也參與 rr_gate 判斷
（`by_rr_gate_reason_code` 有 `EXECUTION_RR_INSUFFICIENT` / `EXECUTION_RR_UNAVAILABLE`），
但先前完全沒有被彙總。`position_rr` 在目前的資料上多半全空，`count=0` 的分布在 UI 整列不顯示
（畫一排破折號只是噪音）。分布的 `median` 與既有的 `median_*` 必須相等，這條由測試鎖住，
避免同一個面板出現兩個互相矛盾的數字。

參數 sweep 沒有 API 與 UI，只能用 CLI（見上節）。

**確認之後的「過程」：drawdown-like failure window**（2026-08-10 補）：
`daily_confirmation_summary.excursion` 與每個分層底下的同名區塊，回答終點報酬答不了的
問題——**確認之後價格曾經逆行多少、多久才失效**。停損是被路徑掃到的，不是被終點掃到的。

窗口沿用 `forward_bars`（預設 5）。`_candidate_bar_range` 本來就為它預留了尾端，
`idx + forward_bars` 必定在界內，所以不必另立一個要解釋、要調、要測的旋鈕。

| 欄位（在 `daily_confirmation_outcome` 下） | 語意 |
|---|---|
| `max_adverse_excursion_pct` | 窗口內最大不利偏移，**負值或 0**（0 = 從未逆行） |
| `max_favorable_excursion_pct` | 同窗口最大有利偏移，**正值或 0** |
| `bars_to_failure` | 第幾根首次失效；未失效為 `None` |
| `failure_state` | `FAILED` / `SURVIVED_WINDOW` / `BOUNDARY_UNAVAILABLE` / `DIRECTION_UNDEFINED` |
| `mae_to_stop_ratio` | `abs(MAE) / stop_distance_pct`；**> 1.0 代表窗口內曾掃到停損** |
| `excursion_window_bars` | 實際採用的根數（資料尾端不足時會小於 `forward_bars`） |

**偏移是「相對部位方向」的，不是原始漲跌幅。** `RESISTANCE` 視為偏空，所以價格上漲才算
不利，符號與原始報酬相反——這樣兩種 role 的數字才放得進同一個分布看。

**MAE 的分母是 `current_price`（確認日收盤），不是 `rr_context.entry_price`**。三個理由：

1. `entry_price` 在相當比例的列上是 `None`（就是 `by_rr_formula_state` 的
   `ENTRY_OR_STOP_MISSING`）。拿它當分母，MAE 會恰好在**最需要看停損的那些列上消失**。
2. 與 `two_bar_close_return`、`forward_return` **同分母**，「過程 vs 終點」才相減得起來。
3. `current_price` 永不為 `None`，不需要額外的 reason code。

停損那層意義改由 `mae_to_stop_ratio` 承接，把不可用性隔離在單一欄位，不污染主指標。

**`AT_ZONE` 一律不計**（`failure_state = DIRECTION_UNDEFINED`，各偏移為 `None`）。它的既有
label（`AT_ZONE_TWO_BAR_RESOLVED_UP/DOWN`）本身就沒有方向偏誤，硬指定一邊會做出沒有人
能解釋的數字。因此 `excursion.rows` 通常小於外層 `rows`，這是預期而非漏算。

「失效」的判準**沿用既有的** `_daily_confirmation_failure_bucket`（SUPPORT 跌破 `price_low`、
RESISTANCE 突破 `price_high`），不另立一套——否則同一份輸出裡會有兩個互相矛盾的「失效」。
zone 邊界為 `NaN` 時視同不存在（`BOUNDARY_UNAVAILABLE`）：拿 `NaN` 去比較會全部得到 `False`，
會把「算不出來」靜靜報成「窗口內沒失效」。

**成本量測推翻了原本的顧慮**（2026-08-10，`tests/test_excursion_cost.py`，
`PY_ENV="SR_EXCURSION_BENCH=1"` 開啟）：這件事一直沒做，理由寫的是「需要逐根回放，
成本比終點統計高一個量級」。實測**方向是反的**——既有的 per-row zone 重建是新增窗口計算的
**10.9 倍**（2991µs vs 275µs），而這還是保守下限，分母只算了 zone 重建、沒含 decision
pipeline。5,000 列總共多花約 1.4 秒。所以不需要分層計算，直接全量。

> 原本的顧慮之所以站不住腳：`_daily_confirmation_outcome` **本來就在做窗口切片**
> （`df["low"].iloc[idx+1:idx+3].min()`），把窗口從 2 根拉到 5 根只是同一個 numpy slice
> 換長度，量級上不可能與「從歷史重建 zone」相比。

**2026-08-10 首次實跑**（11 檔 × `replay_max_rows=5000` → 4,994 列 daily confirmation，
其中 3,862 列有方向；差額是 `AT_ZONE`）：

| 指標 | p10 | p25 | 中位數 | p75 | p90 |
|---|---|---|---|---|---|
| `max_adverse_excursion_pct` | −9.6% | −5.4% | **−2.8%** | −1.1% | 0.0% |
| `max_favorable_excursion_pct` | 0.0% | +1.0% | **+2.8%** | +5.6% | +9.9% |
| `mae_to_stop_ratio` | 0.00 | 0.27 | **0.91** | 4.04 | 9.17 |

三個值得記的結論：

1. **`stop_sweep_rate = 48%`**（`mae_to_stop_ratio` 中位數 0.91）。近半數的列，窗口內的
   逆行幅度**超過了自己設定的停損距離**——終點報酬完全看不到這件事。這是本節存在的理由。
2. **失效很罕見但拖得久**：`failure_state` 只有 116 列 `FAILED`、3,746 列 `SURVIVED_WINDOW`，
   而失效的平均落在第 **3.4** 根（分布 1→8、2→20、3→35、4→23、5→30）。**失效不是隔天發生的**，
   隔日確認的窗口看不完整。
3. **MAE 中位數與 MFE 中位數幾乎對稱**（−2.8% vs +2.8%），但 MAE 的左尾更長
   （p10 −9.6% vs p90 +9.9% 看似對稱，`stddev` 卻是 0.061 vs 0.071）。

分層來看（`by_state`），`stop_sweep_rate` 隨 gate 的寬鬆程度單調變化：`BLOCKED` 51.2%
（2,920 列）、`PROBE_ALLOWED` 38.2%（248 列）、`CHASING_RISK` 26.7%（217 列）。
`ENTRY_READY` 是 0%，但**只有 11 列，低於樣本不足門檻 20，不能當結論**——它值得的是
一次針對性的取樣，而不是拿來宣稱 gate 有效。

**平均數在這裡特別不能看**：`mae_to_stop_ratio` 的 `average = 7.31`、`stddev = 148`、
`max = 6261`——停損距離極小的列會把平均炸掉。判讀一律以中位數與分位數為準。

**這份資料本身被兩個資料品質問題污染**（都是這次靠 MAE 才發現的，見
當時兩者都尚未修復，現皆已處理）：4 根全零 K 棒讓 `max_favorable_excursion_pct`
出現剛好 `1.0000` 的值；0050 未還原的 1:4 分割讓 MAE 出現 −75.5%。分位數受影響有限，
但 `min = -1.0` / `max = 1.0` 這兩個端點值是假的。

### 模型與還原股價的相容性（2026-08-11 實測，結論：不需重訓）

`sr_scoring_v4.joblib` 是用**未還原**股價訓練的，而 2026-08-11 起 `db.fetch_candles`
預設回傳**還原價**（見 [`database-schema.md`](./database-schema.md) 的「股價還原」）。
訓練資料與線上輸入不同源，因此做了一次 A/B 量測。

**量測方法**：同一天、同一批列，只差在有沒有還原——把 `fetch_candles` 的 `adjusted`
預設暫時改成 `False` 跑第二輪，以 `(symbol, as_of)` 逐列對齊。
**刻意不拿先前歸檔的數字對照**：那會混入新增交易日的干擾，分不出是還原造成的還是資料變多造成的。

規模：11 檔 × `replay_max_rows=2000`，2,000 列全部可對齊。

**結論一：邊際分布沒有位移，模型沒有 out-of-domain。**

| 指標 | 還原 p10 / p50 / p90 | 未還原 p10 / p50 / p90 |
|---|---|---|
| `confidence`（模型輸出） | 0.508 / **0.646** / 0.772 | 0.507 / **0.646** / 0.775 |
| `trading_score` | 51.33 / **58.58** / 69.66 | 51.19 / **58.60** / 70.54 |
| `relative_volume` | 0.555 / 1.337 / 3.005 | 0.524 / 1.317 / 3.081 |

中位數幾乎完全相同。`confidence` 本身就是 hold/break 機率導出的**模型產物**，
所以這條直接量到了模型而不只是輸入特徵。

**結論二：個別決策確實改變，而那是還原在修正錯誤輸入。**

| 項目 | 變化 |
|---|---|
| `trading_score` 有差異的列 | 73.1%（最大差 25.0） |
| `confidence` 變動 > 0.10 的列 | 7.8%（82% 的列完全相同） |
| `daily_confirmation_state` 改變 | 5.4% |
| `final_entry_state` 改變 | 1.9% |
| `tier` 翻轉 | **0**（全部維持 `TIER_1_MAIN_STRUCTURE`） |

「**分布不動、個別列大量改變**」正是修正錯誤輸入的樣子，不是模型失效——
變動的那些列，先前是用含假跳空的價格算出來的。

**量測有效性的反向檢查**：事前訂好「若零差異就先懷疑量測沒生效」。
實測 73% 的列有差異、且分散在**全部 11 檔**（每檔 2.7%～9.3%，無一為零），
與「11 檔都有除權息事件」一致。

**沒有涵蓋到的**：`net_score` 與 `expected_value` 不在 decision replay 的 `primary_zone`
投影裡，這次沒比到。`confidence` 與 `trading_score` 都涵蓋了，兩者都在模型鏈路上。

**值得留意的尾巴**：`risk_reward_ratio` 的 **p10 由 0.307 降到 0.204**（低尾往下三分之一），
p25 以上完全不變。RR gate 吃這個值，所以低 RR 那一端會有更多列被擋——方向偏保守，
但日後若看到 `RR_INSUFFICIENT` 變多，成因在這裡。

---

### Production 端的 regression governance gate

`pipeline._merge_regression_governance_gate` 把「同 `model_config_hash` 的最新 decision replay
結論」合併進 production 的 `probability_context.health`。合併規則**只趨保守，絕不放寬**：

- `health_state` 取兩者較嚴重者（`UNRELIABLE` > `DEGRADED` > `HEALTHY`）。
- `allow_entry` 只會被設成 `False`，不會從 `False` 變回 `True`。
- `max_entry_state` 取 `_entry_rank` 較小（較保守）者；`allow_entry=False` 時直接壓成
  `WAIT_CONFIRMATION`，`DEGRADED` 時上限壓到 `SMALL_ENTRY`。
- 查無 regression 結果時原樣回傳 base——**這一層安全網在該模型首次寫入前是 no-op**，見下。

#### gate 在該模型首次 decision-replay 寫入前是 no-op（by-design）

`_merge_regression_governance_gate` 只有在
`fetch_latest_sr_regression_governance(model_config_hash)` 查得**同模型**、
`schema_version=sr_zone_decision_replay_p0` 的最新 replay 結果時才會作用。

若該 `model_config_hash` 尚未跑過任何 `--write-db` 的 decision replay
（新訓練的模型、或 scheduler 關閉且從未手動執行），fetch 回 `None` → gate 為 no-op，
分析維持原本的模型治理。

**這是刻意的安全預設**（gate 只趨保守、不因缺資料而誤擋），
但意味著**這層安全網要等該模型至少跑過一次 evaluation 並寫入 DB 後才生效**。

* 上線流程若倚賴此 gate，**新模型部署後要排入一次 decision replay**。屬營運相依，不是 bug。
* `schema_version` 因此**不可隨意變動**：`fetch_latest_sr_regression_governance` 用它過濾，
  改了會讓 gate 查不到資料而靜默失效（這也是 `pipeline_version` 升到 `p1` 時
  `schema_version` 仍維持 `p0` 的原因）。

**未知的 `health_state`**（欄位改名、上游格式變動、拼字錯誤）不會升嚴重度——維持
「不因資料壞掉而誤擋」的原則——但會加上 `REGRESSION_GOVERNANCE_STATE_UNKNOWN` 到
`warning_flags` 與 `reason_codes`，讓問題在 report 與 decision reason 看得見，而不是靜默
失效。相關的 gate 安全性質由 `tests/test_pipeline.py` 鎖住。

### Evaluation 排程現況（`sr_evaluation` job）

`backend/internal/scheduler/scheduler.go` 的 `Start()` 對 sr_evaluation 有兩個刻意的行為，
兩者都由 `backend/internal/scheduler/scheduler_test.go` 鎖住：

- **預設關閉**：只有 `sr_evaluation.enabled: true` 時才 `cron.AddFunc` 註冊排程。預設 `false`，
  避免開發環境一啟動就對 Python service 打大量 decision replay。關閉時仍可用
  `POST /scheduler/sr-evaluation/run` 或 SR Zone 頁面手動觸發。
- **cron 字串非法只記 log**：`AddFunc` 回錯時只 `log.Error`，不 panic 也不中止 `Start()`，
  其餘排程照常註冊。代價是設定打錯時 sr_evaluation 會靜默不執行，只能從啟動 log 發現。

手動入口 `Scheduler.RunSREvaluation()` 與 cron 走同一條 `runSREvaluation`，因此兩種觸發方式
都會建立 `sr_evaluation_jobs` 紀錄並寫入 `job_runs`；狀態推導與 job 生命週期沒有分岔。

---

## 已知限制

- **`atr_width_multiplier`/`max_merge_width_multiple` 需要依實際股票調參**：
  這兩個常數目前是全域預設值（1.5／2.0），對不同價位、不同波動度的股票
  可能需要不同的合理範圍，尚未针對大規模真實資料做系統性調參。
- **`reject_count`/`break_count` 與 `rejection_count`/`breakout_count` 是同一個量
  的跨層不同命名**（不是兩個獨立數值），詳見上方「命名對照」表。
- **Python `/sr-scoring/*` 與對外 Go `/sr-zones/*` 前綴分裂**，同一功能依語言
  邊界命名不同，詳見上方「命名對照」表。

## 十九、五層分析管線與模型證據

SR Zone v4 將同步分析固定為以下單向資料流：

```text
Data → Features → Score → Evidence → Decision
```

- `pipeline.py` 只負責 orchestration；各 stage 透過 `pipeline_types.py` 的 immutable
  dataclass 傳遞資料。
- Data 固定 K 線、zone、分析時間、模型及不晚於分析日的籌碼快照。
- Features 分開保存 support/resistance 特徵與模型向量，Score 不再自行抓資料。
- Evidence 對 hold/break 的「校準且正規化後最終機率」執行 Permutation SHAP。
  每個 zone 兩個角色都保存 baseline、final probability、原始特徵值、正負貢獻
  與 `additivity_error`。浮點重建誤差容許至 `1e-5`；超過才視為證據失真並中止。
- Decision 的公開入口只接受 `AnalysisEvidence`，不可回頭讀 DataFrame 或自行推論模型。

**Evidence 可降級（不再是硬性 503 相依）**：SHAP 貢獻是可降級的展示層。
`build_evidence` 在下列任一情況降級——`sr_scoring.evidence_enabled=false`、
`shap` 套件未安裝、或模型缺 v4 `explanation_background`——此時各 zone 的
`support`/`resistance` 設為 `null`、仍保留純規則的 `risk_flags`，
`global_evidence.model.explainer` 設為 `null`、`evidence_available=false`，
`/sr-zones` 仍以規則式＋機率分數正常回應（**不再整包 503**）。`load_model`
的唯一硬性 gate 是 feature schema 相容（決定能否評分）；v4/background 缺失
不再阻擋評分。只有找不到模型檔、schema 不相容或無 K 棒才會錯誤。

**Evidence 延遲控制**：`sr_scoring.evidence_max_zones`（預設 `8`，`0`=全部）
只對 `trading_score` 前 N 的 zone 產生 SHAP evidence，其餘 zone 降級為 `null`
但保留 `risk_flags`；同一模型的 SHAP background 與 explainer 每次分析建一次
重用，降低熱路徑成本。

Go backend 呼叫 Python `/sr-zones` 使用專用同步逾時
`python.sr_zones_timeout_sec`（預設 `120` 秒，環境變數
`PYTHON_SR_ZONES_TIMEOUT_SEC`），不沿用 `/analyze`/`model-status` 的 30 秒 client。
逾時會回 `504 Gateway Timeout`；若延長逾時後仍過慢，優先降低
`sr_scoring.evidence_max_zones` 或關閉 `sr_scoring.evidence_enabled`，讓 evidence
降級而不是停用整個 SR Zone 評分。

Python `/sr-zones` 回傳 breaking v2 nested contract：
`analysis`、`features`、`score`、`evidence`、`decision`、`explanation`、`scenario`，
以及每個 zone 各自的 `data/features/score/evidence/explanation/scenario/lifecycle`。
Go 將 analysis/zone 的 evidence、explanation、scenario JSON 連同 `pipeline_version`
保存，歷史快照不會回算。`explanation.model_context.uses_shap_evidence` 依
`explainer` 是否存在判斷，evidence 降級時為 `false`（前端 badge 顯示「rules only」）。

### 十九之一、Explain Engine 白話解釋層

Explain Engine 是 Evidence/Decision 之後的白話層，只用既有 `score`、
`features`、`evidence`、`decision` 的結構化欄位套 deterministic 模板，不接
LLM、不改 scoring 數學、不改 action 門檻。它的目的不是重新判斷買賣，而是把
「為什麼這個區間被視為支撐/壓力、為什麼得到 BuySmall/Hold/Avoid、哪些因素
加分/扣分、哪些風險需要注意」整理成前端可直接顯示的穩定文字。

**資料來源與分工**：

- `decision`：提供 action、primary zone、market regime 與風險提示，是
  `explanation.action_reason` 的主要來源。
- `score`：提供 role、probability、confidence、trading score breakdown、
  recent validation、touch/reject/break counts，是 zone 解釋的主要來源。
- `evidence`：保留 SHAP、risk flags 與模型 metadata；Explain Engine 可引用它，
  但不取代它。前端仍把 SHAP 放在進階細節。
- `features`：保留支撐/壓力方向的原始特徵快照；目前主要間接透過 score 使用。

**頂層 `explanation` contract**：

| 欄位 | 說明 |
|---|---|
| `schema_version` | Explanation schema 版本，目前為 `sr_explain_v1` |
| `summary` | 一句整體結論，對齊當次 `decision.action` |
| `action_reason` | 為什麼得到目前 action，通常引用 primary zone 或「沒有明確主交易區」 |
| `market_drivers` | 趨勢、波動、整體信心、籌碼等主要因素 |
| `risk_notes` | 使用者應注意的風險；可整合 decision risk notes 與全局風險 |
| `model_context` | 模型版本、設定 hash、是否使用 SHAP evidence |

**`zones[].explanation` contract**：

| 欄位 | 說明 |
|---|---|
| `schema_version` | Explanation schema 版本，目前為 `sr_explain_v1` |
| `role_summary` | 此 zone 是支撐、壓力或方向未定的白話說明 |
| `score_reason` | trading score 主要由最高與最低分量解釋，不列完整 breakdown |
| `probability_reason` | 反彈/跌破機率與期望值的白話說明 |
| `confidence_reason` | 樣本數、最近觸碰、守住/跌破穩定度如何影響 confidence |
| `positive_factors` | 加分因素列表 |
| `negative_factors` | 扣分或風險因素列表 |
| `watch_conditions` | 要觀察的價位、量能或突破/跌破條件 |

`AT_ZONE` 必須明確說明「現價在區間內，方向尚未解析」，不得硬判支撐或壓力；
其 direction-only 欄位（反彈/跌破機率、EV/RR、量能確認等）仍依既有 contract
維持 `null` 或不給方向性結論。

**相容與 fallback**：舊分析可能沒有 `explanation` 或值為 JSON `null`。前端不可
顯示空白錯誤，應回退顯示 `decision_summary`、`analysis_tips` 與既有 evidence。
新分析若 `explanation` 缺少必要欄位，應視為 Python contract 或 Go passthrough
問題，而不是由前端臨時補齊完整白話解釋。

**非目標**：Explain Engine v1 不納入持股、部位成本、下單 sizing、LLM 生成、
跨 `/analysis` 舊流程解釋，且不對歷史快照回算 explanation。

### 十九之二、PriceActionEvidence 後續方向

目前 Price Action 訊號分散在 `zone_interaction`、`daily_price_action`、`market_events` 與
`position_action_condition`。後續若要重構，應收斂成單一 `PriceActionEvidence` 資料流：

```text
OHLCV
  -> ZoneInteractionDetector
  -> PriceActionEvidence
  -> Decision Engine
  -> Summary
```

`PriceActionEvidence` 承載 `reclaim_type`、`rejection_type`、`penetration_ratio`、
`close_relative_to_zone` 與 `follow_through` 等欄位，並嵌在 `zone_interaction.price_action_evidence`
中。舊的 `touched`、`closed_above`、`closed_below`、`penetration_pct` 欄位仍保留相容；新的 reclaim /
breakdown / structure 判斷應優先讀 evidence，避免底層 evidence 與 summary 狀態彼此矛盾。

### 十九之三、Scenario Engine 情境層

Scenario Engine 是 `decision` / `score` / `explanation` 之上的結構化情境層，
負責輸出「目前情境、觸發條件、失效條件」。它不改機率、分數、EV/RR 或
action 門檻，也不取代 `explanation.watch_conditions`；`watch_conditions` 是 zone
白話觀察句，`scenario` 是前端可穩定呈現的正式 contract。

**頂層 `scenario` contract**：

| 欄位 | 說明 |
|---|---|
| `schema_version` | Scenario schema 版本，目前為 `sr_scenario_v1` |
| `state` | 對齊目前 decision action，例如 `BuySmall` / `Hold` |
| `title` | 情境標題，例如「小量試單情境」或「等待確認情境」 |
| `summary` | 一句整體情境摘要，引用 market regime 與 primary zone |
| `trigger_conditions` | 讓此情境成立或值得追蹤的條件 |
| `invalidation_conditions` | 讓此情境失效或需要重評估的條件 |
| `market_regime` | `decision.market_regime` 的原樣引用 |
| `primary_zone` | `decision.primary_zone` 的原樣引用，可能為 `null` |
| `global_confidence` | 這次分析的 global confidence，可能為 `null` |

**`zones[].scenario` contract**：

| 欄位 | 說明 |
|---|---|
| `schema_version` | Scenario schema 版本，目前為 `sr_scenario_v1` |
| `state` | `SUPPORT_RETEST` / `RESISTANCE_REJECTION` / `WAIT_FOR_DIRECTION` / `RETEST_REQUIRED` / `BROKEN` |
| `title` | Zone 情境標題，例如「支撐回測情境」 |
| `summary` | 結合 role、bounce/break probability、EV 的短摘要 |
| `trigger_conditions` | 此 zone 情境成立時要看到的價格或確認條件 |
| `invalidation_conditions` | 此 zone 情境失效時要看到的價格或風險條件 |

`AT_ZONE` 的 scenario 必須維持 `WAIT_FOR_DIRECTION`，只描述向上/向下離開區間後
如何再觀察，不得產生方向性的支撐/壓力結論。舊分析可能沒有 `scenario` 或值為
JSON `null`，前端應隱藏 scenario 區塊並繼續顯示既有 explanation/decision。

`analysis.period_summaries`、`analysis.analysis_tips` 與
`analysis.chip_summary` 是持久化快照契約，不因改用五層管線而移除。
`evidence.chip` 與專屬 `chip_summary` 來自同一份 Score stage 計算結果：
前者供 Decision/Evidence 使用，後者維持查詢與舊快照相容。舊資料沒有 evidence
時，前端回退讀取 `analysis.chip_summary`。

### 二十、Summary / Tips 模組邊界

`analysis.period_summaries` 由 `summaries.py` 組裝，負責短 / 中 / 長摘要、
摘要專用混合排序、zone summary serialization 與摘要 reasons。摘要選價採
`trading_score` 50%、`confidence` 20%、距離現價 20%、`confluence` 10%；
完整 `zones` 排序仍由 scoring/ranking 主流程控制。

`analysis.analysis_tips` 由 `tips.py` 組裝，定位為「分析報告閱讀指南 / 小辭典」，
不是產品操作說明。API 仍維持 `string[]` 以相容前端跑馬燈。內容分兩部分：

固定前綴（`_build_analysis_tips` 內硬編字串）：

- 指標小辭典：`RR`、`EV`、`Confidence`、`Trading Score`、`Confluence`。
- 價位語意：`Support`、`Resistance`、`AT_ZONE`、`Primary Zone`。
- 事件語意：`Break`、`Bounce`、`Reclaim`、`Invalidated`、`Pullback`。

enum 狀態目錄（`ANALYSIS_STATUS_TIPS`，由 `analysis_status_tips()` 展開）分成：事件語意、
事件生命週期、區間分級、證據分級、市場狀態、趨勢分級、多空傾向、市場行為、進場權限、日 K Gate、
價格路徑、RR 與模型。每筆為「分類｜CODE：名稱。說明」。此目錄是手工維護、與引擎
（`decision_engine.py`、`event_engine.py`、`types.py`）分離，維護時名稱需對齊 `decision_engine.py`
既有中文 label map（如 `HIGH_VOLUME_BREAKDOWN`＝「放量破位」、`ENTRY_READY`＝「日 K 進場條件成立」）。

尾端動態提醒（`_moving_average_tip`／`_chip_tip`）：均線與籌碼的判讀提醒，並統一提示支撐不是買點、
壓力不是放空點、低信心不是看空、`RR` 高不等於勝率高。

**已知限制**：`ANALYSIS_STATUS_TIPS` 內 `BULLISH_RECOVERY`（市場狀態）、`ENTRY_ALLOWED`（進場權限）
在引擎中無對應實作，`BUY_READY`（進場權限）僅存在於 `decision_engine.py` 的 rank 對照表、實際永不輸出；
三者為刻意保留的前瞻／佔位條目，實務上不會出現在真實分析輸出中。

`scoring_rules.py` 負責 trading score 權重、breakdown、recommendation、entry
relevance breakdown 與 no-direct-chip shadow policy。`utils.py` 放跨模組共用的
數值 helper，例如 `distance_pct_to_zone_bounds`；價格格式化則沿用既有
`formatting.py::fmt_price`。

`ranking.py` 負責 tier assignment、zone sorting、overlap grouping 與 evidence family
mapping。`serialization.py` 負責 zone score response dict，維持 API 欄位契約。

`scoring.py` 現在保留核心 zone scoring、probability/confidence/recent validation、
chip summary、global metrics 與 `score_symbol()` 入口；相容性上仍 re-export 已拆出的
helper，避免舊測試或內部引用一次性中斷。

---

## 二十、Zone 身分與 ZoneMatcher

跨交易日追蹤「同一個 zone」的機制。身分層（階段 A／B）與事件層（階段 C，
`event_instances` / `event_transitions` / `zone_key_aliases`）都已實作，
**兩層目前都只寫不讀**：決策路徑仍然走 `market_event_states`，
既有欄位逐欄相同是驗收條件。

### 為什麼需要身分

事件鏈原本的身分是 `event_engine.py` 的 `_zone_key()`：

```python
return f"{role}:{price_low:.4f}:{price_high:.4f}"
```

**身分綁在浮點邊界上，而 zone 邊界每次分析都由 ATR 重算。** 兩個後果都在 live 資料裡
證實過：同一個支撐會因為邊界微幅漂移而分裂成兩條並存的鏈（下游重複計數，且沒有任何
檢查會發現）；`role` 在 key 裡，所以支撐翻壓力必然產生全新身分、舊鏈直接斷掉。

### 三層模型：身分 / 一世 / 轉換

| 層 | 表 | 語意 |
|---|---|---|
| 身分 | `zone_instances` | 長壽。跨越失效與角色翻轉都不改變 |
| 一世（role incarnation） | `zone_role_incarnations` | 角色翻轉或失效後重生會開新的一世（`seq + 1`） |
| 轉換 | `zone_transitions` | append-only 流水 |

`INVALIDATED` 是**該一世的終態，不是身分的終態**。同一個價位失效後又重新有效，是同一個
身分的下一世——這樣「這個價位長期是不是關鍵」與「這一世活了多久」兩個問題才都答得出來。

`ROLE_FLIPPED` **不是狀態而是 transition**：它結束當前這一世、開啟下一世，持久化的結果是
`zone_role_incarnations` 多一列，而不是某個欄位被改成 `ROLE_FLIPPED`。

欄位定義、狀態機的合法轉換、`from_state` 的不變式與 `EXPIRED` / `INVALIDATED` 的差別，
見 [`database-schema.md`](./database-schema.md)。

### ZoneMatcher（`zone_matcher.py`）

**`is_same_zone()` 是純幾何**：比中心位移比率、寬度變化比率與 `method`，
**不比 role**（角色翻轉要能保持同一身分），也不看時間與資格——把時間條件混進來會讓
「形狀像不像」與「還算不算數」變成同一個判斷，而兩者的失敗方式完全不同。

**血緣型別是連通元件的性質，不是單一邊的性質。** 同一條邊在 1→1 裡是延續、在 2→2 裡是
`RESHAPE`。只有 1→1 的延續保留 `zone_uid`；`SPLIT` / `MERGE` / `RESHAPE` 的所有 child
都取得新 uid，parent 寫成身分終態。**延續不寫血緣邊**——那會是 `parent == child` 的自環，
讓沿 parent 遞迴回溯祖先的查詢無法終止。**只寫實際匹配上的邊，不做笛卡爾積**：
鏈狀元件（P0–C0、P1–C0、P1–C1）很常見，補成 4 條邊會憑空生出 `is_same_zone` 明確否決過的
P0–C1。

**翻轉判定比對的是「當前這一世的 role」而非「上次觀測到的 role」**，所以
`RESISTANCE → AT_ZONE → SUPPORT` 這種穿過 `AT_ZONE` 的翻轉抓得到。`AT_ZONE` 是
「方向暫時無法解析」不是角色，它不結束一世，只在 `zone_transitions` 留
`ROLE_UNRESOLVED` / `ROLE_RESOLVED`。

**門檻的來源**（live 311 列 zone、相鄰分析間 IoU > 0 的 543 筆配對掃描而來，不是猜的）：
純 IoU 會被 zone 寬度污染——寬 zone 漂一點點 IoU 就掉很多，所以改用兩個**尺度無關**的量。
定案是**中心位移 < 0.06、寬度變化 < 0.25**。`IoU ≥ 0.90` 是這個判準的嚴格子集，
而位移判準另外多收 16 組形狀上就是日常漂移的配對（例如
`[109.59,113.65] → [109.80,113.45]`，IoU 0.899 但中心位移只有 0.001）。

> 引用數字前先看門檻：`+16` 是 0.06 的、`+38` 是 0.10 的，這兩個被混用過一次。

**判準的鬆緊直接換成一對多的數量**（0.06 下是 7 組一對多、6 組多對一；放寬到 0.10 會變成
20／19）。一對多不是雜訊而是真實的分裂／合併，放寬只是讓它現形——所以問題不是
「怎麼把一對多壓到 0」，而是「matcher 要不要從第一天就支援它」，答案是要，用血緣表達。

**`role` 必須排除在身分之外，證據比預期強**：IoU ≥ 0.8 的角色翻轉有 18 筆，其中
`volume_profile` 有多筆 **IoU = 1.000** 的翻轉——價格區間一模一樣、邊界完全沒動，
只因為 role 重新解析，舊 `zone_key` 就整個不同、鏈直接斷。而
`AT_ZONE ↔ SUPPORT/RESISTANCE` 在價格穿越 zone 時就會切換，是常態不是例外。

**資格閘門有兩個軸**，只決定「這個身分還有沒有資格進入 matcher」：
`MAX_ABSENCE_TRADING_DAYS = 20`（交易日陳舊度，在 matcher 擋）與
`MAX_OBSERVED_ABSENCES = 3`（看了幾次都沒看到，在 `ListLive` 的 SQL 擋）。
**兩個都要**——單一時間軸分不出「zone 消失了」與「我們根本沒看」。距離用交易日而非
日曆天，交易日曆由呼叫端注入，沿用 candles 的 distinct 日期，不引入外部假日表。

### 接線

`/zone-identity/match`（Python 獨立端點）＋ `MatchZoneIdentities`（Go client）＋
`persistZoneIdentity`（handler）。

**刻意不併進 `/sr-zones` 的核心路徑**：身分層目前**只寫不讀**，沒有任何決策依賴它的輸出；
併進去就得動 `scoring.py` / `pipeline.py` 那條有決策責任的路徑，為一個還沒有讀者的功能
去動它，風險與收益不成比例。代價是每次分析多一趟 HTTP。**寫入失敗只記 log，不影響分析
本身**——所以排查時要主動 grep log，API 回 201 不代表身分寫進去了。

#### 四個已知限制

1. **只接在 `reuse_existing=false` 那條路徑。** `SRAnalysisProvider.Analyze` 也會
   `repo.Create`（`portfolio/analyzer` 與 `/sr-zones` 的 reuse 分支），那些跑法會產生 zone
   但**不算一次觀測**，所以 `observed_absences` 統計的是分析的一個子集。刻意如此：重用既有
   分析的目的就是不重算，把它算成觀測會讓「我們看了幾次」失真。
2. **`as_of` 取的是 wall clock，不是資料日期**，所以 `observed_absences` 量的是分析次數
   而非時間。詳見 `database-schema.md`。
3. **同一 symbol 的併發分析會產生身分 churn 或靜默的交易失敗。** 兩個同時的
   `POST /sr-zones`（前端連點兩下就夠）都會讀 `ListLive` 再寫；zone 若翻轉，兩邊都算出
   `seq = MaxSeq + 1`，第二個 `Apply` 撞唯一索引而整筆 rollback，而錯誤只記 log。
   要修的話是對 `(symbol, timeframe)` 加行程內鎖或 DB advisory lock。
4. **`/zone-identity/match` 對格式錯誤的 `as_of` / `trading_days` 回 500 而不是 422。**
   呼叫端只有 Go backend、值都由程式產生，實際上碰不到；之後若開放人工呼叫要補。

### 事件層：鏈的身分與三段關聯決策

`event_instances` 一列是**一條事件鏈**（一個 zone × 一個 family × 一個 `seq`），
`event_transitions` 記它的狀態轉換。schema 與欄位語意見
[`database-schema.md`](./database-schema.md)「event_instances / event_transitions /
zone_key_aliases」。這裡只講判讀時最容易搞錯的那件事：**事件帶的 `zone_key` 對不上
本次分析的 zone，是常態而不是例外**（2026-08-19 實測 41 筆 ZONE scope 事件有 26 筆對不上），
成因有二——role 編在 key 裡（`SUPPORT` 變 `AT_ZONE` 就換了 key），以及邊界由 ATR 重算會漂走。
身分都還 ACTIVE，只是 key 到不了它。

所以關聯是三段的，**優先序不可調換**：

1. **既有鏈優先**：`(last_zone_key, event_family)` 命中活鏈 → 直接沿用
   `instance.zone_uid`，連 key 都不必解析。上面那兩個成因在這一段就被吃掉了。
2. **carried 護欄**：沒有活鏈 ＋ `carried_from_previous = true` → **不建立新 occurrence**。
   carried 代表「重報」不代表「發生」；少了這條，每一條走完生命週期的鏈都會在此後
   每次分析重生一條新鏈。
3. **key 解析**：本次分析的 `zone_key → zone_uid` map，miss 再退到 `zone_key_aliases`
   （每個身分保留最近 8 個用過的 key）。這一段只服務**新**關聯。

`carried_from_previous` 由 Python 兩條路徑都寫進 `state_json`——新偵測在
`build_event_state_summary` 寫 `false`，carry forward 在 `_normalize_previous_event_state`
無條件寫 `true`。Go 只讀不推導，**缺鍵是異常**而不是「不是 carried」。

**第三段用的 `zone_key` 由單一 authority 產生。** 本次分析那份 `zone_key → zone_uid`
map 的鍵，是 Python 在 zone 序列化時呼叫**同一個** `_zone_key()` 輸出的欄位；Go 只做
字串比對，**不自己用 `fmt.Sprintf("%s:%.4f:%.4f", …)` 重建**。兩份浮點格式化只要哪天
分歧，關聯就會**靜默**失敗——事件掛不到 zone，外觀與「這次沒有 zone 事件」一模一樣。

事件層也沿用身分層四個已知限制中的兩條：`event_instances` 同樣**只在
`reuse_existing=false` 那條路徑寫入**（統計的是分析的子集），同一 symbol 的併發分析
同樣會撞唯一索引而整筆 rollback、且只記 log。

**zone 身分因 `SPLIT` / `MERGE` / `RESHAPE` 終止時，parent 身上的事件不傳給 child**，
而是以 `end_reason = 'ZONE_IDENTITY_ENDED'` 收攤：那條鏈的前提已經消失，接到 child
等於宣稱鏈延續了，而 `RESHAPE` 的定義正是「血緣無法解析」。血緣留在 `zone_relations`，
之後沿 parent 回溯補得回來；反過來先接了就拆不回來。

#### 可觀測性：一筆結構化 log 拆出關聯決策

每次分析輸出一筆 `event identity: zone association*`，欄位包含三段各自的命中數
（`matched_by_chain` / `matched_by_current` / `matched_by_alias`）與各類異常。
級別是刻意分的：`unmatched_zone_keys` / `chain_conflicts` / `chain_key_ambiguous` /
`alias_ambiguous` / `carried_parse_failed` 任一非零升 **Warn**；終態不變式被違反升
**Error**；其餘走 Debug。

**`carried_noop` 不升級別**：終態被 carry forward 是常態，每一條走完的鏈此後每次分析
都會貢獻一筆，讓它升 Warn 等於保證 warn 永遠不會歸零、真正的異常被淹掉。
它仍然出現在欄位裡，判讀方式是逐筆去對 `event_instances`——對應的鏈**應該都已終結**。

這是「鏈靜默凍結」唯一抓得到的東西：那個缺陷的症狀是資料表面上完全正常
（鏈停在 `CONFIRMED` 不再更新），只有把「事件是怎麼找到身分的」逐段數出來才看得見。
升級成可查詢的 metric 是 todo T-050。

### 實測特性

**對 live 311 列真實 zone 重放**（階段 A，2026-08-18）：收斂成 **167 個身分**，
其中身分延續 144、血緣邊 17（SPLIT 4 ／ MERGE 2 ／ RESHAPE 11），
`ROLE_RESOLVED` / `ROLE_UNRESOLVED` / `ROLE_FLIPPED` 為 6 / 8 / **3**——
真翻轉只有 3 筆，而混合口徑（把 `AT_ZONE` 的進出也算進去）會報成 17 筆。
**`AT_ZONE` 的進出不是翻轉**，混在一起會讓真正的翻轉被雜訊淹沒。

**as-of 階梯實跑**（階段 B 端到端，2026-08-19，0050 七階）：

*身分機制確實優於舊 key，但不到「穩定」。* 104 次 zone 觀測下——

| 身分機制 | 身分數 | 觀測數／身分 |
|---|---|---|
| 舊 `_zone_key()` | 86 | 1.21 |
| zone identity | 59 | 1.76 |

*穩定度隨 method 差很多*，這是判讀時最需要記得的一件事：

| method | 活過 >1 次分析 |
|---|---|
| `atr` | 83% |
| `recent_pivot` | 79% |
| `volume_profile` | 27% |
| `vwap_reclaim` | **0%** |

結構性的方法身分延續得好，`volume_profile` 與 `vwap_reclaim` 幾乎每次都是新的。
**這兩個 method 的 zone 不適合拿來談「這個 zone 活了多久」**——不是 matcher 的缺陷，
是這些 method 每次算出來的東西本來就在動。

*2→2 的 `RESHAPE` 代價要知道*：兩個互相重疊的 ATR zone 只要一起漂 0.02 就會四條邊全部
成立、元件變成 2→2，兩個身分一起終止並重鑄。這是「血緣型別是元件的性質」的直接後果，
不是 bug；實測沒有形成 churn 迴圈（後續各階沒有再對同一組 zone 重複 RESHAPE）。

**給下游（T-049 / Bias / Final Entry）的判讀提示**：該關心的是 `ROLE_FLIPPED`，
而 `AT_ZONE` 的來回 churn 多半只是價格在 zone 內外進出。兩者若不分開，下游就得自己
重新推導——那正是這套機制要消滅的模式。

**事件層的 as-of 階梯**（階段 C，2026-08-19，0050 同一組七階）：七階全部 201，
`unmatched_zone_keys` / `invariant_violations` **每一階都是空的**，
`carried_parse_failed` 為 0；事件鏈收斂到 **10 條**（8 條掛 zone、2 條 SYMBOL scope），
而三段關聯決策改進之前同樣的資料會產生 14 條且持續增長——差額正是被 carried 護欄
擋下來的終態重生。`carried_noop` 收斂到 5 筆，逐筆對過都是已終結的鏈。

**血緣終止的實測覆蓋**（2026-08-19，`2330` / `3105` / `6182` / `8150` 的每日階梯，
21 個交易日 × 4 檔 ＝ 84 次分析全部 201）：0050 的七階跑不出任何血緣邊，原因是**階距**
而不是選錯 symbol——週距下 zone 早漂到不重疊，直接走「缺席→失格」，2→2 的元件組不起來。
改成每日階梯後拿到 57 條血緣邊（`6182` 29、`2330` 19、`8150` 9、`3105` 0），
**`ZONE_IDENTITY_ENDED` 收攤路徑執行了 4 次**（6182 的四條 `SUPPORT_RECLAIM`，
parent 都是 `RESHAPED`），寫出來的 `state` / `active` / `end_reason` 三個欄位一致。

判讀時要知道的一塊：**`matched_by_alias` 在兩輪階梯都是 0**。既有鏈優先那一段就把
兩個成因都接住了，即使在 57 條血緣邊的高 churn 資料上也沒有輪到 alias 決定關聯，
它目前只有單元測試覆蓋。

這組數字在 F5 修法（alias 索引排除本輪 `expired_previous`）之後**逐項重現**——
同一組四檔 21 階重跑一次，身分數、血緣邊、事件鏈、transitions、alias 筆數與
`ZONE_IDENTITY_ENDED` 次數全部相同，六條門檻也仍然全過。

**alias 索引的「還活著」與 matcher 用同一個定義。** 這件事踩過一次坑：
`listZoneKeyAliasesSQL` 原本只看 `state='ACTIVE' AND ended_at IS NULL`，而
「失格只收掉這一世、身分本身仍是 ACTIVE」是階段 B 的定案，於是 `observed_absences`
已經超過上限的身分照樣留在索引裡——84 次分析累出 77 筆 `alias_ambiguous`、
16 個 `zone_key` 對到多個 ACTIVE 身分。**只排除本輪 `expired_previous` 補不起來**：
`ListLive` 的次數軸讓失格身分下一輪就不再進 matcher，所以一個身分一生只會被列進
`expired_previous` 一次，之後就永遠沉在索引裡。

現在兩道過濾各管一段：`ListKeyAliases` 的 SQL 用**與 `ListLive` 相同的**
`observed_absences <= zoneIdentityMaxAbsences` 擋掉已經沉下去的，呼叫端再用
`expired_previous` 擋掉這一輪剛失格、次數還沒推過上限的。同一組四檔 21 階在退乾淨的
dev DB 上重跑後，**`alias_ambiguous` 由 77 降到 0**（整輪 84 次分析一筆 warn 都沒有），
而身分數／血緣邊／事件鏈／alias 筆數與 `ZONE_IDENTITY_ENDED` 次數逐項不變。

判讀時注意：修法改的是**查詢路徑**不是資料，`zone_key_aliases` 裡失格身分的列仍在，
所以不帶次數上限直接對 DB group 一樣會數到 16 個撞號 `zone_key`。
要看這條有沒有生效，看 `alias_ambiguous` 的 warn 是否消失。

**逐段命中數只有 warn 時看得到。** `logging` 把 level 寫死 `zap.InfoLevel`，
而完整欄位的 `event identity: zone association` 是 Debug 級別，只有觸發 warn 的那次分析
才會印出整組欄位。所以上面的命中數是 warn 樣本內的統計；全量觀測要等 T-050 的 metric。

要重跑這套驗證，步驟見 [`development-workflow.md`](./development-workflow.md)
「在 dev stack 上做『as-of 階梯』驗收」與其中的「as-of 階梯驗收的六條門檻」。
