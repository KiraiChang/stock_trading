package handler

import (
	"context"
	"database/sql"
	"encoding/json"
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

func TestMapScoreZonesErrorReturnsGatewayTimeoutForClientDeadline(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	err := fmt.Errorf("python sr-zones request failed: %w", context.DeadlineExceeded)
	mapScoreZonesError(c, zap.NewNop(), err)

	if w.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected status=504, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "SR Zone 分析逾時") {
		t.Fatalf("expected timeout message, got body=%s", w.Body.String())
	}
}

// fakeNetTimeoutErr 是一個 Timeout()==true 但不 wrap context.DeadlineExceeded 的
// net.Error，用來釘住 mapScoreZonesError 的 net.Error.Timeout() belt-and-suspenders
// 分支（不依賴特定 Go 版本把 Client.Timeout unwrap 成 DeadlineExceeded 的行為）。
type fakeNetTimeoutErr struct{}

func (fakeNetTimeoutErr) Error() string   { return "i/o timeout" }
func (fakeNetTimeoutErr) Timeout() bool   { return true }
func (fakeNetTimeoutErr) Temporary() bool { return false }

func TestMapScoreZonesErrorReturnsGatewayTimeoutForNetTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	err := fmt.Errorf("python sr-zones request failed: %w", fakeNetTimeoutErr{})
	mapScoreZonesError(c, zap.NewNop(), err)

	if w.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected status=504, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "SR Zone 分析逾時") {
		t.Fatalf("expected timeout message, got body=%s", w.Body.String())
	}
}

type srZoneRepoStub struct {
	eventStateHistory []store.MarketEventState
	analysisSnapshots []store.AnalysisSnapshot
	analyses          []store.SRZoneAnalysis
	zones             map[uint64][]store.SRZone
	decisions         map[uint64]*store.SRDecision
	eventDetections   map[uint64][]store.MarketEventDetection
	eventStates       map[uint64][]store.MarketEventState
	latestEventStates []store.MarketEventState
	dailyCandidates   map[uint64][]store.SRDailyCandidate
	modelGovernances  map[uint64]*store.SRModelGovernance
	nextID            uint64
	createCalls       int
}

func (s *srZoneRepoStub) Create(ctx context.Context, a *store.SRZoneAnalysis, zones []store.SRZone, projections store.SRZoneNormalizedProjections) (uint64, error) {
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
	if projections.Decision != nil {
		projections.Decision.AnalysisID = id
		if s.decisions == nil {
			s.decisions = map[uint64]*store.SRDecision{}
		}
		s.decisions[id] = projections.Decision
	}
	if len(projections.EventDetections) > 0 {
		if s.eventDetections == nil {
			s.eventDetections = map[uint64][]store.MarketEventDetection{}
		}
		for i := range projections.EventDetections {
			projections.EventDetections[i].AnalysisID = id
		}
		s.eventDetections[id] = projections.EventDetections
	}
	if len(projections.EventStates) > 0 {
		if s.eventStates == nil {
			s.eventStates = map[uint64][]store.MarketEventState{}
		}
		for i := range projections.EventStates {
			projections.EventStates[i].AnalysisID = id
		}
		s.eventStates[id] = projections.EventStates
	}
	if len(projections.DailyCandidates) > 0 {
		if s.dailyCandidates == nil {
			s.dailyCandidates = map[uint64][]store.SRDailyCandidate{}
		}
		for i := range projections.DailyCandidates {
			projections.DailyCandidates[i].AnalysisID = id
		}
		s.dailyCandidates[id] = projections.DailyCandidates
	}
	if projections.ModelGovernance != nil {
		projections.ModelGovernance.AnalysisID = id
		if s.modelGovernances == nil {
			s.modelGovernances = map[uint64]*store.SRModelGovernance{}
		}
		s.modelGovernances[id] = projections.ModelGovernance
	}
	return id, nil
}

func (s *srZoneRepoStub) GetDecision(ctx context.Context, analysisID uint64) (*store.SRDecision, error) {
	if row := s.decisions[analysisID]; row != nil {
		return row, nil
	}
	return nil, sql.ErrNoRows
}

func (s *srZoneRepoStub) GetMarketEventDetections(ctx context.Context, analysisID uint64) ([]store.MarketEventDetection, error) {
	return s.eventDetections[analysisID], nil
}

func (s *srZoneRepoStub) GetMarketEventStates(ctx context.Context, analysisID uint64) ([]store.MarketEventState, error) {
	return s.eventStates[analysisID], nil
}

func (s *srZoneRepoStub) GetLatestMarketEventStates(ctx context.Context, symbol, timeframe string) ([]store.MarketEventState, error) {
	return s.latestEventStates, nil
}

func (s *srZoneRepoStub) GetDailyCandidates(ctx context.Context, analysisID uint64) ([]store.SRDailyCandidate, error) {
	return s.dailyCandidates[analysisID], nil
}

func (s *srZoneRepoStub) GetModelGovernance(ctx context.Context, analysisID uint64) (*store.SRModelGovernance, error) {
	if row := s.modelGovernances[analysisID]; row != nil {
		return row, nil
	}
	return nil, sql.ErrNoRows
}

func (s *srZoneRepoStub) Get(ctx context.Context, id uint64) (*store.SRZoneAnalysis, error) {
	for i := range s.analyses {
		if s.analyses[i].ID == id {
			return &s.analyses[i], nil
		}
	}
	return nil, fmt.Errorf("not found")
}

// 排程用的 timeframe-aware 查詢（todo.md T-052）。這些 stub 沒有用到它。
func (s *srZoneRepoStub) GetLatestByTimeframe(ctx context.Context, symbol, timeframe string) (*store.SRZoneAnalysis, error) {
	return nil, nil
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
		"decision_summary":{
			"action":"BuySmall",
			"market_bias":"LEGACY_BIAS",
			"position_action":"LEGACY_HOLD",
			"final_entry_permission":{"state":"LEGACY_WAIT"},
			"price_path":{"path_state":"LEGACY_PATH"},
			"model_governance":{"health_state":"LEGACY_UNKNOWN"},
			"event_state_summary":{
				"market_state":"LEGACY_NORMAL",
				"states":[{
					"type":"HIGH_VOLUME_BREAKDOWN",
					"event_family":"BREAKDOWN",
					"event_scope":"ZONE",
					"event_key":"ZONE:BREAKDOWN:SUPPORT:90.0000:95.0000",
					"zone_key":"SUPPORT:90.0000:95.0000",
					"root_event_type":"HIGH_VOLUME_BREAKDOWN",
					"latest_event_type":"HIGH_VOLUME_BREAKDOWN",
					"direction":"BEARISH",
					"state":"ACTIVE",
					"active":true,
					"confidence":0.72,
					"price_level":90,
					"reason_codes":["HIGH_VOLUME_BREAKDOWN"]
				}]
			},
			"market_events":[{
				"type":"HIGH_VOLUME_BREAKDOWN",
				"event_family":"BREAKDOWN",
				"event_scope":"ZONE",
				"event_key":"ZONE:BREAKDOWN:SUPPORT:90.0000:95.0000",
				"zone_key":"SUPPORT:90.0000:95.0000",
				"direction":"BEARISH",
				"state":"ACTIVE",
				"active":true,
				"confidence":0.72,
				"price_level":90,
				"reason_codes":["HIGH_VOLUME_BREAKDOWN"]
			}],
			"daily_candidate_zones":[{
				"price_low":89.5,
				"price_high":91,
				"label":"89.50 ~ 91.00",
				"role":"SUPPORT",
				"source":"DAILY_CANDLE",
				"lifecycle":"CANDIDATE",
				"decision_role":"TACTICAL",
				"distance_pct":0.01,
				"distance_label":"1.0%%",
				"reason":"日 K 低點與收盤位置形成的短線支撐候選。",
				"event_refs":["HIGH_VOLUME_BREAKDOWN"]
			}]
		},
		"explanation":{"summary":"建議小量試單"},
		"scenario":{"schema_version":"sr_scenario_v1","state":"BuySmall"},
		"probability_context":{
			"schema_version":"sr_probability_context_v1",
			"health":{
				"health_state":"DEGRADED",
				"average_edge_pp":12.5,
				"directional_zone_count":1,
				"zone_count":1,
				"quality_flags":["HOLD_NOT_CALIBRATED"],
				"warning_flags":["LOW_AVERAGE_EDGE"],
				"blocking_flags":[],
				"confidence_gate":{"state":"DEGRADED","allow_entry":true,"max_entry_state":"SMALL_ENTRY","reason_codes":["LOW_AVERAGE_EDGE"]}
			},
			"model_reports":{
				"calibration_report":{"schema_version":"sr_calibration_report_v1"},
				"walk_forward_report":{"schema_version":"sr_walk_forward_report_v1"},
				"dataset_diagnostics":{"schema_version":"sr_dataset_diagnostics_v1"}
			}
		},
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
			"confluence_count":1,
			"explanation":{"role_summary":"此區為支撐"},
			"scenario":{"schema_version":"sr_scenario_v1","state":"SUPPORT_RETEST"},
			"probability_context":{"schema_version":"sr_probability_context_v1","dominant_outcome":"BOUNCE"}
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
	if !strings.Contains(rec.Body.String(), `"explanation":{"summary":"建議小量試單"}`) {
		t.Fatalf("expected explanation in response: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"scenario":{"schema_version":"sr_scenario_v1","state":"BuySmall"}`) {
		t.Fatalf("expected scenario in response: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"normalized_status":{"daily_candidates":"normalized","decision":"normalized","events":"normalized","model_governance":"normalized"}`) {
		t.Fatalf("expected normalized_status in response: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"health_state":"DEGRADED"`) {
		t.Fatalf("expected probability_context in response: %s", rec.Body.String())
	}

	// T-023：zone 的 "score" 只帶評分欄位，不再重複帶已在 data/lifecycle/兄弟鍵
	// 提供的欄位（features/evidence/explanation/scenario/price_low/id/role/status…）。
	var parsed struct {
		Zones []struct {
			Data  map[string]json.RawMessage `json:"data"`
			Score map[string]json.RawMessage `json:"score"`
		} `json:"zones"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(parsed.Zones) != 1 {
		t.Fatalf("expected 1 zone, got %d", len(parsed.Zones))
	}
	score := parsed.Zones[0].Score
	for _, k := range []string{"features", "evidence", "explanation", "scenario", "probability_context", "price_low", "id", "role", "status"} {
		if _, dup := score[k]; dup {
			t.Fatalf("zone score should not duplicate %q", k)
		}
	}
	if _, ok := score["trading_score"]; !ok {
		t.Fatalf("zone score should keep scoring fields like trading_score: %v", score)
	}
	if _, ok := parsed.Zones[0].Data["price_low"]; !ok {
		t.Fatalf("zone data should still carry price_low")
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

func TestSRZoneGetUsesNormalizedRowsForDecisionAndModelGovernance(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC().Truncate(time.Second)
	repo := &srZoneRepoStub{
		analyses: []store.SRZoneAnalysis{{
			ID: 42, Symbol: "2330", Timeframe: "1d", AnalyzedAt: now,
			CurrentPrice: 100, PipelineVersion: "v2", ModelVersion: "v-test", ModelConfigHash: "hash-test",
			DecisionSummary:    store.RawJSON(`{"market_bias":"LEGACY_BIAS","position_action":"LEGACY_HOLD","final_entry_permission":{"state":"LEGACY_WAIT"},"price_path":{"path_state":"LEGACY_PATH","next_decision_price":999},"rr_gate":{"reason_code":"LEGACY_RR"},"primary_zone":{"label":"LEGACY_ZONE"},"confidence_explanation":{"label":"LEGACY_CONFIDENCE"},"model_governance":{"health_state":"LEGACY_UNKNOWN"},"event_state_summary":{"market_state":"LEGACY_NORMAL"},"market_events":[],"daily_candidate_zones":[]}`),
			ProbabilityContext: store.RawJSON(`{"schema_version":"sr_probability_context_v1","health":{"health_state":"LEGACY_UNKNOWN","directional_zone_count":0},"model_reports":{}}`),
		}},
		zones: map[uint64][]store.SRZone{42: {{
			ID: 1, AnalysisID: 42, PriceLow: 90, PriceHigh: 95, Method: "atr", Role: "SUPPORT",
			Status: "PENDING", TradingScore: 70,
		}}},
		decisions: map[uint64]*store.SRDecision{42: {
			AnalysisID: 42, MarketBias: "BEARISH_BIAS", EntryPermissionState: "WAIT_CONFIRMATION",
			PositionAction: "REDUCE_ON_BREAKDOWN", PricePathState: "EVENT_RISK",
			ModelHealthState: "DEGRADED", EventMarketState: "BREAKDOWN_RISK",
			ReasonCodes:               store.RawJSON(`["HIGH_VOLUME_BREAKDOWN"]`),
			DecisionDerivedViewJSON:   store.RawJSON(`{"version":"decision-derived-view-p0","bias_state":"BEARISH_BIAS","bias_reason_codes":["MARKET_ACTION_AVOID"]}`),
			PricePathJSON:             store.RawJSON(`{"path_state":"EVENT_RISK","next_decision_price":581}`),
			EntryExecutabilityJSON:    store.RawJSON(`{"entry_price":581,"executable_now":true,"reason_code":"EXECUTABLE_NOW"}`),
			EntryBlockingZoneJSON:     store.RawJSON(`{"blocked":false,"distance_price":12,"threshold_price":3}`),
			RRGateJSON:                store.RawJSON(`{"minimum_rr":1.5,"actual_rr":2.4,"qualified":true,"reason_code":"RR_OK"}`),
			ConfidenceExplanationJSON: store.RawJSON(`{"value":0.72,"level":"HIGH","label":"高","formula_factors":[],"context_factors":[]}`),
			ZoneSummariesJSON:         store.RawJSON(`{"nearest_decision_zone":null,"nearest_support_zone":null,"nearest_resistance_zone":null,"primary_structural_zone":null,"best_trade_zone":{"label":"580.00 ~ 585.00"},"primary_zone":{"label":"580.00 ~ 585.00","role":"SUPPORT"},"secondary_zones":[]}`),
		}},
		eventDetections: map[uint64][]store.MarketEventDetection{42: {{
			EventKey: "event-1", EventType: "HIGH_VOLUME_BREAKDOWN", EventFamily: "BREAKDOWN",
			EventScope: "ZONE", ZoneKey: "SUPPORT:90.0000:95.0000", Direction: "BEARISH",
			State: "ACTIVE", Active: true, ReasonCodes: store.RawJSON(`["HIGH_VOLUME_BREAKDOWN"]`),
			EventJSON: store.RawJSON(`{"type":"LEGACY_EVENT"}`),
		}}},
		eventStates: map[uint64][]store.MarketEventState{42: {{
			EventKey: "event-1", EventType: "HIGH_VOLUME_BREAKDOWN", EventFamily: "BREAKDOWN",
			EventScope: "ZONE", ZoneKey: "SUPPORT:90.0000:95.0000", RootEventType: "HIGH_VOLUME_BREAKDOWN",
			LatestEventType: "HIGH_VOLUME_BREAKDOWN", Direction: "BEARISH", State: "ACTIVE", Active: true,
			ReasonCodes: store.RawJSON(`["HIGH_VOLUME_BREAKDOWN"]`), StateJSON: store.RawJSON(`{"type":"LEGACY_STATE"}`),
		}}},
		dailyCandidates: map[uint64][]store.SRDailyCandidate{42: {{
			PriceLow: 89.5, PriceHigh: 91, Label: "89.50 ~ 91.00", Role: "SUPPORT",
			Source: "DAILY_CANDLE", Lifecycle: "CANDIDATE", DecisionRole: "TACTICAL",
			DistancePct: store.NullFloat64{NullFloat64: sql.NullFloat64{Float64: 0.01, Valid: true}},
			EventRefs:   store.RawJSON(`["HIGH_VOLUME_BREAKDOWN"]`),
		}}},
		modelGovernances: map[uint64]*store.SRModelGovernance{42: {
			AnalysisID: 42, HealthState: "DEGRADED",
			AverageEdgePP:        store.NullFloat64{NullFloat64: sql.NullFloat64{Float64: 12.5, Valid: true}},
			DirectionalZoneCount: store.NullInt64{NullInt64: sql.NullInt64{Int64: 1, Valid: true}},
			ZoneCount:            store.NullInt64{NullInt64: sql.NullInt64{Int64: 2, Valid: true}},
			AllowEntry:           store.NullBool{NullBool: sql.NullBool{Bool: true, Valid: true}},
			MaxEntryState:        "SMALL_ENTRY",
			QualityFlags:         store.RawJSON(`["HOLD_NOT_CALIBRATED"]`),
			WarningFlags:         store.RawJSON(`["LOW_AVERAGE_EDGE"]`),
			BlockingFlags:        store.RawJSON(`[]`),
			ConfidenceGateJSON:   store.RawJSON(`{"state":"DEGRADED","allow_entry":true,"max_entry_state":"SMALL_ENTRY"}`),
		}},
	}
	handler := NewSRZoneHandler(nil, repo, nil, nil, nil, nil, zap.NewNop())
	router := gin.New()
	router.GET("/sr-zones/:id", handler.Get)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sr-zones/42", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var parsed struct {
		Decision struct {
			MarketBias           string `json:"market_bias"`
			PositionAction       string `json:"position_action"`
			FinalEntryPermission struct {
				State string `json:"state"`
			} `json:"final_entry_permission"`
			MarketEvents []struct {
				Type string `json:"type"`
			} `json:"market_events"`
			DecisionDerivedView struct {
				Version   string `json:"version"`
				BiasState string `json:"bias_state"`
			} `json:"decision_derived_view"`
			EventStateSummary struct {
				MarketState         string           `json:"market_state"`
				ActiveBearishEvents []map[string]any `json:"active_bearish_events"`
			} `json:"event_state_summary"`
			DailyCandidateZones []struct {
				Source string `json:"source"`
			} `json:"daily_candidate_zones"`
			PricePath struct {
				NextDecisionPrice float64 `json:"next_decision_price"`
			} `json:"price_path"`
			EntryExecutability struct {
				ReasonCode string `json:"reason_code"`
			} `json:"entry_executability"`
			EntryBlockingZone struct {
				DistancePrice float64 `json:"distance_price"`
			} `json:"entry_blocking_zone"`
			RRGate struct {
				ReasonCode string `json:"reason_code"`
			} `json:"rr_gate"`
			PrimaryZone struct {
				Label string `json:"label"`
			} `json:"primary_zone"`
			ConfidenceExplanation struct {
				Label string `json:"label"`
			} `json:"confidence_explanation"`
		} `json:"decision"`
		ProbabilityContext struct {
			Health struct {
				HealthState          string   `json:"health_state"`
				DirectionalZoneCount int64    `json:"directional_zone_count"`
				QualityFlags         []string `json:"quality_flags"`
				ConfidenceGate       struct {
					MaxEntryState string `json:"max_entry_state"`
				} `json:"confidence_gate"`
			} `json:"health"`
		} `json:"probability_context"`
		NormalizedStatus map[string]string `json:"normalized_status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if parsed.Decision.MarketBias != "BEARISH_BIAS" ||
		parsed.Decision.PositionAction != "REDUCE_ON_BREAKDOWN" ||
		parsed.Decision.FinalEntryPermission.State != "WAIT_CONFIRMATION" {
		t.Fatalf("decision did not use normalized rows: %+v", parsed.Decision)
	}
	if len(parsed.Decision.MarketEvents) != 1 || parsed.Decision.MarketEvents[0].Type != "HIGH_VOLUME_BREAKDOWN" {
		t.Fatalf("market_events did not use normalized rows: %+v", parsed.Decision.MarketEvents)
	}
	if parsed.Decision.DecisionDerivedView.Version != "decision-derived-view-p0" ||
		parsed.Decision.DecisionDerivedView.BiasState != "BEARISH_BIAS" {
		t.Fatalf("decision_derived_view did not use normalized rows: %+v", parsed.Decision.DecisionDerivedView)
	}
	if parsed.Decision.EventStateSummary.MarketState != "BREAKDOWN_RISK" || len(parsed.Decision.EventStateSummary.ActiveBearishEvents) != 1 {
		t.Fatalf("event_state_summary did not use normalized rows: %+v", parsed.Decision.EventStateSummary)
	}
	if len(parsed.Decision.DailyCandidateZones) != 1 || parsed.Decision.DailyCandidateZones[0].Source != "DAILY_CANDLE" {
		t.Fatalf("daily_candidate_zones did not use normalized rows: %+v", parsed.Decision.DailyCandidateZones)
	}
	if parsed.Decision.PricePath.NextDecisionPrice != 581 ||
		parsed.Decision.EntryExecutability.ReasonCode != "EXECUTABLE_NOW" ||
		parsed.Decision.EntryBlockingZone.DistancePrice != 12 ||
		parsed.Decision.RRGate.ReasonCode != "RR_OK" ||
		parsed.Decision.PrimaryZone.Label != "580.00 ~ 585.00" ||
		parsed.Decision.ConfidenceExplanation.Label != "高" {
		t.Fatalf("decision detail did not use normalized rows: %+v", parsed.Decision)
	}
	if parsed.ProbabilityContext.Health.HealthState != "DEGRADED" ||
		parsed.ProbabilityContext.Health.DirectionalZoneCount != 1 ||
		parsed.ProbabilityContext.Health.ConfidenceGate.MaxEntryState != "SMALL_ENTRY" {
		t.Fatalf("probability_context did not use normalized rows: %+v", parsed.ProbabilityContext.Health)
	}
	for _, key := range []string{"decision", "events", "daily_candidates", "model_governance"} {
		if parsed.NormalizedStatus[key] != "normalized" {
			t.Fatalf("expected normalized_status[%s]=normalized, got %+v", key, parsed.NormalizedStatus)
		}
	}
}

// zone_builder_runtime_config 從 Python 到前端要經過 client struct → store → repo →
// 這裡的回應組裝，任何一段漏掉都會靜默丟資料（T-037 B 就是這樣被丟了一整條）。
// 這個測試鎖住最後一段：欄位有沒有真的出現在 analysis 區塊。
func TestSRZoneGetExposesZoneBuilderRuntimeConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC().Truncate(time.Second)
	repo := &srZoneRepoStub{
		analyses: []store.SRZoneAnalysis{{
			ID: 42, Symbol: "2330", Timeframe: "1d", AnalyzedAt: now,
			CurrentPrice: 100, PipelineVersion: "v2", ModelVersion: "v-test",
			ZoneBuilderRuntimeConfig: store.RawJSON(`{"enabled":true,"bucket":"HIGH_VOLATILITY","reason_code":"VOLATILITY_BUCKET_CONFIG"}`),
		}},
		zones: map[uint64][]store.SRZone{42: {}},
	}
	handler := NewSRZoneHandler(nil, repo, nil, nil, nil, nil, zap.NewNop())
	router := gin.New()
	router.GET("/sr-zones/:id", handler.Get)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sr-zones/42", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var parsed struct {
		Analysis struct {
			ZoneBuilderRuntimeConfig struct {
				Enabled    bool   `json:"enabled"`
				Bucket     string `json:"bucket"`
				ReasonCode string `json:"reason_code"`
			} `json:"zone_builder_runtime_config"`
		} `json:"analysis"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
	}
	got := parsed.Analysis.ZoneBuilderRuntimeConfig
	if !got.Enabled || got.Bucket != "HIGH_VOLATILITY" || got.ReasonCode != "VOLATILITY_BUCKET_CONFIG" {
		t.Fatalf("zone_builder_runtime_config not exposed in analysis block: body=%s", rec.Body.String())
	}
}

func TestSRZoneGetDoesNotUseLegacyJSONWhenNormalizedRowsAreMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC().Truncate(time.Second)
	repo := &srZoneRepoStub{
		analyses: []store.SRZoneAnalysis{{
			ID: 43, Symbol: "2330", Timeframe: "1d", AnalyzedAt: now,
			CurrentPrice: 100, PipelineVersion: "v2", ModelVersion: "v-test",
			DecisionSummary:    store.RawJSON(`{"market_bias":"LEGACY_BIAS","primary_zone":{"label":"LEGACY_ZONE"}}`),
			ProbabilityContext: store.RawJSON(`{"schema_version":"sr_probability_context_v1","health":{"health_state":"LEGACY_HEALTH"}}`),
		}},
		zones: map[uint64][]store.SRZone{43: {{
			ID: 1, AnalysisID: 43, PriceLow: 90, PriceHigh: 95, Method: "atr", Role: "SUPPORT",
			Status: "PENDING", TradingScore: 70,
		}}},
	}
	handler := NewSRZoneHandler(nil, repo, nil, nil, nil, nil, zap.NewNop())
	router := gin.New()
	router.GET("/sr-zones/:id", handler.Get)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sr-zones/43", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var parsed struct {
		Decision           json.RawMessage   `json:"decision"`
		ProbabilityContext json.RawMessage   `json:"probability_context"`
		NormalizedStatus   map[string]string `json:"normalized_status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if string(parsed.Decision) != "null" {
		t.Fatalf("expected decision=null without normalized row, got %s", parsed.Decision)
	}
	if string(parsed.ProbabilityContext) != "null" {
		t.Fatalf("expected probability_context=null without governance row, got %s", parsed.ProbabilityContext)
	}
	for _, key := range []string{"decision", "events", "daily_candidates", "model_governance"} {
		if parsed.NormalizedStatus[key] != "missing" {
			t.Fatalf("expected normalized_status[%s]=missing, got %+v", key, parsed.NormalizedStatus)
		}
	}
	if strings.Contains(rec.Body.String(), "LEGACY_") {
		t.Fatalf("response should not expose legacy raw JSON values: %s", rec.Body.String())
	}
}

// I-018：有 normalized decision 但天生沒有 market event 時，events 應為 normalized
// （空集合是合法的「無事件」），不得誤標成 missing。
func TestLoadSnapshotMarksEventsNormalizedWhenDecisionHasNoEventRows(t *testing.T) {
	repo := &srZoneRepoStub{
		analyses:  []store.SRZoneAnalysis{{ID: 7, Symbol: "2330", Timeframe: "1d"}},
		decisions: map[uint64]*store.SRDecision{7: {AnalysisID: 7}},
	}
	handler := NewSRZoneHandler(nil, repo, nil, nil, nil, nil, zap.NewNop())

	snapshot, err := handler.loadSRZonePipelineSnapshot(context.Background(), 7)
	if err != nil {
		t.Fatalf("loadSRZonePipelineSnapshot: %v", err)
	}
	if len(snapshot.EventDetections) != 0 || len(snapshot.EventStates) != 0 {
		t.Fatalf("expected no event rows, got detections=%d states=%d", len(snapshot.EventDetections), len(snapshot.EventStates))
	}
	if snapshot.Status["events"] != "normalized" {
		t.Fatalf("expected events=normalized when decision present without event rows, got %v", snapshot.Status["events"])
	}
}

func (s *srZoneRepoStub) ListMarketEventStateHistory(ctx context.Context, opts store.MarketEventStateHistoryOptions) ([]store.MarketEventState, error) {
	return s.eventStateHistory, nil
}

// TestEventTimelineRouteNotSwallowedByIDRoute：`/sr-zones/event-timeline` 與同層的
// `/sr-zones/:id` 並存時，必須路由到 timeline handler 而不是被當成 id=event-timeline。
// gin 對「靜態段 vs wildcard」的優先序是靜態優先，但這件事值得鎖住——
// 一旦有人把路徑改成 `/sr-zones/:symbol/event-timeline`，gin 會在啟動時 panic。
func TestEventTimelineRouteNotSwallowedByIDRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &srZoneRepoStub{}
	h := NewSRZoneHandler(nil, repo, nil, nil, nil, nil, zap.NewNop())

	router := gin.New()
	// 刻意照 server.go 的順序註冊，重現真實的路由樹
	router.GET("/sr-zones/:id", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"routed_to": "get_by_id", "id": c.Param("id")})
	})
	router.GET("/sr-zones/event-timeline", h.EventTimeline)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sr-zones/event-timeline?symbol=2330", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("回應不是合法 JSON: %v", err)
	}
	if body["routed_to"] == "get_by_id" {
		t.Fatal("請求被 /sr-zones/:id 吃掉了")
	}
	if body["symbol"] != "2330" {
		t.Errorf("symbol = %v, want 2330", body["symbol"])
	}
}

func TestEventTimelineRequiresSymbol(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewSRZoneHandler(nil, &srZoneRepoStub{}, nil, nil, nil, nil, zap.NewNop())
	router := gin.New()
	router.GET("/sr-zones/event-timeline", h.EventTimeline)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sr-zones/event-timeline", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("缺 symbol 的狀態碼 = %d, want 400", rec.Code)
	}
}

// 空結果的兩個陣列都要是 []，前端會直接 .map()。
func TestEventTimelineEmptyShapes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewSRZoneHandler(nil, &srZoneRepoStub{}, nil, nil, nil, nil, zap.NewNop())
	router := gin.New()
	router.GET("/sr-zones/event-timeline", h.EventTimeline)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sr-zones/event-timeline?symbol=2330", nil))

	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("回應不是合法 JSON: %v", err)
	}
	for _, key := range []string{"chains", "snapshots"} {
		if string(body[key]) == "null" {
			t.Errorf("%q 是 null，應該是 []", key)
		}
	}
}

func (s *srZoneRepoStub) ListAnalysisSnapshots(ctx context.Context, opts store.MarketEventStateHistoryOptions) ([]store.AnalysisSnapshot, error) {
	return s.analysisSnapshots, nil
}

// TestEventTimelineIdentitySinceIgnoresWindow：`identity_since` 不受 `max_analyses`
// 影響（todo.md T-051 R5）。視窗只保證未終結的鏈不被濾掉，視窗之前就終結的舊鏈不會
// 出現在 chains 裡；早期由 chains 推導時，畫面會把「這次沒查到」說成
// 「更早的分析沒有事件鏈」。這條走完整的 HTTP 路徑，鎖住端點實際回出去的值。
func TestEventTimelineIdentitySinceIgnoresWindow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, _, eventRepo := newIdentityStackForTest(t)
	ctx := context.Background()

	oldAt := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)
	newAt := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	for _, a := range []struct {
		id int64
		at time.Time
	}{{1, oldAt}, {2, newAt}} {
		if _, err := db.Exec(db.Rebind(`
			INSERT INTO stock_sr_zone_analyses
				(id, symbol, timeframe, analyzed_at, current_price, global_trend, global_volatility, model_version)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`), a.id, "0050", "1d", a.at, 100.0, 0.0, 0.0, "test"); err != nil {
			t.Fatalf("seed analysis %d failed: %v", a.id, err)
		}
	}

	// 兩條 SYMBOL scope 的鏈（不必先種 zone_instances）：舊的早已終結，新的還活著。
	symbolChain := func(uid, state string, seq int, active bool) store.EventInstance {
		return store.EventInstance{
			EventUID: uid, Symbol: "0050", Timeframe: "1d",
			ZoneScopeKey: store.SymbolScopeKey, EventScope: "SYMBOL",
			EventFamily: "SUPPORT_BREAKDOWN", Seq: seq,
			RootEventType: "SUPPORT_BREAKDOWN", LatestEventType: "SUPPORT_BREAKDOWN",
			State: state, Active: active, Direction: "BEARISH",
			FirstSeenAt: oldAt, LastSeenAt: newAt, DecisionVisible: true,
		}
	}
	ended := symbolChain("E-OLD", "EXPIRED", 1, false)
	ended.EndedAt = sql.NullTime{Time: oldAt, Valid: true}
	ended.EndReason = sql.NullString{String: "EXPIRED", Valid: true}

	if err := eventRepo.Apply(ctx, store.EventIdentityWrite{
		Instances: []store.EventInstance{ended, symbolChain("E-NEW", "CONFIRMED", 2, true)},
		Transitions: []store.EventTransition{
			{EventUID: "E-OLD", AnalysisID: sql.NullInt64{Int64: 1, Valid: true}, ToState: "CONFIRMED", OccurredAt: oldAt},
			{EventUID: "E-NEW", AnalysisID: sql.NullInt64{Int64: 2, Valid: true}, ToState: "CONFIRMED", OccurredAt: newAt},
		},
	}); err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	// 視窗只涵蓋最近一次分析（max_analyses=1），舊鏈因此被 ListChains 濾掉。
	repo := &srZoneRepoStub{analysisSnapshots: []store.AnalysisSnapshot{{ID: 2, AnalyzedAt: newAt}}}
	h := NewSRZoneHandler(nil, repo, nil, nil, nil, nil, zap.NewNop())
	h.SetEventIdentity(eventRepo)

	router := gin.New()
	router.GET("/sr-zones/event-timeline", h.EventTimeline)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/sr-zones/event-timeline?symbol=0050&max_analyses=1", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		IdentitySince *time.Time `json:"identity_since"`
		Chains        []struct {
			EventUID    string    `json:"event_uid"`
			FirstSeenAt time.Time `json:"first_seen_at"`
		} `json:"chains"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("回應不是合法 JSON: %v", err)
	}

	// 前提：舊鏈確實不在回傳的 chains 裡，否則這條測試什麼都沒證明。
	if len(body.Chains) != 1 || body.Chains[0].EventUID != "E-NEW" {
		t.Fatalf("視窗應只留下 E-NEW，得到 %+v", body.Chains)
	}
	if body.IdentitySince == nil || !body.IdentitySince.UTC().Equal(oldAt) {
		t.Errorf("identity_since = %v, want 視窗外舊鏈的 %v", body.IdentitySince, oldAt)
	}
	if !body.IdentitySince.Before(body.Chains[0].FirstSeenAt) {
		t.Error("identity_since 應早於視窗內最早的鏈——這正是 R5 要修的情況")
	}
}

// T-056：兩層壓力欄位要從 DB 一路走到 API response。
//
// 這條路上有**兩份各自獨立的白名單**：analysis.buildDecisionZoneSummariesJSON() 決定哪些鍵
// 寫得進 zone_summaries_json，applyDecisionZoneSummariesJSON() 決定哪些鍵展得回 decision_summary。
// 只補前者的話 Python 產了、DB 也存了，讀歷史決策時仍會被第二道白名單丟掉——這正是
// T-056 review F1 的缺口，所以本測試從 HTTP response 斷言，而不是只測投影函式。
func TestSRZoneGetExposesLayeredResistanceZonesFromZoneSummaries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC().Truncate(time.Second)
	repo := &srZoneRepoStub{
		analyses: []store.SRZoneAnalysis{{
			ID: 56, Symbol: "0050", Timeframe: "1d", AnalyzedAt: now,
			CurrentPrice: 104.65, PipelineVersion: "v2", ModelVersion: "v-test",
		}},
		zones: map[uint64][]store.SRZone{56: {{
			ID: 1, AnalysisID: 56, PriceLow: 105.19, PriceHigh: 111, Method: "atr", Role: "RESISTANCE",
			Status: "PENDING", TradingScore: 70,
		}}},
		decisions: map[uint64]*store.SRDecision{56: {
			AnalysisID: 56, MarketBias: "NEUTRAL_BIAS",
			// 戰術壓力（107.18~107.82，寬度懲罰加權後最相關）與前方擋路壓力
			// （105.19~111.00，純距離最近）是**不同的 zone**，這是 T-056 要同時露出的情況。
			ZoneSummariesJSON: store.RawJSON(`{"nearest_decision_zone":null,"nearest_support_zone":null,` +
				`"nearest_resistance_zone":{"label":"107.18 ~ 107.82","decision_role":"TACTICAL_RESISTANCE"},` +
				`"tactical_resistance_zone":{"label":"107.18 ~ 107.82","decision_role":"TACTICAL_RESISTANCE"},` +
				`"blocking_resistance_zone":{"label":"105.19 ~ 111.00","decision_role":"BLOCKING_RESISTANCE"},` +
				`"primary_structural_zone":{"label":"105.19 ~ 111.00"},"best_trade_zone":null,"primary_zone":null,"secondary_zones":[]}`),
		}},
	}
	handler := NewSRZoneHandler(nil, repo, nil, nil, nil, nil, zap.NewNop())
	router := gin.New()
	router.GET("/sr-zones/:id", handler.Get)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sr-zones/56", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var parsed struct {
		Decision struct {
			NearestResistanceZone struct {
				Label string `json:"label"`
			} `json:"nearest_resistance_zone"`
			TacticalResistanceZone struct {
				Label        string `json:"label"`
				DecisionRole string `json:"decision_role"`
			} `json:"tactical_resistance_zone"`
			BlockingResistanceZone struct {
				Label        string `json:"label"`
				DecisionRole string `json:"decision_role"`
			} `json:"blocking_resistance_zone"`
			PrimaryStructuralZone struct {
				Label string `json:"label"`
			} `json:"primary_structural_zone"`
		} `json:"decision"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if parsed.Decision.TacticalResistanceZone.Label != "107.18 ~ 107.82" ||
		parsed.Decision.TacticalResistanceZone.DecisionRole != "TACTICAL_RESISTANCE" {
		t.Fatalf("tactical_resistance_zone 沒有展開到 decision_summary: %+v", parsed.Decision)
	}
	if parsed.Decision.BlockingResistanceZone.Label != "105.19 ~ 111.00" ||
		parsed.Decision.BlockingResistanceZone.DecisionRole != "BLOCKING_RESISTANCE" {
		t.Fatalf("blocking_resistance_zone 沒有展開到 decision_summary: %+v", parsed.Decision)
	}
	// legacy alias 與大結構參考都不能因為新增欄位而消失。
	if parsed.Decision.NearestResistanceZone.Label != "107.18 ~ 107.82" {
		t.Fatalf("nearest_resistance_zone legacy alias 遺失: %+v", parsed.Decision)
	}
	if parsed.Decision.PrimaryStructuralZone.Label != "105.19 ~ 111.00" {
		t.Fatalf("primary_structural_zone 遺失: %+v", parsed.Decision)
	}
}
