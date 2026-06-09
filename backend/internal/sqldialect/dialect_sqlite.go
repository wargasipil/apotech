//go:build sqlite

package sqldialect

import (
	"fmt"
	"strings"

	"gorm.io/gorm/clause"
)

// ILike returns "col LIKE ?". SQLite's LIKE is ASCII case-insensitive by
// default — which matches every Apotech call site (codes, emails, names).
func ILike(col string) string {
	return col + " LIKE ?"
}

// ILikeAny joins ILike(col) for each column with " OR ".
func ILikeAny(cols ...string) string {
	parts := make([]string, len(cols))
	for i, c := range cols {
		parts[i] = ILike(c)
	}
	return strings.Join(parts, " OR ")
}

// DateTrunc returns the SQLite strftime expression that produces the same
// bucket key ("YYYY-MM-DD" / "YYYY-Www" / "YYYY-MM") the Postgres handler
// already emits.
func DateTrunc(granularity, col string) string {
	switch granularity {
	case "day":
		return fmt.Sprintf("strftime('%%Y-%%m-%%d', %s)", col)
	case "week":
		// strftime('%Y-W%W', …) → "2026-W23" — same ISO-ish week key the
		// frontend already accepts.
		return fmt.Sprintf("strftime('%%Y-W%%W', %s)", col)
	case "month":
		return fmt.Sprintf("strftime('%%Y-%%m', %s)", col)
	default:
		return fmt.Sprintf("strftime('%%Y-%%m-%%d', %s)", col)
	}
}

// UnixEpoch returns the SQLite expression yielding unix-epoch seconds as an
// INTEGER (which SQLite stores as 64-bit).
func UnixEpoch(col string) string {
	return fmt.Sprintf("CAST(strftime('%%s', %s) AS INTEGER)", col)
}

// DatePlusDays returns a SQL expression for "today + N days" as text in
// "YYYY-MM-DD" form. SQLite has no INTERVAL keyword; date() with a modifier
// is the canonical replacement and compares cleanly to a TEXT date column.
func DatePlusDays(days int) string {
	return fmt.Sprintf("date('now', '+%d days')", days)
}

// LockForUpdate is a no-op on SQLite — SQLite serializes writes at the
// database level (one writer at a time), so the existing "lock-then-mutate"
// logic in handlers remains correct without an explicit clause.
func LockForUpdate() clause.Expression {
	return noopClause{}
}

// LockForUpdateSkipLocked is also a no-op on SQLite. The Postgres NSFP path
// uses SKIP LOCKED to let concurrent allocators step around each other; under
// SQLite they serialize naturally and the busy_timeout pragma (set in the
// connection DSN) keeps them from failing under contention.
func LockForUpdateSkipLocked() clause.Expression {
	return noopClause{}
}

// noopClause mimics gorm.io/gorm/clause.Locking's interface (Build, Name,
// MergeClause) but emits nothing. Critical: without Name() returning "FOR",
// GORM treats a Clauses() expression as a WHERE condition and emits a
// trailing "AND" with empty body — which then breaks UPDATE statements with
// "incomplete input" on SQLite.
type noopClause struct{}

func (noopClause) Build(clause.Builder)      {}
func (noopClause) Name() string              { return "FOR" }
func (noopClause) MergeClause(_ *clause.Clause) {}
