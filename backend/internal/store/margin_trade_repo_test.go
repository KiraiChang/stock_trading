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

func newTestMarginTradeRepo(t *testing.T) MarginTradeRepo {
	t.Helper()
	tmp, err := os.CreateTemp("", "margin-trade-test-*.db")
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
	return NewMarginTradeRepo(db)
}

func TestMarginTradeBulkUpsertBatchesAndUpdates(t *testing.T) {
	repo := newTestMarginTradeRepo(t)
	ctx := context.Background()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	trades := make([]MarginTrade, 0, marginTradeBulkBatchSize+10)
	for i := 0; i < marginTradeBulkBatchSize+10; i++ {
		trades = append(trades, MarginTrade{
			Symbol:        "2330",
			TradeDate:     base.AddDate(0, 0, i),
			MarginBalance: int64(10000 + i),
			MarginChange:  int64(i),
			ShortBalance:  int64(2000 + i),
			ShortChange:   int64(-i),
			// MarginUsageRate/ShortUsageRate 留空（NullFloat64 zero value）驗證
			// provider 未提供額度上限時可正確寫入/讀出 NULL。
		})
	}

	if err := repo.BulkUpsert(ctx, trades); err != nil {
		t.Fatalf("bulk upsert failed: %v", err)
	}

	update := trades[5]
	update.MarginBalance = 88888
	update.MarginUsageRate = NullFloat64{sql.NullFloat64{Float64: 0.55, Valid: true}}
	if err := repo.BulkUpsert(ctx, []MarginTrade{update}); err != nil {
		t.Fatalf("bulk upsert (update) failed: %v", err)
	}

	rows, err := repo.GetRange(ctx, "2330", base, base.AddDate(0, 0, len(trades)))
	if err != nil {
		t.Fatalf("get range failed: %v", err)
	}
	if len(rows) != len(trades) {
		t.Fatalf("expected %d rows (no duplicates from re-upsert), got %d", len(trades), len(rows))
	}
	if rows[5].MarginBalance != 88888 {
		t.Fatalf("expected row 5 margin_balance updated to 88888, got %d", rows[5].MarginBalance)
	}
	if !rows[5].MarginUsageRate.Valid || rows[5].MarginUsageRate.Float64 != 0.55 {
		t.Fatalf("expected row 5 margin_usage_rate=0.55, got %+v", rows[5].MarginUsageRate)
	}
	if rows[0].MarginUsageRate.Valid {
		t.Fatalf("expected row 0 margin_usage_rate to remain NULL, got %+v", rows[0].MarginUsageRate)
	}
}
