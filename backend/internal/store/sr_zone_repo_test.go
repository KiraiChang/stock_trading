package store

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/database"
)

func newTestSRZoneRepo(t *testing.T) SRZoneRepo {
	t.Helper()

	tmp, err := os.CreateTemp("", "sr-zone-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	t.Cleanup(func() { os.Remove(tmp.Name()) })

	db, err := NewDB(config.DatabaseConfig{Driver: "sqlite", DSN: tmp.Name()})
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := database.RunMigrations(context.Background(), db, "sqlite", zap.NewNop()); err != nil {
		t.Fatalf("migrations failed: %v", err)
	}

	return NewSRZoneRepo(db)
}

func testAnalysis() *SRZoneAnalysis {
	return &SRZoneAnalysis{
		Symbol:                "2330",
		Timeframe:             "1d",
		AnalyzedAt:            time.Now().UTC().Truncate(time.Second),
		CurrentPrice:          600.0,
		GlobalTrend:           0.03,
		GlobalVolatility:      0.02,
		GlobalExpectedValue:   NullFloat64{sql.NullFloat64{Float64: 0.004, Valid: true}},
		GlobalConfidence:      NullFloat64{sql.NullFloat64{Float64: 0.6, Valid: true}},
		GlobalRiskRewardRatio: NullFloat64{sql.NullFloat64{Float64: 0.9, Valid: true}},
		ModelVersion:          "v1",
		ModelConfigHash:       "abc123def456",
		PipelineVersion:       "v2",
		Evidence:              RawJSON(`{"model":{"explainer":"permutation_shap"}}`),
		Explanation:           RawJSON(`{"summary":"建議小量試單","action_reason":"主交易區為支撐"}`),
		Scenario:              RawJSON(`{"schema_version":"sr_scenario_v1","state":"BuySmall","title":"小量試單情境"}`),
		ProbabilityContext:    RawJSON(`{"schema_version":"sr_probability_context_v1","health":{"directional_zone_count":1}}`),
		PeriodSummaries:       RawJSON(`[{"key":"short","label":"短期"}]`),
		AnalysisTips:          RawJSON(`["短期支撐守穩，籌碼偏多"]`),
		ChipSummary:           RawJSON(`{"missing":false,"score":42.5,"signal":"BULLISH"}`),
		DecisionSummary:       RawJSON(`{"action":"BuySmall","market_regime":{"primary":"TREND_UP"}}`),
	}
}

func testZones() []SRZone {
	return []SRZone{
		{
			PriceLow: 580.0, PriceHigh: 585.0, Method: "atr", Role: "SUPPORT",
			Tier: "TIER_1_MAIN_STRUCTURE", TierLabel: "主結構",
			SupportScore: 0.8, ResistanceScore: 0.1, NetScore: 0.7, NetScoreLabel: "STRONG_SUPPORT",
			Confidence: 0.83, ConfidenceLevel: "HIGH",
			BounceProbability:    NullFloat64{sql.NullFloat64{Float64: 0.72, Valid: true}},
			BreakProbability:     NullFloat64{sql.NullFloat64{Float64: 0.2, Valid: true}},
			ExpectedGain:         NullFloat64{sql.NullFloat64{Float64: 0.048, Valid: true}},
			ExpectedLoss:         NullFloat64{sql.NullFloat64{Float64: -0.072, Valid: true}},
			ExpectedValue:        NullFloat64{sql.NullFloat64{Float64: 0.0056, Valid: true}},
			RiskRewardRatio:      NullFloat64{sql.NullFloat64{Float64: 0.667, Valid: true}},
			RewardRiskPercentile: NullFloat64{sql.NullFloat64{Float64: 91.0, Valid: true}},
			RelativeVolume:       NullFloat64{sql.NullFloat64{Float64: 1.4, Valid: true}},
			VolumeConfirmation:   NullString{sql.NullString{String: "CONFIRMED", Valid: true}},
			TouchCount:           4, RejectCount: 3, BreakCount: 0,
			ZoneMomentum: -0.02, ZoneDirection: "DOWN",
			RecentValidation:      "VALIDATED_RECENTLY",
			TradingScore:          78.5,
			TradingScoreBreakdown: RawJSON(`{"expected_value":26.7,"risk_reward":13.4,"trend":10,"volume":10.2,"confidence":7.2,"chip":11}`),
			TradingRecommendation: "BUY",
			OverlapGroup:          NullInt64{sql.NullInt64{Int64: 0, Valid: true}},
			ConfluenceCount:       2,
			Features:              RawJSON(`{"support":{"touch_count":4}}`),
			Evidence:              RawJSON(`{"support":{"targets":{"hold":{"final_probability":0.72}}}}`),
			Explanation:           RawJSON(`{"role_summary":"此區為支撐","positive_factors":["信心高"]}`),
			Scenario:              RawJSON(`{"schema_version":"sr_scenario_v1","state":"SUPPORT_RETEST"}`),
			ProbabilityContext:    RawJSON(`{"schema_version":"sr_probability_context_v1","dominant_outcome":"BOUNCE"}`),
		},
		{
			// AT_ZONE：confidence 仍有值，但 expected_value/risk_reward_ratio/volume_confirmation 應為 NULL
			PriceLow: 610.0, PriceHigh: 615.0, Method: "volume_profile", Role: "AT_ZONE",
			Tier: "TIER_3_SHORT_TERM", TierLabel: "短期支撐",
			SupportScore: 0.1, ResistanceScore: 0.65, NetScore: -0.55, NetScoreLabel: "STRONG_RESISTANCE",
			Confidence: 0.4, ConfidenceLevel: "MEDIUM",
			TouchCount: 2, RejectCount: 0, BreakCount: 0,
			ZoneMomentum: 0.0, ZoneDirection: "FLAT",
			RecentValidation:      "PENDING_VALIDATION",
			TradingScore:          45.0,
			TradingScoreBreakdown: RawJSON(`{"expected_value":18,"risk_reward":9,"trend":6,"volume":6,"confidence":3,"chip":3}`),
			TradingRecommendation: "NEUTRAL",
			ConfluenceCount:       1, // 沒有 OverlapGroup（獨立 zone，未跟其他方法重疊）
		},
	}
}

func testProjections() SRZoneNormalizedProjections {
	return SRZoneNormalizedProjections{
		Decision: &SRDecision{
			MarketBias:                "BEARISH_BIAS",
			EntryPermissionState:      "WAIT_CONFIRMATION",
			PositionAction:            "REDUCE_ON_BREAKDOWN",
			PricePathState:            "EVENT_RISK",
			ModelHealthState:          "DEGRADED",
			EventMarketState:          "BREAKDOWN_RISK",
			ReasonCodes:               RawJSON(`["SUPPORT_CLOSED_BELOW","HIGH_VOLUME_BREAKDOWN"]`),
			MarketRegimeJSON:          RawJSON(`{"primary":"TREND_DOWN","label":"偏空"}`),
			DecisionDerivedViewJSON:   RawJSON(`{"version":"decision-derived-view-p0","bias_state":"BEARISH_BIAS","bias_reason_codes":["MARKET_ACTION_AVOID"]}`),
			PricePathJSON:             RawJSON(`{"path_state":"EVENT_RISK","next_decision_price":581}`),
			EntryExecutabilityJSON:    RawJSON(`{"entry_price":581,"executable_now":true,"reason_code":"EXECUTABLE_NOW"}`),
			EntryBlockingZoneJSON:     RawJSON(`{"blocked":false,"distance_price":12,"threshold_price":3}`),
			RRContextJSON:             RawJSON(`{"entry_rr":2.4,"entry_rr_source":"PRIMARY_ZONE","position_rr":null,"position_rr_source":"UNAVAILABLE"}`),
			RRGateJSON:                RawJSON(`{"minimum_rr":1.5,"actual_rr":2.4,"qualified":true,"reason_code":"RR_OK"}`),
			MarketContextJSON:         RawJSON(`[{"key":"trend","label":"趨勢","value":"偏空"}]`),
			ConfidenceExplanationJSON: RawJSON(`{"value":0.72,"level":"HIGH","label":"高","formula_factors":[],"context_factors":[]}`),
			RiskNotesJSON:             RawJSON(`["跌破支撐"]`),
			ZoneSummariesJSON:         RawJSON(`{"nearest_decision_zone":{"label":"580.00 ~ 585.00"},"nearest_support_zone":null,"nearest_resistance_zone":null,"primary_structural_zone":null,"best_trade_zone":{"label":"580.00 ~ 585.00"},"primary_zone":{"label":"580.00 ~ 585.00","role":"SUPPORT"},"secondary_zones":[{"label":"600.00 ~ 605.00"}]}`),
			DecisionSummary:           RawJSON(`{"market_bias":"BEARISH_BIAS","position_action":"REDUCE_ON_BREAKDOWN"}`),
		},
		EventDetections: []MarketEventDetection{{
			EventKey:    "ZONE:BREAKDOWN:SUPPORT:580.0000:585.0000",
			EventType:   "HIGH_VOLUME_BREAKDOWN",
			EventFamily: "BREAKDOWN",
			EventScope:  "ZONE",
			ZoneKey:     "SUPPORT:580.0000:585.0000",
			Direction:   "BEARISH",
			State:       "ACTIVE",
			Active:      true,
			Confidence:  NullFloat64{sql.NullFloat64{Float64: 0.72, Valid: true}},
			PriceLevel:  NullFloat64{sql.NullFloat64{Float64: 580, Valid: true}},
			ReasonCodes: RawJSON(`["HIGH_VOLUME_BREAKDOWN"]`),
			EventJSON:   RawJSON(`{"type":"HIGH_VOLUME_BREAKDOWN","active":true}`),
		}},
		EventStates: []MarketEventState{{
			EventKey:        "ZONE:BREAKDOWN:SUPPORT:580.0000:585.0000",
			EventType:       "HIGH_VOLUME_BREAKDOWN",
			EventFamily:     "BREAKDOWN",
			EventScope:      "ZONE",
			ZoneKey:         "SUPPORT:580.0000:585.0000",
			RootEventType:   "HIGH_VOLUME_BREAKDOWN",
			LatestEventType: "HIGH_VOLUME_BREAKDOWN",
			Direction:       "BEARISH",
			State:           "ACTIVE",
			Active:          true,
			Confidence:      NullFloat64{sql.NullFloat64{Float64: 0.72, Valid: true}},
			PriceLevel:      NullFloat64{sql.NullFloat64{Float64: 580, Valid: true}},
			ReasonCodes:     RawJSON(`["HIGH_VOLUME_BREAKDOWN"]`),
			StateJSON:       RawJSON(`{"type":"HIGH_VOLUME_BREAKDOWN","state":"ACTIVE"}`),
		}},
		DailyCandidates: []SRDailyCandidate{{
			PriceLow:      579.5,
			PriceHigh:     581.0,
			Label:         "579.50 ~ 581.00",
			Role:          "SUPPORT",
			Source:        "DAILY_CANDLE",
			Lifecycle:     "CANDIDATE",
			DecisionRole:  "TACTICAL",
			DistancePct:   NullFloat64{sql.NullFloat64{Float64: 0.012, Valid: true}},
			DistanceLabel: "1.2%",
			Reason:        "日 K 低點與收盤位置形成的短線支撐候選。",
			EventRefs:     RawJSON(`["INTRADAY_RECLAIM"]`),
			CandidateJSON: RawJSON(`{"role":"SUPPORT","source":"DAILY_CANDLE"}`),
		}},
		ModelGovernance: &SRModelGovernance{
			HealthState:            "DEGRADED",
			AverageEdgePP:          NullFloat64{sql.NullFloat64{Float64: 12.5, Valid: true}},
			DirectionalZoneCount:   NullInt64{sql.NullInt64{Int64: 2, Valid: true}},
			ZoneCount:              NullInt64{sql.NullInt64{Int64: 3, Valid: true}},
			AllowEntry:             NullBool{sql.NullBool{Bool: true, Valid: true}},
			MaxEntryState:          "SMALL_ENTRY",
			QualityFlags:           RawJSON(`["HOLD_NOT_CALIBRATED"]`),
			WarningFlags:           RawJSON(`["LOW_AVERAGE_EDGE"]`),
			BlockingFlags:          RawJSON(`[]`),
			ConfidenceGateJSON:     RawJSON(`{"allow_entry":true,"max_entry_state":"SMALL_ENTRY"}`),
			CalibrationReportJSON:  RawJSON(`{"schema_version":"sr_calibration_report_v1"}`),
			WalkForwardReportJSON:  RawJSON(`{"schema_version":"sr_walk_forward_report_v1"}`),
			DatasetDiagnosticsJSON: RawJSON(`{"schema_version":"sr_dataset_diagnostics_v1"}`),
			GovernanceJSON:         RawJSON(`{"health_state":"DEGRADED"}`),
		},
	}
}

func TestSRZoneRepoCreateGetRoundTrip(t *testing.T) {
	repo := newTestSRZoneRepo(t)
	ctx := context.Background()

	id, err := repo.Create(ctx, testAnalysis(), testZones(), testProjections())
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if id == 0 {
		t.Fatalf("expected non-zero id")
	}

	saved, err := repo.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if saved.Symbol != "2330" || saved.Timeframe != "1d" || saved.ModelVersion != "v1" {
		t.Fatalf("unexpected saved analysis: %+v", saved)
	}
	if saved.ModelConfigHash != "abc123def456" {
		t.Fatalf("expected model_config_hash to round-trip, got %q", saved.ModelConfigHash)
	}
	if saved.PipelineVersion != "v2" || string(saved.Evidence) != `{"model":{"explainer":"permutation_shap"}}` {
		t.Fatalf("expected pipeline evidence to round-trip, got %+v", saved)
	}
	if string(saved.Explanation) != `{"summary":"建議小量試單","action_reason":"主交易區為支撐"}` {
		t.Fatalf("expected explanation to round-trip, got %s", saved.Explanation)
	}
	if string(saved.Scenario) != `{"schema_version":"sr_scenario_v1","state":"BuySmall","title":"小量試單情境"}` {
		t.Fatalf("expected scenario to round-trip, got %s", saved.Scenario)
	}
	if string(saved.ProbabilityContext) != `{"schema_version":"sr_probability_context_v1","health":{"directional_zone_count":1}}` {
		t.Fatalf("expected probability_context to round-trip, got %s", saved.ProbabilityContext)
	}
	if string(saved.PeriodSummaries) != `[{"key":"short","label":"短期"}]` {
		t.Fatalf("expected period_summaries to round-trip, got %s", saved.PeriodSummaries)
	}
	if string(saved.AnalysisTips) != `["短期支撐守穩，籌碼偏多"]` {
		t.Fatalf("expected analysis_tips to round-trip, got %s", saved.AnalysisTips)
	}
	if string(saved.ChipSummary) != `{"missing":false,"score":42.5,"signal":"BULLISH"}` {
		t.Fatalf("expected chip_summary to round-trip, got %s", saved.ChipSummary)
	}
	if string(saved.DecisionSummary) != `{"action":"BuySmall","market_regime":{"primary":"TREND_UP"}}` {
		t.Fatalf("expected decision_summary to round-trip, got %s", saved.DecisionSummary)
	}
	if saved.GlobalTrend != 0.03 || saved.GlobalVolatility != 0.02 {
		t.Fatalf("unexpected saved global trend/volatility: %+v", saved)
	}
	if !saved.GlobalExpectedValue.Valid || saved.GlobalExpectedValue.Float64 != 0.004 {
		t.Fatalf("unexpected saved global_expected_value: %+v", saved.GlobalExpectedValue)
	}
	if !saved.GlobalConfidence.Valid || saved.GlobalConfidence.Float64 != 0.6 {
		t.Fatalf("unexpected saved global_confidence: %+v", saved.GlobalConfidence)
	}
	if !saved.GlobalRiskRewardRatio.Valid || saved.GlobalRiskRewardRatio.Float64 != 0.9 {
		t.Fatalf("unexpected saved global_risk_reward_ratio: %+v", saved.GlobalRiskRewardRatio)
	}

	decision, err := repo.GetDecision(ctx, id)
	if err != nil {
		t.Fatalf("GetDecision failed: %v", err)
	}
	if decision.AnalysisID != id || decision.MarketBias != "BEARISH_BIAS" || decision.EventMarketState != "BREAKDOWN_RISK" {
		t.Fatalf("unexpected decision projection: %+v", decision)
	}
	if string(decision.ReasonCodes) != `["SUPPORT_CLOSED_BELOW","HIGH_VOLUME_BREAKDOWN"]` {
		t.Fatalf("unexpected decision reason_codes: %s", decision.ReasonCodes)
	}
	if string(decision.MarketRegimeJSON) != `{"primary":"TREND_DOWN","label":"偏空"}` {
		t.Fatalf("unexpected market_regime_json: %s", decision.MarketRegimeJSON)
	}
	if string(decision.DecisionDerivedViewJSON) != `{"version":"decision-derived-view-p0","bias_state":"BEARISH_BIAS","bias_reason_codes":["MARKET_ACTION_AVOID"]}` {
		t.Fatalf("unexpected decision_derived_view_json: %s", decision.DecisionDerivedViewJSON)
	}
	if string(decision.RRContextJSON) != `{"entry_rr":2.4,"entry_rr_source":"PRIMARY_ZONE","position_rr":null,"position_rr_source":"UNAVAILABLE"}` {
		t.Fatalf("unexpected rr_context_json: %s", decision.RRContextJSON)
	}
	if string(decision.EntryExecutabilityJSON) != `{"entry_price":581,"executable_now":true,"reason_code":"EXECUTABLE_NOW"}` {
		t.Fatalf("unexpected entry_executability_json: %s", decision.EntryExecutabilityJSON)
	}
	if string(decision.EntryBlockingZoneJSON) != `{"blocked":false,"distance_price":12,"threshold_price":3}` {
		t.Fatalf("unexpected entry_blocking_zone_json: %s", decision.EntryBlockingZoneJSON)
	}
	if string(decision.ZoneSummariesJSON) == "" || string(decision.ZoneSummariesJSON) == "null" {
		t.Fatalf("expected zone_summaries_json to round-trip, got %s", decision.ZoneSummariesJSON)
	}

	detections, err := repo.GetMarketEventDetections(ctx, id)
	if err != nil {
		t.Fatalf("GetMarketEventDetections failed: %v", err)
	}
	if len(detections) != 1 || detections[0].EventType != "HIGH_VOLUME_BREAKDOWN" || !detections[0].Active {
		t.Fatalf("unexpected event detections: %+v", detections)
	}

	states, err := repo.GetMarketEventStates(ctx, id)
	if err != nil {
		t.Fatalf("GetMarketEventStates failed: %v", err)
	}
	if len(states) != 1 || states[0].State != "ACTIVE" || states[0].LatestEventType != "HIGH_VOLUME_BREAKDOWN" {
		t.Fatalf("unexpected event states: %+v", states)
	}

	candidates, err := repo.GetDailyCandidates(ctx, id)
	if err != nil {
		t.Fatalf("GetDailyCandidates failed: %v", err)
	}
	if len(candidates) != 1 || candidates[0].Role != "SUPPORT" || candidates[0].Source != "DAILY_CANDLE" {
		t.Fatalf("unexpected daily candidates: %+v", candidates)
	}
	if !candidates[0].DistancePct.Valid || candidates[0].DistancePct.Float64 != 0.012 {
		t.Fatalf("unexpected daily candidate distance_pct: %+v", candidates[0].DistancePct)
	}
	if string(candidates[0].EventRefs) != `["INTRADAY_RECLAIM"]` {
		t.Fatalf("unexpected daily candidate event_refs: %s", candidates[0].EventRefs)
	}

	governance, err := repo.GetModelGovernance(ctx, id)
	if err != nil {
		t.Fatalf("GetModelGovernance failed: %v", err)
	}
	if governance.AnalysisID != id || governance.HealthState != "DEGRADED" || governance.ModelVersion != "v1" {
		t.Fatalf("unexpected model governance: %+v", governance)
	}
	if !governance.AllowEntry.Valid || !governance.AllowEntry.Bool || governance.MaxEntryState != "SMALL_ENTRY" {
		t.Fatalf("unexpected confidence gate projection: %+v", governance)
	}
	if string(governance.QualityFlags) != `["HOLD_NOT_CALIBRATED"]` {
		t.Fatalf("unexpected model governance quality_flags: %s", governance.QualityFlags)
	}

	zones, err := repo.GetZones(ctx, id)
	if err != nil {
		t.Fatalf("GetZones failed: %v", err)
	}
	if len(zones) != 2 {
		t.Fatalf("expected 2 zones, got %d", len(zones))
	}
	if string(zones[0].Features) == "null" || string(zones[0].Evidence) == "null" {
		t.Fatalf("expected zone features/evidence to round-trip: %+v", zones[0])
	}
	if string(zones[0].Explanation) == "null" {
		t.Fatalf("expected zone explanation to round-trip: %+v", zones[0])
	}
	if string(zones[0].Scenario) == "null" {
		t.Fatalf("expected zone scenario to round-trip: %+v", zones[0])
	}
	if string(zones[0].ProbabilityContext) == "null" {
		t.Fatalf("expected zone probability_context to round-trip: %+v", zones[0])
	}
	for _, z := range zones {
		if z.AnalysisID != id {
			t.Fatalf("expected zone.AnalysisID=%d, got %d", id, z.AnalysisID)
		}
		if z.Status != "PENDING" {
			t.Fatalf("expected default status PENDING, got %s", z.Status)
		}
	}

	var support, atZone SRZone
	for _, z := range zones {
		switch z.Role {
		case "SUPPORT":
			support = z
		case "AT_ZONE":
			atZone = z
		}
	}
	if support.Confidence != 0.83 || support.ConfidenceLevel != "HIGH" {
		t.Fatalf("expected SUPPORT confidence=0.83/HIGH, got %v/%v", support.Confidence, support.ConfidenceLevel)
	}
	if support.NetScoreLabel != "STRONG_SUPPORT" {
		t.Fatalf("expected SUPPORT net_score_label=STRONG_SUPPORT, got %v", support.NetScoreLabel)
	}
	if support.Tier != "TIER_1_MAIN_STRUCTURE" || support.TierLabel != "主結構" {
		t.Fatalf("expected SUPPORT tier=TIER_1_MAIN_STRUCTURE/主結構, got %v/%v", support.Tier, support.TierLabel)
	}
	if len(support.TradingScoreBreakdown) == 0 {
		t.Fatalf("expected non-empty trading_score_breakdown, got %+v", support)
	}
	if !support.ExpectedGain.Valid || support.ExpectedGain.Float64 != 0.048 {
		t.Fatalf("expected SUPPORT expected_gain=0.048, got %+v", support.ExpectedGain)
	}
	if !support.ExpectedLoss.Valid || support.ExpectedLoss.Float64 != -0.072 {
		t.Fatalf("expected SUPPORT expected_loss=-0.072, got %+v", support.ExpectedLoss)
	}
	if !support.ExpectedValue.Valid || support.ExpectedValue.Float64 != 0.0056 {
		t.Fatalf("expected SUPPORT expected_value=0.0056, got %+v", support.ExpectedValue)
	}
	if !support.RiskRewardRatio.Valid || support.RiskRewardRatio.Float64 != 0.667 {
		t.Fatalf("expected SUPPORT risk_reward_ratio=0.667, got %+v", support.RiskRewardRatio)
	}
	if !support.VolumeConfirmation.Valid || support.VolumeConfirmation.String != "CONFIRMED" {
		t.Fatalf("expected SUPPORT volume_confirmation=CONFIRMED, got %+v", support.VolumeConfirmation)
	}
	if support.TradingRecommendation != "BUY" {
		t.Fatalf("expected SUPPORT trading_recommendation=BUY, got %v", support.TradingRecommendation)
	}
	if !support.OverlapGroup.Valid || support.OverlapGroup.Int64 != 0 || support.ConfluenceCount != 2 {
		t.Fatalf("expected SUPPORT overlap_group=0/confluence_count=2, got %+v/%v", support.OverlapGroup, support.ConfluenceCount)
	}

	if atZone.Confidence != 0.4 {
		t.Fatalf("expected AT_ZONE confidence=0.4, got %v", atZone.Confidence)
	}
	if atZone.OverlapGroup.Valid || atZone.ConfluenceCount != 1 {
		t.Fatalf("expected AT_ZONE overlap_group=NULL/confluence_count=1, got %+v/%v", atZone.OverlapGroup, atZone.ConfluenceCount)
	}
	if atZone.ExpectedValue.Valid || atZone.RiskRewardRatio.Valid || atZone.VolumeConfirmation.Valid {
		t.Fatalf(
			"expected AT_ZONE to have NULL expected_value/risk_reward_ratio/volume_confirmation, got %+v / %+v / %+v",
			atZone.ExpectedValue, atZone.RiskRewardRatio, atZone.VolumeConfirmation,
		)
	}
}

func TestSRZoneRepoGetLatestMarketEventStatesUsesNewestSnapshot(t *testing.T) {
	repo := newTestSRZoneRepo(t)
	ctx := context.Background()

	older := testAnalysis()
	older.AnalyzedAt = time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	olderProjections := testProjections()
	olderProjections.EventStates = []MarketEventState{{
		EventKey:        "ZONE:SUPPORT_BREAKDOWN:SUPPORT:580.0000:585.0000",
		EventType:       "HIGH_VOLUME_BREAKDOWN",
		EventFamily:     "SUPPORT_BREAKDOWN",
		EventScope:      "ZONE",
		ZoneKey:         "SUPPORT:580.0000:585.0000",
		RootEventType:   "HIGH_VOLUME_BREAKDOWN",
		LatestEventType: "HIGH_VOLUME_BREAKDOWN",
		Direction:       "BEARISH",
		State:           "CONFIRMED",
		Active:          true,
		ReasonCodes:     RawJSON(`["OLDER_BREAKDOWN"]`),
		StateJSON:       RawJSON(`{"type":"HIGH_VOLUME_BREAKDOWN","state":"CONFIRMED"}`),
	}}
	if _, err := repo.Create(ctx, older, testZones(), olderProjections); err != nil {
		t.Fatalf("Create older failed: %v", err)
	}

	newer := testAnalysis()
	newer.AnalyzedAt = time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	newerProjections := testProjections()
	newerProjections.EventStates = []MarketEventState{
		{
			EventKey:        "ZONE:SUPPORT_RECLAIM:SUPPORT:580.0000:585.0000",
			EventType:       "INTRADAY_RECLAIM",
			EventFamily:     "SUPPORT_RECLAIM",
			EventScope:      "ZONE",
			ZoneKey:         "SUPPORT:580.0000:585.0000",
			RootEventType:   "INTRADAY_RECLAIM",
			LatestEventType: "INTRADAY_RECLAIM",
			Direction:       "BULLISH",
			State:           "CONFIRMED",
			Active:          true,
			ReasonCodes:     RawJSON(`["NEWER_RECLAIM"]`),
			StateJSON:       RawJSON(`{"type":"INTRADAY_RECLAIM","state":"CONFIRMED"}`),
		},
		{
			EventKey:        "ZONE:SUPPORT_BREAKDOWN:SUPPORT:580.0000:585.0000",
			EventType:       "HIGH_VOLUME_BREAKDOWN",
			EventFamily:     "SUPPORT_BREAKDOWN",
			EventScope:      "ZONE",
			ZoneKey:         "SUPPORT:580.0000:585.0000",
			RootEventType:   "HIGH_VOLUME_BREAKDOWN",
			LatestEventType: "INTRADAY_RECLAIM",
			Direction:       "BEARISH",
			State:           "RESOLVED",
			Active:          false,
			ReasonCodes:     RawJSON(`["RESOLVED_BY_RECLAIM"]`),
			StateJSON:       RawJSON(`{"type":"HIGH_VOLUME_BREAKDOWN","state":"RESOLVED"}`),
		},
	}
	if _, err := repo.Create(ctx, newer, testZones(), newerProjections); err != nil {
		t.Fatalf("Create newer failed: %v", err)
	}

	states, err := repo.GetLatestMarketEventStates(ctx, "2330", "1d")
	if err != nil {
		t.Fatalf("GetLatestMarketEventStates failed: %v", err)
	}
	if len(states) != 2 {
		t.Fatalf("expected all latest lifecycle states, got %+v", states)
	}
	if states[0].EventType != "INTRADAY_RECLAIM" || states[0].ReasonCodes != RawJSON(`["NEWER_RECLAIM"]`) {
		t.Fatalf("unexpected latest active state: %+v", states[0])
	}
	if states[1].EventType != "HIGH_VOLUME_BREAKDOWN" || states[1].State != "RESOLVED" || states[1].Active {
		t.Fatalf("expected latest resolved state in lifecycle snapshot: %+v", states[1])
	}
}

func TestSRZoneRepoCreateDefaultsEmptyChipSummaryToNull(t *testing.T) {
	repo := newTestSRZoneRepo(t)
	ctx := context.Background()

	a := testAnalysis()
	a.ChipSummary = "" // Python 舊版沒帶 chip_summary 時 client 會給空值
	id, err := repo.Create(ctx, a, testZones(), SRZoneNormalizedProjections{})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	saved, err := repo.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if string(saved.ChipSummary) != "null" {
		t.Fatalf("expected empty chip_summary to default to JSON null, got %s", saved.ChipSummary)
	}
}

// 這個欄位有兩處欄位清單要維護（INSERT 的 cols 與 SELECT 的 srZoneAnalysisColumns），
// 只改一處會變成「寫得進去、讀不出來」，所以要有 round-trip 測試鎖住。
func TestSRZoneRepoRoundTripsZoneBuilderRuntimeConfig(t *testing.T) {
	repo := newTestSRZoneRepo(t)
	ctx := context.Background()

	a := testAnalysis()
	a.ZoneBuilderRuntimeConfig = RawJSON(`{"enabled":true,"bucket":"HIGH_VOLATILITY","reason_code":"VOLATILITY_BUCKET_CONFIG"}`)
	id, err := repo.Create(ctx, a, testZones(), SRZoneNormalizedProjections{})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	saved, err := repo.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if string(saved.ZoneBuilderRuntimeConfig) != string(a.ZoneBuilderRuntimeConfig) {
		t.Fatalf("zone_builder_runtime_config not round-tripped: got %s", saved.ZoneBuilderRuntimeConfig)
	}
}

// 空字串會被 DB 的 NOT NULL JSON 欄位拒絕，也代表不了「無紀錄」，要落成 JSON null。
func TestSRZoneRepoCreateDefaultsEmptyZoneBuilderRuntimeConfigToNull(t *testing.T) {
	repo := newTestSRZoneRepo(t)
	ctx := context.Background()

	a := testAnalysis()
	a.ZoneBuilderRuntimeConfig = ""
	id, err := repo.Create(ctx, a, testZones(), SRZoneNormalizedProjections{})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	saved, err := repo.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if string(saved.ZoneBuilderRuntimeConfig) != "null" {
		t.Fatalf("expected empty zone_builder_runtime_config to default to JSON null, got %s", saved.ZoneBuilderRuntimeConfig)
	}
}

func TestSRZoneRepoListFiltersBySymbol(t *testing.T) {
	repo := newTestSRZoneRepo(t)
	ctx := context.Background()

	a1 := testAnalysis()
	a1.Symbol = "2330"
	if _, err := repo.Create(ctx, a1, testZones(), SRZoneNormalizedProjections{}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	a2 := testAnalysis()
	a2.Symbol = "2454"
	if _, err := repo.Create(ctx, a2, testZones(), SRZoneNormalizedProjections{}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	rows, err := repo.List(ctx, "2330", 20)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(rows) != 1 || rows[0].Symbol != "2330" {
		t.Fatalf("expected 1 row for symbol=2330, got %+v", rows)
	}

	all, err := repo.List(ctx, "", 20)
	if err != nil {
		t.Fatalf("List(all) failed: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 rows total, got %d", len(all))
	}
}

func TestSRZoneRepoUpdateZoneStatus(t *testing.T) {
	repo := newTestSRZoneRepo(t)
	ctx := context.Background()

	id, err := repo.Create(ctx, testAnalysis(), testZones(), testProjections())
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	zones, err := repo.GetZones(ctx, id)
	if err != nil {
		t.Fatalf("GetZones failed: %v", err)
	}
	target := zones[0]

	brokenAt := time.Now().UTC().Truncate(time.Second)
	brokenPrice := 88.5
	if err := repo.UpdateZoneStatus(ctx, target.ID, "BROKEN", &brokenAt, &brokenPrice, ""); err != nil {
		t.Fatalf("UpdateZoneStatus failed: %v", err)
	}

	updated, err := repo.GetZones(ctx, id)
	if err != nil {
		t.Fatalf("GetZones failed: %v", err)
	}
	var found *SRZone
	for i := range updated {
		if updated[i].ID == target.ID {
			found = &updated[i]
		}
	}
	if found == nil {
		t.Fatalf("expected to find zone id=%d", target.ID)
	}
	if found.Status != "BROKEN" {
		t.Fatalf("expected status=BROKEN, got %s", found.Status)
	}
	if !found.BrokenAt.Valid || !found.BrokenAt.Time.Equal(brokenAt) {
		t.Fatalf("unexpected broken_at: %+v", found.BrokenAt)
	}
	if !found.BrokenPrice.Valid || found.BrokenPrice.Float64 != brokenPrice {
		t.Fatalf("unexpected broken_price: %+v", found.BrokenPrice)
	}
	if found.ResolvedRole.Valid {
		t.Fatalf("expected resolved_role to stay NULL when resolvedRole=\"\", got %+v", found.ResolvedRole)
	}
}

// 【resolved_role 持久化，見 docs/sr-zone-scoring.md 十五】AT_ZONE 驗證後解析出的方向要能被存回並讀出，
// 不能只更新 status/broken_at/broken_price 而遺漏 resolved_role。
func TestSRZoneRepoUpdateZoneStatusPersistsResolvedRole(t *testing.T) {
	repo := newTestSRZoneRepo(t)
	ctx := context.Background()

	id, err := repo.Create(ctx, testAnalysis(), testZones(), SRZoneNormalizedProjections{})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	zones, err := repo.GetZones(ctx, id)
	if err != nil {
		t.Fatalf("GetZones failed: %v", err)
	}
	var target SRZone
	for _, z := range zones {
		if z.Role == "AT_ZONE" {
			target = z
			break
		}
	}
	if target.ID == 0 {
		t.Fatal("expected test data to include an AT_ZONE zone")
	}

	if err := repo.UpdateZoneStatus(ctx, target.ID, "HELD_SO_FAR", nil, nil, "SUPPORT"); err != nil {
		t.Fatalf("UpdateZoneStatus failed: %v", err)
	}

	updated, err := repo.GetZones(ctx, id)
	if err != nil {
		t.Fatalf("GetZones failed: %v", err)
	}
	var found *SRZone
	for i := range updated {
		if updated[i].ID == target.ID {
			found = &updated[i]
		}
	}
	if found == nil {
		t.Fatalf("expected to find zone id=%d", target.ID)
	}
	if !found.ResolvedRole.Valid || found.ResolvedRole.String != "SUPPORT" {
		t.Fatalf("expected resolved_role=SUPPORT, got %+v", found.ResolvedRole)
	}
}

func TestSRZoneRepoUpdateZoneStatusIgnoresResolvedRoleForDirectionalZone(t *testing.T) {
	repo := newTestSRZoneRepo(t)
	ctx := context.Background()

	id, err := repo.Create(ctx, testAnalysis(), testZones(), SRZoneNormalizedProjections{})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	zones, err := repo.GetZones(ctx, id)
	if err != nil {
		t.Fatalf("GetZones failed: %v", err)
	}
	var target SRZone
	for _, z := range zones {
		if z.Role == "SUPPORT" {
			target = z
			break
		}
	}
	if target.ID == 0 {
		t.Fatal("expected test data to include a SUPPORT zone")
	}

	if err := repo.UpdateZoneStatus(ctx, target.ID, "HELD_SO_FAR", nil, nil, "SUPPORT"); err != nil {
		t.Fatalf("UpdateZoneStatus failed: %v", err)
	}

	updated, err := repo.GetZones(ctx, id)
	if err != nil {
		t.Fatalf("GetZones failed: %v", err)
	}
	for _, z := range updated {
		if z.ID == target.ID && z.ResolvedRole.Valid {
			t.Fatalf("expected non-AT_ZONE resolved_role to remain NULL, got %+v", z.ResolvedRole)
		}
	}
}

func TestSRZoneRepoDeleteCascadesZones(t *testing.T) {
	repo := newTestSRZoneRepo(t)
	ctx := context.Background()

	id, err := repo.Create(ctx, testAnalysis(), testZones(), SRZoneNormalizedProjections{})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := repo.Delete(ctx, id); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if _, err := repo.Get(ctx, id); err == nil {
		t.Fatalf("expected error getting deleted analysis")
	}
	zones, err := repo.GetZones(ctx, id)
	if err != nil {
		t.Fatalf("GetZones after delete failed: %v", err)
	}
	if len(zones) != 0 {
		t.Fatalf("expected 0 zones after cascade delete, got %d", len(zones))
	}
	candidates, err := repo.GetDailyCandidates(ctx, id)
	if err != nil {
		t.Fatalf("GetDailyCandidates after delete failed: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected 0 daily candidates after delete, got %d", len(candidates))
	}
	if _, err := repo.GetModelGovernance(ctx, id); err == nil {
		t.Fatalf("expected error getting deleted model governance")
	}
}

// TestListMarketEventStateHistoryReturnsFullSnapshots：跨分析取歷史序列。
//
// **最容易寫錯的是參數綁定**：查詢的 filter 出現兩次（外層 WHERE 與內層挑 analysis_id 的
// 子查詢），參數也必須帶兩份，順序錯了會查出完全不同的結果卻不報錯。
func TestListMarketEventStateHistoryReturnsFullSnapshots(t *testing.T) {
	repo := newTestSRZoneRepo(t)
	ctx := context.Background()

	state := func(zoneKey, stateName string) MarketEventState {
		return MarketEventState{
			EventKey: "ZONE:SUPPORT_RECLAIM:" + zoneKey, EventType: "INTRADAY_RECLAIM",
			EventFamily: "SUPPORT_RECLAIM", EventScope: "ZONE", ZoneKey: zoneKey,
			RootEventType: "INTRADAY_RECLAIM", LatestEventType: "INTRADAY_RECLAIM",
			Direction: "BULLISH", State: stateName, Active: stateName == "CONFIRMED",
			ReasonCodes: RawJSON(`["CLOSE_RECLAIM"]`), StateJSON: RawJSON(`{}`),
		}
	}

	for i, spec := range []struct {
		day   int
		first string
	}{{20, "CANDIDATE"}, {21, "CONFIRMED"}, {22, "RESOLVED"}} {
		a := testAnalysis()
		a.AnalyzedAt = time.Date(2026, 7, spec.day, 0, 0, 0, 0, time.UTC)
		p := testProjections()
		// 每份快照兩列，用來驗證不會在快照中間被截斷
		p.EventStates = []MarketEventState{state("SUPPORT:580:585", spec.first), state("SUPPORT:600:605", "CANDIDATE")}
		if _, err := repo.Create(ctx, a, testZones(), p); err != nil {
			t.Fatalf("Create #%d failed: %v", i, err)
		}
	}

	rows, err := repo.ListMarketEventStateHistory(ctx, MarketEventStateHistoryOptions{
		Symbol: "2330", Timeframe: "1d",
	})
	if err != nil {
		t.Fatalf("ListMarketEventStateHistory failed: %v", err)
	}
	if len(rows) != 6 {
		t.Fatalf("列數 = %d, want 6（3 次分析 × 每份 2 列）", len(rows))
	}
	// 必須依 analyzed_at 遞增——摺疊邏輯依賴順序
	for i := 1; i < len(rows); i++ {
		if rows[i].AnalyzedAt.Before(rows[i-1].AnalyzedAt) {
			t.Fatalf("第 %d 列的 analyzed_at 早於前一列，排序不對", i)
		}
	}
	if rows[0].State != "CANDIDATE" || rows[len(rows)-1].AnalyzedAt.Day() != 22 {
		t.Errorf("序列頭尾不符：first=%q lastDay=%d", rows[0].State, rows[len(rows)-1].AnalyzedAt.Day())
	}
}

// MaxAnalyses 以**分析次數**為單位而不是列數：對列數截斷會切出半份快照，
// 摺疊時把被切掉的事件誤判成消失，產生不存在的 transition。
func TestListMarketEventStateHistoryLimitsByAnalysisNotRows(t *testing.T) {
	repo := newTestSRZoneRepo(t)
	ctx := context.Background()

	for day := 20; day <= 22; day++ {
		a := testAnalysis()
		a.AnalyzedAt = time.Date(2026, 7, day, 0, 0, 0, 0, time.UTC)
		p := testProjections()
		p.EventStates = []MarketEventState{
			{EventKey: "A", EventType: "INTRADAY_RECLAIM", EventFamily: "SUPPORT_RECLAIM", EventScope: "ZONE",
				ZoneKey: "Z1", RootEventType: "INTRADAY_RECLAIM", LatestEventType: "INTRADAY_RECLAIM",
				Direction: "BULLISH", State: "CANDIDATE", ReasonCodes: RawJSON(`[]`), StateJSON: RawJSON(`{}`)},
			{EventKey: "B", EventType: "INTRADAY_RECLAIM", EventFamily: "SUPPORT_RECLAIM", EventScope: "ZONE",
				ZoneKey: "Z2", RootEventType: "INTRADAY_RECLAIM", LatestEventType: "INTRADAY_RECLAIM",
				Direction: "BULLISH", State: "CANDIDATE", ReasonCodes: RawJSON(`[]`), StateJSON: RawJSON(`{}`)},
		}
		if _, err := repo.Create(ctx, a, testZones(), p); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	rows, err := repo.ListMarketEventStateHistory(ctx, MarketEventStateHistoryOptions{
		Symbol: "2330", Timeframe: "1d", MaxAnalyses: 2,
	})
	if err != nil {
		t.Fatalf("ListMarketEventStateHistory failed: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("列數 = %d, want 4（2 次分析 × 每份 2 列，完整不截斷）", len(rows))
	}
	// 取的是最近兩次，所以最早那天不該出現
	for _, r := range rows {
		if r.AnalyzedAt.Day() == 20 {
			t.Error("MaxAnalyses=2 卻回傳了第三新的分析")
		}
	}
}

// T-048 階段 E：zone 身分要能跟著 zones 一次寫入並原樣讀回。
//
// 這條測的是 INSERT／SELECT 的欄位有沒有同步——named 參數少一個不會編譯失敗，
// 只會靜靜地把 zone_uid 寫成 NULL，而那個結果與「這次分析沒有身分」長得一模一樣。
func TestSRZoneRepoRoundTripsZoneUID(t *testing.T) {
	repo := newTestSRZoneRepo(t)
	ctx := context.Background()

	zones := testZones()
	zones[0].ZoneUID = NullString{sql.NullString{String: "Z-abc", Valid: true}}
	// zones[1] 刻意不給：可空欄位要能與「有值」並存在同一次寫入裡。

	id, err := repo.Create(ctx, testAnalysis(), zones, SRZoneNormalizedProjections{})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	saved, err := repo.GetZones(ctx, id)
	if err != nil {
		t.Fatalf("get zones failed: %v", err)
	}
	if len(saved) != len(zones) {
		t.Fatalf("want %d zones, got %d", len(zones), len(saved))
	}

	var withUID, withoutUID int
	for _, z := range saved {
		if z.ZoneUID.Valid {
			withUID++
			if z.ZoneUID.String != "Z-abc" {
				t.Errorf("zone_uid = %q, want Z-abc", z.ZoneUID.String)
			}
			continue
		}
		withoutUID++
	}
	if withUID != 1 || withoutUID != len(zones)-1 {
		t.Errorf("want 1 帶身分 / %d 不帶，got %d / %d", len(zones)-1, withUID, withoutUID)
	}
}

// seedAnalysisAt 建一筆分析並把 created_at 改成指定時間。
// Create 只會寫 CURRENT_TIMESTAMP，驗不了「跨多天的窗口怎麼取」。
func seedAnalysisAt(t *testing.T, repo SRZoneRepo, symbol string, createdAt time.Time) uint64 {
	t.Helper()
	ctx := context.Background()
	a := testAnalysis()
	a.Symbol = symbol
	id, err := repo.Create(ctx, a, testZones(), SRZoneNormalizedProjections{})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	r := repo.(*srZoneRepo)
	if _, err := r.db.ExecContext(ctx,
		r.db.Rebind(`UPDATE stock_sr_zone_analyses SET created_at=? WHERE id=?`),
		createdAt, id,
	); err != nil {
		t.Fatalf("seed created_at failed: %v", err)
	}
	return id
}

// ListRefsSince 的本體：只回窗口內的分析，最新的優先。
//
// **對照組在同一條裡**：窗口外那筆一定不能出現。少了它，這條測試會在
// 「WHERE 條件寫錯、全部都回」時假綠——而那正是最容易寫錯的地方。
func TestSRZoneRepoListRefsSinceFiltersByWindow(t *testing.T) {
	repo := newTestSRZoneRepo(t)
	now := time.Now().UTC().Truncate(time.Second)
	since := now.AddDate(0, 0, -30)

	outside := seedAnalysisAt(t, repo, "OUTSIDE", since.AddDate(0, 0, -1)) // 31 天前
	older := seedAnalysisAt(t, repo, "OLDER", now.AddDate(0, 0, -20))
	newer := seedAnalysisAt(t, repo, "NEWER", now.AddDate(0, 0, -1))

	rows, err := repo.ListRefsSince(context.Background(), since, 100)
	if err != nil {
		t.Fatalf("ListRefsSince failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("回傳 %d 筆，期望 2 筆（窗口外那筆不該進來）", len(rows))
	}
	if rows[0].ID != newer || rows[1].ID != older {
		t.Errorf("順序 = [%d %d]，期望最新的先（[%d %d]）", rows[0].ID, rows[1].ID, newer, older)
	}
	// **Symbol 也要斷言**：Ref 只有 ID 與 Symbol 兩個欄位，而 Symbol 是
	// sr_zone_verify 失敗時 log 裡唯一能指出「哪一檔」的線索。少了這兩行，
	// 把 SQL 改成 `SELECT id, '' AS symbol` 也會全綠——那時排程照跑，
	// 但驗證失敗的 log 會少掉標的、沒人看得出是哪檔出事。
	if rows[0].Symbol != "NEWER" || rows[1].Symbol != "OLDER" {
		t.Errorf("Symbol = [%q %q]，期望 [\"NEWER\" \"OLDER\"]", rows[0].Symbol, rows[1].Symbol)
	}
	for _, r := range rows {
		if r.ID == outside {
			t.Error("窗口外的分析不該出現在結果裡")
		}
	}
}

// 硬上限：窗口再大也不會一次撈爆，而且截斷時留下的是**最新的**那些。
func TestSRZoneRepoListRefsSinceRespectsMaxAnalyses(t *testing.T) {
	repo := newTestSRZoneRepo(t)
	now := time.Now().UTC().Truncate(time.Second)

	var newest uint64
	for i := 0; i < 5; i++ {
		id := seedAnalysisAt(t, repo, "2330", now.Add(-time.Duration(i)*time.Hour))
		if i == 0 {
			newest = id
		}
	}

	rows, err := repo.ListRefsSince(context.Background(), now.AddDate(0, 0, -30), 2)
	if err != nil {
		t.Fatalf("ListRefsSince failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("回傳 %d 筆，期望被上限截成 2 筆", len(rows))
	}
	if rows[0].ID != newest {
		t.Errorf("截斷後第一筆 = %d，期望最新那筆 %d", rows[0].ID, newest)
	}
}

// 同一秒的多筆分析在撞到上限時必須有確定順序。
//
// **這是真實情境不是造出來的邊界**：created_at 只有秒級精度，而排程一輪會連續
// 分析 watchlist 全部標的，同一秒寫進好幾筆很正常。沒有 id 決勝時，同秒那批由
// 資料庫任意排序，截斷後留下哪幾筆會在不同引擎、不同執行計畫之間漂移——某筆分析
// 就可能一直輪不到驗證。
//
// **這條測試能保護什麼、不能保護什麼（2026-08-25 變異測試實測）**：
//   - 抓得到：tiebreak 方向寫反（id ASC）、ORDER BY 被整段改掉。
//   - **抓不到：ORDER BY 少了 id DESC 這件事本身。** 把查詢改回
//     `ORDER BY created_at DESC` 之後這條照樣 PASS——sqlite 在 created_at 相同時
//     碰巧就是以 rowid 遞減回傳。
//
// 也就是說 id DESC 的正當性**不是這條測試證明的**，而是「跨引擎確定性」這個論證：
// mysql / postgres 沒有義務比照 sqlite 的巧合順序。單元測試只跑 sqlite，
// 本質上驗不出這種跨引擎差異——不要因為這條是綠的就以為那句 ORDER BY 有測試保護。
func TestSRZoneRepoListRefsSinceIsDeterministicWithinSameSecond(t *testing.T) {
	repo := newTestSRZoneRepo(t)
	now := time.Now().UTC().Truncate(time.Second)

	// 五筆全部落在同一秒，id 遞增。
	ids := make([]uint64, 0, 5)
	for i := 0; i < 5; i++ {
		ids = append(ids, seedAnalysisAt(t, repo, "2330", now))
	}
	// 期望截斷後留下 id 最大的三筆，由大到小。
	want := []uint64{ids[4], ids[3], ids[2]}

	// 連續查兩次，除了內容要對，兩次也必須一致。
	var first []uint64
	for round := 0; round < 2; round++ {
		rows, err := repo.ListRefsSince(context.Background(), now.AddDate(0, 0, -1), 3)
		if err != nil {
			t.Fatalf("round %d: ListRefsSince failed: %v", round, err)
		}
		if len(rows) != 3 {
			t.Fatalf("round %d: 回傳 %d 筆，期望被上限截成 3 筆", round, len(rows))
		}
		got := []uint64{rows[0].ID, rows[1].ID, rows[2].ID}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("round %d: 順序 = %v，期望 %v（同秒時以 id DESC 決勝）", round, got, want)
				break
			}
		}
		if round == 0 {
			first = got
		} else {
			for i := range first {
				if got[i] != first[i] {
					t.Errorf("兩次查詢結果不同：%v vs %v——同秒排序不確定", first, got)
					break
				}
			}
		}
	}
}
