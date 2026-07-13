1. 將 SUPPORT_RECLAIM 拆成：
   - SUPPORT_RECLAIM_CANDIDATE
   - SUPPORT_RECLAIM_CONFIRMED

   單日 low 進入 zone、current price 回到 zone 上方，
   只能標記 CANDIDATE。

   必須 close > zone.high，且下一根 K 未重新跌破，
   才可標記 CONFIRMED。

2. Position Action 必須附帶條件：
   - invalidation price
   - recovery price
   - reason codes

   UI 不可只顯示「持有」，
   改為「條件式持有」，並顯示防守線與回穩線。

3. 將 Confidence 改名為 Zone Confidence／區間辨識信心，
   不得讓使用者誤解為反彈勝率。
   Bounce Probability 與 Breakdown Probability 必須分開顯示。

4. 持股分析的調整
   1.   顯示 Risk Budget → Per Share Risk → Max Shares → Excess Shares 的計算鏈。
   2.   Stop 拆成 Defense Price 與 Structural Stop。
   3.   RR 拆成 Market RR 與 Position RR。
   4.   顯示執行減碼後的 realized / unrealized P&L impact。