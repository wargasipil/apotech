package main

import (
	"database/sql"
	"io/fs"
	"log"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"

	"github.com/apotech/backend/internal/config"
	"github.com/apotech/backend/migrations"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: migrate <up|down|status|create|reset|version>")
	}
	cmd := os.Args[1]
	args := os.Args[2:]

	cfg, err := config.Load("")
	if err != nil {
		log.Fatal(err)
	}

	// Select the driver, goose dialect, and migration subdirectory from config.
	sqlDriver, dialect := "pgx", "postgres"
	dsn := cfg.Database.DSN()
	if cfg.Database.IsSQLite() {
		sqlDriver, dialect = "sqlite", "sqlite3"
		dsn = cfg.Database.SQLiteDSN()
	}

	sqlDB, err := sql.Open(sqlDriver, dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer sqlDB.Close()

	if err := goose.SetDialect(dialect); err != nil {
		log.Fatal(err)
	}

	// `create` writes a new .sql file to disk, so it uses the on-disk migrations
	// dir for the active driver (relative to backend/ — the Makefile sets that
	// CWD). Author every new migration in BOTH dialect dirs. Every other command
	// reads the migrations embedded in the binary (rooted at the dialect
	// subdir via fs.Sub), so it works regardless of the working directory.
	dir := "."
	if cmd == "create" {
		dir = "migrations/" + migrations.Dir(cfg.Database.Driver)
	} else {
		sub, err := fs.Sub(migrations.FS, migrations.Dir(cfg.Database.Driver))
		if err != nil {
			log.Fatal(err)
		}
		goose.SetBaseFS(sub)
	}

	if err := goose.Run(cmd, sqlDB, dir, args...); err != nil {
		log.Fatal(err)
	}
}
