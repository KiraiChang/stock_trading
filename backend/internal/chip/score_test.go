package chip

import (
	"database/sql"
	"testing"

	"github.com/trading/backend/internal/store"
)

func TestCalcTotalScore_WeightsSumToOne(t *testing.T) {
	// 全部子分數都是 100 時，total 應該也是 100（0.35+0.20+0.30+0.15=1.0）
	got := CalcTotalScore(100, 100, 100, 100)
	if got != 100 {
		t.Errorf("expected weighted sum of 100 when all sub-scores are 100, got %v", got)
	}
}

func TestCalcTotalScore_WeightedContribution(t *testing.T) {
	got := CalcTotalScore(100, 0, 0, 0)
	if got != 35 {
		t.Errorf("expected institutional weight 0.35 * 100 = 35, got %v", got)
	}
}

func TestClassifySignal(t *testing.T) {
	cases := []struct {
		name       string
		totalScore float64
		marginRisk bool
		want       Signal
	}{
		{"strong positive is bullish", 50, false, Bullish},
		{"at bullish threshold", signalThreshold, false, Bullish},
		{"just below bullish threshold is neutral", signalThreshold - 0.01, false, Neutral},
		{"strong negative is bearish", -50, false, Bearish},
		{"at bearish threshold", -signalThreshold, false, Bearish},
		{"zero is neutral", 0, false, Neutral},
		{"margin risk overrides bullish score", 50, true, Risk},
		{"margin risk overrides neutral score", 0, true, Risk},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ClassifySignal(c.totalScore, c.marginRisk)
			if got != c.want {
				t.Errorf("ClassifySignal(%v, %v) = %v, want %v", c.totalScore, c.marginRisk, got, c.want)
			}
		})
	}
}

func TestCalculate_NoDataIsNeutralWithReasons(t *testing.T) {
	result := Calculate(ChipScoreInput{Symbol: "2330", DailyVolume: 0})
	if result.Signal != Neutral {
		t.Errorf("expected NEUTRAL signal with no data, got %v", result.Signal)
	}
	if len(result.Reasons) == 0 {
		t.Error("expected fallback reasons explaining missing data")
	}
}

func TestCalculate_MarginRiskOverridesBullishInstitutionalTrend(t *testing.T) {
	input := ChipScoreInput{
		Symbol: "2330",
		InstitutionalHistory: []store.InstitutionalTrade{
			{ForeignNetBuy: 100_000, InvestmentTrustNetBuy: 50_000, TotalNetBuy: 150_000},
			{ForeignNetBuy: 100_000, InvestmentTrustNetBuy: 50_000, TotalNetBuy: 150_000},
			{ForeignNetBuy: 100_000, InvestmentTrustNetBuy: 50_000, TotalNetBuy: 150_000},
		},
		MarginHistory: []store.MarginTrade{
			{MarginUsageRate: store.NullFloat64{NullFloat64: sql.NullFloat64{Float64: 0.95, Valid: true}}},
		},
		AvgVolume20: 1_000_000,
		DailyVolume: 2_000_000,
	}
	result := Calculate(input)
	if result.Signal != Risk {
		t.Errorf("expected RISK signal when margin usage rate is dangerously high, got %v (total=%v)", result.Signal, result.TotalScore)
	}
}

func TestCalculate_BrokerFallbackDoesNotBlockOtherScores(t *testing.T) {
	input := ChipScoreInput{
		Symbol: "2330",
		InstitutionalHistory: []store.InstitutionalTrade{
			{ForeignNetBuy: 100_000, TotalNetBuy: 100_000},
		},
		BrokerTrades: nil, // 模擬 FinMind 不支援分點資料
		AvgVolume20:  1_000_000,
		DailyVolume:  2_000_000,
	}
	result := Calculate(input)
	if result.BrokerScore != 0 {
		t.Errorf("expected broker_score=0 fallback, got %v", result.BrokerScore)
	}
	if result.InstitutionalScore == 0 {
		t.Error("expected institutional_score to still be computed despite missing broker data")
	}
}
