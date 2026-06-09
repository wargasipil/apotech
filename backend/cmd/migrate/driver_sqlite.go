//go:build sqlite

package main

import (
	"fmt"

	_ "modernc.org/sqlite"

	"github.com/apotech/backend/internal/config"
)

func openSQLDriver(cfg *config.Config) (driver, dsn string, err error) {
	if cfg.Database.Path == "" {
		return "", "", fmt.Errorf("database.path is required for sqlite driver")
	}
	dsn = "file:" + cfg.Database.Path + "?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(30000)"
	return "sqlite", dsn, nil
}
