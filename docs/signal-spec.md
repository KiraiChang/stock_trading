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
            或連續等高平台高於左右兩側高點
Swing Low:  Low[i]  < Low[i-1]  AND Low[i]  < Low[i+1]
            或連續等低平台低於左右兩側低點

BULLISH:  最近 SwingHigh2 > SwingHigh1 AND SwingLow2 > SwingLow1  (HH + HL)
BEARISH:  最近 SwingHigh2 < SwingHigh1 AND SwingLow2 < SwingLow1  (LH + LL)
SIDEWAYS: 其他
```

---

## Support/Resistance 識別

1. 從最近 60 根 K 棒找出 Local Max（阻力候選）和 Local Min（支撐候選），包含等高/等低平台
2. 將差距 < 1% 的候選點合併為同一價位
3. 以觸碰次數計算 Strength（0~1）
4. 取 Strength 最高的前 3 個

---

## Breakout 條件（全部滿足）

```
1. PreviousClose <= Resistance.Price AND Close > Resistance.Price
2. VolRatio >= 2.0
3. Trend == BULLISH
```

同一根 K 棒若跨越多個阻力，回報最接近當根收盤的已跨越阻力（最高的 crossed
resistance）；同價位以 Strength 較高者優先。

---

## Breakdown 條件

```
1. BreakCandle.PreviousClose >= Support.Price AND BreakCandle.Close < Support.Price
2. BreakCandle 後連續 2 根 K 棒 Close < Support.Price（未收回支撐）
3. Trend == BEARISH
```

跌破當根不立即發訊，第一根確認 K 棒也不發訊；第二根確認 K 棒仍未收回時才輸出
BREAKDOWN。跌破不要求量能門檻。若同一個跌破事件跌破多個支撐，回報最接近確認
K 棒收盤的已跌破支撐（最低的 crossed support）；同價位以 Strength 較高者優先。

---

## Volume Spike 條件（獨立觸發）

```
VolRatio >= 3.0
```

`VOLUME_SPIKE` 使用當根 K 棒時間作為訊號時間戳，與 BREAKOUT/BREAKDOWN 一致。

---

## 訊號去重

`Engine.Evaluate` 在寫入 DB、推送 Redis queue 與 WebSocket broadcast 前會檢查最近訊號。
同一 `symbol + signal_type + direction + relevant level` 在 15 分鐘內不重發：

- BREAKOUT 以 `Resistance` 作為 relevant level。
- BREAKDOWN / SUPPORT_BOUNCE 以 `Support` 作為 relevant level。
- VOLUME_SPIKE 不比對價位。

---

## 假突破過濾（Phase 2 規劃）

Phase 1 無假突破過濾。Phase 2 計畫加入：
- 需收盤確認（非盤中突破即觸發）
- 連續 2 根 K 棒維持在阻力以上
- RSI 不超買（< 80）

---

## 測試

`backend/internal/signal/*_test.go` 涵蓋以上所有規則：`trend_test.go`（趨勢
判斷各種結構組合）、`support_resistance_test.go`（群集/強度/排序截斷）、
`breakout_test.go`（突破/跌破/爆量的每個條件與優先順序）、`engine_test.go`
（`Engine.Evaluate` 接真實 sqlite 的整合測試）。跑法見
[development-guide.md](./development-guide.md#執行-go-測試)。
