package database_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/trading/backend/internal/store"
)

// I-140：signals.timeframe（migration 081）與 created_at 由程式寫入、截到整秒，在 PostgreSQL 與 MySQL 上的行為（SQLite 由 store 的單元測試涵蓋）。
// 函式名一定要是 TestPostgresMigrations…／TestMySQLMigrations… 前綴（兩支腳本用 -test.run 篩名稱，⛔ 不合前綴的不會跑）。

func TestPostgresMigrationsSignalTimeframe(t *testing.T) {
	dsn := skipUnlessDSN(t, "POSTGRES_MIGRATION_DSN", "scripts/test-postgres-migrations.sh")
	runSignalTimeframe(t, migratedDB(t, "postgres", "pgx", dsn), false)
}

func TestMySQLMigrationsSignalTimeframe(t *testing.T) {
	dsn := skipUnlessDSN(t, "MYSQL_MIGRATION_DSN", "scripts/test-mysql-migrations.sh")
	runSignalTimeframe(t, migratedDB(t, "mysql", "mysql", dsn), true)
}

func runSignalTimeframe(t *testing.T, db *sqlx.DB, mysql bool) {
	t.Helper()
	ctx := context.Background()
	repo := store.NewSignalRepo(db)
	base := time.Date(2031, 1, 6, 1, 1, 0, 0, time.UTC)
	tf := func(s string) store.NullString {
		return store.NullString{NullString: sql.NullString{String: s, Valid: true}}
	}
	ins := func(s store.Signal) store.Signal {
		t.Helper()
		s.Direction, s.Price, s.Volume, s.Timestamp = "WATCH", 100, 1000, base
		if err := repo.Insert(ctx, &s); err != nil {
			t.Fatalf("insert: %v", err)
		}
		return s
	}

	// 帶小數秒交給 repo：要截成整秒（⛔ 不是四捨五入成下一秒），讀回一模一樣。
	a := ins(store.Signal{Symbol: "2330", SignalType: "SUPPORT_BOUNCE", Support: 95.5, Timeframe: tf("1m"), CreatedAt: base.Add(700 * time.Millisecond)})
	ins(store.Signal{Symbol: "2330", SignalType: "SUPPORT_BOUNCE", Support: 96, CreatedAt: base.Add(time.Minute)}) // NULL＝舊列
	ins(store.Signal{Symbol: "2330", SignalType: "SUPPORT_BOUNCE", Support: 97, Timeframe: tf("1d"), CreatedAt: base.Add(time.Minute)})
	got, err := repo.LatestBySymbolTypeTimeframe(ctx, "2330", "SUPPORT_BOUNCE", "1m")
	if err != nil || got == nil || got.ID != a.ID {
		t.Fatalf("錨點要是 1m 那一列（NULL 與 1d ⛔ 不算）：%+v %v", got, err)
	}
	if !got.CreatedAt.Equal(base) || got.Timeframe.String != "1m" {
		t.Fatalf("created_at 讀回要等於截秒後的 %v：%+v", base, got)
	}

	// 寫入順序跟 created_at 相反（後寫入的時間較早、用 +08:00 表示）：錨點依 created_at，同一秒依 id（複審 P2-2）。
	newer := ins(store.Signal{Symbol: "1101", SignalType: "SUPPORT_BOUNCE", Timeframe: tf("1m"), CreatedAt: base.Add(19 * time.Minute)})
	ins(store.Signal{Symbol: "1101", SignalType: "SUPPORT_BOUNCE", Timeframe: tf("1m"), CreatedAt: base.Add(3 * time.Minute).In(time.FixedZone("TPE", 8*3600))})
	if g, err := repo.LatestBySymbolTypeTimeframe(ctx, "1101", "SUPPORT_BOUNCE", "1m"); err != nil || g == nil || g.ID != newer.ID {
		t.Fatalf("錨點要是 created_at 最新的一列：%+v %v", g, err)
	}
	tie := ins(store.Signal{Symbol: "1101", SignalType: "SUPPORT_BOUNCE", Timeframe: tf("1m"), CreatedAt: base.Add(19 * time.Minute)})
	if g, _ := repo.LatestBySymbolTypeTimeframe(ctx, "1101", "SUPPORT_BOUNCE", "1m"); g == nil || g.ID != tie.ID {
		t.Fatalf("同一秒要取 id 大的：%+v", g)
	}

	// 證明為什麼程式要先截秒：直接把帶 0.7 秒的值寫進 DB，MySQL 的 DATETIME(0) 會四捨五入成下一秒，PostgreSQL 保留小數秒。
	raw := base.Add(time.Hour + 700*time.Millisecond)
	if _, err := db.ExecContext(ctx, db.Rebind(`INSERT INTO signals (symbol, signal_type, direction, price, volume, vol_ratio, resistance, support, trend, note, strength, ts, timeframe, created_at)
		VALUES (?, 'SUPPORT_BOUNCE', 'WATCH', 100, 1000, 1, 0, 95, 'SIDEWAYS', '', 1, ?, '1m', ?)`), "9999", base, raw); err != nil {
		t.Fatalf("raw insert: %v", err)
	}
	r, err := repo.LatestBySymbolTypeTimeframe(ctx, "9999", "SUPPORT_BOUNCE", "1m")
	if err != nil || r == nil {
		t.Fatalf("raw 讀回：%+v %v", r, err)
	}
	want := raw
	if mysql {
		want = base.Add(time.Hour + time.Second) // 四捨五入
	}
	if !r.CreatedAt.Equal(want) {
		t.Fatalf("直接寫入 %v 讀回 %v，預期 %v", raw, r.CreatedAt, want)
	}
}
