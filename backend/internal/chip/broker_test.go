package chip

import (
	"testing"

	"github.com/trading/backend/internal/store"
)

func TestCalcTopNNetBuy(t *testing.T) {
	// 需已依 net_buy DESC 排序，比照 store.BrokerTradeRepo.GetByDate 的回傳慣例
	trades := []store.BrokerTrade{
		{BrokerName: "A", NetBuy: 6000},
		{BrokerName: "B", NetBuy: 4000},
		{BrokerName: "C", NetBuy: 1000},
		{BrokerName: "D", NetBuy: -2000},
		{BrokerName: "E", NetBuy: -5000},
	}
	topBuy, topSell := CalcTopNNetBuy(trades, 2)
	if topBuy != 10000 {
		t.Errorf("expected topBuy=10000 (6000+4000), got %d", topBuy)
	}
	if topSell != -7000 {
		t.Errorf("expected topSell=-7000 (-2000-5000), got %d", topSell)
	}
}

func TestCalcTopNNetBuy_AllBuySideHasZeroTopSell(t *testing.T) {
	trades := []store.BrokerTrade{
		{NetBuy: 3000},
		{NetBuy: 1000},
	}
	topBuy, topSell := CalcTopNNetBuy(trades, 10)
	if topBuy != 4000 {
		t.Errorf("expected topBuy=4000, got %d", topBuy)
	}
	if topSell != 0 {
		t.Errorf("expected topSell=0 when no seller present, got %d", topSell)
	}
}

func TestCalcConcentration(t *testing.T) {
	cases := []struct {
		name        string
		netBuy      int64
		dailyVolume int64
		want        float64
	}{
		{"normal", 5000, 100000, 0.05},
		{"negative net buy uses absolute value", -5000, 100000, 0.05},
		{"zero volume avoids divide by zero", 5000, 0, 0},
		{"negative volume avoids divide by zero", 5000, -100, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := CalcConcentration(c.netBuy, c.dailyVolume)
			if got != c.want {
				t.Errorf("CalcConcentration(%d, %d) = %v, want %v", c.netBuy, c.dailyVolume, got, c.want)
			}
		})
	}
}

func TestCalcBrokerScore_NoDataFallsBackToNeutral(t *testing.T) {
	score, reasons := CalcBrokerScore(nil, 100000)
	if score != 0 {
		t.Errorf("expected score=0 with no broker data, got %v", score)
	}
	if len(reasons) == 0 {
		t.Error("expected a fallback reason")
	}
}

func TestCalcBrokerScore_BuyConcentrationIsPositive(t *testing.T) {
	trades := []store.BrokerTrade{
		{NetBuy: 8000},
		{NetBuy: 6000},
	}
	score, reasons := CalcBrokerScore(trades, 100000)
	if score <= 0 {
		t.Errorf("expected positive score when buy concentration dominates, got %v", score)
	}
	if len(reasons) == 0 {
		t.Error("expected a reason")
	}
}

func TestCalcBrokerScore_SellConcentrationIsNegative(t *testing.T) {
	trades := []store.BrokerTrade{
		{NetBuy: -8000},
		{NetBuy: -6000},
	}
	score, _ := CalcBrokerScore(trades, 100000)
	if score >= 0 {
		t.Errorf("expected negative score when sell concentration dominates, got %v", score)
	}
}
