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
		EntryReason:          sql.NullString{String: r.Entry.Reason, Valid: r.Entry.Reason != ""},
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

func nullFloat(p *float64) sql.NullFloat64 {
	if p == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *p, Valid: true}
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

func (c *Client) Analyze(ctx context.Context, symbol, timeframe string) (*Result, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf("python service url not configured（請設定 python.service_url / PYTHON_SERVICE_URL）")
	}

	body, err := json.Marshal(map[string]string{"symbol": symbol, "timeframe": timeframe})
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

func truncateBody(body []byte) string {
	const maxLen = 300
	s := string(body)
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
