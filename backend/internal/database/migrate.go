package database

import (
	"context"
	"embed"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"
	"go.uber.org/zap"
)

//go:embed migrations/sqlite/*.sql
var sqliteFS embed.FS

//go:embed migrations/mysql/*.sql
var mysqlFS embed.FS

//go:embed migrations/postgres/*.sql
var postgresFS embed.FS

func RunMigrations(ctx context.Context, db *sqlx.DB, driver string, logger *zap.Logger) error {
	var fs embed.FS
	var dialect, dir string

	switch driver {
	case "sqlite", "":
		fs, dialect, dir = sqliteFS, "sqlite3", "migrations/sqlite"
	case "mysql":
		fs, dialect, dir = mysqlFS, "mysql", "migrations/mysql"
	case "postgres", "postgresql":
		fs, dialect, dir = postgresFS, "postgres", "migrations/postgres"
	default:
		return fmt.Errorf("unsupported driver for migrations: %s", driver)
	}

	goose.SetBaseFS(fs)
	goose.SetLogger(goose.NopLogger())

	if err := goose.SetDialect(dialect); err != nil {
		return fmt.Errorf("goose set dialect: %w", err)
	}

	if err := goose.UpContext(ctx, db.DB, dir); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}

	version, err := goose.GetDBVersion(db.DB)
	if err == nil {
		logger.Info("migrations applied",
			zap.String("driver", driver),
			zap.Int64("version", version),
		)
	}

	return nil
}
