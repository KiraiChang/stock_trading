package market

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
)

// I-139 實作 review：節流的等鎖與等間隔都要能響應取消——交易日判斷有 5 秒期限，⛔ 不能卡在另一個呼叫端持有的鎖上。

func newThrottleRef(t *testing.T, interval time.Duration, url string) (*exchangeReference, *SourceBreaker) {
	t.Helper()
	breaker := NewSourceBreaker(1, time.Hour)
	ref := NewExchangeReference(breaker, ExchangeReferenceOptions{RequestInterval: interval, CalendarURL: url}, zap.NewNop())
	return ref.(*exchangeReference), breaker
}

func TestThrottleWaitForSlotIsCancellable(t *testing.T) {
	c, _ := newThrottleRef(t, 10*time.Second, "")
	c.throttleSlot <- struct{}{} // 另一個呼叫端正拿著鎖（例如正在等它的間隔）
	defer func() { <-c.throttleSlot }()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := c.throttle(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("等鎖期間到期要回 ctx 的錯誤，得到 %v", err)
	}
	if el := time.Since(start); el > time.Second {
		t.Fatalf("等鎖要能響應取消，實際等了 %v", el)
	}
}

func TestThrottleWaitForIntervalIsCancellable(t *testing.T) {
	c, _ := newThrottleRef(t, 10*time.Second, "")
	last := time.Now()
	c.lastRequest = last
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := c.throttle(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("等間隔期間到期要回 ctx 的錯誤，得到 %v", err)
	}
	if el := time.Since(start); el > time.Second {
		t.Fatalf("等間隔要能響應取消，實際等了 %v", el)
	}
	if !c.lastRequest.Equal(last) {
		t.Fatal("沒有發出請求時⛔ 不更新 lastRequest")
	}
	if len(c.throttleSlot) != 0 {
		t.Fatal("放棄之後要釋放鎖")
	}
}

func TestThrottleCancelDoesNotTripBreaker(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("節流等待被取消時⛔ 不該發出請求")
	}))
	defer srv.Close()
	c, breaker := newThrottleRef(t, 10*time.Second, srv.URL)
	c.lastRequest = time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := c.TradingCalendar(ctx, 2026)
	if !errors.Is(err, ErrVerificationUnavailable) {
		t.Fatalf("要回 ErrVerificationUnavailable，得到 %v", err)
	}
	if breaker.IsOpen(SourceTWSECalendar) || breaker.Failures(SourceTWSECalendar) != 0 {
		t.Fatal("呼叫端自己的期限⛔ 不是來源故障，⛔ 不累加 breaker")
	}
}

func TestThrottleSerialisesCallers(t *testing.T) {
	c, _ := newThrottleRef(t, 30*time.Millisecond, "")
	start := time.Now()
	done := make(chan error, 3)
	for i := 0; i < 3; i++ {
		go func() { done <- c.throttle(context.Background()) }()
	}
	for i := 0; i < 3; i++ {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	// 第一個不用等，後兩個各等一個間隔
	if el := time.Since(start); el < 55*time.Millisecond {
		t.Fatalf("多個呼叫端要依序間隔，三次只花了 %v", el)
	}
}

// 實作 review 第二輪 [P2]：鎖空著時，已取消的請求⛔ 不能推進節流時間、⛔ 不能累加斷路器。select 拿到鎖是隨機的，所以重複多次。
func TestThrottleAlreadyCancelledSkipsUpdates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("已取消的請求⛔ 不該送出")
	}))
	defer srv.Close()
	// 間隔 1ns：拿到鎖之後一定「不用等」，舊寫法每次拿到鎖都會推進 lastRequest。
	c, breaker := newThrottleRef(t, time.Nanosecond, srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for i := 0; i < 500; i++ {
		if err := c.throttle(ctx); !errors.Is(err, context.Canceled) {
			t.Fatalf("第 %d 次：已取消要回 context.Canceled，得到 %v", i, err)
		}
		if _, err := c.TradingCalendar(ctx, 2026); !errors.Is(err, ErrVerificationUnavailable) {
			t.Fatalf("第 %d 次：要回 ErrVerificationUnavailable，得到 %v", i, err)
		}
		if !c.lastRequest.IsZero() {
			t.Fatalf("第 %d 次：已取消⛔ 不推進 lastRequest", i)
		}
	}
	if n := breaker.Failures(SourceTWSECalendar); n != 0 || breaker.IsOpen(SourceTWSECalendar) {
		t.Fatalf("呼叫端自己取消⛔ 不累加 breaker，得到 %d 次", n)
	}
}

// 請求送出途中被呼叫端取消，也⛔ 不算來源故障。
func TestRequestCancelledInFlightDoesNotTripBreaker(t *testing.T) {
	hold := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-hold:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(hold)
	c, breaker := newThrottleRef(t, 0, srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := c.TradingCalendar(ctx, 2026); !errors.Is(err, ErrVerificationUnavailable) {
		t.Fatalf("要回 ErrVerificationUnavailable，得到 %v", err)
	}
	if breaker.Failures(SourceTWSECalendar) != 0 {
		t.Fatal("呼叫端的期限⛔ 不算來源故障")
	}
}

// ── I-139 實作 review 第三輪 ────────────────────────────────────────────────

func newCachedRef(t *testing.T, h http.Handler) (*exchangeReference, *SourceBreaker) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	breaker := NewSourceBreaker(1, time.Hour)
	ref := NewExchangeReference(breaker, ExchangeReferenceOptions{CalendarURL: srv.URL, CalendarTTL: time.Hour}, zap.NewNop())
	return ref.(*exchangeReference), breaker
}

// [P2] 讀 body 時才被呼叫端取消或到期，⛔ 不算來源故障。
func TestCalendarBodyReadCancelDoesNotTripBreaker(t *testing.T) {
	hold := make(chan struct{})
	c, breaker := newCachedRef(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"stat":"OK","data":[`))
		w.(http.Flusher).Flush() // 標頭與部分內容已送出，body 還沒讀完
		select {
		case <-hold:
		case <-r.Context().Done():
		}
	}))
	defer close(hold)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := c.TradingCalendar(ctx, 2026); !errors.Is(err, ErrVerificationUnavailable) {
		t.Fatalf("要回 ErrVerificationUnavailable，得到 %v", err)
	}
	if n := breaker.Failures(SourceTWSECalendar); n != 0 {
		t.Fatalf("讀 body 時被呼叫端取消⛔ 不累加 breaker，得到 %d 次", n)
	}
}

const (
	// 日曆要至少一列（空的回應會被當成來源異常）：「開市」版只有元旦，「休市」版多了 11-23。
	calOpen   = `{"stat":"OK","data":[["2026-01-01","開國紀念日","放假一日"]]}`
	calClosed = `{"stat":"OK","data":[["2026-01-01","開國紀念日","放假一日"],["2026-11-23","臨時休市","放假一日"]]}`
)

// [P2] 較早開始、較晚回來的舊請求⛔ 不能蓋掉較新的年度快取（新請求先存「開市」，舊請求晚回來「休市」）。
func TestCalendarCacheIgnoresOlderRequestReturningLate(t *testing.T) {
	var calls atomic.Int32
	firstStarted, releaseFirst := make(chan struct{}), make(chan struct{})
	c, _ := newCachedRef(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if calls.Add(1) == 1 {
			close(firstStarted)
			<-releaseFirst
			_, _ = w.Write([]byte(calClosed))
			return
		}
		_, _ = w.Write([]byte(calOpen))
	}))
	var once sync.Once
	release := func() { once.Do(func() { close(releaseFirst) }) }
	t.Cleanup(release) // 失敗時也要放行，⛔ 不讓 server 關不掉（cleanup 依註冊的反序，先於 srv.Close）
	day := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)

	oldDone := make(chan *TradingCalendar, 1)
	go func() {
		cal, err := c.TradingCalendar(context.Background(), 2026)
		if err != nil {
			t.Error(err)
		}
		oldDone <- cal
	}()
	<-firstStarted
	fresh, err := c.TradingCalendar(context.Background(), 2026) // 新請求先回來、先寫快取
	if err != nil || !fresh.IsTradingDay(day) {
		t.Fatalf("新請求：%v %v", fresh, err)
	}
	release()
	if old := <-oldDone; old == nil || old.IsTradingDay(day) {
		t.Fatal("舊請求自己的結果照樣交給呼叫端（休市）")
	}
	cached, err := c.TradingCalendar(context.Background(), 2026)
	if err != nil || !cached.IsTradingDay(day) {
		t.Fatalf("快取要保留較新的「開市」，⛔ 不能被晚回來的舊請求蓋掉：%v %v", cached, err)
	}
	if n := calls.Load(); n != 2 {
		t.Fatalf("第三次要讀快取，實際請求 %d 次", n)
	}
}

// lateCtx：Done 永遠⛔ 不關閉（請求照常讀完），但 expire 之後 Err 回到期——模擬「期限在回應讀完、寫快取之前到」。
type lateCtx struct {
	context.Context
	expired atomic.Bool
}

func (c *lateCtx) Err() error {
	if c.expired.Load() {
		return context.DeadlineExceeded
	}
	return nil
}

// [P2] 已過期（或已取消）的請求晚回來時，結果照樣交給呼叫端，但⛔ 不寫快取。
func TestCalendarCacheSkipsExpiredRequest(t *testing.T) {
	var calls atomic.Int32
	ctx := &lateCtx{Context: context.Background()}
	c, _ := newCachedRef(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if calls.Add(1) == 1 {
			ctx.expired.Store(true) // 這個請求的期限在回應送出之前就到了
			_, _ = w.Write([]byte(calClosed))
			return
		}
		_, _ = w.Write([]byte(calOpen))
	}))
	day := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)
	if cal, err := c.TradingCalendar(ctx, 2026); err != nil || cal.IsTradingDay(day) {
		t.Fatalf("過期的請求照樣把結果交給呼叫端：%v %v", cal, err)
	}
	cal, err := c.TradingCalendar(context.Background(), 2026)
	if err != nil || !cal.IsTradingDay(day) || calls.Load() != 2 {
		t.Fatalf("過期請求的結果⛔ 不寫快取，下一次要重新請求：%v %v，請求 %d 次", cal, err, calls.Load())
	}
}
