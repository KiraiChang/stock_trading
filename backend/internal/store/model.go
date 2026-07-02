package store

import (
	"database/sql"
	"time"
)

type Candle struct {
	ID        uint64    `db:"id"        json:"id"`
	Symbol    string    `db:"symbol"    json:"symbol"`
	Timeframe string    `db:"timeframe" json:"timeframe"`
	Open      float64   `db:"open"      json:"open"`
	High      float64   `db:"high"      json:"high"`
	Low       float64   `db:"low"       json:"low"`
	Close     float64   `db:"close"     json:"close"`
	Volume    int64     `db:"volume"    json:"volume"`
	Amount    float64   `db:"amount"    json:"amount"`
	Timestamp time.Time `db:"ts"        json:"ts"`
}

type IndicatorSnapshot struct {
	ID         uint64    `db:"id"          json:"id"`
	Symbol     string    `db:"symbol"      json:"symbol"`
	Timeframe  string    `db:"timeframe"   json:"timeframe"`
	Timestamp  time.Time `db:"ts"          json:"ts"`
	MA5        float64   `db:"ma5"         json:"ma5"`
	MA10       float64   `db:"ma10"        json:"ma10"`
	MA20       float64   `db:"ma20"        json:"ma20"`
	MA60       float64   `db:"ma60"        json:"ma60"`
	RSI14      float64   `db:"rsi14"       json:"rsi14"`
	MACD       float64   `db:"macd"        json:"macd"`
	MACDSignal float64   `db:"macd_signal" json:"macd_signal"`
	MACDHist   float64   `db:"macd_hist"   json:"macd_hist"`
	BBUpper    float64   `db:"bb_upper"    json:"bb_upper"`
	BBMiddle   float64   `db:"bb_middle"   json:"bb_middle"`
	BBLower    float64   `db:"bb_lower"    json:"bb_lower"`
	ATR14      float64   `db:"atr14"       json:"atr14"`
	VWAP       float64   `db:"vwap"        json:"vwap"`
	VolMA20    int64     `db:"vol_ma20"    json:"vol_ma20"`
	VolRatio   float64   `db:"vol_ratio"   json:"vol_ratio"`
}

type Signal struct {
	ID         uint64    `db:"id"          json:"id"`
	Symbol     string    `db:"symbol"      json:"symbol"`
	SignalType string    `db:"signal_type" json:"signal_type"`
	Direction  string    `db:"direction"   json:"direction"`
	Price      float64   `db:"price"       json:"price"`
	Volume     int64     `db:"volume"      json:"volume"`
	VolRatio   float64   `db:"vol_ratio"   json:"vol_ratio"`
	Resistance float64   `db:"resistance"  json:"resistance"`
	Support    float64   `db:"support"     json:"support"`
	Trend      string    `db:"trend"       json:"trend"`
	Note       string    `db:"note"        json:"note"`
	Timestamp  time.Time `db:"ts"          json:"ts"`
}

type JobRun struct {
	ID            uint64         `db:"id"             json:"id"`
	JobName       string         `db:"job_name"       json:"job_name"`
	Status        string         `db:"status"         json:"status"` // running/success/partial/failed
	SymbolsTotal  int            `db:"symbols_total"  json:"symbols_total"`
	SymbolsFailed int            `db:"symbols_failed" json:"symbols_failed"`
	Error         sql.NullString `db:"error"        json:"error,omitempty"`
	StartedAt     time.Time      `db:"started_at"     json:"started_at"`
	FinishedAt    sql.NullTime   `db:"finished_at"    json:"finished_at,omitempty"`
}

// ── Stock Analysis models ─────────────────────────────────────

type StockAnalysis struct {
	ID                   uint64      `db:"id"                      json:"id"`
	Symbol               string      `db:"symbol"                  json:"symbol"`
	Timeframe            string      `db:"timeframe"               json:"timeframe"`
	AnalyzedAt           time.Time   `db:"analyzed_at"             json:"analyzed_at"`
	CurrentPrice         float64     `db:"current_price"           json:"current_price"`
	Trend                string      `db:"trend"                   json:"trend"`
	EntryStatus          string      `db:"entry_status"            json:"entry_status"` // ACTIVE / WATCHING
	EntryDirection       string      `db:"entry_direction"         json:"entry_direction"`
	EntryPrice           float64     `db:"entry_price"             json:"entry_price"`
	EntryReason          NullString  `db:"entry_reason"            json:"entry_reason,omitempty"`
	StopLossATR          NullFloat64 `db:"stop_loss_atr"           json:"stop_loss_atr,omitempty"`
	StopLossStructural   NullFloat64 `db:"stop_loss_structural"    json:"stop_loss_structural,omitempty"`
	StopLossComposite    NullFloat64 `db:"stop_loss_composite"     json:"stop_loss_composite,omitempty"`
	TakeProfitNextLevel  NullFloat64 `db:"take_profit_next_level"  json:"take_profit_next_level,omitempty"`
	TakeProfitRiskReward NullFloat64 `db:"take_profit_risk_reward" json:"take_profit_risk_reward,omitempty"`
	TakeProfitATR        NullFloat64 `db:"take_profit_atr"         json:"take_profit_atr,omitempty"`
	// TradeVerification 為 JSON 字串：每個停損/停利方法各自「有沒有被觸及、
	// 何時、什麼價位」，見 internal/analysis/verifier.go
	TradeVerification NullString `db:"trade_verification" json:"trade_verification,omitempty"`
	VerifiedAt        NullTime   `db:"verified_at"        json:"verified_at,omitempty"`
	CreatedAt         time.Time  `db:"created_at"         json:"created_at"`
}

type StockAnalysisLevel struct {
	ID          uint64      `db:"id"           json:"id"`
	AnalysisID  uint64      `db:"analysis_id"  json:"analysis_id"`
	Price       float64     `db:"price"        json:"price"`
	Type        string      `db:"type"         json:"type"` // SUPPORT / RESISTANCE
	Strength    float64     `db:"strength"     json:"strength"`
	Method      string      `db:"method"       json:"method"`
	Status      string      `db:"status"       json:"status"` // PENDING / HELD_SO_FAR / BROKEN
	BrokenAt    NullTime    `db:"broken_at"    json:"broken_at,omitempty"`
	BrokenPrice NullFloat64 `db:"broken_price" json:"broken_price,omitempty"`
}

// ── SR Zone Scoring models（機構級版本，見 Python
// backtest/modular/sr_scoring/scoring.py 開頭的完整說明）────────────────

type SRZoneAnalysis struct {
	ID           uint64    `db:"id"                    json:"id"`
	Symbol       string    `db:"symbol"                json:"symbol"`
	Timeframe    string    `db:"timeframe"             json:"timeframe"`
	AnalyzedAt   time.Time `db:"analyzed_at"           json:"analyzed_at"`
	CurrentPrice float64   `db:"current_price"         json:"current_price"`
	// 只有一個 Global Model：這五個欄位是這次分析唯一、權威的整體評估
	// 區塊，存在這裡（分析快照）而不是每個 zone 各存一份，避免重複資訊。
	// GlobalTrend/GlobalVolatility 是股票層級的量（同一次分析裡所有 zone
	// 共用同一個值）；GlobalExpectedValue/GlobalRiskRewardRatio 是所有
	// zone 依 confidence 加權平均後「唯一收斂」的結果（見
	// scoring.py::_compute_global_metrics）；GlobalConfidence 是所有 zone
	// confidence 的簡單平均。zones 為空、或都沒有明確方向時，
	// GlobalExpectedValue/GlobalConfidence/GlobalRiskRewardRatio 可能是 NULL。
	GlobalTrend           float64     `db:"global_trend"            json:"global_trend"`
	GlobalVolatility      float64     `db:"global_volatility"       json:"global_volatility"`
	GlobalExpectedValue   NullFloat64 `db:"global_expected_value"   json:"global_expected_value,omitempty"`
	GlobalConfidence      NullFloat64 `db:"global_confidence"       json:"global_confidence,omitempty"`
	GlobalRiskRewardRatio NullFloat64 `db:"global_risk_reward_ratio" json:"global_risk_reward_ratio,omitempty"`
	ModelVersion          string      `db:"model_version"           json:"model_version"`
	CreatedAt             time.Time   `db:"created_at"              json:"created_at"`
}

type SRZone struct {
	ID         uint64  `db:"id"                      json:"id"`
	AnalysisID uint64  `db:"analysis_id"             json:"analysis_id"`
	PriceLow   float64 `db:"price_low"               json:"price_low"`
	PriceHigh  float64 `db:"price_high"              json:"price_high"`
	Method     string  `db:"method"                  json:"method"`
	Role       string  `db:"role"                    json:"role"` // SUPPORT / RESISTANCE / AT_ZONE

	// Tier/TierLabel：zone 依寬度在同一次分析裡的相對排名分三層
	// （TIER_1_MAIN_STRUCTURE 主結構 / TIER_2_TRADING_ZONE 交易區 /
	// TIER_3_SHORT_TERM 短期支撐），讓 zone 清單可排序（見
	// scoring.py::_assign_tiers）。
	Tier      string `db:"tier"                    json:"tier"`
	TierLabel string `db:"tier_label"              json:"tier_label"`

	SupportScore    float64 `db:"support_score"           json:"support_score"`
	ResistanceScore float64 `db:"resistance_score"        json:"resistance_score"`
	// NetScore = SupportScore - ResistanceScore，NetScoreLabel 是分類結果
	// （STRONG_SUPPORT/NEUTRAL/STRONG_RESISTANCE），避免只看單一分數判斷。
	NetScore      float64 `db:"net_score"               json:"net_score"`
	NetScoreLabel string  `db:"net_score_label"         json:"net_score_label"`

	// Confidence 綜合樣本數、時間衰減（含最近驗證）、歷史結果穩定度三個
	// 因子（0~1），ConfidenceLevel 是分級結果（LOW/MEDIUM/HIGH/VERY_HIGH）。
	Confidence      float64 `db:"confidence"              json:"confidence"`
	ConfidenceLevel string  `db:"confidence_level"        json:"confidence_level"`

	BounceProbability NullFloat64 `db:"bounce_probability"      json:"bounce_probability,omitempty"`
	BreakProbability  NullFloat64 `db:"break_probability"       json:"break_probability,omitempty"`
	// ExpectedGain/ExpectedLoss 分別是這個 zone 角色解析後的
	// average_bounce_return/average_break_return；ExpectedValue = 反彈機率
	// × ExpectedGain + 跌破機率 × ExpectedLoss（不再用單一 average_return，
	// 見一、修正 EV 計算方式）。RiskRewardRatio = |ExpectedGain/ExpectedLoss|。
	// RewardRiskPercentile 是這個 RiskRewardRatio 在訓練資料集歷史分佈中的
	// 百分位。以上皆只有 Role 為 SUPPORT/RESISTANCE 時才有值。
	ExpectedGain         NullFloat64 `db:"expected_gain"           json:"expected_gain,omitempty"`
	ExpectedLoss         NullFloat64 `db:"expected_loss"           json:"expected_loss,omitempty"`
	ExpectedValue        NullFloat64 `db:"expected_value"          json:"expected_value,omitempty"`
	RiskRewardRatio      NullFloat64 `db:"risk_reward_ratio"       json:"risk_reward_ratio,omitempty"`
	RewardRiskPercentile NullFloat64 `db:"reward_risk_percentile"  json:"reward_risk_percentile,omitempty"`

	// RelativeVolume/VolumeConfirmation 為角色解析後的值，只有 Role 為
	// SUPPORT/RESISTANCE 時才有值。
	RelativeVolume     NullFloat64 `db:"relative_volume"         json:"relative_volume,omitempty"`
	VolumeConfirmation NullString  `db:"volume_confirmation"     json:"volume_confirmation,omitempty"`

	TouchCount  int `db:"touch_count"             json:"touch_count"` // 聚合值，不分方向
	RejectCount int `db:"reject_count"             json:"reject_count"`
	BreakCount  int `db:"break_count"              json:"break_count"`

	// ZoneMomentum/ZoneDirection 是這個 zone 自己的歷史觸碰動能（不是股票
	// 層級的 trend，同一次分析裡不同 zone 會有不同值）。
	ZoneMomentum  float64 `db:"zone_momentum"           json:"zone_momentum"`
	ZoneDirection string  `db:"zone_direction"          json:"zone_direction"`

	RecentValidation string `db:"recent_validation"       json:"recent_validation"`

	TradingScore float64 `db:"trading_score"           json:"trading_score"`
	// TradingScoreBreakdown 是 JSON 字串（{"expected_value":.., "risk_reward":..,
	// "trend":.., "volume":.., "confidence":..}，五個分量的加權貢獻值加總
	// 即為 TradingScore），讓分數「可拆解」（見十三、Score 必須可拆解）。
	TradingScoreBreakdown string `db:"trading_score_breakdown" json:"trading_score_breakdown"`
	TradingRecommendation string `db:"trading_recommendation"  json:"trading_recommendation"`

	Status      string      `db:"status"                  json:"status"` // PENDING / HELD_SO_FAR / BROKEN（保留供未來 verifier 使用）
	BrokenAt    NullTime    `db:"broken_at"               json:"broken_at,omitempty"`
	BrokenPrice NullFloat64 `db:"broken_price"            json:"broken_price,omitempty"`
}

type WatchlistItem struct {
	ID     uint32 `db:"id"     json:"id"`
	Symbol string `db:"symbol" json:"symbol"`
	Name   string `db:"name"   json:"name"`
	Sector string `db:"sector" json:"sector"`
	// Watched 標示是否要透過 WebSocket 即時監聽，最多同時 MaxWatchedSymbols 檔
	// （見 watchlist_repo.go），跟監控清單本身的大小無關
	Watched bool `db:"watched" json:"watched"`
}

// ── Backtest models ───────────────────────────────────────────

type BacktestJob struct {
	ID         uint64       `db:"id"          json:"id"`
	JobID      string       `db:"job_id"      json:"job_id"`
	Type       string       `db:"type"        json:"type"`
	Strategy   string       `db:"strategy"    json:"strategy"`
	Symbols    string       `db:"symbols"     json:"symbols"` // JSON array string
	Timeframe  string       `db:"timeframe"   json:"timeframe"`
	StartDate  string       `db:"start_date"  json:"start_date"`
	EndDate    string       `db:"end_date"    json:"end_date"`
	Status     string       `db:"status"      json:"status"`  // pending/running/done/failed
	Trigger    string       `db:"trigger"     json:"trigger"` // manual/scheduler
	Error      string       `db:"error"       json:"error,omitempty"`
	CreatedAt  time.Time    `db:"created_at"  json:"created_at"`
	StartedAt  sql.NullTime `db:"started_at"  json:"started_at,omitempty"`
	FinishedAt sql.NullTime `db:"finished_at" json:"finished_at,omitempty"`
}

type BacktestResult struct {
	ID           uint64    `db:"id"            json:"id"`
	JobID        string    `db:"job_id"        json:"job_id"`
	Strategy     string    `db:"strategy"      json:"strategy"`
	TotalReturn  float64   `db:"total_return"  json:"total_return"`
	AnnualReturn float64   `db:"annual_return" json:"annual_return"`
	WinRate      float64   `db:"win_rate"      json:"win_rate"`
	MaxDrawdown  float64   `db:"max_drawdown"  json:"max_drawdown"`
	SharpeRatio  float64   `db:"sharpe_ratio"  json:"sharpe_ratio"`
	TotalTrades  int       `db:"total_trades"  json:"total_trades"`
	WinTrades    int       `db:"win_trades"    json:"win_trades"`
	LossTrades   int       `db:"loss_trades"   json:"loss_trades"`
	AvgPnL       float64   `db:"avg_pnl"       json:"avg_pnl"`
	CreatedAt    time.Time `db:"created_at"    json:"created_at"`
}

type BacktestTrade struct {
	ID         uint64       `db:"id"          json:"id"`
	JobID      string       `db:"job_id"      json:"job_id"`
	Symbol     string       `db:"symbol"      json:"symbol"`
	Direction  string       `db:"direction"   json:"direction"`
	EntryTime  sql.NullTime `db:"entry_time"  json:"entry_time,omitempty"`
	ExitTime   sql.NullTime `db:"exit_time"   json:"exit_time,omitempty"`
	EntryPrice float64      `db:"entry_price" json:"entry_price"`
	ExitPrice  float64      `db:"exit_price"  json:"exit_price"`
	Size       float64      `db:"size"        json:"size"`
	PnL        float64      `db:"pnl"         json:"pnl"`
	PnLPct     float64      `db:"pnl_pct"     json:"pnl_pct"`
	Commission float64      `db:"commission"  json:"commission"`
	CreatedAt  time.Time    `db:"created_at"  json:"created_at"`
}
