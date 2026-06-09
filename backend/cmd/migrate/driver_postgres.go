//go:build !sqlite

package main

import (
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/apotech/backend/internal/config"
)

func openSQLDriver(cfg *config.Config) (driver, dsn string, err error) {
	return "pgx", cfg.Database.DSN(), nil
}
