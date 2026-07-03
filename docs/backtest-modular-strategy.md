# 模組化回測策略框架

`python/backtest/modular/` 底下的獨立套件，跟既有的 backtrader 引擎
（`python/backtest/strategy/breakout_v1.py`）並存。純 pandas/numpy 實作，
不依賴 backtrader，三個核心元件（支撐壓力／進場／停損）各自是獨立介面，
可任意替換組合（Strategy Pattern）。透過 `python/backtest/engine.py` 的
`run_backtest()` 統一入口，對 Go/DB/worker 完全透明——見
[python-go-integration-specification.md](./python-go-integration-specification.md)。

> 這裡的支撐/壓力（`support_resistance/`）算出的是**單一價位**（`Level`），
> 供回測與 [stock-analysis.md](./stock-analysis.md) 共用。機率導向、輸出
> **價格區間**的 [sr-zone-scoring.md](./sr-zone-scoring.md) 是另一套獨立實作
> （`sr_scoring/`），刻意不重用這裡的 `Level`/`SRLevels` 型別，只共用底層的
> `calc_atr`/`find_swing_highs`/`find_swing_lows`。

---

## 架構

```
backtest/modular/
  types.py                    Level / SRLevels / EntrySignal / Position / Trade / BacktestReport
  trend.py                    共用 swing high/low + HH/HL 趨勢判斷
  support_resistance/         SupportResistanceStrategy 介面 + 3 種實作
  entries/                    EntryStrategy 介面 + 2 種實作
  exits/                      StopLossStrategy 介面 + 3 種實作
  strategy.py                 TradingStrategy 組合根 + STRATEGY_PRESETS
  backtester.py                BacktestEngine：OHLCV DataFrame → BacktestReport
  service.py                   串接 engine.py 的入口（DB 讀取 + 型別轉換）
  tests/                        pytest，涵蓋每個元件與完整回測情境
```

---

## 支撐壓力（SupportResistanceStrategy）

### SwingHighLowSR

局部極值 + 聚合，跟 Go `internal/signal/support_resistance.go` 邏輯等價：

```
Swing High: High[i] > High[i-pivot_window .. i-1] 且 > High[i+1 .. i+pivot_window]
Swing Low:  對稱，用 Low 比較

合併：候選點間價差 < merge_pct（預設 1%）視為同一 level，取平均價
Strength = 該 level 觸碰次數 / 所有 level 中最大觸碰次數
取 Strength 最高的前 max_levels 個（預設 3）
```

參數預設：`lookback=60, pivot_window=1, merge_pct=0.01, max_levels=3`。

### ATRChannelSR

用近期最高/最低價當通道邊界，改用 ATR（而非固定百分比）衡量觸碰活躍度：

```
Resistance = max(High, 最近 lookback 根)
Support    = min(Low,  最近 lookback 根)
Band       = atr_multiplier * ATR(atr_period)
Strength   = |Close - Level| <= Band 的根數 / lookback 根數
```

參數預設：`lookback=20, atr_period=14, atr_multiplier=0.5`。只回傳各一個
support/resistance（通道邊界），不是多個離散價位。

### VolumeProfileSR

成交量分布找 Point of Control 與 Value Area：

```
1. 將 [min(Low), max(High)] 均分成 num_bins 個區間（預設 24）
2. 每根K棒的成交量歸入 typical price=(H+L+C)/3 所在的 bin
3. POC = 成交量最大的 bin 中心價，Strength=1.0
4. Value Area：從 POC 往左右擴張，每次併入量能較大的相鄰 bin，
   直到累積量能達 value_area_pct（預設 70%），其上緣 VAH／下緣 VAL 為次要 level
5. Level 價格 < 現價 → Support；> 現價 → Resistance
```

同一個 volume node 在價格穿越後支撐/壓力角色互換，是量價分析的標準用法。

---

## 進場訊號（EntryStrategy）

### BreakoutEntry（突破，雙向）

```
LONG:  Close[t] > Resistance.Price
       AND Volume[t] / MA(Volume, vol_period)[t] >= vol_multiplier（預設 2.0）
       AND Trend(t) == BULLISH

SHORT: Close[t] < Support.Price
       AND Trend(t) == BEARISH
       （跌破不要求爆量：恐慌性下跌常伴隨量縮而非量增）
```

與 Go `internal/signal/breakout.go` 的 `CheckBreakout` 對齊。

### PullbackSupportEntry（回測支撐 / retest）

多頭結構中價格拉回測試支撐後止穩的進場訊號，三條件需同時成立：

```
1. Trend(t) == BULLISH
2. |Low[t] - Support.Price| / Support.Price <= tolerance_pct（預設 0.5%）  — 當根曾觸及/貼近支撐
3. Close[t] > Support.Price AND Close[t] > Open[t]                         — 收盤收回支撐上方且是陽線
```

與突破策略互補：突破策略在「創新高」時進場，回測策略在「拉回守住既有支撐
（可能是先前的壓力位、swing low、或 volume node）」時進場。

---

## 停損（StopLossStrategy）

### ATRStopLoss

```
LONG:  stop = entry_price - atr_multiplier * ATR(atr_period)_at_entry
SHORT: stop = entry_price + atr_multiplier * ATR(atr_period)_at_entry
```

進場後固定不變（`update()` 原樣回傳），代表這筆交易願意承擔的最大波動風險。
預設 `atr_period=14, atr_multiplier=2.0`。

### StructuralStopLoss

追蹤最近一個「已確認」的 swing low/high，只收緊不放寬：

```
LONG:  初始 stop = 進場前最近一個 confirmed swing low
       之後每根K棒：若出現「更高」的新 swing low，stop 上移至該價位
       （對應 CLAUDE.md「Structure Broken（HL失效）」：只要不斷創出更高的
       低點，多頭結構還在；收盤跌破 stop 代表 HL 失守，出場）
SHORT: 對稱邏輯，改用 swing high，只下移不上移
```

找不到 swing 點時退回用 lookback 期間的最低/最高價當安全網。預設
`pivot_window=1, lookback=60`。

### CompositeStopLoss

同時用 ATR 停損與結構停損，取較保守（較貼近價格）者：

```
LONG:  effective_stop = max(atr_stop, structural_stop)
SHORT: effective_stop = min(atr_stop, structural_stop)
```

---

## 回測規則（BacktestEngine）

輸入：OHLCV DataFrame（index 為時間、升冪排列，欄位 `open/high/low/close/volume`）。
輸出：`BacktestReport`（總覽指標 + 逐筆 `Trade`）。

1. **避免 lookahead bias**：訊號在 bar t 收盤後產生，成交在 bar t+1 開盤價。
2. **停損成交**：bar 的高/低價觸及停損視為觸發，用停損價成交；若開盤已跳空
   穿越停損，改用開盤價（避免不切實際的成交價）。
3. **進場當根不做停損檢查**，下一根才開始管理部位——避免「同根K棒內先進場
   又立刻出場」的模糊情況，也符合本系統「非高頻」的定位。
4. **全額資金單一部位**：同一時間只持有 0 或 1 個部位，不加碼/分批；size 用
   `equity_at_entry / entry_price` 換算，手續費（`commission_rate`）+ 證交稅
   （`tax_rate`，僅賣出方向課徵）都會計入淨損益。
5. **資料結束時強制平倉**（`ExitReason.EOD_FORCE_CLOSE`），用最後一根收盤價。

指標計算（`total_return`/`annual_return`/`max_drawdown`/`sharpe_ratio`）皆
基於逐 bar 的 mark-to-market 權益曲線；`annual_return` 用 252 個交易日年化。

---

## STRATEGY_PRESETS（可直接用字串名稱呼叫）

| 名稱 | S/R | 進場 | 停損 |
|------|-----|------|------|
| `breakout_swing_atr_v1` | SwingHighLowSR | BreakoutEntry | ATRStopLoss |
| `breakout_volprofile_composite_v1` | VolumeProfileSR | BreakoutEntry | CompositeStopLoss |
| `pullback_atrchannel_structural_v1` | ATRChannelSR | PullbackSupportEntry | StructuralStopLoss |
| `pullback_swing_composite_v1` | SwingHighLowSR | PullbackSupportEntry | CompositeStopLoss |

也可以不用預設組合，直接 `TradingStrategy(name=..., sr_strategy=..., entry_strategy=..., stop_loss_strategy=...)` 自由搭配。

---

## 已知限制

- **多檔股票非共用資金池**：`service.py` 對每檔股票各自跑 100% 資金的獨立
  回測，`_aggregate_result` 用交易筆數加權彙總 win_rate/avg_pnl，
  total_return/annual_return/sharpe 用簡單平均近似整體表現，max_drawdown
  取最差者。不是真正的多檔再平衡組合回測。
- **Volume Profile 用單一 typical price 近似**：沒有 tick 資料可做真正的日內
  成交量分布，用 `(H+L+C)/3` 代表整根K棒的成交量落點。

---

## 型別安全（重要教訓）

`python/backtest/indicators.py` 的 Wilder smoothing 累加迴圈（`calc_atr` /
`calc_rsi` / `calc_macd`）曾經把 Python `float` 累加器跟 numpy 陣列元素相加，
型別被悄悄提升成 `np.float64`。numpy>=2.0 的 `np.float64.__repr__()` 是
`"np.float64(0.1578)"`，這個值若流進 SQLAlchemy 的 SQL bind 參數，
psycopg2 會把它當純文字塞進 SQL，Postgres 因此把 `np` 誤判成 schema 名稱丟出
`InvalidSchemaName`。

修法（已套用）：
1. 這三個函式的回傳值都明確 `float(...)` 轉型。
2. `service.py` 的 `_aggregate_result`/`_trade_to_dict`（DB 寫入前的最後邊界）
   也全部明確轉型，就算未來又有新指標/策略不小心洩漏 numpy 型別，也會在
   這裡被攔下來。
3. `tests/test_type_safety.py` 有迴歸測試，直接斷言這些函式回傳
   `type(x) is float`。

**教訓**：任何會員迴圈中把 Python 原生數值跟 numpy 陣列元素相加的函式，
都要在回傳前明確 `float()`/`int()` 轉型，不能依賴「看起來是 float」就假設
型別安全。
