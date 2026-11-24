package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/analysis"
	"github.com/trading/backend/internal/market"
	"github.com/trading/backend/internal/store"
)

// I-141：盤前、收盤、籌碼同步讀不到自選清單時明確記 failed、0/0；清單讀得到但是空的仍是 success；
// daily_close 讀不到自選清單時，收盤尾端（sr_zone_verify）照跑、狀態各自獨立。
// 計畫書 docs/plans/i141-watchlist-fetch-status.md 第六節。

// okZoneRepo 回一筆分析、一個 zone，Verify 成功並記下 UpdateZoneStatus（尾端會更新 zone 狀態）。
type okZoneRepo struct {
	store.SRZoneRepo
	updated []uint64
}

func (r *okZoneRepo) ListRefsSince(context.Context, time.Time, int) ([]store.SRZoneAnalysisRef, error) {
	return []store.SRZoneAnalysisRef{{ID: 1, Symbol: "2330"}}, nil
}

func (r *okZoneRepo) Get(context.Context, uint64) (*store.SRZoneAnalysis, error) {
	return &store.SRZoneAnalysis{ID: 1, Symbol: "2330", Timeframe: "1d", AnalyzedAt: time.Now().Add(-48 * time.Hour)}, nil
}

func (r *okZoneRepo) GetZones(context.Context, uint64) ([]store.SRZone, error) {
	return []store.SRZone{{ID: 7, Role: "SUPPORT", PriceLow: 90, PriceHigh: 95}}, nil
}

func (r *okZoneRepo) UpdateZoneStatus(_ context.Context, zoneID uint64, _ string, _ *time.Time, _ *float64, _ string) error {
	r.updated = append(r.updated, zoneID)
	return nil
}

type emptyRangeCandles struct{ store.CandleRepo }

func (emptyRangeCandles) GetRange(context.Context, string, string, time.Time, time.Time) ([]store.Candle, error) {
	return nil, nil
}

func watchlistScheduler(jobRuns *schedulerJobRunRepoStub, wl *schedulerWatchlistStub) *Scheduler {
	s := newWatchlistFailScheduler(jobRuns)
	s.watchlist = wl
	s.fetcher = market.NewFetcher(nil, nil, zap.NewNop()) // 空清單時 BackfillHistory 照樣會被呼叫
	return s
}

var i141Jobs = []struct {
	name string
	run  func(*Scheduler)
}{
	{"pre_market", func(s *Scheduler) { s.runPreMarketWork(context.Background()) }},
	{"daily_close", func(s *Scheduler) { s.RunDailyClose() }},
	{"chip_daily_sync", func(s *Scheduler) { s.runChipDailySync(context.Background()) }},
}

func TestWatchlistFetchFailureRecordsFailed(t *testing.T) {
	for _, j := range i141Jobs {
		t.Run(j.name, func(t *testing.T) {
			jobRuns := &schedulerJobRunRepoStub{}
			j.run(watchlistScheduler(jobRuns, &schedulerWatchlistStub{err: leakingError()}))
			if len(jobRuns.started) == 0 || jobRuns.started[0] != j.name {
				t.Fatalf("第一筆要是 %s，得到 %v", j.name, jobRuns.started)
			}
			f := jobRuns.finished[0]
			if f.status != "failed" || f.symbolsTotal != 0 || f.symbolsFailed != 0 {
				t.Fatalf("讀不到自選清單要記 failed、0/0（⛔ 不是 success），得到 %+v", f)
			}
			assertNoLeak(t, j.name, f.errMsg)
			if f.errMsg != "watchlist_fetch:conn_refused" {
				t.Fatalf("error 要是 watchlist_fetch:conn_refused，得到 %q", f.errMsg)
			}
		})
	}
}

func TestWatchlistEmptyStaysSuccess(t *testing.T) {
	for _, j := range i141Jobs {
		t.Run(j.name, func(t *testing.T) {
			jobRuns := &schedulerJobRunRepoStub{}
			j.run(watchlistScheduler(jobRuns, &schedulerWatchlistStub{symbols: []string{}}))
			f := jobRuns.finished[0]
			if jobRuns.started[0] != j.name || f.status != "success" || f.symbolsTotal != 0 || f.symbolsFailed != 0 || f.errMsg != "" {
				t.Fatalf("清單讀得到但是空的是合法的零標的輪次，要 success 0/0，得到 %+v", f)
			}
			if j.name == "daily_close" && (len(jobRuns.started) != 2 || jobRuns.started[1] != "sr_zone_verify") {
				t.Fatalf("空清單的 daily_close 照樣執行尾端，得到 %v", jobRuns.started)
			}
		})
	}
}

func TestDailyCloseWatchlistFailureStillRunsTail(t *testing.T) {
	check := func(t *testing.T, jobRuns *schedulerJobRunRepoStub) {
		t.Helper()
		if len(jobRuns.started) != 2 || jobRuns.started[0] != "daily_close" || jobRuns.started[1] != "sr_zone_verify" {
			t.Fatalf("要恰好兩筆、依序 daily_close → sr_zone_verify，得到 %v", jobRuns.started)
		}
		if f := jobRuns.finished[0]; f.status != "failed" || f.symbolsTotal != 0 || f.errMsg != "watchlist_fetch:conn_refused" {
			t.Fatalf("daily_close 要 failed、0/0、watchlist_fetch，得到 %+v", f)
		}
	}
	t.Run("尾端驗證成功：sr_zone_verify 自己 success，zone 狀態有更新", func(t *testing.T) {
		jobRuns := &schedulerJobRunRepoStub{}
		s := watchlistScheduler(jobRuns, &schedulerWatchlistStub{err: leakingError()})
		repo := &okZoneRepo{}
		s.srZoneRepo = repo
		s.srZoneVerifier = analysis.NewSRZoneVerifier(repo, emptyRangeCandles{})
		s.RunDailyClose()
		check(t, jobRuns)
		if f := jobRuns.finished[1]; f.status != "success" || f.symbolsTotal != 1 || f.symbolsFailed != 0 {
			t.Fatalf("sr_zone_verify 要 success、1/0（跟 daily_close 的 failed 無關），得到 %+v", f)
		}
		if len(repo.updated) != 1 || repo.updated[0] != 7 {
			t.Fatalf("尾端要更新 zone 狀態，得到 %v", repo.updated)
		}
	})
	t.Run("尾端清單失敗：sr_zone_verify 自己 failed，daily_close 的紀錄不受影響", func(t *testing.T) {
		jobRuns := &schedulerJobRunRepoStub{}
		s := watchlistScheduler(jobRuns, &schedulerWatchlistStub{err: leakingError()})
		s.srZoneRepo = &failingZoneRepo{err: errors.New("zone list failed")}
		s.RunDailyClose()
		check(t, jobRuns)
		if f := jobRuns.finished[1]; f.status != "failed" || f.errMsg == "" {
			t.Fatalf("sr_zone_verify 要記自己的 failed，得到 %+v", f)
		}
	})
}
