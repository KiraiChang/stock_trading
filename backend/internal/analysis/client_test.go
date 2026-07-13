package analysis

import (
	"context"
	"encoding/json"
	"errors"
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
	overlapGroup := 0
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
			ModelVersion: "v3", ModelTrainedAt: "2026-06-30T09:00:00+08:00", ModelFeatureNames: []string{"touch_count", "is_support", "chip_total_score"},
			ModelConfigHash: "abc123def456",
			PeriodSummaries: json.RawMessage(`[{"key":"short","label":"短期"}]`),
			AnalysisTips:    json.RawMessage(`["短期支撐守穩，籌碼偏多"]`),
			DecisionSummary: json.RawMessage(`{"action":"BuySmall","market_regime":{"primary":"TREND_UP"}}`),
			Explanation:     json.RawMessage(`{"summary":"建議小量試單","action_reason":"主交易區為支撐"}`),
			Scenario:        json.RawMessage(`{"schema_version":"sr_scenario_v1","state":"BuySmall","title":"小量試單情境"}`),
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
					TouchCount: 4, SupportTouchCount: 3, ResistanceTouchCount: 1, RejectCount: &rejectCount, BreakCount: &breakCount,
					ZoneMomentum: -0.02, ZoneDirection: "DOWN",
					RecentValidation: "VALIDATED_RECENTLY",
					TradingScore:     78.5, TradingScoreBreakdown: map[string]float64{
						"expected_value": 26.7, "risk_reward": 13.4, "trend": 10.0, "volume": 10.2, "confidence": 7.2, "chip": 11.0,
					},
					TradingRecommendation: "BUY",
					OverlapGroup:          &overlapGroup, ConfluenceCount: 2,
					Explanation: json.RawMessage(`{"role_summary":"此區為支撐","positive_factors":["信心高"]}`),
					Scenario:    json.RawMessage(`{"schema_version":"sr_scenario_v1","state":"SUPPORT_RETEST"}`),
				},
				{
					PriceLow: 610.0, PriceHigh: 615.0, Method: "volume_profile", Role: "AT_ZONE",
					Tier: "TIER_3_SHORT_TERM", TierLabel: "短期支撐",
					SupportScore: 0.2, ResistanceScore: 0.3, NetScore: -0.1, NetScoreLabel: "NEUTRAL",
					Confidence: 0.4, ConfidenceLevel: "MEDIUM",
					TouchCount: 2, ZoneMomentum: 0.0, ZoneDirection: "FLAT",
					RecentValidation: "PENDING_VALIDATION",
					TradingScore:     45.0, TradingScoreBreakdown: map[string]float64{
						"expected_value": 18.0, "risk_reward": 9.0, "trend": 6.0, "volume": 6.0, "confidence": 3.0, "chip": 3.0,
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
	if result.ModelVersion != "v3" || result.ModelTrainedAt != "2026-06-30T09:00:00+08:00" || len(result.ModelFeatureNames) != 3 {
		t.Fatalf("unexpected model metadata: %+v", result)
	}
	if result.ModelConfigHash != "abc123def456" {
		t.Fatalf("expected model_config_hash to parse, got %q", result.ModelConfigHash)
	}

	a, zones, err := result.ToStore()
	if err != nil {
		t.Fatalf("ToStore failed: %v", err)
	}
	if a.ModelVersion != "v3" {
		t.Fatalf("expected model_version=v3, got %q", a.ModelVersion)
	}
	if a.ModelConfigHash != "abc123def456" {
		t.Fatalf("expected model_config_hash to carry through ToStore, got %q", a.ModelConfigHash)
	}
	if string(a.PeriodSummaries) != `[{"key":"short","label":"短期"}]` {
		t.Fatalf("expected period_summaries to carry through ToStore, got %s", a.PeriodSummaries)
	}
	if string(a.AnalysisTips) != `["短期支撐守穩，籌碼偏多"]` {
		t.Fatalf("expected analysis_tips to carry through ToStore, got %s", a.AnalysisTips)
	}
	if string(a.DecisionSummary) != `{"action":"BuySmall","market_regime":{"primary":"TREND_UP"}}` {
		t.Fatalf("expected decision_summary to carry through ToStore, got %s", a.DecisionSummary)
	}
	if string(a.Explanation) != `{"summary":"建議小量試單","action_reason":"主交易區為支撐"}` {
		t.Fatalf("expected explanation to carry through ToStore, got %s", a.Explanation)
	}
	if string(a.Scenario) != `{"schema_version":"sr_scenario_v1","state":"BuySmall","title":"小量試單情境"}` {
		t.Fatalf("expected scenario to carry through ToStore, got %s", a.Scenario)
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
	if first.SupportTouchCount != 3 || first.ResistanceTouchCount != 1 {
		t.Fatalf("unexpected direction-specific touch counts: %+v", first)
	}
	if !first.OverlapGroup.Valid || first.OverlapGroup.Int64 != 0 || first.ConfluenceCount != 2 {
		t.Fatalf("unexpected overlap group/confluence count: %+v", first)
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
	if string(first.Explanation) != `{"role_summary":"此區為支撐","positive_factors":["信心高"]}` {
		t.Fatalf("expected zone explanation to carry through, got %s", first.Explanation)
	}
	if string(first.Scenario) != `{"schema_version":"sr_scenario_v1","state":"SUPPORT_RETEST"}` {
		t.Fatalf("expected zone scenario to carry through, got %s", first.Scenario)
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
	if second.ConfluenceCount != 1 {
		t.Fatalf("expected missing confluence_count to default to 1, got %d", second.ConfluenceCount)
	}
	if string(second.Explanation) != "null" {
		t.Fatalf("expected missing explanation to default to null, got %s", second.Explanation)
	}
	if string(second.Scenario) != "null" {
		t.Fatalf("expected missing scenario to default to null, got %s", second.Scenario)
	}
}

func TestZoneScoreResultToStoreCarriesChipSummary(t *testing.T) {
	result := ZoneScoreResult{
		Symbol: "2330", Timeframe: "1d", AnalyzedAt: "2026-07-01T13:30:00+08:00",
		CurrentPrice: 600.0, GlobalTrend: 0.01, GlobalVolatility: 0.01,
		ChipSummary: json.RawMessage(`{"missing":false,"score":42.5,"signal":"BULLISH","institutional_score":60.0}`),
	}
	a, _, err := result.ToStore()
	if err != nil {
		t.Fatalf("ToStore failed: %v", err)
	}
	if string(a.ChipSummary) != `{"missing":false,"score":42.5,"signal":"BULLISH","institutional_score":60.0}` {
		t.Fatalf("expected chip_summary to carry through ToStore, got %s", a.ChipSummary)
	}
}

func TestZoneScoreResultToStoreDefaultsMissingChipSummaryToNull(t *testing.T) {
	// Python 舊版（尚未部署本次欄位）不會回傳 chip_summary，ToStore 要給 JSON
	// null 而不是空字串，DB 的 chip_summary 才是合法 JSON。
	result := ZoneScoreResult{
		Symbol: "2330", Timeframe: "1d", AnalyzedAt: "2026-07-01T13:30:00+08:00",
		CurrentPrice: 600.0, GlobalTrend: 0.01, GlobalVolatility: 0.01,
	}
	a, _, err := result.ToStore()
	if err != nil {
		t.Fatalf("ToStore failed: %v", err)
	}
	if string(a.ChipSummary) != "null" {
		t.Fatalf("expected missing chip_summary to default to null, got %s", a.ChipSummary)
	}
}

func TestZoneScoreResultToStoreRejectsIncompleteTradingScoreBreakdown(t *testing.T) {
	result := ZoneScoreResult{
		Symbol: "2330", Timeframe: "1d", AnalyzedAt: "2026-07-01T13:30:00+08:00",
		CurrentPrice: 600.0, GlobalTrend: 0.01, GlobalVolatility: 0.01,
		Zones: []ZoneScore{{
			PriceLow: 580.0, PriceHigh: 585.0, Method: "atr", Role: "SUPPORT",
			TradingScoreBreakdown: map[string]float64{
				"expected_value": 26.7, "risk_reward": 13.4, "trend": 10.0, "volume": 10.2, "confidence": 7.2,
			},
		}},
	}

	_, _, err := result.ToStore()
	if err == nil {
		t.Fatal("expected error when trading_score_breakdown misses chip")
	}
}

func TestZoneScoreResultToStoreDefaultsMissingModelVersionToUnknown(t *testing.T) {
	// 防禦性處理：Python 理論上一定會回傳 model_version，但如果哪天沒有
	// （例如舊版 Python service 還沒部署這次的欄位），DB 裡要看到明確的
	// "unknown" 而不是容易被誤認為「忘了填」的空字串。
	result := ZoneScoreResult{
		Symbol: "2330", Timeframe: "1d", AnalyzedAt: "2026-07-01T13:30:00+08:00",
		CurrentPrice: 600.0, GlobalTrend: 0.01, GlobalVolatility: 0.01,
	}
	a, _, err := result.ToStore()
	if err != nil {
		t.Fatalf("ToStore failed: %v", err)
	}
	if a.ModelVersion != "unknown" {
		t.Fatalf("expected model_version=unknown when Python omits it, got %q", a.ModelVersion)
	}
	if string(a.PeriodSummaries) != "[]" || string(a.AnalysisTips) != "[]" {
		t.Fatalf("expected missing summary JSON to default to [], got %s / %s", a.PeriodSummaries, a.AnalysisTips)
	}
}

func TestZoneScoreResultNestedV2DecodeAndStore(t *testing.T) {
	payload := []byte(`{
		"pipeline_version":"v2",
		"analysis":{"symbol":"2330","timeframe":"1d","analyzed_at":"2026-07-09T00:00:00Z","current_price":600,
			"period_summaries":[{"key":"short","label":"短期"}],
			"analysis_tips":["短期支撐守穩"],
			"chip_summary":{"missing":false,"score":42.5},
			"model":{"version":"v4","trained_at":"2026-07-08T00:00:00Z","config_hash":"cfg","feature_names":["touch_count"]}},
		"features":{"global_trend":0.03,"global_volatility":0.02},
		"score":{"global_expected_value":0.01,"global_confidence":0.7,"global_risk_reward_ratio":1.5},
		"evidence":{"model":{"explainer":"permutation_shap"}},
		"decision":{"action":"BuySmall"},
		"explanation":{"summary":"建議小量試單"},
		"scenario":{"schema_version":"sr_scenario_v1","state":"BuySmall"},
		"zones":[{
			"data":{"price_low":580,"price_high":585,"method":"atr","role":"SUPPORT"},
			"features":{"support":{"touch_count":4},"resistance":{"touch_count":1}},
			"score":{"price_low":580,"price_high":585,"method":"atr","role":"SUPPORT",
				"tier":"TIER_1_MAIN_STRUCTURE","tier_label":"主結構",
				"support_score":0.8,"resistance_score":0.2,"net_score":0.6,"net_score_label":"STRONG_SUPPORT",
				"confidence":0.7,"confidence_level":"HIGH","touch_count":4,"support_touch_count":3,
				"resistance_touch_count":1,"zone_momentum":0.01,"zone_direction":"UP",
				"recent_validation":"VALIDATED_RECENTLY","trading_score":70,
				"trading_score_breakdown":{"expected_value":20,"risk_reward":10,"trend":10,"volume":10,"confidence":5,"chip":15},
				"trading_recommendation":"BUY","confluence_count":1},
			"evidence":{"support":{"targets":{"hold":{"final_probability":0.7}}}},
			"explanation":{"role_summary":"此區為支撐"},
			"scenario":{"schema_version":"sr_scenario_v1","state":"SUPPORT_RETEST"},
			"lifecycle":{"status":"PENDING","resolved_role":null}
		}]
	}`)

	var result ZoneScoreResult
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatalf("decode nested v2 result: %v", err)
	}
	analysis, zones, err := result.ToStore()
	if err != nil {
		t.Fatalf("ToStore nested v2 result: %v", err)
	}
	if analysis.PipelineVersion != "v2" || analysis.ModelVersion != "v4" {
		t.Fatalf("unexpected analysis metadata: %+v", analysis)
	}
	if string(analysis.Evidence) != `{"model":{"explainer":"permutation_shap"}}` {
		t.Fatalf("unexpected analysis evidence: %s", analysis.Evidence)
	}
	if string(analysis.Explanation) != `{"summary":"建議小量試單"}` {
		t.Fatalf("unexpected analysis explanation: %s", analysis.Explanation)
	}
	if string(analysis.Scenario) != `{"schema_version":"sr_scenario_v1","state":"BuySmall"}` {
		t.Fatalf("unexpected analysis scenario: %s", analysis.Scenario)
	}
	if string(analysis.PeriodSummaries) != `[{"key":"short","label":"短期"}]` ||
		string(analysis.AnalysisTips) != `["短期支撐守穩"]` ||
		string(analysis.ChipSummary) != `{"missing":false,"score":42.5}` {
		t.Fatalf("nested analysis summaries not persisted: %+v", analysis)
	}
	if len(zones) != 1 || string(zones[0].Evidence) == "null" || string(zones[0].Features) == "null" || string(zones[0].Explanation) == "null" || string(zones[0].Scenario) == "null" {
		t.Fatalf("nested zone features/evidence/explanation/scenario not persisted: %+v", zones)
	}
}

func TestZoneScoreNestedDecodeRejectsNullOrMissingScore(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{
			name: "null score",
			raw:  `{"data":{"price_low":90},"features":{},"score":null,"evidence":{},"lifecycle":{}}`,
		},
		{
			name: "missing score",
			raw:  `{"data":{"price_low":90},"features":{},"evidence":{},"lifecycle":{}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var zone ZoneScore
			if err := json.Unmarshal([]byte(tt.raw), &zone); err == nil {
				t.Fatalf("expected invalid nested zone to fail, got %+v", zone)
			}
		})
	}
}

func TestScoreZonesReturnsUpstreamStatusErrorOnNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"detail": "no candles found for symbol=2330 timeframe=1d"})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.ScoreZones(context.Background(), "2330", "1d", 0)
	if err == nil {
		t.Fatal("expected error")
	}

	var upstreamErr *UpstreamStatusError
	if !errors.As(err, &upstreamErr) {
		t.Fatalf("expected *UpstreamStatusError, got %T: %v", err, err)
	}
	if upstreamErr.StatusCode != http.StatusNotFound {
		t.Fatalf("expected status=404, got %d", upstreamErr.StatusCode)
	}
}

func TestScoreZonesReturnsUpstreamStatusErrorOnModelNotTrained(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"detail": "sr_scoring 模型檔不存在"})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.ScoreZones(context.Background(), "2330", "1d", 0)
	if err == nil {
		t.Fatal("expected error")
	}

	var upstreamErr *UpstreamStatusError
	if !errors.As(err, &upstreamErr) {
		t.Fatalf("expected *UpstreamStatusError, got %T: %v", err, err)
	}
	if upstreamErr.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected status=503, got %d", upstreamErr.StatusCode)
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
		if len(body.Symbols) != 2 || body.ModelType != "gradient_boosting" || body.SplitMethod != "time" || body.CalibrationMethod != "sigmoid" {
			t.Fatalf("unexpected request body: %+v", body)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TrainResult{
			Rows: 120, Sources: 2, ModelType: "gradient_boosting", SplitMethod: "time",
			Metrics:   map[string]map[string]float64{"hold": {"accuracy": 0.9}, "break": {"accuracy": 0.85}},
			ModelPath: "models/sr_scoring_v1.joblib", TrainedAt: "2026-07-01T13:30:00+08:00", Version: "v1",
			DatasetSummary: map[string]interface{}{"rows": 120.0, "rows_by_symbol": map[string]interface{}{"2330": 80.0, "2454": 40.0}},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	result, err := client.TrainModel(context.Background(), []string{"2330", "2454"}, "1d", 1500, "gradient_boosting", "time", "sigmoid")
	if err != nil {
		t.Fatalf("TrainModel failed: %v", err)
	}
	if result.Rows != 120 || result.Sources != 2 || result.ModelPath != "models/sr_scoring_v1.joblib" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.Metrics["hold"]["accuracy"] != 0.9 {
		t.Fatalf("unexpected metrics: %+v", result.Metrics)
	}
	if result.SplitMethod != "time" {
		t.Fatalf("unexpected split_method: %+v", result)
	}
	if result.DatasetSummary["rows"] != 120.0 {
		t.Fatalf("unexpected dataset_summary: %+v", result.DatasetSummary)
	}
}

func TestTrainModelReturnsErrorWhenBaseURLNotConfigured(t *testing.T) {
	client := NewClient("")
	if _, err := client.TrainModel(context.Background(), []string{"2330"}, "1d", 0, "", "", ""); err == nil {
		t.Fatal("expected error when baseURL is not configured")
	}
}

func TestGetModelStatusParsesResponseWhenModelExists(t *testing.T) {
	version := "v3"
	trainedAt := "2026-07-01T13:30:00+08:00"
	modelPath := "models/sr_scoring_v3.joblib"
	splitMethod := "time"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sr-scoring/model-status" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ModelStatus{
			Exists: true, Version: &version, TrainedAt: &trainedAt, ModelPath: &modelPath, SplitMethod: &splitMethod,
			Metrics:      map[string]map[string]float64{"hold": {"auc": 0.81}, "break": {"auc": 0.77}},
			FeatureNames: []string{"touch_count", "is_support"},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	status, err := client.GetModelStatus(context.Background())
	if err != nil {
		t.Fatalf("GetModelStatus failed: %v", err)
	}
	if !status.Exists || status.Version == nil || *status.Version != "v3" {
		t.Fatalf("unexpected status: %+v", status)
	}
	if len(status.FeatureNames) != 2 {
		t.Fatalf("unexpected feature names: %+v", status.FeatureNames)
	}
}

func TestGetModelStatusParsesResponseWhenModelMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ModelStatus{Exists: false})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	status, err := client.GetModelStatus(context.Background())
	if err != nil {
		t.Fatalf("GetModelStatus failed: %v", err)
	}
	if status.Exists || status.Version != nil {
		t.Fatalf("expected exists=false with no version, got %+v", status)
	}
}

func TestGetModelStatusReturnsErrorWhenBaseURLNotConfigured(t *testing.T) {
	client := NewClient("")
	if _, err := client.GetModelStatus(context.Background()); err == nil {
		t.Fatal("expected error when baseURL is not configured")
	}
}
