package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/trading/backend/internal/store"
)

type srProviderScorerStub struct {
	calls  int
	result *ZoneScoreResult
	err    error
}

func (s *srProviderScorerStub) ScoreZones(ctx context.Context, symbol, timeframe string, limit int) (*ZoneScoreResult, error) {
	s.calls++
	return s.result, s.err
}

type srProviderRepoStub struct {
	eventStateHistory []store.MarketEventState
	analyses          []store.SRZoneAnalysis
	zones             map[uint64][]store.SRZone
	nextID            uint64

	listErr     error
	getZonesErr error
	createErr   error
	getErr      error

	createCalls int
}

func (s *srProviderRepoStub) Create(ctx context.Context, a *store.SRZoneAnalysis, zones []store.SRZone, projections store.SRZoneNormalizedProjections) (uint64, error) {
	if s.createErr != nil {
		return 0, s.createErr
	}
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

func (s *srProviderRepoStub) GetDecision(ctx context.Context, analysisID uint64) (*store.SRDecision, error) {
	return nil, errors.New("not found")
}

func (s *srProviderRepoStub) GetMarketEventDetections(ctx context.Context, analysisID uint64) ([]store.MarketEventDetection, error) {
	return nil, nil
}

func (s *srProviderRepoStub) GetMarketEventStates(ctx context.Context, analysisID uint64) ([]store.MarketEventState, error) {
	return nil, nil
}

func (s *srProviderRepoStub) GetLatestMarketEventStates(ctx context.Context, symbol, timeframe string) ([]store.MarketEventState, error) {
	return nil, nil
}

func (s *srProviderRepoStub) GetDailyCandidates(ctx context.Context, analysisID uint64) ([]store.SRDailyCandidate, error) {
	return nil, nil
}

func (s *srProviderRepoStub) GetModelGovernance(ctx context.Context, analysisID uint64) (*store.SRModelGovernance, error) {
	return nil, errors.New("not found")
}

func (s *srProviderRepoStub) Get(ctx context.Context, id uint64) (*store.SRZoneAnalysis, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	for i := range s.analyses {
		if s.analyses[i].ID == id {
			return &s.analyses[i], nil
		}
	}
	return nil, errors.New("not found")
}

// 排程用的 timeframe-aware 查詢（todo.md T-052）。這些 stub 沒有用到它。
func (s *srProviderRepoStub) GetLatestByTimeframe(ctx context.Context, symbol, timeframe string) (*store.SRZoneAnalysis, error) {
	return nil, nil
}

// ListRefsSince 只為滿足 store.SRZoneRepo 介面而存在——這個 stub 服務的 SRAnalysisProvider
// 走的是 List／GetLatestByTimeframe，不碰這支（sr_zone_verify 排程才用它）。
// 刻意回空而不是轉呼叫 List：真的有人在這裡依賴它時，測試會因為拿不到資料而失敗，
// 比靜默回傳一份語意不同的清單好。
func (s *srProviderRepoStub) ListRefsSince(ctx context.Context, since time.Time, limit int) ([]store.SRZoneAnalysisRef, error) {
	return nil, nil
}

func (s *srProviderRepoStub) List(ctx context.Context, symbol string, limit int) ([]store.SRZoneAnalysis, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	rows := make([]store.SRZoneAnalysis, 0, len(s.analyses))
	for _, a := range s.analyses {
		if symbol == "" || a.Symbol == symbol {
			rows = append(rows, a)
		}
	}
	return rows, nil
}

func (s *srProviderRepoStub) GetZones(ctx context.Context, analysisID uint64) ([]store.SRZone, error) {
	if s.getZonesErr != nil {
		return nil, s.getZonesErr
	}
	return s.zones[analysisID], nil
}

func (s *srProviderRepoStub) UpdateZoneStatus(ctx context.Context, zoneID uint64, status string, brokenAt *time.Time, brokenPrice *float64, resolvedRole string) error {
	return nil
}

func (s *srProviderRepoStub) Delete(ctx context.Context, id uint64) error {
	return nil
}

func testZoneScoreResult(symbol string, analyzedAt time.Time) *ZoneScoreResult {
	bounce := 0.7
	return &ZoneScoreResult{
		PipelineVersion:  "v2",
		Symbol:           symbol,
		Timeframe:        "1d",
		AnalyzedAt:       analyzedAt.Format(time.RFC3339),
		CurrentPrice:     100,
		GlobalTrend:      0.1,
		GlobalVolatility: 0.2,
		ModelVersion:     "v-test",
		ModelConfigHash:  "hash-test",
		PeriodSummaries:  json.RawMessage(`[]`),
		AnalysisTips:     json.RawMessage(`[]`),
		ChipSummary:      json.RawMessage(`null`),
		DecisionSummary:  json.RawMessage(`{"action":"BuySmall"}`),
		Zones: []ZoneScore{{
			PriceLow: 90, PriceHigh: 95, Method: "atr", Role: "SUPPORT",
			Tier: "TIER_1_MAIN_STRUCTURE", TierLabel: "主結構",
			Confidence: 0.8, ConfidenceLevel: "HIGH",
			BounceProbability: &bounce,
			NetScoreLabel:     "STRONG_SUPPORT",
			RecentValidation:  "PENDING_VALIDATION",
			TradingScore:      70,
			TradingScoreBreakdown: map[string]float64{
				"expected_value": 20, "risk_reward": 10, "trend": 10,
				"volume": 10, "confidence": 10, "chip": 10,
			},
			TradingRecommendation: "BUY",
			ConfluenceCount:       1,
		}},
	}
}

func TestSRAnalysisProviderReusesFreshSnapshot(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	repo := &srProviderRepoStub{
		analyses: []store.SRZoneAnalysis{{
			ID: 9, Symbol: "2330", Timeframe: "1d", AnalyzedAt: now.Add(-time.Hour), CurrentPrice: 100,
		}},
		zones: map[uint64][]store.SRZone{9: {{ID: 1, AnalysisID: 9}}},
	}
	scorer := &srProviderScorerStub{result: testZoneScoreResult("2330", now)}
	provider := NewSRAnalysisProvider(scorer, repo, 24*time.Hour)
	provider.now = func() time.Time { return now }

	result, err := provider.Analyze(context.Background(), "2330", SRAnalysisOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Analysis.ID != 9 || len(result.Zones) != 1 {
		t.Fatalf("expected reusable snapshot, got %+v", result)
	}
	if scorer.calls != 0 || repo.createCalls != 0 {
		t.Fatalf("expected no scoring/create on reuse, scorer=%d create=%d", scorer.calls, repo.createCalls)
	}
}

func TestSRAnalysisProviderForceRefreshCreatesSnapshot(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	repo := &srProviderRepoStub{
		analyses: []store.SRZoneAnalysis{{ID: 9, Symbol: "2330", Timeframe: "1d", AnalyzedAt: now.Add(-time.Hour)}},
		zones:    map[uint64][]store.SRZone{9: {{ID: 1, AnalysisID: 9}}},
		nextID:   10,
	}
	scorer := &srProviderScorerStub{result: testZoneScoreResult("2330", now)}
	provider := NewSRAnalysisProvider(scorer, repo, 24*time.Hour)
	provider.now = func() time.Time { return now }

	result, err := provider.Analyze(context.Background(), "2330", SRAnalysisOptions{ForceRefresh: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Analysis.ID != 10 {
		t.Fatalf("expected new snapshot id=10, got %+v", result.Analysis)
	}
	if scorer.calls != 1 || repo.createCalls != 1 {
		t.Fatalf("expected scoring/create, scorer=%d create=%d", scorer.calls, repo.createCalls)
	}
}

func TestSRAnalysisProviderRefreshesExpiredSnapshot(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	repo := &srProviderRepoStub{
		analyses: []store.SRZoneAnalysis{{ID: 9, Symbol: "2330", Timeframe: "1d", AnalyzedAt: now.Add(-48 * time.Hour)}},
		zones:    map[uint64][]store.SRZone{9: {{ID: 1, AnalysisID: 9}}},
		nextID:   10,
	}
	scorer := &srProviderScorerStub{result: testZoneScoreResult("2330", now)}
	provider := NewSRAnalysisProvider(scorer, repo, 24*time.Hour)
	provider.now = func() time.Time { return now }

	result, err := provider.Analyze(context.Background(), "2330", SRAnalysisOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Analysis.ID != 10 || scorer.calls != 1 {
		t.Fatalf("expected refresh for expired snapshot, result=%+v scorer=%d", result.Analysis, scorer.calls)
	}
}

func TestSRAnalysisProviderWrapsRepoErrors(t *testing.T) {
	repo := &srProviderRepoStub{listErr: errors.New("db down")}
	provider := NewSRAnalysisProvider(&srProviderScorerStub{}, repo, 24*time.Hour)

	_, err := provider.Analyze(context.Background(), "2330", SRAnalysisOptions{})
	if err == nil || !strings.Contains(err.Error(), "list existing sr zone analyses") {
		t.Fatalf("expected contextual list error, got %v", err)
	}
}

func (s *srProviderRepoStub) ListMarketEventStateHistory(ctx context.Context, opts store.MarketEventStateHistoryOptions) ([]store.MarketEventState, error) {
	return s.eventStateHistory, nil
}

func (s *srProviderRepoStub) ListAnalysisSnapshots(ctx context.Context, opts store.MarketEventStateHistoryOptions) ([]store.AnalysisSnapshot, error) {
	return nil, nil
}
