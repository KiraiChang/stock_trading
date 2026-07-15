# 2026/07/15 SR Zone 改善實作計畫書

## 0050 測試後須改善的內容
* P0：修 Event Lifecycle。 REVERSAL_CANDIDATE + NEXT_DAY_FOLLOW_THROUGH → SUPPORT_RECLAIM_CONFIRMED，並解除 HIGH_VOLUME_BREAKDOWN active risk gate。
* P0：重寫 Position Action arbitration。 7/15 這個 fixture 必須輸出 HOLD 或 CONDITIONAL_HOLD，禁止 EXIT。
* P1：統一 Entry State arbitration。 Global Entry State、Daily Entry State 最後必須產生唯一 Final Entry Permission。

## 00981A 測試後須改善的內容
* **先修 Zone overlap / recent_validation。**28.06～28.37 被 7/14 K 棒完整穿越後收回，不應是「尚未測試」。
* 統一 Reclaim Evidence pipeline。Reclaim=None 與 SUPPORT_RECLAIM_CONFIRMED 不能同時存在。
* **增加 Market Regime Transition。**至少加入 RECOVERY、EARLY_TREND，避免 HH+HL+Follow Through 還判 RANGE。
* **Chip missing != neutral。**0 分與沒有資料必須拆開。
* 增加 Position-aware RR。EntryRR 與 PositionRR 分開。

## 2330 測試後須改善的內容
* Decision Relevance 加入 Zone Width Penalty，避免 6% 寬的 Historical Zone 因上緣接近現價搶走 Nearest Zone。
* Nearest Decision Zone 拆成 Nearest Support / Nearest Resistance，再由距離與 Price Path 選 Next Decision。
* 移除「盤中收復」，改成 CLOSE_RECLAIMED_PREVIOUS_CLOSE / 日 K 收復結構。
* Follow Through 拆成 Price Follow Through 與 Momentum Confirmation；今天應是「價格延續、動能確認不足」。
* 修正 Market Bias；今天應是 BULLISH_CONTINUATION，不是 REVERSAL_WATCH。
* Data Completeness 拆層；100% 市場資料完整不代表 RR / Trade Qualification 完整。
```
目前最不需要碰的是 SR Detection 與 Daily Candidate Zone。現在瓶頸已經從「區間抓不準」移到「抓到正確區間後，Decision Engine 不知道該怎麼選、怎麼描述」。
```