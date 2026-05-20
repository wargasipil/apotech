package main

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

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

	sqlDB, err := sql.Open("pgx", cfg.Database.DSN())
	if err != nil {
		log.Fatal(err)
	}
	defer sqlDB.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatal(err)
	}

	// `create` writes a new .sql file to disk, so it uses the on-disk
	// migrations dir (relative to backend/ — the Makefile sets that CWD).
	// Every other command reads the migrations embedded in the binary, so it
	// works regardless of the working directory.
	dir := "."
	if cmd == "create" {
		dir = "migrations"
	} else {
		goose.SetBaseFS(migrations.FS)
	}

	if err := goose.Run(cmd, sqlDB, dir, args...); err != nil {
		log.Fatal(err)
	}
}
