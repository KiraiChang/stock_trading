# API Reference

Base URL：`http://localhost:8080/api/v1`

---

## Candle API

### GET `/candles/:symbol`

取得 K 棒資料。

**Query Parameters：**

| 參數 | 預設 | 說明 |
|------|------|------|
| timeframe | `1d` | `1m`, `5m`, `1d` |
| limit | `60` | 最多 1000 |

**Response：**
```json
{
  "symbol": "2330",
  "timeframe": "1d",
  "candles": [
    { "id": 1, "symbol": "2330", "timeframe": "1d",
      "open": 975.0, "high": 980.0, "low": 970.0, "close": 978.0,
      "volume": 25000000, "amount": 24450000000, "ts": "2024-01-15T00:00:00+08:00" }
  ]
}
```

---

## Indicator API

### GET `/indicators/:symbol`

取得最新技術指標快照。

**Query Parameters：**

| 參數 | 預設 |
|------|------|
| timeframe | `1d` |

**Response：**
```json
{
  "id": 1, "symbol": "2330", "timeframe": "1d",
  "ts": "2024-01-15T00:00:00+08:00",
  "ma5": 975.4, "ma10": 968.2, "ma20": 955.1, "ma60": 920.3,
  "rsi14": 63.5,
  "macd": 8.2, "macd_signal": 6.1, "macd_hist": 2.1,
  "bb_upper": 995.2, "bb_middle": 955.1, "bb_lower": 915.0,
  "atr14": 12.5,
  "vwap": 976.3,
  "vol_ma20": 20000000, "vol_ratio": 1.25
}
```

---

## Signal API

### GET `/signals`

取得訊號記錄。

**Query Parameters：**

| 參數 | 說明 |
|------|------|
| limit | 筆數（預設 50） |
| symbol | 篩選特定股票 |

**Response：**
```json
{
  "signals": [
    {
      "id": 1, "symbol": "2330",
      "signal_type": "BREAKOUT", "direction": "BUY",
      "price": 980.0, "volume": 45000000, "vol_ratio": 2.25,
      "resistance": 975.0, "support": 0, "trend": "BULLISH",
      "note": "突破阻力 975.00，量比 2.25x",
      "ts": "2024-01-15T10:30:00+08:00"
    }
  ],
  "total": 1
}
```

---

## Watchlist API

### GET `/watchlist`
### POST `/watchlist`

```json
{ "symbol": "2330", "name": "台積電", "sector": "半導體" }
```

### DELETE `/watchlist/:symbol`

---

## WebSocket

**連線：** `ws://localhost:8080/ws/market`

**訂閱：**
```json
{ "action": "subscribe", "symbols": ["2330", "2454"] }
```

**取消訂閱：**
```json
{ "action": "unsubscribe", "symbols": ["2330"] }
```

**Server 推送事件：**
```json
{ "type": "candle",    "symbol": "2330", "data": { ...Candle } }
{ "type": "indicator", "symbol": "2330", "data": { ...Snapshot } }
{ "type": "signal",    "symbol": "2330", "data": { ...Signal } }
```
