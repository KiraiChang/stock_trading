# Signal 規格

## 訊號類型

| 類型 | 方向 | 說明 |
|------|------|------|
| BREAKOUT | BUY | 突破阻力 + 爆量 + 多頭 |
| BREAKDOWN | SELL | 跌破支撐 + 空頭 |
| VOLUME_SPIKE | WATCH | 異常爆量（無論方向） |
| SUPPORT_BOUNCE | WATCH | 價格貼近支撐後反彈（只在上面三種都沒觸發時才檢查） |

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
1. BreakCandle.PreviousClose <= Resistance.Price AND BreakCandle.Close > Resistance.Price
2. BreakCandle.VolRatio >= 2.0
3. BreakCandle 後連續 2 根 K 棒 Close > Resistance.Price（站穩壓力上方）
4. RSI14 < 80（RSI 不足或為 0 時不阻塞）
5. Trend == BULLISH
```

突破當根不立即發訊，第一根確認 K 棒也不發訊；第二根確認 K 棒仍站穩時才輸出
BREAKOUT。量能門檻使用突破當根的量比，而非確認當根。若同一個突破事件跨越
多個阻力，回報最接近確認 K 棒收盤的已跨越阻力（最高的 crossed resistance）；
同價位以 Strength 較高者優先。

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

## Support Bounce 條件

```
1. 最新一根 K 棒的 Low 落在某個支撐價位 ±1% 內（觸及支撐）
2. Close > 該支撐價位（收回支撐之上）
3. (Close - Low) / (High - Low) >= 0.5（收在當根振幅的上半部；High == Low 時不觸發）
```

只在 `CheckBreakout`（BREAKOUT、BREAKDOWN、VOLUME_SPIKE）**沒有觸發任何訊號**時才檢查；支撐依序檢查，
第一個符合的就輸出。⛔ 不要求量能、⛔ 不要求趨勢——這是比 BREAKOUT／BREAKDOWN 更早期、方向性較弱的觀察訊號。
盤中用 1 分 K 評估時，支撐每一輪由最近 100 根 K 棒重算、會小幅漂移（I-140 的取證見 `issue.md`）。

---

## 訊號去重

`Engine.Evaluate` 在寫入 DB、推送 Redis queue 與 WebSocket broadcast 前會檢查最近訊號。有兩條規則：

**A（價位鍵，現行；所有類型與日 K 都用它）**：同一 `symbol + signal_type + direction + relevant level` 在 15 分鐘內不重發：

- BREAKOUT 以 `Resistance` 作為 relevant level。
- BREAKDOWN / SUPPORT_BOUNCE 以 `Support` 作為 relevant level。
- VOLUME_SPIKE 不比對價位。
- 時間：DB 判重比的是兩筆的 K 棒時間 `ts`；保留鎖（本機、Redis）從保留當下起算 15 分鐘。
- ⚠️ 已知限制：價位量化到 1e-6，但 `signals` 只存兩位小數——DB 判重拿讀回的兩位小數跟六位小數的候選比，
  支撐沒漂移也比不到，只剩保留鎖擋得住完全相同的價位（I-140）。

**B（盤中 1 分 K 的 SUPPORT_BOUNCE，I-140）**：同一檔上一筆**已寫進 `signals`**、`timeframe = '1m'` 的 SUPPORT_BOUNCE
**送出時間**起算 15 分鐘內，⛔ 不再送出新的 SUPPORT_BOUNCE，**⛔ 不比對支撐價位**——15 分鐘內碰到不同支撐也只提醒一次（明示接受的取捨）。

- 只有「SUPPORT_BOUNCE ＋ 評估用 `1m`」走 B，⛔ 不看呼叫來源（手動評估帶 `timeframe=1m` 也一樣）；其他一律 A。
- **時間**：候選在 DB 判重**之前**取樣一次 `now`；送出時間 `sentAt = now` 截到整秒、寫進 `signals.created_at`；
  到期時刻 `expiresAt = sentAt + 15 分鐘`。DB、本機、Redis 三條路徑都只比 `now ≥ expiresAt`（未到就抑制；時鐘倒退也抑制）。
- **錨點**：DB 判重找同一檔 `timeframe = '1m'`、`created_at` 最新的 SUPPORT_BOUNCE（同一秒取 `id` 大的；timeframe 是 NULL 的舊列⛔ 不算）。
  ⛔ 不能只依 `id`（寫入順序）：`created_at` 在 DB 判重前就取樣，較晚寫入的列可能帶較早的送出時間，依 `id` 會讓它蓋掉仍有效的冷卻。
  保留鎖是第二層，涵蓋「DB 寫失敗但通道送出」的那一筆。
- **被抑制、被扣住（`Withhold`）的候選⛔ 不寫 `signals`、⛔ 不保留**，所以⛔ 不延長冷卻：09:01 送出、09:11 被抑制、09:16 可以送出。
- 細節（送出證據、Redis 的保留方式、重啟與 Redis 狀態）見 `architecture.md`「寫入失敗的一致性契約」的「盤中 1 分 K 的 SUPPORT_BOUNCE（B 規則）」。

⚠️ **production 目前用 A**：B 已實作，live 要不要改用 B，等 I-140 的 A／B 重播結果裁決（`docs/plans/i140-intraday-bounce-dedup.md` 第七節）。

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
