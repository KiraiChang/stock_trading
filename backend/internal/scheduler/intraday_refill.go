package scheduler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/market"
	"github.com/trading/backend/internal/store"
	"github.com/trading/backend/pkg/timeutil"
)

// 手動補抓當日分 K（T-088 的 D3；POST /api/v1/scheduler/intraday-refill/run）。
//
//   - 只限**當日**、自選清單（可指定其中幾檔）；**只寫 K 棒、⛔ 不評估訊號**，收盤後也可以執行（13:30 最後一輪失敗時用）。
//   - ⛔ **不推進「上一次成功評估」**：補抓的列沒有評估段，下一輪指紋不同就會照常評估。
//   - 跟盤中一輪共用同一把鎖、依序執行；等鎖 30 秒、整次 45 秒（小於盤中等鎖的 60 秒）。
//   - 回報補了什麼：新增／更新／不變的根數，與仍無法確認的缺口。最後一根時間與總根數只證明寫入進度，⛔ 不當成「已補齊」的證據。
//   - 權限沿用現有手動排程端點（任何登入的使用者）；⚠️ 多人使用之前要調整（D6，見 docs/api-reference.md）。

const intradayRefillJobName = "intraday_refill"

// 補抓整次逾時的原因碼（data_cause 與 job_runs 摘要）。
const intradayCauseRefillDeadline = "refill_deadline"

// 收盤前 5 分鐘是收盤集合競價（13:25～13:30 之間不撮合），那幾分鐘沒有 K 棒是正常的，⛔ 不列為缺口。
const (
	refillSessionOpenMin  = 9 * 60
	refillCallAuctionFrom = 13*60 + 25
	refillSessionCloseMin = 13*60 + 30
	refillGapListCap      = 20
)

var (
	ErrIntradayRefillBusy          = errors.New("intraday refill: another intraday run is in progress")
	ErrIntradayRefillUnavailable   = errors.New("intraday refill: no intraday batch source configured")
	ErrIntradayRefillUnknownSymbol = errors.New("intraday refill: symbol is not in the watchlist")
)

// IntradayRefillSymbol 是一檔的補抓結果。
type IntradayRefillSymbol struct {
	Symbol string `json:"symbol"`
	// Status：written／no_data／store_failed／fetch_failed（逾時還沒處理到的檔是 fetch_failed＋refill_deadline，即「未完成」）。
	Status string `json:"status"`
	Cause  string `json:"cause,omitempty"`
	// 跟補抓前 DB 裡的棒比對：新增（原本沒有）、更新（同時間戳、值不同）、不變。
	Inserted  int `json:"inserted"`
	Updated   int `json:"updated"`
	Unchanged int `json:"unchanged"`
	// 補抓之後 DB 裡當日的最後一根與根數（寫入進度，⛔ 不等於已補齊）。
	LastBarTS *time.Time `json:"last_bar_ts"`
	BarsToday int        `json:"bars_today"`
	// 仍無法確認的缺口：交易時段內（09:00～13:25 與 13:30）DB 仍沒有 K 棒的分鐘。
	// 可能是當下無成交，也可能是資料缺漏，⛔ 不判定是哪一種。
	GapMinutes    int      `json:"gap_minutes"`
	Gaps          []string `json:"gaps"`
	GapsTruncated bool     `json:"gaps_truncated"`
}

// IntradayRefillReport 是一次補抓的結果。
type IntradayRefillReport struct {
	RunID      uint64                 `json:"run_id"`
	WindowFrom time.Time              `json:"window_from"`
	WindowTo   time.Time              `json:"window_to"`
	Deadline   bool                   `json:"deadline_exceeded"`
	Symbols    []IntradayRefillSymbol `json:"symbols"`
}

// RunIntradayRefill 執行一次手動補抓。symbols 空＝整份自選清單。
func (s *Scheduler) RunIntradayRefill(ctx context.Context, symbols []string) (*IntradayRefillReport, error) {
	if !s.fetcher.HasIntradaySource() {
		return nil, ErrIntradayRefillUnavailable
	}
	watch, err := s.watchlist.Symbols(ctx)
	if err != nil {
		return nil, fmt.Errorf("watchlist fetch failed: %w", err)
	}
	targets, err := refillTargets(watch, symbols)
	if err != nil {
		return nil, err
	}

	requestedAt := s.intradayNow()
	release, waited, ok := s.acquireIntradayLock(s.refillLockWait)
	if !ok {
		return nil, ErrIntradayRefillBusy
	}
	// **整段持鎖都在 45 秒的期限內**（review 2026-11-20）：期限從拿到鎖就開始算，建 run、讀既有的棒、抓取與寫 K 棒都用它；
	// 寫完 K 棒就放鎖，存證與 job_runs 的收尾（各自有重試與上限）在鎖外做——收尾卡住⛔ 不會讓下一輪盤中排程等鎖逾時。
	released := false
	unlock := func() {
		if !released {
			released = true
			release()
		}
	}
	defer unlock()
	lockAt := s.intradayNow()
	dctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.refillDeadline)
	defer cancel()

	// ⛔ 不用 startRun：它有自己的 10 秒上限、⛔ 不受補抓期限約束。這裡直接用期限內的 ctx 建 run，建不了就照跑（沒有存證）。
	runID, err := s.jobRuns.Start(dctx, intradayRefillJobName)
	if err != nil {
		s.log.Error("job_runs start failed", zap.String("job", intradayRefillJobName), zap.Error(err))
		runID = 0
	}
	r := s.newIntradayRound(runID, store.IntradaySourceRefill)
	now := lockAt

	local := now.In(timeutil.TaipeiTZ)
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, timeutil.TaipeiTZ)
	from := day.Add(refillSessionOpenMin * time.Minute)
	to := day.Add(refillSessionCloseMin * time.Minute)
	if local.Before(to) {
		to = local.Truncate(time.Minute)
	}
	report := &IntradayRefillReport{RunID: runID, WindowFrom: from, WindowTo: to, Symbols: []IntradayRefillSymbol{}}

	batchSize := s.fetcher.IntradayBatchSize()
	if batchSize <= 0 {
		batchSize = len(targets)
	}
	outcomes := make([]intradayOutcome, 0, len(targets))
	for start := 0; start < len(targets); start += batchSize {
		end := start + batchSize
		if end > len(targets) {
			end = len(targets)
		}
		batch := targets[start:end]
		if dctx.Err() != nil {
			for _, sym := range batch {
				o := intradayOutcome{res: market.IntradaySymbolResult{Symbol: sym, Status: market.IntradayFetchFailed, Err: dctx.Err()},
					cause: intradayCauseRefillDeadline, attempts: 1}
				outcomes = append(outcomes, o)
				report.Symbols = append(report.Symbols, IntradayRefillSymbol{Symbol: sym, Status: string(o.res.Status), Cause: o.cause, Gaps: []string{}})
			}
			continue
		}
		outs, rows := s.refillBatch(dctx, runID, batch, day, from, to)
		outcomes = append(outcomes, outs...)
		report.Symbols = append(report.Symbols, rows...)
	}
	report.Deadline = dctx.Err() != nil
	// K 棒寫完就放鎖；以下的存證與收尾在鎖外。
	execFinished := s.intradayNow()
	unlock()
	r.insertRound(ctx, requestedAt, &lockAt, waited, store.IntradayOutcomeExecuted)

	for _, o := range outcomes {
		switch o.res.Status {
		case market.IntradayFetchFailed:
			r.tally.addFetchFailure(o.res.Symbol, o.res.Err)
		case market.IntradayStoreFailed:
			r.tally.addStoreFailure(o.res.Symbol, o.res.Err)
		}
	}
	// 行情段（source=refill），⛔ 沒有評估段——不推進上一次成功評估。
	if r.evidenceOK() {
		rows := make([]store.IntradaySymbolRun, 0, len(outcomes))
		for _, o := range outcomes {
			rows = append(rows, intradayDataRow(runID, store.IntradaySourceRefill, now, o, ""))
		}
		wctx, wcancel := evidenceCtx(ctx)
		if err := s.intradayEvidence.InsertSymbolRuns(wctx, rows); err != nil {
			r.evidenceFailed("data", "", err, len(rows))
		}
		wcancel()
	}
	r.finishRoundAt(ctx, execFinished, "")
	r.tally.addEvidenceFailures(r.evidenceFailures)

	if report.Deadline {
		summary := intradayCauseRefillDeadline
		if sum := r.tally.summary(); sum != "" {
			summary += "; " + sum
		}
		s.finishRunStatus(ctx, runID, intradayRefillJobName, "failed", len(targets), r.tally.failedCount(), summary)
	} else {
		s.finishRunWithTally(ctx, runID, intradayRefillJobName, len(targets), r.tally)
	}
	s.log.Info("intraday refill done", zap.Uint64("run_id", runID), zap.Int("symbols", len(targets)),
		zap.Int("failed", r.tally.failedCount()), zap.Bool("deadline_exceeded", report.Deadline))
	return report, nil
}

// refillTargets 驗證要補抓的標的都在自選清單裡；空＝整份清單。保留請求的順序、去重。
func refillTargets(watch, requested []string) ([]string, error) {
	if len(requested) == 0 {
		return append([]string(nil), watch...), nil
	}
	in := make(map[string]bool, len(watch))
	for _, w := range watch {
		in[w] = true
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(requested))
	for _, sym := range requested {
		if !in[sym] {
			return nil, fmt.Errorf("%w: %s", ErrIntradayRefillUnknownSymbol, sym)
		}
		if !seen[sym] {
			seen[sym] = true
			out = append(out, sym)
		}
	}
	return out, nil
}

// refillBatch 補抓一批：先讀 DB 既有的棒 → 抓取 → 只留當日的棒、跟既有的比對 → 寫入 → 重讀算缺口。
func (s *Scheduler) refillBatch(
	ctx context.Context, runID uint64, batch []string, day, from, to time.Time,
) ([]intradayOutcome, []IntradayRefillSymbol) {
	before := make(map[string]map[int64]store.Candle, len(batch))
	if s.intradayCandles != nil {
		for _, sym := range batch {
			cs, err := s.intradayCandles.GetRange(ctx, sym, "1m", from, to.Add(time.Minute-time.Nanosecond))
			if err != nil {
				s.log.Warn("intraday refill: read existing candles failed", zap.String("symbol", sym), zap.Error(err))
				continue
			}
			m := make(map[int64]store.Candle, len(cs))
			for _, c := range cs {
				m[c.Timestamp.Unix()] = c
			}
			before[sym] = m
		}
	}

	outcomes := make([]intradayOutcome, 0, len(batch))
	rows := make([]IntradayRefillSymbol, 0, len(batch))
	fetched, err := s.fetcher.FetchIntradayBatchCandles(ctx, batch)
	if err != nil {
		cause := market.TransientCause(err)
		if ctx.Err() != nil {
			cause = intradayCauseRefillDeadline
		}
		for _, sym := range batch {
			outcomes = append(outcomes, intradayOutcome{res: market.IntradaySymbolResult{Symbol: sym, Status: market.IntradayFetchFailed, Err: err},
				cause: cause, attempts: 1})
			rows = append(rows, IntradayRefillSymbol{Symbol: sym, Status: string(market.IntradayFetchFailed), Cause: cause, Gaps: []string{}})
		}
		return outcomes, rows
	}

	today := make(map[string][]market.Candle, len(fetched))
	counts := make(map[string][3]int, len(batch))
	for _, sym := range batch {
		var keep []market.Candle
		var ins, upd, same int
		for _, c := range fetched[sym] {
			ts := c.Timestamp.In(timeutil.TaipeiTZ)
			if ts.Year() != day.Year() || ts.YearDay() != day.YearDay() {
				continue // 只補當日
			}
			keep = append(keep, c)
			old, ok := before[sym][c.Timestamp.Unix()]
			switch {
			case !ok:
				ins++
			case old.Open != c.Open || old.High != c.High || old.Low != c.Low || old.Close != c.Close || old.Volume != c.Volume:
				upd++
			default:
				same++
			}
		}
		today[sym] = keep
		counts[sym] = [3]int{ins, upd, same}
	}

	results := s.fetcher.StoreIntradayBatch(ctx, runID, batch, today)
	for _, res := range results {
		o := intradayOutcome{res: res, attempts: 1}
		if res.Err != nil {
			o.cause = market.TransientCause(res.Err)
			if ctx.Err() != nil && (errors.Is(res.Err, context.DeadlineExceeded) || errors.Is(res.Err, context.Canceled)) {
				o.cause = intradayCauseRefillDeadline
			}
		}
		outcomes = append(outcomes, o)
		row := IntradayRefillSymbol{Symbol: res.Symbol, Status: string(res.Status), Cause: o.cause, Gaps: []string{}}
		if res.Status == market.IntradayWritten {
			c := counts[res.Symbol]
			row.Inserted, row.Updated, row.Unchanged = c[0], c[1], c[2]
		}
		s.refillAfterState(ctx, &row, from, to)
		rows = append(rows, row)
	}
	return outcomes, rows
}

// refillAfterState 補抓之後重讀 DB：當日的最後一根、根數與仍無法確認的缺口。
func (s *Scheduler) refillAfterState(ctx context.Context, row *IntradayRefillSymbol, from, to time.Time) {
	if s.intradayCandles == nil || to.Before(from) {
		return
	}
	cs, err := s.intradayCandles.GetRange(ctx, row.Symbol, "1m", from, to.Add(time.Minute-time.Nanosecond))
	if err != nil {
		s.log.Warn("intraday refill: read candles after refill failed", zap.String("symbol", row.Symbol), zap.Error(err))
		return
	}
	have := make(map[int64]bool, len(cs))
	for _, c := range cs {
		have[c.Timestamp.Truncate(time.Minute).Unix()] = true
		if row.LastBarTS == nil || c.Timestamp.After(*row.LastBarTS) {
			ts := c.Timestamp
			row.LastBarTS = &ts
		}
	}
	row.BarsToday = len(cs)
	row.Gaps, row.GapMinutes, row.GapsTruncated = refillGaps(have, from, to)
}

// refillGaps 列出 [from, to] 之間（排除收盤集合競價的 13:26～13:29）DB 沒有 K 棒的分鐘，連續的合併成一段。
func refillGaps(have map[int64]bool, from, to time.Time) ([]string, int, bool) {
	gaps := []string{}
	total := 0
	truncated := false
	var runStart, runEnd time.Time
	flush := func() {
		if runStart.IsZero() {
			return
		}
		if len(gaps) >= refillGapListCap {
			truncated = true
		} else if runStart.Equal(runEnd) {
			gaps = append(gaps, runStart.In(timeutil.TaipeiTZ).Format("15:04"))
		} else {
			gaps = append(gaps, runStart.In(timeutil.TaipeiTZ).Format("15:04")+"-"+runEnd.In(timeutil.TaipeiTZ).Format("15:04"))
		}
		runStart, runEnd = time.Time{}, time.Time{}
	}
	for t := from; !t.After(to); t = t.Add(time.Minute) {
		local := t.In(timeutil.TaipeiTZ)
		m := local.Hour()*60 + local.Minute()
		if m > refillCallAuctionFrom && m < refillSessionCloseMin {
			flush()
			continue
		}
		if have[t.Unix()] {
			flush()
			continue
		}
		total++
		if runStart.IsZero() {
			runStart = t
		}
		runEnd = t
	}
	flush()
	return gaps, total, truncated
}
