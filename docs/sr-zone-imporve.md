# SR Zone 改善實作計畫書

> 狀態：P0、P1、P2 核心 decision summary 欄位已實作；需額外資料來源或回測 pipeline 的校準項目改由 `docs/todo.md` T-026 追蹤。此文件保留原始案例與修正建議，作為後續校準與回測擴充依據。

## 目標

將目前 SR Zone 決策輸出從「單一主交易區 + Market Action」調整為「盤勢傾向、進場狀態、部位處置、決策區分層」分工明確的模型，避免把觀察訊號誤讀成下單建議，並讓 0050、00947、2330 類型案例都能用一致語意解釋。

## P0：語意與風險邊界修正

1. 將 UI 與 API 新增 `Market Bias`，值域為 `BULLISH_BIAS`、`NEUTRAL_BIAS`、`BEARISH_BIAS`、`REVERSAL_BIAS`，只描述盤勢傾向，不使用試單、買進、加碼等執行語意。
2. 保留 legacy `market_action` 供相容，但 UI 主顯示改讀 `market_bias`；進場可行性只由 `entry_action_state` 表示。
3. 拆分決策區：
   - `nearest_decision_zone`：離現價最近且可作為短線判讀的 zone。
   - `primary_structural_zone`：主要結構區，偏向高品質、主要趨勢結構。
   - `best_trade_zone`：通過 RR 與進場條件後才成立的交易候選區。
4. 新增 RR gate，輸出最低 RR、是否通過與原因碼；RR 未達門檻時不得把 zone 顯示成 `best_trade_zone`。
5. 新增資料品質與盤後資料模式欄位，明確標示 EOD daily K 限制、缺值資料與中性判讀的差異。
6. 將 confluence 摘要統一為「證據族群」口徑，raw method count 僅保留在明細或 legacy 欄位。

## P1：日 K 決策能力補齊

1. 建立 daily price action 判讀，至少輸出 close location、range expansion、reclaim/rejection、two-day follow-through、gap 與窄幅整理等狀態。
2. 輸出事件鏈，串接 `EXTREME_VOLUME`、`HIGH_VOLUME_BREAKDOWN`、`INTRADAY_RECLAIM`、`REVERSAL_CANDIDATE`，供 explainability 與 regression test 使用。
3. 建立 daily candidate zone 補位邏輯，當既有 zone 無法回答「今天決策價位」時，產生日內高低、收盤收復、gap、均價/成交密集等候選區。
4. 補上 zone role/source/lifecycle，區分 structural、session、event、derived 等來源，以及 active、tested、broken、reclaimed、expired 等生命週期。
5. 將 structural score、relevance score、tradability score 分離，避免長期重要區被誤當成立即可交易區。

## P2：校準與驗證

1. 建立 price path 與 invalidation/recovery/next decision price 輸出，讓 decision 能說明「若跌破/站回/接近下一區」後的狀態轉換。
2. 建立日 K confirmation rule，將 watch / watchlist / actionable 分層，避免單日訊號直接升級成買進。
3. 加入缺資料矩陣與資料品質分數，將 unavailable、neutral、negative 分開呈現。
4. 擴充 backtest label 與 acceptance tests，覆蓋 0050、00947、2330 三類案例與 RR gate、event sequence、zone split 不變量。

## 驗收條件

1. UI 不再以 `Market Action` 顯示買賣語意，改以 `Market Bias` 表示盤勢傾向。
2. `entry_action_state` 才能表達是否可試探、等待確認、小量、加碼或買進。
3. RR 不足的 zone 不會出現在 `best_trade_zone`。
4. 缺值資料會進入 `data_quality`，不會被誤寫成看多、看空或中性。
5. 原有 `market_action`、`primary_zone`、`secondary_zones` 保持相容，既有呼叫端不因新增欄位破壞。

# 0050 的修正建議
1. P0

* 把 Market Action 改成 Market Bias，禁止使用「試單、買進、加碼」等執行語意。

目前 Market Action 還是叫「小量試單」

```
BULLISH_BIAS       偏多觀察
NEUTRAL_BIAS       中性觀察
BEARISH_BIAS       偏空觀察
REVERSAL_BIAS      反轉觀察
```

* 目標
```
層級	| 回答問題
Market Bias	| 市場傾向是什麼
Entry State	| 現在能不能進場
Position Action |	已有部位怎麼做
```

2. P0

* 統一 confluence 顯示：摘要使用 證據族群 ×4，raw method count 只放明細。

3. P1

* 仍缺少事件層輸出，輸出事件鏈：
```
EXTREME_VOLUME
HIGH_VOLUME_BREAKDOWN
INTRADAY_RECLAIM
REVERSAL_CANDIDATE
```
* 舉例
```
昨量 65,388
今量 225,043
相對昨量約 3.44x

跌破 105.4～105.7
最低 102.25
均價 103.58
收回 104.40
```
所以 Decision Engine 應該至少輸出：
```
Volume Event
EXTREME_VOLUME

Break Event
HIGH_VOLUME_BREAKDOWN

Recovery Event
INTRADAY_RECLAIM

Composite Event
REVERSAL_CANDIDATE
```

目前 SUPPORT_RECLAIM_AWAIT_CONFIRMATION 比較像結果狀態，但缺少原因事件鏈。

建議 UI 新增一列：
```
Event Sequence
放量破位 → 盤中收復 → 反轉候選
```

這對 explainability 與 regression test 都很重要。

4. P1

* 拆分三個市場層級：
```
Structural Trend
Tactical Regime
Recovery State
```

* 舉例
```
Structural Trend
偏多

Tactical Regime
短線破位

Recovery State
支撐收復候選
```

# 00947 的修正建議

目前 Zone Engine 不修改。

修正 Decision / Regime semantic consistency：

1. Structure State invariant

若：
- close < MA5
- close < previous close
- intraday drawdown 達 high volatility threshold
- Primary Zone 為 resistance 且 current price 位於 zone 下方

StructureState 不得為 NORMAL。

應輸出 BREAKDOWN 或 SHORT_TERM_WEAKNESS。

2. Zone price semantic

Support Zone 使用：
- DefensePrice
- RecoveryPrice

Resistance Zone 使用：
- TestPrice
- BreakoutPrice

禁止 Resistance Zone 顯示「防守線」。

Primary Resistance 36.28~36.50 應顯示：
TestPrice = 36.28
BreakoutPrice = 36.50

3. Position invariant

position.quantity == 0
=> PositionAction = NO_POSITION

NO_POSITION 時 UI 隱藏 Position Action。

新增 unit test。

4. Explanation vocabulary 必須受 MarketAction 約束。

AVOID：
禁止出現「小量試單」、「買進」、「可進場」。

WATCH：
只能使用「觀察」、「等待確認」。

BUY_SMALL：
才允許「小量試單」。

BUY：
才允許「進場」。

目前 EV < 0 且 MarketAction = AVOID，
風險提醒應改為：
「主交易區預期價值為負，目前不建議試單。」

不要修改 Zone Score、Zone Ranking、Feature Weight。

# 2330 的修正建議
# 股票支撐壓力決策系統修改方向建議書

## 日 K 與盤後資料版本

## 一、文件目的

本文件針對目前股票支撐壓力分析系統提出可實際落地的修改方向。

現階段系統沒有穩定的盤中交易資訊來源，因此本次修改計畫不納入：

* 逐筆成交資料
* 1 分鐘或 5 分鐘 K 線
* 即時 VWAP
* Opening Range
* 盤中 Volume Profile
* 盤中高低點即時辨識
* 即時突破或跌破監控

本次改版僅使用目前可取得的資料：

* 日 K OHLCV
* 每日均價
* 昨收
* 成交量與歷史均量
* ATR、均線、波動率
* 歷史支撐壓力區
* 盤後法人資料
* 融資融券資料
* 分點與集中度資料，如資料來源可取得
* 歷史回測結果

修改目標不是產生盤中買賣訊號，而是讓盤後 Decision Engine 更準確地回答：

1. 下一交易日最接近、最需要觀察的區間在哪裡？
2. 哪個區間是真正的波段結構支撐或壓力？
3. 目前是否存在符合風險報酬的交易區？
4. 缺失資料是否影響本次判斷可信度？
5. 最終 Action 與 Reason Codes 是否一致？

本次台積電案例中，系統將 2318.03～2331.97 選為主要交易區，Score 約 69、EV 約 +3.25%、RR 約 1.29R；同時又提示風險報酬不足。模型最終輸出「等待」基本合理，但主要區間的角色與決策用途不夠清楚。

---

# 二、現階段可處理與不可處理的範圍

## 2.1 現階段可以處理

### 日 K 價格行為

可使用：

```text
Open
High
Low
Close
Daily Average Price
Previous Close
Volume
```

辨識：

* 長下影線
* 長上影線
* 收盤位置
* 收復日均價
* 跌破日均價
* 跳空
* 高量反轉
* 低量整理
* 大實體 K 棒
* Inside Bar
* Outside Bar
* 前低或前高防守

### 歷史 Zone 分析

可處理：

* 歷史轉折點
* 成交密集區
* ATR Zone
* 均線群聚區
* Fibonacci 區間
* 歷史觸碰次數
* 歷史反彈與跌破機率
* 區間近期性
* 區間穩定度
* 多方法共振

### 盤後決策

可處理：

* 下一交易日觀察區
* 結構支撐與壓力
* 進場等待條件
* 停損參考
* 風險報酬判斷
* 條件式持有
* 減碼或退出建議
* 資料完整度警告

## 2.2 現階段不可處理

沒有盤中資料時，不應宣稱能辨識：

* 盤中低點是否有連續主動買盤
* 盤中突破是否有真實量價確認
* 分時 VWAP 是否被重新站回
* 開盤 30 分鐘區間突破
* 盤中委買委賣強弱
* 即時大量單或主力進出
* 分鐘級假突破
* 精確盤中進場時點

因此 UI 與文案應避免：

```text
盤中承接已確認
突破已確認
VWAP Reclaim 已成立
目前可以直接進場
```

應改成：

```text
日 K 顯示低檔承接跡象
下一交易日仍需確認
盤後形成候選支撐
尚未具備盤中確認資料
```

---

# 三、目前系統的主要問題

## 3.1 Primary Zone 定義過度混合

目前單一 `Primary Zone` 同時承擔：

* 歷史最強區間
* 最近區間
* 最佳交易區
* 結構防守區

這四者不一定相同。

例如台積電分析中：

```text
現價：2420
主要支撐：2318.03～2331.97
距離：約 3.6%
RR：1.29R
```

這個區域可能是重要歷史支撐，但不代表它是下一交易日最需要先觀察的價位，也不代表它已經是適合進場的交易區。

## 3.2 日 K 最新資訊沒有成為獨立區間

即使沒有分鐘資料，最新日 K 仍提供大量有效資訊。

台積電當日：

```text
Open：2410
High：2430
Low：2390
Average：2411
Close：2420
```

可計算：

```text
收盤位置 = (Close - Low) / (High - Low)
         = (2420 - 2390) / (2430 - 2390)
         = 0.75
```

代表收盤位於當日區間上方 25%。

同時：

```text
Close > Average Price
Close > Open
```

這些訊號可以建立「日 K 候選支撐」，但不能視為盤中確認。

## 3.3 RR 不足卻仍顯示主交易區

系統一方面說：

```text
主交易區風險報酬比不足
```

另一方面又說：

```text
目前最具決策意義的主交易區
```

語意容易讓使用者誤解成「雖然 RR 不足，但仍然可以考慮交易」。

應將：

* 結構重要性
* 接近程度
* 可交易性

拆成不同欄位。

## 3.4 Missing Data 與 Neutral Data 混淆

籌碼顯示：

```text
法人：-7
融資：0
分點：0
集中度：0
```

必須區分後三者是：

* 真正計算後為 0
* 尚未取得資料
* 資料已過期
* 樣本不足

否則模型會把未知錯當成中性。

---

# 四、修改後的目標架構

在沒有盤中資料的情況下，建議使用以下架構：

```text
Daily Market Data
        │
        ▼
Historical SR Engine
        │
        ├── 歷史結構區間
        ▼
Daily Price Action Engine
        │
        ├── 最新日 K 候選區間
        ▼
Zone Fusion and Classification
        │
        ├── 合併、分類、去重
        ▼
Zone Ranking Engine
        │
        ├── Nearest Decision Zone
        ├── Primary Structural Zone
        └── Best Trade Zone
        ▼
Decision Engine
        │
        ├── Market Action
        ├── Entry State
        ├── Position Action
        └── Reason Codes
        ▼
Explanation and Data Quality
```

這個架構不依賴盤中資料，也能明顯改善現有決策品質。

---

# 五、修改方向一：Primary Zone 拆分

建議取消單一 `primary_zone` 作為所有決策的主要來源。

改成三種區間。

## 5.1 Nearest Decision Zone

定義：

> 依照盤後資料判斷，下一交易日價格最可能優先面對的有效區間。

可能來源：

* 最新日 K 的低點與均價
* 最新日 K 的高點與均價
* 前一日高低點
* 最近 3～5 日高低點
* 最近有效歷史 Zone
* 短期均線區
* ATR 區間

特性：

* 距離現價較近
* 不一定歷史強度最高
* 不一定符合進場條件
* 主要作為下一交易日觀察線

## 5.2 Primary Structural Zone

定義：

> 歷史強度較高，跌破或突破後可能改變波段結構的支撐或壓力區。

主要依據：

* Touch Count
* Reject Count
* Break Count
* Recency
* Stability
* 多方法共振
* 成交量證據
* 歷史反彈或突破結果

台積電原本的：

```text
2318.03～2331.97
```

較適合被標示為：

```text
Primary Structural Support
```

而不是主要交易區。

## 5.3 Best Trade Zone

定義：

> 同時符合機率、EV、RR、趨勢與確認條件的交易區。

必要條件：

```text
Zone 未失效
EV > 0
RR >= 最低門檻
方向與 Market Regime 不衝突
Entry Confirmation 成立
資料完整度達最低標準
```

不符合時：

```text
Best Trade Zone = NONE
```

台積電案例建議輸出：

```text
Nearest Decision Zone：
2390～2411，日 K 候選支撐

Primary Structural Zone：
2318～2332，歷史結構支撐

Best Trade Zone：
暫無
```

---

# 六、修改方向二：新增 Daily Price Action Engine

## 6.1 功能定位

Daily Price Action Engine 使用每日 OHLCV 與日均價，建立最新一根日 K 所形成的候選區間。

它不是盤中引擎，因此不應輸出：

```text
即時承接確認
盤中突破確認
```

而應輸出：

```text
日 K 候選支撐
日 K 候選壓力
隔日等待確認
```

## 6.2 建議輸入結構

```go
type DailyBar struct {
    Symbol        string
    TradeDate     time.Time
    Open          float64
    High          float64
    Low           float64
    Close         float64
    AveragePrice  *float64
    Volume        float64
    PreviousClose float64

    ATR14         *float64
    AvgVolume5    *float64
    AvgVolume20   *float64
    MA5           *float64
    MA10          *float64
    MA20          *float64
}
```

## 6.3 建議計算特徵

### A. Close Position

```text
ClosePosition = (Close - Low) / (High - Low)
```

特殊情況：

```text
High == Low 時回傳 null，不可除以 0
```

參考分級：

```text
>= 0.75：收盤偏強
0.60～0.75：略偏強
0.40～0.60：中性
0.25～0.40：略偏弱
< 0.25：收盤偏弱
```

### B. Candle Body Ratio

```text
BodyRatio = abs(Close - Open) / (High - Low)
```

用於區分：

* 大實體趨勢 K
* 小實體整理 K
* 十字線
* 長影線反轉 K

### C. Lower Wick Ratio

```text
LowerWick = min(Open, Close) - Low
LowerWickRatio = LowerWick / (High - Low)
```

參考條件：

```text
LowerWickRatio >= 0.25
且
ClosePosition >= 0.60
```

Reason Code：

```text
DAILY_LOW_REJECTION
```

這只代表日 K 低檔承接跡象，不代表盤中買盤已確認。

### D. Upper Wick Ratio

```text
UpperWick = High - max(Open, Close)
UpperWickRatio = UpperWick / (High - Low)
```

參考條件：

```text
UpperWickRatio >= 0.25
且
ClosePosition <= 0.40
```

Reason Code：

```text
DAILY_HIGH_REJECTION
```

### E. Average Price Reclaim

若有每日均價：

```text
Low < AveragePrice
Close > AveragePrice
```

Reason Code：

```text
DAILY_AVERAGE_PRICE_RECLAIM
```

這裡必須使用 `Daily Average Price`，不可稱為即時 VWAP。

### F. Volume Ratio

```text
VolumeRatio5 = Volume / AvgVolume5
VolumeRatio20 = Volume / AvgVolume20
```

參考：

```text
>= 1.50：明顯放量
1.20～1.50：溫和放量
0.80～1.20：正常
< 0.80：量縮
```

### G. Gap

```text
GapUp = Open > PreviousHigh
GapDown = Open < PreviousLow
```

如目前只保存昨收，至少可先做：

```text
OpenGapPercent = (Open - PreviousClose) / PreviousClose
```

---

# 七、修改方向三：以日 K 建立候選區間

## 7.1 Daily Support Candidate

建議條件：

```text
ClosePosition >= 0.60
且
LowerWickRatio >= 0.20
且
Close >= Open
```

如果有日均價：

```text
Close >= AveragePrice
```

可增加可信度，但不列為必要條件。

候選區間：

```text
Lower = Low
Upper = min(Open, AveragePrice)
```

若 AveragePrice 缺失：

```text
Upper = min(Open, Close)
```

台積電案例：

```text
Low = 2390
Open = 2410
Average = 2411
Close = 2420
```

候選支撐可定義為：

```text
2390～2410
```

或基於區間緩衝：

```text
2390～2411
```

Zone Type：

```text
DAILY_SUPPORT_CANDIDATE
```

## 7.2 Daily Resistance Candidate

建議條件：

```text
ClosePosition <= 0.40
且
UpperWickRatio >= 0.20
且
Close <= Open
```

候選區間：

```text
Lower = max(Open, AveragePrice)
Upper = High
```

## 7.3 區間寬度限制

日 K 候選區不能無限制過寬。

建議：

```text
最大區間寬度 = min(
    當日 Range 的 60%,
    ATR14 的 50%,
    Current Price 的 1.5%
)
```

若超過，應向中心收斂或降低 Confidence。

---

# 八、修改方向四：Zone Role 與 Zone Source

## 8.1 Zone Role

建議新增：

```go
type ZoneRole string

const (
    ZoneRoleTactical   ZoneRole = "TACTICAL"
    ZoneRoleStructural ZoneRole = "STRUCTURAL"
    ZoneRoleMacro      ZoneRole = "MACRO"
)
```

### Tactical

來源：

* 最新日 K
* 最近 1～5 日高低點
* 短期均線
* 最近突破位
* 最近跳空區

用途：

* 下一交易日或短線觀察
* 不代表已確認
* 優先考慮距離與近期性

### Structural

來源：

* 多次歷史觸碰
* 高拒絕率
* 大成交密集區
* 多方法共振
* 中短期歷史平台

用途：

* 波段結構防守
* 跌破後可能改變趨勢

### Macro

來源：

* 週線或月線
* 長期成交密集區
* 長期均線
* 大型歷史平台

用途：

* 中長期資產配置
* 大幅修正時參考

## 8.2 Zone Source

```go
type ZoneSource string

const (
    ZoneSourceHistoricalPivot ZoneSource = "HISTORICAL_PIVOT"
    ZoneSourceDailyCandle     ZoneSource = "DAILY_CANDLE"
    ZoneSourceMovingAverage  ZoneSource = "MOVING_AVERAGE"
    ZoneSourceVolumeProfile  ZoneSource = "DAILY_VOLUME_PROFILE"
    ZoneSourceFibonacci      ZoneSource = "FIBONACCI"
    ZoneSourceGap            ZoneSource = "GAP"
)
```

其中 `DAILY_VOLUME_PROFILE` 只有在你能由日資料建立近似成交量分布時才使用；若沒有可靠資料，不應硬產生。

---

# 九、修改方向五：Zone Lifecycle 簡化版

沒有盤中資料時，Lifecycle 應以收盤價與隔日資料更新。

建議：

```go
type ZoneLifecycle string

const (
    ZoneLifecycleCandidate   ZoneLifecycle = "CANDIDATE"
    ZoneLifecycleConfirmed   ZoneLifecycle = "CONFIRMED"
    ZoneLifecycleValidated   ZoneLifecycle = "VALIDATED"
    ZoneLifecycleWeakening   ZoneLifecycle = "WEAKENING"
    ZoneLifecycleBroken      ZoneLifecycle = "BROKEN"
    ZoneLifecycleInvalidated ZoneLifecycle = "INVALIDATED"
)
```

## 9.1 Candidate

最新日 K 建立，尚未經後續交易日測試。

## 9.2 Confirmed

後續一個交易日符合：

支撐區：

```text
Low 進入或接近 Zone
且
Close >= Zone Upper
```

壓力區：

```text
High 進入或接近 Zone
且
Close <= Zone Lower
```

## 9.3 Validated

符合以下其中一項：

* 已有兩次以上有效防守
* 歷史回測顯示穩定
* Reject Count 明顯高於 Break Count
* 最近一次測試仍有效

## 9.4 Weakening

支撐區可由以下條件觸發：

* 連續多次測試
* 每次反彈幅度下降
* Close 越來越接近下緣
* Reject Return 下降
* Break Probability 上升

## 9.5 Broken

支撐：

```text
Close < Zone Lower - BreakBuffer
```

壓力：

```text
Close > Zone Upper + BreakBuffer
```

BreakBuffer 建議：

```text
max(
    0.25 × ATR14,
    CurrentPrice × 0.5%
)
```

需透過回測校準，不建議寫死為最終值。

---

# 十、修改方向六：Zone Ranking 拆成三種分數

不應再用單一 Score 同時決定歷史重要性、近期性與可交易性。

## 10.1 Structural Score

用途：

> 找出最重要的歷史結構區。

建議：

```text
Structural Score =
    Historical Strength × 25%
  + Touch Quality × 15%
  + Reject Quality × 15%
  + Stability × 15%
  + Method Confluence × 15%
  + Volume Evidence × 10%
  + Recency × 5%
```

## 10.2 Decision Relevance Score

用途：

> 找出下一交易日最值得優先觀察的區域。

建議：

```text
Decision Relevance Score =
    Distance Score × 30%
  + Recency Score × 20%
  + Daily Price Action Score × 20%
  + Price Path Score × 15%
  + Zone Confidence × 15%
```

沒有盤中資料時，距離與日 K 近期性必須提高權重。

## 10.3 Tradability Score

用途：

> 判斷該區間現在是否適合作為交易設定。

建議：

```text
Tradability Score =
    RR Score × 30%
  + EV Score × 20%
  + Daily Confirmation × 15%
  + Probability Score × 15%
  + Regime Alignment × 10%
  + Data Quality × 10%
```

---

# 十一、修改方向七：Price Path Score

## 11.1 目的

避免較遠的結構區間壓過近端區間。

例如：

```text
現價：2420
近端日 K 支撐：2390～2411
歷史結構支撐：2318～2332
```

價格若向下，理論上先經過 2390～2411，再接近 2318～2332。

因此：

```text
2390～2411
```

應是 Nearest Decision Zone。

而：

```text
2318～2332
```

仍可保留為 Primary Structural Zone。

## 11.2 Blocking Zone

如果現價與目標 Zone 之間存在其他有效 Zone：

```text
每一個 Blocking Zone 扣除一定 Price Path Score
```

建議初始值：

```text
Validated Blocking Zone：扣 25
Confirmed Blocking Zone：扣 20
Candidate Blocking Zone：扣 10
```

實際值再透過回測校準。

---

# 十二、修改方向八：RR Gate

## 12.1 最低 RR

建議依交易類型分開：

```text
日 K 支撐反彈：最低 1.8R
日 K 壓力突破：最低 2.0R
波段支撐進場：最低 2.0R
觀察性試單：最低 1.5R
```

## 12.2 Gate 規則

當：

```text
RR < MinimumRR
```

系統仍可輸出：

```text
Nearest Decision Zone
Primary Structural Zone
```

但必須：

```text
Best Trade Zone = NONE
Entry State != ENTRY_READY
```

Reason Code：

```text
RR_INSUFFICIENT
```

台積電原始分析中 RR 為 1.29R，因此即使結構支撐 Score 約 69，也不應被表達為可交易區。

---

# 十三、修改方向九：Daily Confirmation

沒有分鐘資料時，Entry Confirmation 必須以日 K 為單位。

## 13.1 支撐反彈確認

可定義為：

```text
當日 Low 進入或靠近支撐區
且
Close 回到 Zone Upper 之上
且
ClosePosition >= 0.60
且
成交量不明顯萎縮
```

進一步確認：

```text
下一交易日 Close 不跌破前一日 Low
或
下一交易日 Close 高於前一日 Close
```

## 13.2 壓力突破確認

可定義為：

```text
Close > Resistance Upper + BreakBuffer
且
VolumeRatio20 >= 1.20
且
ClosePosition >= 0.70
```

為避免日 K 假突破，可要求：

```text
下一交易日 Close 仍高於 Resistance Upper
```

## 13.3 Entry State

```go
type EntryState string

const (
    EntryStateNoSetup          EntryState = "NO_SETUP"
    EntryStateWaitDailyConfirm EntryState = "WAIT_DAILY_CONFIRM"
    EntryStateProbeAllowed     EntryState = "PROBE_ALLOWED"
    EntryStateEntryReady       EntryState = "ENTRY_READY"
    EntryStateChasingRisk      EntryState = "CHASING_RISK"
    EntryStateInvalidated      EntryState = "INVALIDATED"
)
```

建議使用 `WAIT_DAILY_CONFIRM`，避免讓使用者誤以為系統正在等待盤中訊號。

---

# 十四、修改方向十：Missing Data Handling

## 14.1 Feature 資料結構

```go
type FeatureStatus string

const (
    FeatureStatusAvailable FeatureStatus = "AVAILABLE"
    FeatureStatusMissing   FeatureStatus = "MISSING"
    FeatureStatusStale     FeatureStatus = "STALE"
    FeatureStatusInvalid   FeatureStatus = "INVALID"
)

type FeatureValue struct {
    Value      *float64
    Status     FeatureStatus
    UpdatedAt  *time.Time
    Confidence float64
    Source     string
}
```

## 14.2 模型輸入

不要只輸入：

```text
branch_score = 0
```

應同時輸入：

```text
branch_score_value
branch_score_missing
branch_score_staleness
branch_score_confidence
```

## 14.3 籌碼分數

建議改為：

```text
Chip Score
Chip Data Completeness
Chip Confidence
```

例如：

```text
法人：-7，AVAILABLE
融資：MISSING
分點：MISSING
集中度：MISSING

Chip Score：-7
Chip Data Completeness：25%
Adjusted Chip Contribution：降低權重
```

不可把後三者視為中性 0。

---

# 十五、修改方向十一：Data Quality Gate

建議新增資料完整度。

```go
type DataQuality struct {
    OverallCompleteness float64
    PriceDataComplete   bool
    TechnicalCoverage   float64
    ChipCoverage        float64
    MissingFeatures     []string
    StaleFeatures       []string
}
```

## 15.1 建議規則

```text
資料完整度 >= 80%：
正常決策

資料完整度 60%～80%：
降低 Decision Confidence

資料完整度 40%～60%：
只能輸出觀察，不可輸出 Entry Ready

資料完整度 < 40%：
輸出 DATA_INCOMPLETE
```

價格資料與技術資料可給較高權重；籌碼資料缺失時不一定要停止分析，但應降低籌碼貢獻與整體信心。

---

# 十六、Decision Engine 建議流程

建議順序：

```text
1. 驗證資料品質
2. 判斷 Market Regime
3. 取得 Historical Zones
4. 產生 Daily Candidate Zones
5. Zone Fusion 與去重
6. 計算 Structural Score
7. 計算 Decision Relevance Score
8. 選出 Nearest Decision Zone
9. 選出 Primary Structural Zone
10. 計算 RR 與 EV
11. 執行 RR Gate
12. 判斷 Daily Confirmation
13. 選出 Best Trade Zone
14. 產生 Market Action
15. 產生 Entry State
16. 產生 Position Action
17. 產生 Reason Codes
```

---

# 十七、台積電案例的建議輸出

根據現有日 K：

```text
Open：2410
High：2430
Low：2390
Average：2411
Close：2420
```

系統可計算：

```text
ClosePosition：0.75
Close > Open
Close > Average
存在低檔收復跡象
```

但因只有單日日 K，不足以確認支撐有效。

建議輸出：

```text
Market Regime：
偏多趨勢

Market Action：
觀察

Entry State：
等待日 K 確認

Position Action：
條件式持有

Nearest Decision Zone：
2390～2411
TACTICAL / CANDIDATE
來源：DAILY_CANDLE

Primary Structural Zone：
2318～2332
STRUCTURAL / VALIDATED
來源：HISTORICAL_SR

Nearest Resistance：
2473～2487

Best Trade Zone：
暫無

Reason Codes：
DAILY_LOW_REJECTION
DAILY_AVERAGE_PRICE_RECLAIM
STRONG_CLOSE_LOCATION
TACTICAL_SUPPORT_CANDIDATE
ENTRY_NOT_CONFIRMED
RR_INSUFFICIENT
```

需要注意：

```text
DAILY_LOW_REJECTION
```

表示日 K 型態，不是盤中即時承接確認。

---

# 十八、API 回傳格式建議

```json
{
  "symbol": "2330",
  "trade_date": "2026-07-14",
  "current_price": 2420,
  "data_mode": "END_OF_DAY",
  "market_regime": {
    "type": "BULLISH_TREND",
    "confidence": 0.57
  },
  "decision": {
    "market_action": "WATCH",
    "entry_state": "WAIT_DAILY_CONFIRM",
    "position_action": "CONDITIONAL_HOLD",
    "best_trade_zone": null,
    "reason_codes": [
      "DAILY_LOW_REJECTION",
      "DAILY_AVERAGE_PRICE_RECLAIM",
      "STRONG_CLOSE_LOCATION",
      "TACTICAL_SUPPORT_CANDIDATE",
      "ENTRY_NOT_CONFIRMED",
      "RR_INSUFFICIENT"
    ]
  },
  "zones": {
    "nearest_decision_zone": {
      "lower": 2390,
      "upper": 2411,
      "direction": "SUPPORT",
      "role": "TACTICAL",
      "lifecycle": "CANDIDATE",
      "source": "DAILY_CANDLE",
      "decision_relevance_score": 78,
      "structural_score": 40
    },
    "primary_structural_zone": {
      "lower": 2318.03,
      "upper": 2331.97,
      "direction": "SUPPORT",
      "role": "STRUCTURAL",
      "lifecycle": "VALIDATED",
      "source": "HISTORICAL_SR",
      "decision_relevance_score": 52,
      "structural_score": 69
    },
    "nearest_resistance": {
      "lower": 2472.56,
      "upper": 2487.44,
      "direction": "RESISTANCE",
      "role": "STRUCTURAL",
      "source": "HISTORICAL_SR"
    }
  },
  "daily_price_action": {
    "close_position": 0.75,
    "body_ratio": 0.25,
    "lower_wick_ratio": 0.5,
    "closed_above_average_price": true,
    "volume_ratio_20": 1.08
  },
  "risk": {
    "rr": 1.29,
    "minimum_rr": 1.8,
    "rr_qualified": false,
    "ev_percent": 3.25
  },
  "data_quality": {
    "overall_completeness": 0.72,
    "price_data_complete": true,
    "chip_coverage": 0.25,
    "missing_features": [
      "MARGIN",
      "BRANCH",
      "CONCENTRATION"
    ]
  }
}
```

---

# 十九、UI 修改建議

## 19.1 明確顯示資料模式

頁面頂部加入：

```text
資料模式：盤後日 K
不含即時盤中確認
```

避免使用者把系統輸出當成即時交易訊號。

## 19.2 摘要區

建議顯示：

```text
市場狀態
偏多趨勢

目前動作
觀察

下一交易日觀察區
2390～2411
日 K 候選支撐，尚待後續收盤確認

結構支撐
2318～2332
歷史強度較高

最近壓力
2473～2487

最佳交易區
暫無

資料完整度
72%
籌碼資料覆蓋率 25%
```

## 19.3 避免的文案

避免：

```text
盤中承接確認
即時突破成立
目前最具決策意義的主交易區
```

建議：

```text
最新日 K 顯示承接跡象
下一交易日需觀察是否守住
目前最重要的歷史結構支撐
目前沒有符合 RR 條件的交易區
```

---

# 二十、實作優先級

## P0：立即修改

### 1. Primary Zone 拆分

新增：

```text
Nearest Decision Zone
Primary Structural Zone
Best Trade Zone
```

### 2. RR Gate

```text
RR 未達最低門檻
→ Best Trade Zone = NONE
→ Entry State 不可為 ENTRY_READY
```

### 3. Missing 與 Neutral 分離

修正法人、融資、分點、集中度等資料狀態。

### 4. UI 標示 EOD 模式

加入：

```text
盤後日 K 分析
不含盤中確認
```

## P1：第二階段

### 5. Daily Price Action Engine

先實作：

* Close Position
* Body Ratio
* Lower Wick Ratio
* Upper Wick Ratio
* Daily Average Price Reclaim
* Volume Ratio
* Gap Percent

### 6. Daily Candidate Zone

建立：

```text
DAILY_SUPPORT_CANDIDATE
DAILY_RESISTANCE_CANDIDATE
```

### 7. Zone Role

加入：

```text
TACTICAL
STRUCTURAL
MACRO
```

### 8. Decision Relevance Score

正式納入：

* 距離
* 日 K 近期性
* Price Path
* Blocking Zone

## P2：第三階段

### 9. Zone Lifecycle

加入：

```text
CANDIDATE
CONFIRMED
VALIDATED
WEAKENING
BROKEN
INVALIDATED
```

### 10. Daily Confirmation 回測

驗證：

* 候選支撐隔日守住率
* 候選壓力隔日壓回率
* 兩日確認後的勝率
* 不同量能條件下的成功率

### 11. 機率與分數校準

重新校準：

* Structural Score
* Decision Relevance Score
* Tradability Score
* Minimum RR

---

# 二十一、回測規格

## 21.1 Daily Candidate Support

建立候選日為 T 日。

測試：

```text
T+1 是否回測候選區
T+1 是否收於區間上緣之上
T+3 是否未跌破區間下緣
T+5 最大反彈幅度
T+5 最大不利幅度
```

## 21.2 Daily Candidate Resistance

測試：

```text
T+1 是否接近候選壓力
T+1 是否收於區間下緣以下
T+3 是否未突破區間上緣
T+5 最大回落幅度
```

## 21.3 Ranking 比較

比較三種輸出：

```text
原始 Primary Zone
Nearest Decision Zone
Primary Structural Zone
```

衡量：

* 哪一個更早被價格測試
* 哪一個方向預測更有效
* 哪一個更適合停損
* 哪一個 RR 更合理
* 哪一個能降低假訊號

---

# 二十二、驗收條件

## 22.1 功能驗收

台積電案例必須能同時輸出：

```text
Nearest Decision Zone：
2390～2411

Primary Structural Zone：
2318～2332

Best Trade Zone：
NONE
```

不能再只輸出一個 Primary Zone。

## 22.2 RR 驗收

當：

```text
RR = 1.29
Minimum RR = 1.8
```

必須：

```text
rr_qualified = false
best_trade_zone = null
entry_state != ENTRY_READY
```

## 22.3 資料品質驗收

缺失籌碼資料不可顯示為：

```text
0，中性
```

必須顯示：

```text
MISSING
STALE
INVALID
```

其中一種狀態。

## 22.4 文案驗收

盤後模式不可使用：

```text
盤中確認
即時承接
即時突破
```

必須使用：

```text
日 K 候選
等待隔日確認
最新收盤結構
```

---

# 二十三、最終建議

在沒有盤中交易資訊來源的情況下，不需要停止 Decision Engine 的改進，也不需要等到分鐘 K 資料完成才開始修改。

目前最值得優先處理的是：

```text
Primary Zone 拆分
RR Gate
Missing Data Handling
Daily Price Action
Decision Relevance Ranking
```

暫時不處理：

```text
即時 VWAP
分鐘 K
Opening Range
盤中 Volume Profile
即時突破監控
盤中下單訊號
```

現階段系統應定位為：

> 盤後日 K 決策輔助系統，而不是盤中交易訊號系統。

其核心輸出應分別回答：

```text
下一交易日最先要觀察哪個區間？
哪個區間是主要歷史結構？
目前是否真的存在符合 RR 與確認條件的交易機會？
```

只要先完成這三層區分，即使只有日 K 與盤後資料，系統的決策一致性、可解釋性與實務價值仍會明顯提升。
