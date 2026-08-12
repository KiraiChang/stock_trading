package database

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"
	"go.uber.org/zap"
)

// mysql migration 的實跑驗證。三份 migration（mysql / postgres / sqlite）裡，
// sqlite 靠 internal/store 的測試、postgres 靠 dev compose 的 backend 啟動，
// 只有 mysql 沒有任何執行路徑（docs/issue.md I-054）。本測試補上那條路徑。
//
// **需要一個真的 MySQL**，所以用環境變數 gate 住：沒設 MYSQL_MIGRATION_DSN 就 skip，
// 一般的 backend/scripts/test.sh ./... 行為完全不受影響。
// 跑法見 scripts/test-mysql-migrations.sh（它會處理 compose 與記憶體錯開）。
//
// 刻意放在 package database 內部而不是外部測試套件：migration 是用 //go:embed 打包的，
// 只有這裡拿得到 mysqlFS，才能用 backend 真正會執行的那份檔案跑 down 驗回滾。
// 從磁碟讀 migrations/ 的 goose CLI 驗的是另一份東西，沒有意義。

const mysqlMigrationDir = "migrations/mysql"

func mysqlTestDB(t *testing.T) *sqlx.DB {
	t.Helper()

	dsn := os.Getenv("MYSQL_MIGRATION_DSN")
	if dsn == "" {
		t.Skip("未設 MYSQL_MIGRATION_DSN，跳過 mysql migration 實跑驗證（用 scripts/test-mysql-migrations.sh 執行）")
	}

	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		t.Fatalf("連不上 MySQL: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// migrationVersions 從 embed 的檔名取出所有版本號，測試的期望值因此跟著檔案走，
// 不用每加一個 migration 就回來改硬編碼的數字。
func migrationVersions(t *testing.T) []int64 {
	t.Helper()

	entries, err := fs.ReadDir(mysqlFS, mysqlMigrationDir)
	if err != nil {
		t.Fatalf("讀不到 embed 的 migration 目錄: %v", err)
	}

	versions := make([]int64, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}
		prefix, _, ok := strings.Cut(name, "_")
		if !ok {
			t.Fatalf("migration 檔名沒有版本號前綴: %s", name)
		}
		v, err := strconv.ParseInt(prefix, 10, 64)
		if err != nil {
			t.Fatalf("migration 檔名的版本號解析失敗: %s (%v)", name, err)
		}
		versions = append(versions, v)
	}
	if len(versions) == 0 {
		t.Fatal("embed 的 mysql migration 目錄是空的")
	}
	return versions
}

// TestMySQLMigrationsUpAndDown 跑完整的 up，驗最終 schema，再 down 到 0 驗回滾。
func TestMySQLMigrationsUpAndDown(t *testing.T) {
	db := mysqlTestDB(t)
	ctx := context.Background()

	// 前置：要求一個乾淨的 DB。migration 的價值就在「從空的跑到最新」，
	// 殘留的表會讓 CREATE TABLE IF NOT EXISTS 靜靜跳過而驗不到東西。
	if n := countTables(t, db); n != 0 {
		t.Fatalf("DB 不是空的（有 %d 張表）。這支測試要求乾淨的 DB，請用 scripts/test-mysql-migrations.sh 執行（它會 down -v）", n)
	}

	versions := migrationVersions(t)
	want := versions[len(versions)-1]

	// 走 cmd/server/main.go 完全相同的進入點，而不是自己拼 goose 呼叫。
	if err := RunMigrations(ctx, db, "mysql", zap.NewNop()); err != nil {
		t.Fatalf("mysql migration up 失敗: %v", err)
	}

	got, err := goose.GetDBVersion(db.DB)
	if err != nil {
		t.Fatalf("讀不到 goose 版本: %v", err)
	}
	if got != want {
		t.Fatalf("goose 版本 = %d, want %d（最後一個 migration 檔）", got, want)
	}

	// mysql 目錄刻意缺 046（sr_zone 的 jsonb 欄位是 postgres 專屬），所以這裡驗的是
	// 「套用數 ＝ 檔案數」而不是「版本號連號」。連號斷言會把合理差異誤判成缺漏。
	var applied int
	if err := db.GetContext(ctx, &applied,
		`SELECT COUNT(*) FROM goose_db_version WHERE version_id > 0 AND is_applied = 1`); err != nil {
		t.Fatalf("查 goose_db_version 失敗: %v", err)
	}
	if applied != len(versions) {
		t.Fatalf("已套用 %d 筆，但 embed 有 %d 個 migration 檔", applied, len(versions))
	}

	assertMarketBackfillJobsSchema(t, db)
	assertCandlePositivePriceCheck(t, db)

	// 回滾驗證：一路 down 到 0，驗證每一筆 migration 的 Down 都真的可逆。
	//
	// 這條路徑曾經在 016 中斷——017／018 是破壞性重建（DROP 再 CREATE），但它們的 Down
	// 只有 DROP、沒有還原前一版結構，所以越過它們之後 016 的
	// `ALTER TABLE stock_sr_zones DROP COLUMN confidence` 會找不到表。
	// 兩筆的 Down 已補上結構重建，這裡才改回滾到底。
	goose.SetBaseFS(mysqlFS)
	if err := goose.SetDialect("mysql"); err != nil {
		t.Fatalf("goose set dialect: %v", err)
	}

	// 分段回滾並在中途檢查結構，理由同 migrate_sqlite_test.go：「一路 down 到 0 沒報錯」
	// 驗不到 017／018 的 Down 重建得對不對（017 的 Down 一執行就把 018 重建的表砍掉）。
	// mysql 的 Down 是獨立的方言檔案，要有自己的覆蓋。
	if err := goose.DownToContext(ctx, db.DB, mysqlMigrationDir, 17); err != nil {
		t.Fatalf("mysql migration down 到 17 失敗: %v", err)
	}
	assertMySQLColumns(t, db, "stock_sr_zones",
		"net_score", "net_score_label", "confidence_level", "reject_count", "break_count",
		"zone_momentum", "trading_score", "trading_recommendation")
	assertMySQLColumns(t, db, "stock_sr_zone_analyses", "overall_trend", "overall_volatility")

	if err := goose.DownToContext(ctx, db.DB, mysqlMigrationDir, 16); err != nil {
		t.Fatalf("mysql migration down 到 16 失敗: %v", err)
	}
	assertMySQLColumns(t, db, "stock_sr_zones",
		"rejection_count", "breakout_count", "avg_return_after_touch", "trend_strength",
		"confidence", "expected_value", "risk_reward_ratio")

	if err := goose.DownToContext(ctx, db.DB, mysqlMigrationDir, 0); err != nil {
		t.Fatalf("mysql migration down 到 0 失敗: %v", err)
	}

	if v, err := goose.GetDBVersion(db.DB); err != nil {
		t.Fatalf("讀不到 goose 版本: %v", err)
	} else if v != 0 {
		t.Fatalf("down 之後版本 = %d, want 0", v)
	}

	// goose_db_version 是 goose 自己的版本表，它不會刪掉自己；除此之外應該一張表都不剩。
	if n := countTablesExcludingGoose(t, db); n != 0 {
		t.Fatalf("down 到 0 之後還剩 %d 張表，代表有 migration 的 Down 沒清乾淨: %v",
			n, tableNames(t, db))
	}
}

// TestMySQLMigrationsPreserveCashDividendVolumeFactor 鎖住 062 回填條件的正確性，
// 對應 migrate_postgres_test.go 的 TestPostgresMigrationsPreserveCashDividendVolumeFactor。
//
// 062 有一段回填「Phase 1 的分割事件 volume_factor = factor」。條件若只寫
// `volume_factor = 1 AND factor <> 1`，會把現金股利也一起改掉——**1 正是現金股利的正確值**，
// 等於把現金股利算進成交量調整。所以條件必須排除 DIVIDEND%。
//
// 平常跑不到，因為 migration 只套用一次；但 **down → up 會重現**：欄位被 drop 之後
// 全部回到預設 1，再 up 時每一筆 factor <> 1 的事件都符合條件。
// TestMySQLMigrationsUpAndDown 是「up 一次、down 到 0」，驗不到這條路徑。
func TestMySQLMigrationsPreserveCashDividendVolumeFactor(t *testing.T) {
	db := mysqlTestDB(t)
	ctx := context.Background()

	// 前一支測試結束在版本 0，只剩 goose_db_version。
	if n := countTablesExcludingGoose(t, db); n != 0 {
		t.Fatalf("DB 不是空的（有 %d 張表）: %v", n, tableNames(t, db))
	}

	goose.SetBaseFS(mysqlFS)
	if err := goose.SetDialect("mysql"); err != nil {
		t.Fatalf("goose set dialect: %v", err)
	}
	if err := goose.UpToContext(ctx, db.DB, mysqlMigrationDir, 62); err != nil {
		t.Fatalf("up 到 62 失敗: %v", err)
	}

	// 一筆現金股利（volume_factor 正確值為 1）與一筆分割（volume_factor = factor）。
	if _, err := db.Exec(`
		INSERT INTO corporate_actions
			(symbol, event_date, action_type, before_price, after_price, factor, volume_factor, source)
		VALUES ('2317','2026-07-02','DIVIDEND_CASH',248,242.2,0.976613,1,'test'),
		       ('0050','2025-06-18','分割',188.65,47.16,0.249987,0.249987,'test')`); err != nil {
		t.Fatalf("寫入測試事件失敗: %v", err)
	}

	// down → up：欄位被 drop 再重建，回填邏輯會重跑。
	if err := goose.DownToContext(ctx, db.DB, mysqlMigrationDir, 61); err != nil {
		t.Fatalf("down 到 61 失敗: %v", err)
	}
	if err := goose.UpToContext(ctx, db.DB, mysqlMigrationDir, 62); err != nil {
		t.Fatalf("重新 up 到 62 失敗: %v", err)
	}

	var cashVF, splitVF float64
	if err := db.Get(&cashVF,
		`SELECT volume_factor FROM corporate_actions WHERE symbol='2317'`); err != nil {
		t.Fatalf("讀現金股利失敗: %v", err)
	}
	if err := db.Get(&splitVF,
		`SELECT volume_factor FROM corporate_actions WHERE symbol='0050'`); err != nil {
		t.Fatalf("讀分割失敗: %v", err)
	}

	if cashVF != 1 {
		t.Errorf("現金股利的 volume_factor = %v, want 1——回填把除權息也改掉了，"+
			"等於把現金股利算進成交量調整", cashVF)
	}
	if diff := splitVF - 0.249987; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("分割的 volume_factor = %v, want 0.249987——回填沒有涵蓋純股數事件", splitVF)
	}

	// volume_factor 的 CHECK 要真的生效（檢查 6 用 LN(volume_factor)，0 會變 -inf）。
	// MySQL 的 CHECK 在 8.0.16 之前只解析不強制，只有實際寫一列違規資料進去才知道有沒有在做事。
	if _, err := db.Exec(`
		INSERT INTO corporate_actions
			(symbol, event_date, action_type, before_price, after_price, factor, volume_factor, source)
		VALUES ('9999','2026-01-01','分割',100,50,0.5,0,'test')`); err == nil {
		t.Error("volume_factor = 0 竟然寫得進去——CHECK 沒生效")
	}

	// 收乾淨，讓之後的測試從空 DB 開始。
	if err := goose.DownToContext(ctx, db.DB, mysqlMigrationDir, 0); err != nil {
		t.Fatalf("收尾 down 到 0 失敗: %v", err)
	}
}

// assertMarketBackfillJobsSchema 抽驗最近新增、且寫法只有真的跑過才知道對不對的欄位。
// failures 用的是 TEXT 的括號預設值 DEFAULT ('[]')——MySQL 8.0.13+ 才支援，
// 這正是「比照既有寫法」擋不掉、只有實跑才驗得到的那類問題。
func assertMarketBackfillJobsSchema(t *testing.T, db *sqlx.DB) {
	t.Helper()

	var col struct {
		DataType   string  `db:"DATA_TYPE"`
		IsNullable string  `db:"IS_NULLABLE"`
		Default    *string `db:"COLUMN_DEFAULT"`
	}
	err := db.Get(&col, `
		SELECT DATA_TYPE, IS_NULLABLE, COLUMN_DEFAULT
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_NAME = 'market_backfill_jobs'
		  AND COLUMN_NAME = 'failures'
	`)
	if err != nil {
		t.Fatalf("查不到 market_backfill_jobs.failures 欄位: %v", err)
	}
	if col.DataType != "text" {
		t.Errorf("failures 型別 = %q, want text", col.DataType)
	}
	if col.IsNullable != "NO" {
		t.Errorf("failures IS_NULLABLE = %q, want NO", col.IsNullable)
	}
	// MySQL 對運算式預設值回傳的是類似 _utf8mb4\'[]\' 的字串，只驗有沒有 []。
	if col.Default == nil || !strings.Contains(*col.Default, "[]") {
		t.Errorf("failures 預設值 = %v, want 含 []（store.RawJSON 沒有實作 sql.Scanner，不能是 NULL）", col.Default)
	}
}

func countTables(t *testing.T, db *sqlx.DB) int {
	t.Helper()
	var n int
	if err := db.Get(&n,
		`SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE()`); err != nil {
		t.Fatalf("數表失敗: %v", err)
	}
	return n
}

// assertMySQLColumns 檢查回滾到某個版本之後，表的欄位確實是那一版的形狀。
func assertMySQLColumns(t *testing.T, db *sqlx.DB, table string, want ...string) {
	t.Helper()
	var names []string
	if err := db.Select(&names, `
		SELECT COLUMN_NAME FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
	`, table); err != nil {
		t.Fatalf("讀 %s 的欄位失敗: %v", table, err)
	}
	if len(names) == 0 {
		t.Fatalf("表 %s 不存在——Down 沒有把前一版結構重建回來", table)
	}
	have := make(map[string]bool, len(names))
	for _, n := range names {
		have[n] = true
	}
	for _, c := range want {
		if !have[c] {
			t.Errorf("%s 缺少欄位 %q——Down 還原的結構不是預期的版本", table, c)
		}
	}
}

// down 之後 goose_db_version 本身會留著（goose 不會刪自己的版本表），要排除掉。
func countTablesExcludingGoose(t *testing.T, db *sqlx.DB) int {
	t.Helper()
	var n int
	if err := db.Get(&n, `
		SELECT COUNT(*) FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME <> 'goose_db_version'
	`); err != nil {
		t.Fatalf("數表失敗: %v", err)
	}
	return n
}

// 失敗時列出殘留的表名，比只給一個數字好查。
func tableNames(t *testing.T, db *sqlx.DB) []string {
	t.Helper()
	var names []string
	if err := db.Select(&names, `
		SELECT TABLE_NAME FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME <> 'goose_db_version'
		ORDER BY TABLE_NAME
	`); err != nil {
		t.Fatalf("列表失敗: %v", err)
	}
	return names
}

// assertCandlePositivePriceCheck 驗 060 的 CHECK 真的被強制執行（見 docs/database-schema.md）。
//
// migration 跑得過**不代表約束有效**：MySQL 8.0.16 之前的版本會把 CHECK 解析掉但不強制，
// 而 `ALTER TABLE ... ADD CONSTRAINT` 在任何版本都不會報錯。只有實際寫一列違規資料
// 進去才知道它有沒有在做事。
func assertCandlePositivePriceCheck(t *testing.T, db *sqlx.DB) {
	t.Helper()
	const insert = `INSERT INTO candles (symbol, timeframe, ` + "`open`" + `, high, low, ` + "`close`" + `, volume, amount, ts)
	                VALUES (?, '1d', ?, ?, ?, ?, 1000, 0, '2026-01-01 00:00:00')`

	// 全零 K 棒是 live 上真的出現過的形狀，必須被擋。
	if _, err := db.Exec(insert, "3630", 0.0, 0.0, 0.0, 0.0); err == nil {
		t.Error("全零 K 棒竟然寫得進去——CHECK 約束沒生效")
	}
	// 只有單一欄位為 0 也要擋，確認約束涵蓋四個價格欄位而不是只看其中一個。
	for i, c := range [][4]float64{{0, 11, 9, 10}, {10, 0, 9, 10}, {10, 11, 0, 10}, {10, 11, 9, 0}} {
		if _, err := db.Exec(insert, fmt.Sprintf("t%d", i), c[0], c[1], c[2], c[3]); err == nil {
			t.Errorf("價格 %v 竟然寫得進去——CHECK 沒涵蓋所有價格欄位", c)
		}
	}
	// 正常的要寫得進去，別擋過頭。
	if _, err := db.Exec(insert, "2330", 99.0, 101.0, 98.0, 100.0); err != nil {
		t.Errorf("正常 K 棒被擋掉了: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM candles WHERE symbol = '2330'`); err != nil {
		t.Errorf("清理測試資料失敗: %v", err)
	}
}
