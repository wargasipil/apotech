package service

import (
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// dialect.go centralizes the handful of SQL constructs that differ between the
// two supported backends (Postgres and SQLite). Every helper branches on
// db.Dialector.Name() so a single query string works on both. See CLAUDE.md
// ("Database backends") for the rationale.

// isSQLite reports whether the active connection is the SQLite backend.
func isSQLite(db *gorm.DB) bool { return db.Dialector.Name() == "sqlite" }

// likeKeyword returns the case-insensitive LIKE operator for the active dialect:
// Postgres ILIKE, or plain LIKE on SQLite (whose LIKE is ASCII-case-insensitive
// by default — adequate for the Latin medicine/customer/supplier names searched
// here).
func likeKeyword(db *gorm.DB) string {
	if isSQLite(db) {
		return "LIKE"
	}
	return "ILIKE"
}

// epochExpr returns a SQL expression yielding the integer Unix epoch (seconds)
// of a timestamp column/expression.
func epochExpr(db *gorm.DB, expr string) string {
	if isSQLite(db) {
		return fmt.Sprintf("CAST(strftime('%%s', %s) AS INTEGER)", expr)
	}
	return fmt.Sprintf("EXTRACT(EPOCH FROM %s)::bigint", expr)
}

// dateText returns a SQL expression formatting a date/timestamp column as
// 'YYYY-MM-DD' text.
func dateText(db *gorm.DB, expr string) string {
	if isSQLite(db) {
		return fmt.Sprintf("strftime('%%Y-%%m-%%d', %s)", expr)
	}
	return fmt.Sprintf("to_char(%s, 'YYYY-MM-DD')", expr)
}

// dateBucketExpr truncates a timestamp column to the start of its day/week/month
// bucket, formatted as 'YYYY-MM-DD' text (the caller parses it and feeds
// dayBucketKey). gran is one of "day"/"week"/"month" (from truncFmt — a fixed,
// trusted set, so inlining it is injection-safe).
func dateBucketExpr(db *gorm.DB, gran, col string) string {
	if isSQLite(db) {
		switch gran {
		case "month":
			return fmt.Sprintf("strftime('%%Y-%%m-01', %s)", col)
		case "week":
			// ISO week starts Monday. strftime('%w') is 0=Sun..6=Sat; subtract
			// (w+6)%7 days to land on Monday.
			return fmt.Sprintf("date(%s, '-' || ((strftime('%%w', %s) + 6) %% 7) || ' days')", col, col)
		default:
			return fmt.Sprintf("date(%s)", col)
		}
	}
	return fmt.Sprintf("to_char(date_trunc('%s', %s), 'YYYY-MM-DD')", gran, col)
}

// dateCast returns the Postgres "::date" cast suffix (empty on SQLite). Used to
// coerce a 'YYYY-MM-DD' string parameter for comparison against a timestamp
// column: Postgres has no timestamptz >= text operator and needs the explicit
// cast, while SQLite compares the value lexically by its date prefix.
func dateCast(db *gorm.DB) string {
	if isSQLite(db) {
		return ""
	}
	return "::date"
}

// dateAddDaysExpr returns "base + n days" as a date expression.
func dateAddDaysExpr(db *gorm.DB, base string, n int) string {
	if isSQLite(db) {
		return fmt.Sprintf("date(%s, '+%d days')", base, n)
	}
	return fmt.Sprintf("(%s + INTERVAL '%d days')", base, n)
}

// applyForUpdate adds a FOR UPDATE row lock on Postgres and is a no-op on SQLite.
// SQLite serializes all writers (BEGIN IMMEDIATE via the _txlock DSN param +
// SetMaxOpenConns(1)), so the read-check-insert oversell guard holds without an
// explicit row lock. Call inside a transaction, exactly where clause.Locking was.
func applyForUpdate(tx *gorm.DB) *gorm.DB {
	if isSQLite(tx) {
		return tx
	}
	return tx.Clauses(clause.Locking{Strength: "UPDATE"})
}

// applyForUpdateSkipLocked adds FOR UPDATE SKIP LOCKED on Postgres; on SQLite it
// is a plain query (serialized writers make SKIP LOCKED moot — there is only one
// active writer, so the lowest unused row is always the one returned).
func applyForUpdateSkipLocked(tx *gorm.DB) *gorm.DB {
	if isSQLite(tx) {
		return tx
	}
	return tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
}
