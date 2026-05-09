package database

import (
	"database/sql"
	"fmt"
	"time"

	"laghaim-go/internal/platform/config"

	_ "github.com/go-sql-driver/mysql"
)

func OpenMySQL(cfg config.DatabaseConfig) (*sql.DB, error) {
	if cfg.DSN == "" {
		return nil, fmt.Errorf("database dsn is empty")
	}

	db, err := sql.Open("mysql", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}

	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime != "" {
		lifetime, err := time.ParseDuration(cfg.ConnMaxLifetime)
		if err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("parse conn max lifetime %q: %w", cfg.ConnMaxLifetime, err)
		}
		db.SetConnMaxLifetime(lifetime)
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping mysql: %w", err)
	}
	return db, nil
}
