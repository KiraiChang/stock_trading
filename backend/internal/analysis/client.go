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
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/trading/backend/internal/store"
)

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

// Client 呼叫 Python HTTP service 的 /analyze 端點
type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 30 * time.Second},
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
	PriceLow              float64            `json:"price_low"`
	PriceHigh             float64            `json:"price_high"`
	Method                string             `json:"method"`
	Role                  string             `json:"role"`
	Tier                  string             `json:"tier"`
	TierLabel             string             `json:"tier_label"`
	SupportScore          float64            `json:"support_score"`
	ResistanceScore       float64            `json:"resistance_score"`
	NetScore              float64            `json:"net_score"`
	NetScoreLabel         string             `json:"net_score_label"`
	Confidence            float64            `json:"confidence"`
	ConfidenceLevel       string             `json:"confidence_level"`
	BounceProbability     *float64           `json:"bounce_probability"`
	BreakProbability      *float64           `json:"break_probability"`
	ExpectedGain          *float64           `json:"expected_gain"`
	ExpectedLoss          *float64           `json:"expected_loss"`
	ExpectedValue         *float64           `json:"expected_value"`
	RiskRewardRatio       *float64           `json:"risk_reward_ratio"`
	RewardRiskPercentile  *float64           `json:"reward_risk_percentile"`
	RelativeVolume        *float64           `json:"relative_volume"`
	VolumeConfirmation    *string            `json:"volume_confirmation"`
	TouchCount            int                `json:"touch_count"`
	RejectCount           *int               `json:"reject_count"`
	BreakCount            *int               `json:"break_count"`
	ZoneMomentum          float64            `json:"zone_momentum"`
	ZoneDirection         string             `json:"zone_direction"`
	RecentValidation      string             `json:"recent_validation"`
	TradingScore          float64            `json:"trading_score"`
	TradingScoreBreakdown map[string]float64 `json:"trading_score_breakdown"`
	TradingRecommendation string             `json:"trading_recommendation"`
}

// ZoneScoreResult 對應 Python score_symbol() 的回傳格式。GlobalTrend/
// GlobalVolatility/GlobalExpectedValue/GlobalConfidence/GlobalRiskRewardRatio
// 是「只有一個 Global Model」的整體評估區塊，只在這裡出現一次，不會在每個
// Zone 裡重複（見 sr_scoring 套件說明的「九、十、十二」）。
type ZoneScoreResult struct {
	Symbol                string      `json:"symbol"`
	Timeframe             string      `json:"timeframe"`
	AnalyzedAt            string      `json:"analyzed_at"` // RFC3339
	CurrentPrice          float64     `json:"current_price"`
	GlobalTrend           float64     `json:"global_trend"`
	GlobalVolatility      float64     `json:"global_volatility"`
	GlobalExpectedValue   *float64    `json:"global_expected_value"`
	GlobalConfidence      *float64    `json:"global_confidence"`
	GlobalRiskRewardRatio *float64    `json:"global_risk_reward_ratio"`
	Zones                 []ZoneScore `json:"zones"`
}

// ToStore 把 Python 回傳的 zone 評分結果轉成可以直接寫入 DB 的型別。
// ModelVersion 目前固定為空字串——Python bundle 的 version 沒有隨每次
// /sr-zones 回應一併回傳，之後如需追蹤模型版本可擴充 Python 端輸出。
func (r *ZoneScoreResult) ToStore() (*store.SRZoneAnalysis, []store.SRZone, error) {
	analyzedAt, err := time.Parse(time.RFC3339, r.AnalyzedAt)
	if err != nil {
		return nil, nil, fmt.Errorf("parse analyzed_at %q: %w", r.AnalyzedAt, err)
	}

	a := &store.SRZoneAnalysis{
		Symbol:                r.Symbol,
		Timeframe:             r.Timeframe,
		AnalyzedAt:            analyzedAt,
		CurrentPrice:          r.CurrentPrice,
		GlobalTrend:           r.GlobalTrend,
		GlobalVolatility:      r.GlobalVolatility,
		GlobalExpectedValue:   nullFloat(r.GlobalExpectedValue),
		GlobalConfidence:      nullFloat(r.GlobalConfidence),
		GlobalRiskRewardRatio: nullFloat(r.GlobalRiskRewardRatio),
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
		breakdownJSON, err := json.Marshal(z.TradingScoreBreakdown)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal trading_score_breakdown: %w", err)
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
			RejectCount:           rejectCount,
			BreakCount:            breakCount,
			ZoneMomentum:          z.ZoneMomentum,
			ZoneDirection:         z.ZoneDirection,
			RecentValidation:      z.RecentValidation,
			TradingScore:          z.TradingScore,
			TradingScoreBreakdown: store.RawJSON(breakdownJSON),
			TradingRecommendation: z.TradingRecommendation,
			Status:                "PENDING",
		})
	}

	return a, zones, nil
}

// scoreZonesRequest 對應 Python ScoreZonesRequest；Limit 為 0 時省略欄位，
// 讓 Python 端套用它自己的預設值（理由同 analyzeRequest）。
type scoreZonesRequest struct {
	Symbol    string `json:"symbol"`
	Timeframe string `json:"timeframe"`
	Limit     int    `json:"limit,omitempty"`
}

// ScoreZones 呼叫 Python HTTP service 的 /sr-zones 端點。limit 為抓取的
// 歷史K棒根數，傳 0 表示使用 Python 端的預設值。
func (c *Client) ScoreZones(ctx context.Context, symbol, timeframe string, limit int) (*ZoneScoreResult, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf("python service url not configured（請設定 python.service_url / PYTHON_SERVICE_URL）")
	}

	body, err := json.Marshal(scoreZonesRequest{Symbol: symbol, Timeframe: timeframe, Limit: limit})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/sr-zones", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("python sr-zones request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("python sr-zones read body error: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("python sr-zones error: status=%d body=%s", resp.StatusCode, truncateBody(respBody))
	}

	var result ZoneScoreResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("python sr-zones decode error: body=%s: %w", truncateBody(respBody), err)
	}
	return &result, nil
}

// ── SR Scoring 模型訓練 ──────────────────────────────────────

// trainHTTPClient 用專屬、逾時時間長很多的 client：訓練要對每檔股票的完整
// 歷史資料做 walk-forward 特徵/label 計算再訓練 sklearn 模型，視股票數與
// 資料長度可能耗時數十秒到數分鐘，不能沿用 c.http 給 /analyze、/sr-zones
// 這種同步、應該秒回的端點用的 30 秒逾時。
var trainHTTPClient = &http.Client{Timeout: 10 * time.Minute}

type trainRequest struct {
	Symbols   []string `json:"symbols,omitempty"`
	Timeframe string   `json:"timeframe,omitempty"`
	Limit     int      `json:"limit,omitempty"`
	ModelType string   `json:"model_type,omitempty"`
}

// TrainResult 對應 Python run_training() 的回傳格式
type TrainResult struct {
	Rows      int                           `json:"rows"`
	Sources   int                           `json:"sources"`
	ModelType string                        `json:"model_type"`
	Metrics   map[string]map[string]float64 `json:"metrics"`
	ModelPath string                        `json:"model_path"`
	TrainedAt string                        `json:"trained_at"`
	Version   string                        `json:"version"`
}

// TrainModel 呼叫 Python /sr-scoring/train 端點，重新訓練 bounce/break
// 機率模型。symbols 為空時由 Go 端呼叫者自行決定預設值（例如整個
// watchlist），這裡不做任何預設判斷。這是同步呼叫（等訓練完成才回應），
// 呼叫端應在背景 goroutine 執行，避免卡住 HTTP handler。
func (c *Client) TrainModel(ctx context.Context, symbols []string, timeframe string, limit int, modelType string) (*TrainResult, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf("python service url not configured（請設定 python.service_url / PYTHON_SERVICE_URL）")
	}

	body, err := json.Marshal(trainRequest{Symbols: symbols, Timeframe: timeframe, Limit: limit, ModelType: modelType})
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

func truncateBody(body []byte) string {
	const maxLen = 300
	s := string(body)
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
