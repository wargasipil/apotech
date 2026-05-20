// Package dbmigrate runs the embedded goose migrations against a live DB.
// Used by the server on boot (auto-migrate) so a freshly deployed binary brings
// its own schema up to date with no separate migrate step.
package dbmigrate

import (
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"

	"github.com/apotech/backend/migrations"
)

// Run applies all pending migrations embedded in the binary. Idempotent: a
// fully-migrated DB is a no-op.
func Run(sqlDB *sql.DB) error {
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("goose dialect: %w", err)
	}
	// "." is the root of the embedded FS (migrations live at the top level).
	if err := goose.Up(sqlDB, "."); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}
