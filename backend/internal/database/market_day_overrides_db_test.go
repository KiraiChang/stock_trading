package database_test

import (
	"context"
	"testing"

	"github.com/jmoiron/sqlx"

	"github.com/trading/backend/internal/store"
)

// I-139：手動設定的 Insert 要在同一步寫入並讀回這次的那一列——postgres 走 RETURNING、mysql 走 transaction（SQLite 由 store 的單元測試涵蓋）。
// 函式名一定要是 TestPostgresMigrations…／TestMySQLMigrations… 前綴（兩支腳本用 -test.run 篩名稱，⛔ 不合前綴的不會跑）。

func TestPostgresMigrationsMarketDayOverrideInsert(t *testing.T) {
	dsn := skipUnlessDSN(t, "POSTGRES_MIGRATION_DSN", "scripts/test-postgres-migrations.sh")
	runMarketDayOverrideInsert(t, migratedDB(t, "postgres", "pgx", dsn))
}

func TestMySQLMigrationsMarketDayOverrideInsert(t *testing.T) {
	dsn := skipUnlessDSN(t, "MYSQL_MIGRATION_DSN", "scripts/test-mysql-migrations.sh")
	runMarketDayOverrideInsert(t, migratedDB(t, "mysql", "mysql", dsn))
}

func runMarketDayOverrideInsert(t *testing.T, db *sqlx.DB) {
	t.Helper()
	ctx := context.Background()
	repo := store.NewMarketDayOverrideRepo(db)
	a, err := repo.Insert(ctx, store.MarketDayOverride{TradeDate: "2031-06-02", Action: store.MarketDayClosed, Reason: "颱風全日休市", UserID: 7})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	b, err := repo.Insert(ctx, store.MarketDayOverride{TradeDate: "2031-06-02", Action: store.MarketDayCleared, Reason: "誤設", UserID: 8})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if a.ID == 0 || b.ID <= a.ID || a.Action != store.MarketDayClosed || a.Reason != "颱風全日休市" || a.UserID != 7 || a.CreatedAt.IsZero() ||
		b.Action != store.MarketDayCleared || b.UserID != 8 {
		t.Fatalf("Insert 要回傳這次寫入的整列：a=%+v b=%+v", a, b)
	}
	latest, err := repo.Latest(ctx, "2031-06-02")
	if err != nil || latest == nil || latest.ID != b.ID {
		t.Fatalf("最新一列要是後寫的那一列：%+v %v", latest, err)
	}
	if _, err := repo.Insert(ctx, store.MarketDayOverride{TradeDate: "2031-06-03", Action: "maybe", Reason: "x", UserID: 1}); err == nil {
		t.Fatal("action 值域外要被 CHECK 擋下")
	}
	if _, err := repo.Insert(ctx, store.MarketDayOverride{TradeDate: "20310603", Action: store.MarketDayClosed, Reason: "x", UserID: 1}); err == nil {
		t.Fatal("trade_date 格式錯誤要被 CHECK 擋下")
	}
	if o, _ := repo.Latest(ctx, "2031-06-03"); o != nil {
		t.Fatalf("被擋下的寫入⛔ 不能留下紀錄：%+v", o)
	}
}
