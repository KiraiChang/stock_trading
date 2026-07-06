package store

import (
	"context"
	"os"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/database"
)

func newTestInstitutionalTradeRepo(t *testing.T) InstitutionalTradeRepo {
	t.Helper()
	tmp, err := os.CreateTemp("", "institutional-trade-test-*.db")
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
	return NewInstitutionalTradeRepo(db)
}

func TestInstitutionalTradeBulkUpsertBatchesAndUpdates(t *testing.T) {
	repo := newTestInstitutionalTradeRepo(t)
	ctx := context.Background()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	trades := make([]InstitutionalTrade, 0, institutionalTradeBulkBatchSize+10)
	for i := 0; i < institutionalTradeBulkBatchSize+10; i++ {
		trades = append(trades, InstitutionalTrade{
			Symbol:                "2330",
			TradeDate:             base.AddDate(0, 0, i),
			ForeignNetBuy:         int64(1000 + i),
			InvestmentTrustNetBuy: int64(500 + i),
			DealerNetBuy:          int64(-100 + i),
			TotalNetBuy:           int64(1400 + 2*i),
		})
	}

	if err := repo.BulkUpsert(ctx, trades); err != nil {
		t.Fatalf("bulk upsert failed: %v", err)
	}

	update := trades[5]
	update.ForeignNetBuy = 99999
	if err := repo.BulkUpsert(ctx, []InstitutionalTrade{update}); err != nil {
		t.Fatalf("bulk upsert (update) failed: %v", err)
	}

	rows, err := repo.GetRange(ctx, "2330", base, base.AddDate(0, 0, len(trades)))
	if err != nil {
		t.Fatalf("get range failed: %v", err)
	}
	if len(rows) != len(trades) {
		t.Fatalf("expected %d rows (no duplicates from re-upsert), got %d", len(trades), len(rows))
	}
	if rows[5].ForeignNetBuy != 99999 {
		t.Fatalf("expected row 5 foreign_net_buy updated to 99999, got %d", rows[5].ForeignNetBuy)
	}
}

func TestInstitutionalTradeGetLatestNReturnsAscending(t *testing.T) {
	repo := newTestInstitutionalTradeRepo(t)
	ctx := context.Background()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	trades := make([]InstitutionalTrade, 0, 10)
	for i := 0; i < 10; i++ {
		trades = append(trades, InstitutionalTrade{
			Symbol:      "2317",
			TradeDate:   base.AddDate(0, 0, i),
			TotalNetBuy: int64(i),
		})
	}
	if err := repo.BulkUpsert(ctx, trades); err != nil {
		t.Fatalf("bulk upsert failed: %v", err)
	}

	rows, err := repo.GetLatestN(ctx, "2317", 5)
	if err != nil {
		t.Fatalf("get latest n failed: %v", err)
	}
	if len(rows) != 5 {
		t.Fatalf("expected 5 rows, got %d", len(rows))
	}
	for i := 1; i < len(rows); i++ {
		if !rows[i].TradeDate.After(rows[i-1].TradeDate) {
			t.Fatalf("expected ascending trade_date order, got %v before %v", rows[i-1].TradeDate, rows[i].TradeDate)
		}
	}
	if rows[len(rows)-1].TotalNetBuy != 9 {
		t.Fatalf("expected last row to be the most recent (TotalNetBuy=9), got %d", rows[len(rows)-1].TotalNetBuy)
	}
}
