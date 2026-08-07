package store

import (
	"context"
	"os"
	"testing"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/database"
)

func newTestMarketBackfillJobRepo(t *testing.T) MarketBackfillJobRepo {
	t.Helper()
	tmp, err := os.CreateTemp("", "market-backfill-job-test-*.db")
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
	return NewMarketBackfillJobRepo(db)
}

func TestMarketBackfillJobLifecycle(t *testing.T) {
	repo := newTestMarketBackfillJobRepo(t)
	ctx := context.Background()

	job := &MarketBackfillJob{
		JobID:        "bf_20260807_010203_000",
		Symbols:      `["2330","2454"]`,
		Days:         1825,
		SymbolsTotal: 2,
	}
	if err := repo.Create(ctx, job); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if job.ID == 0 {
		t.Fatal("create 沒有回填自增 id")
	}

	got, err := repo.GetByJobID(ctx, job.JobID)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	// Create 未指定時要補上預設值；failures 必須是 "[]" 而不是 SQL NULL，
	// 因為 RawJSON 是純 string、沒有實作 sql.Scanner，NULL 會讓 scan 直接失敗。
	if got.Status != "pending" {
		t.Fatalf("status = %q, want pending", got.Status)
	}
	if got.Failures != "[]" {
		t.Fatalf("failures = %q, want []", got.Failures)
	}
	if got.Days != 1825 || got.SymbolsTotal != 2 {
		t.Fatalf("days/total = %d/%d, want 1825/2", got.Days, got.SymbolsTotal)
	}

	if err := repo.UpdateProgress(ctx, job.JobID, 1, 1, RawJSON(`[{"symbol":"2454","error":"boom"}]`)); err != nil {
		t.Fatalf("update progress failed: %v", err)
	}
	got, err = repo.GetByJobID(ctx, job.JobID)
	if err != nil {
		t.Fatalf("get after progress failed: %v", err)
	}
	if got.Status != "running" {
		t.Fatalf("status = %q, want running（UpdateProgress 要順手把狀態推進）", got.Status)
	}
	if got.SymbolsDone != 1 || got.SymbolsFailed != 1 {
		t.Fatalf("done/failed = %d/%d, want 1/1", got.SymbolsDone, got.SymbolsFailed)
	}
	if !got.StartedAt.Valid {
		t.Fatal("started_at 應在第一次 UpdateProgress 時被填上")
	}
	firstStarted := got.StartedAt.Time

	// 第二次更新不可覆蓋 started_at（COALESCE），否則進度一動就重設開始時間。
	if err := repo.UpdateProgress(ctx, job.JobID, 2, 1, RawJSON(`[{"symbol":"2454","error":"boom"}]`)); err != nil {
		t.Fatalf("second update failed: %v", err)
	}
	got, _ = repo.GetByJobID(ctx, job.JobID)
	if !got.StartedAt.Time.Equal(firstStarted) {
		t.Fatalf("started_at 被第二次更新覆蓋了：%v -> %v", firstStarted, got.StartedAt.Time)
	}

	if err := repo.Finish(ctx, job.JobID, "partial", "some symbols failed"); err != nil {
		t.Fatalf("finish failed: %v", err)
	}
	got, _ = repo.GetByJobID(ctx, job.JobID)
	if got.Status != "partial" {
		t.Fatalf("status = %q, want partial", got.Status)
	}
	if !got.Error.Valid || got.Error.String != "some symbols failed" {
		t.Fatalf("error = %+v, want some symbols failed", got.Error)
	}
	if !got.FinishedAt.Valid {
		t.Fatal("finished_at 應在 Finish 時被填上")
	}
}

func TestMarketBackfillJobGetUnknownReturnsError(t *testing.T) {
	repo := newTestMarketBackfillJobRepo(t)

	if _, err := repo.GetByJobID(context.Background(), "bf_does_not_exist"); err == nil {
		t.Fatal("查不到的 job_id 應回錯誤，handler 靠它回 404")
	}
}
