package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/analysis"
	"github.com/trading/backend/internal/store"
)

type srZoneRepoStub struct {
	analyses    []store.SRZoneAnalysis
	zones       map[uint64][]store.SRZone
	nextID      uint64
	createCalls int
}

func (s *srZoneRepoStub) Create(ctx context.Context, a *store.SRZoneAnalysis, zones []store.SRZone) (uint64, error) {
	s.createCalls++
	if s.nextID == 0 {
		s.nextID = 100
	}
	id := s.nextID
	s.nextID++
	a.ID = id
	s.analyses = append([]store.SRZoneAnalysis{*a}, s.analyses...)
	if s.zones == nil {
		s.zones = map[uint64][]store.SRZone{}
	}
	for i := range zones {
		zones[i].ID = uint64(i + 1)
		zones[i].AnalysisID = id
	}
	s.zones[id] = zones
	return id, nil
}

func (s *srZoneRepoStub) Get(ctx context.Context, id uint64) (*store.SRZoneAnalysis, error) {
	for i := range s.analyses {
		if s.analyses[i].ID == id {
			return &s.analyses[i], nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func (s *srZoneRepoStub) List(ctx context.Context, symbol string, limit int) ([]store.SRZoneAnalysis, error) {
	rows := make([]store.SRZoneAnalysis, 0, len(s.analyses))
	for _, a := range s.analyses {
		if symbol == "" || a.Symbol == symbol {
			rows = append(rows, a)
		}
	}
	return rows, nil
}

func (s *srZoneRepoStub) GetZones(ctx context.Context, analysisID uint64) ([]store.SRZone, error) {
	return s.zones[analysisID], nil
}

func (s *srZoneRepoStub) UpdateZoneStatus(ctx context.Context, zoneID uint64, status string, brokenAt *time.Time, brokenPrice *float64, resolvedRole string) error {
	return nil
}

func (s *srZoneRepoStub) Delete(ctx context.Context, id uint64) error {
	return nil
}

func srZoneScoreResponse(symbol string, analyzedAt time.Time) string {
	return fmt.Sprintf(`{
		"pipeline_version":"v2",
		"symbol":%q,
		"timeframe":"1d",
		"analyzed_at":%q,
		"current_price":100,
		"global_trend":0.1,
		"global_volatility":0.2,
		"model_version":"v-test",
		"model_config_hash":"hash-test",
		"period_summaries":[],
		"analysis_tips":[],
		"chip_summary":null,
		"decision_summary":{"action":"BuySmall"},
		"zones":[{
			"price_low":90,
			"price_high":95,
			"method":"atr",
			"role":"SUPPORT",
			"tier":"TIER_1_MAIN_STRUCTURE",
			"tier_label":"主結構",
			"support_score":0.8,
			"resistance_score":0.1,
			"net_score":0.7,
			"net_score_label":"STRONG_SUPPORT",
			"confidence":0.8,
			"confidence_level":"HIGH",
			"bounce_probability":0.7,
			"break_probability":0.2,
			"touch_count":4,
			"support_touch_count":4,
			"resistance_touch_count":0,
			"reject_count":3,
			"break_count":0,
			"zone_momentum":0,
			"zone_direction":"FLAT",
			"recent_validation":"PENDING_VALIDATION",
			"trading_score":70,
			"trading_score_breakdown":{"expected_value":20,"risk_reward":10,"trend":10,"volume":10,"confidence":10,"chip":10},
			"trading_recommendation":"BUY",
			"confluence_count":1
		}]
	}`, symbol, analyzedAt.Format(time.RFC3339))
}

func TestSRZoneCreateDefaultsToNewSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		if r.URL.Path != "/sr-zones" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, srZoneScoreResponse("2330", time.Now().UTC()))
	}))
	defer upstream.Close()

	repo := &srZoneRepoStub{nextID: 10}
	handler := NewSRZoneHandler(analysis.NewClient(upstream.URL), repo, nil, nil, nil, nil, zap.NewNop())
	router := gin.New()
	router.POST("/sr-zones", handler.Create)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/sr-zones", strings.NewReader(`{"symbol":"2330","timeframe":"1d","limit":250}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if upstreamCalls != 1 || repo.createCalls != 1 {
		t.Fatalf("expected direct create path, upstream=%d create=%d", upstreamCalls, repo.createCalls)
	}
	if !strings.Contains(rec.Body.String(), `"id":10`) {
		t.Fatalf("expected new analysis id in response: %s", rec.Body.String())
	}
}

func TestSRZoneCreateCanReuseExistingSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		fmt.Fprint(w, srZoneScoreResponse("2330", time.Now().UTC()))
	}))
	defer upstream.Close()

	now := time.Now().UTC()
	repo := &srZoneRepoStub{
		analyses: []store.SRZoneAnalysis{{
			ID: 9, Symbol: "2330", Timeframe: "1d", AnalyzedAt: now.Add(-time.Hour),
			CurrentPrice: 100, PipelineVersion: "v2", ModelVersion: "v-test",
		}},
		zones: map[uint64][]store.SRZone{9: {{
			ID: 1, AnalysisID: 9, PriceLow: 90, PriceHigh: 95, Method: "atr", Role: "SUPPORT",
			Status: "PENDING", TradingScore: 70,
		}}},
	}
	provider := analysis.NewSRAnalysisProvider(analysis.NewClient(upstream.URL), repo, 24*time.Hour)
	handler := NewSRZoneHandler(analysis.NewClient(upstream.URL), repo, nil, nil, nil, provider, zap.NewNop())
	router := gin.New()
	router.POST("/sr-zones", handler.Create)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/sr-zones", strings.NewReader(`{"symbol":"2330","timeframe":"1d","limit":250,"reuse_existing":true}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if upstreamCalls != 0 || repo.createCalls != 0 {
		t.Fatalf("expected reuse path without upstream/create, upstream=%d create=%d", upstreamCalls, repo.createCalls)
	}
	if !strings.Contains(rec.Body.String(), `"id":9`) {
		t.Fatalf("expected reused analysis id in response: %s", rec.Body.String())
	}
}
