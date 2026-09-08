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
	"github.com/trading/backend/internal/joberr"
	"github.com/trading/backend/internal/market"
	"github.com/trading/backend/internal/store"
	"github.com/trading/backend/pkg/timeutil"
)

// 這一組驗的是**排程層**的契約（計畫書 docs/todo.md T-071 的 #17～#18、#20f4）：
// single-flight、早退時的鎖釋放、job_runs 的寫入與 partial 原因。
// 對帳邏輯本身在 internal/market 有自己的測試，這裡刻意不重複。

// ── stubs ────────────────────────────────────────────────────────────────

// delistingSourceStub 可選擇性地卡住，讓「一輪還在跑」這個狀態穩定可測。
type delistingSourceStub struct {
	mu     sync.Mutex
	calls  int
	err    error
	result market.DelistingSourceResult
	// block 不為 nil 時，FetchDelistings 會等它被關閉才回應。
	block chan struct{}
	// entered 在進入 FetchDelistings 時關閉，讓測試知道背景那輪真的開始了
	// （⛔ 不能靠 sleep 猜——那會讓 CI 上偶爾在鎖還沒被持有時就斷言）。
	entered chan struct{}
}

func (s *delistingSourceStub) FetchDelistings(ctx context.Context) (market.DelistingSourceResult, error) {
	s.mu.Lock()
	s.calls++
	s.mu.Unlock()
	if s.entered != nil {
		close(s.entered)
		s.entered = nil
	}
	if s.block != nil {
		<-s.block
	}
	if s.err != nil {
		return market.DelistingSourceResult{}, s.err
	}
	return s.result, nil
}

func (s *delistingSourceStub) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

// delistingRepoStub 只實作 WithTx——那是 store.DelistingRepo 的全部。
// **回傳 nil 而不呼叫 fn** 等於「交易內一切正常但沒有任何 degradation」，
// 正好是排程層要的成功路徑；要驗早退時就讓它回傳錯誤。
type delistingRepoStub struct {
	err error
}

func (r delistingRepoStub) WithTx(ctx context.Context, fn func(store.DelistingTx) error) error {
	return r.err
}

type delistingReadinessStub struct {
	ready bool
	err   error
}

func (s delistingReadinessStub) StockSymbolSyncSucceededToday(ctx context.Context, now time.Time) (bool, error) {
	return s.ready, s.err
}

// delistingJobRunStub 記錄 job_runs 的寫入。
//
// ⚠️ **要能被並行存取**：single-flight 的測試會讓一輪卡在背景 goroutine，
// 主 goroutine 同時觸發另一輪並斷言結果（`-race` 會抓到沒保護的存取）。
type delistingJobRunStub struct {
	mu       sync.Mutex
	startErr error
	nextID   uint64
	started  []string
	finished []schedulerJobRunFinish
	// finishCtxErr 記錄 Finish 收到的 ctx 當下的 Err()：finishRunStatus 必須走
	// context.WithoutCancel，逾時之後仍要寫得回去（不共用底層 writer 就是把 I-084 重新引入）。
	finishCtxErr []error
}

func (s *delistingJobRunStub) Start(ctx context.Context, jobName string) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.startErr != nil {
		return 0, s.startErr
	}
	s.started = append(s.started, jobName)
	s.nextID++
	return s.nextID, nil
}

func (s *delistingJobRunStub) Finish(ctx context.Context, runID uint64, status string, symbolsTotal, symbolsFailed int, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.finishCtxErr = append(s.finishCtxErr, ctx.Err())
	s.finished = append(s.finished, schedulerJobRunFinish{
		runID: runID, status: status,
		symbolsTotal: symbolsTotal, symbolsFailed: symbolsFailed, errMsg: errMsg,
	})
	return nil
}

func (s *delistingJobRunStub) GetRecent(ctx context.Context, limit int) ([]store.JobRun, error) {
	return nil, nil
}
func (s *delistingJobRunStub) GetLatestPerJob(ctx context.Context) ([]store.JobRun, error) {
	return nil, nil
}
func (s *delistingJobRunStub) DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return 0, nil
}
func (s *delistingJobRunStub) AbortRunning(ctx context.Context) (int64, error) { return 0, nil }

func (s *delistingJobRunStub) snapshot() ([]string, []schedulerJobRunFinish) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.started...), append([]schedulerJobRunFinish(nil), s.finished...)
}

func newDelistingScheduler(
	jobRuns *delistingJobRunStub, source market.DelistingSource,
	repo store.DelistingRepo, readiness market.SyncReadinessChecker,
) *Scheduler {
	s := &Scheduler{jobRuns: jobRuns, log: zap.NewNop()}
	s.SetDelistingReconcile(
		market.NewDelistingReconciler(source, repo, readiness, zap.NewNop()),
		config.DelistingConfig{Enabled: true},
	)
	return s
}

func okSourceResult() market.DelistingSourceResult {
	return market.DelistingSourceResult{
		Rows: []market.DelistingEventRow{{
			Symbol: "2301", CompanyName: "光寶電子",
			DelistedDate: time.Date(2002, 12, 23, 0, 0, 0, 0, timeutil.TaipeiTZ),
		}},
		RowCount: 265, DistinctSymbols: 265, ContentHash: "abc",
	}
}

// ── #17／#17b／#17c：single-flight ────────────────────────────────────────

// cron 撞上進行中的手動觸發時**只記 Warn、不寫 job_run**：兩個入口共用同一個
// job_name，寫一筆 failed 會讓 GetLatestPerJob（ORDER BY started_at DESC, id DESC）
// 選到 cron 那筆，把手動那次的成功顯示成失敗。
func TestDelistingReconcileCronSkippedWhileManualRunningWritesNoJobRun(t *testing.T) {
	jobRuns := &delistingJobRunStub{}
	source := &delistingSourceStub{
		result: okSourceResult(), block: make(chan struct{}), entered: make(chan struct{}),
	}
	entered := source.entered
	s := newDelistingScheduler(jobRuns, source, delistingRepoStub{}, delistingReadinessStub{ready: true})

	if !s.TryStartDelistingReconcile(false) {
		t.Fatal("manual trigger should acquire ownership")
	}
	<-entered // 背景那輪確實已經在跑

	s.RunDelistingReconcile() // cron 撞上，必須立刻返回

	if got := source.callCount(); got != 1 {
		t.Fatalf("blocked cron round must not fetch the source, fetch calls = %d, want 1", got)
	}
	close(source.block)

	waitForFinished(t, jobRuns, 1)
	started, finished := jobRuns.snapshot()
	if len(started) != 1 || started[0] != delistingReconcileJobName {
		t.Fatalf("blocked cron round must not write a job_run, started = %v", started)
	}
	if finished[0].status != "success" {
		t.Fatalf("manual round status = %q, want success", finished[0].status)
	}
	if finished[0].symbolsTotal != 265 || finished[0].symbolsFailed != 0 {
		t.Fatalf("symbols_total/failed = %d/%d, want 265/0",
			finished[0].symbolsTotal, finished[0].symbolsFailed)
	}
}

// 手動與手動重疊：第二次**同步**回 false（handler 據此回 409）。
// ⛔ 不得回 true 再在背景靜默跳過——那是既有 evaluation_universe_sync 的模式。
func TestDelistingReconcileManualOverlapIsRejectedSynchronously(t *testing.T) {
	jobRuns := &delistingJobRunStub{}
	source := &delistingSourceStub{
		result: okSourceResult(), block: make(chan struct{}), entered: make(chan struct{}),
	}
	entered := source.entered
	s := newDelistingScheduler(jobRuns, source, delistingRepoStub{}, delistingReadinessStub{ready: true})

	if !s.TryStartDelistingReconcile(false) {
		t.Fatal("first manual trigger should acquire ownership")
	}
	<-entered

	// accept_shrink 的手動觸發共用同一把鎖（#17c）。
	if s.TryStartDelistingReconcile(true) {
		t.Fatal("second manual trigger must be rejected while a round is running")
	}

	close(source.block)
	waitForFinished(t, jobRuns, 1)
	if started, _ := jobRuns.snapshot(); len(started) != 1 {
		t.Fatalf("rejected manual trigger must not write a job_run, started = %v", started)
	}

	// 前一輪結束後鎖必須放掉，否則這個 job 會永久卡住。
	if !s.TryStartDelistingReconcile(false) {
		t.Fatal("ownership should be released after the round finished")
	}
}

// ── #17d：四種早退都必須釋放鎖，且各寫一筆 failed ────────────────────────

func TestDelistingReconcileEarlyExitsReleaseOwnershipAndRecordFailure(t *testing.T) {
	cases := []struct {
		name      string
		source    *delistingSourceStub
		repo      store.DelistingRepo
		readiness market.SyncReadinessChecker
	}{
		{
			// 依賴不成立：當日 stock_symbol_sync 沒成功（fail-closed 前置）。
			name:      "sync_not_ready",
			source:    &delistingSourceStub{result: okSourceResult()},
			repo:      delistingRepoStub{},
			readiness: delistingReadinessStub{ready: false},
		},
		{
			name:      "fetch_failed",
			source:    &delistingSourceStub{err: errors.New("unexpected status 503")},
			repo:      delistingRepoStub{},
			readiness: delistingReadinessStub{ready: true},
		},
		{
			// 解析出 0 筆：⛔ 空名單一律失敗，不能當成「今天沒有下市公司」。
			name:      "parse_empty",
			source:    &delistingSourceStub{err: market.ErrEmptySuspendListing},
			repo:      delistingRepoStub{},
			readiness: delistingReadinessStub{ready: true},
		},
		{
			name:      "shrink_guard",
			source:    &delistingSourceStub{result: okSourceResult()},
			repo:      delistingRepoStub{err: market.ErrSourceShrunk},
			readiness: delistingReadinessStub{ready: true},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			jobRuns := &delistingJobRunStub{}
			s := newDelistingScheduler(jobRuns, tc.source, tc.repo, tc.readiness)

			s.RunDelistingReconcile()

			_, finished := jobRuns.snapshot()
			if len(finished) != 1 {
				t.Fatalf("early exit should record exactly one job_run, got %+v", finished)
			}
			if finished[0].status != "failed" {
				t.Fatalf("status = %q, want failed", finished[0].status)
			}
			// symbols_failed 恆為 0：這個 job 沒有「逐檔失敗」的概念，
			// ⛔ 不得用假的 1/1 去湊（那兩個欄位的單位是標的數）。
			if finished[0].symbolsFailed != 0 {
				t.Fatalf("symbols_failed = %d, want 0", finished[0].symbolsFailed)
			}
			// error 只能是 joberr 的封閉值域，⛔ 原始錯誤只進 log。
			if !strings.HasPrefix(finished[0].errMsg, "delisting_reconcile:") {
				t.Fatalf("error = %q, want a delisting_reconcile:<reason> summary", finished[0].errMsg)
			}
			if strings.Contains(finished[0].errMsg, "503") {
				t.Fatalf("error leaked the raw upstream message: %q", finished[0].errMsg)
			}

			// 早退**不得**自行釋放鎖，但入口的 defer 必須放掉。
			if !s.TryStartDelistingReconcile(false) {
				t.Fatal("ownership should be released after an early exit")
			}
		})
	}
}

// ── #18：逾時之後仍寫得回 job_runs ───────────────────────────────────────

// finishRunStatus 必須與 finishRunDegraded 共用底層 writer（context.WithoutCancel
// ＋ finishRunWriteTimeout）。⚠️ 這個 job 特別容易踩到——CSV 抓取逾時正是走這條
// explicit-failed 路徑，而那時 ctx 已經被取消。不共用就是把 I-084 重新引入。
func TestDelistingReconcileWritesFailedWithCanceledContext(t *testing.T) {
	jobRuns := &delistingJobRunStub{}
	source := &delistingSourceStub{err: context.DeadlineExceeded}
	s := newDelistingScheduler(jobRuns, source, delistingRepoStub{}, delistingReadinessStub{ready: true})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	s.runDelistingReconcileOwned(ctx, false)

	_, finished := jobRuns.snapshot()
	if len(finished) != 1 || finished[0].status != "failed" {
		t.Fatalf("canceled context should still record a failed job_run, got %+v", finished)
	}
	if finished[0].symbolsFailed != 0 {
		t.Fatalf("symbols_failed = %d, want 0", finished[0].symbolsFailed)
	}
	jobRuns.mu.Lock()
	defer jobRuns.mu.Unlock()
	if len(jobRuns.finishCtxErr) != 1 || jobRuns.finishCtxErr[0] != nil {
		t.Fatalf("Finish must run on a WithoutCancel context, ctx errors = %v", jobRuns.finishCtxErr)
	}
}

// ── #20f4：startRun 失敗必須中止整輪 ─────────────────────────────────────

// runID = 0 代表這一輪沒有任何 audit record。繼續做下去會寫入一整輪資料卻在
// job_runs 上完全看不到；而 Finish 不檢查 affected rows，WHERE id = 0 會靜默
// 更新零列，連失敗都留不下痕跡。
func TestDelistingReconcileAbortsWhenStartRunFails(t *testing.T) {
	jobRuns := &delistingJobRunStub{startErr: errors.New("db down")}
	source := &delistingSourceStub{result: okSourceResult()}
	s := newDelistingScheduler(jobRuns, source, delistingRepoStub{}, delistingReadinessStub{ready: true})

	s.RunDelistingReconcile()

	if got := source.callCount(); got != 0 {
		t.Fatalf("no business work may run without a job_run id, fetch calls = %d", got)
	}
	if _, finished := jobRuns.snapshot(); len(finished) != 0 {
		t.Fatalf("must not call Finish with runID = 0, got %+v", finished)
	}
	if !s.TryStartDelistingReconcile(false) {
		t.Fatal("ownership should be released after aborting")
	}
}

// ── 未注入 reconciler 時不做任何事 ───────────────────────────────────────

func TestDelistingReconcileNoopWithoutReconciler(t *testing.T) {
	jobRuns := &delistingJobRunStub{}
	s := &Scheduler{jobRuns: jobRuns, log: zap.NewNop()}

	s.RunDelistingReconcile()

	if started, finished := jobRuns.snapshot(); len(started) != 0 || len(finished) != 0 {
		t.Fatalf("uninjected job must not touch job_runs, started=%v finished=%v", started, finished)
	}
}

// ── partial 的原因必須原樣寫進 job_runs.error ───────────────────────────

// ⛔ 這裡用的是 joberr.Describe（SafeMessenger 原樣通過），不是 joberr.Summary
// （走 Classify，會把整段計數壓成 internal_error）。壓掉之後排程頁只看得到
// partial、看不到為什麼——資訊淨損失、零安全收益。
func TestDelistingDegradedReasonKeepsCountsAndLeaksNothing(t *testing.T) {
	summary := market.DelistingOutcome{
		SourceMissing: 1, IdentityDoubt: 2, SourceCorrected: 1, Revoked: 1,
	}.Summary()

	got := joberr.Describe(delistingDegradedError{summary: summary})

	if got != summary {
		t.Fatalf("safe summary was rewritten:\n got: %q\nwant: %q", got, summary)
	}
	if got == string(joberr.Internal) {
		t.Fatal("summary was squashed into internal_error")
	}
	for _, unsafe := range []string{"://", "password", "dsn", "select ", "@"} {
		if strings.Contains(strings.ToLower(got), unsafe) {
			t.Fatalf("summary looks unsafe (%q): %q", unsafe, got)
		}
	}
}

// ── cron 字串 ───────────────────────────────────────────────────────────

func TestDelistingCronFallsBackToDefault(t *testing.T) {
	s := &Scheduler{log: zap.NewNop()}
	if got := s.delistingCron(); got != defaultDelistingReconcileCron {
		t.Fatalf("empty config cron = %q, want %q", got, defaultDelistingReconcileCron)
	}

	s.delistingCfg = config.DelistingConfig{Cron: "30 8 * * *"}
	if got := s.delistingCron(); got != "30 8 * * *" {
		t.Fatalf("config cron = %q, want 30 8 * * *", got)
	}
}

// ── 當日 stock_symbol_sync 是否成功（fail-closed 前置）────────────────────

type readinessJobRunRepoStub struct {
	rows []store.JobRun
	err  error
}

func (r readinessJobRunRepoStub) Start(ctx context.Context, jobName string) (uint64, error) {
	return 1, nil
}
func (r readinessJobRunRepoStub) Finish(ctx context.Context, runID uint64, status string, symbolsTotal, symbolsFailed int, errMsg string) error {
	return nil
}
func (r readinessJobRunRepoStub) GetRecent(ctx context.Context, limit int) ([]store.JobRun, error) {
	return nil, nil
}
func (r readinessJobRunRepoStub) GetLatestPerJob(ctx context.Context) ([]store.JobRun, error) {
	return r.rows, r.err
}
func (r readinessJobRunRepoStub) DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return 0, nil
}
func (r readinessJobRunRepoStub) AbortRunning(ctx context.Context) (int64, error) { return 0, nil }

func TestJobRunReadinessRequiresTodaySuccess(t *testing.T) {
	now := time.Date(2026, 9, 7, 7, 0, 0, 0, timeutil.TaipeiTZ)
	taipeiToday := time.Date(2026, 9, 7, 6, 30, 0, 0, timeutil.TaipeiTZ)

	cases := []struct {
		name string
		rows []store.JobRun
		want bool
	}{
		{"today_success", []store.JobRun{
			{JobName: "stock_symbol_sync", Status: "success", StartedAt: taipeiToday}}, true},
		// 「今天跑過但失敗」與「今天還沒跑」處置相同——兩者的 is_listed 都不可信。
		{"today_failed", []store.JobRun{
			{JobName: "stock_symbol_sync", Status: "failed", StartedAt: taipeiToday}}, false},
		{"yesterday_success", []store.JobRun{
			{JobName: "stock_symbol_sync", Status: "success",
				StartedAt: taipeiToday.AddDate(0, 0, -1)}}, false},
		{"never_run", nil, false},
		{"other_job_only", []store.JobRun{
			{JobName: "daily_close", Status: "success", StartedAt: taipeiToday}}, false},
		// UTC 表示法的同一個台北時間仍算今天（06:30 台北 = 前一日 22:30 UTC）。
		{"today_success_in_utc", []store.JobRun{
			{JobName: "stock_symbol_sync", Status: "success",
				StartedAt: taipeiToday.UTC()}}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := NewJobRunReadiness(readinessJobRunRepoStub{rows: tc.rows})
			got, err := r.StockSymbolSyncSucceededToday(context.Background(), now)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("ready = %v, want %v", got, tc.want)
			}
		})
	}
}

// 讀不到 job_runs 時**回錯而不是 false**：呼叫端要把它當成整輪失敗，
// 而不是與「sync 沒跑成功」混為一談。
func TestJobRunReadinessPropagatesRepoError(t *testing.T) {
	r := NewJobRunReadiness(readinessJobRunRepoStub{err: errors.New("db down")})

	if _, err := r.StockSymbolSyncSucceededToday(context.Background(), time.Now()); err == nil {
		t.Fatal("repo error should be propagated")
	}
}

// waitForFinished 等背景那輪把 job_run 寫完。
func waitForFinished(t *testing.T, jobRuns *delistingJobRunStub, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, finished := jobRuns.snapshot(); len(finished) >= want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d finished job_runs", want)
}
