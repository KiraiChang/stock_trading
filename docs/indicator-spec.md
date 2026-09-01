# 技術指標規格

所有指標皆從 `candles` 表讀取最近 120 根 K 棒計算（支援 SQLite / MySQL / PostgreSQL）。

---

## MA（移動平均）

```
MA(N) = Σ(Close[i], i = len-N to len-1) / N
```

計算週期：MA5, MA10, MA20, MA60。

**邊界條件：** K 棒數 < N 時回傳 0。

---

## RSI（相對強弱指數）

參數：period = 14，使用 Wilder smoothing。

```
初始：
  avgGain = mean(gains, 1..14)
  avgLoss = mean(losses, 1..14)

後續 (Wilder)：
  avgGain = (avgGain * 13 + currGain) / 14
  avgLoss = (avgLoss * 13 + currLoss) / 14

RS = avgGain / avgLoss
RSI = 100 - 100 / (1 + RS)
```

**邊界條件：**
- avgLoss = 0 **且 avgGain > 0**（只漲不跌）→ RSI = 100
- avgLoss = 0 **且 avgGain = 0**（整個視窗完全沒有波動）→ **RSI = 50（中性）**
- K 棒數 < 15 → 回傳 0

⚠️ **中間那條是 2026-09-01 才補的**（原記於 `issue.md` I-101）。
`diff == 0` 走的是「跌」那個分支（`lossSum -= 0`），所以**「整段平盤」與「只漲不跌」
會走到同一個 `avgLoss == 0`**。舊實作對兩者都回 100，於是一檔完全沒有成交波動的股票
會被判成極度超買——`isRSIOverbought` 會據此擋掉 BUY，而且 100 塞不進當時的
`rsi14 DECIMAL(6,4)`，整支標的的指標寫不進 DB。

**RSI = 100 這條路徑本身不是 bug，不要一起改掉**：真的只漲不跌時 100 是正確輸出。
所以型別也必須容得下它（`rsi14` 已放寬為 `DECIMAL(7,4)`）——
**語意修正與型別放寬兩件事都要做，缺一不可**。

---

## MACD

參數：fast=12, slow=26, signal=9。

```
MACD Line = EMA(12) - EMA(26)
Signal Line = EMA(9) of MACD Line
Histogram = MACD Line - Signal Line

EMA(N) multiplier = 2 / (N + 1)
EMA[i] = Close[i] * multiplier + EMA[i-1] * (1 - multiplier)
```

**邊界條件：** 需要至少 34 根 K 棒（slow + signal - 1）。

---

## Bollinger Bands

參數：period=20, multiplier=2.0。

```
Middle = MA(20)
StdDev = sqrt(Σ((Close - Middle)²) / 20)  # 母體標準差
Upper = Middle + 2 * StdDev
Lower = Middle - 2 * StdDev
```

---

## ATR（平均真實波幅）

參數：period=14，使用 Wilder smoothing。

```
TrueRange = max(High - Low, |High - PrevClose|, |Low - PrevClose|)

初始 ATR = mean(TR, 1..14)
後續：ATR = (ATR * 13 + TR) / 14
```

---

## VWAP（成交量加權均價）

```
TypicalPrice = (High + Low + Close) / 3
VWAP = Σ(TypicalPrice * Volume) / Σ(Volume)
```

日K 的 VWAP 代表近期平均持股成本。

---

## Volume Spike（爆量偵測）

```
VolMA20 = mean(Volume[-20 to -1])   # 不含當日
VolRatio = CurrentVolume / VolMA20
IsSpike = VolRatio >= 2.0
```

VolRatio >= 3.0 觸發 VOLUME_SPIKE 警示訊號。
