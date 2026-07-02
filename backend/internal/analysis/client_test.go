package analysis

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAnalyzeSendsLimitWhenProvided(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Symbol    string `json:"symbol"`
			Timeframe string `json:"timeframe"`
			Limit     int    `json:"limit"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body failed: %v", err)
		}
		if body.Limit != 500 {
			t.Fatalf("expected limit=500, got %d", body.Limit)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Result{
			Symbol: "2330", Timeframe: "1d", AnalyzedAt: "2026-07-01T13:30:00+08:00",
			CurrentPrice: 600.0, Trend: "BULLISH",
			Entry: Entry{Status: "WATCHING", Direction: "NONE", Price: 600.0, Reason: "test"},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	if _, err := client.Analyze(context.Background(), "2330", "1d", 500); err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
}

func TestAnalyzeOmitsLimitFieldWhenZero(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var raw map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			t.Fatalf("decode request body failed: %v", err)
		}
		if _, ok := raw["limit"]; ok {
			t.Fatalf("expected \"limit\" field to be omitted when 0, got raw body %v", raw)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Result{
			Symbol: "2330", Timeframe: "1d", AnalyzedAt: "2026-07-01T13:30:00+08:00",
			CurrentPrice: 600.0, Trend: "BULLISH",
			Entry: Entry{Status: "WATCHING", Direction: "NONE", Price: 600.0, Reason: "test"},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	if _, err := client.Analyze(context.Background(), "2330", "1d", 0); err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
}

func TestScoreZonesParsesResponseAndMapsToStore(t *testing.T) {
	bounce := 0.72
	brk := 0.18
	gain := 0.048
	loss := -0.072
	ev := 0.0056
	rr := 0.667
	percentile := 91.0
	relVol := 1.4
	volConf := "CONFIRMED"
	rejectCount := 3
	breakCount := 0
	globalEV := 0.004
	globalConfidence := 0.6
	globalRR := 0.9

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sr-zones" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var body struct {
			Symbol    string `json:"symbol"`
			Timeframe string `json:"timeframe"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body failed: %v", err)
		}
		if body.Symbol != "2330" || body.Timeframe != "1d" {
			t.Fatalf("unexpected request body: %+v", body)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ZoneScoreResult{
			Symbol: "2330", Timeframe: "1d", AnalyzedAt: "2026-07-01T13:30:00+08:00",
			CurrentPrice: 600.0, GlobalTrend: 0.03, GlobalVolatility: 0.02,
			GlobalExpectedValue: &globalEV, GlobalConfidence: &globalConfidence, GlobalRiskRewardRatio: &globalRR,
			Zones: []ZoneScore{
				{
					PriceLow: 580.0, PriceHigh: 585.0, Method: "atr", Role: "SUPPORT",
					Tier: "TIER_1_MAIN_STRUCTURE", TierLabel: "主結構",
					SupportScore: 0.8, ResistanceScore: 0.1, NetScore: 0.7, NetScoreLabel: "STRONG_SUPPORT",
					Confidence: 0.83, ConfidenceLevel: "HIGH",
					BounceProbability: &bounce, BreakProbability: &brk,
					ExpectedGain: &gain, ExpectedLoss: &loss, ExpectedValue: &ev,
					RiskRewardRatio: &rr, RewardRiskPercentile: &percentile,
					RelativeVolume: &relVol, VolumeConfirmation: &volConf,
					TouchCount: 4, RejectCount: &rejectCount, BreakCount: &breakCount,
					ZoneMomentum: -0.02, ZoneDirection: "DOWN",
					RecentValidation: "VALIDATED_RECENTLY",
					TradingScore:     78.5, TradingScoreBreakdown: map[string]float64{
						"expected_value": 30.0, "risk_reward": 15.0, "trend": 10.0, "volume": 15.0, "confidence": 8.5,
					},
					TradingRecommendation: "BUY",
				},
				{
					PriceLow: 610.0, PriceHigh: 615.0, Method: "volume_profile", Role: "AT_ZONE",
					Tier: "TIER_3_SHORT_TERM", TierLabel: "短期支撐",
					SupportScore: 0.2, ResistanceScore: 0.3, NetScore: -0.1, NetScoreLabel: "NEUTRAL",
					Confidence: 0.4, ConfidenceLevel: "MEDIUM",
					TouchCount: 2, ZoneMomentum: 0.0, ZoneDirection: "FLAT",
					RecentValidation: "PENDING_VALIDATION",
					TradingScore:     45.0, TradingScoreBreakdown: map[string]float64{
						"expected_value": 20.0, "risk_reward": 10.0, "trend": 7.5, "volume": 7.5, "confidence": 4.0,
					},
					TradingRecommendation: "NEUTRAL",
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	result, err := client.ScoreZones(context.Background(), "2330", "1d", 0)
	if err != nil {
		t.Fatalf("ScoreZones failed: %v", err)
	}
	if result.Symbol != "2330" || len(result.Zones) != 2 || result.GlobalTrend != 0.03 {
		t.Fatalf("unexpected result: %+v", result)
	}

	a, zones, err := result.ToStore()
	if err != nil {
		t.Fatalf("ToStore failed: %v", err)
	}
	if a.Symbol != "2330" || a.CurrentPrice != 600.0 || a.GlobalTrend != 0.03 || a.GlobalVolatility != 0.02 {
		t.Fatalf("unexpected analysis: %+v", a)
	}
	if !a.GlobalExpectedValue.Valid || a.GlobalExpectedValue.Float64 != globalEV {
		t.Fatalf("expected global_expected_value %.3f, got %+v", globalEV, a.GlobalExpectedValue)
	}
	if !a.GlobalConfidence.Valid || a.GlobalConfidence.Float64 != globalConfidence {
		t.Fatalf("expected global_confidence %.3f, got %+v", globalConfidence, a.GlobalConfidence)
	}
	if !a.GlobalRiskRewardRatio.Valid || a.GlobalRiskRewardRatio.Float64 != globalRR {
		t.Fatalf("expected global_risk_reward_ratio %.3f, got %+v", globalRR, a.GlobalRiskRewardRatio)
	}
	if len(zones) != 2 {
		t.Fatalf("expected 2 zones, got %d", len(zones))
	}

	first := zones[0]
	if first.Method != "atr" || first.Role != "SUPPORT" || first.TouchCount != 4 {
		t.Fatalf("unexpected first zone: %+v", first)
	}
	if first.Tier != "TIER_1_MAIN_STRUCTURE" || first.TierLabel != "主結構" {
		t.Fatalf("unexpected first zone tier: %+v", first)
	}
	if first.NetScoreLabel != "STRONG_SUPPORT" || first.ConfidenceLevel != "HIGH" {
		t.Fatalf("unexpected first zone labels: %+v", first)
	}
	if len(first.TradingScoreBreakdown) == 0 {
		t.Fatalf("expected non-empty trading_score_breakdown JSON, got %+v", first)
	}
	if !first.BounceProbability.Valid || first.BounceProbability.Float64 != bounce {
		t.Fatalf("expected bounce probability %.2f, got %+v", bounce, first.BounceProbability)
	}
	if !first.ExpectedGain.Valid || first.ExpectedGain.Float64 != gain {
		t.Fatalf("expected expected_gain %.3f, got %+v", gain, first.ExpectedGain)
	}
	if !first.ExpectedLoss.Valid || first.ExpectedLoss.Float64 != loss {
		t.Fatalf("expected expected_loss %.3f, got %+v", loss, first.ExpectedLoss)
	}
	if !first.ExpectedValue.Valid || first.ExpectedValue.Float64 != ev {
		t.Fatalf("expected expected_value %.4f, got %+v", ev, first.ExpectedValue)
	}
	if !first.RiskRewardRatio.Valid || first.RiskRewardRatio.Float64 != rr {
		t.Fatalf("expected risk_reward_ratio %.3f, got %+v", rr, first.RiskRewardRatio)
	}
	if !first.RewardRiskPercentile.Valid || first.RewardRiskPercentile.Float64 != percentile {
		t.Fatalf("expected reward_risk_percentile %.1f, got %+v", percentile, first.RewardRiskPercentile)
	}
	if !first.VolumeConfirmation.Valid || first.VolumeConfirmation.String != volConf {
		t.Fatalf("expected volume_confirmation %s, got %+v", volConf, first.VolumeConfirmation)
	}
	if first.RejectCount != 3 || first.BreakCount != 0 {
		t.Fatalf("unexpected reject/break count: %+v", first)
	}

	second := zones[1]
	if second.Role != "AT_ZONE" || second.BounceProbability.Valid {
		t.Fatalf("expected AT_ZONE zone with no bounce probability, got %+v", second)
	}
	if second.ExpectedValue.Valid || second.RiskRewardRatio.Valid || second.VolumeConfirmation.Valid {
		t.Fatalf("expected AT_ZONE zone with no expected_value/risk_reward_ratio/volume_confirmation, got %+v", second)
	}
	if second.RejectCount != 0 || second.BreakCount != 0 {
		t.Fatalf("expected AT_ZONE zone to default reject/break count to 0, got %+v", second)
	}
}

func TestScoreZonesReturnsErrorWhenBaseURLNotConfigured(t *testing.T) {
	client := NewClient("")
	if _, err := client.ScoreZones(context.Background(), "2330", "1d", 0); err == nil {
		t.Fatal("expected error when baseURL is not configured")
	}
}

func TestScoreZonesSendsLimitWhenProvided(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Limit int `json:"limit"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body failed: %v", err)
		}
		if body.Limit != 500 {
			t.Fatalf("expected limit=500, got %d", body.Limit)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ZoneScoreResult{
			Symbol: "2330", Timeframe: "1d", AnalyzedAt: "2026-07-01T13:30:00+08:00", CurrentPrice: 600.0,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	if _, err := client.ScoreZones(context.Background(), "2330", "1d", 500); err != nil {
		t.Fatalf("ScoreZones failed: %v", err)
	}
}

func TestTrainModelParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sr-scoring/train" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var body trainRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body failed: %v", err)
		}
		if len(body.Symbols) != 2 || body.ModelType != "gradient_boosting" {
			t.Fatalf("unexpected request body: %+v", body)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TrainResult{
			Rows: 120, Sources: 2, ModelType: "gradient_boosting",
			Metrics:   map[string]map[string]float64{"hold": {"accuracy": 0.9}, "break": {"accuracy": 0.85}},
			ModelPath: "models/sr_scoring_v1.joblib", TrainedAt: "2026-07-01T13:30:00+08:00", Version: "v1",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	result, err := client.TrainModel(context.Background(), []string{"2330", "2454"}, "1d", 1500, "gradient_boosting")
	if err != nil {
		t.Fatalf("TrainModel failed: %v", err)
	}
	if result.Rows != 120 || result.Sources != 2 || result.ModelPath != "models/sr_scoring_v1.joblib" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.Metrics["hold"]["accuracy"] != 0.9 {
		t.Fatalf("unexpected metrics: %+v", result.Metrics)
	}
}

func TestTrainModelReturnsErrorWhenBaseURLNotConfigured(t *testing.T) {
	client := NewClient("")
	if _, err := client.TrainModel(context.Background(), []string{"2330"}, "1d", 0, ""); err == nil {
		t.Fatal("expected error when baseURL is not configured")
	}
}
