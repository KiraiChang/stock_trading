package main

// -migrate 模式（T-087；規格見 docs/plans/t087-migration-in-deploy.md 的 D1）：同一支 binary，只載入 database 設定、連 DB、做完就結束。
// ⛔ 不跑完整的設定契約（deploy.sh 第 5 步已經用新 image 跑過 -check-config）、⛔ 不連 Redis、⛔ 不啟動排程與 HTTP。
// 輸出只有版本號、計數、耗時與 SQLSTATE，⛔ 不帶 DSN 或錯誤原文。
//
//	./trading-backend -migrate=status
//	./trading-backend -migrate=up -run-id=20261201T140501 -expect-fingerprint=…
//
// 結束碼：status：0 成功、2 版本表不存在、1 其他；
// up：0 全部成功（或 pending 是空的）、3 有一支失敗（前面的已提交）、4 逾時、5 -expect-fingerprint 不符（⛔ 沒有套用）、1 其他。

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/trading/backend/internal/config"
	"github.com/trading/backend/internal/database"
	"github.com/trading/backend/internal/store"
)

var (
	migrateFlag           = flag.String("migrate", "", "status 或 up：只做 migration 的狀態查詢或套用後結束（T-087）")
	migrateRunIDFlag      = flag.String("run-id", "", "-migrate：本次執行的識別（application_name 是 trading-migrator-<run-id>）")
	migrateExpectFPFlag   = flag.String("expect-fingerprint", "", "-migrate=up：套用前的已套用集合指紋要等於它，否則⛔ 不套用（結束碼 5）")
	migrateExpectInstFlag = flag.String("expect-instance", "", "-migrate=up：這條連線的實例識別（system_identifier|database|oid|啟動時間）要等於它，否則⛔ 不套用（結束碼 5；只有 PostgreSQL）")
	migrateLockTimeout    = flag.Duration("migrate-lock-timeout", 5*time.Second, "-migrate=up：每條連線的 lock_timeout")
	migrateStmtTimeout    = flag.Duration("migrate-statement-timeout", 10*time.Minute, "-migrate=up：每條連線的 statement_timeout")
	migrateOverallTimeout = flag.Duration("migrate-timeout", 15*time.Minute, "-migrate=up：整體期限")
)

const (
	exitMigrateOK          = 0
	exitMigrateOther       = 1
	exitMigrateNoTable     = 2
	exitMigrateStepFailed  = 3
	exitMigrateTimeout     = 4
	exitMigrateFingerprint = 5
)

var runIDRE = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,40}$`)

type migrateOptions struct {
	Mode             string
	RunID            string
	ExpectFP         string
	ExpectInstance   string
	LockTimeout      time.Duration
	StatementTimeout time.Duration
	Overall          time.Duration
}

func runMigrateFromFlags() int {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "migrate: config load failed: %v\n", loadErrorViolations(err))
		return exitMigrateOther
	}
	return runMigrate(cfg.Database, migrateOptions{
		Mode: *migrateFlag, RunID: *migrateRunIDFlag, ExpectFP: *migrateExpectFPFlag, ExpectInstance: *migrateExpectInstFlag,
		LockTimeout: *migrateLockTimeout, StatementTimeout: *migrateStmtTimeout, Overall: *migrateOverallTimeout,
	}, os.Stdout, openMigrateDB)
}

// openMigrateDB：PostgreSQL 帶 application_name 與兩個 timeout；其他 driver 照一般的連線（只有測試會用到）。
func openMigrateDB(dc config.DatabaseConfig, o database.MigratorConnOptions) (*sqlx.DB, error) {
	switch dc.Driver {
	case "postgres", "postgresql":
		return database.OpenPostgresMigrator(dc.DSN, o)
	default:
		db, err := store.NewDB(dc)
		if err != nil {
			return nil, fmt.Errorf("connect database failed")
		}
		return db, nil
	}
}

type openFunc func(config.DatabaseConfig, database.MigratorConnOptions) (*sqlx.DB, error)

// migrateUpFn：測試替換成會失敗或卡住的版本（驗結束碼 3、4）；正式一律是 database.MigrateUp。
var migrateUpFn = database.MigrateUp

// readInstanceFn：實例識別（只有 PostgreSQL 有）。測試在 SQLite 上替換它。
var readInstanceFn = func(ctx context.Context, db *sqlx.DB, driver string) (string, error) {
	switch driver {
	case "postgres", "postgresql":
		return database.ReadInstance(ctx, db)
	default:
		return "", nil
	}
}

func runMigrate(dc config.DatabaseConfig, o migrateOptions, w io.Writer, open openFunc) int {
	out := func(format string, a ...any) { fmt.Fprintf(w, "migrate: "+format+"\n", a...) }
	if o.Mode != "status" && o.Mode != "up" {
		out("unknown mode (only status or up)")
		return exitMigrateOther
	}
	if dc.Driver == "" || dc.DSN == "" {
		out("database driver and dsn are required")
		return exitMigrateOther
	}
	if o.RunID != "" && !runIDRE.MatchString(o.RunID) {
		out("invalid run-id (only [A-Za-z0-9_.-], up to 40)")
		return exitMigrateOther
	}
	app := "trading-migrator"
	if o.RunID != "" {
		app += "-" + o.RunID
	}
	conn := database.MigratorConnOptions{ApplicationName: app}
	if o.Mode == "up" {
		conn.LockTimeout, conn.StatementTimeout = o.LockTimeout, o.StatementTimeout
	} else {
		conn.StatementTimeout = 30 * time.Second
	}
	out("mode=%s driver=%s application_name=%s", o.Mode, dc.Driver, app)
	db, err := open(dc, conn)
	if err != nil {
		out("%v", err) // 只有步驟名稱，⛔ 不帶 DSN
		return exitMigrateOther
	}
	defer db.Close()

	printStatus := func(prefix string, s *database.Status) {
		for _, l := range s.Lines() {
			out("%s%s", prefix, l)
		}
	}
	statusCtx := func() (context.Context, context.CancelFunc) {
		return context.WithTimeout(context.Background(), 60*time.Second)
	}

	ctx, cancel := statusCtx()
	before, err := database.ReadStatus(ctx, db, dc.Driver)
	cancel()
	noTable := errors.Is(err, database.ErrNoVersionTable)
	switch {
	case noTable:
		out("version table does not exist")
		if o.Mode == "status" {
			return exitMigrateNoTable
		}
		req, rerr := database.RequiredVersions(dc.Driver)
		if rerr != nil {
			out("read embedded migrations failed")
			return exitMigrateOther
		}
		before = &database.Status{Required: req, Pending: req, Fingerprint: database.Fingerprint(nil)}
	case err != nil:
		out("read status failed (%s)", errClass(err))
		return exitMigrateOther
	}
	ctx, cancel = statusCtx()
	instance, ierr := readInstanceFn(ctx, db, dc.Driver)
	cancel()
	if ierr != nil {
		out("read instance failed (%s)", errClass(ierr))
		return exitMigrateOther
	}
	if instance == "" {
		instance = "-" // 只有 PostgreSQL 有實例識別
	}
	if o.Mode == "status" {
		printStatus("", before)
		out("instance %s", instance)
		return exitMigrateOK
	}

	printStatus("before ", before)
	out("before instance %s", instance)
	if o.ExpectInstance != "" && (instance == "-" || o.ExpectInstance != instance) {
		out("instance mismatch: expected %s, got %s; nothing applied", o.ExpectInstance, instance)
		return exitMigrateFingerprint
	}
	if o.ExpectFP != "" && o.ExpectFP != before.Fingerprint {
		out("fingerprint mismatch: expected %s, got %s; nothing applied", o.ExpectFP, before.Fingerprint)
		return exitMigrateFingerprint
	}
	if len(before.Pending) == 0 {
		out("nothing to apply")
		return exitMigrateOK
	}
	out("limits lock_timeout=%s statement_timeout=%s overall=%s", o.LockTimeout, o.StatementTimeout, o.Overall)
	upCtx, upCancel := context.WithTimeout(context.Background(), o.Overall)
	n, upErr := migrateUpFn(upCtx, db, dc.Driver, func(r database.StepResult) {
		if r.End.IsZero() {
			out("step version=%d start=%s", r.Version, r.Start.UTC().Format(time.RFC3339Nano))
			return
		}
		result := "ok"
		if r.Err != nil {
			result = "failed (" + errClass(r.Err) + ")"
		}
		out("step version=%d end=%s duration=%s result=%s", r.Version, r.End.UTC().Format(time.RFC3339Nano), r.End.Sub(r.Start).Round(time.Millisecond), result)
	})
	timedOut := upCtx.Err() != nil
	upCancel()

	ctx, cancel = statusCtx()
	after, serr := database.ReadStatus(ctx, db, dc.Driver)
	cancel()
	if serr == nil {
		printStatus("after ", after)
	} else {
		out("read status after failed (%s)", errClass(serr))
	}

	switch {
	case upErr == nil:
		if serr != nil || len(after.Pending) > 0 {
			out("applied %d but pending is not empty", n)
			return exitMigrateOther
		}
		out("applied %d", n)
		return exitMigrateOK
	case errors.Is(upErr, database.ErrOutOfOrder):
		out("pending has versions below the applied max (goose does not apply out of order); nothing applied")
		return exitMigrateOther
	case timedOut || errors.Is(upErr, context.DeadlineExceeded):
		out("overall timeout after %d applied", n)
		return exitMigrateTimeout
	default:
		out("failed after %d applied", n)
		return exitMigrateStepFailed
	}
}

// errClass：只帶 PostgreSQL 的 SQLSTATE 或錯誤類別，⛔ 不帶錯誤原文（可能有資料值或 DSN）。
func errClass(err error) string {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		return "sqlstate " + pe.Code
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "deadline exceeded"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, database.ErrNoVersionTable):
		return "no version table"
	}
	return "error"
}

// checkSchema 是啟動時的唯讀檢查（DATABASE_AUTO_MIGRATE=false）：版本表不存在、查詢失敗、或 embed 的任何一支沒有套用 → 記 log 並回錯誤。
// log 只有版本號與錯誤類別（⛔ 不帶錯誤原文）。DB 多出較新的版本可以接受（記一行 Info）。
func checkSchema(db *sqlx.DB, driver string, log *zap.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	s, err := database.CheckApplied(ctx, db, driver)
	var pe *database.PendingError
	switch {
	case errors.As(err, &pe):
		log.Error("migrations not applied (run ./trading-backend -migrate=up, or set DATABASE_AUTO_MIGRATE=true in dev)",
			zap.String("pending", database.JoinVersions(pe.Pending)))
		return err
	case errors.Is(err, database.ErrNoVersionTable):
		log.Error("schema check failed: version table does not exist (run ./trading-backend -migrate=up, or set DATABASE_AUTO_MIGRATE=true in dev)")
		return err
	case err != nil:
		log.Error("schema check failed", zap.String("cause", errClass(err)))
		return err
	}
	log.Info("schema check passed (read only)", zap.Int("required", len(s.Required)), zap.String("fingerprint", s.Fingerprint))
	if len(s.Extra) > 0 {
		log.Info("database has migrations newer than this image (accepted)", zap.String("extra", database.JoinVersions(s.Extra)))
	}
	return nil
}
