package database

import (
	"context"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"
	"go.uber.org/zap"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// postgres migration 的實跑驗證。
//
// **為什麼 dev stack 有 postgres 還需要這支**：dev 的 backend 啟動時只跑 Up，
// Down 那一半沒有任何執行路徑。017／018 的回滾鏈斷掉就是這樣一直沒被發現的
// （見 docs/development-workflow.md 的「migration 的 Down 區塊也要能跑」）。
// 要驗 Down 得 down 到 0，那會清光 dev 的資料，所以需要一個可丟棄的實例。
//
// **需要一個真的 postgres**，所以用環境變數 gate 住：沒設 POSTGRES_MIGRATION_DSN 就 skip，
// 一般的 backend/scripts/test.sh ./... 行為完全不受影響。
// 跑法見 scripts/test-postgres-migrations.sh（它會處理 compose 與記憶體錯開）。
//
// 刻意放在 package database 內部：migration 是用 //go:embed 打包的，只有這裡拿得到
// postgresFS，才能用 backend 真正會執行的那份檔案跑 down 驗回滾。

const postgresMigrationDir = "migrations/postgres"

func postgresTestDB(t *testing.T) *sqlx.DB {
	t.Helper()

	dsn := os.Getenv("POSTGRES_MIGRATION_DSN")
	if dsn == "" {
		t.Skip("未設 POSTGRES_MIGRATION_DSN，跳過 postgres migration 實跑驗證（用 scripts/test-postgres-migrations.sh 執行）")
	}

	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Fatalf("連不上 postgres: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// postgresMigrationVersions 從 embed 的檔名取出版本號，期望值跟著檔案走，
// 不用每加一個 migration 就回來改硬編碼的數字。
func postgresMigrationVersions(t *testing.T) []int64 {
	t.Helper()

	entries, err := fs.ReadDir(postgresFS, postgresMigrationDir)
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
		t.Fatal("embed 的 postgres migration 目錄是空的")
	}
	return versions
}

func postgresTableCount(t *testing.T, db *sqlx.DB, excludeGoose bool) int {
	t.Helper()
	query := `SELECT COUNT(*) FROM information_schema.tables
	          WHERE table_schema = 'public' AND table_type = 'BASE TABLE'`
	if excludeGoose {
		query += ` AND table_name <> 'goose_db_version'`
	}
	var n int
	if err := db.Get(&n, query); err != nil {
		t.Fatalf("查表數失敗: %v", err)
	}
	return n
}

func postgresTableNames(t *testing.T, db *sqlx.DB) []string {
	t.Helper()
	var names []string
	if err := db.Select(&names, `SELECT table_name FROM information_schema.tables
	                             WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
	                               AND table_name <> 'goose_db_version' ORDER BY table_name`); err != nil {
		t.Fatalf("列表失敗: %v", err)
	}
	return names
}

func assertPostgresColumns(t *testing.T, db *sqlx.DB, table string, want ...string) {
	t.Helper()
	var names []string
	if err := db.Select(&names, `SELECT column_name FROM information_schema.columns
	                             WHERE table_schema = 'public' AND table_name = $1`, table); err != nil {
		t.Fatalf("讀 %s 的欄位失敗: %v", table, err)
	}
	if len(names) == 0 {
		t.Fatalf("表 %s 不存在或沒有欄位", table)
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

const insertCandle = `INSERT INTO candles (symbol, timeframe, open, high, low, close, volume, amount, ts)
                      VALUES ($1, '1d', $2, $3, $4, $5, 1000, 0, '2026-01-01 00:00:00+00')`

// TestPostgresMigrationsUpAndDown 跑完整 up、驗 schema，再分段 down 到 0 驗回滾鏈。
func TestPostgresMigrationsUpAndDown(t *testing.T) {
	db := postgresTestDB(t)
	ctx := context.Background()

	// 前置：要求一個乾淨的 DB。migration 的價值就在「從空的跑到最新」，
	// 殘留的表會讓 CREATE TABLE IF NOT EXISTS 靜靜跳過而驗不到東西。
	if n := postgresTableCount(t, db, false); n != 0 {
		t.Fatalf("DB 不是空的（有 %d 張表）。這支測試要求乾淨的 DB，請用 scripts/test-postgres-migrations.sh 執行（它會 down -v）", n)
	}

	versions := postgresMigrationVersions(t)
	want := versions[len(versions)-1]

	// 走 cmd/server/main.go 完全相同的進入點，而不是自己拼 goose 呼叫。
	if err := RunMigrations(ctx, db, "postgres", zap.NewNop()); err != nil {
		t.Fatalf("postgres migration up 失敗: %v", err)
	}

	got, err := goose.GetDBVersion(db.DB)
	if err != nil {
		t.Fatalf("讀不到 goose 版本: %v", err)
	}
	if got != want {
		t.Fatalf("goose 版本 = %d, want %d（最後一個 migration 檔）", got, want)
	}

	var applied int
	if err := db.GetContext(ctx, &applied,
		`SELECT COUNT(*) FROM goose_db_version WHERE version_id > 0 AND is_applied = true`); err != nil {
		t.Fatalf("查 goose_db_version 失敗: %v", err)
	}
	if applied != len(versions) {
		t.Fatalf("已套用 %d 筆，但 embed 有 %d 個 migration 檔", applied, len(versions))
	}

	assertPostgresCandlePriceCheck(t, db)

	// 回滾驗證：分段 down 並在中途檢查結構。理由同 migrate_sqlite_test.go——
	// 「一路 down 到 0 沒報錯」驗不到 017／018 的 Down 重建得對不對，因為 017 的 Down
	// 一執行就把 018 重建的表砍掉了，018 的 Down 內容寫錯也沒人知道。
	goose.SetBaseFS(postgresFS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("goose set dialect: %v", err)
	}

	if err := goose.DownToContext(ctx, db.DB, postgresMigrationDir, 17); err != nil {
		t.Fatalf("postgres migration down 到 17 失敗: %v", err)
	}
	assertPostgresColumns(t, db, "stock_sr_zones",
		"net_score", "net_score_label", "confidence_level", "reject_count", "break_count",
		"zone_momentum", "trading_score", "trading_recommendation")
	assertPostgresColumns(t, db, "stock_sr_zone_analyses", "overall_trend", "overall_volatility")

	if err := goose.DownToContext(ctx, db.DB, postgresMigrationDir, 16); err != nil {
		t.Fatalf("postgres migration down 到 16 失敗: %v", err)
	}
	assertPostgresColumns(t, db, "stock_sr_zones",
		"rejection_count", "breakout_count", "avg_return_after_touch", "trend_strength",
		"confidence", "expected_value", "risk_reward_ratio")

	if err := goose.DownToContext(ctx, db.DB, postgresMigrationDir, 0); err != nil {
		t.Fatalf("postgres migration down 到 0 失敗: %v", err)
	}

	if v, err := goose.GetDBVersion(db.DB); err != nil {
		t.Fatalf("讀不到 goose 版本: %v", err)
	} else if v != 0 {
		t.Fatalf("down 之後版本 = %d, want 0", v)
	}

	// goose 不會刪自己的版本表；除此之外應該一張表都不剩。
	if n := postgresTableCount(t, db, true); n != 0 {
		t.Fatalf("down 到 0 之後還剩 %d 張表，代表有 migration 的 Down 沒清乾淨: %v",
			n, postgresTableNames(t, db))
	}
}

// assertPostgresCandlePriceCheck 驗 060 的 CHECK 對**新寫入**確實生效。
//
// migration 跑得過不代表約束有效——`ADD CONSTRAINT ... NOT VALID` 不驗既有列，
// 很容易誤以為它連新資料也不管。只有實際寫一列違規資料進去才知道。
func assertPostgresCandlePriceCheck(t *testing.T, db *sqlx.DB) {
	t.Helper()

	if _, err := db.Exec(insertCandle, "3630", 0.0, 0.0, 0.0, 0.0); err == nil {
		t.Error("全零 K 棒竟然寫得進去——NOT VALID 的約束對新寫入也該生效")
	}
	for i, c := range [][4]float64{{0, 11, 9, 10}, {10, 0, 9, 10}, {10, 11, 0, 10}, {10, 11, 9, 0}} {
		if _, err := db.Exec(insertCandle, "t"+strconv.Itoa(i), c[0], c[1], c[2], c[3]); err == nil {
			t.Errorf("價格 %v 竟然寫得進去——CHECK 沒涵蓋所有價格欄位", c)
		}
	}
	if _, err := db.Exec(insertCandle, "2330", 99.0, 101.0, 98.0, 100.0); err != nil {
		t.Errorf("正常 K 棒被擋掉了: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM candles WHERE symbol = '2330'`); err != nil {
		t.Errorf("清理測試資料失敗: %v", err)
	}
}

// TestPostgresMigrationsToleratePreexistingBadRows 重現 live 當下的處境：
// candles 已經有髒資料（4 根全零 K 棒）而**還沒被清掉**，060 仍然必須套得上去。
//
// 這正是 postgres 版用 `NOT VALID` 的唯一理由。如果哪天有人「順手」把 NOT VALID 拿掉，
// 這支測試會失敗，而不是等到部署到 live 才炸。
func TestPostgresMigrationsToleratePreexistingBadRows(t *testing.T) {
	db := postgresTestDB(t)
	ctx := context.Background()

	// 前一支測試結束在版本 0，只剩 goose_db_version。
	if n := postgresTableCount(t, db, true); n != 0 {
		t.Fatalf("DB 不是空的（有 %d 張表）: %v", n, postgresTableNames(t, db))
	}

	goose.SetBaseFS(postgresFS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("goose set dialect: %v", err)
	}

	// 停在 059：約束還沒加，髒資料寫得進去。
	if err := goose.UpToContext(ctx, db.DB, postgresMigrationDir, 59); err != nil {
		t.Fatalf("up 到 59 失敗: %v", err)
	}
	if _, err := db.Exec(insertCandle, "3630", 0.0, 0.0, 0.0, 0.0); err != nil {
		t.Fatalf("059 版應該還擋不住全零 K 棒，但寫入失敗: %v", err)
	}

	// 關鍵：髒資料還在的情況下，060 必須套得上去。
	if err := goose.UpToContext(ctx, db.DB, postgresMigrationDir, 60); err != nil {
		t.Fatalf("既有髒資料還在時 060 套用失敗——NOT VALID 不見了？: %v", err)
	}

	// 既有的髒列不會被刪掉（NOT VALID 不回頭處理既有資料）。
	var bad int
	if err := db.Get(&bad, `SELECT COUNT(*) FROM candles WHERE low <= 0`); err != nil {
		t.Fatalf("查髒資料失敗: %v", err)
	}
	if bad != 1 {
		t.Fatalf("既有髒列 = %d, want 1（NOT VALID 不該動既有資料）", bad)
	}

	// 但新的違規寫入要被擋。
	if _, err := db.Exec(insertCandle, "3631", 0.0, 0.0, 0.0, 0.0); err == nil {
		t.Error("060 之後仍寫得進全零 K 棒——約束對新寫入沒生效")
	}

	// VALIDATE CONSTRAINT 在髒資料還在時必須失敗——這就是為什麼 issue.md 說
	// 「清完那 4 列之後」才跑它。
	if _, err := db.Exec(`ALTER TABLE candles VALIDATE CONSTRAINT ck_candles_positive_price`); err == nil {
		t.Error("髒資料還在時 VALIDATE CONSTRAINT 竟然成功了")
	}

	// 清掉之後才會成功。這條路徑先驗過，live 清完資料時才不會踩到意外。
	if _, err := db.Exec(`DELETE FROM candles WHERE low <= 0`); err != nil {
		t.Fatalf("清髒資料失敗: %v", err)
	}
	if _, err := db.Exec(`ALTER TABLE candles VALIDATE CONSTRAINT ck_candles_positive_price`); err != nil {
		t.Fatalf("清乾淨之後 VALIDATE CONSTRAINT 仍失敗: %v", err)
	}

	// Down 是真逆操作：約束消失，違規資料又寫得進去。
	if err := goose.DownToContext(ctx, db.DB, postgresMigrationDir, 59); err != nil {
		t.Fatalf("down 到 59 失敗: %v", err)
	}
	if _, err := db.Exec(insertCandle, "3632", 0.0, 0.0, 0.0, 0.0); err != nil {
		t.Fatalf("down 之後約束應該已移除，但寫入仍被擋: %v", err)
	}

	// 收乾淨，避免影響同一個 DB 上之後可能新增的測試。
	if err := goose.DownToContext(ctx, db.DB, postgresMigrationDir, 0); err != nil {
		t.Fatalf("收尾 down 到 0 失敗: %v", err)
	}
}
