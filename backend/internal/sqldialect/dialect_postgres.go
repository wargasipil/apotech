//go:build !sqlite

package sqldialect

import (
	"fmt"
	"strings"

	"gorm.io/gorm/clause"
)

// ILike returns "col ILIKE ?". The caller is responsible for supplying the
// matching argument (typically a "%query%" string).
func ILike(col string) string {
	return col + " ILIKE ?"
}

// ILikeAny joins ILike(col) for each column with " OR ". The caller passes
// one matching argument per column to the .Where call.
func ILikeAny(cols ...string) string {
	parts := make([]string, len(cols))
	for i, c := range cols {
		parts[i] = ILike(c)
	}
	return strings.Join(parts, " OR ")
}

// DateTrunc returns a SQL expression that yields the bucket-key STRING for
// the column at the requested granularity. granularity is one of
// "day"/"week"/"month". Format mirrors the Go-side dayBucketKey output so
// the SQL result lands as a usable map key without further formatting:
//
//	day   → "YYYY-MM-DD"
//	week  → "YYYY-Www"  (ISO week)
//	month → "YYYY-MM"
func DateTrunc(granularity, col string) string {
	switch granularity {
	case "month":
		return fmt.Sprintf("TO_CHAR(%s, 'YYYY-MM')", col)
	case "week":
		// ISO year + ISO week, e.g. "2026-W23".
		return fmt.Sprintf("TO_CHAR(%s, 'IYYY-\"W\"IW')", col)
	default:
		return fmt.Sprintf("TO_CHAR(%s, 'YYYY-MM-DD')", col)
	}
}

// UnixEpoch returns a SQL expression yielding the unix-epoch seconds for a
// timestamp column, cast to bigint.
func UnixEpoch(col string) string {
	return fmt.Sprintf("EXTRACT(EPOCH FROM %s)::bigint", col)
}

// DatePlusDays returns a SQL expression for "today + N days" as a DATE.
// Used for expiring-soon cutoffs in analytics queries.
func DatePlusDays(days int) string {
	return fmt.Sprintf("(CURRENT_DATE + INTERVAL '%d days')", days)
}

// LockForUpdate returns the GORM clause that emits `FOR UPDATE`.
func LockForUpdate() clause.Expression {
	return clause.Locking{Strength: "UPDATE"}
}

// LockForUpdateSkipLocked returns the GORM clause that emits
// `FOR UPDATE SKIP LOCKED` (used by the NSFP pool allocator).
func LockForUpdateSkipLocked() clause.Expression {
	return clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}
}
