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

	// AdjFactor 是這根 K 棒的累積還原係數（見 docs/database-schema.md 的「股價還原」）。
	// open/high/low/close 一律是**原始成交價**，要還原價請用 AdjustedClose() 等方法。
	// 沒跑過重算的資料為 1，語意就是「未調整」，不存在中間狀態。
	AdjFactor float64 `db:"adj_factor" json:"adj_factor"`

	// VolFactor 是**成交量**的累積係數，與 AdjFactor 分開（T-042 Phase 2）。
	// 現金股利讓價格下修但股數沒變，成交量不能跟著調整；分割與配股才會改變股數。
	VolFactor float64 `db:"vol_factor" json:"vol_factor"`
}

// AdjustedOpen/High/Low/Close 回傳還原價：價乘以 AdjFactor。
// AdjustedVolume 用的是 **VolFactor**，不是 AdjFactor。
//
// 為什麼價乘、量除：分割讓股數變多，所以歷史價要縮小、歷史量要放大，方向相反。
//
// **為什麼價與量用不同的係數**（T-042 Phase 2）：現金股利讓價格下修，但股數沒有改變，
// 所以成交量不可以跟著調整。只有分割與配股會改變股數。因此
//
//	AdjustedClose() * AdjustedVolume() == Close * Volume
//
// **只在 AdjFactor == VolFactor 時成立**（純股數事件）。現金股利發生時錢真的離開公司，
// 乘積本來就該變小——這不是 bug。
//
// 係數為 0（欄位沒被 SELECT 出來）時一律當作 1，避免把價格算成 0——
// 「漏 select」不該表現成「這檔股票不值錢」。
func factorOrOne(v float64) float64 {
	if v <= 0 {
		return 1
	}
	return v
}

func (c Candle) adjFactorOrOne() float64 { return factorOrOne(c.AdjFactor) }

// volFactorOrOne：VolFactor 未被 select 或尚未寫入（Phase 1 的舊資料）時，
// 退回 AdjFactor——Phase 1 只有分割，那時價量本來就共用一個係數。
func (c Candle) volFactorOrOne() float64 {
	if c.VolFactor > 0 {
		return c.VolFactor
	}
	return c.adjFactorOrOne()
}

func (c Candle) AdjustedOpen() float64   { return c.Open * c.adjFactorOrOne() }
func (c Candle) AdjustedHigh() float64   { return c.High * c.adjFactorOrOne() }
func (c Candle) AdjustedLow() float64    { return c.Low * c.adjFactorOrOne() }
func (c Candle) AdjustedClose() float64  { return c.Close * c.adjFactorOrOne() }
func (c Candle) AdjustedVolume() float64 { return float64(c.Volume) / c.volFactorOrOne() }

// 公司行動的類型。
//
// SPLIT/DIVIDEND 是我們自己的分類；`TaiwanStockSplitPrice` 回傳的 type 是中文原文
// （分割／反分割／面額變更），直接照存，因為它們是**不同的事件**，只是恰好都用
// after/before 當調整係數。硬塞成同一個標籤之後就分不出來了。
const (
	CorporateActionSplit    = "SPLIT"
	CorporateActionDividend = "DIVIDEND"
	// CorporateActionUnknown 給來源沒有標 type 的事件（實測有一筆 00631L）。
	// 前後價有效就照算係數——沒有標籤不代表事件沒發生。
	CorporateActionUnknown = "UNKNOWN"

	// 除權息（T-042 Phase 2）。分成三種是因為**對股數的影響不同**：
	// 純現金不改變股數（volume_factor = 1），配股會改變。
	CorporateActionDividendCash  = "DIVIDEND_CASH"
	CorporateActionDividendStock = "DIVIDEND_STOCK"
	CorporateActionDividendBoth  = "DIVIDEND_BOTH"

	// 減資（見 docs/issue.md I-069）。與反分割在數學上是同一件事：股數變少、價格變高，
	// 所以係數 > 1，且 volume_factor 等於價格係數。
	CorporateActionCapitalReduction = "CAPITAL_REDUCTION"
)

// 公司行動的資料來源。**定義在這裡而不是散在各 client**，是為了讓測試能拿到
// 與正式路徑完全相同的字串——這些名稱由外部服務決定，長度不受我們控制。
const (
	CorporateActionSourceSplit            = "TaiwanStockSplitPrice"
	CorporateActionSourceDividend         = "YahooDividendsByYear"
	CorporateActionSourceCapitalReduction = "TaiwanStockCapitalReductionReferencePrice"
)

// AllCorporateActionSources 匯出給測試用，理由同 AllCorporateActionTypes。
func AllCorporateActionSources() []string {
	return []string{
		CorporateActionSourceSplit,
		CorporateActionSourceDividend,
		CorporateActionSourceCapitalReduction,
	}
}

// AllCorporateActionTypes 匯出給測試用：DB 的 action_type 欄位必須容得下每一個值
// （`CAPITAL_REDUCTION` 是 17 字元，原本的 VARCHAR(16) 裝不下，見 migration 064）。
func AllCorporateActionTypes() []string {
	return []string{
		CorporateActionSplit, CorporateActionDividend, CorporateActionUnknown,
		CorporateActionDividendCash, CorporateActionDividendStock, CorporateActionDividendBoth,
		CorporateActionCapitalReduction,
		// 來源原文（FinMind 的 TaiwanStockSplitPrice 直接照存）
		"分割", "反分割", "面額變更",
	}
}

// CorporateAction 是一次公司行動。Factor = AfterPrice / BeforePrice，
// 由抓取端算好存起來，避免每個消費者各算一次。
type CorporateAction struct {
	ID          uint64    `db:"id"           json:"id"`
	Symbol      string    `db:"symbol"       json:"symbol"`
	EventDate   time.Time `db:"event_date"   json:"event_date"`
	ActionType  string    `db:"action_type"  json:"action_type"`
	BeforePrice float64   `db:"before_price" json:"before_price"`
	AfterPrice  float64   `db:"after_price"  json:"after_price"`
	Factor      float64   `db:"factor"       json:"factor"`
	// VolumeFactor 是這次事件對**股數**的影響：分割／配股會改變股數（等於價格係數或
	// 1/(1+配股率)），純現金股利不改變股數（為 1）。
	VolumeFactor float64 `db:"volume_factor" json:"volume_factor"`
	Source       string  `db:"source"        json:"source"`
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
	ChipSummary     RawJSON `db:"chip_summary"           json:"chip_summary"`
	DecisionSummary RawJSON `db:"decision_summary"       json:"decision_summary"`
	// ZoneBuilderRuntimeConfig 是這次分析實際採用的 zone builder 設定快照
	// （Python `_resolve_runtime_builders`）：adaptive 是否啟用、落在哪個波動
	// bucket、以及 reason_code（EXPLICIT_BUILDERS / ADAPTIVE_ZONE_BUILDERS_DISABLED /
	// ADAPTIVE_ZONE_BUILDERS_ERROR）。純紀錄用，不參與任何決策或狀態推導。
	// 057 migration 之前的舊資料為 JSON null，代表「沒有這項紀錄」——
	// 不等於「adaptive 未啟用」，前端要據此隱藏區塊而不是顯示成關閉。
	ZoneBuilderRuntimeConfig RawJSON   `db:"zone_builder_runtime_config" json:"zone_builder_runtime_config"`
	CreatedAt                time.Time `db:"created_at"                  json:"created_at"`
}

type SRZoneNormalizedProjections struct {
	Decision        *SRDecision
	EventDetections []MarketEventDetection
	EventStates     []MarketEventState
	DailyCandidates []SRDailyCandidate
	ModelGovernance *SRModelGovernance
}

type SRDecision struct {
	ID                          uint64    `db:"id"                               json:"id"`
	AnalysisID                  uint64    `db:"analysis_id"                      json:"analysis_id"`
	Symbol                      string    `db:"symbol"                           json:"symbol"`
	Timeframe                   string    `db:"timeframe"                        json:"timeframe"`
	AnalyzedAt                  time.Time `db:"analyzed_at"                      json:"analyzed_at"`
	MarketBias                  string    `db:"market_bias"                      json:"market_bias"`
	EntryPermissionState        string    `db:"entry_permission_state"           json:"entry_permission_state"`
	PositionAction              string    `db:"position_action"                  json:"position_action"`
	PricePathState              string    `db:"price_path_state"                 json:"price_path_state"`
	ModelHealthState            string    `db:"model_health_state"               json:"model_health_state"`
	EventMarketState            string    `db:"event_market_state"               json:"event_market_state"`
	ReasonCodes                 RawJSON   `db:"reason_codes"                     json:"reason_codes"`
	MarketRegimeJSON            RawJSON   `db:"market_regime_json"               json:"market_regime_json"`
	DataQualityJSON             RawJSON   `db:"data_quality_json"                json:"data_quality_json"`
	DecisionDerivedViewJSON     RawJSON   `db:"decision_derived_view_json"       json:"decision_derived_view_json"`
	EventSequenceJSON           RawJSON   `db:"event_sequence_json"              json:"event_sequence_json"`
	DailyPriceActionJSON        RawJSON   `db:"daily_price_action_json"          json:"daily_price_action_json"`
	PricePathJSON               RawJSON   `db:"price_path_json"                  json:"price_path_json"`
	DailyConfirmationJSON       RawJSON   `db:"daily_confirmation_json"          json:"daily_confirmation_json"`
	DefenseLinesJSON            RawJSON   `db:"defense_lines_json"               json:"defense_lines_json"`
	EntryExecutabilityJSON      RawJSON   `db:"entry_executability_json"         json:"entry_executability_json"`
	EntryBlockingZoneJSON       RawJSON   `db:"entry_blocking_zone_json"         json:"entry_blocking_zone_json"`
	RRContextJSON               RawJSON   `db:"rr_context_json"                  json:"rr_context_json"`
	RRGateJSON                  RawJSON   `db:"rr_gate_json"                     json:"rr_gate_json"`
	PositionActionConditionJSON RawJSON   `db:"position_action_condition_json"   json:"position_action_condition_json"`
	MarketContextJSON           RawJSON   `db:"market_context_json"              json:"market_context_json"`
	ConfidenceExplanationJSON   RawJSON   `db:"confidence_explanation_json"      json:"confidence_explanation_json"`
	RiskNotesJSON               RawJSON   `db:"risk_notes_json"                  json:"risk_notes_json"`
	ZoneSummariesJSON           RawJSON   `db:"zone_summaries_json"              json:"zone_summaries_json"`
	DecisionSummary             RawJSON   `db:"decision_summary"                 json:"decision_summary"`
	CreatedAt                   time.Time `db:"created_at"                       json:"created_at"`
}

type MarketEventDetection struct {
	ID          uint64      `db:"id"           json:"id"`
	AnalysisID  uint64      `db:"analysis_id"  json:"analysis_id"`
	Symbol      string      `db:"symbol"       json:"symbol"`
	Timeframe   string      `db:"timeframe"    json:"timeframe"`
	AnalyzedAt  time.Time   `db:"analyzed_at"  json:"analyzed_at"`
	EventKey    string      `db:"event_key"    json:"event_key"`
	EventType   string      `db:"event_type"   json:"event_type"`
	EventFamily string      `db:"event_family" json:"event_family"`
	EventScope  string      `db:"event_scope"  json:"event_scope"`
	ZoneKey     string      `db:"zone_key"     json:"zone_key"`
	Direction   string      `db:"direction"    json:"direction"`
	State       string      `db:"state"        json:"state"`
	Active      bool        `db:"active"       json:"active"`
	Confidence  NullFloat64 `db:"confidence"   json:"confidence,omitempty"`
	PriceLevel  NullFloat64 `db:"price_level"  json:"price_level,omitempty"`
	ReasonCodes RawJSON     `db:"reason_codes" json:"reason_codes"`
	EventJSON   RawJSON     `db:"event_json"   json:"event_json"`
	CreatedAt   time.Time   `db:"created_at"   json:"created_at"`
}

type MarketEventState struct {
	ID              uint64      `db:"id"                json:"id"`
	AnalysisID      uint64      `db:"analysis_id"       json:"analysis_id"`
	Symbol          string      `db:"symbol"            json:"symbol"`
	Timeframe       string      `db:"timeframe"         json:"timeframe"`
	AnalyzedAt      time.Time   `db:"analyzed_at"       json:"analyzed_at"`
	EventKey        string      `db:"event_key"         json:"event_key"`
	EventType       string      `db:"event_type"        json:"event_type"`
	EventFamily     string      `db:"event_family"      json:"event_family"`
	EventScope      string      `db:"event_scope"       json:"event_scope"`
	ZoneKey         string      `db:"zone_key"          json:"zone_key"`
	RootEventType   string      `db:"root_event_type"   json:"root_event_type"`
	LatestEventType string      `db:"latest_event_type" json:"latest_event_type"`
	Direction       string      `db:"direction"         json:"direction"`
	State           string      `db:"state"             json:"state"`
	Active          bool        `db:"active"            json:"active"`
	ResolvedBy      NullString  `db:"resolved_by"       json:"resolved_by,omitempty"`
	Confidence      NullFloat64 `db:"confidence"        json:"confidence,omitempty"`
	PriceLevel      NullFloat64 `db:"price_level"       json:"price_level,omitempty"`
	ReasonCodes     RawJSON     `db:"reason_codes"      json:"reason_codes"`
	StateJSON       RawJSON     `db:"state_json"        json:"state_json"`
	CreatedAt       time.Time   `db:"created_at"        json:"created_at"`
}

type SRDailyCandidate struct {
	ID            uint64      `db:"id"             json:"id"`
	AnalysisID    uint64      `db:"analysis_id"    json:"analysis_id"`
	Symbol        string      `db:"symbol"         json:"symbol"`
	Timeframe     string      `db:"timeframe"      json:"timeframe"`
	AnalyzedAt    time.Time   `db:"analyzed_at"    json:"analyzed_at"`
	PriceLow      float64     `db:"price_low"      json:"price_low"`
	PriceHigh     float64     `db:"price_high"     json:"price_high"`
	Label         string      `db:"label"          json:"label"`
	Role          string      `db:"role"           json:"role"`
	Source        string      `db:"source"         json:"source"`
	Lifecycle     string      `db:"lifecycle"      json:"lifecycle"`
	DecisionRole  string      `db:"decision_role"  json:"decision_role"`
	DistancePct   NullFloat64 `db:"distance_pct"   json:"distance_pct,omitempty"`
	DistanceLabel string      `db:"distance_label" json:"distance_label"`
	Reason        string      `db:"reason"         json:"reason"`
	EventRefs     RawJSON     `db:"event_refs"     json:"event_refs"`
	CandidateJSON RawJSON     `db:"candidate_json" json:"candidate_json"`
	CreatedAt     time.Time   `db:"created_at"     json:"created_at"`
}

type SRModelGovernance struct {
	ID                     uint64      `db:"id"                       json:"id"`
	AnalysisID             uint64      `db:"analysis_id"              json:"analysis_id"`
	Symbol                 string      `db:"symbol"                   json:"symbol"`
	Timeframe              string      `db:"timeframe"                json:"timeframe"`
	AnalyzedAt             time.Time   `db:"analyzed_at"              json:"analyzed_at"`
	ModelVersion           string      `db:"model_version"            json:"model_version"`
	ModelConfigHash        string      `db:"model_config_hash"        json:"model_config_hash"`
	HealthState            string      `db:"health_state"             json:"health_state"`
	AverageEdgePP          NullFloat64 `db:"average_edge_pp"          json:"average_edge_pp,omitempty"`
	DirectionalZoneCount   NullInt64   `db:"directional_zone_count"   json:"directional_zone_count,omitempty"`
	ZoneCount              NullInt64   `db:"zone_count"               json:"zone_count,omitempty"`
	AllowEntry             NullBool    `db:"allow_entry"              json:"allow_entry,omitempty"`
	MaxEntryState          string      `db:"max_entry_state"          json:"max_entry_state"`
	QualityFlags           RawJSON     `db:"quality_flags"            json:"quality_flags"`
	WarningFlags           RawJSON     `db:"warning_flags"            json:"warning_flags"`
	BlockingFlags          RawJSON     `db:"blocking_flags"           json:"blocking_flags"`
	ConfidenceGateJSON     RawJSON     `db:"confidence_gate_json"     json:"confidence_gate_json"`
	CalibrationReportJSON  RawJSON     `db:"calibration_report_json"  json:"calibration_report_json"`
	WalkForwardReportJSON  RawJSON     `db:"walk_forward_report_json" json:"walk_forward_report_json"`
	DatasetDiagnosticsJSON RawJSON     `db:"dataset_diagnostics_json" json:"dataset_diagnostics_json"`
	GovernanceJSON         RawJSON     `db:"governance_json"          json:"governance_json"`
	CreatedAt              time.Time   `db:"created_at"               json:"created_at"`
}

type SRModelMetric struct {
	ID           uint64 `db:"id"                   json:"id"`
	TrainJobID   uint64 `db:"train_job_id"         json:"train_job_id"`
	JobID        string `db:"job_id"               json:"job_id"`
	ModelVersion string `db:"model_version"        json:"model_version"`
	ModelType    string `db:"model_type"           json:"model_type"`
	SplitMethod  string `db:"split_method"         json:"split_method"`
	Timeframe    string `db:"timeframe"            json:"timeframe"`
	// db 欄位名 row_count ≠ json 欄位名 rows：rows 是 MySQL 保留字，
	// 裸寫在查詢語句裡在 MySQL 上會語法錯誤（migration 059 改名，見 issue.md I-054）。
	// json tag 維持 rows，所以 API 與前端不受影響。
	Rows               NullInt64   `db:"row_count"            json:"rows,omitempty"`
	Sources            NullInt64   `db:"sources"              json:"sources,omitempty"`
	HoldAUC            NullFloat64 `db:"hold_auc"             json:"hold_auc,omitempty"`
	HoldBrierScore     NullFloat64 `db:"hold_brier_score"     json:"hold_brier_score,omitempty"`
	HoldLogLoss        NullFloat64 `db:"hold_log_loss"        json:"hold_log_loss,omitempty"`
	HoldCalibrated     NullBool    `db:"hold_calibrated"      json:"hold_calibrated,omitempty"`
	HoldTestRows       NullInt64   `db:"hold_test_rows"       json:"hold_test_rows,omitempty"`
	BreakAUC           NullFloat64 `db:"break_auc"            json:"break_auc,omitempty"`
	BreakBrierScore    NullFloat64 `db:"break_brier_score"    json:"break_brier_score,omitempty"`
	BreakLogLoss       NullFloat64 `db:"break_log_loss"       json:"break_log_loss,omitempty"`
	BreakCalibrated    NullBool    `db:"break_calibrated"     json:"break_calibrated,omitempty"`
	BreakTestRows      NullInt64   `db:"break_test_rows"      json:"break_test_rows,omitempty"`
	MetricsJSON        RawJSON     `db:"metrics_json"         json:"metrics_json"`
	DatasetSummaryJSON RawJSON     `db:"dataset_summary_json" json:"dataset_summary_json"`
	CreatedAt          time.Time   `db:"created_at"           json:"created_at"`
}

type SRRegressionResult struct {
	ID                     uint64      `db:"id"                         json:"id"`
	RunID                  string      `db:"run_id"                     json:"run_id"`
	ModelConfigHash        string      `db:"model_config_hash"          json:"model_config_hash"`
	PipelineVersion        string      `db:"pipeline_version"           json:"pipeline_version"`
	DatasetFrom            NullTime    `db:"dataset_from"               json:"dataset_from,omitempty"`
	DatasetTo              NullTime    `db:"dataset_to"                 json:"dataset_to,omitempty"`
	SplitMethod            string      `db:"split_method"               json:"split_method"`
	HoldAUC                NullFloat64 `db:"hold_auc"                   json:"hold_auc,omitempty"`
	HoldBrierScore         NullFloat64 `db:"hold_brier_score"           json:"hold_brier_score,omitempty"`
	BreakAUC               NullFloat64 `db:"break_auc"                  json:"break_auc,omitempty"`
	BreakBrierScore        NullFloat64 `db:"break_brier_score"          json:"break_brier_score,omitempty"`
	Passed                 NullBool    `db:"passed"                     json:"passed,omitempty"`
	SchemaVersion          string      `db:"schema_version"             json:"schema_version"`
	Rows                   NullInt64   `db:"result_rows"                json:"rows,omitempty"`
	Sources                NullInt64   `db:"source_count"               json:"sources,omitempty"`
	GovernanceHealthState  string      `db:"governance_health_state"    json:"governance_health_state"`
	GovernanceStrictPassed NullBool    `db:"governance_strict_passed"   json:"governance_strict_passed,omitempty"`
	MetricsJSON            RawJSON     `db:"metrics_json"               json:"metrics_json"`
	CreatedAt              time.Time   `db:"created_at"                 json:"created_at"`
}

// SREvaluationJob 追蹤一次 SR Zone evaluation / decision replay 背景任務。
// 實際計算仍由 Python /sr-scoring/evaluate 同步執行；Go 在背景 goroutine
// 呼叫後把 report 存回這張表，前端可輪詢進度，不必讓 HTTP request 一直等待。
type SREvaluationJob struct {
	ID              uint64     `db:"id"               json:"id"`
	JobID           string     `db:"job_id"           json:"job_id"`
	Status          string     `db:"status"           json:"status"` // pending / running / done / failed
	Symbols         string     `db:"symbols"          json:"symbols"`
	Timeframe       string     `db:"timeframe"        json:"timeframe"`
	FetchLimit      int        `db:"fetch_limit"      json:"fetch_limit"`
	Mode            string     `db:"mode"             json:"mode"`
	WriteDB         bool       `db:"write_db"         json:"write_db"`
	ReplayMaxRows   int        `db:"replay_max_rows"  json:"replay_max_rows"`
	RunID           NullString `db:"run_id"           json:"run_id,omitempty"`
	SchemaVersion   NullString `db:"schema_version"   json:"schema_version,omitempty"`
	PipelineVersion NullString `db:"pipeline_version" json:"pipeline_version,omitempty"`
	Rows            NullInt64  `db:"result_rows"      json:"rows,omitempty"`
	Sources         NullInt64  `db:"source_count"     json:"sources,omitempty"`
	Report          RawJSON    `db:"report"           json:"report,omitempty"`
	Error           NullString `db:"error"            json:"error,omitempty"`
	StartedAt       NullTime   `db:"started_at"       json:"started_at,omitempty"`
	FinishedAt      NullTime   `db:"finished_at"      json:"finished_at,omitempty"`
	CreatedAt       time.Time  `db:"created_at"       json:"created_at"`
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
	// TIER_3_SHORT_TERM 短期），讓 zone 清單可排序（見
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
	// docs/sr-zone-scoring.md 十五（Zone 生命週期驗證）。
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
	// db 欄位名 row_count ≠ json 欄位名 rows，理由同 SRModelMetric.Rows。
	Rows         NullInt64  `db:"row_count"        json:"rows,omitempty"`
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
	Watched     bool                  `db:"watched" json:"watched"`
	StockSymbol *WatchlistStockSymbol `db:"-"       json:"stock_symbol"`
}

type WatchlistStockSymbol struct {
	Exists       bool     `json:"exists"`
	IsListed     bool     `json:"is_listed"`
	ISINCode     string   `json:"isin_code"`
	Market       string   `json:"market"`
	SecurityType string   `json:"security_type"`
	Industry     string   `json:"industry"`
	LastSeenAt   NullTime `json:"last_seen_at,omitempty"`
}

type StockSymbol struct {
	ID           uint64    `db:"id"            json:"id"`
	Symbol       string    `db:"symbol"        json:"symbol"`
	Name         string    `db:"name"          json:"name"`
	ISINCode     string    `db:"isin_code"     json:"isin_code"`
	Market       string    `db:"market"        json:"market"`
	SecurityType string    `db:"security_type" json:"security_type"`
	Industry     string    `db:"industry"      json:"industry"`
	CFICode      string    `db:"cfi_code"      json:"cfi_code"`
	Remarks      string    `db:"remarks"       json:"remarks"`
	ListedDate   NullTime  `db:"listed_date"   json:"listed_date,omitempty"`
	IsListed     bool      `db:"is_listed"     json:"is_listed"`
	LastSeenAt   time.Time `db:"last_seen_at"  json:"last_seen_at"`
	CreatedAt    time.Time `db:"created_at"    json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"    json:"updated_at"`
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
	Status    string `db:"status"      json:"status"` // pending/running/done/failed
	// db 欄位名 trigger_source ≠ json 欄位名 trigger，理由同 SRModelMetric.Rows。
	Trigger string `db:"trigger_source" json:"trigger"` // manual/scheduler
	Error   string `db:"error"       json:"error,omitempty"`
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
	// db 欄位名 signal_type ≠ json 欄位名 signal，理由同 SRModelMetric.Rows。
	Signal    string    `db:"signal_type"          json:"signal"` // BULLISH/BEARISH/NEUTRAL/RISK
	Reason    RawJSON   `db:"reason"               json:"reason,omitempty"`
	CreatedAt time.Time `db:"created_at"           json:"created_at"`
	UpdatedAt time.Time `db:"updated_at"           json:"updated_at"`
}

// ChipSyncJob 追蹤一次 manual / backfill 籌碼資料同步任務（daily 模式沿用
// 既有 job_runs 表，job_name="chip_daily_sync"，見 scheduler.go）。Failures
// 用 RawJSON 讀寫，DB 欄位 NOT NULL DEFAULT '[]'，理由同 ChipScore.Reason。
type ChipSyncJob struct {
	ID        uint64 `db:"id"             json:"id"`
	JobID     string `db:"job_id"         json:"job_id"`
	Mode      string `db:"mode"           json:"mode"`       // manual / backfill
	Symbols   string `db:"symbols"        json:"symbols"`    // JSON array string
	DataTypes string `db:"data_types"     json:"data_types"` // JSON array string
	FromDate  string `db:"from_date"      json:"from_date"`
	ToDate    string `db:"to_date"        json:"to_date"`
	// db 欄位名 force_sync ≠ json 欄位名 force，理由同 SRModelMetric.Rows。
	Force         bool       `db:"force_sync"     json:"force"`
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

// MarketBackfillJob 追蹤 POST /market/backfill 的執行進度。
// 形狀刻意比照 ChipSyncJob（同一個前端頁面上兩塊 UI 要一致），差別只在回補範圍：
// 籌碼用 from_date/to_date，股價用 Days（往前回補幾天，對齊 Fetcher.BackfillHistory）。
type MarketBackfillJob struct {
	ID            uint64     `db:"id"             json:"id"`
	JobID         string     `db:"job_id"         json:"job_id"`
	Symbols       string     `db:"symbols"        json:"symbols"` // JSON array string
	Days          int        `db:"days"           json:"days"`
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

type Portfolio struct {
	ID              uint64    `db:"id"                 json:"id"`
	TenantID        uint64    `db:"tenant_id"          json:"tenant_id"`
	Name            string    `db:"name"               json:"name"`
	OwnerType       string    `db:"owner_type"         json:"owner_type"`
	OwnerID         NullInt64 `db:"owner_id"           json:"owner_id,omitempty"`
	CreatedByUserID NullInt64 `db:"created_by_user_id" json:"created_by_user_id,omitempty"`
	IsDefault       bool      `db:"is_default"         json:"is_default"`
	CanWrite        bool      `db:"can_write"          json:"can_write"`
	CreatedAt       time.Time `db:"created_at"         json:"created_at"`
	UpdatedAt       time.Time `db:"updated_at"         json:"updated_at"`
}

type Group struct {
	ID        uint64    `db:"id"         json:"id"`
	TenantID  uint64    `db:"tenant_id"  json:"tenant_id"`
	Name      string    `db:"name"       json:"name"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

type GroupMember struct {
	GroupID   uint64    `db:"group_id"   json:"group_id"`
	UserID    uint64    `db:"user_id"    json:"user_id"`
	Role      string    `db:"role"       json:"role"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// Position is the materialized AVG-cost projection rebuilt transactionally
// from immutable PositionTransaction events.
type Position struct {
	PortfolioID uint64    `db:"portfolio_id" json:"portfolio_id"`
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
	PortfolioID   uint64      `db:"portfolio_id"    json:"portfolio_id"`
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
	PortfolioID            uint64      `db:"portfolio_id"           json:"portfolio_id"`
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

// EvaluationUniverseEntry 是評估標的池的一筆成員（T-040 Step 5）。
//
// **與 watchlists 分離**：watchlists 驅動盤中掃描、籌碼同步、日結掃描、signal 與
// production SR 分析；本表只驅動一件事——每日盤後更新日 K。不參與任何交易決策或狀態推導。
// 規格見 docs/evaluation-universe-selection-plan.md 的「Step 5 執行計畫書」。
type EvaluationUniverseEntry struct {
	ID         uint64 `db:"id"           json:"id"`
	Symbol     string `db:"symbol"       json:"symbol"`
	BucketHint string `db:"bucket_hint"  json:"bucket_hint"`
	// BucketEdgeLow / BucketEdgeHigh 是入池時**實際使用的分位數邊界**，刻意存在每一列。
	// BucketHint 單獨存在無法回答「這個 bucket 是用哪組邊界判的」——實測 2026-08-17 有
	// 3 檔 atr_pct 完全未變卻換桶，只因母體變了邊界移動。應填入 zone_builder.py 的
	// LOW/HIGH_VOLATILITY_THRESHOLD 當下的值。
	BucketEdgeLow   float64 `db:"bucket_edge_low"  json:"bucket_edge_low"`
	BucketEdgeHigh  float64 `db:"bucket_edge_high" json:"bucket_edge_high"`
	UniverseVersion string  `db:"universe_version" json:"universe_version"`
	// UniverseRole：primary 參與股票 builder 決策，supplemental 只作交叉觀察。
	UniverseRole string    `db:"universe_role" json:"universe_role"`
	SelectedAt   time.Time `db:"selected_at"   json:"selected_at"`
	Source       string    `db:"source"        json:"source"`
	// Active=false 代表保留紀錄但不再納入每日維護。刻意不刪除：入退池歷史本身是研究紀錄。
	Active bool   `db:"active" json:"active"`
	Note   string `db:"note"   json:"note"`
}

// AllUniverseRoles 是 UniverseRole 的合法值。
//
// 由 TestPostgresMigrationsRealValuesFitAllColumns 與
// TestMySQLMigrationsRealValuesFitAllColumns 取用：universe_role 是 VARCHAR(16) 且沒有
// CHECK 約束，少了那兩支測試，日後新增一個較長的值不會有任何東西擋下。
func AllUniverseRoles() []string { return []string{"primary", "supplemental"} }
