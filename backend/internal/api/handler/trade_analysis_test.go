package handler

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/portfolio"
	"github.com/trading/backend/internal/store"
)

type tradeAnalyzerStub struct {
	result *portfolio.AnalyzeResult
	err    error
	symbol string
	opts   portfolio.AnalyzeOptions
}

func (s *tradeAnalyzerStub) Analyze(ctx context.Context, symbol string, opts portfolio.AnalyzeOptions) (*portfolio.AnalyzeResult, error) {
	s.symbol = symbol
	s.opts = opts
	return s.result, s.err
}

type tradePositionRepoStub struct {
	analyses []store.PositionAnalysis
	err      error
}

func (s *tradePositionRepoStub) List(context.Context, uint64) ([]store.Position, error) {
	return nil, nil
}
func (s *tradePositionRepoStub) Get(context.Context, uint64, string) (*store.Position, error) {
	return nil, nil
}
func (s *tradePositionRepoStub) ListTransactions(context.Context, uint64, string, int) ([]store.PositionTransaction, error) {
	return nil, nil
}
func (s *tradePositionRepoStub) ApplyEvent(context.Context, uint64, *store.PositionTransaction, int64) (*store.Position, error) {
	return nil, nil
}
func (s *tradePositionRepoStub) CreateAnalysis(context.Context, *store.PositionAnalysis) (uint64, error) {
	return 0, nil
}
func (s *tradePositionRepoStub) GetAnalysis(context.Context, uint64, uint64) (*store.PositionAnalysis, error) {
	return nil, nil
}
func (s *tradePositionRepoStub) ListAnalyses(context.Context, uint64, string, int) ([]store.PositionAnalysis, error) {
	return s.analyses, s.err
}

func TestTradeAnalysisAnalyzeContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name            string
		analysis        *store.PositionAnalysis
		wantHasPosition string
		wantState       string
	}{
		{
			name: "flat analysis is valid context",
			analysis: &store.PositionAnalysis{
				Symbol: "2330", PositionState: portfolio.StateFlat, Shares: 0,
				AnalyzedAt: time.Now(), CreatedAt: time.Now(),
			},
			wantHasPosition: `"has_position":false`,
			wantState:       `"position_state":"FLAT"`,
		},
		{
			name: "long analysis reports held position",
			analysis: &store.PositionAnalysis{
				Symbol: "2330", PositionState: portfolio.StateLong, Shares: 100,
				AnalyzedAt: time.Now(), CreatedAt: time.Now(),
			},
			wantHasPosition: `"has_position":true`,
			wantState:       `"position_state":"LONG"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := &tradeAnalyzerStub{result: &portfolio.AnalyzeResult{
				Analysis: tt.analysis,
				SR:       &store.SRZoneAnalysis{ID: 9, Symbol: "2330"},
				Zones:    []store.SRZone{{ID: 1, AnalysisID: 9}},
			}}
			h := &TradeAnalysisHandler{repo: &tradePositionRepoStub{}, analyzer: analyzer, log: zap.NewNop()}
			router := gin.New()
			router.POST("/trade-analysis/analyze", h.Analyze)

			req := httptest.NewRequest(http.MethodPost, "/trade-analysis/analyze", bytes.NewBufferString(`{"symbol":" 2330 ","limit":250,"portfolio_id":2}`))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusCreated {
				t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
			}
			body := rec.Body.String()
			if !strings.Contains(body, tt.wantHasPosition) || !strings.Contains(body, tt.wantState) {
				t.Fatalf("unexpected context body: %s", body)
			}
			if analyzer.symbol != "2330" || analyzer.opts.Limit != 250 || analyzer.opts.PortfolioID != 2 {
				t.Fatalf("analyzer input = symbol %q opts %+v", analyzer.symbol, analyzer.opts)
			}
		})
	}
}

func TestTradeAnalysisListHistory(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &tradePositionRepoStub{analyses: []store.PositionAnalysis{{ID: 1, Symbol: "2330"}}}
	h := &TradeAnalysisHandler{repo: repo, analyzer: &tradeAnalyzerStub{}, log: zap.NewNop()}
	router := gin.New()
	router.GET("/trade-analysis/:symbol/history", h.ListHistory)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/trade-analysis/2330/history?portfolio_id=2", nil))

	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"total":1`) {
		t.Fatalf("unexpected history response: %d %s", rec.Code, rec.Body.String())
	}
}

func TestTradeAnalysisInternalErrorsDoNotLeak(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &TradeAnalysisHandler{
		repo:     &tradePositionRepoStub{},
		analyzer: &tradeAnalyzerStub{err: errors.New("driver: bad connection secret")},
		log:      zap.NewNop(),
	}
	router := gin.New()
	router.POST("/trade-analysis/analyze", h.Analyze)

	req := httptest.NewRequest(http.MethodPost, "/trade-analysis/analyze", bytes.NewBufferString(`{"symbol":"2330","portfolio_id":2}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "internal server error") {
		t.Fatalf("unexpected error mapping: %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "secret") {
		t.Fatalf("internal detail leaked: %s", rec.Body.String())
	}
}

func TestTradeAnalysisRequiresPortfolioID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &TradeAnalysisHandler{repo: &tradePositionRepoStub{}, analyzer: &tradeAnalyzerStub{}, log: zap.NewNop()}
	router := gin.New()
	router.POST("/trade-analysis/analyze", h.Analyze)
	router.GET("/trade-analysis/:symbol/history", h.ListHistory)

	req := httptest.NewRequest(http.MethodPost, "/trade-analysis/analyze", bytes.NewBufferString(`{"symbol":"2330"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "portfolio_id is required") {
		t.Fatalf("unexpected analyze response: %d %s", rec.Code, rec.Body.String())
	}

	historyRec := httptest.NewRecorder()
	router.ServeHTTP(historyRec, httptest.NewRequest(http.MethodGet, "/trade-analysis/2330/history", nil))
	if historyRec.Code != http.StatusBadRequest || !strings.Contains(historyRec.Body.String(), "portfolio_id is required") {
		t.Fatalf("unexpected history response: %d %s", historyRec.Code, historyRec.Body.String())
	}
}
