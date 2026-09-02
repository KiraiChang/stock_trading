package scheduler

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/indicator"
	"github.com/trading/backend/internal/signal"
	"github.com/trading/backend/internal/store"
)

// 這一組驗的是**四條 job path 共用的那段新程式碼**：
//
//	evaluateSymbol → tally → finishRunWithTally → jobRuns.Finish
//
// 四個 job（pre_market / intraday×2 / daily_close）在這一段之後完全一樣，
// 差別只在前半的行情抓取，而那一段本筆沒有改。
//
// ⚠️ **為什麼不直接呼叫 RunDailyClose 等四個入口**：`Scheduler.New` 收的是具體型別
// `*market.Fetcher`，測試注入不了 stub——那是與 T-061 同一類的接縫缺口。
// 本筆不擴張到 market 套件去開那個接縫，改為直接驗共用段。

type evalCandleRepo struct {
	store.CandleRepo
	err     error
	candles []store.Candle
}

func (e *evalCandleRepo) GetLatestN(context.Context, string, string, int) ([]store.Candle, error) {
	return e.candles, e.err
}

// flatCandles 是足量但**完全不觸發任何訊號**的平盤資料——用來當「這一輪成功但
// 沒有訊號」的對照組。
//
// ⚠️ 不能改回「回 0 根」：那會走到 ErrInsufficientCandles，變成硬失敗，
// 對照組就失去意義（第一版就是這樣寫錯的）。
func flatCandles(n int) []store.Candle {
	base := time.Date(2026, 9, 1, 1, 0, 0, 0, time.UTC)
	out := make([]store.Candle, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, store.Candle{
			Symbol: "2330", Timeframe: "1d",
			Open: 100, High: 100, Low: 100, Close: 100,
			Volume: 1000, Timestamp: base.Add(time.Duration(i) * 24 * time.Hour),
		})
	}
	return out
}

type evalIndicatorRepo struct{ store.IndicatorRepo }

func (evalIndicatorRepo) Upsert(context.Context, *store.IndicatorSnapshot) error { return nil }

type evalChipRepo struct{ store.ChipScoreRepo }

func (evalChipRepo) GetLatest(context.Context, string) (*store.ChipScore, error) {
	return nil, sql.ErrNoRows
}

// newEvalScheduler 只接本段需要的東西，其餘傳 nil——沒用到就不會被碰。
func newEvalScheduler(t *testing.T, candleErr error, cs []store.Candle) (*Scheduler, *schedulerJobRunRepoStub) {
	t.Helper()
	candles := &evalCandleRepo{err: candleErr, candles: cs}
	ind := indicator.NewEngine(candles, evalIndicatorRepo{}, &store.RedisClient{}, zap.NewNop())
	sigEng := signal.NewEngine(candles, nil, &store.RedisClient{}, ind, evalChipRepo{}, zap.NewNop())
	jobRuns := &schedulerJobRunRepoStub{}
	s := New(
		nil, sigEng, nil, jobRuns, nil, nil, nil, "", nil, "", false,
		nil, nil, nil, nil, config.SREvaluationConfig{}, false, zap.NewNop(),
	)
	return s, jobRuns
}

// TestEvaluateSymbolRecordsHardFailureAndFinishesPartial 是本筆最核心的迴歸：
// 以前四個呼叫點連回傳值都沒接，job_runs 因此一整天顯示「全部成功」。
func TestEvaluateSymbolRecordsHardFailureAndFinishesPartial(t *testing.T) {
	// 帶敏感標記的 DB 讀取失敗——同時驗「失敗被記錄」與「原文不外洩」。
	const marker = "postgres://u:s3cr3t@db.internal:5432/trading"
	s, jobRuns := newEvalScheduler(t, errors.New("dial "+marker+": connection refused"), nil)

	ctx := context.Background()
	runID := s.startRun(ctx, "daily_close")
	tally := newJobFailureTally()
	s.evaluateSymbol(ctx, tally, "2330", "1d")
	s.evaluateSymbol(ctx, tally, "2454", "1d")
	s.finishRunWithTally(ctx, runID, "daily_close", 2, tally)

	if len(jobRuns.finished) != 1 {
		t.Fatalf("應寫入一筆 job_runs，實際 %d", len(jobRuns.finished))
	}
	got := jobRuns.finished[0]
	if got.status != "failed" {
		t.Errorf("兩檔全失敗應為 failed，得到 %q", got.status)
	}
	if got.symbolsFailed != 2 {
		t.Errorf("symbols_failed = %d, want 2", got.symbolsFailed)
	}
	if !strings.Contains(got.errMsg, "evaluate_failed:2") {
		t.Errorf("摘要應含 evaluate_failed:2，得到 %q", got.errMsg)
	}
	if strings.Contains(got.errMsg, marker) || strings.Contains(got.errMsg, "s3cr3t") {
		t.Errorf("job_runs.error 外洩了 cause：%q", got.errMsg)
	}
}

// TestEvaluateSymbolSuccessLeavesRunSuccess 是對照組——沒有這一格，
// 上面那支可能是「永遠 failed」而不是「正確反映失敗」。
func TestEvaluateSymbolSuccessLeavesRunSuccess(t *testing.T) {
	// 足量但平盤 → 算得出指標、但不觸發任何訊號，這一輪就是單純的成功。
	s, jobRuns := newEvalScheduler(t, nil, flatCandles(120))

	ctx := context.Background()
	runID := s.startRun(ctx, "intraday")
	tally := newJobFailureTally()
	s.evaluateSymbol(ctx, tally, "2330", "1m")
	s.finishRunWithTally(ctx, runID, "intraday", 1, tally)

	got := jobRuns.finished[0]
	if got.status != "success" {
		t.Errorf("沒有失敗也沒有降級時應為 success，得到 %q（err=%q）", got.status, got.errMsg)
	}
	if got.errMsg != "" {
		t.Errorf("成功時 error 應為空，得到 %q", got.errMsg)
	}
}

// TestFinishRunWithTallyDegradedOnlyIsPartialWithZeroFailed 驗降級那條路：
// **partial 但 symbols_failed = 0**——這是 degraded 與硬失敗的判別依據。
func TestFinishRunWithTallyDegradedOnlyIsPartialWithZeroFailed(t *testing.T) {
	s, jobRuns := newEvalScheduler(t, nil, flatCandles(120))

	ctx := context.Background()
	runID := s.startRun(ctx, "intraday")
	tally := newJobFailureTally()
	tally.addDegraded("2454", map[string]error{
		"signal_persist_failed": errors.New("pq: numeric field overflow"),
	})
	s.finishRunWithTally(ctx, runID, "intraday", 11, tally)

	got := jobRuns.finished[0]
	if got.status != "partial" {
		t.Errorf("有降級就要 partial，得到 %q", got.status)
	}
	if got.symbolsFailed != 0 {
		t.Errorf("降級不得計入 symbols_failed，得到 %d", got.symbolsFailed)
	}
	if !strings.Contains(got.errMsg, "degraded:1 (signal_persist_failed:1/numeric_overflow)") {
		t.Errorf("摘要 = %q", got.errMsg)
	}
}

// TestEarlyReturnErrorGoesThroughClassifier 驗四個 job 的 watchlist 早退路徑。
func TestEarlyReturnErrorGoesThroughClassifier(t *testing.T) {
	const marker = "postgres://u:s3cr3t@db.internal:5432/trading"
	err := errors.New("dial " + marker + ": connection refused")

	got := safeJobErrorSummary("watchlist_fetch", err)
	if got != "watchlist_fetch:conn_refused" {
		t.Errorf("早退摘要 = %q, want watchlist_fetch:conn_refused", got)
	}
	if strings.Contains(got, marker) {
		t.Errorf("早退路徑外洩了 cause：%q", got)
	}
}

// ── 早退路徑：實際跑 runPreMarket，抓呼叫點漏接 ──────────────────
//
// ⚠️ **只測 safeJobErrorSummary 本身抓不到漏接**——2026-09-02 的 review 就是這樣
// 抓到 pre_market 那一行還留著 err.Error()：helper 測試全綠，呼叫點卻沒改到。
// 所以這支**實際執行 runPreMarket**。

type failingWatchlist struct {
	store.WatchlistRepo
	err error
}

func (f *failingWatchlist) Symbols(context.Context) ([]string, error) { return nil, f.err }

func TestRunPreMarketEarlyReturnDoesNotLeakCause(t *testing.T) {
	const marker = "postgres://u:s3cr3t@db.internal:5432/trading"
	jobRuns := &schedulerJobRunRepoStub{}
	s := New(
		nil, nil,
		&failingWatchlist{err: errors.New("dial " + marker + ": connection refused")},
		jobRuns, nil, nil, nil, "", nil, "", false,
		nil, nil, nil, nil, config.SREvaluationConfig{}, false, zap.NewNop(),
	)

	s.runPreMarket()

	if len(jobRuns.finished) != 1 {
		t.Fatalf("應寫入一筆 job_runs，實際 %d", len(jobRuns.finished))
	}
	got := jobRuns.finished[0]
	if strings.Contains(got.errMsg, marker) || strings.Contains(got.errMsg, "s3cr3t") ||
		strings.Contains(got.errMsg, "db.internal") {
		t.Errorf("pre_market 的早退路徑外洩了 cause：%q", got.errMsg)
	}
	if got.errMsg != "watchlist_fetch:conn_refused" {
		t.Errorf("job_runs.error = %q, want watchlist_fetch:conn_refused", got.errMsg)
	}
}

func TestRunDailyCloseEarlyReturnDoesNotLeakCause(t *testing.T) {
	const marker = "postgres://u:s3cr3t@db.internal:5432/trading"
	jobRuns := &schedulerJobRunRepoStub{}
	s := New(
		nil, nil,
		&failingWatchlist{err: errors.New("dial " + marker + ": connection refused")},
		jobRuns, nil, nil, nil, "", nil, "", false,
		nil, nil, nil, nil, config.SREvaluationConfig{}, false, zap.NewNop(),
	)

	s.RunDailyClose()

	if len(jobRuns.finished) == 0 {
		t.Fatal("應寫入 job_runs")
	}
	got := jobRuns.finished[0]
	if strings.Contains(got.errMsg, marker) {
		t.Errorf("daily_close 的早退路徑外洩了 cause：%q", got.errMsg)
	}
	if got.errMsg != "watchlist_fetch:conn_refused" {
		t.Errorf("job_runs.error = %q", got.errMsg)
	}
}
