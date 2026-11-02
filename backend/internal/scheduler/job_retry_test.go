package scheduler

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/market"
	"github.com/trading/backend/internal/store"
	"github.com/trading/backend/pkg/timeutil"
)

// 自動重跑（I-123～I-126 計畫書「3.2」～「3.4」「八」）。時間一律用注入的時鐘與計時器，⛔ 不真的等。

// ── 注入的時鐘 ─────────────────────────────────────────────────────

type fakeRetryTimer struct {
	c       *fakeRetryClock
	at      time.Time
	d       time.Duration
	f       func()
	stopped bool
	fired   bool
}

func (t *fakeRetryTimer) Stop() bool {
	t.c.mu.Lock()
	defer t.c.mu.Unlock()
	active := !t.stopped && !t.fired
	t.stopped = true
	return active
}

// fakeRetryClock 是手動推進的時鐘：到期的計時器由 advanceTo 在測試的 goroutine 裡依序同步執行；
// autoFire 回 true 的時長會立刻觸發（層二、層三的等待），並記進 waits。
type fakeRetryClock struct {
	mu       sync.Mutex
	now      time.Time
	timers   []*fakeRetryTimer
	autoFire func(d time.Duration) bool
	waits    []time.Duration
}

func newFakeRetryClock(at time.Time) *fakeRetryClock { return &fakeRetryClock{now: at} }

func (c *fakeRetryClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeRetryClock) AfterFunc(d time.Duration, f func()) retryTimer {
	c.mu.Lock()
	t := &fakeRetryTimer{c: c, at: c.now.Add(d), d: d, f: f}
	auto := c.autoFire != nil && c.autoFire(d)
	if auto {
		t.fired = true
		c.waits = append(c.waits, d)
	} else {
		c.timers = append(c.timers, t)
	}
	c.mu.Unlock()
	if auto {
		go f()
	}
	return t
}

// advanceTo 把時鐘推到 at，依到期順序同步執行到期、未停止的計時器，回傳執行了幾個。
func (c *fakeRetryClock) advanceTo(at time.Time) int {
	c.mu.Lock()
	c.now = at
	c.mu.Unlock()
	n := 0
	for {
		c.mu.Lock()
		var next *fakeRetryTimer
		for _, t := range c.timers {
			if t.stopped || t.fired || t.at.After(c.now) {
				continue
			}
			if next == nil || t.at.Before(next.at) {
				next = t
			}
		}
		if next != nil {
			next.fired = true
		}
		c.mu.Unlock()
		if next == nil {
			return n
		}
		next.f()
		n++
	}
}

// fireDuration 立刻觸發所有時長為 d、還在等的計時器（例如層二的 120 秒預算到期）。
func (c *fakeRetryClock) fireDuration(d time.Duration) {
	c.mu.Lock()
	var due []*fakeRetryTimer
	for _, t := range c.timers {
		if !t.stopped && !t.fired && t.d == d {
			t.fired = true
			due = append(due, t)
		}
	}
	c.mu.Unlock()
	for _, t := range due {
		t.f()
	}
}

// pending 回傳還在等的計時器時刻（台北 HH:MM）。
func (c *fakeRetryClock) pending() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []string
	for _, t := range c.timers {
		if !t.stopped && !t.fired {
			out = append(out, retryClockText(t.at))
		}
	}
	return out
}

func (c *fakeRetryClock) waited() []time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]time.Duration(nil), c.waits...)
}

// tpe 是 2026-11-02（一）的台北時刻；tpeOn 可指定日期。
func tpe(h, m int) time.Time { return tpeOn(2, h, m) }

func tpeOn(day, h, m int) time.Time { return time.Date(2026, 11, day, h, m, 0, 0, timeutil.TaipeiTZ) }

// ── 失敗的種類 ─────────────────────────────────────────────────────

// transientDialErr 是 I-122 看到的 DNS 逾時：`Get …: dial tcp: lookup …: i/o timeout`。
func transientDialErr() error {
	return &url.Error{Op: "Get", URL: "https://isin.twse.com.tw/x", Err: &net.OpError{
		Op: "dial", Net: "tcp", Err: &net.DNSError{Err: "i/o timeout", Name: "isin.twse.com.tw", IsTimeout: true},
	}}
}

var errRetryPermanent = errors.New("twse isin: parse error")

// ── job_runs：記下每一筆，並照真實 SQL 的語意回「每個工作的最新一筆」──────────

type retryRun struct {
	id      uint64
	job     string
	status  string
	total   int
	failed  int
	errMsg  string
	started time.Time
}

type retryJobRunStub struct {
	mu   sync.Mutex
	runs []*retryRun
	// failStartJob 的 Start 一律失敗（寫不進開始紀錄 → runID 0）。
	failStartJob string
}

func (s *retryJobRunStub) Start(_ context.Context, job string) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if job == s.failStartJob {
		return 0, errors.New("job_runs start: db unavailable")
	}
	r := &retryRun{id: uint64(len(s.runs) + 1), job: job, status: "running", started: time.Now()}
	s.runs = append(s.runs, r)
	return r.id, nil
}

func (s *retryJobRunStub) Finish(_ context.Context, id uint64, status string, total, failed int, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == 0 || int(id) > len(s.runs) {
		return store.ErrJobRunNotFound
	}
	r := s.runs[id-1]
	r.status, r.total, r.failed, r.errMsg = status, total, failed, errMsg
	return nil
}

func (s *retryJobRunStub) GetLatestPerJob(context.Context) ([]store.JobRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	latest := map[string]*retryRun{}
	for _, r := range s.runs {
		latest[r.job] = r
	}
	out := make([]store.JobRun, 0, len(latest))
	for _, r := range latest {
		out = append(out, store.JobRun{ID: r.id, JobName: r.job, Status: r.status, StartedAt: r.started})
	}
	return out, nil
}

func (s *retryJobRunStub) GetRecent(context.Context, int) ([]store.JobRun, error) { return nil, nil }
func (s *retryJobRunStub) DeleteBefore(context.Context, time.Time) (int64, error) { return 0, nil }
func (s *retryJobRunStub) AbortRunning(context.Context) (int64, error)            { return 0, nil }

func (s *retryJobRunStub) byJob(job string) []retryRun {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []retryRun
	for _, r := range s.runs {
		if r.job == job {
			out = append(out, *r)
		}
	}
	return out
}

// ── 主檔與對帳的來源 ─────────────────────────────────────────────────

// scriptedSymbolSource 依呼叫次序回 errs[n-1]（nil 或超出清單 → 成功）；第 blockAt 次呼叫會先關閉
// entered、再等 block 關閉（讓「一輪還在跑」穩定可測，⛔ 不靠 sleep）。onCall 在回應之前呼叫
// （例如在一輪中途把時鐘推過午夜）。
type scriptedSymbolSource struct {
	mu      sync.Mutex
	calls   int
	errs    []error
	blockAt int
	entered chan struct{}
	block   chan struct{}
	onCall  func(n int)
}

func (s *scriptedSymbolSource) FetchStockSymbols(context.Context) ([]store.StockSymbol, error) {
	s.mu.Lock()
	s.calls++
	n := s.calls
	var err error
	if n <= len(s.errs) {
		err = s.errs[n-1]
	}
	shouldBlock := s.block != nil && n == s.blockAt
	onCall := s.onCall
	s.mu.Unlock()
	if onCall != nil {
		onCall(n)
	}
	if shouldBlock {
		close(s.entered)
		<-s.block
	}
	if err != nil {
		return nil, err
	}
	return []store.StockSymbol{{Symbol: "2330"}}, nil
}

func (s *scriptedSymbolSource) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

type retryStockRepoStub struct{ store.StockSymbolRepo }

func (retryStockRepoStub) UpsertSnapshot(_ context.Context, symbols []store.StockSymbol, _ time.Time) (store.StockSymbolSyncResult, error) {
	return store.StockSymbolSyncResult{Seen: len(symbols)}, nil
}

// hookReadiness 包住真的 JobRunReadiness（讀 job_runs 的最新一筆），讀完之後呼叫 after——
// 用來控制「對帳讀完前置、還沒登記等待」那一刻的交錯，也可以改寫結果。
type hookReadiness struct {
	inner market.SyncReadinessChecker
	mu    sync.Mutex
	calls int
	after func(n int, ready bool) bool
}

func (h *hookReadiness) StockSymbolSyncSucceededToday(ctx context.Context, now time.Time) (bool, error) {
	ready, err := h.inner.StockSymbolSyncSucceededToday(ctx, now)
	h.mu.Lock()
	h.calls++
	n, after := h.calls, h.after
	h.mu.Unlock()
	if err == nil && after != nil {
		ready = after(n, ready)
	}
	return ready, err
}

func (h *hookReadiness) setAfter(f func(n int, ready bool) bool) {
	h.mu.Lock()
	h.after = f
	h.mu.Unlock()
}

// ── 組裝 ───────────────────────────────────────────────────────────

var retryOn = config.JobRetryConfig{Enabled: true, IntervalMin: 10, MaxReruns: 3, Cutoff: "08:45"}

type retryHarness struct {
	s       *Scheduler
	clock   *fakeRetryClock
	jobRuns *retryJobRunStub
	stock   *scriptedSymbolSource
	ready   *hookReadiness
	listing *delistingSourceStub
	// wg 等連帶觸發的那一輪（它由 spawn 另起 goroutine）。
	wg sync.WaitGroup
}

// newRetryHarness 組一個同時有主檔同步與對帳的 Scheduler；對帳的前置讀的是同一份 job_runs。
func newRetryHarness(t *testing.T, at time.Time, cfg config.JobRetryConfig) *retryHarness {
	t.Helper()
	h := &retryHarness{
		clock:   newFakeRetryClock(at),
		jobRuns: &retryJobRunStub{},
		stock:   &scriptedSymbolSource{},
		listing: &delistingSourceStub{result: okSourceResult()},
	}
	h.ready = &hookReadiness{inner: NewJobRunReadiness(h.jobRuns)}
	s := &Scheduler{
		jobRuns: h.jobRuns, log: zap.NewNop(), finishRetryBackoffs: []time.Duration{time.Millisecond},
		cron: cron.New(cron.WithLocation(timeutil.TaipeiTZ)),
	}
	s.stockSyncer = market.NewStockSymbolSyncer(h.stock, retryStockRepoStub{}, zap.NewNop())
	s.SetDelistingReconcile(
		market.NewDelistingReconciler(h.listing, delistingRepoStub{}, h.ready, zap.NewNop()),
		config.DelistingConfig{})
	s.SetJobRetry(cfg)
	s.retry.clock = h.clock
	s.retry.spawn = func(f func()) {
		h.wg.Add(1)
		go func() {
			defer h.wg.Done()
			f()
		}()
	}
	h.s = s
	return h
}

func (h *retryHarness) runManualStock(t *testing.T) {
	t.Helper()
	if !h.s.acquireStockSymbolSync() {
		t.Fatal("手動同步取不到所有權")
	}
	h.s.runStockSymbolSyncAndRelease(triggerManual)
}

func (h *retryHarness) chainCount(job string) int {
	h.s.retry.mu.Lock()
	defer h.s.retry.mu.Unlock()
	if c := h.s.retry.chains[job]; c != nil {
		return c.count
	}
	return 0
}

func statusesOf(runs []retryRun) []string {
	out := make([]string, 0, len(runs))
	for _, r := range runs {
		out = append(out, r.status)
	}
	return out
}

// waitUntil 輪詢到條件成立（手動入口自己 spawn 的 goroutine 沒有別的辦法等）。
func waitUntil(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("等不到：%s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

// ── 層一：排、跑、停 ─────────────────────────────────────────────────

// 06:30 cron 暫時性失敗 → error 註明 06:40 重跑 → 06:40 重跑成功 → 當天⛔ 不再排。
func TestJobRetryRerunsTransientFailure(t *testing.T) {
	h := newRetryHarness(t, tpe(6, 30), retryOn)
	h.stock.errs = []error{transientDialErr()}

	h.s.RunStockSymbolSync()

	runs := h.jobRuns.byJob(stockSymbolSyncJob)
	if len(runs) != 1 || runs[0].status != "failed" || !strings.HasSuffix(runs[0].errMsg, "; 自動重跑 1/3（06:40）") {
		t.Fatalf("得到 %+v，期望 failed 且註明 06:40 重跑", runs)
	}
	if got := h.clock.pending(); len(got) != 1 || got[0] != "06:40" {
		t.Fatalf("待跑的重跑 = %v，期望 [06:40]", got)
	}
	if n := h.clock.advanceTo(tpe(6, 39)); n != 0 {
		t.Fatalf("06:39 ⛔ 不該觸發，觸發了 %d 個", n)
	}
	if n := h.clock.advanceTo(tpe(6, 40)); n != 1 {
		t.Fatalf("06:40 應觸發 1 個，觸發了 %d 個", n)
	}
	runs = h.jobRuns.byJob(stockSymbolSyncJob)
	if got := statusesOf(runs); len(got) != 2 || got[1] != "success" {
		t.Fatalf("狀態 = %v，期望 [failed success]", got)
	}
	if got := h.clock.pending(); len(got) != 0 {
		t.Fatalf("成功之後⛔ 不該再排，得到 %v", got)
	}
}

// 第 3 次重跑也失敗 → 註明用完、⛔ 不再排。
func TestJobRetryStopsAfterMaxReruns(t *testing.T) {
	h := newRetryHarness(t, tpe(6, 30), retryOn)
	h.stock.errs = []error{transientDialErr(), transientDialErr(), transientDialErr(), transientDialErr()}

	h.s.RunStockSymbolSync()
	for _, m := range []int{40, 50} {
		h.clock.advanceTo(tpe(6, m))
	}
	h.clock.advanceTo(tpe(7, 0))

	runs := h.jobRuns.byJob(stockSymbolSyncJob)
	want := []string{"; 自動重跑 1/3（06:40）", "; 自動重跑 2/3（06:50）", "; 自動重跑 3/3（07:00）", "; 自動重跑已用完（3/3）"}
	if len(runs) != len(want) {
		t.Fatalf("得到 %d 筆，期望 %d 筆：%+v", len(runs), len(want), runs)
	}
	for i, r := range runs {
		if r.status != "failed" || !strings.HasSuffix(r.errMsg, want[i]) {
			t.Fatalf("第 %d 筆 = %s %q，期望 failed、結尾 %q", i+1, r.status, r.errMsg, want[i])
		}
	}
	if got := h.clock.pending(); len(got) != 0 {
		t.Fatalf("用完之後⛔ 不該再排，得到 %v", got)
	}
}

// 只有「暫時性 ＋ 自動發起」的失敗才排；永久錯誤、手動觸發、開關關閉都⛔ 不排。
func TestJobRetryOnlyTransientAutomaticFailures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		cfg    config.JobRetryConfig
		err    error
		manual bool
	}{
		{"永久錯誤", retryOn, errRetryPermanent, false},
		{"手動觸發的失敗", retryOn, transientDialErr(), true},
		{"開關關閉", config.JobRetryConfig{Enabled: false}, transientDialErr(), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newRetryHarness(t, tpe(6, 30), tc.cfg)
			h.stock.errs = []error{tc.err}
			if tc.manual {
				h.runManualStock(t)
			} else {
				h.s.RunStockSymbolSync()
			}
			runs := h.jobRuns.byJob(stockSymbolSyncJob)
			if len(runs) != 1 || runs[0].status != "failed" || strings.Contains(runs[0].errMsg, "自動重跑") {
				t.Fatalf("得到 %+v，期望 failed 且⛔ 沒有重跑的註記", runs)
			}
			if got := h.clock.pending(); len(got) != 0 {
				t.Fatalf("⛔ 不該排重跑，得到 %v", got)
			}
		})
	}
}

// 最晚起跑時刻 ＝ 截止 − 業務預算 − 3 分鐘：三個工作各一，含可設定的截止與公司行動可調的預算。
func TestJobRetryLatestStart(t *testing.T) {
	cutoff0930 := retryOn
	cutoff0930.Cutoff = "09:30"
	for _, tc := range []struct {
		name       string
		job        string
		cfg        config.JobRetryConfig
		corpBudget int
		failAt     time.Time
		want       string
	}{
		{"主檔 08:12 失敗 → 08:22 起跑（等於最晚，照排）", stockSymbolSyncJob, retryOn, 0, tpe(8, 12), "; 自動重跑 1/3（08:22）"},
		{"主檔 08:13 失敗 → 08:23 晚於最晚 08:22", stockSymbolSyncJob, retryOn, 0, tpe(8, 13), "; 自動重跑已停止（最晚起跑 08:22）"},
		{"對帳 08:27 失敗 → 08:37 起跑", delistingReconcileJobName, retryOn, 0, tpe(8, 27), "; 自動重跑 1/3（08:37）"},
		{"對帳 08:28 失敗 → 晚於最晚 08:37", delistingReconcileJobName, retryOn, 0, tpe(8, 28), "; 自動重跑已停止（最晚起跑 08:37）"},
		{"公司行動 07:47 失敗 → 07:57 起跑", corporateActionSyncJob, retryOn, 0, tpe(7, 47), "; 自動重跑 1/3（07:57）"},
		{"公司行動 07:48 失敗 → 晚於最晚 07:57", corporateActionSyncJob, retryOn, 0, tpe(7, 48), "; 自動重跑已停止（最晚起跑 07:57）"},
		{"公司行動的預算改成 30 分鐘 → 最晚 08:12", corporateActionSyncJob, retryOn, 1800, tpe(8, 3), "; 自動重跑已停止（最晚起跑 08:12）"},
		{"公司行動的預算改成 30 分鐘 → 08:02 失敗照排", corporateActionSyncJob, retryOn, 1800, tpe(8, 2), "; 自動重跑 1/3（08:12）"},
		{"截止改成 09:30 → 主檔 08:57 失敗照排", stockSymbolSyncJob, cutoff0930, 0, tpe(8, 57), "; 自動重跑 1/3（09:07）"},
		{"截止改成 09:30 → 主檔最晚 09:07", stockSymbolSyncJob, cutoff0930, 0, tpe(8, 58), "; 自動重跑已停止（最晚起跑 09:07）"},
		{"跨日⛔ 不排", stockSymbolSyncJob, retryOn, 0, tpe(23, 55), "; 自動重跑已停止（最晚起跑 08:22）"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &Scheduler{log: zap.NewNop()}
			s.corporateActionCfg.TimeoutSec = tc.corpBudget
			s.SetJobRetry(tc.cfg)
			s.retry.clock = newFakeRetryClock(tc.failAt)
			if got := s.retry.planRerun(tc.job, triggerCron, true, tc.failAt); got != tc.want {
				t.Fatalf("planRerun = %q，期望 %q", got, tc.want)
			}
		})
	}
}

// 整輪的上限要放得進最晚起跑時刻留的 3 分鐘：startRun 10 秒 ＋ 結束寫入最壞 133 秒。
func TestJobRetryRecordMarginCoversRecordWrites(t *testing.T) {
	worst := startRunWriteTimeout + time.Duration(len(defaultFinishRetryBackoffs)+1)*finishRunWriteTimeout
	for _, d := range defaultFinishRetryBackoffs {
		worst += d
	}
	if worst >= jobRetryRecordMargin {
		t.Fatalf("紀錄寫入最壞 %v，⛔ 不能超過最晚起跑留的 %v", worst, jobRetryRecordMargin)
	}
}

// 計畫書的核心例子：06:30 失敗 → 06:35 手動成功 → 06:40 的重跑⛔ 不執行；之後 cron 又失敗也⛔ 不再排。
func TestJobRetrySuccessInvalidatesPendingRerun(t *testing.T) {
	h := newRetryHarness(t, tpe(6, 30), retryOn)
	h.stock.errs = []error{transientDialErr(), nil, transientDialErr()}

	h.s.RunStockSymbolSync()
	h.clock.advanceTo(tpe(6, 35))
	h.runManualStock(t)
	if n := h.clock.advanceTo(tpe(6, 40)); n != 0 {
		t.Fatalf("06:40 的重跑⛔ 不該執行，觸發了 %d 個", n)
	}
	if got := h.stock.callCount(); got != 2 {
		t.Fatalf("主檔抓了 %d 次，期望 2 次（06:30、06:35）", got)
	}

	h.clock.advanceTo(tpe(7, 0))
	h.s.RunStockSymbolSync()
	runs := h.jobRuns.byJob(stockSymbolSyncJob)
	if last := runs[len(runs)-1]; last.status != "failed" || strings.Contains(last.errMsg, "自動重跑") {
		t.Fatalf("當天成功過之後⛔ 不再排，得到 %+v", last)
	}
	if got := h.clock.pending(); len(got) != 0 {
		t.Fatalf("⛔ 不該排重跑，得到 %v", got)
	}
}

// 換日重置：前一天用完 3 次，隔天 06:30 的失敗重新從 1/3 算。
func TestJobRetryChainResetsNextDay(t *testing.T) {
	h := newRetryHarness(t, tpeOn(2, 6, 30), retryOn)
	h.stock.errs = []error{transientDialErr(), transientDialErr(), transientDialErr(), transientDialErr(), transientDialErr()}
	h.s.RunStockSymbolSync()
	h.clock.advanceTo(tpeOn(2, 6, 40))
	h.clock.advanceTo(tpeOn(2, 6, 50))
	h.clock.advanceTo(tpeOn(2, 7, 0))

	h.clock.advanceTo(tpeOn(3, 6, 30))
	h.s.RunStockSymbolSync()
	runs := h.jobRuns.byJob(stockSymbolSyncJob)
	if last := runs[len(runs)-1]; !strings.HasSuffix(last.errMsg, "; 自動重跑 1/3（06:40）") {
		t.Fatalf("隔天應重新從 1/3 起算，得到 %q", last.errMsg)
	}
}

// ── 一輪算哪一天看它的起跑時刻（review R1）──────────────────────────────

// 前一天 23:55 起跑、隔天 00:05 才成功 → ⛔ 不能把隔天標成已成功：隔天 06:30 的暫時性失敗照排重跑。
func TestJobRetryCrossDaySuccessDoesNotSuppressNextDay(t *testing.T) {
	h := newRetryHarness(t, tpeOn(2, 23, 55), retryOn)
	h.stock.errs = []error{nil, transientDialErr()}
	h.stock.onCall = func(n int) {
		if n == 1 {
			h.clock.advanceTo(tpeOn(3, 0, 5))
		}
	}
	h.s.RunStockSymbolSync()

	h.clock.advanceTo(tpeOn(3, 6, 30))
	h.s.RunStockSymbolSync()

	runs := h.jobRuns.byJob(stockSymbolSyncJob)
	if got := statusesOf(runs); len(got) != 2 || got[0] != "success" || got[1] != "failed" {
		t.Fatalf("狀態 = %v，期望 [success failed]", got)
	}
	if !strings.HasSuffix(runs[1].errMsg, "; 自動重跑 1/3（06:40）") {
		t.Fatalf("隔天 06:30 的失敗應排重跑，得到 %q", runs[1].errMsg)
	}
}

// 前一天 23:55 起跑、隔天 00:05 才失敗 → ⛔ 不排（重跑時刻跨到起跑的隔天）、⛔ 不佔隔天的額度。
func TestJobRetryCrossDayFailureIsNotRerun(t *testing.T) {
	h := newRetryHarness(t, tpeOn(2, 23, 55), retryOn)
	h.stock.errs = []error{transientDialErr(), transientDialErr()}
	h.stock.onCall = func(n int) {
		if n == 1 {
			h.clock.advanceTo(tpeOn(3, 0, 5))
		}
	}
	h.s.RunStockSymbolSync()

	runs := h.jobRuns.byJob(stockSymbolSyncJob)
	if len(runs) != 1 || !strings.HasSuffix(runs[0].errMsg, "; 自動重跑已停止（最晚起跑 08:22）") {
		t.Fatalf("得到 %+v，期望註明已停止（起跑那天的最晚 08:22）", runs)
	}
	if got := h.clock.pending(); len(got) != 0 {
		t.Fatalf("⛔ 不該排重跑，得到 %v", got)
	}
	if got := h.chainCount(stockSymbolSyncJob); got != 0 {
		t.Fatalf("⛔ 不該佔隔天的額度：count = %d", got)
	}
	h.clock.advanceTo(tpeOn(3, 6, 30))
	h.s.RunStockSymbolSync()
	runs = h.jobRuns.byJob(stockSymbolSyncJob)
	if last := runs[len(runs)-1]; !strings.HasSuffix(last.errMsg, "; 自動重跑 1/3（06:40）") {
		t.Fatalf("隔天應從 1/3 起算，得到 %q", last.errMsg)
	}
}

// 前一天起跑、隔天才落盤的主檔 success 過不了隔天對帳的前置（要求起跑於今天）→ ⛔ 不觸發在等的對帳。
func TestDelistingLinkIgnoresCrossDayStockSuccess(t *testing.T) {
	h := newRetryHarness(t, tpeOn(2, 23, 55), retryOn)
	h.stock.onCall = func(n int) {
		if n == 1 {
			h.clock.advanceTo(tpeOn(3, 0, 1))
			h.s.RunDelistingReconcile() // 主檔還在 running → 前置不成立 → 隔天登記等待
			h.clock.advanceTo(tpeOn(3, 0, 5))
		}
	}
	h.s.RunStockSymbolSync()
	h.wg.Wait()

	if got := len(h.jobRuns.byJob(delistingReconcileJobName)); got != 1 {
		t.Fatalf("跨日落盤的主檔 success ⛔ 不該觸發對帳：對帳 %d 筆", got)
	}
	h.s.retry.mu.Lock()
	written, gen := h.s.retry.link.stockWritten, h.s.retry.link.stockGen
	h.s.retry.mu.Unlock()
	if written || gen != 0 {
		t.Fatalf("旗標 = %v、代號 = %d，期望 false、0", written, gen)
	}
}

// 前一天起跑的對帳隔天才因前置不成立結束 → ⛔ 不在隔天登記等待：隔天主檔成功也⛔ 不連帶觸發。
func TestDelistingLinkIgnoresCrossDayNotReady(t *testing.T) {
	h := newRetryHarness(t, tpeOn(2, 23, 58), retryOn)
	h.ready.setAfter(func(n int, ready bool) bool {
		if n == 1 {
			h.clock.advanceTo(tpeOn(3, 0, 2))
		}
		return ready
	})
	h.s.RunDelistingReconcile()

	h.clock.advanceTo(tpeOn(3, 6, 30))
	h.runManualStock(t)
	h.wg.Wait()

	if got := len(h.jobRuns.byJob(delistingReconcileJobName)); got != 1 {
		t.Fatalf("前一天的前置不成立⛔ 不該在隔天登記等待：對帳 %d 筆", got)
	}
}

// 計時器到期時已換日（例如 process 被暫停過）：⛔ 不讓重跑跨日。
func TestJobRetryTimerFiringOnAnotherDayIsCancelled(t *testing.T) {
	h := newRetryHarness(t, tpeOn(2, 6, 30), retryOn)
	h.stock.errs = []error{transientDialErr()}
	h.s.RunStockSymbolSync()

	if n := h.clock.advanceTo(tpeOn(3, 6, 40)); n != 1 {
		t.Fatalf("計時器應到期 1 個，得到 %d", n)
	}
	if got := h.stock.callCount(); got != 1 {
		t.Fatalf("隔天⛔ 不該執行前一天的重跑，主檔抓了 %d 次", got)
	}
}

// 計時器晚到、已超過最晚起跑時刻：⛔ 不跑（整輪要在截止前結束）。
func TestJobRetryTimerFiringPastLatestStartIsCancelled(t *testing.T) {
	h := newRetryHarness(t, tpe(8, 12), retryOn)
	h.stock.errs = []error{transientDialErr()}
	h.s.RunStockSymbolSync()

	if n := h.clock.advanceTo(tpe(8, 30)); n != 1 {
		t.Fatalf("計時器應到期 1 個，得到 %d", n)
	}
	if got := h.stock.callCount(); got != 1 {
		t.Fatalf("超過最晚起跑 08:22 ⛔ 不該執行，主檔抓了 %d 次", got)
	}
}

// Stop() 取消全部待跑的重跑。
func TestJobRetryStopCancelsPendingReruns(t *testing.T) {
	h := newRetryHarness(t, tpe(6, 30), retryOn)
	h.stock.errs = []error{transientDialErr()}
	h.s.RunStockSymbolSync()
	// 對帳的前置讀成成立、抓取暫時性失敗 → 對帳也排了一個。
	h.ready.setAfter(func(int, bool) bool { return true })
	h.listing.err = transientDialErr()
	h.s.RunDelistingReconcile()
	if got := h.clock.pending(); len(got) != 2 {
		t.Fatalf("應有 2 個待跑的重跑，得到 %v", got)
	}

	h.s.Stop()

	if got := h.clock.pending(); len(got) != 0 {
		t.Fatalf("Stop() 之後⛔ 不該有待跑的重跑，得到 %v", got)
	}
	if n := h.clock.advanceTo(tpe(6, 50)); n != 0 {
		t.Fatalf("Stop() 之後⛔ 不該觸發，觸發了 %d 個", n)
	}
	if got := h.s.retry.planRerun(stockSymbolSyncJob, triggerCron, true, h.clock.Now()); got != "" {
		t.Fatalf("Stop() 之後⛔ 不再排，得到 %q", got)
	}
}

// ── 執行所有權（D5 (b)）：兩種起跑順序 ───────────────────────────────

// 重跑先開始：手動拿不到所有權（handler 回 409）、cron 撞上只記 Warn、⛔ 不寫 job_runs。
func TestStockSymbolSyncOwnershipRerunFirst(t *testing.T) {
	h := newRetryHarness(t, tpe(6, 30), retryOn)
	h.stock.errs = []error{transientDialErr()}
	h.stock.blockAt, h.stock.entered, h.stock.block = 2, make(chan struct{}), make(chan struct{})
	h.s.RunStockSymbolSync()

	done := make(chan struct{})
	go func() {
		defer close(done)
		h.clock.advanceTo(tpe(6, 40))
	}()
	<-h.stock.entered

	if h.s.TryStartStockSymbolSync() {
		t.Fatal("重跑執行中，手動觸發應被擋下")
	}
	h.s.RunStockSymbolSync()
	if got := len(h.jobRuns.byJob(stockSymbolSyncJob)); got != 2 {
		t.Fatalf("cron 撞上⛔ 不寫 job_runs：得到 %d 筆，期望 2 筆", got)
	}

	close(h.stock.block)
	<-done
	if got := statusesOf(h.jobRuns.byJob(stockSymbolSyncJob)); len(got) != 2 || got[1] != "success" {
		t.Fatalf("狀態 = %v，期望 [failed success]", got)
	}
}

// 手動先開始：重跑到期時取不到所有權 → 取消、這條鏈結束；手動那輪的失敗⛔ 不再排。
func TestStockSymbolSyncOwnershipManualFirst(t *testing.T) {
	h := newRetryHarness(t, tpe(6, 30), retryOn)
	h.stock.errs = []error{transientDialErr(), transientDialErr()}
	h.stock.blockAt, h.stock.entered, h.stock.block = 2, make(chan struct{}), make(chan struct{})
	h.s.RunStockSymbolSync()

	h.clock.advanceTo(tpe(6, 35))
	if !h.s.TryStartStockSymbolSync() {
		t.Fatal("手動觸發應取得所有權")
	}
	<-h.stock.entered
	if n := h.clock.advanceTo(tpe(6, 40)); n != 1 {
		t.Fatalf("06:40 的計時器應到期，得到 %d", n)
	}
	if got := len(h.jobRuns.byJob(stockSymbolSyncJob)); got != 2 {
		t.Fatalf("重跑撞上手動應取消：得到 %d 筆，期望 2 筆", got)
	}

	close(h.stock.block)
	waitUntil(t, "手動那輪結束", func() bool {
		runs := h.jobRuns.byJob(stockSymbolSyncJob)
		return len(runs) == 2 && runs[1].status != "running" && !h.s.stockSyncRunning.Load()
	})
	runs := h.jobRuns.byJob(stockSymbolSyncJob)
	if runs[1].status != "failed" || strings.Contains(runs[1].errMsg, "自動重跑") {
		t.Fatalf("手動那輪 = %+v，期望 failed 且⛔ 沒有重跑的註記", runs[1])
	}
	if got := h.clock.pending(); len(got) != 0 {
		t.Fatalf("鏈已結束，⛔ 不該再排，得到 %v", got)
	}
}

// ── 連帶重跑（「3.2」規則 4） ─────────────────────────────────────────

// 對帳在等主檔 → 主檔成功落盤 → 對帳連帶執行一次，計入對帳自己的每日上限。
func TestDelistingLinkedRerunAfterStockSymbolSyncSucceeds(t *testing.T) {
	h := newRetryHarness(t, tpe(6, 55), retryOn)
	h.stock.errs = []error{transientDialErr()}
	h.s.RunStockSymbolSync() // 06:55 失敗 → 07:05 重跑
	h.clock.advanceTo(tpe(7, 0))
	h.s.RunDelistingReconcile() // 07:00 前置不成立 → 登記等待

	runs := h.jobRuns.byJob(delistingReconcileJobName)
	if len(runs) != 1 || runs[0].errMsg != delistingNotReadySummary {
		t.Fatalf("對帳 = %+v，期望一筆前置不成立、⛔ 沒有自己的重跑註記", runs)
	}
	if got := h.clock.pending(); len(got) != 1 || got[0] != "07:05" {
		t.Fatalf("只有主檔的重跑在等，得到 %v", got)
	}

	h.clock.advanceTo(tpe(7, 5))
	h.wg.Wait()

	if got := statusesOf(h.jobRuns.byJob(delistingReconcileJobName)); len(got) != 2 || got[1] != "success" {
		t.Fatalf("對帳狀態 = %v，期望 [failed success]", got)
	}
	if got := h.chainCount(delistingReconcileJobName); got != 1 {
		t.Fatalf("連帶觸發要計入對帳的上限：count = %d，期望 1", got)
	}
}

// 對帳的額度用完、或過了對帳的最晚起跑時刻：⛔ 不觸發。
func TestDelistingLinkedRerunRespectsQuotaAndLatestStart(t *testing.T) {
	for _, tc := range []struct {
		name      string
		successAt time.Time
		used      int
		want      int // 對帳的筆數
	}{
		{"08:37 主檔成功（等於最晚）照觸發", tpe(8, 37), 0, 2},
		{"08:38 主檔成功：晚於對帳的最晚起跑 08:37", tpe(8, 38), 0, 1},
		{"對帳的額度已用完", tpe(7, 10), 3, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newRetryHarness(t, tpe(7, 0), retryOn)
			h.s.RunDelistingReconcile() // 主檔今天還沒跑 → 前置不成立 → 登記等待
			h.s.retry.mu.Lock()
			h.s.retry.chainLocked(delistingReconcileJobName, h.clock.Now()).count = tc.used
			h.s.retry.mu.Unlock()

			h.clock.advanceTo(tc.successAt)
			h.runManualStock(t)
			h.wg.Wait()

			if got := len(h.jobRuns.byJob(delistingReconcileJobName)); got != tc.want {
				t.Fatalf("對帳 %d 筆，期望 %d 筆", got, tc.want)
			}
		})
	}
}

// (i) 對帳讀到主檔還在 running → 主檔成功並通知 → 對帳才登記等待 → 仍會觸發，而且只觸發一次。
func TestDelistingLinkRechecksWhenRegisteringWait(t *testing.T) {
	h := newRetryHarness(t, tpe(7, 0), retryOn)
	h.stock.blockAt, h.stock.entered, h.stock.block = 1, make(chan struct{}), make(chan struct{})
	stockDone := make(chan struct{})
	go func() {
		defer close(stockDone)
		h.s.RunStockSymbolSync()
	}()
	<-h.stock.entered // 主檔這一輪已寫了 running

	h.ready.setAfter(func(n int, ready bool) bool {
		if n == 1 {
			// 對帳已讀到「主檔還在 running」；在它登記等待之前，主檔成功並通知。
			close(h.stock.block)
			<-stockDone
		}
		return ready
	})
	h.s.RunDelistingReconcile()
	h.wg.Wait()

	if got := statusesOf(h.jobRuns.byJob(delistingReconcileJobName)); len(got) != 2 || got[0] != "failed" || got[1] != "success" {
		t.Fatalf("對帳狀態 = %v，期望 [failed success]（觸發一次）", got)
	}
}

// (ii) 主檔成功通知時對帳還持有所有權 → 觸發被擋、待觸發留著 → 對帳釋放所有權後補觸發一次。
func TestDelistingLinkRetriesAfterOwnershipReleased(t *testing.T) {
	h := newRetryHarness(t, tpe(6, 55), retryOn)
	h.stock.errs = []error{errRetryPermanent}
	h.s.RunStockSymbolSync() // 06:55 主檔失敗（永久錯誤，⛔ 沒有自己的重跑）
	h.clock.advanceTo(tpe(7, 0))
	h.s.RunDelistingReconcile() // cron：前置不成立 → 登記等待

	// 07:05 手動對帳：讀完前置（仍不成立）之後卡住、佔著所有權。
	h.clock.advanceTo(tpe(7, 5))
	entered, release := make(chan struct{}), make(chan struct{})
	h.ready.setAfter(func(n int, ready bool) bool {
		if n == 2 {
			close(entered)
			<-release
		}
		return ready
	})
	if !h.s.acquireDelistingReconcile() {
		t.Fatal("手動對帳取不到所有權")
	}
	manualDone := make(chan struct{})
	go func() {
		defer close(manualDone)
		h.s.runDelistingReconcileAndRelease(triggerManual)
	}()
	<-entered

	h.s.RunStockSymbolSync() // 主檔成功：通知時對帳還持有所有權
	if got := len(h.jobRuns.byJob(delistingReconcileJobName)); got != 2 {
		t.Fatalf("對帳持有所有權時⛔ 不該多一輪：得到 %d 筆", got)
	}

	close(release)
	<-manualDone
	h.wg.Wait()

	if got := statusesOf(h.jobRuns.byJob(delistingReconcileJobName)); len(got) != 3 || got[2] != "success" {
		t.Fatalf("對帳狀態 = %v，期望 [failed failed success]（釋放後補觸發一次）", got)
	}
}

// (iv) 旗標是「最新一輪已成功落盤」、⛔ 不是「今天曾成功」：
// 06:30 主檔成功 → 06:55 手動同步開始並失敗 → 07:00 對帳前置不成立 → ⛔ 不觸發；之後主檔再成功一次 → 觸發一次。
func TestDelistingLinkFlagIsLatestRound(t *testing.T) {
	h := newRetryHarness(t, tpe(6, 30), retryOn)
	h.stock.errs = []error{nil, errRetryPermanent}
	h.s.RunStockSymbolSync()
	h.clock.advanceTo(tpe(6, 55))
	h.runManualStock(t)
	h.clock.advanceTo(tpe(7, 0))
	h.s.RunDelistingReconcile()
	h.wg.Wait()

	if got := len(h.jobRuns.byJob(delistingReconcileJobName)); got != 1 {
		t.Fatalf("主檔最新一輪是失敗，⛔ 不該觸發：對帳 %d 筆", got)
	}

	h.clock.advanceTo(tpe(7, 10))
	h.runManualStock(t)
	h.wg.Wait()
	if got := statusesOf(h.jobRuns.byJob(delistingReconcileJobName)); len(got) != 2 || got[1] != "success" {
		t.Fatalf("對帳狀態 = %v，期望 [failed success]", got)
	}
}

// 主檔同步成功、但結束紀錄寫不進去（開始紀錄就沒拿到）→ 前置條件讀不到 success → ⛔ 不觸發對帳：
// 旗標要的是「success 已落盤」，⛔ 不是「同步本身成功」。
func TestDelistingLinkRequiresStockSuccessWritten(t *testing.T) {
	h := newRetryHarness(t, tpe(7, 0), retryOn)
	h.s.RunDelistingReconcile() // 登記等待
	h.jobRuns.mu.Lock()
	h.jobRuns.failStartJob = stockSymbolSyncJob
	h.jobRuns.mu.Unlock()

	h.clock.advanceTo(tpe(7, 5))
	h.runManualStock(t)
	h.wg.Wait()

	if got := len(h.jobRuns.byJob(delistingReconcileJobName)); got != 1 {
		t.Fatalf("主檔的 success 沒有落盤，⛔ 不該觸發：對帳 %d 筆", got)
	}
}

// (v) 同一個代號最多觸發一次：連帶觸發的對帳又因前置不成立失敗 → ⛔ 不用同一個代號再觸發、⛔ 不耗盡額度。
func TestDelistingLinkUsesEachStockSuccessOnce(t *testing.T) {
	h := newRetryHarness(t, tpe(6, 55), retryOn)
	h.stock.errs = []error{errRetryPermanent}
	h.s.RunStockSymbolSync()
	h.clock.advanceTo(tpe(7, 0))
	h.s.RunDelistingReconcile() // 登記等待
	// 之後的前置一律讀成不成立（例如 job_runs 與記憶體裡的旗標對不上）。
	h.ready.setAfter(func(int, bool) bool { return false })

	h.clock.advanceTo(tpe(7, 5))
	h.s.RunStockSymbolSync() // 主檔成功（代號 1）→ 連帶觸發 → 又前置不成立
	h.wg.Wait()

	if got := len(h.jobRuns.byJob(delistingReconcileJobName)); got != 2 {
		t.Fatalf("同一個代號只能觸發一次：對帳 %d 筆，期望 2 筆", got)
	}
	if got := h.chainCount(delistingReconcileJobName); got != 1 {
		t.Fatalf("對帳的 count = %d，期望 1", got)
	}
}

// ── 公司行動：層一只管整輪早退，⛔ 不是自己的預算用完 ───────────────────

type scriptedSplitSource struct {
	mu    sync.Mutex
	calls int
	errs  []error
	// waitCtx 為真時等 ctx 結束，再回 http client 被取消時的 url.Error（帶 Timeout()）。
	waitCtx bool
}

func (s *scriptedSplitSource) FetchSplitPrices(ctx context.Context, _, _ time.Time) ([]store.CorporateAction, error) {
	s.mu.Lock()
	s.calls++
	n := s.calls
	s.mu.Unlock()
	if s.waitCtx {
		<-ctx.Done()
		return nil, &url.Error{Op: "Get", URL: "https://api.finmindtrade.com/x", Err: ctx.Err()}
	}
	if n <= len(s.errs) && s.errs[n-1] != nil {
		return nil, s.errs[n-1]
	}
	return nil, nil
}

func newCorporateRetryScheduler(
	t *testing.T, split market.SplitSource, div market.DividendSource, symbols []string,
) (*Scheduler, *schedulerJobRunRepoStub, *fakeRetryClock) {
	t.Helper()
	jobRuns := &schedulerJobRunRepoStub{}
	adj := market.NewAdjuster(split, schedulerActionRepoStub{}, &schedulerAdjusterCandleStub{symbols: symbols}, zap.NewNop())
	adj.SetDividendSource(div)
	s := &Scheduler{jobRuns: jobRuns, adjuster: adj, log: zap.NewNop(), finishRetryBackoffs: []time.Duration{time.Millisecond}}
	s.corporateActionCfg = config.CorporateActionConfig{ShardCount: 1}
	s.SetJobRetry(retryOn)
	clock := newFakeRetryClock(tpe(6, 30))
	clock.autoFire = func(d time.Duration) bool { return d == corporateRetryWait }
	s.retry.clock = clock
	return s, jobRuns, clock
}

func TestCorporateActionSyncRerunsWholeRunEarlyExit(t *testing.T) {
	split := &scriptedSplitSource{errs: []error{transientDialErr()}}
	s, jobRuns, clock := newCorporateRetryScheduler(t, split, &scriptedDividendSource{}, []string{"2330"})

	s.RunCorporateActionSync()
	got := jobRuns.snapshotFinished()
	if len(got) != 1 || got[0].status != "failed" || !strings.HasSuffix(got[0].errMsg, "; 自動重跑 1/3（06:40）") {
		t.Fatalf("得到 %+v，期望分割批次失敗、註明 06:40 重跑", got)
	}
	clock.advanceTo(tpe(6, 40))
	if got := jobRuns.snapshotFinished(); len(got) != 2 || got[1].status != "success" {
		t.Fatalf("得到 %+v，期望重跑成功", got)
	}
}

// 整輪的預算用完（http client 被取消也帶 Timeout()）⛔ 不算暫時性：⛔ 不排重跑。
func TestCorporateActionSyncOwnBudgetExhaustedIsNotRerun(t *testing.T) {
	split := &scriptedSplitSource{waitCtx: true}
	s, jobRuns, clock := newCorporateRetryScheduler(t, split, &scriptedDividendSource{}, []string{"2330"})
	s.corporateActionCfg.TimeoutSec = 1

	s.RunCorporateActionSync()

	got := jobRuns.snapshotFinished()
	if len(got) != 1 || got[0].status != "failed" || strings.Contains(got[0].errMsg, "自動重跑") {
		t.Fatalf("得到 %+v，期望 failed 且⛔ 沒有重跑的註記", got)
	}
	if p := clock.pending(); len(p) != 0 {
		t.Fatalf("⛔ 不該排重跑，得到 %v", p)
	}
}

// 06:30 早退 → 06:35 手動跑完（partial）→ 06:40 的重跑⛔ 不執行：整輪有跑完就算成功。
func TestCorporateActionSyncPartialInvalidatesPendingRerun(t *testing.T) {
	split := &scriptedSplitSource{errs: []error{transientDialErr()}}
	div := &scriptedDividendSource{script: map[string][]error{"8088": {&market.HTTPStatusError{Code: 404}}}}
	s, jobRuns, clock := newCorporateRetryScheduler(t, split, div, []string{"2330", "8088"})

	s.RunCorporateActionSync()
	clock.advanceTo(tpe(6, 35))
	if !s.acquireCorporateActionSync() {
		t.Fatal("取不到所有權")
	}
	s.runCorporateActionSyncAndRelease(triggerManual)
	if got := jobRuns.snapshotFinished(); len(got) != 2 || got[1].status != "partial" {
		t.Fatalf("得到 %+v，期望手動那輪 partial", got)
	}
	if n := clock.advanceTo(tpe(6, 40)); n != 0 {
		t.Fatalf("06:40 的重跑⛔ 不該執行，觸發了 %d 個", n)
	}
}

// cron 撞上手動正在跑的那一輪：只記 Warn、⛔ 不寫 job_runs；手動也拿不到所有權。
func TestCorporateActionSyncOwnership(t *testing.T) {
	split := &blockingSplitSource{entered: make(chan struct{}), block: make(chan struct{})}
	s, jobRuns, _ := newCorporateRetryScheduler(t, split, &scriptedDividendSource{}, []string{"2330"})

	done := make(chan struct{})
	go func() {
		defer close(done)
		s.RunCorporateActionSync()
	}()
	<-split.entered
	if s.TryStartCorporateActionSync() {
		t.Fatal("已有一輪在跑，手動觸發應被擋下")
	}
	s.RunCorporateActionSync()
	close(split.block)
	<-done
	if got := jobRuns.snapshotFinished(); len(got) != 1 {
		t.Fatalf("撞上的那一輪⛔ 不寫 job_runs：得到 %+v", got)
	}
}

type blockingSplitSource struct {
	entered chan struct{}
	block   chan struct{}
}

func (s *blockingSplitSource) FetchSplitPrices(context.Context, time.Time, time.Time) ([]store.CorporateAction, error) {
	close(s.entered)
	<-s.block
	return nil, nil
}

// ── 層三：公司行動失敗標的的第二輪 ────────────────────────────────────

// scriptedDividendSource 依「這一檔的第幾次呼叫」回 script[symbol][n-1]（nil 或超出 → 成功、沒有事件）；
// onCall 在回應之前呼叫。
type scriptedDividendSource struct {
	mu     sync.Mutex
	calls  map[string]int
	script map[string][]error
	onCall func(symbol string, n int)
	// events 是成功時回的事件（nil → 沒有事件）。
	events map[string][]store.CorporateAction
}

func (s *scriptedDividendSource) FetchDividends(ctx context.Context, symbol string) ([]store.CorporateAction, error) {
	s.mu.Lock()
	if s.calls == nil {
		s.calls = map[string]int{}
	}
	s.calls[symbol]++
	n := s.calls[symbol]
	var err error
	if errs := s.script[symbol]; n <= len(errs) {
		err = errs[n-1]
	}
	onCall := s.onCall
	events := s.events[symbol]
	s.mu.Unlock()
	if onCall != nil {
		onCall(symbol, n)
	}
	if err != nil {
		return nil, err
	}
	// 比照真的 http client：ctx 已經結束就失敗。
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr
	}
	return events, nil
}

func (s *scriptedDividendSource) callsOf(symbol string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls[symbol]
}

// 暫時性失敗的標的在第二輪成功 → 從失敗移除；404 ⛔ 不重試。
func TestCorporateActionRetriesTransientSymbols(t *testing.T) {
	div := &scriptedDividendSource{script: map[string][]error{
		"2330": {transientDialErr()},
		"8088": {&market.HTTPStatusError{Code: 404}},
	}}
	s, jobRuns, clock := newCorporateRetryScheduler(t, schedulerSplitSourceStub{}, div, []string{"0050", "2330", "8088"})

	s.RunCorporateActionSync()

	got := jobRuns.snapshotFinished()
	if len(got) != 1 || got[0].status != "partial" || got[0].symbolsFailed != 1 {
		t.Fatalf("得到 %+v，期望 partial、失敗 1 檔", got)
	}
	if !strings.Contains(got[0].errMsg, "8088") || strings.Contains(got[0].errMsg, "2330") {
		t.Fatalf("error = %q，期望只剩 8088", got[0].errMsg)
	}
	if div.callsOf("2330") != 2 || div.callsOf("8088") != 1 || div.callsOf("0050") != 1 {
		t.Fatalf("呼叫次數 2330=%d 8088=%d 0050=%d，期望 2／1／1", div.callsOf("2330"), div.callsOf("8088"), div.callsOf("0050"))
	}
	if w := clock.waited(); len(w) != 1 || w[0] != corporateRetryWait {
		t.Fatalf("等待 = %v，期望先等 %v", w, corporateRetryWait)
	}
}

// 剩餘預算⛔ 不超過 60 秒：⛔ 不跑第二輪，保留第一輪的失敗。
func TestCorporateActionSkipsRetryWithoutBudget(t *testing.T) {
	div := &scriptedDividendSource{script: map[string][]error{"2330": {transientDialErr()}}}
	s, jobRuns, clock := newCorporateRetryScheduler(t, schedulerSplitSourceStub{}, div, []string{"2330"})
	s.corporateActionCfg.TimeoutSec = 30

	s.RunCorporateActionSync()

	if got := jobRuns.snapshotFinished(); len(got) != 1 || got[0].status != "failed" || got[0].symbolsFailed != 1 {
		t.Fatalf("得到 %+v，期望 failed、失敗 1 檔", got)
	}
	if div.callsOf("2330") != 1 || len(clock.waited()) != 0 {
		t.Fatalf("⛔ 不該跑第二輪：2330 呼叫 %d 次、等待 %v", div.callsOf("2330"), clock.waited())
	}
}

// 第二輪做到一半預算用完：做到一半的記**第二輪**的失敗，沒跑到的保留第一輪的失敗（review R2）。
func TestCorporateActionRetryBudgetRunsOutKeepsOnlyUnreachedFirstFailures(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	div := &scriptedDividendSource{script: map[string][]error{
		"2330": {transientDialErr()},
		"2454": {transientDialErr()},
	}}
	div.onCall = func(symbol string, n int) {
		if symbol == "2330" && n == 2 {
			cancel() // 第二輪輪到 2330 時預算用完
		}
	}
	s, _, _ := newCorporateRetryScheduler(t, schedulerSplitSourceStub{}, div, nil)

	_, failed, failures, err := s.adjuster.SyncPerSymbolEvents(ctx, []string{"2330", "2454"})
	if err != nil || failed != 2 {
		t.Fatalf("第一輪 failed=%d err=%v，期望 2、nil", failed, err)
	}
	gotFailed, merged := s.retryTransientSymbols(ctx, failed, failures)

	if gotFailed != 2 || len(merged) != 2 {
		t.Fatalf("合併後 failed=%d、%d 筆，期望 2、2 筆：%+v", gotFailed, len(merged), merged)
	}
	// 2330 第二輪的配息請求被中止：Transient 為假（ctx 已到期），這是第二輪的失敗。
	if f := merged[0]; f.Symbol != "2330" || f.Stage != market.SyncStageDividends || f.Transient {
		t.Fatalf("2330 應記第二輪的失敗（dividends、⛔ 不是暫時性），得到 %+v", f)
	}
	// 2454 第二輪沒跑到：保留第一輪的失敗（dividends、暫時性）。
	if f := merged[1]; f.Symbol != "2454" || f.Stage != market.SyncStageDividends || !f.Transient {
		t.Fatalf("2454 應保留第一輪的失敗（dividends、暫時性），得到 %+v", f)
	}
	if div.callsOf("2454") != 1 {
		t.Fatalf("預算用完之後⛔ 不該再送 2454：呼叫 %d 次", div.callsOf("2454"))
	}
}

// recomputeFailCandleStub 讓重算（寫還原係數）一律失敗。
type recomputeFailCandleStub struct {
	schedulerAdjusterCandleStub
}

func (*recomputeFailCandleStub) ApplyAdjFactors(context.Context, string, []store.AdjFactorRange) error {
	return errors.New("apply adj factors: db unavailable")
}

// review R2：第一輪配息抓取逾時；第二輪抓取與事件寫入成功、重算失敗 → 紀錄要是第二輪的 recompute，
// ⛔ 不是第一輪的 dividends（事件與係數已經不同步，只留 dividends 看不出來）。
func TestCorporateActionRetryRecordsSecondRoundFailure(t *testing.T) {
	div := &scriptedDividendSource{
		script: map[string][]error{"2330": {transientDialErr()}},
		events: map[string][]store.CorporateAction{"2330": {{
			Symbol: "2330", EventDate: tpe(0, 0), ActionType: "cash_dividend", Factor: 0.98, VolumeFactor: 1, Source: "test",
		}}},
	}
	s, jobRuns, _ := newCorporateRetryScheduler(t, schedulerSplitSourceStub{}, div, nil)
	adj := market.NewAdjuster(schedulerSplitSourceStub{}, schedulerActionRepoStub{},
		&recomputeFailCandleStub{schedulerAdjusterCandleStub{symbols: []string{"2330"}}}, zap.NewNop())
	adj.SetDividendSource(div)
	s.adjuster = adj

	s.RunCorporateActionSync()

	got := jobRuns.snapshotFinished()
	if len(got) != 1 || got[0].symbolsFailed != 1 {
		t.Fatalf("得到 %+v，期望失敗 1 檔", got)
	}
	if msg := got[0].errMsg; !strings.Contains(msg, "recompute_failed") || strings.Contains(msg, "dividends_failed") {
		t.Fatalf("error = %q，期望第二輪的 recompute、⛔ 沒有第一輪的 dividends", msg)
	}
	if div.callsOf("2330") != 2 {
		t.Fatalf("2330 應跑兩輪，呼叫 %d 次", div.callsOf("2330"))
	}
}

// 一檔的任一階段是永久失敗就⛔ 不重試（只挑「所有失敗階段都是暫時性」的）。
func TestTransientOnlySymbols(t *testing.T) {
	got := transientOnlySymbols([]market.SyncFailure{
		{Symbol: "2330", Stage: market.SyncStageDividends, Transient: true},
		{Symbol: "2454", Stage: market.SyncStageDividends, Transient: true},
		{Symbol: "2454", Stage: market.SyncStageReductions, Transient: false},
		{Symbol: "8088", Stage: market.SyncStageDividends, Transient: false},
		{Symbol: "2330", Stage: market.SyncStageReductions, Transient: true},
	})
	if strings.Join(got, ",") != "2330" {
		t.Fatalf("得到 %v，期望 [2330]", got)
	}
}

// ── 設定與啟動 log ─────────────────────────────────────────────────

func TestNormalizeJobRetry(t *testing.T) {
	core, logs := observer.New(zapcore.ErrorLevel)
	got := normalizeJobRetry(config.JobRetryConfig{Enabled: true, IntervalMin: 0, MaxReruns: -1, Cutoff: "8點45"}, zap.New(core))
	if !got.enabled || got.interval != defaultJobRetryInterval || got.maxReruns != defaultJobRetryMaxReruns || got.cutoffString() != "08:45" {
		t.Fatalf("得到 %+v，期望退回預設", got)
	}
	if logs.Len() != 1 {
		t.Fatalf("截止時刻格式錯誤應記 1 筆 Error，得到 %d 筆", logs.Len())
	}

	core, logs = observer.New(zapcore.ErrorLevel)
	got = normalizeJobRetry(config.JobRetryConfig{IntervalMin: 1, MaxReruns: 5, Cutoff: "23:30"}, zap.New(core))
	if got.interval != time.Minute || got.maxReruns != 5 || got.cutoffString() != "23:30" || logs.Len() != 0 {
		t.Fatalf("得到 %+v（Error %d 筆），期望照設定", got, logs.Len())
	}
}

// 啟動 log 要寫出開關、截止時刻與三個工作的最晚起跑時刻（部署後的確認靠它）。
func TestJobRetryStartupLog(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)
	s := &Scheduler{log: zap.New(core)}
	s.SetJobRetry(retryOn)
	s.retry.clock = newFakeRetryClock(tpe(5, 0))

	s.retry.logStartup(s.log)

	entries := logs.FilterMessage("job retry config").All()
	if len(entries) != 1 {
		t.Fatalf("應有 1 筆 job retry config，得到 %d", len(entries))
	}
	fields := entries[0].ContextMap()
	for k, want := range map[string]any{
		"enabled":                            true,
		"cutoff":                             "08:45",
		"latest_start_corporate_action_sync": "07:57",
		"latest_start_stock_symbol_sync":     "08:22",
		"latest_start_delisting_reconcile":   "08:37",
	} {
		if fields[k] != want {
			t.Fatalf("%s = %v，期望 %v", k, fields[k], want)
		}
	}
}
