package scheduler

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/indicator"
	"github.com/trading/backend/internal/market"
	"github.com/trading/backend/internal/signal"
	"github.com/trading/backend/internal/store"
	"github.com/trading/backend/pkg/timeutil"
)

// 盤中一輪（T-088＋I-103）：逐檔的抓取與寫入結果、評估前判定能不能評估、逐輪逐檔的存證。
// 現況規格見 docs/yahoo-intraday-integration.md 與 docs/database-schema.md 的 intraday_rounds／intraday_symbol_runs。

const (
	// 盤中一輪與手動補抓共用一把鎖、依序執行；等鎖與補抓整次都有上限（補抓的整次上限小於盤中等鎖的上限，
	// 正常情況下盤中輪次⛔ 不會因為補抓被跳過）。
	defaultIntradayLockWait = 60 * time.Second
	defaultRefillLockWait   = 30 * time.Second
	defaultRefillDeadline   = 45 * time.Second

	// IntradayStaleAfter 是首版的延遲門檻（D2）：最後一根有效 K 棒落後這一輪的時刻超過它就標「行情延遲」。
	// 收到比上一次成功評估新、但仍落後超過它的 K 棒：照常計算、⛔ 不當成即時訊號送出（withheld_stale）。
	IntradayStaleAfter = 10 * time.Minute

	// evidenceWriteTimeout 是每一次存證寫入的上限（DB 卡住時⛔ 不拖住盤中這一輪）。
	evidenceWriteTimeout = 10 * time.Second

	// 不評估的原因（eval_cause）：資料狀態本身，或輸入跟上一次成功評估完全相同。
	intradayEvalCauseUnchanged = "unchanged"
	// 重試預算在請求或寫入途中到期（data_cause）。
	intradayCauseRetryBudget = "retry_budget"
)

// 層二的常數（I-123～I-126 計畫書「3.3」）：每批最多重試 2 次（等 10 秒、再等 20 秒），
// 整輪的重試共用 120 秒的預算。
var intradayRetryWaits = []time.Duration{10 * time.Second, 20 * time.Second}

const intradayRetryBudget = 120 * time.Second

// SetIntradayEvidence 注入盤中逐輪存證與讀 K 棒（算輸入指紋）用的 repo。
func (s *Scheduler) SetIntradayEvidence(repo store.IntradayEvidenceRepo, candles store.CandleRepo) {
	s.intradayEvidence = repo
	s.intradayCandles = candles
}

// IntradayRunningRunID 回傳正在執行的那一輪的 run_id（0＝沒有）。狀態 API 用它分「本輪評估中」與「中斷」。
func (s *Scheduler) IntradayRunningRunID() uint64 { return s.intradayRunningRun.Load() }

func (s *Scheduler) intradayNow() time.Time {
	if s.intradayClockFunc != nil {
		return s.intradayClockFunc()
	}
	return time.Now()
}

// acquireIntradayLock 等鎖，最多 wait。拿到時回傳 release。
func (s *Scheduler) acquireIntradayLock(wait time.Duration) (release func(), waited time.Duration, ok bool) {
	start := time.Now()
	t := time.NewTimer(wait)
	defer t.Stop()
	select {
	case s.intradayLock <- struct{}{}:
		return func() { <-s.intradayLock }, time.Since(start), true
	case <-t.C:
		return nil, time.Since(start), false
	}
}

func (s *Scheduler) runIntradayJob() {
	now := s.intradayNow()
	// 13:30 之後的 cron 觸發（13:35～13:55）⛔ 不建立紀錄，一天 55 輪。
	if !timeutil.IsMarketOpen(now) {
		return
	}
	if !s.fetcher.HasIntradaySource() && !s.intradayEnabled {
		// finmind.intraday_enabled=false（預設）且無批次源：帳號等級不足以使用
		// TaiwanStockKBar dataset，不建立 job_run 紀錄，避免每 5 分鐘
		// 洗一筆「skipped」進資料庫；升級帳號或啟用 Yahoo 後即可恢復
		s.log.Debug("intraday job skipped: no batch source and finmind.intraday_enabled=false")
		return
	}
	// 休市日（I-139）：每一輪記一筆 skipped＋market_closed，只寫 job_runs——⛔ 不拿鎖、⛔ 不寫逐輪存證。
	// 有紀錄才分得出「排程有觸發、確認休市」和「排程漏跑」。
	ctx := context.Background()
	if reason, closed := s.marketClosed(ctx, now); closed {
		s.recordMarketClosed(ctx, "intraday", reason)
		return
	}
	s.runIntradayRound(now)
}

// runIntradayRound 跑盤中一輪：等鎖 → 建 run（**先拿鎖再建**，job_runs 的起訖就是實際執行區間，
// 依序執行的兩輪⛔ 不會被誤判成重疊）→ 抓取與寫入 → 評估 → 存證。
// 等鎖逾時：建一筆 skipped（`lock_wait_timeout`），⛔ 不執行、⛔ 不寫逐檔的列。
func (s *Scheduler) runIntradayRound(scheduledAt time.Time) {
	ctx := context.Background()
	release, waited, ok := s.acquireIntradayLock(s.intradayLockWait)
	if !ok {
		runID := s.startRun(ctx, "intraday")
		s.log.Warn("intraday round skipped: lock wait timeout", zap.Uint64("run_id", runID),
			zap.Time("scheduled_at", scheduledAt), zap.Duration("waited", waited))
		r := s.newIntradayRound(runID, store.IntradaySourceScheduled)
		r.insertRound(ctx, scheduledAt, nil, waited, store.IntradayOutcomeSkippedLockTimeout)
		summary := "lock_wait_timeout"
		if r.evidenceFailures > 0 {
			summary += fmt.Sprintf("; evidence_failed:%d", r.evidenceFailures)
		}
		s.finishRunStatus(ctx, runID, "intraday", "skipped", 0, 0, summary)
		return
	}
	defer release()

	runID := s.startRun(ctx, "intraday")
	if runID == 0 {
		// 照跑、照規則評估（⛔ 不因為存證失敗就停掉盤中訊號），只是寫不了逐輪紀錄。
		// 對 I-123 的判讀而言，這一輪在 job_runs 不存在 → 當天資料不足。
		s.log.Error("intraday evidence unavailable: run not created", zap.Time("scheduled_at", scheduledAt))
	}
	s.intradayRunningRun.Store(runID)
	defer s.intradayRunningRun.Store(0)
	s.log.Info("intraday round started", zap.Uint64("run_id", runID),
		zap.Time("scheduled_at", scheduledAt), zap.Duration("lock_wait", waited))

	r := s.newIntradayRound(runID, store.IntradaySourceScheduled)
	lockAt := s.intradayNow()
	r.insertRound(ctx, scheduledAt, &lockAt, waited, store.IntradayOutcomeExecuted)

	if s.fetcher.HasIntradaySource() {
		s.intradayBatchRound(ctx, r)
		return
	}
	s.intradayFinMindRound(ctx, r)
}

// intradayRound 是一輪的存證狀態。
type intradayRound struct {
	s                *Scheduler
	runID            uint64
	source           string
	tally            *jobFailureTally
	evidenceFailures int
	roundInserted    bool
}

func (s *Scheduler) newIntradayRound(runID uint64, source string) *intradayRound {
	return &intradayRound{s: s, runID: runID, source: source, tally: newJobFailureTally()}
}

// evidenceOK：有 run 也有 repo 才寫得了存證。
func (r *intradayRound) evidenceOK() bool { return r.runID != 0 && r.s.intradayEvidence != nil }

func evidenceCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), evidenceWriteTimeout)
}

// evidenceFailed 記一次存證寫失敗：log（原文只進 log）＋計數（⛔ 不讓整輪 success、卻缺逐檔證據）。
func (r *intradayRound) evidenceFailed(stage, symbol string, err error, n int) {
	r.evidenceFailures += n
	r.s.log.Warn("intraday evidence write failed", zap.Uint64("run_id", r.runID), zap.String("symbol", symbol),
		zap.String("stage", stage), zap.String("cause", string(safeJobErrorReason(err))), zap.Int("rows", n), zap.Error(err))
}

func (r *intradayRound) insertRound(ctx context.Context, scheduledAt time.Time, lockAt *time.Time, waited time.Duration, outcome string) {
	if !r.evidenceOK() {
		return
	}
	row := store.IntradayRound{
		RunID: r.runID, Source: r.source, ScheduledAt: scheduledAt,
		LockWaitMs: waited.Milliseconds(), Outcome: outcome,
	}
	if lockAt != nil {
		row.LockAcquiredAt = store.NullTime{NullTime: sql.NullTime{Time: *lockAt, Valid: true}}
	}
	wctx, cancel := evidenceCtx(ctx)
	defer cancel()
	if err := r.s.intradayEvidence.InsertRound(wctx, row); err != nil {
		r.evidenceFailed("round", "", err, 1)
		return
	}
	r.roundInserted = true
}

func (r *intradayRound) finishRound(ctx context.Context) { r.finishRoundAt(ctx, r.s.intradayNow(), "") }

// finishRoundAt 寫回實際結束時刻（手動補抓傳「K 棒寫完、放鎖」的時刻：之後的收尾在鎖外）；outcome 非空時一併改寫。
func (r *intradayRound) finishRoundAt(ctx context.Context, at time.Time, outcome string) {
	if !r.evidenceOK() || !r.roundInserted {
		return
	}
	wctx, cancel := evidenceCtx(ctx)
	defer cancel()
	if err := r.s.intradayEvidence.FinishRound(wctx, r.runID, at, r.evidenceFailures, outcome); err != nil {
		r.evidenceFailed("round", "", err, 1)
	}
}

// failIntradayRoundOnWatchlist：讀不到自選清單時這一輪**明確記成 failed**（review 2026-11-20）。以前用 total=0、failed=0 收尾，
// 狀態推導會得到 success——一輪完全沒抓取、沒評估，卻被 I-123 的判讀算成成功。
func (s *Scheduler) failIntradayRoundOnWatchlist(ctx context.Context, r *intradayRound, err error) {
	s.log.Error("watchlist fetch failed", zap.Uint64("run_id", r.runID), zap.Error(err))
	// 輪次紀錄也記 failed：狀態 API 才分得出「這一輪失敗了」，⛔ 不會沿用上一輪的逐檔結果（review 2026-11-20）。
	r.finishRoundAt(ctx, s.intradayNow(), store.IntradayOutcomeFailed)
	summary := safeJobErrorSummary("watchlist_fetch", err)
	if r.evidenceFailures > 0 {
		summary += fmt.Sprintf("; evidence_failed:%d", r.evidenceFailures)
	}
	s.finishRunStatus(ctx, r.runID, "intraday", "failed", 0, 0, summary)
}

// intradayOutcome 是一檔在這一輪的抓取結果與它所在那一批的嘗試資訊。
type intradayOutcome struct {
	res       market.IntradaySymbolResult
	cause     string // 安全原因碼（data_cause）；written／no_data 是空字串
	attempts  int
	recovered bool
}

func (s *Scheduler) intradayBatchRound(ctx context.Context, r *intradayRound) {
	// 層二的重試預算從一輪開始時起算（retryIntradayBatch）。
	retryCtx, cancelRetry := s.intradayRetryContext(ctx)
	defer cancelRetry()

	symbols, err := s.watchlist.Symbols(ctx)
	if err != nil {
		s.failIntradayRoundOnWatchlist(ctx, r, err)
		return
	}

	batchSize := s.fetcher.IntradayBatchSize()
	if batchSize <= 0 {
		batchSize = len(symbols)
	}

	outcomes := make([]intradayOutcome, 0, len(symbols))
	for start := 0; start < len(symbols); start += batchSize {
		end := start + batchSize
		if end > len(symbols) {
			end = len(symbols)
		}
		batch := symbols[start:end]
		// 每一批的**第一次**抓取⛔ 不受重試預算限制（與導入重試之前相同）。
		results, err := s.fetcher.FetchAndStoreIntradayBatch(ctx, r.runID, batch)
		attempts, recovered := 1, false
		if err != nil {
			rr := s.retryIntradayBatch(retryCtx, r.runID, start, batch, err)
			attempts = rr.attempts
			if !rr.ok {
				// 批次請求失敗（例如 Yahoo 被限流/封鎖）：記錄後續跑其他批次。觸發條件與欄位⛔ 不變（I-123 的觀察工具在用）。
				// 未來的 Yahoo→FinMind fallback（T-008/T-031）會在此改為回退補資料。
				// `cause` 是**第一次**失敗的安全原因碼（err 是第一次抓取的錯誤）。
				s.log.Warn("intraday batch fetch failed", zap.Uint64("run_id", r.runID), zap.Int("from", start),
					zap.Int("size", len(batch)), zap.String("cause", market.TransientCause(err)), zap.Error(err))
			}
			if rr.results != nil {
				// 重試時 HTTP 成功（即使之後預算在寫入途中到期）：逐檔照實記，⛔ 不再整批當成成功或失敗。
				results, recovered = rr.results, true
			} else {
				results = fetchFailedResults(batch, rr.lastErr)
			}
		}
		for _, res := range results {
			outcomes = append(outcomes, intradayOutcome{
				res: res, cause: intradayDataCause(retryCtx, res), attempts: attempts, recovered: recovered,
			})
		}
	}
	s.finishIntradayRound(ctx, r, symbols, outcomes)
}

// intradayFinMindRound 是 FinMind 的逐檔路徑（live 沒有啟用；D4：比照批次路徑的規則）。
func (s *Scheduler) intradayFinMindRound(ctx context.Context, r *intradayRound) {
	symbols, err := s.watchlist.Symbols(ctx)
	if err != nil {
		s.failIntradayRoundOnWatchlist(ctx, r, err)
		return
	}

	today := timeutil.TodayTaipei()
	outcomes := make([]intradayOutcome, 0, len(symbols))
	for i, sym := range symbols {
		res := s.fetcher.FetchAndStoreMinute(ctx, sym, today)
		if res.Status == market.IntradayFetchFailed && errors.Is(res.Err, market.ErrInsufficientTier) {
			// 帳號等級不足是整個 token 的限制，對其他 symbol 重試也一定會失敗，
			// 記一次 log 後整輪跳過，避免每 5 分鐘對 watchlist 每檔股票都打一次注定失敗的請求
			s.log.Warn("intraday job skipped: finmind token tier insufficient", zap.Error(res.Err))
			r.finishRound(ctx)
			s.finishRunStatus(ctx, r.runID, "intraday", "skipped", len(symbols), len(symbols)-i,
				safeJobErrorSummary("finmind_tier", res.Err))
			return
		}
		if res.Err != nil {
			s.log.Warn("intraday fetch failed", zap.String("symbol", sym), zap.String("status", string(res.Status)), zap.Error(res.Err))
		}
		cause := ""
		if res.Err != nil {
			cause = market.TransientCause(res.Err)
		}
		outcomes = append(outcomes, intradayOutcome{res: res, cause: cause, attempts: 1})
	}
	s.finishIntradayRound(ctx, r, symbols, outcomes)
}

// finishIntradayRound 是兩條路徑共用的後半段：記失敗 → 算指紋 → 寫行情段 → 判定與評估 → 寫評估段 → 收尾。
func (s *Scheduler) finishIntradayRound(ctx context.Context, r *intradayRound, symbols []string, outcomes []intradayOutcome) {
	now := s.intradayNow()
	for _, o := range outcomes {
		switch o.res.Status {
		case market.IntradayFetchFailed:
			r.tally.addFetchFailure(o.res.Symbol, o.res.Err)
		case market.IntradayStoreFailed:
			r.tally.addStoreFailure(o.res.Symbol, o.res.Err)
		}
	}

	// 輸入指紋：寫入之後從 DB 讀評估實際會用到的 K 棒。讀不到就留空（→ 一律評估：失敗的方向是多評估）。
	fingerprints := make(map[string]string, len(outcomes))
	for _, o := range outcomes {
		if o.res.Status != market.IntradayWritten {
			continue
		}
		fp, err := s.intradayFingerprint(ctx, o.res.Symbol)
		if err != nil {
			s.log.Warn("intraday fingerprint failed", zap.Uint64("run_id", r.runID), zap.String("symbol", o.res.Symbol), zap.Error(err))
			continue
		}
		fingerprints[o.res.Symbol] = fp
	}

	// 上一次成功評估（只算評估段成功寫入的；評估失敗、補抓、中斷都⛔ 不算）。讀不到就當沒有（→ 評估）。
	lastOK := map[string]store.IntradaySymbolRun{}
	if s.intradayEvidence != nil {
		rctx, cancel := evidenceCtx(ctx)
		got, err := s.intradayEvidence.LastSuccessfulEvals(rctx, symbols)
		cancel()
		if err != nil {
			s.log.Warn("intraday last successful evaluation lookup failed", zap.Uint64("run_id", r.runID), zap.Error(err))
		} else {
			lastOK = got
		}
	}

	// 行情段：一輪一次寫入。
	inserted := false
	if r.evidenceOK() {
		rows := make([]store.IntradaySymbolRun, 0, len(outcomes))
		for _, o := range outcomes {
			rows = append(rows, intradayDataRow(r.runID, r.source, now, o, fingerprints[o.res.Symbol]))
		}
		wctx, cancel := evidenceCtx(ctx)
		err := s.intradayEvidence.InsertSymbolRuns(wctx, rows)
		cancel()
		if err != nil {
			r.evidenceFailed("data", "", err, len(rows))
		} else {
			inserted = true
		}
	}

	// 判定與評估；評估段在那一檔評估完才寫。
	for _, o := range outcomes {
		sym := o.res.Symbol
		status, cause := s.intradayEvaluate(ctx, r, o, fingerprints[sym], lastOK[sym], now)
		if !inserted {
			continue
		}
		wctx, cancel := evidenceCtx(ctx)
		err := s.intradayEvidence.UpdateSymbolEval(wctx, r.runID, sym, s.intradayNow(), status, cause)
		cancel()
		if err != nil {
			// 失敗的方向是「多評估」：下一輪查不到這次的成功評估 → 再評估一次，同一個訊號由判重擋下。
			r.evidenceFailed("eval", sym, err, 1)
		}
	}

	r.finishRound(ctx)
	r.tally.addEvidenceFailures(r.evidenceFailures)
	s.finishRunWithTally(ctx, r.runID, "intraday", len(symbols), r.tally)
}

// intradayEvaluate 依這一輪的資料決定能不能評估，回傳評估結果（eval_status）與原因（eval_cause）。
func (s *Scheduler) intradayEvaluate(
	ctx context.Context, r *intradayRound, o intradayOutcome, fp string, last store.IntradaySymbolRun, now time.Time,
) (string, string) {
	switch o.res.Status {
	case market.IntradayNoData, market.IntradayFetchFailed, market.IntradayStoreFailed:
		// ⛔ 不呼叫訊號引擎：⛔ 不會從舊 K 棒產生或推送訊號，也⛔ 不會被當成「未觸發」。
		return store.IntradayEvalNotEvaluated, string(o.res.Status)
	}
	if fp != "" && last.Fingerprint.Valid && last.Fingerprint.String == fp {
		// 引擎讀到的 K 棒跟上一次成功評估完全相同，結果也相同。
		return store.IntradayEvalNotEvaluated, intradayEvalCauseUnchanged
	}
	opts := signal.EvaluateOptions{}
	if !o.res.LastBarTS.IsZero() && now.Sub(o.res.LastBarTS) > IntradayStaleAfter {
		opts.Withhold = true
	}
	res, err := s.evaluateSymbolWith(ctx, r.tally, o.res.Symbol, "1m", opts)
	if err != nil {
		return store.IntradayEvalFailed, string(safeJobErrorReason(err))
	}
	switch {
	case res == nil:
		return store.IntradayEvalNoSignal, ""
	case res.Degraded:
		stage, _ := res.FirstStageError()
		return store.IntradayEvalDegraded, string(stage)
	case res.Withheld:
		return store.IntradayEvalWithheldStale, ""
	case res.Suppressed:
		return store.IntradayEvalSignalSuppressed, ""
	case res.SignalGenerated:
		return store.IntradayEvalSignalSent, ""
	}
	return store.IntradayEvalNoSignal, ""
}

func intradayDataRow(runID uint64, source string, at time.Time, o intradayOutcome, fp string) store.IntradaySymbolRun {
	row := store.IntradaySymbolRun{
		RunID: runID, Symbol: o.res.Symbol, Source: source, DataAt: at, DataStatus: string(o.res.Status),
		FetchAttempts: o.attempts, HTTPRecovered: o.recovered,
	}
	if o.cause != "" {
		row.DataCause = store.NullString{NullString: sql.NullString{String: o.cause, Valid: true}}
	}
	if !o.res.LastBarTS.IsZero() {
		row.LastBarTS = store.NullTime{NullTime: sql.NullTime{Time: o.res.LastBarTS, Valid: true}}
	}
	if fp != "" {
		row.Fingerprint = store.NullString{NullString: sql.NullString{String: fp, Valid: true}}
	}
	return row
}

// intradayDataCause 是一檔的安全原因碼。**依實際停在哪個階段分類**：重試預算在請求或寫入途中到期記 retry_budget，
// 其他由 market.TransientCause 決定（封閉值域、⛔ 不含錯誤原文）。
func intradayDataCause(retryCtx context.Context, res market.IntradaySymbolResult) string {
	switch res.Status {
	case market.IntradayWritten, market.IntradayNoData:
		return ""
	}
	if res.Err != nil && context.Cause(retryCtx) == errIntradayRetryBudget &&
		(errors.Is(res.Err, context.Canceled) || errors.Is(res.Err, context.DeadlineExceeded) ||
			errors.Is(res.Err, errIntradayRetryBudget)) {
		return intradayCauseRetryBudget
	}
	return market.TransientCause(res.Err)
}

func fetchFailedResults(batch []string, err error) []market.IntradaySymbolResult {
	out := make([]market.IntradaySymbolResult, 0, len(batch))
	for _, sym := range batch {
		out = append(out, market.IntradaySymbolResult{Symbol: sym, Status: market.IntradayFetchFailed, Err: err})
	}
	return out
}

// intradayFingerprint 是這一檔「評估實際會讀到的 K 棒」的 SHA-256：最近 indicator.Lookback 根 1 分 K
// （指標的 120 根涵蓋訊號引擎的 100 根），依時間排序，對每一根的時間與 OHLCV 取雜湊。
// 同時間戳的棒被更新、較早的棒被補回或修訂，指紋都會變。沒有注入 candles 時回空字串（→ 一律評估）。
func (s *Scheduler) intradayFingerprint(ctx context.Context, symbol string) (string, error) {
	if s.intradayCandles == nil {
		return "", nil
	}
	cs, err := s.intradayCandles.GetLatestN(ctx, symbol, "1m", indicator.Lookback)
	if err != nil {
		return "", err
	}
	return candleFingerprint(cs), nil
}

// CandleFingerprint 是 candleFingerprint 的 exported 版本：I-140 的離線重播要用同一個算法核對 live 記錄的指紋（⛔ 不另寫一份）。
func CandleFingerprint(cs []store.Candle) string { return candleFingerprint(cs) }

func candleFingerprint(cs []store.Candle) string {
	sorted := append([]store.Candle(nil), cs...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Timestamp.Before(sorted[j].Timestamp) })
	h := sha256.New()
	f := func(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }
	for _, c := range sorted {
		fmt.Fprintf(h, "%d|%s|%s|%s|%s|%d\n", c.Timestamp.UnixNano(), f(c.Open), f(c.High), f(c.Low), f(c.Close), c.Volume)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// intradayRetryContext 建這一輪的重試 context：「一輪開始 ＋ intradayRetryBudget」到期。
// **重試的等待、Yahoo 節流器的等待、HTTP 請求與寫 K 棒都用它**，所以重試最晚在一輪開始後
// 120 秒結束——⛔ 不是只檢查「要不要開始重試」。重試關閉時回傳原本的 ctx（⛔ 不會用到）。
func (s *Scheduler) intradayRetryContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if !s.retry.enabled() {
		return ctx, func() {}
	}
	return s.retry.budgetContext(ctx, intradayRetryBudget, errIntradayRetryBudget)
}

// intradayRetryResult 是層二重試的結果。
//
//   - ok：舊的「重試成功」條件（HTTP 成功、而且返回時重試 context 還沒到期）。log 的觸發條件跟著它，⛔ 不變。
//   - results：重試時 HTTP 成功拿到的逐檔結果（即使 ok=false：預算在寫入途中到期時，已寫的照實記）。
//     nil 代表回應本身沒拿到。
//   - attempts：這一批的嘗試次數（含第一次）；lastErr：最後一次失敗的錯誤。
type intradayRetryResult struct {
	results  []market.IntradaySymbolResult
	ok       bool
	attempts int
	lastErr  error
}

// retryIntradayBatch 是層二：批次抓取的暫時性失敗在同一輪內重試。永久錯誤（4xx、解析錯誤）⛔ 不重試。
//
// ⚠️ **每一筆 log 都帶 `run_id` 與 `first_cause`**（觀察期的追加修正，I-123～I-126 計畫書「十二」）：
// 重試成功時第一次的錯誤⛔ 不會再出現在任何地方，少了它只證明得了「救回幾次」，
// 看不出救回的是 DNS、連線還是其他暫時性錯誤。原因碼來自 market.TransientCause（封閉值域、⛔ 不含原文）；
// 每次重試自己的錯誤記在 `cause`。
//
// ⚠️ **回傳逐檔結果**（T-088＋I-103）：以前只回布林值，會丟掉逐檔寫入結果；500 重試得到 200 之後，
// A 寫入成功、B 寫入失敗、C 沒有資料要分別記錄。
func (s *Scheduler) retryIntradayBatch(retryCtx context.Context, runID uint64, from int, batch []string, err error) intradayRetryResult {
	out := intradayRetryResult{attempts: 1, lastErr: err}
	if !s.retry.enabled() {
		return out
	}
	firstCause := market.TransientCause(err)
	for attempt, wait := range intradayRetryWaits {
		if !market.IsTransient(err) {
			return out
		}
		if s.retry.wait(retryCtx, wait) != nil {
			s.log.Warn("intraday batch retry stopped: retry budget exhausted",
				zap.Uint64("run_id", runID), zap.Int("from", from), zap.Int("size", len(batch)), zap.Int("retries", attempt),
				zap.String("first_cause", firstCause), zap.String("cause", market.TransientCause(err)))
			return out
		}
		results, ferr := s.fetcher.FetchAndStoreIntradayBatch(retryCtx, runID, batch)
		out.attempts = attempt + 2
		if ferr == nil {
			out.results = results
		} else {
			out.lastErr = ferr
		}
		// **重試成功要同時滿足兩件事**：回傳 nil，而且重試 context 還沒到期——「HTTP 成功、寫 K 棒途中預算到期」
		// ⛔ 不算重試成功（log 照舊）；逐檔結果照實傳回（已寫的 written、寫入階段逾時的 store_failed）。
		if retryCtx.Err() != nil {
			s.log.Warn("intraday batch retry stopped: retry budget exhausted",
				zap.Uint64("run_id", runID), zap.Int("from", from), zap.Int("size", len(batch)), zap.Int("retries", attempt+1),
				zap.String("first_cause", firstCause), zap.String("cause", market.TransientCause(ferr)), zap.Error(ferr))
			return out
		}
		if ferr == nil {
			out.ok = true
			s.log.Info("intraday batch fetch succeeded after retry",
				zap.Uint64("run_id", runID), zap.Int("from", from), zap.Int("size", len(batch)), zap.Int("retries", attempt+1),
				zap.String("first_cause", firstCause))
			return out
		}
		s.log.Warn("intraday batch retry failed",
			zap.Uint64("run_id", runID), zap.Int("from", from), zap.Int("size", len(batch)), zap.Int("retries", attempt+1),
			zap.String("first_cause", firstCause), zap.String("cause", market.TransientCause(ferr)), zap.Error(ferr))
		err = ferr
	}
	return out
}
