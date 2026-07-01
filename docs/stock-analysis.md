# 個股分析（Stock Analysis）

針對單一股票，用歷史 OHLCV 產生一份「現況分析報告」：支撐、壓力、進場點、
停損點、停利點，供人工判斷用——不是自動下單訊號，也不是回測的逐筆交易列表。

跟 [backtest-modular-strategy.md](./backtest-modular-strategy.md) 共用同一套
S/R／進場／停損元件（`python/backtest/modular/`），差別在於：backtest 是模擬
一段歷史區間內「重複進出場」的完整過程並輸出 PnL；stock-analysis 是只看
「現在」（資料最後一根K棒）算一次快照，並且多了「停利」與「驗證」的概念。

實作位置：
- 計算：`python/backtest/modular/analysis.py`（`analyze_symbol()`）
- 持久化 + 驗證：`backend/internal/analysis/`（`client.go` + `verifier.go`）
- API：`POST /api/v1/analysis`、`GET /api/v1/analysis`、`GET /api/v1/analysis/:id`、`POST /api/v1/analysis/:id/verify`（見 api-reference.md）
- 資料表：`stock_analyses`、`stock_analysis_levels`（見 database-schema.md）
- 前端：「個股分析」頁面（`/analysis`）

---

## 支撐 / 壓力：三種方法合併

跟單一策略只挑一種 S/R 方法不同，個股分析要看的是「全貌」，所以把
`SwingHighLowSR`、`ATRChannelSR`、`VolumeProfileSR`（詳細數學定義見
backtest-modular-strategy.md）三者的結果**直接合併**，各自保留 `method`
欄位標註來源，依 `strength` 由高到低排序。不做去重/合併相近價位的處理——
如果三種方法在相近價位都各自算出一個 level，代表這個價位被多種角度驗證，
本身就是一個有意義的訊號，值得讓使用者自己看到。

---

## 進場點：ACTIVE 或 WATCHING

先檢查「現在」是否已經真正觸發進場條件（`BreakoutEntry` 或
`PullbackSupportEntry`，數學條件見 backtest-modular-strategy.md）：

- **有觸發** → `entry_status = ACTIVE`，`entry_price` 是實際觸發價（當根收盤）。
- **沒觸發** → `entry_status = WATCHING`，回報一個「觀察中」的觸發價位，
  而不是單純回答「沒訊號」：

  ```
  趨勢 BULLISH：
      候選 1 = 最近的上方壓力位（若漲破則視為突破進場）
      候選 2 = 最近的下方支撐位（若拉回測試且守住則視為回測進場）
      取離現價「距離較近」的候選當主要觀察目標

  趨勢 BEARISH：
      候選 = 最近的下方支撐位（若跌破則視為放空進場）

  趨勢 SIDEWAYS：
      無明確方向建議（entry_direction = NONE），停損/停利留空
  ```

「取距離較近的」是因為那通常代表「比較可能先發生」的情境，但這只是輔助
判斷的近似法，不是嚴謹的機率估計。

---

## 停損：三種都算

沿用 backtest-modular-strategy.md 的 `ATRStopLoss` / `StructuralStopLoss` /
`CompositeStopLoss`，用（`ACTIVE` 的實際進場價，或 `WATCHING` 的觀察觸發價）
當作假設的進場點各自算一次，三個數字並列讓使用者自己比較，不預設哪個
「更對」。`entry_direction = NONE` 時三者皆為 `null`。

---

## 停利：三種方法

停損選定後（用 `stop_loss_composite` 當風險距離的基準，因為它是三者裡最
接近「實際會用哪個出場」的估計），算三種停利目標：

```
risk = |entry_price - stop_loss_composite|

1. next_level（下一關卡）：
   LONG  → 進場價上方最近的壓力位
   SHORT → 進場價下方最近的支撐位
   （沒有更遠的 level 時為 null）

2. risk_reward（風險報酬比，預設 2R）：
   LONG  → entry_price + 2 * risk
   SHORT → entry_price - 2 * risk

3. atr_multiple（ATR 倍數，預設 3 倍）：
   LONG  → entry_price + 3 * ATR(14)
   SHORT → entry_price - 3 * ATR(14)
```

三者哲學不同：`next_level` 尊重市場既有結構（可能很近也可能很遠）；
`risk_reward` 保證风险報酬比至少 2:1；`atr_multiple` 用波動度給一個跟
`ATRStopLoss` 對稱的固定目標。並列呈現，不自動選一個當「正確答案」。

---

## 驗證：可重複執行、非一次性判定

`POST /analysis/:id/verify` 每次都用「目前為止最新的 candles」重新計算，
不是把某個時間點的結果蓋棺定論——這樣使用者可以隨時重新檢查，結果永遠
反映當下的真實狀況。驗證只看**嚴格晚於** `analyzed_at` 的K棒，避免用產生
分析當下就已經知道的資料「驗證」自己。

**支撐/壓力**（每個 level 各自判定，跟是否真的進場無關）：
```
支撐位 BROKEN：任一根收盤 < 支撐價
壓力位 BROKEN：任一根收盤 > 支撐價
沒被突破 → HELD_SO_FAR（不是最終判定，只是「目前為止」）
```
用收盤價而非影線，理由跟 Go signal engine 的 breakout/breakdown 判斷一致：
避免盤中雜訊造成誤判。

**停損/停利觸及**（只有 `entry_status = ACTIVE` 才檢查，`WATCHING` 因為沒有
真正進場，記為 `{"applicable": false}`）：
```
停損觸及（LONG）：任一根 Low  <= 停損價
停損觸及（SHORT）：任一根 High >= 停損價
停利觸及（LONG）：任一根 High >= 停利價
停利觸及（SHORT）：任一根 Low  <= 停利價
```
三種停損、三種停利**分開獨立檢查**，不互相配對成「這組停損配這組停利」，
由使用者自己比較各自的觸及時間先後（`hit_at`）來判斷哪一組會先出場。

---

## 已知限制

- 「觀察中」的進場判斷（`_watching_entry`）是規則式的近似法，不是機率模型；
  純粹是「讓使用者知道要盯哪個價位」，不代表該價位一定會被觸及。
- 停利的 `next_level` 若支撐/壓力清單在那個方向上沒有更遠的 level，會是
  `null`，不會硬湊一個數字。
- 驗證不會判斷「先觸及停損還是停利」，只回報各自獨立的觸及時間，需要使用
  者自己比較 `hit_at`。
- 分析需要該股票在 `candles` 表已有至少 35 根K棒（`timeframe=1d` 時建議先
  用「歷史資料回補」頁面 backfill 至少 120 天）。
