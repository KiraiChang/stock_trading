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

// **只跑 sqlite**。這是 issue.md I-054 第 1 項的既有限制：repo 層的 CRUD 從未對真實
// MySQL 驗證過，本 repo 又多了一個未驗證的案例。mysql 的 DDL 由
// scripts/test-mysql-migrations.sh 涵蓋，CRUD 沒有。
func newUniverseRepoForTest(t *testing.T) (EvaluationUniverseRepo, context.Context) {
	t.Helper()
	tmp, err := os.CreateTemp("", "universe-test-*.db")
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
	return NewEvaluationUniverseRepo(db), context.Background()
}

func universeEntry(symbol, bucket string) EvaluationUniverseEntry {
	return EvaluationUniverseEntry{
		Symbol:     symbol,
		BucketHint: bucket,
		// 2026-08-17 凍結的分位數邊界，等於 zone_builder 的 LOW/HIGH_VOLATILITY_THRESHOLD
		BucketEdgeLow:   0.046089927430152715,
		BucketEdgeHigh:  0.06278197721225691,
		UniverseVersion: "v2",
		SelectedAt:      time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC),
		Source:          "T-040_STEP3",
		Note:            "",
	}
}

func TestEvaluationUniverseUpsertIsIdempotentAndUpdates(t *testing.T) {
	repo, ctx := newUniverseRepoForTest(t)

	e := universeEntry("2330", "LOW_VOLATILITY")
	if err := repo.Upsert(ctx, []EvaluationUniverseEntry{e}); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	// 重新匯入 selection report 是常態動作（門檻重定後），insert 會撞 UNIQUE。
	e.BucketHint = "NORMAL_VOLATILITY"
	e.UniverseVersion = "v3"
	if err := repo.Upsert(ctx, []EvaluationUniverseEntry{e}); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	rows, err := repo.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("upsert 產生了第二列：%d", len(rows))
	}
	if rows[0].BucketHint != "NORMAL_VOLATILITY" || rows[0].UniverseVersion != "v3" {
		t.Fatalf("欄位沒被更新：%+v", rows[0])
	}
}

func TestEvaluationUniverseUpsertDefaultsRoleAndKeepsActive(t *testing.T) {
	repo, ctx := newUniverseRepoForTest(t)

	if err := repo.Upsert(ctx, []EvaluationUniverseEntry{universeEntry("2330", "LOW_VOLATILITY")}); err != nil {
		t.Fatal(err)
	}
	rows, _ := repo.List(ctx)
	if rows[0].UniverseRole != "primary" {
		t.Fatalf("role 未預設為 primary：%q", rows[0].UniverseRole)
	}
	if !rows[0].Active {
		t.Fatal("新入池應為 active")
	}

	// 人工停用後重新匯入，**不該把 active 靜默改回 true**——停用是獨立的人工決定。
	if ok, err := repo.SetActive(ctx, "2330", false); err != nil || !ok {
		t.Fatalf("SetActive: ok=%v err=%v", ok, err)
	}
	if err := repo.Upsert(ctx, []EvaluationUniverseEntry{universeEntry("2330", "HIGH_VOLATILITY")}); err != nil {
		t.Fatal(err)
	}
	rows, _ = repo.List(ctx)
	if rows[0].Active {
		t.Fatal("重新匯入把人工停用覆寫掉了")
	}
	if rows[0].BucketHint != "HIGH_VOLATILITY" {
		t.Fatal("其餘欄位仍應被更新")
	}
}

func TestEvaluationUniverseListActiveExcludesDisabled(t *testing.T) {
	repo, ctx := newUniverseRepoForTest(t)

	if err := repo.Upsert(ctx, []EvaluationUniverseEntry{
		universeEntry("2330", "LOW_VOLATILITY"),
		universeEntry("2454", "HIGH_VOLATILITY"),
		universeEntry("0050", "LOW_VOLATILITY"),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SetActive(ctx, "2454", false); err != nil {
		t.Fatal(err)
	}

	active, err := repo.ListActive(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// 排序固定，否則每日排程的抓取順序會漂，job 日誌無法比對
	if len(active) != 2 || active[0].Symbol != "0050" || active[1].Symbol != "2330" {
		t.Fatalf("ListActive 結果不對：%+v", active)
	}
}

func TestEvaluationUniverseSetActiveReportsMissingSymbol(t *testing.T) {
	repo, ctx := newUniverseRepoForTest(t)
	// 找不到就要回報，不能靜默成功——API 層據此回 404
	ok, err := repo.SetActive(ctx, "9999", false)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("不存在的 symbol 不該回報成功")
	}
}

func TestEvaluationUniverseUpsertRejectsZeroSelectedAt(t *testing.T) {
	repo, ctx := newUniverseRepoForTest(t)
	e := universeEntry("2330", "LOW_VOLATILITY")
	e.SelectedAt = time.Time{}
	// 零值會被寫成 0001-01-01，之後分不清「沒填」與「很久以前入池」
	if err := repo.Upsert(ctx, []EvaluationUniverseEntry{e}); err == nil {
		t.Fatal("零值 selected_at 應被拒絕")
	}
	rows, _ := repo.List(ctx)
	if len(rows) != 0 {
		t.Fatal("被拒絕的 upsert 不該留下資料")
	}
}

func TestEvaluationUniverseRejectsInvertedEdges(t *testing.T) {
	repo, ctx := newUniverseRepoForTest(t)
	e := universeEntry("2330", "LOW_VOLATILITY")
	e.BucketEdgeLow, e.BucketEdgeHigh = e.BucketEdgeHigh, e.BucketEdgeLow
	// CHECK 約束擋下：邊界顛倒代表匯入端算錯，讓它進 DB 會污染 bucket_hint 的可信度
	if err := repo.Upsert(ctx, []EvaluationUniverseEntry{e}); err == nil {
		t.Fatal("顛倒的邊界應被 CHECK 擋下")
	}
}

func TestEvaluationUniverseUpsertDoesNotMutateCallerSlice(t *testing.T) {
	repo, ctx := newUniverseRepoForTest(t)

	entries := []EvaluationUniverseEntry{universeEntry("2330", "LOW_VOLATILITY")}
	if entries[0].UniverseRole != "" {
		t.Fatal("前置條件：這個 fixture 刻意不帶 role")
	}
	if err := repo.Upsert(ctx, entries); err != nil {
		t.Fatal(err)
	}
	// 補預設值不該回寫呼叫端的資料：handler 會拿同一份 slice 回傳或寫 log，
	// 若被改寫就無法分辨「使用者指定了 primary」與「使用者沒指定」。
	if entries[0].UniverseRole != "" {
		t.Fatalf("Upsert 改寫了呼叫端的 slice：role=%q", entries[0].UniverseRole)
	}
	// 但 DB 裡仍應是 primary
	rows, _ := repo.List(ctx)
	if rows[0].UniverseRole != "primary" {
		t.Fatalf("DB 裡的 role 應補成 primary，實際 %q", rows[0].UniverseRole)
	}
}
