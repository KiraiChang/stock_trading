package portfolio

import (
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
	long, err := a.buildSnapshot(&store.Position{Symbol: "2330", Shares: 700, AvgCost: 80, Version: 3}, sr, zones)
	if err != nil {
		t.Fatal(err)
	}
	if long.PositionState != StateLong || long.Action != ActionReduce || long.TargetShares != 500 || long.AdjustmentShares != -200 {
		t.Fatalf("unexpected LONG analysis: %+v", long)
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
