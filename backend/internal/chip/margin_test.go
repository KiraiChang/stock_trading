package chip

import (
	"database/sql"
	"testing"

	"github.com/trading/backend/internal/store"
)

func TestCalcMarginScore_MarginIncreaseWithPriceDropIsBearish(t *testing.T) {
	trade := store.MarginTrade{MarginChange: 1000}
	score, reasons := CalcMarginScore(trade, -2.5)
	if score >= 0 {
		t.Errorf("expected negative score (融資增加+下跌偏弱), got %v", score)
	}
	if len(reasons) == 0 {
		t.Error("expected a reason")
	}
}

func TestCalcMarginScore_MarginDecreaseWithPriceRiseIsBullish(t *testing.T) {
	trade := store.MarginTrade{MarginChange: -1000}
	score, _ := CalcMarginScore(trade, 2.5)
	if score <= 0 {
		t.Errorf("expected positive score (融資減少+上漲偏強), got %v", score)
	}
}

func TestCalcMarginScore_ShortIncreaseWithPriceRiseIsBullish(t *testing.T) {
	trade := store.MarginTrade{ShortChange: 500}
	score, _ := CalcMarginScore(trade, 1.5)
	if score <= 0 {
		t.Errorf("expected positive score (融券增加+價格突破可能軋空), got %v", score)
	}
}

func TestCalcMarginScore_NoTriggeredConditionIsNeutral(t *testing.T) {
	trade := store.MarginTrade{}
	score, reasons := CalcMarginScore(trade, 0)
	if score != 0 {
		t.Errorf("expected score=0 when no rule triggers, got %v", score)
	}
	if len(reasons) != 0 {
		t.Errorf("expected no reasons, got %v", reasons)
	}
}

func TestIsMarginRisk(t *testing.T) {
	cases := []struct {
		name      string
		usageRate store.NullFloat64
		threshold float64
		want      bool
	}{
		{"below threshold", store.NullFloat64{NullFloat64: sql.NullFloat64{Float64: 0.5, Valid: true}}, 0.8, false},
		{"at threshold", store.NullFloat64{NullFloat64: sql.NullFloat64{Float64: 0.8, Valid: true}}, 0.8, true},
		{"above threshold", store.NullFloat64{NullFloat64: sql.NullFloat64{Float64: 0.95, Valid: true}}, 0.8, true},
		{"unavailable data is not risky", store.NullFloat64{}, 0.8, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := IsMarginRisk(c.usageRate, c.threshold)
			if got != c.want {
				t.Errorf("IsMarginRisk(%+v, %v) = %v, want %v", c.usageRate, c.threshold, got, c.want)
			}
		})
	}
}

func TestCalcMarginScore_HighUsageRateAddsRiskReason(t *testing.T) {
	trade := store.MarginTrade{MarginUsageRate: store.NullFloat64{NullFloat64: sql.NullFloat64{Float64: 0.9, Valid: true}}}
	_, reasons := CalcMarginScore(trade, 0)
	found := false
	for _, r := range reasons {
		if r == "融資使用率過高，風險升高" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected risk reason in %v", reasons)
	}
}
