package analysis

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestScoreZonesParsesResponseAndMapsToStore(t *testing.T) {
	bounce := 0.72
	brk := 0.18

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sr-zones" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var body struct {
			Symbol    string `json:"symbol"`
			Timeframe string `json:"timeframe"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body failed: %v", err)
		}
		if body.Symbol != "2330" || body.Timeframe != "1d" {
			t.Fatalf("unexpected request body: %+v", body)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ZoneScoreResult{
			Symbol:       "2330",
			Timeframe:    "1d",
			AnalyzedAt:   "2026-07-01T13:30:00+08:00",
			CurrentPrice: 600.0,
			Zones: []ZoneScore{
				{
					PriceLow: 580.0, PriceHigh: 585.0, Method: "atr", Role: "SUPPORT",
					SupportScore: 0.8, ResistanceScore: 0.1,
					BounceProbability: &bounce, BreakProbability: &brk,
					FeaturesAsSupport: &ZoneFeatures{
						TouchCount: 4, RejectionCount: 3, BreakoutCount: 0,
						AvgReturnAfterTouch: 0.02, RelativeVolume: 1.4, Volatility: 0.015, TrendStrength: 0.03,
					},
				},
				{
					PriceLow: 610.0, PriceHigh: 615.0, Method: "volume_profile", Role: "AT_ZONE",
					SupportScore: 0.2, ResistanceScore: 0.3,
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	result, err := client.ScoreZones(context.Background(), "2330", "1d")
	if err != nil {
		t.Fatalf("ScoreZones failed: %v", err)
	}
	if result.Symbol != "2330" || len(result.Zones) != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}

	a, zones, err := result.ToStore()
	if err != nil {
		t.Fatalf("ToStore failed: %v", err)
	}
	if a.Symbol != "2330" || a.CurrentPrice != 600.0 {
		t.Fatalf("unexpected analysis: %+v", a)
	}
	if len(zones) != 2 {
		t.Fatalf("expected 2 zones, got %d", len(zones))
	}

	first := zones[0]
	if first.Method != "atr" || first.Role != "SUPPORT" || first.TouchCount != 4 {
		t.Fatalf("unexpected first zone: %+v", first)
	}
	if !first.BounceProbability.Valid || first.BounceProbability.Float64 != bounce {
		t.Fatalf("expected bounce probability %.2f, got %+v", bounce, first.BounceProbability)
	}

	second := zones[1]
	if second.Role != "AT_ZONE" || second.BounceProbability.Valid {
		t.Fatalf("expected AT_ZONE zone with no bounce probability, got %+v", second)
	}
}

func TestScoreZonesReturnsErrorWhenBaseURLNotConfigured(t *testing.T) {
	client := NewClient("")
	if _, err := client.ScoreZones(context.Background(), "2330", "1d"); err == nil {
		t.Fatal("expected error when baseURL is not configured")
	}
}
