package database

// migration 的狀態與逐支套用（T-087；規格見 docs/plans/t087-migration-in-deploy.md 的 D1、D3）。
//
//   - ReadStatus：**唯讀**讀 goose_db_version（⛔ 不用 goose.GetDBVersion——版本表不存在時它會建表），
//     跟 embed 的 migration 清單比對出 pending（含中間缺號）、extra 與已套用集合的指紋。
//   - CheckApplied：backend 啟動時的唯讀檢查（DATABASE_AUTO_MIGRATE=false）：embed 的每一支都已套用才通過。
//   - MigrateUp：逐支套用（goose UpByOne 迴圈），每支回報一次；任何一支失敗就停下。

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"
)

// ErrNoVersionTable：goose_db_version 不存在（從來沒有套用過 migration）。
var ErrNoVersionTable = errors.New("goose_db_version does not exist")

// PendingError：embed 裡有沒套用的版本（CheckApplied 用）。
type PendingError struct{ Pending []int64 }

func (e *PendingError) Error() string {
	return "migrations not applied: " + JoinVersions(e.Pending)
}

// Status 是 DB 的 migration 狀態（只有版本號與計數，⛔ 不帶 DSN 或錯誤原文）。
type Status struct {
	Required    []int64 // embed 的版本（升冪）
	Applied     []int64 // DB 已套用的版本（升冪；⛔ 不含 goose 的 0 號紀錄）
	Pending     []int64 // embed 有、DB 沒有（含中間缺號）
	Extra       []int64 // DB 有、embed 沒有
	Fingerprint string  // 已套用集合的 SHA-256（排序後的版本以逗號串起來）
}

// JoinVersions 把版本清單串成「1,2,3」（空的是「-」）。
func JoinVersions(vs []int64) string {
	if len(vs) == 0 {
		return "-"
	}
	parts := make([]string, len(vs))
	for i, v := range vs {
		parts[i] = strconv.FormatInt(v, 10)
	}
	return strings.Join(parts, ",")
}

func maxOf(vs []int64) int64 {
	if len(vs) == 0 {
		return 0
	}
	return vs[len(vs)-1]
}

// Lines 是給 -migrate=status 印的固定格式（deploy.sh 依前綴取值）。
func (s *Status) Lines() []string {
	return []string{
		fmt.Sprintf("required count=%d max=%d", len(s.Required), maxOf(s.Required)),
		fmt.Sprintf("applied count=%d max=%d", len(s.Applied), maxOf(s.Applied)),
		"pending " + JoinVersions(s.Pending),
		"extra " + JoinVersions(s.Extra),
		"fingerprint " + s.Fingerprint,
	}
}

type migrationSource struct {
	fs      embed.FS
	dialect string
	dir     string
}

func sourceFor(driver string) (migrationSource, error) {
	switch driver {
	case "sqlite", "":
		return migrationSource{sqliteFS, "sqlite3", "migrations/sqlite"}, nil
	case "mysql":
		return migrationSource{mysqlFS, "mysql", "migrations/mysql"}, nil
	case "postgres", "postgresql":
		return migrationSource{postgresFS, "postgres", "migrations/postgres"}, nil
	default:
		return migrationSource{}, fmt.Errorf("unsupported driver for migrations: %s", driver)
	}
}

// RequiredVersions 是 embed 目錄裡每一支 migration 的版本（跟 goose 用同一種檔名解析，⛔ 不手寫版本號）。
func RequiredVersions(driver string) ([]int64, error) {
	src, err := sourceFor(driver)
	if err != nil {
		return nil, err
	}
	return versionsIn(src.fs, src.dir)
}

func versionsIn(fsys fs.FS, dir string) ([]int64, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations: %w", err)
	}
	var out []int64
	seen := map[int64]bool{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		v, err := goose.NumericComponent(e.Name())
		if err != nil {
			return nil, fmt.Errorf("parse migration version %s: %w", e.Name(), err)
		}
		if seen[v] {
			return nil, fmt.Errorf("duplicate migration version %d", v)
		}
		seen[v] = true
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

// versionTableQuery：確認版本表存在的查詢（依 driver）。
func versionTableQuery(driver string) string {
	switch driver {
	case "postgres", "postgresql":
		return `SELECT CASE WHEN to_regclass('goose_db_version') IS NULL THEN 0 ELSE 1 END`
	case "mysql":
		return `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'goose_db_version'`
	default:
		return `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'goose_db_version'`
	}
}

type queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// readApplied 讀已套用集合：每個 version_id 取最後一列（id 最大）的 is_applied。
// goose v3.21.1 跑 SQL migration 的 down 時會刪掉那個版本的紀錄（撤回的版本⛔ 沒有列）；
// is_applied=false 的列是防禦性處理（舊版 goose 或手動寫入），同樣視為未套用。
func readApplied(ctx context.Context, q queryer, driver string) ([]int64, error) {
	var n int
	if err := q.QueryRowContext(ctx, versionTableQuery(driver)).Scan(&n); err != nil {
		return nil, fmt.Errorf("check version table: %w", err)
	}
	if n == 0 {
		return nil, ErrNoVersionTable
	}
	rows, err := q.QueryContext(ctx, `SELECT version_id, is_applied FROM goose_db_version ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("read version table: %w", err)
	}
	defer rows.Close()
	last := map[int64]bool{}
	for rows.Next() {
		var v int64
		var applied bool
		if err := rows.Scan(&v, &applied); err != nil {
			return nil, fmt.Errorf("scan version table: %w", err)
		}
		last[v] = applied
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read version table: %w", err)
	}
	var out []int64
	for v, ok := range last {
		if ok && v != 0 { // 0 是 goose 建表時寫的起始紀錄，⛔ 不是 migration
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

// Fingerprint 是已套用集合的 SHA-256（排序後的版本以逗號串起來；空集合也有固定值）。
func Fingerprint(applied []int64) string {
	sum := sha256.Sum256([]byte(JoinVersions(applied)))
	return hex.EncodeToString(sum[:])
}

func diff(required, applied []int64) (pending, extra []int64) {
	in := func(list []int64) map[int64]bool {
		m := make(map[int64]bool, len(list))
		for _, v := range list {
			m[v] = true
		}
		return m
	}
	a, r := in(applied), in(required)
	for _, v := range required {
		if !a[v] {
			pending = append(pending, v)
		}
	}
	for _, v := range applied {
		if !r[v] {
			extra = append(extra, v)
		}
	}
	return pending, extra
}

// ReadStatus 唯讀讀 migration 狀態。postgres 在 READ ONLY 交易裡讀；SQLite／MySQL 直接查（測試驗讀完之後表與內容都沒變）。
// 版本表不存在時回 ErrNoVersionTable（⛔ 不建表）。
func ReadStatus(ctx context.Context, db *sqlx.DB, driver string) (*Status, error) {
	required, err := RequiredVersions(driver)
	if err != nil {
		return nil, err
	}
	return readStatus(ctx, db, driver, required)
}

func readStatus(ctx context.Context, db *sqlx.DB, driver string, required []int64) (*Status, error) {
	var applied []int64
	var err error
	switch driver {
	case "postgres", "postgresql":
		tx, terr := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if terr != nil {
			return nil, fmt.Errorf("begin read only: %w", terr)
		}
		applied, err = readApplied(ctx, tx, driver)
		_ = tx.Rollback()
	default:
		applied, err = readApplied(ctx, db, driver)
	}
	if err != nil {
		return nil, err
	}
	s := &Status{Required: required, Applied: applied, Fingerprint: Fingerprint(applied)}
	s.Pending, s.Extra = diff(required, applied)
	return s, nil
}

// CheckApplied 是啟動時的唯讀檢查：embed 的每一支都已套用才回 nil；缺的回 *PendingError（只有版本號）。
// DB 多出 embed 沒有的版本可以接受（回傳的 Status.Extra 給呼叫端記 Info）。
func CheckApplied(ctx context.Context, db *sqlx.DB, driver string) (*Status, error) {
	s, err := ReadStatus(ctx, db, driver)
	if err != nil {
		return nil, err
	}
	if len(s.Pending) > 0 {
		return s, &PendingError{Pending: s.Pending}
	}
	return s, nil
}

// StepResult 是逐支套用的一支。
type StepResult struct {
	Version    int64
	Start, End time.Time
	Err        error
}

// ErrOutOfOrder：pending 裡有低於 DB 已套用最大版本的缺號；goose 預設⛔ 不套用亂序的 migration，所以⛔ 不嘗試。
var ErrOutOfOrder = errors.New("pending migrations below the applied max version")

// MigrateUp 逐支套用 pending（goose UpByOne 迴圈）。每支前後各回報一次（onStep 的 Err 為 nil 時是開始，
// End 有值時是結束）；任何一支失敗就停下、⛔ 不繼續下一支，回傳那個錯誤。ctx 的期限到了也停下（回 ctx.Err()）。
// 套用前 pending 低於已套用最大版本 → ErrOutOfOrder、⛔ 不套用任何一支。
func MigrateUp(ctx context.Context, db *sqlx.DB, driver string, onStep func(StepResult)) (applied int, err error) {
	src, err := sourceFor(driver)
	if err != nil {
		return 0, err
	}
	return migrateUp(ctx, db, src, onStep)
}

func migrateUp(ctx context.Context, db *sqlx.DB, src migrationSource, onStep func(StepResult)) (int, error) {
	required, err := versionsIn(src.fs, src.dir)
	if err != nil {
		return 0, err
	}
	s, err := readStatus(ctx, db, driverOf(src.dialect), required)
	switch {
	case errors.Is(err, ErrNoVersionTable):
		s = &Status{Required: required, Pending: required}
	case err != nil:
		return 0, err
	}
	if len(s.Pending) > 0 && len(s.Applied) > 0 && s.Pending[0] < maxOf(s.Applied) {
		return 0, ErrOutOfOrder
	}
	goose.SetBaseFS(src.fs)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect(src.dialect); err != nil {
		return 0, fmt.Errorf("goose set dialect: %w", err)
	}
	n := 0
	for _, v := range s.Pending {
		if err := ctx.Err(); err != nil {
			return n, err
		}
		start := time.Now()
		if onStep != nil {
			onStep(StepResult{Version: v, Start: start})
		}
		err := goose.UpByOneContext(ctx, db.DB, src.dir)
		end := time.Now()
		if errors.Is(err, goose.ErrNoNextVersion) {
			err = fmt.Errorf("version %d: goose found nothing to apply", v)
		}
		if onStep != nil {
			onStep(StepResult{Version: v, Start: start, End: end, Err: err})
		}
		if err != nil {
			if cerr := ctx.Err(); cerr != nil {
				return n, cerr
			}
			return n, err
		}
		n++
	}
	return n, nil
}

func driverOf(dialect string) string {
	switch dialect {
	case "postgres":
		return "postgres"
	case "mysql":
		return "mysql"
	default:
		return "sqlite"
	}
}
