package main

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/apotech/backend/internal/config"
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

	if err := goose.Run(cmd, sqlDB, "migrations", args...); err != nil {
		log.Fatal(err)
	}
}
