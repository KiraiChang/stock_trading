package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/database"
)

func TestWatchlistSetWatchedEnforcesLimit(t *testing.T) {
	tmp, err := os.CreateTemp("", "watch-test-*.db")
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

	repo := NewWatchlistRepo(db)
	ctx := context.Background()
	symbols := []string{"AAA", "BBB", "CCC", "DDD"}
	for _, s := range symbols {
		if err := repo.Add(ctx, s, s, ""); err != nil {
			t.Fatalf("add %s failed: %v", s, err)
		}
	}

	// 前 3 檔設監聽應該成功
	for _, s := range symbols[:3] {
		if err := repo.SetWatched(ctx, s, true); err != nil {
			t.Fatalf("SetWatched(%s, true) failed: %v", s, err)
		}
	}

	// 第 4 檔應該被擋下
	if err := repo.SetWatched(ctx, "DDD", true); !errors.Is(err, ErrWatchLimitExceeded) {
		t.Fatalf("expected ErrWatchLimitExceeded for 4th symbol, got %v", err)
	}

	items, err := repo.GetAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	watchedCount := 0
	for _, it := range items {
		if it.Watched {
			watchedCount++
		}
	}
	if watchedCount != 3 {
		t.Fatalf("expected 3 watched symbols, got %d", watchedCount)
	}

	// 取消一檔監聽後，應該可以再設定 DDD
	if err := repo.SetWatched(ctx, "AAA", false); err != nil {
		t.Fatalf("unwatch AAA failed: %v", err)
	}
	if err := repo.SetWatched(ctx, "DDD", true); err != nil {
		t.Fatalf("expected DDD to succeed after freeing a slot, got %v", err)
	}
}

func TestWatchlistAddResolvesStockSymbolMetadata(t *testing.T) {
	tmp, err := os.CreateTemp("", "watch-stock-symbol-test-*.db")
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

	ctx := context.Background()
	stockRepo := NewStockSymbolRepo(db)
	seenAt := time.Date(2026, 7, 22, 6, 30, 0, 0, time.UTC)
	if _, err := stockRepo.UpsertSnapshot(ctx, []StockSymbol{{
		Symbol:       "2330",
		Name:         "台積電",
		ISINCode:     "TW0002330008",
		Market:       "上市",
		SecurityType: "Stocks",
		Industry:     "半導體業",
		ListedDate:   NullTime{NullTime: sql.NullTime{Time: time.Date(1962, 2, 9, 0, 0, 0, 0, time.UTC), Valid: true}},
	}}, seenAt); err != nil {
		t.Fatalf("upsert stock symbols failed: %v", err)
	}

	repo := NewWatchlistRepo(db)
	if err := repo.Add(ctx, "2330", "", ""); err != nil {
		t.Fatalf("add watchlist failed: %v", err)
	}

	items, err := repo.GetAll(ctx)
	if err != nil {
		t.Fatalf("get watchlist failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	item := items[0]
	if item.Name != "台積電" || item.Sector != "半導體業" {
		t.Fatalf("expected metadata to be resolved from stock_symbols, got %+v", item)
	}
	if item.StockSymbol == nil || !item.StockSymbol.Exists || !item.StockSymbol.IsListed {
		t.Fatalf("expected stock symbol status to be joined, got %+v", item.StockSymbol)
	}
}
