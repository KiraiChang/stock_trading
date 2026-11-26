// Package tradingday 是排程與 API 共用的「今天是不是交易日」判斷（I-139）。
//
// 現況規格見 docs/architecture.md 的「休市日的行為」。判斷順序：週末 → 手動設定 → TWSE 年度日曆 → 取不到時照平日（fail-open）。
package tradingday

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/market"
	"github.com/trading/backend/internal/store"
	"github.com/trading/backend/pkg/timeutil"
)

// 判斷依據（Result.Source，封閉值域）。
const (
	// SourceWeekend：週六、週日一律休市，⛔ 不查日曆、⛔ 不看手動設定（補行交易日是已知限制）。
	SourceWeekend = "weekend"
	// SourceManual：當天有生效中的手動設定（休市或照常開市）。
	SourceManual = "manual"
	// SourceCalendar：日曆取得成功，依日曆判斷。
	SourceCalendar = "calendar"
	// SourceFallback：日曆取不到（逾時、請求失敗、breaker 開啟、年份驗證失敗），一律當成交易日。
	SourceFallback = "fallback"
)

const (
	// DefaultCalendarTimeout 是共用日曆請求自己的上限（⛔ 不沿用呼叫端的 context 等到底）。
	DefaultCalendarTimeout = 5 * time.Second
	// DefaultDegradedFor 是失敗結果共用的時間：這段時間內所有呼叫端直接用降級結果，⛔ 不再請求、⛔ 不再等待。
	DefaultDegradedFor = 5 * time.Minute
	// DefaultOverrideTimeout 是讀手動設定的上限（v1.2 實作 review）：cron 傳進來的 context 沒有期限，
	// DB 卡住時⛔ 不能讓判斷一直等、排程也到不了日曆與 fallback。逾時照日曆判斷。
	DefaultOverrideTimeout = 2 * time.Second
)

// Result 是一次判斷的結果。Override 只在 Source=manual 時帶。
type Result struct {
	IsTrading bool                     `json:"is_trading"`
	Source    string                   `json:"source"`
	Override  *store.MarketDayOverride `json:"override,omitempty"`
	// OverrideUnavailable：讀手動設定失敗或逾時（T-089）。這時照日曆（或 fallback）判斷、Source 仍是 calendar／fallback，
	// 只是那一天⛔ 沒有確認過手動設定。判斷結果與排程的降級行為⛔ 不因它改變；`json:"-"`，⛔ 不出現在既有 API 的回應。
	OverrideUnavailable bool `json:"-"`
}

// CalendarSource 是年度日曆的來源（market.ExchangeReference 滿足它）。
type CalendarSource interface {
	TradingCalendar(ctx context.Context, year int) (*market.TradingCalendar, error)
}

// OverrideReader 讀某一天最新的手動設定（store.MarketDayOverrideRepo 滿足它）。
type OverrideReader interface {
	Latest(ctx context.Context, tradeDate string) (*store.MarketDayOverride, error)
}

// Judge 是共用的判斷。main 建立一個實例，scheduler 與 API handler 共用（同一份快取與降級狀態）。
type Judge struct {
	cal       CalendarSource
	overrides OverrideReader
	log       *zap.Logger

	timeout         time.Duration
	degradedFor     time.Duration
	overrideTimeout time.Duration
	now             func() time.Time

	mu         sync.Mutex
	flights    map[int]*calendarCall
	degraded   map[int]time.Time // 年度 → 降級結果有效到什麼時候
	inFallback bool              // 目前是不是在降級中（只在狀態轉換時記 log）
}

type calendarCall struct {
	done chan struct{}
	cal  *market.TradingCalendar
	err  error
}

// New 建立判斷。cal 或 overrides 是 nil 時，那一項視為沒有（日曆 nil → 一律 fallback；overrides nil → 沒有手動設定）。
func New(cal CalendarSource, overrides OverrideReader, log *zap.Logger) *Judge {
	if log == nil {
		log = zap.NewNop()
	}
	return &Judge{
		cal: cal, overrides: overrides, log: log,
		timeout: DefaultCalendarTimeout, degradedFor: DefaultDegradedFor, overrideTimeout: DefaultOverrideTimeout, now: time.Now,
		flights: map[int]*calendarCall{}, degraded: map[int]time.Time{},
	}
}

// SetTimingForTest 是測試接縫；production ⛔ 不呼叫。
func (j *Judge) SetTimingForTest(timeout, degradedFor time.Duration, now func() time.Time) {
	j.timeout, j.degradedFor = timeout, degradedFor
	if now != nil {
		j.now = now
	}
}

// SetOverrideTimeoutForTest 是測試接縫；production ⛔ 不呼叫。
func (j *Judge) SetOverrideTimeoutForTest(d time.Duration) { j.overrideTimeout = d }

// Today 判斷今天（台北）。
func (j *Judge) Today(ctx context.Context) Result { return j.On(ctx, j.now()) }

// Now 是判斷用的時鐘（測試可換）；呼叫端要用同一個時鐘算「今天」時用它。
func (j *Judge) Now() time.Time { return j.now() }

// On 判斷 day（取台北的日曆日期）。
func (j *Judge) On(ctx context.Context, day time.Time) Result {
	local := day.In(timeutil.TaipeiTZ)
	d := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, timeutil.TaipeiTZ)
	if wd := d.Weekday(); wd == time.Saturday || wd == time.Sunday {
		return Result{IsTrading: false, Source: SourceWeekend}
	}
	// 手動設定每次都重讀（⛔ 不快取）：設定與取消在下一次判斷就生效。
	overrideUnavailable := false
	if j.overrides != nil {
		o, err := j.readOverride(ctx, d.Format("2006-01-02"))
		switch {
		case err != nil:
			// 讀不到手動設定時照日曆判斷（DB 讀不到時排程本身大多也跑不了）。
			j.log.Warn("trading day override unavailable, using calendar", zap.Error(err))
			overrideUnavailable = true
		case o.Effective():
			return Result{IsTrading: o.Action == store.MarketDayOpen, Source: SourceManual, Override: o}
		}
	}
	cal, ok := j.calendar(ctx, d.Year())
	if !ok {
		return Result{IsTrading: true, Source: SourceFallback, OverrideUnavailable: overrideUnavailable}
	}
	return Result{IsTrading: cal.IsTradingDay(d), Source: SourceCalendar, OverrideUnavailable: overrideUnavailable}
}

type overrideResult struct {
	o   *store.MarketDayOverride
	err error
}

var errOverrideTimeout = judgeError("trading day override read timed out")

// overrideWaitHook 是測試接縫（production 是 nil）：讓讀取端先跑完，重現「結果與期限同時就緒」的情況。
var overrideWaitHook func()

// readOverride 讀手動設定，最多等 overrideTimeout。查詢本身帶有期限的 context；萬一來源⛔ 沒有響應取消，
// 呼叫端也只等到期限為止（那次查詢的結果直接丟掉）。
//
// **cancel 只由等候端呼叫**（實作 review 第二輪）：讀取的 goroutine 送出結果後若自己 cancel，等候端的 select 可能同時看到
// 「結果」與「取消」而隨機走到逾時分支，把成功讀到的手動設定丟掉。期限到時也先看一次結果有沒有已經送到。
func (j *Judge) readOverride(ctx context.Context, date string) (*store.MarketDayOverride, error) {
	octx, cancel := context.WithTimeout(ctx, j.overrideTimeout)
	defer cancel()
	done := make(chan overrideResult, 1)
	go func() {
		o, err := j.overrides.Latest(octx, date)
		done <- overrideResult{o, err}
	}()
	if overrideWaitHook != nil {
		overrideWaitHook()
	}
	select {
	case r := <-done:
		return r.o, r.err
	case <-octx.Done():
		select {
		case r := <-done:
			return r.o, r.err
		default:
		}
		if ctx.Err() == nil {
			return nil, errOverrideTimeout
		}
		return nil, ctx.Err()
	}
}

// calendar 取年度日曆。同一年同一時間只有一個共用請求在飛；請求跟發起它的呼叫端脫鉤（自己的 context 與逾時），
// 呼叫端只用自己的 ctx 等結果——呼叫端先被取消只影響它自己（回 false），⛔ 不算日曆失敗、⛔ 不寫降級結果。
func (j *Judge) calendar(ctx context.Context, year int) (*market.TradingCalendar, bool) {
	if j.cal == nil {
		return nil, false
	}
	j.mu.Lock()
	if until, ok := j.degraded[year]; ok && j.now().Before(until) {
		j.mu.Unlock()
		return nil, false
	}
	call := j.flights[year]
	if call == nil {
		call = &calendarCall{done: make(chan struct{})}
		j.flights[year] = call
		go j.fetch(context.WithoutCancel(ctx), year, call)
	}
	j.mu.Unlock()

	// 共用請求自己有 timeout 的期限；萬一來源⛔ 沒有響應取消，呼叫端最多等兩倍期限就放棄，並把降級結果寫下來，
	// 後面的呼叫端⛔ 不用再各等一次（那次請求之後成功回來時會清掉降級）。
	limit := time.NewTimer(2 * j.timeout)
	defer limit.Stop()
	select {
	case <-call.done:
		return call.cal, call.err == nil
	case <-ctx.Done():
		return nil, false
	case <-limit.C:
		j.mu.Lock()
		if j.flights[year] == call {
			// 卡住的請求移出共用請求（實作 review 第二輪）：降級期滿後的下一次判斷要能重新探測，⛔ 不再等同一筆；
			// 它之後晚回來的結果也⛔ 不會影響判斷（fetch 只在自己仍是目前的共用請求時才更新狀態）。
			delete(j.flights, year)
			j.markDegradedLocked(year, errCalendarStuck)
		}
		j.mu.Unlock()
		return nil, false
	}
}

var errCalendarStuck = judgeError("trading calendar request did not return within its deadline")

// markDegradedLocked 寫下降級結果並在進入降級時記一次 Warn。呼叫端要持有 j.mu。
func (j *Judge) markDegradedLocked(year int, err error) {
	j.degraded[year] = j.now().Add(j.degradedFor)
	if !j.inFallback {
		j.inFallback = true
		j.log.Warn("trading calendar unavailable", zap.Int("year", year),
			zap.String("cause", market.TransientCause(err)), zap.Error(err))
	}
}

func (j *Judge) fetch(base context.Context, year int, call *calendarCall) {
	fctx, cancel := context.WithTimeout(base, j.timeout)
	defer cancel()
	cal, err := j.cal.TradingCalendar(fctx, year)
	if err == nil && cal == nil {
		err = errNilCalendar
	}

	j.mu.Lock()
	if j.flights[year] != call {
		// 已經被放棄的舊請求：只通知還在等它的呼叫端，⛔ 不更新降級狀態與 log（避免晚回來的成功清掉新一輪的降級，或反過來）。
		call.cal, call.err = cal, err
		close(call.done)
		j.mu.Unlock()
		return
	}
	delete(j.flights, year)
	if err != nil {
		j.markDegradedLocked(year, err)
	} else {
		delete(j.degraded, year)
		if j.inFallback {
			j.inFallback = false
			j.log.Info("trading calendar recovered", zap.Int("year", year))
		}
	}
	call.cal, call.err = cal, err
	close(call.done)
	j.mu.Unlock()
}

type judgeError string

func (e judgeError) Error() string { return string(e) }

const errNilCalendar = judgeError("trading calendar source returned nil")
