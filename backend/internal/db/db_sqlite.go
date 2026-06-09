//go:build sqlite

package db

import (
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/apotech/backend/internal/config"
)

// openDialect returns the SQLite dialector under `-tags sqlite`. Uses the
// pure-Go modernc.org/sqlite driver (no CGO) so cross-compiling
// `GOOS=windows GOARCH=amd64` works from Linux/macOS hosts.
//
// DSN pragmas:
//   - journal_mode=WAL: concurrent reads alongside a single writer
//   - foreign_keys=ON:   honors ON DELETE CASCADE etc. (SQLite default is OFF)
//   - busy_timeout=30s:  retry-on-lock so contended writes don't fail fast
func openDialect(cfg *config.Config) (gorm.Dialector, error) {
	if !cfg.Database.IsSQLite() {
		return nil, fmt.Errorf("config requests %q driver but this binary was built with the sqlite tag; rebuild without the tag for postgres", cfg.Database.Driver)
	}
	path := cfg.Database.Path
	if path == "" {
		return nil, fmt.Errorf("database.path is required for sqlite driver")
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create sqlite data dir %s: %w", dir, err)
		}
	}
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(30000)"
	d := sqlite.Dialector{DriverName: "sqlite", DSN: dsn}
	return &d, nil
}
