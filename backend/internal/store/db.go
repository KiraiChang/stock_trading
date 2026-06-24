package store

import (
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/trading/backend/internal/config"
)

// NewDB 依設定的 driver 建立資料庫連線
// driver="sqlite" (開發) 或 driver="mysql" (生產)
func NewDB(cfg config.DatabaseConfig) (*sqlx.DB, error) {
	switch cfg.Driver {
	case "mysql":
		return NewMySQL(cfg.DSN)
	case "postgres", "postgresql":
		return NewPostgres(cfg.DSN)
	case "sqlite", "":
		return NewSQLite(cfg.DSN)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}
}
