package scheduler

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/market"
	"github.com/trading/backend/internal/store"
	"github.com/trading/backend/internal/tradingday"
)

// I-139：休市日的閘門只在 cron 入口。計畫書 docs/plans/i139-trading-calendar-scheduler.md 第七節的排程案例。

type closedCal struct {
	err    error
	closed map[string]bool
}

func (c closedCal) TradingCalendar(_ context.Context, year int) (*market.TradingCalendar, error) {
	if c.err != nil {
		return nil, c.err
	}
	return market.NewTradingCalendarForTest(year, c.closed), nil
}

type manualOverrides map[string]string

func (m manualOverrides) Latest(_ context.Context, d string) (*store.MarketDayOverride, error) {
	a, ok := m[d]
	if !ok {
		return nil, nil
	}
	return &store.MarketDayOverride{TradeDate: d, Action: a, Reason: "test", UserID: 1}, nil
}

// 2026-11-02（週一，tpe() 的日期）分別判成：日曆休市、手動休市、日曆取不到、交易日。
func calendarClosedJudge() *tradingday.Judge {
	return tradingday.New(closedCal{closed: map[string]bool{"2026-11-02": true}}, nil, zap.NewNop())
}
func manualClosedJudge() *tradingday.Judge {
	return tradingday.New(closedCal{}, manualOverrides{"2026-11-02": store.MarketDayClosed}, zap.NewNop())
}
func fallbackJudge() *tradingday.Judge {
	return tradingday.New(closedCal{err: errors.New("twse down")}, nil, zap.NewNop())
}
func tradingJudge() *tradingday.Judge { return tradingday.New(closedCal{}, nil, zap.NewNop()) }

func TestPreMarketCronOnClosedDay(t *testing.T) {
	jobRuns := &schedulerJobRunRepoStub{}
	// fetcher 是 nil：閘門沒擋住就會在回補時 panic
	s := newOKWatchlistScheduler(jobRuns)
	s.SetTradingDay(manualClosedJudge())
	s.intradayClockFunc = func() time.Time { return tpe(8, 50) }

	s.runPreMarketCron()

	if jobRuns.deleteCutoff == nil {
		t.Fatal("休市日的盤前清理要照跑（job_runs 保留期）")
	}
	if len(jobRuns.started) != 1 || jobRuns.started[0] != "pre_market" {
		t.Fatalf("只記一筆 pre_market，得到 %v", jobRuns.started)
	}
	f := jobRuns.finished[0]
	if f.status != "skipped" || f.errMsg != "market_closed (manual)" || f.symbolsTotal != 0 {
		t.Fatalf("要記 skipped＋market_closed (manual)、symbols_total=0，得到 %+v", f)
	}
}

func TestPreMarketCronOnTradingDayRunsWork(t *testing.T) {
	for name, j := range map[string]*tradingday.Judge{"calendar": tradingJudge(), "fallback": fallbackJudge(), "nil": nil} {
		t.Run(name, func(t *testing.T) {
			jobRuns := &schedulerJobRunRepoStub{}
			s := newWatchlistFailScheduler(jobRuns) // 走到工作那一步就會因為讀不到清單而記 watchlist_fetch
			s.SetTradingDay(j)
			s.intradayClockFunc = func() time.Time { return tpe(8, 50) }
			s.runPreMarketCron()
			if jobRuns.deleteCutoff == nil || len(jobRuns.finished) != 1 || !strings.HasPrefix(jobRuns.finished[0].errMsg, "watchlist_fetch") {
				t.Fatalf("交易日（含日曆取不到）要照跑：清理＋盤前，得到 %+v", jobRuns.finished)
			}
		})
	}
}

func TestDailyCloseCronOnClosedDayRunsZoneVerify(t *testing.T) {
	jobRuns := &schedulerJobRunRepoStub{}
	s := newOKWatchlistScheduler(jobRuns)
	s.srZoneRepo = &failingZoneRepo{err: errors.New("zone list failed")}
	s.SetTradingDay(calendarClosedJudge())
	s.intradayClockFunc = func() time.Time { return tpe(15, 0) }

	s.runDailyCloseCron()

	if len(jobRuns.started) != 2 || jobRuns.started[0] != "daily_close" || jobRuns.started[1] != "sr_zone_verify" {
		t.Fatalf("休市日：daily_close 記 skipped、sr_zone_verify 照跑，得到 %v", jobRuns.started)
	}
	if f := jobRuns.finished[0]; f.status != "skipped" || f.errMsg != "market_closed (calendar)" {
		t.Fatalf("daily_close 要記 skipped＋market_closed (calendar)，得到 %+v", f)
	}
	if f := jobRuns.finished[1]; f.status != "failed" {
		t.Fatalf("sr_zone_verify 要照跑（這裡因為假 repo 失敗而 failed），得到 %+v", f)
	}
}

func TestDailyCloseManualEndpointIgnoresGate(t *testing.T) {
	jobRuns := &schedulerJobRunRepoStub{}
	s := newWatchlistFailScheduler(jobRuns)
	s.SetTradingDay(manualClosedJudge())
	s.intradayClockFunc = func() time.Time { return tpe(15, 0) }
	s.RunDailyClose() // 手動端點（POST /scheduler/daily-close/run）呼叫的是它
	if len(jobRuns.finished) == 0 || jobRuns.finished[0].status == "skipped" {
		t.Fatalf("手動端點⛔ 不經過閘門，得到 %+v", jobRuns.finished)
	}
}

func TestIntradayClosedDayEveryRoundSkipped(t *testing.T) {
	for _, tc := range []struct {
		name   string
		judge  *tradingday.Judge
		reason string
	}{
		{"manual", manualClosedJudge(), "market_closed (manual)"},
		{"calendar", calendarClosedJudge(), "market_closed (calendar)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := &scriptSource{batch: 20, respond: func(context.Context, int, []string) (map[string][]market.Candle, error) {
				return map[string][]market.Candle{}, nil
			}}
			h := newIntradayHarness(t, src, []string{"2330"})
			h.s.SetTradingDay(tc.judge)
			at := tpe(9, 0)
			h.s.intradayClockFunc = func() time.Time { return at }

			count := func() int {
				runs, err := h.jobRuns.GetRecent(context.Background(), 200)
				if err != nil {
					t.Fatal(err)
				}
				n := 0
				for _, r := range runs {
					if r.JobName != "intraday" {
						continue
					}
					if r.Status != "skipped" || r.Error.String != tc.reason || r.SymbolsTotal != 0 {
						t.Fatalf("休市日每一輪都要是 skipped＋%s，得到 %+v", tc.reason, r)
					}
					n++
				}
				return n
			}
			for at = tpe(9, 0); !at.After(tpe(13, 55)); at = at.Add(5 * time.Minute) {
				h.s.runIntradayJob()
				switch {
				case at.Equal(tpe(9, 0)) && count() != 1:
					t.Fatalf("09:00 第一輪要記一筆，得到 %d", count())
				case at.Equal(tpe(13, 30)) && count() != 55:
					t.Fatalf("13:30 是最後一輪，要累計 55 筆，得到 %d", count())
				}
			}
			if n := count(); n != 55 {
				t.Fatalf("13:35 之後⛔ 不再記錄，一天共 55 筆，得到 %d", n)
			}
			if src.calls != 0 {
				t.Fatalf("休市日⛔ 不抓取，得到 %d 次請求", src.calls)
			}
			if r, err := h.evidence.LatestRound(context.Background(), store.IntradaySourceScheduled); err != nil || r != nil {
				t.Fatalf("休市日⛔ 不寫 intraday_rounds，得到 %+v, %v", r, err)
			}
			if len(h.s.intradayLock) != 0 {
				t.Fatal("休市日⛔ 不拿鎖")
			}
			if h.candles.signalReads.Load() != 0 {
				t.Fatal("休市日⛔ 不評估")
			}
		})
	}
}

func TestIntradayTradingDayAndFallbackRun(t *testing.T) {
	for name, j := range map[string]*tradingday.Judge{"calendar": tradingJudge(), "fallback": fallbackJudge()} {
		t.Run(name, func(t *testing.T) {
			src := &scriptSource{batch: 20, respond: func(context.Context, int, []string) (map[string][]market.Candle, error) {
				return map[string][]market.Candle{}, nil
			}}
			h := newIntradayHarness(t, src, []string{"2330"})
			h.s.SetTradingDay(j)
			h.s.runIntradayJob()
			if src.calls == 0 {
				t.Fatal("交易日（含日曆取不到）要照常抓取")
			}
			if job := h.lastJob(t, "intraday"); job.Status == "skipped" {
				t.Fatalf("交易日⛔ 不記 skipped，得到 %+v", job)
			}
		})
	}
}

func TestIntradayRefillIgnoresGate(t *testing.T) {
	src := &scriptSource{batch: 20, respond: func(context.Context, int, []string) (map[string][]market.Candle, error) {
		return map[string][]market.Candle{}, nil
	}}
	h := newIntradayHarness(t, src, []string{"2330"})
	h.s.SetTradingDay(manualClosedJudge())
	if _, err := h.s.RunIntradayRefill(context.Background(), nil); err != nil {
		t.Fatalf("手動補抓⛔ 不經過閘門：%v", err)
	}
	if src.calls == 0 {
		t.Fatal("休市日的手動補抓照樣抓取")
	}
}

type blockedOverrides struct{ release chan struct{} }

func (b blockedOverrides) Latest(context.Context, string) (*store.MarketDayOverride, error) {
	<-b.release // DB 卡住、⛔ 不理會 ctx
	return nil, nil
}

type blockedCal struct{ release chan struct{} }

func (b blockedCal) TradingCalendar(context.Context, int) (*market.TradingCalendar, error) {
	<-b.release // 交易所卡住、⛔ 不理會 ctx
	return nil, errors.New("late")
}

// v1.2 實作 review [P2]：手動設定讀不到、日曆卡住，兩條路徑都要按時繼續排程（cron 的 context 沒有期限）。
func TestIntradayContinuesWhenJudgeSourcesStall(t *testing.T) {
	for name, build := range map[string]func(chan struct{}) *tradingday.Judge{
		"手動設定卡住": func(r chan struct{}) *tradingday.Judge {
			j := tradingday.New(closedCal{}, blockedOverrides{release: r}, zap.NewNop())
			j.SetOverrideTimeoutForTest(50 * time.Millisecond)
			return j
		},
		"日曆卡住": func(r chan struct{}) *tradingday.Judge {
			j := tradingday.New(blockedCal{release: r}, nil, zap.NewNop())
			j.SetTimingForTest(50*time.Millisecond, 5*time.Minute, nil)
			return j
		},
	} {
		t.Run(name, func(t *testing.T) {
			release := make(chan struct{})
			defer close(release)
			src := &scriptSource{batch: 20, respond: func(context.Context, int, []string) (map[string][]market.Candle, error) {
				return map[string][]market.Candle{}, nil
			}}
			h := newIntradayHarness(t, src, []string{"2330"})
			h.s.SetTradingDay(build(release))
			done := make(chan struct{})
			go func() { h.s.runIntradayJob(); close(done) }()
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("判斷的來源卡住時，盤中這一輪要按時繼續")
			}
			if src.calls == 0 {
				t.Fatal("要照交易日（日曆或 fallback）執行這一輪")
			}
		})
	}
}
