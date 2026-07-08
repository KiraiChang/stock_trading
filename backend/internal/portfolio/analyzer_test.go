package portfolio

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/trading/backend/internal/analysis"
	"github.com/trading/backend/internal/store"
)

func TestBuildSnapshotCalculatesPricesAmountsAndDefaultHold(t *testing.T) {
	analyzer := &Analyzer{addOnRatio: defaultAddOnRatio}
	holding := testHolding()
	sr := testSRAnalysis(600, `{}`)
	zones := []store.SRZone{
		testZone("SUPPORT", 580, 585, 78, "PENDING"),
		testZone("RESISTANCE", 620, 630, 70, "PENDING"),
	}

	snapshot, err := analyzer.buildSnapshot(holding, sr, zones)
	if err != nil {
		t.Fatalf("buildSnapshot failed: %v", err)
	}

	if snapshot.Action != ActionHold || snapshot.ActionLabel != "繼續持有" {
		t.Fatalf("expected HOLD, got %+v", snapshot)
	}
	if snapshot.UnrealizedPnL != 10000 || snapshot.UnrealizedPnLPct != 0.2 {
		t.Fatalf("unexpected unrealized PnL: %+v / %+v", snapshot.UnrealizedPnL, snapshot.UnrealizedPnLPct)
	}
	assertNullFloat(t, snapshot.StopLossPrice, 580)
	assertNullFloat(t, snapshot.StopLossAmount, 0)
	assertNullFloat(t, snapshot.TakeProfitPrice, 620)
	assertNullFloat(t, snapshot.TakeProfitAmount, 12000)
	assertNullFloat(t, snapshot.AddOnTriggerPrice, 630)
	assertNullFloat(t, snapshot.AddOnAmount, 15000)
	assertDetailAction(t, snapshot.DetailJSON, "")
}

func TestBuildSnapshotActionRules(t *testing.T) {
	tests := []struct {
		name           string
		current        float64
		decision       string
		zones          []store.SRZone
		expectedAction string
	}{
		{
			name:     "stop loss when price breaks nearest support",
			current:  570,
			decision: `{}`,
			zones: []store.SRZone{
				testZone("SUPPORT", 580, 585, 78, "PENDING"),
				testZone("RESISTANCE", 620, 630, 70, "PENDING"),
			},
			expectedAction: ActionStopLoss,
		},
		{
			name:     "take profit near high score resistance",
			current:  609,
			decision: `{}`,
			zones: []store.SRZone{
				testZone("SUPPORT", 580, 585, 78, "PENDING"),
				testZone("RESISTANCE", 620, 630, 70, "PENDING"),
			},
			expectedAction: ActionTakeProfit,
		},
		{
			name:     "reduce when decision summary says avoid",
			current:  600,
			decision: `{"action":"Avoid"}`,
			zones: []store.SRZone{
				testZone("SUPPORT", 580, 585, 78, "PENDING"),
				testZone("RESISTANCE", 620, 630, 70, "PENDING"),
			},
			expectedAction: ActionReduce,
		},
		{
			name:     "watch breakout add on when decision summary is bullish",
			current:  600,
			decision: `{"action":"BuySmall"}`,
			zones: []store.SRZone{
				testZone("SUPPORT", 580, 585, 78, "PENDING"),
				testZone("RESISTANCE", 620, 630, 70, "PENDING"),
			},
			expectedAction: ActionAddOnBreakout,
		},
		{
			name:     "stop loss takes precedence over reduce",
			current:  570,
			decision: `{"action":"Avoid"}`,
			zones: []store.SRZone{
				testZone("SUPPORT", 580, 585, 78, "PENDING"),
				testZone("RESISTANCE", 620, 630, 70, "PENDING"),
			},
			expectedAction: ActionStopLoss,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := &Analyzer{addOnRatio: defaultAddOnRatio}
			snapshot, err := analyzer.buildSnapshot(testHolding(), testSRAnalysis(tt.current, tt.decision), tt.zones)
			if err != nil {
				t.Fatalf("buildSnapshot failed: %v", err)
			}
			if snapshot.Action != tt.expectedAction {
				t.Fatalf("expected action %s, got %s; reasons=%s detail=%s", tt.expectedAction, snapshot.Action, snapshot.Reason, snapshot.DetailJSON)
			}
		})
	}
}

func TestBuildSnapshotUsesResolvedRole(t *testing.T) {
	analyzer := &Analyzer{addOnRatio: defaultAddOnRatio}
	support := testZone("AT_ZONE", 580, 585, 78, "PENDING")
	support.ResolvedRole = store.NullString{NullString: sqlNullString("SUPPORT")}
	resistance := testZone("AT_ZONE", 620, 630, 70, "PENDING")
	resistance.ResolvedRole = store.NullString{NullString: sqlNullString("RESISTANCE")}

	snapshot, err := analyzer.buildSnapshot(testHolding(), testSRAnalysis(600, `{}`), []store.SRZone{support, resistance})
	if err != nil {
		t.Fatalf("buildSnapshot failed: %v", err)
	}
	assertNullFloat(t, snapshot.StopLossPrice, 580)
	assertNullFloat(t, snapshot.TakeProfitPrice, 620)
}

func TestAnalyzeReusesExistingSRZoneSnapshot(t *testing.T) {
	ctx := context.Background()
	holdingRepo := &fakeHoldingRepo{
		holding: &store.Holding{ID: 1, Symbol: "2330", Shares: 100, CostPrice: 500},
		saved:   make(map[uint64]*store.HoldingAnalysis),
	}
	srRepo := &fakeSRZoneRepo{
		analyses: []store.SRZoneAnalysis{
			{ID: 77, Symbol: "2330", Timeframe: "5m", CurrentPrice: 590},
			{ID: 88, Symbol: "2330", Timeframe: "1d", AnalyzedAt: time.Date(2026, 7, 1, 13, 30, 0, 0, time.UTC), CurrentPrice: 600, DecisionSummary: store.RawJSON(`{}`)},
		},
		zones: map[uint64][]store.SRZone{
			88: {
				testZone("SUPPORT", 580, 585, 78, "PENDING"),
				testZone("RESISTANCE", 620, 630, 70, "PENDING"),
			},
		},
	}
	analyzer := &Analyzer{
		holdings:     holdingRepo,
		srZoneRepo:   srRepo,
		addOnRatio:   defaultAddOnRatio,
		defaultLimit: 250,
		now:          func() time.Time { return time.Date(2026, 7, 1, 14, 30, 0, 0, time.UTC) },
	}

	result, err := analyzer.Analyze(ctx, 1, AnalyzeOptions{Timeframe: "1d"})
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if srRepo.createCalls != 0 {
		t.Fatalf("expected existing SR snapshot to be reused, got Create calls=%d", srRepo.createCalls)
	}
	if result.SR.ID != 88 || result.Analysis.SRZoneAnalysisID.Int64 != 88 {
		t.Fatalf("expected holding analysis to reference existing SR id=88, got result=%+v analysis=%+v", result.SR, result.Analysis)
	}
	if len(result.Zones) != 2 {
		t.Fatalf("expected existing zones to be returned, got %+v", result.Zones)
	}
	if len(holdingRepo.saved) != 1 {
		t.Fatalf("expected one holding analysis snapshot, got %+v", holdingRepo.saved)
	}
}

func TestFindExistingSRSnapshotSkipsStaleSnapshot(t *testing.T) {
	ctx := context.Background()
	srRepo := &fakeSRZoneRepo{
		analyses: []store.SRZoneAnalysis{
			{ID: 88, Symbol: "2330", Timeframe: "1d", AnalyzedAt: time.Date(2026, 7, 1, 13, 30, 0, 0, time.UTC), CurrentPrice: 600},
		},
		zones: map[uint64][]store.SRZone{
			88: {testZone("SUPPORT", 580, 585, 78, "PENDING")},
		},
	}
	analyzer := &Analyzer{
		srZoneRepo:    srRepo,
		srReuseMaxAge: defaultSRReuseMaxAge,
		now:           func() time.Time { return time.Date(2026, 7, 8, 13, 30, 0, 0, time.UTC) },
	}

	_, _, ok, err := analyzer.findExistingSRSnapshot(ctx, "2330", "1d")
	if err != nil {
		t.Fatalf("findExistingSRSnapshot failed: %v", err)
	}
	if ok {
		t.Fatalf("expected stale SR snapshot to be skipped")
	}
}

func TestAnalyzeDeletesCreatedSRZoneWhenHoldingAnalysisCreateFails(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sr-zones" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(analysis.ZoneScoreResult{
			Symbol: "2330", Timeframe: "1d", AnalyzedAt: "2026-07-08T13:30:00Z",
			CurrentPrice: 600.0,
			Zones: []analysis.ZoneScore{
				{
					PriceLow: 580, PriceHigh: 585, Role: "SUPPORT", Method: "atr",
					Tier: "TIER_1_MAIN_STRUCTURE", TierLabel: "主結構",
					Confidence: 0.8, ConfidenceLevel: "HIGH",
					TradingScore: 78, TradingRecommendation: "BUY",
					TradingScoreBreakdown: map[string]float64{
						"expected_value": 20,
						"risk_reward":    15,
						"trend":          12,
						"volume":         10,
						"confidence":     9,
						"chip":           12,
					},
				},
			},
		})
	}))
	defer server.Close()

	createErr := errors.New("create analysis failed")
	holdingRepo := &fakeHoldingRepo{
		holding:           &store.Holding{ID: 1, Symbol: "2330", Shares: 100, CostPrice: 500},
		saved:             make(map[uint64]*store.HoldingAnalysis),
		createAnalysisErr: createErr,
	}
	srRepo := &fakeSRZoneRepo{zones: make(map[uint64][]store.SRZone)}
	analyzer := &Analyzer{
		client:        analysis.NewClient(server.URL),
		holdings:      holdingRepo,
		srZoneRepo:    srRepo,
		addOnRatio:    defaultAddOnRatio,
		defaultLimit:  250,
		now:           func() time.Time { return time.Date(2026, 7, 8, 14, 30, 0, 0, time.UTC) },
		srReuseMaxAge: defaultSRReuseMaxAge,
	}

	_, err := analyzer.Analyze(ctx, 1, AnalyzeOptions{Timeframe: "1d"})
	if !errors.Is(err, createErr) {
		t.Fatalf("expected create analysis error, got %v", err)
	}
	if srRepo.createCalls != 1 {
		t.Fatalf("expected one SR create call, got %d", srRepo.createCalls)
	}
	if srRepo.deleteCalls != 1 || srRepo.deletedID != 999 {
		t.Fatalf("expected created SR id=999 to be deleted, deleteCalls=%d deletedID=%d", srRepo.deleteCalls, srRepo.deletedID)
	}
}

func TestAnalyzeKeepsSRZoneWhenHoldingReadBackFails(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(analysis.ZoneScoreResult{
			Symbol: "2330", Timeframe: "1d", AnalyzedAt: "2026-07-08T13:30:00Z",
			CurrentPrice: 600.0,
			Zones: []analysis.ZoneScore{
				{
					PriceLow: 580, PriceHigh: 585, Role: "SUPPORT", Method: "atr",
					Tier: "TIER_1_MAIN_STRUCTURE", TierLabel: "主結構",
					Confidence: 0.8, ConfidenceLevel: "HIGH",
					TradingScore: 78, TradingRecommendation: "BUY",
					TradingScoreBreakdown: map[string]float64{
						"expected_value": 20, "risk_reward": 15, "trend": 12,
						"volume": 10, "confidence": 9, "chip": 12,
					},
				},
			},
		})
	}))
	defer server.Close()

	holdingRepo := &fakeHoldingRepo{
		holding:        &store.Holding{ID: 1, Symbol: "2330", Shares: 100, CostPrice: 500},
		saved:          make(map[uint64]*store.HoldingAnalysis),
		getAnalysisErr: errors.New("read back failed"),
	}
	srRepo := &fakeSRZoneRepo{zones: make(map[uint64][]store.SRZone)}
	analyzer := &Analyzer{
		client:        analysis.NewClient(server.URL),
		holdings:      holdingRepo,
		srZoneRepo:    srRepo,
		addOnRatio:    defaultAddOnRatio,
		defaultLimit:  250,
		now:           func() time.Time { return time.Date(2026, 7, 8, 14, 30, 0, 0, time.UTC) },
		srReuseMaxAge: defaultSRReuseMaxAge,
	}

	result, err := analyzer.Analyze(ctx, 1, AnalyzeOptions{Timeframe: "1d"})
	if err != nil {
		t.Fatalf("expected read-back failure to be tolerated, got %v", err)
	}
	if srRepo.deleteCalls != 0 {
		t.Fatalf("expected SR snapshot to be kept when holding row already persisted, got deleteCalls=%d", srRepo.deleteCalls)
	}
	if result.Analysis == nil || result.Analysis.SRZoneAnalysisID.Int64 != 999 {
		t.Fatalf("expected fallback analysis referencing SR id=999, got %+v", result.Analysis)
	}
	if len(holdingRepo.saved) != 1 {
		t.Fatalf("expected holding analysis to remain persisted, got %d", len(holdingRepo.saved))
	}
}

func testHolding() *store.Holding {
	return &store.Holding{ID: 1, Symbol: "2330", Shares: 100, CostPrice: 500}
}

func testSRAnalysis(current float64, decision string) *store.SRZoneAnalysis {
	return &store.SRZoneAnalysis{
		ID:              10,
		Symbol:          "2330",
		Timeframe:       "1d",
		AnalyzedAt:      time.Date(2026, 7, 1, 13, 30, 0, 0, time.UTC),
		CurrentPrice:    current,
		DecisionSummary: store.RawJSON(decision),
	}
}

func testZone(role string, low, high, score float64, status string) store.SRZone {
	return store.SRZone{
		ID:                    uint64(low),
		PriceLow:              low,
		PriceHigh:             high,
		Role:                  role,
		Tier:                  "TIER_1_MAIN_STRUCTURE",
		Confidence:            0.8,
		ConfidenceLevel:       "HIGH",
		TradingScore:          score,
		TradingRecommendation: "BUY",
		Status:                status,
	}
}

func assertNullFloat(t *testing.T, got store.NullFloat64, want float64) {
	t.Helper()
	if !got.Valid || got.Float64 != want {
		t.Fatalf("expected %.4f, got %+v", want, got)
	}
}

func assertDetailAction(t *testing.T, raw store.RawJSON, want string) {
	t.Helper()
	var detail struct {
		SRDecisionAction string `json:"sr_decision_action"`
	}
	if err := json.Unmarshal([]byte(raw), &detail); err != nil {
		t.Fatalf("decode detail_json failed: %v", err)
	}
	if detail.SRDecisionAction != want {
		t.Fatalf("expected sr_decision_action=%q, got %q", want, detail.SRDecisionAction)
	}
}

func sqlNullString(v string) sql.NullString {
	return sql.NullString{String: v, Valid: true}
}

type fakeHoldingRepo struct {
	holding           *store.Holding
	saved             map[uint64]*store.HoldingAnalysis
	nextID            uint64
	createAnalysisErr error
	getAnalysisErr    error
}

func (r *fakeHoldingRepo) Create(ctx context.Context, h *store.Holding) (uint64, error) {
	return 0, nil
}

func (r *fakeHoldingRepo) Update(ctx context.Context, h *store.Holding) error {
	return nil
}

func (r *fakeHoldingRepo) Delete(ctx context.Context, id uint64) error {
	return nil
}

func (r *fakeHoldingRepo) Get(ctx context.Context, id uint64) (*store.Holding, error) {
	if r.holding == nil || r.holding.ID != id {
		return nil, sql.ErrNoRows
	}
	h := *r.holding
	return &h, nil
}

func (r *fakeHoldingRepo) List(ctx context.Context) ([]store.Holding, error) {
	return nil, nil
}

func (r *fakeHoldingRepo) CreateAnalysis(ctx context.Context, a *store.HoldingAnalysis) (uint64, error) {
	if r.createAnalysisErr != nil {
		return 0, r.createAnalysisErr
	}
	r.nextID++
	saved := *a
	saved.ID = r.nextID
	r.saved[saved.ID] = &saved
	return saved.ID, nil
}

func (r *fakeHoldingRepo) GetAnalysis(ctx context.Context, id uint64) (*store.HoldingAnalysis, error) {
	if r.getAnalysisErr != nil {
		return nil, r.getAnalysisErr
	}
	a, ok := r.saved[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	saved := *a
	return &saved, nil
}

func (r *fakeHoldingRepo) ListAnalyses(ctx context.Context, holdingID uint64, limit int) ([]store.HoldingAnalysis, error) {
	return nil, nil
}

func (r *fakeHoldingRepo) DeleteAnalysis(ctx context.Context, id uint64) error {
	delete(r.saved, id)
	return nil
}

type fakeSRZoneRepo struct {
	analyses    []store.SRZoneAnalysis
	zones       map[uint64][]store.SRZone
	createCalls int
	deleteCalls int
	deletedID   uint64
}

func (r *fakeSRZoneRepo) Create(ctx context.Context, a *store.SRZoneAnalysis, zones []store.SRZone) (uint64, error) {
	r.createCalls++
	created := *a
	created.ID = 999
	r.analyses = append([]store.SRZoneAnalysis{created}, r.analyses...)
	r.zones[999] = append([]store.SRZone(nil), zones...)
	return 999, nil
}

func (r *fakeSRZoneRepo) Get(ctx context.Context, id uint64) (*store.SRZoneAnalysis, error) {
	for i := range r.analyses {
		if r.analyses[i].ID == id {
			a := r.analyses[i]
			return &a, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (r *fakeSRZoneRepo) List(ctx context.Context, symbol string, limit int) ([]store.SRZoneAnalysis, error) {
	rows := make([]store.SRZoneAnalysis, 0, len(r.analyses))
	for _, a := range r.analyses {
		if a.Symbol == symbol {
			rows = append(rows, a)
		}
	}
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}

func (r *fakeSRZoneRepo) GetZones(ctx context.Context, analysisID uint64) ([]store.SRZone, error) {
	zones, ok := r.zones[analysisID]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return append([]store.SRZone(nil), zones...), nil
}

func (r *fakeSRZoneRepo) UpdateZoneStatus(ctx context.Context, zoneID uint64, status string, brokenAt *time.Time, brokenPrice *float64, resolvedRole string) error {
	return nil
}

func (r *fakeSRZoneRepo) Delete(ctx context.Context, id uint64) error {
	r.deleteCalls++
	r.deletedID = id
	delete(r.zones, id)
	return nil
}
