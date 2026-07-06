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

func newTestChipScoreRepo(t *testing.T) ChipScoreRepo {
	t.Helper()
	tmp, err := os.CreateTemp("", "chip-score-test-*.db")
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
	return NewChipScoreRepo(db)
}

func TestChipScoreUpsertIdempotentAndGetLatest(t *testing.T) {
	repo := newTestChipScoreRepo(t)
	ctx := context.Background()
	base := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	for i := 0; i < 5; i++ {
		s := ChipScore{
			Symbol:     "2330",
			TradeDate:  base.AddDate(0, 0, i),
			TotalScore: float64(i * 10),
			Signal:     "NEUTRAL",
			Reason:     RawJSON(`["初始理由"]`),
		}
		if err := repo.Upsert(ctx, &s); err != nil {
			t.Fatalf("upsert day %d failed: %v", i, err)
		}
	}

	// 重跑同一天（upsert 冪等）：分數與 reason 應被更新，不新增一列
	rerun := ChipScore{
		Symbol:     "2330",
		TradeDate:  base.AddDate(0, 0, 2),
		TotalScore: 88.5,
		Signal:     "BULLISH",
		Reason:     RawJSON(`["外資連續買超4日"]`),
	}
	if err := repo.Upsert(ctx, &rerun); err != nil {
		t.Fatalf("rerun upsert failed: %v", err)
	}

	rows, err := repo.GetRange(ctx, "2330", base, base.AddDate(0, 0, 5))
	if err != nil {
		t.Fatalf("get range failed: %v", err)
	}
	if len(rows) != 5 {
		t.Fatalf("expected 5 rows (rerun should upsert, not duplicate), got %d", len(rows))
	}
	if rows[2].TotalScore != 88.5 || rows[2].Signal != "BULLISH" {
		t.Fatalf("expected day 2 updated to total_score=88.5 signal=BULLISH, got %+v", rows[2])
	}
	if string(rows[2].Reason) != `["外資連續買超4日"]` {
		t.Fatalf("expected reason updated, got %s", rows[2].Reason)
	}

	latest, err := repo.GetLatest(ctx, "2330")
	if err != nil {
		t.Fatalf("get latest failed: %v", err)
	}
	if !latest.TradeDate.Equal(base.AddDate(0, 0, 4)) {
		t.Fatalf("expected latest trade_date=%v, got %v", base.AddDate(0, 0, 4), latest.TradeDate)
	}
}
