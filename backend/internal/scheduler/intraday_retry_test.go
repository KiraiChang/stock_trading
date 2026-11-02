package scheduler

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/indicator"
	"github.com/trading/backend/internal/market"
	"github.com/trading/backend/internal/signal"
	"github.com/trading/backend/internal/store"
)

// 層二：intraday 的批次在同一輪內重試（I-123～I-126 計畫書「3.3」「八」）。
// 等待與 120 秒的重試預算都走注入的時鐘：等待立刻觸發，預算到期由測試在指定的那一刻觸發。

type batchAttempt struct {
	symbols []string
	ctxErr  error // 收到時 ctx 的狀態
}

// retryBatchSource 依呼叫次序回 errs[n-1]（nil 或超出 → 成功，每檔一根 1 分 K）；
// onCall 在回應之前呼叫，回非 nil 就當作這次的錯誤。
type retryBatchSource struct {
	mu     sync.Mutex
	calls  []batchAttempt
	errs   []error
	onCall func(ctx context.Context, n int) error
	batch  int
}

func (b *retryBatchSource) FetchIntradayCandlesBatch(ctx context.Context, symbols []string) (map[string][]market.Candle, error) {
	b.mu.Lock()
	b.calls = append(b.calls, batchAttempt{symbols: append([]string(nil), symbols...), ctxErr: ctx.Err()})
	n := len(b.calls)
	var err error
	if n <= len(b.errs) {
		err = b.errs[n-1]
	}
	onCall := b.onCall
	b.mu.Unlock()
	if onCall != nil {
		if e := onCall(ctx, n); e != nil {
			return nil, e
		}
	}
	if err != nil {
		return nil, err
	}
	out := map[string][]market.Candle{}
	for _, s := range symbols {
		out[s] = []market.Candle{{Symbol: s, Timeframe: "1m", Open: 100, High: 100, Low: 100, Close: 100, Volume: 1, Timestamp: tpe(9, 1)}}
	}
	return out, nil
}

func (b *retryBatchSource) RateLimit() int { return 600 }
func (b *retryBatchSource) BatchSize() int { return b.batch }

func (b *retryBatchSource) attempts() []batchAttempt {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]batchAttempt(nil), b.calls...)
}

// retryCandleRepo 是 fetcher 寫 K 棒的那一端；onInsert 可以模擬「寫入途中預算到期」。
type retryCandleRepo struct {
	store.CandleRepo
	onInsert func(ctx context.Context) error
}

func (r *retryCandleRepo) BulkInsert(ctx context.Context, _ []store.Candle) error {
	if r.onInsert != nil {
		return r.onInsert(ctx)
	}
	return nil
}

func newIntradayRetryScheduler(
	t *testing.T, src *retryBatchSource, candles store.CandleRepo, cfg config.JobRetryConfig, symbols []string,
) (*Scheduler, *schedulerJobRunRepoStub, *fakeRetryClock) {
	t.Helper()
	// 評估那一段用平盤資料：算得出指標、⛔ 不觸發訊號，失敗數只來自抓取。
	evalCandles := &evalCandleRepo{candles: flatCandles(120)}
	ind := indicator.NewEngine(evalCandles, evalIndicatorRepo{}, &store.RedisClient{}, zap.NewNop())
	sigEng := signal.NewEngine(evalCandles, nil, &store.RedisClient{}, ind, evalChipRepo{}, zap.NewNop())
	fetcher := market.NewFetcher(nil, candles, zap.NewNop())
	fetcher.SetIntradaySource(src)
	jobRuns := &schedulerJobRunRepoStub{}
	s := New(
		fetcher, sigEng, &schedulerWatchlistStub{symbols: symbols}, jobRuns, nil, nil, nil, "", nil, "", false,
		nil, nil, nil, nil, config.SREvaluationConfig{}, false, zap.NewNop(),
	)
	s.finishRetryBackoffs = []time.Duration{time.Millisecond}
	s.SetJobRetry(cfg)
	clock := newFakeRetryClock(tpe(9, 5))
	clock.autoFire = func(d time.Duration) bool { return d != intradayRetryBudget }
	s.retry.clock = clock
	return s, jobRuns, clock
}

var intradayFourSymbols = []string{"2330", "2454", "2317", "1101"} // batch 2 → 兩批

func finishedOnce(t *testing.T, jobRuns *schedulerJobRunRepoStub) schedulerJobRunFinish {
	t.Helper()
	got := jobRuns.snapshotFinished()
	if len(got) != 1 {
		t.Fatalf("應寫入一筆 intraday，得到 %+v", got)
	}
	return got[0]
}

// 暫時性失敗一次後成功 → ⛔ 不計失敗；等的是 10 秒。
func TestIntradayBatchRetrySucceedsAfterTransientFailure(t *testing.T) {
	src := &retryBatchSource{batch: 2, errs: []error{transientDialErr()}}
	s, jobRuns, clock := newIntradayRetryScheduler(t, src, &retryCandleRepo{}, retryOn, intradayFourSymbols)

	s.runIntradayBatch()

	if got := finishedOnce(t, jobRuns); got.status != "success" || got.symbolsFailed != 0 {
		t.Fatalf("得到 %s %d 檔失敗，期望 success、0 檔", got.status, got.symbolsFailed)
	}
	if n := len(src.attempts()); n != 3 {
		t.Fatalf("抓了 %d 次，期望 3 次（第一批 2 次、第二批 1 次）", n)
	}
	if w := clock.waited(); len(w) != 1 || w[0] != 10*time.Second {
		t.Fatalf("等待 = %v，期望 [10s]", w)
	}
}

// 兩次重試都失敗 → 計入失敗；等 10 秒、再等 20 秒。
func TestIntradayBatchRetryGivesUpAfterTwoRetries(t *testing.T) {
	src := &retryBatchSource{batch: 2, errs: []error{transientDialErr(), transientDialErr(), transientDialErr()}}
	s, jobRuns, clock := newIntradayRetryScheduler(t, src, &retryCandleRepo{}, retryOn, intradayFourSymbols)

	s.runIntradayBatch()

	if got := finishedOnce(t, jobRuns); got.symbolsFailed != 2 {
		t.Fatalf("失敗 %d 檔，期望第一批的 2 檔", got.symbolsFailed)
	}
	if n := len(src.attempts()); n != 4 {
		t.Fatalf("抓了 %d 次，期望 4 次（第一批 3 次、第二批 1 次）", n)
	}
	if w := clock.waited(); len(w) != 2 || w[0] != 10*time.Second || w[1] != 20*time.Second {
		t.Fatalf("等待 = %v，期望 [10s 20s]", w)
	}
}

// 永久錯誤⛔ 不重試；開關關閉⛔ 不重試。
func TestIntradayBatchRetrySkipsPermanentOrDisabled(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  config.JobRetryConfig
		err  error
	}{
		{"404", retryOn, &market.HTTPStatusError{Code: 404}},
		{"429", retryOn, &market.HTTPStatusError{Code: 429}},
		{"開關關閉", config.JobRetryConfig{Enabled: false}, transientDialErr()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := &retryBatchSource{batch: 2, errs: []error{tc.err}}
			s, jobRuns, clock := newIntradayRetryScheduler(t, src, &retryCandleRepo{}, tc.cfg, intradayFourSymbols)

			s.runIntradayBatch()

			if got := finishedOnce(t, jobRuns); got.symbolsFailed != 2 {
				t.Fatalf("失敗 %d 檔，期望 2 檔", got.symbolsFailed)
			}
			if n := len(src.attempts()); n != 2 || len(clock.waited()) != 0 {
				t.Fatalf("抓了 %d 次、等待 %v，期望 2 次、⛔ 沒有等待", n, clock.waited())
			}
		})
	}
}

// 重試的期限：預算在第一批的重試請求途中到期 → 那個請求被中止，之後的批次⛔ 不再重試；
// 每批的**第一次**抓取⛔ 不受預算影響（到期之後照樣送、收到的 ctx 是活的）。
func TestIntradayRetryBudgetBoundsAllRetries(t *testing.T) {
	var clock *fakeRetryClock
	src := &retryBatchSource{batch: 2, errs: []error{transientDialErr(), nil, transientDialErr(), transientDialErr()}}
	src.onCall = func(ctx context.Context, n int) error {
		if n != 2 {
			return nil
		}
		// 第一批的重試：慢的 HTTP，等到預算到期才返回。
		clock.fireDuration(intradayRetryBudget)
		<-ctx.Done()
		return ctx.Err()
	}
	s, jobRuns, c := newIntradayRetryScheduler(t, src, &retryCandleRepo{},
		retryOn, []string{"2330", "2454", "2317", "1101", "2603", "2882"})
	clock = c

	done := make(chan struct{})
	go func() {
		defer close(done)
		s.runIntradayBatch()
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("預算到期之後重試仍在等：重試請求沒有用重試的 context")
	}

	attempts := src.attempts()
	if len(attempts) != 4 {
		t.Fatalf("抓了 %d 次，期望 4 次（三批各一次 ＋ 第一批的重試 1 次）", len(attempts))
	}
	for _, i := range []int{0, 2, 3} {
		if attempts[i].ctxErr != nil {
			t.Fatalf("第 %d 次是某一批的第一次抓取，⛔ 不受重試預算影響，卻收到 %v", i+1, attempts[i].ctxErr)
		}
	}
	if got := finishedOnce(t, jobRuns); got.symbolsFailed != 6 {
		t.Fatalf("失敗 %d 檔，期望 6 檔", got.symbolsFailed)
	}
}

// HTTP 成功、寫 K 棒時預算到期：那一批仍計入失敗，⛔ 不算重試成功
// （FetchAndStoreIntradayBatch 遇到逐檔寫入失敗只記 log、仍回 nil）。
func TestIntradayRetrySuccessRequiresLiveBudget(t *testing.T) {
	var clock *fakeRetryClock
	candles := &retryCandleRepo{onInsert: func(ctx context.Context) error {
		clock.fireDuration(intradayRetryBudget)
		<-ctx.Done()
		return ctx.Err()
	}}
	src := &retryBatchSource{batch: 2, errs: []error{transientDialErr()}}
	s, jobRuns, c := newIntradayRetryScheduler(t, src, candles, retryOn, []string{"2330", "2454"})
	clock = c

	s.runIntradayBatch()

	if got := finishedOnce(t, jobRuns); got.symbolsFailed != 2 {
		t.Fatalf("失敗 %d 檔，期望 2 檔（預算到期⛔ 不算重試成功）", got.symbolsFailed)
	}
	if n := len(src.attempts()); n != 2 {
		t.Fatalf("抓了 %d 次，期望 2 次", n)
	}
}
