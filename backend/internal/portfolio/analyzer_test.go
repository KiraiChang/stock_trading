package portfolio

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/trading/backend/internal/store"
)

func TestBuildSnapshotFlatAndLongActions(t *testing.T) {
	a := &Analyzer{config: DefaultConfig()}
	sr := &store.SRZoneAnalysis{
		ID: 9, Symbol: "2330", AnalyzedAt: time.Now(), CurrentPrice: 100,
		DecisionSummary: store.RawJSON(`{"action":"BuySmall"}`),
	}
	zones := []store.SRZone{
		{ID: 1, Role: "SUPPORT", PriceLow: 90, PriceHigh: 92, Status: "PENDING", TradingScore: 80},
		{ID: 2, Role: "RESISTANCE", PriceLow: 120, PriceHigh: 122, Status: "PENDING", TradingScore: 70},
	}
	flat, err := a.buildSnapshot(&store.Position{Symbol: "2330"}, sr, zones)
	if err != nil {
		t.Fatal(err)
	}
	if flat.PositionState != StateFlat || flat.Action != ActionEnterSmall || flat.TargetShares != 500 {
		t.Fatalf("unexpected FLAT analysis: %+v", flat)
	}
	var flatEvidence map[string]any
	if err := json.Unmarshal([]byte(flat.Evidence), &flatEvidence); err != nil {
		t.Fatal(err)
	}
	flatContext := flatEvidence["decision_context"].(map[string]any)
	if flatContext["mode"] != "FLAT_ENTRY" || flatContext["has_position"].(bool) {
		t.Fatalf("unexpected flat decision_context: %+v", flatContext)
	}
	flatEntry := flatEvidence["entry_decision"].(map[string]any)
	flatPositionDecision := flatEvidence["position_decision"].(map[string]any)
	if !flatEntry["applicable"].(bool) || flatEntry["state"] != ActionEnterSmall {
		t.Fatalf("unexpected flat entry_decision: %+v", flatEntry)
	}
	if flatPositionDecision["applicable"].(bool) || flatPositionDecision["state"] != "NOT_APPLICABLE" {
		t.Fatalf("unexpected flat position_decision: %+v", flatPositionDecision)
	}
	long, err := a.buildSnapshot(&store.Position{Symbol: "2330", Shares: 700, AvgCost: 95, Version: 3}, sr, zones)
	if err != nil {
		t.Fatal(err)
	}
	if long.PositionState != StateLong || long.Action != ActionReduce || long.TargetShares != 500 || long.AdjustmentShares != -200 {
		t.Fatalf("unexpected LONG analysis: %+v", long)
	}
	var longEvidence map[string]any
	if err := json.Unmarshal([]byte(long.Evidence), &longEvidence); err != nil {
		t.Fatal(err)
	}
	longContext := longEvidence["decision_context"].(map[string]any)
	if longContext["mode"] != "LONG_POSITION" || !longContext["has_position"].(bool) {
		t.Fatalf("unexpected long decision_context: %+v", longContext)
	}
	longEntry := longEvidence["entry_decision"].(map[string]any)
	longPositionDecision := longEvidence["position_decision"].(map[string]any)
	if longEntry["applicable"].(bool) || longEntry["state"] != "NOT_APPLICABLE" {
		t.Fatalf("unexpected long entry_decision: %+v", longEntry)
	}
	if !longPositionDecision["applicable"].(bool) || longPositionDecision["state"] != ActionReduce {
		t.Fatalf("unexpected long position_decision: %+v", longPositionDecision)
	}
	if longPositionDecision["position_rr_source"] != "POSITION_AVG_COST" {
		t.Fatalf("expected position rr source, got %+v", longPositionDecision)
	}
}

func TestBuildSnapshotBreakoutUsesConfigurableRMultipleTarget(t *testing.T) {
	config := DefaultConfig()
	config.BreakoutTargetRR = 2
	a := &Analyzer{config: config}
	sr := &store.SRZoneAnalysis{
		ID: 9, Symbol: "2330", AnalyzedAt: time.Now(), CurrentPrice: 100,
		DecisionSummary: store.RawJSON(`{"action":"Buy"}`),
	}
	zones := []store.SRZone{
		{ID: 1, Role: "SUPPORT", PriceLow: 90, PriceHigh: 92, Status: "PENDING", TradingScore: 80},
		{ID: 2, Role: "RESISTANCE", PriceLow: 95, PriceHigh: 98, Status: "BROKEN", TradingScore: 75},
	}
	flat, err := a.buildSnapshot(&store.Position{Symbol: "2330"}, sr, zones)
	if err != nil {
		t.Fatal(err)
	}
	if flat.Action != ActionEnter || !flat.TakeProfitPrice.Valid || flat.TakeProfitPrice.Float64 != 120 {
		t.Fatalf("expected breakout ENTER with 2R target, got %+v", flat)
	}
	if !flat.RiskRewardRatio.Valid || flat.RiskRewardRatio.Float64 != 2 {
		t.Fatalf("expected breakout RR=2, got %+v", flat.RiskRewardRatio)
	}
	var evidence map[string]any
	if err := json.Unmarshal([]byte(flat.Evidence), &evidence); err != nil {
		t.Fatal(err)
	}
	if evidence["take_profit_source"] != "BREAKOUT_R_MULTIPLE" {
		t.Fatalf("unexpected take-profit evidence: %+v", evidence)
	}

	long, err := a.buildSnapshot(&store.Position{Symbol: "2330", Shares: 500, AvgCost: 80}, sr, zones)
	if err != nil {
		t.Fatal(err)
	}
	if long.Action != ActionAdd || long.TargetShares <= 500 {
		t.Fatalf("expected breakout ADD, got %+v", long)
	}
}

func TestBuildSnapshotUsesNewMarketActionOnlyForFlatPosition(t *testing.T) {
	a := &Analyzer{config: DefaultConfig()}
	sr := &store.SRZoneAnalysis{
		ID: 9, Symbol: "2330", AnalyzedAt: time.Now(), CurrentPrice: 100,
		DecisionSummary: store.RawJSON(`{"market_action":"BUY_SMALL","position_action":"HOLD","action":"BuySmall"}`),
	}
	zones := []store.SRZone{
		{ID: 1, Role: "SUPPORT", PriceLow: 90, PriceHigh: 92, Status: "PENDING", TradingScore: 80},
		{ID: 2, Role: "RESISTANCE", PriceLow: 120, PriceHigh: 122, Status: "PENDING", TradingScore: 70},
	}

	flat, err := a.buildSnapshot(&store.Position{Symbol: "2330"}, sr, zones)
	if err != nil {
		t.Fatal(err)
	}
	if flat.Action != ActionEnterSmall || flat.TargetShares != 500 {
		t.Fatalf("expected new market_action to allow flat small entry, got %+v", flat)
	}
}

func TestBuildSnapshotNewMarketBuyDoesNotDirectlyAddExistingPosition(t *testing.T) {
	a := &Analyzer{config: DefaultConfig()}
	sr := &store.SRZoneAnalysis{
		ID: 9, Symbol: "2330", AnalyzedAt: time.Now(), CurrentPrice: 100,
		DecisionSummary: store.RawJSON(`{"market_action":"BUY","position_action":"HOLD","action":"Buy"}`),
	}
	zones := []store.SRZone{
		{ID: 1, Role: "SUPPORT", PriceLow: 90, PriceHigh: 92, Status: "PENDING", TradingScore: 80},
		{ID: 2, Role: "RESISTANCE", PriceLow: 120, PriceHigh: 122, Status: "PENDING", TradingScore: 70},
	}

	long, err := a.buildSnapshot(&store.Position{Symbol: "2330", Shares: 300, AvgCost: 80}, sr, zones)
	if err != nil {
		t.Fatal(err)
	}
	if long.Action == ActionAdd {
		t.Fatalf("new market_action must not directly add existing position: %+v", long)
	}
	if long.Action != ActionHold || long.TargetShares != 300 {
		t.Fatalf("expected existing position to hold when position_action is HOLD, got %+v", long)
	}
}

func TestBuildSnapshotConditionalHoldEvidence(t *testing.T) {
	a := &Analyzer{config: DefaultConfig()}
	sr := &store.SRZoneAnalysis{
		ID: 9, Symbol: "2330", AnalyzedAt: time.Now(), CurrentPrice: 100,
		DecisionSummary: store.RawJSON(`{
			"market_action":"WATCH",
			"position_action":"HOLD",
			"action":"Hold",
			"position_action_condition":{
				"state":"SUPPORT_RECLAIM_CANDIDATE",
				"invalidation_price":90,
				"recovery_price":92,
				"reason_codes":["PRIMARY_SUPPORT","SUPPORT_RECLAIM_AWAIT_CONFIRMATION"]
			}
		}`),
	}
	zones := []store.SRZone{
		{ID: 1, Role: "SUPPORT", PriceLow: 90, PriceHigh: 92, Status: "PENDING", TradingScore: 80},
		{ID: 2, Role: "RESISTANCE", PriceLow: 120, PriceHigh: 122, Status: "PENDING", TradingScore: 70},
	}

	result, err := a.buildSnapshot(&store.Position{Symbol: "2330", Shares: 300, AvgCost: 95}, sr, zones)
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != ActionHold || result.ActionLabel != "條件式持有" {
		t.Fatalf("expected conditional hold, got %+v", result)
	}
	var evidence map[string]any
	if err := json.Unmarshal([]byte(result.Evidence), &evidence); err != nil {
		t.Fatal(err)
	}
	condition := evidence["position_action_condition"].(map[string]any)
	if condition["invalidation_price"].(float64) != 90 || condition["recovery_price"].(float64) != 92 {
		t.Fatalf("unexpected condition evidence: %+v", condition)
	}
	riskSizing := evidence["risk_sizing"].(map[string]any)
	if riskSizing["risk_budget"].(float64) != 10000 || riskSizing["per_share_risk"].(float64) != 10 || riskSizing["max_shares"].(float64) != 1000 {
		t.Fatalf("unexpected risk sizing evidence: %+v", riskSizing)
	}
	stops := evidence["stops"].(map[string]any)
	if stops["defense_price"].(float64) != 90 || stops["structural_stop"].(float64) != 90 {
		t.Fatalf("unexpected stop evidence: %+v", stops)
	}
	rr := evidence["rr"].(map[string]any)
	if rr["market_rr"].(float64) != 2 || rr["position_rr"].(float64) != 5 {
		t.Fatalf("unexpected rr evidence: %+v", rr)
	}
	positionDecision := evidence["position_decision"].(map[string]any)
	if positionDecision["state"] != "CONDITIONAL_HOLD" || positionDecision["position_rr_source"] != "POSITION_AVG_COST" {
		t.Fatalf("unexpected position decision: %+v", positionDecision)
	}
}

func TestBuildSnapshotPositionDecisionMarksUnavailableRR(t *testing.T) {
	a := &Analyzer{config: DefaultConfig()}
	sr := &store.SRZoneAnalysis{
		ID: 9, Symbol: "2330", AnalyzedAt: time.Now(), CurrentPrice: 100,
		DecisionSummary: store.RawJSON(`{"market_action":"WATCH","position_action":"HOLD","action":"Hold"}`),
	}
	result, err := a.buildSnapshot(&store.Position{Symbol: "2330", Shares: 300, AvgCost: 95}, sr, nil)
	if err != nil {
		t.Fatal(err)
	}
	var evidence map[string]any
	if err := json.Unmarshal([]byte(result.Evidence), &evidence); err != nil {
		t.Fatal(err)
	}
	positionDecision := evidence["position_decision"].(map[string]any)
	if positionDecision["position_rr"] != nil || positionDecision["position_rr_source"] != "UNAVAILABLE" {
		t.Fatalf("expected unavailable position RR, got %+v", positionDecision)
	}
}

func TestBuildSnapshotUsesNewPositionActionForExistingPositionRisk(t *testing.T) {
	a := &Analyzer{config: DefaultConfig()}
	zones := []store.SRZone{
		{ID: 1, Role: "SUPPORT", PriceLow: 90, PriceHigh: 92, Status: "PENDING", TradingScore: 80},
		{ID: 2, Role: "RESISTANCE", PriceLow: 120, PriceHigh: 122, Status: "PENDING", TradingScore: 70},
	}

	reduceSR := &store.SRZoneAnalysis{
		ID: 9, Symbol: "2330", AnalyzedAt: time.Now(), CurrentPrice: 100,
		DecisionSummary: store.RawJSON(`{"market_action":"AVOID","position_action":"REDUCE_ON_BREAKDOWN","action":"Avoid"}`),
	}
	reduced, err := a.buildSnapshot(&store.Position{Symbol: "2330", Shares: 300, AvgCost: 80}, reduceSR, zones)
	if err != nil {
		t.Fatal(err)
	}
	if reduced.Action != ActionReduce || reduced.TargetShares != 150 {
		t.Fatalf("expected REDUCE_ON_BREAKDOWN to reduce existing position, got %+v", reduced)
	}

	exitSR := &store.SRZoneAnalysis{
		ID: 10, Symbol: "2330", AnalyzedAt: time.Now(), CurrentPrice: 100,
		DecisionSummary: store.RawJSON(`{"market_action":"AVOID","position_action":"EXIT","action":"Avoid"}`),
	}
	exited, err := a.buildSnapshot(&store.Position{Symbol: "2330", Shares: 300, AvgCost: 80}, exitSR, zones)
	if err != nil {
		t.Fatal(err)
	}
	if exited.Action != ActionExitStop || exited.TargetShares != 0 {
		t.Fatalf("expected EXIT to close existing position, got %+v", exited)
	}
}

func TestBuildSnapshotNoSupportWaitsOrReduces(t *testing.T) {
	a := &Analyzer{config: DefaultConfig()}
	sr := &store.SRZoneAnalysis{ID: 9, Symbol: "2330", AnalyzedAt: time.Now(), CurrentPrice: 100, DecisionSummary: store.RawJSON(`{"action":"Buy"}`)}
	flat, _ := a.buildSnapshot(&store.Position{Symbol: "2330"}, sr, nil)
	if flat.Action != ActionWait {
		t.Fatalf("expected WAIT, got %+v", flat)
	}
	long, _ := a.buildSnapshot(&store.Position{Symbol: "2330", Shares: 100, AvgCost: 90}, sr, nil)
	if long.Action != ActionReduce || long.TargetShares != 50 {
		t.Fatalf("expected risk reduction, got %+v", long)
	}
}

func TestBuildSnapshotSupportAboveCurrentDoesNotForceExit(t *testing.T) {
	a := &Analyzer{config: DefaultConfig()}
	sr := &store.SRZoneAnalysis{ID: 9, Symbol: "2330", AnalyzedAt: time.Now(), CurrentPrice: 100, DecisionSummary: store.RawJSON(`{"action":"Hold"}`)}
	position := &store.Position{Symbol: "2330", Shares: 100, AvgCost: 90}
	// 最靠近的支撐帶在現價上方（100.5~102），但現價下方有有效支撐（90~92）。
	// 上方支撐不能被當成跌破停損而全數出場。
	zones := []store.SRZone{
		{ID: 1, Role: "SUPPORT", PriceLow: 100.5, PriceHigh: 102, Status: "PENDING", TradingScore: 85},
		{ID: 2, Role: "SUPPORT", PriceLow: 90, PriceHigh: 92, Status: "PENDING", TradingScore: 80},
	}
	result, _ := a.buildSnapshot(position, sr, zones)
	if result.Action == ActionExitStop {
		t.Fatalf("support above current must not force EXIT_STOP: %+v", result)
	}
}

func TestBuildSnapshotOnlyNearestBrokenSupportTriggersExit(t *testing.T) {
	a := &Analyzer{config: DefaultConfig()}
	sr := &store.SRZoneAnalysis{ID: 9, Symbol: "2330", AnalyzedAt: time.Now(), CurrentPrice: 100, DecisionSummary: store.RawJSON(`{"action":"Hold"}`)}
	position := &store.Position{Symbol: "2330", Shares: 100, AvgCost: 90}
	zones := []store.SRZone{
		{ID: 1, Role: "SUPPORT", PriceLow: 90, PriceHigh: 92, Status: "PENDING", TradingScore: 80},
		{ID: 2, Role: "SUPPORT", PriceLow: 60, PriceHigh: 62, Status: "BROKEN", TradingScore: 70},
	}
	result, _ := a.buildSnapshot(position, sr, zones)
	if result.Action == ActionExitStop {
		t.Fatalf("distant broken support must not override nearest valid support: %+v", result)
	}
	zones[0].Status = "BROKEN"
	result, _ = a.buildSnapshot(position, sr, zones)
	if result.Action != ActionExitStop || result.TargetShares != 0 {
		t.Fatalf("nearest broken support must exit: %+v", result)
	}
}
