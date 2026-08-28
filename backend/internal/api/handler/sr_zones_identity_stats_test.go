package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/trading/backend/internal/store"
	"go.uber.org/zap"
)

// T-050：身分關聯決策的統計要能落地，而且**降級的那幾次也要留一列**。

type identityStatsRepoStub struct {
	rows      []store.SRIdentityStats
	insertErr error
}

func (s *identityStatsRepoStub) Insert(ctx context.Context, row *store.SRIdentityStats) error {
	if s.insertErr != nil {
		return s.insertErr
	}
	s.rows = append(s.rows, *row)
	return nil
}

func (s *identityStatsRepoStub) List(ctx context.Context, q store.SRIdentityStatsQuery) ([]store.SRIdentityStats, error) {
	return s.rows, nil
}

func newStatsHandler(repo store.SRIdentityStatsRepo) *SRZoneHandler {
	h := &SRZoneHandler{log: zap.NewNop()}
	h.SetIdentityStats(repo)
	return h
}

func TestPersistIdentityStatsMapsEventCounts(t *testing.T) {
	repo := &identityStatsRepoStub{}
	h := newStatsHandler(repo)

	h.persistIdentityStats(context.Background(), 42, "2330", "1d",
		&zoneIdentityMatch{live: make([]store.LiveZone, 7)},
		&zoneIdentityOutcome{},
		&eventIdentityStats{
			MatchedByChain:   5,
			MatchedByCurrent: 3,
			MatchedByAlias:   1,
			UnmatchedKeys:    []string{"k1"},
			ChainConflicts:   []string{"c1", "c2"},
			CarriedParseFail: 4,
			Invariant:        []string{"bad"},
		})

	if len(repo.rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(repo.rows))
	}
	got := repo.rows[0]
	if got.AnalysisID != 42 || got.Symbol != "2330" || got.Timeframe != "1d" {
		t.Fatalf("識別欄位不對：%+v", got)
	}
	if got.MatchedByChain != 5 || got.MatchedByCurrent != 3 || got.MatchedByAlias != 1 {
		t.Errorf("三段命中數不對：%+v", got)
	}
	// **切片欄位存的是長度不是內容**：表只要計數，明細留在 log。
	if got.UnmatchedKeys != 1 || got.ChainConflicts != 2 || got.InvariantViolations != 1 {
		t.Errorf("切片欄位應存長度：%+v", got)
	}
	if got.CarriedParseFail != 4 {
		t.Errorf("carried_parse_fail = %d, want 4", got.CarriedParseFail)
	}
	if got.ZoneLiveCandidates != 7 {
		t.Errorf("zone_live_candidates = %d, want 7（比率的分母參考）", got.ZoneLiveCandidates)
	}
	if got.ZoneIdentityDegraded || got.EventIdentityDegraded {
		t.Error("正常跑完不該標成 degraded")
	}
}

// **降級也要留一列。** 若這時候乾脆不寫，趨勢圖上會看到「這天很乾淨」，
// 而真相是「這天什麼都沒算」——那正是本筆要消滅的那種靜默。
func TestPersistIdentityStatsRecordsDegradedRun(t *testing.T) {
	repo := &identityStatsRepoStub{}
	h := newStatsHandler(repo)

	h.persistIdentityStats(context.Background(), 43, "2330", "1d", nil, nil, nil)

	if len(repo.rows) != 1 {
		t.Fatalf("降級時仍要寫一列，got %d", len(repo.rows))
	}
	got := repo.rows[0]
	if !got.ZoneIdentityDegraded || !got.EventIdentityDegraded {
		t.Errorf("兩個 degraded 旗標都該是 true：%+v", got)
	}
	if got.MatchedByChain != 0 || got.AliasAmbiguous != 0 {
		t.Errorf("降級時計數應全為 0（那正是要靠旗標區分的原因）：%+v", got)
	}
}

// 統計寫入失敗**不能影響分析**——這條與身分層其餘寫入的 fail-open 語意一致。
func TestPersistIdentityStatsFailOpen(t *testing.T) {
	h := newStatsHandler(&identityStatsRepoStub{insertErr: errors.New("db down")})

	// 沒有 panic、沒有回傳值要處理：呼叫端不會因為統計失敗而中斷。
	h.persistIdentityStats(context.Background(), 44, "2330", "1d", nil, nil, nil)
}

// 未注入 repo 時整段不執行，行為與導入前完全相同。
func TestPersistIdentityStatsSkippedWhenRepoMissing(t *testing.T) {
	h := &SRZoneHandler{log: zap.NewNop()}
	h.persistIdentityStats(context.Background(), 45, "2330", "1d", nil, nil, nil)
}

// summary 的比率在查詢時才算——分母隨區間而變，存進表等於把決定寫死。
func TestSummarizeIdentityStatsComputesAliasHitRate(t *testing.T) {
	got := summarizeIdentityStats([]store.SRIdentityStats{
		{MatchedByChain: 6, MatchedByCurrent: 2, MatchedByAlias: 2, AliasAmbiguous: 1},
		{MatchedByChain: 8, MatchedByCurrent: 2, MatchedByAlias: 0, InvariantViolations: 0},
		{ZoneIdentityDegraded: true},
	})

	if got.Analyses != 3 || got.Degraded != 1 {
		t.Fatalf("analyses/degraded = %d/%d, want 3/1", got.Analyses, got.Degraded)
	}
	if got.MatchedTotal != 20 {
		t.Fatalf("matched_total = %d, want 20", got.MatchedTotal)
	}
	if got.AliasHitRate != 0.1 {
		t.Errorf("alias_hit_rate = %v, want 0.1", got.AliasHitRate)
	}
	if got.AliasAmbiguous != 1 {
		t.Errorf("alias_ambiguous 應加總：%d", got.AliasAmbiguous)
	}
}

// 沒有任何命中時不能除以零。
func TestSummarizeIdentityStatsHandlesEmpty(t *testing.T) {
	got := summarizeIdentityStats(nil)
	if got.Analyses != 0 || got.MatchedTotal != 0 || got.AliasHitRate != 0 {
		t.Errorf("空輸入應全為 0：%+v", got)
	}
}
