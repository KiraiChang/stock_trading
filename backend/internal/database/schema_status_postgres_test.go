package database

import (
	"context"
	"embed"
	"errors"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"
	"go.uber.org/zap"
)

// T-087 在真的 PostgreSQL 上的部分：唯讀交易裡的狀態讀取、migrator 連線的啟動參數（application_name、lock_timeout、
// statement_timeout）、有人持有表鎖時在 lock_timeout 內失敗、整體期限到了會停下。
// 跟其他 postgres migration 測試一樣用 POSTGRES_MIGRATION_DSN gate 住（scripts/test-postgres-migrations.sh）；
// 每個測試另建一個 database，⛔ 不動到其他測試在用的 trading。

//go:embed testdata/t087/pglock/*.sql
var t087PGFS embed.FS

func t087PGDB(t *testing.T, name string) (dsn string, db *sqlx.DB) {
	t.Helper()
	admin := postgresTestDB(t)
	ctx := context.Background()
	if _, err := admin.ExecContext(ctx, "DROP DATABASE IF EXISTS "+name); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(os.Getenv("POSTGRES_MIGRATION_DSN"))
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	dsn = u.String()
	db, err = sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
		_, _ = admin.ExecContext(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
	})
	return dsn, db
}

func TestPostgresReadStatusReadOnly(t *testing.T) {
	_, db := t087PGDB(t, "t087_ro")
	ctx := context.Background()
	if _, err := ReadStatus(ctx, db, "postgres"); !errors.Is(err, ErrNoVersionTable) {
		t.Fatalf("版本表不存在：%v", err)
	}
	var exists bool
	if err := db.Get(&exists, `SELECT to_regclass('goose_db_version') IS NOT NULL`); err != nil || exists {
		t.Fatalf("唯讀檢查⛔ 不能建表：%v %v", exists, err)
	}
	if err := RunMigrations(ctx, db, "postgres", zap.NewNop()); err != nil {
		t.Fatal(err)
	}
	if s, err := CheckApplied(ctx, db, "postgres"); err != nil || len(s.Pending) != 0 {
		t.Fatalf("全部已套用：%v %+v", err, s)
	}
	// 實際跑一次 goose down（撤回最後一支；goose 會刪掉它的紀錄）
	goose.SetBaseFS(postgresFS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	if err := goose.DownContext(ctx, db.DB, postgresMigrationDir); err != nil {
		t.Fatalf("goose down：%v", err)
	}
	req, _ := RequiredVersions("postgres")
	var pe *PendingError
	if _, err := CheckApplied(ctx, db, "postgres"); !errors.As(err, &pe) || JoinVersions(pe.Pending) != JoinVersions(req[len(req)-1:]) {
		t.Fatalf("goose down 之後要列出最後一支：%v", err)
	}
	if err := RunMigrations(ctx, db, "postgres", zap.NewNop()); err != nil {
		t.Fatal(err)
	}
	// 缺中間一支（刪掉 079 的紀錄；最大版本照舊）
	if _, err := db.Exec(`DELETE FROM goose_db_version WHERE version_id = 79`); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckApplied(ctx, db, "postgres"); !errors.As(err, &pe) || JoinVersions(pe.Pending) != "79" {
		t.Fatalf("缺 079：%v", err)
	}
}

func TestPostgresMigratorConnParams(t *testing.T) {
	dsn, db := t087PGDB(t, "t087_conn")
	m, err := OpenPostgresMigrator(dsn, MigratorConnOptions{ApplicationName: "trading-migrator-t087test", LockTimeout: time.Second, StatementTimeout: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	var lock, stmt, app string
	if err := m.Get(&lock, `SHOW lock_timeout`); err != nil || lock != "1s" {
		t.Fatalf("lock_timeout：%q %v", lock, err)
	}
	if err := m.Get(&stmt, `SHOW statement_timeout`); err != nil || stmt != "2s" {
		t.Fatalf("statement_timeout：%q %v", stmt, err)
	}
	if err := m.Get(&app, `SELECT current_setting('application_name')`); err != nil || app != "trading-migrator-t087test" {
		t.Fatalf("application_name：%q %v", app, err)
	}
	var n int
	if err := db.Get(&n, `SELECT count(*) FROM pg_stat_activity WHERE application_name = 'trading-migrator-t087test' AND datname = 't087_conn'`); err != nil || n < 1 {
		t.Fatalf("pg_stat_activity 要認得出本次的 migrator：%d %v", n, err)
	}
	if _, err := OpenPostgresMigrator("not a dsn at all ://", MigratorConnOptions{}); err == nil || err.Error() != "parse database dsn failed" {
		t.Fatalf("錯誤⛔ 不帶 DSN：%v", err)
	}
}

// applyFirst 用 goose 只套用第 1 支（t087_lock 表）。
func applyFirst(t *testing.T, db *sqlx.DB) {
	t.Helper()
	goose.SetBaseFS(t087PGFS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	if err := goose.UpToContext(context.Background(), db.DB, "testdata/t087/pglock", 1); err != nil {
		t.Fatal(err)
	}
}

// holdLock：另一個連線在交易裡持有 t087_lock 的 ACCESS EXCLUSIVE（測試結束才放）。
func holdLock(t *testing.T, db *sqlx.DB) func() {
	t.Helper()
	tx, err := db.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`LOCK TABLE t087_lock IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	return func() { _ = tx.Rollback() }
}

var pgLockSrc = migrationSource{t087PGFS, "postgres", "testdata/t087/pglock"}

func TestPostgresMigrateUpLockTimeout(t *testing.T) {
	dsn, db := t087PGDB(t, "t087_lock")
	applyFirst(t, db)
	release := holdLock(t, db)
	defer release()

	m, err := OpenPostgresMigrator(dsn, MigratorConnOptions{ApplicationName: "trading-migrator-t087lock", LockTimeout: time.Second, StatementTimeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	start := time.Now()
	n, err := migrateUp(context.Background(), m, pgLockSrc, nil)
	var pe *pgconn.PgError
	if n != 0 || !errors.As(err, &pe) || pe.Code != "55P03" || time.Since(start) > 5*time.Second {
		t.Fatalf("有人持有表鎖：要在 lock_timeout 內失敗（55P03）、⛔ 不無限等：n=%d err=%v %s", n, err, time.Since(start))
	}
	s, err := ReadStatus(context.Background(), db, "postgres")
	if err != nil {
		// required 是正式的 embed 清單；這裡只看已套用集合
		t.Fatal(err)
	}
	if JoinVersions(s.Applied) != "1" {
		t.Fatalf("第 2 支⛔ 不能算已套用：%+v", s.Applied)
	}
	release()
	if n, err := migrateUp(context.Background(), m, pgLockSrc, nil); err != nil || n != 1 {
		t.Fatalf("鎖放掉之後重跑要成功：n=%d err=%v", n, err)
	}
}

func TestPostgresMigrateUpOverallDeadline(t *testing.T) {
	dsn, db := t087PGDB(t, "t087_deadline")
	applyFirst(t, db)
	release := holdLock(t, db)
	defer release()

	// lock_timeout 與 statement_timeout 都⛔ 不設：只靠整體期限停下
	m, err := OpenPostgresMigrator(dsn, MigratorConnOptions{ApplicationName: "trading-migrator-t087deadline"})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	start := time.Now()
	n, err := migrateUp(ctx, m, pgLockSrc, nil)
	if n != 0 || !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 5*time.Second {
		t.Fatalf("整體期限：要停下、回 DeadlineExceeded：n=%d err=%v %s", n, err, time.Since(start))
	}
	// 期限到了之後，DB 上⛔ 沒有這次 migrator 還在跑的語句（pgx 送出取消、交易回滾）
	deadline := time.Now().Add(5 * time.Second)
	for {
		var active int
		if err := db.Get(&active, `SELECT count(*) FROM pg_stat_activity WHERE application_name = 'trading-migrator-t087deadline' AND state = 'active'`); err != nil {
			t.Fatal(err)
		}
		if active == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("期限到了之後 5 秒仍有 %d 個 active 的語句", active)
		}
		time.Sleep(100 * time.Millisecond)
	}
	var cols int
	if err := db.Get(&cols, `SELECT count(*) FROM information_schema.columns WHERE table_name = 't087_lock' AND column_name = 'note'`); err != nil || cols != 0 {
		t.Fatalf("被期限停下的那支要回滾：%d %v", cols, err)
	}
}

// T-087 review：實例識別——同一個 database 的兩條連線相同（一般連線池與 migrator 連線），不同的 database 不同（版本可以完全一樣）。
func TestPostgresMigratorInstanceIdentity(t *testing.T) {
	dsnA, dbA := t087PGDB(t, "t087_inst_a")
	_, dbB := t087PGDB(t, "t087_inst_b")
	ctx := context.Background()
	for _, db := range []*sqlx.DB{dbA, dbB} {
		if err := RunMigrations(ctx, db, "postgres", zap.NewNop()); err != nil {
			t.Fatal(err)
		}
	}
	m, err := OpenPostgresMigrator(dsnA, MigratorConnOptions{ApplicationName: "trading-migrator-t087inst"})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	a1, err1 := ReadInstance(ctx, dbA)
	a2, err2 := ReadInstance(ctx, m)
	b1, err3 := ReadInstance(ctx, dbB)
	if err1 != nil || err2 != nil || err3 != nil || a1 == "" || a1 != a2 || a1 == b1 {
		t.Fatalf("實例識別：a=%q a'=%q b=%q（%v %v %v）", a1, a2, b1, err1, err2, err3)
	}
	sa, _ := ReadStatus(ctx, dbA, "postgres")
	sb, _ := ReadStatus(ctx, dbB, "postgres")
	if sa.Fingerprint != sb.Fingerprint {
		t.Fatal("反例的前提：兩個 database 的版本指紋相同")
	}
}
