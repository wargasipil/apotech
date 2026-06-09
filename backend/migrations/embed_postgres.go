//go:build !sqlite

package migrations

import "embed"

// FS holds every goose migration for the Postgres flavor.
//
//go:embed *.sql
var FS embed.FS

// Dialect is the goose dialect string for the embedded set.
const Dialect = "postgres"
