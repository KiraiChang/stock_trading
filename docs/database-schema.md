# Database Schema

支援三種資料庫：**SQLite**（開發）、**MySQL 8.0+**、**PostgreSQL 14+**。  
Migration 由 goose 在啟動時自動執行，不需手動跑 SQL。

---

## candles

主要 OHLCV 資料表。

| 欄位 | 類型 | 說明 |
|------|------|------|
| id | BIGSERIAL / BIGINT AUTO_INCREMENT | PK |
| symbol | VARCHAR(10) | 股票代號，e.g. `2330` |
| timeframe | VARCHAR(5) | `1m`, `5m`, `1d` |
| open / high / low / close | DECIMAL(10,2) | 價格 |
| volume | BIGINT | 成交量 |
| amount | DECIMAL(18,2) | 成交金額 |
| ts | TIMESTAMPTZ / DATETIME(0) | K 棒時間 |

**Index：**
- `UNIQUE(symbol, timeframe, ts)`：防止 FinMind 重複寫入
- `INDEX(symbol, timeframe, ts DESC)`：支援 `ORDER BY ts DESC LIMIT N`

---

## indicator_snapshots

技術指標快照，每次計算後 upsert。

| 欄位 | 類型 | 說明 |
|------|------|------|
| ma5/10/20/60 | DECIMAL(10,4) | 移動平均 |
| rsi14 | DECIMAL(6,4) | RSI（0～100） |
| macd / macd_signal / macd_hist | DECIMAL(10,4) | MACD 三線 |
| bb_upper / middle / lower | DECIMAL(10,4) | 布林通道 |
| atr14 | DECIMAL(10,4) | 平均真實波幅 |
| vwap | DECIMAL(10,4) | 成交量加權均價 |
| vol_ma20 | BIGINT | 20 日均量 |
| vol_ratio | DECIMAL(6,4) | 量比（當日量 / MA20） |

---

## signals

訊號記錄，永久保存供回測分析。

| 欄位 | 說明 |
|------|------|
| signal_type | `BREAKOUT`, `BREAKDOWN`, `VOLUME_SPIKE` |
| direction | `BUY`, `SELL`, `WATCH` |
| vol_ratio | 觸發時的量比 |
| resistance / support | 觸發的阻力 / 支撐價位 |
| trend | 當時趨勢狀態（`BULLISH`, `BEARISH`, `SIDEWAYS`） |

---

## watchlists

監控清單，簡單的 symbol 清單。

| 欄位 | 說明 |
|------|------|
| symbol | 股票代號（UNIQUE） |
| name | 股票名稱 |
| sector | 產業別（可空） |

---

## backtest_jobs

回測任務佇列，Go 寫入後由 Python 消費。

| 欄位 | 說明 |
|------|------|
| job_id | UUID 字串，唯一識別 |
| strategy | 策略名稱，e.g. `breakout_v1` |
| symbols | JSON 陣列，e.g. `["2330","2454"]`（PostgreSQL 為 JSONB） |
| timeframe | K 棒週期 |
| start_date / end_date | 回測區間 |
| status | `pending` → `running` → `done` / `error` |
| trigger | `manual`（API 觸發） |
| started_at / finished_at | 執行時間戳 |

---

## backtest_results

回測績效摘要，Python 寫入後由 Go API 讀取。

| 欄位 | 說明 |
|------|------|
| total_return | 總報酬率 |
| annual_return | 年化報酬率 |
| win_rate | 勝率 |
| max_drawdown | 最大回撤 |
| sharpe_ratio | 夏普比率 |
| total_trades / win_trades / loss_trades | 交易統計 |
| avg_pnl | 平均損益 |

---

## backtest_trades

每筆回測交易明細。

| 欄位 | 說明 |
|------|------|
| job_id | FK → backtest_jobs.job_id |
| symbol | 股票代號 |
| direction | `BUY` / `SELL` |
| entry_time / exit_time | 進出場時間 |
| entry_price / exit_price | 進出場價格 |
| size | 交易股數 |
| pnl / pnl_pct | 損益（絕對值 / 百分比） |
| commission | 手續費 |
