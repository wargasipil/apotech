//go:build sqlite

package migrations

import (
	"embed"
	"io/fs"
)

// rawFS contains the SQLite migration set rooted at "sqlite/". FS below
// re-roots it via fs.Sub so goose sees a flat *.sql layout (matching the
// Postgres flavor's embed pattern).
//
//go:embed sqlite/*.sql
var rawFS embed.FS

// FS is the embed FS rooted at the .sql files goose loads.
var FS fs.FS

// Dialect is the goose dialect string for the embedded set.
const Dialect = "sqlite3"

func init() {
	sub, err := fs.Sub(rawFS, "sqlite")
	if err != nil {
		panic("migrations: fs.Sub(sqlite): " + err.Error())
	}
	FS = sub
}
