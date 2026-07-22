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

func TestStockSymbolRepoUpsertSnapshotMarksMissingDelisted(t *testing.T) {
	tmp, err := os.CreateTemp("", "stock-symbol-test-*.db")
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

	repo := NewStockSymbolRepo(db)
	ctx := context.Background()
	day1 := time.Date(2026, 7, 20, 6, 30, 0, 0, time.UTC)
	day2 := day1.Add(24 * time.Hour)

	result, err := repo.UpsertSnapshot(ctx, []StockSymbol{
		stockSymbolForTest("1101", "TCC", "Cement"),
		stockSymbolForTest("2330", "TSMC", "Semiconductor"),
	}, day1)
	if err != nil {
		t.Fatalf("first upsert failed: %v", err)
	}
	if result.Seen != 2 || result.Delisted != 0 {
		t.Fatalf("unexpected first result: %+v", result)
	}

	result, err = repo.UpsertSnapshot(ctx, []StockSymbol{
		stockSymbolForTest("2330", "TSMC", "Semiconductor"),
		stockSymbolForTest("00981A", "ACTIVE ETF", ""),
	}, day2)
	if err != nil {
		t.Fatalf("second upsert failed: %v", err)
	}
	if result.Seen != 2 || result.Delisted != 1 {
		t.Fatalf("unexpected second result: %+v", result)
	}

	oldSymbol, err := repo.Get(ctx, "1101")
	if err != nil {
		t.Fatalf("get 1101 failed: %v", err)
	}
	if oldSymbol.IsListed {
		t.Fatalf("expected 1101 to be marked delisted: %+v", oldSymbol)
	}

	newSymbol, err := repo.Get(ctx, "00981A")
	if err != nil {
		t.Fatalf("get 00981A failed: %v", err)
	}
	if !newSymbol.IsListed || newSymbol.Name != "ACTIVE ETF" {
		t.Fatalf("unexpected new symbol: %+v", newSymbol)
	}

	listed, err := repo.List(ctx, true)
	if err != nil {
		t.Fatalf("list listed failed: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("expected 2 listed symbols, got %d: %+v", len(listed), listed)
	}
}

func TestStockSymbolRepoUpsertSnapshotRejectsEmptySnapshot(t *testing.T) {
	tmp, err := os.CreateTemp("", "stock-symbol-empty-test-*.db")
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

	repo := NewStockSymbolRepo(db)
	_, err = repo.UpsertSnapshot(context.Background(), nil, time.Now())
	if !errors.Is(err, ErrEmptyStockSymbolSnapshot) {
		t.Fatalf("expected ErrEmptyStockSymbolSnapshot, got %v", err)
	}
}

func TestStockSymbolRepoSearch(t *testing.T) {
	tmp, err := os.CreateTemp("", "stock-symbol-search-test-*.db")
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

	repo := NewStockSymbolRepo(db)
	ctx := context.Background()
	seenAt := time.Date(2026, 7, 22, 6, 30, 0, 0, time.UTC)
	if _, err := repo.UpsertSnapshot(ctx, []StockSymbol{
		stockSymbolForTest("2330", "台積電", "半導體業"),
		stockSymbolForTest("2317", "鴻海", "其他電子業"),
	}, seenAt); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}

	rows, err := repo.Search(ctx, StockSymbolSearchOptions{Query: "台積", OnlyListed: true, Limit: 10})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(rows) != 1 || rows[0].Symbol != "2330" {
		t.Fatalf("unexpected search result: %+v", rows)
	}

	if _, err := repo.UpsertSnapshot(ctx, []StockSymbol{
		stockSymbolForTest("2317", "鴻海", "其他電子業"),
	}, seenAt.Add(24*time.Hour)); err != nil {
		t.Fatalf("second upsert failed: %v", err)
	}
	rows, err = repo.Search(ctx, StockSymbolSearchOptions{Query: "2330", OnlyListed: true, Limit: 10})
	if err != nil {
		t.Fatalf("search listed failed: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected delisted 2330 to be hidden when OnlyListed=true, got %+v", rows)
	}
	rows, err = repo.Search(ctx, StockSymbolSearchOptions{Query: "2330", OnlyListed: false, Limit: 10})
	if err != nil {
		t.Fatalf("search all failed: %v", err)
	}
	if len(rows) != 1 || rows[0].IsListed {
		t.Fatalf("expected delisted 2330 when OnlyListed=false, got %+v", rows)
	}
}

func stockSymbolForTest(symbol, name, industry string) StockSymbol {
	return StockSymbol{
		Symbol:       symbol,
		Name:         name,
		ISINCode:     "TW000" + symbol,
		Market:       "TWSE LISTED",
		SecurityType: "Stocks",
		Industry:     industry,
		CFICode:      "ESVUFR",
		ListedDate:   NullTime{NullTime: sql.NullTime{Time: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true}},
	}
}
