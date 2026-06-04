package dbmigrate

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// TestSQLiteMigrationsApply runs the full embedded SQLite migration set against
// a fresh temp database. This is the Phase-2 gate: every translated migration
// must parse and execute on SQLite, and the end-state must include the seeded
// MAIN warehouse the bootstrap flow depends on.
func TestSQLiteMigrationsApply(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "migtest.db")
	dsn := "file:" + dbPath + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_txlock=immediate"

	sqlDB, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	defer sqlDB.Close()

	require.NoError(t, Run(sqlDB, "sqlite"), "full SQLite migration set must apply")

	// Re-running is a no-op (idempotent).
	require.NoError(t, Run(sqlDB, "sqlite"))

	// Spot-check key tables exist.
	for _, tbl := range []string{
		"users", "suppliers", "medicines", "batches", "stock_movements",
		"sales", "sale_items", "purchase_orders", "prescriptions", "nsfp_pool",
		"bpjs_claims", "branches", "audit_log", "stocktake_sessions",
		"warehouses", "user_warehouses", "stock_transfers", "medicine_units",
		"medicine_unit_prices", "app_settings", "unit_bases", "unit_derivatives",
	} {
		var n int
		require.NoError(t, sqlDB.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, tbl).Scan(&n))
		require.Equalf(t, 1, n, "table %q should exist after migration", tbl)
	}

	// The default MAIN warehouse must be seeded with a non-null id (the bootstrap
	// owner's default-warehouse grant depends on it).
	var whID, whCode string
	require.NoError(t, sqlDB.QueryRow(
		`SELECT id, code FROM warehouses WHERE is_default = 1`).Scan(&whID, &whCode))
	require.Equal(t, "MAIN", whCode)
	require.NotEmpty(t, whID)

	// The widened stock_movements type CHECK must accept TRANSFER_IN (baked into
	// 00007 for SQLite). Verify by inserting the prerequisite rows + a transfer
	// movement.
	require.NoError(t, exec(sqlDB,
		`INSERT INTO users (id,email,password_hash,role) VALUES ('u1','u@x.com','h','OWNER')`))
	require.NoError(t, exec(sqlDB,
		`INSERT INTO medicines (id,sku,name,unit,unit_price) VALUES ('m1','SKU1','Med',' ',0)`))
	require.NoError(t, exec(sqlDB,
		`INSERT INTO batches (id,medicine_id,expiry_date) VALUES ('b1','m1','2030-01-01')`))
	require.NoError(t, exec(sqlDB,
		`INSERT INTO stock_movements (id,batch_id,qty,type,user_id,warehouse_id) VALUES ('sm1','b1',5,'TRANSFER_IN','u1',?)`, whID))

	// The original-narrow set must still reject an unknown type.
	require.Error(t, exec(sqlDB,
		`INSERT INTO stock_movements (id,batch_id,qty,type,user_id,warehouse_id) VALUES ('sm2','b1',5,'BOGUS','u1',?)`, whID),
		"CHECK constraint should reject an invalid movement type")
}

func exec(db *sql.DB, q string, args ...any) error {
	_, err := db.Exec(q, args...)
	return err
}
