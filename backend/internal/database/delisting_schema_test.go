package database_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"

	"github.com/trading/backend/internal/database"
	"github.com/trading/backend/internal/store"
)

// migration 076 的**跨引擎行為**驗證（計畫書 docs/todo.md T-071 的 #19／#19b／#19d／#20d）。
//
// ⚠️ **為什麼三個引擎都要跑，而不是只驗 DDL**：`ON DELETE RESTRICT` / `SET NULL`、
// nullable FK、整數型別相容性與 `CHECK` 的實際 enforcement **正是三者會分歧的地方**——
// migration 套得上去**不等於**執行期行為正確。這是 docs/issue.md I-054 記的
// 「mysql 只驗 DDL 不驗 repo 層 CRUD」那個盲區。
//
// ⛔ postgres／mysql 的函式名必須是 `TestPostgresMigrations…` / `TestMySQLMigrations…`
// 前綴：兩支腳本只編譯 `./internal/database` 並用 `-test.run` 篩名稱，
// 換個名字就會被靜默篩掉、永遠不會執行。
// SQLite 那份沒有這個限制，由 backend/scripts/test.sh 每次跑。

type delistingEngine struct {
	// placeholder 是該 engine 的參數佔位符號（`?` 或 `$n`），由 rebind 處理。
	name string
	db   *sqlx.DB
}

func skipUnlessDSN(t *testing.T, env, hint string) string {
	t.Helper()
	dsn := os.Getenv(env)
	if dsn == "" {
		t.Skipf("未設 %s，跳過（用 %s 執行）", env, hint)
	}
	return dsn
}

func migratedDB(t *testing.T, driver, sqlDriver, dsn string) *sqlx.DB {
	t.Helper()
	db, err := sqlx.Connect(sqlDriver, dsn)
	if err != nil {
		t.Fatalf("連不上 %s: %v", driver, err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.RunMigrations(context.Background(), db, driver, zap.NewNop()); err != nil {
		t.Fatalf("%s migration 失敗: %v", driver, err)
	}
	return db
}

// cleanDelisting 把本測試會碰到的資料清掉。
// ⛔ 順序不可反：stock_symbols 仍指向 delisting_events 時，先刪事件會被 RESTRICT 擋住。
func cleanDelisting(t *testing.T, db *sqlx.DB) {
	t.Helper()
	clean := func() {
		db.Exec(`UPDATE stock_symbols SET delisted_date = NULL, delisted_event_id = NULL`)
		db.Exec(`DELETE FROM delisting_source_snapshots`)
		db.Exec(`DELETE FROM delisting_events`)
		db.Exec(`DELETE FROM stock_symbols WHERE symbol LIKE 'T071%'`)
		db.Exec(`DELETE FROM job_runs WHERE job_name = 'delisting_reconcile_test'`)
	}
	clean()
	t.Cleanup(clean)
}

// seedEvent 建一筆事件並回傳 id。
func seedEvent(t *testing.T, db *sqlx.DB, symbol string) int64 {
	t.Helper()
	day := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	_, err := db.Exec(db.Rebind(`
		INSERT INTO delisting_events (source, symbol, company_name, delisted_date)
		VALUES (?, ?, ?, ?)`), "t071_test", symbol, "測試公司", day)
	if err != nil {
		t.Fatalf("建事件: %v", err)
	}
	var id int64
	if err := db.Get(&id, db.Rebind(
		`SELECT id FROM delisting_events WHERE source = ? AND symbol = ?`), "t071_test", symbol); err != nil {
		t.Fatalf("查事件 id: %v", err)
	}
	return id
}

func seedSymbol(t *testing.T, db *sqlx.DB, symbol string) {
	t.Helper()
	_, err := db.Exec(db.Rebind(`
		INSERT INTO stock_symbols (symbol, name, market, security_type, is_listed)
		VALUES (?, ?, ?, ?, ?)`), symbol, "測試股", "上市", "股票", false)
	if err != nil {
		t.Fatalf("建主檔列: %v", err)
	}
}

// ── #19d：delisted_event_id 的 FK 是 RESTRICT ──────────────────────────────
//
// ⛔ 不可照抄鄰近 job_run_id 的 SET NULL：delisted_event_id 是**所有權標記**，
// 被清成 NULL 會讓 job-owned 的投影被誤認成人工值，從此不參與重算、永久無法收斂。
func assertDelistedEventFKRestricts(t *testing.T, db *sqlx.DB) {
	t.Helper()
	cleanDelisting(t, db)

	eventID := seedEvent(t, db, "T071A")
	seedSymbol(t, db, "T071A")
	day := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if _, err := db.Exec(db.Rebind(`
		UPDATE stock_symbols SET delisted_date = ?, delisted_event_id = ? WHERE symbol = ?`),
		day, eventID, "T071A"); err != nil {
		t.Fatalf("投影: %v", err)
	}

	// 被引用的事件不得被刪除。
	_, err := db.Exec(db.Rebind(`DELETE FROM delisting_events WHERE id = ?`), eventID)
	if err == nil {
		t.Error("刪除被投影引用的事件應該被 RESTRICT 擋住，但成功了")
	}

	// 投影資料必須完全不變。
	var gotID sql.NullInt64
	if err := db.Get(&gotID, db.Rebind(
		`SELECT delisted_event_id FROM stock_symbols WHERE symbol = ?`), "T071A"); err != nil {
		t.Fatalf("回查: %v", err)
	}
	if !gotID.Valid || gotID.Int64 != eventID {
		t.Errorf("刪除被拒後投影資料應不變，delisted_event_id 得到 %+v，期望 %d", gotID, eventID)
	}
}

// ── #20d：兩欄不變量的 CHECK ───────────────────────────────────────────────
//
// ⛔ **必須先建立一筆有效的 delisting_events.id 再拿它去寫**——否則寫入可能只是
// 因為 FK 找不到目標而失敗，`CHECK` 根本沒被驗到。
func assertDelistedPairCheck(t *testing.T, db *sqlx.DB) {
	t.Helper()
	cleanDelisting(t, db)

	eventID := seedEvent(t, db, "T071B") // ← 有效的 FK 目標，確保失敗來自 CHECK 而非 FK
	seedSymbol(t, db, "T071B")

	// 半套狀態：指向事件卻沒有日期。撤銷或重新上市時只清一欄就會產生它。
	_, err := db.Exec(db.Rebind(`
		UPDATE stock_symbols SET delisted_date = NULL, delisted_event_id = ? WHERE symbol = ?`),
		eventID, "T071B")
	if err == nil {
		t.Error("delisted_date IS NULL + delisted_event_id 非 NULL 應被 CHECK 拒絕，但成功了")
	} else if strings.Contains(strings.ToLower(err.Error()), "foreign key") {
		t.Errorf("失敗原因應是 CHECK 而不是 FK（fixture 沒建好）：%v", err)
	}

	// 該列必須保持不變。
	var date sql.NullTime
	var id sql.NullInt64
	if err := db.Get(&struct {
		D *sql.NullTime  `db:"delisted_date"`
		I *sql.NullInt64 `db:"delisted_event_id"`
	}{&date, &id}, db.Rebind(
		`SELECT delisted_date, delisted_event_id FROM stock_symbols WHERE symbol = ?`), "T071B"); err != nil {
		// sqlx 的 struct scan 對指標欄位挑剔，退回逐欄查。
		if err2 := db.Get(&date, db.Rebind(`SELECT delisted_date FROM stock_symbols WHERE symbol = ?`), "T071B"); err2 != nil {
			t.Fatalf("回查 delisted_date: %v", err2)
		}
		if err2 := db.Get(&id, db.Rebind(`SELECT delisted_event_id FROM stock_symbols WHERE symbol = ?`), "T071B"); err2 != nil {
			t.Fatalf("回查 delisted_event_id: %v", err2)
		}
	}
	if date.Valid || id.Valid {
		t.Errorf("CHECK 拒絕後該列應保持不變，得到 date=%+v id=%+v", date, id)
	}
}

// ── #19／#19b：job_runs 的 30 天清理不得刪到快照 ────────────────────────────
//
// job_runs 只保留 30 天，而快照是縮水防護的基準、**必須長期保存**。
// 一般 FK 會讓 DeleteBefore 失敗；CASCADE 更糟——會把基準一起刪掉。
func assertSnapshotSurvivesJobRunCleanup(t *testing.T, db *sqlx.DB) {
	t.Helper()
	cleanDelisting(t, db)

	ctx := context.Background()
	jobRuns := store.NewJobRunRepo(db)
	runID, err := jobRuns.Start(ctx, "delisting_reconcile_test")
	if err != nil {
		t.Fatalf("startRun: %v", err)
	}
	if err := jobRuns.Finish(ctx, runID, "success", 1, 0, ""); err != nil {
		t.Fatalf("finishRun: %v", err)
	}

	if _, err := db.Exec(db.Rebind(`
		INSERT INTO delisting_source_snapshots (source, job_run_id, row_count, content_hash)
		VALUES (?, ?, ?, ?)`), "t071_test", runID, 265, "deadbeef"); err != nil {
		t.Fatalf("寫快照: %v", err)
	}

	// #19：清理涵蓋期不得失敗、不得刪到快照。
	if _, err := jobRuns.DeleteBefore(ctx, time.Now().Add(24*time.Hour)); err != nil {
		t.Fatalf("DeleteBefore 不得因 FK 失敗: %v", err)
	}

	var n int
	if err := db.Get(&n, db.Rebind(
		`SELECT COUNT(*) FROM delisting_source_snapshots WHERE source = ?`), "t071_test"); err != nil {
		t.Fatalf("查快照: %v", err)
	}
	if n != 1 {
		t.Fatalf("快照必須長期保存，清理後剩 %d 筆（期望 1）", n)
	}

	// #19b：被清掉的 job_run 對應的 job_run_id 變 NULL，schema／repo 要能接受。
	var gotRunID sql.NullInt64
	if err := db.Get(&gotRunID, db.Rebind(
		`SELECT job_run_id FROM delisting_source_snapshots WHERE source = ?`), "t071_test"); err != nil {
		t.Fatalf("查 job_run_id: %v", err)
	}
	if gotRunID.Valid {
		t.Errorf("job_run 被清掉後 job_run_id 應為 NULL，得到 %d", gotRunID.Int64)
	}
}

// snapshotOrderingByID 驗「上一筆 accepted 一律以 id DESC 取得」。
//
// ⛔ 不可只靠 fetched_at：兩次連續執行可能落在同一個時間戳（秒級精度），
// 排序就變成未定義，縮水防護會拿到不確定的基準。
func assertSnapshotOrderingUsesID(t *testing.T, db *sqlx.DB) {
	t.Helper()
	cleanDelisting(t, db)

	// 刻意用**同一個** fetched_at 寫兩筆——只靠時間戳的話這裡就分不出先後。
	same := time.Date(2026, 9, 5, 7, 0, 0, 0, time.UTC)
	for _, rc := range []int{265, 200} {
		if _, err := db.Exec(db.Rebind(`
			INSERT INTO delisting_source_snapshots (source, fetched_at, row_count, content_hash, accepted)
			VALUES (?, ?, ?, ?, ?)`), "t071_test", same, rc, "h", true); err != nil {
			t.Fatalf("寫快照: %v", err)
		}
	}

	var rowCount int
	if err := db.Get(&rowCount, db.Rebind(`
		SELECT row_count FROM delisting_source_snapshots
		WHERE source = ? AND accepted = ?
		ORDER BY id DESC LIMIT 1`), "t071_test", true); err != nil {
		t.Fatalf("查最新快照: %v", err)
	}
	if rowCount != 200 {
		t.Errorf("最新 accepted 快照的 row_count 應為 200（後寫入那筆），得到 %d", rowCount)
	}
}

func runDelistingSchemaSuite(t *testing.T, db *sqlx.DB) {
	t.Run("FK-restrict", func(t *testing.T) { assertDelistedEventFKRestricts(t, db) })
	t.Run("pair-check", func(t *testing.T) { assertDelistedPairCheck(t, db) })
	t.Run("snapshot-survives-cleanup", func(t *testing.T) { assertSnapshotSurvivesJobRunCleanup(t, db) })
	t.Run("snapshot-ordering", func(t *testing.T) { assertSnapshotOrderingUsesID(t, db) })
}

func TestPostgresMigrationsDelistingSchema(t *testing.T) {
	dsn := skipUnlessDSN(t, "POSTGRES_MIGRATION_DSN", "scripts/test-postgres-migrations.sh")
	runDelistingSchemaSuite(t, migratedDB(t, "postgres", "pgx", dsn))
}

func TestMySQLMigrationsDelistingSchema(t *testing.T) {
	dsn := skipUnlessDSN(t, "MYSQL_MIGRATION_DSN", "scripts/test-mysql-migrations.sh")
	runDelistingSchemaSuite(t, migratedDB(t, "mysql", "mysql", dsn))
}

// SQLite 版由 backend/scripts/test.sh 每次跑——不需要 container，用暫存檔即可。
//
// ⚠️ 這裡刻意走 store.NewSQLite 而不是自己 sqlx.Connect：**FK 是 connection-local**，
// 繞過那條初始化路徑就拿不到 `_pragma=foreign_keys(1)`，
// 於是 RESTRICT／SET NULL 全部靜默失效、這幾條測試會變成什麼都沒驗到。
func TestSQLiteDelistingSchema(t *testing.T) {
	dir := t.TempDir()
	db, err := store.NewSQLite(dir + "/delisting.db")
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.RunMigrations(context.Background(), db, "sqlite", zap.NewNop()); err != nil {
		t.Fatalf("sqlite migration 失敗: %v", err)
	}

	// 前置：確認 FK 真的是開的，否則下面每一條都會假通過。
	var fk int
	if err := db.Get(&fk, `PRAGMA foreign_keys`); err != nil {
		t.Fatalf("查 foreign_keys: %v", err)
	}
	if fk != 1 {
		t.Fatalf("foreign_keys 必須是 1，否則 FK 契約全部靜默失效（得到 %d）", fk)
	}

	runDelistingSchemaSuite(t, db)
}

var _ = errors.Is
