// Package migrations embeds the goose SQL migration files into the binary
// so the server (and `cmd/migrate`) can run them without the source tree on
// disk. The actual `embed.FS` + dialect string lives in one of:
//
//   - embed_postgres.go (`//go:build !sqlite`) — embeds backend/migrations/*.sql
//   - embed_sqlite.go   (`//go:build sqlite`)   — embeds backend/migrations_sqlite/*.sql
//
// The migration sets are kept in parallel directories so each dialect's SQL
// stays readable in isolation; the version-id numeric prefixes match between
// the two so goose tracking is comparable.
package migrations
