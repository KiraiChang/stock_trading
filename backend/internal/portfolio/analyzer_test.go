package portfolio

import (
	"database/sql"
	"encoding/json"
	"testing"
	"time"

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
