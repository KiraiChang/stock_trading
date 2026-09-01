package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/database"
)

// summary 走**未套 limit** 的 aggregate，limit 只截斷明細——否則 alias_hit_rate 的
// 分母會由 limit 決定而不是由查詢區間決定（2026-08-31 修正的 bug）。
//
// **只跑 sqlite**，與其他 repo 測試相同的既有限制（issue.md I-054 第 1 項）：
// Summarize 的 COUNT / SUM / CASE WHEN 在 postgres 與 mysql 上從未被實際執行過。
func newSRIdentityStatsRepoForTest(t *testing.T) (SRIdentityStatsRepo, *sqlx.DB, context.Context) {
	t.Helper()
	tmp, err := os.CreateTemp("", "sr-identity-stats-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	t.Cleanup(func() { os.Remove(tmp.Name()) })

	db, err := NewDB(config.DatabaseConfig{Driver: "sqlite", DSN: tmp.Name()})
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.RunMigrations(context.Background(), db, "sqlite", zap.NewNop()); err != nil {
		t.Fatalf("migrations failed: %v", err)
	}
	return NewSRIdentityStatsRepo(db), db, context.Background()
}

var statsDay = time.Date(2026, 8, 20, 6, 0, 0, 0, time.UTC)

// insertStat 寫一列並把 created_at 固定成指定時刻。
//
// **created_at 一定要自己覆寫**：Insert 走的是 DDL 的 CURRENT_TIMESTAMP，
// 由 sqlite 自己格式化，與 Go 綁進去的 time.Time 不是同一種字串格式，混用會讓
// 日期比較的結果取決於格式而不是取決於時間。
func insertStat(t *testing.T, repo SRIdentityStatsRepo, db *sqlx.DB, ctx context.Context, row SRIdentityStats, at time.Time) {
	t.Helper()
	if err := repo.Insert(ctx, &row); err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	if _, err := db.ExecContext(ctx, db.Rebind(
		`UPDATE sr_identity_stats SET created_at = ? WHERE analysis_id = ?`), at, row.AnalysisID); err != nil {
		t.Fatalf("backdate failed: %v", err)
	}
}

// 本筆的驗收準則：limit 只截斷明細，聚合一律涵蓋完整區間。
//
// 修法前 summary 是對「被 limit 截斷的那批」加總，所以 alias_hit_rate 的分母
// 由 limit 決定而不是由 days 決定——這支測試就是在擋那件事回來。
func TestSRIdentityStatsSummarizeIgnoresLimit(t *testing.T) {
	repo, db, ctx := newSRIdentityStatsRepoForTest(t)
	for i := 1; i <= 5; i++ {
		insertStat(t, repo, db, ctx, SRIdentityStats{
			AnalysisID: uint64(i), Symbol: "2330", Timeframe: "1d",
			MatchedByChain: 2, MatchedByAlias: 1,
		}, statsDay)
	}

	rows, err := repo.List(ctx, SRIdentityStatsQuery{Limit: 1})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("limit=1 應只回 1 列，得到 %d", len(rows))
	}

	small, err := repo.Summarize(ctx, SRIdentityStatsQuery{Limit: 1})
	if err != nil {
		t.Fatalf("summarize failed: %v", err)
	}
	large, err := repo.Summarize(ctx, SRIdentityStatsQuery{Limit: 1000})
	if err != nil {
		t.Fatalf("summarize failed: %v", err)
	}
	if small != large {
		t.Fatalf("limit 不該影響聚合：limit=1 %+v vs limit=1000 %+v", small, large)
	}
	if small.Analyses != 5 || small.MatchedByChain != 10 || small.MatchedByAlias != 5 {
		t.Errorf("聚合應涵蓋全部 5 列：%+v", small)
	}
}

// 每一欄餵互不相同的值，逐欄斷言。
//
// **這支專門抓「SQL 選錯欄位」**：limit / 日期 / 空集合 / degraded 那幾支用的資料
// 都不需要各欄可區分，aggregate 少一個 SUM、或把 SUM(alias_ambiguous) 掃進
// chain_conflicts，那幾支照樣全綠。而 handler 改吃原始計數之後，這一層是唯一的守門。
func TestSRIdentityStatsSummarizeMapsEveryColumn(t *testing.T) {
	repo, db, ctx := newSRIdentityStatsRepoForTest(t)
	// 不在 aggregate 裡的欄位刻意塞大值：若 SQL 誤把它們加進來，下面的斷言會炸。
	insertStat(t, repo, db, ctx, SRIdentityStats{
		AnalysisID: 1, Symbol: "2330", Timeframe: "1d",
		MatchedByChain: 1, MatchedByCurrent: 2, MatchedByAlias: 3,
		UnmatchedKeys: 4, ChainConflicts: 5, ChainKeyAmbiguous: 6,
		AliasAmbiguous: 7, CarriedParseFail: 8, InvariantViolations: 9,
		CarriedNoop: 1000, ZoneEndedSkipped: 2000, ZoneLiveCandidates: 3000, ZoneEnded: 4000,
		EventIdentityDegraded: true,
	}, statsDay)
	insertStat(t, repo, db, ctx, SRIdentityStats{
		AnalysisID: 2, Symbol: "2330", Timeframe: "1d",
		MatchedByChain: 10, MatchedByCurrent: 20, MatchedByAlias: 30,
		UnmatchedKeys: 40, ChainConflicts: 50, ChainKeyAmbiguous: 60,
		AliasAmbiguous: 70, CarriedParseFail: 80, InvariantViolations: 90,
		ZoneIdentityDegraded: true,
	}, statsDay)

	got, err := repo.Summarize(ctx, SRIdentityStatsQuery{})
	if err != nil {
		t.Fatalf("summarize failed: %v", err)
	}
	want := SRIdentityStatsAggregate{
		Analyses: 2, Degraded: 1,
		MatchedByChain: 11, MatchedByCurrent: 22, MatchedByAlias: 33,
		UnmatchedKeys: 44, ChainConflicts: 55, ChainKeyAmbiguous: 66,
		AliasAmbiguous: 77, CarriedParseFail: 88, InvariantViolations: 99,
	}
	if got != want {
		t.Errorf("逐欄聚合不符\n got: %+v\nwant: %+v", got, want)
	}
}

// per-symbol 趨勢是正式 API contract：List 與 Summarize 必須落在同一個母體。
//
// 共用 builder 只保證「兩邊拼出同一份 WHERE」，**不保證那份 WHERE 是對的**——
// 重構時把 symbol 一起漏掉，兩邊仍然一致、其餘測試全綠，而 ?symbol=2330 的
// summary 會悄悄聚合全市場。
func TestSRIdentityStatsSummarizeFiltersBySymbol(t *testing.T) {
	repo, db, ctx := newSRIdentityStatsRepoForTest(t)
	insertStat(t, repo, db, ctx, SRIdentityStats{AnalysisID: 1, Symbol: "2330", Timeframe: "1d", MatchedByChain: 1}, statsDay)
	insertStat(t, repo, db, ctx, SRIdentityStats{AnalysisID: 2, Symbol: "2330", Timeframe: "1d", MatchedByChain: 2}, statsDay)
	insertStat(t, repo, db, ctx, SRIdentityStats{AnalysisID: 3, Symbol: "0050", Timeframe: "1d", MatchedByChain: 100}, statsDay)

	q := SRIdentityStatsQuery{Symbol: "2330"}
	rows, err := repo.List(ctx, q)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	agg, err := repo.Summarize(ctx, q)
	if err != nil {
		t.Fatalf("summarize failed: %v", err)
	}
	if agg.Analyses != len(rows) {
		t.Fatalf("同一個 symbol 母體應一致：summary %d vs rows %d", agg.Analyses, len(rows))
	}
	if agg.Analyses != 2 || agg.MatchedByChain != 3 {
		t.Errorf("symbol=2330 不該含 0050 的那列：%+v", agg)
	}

	all, err := repo.Summarize(ctx, SRIdentityStatsQuery{})
	if err != nil {
		t.Fatalf("summarize failed: %v", err)
	}
	if all.Analyses != 3 || all.MatchedByChain != 103 {
		t.Errorf("省略 symbol 應涵蓋全部：%+v", all)
	}
}

// From / To 用的是 >= 與 <=，**邊界那天要落在區間內**，而且兩邊判定一致。
func TestSRIdentityStatsSummarizeDateBoundaries(t *testing.T) {
	repo, db, ctx := newSRIdentityStatsRepoForTest(t)
	d1 := statsDay
	d2 := statsDay.AddDate(0, 0, 1)
	d3 := statsDay.AddDate(0, 0, 2)
	insertStat(t, repo, db, ctx, SRIdentityStats{AnalysisID: 1, Symbol: "2330", Timeframe: "1d", MatchedByChain: 1}, d1)
	insertStat(t, repo, db, ctx, SRIdentityStats{AnalysisID: 2, Symbol: "2330", Timeframe: "1d", MatchedByChain: 2}, d2)
	insertStat(t, repo, db, ctx, SRIdentityStats{AnalysisID: 3, Symbol: "2330", Timeframe: "1d", MatchedByChain: 4}, d3)

	cases := map[string]struct {
		q          SRIdentityStatsQuery
		wantRows   int
		wantChains int
	}{
		"含頭尾兩端":  {SRIdentityStatsQuery{From: d1, To: d3}, 3, 7},
		"邊界日本身要含": {SRIdentityStatsQuery{From: d2, To: d2}, 1, 2},
		"只給 From": {SRIdentityStatsQuery{From: d2}, 2, 6},
		"只給 To":   {SRIdentityStatsQuery{To: d2}, 2, 3},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rows, err := repo.List(ctx, tc.q)
			if err != nil {
				t.Fatalf("list failed: %v", err)
			}
			agg, err := repo.Summarize(ctx, tc.q)
			if err != nil {
				t.Fatalf("summarize failed: %v", err)
			}
			if len(rows) != tc.wantRows || agg.Analyses != tc.wantRows {
				t.Fatalf("列數不符：rows %d、summary %d，want %d", len(rows), agg.Analyses, tc.wantRows)
			}
			if agg.MatchedByChain != tc.wantChains {
				t.Errorf("matched_by_chain = %d, want %d", agg.MatchedByChain, tc.wantChains)
			}
		})
	}
}

// 空集合：COUNT(*) 回 0、SUM 回 NULL 由 COALESCE 收成 0，不能回 error 也不能掃出 NULL。
func TestSRIdentityStatsSummarizeEmpty(t *testing.T) {
	repo, _, ctx := newSRIdentityStatsRepoForTest(t)
	got, err := repo.Summarize(ctx, SRIdentityStatsQuery{Symbol: "9999"})
	if err != nil {
		t.Fatalf("空集合不該回 error：%v", err)
	}
	if got != (SRIdentityStatsAggregate{}) {
		t.Errorf("空集合應全為 0：%+v", got)
	}
}
