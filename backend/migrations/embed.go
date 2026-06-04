package migrations

import "embed"

// FS holds every goose migration for both dialects, embedded into the binary so
// the server (and the migrate command) can run them without the source tree on
// disk. This is what makes the single self-contained binary work in Docker and
// on Windows. Postgres and SQLite each have their own subtree (with matching
// version numbers); Dir(driver) selects which one goose reads.
//
//go:embed postgres/*.sql sqlite/*.sql
var FS embed.FS

// Dir returns the embedded migration subdirectory for a config.Database.Driver
// value ("postgres" or "sqlite"). Anything other than "sqlite" maps to the
// Postgres set (Postgres is the default backend).
func Dir(driver string) string {
	if driver == "sqlite" {
		return "sqlite"
	}
	return "postgres"
}
