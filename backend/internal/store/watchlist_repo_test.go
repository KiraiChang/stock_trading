package store

import (
	"context"
	"errors"
	"os"
	"testing"

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
