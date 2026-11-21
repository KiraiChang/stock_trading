package tradingday

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/trading/backend/internal/market"
	"github.com/trading/backend/internal/store"
	"github.com/trading/backend/pkg/timeutil"
)

type fakeCal struct {
	calls   atomic.Int32
	err     error
	year    int
	closed  map[string]bool
	block   chan struct{} // 非 nil：等它關閉（或請求的 ctx 結束）才回
	started chan struct{} // 非 nil：開始請求時送一個訊號
}

func (f *fakeCal) TradingCalendar(ctx context.Context, year int) (*market.TradingCalendar, error) {
	f.calls.Add(1)
	if f.started != nil {
		f.started <- struct{}{}
	}
	if f.block != nil {
		select {
		case <-f.block:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if f.err != nil {
		return nil, f.err
	}
	y := year
	if f.year != 0 {
		y = f.year
	}
	return market.NewTradingCalendarForTest(y, f.closed), nil
}

type fakeOverrides struct {
	mu  sync.Mutex
	m   map[string]*store.MarketDayOverride
	err error
}

func (f *fakeOverrides) Latest(_ context.Context, d string) (*store.MarketDayOverride, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	return f.m[d], nil
}

func (f *fakeOverrides) set(d, action string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.m == nil {
		f.m = map[string]*store.MarketDayOverride{}
	}
	f.m[d] = &store.MarketDayOverride{TradeDate: d, Action: action, Reason: "test", UserID: 1}
}

func tpe(y int, m time.Month, d, hh, mm int) time.Time {
	return time.Date(y, m, d, hh, mm, 0, 0, timeutil.TaipeiTZ)
}

var (
	monday   = tpe(2026, 11, 23, 9, 0)  // 交易日
	holiday  = tpe(2026, 12, 25, 9, 0)  // 日曆的休市日（週五）
	saturday = tpe(2026, 11, 28, 10, 0) // 週六
)

func newJudge(cal CalendarSource, ov OverrideReader, now time.Time) (*Judge, *observer.ObservedLogs) {
	core, logs := observer.New(zap.InfoLevel)
	j := New(cal, ov, zap.New(core))
	clock := now
	j.SetTimingForTest(50*time.Millisecond, 5*time.Minute, func() time.Time { return clock })
	return j, logs
}

func TestJudgeCalendarAndWeekend(t *testing.T) {
	ctx := context.Background()
	cal := &fakeCal{closed: map[string]bool{"2026-12-25": true}}
	j, _ := newJudge(cal, nil, monday)

	if r := j.On(ctx, monday); !r.IsTrading || r.Source != SourceCalendar {
		t.Fatalf("交易日要是 true/calendar，得到 %+v", r)
	}
	if r := j.On(ctx, holiday); r.IsTrading || r.Source != SourceCalendar {
		t.Fatalf("日曆的休市日要是 false/calendar，得到 %+v", r)
	}
	if r := j.On(ctx, saturday); r.IsTrading || r.Source != SourceWeekend {
		t.Fatalf("週六要是 false/weekend，得到 %+v", r)
	}
	// 台北日期：UTC 前一天 16:30 已經是台北的週一
	if r := j.On(ctx, time.Date(2026, 11, 22, 16, 30, 0, 0, time.UTC)); !r.IsTrading || r.Source != SourceCalendar {
		t.Fatalf("要用台北的日曆日期判斷，得到 %+v", r)
	}
	if r := j.Today(ctx); !r.IsTrading {
		t.Fatalf("Today 用注入的時鐘（週一），得到 %+v", r)
	}
}

func TestJudgeWeekendIgnoresOverridesAndCalendar(t *testing.T) {
	ov := &fakeOverrides{}
	ov.set("2026-11-28", store.MarketDayOpen)
	cal := &fakeCal{err: errors.New("down")}
	j, _ := newJudge(cal, ov, saturday)
	if r := j.On(context.Background(), saturday); r.IsTrading || r.Source != SourceWeekend {
		t.Fatalf("週末一律休市（有手動開市、日曆取不到時也一樣），得到 %+v", r)
	}
	if cal.calls.Load() != 0 {
		t.Fatal("週末⛔ 不查日曆")
	}
}

func TestJudgeManualOverrides(t *testing.T) {
	ctx := context.Background()
	ov := &fakeOverrides{}
	cal := &fakeCal{closed: map[string]bool{"2026-12-25": true}}
	j, _ := newJudge(cal, ov, monday)

	ov.set("2026-11-23", store.MarketDayClosed)
	if r := j.On(ctx, monday); r.IsTrading || r.Source != SourceManual || r.Override == nil {
		t.Fatalf("手動休市要推翻日曆的交易日，得到 %+v", r)
	}
	ov.set("2026-12-25", store.MarketDayOpen)
	if r := j.On(ctx, holiday); !r.IsTrading || r.Source != SourceManual {
		t.Fatalf("手動開市要推翻日曆的休市日，得到 %+v", r)
	}
	// 取消之後下一次判斷就回到日曆（⛔ 不等快取）
	ov.set("2026-11-23", store.MarketDayCleared)
	if r := j.On(ctx, monday); !r.IsTrading || r.Source != SourceCalendar || r.Override != nil {
		t.Fatalf("最新一列是 cleared 要回到日曆，得到 %+v", r)
	}
	// 讀不到手動設定：照日曆判斷
	ov.err = errors.New("db down")
	if r := j.On(ctx, monday); !r.IsTrading || r.Source != SourceCalendar {
		t.Fatalf("讀不到手動設定時照日曆判斷，得到 %+v", r)
	}
}

func TestJudgeFallbackSharedAndRetried(t *testing.T) {
	ctx := context.Background()
	now := monday
	cal := &fakeCal{err: errors.New("twse 503")}
	core, logs := observer.New(zap.InfoLevel)
	j := New(cal, nil, zap.New(core))
	j.SetTimingForTest(50*time.Millisecond, 5*time.Minute, func() time.Time { return now })

	if r := j.On(ctx, monday); !r.IsTrading || r.Source != SourceFallback {
		t.Fatalf("日曆取不到要 fail-open：true/fallback，得到 %+v", r)
	}
	for i := 0; i < 3; i++ {
		if r := j.On(ctx, monday); r.Source != SourceFallback {
			t.Fatalf("5 分鐘內共用降級結果，得到 %+v", r)
		}
	}
	if n := cal.calls.Load(); n != 1 {
		t.Fatalf("5 分鐘內⛔ 不再請求，實際請求 %d 次", n)
	}
	now = now.Add(5*time.Minute + time.Second)
	j.On(ctx, monday)
	if n := cal.calls.Load(); n != 2 {
		t.Fatalf("過了 5 分鐘才重試，實際請求 %d 次", n)
	}
	if n := logs.FilterMessage("trading calendar unavailable").Len(); n != 1 {
		t.Fatalf("Warn 只在進入 fallback 時記一次，實際 %d 次", n)
	}
	// 恢復：記 Info、回到日曆
	cal.err = nil
	now = now.Add(5*time.Minute + time.Second)
	if r := j.On(ctx, monday); r.Source != SourceCalendar {
		t.Fatalf("恢復後要回到日曆，得到 %+v", r)
	}
	if logs.FilterMessage("trading calendar recovered").Len() != 1 {
		t.Fatal("恢復時要記一次 Info")
	}
}

func TestJudgeWrongYearIsFallback(t *testing.T) {
	// fetchCalendar 驗年份失敗時回錯誤（market 的既有測試）；這裡確認錯誤一律走 fallback
	cal := &fakeCal{err: errors.New("calendar year mismatch")}
	j, _ := newJudge(cal, nil, monday)
	if r := j.On(context.Background(), monday); r.Source != SourceFallback || !r.IsTrading {
		t.Fatalf("年份驗證失敗要 fallback，得到 %+v", r)
	}
}

func TestJudgeCalendarTimeout(t *testing.T) {
	cal := &fakeCal{block: make(chan struct{})} // 永遠卡住，只能靠逾時
	defer close(cal.block)
	j, _ := newJudge(cal, nil, monday)
	start := time.Now()
	r := j.On(context.Background(), monday)
	if r.Source != SourceFallback || !r.IsTrading {
		t.Fatalf("日曆卡住要在逾時後回 true/fallback，得到 %+v", r)
	}
	if el := time.Since(start); el > 2*time.Second {
		t.Fatalf("要在逾時（測試設 50ms）內回，實際 %v", el)
	}
}

func TestJudgeSingleFlight(t *testing.T) {
	release := make(chan struct{})
	cal := &fakeCal{block: release, started: make(chan struct{}, 10)}
	j, _ := newJudge(cal, nil, monday)
	j.SetTimingForTest(5*time.Second, 5*time.Minute, nil)

	var wg sync.WaitGroup
	results := make([]Result, 5)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = j.On(context.Background(), monday)
		}(i)
	}
	<-cal.started
	time.Sleep(50 * time.Millisecond) // 讓其他呼叫端都進到等待
	close(release)
	wg.Wait()
	if n := cal.calls.Load(); n != 1 {
		t.Fatalf("同時多個呼叫端只發一個請求，實際 %d 次", n)
	}
	for i, r := range results {
		if r.Source != SourceCalendar || !r.IsTrading {
			t.Fatalf("呼叫端 %d 要拿到日曆的結果，得到 %+v", i, r)
		}
	}
}

// v1.1 review：發起共用查詢的呼叫端（API 請求）先取消，其他呼叫端（排程）照樣拿到結果，⛔ 不觸發 5 分鐘降級。
func TestJudgeInitiatorCancelDoesNotDegrade(t *testing.T) {
	release := make(chan struct{})
	cal := &fakeCal{block: release, started: make(chan struct{}, 10)}
	j, _ := newJudge(cal, nil, monday)
	j.SetTimingForTest(5*time.Second, 5*time.Minute, nil)

	apiCtx, cancelAPI := context.WithCancel(context.Background())
	apiDone := make(chan Result, 1)
	go func() { apiDone <- j.On(apiCtx, monday) }()
	<-cal.started // API 請求發起了共用查詢

	schedDone := make(chan Result, 1)
	go func() { schedDone <- j.On(context.Background(), monday) }()
	time.Sleep(50 * time.Millisecond)

	cancelAPI()
	if r := <-apiDone; r.Source != SourceFallback {
		t.Fatalf("被取消的呼叫端自己回 fallback，得到 %+v", r)
	}
	close(release)
	if r := <-schedDone; r.Source != SourceCalendar || !r.IsTrading {
		t.Fatalf("其他呼叫端要照樣拿到日曆的結果，得到 %+v", r)
	}
	// 沒有寫入降級結果：下一次判斷直接是 calendar
	if r := j.On(context.Background(), monday); r.Source != SourceCalendar {
		t.Fatalf("呼叫端取消⛔ 不算日曆失敗，得到 %+v", r)
	}
	if n := cal.calls.Load(); n != 2 {
		// 第二次是 fakeCal 沒有快取（真的 exchangeReference 會走 TTL 快取），重點是中間沒有被降級擋掉
		t.Fatalf("預期 2 次請求（共用的那次＋下一次判斷），實際 %d 次", n)
	}
}

func TestJudgeNilCalendarIsFallback(t *testing.T) {
	j, _ := newJudge(nil, nil, monday)
	if r := j.On(context.Background(), monday); r.Source != SourceFallback || !r.IsTrading {
		t.Fatalf("沒有日曆來源時 fail-open，得到 %+v", r)
	}
}

type stuckOverrides struct {
	honorCtx bool
	release  chan struct{}
}

func (s stuckOverrides) Latest(ctx context.Context, _ string) (*store.MarketDayOverride, error) {
	if s.honorCtx {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-s.release:
			return nil, nil
		}
	}
	<-s.release // ⛔ 不理會 ctx 的來源
	return nil, nil
}

// v1.2 實作 review [P2]：cron 傳的是沒有期限的 context。手動設定讀不到（DB 卡住）時，判斷要在期限內繼續、照日曆判斷。
func TestJudgeOverrideReadHasDeadline(t *testing.T) {
	for _, honor := range []bool{true, false} {
		release := make(chan struct{})
		cal := &fakeCal{}
		core, logs := observer.New(zap.InfoLevel)
		j := New(cal, stuckOverrides{honorCtx: honor, release: release}, zap.New(core))
		j.SetTimingForTest(time.Second, 5*time.Minute, func() time.Time { return monday })
		j.SetOverrideTimeoutForTest(50 * time.Millisecond)
		start := time.Now()
		r := j.On(context.Background(), monday)
		el := time.Since(start)
		close(release)
		if r.Source != SourceCalendar || !r.IsTrading {
			t.Fatalf("honorCtx=%v：讀不到手動設定時照日曆判斷，得到 %+v", honor, r)
		}
		if el > 500*time.Millisecond {
			t.Fatalf("honorCtx=%v：手動設定的查詢要有自己的期限（測試 50ms），實際等了 %v", honor, el)
		}
		if logs.FilterMessage("trading day override unavailable, using calendar").Len() != 1 {
			t.Fatalf("honorCtx=%v：要記一次 Warn", honor)
		}
	}
}

type ignoringCal struct {
	calls   atomic.Int32
	release chan struct{}
}

func (c *ignoringCal) TradingCalendar(context.Context, int) (*market.TradingCalendar, error) {
	c.calls.Add(1)
	<-c.release // ⛔ 不理會 ctx 的來源
	return market.NewTradingCalendarForTest(2026, nil), nil
}

// 日曆來源⛔ 沒有響應取消時，呼叫端最多等兩倍期限就回 fallback，並把降級結果寫下來，後面的呼叫端⛔ 不用再等。
func TestJudgeStuckCalendarStillFallsBack(t *testing.T) {
	cal := &ignoringCal{release: make(chan struct{})}
	defer close(cal.release)
	j, logs := newJudge(cal, nil, monday) // 期限 50ms
	start := time.Now()
	if r := j.On(context.Background(), monday); r.Source != SourceFallback || !r.IsTrading {
		t.Fatalf("要回 true/fallback，得到 %+v", r)
	}
	if el := time.Since(start); el > 500*time.Millisecond {
		t.Fatalf("要在兩倍期限內放棄，實際等了 %v", el)
	}
	start = time.Now()
	if r := j.On(context.Background(), monday); r.Source != SourceFallback {
		t.Fatalf("降級期間直接用 fallback，得到 %+v", r)
	}
	if el := time.Since(start); el > 20*time.Millisecond {
		t.Fatalf("降級期間⛔ 不再等待，實際等了 %v", el)
	}
	if n := cal.calls.Load(); n != 1 {
		t.Fatalf("降級期間⛔ 不再請求，實際 %d 次", n)
	}
	if logs.FilterMessage("trading calendar unavailable").Len() != 1 {
		t.Fatal("進入降級時記一次 Warn")
	}
}

// 實作 review 第二輪 [P2]：成功讀到的手動設定⛔ 不能被誤判成逾時。舊寫法是讀取端送出結果後自己 cancel，
// 讀取端先跑完時，等候端的 select 會同時看到結果與取消、隨機走到逾時分支。
// 測試容器只有一顆 CPU，讀取端總是在等候端進入 select 之後才跑，所以用 overrideWaitHook 讓讀取端先跑完。
func TestJudgeOverrideResultNotLostToCancel(t *testing.T) {
	overrideWaitHook = func() { time.Sleep(100 * time.Microsecond) }
	defer func() { overrideWaitHook = nil }()
	ov := &fakeOverrides{}
	ov.set("2026-11-23", store.MarketDayClosed)
	ov.set("2026-12-25", store.MarketDayOpen)
	j, _ := newJudge(&fakeCal{closed: map[string]bool{"2026-12-25": true}}, ov, monday)
	for i := 0; i < 2000; i++ {
		if r := j.On(context.Background(), monday); r.Source != SourceManual || r.IsTrading {
			t.Fatalf("第 %d 次：手動休市被日曆覆蓋，得到 %+v", i, r)
		}
		if r := j.On(context.Background(), holiday); r.Source != SourceManual || !r.IsTrading {
			t.Fatalf("第 %d 次：手動開市被日曆覆蓋，得到 %+v", i, r)
		}
	}
}

// seqCal：第 1 次請求卡住（⛔ 不理會 ctx）直到 release、之後回成功；第 2 次以後依 errs 回。
type seqCal struct {
	calls   atomic.Int32
	release chan struct{}
	errs    []error
}

func (c *seqCal) TradingCalendar(context.Context, int) (*market.TradingCalendar, error) {
	n := int(c.calls.Add(1))
	if n == 1 {
		<-c.release
		return market.NewTradingCalendarForTest(2026, nil), nil
	}
	if n-2 < len(c.errs) && c.errs[n-2] != nil {
		return nil, c.errs[n-2]
	}
	return market.NewTradingCalendarForTest(2026, nil), nil
}

// 實作 review 第二輪 [P2]：日曆請求卡住之後，降級期滿要能重新探測；舊請求晚回來⛔ 不能覆蓋新一輪的判斷。
func TestJudgeStuckCalendarRetriesAfterDegradeAndIgnoresLateResult(t *testing.T) {
	now := monday
	cal := &seqCal{release: make(chan struct{}), errs: []error{errors.New("still down")}}
	core, logs := observer.New(zap.InfoLevel)
	j := New(cal, nil, zap.New(core))
	j.SetTimingForTest(50*time.Millisecond, 5*time.Minute, func() time.Time { return now })

	if r := j.On(context.Background(), monday); r.Source != SourceFallback {
		t.Fatalf("第一次卡住要 fallback，得到 %+v", r)
	}
	now = now.Add(5*time.Minute + time.Second)
	if r := j.On(context.Background(), monday); r.Source != SourceFallback {
		t.Fatalf("重新探測仍失敗要 fallback，得到 %+v", r)
	}
	if n := cal.calls.Load(); n != 2 {
		t.Fatalf("降級期滿要重新探測（⛔ 不再等卡住的那一筆），實際請求 %d 次", n)
	}
	// 舊請求現在才成功回來：⛔ 不能清掉新一輪的降級
	close(cal.release)
	time.Sleep(50 * time.Millisecond)
	if r := j.On(context.Background(), monday); r.Source != SourceFallback {
		t.Fatalf("舊請求晚回來的成功⛔ 不能覆蓋新一輪的降級，得到 %+v", r)
	}
	if n := cal.calls.Load(); n != 2 {
		t.Fatalf("降級期間⛔ 不再請求，實際 %d 次", n)
	}
	if logs.FilterMessage("trading calendar recovered").Len() != 0 {
		t.Fatal("舊請求晚回來⛔ 不記恢復")
	}
	// 再過一個降級期：第三次探測成功 → 回到日曆
	now = now.Add(5*time.Minute + time.Second)
	if r := j.On(context.Background(), monday); r.Source != SourceCalendar {
		t.Fatalf("探測成功要回到日曆，得到 %+v", r)
	}
}
