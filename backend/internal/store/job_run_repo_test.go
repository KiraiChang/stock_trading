package store

import (
	"context"
	"os"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/database"
)

// **只跑 sqlite**。與其他 repo 測試同一個既有限制（issue.md I-054 第 1 項）：
// mysql 的 DDL 由 scripts/test-mysql-migrations.sh 涵蓋，CRUD 沒有。
//
// 這裡跑到的 GetLatestPerJob 用了 window function（ROW_NUMBER() OVER），
// 所以這支測試同時也是「sqlite 支不支援 window function」的驗證——
// 不支援的話會在這裡爆，不會漏到 live。
func newJobRunRepoForTest(t *testing.T) (JobRunRepo, context.Context) {
	t.Helper()
	tmp, err := os.CreateTemp("", "jobrun-test-*.db")
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
	return NewJobRunRepo(db), context.Background()
}

// seedRun 直接塞一筆帶指定 started_at 的紀錄。Start() 只會寫 CURRENT_TIMESTAMP，
// 驗不了「跨多天的紀錄怎麼取」，所以這裡走原生 SQL。
func seedRun(t *testing.T, repo JobRunRepo, jobName, status string, startedAt time.Time) uint64 {
	t.Helper()
	id, err := repo.Start(context.Background(), jobName)
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}
	r := repo.(*jobRunRepo)
	if _, err := r.db.ExecContext(context.Background(),
		r.db.Rebind(`UPDATE job_runs SET status=?, started_at=? WHERE id=?`),
		status, startedAt, id,
	); err != nil {
		t.Fatalf("seed failed: %v", err)
	}
	return id
}

// GetLatestPerJob 的本體：每個 job 只回一筆，而且是最新的那筆。
func TestGetLatestPerJobReturnsNewestRunPerJob(t *testing.T) {
	repo, ctx := newJobRunRepoForTest(t)
	now := time.Now().UTC().Truncate(time.Second)

	seedRun(t, repo, "intraday", "success", now.Add(-2*time.Hour))
	seedRun(t, repo, "intraday", "failed", now.Add(-1*time.Hour)) // 最新
	seedRun(t, repo, "pre_market", "success", now.Add(-6*time.Hour))

	rows, err := repo.GetLatestPerJob(ctx)
	if err != nil {
		t.Fatalf("GetLatestPerJob failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("回傳 %d 筆，期望每個 job 各一筆（共 2 筆）", len(rows))
	}

	byJob := map[string]JobRun{}
	for _, r := range rows {
		byJob[r.JobName] = r
	}
	if got := byJob["intraday"].Status; got != "failed" {
		t.Errorf("intraday 應回最新那筆（failed），得到 %q", got)
	}
	if got := byJob["pre_market"].Status; got != "success" {
		t.Errorf("pre_market status = %q, 期望 success", got)
	}
}

// **這條是 ORDER BY 帶 id DESC 的理由**：started_at 精度到秒，同一秒起跑的兩筆
// （手動觸發撞上排程）少了 id 決勝就沒有確定順序，狀態頁會在兩筆之間跳動。
func TestGetLatestPerJobBreaksTieByID(t *testing.T) {
	repo, ctx := newJobRunRepoForTest(t)
	sameInstant := time.Now().UTC().Truncate(time.Second)

	seedRun(t, repo, "daily_close", "failed", sameInstant)
	wantID := seedRun(t, repo, "daily_close", "success", sameInstant) // 同一秒、id 較大

	rows, err := repo.GetLatestPerJob(ctx)
	if err != nil {
		t.Fatalf("GetLatestPerJob failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("回傳 %d 筆，期望 1 筆", len(rows))
	}
	if rows[0].ID != wantID {
		t.Errorf("id = %d, 期望 %d（同一秒時必須用 id 決勝）", rows[0].ID, wantID)
	}
}

// 沒有任何紀錄時回空集合而不是錯誤——handler 靠這個狀態把 job 標成 never_run。
func TestGetLatestPerJobOnEmptyTable(t *testing.T) {
	repo, ctx := newJobRunRepoForTest(t)

	rows, err := repo.GetLatestPerJob(ctx)
	if err != nil {
		t.Fatalf("空表不該回錯誤，得到 %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("空表應回 0 筆，得到 %d", len(rows))
	}
}

// **視窗問題的 repo 層佐證**（見 docs/api-reference.md 的 GET /scheduler/status）：
// 一天的 intraday 就有 55 筆，
// 比舊的 GetRecent(50) 視窗還多。GetLatestPerJob 不受這件事影響——
// 這條在同一份資料上把兩個方法擺在一起比，證明差異真的來自取數方式。
func TestGetLatestPerJobIsNotCrowdedOutByHighFrequencyJob(t *testing.T) {
	repo, ctx := newJobRunRepoForTest(t)
	day := time.Now().UTC().Truncate(time.Hour)

	seedRun(t, repo, "corporate_action_sync", "success", day.Add(-7*time.Hour))
	for i := 0; i < 55; i++ {
		seedRun(t, repo, "intraday", "success", day.Add(-5*time.Hour+time.Duration(i)*5*time.Minute))
	}

	// 對照：舊路徑取最近 50 筆，corporate_action_sync 根本不在裡面。
	recent, err := repo.GetRecent(ctx, 50)
	if err != nil {
		t.Fatalf("GetRecent failed: %v", err)
	}
	for _, r := range recent {
		if r.JobName == "corporate_action_sync" {
			t.Fatal("測試前提不成立：corporate_action_sync 不該落在最近 50 筆內")
		}
	}

	// 本體：新路徑照樣找得到它。
	rows, err := repo.GetLatestPerJob(ctx)
	if err != nil {
		t.Fatalf("GetLatestPerJob failed: %v", err)
	}
	found := false
	for _, r := range rows {
		if r.JobName == "corporate_action_sync" {
			found = true
		}
	}
	if !found {
		t.Error("corporate_action_sync 今天跑過，GetLatestPerJob 卻找不到它")
	}
}

// retention：cutoff 之前的刪掉、之後的留著。
// **對照組在同一條裡**：當天那筆一定不能被刪，否則保留期形同虛設。
func TestDeleteBeforeKeepsRunsInsideRetention(t *testing.T) {
	repo, ctx := newJobRunRepoForTest(t)
	now := time.Now().UTC().Truncate(time.Second)
	cutoff := now.AddDate(0, 0, -30)

	seedRun(t, repo, "old", "success", cutoff.AddDate(0, 0, -1))  // 31 天前 → 刪
	seedRun(t, repo, "edge", "success", cutoff.AddDate(0, 0, 1))  // 29 天前 → 留
	seedRun(t, repo, "today", "success", now)                     // 當天 → 留

	deleted, err := repo.DeleteBefore(ctx, cutoff)
	if err != nil {
		t.Fatalf("DeleteBefore failed: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("刪了 %d 筆，期望剛好 1 筆（只有 31 天前那筆該刪）", deleted)
	}

	rows, err := repo.GetLatestPerJob(ctx)
	if err != nil {
		t.Fatal(err)
	}
	remaining := map[string]bool{}
	for _, r := range rows {
		remaining[r.JobName] = true
	}
	if remaining["old"] {
		t.Error("31 天前的紀錄應該被刪掉")
	}
	if !remaining["edge"] {
		t.Error("29 天前的紀錄在保留期內，不該被刪")
	}
	if !remaining["today"] {
		t.Error("當天的紀錄不該被刪")
	}
}
