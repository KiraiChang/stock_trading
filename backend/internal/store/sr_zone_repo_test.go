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
