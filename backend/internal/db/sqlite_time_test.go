package db

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type timeProbe struct {
	ID        string `gorm:"primaryKey"`
	CreatedAt time.Time
}

func (timeProbe) TableName() string { return "time_probe" }

// TestSqliteTimeStorage is the Phase-0 gate, kept as a permanent regression
// test: it guarantees that the glebarez/modernc SQLite driver stores time.Time
// in a layout where (a) CAST(strftime('%s', col) AS INTEGER) yields the correct
// Unix epoch (used by the analytics/sales epoch aggregations) and (b) lexical
// range comparisons `col >= ? AND col < ?` with time.Time bound params select
// the right rows (used pervasively for date-range filters). The driver's
// default serialization (RFC3339 with offset + fractional seconds) already
// satisfies both — no custom serializer is required. If a future driver bump
// breaks this, the SQLite analytics/date-range queries would silently return
// zero/NULL, so fail loudly here instead.
func TestSqliteTimeStorage(t *testing.T) {
	loc := time.FixedZone("WIB", 7*3600) // Indonesia has no DST: constant offset.
	gdb, err := gorm.Open(sqlite.Open("file:timeprobe?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, gdb.Exec(`CREATE TABLE time_probe (id TEXT PRIMARY KEY, created_at DATETIME)`).Error)

	ts := time.Date(2026, 6, 4, 12, 0, 0, 123456789, loc) // 05:00:00 UTC
	require.NoError(t, gdb.Create(&timeProbe{ID: "a", CreatedAt: ts}).Error)

	// (a) strftime epoch conversion.
	var epoch *int64
	require.NoError(t, gdb.Raw(`SELECT CAST(strftime('%s', created_at) AS INTEGER) FROM time_probe WHERE id='a'`).Scan(&epoch).Error)
	require.NotNil(t, epoch, "strftime returned NULL — driver time layout is not strftime-parseable")
	require.Equal(t, ts.Unix(), *epoch)

	// (b) lexical range comparison with time.Time bound params.
	var in, out int64
	require.NoError(t, gdb.Raw(`SELECT COUNT(*) FROM time_probe WHERE created_at >= ? AND created_at < ?`, ts.Add(-time.Hour), ts.Add(time.Hour)).Scan(&in).Error)
	require.NoError(t, gdb.Raw(`SELECT COUNT(*) FROM time_probe WHERE created_at >= ?`, ts.Add(time.Hour)).Scan(&out).Error)
	require.Equal(t, int64(1), in, "in-range filter missed the row")
	require.Equal(t, int64(0), out, "out-of-range filter matched the row")

	// Round-trip back into time.Time scans cleanly.
	var got timeProbe
	require.NoError(t, gdb.First(&got, "id = ?", "a").Error)
	require.Equal(t, ts.Unix(), got.CreatedAt.Unix())
}
