package chip

import (
	"testing"

	"github.com/trading/backend/internal/store"
)

func TestCalcConsecutiveNetBuyDays(t *testing.T) {
	cases := []struct {
		name string
		in   []int64
		want int
	}{
		{"empty", nil, 0},
		{"single buy day", []int64{500}, 1},
		{"single sell day", []int64{-500}, -1},
		{"last day flat", []int64{100, 200, 0}, 0},
		{"three consecutive buy days", []int64{-100, 200, 300, 400}, 3},
		{"two consecutive sell days", []int64{500, -100, -200}, -2},
		{"buy streak broken by a sell day", []int64{100, -50, 200, 300}, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := CalcConsecutiveNetBuyDays(c.in)
			if got != c.want {
				t.Errorf("CalcConsecutiveNetBuyDays(%v) = %d, want %d", c.in, got, c.want)
			}
		})
	}
}

func TestCalcInstitutionalScore_NoDataIsNeutral(t *testing.T) {
	score, reasons := CalcInstitutionalScore(nil, 1_000_000, 2_000_000)
	if score != 0 {
		t.Errorf("expected score=0 with no history, got %v", score)
	}
	if len(reasons) == 0 {
		t.Error("expected a reason explaining missing data")
	}
}

func TestCalcInstitutionalScore_ConsecutiveBuyingIsPositive(t *testing.T) {
	hist := []store.InstitutionalTrade{
		{ForeignNetBuy: 1000, InvestmentTrustNetBuy: 500, DealerNetBuy: 100, TotalNetBuy: 1600},
		{ForeignNetBuy: 1200, InvestmentTrustNetBuy: 600, DealerNetBuy: 100, TotalNetBuy: 1900},
		{ForeignNetBuy: 1500, InvestmentTrustNetBuy: 700, DealerNetBuy: 100, TotalNetBuy: 2300},
	}
	score, reasons := CalcInstitutionalScore(hist, 1_000_000, 2_000_000)
	if score <= 0 {
		t.Errorf("expected positive score for consecutive buying, got %v", score)
	}
	if len(reasons) == 0 {
		t.Error("expected reasons describing the buying streak")
	}
}

func TestCalcInstitutionalScore_ConsecutiveSellingIsNegative(t *testing.T) {
	hist := []store.InstitutionalTrade{
		{ForeignNetBuy: -1000, InvestmentTrustNetBuy: -500, DealerNetBuy: -100, TotalNetBuy: -1600},
		{ForeignNetBuy: -1200, InvestmentTrustNetBuy: -600, DealerNetBuy: -100, TotalNetBuy: -1900},
		{ForeignNetBuy: -1500, InvestmentTrustNetBuy: -700, DealerNetBuy: -100, TotalNetBuy: -2300},
	}
	score, _ := CalcInstitutionalScore(hist, 1_000_000, 2_000_000)
	if score >= 0 {
		t.Errorf("expected negative score for consecutive selling, got %v", score)
	}
}

func TestCalcInstitutionalScore_HeavierBuyingScoresHigher(t *testing.T) {
	light := []store.InstitutionalTrade{
		{ForeignNetBuy: 100, TotalNetBuy: 100},
		{ForeignNetBuy: 100, TotalNetBuy: 100},
	}
	heavy := []store.InstitutionalTrade{
		{ForeignNetBuy: 100_000, TotalNetBuy: 100_000},
		{ForeignNetBuy: 100_000, TotalNetBuy: 100_000},
	}
	lightScore, _ := CalcInstitutionalScore(light, 1_000_000, 2_000_000)
	heavyScore, _ := CalcInstitutionalScore(heavy, 1_000_000, 2_000_000)
	if heavyScore <= lightScore {
		t.Errorf("expected heavier net buying to score higher: light=%v heavy=%v", lightScore, heavyScore)
	}
}

func TestCalcInstitutionalScore_ZeroAvgVolumeDoesNotPanic(t *testing.T) {
	hist := []store.InstitutionalTrade{{ForeignNetBuy: 1000, TotalNetBuy: 1000}}
	score, _ := CalcInstitutionalScore(hist, 0, 0)
	if score < -100 || score > 100 {
		t.Errorf("score out of expected range: %v", score)
	}
}
