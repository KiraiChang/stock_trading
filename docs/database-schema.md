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
| watched | 是否透過 WebSocket 即時監聽；同時最多 3 檔為 `true`（`store.MaxWatchedSymbols`），由 `PATCH /watchlist/:symbol/watch` 設定，超過上限回 409 |

---

## users

使用者帳號，密碼以 bcrypt 雜湊儲存。

| 欄位 | 說明 |
|------|------|
| email | 唯一識別，作為登入帳號 |
| password_hash | bcrypt hash，cost=10 |
| status | `active`（可登入）或 `inactive`（預設，需管理員啟用） |
| created_at | 建立時間 |

> 新帳號預設 `inactive`，需要透過 `PATCH /users/:id/status` 或前端使用者管理頁面手動啟用後才能登入。

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

---

## stock_analyses

個股現況分析快照，Go 呼叫 Python 計算後寫入；`trade_verification`/
`verified_at` 由 `POST /api/v1/analysis/:id/verify` 更新（可重複執行，每次
重新計算，非一次性狀態機）。詳見 [stock-analysis.md](./stock-analysis.md)。

| 欄位 | 說明 |
|------|------|
| symbol / timeframe | 分析標的與週期 |
| analyzed_at | 分析當下最後一根K棒的時間（驗證時只看嚴格晚於此時間的資料） |
| current_price | 分析當下收盤價 |
| trend | `BULLISH` / `BEARISH` / `SIDEWAYS` |
| entry_status | `ACTIVE`（已觸發真正進場條件）/ `WATCHING`（觀察中的觸發價位） |
| entry_direction | `LONG` / `SHORT` / `NONE` |
| entry_price | 已觸發：實際進場價；觀察中：觸發價位 |
| entry_reason | 人類可讀的判斷依據 |
| stop_loss_atr / stop_loss_structural / stop_loss_composite | 三種停損價位 |
| take_profit_next_level / take_profit_risk_reward / take_profit_atr | 三種停利價位 |
| trade_verification | JSON：每個停損/停利方法各自「有沒有被觸及、何時、什麼價位」；`entry_status=WATCHING` 時為 `{"applicable": false}` |
| verified_at | 最近一次驗證時間，`NULL` 代表尚未驗證過 |

**Index：** `INDEX(symbol, created_at DESC)`，支援查某檔股票的歷史分析列表。

---

## stock_analysis_levels

`stock_analyses` 底下的支撐/壓力位清單（一對多），驗證時逐筆更新。

| 欄位 | 說明 |
|------|------|
| analysis_id | FK → stock_analyses.id |
| price | 價位 |
| type | `SUPPORT` / `RESISTANCE` |
| strength | 強度（0~1，越高代表訊號越強/越多方法認同） |
| method | 產生此 level 的演算法：`swing` / `atr_channel` / `volume_profile_poc` / `volume_profile_vah` / `volume_profile_val` |
| status | `PENDING`（尚未驗證）/ `HELD_SO_FAR`（目前為止沒被突破）/ `BROKEN`（已被突破） |
| broken_at / broken_price | 第一次被突破的時間與收盤價（`status=BROKEN` 時才有值） |

---

## stock_sr_zone_analyses

SR Zone Scoring 分析快照（機構級版本，見
[sr-zone-scoring.md](./sr-zone-scoring.md)），Go 呼叫 Python `POST /sr-zones`
計算後寫入。跟 `stock_analyses` 不同的地方：這裡沒有驗證/verify 機制。

| 欄位 | 說明 |
|------|------|
| symbol / timeframe | 分析標的與週期 |
| analyzed_at | 分析當下最後一根K棒的時間 |
| current_price | 分析當下收盤價 |
| global_trend | 股票層級趨勢（`trend_slope`），同一次分析所有 zone 共用，只存一次 |
| global_volatility | 股票層級波動率（`ATR / close`），同一次分析所有 zone 共用，只存一次 |
| global_expected_value | 所有「有明確方向」的 zone 依 confidence 加權平均的 EV，是唯一收斂的權威數字（可為 `NULL`） |
| global_confidence | 所有 zone confidence 的簡單平均（可為 `NULL`） |
| global_risk_reward_ratio | 所有「有明確方向」的 zone 依 confidence 加權平均的 RR（可為 `NULL`） |
| model_version | 產生這筆分析所用的模型版本（來自 `ModelBundle.version`，例如 `"v2"`）；Python 端萬一沒回傳則寫 `"unknown"` |

**Index：** `INDEX(symbol, created_at DESC)`。

---

## stock_sr_zones

`stock_sr_zone_analyses` 底下的 zone 清單（一對多）。跟 `stock_analysis_levels`
不同：每個 zone 是一段**價格區間**（`price_low`~`price_high`），不是單一
價位，且欄位數量遠多於 Level（含機率、EV、RR、量能確認等 ML 產出的數字）。

| 欄位 | 說明 |
|------|------|
| analysis_id | FK → stock_sr_zone_analyses.id（Go 端手動 2 步刪除，非 DB `ON DELETE CASCADE`） |
| price_low / price_high | 區間上下緣 |
| method | `atr` / `volume_profile` |
| role | `SUPPORT` / `RESISTANCE` / `AT_ZONE`（依現價動態判斷，不是建立時就固定） |
| tier / tier_label | 依區間寬度分三層：`TIER_1_MAIN_STRUCTURE`（主結構）/ `TIER_2_TRADING_ZONE`（交易區）/ `TIER_3_SHORT_TERM`（短期支撐） |
| support_score / resistance_score | 依機率貝式收縮而來的強度分數（0~1） |
| net_score / net_score_label | `support_score - resistance_score`；`STRONG_SUPPORT` / `NEUTRAL` / `STRONG_RESISTANCE` |
| confidence / confidence_level | 多因子可信度（樣本數/時間衰減/歷史穩定度）；`LOW`/`MEDIUM`/`HIGH`/`VERY_HIGH` |
| bounce_probability / break_probability | 反彈/跌破機率（`role=AT_ZONE` 時為 `NULL`） |
| expected_gain / expected_loss / expected_value | 角色解析後的平均反彈/跌破報酬、加權期望值 |
| risk_reward_ratio / reward_risk_percentile | `|expected_gain/expected_loss|`；此比值在訓練資料歷史分佈中的百分位 |
| relative_volume / volume_confirmation | 角色解析後的相對量能；`CONFIRMED`/`WEAK`/`NEUTRAL`/`FAILED` |
| touch_count / reject_count / break_count | 觸碰/拒絕/突破次數（聚合值，不分方向） |
| zone_momentum / zone_direction | 這個 zone 自己的歷史觸碰動能（逐 zone 不同，非股票層級量）；`UP`/`DOWN`/`FLAT` |
| recent_validation | `VALIDATED_RECENTLY` / `PENDING_VALIDATION` / `NOT_TESTED_RECENTLY` / `EXPIRED` |
| trading_score | 可拆解的綜合交易分數（0~100） = EV(40%) + RR(20%) + Trend(15%) + Volume(15%) + Confidence(10%) |
| trading_score_breakdown | JSON：`trading_score` 五個分量各自的加權貢獻值，加總即為 `trading_score` |
| trading_recommendation | `STRONG_BUY`/`BUY`/`WATCH`/`NEUTRAL`/`AVOID`/`STRONG_SELL` |
| status / broken_at / broken_price | 保留供未來 verifier 使用，目前沒有任何程式碼會更新，永遠是 `PENDING` |

**Index：** `INDEX(analysis_id)`；查詢時額外依 `tier` 排序（`CASE tier WHEN
'TIER_1_MAIN_STRUCTURE' THEN 1 ...`）後再依 `trading_score DESC`。

---

## sr_scoring_train_jobs

SR Zone Scoring 機率模型的訓練任務紀錄（見
[sr-zone-scoring.md](./sr-zone-scoring.md)「訓練任務可觀測化」）。訓練本身在
Go 背景 goroutine 呼叫 Python 同步執行，這張表讓 `POST /sr-zones/train`
可以立即回傳 `job_id`，前端輪詢 `GET /sr-zones/train-jobs/:job_id` 查詢進度，
不用只靠伺服器 log。

| 欄位 | 說明 |
|------|------|
| job_id | 任務識別碼（`sr_train_<時間戳>` 格式），API 查詢用這個而不是 `id` |
| status | `pending`（已建立，尚未開始）/ `running`（訓練中）/ `done`（成功）/ `failed`（失敗） |
| symbols | JSON 陣列字串，這次訓練用的股票代號清單 |
| timeframe / fetch_limit / model_type | 訓練參數（K棒週期、每檔股票抓取根數、`gradient_boosting`/`logistic_regression`） |
| rows / sources | 訓練資料筆數、來源股票數；只有 `status=done` 才有值 |
| metrics | JSON：`{"hold": {...}, "break": {...}}`，兩個模型各自的 accuracy/precision/recall/auc/brier_score/log_loss/train_rows/test_rows/positive_rate_train/positive_rate_test/calibrated；只有 `status=done` 才有值。DB 欄位 `NOT NULL DEFAULT ''`（用 `store.RawJSON` 讀寫，不能是 SQL `NULL`，空字串在 API 回應會序列化成 `null`） |
| model_path / model_version | 訓練完成後寫入的模型檔路徑與版本；只有 `status=done` 才有值 |
| dataset_summary | JSON：`summarize_training_dataset()` 的診斷摘要（見 sr-zone-scoring.md「四」），只有 `status=done` 才有值。DB 欄位同樣 `NOT NULL DEFAULT ''` |
| error | 失敗原因；只有 `status=failed` 才有值 |
| started_at / finished_at | 開始/結束時間；`status=pending` 時兩者皆為 `NULL` |
| created_at | 任務建立時間（等同呼叫 `POST /sr-zones/train` 的時間） |

**Index：** `INDEX(created_at DESC)`。
