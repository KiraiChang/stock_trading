// Package analysis 提供個股現況分析（支撐/壓力/進場/停損/停利）：
// 實際計算委由 Python（backtest/modular/analysis.py，重用既有的模組化策略
// 元件），Go 只負責呼叫、持久化與後續驗證，避免同一套數學邏輯在兩個語言
// 各刻一份（見 docs/python-go-integration-specification.md 的 Strategy
// Consistency Rule）。
package analysis

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/trading/backend/internal/store"
)

// UpstreamStatusError 保留 Python service 回應的實際 HTTP 狀態碼（例如
// 404「沒有 candles」、503「模型未訓練」），讓呼叫端可以用 errors.As 判斷
// 具體是哪種情況、回給前端對應的通用訊息——而不是把所有非 200 回應都壓成
// 同一種「Python service 錯誤」，導致前端沒辦法分辨「該補資料」還是
// 「該去訓練模型」。Error() 保留原始回應內容只用於伺服器 log，不會被拿去
// 直接顯示給前端（詳細錯誤文字外洩到前端的風險見 sr_zones.go handler 的
// 錯誤處理慣例）。
type UpstreamStatusError struct {
	StatusCode int
	Body       string
}

func (e *UpstreamStatusError) Error() string {
	return fmt.Sprintf("upstream status=%d body=%s", e.StatusCode, e.Body)
}

type Level struct {
	Price    float64 `json:"price"`
	Strength float64 `json:"strength"`
	Method   string  `json:"method"`
}

type Entry struct {
	Status    string  `json:"status"` // ACTIVE / WATCHING
	Direction string  `json:"direction"`
	Price     float64 `json:"price"`
	Reason    string  `json:"reason"`
}

type StopLoss struct {
	ATR        *float64 `json:"atr"`
	Structural *float64 `json:"structural"`
	Composite  *float64 `json:"composite"`
}

type TakeProfit struct {
	NextLevel   *float64 `json:"next_level"`
	RiskReward  *float64 `json:"risk_reward"`
	ATRMultiple *float64 `json:"atr_multiple"`
}

// Result 對應 Python analyze_symbol() 的回傳格式
type Result struct {
	Symbol       string     `json:"symbol"`
	Timeframe    string     `json:"timeframe"`
	AnalyzedAt   string     `json:"analyzed_at"` // RFC3339
	CurrentPrice float64    `json:"current_price"`
	Trend        string     `json:"trend"`
	Supports     []Level    `json:"supports"`
	Resistances  []Level    `json:"resistances"`
	Entry        Entry      `json:"entry"`
	StopLoss     StopLoss   `json:"stop_loss"`
	TakeProfit   TakeProfit `json:"take_profit"`
}

// ToStore 把 Python 回傳的分析結果轉成可以直接寫入 DB 的型別
func (r *Result) ToStore() (*store.StockAnalysis, []store.StockAnalysisLevel, error) {
	analyzedAt, err := time.Parse(time.RFC3339, r.AnalyzedAt)
	if err != nil {
		return nil, nil, fmt.Errorf("parse analyzed_at %q: %w", r.AnalyzedAt, err)
	}

	a := &store.StockAnalysis{
		Symbol:               r.Symbol,
		Timeframe:            r.Timeframe,
		AnalyzedAt:           analyzedAt,
		CurrentPrice:         r.CurrentPrice,
		Trend:                r.Trend,
		EntryStatus:          r.Entry.Status,
		EntryDirection:       r.Entry.Direction,
		EntryPrice:           r.Entry.Price,
		EntryReason:          store.NullString{NullString: sql.NullString{String: r.Entry.Reason, Valid: r.Entry.Reason != ""}},
		StopLossATR:          nullFloat(r.StopLoss.ATR),
		StopLossStructural:   nullFloat(r.StopLoss.Structural),
		StopLossComposite:    nullFloat(r.StopLoss.Composite),
		TakeProfitNextLevel:  nullFloat(r.TakeProfit.NextLevel),
		TakeProfitRiskReward: nullFloat(r.TakeProfit.RiskReward),
		TakeProfitATR:        nullFloat(r.TakeProfit.ATRMultiple),
	}

	levels := make([]store.StockAnalysisLevel, 0, len(r.Supports)+len(r.Resistances))
	for _, lv := range r.Supports {
		levels = append(levels, store.StockAnalysisLevel{Price: lv.Price, Type: "SUPPORT", Strength: lv.Strength, Method: lv.Method, Status: "PENDING"})
	}
	for _, lv := range r.Resistances {
		levels = append(levels, store.StockAnalysisLevel{Price: lv.Price, Type: "RESISTANCE", Strength: lv.Strength, Method: lv.Method, Status: "PENDING"})
	}

	return a, levels, nil
}

func nullFloat(p *float64) store.NullFloat64 {
	if p == nil {
		return store.NullFloat64{}
	}
	return store.NullFloat64{NullFloat64: sql.NullFloat64{Float64: *p, Valid: true}}
}

func nullString(p *string) store.NullString {
	if p == nil {
		return store.NullString{}
	}
	return store.NullString{NullString: sql.NullString{String: *p, Valid: true}}
}

func nullInt(p *int) store.NullInt64 {
	if p == nil {
		return store.NullInt64{}
	}
	return store.NullInt64{NullInt64: sql.NullInt64{Int64: int64(*p), Valid: true}}
}

// Client 呼叫 Python HTTP service 的 /analyze 端點
type Client struct {
	baseURL     string
	http        *http.Client
	srZonesHTTP *http.Client
}

const (
	defaultHTTPTimeout        = 30 * time.Second
	defaultSRZonesHTTPTimeout = 120 * time.Second
)

func NewClient(baseURL string) *Client {
	return NewClientWithSRZonesTimeout(baseURL, defaultSRZonesHTTPTimeout)
}

func NewClientWithSRZonesTimeout(baseURL string, srZonesTimeout time.Duration) *Client {
	if srZonesTimeout <= 0 {
		srZonesTimeout = defaultSRZonesHTTPTimeout
	}
	return &Client{
		baseURL:     baseURL,
		http:        &http.Client{Timeout: defaultHTTPTimeout},
		srZonesHTTP: &http.Client{Timeout: srZonesTimeout},
	}
}

// analyzeRequest 對應 Python AnalyzeRequest；Limit 為 0 時省略欄位，
// 讓 Python 端套用它自己的預設值（DEFAULT_FETCH_LIMIT），而不是把 0 傳過去
// 變成「抓 0 根」。
type analyzeRequest struct {
	Symbol    string `json:"symbol"`
	Timeframe string `json:"timeframe"`
	Limit     int    `json:"limit,omitempty"`
}

// Analyze 呼叫 Python /analyze 端點。limit 為抓取的歷史K棒根數，傳 0 表示
// 使用 Python 端的預設值（見 analyzeRequest 註解）。
func (c *Client) Analyze(ctx context.Context, symbol, timeframe string, limit int) (*Result, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf("python service url not configured（請設定 python.service_url / PYTHON_SERVICE_URL）")
	}

	body, err := json.Marshal(analyzeRequest{Symbol: symbol, Timeframe: timeframe, Limit: limit})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/analyze", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("python analyze request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("python analyze read body error: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("python analyze error: status=%d body=%s", resp.StatusCode, truncateBody(respBody))
	}

	var result Result
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("python analyze decode error: body=%s: %w", truncateBody(respBody), err)
	}
	return &result, nil
}

// ── SR Zone Scoring（機構級版本，2026-07 重新設計，見 Python
// backtest/modular/sr_scoring/scoring.py 開頭的完整說明）──────────────

type ZoneScore struct {
	PriceLow                float64            `json:"price_low"`
	PriceHigh               float64            `json:"price_high"`
	Method                  string             `json:"method"`
	Role                    string             `json:"role"`
	Tier                    string             `json:"tier"`
	TierLabel               string             `json:"tier_label"`
	SupportScore            float64            `json:"support_score"`
	ResistanceScore         float64            `json:"resistance_score"`
	NetScore                float64            `json:"net_score"`
	NetScoreLabel           string             `json:"net_score_label"`
	Confidence              float64            `json:"confidence"`
	ConfidenceLevel         string             `json:"confidence_level"`
	BounceProbability       *float64           `json:"bounce_probability"`
	BreakProbability        *float64           `json:"break_probability"`
	ExpectedGain            *float64           `json:"expected_gain"`
	ExpectedLoss            *float64           `json:"expected_loss"`
	ExpectedValue           *float64           `json:"expected_value"`
	RiskRewardRatio         *float64           `json:"risk_reward_ratio"`
	RewardRiskPercentile    *float64           `json:"reward_risk_percentile"`
	RelativeVolume          *float64           `json:"relative_volume"`
	VolumeConfirmation      *string            `json:"volume_confirmation"`
	TouchCount              int                `json:"touch_count"`
	SupportTouchCount       int                `json:"support_touch_count"`
	ResistanceTouchCount    int                `json:"resistance_touch_count"`
	RejectCount             *int               `json:"reject_count"`
	BreakCount              *int               `json:"break_count"`
	ZoneMomentum            float64            `json:"zone_momentum"`
	ZoneDirection           string             `json:"zone_direction"`
	RecentValidation        string             `json:"recent_validation"`
	TradingScore            float64            `json:"trading_score"`
	TradingScoreBreakdown   map[string]float64 `json:"trading_score_breakdown"`
	TradingRecommendation   string             `json:"trading_recommendation"`
	ZoneQualityScore        *float64           `json:"zone_quality_score,omitempty"`
	EntryRelevanceScore     *float64           `json:"entry_relevance_score,omitempty"`
	EntryRelevanceBreakdown map[string]float64 `json:"entry_relevance_breakdown,omitempty"`
	// OverlapGroup/ConfluenceCount：跨方法（ATR/volume_profile）重疊的 zone
	// 分群（見 Python scoring.py::_group_overlapping_zones）。不合併/刪除
	// 任何 zone，只標記供 UI 顯示「多方法共振」。ConfluenceCount 恆 >= 1；
	// OverlapGroup 只有 ConfluenceCount > 1 時才有值。
	OverlapGroup          *int            `json:"overlap_group"`
	ConfluenceCount       int             `json:"confluence_count"`
	ConfluenceFamilyCount int             `json:"confluence_family_count,omitempty"`
	ConfluenceFamilies    []string        `json:"confluence_families,omitempty"`
	Features              json.RawMessage `json:"features,omitempty"`
	Evidence              json.RawMessage `json:"evidence,omitempty"`
	Explanation           json.RawMessage `json:"explanation,omitempty"`
	Scenario              json.RawMessage `json:"scenario,omitempty"`
	ProbabilityContext    json.RawMessage `json:"probability_context,omitempty"`
}

func (z *ZoneScore) UnmarshalJSON(data []byte) error {
	type plain ZoneScore
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	var nested struct {
		Score              *plain          `json:"score"`
		Features           json.RawMessage `json:"features"`
		Evidence           json.RawMessage `json:"evidence"`
		Explanation        json.RawMessage `json:"explanation"`
		Scenario           json.RawMessage `json:"scenario"`
		ProbabilityContext json.RawMessage `json:"probability_context"`
	}
	_, hasScore := fields["score"]
	_, hasData := fields["data"]
	_, hasLifecycle := fields["lifecycle"]
	if hasScore || hasData || hasLifecycle {
		if err := json.Unmarshal(data, &nested); err != nil {
			return err
		}
		if nested.Score == nil {
			return errors.New("nested zone score must contain a non-null score")
		}
		*z = ZoneScore(*nested.Score)
		z.Features = nested.Features
		z.Evidence = nested.Evidence
		z.Explanation = nested.Explanation
		z.Scenario = nested.Scenario
		z.ProbabilityContext = nested.ProbabilityContext
		return nil
	}
	var direct plain
	if err := json.Unmarshal(data, &direct); err != nil {
		return err
	}
	*z = ZoneScore(direct)
	return nil
}

type zonePipelineAnalysis struct {
	Symbol          string          `json:"symbol"`
	Timeframe       string          `json:"timeframe"`
	AnalyzedAt      string          `json:"analyzed_at"`
	CurrentPrice    float64         `json:"current_price"`
	PeriodSummaries json.RawMessage `json:"period_summaries"`
	AnalysisTips    json.RawMessage `json:"analysis_tips"`
	ChipSummary     json.RawMessage `json:"chip_summary"`
	Model           struct {
		Version      string   `json:"version"`
		TrainedAt    string   `json:"trained_at"`
		ConfigHash   string   `json:"config_hash"`
		FeatureNames []string `json:"feature_names"`
	} `json:"model"`
}

type zonePipelineFeatures struct {
	GlobalTrend      float64 `json:"global_trend"`
	GlobalVolatility float64 `json:"global_volatility"`
}

type zonePipelineScore struct {
	GlobalExpectedValue   *float64 `json:"global_expected_value"`
	GlobalConfidence      *float64 `json:"global_confidence"`
	GlobalRiskRewardRatio *float64 `json:"global_risk_reward_ratio"`
}

// ZoneScoreResult is the breaking v2 Data -> Features -> Score -> Evidence ->
// Decision response contract returned by Python.
type ZoneScoreResult struct {
	PipelineVersion    string               `json:"pipeline_version"`
	Analysis           zonePipelineAnalysis `json:"analysis"`
	Features           zonePipelineFeatures `json:"features"`
	Score              zonePipelineScore    `json:"score"`
	Evidence           json.RawMessage      `json:"evidence"`
	Decision           json.RawMessage      `json:"decision"`
	Explanation        json.RawMessage      `json:"explanation"`
	Scenario           json.RawMessage      `json:"scenario"`
	ProbabilityContext json.RawMessage      `json:"probability_context"`
	Zones              []ZoneScore          `json:"zones"`

	// Legacy construction fields remain internal test/build compatibility only.
	Symbol                string          `json:"symbol,omitempty"`
	Timeframe             string          `json:"timeframe,omitempty"`
	AnalyzedAt            string          `json:"analyzed_at,omitempty"`
	CurrentPrice          float64         `json:"current_price,omitempty"`
	GlobalTrend           float64         `json:"global_trend,omitempty"`
	GlobalVolatility      float64         `json:"global_volatility,omitempty"`
	GlobalExpectedValue   *float64        `json:"global_expected_value,omitempty"`
	GlobalConfidence      *float64        `json:"global_confidence,omitempty"`
	GlobalRiskRewardRatio *float64        `json:"global_risk_reward_ratio,omitempty"`
	ModelVersion          string          `json:"model_version,omitempty"`
	ModelTrainedAt        string          `json:"model_trained_at,omitempty"`
	ModelFeatureNames     []string        `json:"model_feature_names,omitempty"`
	ModelConfigHash       string          `json:"model_config_hash,omitempty"`
	PeriodSummaries       json.RawMessage `json:"period_summaries,omitempty"`
	AnalysisTips          json.RawMessage `json:"analysis_tips,omitempty"`
	ChipSummary           json.RawMessage `json:"chip_summary,omitempty"`
	DecisionSummary       json.RawMessage `json:"decision_summary,omitempty"`
}

// ToStore 把 Python 回傳的 zone 評分結果轉成可以直接寫入 DB 的型別。
// ModelVersion 若 Python 端沒有回傳（理論上不應該發生，防禦性處理），寫入
// "unknown" 而不是空字串——空字串在 DB 裡看起來像「忘了填」，"unknown" 才
// 明確代表「這筆資料就是沒有版本資訊」。
func (r *ZoneScoreResult) ToStore() (*store.SRZoneAnalysis, []store.SRZone, store.SRZoneNormalizedProjections, error) {
	analysis := r.Analysis
	features := r.Features
	score := r.Score
	decision := r.Decision
	explanation := r.Explanation
	scenario := r.Scenario
	probabilityContext := r.ProbabilityContext
	periodSummaries := analysis.PeriodSummaries
	analysisTips := analysis.AnalysisTips
	chipSummary := analysis.ChipSummary
	if analysis.Symbol == "" {
		analysis.Symbol, analysis.Timeframe, analysis.AnalyzedAt = r.Symbol, r.Timeframe, r.AnalyzedAt
		analysis.CurrentPrice = r.CurrentPrice
		analysis.Model.Version, analysis.Model.TrainedAt = r.ModelVersion, r.ModelTrainedAt
		analysis.Model.ConfigHash, analysis.Model.FeatureNames = r.ModelConfigHash, r.ModelFeatureNames
		features.GlobalTrend, features.GlobalVolatility = r.GlobalTrend, r.GlobalVolatility
		score.GlobalExpectedValue, score.GlobalConfidence = r.GlobalExpectedValue, r.GlobalConfidence
		score.GlobalRiskRewardRatio = r.GlobalRiskRewardRatio
		decision = r.DecisionSummary
		periodSummaries, analysisTips, chipSummary = r.PeriodSummaries, r.AnalysisTips, r.ChipSummary
	}
	analyzedAt, err := time.Parse(time.RFC3339, analysis.AnalyzedAt)
	if err != nil {
		return nil, nil, store.SRZoneNormalizedProjections{}, fmt.Errorf("parse analyzed_at %q: %w", analysis.AnalyzedAt, err)
	}

	modelVersion := analysis.Model.Version
	if modelVersion == "" {
		modelVersion = "unknown"
	}

	a := &store.SRZoneAnalysis{
		Symbol:                analysis.Symbol,
		Timeframe:             analysis.Timeframe,
		AnalyzedAt:            analyzedAt,
		CurrentPrice:          analysis.CurrentPrice,
		GlobalTrend:           features.GlobalTrend,
		GlobalVolatility:      features.GlobalVolatility,
		GlobalExpectedValue:   nullFloat(score.GlobalExpectedValue),
		GlobalConfidence:      nullFloat(score.GlobalConfidence),
		GlobalRiskRewardRatio: nullFloat(score.GlobalRiskRewardRatio),
		ModelVersion:          modelVersion,
		ModelConfigHash:       analysis.Model.ConfigHash,
		PipelineVersion:       r.PipelineVersion,
		Evidence:              rawJSONOrDefault(r.Evidence, "null"),
		Explanation:           rawJSONOrDefault(explanation, "null"),
		Scenario:              rawJSONOrDefault(scenario, "null"),
		ProbabilityContext:    rawJSONOrDefault(probabilityContext, "null"),
		PeriodSummaries:       rawJSONOrDefault(periodSummaries, "[]"),
		AnalysisTips:          rawJSONOrDefault(analysisTips, "[]"),
		ChipSummary:           rawJSONOrDefault(chipSummary, "null"),
		DecisionSummary:       rawJSONOrDefault(decision, "null"),
	}

	zones := make([]store.SRZone, 0, len(r.Zones))
	for _, z := range r.Zones {
		rejectCount, breakCount := 0, 0
		if z.RejectCount != nil {
			rejectCount = *z.RejectCount
		}
		if z.BreakCount != nil {
			breakCount = *z.BreakCount
		}
		if err := validateTradingScoreBreakdown(z.TradingScoreBreakdown); err != nil {
			return nil, nil, store.SRZoneNormalizedProjections{}, fmt.Errorf("invalid trading_score_breakdown for zone %.2f-%.2f: %w", z.PriceLow, z.PriceHigh, err)
		}
		breakdownJSON, err := json.Marshal(z.TradingScoreBreakdown)
		if err != nil {
			return nil, nil, store.SRZoneNormalizedProjections{}, fmt.Errorf("marshal trading_score_breakdown: %w", err)
		}
		confluenceCount := z.ConfluenceCount
		if confluenceCount <= 0 {
			confluenceCount = 1
		}
		zones = append(zones, store.SRZone{
			PriceLow:              z.PriceLow,
			PriceHigh:             z.PriceHigh,
			Method:                z.Method,
			Role:                  z.Role,
			Tier:                  z.Tier,
			TierLabel:             z.TierLabel,
			SupportScore:          z.SupportScore,
			ResistanceScore:       z.ResistanceScore,
			NetScore:              z.NetScore,
			NetScoreLabel:         z.NetScoreLabel,
			Confidence:            z.Confidence,
			ConfidenceLevel:       z.ConfidenceLevel,
			BounceProbability:     nullFloat(z.BounceProbability),
			BreakProbability:      nullFloat(z.BreakProbability),
			ExpectedGain:          nullFloat(z.ExpectedGain),
			ExpectedLoss:          nullFloat(z.ExpectedLoss),
			ExpectedValue:         nullFloat(z.ExpectedValue),
			RiskRewardRatio:       nullFloat(z.RiskRewardRatio),
			RewardRiskPercentile:  nullFloat(z.RewardRiskPercentile),
			RelativeVolume:        nullFloat(z.RelativeVolume),
			VolumeConfirmation:    nullString(z.VolumeConfirmation),
			TouchCount:            z.TouchCount,
			SupportTouchCount:     z.SupportTouchCount,
			ResistanceTouchCount:  z.ResistanceTouchCount,
			RejectCount:           rejectCount,
			BreakCount:            breakCount,
			ZoneMomentum:          z.ZoneMomentum,
			ZoneDirection:         z.ZoneDirection,
			RecentValidation:      z.RecentValidation,
			TradingScore:          z.TradingScore,
			TradingScoreBreakdown: store.RawJSON(breakdownJSON),
			TradingRecommendation: z.TradingRecommendation,
			OverlapGroup:          nullInt(z.OverlapGroup),
			ConfluenceCount:       confluenceCount,
			Status:                "PENDING",
			Features:              rawJSONOrDefault(z.Features, "null"),
			Evidence:              rawJSONOrDefault(z.Evidence, "null"),
			Explanation:           rawJSONOrDefault(z.Explanation, "null"),
			Scenario:              rawJSONOrDefault(z.Scenario, "null"),
			ProbabilityContext:    rawJSONOrDefault(z.ProbabilityContext, "null"),
		})
	}

	projections, err := buildSRZoneNormalizedProjections(decision, probabilityContext)
	if err != nil {
		return nil, nil, store.SRZoneNormalizedProjections{}, err
	}
	return a, zones, projections, nil
}

func buildSRZoneNormalizedProjections(decision, probabilityContext json.RawMessage) (store.SRZoneNormalizedProjections, error) {
	projections := store.SRZoneNormalizedProjections{}
	decisionObj, ok, err := decodeJSONObject(decision)
	if err != nil {
		return store.SRZoneNormalizedProjections{}, err
	}
	if !ok {
		modelGovernance, err := buildModelGovernanceProjection(probabilityContext)
		if err != nil {
			return store.SRZoneNormalizedProjections{}, err
		}
		projections.ModelGovernance = modelGovernance
		return projections, nil
	}

	reasonCodes := collectDecisionReasonCodes(decisionObj)
	reasonCodesJSON, err := json.Marshal(reasonCodes)
	if err != nil {
		return store.SRZoneNormalizedProjections{}, fmt.Errorf("marshal decision reason_codes: %w", err)
	}

	projections.Decision = &store.SRDecision{
		MarketBias:                  stringAt(decisionObj, "market_bias"),
		EntryPermissionState:        stringAt(decisionObj, "final_entry_permission", "state"),
		PositionAction:              stringAt(decisionObj, "position_action"),
		PricePathState:              stringAt(decisionObj, "price_path", "path_state"),
		ModelHealthState:            stringAt(decisionObj, "model_governance", "health_state"),
		EventMarketState:            stringAt(decisionObj, "event_state_summary", "market_state"),
		ReasonCodes:                 store.RawJSON(reasonCodesJSON),
		MarketRegimeJSON:            decisionRawJSONAt(decisionObj, "null", "market_regime"),
		DataQualityJSON:             decisionRawJSONAt(decisionObj, "null", "data_quality"),
		DecisionDerivedViewJSON:     decisionRawJSONAt(decisionObj, "null", "decision_derived_view"),
		EventSequenceJSON:           decisionRawJSONAt(decisionObj, "[]", "event_sequence"),
		DailyPriceActionJSON:        decisionRawJSONAt(decisionObj, "null", "daily_price_action"),
		PricePathJSON:               decisionRawJSONAt(decisionObj, "null", "price_path"),
		DailyConfirmationJSON:       decisionRawJSONAt(decisionObj, "null", "daily_confirmation"),
		DefenseLinesJSON:            decisionRawJSONAt(decisionObj, "null", "defense_lines"),
		EntryExecutabilityJSON:      decisionRawJSONAt(decisionObj, "null", "entry_executability"),
		EntryBlockingZoneJSON:       decisionRawJSONAt(decisionObj, "null", "entry_blocking_zone"),
		RRContextJSON:               decisionRawJSONAt(decisionObj, "null", "rr_context"),
		RRGateJSON:                  decisionRawJSONAt(decisionObj, "null", "rr_gate"),
		PositionActionConditionJSON: decisionRawJSONAt(decisionObj, "null", "position_action_condition"),
		MarketContextJSON:           decisionRawJSONAt(decisionObj, "[]", "market_context"),
		ConfidenceExplanationJSON:   decisionRawJSONAt(decisionObj, "null", "confidence_explanation"),
		RiskNotesJSON:               decisionRawJSONAt(decisionObj, "[]", "risk_notes"),
		ZoneSummariesJSON:           buildDecisionZoneSummariesJSON(decisionObj),
		DecisionSummary:             rawJSONOrDefault(decision, "null"),
	}

	if events, ok := sliceAt(decisionObj, "market_events"); ok {
		projections.EventDetections = make([]store.MarketEventDetection, 0, len(events))
		for _, item := range events {
			event, ok := item.(map[string]any)
			if !ok {
				continue
			}
			eventJSON, err := json.Marshal(event)
			if err != nil {
				return store.SRZoneNormalizedProjections{}, fmt.Errorf("marshal market event: %w", err)
			}
			reasonsJSON, err := json.Marshal(stringSliceAt(event, "reason_codes"))
			if err != nil {
				return store.SRZoneNormalizedProjections{}, fmt.Errorf("marshal market event reason_codes: %w", err)
			}
			projections.EventDetections = append(projections.EventDetections, store.MarketEventDetection{
				EventKey:    stringAt(event, "event_key"),
				EventType:   stringAt(event, "type"),
				EventFamily: stringAt(event, "event_family"),
				EventScope:  stringAt(event, "event_scope"),
				ZoneKey:     stringAt(event, "zone_key"),
				Direction:   stringAt(event, "direction"),
				State:       stringAt(event, "state"),
				Active:      boolAt(event, "active"),
				Confidence:  nullFloat(anyFloatAt(event, "confidence")),
				PriceLevel:  nullFloat(anyFloatAt(event, "price_level")),
				ReasonCodes: store.RawJSON(reasonsJSON),
				EventJSON:   store.RawJSON(eventJSON),
			})
		}
	}

	if states, ok := sliceAt(decisionObj, "event_state_summary", "states"); ok {
		projections.EventStates = make([]store.MarketEventState, 0, len(states))
		for _, item := range states {
			state, ok := item.(map[string]any)
			if !ok {
				continue
			}
			stateJSON, err := json.Marshal(state)
			if err != nil {
				return store.SRZoneNormalizedProjections{}, fmt.Errorf("marshal market event state: %w", err)
			}
			reasonsJSON, err := json.Marshal(stringSliceAt(state, "reason_codes"))
			if err != nil {
				return store.SRZoneNormalizedProjections{}, fmt.Errorf("marshal event state reason_codes: %w", err)
			}
			projections.EventStates = append(projections.EventStates, store.MarketEventState{
				EventKey:        stringAt(state, "event_key"),
				EventType:       stringAt(state, "type"),
				EventFamily:     stringAt(state, "event_family"),
				EventScope:      stringAt(state, "event_scope"),
				ZoneKey:         stringAt(state, "zone_key"),
				RootEventType:   stringAt(state, "root_event_type"),
				LatestEventType: stringAt(state, "latest_event_type"),
				Direction:       stringAt(state, "direction"),
				State:           stringAt(state, "state"),
				Active:          boolAt(state, "active"),
				ResolvedBy:      nullString(anyStringAt(state, "resolved_by")),
				Confidence:      nullFloat(anyFloatAt(state, "confidence")),
				PriceLevel:      nullFloat(anyFloatAt(state, "price_level")),
				ReasonCodes:     store.RawJSON(reasonsJSON),
				StateJSON:       store.RawJSON(stateJSON),
			})
		}
	}

	if candidates, ok := sliceAt(decisionObj, "daily_candidate_zones"); ok {
		projections.DailyCandidates = make([]store.SRDailyCandidate, 0, len(candidates))
		for _, item := range candidates {
			candidate, ok := item.(map[string]any)
			if !ok {
				continue
			}
			candidateJSON, err := json.Marshal(candidate)
			if err != nil {
				return store.SRZoneNormalizedProjections{}, fmt.Errorf("marshal daily candidate: %w", err)
			}
			eventRefsJSON, err := json.Marshal(stringSliceAt(candidate, "event_refs"))
			if err != nil {
				return store.SRZoneNormalizedProjections{}, fmt.Errorf("marshal daily candidate event_refs: %w", err)
			}
			projections.DailyCandidates = append(projections.DailyCandidates, store.SRDailyCandidate{
				PriceLow:      floatAt(candidate, "price_low"),
				PriceHigh:     floatAt(candidate, "price_high"),
				Label:         stringAt(candidate, "label"),
				Role:          stringAt(candidate, "role"),
				Source:        stringAt(candidate, "source"),
				Lifecycle:     stringAt(candidate, "lifecycle"),
				DecisionRole:  stringAt(candidate, "decision_role"),
				DistancePct:   nullFloat(anyFloatAt(candidate, "distance_pct")),
				DistanceLabel: stringAt(candidate, "distance_label"),
				Reason:        stringAt(candidate, "reason"),
				EventRefs:     store.RawJSON(eventRefsJSON),
				CandidateJSON: store.RawJSON(candidateJSON),
			})
		}
	}
	modelGovernance, err := buildModelGovernanceProjection(probabilityContext)
	if err != nil {
		return store.SRZoneNormalizedProjections{}, err
	}
	projections.ModelGovernance = modelGovernance
	return projections, nil
}

func buildModelGovernanceProjection(probabilityContext json.RawMessage) (*store.SRModelGovernance, error) {
	probabilityObj, ok, err := decodeJSONObject(probabilityContext)
	if err != nil || !ok {
		return nil, err
	}
	health, ok := mapAt(probabilityObj, "health")
	if !ok {
		return nil, nil
	}
	healthJSON, err := json.Marshal(health)
	if err != nil {
		return nil, fmt.Errorf("marshal model governance health: %w", err)
	}
	qualityFlagsJSON, err := json.Marshal(stringSliceAt(health, "quality_flags"))
	if err != nil {
		return nil, fmt.Errorf("marshal model governance quality_flags: %w", err)
	}
	warningFlagsJSON, err := json.Marshal(stringSliceAt(health, "warning_flags"))
	if err != nil {
		return nil, fmt.Errorf("marshal model governance warning_flags: %w", err)
	}
	blockingFlagsJSON, err := json.Marshal(stringSliceAt(health, "blocking_flags"))
	if err != nil {
		return nil, fmt.Errorf("marshal model governance blocking_flags: %w", err)
	}
	confidenceGateJSON, err := marshalJSONObjectAt(health, "confidence_gate")
	if err != nil {
		return nil, err
	}
	calibrationReportJSON, err := marshalJSONObjectAt(probabilityObj, "model_reports", "calibration_report")
	if err != nil {
		return nil, err
	}
	walkForwardReportJSON, err := marshalJSONObjectAt(probabilityObj, "model_reports", "walk_forward_report")
	if err != nil {
		return nil, err
	}
	datasetDiagnosticsJSON, err := marshalJSONObjectAt(probabilityObj, "model_reports", "dataset_diagnostics")
	if err != nil {
		return nil, err
	}
	return &store.SRModelGovernance{
		HealthState:            stringAt(health, "health_state"),
		AverageEdgePP:          nullFloat(anyFloatAt(health, "average_edge_pp")),
		DirectionalZoneCount:   nullIntFromFloat(anyFloatAt(health, "directional_zone_count")),
		ZoneCount:              nullIntFromFloat(anyFloatAt(health, "zone_count")),
		AllowEntry:             nullBool(anyBoolAt(health, "confidence_gate", "allow_entry")),
		MaxEntryState:          stringAt(health, "confidence_gate", "max_entry_state"),
		QualityFlags:           store.RawJSON(qualityFlagsJSON),
		WarningFlags:           store.RawJSON(warningFlagsJSON),
		BlockingFlags:          store.RawJSON(blockingFlagsJSON),
		ConfidenceGateJSON:     store.RawJSON(confidenceGateJSON),
		CalibrationReportJSON:  store.RawJSON(calibrationReportJSON),
		WalkForwardReportJSON:  store.RawJSON(walkForwardReportJSON),
		DatasetDiagnosticsJSON: store.RawJSON(datasetDiagnosticsJSON),
		GovernanceJSON:         store.RawJSON(healthJSON),
	}, nil
}

func decodeJSONObject(raw json.RawMessage) (map[string]any, bool, error) {
	if len(raw) == 0 || !json.Valid(raw) || string(raw) == "null" {
		return nil, false, nil
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, false, fmt.Errorf("decode decision projection: %w", err)
	}
	return obj, true, nil
}

func collectDecisionReasonCodes(obj map[string]any) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(values []string) {
		for _, value := range values {
			if value == "" {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			out = append(out, value)
		}
	}
	add(stringSliceAt(obj, "final_entry_permission", "reason_codes"))
	add(stringSliceAt(obj, "price_path", "reason_codes"))
	add(stringSliceAt(obj, "model_governance", "confidence_gate", "reason_codes"))
	if events, ok := sliceAt(obj, "event_state_summary", "active_bearish_events"); ok {
		for _, item := range events {
			if event, ok := item.(map[string]any); ok {
				add(stringSliceAt(event, "reason_codes"))
			}
		}
	}
	return out
}

func valueAt(obj map[string]any, path ...string) (any, bool) {
	var current any = obj
	for _, key := range path {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = m[key]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func mapAt(obj map[string]any, path ...string) (map[string]any, bool) {
	value, ok := valueAt(obj, path...)
	if !ok {
		return nil, false
	}
	m, ok := value.(map[string]any)
	return m, ok
}

func marshalJSONObjectAt(obj map[string]any, path ...string) ([]byte, error) {
	value, ok := valueAt(obj, path...)
	if !ok || value == nil {
		return []byte("null"), nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal %q: %w", path[len(path)-1], err)
	}
	return data, nil
}

func decisionRawJSONAt(obj map[string]any, fallback string, path ...string) store.RawJSON {
	value, ok := valueAt(obj, path...)
	if !ok || value == nil {
		return store.RawJSON(fallback)
	}
	data, err := json.Marshal(value)
	if err != nil || !json.Valid(data) {
		return store.RawJSON(fallback)
	}
	return store.RawJSON(data)
}

func buildDecisionZoneSummariesJSON(obj map[string]any) store.RawJSON {
	out := map[string]any{
		"nearest_decision_zone":   nil,
		"nearest_support_zone":    nil,
		"nearest_resistance_zone": nil,
		"primary_structural_zone": nil,
		"best_trade_zone":         nil,
		"primary_zone":            nil,
		"secondary_zones":         []any{},
	}
	for _, key := range []string{
		"nearest_decision_zone",
		"nearest_support_zone",
		"nearest_resistance_zone",
		"primary_structural_zone",
		"best_trade_zone",
		"primary_zone",
		"secondary_zones",
	} {
		if value, ok := valueAt(obj, key); ok {
			out[key] = value
		}
	}
	data, err := json.Marshal(out)
	if err != nil || !json.Valid(data) {
		return store.RawJSON(`{"nearest_decision_zone":null,"nearest_support_zone":null,"nearest_resistance_zone":null,"primary_structural_zone":null,"best_trade_zone":null,"primary_zone":null,"secondary_zones":[]}`)
	}
	return store.RawJSON(data)
}

func stringAt(obj map[string]any, path ...string) string {
	value := anyStringAt(obj, path...)
	if value == nil {
		return ""
	}
	return *value
}

func anyStringAt(obj map[string]any, path ...string) *string {
	value, ok := valueAt(obj, path...)
	if !ok || value == nil {
		return nil
	}
	text, ok := value.(string)
	if !ok {
		return nil
	}
	return &text
}

func anyFloatAt(obj map[string]any, path ...string) *float64 {
	value, ok := valueAt(obj, path...)
	if !ok || value == nil {
		return nil
	}
	number, ok := value.(float64)
	if !ok {
		return nil
	}
	return &number
}

func anyBoolAt(obj map[string]any, path ...string) *bool {
	value, ok := valueAt(obj, path...)
	if !ok || value == nil {
		return nil
	}
	flag, ok := value.(bool)
	if !ok {
		return nil
	}
	return &flag
}

func floatAt(obj map[string]any, path ...string) float64 {
	value := anyFloatAt(obj, path...)
	if value == nil {
		return 0
	}
	return *value
}

func boolAt(obj map[string]any, path ...string) bool {
	value, ok := valueAt(obj, path...)
	if !ok {
		return false
	}
	flag, _ := value.(bool)
	return flag
}

func nullBool(value *bool) store.NullBool {
	if value == nil {
		return store.NullBool{NullBool: sql.NullBool{}}
	}
	return store.NullBool{NullBool: sql.NullBool{Bool: *value, Valid: true}}
}

func nullIntFromFloat(value *float64) store.NullInt64 {
	if value == nil {
		return store.NullInt64{NullInt64: sql.NullInt64{}}
	}
	return store.NullInt64{NullInt64: sql.NullInt64{Int64: int64(*value), Valid: true}}
}

func sliceAt(obj map[string]any, path ...string) ([]any, bool) {
	value, ok := valueAt(obj, path...)
	if !ok {
		return nil, false
	}
	items, ok := value.([]any)
	return items, ok
}

func stringSliceAt(obj map[string]any, path ...string) []string {
	items, ok := sliceAt(obj, path...)
	if !ok {
		return []string{}
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if text, ok := item.(string); ok && text != "" {
			out = append(out, text)
		}
	}
	return out
}

func validateTradingScoreBreakdown(b map[string]float64) error {
	required := []string{"expected_value", "risk_reward", "trend", "volume", "confidence", "chip"}
	for _, key := range required {
		if _, ok := b[key]; !ok {
			return fmt.Errorf("missing %q", key)
		}
	}
	return nil
}

func rawJSONOrDefault(raw json.RawMessage, fallback string) store.RawJSON {
	if len(raw) == 0 || !json.Valid(raw) {
		return store.RawJSON(fallback)
	}
	return store.RawJSON(string(raw))
}

// scoreZonesRequest 對應 Python ScoreZonesRequest；Limit 為 0 時省略欄位，
// 讓 Python 端套用它自己的預設值（理由同 analyzeRequest）。
type scoreZonesRequest struct {
	Symbol              string                         `json:"symbol"`
	Timeframe           string                         `json:"timeframe"`
	Limit               int                            `json:"limit,omitempty"`
	PreviousEventStates []scoreZonesPreviousEventState `json:"previous_event_states,omitempty"`
}

type scoreZonesPreviousEventState struct {
	EventKey          string            `json:"event_key"`
	Type              string            `json:"type"`
	EventFamily       string            `json:"event_family"`
	EventScope        string            `json:"event_scope"`
	ZoneKey           string            `json:"zone_key"`
	RootEventType     string            `json:"root_event_type"`
	LatestEventType   string            `json:"latest_event_type"`
	Direction         string            `json:"direction"`
	State             string            `json:"state"`
	Active            bool              `json:"active"`
	ConfirmationState string            `json:"confirmation_state,omitempty"`
	ExpiresAfterBars  *int              `json:"expires_after_bars,omitempty"`
	AgeBars           int               `json:"age_bars"`
	ZoneRef           map[string]any    `json:"zone_ref,omitempty"`
	ResolvedBy        store.NullString  `json:"resolved_by,omitempty"`
	Confidence        store.NullFloat64 `json:"confidence,omitempty"`
	PriceLevel        store.NullFloat64 `json:"price_level,omitempty"`
	ReasonCodes       store.RawJSON     `json:"reason_codes"`
}

// ScoreZones 呼叫 Python HTTP service 的 /sr-zones 端點。limit 為抓取的
// 歷史K棒根數，傳 0 表示使用 Python 端的預設值。
func (c *Client) ScoreZones(ctx context.Context, symbol, timeframe string, limit int) (*ZoneScoreResult, error) {
	return c.ScoreZonesWithPreviousEvents(ctx, symbol, timeframe, limit, nil)
}

func (c *Client) ScoreZonesWithPreviousEvents(
	ctx context.Context,
	symbol, timeframe string,
	limit int,
	previousEventStates []store.MarketEventState,
) (*ZoneScoreResult, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf("python service url not configured（請設定 python.service_url / PYTHON_SERVICE_URL）")
	}

	body, err := json.Marshal(scoreZonesRequest{
		Symbol:              symbol,
		Timeframe:           timeframe,
		Limit:               limit,
		PreviousEventStates: scoreZonesPreviousEventStates(previousEventStates),
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/sr-zones", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.srZonesHTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("python sr-zones request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("python sr-zones read body error: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &UpstreamStatusError{StatusCode: resp.StatusCode, Body: truncateBody(respBody)}
	}

	var result ZoneScoreResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("python sr-zones decode error: body=%s: %w", truncateBody(respBody), err)
	}
	return &result, nil
}

func scoreZonesPreviousEventStates(states []store.MarketEventState) []scoreZonesPreviousEventState {
	if len(states) == 0 {
		return nil
	}
	out := make([]scoreZonesPreviousEventState, 0, len(states))
	for _, state := range states {
		reasons := state.ReasonCodes
		if reasons == "" {
			reasons = store.RawJSON("[]")
		}
		stateJSON := rawJSONObject(state.StateJSON)
		out = append(out, scoreZonesPreviousEventState{
			EventKey:          state.EventKey,
			Type:              state.EventType,
			EventFamily:       state.EventFamily,
			EventScope:        state.EventScope,
			ZoneKey:           state.ZoneKey,
			RootEventType:     state.RootEventType,
			LatestEventType:   state.LatestEventType,
			Direction:         state.Direction,
			State:             state.State,
			Active:            state.Active,
			ConfirmationState: stringValueAt(stateJSON, "confirmation_state"),
			ExpiresAfterBars:  intPtrValueAt(stateJSON, "expires_after_bars"),
			AgeBars:           intValueAt(stateJSON, "age_bars"),
			ZoneRef:           objectValueAt(stateJSON, "zone_ref"),
			ResolvedBy:        state.ResolvedBy,
			Confidence:        state.Confidence,
			PriceLevel:        state.PriceLevel,
			ReasonCodes:       reasons,
		})
	}
	return out
}

func rawJSONObject(raw store.RawJSON) map[string]any {
	if raw == "" || raw == "null" {
		return nil
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return nil
	}
	return obj
}

func objectValueAt(obj map[string]any, key string) map[string]any {
	if obj == nil {
		return nil
	}
	value, ok := obj[key].(map[string]any)
	if !ok {
		return nil
	}
	return value
}

func stringValueAt(obj map[string]any, key string) string {
	if obj == nil {
		return ""
	}
	value, _ := obj[key].(string)
	return value
}

// intValueAt 讀 state_json 內的整數欄位（JSON 數字會被 decode 成 float64），缺值回 0。
func intValueAt(obj map[string]any, key string) int {
	if obj == nil {
		return 0
	}
	switch v := obj[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return 0
}

// intPtrValueAt 與 intValueAt 相同，但缺值/型別不符時回 nil——用於 expires_after_bars：
// 送 nil 讓 Python 端套自己的預設，而不是誤傳 0 造成立即過期。
func intPtrValueAt(obj map[string]any, key string) *int {
	if obj == nil {
		return nil
	}
	switch v := obj[key].(type) {
	case float64:
		n := int(v)
		return &n
	case int:
		return &v
	}
	return nil
}

// ── SR Scoring 模型訓練 ──────────────────────────────────────

// trainHTTPClient 用專屬、逾時時間長很多的 client：訓練要對每檔股票的完整
// 歷史資料做 walk-forward 特徵/label 計算再訓練 sklearn 模型，視股票數與
// 資料長度可能耗時數十秒到數分鐘，不能沿用 c.http 給 /analyze、/sr-zones
// 這種同步、應該秒回的端點用的 30 秒逾時。
var trainHTTPClient = &http.Client{Timeout: 10 * time.Minute}

type trainRequest struct {
	Symbols           []string `json:"symbols,omitempty"`
	Timeframe         string   `json:"timeframe,omitempty"`
	Limit             int      `json:"limit,omitempty"`
	ModelType         string   `json:"model_type,omitempty"`
	SplitMethod       string   `json:"split_method,omitempty"`
	CalibrationMethod string   `json:"calibration_method,omitempty"`
}

// TrainResult 對應 Python run_training() 的回傳格式。SplitMethod/
// DatasetSummary 是機率校準與時間序列切分改善的一部分（見
// sr-zone-scoring.md「模型驗證與校準」）：DatasetSummary 形狀不固定
// （symbol/role 分佈是 int、positive rate 是 float），故用
// map[string]interface{} 而非強型別 struct，原樣保存供診斷用。
type TrainResult struct {
	Rows           int                           `json:"rows"`
	Sources        int                           `json:"sources"`
	ModelType      string                        `json:"model_type"`
	SplitMethod    string                        `json:"split_method"`
	Metrics        map[string]map[string]float64 `json:"metrics"`
	ModelPath      string                        `json:"model_path"`
	TrainedAt      string                        `json:"trained_at"`
	Version        string                        `json:"version"`
	DatasetSummary map[string]interface{}        `json:"dataset_summary"`
}

type SREvaluationRequest struct {
	Symbols                 []string                    `json:"symbols"`
	Timeframe               string                      `json:"timeframe,omitempty"`
	Limit                   int                         `json:"limit,omitempty"`
	ModelPath               string                      `json:"model_path,omitempty"`
	WriteDB                 bool                        `json:"write_db,omitempty"`
	DecisionReplay          bool                        `json:"decision_replay,omitempty"`
	ReplayMaxRows           int                         `json:"replay_max_rows,omitempty"`
	RunID                   string                      `json:"run_id,omitempty"`
	PipelineVersion         string                      `json:"pipeline_version,omitempty"`
	Passed                  *bool                       `json:"passed,omitempty"`
	MinHistoryBars          int                         `json:"min_history_bars,omitempty"`
	RebuildEveryBars        int                         `json:"rebuild_every_bars,omitempty"`
	ForwardBars             int                         `json:"forward_bars,omitempty"`
	ThresholdPct            float64                     `json:"threshold_pct,omitempty"`
	ATRWidthMultiplier      float64                     `json:"atr_width_multiplier,omitempty"`
	MaxMergeWidthMultiple   float64                     `json:"max_merge_width_multiple,omitempty"`
	ATRLookback             int                         `json:"atr_lookback,omitempty"`
	ATRPeriod               int                         `json:"atr_period,omitempty"`
	ChipScoresBySymbol      map[string][]map[string]any `json:"chip_scores_by_symbol,omitempty"`
	ModelGovernanceBySymbol map[string][]map[string]any `json:"model_governance_by_symbol,omitempty"`
}

// TrainModel 呼叫 Python /sr-scoring/train 端點，重新訓練 bounce/break
// 機率模型。symbols 為空時由 Go 端呼叫者自行決定預設值（例如整個
// watchlist），這裡不做任何預設判斷。這是同步呼叫（等訓練完成才回應），
// 呼叫端應在背景 goroutine 執行，避免卡住 HTTP handler。
func (c *Client) TrainModel(ctx context.Context, symbols []string, timeframe string, limit int, modelType, splitMethod, calibrationMethod string) (*TrainResult, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf("python service url not configured（請設定 python.service_url / PYTHON_SERVICE_URL）")
	}

	body, err := json.Marshal(trainRequest{
		Symbols: symbols, Timeframe: timeframe, Limit: limit, ModelType: modelType,
		SplitMethod: splitMethod, CalibrationMethod: calibrationMethod,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/sr-scoring/train", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := trainHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("python sr-scoring train request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("python sr-scoring train read body error: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("python sr-scoring train error: status=%d body=%s", resp.StatusCode, truncateBody(respBody))
	}

	var result TrainResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("python sr-scoring train decode error: body=%s: %w", truncateBody(respBody), err)
	}
	return &result, nil
}

func (c *Client) RunSREvaluation(ctx context.Context, request SREvaluationRequest) (map[string]any, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf("python service url not configured（請設定 python.service_url / PYTHON_SERVICE_URL）")
	}

	body, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/sr-scoring/evaluate", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := trainHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("python sr-scoring evaluate request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("python sr-scoring evaluate read body error: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &UpstreamStatusError{StatusCode: resp.StatusCode, Body: truncateBody(respBody)}
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("python sr-scoring evaluate decode error: body=%s: %w", truncateBody(respBody), err)
	}
	return result, nil
}

// ModelStatus 對應 Python GET /sr-scoring/model-status 的回傳格式。跟
// TrainResult 不同：這支端點永遠回 200，用 Exists 表示模型存不存在，
// 不是像 /sr-zones 那樣在模型不存在時丟錯——目的是讓前端在呼叫
// POST /sr-zones 之前，先知道模型準備好了沒（見 sr-zone-scoring.md「模型
// 可追蹤性」）。Exists=false 時其餘欄位皆為 zero value。
type ModelStatus struct {
	Exists         bool                          `json:"exists"`
	Version        *string                       `json:"version"`
	TrainedAt      *string                       `json:"trained_at"`
	ModelPath      *string                       `json:"model_path"`
	SplitMethod    *string                       `json:"split_method"`
	Metrics        map[string]map[string]float64 `json:"metrics"`
	FeatureNames   []string                      `json:"feature_names"`
	ConfigHash     *string                       `json:"config_hash"`
	TrainingConfig map[string]interface{}        `json:"training_config"`
}

// GetModelStatus 呼叫 Python GET /sr-scoring/model-status 端點。用一般的
// c.http（30 秒 timeout），不是訓練用的長 timeout client，因為這只是查詢
// 現況，不會觸發任何訓練動作。
func (c *Client) GetModelStatus(ctx context.Context) (*ModelStatus, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf("python service url not configured（請設定 python.service_url / PYTHON_SERVICE_URL）")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/sr-scoring/model-status", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("python model-status request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("python model-status read body error: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("python model-status error: status=%d body=%s", resp.StatusCode, truncateBody(respBody))
	}

	var result ModelStatus
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("python model-status decode error: body=%s: %w", truncateBody(respBody), err)
	}
	return &result, nil
}

func truncateBody(body []byte) string {
	const maxLen = 300
	s := string(body)
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
