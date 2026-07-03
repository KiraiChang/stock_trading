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

func TestCandleBulkInsertBatchesAndUpserts(t *testing.T) {
	tmp, err := os.CreateTemp("", "candle-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	defer os.Remove(tmp.Name())

	db, err := NewDB(config.DatabaseConfig{Driver: "sqlite", DSN: tmp.Name()})
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer db.Close()
	if err := database.RunMigrations(context.Background(), db, "sqlite", zap.NewNop()); err != nil {
		t.Fatalf("migrations failed: %v", err)
	}

	repo := NewCandleRepo(db)
	ctx := context.Background()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := make([]Candle, 0, candleBulkInsertBatchSize+25)
	for i := 0; i < candleBulkInsertBatchSize+25; i++ {
		candles = append(candles, Candle{
			Symbol:    "2330",
			Timeframe: "1d",
			Open:      float64(100 + i),
			High:      float64(101 + i),
			Low:       float64(99 + i),
			Close:     float64(100 + i),
			Volume:    int64(1000 + i),
			Amount:    float64(100000 + i),
			Timestamp: base.AddDate(0, 0, i),
		})
	}

	if err := repo.BulkInsert(ctx, candles); err != nil {
		t.Fatalf("bulk insert failed: %v", err)
	}

	update := candles[10]
	update.Open = 900
	update.High = 910
	update.Low = 890
	update.Close = 905
	update.Volume = 9999
	update.Amount = 8888
	if err := repo.BulkInsert(ctx, []Candle{update}); err != nil {
		t.Fatalf("bulk upsert failed: %v", err)
	}

	rows, err := repo.GetRange(ctx, "2330", "1d", base, base.AddDate(0, 0, len(candles)))
	if err != nil {
		t.Fatalf("get range failed: %v", err)
	}
	if len(rows) != len(candles) {
		t.Fatalf("expected %d candles, got %d", len(candles), len(rows))
	}
	got := rows[10]
	if got.Open != update.Open || got.Close != update.Close || got.Volume != update.Volume || got.Amount != update.Amount {
		t.Fatalf("expected row 10 to be updated, got open=%v close=%v volume=%v amount=%v", got.Open, got.Close, got.Volume, got.Amount)
	}
}
