# Database Schema

## candles

主要 OHLCV 資料表。

```sql
CREATE TABLE candles (
    id         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    symbol     VARCHAR(10)   NOT NULL,
    timeframe  VARCHAR(5)    NOT NULL,   -- '1m', '5m', '1d'
    open       DECIMAL(10,2) NOT NULL,
    high       DECIMAL(10,2) NOT NULL,
    low        DECIMAL(10,2) NOT NULL,
    close      DECIMAL(10,2) NOT NULL,
    volume     BIGINT        NOT NULL,
    amount     DECIMAL(18,2) NOT NULL DEFAULT 0,
    ts         DATETIME(0)   NOT NULL,
    created_at DATETIME(0)   NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_symbol_tf_ts (symbol, timeframe, ts),
    INDEX idx_symbol_tf_ts (symbol, timeframe, ts DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**Index 設計：**
- `uk_symbol_tf_ts`：防止重複寫入（FinMind 重複拉取同一天資料）
- `idx_symbol_tf_ts`：支援 `ORDER BY ts DESC LIMIT N` 查詢

---

## indicator_snapshots

指標快照快取，每次計算後 upsert。

| 欄位 | 類型 | 說明 |
|------|------|------|
| ma5/10/20/60 | DECIMAL(10,4) | 移動平均 |
| rsi14 | DECIMAL(6,4) | RSI（0~100） |
| macd/macd_signal/macd_hist | DECIMAL(10,4) | MACD 三線 |
| bb_upper/middle/lower | DECIMAL(10,4) | 布林通道 |
| atr14 | DECIMAL(10,4) | 平均真實波幅 |
| vwap | DECIMAL(10,4) | 成交量加權均價 |
| vol_ma20 | BIGINT | 20日均量 |
| vol_ratio | DECIMAL(6,4) | 量比（當日量 / MA20） |

---

## signals

訊號記錄，永久保存供回測分析。

| 欄位 | 說明 |
|------|------|
| signal_type | `BREAKOUT`, `BREAKDOWN`, `VOLUME_SPIKE` |
| direction | `BUY`, `SELL`, `WATCH` |
| vol_ratio | 觸發時的量比 |
| resistance/support | 觸發的阻力/支撐價位 |
| trend | 當時趨勢狀態 |

---

## watchlists

監控清單，簡單的 symbol 清單。
