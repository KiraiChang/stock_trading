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
  `POST /sr-scoring/train`）
- Go 持久化：`backend/internal/analysis/client.go`（`ScoreZones`/
  `TrainModel`）、`backend/internal/store/sr_zone_repo.go`、
  `backend/internal/store/sr_scoring_train_job_repo.go`（訓練任務追蹤）
- Go 驗證：`backend/internal/analysis/sr_zone_verifier.go`（`SRZoneVerifier`，
  見「十四」），排程整合見 `backend/internal/scheduler/scheduler.go`
- API：`POST /api/v1/sr-zones`、`GET /api/v1/sr-zones`、
  `GET /api/v1/sr-zones/:id`、`POST /api/v1/sr-zones/:id/verify`、
  `POST /api/v1/sr-zones/train`、`GET /api/v1/sr-zones/train-jobs`、
  `GET /api/v1/sr-zones/train-jobs/:job_id`、
  `DELETE /api/v1/sr-zones/:id`（見 api-reference.md）
- 資料表：`stock_sr_zone_analyses`、`stock_sr_zones`、
  `sr_scoring_train_jobs`（見 database-schema.md）
- 前端：「支撐/壓力機率分析」頁面（`SRZones.svelte`）

---

## 整體流程

```
score_symbol(symbol, timeframe)
    ↓ fetch_candles（重用 db.py）
    ↓ ATRZoneBuilder().build(df) + VolumeProfileZoneBuilder().build(df)
    ↓ 依寬度分 3 個 Tier（見「六、Zone Tier」）
    ↓ 對每個 zone：
    │     compute_zone_features()（角色解析前後各算一次：support 視角 / resistance 視角）
    │     confidence()（多因子，跟角色無關，兩個視角共用同一個值）
    │     get_model() 預測 hold/break 機率 → 正規化 → 依 confidence 收縮成 support_score/resistance_score
    │     net_score = support_score - resistance_score
    │     依現價判斷 role（SUPPORT / RESISTANCE / AT_ZONE）
    │     role != AT_ZONE 才算：bounce/break probability、EV、RR、reward/risk percentile、volume_confirmation
    │     recent_validation、zone_momentum/zone_direction
    │     trading_score_breakdown → trading_score → trading_recommendation
    ↓ 依 Tier 排序，同層內依 trading_score 排序
    ↓ 用所有 zone 算出唯一的 Global Model（見「七、Global Model」）
    ↓ 回傳 {symbol, current_price, global_*, zones: [...]}
```

模型未訓練時 `get_model()` 會直接拋 `RuntimeError`（fail-fast），`/sr-zones`
回 `503`，不會靜默回傳中性機率——寧可讓呼叫端明確知道「還沒訓練」，也不要
給一個看起來正常、實際上沒有意義的數字。

---

## 一、Zone 建立（Zone Builder）

`zone_builder.py` 提供兩種獨立方法，各自產生候選 zone，方法不同，不跨方法
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
]
```

`model.py::train_model(dataset, model_type)` 訓練**兩個獨立分類器**：
`hold_model`（預測反彈/支撐延續）與 `break_model`（預測跌破/突破延續），
`model_type` 可選 `gradient_boosting`（預設）或 `logistic_regression`。
訓練同時算出 `rr_reference`——所有 `abs(bounce/break)` 非零樣本的排序陣列，
存進 `ModelBundle`，供即時評分時查百分位（見「八」）。

`MODEL_VERSION = "v2"`：v1 的 feature schema 用單一 `avg_return_after_touch`，
跟 v2 的雙欄位（`average_bounce_return`/`average_break_return`）不相容，
**v1 模型檔不能直接套用在新版程式碼上，需要重新訓練**（`python -m
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

---

## 七、Support/Resistance Score 與 Net Score

`net_score = support_score - resistance_score`，`net_score_label` 依門檻
（`±0.15`）分類成 `STRONG_SUPPORT` / `NEUTRAL` / `STRONG_RESISTANCE`——避免
只看單一分數判斷，要求使用者同時比較兩邊的強度。

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

---

## 十二、Trading Score（可拆解）與 Trading Recommendation

```
Trading Score = EV(40%) + RR(20%) + Trend(15%) + Volume(15%) + Confidence(10%)
```

每個分量先正規化到 `[0,1]`（`_normalize_signed`：0.5 為中性，±cap 為 0/1
分），再乘上對應權重，回傳值就是「這個分量對總分的實際貢獻」，直接存在
`trading_score_breakdown`（五個分量加總 = `trading_score`），使用者可以逐項
檢視分數怎麼來的，不用自己再乘一次權重：

| 分量 | 權重 | 正規化方式 |
|---|---|---|
| `expected_value` | 40% | `expected_value` 正規化（cap=0.05）；`role=AT_ZONE` 或缺值時用中性值 0.5 |
| `risk_reward` | 20% | `min(1, risk_reward_ratio / 3.0)`；缺值時用中性值 0.5 |
| `trend` | 15% | `overall_trend` 正規化（cap=0.1），`role=RESISTANCE` 時取負號對齊方向 |
| `volume` | 15% | `volume_confirmation` 查表（CONFIRMED=1.0／NEUTRAL=0.5／WEAK=0.3／FAILED=0.0）；缺值時用中性值 0.5 |
| `confidence` | 10% | 直接用 `confidence`（本身已經是 0~1） |

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

## 十三、Global Model（只有一個）

同一次分析只有**一個**權威的整體評估區塊，不在每個 zone 重複輸出：

| 欄位 | 計算方式 |
|---|---|
| `global_trend` | `trend_slope(df)` 只算一次（股票層級量） |
| `global_volatility` | `zone_volatility(df)` 只算一次（股票層級量） |
| `global_expected_value` | 對所有「有明確方向」（role != AT_ZONE 且 `expected_value` 非 None）的 zone，用 confidence 加權平均：`Σ(zone_EV × zone.confidence) / Σ(zone.confidence)` |
| `global_confidence` | 所有 zone confidence 的**簡單平均**（不分角色，不用 confidence 加權自己，避免循環） |
| `global_risk_reward_ratio` | 比照 `global_expected_value`，用同一套 confidence 加權平均 `risk_reward_ratio` |

zones 為空、或都沒有明確方向時，`global_expected_value`/`global_confidence`/
`global_risk_reward_ratio` 可能是 `null`。這是為了讓「Final EV」唯一收斂：
不再需要使用者自己判斷「要看哪個 zone 才代表這檔股票」。

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

## 十四、Zone 生命週期驗證（Verifier）

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

## 已知限制

- **`atr_width_multiplier`/`max_merge_width_multiple` 需要依實際股票調參**：
  這兩個常數目前是全域預設值（1.5／2.0），對不同價位、不同波動度的股票
  可能需要不同的合理範圍，尚未针對大規模真實資料做系統性調參。
- **`reject_count`/`break_count`（API/DB 輸出欄位）跟 `rejection_count`/
  `breakout_count`（`ZoneFeatures` 訓練特徵欄位）是兩個不同層級的欄位**，
  只是名稱相似，容易誤認為改名或型別不一致，實際上一個是 ML 特徵、一個是
  API 輸出（`scoring.py::score_zone` 裡有做對應）。
- **Python `POST /sr-scoring/train` 與 Go `POST /sr-zones/train`
  是同一個功能的兩個不同路徑段**（`sr-scoring` vs `sr-zones`），依語言邊界
  命名不同，容易混淆，撰寫新文件或程式碼時要留意分清楚是哪一層。
