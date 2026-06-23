# Signal 規格

## 訊號類型

| 類型 | 方向 | 說明 |
|------|------|------|
| BREAKOUT | BUY | 突破阻力 + 爆量 + 多頭 |
| BREAKDOWN | SELL | 跌破支撐 + 空頭 |
| VOLUME_SPIKE | WATCH | 異常爆量（無論方向） |

---

## Trend Detection（趨勢識別）

使用 Swing High/Low 識別市場結構：

```
Swing High: High[i] > High[i-1] AND High[i] > High[i+1]
Swing Low:  Low[i]  < Low[i-1]  AND Low[i]  < Low[i+1]

BULLISH:  最近 SwingHigh2 > SwingHigh1 AND SwingLow2 > SwingLow1  (HH + HL)
BEARISH:  最近 SwingHigh2 < SwingHigh1 AND SwingLow2 < SwingLow1  (LH + LL)
SIDEWAYS: 其他
```

---

## Support/Resistance 識別

1. 從最近 60 根 K 棒找出 Local Max（阻力候選）和 Local Min（支撐候選）
2. 將差距 < 1% 的候選點合併為同一價位
3. 以觸碰次數計算 Strength（0~1）
4. 取 Strength 最高的前 3 個

---

## Breakout 條件（全部滿足）

```
1. Close > Resistance.Price
2. VolRatio >= 2.0
3. Trend == BULLISH
```

---

## Breakdown 條件

```
1. Close < Support.Price
2. Trend == BEARISH
```

---

## Volume Spike 條件（獨立觸發）

```
VolRatio >= 3.0
```

---

## 假突破過濾（Phase 2 規劃）

Phase 1 無假突破過濾。Phase 2 計畫加入：
- 需收盤確認（非盤中突破即觸發）
- 連續 2 根 K 棒維持在阻力以上
- RSI 不超買（< 80）
