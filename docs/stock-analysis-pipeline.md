# Stock Analysis Pipeline Design

> Version: v1.0
>
> 目的：
> 將每日分析流程拆分成兩個主要 Stage：
>
> - POST_CLOSE_TECHNICAL（收盤技術分析）
> - POST_CLOSE_FULL（盤後完整分析）
>
> 並保留 Feature Builder 供 AI 模型訓練與推論使用。

---

# Pipeline

```text
收盤 (14:30)

        │
        ▼

POST_CLOSE_TECHNICAL
(15:00 左右)

        │
        ▼

POST_CLOSE_FULL
(晚上，籌碼更新)

        │
        ▼

MODEL_FEATURE_BUILD
(產生 AI Feature)

        │
        ▼

LightGBM
XGBoost
Transformer

        │
        ▼

Decision Engine

        │
        ▼

Telegram / API / Dashboard
```

---

# Stage 1：POST_CLOSE_TECHNICAL

> 執行時間：約 15:00
>
> 不依賴今日籌碼資料。

---

## Step 1：取得收盤資料

更新：

- Open
- High
- Low
- Close
- Volume
- Amount
- VWAP
- 漲跌
- 漲跌幅
- 振幅

檢查：

- 停牌
- 除權息
- 異常成交量
- 缺漏資料

---

## Step 2：更新 Daily K

更新資料表：

```
daily_kline
```

內容：

- OHLCV
- 成交金額
- 均價

---

## Step 3：重新計算 Technical Indicator

### Moving Average

- MA5
- MA10
- MA20
- MA60
- MA120
- MA240

### EMA

- EMA5
- EMA13
- EMA21
- EMA55

### Momentum

- RSI
- KD
- MACD
- ROC
- Momentum

### Trend

- ADX
- DI+
- DI-
- SuperTrend
- Trend Strength

### Volatility

- ATR
- Bollinger Band
- Keltner Channel

### Volume

- Relative Volume
- Volume MA
- VWAP
- OBV

---

## Step 4：重新計算支撐壓力

計算：

- Horizontal Support
- Horizontal Resistance
- ATR Zone
- Volume Profile
- Pivot Point
- Swing High
- Swing Low

輸出：

- Support1
- Support2
- Resistance1
- Resistance2
- Breakout Price
- Pullback Zone

---

## Step 5：Pattern Detection

K棒：

- Hammer
- Doji
- Shooting Star
- Bullish Engulfing
- Bearish Engulfing

型態：

- Double Top
- Double Bottom
- Triangle
- Flag
- Cup Handle

---

## Step 6：量價分析

分析：

- 價漲量增
- 價漲量縮
- 價跌量增
- 價跌量縮
- 放量突破
- 縮量整理
- 爆量長紅
- 爆量長黑

---

## Step 7：Trend Analysis

判斷：

- Bullish
- Bearish
- Sideway

輸出：

```
Trend Score
```

---

## Step 8：支撐分析

輸出：

- Support Probability
- Bounce Probability

---

## Step 9：突破分析

輸出：

- Breakout Probability
- Fake Breakout Probability

---

## Step 10：Risk / Reward

計算：

- Entry
- Stop Loss
- Target1
- Target2
- Risk
- Reward
- RR Ratio

---

## Step 11：Expected Value

計算：

- Win Rate
- Loss Rate
- Expected Return
- EV

---

## Step 12：Technical Confidence

整合：

- Trend
- Pattern
- Volume
- Support
- Resistance
- RR
- EV

輸出：

```
Technical Score
Technical Confidence
```

---

## Step 13：產生 Technical Report

輸出：

- Trend
- Technical Score
- Confidence
- Entry
- Stop
- Target
- RR
- EV
- Technical Recommendation

Status：

```
POST_CLOSE_TECHNICAL
```

---

# Stage 2：POST_CLOSE_FULL

> 執行時間：晚上
>
> 今日籌碼資料全部更新後。

---

## Step 1：下載籌碼

更新：

- 外資
- 投信
- 自營商
- 融資
- 融券
- 借券
- 主力
- 分點

---

## Step 2：更新資料庫

資料表：

```
chip_daily
```

---

## Step 3：重新計算 Chip Feature

### 外資

- Net Buy
- Consecutive Buy
- Consecutive Sell
- Buy Ratio

### 投信

- Net Buy
- Consecutive Buy

### 自營商

- Net Buy

### 融資

- Increase Rate
- Margin Ratio

### 融券

- Short Ratio

### 借券

- Borrow Ratio

### 主力

- Concentration
- Distribution

---

## Step 4：Chip Score

輸出：

- Foreign Score
- Investment Score
- Dealer Score
- Margin Score
- Major Score

整合：

```
Chip Score
```

---

## Step 5：Market Analysis

分析：

- 加權指數
- OTC
- 台指期
- SOX
- Nasdaq
- S&P500

輸出：

```
Market Score
```

---

## Step 6：Sector Analysis

分析：

- IC Design
- AI Server
- PCB
- Memory
- CoWoS
- Passive Components
- Networking
- Optics

輸出：

```
Sector Score
```

---

## Step 7：重新修正模型

重新計算：

- Breakout Probability
- Support Probability
- Trend Probability

---

## Step 8：重新修正 RR

依據：

- 籌碼
- 市場
- 族群

修正：

- Entry
- Stop
- Target

---

## Step 9：重新計算 EV

重新計算：

- Win Rate
- Loss Rate
- Expected Return
- EV

---

## Step 10：重新計算 Confidence

整合：

- Technical
- Chip
- Market
- Sector

輸出：

```
Final Confidence
```

---

## Step 11：Decision Engine

輸出：

- Strong Buy
- Buy on Pullback
- Watch
- Reduce
- Avoid

---

## Step 12：Revision Analysis

比較：

```
POST_CLOSE_TECHNICAL

↓

POST_CLOSE_FULL
```

紀錄：

- Score Change
- Confidence Change
- Recommendation Change

原因：

- 外資買超
- 外資賣超
- 主力集中
- 主力分散
- 融資增加
- 融券增加
- 市場轉弱
- 族群轉強

---

## Step 13：產生 Final Report

輸出：

- Technical Score
- Chip Score
- Market Score
- Sector Score
- Final Confidence
- Entry
- Stop
- Target
- RR
- EV
- Recommendation

Status：

```
POST_CLOSE_FULL
```

---

# Stage 3：MODEL_FEATURE_BUILD（Internal）

> 不直接提供給使用者。

---

## Feature Merge

整合：

### Technical

- MA
- EMA
- RSI
- MACD
- ATR
- ADX
- Trend
- Pattern

### Price

- OHLCV
- VWAP
- Relative Volume

### Support / Resistance

- Support Distance
- Resistance Distance
- ATR Zone
- Volume Profile

### Chip

- Foreign
- Investment
- Dealer
- Margin
- Lending
- Major

### Market

- Market Score
- Volatility
- Index Trend

### Sector

- Sector Score
- Relative Strength

---

## Feature Engineering

建立：

- Lag Feature
- Rolling Feature
- Window Feature
- Momentum Feature
- Volatility Feature
- Ratio Feature

預計：

```
150~300 Features
```

---

## Label Builder

建立：

### Classification

- Tomorrow Up
- Tomorrow Down
- 5-Day Up
- Breakout Success
- Support Hold
- RR >= 2
- RR >= 3

### Regression

- Future Return
- Max Gain
- Max Drawdown

---

## Model Input

輸出：

```
Feature Vector
```

提供：

- LightGBM
- XGBoost
- Transformer

---

# AI Prediction

模型：

- LightGBM
- XGBoost
- Transformer

輸出：

- Breakout Probability
- Support Probability
- Trend Probability
- Win Rate
- Expected Return
- Max Drawdown
- Confidence

---

# Decision Engine

融合：

- Rule Engine
- Technical Score
- Chip Score
- Market Score
- Sector Score
- AI Probability
- RR
- EV

最終輸出：

- Buy
- Buy on Pullback
- Hold
- Watch
- Reduce
- Sell

並提供：

- Entry
- Stop Loss
- Target
- Confidence
- Risk Level
- Decision Reason