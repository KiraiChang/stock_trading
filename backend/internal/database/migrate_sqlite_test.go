package database

import (
	"context"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"

	"github.com/pressly/goose/v3"
	"go.uber.org/zap"
)

// sqlite 的 migration 回滾驗證。
//
// `internal/store` 的每支測試都會實跑 sqlite 的 goose **Up**，但**沒有任何流程跑過 Down**——
// 017／018 的 Down 曾經只 DROP 不還原、讓回滾鏈在 016 斷掉，就是因此一直沒被發現
// （規範見 docs/development-workflow.md 的「migration 的 Down 區塊也要能跑」）。
// 這支測試補上 Down 那一半。
//
// 與 mysql 版不同，這支**不需要環境變數也不需要 container**：sqlite 用暫存檔就能跑，
// 所以它會在每次 `backend/scripts/test.sh` 都執行，是回滾鏈的常態回歸保護。
func TestSQLiteMigrationsUpAndDown(t *testing.T) {
	tmp, err := os.CreateTemp("", "sqlite-migration-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	t.Cleanup(func() { os.Remove(tmp.Name()) })

	db, err := sqlx.Connect("sqlite", tmp.Name())
	if err != nil {
		t.Fatalf("連不上 sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	// 與 store.NewSQLite 一致：sqlite 建議單一 writer。
	db.SetMaxOpenConns(1)

	ctx := context.Background()

	if err := RunMigrations(ctx, db, "sqlite", zap.NewNop()); err != nil {
		t.Fatalf("sqlite migration up 失敗: %v", err)
	}

	goose.SetBaseFS(sqliteFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("goose set dialect: %v", err)
	}

	// 分段回滾，中途檢查結構——不是為了好看，而是因為「一路 down 到 0 沒報錯」**驗不到**
	// 017／018 的 Down 重建得對不對：017 的 Down 一執行就把 018 重建的表砍掉了，
	// 018 的 Down 內容只要不出錯就會過。所以在越過每一筆之後立刻檢查表的形狀。
	//
	// 停在 17 ＝ 018 已回滾，表應該回到 017 版的形狀。
	if err := goose.DownToContext(ctx, db.DB, "migrations/sqlite", 17); err != nil {
		t.Fatalf("sqlite migration down 到 17 失敗: %v", err)
	}
	assertColumns(t, db, "stock_sr_zones",
		// 017 版獨有的欄位（018 才會再改掉），確認 018 的 Down 還原的是 017 而不是別的版本
		"net_score", "net_score_label", "confidence_level", "reject_count", "break_count",
		"zone_momentum", "trading_score", "trading_recommendation")
	assertColumns(t, db, "stock_sr_zone_analyses", "overall_trend", "overall_volatility")

	// 停在 16 ＝ 017 已回滾，表應該回到「015 ＋ 016」的形狀。
	if err := goose.DownToContext(ctx, db.DB, "migrations/sqlite", 16); err != nil {
		t.Fatalf("sqlite migration down 到 16 失敗: %v", err)
	}
	assertColumns(t, db, "stock_sr_zones",
		// 015 的欄位 ＋ 016 加的三欄。confidence 必須在，否則 016 的 Down 會炸——
		// 這正是回滾鏈曾經斷掉的地方。
		"rejection_count", "breakout_count", "avg_return_after_touch", "trend_strength",
		"confidence", "expected_value", "risk_reward_ratio")
	assertNoColumns(t, db, "stock_sr_zones", "net_score", "confidence_level", "trading_score")

	if err := goose.DownToContext(ctx, db.DB, "migrations/sqlite", 0); err != nil {
		t.Fatalf("sqlite migration down 到 0 失敗: %v", err)
	}

	if v, err := goose.GetDBVersion(db.DB); err != nil {
		t.Fatalf("讀不到 goose 版本: %v", err)
	} else if v != 0 {
		t.Fatalf("down 之後版本 = %d, want 0", v)
	}

	// goose 不會刪自己的版本表；除此之外應該一張表都不剩。
	var names []string
	if err := db.Select(&names, `
		SELECT name FROM sqlite_master
		WHERE type='table' AND name <> 'goose_db_version' AND name NOT LIKE 'sqlite_%'
		ORDER BY name
	`); err != nil {
		t.Fatalf("列表失敗: %v", err)
	}
	if len(names) != 0 {
		t.Fatalf("down 到 0 之後還剩 %d 張表，代表有 migration 的 Down 沒清乾淨: %v", len(names), names)
	}
}

func sqliteColumns(t *testing.T, db *sqlx.DB, table string) map[string]bool {
	t.Helper()
	var names []string
	if err := db.Select(&names, `SELECT name FROM pragma_table_info(?)`, table); err != nil {
		t.Fatalf("讀 %s 的欄位失敗: %v", table, err)
	}
	if len(names) == 0 {
		t.Fatalf("表 %s 不存在或沒有欄位", table)
	}
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return set
}

func assertColumns(t *testing.T, db *sqlx.DB, table string, want ...string) {
	t.Helper()
	cols := sqliteColumns(t, db, table)
	for _, c := range want {
		if !cols[c] {
			t.Errorf("%s 缺少欄位 %q——Down 還原的結構不是預期的版本", table, c)
		}
	}
}

func assertNoColumns(t *testing.T, db *sqlx.DB, table string, unwanted ...string) {
	t.Helper()
	cols := sqliteColumns(t, db, table)
	for _, c := range unwanted {
		if cols[c] {
			t.Errorf("%s 不該有欄位 %q——回滾沒有退到正確的版本", table, c)
		}
	}
}
