package store

import (
	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

func NewSQLite(dsn string) (*sqlx.DB, error) {
	if dsn == "" {
		dsn = "./trading.db"
	}
	db, err := sqlx.Connect("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// WAL 模式提升並行讀取效能
	db.MustExec(`PRAGMA journal_mode=WAL`)
	db.MustExec(`PRAGMA foreign_keys=ON`)
	// SQLite 建議單一 writer，讀取可多個
	db.SetMaxOpenConns(1)
	return db, nil
}
