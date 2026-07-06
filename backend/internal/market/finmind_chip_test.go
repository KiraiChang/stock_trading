package market

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/trading/backend/internal/config"
)

func newTestFinMindServer(t *testing.T, body string) *FinMindClient {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	return NewFinMindClient(config.FinMindConfig{
		BaseURL:   server.URL,
		RateLimit: 1000,
	})
}

// institutionalFixture 是真實 TaiwanStockInstitutionalInvestorsBuySell API
// 回應的格式（欄位命名與單位已用真實 API 呼叫驗證，見
// docs/chip-analysis-design.md 開發紀錄），涵蓋兩個交易日、五種身份別。
const institutionalFixture = `{"msg":"success","status":200,"data":[
	{"date":"2024-01-02","stock_id":"2330","buy":0,"name":"Foreign_Dealer_Self","sell":0},
	{"date":"2024-01-02","stock_id":"2330","buy":80000,"name":"Dealer_self","sell":641052},
	{"date":"2024-01-02","stock_id":"2330","buy":25585,"name":"Dealer_Hedging","sell":472504},
	{"date":"2024-01-02","stock_id":"2330","buy":19034488,"name":"Foreign_Investor","sell":11202763},
	{"date":"2024-01-02","stock_id":"2330","buy":869000,"name":"Investment_Trust","sell":109685},
	{"date":"2024-01-03","stock_id":"2330","buy":0,"name":"Foreign_Dealer_Self","sell":0},
	{"date":"2024-01-03","stock_id":"2330","buy":335000,"name":"Dealer_self","sell":600000},
	{"date":"2024-01-03","stock_id":"2330","buy":217021,"name":"Dealer_Hedging","sell":201741},
	{"date":"2024-01-03","stock_id":"2330","buy":13430460,"name":"Foreign_Investor","sell":21022009},
	{"date":"2024-01-03","stock_id":"2330","buy":676140,"name":"Investment_Trust","sell":84010}
]}`

func TestFetchInstitutionalTrades_AggregatesByDateAndIdentity(t *testing.T) {
	client := newTestFinMindServer(t, institutionalFixture)
	start := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)

	trades, err := client.FetchInstitutionalTrades(context.Background(), "2330", start, end)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	if len(trades) != 2 {
		t.Fatalf("expected 2 days, got %d", len(trades))
	}

	day1 := trades[0]
	wantForeign := int64(0 - 0 + 19034488 - 11202763) // Foreign_Dealer_Self + Foreign_Investor net
	wantTrust := int64(869000 - 109685)
	wantDealer := int64(80000 - 641052 + 25585 - 472504)
	if day1.ForeignNetBuy != wantForeign {
		t.Errorf("day1 ForeignNetBuy = %d, want %d", day1.ForeignNetBuy, wantForeign)
	}
	if day1.InvestmentTrustNetBuy != wantTrust {
		t.Errorf("day1 InvestmentTrustNetBuy = %d, want %d", day1.InvestmentTrustNetBuy, wantTrust)
	}
	if day1.DealerNetBuy != wantDealer {
		t.Errorf("day1 DealerNetBuy = %d, want %d", day1.DealerNetBuy, wantDealer)
	}
	wantTotal := wantForeign + wantTrust + wantDealer
	if day1.TotalNetBuy != wantTotal {
		t.Errorf("day1 TotalNetBuy = %d, want %d (sum of the three groups)", day1.TotalNetBuy, wantTotal)
	}
}

// marginFixture 是真實 TaiwanStockMarginPurchaseShortSale API 回應的格式，
// 餘額/限額欄位單位為「張」（已用真實 API 呼叫驗證：MarginPurchaseLimit
// 換算後約等於流通股數的 25% 法定融資限額比例）。
const marginFixture = `{"msg":"success","status":200,"data":[
	{"date":"2024-01-02","stock_id":"2330","MarginPurchaseBuy":310,"MarginPurchaseCashRepayment":10,"MarginPurchaseLimit":6483017,"MarginPurchaseSell":513,"MarginPurchaseTodayBalance":12844,"MarginPurchaseYesterdayBalance":13057,"Note":" ","OffsetLoanAndShort":1,"ShortSaleBuy":2,"ShortSaleCashRepayment":0,"ShortSaleLimit":6483017,"ShortSaleSell":21,"ShortSaleTodayBalance":208,"ShortSaleYesterdayBalance":189}
]}`

func TestFetchMarginTrades_ConvertsLotsToSharesAndComputesUsageRate(t *testing.T) {
	client := newTestFinMindServer(t, marginFixture)
	start := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)

	trades, err := client.FetchMarginTrades(context.Background(), "2330", start, end)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	if len(trades) != 1 {
		t.Fatalf("expected 1 row, got %d", len(trades))
	}

	got := trades[0]
	if got.MarginBalance != 12844*marginLotToShares {
		t.Errorf("MarginBalance = %d, want %d (12844 lots * 1000)", got.MarginBalance, 12844*marginLotToShares)
	}
	if got.MarginChange != (12844-13057)*marginLotToShares {
		t.Errorf("MarginChange = %d, want %d", got.MarginChange, (12844-13057)*marginLotToShares)
	}
	if got.ShortBalance != 208*marginLotToShares {
		t.Errorf("ShortBalance = %d, want %d", got.ShortBalance, 208*marginLotToShares)
	}
	if got.ShortChange != (208-189)*marginLotToShares {
		t.Errorf("ShortChange = %d, want %d", got.ShortChange, (208-189)*marginLotToShares)
	}
	if got.MarginUsageRate == nil {
		t.Fatal("expected MarginUsageRate to be computed (limit > 0)")
	}
	wantRate := 12844.0 / 6483017.0
	if *got.MarginUsageRate != wantRate {
		t.Errorf("MarginUsageRate = %v, want %v", *got.MarginUsageRate, wantRate)
	}
}

func TestFetchMarginTrades_ZeroLimitYieldsNilUsageRate(t *testing.T) {
	body := `{"msg":"success","status":200,"data":[
		{"date":"2024-01-02","stock_id":"1101","MarginPurchaseLimit":0,"MarginPurchaseTodayBalance":0,"MarginPurchaseYesterdayBalance":0,"ShortSaleLimit":0,"ShortSaleTodayBalance":0,"ShortSaleYesterdayBalance":0}
	]}`
	client := newTestFinMindServer(t, body)
	d := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)

	trades, err := client.FetchMarginTrades(context.Background(), "1101", d, d)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	if trades[0].MarginUsageRate != nil || trades[0].ShortUsageRate != nil {
		t.Errorf("expected nil usage rates when limit=0, got margin=%v short=%v", trades[0].MarginUsageRate, trades[0].ShortUsageRate)
	}
}

func TestFetchBrokerTrades_ReturnsUnsupportedError(t *testing.T) {
	client := NewFinMindClient(config.FinMindConfig{BaseURL: "http://unused", RateLimit: 1000})
	_, err := client.FetchBrokerTrades(context.Background(), "2330", time.Now())
	if err != ErrBrokerDataUnsupported {
		t.Errorf("expected ErrBrokerDataUnsupported, got %v", err)
	}
}
