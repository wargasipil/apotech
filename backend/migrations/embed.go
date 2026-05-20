package migrations

import "embed"

// FS holds every goose migration, embedded into the binary so the server (and
// the migrate command) can run them without the source tree on disk. This is
// what makes the single self-contained binary work in Docker and on Windows.
//
//go:embed *.sql
var FS embed.FS
