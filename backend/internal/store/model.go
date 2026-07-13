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
	ID         uint64  `db:"id"          json:"id"`
	Symbol     string  `db:"symbol"      json:"symbol"`
	SignalType string  `db:"signal_type" json:"signal_type"`
	Direction  string  `db:"direction"   json:"direction"`
	Price      float64 `db:"price"       json:"price"`
	Volume     int64   `db:"volume"      json:"volume"`
	VolRatio   float64 `db:"vol_ratio"   json:"vol_ratio"`
	Resistance float64 `db:"resistance"  json:"resistance"`
	Support    float64 `db:"support"     json:"support"`
	Trend      string  `db:"trend"       json:"trend"`
	Note       string  `db:"note"        json:"note"`
	// Strength（預設 1.0，代表未受籌碼調整的原始強度；籌碼加權後可能上修
	// 或下修，例如 0.6~1.3，不是機率、不強制限制在 [0,1]）與 ChipSignal 是
	// 【籌碼分析整合】新增欄位：Engine.Evaluate 依 chip_scores 的訊號調整
	// 原始訊號強度時寫入，讓「這個訊號被籌碼加權過」變成可結構化查詢的
	// 資料，不用從 Note 自由文字解析。ChipSignal 為空字串代表評估當下查無
	// 籌碼資料，Strength 維持預設值。
	Strength   float64    `db:"strength"    json:"strength"`
	ChipSignal NullString `db:"chip_signal" json:"chip_signal,omitempty"`
	Timestamp  time.Time  `db:"ts"          json:"ts"`
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
	// confidence 的簡單平均。GlobalExpectedValue/GlobalRiskRewardRatio 在
	// zones 為空或都沒有明確方向時可能是 NULL；GlobalConfidence 只有
	// zones 為空時才是 NULL。
	GlobalTrend           float64     `db:"global_trend"            json:"global_trend"`
	GlobalVolatility      float64     `db:"global_volatility"       json:"global_volatility"`
	GlobalExpectedValue   NullFloat64 `db:"global_expected_value"   json:"global_expected_value,omitempty"`
	GlobalConfidence      NullFloat64 `db:"global_confidence"       json:"global_confidence,omitempty"`
	GlobalRiskRewardRatio NullFloat64 `db:"global_risk_reward_ratio" json:"global_risk_reward_ratio,omitempty"`
	ModelVersion          string      `db:"model_version"           json:"model_version"`
	// ModelConfigHash 是訓練這個模型時的 DatasetConfig/zone builder 參數/
	// model_type/calibration_method 快照的短 hash（見 Python model.py::
	// compute_config_hash）。比 ModelVersion 更細：同一個 ModelVersion 底下
	// 換過幾次訓練參數都可能有不同的 hash，讓「這筆分析用哪組訓練設定產生」
	// 可以事後追溯，重訓改參數後舊分析可被這個值辨識出來。舊模型檔沒有這個
	// 欄位時 Python 端回傳空字串，這裡直接存空字串（不像 ModelVersion 那樣
	// 防禦性地填 "unknown"，因為空字串已經足夠表示「沒有這項資訊」）。
	ModelConfigHash    string  `db:"model_config_hash"       json:"model_config_hash"`
	PipelineVersion    string  `db:"pipeline_version"        json:"pipeline_version"`
	Evidence           RawJSON `db:"evidence"                json:"evidence"`
	Explanation        RawJSON `db:"explanation"             json:"explanation"`
	Scenario           RawJSON `db:"scenario"                json:"scenario"`
	ProbabilityContext RawJSON `db:"probability_context"     json:"probability_context"`
	// PeriodSummaries 是 Python 端整理好的短/中/長期支撐壓力摘要 JSON；
	// AnalysisTips 是前端輪播的白話解讀提示。兩者保存於 analysis 快照，
	// 讓歷史分析不需要重新呼叫 Python 也能顯示同一份結論。
	PeriodSummaries RawJSON `db:"period_summaries"       json:"period_summaries"`
	AnalysisTips    RawJSON `db:"analysis_tips"          json:"analysis_tips"`
	// ChipSummary 是整檔層級的籌碼拆解 JSON（總分/訊號/四子分數/無資料旗標，
	// 見 Python _build_chip_summary），供前端「共用籌碼面板」顯示。跟 zone 無關、
	// 整份分析共用一份，所以存在 analysis 快照而不是每個 zone。查無籌碼資料時
	// 為 {"missing":true,...}；舊資料沒有這欄時為 JSON null。
	ChipSummary     RawJSON   `db:"chip_summary"           json:"chip_summary"`
	DecisionSummary RawJSON   `db:"decision_summary"       json:"decision_summary"`
	CreatedAt       time.Time `db:"created_at"              json:"created_at"`
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

	TouchCount int `db:"touch_count" json:"touch_count"` // 兩個方向加總（zone 整體活躍度）
	// SupportTouchCount/ResistanceTouchCount 分開統計兩個方向各自的觸碰次數，
	// 兩者相加等於 TouchCount。Confidence 依 Role 只用其中一個方向的樣本數/
	// 穩定度計算（不會被另一個方向稀釋或拉抬），見 Python scoring.py::
	// score_zone 的說明。
	SupportTouchCount    int `db:"support_touch_count"    json:"support_touch_count"`
	ResistanceTouchCount int `db:"resistance_touch_count" json:"resistance_touch_count"`
	RejectCount          int `db:"reject_count"           json:"reject_count"`
	BreakCount           int `db:"break_count"            json:"break_count"`

	// ZoneMomentum/ZoneDirection 是這個 zone 自己的歷史觸碰動能（不是股票
	// 層級的 trend，同一次分析裡不同 zone 會有不同值）。
	ZoneMomentum  float64 `db:"zone_momentum"           json:"zone_momentum"`
	ZoneDirection string  `db:"zone_direction"          json:"zone_direction"`

	RecentValidation string `db:"recent_validation"       json:"recent_validation"`

	TradingScore float64 `db:"trading_score"           json:"trading_score"`
	// TradingScoreBreakdown 是 RawJSON（見 null.go 說明）：{"expected_value":..,
	// "risk_reward":.., "trend":.., "volume":.., "confidence":.., "chip":..}，
	// 六個分量的加權貢獻值加總即為 TradingScore（見十三、Score 必須可拆解）。
	TradingScoreBreakdown RawJSON `db:"trading_score_breakdown" json:"trading_score_breakdown"`
	TradingRecommendation string  `db:"trading_recommendation"  json:"trading_recommendation"`

	// OverlapGroup/ConfluenceCount：跨方法（ATR/volume_profile）重疊的 zone
	// 分群（見 Python scoring.py::_group_overlapping_zones）。不合併/刪除
	// 任何 zone，只標記供 UI 顯示「多方法共振」。ConfluenceCount 恆 >= 1；
	// OverlapGroup 只有 ConfluenceCount > 1 時才有值。
	OverlapGroup    NullInt64 `db:"overlap_group"    json:"overlap_group,omitempty"`
	ConfluenceCount int       `db:"confluence_count" json:"confluence_count"`

	Status      string      `db:"status"                  json:"status"` // PENDING / HELD_SO_FAR / BROKEN（由 analysis.SRZoneVerifier 更新，見 sr-zone-scoring.md「十四」）
	BrokenAt    NullTime    `db:"broken_at"               json:"broken_at,omitempty"`
	BrokenPrice NullFloat64 `db:"broken_price"            json:"broken_price,omitempty"`

	// ResolvedRole 只有 Role=AT_ZONE 的 zone 在後續驗證時，價格真正收盤離開
	// 區間後才會被解析並寫入（SUPPORT 或 RESISTANCE）；Role != AT_ZONE 的
	// zone 永遠是 NULL（角色從分析當下就已明確，不需要另外解析）。不覆寫
	// 原始 Role，是為了保留「分析當下是 AT_ZONE」這個歷史資訊，同時讓
	// status/broken_at/broken_price 有明確對應的方向可以解釋，見
	// sr_zone_improve.md review #2。
	ResolvedRole       NullString `db:"resolved_role" json:"resolved_role,omitempty"`
	Features           RawJSON    `db:"features"      json:"features"`
	Evidence           RawJSON    `db:"evidence"      json:"evidence"`
	Explanation        RawJSON    `db:"explanation"   json:"explanation"`
	Scenario           RawJSON    `db:"scenario"      json:"scenario"`
	ProbabilityContext RawJSON    `db:"probability_context" json:"probability_context"`
}

// SRScoringTrainJob 追蹤一次「重新訓練 hold/break 機率模型」的背景任務
// （見 sr-zone-scoring.md「訓練任務可觀測化」）。訓練本身在 Go 背景 goroutine
// 呼叫 Python 同步執行，這張表讓前端可以查詢「現在跑到哪裡、成功了沒、
// metrics 是什麼」，不用只靠伺服器 log。
type SRScoringTrainJob struct {
	ID         uint64 `db:"id"          json:"id"`
	JobID      string `db:"job_id"      json:"job_id"`
	Status     string `db:"status"      json:"status"`  // pending / running / done / failed
	Symbols    string `db:"symbols"     json:"symbols"` // JSON array string
	Timeframe  string `db:"timeframe"   json:"timeframe"`
	FetchLimit int    `db:"fetch_limit" json:"fetch_limit"`
	ModelType  string `db:"model_type"  json:"model_type"`
	// Rows/Sources/Metrics/ModelPath/ModelVersion/DatasetSummary 只有
	// status=done 才有值；Error 只有 status=failed 才有值。
	Rows         NullInt64  `db:"rows"             json:"rows,omitempty"`
	Sources      NullInt64  `db:"sources"          json:"sources,omitempty"`
	Metrics      RawJSON    `db:"metrics"          json:"metrics,omitempty"`
	ModelPath    NullString `db:"model_path"       json:"model_path,omitempty"`
	ModelVersion NullString `db:"model_version"    json:"model_version,omitempty"`
	SplitMethod  NullString `db:"split_method"     json:"split_method,omitempty"`
	// DatasetSummary 是 summarize_training_dataset() 的原樣保存（見
	// sr-zone-scoring.md「模型驗證與校準」），供人工判斷這次訓練出來的模型
	// 可不可信，不影響任何業務邏輯。
	DatasetSummary RawJSON    `db:"dataset_summary"  json:"dataset_summary,omitempty"`
	Error          NullString `db:"error"            json:"error,omitempty"`
	StartedAt      NullTime   `db:"started_at"       json:"started_at,omitempty"`
	FinishedAt     NullTime   `db:"finished_at"      json:"finished_at,omitempty"`
	CreatedAt      time.Time  `db:"created_at"       json:"created_at"`
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
	ID        uint64 `db:"id"          json:"id"`
	JobID     string `db:"job_id"      json:"job_id"`
	Type      string `db:"type"        json:"type"`
	Strategy  string `db:"strategy"    json:"strategy"`
	Symbols   string `db:"symbols"     json:"symbols"` // JSON array string
	Timeframe string `db:"timeframe"   json:"timeframe"`
	StartDate string `db:"start_date"  json:"start_date"`
	EndDate   string `db:"end_date"    json:"end_date"`
	Status    string `db:"status"      json:"status"`  // pending/running/done/failed
	Trigger   string `db:"trigger"     json:"trigger"` // manual/scheduler
	Error     string `db:"error"       json:"error,omitempty"`
	// UseChipFilter/ChipMinScore：【籌碼分析整合】是否在進場時套用
	// chip_scores.total_score 門檻過濾（見 docs/chip-analysis-design.md 第9節），
	// Python 端逐 bar 比對，未達門檻的訊號不會進場。
	UseChipFilter bool         `db:"use_chip_filter" json:"use_chip_filter"`
	ChipMinScore  float64      `db:"chip_min_score"  json:"chip_min_score"`
	CreatedAt     time.Time    `db:"created_at"  json:"created_at"`
	StartedAt     sql.NullTime `db:"started_at"  json:"started_at,omitempty"`
	FinishedAt    sql.NullTime `db:"finished_at" json:"finished_at,omitempty"`
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

// ── Chip Analysis models（見 docs/chip-analysis-design.md）───────────

type InstitutionalTrade struct {
	ID                    uint64    `db:"id"                        json:"id"`
	Symbol                string    `db:"symbol"                    json:"symbol"`
	TradeDate             time.Time `db:"trade_date"                json:"trade_date"`
	ForeignNetBuy         int64     `db:"foreign_net_buy"           json:"foreign_net_buy"`
	InvestmentTrustNetBuy int64     `db:"investment_trust_net_buy"  json:"investment_trust_net_buy"`
	DealerNetBuy          int64     `db:"dealer_net_buy"            json:"dealer_net_buy"`
	TotalNetBuy           int64     `db:"total_net_buy"             json:"total_net_buy"`
	CreatedAt             time.Time `db:"created_at"                json:"created_at"`
	UpdatedAt             time.Time `db:"updated_at"                json:"updated_at"`
}

type MarginTrade struct {
	ID              uint64      `db:"id"                 json:"id"`
	Symbol          string      `db:"symbol"             json:"symbol"`
	TradeDate       time.Time   `db:"trade_date"         json:"trade_date"`
	MarginBalance   int64       `db:"margin_balance"     json:"margin_balance"`
	MarginChange    int64       `db:"margin_change"      json:"margin_change"`
	ShortBalance    int64       `db:"short_balance"      json:"short_balance"`
	ShortChange     int64       `db:"short_change"       json:"short_change"`
	MarginUsageRate NullFloat64 `db:"margin_usage_rate"  json:"margin_usage_rate,omitempty"`
	ShortUsageRate  NullFloat64 `db:"short_usage_rate"   json:"short_usage_rate,omitempty"`
	CreatedAt       time.Time   `db:"created_at"         json:"created_at"`
	UpdatedAt       time.Time   `db:"updated_at"         json:"updated_at"`
}

type BrokerTrade struct {
	ID         uint64    `db:"id"          json:"id"`
	Symbol     string    `db:"symbol"      json:"symbol"`
	TradeDate  time.Time `db:"trade_date"  json:"trade_date"`
	BrokerName string    `db:"broker_name" json:"broker_name"`
	BranchName string    `db:"branch_name" json:"branch_name"`
	BuyVolume  int64     `db:"buy_volume"  json:"buy_volume"`
	SellVolume int64     `db:"sell_volume" json:"sell_volume"`
	NetBuy     int64     `db:"net_buy"     json:"net_buy"`
	CreatedAt  time.Time `db:"created_at"  json:"created_at"`
}

// ChipScore 是每日籌碼分析結果快照（見 internal/chip 套件的計分邏輯）。
// Reason 用 RawJSON（純 string，非 sql.Null* 包裝）讀寫，DB 欄位 NOT NULL
// DEFAULT '[]'，避免 RawJSON.Scan 遇到 SQL NULL 出錯。
type ChipScore struct {
	ID                 uint64    `db:"id"                   json:"id"`
	Symbol             string    `db:"symbol"               json:"symbol"`
	TradeDate          time.Time `db:"trade_date"           json:"trade_date"`
	InstitutionalScore float64   `db:"institutional_score"  json:"institutional_score"`
	MarginScore        float64   `db:"margin_score"         json:"margin_score"`
	BrokerScore        float64   `db:"broker_score"         json:"broker_score"`
	ConcentrationScore float64   `db:"concentration_score"  json:"concentration_score"`
	TotalScore         float64   `db:"total_score"          json:"total_score"`
	Signal             string    `db:"signal"               json:"signal"` // BULLISH/BEARISH/NEUTRAL/RISK
	Reason             RawJSON   `db:"reason"               json:"reason,omitempty"`
	CreatedAt          time.Time `db:"created_at"           json:"created_at"`
	UpdatedAt          time.Time `db:"updated_at"           json:"updated_at"`
}

// ChipSyncJob 追蹤一次 manual / backfill 籌碼資料同步任務（daily 模式沿用
// 既有 job_runs 表，job_name="chip_daily_sync"，見 scheduler.go）。Failures
// 用 RawJSON 讀寫，DB 欄位 NOT NULL DEFAULT '[]'，理由同 ChipScore.Reason。
type ChipSyncJob struct {
	ID            uint64     `db:"id"             json:"id"`
	JobID         string     `db:"job_id"         json:"job_id"`
	Mode          string     `db:"mode"           json:"mode"`       // manual / backfill
	Symbols       string     `db:"symbols"        json:"symbols"`    // JSON array string
	DataTypes     string     `db:"data_types"     json:"data_types"` // JSON array string
	FromDate      string     `db:"from_date"      json:"from_date"`
	ToDate        string     `db:"to_date"        json:"to_date"`
	Force         bool       `db:"force"          json:"force"`
	Status        string     `db:"status"         json:"status"` // pending/running/done/partial/failed
	SymbolsTotal  int        `db:"symbols_total"  json:"symbols_total"`
	SymbolsDone   int        `db:"symbols_done"   json:"symbols_done"`
	SymbolsFailed int        `db:"symbols_failed" json:"symbols_failed"`
	Failures      RawJSON    `db:"failures"       json:"failures,omitempty"`
	Error         NullString `db:"error"          json:"error,omitempty"`
	StartedAt     NullTime   `db:"started_at"     json:"started_at,omitempty"`
	FinishedAt    NullTime   `db:"finished_at"    json:"finished_at,omitempty"`
	CreatedAt     time.Time  `db:"created_at"     json:"created_at"`
}

// ── Holdings / Portfolio Analysis models ─────────────────────

// Position is the materialized AVG-cost projection rebuilt transactionally
// from immutable PositionTransaction events.
type Position struct {
	Symbol      string    `db:"symbol"       json:"symbol"`
	Shares      float64   `db:"shares"       json:"shares"`
	AvgCost     float64   `db:"avg_cost"     json:"avg_cost"`
	RealizedPnL float64   `db:"realized_pnl" json:"realized_pnl"`
	Version     int64     `db:"version"      json:"version"`
	LastEventID uint64    `db:"last_event_id" json:"last_event_id"`
	UpdatedAt   time.Time `db:"updated_at"   json:"updated_at"`
}

type PositionTransaction struct {
	ID            uint64      `db:"id"              json:"id"`
	Symbol        string      `db:"symbol"          json:"symbol"`
	EventType     string      `db:"event_type"      json:"event_type"`
	OccurredAt    time.Time   `db:"occurred_at"     json:"occurred_at"`
	Shares        NullFloat64 `db:"shares"          json:"shares,omitempty"`
	Price         NullFloat64 `db:"price"           json:"price,omitempty"`
	Fee           float64     `db:"fee"             json:"fee"`
	Tax           float64     `db:"tax"             json:"tax"`
	TargetShares  NullFloat64 `db:"target_shares"   json:"target_shares,omitempty"`
	TargetAvgCost NullFloat64 `db:"target_avg_cost" json:"target_avg_cost,omitempty"`
	Note          string      `db:"note"            json:"note"`
	CreatedAt     time.Time   `db:"created_at"      json:"created_at"`
}

type PositionAnalysis struct {
	ID                     uint64      `db:"id"                     json:"id"`
	Symbol                 string      `db:"symbol"                 json:"symbol"`
	PositionState          string      `db:"position_state"         json:"position_state"`
	PositionVersion        int64       `db:"position_version"       json:"position_version"`
	Shares                 float64     `db:"shares"                 json:"shares"`
	AvgCost                float64     `db:"avg_cost"               json:"avg_cost"`
	RealizedPnL            float64     `db:"realized_pnl"           json:"realized_pnl"`
	AnalyzedAt             time.Time   `db:"analyzed_at"            json:"analyzed_at"`
	CurrentPrice           float64     `db:"current_price"          json:"current_price"`
	SRZoneAnalysisID       NullInt64   `db:"sr_zone_analysis_id"    json:"sr_zone_analysis_id,omitempty"`
	Action                 string      `db:"action"                 json:"action"`
	ActionLabel            string      `db:"action_label"           json:"action_label"`
	TargetShares           float64     `db:"target_shares"          json:"target_shares"`
	AdjustmentShares       float64     `db:"adjustment_shares"      json:"adjustment_shares"`
	AdjustmentSide         string      `db:"adjustment_side"        json:"adjustment_side"`
	AdjustmentAmount       float64     `db:"adjustment_amount"      json:"adjustment_amount"`
	EntryPrice             NullFloat64 `db:"entry_price"            json:"entry_price,omitempty"`
	StopLossPrice          NullFloat64 `db:"stop_loss_price"        json:"stop_loss_price,omitempty"`
	TakeProfitPrice        NullFloat64 `db:"take_profit_price"      json:"take_profit_price,omitempty"`
	RiskAmount             NullFloat64 `db:"risk_amount"            json:"risk_amount,omitempty"`
	ExpectedRewardAmount   NullFloat64 `db:"expected_reward_amount" json:"expected_reward_amount,omitempty"`
	RiskRewardRatio        NullFloat64 `db:"risk_reward_ratio"      json:"risk_reward_ratio,omitempty"`
	UnrealizedPnL          float64     `db:"unrealized_pnl"         json:"unrealized_pnl"`
	UnrealizedPnLPct       float64     `db:"unrealized_pnl_pct"     json:"unrealized_pnl_pct"`
	ConfigJSON             RawJSON     `db:"config_json"            json:"config"`
	Reason                 RawJSON     `db:"reason"                 json:"reason"`
	Evidence               RawJSON     `db:"evidence"               json:"evidence"`
	TriggerConditions      RawJSON     `db:"trigger_conditions"     json:"trigger_conditions"`
	InvalidationConditions RawJSON     `db:"invalidation_conditions" json:"invalidation_conditions"`
	RuleVersion            string      `db:"rule_version"           json:"rule_version"`
	CreatedAt              time.Time   `db:"created_at"             json:"created_at"`
}
