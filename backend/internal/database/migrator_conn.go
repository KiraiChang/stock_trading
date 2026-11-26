package database

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

// MigratorConnOptions 是 -migrate 模式的連線參數（只用在 PostgreSQL）。
type MigratorConnOptions struct {
	// ApplicationName：pg_stat_activity 認得出這一次的 migrator（trading-migrator-<run_id>；D7 的停止確認只看它）。
	ApplicationName string
	// LockTimeout／StatementTimeout：每條連線的 lock_timeout 與 statement_timeout（0＝⛔ 不設）。
	LockTimeout      time.Duration
	StatementTimeout time.Duration
}

// OpenPostgresMigrator 用 DSN 開一個只給 migration 用的連線池：application_name、lock_timeout、statement_timeout
// 放在連線的啟動參數裡（每一條新連線都帶，⛔ 不靠 SET——連線池換連線時 SET 會掉）。
// 錯誤只說是哪一步（⛔ 不帶 DSN）。
func OpenPostgresMigrator(dsn string, o MigratorConnOptions) (*sqlx.DB, error) {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse database dsn failed")
	}
	if cfg.RuntimeParams == nil {
		cfg.RuntimeParams = map[string]string{}
	}
	if o.ApplicationName != "" {
		cfg.RuntimeParams["application_name"] = o.ApplicationName
	}
	if o.LockTimeout > 0 {
		cfg.RuntimeParams["lock_timeout"] = strconv.FormatInt(o.LockTimeout.Milliseconds(), 10)
	}
	if o.StatementTimeout > 0 {
		cfg.RuntimeParams["statement_timeout"] = strconv.FormatInt(o.StatementTimeout.Milliseconds(), 10)
	}
	db := sqlx.NewDb(stdlib.OpenDB(*cfg), "pgx")
	// goose 逐支套用時一次只用一條連線；上限設小，pg_stat_activity 的筆數也好判讀
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect database failed")
	}
	return db, nil
}

// InstanceSQL 是「哪一個 PostgreSQL 實例上的哪一個 database」的識別（T-087 review）：叢集的 system_identifier、database 名稱與 oid、
// postmaster 的啟動時間（微秒）。migrator 與 deploy.sh（docker exec DB_CONTAINER psql）用**同一段 SQL**，輸出逐字比對——
// 版本指紋只涵蓋已套用的版本，兩個不同的 DB 可能相同，要再核對兩條連線指向同一個實例與 database。
const InstanceSQL = `SELECT s.system_identifier::text || '|' || current_database() || '|' || d.oid::text || '|' || ` +
	`to_char(pg_postmaster_start_time() AT TIME ZONE 'UTC', 'YYYYMMDDHH24MISSUS') ` +
	`FROM pg_control_system() s, pg_database d WHERE d.datname = current_database()`

// ReadInstance 讀這條連線的實例識別（只有 PostgreSQL；唯讀）。
func ReadInstance(ctx context.Context, db *sqlx.DB) (string, error) {
	var id string
	if err := db.GetContext(ctx, &id, InstanceSQL); err != nil {
		return "", fmt.Errorf("read instance: %w", err)
	}
	return id, nil
}
