重構目前 Trading Decision Engine，暫時不要修改 Zone Score、Zone Ranking 或 Feature Weight。

只完成以下四項修改：

1. Market Regime decomposition
   將目前單一 regime 拆成：
- TrendRegime
- StructureState
- VolatilityState

StructureState 必須支援：
NORMAL
RECOVERY_CANDIDATE
RECOVERY
RECOVERY_INVALIDATED
BREAKDOWN

長期趨勢不得覆蓋短期結構破壞。
例如 long-term bullish + recovery invalidated 應輸出：
「長期偏多，但短線結構轉弱」。

2. Decision Hard Risk Gate
   Decision pipeline 必須固定為：

Regime
→ Structure
→ Risk Gate
→ EV
→ Score
→ Action

新增 RR hard gate：
RR < 1.5 => WATCH
RR >= 1.5 才允許 BUY_SMALL
RR >= 2.0 才允許 BUY

Score 與 EV 不得覆蓋 hard risk gate。

3. Zone Interaction State
   新增 ZoneInteraction：

DistancePct
Touched
PenetrationPct
ClosedInside
ClosedAbove
ClosedBelow

Zone 判斷必須同時使用 candle high/low/close，
不能只使用 current price 計算 distance。

UI 必須能顯示：
- 尚未測試
- 今日已測試
- 進入區間
- 收回區間上方
- 有效跌破

4. Separate Market Action and Position Action
   Decision output 拆成：

MarketAction
PositionAction

MarketAction：
WATCH
BUY_SMALL
BUY
AVOID

PositionAction：
HOLD
REDUCE_ON_BREAKDOWN
REDUCE
EXIT

市場訊號不得直接作為既有持倉操作建議。

不要修改現有 Zone Engine scoring。
不要重新調整 feature weights。
新增 unit tests 驗證上述 decision precedence 與 state transition。