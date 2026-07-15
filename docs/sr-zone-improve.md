# 2026/07/15 SR Zone 改善實作計畫書 進階版

## 需要解決問題
* **修 Zone Validation Date Window / Overlap。**先確認為什麼 7/14 完整穿越 28.06～28.37，結果仍是 UNTESTED / PENDING_VALIDATION。
* **統一 PriceActionEvidence。**禁止 Reclaim=None 與 SUPPORT_RECLAIM_CONFIRMED 同時存在。
* **Chip missingness。**拆 score / availability / coverage / confidence / effective_impact。
* **增加 Regime Transition。**不要繼續塞進 Market Bias。
* Blocking Zone 增加 source metadata。

## 處理方向
### recent validation 有日期窗口問題
結果仍顯示：
```
Primary Zone 28.06～28.37
尚未測試
PENDING_VALIDATION
Touch 6 次
```
這在資料上不成立。

7/14：
```
Bar:
27.34 ───────── 29.30

Zone:
28.06 ─ 28.37
```
K 棒完整穿越 zone。

區間 overlap：
```
bar.high >= zone.low
29.30 >= 28.06  TRUE

bar.low <= zone.high
27.34 <= 28.37  TRUE
```
所以：
```
INTERSECTS = TRUE
```
而且：
```
bar.low < zone.low
close > zone.high
```
應該分類為：
```
UNDERCUT_RECLAIM
```
不是：
```
UNTESTED
```
#### 處理方向
做sr-zone分析時log以下內容，
```
   analysis_date
   validation_start_date
   validation_end_date
   latest_validation_bar.date
```
並且reivew程式碼正確應該要是這樣的
```
zone_generation_end_date
<
validation_bar_date
<=
analysis_date
```

### 只修改了 Decision Summary / Market Bias mapping，沒有修底層 PriceActionEvidence
幫忙review資料流應該如下 
```
OHLCV
  ↓
ZoneInteractionDetector
  ↓
PriceActionEvidence
  ├─ reclaim_type
  ├─ rejection_type
  ├─ penetration_ratio
  ├─ close_relative_to_zone
  └─ follow_through
        ↓
Decision Engine
        ↓
Summary
```
### Market Regime 仍然偏慢
應該新增一個維度：
```
Market Regime:
RANGE

Regime Transition:
RANGE_TO_UPTREND

或者
Regime:
RECOVERY
```
結構正在往哪裡移動。的狀況

### Chip Aggregator 也要改

正確算法應該：
```
available_weight =
0.35

chip_score =
(-55 × 0.35) / 0.35

= -55
```
但：
```
chip_confidence = 35%
```
輸出：
```
Chip Score:
-55

Chip Confidence:
LOW

Coverage:
35%
```
Decision Engine 再決定：
```
effective_chip_score =
chip_score × confidence

= -55 × 0.35
= -19.25
```
在來說明部分

目前：

```
籌碼中性 -19
```

應該：
```
籌碼偏空，但資料覆蓋率低，effective impact -19
```

### 還有一個新發現：你的壓力 zone 有重疊問題
系統：
```
Blocking Zone:
29.52 ~ 29.70

Nearest Decision Zone:
30.24 ~ 30.55

Short Resistance:
30.61 ~ 30.79

Medium Resistance:
30.24 ~ 30.55
```
我想問：
```
29.52～29.70 是從哪個 detector 出來的？
```
因為 UI 的短／中／長 zone 沒有它。

這代表 Blocking Zone 可能來自：
```
all zones
```
而 Summary Zone 來自：
```
selected zones
```
這本身沒錯。

但使用者會看到：
```
Blocking Zone 29.52～29.70
```
往下找：

找不到這個 zone。

我建議 Blocking Zone 顯示 source：
```
29.52 ~ 29.70
Source: VOLUME_PROFILE
Timeframe: SHORT
Confidence: 58%
```
否則 Decision Engine 的可解釋性會斷掉。