// Package sqldialect emits SQL fragments + GORM clauses whose syntax differs
// between Postgres (default build) and SQLite (`-tags sqlite`). Handler code
// imports this package so it stays driver-agnostic:
//
//	q.Where(sqldialect.ILike("name")+" OR "+sqldialect.ILike("phone"), like, like)
//	tx.Clauses(sqldialect.LockForUpdate()).First(&row, "id = ?", id)
//
// The two backing files dialect_postgres.go and dialect_sqlite.go expose the
// same API; the build tag picks one at compile time.
package sqldialect
