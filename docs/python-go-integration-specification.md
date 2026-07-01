# Python ↔ Go Integration Specification

## System Overview

```
[Python]  ← Research / Backtest Layer
   ↑  ↓  （共用同一個 DB）
[Go]      ← Production / Real-time Layer
```

---

# 1. Role Separation

## Go（Production Layer）

- 即時行情處理（FinMind 輪詢 / 選填 Fugle 並行）
- 技術指標計算（MA / RSI / MACD / VWAP / ATR / Bollinger）
- Breakout / Breakdown 判斷
- Signal 生成與 WebSocket 推播
- Watchlist 掃描（~1900 檔）
- 回測任務管理（寫入 backtest_jobs）
- 個股分析結果持久化與**驗證**（`internal/analysis.Verifier`：比對 candles
  跟已存的支撐/壓力/停損/停利，純 Go，不呼叫 Python）

## Python（Research Layer）

- 策略研究與回測，兩種引擎並存：
  - `backtest/strategy/breakout_v1.py`（backtrader，與 Go signal engine 1:1 對齊）
  - `backtest/modular/`（純 pandas/numpy，Strategy Pattern 可替換 S/R／進場／
    停損元件，見 [backtest-modular-strategy.md](./backtest-modular-strategy.md)）
- 指標驗證（`backtest/indicators.py`，與 Go 1:1 對齊）
- 統計分析
- 回測結果寫回 DB（backtest_results + backtest_trades）——兩種引擎輸出格式相同
- 個股現況分析計算（`backtest/modular/analysis.py`：支撐/壓力/進場/停損/停利，
  純函式、**不寫 DB**，由 Go 呼叫後負責持久化，見 3.2）

---

# 2. Go → Python 驅動方式

## 方式 A：DB Polling（已實作，預設）

```
Go 寫入 backtest_jobs（status='pending'）
        ↓
Python worker 每 10 秒掃描 pending 任務
        ↓
執行 backtrader 回測
        ↓
寫入 backtest_results + backtest_trades
        ↓
更新 backtest_jobs.status = 'done'
```

啟動：`python worker.py`

## 方式 B：HTTP Service（已實作，可選）

```
Go POST /backtest → Python FastAPI（port 8001）
Python 執行回測（同步）
Python 回傳結果 + 寫回 DB
```

啟動：`uvicorn http_server:app --port 8001`  
Go 端需在 `config.yaml` 設定 `python.service_url: http://localhost:8001`。

---

# 3. Job Schema

寫入 `backtest_jobs` 資料表：

```json
{
  "job_id": "bt_20260624_001",
  "type": "backtest",
  "strategy": "breakout_v1",
  "symbols": ["8088", "2399"],
  "timeframe": "1d",
  "start_date": "2023-01-01",
  "end_date": "2026-06-01",
  "status": "pending",
  "trigger": "manual"
}
```

---

# 3.1 策略引擎分派

`backend/internal/backtest`／`python/worker.py`／`python/http_server.py` 完全
不知道有兩種引擎存在——分派邏輯全部封裝在 `python/backtest/engine.py` 的
`run_backtest()`：

```python
if strategy in MODULAR_STRATEGIES:      # backtest/modular/strategy.py 的 STRATEGY_PRESETS
    return run_modular_backtest(...)     # 純 pandas/numpy 引擎
# 否則走 STRATEGY_MAP（backtrader）
```

新增模組化策略只需要在 `STRATEGY_PRESETS` 註冊，`job.Strategy` 欄位填對應
名稱即可路由，Go 端、資料庫 schema、`db_writer.py` 都不需要改動。

---

# 3.2 個股分析（Stock Analysis）流程

跟回測不同，個股分析是**同步**呼叫（不是寫 job 表等 worker 輪詢），而且
「計算」跟「驗證」拆在兩個語言各自負責，理由分別是：

- **計算**（支撐/壓力/進場/停損/停利）需要 `backtest/modular` 的演算法，
  所以在 Python：`POST /analyze`（`python/http_server.py`）同步回傳結果，
  不寫 DB。
- **驗證**（比對後續 candles，檢查支撐/壓力有沒有被突破、停損/停利有沒有
  被觸及）只是價格數字比大小，不需要重跑策略邏輯，所以直接在 Go 做
  （`internal/analysis.Verifier`），這樣驗證功能不會被「Python service 沒開」
  卡住，也不用把 candles 資料再送一次去 Python。

```
Go POST Python /analyze {symbol, timeframe}
    ↓ 同步回傳（不寫 DB）
Go 寫入 stock_analyses + stock_analysis_levels
    ↓（使用者之後手動觸發 POST /analysis/:id/verify，可重複執行）
Go 讀 candles（analyzed_at 之後），純 Go 比對：
    - 支撐位：收盤 < 支撐價 → BROKEN
    - 壓力位：收盤 > 壓力價 → BROKEN
    - 停損/停利（僅 entry_status=ACTIVE 才檢查）：當根最低/最高價觸及即算 Hit
    ↓
更新 stock_analyses.trade_verification（JSON）+ stock_analysis_levels.status
```

完整數學規格與資料表結構見 [stock-analysis.md](./stock-analysis.md)、
[database-schema.md](./database-schema.md)。

---

# 4. Backtest Data Standard

## 核心原則

> Python 與 Go 必須使用同一份 DB schema，讀取同一份 candles 資料。

## Candle Schema（正式定義）

```go
type Candle struct {
    Symbol    string
    Timeframe string
    Open      float64
    High      float64
    Low       float64
    Close     float64
    Volume    int64
    Amount    float64
    Timestamp int64  // Unix timestamp
}
```

## Python DataFrame 欄位對應

```python
columns = ["symbol", "timeframe", "open", "high", "low",
           "close", "volume", "amount", "timestamp"]
```

timestamp 取法：
- SQLite：`CAST(strftime('%s', ts) AS INTEGER)`
- MySQL：`UNIX_TIMESTAMP(ts)`
- PostgreSQL：`EXTRACT(EPOCH FROM ts)::BIGINT`

---

# 5. Strategy Consistency Rule

> Python 回測邏輯必須與 Go production 邏輯 1:1 對齊。

禁止：
- Python 使用不同的 MA 計算方式
- Python 加入 Go 沒有的平滑處理
- 不同的成交量平均窗口

必須共用的參數：
- MA window（5 / 10 / 20 / 60）
- Volume average window（20）
- Breakout 條件（Close > Resistance, VolRatio >= 2.0, Trend == BULLISH）
- Support/Resistance 識別邏輯（window=3, merge threshold=1%）

---

# 5.1 型別安全規則（寫入 DB 前必須是原生型別）

`backtest_results`/`backtest_trades` 是透過 SQLAlchemy `text()` + 具名參數寫入，
**任何要寫入的數值都必須是原生 Python `float`/`int`，不能是 `np.float64`/
`np.int64`**。numpy>=2.0 的純量 `repr()` 格式改變後，這類型別若流進 SQL bind
參數會讓 psycopg2 退化成把 repr 字串塞進 SQL，導致 Postgres 解析錯誤
（實際案例見 [backtest-modular-strategy.md](./backtest-modular-strategy.md#型別安全重要教訓)）。

規則：任何在累加迴圈中把原生數值跟 numpy 陣列元素相加的函式，回傳前必須
明確 `float()`/`int()` 轉型；`db_writer.py` 呼叫端（`write_result`/
`write_trades`）視為最後一道防線，組裝要寫入的 dict 時也要明確轉型。

---

# 6. 資料庫設定同步

`backend/config.yaml` 與 `python/config.yaml` 的 `database` 區段需指向同一個 DB。  
Docker Compose 環境下透過環境變數統一設定，不需手動同步。

---

# 7. Design Philosophy

- Go = real-time decision engine
- Python = truth validation engine
- DB = historical truth source
- Strategy correctness > performance
