# 2026/07/15 SR Zone 改善實作計畫書

## 文件用途

此文件整理 0050、00981A、2330 三組 SR Zone 測試後暴露的 Decision Engine 問題，作為後續實作計畫與驗收依據。重點不是重新調整 SR Detection 或 Daily Candidate Zone，而是改善「抓到正確區間後，Decision Engine 如何選區、解釋、仲裁 action」。

目前已由其他文件承接的項目不在此重複展開：

- `Chip missing != neutral` 已由 `decision_summary.data_quality.features` 現況承接，會區分 missing、neutral、negative、positive、stale、invalid。
- Daily Confirmation 的歷史驗證與成效統計由 `docs/todo.md` T-028 追蹤。
- 分數與權重校準由 `docs/todo.md` T-014 追蹤。

目前實作狀態：

- P0 已落地：confirmed reclaim 不再被同 zone 的舊 breakdown gate 強制拉成 `EXIT`，且 reclaim evidence 使用同一份 interaction 判斷。
- P1 已落地：新增 `final_entry_permission`、`RECOVERY` / `EARLY_TREND`、EOD 收盤收復 label、follow-through 拆層與 `BULLISH_CONTINUATION`。
- P2 已落地：新增 zone width penalty、nearest support/resistance、`rr_context` 與 completeness 拆層。

## Case Matrix

| Case | 現象 | 預期結果 | 疑似模組 | 優先度 | 驗收條件 |
|---|---|---|---|---|---|
| 0050 | `REVERSAL_CANDIDATE + NEXT_DAY_FOLLOW_THROUGH` 後仍受 `HIGH_VOLUME_BREAKDOWN` active risk gate 影響 | 轉為 `SUPPORT_RECLAIM_CONFIRMED`，解除不該延續的 breakdown 風險閘門 | Event Lifecycle / Reclaim Evidence | P0 | fixture 不再因已確認收復的舊 breakdown 直接降為 EXIT |
| 0050 | 7/15 fixture 的 Position Action 輸出過度防守 | 輸出 `HOLD` 或條件式持有，不得輸出 `EXIT` | Position Action arbitration | P0 | `position_action` 為 `HOLD` 或可明確解釋的條件式持有，並保留防守線 |
| 0050 | Global Entry State 與 Daily Entry State 容易產生多重語意 | 產生唯一 Final Entry Permission | Entry State arbitration | P1 | UI/API 只需讀一個 final permission 即可判斷是否可進場 |
| 00981A | 28.06-28.37 被 7/14 K 棒完整穿越後收回，卻仍顯示尚未測試 | recent validation 應反映已測試且收回 | Zone overlap / recent_validation | P0 | 該區不再標為未測試，並能輸出收復證據 |
| 00981A | `reclaim=None` 與 `SUPPORT_RECLAIM_CONFIRMED` 同時存在 | Reclaim evidence 單一路徑輸出，不得互相矛盾 | Reclaim Evidence pipeline | P0 | 同一份 summary 不會同時輸出空 reclaim 與 confirmed reclaim |
| 00981A | HH + HL + Follow Through 仍判為 RANGE | 增加 `RECOVERY`、`EARLY_TREND` 等 transition state | Market Regime Transition | P1 | recovery / early trend case 不再只落入 range |
| 00981A | Entry RR 與既有部位 RR 混在一起 | Entry RR 與 Position RR 分開 | RR / Position Context | P2 | 新進場與既有持股的 RR、停損、防守線能分別呈現 |
| 2330 | 6% 寬 historical zone 因上緣接近現價搶走 nearest zone | Decision relevance 加入 zone width penalty | Zone ranking / Decision relevance | P2 | 過寬區間不會只因邊界接近現價成為 nearest decision zone |
| 2330 | nearest decision zone 無法同時表達上下方下一決策點 | 拆成 nearest support / nearest resistance，再由 price path 選 next decision | Price Path / Zone selection | P2 | summary 可同時回答最近支撐、最近壓力與下一決策價 |
| 2330 | 「盤中收復」語意不適合 EOD daily 判讀 | 改為 `CLOSE_RECLAIMED_PREVIOUS_CLOSE` 或日 K 收復結構 | Daily Price Action / Event naming | P1 | EOD 模式不再使用易誤解的盤中收復語意 |
| 2330 | Follow Through 同時混合價格延續與動能確認 | 拆成 Price Follow Through 與 Momentum Confirmation | Daily Price Action | P1 | 可表達「價格延續、動能確認不足」 |
| 2330 | Market Bias 應為 `BULLISH_CONTINUATION`，卻偏向 reversal watch | 增加 bullish continuation 判讀 | Market Bias | P1 | 延續型多頭不再被標成反轉觀察 |
| 2330 | Data completeness 100% 被誤讀成交易資格完整 | 拆成 market data completeness、RR completeness、trade qualification completeness | Data Quality / Trade Qualification | P2 | 市場資料完整不等於 RR 或交易資格完整 |

## P0：Event Lifecycle 與 Position Action Safety

1. 建立 Event Lifecycle 仲裁規則：
   - `REVERSAL_CANDIDATE + NEXT_DAY_FOLLOW_THROUGH` 應升級為 `SUPPORT_RECLAIM_CONFIRMED`。
   - confirmed reclaim 後，舊的 `HIGH_VOLUME_BREAKDOWN` 不得繼續作為 active risk gate。
   - 同一 zone 同一時間不得同時輸出空 reclaim 與 confirmed reclaim。

2. 修正 Position Action arbitration：
   - reclaim confirmed 或 recovery confirmed 的 case 不得直接輸出 `EXIT`。
   - 0050 7/15 fixture 的目標輸出為 `HOLD` 或條件式持有。
   - 若輸出條件式持有，必須保留 `invalidation_price`、`recovery_price` 與 reason codes。

3. 修正 Zone overlap / recent validation：
   - K 棒完整穿越 zone 後收回時，該 zone 不應繼續顯示為「尚未測試」。
   - recent validation 與 reclaim evidence 必須使用同一份 interaction 判斷結果。

## P1：Entry Permission、Regime 與 Bias 語意

1. 統一 Entry State arbitration：
   - 將 Global Entry State 與 Daily Entry State 仲裁成唯一 Final Entry Permission。
   - Final Entry Permission 應成為 UI/API 判斷是否可進場的主要欄位。
   - legacy `entry_action_state` 與 `daily_entry_state` 可保留，但不得讓使用者自行拼湊結論。

2. 增加 Market Regime Transition：
   - 至少支援 `RECOVERY`、`EARLY_TREND`。
   - HH + HL + Follow Through 的 case 不應只落入 `RANGE_BOUND`。

3. 修正 EOD daily event naming：
   - EOD 模式避免使用「盤中收復」作為主要語意。
   - 改用 close-based 名稱，例如 `CLOSE_RECLAIMED_PREVIOUS_CLOSE` 或日 K 收復結構。

4. 拆分 Follow Through 與 Market Bias：
   - Price Follow Through 表達價格延續。
   - Momentum Confirmation 表達動能是否確認。
   - Market Bias 增加 `BULLISH_CONTINUATION`，避免延續型多頭被標成 reversal watch。

## P2：Zone Selection、RR 與 Completeness

1. Decision relevance 加入 zone width penalty：
   - 過寬 historical zone 不應只因上緣接近現價搶走 nearest decision zone。
   - penalty 應只影響當下決策相關性，不改寫 zone 的 structural quality。

2. 拆分 nearest decision zone：
   - 輸出 nearest support 與 nearest resistance。
   - `price_path.next_decision_price` 再依距離、方向、blocking zone 選出下一決策價。

3. 增加 position-aware RR：
   - Entry RR 用於新進場。
   - Position RR 用於既有持股的防守、減碼、續抱判斷。
   - UI/API 不應把兩者混成單一 RR。

4. 拆分 data completeness：
   - Market data completeness：OHLC、volume、chip 等資料是否完整。
   - RR completeness：是否具備進出場、停損、目標價所需資料。
   - Trade qualification completeness：是否滿足交易候選條件。

## 驗收與測試建議

1. 建立三個 case fixture：
   - 0050：7/15 reclaim / follow-through / position action。
   - 00981A：28.06-28.37 zone 穿越後收回。
   - 2330：寬 historical zone、bullish continuation、data completeness 拆層。

2. 每個 fixture 至少驗證：
   - `market_events` 與 `event_sequence` 不互相矛盾。
   - `market_regime.structure_state` 與 reclaim evidence 一致。
   - `market_bias`、`position_action`、Final Entry Permission 語意一致。
   - `nearest support`、`nearest resistance`、`next_decision_price` 能回答不同問題。
   - `data_quality` 完整度不被誤讀成交易資格完整。

3. 實作順序：
   - 先做 P0，避免錯誤 EXIT 或 reclaim/breakdown 矛盾。
   - 再做 P1，統一使用者看到的 entry/regime/bias 語意。
   - 最後做 P2，改善 zone ranking、RR 與 completeness 的可解釋性。
