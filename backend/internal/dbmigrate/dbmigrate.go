// Package dbmigrate runs the embedded goose migrations against a live DB.
// Used by the server on boot (auto-migrate) so a freshly deployed binary brings
// its own schema up to date with no separate migrate step.
package dbmigrate

import (
	"database/sql"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"

	"github.com/apotech/backend/migrations"
)

// Run applies all pending migrations embedded in the binary for the given
// driver ("postgres" or "sqlite"). Idempotent: a fully-migrated DB is a no-op.
func Run(sqlDB *sql.DB, driver string) error {
	dialect := "postgres"
	if driver == "sqlite" {
		dialect = "sqlite3"
	}
	// Root the embedded FS at the dialect subdirectory so goose reads "." and
	// never touches the OS filesystem.
	sub, err := fs.Sub(migrations.FS, migrations.Dir(driver))
	if err != nil {
		return fmt.Errorf("migrations sub-fs: %w", err)
	}
	goose.SetBaseFS(sub)
	if err := goose.SetDialect(dialect); err != nil {
		return fmt.Errorf("goose dialect: %w", err)
	}
	if err := goose.Up(sqlDB, "."); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}
