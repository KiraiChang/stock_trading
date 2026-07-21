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
`scoring.py` 提供 `_trading_score_breakdown_no_direct_chip` 作為離線比較用 shadow
policy。該 policy 移除直接 `chip` 分量，並把其餘分量恢復為 EV 40% / RR 20% /
Trend 15% / Volume 15% / Confidence 10%。後續若要調整 production 權重，應先比較
現況與 shadow policy 的 top1/top3 zone 排名、摘要支撐/壓力選擇、分數差異分布，
再決定是否移除或調低直接 `chip` 權重。

**摘要 `reasons` 不再含籌碼句**：籌碼從 `_zone_summary` 的 `reasons[]` 拉出改成上述
結構化 `chip` 欄位，`reasons` 只保留均線、驗證、量能、信心、共振等非籌碼理由，
避免同一件事在文字與數字兩處重複。整檔跑馬燈 `analysis_tips` 仍保留一句白話籌碼
提示（`_chip_reason`）。偏多/偏空門檻統一為 `CHIP_SIGNAL_THRESHOLD`（±20，對齊
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

`decision_summary.market_bias` 是對外的多空傾向標籤（`BULLISH_BIAS` / `BEARISH_BIAS` /
`NEUTRAL_BIAS` / `REVERSAL_BIAS` / `BULLISH_CONTINUATION`）。當 `short_term_regime` 為 `RECOVERY`
或 `EARLY_TREND` 時會輸出 `BULLISH_CONTINUATION`（多頭延續），避免延續型多頭被標成反轉觀察。
但此升級只在 `market_action != AVOID` 時生效：若長期偏空（`primary=TREND_DOWN`）使 action 落到
`AVOID`，即使短線是收復確認，`market_bias` 也會回歸與 action 一致的偏空標籤，確保
`market_bias`、`market_action`、`final_entry_permission` 三者語意一致，不會出現「多頭延續 bias +
避開 action」的矛盾輸出。

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
P1 的無資料表 lifecycle 摘要，用來區分 active / resolved event：

- `states`：每個 event family / zone key 的最新狀態。
- `active`：目前仍有效的事件狀態。
- `resolved`：已被後續事件解除的狀態。
- `active_bearish_events`：Decision hard gate 會使用的 active bearish risk。
- `market_state`：由 active event state 推導的短線市場狀態。

同一 zone 若出現 `HIGH_VOLUME_BREAKDOWN → INTRADAY_RECLAIM → REVERSAL_CANDIDATE`，
raw `market_events` 仍保留完整鏈，但 `HIGH_VOLUME_BREAKDOWN` 在
`event_state_summary` 會變成 `RESOLVED`，不得再作為 active bearish gate 永久強制
`EXIT`。未被收復的 active breakdown 仍會讓 `price_path.path_state=EVENT_RISK` 並降風險。

Decision gating 一律只消費 `event_state_summary.active`：primary zone 選擇
（`_pick_primary_zone`）、market action（`_decision_action`）、entry action state、
market bias（`_market_bias`）與 event-aware entry relevance（`_entry_relevance_score_with_events`）
都吃 active 事件集合，已被 resolve 的 breakdown 不會再懲罰 relevance 或翻空 bias。完整 raw
event chain 保留給對外呈現（`market_events` / `event_sequence` / `event_state_summary`）。

例外（刻意）：`_daily_candidate_zones` 與 `_defense_lines` 仍消費完整 raw `market_events`——
前者用「歷史上出現過 `INTRADAY_RECLAIM` / `REVERSAL_CANDIDATE`」決定是否補日 K 候選區，後者用最近
微結構事件的 zone_ref 定位戰術防守線；兩者是「候選區產生」與「防守線呈現」，不是進場 gating，需要
完整事件脈絡才完整，故與「gating 只吃 active」並存而不矛盾。

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
「動能是否確認」。

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

### 唯一 Action

`action` 是整份分析唯一的操作結論，避免使用者自行從多張 zone 卡片拼湊結果。目前限定四種：

| Action | 語意 | 典型條件 |
|---|---|---|
| `Buy` | 條件完整，可正常部位進場 | regime 偏多或區間下緣、primary support 高信心、EV/RR 正向、波動與風險可控 |
| `BuySmall` | 條件偏正向但仍有明顯保留 | 偏多但波動高、confidence 未達很高、EV/RR 普通、或距離主支撐略遠 |
| `Hold` | 不追價，等待更好的價格或確認 | 方向不差但現價不在合理風險報酬位置、primary zone 不夠近、或支撐/壓力訊號混合 |
| `Avoid` | 不建議操作 | regime 偏空、primary zone 失效、低信心且高波動、EV/RR 明顯不佳、或價格接近強壓但缺乏突破證據 |

Action 應由 Market Regime、primary zone、`entry_relevance_score`、market events、`zone_quality_score`、`confidence`、`expected_value`、`risk_reward_ratio`、`chip_summary` 與風險條件共同決定。`trading_score` 保留為 legacy quality score，不得單獨直接決定 `Buy` / `BuySmall`。若任一核心資料缺失，預設應保守降級，例如 `Buy` 降為 `BuySmall`，`BuySmall` 降為 `Hold`。

`position_action=HOLD` 不代表無條件持有；若有 `position_action_condition`，前端必須顯示為「條件式持有」，並列出 `invalidation_price`（防守線）、`recovery_price`（回穩線）與 `reason_codes`。

`entry_action_state` 是進場階段語意，不取代 `action` / `market_action`：

| State | 語意 |
|---|---|
| `WAIT_CONFIRMATION` | 等待確認，不進場 |
| `PROBE_ENTRY` | 仍在待確認語境，只能視為觀察性試探 |
| `SMALL_ENTRY` | 條件普通但已確認，可小量進場 |
| `ACCUMULATE` | 條件完整但仍適合分批累積 |
| `BUY` | 條件完整，可正常買進 |

若 zone 是 `PENDING_VALIDATION` 或 position reason 含 `SUPPORT_RECLAIM_AWAIT_CONFIRMATION`，即使 legacy `action=BuySmall`，`entry_action_state` 也不得高於 `PROBE_ENTRY`，避免「尚待確認」與「小量試單」語意衝突。

`final_entry_permission` 是 `entry_action_state` 與 `daily_confirmation.state` 的保守仲裁結果，前端若要顯示
「是否允許進場」應優先讀此欄位；legacy `entry_action_state` / `daily_entry_state` 保留給明細與相容。
若 daily confirmation 為 `INVALIDATED`，final permission 會降為 `NO_SETUP`。`BUY` 只在 entry 端與
daily 端都達最高層級時輸出；一般 `ENTRY_READY` 只會把 `BUY` 放行到 `ACCUMULATE`，避免繞過日 K
把關。現階段 `daily_confirmation` 最高只輸出 `ENTRY_READY`（不產生 `BUY_READY`），因此
`final_entry_permission.state` 目前上限為 `ACCUMULATE`；`BUY` 為保留態，待 daily 端加入 `BUY_READY`
（例如兩日確認完成）後才會端到端啟用。

目前 action pipeline：

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
`position_rr` 用於持股防守、減碼或續抱語境。現階段尚未接入實際 position zone 來源時，
`position_rr=null` 且 `position_rr_source=UNAVAILABLE`，避免把 entry RR 誤讀成既有部位 RR。
接上持股防守區後才可輸出 `position_rr_source=POSITION_ZONE`。

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
`price_low`/`price_high`、`role`、`source`、`lifecycle`、`decision_role`、
`distance_pct`、`reason`、`event_refs` 與完整 `candidate_json`。

P2-C-5 起，`stock_sr_decisions` 也保存尚未拆成獨立表、但前端決策面需要的 detail JSON：
`market_regime_json`、`data_quality_json`、`event_sequence_json`、`daily_price_action_json`、
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

`scoring.py::_group_overlapping_zones()`（union-find）只比較**不同 method**
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
