package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	_ "modernc.org/sqlite"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/database"
)

// T-087 的 -migrate 模式（結束碼與輸出）與啟動時的唯讀檢查。用 SQLite 暫存檔；PostgreSQL 的連線參數、唯讀交易與 lock_timeout
// 在 internal/database 的 PostgreSQL 測試。

func tempSQLite(t *testing.T) config.DatabaseConfig {
	t.Helper()
	f, err := os.CreateTemp("", "t087-cmd-*.db")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })
	return config.DatabaseConfig{Driver: "sqlite", DSN: f.Name()}
}

func run(t *testing.T, dc config.DatabaseConfig, o migrateOptions) (int, string) {
	t.Helper()
	var buf bytes.Buffer
	code := runMigrate(dc, o, &buf, openMigrateDB)
	out := buf.String()
	if dc.DSN != "" && strings.Contains(out, dc.DSN) {
		t.Fatalf("輸出⛔ 不能帶 DSN：\n%s", out)
	}
	return code, out
}

func openSQLite(t *testing.T, dc config.DatabaseConfig) *sqlx.DB {
	t.Helper()
	db, err := sqlx.Connect("sqlite", dc.DSN)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return db
}

func upOpts() migrateOptions {
	return migrateOptions{Mode: "up", RunID: "20261125T150000", LockTimeout: 5e9, StatementTimeout: 600e9, Overall: 900e9}
}

func TestMigrateArgs(t *testing.T) {
	dc := tempSQLite(t)
	for name, c := range map[string]struct {
		dc config.DatabaseConfig
		o  migrateOptions
	}{
		"不認得的模式":    {dc, migrateOptions{Mode: "down"}},
		"沒有 DSN":    {config.DatabaseConfig{Driver: "sqlite"}, migrateOptions{Mode: "status"}},
		"run-id 字元": {dc, migrateOptions{Mode: "status", RunID: "a;b"}},
	} {
		if code, out := run(t, c.dc, c.o); code != exitMigrateOther {
			t.Fatalf("%s：要結束碼 1：%d\n%s", name, code, out)
		}
	}
}

func TestMigrateStatusCodes(t *testing.T) {
	dc := tempSQLite(t)
	if code, out := run(t, dc, migrateOptions{Mode: "status"}); code != exitMigrateNoTable || !strings.Contains(out, "version table does not exist") {
		t.Fatalf("版本表不存在：要結束碼 2：%d\n%s", code, out)
	}
	if code, out := run(t, dc, upOpts()); code != exitMigrateOK || !strings.Contains(out, "after pending -") ||
		!strings.Contains(out, "application_name=trading-migrator-20261125T150000") {
		t.Fatalf("空的 DB 套用全部：%d\n%s", code, out)
	}
	code, out := run(t, dc, migrateOptions{Mode: "status"})
	if code != exitMigrateOK || !strings.Contains(out, "migrate: pending -") || !strings.Contains(out, "migrate: fingerprint ") {
		t.Fatalf("status：%d\n%s", code, out)
	}
	if code, out := run(t, dc, upOpts()); code != exitMigrateOK || !strings.Contains(out, "nothing to apply") {
		t.Fatalf("pending 空：%d\n%s", code, out)
	}
}

func fingerprintOf(t *testing.T, out string) string {
	t.Helper()
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "migrate: before fingerprint ") {
			return strings.TrimPrefix(l, "migrate: before fingerprint ")
		}
	}
	t.Fatalf("找不到 before fingerprint：\n%s", out)
	return ""
}

// downOne：用 goose 撤回最後一支，讓 pending 有一支
func downOne(t *testing.T, dc config.DatabaseConfig) {
	t.Helper()
	db := openSQLite(t, dc)
	if err := database.RunMigrations(context.Background(), db, "sqlite", zap.NewNop()); err != nil {
		t.Fatal(err)
	}
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err := goose.DownContext(context.Background(), db.DB, "migrations/sqlite"); err != nil {
		t.Fatal(err)
	}
	db.Close()
}

func TestMigrateUpExpectFingerprint(t *testing.T) {
	dc := tempSQLite(t)
	downOne(t, dc)
	o := upOpts()
	o.ExpectFP = strings.Repeat("0", 64)
	code, out := run(t, dc, o)
	if code != exitMigrateFingerprint || !strings.Contains(out, "nothing applied") || strings.Contains(out, "step version=") {
		t.Fatalf("指紋不符：要結束碼 5、⛔ 沒有套用：%d\n%s", code, out)
	}
	if code, out := run(t, dc, migrateOptions{Mode: "status"}); code != 0 || strings.Contains(out, "pending -") {
		t.Fatalf("指紋不符之後 pending 仍在：\n%s", out)
	}
	o.ExpectFP = fingerprintOf(t, out)
	if code, out = run(t, dc, o); code != exitMigrateOK || !strings.Contains(out, "applied 1") || !strings.Contains(out, "result=ok") {
		t.Fatalf("指紋相符：%d\n%s", code, out)
	}
}

func TestMigrateUpStepFailureAndTimeoutCodes(t *testing.T) {
	orig := migrateUpFn
	t.Cleanup(func() { migrateUpFn = orig })

	dc := tempSQLite(t)
	downOne(t, dc)
	migrateUpFn = func(ctx context.Context, db *sqlx.DB, driver string, onStep func(database.StepResult)) (int, error) {
		return 0, errors.New("boom: secret-ish detail")
	}
	code, out := run(t, dc, upOpts())
	if code != exitMigrateStepFailed || strings.Contains(out, "secret-ish") || !strings.Contains(out, "after pending ") {
		t.Fatalf("一支失敗：要結束碼 3、⛔ 不印錯誤原文、印出之後的狀態：%d\n%s", code, out)
	}

	migrateUpFn = func(ctx context.Context, db *sqlx.DB, driver string, onStep func(database.StepResult)) (int, error) {
		<-ctx.Done()
		return 0, ctx.Err()
	}
	o := upOpts()
	o.Overall = 50e6 // 50ms
	if code, out := run(t, dc, o); code != exitMigrateTimeout || !strings.Contains(out, "overall timeout") {
		t.Fatalf("整體期限：要結束碼 4：%d\n%s", code, out)
	}

	migrateUpFn = func(ctx context.Context, db *sqlx.DB, driver string, onStep func(database.StepResult)) (int, error) {
		return 0, database.ErrOutOfOrder
	}
	if code, out := run(t, dc, upOpts()); code != exitMigrateOther || !strings.Contains(out, "out of order") {
		t.Fatalf("亂序：要結束碼 1：%d\n%s", code, out)
	}
}

func TestCheckSchemaAtStartup(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	log := zap.New(core)
	dc := tempSQLite(t)
	db := openSQLite(t, dc)
	if err := checkSchema(db, "sqlite", log); err == nil || logs.FilterMessageSnippet("version table does not exist").Len() != 1 {
		t.Fatalf("空的 DB：唯讀檢查要拒絕：%v", err)
	}
	var n int
	if err := db.Get(&n, `SELECT COUNT(*) FROM sqlite_master WHERE name = 'goose_db_version'`); err != nil || n != 0 {
		t.Fatalf("唯讀檢查⛔ 不能建版本表：%d %v", n, err)
	}
	if err := database.RunMigrations(context.Background(), db, "sqlite", zap.NewNop()); err != nil {
		t.Fatal(err)
	}
	if err := checkSchema(db, "sqlite", log); err != nil || logs.FilterMessageSnippet("schema check passed").Len() != 1 {
		t.Fatalf("全部已套用要通過：%v", err)
	}
	if _, err := db.Exec(`DELETE FROM goose_db_version WHERE version_id = 79`); err != nil {
		t.Fatal(err)
	}
	if err := checkSchema(db, "sqlite", log); err == nil {
		t.Fatal("缺 079 要拒絕")
	}
	e := logs.FilterMessageSnippet("migrations not applied").All()
	if len(e) != 1 || e[0].ContextMap()["pending"] != "79" {
		t.Fatalf("log 要列出缺的版本號：%+v", e)
	}
}

// T-087 review：migrator 要核對自己連到的實例與 database 跟 deploy.sh 備份、停止確認用的是同一個（-expect-instance）。
func TestMigrateExpectInstance(t *testing.T) {
	orig := readInstanceFn
	t.Cleanup(func() { readInstanceFn = orig })

	dc := tempSQLite(t)
	if code, out := run(t, dc, migrateOptions{Mode: "status"}); code != exitMigrateNoTable && !strings.Contains(out, "instance") {
		t.Fatalf("%d\n%s", code, out)
	}
	downOne(t, dc)
	if code, out := run(t, dc, migrateOptions{Mode: "status"}); code != 0 || !strings.Contains(out, "migrate: instance -") {
		t.Fatalf("SQLite ⛔ 沒有實例識別，印 -：%d\n%s", code, out)
	}
	o := upOpts()
	o.ExpectInstance = "1|trading|16384|20261125000000000000"
	if code, out := run(t, dc, o); code != exitMigrateFingerprint || strings.Contains(out, "step version=") {
		t.Fatalf("SQLite 帶 -expect-instance 一律不符、⛔ 不套用：%d\n%s", code, out)
	}

	readInstanceFn = func(context.Context, *sqlx.DB, string) (string, error) { return "7|trading|5|x", nil }
	o.ExpectInstance = "7|trading_other|6|x"
	code, out := run(t, dc, o)
	if code != exitMigrateFingerprint || !strings.Contains(out, "instance mismatch") || !strings.Contains(out, "before instance 7|trading|5|x") || strings.Contains(out, "step version=") {
		t.Fatalf("實例不同（版本相同）：要結束碼 5、⛔ 沒有套用：%d\n%s", code, out)
	}
	o.ExpectInstance = "7|trading|5|x"
	if code, out := run(t, dc, o); code != exitMigrateOK || !strings.Contains(out, "applied 1") {
		t.Fatalf("實例相符：%d\n%s", code, out)
	}
}
