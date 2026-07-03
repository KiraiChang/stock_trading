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
			TradingScoreBreakdown: RawJSON(`{"expected_value":30,"risk_reward":15,"trend":10,"volume":15,"confidence":8.5}`),
			TradingRecommendation: "BUY",
			OverlapGroup:          NullInt64{sql.NullInt64{Int64: 0, Valid: true}},
			ConfluenceCount:       2,
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
			TradingScoreBreakdown: RawJSON(`{"expected_value":20,"risk_reward":10,"trend":7.5,"volume":7.5,"confidence":4}`),
			TradingRecommendation: "NEUTRAL",
			ConfluenceCount:       1, // 沒有 OverlapGroup（獨立 zone，未跟其他方法重疊）
		},
	}
}

func TestSRZoneRepoCreateGetRoundTrip(t *testing.T) {
	repo := newTestSRZoneRepo(t)
	ctx := context.Background()

	id, err := repo.Create(ctx, testAnalysis(), testZones())
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

	zones, err := repo.GetZones(ctx, id)
	if err != nil {
		t.Fatalf("GetZones failed: %v", err)
	}
	if len(zones) != 2 {
		t.Fatalf("expected 2 zones, got %d", len(zones))
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

func TestSRZoneRepoListFiltersBySymbol(t *testing.T) {
	repo := newTestSRZoneRepo(t)
	ctx := context.Background()

	a1 := testAnalysis()
	a1.Symbol = "2330"
	if _, err := repo.Create(ctx, a1, testZones()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	a2 := testAnalysis()
	a2.Symbol = "2454"
	if _, err := repo.Create(ctx, a2, testZones()); err != nil {
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

	id, err := repo.Create(ctx, testAnalysis(), testZones())
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
	if err := repo.UpdateZoneStatus(ctx, target.ID, "BROKEN", &brokenAt, &brokenPrice); err != nil {
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
}

func TestSRZoneRepoDeleteCascadesZones(t *testing.T) {
	repo := newTestSRZoneRepo(t)
	ctx := context.Background()

	id, err := repo.Create(ctx, testAnalysis(), testZones())
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
}
