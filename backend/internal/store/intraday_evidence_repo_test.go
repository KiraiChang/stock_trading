package store

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/database"
)

func newIntradayEvidenceTestRepo(t *testing.T) IntradayEvidenceRepo {
	t.Helper()
	db, err := NewDB(config.DatabaseConfig{Driver: "sqlite", DSN: t.TempDir() + "/evidence.db"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.RunMigrations(context.Background(), db, "sqlite", zap.NewNop()); err != nil {
		t.Fatal(err)
	}
	return NewIntradayEvidenceRepo(db)
}

func ns(s string) NullString { return NullString{NullString: sql.NullString{String: s, Valid: s != ""}} }

func symRow(runID uint64, sym, source, status, fp string, at time.Time) IntradaySymbolRun {
	return IntradaySymbolRun{RunID: runID, Symbol: sym, Source: source, DataAt: at, DataStatus: status, Fingerprint: ns(fp), FetchAttempts: 1}
}

// T-088＋I-103：逐輪逐檔的存證。最近一筆、上一次成功評估、逐輪列出、清理。
func TestIntradayEvidenceRepo(t *testing.T) {
	ctx := context.Background()
	repo := newIntradayEvidenceTestRepo(t)
	t0 := time.Date(2026, 11, 2, 9, 5, 0, 0, time.UTC)

	if r, err := repo.LatestRound(ctx, IntradaySourceScheduled); err != nil || r != nil {
		t.Fatalf("空表應回 nil，得到 %+v（err=%v）", r, err)
	}
	if err := repo.InsertRound(ctx, IntradayRound{RunID: 10, Source: IntradaySourceScheduled, ScheduledAt: t0,
		LockAcquiredAt: NullTime{NullTime: sql.NullTime{Time: t0, Valid: true}}, Outcome: IntradayOutcomeExecuted}); err != nil {
		t.Fatal(err)
	}
	if err := repo.InsertRound(ctx, IntradayRound{RunID: 11, Source: IntradaySourceScheduled, ScheduledAt: t0.Add(5 * time.Minute),
		LockWaitMs: 60000, Outcome: IntradayOutcomeSkippedLockTimeout}); err != nil {
		t.Fatal(err)
	}
	if err := repo.FinishRound(ctx, 10, t0.Add(time.Minute), 2, ""); err != nil {
		t.Fatal(err)
	}
	if err := repo.FinishRound(ctx, 999, t0, 0, ""); !errors.Is(err, ErrIntradayEvidenceNotFound) {
		t.Fatalf("不存在的輪次應回 not found，得到 %v", err)
	}
	r, err := repo.LatestRound(ctx, IntradaySourceScheduled)
	if err != nil || r == nil || r.RunID != 11 || r.Outcome != IntradayOutcomeSkippedLockTimeout || r.LockAcquiredAt.Valid {
		t.Fatalf("最新一輪 = %+v（err=%v）", r, err)
	}

	// run 10：A 成功評估（指紋 fa）、B 評估失敗；run 12：A 沒有評估段（評估中）；run 13：A 補抓。
	if err := repo.InsertSymbolRuns(ctx, []IntradaySymbolRun{
		symRow(10, "A", IntradaySourceScheduled, "written", "fa", t0),
		symRow(10, "B", IntradaySourceScheduled, "written", "fb", t0),
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateSymbolEval(ctx, 10, "A", t0, IntradayEvalNoSignal, ""); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateSymbolEval(ctx, 10, "B", t0, IntradayEvalFailed, "insufficient_data"); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateSymbolEval(ctx, 10, "Z", t0, IntradayEvalNoSignal, ""); !errors.Is(err, ErrIntradayEvidenceNotFound) {
		t.Fatalf("不存在的列應回 not found，得到 %v", err)
	}
	if err := repo.InsertSymbolRuns(ctx, []IntradaySymbolRun{symRow(12, "A", IntradaySourceScheduled, "written", "fa2", t0.Add(10*time.Minute))}); err != nil {
		t.Fatal(err)
	}
	if err := repo.InsertSymbolRuns(ctx, []IntradaySymbolRun{symRow(13, "A", IntradaySourceRefill, "written", "", t0.Add(11*time.Minute))}); err != nil {
		t.Fatal(err)
	}

	latest, err := repo.LatestSymbolRuns(ctx, IntradaySourceScheduled, []string{"A", "B", "C"})
	if err != nil || latest["A"].RunID != 12 || latest["B"].RunID != 10 || len(latest) != 2 {
		t.Fatalf("最近一筆 = %+v（err=%v）", latest, err)
	}
	if latest["A"].EvalStatus.Valid {
		t.Fatalf("run 12 還沒有評估段，得到 %+v", latest["A"])
	}
	refill, err := repo.LatestSymbolRuns(ctx, IntradaySourceRefill, []string{"A"})
	if err != nil || refill["A"].RunID != 13 {
		t.Fatalf("補抓最近一筆 = %+v（err=%v）", refill, err)
	}
	ok, err := repo.LastSuccessfulEvals(ctx, []string{"A", "B"})
	if err != nil || ok["A"].RunID != 10 || ok["A"].Fingerprint.String != "fa" {
		t.Fatalf("上一次成功評估 = %+v（err=%v）", ok, err)
	}
	if _, has := ok["B"]; has {
		t.Fatal("B 只有評估失敗，⛔ 不算成功評估")
	}
	list, err := repo.ListSymbolRuns(ctx, 10)
	if err != nil || len(list) != 2 || list[0].Symbol != "A" || list[1].EvalCause.String != "insufficient_data" {
		t.Fatalf("逐輪列出 = %+v（err=%v）", list, err)
	}

	// 門檻 t0+10m：輪次 10（t0）、11（t0+5m）與 run 10 的兩列（t0）→ 共 4；run 12（剛好 t0+10m）與 13 保留。
	n, err := repo.DeleteBefore(ctx, t0.Add(10*time.Minute))
	if err != nil || n != 4 {
		t.Fatalf("清理筆數 = %d（err=%v），期望 4", n, err)
	}
	if list, _ := repo.ListSymbolRuns(ctx, 10); len(list) != 0 {
		t.Fatalf("run 10 應被清掉，剩 %d 列", len(list))
	}
}

func TestIntradayEvalSucceeded(t *testing.T) {
	for _, s := range []string{IntradayEvalNoSignal, IntradayEvalSignalSent, IntradayEvalSignalSuppressed, IntradayEvalWithheldStale, IntradayEvalDegraded} {
		if !IntradayEvalSucceeded(s) {
			t.Errorf("%s 應算成功評估", s)
		}
	}
	for _, s := range []string{IntradayEvalFailed, IntradayEvalNotEvaluated, ""} {
		if IntradayEvalSucceeded(s) {
			t.Errorf("%q ⛔ 不算成功評估", s)
		}
	}
}
