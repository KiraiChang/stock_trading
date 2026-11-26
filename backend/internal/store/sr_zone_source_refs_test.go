package store

import (
	"context"
	"testing"
	"time"
)

// T-089 D2：交易計畫的來源回查。依 id 一次取回 id、timeframe、analyzed_at；不存在的 id 不在結果裡；空的 ids ⛔ 不查詢。
func TestSRZoneRepoGetSourceRefs(t *testing.T) {
	repo := newTestSRZoneRepo(t)
	ctx := context.Background()

	day := time.Date(2026, 11, 24, 16, 0, 0, 0, time.UTC) // 台北 11-25 00:00
	a1 := testAnalysis()
	a1.Symbol, a1.Timeframe, a1.AnalyzedAt = "2330", "1d", day
	id1, err := repo.Create(ctx, a1, testZones(), SRZoneNormalizedProjections{})
	if err != nil {
		t.Fatalf("Create a1: %v", err)
	}
	a2 := testAnalysis()
	a2.Symbol, a2.Timeframe, a2.AnalyzedAt = "2330", "1h", day.Add(5*time.Hour)
	id2, err := repo.Create(ctx, a2, testZones(), SRZoneNormalizedProjections{})
	if err != nil {
		t.Fatalf("Create a2: %v", err)
	}

	refs, err := repo.GetSourceRefs(ctx, []uint64{id2, 999999, id1})
	if err != nil {
		t.Fatalf("GetSourceRefs: %v", err)
	}
	if len(refs) != 2 {
		t.Fatalf("要 2 筆（不存在的 id 不在結果裡），得到 %+v", refs)
	}
	got := map[uint64]SRZoneAnalysisSourceRef{}
	for _, r := range refs {
		got[r.ID] = r
	}
	if r := got[id1]; r.Timeframe != "1d" || !r.AnalyzedAt.Equal(day) {
		t.Fatalf("a1 要是 1d、%v，得到 %+v", day, r)
	}
	if r := got[id2]; r.Timeframe != "1h" || !r.AnalyzedAt.Equal(day.Add(5*time.Hour)) {
		t.Fatalf("a2 要是 1h、%v，得到 %+v", day.Add(5*time.Hour), r)
	}

	empty, err := repo.GetSourceRefs(ctx, nil)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("空的 ids 要回空的 slice、沒有錯誤，得到 %+v、%v", empty, err)
	}
}
