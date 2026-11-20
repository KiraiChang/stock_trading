package scheduler

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/database"
	"github.com/trading/backend/internal/indicator"
	"github.com/trading/backend/internal/market"
	"github.com/trading/backend/internal/signal"
	"github.com/trading/backend/internal/store"
)

// T-088＋I-103：盤中逐檔的抓取與寫入結果、評估前的判定、逐輪逐檔的存證。
// 計畫書 docs/plans/t088-i103-intraday-data-availability.md 第七節的案例都在這裡（真的 SQLite、真的訊號引擎）。

type scriptSource struct {
	mu      sync.Mutex
	calls   int
	batch   int
	respond func(ctx context.Context, n int, symbols []string) (map[string][]market.Candle, error)
}

func (s *scriptSource) FetchIntradayCandlesBatch(ctx context.Context, symbols []string) (map[string][]market.Candle, error) {
	s.mu.Lock()
	s.calls++
	n := s.calls
	s.mu.Unlock()
	return s.respond(ctx, n, symbols)
}
func (s *scriptSource) RateLimit() int { return 600 }
func (s *scriptSource) BatchSize() int { return s.batch }

// countingCandles 包住真的 CandleRepo：數訊號引擎的讀取（GetLatestN 100 根），並可讓指定的寫入失敗。
type countingCandles struct {
	store.CandleRepo
	signalReads atomic.Int32
	failInsert  func(ctx context.Context, cs []store.Candle) error
}

func (c *countingCandles) GetLatestN(ctx context.Context, symbol, tf string, n int) ([]store.Candle, error) {
	if n == 100 {
		c.signalReads.Add(1)
	}
	return c.CandleRepo.GetLatestN(ctx, symbol, tf, n)
}

func (c *countingCandles) BulkInsert(ctx context.Context, cs []store.Candle) error {
	if c.failInsert != nil {
		if err := c.failInsert(ctx, cs); err != nil {
			return err
		}
	}
	return c.CandleRepo.BulkInsert(ctx, cs)
}

type intradayHarness struct {
	s        *Scheduler
	db       *sqlx.DB
	candles  *countingCandles
	real     store.CandleRepo
	signals  store.SignalRepo
	evidence store.IntradayEvidenceRepo
	jobRuns  store.JobRunRepo
	clock    *fakeRetryClock
	eng      *signal.Engine
}

func newSchedulerSQLite(t *testing.T) *sqlx.DB {
	t.Helper()
	db, err := store.NewDB(config.DatabaseConfig{Driver: "sqlite", DSN: t.TempDir() + "/intraday.db"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.RunMigrations(context.Background(), db, "sqlite", zap.NewNop()); err != nil {
		t.Fatal(err)
	}
	return db
}

func newIntradayHarness(t *testing.T, src market.BatchQuoteSource, symbols []string) *intradayHarness {
	t.Helper()
	db := newSchedulerSQLite(t)
	real := store.NewCandleRepo(db)
	cc := &countingCandles{CandleRepo: real}
	rdb := store.NewRedis(store.DisabledRedisConfig())
	ind := indicator.NewEngine(cc, store.NewIndicatorRepo(db), rdb, zap.NewNop())
	signals := store.NewSignalRepo(db)
	eng := signal.NewEngine(cc, signals, rdb, ind, store.NewChipScoreRepo(db), zap.NewNop())
	fetcher := market.NewFetcher(nil, cc, zap.NewNop())
	if src != nil {
		fetcher.SetIntradaySource(src)
	}
	jobRuns := store.NewJobRunRepo(db)
	s := New(fetcher, eng, &schedulerWatchlistStub{symbols: symbols}, jobRuns, nil, nil, nil, "", nil, "", false,
		nil, nil, nil, nil, config.SREvaluationConfig{}, false, zap.NewNop())
	s.finishRetryBackoffs = []time.Duration{time.Millisecond}
	s.SetJobRetry(retryOn)
	clock := newFakeRetryClock(tpe(9, 5))
	clock.autoFire = func(d time.Duration) bool { return d != intradayRetryBudget }
	s.retry.clock = clock
	evidence := store.NewIntradayEvidenceRepo(db)
	s.SetIntradayEvidence(evidence, real)
	s.intradayClockFunc = func() time.Time { return tpe(9, 5) }
	return &intradayHarness{s: s, db: db, candles: cc, real: real, signals: signals, evidence: evidence, jobRuns: jobRuns, clock: clock, eng: eng}
}

func (h *intradayHarness) lastJob(t *testing.T, name string) store.JobRun {
	t.Helper()
	runs, err := h.jobRuns.GetRecent(context.Background(), 50)
	if err != nil {
		t.Fatal(err)
	}
	// 取 id 最大的那一筆：同一秒內的兩輪 started_at 相同，⛔ 不靠排序。
	var out store.JobRun
	for _, r := range runs {
		if r.JobName == name && r.ID > out.ID {
			out = r
		}
	}
	if out.ID == 0 {
		t.Fatalf("沒有 %s 的 job_runs", name)
	}
	return out
}

func (h *intradayHarness) rows(t *testing.T, runID uint64) map[string]store.IntradaySymbolRun {
	t.Helper()
	rows, err := h.evidence.ListSymbolRuns(context.Background(), runID)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]store.IntradaySymbolRun{}
	for _, r := range rows {
		out[r.Symbol] = r
	}
	return out
}

// breakout1m 是會觸發 BREAKOUT 的 1 分 K（同 signal 套件的 breakoutCandles，改成分鐘間隔），最後一根在 last。
func breakout1m(symbol string, last time.Time) []store.Candle {
	highs := []float64{
		81, 83, 85, 87, 89, 91, 93, 95, 93, 92, 91, 90, 88, 86, 84, 83, 85,
		87, 89, 92, 100, 96, 94, 92, 90, 88, 90, 92, 94, 96, 98, 99, 99, 99,
	}
	n := len(highs) + 3
	base := last.Add(-time.Duration(n-1) * time.Minute)
	out := make([]store.Candle, 0, n)
	for i, high := range highs {
		cl := high - 1
		out = append(out, store.Candle{Symbol: symbol, Timeframe: "1m", Open: cl - 0.1, High: high, Low: high - 4, Close: cl,
			Volume: 1000, Timestamp: base.Add(time.Duration(i) * time.Minute)})
	}
	tail := []store.Candle{
		{Open: 99, High: 106, Low: 98, Close: 105, Volume: 5000},
		{Open: 105, High: 108, Low: 103, Close: 106, Volume: 1200},
		{Open: 106, High: 109, Low: 104, Close: 107, Volume: 1100},
	}
	for i, c := range tail {
		c.Symbol, c.Timeframe = symbol, "1m"
		c.Timestamp = base.Add(time.Duration(len(highs)+i) * time.Minute)
		out = append(out, c)
	}
	return out
}

func flat1m(symbol string, last time.Time, n int) []store.Candle {
	out := make([]store.Candle, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, store.Candle{Symbol: symbol, Timeframe: "1m", Open: 100, High: 100, Low: 100, Close: 100,
			Volume: 1000, Timestamp: last.Add(-time.Duration(n-1-i) * time.Minute)})
	}
	return out
}

func toMarket(cs []store.Candle) []market.Candle {
	out := make([]market.Candle, 0, len(cs))
	for _, c := range cs {
		out = append(out, market.Candle{Symbol: c.Symbol, Timeframe: c.Timeframe, Open: c.Open, High: c.High, Low: c.Low,
			Close: c.Close, Volume: c.Volume, Timestamp: c.Timestamp})
	}
	return out
}

func seed(t *testing.T, repo store.CandleRepo, cs []store.Candle) {
	t.Helper()
	if err := repo.BulkInsert(context.Background(), cs); err != nil {
		t.Fatal(err)
	}
}

var http500 = &market.HTTPStatusError{Code: 500}

// 首版最重要的一條：DB 裡的舊 K 棒符合突破條件，HTTP 500 且重試用完 → ⛔ 不呼叫訊號引擎、⛔ 不產生或推送訊號，
// 存證是「行情更新失敗、本輪未評估」，⛔ 不是「未觸發」。
func TestIntradayFetchFailureDoesNotEvaluateStaleCandles(t *testing.T) {
	src := &scriptSource{batch: 10, respond: func(context.Context, int, []string) (map[string][]market.Candle, error) {
		return nil, http500
	}}
	h := newIntradayHarness(t, src, []string{"2330"})
	seed(t, h.real, breakout1m("2330", tpe(9, 4)))
	var broadcast int
	h.eng.BroadcastFn = func(string, *store.Signal) { broadcast++ }

	h.s.runIntradayRound(tpe(9, 5))

	if n := h.candles.signalReads.Load(); n != 0 {
		t.Fatalf("抓取失敗時⛔ 不該呼叫訊號引擎，卻讀了 %d 次", n)
	}
	if rows, _ := h.signals.GetBySymbol(context.Background(), "2330", 10); len(rows) != 0 || broadcast != 0 {
		t.Fatalf("⛔ 不該產生或推送訊號：signals %d 筆、推播 %d 次", len(rows), broadcast)
	}
	job := h.lastJob(t, "intraday")
	if job.Status != "failed" || job.SymbolsFailed != 1 || !strings.Contains(job.Error.String, "fetch_failed:1 (2330:") {
		t.Fatalf("job_runs = %s %d %q", job.Status, job.SymbolsFailed, job.Error.String)
	}
	row := h.rows(t, job.ID)["2330"]
	if row.DataStatus != "fetch_failed" || row.DataCause.String != "http_5xx" || row.FetchAttempts != 3 || row.HTTPRecovered {
		t.Fatalf("行情段 = %+v", row)
	}
	if row.EvalStatus.String != store.IntradayEvalNotEvaluated || row.EvalCause.String != "fetch_failed" {
		t.Fatalf("評估段應是 not_evaluated／fetch_failed，得到 %q／%q", row.EvalStatus.String, row.EvalCause.String)
	}
	// 對照：同一批舊 K 棒拿去評估確實會觸發——上面⛔ 不觸發是因為沒有評估，⛔ 不是條件不成立。
	sig, err := h.eng.Evaluate(context.Background(), "2330", "1m")
	if err != nil || sig == nil || sig.SignalType != "BREAKOUT" {
		t.Fatalf("對照組應觸發 BREAKOUT，得到 %+v（err=%v）", sig, err)
	}
}

// 500 重試得到 200 之後：A 寫入成功、B 寫入失敗、C 沒有資料，三檔分別記錄；「HTTP 救回」⛔ 不等於「寫入成功」。
func TestIntradayRetryKeepsPerSymbolResults(t *testing.T) {
	src := &scriptSource{batch: 10, respond: func(_ context.Context, n int, _ []string) (map[string][]market.Candle, error) {
		if n == 1 {
			return nil, http500
		}
		return map[string][]market.Candle{
			"A": toMarket(flat1m("A", tpe(9, 4), 40)),
			"B": toMarket(flat1m("B", tpe(9, 4), 40)),
		}, nil
	}}
	h := newIntradayHarness(t, src, []string{"A", "B", "C"})
	h.candles.failInsert = func(_ context.Context, cs []store.Candle) error {
		if len(cs) > 0 && cs[0].Symbol == "B" {
			return errors.New("disk full")
		}
		return nil
	}

	h.s.runIntradayRound(tpe(9, 5))

	job := h.lastJob(t, "intraday")
	if job.Status != "partial" || job.SymbolsFailed != 1 || !strings.Contains(job.Error.String, "store_failed:1 (B:") ||
		strings.Contains(job.Error.String, "fetch_failed") {
		t.Fatalf("job_runs = %s %d %q", job.Status, job.SymbolsFailed, job.Error.String)
	}
	rows := h.rows(t, job.ID)
	for sym, want := range map[string]string{"A": "written", "B": "store_failed", "C": "no_data"} {
		r := rows[sym]
		if r.DataStatus != want || r.FetchAttempts != 2 || !r.HTTPRecovered {
			t.Fatalf("%s = %s（attempts %d、recovered %v），期望 %s、2、true", sym, r.DataStatus, r.FetchAttempts, r.HTTPRecovered, want)
		}
	}
	if rows["A"].EvalStatus.String != store.IntradayEvalNoSignal || !rows["A"].LastBarTS.Valid || rows["A"].Fingerprint.String == "" {
		t.Fatalf("A 應評估（no_signal）並記下最後一根與指紋：%+v", rows["A"])
	}
	for _, sym := range []string{"B", "C"} {
		if rows[sym].EvalStatus.String != store.IntradayEvalNotEvaluated || rows[sym].EvalCause.String != rows[sym].DataStatus {
			t.Fatalf("%s 應⛔ 不評估：%+v", sym, rows[sym])
		}
	}
}

// 重試預算到期時依實際停在哪個階段分類：已提交 written、寫入階段逾時 store_failed（retry_budget）、確認沒有 no_data。
func TestIntradayRetryBudgetClassifiesByStage(t *testing.T) {
	src := &scriptSource{batch: 10, respond: func(_ context.Context, n int, _ []string) (map[string][]market.Candle, error) {
		if n == 1 {
			return nil, transientDialErr()
		}
		return map[string][]market.Candle{
			"A": toMarket(flat1m("A", tpe(9, 4), 40)),
			"B": toMarket(flat1m("B", tpe(9, 4), 40)),
		}, nil
	}}
	h := newIntradayHarness(t, src, []string{"A", "B", "C"})
	h.candles.failInsert = func(ctx context.Context, cs []store.Candle) error {
		if len(cs) > 0 && cs[0].Symbol == "B" {
			h.clock.fireDuration(intradayRetryBudget) // B 寫入途中預算到期
			<-ctx.Done()
			return ctx.Err()
		}
		return nil
	}

	h.s.runIntradayRound(tpe(9, 5))

	job := h.lastJob(t, "intraday")
	rows := h.rows(t, job.ID)
	if rows["A"].DataStatus != "written" || rows["A"].EvalStatus.String == store.IntradayEvalNotEvaluated {
		t.Fatalf("A 已提交，應 written 並評估：%+v", rows["A"])
	}
	if rows["B"].DataStatus != "store_failed" || rows["B"].DataCause.String != intradayCauseRetryBudget {
		t.Fatalf("B 寫入階段逾時，應 store_failed／retry_budget：%+v", rows["B"])
	}
	if rows["C"].DataStatus != "no_data" {
		t.Fatalf("C 回應確認沒有，應 no_data：%+v", rows["C"])
	}
	if !strings.Contains(job.Error.String, "store_failed:1 (B:") || job.SymbolsFailed != 1 {
		t.Fatalf("job_runs = %d %q", job.SymbolsFailed, job.Error.String)
	}
}

// HTTP 回應本身沒拿到（重試的請求被預算截斷）→ 這一批 fetch_failed（retry_budget）。
func TestIntradayRetryBudgetDuringRequestIsFetchFailed(t *testing.T) {
	var h *intradayHarness
	src := &scriptSource{batch: 10, respond: func(ctx context.Context, n int, _ []string) (map[string][]market.Candle, error) {
		if n == 1 {
			return nil, transientDialErr()
		}
		h.clock.fireDuration(intradayRetryBudget)
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	h = newIntradayHarness(t, src, []string{"A", "B"})

	h.s.runIntradayRound(tpe(9, 5))

	job := h.lastJob(t, "intraday")
	for sym, r := range h.rows(t, job.ID) {
		if r.DataStatus != "fetch_failed" || r.DataCause.String != intradayCauseRetryBudget || r.HTTPRecovered {
			t.Fatalf("%s = %+v，期望 fetch_failed／retry_budget", sym, r)
		}
	}
}

// 引擎讀到的 K 棒跟上一次成功評估相同 → ⛔ 不評估（unchanged）；評估失敗後同一份資料再來 → 會重試評估。
func TestIntradayUnchangedInputSkipsOnlyAfterSuccessfulEvaluation(t *testing.T) {
	data := toMarket(flat1m("A", tpe(9, 4), 40))
	short := toMarket(flat1m("S", tpe(9, 4), 10)) // 不足 35 根 → 評估失敗
	src := &scriptSource{batch: 10, respond: func(context.Context, int, []string) (map[string][]market.Candle, error) {
		return map[string][]market.Candle{"A": data, "S": short}, nil
	}}
	h := newIntradayHarness(t, src, []string{"A", "S"})

	h.s.runIntradayRound(tpe(9, 5))
	first := h.rows(t, h.lastJob(t, "intraday").ID)
	if first["A"].EvalStatus.String != store.IntradayEvalNoSignal || first["S"].EvalStatus.String != store.IntradayEvalFailed {
		t.Fatalf("第一輪：A=%q S=%q", first["A"].EvalStatus.String, first["S"].EvalStatus.String)
	}
	reads := h.candles.signalReads.Load()

	h.s.runIntradayRound(tpe(9, 10))
	second := h.rows(t, h.lastJob(t, "intraday").ID)
	if second["A"].EvalStatus.String != store.IntradayEvalNotEvaluated || second["A"].EvalCause.String != intradayEvalCauseUnchanged {
		t.Fatalf("A 輸入相同，應 not_evaluated／unchanged：%+v", second["A"])
	}
	if second["S"].EvalStatus.String != store.IntradayEvalFailed {
		t.Fatalf("S 上一次評估失敗，同一份資料要再評估：%+v", second["S"])
	}
	if h.candles.signalReads.Load() != reads {
		// S 在指標階段就失敗（⛔ 不讀到訊號引擎的 100 根），A 被跳過：訊號引擎的讀取數不變。
		t.Fatalf("第二輪⛔ 不該呼叫訊號引擎讀 A")
	}
}

// 只修訂較早一根（根數、最後一根、成交量合計都不變）→ 指紋改變。
func TestCandleFingerprintDetectsEarlierRevision(t *testing.T) {
	base := flat1m("A", tpe(9, 30), 120)
	revised := append([]store.Candle(nil), base...)
	revised[50].High = 101 // 中間某一根的高點
	if candleFingerprint(base) == candleFingerprint(revised) {
		t.Fatal("只修訂較早一根，指紋應改變")
	}
	reversed := make([]store.Candle, len(base))
	for i := range base {
		reversed[len(base)-1-i] = base[i]
	}
	if candleFingerprint(base) != candleFingerprint(reversed) {
		t.Fatal("同一組 K 棒，順序不同指紋應相同")
	}
	if indicator.Lookback < 100 {
		t.Fatalf("指紋窗口（indicator.Lookback=%d）要涵蓋訊號引擎讀的 100 根", indicator.Lookback)
	}
}

// 新但延遲超過門檻、觸發 → withheld_stale：⛔ 不寫 signals、⛔ 不推播。
func TestIntradayStaleDataWithholdsSignal(t *testing.T) {
	stale := toMarket(breakout1m("2330", tpe(8, 50))) // 最後一根落後 15 分鐘
	src := &scriptSource{batch: 10, respond: func(context.Context, int, []string) (map[string][]market.Candle, error) {
		return map[string][]market.Candle{"2330": stale}, nil
	}}
	h := newIntradayHarness(t, src, []string{"2330"})
	var broadcast int
	h.eng.BroadcastFn = func(string, *store.Signal) { broadcast++ }

	h.s.runIntradayRound(tpe(9, 5))

	row := h.rows(t, h.lastJob(t, "intraday").ID)["2330"]
	if row.EvalStatus.String != store.IntradayEvalWithheldStale {
		t.Fatalf("應 withheld_stale，得到 %+v", row)
	}
	if rows, _ := h.signals.GetBySymbol(context.Background(), "2330", 10); len(rows) != 0 || broadcast != 0 {
		t.Fatalf("延遲的資料⛔ 不當即時訊號送出：signals %d、推播 %d", len(rows), broadcast)
	}
}

// 等鎖逾時 → skipped＋lock_wait_timeout、輪次紀錄 skipped_lock_timeout、⛔ 沒有逐檔的列。
func TestIntradayRoundSkippedOnLockTimeout(t *testing.T) {
	src := &scriptSource{batch: 10, respond: func(context.Context, int, []string) (map[string][]market.Candle, error) {
		t.Fatal("等鎖逾時⛔ 不該抓取")
		return nil, nil
	}}
	h := newIntradayHarness(t, src, []string{"A"})
	h.s.intradayLockWait = 30 * time.Millisecond
	h.s.intradayLock <- struct{}{} // 另一輪正在跑

	h.s.runIntradayRound(tpe(9, 15))

	job := h.lastJob(t, "intraday")
	if job.Status != "skipped" || job.Error.String != "lock_wait_timeout" {
		t.Fatalf("job_runs = %s %q", job.Status, job.Error.String)
	}
	round, err := h.evidence.LatestRound(context.Background(), store.IntradaySourceScheduled)
	if err != nil || round == nil || round.Outcome != store.IntradayOutcomeSkippedLockTimeout || round.LockAcquiredAt.Valid ||
		!round.ScheduledAt.Equal(tpe(9, 15)) {
		t.Fatalf("輪次紀錄 = %+v（err=%v）", round, err)
	}
	if rows := h.rows(t, job.ID); len(rows) != 0 {
		t.Fatalf("等鎖逾時⛔ 不寫逐檔的列，得到 %d 列", len(rows))
	}
}

// 依序執行：先拿鎖再建 run，兩輪的 job_runs 起訖⛔ 不重疊；排定時點與實際開始分開記。
func TestIntradayRoundsRunSequentiallyWithSeparateScheduleAndExecution(t *testing.T) {
	src := &scriptSource{batch: 10, respond: func(context.Context, int, []string) (map[string][]market.Candle, error) {
		return map[string][]market.Candle{"A": toMarket(flat1m("A", tpe(9, 4), 40))}, nil
	}}
	h := newIntradayHarness(t, src, []string{"A"})
	h.s.intradayLock <- struct{}{} // 第一輪拿到鎖之前先被佔住
	done := make(chan struct{})
	go func() { defer close(done); h.s.runIntradayRound(tpe(9, 5)) }()
	time.Sleep(50 * time.Millisecond)
	<-h.s.intradayLock // 放鎖
	<-done

	round, err := h.evidence.LatestRound(context.Background(), store.IntradaySourceScheduled)
	if err != nil || round == nil || !round.ScheduledAt.Equal(tpe(9, 5)) || !round.LockAcquiredAt.Valid ||
		round.LockWaitMs < 40 || !round.ExecFinishedAt.Valid || round.Outcome != store.IntradayOutcomeExecuted {
		t.Fatalf("輪次紀錄 = %+v（err=%v）", round, err)
	}
}

type failingEvidence struct {
	store.IntradayEvidenceRepo
	failInsert bool
}

func (f *failingEvidence) InsertSymbolRuns(ctx context.Context, rows []store.IntradaySymbolRun) error {
	if f.failInsert {
		return errors.New("disk full")
	}
	return f.IntradayEvidenceRepo.InsertSymbolRuns(ctx, rows)
}

// 逐檔紀錄寫失敗 → evidence_failed:N、該輪至少 partial（⛔ 不出現「整輪 success、卻缺逐檔證據」）。
func TestIntradayEvidenceWriteFailureMarksPartial(t *testing.T) {
	src := &scriptSource{batch: 10, respond: func(context.Context, int, []string) (map[string][]market.Candle, error) {
		return map[string][]market.Candle{"A": toMarket(flat1m("A", tpe(9, 4), 40)), "B": toMarket(flat1m("B", tpe(9, 4), 40))}, nil
	}}
	h := newIntradayHarness(t, src, []string{"A", "B"})
	h.s.intradayEvidence = &failingEvidence{IntradayEvidenceRepo: h.evidence, failInsert: true}

	h.s.runIntradayRound(tpe(9, 5))

	job := h.lastJob(t, "intraday")
	if job.Status != "partial" || job.SymbolsFailed != 0 || !strings.Contains(job.Error.String, "evidence_failed:2") {
		t.Fatalf("job_runs = %s %d %q", job.Status, job.SymbolsFailed, job.Error.String)
	}
	round, _ := h.evidence.LatestRound(context.Background(), store.IntradaySourceScheduled)
	if round == nil || round.EvidenceFailures != 2 {
		t.Fatalf("輪次紀錄的 evidence_failures 應是 2，得到 %+v", round)
	}
	if n := h.candles.signalReads.Load(); n != 2 {
		t.Fatalf("存證失敗⛔ 不影響評估，訊號引擎應讀 2 次，得到 %d", n)
	}
}

type failingStartJobRuns struct{ store.JobRunRepo }

func (failingStartJobRuns) Start(context.Context, string) (uint64, error) { return 0, errors.New("db down") }

// 建立 run 失敗 → 照樣抓取與評估（⛔ 不停掉盤中訊號），只是寫不了逐輪紀錄。
func TestIntradayRunNotCreatedStillEvaluates(t *testing.T) {
	src := &scriptSource{batch: 10, respond: func(context.Context, int, []string) (map[string][]market.Candle, error) {
		return map[string][]market.Candle{"A": toMarket(flat1m("A", tpe(9, 4), 40))}, nil
	}}
	h := newIntradayHarness(t, src, []string{"A"})
	h.s.jobRuns = failingStartJobRuns{h.jobRuns}

	h.s.runIntradayRound(tpe(9, 5))

	if n := h.candles.signalReads.Load(); n != 1 {
		t.Fatalf("應照樣評估，訊號引擎讀了 %d 次", n)
	}
	if round, _ := h.evidence.LatestRound(context.Background(), store.IntradaySourceScheduled); round != nil {
		t.Fatalf("沒有 run 就⛔ 不寫輪次紀錄，得到 %+v", round)
	}
}

// ── 手動補抓 ─────────────────────────────────────────────────────────

// 只寫 K 棒、⛔ 不評估；回報新增／更新／不變與缺口；⛔ 不推進上一次成功評估（下一輪照常評估）。
func TestIntradayRefillWritesOnlyAndReportsChanges(t *testing.T) {
	src := &scriptSource{batch: 10, respond: func(context.Context, int, []string) (map[string][]market.Candle, error) {
		return map[string][]market.Candle{"A": {
			{Symbol: "A", Timeframe: "1m", Open: 1, High: 1, Low: 1, Close: 1, Volume: 1, Timestamp: tpeOn(1, 13, 0)}, // 前一天，⛔ 不補
			{Symbol: "A", Timeframe: "1m", Open: 1, High: 1, Low: 1, Close: 1, Volume: 1, Timestamp: tpe(9, 1)},      // 不變
			{Symbol: "A", Timeframe: "1m", Open: 1, High: 2, Low: 1, Close: 2, Volume: 2, Timestamp: tpe(9, 2)},      // 更新
			{Symbol: "A", Timeframe: "1m", Open: 2, High: 2, Low: 2, Close: 2, Volume: 3, Timestamp: tpe(9, 3)},      // 新增
		}}, nil
	}}
	h := newIntradayHarness(t, src, []string{"A"})
	seed(t, h.real, []store.Candle{
		{Symbol: "A", Timeframe: "1m", Open: 1, High: 1, Low: 1, Close: 1, Volume: 1, Timestamp: tpe(9, 1)},
		{Symbol: "A", Timeframe: "1m", Open: 1, High: 1, Low: 1, Close: 1, Volume: 1, Timestamp: tpe(9, 2)},
	})

	report, err := h.s.RunIntradayRefill(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if n := h.candles.signalReads.Load(); n != 0 {
		t.Fatalf("補抓⛔ 不評估訊號，訊號引擎讀了 %d 次", n)
	}
	got := report.Symbols[0]
	if got.Status != "written" || got.Inserted != 1 || got.Updated != 1 || got.Unchanged != 1 {
		t.Fatalf("回報 = %+v", got)
	}
	if got.BarsToday != 3 || got.LastBarTS == nil || !got.LastBarTS.Equal(tpe(9, 3)) {
		t.Fatalf("寫入進度 = %+v", got)
	}
	// 窗口 09:00～09:05：有 09:01～09:03，缺 09:00、09:04～09:05。
	if got.GapMinutes != 3 || strings.Join(got.Gaps, ",") != "09:00,09:04-09:05" {
		t.Fatalf("缺口 = %d %v", got.GapMinutes, got.Gaps)
	}
	job := h.lastJob(t, intradayRefillJobName)
	if job.Status != "success" {
		t.Fatalf("job_runs = %s %q", job.Status, job.Error.String)
	}
	row := h.rows(t, job.ID)["A"]
	if row.Source != store.IntradaySourceRefill || row.EvalStatus.Valid {
		t.Fatalf("補抓的列⛔ 不該有評估段：%+v", row)
	}
	if last, _ := h.evidence.LastSuccessfulEvals(context.Background(), []string{"A"}); len(last) != 0 {
		t.Fatalf("補抓⛔ 不推進上一次成功評估，得到 %+v", last)
	}
}

// 自選清單是空的：回應的 symbols 是空陣列（⛔ 不是 null，前端會直接 map）。
func TestIntradayRefillEmptyWatchlistReturnsEmptyList(t *testing.T) {
	src := &scriptSource{batch: 10, respond: func(context.Context, int, []string) (map[string][]market.Candle, error) {
		return nil, nil
	}}
	h := newIntradayHarness(t, src, nil)
	report, err := h.s.RunIntradayRefill(context.Background(), nil)
	if err != nil || report.Symbols == nil || len(report.Symbols) != 0 {
		t.Fatalf("得到 %+v（err=%v）", report, err)
	}
}

func TestIntradayRefillRejectsBusyUnknownAndMissingSource(t *testing.T) {
	src := &scriptSource{batch: 10, respond: func(context.Context, int, []string) (map[string][]market.Candle, error) {
		return nil, nil
	}}
	h := newIntradayHarness(t, src, []string{"A"})
	if _, err := h.s.RunIntradayRefill(context.Background(), []string{"ZZZ"}); !errors.Is(err, ErrIntradayRefillUnknownSymbol) {
		t.Fatalf("不在自選清單的標的應被拒絕，得到 %v", err)
	}
	h.s.refillLockWait = 20 * time.Millisecond
	h.s.intradayLock <- struct{}{}
	if _, err := h.s.RunIntradayRefill(context.Background(), nil); !errors.Is(err, ErrIntradayRefillBusy) {
		t.Fatalf("鎖被佔住應回 busy，得到 %v", err)
	}
	<-h.s.intradayLock

	noSrc := newIntradayHarness(t, nil, []string{"A"})
	if _, err := noSrc.s.RunIntradayRefill(context.Background(), nil); !errors.Is(err, ErrIntradayRefillUnavailable) {
		t.Fatalf("沒有批次盤中源應回 unavailable，得到 %v", err)
	}
}

// 補抓整次超過上限 → failed＋refill_deadline，沒處理到的檔記「未完成」。
func TestIntradayRefillDeadline(t *testing.T) {
	src := &scriptSource{batch: 1, respond: func(ctx context.Context, _ int, symbols []string) (map[string][]market.Candle, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	h := newIntradayHarness(t, src, []string{"A", "B"})
	h.s.refillDeadline = 30 * time.Millisecond

	report, err := h.s.RunIntradayRefill(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Deadline || len(report.Symbols) != 2 {
		t.Fatalf("回報 = %+v", report)
	}
	for _, s := range report.Symbols {
		if s.Status != "fetch_failed" || s.Cause != intradayCauseRefillDeadline {
			t.Fatalf("%s = %+v", s.Symbol, s)
		}
	}
	job := h.lastJob(t, intradayRefillJobName)
	if job.Status != "failed" || !strings.HasPrefix(job.Error.String, "refill_deadline") {
		t.Fatalf("job_runs = %s %q", job.Status, job.Error.String)
	}
}

// ── FinMind 逐檔路徑（D4） ──────────────────────────────────────────

type emptyMinuteClient struct{ market.MarketDataSource }

func (emptyMinuteClient) FetchMinuteCandles(context.Context, string, time.Time) ([]market.Candle, error) {
	return nil, nil
}

// 空回應 → no_data、⛔ 不用舊 K 棒評估。
func TestIntradayFinMindEmptyResponseDoesNotEvaluate(t *testing.T) {
	h := newIntradayHarness(t, nil, []string{"2330"})
	h.s.fetcher = market.NewFetcher(emptyMinuteClient{}, h.candles, zap.NewNop())
	h.s.intradayEnabled = true
	seed(t, h.real, breakout1m("2330", tpe(9, 4)))

	h.s.runIntradayRound(tpe(9, 5))

	if n := h.candles.signalReads.Load(); n != 0 {
		t.Fatalf("空回應⛔ 不用舊 K 棒評估，訊號引擎讀了 %d 次", n)
	}
	row := h.rows(t, h.lastJob(t, "intraday").ID)["2330"]
	if row.DataStatus != "no_data" || row.EvalCause.String != "no_data" {
		t.Fatalf("列 = %+v", row)
	}
}


// 讀不到自選清單 → 這一輪明確記成 failed（保留 watchlist_fetch 原因），⛔ 不是 total=0、failed=0 推出來的 success。
func TestIntradayWatchlistFailureIsFailedRound(t *testing.T) {
	for _, tc := range []struct {
		name  string
		batch bool
	}{{"Yahoo 批次", true}, {"FinMind 逐檔", false}} {
		t.Run(tc.name, func(t *testing.T) {
			src := &scriptSource{batch: 10, respond: func(context.Context, int, []string) (map[string][]market.Candle, error) {
				t.Fatal("讀不到自選清單⛔ 不該抓取")
				return nil, nil
			}}
			var h *intradayHarness
			if tc.batch {
				h = newIntradayHarness(t, src, nil)
			} else {
				h = newIntradayHarness(t, nil, nil)
				h.s.fetcher = market.NewFetcher(emptyMinuteClient{}, h.candles, zap.NewNop())
			}
			h.s.watchlist = &schedulerWatchlistStub{err: errors.New("db down")}

			h.s.runIntradayRound(tpe(9, 5))

			job := h.lastJob(t, "intraday")
			if job.Status != "failed" || !strings.HasPrefix(job.Error.String, "watchlist_fetch") {
				t.Fatalf("job_runs = %s %q", job.Status, job.Error.String)
			}
			// 輪次紀錄也是 failed：狀態 API 才分得出這一輪失敗了，⛔ 不沿用上一輪的逐檔結果。
			round, err := h.evidence.LatestRound(context.Background(), store.IntradaySourceScheduled)
			if err != nil || round == nil || round.Outcome != store.IntradayOutcomeFailed || !round.ExecFinishedAt.Valid {
				t.Fatalf("輪次紀錄 = %+v（err=%v）", round, err)
			}
		})
	}
}

// 手動補抓的收尾卡住（job_runs 寫不回去）時，鎖已經放掉：下一輪盤中排程⛔ 不會因此等鎖逾時。
type blockingFinishJobRuns struct {
	store.JobRunRepo
	gate    chan struct{}
	entered chan struct{} // 第一次進入 Finish 時關閉：確認補抓已經走到收尾
	once    sync.Once
}

func (b *blockingFinishJobRuns) Finish(ctx context.Context, runID uint64, status string, total, failed int, errMsg string) error {
	b.once.Do(func() { close(b.entered) })
	<-b.gate
	return b.JobRunRepo.Finish(ctx, runID, status, total, failed, errMsg)
}

func TestIntradayRefillReleasesLockBeforeFinishing(t *testing.T) {
	src := &scriptSource{batch: 10, respond: func(context.Context, int, []string) (map[string][]market.Candle, error) {
		return map[string][]market.Candle{"A": toMarket(flat1m("A", tpe(9, 4), 3))}, nil
	}}
	h := newIntradayHarness(t, src, []string{"A"})
	gate := make(chan struct{})
	jr := &blockingFinishJobRuns{JobRunRepo: h.jobRuns, gate: gate, entered: make(chan struct{})}
	h.s.jobRuns = jr

	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, err := h.s.RunIntradayRefill(context.Background(), nil); err != nil {
			t.Error(err)
		}
	}()

	// **先確認補抓已經進入 job_runs 的收尾**（review：否則可能在補抓還沒拿鎖之前就拿到鎖、誤判通過）。
	select {
	case <-jr.entered:
	case <-time.After(3 * time.Second):
		close(gate)
		t.Fatal("補抓沒有走到收尾")
	}
	release, _, ok := h.s.acquireIntradayLock(200 * time.Millisecond)
	if ok {
		release()
	}
	select {
	case <-done:
		t.Fatal("收尾還卡著，補抓⛔ 不該已經結束")
	default:
	}
	close(gate)
	<-done
	if !ok {
		t.Fatal("收尾卡住時鎖仍被補抓佔著，下一輪盤中排程會等鎖逾時")
	}
	if rows, _ := h.real.GetLatestN(context.Background(), "A", "1m", 10); len(rows) != 3 {
		t.Fatalf("K 棒應已寫入，得到 %d 根", len(rows))
	}
}
