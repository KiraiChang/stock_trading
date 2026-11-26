package analysis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/trading/backend/internal/store"
)

// T-089 D1：SR 快照只在「同一根日 K」時重用。
func d1Provider(t *testing.T, snapshotAt time.Time, candles LatestCandleReader, maxAge time.Duration) (*SRAnalysisProvider, *srProviderScorerStub, *srProviderRepoStub, time.Time) {
	t.Helper()
	now := time.Date(2026, 11, 25, 8, 0, 0, 0, time.UTC) // 台北 11-25 16:00
	repo := &srProviderRepoStub{
		analyses: []store.SRZoneAnalysis{{ID: 9, Symbol: "2330", Timeframe: "1d", AnalyzedAt: snapshotAt, CurrentPrice: 100}},
		zones:    map[uint64][]store.SRZone{9: {{ID: 1, AnalysisID: 9}}},
		nextID:   10,
	}
	scorer := &srProviderScorerStub{result: testZoneScoreResult("2330", now)}
	p := NewSRAnalysisProvider(scorer, repo, candles, maxAge)
	p.now = func() time.Time { return now }
	return p, scorer, repo, now
}

// 台北日期的 00:00（日 K 的時間）
func tpeDay(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.FixedZone("Asia/Taipei", 8*3600))
}

func TestSRProviderReusesWhenSnapshotIsLatestCandle(t *testing.T) {
	day := tpeDay(2026, 11, 25)
	p, scorer, repo, _ := d1Provider(t, day, &latestCandleStub{ts: day}, 24*time.Hour)
	r, err := p.Analyze(context.Background(), "2330", SRAnalysisOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Analysis.ID != 9 || scorer.calls != 0 || repo.createCalls != 0 {
		t.Fatalf("快照就是最新那根日 K、在年齡內 → 重用；得到 id=%d scorer=%d create=%d", r.Analysis.ID, scorer.calls, repo.createCalls)
	}
}

// 設定 72 小時時舊行為會重用前一天的快照（分析紀錄時間新、行情日期舊）；D1 之後新日 K 入庫就一定重算。
func TestSRProviderRefreshesWhenNewerCandleArrived(t *testing.T) {
	snap := tpeDay(2026, 11, 24)
	latest := tpeDay(2026, 11, 25)
	p, scorer, repo, _ := d1Provider(t, snap, &latestCandleStub{ts: latest}, 72*time.Hour)
	r, err := p.Analyze(context.Background(), "2330", SRAnalysisOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Analysis.ID != 10 || scorer.calls != 1 || repo.createCalls != 1 {
		t.Fatalf("最新日 K 比快照新 → 重算（就算仍在 72 小時內）；得到 id=%d scorer=%d create=%d", r.Analysis.ID, scorer.calls, repo.createCalls)
	}
}

func TestSRProviderRefreshesWhenLatestCandleUnknown(t *testing.T) {
	day := tpeDay(2026, 11, 25)
	cases := map[string]LatestCandleReader{
		"查無 K 棒（sql.ErrNoRows）": &latestCandleStub{noData: true},
		"查詢失敗":                  &latestCandleStub{err: errors.New("db down")},
		"沒有注入 candles":          nil,
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			p, scorer, repo, _ := d1Provider(t, day, c, 24*time.Hour)
			r, err := p.Analyze(context.Background(), "2330", SRAnalysisOptions{})
			if err != nil {
				t.Fatalf("查不到最新 K 棒⛔ 不能讓分析失敗，得到 %v", err)
			}
			if r.Analysis.ID != 10 || scorer.calls != 1 || repo.createCalls != 1 {
				t.Fatalf("確認不了是不是同一根 → ⛔ 不重用、照常呼叫 Python；得到 id=%d scorer=%d create=%d", r.Analysis.ID, scorer.calls, repo.createCalls)
			}
		})
	}
}

func TestSRProviderForceRefreshSkipsCandleLookup(t *testing.T) {
	day := tpeDay(2026, 11, 25)
	c := &latestCandleStub{ts: day}
	p, scorer, _, _ := d1Provider(t, day, c, 24*time.Hour)
	r, err := p.Analyze(context.Background(), "2330", SRAnalysisOptions{ForceRefresh: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.Analysis.ID != 10 || scorer.calls != 1 || c.calls != 0 {
		t.Fatalf("強制刷新照舊：重算、⛔ 不查重用條件；得到 id=%d scorer=%d candleCalls=%d", r.Analysis.ID, scorer.calls, c.calls)
	}
}
