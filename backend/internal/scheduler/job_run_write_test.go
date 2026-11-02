package scheduler

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/database"
	"github.com/trading/backend/internal/market"
	"github.com/trading/backend/internal/store"
)

// ── job_runs 的寫入：結束寫入的重試與 startRun 的上限（I-125；I-123～I-126 計畫書「3.5」）──

// writeAttempt 記一次寫入收到的 ctx 狀態。
type writeAttempt struct {
	err         error // ctx.Err()
	hasDeadline bool
	remaining   time.Duration // 收到時離期限還有多久
}

// flakyJobRunRepo 讓前 failFinish 次 Finish 失敗，並記下每一次寫入收到的 ctx。
type flakyJobRunRepo struct {
	schedulerJobRunRepoStub
	mu          sync.Mutex
	failFinish  int
	notFound    bool // Finish 一律回 store.ErrJobRunNotFound
	finishCalls []writeAttempt
	startCalls  []writeAttempt
}

func attemptOf(ctx context.Context) writeAttempt {
	dl, ok := ctx.Deadline()
	a := writeAttempt{err: ctx.Err(), hasDeadline: ok}
	if ok {
		a.remaining = time.Until(dl)
	}
	return a
}

func (r *flakyJobRunRepo) Start(ctx context.Context, jobName string) (uint64, error) {
	r.mu.Lock()
	r.startCalls = append(r.startCalls, attemptOf(ctx))
	r.mu.Unlock()
	return r.schedulerJobRunRepoStub.Start(ctx, jobName)
}

func (r *flakyJobRunRepo) Finish(ctx context.Context, runID uint64, status string, total, failed int, errMsg string) error {
	r.mu.Lock()
	r.finishCalls = append(r.finishCalls, attemptOf(ctx))
	fail := len(r.finishCalls) <= r.failFinish
	notFound := r.notFound
	r.mu.Unlock()
	if notFound {
		return store.ErrJobRunNotFound
	}
	if fail {
		// I-124 的形狀：DB 進入 recovery mode 時連線失敗。
		return errors.New("failed to connect: FATAL: the database system is in recovery mode (SQLSTATE 57P03)")
	}
	return r.schedulerJobRunRepoStub.Finish(ctx, runID, status, total, failed, errMsg)
}

func newWriteTestScheduler(repo *flakyJobRunRepo, backoffs []time.Duration) *Scheduler {
	return &Scheduler{jobRuns: repo, log: zap.NewNop(), finishRetryBackoffs: backoffs}
}

func TestFinishRunStatusRetriesUntilWritten(t *testing.T) {
	repo := &flakyJobRunRepo{failFinish: 2}
	s := newWriteTestScheduler(repo, []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond})

	if ok := s.finishRunStatus(context.Background(), 1, "corporate_action_sync", "failed", 1, 1, "x"); !ok {
		t.Fatal("第 3 次寫入成功，finishRunStatus 應回報 true")
	}
	if got := len(repo.finishCalls); got != 3 {
		t.Fatalf("Finish 呼叫 %d 次，期望 3 次（失敗 2 次後成功）", got)
	}
	if got := repo.snapshotFinished(); len(got) != 1 || got[0].status != "failed" {
		t.Fatalf("應恰好寫入一筆 failed，得到 %+v", got)
	}
}

// 剩餘限制（I-125）：重試全部失敗時⛔ 沒有任何一次寫入成功，那一筆仍停在 running，
// 只能等下次啟動由 AbortRunning 回收。
func TestFinishRunStatusGivesUpAfterBackoffs(t *testing.T) {
	repo := &flakyJobRunRepo{failFinish: 100}
	s := newWriteTestScheduler(repo, []time.Duration{time.Millisecond, time.Millisecond})

	if ok := s.finishRunStatus(context.Background(), 1, "corporate_action_sync", "failed", 1, 1, "x"); ok {
		t.Fatal("每一次都失敗，finishRunStatus 應回報 false")
	}
	if got := len(repo.finishCalls); got != 3 {
		t.Fatalf("Finish 呼叫 %d 次，期望 1 ＋ 2 次重試", got)
	}
	if got := repo.snapshotFinished(); len(got) != 0 {
		t.Fatalf("⛔ 不應有任何寫入成功，得到 %+v", got)
	}
}

// 每一次重試都有自己的 finishRunWriteTimeout，而且⛔ 不受呼叫端的取消影響
// （job 的 ctx 逾時後照樣寫得回去，2026-08-24 那次修正的同一條原則）。
func TestFinishRunStatusEachAttemptHasOwnTimeout(t *testing.T) {
	repo := &flakyJobRunRepo{failFinish: 1}
	s := newWriteTestScheduler(repo, []time.Duration{time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	s.finishRunStatus(ctx, 1, "intraday", "success", 11, 0, "")

	if len(repo.finishCalls) != 2 {
		t.Fatalf("Finish 呼叫 %d 次，期望 2 次", len(repo.finishCalls))
	}
	for i, a := range repo.finishCalls {
		if a.err != nil || !a.hasDeadline || a.remaining <= 0 || a.remaining > finishRunWriteTimeout {
			t.Errorf("第 %d 次寫入的 ctx = %+v，應未取消、帶自己的 %v 期限", i+1, a, finishRunWriteTimeout)
		}
	}
}

func TestDefaultFinishRetryBackoffsTotalWithinBudget(t *testing.T) {
	// 「3.2」的整輪截止依這個數字留餘裕：7 次 × 10 秒 ＋ 等待 63 秒 = 133 秒。
	total := time.Duration(len(defaultFinishRetryBackoffs)+1) * finishRunWriteTimeout
	for _, d := range defaultFinishRetryBackoffs {
		total += d
	}
	if total != 133*time.Second {
		t.Fatalf("結束寫入重試的最壞總時間 = %v，期望 133s（改了就要同步改計畫書與截止的餘裕）", total)
	}
}

// startRun 加了自己的寫入上限，而且⛔ 不受呼叫端的取消影響。
func TestStartRunHasWriteTimeout(t *testing.T) {
	repo := &flakyJobRunRepo{}
	s := newWriteTestScheduler(repo, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if id := s.startRun(ctx, "stock_symbol_sync"); id == 0 {
		t.Fatal("startRun 應取得 id")
	}
	a := repo.startCalls[0]
	if a.err != nil || !a.hasDeadline || a.remaining <= 0 || a.remaining > startRunWriteTimeout {
		t.Fatalf("startRun 的 ctx = %+v，應未取消、帶 %v 的期限", a, startRunWriteTimeout)
	}
}

// ── 沒有有效紀錄時⛔ 不能回報寫入成功（I-125 階段 A 的 review）────────────

// newSQLiteJobRunScheduler 用**真實的** SQLite repo（正式拓撲：store.NewDB → migrations）。
func newSQLiteJobRunScheduler(t *testing.T) (*Scheduler, store.JobRunRepo) {
	t.Helper()
	db, err := store.NewDB(config.DatabaseConfig{Driver: "sqlite", DSN: t.TempDir() + "/jobruns.db"})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.RunMigrations(context.Background(), db, "sqlite", zap.NewNop()); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	repo := store.NewJobRunRepo(db)
	return &Scheduler{jobRuns: repo, log: zap.NewNop(), finishRetryBackoffs: []time.Duration{time.Millisecond}}, repo
}

// review 的重現：startRun 失敗拿到 0 → 之後 DB 恢復 → 結束寫入⛔ 不能回報成功（表裡一筆都沒有）。
func TestFinishRunStatusWithoutRecordReportsFailure(t *testing.T) {
	ctx := context.Background()
	s, repo := newSQLiteJobRunScheduler(t)

	if ok := s.finishRunStatus(ctx, 0, "stock_symbol_sync", "success", 53401, 0, ""); ok {
		t.Fatal("runID 0 沒有紀錄可寫，finishRunStatus ⛔ 不能回報成功")
	}
	if ok := s.finishRunStatus(ctx, 999999, "stock_symbol_sync", "success", 53401, 0, ""); ok {
		t.Fatal("紀錄不存在，finishRunStatus ⛔ 不能回報成功")
	}
	if rows, err := repo.GetRecent(ctx, 10); err != nil || len(rows) != 0 {
		t.Fatalf("表裡應該沒有任何紀錄，得到 %d 筆（err=%v）", len(rows), err)
	}

	id := s.startRun(ctx, "stock_symbol_sync")
	if ok := s.finishRunStatus(ctx, id, "stock_symbol_sync", "success", 53401, 0, ""); !ok {
		t.Fatal("存在的紀錄應回報成功")
	}
}

// runID 0 時⛔ 不送 UPDATE：即使底層（例如 stub）回 nil，也⛔ 不能回報成功。
func TestFinishRunStatusSkipsZeroRunID(t *testing.T) {
	repo := &flakyJobRunRepo{}
	s := newWriteTestScheduler(repo, []time.Duration{time.Millisecond})

	if ok := s.finishRunStatus(context.Background(), 0, "stock_symbol_sync", "success", 1, 0, ""); ok {
		t.Fatal("runID 0，finishRunStatus ⛔ 不能回報成功")
	}
	if got := len(repo.finishCalls); got != 0 {
		t.Fatalf("Finish 呼叫 %d 次，期望 0 次", got)
	}
}

// 紀錄不存在時⛔ 不重試：重試也不會讓它出現。
func TestFinishRunStatusDoesNotRetryMissingRecord(t *testing.T) {
	repo := &flakyJobRunRepo{notFound: true}
	s := newWriteTestScheduler(repo, []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond})

	if ok := s.finishRunStatus(context.Background(), 7, "intraday", "success", 11, 0, ""); ok {
		t.Fatal("紀錄不存在，⛔ 不能回報成功")
	}
	if got := len(repo.finishCalls); got != 1 {
		t.Fatalf("Finish 呼叫 %d 次，期望只呼叫 1 次（⛔ 不重試）", got)
	}
}

// ── I-126：狀態與原因 ─────────────────────────────────────────────

// 整輪失敗記 failed、0/0（舊版是 0/1 → 落到 partial，一筆主檔都沒更新卻顯示 partial）。
func TestStockSymbolSyncWholeFailureIsFailed(t *testing.T) {
	t.Run("同步失敗", func(t *testing.T) {
		jobRuns := &schedulerJobRunRepoStub{}
		s := newWatchlistFailScheduler(jobRuns)
		s.stockSyncer = market.NewStockSymbolSyncer(failingSymbolSource{}, nil, zap.NewNop())

		s.runStockSymbolSync(context.Background())

		got := jobRuns.snapshotFinished()
		if len(got) != 1 {
			t.Fatalf("應寫入一筆，得到 %+v", got)
		}
		if got[0].status != "failed" || got[0].symbolsTotal != 0 || got[0].symbolsFailed != 0 {
			t.Fatalf("得到 %s %d/%d，期望 failed 0/0", got[0].status, got[0].symbolsTotal, got[0].symbolsFailed)
		}
		if !strings.HasPrefix(got[0].errMsg, "symbol_sync:") {
			t.Fatalf("error = %q，應是 symbol_sync:<reason>", got[0].errMsg)
		}
	})
	t.Run("沒有設定 syncer", func(t *testing.T) {
		jobRuns := &schedulerJobRunRepoStub{}
		s := newWatchlistFailScheduler(jobRuns)

		s.runStockSymbolSync(context.Background())

		got := jobRuns.snapshotFinished()
		if len(got) != 1 || got[0].status != "failed" || got[0].symbolsTotal != 0 || got[0].symbolsFailed != 0 {
			t.Fatalf("得到 %+v，期望 failed 0/0", got)
		}
	})
}

// 前置條件不成立寫專屬的安全文字，⛔ 不再壓成 internal_error。
func TestDelistingNotReadyHasOwnReason(t *testing.T) {
	jobRuns := &delistingJobRunStub{}
	s := newDelistingScheduler(jobRuns, &delistingSourceStub{result: okSourceResult()},
		delistingRepoStub{}, delistingReadinessStub{ready: false})

	s.RunDelistingReconcile()

	_, finished := jobRuns.snapshot()
	if len(finished) != 1 {
		t.Fatalf("應寫入一筆，得到 %+v", finished)
	}
	if finished[0].status != "failed" || finished[0].errMsg != "delisting_reconcile:stock_symbol_sync_not_ready" {
		t.Fatalf("得到 %s %q，期望 failed delisting_reconcile:stock_symbol_sync_not_ready", finished[0].status, finished[0].errMsg)
	}
}

// 其他失敗照舊走 joberr 的分類。
func TestDelistingOtherFailuresStillClassified(t *testing.T) {
	jobRuns := &delistingJobRunStub{}
	s := newDelistingScheduler(jobRuns, &delistingSourceStub{err: errors.New("dial tcp: i/o timeout")},
		delistingRepoStub{}, delistingReadinessStub{ready: true})

	s.RunDelistingReconcile()

	_, finished := jobRuns.snapshot()
	if len(finished) != 1 || finished[0].errMsg != "delisting_reconcile:timeout" {
		t.Fatalf("得到 %+v，期望 delisting_reconcile:timeout", finished)
	}
}
