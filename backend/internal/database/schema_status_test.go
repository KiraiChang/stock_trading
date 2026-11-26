package database

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"
	"go.uber.org/zap"
	_ "modernc.org/sqlite"
)

// T-087 的唯讀檢查與逐支套用（規格見 docs/plans/t087-migration-in-deploy.md 的 D1、D3 與第六節）。
// SQLite 每次 backend/scripts/test.sh 都跑；PostgreSQL 的唯讀交易、lock_timeout、整體期限在 schema_status_postgres_test.go。

//go:embed testdata/t087/ok/*.sql testdata/t087/fail/*.sql
var t087FS embed.FS

func sqliteTempDB(t *testing.T) *sqlx.DB {
	t.Helper()
	tmp, err := os.CreateTemp("", "t087-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	t.Cleanup(func() { os.Remove(tmp.Name()) })
	db, err := sqlx.Connect("sqlite", tmp.Name())
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return db
}

func migratedSQLite(t *testing.T) *sqlx.DB {
	t.Helper()
	db := sqliteTempDB(t)
	if err := RunMigrations(context.Background(), db, "sqlite", zap.NewNop()); err != nil {
		t.Fatal(err)
	}
	return db
}

// versionTableSnapshot：版本表的完整內容與結構（唯讀檢查前後要相同）。
func versionTableSnapshot(t *testing.T, db *sqlx.DB) string {
	t.Helper()
	var rows []string
	r, err := db.Query(`SELECT id, version_id, is_applied FROM goose_db_version ORDER BY id`)
	if err != nil {
		return "no table: " + err.Error()
	}
	defer r.Close()
	for r.Next() {
		var id, v int64
		var ok bool
		if err := r.Scan(&id, &v, &ok); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, fmt.Sprintf("%d:%d:%v", id, v, ok))
	}
	var schema string
	_ = db.Get(&schema, `SELECT sql FROM sqlite_master WHERE name = 'goose_db_version'`)
	return schema + "|" + strings.Join(rows, ",")
}

func lastVersion(t *testing.T) int64 {
	t.Helper()
	req, err := RequiredVersions("sqlite")
	if err != nil || len(req) == 0 {
		t.Fatalf("embed 清單：%v %v", req, err)
	}
	return req[len(req)-1]
}

func TestRequiredVersionsFromEmbed(t *testing.T) {
	for _, d := range []string{"sqlite", "postgres", "mysql"} {
		req, err := RequiredVersions(d)
		// sqlite／mysql 有幾個版本只在 postgres（例如 046、063～065），所以只要求非空、從 1 開始；postgres 至少 80 支
		if err != nil || len(req) == 0 || req[0] != 1 || (d == "postgres" && len(req) < 80) {
			t.Fatalf("%s：%v %v", d, req, err)
		}
		for i := 1; i < len(req); i++ {
			if req[i] <= req[i-1] {
				t.Fatalf("%s 要升冪：%v", d, req)
			}
		}
	}
}

func TestReadStatusNoVersionTableDoesNotCreate(t *testing.T) {
	db := sqliteTempDB(t)
	if _, err := ReadStatus(context.Background(), db, "sqlite"); !errors.Is(err, ErrNoVersionTable) {
		t.Fatalf("版本表不存在要回 ErrNoVersionTable：%v", err)
	}
	if _, err := CheckApplied(context.Background(), db, "sqlite"); !errors.Is(err, ErrNoVersionTable) {
		t.Fatalf("CheckApplied 也要拒絕：%v", err)
	}
	var n int
	if err := db.Get(&n, `SELECT COUNT(*) FROM sqlite_master WHERE name = 'goose_db_version'`); err != nil || n != 0 {
		t.Fatalf("唯讀檢查⛔ 不能建表：n=%d err=%v", n, err)
	}
}

func TestCheckAppliedAllPresentAndReadOnly(t *testing.T) {
	db := migratedSQLite(t)
	before := versionTableSnapshot(t, db)
	s, err := CheckApplied(context.Background(), db, "sqlite")
	if err != nil {
		t.Fatalf("全部都在要通過：%v", err)
	}
	if len(s.Pending) != 0 || len(s.Extra) != 0 || len(s.Applied) != len(s.Required) || s.Fingerprint != Fingerprint(s.Required) {
		t.Fatalf("狀態不對：%+v", s)
	}
	if after := versionTableSnapshot(t, db); after != before {
		t.Fatalf("檢查前後版本表要相同：\n%s\n%s", before, after)
	}
	lines := strings.Join(s.Lines(), "\n")
	for _, want := range []string{"required count=", "pending -", "extra -", "fingerprint " + s.Fingerprint} {
		if !strings.Contains(lines, want) {
			t.Fatalf("輸出缺「%s」：\n%s", want, lines)
		}
	}
}

func TestCheckAppliedMissingMiddleVersion(t *testing.T) {
	db := migratedSQLite(t)
	// 刪掉中間一支（079）的紀錄：最大版本照舊是 080，只看最大版本的檢查會放行
	if _, err := db.Exec(`DELETE FROM goose_db_version WHERE version_id = 79`); err != nil {
		t.Fatal(err)
	}
	s, err := CheckApplied(context.Background(), db, "sqlite")
	var pe *PendingError
	if !errors.As(err, &pe) || JoinVersions(pe.Pending) != "79" || JoinVersions(s.Pending) != "79" {
		t.Fatalf("缺中間一支要拒絕並列出 79：%v %+v", err, s)
	}
	if !strings.Contains(err.Error(), "79") {
		t.Fatalf("錯誤要列出版本號：%v", err)
	}
}

func TestCheckAppliedAcceptsExtraVersion(t *testing.T) {
	db := migratedSQLite(t)
	if _, err := db.Exec(`INSERT INTO goose_db_version (version_id, is_applied) VALUES (999, 1)`); err != nil {
		t.Fatal(err)
	}
	s, err := CheckApplied(context.Background(), db, "sqlite")
	if err != nil || JoinVersions(s.Extra) != "999" {
		t.Fatalf("多出 embed 沒有的版本要通過、列在 Extra：%v %+v", err, s)
	}
}

func TestCheckAppliedAfterRealGooseDown(t *testing.T) {
	db := migratedSQLite(t)
	goose.SetBaseFS(sqliteFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err := goose.DownContext(context.Background(), db.DB, "migrations/sqlite"); err != nil {
		t.Fatalf("goose down：%v", err)
	}
	last := lastVersion(t)
	var n int
	if err := db.Get(&n, `SELECT COUNT(*) FROM goose_db_version WHERE version_id = ?`, last); err != nil || n != 0 {
		t.Fatalf("goose v3.21.1 的 down 會刪掉那個版本的紀錄：n=%d err=%v", n, err)
	}
	_, err := CheckApplied(context.Background(), db, "sqlite")
	var pe *PendingError
	if !errors.As(err, &pe) || JoinVersions(pe.Pending) != fmt.Sprint(last) {
		t.Fatalf("實際跑一次 down 之後要拒絕並列出 %d：%v", last, err)
	}
}

func TestCheckAppliedLastRowNotApplied(t *testing.T) {
	db := migratedSQLite(t)
	// 防禦性：某版本最後一列是 is_applied=false（舊版 goose 或手動寫入）→ 視為未套用
	if _, err := db.Exec(`INSERT INTO goose_db_version (version_id, is_applied) VALUES (5, 0)`); err != nil {
		t.Fatal(err)
	}
	s, err := ReadStatus(context.Background(), db, "sqlite")
	if err != nil || JoinVersions(s.Pending) != "5" {
		t.Fatalf("最後一列 is_applied=false 要視為未套用：%v %+v", err, s)
	}
	// 之後再補一列 true：最後一列是 true → 已套用
	if _, err := db.Exec(`INSERT INTO goose_db_version (version_id, is_applied) VALUES (5, 1)`); err != nil {
		t.Fatal(err)
	}
	if s, err := ReadStatus(context.Background(), db, "sqlite"); err != nil || len(s.Pending) != 0 {
		t.Fatalf("最後一列是 true 要算已套用：%v %+v", err, s)
	}
}

func TestFingerprintDependsOnSet(t *testing.T) {
	if Fingerprint([]int64{1, 2, 3}) == Fingerprint([]int64{1, 3}) || Fingerprint(nil) == "" || len(Fingerprint(nil)) != 64 {
		t.Fatal("指紋要依已套用集合而不同、固定長度")
	}
}

func srcOf(dir string) migrationSource { return migrationSource{t087FS, "sqlite3", dir} }

func TestMigrateUpStopsAtFailingStep(t *testing.T) {
	db := sqliteTempDB(t)
	var steps []StepResult
	n, err := migrateUp(context.Background(), db, srcOf("testdata/t087/fail"), func(r StepResult) { steps = append(steps, r) })
	if err == nil || n != 1 {
		t.Fatalf("第二支失敗：要回錯誤、已套用 1 支：n=%d err=%v", n, err)
	}
	// 開始／結束各一次：1 開始、1 結束（ok）、2 開始、2 結束（失敗）；⛔ 沒有第三支
	if len(steps) != 4 || steps[1].Err != nil || steps[3].Err == nil || steps[3].Version != 2 {
		t.Fatalf("逐支回報不對：%+v", steps)
	}
	s, err := readStatus(context.Background(), db, "sqlite", []int64{1, 2, 3})
	if err != nil || JoinVersions(s.Applied) != "1" || JoinVersions(s.Pending) != "2,3" {
		t.Fatalf("第一支已提交、第二支與第三支沒有：%v %+v", err, s)
	}
	var tables int
	_ = db.Get(&tables, `SELECT COUNT(*) FROM sqlite_master WHERE name IN ('t087_b', 't087_c')`)
	if tables != 0 {
		t.Fatalf("失敗那支的交易要回滾、第三支⛔ 不執行：%d", tables)
	}
}

func TestMigrateUpAllAndNothingPending(t *testing.T) {
	db := sqliteTempDB(t)
	n, err := migrateUp(context.Background(), db, srcOf("testdata/t087/ok"), nil)
	if err != nil || n != 2 {
		t.Fatalf("全部成功：n=%d err=%v", n, err)
	}
	if n, err := migrateUp(context.Background(), db, srcOf("testdata/t087/ok"), nil); err != nil || n != 0 {
		t.Fatalf("pending 空：n=%d err=%v", n, err)
	}
}

func TestMigrateUpRefusesOutOfOrder(t *testing.T) {
	db := sqliteTempDB(t)
	if _, err := migrateUp(context.Background(), db, srcOf("testdata/t087/ok"), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM goose_db_version WHERE version_id = 1`); err != nil {
		t.Fatal(err)
	}
	before := versionTableSnapshot(t, db)
	if n, err := migrateUp(context.Background(), db, srcOf("testdata/t087/ok"), nil); !errors.Is(err, ErrOutOfOrder) || n != 0 {
		t.Fatalf("pending 低於已套用最大版本：要 ErrOutOfOrder、⛔ 不套用：n=%d err=%v", n, err)
	}
	if after := versionTableSnapshot(t, db); after != before {
		t.Fatal("⛔ 不能動到版本表")
	}
}

func TestMigrateUpHonorsContext(t *testing.T) {
	db := sqliteTempDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if n, err := migrateUp(ctx, db, srcOf("testdata/t087/ok"), nil); n != 0 || !errors.Is(err, context.Canceled) {
		t.Fatalf("期限已到：⛔ 不套用、回 ctx 的錯誤：n=%d err=%v", n, err)
	}
}

