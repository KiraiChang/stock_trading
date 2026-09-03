package scheduler

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/analysis"
	"github.com/trading/backend/internal/chip"
	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/market"
	"github.com/trading/backend/internal/store"
)

// 這一組是 docs/issue.md I-104 的關閉條件：**對每個 job 注入帶敏感標記的錯誤，
// 斷言寫入的 error 欄位不含該標記。**
//
// ⚠️ **每個案例都要真的走到「這次改動的那一行」。** 2026-09-02 的第一版所有案例
// 都共用一個 failing watchlist，於是多數只觸發「取清單失敗」的早退分支——
// 逐檔那條路徑（`joberr.SummaryFor`）根本沒被執行，測試卻是綠的。
//
// ⛔ 新增走 job 錯誤欄位的路徑時，要在這裡補一個**實際執行到該行**的案例。

const leakMarker = "postgres://trading_user:s3cr3t@db.internal:5432/trading"

func leakingError() error { return errors.New("dial " + leakMarker + ": connection refused") }

func assertNoLeak(t *testing.T, jobName, errMsg string) {
	t.Helper()
	for _, bad := range []string{leakMarker, "s3cr3t", "db.internal", "5432"} {
		if strings.Contains(errMsg, bad) {
			t.Errorf("%s 的 error 欄位外洩了 %q：%q", jobName, bad, errMsg)
		}
	}
	if errMsg == "" {
		t.Errorf("%s 應寫入分類後的原因，得到空字串", jobName)
	}
}

// ── A. 取清單失敗的早退路徑 ─────────────────────────────────────

func TestJobEarlyReturnsDoNotLeakCause(t *testing.T) {
	tests := []struct {
		name  string
		build func(*schedulerJobRunRepoStub) *Scheduler
		run   func(*Scheduler)
	}{
		{"pre_market", newWatchlistFailScheduler, func(s *Scheduler) { s.runPreMarket() }},
		{"daily_close", newWatchlistFailScheduler, func(s *Scheduler) { s.RunDailyClose() }},
		{"chip_daily_sync", newWatchlistFailScheduler, func(s *Scheduler) { s.runChipDailySync(context.Background()) }},
		{"sr_evaluation", newWatchlistFailScheduler, func(s *Scheduler) { s.runSREvaluation(context.Background()) }},
		{"sr_analysis", func(j *schedulerJobRunRepoStub) *Scheduler {
			s := newWatchlistFailScheduler(j)
			s.srAnalysisRunner = failingAnalysisRunner{}
			return s
		}, func(s *Scheduler) { s.runSRAnalysisOwned(context.Background(), false) }},
		{"sr_zone_verify", func(j *schedulerJobRunRepoStub) *Scheduler {
			s := newWatchlistFailScheduler(j)
			s.srZoneRepo = &failingZoneRepo{err: leakingError()}
			return s
		}, func(s *Scheduler) { s.runSRZoneVerification(context.Background()) }},
		{"evaluation_universe_sync", func(j *schedulerJobRunRepoStub) *Scheduler {
			s := newWatchlistFailScheduler(j)
			s.evaluationUniverse = &universeRepoStub{err: leakingError()}
			return s
		}, func(s *Scheduler) { s.runEvaluationUniverseSync(context.Background()) }},
		{"stock_symbol_sync", func(j *schedulerJobRunRepoStub) *Scheduler {
			s := newWatchlistFailScheduler(j)
			// StockSymbolSyncer 的建構子收 StockSymbolSource **介面**，注入得了。
			s.stockSyncer = market.NewStockSymbolSyncer(
				failingSymbolSource{}, nil, zap.NewNop())
			return s
		}, func(s *Scheduler) { s.runStockSymbolSync(context.Background()) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobRuns := &schedulerJobRunRepoStub{}
			s := tt.build(jobRuns)
			tt.run(s)

			if len(jobRuns.finished) == 0 {
				t.Fatalf("%s 應寫入 job_runs", tt.name)
			}
			assertNoLeak(t, tt.name, jobRuns.finished[0].errMsg)
		})
	}
}

// ── B. 逐檔失敗路徑（這次改動的另一半）────────────────────────
//
// ⚠️ **這些案例必須讓清單成功、逐檔才失敗**——否則會被 A 的早退吃掉。

func TestPerSymbolFailurePathsDoNotLeakCause(t *testing.T) {
	t.Run("sr_analysis 逐檔失敗", func(t *testing.T) {
		jobRuns := &schedulerJobRunRepoStub{}
		s := newOKWatchlistScheduler(jobRuns)
		s.srAnalysisRunner = failingAnalysisRunner{}
		// 查不到最新分析 → 照跑，測試才走得到逐檔失敗那一行。
		s.srZoneRepo = &failingZoneRepo{err: errors.New("no rows")}

		s.runSRAnalysisOwned(context.Background(), false)

		if len(jobRuns.finished) == 0 {
			t.Fatal("應寫入 job_runs")
		}
		got := jobRuns.finished[0].errMsg
		assertNoLeak(t, "sr_analysis(per-symbol)", got)
		// 逐檔那條走的是 SummaryFor，形式是 stage:symbol:reason。
		if !strings.HasPrefix(got, "sr_analysis:2330:") {
			t.Errorf("應為 stage:symbol:reason 形式，得到 %q", got)
		}
	})

	t.Run("evaluation_universe_sync 逐檔失敗", func(t *testing.T) {
		jobRuns := &schedulerJobRunRepoStub{}
		s := newOKWatchlistScheduler(jobRuns)
		s.evaluationUniverse = &universeRepoStub{symbols: []string{"2330"}}
		// 真的接一個會逐檔失敗的 fetcher，才走得到 onSymbol 裡的 SummaryFor。
		s.fetcher = market.NewFetcher(failingMarketSource{}, leakCandleStub{}, zap.NewNop())

		s.runEvaluationUniverseSync(context.Background())

		if len(jobRuns.finished) == 0 {
			t.Fatal("應寫入 job_runs")
		}
		got := jobRuns.finished[0].errMsg
		assertNoLeak(t, "evaluation_universe_sync(per-symbol)", got)
		if !strings.HasPrefix(got, "universe_sync:2330:") {
			t.Errorf("應為 stage:symbol:reason 形式，得到 %q", got)
		}
	})

	t.Run("chip_daily_sync 逐檔失敗", func(t *testing.T) {
		jobRuns := &schedulerJobRunRepoStub{}
		s := newOKWatchlistScheduler(jobRuns)
		// chip.NewSyncer 收 market.ChipDataSource 介面，注入得了。
		// SyncDaily 傳 nil dataTypes → 全部類型都跑，所以 institutionalRepo 也要給
		// （computeAndStoreScore 會用它）。讓它回錯，逐檔就會失敗。
		s.chipSyncer = chip.NewSyncer(failingChipSource{}, failingInstRepo{}, failingMarginRepo{},
			nil, nil, leakCandleStub{}, zap.NewNop())

		s.runChipDailySync(context.Background())

		if len(jobRuns.finished) == 0 {
			t.Fatal("應寫入 job_runs")
		}
		got := jobRuns.finished[0].errMsg
		assertNoLeak(t, "chip_daily_sync(per-symbol)", got)
		if !strings.HasPrefix(got, "chip_sync:2330:") {
			t.Errorf("應為 stage:symbol:reason 形式，得到 %q", got)
		}
	})

	t.Run("sr_zone_verify 逐檔失敗", func(t *testing.T) {
		jobRuns := &schedulerJobRunRepoStub{}
		s := newOKWatchlistScheduler(jobRuns)
		// ListRefsSince 回一筆 → 進入逐檔迴圈；Verify 靠 repo 失敗而回錯。
		s.srZoneRepo = &verifyZoneRepo{ref: store.SRZoneAnalysisRef{ID: 1, Symbol: "2330"}, err: leakingError()}
		s.srZoneVerifier = analysis.NewSRZoneVerifier(
			&verifyZoneRepo{ref: store.SRZoneAnalysisRef{ID: 1, Symbol: "2330"}, err: leakingError()},
			leakCandleStub{})

		s.runSRZoneVerification(context.Background())

		if len(jobRuns.finished) == 0 {
			t.Fatal("應寫入 job_runs")
		}
		got := jobRuns.finished[0].errMsg
		assertNoLeak(t, "sr_zone_verify(per-symbol)", got)
		if !strings.HasPrefix(got, "zone_verify:2330:") {
			t.Errorf("應為 stage:symbol:reason 形式，得到 %q", got)
		}
	})
}

// ── C1. corporate_action_sync：逐檔事件同步中止 ─────────────────

func TestCorporateActionSyncDoesNotLeakCause(t *testing.T) {
	t.Run("列出標的失敗（早退）", func(t *testing.T) {
		jobRuns := &schedulerJobRunRepoStub{}
		candles := &schedulerAdjusterCandleStub{err: leakingError()}
		adj := market.NewAdjuster(schedulerSplitSourceStub{}, schedulerActionRepoStub{}, candles, zap.NewNop())
		adj.SetDividendSource(&schedulerDividendStub{})
		s := &Scheduler{jobRuns: jobRuns, adjuster: adj, log: zap.NewNop()}

		s.RunCorporateActionSync()

		if len(jobRuns.finished) == 0 {
			t.Fatal("應寫入 job_runs")
		}
		assertNoLeak(t, "corporate_action_sync(early)", jobRuns.finished[0].errMsg)
	})

	t.Run("watchlist 失敗（errParts）", func(t *testing.T) {
		jobRuns := &schedulerJobRunRepoStub{}
		// 標的清單成功、watchlist 失敗 → 走 errParts 的
		// joberr.Summary("corporate_action_watchlist", …) 那一行。
		candles := &schedulerAdjusterCandleStub{symbols: []string{"2330"}}
		adj := market.NewAdjuster(schedulerSplitSourceStub{}, schedulerActionRepoStub{}, candles, zap.NewNop())
		adj.SetDividendSource(&schedulerDividendStub{})
		s := &Scheduler{
			jobRuns:   jobRuns,
			adjuster:  adj,
			watchlist: &schedulerWatchlistStub{err: leakingError()},
			log:       zap.NewNop(),
		}

		s.RunCorporateActionSync()

		if len(jobRuns.finished) == 0 {
			t.Fatal("應寫入 job_runs")
		}
		got := jobRuns.finished[0].errMsg
		assertNoLeak(t, "corporate_action_sync(errParts)", got)
		if !strings.Contains(got, "corporate_action_watchlist:") {
			t.Errorf("應含 watchlist 的 stage:reason，得到 %q", got)
		}
	})
}

// ── C2. sr_evaluation 的其餘替換點 ─────────────────────────────

func TestSREvaluationBranchesDoNotLeakCause(t *testing.T) {
	// 上游回 500 → 同時走 MarkFailed 與 finishRun 兩個替換點。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("upstream failed at " + leakMarker))
	}))
	defer srv.Close()

	t.Run("job create 失敗", func(t *testing.T) {
		jobRuns := &schedulerJobRunRepoStub{}
		jobs := &evalJobStub{createErr: leakingError()}
		s := newSREvaluationScheduler(jobRuns, jobs, srv.URL)

		s.runSREvaluation(context.Background())

		if len(jobRuns.finished) == 0 {
			t.Fatal("應寫入 job_runs")
		}
		assertNoLeak(t, "sr_evaluation(create)", jobRuns.finished[0].errMsg)
	})

	t.Run("MarkDone 失敗", func(t *testing.T) {
		// 上游成功、MarkDone 失敗——這條走的是最後一個 finishRun 替換點（`:816`）。
		okSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"run_id":"r1","rows":1}`))
		}))
		defer okSrv.Close()

		jobRuns := &schedulerJobRunRepoStub{}
		jobs := &evalJobStub{markDoneErr: leakingError()}
		s := newSREvaluationScheduler(jobRuns, jobs, okSrv.URL)

		s.runSREvaluation(context.Background())

		if len(jobRuns.finished) == 0 {
			t.Fatal("應寫入 job_runs")
		}
		assertNoLeak(t, "sr_evaluation(MarkDone)", jobRuns.finished[0].errMsg)
	})

	t.Run("上游失敗（同時寫 job_runs 與 job 紀錄）", func(t *testing.T) {
		jobRuns := &schedulerJobRunRepoStub{}
		jobs := &evalJobStub{}
		s := newSREvaluationScheduler(jobRuns, jobs, srv.URL)

		s.runSREvaluation(context.Background())

		if len(jobRuns.finished) == 0 {
			t.Fatal("應寫入 job_runs")
		}
		assertNoLeak(t, "sr_evaluation(upstream)/job_runs", jobRuns.finished[0].errMsg)
		assertNoLeak(t, "sr_evaluation(upstream)/MarkFailed", jobs.failedMsg)
	})
}

// ── C3. corporate action 的 SyncSplits 早退 ────────────────────

func TestCorporateActionSplitFailureDoesNotLeakCause(t *testing.T) {
	jobRuns := &schedulerJobRunRepoStub{}
	adj := market.NewAdjuster(failingSplitSource{}, schedulerActionRepoStub{},
		&schedulerAdjusterCandleStub{symbols: []string{"2330"}}, zap.NewNop())
	adj.SetDividendSource(&schedulerDividendStub{})
	s := &Scheduler{jobRuns: jobRuns, adjuster: adj, log: zap.NewNop()}

	s.RunCorporateActionSync()

	if len(jobRuns.finished) == 0 {
		t.Fatal("應寫入 job_runs")
	}
	assertNoLeak(t, "corporate_action_sync(splits)", jobRuns.finished[0].errMsg)
}

// ── C4. candle_gap_detection 的兩個替換點 ──────────────────────

func TestCandleGapDetectionUnavailableDoesNotLeakCause(t *testing.T) {
	t.Run("日曆取不到", func(t *testing.T) {
		reference := &gapReferenceStub{calErr: leakingError()}
		s, jobRuns := newGapScheduler(&gapVerificationStub{}, reference, &gapCandleStub{}, nil, nil)

		s.runCandleGapDetection(context.Background(), []string{"2330"},
			map[string]store.StockSymbolState{"2330": listedState("上市")}, nil)

		run := gapJobRun(t, jobRuns)
		assertNoLeak(t, "candle_gap_detection(calendar)", run.errMsg)
		if !strings.HasPrefix(run.errMsg, "verification_unavailable: ") {
			t.Errorf("應保留前綴，得到 %q", run.errMsg)
		}
	})

	t.Run("對照源日期取不到", func(t *testing.T) {
		reference := &gapReferenceStub{
			traded:        map[string]map[string]bool{},
			sourceAsOfErr: leakingError(),
		}
		s, jobRuns := newGapScheduler(&gapVerificationStub{}, reference, &gapCandleStub{}, nil, nil)

		s.runCandleGapDetection(context.Background(), []string{"2330"},
			map[string]store.StockSymbolState{"2330": listedState("上市")}, nil)

		run := gapJobRun(t, jobRuns)
		assertNoLeak(t, "candle_gap_detection(stale)", run.errMsg)
	})
}

// ── D. tally 的摘要 ────────────────────────────────────────────

func TestTallySummaryDoesNotLeakCause(t *testing.T) {
	tally := newJobFailureTally()
	tally.addFetchFailure("2330", leakingError())
	tally.addEvaluateFailure("2454", leakingError())
	tally.addDegraded("6182", map[string]error{"signal_persist_failed": leakingError()})

	assertNoLeak(t, "tally.summary", tally.summary())
}

func TestSafeJobErrorSummaryDoesNotLeak(t *testing.T) {
	got := safeJobErrorSummary("watchlist_fetch", leakingError())
	assertNoLeak(t, "safeJobErrorSummary", got)
	if !strings.HasPrefix(got, "watchlist_fetch:") {
		t.Errorf("應為 stage:reason 形式，得到 %q", got)
	}
}

// ── stub ───────────────────────────────────────────────────────

func newWatchlistFailScheduler(jobRuns *schedulerJobRunRepoStub) *Scheduler {
	return New(
		nil, nil, &schedulerWatchlistStub{err: leakingError()}, jobRuns,
		nil, nil, nil, "", nil, "", false,
		nil, nil, nil, nil, config.SREvaluationConfig{}, false, zap.NewNop(),
	)
}

// newOKWatchlistScheduler 讓清單成功，逼測試走到逐檔那條路徑。
func newOKWatchlistScheduler(jobRuns *schedulerJobRunRepoStub) *Scheduler {
	return New(
		nil, nil, &schedulerWatchlistStub{symbols: []string{"2330"}}, jobRuns,
		nil, nil, nil, "", nil, "", false,
		nil, nil, nil, nil, config.SREvaluationConfig{}, false, zap.NewNop(),
	)
}

type failingZoneRepo struct {
	store.SRZoneRepo
	err error
}

func (f *failingZoneRepo) ListRefsSince(context.Context, time.Time, int) ([]store.SRZoneAnalysisRef, error) {
	return nil, f.err
}

// GetLatestByTimeframe 回錯 → srAnalysisSkipReason 判「照跑」，
// 測試因此走得到逐檔那條路徑。
func (f *failingZoneRepo) GetLatestByTimeframe(context.Context, string, string) (*store.SRZoneAnalysis, error) {
	return nil, f.err
}

type failingAnalysisRunner struct{}

func (failingAnalysisRunner) RunAnalysis(context.Context, string, string, int) (uint64, error) {
	return 0, leakingError()
}

// leakCandleStub 提供最小的 CandleRepo 行為，讓需要它的路徑走得下去。
//
// ⚠️ **GetRange 回一根 K 棒是刻意的**：chip 的 hasDataForDate 在 candles 為空時
// 會往下問 institutional/margin/broker 三個 repo，那會把測試拖進一堆無關的 stub。
// 回一根就直接短路成「有資料」，測試專注在 fetch 失敗那條路徑。
type leakCandleStub struct{ store.CandleRepo }

func (leakCandleStub) GetRange(context.Context, string, string, time.Time, time.Time) ([]store.Candle, error) {
	return []store.Candle{{Symbol: "2330", Timeframe: "1d", Close: 100}}, nil
}
func (leakCandleStub) GetLatestN(context.Context, string, string, int) ([]store.Candle, error) {
	return nil, nil
}
func (leakCandleStub) BulkInsert(context.Context, []store.Candle) error { return nil }
func (leakCandleStub) SymbolsWithCandleOn(context.Context, []string, string, time.Time) ([]string, error) {
	return nil, nil
}

// failingMarketSource 讓 BackfillHistory 的逐檔抓取失敗。
type failingMarketSource struct{ market.MarketDataSource }

func (failingMarketSource) FetchDailyCandles(context.Context, string, time.Time, time.Time) ([]market.Candle, error) {
	return nil, leakingError()
}

// failingChipSource 讓 chip.SyncDaily 逐檔失敗。
type failingChipSource struct{ market.ChipDataSource }

func (failingChipSource) FetchInstitutionalTrades(context.Context, string, time.Time, time.Time) ([]market.InstitutionalTrade, error) {
	return nil, leakingError()
}
func (failingChipSource) FetchMarginTrades(context.Context, string, time.Time, time.Time) ([]market.MarginTrade, error) {
	return nil, leakingError()
}
func (failingChipSource) FetchBrokerTrades(context.Context, string, time.Time) ([]market.BrokerTrade, error) {
	return nil, leakingError()
}

// failingInstRepo 讓 computeAndStoreScore 走失敗路徑而不是 nil panic。
type failingInstRepo struct{ store.InstitutionalTradeRepo }

func (failingInstRepo) GetRange(context.Context, string, time.Time, time.Time) ([]store.InstitutionalTrade, error) {
	return nil, leakingError()
}

func (failingInstRepo) GetByDate(context.Context, string, time.Time) (*store.InstitutionalTrade, error) {
	return nil, sql.ErrNoRows
}

// failingMarginRepo：targetDatePublished 也會問它。
type failingMarginRepo struct{ store.MarginTradeRepo }

func (failingMarginRepo) GetByDate(context.Context, string, time.Time) (*store.MarginTrade, error) {
	return nil, sql.ErrNoRows
}
func (failingMarginRepo) GetRange(context.Context, string, time.Time, time.Time) ([]store.MarginTrade, error) {
	return nil, leakingError()
}

// verifyZoneRepo 回一筆 ref 讓 sr_zone_verify 進入逐檔迴圈，其餘查詢一律失敗。
type verifyZoneRepo struct {
	store.SRZoneRepo
	ref store.SRZoneAnalysisRef
	err error
}

func (v *verifyZoneRepo) ListRefsSince(context.Context, time.Time, int) ([]store.SRZoneAnalysisRef, error) {
	return []store.SRZoneAnalysisRef{v.ref}, nil
}

// Get 回錯 → Verify 立刻回錯，逐檔那條就走得到 SummaryFor。
func (v *verifyZoneRepo) Get(context.Context, uint64) (*store.SRZoneAnalysis, error) {
	return nil, v.err
}

// evalJobStub 讓 sr_evaluation 的 job 紀錄可注入。
type evalJobStub struct {
	store.SREvaluationJobRepo
	createErr   error
	markDoneErr error
	failedMsg   string
}

func (e *evalJobStub) MarkDone(context.Context, string, store.RawJSON, string, string, string, int, int) error {
	return e.markDoneErr
}

func (e *evalJobStub) Create(context.Context, *store.SREvaluationJob) (uint64, error) {
	return 1, e.createErr
}
func (e *evalJobStub) MarkRunning(context.Context, string) error { return nil }
func (e *evalJobStub) MarkFailed(_ context.Context, _ string, msg string) error {
	e.failedMsg = msg
	return nil
}

func newSREvaluationScheduler(jobRuns *schedulerJobRunRepoStub, jobs store.SREvaluationJobRepo, url string) *Scheduler {
	return &Scheduler{
		jobRuns:          jobRuns,
		watchlist:        &schedulerWatchlistStub{symbols: []string{"2330"}},
		analysisClient:   analysis.NewClient(url),
		srEvaluationJobs: jobs,
		log:              zap.NewNop(),
	}
}

// failingSplitSource 讓 SyncSplits 早退。
type failingSplitSource struct{}

func (failingSplitSource) FetchSplitPrices(context.Context, time.Time, time.Time) ([]store.CorporateAction, error) {
	return nil, leakingError()
}

type failingSymbolSource struct{}

func (failingSymbolSource) FetchStockSymbols(context.Context) ([]store.StockSymbol, error) {
	return nil, leakingError()
}
