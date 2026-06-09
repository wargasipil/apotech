//go:build !sqlite

package e2e

import (
	"testing"

	"github.com/apotech/backend/internal/config"
)

// applyTestDBOverride is a no-op for the Postgres test suite: tests use the
// shared dev DB declared in config.yaml. Each call still runs every test
// against that pre-migrated DB; isolation is per-row, not per-DB.
func applyTestDBOverride(_ *testing.T, _ *config.Config) {}

// runMigrationsOnSetup is false for Postgres: the dev DB is pre-migrated by
// `make migrate-up` (or auto-migrate on `make run`).
const runMigrationsOnSetup = false
