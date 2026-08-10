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

// 060 的 CHECK 約束驗證（見 docs/issue.md I-064）。
//
// 為什麼要測「資料有沒有活過重建」：sqlite 沒有 ALTER TABLE ADD CONSTRAINT，060 只能
// 整張表重建。重建寫錯（漏 INSERT ... SELECT、欄位順序錯位）不會讓 migration 失敗，
// 只會安靜地把 candles 清空——正是 development-workflow.md 要求「越過每一筆之後立刻
// 檢查表的形狀」的那類錯誤。
func TestSQLiteCandlePositivePriceConstraint(t *testing.T) {
	tmp, err := os.CreateTemp("", "sqlite-candle-check-*.db")
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
	db.SetMaxOpenConns(1)

	ctx := context.Background()
	goose.SetBaseFS(sqliteFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("goose set dialect: %v", err)
	}

	// 先停在 059（約束還沒加），塞一列正常資料，用來驗證 060 的重建有沒有搬走資料。
	if err := goose.UpToContext(ctx, db.DB, "migrations/sqlite", 59); err != nil {
		t.Fatalf("up 到 59 失敗: %v", err)
	}
	insert := `INSERT INTO candles (symbol, timeframe, open, high, low, close, volume, amount, ts)
	           VALUES (?, '1d', ?, ?, ?, ?, 1000, 0, '2026-01-01 00:00:00')`
	if _, err := db.Exec(insert, "2330", 99.0, 101.0, 98.0, 100.0); err != nil {
		t.Fatalf("059 版寫入正常 K 棒失敗: %v", err)
	}

	if err := goose.UpToContext(ctx, db.DB, "migrations/sqlite", 60); err != nil {
		t.Fatalf("up 到 60 失敗: %v", err)
	}

	var count int
	if err := db.Get(&count, `SELECT COUNT(*) FROM candles WHERE symbol = '2330'`); err != nil {
		t.Fatalf("讀 candles 失敗: %v", err)
	}
	if count != 1 {
		t.Fatalf("060 重建後既有資料剩 %d 列, want 1——重建沒有把資料搬過去", count)
	}

	// 約束生效：全零 K 棒（就是 I-064 那 4 列的形狀）必須被擋下。
	if _, err := db.Exec(insert, "3630", 0.0, 0.0, 0.0, 0.0); err == nil {
		t.Fatal("全零 K 棒竟然寫得進去——CHECK 約束沒生效")
	}
	// 單一欄位為 0 也要擋。
	if _, err := db.Exec(insert, "3631", 10.0, 11.0, 0.0, 10.0); err == nil {
		t.Fatal("low=0 的 K 棒竟然寫得進去——CHECK 約束沒涵蓋所有價格欄位")
	}
	// 正常的仍然寫得進去，別擋過頭。
	if _, err := db.Exec(insert, "2454", 50.0, 52.0, 49.0, 51.0); err != nil {
		t.Fatalf("正常 K 棒被擋掉了: %v", err)
	}

	// Down 是真逆操作：約束消失、資料仍在。
	if err := goose.DownToContext(ctx, db.DB, "migrations/sqlite", 59); err != nil {
		t.Fatalf("down 到 59 失敗: %v", err)
	}
	if err := db.Get(&count, `SELECT COUNT(*) FROM candles`); err != nil {
		t.Fatalf("down 後讀 candles 失敗: %v", err)
	}
	if count != 2 {
		t.Fatalf("down 後資料剩 %d 列, want 2——Down 的重建沒有把資料搬回去", count)
	}
	if _, err := db.Exec(insert, "3630", 0.0, 0.0, 0.0, 0.0); err != nil {
		t.Fatalf("down 之後約束應該已移除，但寫入仍被擋: %v", err)
	}
}
