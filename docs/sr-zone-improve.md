# 股票支撐／壓力決策引擎完整修改計畫書

## 1. 修改目標

目前系統的 Zone Detection、EV／RR、籌碼整合、Daily Candidate Zone、Entry Relevance 與 Defense Line 已具備可用基礎。下一階段不再優先增加 RSI、MACD、KD 等技術指標，而是集中處理以下核心問題：

1. 事件生命週期不完整。
2. Market Bias、Entry Permission、Position Action 職責重疊。
3. Entry、Daily Confirmation 與 Position Decision 尚未完全統一仲裁。
4. 部分欄位名稱、角色與顯示語意不一致。
5. 缺少穩定的 regression fixture、walk-forward 與版本回歸驗證。
6. 模型 AUC 接近隨機水準，分數與機率不可直接視為可靠預測。

---

## 2. 保留的既有模組

以下模組保留，不建議重寫：

- Zone Detection Engine
- Zone Scoring Engine
- Entry Relevance Engine
- Expected Value Engine
- Risk／Reward Engine
- Chip Scoring Engine
- Daily Price Action Engine
- Daily Candidate Zone Engine
- Defense Line Engine
- Price Path Engine
- Explanation Engine

建議保留的資料結構：

- `nearest_support_zone`
- `nearest_resistance_zone`
- `primary_structural_zone`
- `primary_zone`
- `entry_relevance_breakdown`
- `confluence_family_count`
- `confluence_families`
- `daily_candidate_zones`
- `defense_lines.tactical`
- `defense_lines.swing`
- `defense_lines.strategic`
- `price_path.transitions`
- `final_entry_permission`
- `position_action_condition`

---

## 3. 目標架構

建議將分析流程固定為八層：

```text
1. Market Data Layer
2. Feature Layer
3. Zone Detection Layer
4. Event Detection Layer
5. Event Lifecycle Layer
6. Market State Layer
7. Decision Arbitration Layer
8. Explanation / API Layer
```

資料流：

```text
OHLCV + Chip
    ↓
Feature Engineering
    ↓
Zone Detection
    ↓
Raw Event Detection
    ↓
Active Event State
    ↓
Market State
    ↓
Entry / Position Arbitration
    ↓
Explanation / API / UI
```

設計原則：

```text
Detection 與 Decision 分離
Historical Event 與 Active State 分離
Market Bias、Entry Permission、Position Action 分離
所有最終狀態只能有單一輸出
所有事件都必須能確認、失敗、失效、解決或過期
```

---

# 4. 第一階段：資料語意與明確 Bug 修正

優先級：P0  
預估工期：1～2 天

## 4.1 分離時間層級與角色名稱

目前可能出現：

```json
{
  "role": "RESISTANCE",
  "tier_label": "短期支撐"
}
```

應改為：

```json
{
  "tier": "TIER_3_SHORT_TERM",
  "tier_label": "短期",
  "role": "RESISTANCE",
  "role_label": "壓力",
  "display_label": "短期壓力"
}
```

規則：

- `tier_label` 只描述時間層級。
- `role_label` 只描述支撐／壓力角色。
- `display_label` 由 API 或前端組合。

## 4.2 統一籌碼訊號門檻

建議門檻：

```text
-100 ～ -30：BEARISH
-30 ～ -10：WEAK_BEARISH
-10 ～ +10：NEUTRAL
+10 ～ +30：WEAK_BULLISH
+30 ～ +100：BULLISH
```

建議 Schema：

```json
{
  "score": 14.99,
  "signal": "WEAK_BULLISH",
  "directional": true,
  "decision_weight": 0.15,
  "coverage": 1.0,
  "confidence": 1.0
}
```

UI 顏色與文字必須依 `signal` 顯示，不能只依正負號著色。

## 4.3 移除重複 Action 欄位

目前容易出現：

```text
market_action = BUY
action = Buy
final_entry_permission = WAIT_CONFIRMATION
```

建議停用：

```text
market_action
action
action_label
```

保留唯一三軸：

```json
{
  "market_bias": "BULLISH_CONTINUATION",
  "final_entry_permission": {
    "state": "WAIT_CONFIRMATION"
  },
  "position_action": "HOLD"
}
```

| 欄位 | 回答問題 |
|---|---|
| `market_bias` | 市場方向與偏向 |
| `final_entry_permission` | 未持有者現在能否進場 |
| `position_action` | 已持有者目前如何處理 |

## 4.4 統一 Confluence 顯示

評分只使用 `confluence_family_count`，不使用 `confluence_count`。

```json
{
  "confluence": {
    "family_count": 3,
    "raw_method_count": 8,
    "families": [
      "RECENT_MICROSTRUCTURE",
      "STRUCTURAL_ATR",
      "VOLUME_PROFILE"
    ]
  }
}
```

UI 摘要只顯示「證據族群 ×3」，`raw_method_count` 只在 Debug 或完整明細顯示。

---

# 5. 第二階段：Event Lifecycle Engine

優先級：P0  
預估工期：3～5 天

## 5.1 拆分 Raw Event 與 Stateful Event

### `market_event_detections`

```sql
CREATE TABLE market_event_detections (
    id BIGSERIAL PRIMARY KEY,
    symbol VARCHAR(20) NOT NULL,
    timeframe VARCHAR(10) NOT NULL,
    trade_date DATE NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    direction VARCHAR(20) NOT NULL,
    event_scope VARCHAR(30) NOT NULL,
    zone_key VARCHAR(100),
    zone_low NUMERIC(18,6),
    zone_high NUMERIC(18,6),
    price_level NUMERIC(18,6),
    confidence NUMERIC(8,6),
    reason TEXT,
    model_version VARCHAR(30),
    config_hash VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### `market_event_states`

```sql
CREATE TABLE market_event_states (
    id BIGSERIAL PRIMARY KEY,
    symbol VARCHAR(20) NOT NULL,
    timeframe VARCHAR(10) NOT NULL,
    root_event_id BIGINT NOT NULL,
    event_family VARCHAR(50) NOT NULL,
    event_scope VARCHAR(30) NOT NULL,
    current_state VARCHAR(40) NOT NULL,
    current_direction VARCHAR(20),
    started_at DATE NOT NULL,
    updated_at DATE NOT NULL,
    resolved_at DATE,
    resolution_type VARCHAR(50),
    active BOOLEAN NOT NULL DEFAULT TRUE,
    version INTEGER NOT NULL DEFAULT 1,
    metadata JSONB NOT NULL DEFAULT '{}'
);
```

建議索引：

```sql
CREATE INDEX idx_market_event_states_active
ON market_event_states(symbol, timeframe, active);

CREATE INDEX idx_market_event_detections_lookup
ON market_event_detections(symbol, timeframe, trade_date, event_type);
```

## 5.2 事件 Scope

```text
GLOBAL
STRUCTURAL_ZONE
PRIMARY_ZONE
TACTICAL_ZONE
DAILY_ZONE
```

事件唯一識別建議：

```text
symbol + timeframe + event_family + event_scope + zone_key
```

## 5.3 事件狀態

```text
DETECTED
ACTIVE
CONFIRMING
CONFIRMED
RESOLVED
FAILED
INVALIDATED
EXPIRED
```

事件結果：

```text
BULLISH_RECOVERY
BEARISH_CONTINUATION
FALSE_BREAKDOWN
FAILED_RECLAIM
SUCCESSFUL_BREAKOUT
FAILED_BREAKOUT
TIME_EXPIRED
```

## 5.4 0050 標準 Fixture 狀態轉移

### 2026-07-14

```text
NORMAL
→ EXTREME_VOLUME
→ HIGH_VOLUME_BREAKDOWN
→ INTRADAY_RECLAIM
→ REVERSAL_CANDIDATE
```

### 2026-07-15

```text
REVERSAL_CANDIDATE
→ NEXT_DAY_FOLLOW_THROUGH
→ SUPPORT_RECLAIM_CONFIRMING
```

### 2026-07-16

```text
SUPPORT_RECLAIM_CONFIRMING
→ SUPPORT_RECLAIM_CONFIRMED
→ BULLISH_RECOVERY
```

最終事件：

```json
{
  "current_state": "RESOLVED",
  "resolution_type": "BULLISH_RECOVERY",
  "active": false
}
```

歷史事件仍保留，但不得繼續進入 Active Risk Gate。

## 5.5 State Transition 規則

所有規則必須實作為純函數，不可散落於 UI、handler 或 repository。

```text
REVERSAL_CANDIDATE
AND latest.close > primary_zone.high
AND latest.close_location >= 0.65
AND latest.close >= previous.close
→ SUPPORT_RECLAIM_CONFIRMING
```

```text
SUPPORT_RECLAIM_CONFIRMING
AND close_above_zone_count >= 2
AND latest.close > primary_zone.high
→ SUPPORT_RECLAIM_CONFIRMED
```

```text
SUPPORT_RECLAIM_CONFIRMING
AND latest.close < primary_zone.low
→ FAILED_RECLAIM
```

```text
SUPPORT_RECLAIM_CONFIRMED
AND latest.close >= next_resistance.low
→ RESISTANCE_TESTING
```

---

# 6. 第三階段：Market State Engine

優先級：P1  
預估工期：2～3 天

將目前重疊的 Regime 欄位收斂為四軸。

## 6.1 Structural Trend

```text
STRONG_UP
TREND_UP
SIDEWAYS
TREND_DOWN
STRONG_DOWN
```

## 6.2 Tactical Regime

```text
EXPANSION
PULLBACK
BREAKDOWN_SHOCK
RECOVERY
CONSOLIDATION
BREAKOUT_ATTEMPT
```

## 6.3 Structure State

```text
SUPPORT_HOLDING
SUPPORT_RECLAIM_CANDIDATE
SUPPORT_RECLAIM_CONFIRMED
SUPPORT_FAILED
RESISTANCE_TESTING
RESISTANCE_BREAKOUT_CONFIRMED
```

## 6.4 Volatility State

```text
CONTRACTED
NORMAL
EXPANDED
EXTREME
NORMALIZING_AFTER_EXTREME
```

標準輸出：

```json
{
  "structural_trend": "TREND_UP",
  "tactical_regime": "RECOVERY",
  "structure_state": "SUPPORT_RECLAIM_CONFIRMED",
  "volatility_state": "NORMALIZING_AFTER_EXTREME"
}
```

---

# 7. 第四階段：Decision Arbitration Engine

優先級：P0  
預估工期：3～4 天

所有最終 Decision 必須由單一仲裁器產生。

## 7.1 Entry State

```text
BLOCKED
WAIT_CONFIRMATION
PROBE_ONLY
SMALL_ENTRY
ACCUMULATE
FULL_ENTRY
```

排序：

```text
BLOCKED < WAIT_CONFIRMATION < PROBE_ONLY < SMALL_ENTRY < ACCUMULATE < FULL_ENTRY
```

## 7.2 Position Action

```text
EXIT
REDUCE
CONDITIONAL_HOLD
HOLD
ADD_ON_RETEST
ADD_ON_BREAKOUT
```

## 7.3 仲裁輸入

```text
Zone Quality
Entry Relevance
Zone Lifecycle
Active Event State
Daily Confirmation
RR Gate
EV Gate
Market Regime
Distance to Support
Distance to Resistance
Volume State
Chip State
Momentum State
Blocking Zone
```

## 7.4 Hard Gates

### 禁止進場

```text
active_event = HIGH_VOLUME_BREAKDOWN
AND reclaim_state NOT IN CONFIRMING / CONFIRMED
```

```text
risk_reward_ratio < minimum_rr
```

```text
current_price inside blocking resistance
AND breakout not confirmed
```

```text
expected_value <= 0
AND confidence < configured_threshold
```

### 最多觀察性試探

```text
zone.lifecycle = CANDIDATE
OR recent_validation = PENDING_VALIDATION
```

### 可小量進場

```text
zone.lifecycle = CONFIRMED
AND RR >= 1.8
AND EV > 0
AND no active bearish event
AND price not inside blocking resistance
```

### 可加碼

```text
support_reclaim_confirmed
AND retest_holds
AND momentum >= WEAK_POSITIVE
AND nearest_resistance_distance allows RR
```

## 7.5 Entry 仲裁矩陣

| Global Entry | Daily Entry | Final Entry |
|---|---|---|
| BLOCKED | 任意 | BLOCKED |
| WAIT_CONFIRMATION | PROBE_ALLOWED | WAIT_CONFIRMATION 或 PROBE_ONLY，依 policy |
| PROBE_ONLY | PROBE_ALLOWED | PROBE_ONLY |
| SMALL_ENTRY | PROBE_ALLOWED | PROBE_ONLY |
| SMALL_ENTRY | CONFIRMED | SMALL_ENTRY |
| ACCUMULATE | CONFIRMED | ACCUMULATE |

## 7.6 Position 仲裁規則

### HOLD

```text
price > primary_zone.high
AND structure_state = SUPPORT_RECLAIM_CONFIRMED
AND no active bearish event
```

### CONDITIONAL_HOLD

```text
price near primary_zone
AND structure_state IN SUPPORT_HOLDING / SUPPORT_RECLAIM_CANDIDATE
```

### REDUCE

```text
close < tactical_defense
AND bearish_follow_through
```

### EXIT

```text
close < swing_defense
AND active bearish event confirmed
AND no reclaim within configured window
```

禁止僅因歷史 `HIGH_VOLUME_BREAKDOWN` 曾經發生就持續輸出 EXIT。

---

# 8. 第五階段：Best Trade Zone 與 Price Path

優先級：P1  
預估工期：2 天

## 8.1 Best Trade Zone 不回傳 null

```json
{
  "state": "MISSED",
  "zone": {
    "price_low": 104.73,
    "price_high": 105.37
  },
  "reason": "CURRENT_PRICE_MOVED_ABOVE_ZONE"
}
```

State：

```text
AVAILABLE
APPROACHING
ACTIVE
MISSED
INVALIDATED
NOT_AVAILABLE
```

## 8.2 分離 Immediate Trigger 與 Structural Decision

```json
{
  "immediate_trigger": {
    "price": 106.75,
    "type": "DAILY_HIGH_BREAK"
  },
  "next_decision_zone": {
    "price_low": 107.23,
    "price_high": 107.87,
    "type": "STRUCTURAL_RESISTANCE"
  }
}
```

## 8.3 Transition 加入 Priority

```json
{
  "priority": 1,
  "if": "close_below_104.73",
  "then": "INVALIDATION_RISK",
  "price": 104.73
}
```

順序：

```text
1. Invalidation
2. Recovery
3. Immediate Trigger
4. Next Structural Decision
5. Strategic Re-evaluation
```

---

# 9. 第六階段：Confidence 與統計模型

優先級：P0  
預估工期：5～10 天

目前 Hold 與 Break 模型 AUC 約落在 0.50～0.52，接近隨機分類。此時：

- 不得把機率直接稱為可信勝率。
- 不得僅因 `calibrated=true` 就視為模型有效。
- EV、RR 與 Trading Score 必須清楚標示規則模型或統計模型。

## 9.1 Model Health Gate

```json
{
  "model_health": {
    "status": "WEAK_EDGE",
    "auc": 0.5243,
    "brier_score": 0.2499,
    "sample_size": 596,
    "usable_for_ranking": true,
    "usable_for_probability_claim": false
  }
}
```

Status：

```text
UNAVAILABLE
INSUFFICIENT_SAMPLE
WEAK_EDGE
USABLE
STRONG
DEGRADED
```

暫定門檻：

```text
AUC < 0.53：WEAK_EDGE
0.53 ～ 0.58：USABLE_FOR_RANKING
0.58 ～ 0.65：USABLE
> 0.65：STRONG
```

## 9.2 Confidence 因子補齊

```text
sample_factor
recency_factor
stability_factor
method_independence_factor
market_regime_factor
```

示意：

```text
confidence =
0.25 * sample_factor
+ 0.20 * recency_factor
+ 0.20 * stability_factor
+ 0.20 * method_independence_factor
+ 0.15 * market_regime_factor
```

尚未實作的因子應輸出：

```json
{
  "available": false
}
```

不要輸出 null 假裝已完成。

## 9.3 機率與分數命名

若模型能力不足，將 `bounce_probability` 暫時改為：

```text
bounce_score
```

或：

```text
estimated_bounce_likelihood
```

直到 out-of-sample 與 calibration 通過門檻後，再使用正式 Probability 命名。

---

# 10. 第七階段：資料庫正規化

優先級：P1  
預估工期：3～5 天

## 10.1 JSON 字串改為 JSONB

```sql
ALTER TABLE stock_sr_zone_analyses
ALTER COLUMN decision_summary TYPE JSONB
USING decision_summary::jsonb;
```

其他 JSON 欄位同理。

## 10.2 建議拆表

```text
stock_sr_zone_analyses
stock_sr_zones
stock_sr_decisions
stock_sr_daily_candidates
market_event_detections
market_event_states
stock_sr_model_metrics
stock_sr_regression_results
```

### `stock_sr_zones`

```sql
CREATE TABLE stock_sr_zones (
    id BIGSERIAL PRIMARY KEY,
    analysis_id BIGINT NOT NULL REFERENCES stock_sr_zone_analyses(id),
    zone_role VARCHAR(20) NOT NULL,
    tier VARCHAR(40) NOT NULL,
    source VARCHAR(40) NOT NULL,
    lifecycle VARCHAR(30) NOT NULL,
    price_low NUMERIC(18,6) NOT NULL,
    price_high NUMERIC(18,6) NOT NULL,
    confidence NUMERIC(8,6),
    trading_score NUMERIC(10,6),
    expected_value NUMERIC(10,8),
    risk_reward_ratio NUMERIC(10,6),
    confluence_family_count INTEGER,
    metadata JSONB NOT NULL DEFAULT '{}'
);
```

## 10.3 Analysis Snapshot 不可變

每次分析建立 immutable snapshot：

```text
symbol
timeframe
analyzed_at
model_version
model_config_hash
pipeline_version
input_data_hash
```

重新分析必須新增一筆新紀錄，不得覆寫舊分析。

---

# 11. 第八階段：Regression Fixture 與 Walk-Forward

優先級：P0  
預估工期：持續進行

## 11.1 建立固定 Fixture

### 0050 / 2026-07-14

```text
Expected:
- HIGH_VOLUME_BREAKDOWN
- INTRADAY_RECLAIM
- REVERSAL_CANDIDATE
- final_entry_permission != SMALL_ENTRY
- position_action = CONDITIONAL_HOLD
```

### 0050 / 2026-07-15

```text
Expected:
- NEXT_DAY_FOLLOW_THROUGH
- SUPPORT_RECLAIM_CONFIRMING
- position_action IN HOLD / CONDITIONAL_HOLD
- position_action != EXIT
```

### 0050 / 2026-07-16

```text
Expected:
- SUPPORT_RECLAIM_CONFIRMED
- market_bias = BULLISH_CONTINUATION
- final_entry_permission = WAIT_CONFIRMATION
- position_action = HOLD
```

## 11.2 Fixture 格式

```yaml
symbol: "0050"
timeframe: "1d"
trade_date: "2026-07-16"

expected:
  structural_trend: TREND_UP
  tactical_regime: RECOVERY
  structure_state: SUPPORT_RECLAIM_CONFIRMED
  market_bias: BULLISH_CONTINUATION
  final_entry_permission:
    allowed:
      - WAIT_CONFIRMATION
  position_action:
    allowed:
      - HOLD
  forbidden:
    - EXIT
```

## 11.3 回歸測試類型

```text
Unit Test
State Transition Test
Decision Arbitration Test
Golden Snapshot Test
Database Migration Test
Backtest Regression Test
Walk-Forward Test
Calibration Test
```

## 11.4 Walk-Forward 方法

```text
Train：前 24 個月
Validation：後 3 個月
Test：再後 3 個月
向前滾動
```

每一輪保存：

```text
AUC
Brier Score
Log Loss
Calibration Error
Coverage
Average EV
Win Rate
Max Drawdown
Profit Factor
Signal Count
```

---

# 12. API 目標 Schema

```json
{
  "analysis": {
    "symbol": "0050",
    "timeframe": "1d",
    "analyzed_at": "2026-07-16",
    "model_version": "v4",
    "config_hash": "ec4cd416c66b"
  },
  "market_state": {
    "structural_trend": "TREND_UP",
    "tactical_regime": "RECOVERY",
    "structure_state": "SUPPORT_RECLAIM_CONFIRMED",
    "volatility_state": "NORMALIZING_AFTER_EXTREME"
  },
  "decision": {
    "market_bias": "BULLISH_CONTINUATION",
    "final_entry_permission": {
      "state": "WAIT_CONFIRMATION",
      "reason_codes": [
        "BLOCKING_RESISTANCE_AHEAD",
        "MOMENTUM_NOT_CONFIRMED"
      ]
    },
    "position_action": {
      "state": "HOLD",
      "reason_codes": [
        "SUPPORT_RECLAIM_CONFIRMED",
        "PRICE_ABOVE_PRIMARY_ZONE"
      ]
    }
  },
  "zones": {
    "primary": {},
    "nearest_support": {},
    "nearest_resistance": {},
    "primary_structural": {},
    "best_trade_zone": {
      "state": "MISSED",
      "zone": {}
    }
  },
  "events": {
    "active": [],
    "resolved": [],
    "sequence": []
  },
  "price_path": {
    "invalidation": {},
    "recovery": {},
    "immediate_trigger": {},
    "next_decision_zone": {}
  },
  "defense_lines": {
    "tactical": {},
    "swing": {},
    "strategic": {}
  },
  "model_health": {
    "status": "WEAK_EDGE",
    "usable_for_ranking": true,
    "usable_for_probability_claim": false
  }
}
```

---

# 13. 實作順序

## Sprint 1：語意與 P0 Bug

```text
1. 修正 tier_label / role_label
2. 統一籌碼 signal
3. 停用 market_action / action
4. 統一 confluence family 顯示
5. 修正 Position Action 與 Entry 衝突
```

驗收：

- 不再出現 BUY + WAIT_CONFIRMATION。
- 不再出現 Resistance + 短期支撐。
- UI 不再使用 raw confluence count 當獨立證據數。

## Sprint 2：Event Lifecycle

```text
1. 建立 event detection table
2. 建立 event state table
3. 實作 event scope
4. 實作 state transition
5. 實作 resolve / fail / expire
6. 接上 Active Risk Gate
```

驗收：

- 7/14 的 breakdown 不會在 7/16 繼續阻擋 Position。
- Event Sequence 能從 Candidate 升級至 Confirmed／Resolved。

## Sprint 3：Decision Arbitration

```text
1. 建立 Entry precedence
2. 建立 Position rules
3. 建立 hard gate
4. 建立唯一 final decision
5. 建立 reason codes
```

驗收：

- Entry、Daily Entry、Position 不再互相矛盾。
- API 僅有一個最終 Entry Permission。

## Sprint 4：Model Health 與 Confidence

```text
1. 實作 model health gate
2. 補齊 confidence factors
3. 區分 score 與 probability
4. 建立 calibration report
5. 建立 walk-forward output
```

驗收：

- AUC 低時，UI 不宣稱精確勝率。
- 所有機率都有 sample size、calibration 與 health 狀態。

## Sprint 5：Regression 與資料庫正規化

```text
1. 建立 0050 fixtures
2. 建立其他 ETF / 個股 fixtures
3. 建立 golden snapshot
4. JSON text 改 JSONB
5. 拆分 zone / event / decision tables
```

驗收：

- 每次模型升版自動跑回歸。
- 狀態轉移結果可重現。
- 舊分析不可被覆寫。

---

# 14. 完成定義

此修改計畫完成後，系統必須具備：

- Zone Detection 與 Decision 完全解耦。
- Historical Event 與 Active Event 分離。
- Event 有完整生命週期。
- Market Bias、Entry、Position 各自只有唯一責任。
- 所有 Decision 由單一 Arbitration Engine 產生。
- Best Trade Zone 不再以 null 表示。
- Immediate Trigger 與 Structural Decision Zone 分離。
- Confidence 可解釋且因子完整。
- 模型能力不足時自動降級為 Ranking，不宣稱高可信機率。
- 每個版本皆可透過固定 Fixture 與 Walk-Forward 驗證。
- 每筆分析皆可依 model hash、pipeline version、input hash 重現。
