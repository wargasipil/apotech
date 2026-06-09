//go:build sqlite

package e2e

import (
	"path/filepath"
	"testing"

	"github.com/apotech/backend/internal/config"
)

// applyTestDBOverride redirects this test's DB to a fresh per-test SQLite file
// under t.TempDir(). Auto-cleans on test exit.
func applyTestDBOverride(t *testing.T, cfg *config.Config) {
	t.Helper()
	cfg.Database.Driver = "sqlite"
	cfg.Database.Path = filepath.Join(t.TempDir(), "apotech.db")
}

// runMigrationsOnSetup is true under SQLite: each test starts from an empty
// file and SetupEnv brings it up to schema before EnsureBootstrapOwner runs.
const runMigrationsOnSetup = true
